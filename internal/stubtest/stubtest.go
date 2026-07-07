// Package stubtest provides a reusable fake-agent (cmd/stubagent) and helpers for Stagecoach's
// integration and property tests (PRD §20.1 layer 3). Build compiles the stub once per test process;
// Manifest/NewScript return test-only provider.Manifests whose Env knobs drive the stub's behavior
// through the real provider.Execute seam. Used by generate.CommitStaged integration tests
// (P1.M3.T4.S2) and the property/invariant tests (P1.M5.T1).
package stubtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/dustin/stagecoach/internal/provider"
)

// Options configures a stub invocation; Manifest/Env translate it to STAGECOACH_STUB_* env vars.
type Options struct {
	Out            string // STAGECOACH_STUB_OUT (single-response; used when Script=="")
	Exit           int    // STAGECOACH_STUB_EXIT (default 0; non-zero ⇒ failed-agent simulation)
	SleepMS        int    // STAGECOACH_STUB_SLEEP_MS (default 0; >0 ⇒ slow/timing-out agent)
	Stderr         string // STAGECOACH_STUB_STDERR (default "")
	Script         string // STAGECOACH_STUB_SCRIPT path (call-varying mode; "" disables)
	Counter        string // STAGECOACH_STUB_COUNTER path (used with Script)
	Output         string // manifest Output; "" → "raw"
	StripCodeFence *bool  // manifest StripCodeFence; nil → true
	ArgsFile       string // STAGECOACH_STUB_ARGSFILE (writes the stub's os.Args to this path — observe rendered argv)
}

var (
	stubOnce sync.Once
	stubPath string
)

// Build compiles ./cmd/stubagent ONCE per test process (cached) and returns its path. Skips t if
// the go toolchain isn't on PATH. The path is reused across all tests in the binary.
func Build(t testing.TB) string {
	t.Helper()
	stubOnce.Do(func() {
		goPath, err := exec.LookPath("go")
		if err != nil {
			t.Skipf("go toolchain not on PATH; cannot build stubagent: %v", err)
			return
		}
		dir, err := os.MkdirTemp("", "stagecoach-stubagent-*")
		if err != nil {
			t.Fatalf("mkdtemp: %v", err)
		}
		name := "stubagent"
		if runtime.GOOS == "windows" {
			name = "stubagent.exe"
		}
		stubPath = filepath.Join(dir, name)
		// Import-path form resolves from any cwd (no cmd.Dir needed).
		build := exec.Command(goPath, "build", "-o", stubPath, "github.com/dustin/stagecoach/cmd/stubagent")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build stubagent: %v\n%s", err, out)
		}
	})
	return stubPath
}

// optsEnvMap is the single source of truth for the STAGECOACH_STUB_* knobs (Env and Manifest both use it).
func optsEnvMap(o Options) map[string]string {
	m := map[string]string{
		"STAGECOACH_STUB_EXIT": strconv.Itoa(o.Exit),
	}
	if o.SleepMS > 0 {
		m["STAGECOACH_STUB_SLEEP_MS"] = strconv.Itoa(o.SleepMS)
	}
	if o.Stderr != "" {
		m["STAGECOACH_STUB_STDERR"] = o.Stderr
	}
	if o.ArgsFile != "" {
		m["STAGECOACH_STUB_ARGSFILE"] = o.ArgsFile
	}
	if o.Script != "" {
		m["STAGECOACH_STUB_SCRIPT"] = o.Script
		if o.Counter != "" {
			m["STAGECOACH_STUB_COUNTER"] = o.Counter
		}
	} else {
		m["STAGECOACH_STUB_OUT"] = o.Out // single-response mode
	}
	return m
}

// Env returns the "K=V" env slice for o (os.Environ() + STAGECOACH_STUB_*). Use to build a raw CmdSpec.
func Env(o Options) []string {
	env := os.Environ()
	for k, v := range optsEnvMap(o) {
		env = append(env, k+"="+v)
	}
	return env
}

// Local pointer helpers (provider.strPtr/boolPtr are unexported — different package can't call them).
func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

// Manifest returns a test-only provider.Manifest pointing Command at the stub, ready to Render+Execute.
func Manifest(bin string, o Options) provider.Manifest {
	out := o.Output
	if out == "" {
		out = "raw"
	}
	scf := true
	if o.StripCodeFence != nil {
		scf = *o.StripCodeFence
	}
	return provider.Manifest{
		Name:           "stub",
		Command:        strPtr(bin),
		PromptDelivery: strPtr("stdin"),
		Output:         strPtr(out),
		StripCodeFence: boolPtr(scf),
		Env:            optsEnvMap(o),
	}
}

// NewScript wires call-varying mode: responses[0] is call 1's stdout, responses[1] call 2's, etc.;
// blank entries are significant (empty output ⇒ ParseOutput ok=false). After the list is exhausted
// the last response repeats. Files live in t.TempDir() (auto-cleaned).
func NewScript(t testing.TB, bin string, responses []string) provider.Manifest {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "script.txt")
	if err := os.WriteFile(script, []byte(strings.Join(responses, "\n")), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	counter := filepath.Join(dir, "counter") // absent ⇒ stub reads 0
	return Manifest(bin, Options{Script: script, Counter: counter})
}
