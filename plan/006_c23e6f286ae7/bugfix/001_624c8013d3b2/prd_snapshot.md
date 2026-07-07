# Bug Fix Requirements

## Overview

Creative end-to-end QA of the **FR52 per-repo run lock** feature (PRD §18.5; tasks P1.M1.T1/T2/T3) implemented in `internal/lock/`, `internal/exitcode/`, `internal/cmd/default_action.go`, `internal/generate/generate.go`, and `internal/decompose/decompose.go`.

**Testing performed:**
- Read PRD §18.5 (location, contents, mechanism, contention behavior, limits) and mapped every requirement to code.
- Ran the full test suite (`go test ./...`) — all pass; `go vet ./...` clean.
- Ran the lock unit tests (`internal/lock`) — 14/14 pass.
- Ran the e2e contention suite (`go test -tags e2e -run TestE2ELockContention`) — 5/5 pass.
- Manually reproduced cross-process scenarios with the real compiled binary + stub agent against fresh temp repos (single-commit no-op fast path, decompose no-op fast path, contention messages).
- Inspected lock-file contents on disk, the contention code path, signal-handler interaction, and all commit-producing entry points.

**Overall assessment:** The lock primitive itself is well-built and correct — flock-based acquisition, auto-release on process death (verified the SIGINT `os.Exit(3)` path is safe because flock releases on fd close), correct XDG location resolution with no CWD/repo-internal fallback, correct sha256 canonical-path hashing (symlinks verified), correct Busy=5 exit code, correct read-only-subcommand bypass, and correct single-commit no-op fast path. **One Major issue** undermines the headline safety/UX claim for an entire commit path, plus three Minor polish items.

## Critical Issues (Must Fix)

None. Core concurrency safety (the primary purpose of FR52 — preventing two stagecoach processes from racing on HEAD) works on BOTH commit paths. No data-corruption, double-commit, stale-lock, or false-no-op conditions were found.

## Major Issues (Should Fix)

### Issue 1: The no-op fast path (exit 0 on accidental double-run) never fires on the multi-commit decomposition path

**Severity**: Major
**PRD Reference**: §18.5 "Contention behavior" (no-op fast path: "the contending run's own staged snapshot (`write-tree`) … is byte-identical to it … exits 0 with 'nothing to do …'"); README.md:330 ("Safe to run twice … if nothing new is staged it exits `0`"); docs/cli.md:379. Cross-references the decompose path (§13.6, FR-M1b, G11).
**Expected Behavior**: An accidental double-invoke (e.g. double-tapping a lazygit keybind bound to `stagecoach`) with a dirty working tree and nothing staged should exit **0** with *"nothing to do — an in-progress run already covers your staged changes."* — for **both** the single-commit path and the decompose path. The README states this unconditionally: "if nothing new is staged it exits `0`."
**Actual Behavior**: On the **decompose path** the contender always exits **5 (Busy)**, never 0. The no-op fast path is structurally impossible to satisfy on this path:

- Decomposition activates iff *nothing is staged* (FR-M1). The holder resets the index to `baseTree` (HEAD's tree) when it freezes `T_start`, then publishes `lock.SetSnapshot(tStart)` where `T_start` is the **working-tree** snapshot (`internal/decompose/decompose.go:169`).
- The contender computes its own staged snapshot via `g.WriteTree(ctx)` in `handleLockContention` (`internal/cmd/default_action.go`). With nothing staged, `git write-tree` returns HEAD's tree (`baseTree`), which by definition ≠ `T_start` (the working tree has changes, otherwise decompose would not have activated).
- `contenderTree == snap` is therefore **always false** on the decompose path → the contender falls through to Busy(5).

This was confirmed empirically. Holder lock file published `snapshot=c0f5cf74…` (`T_start`); the contender's `git write-tree` returned `d7a57d28…` == `git rev-parse HEAD^{tree}` (`baseTree`); contender exited `5` with the busy message instead of `0` with the no-op message.

**Steps to Reproduce**:
1. Build the binary and stub agent: `go build -o /tmp/stagecoach ./cmd/stagecoach && go build -o /tmp/stubagent ./cmd/stubagent`.
2. Create a stub config (`config_version=3`, `[provider.stub]` pointing at the stub binary, `output="raw"`).
3. `git init` a temp repo, seed one commit, create **one** untracked file (so decompose takes the FR-M2b one-file shortcut and the stub can satisfy the message role), leave it **unstaged**.
4. Launch `stagecoach --provider stub` with `STAGECOACH_STUB_SLEEP_MS=6000` so the holder holds the lock and publishes `snapshot=T_start`.
5. While it sleeps, launch a second `stagecoach --provider stub` against the same repo (same dirty tree, still nothing staged).
6. Observe: the second run exits **5 (Busy)**, not **0**. The README promises 0.

(Note: the e2e suite only covers the no-op fast path on the **single-commit** path — `lock_scenarios_test.go` scenario B stages a file so both runs share an index tree. There is **no** e2e scenario for the decompose no-op fast path, which is why this gap survived validation.)

**Suggested Fix**: The root cause is comparing an **index-derived** tree (`write-tree`) against a **working-tree-derived** snapshot (`T_start`) on the decompose path. Options, in order of cleanliness:
1. **Best:** On the decompose path, publish a snapshot the contender can actually reproduce from a read-only, lock-free git call. Since the index is empty at decompose activation, the contender would need to compare its **working-tree** state against `T_start`. A read-only equivalent is `git stash create` / a temporary `add -A` + `write-tree` + index restore — but that mutates the index transiently, which the holder already does in `FreezeWorkingTree` and the contender should not do without the lock. The safest minimal fix is to **qualify the documentation** to state the no-op fast path applies to the single-commit (staged) path only, and decompose accidental double-runs exit Busy — and add an e2e scenario that asserts this so the behavior is pinned. (This is the lowest-risk change and makes the docs honest.)
2. **Alternative (more invasive):** Have the holder, on the decompose path, also publish the **index** tree it will use (or `baseTree`), and have `handleLockContention` accept a match against either the working-tree snapshot or the base tree when the contender is on the decompose path. This requires the contender to know it's on the decompose path before taking the lock (it does: `shouldDecompose` is evaluated in `runDefault` after `Acquire` fails the first time, so the contention handler would need that signal) — non-trivial and risks false no-ops. Option 1 is recommended.

Whichever is chosen, add an e2e scenario (`internal/e2e/lock_scenarios_test.go`) that reproduces the decompose accidental-double-run and asserts the *documented* exit code, so this cannot regress silently again.

## Minor Issues (Nice to Fix)

### Issue 2: Lock files accumulate indefinitely and are never removed

**Severity**: Minor
**PRD Reference**: §18.5 "Location" / "Mechanism" (flock-based, auto-release on exit — does not specify cleanup).
**Expected Behavior**: Reasonable disk hygiene; lock files should not grow without bound.
**Actual Behavior**: Every distinct repo path ever visited leaves a permanent `<hash>.lock` file in `$XDG_RUNTIME_DIR/stagecoach/locks/` (or the cache fallback). flock auto-releases on process exit, so the files are inert empty shells, but they are never deleted. Observed **210** lock files accumulated during this test session (one per temp repo from the test/e2e runs, plus one per real repo ever used). On a long-lived developer machine this grows unbounded; a CI runner that creates temp repos per job will accumulate them too. Running `stagecoach` in a non-git directory also creates (and leaves) a lock file before the later git operations fail.
**Steps to Reproduce**: Run the e2e suite or any scenario that creates multiple temp repos, then `ls $XDG_RUNTIME_DIR/stagecoach/locks/*.lock | wc -l`.
**Suggested Fix**: Either (a) document that lock files are intentionally left as inert flock targets and are safe to delete, or (b) on `Release()`, attempt `os.Remove(l.path)` (ignoring errors — if another process is blocked on the same fd/path this is harmless because the next `Acquire` recreates it). Removing-on-release is the conventional pattern for flock lock files.

### Issue 3: The contention message names the holder's repo via the non-canonical CWD path

**Severity**: Minor
**PRD Reference**: §18.5 "Contents" (`pid`, `hostname`, the repo path — "diagnostic … let the contention message name *who* holds the lock"); §18.5 "Location" (hash uses the **canonical** path).
**Expected Behavior**: For a repo reached through a symlink, the contention message should identify the repo unambiguously to the contender.
**Actual Behavior**: The lock **hash** is correctly canonicalized via `filepath.EvalSymlinks` (`lockHash`), so two terminals in the same repo (one via a symlink) contend on the same lock file. But the `repo=` field written to the lock file is the raw `repoPath` passed into `Acquire`, which is `os.Getwd()` from `runDefault` — **not** canonicalized (`internal/lock/lock.go` `Acquire`: `repo: repoPath`). The contention message therefore prints the *holder's* raw CWD. If the holder entered via a symlink and the contender via the real path (or vice-versa), the displayed path string differs from the contender's own CWD, which can confuse a user trying to confirm "is that my repo?"
**Steps to Reproduce**: `git init` a repo, symlink it elsewhere, start a blocking `stagecoach` run via the symlink path, then run a contender from the real path (or the reverse). The Busy message's `repo=` will show whichever path the holder used.
**Suggested Fix**: Store the canonicalized path in the `repo=` field (reuse the `canonical` value already computed in `lockHash`, or canonicalize in `Acquire`). The hash and the diagnostic string would then agree.

### Issue 4: `SetSnapshot` rewrite can be observed mid-write by a contender (truncated/partial read)

**Severity**: Minor
**PRD Reference**: §18.5 "Mechanism" / "Contention behavior" (the contender reads the holder's `snapshot=` and `pid`/`hostname`/repo for the message).
**Expected Behavior**: A contender that loses the lock race should always read a complete, well-formed lock file.
**Actual Behavior**: `Locker.setSnapshot` rewrites the lock file as `Truncate(0)` → `Seek(0,0)` → `writeContents(sha)` (`internal/lock/lock.go`). A contender calls `os.ReadFile(path)` (a separate open/read/close) in `Acquire` when `flock` returns `EWOULDBLOCK`. If the contender's read lands between the `Truncate` and the completion of `writeContents`, it reads an empty or partially-written file. `parseContents` then yields empty fields. The functional impact is benign and conservative (empty `snapshot=` → the no-op fast path is skipped → falls through to Busy, which is the safe outcome), but the resulting Busy message can render with empty diagnostics, e.g. *"another stagecoach run is already in progress on  (pid  on )."* — ugly and uninformative.
**Steps to Reproduce**: Hard to hit deterministically (microsecond window) but reachable under load; race-prone by construction since the holder rewrites in place without atomic replacement.
**Suggested Fix**: Write the new contents to a sibling temp file and `os.Rename` over the lock file (atomic on POSIX), or write the full contents into a buffer and `Write` it in a single call after `Truncate`+`Seek` (still not strictly atomic across the truncate/write, but narrows the window to near-zero and avoids the empty-file state). At minimum, guard `handleLockContention` so a contender with an empty `repo=`/`pid`/`hostname` still prints a sensible message.

## Testing Summary

- Total tests performed: ~30 (14 lock unit tests, 5 e2e contention scenarios, full `go test ./...` suite, `go vet`, plus 6+ manual cross-process reproductions with the real binary against fresh repos).
- Passing: all automated tests pass; vet clean.
- Failing: 0 automated (the gap is a behavior-vs-documentation mismatch not covered by any existing test — see Issue 1).
- Areas with good coverage:
  - Lock primitive mechanics: acquire/release idempotency, flock contention → `*HeldError`, auto-release after exit, XDG location resolution (runtime/cache/home/relative-rejected/no-CWD-error), canonical-symlink hashing, `SetSnapshot` method/package/no-op-after-release.
  - Exit code `Busy=5` registration and `exitcode.For` routing.
  - Single-commit no-op fast path and Busy refusal (e2e A & B).
  - No-stale-lock after exit (e2e C), read-only-subcommand bypass (e2e D), dry-run acquires lock (e2e E).
  - All commit-producing entry points funnel through `runDefault` (which acquires the lock); hook mode and read-only subcommands correctly bypass it.
- Areas needing more attention:
  - **Decompose-path contention** — no e2e scenario covers the no-op fast path (or Busy) when the holder is in a decompose run; this is exactly where Issue 1 hides. Add a decompose accidental-double-run scenario.
  - Concurrency-hardening edge cases: lock-file cleanup (Issue 2), canonical `repo=` display (Issue 3), atomic snapshot rewrite (Issue 4) — none currently asserted.
