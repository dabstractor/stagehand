---
name: "P1.M1.T2.S2 — Contention logic, acquire/release wiring in runDefault, and SetSnapshot calls in generate.go + decompose.go"
description: |
  Wire the FR52 per-repo run lock (PRD §18.5) into the commit-producing paths. Three surgical edits +
  one helper + tests:
  (1) `internal/cmd/default_action.go`: after `g := git.New(repoDir)` (line 55), `lock.Acquire(repoDir)`;
      on `*lock.HeldError` call a new `handleLockContention(stderr, held, g, ctx)` helper (no-op fast
      path → exit 0 "nothing to do"; else → exitcode.New(Busy, nil) with the holder message); on other
      error → exitcode.New(Error, …); on success `defer locker.Release()`. One insertion + one defer
      covers BOTH the single-commit path and the decompose path (runDecompose is called from runDefault).
  (2) `internal/generate/generate.go`: add `lock.SetSnapshot(treeSHA)` right after `signal.SetSnapshot(...)`.
  (3) `internal/decompose/decompose.go`: add `lock.SetSnapshot(tStart)` right after FreezeWorkingTree.
  (4) `internal/cmd/lock_contention_test.go` (NEW): unit tests for handleLockContention (no-op / busy-
      tree-differs / busy-empty-snapshot / busy-writetree-error / silent-exit) using a fake git.Git
      (embeds the interface, overrides only WriteTree), plus a wiring test proving the lock is released
      after a normal run. Read-only subcommands + hook mode bypass the lock structurally (no code needed).
  Consumes the COMPLETE `internal/lock` (P1.M1.T1.S1) and the in-progress `exitcode.Busy=5`
  (P1.M1.T2.S1, assume landed). The full two-subprocess contention E2E is P1.M1.T2.S3 (NOT this task).
---

## Goal

**Feature Goal**: Make the FR52 per-repo run lock actually guard the commit-producing runs. Two
stagehand invocations against one repo can no longer race on `update-ref`: the first acquires and
proceeds; the second either exits 0 ("nothing to do — an in-progress run already covers your staged
changes", the accidental-double-run no-op fast path) or exits 5 (Busy, naming the holder's pid/host and
leaving the new changes staged). The holder publishes its frozen snapshot SHA so the fast path can
compare the contender's own `WriteTree` against it.

**Deliverable**:
1. **MODIFY** `internal/cmd/default_action.go` — (a) add `internal/lock` import; (b) insert
   acquire/`defer Release`/contention-handling after `g := git.New(repoDir)` (line 55); (c) add the
   `handleLockContention` helper next to `handleGenError`/`handleDecomposeError`.
2. **MODIFY** `internal/generate/generate.go` — add `internal/lock` import + `lock.SetSnapshot(treeSHA)`
   immediately after `signal.SetSnapshot(treeSHA, parentSHA, "")`.
3. **MODIFY** `internal/decompose/decompose.go` — add `internal/lock` import + `lock.SetSnapshot(tStart)`
   immediately after the `FreezeWorkingTree` error check.
4. **CREATE** `internal/cmd/lock_contention_test.go` (package cmd) — `handleLockContention` unit tests
   (5 cases) + a lock-released-after-run wiring test.

**Success Definition**: `go build/vet/gofmt` clean; `go test -race ./...` green (existing runDefault /
generate / decompose suites unaffected — `lock.SetSnapshot` is a nil-safe no-op without a lock, and each
test repo is unique so no cross-test contention); `handleLockContention` returns exit 0 on the no-op
fast path and exit 5 on busy; a normal commit run releases the lock (a subsequent `lock.Acquire` of the
same repo succeeds); the holder's lock file gains a `snapshot=` line after a run that holds the lock.

## User Persona

**Target User**: (1) End users who accidentally run stagehand twice in two terminals on the same repo
(the common double-run); (2) wrapper scripts / CI that need to distinguish "busy, retry" (exit 5) from a
real failure (1/3/124).

**Use Case**: User has stagehand generating in terminal A (10s agent call). They accidentally hit their
lazygit `stagehand` binding in terminal B. Terminal B either sees its staged set is already covered by
A's snapshot (exit 0, "nothing to do") or sees genuinely new work and exits 5 naming A's pid — leaving
the new work staged for a re-run after A finishes. Neither terminal's HEAD is ever clobbered.

**User Journey**: terminal B → `runDefault` → `lock.Acquire(repoDir)` → `*HeldError` (A holds it) →
`handleLockContention` → reads `held.Contents.Snapshot` → contender `g.WriteTree(ctx)` → match →
"nothing to do" → `exitcode.New(Success, nil)` → main exits 0 (silent). Or mismatch → busy message →
`exitcode.New(Busy, nil)` → main exits 5 (silent).

**Pain Points Addressed**: Eliminates the §13.5 CAS-abort race where the second stagehand to commit
moves HEAD under the first, leaving a dangling snapshot and the confusing "already committed" message.
The lock is the FIRST line of defense (§18.5); the CAS remains the second (defense in depth).

## Why

- **PRD §18.5 (FR52) is the mandate.** The contention-behavior paragraph specifies exactly: no-op fast
  path (holder published `snapshot=` AND contender's `write-tree` is byte-identical → exit 0,
  "nothing to do — an in-progress run already covers your staged changes"); otherwise non-zero exit
  naming pid/host/repo with the changes left staged. §18.5 also mandates: lock is advisory `flock`,
  lives OUTSIDE the repo, `write-tree` for the fast-path probe is "index-read-only and therefore safe
  to take without the lock." This subtask implements that behavior in the CLI layer.
- **PRD §9.9 FR52 / §13.4:** the stage-while-generating workflow is safe for ONE process; the run lock
  makes the two-process race "impossible to stumble into." This wiring is what makes the lock (S1's
  primitive) actually do that.
- **Defense in depth with §18.1/§13.5:** the lock prevents the common local double-run; the CAS
  guarantees never-clobber-HEAD even on a shared filesystem the lock can't cover. Both stay. The lock is
  the first line; this task is the first line's wiring.
- **Unblocks S3 (E2E):** S3's subprocess-level contention scenarios (held→Busy, double-run→0, stale
  lock, bypass) exercise exactly the code this task writes. Without the wiring, S3 has nothing to test.

## What

Three production edits + one helper + one test file:

1. **`runDefault` acquires the lock** right after `g := git.New(repoDir)`, before the auto-stage state
   machine. On `*lock.HeldError` it delegates to `handleLockContention`; on any other acquire error it
   returns `exitcode.New(exitcode.Error, …)`; on success it `defer`s `locker.Release()`. The single
   insertion + single defer cover both the single-commit path and the decompose path (`runDecompose` is
   invoked from `runDefault`).
2. **`handleLockContention(stderr, held, g, ctx)`** implements the no-op fast path (snapshot non-empty
   AND contender `WriteTree` SHA matches → exit 0, silent) and the busy path (everything else → exit 5,
   silent, with the holder message). Both return `exitcode.New(code, nil)` (silent — message already
   printed) so main does not double-print.
3. **`generate.go` and `decompose.go` publish the snapshot** via the package-level nil-safe
   `lock.SetSnapshot(treeSHA)` / `lock.SetSnapshot(tStart)`, right where `signal.SetSnapshot` /
   `FreezeWorkingTree` already capture the frozen tree. This is what makes the no-op fast path possible.
4. **Read-only subcommands and hook mode bypass the lock structurally** — they have their own `RunE`
   and never reach `runDefault` (hook mode writes a message file; git commits). No bypass code needed.

### Success Criteria

- [ ] `internal/cmd/default_action.go` imports `internal/lock`; acquires after `g := git.New(repoDir)`;
      `defer locker.Release()` on success; `*HeldError` → `handleLockContention`; other err → exitcode.Error.
- [ ] `handleLockContention` exists in `default_action.go` with the exact logic in §Blueprint (no-op →
      `exitcode.New(Success,nil)`; busy → `exitcode.New(Busy,nil)`; both silent).
- [ ] `internal/generate/generate.go` has `lock.SetSnapshot(treeSHA)` immediately after `signal.SetSnapshot(...)`.
- [ ] `internal/decompose/decompose.go` has `lock.SetSnapshot(tStart)` immediately after FreezeWorkingTree.
- [ ] `internal/cmd/lock_contention_test.go` exists (package cmd) with the 5 `handleLockContention` cases.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean.
- [ ] `go test -race ./...` green (existing suites unaffected; new tests pass).
- [ ] Read-only subcommands + hook mode unchanged (no lock added there).
- [ ] No change to `internal/lock/*`, `internal/exitcode/*`, `internal/git/*`, or `PRD.md`/`tasks.json`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact insertion points (verified line anchors), the exact
`lock`/`exitcode`/`git.Git` API surface being consumed (with the nil-safe `SetSnapshot` semantics), the
exact `handleLockContention` body, the exact acquire-site code, the exact fake-git test pattern (embed
the interface, override only WriteTree), the exact `*HeldError` test construction, and the test-isolation
gotcha (XDG dir). The three edit anchors are quoted from the live files. No inference required.

### Documentation & References

```yaml
# MUST READ — the binding contract + the authoritative seams
- file: PRD.md
  why: "§18.5 (FR52, the per-repo run lock) — the 'Contention behavior' paragraph is the EXACT spec for
        handleLockContention: no-op fast path (holder snapshot + byte-identical write-tree → exit 0
        'nothing to do — an in-progress run already covers your staged changes'); otherwise non-zero
        naming pid/host/repo, changes left staged. §18.5 also: write-tree is 'index-read-only and
        therefore safe to take without the lock'; lock is advisory flock, never force-break. §18.1/§13.5
        = the CAS that is the second line of defense (this lock is the first). §13.4 = stage-while-generating."
  critical: "This subtask owns ONLY the wiring (runDefault acquire/release + helper) + the two SetSnapshot
             calls + unit tests. The lock PRIMITIVE is P1.M1.T1.S1 (COMPLETE — do not touch internal/lock);
             the Busy CONSTANT is P1.M1.T2.S1 (parallel — assume exitcode.Busy==5 lands); the full
             subprocess E2E is P1.M1.T2.S3 (NOT this task). Do NOT add the lock to hookexec or read-only
             subcommands (see §What)."

- docfile: plan/006_c23e6f286ae7/architecture/integration_seams.md
  why: "§1 is the authoritative spec for the runDefault edit: the exact insertion point (after
        g:=git.New(repoDir) line 55, before the auto-stage state machine line 73), the confirmation that
        ONE insertion + ONE defer covers both single-commit and decompose paths, and that read-only
        subcommands bypass naturally. §3 (generate.go) and §4 (decompose.go) are the exact SetSnapshot
        anchors. §7 sketches the E2E contention test pattern (that is S3, not S2 — informational here)."
  critical: "§1 used the placeholder name 'lock.HandleAcquireError'; the ITEM CONTRACT (§3.a) overrides
             it: the helper is a LOCAL cmd-package function 'handleLockContention(stderr, heldErr, g, ctx)'
             — follow the item contract, not the seams placeholder. §1 confirms snapshot= is published by
             the LIBRARY (generate/decompose), NOT runDefault — runDefault does not know the tree SHA."

- docfile: plan/006_c23e6f286ae7/P1M1T1S1/PRP.md
  why: "The CONTRACT for the lock API S2 consumes: Acquire(repoPath)(*Locker,error) returns *HeldError on
        contention; Release() idempotent; package-level SetSnapshot(sha) nil-safe no-op when no lock held;
        HeldError{Contents LockContents, Path string}; LockContents{Pid,Hostname,Repo,Timestamp,Snapshot};
        IsHeldError(err). The SetSnapshot singleton mirrors internal/signal.active."
  critical: "Treat internal/lock as COMPLETE and READ-ONLY. S2 only CALLS it. The package-level SetSnapshot
             is the bridge library layers call unconditionally (nil-safe)."

- docfile: plan/006_c23e6f286ae7/P1M1T2S1/PRP.md
  why: "The CONTRACT for exitcode.Busy (=5, after Rescue before Timeout) and the silent-exit pattern
        New(code,nil)→Error()==''→For()==code (the *ExitError short-circuit). handleLockContention reuses
        this EXACT pattern (New(Success,nil) and New(Busy,nil)) so main does not double-print."
  critical: "S1 and S2 run in PARALLEL. S1 owns docs/cli.md (the 5/Busy row + contention note). S2 must NOT
             edit docs/cli.md (conflict risk) — only verify the note exists as a gate. exitcode.Busy is the
             symbol S2 returns; assume it lands as specified."

- file: internal/lock/lock.go
  why: "THE API being consumed. Read in full: Acquire, Release, SetSnapshot (method + package-level),
        HeldError, LockContents, IsHeldError. Confirms the package-level SetSnapshot is nil-safe when
        current==nil (so generate/decompose tests with no lock are unaffected)."
  pattern: "Acquire returns (*Locker, error); on contention the error IS the *HeldError (errors.As
            recovers it). The singleton (current atomic.Pointer[Locker]) is set on Acquire, cleared on
            Release — so package-level SetSnapshot reaches the live lock."
  gotcha: "lockDir() resolves XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache/stagehand/locks (NO CWD fallback
           — a lock in the repo is the §18.5 anti-pattern). Tests must isolate these env vars (see §Gotchas)."

- file: internal/cmd/default_action.go
  why: "EDIT TARGET #1. Line 55 (g:=git.New(repoDir)) is the insertion point; line 73 starts the auto-stage
        state machine; runDecompose (line ~110) is the decompose entry — all under ONE defer. handleGenError
        /handleDecomposeError (lines ~260/300) are the existing handle* helpers — handleLockContention goes
        beside them. Imports context/errors/fmt/io/os already present; 'lock' is NOT (add it)."
  pattern: "handle* helpers print the detailed message to stderr, then return a SILENT exitcode.New(code,
            nil) so main does not double-print. handleLockContention mirrors this exactly."
  gotcha: "stderr (cmd.ErrOrStderr()) and ctx (cmd.Context()) are already captured at the top of runDefault —
           pass them to handleLockContention. repoDir and g are in scope at the insertion point."

- file: internal/generate/generate.go
  why: "EDIT TARGET #2. Inside CommitStaged, after Step 3 WriteTree: 'signal.SetSnapshot(treeSHA, parentSHA,
        \"\")' is the anchor — add 'lock.SetSnapshot(treeSHA)' on the next line. CommitStaged is called by
        pkg/stagehand.GenerateCommit (public API) AND runDefault (dogfood) — one site covers both."
  gotcha: "Add the 'internal/lock' import. The call is a no-op when no lock is held (library use, tests)."

- file: internal/decompose/decompose.go
  why: "EDIT TARGET #3. Inside Decompose, step 3: after 'tStart, err := deps.Git.FreezeWorkingTree(...)' +
        its error check, add 'lock.SetSnapshot(tStart)' BEFORE the one-file short-circuit. tStart is the
        decompose equivalent of the single-commit treeSHA."
  gotcha: "The runSingleEscape path (Single||Commits==1, line ~143) returns BEFORE the freeze — it routes
           to the v1 generate path where generate.go's SetSnapshot fires. So both escape routes each have
           exactly one SetSnapshot; do NOT add a second one in runSingleEscape/runSingleShortcut."

- file: internal/signal/signal.go
  why: "READ-ONLY reference. signal.SetSnapshot + the `active` singleton is the EXACT pattern the lock
        package mirrors (lock.current / lock.SetSnapshot). Reading it confirms the nil-safe singleton
        idiom S2 relies on: library layers call SetSnapshot unconditionally; it no-ops when nothing is
        installed/held."
  critical: "Do NOT edit signal.go. It is the model, not the target."

- file: internal/cmd/default_action_test.go
  why: "Test conventions: tests invoke the REAL cobra command via Execute(ctx) against a t.TempDir() repo
        (setupStubRepo / setupStubRepoRaw) with a stub agent, asserting on exitcode.For(err) + stdout/stderr
        buffers. isolateHome(t) sets HOME + XDG_CONFIG_HOME. Reuse these helpers for the wiring test."
  gotcha: "isolateHome does NOT set XDG_RUNTIME_DIR / XDG_CACHE_HOME — set them explicitly in lock tests so
           the lock dir lands in the test's temp tree (see §Gotchas G7)."

# External references (exact, anchor-level)
- url: https://pkg.go.dev/os/signal
  why: "(context only) the lock + signal singletons are both process-global atomic.Pointer patterns; the
        nil-safe wrapper idiom is standard Go for optional global state."
- url: https://git-scm.com/docs/git-write-tree
  why: "Confirms write-tree reads the index and writes a tree object (no ref mutation) — the reason the
        contender's WriteTree probe is safe WITHOUT holding the lock (PRD §18.5)."
```

### Current Codebase Tree (relevant slice — after P1.M1.T1.S1 COMPLETE, P1.M1.T2.S1 in progress)

```bash
stagehand/
└── internal/
    ├── cmd/
    │   ├── default_action.go        # EDIT: +lock import, acquire/defer/handleLockContention
    │   ├── default_action_test.go   # (existing; reuse helpers; optionally +1 wiring test)
    │   └── ... (root.go, hookexec.go [own RunE — NO lock], providers.go, config.go, ...)
    ├── decompose/
    │   └── decompose.go             # EDIT: +lock import, lock.SetSnapshot(tStart)
    ├── exitcode/
    │   └── exitcode.go              # P1.M1.T2.S1 adds Busy=5 (CONSUME; do not edit)
    ├── generate/
    │   └── generate.go              # EDIT: +lock import, lock.SetSnapshot(treeSHA)
    ├── git/
    │   └── git.go                   # git.Git.WriteTree (CONSUME; do not edit)
    └── lock/
        ├── lock.go                  # P1.M1.T1.S1 COMPLETE (CONSUME; do not edit)
        └── lock_test.go             # (reference: HeldError construction, contention test pattern)
```

### Desired Codebase Tree After This Subtask

```bash
stagehand/
└── internal/
    ├── cmd/
    │   ├── default_action.go        # MODIFIED: +lock import; acquire/defer; +handleLockContention helper
    │   └── lock_contention_test.go  # NEW: package cmd; 5 handleLockContention unit cases + wiring test
    ├── decompose/
    │   └── decompose.go             # MODIFIED: +lock import; +lock.SetSnapshot(tStart)
    └── generate/
        └── generate.go              # MODIFIED: +lock import; +lock.SetSnapshot(treeSHA)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/cmd/default_action.go` | MODIFY | +`internal/lock` import; acquire/`defer Release`/contention handling after `g:=git.New(repoDir)`; +`handleLockContention` helper. |
| `internal/cmd/lock_contention_test.go` | CREATE | `package cmd`; `handleLockContention` unit tests (5 cases) via a fake git.Git (embeds interface, overrides WriteTree); +lock-released-after-run wiring test. |
| `internal/generate/generate.go` | MODIFY | +`internal/lock` import; +`lock.SetSnapshot(treeSHA)` after `signal.SetSnapshot(...)`. |
| `internal/decompose/decompose.go` | MODIFY | +`internal/lock` import; +`lock.SetSnapshot(tStart)` after FreezeWorkingTree. |

**Explicitly NOT touched:** `internal/lock/*` (S1 primitive, COMPLETE), `internal/exitcode/*` (S1
constant, parallel), `internal/git/*`, `internal/cmd/hookexec.go` + read-only subcommands (bypass
structurally), `docs/cli.md` (S1 owns the Busy row — conflict risk; verify only),
`docs/how-it-works.md` (S1 added the concurrency subsection), `README.md` (S3), `PRD.md`,
`tasks.json`, `prd_snapshot.md`, anything under `plan/`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — ONE insertion + ONE defer covers BOTH commit paths): place lock.Acquire AFTER
// g:=git.New(repoDir) (line 55) and BEFORE the auto-stage state machine (line 73). The decompose path
// is reached via runDecompose (called from runDefault ~line 110), so it is under the same defer.
// defer locker.Release() fires on EVERY early-exit (nothing-to-commit 2, dry-run 0, CAS 1, rescue 3,
// push-fail 1) and on success. Do NOT add a second acquire in runDecompose.

// CRITICAL (G2 — use errors.As, not a type-assert, not IsHeldError): `var held *lock.HeldError;
// if errors.As(lockErr, &held)` matches the codebase-wide convention (handleGenError/handleDecomposeError)
// and yields the typed *held the helper needs. lock.IsHeldError returns only bool (insufficient). A raw
// type-assertion works today (Acquire returns *HeldError directly) but errors.As is wrapsafe + idiomatic.

// CRITICAL (G3 — the silent-exit pattern is intentional): handleLockContention returns
// exitcode.New(exitcode.Success, nil) and exitcode.New(exitcode.Busy, nil). Err==nil ⟹ ExitError.Error()==""
// ⟹ main does NOT double-print (the message was already written to stderr). This is the SAME pattern
// handleGenError/handleDecomposeError use for rescue/CAS. Do NOT pass a non-nil err — it causes double-print.

// CRITICAL (G4 — WriteTree is safe WITHOUT the lock): the contender's g.WriteTree(ctx) probe is the
// no-op fast path. write-tree reads the index and writes ONE tree object (a harmless dangling object if
// the run bails) — it touches NO ref. PRD §18.5: "write-tree, which is index-read-only and therefore safe
// to take without the lock." Verified: git.Git.WriteTree(ctx)(sha,error) exists (git.go:65).

// GOTCHA (G5 — WriteTree error → Busy, NOT a separate error): if the contender's index has unresolved
// merge conflicts, WriteTree errors. The holder still holds the lock, so the contender cannot proceed.
// handleLockContention falls through to Busy (the contender re-runs later; the real conflict error then
// surfaces when the lock is free and actionable). Do NOT surface the WriteTree error mid-contention.

// GOTCHA (G6 — do NOT add the lock to hookexec or read-only subcommands): hookexec.go has its own RunE
// (runHookExec) and only WRITES a commit message to a msg-file (git performs the commit) — it is NOT a
// ref-mutating action, so it needs no lock. config/providers/models/--version have their own RunE and
// never reach runDefault — the bypass is structural. Adding acquire/release there is dead code + scope creep.

// GOTCHA (G7 — test lock-dir isolation): isolateHome(t) sets HOME + XDG_CONFIG_HOME but NOT
// XDG_RUNTIME_DIR or XDG_CACHE_HOME. lock.lockDir() resolves XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache.
// In lock tests, explicitly t.Setenv("XDG_CACHE_HOME", t.TempDir()) AND t.Setenv("XDG_RUNTIME_DIR", "")
// so the lock lands in the test's temp tree (can't collide with a real user's locks or other tests).
// Each test's t.TempDir() repo is unique, so repo-key collisions across tests don't happen regardless.

// GOTCHA (G8 — fake git.Git via interface embedding): to unit-test handleLockContention without a real
// repo AND without faking ~15 git.Git methods, embed the interface and override only WriteTree:
//   type contentionFakeGit struct { git.Git; writeTreeSHA string; writeTreeErr error }
//   func (f *contentionFakeGit) WriteTree(context.Context)(string,error){ return f.writeTreeSHA,f.writeTreeErr }
// Any uncalled method is nil (panics if invoked) — but the helper calls ONLY WriteTree. This is the
// standard Go idiom for faking a large interface for one method.

// GOTCHA (G9 — existing suites stay green by design): lock.SetSnapshot is a nil-safe no-op when no lock
// is held (lock.current==nil), which is the case in all existing generate/decompose tests. So adding the
// two SetSnapshot call sites changes ZERO existing behavior. The runDefault acquire/release is the only
// behavioral change, and each test's repo is unique (no contention). Gate: go test -race ./... stays green.

// GOTCHA (G10 — docs/cli.md is S1's, not S2's): S1 (parallel) owns the 5/Busy exit-code row + the
// contention note. S2 must NOT edit docs/cli.md (parallel-edit conflict). S2 only VERIFIES the note is
// present as a post-merge gate. If S1 somehow didn't add it, flag rather than edit (coordinate).

// GOTCHA (G11 — the helper is in default_action.go, not a new file): handleGenError/handleDecomposeError/
// runPush live in default_action.go. handleLockContention follows that convention (keeps handle* helpers
// together). The TESTS go in a new lock_contention_test.go (keeps the big default_action_test.go focused).
```

## Implementation Blueprint

### Data models and structure

No new types in production code. The helper consumes the existing `*lock.HeldError` /
`lock.LockContents` (S1) and `git.Git` / `exitcode` symbols. The only new type is the test-only
`contentionFakeGit` (in the test file).

### The `handleLockContention` helper (exact — copy into default_action.go)

```go
// handleLockContention implements the FR52 / §18.5 contention behavior when lock.Acquire returns a
// *lock.HeldError. It never blocks and never force-breaks the lock. No-op fast path: if the holder
// published a snapshot AND the contender's own write-tree (index-read-only, safe without the lock) is
// byte-identical, nothing new is staged → exit 0 ("nothing to do…"). Otherwise → exit Busy (5) naming
// the holder, leaving the contender's new changes staged. Both returns are SILENT (message already
// printed to stderr) so main does not double-print — same pattern as handleGenError/handleDecomposeError.
func handleLockContention(stderr io.Writer, heldErr *lock.HeldError, g git.Git, ctx context.Context) error {
	// No-op fast path (§18.5): holder published snapshot= and the contender's index matches it.
	if snap := heldErr.Contents.Snapshot; snap != "" {
		contenderTree, werr := g.WriteTree(ctx) // index-read-only + one harmless dangling tree (G4)
		if werr == nil && contenderTree == snap {
			fmt.Fprintln(stderr, "nothing to do — an in-progress run already covers your staged changes.")
			return exitcode.New(exitcode.Success, nil) // exit 0, SILENT
		}
		// werr != nil (e.g. merge conflicts) or SHAs differ → fall through to Busy (G5).
	}
	fmt.Fprintf(stderr,
		"stagehand: another stagehand run is already in progress on %s (pid %s on %s). "+
			"Your newly-staged changes will remain staged — re-run stagehand after it finishes. Lock: %s.\n",
		heldErr.Contents.Repo, heldErr.Contents.Pid, heldErr.Contents.Hostname, heldErr.Path)
	return exitcode.New(exitcode.Busy, nil) // exit 5, SILENT
}
```

### The acquire-site code (exact — insert in runDefault after `g := git.New(repoDir)`)

```go
	g := git.New(repoDir)

	// FR52 / PRD §18.5: acquire the per-repo run lock so two stagehand processes cannot race on
	// update-ref. One acquire + one defer covers BOTH the single-commit path and the decompose path
	// (runDecompose is called below). Read-only subcommands never reach runDefault; hook mode only
	// writes a message (git commits) — neither needs the lock.
	locker, lockErr := lock.Acquire(repoDir)
	if lockErr != nil {
		var held *lock.HeldError
		if errors.As(lockErr, &held) { // contention → no-op fast path (0) or Busy (5), both silent
			return handleLockContention(stderr, held, g, ctx)
		}
		return exitcode.New(exitcode.Error, fmt.Errorf("acquire run lock: %w", lockErr))
	}
	defer locker.Release()

	// ---- §9.4 auto-stage-all state machine (FR16–FR20) ----
```

### The two SetSnapshot one-liners (exact)

```go
// generate.go — right after: signal.SetSnapshot(treeSHA, parentSHA, "") // arm rescue (§18.4)
lock.SetSnapshot(treeSHA) // publish frozen index tree for the FR52 no-op fast path (nil-safe: no-op w/o lock)

// decompose.go — right after the FreezeWorkingTree error-check block (tStart in scope)
lock.SetSnapshot(tStart) // publish frozen working-tree snapshot for the FR52 no-op fast path (nil-safe)
```

Both files need `import "github.com/dustin/stagehand/internal/lock"` added.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/cmd/default_action.go — acquire/release + helper
  - EDIT 1a — add the import: "github.com/dustin/stagehand/internal/lock" (alphabetical with the other
    github.com/dustin/stagehand/internal/* imports).
  - EDIT 1b — INSERT the acquire block (§"acquire-site code") immediately AFTER `g := git.New(repoDir)`
    (line 55) and BEFORE the `// ---- §9.4 auto-stage-all state machine` comment (line 73). Use the
    captured `stderr` (cmd.ErrOrStderr()) and `ctx` (cmd.Context()) already at the top of runDefault.
  - EDIT 1c — ADD the `handleLockContention` function (§"handleLockContention helper") next to
    handleGenError/handleDecomposeError (e.g. right before handleGenError).
  - DO NOT: add a second acquire in runDecompose; touch hookexec.go or read-only subcommands; edit
    For()/exitcode/lock/git.
  - VERIFY: go build ./internal/cmd/ → exit 0.

Task 2: EDIT internal/generate/generate.go — publish snapshot (single-commit path)
  - EDIT 2a — add import "github.com/dustin/stagehand/internal/lock".
  - EDIT 2b — on the line AFTER `signal.SetSnapshot(treeSHA, parentSHA, "") // arm rescue (§18.4)`,
    add `lock.SetSnapshot(treeSHA)` (with the comment above).
  - DO NOT add a second SetSnapshot elsewhere in generate.go.
  - VERIFY: go build ./internal/generate/ → exit 0.

Task 3: EDIT internal/decompose/decompose.go — publish snapshot (decompose path)
  - EDIT 3a — add import "github.com/dustin/stagehand/internal/lock".
  - EDIT 3b — AFTER the FreezeWorkingTree error-check block (`if err != nil { return ... }`) and BEFORE
    the one-file short-circuit comment, add `lock.SetSnapshot(tStart)` (with the comment above).
  - DO NOT add SetSnapshot in runSingleEscape/runSingleShortcut (the freeze — and thus this publish —
    happens once, before those branches; the single-escape path routes to generate.go's publish).
  - VERIFY: go build ./internal/decompose/ → exit 0.

Task 4: CREATE internal/cmd/lock_contention_test.go — unit tests for the helper + a wiring test
  - FILE: internal/cmd/lock_contention_test.go ; PACKAGE: cmd (white-box — reuses rootCmd/Execute/
    setupStubRepo/isolateHome from default_action_test.go; same package, no redeclaration).
  - IMPORTS: context, errors, fmt, strings, testing + internal/exitcode, internal/git, internal/lock.
  - DEFINE the fake: type contentionFakeGit struct { git.Git; writeTreeSHA string; writeTreeErr error }
    + func (f *contentionFakeGit) WriteTree(context.Context) (string,error) { return f.writeTreeSHA, f.writeTreeErr }.
  - WRITE 5 unit cases (each builds a *lock.HeldError directly + a contentionFakeGit, calls
    handleLockContention(&buf, held, g, ctx), asserts exitcode.For(err) + err.Error()=="" + buf text):
      TestHandleLockContention_NoOpFastPath:    snap="abc123", WriteTree→("abc123",nil) → For==Success(0); buf has "nothing to do"; err.Error()=="".
      TestHandleLockContention_Busy_TreeDiffers: snap="abc123", WriteTree→("zzz",nil)   → For==Busy(5);  buf has busy msg + "4242" + "testhost"; err.Error()=="".
      TestHandleLockContention_Busy_EmptySnapshot: snap=""                            → For==Busy(5);  buf has busy msg (fast path skipped).
      TestHandleLockContention_Busy_WriteTreeErr: snap="abc123", WriteTree→("",errors.New("merge")) → For==Busy(5) (falls through, G5).
      TestHandleLockContention_SilentExits: for both Success and Busy returns, err.Error()=="" (no double-print in main).
  - WRITE 1 wiring test: TestRunDefault_LockReleasedAfterRun — isolateHome + t.Setenv("XDG_CACHE_HOME",
    t.TempDir()) + t.Setenv("XDG_RUNTIME_DIR",""); setupStubRepo("feat: x"); stage a file;
    Execute(ctx) succeeds (exit 0); THEN lock.Acquire(repoDir) must succeed (not contend) → proves
    defer Release fired. (Reuse setupStubRepo's repo dir — capture it; or newRepo-style temp.)
  - VERIFY: go test -race ./internal/cmd/ -run 'HandleLockContention|LockReleased' -v → all PASS.

Task 5: (OPTIONAL/RECOMMENDED) SetSnapshot-published proof
  - ADD to generate_test.go (package generate) OR decompose_test.go: acquire the lock for the test's
    repo (t.Setenv XDG isolation as in G7), run CommitStaged/Decompose with a stub agent, then read the
    lock file (lockPath(repo)) and assert the `snapshot=` line is non-empty (a 40-hex SHA). This proves
    the Task 2/3 one-liners are live. Keep it lightweight; the full two-process contention is S3.

Task 6: VALIDATE — full gate set + scope discipline
  - RUN: go build ./... ; go vet ./... ; gofmt -l . ; go test -race ./...
  - RUN: git grep -n 'lock.Acquire\|handleLockContention' internal/cmd/default_action.go  (expect: the acquire + the helper def + the call)
  - RUN: git grep -n 'lock.SetSnapshot' internal/generate/generate.go internal/decompose/decompose.go  (expect: one match each)
  - RUN: git diff --stat → expect ONLY default_action.go, generate.go, decompose.go (modified) +
    lock_contention_test.go (new) [± generate_test.go/decompose_test.go if Task 5 done].
  - RUN (gate, not edit): grep -n 'busy, retry\|nothing to do' docs/cli.md  (expect: present — S1's note).
```

### Implementation Patterns & Key Details

```go
// === Why the helper returns SILENT exits (G3) ===
// handleLockContention writes the user-facing message to stderr ONCE, then returns exitcode.New(code, nil).
// main.go calls exitcode.For(err) → code; because ExitError{Err:nil}.Error()=="", main's "err.Error() != """
// guard skips printing. A non-nil err would make main print "stagehand: <msg>" AGAIN. This is identical
// to handleGenError's rescue/CAS branches and handleDecomposeError's silent mapping.

// === Why errors.As over IsHeldError (G2) ===
// lock.IsHeldError(err) returns bool — useful for a yes/no, but the helper needs the *HeldError value
// (Contents.Snapshot/Pid/Hostname/Repo + Path). `var held *lock.HeldError; errors.As(lockErr, &held)`
// recovers it in one call and is the wrapsafe, codebase-standard form (handleGenError/handleDecomposeError
// both use errors.As for *RescueError/*CASError).

// === Why WriteTree is the fast-path probe (G4) ===
// The contender must answer "is my staged set already covered by the holder's snapshot?" without mutating
// anything. `git write-tree` reads the index and emits ONE tree object (dangling if the run bails —
// harmless). It touches no ref. PRD §18.5 explicitly blesses this: "write-tree, which is index-read-only
// and therefore safe to take without the lock." Comparing the resulting SHA to held.Contents.Snapshot is
// a byte-exact equivalence check (same tree ⟹ same index content).

// === Why the decompose SetSnapshot goes after FreezeWorkingTree, not WriteTree ===
// On the decompose path the frozen snapshot is T_start (the WHOLE working-tree change set, captured by
// FreezeWorkingTree = AddAll + WriteTree + ReadTree-to-base). T_start is what the planner/loop draw from
// and what a contender's re-run would re-freeze. Publishing T_start (not some intermediate) makes the
// no-op comparison meaningful. The single-commit path publishes treeSHA (the plain WriteTree result) —
// the equivalent frozen snapshot for that path.

// === Why one defer covers both paths (G1) ===
// runDecompose is INVOKED from runDefault (line ~110), not as a separate cobra RunE. So control returns
// to runDefault before the defer unwinds — the defer fires whether the run took the single-commit or the
// decompose branch. There is no second cobra entry point to wire.
```

### Integration Points

```yaml
LOCK (consumed — internal/lock, COMPLETE):
  - lock.Acquire(repoDir) → (*Locker, error); *HeldError on contention
  - (*Locker).Release() — idempotent; defer'd in runDefault
  - lock.SetSnapshot(sha) — package-level, nil-safe; called from generate.go + decompose.go

EXITCODE (consumed — internal/exitcode, P1.M1.T2.S1 parallel):
  - exitcode.Busy (==5), exitcode.Success (==0)
  - exitcode.New(code, nil) — silent-exit; exitcode.For(err) resolves via the *ExitError short-circuit

GIT (consumed — internal/git):
  - git.Git.WriteTree(ctx) (sha string, err error) — the contender's fast-path probe

CMD (modified here):
  - internal/cmd/default_action.go: +lock import; acquire/defer/handleLockContention in runDefault; +helper
  - internal/cmd/lock_contention_test.go: NEW (package cmd) — 5 unit cases + wiring test

LIBRARY (modified here — publish snapshot):
  - internal/generate/generate.go: +lock.SetSnapshot(treeSHA) after signal.SetSnapshot
  - internal/decompose/decompose.go: +lock.SetSnapshot(tStart) after FreezeWorkingTree

BYPASS (structural — NO code):
  - read-only subcommands (config/providers/models/--version/--help): own RunE, never reach runDefault
  - hook mode (hookexec.go runHookExec): writes a msg-file; git commits — not a ref mutator

NO-TOUCH (explicitly — owned by siblings):
  - internal/lock/* (P1.M1.T1.S1 COMPLETE), internal/exitcode/* (P1.M1.T2.S1 parallel)
  - docs/cli.md (S1 owns the Busy row + contention note — verify only, G10)
  - docs/how-it-works.md (S1's concurrency subsection), README.md (S3)
  - PRD.md, tasks.json, prd_snapshot.md, plan/*
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand

gofmt -l internal/cmd/default_action.go internal/cmd/lock_contention_test.go internal/generate/generate.go internal/decompose/decompose.go
# Expected: empty (run gofmt -w on any listed file).

go vet ./internal/cmd/... ./internal/generate/... ./internal/decompose/...
# Expected: exit 0 (no unused import, no shadowing — note: 'locker' MUST be used by defer Release).

go build ./...
# Expected: exit 0. If `undefined: lock`, you forgot the import in that file.
# If `locker declared and not used`, you forgot `defer locker.Release()`.
```

### Level 2: Unit Tests (the helper + wiring)

```bash
cd /home/dustin/projects/stagehand

# The new helper unit tests (fast, in-process — no real git needed):
go test -race ./internal/cmd/ -v -run 'HandleLockContention|LockReleased'
# Expected: TestHandleLockContention_NoOpFastPath, _Busy_TreeDiffers, _Busy_EmptySnapshot,
#           _Busy_WriteTreeErr, _SilentExits, TestRunDefault_LockReleasedAfterRun — all PASS.

# (If Task 5 done) the SetSnapshot-published proof:
go test -race ./internal/generate/ -v -run 'SetSnapshot'   # or ./internal/decompose/
```

### Level 3: Whole-Repository Regression (no collateral)

```bash
cd /home/dustin/projects/stagehand

go test -race ./...     # Expected: ALL packages green. Existing runDefault/generate/decompose suites
                        # are unaffected: lock.SetSnapshot is a nil-safe no-op without a lock (G9), and
                        # each test's t.TempDir() repo is unique (no cross-test contention).

go vet ./...            # Expected: exit 0.

# Confirm ONLY in-scope files changed:
git diff --stat -- internal/cmd/default_action.go internal/generate/generate.go internal/decompose/decompose.go
git status --porcelain -- internal/cmd/lock_contention_test.go
# Expected: the 3 modified files + the 1 new test file (± generate_test.go/decompose_test.go if Task 5).

# Confirm sibling territories UNTOUCHED:
git diff --stat -- internal/lock/ internal/exitcode/ internal/git/ internal/cmd/hookexec.go docs/cli.md docs/how-it-works.md README.md
# Expected: EMPTY.

# Confirm the wiring is present (greppable):
git grep -n 'lock.Acquire(repoDir)' internal/cmd/default_action.go            # → the acquire line
git grep -n 'func handleLockContention' internal/cmd/default_action.go        # → the helper def
git grep -n 'lock.SetSnapshot' internal/generate/generate.go internal/decompose/decompose.go  # → 1 each
```

### Level 4: Behavioral Smoke Test (prove the contention logic end-to-end in-process)

```bash
cd /home/dustin/projects/stagehand

# Throwaway main: proves handleLockContention's two outcomes resolve to the right exit codes via
# exitcode.For (the exact path runDefault→main uses). Requires exitcode.Busy (P1.M1.T2.S1) landed.
cat > /tmp/sh_lock_check.go <<'EOF'
package main
import ("bytes";"context";"fmt"
  "github.com/dustin/stagehand/internal/cmd"   // NOTE: cmd is internal — run from module root
  _ "github.com/dustin/stagehand/internal/exitcode"
  "github.com/dustin/stagehand/internal/git"
  "github.com/dustin/stagehand/internal/lock")
type fg struct{ git.Git; sha string }
func (f *fg) WriteTree(context.Context)(string,error){ return f.sha, nil }
func main(){
  // no-op fast path
  var b bytes.Buffer
  held := &lock.HeldError{Contents: lock.LockContents{Pid:"1",Hostname:"h",Repo:"/r",Snapshot:"ABC"}, Path:"/x.lock"}
  // handleLockContention is unexported? — it is package cmd. Run via a tiny cmd test instead (see Level 2).
  fmt.Println(b.String(), held, context.Background())
}
EOF
# NOTE: handleLockContention is unexported (package cmd), so a /tmp main can't call it directly. The
# authoritative proof is the in-package unit test (Level 2). The behavioral guarantee at the binary level
# (two real stagehand subprocesses → exit 0 / 5) is P1.M1.T2.S3's E2E. Delete the scratch file:
rm -f /tmp/sh_lock_check.go

# Instead, the in-process proof is the unit test (run it verbose — it exercises both branches):
go test -race ./internal/cmd/ -v -run 'HandleLockContention'
# Expected: For(Success-path err)==0 and For(Busy-path err)==5, both with err.Error()=="".
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green (existing suites unaffected; new tests pass).

### Feature Validation

- [ ] `runDefault` acquires the lock after `g := git.New(repoDir)` and `defer`s `Release()` on success.
- [ ] `*lock.HeldError` → `handleLockContention`; other acquire error → `exitcode.New(exitcode.Error, …)`.
- [ ] `handleLockContention`: snapshot match → `exitcode.New(exitcode.Success, nil)` + "nothing to do…";
      else → `exitcode.New(exitcode.Busy, nil)` + holder message (pid/host/repo/lock path).
- [ ] `generate.go` has `lock.SetSnapshot(treeSHA)` after `signal.SetSnapshot(...)`.
- [ ] `decompose.go` has `lock.SetSnapshot(tStart)` after FreezeWorkingTree.
- [ ] A normal commit run releases the lock (wiring test: subsequent `lock.Acquire` succeeds).
- [ ] Both Success and Busy helper returns are silent (`err.Error()==""`).

### Scope Discipline Validation

- [ ] ONLY `default_action.go`, `generate.go`, `decompose.go` (modified) + `lock_contention_test.go` (new).
- [ ] Did NOT touch `internal/lock/*`, `internal/exitcode/*`, `internal/git/*`.
- [ ] Did NOT add the lock to `hookexec.go` or read-only subcommands (bypass is structural — G6).
- [ ] Did NOT edit `docs/cli.md` (S1 owns it — G10; verify the note exists as a gate only).
- [ ] Did NOT build the full two-subprocess contention E2E (that is P1.M1.T2.S3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] `handleLockContention` placed next to `handleGenError`/`handleDecomposeError` (codebase convention).
- [ ] Uses `errors.As` (not type-assert / `IsHeldError`) for the `*HeldError` extraction (G2).
- [ ] Messages use the em-dash `—` (matches the codebase — e.g. the FR18 auto-stage notice).
- [ ] Test fake embeds `git.Git` and overrides only `WriteTree` (G8) — no ~15-method fake.
- [ ] Lock tests isolate `XDG_CACHE_HOME`/`XDG_RUNTIME_DIR` (G7).

---

## Anti-Patterns to Avoid

- ❌ Don't add a SECOND lock acquire in `runDecompose` — it's called from `runDefault`, so the single
  acquire + defer there already covers it. Two acquires would self-deadlock or double-release (G1).
- ❌ Don't use a type assertion (`lockErr.(*lock.HeldError)`) or `lock.IsHeldError` for the contention
  branch — use `errors.As(lockErr, &held)` (codebase convention, wrapsafe, yields the typed value) (G2).
- ❌ Don't pass a non-nil `err` to `exitcode.New` in the helper — the silent-exit (`New(code, nil)`) is
  intentional so main doesn't double-print the message you already wrote to stderr (G3).
- ❌ Don't surface the contender's `WriteTree` error as a separate exit — fall through to Busy; the real
  error surfaces on re-run when the lock is free and actionable (G5).
- ❌ Don't add the lock to `hookexec.go` (hook mode writes a message; git commits — not a ref mutator) or
  to read-only subcommands (they have their own `RunE`, never reach `runDefault`) (G6).
- ❌ Don't edit `docs/cli.md` — S1 (parallel) owns the Busy row + contention note; editing risks a
  conflict. Verify the note exists as a gate; flag if missing (G10).
- ❌ Don't fake all ~15 `git.Git` methods in the test — embed the interface and override only `WriteTree`
  (the sole method the helper calls) (G8).
- ❌ Don't forget XDG isolation in lock tests — `isolateHome` doesn't set `XDG_CACHE_HOME`/
  `XDG_RUNTIME_DIR`; set them to a temp dir / empty so the lock lands in the test's tree (G7).
- ❌ Don't add `lock.SetSnapshot` in `runSingleEscape`/`runSingleShortcut` — the freeze (and thus the one
  publish) happens once before those branches; the single-escape path routes through `generate.go`'s
  publish.
- ❌ Don't build the two-subprocess contention E2E — that is P1.M1.T2.S3. S2 is the wiring + helper +
  unit tests.
- ❌ Don't modify `internal/lock/*` (COMPLETE primitive), `internal/exitcode/*` (parallel S1),
  `internal/git/*`, `PRD.md`, `tasks.json`, or `plan/*`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a well-bounded wiring task with every edit anchored to verified live line numbers and
every consumed API (`lock.Acquire`/`Release`/`SetSnapshot`/`HeldError`, `exitcode.New`/`For`, `git.Git.WriteTree`)
quoted from the actual source. The `handleLockContention` body, the acquire-site block, and the two
SetSnapshot one-liners are specified verbatim. The two most likely mistakes — (a) double-printing by
passing a non-nil err (G3) and (b) adding the lock to hookexec/read-only subcommands (G6) — are
front-loaded as CRITICAL gotchas. The test strategy (embed-interface fake + direct `*HeldError`
construction) is minimal and robust. Existing suites stay green by construction (`lock.SetSnapshot` is a
nil-safe no-op without a lock; unique test repos prevent contention — G9). The residual uncertainty is
the parallel dependency on `exitcode.Busy` (P1.M1.T2.S1): if that constant isn't yet present at build
time, `go build` fails with `undefined: exitcode.Busy` — a clear, single-line signal resolved when S1
lands. The `docs/cli.md` non-edit (G10) avoids the parallel-edit conflict with S1. The full contention
E2E is correctly deferred to S3, keeping S2's scope tight and independently verifiable.
