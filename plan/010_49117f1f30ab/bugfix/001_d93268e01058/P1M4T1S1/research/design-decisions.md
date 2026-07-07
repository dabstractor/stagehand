# P1.M4.T1.S1 — Design Decisions & Research Notes

> Research backing `PRP.md`: a test-only change (Issue 5). Tighten 3 decompose hook tests from a substring
> `strings.Contains` check to a LINE-STRUCTURE check so they would catch a regression of the Issue 2
> trailing-newline fix (P1.M1.T2.S1, LANDED). No code, no setup changes, no negative-test changes.

## 0. Scope: 3 assertion edits across 2 test files. Test-only.

Edit ONLY the assertion condition in 3 test functions (and optionally tighten its error message). Touch
NOTHING else — not the test setup, not the hook installation, not the helpers, not the negative test:
- `internal/decompose/message_test.go` — `TestPublishCommit_PrepareCommitMsgAnnotates` (assertion at L428).
- `internal/decompose/chain_test.go` — `TestResolveArbiter_NullNewCommit_RunsHooks` (L649) +
  `TestResolveArbiter_TipAmend_RunsHooks` (L675).

Both files are white-box `package decompose` (they call the unexported `publishCommit`/`resolveArbiter`).

## 1. The assertion form: `strings.Contains("\n"+X+"\n", "\n[HOOK-RAN]\n")` (the task's recommendation)

Replace `strings.Contains(X, "[HOOK-RAN]")` (X = `logMsg` / `headMsg`) with
`strings.Contains("\n"+X+"\n", "\n[HOOK-RAN]\n")`. Why this is the right check:

- **FAILS on the Issue 2 corruption** `feat: add new[HOOK-RAN]` (no `\n` before the marker): the padded
  haystack `"\nfeat: add new[HOOK-RAN]\n"` does NOT contain `"\n[HOOK-RAN]\n"` → test fails. ✓ (This is the
  regression guard the task wants.)
- **PASSES on the fixed output** `feat: add new\n[HOOK-RAN]`: the padded haystack
  `"\nfeat: add new\n[HOOK-RAN]\n"` DOES contain `"\n[HOOK-RAN]\n"` (the `\n` before the marker is the one
  after "add new"; the `\n` after is the pad we appended) → test passes. ✓

Why pad BOTH sides:
- **Trailing `+"\n"`:** the helpers (`msgRunGit`/`msgGitOut` at message_test.go:51-58, `chnRunGit` at
  chain_test.go:52-58) `strings.TrimSpace` the git output, so the trailing `\n` git emits is STRIPPED. The
  marker is the LAST line of the (trimmed) message, so without the trailing pad there'd be no `\n` after
  `[HOOK-RAN]` and the check would fail spuriously. The task §1 confirms: "we check for a line break BEFORE
  the marker, not after" — the trailing pad restores the boundary TrimSpace removed.
- **Leading `"\n"+`:** handles the (defensive) case where the marker is the FIRST line. In these 3 tests the
  message always has a subject line first (`feat: add new` / the tip's message), so the marker is never
  first — but the leading pad makes the idiom correct unconditionally and matches the task's verbatim form.

## 2. The Issue 2 fix is LANDED → the tightened assertions PASS today; they'd FAIL on a regression

Verified at `internal/hooks/runner.go:107-110`: `if !strings.HasSuffix(finalMsg, "\n") { finalMsg += "\n" }`
before `msgFile.WriteString(finalMsg)`. So the message file fed to `prepare-commit-msg` ends with `\n`, the
hook's `echo '[HOOK-RAN]' >> "$1"` append lands on its own line, and `git log --format=%B` (TrimSpace'd)
yields `<subject>\n[HOOK-RAN]`. The tightened assertions pass against the current (fixed) code. If a future
change regresses the trailing newline, the message becomes `<subject>[HOOK-RAN]` and all 3 tests fail — the
guard the original `strings.Contains` could not provide (it passed on the corrupted form, which is exactly
why Issue 2 shipped undetected through this suite).

## 3. The 3 tests use the IDENTICAL hook; the message shape is uniform

All three install the same append hook:
- message_test.go:419 — `msgInstallHook(t, repo, "prepare-commit-msg", "#!/bin/sh\necho '[HOOK-RAN]' >> \"$1\"\n")`.
- chain_test.go:627 — `chnBuildChainWithHook` → `chnInstallHook(t, repo, "prepare-commit-msg",
  "#!/bin/sh\necho '[HOOK-RAN]' >> \"$1\"\n")` (used by both NullNewCommit and TipAmend).

So in every case the committed message is `<subject>\n[HOOK-RAN]` (post-fix) — the line-structure check is
correct for all three. (TipAmend may carry the marker twice if the chain-build also ran the hook, but
`strings.Contains` finds it as long as ONE occurrence is a standalone line — which it is.)

## 4. Do NOT touch the negative test, the setup, or the helpers

- `TestResolveArbiter_MidChain_SkipsHooks` (chain_test.go ~698-710) asserts the ABSENCE of `[HOOK-RAN]`
  (`if strings.Contains(msg, "[HOOK-RAN]") { t.Errorf(...) }`) in a loop over rebuilt commits — that is the
  §20.2 mid-chain-fidelity check and is CORRECT as-is. Leave it byte-identical.
- The test setup (repo init, chain build, stub manifest, `publishCommit`/`resolveArbiter` calls) and the
  hook installation are unchanged — only the positive-marker assertion condition changes.
- The helpers (`msgRunGit`/`msgGitOut`/`chnRunGit`/`msgInstallHook`/`chnInstallHook`/
  `chnBuildChainWithHook`) are unchanged.

## 5. No conflict with the parallel P1.M3.T1.S3

The running P1.M3.T1.S3 (Issue 4, empty-message guard) edits `internal/decompose/message.go` (`publishCommit`
CODE — inserts a guard after `RunCommitHooks`). It does NOT touch `message_test.go` or `chain_test.go` (its
PRP names no test functions in those files; it's a one-guard code insertion). So the two test files this task
edits are untouched by the parallel work. No overlap, no merge conflict. ✓ (S3 adds the empty-message guard;
this task tightens the EXISTING hook-annotation assertions — independent.)

## Sources
- `internal/decompose/message_test.go` L407-430 (the test + assertion L428) + L51-58 (`msgRunGit` TrimSpace).
- `internal/decompose/chain_test.go` L631-677 (the two tests + assertions L649/L675) + L52-58 (`chnRunGit`
  TrimSpace) + L621-627 (`chnBuildChainWithHook` — the hook body).
- `internal/decompose/chain_test.go` ~L698-710 (`TestResolveArbiter_MidChain_SkipsHooks` — the negative test
  to leave alone).
- `internal/hooks/runner.go:107-110` — confirms the Issue 2 fix (trailing newline) is LANDED.
- The Bug-Fix PRD §h2.3/h3.4 (Issue 5) + the task's recommended assertion form.
- `plan/010…/bugfix/001_d93268e01058/P1M3T1S3/PRP.md` — confirms the parallel task is code-only (message.go),
  no test-file overlap.
