# Research: Trailing Newline Fix (Issue 2) — P1.M1.T2.S1

> **Purpose:** Pin the exact, source-verified fix for the missing trailing newline in the hook message
> file (Issue 2 — Major), checked against the live codebase on 2026-07-06. Baseline GREEN; the bug is at
> runner.go ~line 103 (`msgFile.WriteString(finalMsg)` with no newline check). The prior PRP (S1, Issue 1
> argc fix) touches runner.go:195 + comments at :52/:178 — NOT the WriteString area → no conflict.

---

## 1. Environment & Baseline

| Item | Value / Evidence |
|---|---|
| Module | `github.com/dustin/stagehand`, `go 1.22` |
| Edit targets | `internal/hooks/runner.go` (newline guard before WriteString), `internal/hooks/runner_test.go` (new test) |
| Bug line | runner.go ~line 103: `msgFile.WriteString(finalMsg)` — NO trailing-newline check. |
| Baseline | `go test ./internal/hooks/` → **ok (2.125s)**. |
| Prior PRP (S1) | Issue 1 (argc fix): runner.go:195 argv + comments :52/:178. Does NOT touch the WriteString area → **no conflict**. |
| `strings` import | PRESENT (runner.go line 21). `strings.HasSuffix` available — NO new import. |

---

## 2. The Bug (Issue 2)

`RunCommitHooks` writes the message to a temp file at runner.go ~line 103 via
`msgFile.WriteString(finalMsg)` with NO trailing newline. Git's `strbuf_complete_line()` ensures
COMMIT_EDITMSG always ends with `\n`. A `prepare-commit-msg` hook that appends via
`echo "Signed-off-by: ..." >> "$1"` (no leading `\n`) produces `feat: changeSigned-off-by: ...`
(corrupted subject) instead of two clean lines.

**Why existing tests didn't catch it:** the existing `TestRunCommitHooks_CommitMsgAppends_Annotated`
(runner_test.go:170) uses `printf '\nSigned-off-by: ...\n' >> "$1"` — the LEADING `\n` in the printf
creates the line break EVEN WITHOUT the trailing-newline fix, masking the bug.

---

## 3. The Fix (one guard before WriteString)

Insert, immediately before the `msgFile.WriteString(finalMsg)` call:
```go
// git parity (strbuf_complete_line): git's COMMIT_EDITMSG always ends with \n so an append-style
// prepare-commit-msg/commit-msg hook (e.g. `echo "Signed-off-by: ..." >> "$1"`) starts on a new
// line, not concatenated onto the subject. Add a trailing \n if the message lacks one.
if !strings.HasSuffix(finalMsg, "\n") {
    finalMsg += "\n"
}
```

**Why this is safe for the return value:** `finalMsg` is reassigned at the read-back
(runner.go ~line 131: `finalMsg = stripCommentLines(string(data), commentChar)`).
`stripCommentLines` does `strings.TrimRight(b.String(), "\n")`, so the trailing `\n` is CONSUMED
on read-back — the returned `finalMsg` does NOT carry a spurious trailing newline. The mutation
only affects the file content fed to the hooks.

---

## 4. The Test (mirror TestRunCommitHooks_CommitMsgAppends_Annotated but with `echo` not `printf '\n'`)

```go
func TestRunCommitHooks_PrepareCommitMsg_AppendOnNewLine(t *testing.T) {
    repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)
    // echo (NOT printf '\n...') appends WITHOUT a leading \n — exercises the trailing-newline bug.
    installHook(t, repo, "prepare-commit-msg", `echo "Signed-off-by: Dev <dev@example.com>" >> "$1"`)
    _, finalMsg, err := RunCommitHooks(context.Background(), g, defaultCfg(), snapshotTree, parentSHA,
        "feat: change", HookOpts{})
    if err != nil {
        t.Fatalf("RunCommitHooks err = %v, want nil", err)
    }
    // Signed-off-by MUST be on a separate line (preceded by \n), NOT concatenated onto the subject.
    if !strings.Contains(finalMsg, "\nSigned-off-by:") {
        t.Errorf("Signed-off-by not on a separate line (Issue 2 regression); finalMsg=%q", finalMsg)
    }
    if !strings.HasPrefix(finalMsg, "feat: change") {
        t.Errorf("finalMsg = %q, want the original subject preserved", finalMsg)
    }
}
```

**Why this FAILS before the fix and PASSES after:**
- Before: the file is `feat: change` (no `\n`). `echo "..." >> "$1"` appends → `feat: changeSigned-off-by: Dev <dev@example.com>\n`. stripCommentLines reads one line → `feat: changeSigned-off-by: Dev <dev@example.com>`. `Contains("\nSigned-off-by:")` FAILS (no `\n` before Signed-off-by).
- After: the file is `feat: change\n`. `echo "..." >> "$1"` → `feat: change\nSigned-off-by: Dev <dev@example.com>\n`. stripCommentLines reads two lines → `feat: change\nSigned-off-by: Dev <dev@example.com>`. `Contains("\nSigned-off-by:")` PASSES.

---

## 5. Decisions Log

| # | Question | Decision | Rationale |
|---|---|---|---|
| D1 | Where to add the guard? | Immediately BEFORE `msgFile.WriteString(finalMsg)`, mutating `finalMsg`. | The guard ensures the FILE content has the trailing \n. The mutation doesn't leak: finalMsg is reassigned at the read-back (stripCommentLines does TrimRight("\n")). |
| D2 | Test append mechanism? | `echo "..." >> "$1"` (NOT `printf '\n...'`). | echo appends WITHOUT a leading \n — exercises the bug. The existing tests use printf '\n...' which masks it. |
| D3 | Test assertion? | `strings.Contains(finalMsg, "\nSigned-off-by:")` — the trailer on a SEPARATE line. | NOT `strings.Contains(finalMsg, "Signed-off-by:")` (passes even when corrupted). The `\n` prefix is the precise line-boundary check. |
| D4 | Return value safety? | Safe — stripCommentLines TrimRight("\n") consumes the added \n. | The returned finalMsg does NOT carry a spurious trailing \n. Verified via the source. |
| D5 | Scope? | ONLY runner.go (guard + comment) + runner_test.go (1 test). | Issue 2 only. Issue 1 (argc) is S1; Issue 3 (no_verify) is P1.M2; Issue 4 (empty msg) is P1.M3. |
