# P1.M3.T1.S2 — Design Decisions (FR-T1 multi-turn gate in hook.Run, preserving FR-H5)

Ground truth read before writing this note:
- **Bug-Fix PRD §h2.0/§h2.3** (Issue 2: the git hook exec path does not get the multi-turn fallback) +
  §h3.1 (Issue 3: per-chunk token estimate) + §h3.3 (Issue 4: payload inconsistency).
- **The S1 CONTRACT** (P1.M3.T1S1/PRP.md): binds `resolved := deps.Manifest.Resolve()` (exposes
  `resolved.SessionMode`) + hoists `var payload string` (survives the loop) + L158 `:=`→`=`. Pure refactor,
  no behavioral change. S2 consumes `resolved` + the hoisted `payload`.
- **The CANONICAL reference gate** — `internal/generate/generate.go:300-360` (CommitStaged's FR-T1 gate),
  read in FULL. It already carries the Issue 3 fix (progress line `"↳ falling back to multi-turn: %d turns
  (chunks of ~%d tokens), ~%dm total\n"`) and the Issue 4 fix (`mtPayload := prompt.BuildUserPayload(diff,
  cfg.Context, rejected)` — from `diff`, NOT the one-shot `payload`).
- **internal/hook/exec.go** (read in FULL): `Run` — the loop (L157-205) + the exhaustion return
  (`return fmt.Errorf("stagecoach: hook generation failed after %d retries", cfg.MaxDuplicateRetries)` — the
  INSERTION POINT). Imports: context/errors/fmt/os/strings + config/generate/git/prompt/provider. **NO `time`**
  (confirmed).
- **internal/generate/multiturn.go**: `ChunkCount(payload, chunkTokens) int` = `len(chunkPayload(...))` (L96,
  EXPORTED); `Run(ctx, deps, cfg, manifest, sysPrompt, payload, model, reasoning) (msg string, ok bool,
  cause error)` (L145, EXPORTED).
- **internal/cmd/hookexec.go**: `neverBlock` (L69-72) maps `hook.Run`'s non-ErrNoOp error → `exitcode.New(
  exitcode.Error, nil)` (exit 0 + one stderr line, or exit 1 if --strict); L137-138 `return neverBlock(rerr)`.
  This is the FR-H5 preservation: the exhaustion error → exit 0 + msg-file untouched.
- **Test patterns**: `internal/hook/exec_test.go` (stubtest.Build/Manifest/NewScript + initTempRepo + the
  never-block assertion `err!=nil && ReadFile(msgFile)==orig`); `internal/generate/generate_test.go:857`
  `appendScriptManifest` (local 5-line helper: NewScript + SessionMode=&"append"); generate's multiturn tests
  (the proven mock pattern).
- Verified at research time: `go build ./... && go test ./internal/hook/...` GREEN.

---

## §0 — Scope: insert the FR-T1 gate into hook.Run (exec.go) + add `time` + 4 tests

**S2 owns:** (a) add `"time"` to exec.go imports; (b) insert the FR-T1 gate between the loop and the
exhaustion return; (c) 4 new tests in exec_test.go (+ a local `appendScriptManifest` helper).

**Frozen / do NOT touch:** `internal/generate/*` (the canonical reference gate — S2 MIRRORS it, doesn't
edit it), `internal/cmd/hookexec.go` (the neverBlock closure — already maps the exhaustion error correctly),
`internal/provider/*`, `pkg/stagecoach/*`, the existing hook loop body + never-block returns. No conflict with
the parallel P1.M2.T1.S2 (pkg/stagecoach only) or S1 (exec.go refactor, which S2 builds on).

---

## §1 — The gate MIRRORS CommitStaged's reference gate, with 4 hook-specific adaptations

**Decision:** the gate is a faithful port of `generate.go:300-360` (which already has the Issue 3 + Issue 4
fixes), adapted to the hook's outcome model. The 4 adaptations:

1. **`generate.ChunkCount`** (EXPORTED, multiturn.go:96) replaces `len(chunkPayload(...))` (UNEXPORTED —
   unreachable from package hook). `turns := generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) + 1`.
2. **`generate.Run`** (EXPORTED) replaces the unexported `Run`. Same signature/return `(msg, ok, cause)`.
3. **Nil-guard `deps.Verbose`** for the `VerboseWarn` trigger (the hook's existing style: `if deps.Verbose
   != nil { deps.Verbose.VerboseRetry(...) }`). CommitStaged assumes Verbose non-nil; the hook does NOT
   (tests pass `generate.Deps{Git: g, Manifest: m}` with Verbose nil).
4. **NO `signal.SetCandidate`** (the hook has no rescue/signal — git owns the commit). On success →
   `return WriteMessageFile(msgFile, finalMsg)` (the ONLY write site). On ANY failure → fall through to the
   exhaustion return (NOT a RescueError — the hook returns the plain exhaustion error).

The mtPayload logic (Issue 4 + FR-T12) is copied VERBATIM from the reference: `mtPayload := prompt.
BuildUserPayload(diff, cfg.Context, rejected)` (from `diff`, not `payload`); if `cfg.TokenLimit != 0`,
re-capture via `deps.Git.StagedDiff(... TokenLimit: 0 ...)` and rebuild. The progress line (Issue 3) is
copied verbatim. See §2 for the full code.

---

## §2 — The gate code (copy-ready; inserted before the exhaustion return)

```go
// (after the loop, BEFORE `return fmt.Errorf("stagecoach: hook generation failed after %d retries", ...)`

// FR-T1 multi-turn fallback (PRD §9.24). The one-shot loop above exhausted; if the provider is
// multi-turn-capable (append session mode) and the untruncated payload exceeds one chunk, retry as a
// lossless N+1-turn session. On success the message is written to the msg-file (the ONLY write site);
// on ANY failure (turn error, empty final parse, or duplicate subject) fall through to the exhaustion
// error — the cmd layer's neverBlock maps that to exit 0 + an untouched msg-file (FR-H5 holds always).
if cfg.MultiTurnFallback &&
	resolved.SessionMode != nil && *resolved.SessionMode == "append" {

	// FR-T2/FR-T12 (Issue 4): mtPayload is ALWAYS rebuilt from the untruncated `diff` (NOT reused from the
	// one-shot `payload`, which may carry the retryInstr corrective preamble from a failed parse). When
	// token_limit is set (non-zero) the one-shot `diff` was truncated → RE-CAPTURE with TokenLimit=0.
	mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
	if cfg.TokenLimit != 0 {
		fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
			MaxDiffBytes:        cfg.MaxDiffBytes,
			MaxMDLines:          cfg.MaxMdLines,
			BinaryExtensions:    cfg.BinaryExtensions,
			Excludes:            deps.Excludes,
			TokenLimit:          0, // FR-T12: multi-turn ignores token_limit
			DiffContext:         cfg.DiffContextValue(),
			PromptReserveTokens: 0, // multi-turn chunking doesn't use the one-shot reserve
		})
		if derr == nil {
			mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
		}
		// On re-capture error, fall back to the (possibly-truncated) one-shot diff's payload (best-effort).
	}

	// Condition (b): the (now-untruncated) payload must exceed one chunk for multi-turn to help.
	if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {
		turns := generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) + 1 // N chunks + 1 final turn
		totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
		if totalMin < 1 {
			totalMin = 1
		}
		fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n",
			turns, cfg.MultiTurnChunkTokens, totalMin)

		if deps.Verbose != nil { // hook nil-guard (CommitStaged assumes non-nil; the hook does not)
			deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")
		}

		msg2, ok2, cause := generate.Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)

		if cause == nil && ok2 {
			finalMsg := generate.FinalizeMessage(msg2, cfg)
			if !generate.IsDuplicate(generate.ExtractSubject(finalMsg), recent) {
				return WriteMessageFile(msgFile, finalMsg) // SUCCESS — the ONLY write site (FR-H4)
			}
			// Duplicate subject → fall through to exhaustion (FR-H5: exit 0, msg-file untouched).
		}
		// cause != nil (turn error/timeout) OR ok2==false (final parse empty) OR duplicate → fall through.
	}
}
return fmt.Errorf("stagecoach: hook generation failed after %d retries", cfg.MaxDuplicateRetries)
```

---

## §3 — FR-H5 never-block holds in EVERY gate outcome (the load-bearing guarantee)

**Decision:** the gate writes the msg-file ONLY on full success (`cause==nil && ok2 && !duplicate`). Every
other outcome falls through to the existing exhaustion error return, which the cmd layer's `neverBlock`
(hookexec.go:69-72, invoked at L137-138) maps to exit 0 + one stderr line + an UNTOUCHED msg-file (or exit 1
if `--strict`). The three failure outcomes:
- **cause != nil** (a multi-turn turn errored / timed out) → fall through → exhaustion error → neverBlock.
- **ok2 == false** (the final turn's output didn't parse to a message) → fall through → neverBlock.
- **duplicate subject** (the finalized message matches a recent subject) → fall through → neverBlock
  (one-shot parity: the hook does NOT rescue; git's commit proceeds with an empty editor).

No `signal.SetCandidate`, no `RescueError`, no candidate/msg-file mutation on failure. The msg-file is
byte-identical to its pre-`Run` content unless the gate returns from `WriteMessageFile`. (Test 2 pins this.)

---

## §4 — `time` import is REQUIRED (exec.go does NOT currently import it)

**Decision:** add `"time"` to exec.go's import block. The progress line computes `totalMin := int((cfg.
Timeout * time.Duration(turns)).Minutes())` — `cfg.Timeout` is a `time.Duration`, multiplied by
`time.Duration(turns)`. Without the import this won't compile. (Confirmed: exec.go's imports are context/
errors/fmt/os/strings + 5 internal — no `time`.) This is the ONLY import change.

---

## §5 — Variables in scope at the insertion point (all confirmed; S1 exposes `resolved` + `payload`)

After S1's refactor, at the point BEFORE the exhaustion return, these are all in function scope: `ctx`,
`deps`, `cfg`, `msgFile`, `sysPrompt` (L118), `diff` (L134), `recent` (L144), `msgModel`/`msgReasoning`
(L146), `resolved` (S1 Edit 1), `rejected` (L154), `payload` (S1 hoist — UNUSED by the gate, which rebuilds
mtPayload from `diff` per Issue 4; the hoist was the refactor prerequisite that S1 landed). The gate reads:
`cfg`, `resolved`, `diff`, `ctx`, `deps`, `rejected`, `msgModel`, `msgReasoning`, `sysPrompt`, `recent`,
`msgFile`. All present. (The hoisted `payload` is not read by the gate — Issue 4 made the gate rebuild from
`diff`; S1's hoist is still correct as the gate's structural prerequisite + future-proofing.)

---

## §6 — `deps.Verbose` nil-safety in the multi-turn tests

**Decision:** the gate nil-guards its own `VerboseWarn` (§1 adaptation 3). `generate.Run` delegates per-turn
verbose to `provider.Execute`, which nil-guards `deps.Verbose` (the existing hook one-shot loop passes Verbose
to `provider.Execute` with nil in `TestRun_StubExit1_NeverBlock` and does not panic ⇒ Execute nil-guards).
So `generate.Run` with `deps.Verbose == nil` is safe. The multi-turn tests MAY pass Verbose nil (matching the
existing hook test style) or non-nil (mirroring generate's multiturn tests); either is safe. Prefer non-nil
(a real `ui`) in the SUCCESS test to exercise the per-turn verbose path; nil is fine for the failure/skip
tests.

---

## §7 — Test plan (4 tests + a local `appendScriptManifest` helper)

**Decision:** add to `internal/hook/exec_test.go`. Mirror the existing hook test idiom (initTempRepo +
stubtest.Build + the never-block assertion `err!=nil && ReadFile(msgFile)==orig`) and generate's multiturn
mock pattern. Add a LOCAL `appendScriptManifest(t, bin, responses)` helper (replicate generate_test.go:857 —
`stubtest.NewScript(...)` + `m.SessionMode = &"append"`; it is NOT exported from generate, so the hook test
needs its own copy).

1. **TestRun_MultiTurnSuccess_WritesMessageFile** — large diff + `appendScriptManifest(bin, ["", "ok", "ok",
   "feat: multi-turn win"])` + cfg{MultiTurnFallback:true, MultiTurnChunkTokens:4, MaxDuplicateRetries:0,
   TokenLimit:0}. One-shot exhausts on script[0]="" → gate fires → generate.Run consumes ["ok","ok","feat:
   multi-turn win"] → final "feat: multi-turn win" → WriteMessageFile. Assert `err==nil` + msg-file content
   starts with "feat: multi-turn win".
2. **TestRun_MultiTurnFailure_NeverBlock** — large diff + an append-mode EXIT-1 stub (`m := stubtest.Manifest(
   bin, Options{Exit:1, Out:""}); m.SessionMode = &"append"`) + cfg{MultiTurnFallback:true,
   MultiTurnChunkTokens:4, MaxDuplicateRetries:0}. One-shot exhausts (exit 1) → gate fires → generate.Run
   fails (turn exit 1 → cause!=nil) → fall through → exhaustion error. Assert `err!=nil && err.Error()
   contains "hook generation failed"` + msg-file BYTE-IDENTICAL to pre-Run content.
3. **TestRun_MultiTurnSkipped_NonAppend** — large diff + `stubtest.NewScript(bin, [""])` (SessionMode nil —
   NOT append) + cfg{MultiTurnFallback:true, MultiTurnChunkTokens:4}. Gate's outer if (SessionMode=="append")
   is false → skip → existing exhaustion error. Assert `err!=nil` + exhaustion message.
4. **TestRun_MultiTurnSmallPayloadSkip** — TINY diff (1-char change) + `appendScriptManifest(bin, [""])` +
   cfg{MultiTurnFallback:true, MultiTurnChunkTokens: <large, e.g. 100000>}. Conditions (a,c,d) true but (b)
   `EstimateTokens(mtPayload) > chunkTokens` FALSE → skip → exhaustion error. Assert `err!=nil`.

---

## §8 — No new deps; go.mod UNCHANGED

**Decision:** the gate uses already-imported symbols (prompt, git, generate, fmt, os) + the one new import
(`time`, stdlib). No new external dep. `go mod tidy` is a no-op; `git diff --exit-code go.mod go.sum` empty.

---

## Summary table (the 8 calls at a glance)

| § | Decision | Source |
|---|----------|--------|
| 0 | Insert the gate into hook.Run (exec.go) + add `time` + 4 tests; generate.go/cmd frozen | contract |
| 1 | Mirror CommitStaged's gate (generate.go:300-360, with Issue 3/4 fixes); 4 hook adaptations | reference gate |
| 2 | Copy-ready gate code (ChunkCount, generate.Run, nil-guard Verbose, WriteMessageFile/fall-through) | §1 |
| 3 | FR-H5: write msg-file ONLY on success; every failure → exhaustion → cmd neverBlock → exit 0 + untouched | hookexec.go:69 |
| 4 | Add `"time"` import (exec.go lacks it; totalMin uses time.Duration) | exec.go imports |
| 5 | All gate-read variables in scope after S1 (resolved, diff, cfg, etc.) | S1 contract |
| 6 | Verbose nil-safe (Execute nil-guards; gate nil-guards VerboseWarn) | existing hook tests |
| 7 | 4 tests + local appendScriptManifest helper (replicate generate_test.go:857) | hook + generate test idioms |
| 8 | No new deps; go.mod UNCHANGED | stdlib `time` only |
