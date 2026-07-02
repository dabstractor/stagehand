package provider

import (
	"fmt"
	"os"
	"strings"
)

// CmdSpec is the fully-specified subprocess invocation produced by Manifest.Render (PRD §12.2). It is
// the contract between the renderer (this package, T4) and the executor (P1.M2.T5): the executor runs
// exec.Command(spec.Command, spec.Args...) with cmd.Stdin/cmd.Env derived from Stdin/Env. It is pure
// data — Render performs NO spawning (the os.Environ() call for Env is the sole side effect).
//
// Stdin semantics: Stdin carries the payload to pipe for stdin-delivery providers; it is "" for
// positional/flag-delivery providers, which the executor interprets as "use os.DevNull" (matching PRD
// §12.2 cmd.Stdin = (delivery=="stdin") ? reader : /dev/null). CmdSpec intentionally does NOT carry
// the delivery mode — Stdin="" disambiguates.
//
// Env semantics: os.Environ() (the parent process env) followed by the manifest's Env entries as
// "KEY=VAL". exec.Cmd.Env uses last-wins, so manifest entries (appended last) override the parent —
// matching PRD §12.2 cmd.Env = os.Environ() + m.env.
type CmdSpec struct {
	Command string   // the executable (resolved manifest.Command), e.g. "pi", "agent"
	Args    []string // the flag portion AFTER command, in §12.2 token order (NOT including Command)
	Stdin   string   // payload to pipe (stdin delivery); "" → executor uses os.DevNull
	Env     []string // os.Environ() + manifest Env entries as "KEY=VAL" (manifest wins on collision)
}

// RenderMode selects which flag-set Manifest.Render appends after the system-prompt block (PRD §11.5,
// §12.2). It is the v2 "mode" dimension of the §12.2 rendering ternary
// `args += (mode == "tooled") ? m.tooled_flags : m.bare_flags`.
//
// Render's `mode ...RenderMode` parameter is VARIADIC and defaults to RenderBare when omitted, so every
// v1 caller (generate.CommitStaged, pkg/stagehand.runPipeline, all tests) is unchanged. The decompose
// stager (P3.M2.T3) passes RenderTooled.
type RenderMode string

const (
	// RenderBare appends BareFlags — tools off, session-less, chrome-less, ephemeral (PRD §12.1).
	// The DEFAULT mode. Serves the planner / message / arbiter roles and the entire v1 single-commit path.
	RenderBare RenderMode = "bare"

	// RenderTooled appends TooledFlags — tools on, git-scoped, non-interactive (PRD §12.1 tooled_flags).
	// Serves the stager role (the only role that mutates the index, §11.5). Errors at render time if
	// TooledFlags is nil/empty — that provider cannot serve as a stager.
	RenderTooled RenderMode = "tooled"
)

// Render turns a provider Manifest + a (model, sysPrompt, userPayload, reasoning) tuple into a CmdSpec
// per PRD §12.2 "Command rendering algorithm". It is the bridge "logical intent → concrete argv".
//
// Lifecycle: Render calls m.Validate() (returns its error — covers "Validate prompt_delivery mode" +
// missing Command/Name) then m.Resolve() (every pointer non-nil → safe deref on a COPY; caller's m
// untouched). This makes Render robust to a caller that obtained the manifest from Registry.Get and
// skipped Validate/Resolve (the registry stores merged-but-unresolved manifests per P1.M2.T3).
//
// Token order (§12.2 v3 — AUTHORITATIVE; the §12.3–§12.7 narrative "Rendered:" blocks are illustrative):
//
//	args = [subcommand...]
//	+ (--provider, <inference>)          if provider_flag != "" && model contains "/" (FR-R5b fold)
//	+ (model_flag,    model)              if model_flag    != "" && model    != ""
//	+ reasoning_level_tokens...          if reasoning != "" && ReasoningLevels[reasoning] non-empty (FR-R6)
//	+ (system_prompt_flag, sys)          if system_prompt_flag != "" && sys != ""
//	+ (mode==tooled ? tooled_flags : bare_flags)...    # §11.5/§12.2 mode ternary; default bare
//	+ print_flag                         if print_flag != ""        // ALWAYS LAST (after flags)
//	+ payload                            per prompt_delivery switch (positional/flag only)
//
// The inference provider is folded into the model slash-prefix (FR-R5b): a provider_flag provider (pi)
// takes "inference/model" — the prefix before "/" becomes the --provider value, the rest is the model.
// A bare model (no "/") on such a provider is a HARD ERROR. Providers without a provider_flag (opencode,
// claude, single-backend) pass the model VERBATIM — opencode's "openai/gpt-5.4" is its own combined form.
//
// Reasoning tokens (FR-R6): if the reasoning param matches a key in ReasoningLevels and the value is
// non-empty, those tokens are appended after the model flag. Absent level, nil map, or empty token list
// ⇒ SILENT no-op — NEVER an error.
//
// System-prompt + payload (§12.2 "Note on system prompt + stdin"): when system_prompt_flag != "" the
// sys prompt is emitted via the flag and the payload is just the user payload; when system_prompt_flag
// == "" the sys prompt is PREPENDED to the payload as a fallback (delimiter "\n\n", matching every
// §12.5–§12.7 narrative). The unified payload is then routed by the delivery switch: stdin → spec.Stdin;
// positional → trailing arg; flag → prompt_flag + payload.
//
// `mode ...RenderMode` (variadic, default `RenderBare`) selects the flag-set appended after the
// system-prompt block: `RenderBare` (the default) appends `BareFlags` (tools off — planner/message/
// arbiter + the entire v1 single-commit path); `RenderTooled` appends `TooledFlags` (git tools on —
// the stager role, §11.5). `RenderTooled` on a manifest with nil/empty `TooledFlags` returns an
// error — that provider cannot serve as a stager (§12.1). The variadic default keeps every v1 caller
// unchanged.
func (m Manifest) Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("provider render %q: %w", m.Name, err)
	}
	r := m.Resolve() // safe `*r.X` deref for every pointer; copy — caller's m untouched

	// model default fallback (param "" → manifest default). Explicit non-empty wins.
	modelToUse := model
	if modelToUse == "" {
		modelToUse = *r.DefaultModel
	}

	// §12.2 token order. append(nil, x...) is safe for absent slices (Subcommand/BareFlags nil → no-op).
	args := make([]string, 0, 16)
	args = append(args, r.Subcommand...)

	// FR-R5b: a provider with a provider_flag (pi — the only one) takes "inference/model". Split the
	// slash-prefix → --provider <prefix>; the rest is the model. A bare model (no "/") on such a provider
	// is a HARD ERROR — never a silent bare --model that routes wrong. Providers without a provider_flag
	// (opencode + all single-backend) pass the model VERBATIM (opencode's "openai/gpt-5.4" is its own
	// combined form — do NOT split it).
	if *r.ProviderFlag != "" && modelToUse != "" {
		if i := strings.Index(modelToUse, "/"); i >= 0 {
			args = append(args, *r.ProviderFlag, modelToUse[:i])
			modelToUse = modelToUse[i+1:]
		} else {
			return nil, fmt.Errorf(
				"provider render %q: model %q on %s must be inference/model, e.g. \"zai/glm-5.2\"",
				m.Name, modelToUse, m.Name)
		}
	}
	if *r.ModelFlag != "" && modelToUse != "" {
		args = append(args, *r.ModelFlag, modelToUse)
	}

	// FR-R6: append the resolved reasoning level's tokens if the manifest declares them. Absent level,
	// nil map, or empty token list ⇒ SILENT no-op (provider/model lacks reasoning control) — never an error.
	if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0 {
		args = append(args, r.ReasoningLevels[reasoning]...)
	}

	if *r.SystemPromptFlag != "" && sysPrompt != "" {
		args = append(args, *r.SystemPromptFlag, sysPrompt)
	}
	// §11.5 / §12.2 mode: bare (tools off, planner/message/arbiter + all v1 callers) vs tooled
	// (git tools on, stager). Defaults to bare when mode is omitted (variadic) — keeps every v1
	// caller byte-for-byte unchanged. Tooled with empty tooled_flags is an error (§12.1: that
	// provider cannot serve as a stager).
	selectedMode := RenderBare
	if len(mode) > 0 && mode[0] != "" {
		selectedMode = mode[0]
	}
	switch selectedMode {
	case RenderTooled:
		if len(r.TooledFlags) == 0 {
			return nil, fmt.Errorf("provider %q: tooled mode requires non-empty tooled_flags", m.Name)
		}
		args = append(args, r.TooledFlags...)
	default: // RenderBare — also the fallback for "" / any unrecognized mode (PRD §12.2 ternary)
		args = append(args, r.BareFlags...)
	}
	if *r.PrintFlag != "" {
		args = append(args, *r.PrintFlag) // LAST per §12.2
	}

	// Unified payload + the system-prompt-prepend fallback (§12.2 note). The single sys-flag check is
	// correct for ALL delivery modes (research §2). Delimiter is exactly "\n\n"; empty sys → no prepend.
	payload := userPayload
	if *r.SystemPromptFlag == "" && sysPrompt != "" {
		payload = sysPrompt + "\n\n" + userPayload
	}

	// prompt_delivery switch. stdin → payload to Stdin (nothing appended); positional → trailing arg;
	// flag → prompt_flag + payload. Default → error (Validate already rejects invalid values).
	spec := &CmdSpec{Command: *r.Command, Args: args}
	switch *r.PromptDelivery {
	case "stdin":
		spec.Stdin = payload
	case "positional":
		spec.Args = append(spec.Args, payload)
	case "flag":
		spec.Args = append(spec.Args, *r.PromptFlag, payload)
	default:
		return nil, fmt.Errorf("provider render %q: unsupported prompt_delivery %q", m.Name, *r.PromptDelivery)
	}

	// Env = os.Environ() + manifest Env entries (manifest appended last → exec last-wins → override).
	env := os.Environ()
	for k, v := range r.Env {
		env = append(env, k+"="+v)
	}
	spec.Env = env

	return spec, nil
}
