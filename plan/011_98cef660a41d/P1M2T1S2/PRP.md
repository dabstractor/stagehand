---
name: "P1.M2.T1.S2 — reapStaleLocks function + wire into Acquire + fix over-claims"
description: |
  Closes the stale lock-FILE reaping half of §18.5 (FR52). `flock` auto-releases on process death so the
  *lock* is never stale, but exits that bypass the deferred `os.Remove` (SIGKILL, crash, signal-rescue
  `os.Exit`) orphan the lock *file* → unbounded disk litter. This task adds `reapStaleLocks(dir)` (glob
  `*.lock` → parseContents → `processAlive` → unlink dead-pid files), wires it into `Acquire` right after the
  holder's own flock is taken (the holder's live pid is never reaped), and corrects the three "no stale locks"
  over-claims in `lock.go` (lines 2/31/67) + the two contradicted lines in `docs/how-it-works.md` (170/179) to
  the accurate lock-vs-FILE framing. PRD: §18.5 (Concurrency: the per-repo run lock). Research:
  `architecture/lock_reaping.md` (Fix 1: Stale-File Reaping in Acquire — the authoritative reapStaleLocks spec)
  and `research/s2_reap_stale_locks_map.md` (verified-against-live touchpoint map).

  ⚠️ **THE central design call — port reapStaleLocks VERBATIM from lock_reaping.md; the body is 11 lines.**
  `filepath.Glob(filepath.Join(dir, "*.lock"))` → for each match: `os.ReadFile` (err → continue),
  `parseContents(data)`, `strconv.Atoi(c.Pid)` (err → continue — malformed/empty pid is skipped best-effort),
  `if !processAlive(pid, c.Hostname) { os.Remove(f) }` (ignore the remove error). All errors are swallowed —
  reaping is best-effort disk hygiene; a failed ReadFile/Glob/Remove is a no-op, never fatal.

  ⚠️ **THE second design call — ZERO new imports; lock.go already has os/path/filepath/strconv.** Verified live:
  lock.go's import block has `os`, `path/filepath`, `strconv` (and bufio/crypto/sha256/encoding/hex/errors/
  fmt/strings/sync/atomic/time). reapStaleLocks uses ONLY already-imported symbols. Do NOT add an import
  (an unused import fails `go vet`) and do NOT change go.mod.

  ⚠️ **THE third design call — wire reapStaleLocks into Acquire AFTER `l.writeContents("")` + `current.Store(l)`,
  passing `filepath.Dir(path)`.** The holder's own file is in the glob's matches, BUT its pid is `os.Getpid()`
  (live) → `processAlive` returns true → it is NEVER reaped. `filepath.Dir(path)` (NOT a fresh `lockDir()` call)
  reuses the already-resolved lock file's directory, guaranteeing the glob and the acquired file share the same
  dir (no env-shift discrepancy). Placement after `current.Store(l)` (and before `return l, nil`) means the
  holder is fully set up before reaping runs.

  ⚠️ **THE fourth design call — S1 (processAlive) is a HARD dependency; S2 will NOT compile until S1 lands.**
  Verified: `grep "func processAlive" internal/lock/` is EMPTY today (S1 is mid-Implementing). S2 calls
  `processAlive(pid, c.Hostname)` (unexported, package-internal — defined in lock_unix.go/lock_windows.go by S1).
  Do NOT re-implement processAlive in S2 — assume S1 delivered exactly its PRP's spec. If S1 hasn't landed when
  S2 runs, `go build` fails on "undefined: processAlive" — that is the expected signal to wait for S1.

  ⚠️ **THE fifth design call — S2 ships NO committed tests (P1.M2.T3.S1 owns the reaping tests).** The plan
  splits scope: S2 = function + wiring + doc fixes; P1.M2.T3.S1 = "Stale-file reaping tests (lock_test.go)".
  S2 does NOT add tests. The `unused` lint is satisfied because reapStaleLocks is called from Acquire (same
  package) → it is "used"; processAlive is called from reapStaleLocks + S1's lock_unix_test.go. S2's Validation
  Loop includes a THROWAWAY (non-committed) reap-sanity check (Level 3) to confirm the wiring works without
  overlapping P1.M2.T3.S1's committed-test scope.

  ⚠️ **THE sixth design call — fix the 3 lock.go over-claims (lines 2/31/67) + the 2 how-it-works.md lines
  (170/179) to the lock-vs-FILE framing.** The current docs say "no stale locks" / "no stale-lock reaping or
  PID-liveness checks needed" — directly contradicted by this change. Rewrite each to: the LOCK never goes stale
  (flock auto-releases on process death); orphaned FILES are reaped by pid-liveness on the next Acquire; the
  signal path releases the file before exit. These are Mode-A doc fixes that MUST ride with the code (the item
  is explicit: "directly contradicted by this change").

  Deliverable (edits to existing files only — NO new files, NO new deps): (1) `internal/lock/lock.go` — add
  `reapStaleLocks(dir string)`, wire it into Acquire, fix the 3 over-claim doc comments; (2) `docs/how-it-works.md`
  — fix lines 170 + 179. INPUT = processAlive (S1, the hard dep) + parseContents (lock.go:211) + the resolved
  `path` in Acquire. OUTPUT = stale lock files reaped on every Acquire (dead-pid files removed; the holder's own
  + any live-pid file survive); doc comments + how-it-works.md accurately describe the reaping mechanism.
  SCOPE: `internal/lock/lock.go` + `docs/how-it-works.md` ONLY. Do NOT touch lock_unix.go/lock_windows.go
  (S1/frozen), lock_test.go/lock_unix_test.go (S1 + P1.M2.T3.S1), signal/main (P1.M2.T2).
---

## Goal

**Feature Goal**: Add `reapStaleLocks(dir string)` to `internal/lock/lock.go` and wire it into `Acquire` so
that on every lock acquisition, orphaned `*.lock` files whose recorded `pid` is dead (not a live process on
its recorded `hostname`) are removed — keeping the lock directory clean. The holder's own file (live pid) and
any live contender's file are NEVER reaped (FR52 "never force-break" / "never reap a live pid"). Also correct
the three "no stale locks" over-claims in `lock.go` (2/31/67) and the two contradicted lines in
`docs/how-it-works.md` (170/179) to the accurate lock-vs-FILE framing.

**Deliverable** (edits to existing files only):
1. **`internal/lock/lock.go`** — (a) add `reapStaleLocks(dir string)` (the lock_reaping.md spec, verbatim);
   (b) call `reapStaleLocks(filepath.Dir(path))` in `Acquire` after `l.writeContents("")` + `current.Store(l)`,
   before `return l, nil`; (c) fix the 3 over-claim doc comments at lines 2, 31, 67.
2. **`docs/how-it-works.md`** — fix lines 170 + 179 (the "no stale-lock reaping" / "No stale locks" claims) to
   the lock-vs-FILE framing (flock auto-releases the lock; orphaned files reaped by pid-liveness on Acquire;
   signal path releases before exit).

**Success Definition**: `gofmt -l internal/lock/`, `go vet ./internal/lock/`, `go build ./...` clean (once S1's
processAlive has landed); `go test -race ./...` green (existing tests unchanged — S2 adds none); `make lint`
green (reapStaleLocks is "used" via Acquire; no U1000); stale lock files with dead pids are removed on Acquire;
the holder's own file + any live-pid file survive; the 5 doc sites (3 lock.go + 2 how-it-works.md) accurately
describe reaping; go.mod/go.sum byte-unchanged; zero new imports; only `internal/lock/lock.go` +
`docs/how-it-works.md` touched.

## User Persona

**Target User**: The user whose lock directory (`$XDG_RUNTIME_DIR/stagehand/locks/` or cache fallback)
accumulates orphaned `*.lock` files from interrupted runs (Ctrl-C rescue, SIGKILL, crash). After S2, each
`stagehand` run cleans up dead holders' files automatically. Transitively, the FR52 contention path (the next
Acquire that would otherwise see a cluttered dir).

**Use Case**: A user Ctrl-C's a `stagehand` run after the snapshot (signal-rescue `os.Exit` orphans the file),
then re-runs `stagehand`. The re-run's `Acquire` takes its own flock, then `reapStaleLocks` removes the dead
(pid no longer alive) orphan. The directory stays bounded.

**User Journey**: `stagehand` → `Acquire(repoPath)` → flock succeeds → writeContents + current.Store →
**reapStaleLocks(dir)** (S2) → for each `*.lock`, processAlive(pid, hostname)? dead → os.Remove; live → keep →
proceed with the run.

**Pain Points Addressed**: unbounded disk litter (every interrupted run left a file); the docs dishonestly
claimed "no stale locks" when the FILE does go stale. S2 fixes both the litter and the honesty.

## Why

- **Closes the §18.5 stale-FILE gap.** `flock` auto-releases on death (the lock is never stale), but the FILE
  is orphaned by exits that skip `defer locker.Release()` (SIGKILL, crash, signal-rescue `os.Exit`). Reaping
  on Acquire keeps the directory bounded — the documented §18.5 design.
- **Safe by construction.** Reaping unlinks ONLY dead-pid files (processAlive is conservative on every
  ambiguity → a live pid is never reaped). A dead pid holds no fd → no flock → unlinking cannot defeat
  contention the way unlinking a live holder's inode-bound-flock file would.
- **Cheap + best-effort.** One glob + a pid-liveness check per file on the Acquire path (already a
  filesystem-heavy op). Errors swallowed (reaping failure is a no-op, never fatal).
- **Docs honesty.** The 3 lock.go + 2 how-it-works.md over-claims are directly contradicted by the reaping
  mechanism; fixing them with the code (Mode A) keeps the docs truthful.
- **No API/config/deps change.** One unexported function + one wiring line + doc text. go.mod unchanged.

## What

A compiled `internal/lock` package where `Acquire` reaps stale lock files after taking its own flock, plus
truthful doc comments + how-it-works.md text. No new types, no new exported surface (reapStaleLocks is
unexported, called only from Acquire), no dependency change.

### Success Criteria

- [ ] `reapStaleLocks(dir string)` exists in `internal/lock/lock.go`, matching lock_reaping.md verbatim:
      `filepath.Glob(filepath.Join(dir, "*.lock"))` → per match: `os.ReadFile` (err→continue), `parseContents`,
      `strconv.Atoi(c.Pid)` (err→continue), `if !processAlive(pid, c.Hostname) { os.Remove(f) }` (ignore error).
      All errors swallowed (best-effort).
- [ ] `Acquire` calls `reapStaleLocks(filepath.Dir(path))` AFTER `l.writeContents("")` + `current.Store(l)`,
      BEFORE `return l, nil`.
- [ ] ZERO new imports in lock.go (os/path/filepath/strconv already present); go.mod/go.sum unchanged.
- [ ] The 3 over-claim doc comments at lock.go:2, 31, 67 are rewritten to the lock-vs-FILE framing (the LOCK
      never goes stale; orphaned FILES reaped by pid-liveness on Acquire; signal path releases before exit).
- [ ] docs/how-it-works.md lines 170 + 179 are rewritten to the same lock-vs-FILE framing (no "no stale locks"
      / "no reaping" claims remain).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/lock/`, `go test -race ./...`, `make lint` all
      clean/green (once S1's processAlive has landed); only `internal/lock/lock.go` + `docs/how-it-works.md`
      touched; lock_unix.go/lock_windows.go/lock_test.go/lock_unix_test.go byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the verbatim reapStaleLocks body
(quoted below + in lock_reaping.md), the exact Acquire wiring point, the zero-new-imports fact, the S1
hard-dependency note, and the exact doc-rewrite text (3 lock.go lines + 2 how-it-works.md lines). No PRD/git
internals beyond "flock auto-releases; orphaned files are reaped by pid-liveness".

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/011_98cef660a41d/P1M2T1S2/research/s2_reap_stale_locks_map.md
  why: the AUTHORITATIVE verified-against-live touchpoint map. §1 = S1 is a HARD dep (processAlive not yet in
       the live file); §2 = zero new imports; §3 = the reapStaleLocks body (copy it); §4 = the Acquire wiring
       point (after current.Store(l), filepath.Dir(path)); §5 = the 3 lock.go doc rewrites (lines 2/31/67);
       §6 = the 2 how-it-works.md rewrites (lines 170/179); §7 = test scope (P1.M2.T3.S1); §8 = holder-safety.
  critical: §1 (S2 won't compile until S1 lands — don't re-implement processAlive) + §3 (copy verbatim) +
       §4 (filepath.Dir(path), NOT a fresh lockDir() call).

- docfile: plan/011_98cef660a41d/architecture/lock_reaping.md
  section: "Fix 1: Stale-File Reaping in Acquire" → "reapStaleLocks(dir string)" + "Doc-Comment Corrections"
  why: the AUTHORITATIVE reapStaleLocks spec (verbatim 11-line body) + the safety invariant ("a live pid is
       NEVER reaped … pid-liveness check is precisely what makes unlinking safe") + the 3 doc-comment
       correction sites.
  critical: the exact reapStaleLocks body (Glob → ReadFile → parseContents → Atoi → processAlive → Remove) and
       the "reaping by age/timestamp is rejected" rule (only pid-liveness).

- docfile: plan/011_98cef660a41d/P1M2T1S1/PRP.md
  why: the CONTRACT for processAlive (S1) — the hard dependency. Confirms processAlive is unexported,
       package-internal, in lock_unix.go (!windows) + lock_windows.go (windows), with signature
       `processAlive(pid int, hostname string) bool`. S2 calls it; does NOT redefine it.
  critical: S2 will not compile until S1 lands (`undefined: processAlive`). Do NOT add processAlive to lock.go
       (it is platform-specific → build-tag files only).

- file: internal/lock/lock.go   (the file you EDIT)
  section: package doc (1-12 — line 2 over-claim); Locker doc (~30-33 — line 31 over-claim); Acquire (71-109 —
           line 67 over-claim + the wiring point at writeContents/current.Store/return); parseContents (211-239);
           imports (13-25).
  why: the file you edit. reapStaleLocks goes near parseContents (its helper) or after Acquire (its caller);
       the wiring line goes after `current.Store(l)`; the 3 doc fixes are at lines 2/31/67.
  pattern: reapStaleLocks mirrors the codebase's best-effort I/O style (errors ignored — see Release's
           `os.Remove(path)` and writeContents's ignored Write/Truncate errors). parseContents is the parser to
           reuse (NOT a re-implementation).
  gotcha: ZERO new imports — os/path/filepath/strconv are already in the import block. `filepath.Dir(path)`
           (the resolved lock path's dir), NOT a fresh `lockDir()` call (avoids env-shift discrepancy).

- file: docs/how-it-works.md   (lines 170 + 179 — the "Auto-release" section under "Per-repo run lock (FR52)")
  why: the two contradicted doc lines. Line 170: "...auto-releases on process death ... — no stale-lock
       reaping or PID-liveness checks needed." Line 179: "**Auto-release.** ... No stale locks, no PID-liveness
       checks, no reaping."
  pattern: rewrite to the lock-vs-FILE framing (flock auto-releases the LOCK; orphaned FILES reaped by
           pid-liveness on the next Acquire; signal path releases before exit; Windows = no-op reaping + CAS).
  gotcha: keep the surrounding structure (the numbered list item at 170; the **Auto-release.** paragraph at 179).
           Only replace the over-claiming tail sentences.

- file: PRD.md   §18.5 (h3.91) "Concurrency: the per-repo run lock (FR52)" — esp. "Stale-file reaping (lock vs file)"
  why: the PRD is the source of truth for the lock-vs-FILE framing. It states: flock auto-releases (lock never
       stale); the orphaned FILE is reaped by `kill(pid,0)`→ESRCH on Acquire; a live pid is NEVER reaped;
       reaping by age/timestamp is rejected; hostname-matching scopes reaping to this host.
  critical: the PRD wording is the template for the doc rewrites — mirror its "lock vs file" distinction.
```

### Current Codebase tree (relevant slice)

```bash
internal/lock/
  lock.go              # package doc (line 2 over-claim) + Locker doc (31) + Acquire (67 over-claim + wiring point) + parseContents (211) + imports (13)  ← EDIT (reapStaleLocks + wire + 3 doc fixes)
  lock_unix.go         # processAlive (!windows) — S1 (HARD dep; frozen once landed)                                                                                       ← (NO edit; S1)
  lock_windows.go      # processAlive (windows) — S1                                                                                                                       ← (NO edit; S1)
  lock_test.go         # existing lock tests (lockDir, Acquire, etc.)                                                                                                       ← (NO edit; P1.M2.T3.S1 adds reaping tests here)
  lock_unix_test.go    # processAlive tests (!windows) — S1                                                                                                                ← (NO edit; S1)
docs/
  how-it-works.md      # lines 170 + 179 (the "no stale locks" / "no reaping" over-claims)                                                                                  ← EDIT (2 doc fixes)
go.mod / go.sum        # unchanged (no new dep)
# NO new files. NO tests (P1.M2.T3.S1). NO signal/main (P1.M2.T2).
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Edits: internal/lock/lock.go (reapStaleLocks + Acquire wiring + 3 doc fixes) + docs/how-it-works.md (2 doc fixes).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (design call #4): S1 (processAlive) is a HARD dependency. grep "func processAlive" internal/lock/
// is EMPTY today (S1 mid-Implementing). S2 calls processAlive(pid, c.Hostname); it will not compile until S1
// lands. Do NOT redefine processAlive in lock.go (it is platform-specific → build-tag files only). If `go build`
// fails on "undefined: processAlive", S1 hasn't landed — that is the expected signal.

// CRITICAL (design call #2): ZERO new imports. lock.go ALREADY imports os, path/filepath, strconv (plus bufio/
// crypto/sha256/encoding/hex/errors/fmt/strings/sync/atomic/time). reapStaleLocks uses filepath.Glob,
// filepath.Join, os.ReadFile, os.Remove, strconv.Atoi — ALL already imported. Do NOT add an import.

// CRITICAL (design call #3): wire reapStaleLocks(filepath.Dir(path)) into Acquire AFTER l.writeContents("")
// + current.Store(l), BEFORE return l, nil. Use filepath.Dir(path) (the resolved lock path's dir), NOT a fresh
// lockDir() call — reusing the already-resolved path guarantees the glob dir == the acquired file's dir.

// CRITICAL (design call #1): port reapStaleLocks VERBATIM from lock_reaping.md. All errors swallowed (Glob _,
// ReadFile → continue, Atoi → continue, Remove error ignored). Reaping is best-effort disk hygiene — NEVER fatal.

// CRITICAL (design call #5): S2 ships NO committed tests. P1.M2.T3.S1 owns "Stale-file reaping tests
// (lock_test.go)". The `unused` lint is satisfied (reapStaleLocks is called from Acquire → "used"). Do NOT add
// a test file — it overlaps P1.M2.T3.S1.

// CRITICAL (design call #6): fix ALL 5 doc sites (lock.go:2/31/67 + how-it-works.md:170/179). The "no stale
// locks" / "no reaping" claims are directly contradicted by this change. Leaving any unfixed is a doc lie that
// the item explicitly forbids ("directly contradicted by this change").

// GOTCHA: the holder's own file is in reapStaleLocks's glob matches, BUT its pid is os.Getpid() (live) →
// processAlive returns true → it is NEVER reaped. This is why the wiring is safe AFTER the holder's own setup.

// GOTCHA: reapStaleLocks must NOT reap a file whose pid is alive, even if it appears stuck — that would let a
// contender O_CREATE a fresh inode and flock it, defeating FR52. processAlive's conservatism (alive/ambiguous
// → true) is the keystone; S2 trusts it (do NOT add a secondary age/timestamp check — PRD §18.5 rejects that).

// GOTCHA: ignore the os.Remove error. The file may already be gone (a concurrent Acquire reaped it), or a
// permissions issue — both harmless (best-effort). The codebase style is to ignore best-effort cleanup errors
// (see Release's os.Remove(path) with no error check).

// GOTCHA: do NOT touch lock_unix.go/lock_windows.go (S1 — frozen once landed), lock_test.go/lock_unix_test.go
// (S1 + P1.M2.T3.S1), or signal/main (P1.M2.T2 — the exit-path ReleaseCurrent + OnRescueExit seam, a separate
// concern). This task is lock.go + docs/how-it-works.md ONLY.
```

## Implementation Blueprint

### Data models and structure

No new types. One unexported function + one wiring line + doc text. The function:

```go
// internal/lock/lock.go — reapStaleLocks (place near parseContents, its helper, or after Acquire):
// reapStaleLocks removes every *.lock file in dir whose recorded pid is not a live process on its recorded
// hostname (PRD §18.5 stale-FILE reaping). Called from Acquire AFTER the holder's own flock succeeds — the
// holder's pid is os.Getpid() (live), so its own file is never reaped. SAFETY INVARIANT: a LIVE pid is NEVER
// reaped (processAlive is conservative on every ambiguity) — unlinking a live holder's inode-bound-flock file
// would let a contender O_CREATE a fresh inode and flock it, defeating FR52. Only a DEAD pid (no open fd → no
// flock) is safe to unlink. Malformed/empty pid → skip (best-effort). All errors are ignored (reaping is
// best-effort disk hygiene; a failed Glob/ReadFile/Remove is a no-op, never fatal).
func reapStaleLocks(dir string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.lock"))
	for _, f := range matches {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		c := parseContents(data)
		pid, err := strconv.Atoi(c.Pid)
		if err != nil {
			continue // malformed/empty pid → skip
		}
		if !processAlive(pid, c.Hostname) {
			os.Remove(f) // dead pid → safe to unlink (ignore error)
		}
	}
}
```

```go
// internal/lock/lock.go — the Acquire wiring (after current.Store(l), before return l, nil):
	l.writeContents("")
	current.Store(l)
	reapStaleLocks(filepath.Dir(path)) // §18.5: reap orphaned *.lock files whose pid is dead (holder's live pid survives)
	return l, nil
```

```go
// internal/lock/lock.go — the 3 doc-comment rewrites (lock-vs-FILE framing):
// Line 2 (package doc):
//   BEFORE: // flock(LOCK_EX|LOCK_NB) auto-released on process death (no stale locks).
//   AFTER:  // flock(LOCK_EX|LOCK_NB) auto-released on process death — the LOCK never goes stale;
//           // orphaned FILES (SIGKILL/crash/signal-rescue os.Exit bypassing the deferred Remove) are
//           // reaped by pid-liveness on Acquire.
// Line 31 (Locker doc):
//   BEFORE: // closes — no stale locks. Create via Acquire; release via Release (idempotent)
//   AFTER:  // closes — the LOCK never goes stale (flock auto-releases); orphaned FILES are reaped by
//           // pid-liveness on the next Acquire. Create via Acquire; release via Release (idempotent)
// Line 67 (Acquire doc):
//   BEFORE: // flock auto-released on process death (no stale locks).
//   AFTER:  // flock auto-released on process death — the LOCK never goes stale; after taking its own flock,
//           // Acquire reaps orphaned *.lock FILES whose recorded pid is dead (the holder's own live-pid
//           // file survives).
```

```go
// docs/how-it-works.md — the 2 line rewrites:
// Line 170 (numbered list item 1 tail):
//   BEFORE: ...auto-releases on process death (SIGKILL, crash, power loss) — no stale-lock reaping or PID-liveness checks needed.
//   AFTER:  ...auto-releases on process death (SIGKILL, crash, power loss) — the LOCK never goes stale. Orphaned lock FILES (left by exits that bypass the deferred cleanup) are reaped by pid-liveness on the next Acquire, and the signal path releases the file before exiting.
// Line 179 (the **Auto-release.** paragraph):
//   BEFORE: **Auto-release.** The lock uses POSIX flock — it releases when the file descriptor or process closes. No stale locks, no PID-liveness checks, no reaping. On Windows, flock is a no-op stub; the §13.5 CAS is the guarantee there.
//   AFTER:  **Auto-release + file reaping.** The lock uses POSIX flock — it releases when the file descriptor or process closes, so the LOCK is never stale. The lock FILE, however, is orphaned by exits that bypass the deferred cleanup (SIGKILL, crash, signal-rescue os.Exit); on the next Acquire, stagehand reaps every *.lock whose recorded pid is dead (kill(pid,0)→ESRCH), and the signal path releases the file before exiting. On Windows, flock is a no-op stub, reaping is a no-op too, and the §13.5 CAS is the guarantee there.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: lock.go — add reapStaleLocks(dir string)
  - ADD the function per the Data Models block. Place it near parseContents (its helper) or immediately after
    Acquire (its caller) — co-located with related logic.
  - BODY: filepath.Glob(filepath.Join(dir,"*.lock")) → per match: os.ReadFile (err→continue), parseContents,
    strconv.Atoi(c.Pid) (err→continue), if !processAlive(pid,c.Hostname) { os.Remove(f) }.
  - DOC: §18.5 stale-FILE reaping; the safety invariant (live pid NEVER reaped); best-effort (errors ignored);
    the holder's own live pid survives.
  - GOTCHA: ZERO new imports. All errors swallowed. Do NOT add age/timestamp reaping (PRD rejects it).

Task 2: lock.go — wire reapStaleLocks into Acquire
  - IN Acquire, AFTER `l.writeContents("")` + `current.Store(l)` and BEFORE `return l, nil`, add:
    `reapStaleLocks(filepath.Dir(path))`.
  - GOTCHA: filepath.Dir(path) (the resolved lock path's dir), NOT a fresh lockDir() call. After current.Store
    so the holder is fully set up first. The holder's own file is in matches but its pid (os.Getpid) is live →
    not reaped.

Task 3: lock.go — fix the 3 over-claim doc comments (lines 2, 31, 67)
  - REWRITE each per the Data Models block to the lock-vs-FILE framing. Remove every "no stale locks" claim.
  - GOTCHA: the package doc (line 2) may span the framing — keep the surrounding context (the XDG-runtime/cache
    location, the stdlib-only note) intact; only replace the over-claiming clause.

Task 4: docs/how-it-works.md — fix lines 170 + 179
  - REWRITE line 170's tail (the "no stale-lock reaping or PID-liveness checks needed" clause) and the entire
    **Auto-release.** paragraph (line 179) per the Data Models block.
  - GOTCHA: keep the surrounding structure (the numbered list at 170; the paragraph header at 179). Mirror the
    PRD §18.5 "lock vs file" wording. Do NOT touch other parts of the doc.

Task 5: VERIFY (no further file change)
  - RUN `gofmt -w internal/lock/lock.go`; `go vet ./internal/lock/`; `go build ./...` (REQUIRES S1's
    processAlive — if it fails on "undefined: processAlive", S1 hasn't landed); `go test -race ./...`;
    `make lint`.
  - go.mod/go.sum byte-unchanged. ZERO new imports. Only lock.go + docs/how-it-works.md touched.
    lock_unix.go/lock_windows.go/lock_test.go/lock_unix_test.go byte-unchanged.
  - THROWAWAY reap-sanity check (Level 3, non-committed): confirm the wiring reaps a dead-pid file + spares
    the holder's own. (Committed tests are P1.M2.T3.S1's scope.)
```

### Implementation Patterns & Key Details

```go
// reapStaleLocks — the verbatim lock_reaping.md spec (11 lines, all errors swallowed):
func reapStaleLocks(dir string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.lock"))     // _ : Glob error → empty list → no-op
	for _, f := range matches {
		data, err := os.ReadFile(f)
		if err != nil { continue }                                  // file vanished → skip
		c := parseContents(data)                                    // reuse the existing parser (lock.go:211)
		pid, err := strconv.Atoi(c.Pid)
		if err != nil { continue }                                  // malformed/empty pid → skip (best-effort)
		if !processAlive(pid, c.Hostname) {                         // S1's helper; conservative → live never reaped
			os.Remove(f)                                            // dead pid → safe (ignore error)
		}
	}
}

// The Acquire wiring — after the holder's own setup, using the resolved path's dir:
	l.writeContents("")
	current.Store(l)
	reapStaleLocks(filepath.Dir(path))   // NOT lockDir() — reuse the resolved path's dir (no env-shift gap)
	return l, nil

// The best-effort I/O style (codebase convention — see Release's os.Remove + writeContents's ignored errors):
//   errors are swallowed because reaping is disk hygiene, never a correctness path. flock + the §13.5 CAS
//   are the correctness guarantees; reaping just keeps the directory tidy.
```

```go
// Level 3 throwaway reap-sanity check (NOT a committed test — P1.M2.T3.S1 owns those):
// In a scratch program / REPL against a temp lock dir:
//   dir := t.TempDir(); t.Setenv("XDG_RUNTIME_DIR", dir); t.Setenv("XDG_CACHE_HOME","")
//   // plant a DEAD-pid orphan: write dir/dead.lock with pid=<a reaped child pid> + hostname=thisHost
//   // plant the holder's OWN file via Acquire (its pid is live)
//   l, err := Acquire(repoPath)   // triggers reapStaleLocks(dir)
//   // ASSERT: dir/dead.lock is GONE (reaped); the holder's <hash>.lock SURVIVES.
// This confirms the wiring + the holder-safety invariant without adding a committed test.
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — no new dep; all symbols already imported. go mod tidy is a no-op.

PACKAGE EDGES:
  - internal/lock → (stdlib only). reapStaleLocks calls processAlive (S1, same package, build-tag file).
    NO new import. NO new module dep. The package stays a stdlib-only leaf (no stagehand imports).

FROZEN / NOT-EDITED:
  - internal/lock/lock_unix.go + lock_windows.go — S1's processAlive (the hard dep; frozen once landed).
  - internal/lock/lock_test.go + lock_unix_test.go — existing tests + S1's processAlive tests. P1.M2.T3.S1
    adds the committed reaping tests here. S2 adds NONE.
  - internal/signal/* + cmd/stagehand/main.go — P1.M2.T2 owns the exit-path ReleaseCurrent + OnRescueExit
    seam (the prevention half; this task is the reaping backstop). Different concern.
  - Acquire's flock/parseContents/HeldError logic — UNCHANGED (reapStaleLocks is ADDITIVE: one new call line).

DOWNSTREAM CONSUMERS (do NOT implement here):
  - P1.M2.T3.S1 (next-tests): the committed reaping tests (plant a dead-pid file → Acquire → assert reaped;
    plant a live-pid file → assert survives; the holder's own file survives). S2's function is the test target.
  - P1.M2.T2 (parallel-concern): the exit-path ReleaseCurrent + OnRescueExit signal seam — PREVENTS the common
    orphan producer (signal-rescue os.Exit). S2 is the BACKSTOP (SIGKILL/crash still orphan; reaping catches them).

NO DATABASE / NO ROUTES / NO CONFIG / NO CLI / NO NEW FILES / NO COMMITTED TESTS.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/lock/lock.go
test -z "$(gofmt -l internal/lock/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/lock/        # Catches a malformed function / unused var.
go build ./...                 # REQUIRES S1's processAlive. If it fails "undefined: processAlive", S1 hasn't landed.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm ZERO new imports in lock.go:
git diff internal/lock/lock.go | grep -E '^\+\s*"(os|path/filepath|strconv)"' && echo "UNEXPECTED new import (re-check)" || echo "no new imports (good)"
# Confirm reapStaleLocks + the wiring landed:
grep -n 'func reapStaleLocks' internal/lock/lock.go          # the function def (1 hit)
grep -n 'reapStaleLocks(filepath.Dir(path))' internal/lock/lock.go  # the Acquire wiring (1 hit)
# Expected: clean + build succeeds (post-S1). If `go build` errors "undefined: processAlive", wait for S1.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test -race ./internal/lock/   # existing lock tests (lockDir/Acquire/Release/SetSnapshot) + S1's processAlive tests stay green.
go test -race ./...              # full module — no regression.
# Expected: green throughout. S2 adds NO new test (P1.M2.T3.S1 owns the committed reaping tests). The wiring is
#   additive (one reapStaleLocks call in Acquire); existing Acquire tests exercise the happy path unchanged.
# NOTE: if an existing Acquire test plants a fixture lock file with a stale pid in the lock dir, reapStaleLocks
#   may now remove it — re-check any such test (unlikely; existing tests use t.Setenv XDG isolation per-call).
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagehand ./cmd/stagehand && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only lock.go + docs/how-it-works.md changed:
git diff --name-only | grep -Ev '^internal/lock/lock\.go$|^docs/how-it-works\.md$' \
  && echo "UNEXPECTED file changed" || echo "only lock.go + docs/how-it-works.md changed (good)"
# Confirm the frozen S1/signal/test files are byte-unchanged:
git diff --exit-code -- internal/lock/lock_unix.go internal/lock/lock_windows.go internal/lock/lock_test.go internal/lock/lock_unix_test.go internal/signal cmd/stagehand && echo "frozen files UNCHANGED (expected)"
# Confirm ALL 5 doc sites are fixed (no over-claim remains):
grep -n 'no stale locks' internal/lock/lock.go && echo "BAD: lock.go over-claim remains" || echo "lock.go over-claims fixed (good)"
grep -n 'no stale-lock reaping\|No stale locks' docs/how-it-works.md && echo "BAD: how-it-works over-claim remains" || echo "how-it-works over-claims fixed (good)"

# THROWAWAY reap-sanity check (NOT committed — P1.M2.T3.S1 owns committed tests). Confirm the wiring works:
cat > /tmp/reap_sanity_test.go <<'EOF'
//go:build ignore
package main
// Minimal: plant a dead-pid orphan + the holder's own file via Acquire; assert the orphan is reaped + the
// holder's survives. (Sketched; run as a throwaway `go run` against a temp XDG dir + a forked-then-waited pid.)
EOF
echo "throwaway sanity check sketched (run manually if desired; P1.M2.T3.S1 adds the committed version)"
```

### Level 4: Creative & Domain-Specific Validation

```bash
# THE lint gate — reapStaleLocks is "used" (called from Acquire) → no U1000:
make lint 2>&1 | grep -iE 'reapStaleLocks|unused|U1000' && echo "BAD: reapStaleLocks flagged" \
  || echo "reapStaleLocks not flagged by unused (good — Acquire is its caller)"
# Safety-invariant audit: confirm reapStaleLocks NEVER unlinks without the processAlive(false) gate:
grep -B2 'os.Remove(f)' internal/lock/lock.go | grep 'processAlive' && echo "Remove gated on processAlive (good)" || echo "BAD: os.Remove not gated"
# Doc-honesty audit: confirm the lock-vs-FILE framing is present at all 5 sites:
grep -c 'reaped by pid-liveness\|orphaned.*FILES\|orphaned lock FILES' internal/lock/lock.go docs/how-it-works.md  # ≥5 (3 lock.go + 2 how-it-works)
# Cross-platform build (processAlive is build-tagged; reapStaleLocks in lock.go compiles on both):
GOOS=windows go build ./internal/lock/ && echo "windows build OK" || echo "windows build FAILED"
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `gofmt -l internal/lock/`, `go vet ./internal/lock/`, `go build ./...` (post-S1),
      `go mod tidy` no-op, ZERO new imports.
- [ ] Level 2 green: `go test -race ./internal/lock/` + `go test -race ./...` (existing tests unchanged; S2 adds none).
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only lock.go + how-it-works.md changed; frozen files
      (lock_unix.go/lock_windows.go/lock_test.go/lock_unix_test.go/signal/main) byte-unchanged; all 5 doc sites fixed.
- [ ] Level 4: `make lint` green — no `unused`/U1000 for reapStaleLocks; `os.Remove` gated on `processAlive`;
      `GOOS=windows go build ./internal/lock/` succeeds.

### Feature Validation

- [ ] `reapStaleLocks(dir string)` exists, matching lock_reaping.md verbatim (Glob→ReadFile→parseContents→Atoi→processAlive→Remove; all errors swallowed).
- [ ] `Acquire` calls `reapStaleLocks(filepath.Dir(path))` after `current.Store(l)`, before `return l, nil`.
- [ ] The holder's own file (live pid) is never reaped; only dead-pid files are removed.
- [ ] lock.go:2/31/67 rewritten to the lock-vs-FILE framing (no "no stale locks" remains).
- [ ] docs/how-it-works.md:170/179 rewritten to the lock-vs-FILE framing (no "no stale-lock reaping"/"No stale locks" remains).

### Code Quality Validation

- [ ] reapStaleLocks mirrors lock_reaping.md verbatim; errors swallowed (best-effort, codebase convention).
- [ ] Reuses parseContents (NOT a re-implementation); uses filepath.Dir(path) (NOT a fresh lockDir() call).
- [ ] No scope creep into processAlive (S1), committed tests (P1.M2.T3.S1), signal/main exit-path (P1.M2.T2),
      or any other file.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Mode-A doc comment on reapStaleLocks (§18.5; safety invariant; best-effort; holder survives).
- [ ] lock.go:2/31/67 + how-it-works.md:170/179 all fixed (the item's "DOCS: Mode A" requirement).
- [ ] go.mod/go.sum byte-unchanged; no new files; no committed tests.

---

## Anti-Patterns to Avoid

- ❌ Don't redefine `processAlive` in lock.go. It is S1's platform-specific helper (build-tag files only).
  S2 CALLS it; if `go build` fails "undefined: processAlive", S1 hasn't landed — wait for it (the hard dep).
- ❌ Don't add imports or change go.mod. lock.go ALREADY imports os/path/filepath/strconv. An unused import
  fails `go vet`; a new dep is wrong (reaping is stdlib-only).
- ❌ Don't deviate from the lock_reaping.md reapStaleLocks body. The 11-line cascade (Glob→ReadFile→parseContents
  →Atoi→processAlive→Remove, all errors swallowed) is the authoritative design. Don't add age/timestamp reaping
  (PRD §18.5 explicitly rejects it — a slow-but-live run must never have its file pulled).
- ❌ Don't call `lockDir()` inside Acquire to get the reaping dir. Use `filepath.Dir(path)` — the resolved lock
  path's directory. Reusing `path` guarantees the glob dir == the acquired file's dir (no env-shift discrepancy
  if XDG vars changed between the lockPath call and now).
- ❌ Don't wire reapStaleLocks BEFORE `current.Store(l)` or before `writeContents("")`. The holder must be fully
  set up first (its file written with the live pid) so reapStaleLocks sees the correct contents and the singleton
  is current. Place it after `current.Store(l)`, before `return l, nil`.
- ❌ Don't add a committed test. P1.M2.T3.S1 owns "Stale-file reaping tests (lock_test.go)". S2's function is
  "used" via Acquire → no `unused`-lint issue. Adding a test overlaps P1.M2.T3.S1. (A throwaway Level-3 sanity
  check is fine — it's not committed.)
- ❌ Don't surface reaping errors. Glob/ReadFile/Atoi/Remove errors are ALL swallowed (best-effort disk hygiene;
  never fatal). The codebase convention is to ignore best-effort cleanup errors (Release's os.Remove, writeContents's
  Write/Truncate). A `return err` from reapStaleLocks would make reaping a correctness path — it isn't.
- ❌ Don't leave any of the 5 doc sites unfixed. The "no stale locks"/"no reaping" claims are directly contradicted
  by this change. Fix lock.go:2/31/67 AND how-it-works.md:170/179 — the item is explicit these "MUST ride with the code".
- ❌ Don't touch lock_unix.go/lock_windows.go (S1 — frozen), lock_test.go/lock_unix_test.go (S1 + P1.M2.T3.S1),
  internal/signal/* or cmd/stagehand/main.go (P1.M2.T2 — the exit-path ReleaseCurrent seam). This task is
  lock.go + docs/how-it-works.md ONLY.
- ❌ Don't reap a file whose pid is alive. The safety invariant ("never reap a live pid") is encoded in
  processAlive's conservatism (alive/ambiguous → true); S2 trusts it. Do NOT add a secondary check that could
  override it (e.g. "reap if older than N" — that would unlink a live holder's file and break FR52).
- ❌ Don't change go.mod/go.sum or add files. One function + one wiring line + 5 doc-text edits.
- ❌ Don't skip `go build ./...` (it's the S1-dependency gate) or `make lint` (the `unused` gate for reapStaleLocks).
  And don't skip the doc-honesty audit (grep for the removed phrases — all 5 sites must be clean).
