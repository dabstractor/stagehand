package provider

// BuiltinManifests returns the compiled-in default provider manifests (PRD §12.3 pi + §12.4 claude),
// keyed by manifest name. These are the zero-config defaults a user override (config [provider.<name>])
// merges onto via MergeManifest (S2) in the registry (P1.M2.T3).
//
// The manifests are constructed FRESH on every call (no package-level var): strPtr/boolPtr allocate new
// pointers and slice literals allocate new backing arrays each call, so no caller can corrupt a built-in
// by mutating a returned BareFlags/Env. MergeManifest (S2) already never mutates base, so the normal
// registry path is safe either way — fresh-per-call additionally guards against any direct mutation.
//
// This subtask lands pi + claude (the §12.7.1 "explicit tool-disable switch" providers). The remaining
// four (gemini, opencode, codex, cursor — the read-only-constrained providers) are added by S2/S3.
func BuiltinManifests() map[string]Manifest {
	return map[string]Manifest{
		"pi":     builtinPi(),
		"claude": builtinClaude(),
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
