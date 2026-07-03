package provider

// MergeManifest overlays the non-nil/non-empty fields of override onto a copy of base and returns
// the merged manifest, per PRD §16.1 ("Provider manifests merge field-by-field: a user override that
// sets only default_model leaves all other fields from the built-in manifest intact").
//
// THREE merge regimes (from the work-item contract):
//
//  1. Scalar pointer fields (*string / *bool): override.Field != nil → result takes override.Field.
//     An EXPLICIT "" or false OVERRIDES (non-nil is the override signal — the whole reason S1 made the
//     optional scalars pointers; a present zero value is a deliberate override, not an absence).
//
//  2. Slices (Subcommand, BareFlags, TooledFlags): len(override.Slice) > 0 → result REPLACES base's slice
//     wholesale (NO element-level merge). An empty/nil override slice is treated as "not overridden"
//     (base preserved).
//
//  3. Env map: merged KEY-BY-KEY into a FRESH map — each key in override.Env overwrites the same
//     base key, while base keys absent from override survive. A nil/empty override.Env leaves base.Env.
//     (CRITICAL: a fresh map is allocated to avoid mutating the caller's base.Env — maps are reference
//     types and out := base aliases them.)
//
// Name is NOT field-merged — result.Name == base.Name. name is the [provider.<name>] table key
// (never written into the table body), so a decoded override always has Name==""; the registry
// (P1.M2.T3) sets the final Name from the table key.
//
// MergeManifest does NOT Validate or Resolve — a partial override legitimately lacks Command (it
// inherits the built-in's). The registry runs Validate on the merged result, then Resolve. Pure merge.
func MergeManifest(base, override Manifest) Manifest {
	out := base

	// --- regime 1: scalar pointer fields — non-nil override WINS (explicit "" / false included) ---
	if override.Detect != nil {
		out.Detect = override.Detect
	}
	if override.Command != nil {
		out.Command = override.Command
	}
	if override.PromptDelivery != nil {
		out.PromptDelivery = override.PromptDelivery
	}
	if override.PromptFlag != nil {
		out.PromptFlag = override.PromptFlag
	}
	if override.PrintFlag != nil {
		out.PrintFlag = override.PrintFlag
	}
	if override.ModelFlag != nil {
		out.ModelFlag = override.ModelFlag
	}
	if override.DefaultModel != nil {
		out.DefaultModel = override.DefaultModel
	}
	if override.SystemPromptFlag != nil {
		out.SystemPromptFlag = override.SystemPromptFlag
	}
	if override.ProviderFlag != nil {
		out.ProviderFlag = override.ProviderFlag
	}

	if override.Output != nil {
		out.Output = override.Output
	}
	if override.JsonField != nil {
		out.JsonField = override.JsonField
	}
	if override.StripCodeFence != nil {
		out.StripCodeFence = override.StripCodeFence
	}
	if override.Experimental != nil {
		out.Experimental = override.Experimental
	}
	if override.RetryInstruction != nil {
		out.RetryInstruction = override.RetryInstruction
	}

	// --- regime 2: slices — non-empty override REPLACES wholesale (no element merge) ---
	if len(override.Subcommand) > 0 {
		out.Subcommand = override.Subcommand
	}
	if len(override.ListModelsCommand) > 0 {
		out.ListModelsCommand = override.ListModelsCommand
	}
	if len(override.BareFlags) > 0 {
		out.BareFlags = override.BareFlags
	}
	if len(override.TooledFlags) > 0 {
		out.TooledFlags = override.TooledFlags
	}

	// --- regime 3: Env map — key-by-key merge into a FRESH map (override keys win; base keys survive) ---
	// CRITICAL: out.Env currently ALIASES base.Env (out := base copied the map header). We MUST allocate
	// a fresh map and copy both sides into it; mutating out.Env in place would corrupt the caller's
	// base.Env (a silent side effect). Slices are safe above because we only reassign the header.
	if len(override.Env) > 0 {
		merged := make(map[string]string, len(base.Env)+len(override.Env))
		for k, v := range base.Env {
			merged[k] = v
		}
		for k, v := range override.Env {
			merged[k] = v
		}
		out.Env = merged
	}

	// --- regime 3 (cont): ReasoningLevels map — key-by-key merge into a FRESH map ---
	// CRITICAL: out.ReasoningLevels currently ALIASES base.ReasoningLevels (out := base copied the map header).
	// We MUST allocate a fresh map; mutating out.ReasoningLevels in place would corrupt the caller's base.
	if len(override.ReasoningLevels) > 0 {
		merged := make(map[string][]string, len(base.ReasoningLevels)+len(override.ReasoningLevels))
		for k, v := range base.ReasoningLevels {
			merged[k] = v
		}
		for k, v := range override.ReasoningLevels {
			merged[k] = v // override key wins; wholesale slice replacement per key
		}
		out.ReasoningLevels = merged
	}

	// Name: NOT merged — out.Name == base.Name (the struct copy). The registry sets the final Name.
	return out
}
