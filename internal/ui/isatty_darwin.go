//go:build darwin

package ui

import (
	"syscall"
	"unsafe"
)

// ioctlReadTermios is the read-termios ioctl request for Darwin/macOS. It succeeds (errno == 0)
// ONLY on a real terminal/pty; /dev/null, pipes, files, and redirects fail with ENOTTY.
const ioctlReadTermios = syscall.TIOCGETA

// isTerminalFd reports whether fd is a real terminal/pty via an ioctl probe. fd comes from
// os.File.Fd(). Uses a raw [128]byte buffer (not syscall.Termios) because we only test errno==0,
// never read the data — a raw buffer is layout-agnostic across BSD variants. 128 bytes is larger
// than any termios struct on linux/darwin/freebsd.
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
