name: "P1.M3.T1.S1 — Export Status() read path + orphan detection helper (Unix + Windows) (FR-K4)"
description: >
  The READ-ONLY diagnostic API for FR-K4 (PRD §9.27): a `lock.Status(repoPath)` that returns the full
  parsed lock-file state — path, contents, holder liveness, AND a NEW orphan heuristic — WITHOUT ever
  acquiring the flock or breaking/removing a lock (FR52 preserved). This is purely ADDITIVE to the
  stdlib-only internal/lock package: (a) ONE new exported func `Status` in internal/lock/lock.go that
  composes THREE existing unexported helpers — `lockPath(repoPath)` (returns (string,ERROR) — see
  gotcha), `parseContents(data)` (returns LockContents), `processAlive(pid, hostname)` (returns bool) —
  plus a NEW fourth helper `appearsOrphaned(pid)`; (b) ONE new file internal/lock/orphan_unix.go
  (`//go:build !windows`) implementing `appearsOrphaned` via runtime.GOOS dispatch: Linux reads
  `/proc/<pid>/status`'s `PPid:` field (bufio.Scanner + strings.Fields), Darwin shells out to
  `ps -o ppid= -p <pid>` (os/exec + strconv.Atoi) — returns ppid==1, and returns FALSE (conservative:
  don't claim orphan) on ANY error (proc missing, ps non-zero exit, parse failure); (c) ONE new file
  internal/lock/orphan_windows.go (`//go:build windows`): `func appearsOrphaned(pid int) bool { return false }`
  (FR-K7 — Windows has no init-reparenting analog). The lock package STAYS stdlib-only (os, os/exec,
  bufio, strconv, strings, runtime, fmt are all stdlib). Consumed by P1.M3.T2.S1 (the `lock status`
  cobra subcommand) and P1.M3.T3.S1 (the Busy-message orphan hint). NO existing behavior changes —
  Acquire/Release/reapStaleLocks/processAlive are untouched; Status reads the SAME lockPath + file
  format + processAlive that Acquire/HeldError already use. Mock NOTHING — tests plant temp lock files
  and use known pids (self pid, dead pid) via the existing writeLockFile helper. The orphan==true path
  is proven by the E2E harness (P1.M4.T1.S1); unit tests pin the deterministic conservative-false
  branches (dead pid, malformed pid, no-lock) + self-not-orphan.

---

## Goal

**Feature Goal**: Give Stagecoach a read-only way to answer "what is the state of this repo's run
lock?" — path, parsed contents, holder liveness, and whether the holder appears orphaned
(reparented-to-init) — so that a blocked user (or the Busy message) can decide whether to `kill`/`rm`
WITHOUT acquiring or breaking the lock. This closes the §9.27 FR-K4 diagnostic gap: the lock package
had a *write* path (Acquire) and a *contention* path (HeldError) but NO exported read-only path that a
`stagecoach lock status` subcommand or the Busy-message orphan hint could call.

**Deliverable**: Three additions to `internal/lock` (stdlib-only):
1. `func Status(repoPath string) (path string, contents LockContents, alive bool, orphan bool, err error)` — exported, in `internal/lock/lock.go`.
2. `internal/lock/orphan_unix.go` (`//go:build !windows`) — `func appearsOrphaned(pid int) bool` with Linux `/proc` + Darwin `ps` branches via `runtime.GOOS`.
3. `internal/lock/orphan_windows.go` (`//go:build windows`) — `func appearsOrphaned(pid int) bool { return false }`.
Plus tests in `internal/lock/` (Unix-only `_test.go` for the `//go:build !windows` branches; cross-compile proves Windows).

**Success Definition**:
- `Status` never acquires the flock, never removes/breaks a lock (FR52 preserved) — verified by the
  fact it calls ONLY `lockPath`/`os.ReadFile`/`parseContents`/`strconv.Atoi`/`processAlive`/`appearsOrphaned`
  (no `flock`, no `os.Remove`, no mutation). Grep guard enforces this.
- `Status` returns `path==""`, zero contents, all-false, nil error when no lock file exists (the
  "no lock held" case) — verified by a unit test.
- `Status` returns the path + parsed contents + `alive` for a planted lock file holding our own pid;
  `orphan` is `appearsOrphaned(self)` (false in normal dev shells).
- `appearsOrphaned` is conservative: returns `false` for a dead/non-existent pid (proc missing / ps
  error → don't claim orphan) and for self (ppid != 1). The `ppid==1` (true) path is the E2E harness.
- `go build ./...` + `GOOS=windows` + `GOOS=linux` + `GOOS=darwin` all clean (the Windows stub and the
  Unix `os/exec` import both compile; the runtime.GOOS file compiles on every Unix target).
- `gofmt -l internal/lock/` empty; `go vet ./internal/lock/...` clean; `make lint` clean; `make test`
  (race) green; `internal/lock` tests green.
- [Mode A] Godoc on `Status` (read-only, never breaks) and on `appearsOrphaned` (the ppid==1 heuristic
  + its subreaper limitation) — PRD §9.27 FR-K4 + Mode-A doc convention.

## User Persona (if applicable)

**Target User**: A developer whose `stagecoach` invocation got the §18.5 Busy message ("another run
is in progress on <repo>"), or who is debugging a stuck lock — the §9.27 "the lock stays forever" case.

**Use Case**: The user wants to SEE the lock's state (path, holder pid/host, is it alive, is it
orphaned) before deciding to `kill <N>` or `rm <path>`. `stagecoach lock status` (P1.M3.T2.S1) prints
exactly this; the Busy message (P1.M3.T3.S1) adds the orphan hint inline. Both call `lock.Status`.

**User Journey**: user runs `stagecoach` → Busy message with a lock path → user runs `stagecoach lock
status` → sees "pid 1234 alive, appears orphaned (launcher exited)" → user `kill 1234` → next run
unblocked. Status never touched the lock itself; the user did, deliberately.

**Pain Points Addressed**: FR-K4 — the missing "show me the lock so I can decide" surface. Without it,
the only lock info is buried mid-sentence in the Busy message and there is no liveness/orphan read.

## Why

- **FR-K4 / §9.27**: the lock package's only exports today are `Acquire` (takes the lock),
  `HeldError` (returned on contention), `ReleaseCurrent`/`SetSnapshot` (holder-side), `IsHeldError`.
  There is NO exported read-only "what's the state of this repo's lock" entry point. `lock status`
  and the orphan hint cannot call `Acquire` (it would take the lock / block on contention). `Status`
  is that read-only path — it reuses the SAME `lockPath` + file format + `processAlive` Acquire already
  uses, so the two never disagree.
- **FR52 preserved by construction**: `Status` is read-only (no `flock`, no `os.Remove`). It changes
  nothing; the user decides whether to `kill`/`rm`. This is the explicit FR-K4 contract ("It changes
  nothing").
- **The orphan heuristic is NEW platform code**: detecting the HOLDER's parent (not our own `getppid`)
  is net-new. Linux: `/proc/<pid>/status` PPid. Darwin: `ps -o ppid=`. Windows: always false (FR-K7).
  `appearsOrphaned` is unexported and composed only by `Status` (when the holder is alive); the
  consumers (P1.M3.T2.S1 / P1.M3.T3.S1) reach it through `Status`'s `orphan` return.
- **Stdlib-only invariant held**: the new imports (`os/exec`, `runtime`, `bufio`, `strconv`, `strings`,
  `os`, `fmt`) are all stdlib — the package stays a self-contained leaf with no stagecoach imports and
  no golang.org/x/sys (per the `package lock` doc comment).

## What

**User-visible behavior**: None directly (this is a library API). Once P1.M3.T2.S1 lands,
`stagecoach lock status` prints the `Status` output; once P1.M3.T3.S1 lands, the Busy message uses the
`orphan` return. This item lands the API both consume.

**Technical change**:
- `Status(repoPath)` resolves the lock path via `lockPath(repoPath)` (handle its `(string,error)`
  return), `os.ReadFile`s it (file-not-exist ⇒ no-lock; other error ⇒ propagate), `parseContents` →
  `LockContents`, `strconv.Atoi(contents.Pid)` (malformed ⇒ return path+contents with alive/orphan
  false), `processAlive(pid, contents.Hostname)` ⇒ alive; if alive, `appearsOrphaned(pid)` ⇒ orphan.
- `appearsOrphaned` (Unix): `runtime.GOOS == "linux"` ⇒ parse `/proc/<pid>/status` `PPid:` line;
  else (darwin/BSDs) ⇒ `ps -o ppid= -p <pid>`. Return `ppid == 1`; on ANY error return `false`.
- `appearsOrphaned` (Windows): `return false`.

### Success Criteria
- [ ] `func Status(repoPath string) (path string, contents LockContents, alive bool, orphan bool, err error)`
      exists and is exported in `internal/lock/lock.go`, with a Mode-A godoc stating it is READ-ONLY
      (never acquires the flock, never breaks/removes a lock — FR52 preserved) and that `path==""`
      means no lock held.
- [ ] `Status` composes ONLY: `lockPath`, `os.ReadFile`, `parseContents`, `strconv.Atoi`,
      `processAlive`, `appearsOrphaned`. NO `flock`, NO `os.Remove`, NO `Acquire`, NO mutation.
      (grep-guarded.)
- [ ] `Status` returns `("", LockContents{}, false, false, nil)` when the lock file does not exist
      (`os.IsNotExist`); it propagates other `os.ReadFile` errors and the `lockPath` error via `err`.
- [ ] `internal/lock/orphan_unix.go` (`//go:build !windows`) implements
      `func appearsOrphaned(pid int) bool` with a Mode-A godoc explaining the ppid==1 ≈ reparented
      heuristic and its subreaper limitation (PR_SET_CHILD_SUBREAPER — ppid≠1 under systemd-run etc.).
- [ ] Linux branch reads `/proc/<pid>/status`, matches the `PPid:` token via `strings.Fields`, and
      returns `ppid==1`. Darwin branch runs `ps -o ppid= -p <pid>` via `os/exec`, `TrimSpace`+`Atoi`,
      returns `ppid==1`. BOTH return `false` on any error (proc missing / ps non-zero / parse fail).
- [ ] `internal/lock/orphan_windows.go` (`//go:build windows`) is `func appearsOrphaned(pid int) bool { return false }`.
- [ ] `go build ./...` + `GOOS=windows go build ./...` + `GOOS=linux go build ./...` + `GOOS=darwin go build ./...` clean.
- [ ] `gofmt -l internal/lock/` empty; `go vet ./internal/lock/...` clean; `make lint` clean.
- [ ] `go test ./internal/lock/ -race` green; `make test` (full race suite) green (regression: existing
      Acquire/Release/reaping/processAlive tests untouched).
- [ ] Grep guards (§Validation Level 4) all pass.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact signatures of all THREE existing helpers Status composes (including the
`(string,error)` return of `lockPath` that the item description glossed over), the exact `LockContents`
struct shape, the exact build-tag convention (`//go:build !windows` / `//go:build windows`, modern form,
blank-line-before-package), the stdlib-only invariant + why the single-file `runtime.GOOS` approach is
valid (every import referenced by ≥1 compiled function; Go errors on unused imports not unused funcs),
the parse details for `/proc/<pid>/status` (`PPid:` token, `strings.Fields`, ENOENT) and `ps -o ppid=`
(header suppression via trailing `=`, TrimSpace, non-zero exit on dead pid), the conservative-false
contract, the existing test helpers to reuse (`writeLockFile`, `t.Setenv("XDG_RUNTIME_DIR", …)`, the
`exec.Command("true")` dead-pid trick), the coverage-gate exemption for `internal/lock`, and 7 grep guards.

### Documentation & References

```yaml
# MUST READ — codebase-specific findings (exact signatures incl. lockPath's error return, build-tag form,
# the runtime.GOOS approach validity, test helpers to reuse, the conservative test net)
- docfile: plan/014_37208f58ffa2/P1M3T1S1/research/findings.md
  why: "§1 the EXACT signatures Status composes (lockPath returns (string,ERROR) — critical gotcha);
        §2 file-not-exist vs other-error; §3 malformed-pid handling; §4 build-tag form; §5 stdlib-only +
        runtime.GOOS validity; §6 test helpers to REUSE (writeLockFile, dead-pid trick); §7 the orphan==true
        case is E2E (P1.M4.T1.S1), unit tests pin conservative-false; §8 coverage gate exempts internal/lock;
        §9 consumer contracts (P1.M3.T2.S1/T3.S1 reach orphan via Status); §11 validation commands."
  critical: "lockPath returns (string, error) — Status MUST handle the error. parseContents never errors
             (silently skips malformed lines). processAlive is conservative (true on ambiguity)."

# MUST READ — the platform-specific parse details + copy-pasteable Go (Linux /proc + Darwin ps + conservative error handling)
- docfile: plan/014_37208f58ffa2/P1M3T1S1/research/external_ppid.md
  why: "§1 Linux /proc/<pid>/status PPid field (token 'PPid:' with colon, strings.Fields, ENOENT);
        §2 Darwin ps -o ppid= (trailing '=' suppresses header, TrimSpace, *exec.ExitError on dead pid);
        §3 the ppid==1 heuristic + the PR_SET_CHILD_SUBREAPER limitation (why conservative-false is correct);
        §4 //go:build syntax + runtime.GOOS; the copy-pasteable single-file Go snippet."
  critical: "Compare the WHOLE token fields[0]==\"PPid:\" (strings.Fields keeps the colon glued to the word) — NOT
             a bare prefix, to avoid matching the 'Pid:' line. On Darwin TrimSpace before Atoi (ps right-justifies).
             Return false on ANY error — this is the conservative contract."

# MUST READ — the authoritative lock-extension architecture (FR-K4 Status signature + orphan-detection spec)
- docfile: plan/014_37208f58ffa2/architecture/lock_extension.md
  why: "'FR-K4: Lock status read path (NEW exports)' gives the exact Status signature + the wrap order
        (lockPath→ReadFile→parseContents→processAlive→orphan); 'Orphan detection (NEW)' specifies the
        orphan_unix.go/orphan_windows.go split + the Linux/Darwin/Windows behavior; 'Lock file location'
        + 'Lock states' explain why path=='' means no lock."
  critical: "Status calls appearsOrphaned ONLY when the holder is alive (a dead pid's orphan status is
             irrelevant — processAlive already returned false). Confirms the three lock states and that
             Status must NEVER acquire or break a lock."

# MUST READ — the file being edited (the helpers Status composes + where to add Status)
- file: internal/lock/lock.go
  why: "lockPath (line 302, returns (string,error)); parseContents (line 259); LockContents struct (line 47);
        HeldError (line 54); the package doc (stdlib-only invariant, line 1). Status is added as a new
        exported func (place near ReleaseCurrent / IsHeldError — the other exported helpers)."
  pattern: "The exported helpers (ReleaseCurrent line 208, IsHeldError line 311) are each self-contained,
            godoc'd, stdlib-only. Follow that style for Status."
  gotcha: "lockPath returns (string, error) — the item description's 'lockPath(repoPath)' glossed over it.
           Status MUST do `path, err := lockPath(repoPath); if err != nil { return ... err }`."

# MUST READ — the existing Unix twin file (build-tag form + processAlive conservative style to mirror)
- file: internal/lock/lock_unix.go
  why: "Build tag `//go:build !windows` (line 1, NO legacy +build line). processAlive (line 35) is the
        conservative-liveness twin of appearsOrphaned — mirror its godoc tone (state the heuristic, the
        conservative-on-ambiguity invariant, the cross-platform twin reference)."
  pattern: "processAlive's godoc lists every branch + why each is conservative. appearsOrphaned's godoc
            should do the same (Linux/Darwin/Windows, ppid==1, false-on-error, subreaper caveat)."
  gotcha: "No legacy `// +build` line — Go 1.22 codebase uses modern form only."

# CONTEXT — the Windows twin (the always-true processAlive pattern → appearsOrphaned mirrors with always-false)
- file: internal/lock/lock_windows.go
  why: "Build tag `//go:build windows`. processAlive returns true (FR-K7). appearsOrphaned returns false
        on Windows — mirror this file's structure (one-line func + godoc citing FR-K7 + the cross-platform
        twin reference)."
  critical: "Windows has no init-reparenting analog AND no real flock — appearsOrphaned is always false there."

# CONTEXT — the test file with the helpers to REUSE (writeLockFile, dead-pid trick, resetCurrent)
- file: internal/lock/lock_unix_test.go
  why: "writeLockFile(t, path, pid, hostname) (line 60) writes a minimal key=value lock file in the EXACT
        format parseContents reads — REUSE it for Status tests. The exec.Command('true') dead-pid trick
        (line 38) gives a guaranteed-dead pid for the conservative-false orphan test. Build tag
        `//go:build !windows`."
  pattern: "The processAlive tests (TestProcessAlive_*) are the model for appearsOrphaned tests:
            self-alive/self-not-orphan + foreign/empty/conservative + dead-pid-false."

# CONTEXT — PRD §9.27 FR-K4 (the requirement this implements) + §18.5 (the lock model Status reads)
- docfile: plan/014_37208f58ffa2/prd_snapshot.md
  section: "§9.27 (FR-K4 lock status read-only diagnostic) and §18.5 (the per-repo run lock, FR52)"
  why: "FR-K4 specifies: 'prints, for the current repo's lock: the path; the holder's parsed
        pid/hostname/repo/timestamp/snapshot; whether the holder process is alive (kill(pid,0)); and — on
        Unix — whether it appears orphaned (its own parent pid is init/a subreaper). With no lock held it
        prints no run lock. It changes nothing.' §18.5 documents path=='' / the file format / FR52."
  critical: "FR-K4: Status 'changes nothing' (read-only). FR-K7: Windows reports liveness 'via the platform
             process check where available and unknown otherwise' — on Windows processAlive is always true,
             so alive=true there; the 'unknown' nuance is the consumer's (P1.M3.T2.S1) display concern,
             NOT this item's (Status returns the bool from processAlive as-is)."
```

### Current Codebase tree (relevant slice)

```bash
internal/lock/
  lock.go                 # EDIT — add exported func Status (near ReleaseCurrent/IsHeldError)
  lock_unix.go            # READ-ONLY — processAlive twin; build-tag form to mirror
  lock_windows.go         # READ-ONLY — processAlive twin (always-true); appearsOrphaned mirrors (always-false)
  lock_test.go            # READ-ONLY — resetCurrent, parseContents-usage patterns
  lock_unix_test.go       # EDIT — add appearsOrphaned + Status tests (reuse writeLockFile, dead-pid trick)
# (NO lock_windows_test.go today — optional; cross-compile is the Windows validation)
go.mod                    # READ-ONLY — module github.com/dustin/stagecoach, go 1.22 (NO new dep — all imports stdlib)
Makefile                  # test=line 70 (-race); lint=line 103; coverage-gate=line 77 (internal/lock NOT gated)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/lock/
  lock.go                 # MODIFIED — +func Status (exported, read-only)
  orphan_unix.go          # NEW (//go:build !windows) — func appearsOrphaned(pid int) bool (Linux /proc + Darwin ps)
  orphan_windows.go       # NEW (//go:build windows)  — func appearsOrphaned(pid int) bool { return false }
  lock_unix_test.go       # MODIFIED — +appearsOrphaned tests + +Status tests (Unix-only; //go:build !windows)
# (OPTIONAL: lock_windows_test.go — trivial !appearsOrphaned(1); add only if zero-effort. Cross-compile suffices.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (lockPath returns an ERROR — the item description glossed over it):
//   func lockPath(repoPath string) (string, error)   // lock.go:302
// lockDir() can fail (no XDG_RUNTIME_DIR/XDG_CACHE_HOME AND os.UserHomeDir fails — NO CWD fallback, the
// §18.5 anti-pattern). Status MUST do: `path, err := lockPath(repoPath); if err != nil { return "", LockContents{}, false, false, err }`.
// Tests isolate via t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) + t.Setenv("XDG_CACHE_HOME", "").

// CRITICAL (the "no lock" case is ONLY os.IsNotExist): os.ReadFile errors split into two kinds.
//   if errors.Is(err, fs.ErrNotExist) { return "", LockContents{}, false, false, nil }   // no lock held
//   return "", LockContents{}, false, false, err                                          // real error (perm, I/O)
// Use os.IsNotExist(err) OR errors.Is(err, fs.ErrNotExist) — both work; the codebase uses os.IsNotExist
// style implicitly (reapStaleLocks ignores all errors; Status must distinguish not-exist from others).

// CRITICAL (parseContents NEVER errors): parseContents(data) returns LockContents with empty fields for
// missing/malformed lines. An empty/garbage pid= line ⇒ c.Pid=="" or garbage ⇒ strconv.Atoi errors.
// Status returns the path + parsed contents (diagnostic value) with alive=false, orphan=false (can't
// assess liveness without a parseable pid). Mirrors reapStaleLocks's `continue` on Atoi error.

// CRITICAL (appearsOrphaned is called ONLY when alive): a dead pid's orphan status is irrelevant —
// processAlive already returned false, so Status returns orphan=false without calling appearsOrphaned.
// This is the architecture doc's explicit ordering: "Status calls appearsOrphaned only when the holder is alive."

// CRITICAL (compare the WHOLE "PPid:" token, not a prefix): strings.Fields splits on whitespace and keeps
// the colon glued to "PPid". Match `fields[0] == "PPid:"` — a bare `strings.HasPrefix(line, "PPid")` would
// ALSO match the "Pid:" line. (Pid comes BEFORE PPid in /proc/<pid>/status.)

// CRITICAL (Darwin ps output has leading whitespace): `ps -o ppid= -p <pid>` right-justifies the number,
// e.g. "     1". strconv.Atoi(strings.TrimSpace(string(out))) — TrimSpace is MANDATORY or Atoi fails.

// GOTCHA (stdlib-only invariant + the runtime.GOOS single-file approach is VALID): the lock package imports
// ONLY stdlib (package doc, lock.go:1). The new imports — os/exec (Darwin), runtime (GOOS switch), bufio,
// strconv, strings, os, fmt — are ALL stdlib. A SINGLE orphan_unix.go (//go:build !windows) with a
// runtime.GOOS switch compiles the WHOLE file on every Unix target, so every import is referenced by ≥1
// function (ppidLinux uses os/bufio/strconv/strings/fmt; ppidViaPs uses os/exec/strconv/strings). Go errors
// on unused IMPORTS, not unused package-level FUNCTIONS — so ppidViaPs existing-but-uncalled-on-Linux is fine.
// This is simpler than build-tag-per-OS (one file, not two) and matches the item description's
// "on Darwin (runtime.GOOS=='darwin')".

// GOTCHA (build-tag form — MODERN only, no legacy +build): the codebase uses `//go:build !windows` /
// `//go:build windows` with NO `// +build` line (lock_unix.go:1, lock_windows.go:1). Go 1.22 → modern form
// is canonical. The constraint line MUST be followed by a BLANK LINE before `package lock`.

// GOTCHA (orphan==true is hard to unit-test deterministically): creating a real reparented-to-init pid in a
// unit test is flaky/OS-dependent. The deterministic net: appearsOrphaned(deadPid)==false (conservative —
// /proc missing, ps non-zero exit), appearsOrphaned(selfPid)==false in normal dev shells (parent != 1;
// CI-under-init could differ — assert false with a comment, it's a heuristic). The orphan==true path is
// proven by the E2E harness (P1.M4.T1.S1). This mirrors the existing processAlive test philosophy
// (TestProcessAlive_* test the branches you can pin; the contention race is a separate test).

// GOTCHA (coverage gate does NOT include internal/lock): Makefile line 77 gates ONLY
// internal/{git,provider,generate,config}. No coverage threshold pressure on this item.
```

## Implementation Blueprint

### Data models and structure

None NEW. `Status` returns the EXISTING `LockContents` struct (lock.go:47: `Pid, Hostname, Repo,
Timestamp, Snapshot string`) plus three scalars. `appearsOrphaned` takes/returns primitives only.
No new types, no fields, no packages.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/lock/orphan_unix.go (//go:build !windows) — the Unix orphan heuristic
  - FILE HEADER: `//go:build !windows` then a BLANK LINE then `package lock` (mirror lock_unix.go:1).
  - IMPORTS (all stdlib): bufio, fmt, os, os/exec, runtime, strconv, strings.
  - IMPLEMENT:
      func appearsOrphaned(pid int) bool {
          ppid, err := ppidOf(pid)
          if err != nil { return false }          // conservative: don't claim orphan on ambiguity
          return ppid == 1
      }
    where ppidOf dispatches on runtime.GOOS:
      func ppidOf(pid int) (int, error) {
          if runtime.GOOS == "linux" { return ppidLinux(pid) }
          return ppidViaPs(pid)   // darwin + other BSDs
      }
    Linux branch (ppidLinux): os.Open("/proc/<pid>/status"); defer Close; bufio.Scanner; per line
      fields := strings.Fields(line); if len(fields) >= 2 && fields[0] == "PPid:" { return strconv.Atoi(fields[1]) };
      after loop: return scanner.Err() or fmt.Errorf("no PPid field for pid %d", pid).  (ENOENT on dead pid propagates as the Open error.)
    Darwin branch (ppidViaPs): out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output();
      if err != nil { return 0, err }; return strconv.Atoi(strings.TrimSpace(string(out))).
  - GODOC [Mode A] on appearsOrphaned: explain it is a HEURISTIC — ppid==1 ≈ the holder was reparented to
    init/launchd (its launcher exited without killing it — §9.27's orphaned-but-alive case). State the
    LIMITATION: under a subreaper (PR_SET_CHILD_SUBREAPER — systemd, systemd-run, some shells, containers)
    an orphan's ppid may be != 1, so this can MISS orphans (false negative); it never FALSE-POSITIVEs a
    legitimately-parented process in the common case. State the conservative contract: returns false on ANY
    error (proc gone, ps failure, parse failure) — orphan detection is a diagnostic HINT, never a destructive
    trigger. Reference FR-K4 + the cross-platform twin orphan_windows.go (always-false, FR-K7).
  - FOLLOW pattern: lock_unix.go processAlive godoc (lists every branch + conservative invariant + cross-platform twin).
  - NAMING: appearsOrphaned (unexported, per contract), ppidOf/ppidLinux/ppidViaPs (unexported helpers).
  - GOTCHA: match fields[0] == "PPid:" (WHOLE token, colon attached) — NOT HasPrefix, to avoid the "Pid:" line.
  - GOTCHA: TrimSpace before Atoi on Darwin (ps right-justifies).
  - GOTCHA: the whole file compiles on every Unix target; every import is used (ppidLinux↔bufio/fmt/os/strconv/strings; ppidViaPs↔os/exec/strconv/strings; runtime used by ppidOf).

Task 2: CREATE internal/lock/orphan_windows.go (//go:build windows) — the Windows no-op
  - FILE HEADER: `//go:build windows` then BLANK LINE then `package lock`.
  - IMPLEMENT: func appearsOrphaned(pid int) bool { return false }
  - GODOC [Mode A]: Windows has no init-reparenting analog AND flock is a no-op here (the §13.5 CAS is the
    guarantee — FR-K7), so orphan detection is always false. Cross-platform twin of orphan_unix.go's
    appearsOrphaned; called by Status only when the holder is alive (always true on Windows per processAlive).
  - FOLLOW pattern: lock_windows.go processAlive (one-line func + godoc citing FR-K7 + cross-platform twin).

Task 3: ADD func Status to internal/lock/lock.go — the exported read-only diagnostic
  - PLACE: near the other exported helpers (e.g. right after ReleaseCurrent ~line 217, or after IsHeldError
    at the file tail — pick the spot gofmt keeps clean; Status is self-contained and stdlib-only).
  - SIGNATURE: func Status(repoPath string) (path string, contents LockContents, alive bool, orphan bool, err error)
  - IMPLEMENTATION (the exact wrap order from architecture/lock_extension.md):
      path, err = lockPath(repoPath)
      if err != nil { return "", LockContents{}, false, false, fmt.Errorf("lock path: %w", err) }
      data, err := os.ReadFile(path)
      if err != nil {
          if os.IsNotExist(err) { return "", LockContents{}, false, false, nil }   // no lock held
          return "", LockContents{}, false, false, fmt.Errorf("read lock file: %w", err)
      }
      contents = parseContents(data)
      pid, perr := strconv.Atoi(contents.Pid)
      if perr != nil { return path, contents, false, false, nil }   // malformed pid — path+contents are still useful; can't assess liveness
      alive = processAlive(pid, contents.Hostname)
      if alive { orphan = appearsOrphaned(pid) }                    // dead pid's orphan status is irrelevant
      return path, contents, alive, orphan, nil
  - IMPORTS: Status uses fmt, os, strconv — ALL already imported in lock.go (lock.go imports bufio, crypto/sha256,
    encoding/hex, errors, fmt, os, path/filepath, strconv, strings, sync/atomic, time). NO new import needed.
    (appearsOrphaned/processAlive are in the same package — no import.)
  - GODOC [Mode A]: "Status returns the parsed lock-file state for repoPath: the lock file path, the holder's
    parsed contents, whether the holder process is alive (processAlive), and — on Unix — whether it APPEARS
    orphaned (its parent is init/a subreaper, i.e. reparented — appearsOrphaned). READ-ONLY: it never acquires
    the flock and never breaks/removes a lock (FR52 preserved). path=='' (with a nil error) means no lock is
    held for this repo. Consumed by `stagecoach lock status` (FR-K4) and the Busy-message orphan hint (FR-K5)."
  - FOLLOW pattern: ReleaseCurrent (line 208) / IsHeldError (line 311) — self-contained exported helper, stdlib-only.
  - GOTCHA: lockPath returns (string, error) — handle it. os.IsNotExist is the ONLY swallow. Don't call
    appearsOrphaned when !alive. NO flock, NO os.Remove, NO Acquire anywhere in Status.

Task 4: ADD tests to internal/lock/lock_unix_test.go (//go:build !windows) — REUSE writeLockFile
  - TestAppearsOrphaned_DeadPidIsConservativeFalse:
      deadPID via exec.Command("true"); Start; Wait. assert !appearsOrphaned(deadPID)  // /proc missing / ps non-zero → false
  - TestAppearsOrphaned_SelfIsNotOrphan (normal dev shells):
      // parent != 1 in a normal shell; assert !appearsOrphaned(os.Getpid()) with a comment that CI-under-init
      // could differ (heuristic). This pins the function runs + the common case.
  - TestStatus_NoLockFile (the path=='' contract):
      t.Setenv("XDG_RUNTIME_DIR", t.TempDir()); t.Setenv("XDG_CACHE_HOME",""); repo := t.TempDir()
      path, contents, alive, orphan, err := Status(repo)
      assert err==nil, path=="", contents==(zero LockContents), alive==false, orphan==false
  - TestStatus_PlantedSelfLock (alive path):
      isolate XDG; repo := t.TempDir(); resolve lockPath via the package-internal lockPath(repo); MkdirAll dir;
      writeLockFile(t, path, strconv.Itoa(os.Getpid()), thisHost)   // reuse the existing helper
      path2, contents, alive, orphan, err := Status(repo)
      assert err==nil; path2==path; contents.Pid==strconv.Itoa(os.Getpid()); contents.Hostname==thisHost; alive==true; orphan==appearsOrphaned(os.Getpid())
  - TestStatus_MalformedPid (parseContents ok, Atoi fails → alive/orphan false, path+contents returned):
      isolate XDG; plant a lock file with pid=not-a-number; Status → err==nil, path set, contents.Pid=="not-a-number", alive==false, orphan==false
  - TestStatus_LockPathError (lockPath fails → err propagated):
      t.Setenv("XDG_RUNTIME_DIR",""); t.Setenv("XDG_CACHE_HOME",""); t.Setenv("HOME","")  // forces lockDir error
      _, _, _, _, err := Status(t.TempDir()); assert err != nil   // (os.UserHomeDir fails with no HOME on most systems)
  - FOLLOW pattern: the existing TestProcessAlive_* (self/foreign/empty/dead branches) + the writeLockFile reuse
    in TestAcquire_ReapsDeadPidFile_SparesLive.
  - NAMING: TestAppearsOrphaned_<Scenario>; TestStatus_<Scenario>. snake-camel as the existing tests.
  - GOTCHA: Status does NOT touch the `current` singleton, so resetCurrent(t) is NOT needed for Status tests
    (only Acquire tests need it). Do NOT Acquire in Status tests — plant the file directly via writeLockFile.
  - GOTCHA: macOS t.TempDir() is under /var → /private/var; contents.Repo won't equal the raw repo (canonical
    path). For the planted-self-lock test, compare Pid/Hostname (which writeLockFile sets verbatim), NOT Repo.
  - COVERAGE: the orphan==true branch is NOT unit-tested (E2E, P1.M4.T1.S1) — add a comment stating this.

Task 5 (OPTIONAL): ADD a minimal internal/lock/lock_windows_test.go (//go:build windows)
  - func TestAppearsOrphaned_AlwaysFalse(t) { if appearsOrphaned(1) { t.Error(...) } }
  - LOW value (one-liner stub); add ONLY if zero-effort. The cross-compile + the !windows tests are the real net.
  - GOTCHA: there is NO lock_windows_test.go today, so this would be the FIRST windows test file — acceptable
    but not required by the contract ("Mock nothing — test with temp lock files and known pids" ⇒ those live in !windows).

Task 6: VERIFY — build (native+cross-compile), vet, format, full regression, lint, grep guards
  - go build ./... ; GOOS=windows go build ./... ; GOOS=linux go build ./... ; GOOS=darwin go build ./...
  - go vet ./internal/lock/...
  - gofmt -l internal/lock/        # must be empty
  - go test ./internal/lock/ -race -v
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: Status is the read-only composite (the entire feature). It reuses the SAME lockPath + file
// format + processAlive Acquire/HeldError use, so the read path and write path can never disagree.
func Status(repoPath string) (path string, contents LockContents, alive bool, orphan bool, err error) {
	path, err = lockPath(repoPath)
	if err != nil {
		return "", LockContents{}, false, false, fmt.Errorf("lock path: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", LockContents{}, false, false, nil // no lock held (FR-K4 "no run lock for <repo>")
		}
		return "", LockContents{}, false, false, fmt.Errorf("read lock file: %w", err)
	}
	contents = parseContents(data) // never errors; empty fields for malformed lines
	pid, perr := strconv.Atoi(contents.Pid)
	if perr != nil {
		return path, contents, false, false, nil // path+contents are diagnostic; can't assess liveness
	}
	alive = processAlive(pid, contents.Hostname)
	if alive {
		orphan = appearsOrphaned(pid) // a dead pid's orphan status is irrelevant
	}
	return path, contents, alive, orphan, nil
}

// PATTERN: the Unix orphan heuristic (single file, runtime.GOOS dispatch, conservative-false-on-error).
func appearsOrphaned(pid int) bool {
	ppid, err := ppidOf(pid)
	if err != nil {
		return false // conservative: don't claim orphan on ambiguity (proc gone, ps failure, parse fail)
	}
	return ppid == 1 // reparented to init/launchd — §9.27 orphaned-but-alive; subreapers may have ppid≠1 (limitation)
}

func ppidOf(pid int) (int, error) {
	if runtime.GOOS == "linux" {
		return ppidLinux(pid)
	}
	return ppidViaPs(pid) // darwin + BSDs
}

func ppidLinux(pid int) (int, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err // ENOENT when pid is gone → appearsOrphaned returns false
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text()) // splits on tab/spaces; colon stays glued to "PPid"
		if len(fields) >= 2 && fields[0] == "PPid:" {
			return strconv.Atoi(fields[1])
		}
	}
	if err := s.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("orphan: no PPid field for pid %d", pid)
}

func ppidViaPs(pid int) (int, error) {
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, err // *exec.ExitError when pid is missing → appearsOrphaned returns false
	}
	return strconv.Atoi(strings.TrimSpace(string(out))) // TrimSpace MANDATORY (ps right-justifies)
}

// PATTERN: the Windows no-op (orphan_windows.go).
func appearsOrphaned(pid int) bool { return false } // FR-K7: Windows has no init-reparenting analog
```

### Integration Points

```yaml
EXPORTS (internal/lock):
  - ADD func Status(repoPath string) (path string, contents LockContents, alive bool, orphan bool, err error)
  - ADD func appearsOrphaned(pid int) bool   # UNEXPORTED — internal to the package; Status is the only caller

IMPORTS:
  - internal/lock/lock.go: NO new imports (fmt, os, strconv already present). Status calls same-package helpers.
  - internal/lock/orphan_unix.go (NEW): bufio, fmt, os, os/exec, runtime, strconv, strings (ALL stdlib).
  - internal/lock/orphan_windows.go (NEW): NONE (one-liner, no imports).

CONSUMERS (treat as contracts — they land AFTER this item; do NOT implement them here):
  - P1.M3.T2.S1 (stagecoach lock status): calls lock.Status(repoDir); path=='' ⇒ "no run lock for <repo>";
    else prints path/contents/alive/orphan. READ-ONLY (FR-K4).
  - P1.M3.T3.S1 (Busy message orphan hint): in handleLockContention (default_action.go:300), call
    lock.Status(repoDir) to obtain the `orphan` bool (the lock file still exists at contention time — Acquire
    returned HeldError, no removal happened) and conditionally print the orphan hint. appearsOrphaned stays
    unexported; the consumer reaches it via Status. (On Windows Status reports alive=true, orphan=false.)

NO database / migration / routes / new types / new flag / config change / docs change / CLI command.
  - The `lock status` SUBCOMMAND is P1.M3.T2.S1 (separate item). This item is ONLY the library API.
  - The Busy-message REFORMAT is P1.M3.T3.S1 (separate item).
  - The README/docs SYNC is P1.M4.T2.S1.
  - The E2E orphan scenarios are P1.M4.T1.S1 (proves the orphan==true path this item cannot unit-test).

SCOPE FENCES:
  - Touches ONLY internal/lock/{lock.go (edit), orphan_unix.go (new), orphan_windows.go (new), lock_unix_test.go (edit)}.
  - Does NOT edit Acquire/Release/reapStaleLocks/processAlive/lockPath/parseContents (read-only reuse),
    default_action.go, root.go, cmd/stagecoach/main.go, internal/signal/*, internal/config/*, or any PRD/task file.
  - Adds NO third-party dependency (go.mod unchanged — all new imports are stdlib).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross-compile (orphan_windows.go must compile; orphan_unix.go's os/exec must compile on linux+darwin;
# the runtime.GOOS file must compile on every Unix target).
go build ./...
GOOS=linux   go build ./...
GOOS=darwin  go build ./...
GOOS=windows go build ./...
# Expected: all clean. If GOOS=windows fails, orphan_windows.go is missing the build tag or has a stray Unix symbol.
#           If GOOS=linux fails, an import is unused on Linux (re-check the runtime.GOOS approach — every import
#           must be referenced by ≥1 function in the file).

# Vet.
go vet ./internal/lock/...
# Expected: clean.

# Format — every new/edited file must be gofmt-clean (build-tag blank line, import grouping, func indentation).
gofmt -l internal/lock/
# Expected: empty. If listed: gofmt -w internal/lock/orphan_unix.go internal/lock/orphan_windows.go internal/lock/lock.go internal/lock/lock_unix_test.go

# Lint.
make lint      # golangci-lint (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. `unused` could fire only if appearsOrphaned/ppidOf were uncalled — Status calls appearsOrphaned.

# Scope guard: ONLY internal/lock/ files changed (or + the optional lock_windows_test.go).
git status --porcelain
# Expected: internal/lock/lock.go, internal/lock/orphan_unix.go, internal/lock/orphan_windows.go,
#           internal/lock/lock_unix_test.go (and optionally internal/lock/lock_windows_test.go). ZERO changes outside internal/lock/.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The lock package tests (Unix host runs the //go:build !windows tests incl. the new appearsOrphaned + Status tests).
go test ./internal/lock/ -race -v
# Expected: ALL pre-existing tests (Acquire/Release/reaping/processAlive/parseContents/lockPath) PASS unchanged
#           + the new TestAppearsOrphaned_* and TestStatus_* PASS.
#           - TestStatus_NoLockFile: path=="", nil err.
#           - TestStatus_PlantedSelfLock: alive==true, contents match.
#           - TestStatus_MalformedPid: path set, alive==false, orphan==false.
#           - TestStatus_LockPathError: err != nil.
#           - TestAppearsOrphaned_DeadPidIsConservativeFalse: false.
#           - TestAppearsOrphaned_SelfIsNotOrphan: false (common dev case).

# Full race suite (regression — this item is additive; no existing behavior changes).
make test
# Expected: green (race detector). internal/lock has no shared mutable state in the new code (Status reads; appearsOrphaned is pure).

# NOTE: internal/lock is NOT in the coverage-gate list (Makefile line 77 gates internal/{git,provider,generate,config} only),
# so `make coverage-gate` is unaffected. The orphan==true branch is uncovered by design (E2E, P1.M4.T1.S1) — document it in the test.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary (proves the new exported Status links into the binary; no import cycle / link error).
make build

# Manual sanity: Status is a library API — exercise it via a tiny throwaway program (or defer to P1.M3.T2.S1's
# `lock status` subcommand once it lands). Throwaway check on a Unix dev host:
cat > /tmp/status_check.go <<'EOF'
package main
import ("fmt"; "os"; "github.com/dustin/stagecoach/internal/lock")
func main() {
    repo := os.Args[1]
    path, c, alive, orphan, err := lock.Status(repo)
    fmt.Printf("path=%q alive=%v orphan=%v err=%v pid=%q host=%q\n", path, alive, orphan, err, c.Pid, c.Hostname)
}
EOF
go run /tmp/status_check.go "$(pwd)"     # a repo with NO stagecoach lock → path="" (no lock held)
# Plant a fake lock for the current repo's path to observe the alive/orphan read:
#   (resolve the path via the same hash the package uses; or just acquire+release in another throwaway and read mid-flight)
# This is a smoke check — the unit tests (Task 4) are the real proof. Remove /tmp/status_check.go after.

# Expected: path=="" with nil err for a repo with no lock. (The lock-status subcommand P1.M3.T2.S1 is the user-facing surface.)
```

> **Note**: this item is a library API; its user-visible surface (`stagecoach lock status`, the Busy-message
> orphan hint) is P1.M3.T2.S1 / P1.M3.T3.S1. The within-scope proof is: clean build (incl. cross-compile) +
> vet/gofmt/lint + the unit tests (Task 4) + the full regression suite green + the grep guards. The
> orphan==true end-to-end scenario (real reparented-to-init holder) is P1.M4.T1.S1.

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: Status is exported and has the EXACT 5-value signature.
grep -n 'func Status(repoPath string) (path string, contents LockContents, alive bool, orphan bool, err error)' internal/lock/lock.go
# Expect: 1 hit.

# Guard 2: Status is READ-ONLY — it must NOT call flock, os.Remove, Acquire, or any mutation.
grep -n 'func Status' internal/lock/lock.go
# then inspect the body: it may reference ONLY lockPath, os.ReadFile, parseContents, strconv.Atoi, processAlive, appearsOrphaned.
grep -A25 'func Status(repoPath' internal/lock/lock.go | grep -E 'flock|os\.Remove|Acquire|os\.OpenFile|writeContents'
# Expect: ZERO hits (Status never mutates / never takes the lock — FR52 preserved).

# Guard 3: appearsOrphaned is UNEXPORTED and present in BOTH platform files.
grep -rn 'func appearsOrphaned' internal/lock/
# Expect: orphan_unix.go (1) + orphan_windows.go (1). Lowercase 'a' — unexported.

# Guard 4: build tags are the MODERN form with a blank line before package.
head -3 internal/lock/orphan_unix.go      # Expect: `//go:build !windows`, blank, `package lock`
head -3 internal/lock/orphan_windows.go   # Expect: `//go:build windows`, blank, `package lock`
grep -c '+build' internal/lock/orphan_*.go # Expect: 0 (no legacy +build lines).

# Guard 5: the Linux branch matches the WHOLE "PPid:" token (not a prefix that would match "Pid:").
grep -n 'PPid:' internal/lock/orphan_unix.go
# Expect: a line like `if len(fields) >= 2 && fields[0] == "PPid:" {`.

# Guard 6: the Darwin branch TrimSpaces before Atoi (ps right-justifies).
grep -n 'TrimSpace' internal/lock/orphan_unix.go
# Expect: 1 hit inside ppidViaPs.

# Guard 7: conservative-false-on-error in appearsOrphaned (the core contract).
grep -A4 'func appearsOrphaned' internal/lock/orphan_unix.go
# Expect: `ppid, err := ppidOf(pid); if err != nil { return false }; return ppid == 1`.

# Guard 8: Status calls appearsOrphaned ONLY when alive.
grep -B2 -A1 'appearsOrphaned(pid)' internal/lock/lock.go
# Expect: the call is inside `if alive { ... }`.

# Guard 9: stdlib-only — no golang.org/x/sys or stagecoach imports in the new files.
grep -E 'golang.org/x|stagecoach' internal/lock/orphan_unix.go internal/lock/orphan_windows.go
# Expect: ZERO hits (package stays a self-contained stdlib-only leaf).

# Guard 10: scope — only internal/lock/ changed.
git status --porcelain
# Expect: files only under internal/lock/ (lock.go, orphan_unix.go, orphan_windows.go, lock_unix_test.go; +optional lock_windows_test.go).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=linux` + `GOOS=darwin` + `GOOS=windows` all clean
- [ ] `go vet ./internal/lock/...` clean
- [ ] `gofmt -l internal/lock/` empty
- [ ] `make lint` zero errors (appearsOrphaned is called by Status ⇒ no `unused`)
- [ ] `go test ./internal/lock/ -race -v` green (existing tests unchanged + new appearsOrphaned/Status tests pass)
- [ ] `make test` (full race suite) green

### Feature Validation
- [ ] `func Status(repoPath string) (path string, contents LockContents, alive bool, orphan bool, err error)` exported in lock.go
- [ ] `Status` returns `("", LockContents{}, false, false, nil)` when the lock file does not exist; propagates other errors via `err`
- [ ] `Status` composes ONLY lockPath/os.ReadFile/parseContents/strconv.Atoi/processAlive/appearsOrphaned (grep guard 2: no flock/Remove/Acquire)
- [ ] `Status` calls `appearsOrphaned` ONLY when `alive` (grep guard 8)
- [ ] `appearsOrphaned` exists in BOTH orphan_unix.go and orphan_windows.go, unexported (grep guard 3)
- [ ] orphan_unix.go Linux branch matches whole `"PPid:"` token; Darwin branch TrimSpaces (grep guards 5–6)
- [ ] `appearsOrphaned` returns false on any error and `ppid==1` otherwise (grep guard 7)
- [ ] orphan_windows.go is the one-line `return false` (FR-K7)
- [ ] Grep guards 1–10 (Level 4) all pass

### Scope-Boundary Validation
- [ ] `git status` shows ONLY files under `internal/lock/` (lock.go edit + orphan_unix.go/orphan_windows.go new + lock_unix_test.go edit; +optional lock_windows_test.go)
- [ ] NO edit to Acquire/Release/reapStaleLocks/processAlive/lockPath/parseContents (read-only reuse), default_action.go, root.go,
      cmd/stagecoach/main.go, internal/signal/*, internal/config/*, or any test outside internal/lock/
- [ ] NO new CLI flag, NO new exported TYPE, NO third-party dependency (go.mod unchanged — all new imports stdlib)
- [ ] NO `lock status` subcommand (that's P1.M3.T2.S1), NO Busy-message change (P1.M3.T3.S1)

### Code Quality & Docs
- [ ] [Mode A] Godoc on `Status`: read-only, never breaks/removes a lock (FR52), path=='' means no lock, consumed by lock status + Busy hint
- [ ] [Mode A] Godoc on `appearsOrphaned` (both files): the ppid==1 heuristic, the subreaper limitation (PR_SET_CHILD_SUBREAPER),
      the conservative-false-on-error contract, FR-K4/K7 + the cross-platform twin reference
- [ ] Build tags modern form (`//go:build !windows` / `//go:build windows`) with blank-line-before-package; no legacy `// +build`
- [ ] Tests reuse `writeLockFile` and the dead-pid trick; document the orphan==true branch is E2E (P1.M4.T1.S1)

---

## Anti-Patterns to Avoid

- ❌ Don't have `Status` acquire the flock, call `Acquire`, `os.Remove`, or mutate anything. It is READ-ONLY
  (FR-K4: "It changes nothing"; FR52 preserved). The grep guard 2 enforces this. If you find yourself needing
  `flock`, you are reimplementing `Acquire` — stop. Status reads the file `Acquire` would have contended on.
- ❌ Don't ignore `lockPath`'s error return. `func lockPath(repoPath string) (string, error)` — lockDir can fail
  (no XDG + no HOME + no CWD fallback). Propagate it via `err`. (The item description's `lockPath(repoPath)`
  pseudo-code glossed over the error; the real signature has it.)
- ❌ Don't conflate "file not found" with "read error". `os.IsNotExist(err)` ⇒ no-lock (path="", nil err); any
  OTHER `os.ReadFile` error ⇒ propagate via `err`. Swallowing a permission error as "no lock" would mislead.
- ❌ Don't call `appearsOrphaned` when the holder is dead. A dead pid's orphan status is meaningless (it'll be
  reaped anyway). Gate the call on `if alive { orphan = appearsOrphaned(pid) }` (grep guard 8). processAlive
  already returned false for a dead pid.
- ❌ Don't match `PPid` with a bare prefix check (`strings.HasPrefix(line, "PPid")`). The `Pid:` line precedes
  `PPid:` in `/proc/<pid>/status` and a prefix match would hit it. Compare the WHOLE token
  `fields[0] == "PPid:"` (strings.Fields keeps the colon glued).
- ❌ Don't skip `strings.TrimSpace` on the Darwin `ps` output. `ps -o ppid=` right-justifies the number
  (`"     1"`); `strconv.Atoi` on leading whitespace fails. Always `strconv.Atoi(strings.TrimSpace(string(out)))`.
- ❌ Don't make `appearsOrphaned` return `true` on error. It is a diagnostic HINT feeding a user's `kill`/`rm`
  decision — false-positive orphan claims could prompt the user to kill a legitimately-parented run. Return
  `false` on ANY error/ambiguity (proc gone, ps failure, parse fail). The only `true` is `ppid == 1`.
- ❌ Don't export `appearsOrphaned`. The contract pins it unexported; `Status` is the single entry point and
  the consumers (P1.M3.T2.S1/T3.S1) reach the orphan bool via `Status`'s return. Exporting it would widen the
  surface unnecessarily.
- ❌ Don't split Linux/Darwin into `ppid_linux.go`/`ppid_darwin.go` (build-tag-per-OS). The single-file
  `runtime.GOOS` approach (one `orphan_unix.go`) is simpler, matches the item description's
  "on Darwin (runtime.GOOS=='darwin')", and is valid because every import is referenced by ≥1 compiled function
  (Go errors on unused imports, not unused package-level funcs). Two files is needless complexity here.
- ❌ Don't add a `golang.org/x/sys` or stagecoach import to the new files. The lock package is stdlib-only by
  invariant (package doc, lock.go:1). The new imports — os/exec, runtime, bufio, strconv, strings, os, fmt —
  are ALL stdlib. (grep guard 9 enforces.)
- ❌ Don't use the legacy `// +build` build-tag form. The codebase is Go 1.22 and uses modern `//go:build` only
  (lock_unix.go:1, lock_windows.go:1). Match it exactly, with the blank-line-before-package rule.
- ❌ Don't add a unit test that asserts the orphan==true path by trying to create a real reparented-to-init pid.
  It's flaky/OS-dependent. Pin the DETERMINISTIC branches (dead pid → false; self → false in dev shells;
  malformed pid → alive/orphan false; no lock → path==''; lockPath error → err). The orphan==true scenario is
  the E2E harness (P1.M4.T1.S1), which spawns a real stagecoach subprocess and kills the launcher. Document
  this in the test (a comment on the uncovered branch).
- ❌ Don't touch the `current` singleton or call `resetCurrent(t)` in Status tests. Status never Acquires and
  never touches `current` — it reads the file directly via `lockPath`/`os.ReadFile`. Plant the fixture with
  `writeLockFile`, don't `Acquire`.
- ❌ Don't implement the `lock status` subcommand or the Busy-message reformat here. Those are P1.M3.T2.S1 and
  P1.M3.T3.S1 (separate items). This item is SOLELY the library API (`Status` + `appearsOrphaned`) both consume.

---

## Confidence Score: 9/10

This is a small, purely-additive change to a self-contained stdlib-only package, with every integration point
verified against the real code: the EXACT signatures of the three helpers `Status` composes (including
`lockPath`'s `(string,error)` return that the item description glossed over), the `LockContents` struct, the
build-tag convention (modern `//go:build`, no legacy `+build`, blank-line rule), the stdlib-only invariant +
why the single-file `runtime.GOOS` approach is valid, the `/proc/<pid>/status` `PPid:` parse (whole-token
match) and the Darwin `ps -o ppid=` parse (TrimSpace + non-zero-exit handling), the conservative-false
contract, the test helpers to reuse (`writeLockFile`, the `exec.Command("true")` dead-pid trick, XDG
isolation), the coverage-gate exemption for `internal/lock`, the consumer contracts (P1.M3.T2.S1/T3.S1 reach
orphan via `Status`), and 10 grep guards. The change is small enough to be fully specified verbatim. The -1
from 10/10 reflects: (a) the orphan==true path is provable only by the E2E harness (P1.M4.T1.S1), not this
item's unit tests — a deliberate, documented limitation, not a gap; and (b) the macOS `t.TempDir()` symlink
quirk means the planted-lock test must compare Pid/Hostname (set verbatim by `writeLockFile`) rather than Repo
(canonical-path-dependent) — a minor test-design care the implementer must observe.
