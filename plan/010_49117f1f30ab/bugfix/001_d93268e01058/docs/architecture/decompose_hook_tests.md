# Decompose/Arbiter Hook Test Assertions (Issue 5)

## The Bug

Three decompose hook tests use `strings.Contains(logMsg, "[HOOK-RAN]")` which passes even when the
message is corrupted by Issue 2 (trailing newline bug → marker concatenated onto subject line).

## Affected Tests

### 1. `TestPublishCommit_PrepareCommitMsgAnnotates` — message_test.go:407-429
```go
msgInstallHook(t, repo, "prepare-commit-msg", "#!/bin/sh\necho '[HOOK-RAN]' >> \"$1\"\n")
...
logMsg := msgGitOut(t, repo, "log", "--format=%B", "-n1")
if !strings.Contains(logMsg, "[HOOK-RAN]") {
    t.Errorf("committed message = %q, want it to carry the [HOOK-RAN] append (hooks ran)", logMsg)
}
```

### 2. `TestResolveArbiter_NullNewCommit_RunsHooks` — chain_test.go:631-652
```go
headMsg := chnRunGit(t, repo, "log", "--format=%B", "-1")
if !strings.Contains(headMsg, "[HOOK-RAN]") {
    t.Errorf("HEAD message = %q, want it to carry [HOOK-RAN] (resolveNewCommit ran hooks)", headMsg)
}
```

### 3. `TestResolveArbiter_TipAmend_RunsHooks` — chain_test.go:654-677
```go
headMsg := chnRunGit(t, repo, "log", "--format=%B", "-1")
if !strings.Contains(headMsg, "[HOOK-RAN]") {
    t.Errorf("amended tip message = %q, want it to carry [HOOK-RAN] (resolveTipAmend ran hooks — amend parity)", headMsg)
}
```

## Why They Mask Issue 2

The hook body is always `echo '[HOOK-RAN]' >> "$1"` — an append-style hook. When the message file
has no trailing newline (Issue 2 bug), the marker concatenates onto the subject line:
```
feat: add new[HOOK-RAN]   ← CORRUPTED (Issue 2 bug active)
```
But `strings.Contains(logMsg, "[HOOK-RAN]")` returns true regardless → test passes.

With the Issue 2 fix, the marker lands on a separate line:
```
feat: add new
[HOOK-RAN]   ← CORRECT
```

## Additional Masking: TrimSpace in helpers

Both read-back helpers (`msgGitOut`/`msgRunGit` and `chnRunGit`) use `strings.TrimSpace(string(out))`:
```go
func msgRunGit(t *testing.T, dir string, args ...string) string {
    ...
    return strings.TrimSpace(string(out))
}
```
This strips trailing newlines from `git log --format=%B` output, making any trailing-newline
discrepancy invisible.

## The Fix (after Issue 2 is fixed)

Tighten assertions to check that `[HOOK-RAN]` lands on a **separate line** (the git-parity outcome):

**Option A** — wrap and check boundary:
```go
if !strings.Contains("\n"+logMsg+"\n", "\n[HOOK-RAN]") {
    t.Errorf("committed message = %q, want [HOOK-RAN] on a SEPARATE line (trailing-newline parity)", logMsg)
}
```

**Option B** — exact suffix:
```go
if !strings.HasSuffix(logMsg, "\n[HOOK-RAN]") && !strings.HasSuffix(logMsg, "\n[HOOK-RAN]\n") {
    t.Errorf("committed message = %q, want [HOOK-RAN] on its own line", logMsg)
}
```

**Option C** — split into lines and check:
```go
lines := strings.Split(logMsg, "\n")
found := false
for _, line := range lines {
    if line == "[HOOK-RAN]" {
        found = true
        break
    }
}
if !found {
    t.Errorf("committed message = %q, want [HOOK-RAN] as a standalone line", logMsg)
}
```

Any of these would fail with the Issue 2 corruption (`feat: add new[HOOK-RAN]` has no line break).

## Install Helpers

### `msgInstallHook` — message_test.go:407-417
### `chnInstallHook` — chain_test.go:612-622
Both are byte-identical (mkdir hooks dir, write file at 0755 for exec bit). They mirror the idiom
in `internal/hooks/runner_test.go`'s `installHook`.

## Negative Test (already correct)

`TestResolveArbiter_MidChain_SkipsHooks` — chain_test.go:698-710 — asserts `[HOOK-RAN]` is NOT present
in mid-chain rebuilt commits. This test is correct as-is.

## Dependencies
- **MUST run AFTER Issue 2 is fixed.** The tightened assertions only pass once the trailing newline
  fix ensures `[HOOK-RAN]` lands on its own line. Running before Issue 2 would cause test failures
  that are actually the bug being reproduced (not a test error).
