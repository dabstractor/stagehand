package provider

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These tests exercise Executor.Run against REAL host binaries as fake agents
// (/bin/echo, /bin/cat, /bin/sh, /bin/sleep, /bin/pwd) — no real LLM, no
// stub-binary generator (PRD §20.1 layer 1/2). Each test builds a minimal
// inline Manifest literal plus an Executor{Dir: ""} (Dir="" ⇒ inherit the
// stagehand cwd, which is fine for echo/cat/sh/sleep; the Dir test sets Dir
// explicitly). The timing-sensitive timeout/cancel/grace tests are individual
// functions so a failure points straight at the behavior that regressed.

// TestRun_StdinFedExactly proves the rendered payload is piped to the child's
// stdin byte-for-byte: /bin/cat echoes its stdin verbatim.
func TestRun_StdinFedExactly(t *testing.T) {
	m := Manifest{Name: "cat", Command: "/bin/cat", PromptDelivery: DeliveryStdin}
	e := &Executor{}

	out, err := e.Run(context.Background(), m, "", "", "", "FED-EXACT")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if out != "FED-EXACT" {
		t.Errorf("stdout = %q, want %q (stdin must be fed byte-exactly)", out, "FED-EXACT")
	}
}

// TestRun_StdoutCaptured proves the child's stdout is captured (FR24): a
// rendered positional arg reaches /bin/echo and its output is returned.
func TestRun_StdoutCaptured(t *testing.T) {
	m := Manifest{Name: "echo", Command: "/bin/echo", PromptDelivery: DeliveryPositional}
	e := &Executor{}

	out, err := e.Run(context.Background(), m, "", "", "", "hi")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if out != "hi\n" {
		t.Errorf("stdout = %q, want %q", out, "hi\n")
	}
}

// TestRun_AgentErrorNonZeroExit proves a non-zero-exit fake yields a typed
// *AgentError carrying the exit code and a stderr excerpt.
func TestRun_AgentErrorNonZeroExit(t *testing.T) {
	m := Manifest{
		Name:           "sh",
		Command:        "/bin/sh",
		Subcommand:     []string{"-c", "echo oops >&2; exit 7"},
		PromptDelivery: DeliveryStdin,
	}
	e := &Executor{}

	out, err := e.Run(context.Background(), m, "", "", "", "")
	if err == nil {
		t.Fatalf("Run returned nil error; want *AgentError (out=%q)", out)
	}
	var ae *AgentError
	if !errors.As(err, &ae) {
		t.Fatalf("Run error is %T; want *AgentError", err)
	}
	if ae.Code != 7 {
		t.Errorf("AgentError.Code = %d, want 7", ae.Code)
	}
	if !strings.Contains(ae.Stderr, "oops") {
		t.Errorf("AgentError.Stderr = %q; want it to contain %q", ae.Stderr, "oops")
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty on non-zero exit", out)
	}
	if ae.Command != "/bin/sh" || ae.Name != "sh" {
		t.Errorf("AgentError = %+v; want Name=%q Command=%q", ae, "sh", "/bin/sh")
	}
}

// TestRun_TimeoutKillsGroup proves a deadline ctx triggers a process-group
// SIGTERM that fells the stub fast (well under the 2s grace) and returns a
// *TimeoutError that wraps context.DeadlineExceeded.
func TestRun_TimeoutKillsGroup(t *testing.T) {
	m := Manifest{
		Name:           "sleep",
		Command:        "/bin/sleep",
		Subcommand:     []string{"30"},
		PromptDelivery: DeliveryStdin,
	}
	e := &Executor{}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	start := time.Now()
	out, err := e.Run(ctx, m, "", "", "", "")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("Run returned nil error; want *TimeoutError (out=%q)", out)
	}
	var te *TimeoutError
	if !errors.As(err, &te) {
		t.Fatalf("Run error is %T; want *TimeoutError", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false; want true (TimeoutError must wrap it)")
	}
	// SIGTERM alone kills /bin/sleep, so the 2s grace must NOT be needed:
	// elapsed should be ~80ms. Allow generous headroom but well under 2s so a
	// regression to "SIGTERM did nothing, waited the full grace" is caught.
	if elapsed >= time.Second {
		t.Errorf("elapsed = %v; want < 1s (proves SIGTERM killed the group fast, not the 2s grace)", elapsed)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty on timeout", out)
	}
}

// TestRun_CancelReturnsCanceled proves an explicitly-cancelled ctx (the signal-
// handler seam) returns context.Canceled, not *TimeoutError.
func TestRun_CancelReturnsCanceled(t *testing.T) {
	m := Manifest{
		Name:           "sleep",
		Command:        "/bin/sleep",
		Subcommand:     []string{"30"},
		PromptDelivery: DeliveryStdin,
	}
	e := &Executor{}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(40 * time.Millisecond)
		cancel()
	}()

	out, err := e.Run(ctx, m, "", "", "", "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v; want errors.Is(err, context.Canceled) true (out=%q)", err, out)
	}
	// And it must NOT be a *TimeoutError — cancel is distinct from deadline.
	var te *TimeoutError
	if errors.As(err, &te) {
		t.Errorf("Run error is *TimeoutError; want context.Canceled for an explicit cancel")
	}
}

// TestRun_GraceSIGKILLOnIgnoredTERM proves BOTH the process-group targeting
// (-PGID) AND the 2s-grace SIGKILL escalation: the child ignores SIGTERM (an
// ignored disposition is inherited across exec, so even /bin/sleep ignores it),
// so SIGTERM cannot kill the group — the 2s grace must expire and SIGKILL must
// fire. Elapsed lands at ~timeout(80ms) + grace(2s) ≈ 2.08s, which would be
// impossible if either the group kill or the grace escalation were broken.
func TestRun_GraceSIGKILLOnIgnoredTERM(t *testing.T) {
	m := Manifest{
		Name:           "sh",
		Command:        "/bin/sh",
		Subcommand:     []string{"-c", "trap '' TERM; sleep 30"},
		PromptDelivery: DeliveryStdin,
	}
	e := &Executor{}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	start := time.Now()
	out, err := e.Run(ctx, m, "", "", "", "")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("Run returned nil error; want *TimeoutError (out=%q)", out)
	}
	var te *TimeoutError
	if !errors.As(err, &te) {
		t.Fatalf("Run error is %T; want *TimeoutError", err)
	}
	// Elapsed MUST be ≥ the grace, proving SIGKILL escalation fired (a pure
	// SIGTERM-kill would have returned at ~80ms). Lower bound sits safely below
	// the theoretical 80ms+2s≈2.08s; upper bound bounds the test's wall time.
	if elapsed < 1900*time.Millisecond {
		t.Errorf("elapsed = %v; want >= 1.9s (proves the 2s-grace SIGKILL fired, since SIGTERM is ignored)", elapsed)
	}
	if elapsed >= 4*time.Second {
		t.Errorf("elapsed = %v; want < 4s (grace SIGKILL should fire at ~2.08s)", elapsed)
	}
}

// TestRun_DefaultModelResolved proves Run resolves DefaultModel before Render:
// passing model="" surfaces m.DefaultModel in the rendered arg slice. /bin/echo
// prints its positional args, so the flag+default land in stdout.
func TestRun_DefaultModelResolved(t *testing.T) {
	m := Manifest{
		Name:           "echo",
		Command:        "/bin/echo",
		ModelFlag:      "--model",
		DefaultModel:   "glm-5-turbo",
		PromptDelivery: DeliveryPositional,
	}
	e := &Executor{}

	out, err := e.Run(context.Background(), m, "", "", "", "BODY") // model==""
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if want := "--model glm-5-turbo"; !strings.Contains(out, want) {
		t.Errorf("stdout = %q; want it to contain %q (default model must be resolved)", out, want)
	}
}

// TestRun_RenderErrorPropagates proves Run surfaces the Render error and starts
// NO process when the manifest is invalid (unknown prompt_delivery).
func TestRun_RenderErrorPropagates(t *testing.T) {
	m := Manifest{Name: "bogus", Command: "/bin/echo", PromptDelivery: "telepathy"}
	e := &Executor{}

	out, err := e.Run(context.Background(), m, "", "", "", "BODY")
	if err == nil {
		t.Fatalf("Run returned nil error; want the Render error for prompt_delivery %q", "telepathy")
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty on Render error", out)
	}
	if !strings.Contains(err.Error(), "unknown prompt_delivery") {
		t.Errorf("error = %q; want it to mention %q", err.Error(), "unknown prompt_delivery")
	}
}

// TestRun_EnvAdditionsMerged proves the manifest's Env additions reach the
// child environment (cmd.Env = os.Environ() + r.Env).
func TestRun_EnvAdditionsMerged(t *testing.T) {
	m := Manifest{
		Name:           "sh",
		Command:        "/bin/sh",
		Subcommand:     []string{"-c", "echo $SH_TEST"},
		Env:            map[string]string{"SH_TEST": "xyz"},
		PromptDelivery: DeliveryStdin,
	}
	e := &Executor{}

	out, err := e.Run(context.Background(), m, "", "", "", "")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if out != "xyz\n" {
		t.Errorf("stdout = %q, want %q (manifest Env must reach the child)", out, "xyz\n")
	}
}

// TestRun_DirSetsCwd proves Executor.Dir sets the child's working directory.
// EvalSymlinks guards against a symlinked TMPDIR (some hosts run /tmp via a
// symlink; /bin/pwd prints the canonical physical path).
func TestRun_DirSetsCwd(t *testing.T) {
	dir := t.TempDir()
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		want = dir
	}
	m := Manifest{Name: "pwd", Command: "/bin/pwd", PromptDelivery: DeliveryStdin}
	e := &Executor{Dir: dir}

	out, err := e.Run(context.Background(), m, "", "", "", "")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if got := strings.TrimSpace(out); got != want {
		t.Errorf("pwd output = %q, want %q (Dir must set the child cwd)", got, want)
	}
}
