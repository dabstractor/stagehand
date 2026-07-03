//go:build !windows
// +build !windows

package provider

import (
	"os/exec"
	"syscall"
)

// applyProcessGroup sets the child up as a process-group leader (PGID == PID)
// via SysProcAttr.Setpgid, so a single kill(-pgid) fells the agent AND all of
// its children (sub-shells, MCP servers) — PRD §18.4. This is the Unix-only
// half of the executor.go syscall split (the goreleaser §21.2 snapshot gate
// cross-compiles windows, where Setpgid does not exist).
func applyProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateGroup signals the whole process group: SIGTERM for a graceful stop
// (force=false) or SIGKILL for a hard kill (force=true). Negating the pgid
// targets the group rather than just the leader, matching the §18.4 escalation
// (SIGTERM → ~2s grace → SIGKILL the group) byte-for-byte.
func terminateGroup(pgid int, force bool) {
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	_ = syscall.Kill(-pgid, sig)
}
