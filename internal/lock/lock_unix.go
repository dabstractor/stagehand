//go:build !windows

package lock

import (
	"errors"
	"syscall"
)

// flock acquires an exclusive, non-blocking advisory lock on fd (LOCK_EX|LOCK_NB).
// On success the lock is held until fd is closed (auto-released on process death).
// On contention it returns an error wrapping syscall.EWOULDBLOCK.
func flock(fd int) error {
	return syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
}

// isWouldBlock reports whether err indicates the lock is held by another process.
func isWouldBlock(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK)
}
