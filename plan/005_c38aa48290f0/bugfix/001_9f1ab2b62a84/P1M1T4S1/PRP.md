name: "P1.M1.T4.S1 — Replace char-device heuristic with platform-specific isatty ioctl probe"
description: |
  Fixes Issue 4 (Minor): `ui.IsTerminal` (internal/ui/output.go L20–L30) tests
  `stat.Mode() & os.ModeCharDevice != 0`. `/dev/null` IS a char device → returns `true`, so
  `IsTerminal(os.Stdin)` misfires when stdin is `/dev/null`. This breaks the FR-L3 `config init
  --interactive` TTY gate (bypasses the "requires a terminal" message, crashes with "unexpected end
  of input") and the FR-I3c `integrate DefaultConfirm` non-interactive auto-decline (skips the
  "non-interactive stdin — declining" notice). The fix replaces the heuristic with a TRUE isatty
  probe: `ioctl(TCGETS/TIOCGETA)` on Linux/Darwin (errno==0 ⇒ terminal), `GetConsoleMode` on
  Windows (nonzero ⇒ console), and the OLD char-device heuristic as a safe fallback on any other
  GOOS. Implemented as 4 build-tag-gated files in `internal/ui/` (isatty_linux.go,
  isatty_darwin.go, isatty_windows.go, isatty_other.go) following the established procgroup_*.go /
  signal_*.go platform-split pattern — STDLIB-ONLY (`syscall` + `unsafe`, NO golang.org/x/term or
  golang.org/x/sys, keeping go.mod/go.sum byte-unchanged). The public signature
  `func IsTerminal(f *os.File) bool` is UNCHANGED; output.go's body becomes a one-liner delegating
  to `isTerminalFd(f.Fd())`. All 4 callers benefit automatically. TWO new tests
  (TestIsTerminal_DevNull + the existing TestIsTerminal_Pipe) lock the fix. No deps, no API change,
  no config/loader change.

---

## Goal

**Feature Goal**: `ui.IsTerminal` returns `true` ONLY for real terminals/ptys, and `false` for
`/dev/null`, pipes, files, and redirects — across all supported platforms (linux, darwin, windows;
amd64+arm64; CGO_ENABLED=0) — using a stdlib-only true isatty probe (ioctl on Unix, GetConsoleMode
on Windows), with a safe char-device fallback for any untested GOOS.

**Deliverable**: 5 files in `internal/ui/`:
1. `internal/ui/isatty_linux.go` (NEW) — `//go:build linux`: `const ioctlReadTermios = syscall.TCGETS` + `func isTerminalFd(fd uintptr) bool` (ioctl probe).
2. `internal/ui/isatty_darwin.go` (NEW) — `//go:build darwin`: `const ioctlReadTermios = syscall.TIOCGETA` + `func isTerminalFd(fd uintptr) bool` (ioctl probe).
3. `internal/ui/isatty_windows.go` (NEW) — `//go:build windows`: `var procGetConsoleMode = ...kernel32.dll!GetConsoleMode` + `func isTerminalFd(fd uintptr) bool` (GetConsoleMode probe).
4. `internal/ui/isatty_other.go` (NEW) — `//go:build !linux && !darwin && !windows`: `func isTerminalFd(fd uintptr) bool` (old char-device heuristic — safe fallback).
5. `internal/ui/output.go` (MODIFY) — `IsTerminal` body → `return isTerminalFd(f.Fd())`; update the doc comment to drop the "NOT a true isatty ioctl" note.
6. `internal/ui/output_test.go` (MODIFY) — add `runtime` import + `TestIsTerminal_DevNull`; `TestIsTerminal_Pipe` stays as-is (already green).

**Success Definition**:
- `IsTerminal(os.Stdin)` returns `false` when stdin is `/dev/null` (Unix) — verified by the new test.
- `IsTerminal` on a pipe (os.Pipe) still returns `false` — verified by the existing TestIsTerminal_Pipe (unchanged, stays green).
- `IsTerminal` on a real terminal/pty returns `true` (ioctl succeeds with errno==0; GetConsoleMode returns nonzero).
- The package compiles on EVERY supported GOOS (linux/darwin/windows × amd64/arm64, CGO_ENABLED=0) AND any other GOOS (via the `_other` fallback) — verified by cross-compilation.
- go.mod/go.sum are byte-unchanged (no new dependencies).
- The public signature `func IsTerminal(f *os.File) bool` is unchanged; all 4 callers compile unmodified.

## User Persona (if applicable)

**Target User**: A developer (or automation/CI script) running Stagecoach non-interactively with stdin
redirected from `/dev/null` — a ubiquitous pattern (`some-cmd < /dev/null`, `</dev/null` in cron,
`head -c0 </dev/null`, git hooks spawned without a tty).

**Use Case**: `stagecoach config init --interactive --force < /dev/null` should print the clean FR-L3
"`config init --interactive` requires a terminal on stdin" hint and exit 1 — NOT bypass the gate,
print "Detected providers" + the prompt, then crash with "unexpected end of input".

**Pain Points Addressed**: Today `/dev/null` (a char device) is mistaken for a terminal, so the
interactive gate is bypassed and DefaultConfirm's explicit non-interactive notice is skipped. The
fix makes `/dev/null` correctly detected as non-TTY, restoring the documented FR-L3 / FR-I3c behavior.

## Why

- **PRD refs**: §9.23 FR-L3 (`config init --interactive` TTY gate: non-TTY → exit 1 pointing at plain `config init`); §9.21 FR-I3c (integrate `DefaultConfirm` non-interactive auto-decline: "When stdin is NOT a terminal … it AUTO-DECLINES without blocking").
- **Correctness**: the current heuristic conflates "char device" with "terminal". `/dev/null` is a char device but NOT a terminal. The true isatty probe (ioctl `TCGETS`/`TIOCGETA`, or `GetConsoleMode`) succeeds ONLY on real terminals/ptys.
- **Dep-free principle preserved**: the fix uses ONLY stdlib `syscall` + `unsafe` — exactly the same seams the project already uses for `procgroup_*.go` and `signal_*.go`. No `golang.org/x/term`, no `golang.org/x/sys` (go.mod/go.sum untouched).
- **Lowest-risk mechanism**: the public `IsTerminal` signature is frozen (4 callers depend on it); only the BODY changes. All callers benefit automatically — zero edits outside `internal/ui/`.

## What

No user-visible API change. `IsTerminal`'s signature and return type are unchanged; its RETURN VALUES become more accurate (false for `/dev/null`). Behavior changes:

1. `config init --interactive < /dev/null` → now hits the FR-L3 "requires a terminal" gate (exit 1, clean message) instead of bypassing it and crashing.
2. `integrate ... DefaultConfirm` with `/dev/null` stdin → now takes the explicit FR-I3c "non-interactive stdin — declining" auto-decline path (was declining by EOF accident, skipping the notice).
3. Color resolution (`hookexec.go`, `default_action.go`) is cosmetically unaffected in practice (stdout is rarely `/dev/null`), but is now technically more correct.

### Success Criteria

- [ ] `IsTerminal` of an `os.File` opened on `/dev/null` returns `false` (Unix test).
- [ ] `IsTerminal` of an `os.Pipe()` reader/writer returns `false` (existing test, unchanged).
- [ ] The package cross-compiles for `linux/{amd64,arm64}`, `darwin/{amd64,arm64}`, `windows/{amd64,arm64}` with `CGO_ENABLED=0`.
- [ ] The package compiles for an "other" GOOS (e.g. `GOOS=freebsd`) via the `_other` fallback (build never breaks on an untested platform).
- [ ] `go.mod` / `go.sum` are byte-for-byte unchanged (no `go get`, no new require).
- [ ] `func IsTerminal(f *os.File) bool` signature unchanged; all 4 callers compile unmodified.
- [ ] `go build ./...`, `go vet ./internal/ui/...`, `go test ./internal/ui/...`, `gofmt` all pass.

## All Needed Context

### Context Completeness Check

_This PRP names the exact bug site (output.go L20–L30), the exact 4 new files with their build tags
and complete copy-ready Go source (including the `unsafe.Pointer` + `syscall.Syscall6` idiom and the
`kernel32.dll!GetConsoleMode` lazy-resolution idiom copied from procgroup_windows.go), the verified
syscall constants (TCGETS/TIOCGETA/SYS_IOCTL all confirmed via `go doc`), the exhaustive-build-tag
proof, the frozen-signature contract, the 4 callers that auto-benefit, the existing test that must
stay green, and the new test (copy-ready, with the `runtime.GOOS` skip guard). An implementer with
zero codebase knowledge can complete it in one pass._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/architecture/issue_analysis.md
  why: §Issue 4 root-cause + fix design — the authoritative design (4 platform files, [128]byte buffer,
       GetConsoleMode, fallback heuristic). Mirrors the contract in the work item description.
  section: "## Issue 4 (Minor): `ui.IsTerminal` treats `/dev/null` as a terminal"
  critical: "Confirms the ioctl approach, the linux/darwin/windows/other split, and the 4 caller sites."

- docfile: plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/P1M1T4S1/research/findings.md
  why: Verified syscall constants (TCGETS/TIOCGETA/SYS_IOCTL via `go doc`), the procgroup Windows
       lazy-resolution idiom, the baseline cross-compile results, the exhaustive build-tag proof, and
       external API references (x/term, GetConsoleMode docs).
  critical: "Justifies [128]byte over syscall.Termios (layout-agnostic; we only test errno==0). Confirms
             baseline builds green on linux/darwin/windows before the change."

- file: internal/ui/output.go
  why: The function under fix. IsTerminal L20–L30 (the char-device heuristic to replace). The doc comment
       to update (drop "NOT a true isatty ioctl"). The body becomes `return isTerminalFd(f.Fd())`.
  pattern: |
    // BEFORE (L20-L30):
    func IsTerminal(f *os.File) bool {
        stat, err := f.Stat()
        if err != nil { return false }
        return (stat.Mode() & os.ModeCharDevice) != 0
    }
    // AFTER:
    func IsTerminal(f *os.File) bool {
        return isTerminalFd(f.Fd())   // platform-specific (isatty_*.go)
    }
  gotcha: "Keep the signature EXACTLY `func IsTerminal(f *os.File) bool`. `f.Fd()` returns uintptr — that
           is the arg type of isTerminalFd. The `os` and `io`/`fmt` imports stay; IsTerminal no longer calls
           f.Stat() so no new imports are needed in output.go (stat was already used only here; if the linter
           flags an unused import, the other funcs in the file still use os via os.Stdout/os.Stderr in New)."

- file: internal/provider/procgroup_windows.go
  why: The exact Windows kernel32 lazy-resolution idiom to mirror for isatty_windows.go.
  pattern: |
    var procGenerateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")
    ...
    r1, _, err := procGenerateConsoleCtrlEvent.Call(uintptr(...), uintptr(...))
    if r1 == 0 { return err }   // 0 = Win32 FALSE = failure
    return nil
  gotcha: "We mirror this for procGetConsoleMode. GetConsoleMode returns nonzero (TRUE) on a console handle,
           0 otherwise. Our isTerminalFd returns `r1 != 0` (we do NOT need the err — only the boolean)."

- file: internal/provider/procgroup_unix.go
  why: The `//go:build !windows` platform-split precedent (coarse). Our linux/darwin split is FINER because
       the ioctl const differs per Unix flavor.
  pattern: "//go:build !windows  (then package, imports, func). We split into //go:build linux and //go:build darwin."
  gotcha: "A single !windows file CANNOT conditionally set the ioctl const (Go consts aren't conditionally
           compiled by runtime). So: 2 separate files (linux, darwin), each its own const + its own isTerminalFd."

- file: internal/ui/output_test.go
  why: (1) TestIsTerminal_Pipe L269 — EXISTING, must stay green (pipes are not char devices AND ioctl fails on
       them → false under both impls; do NOT edit). (2) The new TestIsTerminal_DevNull goes alongside it.
       (3) Current imports (bytes, fmt, os, strings, testing) — ADD `runtime` for the GOOS skip guard.
  pattern: |
    func TestIsTerminal_DevNull(t *testing.T) {
        if runtime.GOOS == "windows" {
            t.Skip("/dev/null is a Unix path; Windows uses NUL and is tested on a Windows CI runner")
        }
        f, err := os.Open("/dev/null")
        if err != nil { t.Fatalf("os.Open(/dev/null): %v", err) }
        defer f.Close()
        if IsTerminal(f) { t.Error("IsTerminal(/dev/null) = true, want false (true isatty probe must reject it)") }
        // PROVE the old heuristic would have misfired: /dev/null IS a char device.
        st, err := os.Stat("/dev/null")
        if err != nil { t.Fatalf("os.Stat(/dev/null): %v", err) }
        if (st.Mode() & os.ModeCharDevice) == 0 {
            t.Error("/dev/null is unexpectedly NOT a char device — the regression premise is invalid on this OS")
        }
    }
  gotcha: "The `os.Stat` check is a BELT-AND-SUSPENDERS assertion that proves WHY the old heuristic was wrong
           (it guards against a future test env where /dev/null stops being a char device). If it ever fails,
           the regression premise itself changed — investigate rather than weaken the assertion."
```

### Current Codebase tree (relevant slice)

```bash
internal/ui/output.go          # IsTerminal L20-L30 (char-device heuristic — to replace)
internal/ui/output_test.go     # TestIsTerminal_Pipe L269 (keep); + TestIsTerminal_DevNull (new)
internal/provider/procgroup_unix.go     # //go:build !windows platform-split precedent
internal/provider/procgroup_windows.go  # //go:build windows; kernel32 NewLazyDLL idiom (to mirror)
internal/signal/signal_unix.go          # //go:build !windows precedent (2nd example)
internal/signal/signal_windows.go       # //go:build windows precedent (2nd example)
# callers (NO edits — all auto-benefit):
internal/cmd/config_init_interactive.go:20   # interactiveStdinIsTTY → IsTerminal(os.Stdin)
internal/integrate/protocol.go:251            # DefaultConfirm → IsTerminal(os.Stdin)
internal/cmd/hookexec.go:130                  # ResolveColor → IsTerminal(os.Stdout)
internal/cmd/default_action.go:46             # ResolveColor → IsTerminal(os.Stdout)
```

### Desired Codebase tree with files to be added/changed

```bash
internal/ui/output.go          # MODIFY: IsTerminal body → return isTerminalFd(f.Fd()); update doc comment
internal/ui/isatty_linux.go    # NEW (//go:build linux):     const ioctlReadTermios + isTerminalFd (ioctl)
internal/ui/isatty_darwin.go   # NEW (//go:build darwin):    const ioctlReadTermios + isTerminalFd (ioctl)
internal/ui/isatty_windows.go  # NEW (//go:build windows):   procGetConsoleMode + isTerminalFd (GetConsoleMode)
internal/ui/isatty_other.go    # NEW (//go:build !linux && !darwin && !windows): isTerminalFd (char-device fallback)
internal/ui/output_test.go     # MODIFY: + import "runtime"; + TestIsTerminal_DevNull
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (deps): the project is DELIBERATELY dependency-free. go.mod has NO golang.org/x/term and NO
// golang.org/x/sys. Do NOT `go get` anything. Use ONLY stdlib `syscall` + `unsafe`. This is the SAME seam
// procgroup_*.go and signal_*.go already use. go.mod/go.sum MUST stay byte-unchanged (verify with git diff).

// CRITICAL (build tags): every GOOS must satisfy EXACTLY ONE of the 4 build constraints, or the package
// won't compile (either "undefined: isTerminalFd" or "redeclared: isTerminalFd"). The 4 tags are:
//   linux | darwin | windows | !(linux || darwin || windows)
// Together they are a partition of all GOOS values. The _other fallback (//go:build !linux && !darwin && !windows)
// is the NEVER-BREAK-THE-BUILD safety net: an untested GOOS (freebsd, netbsd, openbsd, solaris, ...) still compiles.

// CRITICAL (imports per file): the unix files (linux, darwin) need BOTH "syscall" AND "unsafe"
// (unsafe.Pointer(&buf) is the standard Go ioctl pattern — used by golang.org/x/term internally).
// The windows file needs "syscall" AND "unsafe" (unsafe.Pointer(&mode) for the LPDWORD out-param).
// The _other file needs "os" (for os.File + os.ModeCharDevice) — NOT "syscall" or "unsafe".

// GOTCHA (syscall const availability): syscall.TCGETS exists ONLY on linux; syscall.TIOCGETA ONLY on darwin/freebsd.
// That is WHY we split linux and darwin into separate files (a const cannot be conditionally set in one file).
// Verified: `GOOS=linux go doc syscall.TCGETS` and `GOOS=darwin go doc syscall.TIOCGETA` both succeed.
// syscall.SYS_IOCTL exists on BOTH linux and darwin (verified). It does NOT exist on windows — but the windows
// file doesn't reference it (uses GetConsoleMode instead), so cross-compilation is clean.

// GOTCHA ([128]byte vs syscall.Termios): use a raw [128]byte buffer, NOT syscall.Termios. We only test whether
// the ioctl SUCCEEDS (errno == 0); we never read the returned termios data. A raw buffer is layout-agnostic
// across BSD variants (Termios struct field order/size differs). 128 bytes is larger than any termios on
// linux/darwin/freebsd. This is the pattern x/term uses internally (it uses Termios only because it reads fields).

// GOTCHA (errno check): syscall.Syscall6 returns (_, _, errno). Return `errno == 0` (0 == success). Do NOT check
// the first return value (it's the raw syscall return; meaningless for ioctl). On a non-terminal fd (/dev/null,
// pipe, file), the ioctl fails with ENOTTY (errno 25 on linux, 25 on darwin) → errno != 0 → false. Correct.

// GOTCHA (output.go unused import): after replacing IsTerminal's body, `os` may appear unused — but it is NOT:
// New() references os.Stdout and os.Stderr. noColorEnvSet uses os.LookupEnv. Leave the imports as-is. If `go vet`
// flags an unused import, the fix is to KEEP `os` (it's used elsewhere in the file) — do not remove it.

// GOTCHA (signature freeze): do NOT change `func IsTerminal(f *os.File) bool`. Four callers depend on it:
// config_init_interactive.go:20, integrate/protocol.go:251, hookexec.go:130, default_action.go:46. f.Fd()
// returns uintptr — that is the isTerminalFd arg type. Do NOT add a context param, an error return, or generics.

// GOTCHA (Windows /dev/null): /dev/null is a Unix path. On Windows it's NUL. The TestIsTerminal_DevNull test
// MUST skip on windows (runtime.GOOS == "windows"). The Windows GetConsoleMode path is validated separately
// on a Windows CI runner (the contract acknowledges this). Do NOT try to make the /dev/null test cross-platform.
```

## Implementation Blueprint

### Data models and structure

None — no structs, no config, no API surface. The only "data" is the `[128]byte` ioctl buffer (a stack
local, not a type declaration) and the `uint32` Windows console-mode out-param (also a stack local).
`isTerminalFd` is an unexported package-level function with ONE definition per GOOS (selected by build tags).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/ui/isatty_linux.go
  - BUILD TAG: `//go:build linux` (MUST be line 1, blank line before `package ui`).
  - PACKAGE: `package ui`.
  - IMPORTS: `syscall`, `unsafe`.
  - DEFINE: `const ioctlReadTermios = syscall.TCGETS` (linux read-termios ioctl constant).
  - DEFINE: `func isTerminalFd(fd uintptr) bool` — allocate `var buf [128]byte`, call
    `syscall.Syscall6(syscall.SYS_IOCTL, fd, uintptr(ioctlReadTermios), uintptr(unsafe.Pointer(&buf)), 0, 0, 0)`,
    capture the errno (3rd return), return `errno == 0`.
  - DOC: one comment explaining the ioctl probe (true isatty; /dev/null fails with ENOTTY).
  - FOLLOW pattern: the ioctl + unsafe.Pointer idiom is standard Go (golang.org/x/term uses it internally).

Task 2: CREATE internal/ui/isatty_darwin.go
  - BUILD TAG: `//go:build darwin` (line 1).
  - PACKAGE: `package ui`.
  - IMPORTS: `syscall`, `unsafe`.
  - DEFINE: `const ioctlReadTermios = syscall.TIOCGETA` (darwin/macOS read-termios ioctl constant).
  - DEFINE: `func isTerminalFd(fd uintptr) bool` — IDENTICAL body to Task 1 (the only difference is the const).
  - NOTE: the body is duplicated intentionally (a const can't be conditionally set in one file); this matches
    the procgroup pattern where unix/windows each carry their own setupProcessGroup.

Task 3: CREATE internal/ui/isatty_windows.go
  - BUILD TAG: `//go:build windows` (line 1).
  - PACKAGE: `package ui`.
  - IMPORTS: `syscall`, `unsafe`.
  - DEFINE: `var procGetConsoleMode = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleMode")`
    (MIRROR procgroup_windows.go L15's procGenerateConsoleCtrlEvent lazy-resolution idiom EXACTLY).
  - DEFINE: `func isTerminalFd(fd uintptr) bool` — allocate `var mode uint32`, call
    `procGetConsoleMode.Call(fd, uintptr(unsafe.Pointer(&mode)))`, capture r1 (1st return),
    return `r1 != 0` (GetConsoleMode returns nonzero/TRUE on a console handle, 0 otherwise).
  - DOC: note that NUL is not a console handle → returns false (the Windows analog of the /dev/null fix).

Task 4: CREATE internal/ui/isatty_other.go
  - BUILD TAG: `//go:build !linux && !darwin && !windows` (line 1) — the NEVER-BREAK-THE-BUILD fallback.
  - PACKAGE: `package ui`.
  - IMPORTS: `os`.
  - DEFINE: `func isTerminalFd(fd uintptr) bool` — the OLD char-device heuristic, adapted to take fd:
    open `f := os.NewFile(fd, "")` (wraps the raw fd as an *os.File), call `f.Stat()`, on error return false,
    else return `(stat.Mode() & os.ModeCharDevice) != 0`. (os.NewFile does NOT take ownership/close the fd here.)
  - DOC: comment that this is the safe fallback for untested GOOS (returns the legacy heuristic so the build
    never breaks); real terminals on these platforms may be misdetected as non-TTY (a known limitation).

Task 5: MODIFY internal/ui/output.go — replace IsTerminal body
  - LOCATE: IsTerminal at ~L20–L30 (the `stat, err := f.Stat(); ... return (stat.Mode() & os.ModeCharDevice) != 0` body).
  - REPLACE the body with: `return isTerminalFd(f.Fd())`.
  - UPDATE the doc comment: remove the "NOT a true isatty ioctl" note; explain it now delegates to the
    platform-specific isTerminalFd (isatty_linux.go / isatty_darwin.go / isatty_windows.go / isatty_other.go),
    that it returns false for /dev/null/pipes/files/redirects, and that --no-color/NO_COLOR remain the
    authoritative overrides. Keep the "stat-error → false" semantics note (now handled inside the platform files).
  - VERIFY: `os` import is still used (New() uses os.Stdout/os.Stderr; noColorEnvSet uses os.LookupEnv) — do NOT remove it.

Task 6: MODIFY internal/ui/output_test.go — add TestIsTerminal_DevNull + runtime import
  - ADD import: `"runtime"` (for the GOOS skip guard).
  - ADD `func TestIsTerminal_DevNull(t *testing.T)` (copy-ready source in the file: internal/ui/output_test.go
    pattern block above): skip on windows; os.Open("/dev/null"); assert !IsTerminal(f); os.Stat to PROVE
    /dev/null IS a char device (belt-and-suspenders — guards the regression premise).
  - DO NOT modify TestIsTerminal_Pipe (L269) — it stays green (pipes fail ioctl too).

Task 7: VERIFY — format, build (all platforms), vet, test
  - gofmt -w internal/ui/*.go
  - go build ./...
  - cross-compile matrix: for goos in linux darwin windows; for goarch in amd64 arm64:
        GOOS=$goos GOARCH=$goarch CGO_ENABLED=0 go build ./...
  - compile-check the fallback: GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build ./internal/ui/...
  - go vet ./internal/ui/...
  - go test ./internal/ui/... -run 'IsTerminal' -v
  - go test ./...
  - git diff --stat go.mod go.sum   # MUST be empty (no new deps)
```

### Implementation Patterns & Key Details

```go
// === Task 1/2: isatty_linux.go (and isatty_darwin.go with TIOCGETA) ===
//go:build linux
// (darwin file uses: //go:build darwin  and  const ioctlReadTermios = syscall.TIOCGETA)

package ui

import (
	"syscall"
	"unsafe"
)

// ioctlReadTermios is the read-termios ioctl request: TCGETS on Linux, TIOCGETA on Darwin/macOS.
// It succeeds (errno == 0) ONLY on a real terminal/pty; /dev/null, pipes, files, and redirects fail
// with ENOTTY. This is the same probe golang.org/x/term/unix uses (stdlib-only here — no x/sys dep).
const ioctlReadTermios = syscall.TCGETS // darwin: syscall.TIOCGETA

// isTerminalFd reports whether fd is a real terminal/pty via an ioctl probe. fd comes from os.File.Fd().
// Uses a raw [128]byte buffer (not syscall.Termios) because we only test errno==0, never read the data —
// a raw buffer is layout-agnostic across BSD variants (Termios field order/size differs). 128 bytes is
// larger than any termios struct on linux/darwin/freebsd.
func isTerminalFd(fd uintptr) bool {
	var buf [128]byte
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL, fd,
		uintptr(ioctlReadTermios),
		uintptr(unsafe.Pointer(&buf)),
		0, 0, 0,
	)
	return errno == 0
}

// === Task 3: isatty_windows.go ===
//go:build windows

package ui

import (
	"syscall"
	"unsafe"
)

// procGetConsoleMode resolves kernel32!GetConsoleMode lazily (stdlib-only — no golang.org/x/sys dependency,
// matching procgroup_windows.go's procGenerateConsoleCtrlEvent idiom). Resolved on first Call; kernel32 is
// always present on Windows.
var procGetConsoleMode = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleMode")

// isTerminalFd reports whether fd is a real console handle via GetConsoleMode. It returns nonzero (TRUE) on
// a console handle, 0 otherwise (e.g. NUL, a pipe, a file). fd comes from os.File.Fd() (a syscall.Handle).
func isTerminalFd(fd uintptr) bool {
	var mode uint32
	r1, _, _ := procGetConsoleMode.Call(fd, uintptr(unsafe.Pointer(&mode)))
	return r1 != 0
}

// === Task 4: isatty_other.go (fallback for untested GOOS — keeps the build green) ===
//go:build !linux && !darwin && !windows

package ui

import "os"

// isTerminalFd is the safe fallback for GOOS values outside {linux, darwin, windows}: it falls back to the
// legacy char-device heuristic so the build NEVER breaks on an untested platform. This means /dev/null may
// be misdetected as a terminal on these platforms (a known limitation); the supported targets use the true
// ioctl/GetConsoleMode probe. See isatty_linux.go / isatty_darwin.go / isatty_windows.go.
func isTerminalFd(fd uintptr) bool {
	f := os.NewFile(fd, "") // wrap the raw fd as an *os.File (does not take ownership / close)
	if f == nil {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// === Task 5: output.go IsTerminal (the one-liner) ===
// IsTerminal reports whether f is a real terminal/pty (true isatty probe). Returns false for /dev/null,
// pipes, files, and redirects — delegating to the platform-specific isTerminalFd:
//   - linux:   ioctl(TCGETS) succeeds iff terminal
//   - darwin:  ioctl(TIOCGETA) succeeds iff terminal
//   - windows: GetConsoleMode returns nonzero iff console handle
//   - other:   legacy char-device heuristic (safe fallback; see isatty_other.go)
// stat/ioctl errors → false (treat as non-TTY → the safe default). --no-color / NO_COLOR remain the
// authoritative overrides (see ResolveColor). Signature is stable; all callers (config init --interactive,
// integrate DefaultConfirm, hook exec / default-action color resolution) benefit automatically.
func IsTerminal(f *os.File) bool {
	return isTerminalFd(f.Fd())
}

// === Task 6: TestIsTerminal_DevNull (new test in output_test.go) ===
// Place after TestIsTerminal_Pipe (L269). Add "runtime" to the import block.
func TestIsTerminal_DevNull(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("/dev/null is a Unix path; Windows uses NUL and is exercised on a Windows CI runner")
	}
	f, err := os.Open("/dev/null")
	if err != nil {
		t.Fatalf("os.Open(/dev/null): %v", err)
	}
	defer f.Close()

	if IsTerminal(f) {
		t.Error("IsTerminal(/dev/null) = true, want false (true isatty probe must reject a char device that is not a tty)")
	}

	// Belt-and-suspenders: PROVE /dev/null IS a char device — i.e. the OLD heuristic would have misfired.
	// If this ever fails, the regression premise changed on this OS; investigate rather than weaken the test.
	st, err := os.Stat("/dev/null")
	if err != nil {
		t.Fatalf("os.Stat(/dev/null): %v", err)
	}
	if (st.Mode() & os.ModeCharDevice) == 0 {
		t.Error("/dev/null is unexpectedly NOT a char device on this OS — the Issue-4 regression premise is invalid")
	}
}
```

### Integration Points

```yaml
PUBLIC API: NONE changed. `func IsTerminal(f *os.File) bool` signature is FROZEN. The 4 callers compile
  unmodified and benefit automatically:
    - internal/cmd/config_init_interactive.go:20  (interactiveStdinIsTTY → IsTerminal(os.Stdin))
    - internal/integrate/protocol.go:251          (DefaultConfirm non-interactive auto-decline gate)
    - internal/cmd/hookexec.go:130                (ResolveColor → IsTerminal(os.Stdout))
    - internal/cmd/default_action.go:46           (ResolveColor → IsTerminal(os.Stdout))
  Do NOT edit any of these files — they already call IsTerminal correctly; the fix is entirely inside internal/ui/.

DEPENDENCIES: NONE added. go.mod / go.sum MUST be byte-unchanged. The fix uses ONLY stdlib `syscall` and
  `unsafe` — the same seams procgroup_*.go and signal_*.go already use. Do NOT run `go get golang.org/x/term`
  or `go get golang.org/x/sys`.

BUILD TAGS: 4 new files partition ALL GOOS values:
    linux  → isatty_linux.go    (ioctl TCGETS)
    darwin → isatty_darwin.go   (ioctl TIOCGETA)
    windows→ isatty_windows.go  (GetConsoleMode)
    other  → isatty_other.go    (char-device heuristic fallback)
  Every GOOS satisfies EXACTLY ONE constraint → exactly one isTerminalFd definition → the package always compiles.

CONFIG / DOCS: NONE. No config keys, no help text, no user-facing docs. IsTerminal's behavior is an internal
  implementation detail; the PRD's FR-L3 and FR-I3c describe the EXTERNAL behavior (TTY gate, auto-decline)
  that this fix makes WORK CORRECTLY for the /dev/null case. (If a later docs task references isatty, it's P1.M1.T5.)
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/ui/*.go
git diff --stat                            # confirm ONLY internal/ui/ files changed (5 files: 4 new + output.go + output_test.go)
go build ./...
# Expected: zero errors. The 4 build tags partition GOOS, so exactly one isTerminalFd is compiled per GOOS.
```

### Level 2: Cross-Platform Build (the CRITICAL gate — all supported targets)

```bash
# The full support matrix (PRD §20.4): linux/darwin/windows × amd64/arm64, CGO_ENABLED=0.
for goos in linux darwin windows; do
  for goarch in amd64 arm64; do
    echo -n "$goos/$goarch: "
    GOOS=$goos GOARCH=$goarch CGO_ENABLED=0 go build ./... && echo "ok" || echo "FAIL"
  done
done
# Expected: all 6 = ok. If windows fails, check isatty_windows.go's GetConsoleMode Call signature.
# If linux/darwin fail, check the ioctl const (TCGETS vs TIOCGETA) and the unsafe.Pointer import.

# Fallback safety net: the _other file must compile on an untested GOOS (never break the build).
GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build ./internal/ui/... && echo "freebsd fallback ok" || echo "FAIL"

go vet ./internal/ui/...
# Expected: clean.
```

### Level 3: Unit Tests (the contract)

```bash
go test ./internal/ui/... -run 'IsTerminal' -v
# REQUIRED outcomes:
#  TestIsTerminal_Pipe:     PASS (existing, unchanged — pipes fail ioctl AND aren't char devices → false).
#  TestIsTerminal_DevNull:  PASS (NEW — /dev/null is a char device BUT ioctl fails with ENOTTY → false;
#                           the os.Stat assertion confirms it IS a char device, proving the old heuristic was the bug).

go test ./...
# Expected: all green. No caller test breaks (IsTerminal signature unchanged; callers unedited).

# Dep-free invariant (CRITICAL):
git diff --stat go.mod go.sum
# Expected: (no output) — zero changes to dependency files.
```

### Level 4: Integration (manual repro from the issue)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach

# Issue-4 repro #1: config init --interactive < /dev/null must now hit the FR-L3 TTY gate (clean message).
# (Run in a dir with no stagecoach config, or use --force; set STAGECOACH_HOME to a temp dir to avoid clobbering.)
export STAGECOACH_HOME=/tmp/sh-iss4
rm -rf "$STAGECOACH_HOME"; mkdir -p "$STAGECOACH_HOME"
/tmp/stagecoach config init --interactive --force < /dev/null 2>/tmp/iss4.err; echo "exit=$?"
# Expected: exit 1 AND stderr contains the "requires a terminal" message; stderr does NOT contain
#           "unexpected end of input". (Before the fix: exit 1 with "unexpected end of input".)
grep -i "terminal" /tmp/iss4.err && echo "FR-L3 gate fired correctly"

# Issue-4 repro #2: IsTerminal on /dev/null directly (the unit-level proof — run on Unix).
cat > /tmp/iss4_probe.go <<'EOF'
package main
import ("fmt"; "os"; "github.com/dustin/stagecoach/internal/ui")
func main() {
	f, _ := os.Open("/dev/null"); defer f.Close()
	fmt.Println("IsTerminal(/dev/null) =", ui.IsTerminal(f), "(want false)")
}
EOF
go run /tmp/iss4_probe.go
# Expected: IsTerminal(/dev/null) = false (want false)

# Note: the Windows GetConsoleMode path is validated on a Windows CI runner (the /dev/null test skips on windows).
```

## Final Validation Checklist

### Technical
- [ ] `gofmt -d internal/ui/*.go` shows no diff after formatting.
- [ ] `go build ./...` clean; the 6-target cross-compile matrix (linux/darwin/windows × amd64/arm64, CGO_ENABLED=0) all `ok`.
- [ ] `GOOS=freebsd ... go build ./internal/ui/...` ok (the `_other` fallback compiles on an untested GOOS).
- [ ] `go vet ./internal/ui/...` clean.
- [ ] `go test ./...` green (incl. the new `TestIsTerminal_DevNull` and the unchanged `TestIsTerminal_Pipe`).
- [ ] `git diff --stat go.mod go.sum` is EMPTY (no new dependencies — dep-free invariant preserved).

### Feature
- [ ] `IsTerminal(/dev/null)` returns `false` (Unix) — verified by TestIsTerminal_DevNull.
- [ ] `IsTerminal(os.Pipe())` reader/writer return `false` — verified by TestIsTerminal_Pipe (unchanged).
- [ ] `config init --interactive < /dev/null` now prints the FR-L3 "requires a terminal" message (not "unexpected end of input").
- [ ] Real terminals/ptys still return `true` (ioctl succeeds with errno==0; GetConsoleMode returns nonzero).

### Code Quality
- [ ] Only `internal/ui/` files changed: 4 NEW (`isatty_linux.go`, `isatty_darwin.go`, `isatty_windows.go`, `isatty_other.go`) + `output.go` (body) + `output_test.go` (1 test + import). NO caller files edited.
- [ ] `func IsTerminal(f *os.File) bool` signature UNCHANGED (4 callers compile unmodified).
- [ ] Follows the established platform-split pattern (procgroup_*.go / signal_*.go) — stdlib `syscall` + `unsafe` only.
- [ ] The Windows file mirrors procgroup_windows.go's `syscall.NewLazyDLL("kernel32.dll").NewProc(...)` idiom exactly.
- [ ] Doc comments updated: output.go drops "NOT a true isatty ioctl"; each platform file explains its probe.

### Scope Boundaries (do NOT cross)
- [ ] Do NOT edit any caller of IsTerminal (config_init_interactive.go, protocol.go, hookexec.go, default_action.go) — they auto-benefit.
- [ ] Do NOT add `golang.org/x/term` or `golang.org/x/sys` (dep-free invariant; go.mod/go.sum byte-unchanged).
- [ ] Do NOT change the `IsTerminal` signature (no context param, no error return, no generics).
- [ ] Do NOT edit `docs/` or any config/loader (this is an internal-implementation fix; docs sync is P1.M1.T5).
- [ ] Do NOT weaken or remove `TestIsTerminal_Pipe` (it stays green and guards the pipe case).
- [ ] Do NOT use `syscall.Termios` (use `[128]byte` — layout-agnostic across BSD variants).

---

## Anti-Patterns to Avoid
- ❌ Don't add `golang.org/x/term` or `golang.org/x/sys` — the project is deliberately dep-free; use stdlib `syscall`+`unsafe` (same as procgroup_*.go / signal_*.go).
- ❌ Don't collapse linux+darwin into one `//go:build !windows` file — the ioctl const (TCGETS vs TIOCGETA) differs and can't be conditionally set; split into 2 files.
- ❌ Don't omit `isatty_other.go` — without it, an untested GOOS (freebsd/netbsd/...) breaks the build ("undefined: isTerminalFd"). The 4 build tags MUST partition all GOOS values.
- ❌ Don't check the syscall's first return value — ioctl's meaningful result is the errno (3rd return of Syscall6); return `errno == 0`.
- ❌ Don't use `syscall.Termios` — struct layout differs across BSD variants; use a raw `[128]byte` buffer (we only test errno==0, never read the data).
- ❌ Don't change the `IsTerminal` signature — 4 callers depend on `func IsTerminal(f *os.File) bool`.
- ❌ Don't remove the `os` import from output.go — it's still used by New() (os.Stdout/os.Stderr) and noColorEnvSet (os.LookupEnv).
- ❌ Don't edit the 4 caller files — they already call IsTerminal correctly; the fix is 100% inside internal/ui/.
- ❌ Don't make TestIsTerminal_DevNull cross-platform — skip on `runtime.GOOS == "windows"` (/dev/null is Unix-only; Windows is tested on a Windows CI runner).
- ❌ Don't touch TestIsTerminal_Pipe — it stays green unchanged (pipes fail the ioctl probe too).

---

## Confidence Score

**9.5/10** for one-pass success. Every detail is pinned: the exact bug site (output.go L20–L30), the 4 new
files with COMPLETE copy-ready Go source (build tags, imports, consts, function bodies), the verified syscall
constants (TCGETS/TIOCGETA/SYS_IOCTL all confirmed via `go doc`; GetConsoleMode semantics from MS docs), the
exhaustive-build-tag partition proof (no GOOS left without exactly one isTerminalFd), the frozen-signature
contract (4 callers auto-benefit, zero caller edits), the established procgroup_*/signal_* platform-split
precedent (Windows kernel32 lazy-resolution idiom copied verbatim), the baseline cross-compile green state
(linux/darwin/windows all build before the change), the dep-free invariant (go.mod/go.sum byte-unchanged), and
two copy-ready tests (the new DevNull test with the runtime.GOOS skip + the os.Stat premise guard; the existing
Pipe test stays untouched). The only residual risk is a Windows GetConsoleMode signature subtlety (mitigated by
mirroring procgroup_windows.go's exact `.Call(...)` idiom and the cross-compile gate in Level 2) and the
`os.NewFile` ownership semantics in the `_other` fallback (mitigated: os.NewFile does not close the caller's fd;
the test matrix doesn't exercise `_other` at runtime but the compile-check confirms it builds). No external deps,
no API change, no config change.
