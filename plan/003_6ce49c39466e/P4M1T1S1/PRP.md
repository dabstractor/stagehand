---
name: "P4.M1.T1.S1 — Progress label '<Verb> with <model> in <provider>…' (PRD §9.13 FR51b) across the generate (single-commit) and decompose paths; --verbose enumerates all four decompose roles"
description: |

  EDIT `internal/ui/output.go` (add the pure `ProgressLabel` helper + shared `invocation`; refresh the
  `Progress` doc comment with the FR51b example — Mode A), EDIT `internal/ui/verbose.go` (add `RoleLine`
  + `VerboseRoles`), EDIT `internal/cmd/default_action.go` (route both progress call sites through
  `ProgressLabel`; generate path resolves the message-role provider via auto-detect; `runDecompose`
  captures `RoleModels` and surfaces the PLANNER role on the main line + emits `VerboseRoles`), and EDIT
  the three test files.

  CONTRACT (P4.M1.T1.S1, verbatim from the work item):
    1. RESEARCH NOTE: PRD §9.13 FR51b. ui.Progress (output.go) takes a free string and prefixes it with
       '↳ ' to stderr. The model string already carries the inference prefix (FR-R5b), so no special
       formatting is needed. Call sites: generate (message role) and decompose (planner role surfaced on
       the main line; --verbose prints all four roles).
    2. INPUT: P1.M2.T1.S2 (resolved provider/model/reasoning per role). The ui package (output.go
       Progress), the generate progress call site, the decompose progress call site(s).
    3. LOGIC: (a) Add a formatting helper (in ui or at the call site) that builds '<Verb> with <model>
       in <provider>…' — e.g. 'Generating with zai/glm-5.2 in pi…', 'Generating with sonnet in claude…',
       'Decomposing with anthropic/claude-sonnet-4 in opencode…'. When model is empty (the provider's own
       default), show '<provider>' alone. (b) Single-commit path (generate) surfaces the MESSAGE role's
       resolved config; verb 'Generating'. (c) Decompose surfaces the PLANNER role's resolved config on
       the main progress line (verb 'Decomposing'); --verbose prints all four roles
       (planner/stager/message/arbiter) with their resolved provider/model/reasoning.
    4. OUTPUT: progress lines name the resolved invocation; --verbose enumerates all four decompose roles.
    5. DOCS: [Mode A] ui.Progress doc comment with the FR51b example. Rides WITH the work.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/decompose/*.go — CONSUMED READ-ONLY. ResolveRoles already returns RoleModels (the
      P1.M2.T1.S2 v3 form: Planner/Stager/Message/Arbiter config.RoleConfig with resolved Provider/Model/
      Reasoning). The parallel sibling P3.M2.T1.S2 edits decompose.go's one-file short-circuit — it does
      NOT touch ui/* or the progress call sites (no file overlap; safe to consume its outputs).
    - internal/config/*.go — CONSUMED. config.RoleConfig{Provider,Model,Reasoning}; config.ResolveRoleModel.
    - internal/provider/{registry.go,manifest.go} — CONSUMED. NewRegistry, DecodeUserOverrides,
      DefaultProvider, List, IsInstalled, Get (mirror pkg/stagecoach.buildDeps for the generate autodetect).
    - pkg/stagecoach/stagecoach.go — CONSUMED (GenerateCommit; buildDeps is the autodetect reference).
    - All other files (git, generate, exitcode, signal, cmd/stagecoach/main.go, root.go) — UNCHANGED.
    - PRD.md, tasks.json, prd_snapshot.md, .gitignore — NEVER modify.

  DELIVERABLES (3 production EDITS + 3 test EDITS; 0 NEW files):
    EDIT internal/ui/output.go     — +ProgressLabel(verb,model,provider) + invocation(model,provider);
                                     refresh Progress doc comment (Mode A, FR51b example).
    EDIT internal/ui/verbose.go    — +type RoleLine +VerboseRoles([]RoleLine).
    EDIT internal/cmd/default_action.go — generate label via ProgressLabel (resolve provider w/ autodetect);
                                     runDecompose: capture RoleModels, VerboseRoles, planner main label.
    EDIT internal/ui/output_test.go    — +TestProgressLabel_* (FR51b byte-exact combos).
    EDIT internal/ui/verbose_test.go   — +TestVerboseRoles_* (on/off/nil + reasoning suffix).
    EDIT internal/cmd/default_action_test.go — + label-observable tests (generate + decompose, via errBuf).

  SUCCESS: `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `↳ Generating with sonnet in claude…`,
  `↳ Generating with zai/glm-5.2 in pi…`, `↳ Generating with claude…` (model empty), and
  `↳ Decomposing with anthropic/claude-sonnet-4 in opencode…` are produced by ProgressLabel;
  `VerboseRoles` emits four `DEBUG: <role> <invocation>(reasoning: …)?` lines (planner carries
  `(reasoning: high)` from the shipped default; others omit it) and is a no-op when verbose is off / nil;
  the generate path now names the resolved (auto-detected) provider even when `--provider` is unset;
  the decompose main line names the PLANNER role's resolved model+provider; existing tests stay green.

---

## Goal

**Feature Goal**: Implement PRD §9.13 **FR51b** — the `↳ <Verb>…` progress line (stderr) names the
resolved model invocation as `<Verb> with <model> in <provider>…` (or `<Verb> with <provider>…` when the
model is the provider's own default), and `--verbose` enumerates all four decompose roles' resolved
provider/model/reasoning. Concretely: (1) a pure `ui.ProgressLabel(verb, model, provider)` formatter shared
by both paths; (2) the generate (single-commit) path surfaces the MESSAGE role's resolved config (verb
`Generating`), resolving the provider via auto-detect so the label is correct even when `--provider` is
unset; (3) the decompose path surfaces the PLANNER role's resolved config on the main line (verb
`Decomposing`) and, under `--verbose`, prints all four roles (planner/stager/message/arbiter).

**Deliverable** (3 production EDITS + 3 test EDITS; 0 new files):
1. `internal/ui/output.go` — `ProgressLabel` + unexported `invocation`; refreshed `Progress` doc comment.
2. `internal/ui/verbose.go` — `RoleLine` + `VerboseRoles`.
3. `internal/cmd/default_action.go` — both progress call sites routed through `ProgressLabel`; generate
   path auto-detects the provider; `runDecompose` captures `RoleModels`, calls `VerboseRoles`, surfaces the
   planner on the main line.
4–6. The three test files.

**Success Definition**:
- **ProgressLabel (FR51b byte format)**: `ProgressLabel("Generating","sonnet","claude")` ==
  `"Generating with sonnet in claude…"`; `("Generating","zai/glm-5.2","pi")` ==
  `"Generating with zai/glm-5.2 in pi…"`; `("Generating","","claude")` == `"Generating with claude…"`
  (model empty → provider alone); `("Generating","sonnet","")` == `"Generating…"` (nothing resolved →
  minimal fallback); `("Decomposing","anthropic/claude-sonnet-4","opencode")` ==
  `"Decomposing with anthropic/claude-sonnet-4 in opencode…"`.
- **Generate path**: stderr contains `↳ Generating with <cfg.Model> in <resolved-provider>…` when a model
  is set, or `↳ Generating with <resolved-provider>…` when the model is empty, where `<resolved-provider>`
  is `cfg.Provider` or the auto-detected default when `--provider` is unset. (Previously the unset case
  printed only `↳ Generating…`.)
- **Decompose main line**: stderr contains `↳ Decomposing with <planner.Model> in <planner.Provider>…`
  (the PLANNER role's resolved config from `ResolveRoles`, including post-FR-D4 stager fallback does NOT
  affect the planner).
- **Decompose --verbose**: with `cfg.Verbose`, stderr additionally contains four `DEBUG:` lines — one per
  role — each `<invocation>` (model+provider) with a `(reasoning: high)` suffix on the planner (shipped
  default) and no suffix on stager/message/arbiter (reasoning "" ⇒ off). Without `--verbose` (or nil sink)
  `VerboseRoles` writes zero bytes.
- **No regressions**: existing `output_test.go` Progress tests (which pass exact strings to `Progress`)
  stay green; existing `default_action_test.go` errBuf assertions (`(no commit created)`, rescue/CAS) stay
  green (none assert on the old label format — verified).
- `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED.

## User Persona

**Target User**: the end user at the shell (the "plan-holder", §7.1) and the lazygit integrator (§15.5).

**Use Case**: the user runs `stagecoach` and, while generation runs, sees WHICH agent+model is actually
doing the work (`↳ Generating with zai/glm-5.2 in pi…`) instead of a generic `↳ Generating…`. For
decompose, the main line names the planner; `--verbose` shows the full four-role roster so a tinkerer
(§7.3) can confirm per-role routing before the run.

**User Journey**: (1) `stagecoach` → `↳ Generating with sonnet in claude…` on stderr; (2) commit lands on
stdout; (3) `stagecoach -v` (decompose) → `↳ Decomposing with … in ……` then four `DEBUG: role …` lines;
(4) the user adjusts `--planner-model` / `--stager-model` and re-runs to see the roster change.

**Pain Points Addressed**: the progress line was opaque (often just `Generating…` for the auto-detected
provider); decompose never named which model was planning vs staging. FR51b makes the resolved invocation
visible and, under `--verbose`, the whole four-role plan.

## Why

- **Business value**: FR51b is a P1 UX/transparency requirement (§9.13). It directly supports G7 (the user
  knows which agent is acting) and the multi-agent tinkerer persona (G12 per-role visibility). Small,
  self-contained, high signal-per-line.
- **Integration with existing features**: consumes P1.M2.T1.S2's resolved per-role `RoleModels`
  (`{Provider, Model, Reasoning}`) — which the decompose path already computes but currently DISCARDS —
  and the v3 model-prefix fold (FR-R5b: the model string already carries the inference backend, so the
  label needs no extra formatting). Reuses `ui.Progress` (unchanged signature) and the `Verbose` DEBUG
  convention.
- **Problems this solves and for whom**: removes the guesswork of "which model is this?" during a run, and
  gives the tinkerer a one-flag roster of the four decompose roles. The generate-path bonus: auto-detected
  providers are now NAMED in the label (the old code only named an explicit `--provider`).

## What

**User-visible behavior**:
- Single-commit (`stagecoach` with staged changes, or `--single`/`--all`/dry-run): stderr shows
  `↳ Generating with <model> in <provider>…` (model set) or `↳ Generating with <provider>…` (model =
  provider default). The provider is the resolved one (explicit or auto-detected).
- Decompose (`stagecoach` with nothing staged + dirty tree): stderr shows
  `↳ Decomposing with <planner-model> in <planner-provider>…`.
- `stagecoach -v` (decompose): after the main line, four `DEBUG:` lines enumerate planner/stager/message/
  arbiter with their resolved `<model> in <provider>` (and `(reasoning: high)` on the planner).
- Stdout is unchanged (still the commit report / dry-run message) — all progress/verbose goes to stderr
  (FR51), preserving pipe use (`stagecoach --dry-run --no-color | tee /tmp/msg.txt`).

**Technical requirements**: a pure `ui.ProgressLabel`; a shared unexported `ui.invocation`; a
`ui.VerboseRoles([]ui.RoleLine)` method; the generate path builds the registry + auto-detects (mirror
`buildDeps`); `runDecompose` keeps `RoleModels` and feeds the planner to the label + all four to
`VerboseRoles`. No new packages, no config/provider-schema changes, no go.mod changes.

### Success Criteria

- [ ] `ui.ProgressLabel` produces the FR51b byte format for all combos (model set / empty; provider set /
      empty; both verbs) — locked by `TestProgressLabel_*`.
- [ ] `ui.invocation` is the single source for `<model> in <provider>` / `<provider>` / `""` (shared by
      `ProgressLabel` and `VerboseRoles`).
- [ ] Generate path stderr contains the FR51b label with the RESOLVED provider (incl. auto-detect).
- [ ] Decompose main-line stderr contains the PLANNER role's resolved model+provider.
- [ ] `VerboseRoles` prints four `DEBUG:` lines under `--verbose` (planner has `(reasoning: high)`) and
      zero bytes when off / nil sink.
- [ ] `ui.Progress` doc comment updated to the FR51b example (Mode A).
- [ ] Existing `output_test.go`, `verbose_test.go`, `default_action_test.go` assertions stay green.
- [ ] `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
      `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed?_ — YES. Every consumed
symbol is named with its exact signature + file + line; the exact FR51b byte format (with all edge cases)
is specified; the exact `ProgressLabel`/`invocation`/`VerboseRoles` bodies are given; the exact insertion
points in `default_action.go` (generate validate+label block ~L128-148; `runDecompose` ~L271-300) are
given with the lines to replace; the autodetect mirror of `buildDeps` is quoted; the
`RoleModels.{Planner,Stager,Message,Arbiter}` field names + `config.RoleConfig{Provider,Model,Reasoning}`
are named; the test harness (`u.Progress` exact-string tests, errBuf assertions, `ui.NewVerbose(&buf,bool)`)
is described; and the non-obvious points (no existing test asserts on the old label format → safe; planner
reasoning defaults to "high" → suffix; main label has NO reasoning; auto-detect makes the unset-provider
case name the provider; reuse the one `verbose` sink for both `VerboseRoles` and `deps.Verbose`) are
explained, not just named.

### Documentation & References

```yaml
# MUST READ
- url: PRD.md §9.13 FR51b (Progress label shows the resolved model and provider)
  why: "Authoritative format: '<Verb> with <model> in <provider>…'; model empty ⇒ '<provider>' alone;
        single-commit role = message; decompose main line = planner; --verbose = all four roles."
  critical: "The model string already carries the inference prefix (FR-R5b: 'zai/glm-5.2',
        'anthropic/claude-sonnet-4') — NO special formatting/splitting. Reasoning is NOT in the main
        label (only in the --verbose enumeration)."

- url: PRD.md §15.5 (Example invocations) — the FR51b example strings
  why: "'Generating with sonnet in claude…', 'Generating with zai/glm-5.2 in pi…', 'Decomposing with
        anthropic/claude-sonnet-4 in opencode…'. Use these verbatim in tests + the Progress doc comment."

# CODEBASE FILES — pattern sources + consumed dependencies (all verified, paths exact)
- file: internal/ui/output.go   # EDIT TARGET — ProgressLabel + invocation + Progress doc comment
  why: "Progress(msg) writes '↳ '+msg+'\\n' to stderr (L103). progressPrefix='↳ '. The label formatting
        currently lives at the CALL SITE (default_action.go), not here — FR51b centralizes it. New
        ProgressLabel is PURE (no I/O) so it is trivially unit-testable (like ResolveColor)."
  pattern: "Add an unexported func invocation(model, provider string) string returning '' (provider=='')
        | '<model> in <provider>' (model!='') | '<provider>'. Add ProgressLabel(verb, model, provider
        string) string = verb+'…' if invocation=='' else verb+' with '+invocation+'…'. Refresh the
        Progress doc comment's example to 'Progress(ui.ProgressLabel(\"Generating\",\"sonnet\",\"claude\"))'."
  gotcha: "Progress itself is UNCHANGED (signature + body). Only its DOC COMMENT changes (Mode A). Do not
        colorize ProgressLabel output (Progress is plain by design — FR51 stdout-clean)."

- file: internal/ui/verbose.go   # EDIT TARGET — RoleLine + VerboseRoles
  why: "Verbose{w io.Writer; on bool}; methods VerboseCommand/VerboseRawOutput/VerboseRetry all guard
        'if v==nil || v.w==nil || !v.on { return }' then Fprintln/Fprintf 'DEBUG: …' to v.w. NewVerbose
        (w, on). VerboseRoles mirrors this idiom."
  pattern: "type RoleLine struct { Name, Model, Provider, Reasoning string }. func (v *Verbose)
        VerboseRoles(roles []RoleLine) — guard; for each role: fmt.Fprintf(v.w, 'DEBUG: %-8s %s%s\\n',
        r.Name, invocation(r.Model, r.Provider), reasoningSuffix(r.Reasoning)). reasoningSuffix(level)
        returns ' (reasoning: '+level+')' when level∈{low,medium,high} else ''. invocation is the SAME
        unexported helper from output.go (same package)."
  gotcha: "ui is ONE package (output.go + verbose.go share unexported symbols) — invocation defined once
        in output.go is visible in verbose.go. The %-8s pads role names (planner/stager/message/arbiter =
        7/6/7/7 chars) so invocations align. reasoning '' or 'off' ⇒ no suffix (the shipped default for
        stager/message/arbiter is '' ⇒ off; planner is 'high')."

- file: internal/cmd/default_action.go   # EDIT TARGET — both call sites
  why: "Two Progress call sites. GENERATE (~L128-148): a block that validates cfg.Provider then builds the
        label inline ('Generating' + ' with '+cfg.Provider + ' ('+cfg.Model+')' + '…'); uses cfg.Provider
        DIRECTLY (no autodetect ⇒ 'Generating…' when --provider unset). DECOMPOSE (runDecompose ~L271-300):
        builds reg, calls ResolveRoles and DISCARDS RoleModels (_), label = 'Decomposing'+' with
        '+cfg.Provider+'…' (no model), deps.Verbose = ui.NewVerbose(stderr,cfg.Verbose) inline."
  pattern: "GENERATE: build reg once (handle DecodeUserOverrides error — old code used _); compute
        labelProvider = cfg.Provider or reg.DefaultProvider(installed) (mirror buildDeps' installed loop);
        keep the explicit-provider reg.Get validation; u.Progress(ui.ProgressLabel('Generating',
        cfg.Model, labelProvider)). DECOMPOSE: roleManifests, roleModels, err := ResolveRoles(*cfg, reg);
        verbose := ui.NewVerbose(stderr, cfg.Verbose); verbose.VerboseRoles([...4 RoleLine from roleModels...]);
        u.Progress(ui.ProgressLabel('Decomposing', roleModels.Planner.Model, roleModels.Planner.Provider));
        deps.Verbose = verbose (reuse)."
  gotcha: "default_action.go ALREADY imports ui, provider, decompose, config — no new imports. cfg is
        *config.Config; ResolveRoles takes a config.Config VALUE → *cfg. Preserve the existing explicit-
        provider validation (reg.Get(cfg.Provider) only when cfg.Provider != '') — autodetect is validated
        inside GenerateCommit; do NOT add an IsInstalled gate at the CLI for the generate path (buildDeps
        owns pre-flight). Order in runDecompose: main label FIRST, then verbose roles."

- file: internal/decompose/roles.go   # CONSUMED — ResolveRoles + RoleModels
  why: "ResolveRoles(cfg config.Config, reg *provider.Registry) (RoleManifests, RoleModels, error).
        RoleModels{Planner, Stager, Message, Arbiter config.RoleConfig}. Each is RESOLVED (post-auto-detect,
        post-FR-D4 stager fallback; reasoning resolved per-field→global→shipped default). The decompose
        path currently discards RoleModels with '_' — FR51b keeps it."
  pattern: "roleModels.Planner.{Model,Provider,Reasoning} → main label + planner verbose line;
        roleModels.{Stager,Message,Arbiter} → the other three verbose lines."
  gotcha: "The FR-D4 stager fallback can change the STAGER's provider+model — that is correctly reflected
        in roleModels.Stager (ResolveRoles switches both). It does NOT affect the planner. So the main
        decompose label (planner) is stable regardless of stager fallback."

- file: internal/config/roles.go + config.go   # CONSUMED — ResolveRoleModel + RoleConfig
  why: "config.ResolveRoleModel(role, cfg) (provider, model, reasoning string) — planner reasoning
        defaults to 'high' (defaultRoleReasoning); stager/message/arbiter to '' (off). config.RoleConfig =
        {Provider, Model, Reasoning string}. So roleModels.X.Reasoning is the RESOLVED level."
  pattern: "No edit. Reasoning drives the verbose suffix only (low/medium/high ⇒ '(reasoning: <level>)')."

- file: pkg/stagecoach/stagecoach.go   # CONSUMED (reference) — buildDeps autodetect
  why: "buildDeps computes `installed` via reg.List()+IsInstalled, then reg.DefaultProvider(installed) when
        cfg.Provider=='' — the exact logic the generate-path label must mirror so the label names the
        auto-detected provider."
  pattern: "Copy the 5-line installed loop + DefaultProvider call into the generate validate+label block."

- file: internal/provider/registry.go   # CONSUMED — DefaultProvider, List, IsInstalled, Get, NewRegistry
  why: "DefaultProvider(installed []string) string returns the highest-priority INSTALLED built-in;
        List() []Manifest; IsInstalled(m); Get(name); NewRegistry(overrides)."

# TEST FILES — patterns
- file: internal/ui/output_test.go   # EDIT — +TestProgressLabel_*
  why: "TestProgress_PrefixAndStream asserts exact '↳ …\\n' bytes via a *bytes.Buffer stderr. Mirror for
        ProgressLabel (pure function: assert the returned string with %q)."
  pattern: "Table-driven: {verb, model, provider, want}. Rows: ('Generating','sonnet','claude')→'Generating
        with sonnet in claude…'; ('Generating','zai/glm-5.2','pi')→'… with zai/glm-5.2 in pi…';
        ('Generating','','claude')→'Generating with claude…'; ('Generating','sonnet','')→'Generating…';
        ('Decomposing','anthropic/claude-sonnet-4','opencode')→'Decomposing with … in opencode…'."
  gotcha: "ProgressLabel is PURE (returns a string) — do NOT route it through Progress in the unit test;
        assert the string directly. (Progress routing is tested at the cmd level via errBuf.)"

- file: internal/ui/verbose_test.go   # EDIT — +TestVerboseRoles_*
  why: "TestVerbose_CommandWhenOn asserts 'DEBUG: command: …\\n' exact bytes from a *bytes.Buffer. Mirror
        for VerboseRoles."
  pattern: "on: 4 RoleLine → assert 4 lines, planner line ends '(reasoning: high)', others have no suffix,
        invocations are '<model> in <provider>'. off: v=NewVerbose(&buf,false) → buf empty. nil:
        var v *Verbose; v.VerboseRoles(…) → no panic."

- file: internal/cmd/default_action_test.go   # EDIT — +label-observable tests
  why: "Tests use rootCmd.SetOut/SetErr(&buf)+SetArgs+Execute; setupStubRepo builds a stub provider.
        errBuf captures stderr (where Progress writes)."
  pattern: "Generate: stage a change, SetArgs(['--provider','stub','--model','glm-5.2']), Execute; assert
        errBuf.Contains('↳ Generating with glm-5.2 in stub…'). Decompose: dirty/un-staged tree, the
        decompose path needs ResolveRoles (stub has no tooled_flags → stager fallback fails) — so assert
        the MAIN LABEL appears in errBuf BEFORE the ResolveRoles error path: errBuf.Contains('↳ Decomposing
        with '). (Print the label before calling Decompose.) --verbose decompose: SetArgs(['-v']) and assert
        errBuf.Contains('DEBUG: planner') etc. NOTE: a full happy-path decompose Execute is infeasible (bare
        stub can't run the tooled stager) — assert the label + verbose lines are EMITTED (they print before
        Decompose runs); do not assert commit creation."
  gotcha: "resetFlags between Executes (existing restoreRootState does this). The decompose verbose test
        prints roles right after the main label, BEFORE Decompose — so even though Decompose then errors
        (no tooled stager), the label+verbose lines are already in errBuf. Assert on those."
```

### Current Codebase tree (relevant subset)

```bash
internal/ui/
  output.go        # EDIT — +ProgressLabel +invocation; refresh Progress doc comment
  output_test.go   # EDIT — +TestProgressLabel_*
  verbose.go       # EDIT — +RoleLine +VerboseRoles
  verbose_test.go  # EDIT — +TestVerboseRoles_*
internal/cmd/
  default_action.go        # EDIT — generate label (autodetect) + runDecompose (RoleModels + VerboseRoles + planner label)
  default_action_test.go   # EDIT — +label-observable tests (generate + decompose + -v)
internal/decompose/roles.go # CONSUMED — ResolveRoles, RoleModels (P1.M2.T1.S2 v3 form)
internal/config/{config.go,roles.go} # CONSUMED — RoleConfig, ResolveRoleModel
internal/provider/registry.go        # CONSUMED — DefaultProvider/List/IsInstalled/Get/NewRegistry
pkg/stagecoach/stagecoach.go           # CONSUMED (reference) — buildDeps autodetect
```

### Desired Codebase tree (files this task EDITS — no new files)

```bash
internal/ui/output.go            # +func invocation(model,provider) +func ProgressLabel(verb,model,provider)
                                 #  + Progress doc comment → FR51b example
internal/ui/verbose.go           # +type RoleLine +func (v *Verbose) VerboseRoles([]RoleLine)
internal/cmd/default_action.go   # generate: reg-once + autodetect + ProgressLabel; runDecompose: keep
                                 #  RoleModels + VerboseRoles + planner ProgressLabel + reuse verbose sink
internal/ui/output_test.go       # +TestProgressLabel_* (FR51b byte-exact table)
internal/ui/verbose_test.go      # +TestVerboseRoles_* (on/off/nil + reasoning suffix)
internal/cmd/default_action_test.go # + label-observable (generate) + label+verbose (decompose/-v)
```

### Known Gotchas of our codebase & Library Quirks

```go
// G-ONE-PACKAGE: ui is ONE package — output.go and verbose.go share unexported symbols. Define
//   invocation() ONCE (output.go); verbose.go's VerboseRoles calls it directly. Do not duplicate it.

// G-NO-OLD-LABEL-ASSERTIONS (verified safe): NO existing test asserts on the generate-path label text
//   ('Generating with …'). output_test.go passes EXACT strings to Progress (unaffected by the helper);
//   default_action_test.go errBuf assertions are '(no commit created)' + rescue/CAS only. Changing the
//   label format from 'Generating with stub (m)…' to 'Generating with m in stub…' breaks nothing.

// G-AUTODETECT-MIRROR: the generate path must resolve the provider the SAME way buildDeps does
//   (reg.List()+IsInstalled → reg.DefaultProvider(installed)) so the label names the auto-detected
//   provider when --provider is unset. The OLD code used cfg.Provider directly → 'Generating…' (no
//   provider) in the unset case. Do NOT add an IsInstalled pre-flight gate at the CLI (buildDeps owns it);
//   just compute the name for the label.

// G-DECODE-OVERRIDES-ERROR: the old generate block ignored DecodeUserOverrides's error ('overrides, _ :=').
//   Handle it (return exitcode.New(Error, …)) — consistent with runDecompose and a real fail-fast improvement.

// G-MODEL-CARRIES-INFERENCE (FR-R5b): the model string already encodes the inference backend as a slash
//   prefix ('zai/glm-5.2', 'anthropic/claude-sonnet-4') or is bare for single-backend ('sonnet'). The
//   label prints it VERBATIM — do NOT split on '/' or try to extract a sub-provider. The contract is
//   explicit: 'no special formatting is needed'.

// G-REASONING-MAIN-LABEL-NONE: FR51b's main label is '<model> in <provider>' ONLY (PRD examples have no
//   reasoning). Reasoning appears ONLY in the --verbose 4-role enumeration. Do not add reasoning to
//   ProgressLabel's output.

// G-PLANNER-REASONING-HIGH: config.ResolveRoleModel resolves planner reasoning to 'high' by default
//   (defaultRoleReasoning); stager/message/arbiter to '' (off). So the planner verbose line carries
//   '(reasoning: high)' automatically once you read roleModels.Planner.Reasoning. reasoningSuffix must
//   emit the suffix for low/medium/high and NOTHING for ''/'off'.

// G-VERBOSE-SINK-REUSE: runDecompose currently builds deps.Verbose = ui.NewVerbose(stderr, cfg.Verbose)
//   inline. Extract it to a local `verbose`, use it for VerboseRoles, AND pass it as deps.Verbose — one
//   sink, so the role roster + the loop's DEBUG: command/raw/retry share the same destination + on/off.

// G-PROGRESS-TO-STDERR: Progress writes to stderr (FR51 — stdout stays clean for piping). The label is
//   observed in errBuf, NOT outBuf. VerboseRoles also writes to stderr (the verbose sink = stderr).

// G-RESOLVEROLES-VALUE: ResolveRoles(*cfg, reg) takes a config.Config VALUE (cfg is *config.Config here).
//   The decompose path already does this; just stop discarding the 2nd return (RoleModels).

// G-FR-D4-STAGER-FALLBACK: ResolveRoles may switch the STAGER provider+model (tooled_flags fallback).
//   That is reflected in roleModels.Stager (correct for the verbose line). It does NOT touch the planner,
//   so the main decompose label is stable. Do not re-derive the planner.
```

## Implementation Blueprint

### Data models and structure

No new data models in the domain sense. Two small UI-side additions:

```go
// internal/ui/output.go

// invocation renders the FR51b "<model> in <provider>" core (shared by ProgressLabel + VerboseRoles).
// provider=="" ⇒ "" (nothing resolved — e.g. no provider installed); model=="" ⇒ "<provider>" alone
// (the provider's own default); else "<model> in <provider>". The model string already carries the
// inference backend where relevant (FR-R5b), so it is printed VERBATIM.
func invocation(model, provider string) string

// ProgressLabel builds the FR51b progress-line body (without the "↳ " prefix — Progress adds that):
// "<verb>…" when nothing is resolved (provider==""), else "<verb> with <invocation>…". Pure (no I/O).
func ProgressLabel(verb, model, provider string) string
```

```go
// internal/ui/verbose.go

// RoleLine is one role's resolved (provider, model, reasoning) for the --verbose four-role roster
// (PRD §9.13 FR51b). The caller maps config.RoleConfig → RoleLine at the decompose call site.
type RoleLine struct {
	Name      string // "planner" | "stager" | "message" | "arbiter"
	Model     string
	Provider  string
	Reasoning string // off|low|medium|high; "" ⇒ off (no suffix)
}

// VerboseRoles prints the four-role roster (one "DEBUG:" line each) when verbose is on. No-op when
// v==nil, v.w==nil, or !v.on (same guard idiom as VerboseCommand/VerboseRawOutput/VerboseRetry).
func (v *Verbose) VerboseRoles(roles []RoleLine)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/ui/output.go — invocation + ProgressLabel + Progress doc comment
  - ADD func invocation(model, provider string) string:
        if provider == "" { return "" }
        if model != "" { return model + " in " + provider }
        return provider
  - ADD func ProgressLabel(verb, model, provider string) string:
        inv := invocation(model, provider)
        if inv == "" { return verb + "…" }
        return verb + " with " + inv + "…"
  - UPDATE the Progress doc comment's example (Mode A, FR51b): replace the old
    'Progress("Generating with pi (glm-5.2)…")' example with a note that callers build the body via
    ProgressLabel, e.g. 'u.Progress(ui.ProgressLabel("Generating", "sonnet", "claude")) →
    "↳ Generating with sonnet in claude…\n"'. Keep Progress's signature + body UNCHANGED.
  - FOLLOW pattern: ResolveColor (a pure helper in the same file) for the no-I/O style + doc comment.
  - NAMING: invocation (unexported, shared), ProgressLabel (exported).
  - PLACEMENT: internal/ui/output.go (near Progress).

Task 2: EDIT internal/ui/verbose.go — RoleLine + VerboseRoles
  - ADD type RoleLine struct { Name, Model, Provider, Reasoning string } (doc comment per Data models).
  - ADD func (v *Verbose) VerboseRoles(roles []RoleLine):
        if v == nil || v.w == nil || !v.on { return }
        for _, r := range roles {
            fmt.Fprintf(v.w, "DEBUG: %-8s %s%s\n", r.Name, invocation(r.Model, r.Provider),
                reasoningSuffix(r.Reasoning))
        }
  - ADD func reasoningSuffix(level string) string:
        switch level { case "low","medium","high": return " (reasoning: " + level + ")" }
        return ""   // "" or "off" or unknown ⇒ no suffix
  - FOLLOW pattern: VerboseCommand/VerboseRawOutput/VerboseRetry (guard + DEBUG: prefix + Fprintf).
  - GOTCHA: invocation is defined in output.go (same package) — call it directly; do not redefine.
    The %-8s pads role names so invocations align (planner/stager/message/arbiter are 7/6/7/7 chars).
  - NAMING: RoleLine (exported — it is a parameter type), VerboseRoles, reasoningSuffix (unexported).
  - PLACEMENT: internal/ui/verbose.go (after VerboseRetry).

Task 3: EDIT internal/cmd/default_action.go — generate label (autodetect) via ProgressLabel
  - REPLACE the generate-path validate+label block (the `if cfg.Provider != "" { overrides,_:=…; reg:=…;
    reg.Get }` + the inline `label := "Generating" …` + `u.Progress(label)`) with:
        overrides, oerr := provider.DecodeUserOverrides(cfg.Providers)
        if oerr != nil {
            return exitcode.New(exitcode.Error, fmt.Errorf("provider overrides: %w", oerr))
        }
        reg := provider.NewRegistry(overrides)
        // FR51b: resolve the message role's provider (auto-detect mirrors pkg/stagecoach.buildDeps) so
        // the label names the resolved invocation even when --provider is unset.
        labelProvider := cfg.Provider
        if labelProvider == "" {
            var installed []string
            for _, m := range reg.List() {
                if reg.IsInstalled(m) {
                    installed = append(installed, m.Name)
                }
            }
            labelProvider = reg.DefaultProvider(installed)
        }
        // Validate an EXPLICIT provider (autodetect is validated inside GenerateCommit/buildDeps).
        if cfg.Provider != "" {
            if _, ok := reg.Get(cfg.Provider); !ok {
                return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q", cfg.Provider))
            }
        }
        u.Progress(ui.ProgressLabel("Generating", cfg.Model, labelProvider))
  - PRESERVE: the subsequent stagecoach.GenerateCommit call + Options EXACTLY as-is.
  - GOTCHA: do NOT add reg.IsInstalled at the CLI (buildDeps owns pre-flight). The only behavioral change
    vs the old block: (a) the label format (FR51b), (b) autodetect names the provider, (c) the
    DecodeUserOverrides error is now surfaced (was ignored).

Task 4: EDIT internal/cmd/default_action.go — runDecompose: RoleModels + VerboseRoles + planner label
  - CHANGE `roleManifests, _, err := decompose.ResolveRoles(*cfg, reg)` → keep RoleModels:
        roleManifests, roleModels, err := decompose.ResolveRoles(*cfg, reg)
  - EXTRACT the verbose sink + emit the roster + planner label BEFORE building deps:
        verbose := ui.NewVerbose(stderr, cfg.Verbose)
        verbose.VerboseRoles([]ui.RoleLine{
            {Name: "planner", Model: roleModels.Planner.Model, Provider: roleModels.Planner.Provider, Reasoning: roleModels.Planner.Reasoning},
            {Name: "stager",  Model: roleModels.Stager.Model,  Provider: roleModels.Stager.Provider,  Reasoning: roleModels.Stager.Reasoning},
            {Name: "message", Model: roleModels.Message.Model, Provider: roleModels.Message.Provider, Reasoning: roleModels.Message.Reasoning},
            {Name: "arbiter", Model: roleModels.Arbiter.Model, Provider: roleModels.Arbiter.Provider, Reasoning: roleModels.Arbiter.Reasoning},
        })
        // FR51b: main line surfaces the PLANNER role's resolved invocation.
        u.Progress(ui.ProgressLabel("Decomposing", roleModels.Planner.Model, roleModels.Planner.Provider))
  - SET deps.Verbose = verbose (reuse the same sink; replace the inline `ui.NewVerbose(stderr, cfg.Verbose)`).
  - PRESERVE: the rest of runDecompose (deps literal fields, decompose.Decompose call, printDecomposeCommit
    loop, handleDecomposeError). Keep deps.Out = stderr, deps.Roles = roleManifests, deps.Config = *cfg.
  - ORDER: main label FIRST, then verbose roles is also acceptable; pick main-label-then-verbose and
    document it. (Either order is correct; be consistent in the test assertions.)
  - GOTCHA: ResolveRoles may have already been called — no, runDecompose calls it once; just stop
    discarding the 2nd return. The FR-D4 stager fallback is already in roleModels.Stager (correct).

Task 5: EDIT internal/ui/output_test.go — TestProgressLabel_*
  - ADD a table-driven TestProgressLabel (PURE — assert the returned string):
        {"Generating","sonnet","claude"} → "Generating with sonnet in claude…"
        {"Generating","zai/glm-5.2","pi"} → "Generating with zai/glm-5.2 in pi…"
        {"Generating","","claude"}        → "Generating with claude…"
        {"Generating","sonnet",""}        → "Generating…"
        {"Decomposing","anthropic/claude-sonnet-4","opencode"} → "Decomposing with anthropic/claude-sonnet-4 in opencode…"
  - FOLLOW pattern: TestResolveColor_Logic (table-driven %q assertions on a pure function).
  - PLACEMENT: internal/ui/output_test.go.

Task 6: EDIT internal/ui/verbose_test.go — TestVerboseRoles_*
  - ADD TestVerboseRoles_On: NewVerbose(&buf,true); VerboseRoles(4 RoleLine: planner model "p" provider
    "pi" reasoning "high"; stager/provider; message; arbiter — reasoning "" ); assert buf has 4 lines,
    the planner line is "DEBUG: planner  p in pi (reasoning: high)\n", the stager line has NO suffix.
  - ADD TestVerboseRoles_Off: NewVerbose(&buf,false) → buf empty.
  - ADD TestVerboseRoles_NilSafe: var v *Verbose; v.VerboseRoles(…) → no panic.
  - ADD (optional) TestReasoningSuffix: ""→"", "off"→"", "low"→" (reasoning: low)", "high"→" (reasoning: high)".
  - FOLLOW pattern: TestVerbose_CommandWhenOn (exact-bytes *bytes.Buffer assertion).
  - GOTCHA: the %-8s produces "planner " (8 wide) then a literal space → two spaces after "planner" in
    the output. Encode the EXACT expected bytes (use %q or a helper) so the assertion is byte-exact.

Task 7: EDIT internal/cmd/default_action_test.go — label-observable (generate + decompose + -v)
  - ADD TestProgressLabel_GenerateVisible: setupStubRepo; stage a change; SetArgs(["--provider","stub",
    "--model","glm-5.2"]); Execute; assert errBuf.Contains("↳ Generating with glm-5.2 in stub…").
    (Variant: no --model → errBuf.Contains("↳ Generating with stub…").)
  - ADD TestProgressLabel_DecomposeMainLineVisible: setupStubRepo; commit a file; leave an un-staged
    change; SetArgs([]) (bare → decompose routing); Execute; assert errBuf.Contains("↳ Decomposing with ")
    (the label prints BEFORE Decompose runs, so it is present even though Decompose then errors on the
    non-tooled stub stager). Do NOT assert commit creation.
  - ADD TestProgressLabel_DecomposeVerboseRoles: same as above but SetArgs(["-v"]); assert errBuf.Contains
    ("DEBUG: planner") AND ("DEBUG: stager") AND ("DEBUG: message") AND ("DEBUG: arbiter"), and the
    planner line Contains("(reasoning: high)"). (VerboseRoles prints before Decompose runs.)
  - FOLLOW pattern: setupStubRepo + rootCmd.SetOut/SetErr/SetArgs + Execute(context.Background()) +
    saveRootState/restoreRootState + resetFlags.
  - GOTCHA: resetFlags between Executes (restoreRootState handles it). The decompose tests rely on the
    label/verbose printing BEFORE Decompose — so the assertions hold despite the downstream stager error.
  - PLACEMENT: internal/cmd/default_action_test.go.
```

### Implementation Patterns & Key Details

```go
// invocation + ProgressLabel (output.go) — PURE, shared, byte-exact.
func invocation(model, provider string) string {
	if provider == "" {
		return ""
	}
	if model != "" {
		return model + " in " + provider
	}
	return provider
}
func ProgressLabel(verb, model, provider string) string {
	if inv := invocation(model, provider); inv != "" {
		return verb + " with " + inv + "…"
	}
	return verb + "…"
}

// VerboseRoles (verbose.go) — same guard idiom as the other Verbose methods; uses invocation (output.go).
func (v *Verbose) VerboseRoles(roles []RoleLine) {
	if v == nil || v.w == nil || !v.on {
		return
	}
	for _, r := range roles {
		fmt.Fprintf(v.w, "DEBUG: %-8s %s%s\n", r.Name, invocation(r.Model, r.Provider), reasoningSuffix(r.Reasoning))
	}
}
func reasoningSuffix(level string) string {
	switch level {
	case "low", "medium", "high":
		return " (reasoning: " + level + ")"
	}
	return ""
}

// Generate-path autodetect (default_action.go) — mirror pkg/stagecoach.buildDeps exactly:
//   labelProvider := cfg.Provider
//   if labelProvider == "" {
//       var installed []string
//       for _, m := range reg.List() { if reg.IsInstalled(m) { installed = append(installed, m.Name) } }
//       labelProvider = reg.DefaultProvider(installed)
//   }
// ... then u.Progress(ui.ProgressLabel("Generating", cfg.Model, labelProvider)).
//
// Decompose (runDecompose) — keep RoleModels, emit roster + planner label, reuse the verbose sink:
//   roleManifests, roleModels, err := decompose.ResolveRoles(*cfg, reg)
//   verbose := ui.NewVerbose(stderr, cfg.Verbose)
//   verbose.VerboseRoles([]ui.RoleLine{ /* planner/stager/message/arbiter from roleModels */ })
//   u.Progress(ui.ProgressLabel("Decomposing", roleModels.Planner.Model, roleModels.Planner.Provider))
//   deps := decompose.Deps{ ..., Verbose: verbose, ... }
```

### Integration Points

```yaml
UI (internal/ui):
  - output.go: ADD invocation (unexported) + ProgressLabel (exported); UPDATE Progress doc comment.
  - verbose.go: ADD RoleLine (exported) + VerboseRoles (exported) + reasoningSuffix (unexported).
  - no new imports (fmt/io/strings already present); ui stays stdlib-only.
CMD (internal/cmd/default_action.go):
  - generate validate+label block (~L128-148): reg-once + autodetect + ProgressLabel.
  - runDecompose (~L271-300): keep RoleModels + VerboseRoles + planner ProgressLabel + reuse verbose sink.
  - imports already present (ui, provider, decompose, config, fmt, errors, exitcode, git, generate).
CONFIG/PROVIDER/DECOMPOSE: CONSUMED — no edits (RoleConfig, ResolveRoleModel, ResolveRoles/RoleModels,
  DefaultProvider/List/IsInstalled/Get/NewRegistry all consumed as-is).
NO new env vars / config keys / flags / exit codes / go.mod changes.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                       # ProgressLabel + invocation + RoleLine + VerboseRoles + reasoningSuffix
go vet ./...                         # printf format (%-8s), unused, shadowed
gofmt -l internal/ pkg/              # MUST print nothing
golangci-lint run                    # repo linter (Makefile `make lint`)

# Scope-specific quick check:
go build ./internal/ui/... ./internal/cmd/... && go vet ./internal/ui/... ./internal/cmd/...

# Expected: zero errors. Verify: invocation is defined ONCE (output.go) and called from verbose.go (same
# package) — no redefinition. Verify Progress's signature/body is UNCHANGED (only its doc comment moved).
# Verify reasoningSuffix's switch is exhaustive over low/medium/high. Verify the %-8s format compiles.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Pure formatter:
go test -race ./internal/ui/ -run 'TestProgressLabel' -v
go test -race ./internal/ui/ -run 'TestVerboseRoles|TestReasoningSuffix' -v

# Full ui package (regression — Progress/Success/Error/Verbose* stay green):
go test -race ./internal/ui/...

# Label-observable at the CLI:
go test -race ./internal/cmd/ -run 'TestProgressLabel_' -v

# Full suite (Makefile `make test`):
go test -race ./...

# Expected: all pass. If TestProgressLabel fails, the byte format drifted from FR51b (check the model-empty
# and provider-empty edges). If TestVerboseRoles_On fails on the planner line, reasoningSuffix did not fire
# for "high" OR the %-8s spacing is off — assert byte-exact (use %q). If a generate label test fails, the
# autodetect did not run (labelProvider stayed "") — verify the reg.List/IsInstalled/DefaultProvider block.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build + exercise the real label surface (smoke):
go build -o /tmp/stagecoach ./cmd/stagecoach
# (In a repo with staged changes + a configured provider/model, stderr shows the FR51b line.)
# CI relies on TestProgressLabel_GenerateVisible / _DecomposeMainLineVisible / _DecomposeVerboseRoles.

# Coverage gate (gated: git/provider/generate/config — ui is NOT gated; confirm no regression):
make coverage-gate

# Expected: PASS. This task edits internal/ui (not gated) + internal/cmd (not gated); it adds tests so no
# gated package's coverage drops. (No new exported symbols in gated packages.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# FR51b byte-format invariant (TestProgressLabel encodes this):
#   model set   + provider set → "<Verb> with <model> in <provider>…"
#   model empty + provider set → "<Verb> with <provider>…"
#   provider empty (nothing resolved) → "<Verb>…"   # generate path, no provider installed
#   decompose planner → verb "Decomposing"; the model carries its inference prefix verbatim (FR-R5b).

# Verbose roster invariant (TestVerboseRoles encodes this):
#   exactly 4 lines, one per role, "DEBUG: <padded-name> <invocation><suffix>";
#   planner suffix = " (reasoning: high)" (shipped default); others no suffix; nil-safe + off ⇒ 0 bytes.

# Stream invariant: Progress + VerboseRoles write to STDERR only (FR51) — stdout stays clean for piping
# (`stagecoach --dry-run --no-color | tee …`). Assert outBuf is empty of "↳"/"DEBUG:" in the cmd tests.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` compiles (ProgressLabel/invocation/RoleLine/VerboseRoles/reasoningSuffix).
- [ ] `go vet ./...` clean (watch the %-8s printf).
- [ ] `go test -race ./...` green (Makefile `make test`).
- [ ] `golangci-lint run` clean (Makefile `make lint`).
- [ ] `gofmt -l internal/ pkg/` empty.
- [ ] `make coverage-gate` PASS (no gated-package regression).
- [ ] go.mod/go.sum UNCHANGED (stdlib only; ui/provider/decompose/config already imported by cmd).

### Feature Validation

- [ ] `ProgressLabel` produces the FR51b byte format for all combos (TestProgressLabel_* green).
- [ ] Generate path stderr shows `↳ Generating with <model> in <resolved-provider>…` (model set) /
      `↳ Generating with <resolved-provider>…` (model empty), incl. the auto-detected provider.
- [ ] Decompose main-line stderr shows `↳ Decomposing with <planner-model> in <planner-provider>…`.
- [ ] `VerboseRoles` prints 4 `DEBUG:` lines under `-v` (planner `(reasoning: high)`); 0 bytes when off/nil.
- [ ] Progress + VerboseRoles go to STDERR only (stdout clean).
- [ ] Existing `output_test.go` / `verbose_test.go` / `default_action_test.go` assertions stay green.

### Code Quality Validation

- [ ] `invocation` defined ONCE (output.go), shared by ProgressLabel + VerboseRoles (no duplication).
- [ ] VerboseRoles uses the SAME nil/w/off guard idiom as the other Verbose methods.
- [ ] generate autodetect mirrors `buildDeps` exactly (List/IsInstalled/DefaultProvider).
- [ ] runDecompose reuses ONE verbose sink for both VerboseRoles and deps.Verbose.
- [ ] Progress signature/body UNCHANGED (only its doc comment updated — Mode A).
- [ ] No reasoning in the main label (FR51b); reasoning only in the --verbose roster.

### Documentation & Deployment

- [ ] Mode-A: `Progress` doc comment updated with the FR51b example (`ProgressLabel` usage + `↳ …` result).
- [ ] Doc comments on ProgressLabel, invocation, RoleLine, VerboseRoles, reasoningSuffix citing FR51b.
- [ ] No new env vars / config keys / flags / exit codes.
- [ ] Changeset-level README/docs sync is P4.M2.T1.S1 (NOT this task — do not edit README.md).

---

## Anti-Patterns to Avoid

- ❌ Don't split the model string on `/` or try to extract a sub-provider — FR-R5b says the model already
  carries the inference backend; print it VERBATIM ("no special formatting is needed").
- ❌ Don't add reasoning to the main progress label — FR51b's label is `<model> in <provider>` only;
  reasoning appears ONLY in the `--verbose` four-role roster.
- ❌ Don't define `invocation` twice — it lives ONCE in output.go and is shared by verbose.go (same
  package). Duplicating it drifts the two formats.
- ❌ Don't change `ui.Progress`'s signature or body — only its DOC COMMENT (Mode A). The label formatting
  moves to `ProgressLabel`, which the call sites pass INTO `Progress`.
- ❌ Don't use `cfg.Provider` directly for the generate label when `--provider` is unset — auto-detect via
  `reg.DefaultProvider(installed)` (mirror buildDeps) so the label names the resolved provider. The old
  code printed only "Generating…" in that case.
- ❌ Don't discard `ResolveRoles`'s `RoleModels` (the `_`) in runDecompose — FR51b needs the planner's
  resolved model+provider for the main line and all four roles for `--verbose`.
- ❌ Don't build two verbose sinks in runDecompose — extract ONE `verbose` and reuse it for both
  `VerboseRoles` and `deps.Verbose` (shared destination + on/off).
- ❌ Don't add an `IsInstalled` pre-flight gate at the CLI for the generate path — `buildDeps` owns
  pre-flight; the CLI only computes the provider NAME for the label.
- ❌ Don't ignore the `DecodeUserOverrides` error in the generate block (the old `_`) — surface it
  (exitcode.New(Error, …)), consistent with runDecompose.
- ❌ Don't assert commit creation in the decompose label tests — the bare stub can't run the tooled
  stager, so Decompose errors downstream; assert the label + verbose lines are EMITTED (they print first).
- ❌ Don't touch decompose.go / config / provider / pkg/stagecoach — this task is ui + cmd only. The
  parallel P3.M2.T1.S2 owns decompose.go's one-file short-circuit (no file overlap).
