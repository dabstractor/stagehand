//go:build !linux && !darwin && !windows

package ui

import "os"

// isTerminalFd is the safe fallback for GOOS values outside {linux, darwin, windows}: it falls
// back to the legacy char-device heuristic so the build NEVER breaks on an untested platform. This
// means /dev/null may be misdetected as a terminal on these platforms (a known limitation); the
// supported targets use the true ioctl/GetConsoleMode probe. See isatty_linux.go /
// isatty_darwin.go / isatty_windows.go.
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
