# S2 Verified Touchpoint Map — FR-T1 gate into runPipeline (P1.M2.T1.S2)

> Verified against the LIVE repo (module github.com/dustin/stagecoach, 2026-07-05). Research only.
> Authoritative gate code: `docs/architecture/resolution_strategy.md` ISSUE 1 Edit 3 (verbatim below).

## 1. S1's hoist is ALREADY applied in the live file

`pkg/stagecoach/stagecoach.go` runPipeline var block (~L488) already has:
`var payload string // hoisted: survives the loop for the FR-T1 multi-turn gate (mirrors CommitStaged)`
and the loop uses `payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)` (`=`, not `:=`).
So S2 starts from the post-S1 state. S2's gate does NOT actually read `payload` — it rebuilds
`mtPayload` from `diff` (Issue 4 fix, see §3). The hoist is structurally harmless (payload stays
function-scoped + used by the loop body); do NOT try to "use" the hoisted payload in the gate.

## 2. The insertion point (live, post-S1)

```go
	}   // end of generation+dedupe loop
	if !success {
		return Result{}, &generate.RescueError{
			Kind: generate.ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
			Candidate: candidate, Cause: lastCause,
		}
	}
```
S2 WRAPS this: the gate goes INSIDE `if !success { … }`, BEFORE the inner `if !success { return }`.
On a multi-turn win the gate sets `msg`/`success=true` → the inner `if !success` is false → falls
through to the (unchanged) dry-run success early-return / commit tail.

## 3. The authoritative gate code (resolution_strategy.md ISSUE 1 Edit 3 — port VERBATIM)

Mirrors CommitStaged (generate.go ~L290-374) with `generate.`/`prompt.`/`git.`/`signal.` prefixes
(runPipeline is package stagecoach). Uses `generate.ChunkCount` (NOT the unexported chunkPayload),
`generate.Run`, Issue 4 (`mtPayload` from `diff`), Issue 3 (chunk-tokens in the Fprintf):

```go
if !success {
	// ---- FR-T1 multi-turn fallback trigger gate (PRD §9.24) — ported from CommitStaged. ----
	if cfg.MultiTurnFallback &&
		resolved.SessionMode != nil && *resolved.SessionMode == "append" {

		mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected) // FR-T2/Issue4: rebuild from untruncated diff
		if cfg.TokenLimit != 0 { // FR-T12: re-capture with TokenLimit=0
			fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
				MaxDiffBytes: cfg.MaxDiffBytes, MaxMDLines: cfg.MaxMdLines,
				BinaryExtensions: cfg.BinaryExtensions, Excludes: deps.Excludes,
				TokenLimit: 0, DiffContext: cfg.DiffContextValue(), PromptReserveTokens: 0,
			})
			if derr == nil {
				mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
			}
		}

		if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens { // condition (b)
			turns := generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) + 1 // N chunks + 1 final
			totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
			if totalMin < 1 { totalMin = 1 }
			fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n",
				turns, cfg.MultiTurnChunkTokens, totalMin) // Issue 3 format
			deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")

			msg2, ok2, cause := generate.Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)
			if cause == nil && ok2 {
				finalMsg := generate.FinalizeMessage(msg2, cfg)
				signal.SetCandidate(finalMsg)
				if !generate.IsDuplicate(generate.ExtractSubject(finalMsg), recent) {
					msg = finalMsg
					success = true // multi-turn won → skip rescue
				} else {
					candidate = finalMsg // duplicate → rescue with finalized candidate
				}
			} else {
				if cause != nil { lastCause = cause }
				if msg2 != "" { candidate = msg2 }
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

## 4. ZERO new imports — all already present in stagecoach.go

Verified import block: `context errors fmt io os strings time` + `config decompose exclude generate git
prompt provider signal ui`. The gate uses `fmt`, `os`, `time`, `git`, `prompt`, `generate`, `signal` — ALL
already imported. Do NOT add imports (an unused import fails `go vet`). `decompose`/`exclude`/`ui`/etc. are
unchanged. go.mod/go.sum unchanged (no new dep).

## 5. Variables confirmed IN SCOPE at the insertion point (all verified in live runPipeline)

`ctx` `deps` `cfg` `resolved` (deps.Manifest.Resolve(), ~L470) `sysPrompt` (~L425) `msgModel`/`msgReasoning`
(~L474, from config.ResolveRoleModel("message", cfg)) `diff` (~L446, StagedDiff) `rejected` `recent` (~L464)
`candidate`/`msg`/`success`/`lastCause` (var block) `treeSHA`/`parentSHA`. NO new variables to introduce
upstream — the gate is self-contained.

## 6. Dry-run success path — NO change

The gate only sets `msg`/`success`. On a multi-turn win with `dryRun=true`, the existing
`if dryRun { signal.ClearSnapshot(); return Result{CommitSHA:"", Subject:..., Message: msg, ...}, nil }`
(~L565) fires correctly — `msg` now carries the multi-turn result. The commit tail is skipped. Byte-identical
to the one-shot dry-run success except `msg`'s origin. FR49 satisfied: dry-run now runs the FULL pipeline
incl. multi-turn.

## 7. Test infra — append-mode stub via TOML (appendScriptManifest is NOT in stagecoach_test.go)

`appendScriptManifest` exists ONLY in internal/generate (returns a direct provider.Manifest via
stubtest.NewScript + SessionMode=&"append"). pkg/stagecoach tests go through GenerateCommit → config.Load →
buildDeps (registry), so the stub is registered via repo-local `.stagecoach.toml` `[provider.stub]`.

VERIFIED good news: `RenderMultiTurn` is a METHOD on `provider.Manifest` (render.go:203, value receiver) and
`SessionMode *string \`toml:"session_mode"\`` (manifest.go:66) is a TOML field. So a TOML `[provider.stub]`
with `session_mode = "append"` produces a manifest that supports multi-turn through the registry path — NO
direct-manifest injection seam needed.

S2's tests EXTEND the existing `setupScriptedRepo`/`setupTestRepo` pattern (which write `[provider.stub]` +
`STAGECOACH_STUB_SCRIPT`/`STAGECOACH_STUB_COUNTER` env): add `session_mode = "append"` to the `[provider.stub]`
TOML block (a new helper, e.g. `setupScriptedAppendRepo`, or an option on the existing helper). The script's
call-varying responses: one-shot retries return empty/garbage (exhaust MaxDuplicateRetries) → multi-turn
turns return chunks → final turn returns the message. Mirror generate_multiturn_test.go: LARGE diff via a
`strings.Builder` loop (~60 lines) + `cfg.MultiTurnChunkTokens = 50` (tiny ⇒ N≥2) + `cfg.MultiTurnFallback = true`.

NOTE on the 4 required tests: they call `GenerateCommit(ctx, Options{Provider:"stub", DryRun:true})` after
`os.Chdir` into the scripted repo (mirror TestGenerateCommit_DryRun at stagecoach_test.go:232). Conditions:
(1) success — large diff + append + DryRun ⇒ exit 0, Subject set, CommitSHA empty; (2) skipped non-append —
SessionMode unset ⇒ gate skips ⇒ existing rescue; (3) small payload — tiny diff ⇒ condition (b) false ⇒
rescue; (4) mid-turn failure — stub exits 1 mid-Run ⇒ RescueError (non-zero). The repo's existing
call-varying script infra (`STAGECOACH_STUB_SCRIPT` + counter) drives the pass/fail per turn.
