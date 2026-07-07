---
name: "P1.M2.T4.S1 (bugfix Issue 4b) — Guard handleLockContention empty diagnostic fields + partial-read unit test"
description: |

  Bugfix for Issue 4b (Minor — contender-side defense-in-depth for the truncated/partial lock-file read):
  `handleLockContention` (`internal/cmd/default_action.go`) builds its Busy (exit 5) message via a single
  `fmt.Fprintf` substituting `heldErr.Contents.Repo`, `.Pid`, `.Hostname`, `.Path`. If a contender read a
  partial/empty lock file (the residual race window that Issue 4a / P1.M2.T3.S1 narrows but cannot fully
  eliminate — `flock`'s advisory semantics + a contender's separate `os.ReadFile` open/read/close), those
  three diagnostic fields are empty strings and the message renders as
  `"stagecoach: another stagecoach run is already in progress on  (pid  on ). …"` — ugly, uninformative, with
  double-space artifacts. Fix: BEFORE the `fmt.Fprintf`, substitute sensible fallbacks — `Repo==""` →
  `"an unknown repo"`, `Pid==""` → `"<unknown>"`, `Hostname==""` → `"<unknown>"`. `Path` is passed through
  unchanged (it is the lock file path from `lockPath`, always non-empty). The message tone, the no-op fast
  path, the exit code (`exitcode.New(exitcode.Busy, nil)` → 5, SILENT), and the function signature are all
  PRESERVED.

  ⚠️ **THE central design call — defense-in-depth, NOT a rename or rewrite.** Issue 4a (P1.M2.T3.S1, running
  IN PARALLEL) fixes the ROOT CAUSE in `internal/lock/lock.go` (the holder-side `setSnapshot` rewrite becomes
  Seek→Write→Truncate→Sync so the file is never empty mid-rewrite). This subtask (4b) is the CONTENDER-SIDE
  guard in `internal/cmd/default_action.go` — a DIFFERENT file, so the two land with NO merge conflict. 4a
  narrows the empty-file window to ~zero; 4b guarantees that even the residual/nondeterministic window (or a
  genuinely malformed file from any cause) cannot render gibberish. Both ship. Neither depends on the other at
  the code level. See research §6.

  ⚠️ **THE scope fences — what MUST NOT change.** (1) Exit code: still `exitcode.New(exitcode.Busy, nil)`
  (exit 5, SILENT — `err.Error()==""`, same as every other Busy test asserts). (2) The no-op fast path
  (`if snap := heldErr.Contents.Snapshot; snap != "" { … }`) is UNTOUCHED. (3) Message tone/wording: identical;
  only the 3 diagnostic substitutions gain fallback values; `Path` is unchanged. (4) `handleLockContention`'s
  signature is unchanged. (5) `internal/lock/lock.go` is NOT touched (4a owns it). This subtask edits ONLY
  `internal/cmd/default_action.go` (the guard) + `internal/cmd/lock_contention_test.go` (one new test).

  ⚠️ **THE test decision — table-driven, two rows covering both Busy-entry paths, plus the exact
  broken-pattern check.** The new `TestHandleLockContention_Busy_EmptyDiagnostics` constructs a
  `*lock.HeldError` with EMPTY Repo/Pid/Hostname and exercises BOTH ways to reach the Busy branch: (a)
  `Snapshot==""` → fast path skipped; (b) `Snapshot="abc123"` + contender `WriteTree="zzz999"` → fast path
  fails → fall through (the contract's "non-matching snapshot"). Each row asserts exit 5, SILENT, the
  fallback text present (`"an unknown repo"` + `"<unknown>"`), and the broken pattern ABSENT
  (`!strings.Contains(msg, "on  (")` — the contract's exact check — plus `!strings.Contains(msg, "  ")` as a
  robust no-double-space guard). Reuses the EXISTING `contentionFakeGit` test double; NO new imports.

  Builds on the ALREADY-LANDED Issue 1/2/3 work present in the working tree. It does NOT touch
  `internal/lock/*`, `internal/exitcode/*`, platform files, docs, go.mod, or any caller.

  Deliverable: an edit to `internal/cmd/default_action.go` (the fallback block before the Busy `fmt.Fprintf`)
  + an addition to `internal/cmd/lock_contention_test.go` (one new table-driven test). NO new files, NO new
  imports, NO go.mod change, NO docs, NO signature change. OUTPUT: the Busy message is never gibberish on a
  partial/empty lock read; verified by `go test ./internal/cmd/ -run TestHandleLockContention`.

---

## Goal

**Feature Goal**: Make `handleLockContention`'s Busy (exit 5) message robust to a contender that read a
partial/empty lock file (the residual race window from Issue 4): substitute sensible fallbacks
(`"an unknown repo"` / `"<unknown>"` / `"<unknown>"`) for empty `Repo`/`Pid`/`Hostname` diagnostics BEFORE
formatting, so the message never renders as `"on  (pid  on )"`. Defense-in-depth alongside Issue 4a's
holder-side never-empty rewrite.

**Deliverable** (edits to two existing files):
1. **`internal/cmd/default_action.go`** — insert a 3-field fallback block between the no-op fast-path `if`
   and the Busy `fmt.Fprintf`; substitute the fallbacks into the `fmt.Fprintf` in place of the raw
   `heldErr.Contents.*` values. `Path` unchanged.
2. **`internal/cmd/lock_contention_test.go`** — ADD `TestHandleLockContention_Busy_EmptyDiagnostics`
   (table-driven, 2 rows: empty-snapshot + non-matching-snapshot, both with empty diagnostics).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/cmd/` clean;
`go test -race ./internal/cmd/ -run TestHandleLockContention` green — the new test passes AND every existing
`TestHandleLockContention_*` / `TestRunDefault_*` test stays green. `go test -race ./...` green (no caller
regresses; the guard is message-only). go.mod/go.sum unchanged; `handleLockContention` signature unchanged;
no file outside `internal/cmd/default_action.go` + `internal/cmd/lock_contention_test.go` touched.

## User Persona

**Target User**: A developer who accidentally double-runs `stagecoach` (or runs it in two terminals on the
same repo) AND whose contender process loses the flock race during the (now ~zero-width, but residual)
empty-file window. Transitively PRD §18.5 "Mechanism" / "Contention behavior" (the contender reads the
holder's `snapshot=` + `pid`/`hostname`/`repo` for the Busy message).

**Use Case**: Contender B's `lock.Acquire` loses the flock race → reads the holder's lock file →
`parseContents` yields empty diagnostics (partial read) → `handleLockContention` formats the Busy message.
B must see a sensible message, not `"on  (pid  on )"`.

**User Journey**: B's `Acquire` EWOULDBLOCK → `os.ReadFile` in the residual window → empty `Repo/Pid/Hostname`
→ `handleLockContention` substitutes fallbacks → B sees `"…in progress on an unknown repo (pid <unknown> on
<unknown>). …"` → B knows another run holds the lock and re-runs later.

**Pain Points Addressed**: removes the ugly, uninformative, double-space-artifact Busy message caused by a
partial/empty lock read. Diagnostic/UX correctness only; the conservative functional behavior (empty
snapshot → skip no-op → Busy, exit 5) is unchanged.

## Why

- **Completes the Issue-4 fix as defense-in-depth.** 4a (P1.M2.T3.S1) fixes the root cause (never-empty
  rewrite); 4b (this subtask) makes the contender's message rendering robust regardless of file state. The
  issue_analysis.md and the work-item contract both call for both. Neither is redundant.
- **No concurrency-safety impact.** The guard is message-only — the exit code (Busy=5), the no-op fast path,
  and the silent-return contract are all preserved. Pure UX/diagnostic polish.
- **Minimal, internal, no surface change.** No public signature change, no new deps, no docs, no caller
  edits, no platform-file edits. DOCS: none (P1.M3 owns the doc sweep).
- **Parallel-safe.** 4a edits `internal/lock/lock.go`; 4b edits `internal/cmd/default_action.go` + its test.
  Different files → no merge conflict; the two land independently and compose.

## What

A 3-field fallback block inserted before the Busy `fmt.Fprintf` in `handleLockContention`; one table-driven
test added. No public API change, no exit-code change, no fast-path change, no lock.go change.

### Success Criteria

- [ ] `handleLockContention` computes `repo`/`pid`/`hostname` locals BEFORE the Busy `fmt.Fprintf`, each
      falling back when the corresponding `heldErr.Contents.*` field is `""`: Repo → `"an unknown repo"`,
      Pid → `"<unknown>"`, Hostname → `"<unknown>"`. `Path` is passed through unchanged (always non-empty).
- [ ] The Busy `fmt.Fprintf` substitutes the locals (`repo`, `pid`, `hostname`) — NOT the raw
      `heldErr.Contents.*` values — for the first 3 `%s`; the 4th `%s` stays `heldErr.Path`.
- [ ] The no-op fast path (`if snap := heldErr.Contents.Snapshot; snap != "" { … }`) is byte-unchanged.
- [ ] The exit code is still `exitcode.New(exitcode.Busy, nil)` (exit 5, SILENT); `handleLockContention`'s
      signature is unchanged.
- [ ] `TestHandleLockContention_Busy_EmptyDiagnostics` added: table-driven, 2 rows (empty-snapshot +
      non-matching-snapshot), both with empty Repo/Pid/Hostname; each asserts exit 5, SILENT, the fallback
      text present (`"an unknown repo"` + `"<unknown>"`), `!strings.Contains(msg, "on  (")`, and
      `!strings.Contains(msg, "  ")`.
- [ ] Existing tests unchanged + green: `TestHandleLockContention_NoOpFastPath`, `_Busy_TreeDiffers`,
      `_Busy_EmptySnapshot`, `_Busy_WriteTreeErr`, `_SilentExits`, `TestRunDefault_LockReleasedAfterRun`.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/cmd/`, `go test -race ./internal/cmd/`,
      `go test -race ./...` clean/green; go.mod/go.sum byte-unchanged; only `default_action.go` +
      `lock_contention_test.go` touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact current
`handleLockContention` code (quoted in research §1), the exact fallback values + the insert site (research
§3), the scope fences (research §4 — what NOT to change), and the test design with a concrete code sketch
(research §5 — table rows, assertions, the `contentionFakeGit` reuse, no new imports). No PRD/git/provider
knowledge required — this is a 3-field guard + one test.

### Documentation & References

```yaml
# MUST READ — the authoritative research (the exact edit + the test sketch)
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M2T4S1/research/handlelockcontention-empty-field-guard.md
  why: the exact current code (§1), the bug's double-space rendering (§2), the exact guarded edit (§3), the
       scope fences (§4), the test design with both Busy-entry rows + the broken-pattern checks (§5), and why
       4a+4b compose as defense-in-depth (§6).
  critical: §3 (the exact fallback values + insert site) and §5 (the test must check BOTH `!Contains(msg,
       "on  (")` AND the fallback substrings) are the things most likely to be implemented wrong.

# The bug report + root cause + the chosen two-part fix (4a holder-side + 4b contender-side)
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/issue_analysis.md
  section: "Issue 4 (Minor) — SetSnapshot rewrite observable mid-write (truncated/partial read)" → "Guard
       handleLockContention: if repo/pid/hostname are empty, substitute sensible fallbacks".
  why: confirms the partial-read root cause (separate os.ReadFile in Acquire's EWOULDBLOCK branch → empty
       fields → gibberish message) and that 4b is the contender-side guard (this subtask), distinct from 4a.
  critical: the guard is message-only — do NOT change the exit code or the fast path.

# The system context (the control-flow spine + the data structures)
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/system_context.md
  section: "handleLockContention (cmd/default_action.go:241)" + "Key data structures" (LockContents / HeldError).
  why: confirms the Busy branch is reached via (a) empty snapshot → skip, OR (b) non-matching tree → fall
       through; and that `Contents.{Repo,Pid,Hostname}` are the 3 diagnostic strings (Path is separate,
       always non-empty). Informs the test's two rows.
  critical: Path comes from lockPath (always non-empty) — do NOT guard it.

# The file being fixed
- file: internal/cmd/default_action.go
  section: handleLockContention (L~241-256) — the no-op fast-path `if` + the Busy `fmt.Fprintf`.
  why: the single function you edit. Insert the 3-field fallback block between the fast-path `if` and the
       Busy `fmt.Fprintf`; substitute the locals into the `fmt.Fprintf`.
  pattern: locals-with-empty-guard: `repo := heldErr.Contents.Repo; if repo == "" { repo = "an unknown repo" }`
       (repeat for pid/hostname); then `fmt.Fprintf(stderr, "...on %s (pid %s on %s)...Lock: %s.\n", repo, pid,
       hostname, heldErr.Path)`.
  gotcha: keep the fast-path `if` block byte-unchanged; keep `return exitcode.New(exitcode.Busy, nil)`; keep
       the message wording identical (only the 3 substitutions change source); pass `heldErr.Path` through
       unchanged (it is always non-empty).

# The tests (the pattern for the new one + the tests that must stay green)
- file: internal/cmd/lock_contention_test.go
  section: TestHandleLockContention_Busy_TreeDiffers (the closest analog — Busy exit + message-substring
           assertions), TestHandleLockContention_Busy_EmptySnapshot (Snapshot="" → skip fast path → Busy),
           TestHandleLockContention_SilentExits (the err.Error()=="" silent contract), contentionFakeGit
           (the test double — embeds git.Git, overrides only WriteTree).
  why: the new test mirrors TestHandleLockContention_Busy_TreeDiffers's shape (held := &lock.HeldError{…};
       g := &contentionFakeGit{…}; var buf bytes.Buffer; err := handleLockContention(&buf, held, g, ctx);
       assert exitcode.For(err) + err.Error() + buf.String()). Confirms the silent-exit contract + the
       contentionFakeGit reuse.
  pattern: `package cmd` (white-box); construct `&lock.HeldError{Contents: lock.LockContents{…}, Path: …}`;
       assert `exitcode.For(err) == exitcode.Busy`, `err.Error() == ""`, and `strings.Contains(buf.String(),
       …)` / `!strings.Contains(buf.String(), …)`.
  gotcha: NO new imports — `bytes`, `context`, `strings`, `exitcode`, `lock` are all already imported. For the
       empty-snapshot row the fake's WriteTree is never called (fast path skipped) → a zero-value
       `&contentionFakeGit{}` suffices; for the non-matching row set `writeTreeSHA: "zzz999"`.

# The contender read path (why the window bites — NO edit, just context)
- file: internal/lock/lock.go
  section: Acquire's EWOULDBLOCK branch (`data, _ := os.ReadFile(path)` → parseContents → *HeldError).
  why: proves the contender does a SEPARATE open/read/close (different fd) — so even after 4a's never-empty
       rewrite, a contender can still (non-deterministically) read a partial file. This is WHY 4b exists.
  critical: do NOT edit lock.go — 4a (P1.M2.T3.S1) owns it; this subtask is the contender-side guard only.

# The data structures (field names — confirms the 3 diagnostic strings + Path)
- file: internal/lock/lock.go
  section: `type LockContents struct { Pid, Hostname, Repo, Timestamp, Snapshot string }` +
           `type HeldError struct { Contents LockContents; Path string }`.
  why: the exact fields the guard reads. Repo/Pid/Hostname are the 3 to guard; Path is separate (always
       non-empty); Timestamp/Snapshot are not in the Busy message.

# The parallel subtask (4a — the holder-side fix; DIFFERENT file, no conflict)
- file: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M2T3S1/PRP.md
  why: 4a restructures setSnapshot/writeContents in internal/lock/lock.go (Seek→Write→Truncate→Sync). It does
       NOT touch default_action.go. 4b (this subtask) is the contender-side guard in default_action.go. The
       two compose as defense-in-depth and land with no merge conflict (different files).
  critical: treat 4a's PRP as context only — do NOT edit lock.go here, and do NOT duplicate 4a's work.

# The prerequisites (LANDED — composition context)
- file: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M2T1S1/PRP.md   (Issue 2 — Release removes the file)
- file: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M2T2S1/PRP.md   (Issue 3 — canonical repo= field)
  why: BOTH have landed in the working tree. They do not affect handleLockContention's signature or the Busy
       message format. (Issue 3 makes the Repo field canonical when present — but a partial read still yields
       "" and hits this guard; the canonicalization and this guard are orthogonal.)
  critical: the default_action.go you edit ALREADY has the Issue-1 e2e wiring — do not revert it.
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  default_action.go          # handleLockContention (L~241-256) — EDIT (insert 3-field fallback block before
                             #   the Busy fmt.Fprintf; substitute locals into the Fprintf). runDefault /
                             #   shouldDecompose / runDecompose / handleGenError / handleDecomposeError UNTOUCHED.
  lock_contention_test.go    # ADD TestHandleLockContention_Busy_EmptyDiagnostics. contentionFakeGit + the 5
                             #   existing tests UNCHANGED. NO new imports.
  (root.go, …)               # UNTOUCHED
internal/lock/lock.go        # NOT TOUCHED (4a / P1.M2.T3.S1 owns it, in parallel). HeldError + LockContents
                             #   struct definitions are READ-ONLY context here.
internal/exitcode/exitcode.go # NOT TOUCHED (Busy=5; exitcode.New/For are read-only context).
go.mod / go.sum              # unchanged (stdlib only; fmt/io already imported in default_action.go)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Edits to internal/cmd/default_action.go (the guard) +
# internal/cmd/lock_contention_test.go (1 new table-driven test).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (scope fence — message-only): the guard MUST NOT change the exit code (still
// exitcode.New(exitcode.Busy, nil) → 5, SILENT), the no-op fast path, the message wording, or the function
// signature. It only substitutes fallback strings for empty Repo/Pid/Hostname. Path is passed through
// unchanged (always non-empty — it is the lock file path from lockPath).

// CRITICAL (scope fence — do NOT touch lock.go): internal/lock/lock.go is 4a's (P1.M2.T3.S1) scope, running
// in parallel. This subtask edits ONLY internal/cmd/default_action.go + internal/cmd/lock_contention_test.go.
// Editing lock.go would collide with 4a's merge.

// GOTCHA: keep the Busy fmt.Fprintf wording BYTE-IDENTICAL — only the 3 diagnostic %s substitutions change
// their SOURCE (from heldErr.Contents.Repo/.Pid/.Hostname to the repo/pid/hostname locals). The 4th %s stays
// heldErr.Path. Do NOT reword the message (the existing TestHandleLockContention_Busy_TreeDiffers asserts
// substrings like "4242" and "testhost" appear — rewording could break it; the guard preserves them when
// the fields are non-empty).

// GOTCHA: compute the fallbacks as LOCALS (repo/pid/hostname) and substitute those into the Fprintf. Do NOT
// mutate heldErr.Contents (it is a value field on the pointer receiver, but mutating it is unnecessary and
// surprising — locals are cleaner and leave heldErr untouched for any future caller inspection).

// GOTCHA: the empty-snapshot test row (Snapshot="") skips the fast path entirely → contentionFakeGit's
// WriteTree is NEVER called → a zero-value &contentionFakeGit{} is fine (its nil embedded git.Git is never
// dereferenced). The non-matching-snapshot row MUST set writeTreeSHA to something != the snapshot so the
// fast path fails and falls through to Busy.

// GOTCHA: NO new imports in lock_contention_test.go — bytes/context/strings/testing/exitcode/git/lock are
// all already imported. Adding an unused import fails `go vet`.

// GOTCHA: the broken-pattern check is `!strings.Contains(msg, "on  (")` (two spaces + open paren — the
// contract's exact check). Also assert `!strings.Contains(msg, "  ")` (no double space anywhere) as a robust
// guard — the guarded message contains no legit double space, so this is safe and stronger.

// GOTCHA: assert err.Error()=="" for the Busy rows (the SILENT contract — the message is already printed to
// stderr; main must not double-print). Every existing Busy test asserts this; the new test must too.
```

## Implementation Blueprint

### Data models and structure

No new types. No struct changes. Three local string variables (`repo`/`pid`/`hostname`) computed with empty
guards before the Busy `fmt.Fprintf`.

```go
// internal/cmd/default_action.go — handleLockContention, the guarded Busy branch:
// 	// Issue 4b guard: a contender may read a partial/empty lock file (the residual race window from Issue
// 	// 4a's SetSnapshot rewrite) yielding empty Repo/Pid/Hostname diagnostics. Substitute sensible fallbacks
// 	// so the Busy message never renders as "on  (pid  on )". Path is always non-empty (it is the lock file
// 	// path from lockPath), so it is passed through unchanged.
// 	repo := heldErr.Contents.Repo
// 	if repo == "" {
// 		repo = "an unknown repo"
// 	}
// 	pid := heldErr.Contents.Pid
// 	if pid == "" {
// 		pid = "<unknown>"
// 	}
// 	hostname := heldErr.Contents.Hostname
// 	if hostname == "" {
// 		hostname = "<unknown>"
// 	}
// 	fmt.Fprintf(stderr,
// 		"stagecoach: another stagecoach run is already in progress on %s (pid %s on %s). "+
// 			"Your newly-staged changes will remain staged — re-run stagecoach after it finishes. Lock: %s.\n",
// 		repo, pid, hostname, heldErr.Path)
// 	return exitcode.New(exitcode.Busy, nil) // exit 5, SILENT
```

> **gofmt note:** run `gofmt -w internal/cmd/default_action.go internal/cmd/lock_contention_test.go`. Add a
> brief comment block citing Issue 4b + the defense-in-depth relationship to 4a (see the sketch above).
>
> **Imports:** UNCHANGED in both files. `default_action.go` already imports `fmt` + `io`; the test file
> already imports `bytes`/`context`/`strings`/`testing`/`exitcode`/`git`/`lock`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: default_action.go — insert the 3-field fallback block + substitute locals into the Busy fmt.Fprintf
  - LOCATE handleLockContention (L~241-256). Find the Busy fmt.Fprintf that follows the no-op fast-path `if`.
  - INSERT (between the fast-path `if` block and the fmt.Fprintf) the fallback block from the Data Models
      sketch: compute `repo`/`pid`/`hostname` locals, each with an empty-string guard → fallback value.
  - EDIT the fmt.Fprintf's argument list: replace `heldErr.Contents.Repo, heldErr.Contents.Pid,
      heldErr.Contents.Hostname, heldErr.Path` with `repo, pid, hostname, heldErr.Path` (Path unchanged).
  - PRESERVE: the fast-path `if` block (byte-unchanged), the message wording (byte-identical), the
      `return exitcode.New(exitcode.Busy, nil)`, and the function signature.
  - GOTCHA: do NOT mutate heldErr.Contents (use locals). Do NOT reword the message. Do NOT guard Path.

Task 2: lock_contention_test.go — ADD TestHandleLockContention_Busy_EmptyDiagnostics
  - ADD the test per the sketch below: table-driven, 2 rows (empty-snapshot + non-matching-snapshot), both
      with Repo/Pid/Hostname = "". Each row asserts exit 5, SILENT, fallback substrings present, broken
      pattern absent.
  - REUSE contentionFakeGit (same package). empty-snapshot row: &contentionFakeGit{} (WriteTree uncalled).
      non-matching row: &contentionFakeGit{writeTreeSHA: "zzz999"} with Snapshot: "abc123".
  - PATTERN: mirror TestHandleLockContention_Busy_TreeDiffers (held := &lock.HeldError{…}; var buf
      bytes.Buffer; err := handleLockContention(&buf, held, g, context.Background()); assert exitcode.For +
      err.Error() + buf.String()).
  - GOTCHA: NO new imports. NO t.Parallel needed (these are pure unit tests with no global singleton). Assert
      err.Error()=="" (SILENT).

Task 3: VERIFY (no further edits)
  - RUN `gofmt -w internal/cmd/default_action.go internal/cmd/lock_contention_test.go`;
      `go vet ./internal/cmd/`; `go build ./...`;
      `go test -race ./internal/cmd/ -run TestHandleLockContention -v`;
      `go test -race ./...` (no caller regresses).
  - go.mod/go.sum byte-unchanged. internal/lock/* + internal/exitcode/* + platform files + docs byte-unchanged.
      Confirm handleLockContention's signature is unchanged.
```

### Test Specs (lock_contention_test.go — 1 new test)

```go
// ---------------------------------------------------------------------------
// TestHandleLockContention_Busy_EmptyDiagnostics — Issue 4b: a contender that read a partial/empty lock
// file (the residual race window) has empty Repo/Pid/Hostname. The guard must substitute sensible
// fallbacks so the Busy message never renders as "on  (pid  on )". Covers BOTH ways to reach the Busy
// branch with empty diagnostics: (a) empty snapshot → fast path skipped; (b) non-matching snapshot → fast
// path fails → fall through. Exit code stays Busy(5), SILENT.
// ---------------------------------------------------------------------------
func TestHandleLockContention_Busy_EmptyDiagnostics(t *testing.T) {
	cases := []struct {
		name       string
		snapshot   string
		writeTree  string // contender's WriteTree result (only consulted when snapshot != "")
	}{
		{"empty_snapshot", "", ""},              // fast path SKIPPED (snapshot empty) → Busy; WriteTree uncalled
		{"nonmatching_snapshot", "abc123", "zzz999"}, // fast path FAILS (trees differ) → fall through → Busy
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			held := &lock.HeldError{
				Contents: lock.LockContents{
					Pid:      "", // empty — the partial-read reproduction
					Hostname: "",
					Repo:     "",
					Snapshot: tc.snapshot,
				},
				Path: "/x.lock", // always non-empty (lock file path) — passed through unchanged
			}
			g := &contentionFakeGit{writeTreeSHA: tc.writeTree}

			var buf bytes.Buffer
			err := handleLockContention(&buf, held, g, context.Background())

			if code := exitcode.For(err); code != exitcode.Busy {
				t.Errorf("exitcode.For(err) = %d, want %d (Busy)", code, exitcode.Busy)
			}
			if err.Error() != "" {
				t.Errorf("err.Error() = %q, want empty (silent)", err.Error())
			}
			msg := buf.String()
			// The fallbacks are present:
			if !strings.Contains(msg, "an unknown repo") {
				t.Errorf("stderr = %q, want to contain repo fallback 'an unknown repo'", msg)
			}
			if !strings.Contains(msg, "<unknown>") {
				t.Errorf("stderr = %q, want to contain pid/hostname fallback '<unknown>'", msg)
			}
			// The lock Path is still reported (always non-empty):
			if !strings.Contains(msg, "/x.lock") {
				t.Errorf("stderr = %q, want to contain the lock path '/x.lock'", msg)
			}
			// The broken pattern is ABSENT (the contract's exact check + a robust no-double-space guard):
			if strings.Contains(msg, "on  (") {
				t.Errorf("stderr = %q, contains broken 'on  (' (double-space) pattern", msg)
			}
			if strings.Contains(msg, "  ") {
				t.Errorf("stderr = %q, contains a double-space (no legit double space exists in the message)", msg)
			}
		})
	}
}
```

> **Note on the `nonmatching_snapshot` row:** `contentionFakeGit{writeTreeSHA: "zzz999"}` returns
> `("zzz999", nil)` from WriteTree; the held `Snapshot="abc123"` differs → `werr == nil && contenderTree ==
> snap` is false → falls through to the Busy branch. The `empty_snapshot` row has `Snapshot=""` → the fast
> path `if` is skipped entirely → WriteTree is never called (the fake's zero-value `writeTreeSHA=""` is
> irrelevant). Both rows exercise the guarded Busy message with empty diagnostics.

### Implementation Patterns & Key Details

```go
// THE guard — locals with empty-string fallbacks, substituted into the unchanged fmt.Fprintf wording:
repo := heldErr.Contents.Repo
if repo == "" {
	repo = "an unknown repo"
}
pid := heldErr.Contents.Pid
if pid == "" {
	pid = "<unknown>"
}
hostname := heldErr.Contents.Hostname
if hostname == "" {
	hostname = "<unknown>"
}
fmt.Fprintf(stderr,
	"stagecoach: another stagecoach run is already in progress on %s (pid %s on %s). "+
		"Your newly-staged changes will remain staged — re-run stagecoach after it finishes. Lock: %s.\n",
	repo, pid, hostname, heldErr.Path) // ← 3 locals + Path (always non-empty)

// WHY locals (not mutating heldErr.Contents): cleaner, leaves heldErr untouched for caller inspection, and
// makes the guard a pure read+format step. Mutating the Contents would work but is surprising.

// WHY Path is not guarded: it is the lock file path from lockPath (a <sha256>.lock filename under the XDG
// lock dir) — always non-empty by construction. Guarding it would be dead code.

// WHY both 4a and 4b: 4a (P1.M2.T3.S1) narrows the empty-file window to ~zero at the holder side; 4b (here)
// makes the contender's message robust regardless. Defense-in-depth — neither redundant nor dependent.
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — stdlib only; fmt/io already imported in default_action.go. go mod tidy
      is a no-op.

PACKAGE EDGES: NONE added/removed. internal/cmd already imports internal/lock + internal/exitcode +
      internal/git (the guard reads heldErr.Contents.* — existing fields; no new type use).

FROZEN / NOT-EDITED:
  - internal/lock/lock.go (4a / P1.M2.T3.S1 owns it; in parallel) — HeldError + LockContents struct defs
        are READ-ONLY context.
  - internal/exitcode/exitcode.go (Busy=5; exitcode.New/For unchanged).
  - internal/lock/lock_unix.go + lock_windows.go (platform flock — untouched).
  - handleLockContention's SIGNATURE: `func handleLockContention(stderr io.Writer, heldErr *lock.HeldError,
        g git.Git, ctx context.Context) error` — unchanged.
  - The no-op fast-path `if` block — byte-unchanged.
  - All handleLockContention callers (runDefault), handleGenError/handleDecomposeError, docs/*, go.mod/go.sum.

DOWNSTREAM / RELATED (do NOT implement here):
  - P1.M2.T3.S1 (Issue 4a): the holder-side never-empty rewrite in internal/lock/lock.go. Different file.
  - P1.M3.T1 (doc sweep): the changeset-level doc sync. DOCS: none for this subtask.

NO DATABASE / ROUTES / CONFIG / CLI / NEW FILES / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/cmd/default_action.go internal/cmd/lock_contention_test.go
test -z "$(gofmt -l internal/cmd/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/cmd/     # catches an unused import / a signature drift / a malformed fmt.Fprintf.
go build ./...             # whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm handleLockContention's signature is UNCHANGED:
grep -n 'func handleLockContention' internal/cmd/default_action.go
#   expect: func handleLockContention(stderr io.Writer, heldErr *lock.HeldError, g git.Git, ctx context.Context) error
# Confirm the no-op fast-path `if` is intact + the exit code is still Busy:
grep -n 'exitcode.New(exitcode.Busy, nil)' internal/cmd/default_action.go
# Confirm lock.go + exitcode.go + platform files are UNCHANGED (4a owns lock.go; no collision):
git diff --exit-code internal/lock/ internal/exitcode/ && echo "lock + exitcode UNCHANGED (expected)"
```

### Level 2: Unit Tests (Component Validation)

```bash
go test -race ./internal/cmd/ -run TestHandleLockContention -v
# Expected PASS — verify explicitly:
#   TestHandleLockContention_Busy_EmptyDiagnostics (NEW, 2 rows): exit 5, SILENT, fallbacks present,
#       broken 'on  (' absent, no double space, Path reported.
#   TestHandleLockContention_NoOpFastPath: UNCHANGED, still green (snapshot matches → exit 0).
#   TestHandleLockContention_Busy_TreeDiffers: UNCHANGED, still green (non-empty diagnostics → message still
#       contains "4242" + "testhost" — the guard preserves non-empty values via the locals).
#   TestHandleLockContention_Busy_EmptySnapshot: UNCHANGED, still green (Snapshot="" → Busy).
#   TestHandleLockContention_Busy_WriteTreeErr: UNCHANGED, still green.
#   TestHandleLockContention_SilentExits: UNCHANGED, still green.
go test -race ./internal/cmd/     # the whole cmd package (incl. TestRunDefault_LockReleasedAfterRun).
go test -race ./...               # Full suite — NO regressions (the guard is message-only; no caller changed).
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only the two listed files changed:
git diff --name-only | grep -Ev 'internal/cmd/default_action\.go|internal/cmd/lock_contention_test\.go' \
  && echo "UNEXPECTED file changed" || echo "only default_action.go + lock_contention_test.go changed (good)"
# Confirm the guarded Busy branch reads sensibly on the empty-diagnostics case (eyeball the edit):
sed -n '/func handleLockContention/,/^}/p' internal/cmd/default_action.go
```

### Level 4: Creative & Domain-Specific Validation

```bash
# The partial-read race window itself is nondeterministic (4a narrows it to ~zero; 4b guards the message
# regardless). The unit test (Level 2) is the contract-specified proxy. For manual confidence, the e2e
# contention suite still passes (it asserts the Busy path with WELL-FORMED lock files, not the empty-window):
go test -tags e2e -run TestE2ELockContention ./... 2>/dev/null || echo "(e2e tag optional; the unit test is the gate)"
# golangci-lint (project-wide gate):
make lint 2>/dev/null || golangci-lint run ./internal/cmd/ 2>/dev/null || echo "(golangci-lint optional in dev)"
# Manual message-render check (defensive): simulate an empty-diagnostics HeldError by inspecting that the
# guarded fmt.Fprintf wording + fallbacks produce a clean string with no double spaces — covered by the test.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/cmd/`, `go mod tidy` no-op;
      `handleLockContention` signature unchanged; lock.go + exitcode.go + platform files byte-unchanged;
      go.mod/go.sum byte-unchanged.
- [ ] Level 2 green: `go test -race ./internal/cmd/ -run TestHandleLockContention` (new test + all existing)
      AND `go test -race ./...`.
- [ ] Level 3: binary builds; only `default_action.go` + `lock_contention_test.go` changed.

### Feature Validation

- [ ] `handleLockContention` substitutes `"an unknown repo"` / `"<unknown>"` / `"<unknown>"` for empty
      `Repo`/`Pid`/`Hostname` before the Busy `fmt.Fprintf`; `Path` passed through unchanged.
- [ ] The Busy `fmt.Fprintf` uses the `repo`/`pid`/`hostname` locals (NOT the raw `heldErr.Contents.*`).
- [ ] The no-op fast path is byte-unchanged; the exit code is still `exitcode.New(exitcode.Busy, nil)` (5, SILENT).
- [ ] `TestHandleLockContention_Busy_EmptyDiagnostics` (2 rows): exit 5, SILENT, fallbacks present,
      `!Contains(msg, "on  (")`, `!Contains(msg, "  ")`, Path reported.
- [ ] Existing `TestHandleLockContention_*` / `TestRunDefault_LockReleasedAfterRun` unchanged + green.

### Code Quality Validation

- [ ] The guard uses locals (does not mutate `heldErr.Contents`); the message wording is byte-identical.
- [ ] Mirrors existing lock_contention_test.go conventions (white-box, contentionFakeGit reuse, exitcode.For +
      err.Error() + buf.String() assertions, no new imports).
- [ ] No scope creep into lock.go / exitcode.go / the fast path / callers / platform files / docs.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] No docs edits (DOCS: none — internal message-quality fix; P1.M3 owns the doc sweep).
- [ ] go.mod/go.sum byte-unchanged; no new files; no public-signature change.

---

## Anti-Patterns to Avoid

- ❌ Don't change the exit code, the no-op fast path, the message wording, or `handleLockContention`'s
  signature. The guard is message-only — three diagnostic substitutions gain fallback values; everything
  else is byte-preserved.
- ❌ Don't guard `heldErr.Path`. It is the lock file path from `lockPath` (a `<sha256>.lock` filename) —
  always non-empty by construction. Guarding it is dead code.
- ❌ Don't mutate `heldErr.Contents`. Use locals (`repo`/`pid`/`hostname`) and substitute those into the
  `fmt.Fprintf`. Mutating the Contents works but is surprising and leaves the input altered for any caller.
- ❌ Don't touch `internal/lock/lock.go`. It is 4a's (P1.M2.T3.S1) scope, running in parallel — editing it
  would collide with 4a's merge. This subtask is the contender-side guard in `default_action.go` only.
- ❌ Don't add new imports. `default_action.go` already imports `fmt` + `io`; the test file already imports
  `bytes`/`context`/`strings`/`testing`/`exitcode`/`git`/`lock`. An unused import fails `go vet`.
- ❌ Don't drop the `err.Error()==""` (SILENT) assertion in the new test. The Busy exit is silent (the message
  is already on stderr; main must not double-print) — every existing Busy test asserts this.
- ❌ Don't replace the broken-pattern check with only a fallback-substring check. Assert BOTH that the
  fallbacks are present AND that the broken `on  (` / double-space pattern is ABSENT — the two together prove
  the guard works.
- ❌ Don't forget the `nonmatching_snapshot` row needs `writeTreeSHA` != the snapshot. With `Snapshot="abc123"`
  and the default zero-value `writeTreeSHA=""`, the fast path computes `contenderTree=""` ≠ `"abc123"` → it
  still falls through to Busy, but setting `writeTreeSHA: "zzz999"` makes the intent explicit and avoids
  relying on the empty-string accident.
- ❌ Don't reword the Busy message. `TestHandleLockContention_Busy_TreeDiffers` asserts `"4242"` + `"testhost"`
  appear; the guard preserves non-empty values via the locals, so those assertions still hold — but rewording
  the template risks breaking them. Keep the wording byte-identical.
- ❌ Don't call `t.Parallel()` unnecessarily. These are pure unit tests with no global singleton (unlike the
  lock package's `current` singleton) — parallelism is harmless but unneeded; match the existing
  `TestHandleLockContention_*` style (sequential).

---

## Confidence Score

**9/10** — a tightly-scoped, message-only guard (three empty-string fallbacks substituted into an unchanged
`fmt.Fprintf` wording) with the exit code, fast path, signature, and tone all explicitly preserved, plus a
table-driven test covering BOTH Busy-entry paths (empty snapshot → skip; non-matching snapshot → fall through)
with the exact contract-specified broken-pattern check (`!Contains(msg, "on  (")`) and a robust no-double-space
guard. The parallel subtask (4a) edits a different file (`internal/lock/lock.go`), so there is no merge
conflict and the two compose as defense-in-depth. The test reuses the existing `contentionFakeGit` double and
needs no new imports. The -1 reserves for the implementer's choice of assertion granularity (the contract
mandates `!Contains(msg, "on  (")`; the `!Contains(msg, "  ")` no-double-space guard is a robustness bonus).
