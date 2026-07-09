package provider

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// providerFiles — the 8 shipped reference manifests (PRD §14 + §12.5.1 + §12.5.2), each decode-parity-checked against its
// compiled-in built-in. repoPath is relative to the repo root (providers/<name>.toml).
var providerFiles = []struct {
	name     string
	repoPath string
}{
	{"pi", "providers/pi.toml"},
	{"claude", "providers/claude.toml"},
	{"opencode", "providers/opencode.toml"},
	{"codex", "providers/codex.toml"},
	{"cursor", "providers/cursor.toml"},
	{"agy", "providers/agy.toml"},
	{"qwen-code", "providers/qwen-code.toml"},
}

// repoRoot returns the repository root directory (two levels up from this file:
// internal/provider/referencefiles_test.go -> repo root).
func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// TestProviderReferenceFiles_DecodeParity reads each providers/<name>.toml, decodes it via
// toml.Unmarshal, and asserts it equals the corresponding BuiltinManifests() entry.
// This is the sync-guard that proves the reference docs never drift from the compiled-in code.
func TestProviderReferenceFiles_DecodeParity(t *testing.T) {
	root := repoRoot()
	builtins := BuiltinManifests()

	for _, tc := range providerFiles {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, tc.repoPath))
			if err != nil {
				t.Fatalf("failed to read %s: %v", tc.repoPath, err)
			}

			var decoded Manifest
			if err := toml.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("failed to decode %s: %v", tc.repoPath, err)
			}

			want, ok := builtins[tc.name]
			if !ok {
				t.Fatalf("BuiltinManifests() has no entry for %q", tc.name)
			}

			if !reflect.DeepEqual(decoded, want) {
				t.Errorf("decoded %s does not match builtin\n  built-in: %+v\n  decoded:  %+v", tc.name, want, decoded)
			}
		})
	}
}

// TestProviderReferenceFiles_AllBuiltinsCovered asserts that every entry in BuiltinManifests()
// has a corresponding reference .toml file. A 7th builtin added later without a .toml is caught here.
func TestProviderReferenceFiles_AllBuiltinsCovered(t *testing.T) {
	builtins := BuiltinManifests()
	tomlNames := make(map[string]bool)
	for _, pf := range providerFiles {
		tomlNames[pf.name] = true
	}

	for name := range builtins {
		if !tomlNames[name] {
			t.Errorf("BuiltinManifests() has entry %q but no reference file in providerFiles", name)
		}
	}

	for _, pf := range providerFiles {
		if _, ok := builtins[pf.name]; !ok {
			t.Errorf("providerFiles has entry %q but BuiltinManifests() has no such builtin", pf.name)
		}
	}
}
