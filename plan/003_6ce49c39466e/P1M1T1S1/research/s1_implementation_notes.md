# S1 Implementation Notes — Manifest v3 schema + Render signature keystone

> Scope: P1.M1.T1.S1 — the load-bearing keystone. Remove `DefaultProvider`; add `ReasoningLevels`;
> fold the inference provider into the `model` slash-prefix (FR-R5b) at `Render`; emit reasoning
> tokens (FR-R6); new `Render` signature. Verified against live source on 2026-07-01.

## 1. Current Manifest fields + the two regimes (manifest.go)

Pointer scalars (`*string`/`*bool`): nil=absent, non-nil=explicit. Slices (`[]string`) + maps: nil =
natural absent sentinel. `strPtr`/`boolPtr` helpers at file bottom.

**REMOVE:** `DefaultProvider *string \`toml:"default_provider"\`` (manifest.go:59).
**ADD:** `ReasoningLevels map[string][]string \`toml:"reasoning_levels"\`` (a `[reasoning_levels]` subtable;
go-toml/v2 decodes it into this map). ReasoningLevels follows the MAP regime (like Env) — nil is a fine
"absent" sentinel; reads on a nil map are safe (`m[k]` → nil slice, `len(nil)==0`).

## 2. Resolve() deltas (manifest.go)

- DELETE the `DefaultProvider` nil-default block (lines 159-160).
- ReasoningLevels: leave as-is (nil stays nil) — consistent with Env/BareFlags/TooledFlags. Reads are
  nil-safe; Resolve need NOT allocate an empty map. (No-op for reasoning when nil/empty.)
- The struct doc comment's enumeration of map/slice fields + Resolve's "left as-is" comment should add
  ReasoningLevels. Resolve's "four PRD-defaulted fields" count is unaffected (DefaultProvider wasn't a
  §12.1 default anyway).

## 3. Validate() — UNCHANGED

DefaultProvider was never validated; ReasoningLevels has no enum/required constraint (empty map OK per
FR-R6 graceful no-op). Do NOT add a Validate rule.

## 4. render.go — new signature + body (THE keystone)

**NEW signature:**
```go
func (m Manifest) Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error)
```
(The `provider` param is GONE — it folds into the model slash-prefix. `reasoning` is new.)

**Body changes** (replace the model/provider fallback + FR-R5b backstop + the --provider emit):
```go
r := m.Resolve()
modelToUse := model
if modelToUse == "" { modelToUse = *r.DefaultModel }   // unchanged

args := make([]string, 0, 16)
args = append(args, r.Subcommand...)

// FR-R5b: a provider with a provider_flag (pi) takes "inference/model"; split the prefix → --provider.
// A bare model (no "/") on such a provider is a HARD ERROR, never a silent bare --model. Providers
// without a provider_flag (opencode + all single-backend) pass model verbatim.
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
// FR-R6: reasoning level — append the resolved level's tokens if declared; absent/empty ⇒ silent no-op
// (provider or model lacks reasoning control). NEVER an error.
if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0 {
    args = append(args, r.ReasoningLevels[reasoning]...)
}
// ... SystemPromptFlag / mode ternary / print_flag / delivery switch / Env — UNCHANGED ...
```
**Imports:** `strings` is needed (strings.Index). render.go already imports `fmt`, `os`; ADD `"strings"`.

**Removed (gone with DefaultProvider):** `providerToUse` derivation (lines ~94-96), the old FR-R5b
backstop (`*r.ProviderFlag != "" && modelToUse != "" && providerToUse == ""` → error), and the
`--provider <providerToUse>` emit. Update the Render doc comment's token-order block to the v3 order
(provider folds into model; reasoning added after model flag).

## 5. merge.go — drop DefaultProvider block; add ReasoningLevels merge

- DELETE the `override.DefaultProvider` regime-1 block (lines 59-60).
- ADD ReasoningLevels merge. Policy: key-by-key into a FRESH map (matches Env regime-3 intent +
  "field-by-field merge"; a user overriding `high` keeps base's `low`/`medium`). MUST allocate a fresh
  map (maps are reference types — `out := base` aliases base.ReasoningLevels; mutating corrupts the
  caller's base). Adapt the Env block:
```go
if len(override.ReasoningLevels) > 0 {
    merged := make(map[string][]string, len(base.ReasoningLevels)+len(override.ReasoningLevels))
    for k, v := range base.ReasoningLevels {
        merged[k] = v
    }
    for k, v := range override.ReasoningLevels {
        merged[k] = v // override key wins; wholesale slice replace per key
    }
    out.ReasoningLevels = merged
}
```

## 6. builtin.go — 8 builtins (DefaultProvider only on pi:50)

- `builtinPi`: DELETE the `DefaultProvider: strPtr("")` line. ADD ReasoningLevels (see §7).
- The other 7 builtins set DefaultProvider NIL (absent) — no field to delete, but their COMMENT
  blocks reference "DefaultProvider is NIL" (claude:96/130, gemini:142/160, agy:180/200, opencode:214/230,
  codex:250/274, cursor:288/312). Comments are harmless for compilation; clean them in the files S1
  materially edits for Mode-A doc quality (optional; P1.M2 owns decompose files).
- `BuiltinManifests` map: unchanged (7 entries — qwen-code is P2, NOT S1).

## 7. ReasoningLevels for pi/claude (FR-D5: verify, else nil+TODO)

FR-D5: reasoning/thinking-effort flag tokens MUST be verified per agent's live docs/`--help`. Exact pi
and claude tokens are NOT confirmed. The contract says "populate ... (verify per FR-D5 — leave # TO
CONFIRM if unsure)". The SAFE, can't-break-anything choice: leave ReasoningLevels NIL for pi and claude
in S1 with a `// TODO(FR-D5): populate reasoning_levels once tokens verified` comment — FR-R6 makes a
nil/empty table a graceful no-op, so reasoning="" (which S1's call sites pass) → no-op regardless. If
the implementer CAN verify tokens (e.g. claude `--thinking-effort low|medium|high`), populate them with
a `// TO CONFIRM (FR-D5)` comment. Defaulting to nil+TODO is the conservative one-pass choice.

## 8. The 6 non-test Render call sites (update arity; reasoning="" temporarily)

| file:line | v3 call |
|---|---|
| internal/generate/generate.go:196 | `deps.Manifest.Render(cfg.Model, sysPrompt, payload, "")` |
| pkg/stagecoach/stagecoach.go:461 | `deps.Manifest.Render(cfg.Model, sysPrompt, payload, "")` |
| internal/decompose/planner.go:98 | `deps.Manifest.Render(mdl, sysPrompt, payload, "", provider.RenderBare)` |
| internal/decompose/message.go:129 | `deps.Manifest.Render(mdl, sysPrompt, payload, "", provider.RenderBare)` |
| internal/decompose/arbiter.go:97 | `deps.Manifest.Render(mdl, sysPrompt, payload, "", provider.RenderBare)` |
| internal/decompose/stager.go:86 | `deps.Manifest.Render(mdl, "", task, "", provider.RenderTooled)` |

Each drops the literal `""` provider arg and inserts `""` for reasoning (P1.M2 wires real per-role
reasoning). NOTE: at the decompose sites `mdl` comes from `config.ResolveRoleModel(role,cfg)` whose
provider return is already discarded (`_,`) — so dropping the provider arg is purely mechanical.

## 9. The ONE subtle production-compile break: decompose/roles.go (NOT a Render call site)

Removing DefaultProvider breaks `inferenceProvider(m)` (roles.go:189: `return *m.Resolve().DefaultProvider`),
used by the ResolveRoles FR-R5b guard (roles.go:151: `if mdl != "" && isMultiProvider(m) &&
inferenceProvider(m) == "" { error }`). This guard is the role-resolution EARLY re-check; its PROPER v3
rework (model-prefix-based) is P1.M2.T1.S2.

**Minimal S1 fix** (keep production compiling, behavior correct via Render's new chokepoint):
- DELETE the `inferenceProvider` function (roles.go:185-190).
- DELETE the guard block that uses it (roles.go:143-156), replacing with a TODO comment:
  `// TODO(P1.M2.T1.S2): re-add the role-named FR-R5b guard using the model slash-prefix (no "/" ⇒ error).`
- KEEP `isMultiProvider` (roles.go:177-182) — unused package-level funcs are legal in Go; P1.M2 reuses it.

This is SAFE because S1's new Render now ENFORCES FR-R5b at the single command-emission chokepoint
(no "/" on a provider_flag provider → error) for ALL paths, including decompose. The only loss is the
role-named early error message, which P1.M2.T1.S2 restores.

## 10. providers/pi.toml (Mode A doc)

Remove the `default_provider = ""` line (pi.toml:52). Only pi.toml carries it. (The other providers/*.toml
omit the key.)

## 11. Scope discipline + the S1/S2 boundary (CRITICAL for validation)

S1 = PRODUCTION (manifest.go, render.go, merge.go, builtin.go, the 6 Render call sites, decompose/roles.go
minimal fix, providers/pi.toml). S2 (P1.M1.T1.S2) = the ~41+ TEST call sites + new FR-R5b/reasoning tests.

**Consequence:** changing the Render arity + removing DefaultProvider BREAKS TEST compilation in ~13 test
files (render_test.go, builtin_test.go, realagent_test.go, stubtest_test.go, manifest_test.go, merge_test.go,
registry_test.go, generate_test.go, + decompose/*_test.go). `go build ./...` does NOT compile _test.go files,
so **`go build ./...` is S1's GREEN gate** ("a compiling tree"). `go vet ./...` and `go test ./...` will FAIL
on test compilation until S2 — this is the EXPECTED keystone state. Do NOT chase test compilation in S1.

To VALIDATE the new Render behavior without the (S2-owned) tests: a throwaway `go run` smoke (§ of the PRP)
exercising (a) pi + `zai/glm-5.2` → `--provider zai --model glm-5.2`; (b) pi + `glm-5.2` (no slash) → error;
(c) claude + `sonnet` (no provider_flag) → verbatim `--model sonnet`; (d) reasoning emit when a level is
declared + no-op when not.
