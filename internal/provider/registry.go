package provider

import (
	"os/exec"
	"sort"
)

// Registry is a precomputed, field-by-field merge of the six built-in provider
// manifests (decisions.md §6) with optional user overrides, plus an os/exec
// based Detect of which providers are installed on $PATH. It is the merge
// engine behind config Load (P1.M5.T3.S1) and the backing store for the CLI
// providers list/show subcommands (P1.M7.T3.S1, FR46–FR48).
//
// To preserve the no-import-cycle invariant (plan_overview key decision 1),
// Registry is constructed with the resolved overrides injected directly as a
// map[string]Manifest; this package NEVER imports internal/config. The merge
// is computed once, at construction, so Get/List/Detect are pure lookups over
// a private map that the registry fully owns: every caller-supplied map and
// slice is deep-copied, so neither Builtins() nor the overrides map can be
// aliased or mutated through the registry.
type Registry struct {
	merged map[string]Manifest
}

// NewRegistry builds a Registry by cloning each built-in manifest and then
// layering the overrides field-by-field per decisions.md §6. When an override
// shares a name with a built-in, its set fields replace ONLY those fields — an
// override setting default_model alone leaves bare_flags, print_flag, etc.
// intact. An override naming a provider absent from the builtins is added as a
// brand-new provider used as-is (FR48). The returned registry owns deep copies
// of every map and slice, so the builtins and overrides maps are never aliased
// or mutated.
func NewRegistry(builtins, overrides map[string]Manifest) *Registry {
	merged := make(map[string]Manifest, len(builtins)+len(overrides))
	for name, m := range builtins {
		merged[name] = cloneManifest(m)
	}
	for name, ov := range overrides {
		if base, ok := merged[name]; ok {
			merged[name] = mergeManifest(base, ov)
		} else {
			merged[name] = cloneManifest(ov)
		}
	}
	return &Registry{merged: merged}
}

// cloneManifest returns a value copy of m in which the slice and map fields
// point at fresh allocations, so mutating them after the call cannot bleed back
// into the source (mirrors the fresh-map philosophy of Builtins). Scalar
// strings and the StripCodeFence bool are value-copied by the struct copy.
func cloneManifest(m Manifest) Manifest {
	out := m // value copy: scalars, bool, and slice/map headers
	if m.Subcommand != nil {
		out.Subcommand = append([]string(nil), m.Subcommand...)
	}
	if m.BareFlags != nil {
		out.BareFlags = append([]string(nil), m.BareFlags...)
	}
	if m.Env != nil {
		ne := make(map[string]string, len(m.Env))
		for k, v := range m.Env {
			ne[k] = v
		}
		out.Env = ne
	}
	return out
}

// mergeManifest applies the override ov onto base per decisions.md §6: every
// non-empty scalar string field on ov replaces base's, the slice/map fields are
// replaced WHOLESALE when non-empty (Env is never deep-merged), and the result
// is a fresh, independently-owned Manifest. The slices/maps from ov are copied
// into the result, so the registry never aliases the caller's override state.
//
// StripCodeFence is a bool whose Go zero value is false, so an override can
// turn it ON (true) but cannot force it OFF (false) over a base that already
// set it true via this field-by-field merge. This is a documented v1 limitation.
func mergeManifest(base, ov Manifest) Manifest {
	out := cloneManifest(base)
	if ov.Name != "" {
		out.Name = ov.Name
	}
	if ov.Detect != "" {
		out.Detect = ov.Detect
	}
	if ov.Command != "" {
		out.Command = ov.Command
	}
	if ov.PromptDelivery != "" {
		out.PromptDelivery = ov.PromptDelivery
	}
	if ov.PromptFlag != "" {
		out.PromptFlag = ov.PromptFlag
	}
	if ov.PrintFlag != "" {
		out.PrintFlag = ov.PrintFlag
	}
	if ov.ModelFlag != "" {
		out.ModelFlag = ov.ModelFlag
	}
	if ov.DefaultModel != "" {
		out.DefaultModel = ov.DefaultModel
	}
	if ov.SystemPromptFlag != "" {
		out.SystemPromptFlag = ov.SystemPromptFlag
	}
	if ov.ProviderFlag != "" {
		out.ProviderFlag = ov.ProviderFlag
	}
	if ov.DefaultProvider != "" {
		out.DefaultProvider = ov.DefaultProvider
	}
	if ov.Output != "" {
		out.Output = ov.Output
	}
	if ov.JSONField != "" {
		out.JSONField = ov.JSONField
	}
	if ov.RetryInstruction != "" {
		out.RetryInstruction = ov.RetryInstruction
	}
	if len(ov.Subcommand) > 0 {
		out.Subcommand = append([]string(nil), ov.Subcommand...)
	}
	if len(ov.BareFlags) > 0 {
		out.BareFlags = append([]string(nil), ov.BareFlags...)
	}
	if len(ov.Env) > 0 {
		ne := make(map[string]string, len(ov.Env))
		for k, v := range ov.Env {
			ne[k] = v
		}
		out.Env = ne
	}
	// StripCodeFence: zero value is false, so an override can only flip it
	// true, never false-over-true (documented v1 limitation, decisions.md §6).
	if ov.StripCodeFence {
		out.StripCodeFence = true
	}
	return out
}

// Get returns the fully-resolved manifest for name, and ok=false if name is not
// a known provider. The returned Manifest is a value copy whose slices/maps
// still reference the registry's owned allocations; callers may read freely.
// Used by FR47 (providers show) and the config loader (M5).
func (r *Registry) Get(name string) (Manifest, bool) {
	m, ok := r.merged[name]
	return m, ok
}

// List returns the names of every provider in the registry, sorted with no
// duplicates (FR46 providers list). The returned slice is freshly allocated.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.merged))
	for name := range r.merged {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Detect reports, for each provider name, whether the provider's executable is
// resolvable on $PATH via exec.LookPath (FR46 detect). The executable probed is
// the manifest's Detect field, falling back to Command when Detect is empty
// (PRD §12.1); a provider with both empty is reported as not installed. This is
// the only method that performs I/O — Get/List/merge remain pure.
func (r *Registry) Detect() map[string]bool {
	result := make(map[string]bool, len(r.merged))
	for name, m := range r.merged {
		target := m.Detect
		if target == "" {
			target = m.Command
		}
		if target == "" {
			result[name] = false
			continue
		}
		_, err := exec.LookPath(target)
		result[name] = err == nil
	}
	return result
}
