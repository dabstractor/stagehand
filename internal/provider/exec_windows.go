//go:build windows
// +build windows

package provider

import (
	"os"
	"os/exec"
)

// applyProcessGroup is a no-op on Windows: there is no POSIX process-group /
// Setpgid concept. A fuller future Windows port would use Job Objects +
// CREATE_NEW_PROCESS_GROUP (and taskkill /T), but that is out of scope for the
// v1.0 best-effort cross-compile fix — this stub merely lets the provider
// package build for GOOS=windows so the goreleaser §21.2 snapshot gate passes.
// The behavior it gates (process-group isolation, §18.4) is documented as
// Unix-oriented in executor.go's package doc.
func applyProcessGroup(cmd *exec.Cmd) {}

// terminateGroup best-effort kills the single process on Windows. There are no
// POSIX signal groups here, so only the leader is killed (children are NOT
// reaped) and force is ignored — matching the v1.0 best-effort Windows intent
// documented in executor.go.
func terminateGroup(pgid int, force bool) {
	if proc, err := os.FindProcess(pgid); err == nil {
		_ = proc.Kill()
	}
}
