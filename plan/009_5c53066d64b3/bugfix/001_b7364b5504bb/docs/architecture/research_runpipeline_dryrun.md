# Issue 1 — runPipeline (DryRun / SystemExtra) is missing the multi-turn fallback gate

> Scope: `pkg/stagehand/stagehand.go::runPipeline` (lines 415–611) vs. the reference
> implementation `internal/generate/generate.go::CommitStaged` (lines 138–475).
> The reference (CommitStaged) carries the **FR-T1 multi-turn fallback trigger gate**
> (generate.go:290–374); the mirror (runPipeline) does **NOT**. This is the MAJOR bug.
> All line numbers are against the working tree as of 2026-07-05.

---

## 1. Files Retrieved

1. `pkg/stagehand/stagehand.go` (full, 611 lines) — the public library surface; holds
   `GenerateCommit` (132), the routing decision (160–165), and `runPipeline` (415–611).
   **This is the file that must change.**
2. `internal/generate/generate.go` (138–390) — `CommitStaged`, the frozen/tested reference
   pipeline. Contains the canonical multi-turn gate (290–374) that runPipeline must mirror.
3. `internal/generate/multiturn.go` (full, 200 lines) — `Run` (exported, 136) and
   `chunkPayload` (**unexported**, 52). `Run` is the multi-turn transport; `chunkPayload`
   is only used by CommitStaged for the progress-message turn count.
4. `internal/generate/rescue.go` (1–147) — `FormatRescue` / `RescueError` user-facing text.
5. `internal/generate/generate.go` (59–115) — `RescueError` struct + `Error()` messages.
6. `internal/config/config.go` (84–85, 179–180) — `MultiTurnFallback`, `MultiTurnChunkTokens`
   config fields + defaults.
7. `internal/git/tokens.go:25` — `git.EstimateTokens` (exported, used by the gate's cond (b)).
8. `internal/ui/verbose.go:103` — `Verbose.VerboseWarn` (exported, used by FR-T11 line).
9. `pkg/stagehand/stagehand_test.go` (231–810, sampled) — existing DryRun/SystemExtra tests;
   **none** exercise the multi-turn path on runPipeline (confirming the gap).

---

## 2. The routing decision (GenerateCommit)

`GenerateCommit` (`pkg/stagehand/stagehand.go:132`) resolves config/deps, then routes.
Exact code (`stagehand.go:160–165`):

```go
	// Common path: no DryRun, no SystemExtra → delegate to the frozen, tested orchestrator.
	if !opts.DryRun && opts.SystemExtra == "" {
		res, gerr := generate.CommitStaged(ctx, deps, cfg)
		...
	}

	// Advanced path: DryRun and/or SystemExtra → self-contained (CommitStaged can't honor these).
	return runPipeline(ctx, deps, cfg, opts.SystemExtra, opts.DryRun)
```

**Routing truth table:**

| DryRun | SystemExtra | Path              | Multi-turn gate? |
|--------|-------------|-------------------|------------------|
| false  | ""          | `CommitStaged`    | ✅ present (290–374) |
| true   | any         | `runPipeline`     | ❌ **MISSING**    |
| false  | non-""      | `runPipeline`     | ❌ **MISSING**    |

So multi-turn works for the plain `git commit`-style call but is **silently absent** for
`--dry-run` and for any `SystemExtra` / library consumer passing extra instructions. The bug
is purely an omission in the mirror — the fix is to port the gate verbatim, not to re-architect.

---

## 3. runPipeline generation loop (the heart of the bug)

Function header (`stagehand.go:415`):
```go
func runPipeline(ctx context.Context, deps generate.Deps, cfg config.Config,
	systemExtra string, dryRun bool) (Result, error) {
```

**Pre-loop var hoists (`stagehand.go:483–487`):**
```go
	retryInstr := *resolved.RetryInstruction
	var rejected []string
	var candidate, msg string
	var parseFail, success bool
	var lastCause error
```

**Loop declaration and `payload` (`stagehand.go:489–492`):**
```go
	for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
		payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)   // <-- loop-scoped `:=`
		if parseFail {
			payload = retryInstr + "\n\n" + payload
		}
```

⚠️ **CRITICAL:** `payload` is declared with `:=` **inside** the loop (line 490), so it is
**loop-scoped and does NOT survive** past the closing brace. In the reference `CommitStaged`,
`payload` is hoisted with `var payload string` **before** the loop (generate.go:226) so the
FR-T1 gate can read it at generate.go:311 (`mtPayload := payload`). **This single scoping
difference is the structural root cause** that blocks a naive copy of the gate into runPipeline:
the gate's "fast path" (`mtPayload := payload` when `cfg.TokenLimit == 0`) needs the last
payload, which runPipeline has thrown away.

**Rescue return (`stagehand.go:543–548`) — exactly where the gate must be inserted:**
```go
	if !success {
		return Result{}, &generate.RescueError{
			Kind: generate.ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
			Candidate: candidate, Cause: lastCause,
		}
	}
```

---

## 4. Variables in scope at the rescue point (line 543)

This determines what the multi-turn gate can use without restructuring. Survey of every
identifier the gate references:

| Variable      | In scope? | Where declared                | Used by the gate as |
|---------------|-----------|-------------------------------|---------------------|
| `ctx`         | ✅ | param (415)               | `generate.Run(ctx, …)` |
| `deps`        | ✅ | param (415, type `generate.Deps`) | `Run(deps,…)`; `deps.Git.StagedDiff`; `deps.Verbose.VerboseWarn`; `deps.Manifest` |
| `cfg`         | ✅ | param (415)               | `cfg.MultiTurnFallback`, `cfg.MultiTurnChunkTokens`, `cfg.TokenLimit`, `cfg.Timeout`, `cfg.Context`, `cfg.MaxDiffBytes`, … |
| `parentSHA`   | ✅ | 417 (`RevParseHEAD`)      | (in scope; not needed by gate body but available) |
| `treeSHA`     | ✅ | 447 (`WriteTree`)         | (in scope; RescueError.TreeSHA) |
| `sysPrompt`   | ✅ | 425                       | `Run(…, sysPrompt, …)` |
| `resolved`    | ✅ | 470 (`deps.Manifest.Resolve()`) | **cond (d):** `*resolved.SessionMode == "append"` |
| `msgModel`    | ✅ | 474                       | `Run(…, msgModel, msgReasoning)` |
| `msgReasoning`| ✅ | 474                       | `Run(…, msgReasoning)` |
| `rejected`    | ✅ | 484                       | rebuild `mtPayload` after FR-T12 re-capture |
| `candidate`   | ✅ | 485                       | set on multi-turn duplicate (one-shot parity) |
| `msg`         | ✅ | 485                       | set on multi-turn success |
| `parseFail`   | ✅ | 486                       | (in scope) |
| `success`     | ✅ | 486                       | the gate sets `success = true` on win |
| `lastCause`   | ✅ | 486                       | `lastCause = cause` when Run fails (FR-T7) |
| `diff`        | ✅ | 446 (`StagedDiff`, TokenLimit=cfg.TokenLimit) | (available; the gate re-captures `fullDiff` itself when needed) |
| **`payload`** | ❌ | **loop-scoped `:=` at 490** | **MISSING — the gate needs the last payload for its fast path** |

**Conclusion:** every gate input is in scope **except `payload`**. Porting the gate requires
**one structural change**: hoist `payload` to function scope (`var payload string` after
line 486) and switch line 490 from `:=` to `=`. No other restructuring is needed.

> Note on `fullDiff`: the reference re-captures `fullDiff` *inside* the `if cfg.TokenLimit != 0`
> branch (generate.go:315–326) via a second `StagedDiff`. In runPipeline that block can be
> copied verbatim — `deps.Git`, `cfg`, `deps.Excludes`, `rejected` are all in scope.

---

## 5. The canonical FR-T1 multi-turn gate (CommitStaged, to be ported)

Exact reference code, `internal/generate/generate.go:290–374`. This is the **byte-for-byte
template** for the runPipeline insertion (modulo `generate.` package prefixes and the
`payload` hoist):

```go
	if !success {
		// FR-T1 multi-turn fallback trigger gate (PRD §9.24). Multi-turn activates ONLY when
		// one-shot exhausted (a) AND payload exceeds one chunk (b) AND multi_turn_fallback (c)
		// AND resolved manifest session_mode=="append" (d). Else fall through to rescue (FR-T7).
		// FR-T12: multi-turn IGNORES token_limit; re-capture the diff with TokenLimit=0.
		if cfg.MultiTurnFallback &&
			resolved.SessionMode != nil && *resolved.SessionMode == "append" {

			mtPayload := payload
			if cfg.TokenLimit != 0 {
				fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
					MaxDiffBytes:     cfg.MaxDiffBytes,
					MaxMDLines:       cfg.MaxMdLines,
					BinaryExtensions: cfg.BinaryExtensions,
					Excludes:         deps.Excludes,
					TokenLimit:       0, // FR-T12
					DiffContext:      cfg.DiffContextValue(),
					PromptReserveTokens: 0,
				})
				if derr == nil {
					mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
				}
			}

			if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {        // cond (b)
				turns := len(chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1 // <-- PROBLEM (see §9)
				totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
				if totalMin < 1 { totalMin = 1 }
				fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", turns, totalMin)
				deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")

				msg2, ok2, cause := Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)

				if cause == nil && ok2 {
					finalMsg := FinalizeMessage(msg2, cfg)
					signal.SetCandidate(finalMsg)
					if !IsDuplicate(ExtractSubject(finalMsg), recent) {
						msg = finalMsg
						success = true
					} else {
						candidate = finalMsg
					}
				} else {
					if cause != nil { lastCause = cause }
					if msg2 != ""   { candidate = msg2 }
				}
			}
		}
		if !success {
			return Result{}, &RescueError{
				Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
				Candidate: candidate, Cause: lastCause,
			}
		}
	}
```

When porting into runPipeline, prefix package-qualified calls with `generate.` /
`prompt.` / `git.` (runPipeline already does this — e.g. `generate.FinalizeMessage`,
`prompt.BuildUserPayload`, `git.EstimateTokens`). `Run` becomes `generate.Run`,
`FinalizeMessage` → `generate.FinalizeMessage`, `IsDuplicate`/`ExtractSubject` →
`generate.IsDuplicate`/`generate.ExtractSubject`, `RescueError` → `generate.RescueError`.

---

## 6. Dry-run success path & early-return point

After the gate + rescue, runPipeline runs the FR-E1 editor gate, then branches on `dryRun`
(`stagehand.go:565–575`):

```go
	// ---- Dry-run success: skip commit-tree/update-ref. ----
	if dryRun {
		signal.ClearSnapshot() // disarm — no rescue on dry-run success
		return Result{
			CommitSHA: "",
			Subject:   generate.ExtractSubject(msg),
			Message:   msg,
			Provider:  deps.Manifest.Name,
			Model:     model,
		}, nil
	}
```

**Key implication for the multi-turn fix:** the gate only ever sets `msg`/`success` — it does
**not** touch `dryRun`, `CommitTree`, or `UpdateRefCAS`. So **the dry-run success path needs no
change.** A multi-turn win on dry-run flows naturally: gate sets `success=true`+`msg`, the loop
falls through to the editor gate, then the `if dryRun` early-return fires exactly as today. ✅
The same is true on the commit path — the gate is a pure "fill `msg`/`success`" transformation;
all downstream plumbing is shared. **This is why the gate can be lifted verbatim.**

The commit tail (`stagehand.go:577–610`) is untouched by the fix: `CommitTree` (581),
`signal.RestoreDefault()` (592), `UpdateRefCAS` (595), `CASError` mapping (596–600),
`signal.ClearSnapshot()` (602), final `return Result{CommitSHA: newSHA, …}` (604).

---

## 7. RescueError type & user-facing message

`internal/generate/generate.go:84–101` (type alias re-exported as `pkg/stagehand.RescueError`):

```go
type RescueError struct {
	Kind      error  // ErrTimeout or ErrRescue — Unwrap() returns this
	TreeSHA   string // frozen snapshot (always non-empty post-WriteTree)
	ParentSHA string // "" on root commit
	Candidate string // last generated message ("" if none)
	Cause     error  // context.DeadlineExceeded / *exec.ExitError / nil
}
func (e *RescueError) Error() string {
	switch e.Kind {
	case ErrTimeout:
		return "stagehand: generation timed out after the snapshot was taken"
	default:
		return "stagehand: commit generation failed after retries"
	}
}
func (e *RescueError) Unwrap() error { return e.Kind }
```

- Sentinel vars: `ErrTimeout = "stagehand: generation timed out"` (62),
  `ErrRescue = "stagehand: commit generation failed after retries"` (67).
- **User-facing block** is NOT `Error()` — the CLI renders `generate.FormatRescue(treeSHA,
  parentSHA, candidate)` (rescue.go:43). The visible banner is `"❌ Commit generation failed.\n"`
  + the `git commit-tree … | xargs git update-ref HEAD` recipe, with the optional candidate note
  appended. The gate does not change any of this — on multi-turn failure the existing
  `return … &RescueError{Cause: cause}` (where `cause` came from `generate.Run`) flows to the
  same FormatRescue path. So the multi-turn fix produces **byte-identical rescue UX** (FR-T7).

---

## 8. runPipeline loop vs. CommitStaged loop — exact diff

Structurally the two loops are **identical except for two things**:

### Difference A — `payload` scoping (the blocker)
| | CommitStaged (generate.go) | runPipeline (stagehand.go) |
|---|---|---|
| Decl | `var payload string` **before** loop (226) | `payload := …` **inside** loop (490) |
| Survives loop? | ✅ yes → gate reads it (311) | ❌ no → gate cannot see it |

### Difference B — the FR-T1 multi-turn gate itself
| | CommitStaged (generate.go) | runPipeline (stagehand.go) |
|---|---|---|
| Present? | ✅ 290–374 | ❌ absent |
| `if !success { return …RescueError }` | nested under a second `if !success {}` (369) | bare return (544) |

### Everything else is identical (byte-for-byte mirror):
- Loop bound `for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++` (229 vs 489).
- Payload build + `parseFail` prepend (231 vs 490–492).
- `Render` (235 vs 495), `provider.Execute` (240 vs 500).
- Timeout/Canceled immediate-rescue arms (243–260 vs 502–513) — **identical**.
- `ParseOutput` (263 vs 518), `FinalizeMessage` (267 vs 527), `signal.SetCandidate` (268 vs 528).
- Dedupe branch (270–279 vs 530–539).
- Success set+break (281–283 vs 541–542).

(The only other cosmetic differences: CommitStaged's `Render` error wraps as
`"commit staged: render: %w"`, runPipeline as `"render: %w"`; CommitStaged sets
`lock.SetSnapshot` at step 4 (runPipeline does not, since `lock` is a CommitStaged-only
no-op-fast-path concern and runPipeline has no lock). Neither affects the gate.)

**Therefore the fix = Difference A (hoist `payload`) + Difference B (insert gate). Nothing else.**

---

## 9. Helper functions runPipeline calls (and one export constraint)

| Helper | Exported? | Location | Notes for the fix |
|---|---|---|---|
| `prompt.BuildUserPayload(diff, ctx, rejected)` | ✅ | internal/prompt | loop body + gate FR-T12 rebuild |
| `deps.Manifest.Render(model, sys, payload, reasoning)` | ✅ | provider | one-shot render |
| `provider.Execute(ctx, spec, timeout, verbose)` | ✅ | internal/provider | one-shot exec |
| `provider.ParseOutput(out, manifest)` | ✅ | internal/provider | parse |
| `generate.FinalizeMessage`, `generate.ExtractSubject`, `generate.IsDuplicate` | ✅ | internal/generate | dedupe/finalize |
| `generate.EditMessage` | ✅ | internal/generate | FR-E1 editor gate (post-loop) |
| `git.EstimateTokens(s)` | ✅ | internal/git/tokens.go:25 | gate **cond (b)** — works as-is |
| `deps.Verbose.VerboseWarn(msg)` | ✅ | internal/ui/verbose.go:103 | FR-T11 verbose line |
| **`generate.Run(ctx, deps, cfg, manifest, sysPrompt, payload, model, reasoning)`** | ✅ | multiturn.go:136 | **the multi-turn transport — callable from runPipeline** |
| **`chunkPayload(payload, chunkTokens)`** | ❌ **unexported** | multiturn.go:52 | used by CommitStaged **only** for the progress-message turn count (333) |

### ⚠️ The ONE export constraint (decision needed for the fix)

The gate's condition (b) (`git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens`) and the
`generate.Run` call **do not** need `chunkPayload`. `chunkPayload` is used in exactly **one**
spot (generate.go:333) to compute `turns` for the `fmt.Fprintf(os.Stderr, "↳ falling back to
multi-turn: %d turns, ~%dm total\n", turns, totalMin)` progress line.

`pkg/stagehand` cannot call `chunkPayload` (it is unexported in `internal/generate`). Three
options for the implementer (pick one; all keep scope narrow):

1. **Export a tiny counter** — add `func ChunkCount(payload string, chunkTokens int) int`
   (or export `chunkPayload` as `ChunkPayload`) in `multiturn.go`, mirroring the existing
   behavior; use it in runPipeline's progress line. Smallest internal change; keeps the user
   message faithful. (If `chunkPayload` is exported, CommitStaged can be left using the lower-
   case alias or switched too — either is safe.)
2. **Drop the turns/total-min progress line in runPipeline** only — emit just the
   `VerboseWarn("one-shot exhausted → multi-turn fallback")` line (which needs no `chunkPayload`).
   Zero internal-package change; slightly less informative stderr.
3. **Reuse `generate.Run`'s own logging** — `Run` already logs per-turn verbose via
   `deps.Verbose`; skip the pre-flight `turns` estimate entirely.

**Recommendation:** Option 1 (export `chunkPayload` or add `ChunkCount`) — it is the only way
to give runPipeline byte-identical UX to CommitStaged and is a trivial, well-contained export.
The progress line is the *sole* external-visible behavioral delta between the two options.

---

## 10. Precise insertion point & edit recipe

**Edit 1 — hoist `payload` (`stagehand.go`, the var block ~484–487).**

Before:
```go
	var rejected []string
	var candidate, msg string
	var parseFail, success bool
	var lastCause error
```
After (add one line):
```go
	var rejected []string
	var candidate, msg string
	var parseFail, success bool
	var lastCause error
	var payload string // hoisted: survives the loop for the FR-T1 multi-turn gate (mirrors CommitStaged)
```

**Edit 2 — loop body, switch `:=` to `=` (`stagehand.go:490`).**

Before:
```go
		payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
```
After:
```go
		payload = prompt.BuildUserPayload(diff, cfg.Context, rejected) // `=` not `:=` (payload hoisted above)
```

**Edit 3 — insert the gate (`stagehand.go:543–548`).**

Before:
```go
	if !success {
		return Result{}, &generate.RescueError{
			Kind: generate.ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
			Candidate: candidate, Cause: lastCause,
		}
	}
```
After (wrap the return; paste the gate from §5 with `generate.`/`prompt.`/`git.` prefixes;
use `generate.Run`; resolve the `chunkPayload`/`turns` line per §9 option 1–3):
```go
	if !success {
		// ---- FR-T1 multi-turn fallback trigger gate (PRD §9.24) — ported from CommitStaged (generate.go:290-374). ----
		if cfg.MultiTurnFallback &&
			resolved.SessionMode != nil && *resolved.SessionMode == "append" {

			mtPayload := payload // last one-shot payload (hoisted in Edit 1)
			if cfg.TokenLimit != 0 {
				fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
					MaxDiffBytes:        cfg.MaxDiffBytes,
					MaxMDLines:          cfg.MaxMdLines,
					BinaryExtensions:    cfg.BinaryExtensions,
					Excludes:            deps.Excludes,
					TokenLimit:          0, // FR-T12: multi-turn ignores token_limit
					DiffContext:         cfg.DiffContextValue(),
					PromptReserveTokens: 0,
				})
				if derr == nil {
					mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
				}
			}

			if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {
				// (turns/progress line — see §9; needs chunkPayload export OR a ChunkCount helper)
				deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")

				msg2, ok2, cause := generate.Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)

				if cause == nil && ok2 {
					finalMsg := generate.FinalizeMessage(msg2, cfg)
					signal.SetCandidate(finalMsg)
					if !generate.IsDuplicate(generate.ExtractSubject(finalMsg), recent) {
						msg = finalMsg
						success = true
					} else {
						candidate = finalMsg
					}
				} else {
					if cause != nil {
						lastCause = cause
					}
					if msg2 != "" {
						candidate = msg2
					}
				}
			}
		}
		if !success {
			return Result{}, &generate.RescueError{
				Kind: generate.ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
				Candidate: candidate, Cause: lastCause,
			}
		}
	}
```

`recent` is in scope (declared `stagehand.go:464` as `var recent []string`, populated when
`!isUnborn`). All gate identifiers confirmed in scope per §4. Imports `fmt`, `os`, `time` already
present in `stagehand.go` (lines 5–7) — needed only if the progress line is kept.

---

## 11. Risks & open questions

1. **`chunkPayload` export (§9)** — the only design decision the implementer must make. Default
   recommendation: export it (or add `ChunkCount`). If dropped (option 2), the stderr progress
   line is omitted on the runPipeline path only — minor UX divergence, no correctness impact.
2. **`recent` nil-safety** — on an unborn repo `recent == nil`; `generate.IsDuplicate` already
   handles nil (it's the same call CommitStaged makes), so no extra guard.
3. **`signal.SetCandidate` / `signal.ClearSnapshot`** — already called on the dry-run success
   path (566) and commit success path (602); the gate calls `signal.SetCandidate(finalMsg)`
   exactly as CommitStaged does, so signal state stays consistent. On a multi-turn *failure*
   the snapshot stays armed → existing §18.4 rescue signal handler sees it. Identical to
   CommitStaged. ✅
4. **`dryRun` + multi-turn interaction** — verified in §6: the gate is purely `msg`/`success`-
   setting; dry-run early-return at line 565 fires correctly on a multi-turn win, and the
   dangling-tree-in-dry-run note (stagehand.go:437 comment) still holds (tree is intentional
   and harmless since commit-tree is skipped). No new dangling-tree concern.
5. **Tests** — `pkg/stagehand/stagehand_test.go` has DryRun/SystemExtra tests (231–810) but
   **none** assert multi-turn behavior on runPipeline. New tests should mirror
   `TestCommitStaged_MultiTurnFallbackSuccess` (generate_test.go:868) and
   `TestCommitStaged_MultiTurnSkipped_NonAppend` (895) on the `GenerateCommit` DryRun +
   SystemExtra paths. The existing `appendScriptManifest` helper + stub provider infra in
   `pkg/stagehand/stagehand_test.go` is reusable.

---

## 12. Start Here

Open `pkg/stagehand/stagehand.go` and apply the three edits in §10 (hoist `payload` at 487,
switch 490 to `=`, insert the gate at 543). Keep the canonical gate text open side-by-side at
`internal/generate/generate.go:290–374`. Resolve the `chunkPayload` export (§9, option 1) in
`internal/generate/multiturn.go:52` first if you want byte-identical UX; otherwise drop the
`turns` progress line. Then add DryRun/SystemExtra multi-turn tests mirroring
`internal/generate/generate_test.go:868/895/918`.
