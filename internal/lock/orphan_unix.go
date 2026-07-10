//go:build !windows

package lock

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// appearsOrphaned is a HEURISTIC that reports whether the holder process (pid)
// APPEARS orphaned — i.e. reparented to init/launchd because its launcher exited
// without killing it (PRD §9.27's "orphaned-but-alive" case, the lock-stays-
// forever hazard FR-K4 diagnoses). It is called by Status ONLY when the holder
// is alive (a dead pid will be reaped anyway, so its orphan status is moot).
//
// The heuristic: ppid == 1 ⇒ reparented to init. CONSERVATIVE INVARIANT — it
// returns false on ANY error or ambiguity (proc gone, ps failure, parse failure):
// orphan detection is a diagnostic HINT feeding the user's kill/rm decision, so
// a false-positive orphan claim (prompting the user to kill a legitimately-
// parented run) is worse than a false negative. The only `true` is ppid == 1.
//
// LIMITATION: under a subreaper (PR_SET_CHILD_SUBREAPER — systemd, systemd-run,
// some shells, containers) an orphan's ppid may be != 1 (it is reparented to the
// subreaper, not init), so this can MISS orphans (false negative); it never
// false-positives a legitimately-parented process in the common case. The
// orphan==true path is proven end-to-end by the E2E harness (P1.M4.T1.S1).
//
// Cross-platform: orphan_windows.go provides an always-false twin (FR-K7 —
// Windows has no init-reparenting analog). Platform dispatch is via runtime.GOOS
// in a single file (not build-tag-per-OS): every import is referenced by ≥1
// compiled function, so the whole file compiles on every Unix target.
func appearsOrphaned(pid int) bool {
	ppid, err := ppidOf(pid)
	if err != nil {
		return false // conservative: don't claim orphan on ambiguity
	}
	return ppid == 1 // reparented to init/launchd (subreapers may have ppid≠1 — limitation)
}

// ppidOf returns the parent pid of pid, dispatching on runtime.GOOS. Linux reads
// /proc/<pid>/status (no fork); everything else (darwin/BSDs) shells out to ps.
func ppidOf(pid int) (int, error) {
	if runtime.GOOS == "linux" {
		return ppidLinux(pid)
	}
	return ppidViaPs(pid)
}

// ppidLinux reads the PPid: field from /proc/<pid>/status. ENOENT (pid gone) or
// any other Open/scan error propagates; appearsOrphaned maps it to false.
func ppidLinux(pid int) (int, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err // ENOENT when pid is gone → appearsOrphaned returns false
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		// strings.Fields splits on whitespace; the colon stays glued to "PPid",
		// so fields[0] == "PPid:" (the WHOLE token — NOT a bare prefix, to avoid
		// matching the "Pid:" line that precedes "PPid:" in /proc/<pid>/status).
		fields := strings.Fields(s.Text())
		if len(fields) >= 2 && fields[0] == "PPid:" {
			return strconv.Atoi(fields[1])
		}
	}
	if err := s.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("orphan: no PPid field for pid %d", pid)
}

// ppidViaPs runs `ps -o ppid= -p <pid>` (the trailing '=' suppresses the header)
// and parses the right-justified number. A non-zero exit (pid missing) returns
// the *exec.ExitError; appearsOrphaned maps it to false.
func ppidViaPs(pid int) (int, error) {
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, err // *exec.ExitError when pid is missing → appearsOrphaned returns false
	}
	// TrimSpace is MANDATORY: ps right-justifies the number (e.g. "     1"),
	// and strconv.Atoi fails on leading whitespace.
	return strconv.Atoi(strings.TrimSpace(string(out)))
}
