package provider

// Builtins returns the six compiled-in provider manifests — pi, claude, gemini,
// opencode, codex, and cursor — that ship with stagehand so the v1 binary works
// against six real agents with zero configuration. The field values are the
// verified defaults from external_deps.md §B.1–B.6 (cross-checked against live
// `--help` on 2026-06-30) with the four §C corrections applied:
//
//   - §C.1: claude carries the full five-flag bare_flags set including
//     --disable-slash-commands and --no-chrome (encoded as a 7-element slice
//     because --setting-sources and --tools each take an empty-string value).
//   - §C.2: codex uses prompt_delivery="stdin" (NOT positional) and adds
//     --ephemeral.
//   - §C.3: gemini is positional delivery with an empty print_flag.
//   - §C.4: cursor is unchanged (command/detect "agent", --mode ask --trust).
//
// This map is the single source of truth for the provider system. The registry
// (M2.T3.S2) clones it and merges user overrides field-by-field; the config
// override base (M5.T3.S1) layers user [provider.<name>] sections onto it; and
// the reference-manifest emitter (M8.T1.S1) writes it to disk as TOML. A fresh
// map is returned on every call so those downstream merges can never mutate
// shared backing state. To add a provider not covered here, define it under
// [provider.<name>] in the user config — see PRD §12.8.
func Builtins() map[string]Manifest {
	return map[string]Manifest{
		"pi": {
			Name:             "pi",
			Detect:           "pi",
			Command:          "pi",
			PromptDelivery:   DeliveryStdin,
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
			Output:           OutputRaw,
			StripCodeFence:   true,
			RetryInstruction: "Output ONLY the commit message. No preamble, no markdown, no quotes.",
		},
		"claude": {
			Name:             "claude",
			Detect:           "claude",
			Command:          "claude",
			PromptDelivery:   DeliveryStdin,
			PrintFlag:        "-p",
			ModelFlag:        "--model",
			DefaultModel:     "sonnet",
			SystemPromptFlag: "--system-prompt",
			ProviderFlag:     "",
			// Five bare flags (§C correction #1); note the empty-string values
			// after --setting-sources and --tools, which Render must keep.
			BareFlags: []string{
				"--setting-sources", "",
				"--tools", "",
				"--disable-slash-commands", "--no-chrome", "--no-session-persistence",
			},
			Output:         OutputRaw,
			StripCodeFence: true,
		},
		"gemini": {
			Name:             "gemini",
			Detect:           "gemini",
			Command:          "gemini",
			PromptDelivery:   DeliveryPositional,
			PrintFlag:        "",
			ModelFlag:        "-m",
			DefaultModel:     "gemini-2.5-pro",
			SystemPromptFlag: "", // none → prepend to payload
			ProviderFlag:     "",
			BareFlags:        []string{"--approval-mode", "default"},
			Output:           OutputRaw,
			StripCodeFence:   true,
		},
		"opencode": {
			Name:             "opencode",
			Detect:           "opencode",
			Command:          "opencode",
			Subcommand:       []string{"run"},
			PromptDelivery:   DeliveryPositional,
			PrintFlag:        "",
			ModelFlag:        "-m",
			DefaultModel:     "",
			SystemPromptFlag: "", // none → prepend to payload
			ProviderFlag:     "",
			BareFlags:        nil,
			Output:           OutputRaw,
			StripCodeFence:   true,
		},
		"codex": {
			Name:             "codex",
			Detect:           "codex",
			Command:          "codex",
			Subcommand:       []string{"exec"},
			PromptDelivery:   DeliveryStdin, // §C correction #2: positional→stdin
			PrintFlag:        "",
			ModelFlag:        "-m",
			DefaultModel:     "",
			SystemPromptFlag: "", // none → prepend to payload
			ProviderFlag:     "",
			BareFlags: []string{
				"--sandbox", "read-only",
				"--ask-for-approval", "never",
				"--ephemeral", // §C correction #2: added
			},
			Output:         OutputRaw,
			StripCodeFence: true,
		},
		"cursor": {
			Name:             "cursor",
			Detect:           "agent",
			Command:          "agent",
			PromptDelivery:   DeliveryPositional,
			PrintFlag:        "-p",
			ModelFlag:        "--model",
			DefaultModel:     "",
			SystemPromptFlag: "", // none → prepend to payload
			ProviderFlag:     "",
			BareFlags:        []string{"--mode", "ask", "--trust"},
			Output:           OutputRaw,
			StripCodeFence:   true,
		},
	}
}
