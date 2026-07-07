---
name: "P1.M2.T2.S1 (bugfix Issue 3) — Store canonical path in repo= field + symlink diagnostic test"
description: |
  Bugfix for Issue 3 (Minor — diagnostic accuracy): the FR52 lock file's `repo=` field stores the RAW
  `repoPath` (the holder's CWD, possibly a symlink) while the lock FILENAME hash uses the CANONICAL path
  (`filepath.EvalSymlinks` in `lockHash`). For a repo reached via a symlink, the contention message prints
  a path that differs from a contender's own CWD — confusing "is that my repo?" Fix: make `lockHash` return
  BOTH the canonical path and the hash (single canonicalization source), and have `Acquire` store the
  canonical path in the `repo=` field. The filename/hash is UNCHANGED (same canonical → same sha256), so
  lock identity, contention, and the no-op fast path are unaffected — only the diagnostic string improves.
  `handleLockContention` (`internal/cmd/default_action.go:254`) prints `Contents.Repo` verbatim, so the
  contention message auto-fixes with NO caller/edit change.

  ⚠️ **THE central design call — `lockHash` returns `(canonical, hash)`; `lockPath` discards the canonical;
  `Acquire` reuses it for `repo=`.** The item's preferred Option 1. `lockPath`'s SIGNATURE stays
  `(string, error)` (only its body changes: `_, hash := lockHash(repoPath)`), so `lockPath`-based tests are
  unaffected. `Acquire` adds ONE line (`canonical, _ := lockHash(repoPath)`) and changes the struct literal
  `repo: repoPath` → `repo: canonical`. `lockHash` is deterministic, so calling it twice in `Acquire` (once
  via `lockPath`, once directly) is safe; the second `EvalSymlinks` is a cheap syscall on a once-per-run
  path. `lockHash` is the SOLE canonicalization site (DRY — no duplicated EvalSymlinks logic).

  ⚠️ **THE non-obvious ripple — two existing tests assert `c.Repo == repo` (raw `t.TempDir()`) and will
  BREAK on macOS after the fix.** `t.TempDir()` on macOS is `/var/folders/...` and `/var → /private/var`,
  so `EvalSymlinks(repo) != repo`. `TestAcquireRelease_RoundTrip` (L199) and `TestSetSnapshot_UpdatesFile`
  (L225) BOTH assert `c.Repo != repo`; after the fix `c.Repo` is canonical, so both must compare against
  `canonical, _ := lockHash(repo)` (the robust, cross-platform oracle — reuses the function under test).
  `TestHash_CanonicalSymlink` (L108/109/114) must update for the new `lockHash` signature
  (`_, hash := lockHash(...)` ×3). These three test edits are REQUIRED (not optional) — without them the
  suite fails on macOS (and any Linux box whose tmpdir root is a symlink).

  ⚠️ **THE composition constraint — read lock contents BEFORE `Release()` (Issue 2 landed).** The parallel
  task P1.M2.T1.S1 (Issue 2) has ALREADY landed in the working tree: `Release()` now `os.Remove`s the lock
  file after closing the fd, and `TestRelease_RemovesLockFile` exists. So the new symlink test (and the
  existing content-reading tests) MUST `os.ReadFile(l.path)` BEFORE `l.Release()` — the file is gone after
  Release. All three affected tests already read-then-Release; the new test must follow suit (NO deferred
  read-after-Release).

  ⚠️ **NO caller/change outside `internal/lock/`.** `handleLockContention` (default_action.go:254) prints
  `Contents.Repo` with no transformation → the Busy message auto-shows the canonical path once Acquire
  stores it. `lockHash`/`lockPath` are unexported (leaf package) → no external callers. default_action.go,
  generate, decompose, docs, platform files are ALL UNTOUCHED.

  Deliverable: edits to `internal/lock/lock.go` (`lockHash` signature + body; `lockPath` body; `Acquire`
  one-line + struct-literal; doc comments) + edits to `internal/lock/lock_test.go` (3 existing tests
  updated + 1 new `TestAcquire_RepoFieldIsCanonical`). NO new files, NO new imports (`path/filepath` +
  `crypto/sha256` already imported), NO go.mod change, NO docs. OUTPUT: the `repo=` field and the contention
  message name the canonical repo path; verified by `go test ./internal/lock/`.
---

## Goal

**Feature Goal**: Make the FR52 lock file's `repo=` diagnostic field store the CANONICAL repo path
(`filepath.EvalSymlinks`, with `filepath.Abs` fallback) — the same canonicalization the lock filename hash
already uses — so two terminals in the same repo (one via a symlink) see an unambiguous, agreed-upon repo
path in the contention message. The filename/hash is unchanged; only the diagnostic string improves.

**Deliverable** (edits to two existing files):
1. **`internal/lock/lock.go`** — `lockHash(repoPath string) (canonical, hash string)` (returns both);
   `lockPath` body uses `_, hash := lockHash(repoPath)`; `Acquire` adds `canonical, _ := lockHash(repoPath)`
   and stores `repo: canonical` (was `repo: repoPath`); refresh doc comments (lockHash = single canonical
   source; Acquire repo= is canonical per Issue 3).
2. **`internal/lock/lock_test.go`** — update `TestHash_CanonicalSymlink` (new signature, `_, hash :=` ×3);
   update `TestAcquireRelease_RoundTrip` + `TestSetSnapshot_UpdatesFile` to compare `c.Repo` against
   `canonical, _ := lockHash(repo)` (the macOS-symlink-tmpdir fix); ADD `TestAcquire_RepoFieldIsCanonical`
   (the symlink-diagnostic headline test).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/lock/` clean;
`go test -race ./internal/lock/` green — the new test passes (Acquire via a symlink writes the canonical
real path to `repo=`, not the symlink) AND all existing lock tests stay green (incl. on macOS, where the
two `c.Repo` assertions now compare against canonical). `go test -race ./...` green (no caller changed).
go.mod/go.sum unchanged; no file outside `internal/lock/lock.go` + `internal/lock/lock_test.go` touched.

## User Persona

**Target User**: A developer working in a repo reached through a symlink (e.g. `~/code` → `/Volumes/Work/code`
or `/var → /private/var` on macOS) who hits FR52 contention and needs to confirm "is that other stagecoach
run in MY repo?" Transitively PRD §18.5 "Contents" (the contention message names *who* holds the lock).

**Use Case**: Terminal A runs `stagecoach` from a symlinked path; Terminal B (in the same repo via the real
path) runs `stagecoach` and gets the Busy message. The message's `repo=` must show a path B recognizes as
the same repo — the canonical real path, not A's symlink.

**User Journey**: `Acquire(os.Getwd())` → `repo:=canonical` written to `<hash>.lock` → contender's
`Acquire` fails with `*HeldError` → `handleLockContention` prints `Contents.Repo` (now canonical) → the
contender sees the unambiguous canonical path. (Today `repo:=repoPath` prints the holder's raw CWD.)

**Pain Points Addressed**: removes "the Busy message showed a weird path that isn't my CWD — is that really
my repo?" for symlinked checkouts. Diagnostic-only; no functional/lock-correctness change.

## Why

- **Fixes the Issue-3 diagnostic mismatch.** The hash (filename) is already canonical; making `repo=` match
  it removes the symlink-path confusion. The fix is one canonicalization source + one field assignment.
- **Safe / zero functional impact.** The hash input (canonical) is UNCHANGED → same `<hash>.lock` filename →
  lock identity, contention, the no-op fast path, and Issue-2 cleanup are all unaffected. Only the
  diagnostic `repo=` string changes (raw → canonical).
- **DRY.** `lockHash` becomes the single canonicalization source; `Acquire` reuses its canonical return
  instead of re-implementing EvalSymlinks (the temptation that would diverge from the hash's canonical).
- **Auto-fixes the contention message.** `handleLockContention` prints `Contents.Repo` verbatim → no caller
  edit needed; the message improves transparently.
- **No new surface.** DOCS: none (diagnostic-only; P1.M3 owns any doc sweep).

## What

`lockHash` returns `(canonical, hash)`; `Acquire` stores the canonical in `repo=`; three existing tests
adapt (signature + canonical-aware `c.Repo` assertions); one new symlink-diagnostic test is added. No
caller outside `internal/lock/` changes (the contention message formats `Contents.Repo` directly).

### Success Criteria

- [ ] `lockHash(repoPath string) (canonical, hash string)` returns both (EvalSymlinks → Abs fallback →
      sha256 hex). It is the SOLE canonicalization site in the package.
- [ ] `lockPath(repoPath string) (string, error)` signature UNCHANGED; body uses `_, hash := lockHash(repoPath)`.
- [ ] `Acquire` stores `repo: canonical` (via `canonical, _ := lockHash(repoPath)`), NOT `repo: repoPath`.
      The `repoPath` parameter name is unchanged (still `os.Getwd()` from callers).
- [ ] `TestHash_CanonicalSymlink` updated for the new signature (`_, hash := lockHash(...)` ×3).
- [ ] `TestAcquireRelease_RoundTrip` + `TestSetSnapshot_UpdatesFile` compare `c.Repo` against
      `canonical, _ := lockHash(repo)` (macOS-symlink-tmpdir-safe), NOT the raw `repo`.
- [ ] `TestAcquire_RepoFieldIsCanonical` added: Acquire via a symlink → read lock file BEFORE Release →
      `parseContents` → `c.Repo == canonical` AND `c.Repo != link`.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/lock/`, `go test -race ./internal/lock/`,
      `go test -race ./...` clean/green; go.mod/go.sum unchanged; only `lock.go` + `lock_test.go` touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact `lockHash`/`lockPath`/
`Acquire` before/after (quoted below), the macOS-symlink-tmpdir ripple (the two `c.Repo` assertions), the
new-test design (with a concrete code sketch), and the read-before-Release constraint (Issue 2 landed).
No PRD/git/provider knowledge required — this is a one-function-signature + one-field refactor + tests.

### Documentation & References

```yaml
# MUST READ — the authoritative research (every edit + the macOS ripple + the new test sketch)
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M2T2S1/research/canonical-repo-field.md
  why: the lockHash/lockPath/Acquire before/after (§1), the FULL test-ripple table + the macOS-symlink-tmpdir
       break (§2 — the non-obvious REQUIRED edit to TestAcquireRelease_RoundTrip + TestSetSnapshot_UpdatesFile),
       the Issue-2 composition constraint (§3 — read before Release), default_action.go auto-fix proof (§4),
       scope fences (§5), validation commands (§6).
  critical: §2 — without updating the two `c.Repo != repo` assertions, the suite FAILS on macOS (t.TempDir()
       is under /var → /private/var). Use `canonical, _ := lockHash(repo)` as the oracle.

# The bug report + root cause
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/issue_analysis.md
  section: "Issue 3 (Minor) — contention message names the holder's repo via the non-canonical CWD path"
  why: confirms the hash IS canonical (lockHash EvalSymlinks) but repo= is the raw repoPath; the fix is to
       "Store the canonicalized path in the repo= field (reuse the canonical value already computed in
       lockHash)."

# The file being fixed
- file: internal/lock/lock.go
  section: lockHash (L216) + lockPath (L227) + Acquire (L66; the `repo: repoPath` literal is at the Locker struct).
  why: the three functions you edit. lockHash currently returns ONLY the hash (discards canonical); Acquire
       stores `repo: repoPath` (raw). lockPath's body calls `lockHash(repoPath)+".lock"`.
  pattern: lockHash returns `(canonical, hash)`; lockPath does `_, hash := lockHash(repoPath)`; Acquire adds
       `canonical, _ := lockHash(repoPath)` and stores `repo: canonical`.
  gotcha: keep the `repoPath` parameter name in Acquire (callers pass os.Getwd()); only the struct-literal
       field value changes. lockPath's SIGNATURE must NOT change (2 tests depend on `(string, error)`).

# The tests to update + the pattern for the new one
- file: internal/lock/lock_test.go
  section: TestHash_CanonicalSymlink (L108/109/114), TestAcquireRelease_RoundTrip (L199), TestSetSnapshot_UpdatesFile
           (L225), TestRelease_RemovesLockFile (L155 — Issue 2, the XDG-isolation + resetCurrent pattern to mirror).
  why: the 3 tests that MUST change (signature; canonical-aware c.Repo) + the clean pattern (resetCurrent,
       t.Setenv XDG_RUNTIME_DIR, read-before-Release) for the new test.
  pattern: white-box `package lock`; call resetCurrent(t); isolate XDG (t.Setenv XDG_RUNTIME_DIR=t.TempDir()
           + clear XDG_CACHE_HOME); reuse `canonical, _ := lockHash(repo)` as the oracle; read os.ReadFile(l.path)
           BEFORE l.Release() (Issue 2 removes the file on Release).
  gotcha: do NOT call t.Parallel() (the `current` singleton is process-global; existing lock tests are sequential).

# The auto-fixing consumer (NO edit — just the proof it auto-fixes)
- file: internal/cmd/default_action.go
  section: handleLockContention (L241-254; L254 formats heldErr.Contents.Repo directly).
  why: proves the contention message prints Contents.Repo verbatim (no transformation) → once Acquire stores
       canonical, the message auto-shows it. NO edit to default_action.go or any caller.
  critical: do NOT edit default_action.go — it consumes Contents.Repo unchanged.

# The prerequisite (LANDED — composition constraint)
- file: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M2T1S1/PRP.md
  why: Issue 2 (Release removes the lock file) has LANDED in lock.go (Release already does close→os.Remove;
       TestRelease_RemovesLockFile exists). So the new test must read the lock file BEFORE Release.
  critical: read-before-Release. A deferred read-after-Release would hit os.IsNotExist (Issue 2 removes it).
```

### Current Codebase tree (relevant slice)

```bash
internal/lock/
  lock.go            # lockHash (L216) + lockPath (L227) + Acquire (L66, repo literal) — EDIT (3 fns + doc comments)
  lock_test.go       # TestHash_CanonicalSymlink + RoundTrip + SetSnapshot_UpdatesFile — EDIT; ADD TestAcquire_RepoFieldIsCanonical
  lock_unix.go       # real flock — NO edit
  lock_windows.go    # no-op flock — NO edit
internal/cmd/default_action.go   # handleLockContention prints Contents.Repo — NO edit (auto-fixes)
go.mod / go.sum      # unchanged (no new dep; path/filepath + crypto/sha256 already imported)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Edits to internal/lock/lock.go (lockHash/lockPath/Acquire + doc comments) +
# internal/lock/lock_test.go (3 updates + 1 new test).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (macOS ripple): t.TempDir() on macOS is /var/folders/... and /var → /private/var, so
// filepath.EvalSymlinks(t.TempDir()) != t.TempDir(). After the fix, c.Repo is the canonical resolved path,
// so `c.Repo != repo` (raw) FAILS on macOS. Use `canonical, _ := lockHash(repo)` as the expected value in
// TestAcquireRelease_RoundTrip + TestSetSnapshot_UpdatesFile (robust on every platform).

// CRITICAL (Issue 2 composition): Release() now os.Remove's the lock file (P1.M2.T1.S1 landed). The new
// test (and the existing content-reading tests) MUST os.ReadFile(l.path) BEFORE l.Release(). Reading after
// Release hits os.IsNotExist. (RoundTrip/SetSnapshot already read-then-Release; the new test must too.)

// CRITICAL: lockPath's SIGNATURE must stay (string, error). Only its body changes (`, hash := lockHash`).
// TestAcquire_PathMatchesLockPath + TestLockPath_CanonicalSymlink depend on the (string, error) signature.

// CRITICAL: lockHash is the SOLE canonicalization site. Do NOT add a second filepath.EvalSymlinks in Acquire
// or a sibling helper — reuse lockHash's canonical return (DRY; avoids divergence between the hash and repo=).

// GOTCHA: calling lockHash twice in Acquire (once via lockPath, once directly for canonical) is safe — it is
// deterministic. The second EvalSymlinks is a cheap syscall on a once-per-run path. (If you prefer to invoke
// EvalSymlinks literally once, inline lockDir+hash in Acquire and skip lockPath — but that bypasses the
// lockPath abstraction; the double-call is the lower-risk choice and matches the item's clauses.)

// GOTCHA: keep the repoPath parameter name in Acquire (callers in default_action.go pass os.Getwd()). Only
// the struct-literal `repo:` value changes (repoPath → canonical). Renaming the param is gratuitous churn.

// GOTCHA: the filename/hash is UNCHANGED — same canonical input → same sha256 → same <hash>.lock. So lock
// identity, contention, the no-op fast path, and Issue-2 cleanup are all unaffected. Do NOT "also" change
// the hash input; this task is repo=-field-only.

// GOTCHA: do NOT edit default_action.go / generate / decompose. handleLockContention formats
// heldErr.Contents.Repo verbatim (L254) → the message auto-fixes. The change is internal to internal/lock/.

// GOTCHA (test): reuse `canonical, _ := lockHash(repo)` as the expected-value oracle in the updated tests
// (NOT a separate filepath.EvalSymlinks call) — it reuses the function under test and cannot diverge from it.
```

## Implementation Blueprint

### Data models and structure

No new types. `lockHash` gains a second return value; `Acquire` gains one local + one changed field.

```go
// internal/lock/lock.go — lockHash (return both; single canonicalization source):
// lockHash returns the repo's canonical path and its sha256 hex hash. The canonical path (EvalSymlinks,
// falling back to Abs) is the single source of truth reused by BOTH the lock filename (hash) and the
// diagnostic repo= field (Issue 3) — the two always agree for a symlinked checkout.
func lockHash(repoPath string) (canonical, hash string) {
	canonical, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		canonical, _ = filepath.Abs(repoPath)
	}
	sum := sha256.Sum256([]byte(canonical))
	return canonical, hex.EncodeToString(sum[:])
}

// lockPath — body only: discard the canonical, keep the hash. Signature UNCHANGED ((string, error)).
func lockPath(repoPath string) (string, error) {
	dir, err := lockDir()
	if err != nil {
		return "", err
	}
	_, hash := lockHash(repoPath)
	return filepath.Join(dir, hash+".lock"), nil
}

// Acquire — add the canonical line; change the repo field. (Other lines unchanged.)
func Acquire(repoPath string) (*Locker, error) {
	path, err := lockPath(repoPath)
	if err != nil {
		return nil, fmt.Errorf("lock path: %w", err)
	}
	canonical, _ := lockHash(repoPath) // Issue 3: canonical path for the repo= diagnostic
	// ... MkdirAll / OpenFile / flock / EWOULDBLOCK-contention unchanged ...
	l := &Locker{
		file:      f,
		path:      path,
		pid:       pid,
		hostname:  host,
		repo:      canonical, // Issue 3: canonical (was repoPath — raw CWD/symlink)
		timestamp: ts,
	}
	l.writeContents("")
	current.Store(l)
	return l, nil
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: lock.go — lockHash returns (canonical, hash); lockPath body; Acquire stores canonical
  - EDIT lockHash: signature `func lockHash(repoPath string) (canonical, hash string)`; body returns both
    (canonical via EvalSymlinks→Abs; hash via sha256). Refresh the doc comment (single canonical source;
    reused by filename + repo= per Issue 3).
  - EDIT lockPath body: `_, hash := lockHash(repoPath)` (was `lockHash(repoPath)+".lock"`). Signature UNCHANGED.
  - EDIT Acquire: add `canonical, _ := lockHash(repoPath)` right after the `lockPath` call; change the
    Locker literal `repo: repoPath` → `repo: canonical`.
  - GOTCHA: keep repoPath param name; do NOT change lockPath's signature; lockHash is the sole EvalSymlinks
    site; the filename/hash is unchanged.
  - WHY ONE TASK: the three edits are tightly coupled (lockHash's new return feeds lockPath + Acquire); a
    partial edit would not compile (lockPath/Acquire would mismatch lockHash's signature).

Task 2: lock_test.go — update 3 existing tests for the signature + canonical repo field
  - EDIT TestHash_CanonicalSymlink (L108/109/114): `hash1 := lockHash(tmpRepo)` → `_, hash1 := lockHash(tmpRepo)`
    (and hash2, hash3). Keep the symlink-equality + determinism assertions (they're on the hashes).
  - EDIT TestAcquireRelease_RoundTrip (L199): before the assertion add `canonical, _ := lockHash(repo)`;
    change `if c.Repo != repo` → `if c.Repo != canonical`. (macOS-symlink-tmpdir fix.)
  - EDIT TestSetSnapshot_UpdatesFile (L225): same — `canonical, _ := lockHash(repo)`; `if c.Repo != canonical`.
  - GOTCHA: use lockHash as the oracle (not a separate EvalSymlinks). These edits are REQUIRED (the suite
    fails on macOS without them).

Task 3: lock_test.go — ADD TestAcquire_RepoFieldIsCanonical (the headline symlink diagnostic test)
  - ADD the test per the sketch below: resetCurrent(t); isolate XDG (t.Setenv XDG_RUNTIME_DIR=t.TempDir() +
    clear XDG_CACHE_HOME); create realRepo + a symlink `link`; `canonical, _ := lockHash(link)`; Acquire(link);
    READ os.ReadFile(l.path) BEFORE Release; parseContents; assert c.Repo==canonical AND c.Repo!=link; Release.
  - PATTERN: mirror TestRelease_RemovesLockFile (XDG isolation + resetCurrent) + TestHash_CanonicalSymlink
    (the symlink fixture). white-box `package lock`.
  - GOTCHA: read BEFORE Release (Issue 2 removes the file). No t.Parallel().

Task 4: VERIFY (no further edits)
  - RUN `gofmt -w internal/lock/lock.go internal/lock/lock_test.go`; `go vet ./internal/lock/`;
    `go build ./...`; `go test -race ./internal/lock/ -v`; `go test -race ./...` (no caller regresses).
  - go.mod/go.sum byte-unchanged. default_action.go + platform files byte-unchanged. lockHash is the sole
    EvalSymlinks site (grep confirms).
```

### Implementation Patterns & Key Details

```go
// THE single-source canonicalization (lockHash returns both; reused, not duplicated):
func lockHash(repoPath string) (canonical, hash string) {
	canonical, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		canonical, _ = filepath.Abs(repoPath)
	}
	sum := sha256.Sum256([]byte(canonical))
	return canonical, hex.EncodeToString(sum[:])
}
// lockPath: `_, hash := lockHash(repoPath)` (signature unchanged).
// Acquire:  `canonical, _ := lockHash(repoPath)` then `repo: canonical`.

// THE macOS-safe oracle for the updated c.Repo assertions (reuse the function under test):
canonical, _ := lockHash(repo)
if c.Repo != canonical {
	t.Errorf("repo = %q, want canonical %q", c.Repo, canonical)
}
```

```go
// lock_test.go — the NEW symlink-diagnostic test (the headline deliverable):
func TestAcquire_RepoFieldIsCanonical(t *testing.T) {
	resetCurrent(t)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // isolate — don't touch the real lock dir
	t.Setenv("XDG_CACHE_HOME", "")

	tmpDir := t.TempDir()
	realRepo := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(realRepo, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	link := filepath.Join(tmpDir, "link")
	if err := os.Symlink(realRepo, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	canonical, _ := lockHash(link) // the expected canonical (EvalSymlinks(link) == realRepo)

	l, err := Acquire(link) // acquire via the SYMLINK (raw path)
	if err != nil {
		t.Fatalf("Acquire via symlink: %v", err)
	}
	// Read the lock file BEFORE Release — Issue 2 (P1.M2.T1.S1) removes the file on Release.
	data, err := os.ReadFile(l.path)
	if err != nil {
		l.Release()
		t.Fatalf("ReadFile before Release: %v", err)
	}
	l.Release()

	c := parseContents(data)
	if c.Repo != canonical {
		t.Errorf("repo field = %q, want canonical %q (Issue 3: repo= must be canonical, not the raw symlink %q)",
			c.Repo, canonical, link)
	}
	if c.Repo == link {
		t.Errorf("repo field is the raw symlink path %q — Issue 3 not fixed", link)
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — no new dep; "path/filepath" + "crypto/sha256" already imported.
      go mod tidy MUST be a no-op.

PACKAGE EDGES: NONE added/removed. internal/lock is a stdlib-only leaf (no stagecoach imports).

FROZEN / NOT-EDITED:
  - internal/cmd/default_action.go (handleLockContention prints Contents.Repo verbatim → auto-fixes).
  - internal/lock/lock_unix.go + lock_windows.go (the fix is in shared lock.go; flock semantics unchanged).
  - internal/lock/lock.go Release/setSnapshot/lockDir/writeContents/parseContents (Issue 2 landed on Release;
    Issue 4a/4b own setSnapshot/handleLockContention — different functions/file).
  - All Acquire callers (default_action passes os.Getwd(); the repo field is diagnostic-only).
  - docs/* (DOCS: none — P1.M3 owns the doc sweep).

DOWNSTREAM / RELATED (do NOT implement here):
  - P1.M2.T3.S1 (Issue 4a): atomic setSnapshot (Seek→Write→Truncate). Different function.
  - P1.M2.T4.S1 (Issue 4b): guard handleLockContention against empty diagnostic fields. Different file.
  - The hash/filename is UNCHANGED → no downstream lock-identity impact.

NO DATABASE / ROUTES / CONFIG / CLI / NEW FILES / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/lock/lock.go internal/lock/lock_test.go
test -z "$(gofmt -l internal/lock/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/lock/   # catches a malformed lockHash return / an unused `canonical` / a lockPath mismatch.
go build ./...            # whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm lockHash is the SOLE canonicalization site (no stray EvalSymlinks added in Acquire):
grep -n 'EvalSymlinks\|filepath.Abs' internal/lock/lock.go   # expect: ONLY inside lockHash.
# Confirm default_action.go is UNCHANGED (the message auto-fixes; no edit):
git diff --exit-code internal/cmd/default_action.go && echo "default_action.go UNCHANGED (expected)"
```

### Level 2: Unit Tests (Component Validation)

```bash
go test -race ./internal/lock/ -v
# Expected PASS — verify explicitly:
#   TestAcquire_RepoFieldIsCanonical (NEW): Acquire via symlink → repo= == canonical (real path), != symlink.
#   TestHash_CanonicalSymlink: updated `_, hash :=` ×3; symlink-equality + determinism still hold.
#   TestAcquireRelease_RoundTrip: c.Repo == canonical (macOS-safe oracle) — NOT the raw repo.
#   TestSetSnapshot_UpdatesFile: c.Repo == canonical after SetSnapshot.
#   TestAcquire_PathMatchesLockPath / TestLockPath_CanonicalSymlink: UNCHANGED, still green.
#   TestRelease_RemovesLockFile (Issue 2): still green.
#   All TestLockDir_* / contention / IsHeldError: unchanged, green.
go test -race ./...   # Full suite — NO regressions (no caller changed; default_action.go untouched).
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only the two listed files changed:
git diff --name-only | grep -Ev 'internal/lock/lock\.go|internal/lock/lock_test\.go' \
  && echo "UNEXPECTED file changed" || echo "only internal/lock/lock.go + lock_test.go changed (good)"
# Confirm lockHash's new signature is consumed everywhere (no stale 1-return call):
grep -n ':= lockHash(' internal/lock/   # every call site must bind BOTH returns (canonical, hash) or (_, hash).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Cross-platform sanity (the macOS ripple is the risk): if running on macOS, `go test -race ./internal/lock/`
# directly exercises the /var → /private/var symlink case for t.TempDir(). On Linux, the canonical usually
# equals the raw tmpdir, but the `canonical, _ := lockHash(repo)` oracle is correct regardless. The new
# TestAcquire_RepoFieldIsCanonical proves the symlink→canonical behavior on EVERY platform (it constructs
# the symlink explicitly). golangci-lint: `make lint` (project-wide gate).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/lock/`, `go mod tidy` no-op;
      lockHash is the sole EvalSymlinks site; default_action.go + go.mod/go.sum byte-unchanged.
- [ ] Level 2 green: `go test -race ./internal/lock/` (new + 3 updated + all others) AND `go test -race ./...`.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only lock.go + lock_test.go changed; every
      `lockHash(` call site binds both returns.

### Feature Validation

- [ ] `lockHash(repoPath) (canonical, hash string)` returns both; `lockPath` body uses `_, hash`; `Acquire`
      stores `repo: canonical`.
- [ ] `lockPath` signature UNCHANGED `(string, error)`; `Acquire`'s `repoPath` param name unchanged.
- [ ] `TestAcquire_RepoFieldIsCanonical`: Acquire via symlink → repo= == canonical, != symlink (read before Release).
- [ ] `TestAcquireRelease_RoundTrip` + `TestSetSnapshot_UpdatesFile`: `c.Repo == canonical` (macOS-safe).
- [ ] The contention message auto-shows the canonical path (handleLockContention unchanged).

### Code Quality Validation

- [ ] lockHash is the single canonicalization source (DRY); no duplicated EvalSymlinks.
- [ ] lockPath's signature preserved (no unnecessary ripple to its tests).
- [ ] Mirrors existing lock_test.go conventions (white-box, resetCurrent, XDG isolation, read-before-Release).
- [ ] No scope creep into Release/setSnapshot/handleLockContention/platform files/docs/callers.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] No docs edits (DOCS: none — diagnostic-only; P1.M3 owns the doc sweep).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ Don't change `lockPath`'s SIGNATURE. Only its body (`_, hash := lockHash(repoPath)`). Two tests
  (`TestAcquire_PathMatchesLockPath`, `TestLockPath_CanonicalSymlink`) depend on `(string, error)`.
- ❌ Don't add a second `filepath.EvalSymlinks` in `Acquire` or a sibling `canonicalRepo` helper. Reuse
  `lockHash`'s canonical return — it's the single source (DRY; the hash and repo= can never diverge).
- ❌ Don't leave `TestAcquireRelease_RoundTrip` / `TestSetSnapshot_UpdatesFile` asserting `c.Repo != repo`
  (raw). After the fix `c.Repo` is canonical → the assertion FAILS on macOS (`/var → /private/var`). Use
  `canonical, _ := lockHash(repo)` as the oracle. This edit is REQUIRED, not optional.
- ❌ Don't read the lock file AFTER `Release()` in the new test. Issue 2 (P1.M2.T1.S1, landed) `os.Remove`s
  the file on Release → `os.ReadFile` would hit `os.IsNotExist`. Read BEFORE Release.
- ❌ Don't change the hash/filename input. The canonical is the SAME input the hash already used → the
  `<hash>.lock` filename is unchanged. This task is `repo=`-field-only; lock identity must not shift.
- ❌ Don't edit `default_action.go` / `generate` / `decompose`. `handleLockContention` (default_action.go:254)
  formats `Contents.Repo` verbatim → the message auto-fixes. The change is internal to `internal/lock/`.
- ❌ Don't edit `lock_unix.go` / `lock_windows.go` — the fix is in shared `lock.go`; flock semantics unchanged.
- ❌ Don't rename `Acquire`'s `repoPath` parameter (callers pass `os.Getwd()`; the name is fine). Only the
  struct-literal `repo:` value changes.
- ❌ Don't add a new import or dependency — `"path/filepath"` + `"crypto/sha256"` are already imported.
- ❌ Don't call `t.Parallel()` in the new test — the `current` singleton is process-global; existing lock
  tests are sequential and use `resetCurrent(t)`.
- ❌ Don't skip `go test -race ./internal/lock/` — it confirms the new signature is consumed everywhere, the
  macOS-symlink oracle is correct, and the read-before-Release ordering holds.

---

## Confidence Score

**9/10** — a tightly-scoped one-function-signature refactor + one-field assignment with the full test
ripple enumerated (including the non-obvious macOS `t.TempDir()` symlink break in two existing assertions),
the Issue-2 composition constraint (read-before-Release) called out, and the auto-fixing consumer
(default_action.go needs no edit) proven. Every edit site is pinned to file:line with concrete before/after
code. The -1 reserves for the implementer's choice between double-calling lockHash (lower-risk, chosen) vs
inlining lockPath to compute canonical literally once (the item's "once" ideal) — both correct.
