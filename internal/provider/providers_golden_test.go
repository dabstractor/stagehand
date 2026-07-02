package provider

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// TestProvidersTOML_MatchBuiltins is the golden-equivalence guard (P1.M8.T1.S1)
// for the contributor-facing reference manifests shipped at repo-root
// providers/<name>.toml. Each file must decode (go-toml/v2) to the SAME
// provider.Manifest that internal/provider/builtin.go compiles into the binary,
// so the on-disk reference copies and the built-ins can never silently drift.
//
// The built-ins remain authoritative: a missing, malformed, or divergent file
// FAILS the subtest (it is never skipped). nil/empty normalization for the
// Subcommand and BareFlags slices (reusing normalizeEmpty from builtin_test.go)
// and for the Env map (defensive — all six built-ins have nil Env and the .toml
// files omit the [env] table) accounts for the go-toml/v2 decode quirk where an
// absent array/table is nil but an explicit empty one is not.
//
// `go test` sets CWD to the package directory, so "../../providers" resolves to
// the repo-root providers/ directory (PRD §14).
func TestProvidersTOML_MatchBuiltins(t *testing.T) {
	dir := filepath.Join("..", "..", "providers")

	builtins := Builtins()
	// Deterministic key order for stable subtest names.
	names := make([]string, 0, len(builtins))
	for name := range builtins {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, name+".toml"))
			if err != nil {
				t.Fatalf("missing providers/%s.toml: %v", name, err)
			}

			var got Manifest
			if err := toml.Unmarshal(raw, &got); err != nil {
				t.Fatalf("decode providers/%s.toml: %v", name, err)
			}

			want := builtins[name]
			// Collapse nil and []string{} so a TOML round-trip compares equal
			// (go-toml/v2 decodes an absent/empty array as []string{}).
			got.Subcommand = normalizeEmpty(got.Subcommand)
			want.Subcommand = normalizeEmpty(want.Subcommand)
			got.BareFlags = normalizeEmpty(got.BareFlags)
			want.BareFlags = normalizeEmpty(want.BareFlags)
			// Defensive Env normalization: absent table → nil, present-empty
			// table → map[string]string{}; the .toml files omit [env] entirely.
			if len(got.Env) == 0 {
				got.Env = nil
			}
			if len(want.Env) == 0 {
				want.Env = nil
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("providers/%s.toml decodes to a manifest that differs from the compiled builtin\n  got  %+v\n  want %+v", name, got, want)
			}
		})
	}
}
