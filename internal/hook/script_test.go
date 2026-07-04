package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookScript_NonStrict(t *testing.T) {
	got := hookScript(false, "")
	want := "#!/bin/sh\n" +
		"# stagehand prepare-commit-msg hook v1\n" +
		`exec stagehand hook exec "$@"` + "\n"
	if got != want {
		t.Fatalf(`hookScript(false, "") = %q, want %q`, got, want)
	}
	lines := strings.Split(got, "\n")
	if lines[0] != "#!/bin/sh" {
		t.Fatalf("first line = %q, want shebang", lines[0])
	}
	if lines[1] != Marker {
		t.Fatalf("second line = %q, want Marker %q", lines[1], Marker)
	}
	if !strings.Contains(got, `exec stagehand hook exec "$@"`) {
		t.Fatalf(`hookScript(false, "") missing expected exec line: %q`, got)
	}
	if strings.Contains(got, "--strict") {
		t.Fatalf(`hookScript(false, "") must not contain --strict: %q`, got)
	}
}

func TestHookScript_Strict(t *testing.T) {
	got := hookScript(true, "")
	if !strings.HasPrefix(got, "#!/bin/sh\n"+Marker+"\n") {
		t.Fatalf(`hookScript(true, "") does not start with shebang + Marker: %q`, got)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	last := lines[len(lines)-1]
	want := `exec stagehand hook exec --strict "$@"`
	if last != want {
		t.Fatalf(`hookScript(true, "") last line = %q, want %q`, last, want)
	}
}

func TestHookScript_MarkerPresent(t *testing.T) {
	if !strings.Contains(hookScript(false, ""), Marker) {
		t.Fatalf(`hookScript(false, "") does not contain Marker`)
	}
	if !strings.Contains(hookScript(true, ""), Marker) {
		t.Fatalf(`hookScript(true, "") does not contain Marker`)
	}
}

func TestHookScript_POSIX(t *testing.T) {
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not found on PATH; skipping POSIX syntax check")
	}

	for _, tc := range []struct {
		name   string
		script string
	}{
		{"non-strict", hookScript(false, "")},
		{"strict", hookScript(true, "")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "prepare-commit-msg")
			if err := os.WriteFile(f, []byte(tc.script), ScriptMode); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			cmd := exec.Command(shPath, "-n", f)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("sh -n %s failed: %v\n%s", f, err, out)
			}
		})
	}
}

func TestScriptMode(t *testing.T) {
	if ScriptMode != 0o755 {
		t.Fatalf("ScriptMode = %v, want 0o755", ScriptMode)
	}
}

// TestHookScript_ConfigPathBaked is the regression test for report Finding 4: when a configPath is
// passed, the installed hook script EXPORTS STAGEHAND_CONFIG=<configPath> before the exec line so that
// `hook exec` at commit time resolves the SAME config the user explicitly selected at
// `hook install --config <path>` time. Without this, `--config` passed to `hook install` was silently
// ignored — `hook exec` fell back to env/discovery and could resolve a DIFFERENT config.
func TestHookScript_ConfigPathBaked(t *testing.T) {
	// Non-empty configPath ⇒ the export line is present and well-formed.
	got := hookScript(false, "/special/path.toml")
	if !strings.Contains(got, "export STAGEHAND_CONFIG='/special/path.toml'\n") {
		t.Errorf("configPath not baked into script as a STAGEHAND_CONFIG export:\n%s", got)
	}
	// The exec line must STILL be present after the export.
	if !strings.Contains(got, `exec stagehand hook exec "$@"`) {
		t.Errorf("exec line missing after config bake:\n%s", got)
	}
	// The export must come BEFORE the exec line (so the env is set when stagehand runs).
	exportIdx := strings.Index(got, "export STAGEHAND_CONFIG=")
	execIdx := strings.Index(got, "exec stagehand")
	if exportIdx < 0 || execIdx < 0 || exportIdx > execIdx {
		t.Errorf("export must precede exec; exportIdx=%d execIdx=%d\n%s", exportIdx, execIdx, got)
	}

	// configPath="" ⇒ NO export line (the default no-op case is byte-identical to the old behavior).
	noCfg := hookScript(false, "")
	if strings.Contains(noCfg, "STAGEHAND_CONFIG") {
		t.Errorf("empty configPath must NOT emit a STAGEHAND_CONFIG line:\n%s", noCfg)
	}

	// Paths with single quotes are escaped safely (POSIX single-quote-escape idiom).
	quoted := hookScript(false, "/path/with/'quote.toml")
	if !strings.Contains(quoted, `export STAGEHAND_CONFIG='/path/with/'\''quote.toml'`+"\n") {
		t.Errorf("single-quote path not safely escaped:\n%s", quoted)
	}
}

// TestHookScript_ConfigPathBaked_POSIX verifies the config-baked script parses under sh -n (the export
// line is POSIX-sh-clean, including for paths with spaces).
func TestHookScript_ConfigPathBaked_POSIX(t *testing.T) {
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not found on PATH; skipping POSIX syntax check")
	}
	for _, tc := range []struct {
		name   string
		path   string
		strict bool
	}{
		{"non-strict plain", "/cfg.toml", false},
		{"strict plain", "/cfg.toml", true},
		{"path with space", "/my config/cfg.toml", false},
		{"path with quote", "/cfg/'weird.toml", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := hookScript(tc.strict, tc.path)
			f := filepath.Join(t.TempDir(), "prepare-commit-msg")
			if err := os.WriteFile(f, []byte(script), ScriptMode); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
			}
			cmd := exec.Command(shPath, "-n", f)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("sh -n failed on config-baked script: %v\n%s\nscript:\n%s", err, out, script)
			}
		})
	}
}
