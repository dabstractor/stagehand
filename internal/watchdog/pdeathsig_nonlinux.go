//go:build !linux

package watchdog

import "syscall"

// armPdeathsig is a no-op on non-Linux (§9.27 FR-K7): darwin/BSDs have no PR_SET_PDEATHSIG
// equivalent in this stdlib form, and Windows has no parent-death concept at all. The getppid poll
// (arm_unix.go) is the detector on these platforms. This stub exists so arm_unix.go's reference to
// armPdeathsig compiles on darwin (the runtime.GOOS=="linux" gate means it is never executed there,
// but the compiler still needs the symbol).
//
// GOTCHA: with `!linux` this file is ALSO compiled on windows, where arm_windows.go never calls
// armPdeathsig → a hypothetical cross-platform lint would flag it as unused (staticcheck U1000).
// All native-linux gates pass here; if CI ever cross-lints windows, change this build tag to
// `//go:build !linux && !windows`.
func armPdeathsig(sig syscall.Signal) error {
	_ = sig
	return nil
}
