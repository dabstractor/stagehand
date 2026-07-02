package provider

import (
	"fmt"
	"os/exec"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

// preferredBuiltins is the FR-D1 cascading provider priority (PRD §9.16 FR-D1): open/self-hostable
// harnesses first (pi, opencode, cursor, agy), closed subscription CLIs last (gemini, codex, claude);
// pi first. DefaultProvider returns the first name in this list that the caller reports installed.
// It MUST stay in sync with BuiltinManifests() keys — a test (TestPreferredBuiltins_MatchesBuiltinKeys)
// enforces this. Only built-in names are candidates; user-defined §12.8 providers are never auto-selected.
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "qwen-code", "codex", "claude"}

// Registry holds the fully-merged provider manifests: the built-in defaults (BuiltinManifests, P1.M2.T2)
// overlaid field-by-field with user overrides via MergeManifest (S2) per PRD §16.1/§12.8. Brand-new §12.8
// provider names are added verbatim. It is the single read site for the CLI providers subcommands
// (P1.M4.T1.S3) and provider resolution in the generate flow (P1.M3.T4).
//
// NewRegistry does NOT Validate or Resolve — consumers call Validate then Resolve on the manifest they
// Get (lifecycle: decode → merge → [store] → Validate → Resolve → consume). This keeps the registry a
// pure data structure and lets `providers show` display a partially-defined provider for debugging.
type Registry struct {
	manifests map[string]Manifest
}

// NewRegistry builds a Registry seeded with BuiltinManifests(), then overlays each userOverride: if the
// name matches a built-in, MergeManifest (S2) field-merges it (PRD §16.1); otherwise the override is
// added verbatim as a brand-new provider (§12.8). In both branches the table key is set as the manifest
// Name (the [provider.<name>] body carries no "name" key). userOverrides are EXPECTED to be decoded
// Manifests — use DecodeUserOverrides to bridge from config.Providers (raw map). No Validate/Resolve.
func NewRegistry(userOverrides map[string]Manifest) *Registry {
	manifests := make(map[string]Manifest, len(userOverrides)+8) // built-ins + overrides headroom
	// Seed with a FRESH copy of the built-ins (BuiltinManifests constructs fresh each call; copying
	// localizes the no-shared-mutation guarantee).
	for name, m := range BuiltinManifests() {
		manifests[name] = m
	}
	// Overlay user overrides: merge onto a built-in, or add brand-new verbatim (§12.8).
	for name, override := range userOverrides {
		if base, ok := manifests[name]; ok {
			merged := MergeManifest(base, override) // S2: field-by-field; preserves base.Name; never mutates base
			merged.Name = name                      // table key is the authoritative identity
			manifests[name] = merged
		} else {
			override.Name = name // a §12.8 provider: identity comes from the table key
			manifests[name] = override
		}
	}
	return &Registry{manifests: manifests}
}

// Get returns the merged manifest for name and whether it exists in the registry.
func (r *Registry) Get(name string) (Manifest, bool) {
	m, ok := r.manifests[name]
	return m, ok
}

// List returns every manifest in the registry, sorted ascending by Name (deterministic for `providers
// list`). The returned slice is a fresh copy; the internal map is not exposed.
func (r *Registry) List() []Manifest {
	out := make([]Manifest, 0, len(r.manifests))
	for _, m := range r.manifests {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// IsInstalled reports whether the provider's discovery command is on $PATH (FR46). It probes
// m.DetectCommand() (Detect if set & non-empty, else Command) via exec.LookPath. A manifest with
// neither Detect nor Command set (DetectCommand()=="") reports false. NOTE cursor is the only built-in
// where Detect ≠ Name (Detect="agent" — the binary), so this correctly probes "agent", not "cursor".
func (r *Registry) IsInstalled(m Manifest) bool {
	cmd := m.DetectCommand()
	if cmd == "" {
		return false
	}
	_, err := exec.LookPath(cmd)
	return err == nil
}

// MarshalTOML returns the stored (merged) manifest for name as TOML (FR47 — providers show). It marshals
// the MERGED manifest (built-in ⊕ user overrides), NOT a Resolve() copy: go-toml OMITS nil *string/*bool
// pointers (free omitempty), so the output shows what is actually configured with absent fields
// suppressed. Returns a wrapped error if name is unknown.
func (r *Registry) MarshalTOML(name string) (string, error) {
	m, ok := r.manifests[name]
	if !ok {
		return "", fmt.Errorf("provider registry: unknown provider %q", name)
	}
	data, err := toml.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("provider registry: marshal %q: %w", name, err)
	}
	return string(data), nil
}

// DefaultProvider returns the first built-in (in preference order, pi first) that the caller reports
// installed, or "" if none of the preferred built-ins are installed (FR46 — "show the resolved default").
// installed is the caller's list of installed provider NAMES (computed via IsInstalled over List()).
// Taking it as a param keeps this pure/testable (no exec inside). Only built-in names are candidates;
// user-defined §12.8 providers are never auto-selected.
func (r *Registry) DefaultProvider(installed []string) string {
	present := make(map[string]struct{}, len(installed))
	for _, name := range installed {
		present[name] = struct{}{}
	}
	for _, name := range preferredBuiltins {
		if _, ok := present[name]; ok {
			return name
		}
	}
	return ""
}

// FirstTooledProvider returns the first built-in (in preference order, pi first) that the caller
// reports installed AND whose manifest has non-empty TooledFlags (i.e. can serve as the stager),
// or "" if none of the preferred built-ins are installed and stager-capable (FR-D4 — PRD §9.16).
// It mirrors DefaultProvider's structure but adds the TooledFlags filter: only pi and claude are
// stager-capable today (builtin.go). installed is the caller's list of installed provider NAMES
// (computed via IsInstalled over List()). Taking it as a param keeps this pure/testable (no exec inside).
// Only built-in names are candidates; user-defined §12.8 providers are never auto-selected.
func (r *Registry) FirstTooledProvider(installed []string) string {
	present := make(map[string]struct{}, len(installed))
	for _, name := range installed {
		present[name] = struct{}{}
	}
	for _, name := range preferredBuiltins {
		if _, ok := present[name]; !ok {
			continue
		}
		m, ok := r.Get(name)
		if !ok {
			continue
		}
		if len(m.TooledFlags) > 0 {
			return name
		}
	}
	return ""
}

// DecodeUserOverrides bridges config.Providers (raw map[string]map[string]any, P1.M1.T4) to the
// map[string]Manifest NewRegistry consumes. For each [provider.<name>] entry it re-encodes the raw map
// to TOML and unmarshals into a Manifest (the pattern the frozen config.go comment specifies for "the
// registry (P1.M2.T3)"), then sets Name from the table key (the TOML body carries no "name"). Returns a
// non-nil (possibly empty) map for nil/empty input. A malformed entry yields a wrapped error naming the
// key. This is the ONLY place that touches the raw config-map shape; the Registry works in typed
// Manifests. It does NOT import internal/config (it takes the type, not the package) — no import cycle.
func DecodeUserOverrides(raw map[string]map[string]any) (map[string]Manifest, error) {
	out := make(map[string]Manifest, len(raw))
	for name, entry := range raw {
		data, err := toml.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("provider override %q: re-encode to TOML: %w", name, err)
		}
		var m Manifest
		if err := toml.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("provider override %q: decode: %w", name, err)
		}
		m.Name = name // the table key is the identity; the body has no "name"
		out[name] = m
	}
	return out, nil
}
