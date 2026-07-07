# System Context — FR52 Per-Repo Run Lock (Bugfix 001)

## What this changeset touches

Stagecoach's **per-repo run lock** (FR52 / PRD §18.5) serializes concurrent
commit-producing runs against the same git repo so two processes cannot race on
`update-ref` (HEAD). The lock is the *first* line of defense; the §13.5
`update-ref` CAS is the *second* (holds even on shared/network FS the lock cannot
cover).

This bugfix addresses 1 Major + 3 Minor issues found in creative QA — none affect
core concurrency safety (no data-corruption, double-commit, stale-lock, or
false-no-op). All four issues are correctness/UX/hygiene polish.

## File map (confirmed by codebase recon)

| File | Role | Touched by issue |
|------|------|------------------|
| `internal/lock/lock.go` | Core lock primitive: `Acquire`, `Release`, `setSnapshot`/`writeContents`, `parseContents`, `lockHash`/`lockPath`/`lockDir`. Stdlib-only leaf package. | 2, 3, 4 |
| `internal/lock/lock_unix.go` / `lock_windows.go` | Platform flock (`LOCK_EX\|LOCK_NB`) / no-op stub. | (none — read-only context) |
| `internal/lock/lock_test.go` | 14 unit tests: lockDir resolution, canonical-symlink hash/path, Acquire/Release round-trip, SetSnapshot, HeldError. | 2, 3, 4 (new tests) |
| `internal/cmd/default_action.go` | CLI router spine: `runDefault` (acquires lock first, lines 59–67), `shouldDecompose`, `runDecompose`, `handleLockContention` (lines 241–256). | 1 (e2e), 4 (guard) |
| `internal/cmd/lock_contention_test.go` | Unit tests for `handleLockContention` (4 branches + silent-exit + lock-released). | 4 (new test) |
| `internal/decompose/decompose.go` | Decomposition orchestrator. `Decompose` → `FreezeWorkingTree` (line 165) → `lock.SetSnapshot(tStart)` (line 169). | 1 (context only — NOT modified under Option 1) |
| `internal/exitcode/exitcode.go` | `Busy=5` constant; `For` routes via `*ExitError` short-circuit (no dedicated `errors.Is` arm needed). | (none — read-only context) |
| `internal/e2e/lock_scenarios_test.go` | 5 cross-process contention scenarios A–E (build tag `e2e`). | 1 (new scenario F) |
| `internal/e2e/harness_test.go` | Shared e2e helpers: `buildStagecoach`, `newRepo`, `runStagecoach`, `writeStubConfig`, `stubEnv`, `waitForMarker`. | 1 (reuse) |
| `README.md` (line 330) | "Safe to run twice" FAQ claim. | 1 |
| `docs/cli.md` (line 379) | Contention-behavior prose under exit-code table. | 1 |
| `docs/how-it-works.md` (line 155) | "No-op fast path" subsection. | 1 |

## Control-flow spine (the contention decision)

```
runDefault (cmd/default_action.go:35)
  ├─ lock.Acquire(repoDir)                           [line 59]
  │     ├─ OK → *Locker; defer locker.Release()      [line 67]
  │     └─ *lock.HeldError → handleLockContention()  [line 63] → returns immediately
  │
  ├─ [only after Acquire OK] HasStagedChanges?       [line 83]
  │     └─ !hasStaged && shouldDecompose && dirty → runDecompose  [line 98]
  │           └─ decompose.Decompose → FreezeWorkingTree → lock.SetSnapshot(tStart)  [decompose.go:165-169]
  │                 (the snapshot is a WORKING-TREE tree, NOT an index tree — Issue 1)
  │
  └─ GenerateCommit (single-commit path)             [line ~181]

handleLockContention (cmd/default_action.go:241)
  ├─ snap = held.Contents.Snapshot
  ├─ if snap != "":
  │     contenderTree = g.WriteTree(ctx)             [index-derived tree]
  │     if contenderTree == snap → exit 0 (no-op)    [NEVER true on decompose path — Issue 1]
  └─ → exit 5 (Busy) + message naming holder's repo/pid/hostname
```

## Key data structures

```go
// internal/lock/lock.go
type Locker struct {
    file      *os.File  // fd holding flock; nil after Release
    path      string    // lock file path (<hash>.lock)
    pid       string
    hostname  string
    repo      string    // BUG (Issue 3): raw repoPath, NOT canonical
    timestamp string
    // NOTE: no `snapshot` field — snapshot is a writeContents param only
}

type LockContents struct{ Pid, Hostname, Repo, Timestamp, Snapshot string }
type HeldError struct{ Contents LockContents; Path string }
```

On-disk format (fixed order, newline key=value, no escaping):
```
pid=<pid>
hostname=<host>
repo=<raw-repoPath>
timestamp=<RFC3339-UTC>
snapshot=<sha-or-empty>
```

## Snapshot publish axis mismatch (the root of Issue 1)

| Path | What holder publishes as `snapshot=` | What contender computes via `WriteTree()` |
|------|--------------------------------------|-------------------------------------------|
| Single-commit (staged) | `WriteTree()` result (index tree) | `WriteTree()` result (index tree) — **same axis → can match** |
| Decompose (nothing staged) | `T_start` (working-tree tree, from `FreezeWorkingTree` step 2) | `baseTree` = `HEAD^{tree}` (index was reset to baseTree by `FreezeWorkingTree` step 3) — **different axis → NEVER matches** |

Decompose activates iff nothing is staged (FR-M1) → contender's index == HEAD == baseTree,
but holder published the working-tree snapshot T_start ≠ baseTree (tree has changes or
decompose wouldn't activate). So `contenderTree == snap` is **always false** on the decompose
path → contender always exits Busy(5), never the documented 0.
