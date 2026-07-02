// Package git wraps the real host git binary (stdlib os/exec only — NO git
// library, per PRD §22.3: "Stagehand shells out to the real git binary
// (matching commit-pi). go-git is tempting but adds a large dependency ...
// Shelling out is simpler, matches the reference implementation, and
// guarantees identical semantics to the user's git."). The [Git] type is
// bound to one working directory and exposes low-level plumbing methods
// (WriteTree/CommitTree/UpdateRefCAS in M3.T2, StagedDiff in M3.T3,
// CommitCount/RecentMessages/RecentSubjects/HasStagedChanges/AddAll in
// M3.T4) that all funnel through the unexported [Git.run] exec seam.
//
// Every git invocation is built as a []string and run directly via
// exec.Command — NEVER via sh -c / zsh -c (PRD §19: "No shell interpolation.
// Commands are built as []string and run via exec.Command directly ... This
// eliminates the entire class of shell-injection bugs"), and the diff payload
// is delivered over stdin, never interpolated into an argument. This makes
// shell injection structurally impossible across the whole package.
//
// The Git wrapper interface and its method surface are described in
// decisions.md §2 (internal/git.Git, backed by exec.Command("git", ...),
// tested with a temp repo + the real git binary per PRD §20.1 layer 2). This
// file is the first/primary file of package git and OWNS the package doc;
// sibling files (plumbing.go/diff.go/log.go/stage.go, and the white-box test
// helpers in gittestutil_test.go) use a plain "package git" line, mirroring
// how internal/ui/exitcode.go owns the "// Package ui" doc.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Git wraps the real git binary for a single working directory (PRD §22.3:
// shell out to real git, no go-git; decisions.md §2: the internal/git.Git
// wrapper interface backed by exec.Command("git", ...)). It resolves the git
// binary once at construction (see [New]) and funnels every plumbing call
// through the unexported [Git.run] exec helper, which captures stdout/stderr
// and returns a typed *[ExitError] on a non-zero exit.
type Git struct {
	// dir is the working directory passed to every git invocation's cmd.Dir.
	// An empty string means "inherit the stagehand process's current working
	// directory", matching the provider Executor's Dir semantics.
	dir string

	// git is the resolved path to the git binary (the result of
	// exec.LookPath("git") captured in [New]). Because LookPath returns an
	// absolute path (with a path separator), exec.Command uses it directly
	// and does NOT re-search PATH on each [Git.run] call.
	git string
}

// New returns a [Git] bound to dir that shells out to the real git binary
// (PRD §22.3). It resolves "git" via exec.LookPath at construction time so a
// missing binary surfaces as a clear error up front (at repo-open time)
// rather than as an opaque failure deep inside the first plumbing call. An
// empty dir is accepted and means "inherit the stagehand process's current
// working directory" (matching the provider Executor's Dir semantics).
func New(dir string) (*Git, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git: git binary not found in PATH: %w", err)
	}
	return &Git{dir: dir, git: path}, nil
}

// run is the single low-level exec seam every plumbing method (and the
// white-box tests) call. It builds the git command directly from the args
// slice via exec.Command(g.git, args...) — NEVER via sh -c / zsh -c and never
// interpolating args into a shell string (PRD §19; §22.3: shell out to the
// real git binary). The child runs in g.dir, inherits the process
// environment (cmd.Env is left nil so PATH/HOME/GIT_* reach git — no
// os.Environ() call is needed) and has its stdin connected to /dev/null
// (cmd.Stdin nil). No SysProcAttr/Setpgid is set: process-group kill is the
// provider Executor's concern for agent subprocesses; git plumbing here is
// purely synchronous.
//
// On a non-zero git exit run returns a typed *[ExitError] carrying the Args,
// the exit Code (extracted from the stdlib *exec.ExitError via errors.As),
// and the Stderr captured into our own buffer (NOTE: (*exec.ExitError).Stderr
// is empty when cmd.Stderr is set, so the stderr text MUST be read from our
// buffer). On a start failure (binary gone, permission) it returns a wrapped
// non-typed error. On success it returns the RAW captured stdout (trailing
// newline included; callers trim as needed).
func (g *Git) run(args ...string) (string, error) {
	// Build the command directly from the []string — never sh -c (§19). The
	// resolved absolute path has a separator, so exec.Command skips a
	// re-LookPath and uses g.git verbatim.
	cmd := exec.Command(g.git, args...)
	cmd.Dir = g.dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// cmd.Env is left nil: the child inherits the process environment so
	// PATH/HOME/GIT_* reach git. cmd.Stdin is left nil: os/exec connects the
	// child's stdin to /dev/null. No SysProcAttr/Setpgid (provider concern).

	if err := cmd.Run(); err != nil {
		// Distinguish a non-zero exit (→ typed *ExitError) from a start
		// failure (→ wrapped generic error) via errors.As on the QUALIFIED
		// stdlib *exec.ExitError. Read Stderr from OUR buffer: the stdlib
		// (*exec.ExitError).Stderr is only populated when cmd.Stderr==nil,
		// and we attached our own buffer above.
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", &ExitError{Args: args, Code: ee.ExitCode(), Stderr: stderr.String()}
		}
		return "", fmt.Errorf("git %v: %w", args, err)
	}
	return stdout.String(), nil
}

// ExitError is returned by [Git.run] (and the plumbing methods built on it)
// when git exits with a non-zero status. It is the normal, routable signal
// for conditions such as "not a git repository", "conflict in index", or a
// CAS HEAD-moved failure, so downstream callers and tests can errors.As into
// it and route precisely (mirroring the sibling provider.AgentError pattern).
//
// The Stderr field holds the text captured into our own bytes.Buffer (NOT the
// empty (*exec.ExitError).Stderr), and Code is the real exit code extracted
// from the stdlib *exec.ExitError via errors.As.
type ExitError struct {
	// Args is the []string passed to exec.Command (the git subcommand and its
	// flags), for a clear diagnostic and for test assertions.
	Args []string

	// Code is the git process's exit code (e.g. 128 for a usage/not-a-repo
	// error), extracted from the stdlib *exec.ExitError via errors.As.
	Code int

	// Stderr is the git process's stderr, captured into our own buffer (the
	// stdlib (*exec.ExitError).Stderr is empty when cmd.Stderr is set).
	Stderr string
}

// Error implements the error interface with a one-line summary: the exit
// code, the joined argument list, and a trimmed stderr excerpt.
func (e *ExitError) Error() string {
	return fmt.Sprintf("git exited %d (%s): %s", e.Code, strings.Join(e.Args, " "), strings.TrimSpace(e.Stderr))
}
