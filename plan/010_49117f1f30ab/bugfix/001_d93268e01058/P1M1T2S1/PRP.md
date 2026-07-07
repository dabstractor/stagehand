---
name: "P1.M1.T2.S1 — Add trailing newline to message file before prepare-commit-msg runs"
description: |
  One-guard bugfix (Issue 2 — Major): `RunCommitHooks` (runner.go ~line 103) writes the message to the
  hook temp file via `msgFile.WriteString(finalMsg)` with NO trailing newline. Git's
  `strbuf_complete_line()` ensures COMMIT_EDITMSG always ends with `\n`. A `prepare-commit-msg` hook that
  appends via `echo "..." >> "$1"` (no leading `\n`) corrupts the subject: `feat: changeSigned-off-by: ...`
  instead of two clean lines. Fix: before WriteString, add `if !strings.HasSuffix(finalMsg, "\n") {
  finalMsg += "\n" }` (mirrors git's strbuf_complete_line). The mutation is consumed by stripCommentLines's
  TrimRight("\n") on read-back — the returned finalMsg has no spurious trailing newline. Add a test using
  `echo` (NOT `printf '\n...'`) to exercise the bug. Baseline GREEN; strings already imported.
---

## Goal

**Feature Goal**: Restore git-parity for the hook message-file format: ensure the file fed to
`prepare-commit-msg` / `commit-msg` always ends with a trailing `\n` (mirroring git's
`strbuf_complete_line()`), so append-style hooks (`echo "Signed-off-by: ..." >> "$1"`) produce clean
multi-line messages instead of corrupting the subject line.

**Deliverable** (1 guard + 1 comment + 1 test):
1. `internal/hooks/runner.go` — insert a trailing-newline guard before `msgFile.WriteString(finalMsg)`
   (~line 103) + an in-source comment noting git parity (strbuf_complete_line).
2. `internal/hooks/runner_test.go` — add `TestRunCommitHooks_PrepareCommitMsg_AppendOnNewLine` using
   `echo` (not `printf '\n...'`) to exercise the bug.

**Success Definition**: A `prepare-commit-msg` hook that appends via `echo "..." >> "$1"` produces a
message with the appended content on a SEPARATE line (not concatenated onto the subject). The returned
`finalMsg` has NO spurious trailing newline (stripCommentLines consumes it). `go build/vet/gofmt` clean;
`go test -race ./...` green. The existing hook tests still pass.

## User Persona

**Target User**: The user with a `prepare-commit-msg` or `commit-msg` hook that appends a trailer
(`Signed-off-by`, branch name, ticket ref) via `echo "..." >> "$1"` — the Linux-kernel `Signed-off-by`
pattern, corporate contribution agreements, `git commit -s` parity.

**Use Case**: `stagecoach` generates `feat: change`; the user's `prepare-commit-msg` hook appends
`Signed-off-by: Dev <dev@example.com>`. Before the fix: the commit message is
`feat: changeSigned-off-by: Dev <dev@example.com>` (corrupted). After: two clean lines.

**Pain Points Addressed**: Eliminates the silent subject-line corruption for the most common
commit-message shape (single-line) with the most common append-style hook pattern (`echo >> "$1"`).

## Why

- **The HIGHEST-IMPACT finding in the hooks bug report.** Issue 2 silently produces wrong commit messages for a mainstream workflow (single-line message + `Signed-off-by`/branch-name/ticket-ref append hook). The corruption is invisible to the user until they inspect `git log`.
- **Git-parity guarantee.** Git's `strbuf_complete_line()` (in `commit.c` / `builtin/commit.c`) ensures `COMMIT_EDITMSG` always ends with `\n`. The hex-dump comparison confirmed: git writes `feat: change\n` (`0a` trailer); stagecoach writes `feat: change` (no trailer).
- **One-line fix, zero behavioral risk.** The guard adds a `\n` only when the message lacks one. `stripCommentLines`'s `TrimRight("\n")` on read-back consumes it, so the returned `finalMsg` is byte-identical to the pre-fix return for messages that already end with `\n`, and has NO spurious trailing `\n` for messages that didn't. The existing `TestRunCommitHooks_CommitMsgAppends_Annotated` and `TestRunCommitHooks_PrepareCommitMsg_Foreign_AnnotationReadBack` still pass.
- **The existing tests masked the bug** by using `printf '\n...' >> "$1"` (leading `\n` creates the break without the fix). The new test uses `echo` (no leading `\n`) to catch the regression.

## What

A trailing-newline guard before the `msgFile.WriteString(finalMsg)` call + an in-source comment + a test
using `echo` to exercise the bug. No signature change, no caller change, no other package.

### Success Criteria

- [ ] runner.go has `if !strings.HasSuffix(finalMsg, "\n") { finalMsg += "\n" }` before `msgFile.WriteString(finalMsg)`.
- [ ] An in-source comment at the guard cites git parity (strbuf_complete_line).
- [ ] The new test uses `echo "..." >> "$1"` (NOT `printf '\n...'`) and asserts `strings.Contains(finalMsg, "\nSigned-off-by:")`.
- [ ] The existing `TestRunCommitHooks_CommitMsgAppends_Annotated` + `TestRunCommitHooks_PrepareCommitMsg_Foreign_AnnotationReadBack` still pass.
- [ ] The returned `finalMsg` has NO spurious trailing `\n` (stripCommentLines consumes it).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim buggy WriteString block (runner.go ~line 103), the
exact guard to insert, the stripCommentLines read-back logic (proving the mutation is consumed), the
RunCommitHooks signature, the test helpers (`primeRunnerRepo`/`installHook`/`defaultCfg`), the exact test
body (copy-paste-ready with the FAILS-before/PASSES-after proof), and the reason existing tests masked the
bug. `strings` is confirmed imported. No inference.

### Documentation & References

```yaml
# MUST READ — the bug report
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/TEST_RESULTS.md
  why: "Issue 2: the exact bug (WriteString with no trailing \n), the side-by-side git comparison (hex dump), the corrupted-output example ('feat: changeSigned-off-by: ...'), the prescribed fix (HasSuffix guard before WriteString), and the test prescription."
  critical: "States this is 'the single highest-impact finding' and prescribes the exact fix: `if !strings.HasSuffix(finalMsg, \"\\n\") { finalMsg += \"\\n\" }` before writing. Notes the existing tests masked it via printf '\\n...'."

- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/P1M1T2S1/research/trailing_newline_fix_notes.md
  why: "EMPIRICALLY VERIFIED (live code, 2026-07-06): the bug at runner.go ~line 103; strings imported (line 21); stripCommentLines does TrimRight('\\n') (consuming the guard); the RunCommitHooks signature; the test helpers; the FAILS-before/PASSES-after proof; decisions D1–D5. READ THIS FIRST."
  critical: "§3 (the fix + why it's safe for the return value) and §4 (the test + why printf '\\n...' masked the bug) are the core. §2 explains why existing tests didn't catch it."

# The edit target
- file: internal/hooks/runner.go
  why: "EDIT (1 guard + 1 comment, before WriteString ~line 103). The guard: `if !strings.HasSuffix(finalMsg, \"\\n\") { finalMsg += \"\\n\" }`. The WriteString block: `if _, werr := msgFile.WriteString(finalMsg); werr != nil { ... }`. Insert the guard IMMEDIATELY before it."
  pattern: "The guard mutates `finalMsg` (a local string) before writing. `finalMsg` is later reassigned at the read-back (~line 131: `finalMsg = stripCommentLines(string(data), commentChar)`), so the mutation only affects the file content. stripCommentLines does TrimRight('\\n'), consuming the added \\n — the returned finalMsg has no spurious trailing \\n."
  gotcha: "`strings` is ALREADY imported (line 21) — no new import. Do NOT touch the WriteString error-handling block, the hook invocation sequence, or the read-back/stripCommentLines logic."

- file: internal/hooks/runner_test.go
  why: "EDIT (1 new test). Mirror `TestRunCommitHooks_CommitMsgAppends_Annotated` (line 170) but use `echo` (NOT `printf '\\n...'`) and install on `prepare-commit-msg` (not `commit-msg`). The test uses `primeRunnerRepo(t)` + `installHook(t, repo, name, body)` + `RunCommitHooks(ctx, g, cfg, snapshotTree, parentSHA, msg, HookOpts{})`."
  pattern: "Existing test shape: `repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)` → `installHook(t, repo, \"hook\", \"script\")` → `_, finalMsg, err := RunCommitHooks(...)` → assert on finalMsg. The KEY difference: use `echo \"...\" >> \"$1\"` (no leading \\n) to exercise the bug."
  gotcha: "The assertion MUST be `strings.Contains(finalMsg, \"\\nSigned-off-by:\")` (the trailer on a SEPARATE line), NOT `strings.Contains(finalMsg, \"Signed-off-by:\")` (passes even when corrupted to `feat: changeSigned-off-by:...`)."

# Read-only refs
- file: internal/hooks/runner.go # stripCommentLines
  why: "READ-ONLY. `stripCommentLines(s, commentChar)` splits on \\n, filters comment lines, rebuilds, then `strings.TrimRight(b.String(), \"\\n\")`. This is WHY the trailing-\\n guard is safe: the added \\n is consumed on read-back, so the returned finalMsg has no spurious trailing \\n."
- file: internal/hooks/runner.go # RunCommitHooks signature (line 67)
  why: "READ-ONLY. `func RunCommitHooks(ctx, g git.Git, cfg config.Config, snapshotTree, parentSHA, msg string, opts HookOpts) (finalTree, finalMsg string, err error)`. The test calls this with `msg = \"feat: change\"` (no trailing \\n)."

# External reference
- url: https://git-scm.com/docs/githooks#_prepare_commit_msg
  why: "Documents prepare-commit-msg: 'the file ... contains the commit log message so far' — the hook may append to it. Confirms the git-parity expectation that the file ends with \\n (so append starts on a new line)."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/hooks/
    ├── runner.go           # EDIT: trailing-newline guard before WriteString (~line 103) + comment
    └── runner_test.go      # EDIT: +TestRunCommitHooks_PrepareCommitMsg_AppendOnNewLine
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/hooks/runner.go           # +HasSuffix guard + strbuf_complete_line comment
    internal/hooks/runner_test.go      # +1 test (echo append → separate line)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/hooks/runner.go` | MODIFY (1 guard + comment) | Add `if !strings.HasSuffix(finalMsg, "\n") { finalMsg += "\n" }` before WriteString + a git-parity comment. **Only production file.** |
| `internal/hooks/runner_test.go` | MODIFY (append 1 test) | Add `TestRunCommitHooks_PrepareCommitMsg_AppendOnNewLine` using `echo` to exercise the bug. |

**Explicitly NOT touched**: runner.go's WriteString error-handling block, the hook invocation sequence
(runPrepareCommitMsg/runCommitMsg), the read-back/stripCommentLines logic, any other hooks file
(subset.go, detect.go), Issue 1 (argc — S1), Issue 3 (no_verify — P1.M2), Issue 4 (empty msg — P1.M3),
any other package, docs, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — the guard mutates finalMsg BEFORE WriteString): insert the guard immediately before the
// `if _, werr := msgFile.WriteString(finalMsg); werr != nil {` block. The guard mutates the local
// `finalMsg` string; on read-back (~line 131) finalMsg is reassigned from stripCommentLines, so the
// mutation ONLY affects the file content (what the hooks see). Do NOT add the guard AFTER the hooks run
// or in the read-back — it must be before the file is written.

// CRITICAL (G2 — stripCommentLines CONSUMES the trailing \n): the returned finalMsg does NOT have a
// spurious trailing \n. stripCommentLines does TrimRight("\n"). So a caller that checks
// `finalMsg == "feat: change"` (HasPrefix, or exact match of a pre-hook message) still works. Do NOT
// worry about the guard leaking a trailing \n into the return value.

// CRITICAL (G3 — the test MUST use echo, NOT printf '\n...'): echo appends WITHOUT a leading \n — this
// is what exercises the bug. The existing TestRunCommitHooks_CommitMsgAppends_Annotated uses
// `printf '\nSigned-off-by: ...\n' >> "$1"` (with a LEADING \n) which creates the line break even
// without the fix, MASKING the bug. The new test uses `echo "..." >> "$1"` (no leading \n).

// CRITICAL (G4 — the assertion checks \nSigned-off-by:, NOT just Signed-off-by:): the corrupted output
// is `feat: changeSigned-off-by: Dev <dev@example.com>` — the substring "Signed-off-by:" IS present
// (concatenated onto the subject). Only "\nSigned-off-by:" (the trailer on a SEPARATE line) distinguishes
// correct from corrupt. Use `strings.Contains(finalMsg, "\nSigned-off-by:")`.

// GOTCHA (G5 — strings is already imported): runner.go line 21 imports "strings". No new import needed.
// Do NOT add an import — it would be unused (gofmt/Go would complain).

// GOTCHA (G6 — the prior S1 task touches DIFFERENT lines): S1 (Issue 1, argc) edits runner.go:195 (the
// runPrepareCommitMsg argv) + comments at :52/:178. It does NOT touch the WriteString area (~line 103).
// No conflict. If S1 has landed, the line numbers may have shifted slightly — grep for WriteString to
// locate the exact insertion point.
```

## Implementation Blueprint

### Data models and structure

No data-model change. The fix is a string guard (`HasSuffix` + append `\n`) before a file write. The
`finalMsg` local is later reassigned from `stripCommentLines` (read-back), so the mutation is scoped to
the file content only.

### The edit (exact — current → target)

**runner.go** (before the WriteString block, ~line 103):
```go
// CURRENT (the bug — no trailing newline)
	defer os.Remove(msgPath)
	if _, werr := msgFile.WriteString(finalMsg); werr != nil {

// TARGET (insert the guard before WriteString)
	defer os.Remove(msgPath)
	// git parity (strbuf_complete_line): git's COMMIT_EDITMSG always ends with \n so an append-style
	// prepare-commit-msg/commit-msg hook (e.g. `echo "Signed-off-by: ..." >> "$1"`) starts on a new
	// line, not concatenated onto the subject. Add a trailing \n if the message lacks one.
	if !strings.HasSuffix(finalMsg, "\n") {
		finalMsg += "\n"
	}
	if _, werr := msgFile.WriteString(finalMsg); werr != nil {
```

**runner_test.go** (append after the existing CommitMsgAppends test, ~line 185):
```go
// --- Issue 2: trailing newline before prepare-commit-msg (append-style hooks) ---

func TestRunCommitHooks_PrepareCommitMsg_AppendOnNewLine(t *testing.T) {
	repo, snapshotTree, parentSHA, g := primeRunnerRepo(t)
	// echo (NOT printf '\n...') appends WITHOUT a leading \n — exercises the trailing-newline bug.
	installHook(t, repo, "prepare-commit-msg", `echo "Signed-off-by: Dev <dev@example.com>" >> "$1"`)

	_, finalMsg, err := RunCommitHooks(context.Background(), g, defaultCfg(), snapshotTree, parentSHA,
		"feat: change", HookOpts{})
	if err != nil {
		t.Fatalf("RunCommitHooks err = %v, want nil", err)
	}
	// The Signed-off-by MUST be on a separate line (preceded by \n), NOT concatenated onto the subject.
	if !strings.Contains(finalMsg, "\nSigned-off-by:") {
		t.Errorf("Signed-off-by not on a separate line (Issue 2 regression); finalMsg=%q", finalMsg)
	}
	if !strings.HasPrefix(finalMsg, "feat: change") {
		t.Errorf("finalMsg = %q, want the original subject preserved", finalMsg)
	}
}
```

### Implementation Tasks (ordered by dependencies — TDD: failing test first)

```yaml
Task 1: ADD the failing test (runner_test.go) — do FIRST (TDD)
  - FILE: internal/hooks/runner_test.go
  - PLACE: after the existing `TestRunCommitHooks_CommitMsgAppends_Annotated` (~line 185).
  - WRITE: the test verbatim from the edit above. Uses `primeRunnerRepo`/`installHook`/`defaultCfg`/`RunCommitHooks`.
  - KEY: `echo` (not printf), `prepare-commit-msg` (not commit-msg), assertion on `\nSigned-off-by:`.
  - VERIFY (before the fix): `go test -run TestRunCommitHooks_PrepareCommitMsg_AppendOnNewLine ./internal/hooks/ -v`
    → FAILS (Signed-off-by concatenated onto the subject; `\nSigned-off-by:` absent). This confirms the test catches the bug.
  - (After Task 2 the test PASSES.)

Task 2: ADD the trailing-newline guard (runner.go)
  - FILE: internal/hooks/runner.go
  - LOCATE the WriteString block: `defer os.Remove(msgPath)` → `if _, werr := msgFile.WriteString(finalMsg); werr != nil {`.
    (If S1 shifted the lines, grep for `WriteString(finalMsg)` to locate it.)
  - INSERT (between `defer os.Remove(msgPath)` and the WriteString): the guard + comment verbatim from the edit above.
  - DO NOT: touch the WriteString error-handling block, the hook calls, or the read-back/stripCommentLines.
  - RUN: gofmt -w internal/hooks/runner.go
  - VERIFY: `go test -run TestRunCommitHooks_PrepareCommitMsg_AppendOnNewLine ./internal/hooks/ -v` → PASSES.

Task 3: VALIDATE
  - RUN: gofmt -l .
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test -race ./internal/hooks/ -v   # ALL hook tests pass (the new one + all existing)
  - RUN: go test -race ./...                    # full suite green
  - FIX-FORWARD: read failures, fix, re-run.
```

### Implementation Patterns & Key Details

```go
// === Why the guard is safe for the return value ===
// finalMsg (the incoming msg) is mutated: finalMsg += "\n" if it lacks a trailing \n.
// The mutated finalMsg is written to the file. The hooks (prepare/commit-msg) may append.
// On read-back (~line 131): finalMsg = stripCommentLines(string(data), commentChar).
// stripCommentLines rebuilds the string and does TrimRight("\n").
// So: the added \n is consumed; the returned finalMsg has no spurious trailing \n.
// A caller checking HasPrefix(finalMsg, "feat: change") or the exact pre-hook message still works.

// === Why echo (not printf '\n...') exercises the bug ===
// printf '\nSigned-off-by: ...\n' >> "$1" prepends a \n — creates the line break WITHOUT the fix.
// echo "Signed-off-by: ..." >> "$1" appends with NO leading \n — concatenated onto the subject
// WITHOUT the fix. The test uses echo to catch the regression precisely.

// === The FAILS-before / PASSES-after proof ===
// BEFORE (no guard): file = "feat: change" → echo appends → "feat: changeSigned-off-by: ...\n"
//   → stripCommentLines → "feat: changeSigned-off-by: ..." → Contains("\nSigned-off-by:") FAILS.
// AFTER (guard): file = "feat: change\n" → echo appends → "feat: change\nSigned-off-by: ...\n"
//   → stripCommentLines → "feat: change\nSigned-off-by: ..." → Contains("\nSigned-off-by:") PASSES.
```

### Integration Points

```yaml
PRODUCTION (internal/hooks/runner.go):
  - +trailing-newline guard before WriteString (~line 103)
  - +in-source comment (git parity: strbuf_complete_line)
  - the WriteString block, hook calls, read-back/stripCommentLines: UNCHANGED

TESTS (internal/hooks/runner_test.go):
  - +TestRunCommitHooks_PrepareCommitMsg_AppendOnNewLine (echo append → separate line)

CONSUMED (READ-ONLY):
  - runner.go: RunCommitHooks (line 67), stripCommentLines (TrimRight consumes the \n)
  - runner_test.go: primeRunnerRepo, installHook, defaultCfg, HookOpts

GATE: go test -race ./internal/hooks/ → GREEN (new test + all existing pass)

NO-TOUCH (explicitly):
  - runner.go:195 (argv — S1 Issue 1), runner.go:52/178 (comments — S1)
  - Issue 3 (no_verify git-config — P1.M2), Issue 4 (empty msg guard — P1.M3), Issue 5 (decompose test tightening — P1.M4)
  - any other package; docs; PRD.md, tasks.json, prd_snapshot.md, plan/*
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/hooks/   # Expected: empty
go vet ./internal/hooks/... # Expected: exit 0
go build ./...              # Expected: exit 0 (one-line guard; no signature change)

# Expected: Zero errors.
```

### Level 2: The New Test (fails before the fix, passes after)

```bash
cd /home/dustin/projects/stagecoach

# The new test (should PASS after the fix):
go test -race ./internal/hooks/ -v -run TestRunCommitHooks_PrepareCommitMsg_AppendOnNewLine

# Expected: PASS. The Signed-off-by is on a separate line (\nSigned-off-by: present).
# (Before the fix: FAIL — "Signed-off-by not on a separate line"; finalMsg has it concatenated.)

# All hook tests (the new one + every existing):
go test -race ./internal/hooks/ -v

# Expected: ALL pass. The existing TestRunCommitHooks_CommitMsgAppends_Annotated (printf '\n...') +
# TestRunCommitHooks_PrepareCommitMsg_Foreign_AnnotationReadBack still pass (the guard doesn't affect
# messages that already have a trailing \n, and the printf '\n...' leading-\n masking is irrelevant now).
```

### Level 3: Whole-Repository Regression

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...    # Expected: ALL packages green
go vet ./...           # Expected: exit 0

# Confirm ONLY the 2 intended files changed.
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/hooks/runner.go + internal/hooks/runner_test.go only.
```

### Level 4: Return-Value Safety (no spurious trailing \n)

```bash
cd /home/dustin/projects/stagecoach

# The guard adds \n to the FILE content, but stripCommentLines's TrimRight("\n") consumes it on read-back.
# So the returned finalMsg has NO spurious trailing \n. Verify with a hook that does NOT append (the
# message should be returned unchanged, without a trailing \n):
go test -race ./internal/hooks/ -v -run 'TestRunCommitHooks_CommitMsgAppends_Annotated'
# Expected: PASS — the existing test checks HasPrefix(finalMsg, "feat: a change") (the subject preserved),
# which would fail if the guard leaked a trailing \n into the return (it doesn't — stripCommentLines consumes it).
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.

### Feature Validation

- [ ] runner.go has `if !strings.HasSuffix(finalMsg, "\n") { finalMsg += "\n" }` before WriteString.
- [ ] An in-source comment cites git parity (strbuf_complete_line).
- [ ] The new test uses `echo` (not `printf '\n...'`) and asserts `strings.Contains(finalMsg, "\nSigned-off-by:")`.
- [ ] The existing hook tests still pass (CommitMsgAppends_Annotated, PrepareCommitMsg_Foreign_AnnotationReadBack, etc.).
- [ ] The returned `finalMsg` has NO spurious trailing `\n` (stripCommentLines consumes it).

### Scope Discipline Validation

- [ ] ONLY `internal/hooks/runner.go` + `internal/hooks/runner_test.go` modified.
- [ ] Did NOT edit runner.go:195 (argv — S1), the WriteString error block, the hook calls, or the read-back.
- [ ] Did NOT touch Issue 1 (S1), Issue 3 (P1.M2), Issue 4 (P1.M3), Issue 5 (P1.M4), or any other package.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Anti-Patterns to Avoid

- ❌ Don't add the guard AFTER the hooks run or in the read-back — it must be BEFORE `WriteString` (the
  file content is what hooks see). (gotcha G1)
- ❌ Don't worry about the guard leaking a trailing `\n` into the return value — `stripCommentLines`'s
  `TrimRight("\n")` consumes it. (G2)
- ❌ Don't use `printf '\n...' >> "$1"` in the test — the leading `\n` creates the line break WITHOUT the
  fix, MASKING the bug. Use `echo "..." >> "$1"` (no leading `\n`). (G3)
- ❌ Don't assert `strings.Contains(finalMsg, "Signed-off-by:")` — that substring is present even when
  corrupted (`feat: changeSigned-off-by:...`). Use `"\nSigned-off-by:"` (the line-boundary check). (G4)
- ❌ Don't add a `strings` import — it's already imported (line 21). (G5)
- ❌ Don't touch the WriteString error-handling block, the hook invocation sequence, or the read-back.
- ❌ Don't edit Issue 1 (S1), Issue 3 (P1.M2), Issue 4 (P1.M3), or any other package.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a one-guard fix (`if !strings.HasSuffix(finalMsg, "\n") { finalMsg += "\n" }`) with
the verbatim buggy WriteString block quoted from the live source (runner.go ~line 103), the exact target
(copy-paste-ready), the `strings` import confirmed (line 21), the `stripCommentLines` read-back logic
proving the mutation is consumed (TrimRight("\n")), and a fully-specified test using the precise mechanism
(`echo`, not `printf '\n...'`) that exercises the bug with a FAILS-before/PASSES-after proof. The fix
mirrors git's `strbuf_complete_line()` — a proven pattern. The existing tests are confirmed to still pass
(the guard doesn't affect messages that already end with `\n`, and the `printf '\n...'` masking is
irrelevant once the guard is in place). The prior PRP (S1, Issue 1 argc) touches different lines
(runner.go:195 + comments :52/:178) — no conflict. The residual 0.5 uncertainty is purely line-number
drift from the parallel S1 work (if S1 shifted the WriteString line) — mitigated by the "grep for
WriteString(finalMsg) to locate it" guidance.
