# Provider System V2 Delta — Detailed Changes

## 1. Manifest Struct Extensions

### Current Manifest Fields (manifest.go)
```go
type Manifest struct {
    Name             string    `toml:"name"`
    Detect           *string   `toml:"detect"`
    Command          *string   `toml:"command"`
    Subcommand       []string  `toml:"subcommand"`
    PromptDelivery   *string   `toml:"prompt_delivery"`
    PromptFlag       *string   `toml:"prompt_flag"`
    PrintFlag        *string   `toml:"print_flag"`
    ModelFlag        *string   `toml:"model_flag"`
    DefaultModel     *string   `toml:"default_model"`
    SystemPromptFlag *string   `toml:"system_prompt_flag"`
    ProviderFlag     *string   `toml:"provider_flag"`
    DefaultProvider  *string   `toml:"default_provider"`
    BareFlags        []string  `toml:"bare_flags"`
    Output           *string   `toml:"output"`
    JsonField        *string   `toml:"json_field"`
    StripCodeFence   *bool     `toml:"strip_code_fence"`
    RetryInstruction *string   `toml:"retry_instruction"`
    Env              map[string]string `toml:"env"`
}
```

### New Fields to Add
```go
    // --- tooled mode (v2; §11.5, §12.1) ---
    // Flags for the STAGER role (tools on, git-scoped, non-interactive).
    // nil/empty => provider cannot serve as stager. Used in place of BareFlags
    // when mode=="tooled" in Render.
    TooledFlags []string `toml:"tooled_flags"`

    // --- experimental (§12.7.2, §12.5.1) ---
    // true => provider ships from docs/issue-tracker research, not verified --help.
    // `providers list` should mark experimental providers distinctly.
    Experimental *bool `toml:"experimental"`
```

### Resolve() Updates
- `TooledFlags`: leave as-is (nil stays nil — same regime as BareFlags/Subcommand)
- `Experimental`: nil → `boolPtr(false)` (default non-experimental)

### Validate() Updates
- No new validation rules needed for these fields.

## 2. MergeManifest Updates

Add to the "regime 2: slices" section:
```go
if len(override.TooledFlags) > 0 {
    out.TooledFlags = override.TooledFlags
}
```

Add to the "regime 1: scalar pointer fields" section:
```go
if override.Experimental != nil {
    out.Experimental = override.Experimental
}
```

## 3. Render Mode Support

### Current Signature
```go
func (m Manifest) Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error)
```

### V2 Signature (backward-compatible)
```go
// RenderMode selects bare (tools off, for planner/message/arbiter) vs tooled (git tools, for stager).
type RenderMode string
const (
    RenderBare   RenderMode = "bare"
    RenderTooled RenderMode = "tooled"
)

// Render creates a CmdSpec. mode defaults to "bare" when empty.
// Tooled mode with empty tooled_flags => error (provider cannot serve as stager).
func (m Manifest) Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode) (*CmdSpec, error)
```

### Rendering Algorithm Change (§12.2)
Replace `args += m.bare_flags` with:
```go
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
default: // RenderBare
    args = append(args, r.BareFlags...)
}
```

All existing callers (generate.CommitStaged, pkg/stagecoach.runPipeline) pass no mode → defaults to bare. The decompose stager passes RenderTooled.

## 4. agy Provider Manifest (§12.5.1)

```go
func builtinAgy() Manifest {
    return Manifest{
        Name:             "agy",
        Detect:           strPtr("agy"),
        Command:          strPtr("agy"),
        PromptDelivery:   strPtr("stdin"),
        PrintFlag:        strPtr("-p"),
        ModelFlag:        strPtr("-m"),
        DefaultModel:     strPtr("gemini-2.5-pro"),
        SystemPromptFlag: strPtr(""),   // none first-class → prepend to payload
        ProviderFlag:     strPtr(""),
        BareFlags:        []string{"--approval-mode", "default"},
        TooledFlags:      nil,          // intentionally empty until verified (§12.5.1.1 item 4)
        Experimental:     boolPtr(true), // ships experimental pending non-TTY fix
        Output:           strPtr("raw"),
        StripCodeFence:   boolPtr(true),
    }
}
```

Must be added to `BuiltinManifests()` map and `providers/agy.toml` created.

## 5. Provider Priority Reordering (FR-D1)

### Current preferredBuiltins
```go
var preferredBuiltins = []string{"pi", "claude", "gemini", "opencode", "codex", "cursor"}
```

### V2 preferredBuiltins
```go
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "codex", "claude"}
```

Rationale: open/self-hostable harnesses first (pi, opencode); closed subscription CLIs last.

## 6. pi Default Model Update (FR-D2)

### Current pi manifest
```go
DefaultModel: strPtr("glm-5-turbo"),  // the author's personal z.ai subscription
```

### V2 pi manifest
```go
DefaultModel: strPtr(""),  // empty; config init fills per-role from FR-D4 table
```

The `glm-5-turbo`/`zai` setup becomes a documented personal override, NOT a default.

## 7. Per-Role Model Defaults (FR-D4)

A constant table mapping provider→role→default model. The bootstrap config materializes one provider's column. Model names MUST be re-verified at implementation (FR-D5).

```go
// RoleTier identifies a role's model sizing (§9.16 FR-D3).
type RoleTier string
const (
    TierSmart RoleTier = "planner"  // flagship/smart
    TierMid   RoleTier = "stager"   // mid (tooled, needs dependable tool calls)
    TierFast  RoleTier = "message"  // fast (bare text generation)
    // arbiter = mid
)
```

The table is defined as a Go map or constant set. `config init` uses it to populate the bootstrap config.

## 8. (Provider, Model) Coupling (FR-R5b)

For multi-provider agents (pi, opencode, agy — those with non-empty `provider_flag`), a model is AMBIGUOUS without its provider. The Render method already handles this at the flag level (only emits `--provider` when `provider_flag != ""`), so the coupling enforcement belongs in **role resolution**: when resolving a role's `(provider, model)`, if the resolved provider's manifest has a non-empty `provider_flag` and `model != ""` but `provider == ""`, surface a configuration error.
