package provider

import (
	"errors"
	"fmt"
)

// PRD §12.1 default values (applied by Resolve to nil optional fields).
const (
	DefaultPromptDelivery   = "stdin"                                                                // §12.1 prompt_delivery default
	DefaultOutput           = "raw"                                                                  // §12.1 output default
	DefaultStripCodeFence   = true                                                                   // §12.1 strip_code_fence default
	DefaultRetryInstruction = "Output ONLY the commit message. No preamble, no markdown, no quotes." // §12.1 retry_instruction
)

// validPromptDeliveries / validOutputs are the §12.1 / §12.2 enum members Validate enforces.
var (
	validPromptDeliveries = map[string]struct{}{"stdin": {}, "positional": {}, "flag": {}}
	validOutputs          = map[string]struct{}{"raw": {}, "json": {}}
)

// Manifest describes one AI-provider CLI per PRD §12.1. Built-in manifests are compiled in (P1.M2.T2);
// user manifests live under [provider.<name>] in config (raw map — config.Providers) and are merged
// field-by-field onto a built-in by the registry (P1.M2.T3) per PRD §16.1.
//
// DESIGN CALL — POINTER SCALARS. go-toml/v2 has no omitempty (arch §5.4 / FINDING 5), so the optional
// SCALAR fields are *string / *bool: a field ABSENT in a user override decodes to nil (→ inherit the
// built-in value on merge), while a field PRESENT — even set to "" or false — decodes to a NON-NIL
// pointer (→ override). This is the only way a field-by-field struct merge can honor a user's
// strip_code_fence=false or print_flag="" override. Verified empirically; see research FINDING C/D.
// Slices (Subcommand, BareFlags, TooledFlags) and the Env map stay plain: nil is their natural "absent" sentinel
// (absent → nil; present → non-nil even if empty), so pointers would add only dereference noise.
// Name is plain: it is the identity, always set by the registry from the [provider.<name>] table key.
//
// Decode target + merge participant + (after Resolve) consumed value — one struct, three roles.
type Manifest struct {
	// --- discovery (§12.1) ---
	Name              string   `toml:"name"`                // REQUIRED. The identity; registry sets this from the table key.
	Detect            *string  `toml:"detect"`              // nil/"" => DetectCommand falls back to Command.
	Command           *string  `toml:"command"`             // REQUIRED (post-merge). nil in a partial override => inherit.
	ListModelsCommand []string `toml:"list_models_command"` // nil/empty => no listing command (FR-L1 falls back to FR-D4 curated table); populated ONLY for verified providers (FR-D5); NEVER an HTTP call (§6.2 N2).
	Subcommand        []string `toml:"subcommand"`          // nil => none; inserted between command and flags.

	// --- prompt delivery (§12.1) ---
	PromptDelivery *string `toml:"prompt_delivery"` // stdin|positional|flag; nil => Resolve→"stdin".
	PromptFlag     *string `toml:"prompt_flag"`     // used only when PromptDelivery=="flag".

	// --- non-interactive / print mode (§12.1) ---
	PrintFlag *string `toml:"print_flag"` // nil/"" => no print flag appended.

	// --- model (§12.1) ---
	ModelFlag    *string `toml:"model_flag"`    // e.g. "--model"
	DefaultModel *string `toml:"default_model"` // nil/"" => user must set a model.

	// --- system prompt (§12.1) ---
	SystemPromptFlag *string `toml:"system_prompt_flag"` // nil/"" => prepend sys to payload (§12.2).

	// --- sub-provider (§12.1) ---
	ProviderFlag *string `toml:"provider_flag"`

	// --- bare mode (§12.1) ---
	BareFlags []string `toml:"bare_flags"` // appended verbatim; nil => none.

	// --- tooled mode (v2; §11.5, §12.1) ---
	// Flags for the STAGER role (tools on, git-scoped, non-interactive). nil/empty => this
	// provider does not support tooled mode and cannot serve as a stager. Used in place of
	// BareFlags when mode=="tooled" in Render.
	TooledFlags []string `toml:"tooled_flags"`

	// --- experimental (§12.7.2, §12.5.1) ---
	// true => provider ships from docs/issue-tracker research, not a verified --help.
	// `providers list` marks experimental providers distinctly.
	Experimental *bool `toml:"experimental"`

	// --- output (§12.1) ---
	Output         *string `toml:"output"`           // raw|json; nil => Resolve→"raw".
	JsonField      *string `toml:"json_field"`       // used only when Output=="json".
	StripCodeFence *bool   `toml:"strip_code_fence"` // nil => Resolve→true.

	// --- retry (§12.1) ---
	RetryInstruction *string `toml:"retry_instruction"` // prepended on a parse-retry; nil => Resolve→default.

	// --- environment (§12.1) ---
	Env map[string]string `toml:"env"` // set ONLY for the subprocess; nil => none.

	// --- reasoning levels (v3; FR-R6) ---
	// Per-level flag tokens appended at Render to express reasoning/thinking effort (off|low|medium|high).
	// nil/empty ⇒ graceful no-op (provider/model lacks reasoning control) — NEVER an error. Decoded from
	// the [reasoning_levels] subtable. Map regime (like Env): nil is the natural "absent" sentinel.
	ReasoningLevels map[string][]string `toml:"reasoning_levels"`
}

// Validate checks the merged manifest's required fields and enum members (PRD §12.1). It is
// nil-tolerant on optional enums (a nil PromptDelivery/Output will take its default via Resolve) but
// strict on Name (non-empty) and Command (non-nil, non-empty), and rejects any NON-NIL but invalid
// enum. Safe to run on a partial override (nil optionals pass); its primary call site is post-merge.
func (m Manifest) Validate() error {
	if m.Name == "" {
		return errors.New("provider manifest: name is required")
	}
	if m.Command == nil || *m.Command == "" {
		return errors.New("provider manifest: command is required")
	}
	if m.PromptDelivery != nil {
		if _, ok := validPromptDeliveries[*m.PromptDelivery]; !ok {
			return fmt.Errorf("provider manifest %q: prompt_delivery %q must be one of stdin|positional|flag", m.Name, *m.PromptDelivery)
		}
	}
	if m.Output != nil {
		if _, ok := validOutputs[*m.Output]; !ok {
			return fmt.Errorf("provider manifest %q: output %q must be one of raw|json", m.Name, *m.Output)
		}
	}
	return nil
}

// DetectCommand returns the discovery command: Detect if set and non-empty, else Command (§12.1:
// "If absent, `command` is used"). Returns "" if neither is set (the registry treats "" as
// "not installed" via exec.LookPath).
func (m Manifest) DetectCommand() string {
	if m.Detect != nil && *m.Detect != "" {
		return *m.Detect
	}
	if m.Command != nil {
		return *m.Command
	}
	return ""
}

// Resolve returns a copy of m with every nil OPTIONAL pointer filled to its default, so consumers
// (renderer/executor/parser) can dereference every pointer safely. The four PRD-defaulted fields take
// their §12.1 defaults; the remaining optional *string fields take *"" (semantically "not used");
// Command is left nil if it was nil (Validate, run before Resolve, flags a missing command); slices
// and the Env map are left as-is (nil stays nil — append(nil...) is a no-op for the renderer).
//
// EXPLICIT values — including a non-nil *false or *"" — are PRESERVED (Resolve never overwrites a
// present value; this is the correctness keystone of the pointer design).
func (m Manifest) Resolve() Manifest {
	out := m // copy the headers/pointers/slices/map
	if out.Detect == nil {
		out.Detect = strPtr("")
	}
	// Command left nil if absent; Validate enforces requiredness.
	if out.PromptDelivery == nil {
		out.PromptDelivery = strPtr(DefaultPromptDelivery)
	}
	if out.PromptFlag == nil {
		out.PromptFlag = strPtr("")
	}
	if out.PrintFlag == nil {
		out.PrintFlag = strPtr("")
	}
	if out.ModelFlag == nil {
		out.ModelFlag = strPtr("")
	}
	if out.DefaultModel == nil {
		out.DefaultModel = strPtr("")
	}
	if out.SystemPromptFlag == nil {
		out.SystemPromptFlag = strPtr("")
	}
	if out.ProviderFlag == nil {
		out.ProviderFlag = strPtr("")
	}

	if out.Output == nil {
		out.Output = strPtr(DefaultOutput)
	}
	if out.JsonField == nil {
		out.JsonField = strPtr("")
	}
	if out.StripCodeFence == nil {
		out.StripCodeFence = boolPtr(DefaultStripCodeFence)
	}
	if out.Experimental == nil {
		out.Experimental = boolPtr(false) // §12.7.2 default: non-experimental unless explicitly set
	}
	if out.RetryInstruction == nil {
		out.RetryInstruction = strPtr(DefaultRetryInstruction)
	}
	// Subcommand / BareFlags / TooledFlags / Env / ReasoningLevels: left as-is (nil stays nil).
	return out
}

// strPtr / boolPtr are unexported helpers for constructing pointer fields (Resolve + same-package tests).
func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
