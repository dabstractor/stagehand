//go:build linux

package watchdog

import (
	"runtime"
	"syscall"
)

// armPdeathsig best-effort arms the kernel to deliver sig to THIS process when its parent dies
// (PR_SET_PDEATHSIG). It is the Linux fast path for §9.27 FR-K1/K2: when it works, the kernel sends
// a real SIGTERM with no 1s-poll latency, and SIGTERM is already in the caught set so it flows
// through signal.Notify → handle() naturally.
//
// prctl is PER-THREAD, so the goroutine is pinned with runtime.LockOSThread() for the syscall (the
// runtime would otherwise migrate it to another thread). The deferred Unlock means the runtime MAY
// later retire this thread → the setting can be lost; the getppid poll (arm_unix.go) covers that.
//
// Best-effort: returns the errno (the caller ignores it); never fatal. arg2 is the signal VALUE
// (uintptr(sig)), NOT a pointer — this matches Go's own exec_linux.go:550
// (RawSyscall6(SYS_PRCTL, PR_SET_PDEATHSIG, uintptr(sys.Pdeathsig), 0,0,0,0)).
func armPdeathsig(sig syscall.Signal) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	_, _, errno := syscall.Syscall6(
		syscall.SYS_PRCTL,
		uintptr(syscall.PR_SET_PDEATHSIG), // == 0x1 (exported on every Linux arch); the literal uintptr(1) is equivalent.
		uintptr(sig),                      // VALUE, not a pointer (matches Go's exec_linux.go:550).
		0, 0, 0, 0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}
