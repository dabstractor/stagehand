---
name: "P1.M1.T1.S1 — Manifest v3 schema + Render signature keystone (remove DefaultProvider, add ReasoningLevels, model-prefix fold, reasoning emit)"
description: |
  v3 load-bearing keystone. (a) Manifest: REMOVE `DefaultProvider *string`; ADD
  `ReasoningLevels map[string][]string` (`toml:"reasoning_levels"`). Resolve() drops the DefaultProvider
  default + is nil-safe on ReasoningLevels; Validate() unchanged; MergeManifest drops the DefaultProvider
  block + adds a key-by-key ReasoningLevels merge (fresh map). (b) Render NEW signature
  `Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode)` — the `provider` param is
  gone (folds into the model slash-prefix). A `provider_flag` provider (pi) splits `backend/model` →
  `--provider <backend> --model <rest>`; a bare model (no `/`) on such a provider is a HARD ERROR (FR-R5b).
  Reasoning tokens (`m.reasoning_levels[reasoning]`) are appended after the model flag — absent/empty ⇒
  silent no-op, never an error (FR-R6). (c) 8 builtins: drop DefaultProvider (only pi carries it); populate
  ReasoningLevels for pi/claude only if FR-D5-verified (else nil+TODO). (d) 6 non-test Render call sites →
  new arity, `reasoning=""` temporarily (P1.M2 wires real values). (e) decompose/roles.go: remove the dead
  `inferenceProvider` + its guard (DefaultProvider gone); Render now enforces FR-R5b at the chokepoint;
  P1.M2.T1.S2 re-adds the role-named guard. (f) providers/pi.toml: drop `default_provider`. CmdSpec +
  provider.Execute UNCHANGED. S1 gate = `go build ./...` (test files are S2's scope).
---

## Goal

**Feature Goal**: Land the v3 provider-system keystone so that `Manifest.Render` — the single
command-emission chokepoint every call path flows through — enforces FR-R5b (the inference backend is the
slash-prefix on `model`, never a separate field or a silent bare `--model`) and emits per-role reasoning
tokens (FR-R6), with the `DefaultProvider` field eliminated entirely. This is the structural fix for the
provider/sub-provider conflation bug class: **the prefix IS the field**.

**Deliverable** (production-only; one signature + one schema + ripple):
1. `internal/provider/manifest.go`: remove `DefaultProvider`; add `ReasoningLevels map[string][]string`; update `Resolve()` (drop DefaultProvider default; nil-safe ReasoningLevels) + struct doc comment. `Validate()` unchanged.
2. `internal/provider/render.go`: new signature `Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode)`; new body implementing the model-prefix fold + no-slash hard error (FR-R5b) + reasoning-token append (FR-R6); updated Render doc comment + add `"strings"` import.
3. `internal/provider/merge.go`: drop the `DefaultProvider` regime-1 block; add a fresh-map key-by-key `ReasoningLevels` merge.
4. `internal/provider/builtin.go`: drop `DefaultProvider` from `builtinPi`; add `ReasoningLevels` to pi/claude ONLY if FR-D5-verified (else nil + `TODO(FR-D5)`); leave the other 6 nil.
5. The 6 non-test `Render` call sites updated to the new arity (`reasoning=""` temporarily).
6. `internal/decompose/roles.go`: remove the dead `inferenceProvider` func + its ResolveRoles guard block (DefaultProvider gone); keep `isMultiProvider`; `TODO(P1.M2.T1.S2)`.
7. `providers/pi.toml`: remove `default_provider`.

**Success Definition**: `go build ./...` is green — production compiles with the new Render chokepoint
enforcing FR-R5b (model-prefix fold + no-slash error) and emitting reasoning tokens; the `DefaultProvider`
field is gone from the schema; `CmdSpec` and `provider.Execute` are unchanged. (The ~41+ test call sites
across ~13 test files will NOT compile until the immediately-following S2 rewrites them — this is the
expected keystone state; S1's gate is `go build ./...`, not `go test`.)

## User Persona

**Target User**: The Stagehand contributor implementing the v3 config/provider/decompose subtasks that
depend on this keystone (S2 test rewrites, P1.M2 per-role reasoning plumbing + ResolveRoles v3, P1.M3
config v3 migration, P2 qwen-code, P3 decompose freeze). This is a foundation subtask — no end-user-visible
behavior change in isolation (call sites pass `reasoning=""`).

**Use Case**: Every downstream v3 subtask references the new `Render` arity, `Manifest.ReasoningLevels`,
and the absence of `DefaultProvider`. P1.M2 wires real per-role reasoning values into the 6 call sites and
reworks ResolveRoles to use the model-prefix guard.

**Pain Points Addressed**: Eliminates the provider/sub-provider conflation bug class at its root — there
is no longer a separate inference-provider field to forget or to confuse with the agent name. The prefix
on `model` is the single source of truth, enforced at the one chokepoint.

## Why

- **The keystone; everything flows through Render.** Every command-emission path (v1 generate, all four
  decompose roles) calls `Manifest.Render`. Folding the inference provider into the model slash-prefix AT
  RENDER means no path can produce an unroutable command — the single gate (system_context.md §1).
- **Removes the field that caused the conflation bugs.** `DefaultProvider` was a separate inference-backend
  field that callers repeatedly confused with the manifest name; the prior FR-R5b backstop keyed off it and
  was itself defeated (bugfix history). Eliminating the field and making the prefix authoritative is the
  structural cure.
- **Unblocks FR-R6 (reasoning) cleanly.** The new `reasoning` param + `ReasoningLevels` table let Render
  emit thinking-effort tokens with a graceful no-op (FR-R6) when a provider/model lacks them — no separate
  flag plumbing needed at the chokepoint.
- **Low-risk call-site ripple.** Per `scout_render_callsites.md §A`, the `provider` param is the literal
  `""` at EVERY non-test call site today (Render resolves the backend internally), so dropping it is purely
  mechanical; only the new `reasoning=""` arg is added.
- **CmdSpec/Execute untouched.** The change is internal to Render; the pure-data `CmdSpec` is unchanged, so
  `provider.Execute` needs no edits (scout §B).

## What

A production-only refactor: schema change (remove DefaultProvider, add ReasoningLevels), Render signature +
body change (model-prefix fold, FR-R5b error, reasoning emit), merge/builtin/call-site ripple, one
decompose/roles.go dead-code removal, and a one-line toml doc fix. No new behavior visible to end users in
isolation (call sites pass `reasoning=""`).

### Success Criteria

- [ ] `Manifest` has NO `DefaultProvider` field; HAS `ReasoningLevels map[string][]string \`toml:"reasoning_levels"\``.
- [ ] `Resolve()` no longer references DefaultProvider; ReasoningLevels is nil-safe (nil stays nil).
- [ ] `Validate()` is unchanged (no new rules).
- [ ] `Render` signature is `Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode)`.
- [ ] Render: `provider_flag` provider (pi) + `backend/model` → `--provider <backend> --model <rest>`; + bare model (no `/`) → HARD ERROR `model %q on %s must be inference/model, e.g. "zai/glm-5.2"`.
- [ ] Render: no-`provider_flag` provider → model verbatim.
- [ ] Render: `reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0` → append tokens; else silent no-op (never an error).
- [ ] `MergeManifest` drops the DefaultProvider block; adds a fresh-map key-by-key ReasoningLevels merge.
- [ ] `builtinPi` has no DefaultProvider; pi/claude ReasoningLevels populated only if FR-D5-verified (else nil+TODO); other 6 nil.
- [ ] The 6 non-test Render call sites use the new arity with `reasoning=""`.
- [ ] `decompose/roles.go` `inferenceProvider` + its guard removed; `isMultiProvider` kept; TODO for P1.M2.T1.S2.
- [ ] `providers/pi.toml` has no `default_provider`.
- [ ] `CmdSpec` and `provider.Execute` unchanged.
- [ ] **`go build ./...` is green** (the S1 gate). `go vet`/`go test` will fail on test files until S2 — expected.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact current Manifest fields, the exact current Render body
(to replace), the exact MergeManifest regimes, the 8 builtins (which carries DefaultProvider), all 6
non-test call sites with their new arity, the one subtle compile break (`inferenceProvider`) and its
minimal fix, and the executable validation (`go build` + a `go run` smoke). The architecture scouts
(`system_context.md §1`, `scout_render_callsites.md`) pre-resolved the call-site census, the
CmdSpec-unchanged finding, and the S1/S2 test boundary.

### Documentation & References

```yaml
# MUST READ — the binding architecture (do not re-litigate)
- docfile: plan/003_6ce49c39466e/architecture/system_context.md
  why: "§1 is THE keystone spec: the target Render signature, the model-prefix fold rule (FR-R5b), the reasoning emit rule (FR-R6), the exact non-test call-site ripple table, the Manifest struct deltas, the builtin/pi.toml changes, and the CmdSpec-unchanged guarantee."
  critical: "§1 states the provider param is the literal '' at every non-test call site (low-risk removal) and that DefaultProvider is consumed at render.go + decompose/roles.go. The decompose/roles.go consumption is the one compile break S1 must minimally fix (research §9)."

- docfile: plan/003_6ce49c39466e/architecture/scout_render_callsites.md
  why: "§A enumerates the 6 non-test Render call sites (file:line + current args + v3 args). §B confirms Execute/CmdSpec need NO changes. §D maps every DefaultProvider/DefaultModel/ProviderFlag non-test reference. §E counts the ~41 TEST call sites (S2's scope, NOT S1). §F is the change-impact summary."
  critical: "§E + §F: the ~41 test call sites are S2's job. S1's gate is go build ./... (production). Do NOT try to make go test green in S1."

# The production files under edit
- file: internal/provider/manifest.go
  why: "EDIT. Manifest struct (remove DefaultProvider line 59; add ReasoningLevels); Resolve() (delete DefaultProvider nil-default lines 159-160; ReasoningLevels nil-safe); struct doc comment. Validate() unchanged. strPtr/boolPtr helpers reused."
  pattern: "Pointer scalars default via `if out.X == nil { out.X = <ptr> }`; maps/slices left nil (Env/BareFlags/TooledFlags precedent). ReasoningLevels follows the map regime — leave nil in Resolve."
  gotcha: "ReasoningLevels reads are nil-safe (m[k]→nil, len(nil)==0) — do NOT allocate an empty map in Resolve. Do NOT add a Validate rule for it."

- file: internal/provider/render.go
  why: "EDIT (THE keystone body). Current Render(model, provider, sysPrompt, userPayload, mode...) → new Render(model, sysPrompt, userPayload, reasoning, mode...). Replace the providerToUse derivation + old FR-R5b backstop + --provider emit with the model-prefix fold + no-slash error + reasoning emit. ADD import \"strings\"."
  pattern: "Token order v3: subcommand → (provider_flag split from model prefix, FR-R5b) → model_flag → reasoning tokens (FR-R6) → system_prompt_flag → mode(bare/tooled) → print_flag → delivery switch. Rest (sys-prepend fallback, Env) UNCHANGED."
  gotcha: "Split on FIRST '/' only (strings.Index). After split, modelToUse becomes the REST (post-prefix). The no-slash error fires ONLY when *r.ProviderFlag != \"\" && modelToUse != \"\" && no '/'. Non-provider_flag providers (opencode + single-backend) pass model VERBATIM (opencode's 'openai/gpt-5.4' is its own combined form — do NOT split it)."

- file: internal/provider/merge.go
  why: "EDIT. Delete the override.DefaultProvider regime-1 block (lines 59-60). Add a fresh-map key-by-key ReasoningLevels merge (regime-3 style, adapted for map[string][]string). MUST allocate a fresh map (out := base aliases base.ReasoningLevels)."
  pattern: "Mirror the Env block: `if len(override.ReasoningLevels) > 0 { merged := make(...); copy base; copy override (override key wins); out.ReasoningLevels = merged }`."

- file: internal/provider/builtin.go
  why: "EDIT. builtinPi (line 50): DELETE `DefaultProvider: strPtr(\"\")`. Add ReasoningLevels to pi+claude ONLY if FR-D5-verified; else nil + TODO(FR-D5). Other 6: no field change (DefaultProvider already nil). BuiltinManifests map unchanged (7 entries; qwen-code = P2)."
  gotcha: "Only builtinPi sets DefaultProvider. The other builtins' 'DefaultProvider is NIL' COMMENT blocks are harmless to compilation; clean them in Mode-A for doc quality where S1 edits. Do NOT add qwen-code here (P2.M1)."

- file: internal/decompose/roles.go
  why: "EDIT (minimal compile fix). inferenceProvider (lines 185-190) returns *m.Resolve().DefaultProvider → breaks when the field is removed. Its ResolveRoles guard (lines 143-156) is the role-named FR-R5b EARLY re-check; the proper v3 (model-prefix) rework is P1.M2.T1.S2."
  pattern: "DELETE inferenceProvider + the guard block; replace with `// TODO(P1.M2.T1.S2): re-add the role-named FR-R5b guard using the model slash-prefix`. KEEP isMultiProvider (unused package-level funcs are legal in Go; P1.M2 reuses it)."
  gotcha: "Do NOT neuter inferenceProvider to `return \"\"` — that would make the guard ALWAYS fire for pi and break decompose. REMOVE the guard entirely; Render's new chokepoint enforces FR-R5b for all paths meanwhile."

# The 6 non-test Render call sites
- file: internal/generate/generate.go
  why: "EDIT line ~196 (inside CommitStaged's generate→dedupe loop): Render(cfg.Model, \"\", sysPrompt, payload) → Render(cfg.Model, sysPrompt, payload, \"\"). Drop the provider arg, add reasoning=\"\"."
- file: pkg/stagehand/stagehand.go
  why: "EDIT line ~461 (runPipeline): Render(cfg.Model, \"\", sysPrompt, payload) → Render(cfg.Model, sysPrompt, payload, \"\")."
- file: internal/decompose/planner.go
  why: "EDIT line ~98: Render(mdl, \"\", sysPrompt, payload, provider.RenderBare) → Render(mdl, sysPrompt, payload, \"\", provider.RenderBare)."
- file: internal/decompose/message.go
  why: "EDIT line ~129: same shape as planner."
- file: internal/decompose/arbiter.go
  why: "EDIT line ~97: same shape as planner."
- file: internal/decompose/stager.go
  why: "EDIT line ~86: Render(mdl, \"\", \"\", task, provider.RenderTooled) → Render(mdl, \"\", task, \"\", provider.RenderTooled)."

- file: providers/pi.toml
  why: "EDIT (Mode A doc). Remove the `default_provider = ...` line (~line 52). Only pi.toml carries it."

# Read-only refs (do NOT edit in S1)
- file: internal/provider/executor.go
  why: "Execute(ctx, *spec, timeout, vb) consumes CmdSpec. CmdSpec is UNCHANGED → NO edits. (scout §B.)"
- file: internal/config/roles.go
  why: "ResolveRoleModel(role, cfg) returns (provider, model); decompose sites discard provider (`_,`). P1.M2.T1.S2 makes it 3-return (+reasoning). NOT edited in S1."
- file: internal/provider/registry.go
  why: "Registry.DefaultProvider(installed) is the auto-DETECT METHOD (FR-D1), NOT the removed field — UNAFFECTED. Do not touch."

- docfile: plan/003_6ce49c39466e/P1M1T1S1/research/s1_implementation_notes.md
  why: "Distilled S1 findings: the verbatim new Render body, the merge ReasoningLevels block, the inferenceProvider removal, the pi/claude ReasoningLevels FR-D5 guidance, the call-site arity table, and the S1/S2 + go-build-gate boundary."
```

### Current Codebase Tree (relevant slice)

```bash
stagehand/
├── internal/provider/
│   ├── manifest.go        # EDIT (struct -DefaultProvider +ReasoningLevels; Resolve; doc)
│   ├── render.go          # EDIT (keystone: new signature + body; +strings import)
│   ├── merge.go           # EDIT (-DefaultProvider block; +ReasoningLevels fresh-map merge)
│   ├── builtin.go         # EDIT (-DefaultProvider from pi; +ReasoningLevels pi/claude-if-verified)
│   ├── executor.go        # read-only — CmdSpec/Execute UNCHANGED
│   └── registry.go        # read-only — DefaultProvider METHOD unaffected
├── internal/decompose/
│   ├── roles.go           # EDIT (remove dead inferenceProvider + guard; keep isMultiProvider)
│   ├── planner.go         # EDIT (Render arity, 1 line)
│   ├── message.go         # EDIT (Render arity, 1 line)
│   ├── arbiter.go         # EDIT (Render arity, 1 line)
│   └── stager.go          # EDIT (Render arity, 1 line)
├── internal/generate/generate.go   # EDIT (Render arity, 1 line)
├── pkg/stagehand/stagehand.go      # EDIT (Render arity, 1 line)
└── providers/pi.toml               # EDIT (remove default_provider)
```

### Desired Codebase Tree After S1

```bash
stagehand/
└── (only existing files modified — no new files)
    internal/provider/{manifest,render,merge,builtin}.go   # schema + keystone + merge + builtins
    internal/decompose/{roles,planner,message,arbiter,stager}.go  # dead-code removal + 4 arity fixes
    internal/generate/generate.go        # arity fix
    pkg/stagehand/stagehand.go           # arity fix
    providers/pi.toml                    # -default_provider
# NOTE: ~13 _test.go files will NOT compile until S2 (P1.M1.T1.S2). This is EXPECTED — see Validation.
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/provider/manifest.go` | MODIFY | -DefaultProvider; +ReasoningLevels; Resolve; doc. |
| `internal/provider/render.go` | MODIFY | New Render signature + body (FR-R5b fold/error + FR-R6 emit); +strings. **Keystone.** |
| `internal/provider/merge.go` | MODIFY | -DefaultProvider block; +ReasoningLevels fresh-map merge. |
| `internal/provider/builtin.go` | MODIFY | -DefaultProvider (pi); +ReasoningLevels (pi/claude if verified). |
| `internal/decompose/roles.go` | MODIFY | Remove dead `inferenceProvider` + guard; keep `isMultiProvider`; TODO P1.M2. |
| `internal/decompose/{planner,message,arbiter,stager}.go` | MODIFY | Render arity (drop provider arg, add `""` reasoning). |
| `internal/generate/generate.go` | MODIFY | Render arity. |
| `pkg/stagehand/stagehand.go` | MODIFY | Render arity. |
| `providers/pi.toml` | MODIFY | Remove `default_provider` (Mode A doc). |

**Explicitly NOT touched in S1**: all `*_test.go` (S2 / P1.M1.T1.S2), `internal/provider/executor.go` +
`CmdSpec` (unchanged), `internal/provider/registry.go` (DefaultProvider METHOD unaffected),
`internal/config/*` (P1.M2/P1.M3), `internal/cmd/*`, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (S1 gate): changing the Render arity breaks ~41 TEST call sites in ~13 _test.go files.
// `go build ./...` does NOT compile _test.go, so it is GREEN and is S1's gate ("a compiling tree").
// `go vet ./...` and `go test ./...` WILL FAIL on test compilation until S2 (P1.M1.T1.S2). This is
// the EXPECTED keystone state — do NOT try to fix test files in S1 (that is S2's whole job).

// CRITICAL (the one production compile break): removing DefaultProvider breaks decompose/roles.go
// inferenceProvider(m) which does `return *m.Resolve().DefaultProvider`. REMOVE inferenceProvider AND
// its ResolveRoles guard block (lines ~143-156). Do NOT neuter it to `return ""` (that makes the guard
// always fire for pi and breaks decompose). Removing the guard is SAFE: S1's new Render enforces FR-R5b
// at the chokepoint for all paths. P1.M2.T1.S2 re-adds the role-named (model-prefix) guard.

// CRITICAL (FR-R5b split): split on the FIRST '/' only (strings.Index, NOT strings.Split — you need
// the index to slice prefix/rest). Only split when *r.ProviderFlag != "". opencode has ProviderFlag=""
// and takes 'openai/gpt-5.4' VERBATIM (its own combined form) — do NOT split it. After split, the REST
// (post-prefix) is what the model_flag emits.

// CRITICAL (FR-R5b error wording): the no-slash error must name an example: `model %q on %s must be
// inference/model, e.g. "zai/glm-5.2"`. It fires ONLY when *r.ProviderFlag != "" && modelToUse != "" &&
// no '/' in modelToUse. A non-provider_flag provider with a bare model (claude 'sonnet') is VALID —
// passes verbatim, no error.

// CRITICAL (FR-R6 no-op): reasoning emit is `if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0`.
// Absent level / nil map / empty token list ⇒ SILENT no-op. NEVER return an error for a missing level.
// Reads on a nil ReasoningLevels map are safe (m[k]→nil slice, len(nil)==0) — do NOT init an empty map.

// GOTCHA (merge aliasing): out := base aliases base.ReasoningLevels (map header). The ReasoningLevels
// merge MUST allocate a FRESH map and copy both sides (like the Env block) — mutating out.ReasoningLevels
// in place would corrupt the caller's base. Slices (Subcommand/BareFlags/TooledFlags) are safe because
// you only reassign the header; maps are reference types and need the fresh-map copy.

// GOTCHA (FR-D5 reasoning tokens): the exact pi/claude thinking-effort flag tokens are NOT verified.
// Leaving ReasoningLevels nil for pi+claude in S1 (+ a TODO(FR-D5) comment) is SAFE — FR-R6 makes it a
// graceful no-op, and S1's call sites pass reasoning="" anyway. Only populate if you can verify the
// tokens; mark any populated map with a `// TO CONFIRM (FR-D5)` comment.

// GOTCHA (Resolve): ReasoningLevels is left nil (like Env/BareFlags/TooledFlags) — NOT defaulted to an
// empty map. Update Resolve's "left as-is" comment + the struct doc comment's field enumeration to name
// ReasoningLevels.

// GOTCHA (isMultiProvider): keep it in decompose/roles.go even though its only current caller (the
// removed guard) is gone — unused package-level FUNCTIONS are legal in Go (only unused locals/imports
// error). P1.M2.T1.S2 reuses it. Do NOT delete it.
```

## Implementation Blueprint

### Data models and structure

Schema-only deltas (no new types). The two regimes (pointer scalars vs maps/slices) govern placement:
`ReasoningLevels` follows the MAP regime (like `Env`). The relevant existing types/helpers (unchanged):
`CmdSpec` (render.go), `RenderMode`/`RenderBare`/`RenderTooled` (render.go), `strPtr`/`boolPtr` (manifest.go).

```go
// NEW field (manifest.go) — map regime, nil-safe
ReasoningLevels map[string][]string `toml:"reasoning_levels"` // [reasoning_levels] subtable; nil/empty ⇒ FR-R6 no-op

// REMOVED field
// DefaultProvider *string `toml:"default_provider"`   // gone — the model slash-prefix replaces it (FR-R5b)

// Render signature (render.go) — provider param removed, reasoning added
func (m Manifest) Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: manifest.go — schema + Resolve + doc
  - REMOVE the `DefaultProvider *string \`toml:"default_provider"\`` field (line 59) + its doc line.
  - ADD (map regime, near Env or after TooledFlags):
        // --- reasoning levels (v3; §12.1, FR-R6) ---
        // Per-level flag tokens appended at Render to express reasoning/thinking effort (off|low|medium|high).
        // nil/empty ⇒ graceful no-op (provider/model lacks reasoning control) — NEVER an error. Decoded from
        // the [reasoning_levels] subtable. Map regime (like Env): nil is the natural "absent" sentinel.
        ReasoningLevels map[string][]string `toml:"reasoning_levels"`
  - Resolve(): DELETE the DefaultProvider nil-default block (lines 159-160). Do NOT add a ReasoningLevels
    default (nil stays nil). Update the "left as-is" comment to name ReasoningLevels.
  - Update the struct doc comment's slice/map enumeration to include ReasoningLevels; remove the
    DefaultProvider mention.
  - Validate(): NO change.

Task 2: render.go — the keystone (new signature + body)
  - ADD import `"strings"` (needed for strings.Index). Keep "fmt", "os".
  - CHANGE the signature:
        func (m Manifest) Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error)
  - REPLACE the model/provider fallback + old FR-R5b backstop + --provider emit with the v3 logic. Body
    skeleton (see Implementation Patterns for the verbatim region):
        r := m.Resolve()
        modelToUse := model
        if modelToUse == "" { modelToUse = *r.DefaultModel }
        args := make([]string, 0, 16)
        args = append(args, r.Subcommand...)
        // FR-R5b fold + no-slash error (provider_flag providers only)
        if *r.ProviderFlag != "" && modelToUse != "" {
            if i := strings.Index(modelToUse, "/"); i >= 0 {
                args = append(args, *r.ProviderFlag, modelToUse[:i])
                modelToUse = modelToUse[i+1:]
            } else {
                return nil, fmt.Errorf("provider render %q: model %q on %s must be inference/model, e.g. \"zai/glm-5.2\"", m.Name, modelToUse, m.Name)
            }
        }
        if *r.ModelFlag != "" && modelToUse != "" { args = append(args, *r.ModelFlag, modelToUse) }
        // FR-R6 reasoning emit (silent no-op)
        if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0 { args = append(args, r.ReasoningLevels[reasoning]...) }
        // ... SystemPromptFlag, mode ternary, print_flag, delivery switch, Env — UNCHANGED from current ...
  - DELETE the now-dead providerToUse derivation, the old FR-R5b backstop (providerToUse=="" error), and
    the `--provider <providerToUse>` emit.
  - UPDATE the Render doc comment's token-order block to v3 (provider folds into model; reasoning after
    model flag; no provider param).

Task 3: merge.go — drop DefaultProvider, add ReasoningLevels merge
  - DELETE the regime-1 block `if override.DefaultProvider != nil { out.DefaultProvider = override.DefaultProvider }`.
  - ADD (regime-3 style, fresh map) after the Env block:
        if len(override.ReasoningLevels) > 0 {
            merged := make(map[string][]string, len(base.ReasoningLevels)+len(override.ReasoningLevels))
            for k, v := range base.ReasoningLevels { merged[k] = v }
            for k, v := range override.ReasoningLevels { merged[k] = v }
            out.ReasoningLevels = merged
        }
  - GOTCHA: fresh map is MANDATORY (maps alias on out := base).

Task 4: builtin.go — 8 builtins
  - builtinPi: DELETE the `DefaultProvider: strPtr("")` line. ReasoningLevels: nil + `// TODO(FR-D5):
    reasoning_levels tokens once verified` (conservative) OR populate with verified tokens + `// TO CONFIRM`.
  - builtinClaude: ReasoningLevels: same FR-D5 guidance (nil+TODO unless verified). No DefaultProvider to remove (was nil).
  - Other 6 (gemini/agy/opencode/codex/cursor): leave ReasoningLevels nil; no field change.
  - Optionally clean stale "DefaultProvider is NIL" comment lines in the files S1 materially edits (Mode A).
  - BuiltinManifests map: UNCHANGED (7 entries; do NOT add qwen-code — that is P2.M1).

Task 5: the 6 non-test Render call sites — new arity (reasoning="" temporarily)
  - generate.go:196   → deps.Manifest.Render(cfg.Model, sysPrompt, payload, "")
  - stagehand.go:461  → deps.Manifest.Render(cfg.Model, sysPrompt, payload, "")
  - planner.go:98     → deps.Manifest.Render(mdl, sysPrompt, payload, "", provider.RenderBare)
  - message.go:129    → deps.Manifest.Render(mdl, sysPrompt, payload, "", provider.RenderBare)
  - arbiter.go:97     → deps.Manifest.Render(mdl, sysPrompt, payload, "", provider.RenderBare)
  - stager.go:86      → deps.Manifest.Render(mdl, "", task, "", provider.RenderTooled)
  - Each: drop the literal "" provider arg; insert "" for reasoning. P1.M2 wires real per-role reasoning.

Task 6: decompose/roles.go — remove the dead DefaultProvider consumer (compile fix)
  - DELETE the `inferenceProvider` function (lines ~185-190).
  - DELETE the ResolveRoles guard block that uses it (lines ~143-156); replace with a one-line comment:
        // TODO(P1.M2.T1.S2): re-add the role-named FR-R5b guard using the model slash-prefix
        // (a bare model on a provider_flag agent ⇒ error). Render now enforces FR-R5b at the chokepoint.
  - KEEP `isMultiProvider` (unused package-level func is legal; P1.M2 reuses it).
  - GOTCHA: do NOT neuter inferenceProvider to return "" — remove the guard entirely.

Task 7: providers/pi.toml — Mode A doc
  - Remove the `default_provider = ...` line (~line 52). (Only pi.toml carries it.)

Task 8: VALIDATE
  - RUN: go build ./...        # S1's GREEN gate (production compiles; _test.go excluded)
  - EXPECT: go vet ./... and go test ./... FAIL on test-file compilation until S2 — this is EXPECTED.
  - SMOKE (throwaway, proves the keystone logic without the S2 tests): see Validation Loop Level 4.
```

### Implementation Patterns & Key Details

```go
// === render.go — the new Render keystone region (verbatim) ===
func (m Manifest) Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("provider render %q: %w", m.Name, err)
	}
	r := m.Resolve()

	modelToUse := model
	if modelToUse == "" {
		modelToUse = *r.DefaultModel
	}

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
	// ... [mode ternary (bare/tooled), print_flag, sys-prepend payload fallback, delivery switch,
	//      Env build] — UNCHANGED from the current Render body ] ...
}
```

```go
// === merge.go — the ReasoningLevels fresh-map merge (verbatim) ===
	// (place after the Env block; regime-3 style, adapted for map[string][]string)
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
```

### Integration Points

```yaml
MANIFEST SCHEMA (internal/provider/manifest.go):
  - removed: "DefaultProvider *string"
  - added:   "ReasoningLevels map[string][]string `toml:\"reasoning_levels\"`"  # map regime, nil-safe
  - Resolve: -DefaultProvider default; ReasoningLevels left nil (no empty-map init)
  - Validate: UNCHANGED

RENDER (internal/provider/render.go) — THE KEYSTONE:
  - signature: "Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode)"
  - FR-R5b: provider_flag + "backend/model" → split → --provider <backend> --model <rest>; no "/" → HARD ERROR
  - FR-R6:  reasoning tokens appended after model flag; absent/empty ⇒ silent no-op
  - import: + "strings"

MERGE (internal/provider/merge.go):
  - removed: DefaultProvider regime-1 block
  - added:   ReasoningLevels fresh-map key-by-key merge

BUILTINS (internal/provider/builtin.go):
  - builtinPi: -DefaultProvider; ReasoningLevels nil+TODO(FR-D5) [or verified tokens + TO CONFIRM]
  - builtinClaude: ReasoningLevels nil+TODO(FR-D5)
  - other 6: nil; BuiltinManifests map unchanged (7 entries; qwen-code = P2)

CALL SITES (6 non-test, reasoning="" temporarily — P1.M2 wires real values):
  - generate.go:196, stagehand.go:461, planner.go:98, message.go:129, arbiter.go:97, stager.go:86

DEAD-CODE REMOVAL (internal/decompose/roles.go):
  - removed: inferenceProvider func + its ResolveRoles guard block
  - kept:    isMultiProvider (P1.M2 reuses)
  - TODO:    P1.M2.T1.S2 re-adds the role-named model-prefix FR-R5b guard

DOC (providers/pi.toml): removed `default_provider`

NO-TOUCH (explicitly):
  - internal/provider/executor.go + CmdSpec   # unchanged (scout §B)
  - internal/provider/registry.go             # DefaultProvider METHOD (auto-detect) unaffected
  - internal/config/*                         # P1.M2 (reasoning plumbing) / P1.M3 (config v3 migration)
  - all *_test.go                             # S2 (P1.M1.T1.S2) — the ~41 Render test call sites + new tests
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by LATER subtasks, NOT S1):
  - S2 (P1.M1.T1.S2): rewrite the ~41 Render test call sites + add FR-R5b (model-prefix) + reasoning render tests
  - P1.M2.T1.S1: Config.Reasoning field + file/overlay/env/flag plumbing
  - P1.M2.T1.S2: ResolveRoles v3 (model-prefix FR-R5b guard) + 3-return ResolveRoleModel + wire real reasoning into the 6 call sites
  - P1.M3: config v3 migration (remove default_provider from user configs on load/upgrade)
  - P2.M1: qwen-code built-in + ReasoningLevels tokens
```

## Validation Loop

### Level 1: Build (S1's GREEN gate)

```bash
cd /home/dustin/projects/stagehand

go build ./...            # EXPECTED: exit 0 — production compiles (the "compiling tree" deliverable)

# EXPECTED FAILURES (do NOT chase — these are S2's scope):
#   go vet ./...   → fails: test files reference the old Render arity / removed DefaultProvider field
#   go test ./...  → fails: same (test compilation)
# These fail until S2 (P1.M1.T1.S2) rewrites the ~41 test call sites. This is the EXPECTED keystone state.
```

### Level 2: Compile-Only Checks on Edited Packages (catch typos early)

```bash
cd /home/dustin/projects/stagehand

# Build ONLY the production packages you edited (excludes _test.go) — fast signal, no test-breakage noise
go build ./internal/provider/ ./internal/decompose/ ./internal/generate/ ./pkg/stagehand/
gofmt -l internal/provider/manifest.go internal/provider/render.go internal/provider/merge.go \
          internal/provider/builtin.go internal/decompose/*.go internal/generate/generate.go \
          pkg/stagehand/stagehand.go
# Expected: empty (run gofmt -w on any listed file).
```

### Level 3: Targeted Verification (grep invariants — no test run needed)

```bash
cd /home/dustin/projects/stagehand

# DefaultProvider is GONE from non-test production source (only the registry METHOD + comments may remain)
grep -rn "DefaultProvider" --include="*.go" . | grep -v "_test.go" | grep -v "/plan/"
# Expected: NO field references (`.DefaultProvider`) in manifest/merge/builtin/render/decompose-roles;
#           only `Registry.DefaultProvider(` (the method) + benign comment lines remain.

# Render new arity at all 6 non-test sites
grep -rn "\.Render(" --include="*.go" internal/ pkg/ | grep -v "_test.go"
# Expected: every non-test Render call has 4 string-ish positional args (+ optional mode), NO provider arg.

# ReasoningLevels field present; Render signature updated
grep -n "ReasoningLevels map" internal/provider/manifest.go
grep -n "func (m Manifest) Render(model, sysPrompt, userPayload, reasoning string" internal/provider/render.go
grep -n "strings.Index(modelToUse" internal/provider/render.go   # FR-R5b split
grep -n 'must be inference/model' internal/provider/render.go    # FR-R5b error

# decompose/roles.go: inferenceProvider removed; isMultiProvider kept
grep -n "func inferenceProvider" internal/decompose/roles.go     # Expected: NO match (removed)
grep -n "func isMultiProvider" internal/decompose/roles.go       # Expected: 1 match (kept)

# providers/pi.toml: default_provider removed
grep -n "default_provider" providers/pi.toml                     # Expected: NO match
```

### Level 4: Keystone Behavior Smoke (throwaway — the S2 tests will codify these permanently)

```bash
cd /home/dustin/projects/stagehand
cat > /tmp/sh_keystone_smoke.go <<'EOF'
package main
import ("fmt"; "github.com/dustin/stagehand/internal/provider")
func pi() provider.Manifest {
	pf, mf, spf, prf := "--provider", "--model", "--system-prompt", "pi"
	t := true
	return provider.Manifest{
		Name: "pi", Command: &prf, ProviderFlag: &pf, ModelFlag: &mf, SystemPromptFlag: &spf,
		Output: &"raw"[0:0:0][:0:0][:0:0], // (placeholder; build a real *string below)
		StripCodeFence: &t, PromptDelivery: &"stdin"[0:0:0][:0:0][:0:0],
	}
}
func main() {
	// (a) FR-R5b fold: "zai/glm-5.2" -> --provider zai --model glm-5.2
	// (b) FR-R5b error: "glm-5.2" (no slash) -> error
	// (c) no provider_flag (claude) + "sonnet" -> verbatim --model sonnet
	// (d) reasoning emit when declared; no-op when not
	fmt.Println("see PRP Task 8 smoke — build manifests via the package's exported fields")
}
EOF
# NOTE: provider.strPtr is unexported, so a /tmp main in `package main` CANNOT set pointer fields
# directly. Instead, validate via a TEMPORARY in-package test that you DELETE after (the permanent
# tests are S2). Write internal/provider/zz_s1_smoke_test.go (package provider), run, then remove it:
cat > internal/provider/zz_s1_smoke_test.go <<'EOF'
package provider
import ("strings"; "testing")
func TestS1_KeystoneSmoke(t *testing.T) {
	pi := Manifest{Name:"pi", Command:strPtr("pi"), ProviderFlag:strPtr("--provider"),
		ModelFlag:strPtr("--model"), SystemPromptFlag:strPtr("--system-prompt"),
		PromptDelivery:strPtr("stdin"), Output:strPtr("raw"), StripCodeFence:boolPtr(true)}
	// (a) fold
	s,_ := pi.Render("zai/glm-5.2", "sys", "payload", "")
	if !contains(s.Args,"--provider")||!contains(s.Args,"zai")||!contains(s.Args,"glm-5.2")||contains(s.Args,"zai/glm-5.2") {t.Fatal("fold",s.Args)}
	// (b) no-slash error
	if _,err:=pi.Render("glm-5.2","sys","payload","");err==nil{t.Fatal("want no-slash error")}
	// (c) claude verbatim
	cl:=Manifest{Name:"claude",Command:strPtr("claude"),ModelFlag:strPtr("--model"),PromptDelivery:strPtr("stdin"),Output:strPtr("raw"),StripCodeFence:boolPtr(true)}
	sc,_:=cl.Render("sonnet","sys","payload","")
	if !contains(sc.Args,"sonnet"){t.Fatal("verbatim",sc.Args)}
	// (d) reasoning emit + no-op
	pi2:=pi; pi2.ReasoningLevels=map[string][]string{"high":{"--thinking","high"}}
	rh,_:=pi2.Render("zai/m","sys","payload","high")
	if !contains(rh.Args,"--thinking"){t.Fatal("reasoning emit",rh.Args)}
	rn,_:=pi.Render("zai/m","sys","payload","high") // no table -> no-op, no error
	_ = rn
}
func contains(s []string, w string) bool { for _,x:=range s { if x==w {return true} }; return false }
var _ = strings.TrimSpace
EOF
go test -run TestS1_KeystoneSmoke ./internal/provider/   # NOTE: will fail to COMPILE if render_test.go
# is also present and broken — so run this BEFORE the broader test breakage matters, or temporarily.
# After it passes, DELETE internal/provider/zz_s1_smoke_test.go (S2 owns the permanent tests).
rm -f internal/provider/zz_s1_smoke_test.go
```

> **Smoke caveat:** because the OTHER test files in `internal/provider/` (render_test.go etc.) call the
> OLD Render arity, `go test ./internal/provider/` will not compile even with the smoke test added. The
> cleanest way to run the smoke is to make it the ONLY compilable test temporarily, or hand-trace the 4
> cases against the verbatim Render body in Implementation Patterns. The permanent codification is S2.

## Final Validation Checklist

### Technical Validation

- [ ] **`go build ./...` exits 0** (S1's gate — production compiles).
- [ ] `gofmt -l` on all edited production files reports nothing.
- [ ] `go vet ./...` / `go test ./...` are EXPECTED to fail on test compilation (S2 owns the fix) — NOT a S1 failure.

### Feature Validation

- [ ] `Manifest` has no `DefaultProvider`; has `ReasoningLevels map[string][]string` (`toml:"reasoning_levels"`).
- [ ] `Resolve()` no DefaultProvider default; ReasoningLevels nil-safe (left nil).
- [ ] `Render` signature = `Render(model, sysPrompt, userPayload, reasoning, mode...)`.
- [ ] Render: pi + `zai/glm-5.2` → `--provider zai --model glm-5.2`; + bare model → HARD ERROR (FR-R5b).
- [ ] Render: claude (no provider_flag) + `sonnet` → verbatim `--model sonnet` (no split, no error).
- [ ] Render: opencode + `openai/gpt-5.4` (provider_flag="") → verbatim (NOT split — its own combined form).
- [ ] Render: reasoning tokens appended when declared; silent no-op when absent (FR-R6, never an error).
- [ ] `MergeManifest`: no DefaultProvider block; fresh-map key-by-key ReasoningLevels merge.
- [ ] `builtinPi`: no DefaultProvider; pi/claude ReasoningLevels nil+TODO(FR-D5) (or verified tokens).
- [ ] 6 non-test Render call sites use the new arity (`reasoning=""`).
- [ ] `decompose/roles.go`: `inferenceProvider` + guard removed; `isMultiProvider` kept; TODO P1.M2.
- [ ] `providers/pi.toml`: no `default_provider`.
- [ ] `CmdSpec` + `provider.Execute` unchanged.

### Scope Discipline Validation

- [ ] ONLY production files in the edit list are modified (git diff --stat confirms).
- [ ] Did NOT edit any `*_test.go` (S2 / P1.M1.T1.S2).
- [ ] Did NOT edit `executor.go` / `CmdSpec` / `registry.go` (DefaultProvider METHOD).
- [ ] Did NOT touch `internal/config/*` (P1.M2 / P1.M3) or `internal/cmd/*`.
- [ ] Did NOT add qwen-code or its ReasoningLevels (P2.M1).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Field regimes match (ReasoningLevels = map regime like Env; nil-safe).
- [ ] FR-R5b split uses `strings.Index` (first `/`); only for `provider_flag` providers.
- [ ] ReasoningLevels merge allocates a fresh map (no base aliasing).
- [ ] Stale DefaultProvider comment references cleaned in materially-edited files (Mode A).
- [ ] `isMultiProvider` retained (not deleted) for P1.M2.

---

## Anti-Patterns to Avoid

- ❌ Don't try to make `go test ./...` green in S1 — the ~41 test call sites are S2's whole job. S1's gate
  is `go build ./...`. Chasing test compilation in S1 crosses the subtask boundary and wastes effort.
- ❌ Don't neuter `inferenceProvider` to `return ""` to keep it compiling — that makes the ResolveRoles
  guard ALWAYS fire for pi and breaks decompose. REMOVE the function + its guard; Render's new chokepoint
  enforces FR-R5b meanwhile. P1.M2.T1.S2 restores the role-named guard.
- ❌ Don't split the model on `/` for providers WITHOUT a `provider_flag` (opencode, all single-backend).
  opencode's `openai/gpt-5.4` is its OWN combined form — pass it verbatim. Split ONLY when `*r.ProviderFlag != ""`.
- ❌ Don't use `strings.Split` — use `strings.Index` so you can slice `prefix = m[:i]`, `rest = m[i+1:]`.
  (Split also works but Index is cleaner for "first slash only".)
- ❌ Don't make a missing reasoning level an error. FR-R6 is explicit: absent/empty ⇒ silent no-op. The
  guard is `reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0` — and a nil map read is safe.
- ❌ Don't allocate an empty `ReasoningLevels` map in `Resolve()` — leave it nil (like Env/BareFlags). Reads
  on a nil map are safe; an empty map adds noise with no benefit.
- ❌ Don't mutate `out.ReasoningLevels` in place in `MergeManifest` — `out := base` aliases base's map.
  Allocate a FRESH map and copy both sides (mirror the Env block).
- ❌ Don't populate pi/claude `ReasoningLevels` with guessed tokens. FR-D5 requires verification; an
  unverified token list could emit a wrong flag. Default to nil+TODO(FR-D5) (graceful no-op) unless verified.
- ❌ Don't delete `isMultiProvider` from decompose/roles.go — it becomes unused but that's legal for a
  package-level func, and P1.M2.T1.S2 reuses it.
- ❌ Don't add qwen-code or change the BuiltinManifests map / registry priority — those are P2.M1.
- ❌ Don't change `CmdSpec` or `provider.Execute` — the change is internal to Render (scout §B confirms).

---

## Confidence Score

**8.5/10** for one-pass implementation success.

Rationale: This is the well-specified keystone — the architecture (`system_context.md §1` +
`scout_render_callsites.md`) pre-resolved the new Render signature, the model-prefix fold rule (FR-R5b),
the reasoning emit rule (FR-R6), the exact 6 non-test call-site ripple, the CmdSpec-unchanged guarantee,
and the field/merge regimes. The verbatim new Render body and the fresh-map merge are given. The two
non-obvious traps — (1) the `decompose/roles.go` `inferenceProvider` compile break and its correct
minimal fix (remove, don't neuter), and (2) the S1/S2 test-boundary making `go test` an EXPECTED failure
with `go build ./...` as the real gate — are both called out explicitly with the reasoning. The residual
uncertainty (not 9–10): the FR-D5 reasoning-token verification for pi/claude (mitigated: nil+TODO is the
safe default), and the smoke-test friction (the other broken test files impede `go test` even for an
in-package smoke) — mitigated by hand-tracing against the verbatim body and by S2 codifying the permanent
tests. Downstream subtasks (S2 tests, P1.M2 ResolveRoles+reasoning, P1.M3 config migration) are cleanly
fenced and cannot be broken by S1 because call sites pass `reasoning=""` and the field removal's only
production consumer (`inferenceProvider`) is removed in-task.
