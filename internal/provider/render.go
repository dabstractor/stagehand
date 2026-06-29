package provider

import (
	"fmt"
	"os"
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

// Render turns a provider Manifest + a (model, provider, sysPrompt, userPayload) tuple into a CmdSpec
// per PRD §12.2 "Command rendering algorithm". It is the bridge "logical intent → concrete argv".
//
// Lifecycle: Render calls m.Validate() (returns its error — covers "Validate prompt_delivery mode" +
// missing Command/Name) then m.Resolve() (every pointer non-nil → safe deref on a COPY; caller's m
// untouched). This makes Render robust to a caller that obtained the manifest from Registry.Get and
// skipped Validate/Resolve (the registry stores merged-but-unresolved manifests per P1.M2.T3).
//
// Token order (§12.2 — AUTHORITATIVE; the §12.3–§12.7 narrative "Rendered:" blocks are illustrative):
//
//	args = [subcommand...]
//	+ (provider_flag, provider)        if provider_flag != "" && provider != ""
//	+ (model_flag,    model)           if model_flag    != "" && model    != ""
//	+ (system_prompt_flag, sys)        if system_prompt_flag != "" && sys != ""
//	+ bare_flags...
//	+ print_flag                       if print_flag != ""        // ALWAYS LAST (after bare_flags)
//	+ payload                          per prompt_delivery switch (positional/flag only)
//
// model/provider default to the resolved manifest's DefaultModel/DefaultProvider when the param is ""
// (mirrors the renderArgs test scaffolding; lets the pi golden test pass with model="" → glm-5-turbo,
// and honors a §12.8 user manifest's default_provider). An explicit non-empty param always wins.
//
// System-prompt + payload (§12.2 "Note on system prompt + stdin"): when system_prompt_flag != "" the
// sys prompt is emitted via the flag and the payload is just the user payload; when system_prompt_flag
// == "" the sys prompt is PREPENDED to the payload as a fallback (delimiter "\n\n", matching every
// §12.5–§12.7 narrative). The unified payload is then routed by the delivery switch: stdin → spec.Stdin;
// positional → trailing arg; flag → prompt_flag + payload.
func (m Manifest) Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("provider render %q: %w", m.Name, err)
	}
	r := m.Resolve() // safe `*r.X` deref for every pointer; copy — caller's m untouched

	// model/provider default fallback (param "" → manifest default). Explicit non-empty wins.
	modelToUse := model
	if modelToUse == "" {
		modelToUse = *r.DefaultModel
	}
	providerToUse := provider
	if providerToUse == "" {
		providerToUse = *r.DefaultProvider
	}

	// §12.2 token order. append(nil, x...) is safe for absent slices (Subcommand/BareFlags nil → no-op).
	args := make([]string, 0, 16)
	args = append(args, r.Subcommand...)
	if *r.ProviderFlag != "" && providerToUse != "" {
		args = append(args, *r.ProviderFlag, providerToUse)
	}
	if *r.ModelFlag != "" && modelToUse != "" {
		args = append(args, *r.ModelFlag, modelToUse)
	}
	if *r.SystemPromptFlag != "" && sysPrompt != "" {
		args = append(args, *r.SystemPromptFlag, sysPrompt)
	}
	args = append(args, r.BareFlags...)
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
