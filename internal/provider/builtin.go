package provider

// BuiltinManifests returns the compiled-in default provider manifests (PRD §12.3 pi, §12.4 claude,
// §12.5 gemini, §12.6 opencode), keyed by manifest name. These are the zero-config defaults a user
// override (config [provider.<name>]) merges onto via MergeManifest (S2) in the registry (P1.M2.T3).
//
// The manifests are constructed FRESH on every call (no package-level var): strPtr/boolPtr allocate new
// pointers and slice literals allocate new backing arrays each call, so no caller can corrupt a built-in
// by mutating a returned BareFlags/Env. MergeManifest (S2) already never mutates base, so the normal
// registry path is safe either way — fresh-per-call additionally guards against any direct mutation.
//
// This subtask adds gemini + opencode (§12.7.1 "read-only constraint" providers: no global tool-disable
// switch; constrained to read-only, never-ask profiles). pi + claude (the "explicit tool-disable switch"
// pair) landed in S1. The remaining two (codex, cursor) are added by S3.
func BuiltinManifests() map[string]Manifest {
	return map[string]Manifest{
		"pi":       builtinPi(),
		"claude":   builtinClaude(),
		"gemini":   builtinGemini(),
		"opencode": builtinOpenCode(),
	}
}

// builtinPi returns the pi manifest per PRD §12.3 (FULLY VERIFIED vs `pi --help`, external_deps.md §pi).
// Rendered with provider="zai", model=default, sys set, it reproduces the commit-pi invocation
// byte-for-byte (see TestBuiltinManifests_RenderedCommand_Pi_MatchesCommitPi).
//
// NOTE the explicit-empty DefaultProvider: §12.3 WRITES `default_provider = ""` (non-nil *""), meaning
// "do not add --provider unless the user configures one." This is NOT the same as a nil DefaultProvider
// (absent key) — the decode-parity test enforces the distinction.
func builtinPi() Manifest {
	return Manifest{
		Name:             "pi",
		Detect:           strPtr("pi"),
		Command:          strPtr("pi"),
		PromptDelivery:   strPtr("stdin"),
		PrintFlag:        strPtr("-p"),
		ModelFlag:        strPtr("--model"),
		DefaultModel:     strPtr("glm-5-turbo"),
		SystemPromptFlag: strPtr("--system-prompt"),
		ProviderFlag:     strPtr("--provider"),
		DefaultProvider:  strPtr(""), // §12.3 explicit empty (NON-NIL) — user sets e.g. "zai"
		BareFlags: []string{
			"--no-tools",
			"--no-extensions",
			"--no-skills",
			"--no-prompt-templates",
			"--no-context-files",
			"--no-session",
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// Subcommand, PromptFlag, JsonField, RetryInstruction, Env: nil (absent in §12.3).
	}
}

// builtinClaude returns the claude manifest per PRD §12.4 (VERIFIED vs `claude --help`, external_deps.md
// §claude). claude disables tools via `--tools ""` (documented "Use \"\" to disable all tools") and
// settings via `--setting-sources ""`; `--no-session-persistence` makes it ephemeral (valid only with -p).
//
// NOTE: (1) ProviderFlag is strPtr("") — §12.4 WRITES `provider_flag = "" # n/a` (non-nil empty); the
// §12.2 renderer's `if provider_flag and provider` is therefore false → no --provider emitted (claude has
// no sub-provider concept). (2) DefaultProvider is NIL — §12.4 OMITS the key entirely (do NOT set it).
// (3) BareFlags has TWO "" value tokens (the args to --tools / --setting-sources) — do NOT drop them.
func builtinClaude() Manifest {
	return Manifest{
		Name:             "claude",
		Detect:           strPtr("claude"),
		Command:          strPtr("claude"),
		PromptDelivery:   strPtr("stdin"),
		PrintFlag:        strPtr("-p"),
		ModelFlag:        strPtr("--model"),
		DefaultModel:     strPtr("sonnet"),
		SystemPromptFlag: strPtr("--system-prompt"),
		ProviderFlag:     strPtr(""), // §12.4 explicit empty (NON-NIL) — n/a for claude
		BareFlags: []string{
			"--tools", "", // disable ALL built-in tools (value arg = "")
			"--setting-sources", "", // load no settings sources (value arg = "")
			"--no-session-persistence", // ephemeral (only valid with -p)
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// Subcommand, PromptFlag, JsonField, RetryInstruction, Env, DefaultProvider: nil (absent in §12.4).
	}
}

// builtinGemini returns the gemini manifest per PRD §12.5 (VERIFIED vs `gemini --help`, external_deps.md
// §gemini), with prompt_delivery REVISED to "stdin" per the work-item contract (external_deps.md §gemini
// recommendation + Appendix E #1: stdin avoids arg-length limits on ~300 KB diffs; gemini appends stdin
// to the prompt). gemini has no global tool-disable switch; `--approval-mode default` constrains it to a
// read-only, never-ask profile (§12.7.1 "read-only constraint").
//
// NOTE: (1) PrintFlag/SystemPromptFlag/ProviderFlag are strPtr("") — §12.5 WRITES them "" (NON-NIL empty):
// no print flag (positional/stdin implies one-shot), no sys-prompt flag (sys PREPENDED to the payload per
// §12.2), no sub-provider. (2) DefaultProvider is NIL — §12.5 OMITS the key (do NOT set it). (3) The sys
// prompt is prepended to the payload (no --system-prompt flag exists on gemini-cli).
func builtinGemini() Manifest {
	return Manifest{
		Name:             "gemini",
		Detect:           strPtr("gemini"),
		Command:          strPtr("gemini"),
		PromptDelivery:   strPtr("stdin"), // REVISED from §12.5 "positional" (work-item + external_deps.md + Appx E #1)
		PrintFlag:        strPtr(""),      // §12.5 explicit empty (NON-NIL) — positional/stdin implies one-shot
		ModelFlag:        strPtr("-m"),
		DefaultModel:     strPtr("gemini-2.5-pro"),
		SystemPromptFlag: strPtr(""), // §12.5 explicit empty (NON-NIL) — no sys flag; sys prepended to payload (§12.2)
		ProviderFlag:     strPtr(""), // §12.5 explicit empty (NON-NIL) — gemini has no sub-provider
		BareFlags: []string{
			"--approval-mode", "default", // read-only, never-ask profile (don't auto-run tools)
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// Subcommand, PromptFlag, DefaultProvider, JsonField, RetryInstruction, Env: nil (absent in §12.5).
	}
}

// builtinOpenCode returns the opencode manifest per PRD §12.6 (VERIFIED vs `opencode run --help`,
// external_deps.md §opencode), VERBATIM (no revisions). opencode `run` is non-interactive and prints the
// final message to stdout. It has no global tool-disable switch and no bare flags — `run` is already a
// read-only, non-interactive one-shot (§12.7.1 "read-only constraint").
//
// NOTE: (1) Subcommand = ["run"] (§12.6 writes subcommand = ["run"] → NON-NIL 1-element). (2)
// PrintFlag/DefaultModel/SystemPromptFlag/ProviderFlag are strPtr("") — §12.6 WRITES them "" (NON-NIL
// empty): `run` is already non-interactive (no print flag), user MUST set model (no single sensible
// default — model space is huge), no sys-prompt flag (sys prepended), provider is part of the model string.
// (3) BareFlags = []string{} — §12.6 writes bare_flags = []; a present empty array decodes NON-NIL empty
// (FINDING D). (4) DefaultProvider is NIL — §12.6 OMITS the key.
func builtinOpenCode() Manifest {
	return Manifest{
		Name:             "opencode",
		Detect:           strPtr("opencode"),
		Command:          strPtr("opencode"),
		Subcommand:       []string{"run"}, // §12.6 `subcommand = ["run"]` → NON-NIL 1-element slice
		PromptDelivery:   strPtr("positional"),
		PrintFlag:        strPtr(""), // §12.6 explicit empty (NON-NIL) — `run` is already non-interactive
		ModelFlag:        strPtr("-m"),
		DefaultModel:     strPtr(""), // §12.6 explicit empty (NON-NIL) — user MUST set model (Appx E #3)
		SystemPromptFlag: strPtr(""), // §12.6 explicit empty (NON-NIL) — no sys flag on `run`; sys prepended (§12.2)
		ProviderFlag:     strPtr(""), // §12.6 explicit empty (NON-NIL) — provider is part of the model string
		BareFlags:        []string{}, // §12.6 `bare_flags = []` → NON-NIL empty slice (FINDING D); do NOT omit
		Output:           strPtr("raw"),
		StripCodeFence:   boolPtr(true),
		// PromptFlag, DefaultProvider, JsonField, RetryInstruction, Env: nil (absent in §12.6).
	}
}
