package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagecoach/internal/ui"
)

// mustBin skips the test if any named binary is not resolvable on PATH.
func mustBin(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			t.Skipf("required binary %q not in PATH: %v", n, err)
		}
	}
}

// 1. Normal run: `cat` echoes stdin to stdout verbatim.
func TestExecute_CatEchoesStdin(t *testing.T) {
	mustBin(t, "cat")
	spec := CmdSpec{Command: "cat", Args: nil, Stdin: "hello world\n", Env: os.Environ()}
	out, _, err := Execute(context.Background(), spec, 0, nil)
	if err != nil {
		t.Fatalf("Execute: err = %v, want nil", err)
	}
	if out != "hello world\n" {
		t.Errorf("stdout = %q, want %q", out, "hello world\n")
	}
}

// 2. Large stdin: 1MiB round-trips byte-for-byte (proves stdin piping, not a tiny buffer).
func TestExecute_LargeStdin(t *testing.T) {
	mustBin(t, "cat")
	payload := strings.Repeat("x", 1<<20) // 1 MiB
	spec := CmdSpec{Command: "cat", Stdin: payload, Env: os.Environ()}
	out, _, err := Execute(context.Background(), spec, 30*time.Second, nil)
	if err != nil {
		t.Fatalf("Execute: err = %v, want nil", err)
	}
	if out != payload {
		t.Errorf("large stdin: len(stdout) = %d, want %d", len(out), len(payload))
	}
}

//  3. THE KEYSTONE — timeout kills the process: `sleep 30` + 200ms timeout ⇒ DeadlineExceeded,
//     AND Execute returns within seconds (not 30s) — proving cmd.Cancel fired and killed the group.
func TestExecute_TimeoutKillsProcess(t *testing.T) {
	mustBin(t, "sleep")
	spec := CmdSpec{Command: "sleep", Args: []string{"30"}, Stdin: "", Env: os.Environ()}
	start := time.Now()
	_, _, err := Execute(context.Background(), spec, 200*time.Millisecond, nil)
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Execute took %v; the process-group kill should have fired within ~3s (WaitDelay)", elapsed)
	}
}

// 4. Parent-context cancel ⇒ context.Canceled (distinguished from timeout).
func TestExecute_ParentContextCancel(t *testing.T) {
	mustBin(t, "sleep")
	ctx, cancel := context.WithCancel(context.Background())
	spec := CmdSpec{Command: "sleep", Args: []string{"30"}, Env: os.Environ()}
	go func() { time.Sleep(150 * time.Millisecond); cancel() }()
	_, _, err := Execute(ctx, spec, 0, nil) // no timeout; rely on parent cancel
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// 5. stderr captured SEPARATELY + non-zero exit surfaced: `cat /nonexistent` writes to stderr, exits 1.
func TestExecute_StderrCaptureAndNonZeroExit(t *testing.T) {
	mustBin(t, "cat")
	spec := CmdSpec{Command: "cat", Args: []string{"/nonexistent/path/xyz"}, Env: os.Environ()}
	out, errb, err := Execute(context.Background(), spec, 5*time.Second, nil)
	if err == nil {
		t.Fatal("err = nil, want non-nil (non-zero exit)")
	}
	if errb == "" {
		t.Errorf("stderr = empty, want a 'No such file' message")
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
}

// 6. Env propagation: a manifest/env var reaches the child (`printenv VAR` prints its value).
func TestExecute_EnvPropagation(t *testing.T) {
	mustBin(t, "printenv")
	env := append(os.Environ(), "STAGECOACH_TEST_VAR=s3cr3t")
	spec := CmdSpec{Command: "printenv", Args: []string{"STAGECOACH_TEST_VAR"}, Env: env}
	out, _, err := Execute(context.Background(), spec, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("Execute: err = %v, want nil", err)
	}
	if strings.TrimSpace(out) != "s3cr3t" {
		t.Errorf("env var: stdout = %q, want %q", out, "s3cr3t")
	}
}

//  7. No stdin (positional/flag delivery): Stdin="" ⇒ no pipe ⇒ child gets /dev/null ⇒ cat exits 0
//     immediately with empty output (and does NOT hang).
func TestExecute_NoStdinDoesNotHang(t *testing.T) {
	mustBin(t, "cat")
	spec := CmdSpec{Command: "cat", Stdin: "", Env: os.Environ()}
	out, _, err := Execute(context.Background(), spec, 3*time.Second, nil)
	if err != nil {
		t.Fatalf("Execute: err = %v, want nil", err)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty (no stdin)", out)
	}
}

// 8. Command not found ⇒ wrapped Start() error.
func TestExecute_CommandNotFound(t *testing.T) {
	spec := CmdSpec{Command: "definitely-not-a-real-binary-xyz-stagecoach", Env: os.Environ()}
	_, _, err := Execute(context.Background(), spec, 3*time.Second, nil)
	if err == nil {
		t.Fatal("err = nil, want non-nil (command not found)")
	}
	if !strings.Contains(err.Error(), "start") && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "executable") {
		t.Errorf("err = %v, want it to mention start/not found/executable", err)
	}
}

//  10. Verbose mode: verifies DEBUG: command and DEBUG: raw output land in the injected buffer,
//     and that NO env vars (PATH, etc.) leak into the verbose output (PRD §19 security guard).
//     Also verifies the DEBUG: payload line emits from spec.PayloadBytes (FR50) — after the fix the
//     executor reads PayloadBytes, not len(Stdin), so the keystone must construct a spec with
//     PayloadBytes set and assert the payload-size line.
func TestExecute_Verbose(t *testing.T) {
	mustBin(t, "cat")
	var buf bytes.Buffer
	vb := ui.NewVerbose(&buf, true)
	spec := CmdSpec{Command: "cat", Stdin: "feat: hello\n", PayloadBytes: len("feat: hello\n"), Env: os.Environ()}
	out, _, err := Execute(context.Background(), spec, 5*time.Second, vb)
	if err != nil {
		t.Fatalf("Execute: err = %v, want nil", err)
	}
	if out != "feat: hello\n" {
		t.Errorf("stdout = %q, want %q", out, "feat: hello\n")
	}
	got := buf.String()
	if !strings.Contains(got, "DEBUG: command: cat\n") {
		t.Errorf("verbose missing DEBUG command line; got %q", got)
	}
	if !strings.Contains(got, "DEBUG: raw output:\nfeat: hello\n") {
		t.Errorf("verbose missing DEBUG raw output line; got %q", got)
	}
	if want := "DEBUG: payload: 12 bytes"; !strings.Contains(got, want) {
		t.Errorf("verbose missing DEBUG payload line %q; got %q", want, got)
	}
	if strings.Contains(got, "PATH") {
		t.Errorf("SECURITY (§19): env var leaked to verbose output; got %q", got)
	}
	if strings.Contains(got, "API_KEY") {
		t.Errorf("SECURITY (§19): API_KEY leaked to verbose output; got %q", got)
	}
}

// 10b. THE REGRESSION: positional-delivery spec (Stdin=="", payload as a trailing arg, PayloadBytes
//
//	set by the renderer) MUST emit the DEBUG: payload line. Before the fix the executor called
//	VerbosePayload(len(spec.Stdin)) == VerbosePayload(0), hitting the no-op guard — so
//	positional/flag providers printed NO payload-size line at all, defeating FR50 for them.
func TestExecute_VerbosePayload_PositionalDelivery(t *testing.T) {
	mustBin(t, "echo")
	var buf bytes.Buffer
	vb := ui.NewVerbose(&buf, true)
	payload := "some diff payload that is NOT on stdin"
	spec := CmdSpec{
		Command:      "echo",
		Args:         []string{payload}, // positional delivery: payload is a trailing arg
		Stdin:        "",                // positional → no stdin
		PayloadBytes: len(payload),      // the renderer sets this; we set it directly here
		Env:          os.Environ(),
	}
	if _, _, err := Execute(context.Background(), spec, 3*time.Second, vb); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := buf.String()
	want := fmt.Sprintf("DEBUG: payload: %d bytes", len(payload))
	if !strings.Contains(got, want) {
		t.Errorf("positional delivery: missing %q in verbose output:\n%s", want, got)
	}
}

// 9. Non-zero exit: `false` exits 1 ⇒ non-nil wrapped error (exit-failure path).
func TestExecute_NonZeroExit(t *testing.T) {
	mustBin(t, "false")
	spec := CmdSpec{Command: "false", Env: os.Environ()}
	out, _, err := Execute(context.Background(), spec, 0, nil)
	if err == nil {
		t.Fatal("err = nil, want non-nil (`false` exits 1)")
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
}
