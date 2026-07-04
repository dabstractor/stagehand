# Research: Lock Wiring (runDefault) + SetSnapshot Calls + Contention Helper

> **Purpose:** Pin the exact edits, the contention-helper logic, and the test strategy for
> P1.M1.T2.S2 — wiring the FR52 per-repo run lock into `runDefault` and publishing the snapshot
> SHA from both commit paths (`generate.go`, `decompose.go`). Built on the COMPLETE `internal/lock`
> package (P1.M1.T1.S1) and the IN-PROGRESS `exitcode.Busy=5` (P1.M1.T2.S1, assume landed).
> All line numbers verified against the live codebase on 2026-07-03.

---

## 1. Inputs (CONTRACTS this subtask consumes)

### 1.1 `internal/lock` (P1.M1.T1.S1 — COMPLETE, read lock.go in full)

Public surface used by S2:
- `func Acquire(repoPath string) (*Locker, error)` — on contention returns `*HeldError`; on other
  error returns a wrapped `fmt.Errorf`. On success returns `*Locker` and stores it in the package
  singleton `current` (so package-level `SetSnapshot` reaches it).
- `func (l *Locker) Release()` — idempotent; closes fd (auto-releases flock) + clears singleton.
- `func SetSnapshot(sha string)` — **package-level**, nil-safe no-op when `current==nil` (library
  / tests with no lock). This is the bridge the library layers call unconditionally.
- `type HeldError struct { Contents LockContents; Path string }` — `Contents` is the holder's parsed
  lock file; `Path` is the lock file path. `Error()` names pid+host.
- `type LockContents struct { Pid, Hostname, Repo, Timestamp, Snapshot string }`.
- `func IsHeldError(err error) bool` — `errors.As`-based predicate.

**The lock file key = sha256(canonical repo path).** Two runs on the same repo contend; two repos
don't. `Acquire(repoDir)` with the contender's `repoDir` is correct (same key → same lock file).

### 1.2 `internal/exitcode` (P1.M1.T2.S1 — assume landed as-specified)

- `Busy = 5` constant (after `Rescue`, before `Timeout=124`).
- `New(code, nil) *ExitError` → `Error()==""` (SILENT) + `For()` returns `code` via the `*ExitError`
  short-circuit. `New(Success, nil)→For==0`; `New(Busy, nil)→For==5`. This is the silent-exit pattern
  `handleGenError`/`handleDecomposeError` already use — `handleLockContention` reuses it identically.

### 1.3 `internal/git.Git` (the interface, git.go)

- `WriteTree(ctx context.Context) (sha string, err error)` — materializes the index into a tree
  object, returns its SHA. **Index-read-only + writes one dangling tree object** — safe to call
  WITHOUT holding the lock (it touches no ref; a dangling tree is harmless). This is the contender's
  fast-path probe. Fails (exit 128) only on unresolved merge conflicts in the index.

## 2. The three edit seams (verified against live code)

### 2.1 `internal/cmd/default_action.go` — acquire/release + helper

The insertion point (integration_seams §1, verified):
```go
// lines 54-55 (EXISTING)
repoDir, err := os.Getwd()
if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: getwd: %w", err)) }
g := git.New(repoDir)
// ← INSERT lock acquire here. repoDir is the key; g is the contender's git for the fast-path probe.
//   The auto-stage state machine begins at line 73 (flagAll / HasStagedChanges / runDecompose).
```

`repoDir` and `g` are both in scope at the insertion point. `defer locker.Release()` placed here
fires on EVERY early-exit below: nothing-to-commit (2), dry-run (0), CAS (1), rescue (3), push-fail
(1) — and on success (0). This single defer covers BOTH the single-commit path (lines 73–204) and
the decompose path (`runDecompose` called at line 110). Read-only subcommands have their own `RunE`
and never reach `runDefault`; `hookexec.go` (hook mode) has its own `RunE` (`runHookExec`) and only
WRITES a message (git commits) — it is NOT a ref-mutating action, so it correctly needs no lock.

**Imports already present in default_action.go:** `context, errors, fmt, io, os` + cobra + stagehand
internals. **`lock` is NOT imported** — ADD `"github.com/dustin/stagehand/internal/lock"`.

### 2.2 `internal/generate/generate.go` — SetSnapshot (single-commit path)

Verified anchor (generate.go, inside `CommitStaged`, after Step 3 WriteTree):
```go
treeSHA, err := deps.Git.WriteTree(ctx)
if err != nil { return Result{}, err }
// *** SNAPSHOT TAKEN — HEAD & committed content are frozen w.r.t. this run. ***
signal.SetSnapshot(treeSHA, parentSHA, "") // arm rescue (§18.4)
// ← ADD: lock.SetSnapshot(treeSHA)   (one line, mirrors the signal.SetSnapshot call above)
```
`treeSHA` is the frozen index tree — exactly the value a contender compares against. Import
`internal/lock`. `CommitStaged` is called by `pkg/stagehand.GenerateCommit` (the public API) AND by
`runDefault` (US12 dogfooding) — so this single site publishes for both CLI and library callers.

### 2.3 `internal/decompose/decompose.go` — SetSnapshot (decompose path)

Verified anchor (decompose.go, inside `Decompose`, step 3 freeze):
```go
tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)
if err != nil {
    return DecomposeResult{}, fmt.Errorf("%w: freeze working tree: %w", ErrDecomposeFailed, err)
}
// ← ADD: lock.SetSnapshot(tStart)   (right after the freeze succeeds, before the one-file short-circuit)
```
`tStart` is the frozen working-tree snapshot (the decompose equivalent of the single-commit `treeSHA`).
Import `internal/lock`. NOTE: the `runSingleEscape` path (line 143, `Single||Commits==1`) returns
BEFORE the freeze — it routes to `runSingleEscape` which calls the v1 `generate` path (where
`generate.go`'s SetSnapshot fires). So both escape routes are covered by exactly one SetSnapshot each.

## 3. The contention helper (exact logic)

`handleLockContention` lives in `default_action.go` alongside `handleGenError`/`handleDecomposeError`
(the codebase convention for `handle*` mapping helpers). Signature per the contract:
```go
func handleLockContention(stderr io.Writer, heldErr *lock.HeldError, g git.Git, ctx context.Context) error
```

Logic (matches contract §3.a.ii/iii + PRD §18.5 "Contention behavior"):
```go
func handleLockContention(stderr io.Writer, heldErr *lock.HeldError, g git.Git, ctx context.Context) error {
    // No-op fast path: holder published a snapshot AND the contender's index is byte-identical.
    if snap := heldErr.Contents.Snapshot; snap != "" {
        contenderTree, werr := g.WriteTree(ctx) // index-read-only + one harmless dangling tree; safe w/o lock
        if werr == nil && contenderTree == snap {
            fmt.Fprintln(stderr, "nothing to do — an in-progress run already covers your staged changes.")
            return exitcode.New(exitcode.Success, nil) // exit 0, SILENT (message already printed)
        }
    }
    // Busy: snapshot empty, tree SHAs differ, or WriteTree failed → refuse, leave new changes staged.
    fmt.Fprintf(stderr,
        "stagehand: another stagehand run is already in progress on %s (pid %s on %s). "+
            "Your newly-staged changes will remain staged — re-run stagehand after it finishes. Lock: %s.\n",
        heldErr.Contents.Repo, heldErr.Contents.Pid, heldErr.Contents.Hostname, heldErr.Path)
    return exitcode.New(exitcode.Busy, nil) // exit 5, SILENT (message already printed)
}
```

**Branch analysis:**
| Condition | Outcome | Exit |
|---|---|---|
| snapshot != "" && WriteTree ok && contenderTree == snapshot | "nothing to do…" | 0 (Success) |
| snapshot == "" | busy message | 5 (Busy) |
| snapshot != "" && contenderTree != snapshot | busy message | 5 (Busy) |
| snapshot != "" && WriteTree errors (e.g. merge conflicts) | busy message (falls through) | 5 (Busy) |

**Why WriteTree error → Busy (not a separate error):** the contender cannot proceed anyway (lock
held). Surfacing a merge-conflict error mid-contention is confusing and unactionable while the holder
runs. Refusing as Busy lets the user re-run later — at which point, with the lock free, the real
WriteTree error surfaces when actionable. Documented as decision D2.

**Em-dash:** both messages use `—` (U+2014), matching the codebase convention (e.g. the FR18
auto-stage notice `"Nothing staged — staging all changes"`).

## 4. The acquire-site code (exact)

```go
g := git.New(repoDir)

// FR52 / §18.5: acquire the per-repo run lock so two stagehand processes cannot race on update-ref.
// One insertion covers both the single-commit and decompose paths (runDecompose is called below);
// defer Release fires on every early-exit. Read-only subcommands never reach runDefault.
locker, lockErr := lock.Acquire(repoDir)
if lockErr != nil {
    var held *lock.HeldError
    if errors.As(lockErr, &held) { // contention → no-op fast path (exit 0) or Busy (exit 5)
        return handleLockContention(stderr, held, g, ctx)
    }
    return exitcode.New(exitcode.Error, fmt.Errorf("acquire run lock: %w", lockErr))
}
defer locker.Release()
```

`errors.As` (not a type assertion, not `lock.IsHeldError`) — matches the codebase-wide convention
(`handleGenError`/`handleDecomposeError` use `errors.As`), is wrapsafe, and yields the typed `*held`
the helper needs. (`lock.IsHeldError` exists but returns only bool — insufficient since the helper
needs the `*HeldError` value.)

## 5. Test strategy

### 5.1 `handleLockContention` unit tests (NEW file `internal/cmd/lock_contention_test.go`, package cmd)

The helper depends only on `(io.Writer, *lock.HeldError, git.Git, context.Context)`. To avoid a
real git repo AND a full fake of the ~15-method `git.Git` interface, embed the interface and override
only `WriteTree` (the sole method the helper calls):
```go
type contentionFakeGit struct {
    git.Git            // embed the interface (nil); all methods panic if called — but only WriteTree is used
    writeTreeSHA string
    writeTreeErr error
}
func (f *contentionFakeGit) WriteTree(context.Context) (string, error) {
    return f.writeTreeSHA, f.writeTreeErr
}
```
Construct `*lock.HeldError` directly (fields are exported):
```go
&lock.HeldError{
    Contents: lock.LockContents{Pid: "4242", Hostname: "testhost", Repo: "/path/repo", Snapshot: "abc123"},
    Path:     "/tmp/xyz.lock",
}
```
Cases (assert via `exitcode.For(err)` + `err.Error()==""` for silence + `strings.Contains(stderr,...)`):
1. `TestHandleLockContention_NoOpFastPath` — snapshot=="abc123", WriteTree→("abc123",nil) → For==0, stderr has "nothing to do".
2. `TestHandleLockContention_Busy_TreeDiffers` — snapshot=="abc123", WriteTree→("zzz",nil) → For==5, stderr has busy msg + "4242" + "testhost".
3. `TestHandleLockContention_Busy_EmptySnapshot` — snapshot=="" → For==5, stderr has busy msg (fast path skipped).
4. `TestHandleLockContention_Busy_WriteTreeError` — snapshot=="abc123", WriteTree→("",err) → For==5 (falls through to busy).
5. `TestHandleLockContention_SilentExit` — all Busy/Success returns have `err.Error()==""` (no double-print in main).

### 5.2 Wiring test — lock released after a normal run (default_action_test.go style)

Proves the `defer locker.Release()` is live (not dead code): run a normal commit via `Execute(ctx)`
against a temp repo (reuse `setupStubRepo`/`isolateHome`), then AFTER it returns successfully,
`lock.Acquire(repoDir)` must succeed (not contend) — proving Release fired. Add to
`lock_contention_test.go` or `default_action_test.go`.

**GOTCHA — lock-dir isolation in tests:** `isolateHome` sets HOME + XDG_CONFIG_HOME but NOT
XDG_RUNTIME_DIR or XDG_CACHE_HOME. `lock.lockDir()` resolves XDG_RUNTIME_DIR → XDG_CACHE_HOME →
`~/.cache`. In CI/dev boxes where XDG_CACHE_HOME may be inherited, set it explicitly in the test
(`t.Setenv("XDG_CACHE_HOME", t.TempDir())` and `t.Setenv("XDG_RUNTIME_DIR","")`) so the lock lands
in the test's own temp tree and can't collide with a real user's locks or other tests. Each test uses
a unique `t.TempDir()` repo, so repo-key collisions across tests don't happen regardless.

### 5.3 SetSnapshot-published test (generate + decompose)

The one-line `lock.SetSnapshot` additions are no-ops when no lock is held, so existing
generate/decompose tests pass UNCHANGED (gate: green suite). To prove the call is LIVE, add a focused
test: `lock.Acquire(testRepo)` → run `CommitStaged`/`Decompose` with a stub agent → read the lock
file → assert the `snapshot=` line is non-empty and equals the committed tree's SHA
(`git rev-parse HEAD^{tree}`). This is the wiring proof for the two new one-liners. (The full
two-subprocess contention E2E is P1.M1.T2.S3 — do NOT build that here.)

## 6. Scope boundaries (do NOT do)

- Do NOT implement the full two-subprocess contention E2E (held→Busy, double-run→0, stale-lock,
  dry-run bypass) — that is P1.M1.T2.S3.
- Do NOT touch `internal/lock/*` (P1.M1.T1.S1, COMPLETE) or `internal/exitcode/*` (P1.M1.T2.S1).
- Do NOT add the lock to `hookexec.go` (hook mode writes a message; git commits — not a ref mutator).
- Do NOT add the lock to read-only subcommands (`config`, `providers`, `models`, `--version`) — they
  have their own `RunE` and never reach `runDefault`; the bypass is structural, no code needed.
- Do NOT edit `docs/cli.md`'s Busy row (S1 owns it — parallel-edit conflict risk). S2 only VERIFIES
  the contention note is present as a gate.
- Do NOT add deps (lock is stdlib-only; exitcode/generate already imported).

## 7. Decisions log

| # | Point | Decision | Why |
|---|---|---|---|
| D1 | `errors.As` vs type-assert vs `IsHeldError` | `errors.As(lockErr, &held)` | Codebase convention (handleGenError/handleDecomposeError); wrapsafe; yields the typed value the helper needs. |
| D2 | WriteTree error during fast-path → ? | Fall through to Busy (exit 5) | Unactionable while lock held; the real error surfaces on re-run when the lock is free. Keeps the helper a single return shape. |
| D3 | Where does `handleLockContention` live? | `default_action.go` (next to handleGenError/handleDecomposeError) | Codebase convention for `handle*` mapping helpers. |
| D4 | Mock git.Git how? | Embed `git.Git` interface, override only WriteTree | Avoids a real repo AND faking ~15 methods; the helper calls only WriteTree. |
| D5 | Edit docs/cli.md? | NO — verify only | S1 (parallel) owns the Busy row + contention note; editing risks a conflict. S1's PRP explicitly includes the note. |
| D6 | hookexec lock? | No | Hook mode writes a message file; git performs the commit — no stagehand ref mutation, so no lock. |
| D7 | Test lock-dir isolation | Explicit `t.Setenv("XDG_CACHE_HOME", temp)` + unset XDG_RUNTIME_DIR | isolateHome doesn't set cache/runtime dirs; avoids colliding with real user locks. |
