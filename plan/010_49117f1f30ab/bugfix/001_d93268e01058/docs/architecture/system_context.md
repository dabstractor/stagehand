# System Context — Bugfix 001 (Hook Execution on the Commit Path)

## Overview

This bugfix addresses 4 Major issues + 1 Minor test-quality gap in stagehand's
commit-path hook execution feature (v2.4, FR-V1–V8). The core mechanism works
(ordering, --no-verify scope, FR-V3 freeze, FR-V7 rescue, post-commit best-effort,
FR-V4 recursion prevention are all correct), but four divergences from documented
git parity need fixing.

## Affected Files

| File | Issues | Role |
|------|--------|------|
| `internal/hooks/runner.go` | 1, 2, 4 | Hooks runner core — argv, msg file format, empty guard |
| `internal/config/git.go` | 3 | Git-config layer — missing noVerify reader |
| `internal/config/config.go` | 3 | Doc comment key name |
| `internal/generate/generate.go` | 4 | CommitStaged — empty guard after hooks |
| `pkg/stagehand/stagehand.go` | 4 | runPipeline — empty guard after hooks |
| `internal/decompose/message.go` | 4 | publishCommit — empty guard after hooks |
| `internal/generate/finalize.go` | 4 | ErrEmptyMessage sentinel (existing, reused) |
| `internal/decompose/message_test.go` | 5 | Test assertion tightening |
| `internal/decompose/chain_test.go` | 5 | Test assertion tightening |
| `docs/cli.md` | 3 | Git-config key name fix |
| `docs/configuration.md` | 3 | Git-config key name fix |
| `internal/hooks/runner_test.go` | 1, 2 | New tests for argc + newline |

## Issue Summary

### Issue 1 (Major): prepare-commit-msg argc
- **Bug**: `runPrepareCommitMsg` passes `[]string{msgPath, ""}` (argc=2).
- **Fix**: Change to `[]string{msgPath}` (argc=1). Correct the false "VERIFIED argc=2" comments.
- **Location**: `internal/hooks/runner.go:195`, comments at lines 52, 178.

### Issue 2 (Major): Missing trailing newline in message file
- **Bug**: `msgFile.WriteString(finalMsg)` writes NO trailing newline.
- **Fix**: Ensure `finalMsg` ends with `\n` before writing (mirrors git's `strbuf_complete_line()`).
- **Location**: `internal/hooks/runner.go:103`.
- **Highest-impact bug**: corrupts Signed-off-by trailers and append-style hooks.

### Issue 3 (Major): Missing noVerify git-config reader + invalid key name
- **Bug A**: `loadGitConfig()` never queries `stagehand.noVerify` (grep returns 0 matches).
- **Bug B**: Docs say `stagehand.no_verify` (snake_case) but git rejects underscores in final key segment.
- **Fix A**: Add `gitConfigBool(repoDir, "stagehand.noVerify")` mirroring the `push` reader at git.go:174.
- **Fix B**: Update docs to `stagehand.noVerify` (camelCase). TOML file key stays `no_verify`.
- **Location**: `internal/config/git.go` (~after line 177), `docs/cli.md:44`, `docs/configuration.md:155`, `internal/config/config.go:130`.

### Issue 4 (Major): No empty-message guard after hooks
- **Bug**: After `RunCommitHooks` returns, all 3 callers (CommitStaged, runPipeline, publishCommit) pass the hook-adjusted msg to CommitTree with NO empty check.
- **Fix**: After `RunCommitHooks` returns, check if the finalized message is empty (after trimming) and abort (return `ErrEmptyMessage`, exit 1, mirroring `EditMessage`'s existing guard).
- **Location**: 3 sites — `generate.go:431`, `stagehand.go:~672`, `message.go:~233`.

### Issue 5 (Minor): Loose test assertions mask Issue 2
- **Bug**: 3 decompose hook tests use `strings.Contains(logMsg, "[HOOK-RAN]")` which passes even with the corruption from Issue 2.
- **Fix**: Tighten assertions to check line structure (e.g. `strings.Contains("\n"+logMsg+"\n", "\n[HOOK-RAN]\n")`).
- **Location**: `message_test.go:428`, `chain_test.go:649`, `chain_test.go:675`.

## Architectural Patterns to Follow

1. **Git-config key naming**: camelCase for multi-word keys (e.g. `autoStageAll`, `maxDiffBytes`). Never underscores in git-config keys.
2. **ErrEmptyMessage**: Non-rescue, bare error → exit 1 (same as EditMessage's existing path).
3. **overlay() only-true-propagates**: A git-config `noVerify=false` is a no-op (same as `push`); force-false via env/flag (DIRECT set).
4. **TDD**: Every subtask implies "write failing test → implement → pass test."
5. **Doc sync (Mode A)**: Per-file docs updated with the implementing subtask. Final task sweeps README/overview.

## Dependencies Between Issues

- Issue 5 (test tightening) **depends on** Issue 2 (newline fix) — the tightened assertions only make sense once the newline is present.
- Issues 1, 2, 3, 4 are **independent** of each other — they touch different code paths / files.
- Issue 4's `ErrEmptyMessage` reuse depends on the existing sentinel in `finalize.go` (already present).
