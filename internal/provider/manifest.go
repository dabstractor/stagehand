// Package provider holds stagehand's agent-agnostic provider subsystem: the
// canonical Manifest schema (PRD §12.1) that describes how to invoke a given
// agent, and the pure Render method (PRD §12.2) that turns a resolved intent
// (model, provider, system prompt, user payload) into an exact argument slice,
// stdin payload, and environment additions — never a shell string (PRD §19).
//
// This file defines the foundational Manifest and Rendered types plus the
// Render algorithm. Sibling files added by later tasks supply the six
// built-in manifests (builtin.go), the config override merge (registry.go),
// the os/exec-based executor (executor.go), and the output parser (parse.go);
// those files carry a plain "package provider" line, leaving the package doc
// here.
package provider

import (
	"fmt"
	"sort"
)

// Enumerated field values for Manifest.PromptDelivery and Manifest.Output
// (PRD §12.1). Render treats an empty PromptDelivery as stdin, the §12.1
// default, so the empty string need not be referenced by callers.
const (
	// DeliveryStdin pipes the user payload to the process stdin. This is the
	// §12.1 default when prompt_delivery is omitted.
	DeliveryStdin = "stdin"
	// DeliveryPositional appends the user payload as the final positional arg.
	DeliveryPositional = "positional"
	// DeliveryFlag appends the user payload after Manifest.PromptFlag.
	DeliveryFlag = "flag"

	// OutputRaw means stdout (cleaned) IS the commit message. This is the
	// §12.1 default when output is omitted.
	OutputRaw = "raw"
	// OutputJSON means stdout is JSON; Manifest.JSONField selects the field.
	OutputJSON = "json"
)

// Manifest is the canonical description of how to invoke a single agent,
// defined verbatim by PRD §12.1. Every field is optional except Name and
// Command. Field names are idiomatic Go; the toml struct tags bind to the
// §12.1 snake_case TOML keys (go-toml/v2 does not auto CamelCase→snake_case,
// so the explicit tags are required for the future config loader, M5). The
// tags are inert reflect strings — this file needs no go-toml import.
type Manifest struct {
	// Name is the provider's identifier (§12.1), e.g. "pi", "claude".
	Name string `toml:"name"`
	// Detect is the command looked up on $PATH to decide whether this
	// provider is installed; if empty, Command is used (§12.1).
	Detect string `toml:"detect"`
	// Command is the executable to run, resolved via exec.LookPath by the
	// executor; may be an absolute path (§12.1).
	Command string `toml:"command"`
	// Subcommand holds optional tokens inserted between Command and the
	// flags, e.g. opencode's ["run"], codex's ["exec"] (§12.1).
	Subcommand []string `toml:"subcommand"`
	// PromptDelivery selects how the user payload reaches the agent:
	// "stdin" (default), "positional", or "flag" (§12.1). See Delivery*.
	PromptDelivery string `toml:"prompt_delivery"`
	// PromptFlag is used only when PromptDelivery is "flag" (§12.1).
	PromptFlag string `toml:"prompt_flag"`
	// PrintFlag is the token (if any) that puts the agent into non-
	// interactive "print and exit" mode, e.g. "-p" (§12.1).
	PrintFlag string `toml:"print_flag"`
	// ModelFlag is the flag carrying the model name, e.g. "--model" (§12.1).
	ModelFlag string `toml:"model_flag"`
	// DefaultModel is the model used when the user specifies none; resolved
	// by the caller/executor, never by Render (§12.1).
	DefaultModel string `toml:"default_model"`
	// SystemPromptFlag is the flag carrying the system prompt. Empty means
	// the agent has no such flag and the system prompt is prepended to the
	// user payload instead (§12.1, §12.2 fallback).
	SystemPromptFlag string `toml:"system_prompt_flag"`
	// ProviderFlag is the flag selecting a sub-provider for agents that
	// route to multiple backends, e.g. pi's "--provider" (§12.1).
	ProviderFlag string `toml:"provider_flag"`
	// DefaultProvider is the sub-provider used when the user specifies
	// none; resolved by the caller/executor, never by Render (§12.1).
	DefaultProvider string `toml:"default_provider"`
	// BareFlags are appended verbatim to make the call tool-less,
	// session-less, extension-less, chrome-less, and ephemeral. Empty-string
	// elements are significant (e.g. claude's "--tools", "") and are kept
	// as-is (§12.1).
	BareFlags []string `toml:"bare_flags"`
	// Output selects how stdout is interpreted: "raw" (default) or "json"
	// (§12.1). Consumed by the output parser, not by Render.
	Output string `toml:"output"`
	// JSONField selects the field to extract when Output is "json", e.g.
	// "result" (§12.1). Consumed by the output parser, not by Render.
	JSONField string `toml:"json_field"`
	// StripCodeFence removes a single layer of ``` or ~~~ fencing from the
	// output when true (§12.1). Consumed by the output parser, not Render.
	StripCodeFence bool `toml:"strip_code_fence"`
	// RetryInstruction is prepended to the payload on a parse-retry
	// (§12.1). Consumed by the generate retry loop, not by Render.
	RetryInstruction string `toml:"retry_instruction"`
	// Env holds extra environment variables set ONLY for the agent
	// subprocess (never global), keyed by variable name (§12.1).
	Env map[string]string `toml:"env"`
}

// Rendered is the result of Manifest.Render: everything the executor needs to
// build the process, in a shell-free form. Args excludes the command token —
// the executor runs exec.Command(m.Command, r.Args...). Env holds the
// manifest's environment additions as sorted "KEY=VALUE" strings, intended to
// be appended to os.Environ() by the executor (keeping Render pure).
type Rendered struct {
	// Args is the argument slice AFTER the command token, in the §12.2 order
	// (subcommand, provider, model, system_prompt, bare_flags, print_flag,
	// then delivery).
	Args []string
	// StdinPayload is the bytes piped to the process stdin when
	// DeliverViaStdin is true; otherwise empty.
	StdinPayload string
	// DeliverViaStdin is true when the user payload is delivered via stdin
	// (prompt_delivery "stdin" or the empty default); false for positional
	// or flag delivery.
	DeliverViaStdin bool
	// Env is the manifest's env additions as sorted "KEY=VALUE" strings.
	Env []string
}

// Render turns a fully-resolved intent into an exact argument slice, stdin
// payload, and sorted environment additions per the §12.2 command-rendering
// algorithm. It is pure: it performs no I/O, imports no os/exec, builds no
// exec.Cmd, and resolves no defaults (DefaultModel/DefaultProvider resolution
// is the caller/executor's job). The returned Args exclude the command token.
//
// The §12.2 arg order is fixed: Subcommand, then (ProviderFlag, provider),
// (ModelFlag, model), (SystemPromptFlag, sys) — each pair only when its flag
// AND value are non-empty — then BareFlags verbatim (empty-string elements
// kept), then PrintFlag if non-empty (AFTER BareFlags), then the delivery
// switch.
//
// System-prompt-via-payload fallback (§12.2): when SystemPromptFlag is empty
// and sys is non-empty, the system prompt is prepended to the user payload as
// sys + "\n\n" + user. This effective payload is used for stdin, positional,
// AND flag delivery alike (the §B examples for gemini/opencode/cursor prove
// positional delivery prepends). The "&& sys != \"\" guard prevents a stray
// "\n\n" when there is no system prompt.
func (m Manifest) Render(model, provider, sys, user string) (*Rendered, error) {
	effUser := user
	if m.SystemPromptFlag == "" && sys != "" {
		effUser = sys + "\n\n" + user
	}

	// Defensive copy of Subcommand into a fresh slice so subsequent appends
	// (BareFlags, delivery) can never mutate the caller's slice.
	args := make([]string, 0, len(m.Subcommand)+len(m.BareFlags)+8)
	args = append(args, m.Subcommand...)

	if m.ProviderFlag != "" && provider != "" {
		args = append(args, m.ProviderFlag, provider)
	}
	if m.ModelFlag != "" && model != "" {
		args = append(args, m.ModelFlag, model)
	}
	if m.SystemPromptFlag != "" && sys != "" {
		args = append(args, m.SystemPromptFlag, sys)
	}
	// Verbatim: keep empty-string elements (claude relies on "--tools", "").
	args = append(args, m.BareFlags...)
	if m.PrintFlag != "" {
		args = append(args, m.PrintFlag)
	}

	r := &Rendered{Args: args}
	switch m.PromptDelivery {
	case "", DeliveryStdin:
		r.StdinPayload = effUser
		r.DeliverViaStdin = true
	case DeliveryPositional:
		r.Args = append(r.Args, effUser)
	case DeliveryFlag:
		r.Args = append(r.Args, m.PromptFlag, effUser)
	default:
		return nil, fmt.Errorf("provider %q: unknown prompt_delivery %q (want stdin|positional|flag)", m.Name, m.PromptDelivery)
	}

	if len(m.Env) > 0 {
		keys := make([]string, 0, len(m.Env))
		for k := range m.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		r.Env = make([]string, 0, len(keys))
		for _, k := range keys {
			r.Env = append(r.Env, k+"="+m.Env[k])
		}
	}

	return r, nil
}
