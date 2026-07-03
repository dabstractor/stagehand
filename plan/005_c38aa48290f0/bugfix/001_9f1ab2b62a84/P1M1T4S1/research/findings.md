# Research Findings — P1.M1.T4.S1 (Replace char-device heuristic with platform-specific isatty ioctl probe)

## 1. Current State (verified by reading source)

### IsTerminal — the function under fix
**File**: `internal/ui/output.go` L20–L30
```go
// IsTerminal reports whether f is a terminal (character device). Stdlib-only TTY heuristic: a real
// terminal/pty is a char device; a pipe, file, or redirect is not. ... NOT a true isatty ioctl; ...
func IsTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
```
**Signature (FROZEN)**: `func IsTerminal(f *os.File) bool` — 4 callers depend on it; must NOT change.
**Bug**: `/dev/null` IS a char device → returns `true`. Real terminals/ptys are also char devices, so the heuristic can't distinguish them.

### Callers of IsTerminal (grep-verified, 4 sites — all benefit automatically, no edits needed)
| File | Line | Call | Effect of fix |
|------|------|------|---------------|
| `internal/cmd/config_init_interactive.go` | 20 | `interactiveStdinIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }` | FR-L3 TTY gate now correctly fires on `/dev/null` stdin → clean "requires a terminal" message |
| `internal/integrate/protocol.go` | 251 | `if !ui.IsTerminal(os.Stdin) { ... }` (DefaultConfirm) | FR-I3c auto-decline path now taken on `/dev/null` |
| `internal/cmd/hookexec.go` | 130 | `ui.ResolveColor(cfg.NoColor, ui.IsTerminal(os.Stdout))` | color gating (cosmetic — stdout rarely /dev/null) |
| `internal/cmd/default_action.go` | 46 | `ui.ResolveColor(cfg.NoColor, ui.IsTerminal(os.Stdout))` | color gating (cosmetic) |

### Existing test that MUST still pass
**File**: `internal/ui/output_test.go` L269 `TestIsTerminal_Pipe`
- Opens `r, w, err := os.Pipe()`, asserts `IsTerminal(r)==false` AND `IsTerminal(w)==false`.
- A pipe is NOT a char device, so it returns false under BOTH the old and new impl. The ioctl probe also returns errno!=0 on a pipe. → test stays green.
- **Imports currently in output_test.go**: `bytes`, `fmt`, `os`, `strings`, `testing`. The new `/dev/null` test needs `runtime` added for the `runtime.GOOS == "windows"` skip guard.

## 2. Platform-file pattern to follow (verified by reading)

The project has TWO established platform-split precedents, both stdlib-only (`syscall`, NO golang.org/x/sys):

### Precedent A — coarse `!windows` / `windows` split
- `internal/provider/procgroup_unix.go` (`//go:build !windows`) + `procgroup_windows.go` (`//go:build windows`)
- `internal/signal/signal_unix.go` (`//go:build !windows`) + `signal_windows.go` (`//go:build windows`)
- Each platform file defines the SAME function signature; only the body differs.

### Precedent B — Windows kernel32 lazy resolution (the exact idiom for our Windows file)
`internal/provider/procgroup_windows.go` L15:
```go
var procGenerateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")
// later:
r1, _, err := procGenerateConsoleCtrlEvent.Call(uintptr(...), uintptr(...))
if r1 == 0 { return err }  // 0 = failure
return nil
```
**We mirror this EXACTLY** for `GetConsoleMode`:
```go
var procGetConsoleMode = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleMode")
```

### Why we need FINER than `!windows` (linux vs darwin split)
The ioctl constant differs by Unix flavor:
- **Linux**: `syscall.TCGETS` (verified: `GOOS=linux go doc syscall.TCGETS` → exists)
- **Darwin/BSD**: `syscall.TIOCGETA` (verified: `GOOS=darwin go doc syscall.TIOCGETA` → exists)
Both share `syscall.SYS_IOCTL` (verified on both). A single `!windows` file CANNOT conditionally define a `const` per-OS, so we split into `isatty_linux.go` + `isatty_darwin.go`, each carrying its own const + its own (identical-body) `isTerminalFd`. This matches the contract (a)+(b)+(c)+(d) each defining `isTerminalFd`.

## 3. Build-tag strategy (4 files) — exhaustive GOOS coverage

| File | Build tag | Defines | Body |
|------|-----------|---------|------|
| `internal/ui/isatty_linux.go` | `//go:build linux` | `const ioctlReadTermios = syscall.TCGETS` + `func isTerminalFd(fd uintptr) bool` | ioctl probe; `errno==0` → true |
| `internal/ui/isatty_darwin.go` | `//go:build darwin` | `const ioctlReadTermios = syscall.TIOCGETA` + `func isTerminalFd(fd uintptr) bool` | ioctl probe; `errno==0` → true |
| `internal/ui/isatty_windows.go` | `//go:build windows` | `var procGetConsoleMode = ...NewProc("GetConsoleMode")` + `func isTerminalFd(fd uintptr) bool` | GetConsoleMode; `r1!=0` → true |
| `internal/ui/isatty_other.go` | `//go:build !linux && !darwin && !windows` | `func isTerminalFd(fd uintptr) bool` | OLD char-device heuristic (safe fallback) |

**Exhaustiveness proof**: every GOOS satisfies EXACTLY ONE of {linux, darwin, windows, !(linux||darwin||windows)}. No GOOS is left without exactly one `isTerminalFd` definition → the package always compiles (no "missing function" / "duplicate function" build error on any untested platform). The `isatty_other.go` fallback guarantees a never-broken-build safety net (the contract's explicit requirement (d)).

## 4. The ioctl probe (Unix) — why `[128]byte`, not `syscall.Termios`

`syscall.Termios` EXISTS on linux (`go doc syscall.Termios` confirms), but its struct layout DIFFERS across BSD variants (field order/size vary). A raw `[128]byte` buffer is layout-agnostic — we only care whether the ioctl SUCCEEDS (errno==0), never read the returned termios data. This is the SAME pattern `golang.org/x/term` uses internally (it uses `syscall.Termios` only because it reads fields; we don't). The `[128]byte` is large enough for any termios struct on linux/darwin/freebsd (all < 128 bytes).

```go
func isTerminalFd(fd uintptr) bool {
	var buf [128]byte
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd,
		uintptr(ioctlReadTermios), uintptr(unsafe.Pointer(&buf)), 0, 0, 0)
	return errno == 0
}
```
**Imports required in BOTH unix files**: `"syscall"` AND `"unsafe"` (for `unsafe.Pointer(&buf)`). `unsafe` is stdlib (no dep).

## 5. The GetConsoleMode probe (Windows)

Win32 signature: `BOOL GetConsoleMode(HANDLE hConsoleHandle, LPDWORD lpMode)` — returns **nonzero on a console handle, 0 otherwise**. We pass `fd` (a syscall.Handle from `f.Fd()`) and a `*uint32` to receive the mode; we only inspect the return value, never the mode bits.

```go
func isTerminalFd(fd uintptr) bool {
	var mode uint32
	r1, _, _ := procGetConsoleMode.Call(fd, uintptr(unsafe.Pointer(&mode)))
	return r1 != 0
}
```
**Note**: `/dev/null` does not exist on Windows (it's `NUL`), and NUL is not a console handle, so GetConsoleMode returns 0 → false. The old char-device heuristic wasn't even applicable on Windows (os.ModeCharDevice is a Unix concept). So the Windows fix is "more correct" but lower-impact. **Import**: `"unsafe"` + `"syscall"`.

## 6. Baseline verification (run before writing this file)
```
linux/amd64:  GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build ./... → ok
darwin/amd64: GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build ./... → ok
windows/amd64:GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./... → ok
go vet ./internal/ui/... → clean
go.mod: go 1.22 (//go:build syntax supported since go 1.17); NO golang.org/x/term or golang.org/x/sys.
```

## 7. Test plan (TDD)

| Test | File | Logic | Platform guard |
|------|------|-------|----------------|
| `TestIsTerminal_DevNull` (NEW) | output_test.go | `f,_ := os.Open("/dev/null")`; assert `!IsTerminal(f)`; ALSO assert `(os.Stat mode & os.ModeCharDevice)!=0` to PROVE the old heuristic would've misfired | `if runtime.GOOS == "windows" { t.Skip() }` |
| `TestIsTerminal_Pipe` (EXISTING) | output_test.go L269 | pipe reader/writer → false | none (pipes are cross-platform) |
| `TestIsTerminal_RealStdinConsistency` (NEW, optional) | output_test.go | if os.Stdin is a pipe in CI, `IsTerminal(os.Stdin)` should be false | conceptual, low-value — the DevNull + Pipe tests are the core |

**Why skip on Windows**: `/dev/null` is a Unix path. Windows uses `NUL`. The Windows GetConsoleMode path requires a Windows CI runner to test directly (the contract acknowledges this). The DevNull test is Unix-only.

## 8. External references (conventional isatty implementations)

- **golang.org/x/term `IsTerminal`**: https://pkg.go.dev/golang.org/x/term#IsTerminal — the canonical Go isatty. Uses `unix.IoctlGetTermios(fd, ioctlReadTermios)` which calls `ioctl(fd, TCGETS/TIOCGETA)` — EXACTLY our syscall approach, just wrapped. We can't use it (dep-free constraint) but it validates the ioctl constant choice.
- **golang.org/x/sys/unix `IoctlGetTermios`**: https://pkg.go.dev/golang.org/x/sys/unix#IoctlGetTermios — source of the `ioctlReadTermios` const naming (TCGETS on linux, TIOCGETA on darwin/freebsd). Confirms our const values.
- **Microsoft `GetConsoleMode`**: https://learn.microsoft.com/en-us/windows/console/getconsolemode — "If the handle is not a console handle, the return value is zero." Confirms our Windows logic.
- **Darwin `ioctl(2)` / TIOCGETA**: `man 4 tty` on macOS — TIOCGETA = "get termios struct". Confirms TIOCGETA is the macOS read-termios ioctl.
- **Linux `ioctl(2)` / TCGETS**: `man 3 ioctl_tty` — TCGETS = "get the current serial port settings". Confirms TCGETS is the Linux read-termios ioctl.
