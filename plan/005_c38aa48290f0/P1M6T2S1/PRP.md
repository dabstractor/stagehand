name: "P1.M6.T2.S1 — config init --interactive wizard"
description: |

---

## Goal

**Feature Goal**: Add `config init --interactive` (PRD §9.23 FR-L3 / §15.3): a TTY-gated wizard that
walks the user through (1) picking a provider from the DETECTED set (FR-D1 default highlighted), (2)
accepting or editing each of that provider's per-role model defaults (FR-D4), and (3) — for a
multi-backend provider (pi, opencode) — prompting for the `inference/` model prefix on any EDITED model
rather than guessing (FR-D2 / FR-R5b). It then writes the SAME file `config init` writes (FR-B1), via a
SHARED bootstrap generator. Non-TTY stdin → exit 1 pointing at plain `config init` (which MUST stay
non-interactive — FR-B3 runs it from post-install scripts / first-run fallback).

**Deliverable**:
1. `internal/config/bootstrap.go` (EDIT) — refactor the PURE `buildBootstrapConfig(target, installed)`
   to take a 3rd param `overrides map[string]string` (role→model); add the public seam
   `GenerateBootstrapConfigWithOverrides(prov string, overrides map[string]string) string`; refactor
   `GenerateBootstrapConfig(prov)` to delegate (`return GenerateBootstrapConfigWithOverrides(prov, nil)`,
   byte-identical). Branch pi's NOTE so it never lies when the wizard fills models.
2. `internal/provider/registry.go` (EDIT) — add exported `PreferredBuiltins() []string` (the FR-D1
   priority order, for the wizard's detected-set menu ordering). NOT a manifest-schema change.
3. `internal/cmd/config.go` (EDIT) — register `--interactive` on `configInitCmd`; branch at the TOP of
   `runConfigInit` to route to the interactive path; extract a shared
   `writeBootstrapFile(cmd, path, content, force) error` (force-check + MkdirAll + WriteFile) used by
   BOTH paths.
4. `internal/cmd/config_init_interactive.go` (NEW) — `flagConfigInteractive` var, the overridable TTY-gate
   package var, `runConfigInitInteractive` RunE-helper, the PURE `runInteractiveWizard`, the
   `needsInferencePrefix` predicate, menu/render helpers.
5. `internal/cmd/config_init_interactive_test.go` (NEW) — pure wizard tests (golden byte-identity +
   edits + multi-backend re-prompt + invalid-choice re-prompt), non-TTY gate Execute test, Execute
   happy-path file-write (forced TTY + SetIn), `--interactive --template` conflict, `--interactive
   --provider` pre-select, `--interactive --force` overwrite.
6. `docs/cli.md` (EDIT) — `--interactive` row + paragraph + example in the `### config init` section.
7. `docs/configuration.md` (EDIT) — an "Interactive bootstrap" note in the Bootstrap section.

**Success Definition**:
- `go build ./...`, `go test ./internal/cmd/... ./internal/config/... ./internal/provider/... -v`,
  `go vet ./...`, `golangci-lint run`, `gofmt -l` all green.
- `stagehand config init --interactive` with a TTY walks the three-step flow and writes a config that is
  BYTE-IDENTICAL to `GenerateBootstrapConfig(<chosen>)` when all defaults are accepted, and differs only
  in the edited role-model lines (+ pi note) when edits are made (the keystone contract).
- A multi-backend provider (pi/opencode): editing a model to a bare value (no `/`) is rejected with a
  re-prompt; the wizard never writes a bare model that FR-R5b would hard-error on.
- Non-TTY stdin (e.g. piped/dev-null) → exit 1 with a message pointing at plain `stagehand config init`.
- `--interactive` composes: with `--force` (overwrites), with `--provider <name>` (pre-selects); errors
  clearly on `--interactive --template` (mutually exclusive).
- Plain `stagehand config init` (no `--interactive`) is UNCHANGED in behavior and byte-output
  (regression-safe — the refactor delegates, nil overrides = identity).

## User Persona

**Target User**: the "plan-holder" (PRD §7.1) and "multi-agent tinkerer" (§7.3) bootstrapping stagehand
for the first time, and the "API-key refusenik" (§7.2) who wants guided setup without editing TOML by
hand.

**Use Case**: `stagehand config init --interactive` detects installed agents, highlights the FR-D1
default, and lets the user accept curated per-role models or type their own — with guardrails so a
multi-backend model always carries its inference prefix.

**Pain Points Addressed**: hand-editing TOML is error-prone (FR-R5b's slash-prefix rule bites silently);
the plain `config init` writes pi's models BLANK (correctly, FR-D2) but offers no guided way to fill
them. The wizard front-ends the SAME writer so the result is always a valid FR-B1 config.

## Why

- **FR-L3 (PRD §9.23 / §15.3)**: the feature contract — TTY-gated wizard that picks from the detected
  set (FR-D1 default highlighted), shows per-role defaults (FR-D4) for accept-or-edit, prompts for the
  inference prefix on multi-backend edited models (FR-D2/FR-R5b), writes the FR-B1 file; composes with
  `--force`.
- **FR-B1 / FR-B3 (§9.17)**: the wizard is a TTY front-end over the SAME writer; plain `config init`
  stays non-interactive because FR-B3 runs it from post-install scripts / first-run fallback.
- **architecture/system_context.md §3**: "`runConfigInit` (`internal/cmd/config.go`) +
  `config.GenerateBootstrapConfig(provider)` (`internal/config/bootstrap.go`) already implement FR-B1;
  the wizard is a TTY front-end that writes the SAME file shape. FR-D1 detection order lives in
  `provider.Registry.DefaultProvider` / `preferredBuiltins`. … TTY-detection precedent in internal/ui."
- **Scope fences (P1.M6.T1.S2 parallel task)**: S2 delivers `stagehand models [<provider>]` (the
  read-only listing). This task (S1 of T2) delivers the WRITE-side wizard. The two share data
  (`DefaultModelsForProvider`, `ListModelsCommand`, the registry) but do not edit each other's files.
  This task does NOT touch the manifest schema, `providers/*.toml`, `builtin.go`, or `merge.go` (S1's
  contract). It adds ONE non-schema helper to `registry.go` (`PreferredBuiltins`) and edits
  `bootstrap.go`'s generator (additive — nil overrides = identity).

## What

A `--interactive` flag on `config init` that, when stdin is a TTY, runs a guided wizard producing a
populated FR-B1 config via the shared `GenerateBootstrapConfigWithOverrides` generator. The wizard is
non-destructive (refuses to overwrite unless `--force`) and writes to the resolved config path (honors
`--config` / `STAGEHAND_CONFIG`, exactly like plain `config init`). All prompts go to stdout; the
written path is printed on success.

### Success Criteria

- [ ] `configInitCmd` has a `--interactive` bool flag; `runConfigInit` routes to the interactive path
      when set (before the template/force checks).
- [ ] Non-TTY stdin → `exitcode.New(exitcode.Error, …)` exit 1, message points at plain `config init`.
- [ ] The wizard lists DETECTED providers in FR-D1 priority order with the detected default highlighted;
      if nothing is detected → exit 1 pointing at plain `config init` (which defaults to pi).
- [ ] Per-role accept-or-edit (planner/stager/message/arbiter) over `DefaultModelsForProvider` defaults;
      empty line = accept default, non-empty = the value.
- [ ] Multi-backend (pi/opencode): a non-empty EDITED model lacking `/` is rejected with a re-prompt
      (never guessed, never written bare).
- [ ] Output is `GenerateBootstrapConfigWithOverrides(chosen, overrides)`; accept-all ⇒ byte-identical
      to `GenerateBootstrapConfig(chosen)`.
- [ ] `--interactive --force` overwrites; `--interactive --provider <name>` pre-selects (skip the
      provider prompt, validate via `reg.Get`); `--interactive --template` is a usage error (exit 1).
- [ ] Plain `config init` (no `--interactive`) output is byte-unchanged (regression test).
- [ ] `docs/cli.md` + `docs/configuration.md` document `--interactive`.

## All Needed Context

### Context Completeness Check

_This PRP names the EXACT generator to refactor (`buildBootstrapConfig` → +overrides), the EXACT new
public seam (`GenerateBootstrapConfigWithOverrides`), the EXACT reused cmd helpers
(`newRegistry`/`installedNames`/`resolvedDefault` in providers.go, same package), the EXACT TTY
precedent (`ui.IsTerminal`), the EXACT multi-backend predicate and its membership (pi, opencode,
provider_flag), the EXACT flag-composition matrix, the EXACT answer protocol for scriptable tests, and
the EXACT docs insertion points. An implementer with no prior codebase knowledge can build it from this
document + codebase access._

### Documentation & References

```yaml
- file: internal/cmd/config.go
  why: THE host of `config init`. runConfigInit does: path=ResolveConfigPath(flagConfig) → force check →
       MkdirAll → template? → provider validate (reg.Get) → GenerateBootstrapConfig → WriteFile → print.
       Register --interactive in init() next to --provider/--force/--template; branch at the TOP of
       runConfigInit. The exampleConfigTemplate / preferredBuiltins (LOCAL, stale) live here too.
  pattern: |
       configInitCmd.Flags().String("provider", "", "...")
       func runConfigInit(cmd, args) error { path := config.ResolveConfigPath(flagConfig); force, _ := cmd.Flags().GetBool("force"); ... }
  gotcha: |
    cmd/config.go's LOCAL `preferredBuiltins` is STALE (missing "qwen-code") — DO NOT reuse it for the
    wizard's menu order; use the new `provider.PreferredBuiltins()` instead. (The stale copy only feeds
    the --provider validation error message; leave it.)

- file: internal/config/bootstrap.go
  why: THE generator to refactor. GenerateBootstrapConfig(prov) → buildBootstrapConfig(target, installed).
       buildBootstrapConfig computes models=DefaultModelsForProvider(target); piBlanked; stagerFallback;
       writes header + config_version + [defaults] + 4 [role.*] + commented others + [generation].
       Refactor to buildBootstrapConfig(target, installed, overrides); add GenerateBootstrapConfigWithOverrides.
  pattern: |
       func GenerateBootstrapConfig(prov string) string { return GenerateBootstrapConfigWithOverrides(prov, nil) }
       func GenerateBootstrapConfigWithOverrides(prov string, overrides map[string]string) string { ...buildBootstrapConfig(target, installed, overrides) }
  gotcha: |
    (1) Apply overrides AFTER the pi-blank + stagerFallback computation (so the stager-provider routing
    is preserved; only the MODEL values change). (2) For pi (piBlanked), branch the NOTE: no overrides →
    existing "empty, supply your own" note (byte-identical); overrides present → shorter format-focused
    note (don't write "models are empty" when the wizard filled them). (3) stager override sets
    stagerModel only (NOT stagerName). (4) Nil/empty overrides ⇒ byte-identical output (the golden test).

- file: internal/cmd/providers.go
  why: THE reused cmd helpers (same package — no import). newRegistry() (nil Config() ⇒ built-ins only);
       installedNames(reg) (detected names); resolvedDefault(Config(), reg, installed) (nil cfg ⇒
       reg.DefaultProvider(installed), the FR-D1 detected default). config init is in shouldSkipConfigLoad
       so Config() is nil during the wizard — newRegistry handles that (uses no overrides).
  pattern: reg, err := newRegistry(); installed := installedNames(reg); dflt := resolvedDefault(Config(), reg, installed)
  gotcha: Do NOT re-implement these; call them directly (same package cmd). resolvedDefault(nil,...) is
          exactly the detected default the wizard wants to highlight.

- file: internal/provider/registry.go
  why: Add `PreferredBuiltins() []string` here (returns the unexported `preferredBuiltins` slice, or a
       copy). The FR-D1 order [pi, opencode, cursor, agy, gemini, qwen-code, codex, claude] is the
       authoritative source — the wizard orders its detected menu by it. Also: Get/IsInstalled/
       DefaultProvider/List are the registry surface (reuse).
  pattern: func (r *Registry) PreferredBuiltins() []string { return preferredBuiltins }
  gotcha: This is NOT a manifest-schema change (no Manifest field, no providers/*.toml, no builtin.go) →
          does not collide with P1.M6.T1.S1. The default HIGHLIGHT still comes from resolvedDefault/
          DefaultProvider (correct internally); PreferredBuiltins is only for menu ORDERING.

- file: internal/config/role_defaults.go
  why: DefaultModelsForProvider(name) returns map[role]model (a COPY). The wizard shows these per-role
       defaults; for pi they are the FR-D4 values BUT buildBootstrapConfig blanks them (piBlanked) — so
       the wizard's "default" display for pi is BLANK (correct: the user must supply backend/model).
  pattern: col := config.DefaultModelsForProvider("claude")  // {"planner":"opus",...}
  gotcha: For pi the per-role defaults from this function are non-blank (gpt-5.4 etc.) BUT the bootstrap
          BLANKS them. The wizard must display the BLANKED defaults (what will actually be written), not
          the raw table — i.e. show "" for pi roles. Simplest: derive display defaults from
          GenerateBootstrapConfig's logic (piBlanked) OR just show "" for pi and the table value otherwise.

- file: internal/ui/output.go
  why: THE TTY precedent. IsTerminal(f *os.File) bool = `(stat.Mode() & os.ModeCharDevice) != 0`;
       stat-error → false (safe non-TTY default). Used for FR51 color gating. The wizard's gate is the
       same idea on os.Stdin.
  pattern: if !ui.IsTerminal(os.Stdin) { return exitcode.New(exitcode.Error, ...) }
  gotcha: Make the predicate a package-level overridable var (`interactiveStdinIsTTY = func() bool {...}`)
          so the happy-path Execute test can force true while piping answers via rootCmd.SetIn(reader).

- file: internal/provider/render.go (lines ~105-118)
  why: FR-R5b enforcement — the AUTHORITATIVE chokepoint. A provider_flag provider (pi) with a non-empty
       model lacking "/" → HARD ERROR. opencode (no provider_flag) passes the model verbatim. The wizard
       PREVENTS the user from writing a config that Render would reject (for pi) or that is functionally
       wrong (for opencode) by re-prompting on a bare edited model.
  pattern: if *r.ProviderFlag != "" && modelToUse != "" && !strings.Contains(modelToUse, "/") { hard error }
  gotcha: The wizard's needsInferencePrefix must include opencode (Render won't catch a bare opencode
          model, but `openai/gpt-5.4` IS the correct form). Predicate: name=="pi" || name=="opencode" ||
          (m.ProviderFlag != nil && *m.ProviderFlag != "").

- file: internal/cmd/config_test.go
  why: THE test harness. setupNoRepo(t) (isolates HOME/XDG + chdir to a non-git temp dir), saveRootState/
       restoreRootState/resetFlags, writeConfigFile, rootCmd.SetArgs + Execute(context.Background()),
       exitcode.For(err) assertions. config init tests prove the file-write contract — mirror for the
       interactive tests. TestConfigInit_ProviderPin_ExactOutput is the byte-level precedent.
  pattern: setupNoRepo(t); rootCmd.SetArgs([]string{"config","init","--interactive"}); err := Execute(...)
  gotcha: Tests run with a NON-TTY stdin (test runner) → the non-TTY gate fires naturally in an Execute
          test. To test the happy path via Execute: set interactiveStdinIsTTY=true + rootCmd.SetIn(reader)
          + restore both in a defer.

- docfile: plan/005_c38aa48290f0/P1M6T1S2/PRP.md
  why: The PARALLEL task (models command). It adds `config.DefaultModelsVerificationDate` and the
       `models` command; it CONSUMES Manifest.ListModelsCommand. This task is ORTHOGONAL (the write-side
       wizard) but shares DefaultModelsForProvider + the registry. Do not duplicate its work; if it has
       shipped, the wizard MAY (optionally) mention `stagehand models` in its prompts, but that is NOT
       required by FR-L3 — keep the wizard self-contained.
  section: Goal + Anti-Patterns (the shared seams).

- url: PRD §9.23 FR-L3 + §9.17 FR-B1/B3 + §9.16 FR-D1/D2/D4 + §12/FR-R5b (in prd_snapshot.md)
  why: THE contracts. FR-L3 = the wizard flow; FR-B1 = the file shape; FR-B3 = plain init stays
       non-interactive; FR-D1 = detection order; FR-D2 = pi default blank, never guess a backend;
       FR-D4 = per-role tier table; FR-R5b = the inference/ slash-prefix hard error.
  section: "9.23", "9.17", "9.16", "9.15 FR-R5b".
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  config.go                      # HOST: configInitCmd + runConfigInit + exampleConfigTemplate + (stale) preferredBuiltins. EDIT (flag + branch + shared writer).
  providers.go                   # REUSE: newRegistry/installedNames/resolvedDefault (same package).
  config_test.go                 # TEST HARNESS: setupNoRepo/saveRootState/writeConfigFile/Execute. Mirror.
  root.go                        # shouldSkipConfigLoad (init→skip) + Config() (nil during init) + flagConfig. READ-ONLY.
internal/config/
  bootstrap.go                   # EDIT: buildBootstrapConfig +overrides; GenerateBootstrapConfigWithOverrides; delegate; pi note branch.
  role_defaults.go               # REUSE: DefaultModelsForProvider. READ-ONLY.
internal/provider/
  registry.go                    # EDIT: +PreferredBuiltins(). REUSE: Get/IsInstalled/DefaultProvider/List.
  render.go                      # READ-ONLY: FR-R5b hard-error (the rule the wizard prevents reaching).
internal/ui/output.go            # REUSE: IsTerminal (TTY precedent). READ-ONLY.
docs/
  cli.md                         # EDIT: config init section (+--interactive row/para/example).
  configuration.md               # EDIT: Bootstrap section (+interactive note).
```

### Desired Codebase tree with files to be added/edited

```bash
internal/config/bootstrap.go               # EDIT — buildBootstrapConfig(target, installed, overrides); GenerateBootstrapConfigWithOverrides; delegate; pi note branch
internal/provider/registry.go              # EDIT — +PreferredBuiltins() []string
internal/cmd/config.go                     # EDIT — register --interactive; branch in runConfigInit; extract writeBootstrapFile helper
internal/cmd/config_init_interactive.go    # NEW  — flagConfigInteractive; interactiveStdinIsTTY; runConfigInitInteractive; runInteractiveWizard; needsInferencePrefix; menu helpers
internal/cmd/config_init_interactive_test.go # NEW — pure wizard golden/edits/re-prompt; non-TTY gate; Execute happy-path; flag composition
docs/cli.md                                # EDIT — config init --interactive docs
docs/configuration.md                      # EDIT — interactive bootstrap note
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (byte-identity is the keystone contract): GenerateBootstrapConfig(prov) MUST stay byte-
//   identical after the refactor. Implement it as `return GenerateBootstrapConfigWithOverrides(prov, nil)`
//   and ensure buildBootstrapConfig(target, installed, nil) is byte-identical to today's two-arg form.
//   The golden test asserts GenerateBootstrapConfigWithOverrides(chosen, nil) == GenerateBootstrapConfig(chosen).

// CRITICAL (apply overrides AFTER pi-blank + stagerFallback): the stager-provider routing (stagerName)
//   is a structural decision the bootstrap owns; overrides change only MODEL values. stager override →
//   stagerModel (NOT stagerName). planner/message/arbiter overrides → models[role].

// CRITICAL (pi NOTE must not lie): when piBlanked AND overrides has any entry, emit the format-focused
//   note ("pi is multi-backend; models carry the inference/ prefix, e.g. zai/glm-5.2"), NOT the "models
//   are empty" note. No overrides → the existing note (byte-identical, golden test passes).

// CRITICAL (config init skips config.Load): shouldSkipConfigLoad returns true for "init" → Config() is
//   nil during the wizard. newRegistry() handles nil Config() (uses no overrides → built-ins only).
//   resolvedDefault(nil, reg, installed) = reg.DefaultProvider(installed) = the detected default. Do NOT
//   call config.Load or rely on cfg.Providers.

// CRITICAL (TTY gate is separate from the IO): ui.IsTerminal(os.Stdin) is the gate; the PURE wizard
//   takes an io.Reader (answers) + io.Writer (prompts) and contains NO os.Stdin/IsTerminal. Make the
//   gate a package var `interactiveStdinIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }` so the
//   Execute happy-path test forces true and pipes answers via rootCmd.SetIn(reader). The non-TTY gate
//   test runs unmodified (test stdin is not a TTY → gate fires → exit 1).

// CRITICAL (multi-backend = pi OR opencode OR provider_flag): needsInferencePrefix must include opencode
//   (Render won't hard-error a bare opencode model, but `openai/gpt-5.4` IS the correct form). Predicate:
//   name=="pi" || name=="opencode" || (m.ProviderFlag != nil && *m.ProviderFlag != ""). Registry manifests
//   are merged-but-UNRESOLVED → handle nil ProviderFlag. An EDITED non-empty model lacking "/" → re-prompt
//   (loop until valid or blank); NEVER write it bare.

// CRITICAL (stale preferredBuiltins in cmd/config.go): the LOCAL copy omits "qwen-code". DO NOT reuse it
//   for menu order. Use provider.PreferredBuiltins() (the new accessor over the authoritative slice).

// CRITICAL (pi display defaults are BLANK): DefaultModelsForProvider("pi") returns gpt-5.4 etc., but the
//   bootstrap BLANKS them (piBlanked). The wizard must show the BLANKED defaults (what gets written) for
//   pi roles — show "" for pi, the table value otherwise. (Otherwise the user "accepts" gpt-5.4 but the
//   file writes "" — confusing.)

// CRITICAL (never os.Exit): return exitcode.New(exitcode.Error, err) for every error path (exit 1); main
//   maps via exitcode.For. Matches config.go/providers.go.

// CRITICAL (--interactive --template is a usage error): they write different things (populated vs inert
//   reference). Exit 1 with a clear "choose one" message. --interactive --provider <name> pre-selects
//   (skip the provider prompt; validate via reg.Get; detection NOT required for an explicit pin).
```

## Implementation Blueprint

### Data models and structure

No new persistent data model. The wizard produces two values consumed by the existing generator:

```go
// The wizard's output: a chosen provider + per-role model overrides (role→model). overrides is nil/empty
// when the user accepts every default (→ byte-identical to GenerateBootstrapConfig(chosen)).
type wizardResult struct {
	provider  string
	overrides map[string]string // role ("planner"|"stager"|"message"|"arbiter") → model
}

// The new generator seam (internal/config/bootstrap.go):
//   GenerateBootstrapConfigWithOverrides(prov string, overrides map[string]string) string
// GenerateBootstrapConfig(prov) becomes: return GenerateBootstrapConfigWithOverrides(prov, nil)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/provider/registry.go — add PreferredBuiltins()
  - IMPLEMENT: `func (r *Registry) PreferredBuiltins() []string { return preferredBuiltins }` (returns
    the package-level preferredBuiltins slice — the FR-D1 order). Add a doc comment citing FR-D1 + that
    this is for menu ordering (the default HIGHLIGHT comes from DefaultProvider).
  - FOLLOW pattern: the existing DefaultProvider/FirstTooledProvider (which iterate preferredBuiltins).
  - NAMING: PreferredBuiltins (exported; method on *Registry for symmetry with Get/List).
  - DO NOT: change preferredBuiltins itself, the manifest schema, providers/*.toml, builtin.go, or merge.go.
  - DEPENDENCIES: none.

Task 2: EDIT internal/config/bootstrap.go — overrides-aware generator
  - IMPLEMENT:
    (a) `func GenerateBootstrapConfigWithOverrides(prov string, overrides map[string]string) string` —
        same detection as GenerateBootstrapConfig (reg := provider.NewRegistry(nil); installed :=
        bootstrapProviderNames(reg); target := prov or reg.DefaultProvider(installed) or "pi"), then
        `return buildBootstrapConfig(target, installed, overrides)`.
    (b) Refactor `func GenerateBootstrapConfig(prov string) string { return
        GenerateBootstrapConfigWithOverrides(prov, nil) }` (delete the old body).
    (c) Change `buildBootstrapConfig(target string, installed []string)` →
        `buildBootstrapConfig(target string, installed []string, overrides map[string]string)`.
    (d) AFTER the existing `models := DefaultModelsForProvider(target)` / piBlanked / stagerFallback
        block, APPLY overrides: for planner/message/arbiter, `if v, ok := overrides[role]; ok {
        models[role] = v }`; for stager, `if v, ok := overrides["stager"]; ok { stagerModel = v }`
        (stagerName UNCHANGED). Compute `piHasOverrides := piBlanked && len(overrides) > 0`.
    (e) Branch the pi NOTE: the existing `if piBlanked { <"empty, supply your own" note block> }` becomes
        `if piBlanked && !piHasOverrides { <existing note, byte-identical> } else if piBlanked &&
        piHasOverrides { <format-focused note: "pi is a multi-backend provider — prefix the model with
        your inference backend, e.g. model = \"zai/glm-5.2\". A bare model (no '/') on pi is a config
        error (FR-R5b)."> }`.
  - FOLLOW pattern: the existing buildBootstrapConfig structure (header → config_version → [defaults] →
    [role.*] → commented others → [generation]); writeRoleBlock/writeCommentedRoleBlock unchanged.
  - NAMING: GenerateBootstrapConfigWithOverrides, overrides param, piHasOverrides.
  - GOTCHA: nil/empty overrides ⇒ NONE of the override branches fire ⇒ byte-identical output (golden
    test). Do NOT touch stagerName. Do NOT change the no-override pi note text.
  - DEPENDENCIES: Task 1 (not strictly; can parallelize, but Task 1 is trivial — do first).

Task 3: EDIT internal/cmd/config.go — flag registration + branch + shared writer
  - IMPLEMENT:
    (a) In init(): add `configInitCmd.Flags().Bool("interactive", false, "Guided TTY wizard: pick a
        detected provider, accept or edit per-role models (prompts for the inference/ prefix on
        multi-backend providers); writes the same config as plain 'config init'")`.
    (b) At the TOP of runConfigInit, before the path/force/template logic:
        `if interactive, _ := cmd.Flags().GetBool("interactive"); interactive { return
        runConfigInitInteractive(cmd, args) }`.
    (c) Extract `func writeBootstrapFile(cmd *cobra.Command, path, content string, force bool) error`
        containing the force-check (os.Stat → "already exists" error unless force) + MkdirAll + WriteFile
        (0644). Refactor runConfigInit to call it (replacing the inline force-check + MkdirAll + WriteFile),
        keeping the template/populated print messages in runConfigInit. runConfigInitInteractive will call
        writeBootstrapFile too (DRY — one place for the file-write contract).
  - FOLLOW pattern: the existing runConfigInit force-check/MkdirAll/WriteFile (move, don't reinvent).
  - GOTCHA: keep runConfigInit's print messages ("Wrote config to" / "Wrote example config to") in
    runConfigInit; writeBootstrapFile does ONLY force-check + mkdir + write (returns nil/error). The
    interactive path prints "Wrote config to <path>" (it writes a populated config).
  - DEPENDENCIES: Task 4 (runConfigInitInteractive exists in the new file). Define writeBootstrapFile
    first so both paths compile.

Task 4: CREATE internal/cmd/config_init_interactive.go — the wizard
  - IMPLEMENT (package cmd):
    (a) `var flagConfigInteractive bool` (if you prefer a named var; or read via cmd.Flags().GetBool).
    (b) `var interactiveStdinIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }` — overridable gate.
    (c) `func needsInferencePrefix(name string, m provider.Manifest) bool { pf := ""; if m.ProviderFlag !=
        nil { pf = *m.ProviderFlag }; return name == "pi" || name == "opencode" || pf != "" }`.
    (d) `func runConfigInitInteractive(cmd *cobra.Command, args []string) error`:
        1. Composition: tmpl, _ := cmd.Flags().GetBool("template"); if tmpl → exitcode.New(Error,
           "--interactive writes a populated config; --template writes the inert reference — choose one").
        2. TTY gate: if !interactiveStdinIsTTY() → exitcode.New(Error, "--interactive requires a terminal
           on stdin; run plain 'stagehand config init' instead (it stays non-interactive for post-install
           scripts, FR-B3)").
        3. reg, err := newRegistry(); if err → exitcode.New(Error, fmt.Errorf("stagehand: %w", err)).
        4. installed := installedNames(reg); defaultName := resolvedDefault(Config(), reg, installed).
           if len(installed) == 0 → exitcode.New(Error, "no providers detected on $PATH; run plain
           'stagehand config init' to default to pi, or install one of: <provider.PreferredBuiltins()>").
        5. Pre-select? pinName, _ := cmd.Flags().GetString("provider"); if pinName != "" { validate via
           reg.Get(pinName); if !ok → exitcode.New(Error, "unknown provider %q ...") ; chosen = pinName }.
        6. result, err := runInteractiveWizard(cmd.InOrStdin(), cmd.OutOrStdout(), reg, installed,
           defaultName, pinName); if err → exitcode.New(Error, err).
        7. content := config.GenerateBootstrapConfigWithOverrides(result.provider, result.overrides).
        8. path := config.ResolveConfigPath(flagConfig); force, _ := cmd.Flags().GetBool("force"); if
           err := writeBootstrapFile(cmd, path, content, force); err → exitcode.New(Error, err).
        9. fmt.Fprintf(cmd.OutOrStdout(), "Wrote config to %s\n", path); return nil.
    (e) `func runInteractiveWizard(r io.Reader, w io.Writer, reg *provider.Registry, installed []string,
        defaultName, pinName string) (wizardResult, error)` — PURE (no os.Stdin/IsTerminal):
        1. Reader: `br := bufio.NewReader(r)`. Helper `readLine() (string, error)` reads one line
           (strings.TrimSpace); io.EOF on a prompt → error "unexpected end of input".
        2. Choose provider (ONLY if pinName == ""): list detected in provider.PreferredBuiltins() order
           (filter to installed), mark defaultName with "(default)". Prompt: "Detected providers:\n 1. pi
           (default)\n 2. claude\nPick a provider [<default>]: ". Read a line; "" → defaultName; else
           validate it is in the detected set (reg.Get + installed check); invalid → re-prompt
           ("unknown/undetected provider %q; choose from the list") until valid or EOF.
        3. chosen = pinName or the picked name. m, _ := reg.Get(chosen). multi := needsInferencePrefix.
        4. overrides := map[string]string{} (treat as "no overrides" if empty — see (7)).
        5. For each role in []string{"planner","stager","message","arbiter"}:
             - default display value: for pi (chosen=="pi") show "" (piBlanked); else
               DefaultModelsForProvider(chosen)[role] (for stager on a non-stager-capable provider, show
               the FALLBACK model — compute via the same logic, OR just show "" and let the user type;
               simplest correct: show config.DefaultModelsForProvider(chosen)["stager"] which is "" for
               non-stager-capable → show the fallback by calling the stager logic. To avoid duplicating
               stagerFallback, show "" for stager when the table value is "" and note "routed to <fb>").
               PRAGMATIC CHOICE: display DefaultModelsForProvider(chosen)[role]; if it's "" (pi-blanked or
               non-stager-capable stager), display "" with a hint. The ACCEPTED value written is "" in
               that case (matches buildBootstrapConfig), so no override is added on accept.
             - Prompt: "<role> model [<default>]: " (+ for multi: "; include the inference/ prefix, e.g.
               zai/glm-5.2"). Read a line; "" → accept default (do NOT add to overrides); else value:
                 if multi && !strings.Contains(value, "/") → re-prompt ("multi-backend provider: include
                 the inference backend as a prefix, e.g. zai/glm-5.2") until valid or blank.
                 else overrides[role] = value.
        6. return wizardResult{provider: chosen, overrides: overrides}.
        7. EMPTY-MAP DISCIPLINE: if len(overrides)==0, return overrides as NIL (not an empty map) so
           GenerateBootstrapConfigWithOverrides(chosen, nil) is byte-identical to GenerateBootstrapConfig
           (the golden test). `if len(overrides) == 0 { overrides = nil }`.
  - FOLLOW pattern: config.go (exitcode.New; cmd.OutOrStdout; ResolveConfigPath(flagConfig)); providers.go
    (newRegistry/installedNames/resolvedDefault). bufio.NewReader for line-oriented stdin (stdlib only).
  - NAMING: runConfigInitInteractive, runInteractiveWizard, needsInferencePrefix, wizardResult,
    interactiveStdinIsTTY. CamelCase funcs.
  - PLACEMENT: single new file internal/cmd/config_init_interactive.go.
  - DEPENDENCIES: Tasks 1-3 (PreferredBuiltins; GenerateBootstrapConfigWithOverrides; writeBootstrapFile;
    the --interactive flag registration in config.go init()). Imports: bufio, fmt, io, os, strings, cobra,
    internal/config, internal/exitcode, internal/provider, internal/ui.

Task 5: EDIT docs/cli.md — config init --interactive
  - IMPLEMENT: in the `### config init` section, (a) add a `--interactive` row to the Flag table
    ("Guided TTY wizard — pick a detected provider, accept or edit per-role models; prompts for the
    inference/ prefix on multi-backend providers (pi, opencode). Writes the same file as plain `config
    init`. Non-TTY → exit 1 (use plain `config init`)."); (b) add a paragraph + example block after the
    existing examples:
      `stagehand config init --interactive   # guided: pick provider, edit per-role models`
      `stagehand config init --interactive --provider pi   # pre-select pi, edit its models`
    Note the composition (--force, --provider) and the --template mutual-exclusion.
  - FOLLOW pattern: the existing config init flag table + example blocks.
  - PLACEMENT: within `### config init`, before `### config upgrade`. Do NOT duplicate the FR-D4 table.
  - DEPENDENCIES: Task 4.

Task 6: EDIT docs/configuration.md — interactive bootstrap note
  - IMPLEMENT: in the "Bootstrap (`config init`)" section (~line 33-50), add a short note: "`config init
    --interactive` runs a TTY-gated wizard: it lists detected providers (FR-D1 default highlighted),
    shows each role's curated default (FR-D4) for accept-or-edit, and — for multi-backend providers
    (pi, opencode) — prompts for the `inference/model` prefix on edited models (FR-D2/FR-R5b) rather than
    guessing. It writes the SAME file as plain `config init`. Non-TTY stdin exits 1 pointing at plain
    `config init` (which stays non-interactive for post-install/first-run use, FR-B3)."
  - FOLLOW pattern: the existing Bootstrap section prose.
  - PLACEMENT: the Bootstrap section, near the existing flag list.
  - DEPENDENCIES: Task 4.

Task 7: CREATE internal/cmd/config_init_interactive_test.go — the full test matrix
  - IMPLEMENT (reuse setupNoRepo/saveRootState/restoreRootState/resetFlags/writeConfigFile/Execute from
    the package; mirror config_test.go). Use t.Setenv to put a fake binary on PATH for DETECTION where
    needed (e.g. fake "claude" so it is detected; remember cursor's Detect="agent"):
    A. TestWizard_AcceptDefaults_ByteIdentical (PURE, the keystone): build a registry with a detected
       provider (e.g. fake claude on PATH, or call runInteractiveWizard with a hand-built reg). Pipe
       accept-all answers ("\n\n\n\n\n" for the 5 prompts, or 4 if --provider pre-selects). Assert
       result.provider == defaultName AND result.overrides == nil. Then assert
       `config.GenerateBootstrapConfigWithOverrides(result.provider, result.overrides) ==
       config.GenerateBootstrapConfig(result.provider)` (byte-identical — the contract).
    B. TestWizard_EditsSingleBackend (PURE): pipe "claude\nmy-plan\n\nmy-msg\n\n" → assert
       result.provider=="claude", result.overrides=={"planner":"my-plan","message":"my-msg"} (stager/
       arbiter absent). Assert GenerateBootstrapConfigWithOverrides output contains `model = "my-plan"`
       under [role.planner] and the DEFAULT under [role.stager]/[role.arbiter].
    C. TestWizard_MultiBackend_RePrompt (PURE, pi): pipe answers that first give a bare model
       ("gpt-5.4") then a valid prefixed one ("zai/gpt-5.4") for planner → assert the bare was rejected
       (a re-prompt line in the output buffer) and result.overrides["planner"]=="zai/gpt-5.4".
    D. TestWizard_MultiBackend_BlankAccepted (PURE, pi): accept-all on pi → overrides nil → output is
       the blank-models pi config (byte-identical to GenerateBootstrapConfig("pi")).
    E. TestWizard_InvalidProvider_RePrompt (PURE): pipe "ghost\nclaude\n..." → assert re-prompt + final
       chosen=="claude".
    F. TestWizard_EOF_IsError (PURE): pipe "" (immediate EOF) → error (truncated script fails loudly).
    G. TestInteractive_NonTTY_Exits1 (EXECUTE): setupNoRepo; rootCmd.SetArgs([]string{"config","init",
       "--interactive"}); do NOT set interactiveStdinIsTTY. Execute → err non-nil;
       exitcode.For(err)==exitcode.Error; err.Error() contains "terminal" and "config init".
    H. TestInteractive_HappyPath_WritesFile (EXECUTE): put a fake "claude" on PATH (t.Setenv PATH);
       interactiveStdinIsTTY = func() bool { return true } (defer restore); rootCmd.SetIn(strings.NewReader(
       "\n\n\n\n\n")) (accept-all); rootCmd.SetArgs([]string{"config","init","--interactive"}); Execute →
       nil; read the written file; assert it EQUALS config.GenerateBootstrapConfig("claude") (the
       detected default). (If claude isn't the detected default in the test env, use --provider claude.)
    I. TestInteractive_ProviderPreSelect (EXECUTE): --interactive --provider gemini + SetIn role answers
       → file is the gemini config with edits; provider prompt was NOT shown (buffer has no "Pick a
       provider").
    J. TestInteractive_Force_Overwrites (EXECUTE): pre-create the config; --interactive --force + accept
       → overwrites (no "already exists" error).
    K. TestInteractive_Template_Conflict (EXECUTE): --interactive --template → exit 1; err contains
       "choose one".
    L. TestInteractive_NothingDetected_Exits1 (EXECUTE): clean PATH (no agents) + forced TTY → exit 1;
       err points at plain config init.
    M. TestPlainConfigInit_Unchanged (REGRESSION): plain "config init" (no --interactive) output is
       byte-identical to pre-refactor (assert GenerateBootstrapConfig("pi") matches the existing
       TestConfigInit_ProviderPin_ExactOutput expectations — re-run that test mentally; it must still pass).
  - FOLLOW pattern: config_test.go (setupNoRepo, saveRootState/restoreRootState/resetFlags, Execute,
    exitcode.For, writeConfigFile). For PURE wizard tests, call runInteractiveWizard directly with
    strings.NewReader + bytes.Buffer (no Execute, no root state juggling).
  - NAMING: TestWizard_<Scenario> (pure), TestInteractive_<Scenario> (Execute). COVERAGE: golden
    byte-identity, single-backend edits, multi-backend re-prompt + blank-accept, invalid-provider
    re-prompt, EOF error, non-TTY gate, Execute happy-path file-write, --provider pre-select, --force,
    --template conflict, nothing-detected, plain-init regression.
  - PLACEMENT: internal/cmd/config_init_interactive_test.go (same package cmd).
  - DEPENDENCIES: Tasks 1-4.
```

### Implementation Patterns & Key Details

```go
// === bootstrap.go: the overrides-aware generator (nil overrides = byte-identity) ===
func GenerateBootstrapConfig(prov string) string {
	return GenerateBootstrapConfigWithOverrides(prov, nil)
}

func GenerateBootstrapConfigWithOverrides(prov string, overrides map[string]string) string {
	reg := provider.NewRegistry(nil)
	installed := bootstrapProviderNames(reg)
	target := prov
	if target == "" {
		if det := reg.DefaultProvider(installed); det != "" {
			target = det
		} else {
			target = "pi"
		}
	}
	return buildBootstrapConfig(target, installed, overrides)
}

// Inside buildBootstrapConfig, AFTER `models := DefaultModelsForProvider(target)` / piBlanked /
// stagerFallback / `if piBlanked { stagerModel = "" }`:
func applyOverrides(models map[string]string, stagerModel *string, overrides map[string]string) {
	if overrides == nil {
		return
	}
	for _, role := range []string{"planner", "message", "arbiter"} {
		if v, ok := overrides[role]; ok {
			models[role] = v
		}
	}
	if v, ok := overrides["stager"]; ok {
		*stagerModel = v
	}
}
// pi note branch:
//   if piBlanked && len(overrides) == 0 { <existing "empty, supply your own" note> }
//   else if piBlanked { <format-focused note: prefix required, bare is an error (FR-R5b)> }

// === config_init_interactive.go: the TTY gate (overridable) + pure wizard ===
var interactiveStdinIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }

func needsInferencePrefix(name string, m provider.Manifest) bool {
	pf := ""
	if m.ProviderFlag != nil {
		pf = *m.ProviderFlag
	}
	return name == "pi" || name == "opencode" || pf != ""
}

func runConfigInitInteractive(cmd *cobra.Command, _ []string) error {
	if tmpl, _ := cmd.Flags().GetBool("template"); tmpl {
		return exitcode.New(exitcode.Error, fmt.Errorf("--interactive writes a populated config; --template writes the inert reference — choose one"))
	}
	if !interactiveStdinIsTTY() {
		return exitcode.New(exitcode.Error, fmt.Errorf("--interactive requires a terminal on stdin; run plain 'stagehand config init' instead (it stays non-interactive for post-install scripts, FR-B3)"))
	}
	reg, err := newRegistry()
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: %w", err))
	}
	installed := installedNames(reg)
	if len(installed) == 0 {
		return exitcode.New(exitcode.Error, fmt.Errorf("no providers detected on $PATH; run plain 'stagehand config init' to default to pi, or install one of: %s", strings.Join(reg.PreferredBuiltins(), ", ")))
	}
	pinName, _ := cmd.Flags().GetString("provider")
	if pinName != "" {
		if _, ok := reg.Get(pinName); !ok {
			return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q (use a built-in: %s)", pinName, strings.Join(reg.PreferredBuiltins(), ", ")))
		}
	}
	defaultName := resolvedDefault(Config(), reg, installed)
	res, err := runInteractiveWizard(cmd.InOrStdin(), cmd.OutOrStdout(), reg, installed, defaultName, pinName)
	if err != nil {
		return exitcode.New(exitcode.Error, err)
	}
	content := config.GenerateBootstrapConfigWithOverrides(res.provider, res.overrides)
	path := config.ResolveConfigPath(flagConfig)
	force, _ := cmd.Flags().GetBool("force")
	if err := writeBootstrapFile(cmd, path, content, force); err != nil {
		return exitcode.New(exitcode.Error, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Wrote config to %s\n", path)
	return nil
}

// runInteractiveWizard is PURE: reads answers from r, writes prompts to w. No os.Stdin / IsTerminal.
func runInteractiveWizard(r io.Reader, w io.Writer, reg *provider.Registry, installed []string, defaultName, pinName string) (wizardResult, error) {
	br := bufio.NewReader(r)
	readLine := func() (string, error) {
		line, err := br.ReadString('\n')
		if err != nil && line == "" {
			return "", io.EOF // truncated script → caller errors loudly
		}
		return strings.TrimSpace(strings.TrimRight(line, "\r\n")), nil
	}
	chosen := pinName
	if chosen == "" {
		// list detected in FR-D1 order, highlight default; loop until valid/EOF
		chosen = /* ...prompt + validate... */ ""
	}
	m, _ := reg.Get(chosen)
	multi := needsInferencePrefix(chosen, m)
	overrides := map[string]string{}
	for _, role := range []string{"planner", "stager", "message", "arbiter"} {
		// display default: "" for pi (piBlanked) else DefaultModelsForProvider(chosen)[role]
		// prompt; readLine; "" → accept (no override); non-empty → validate (multi needs "/") or re-prompt
		_ = multi
	}
	if len(overrides) == 0 {
		overrides = nil // ⇒ byte-identical to GenerateBootstrapConfig(chosen)
	}
	return wizardResult{provider: chosen, overrides: overrides}, nil
}
```

### Integration Points

```yaml
COMMAND FLAG (config.go init()):
  - add: "configInitCmd.Flags().Bool(\"interactive\", false, <wizard text>) — local flag on config init"
  - branch: "at the TOP of runConfigInit: if interactive { return runConfigInitInteractive(cmd, args) }"

CONFIG GENERATOR (bootstrap.go):
  - add: "GenerateBootstrapConfigWithOverrides(prov, overrides) — the seam the wizard calls"
  - refactor: "GenerateBootstrapConfig(prov) delegates to ...WithOverrides(prov, nil) — byte-identical"
  - edit: "buildBootstrapConfig gains a 3rd `overrides` param; applies after pi-blank/stagerFallback"

REGISTRY (registry.go):
  - add: "PreferredBuiltins() []string — the FR-D1 order for the wizard's menu (NOT a schema change)"

FILE-WRITE (config.go, shared):
  - extract: "writeBootstrapFile(cmd, path, content, force) error — force-check + MkdirAll + WriteFile;
              used by BOTH runConfigInit and runConfigInitInteractive (DRY)"

TTY GATE (config_init_interactive.go):
  - var: "interactiveStdinIsTTY = func() bool { return ui.IsTerminal(os.Stdin) } — overridable for tests"

DOWNSTREAM (NOT this task):
  - "P1.M6.T1.S2 (models command) is orthogonal; the wizard MAY optionally mention `stagehand models`
    but FR-L3 does not require it — keep the wizard self-contained."
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After each file edit — fix before proceeding.
gofmt -w internal/cmd/config.go internal/cmd/config_init_interactive.go internal/cmd/config_init_interactive_test.go \
  internal/config/bootstrap.go internal/provider/registry.go
go vet ./internal/cmd/... ./internal/config/... ./internal/provider/...
golangci-lint run ./internal/cmd/... ./internal/config/... ./internal/provider/...

# Expected: zero errors. gofmt -l prints nothing for edited files.
gofmt -l internal/cmd/ internal/config/ internal/provider/
```

### Level 2: Unit Tests (Component Validation)

```bash
# The wizard tests (Task 7): pure golden byte-identity + edits + re-prompts, gate, composition.
go test ./internal/cmd/ -run 'TestWizard|TestInteractive|TestPlainConfigInit' -v

# The bootstrap refactor (no regression; overrides nil = identity).
go test ./internal/config/ -run 'Bootstrap|GenerateBootstrap' -v

# The PreferredBuiltins accessor.
go test ./internal/provider/ -run 'PreferredBuiltins|DefaultProvider' -v

# Full cmd + config + provider packages (no regression).
go test ./internal/cmd/... ./internal/config/... ./internal/provider/... -v

# Expected: all pass. The keystone test TestWizard_AcceptDefaults_ByteIdentical MUST pass — if it fails,
# the refactor broke byte-identity (overrides nil must equal GenerateBootstrapConfig). Fix the generator,
# NOT the test.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary.
go build -o /tmp/stagehand ./cmd/stagehand

# Non-TTY gate (stdin is /dev/null → not a TTY → exit 1, points at plain config init).
/tmp/stagehand config init --interactive </dev/null
# Expected: exit 1; message contains "terminal" and "config init".

# Happy path with piped answers (accept-all; put a fake claude on PATH so it is detected).
mkdir -p /tmp/w-bin && printf '#!/bin/sh\nexit 0\n' >/tmp/w-bin/claude && chmod +x /tmp/w-bin/claude
printf '\n\n\n\n\n' | PATH=/tmp/w-bin:$PATH /tmp/stagehand config init --interactive
# Expected: "Wrote config to <path>"; the file EQUALS `stagehand config init --provider claude` output.

# Verify byte-identity: write plain init to a second path and diff.
PATH=/tmp/w-bin:$PATH /tmp/stagehand config init --provider claude --config /tmp/plain.toml --force
diff <(cat ~/.config/stagehand/config.toml) /tmp/plain.toml && echo "BYTE-IDENTICAL"

# Multi-backend prefix prompting (pi): a bare model is re-prompted; a prefixed one is accepted.
printf 'pi\ngpt-5.4\nzai/gpt-5.4\nzai/gpt-5.4-mini\nzai/gpt-5.4-nano\nzai/gpt-5.4-mini\n' \
  | PATH=/tmp/w-bin:$PATH /tmp/stagehand config init --interactive --force
# Expected: the re-prompt for the bare "gpt-5.4" appears; the written pi config has zai/* prefixed models.

# --provider pre-select (skip the provider prompt).
printf '\n\n\n\n' | PATH=/tmp/w-bin:$PATH /tmp/stagehand config init --interactive --provider claude --force
# Expected: no "Pick a provider" prompt; claude config written.

# --interactive --template is a usage error.
PATH=/tmp/w-bin:$PATH /tmp/stagehand config init --interactive --template
# Expected: exit 1; message contains "choose one".

# Plain config init is unchanged (regression).
PATH=/tmp/w-bin:$PATH /tmp/stagehand config init --force
# Expected: identical to pre-refactor output (the detected default, e.g. claude or pi).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Golden byte-identity audit (the keystone contract): GenerateBootstrapConfigWithOverrides(p, nil) ==
# GenerateBootstrapConfig(p) for every built-in.
go test ./internal/config/ -run TestBootstrap_OverridesNil_IsIdentity -v
# (Add this focused test if not covered by TestWizard_AcceptDefaults_ByteIdentical.)

# FR-R5b hand-off: a config the wizard writes for pi must NEVER contain a bare non-empty model on pi
# (Render would hard-error). Grep the written pi config: every model is "" or contains "/".
grep -E 'model = "[^"/]*"' ~/.config/stagehand/config.toml && echo "FAIL: bare model present" || echo "OK: all pi models blank or prefixed"

# No net/http / no new deps: the wizard is stdlib-only (bufio, fmt, io, os, strings).
grep -rn "net/http" internal/cmd/config_init_interactive.go || echo "OK: no net/http"

# Coverage gate (Makefile target).
make coverage-gate   # ≥85% on internal/{git,provider,generate,config}; cmd should aim similar.

# Expected: byte-identity holds; no bare pi models; no net/http; coverage gate passes.
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed.
- [ ] `go build ./...` succeeds.
- [ ] `go test ./internal/cmd/... ./internal/config/... ./internal/provider/... -v` — all pass.
- [ ] `go vet ./...` clean.
- [ ] `golangci-lint run` clean.
- [ ] `gofmt -l internal/cmd/ internal/config/ internal/provider/` prints nothing.

### Feature Validation

- [ ] `--interactive` flag on `config init`; `runConfigInit` routes to the interactive path when set.
- [ ] Non-TTY stdin → exit 1 pointing at plain `config init`.
- [ ] Wizard lists DETECTED providers in FR-D1 order, highlights the detected default; nothing detected → exit 1.
- [ ] Per-role accept-or-edit over FR-D4 defaults; empty line = accept.
- [ ] Multi-backend (pi/opencode): bare edited model re-prompted; never written bare.
- [ ] Output is `GenerateBootstrapConfigWithOverrides(chosen, overrides)`; accept-all ⇒ byte-identical to
      `GenerateBootstrapConfig(chosen)` (TestWizard_AcceptDefaults_ByteIdentical passes).
- [ ] `--interactive --force` overwrites; `--interactive --provider <name>` pre-selects; `--interactive
      --template` is a usage error (exit 1).
- [ ] Plain `config init` output byte-unchanged (regression).
- [ ] `docs/cli.md` + `docs/configuration.md` document `--interactive`.

### Code Quality Validation

- [ ] Follows the config.go/providers.go patterns (exitcode.New; cmd.OutOrStdout; ResolveConfigPath).
- [ ] Reuses `newRegistry`/`installedNames`/`resolvedDefault` (same package) — no re-implementation.
- [ ] The PURE wizard takes `io.Reader`/`io.Writer` (testable with bytes; no os.Stdin inside).
- [ ] TTY gate is an overridable package var (the happy-path Execute test forces true + SetIn).
- [ ] Generator refactor is additive (nil overrides = identity); no duplication of the FR-D4 table.
- [ ] No `os.Exit`; no `net/http`; no new deps; no manifest-schema change (only `+PreferredBuiltins`).

### Documentation & Deployment

- [ ] `docs/cli.md` flag row + paragraph + example state the flow, composition, and non-TTY behavior.
- [ ] `docs/configuration.md` Bootstrap section has the interactive note.

---

## Anti-Patterns to Avoid

- ❌ Don't re-implement `newRegistry`/`installedNames`/`resolvedDefault` — they are unexported helpers in
  package `cmd` (providers.go); call them directly (same package). `newRegistry()` handles nil Config()
  (built-ins only) — exactly what config init needs.
- ❌ Don't break byte-identity. `GenerateBootstrapConfig(prov)` MUST stay byte-identical; implement it as
  `return GenerateBootstrapConfigWithOverrides(prov, nil)` and ensure nil overrides changes nothing. The
  golden test (TestWizard_AcceptDefaults_ByteIdentical) enforces this — fix the generator if it fails.
- ❌ Don't apply overrides BEFORE the pi-blank/stagerFallback — they must apply AFTER (stager routing is
  structural; overrides change only MODEL values). stager override → stagerModel, NOT stagerName.
- ❌ Don't emit the pi "models are empty" note when the wizard filled them — branch on
  `piBlanked && len(overrides)==0`; otherwise the file lies to the user.
- ❌ Don't hardcode `ui.IsTerminal(os.Stdin)` inline — make it the overridable `interactiveStdinIsTTY` var
  so the happy-path Execute test can force true while piping answers via `rootCmd.SetIn`.
- ❌ Don't read answers with `fmt.Scan` inside `runConfigInitInteractive` directly off `os.Stdin` — keep the
  wizard PURE (`runInteractiveWizard(r io.Reader, w io.Writer, …)`) and feed it `cmd.InOrStdin()`; that
  makes the contract tests (piped bytes) trivial.
- ❌ Don't forget opencode in `needsInferencePrefix` — Render only hard-errors pi (provider_flag), but
  opencode's correct form is STILL `backend/model`; re-prompt bare opencode models too.
- ❌ Don't return an EMPTY (non-nil) overrides map on accept-all — return NIL so
  `GenerateBootstrapConfigWithOverrides(chosen, nil)` is byte-identical (an empty map would also be fine
  for the generator, but nil is the clean "no edits" signal and matches the golden test exactly).
- ❌ Don't reuse cmd/config.go's STALE local `preferredBuiltins` (missing qwen-code) for menu order — use
  the new `provider.PreferredBuiltins()`.
- ❌ Don't display pi's raw `DefaultModelsForProvider` values (gpt-5.4) as the "default" — the bootstrap
  BLANKS them; show "" for pi roles (what actually gets written on accept).
- ❌ Don't touch the manifest schema, `providers/*.toml`, `builtin.go`, or `merge.go` — that is S1's
  (P1.M6.T1.S1) contract. The only provider-package edit is `+PreferredBuiltins()` (not a schema change).
- ❌ Don't add `init` to a non-`shouldSkipConfigLoad` path or call `config.Load` from the wizard — config
  init intentionally skips config load; `Config()` is nil; `newRegistry()` handles that.
- ❌ Don't use `net/http`, `golang.org/x/term`, or any new dependency — the wizard is stdlib-only
  (bufio/fmt/io/os/strings); the TTY check reuses the existing `ui.IsTerminal` heuristic.
