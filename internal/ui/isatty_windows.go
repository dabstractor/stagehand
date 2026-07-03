//go:build windows

package ui

import (
	"syscall"
	"unsafe"
)

// procGetConsoleMode resolves kernel32!GetConsoleMode lazily (stdlib-only — no golang.org/x/sys
// dependency, matching procgroup_windows.go's procGenerateConsoleCtrlEvent idiom). Resolved on
// first Call; kernel32 is always present on Windows.
var procGetConsoleMode = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleMode")

// isTerminalFd reports whether fd is a real console handle via GetConsoleMode. It returns nonzero
// (TRUE) on a console handle, 0 otherwise (e.g. NUL, a pipe, a file). fd comes from os.File.Fd()
// (a syscall.Handle).
func isTerminalFd(fd uintptr) bool {
	var mode uint32
	r1, _, _ := procGetConsoleMode.Call(fd, uintptr(unsafe.Pointer(&mode)))
	return r1 != 0
}
