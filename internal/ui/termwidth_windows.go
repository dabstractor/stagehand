//go:build windows

package ui

import (
	"syscall"
	"unsafe"
)

// procGetConsoleScreenBufferInfo resolves kernel32!GetConsoleScreenBufferInfo lazily (stdlib-only —
// no golang.org/x/sys dependency, matching isatty_windows.go's GetConsoleMode idiom). Fills a
// CONSOLE_SCREEN_BUFFER_INFO for a console handle.
var procGetConsoleScreenBufferInfo = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleScreenBufferInfo")

// coord mirrors COORD {SHORT X, Y} (2× int16). smallRect mirrors SMALL_RECT {SHORT Left, Top, Right,
// Bottom} (4× int16). consoleScreenBufferInfo mirrors CONSOLE_SCREEN_BUFFER_INFO in exact field
// order so the syscall lays the struct out correctly (padding/alignment match Win32).
type coord struct{ X, Y int16 }
type smallRect struct{ Left, Top, Right, Bottom int16 }
type consoleScreenBufferInfo struct {
	dwSize              coord
	dwCursorPosition    coord
	wAttributes         uint16
	srWindow            smallRect
	dwMaximumWindowSize coord
}

// terminalWidthFd reports the VISIBLE column width of the console attached to fd via
// GetConsoleScreenBufferInfo's srWindow (Right - Left + 1 = the viewport the user sees, not the
// larger scrollback buffer), or 0 when fd is not a console handle or the call fails (returns 0).
// fd comes from os.File.Fd() (a syscall.Handle).
func terminalWidthFd(fd uintptr) int {
	var info consoleScreenBufferInfo
	r1, _, _ := procGetConsoleScreenBufferInfo.Call(fd, uintptr(unsafe.Pointer(&info)))
	if r1 == 0 {
		return 0
	}
	w := int(info.srWindow.Right) - int(info.srWindow.Left) + 1
	if w <= 0 {
		return 0
	}
	return w
}
