# P4.M1.T1.S1 — Research Findings (FR51b progress label)

## Work item
Progress label `<Verb> with <model> in <provider>…` (PRD §9.13 FR51b) across the generate (single-commit)
and decompose paths. Main line surfaces MESSAGE role (generate) / PLANNER role (decompose); `--verbose`
enumerates all four decompose roles.

## CRITICAL: no conflict with the parallel sibling (P3.M2.T1.S2)
P3.M2.T1.S2 (one-file short-circuit) edits `internal/decompose/decompose.go` (inserts `runOneFileShortcut`
between the freeze and `callPlanner`). It does NOT touch `internal/ui/*` or the progress label call sites
in `internal/cmd/default_action.go`. No file overlap. This task edits `internal/ui/output.go`,
`internal/ui/verbose.go`, `internal/cmd/default_action.go`, + tests. Safe to consume P3.M2.T1.S2's
`Decompose`/`ResolveRoles` outputs as-is (RoleModels shape is already the P1.M2.T1.S2 v3 form).

## Current state (verified)

### Progress plumbing
- `ui.Progress(msg string)` (output.go:103) writes `↳ <msg>\n` to STDERR (FR51: stdout clean for piping).
  Plain (not colorized). Doc comment currently shows the OLD example `"Generating with pi (glm-5.2)…"` →
  Mode-A docs task: update to the FR51b form.
- TWO call sites, both in `internal/cmd/default_action.go`:
  - GENERATE path (~line 147): builds label inline — `"Generating"` + (`" with "+cfg.Provider` + optional
    `" ("+cfg.Model+")"`) + `"…"`. Uses cfg.Provider/cfg.Model DIRECTLY (no autodetect → when
    `--provider` is unset the label is just `"Generating…"`).
  - DECOMPOSE path (`runDecompose`, ~line 294): `"Decomposing"` + (`" with "+cfg.Provider`) + `"…"`; does
    NOT use cfg.Model; DISCARDS `ResolveRoles`'s 2nd return (RoleModels) with `_`.

### Role resolution (the source of resolved provider/model/reasoning)
- `decompose.ResolveRoles(cfg, reg) (RoleManifests, RoleModels, error)` (roles.go:89). RoleModels has
  `Planner/Stager/Message/Arbiter config.RoleConfig`; `config.RoleConfig = {Provider, Model, Reasoning}`
  (config.go:34). These are the RESOLVED values (post-auto-detect, post-FR-D4 stager fallback, reasoning
  resolved per-field → global → shipped default via `config.ResolveRoleModel`).
- `config.ResolveRoleModel(role, cfg) (provider, model, reasoning string)` — planner reasoning defaults to
  `"high"` (defaultRoleReasoning); stager/message/arbiter default to `""` (off). So in the verbose
  enumeration the planner line carries `(reasoning: high)`; the others omit it.
- For the GENERATE path there is NO ResolveRoles call (single role = message = global). The resolved
  provider must be derived at the CLI: `cfg.Provider`, or `reg.DefaultProvider(installed)` when `""`
  (mirror `pkg/stagecoach.buildDeps`). Model = `cfg.Model` (empty ⇒ provider-only label per FR51b).

### Verbose sink
- `ui.Verbose` (verbose.go) has `VerboseCommand`/`VerboseRawOutput`/`VerboseRetry`, all `"DEBUG: …\n"` to
  `v.w`, no-op when `v==nil || v.w==nil || !v.on`. The CLI builds `ui.NewVerbose(stderr, cfg.Verbose)`
  (currently inline in `runDecompose` as `deps.Verbose`). No generic "print a debug line" method and no
  roles method — this task adds `VerboseRoles`.

### Test harness (verified safe)
- `output_test.go`: tests pass the EXACT string to `Progress` (e.g. `u.Progress("Generating…")`) and assert
  the `↳ …\n` bytes. They do NOT exercise the formatted label → unaffected by the new helper.
- `default_action_test.go`: errBuf assertions are for `"(no commit created)"` and rescue/CAS messages —
  NO test asserts on the `"Generating with …"` label text. So changing generate's label from
  `"Generating with stub (m)…"` to `"Generating with m in stub…"` breaks nothing.
- `verbose_test.go`: `VerboseCommand`/`VerboseRawOutput`/`VerboseRetry` unit tests — mirror for
  `VerboseRoles`.

## Design decisions

### ui.ProgressLabel(verb, model, provider string) string  (output.go, pure)
Factor an unexported `invocation(model, provider string) string` (shared by ProgressLabel + VerboseRoles):
- provider == "" → "" (nothing resolved; e.g. generate path with no provider installed)
- model != ""    → `<model> in <provider>`  (e.g. `zai/glm-5.2 in pi`, `sonnet in claude`)
- model == ""    → `<provider>`             (FR51b "show <provider> alone")
ProgressLabel:
- invocation == "" → `<verb>…`            (preserves the old minimal fallback)
- else             → `<verb> with <invocation>…`
Examples: `Generating with sonnet in claude…`, `Generating with zai/glm-5.2 in pi…`,
`Generating with claude…` (model empty), `Decomposing with anthropic/claude-sonnet-4 in opencode…`.

### ui.VerboseRoles(roles []ui.RoleLine)  (verbose.go)
`RoleLine{Name, Model, Provider, Reasoning string}`. Per role:
`DEBUG: %-8s <invocation><suffix>\n` where suffix = `(reasoning: <level>)` when level ∈ {low,medium,high},
else "". No-op when nil/w==nil/!on (same guard idiom as the other Verbose methods). The %-8s pads role
names so the invocations align (planner/stager/message/arbiter are 7/6/7/7 chars).

### Generate path (single-commit) — default_action.go
Restructure the validate+label block to build the registry ONCE (always), resolve the message-role
provider (autodetect when cfg.Provider==""), keep the existing explicit-provider `reg.Get` validation,
then `u.Progress(ui.ProgressLabel("Generating", cfg.Model, providerName))`. Now handles the
`DecodeUserOverrides` error (the old code ignored it with `_`).

### Decompose path — runDecompose (default_action.go)
Capture `roleModels` (drop the `_`); build `verbose := ui.NewVerbose(stderr, cfg.Verbose)` once; call
`verbose.VerboseRoles([...4 roles from roleModels...])` (no-op unless --verbose); main line
`u.Progress(ui.ProgressLabel("Decomposing", roleModels.Planner.Model, roleModels.Planner.Provider))`;
reuse `verbose` as `deps.Verbose` (currently built inline). Order: main label first, then verbose roles.

## Reasoning in the label? NO
FR51b main label = `<model> in <provider>` only (PRD examples have no reasoning). Reasoning appears ONLY
in the `--verbose` 4-role enumeration.

## Files touched (4 edits, 0 new files)
EDIT internal/ui/output.go     — +ProgressLabel +invocation (unexported) ; update Progress doc comment.
EDIT internal/ui/verbose.go    — +RoleLine +VerboseRoles.
EDIT internal/cmd/default_action.go — generate label via ProgressLabel (+autodetect); runDecompose
  captures roleModels + VerboseRoles + planner label; add `"github.com/dustin/stagecoach/internal/ui"`
  import if not present (it IS already imported).
EDIT tests: output_test.go (+TestProgressLabel_*), verbose_test.go (+TestVerboseRoles_*),
  default_action_test.go (+ label observable via errBuf, both paths).
go.mod/go.sum UNCHANGED. Validation: `go build ./... && go vet ./... && go test -race ./... &&
golangci-lint run && gofmt -l internal/ pkg/`. ui IS NOT coverage-gated (gated: git/provider/generate/
config) — but ProgressLabel/VerboseRoles get tests for the codebase's doc-first style + to lock the FR51b
byte format.
