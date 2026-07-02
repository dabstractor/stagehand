package provider

import (
	"bytes"
	"reflect"
	"sort"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// normalizeEmpty collapses a zero-length slice to nil. go-toml/v2 unmarshals an
// absent or empty TOML array into a non-nil []string{}, so a raw reflect.DeepEqual
// across a marshal/unmarshal boundary would spuriously fail for manifests whose
// Subcommand or BareFlags is nil (opencode, pi). Normalizing both sides first
// makes nil and []string{} compare equal.
func normalizeEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

// manifestsEqual compares two Manifests after normalizing their nil/empty
// slices, the only field where a TOML round-trip is not identity-preserving.
func manifestsEqual(a, b Manifest) bool {
	a.Subcommand = normalizeEmpty(a.Subcommand)
	b.Subcommand = normalizeEmpty(b.Subcommand)
	a.BareFlags = normalizeEmpty(a.BareFlags)
	b.BareFlags = normalizeEmpty(b.BareFlags)
	return reflect.DeepEqual(a, b)
}

// contains reports whether ss contains v.
func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

// TestBuiltins_KeysAndCount asserts Builtins returns exactly the six expected
// provider keys and a fresh, independently-addressable map on every call (the
// registry, M2.T3.S2, mutates its own merge copy and must never touch a shared
// backing map).
func TestBuiltins_KeysAndCount(t *testing.T) {
	m := Builtins()

	if got := len(m); got != 6 {
		t.Fatalf("len(Builtins()) = %d, want 6", got)
	}

	wantKeys := map[string]struct{}{
		"pi": {}, "claude": {}, "gemini": {}, "opencode": {}, "codex": {}, "cursor": {},
	}
	gotKeys := make(map[string]struct{}, len(m))
	for k := range m {
		gotKeys[k] = struct{}{}
	}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Errorf("keys = %v, want %v", gotKeys, wantKeys)
	}

	// Two calls yield deeply-equal maps but distinct allocations: mutating one
	// must not bleed into the other. Map-indexed values are not addressable, so
	// take a local copy, mutate it, and write it back.
	a, b := Builtins(), Builtins()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("two Builtins() calls differ")
	}
	piA := a["pi"]
	piA.DefaultModel = "mutated-by-caller-A"
	a["pi"] = piA
	if b["pi"].DefaultModel == "mutated-by-caller-A" {
		t.Error("mutating a value from one Builtins() call leaked into another call's map")
	}
}

// TestBuiltins_MatchesOracle asserts Builtins() equals an INDEPENDENT copy of
// the external_deps.md §B.1–B.6 manifests (with the four §C corrections). This
// literal is deliberately kept separate from sixBuiltinManifests() (which now
// delegates to Builtins()) so it remains a genuine cross-check rather than a
// tautology.
func TestBuiltins_MatchesOracle(t *testing.T) {
	want := map[string]Manifest{
		"pi": {
			Name:             "pi",
			Detect:           "pi",
			Command:          "pi",
			PromptDelivery:   "stdin",
			PrintFlag:        "-p",
			ModelFlag:        "--model",
			DefaultModel:     "glm-5-turbo",
			SystemPromptFlag: "--system-prompt",
			ProviderFlag:     "--provider",
			DefaultProvider:  "",
			BareFlags: []string{
				"--no-tools", "--no-extensions", "--no-skills",
				"--no-prompt-templates", "--no-context-files", "--no-session",
			},
			Output:           "raw",
			StripCodeFence:   true,
			RetryInstruction: "Output ONLY the commit message. No preamble, no markdown, no quotes.",
		},
		"claude": {
			Name:             "claude",
			Detect:           "claude",
			Command:          "claude",
			PromptDelivery:   "stdin",
			PrintFlag:        "-p",
			ModelFlag:        "--model",
			DefaultModel:     "sonnet",
			SystemPromptFlag: "--system-prompt",
			ProviderFlag:     "",
			BareFlags: []string{
				"--setting-sources", "",
				"--tools", "",
				"--disable-slash-commands", "--no-chrome", "--no-session-persistence",
			},
			Output:         "raw",
			StripCodeFence: true,
		},
		"gemini": {
			Name:             "gemini",
			Detect:           "gemini",
			Command:          "gemini",
			PromptDelivery:   "positional",
			PrintFlag:        "",
			ModelFlag:        "-m",
			DefaultModel:     "gemini-2.5-pro",
			SystemPromptFlag: "",
			ProviderFlag:     "",
			BareFlags:        []string{"--approval-mode", "default"},
			Output:           "raw",
			StripCodeFence:   true,
		},
		"opencode": {
			Name:             "opencode",
			Detect:           "opencode",
			Command:          "opencode",
			Subcommand:       []string{"run"},
			PromptDelivery:   "positional",
			PrintFlag:        "",
			ModelFlag:        "-m",
			DefaultModel:     "",
			SystemPromptFlag: "",
			ProviderFlag:     "",
			BareFlags:        nil,
			Output:           "raw",
			StripCodeFence:   true,
		},
		"codex": {
			Name:             "codex",
			Detect:           "codex",
			Command:          "codex",
			Subcommand:       []string{"exec"},
			PromptDelivery:   "stdin", // §C.2 correction
			PrintFlag:        "",
			ModelFlag:        "-m",
			DefaultModel:     "",
			SystemPromptFlag: "",
			ProviderFlag:     "",
			BareFlags: []string{
				"--sandbox", "read-only",
				"--ask-for-approval", "never",
				"--ephemeral", // §C.2 correction
			},
			Output:         "raw",
			StripCodeFence: true,
		},
		"cursor": {
			Name:             "cursor",
			Detect:           "agent",
			Command:          "agent",
			PromptDelivery:   "positional",
			PrintFlag:        "-p",
			ModelFlag:        "--model",
			DefaultModel:     "",
			SystemPromptFlag: "",
			ProviderFlag:     "",
			BareFlags:        []string{"--mode", "ask", "--trust"},
			Output:           "raw",
			StripCodeFence:   true,
		},
	}

	got := Builtins()
	if !reflect.DeepEqual(got, want) {
		// Report which keys differ for easier diagnosis.
		all := make(map[string]struct{}, len(want))
		for k := range want {
			all[k] = struct{}{}
		}
		for k := range got {
			all[k] = struct{}{}
		}
		for k := range all {
			gm, wok := got[k], want[k]
			if !reflect.DeepEqual(gm, wok) {
				t.Errorf("manifest %q mismatch:\n  got  %+v\n  want %+v", k, gm, wok)
			}
		}
	}
}

// TestBuiltins_TOMLRoundTrip asserts every built-in manifest survives a
// go-toml/v2 marshal→unmarshal cycle. This validates the toml struct tags on
// Manifest (which the future config loader, M5, depends on) at the cheapest
// possible point. Two complementary assertions: (1) structural equality after
// normalizing nil/empty slices, and (2) byte idempotency of the re-marshaled
// result (immune to the nil→[]string{} quirk).
func TestBuiltins_TOMLRoundTrip(t *testing.T) {
	// Deterministic key order for stable subtest names.
	names := make([]string, 0, len(Builtins()))
	for k := range Builtins() {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			orig := Builtins()[name]

			buf, err := toml.Marshal(orig)
			if err != nil {
				t.Fatalf("toml.Marshal(%q): unexpected error: %v", name, err)
			}

			var back Manifest
			if err := toml.Unmarshal(buf, &back); err != nil {
				t.Fatalf("toml.Unmarshal(%q): unexpected error: %v\nTOML:\n%s", name, err, buf)
			}

			if !manifestsEqual(orig, back) {
				t.Errorf("%q: round-trip not equal after normalization\n  orig %+v\n  back %+v\nTOML:\n%s",
					name, orig, back, buf)
			}

			// Idempotency: re-marshaling the round-tripped value yields the
			// same bytes as the original marshal. This is independent of the
			// nil/empty slice quirk.
			buf2, err := toml.Marshal(back)
			if err != nil {
				t.Fatalf("re-toml.Marshal(%q): unexpected error: %v", name, err)
			}
			if !bytes.Equal(buf, buf2) {
				t.Errorf("%q: marshal is not idempotent\n first: %q\nsecond: %q", name, buf, buf2)
			}
		})
	}
}

// TestBuiltins_SpotChecks pins the four §C corrections that improve fidelity
// over the PRD's illustrative TOML.
func TestBuiltins_SpotChecks(t *testing.T) {
	m := Builtins()
	claude, codex, gemini, cursor := m["claude"], m["codex"], m["gemini"], m["cursor"]

	t.Run("claude_5_flags_7_slice_elements", func(t *testing.T) {
		// §C.1: the slice encodes 5 logical flags as 7 elements (two flags take
		// empty-string values). Asserting len==7, NOT 5.
		if got := len(claude.BareFlags); got != 7 {
			t.Errorf("len(claude.BareFlags) = %d, want 7", got)
		}
		if !contains(claude.BareFlags, "--disable-slash-commands") {
			t.Errorf("claude.BareFlags = %#v, want it to contain --disable-slash-commands", claude.BareFlags)
		}
		if !contains(claude.BareFlags, "--no-chrome") {
			t.Errorf("claude.BareFlags = %#v, want it to contain --no-chrome", claude.BareFlags)
		}
	})

	t.Run("codex_stdin_and_ephemeral", func(t *testing.T) {
		// §C.2: prompt_delivery corrected from "positional" to "stdin".
		if codex.PromptDelivery != DeliveryStdin {
			t.Errorf("codex.PromptDelivery = %q, want %q", codex.PromptDelivery, DeliveryStdin)
		}
		if !contains(codex.BareFlags, "--ephemeral") {
			t.Errorf("codex.BareFlags = %#v, want it to contain --ephemeral", codex.BareFlags)
		}
	})

	t.Run("gemini_empty_print_flag", func(t *testing.T) {
		// §C.3: gemini has no print flag.
		if gemini.PrintFlag != "" {
			t.Errorf("gemini.PrintFlag = %q, want \"\"", gemini.PrintFlag)
		}
	})

	t.Run("cursor_command_and_detect_agent", func(t *testing.T) {
		// §C.4: cursor unchanged — its executable is "agent".
		if cursor.Command != "agent" {
			t.Errorf("cursor.Command = %q, want %q", cursor.Command, "agent")
		}
		if cursor.Detect != "agent" {
			t.Errorf("cursor.Detect = %q, want %q", cursor.Detect, "agent")
		}
	})
}
