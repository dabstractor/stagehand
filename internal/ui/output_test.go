package ui

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestResolveColor_Logic — matrix: (noColor, isTTY) → expected
// ---------------------------------------------------------------------------

func TestResolveColor_Logic(t *testing.T) {
	tests := []struct {
		name    string
		noColor bool
		isTTY   bool
		want    bool
	}{
		{"noColor=true, TTY → false", true, true, false},
		{"noColor=true, pipe → false", true, false, false},
		{"noColor=false, TTY → true", false, true, true},
		{"noColor=false, pipe → false", false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveColor(tc.noColor, tc.isTTY)
			if got != tc.want {
				t.Errorf("ResolveColor(%v, %v) = %v, want %v", tc.noColor, tc.isTTY, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestResolveColor_NoColorEnv — NO_COLOR env overrides
// ---------------------------------------------------------------------------

func TestResolveColor_NoColorEnv(t *testing.T) {
	t.Run("NO_COLOR=1 disables", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		if ResolveColor(false, true) {
			t.Error("ResolveColor(false, true) = true, want false (NO_COLOR=1)")
		}
	})

	t.Run("NO_COLOR empty does NOT disable", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		// NO_COLOR="" → ok && v!="" is false → noColorEnvSet returns false → isTTY-gated
		if !ResolveColor(false, true) {
			t.Error("ResolveColor(false, true) = false, want true (NO_COLOR empty does not disable)")
		}
		if ResolveColor(false, false) {
			t.Error("ResolveColor(false, false) = true, want false (pipe)")
		}
	})

	t.Run("NO_COLOR unset → isTTY-gated", func(t *testing.T) {
		// Ensure NO_COLOR is unset for this subtest
		t.Setenv("NO_COLOR", "")
		os.Unsetenv("NO_COLOR")
		if !ResolveColor(false, true) {
			t.Error("ResolveColor(false, true) = false, want true (no NO_COLOR, TTY)")
		}
		if ResolveColor(false, false) {
			t.Error("ResolveColor(false, false) = true, want false (pipe)")
		}
	})

	t.Run("NO_COLOR independent of STAGEHAND_NO_COLOR", func(t *testing.T) {
		// NO_COLOR is handled by ui; STAGEHAND_NO_COLOR is handled by config and folded into noColor.
		// Setting STAGEHAND_NO_COLOR should NOT affect ResolveColor (it doesn't read it).
		t.Setenv("NO_COLOR", "")
		os.Unsetenv("NO_COLOR")
		t.Setenv("STAGEHAND_NO_COLOR", "1")
		if !ResolveColor(false, true) {
			t.Error("ResolveColor(false, true) = false, want true (STAGEHAND_NO_COLOR not read by ui)")
		}
	})

	t.Run("noColor=true overrides NO_COLOR unset", func(t *testing.T) {
		os.Unsetenv("NO_COLOR")
		if ResolveColor(true, true) {
			t.Error("ResolveColor(true, true) = true, want false (noColor flag)")
		}
	})
}

// ---------------------------------------------------------------------------
// TestColorHelpers_NoOpWhenColorOff — plain strings when color=false
// ---------------------------------------------------------------------------

func TestColorHelpers_NoOpWhenColorOff(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	u := New(&outBuf, &errBuf, false)

	if u.Color() {
		t.Error("Color() = true, want false")
	}

	// Green/Red/Yellow return plain string
	if got := u.Green("x"); got != "x" {
		t.Errorf("Green(%q) = %q, want %q", "x", got, "x")
	}
	if got := u.Red("y"); got != "y" {
		t.Errorf("Red(%q) = %q, want %q", "y", got, "y")
	}
	if got := u.Yellow("z"); got != "z" {
		t.Errorf("Yellow(%q) = %q, want %q", "z", got, "z")
	}

	// Progress writes plain to stderr
	u.Progress("hello")
	if outBuf.Len() != 0 {
		t.Errorf("stdout = %q, want empty", outBuf.String())
	}
	if got := errBuf.String(); got != "↳ hello\n" {
		t.Errorf("stderr = %q, want %q", got, "↳ hello\n")
	}

	// Success writes plain to stderr
	outBuf.Reset()
	errBuf.Reset()
	u.Success("ok")
	if outBuf.Len() != 0 {
		t.Errorf("stdout = %q, want empty", outBuf.String())
	}
	if got := errBuf.String(); got != "↳ ok\n" {
		t.Errorf("stderr = %q, want %q", got, "↳ ok\n")
	}

	// Error writes plain to stderr
	outBuf.Reset()
	errBuf.Reset()
	u.Error("fail")
	if outBuf.Len() != 0 {
		t.Errorf("stdout = %q, want empty", outBuf.String())
	}
	if got := errBuf.String(); got != "fail\n" {
		t.Errorf("stderr = %q, want %q", got, "fail\n")
	}
}

// ---------------------------------------------------------------------------
// TestColorHelpers_WrapWhenColorOn — ANSI codes when color=true
// ---------------------------------------------------------------------------

func TestColorHelpers_WrapWhenColorOn(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	u := New(&outBuf, &errBuf, true)

	if !u.Color() {
		t.Error("Color() = false, want true")
	}

	// Green wraps in ANSI green
	if got := u.Green("x"); got != "\x1b[32mx\x1b[0m" {
		t.Errorf("Green(%q) = %q, want %q", "x", got, "\x1b[32mx\x1b[0m")
	}
	// Red wraps in ANSI red
	if got := u.Red("y"); got != "\x1b[31my\x1b[0m" {
		t.Errorf("Red(%q) = %q, want %q", "y", got, "\x1b[31my\x1b[0m")
	}
	// Yellow wraps in ANSI yellow
	if got := u.Yellow("z"); got != "\x1b[33mz\x1b[0m" {
		t.Errorf("Yellow(%q) = %q, want %q", "z", got, "\x1b[33mz\x1b[0m")
	}

	// Progress writes PLAIN to stderr (Progress is not colorized — it's the ↳ prefix + msg)
	outBuf.Reset()
	errBuf.Reset()
	u.Progress("test")
	if outBuf.Len() != 0 {
		t.Errorf("stdout = %q, want empty", outBuf.String())
	}
	if got := errBuf.String(); got != "↳ test\n" {
		t.Errorf("stderr = %q, want %q", got, "↳ test\n")
	}

	// Success writes green-wrapped to stderr
	outBuf.Reset()
	errBuf.Reset()
	u.Success("created")
	if outBuf.Len() != 0 {
		t.Errorf("stdout = %q, want empty", outBuf.String())
	}
	want := "\x1b[32m↳ created\x1b[0m\n"
	if got := errBuf.String(); got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}

	// Error writes red-wrapped to stderr
	outBuf.Reset()
	errBuf.Reset()
	u.Error("failed")
	if outBuf.Len() != 0 {
		t.Errorf("stdout = %q, want empty", outBuf.String())
	}
	want = "\x1b[31mfailed\x1b[0m\n"
	if got := errBuf.String(); got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// TestProgress_PrefixAndStream — exact bytes, stream isolation
// ---------------------------------------------------------------------------

func TestProgress_PrefixAndStream(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	u := New(&outBuf, &errBuf, false)

	u.Progress("Generating…")

	// stdout must be empty (FR51)
	if outBuf.Len() != 0 {
		t.Errorf("stdout = %q, want empty (Progress must go to stderr)", outBuf.String())
	}

	// stderr must contain the exact bytes: ↳ + space + msg + newline
	want := "↳ Generating…\n"
	if got := errBuf.String(); got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}

	// Verify the ↳ is U+21B3 and … is U+2026
	got := errBuf.String()
	if !strings.HasPrefix(got, "↳ ") {
		t.Errorf("stderr does not start with '↳ ' (U+21B3 + space): %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("stderr does not contain '…' (U+2026): %q", got)
	}
}

// ---------------------------------------------------------------------------
// TestProgressLabel — FR51b byte-exact format (pure function, no I/O)
// ---------------------------------------------------------------------------

func TestProgressLabel(t *testing.T) {
	tests := []struct {
		name                  string
		verb, model, provider string
		want                  string
	}{
		{"model+provider", "Generating", "sonnet", "claude", "Generating with sonnet in claude…"},
		{"slash-prefixed model", "Generating", "zai/glm-5.2", "pi", "Generating with zai/glm-5.2 in pi…"},
		{"model empty → provider alone", "Generating", "", "claude", "Generating with claude…"},
		{"provider empty → minimal", "Generating", "sonnet", "", "Generating…"},
		{"both empty", "Generating", "", "", "Generating…"},
		{"decompose", "Decomposing", "anthropic/claude-sonnet-4", "opencode", "Decomposing with anthropic/claude-sonnet-4 in opencode…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ProgressLabel(tc.verb, tc.model, tc.provider)
			if got != tc.want {
				t.Errorf("ProgressLabel(%q, %q, %q) = %q, want %q", tc.verb, tc.model, tc.provider, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestIsTerminal_Pipe — pipe reader is NOT a terminal
// ---------------------------------------------------------------------------

func TestIsTerminal_Pipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if IsTerminal(r) {
		t.Error("IsTerminal(pipe reader) = true, want false")
	}
	if IsTerminal(w) {
		t.Error("IsTerminal(pipe writer) = true, want false")
	}
}

// ---------------------------------------------------------------------------
// TestIsTerminal_DevNull — /dev/null is NOT a terminal (true isatty probe rejects it)
// ---------------------------------------------------------------------------

func TestIsTerminal_DevNull(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("/dev/null is a Unix path; Windows uses NUL and is exercised on a Windows CI runner")
	}
	f, err := os.Open("/dev/null")
	if err != nil {
		t.Fatalf("os.Open(/dev/null): %v", err)
	}
	defer f.Close()

	if IsTerminal(f) {
		t.Error("IsTerminal(/dev/null) = true, want false (true isatty probe must reject a char device that is not a tty)")
	}

	// Belt-and-suspenders: PROVE /dev/null IS a char device — i.e. the OLD heuristic would have misfired.
	// If this ever fails, the regression premise changed on this OS; investigate rather than weaken the test.
	st, err := os.Stat("/dev/null")
	if err != nil {
		t.Fatalf("os.Stat(/dev/null): %v", err)
	}
	if (st.Mode() & os.ModeCharDevice) == 0 {
		t.Error("/dev/null is unexpectedly NOT a char device on this OS — the Issue-4 regression premise is invalid")
	}
}

// ---------------------------------------------------------------------------
// TestNilWriters — nil defaults to os.Stdout / os.Stderr
// ---------------------------------------------------------------------------

func TestNilWriters(t *testing.T) {
	u := New(nil, nil, false)
	if u.Color() {
		t.Error("Color() = true, want false")
	}
	// Green should still work (no panic) with nil-defaulted writers
	if got := u.Green("x"); got != "x" {
		t.Errorf("Green(%q) = %q, want %q", "x", got, "x")
	}
}

// ---------------------------------------------------------------------------
// TestNew_Defaults — verify defaults when nil writers
// ---------------------------------------------------------------------------

func TestNew_Defaults(t *testing.T) {
	u := New(nil, nil, true)
	if !u.Color() {
		t.Error("Color() = false, want true")
	}
	// Progress should not panic writing to default os.Stderr
	u.Progress(fmt.Sprintf("test %s", "msg"))
}

// ---------------------------------------------------------------------------
// TestNoColorEnvSet — unit test for the unexported helper
// ---------------------------------------------------------------------------

func TestNoColorEnvSet(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		os.Unsetenv("NO_COLOR")
		if noColorEnvSet() {
			t.Error("noColorEnvSet() = true, want false (unset)")
		}
	})
	t.Run("empty", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		if noColorEnvSet() {
			t.Error("noColorEnvSet() = true, want false (empty)")
		}
	})
	t.Run("non-empty", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		if !noColorEnvSet() {
			t.Error("noColorEnvSet() = false, want true (NO_COLOR=1)")
		}
	})
	t.Run("space", func(t *testing.T) {
		t.Setenv("NO_COLOR", " ")
		if !noColorEnvSet() {
			t.Error("noColorEnvSet() = false, want true (NO_COLOR=space)")
		}
	})
}
