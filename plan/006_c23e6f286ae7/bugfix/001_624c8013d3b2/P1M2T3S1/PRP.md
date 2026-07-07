---
name: "P1.M2.T3.S1 (bugfix Issue 4a) — Atomic in-place setSnapshot rewrite (Seek→Write→Truncate) + invariant test"
description: |
  Bugfix for Issue 4 (Minor — truncated/partial read mid-rewrite): `Locker.setSnapshot`
  (`internal/lock/lock.go`) currently rewrites the lock file as `Truncate(0) → Seek(0,0) → writeContents`,
  which empties the file BEFORE refilling it. A contender that loses the `flock` race does a SEPARATE
  `os.ReadFile(path)` (open/read/close on a different fd) in `Acquire` on `EWOULDBLOCK`; if its read lands
  in the empty-file window, `parseContents` yields all-empty fields and the Busy (exit 5) message renders
  as `"another stagecoach run is already in progress on  (pid  on )."` — ugly and uninformative. Functionally
  conservative (empty snapshot → no-op fast path skipped → Busy is the safe outcome), but the diagnostic is
  broken. Fix: rewrite IN PLACE on the held fd as `Seek(0,0) → Write(fullBuffer) → Truncate(len) → Sync`
  with **Write BEFORE Truncate**, so the file is NEVER empty during the rewrite (always the old content, a
  prefix of the new content, or the complete new content).

  ⚠️ **THE central HARD constraint — flock is INODE-bound; temp-file + `os.Rename` is FORBIDDEN.**
  `flock(LOCK_EX|LOCK_NB)` binds the advisory lock to the inode of the held fd, NOT the filename
  (`architecture/flock_inode_constraint.md`). `os.Rename(temp, lockPath)` installs a NEW inode at lockPath;
  the holder's fd still points at the OLD inode (flock retained but nameless); a contender `OpenFile(lockPath)`
  → new inode → `flock` SUCCEEDS → **FR52 contention detection is silently bypassed** (two processes race on
  HEAD — the headline safety guarantee is defeated). The PRD lists "temp-file + rename (atomic on POSIX)"
  generically WITHOUT accounting for the inode binding — the implementer MUST NOT use it. The ONLY correct
  fix is the in-place rewrite on the SAME held fd. See research §2.

  ⚠️ **THE write-ordering invariant — Write BEFORE Truncate (NOT Truncate before Write).** During the rewrite
  the fd/inode content is one of {old content, prefix of new content, complete new content} — NEVER empty.
  The leading `pid`/`hostname`/`repo`/`timestamp` lines precede `snapshot`, so a contender always reads at
  least the diagnostic lines. The current bug is precisely Truncate-BEFORE-Write (= empty file in the
  window). The fix inverts the order. See research §2.

  ⚠️ **THE consolidation decision — make `writeContents` the uniform Seek→Write→Truncate→Sync primitive,
  called from BOTH `Acquire` and `setSnapshot` (Option A).** `writeContents` is already called from two
  sites: `Acquire` (initial write, `l.writeContents("")`) and `setSnapshot` (the rewrite). Making
  `writeContents` itself do Seek→Write→Truncate→Sync (instead of the current bare `Fprintf`) is DRY (one
  rewrite path), makes `setSnapshot` a thin nil-guard + delegate, AND fixes a LATENT `Acquire` bug as a
  bonus: today `Acquire` opens `O_CREATE|O_RDWR` and `Fprintf`s without Truncate, so if the file already
  existed (a prior crashed process left a LONGER stale file) the prefix is overwritten but stale trailing
  bytes remain on disk. The new Truncate cuts them → the file is exactly the new content. Strictly better;
  no regression. (Acceptable variant: inline the buffered Write into `setSnapshot` and leave `writeContents`
  as Fprintf-only — also correct. Option A is chosen for DRY + the Acquire fix. See research §3.)

  ⚠️ **THE error-handling decision — PRESERVE the ignore-errors style.** The current code ignores
  Seek/Truncate/Sync/Fprintf errors, and the public `SetSnapshot` signatures (method + package-level) are
  VOID — the contract forbids changing them. A write failure here is non-fatal (the lock is still held via
  flock; the snapshot is a fast-path/diagnostic nicety). Do NOT add error returns to `writeContents`/
  `setSnapshot`. See research §3.

  ⚠️ **THE test decision — invariant test, NOT a nondeterministic race test.** The contract is explicit: "A
  full race test is non-deterministic; instead assert the file is never empty immediately after a setSnapshot
  call." The new `TestSetSnapshot_FileNeverEmptyWellFormed` asserts: (1) the file is non-empty + well-formed
  (all 5 keys) after every SetSnapshot; (2) the SHRINK case — a long snapshot followed by a short one — leaves
  NO stale trailing bytes, proven by `strings.HasSuffix(data, "snapshot=<short>\n")` on the RAW bytes (a
  parse-only check CANNOT catch a missing Truncate, because `parseContents` silently skips the trailing
  malformed line). The nil/released no-op is already covered by `TestSetSnapshot_NilSafeNoOp` +
  `TestSetSnapshot_MethodAfterRelease` (unchanged, stay green). The contender-side empty-field guard is
  Issue 4b (P1.M2.T4.S1) — explicitly NOT this subtask. See research §4.

  Builds on the ALREADY-LANDED Issue 2 (Release removes the lock file) and Issue 3 (canonical repo= field)
  fixes present in the working tree. It does NOT touch `Acquire`/`Release`/`lockHash`/`lockPath`/
  `handleLockContention`/platform files/docs/go.mod.

  Deliverable: edits to `internal/lock/lock.go` (`setSnapshot` + `writeContents` doc + body) + an addition
  to `internal/lock/lock_test.go` (one new invariant test + `"strings"` import). NO new files, NO new
  imports in lock.go (`fmt` already imported), NO go.mod change, NO docs, NO public-signature change.
  OUTPUT: the lock file is never observable empty during a snapshot rewrite; verified by `go test ./internal/lock/`.
---

## Goal

**Feature Goal**: Eliminate the empty-file window in `setSnapshot` by rewriting the lock file IN PLACE on
the held fd as `Seek(0,0) → Write(fullBuffer) → Truncate(len) → Sync` (Write-before-Truncate), so a
contender's `os.ReadFile` can never observe an empty or prefix-truncated-diagnostic file during a snapshot
update. The fd/inode never changes (preserving flock semantics); the file is always {old content, prefix of
new content, or complete new content}.

**Deliverable** (edits to two existing files):
1. **`internal/lock/lock.go`** — restructure `setSnapshot` (keep the `l.file==nil` guard; delegate to
   `writeContents`) and `writeContents` (build the full content string, then `Seek(0,0) → Write([]byte) →
   Truncate(int64(len)) → Sync`). Refresh both doc comments (the write-ordering invariant + the inode
   constraint). Ignore errors (codebase style).
2. **`internal/lock/lock_test.go`** — ADD `TestSetSnapshot_FileNeverEmptyWellFormed` (the never-empty +
   well-formed + shrink/Truncate invariant) + the `"strings"` import.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/lock/` clean;
`go test -race ./internal/lock/` green — the new invariant test passes AND every existing lock test stays
green (incl. `TestSetSnapshot_UpdatesFile`, `TestSetSnapshot_NilSafeNoOp`,
`TestSetSnapshot_MethodAfterRelease`, the Issue-2/Issue-3 tests). `go test -race ./...` green (no caller
regresses; `Acquire` transparently uses the new `writeContents`). go.mod/go.sum unchanged; no public
`SetSnapshot` signature changed; no file outside `internal/lock/lock.go` + `internal/lock/lock_test.go`
touched.

## User Persona

**Target User**: A developer who accidentally double-runs `stagecoach` (or runs it in two terminals on the
same repo) and gets the Busy (exit 5) message. Transitively PRD §18.5 "Mechanism" / "Contention behavior"
(the contender reads the holder's `snapshot=` + `pid`/`hostname`/`repo` for the message).

**Use Case**: Holder process A publishes a snapshot via `SetSnapshot`; contender B's `Acquire` loses the
flock race and reads A's lock file for the Busy message. B must never read an empty/partial file that
renders the message as `"on  (pid  on )"`.

**User Journey**: A's `setSnapshot(sha)` → `writeContents` rewrites in place (never empty) → B's
`os.ReadFile` lands in the window → B reads the old content OR a prefix (incl. all diagnostic lines) OR the
complete new content → `parseContents` yields non-empty diagnostics → the Busy message names A's pid/
hostname/repo correctly.

**Pain Points Addressed**: removes the ugly, uninformative `"on  (pid  on )"` Busy message caused by the
empty-file window. Diagnostic correctness only; the conservative functional behavior (empty snapshot → skip
no-op → Busy) is unchanged.

## Why

- **Fixes the Issue-4 empty-file window at its root.** Write-before-Truncate on the held fd means the file
  is never empty during the rewrite — a contender always reads a well-formed (or at least
  diagnostic-complete) file.
- **Respects the inode constraint (does NOT break FR52).** The in-place rewrite keeps the same fd/inode, so
  flock semantics are preserved. A naive temp-file+Rename fix would silently disable cross-process
  contention detection — this fix avoids that trap entirely.
- **DRY + fixes a latent Acquire bug.** Consolidating the rewrite into `writeContents` (called from both
  `Acquire` and `setSnapshot`) gives one atomic-ish primitive and also cuts stale trailing bytes on
  `Acquire`'s initial write to a pre-existing (crash-leftover) file.
- **Minimal, internal, no surface change.** No public signature change, no new deps, no docs, no caller
  edits. DOCS: none (P1.M3 owns the doc sweep); Issue 4b (P1.M2.T4.S1) owns the contender-side empty-field
  guard as defense-in-depth.

## What

`setSnapshot` and `writeContents` restructured to Seek→Write→Truncate→Sync (Write-before-Truncate) on the
held fd; one invariant test added. No public API change, no inode-rename, no contender-side guard (that is
4b).

### Success Criteria

- [ ] `setSnapshot(sha)`: `if l.file == nil { return }` then `l.writeContents(sha)` (keeps the nil guard;
      delegates to the new primitive). Doc comment states the write-ordering invariant + the inode constraint.
- [ ] `writeContents(snapshot)`: build the full `fmt.Sprintf("pid=%s\nhostname=%s\nrepo=%s\ntimestamp=%s\nsnapshot=%s\n", ...)`;
      `l.file.Seek(0,0)`; `l.file.Write([]byte(content))`; `l.file.Truncate(int64(len(content)))`;
      `l.file.Sync()`. ORDER: Seek → Write → Truncate → Sync (Write BEFORE Truncate). Errors ignored.
- [ ] NO `os.Rename` / temp-file anywhere in the rewrite path (inode constraint).
- [ ] Public `SetSnapshot` signatures UNCHANGED: `func (l *Locker) SetSnapshot(sha string)` and
      `func SetSnapshot(sha string)` both still void.
- [ ] `TestSetSnapshot_FileNeverEmptyWellFormed` added: after Acquire + after each SetSnapshot, the file is
      non-empty and parses to all 4 non-empty diagnostic fields (pid/hostname/repo/timestamp) + the expected
      snapshot; the SHRINK case (long→short) leaves no stale trailing bytes (`strings.HasSuffix(data, "snapshot=<short>\n")`).
- [ ] Existing tests unchanged + green: `TestSetSnapshot_UpdatesFile`, `TestSetSnapshot_NilSafeNoOp`,
      `TestSetSnapshot_MethodAfterRelease`, `TestAcquireRelease_RoundTrip`, the Issue-2/Issue-3 tests.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/lock/`, `go test -race ./internal/lock/`,
      `go test -race ./...` clean/green; go.mod/go.sum byte-unchanged; only `lock.go` + `lock_test.go` touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact `setSnapshot`/
`writeContents` before/after (quoted below), the inode constraint + the Write-before-Truncate ordering, the
invariant-test design (with a concrete code sketch including the shrink/Truncate-proof assertion), and the
test conventions (resetCurrent, XDG isolation, read-before-Release). No PRD/git/provider knowledge required
— this is a two-method restructure + one test.

### Documentation & References

```yaml
# MUST READ — the authoritative research (every edit + the invariant + the test sketch)
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M2T3S1/research/setsnapshot-atomic-rewrite.md
  why: the current-code bug (§1), the HARD inode constraint + Write-before-Truncate (§2), the chosen
       Option-A consolidation + the latent Acquire fix + error-handling (§3), the invariant-test design
       incl. WHY the shrink case needs a raw-bytes suffix check (§4), scope fences (§5).
  critical: §2 (NEVER os.Rename; Write BEFORE Truncate) and §4 (a parse-only check CANNOT catch a missing
       Truncate — use strings.HasSuffix on the raw bytes) are the things most likely to be implemented wrong.

# THE critical constraint (the inode binding — why rename is forbidden)
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/flock_inode_constraint.md
  why: proves flock is inode-bound; temp-file+Rename orphans the holder's flock and lets a contender flock
       the new inode and SUCCEED (bypassing FR52). Mandates the in-place Seek→Write→Truncate→Sync fix.
  critical: NEVER os.Rename over the lock path. ALWAYS rewrite in place on the held fd. Write-before-Truncate.

# The bug report + root cause + chosen fix
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/issue_analysis.md
  section: "Issue 4 (Minor) — SetSnapshot rewrite observable mid-write (truncated/partial read)"
  why: confirms the Truncate(0)→Seek→writeContents root cause; the empty-file window; the chosen "in-place
       rewrite + empty-field guard" fix (this subtask is the in-place rewrite; 4b is the guard).
  critical: the fix is Seek→Write→Truncate on the held fd (Option 2 of the PRD's suggestions; Option 1
       rename is FORBIDDEN by the inode constraint).

# The file being fixed
- file: internal/lock/lock.go
  section: setSnapshot (L131-140) + writeContents (L167-174).
  why: the two methods you restructure. setSnapshot currently does Truncate→Seek→writeContents (BUG);
       writeContents currently does a bare Fprintf+Sync (no Seek/Truncate). writeContents is ALSO called
       from Acquire (L~104, `l.writeContents("")`) — the new primitive serves both.
  pattern: writeContents builds the full string (fmt.Sprintf) then Seek(0,0)→Write([]byte)→Truncate(int64(len))→Sync;
       setSnapshot keeps the `if l.file==nil {return}` guard and delegates to writeContents.
  gotcha: keep the nil guard in setSnapshot (writeContents has none — it's called in controlled contexts);
       ignore all I/O errors (codebase style); do NOT add a nil guard to writeContents (Acquire's call site
       always has a non-nil l.file; setSnapshot guards before delegating).

# The tests (the pattern for the new one + the tests that must stay green)
- file: internal/lock/lock_test.go
  section: TestSetSnapshot_UpdatesFile (the closest analog — Acquire + SetSnapshot + ReadFile + parseContents),
           TestSetSnapshot_NilSafeNoOp + TestSetSnapshot_MethodAfterRelease (the nil/released no-op, already
           covered), TestRelease_RemovesLockFile + TestAcquire_RepoFieldIsCanonical (the XDG-isolation +
           resetCurrent + read-before-Release pattern to mirror).
  why: the new test mirrors TestSetSnapshot_UpdatesFile's shape (Acquire → SetSnapshot → ReadFile → parse) and
       the newer tests' hygiene (resetCurrent, XDG isolation). Confirms the nil/released no-op is already tested.
  pattern: white-box `package lock`; resetCurrent(t); NO t.Parallel() (the `current` singleton is process-global);
       isolate XDG (t.Setenv XDG_RUNTIME_DIR=t.TempDir() + clear XDG_CACHE_HOME); read os.ReadFile(l.path) BEFORE
       Release (Issue 2 removes the file on Release — `defer l.Release()` is fine because reads are in the body).
  gotcha: add "strings" to the imports for strings.HasSuffix. Do NOT t.Parallel.

# The contender read path (why the window bites — NO edit, just context)
- file: internal/lock/lock.go
  section: Acquire's EWOULDBLOCK branch (`data, _ := os.ReadFile(path)` → parseContents → *HeldError).
  why: proves the contender does a SEPARATE open/read/close (different fd) — so the holder's in-place rewrite
       is observed via the SAME inode the contender opens by name. Confirms the inode-preserving fix is correct.
  critical: do NOT edit Acquire — it is the proof the fix targets the right read path.

# The defense-in-depth (NOT this subtask — Issue 4b owns it)
- file: internal/cmd/default_action.go
  section: handleLockContention (L~241-254).
  why: the contender-side empty-field guard ("an unknown repo" / "pid <unknown>") is Issue 4b (P1.M2.T4.S1).
       This subtask (4a) does NOT touch default_action.go.
  critical: do NOT edit default_action.go here — it is 4b's scope.

# The prerequisites (LANDED — composition context)
- file: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M2T1S1/PRP.md   (Issue 2 — Release removes the file)
- file: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M2T2S1/PRP.md   (Issue 3 — canonical repo= field)
  why: BOTH have landed in the working tree (Acquire stores `repo: canonical`; Release close-then-removes).
       So the new test MUST read the lock file BEFORE Release (deferred Release is fine; reads happen first),
       and the `c.Repo` value is canonical (compare with `canonical, _ := lockHash(repo)` if asserting repo).
  critical: read-before-Release. The lock.go you edit ALREADY has the Issue-2/Issue-3 code — do not revert it.
```

### Current Codebase tree (relevant slice)

```bash
internal/lock/
  lock.go            # setSnapshot (L131) + writeContents (L167) — EDIT (doc + body). Acquire/Release/lockHash/
                     #   lockPath ALREADY carry the landed Issue-2/Issue-3 fixes — do NOT touch them.
  lock_test.go       # ADD TestSetSnapshot_FileNeverEmptyWellFormed + "strings" import. Existing tests unchanged.
  lock_unix.go       # real flock — NO edit
  lock_windows.go    # no-op flock — NO edit
internal/cmd/default_action.go   # handleLockContention — NO edit (Issue 4b / P1.M2.T4.S1 owns the guard)
go.mod / go.sum      # unchanged (stdlib only; fmt already imported in lock.go)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Edits to internal/lock/lock.go (setSnapshot + writeContents) +
# internal/lock/lock_test.go (1 new test + "strings" import).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (inode constraint — NEVER os.Rename): flock binds to the INODE, not the filename. A temp-file +
// os.Rename over the lock path installs a NEW inode at lockPath; the holder's fd+flock stay on the OLD inode
// (now nameless); a contender OpenFile(lockPath) → new inode → flock SUCCEEDS → FR52 bypassed. The fix MUST
// rewrite in place on the held fd. (architecture/flock_inode_constraint.md.)

// CRITICAL (write ORDER — Write BEFORE Truncate): Seek(0,0) → Write(full) → Truncate(len) → Sync. During the
// rewrite the file is {old content | prefix of new | complete new} — NEVER empty. Truncate-before-Write (the
// current bug) yields an empty file in the window. The leading diagnostic lines precede snapshot, so a
// contender always reads at least pid/hostname/repo/timestamp.

// CRITICAL (Truncate is what cuts stale trailing bytes): when the new content is SHORTER than the previous
// (e.g. a long snapshot → a short one), Write overwrites the prefix but leaves the previous tail on disk;
// Truncate(len(new)) cuts it. Omitting Truncate leaves corrupt on-disk content (a malformed trailing line
// that parseContents silently skips — so the bug is invisible to parse-only assertions; the test MUST check
// raw bytes via strings.HasSuffix).

// GOTCHA: writeContents is called from BOTH Acquire (initial write, l.writeContents("")) and setSnapshot
// (rewrite). Making it the uniform Seek→Write→Truncate→Sync primitive serves both (DRY) and fixes a latent
// Acquire stale-trailing-bytes bug (a crash-leftover longer file). setSnapshot keeps its `if l.file==nil`
// guard and delegates; writeContents has NO nil guard (its callers guarantee l.file != nil).

// GOTCHA: preserve the ignore-errors style. The current code ignores Seek/Truncate/Sync/Fprintf errors and
// the public SetSnapshot signatures are void (the contract forbids changing them). Do NOT add error returns
// to writeContents/setSnapshot. A write failure is non-fatal (flock still holds; snapshot is a nicety).

// GOTCHA: do NOT add a nil guard to writeContents. Acquire's call site always has l.file != nil (the Locker
// is constructed with the open fd first); setSnapshot guards `if l.file==nil {return}` BEFORE calling
// writeContents. Adding a guard to writeContents is dead code (and diverges from the current shape).

// GOTCHA: read the lock file BEFORE Release in the new test. Issue 2 (landed) os.Remove's the file in Release.
// `defer l.Release()` is fine because the os.ReadFile calls are in the test body (run before the deferred
// Release). A deferred read-after-Release would hit os.IsNotExist.

// GOTCHA: do NOT call t.Parallel() in the new test. The `current` singleton is process-global; every existing
// lock test is sequential and calls resetCurrent(t). Parallelism would race the singleton.

// GOTCHA: the SHRINK-case assertion MUST be on the RAW bytes (strings.HasSuffix), NOT a parse-only check.
// parseContents silently skips a malformed trailing line, so a Truncate-omitting bug still reports the right
// snapshot via parse — only a raw-bytes suffix check catches it. This is the assertion that proves Truncate.
```

## Implementation Blueprint

### Data models and structure

No new types. Two methods restructured; the format string is hoisted into a local buffer.

```go
// internal/lock/lock.go — setSnapshot (nil guard + delegate):
// setSnapshot rewrites the lock file's snapshot= line IN PLACE on the held fd, preserving the cached
// pid/hostname/repo/timestamp plus the new sha. No-op if the lock has already been released.
//
// Issue 4 fix: the rewrite is Seek→Write→Truncate→Sync (see writeContents) — Write BEFORE Truncate so the
// file is NEVER empty during the rewrite (a contender's os.ReadFile in Acquire's EWOULDBLOCK branch never
// observes an empty/partial-diagnostic file). NEVER temp-file+os.Rename: flock is inode-bound and rename
// would orphan the holder's flock, bypassing FR52 contention detection (architecture/flock_inode_constraint.md).
func (l *Locker) setSnapshot(sha string) {
	if l.file == nil {
		return
	}
	l.writeContents(sha)
}

// writeContents writes the lock file contents (pid/hostname/repo/timestamp/snapshot) as a single buffered,
// in-place rewrite on the held fd: Seek(0,0) → Write(full) → Truncate(len) → Sync. Used by BOTH Acquire
// (the initial write) and setSnapshot (the snapshot update).
//
// Write-before-Truncate is the Issue 4 invariant: the file is never empty mid-rewrite — it is always the
// old content, a prefix of the new content, or the complete new content. The single Write overwrites the
// old content from offset 0; Truncate(int64(len(content))) then cuts any stale trailing bytes left from a
// previous, possibly-longer content (the shrink case). The fd/inode is unchanged, so flock semantics hold.
// I/O errors are intentionally ignored (codebase style; the public SetSnapshot signatures are void and a
// write failure is non-fatal — flock still holds; the snapshot is a fast-path/diagnostic nicety).
func (l *Locker) writeContents(snapshot string) {
	content := fmt.Sprintf("pid=%s\nhostname=%s\nrepo=%s\ntimestamp=%s\nsnapshot=%s\n",
		l.pid, l.hostname, l.repo, l.timestamp, snapshot)
	l.file.Seek(0, 0)
	l.file.Write([]byte(content))
	l.file.Truncate(int64(len(content)))
	l.file.Sync()
}
```

> **gofmt note:** run `gofmt -w internal/lock/lock.go internal/lock/lock_test.go`. One doc comment per method
> (citing Issue 4 + the write-ordering invariant + the inode constraint) is required.
>
> **Imports:** lock.go imports are UNCHANGED (`fmt` is already imported; the new code adds no import).
> lock_test.go adds `"strings"` (for `strings.HasSuffix` in the shrink-case assertion).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: lock.go — restructure writeContents (the uniform in-place rewrite primitive)
  - REPLACE writeContents body: build `content := fmt.Sprintf("pid=%s\nhostname=%s\nrepo=%s\ntimestamp=%s\nsnapshot=%s\n",
    l.pid, l.hostname, l.repo, l.timestamp, snapshot)`; then `l.file.Seek(0,0)`; `l.file.Write([]byte(content))`;
    `l.file.Truncate(int64(len(content)))`; `l.file.Sync()`. ORDER: Seek → Write → Truncate → Sync.
  - REFRESH the writeContents doc comment (in-place rewrite; Write-before-Truncate; used by Acquire +
    setSnapshot; errors ignored; fd/inode unchanged → flock holds).
  - GOTCHA: Write BEFORE Truncate (never empty). Do NOT add a nil guard (callers guarantee l.file != nil).
      Do NOT add error returns. Do NOT os.Rename. Keep the exact format string (5 key=value lines, \n-terminated).

Task 2: lock.go — restructure setSnapshot (nil guard + delegate)
  - REPLACE setSnapshot body: `if l.file == nil { return }` then `l.writeContents(sha)`.
  - REFRESH the setSnapshot doc comment (Issue 4; Seek→Write→Truncate via writeContents; never empty; never
    rename — inode constraint; no-op if released).
  - GOTCHA: keep the `if l.file == nil { return }` guard (writeContents has none). The public SetSnapshot
      method + package-level SetSnapshot signatures are UNCHANGED (void).

Task 3: lock_test.go — ADD TestSetSnapshot_FileNeverEmptyWellFormed + "strings" import
  - ADD "strings" to the import block.
  - ADD the test per the sketch below: resetCurrent(t); isolate XDG (t.Setenv XDG_RUNTIME_DIR=t.TempDir() +
    clear XDG_CACHE_HOME); Acquire(t.TempDir()); defer l.Release(). Then:
      (a) assertWellFormed(""): read the file; len>0; parseContents → pid/hostname/repo/timestamp all non-empty;
          snapshot == "".
      (b) SetSnapshot("abc123def456"); assertWellFormed("abc123def456").
      (c) SHRINK: SetSnapshot("lorem-ipsum-dolor-sit-amet-XXXX-YYYY"); assertWellFormed(that). Then
          SetSnapshot("short"); assertWellFormed("short"); AND strings.HasSuffix(rawData, "snapshot=short\n")
          (proves Truncate cut the stale tail — the parse-only check cannot).
  - PATTERN: mirror TestSetSnapshot_UpdatesFile (Acquire + SetSnapshot + ReadFile + parseContents) + the
    newer tests' hygiene (resetCurrent, XDG isolation, read-before-Release).
  - GOTCHA: NO t.Parallel(). Read BEFORE Release (deferred Release is fine). The shrink-case suffix check is
      the meaningful Truncate proof — do NOT drop it for a parse-only check.

Task 4: VERIFY (no further edits)
  - RUN `gofmt -w internal/lock/lock.go internal/lock/lock_test.go`; `go vet ./internal/lock/`;
    `go build ./...`; `go test -race ./internal/lock/ -v`; `go test -race ./...` (no caller regresses).
  - go.mod/go.sum byte-unchanged. default_action.go + platform files + Acquire/Release/lockHash/lockPath
    byte-unchanged. Confirm NO os.Rename in the rewrite path (grep).
```

### Test Specs (lock_test.go — 1 new test)

```go
import (
	"errors"
	"os"
	"path/filepath"
	"strings"   // ADD — for the shrink-case suffix check
	"sync"
	"testing"
)

// TestSetSnapshot_FileNeverEmptyWellFormed verifies Issue 4a's write-ordering invariant: after every
// setSnapshot (Seek→Write→Truncate→Sync, Write-before-Truncate), the lock file is NEVER empty and is
// well-formed (all 5 key=value lines; the 4 diagnostic fields non-empty). A contender's os.ReadFile in
// Acquire's EWOULDBLOCK branch therefore never observes an empty/partial-diagnostic file. Also verifies the
// SHRINK case (long snapshot → short snapshot): Truncate must cut the stale trailing bytes so no garbage
// remains — proven by a raw-bytes suffix check (parseContents alone CANNOT catch a missing Truncate, since
// it silently skips the trailing malformed line).
//
// (The nil/released no-op is already covered by TestSetSnapshot_NilSafeNoOp + TestSetSnapshot_MethodAfterRelease,
// which this change leaves unchanged. A deterministic cross-process race test is not feasible — the window
// is microsecond-wide and contention is nondeterministic; this invariant test is the contract-specified proxy.)
func TestSetSnapshot_FileNeverEmptyWellFormed(t *testing.T) {
	resetCurrent(t)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // isolate — don't touch the real lock dir
	t.Setenv("XDG_CACHE_HOME", "")
	repo := t.TempDir()

	l, err := Acquire(repo)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer l.Release() // reads happen in the body before this runs (Issue 2 removes the file on Release)

	// assertWellFormed reads the file immediately and checks the Issue-4 invariant: non-empty + all 4
	// diagnostic fields present + the expected snapshot. The "never empty immediately after setSnapshot"
	// check is the len(data)==0 assertion inside.
	assertWellFormed := func(t *testing.T, wantSnapshot string) {
		t.Helper()
		data, err := os.ReadFile(l.path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("lock file is EMPTY after writeContents (Issue 4 invariant violated — write-before-truncate broken)")
		}
		c := parseContents(data)
		if c.Pid == "" || c.Hostname == "" || c.Repo == "" || c.Timestamp == "" {
			t.Errorf("empty diagnostic field after rewrite: pid=%q hostname=%q repo=%q timestamp=%q",
				c.Pid, c.Hostname, c.Repo, c.Timestamp)
		}
		if c.Snapshot != wantSnapshot {
			t.Errorf("snapshot = %q, want %q", c.Snapshot, wantSnapshot)
		}
	}

	// (a) The initial write (Acquire → writeContents("")) is well-formed.
	assertWellFormed(t, "")

	// (b) A snapshot update keeps the file non-empty + well-formed.
	SetSnapshot("abc123def456")
	assertWellFormed(t, "abc123def456")

	// (c) SHRINK case: a LONG snapshot followed by a SHORT one. Truncate must cut the stale tail.
	SetSnapshot("lorem-ipsum-dolor-sit-amet-XXXX-YYYY") // 36-char snapshot
	assertWellFormed(t, "lorem-ipsum-dolor-sit-amet-XXXX-YYYY")
	SetSnapshot("short") // shorter than the previous → trailing bytes would remain WITHOUT Truncate
	assertWellFormed(t, "short")

	// The raw-bytes suffix check is the meaningful Truncate proof: if Truncate didn't run, the file tail
	// would be the leftover "...XXXX-YYYY\n" instead of "snapshot=short\n". (parseContents would still
	// report snapshot="short" — it skips the malformed trailing line — so this raw check is required.)
	data, err := os.ReadFile(l.path)
	if err != nil {
		t.Fatalf("ReadFile (shrink): %v", err)
	}
	if !strings.HasSuffix(string(data), "snapshot=short\n") {
		tail := string(data)
		if len(tail) > 80 {
			tail = tail[len(tail)-80:]
		}
		t.Errorf("Truncate did not cut stale trailing bytes (Issue 4): file tail = %q, want suffix %q",
			tail, "snapshot=short\n")
	}
}
```

### Implementation Patterns & Key Details

```go
// THE in-place rewrite (Seek→Write→Truncate→Sync; Write BEFORE Truncate; NEVER rename):
func (l *Locker) writeContents(snapshot string) {
	content := fmt.Sprintf("pid=%s\nhostname=%s\nrepo=%s\ntimestamp=%s\nsnapshot=%s\n",
		l.pid, l.hostname, l.repo, l.timestamp, snapshot)
	l.file.Seek(0, 0)                    // rewind to the start of the SAME inode (flock stays held)
	l.file.Write([]byte(content))        // overwrite from offset 0 (old trailing bytes may remain)
	l.file.Truncate(int64(len(content))) // cut stale trailing bytes to exact length (the shrink case)
	l.file.Sync()                        // durably persist (best-effort; error ignored — codebase style)
}
// setSnapshot: `if l.file == nil { return }; l.writeContents(sha)`.

// WHY Write-before-Truncate (the invariant): during the rewrite the inode's content is one of
//   {old content, prefix of new content, complete new content} — NEVER empty. A contender's os.ReadFile
//   therefore reads at least the leading diagnostic lines (pid/hostname/repo/timestamp come before snapshot).
//   Contrast with the BUG (Truncate(0) before Write): the file is empty in the window.

// WHY Truncate is still needed (the shrink case): when new content is SHORTER than the previous, Write
// overwrites the prefix but leaves the previous tail on disk. Truncate(len(new)) cuts it. Without Truncate,
// the on-disk file is corrupt (parseContents skips the malformed trailing line, so the bug is INVISIBLE to
// parse-only assertions — the test must check raw bytes via strings.HasSuffix).

// WHY no os.Rename (the inode constraint): flock is bound to the inode. Rename installs a new inode at
// lockPath; the holder's flock stays on the old (now-nameless) inode; a contender flocks the new inode and
// SUCCEEDS → FR52 bypassed. In-place rewrite on the held fd is the ONLY safe fix.
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — stdlib only; fmt already imported in lock.go. go mod tidy is a no-op.

PACKAGE EDGES: NONE added/removed. internal/lock is a stdlib-only leaf (no stagecoach imports).

FROZEN / NOT-EDITED:
  - internal/lock/lock.go Acquire (it already calls l.writeContents("") — transparently uses the new
        primitive; the landed Issue-2/Issue-3 code stays), Release (Issue 2 close-then-remove), lockHash/
        lockPath (Issue 3 canonical), parseContents, SetSnapshot (method + package-level signatures).
  - internal/lock/lock_unix.go + lock_windows.go (the fix is in shared lock.go; flock semantics unchanged).
  - internal/cmd/default_action.go handleLockContention (Issue 4b / P1.M2.T4.S1 owns the empty-field guard).
  - All Acquire callers, generate/decompose, docs/*, go.mod/go.sum.

DOWNSTREAM / RELATED (do NOT implement here):
  - P1.M2.T4.S1 (Issue 4b): the contender-side empty-field guard in handleLockContention. Different file.
  - The SetSnapshot public API is UNCHANGED → no downstream caller impact. generate.go/decompose.go call
    lock.SetSnapshot(sha) exactly as before; they transparently get the never-empty rewrite.

NO DATABASE / ROUTES / CONFIG / CLI / NEW FILES / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/lock/lock.go internal/lock/lock_test.go
test -z "$(gofmt -l internal/lock/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/lock/   # catches an unused import / a malformed writeContents / a signature drift.
go build ./...            # whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm NO os.Rename in the rewrite path (the inode constraint — the forbidden pattern):
grep -n 'os.Rename\|Rename(' internal/lock/lock.go && echo "BAD: rename present" || echo "no rename (good — inode constraint honored)"
# Confirm the public SetSnapshot signatures are UNCHANGED (void):
grep -n 'func.*SetSnapshot' internal/lock/lock.go   # expect: `func (l *Locker) SetSnapshot(sha string)` + `func SetSnapshot(sha string)` (no return).
# Confirm default_action.go + platform files are UNCHANGED:
git diff --exit-code internal/cmd/default_action.go internal/lock/lock_unix.go internal/lock/lock_windows.go && echo "default_action + platform files UNCHANGED (expected)"
```

### Level 2: Unit Tests (Component Validation)

```bash
go test -race ./internal/lock/ -v
# Expected PASS — verify explicitly:
#   TestSetSnapshot_FileNeverEmptyWellFormed (NEW): file never empty after writeContents; all diagnostics
#       present; SHRINK case → strings.HasSuffix "snapshot=short\n" (Truncate cut the stale tail).
#   TestSetSnapshot_UpdatesFile: UNCHANGED, still green (SetSnapshot → snapshot=abc123; pid/repo intact).
#   TestSetSnapshot_NilSafeNoOp + TestSetSnapshot_MethodAfterRelease: UNCHANGED, still green (nil/released no-op).
#   TestAcquireRelease_RoundTrip: still green (Acquire's initial write uses the new writeContents; parses clean).
#   TestRelease_RemovesLockFile (Issue 2) + TestAcquire_RepoFieldIsCanonical (Issue 3): still green.
#   All TestLockDir_* / TestHash_* / TestLockPath_* / contention / IsHeldError: unchanged, green.
go test -race ./...   # Full suite — NO regressions (no caller changed; default_action.go untouched).
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only the two listed files changed:
git diff --name-only | grep -Ev 'internal/lock/lock\.go|internal/lock/lock_test\.go' \
  && echo "UNEXPECTED file changed" || echo "only internal/lock/lock.go + lock_test.go changed (good)"
# Confirm the write ordering in the new writeContents (Seek → Write → Truncate → Sync, in that order):
sed -n '/func (l \*Locker) writeContents/,/^}/p' internal/lock/lock.go
# Confirm setSnapshot still has the nil guard + delegates:
sed -n '/func (l \*Locker) setSnapshot/,/^}/p' internal/lock/lock.go
```

### Level 4: Creative & Domain-Specific Validation

```bash
# The race window itself is nondeterministic (microsecond-wide), so a passing/failing cross-process race
# test is NOT a reliable gate — the invariant test (Level 2) is the contract-specified proxy. For manual
# confidence, the e2e contention suite still passes (it asserts the Busy path, not the empty-window):
go test -tags e2e -run TestE2ELockContention ./... 2>/dev/null || echo "(e2e tag optional; the unit invariant test is the gate)"
# golangci-lint (project-wide gate):
make lint 2>/dev/null || golangci-lint run ./internal/lock/ 2>/dev/null || echo "(golangci-lint optional in dev)"
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/lock/`, `go mod tidy` no-op;
      NO `os.Rename` in lock.go; public `SetSnapshot` signatures void/unchanged; default_action.go +
      platform files + go.mod/go.sum byte-unchanged.
- [ ] Level 2 green: `go test -race ./internal/lock/` (new invariant test + all existing) AND `go test -race ./...`.
- [ ] Level 3: binary builds; only `lock.go` + `lock_test.go` changed; write order is Seek→Write→Truncate→Sync.

### Feature Validation

- [ ] `writeContents` does Seek(0,0) → Write([]byte(content)) → Truncate(int64(len(content))) → Sync, in
      that order (Write BEFORE Truncate); builds the full 5-line content via fmt.Sprintf; errors ignored.
- [ ] `setSnapshot` keeps `if l.file == nil { return }` and delegates to `writeContents(sha)`.
- [ ] NO `os.Rename` / temp-file in the rewrite path (inode constraint honored).
- [ ] `TestSetSnapshot_FileNeverEmptyWellFormed`: file never empty + all 4 diagnostics present after every
      write; SHRINK case leaves no stale trailing bytes (`strings.HasSuffix(..., "snapshot=short\n")`).
- [ ] Existing `TestSetSnapshot_*` / `TestAcquireRelease_RoundTrip` / Issue-2 / Issue-3 tests unchanged + green.

### Code Quality Validation

- [ ] `writeContents` is the single in-place rewrite primitive (DRY; used by Acquire + setSnapshot).
- [ ] Doc comments on `setSnapshot` + `writeContents` cite Issue 4, the Write-before-Truncate invariant, and
      the inode (no-rename) constraint.
- [ ] Mirrors existing lock_test.go conventions (white-box, resetCurrent, XDG isolation, read-before-Release).
- [ ] No scope creep into Acquire/Release/lockHash/lockPath/handleLockContention/platform files/docs/callers.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] No docs edits (DOCS: none — internal correctness; P1.M3 owns the doc sweep).
- [ ] go.mod/go.sum byte-unchanged; no new files; no public-signature change.

---

## Anti-Patterns to Avoid

- ❌ Don't use temp-file + `os.Rename` over the lock path. flock is inode-bound; rename orphans the holder's
  flock on the old inode and lets a contender flock the new inode and SUCCEED → FR52 contention detection
  bypassed. ALWAYS rewrite in place on the held fd. (architecture/flock_inode_constraint.md.)
- ❌ Don't Truncate BEFORE Write. That is the CURRENT BUG (empty file in the window). The fix inverts the
  order: Seek → Write → Truncate → Sync (Write-before-Truncate → file is never empty mid-rewrite).
- ❌ Don't drop the `Truncate` call. When the new content is shorter than the previous (the shrink case),
  Write overwrites the prefix but leaves stale trailing bytes. Truncate(len(new)) cuts them. Without it the
  on-disk file is corrupt (invisible to parse-only checks — the test must assert raw bytes).
- ❌ Don't change the public `SetSnapshot` signatures (method or package-level). They are void; the contract
  forbids signature changes. The restructure is entirely within the unexported `setSnapshot`/`writeContents`.
- ❌ Don't add error returns to `writeContents`/`setSnapshot`. Preserve the codebase's ignore-errors style
  (Seek/Truncate/Sync/Fprintf errors are all ignored today). A write failure is non-fatal (flock still holds).
- ❌ Don't add a nil guard to `writeContents`. Its callers (Acquire, setSnapshot) guarantee `l.file != nil`
  (Acquire constructs the Locker with the open fd; setSnapshot guards before delegating). A guard there is
  dead code and diverges from the current shape. Keep the guard in `setSnapshot` only.
- ❌ Don't change the 5-line format string. It must stay `pid=%s\nhostname=%s\nrepo=%s\ntimestamp=%s\nsnapshot=%s\n`
  (parseContents + the existing tests depend on these exact keys/order). Hoist it into a `content` buffer; don't
  alter its contents.
- ❌ Don't touch `Acquire`/`Release`/`lockHash`/`lockPath`/`parseContents`. Acquire already calls
  `l.writeContents("")` and transparently uses the new primitive; the Issue-2/Issue-3 code is already landed.
- ❌ Don't edit `internal/cmd/default_action.go`. The contender-side empty-field guard is Issue 4b
  (P1.M2.T4.S1). This subtask (4a) is the holder-side rewrite only.
- ❌ Don't edit `lock_unix.go`/`lock_windows.go`. The fix is in shared `lock.go`; flock semantics are unchanged.
- ❌ Don't write a deterministic cross-process race test. The window is microsecond-wide and nondeterministic;
  the invariant test (never-empty + shrink/Truncate) is the contract-specified proxy. (4b adds the
  contender-side guard as defense-in-depth.)
- ❌ Don't call `t.Parallel()` in the new test. The `current` singleton is process-global; existing lock tests
  are sequential and use `resetCurrent(t)`.
- ❌ Don't read the lock file AFTER `Release()`. Issue 2 (landed) `os.Remove`s the file on Release →
  `os.ReadFile` would hit `os.IsNotExist`. Use `defer l.Release()` and read in the test body (runs first).
- ❌ Don't replace the shrink-case raw-bytes suffix check with a parse-only check. `parseContents` silently
  skips a malformed trailing line, so a missing-Truncate bug still reports the right snapshot via parse. The
  `strings.HasSuffix(data, "snapshot=short\n")` assertion is what proves Truncate actually ran.
- ❌ Don't add a new import to lock.go (`fmt` is already imported) or a new dependency. go.mod is unchanged.

---

## Confidence Score

**9/10** — a tightly-scoped two-method restructure (Seek→Write→Truncate→Sync; Write-before-Truncate) with the
HARD inode constraint (no rename) called out up front, the latent-Acquire-bug bonus explained, the
ignore-errors style preserved, and the invariant test designed so its shrink-case raw-bytes assertion is the
only check that can actually catch a missing-Truncate regression (parse-only cannot). Both prerequisites
(Issue 2 + Issue 3) are already landed, so the base is stable; no public signature or caller changes; the
contender-side guard is cleanly fenced off to 4b. The -1 reserves for the implementer's choice between Option
A (consolidate into writeContents — chosen, DRY + fixes Acquire) vs Option B (inline in setSnapshot, leave
writeContents) — both correct and invariant-satisfying.
