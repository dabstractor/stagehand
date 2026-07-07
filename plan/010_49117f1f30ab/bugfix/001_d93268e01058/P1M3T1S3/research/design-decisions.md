# P1.M3.T1.S3 — decompose.publishCommit empty-message guard: Design Decisions

The non-obvious calls for the Issue-4 guard on the THIRD and final call site. Ground truth: Bug-Fix
PRD §h3.3 (Issue 4), the work-item CONTRACT, the S1/S2 sibling PRPs (the same-pattern precedent — S1
shipped `generate.CommitStaged`'s guard, S2 ships `pkg/stagecoach.runPipeline`'s), and the actual code at
`internal/decompose/message.go` (`publishCommit`) + `internal/decompose/message_test.go` + `internal/decompose/decompose.go`
(`runLoop`) + `internal/generate/finalize.go` (`ErrEmptyMessage`) + `internal/exitcode/exitcode.go`.

Read this BEFORE implementing — it is the single most important reference.

---

## §0 — Scope: `decompose.publishCommit` ONLY (the 3rd and final call site of Issue 4)

Issue 4 names THREE call sites that all lack the post-`RunCommitHooks` empty-message guard:

| Call site | File | Status | Owner |
|-----------|------|--------|-------|
| `generate.CommitStaged` | `internal/generate/generate.go:436-439` | **DONE** | S1 (P1.M3.T1.S1) |
| `pkg/stagecoach.runPipeline` | `pkg/stagecoach/stagecoach.go` (after hooks block) | **IMPLEMENTING** (parallel) | S2 (P1.M3.T1.S2) |
| `decompose.publishCommit` | `internal/decompose/message.go` (after `if herr != nil`) | **THIS TASK** | S3 |

S3 edits `internal/decompose/message.go` (+ its test) ONLY. Do NOT touch `internal/generate/*` (S1),
`pkg/stagecoach/*` (S2), `runLoop`, `Decompose()`, the CLI, or any other file. The three guards are
independent (one per call site); S3 is the decompose one.

---

## §1 — The guard (exact)

Inserted in `publishCommit` AFTER the closing `}` of the `if herr != nil { return "", herr }` block
(file line ~234) and BEFORE `newSHA, err := deps.Git.CommitTree(ctx, finalTree, parents, finalMsg)`
(file line ~235):

```go
// §9.25 git parity (Issue 4): a prepare-commit-msg / commit-msg hook may have emptied the message
// file (a rejection / force-re-edit pattern). git aborts "Aborting commit due to empty commit
// message."; mirror it — return the BARE generate.ErrEmptyMessage (exit 1, NOT a rescue), same as
// the --edit path (generate.EditMessage), generate.CommitStaged's guard (S1), and runPipeline's
// guard (S2). HEAD + live index are untouched (the abort returns before CommitTree → no update-ref ran).
if strings.TrimSpace(finalMsg) == "" {
	return "", generate.ErrEmptyMessage
}
```

Two `publishCommit`-specific details (vs S1/S2):

1. **The variable is `finalMsg`** (the hook-adjusted message from `finalTree, finalMsg, herr := hooks.RunCommitHooks(...)`),
   NOT `msg` (S1/S2 use `msg` because their hooks blocks reassign `msg = fm`). `publishCommit` keeps the
   hook result in a SEPARATE `finalTree`/`finalMsg` pair (it never reassigns `tree`/`msg`), so the guard
   reads `finalMsg`.
2. **The return is `("", generate.ErrEmptyMessage)`** — `publishCommit` returns `(string, error)` where the
   string is the new commit SHA, so the empty string is the "no SHA" zero value (mirrors `return "", herr`
   one line above). S1/S2 return `Result{}, ErrEmptyMessage` (different return type).

---

## §2 — BARE error (NOT a *RescueError, NOT a *CASError) → exit 1

`generate.ErrEmptyMessage` (finalize.go:45: `var ErrEmptyMessage = errors.New("stagecoach: empty commit
message — aborted")`) is a BARE `error`, not `*generate.RescueError` (which would be exit 3 / a rescue)
and not `*generate.CASError` (which would be the §13.5 partial-commit CAS path). The CLI maps it to exit 1
via the EXISTING `exitcode.For` (exitcode.go:65: `if errors.Is(err, generate.ErrEmptyMessage) { ... return
Error }` — exit 1, verified by exitcode_test.go:25). No CLI/exitcode change is needed.

This matches S1 (CommitStaged), S2 (runPipeline), the existing `--edit` path (`generate.EditMessage` →
`if edited == "" { return ErrEmptyMessage }`), and git's "Aborting commit due to empty commit message.".

---

## §3 — runLoop propagation: the bare error hits the EXISTING hard-error path (verified)

The work item says the guard "propagates to runLoop's FR-M12 handling." Verified against
`internal/decompose/decompose.go` — runLoop's `publishCommit` error handling (around line 484-491) is:

```go
newSHA, err := publishCommit(ctx, deps, res.treeB, prevSHA, res.msg)
if err != nil {
	var ce *generate.CASError
	if errors.As(err, &ce) {
		// FR-M12b: CAS failed → §13.5 message. Prior commits stand.
		...
		return ce
	}
	return err // HARD — propagate (the bare ErrEmptyMessage takes THIS path)
}
```

A bare `generate.ErrEmptyMessage`:
- `errors.As(err, &ce)` → **false** (ErrEmptyMessage is not `*CASError`) ⇒ does NOT enter the FR-M12b CAS path.
- Falls through to `return err` (the HARD path) ⇒ propagates verbatim up to `Decompose()` ⇒ CLI ⇒ exit 1.

The two other `publishCommit` call sites (single-concept at decompose.go:336, arbiter at decompose.go:390)
both do `if err != nil { return DecomposeResult{}, err }` — they propagate the bare error directly too.

**So S3 does NOT touch `runLoop`, `Decompose()`, or any orchestrator code.** The guard's bare error rides
the EXISTING hard-error propagation that any non-CAS `publishCommit` failure already takes. Prior
committed concepts are already in HEAD (they passed `UpdateRefCAS`); the empty-message concept is NOT
committed (the abort returns before `CommitTree`). This is the git-parity outcome.

---

## §4 — TDD: the failing test mirrors `TestPublishCommit_PreCommitAbort_RescueError`

The closest in-repo template is `TestPublishCommit_PreCommitAbort_RescueError` (message_test.go:436) — it
calls `publishCommit` DIRECTLY with a real shell-script hook and asserts the error type + HEAD unchanged.
S3's test mirrors it exactly, swapping the hook + the assertion:

| Aspect | PreCommitAbort test (existing) | HookEmptiesMessage test (NEW) |
|--------|-------------------------------|-------------------------------|
| Hook installed | `pre-commit` → `exit 1` | `commit-msg` → `> "$1"; exit 0` |
| Expected error | `*generate.RescueError` (FR-V7) | `generate.ErrEmptyMessage` (Issue 4) |
| Assertion | `errors.As(err, &re)` | `errors.Is(err, generate.ErrEmptyMessage)` |
| HEAD | unchanged (no commit) | unchanged (no commit) |

**Before the guard (TDD red):** a `commit-msg` that empties the file → `RunCommitHooks` returns `finalMsg=""`
→ `CommitTree(ctx, finalTree, parents, "")` creates an empty-message commit (git `commit-tree` is a plumbing
command — unlike `git commit`, it does NOT refuse an empty message; Issue 4 confirmed `git log -1 --format=%B | xxd → 0a`)
→ `UpdateRefCAS` succeeds → `publishCommit` returns `(newSHA, nil)`. So the test's `errors.Is` fails (err==nil)
AND HEAD moved. **The test FAILS on the unfixed tree** (the TDD proof it reproduces Issue 4).

**After the guard (TDD green):** the guard returns `("", generate.ErrEmptyMessage)` before `CommitTree` →
`errors.Is` passes + HEAD unchanged.

Why `commit-msg` (not `prepare-commit-msg`): the work item specifies it, and it is the LAST hook to touch
the message file, so emptying it unambiguously yields `finalMsg=""`. (`prepare-commit-msg` would also work —
Issue 4 notes both hooks trigger the bug — but `commit-msg` is the cleaner reproduction.)

---

## §5 — No new imports (either file)

- `internal/decompose/message.go` ALREADY imports `"strings"` (line 34) and
  `"github.com/dustin/stagecoach/internal/generate"` (line 37 — for `RescueError`, `CASError`,
  `ExtractSubject`, `IsDuplicate`, `FinalizeMessage`, `EditMessage`). The guard uses `strings.TrimSpace` +
  `generate.ErrEmptyMessage` ⇒ NO new import.
- `internal/decompose/message_test.go` ALREADY imports `"errors"` (line 6), `"strings"` (line 11), and
  `"github.com/dustin/stagecoach/internal/generate"` (line 16). The test uses `errors.Is` +
  `generate.ErrEmptyMessage` ⇒ NO new import.

`go mod tidy` is a no-op; `git diff --exit-code go.mod go.sum` is empty.

---

## §6 — The existing helpers are reused verbatim

The test reuses (all in `message_test.go`, `package decompose`):
- `msgInstallHook(t, repo, name, body string)` (line 392) — writes an executable hook to
  `<repo>/.git/hooks/<name>`, mode 0755. S3 calls `msgInstallHook(t, repo, "commit-msg", "#!/bin/sh\n> \"$1\"\nexit 0\n")`.
- `messageDeps(t, repo, m provider.Manifest) Deps` (line 74) — wires `Git`/`Config`/`Roles`/`Verbose`.
- `msgInitRepo`, `msgCommitRaw`, `msgWriteFile`, `msgStageFile`, `msgGitOut`, `msgHeadSHA` — the fixture
  primitives (identical to the pre-commit-abort test's setup).
- `stubtest.Build(t)` + `stubtest.Manifest(bin, stubtest.Options{})` — the stub agent (publishCommit's
  `msg` arg is passed in directly by the test, so the stub is only needed to satisfy `messageDeps`'s
  manifest param; its output is irrelevant — the HOOK empties the message, not the generator).

---

## §7 — Complementary to (not overlapping) the EditMessage guard

The arbiter path (`resolveArbiter`, decompose.go:380-392) runs `generate.EditMessage` (the §9.22 FR-E1
editor gate) BEFORE `publishCommit`. `EditMessage` already guards its OWN path
(`if edited == "" { return ErrEmptyMessage }`, finalize.go:118). S3's guard is in `publishCommit`, which
runs AFTER `EditMessage` and guards the HOOKS path specifically: `EditMessage` does not run hooks, and
`publishCommit` does not run the editor, so the two guards cover disjoint inputs. In the arbiter flow, if
the editor produces a non-empty message but a `commit-msg` hook then empties it, `EditMessage`'s guard
does NOT fire (it already returned) — only `publishCommit`'s new guard catches it. This is exactly the
Issue-4 gap on the decompose path.

---

## §8 — Test placement + naming

Append `TestPublishCommit_HookEmptiesMessage_Aborts` to `internal/decompose/message_test.go` (after
`TestPublishCommit_PreCommitAbort_RescueError`, ~line 470, keeping the publishCommit hook tests grouped).
`package decompose` (white-box — `publishCommit` is unexported). Naming follows the file's convention
(`TestPublishCommit_<Scenario>_<Outcome>`).
