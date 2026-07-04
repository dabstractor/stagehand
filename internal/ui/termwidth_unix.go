//go:build linux || darwin

package ui

import (
	"syscall"
	"unsafe"
)

// winsize mirrors the kernel's struct winsize (termios.h): visible row/col count + pixel size. All
// uint16 (8 bytes total) — the shape TIOCGWINSZ fills. Declared once here because the layout is
// identical on Linux and Darwin.
type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// terminalWidthFd reports the column width of the terminal attached to fd via the TIOCGWINSZ ioctl
// (syscall.TIOCGWINSZ resolves correctly on both Linux and Darwin), or 0 when fd is not a terminal
// or the ioctl fails (ENOTTY for pipes/files/redirects). fd comes from os.File.Fd(). Uses the same
// raw Syscall6 ioctl idiom as isTerminalFd in isatty_linux.go / isatty_darwin.go; a zero Col (some
// terminals report it) is treated as "unknown" so callers fall back to a default.
func terminalWidthFd(fd uintptr) int {
	var ws winsize
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL, fd,
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)),
		0, 0, 0,
	)
	if errno != 0 || ws.Col == 0 {
		return 0
	}
	return int(ws.Col)
}
