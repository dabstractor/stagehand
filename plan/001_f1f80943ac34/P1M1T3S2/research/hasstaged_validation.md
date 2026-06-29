# P1.M1.T3.S2 — HasStagedChanges: Research & Validation Notes

> Research backing the PRP. Pinning behavior empirically on this box (git 2.54.0) so the
> implementing agent needs zero inference.

## 1. The contract (verbatim from the work item + critical_findings.md FINDING 6)

`HasStagedChanges(ctx context.Context) (bool, error)` runs `git -C <repo> diff --cached --quiet`
and translates the exit code:

| git exit | meaning                          | return          |
|---------:|----------------------------------|-----------------|
|   **0**  | NO staged differences (index==HEAD) | `(false, nil)`  |
|   **1**  | staged differences EXIST          | `(true, nil)`   |
| **>1**   | real error                       | `(false, err)`  |

**The trap (FINDING 6):** exit codes are INVERTED from the usual convention. A naive
`if err != nil { /* error */ }` would treat exit 1 (something staged) as an error. Must check
the exit code explicitly: `0 → false`, `1 → true`, anything else → error.

Signature is FIXED by the already-landed `Git` interface (`internal/git/git.go`):

```go
// HasStagedChanges reports whether the index differs from HEAD (git diff --cached --quiet:
// exit 1 ⇒ true, exit 0 ⇒ false). NOT an error when changes exist (FINDING 6).
HasStagedChanges(ctx context.Context) (bool, error)
```

## 2. Empirical exit-code verification (git version 2.54.0 — this box)

Ran against throwaway repos:

```
clean fresh repo (nothing staged)        EXIT=0   → (false, nil)
write a.go + git add a.go                EXIT=1   → (true, nil)
commit init, nothing NEW staged          EXIT=0   → (false, nil)   [proves index-vs-HEAD, not "anything exists"]
non-repo dir (git -C <tmp> diff --cached --quiet)  EXIT=129  → (false, err)   [exit>1 path]
```

Findings:
- `--quiet` produces **NO stdout** (verified: empty) — the exit code is the sole signal. The
  implementation may discard stdout (`_`).
- The non-repo case exits **129**, not 128 (git treats `diff --cached` outside a repo as
  `git diff --no-index`, where `--cached` is an unknown option → exit 129). Either way, any
  non-{0,1} code is an error; a `switch` with a `default` error arm handles 128, 129, and any
  other value uniformly.

## 3. The implementation (delegates to `run()`, NOT exec directly)

`run()` (landed by P1.M1.T2.S1) is the single shell-out point. Its contract is the foundation:

```go
func (g *gitRunner) run(ctx, repo, args...) (stdout, stderr string, exitCode int, err error)
// INVARIANT: err == nil for EVERY real git exit (0, 1, 128, 129, …); the code carries the signal.
//            err != nil  ⟺  infrastructural failure (LookPath miss / context cancel / start-I/O),
//            with exitCode == -1 in that case.
```

Because `run()` already absorbs `*exec.ExitError` (err stays nil, code exposed), `HasStagedChanges`
branches on `code` — it does NOT re-implement the exec plumbing (mirrors RevParseHEAD S2,
WriteTree S3, DiffTree S6). A `switch` is the clearest encoding of the two-value map:

```go
func (g *gitRunner) HasStagedChanges(ctx context.Context) (bool, error) {
	_, stderr, code, err := g.run(ctx, g.workDir, "diff", "--cached", "--quiet")
	if err != nil {
		return false, err // git binary missing / context cancelled / start failure (code=-1)
	}
	switch code {
	case 0:
		return false, nil // nothing staged (index == HEAD)
	case 1:
		return true, nil  // staged changes exist — exit 1 is the "has staged" signal, NOT an error (FINDING 6)
	default:
		return false, fmt.Errorf("git diff --cached --quiet: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
}
```

**Decision D1 — switch over if-chain:** there are exactly two "good" codes (0, 1) and everything
else is an error. A `switch` with `case 0 / case 1 / default` makes the two-value map + catch-all
explicit and unreadable-by-accident. (RevParseHEAD used `if code == 128 / if code != 0` because it
had a single special code; here there are two.)

**Decision D2 — discard stdout (`_`):** `--quiet` guarantees no stdout (verified). Capturing and
ignoring is fine, but `_, stderr, code, err` is the cleanest. `stderr` IS captured for the
default/error arm (carries git's diagnostic, e.g. "not a git repository" / "unknown option cached").

**Decision D3 — no new imports:** `fmt` and `strings` are BOTH already imported in `git.go`
(`import bytes context errors fmt io os/exec strings`). Unlike S2 (RevParseHEAD), which had to add
`strings`, T3.S2 needs ZERO import changes.

## 4. Comparison with the architecture-doc canonical pattern

`git_plumbing_reference.md` (§around line 185) gives a canonical `hasStagedChanges` that execs
directly:

```go
// reference (exec-direct) — do NOT copy verbatim; delegate to run()
args := append([]string{"-C", repo, "diff", "cached", "quiet", "--"}, exclude...)
...
if err == nil { return false, nil }
if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 { return true, nil }
return false, fmt.Errorf("git diff --cached --quiet: %w", err)
```

Differences from our implementation (all intentional, per codebase convention):
- We delegate to `run()` (single shell-out point, PRD §19) instead of `exec.CommandContext`.
- `run()` already performs `errors.As(*exec.ExitError)` and exposes `exitCode` with `err==nil`,
  so we branch on `code` rather than re-doing the type assertion.
- We include the code AND trimmed stderr in the error (matches WriteTree/CommitTree/DiffTree style),
  whereas the reference wraps the raw exec err.
- **No pathspec/exclude argument** in v1 — the contract is the bare `git diff --cached --quiet`
  (the auto-stage-all / commit gate answers "is ANYTHING staged?", not "is anything staged
  outside lock/vendor?"). The arch-doc's optional `-- ':!*.lock'` pathspec is a future enhancement,
  out of scope.

## 5. Test matrix (each test pins one distinct branch)

| Test | Fixture | Asserts | Branch pinned |
|---|---|---|---|
| `TestHasStagedChanges_NothingStaged` | `initRepo` (0 commits, nothing staged) | `(false, nil)` | `code==0` |
| `TestHasStagedChanges_StagedFile` | `initRepo` + `writeFile` + `stageFile` | `(true, nil)` | `code==1` (the INVERTED semantics) |
| `TestHasStagedChanges_CommittedNothingStaged` | `initRepo` + `makeEmptyCommit` (HEAD exists, nothing new staged) | `(false, nil)` | `code==0` — proves index-vs-HEAD comparison, not "anything exists" |
| `TestHasStagedChanges_NotARepo` | `t.TempDir()` WITHOUT `initRepo` | `(false, err)` containing "git diff --cached --quiet: failed" | `default` (exit 129) — proves exit>1 → error, not misread |
| `TestHasStagedChanges_GitBinaryMissing` | `t.Setenv("PATH","")` | `(false, err)` containing "git binary not found" | `err != nil` (LookPath miss) |
| `TestHasStagedChanges_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)` | `err != nil` (ctx cancel) |

**Reusable helpers (do NOT redeclare):** `initRepo` (git_test.go), `writeFile`/`stageFile`
(committree_test.go, S4), `makeEmptyCommit` (revparse_test.go, S2). All are `package git`, so they
are in scope for `hasstaged_test.go`. **No new helpers needed** — the tests use only `initRepo`,
`writeFile`, `stageFile`, `makeEmptyCommit`, `New`, and `HasStagedChanges`. (If a helper were
needed, use an `hs` prefix to avoid collision with S1-T3.S1's `sd`, S5's `cas`, S6's `dt`.)

**New file imports:** `context, errors, strings, testing` (all stdlib; `errors` for `errors.Is`,
`strings` for `strings.Contains` in the err-message assertions). No `os`/`os/exec`/`regexp`.

## 6. Scope & non-overlap (parallel execution with P1.M1.T3.S1)

- T3.S1 (StagedDiff) edits git.go in the `StagedDiff` region (immediately after `parseDiffTree`) and
  removes the `StagedDiff` line from `git_test.go`'s `TestStubsPanic`.
- T3.S2 (this subtask) edits git.go in the `HasStagedChanges` region (the panic stub, further down)
  and removes the `HasStagedChanges` line from `TestStubsPanic`.
- These are **distinct, non-overlapping regions** (different method bodies; different
  `assertPanics` lines). Both can land in parallel without conflict.
- After T3.S2 removes its line, `TestStubsPanic` covers 4 remaining stubs: RecentMessages,
  RecentSubjects, CommitCount, AddAll (those are P1.M1.T3.S3–S5).
- T3.S2 touches ONLY `internal/git/`: git.go (HasStagedChanges body), git_test.go (one line
  removed), and the new `hasstaged_test.go`. No interface/struct/run()/runWithInput changes.

## 7. Downstream consumers (informational — NOT implemented here)

- **P1.M3.T4 (CommitStaged orchestrator):** `staged, err := g.HasStagedChanges(ctx)` is the gate
  before snapshot+generate. `false` (with `auto_stage_all`) → `g.AddAll()` then re-check
  (P1.M3.T2.S2, FINDING 11); still `false` → exit 2 "nothing to commit".
- **P1.M4.T1.S2 (CLI default action):** wires the auto-stage-all + CommitStaged + success report.
- This method is read-only w.r.t. refs/index (it mutates nothing) — PRD §18.1 invariant preserved.
