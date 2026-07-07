# Issue 1: Provider/Sub-Provider Conflation (Critical)

## Root Cause

`Manifest.Render(model, provider, sysPrompt, payload, mode...)` in `internal/provider/render.go`
treats its `provider` parameter as the **sub-provider/backend** (e.g. `zai`, `openrouter`). When
the param is `""`, Render falls back to `*r.DefaultProvider` (the manifest's resolved sub-provider).

**Every caller passes the manifest/agent NAME** (e.g. `"pi"`) as this parameter, NOT the sub-provider:

### Call Site 1: Single-commit path (`internal/generate/generate.go`)
```go
// Line ~192 (inside the generate→dedupe loop):
spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
//                                              ^^^^^^^^^^^
// cfg.Provider is the MANIFEST NAME ("pi"), passed as the sub-provider.
```
`cfg.Provider` comes from `config.Load` and is always the manifest/agent name (pi, claude, gemini…)
or `""` (auto-detect). It is NEVER the sub-provider.

### Call Site 2-5: Decompose role files (`internal/decompose/{planner,stager,message,arbiter}.go`)
Each role file derives `(prov, mdl)` via `config.ResolveRoleModel(role, deps.Config)`:
```go
// planner.go:28, stager.go:38, message.go:77, arbiter.go:55
prov, mdl := config.ResolveRoleModel("planner", deps.Config)
// ...
spec, rerr := deps.Roles.Planner.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)
//                                               ^^^^
// prov = cfg.Provider (manifest name "pi") — same bug.
```

`ResolveRoleModel` (`internal/config/roles.go:28`) returns `cfg.Provider` as the `provider` value
(falling back through role → global). This IS the manifest name. It's correct for looking up the
manifest via `reg.Get(prov)` in `ResolveRoles` (roles.go), but WRONG when passed to Render as the
sub-provider.

### What Render does with it
```go
// render.go:
providerToUse := provider  // = "pi" (manifest name)
if providerToUse == "" {
    providerToUse = *r.DefaultProvider  // NEVER REACHED — "pi" is non-empty
}
// Emits: --provider pi (the manifest name, not the sub-provider)
```

### Consequence
1. `--provider pi` is emitted, which is not a valid pi sub-provider.
2. The user's configured `default_provider` (e.g. `"openrouter"`) in `[provider.pi]` is **silently
   ignored** — it survives the FR37a merge into the manifest's `DefaultProvider` field, but Render
   never reads it because the caller overrides with the manifest name.
3. The FR37a fix that preserves `default_provider` across config layers is **entirely defeated**.
4. This triggers in ALL common pi configurations: bootstrap config, `--provider pi`, `git config
   stagecoach.provider pi`, `STAGECOACH_PROVIDER=pi`.

## Why CI Doesn't Catch It

The shipped render unit tests (`TestRender_GoldenPerProvider`, `TestRender_PersonalOverride`) invoke
`Render` **directly** with the sub-provider string (`""` or `"zai"`), bypassing the caller
conflation. The caller-level tests (`generate_test.go`, `decompose/*_test.go`) don't assert on the
provider token in the rendered command.

## The Fix

Pass `""` for the provider parameter at ALL call sites, so Render falls back to `*r.DefaultProvider`
(the merged sub-provider):

| File | Current | Fixed |
|------|---------|-------|
| `generate.go` (~L192) | `Render(cfg.Model, cfg.Provider, ...)` | `Render(cfg.Model, "", ...)` |
| `planner.go` (~L75) | `Render(mdl, prov, ...)` | `Render(mdl, "", ...)` |
| `stager.go` (~L50) | `Render(mdl, prov, ...)` | `Render(mdl, "", ...)` |
| `message.go` (~L103) | `Render(mdl, prov, ...)` | `Render(mdl, "", ...)` |
| `arbiter.go` (~L73) | `Render(mdl, prov, ...)` | `Render(mdl, "", ...)` |

**No changes needed to:**
- `render.go` — its fallback logic is already correct.
- `config/roles.go` — `ResolveRoleModel` correctly returns the manifest name for `reg.Get()`; the
  callers just stop passing it to Render.
- `provider/merge.go` — FR37a merge is already correct.

**What still works after the fix:**
- When `default_provider = "openrouter"` is set in `[provider.pi]`: merge preserves it → Render
  falls back to it → emits `--provider openrouter`. ✓
- When `default_provider` is not set (pi default): `*r.DefaultProvider == ""` → Render omits
  `--provider`. ✓ (This is the correct pi default per §12.3.)
- Non-pi providers (claude, gemini, etc.): `ProviderFlag == ""` → Render never emits `--provider`
  regardless of the param. ✓

## Test Strategy

1. **Caller-level unit tests**: Build a real manifest (pi-shaped) with `DefaultProvider="openrouter"`,
   call `CommitStaged`/each decompose role via the stub, and assert the rendered `CmdSpec.Args`
   contains `--provider openrouter` (NOT `--provider pi`). Use `deps.Verbose` to capture the rendered
   command, OR inspect the stub's received args via an env-var probe.

2. **End-to-end integration test**: Drive the real CLI binary with a pi-shaped stubagent config
   (`default_provider = "openrouter"`), run `stagecoach --dry-run --verbose`, and assert the DEBUG
   command line shows `--provider openrouter`. This mirrors the PRD's reproduction steps.

## Files to Touch

| File | Change | Doc Mode |
|------|--------|----------|
| `internal/generate/generate.go` | Pass `""` for provider param (1 line) | JSDoc on Render call |
| `internal/decompose/planner.go` | Pass `""` for provider param (1 line) | none — no user-facing change |
| `internal/decompose/stager.go` | Pass `""` for provider param (1 line) | none |
| `internal/decompose/message.go` | Pass `""` for provider param (1 line) | none |
| `internal/decompose/arbiter.go` | Pass `""` for provider param (1 line) | none |
| `internal/generate/generate_test.go` | Assert sub-provider rendering | — |
| `internal/decompose/*_test.go` | Assert sub-provider rendering per role | — |
| `internal/cmd/default_action_test.go` | E2E integration test | — |
