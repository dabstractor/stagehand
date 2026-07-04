//go:build !linux && !darwin && !windows

package ui

// terminalWidthFd is the safe fallback for GOOS values outside {linux, darwin, windows}: terminal
// width detection isn't implemented, so return 0 and let callers fall back to a fixed default width.
// Mirrors isatty_other.go's "the build never breaks on an untested platform" stance. The supported
// targets use the real ioctl/console probe (termwidth_unix.go / termwidth_windows.go).
func terminalWidthFd(fd uintptr) int { return 0 }
