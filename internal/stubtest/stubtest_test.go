package stubtest

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/provider"
)

func TestStub_EchoSuccess(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "feat: add x"})
	spec, err := m.Render("", "", "fake prompt payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out, _, err := provider.Execute(context.Background(), *spec, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "feat: add x" {
		t.Errorf("stdout = %q, want %q", out, "feat: add x")
	}
}

func TestStub_MultilineOut(t *testing.T) {
	bin := Build(t)
	want := "subject\n\nbody line"
	m := Manifest(bin, Options{Out: want})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out, _, err := provider.Execute(context.Background(), *spec, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestStub_NonZeroExit(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "", Exit: 1})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	_, _, err = provider.Execute(context.Background(), *spec, 5*time.Second, nil)
	if err == nil {
		t.Fatal("err = nil, want non-nil (exit 1)")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("err = %T, want *exec.ExitError", err)
	}
}

func TestStub_TimeoutKilled(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "", SleepMS: 2000})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	start := time.Now()
	var execErr error
	_, _, execErr = provider.Execute(context.Background(), *spec, 200*time.Millisecond, nil)
	elapsed := time.Since(start)
	err = execErr
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Execute took %v; process-group kill should have fired within ~3s", elapsed)
	}
}

func TestStub_StderrCapture(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "", Stderr: "boom"})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out, errb, err := provider.Execute(context.Background(), *spec, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if errb != "boom" {
		t.Errorf("stderr = %q, want %q", errb, "boom")
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
}

func TestStub_DrainStdinNoDeadlock(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "ok", SleepMS: 500})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// Override Stdin with a 1 MiB payload to pin drain-before-sleep (§4).
	spec.Stdin = strings.Repeat("x", 1<<20)
	out, _, err := provider.Execute(context.Background(), *spec, 10*time.Second, nil)
	if err != nil {
		t.Fatalf("Execute: %v (possible deadlock — stub must drain stdin before sleeping)", err)
	}
	if out != "ok" {
		t.Errorf("stdout = %q, want %q", out, "ok")
	}
}

func TestStub_ScriptCallVarying(t *testing.T) {
	bin := Build(t)
	m := NewScript(t, bin, []string{"feat: dup", "feat: fresh"})

	// Call 1: first response.
	spec, err := m.Render("", "", "payload1", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out, _, err := provider.Execute(context.Background(), *spec, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("Execute call 1: %v", err)
	}
	if out != "feat: dup" {
		t.Errorf("call 1 stdout = %q, want %q", out, "feat: dup")
	}

	// Call 2: second response.
	spec2, err2 := m.Render("", "", "payload2", "off")
	if err2 != nil {
		t.Fatalf("Render: %v", err2)
	}
	out, _, err = provider.Execute(context.Background(), *spec2, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("Execute call 2: %v", err)
	}
	if out != "feat: fresh" {
		t.Errorf("call 2 stdout = %q, want %q", out, "feat: fresh")
	}

	// Call 3: clamps to last.
	spec3, err3 := m.Render("", "", "payload3", "off")
	if err3 != nil {
		t.Fatalf("Render: %v", err3)
	}
	out, _, err = provider.Execute(context.Background(), *spec3, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("Execute call 3: %v", err)
	}
	if out != "feat: fresh" {
		t.Errorf("call 3 stdout = %q, want %q (clamp to last)", out, "feat: fresh")
	}
}

func TestStub_ScriptBlankIsParseFailure(t *testing.T) {
	bin := Build(t)
	m := NewScript(t, bin, []string{"", "feat: good"})

	// Call 1: blank → empty stdout → ParseOutput ok=false.
	spec, err := m.Render("", "", "payload1", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out, _, err := provider.Execute(context.Background(), *spec, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("Execute call 1: %v", err)
	}
	if out != "" {
		t.Errorf("call 1 stdout = %q, want empty", out)
	}

	// Call 2: the good response.
	spec, err2 := m.Render("", "", "payload2", "off")
	if err2 != nil {
		t.Fatalf("Render: %v", err2)
	}
	out, _, err = provider.Execute(context.Background(), *spec, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("Execute call 2: %v", err)
	}
	if out != "feat: good" {
		t.Errorf("call 2 stdout = %q, want %q", out, "feat: good")
	}
}

func TestStub_MalformedEnvNoPanic(t *testing.T) {
	bin := Build(t)
	// Bypass Manifest/Render — build a raw CmdSpec with a malformed EXIT value.
	spec := provider.CmdSpec{
		Command: bin,
		Stdin:   "payload",
		Env:     append(os.Environ(), "STAGEHAND_STUB_EXIT=not-a-number", "STAGEHAND_STUB_OUT=x"),
	}
	out, _, err := provider.Execute(context.Background(), spec, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("Execute: %v (malformed EXIT should not cause non-zero exit)", err)
	}
	if out != "x" {
		t.Errorf("stdout = %q, want %q", out, "x")
	}
}
