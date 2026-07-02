package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Executor turns a resolved provider intent into a real, timeout-enforced,
// process-group-isolated agent run. It is the only component in the product
// that actually spawns an agent subprocess: every agent invocation — the
// generate inner loop (M6.T1), the stub-provider integration harness
// (M6.T3.S1), and the real-agent opt-in suite (M8.T3) — flows through Run.
//
// Run renders the manifest (P1.M2.T1.S1, a pure function), execs the agent
// command directly with the rendered []string argument slice (NEVER sh -c,
// per PRD §19), feeds the payload via stdin when the manifest requests it,
// captures stdout (FR24), and — when the supplied context is cancelled or
// times out — kills the agent AND all of its children as a single process
// group (PRD §18.4: SysProcAttr.Setpgid makes the agent a group leader so a
// single kill(-pgid) fells the whole tree). On a deadline it returns a typed
// *TimeoutError so the generate orchestrator can route the rescue path
// (FR25); on a non-zero exit it returns a typed *AgentError carrying the
// stderr excerpt and exit code.
//
// The context.Context passed to Run IS the cancellation seam the signal
// handler (M6.T2.S1) observes: the generate caller owns a cancellable
// context, threads ctx into Run and the cancel func into the signal handler,
// so a SIGINT/SIGTERM calls cancel → ctx.Done() fires in Run → Run kills the
// group. No stored context, public Cancel accessor, or observe channel is
// needed; the ctx parameter is the seam.
//
// Setpgid is a Unix concept (linux/darwin/freebsd); a future windows port
// needs CREATE_NEW_PROCESS_GROUP / taskkill /T and is explicitly out of
// scope here (the validation gate runs on linux).
type Executor struct {
	// Dir is the working directory the agent subprocess runs in (PRD §12.2:
	// cmd.Dir = repo root). An empty string means "inherit the stagehand
	// process's current directory".
	Dir string
}

// NewExecutor returns an Executor whose subprocesses run in dir. It exists
// for ergonomic, self-documenting construction at call sites (e.g. the
// generate orchestrator passes the repo root).
func NewExecutor(dir string) *Executor {
	return &Executor{Dir: dir}
}

// Run resolves model/provider defaults from the manifest, renders the command
// (P1.M2.T1.S1), execs the agent directly with the []string arg slice (PRD
// §19 — never sh -c), and returns the captured stdout (FR24).
//
// Cancellation/timeout semantics (FR25, §18.4): the supplied ctx is raced
// against the child's exit via a goroutine on cmd.Wait and a select. When
// ctx is cancelled or its deadline elapses, Run sends SIGTERM to the agent's
// whole process group (syscall.Kill(-pgid, SIGTERM); Setpgid made the agent
// a group leader so its PID equals its PGID), waits up to ~2s for exit, and
// then escalates to SIGKILL on the group — guaranteeing the agent AND any
// children (sub-shells, MCP servers) are felled even if they ignore SIGTERM.
// A deadline expiry yields a *TimeoutError (Unwrap → context.DeadlineExceeded,
// so callers can treat it uniformly); an explicit cancel yields ctx.Err()
// (context.Canceled). A non-zero exit yields a *AgentError with the exit code
// and a short stderr excerpt.
//
// The caller owns ctx: the generate orchestrator wraps a parent context with
// context.WithTimeout(parent, cfg.Timeout) (FR25's default 120s) and — for
// signal-driven cancellation — hands the cancel func to the signal handler
// (M6.T2.S1). Run takes no timeout parameter of its own.
func (e *Executor) Run(ctx context.Context, m Manifest, model, provider, sys, payload string) (string, error) {
	// Render is pure and does NOT resolve defaults (P1.M2.T1.S1 seam); Run
	// owns DefaultModel/DefaultProvider resolution so a caller passing ""
	// gets the manifest's configured default.
	if model == "" {
		model = m.DefaultModel
	}
	if provider == "" {
		provider = m.DefaultProvider
	}

	r, err := m.Render(model, provider, sys, payload)
	if err != nil {
		return "", err
	}

	// Build the command directly from Render's arg slice — never sh -c (§19).
	// r.Args excludes the command token by construction.
	cmd := exec.Command(m.Command, r.Args...)
	cmd.Dir = e.Dir
	if r.DeliverViaStdin {
		cmd.Stdin = bytes.NewReader([]byte(r.StdinPayload))
	} // else nil ⇒ os/exec connects the child's stdin to /dev/null automatically
	cmd.Env = append(os.Environ(), r.Env...)
	// Setpgid makes the agent a process-group leader (PGID == PID) so a single
	// kill(-pgid) fells the agent and all its children (PRD §18.4).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("provider %q: failed to start %q: %w", m.Name, m.Command, err)
	}

	// With Setpgid the child's PID equals its process-group ID, so negating
	// it targets the whole group rather than just the leader (§18.4).
	pgid := cmd.Process.Pid
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err == nil {
			return stdout.String(), nil
		}
		return "", &AgentError{
			Name:    m.Name,
			Command: m.Command,
			Code:    exitCodeOf(err),
			Stderr:  excerpt(stderr.String()),
		}
	case <-ctx.Done():
		cause := ctx.Err()
		// SIGTERM the whole group; then, if it has not exited within a grace
		// period, escalate to SIGKILL (§18.4). Always drain `done` so the
		// Wait goroutine never leaks.
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			<-done
		}
		if cause == context.DeadlineExceeded {
			return "", &TimeoutError{Deadline: time.Now()}
		}
		return "", cause
	}
}

// exitCodeOf extracts the child's exit code from a cmd.Wait error, returning
// -1 when the process was killed by an unexpected signal (not a clean exit
// nor the ctx-driven group kill, which never reaches this path).
func exitCodeOf(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// excerpt returns a short, trimmed slice of stderr for diagnostic messages:
// at most the last ~500 characters. It keeps the tail (the actual error)
// when stderr is long.
func excerpt(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 500 {
		s = s[len(s)-500:]
	}
	return s
}

// TimeoutError is returned when Run's context deadline elapsed and the agent
// process group was killed (FR25). It wraps context.DeadlineExceeded so
// callers can errors.Is(err, context.DeadlineExceeded) to treat timeouts
// uniformly while still errors.As-ing into this type to detect the rescue
// trigger (PRD §18.4).
type TimeoutError struct {
	// Deadline is the time the run was killed (best-effort; the actual
	// ctx deadline is owned by the caller and not exposed here).
	Deadline time.Time
}

// Error implements the error interface.
func (e *TimeoutError) Error() string { return "provider: generation timed out" }

// Unwrap allows errors.Is(err, context.DeadlineExceeded) to succeed, so the
// generate orchestrator can branch on the standard sentinel while still
// detecting the typed timeout via errors.As for the rescue path.
func (e *TimeoutError) Unwrap() error { return context.DeadlineExceeded }

// AgentError is returned when the agent subprocess exited with a non-zero
// status (bad flags, missing config, crash). It carries enough context for a
// clear diagnostic and for the generate orchestrator's rescue routing
// (decisions.md §3): the provider Name, the Command that was run, the exit
// Code, and a short Stderr excerpt.
type AgentError struct {
	Name    string // m.Name — the provider identifier, for a clear message
	Command string // m.Command — the executable that was run
	Code    int    // exit code (-1 if the process died from an unexpected signal)
	Stderr  string // short excerpt (last ~500 chars, trimmed) of the child's stderr
}

// Error implements the error interface with a one-line summary.
func (e *AgentError) Error() string {
	return fmt.Sprintf("provider %q: %s exited %d: %s", e.Name, e.Command, e.Code, e.Stderr)
}
