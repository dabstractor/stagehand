---
name: "P1.M2.T1.S1 (bugfix Issue 2) — Remove lock file in Release() and add cleanup unit test"
description: |
  Bugfix for Issue 2 (Minor — disk hygiene): FR52 per-repo run-lock files accumulate indefinitely.
  `Locker.Release()` (`internal/lock/lock.go`) closes the fd + clears the singleton but never removes
  the `<hash>.lock` file from `$XDG_RUNTIME_DIR/stagecoach/locks/`. flock auto-releases on fd close, so the
  leftovers are inert shells, but they grow without bound (210 observed during QA). Fix: on `Release()`,
  after closing the fd, call `os.Remove(l.path)` ignoring errors — the conventional flock lock-file
  pattern (PRD §18.5 issue-analysis option b). The next `Acquire` recreates the file via
  `OpenFile(O_CREATE|O_RDWR)`, so removal cannot break re-acquisition.

  ⚠️ **THE central design call — CRITICAL ordering: close the fd FIRST (releases the flock), THEN
  `os.Remove`.** The inverted order (remove while still holding the flock on inode A) is a real FR52
  safety bug: unlinking the path lets a contender `OpenFile(path, O_CREATE)` create a NEW inode B and
  flock it (B is free) → both processes believe they hold the lock → two concurrent stagecoach runs on the
  same repo. Close-then-remove guarantees the holder has released before the path is unlinked. `os.Remove`
  errors are IGNORED (file may already be gone, or a concurrent Acquire recreated it — both harmless). The
  idempotency guard (`l.file == nil` → return) is PRESERVED, so a second `Release()` returns before
  reaching Remove (no "file already gone" noise).

  ⚠️ **THE second design call — Windows correctness without build-tag changes.** `lock_windows.go`'s
  `flock` is a no-op (no POSIX flock; the §13.5 update-ref CAS is the real safety guarantee). The remove is
  still correct there: the file is just a marker; `Acquire`'s `OpenFile(O_CREATE)` recreates it. The fix
  lives in `lock.go` (shared, no build tag), so `lock_unix.go`/`lock_windows.go` are UNTOUCHED.

  ⚠️ **THE third design call — capture `path := l.path` into a local before/after nil-ing `l.file`, then
  `os.Remove(path)`.** `l.path` is left untouched on the struct (it remains useful for diagnostics /
  `HeldError.Path`, which is captured at Acquire-contention time, before any Release). The local capture
  is the contract's exact form and is mildly defensive.

  SCOPE: edit `internal/lock/lock.go` (`Release()` — refresh its doc comment + 2 lines: capture path +
  `os.Remove(path)`) and add ONE unit test to `internal/lock/lock_test.go` (`TestRelease_RemovesLockFile`:
  Acquire → file exists → Release → `os.IsNotExist` → second Release no-op → re-Acquire succeeds). `"os"`
  is already imported (no new imports). NO production callers change, NO docs (the cleanup is transparent;
  P1.M3 owns the doc sweep), NO platform-file edits. INPUT = the current `Release()` + `l.path` (the
  `<hash>.lock` path from `lockPath`). OUTPUT = lock files stop accumulating; verified by
  `go test ./internal/lock/`.
---

## Goal

**Feature Goal**: Stop FR52 per-repo run-lock files from accumulating on disk by removing the lock file
(best-effort, error-ignored) inside `Locker.Release()`, after the fd is closed — preserving Release
idempotency and not breaking re-acquisition.

**Deliverable** (edits to two existing files):
1. **`internal/lock/lock.go`** — in `Release()`, after `l.file.Close()` capture `path := l.path`, then
   after `l.file = nil` + the singleton clear, call `os.Remove(path)` (ignore the error). Refresh the
   `Release` doc comment to state it now removes the file (best-effort).
2. **`internal/lock/lock_test.go`** — add `TestRelease_RemovesLockFile`: isolate `XDG_RUNTIME_DIR` to a
   temp dir, `Acquire(repo)` → assert the file exists at `l.path` → `Release()` → assert `os.Stat(l.path)`
   is `os.IsNotExist` → second `Release()` is a no-op (no panic) → a fresh `Acquire(repo)` succeeds (file
   recreated) and is released.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/lock/` clean;
`go test -race ./internal/lock/` green — the new test passes AND all 14 existing lock tests stay green
(the file exists between Acquire and Release; removed only on Release; re-Acquire recreates it).
`os.Remove` errors are ignored; close-then-remove ordering is honored. go.mod/go.sum unchanged; no file
outside `internal/lock/lock.go` + `internal/lock/lock_test.go` is touched.

## User Persona

**Target User**: Long-lived developer machines and CI runners that create many distinct repo paths —
transitively PRD §18.5 "Location/Mechanism" (disk hygiene; the lock spec didn't mandate cleanup).

**Use Case**: Running stagecoach across many repos (or many temp repos per CI job) no longer leaves a
permanent `<hash>.lock` per repo in `$XDG_RUNTIME_DIR/stagecoach/locks/`.

**User Journey**: `stagecoach` runs → `Acquire` creates `<hash>.lock` + flocks it → work → `Release`
closes the fd (auto-releases flock) AND now `os.Remove`s the file → the locks dir stays bounded.

**Pain Points Addressed**: removes unbounded inert lock-file accumulation (210 seen in QA) without
weakening the FR52 lock or changing any user-facing behavior.

## Why

- **Disk hygiene (the headline).** Inert flock-target shells should not accumulate per repo forever; this
  is the conventional "remove lock file on release" pattern for flock-based locks.
- **Safe.** flock auto-releases on fd close; removing the file afterward is harmless — the next `Acquire`
  recreates it via `OpenFile(O_CREATE|O_RDWR)`. The close-then-remove ordering preserves FR52 correctness.
- **Transparent.** No user-facing/config/API/docs surface change — the cleanup just happens. docs/
  configuration.md makes no accumulation claim to correct (P1.M3 owns any doc sweep).
- **Tiny.** One method (+ its doc comment) + one unit test; no new imports, no platform-file changes.

## What

`Locker.Release()` removes the lock file (best-effort) after closing the fd; a unit test pins the
cleanup + idempotency + re-acquisition. No caller changes, no docs, no platform edits.

### Success Criteria

- [ ] `Release()` calls `os.Remove(l.path)` (via a captured local `path`) AFTER `l.file.Close()` + nil-ing
      `l.file` + clearing the singleton; the error is ignored (not even captured into `_` is required, but
      `os.Remove(path)` as a bare statement is fine).
- [ ] The idempotency guard (`l == nil || l.file == nil` → return) is preserved UNCHANGED — a second
      `Release()` returns before reaching `os.Remove`.
- [ ] The `Release` doc comment is updated to state it removes the lock file (best-effort, error-ignored).
- [ ] `TestRelease_RemovesLockFile` exists and passes: Acquire → file exists → Release → `os.IsNotExist` →
      second Release no-op → re-Acquire succeeds.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/lock/`, `go test -race ./internal/lock/` (new +
      existing 14) clean/green; go.mod/go.sum unchanged; only `internal/lock/lock.go` +
      `internal/lock/lock_test.go` touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the current `Release()` body
(quoted below), the close-then-remove ordering rule, and the test pattern (quoted below). No PRD/git/
provider knowledge required — this is a one-method hygiene fix.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M2T1S1/research/release_removes_lock_file.md
  why: the exact Release() before/after, the CRITICAL close-before-remove rationale (avoid a contender
       flocking a fresh inode while the holder still holds the old one), the Windows no-op-flock note,
       and the re-creation-on-next-Acquire proof (OpenFile O_CREATE).
  critical: close FIRST, THEN os.Remove. The inverted order is a real FR52 safety regression.

- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/issue_analysis.md
  section: "Issue 2 (Minor) — Lock files accumulate, never removed"
  why: the root-cause confirmation (Release only closes fd + clears singleton, never removes) and the
       chosen fix (option b — "the conventional pattern for flock lock files", remove-after-close).
  critical: "The remove-after-close ordering is critical: close FIRST (release flock), THEN remove."

- file: internal/lock/lock.go
  section: Release() (~L108-117), Acquire() (the OpenFile(O_CREATE|O_RDWR) re-creation path), lockPath()
  why: the method you edit + the proof that removal is safe (Acquire recreates the file/dir).
  pattern: Release is a value-receiver method on *Locker; the singleton `current atomic.Pointer[Locker]`
           is cleared iff `current.Load() == l`. `"os"` is already imported — NO new import.
  gotcha: preserve the `l.file == nil` idempotency guard verbatim; leave `l.path` on the struct (do NOT
           nil it — HeldError.Path/tests may reference it).

- file: internal/lock/lock_test.go
  why: the test conventions — `package lock` (white-box), the `resetCurrent(t)` helper (clears the
       singleton on cleanup), `t.Setenv` for XDG isolation (see TestLockDir_*), `t.TempDir()` for repos.
  pattern: mirror TestAcquireRelease_RoundTrip's structure (resetCurrent(t); repo:=t.TempDir(); Acquire;
           …; Release) AND TestLockDir_RuntimePreferred's XDG isolation (t.Setenv XDG_RUNTIME_DIR +
           clear XDG_CACHE_HOME). Access `l.path` directly (white-box).
  gotcha: isolate XDG_RUNTIME_DIR to a temp dir in the new test (cleaner than the existing Acquire tests,
           which write to the real lock dir — don't replicate that gap).

- file: internal/lock/lock_unix.go   + internal/lock/lock_windows.go
  why: confirms flock is real on unix (LOCK_EX|LOCK_NB; auto-release on fd close) and a NO-OP on Windows.
       The fix is in lock.go (shared) → NO build-tag edits; both platform files untouched.

- file: docs/configuration.md (lock-file location, ~L233-247)
  why: confirms NO doc edit is needed — it documents the lock-file LOCATION but makes no accumulation
       claim to correct. (The doc sweep is P1.M3's job, not this task.)
```

### Current Codebase tree (relevant slice)

```bash
internal/lock/
  lock.go            # Release() ~L108-117 — EDIT (add os.Remove after Close; refresh doc comment)
  lock_test.go       # 14 tests, package lock (white-box) — EDIT (add TestRelease_RemovesLockFile)
  lock_unix.go       # real flock (LOCK_EX|LOCK_NB) — NO edit
  lock_windows.go    # no-op flock — NO edit
go.mod / go.sum      # unchanged (no new dep; "os" already imported)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Two edits: internal/lock/lock.go (Release + doc comment) + internal/lock/lock_test.go (one test).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: close the fd FIRST (l.file.Close() releases the advisory flock), THEN os.Remove(path).
// Remove-while-held is a real FR52 safety bug: unlinking the path lets a contender OpenFile(O_CREATE) a
// NEW inode and flock it (free) → both processes hold "the lock" → two concurrent stagecoach runs. The
// contract + issue_analysis both mandate close-then-remove.

// CRITICAL: preserve the idempotency guard `if l == nil || l.file == nil { return }` VERBATIM, at the
// top. The second Release() hits this guard (l.file was nil'd by the first) and returns BEFORE os.Remove —
// so there is no "file already removed" noise and no double-remove race from repeated Release calls.

// CRITICAL: ignore the os.Remove error (do not wrap/return it). The file may already be gone, or a
// concurrent Acquire may have recreated then removed it. Both are harmless. A bare `os.Remove(path)`
// statement is fine (Go permits ignoring a returned error for an os.* call at a statement — golangci-lint
// may flag unused results on some funcs, but os.Remove's result is NOT in the default unchecked list; if
// `go vet`/lint complains, write `_ = os.Remove(path)`).

// GOTCHA: capture `path := l.path` and remove THAT (the contract's exact form). Leave l.path on the struct
// (do NOT nil it) — it's harmless and HeldError.Path / existing tests may reference it post-Release.

// GOTCHA: Windows flock is a no-op (lock_windows.go) but the remove is STILL correct there — the file is a
// marker; Acquire's OpenFile(O_CREATE) recreates it. The fix is in lock.go (no build tag) → do NOT touch
// lock_unix.go / lock_windows.go.

// GOTCHA (test): isolate XDG_RUNTIME_DIR to t.TempDir() (t.Setenv) + clear XDG_CACHE_HOME, so the test
// never touches the real $XDG_RUNTIME_DIR/stagecoach/locks/. Call resetCurrent(t) to avoid singleton
// poisoning. Do NOT call t.Parallel() (the existing lock tests don't — the singleton is process-global).
```

## Implementation Blueprint

### Data models and structure

No new types. The only structural change is inside `Release()`:

```go
// internal/lock/lock.go — Release() after the fix:
// Release releases the lock and removes the lock file. Idempotent: a second call is a no-op (the
// l.file==nil guard returns before the remove). Closing the fd auto-releases the flock; this also
// clears the singleton if it points at l, and best-effort removes the lock file (Issue 2 — disk
// hygiene). CRITICAL ORDERING: the fd is closed FIRST (releasing the flock on the inode) and the file
// is removed AFTER, so the remove never races a still-held flock (which would let a contender flock a
// freshly-created inode and defeat FR52). os.Remove errors are ignored (file may already be gone or
// recreated by a concurrent Acquire — both harmless; the next Acquire recreates it via O_CREATE).
func (l *Locker) Release() {
	if l == nil || l.file == nil {
		return
	}
	l.file.Close() // release the flock FIRST
	path := l.path
	l.file = nil
	if current.Load() == l {
		current.Store(nil)
	}
	os.Remove(path) // best-effort cleanup; ignore errors
}
```

```go
// internal/lock/lock_test.go — the new test (white-box, isolated XDG):
func TestRelease_RemovesLockFile(t *testing.T) {
	resetCurrent(t)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // isolate — don't touch the real lock dir
	t.Setenv("XDG_CACHE_HOME", "")
	repo := t.TempDir()

	l, err := Acquire(repo)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	path := l.path

	// Sanity: the lock file exists while held.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file missing immediately after Acquire: %v", err)
	}

	l.Release()

	// Issue 2 fix: Release removes the lock file.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("after Release, os.Stat(lock) err = %v, want os.IsNotExist (file should be removed)", err)
	}

	// Idempotency: a second Release is a no-op (no panic on the now-absent file).
	l.Release()

	// Re-acquisition recreates the file (OpenFile O_CREATE) — removal must not break re-Acquire.
	l2, err := Acquire(repo)
	if err != nil {
		t.Fatalf("re-Acquire after Release: %v", err)
	}
	if _, err := os.Stat(l2.path); err != nil {
		t.Errorf("lock file missing after re-Acquire: %v", err)
	}
	l2.Release()
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: lock.go — Release() removes the lock file (+ doc comment)
  - EDIT Release(): after `l.file.Close()`, add `path := l.path`; keep `l.file = nil` + the
    `if current.Load() == l { current.Store(nil) }` block UNCHANGED; then add `os.Remove(path)`.
  - REFRESH the Release doc comment to state it removes the lock file (best-effort) and WHY close precedes
    remove (the FR52 contender-inode safety argument).
  - GOTCHA: close FIRST, then remove. Preserve the idempotency guard verbatim. No new import ("os" present).

Task 2: lock_test.go — add TestRelease_RemovesLockFile
  - ADD the test per the Data Models block: resetCurrent(t); isolate XDG_RUNTIME_DIR + clear XDG_CACHE_HOME;
    repo:=t.TempDir(); Acquire; capture l.path; assert exists; Release; assert os.IsNotExist; second
    Release (no panic); re-Acquire succeeds + file recreated; Release.
  - PATTERN: mirror TestAcquireRelease_RoundTrip (structure) + TestLockDir_RuntimePreferred (XDG isolation).
  - GOTCHA: white-box `package lock` (l.path accessible); call resetCurrent(t); no t.Parallel().

Task 3: VERIFY (no further file change)
  - RUN `gofmt -w internal/lock/lock.go internal/lock/lock_test.go`; `go vet ./internal/lock/`;
    `go build ./...`; `go test -race ./internal/lock/`; `go test -race ./...` (no regressions elsewhere).
  - go.mod/go.sum byte-unchanged. No file outside internal/lock/ touched.
```

### Implementation Patterns & Key Details

```go
// The close-then-remove invariant — the whole safety argument:
l.file.Close()   // 1. release the advisory flock on the inode
path := l.path   // 2. capture (l.path left on the struct for diagnostics)
l.file = nil     // 3. idempotency: a second Release returns at the top guard
if current.Load() == l { current.Store(nil) } // 4. clear the singleton
os.Remove(path)  // 5. unlink the path AFTER the flock is released; ignore the error

// Why remove is safe re: re-acquisition (Acquire, unchanged):
//   os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)  // recreates the file (and dir via MkdirAll) if absent
//   → a removed lock file is transparent to the next Acquire.
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — no new dep; "os" already imported. go mod tidy MUST be a no-op.

FROZEN / NOT-EDITED:
  - internal/lock/lock_unix.go + lock_windows.go (the fix is in shared lock.go; flock semantics unchanged).
  - internal/lock/lock.go Acquire / setSnapshot / lockPath / lockHash (other lock-hardening tasks own those:
    P1.M2.T2.S1 canonical repo=; P1.M2.T3.S1 atomic setSnapshot; P1.M2.T4.S1 contention guard).
  - internal/e2e/lock_scenarios_test.go (P1.M1.T2.S1, parallel — different file).
  - docs/* (P1.M3 owns the doc sweep; docs/configuration.md makes no accumulation claim to correct).
  - All Release() callers (default_action.go, generate, decompose) — Release's contract is unchanged
    (still idempotent, still nil-safe); it just also removes the file now.

NO DATABASE / NO ROUTES / NO CONFIG / NO CLI / NO NEW FILES.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/lock/lock.go internal/lock/lock_test.go
test -z "$(gofmt -l internal/lock/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/lock/        # Catches a stray unused/err. (os.Remove's result is OK to discard.)
go build ./...                 # Whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: clean. If go vet/lint flags the discarded os.Remove result, write `_ = os.Remove(path)`.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test -race ./internal/lock/ -v        # the new TestRelease_RemovesLockFile + all 14 existing tests
# Expected: 15/15 PASS. Key assertions: after Release os.Stat(lock) ⇒ os.IsNotExist; second Release is a
#   no-op; re-Acquire succeeds and recreates the file. The existing contention/round-trip/setSnapshot tests
#   stay green (file exists between Acquire and Release; re-Acquire recreates it).
go test -race ./...                      # Full suite — NO regressions (no caller changed).
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only the two listed files changed:
git diff --name-only | grep -Ev 'internal/lock/lock\.go|internal/lock/lock_test\.go' \
  && echo "UNEXPECTED file changed" || echo "only internal/lock/lock.go + lock_test.go changed (good)"
# Manual hygiene smoke (optional): run a stagecoach invocation in a temp repo, then confirm the lock file is
# gone after exit:
#   TMP=$(mktemp -d); git -C "$TMP" init -q; … run stagecoach … ; ls $XDG_RUNTIME_DIR/stagecoach/locks/
#   (expect: the <hash>.lock for $TMP is absent post-run; previously it was left behind).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Concurrency sanity (optional): the close-then-remove ordering is the safety keystone. A focused test
# could Acquire in one goroutine, Release in another, and a contender-Acquire in a third — but the
# microsecond race window is impractical to hit deterministically, and the existing
# TestAcquire_Contention_HeldError already pins Release→re-Acquire correctness. The unit test in Task 2
# is sufficient. golangci-lint: `make lint` (project-wide gate).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/lock/`, `go mod tidy` no-op.
- [ ] Level 2 green: `go test -race ./internal/lock/` (new + 14 existing) AND `go test -race ./...`.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only the two listed files changed.

### Feature Validation

- [ ] `Release()` calls `os.Remove(l.path)` (via captured local) AFTER `Close()` + nil + singleton clear.
- [ ] Close-then-remove ordering honored; `os.Remove` error ignored.
- [ ] Idempotency guard preserved (second Release is a no-op).
- [ ] `TestRelease_RemovesLockFile` passes: file gone after Release; re-Acquire recreates it.

### Code Quality Validation

- [ ] Mirrors existing `lock_test.go` conventions (white-box, resetCurrent, t.Setenv XDG isolation).
- [ ] Release doc comment refreshed to document the removal + the close-before-remove rationale.
- [ ] No scope creep into Acquire/setSnapshot/platform files/docs (other tasks own those).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] No docs edits (DOCS: none — the cleanup is transparent; docs/configuration.md needs no change;
      P1.M3 owns the doc sweep).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ Don't `os.Remove` BEFORE `l.file.Close()`. Removing while still holding the flock lets a contender
  create+ flock a fresh inode → both processes hold "the lock" → FR52 defeated. Close FIRST, then remove.
- ❌ Don't remove the `if l == nil || l.file == nil { return }` idempotency guard or move the remove above
  it — the second Release must be a no-op (it returns before reaching os.Remove).
- ❌ Don't return/wrap the `os.Remove` error. It's best-effort; the file may already be gone or recreated by
  a concurrent Acquire. A bare `os.Remove(path)` (or `_ = os.Remove(path)` if lint insists) is correct.
- ❌ Don't nil `l.path` on the struct. Leave it — it's harmless and may be referenced for diagnostics /
  HeldError.Path (captured at Acquire-contention time, pre-Release).
- ❌ Don't touch `lock_unix.go` / `lock_windows.go`. The fix is in shared `lock.go`; flock semantics are
  unchanged (real on unix, no-op on Windows where the file is just a marker and O_CREATE recreates it).
- ❌ Don't add a new import or dependency. `"os"` is already imported in lock.go.
- ❌ Don't isolate the new test by writing to the real `$XDG_RUNTIME_DIR` (as some existing Acquire tests
  do). Use `t.Setenv("XDG_RUNTIME_DIR", t.TempDir())` + clear `XDG_CACHE_HOME` — the cleaner pattern.
- ❌ Don't call `t.Parallel()` in the new test — the `current` singleton is process-global; the existing
  lock tests are sequential and use `resetCurrent(t)`.
- ❌ Don't edit docs/configuration.md or any other doc — the cleanup is transparent; P1.M3 owns the doc sweep.
- ❌ Don't change go.mod/go.sum or add new files. This is a one-method hygiene fix + one test.
- ❌ Don't skip `go test -race ./internal/lock/` — it confirms the singleton/flock interaction stays correct
  and the new test reliably observes the removal.
