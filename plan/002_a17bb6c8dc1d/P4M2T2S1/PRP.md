---
name: "P4.M2.T2.S1 — Register config upgrade (verify) + apply help de-duplication FR-B6 (verify + lock-in) + verify populated config-init flag wiring (verify + lock-in). Primarily a VERIFICATION + REGRESSION-TEST HARDENING task: all three contract items are already satisfied in the current code; the deliverable value is locking them in so they cannot regress."
description: |

  EDIT `internal/cmd/config_test.go` (STRENGTHEN `TestConfigGroup_NoSubcommandPrintsHelp` + ADD a full
  init→upgrade lifecycle test) and EDIT `internal/cmd/providers_test.go` (STRENGTHEN
  `TestProvidersGroup_NoSubcommandPrintsHelp`). NO production source edits are required — the contract's
  three code changes (register `configUpgradeCmd`; remove the manual "Subcommands:" block from
  `configCmd.Long`/`providersCmd.Long`; wire `config init` flags) are ALL ALREADY PRESENT in the current
  codebase (verified at runtime + source — see plan/002_a17bb6c8dc1d/P4M2T2S1/research/VERIFIED_CURRENT_STATE.md).
  The task is therefore: (1) VERIFY the three items hold; (2) ADD regression tests that LOCK them in;
  (3) DEFENSIVELY apply a minimal one-line fix ONLY IF (contrary to current verification) any item is
  found missing at implementation time.

  ───────────────────────────────────────────────────────────────────────────────────────────────────
  VERIFIED CURRENT STATE (captured 2026-07-01 — re-confirm at implementation time; see research file):
  ───────────────────────────────────────────────────────────────────────────────────────────────────
  1. `configUpgradeCmd` IS registered: `internal/cmd/config.go` `init()` calls
     `configCmd.AddCommand(configUpgradeCmd)`. Runtime: `stagecoach config --help` lists `upgrade` in
     "Available Commands"; `stagecoach config upgrade --help` prints help + exits 0.
  2. FR-B6 IS applied: `grep -rn "Subcommands" internal/cmd/` → NONE. `configCmd.Long` and
     `providersCmd.Long` contain only prose; cobra's auto "Available Commands" is the single source.
     Runtime: each leaf (config: init/path/upgrade; providers: list/show) appears EXACTLY ONCE.
  3. `config init` flags ARE wired: `config.go` `init()` registers `--provider`/`--force`/`--template`
     as LOCAL flags on `configInitCmd` (correct — config init is in `shouldSkipConfigLoad`, so it reads
     them via `cmd.Flags().Get*` in `runConfigInit`, NOT via root's persistent set / config.Load).
     Runtime: all three appear in `stagecoach config init --help`.
  Full lifecycle WORKS: init writes a POPULATED config (with `config_version = 2`); `config upgrade` on
  it reports "already up to date"; an older (`config_version = 1`) file upgrades to 2; no-file errors
  pointing at `config init`. (`CurrentConfigVersion = 2`; `GenerateBootstrapConfig` writes the version.)

  ───────────────────────────────────────────────────────────────────────────────────────────────────
  CRITICAL SCOPE BOUNDARY — PARALLEL COORDINATION WITH P4.M2.T1.S1 (READ THIS FIRST):
  ───────────────────────────────────────────────────────────────────────────────────────────────────
  P4.M2.T1.S1 is being implemented IN PARALLEL and EDITS `pkg/stagecoach/stagecoach.go` +
  `pkg/stagecoach/stagecoach_test.go` (the public Decompose API). It does NOT touch `internal/cmd/`. This
  task edits ONLY `internal/cmd/config_test.go` + `internal/cmd/providers_test.go` (test files). The two
  tasks touch DISJOINT files → no merge conflict. Do NOT edit `pkg/stagecoach/*` (parallel ownership).
  Do NOT edit production source in `internal/cmd/config.go`/`providers.go`/`root.go` UNLESS the defensive
  conditional below fires (an item is unexpectedly missing) — the code is already correct.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/cmd/config.go, providers.go, root.go — production source is CORRECT as-is; edit ONLY if
      the defensive conditional fires (item missing at impl time). Any edit here risks racing the
      parallel tree / the already-complete P1.M4.* work.
    - pkg/stagecoach/* — OWNED BY P4.M2.T1.S1 (parallel). DO NOT TOUCH.
    - internal/config/* (GlobalConfigPath, GenerateBootstrapConfig, CurrentConfigVersion, Load) — CONSUMED.
    - PRD.md, tasks.json, .gitignore — NEVER modify.

  DELIVERABLES (2 test-file EDITS — no production source edits required):
    EDIT internal/cmd/config_test.go   — STRENGTHEN TestConfigGroup_NoSubcommandPrintsHelp (assert
                                         "upgrade" registration + FR-B6 no "Subcommands:" + "Available
                                         Commands" single-source) + ADD TestConfigLifecycle_InitThenUpgrade
                                         (init writes populated config_version=2 → upgrade "already up to date").
    EDIT internal/cmd/providers_test.go — STRENGTHEN TestProvidersGroup_NoSubcommandPrintsHelp (assert
                                         FR-B6 no "Subcommands:" + "Available Commands" single-source).

  SUCCESS: `go build ./... && go vet ./... && go test -race ./internal/cmd/...` green; `golangci-lint run`
  clean; `gofmt -l internal/` empty; the strengthened tests FAIL if a manual "Subcommands:" block is
  re-added or `configUpgradeCmd` is unregistered (regression locked in); the lifecycle test proves
  init→upgrade end-to-end; runtime `stagecoach config`/`stagecoach config init --help`/`stagecoach config
  upgrade --help`/`stagecoach providers` match the contract (no duplication, upgrade registered, flags
  wired). go.mod/go.sum UNCHANGED.

---

## Goal

**Feature Goal**: Finalize and HARDEN the v2 config-CLI surface so that the three contract items hold and
cannot regress (PRD §9.17 FR-B5/FR-B6, §15.3, §9.8 FR38). Concretely: (1) `config upgrade` is registered
on the `config` parent command and appears exactly once in its help; (2) the `config` and `providers`
parent commands list each leaf EXACTLY ONCE via cobra's auto-generated "Available Commands" — no manual
"Subcommands:" block (FR-B6); (3) `config init`'s `--provider`/`--force`/`--template` flags are wired and
functional; (4) the full `config init` → (populated config) → `config upgrade` lifecycle works. All four
are already TRUE in the current code — this task's job is to VERIFY them at runtime and ADD regression
tests that encode FR-B6 + the registration + the lifecycle as executable, failing-on-regression checks.

**Deliverable** (2 test-file EDITS — no production source edits required):
1. `internal/cmd/config_test.go` (EDIT) — STRENGTHEN `TestConfigGroup_NoSubcommandPrintsHelp` to assert:
   (a) `upgrade` appears in the help (registration, the gap the existing test misses — it only checked
   `init`+`path`); (b) the help does NOT contain the literal `"Subcommands:"` (FR-B6 negative); (c) the
   help DOES contain `"Available Commands:"` exactly once (FR-B6 positive — cobra is the single source).
   ADD `TestConfigLifecycle_InitThenUpgrade`: runs `config init` (populated) → asserts the written file
   contains an uncommented `config_version = 2` and is populated (e.g. a `[defaults]` or `[role.` block)
   → runs `config upgrade` → asserts "already up to date" / "no changes" stdout + the file unchanged.
2. `internal/cmd/providers_test.go` (EDIT) — STRENGTHEN `TestProvidersGroup_NoSubcommandPrintsHelp` to
   assert: (a) the help does NOT contain `"Subcommands:"` (FR-B6 negative); (b) the help DOES contain
   `"Available Commands:"` exactly once (FR-B6 positive). (`list`+`show` presence is already asserted.)

**Success Definition**:
- **Verification (runtime, must hold — re-run at impl time)**: `stagecoach config` help lists `init`,
  `path`, `upgrade` each EXACTLY ONCE under "Available Commands" with no "Subcommands:" block;
  `stagecoach providers` help lists `list`, `show` each EXACTLY ONCE with no "Subcommands:" block;
  `stagecoach config init --help` shows `--provider`/`--force`/`--template` in its "Flags:" section;
  `stagecoach config upgrade --help` is reachable and exits 0.
- **Regression lock-in**: the strengthened config + providers tests FAIL if (a) a manual "Subcommands:"
  block is re-added to either parent's `Long`, OR (b) `configCmd.AddCommand(configUpgradeCmd)` is removed
  (the `upgrade`-presence assertion fails). The lifecycle test proves init→upgrade end-to-end.
- **Lifecycle**: `TestConfigLifecycle_InitThenUpgrade` green (init writes populated `config_version=2` →
  upgrade reports already-current → file byte-identical).
- `go build ./... && go vet ./... && go test -race ./internal/cmd/...` green; `golangci-lint run` clean;
  `gofmt -l internal/` empty; go.mod/go.sum UNCHANGED; NO production source changes (unless the
  defensive conditional fires).

## User Persona

**Target User**: a Stagecoach end user running the CLI (`stagecoach config …`, `stagecoach providers …`) —
specifically a new user bootstrapping their config (`config init`) and a returning user refreshing it
after an upgrade (`config upgrade`). Transitive: the maintainer, who must not let FR-B6's help
de-duplication silently regress.

**Use Case**: the user runs `stagecoach config` to discover the available config subcommands, `stagecoach
config init` to bootstrap a working config, and later `stagecoach config upgrade` after a binary upgrade
that bumped the schema version. They expect clean, non-redundant help and a working init→upgrade flow.

**User Journey**: (1) `stagecoach config` → sees init/path/upgrade once each; (2) `stagecoach config init`
→ "Wrote config to …"; (3) [binary upgrade] `stagecoach config upgrade` → "Upgraded … to version N" (or
"already up to date"); (4) `stagecoach config path` → confirms the file location.

**Pain Points Addressed**: (a) redundant help text (the v1 `config` help showed init/path twice — once in
prose, once in Available Commands) that confuses discovery; (b) a `config upgrade` command that was
implemented (P1.M4.T3.S1) but, per the contract's note, needed explicit registration verification; (c)
un-tested init→upgrade lifecycle that could break silently.

## Why

- **Business value**: closes the v2 config-CLI surface (PRD §9.17, §15.3, G8). FR-B6's help de-duplication
  is a documented, user-facing quality fix (the v1 `stagecoach config` output was redundant); the
  `config upgrade` command (FR-B5) is the remediation the load-time `config_version` advisory points
  users at. This task ensures both are wired, verified, and regression-proofed.
- **Integration with existing features**: `config upgrade` (P1.M4.T3.S1) consumes `config.CurrentConfigVersion`
  + `config.GlobalConfigPath` + the pure `upgradeConfigVersion` transform; `config init` (P1.M4.T2.S1)
  consumes `config.GenerateBootstrapConfig` (which writes the populated config INCLUDING
  `config_version`). The load-time advisory (P1.M4.T1.S1) points users at `config upgrade`. All three
  leaves are in `shouldSkipConfigLoad` (root.go) so they work outside a git repo.
- **Problems this solves and for whom**: locks in FR-B6 so a future edit cannot silently re-introduce the
  duplicated help; locks in the `config upgrade` registration so it cannot be dropped during a refactor;
  adds the missing init→upgrade lifecycle test. Without these, the code is correct TODAY but undefended
  against regression — exactly the gap a verification + hardening task fills.

## What

**User-visible behavior** (CLI help — must hold; verified at runtime, locked in by tests):
- `stagecoach config` (bare or `--help`) prints the `config` Short/Long prose + a single cobra
  "Available Commands:" block listing `init`, `path`, `upgrade` each EXACTLY ONCE. NO "Subcommands:" block.
- `stagecoach providers` (bare or `--help`) prints the `providers` prose + a single "Available Commands:"
  block listing `list`, `show` each EXACTLY ONCE. NO "Subcommands:" block.
- `stagecoach config init --help` shows `--provider`, `--force`, `--template` in its local "Flags:" section.
- `stagecoach config upgrade --help` is reachable and exits 0.

**Functional behavior** (lifecycle — verified, locked in by the new lifecycle test):
- `config init` (populated, default) writes a config containing an uncommented `config_version = 2` and
  populated `[defaults]`/`[role.*]` blocks (NOT the inert all-commented template).
- `config upgrade` on that fresh-init'd config reports "already at version 2 (no changes)" and leaves the
  file byte-identical (idempotent). On an older (`config_version = 1`) file it rewrites to 2. With no
  file it errors (exit 1) pointing at `config init`.

**Technical requirements**: 2 test-file edits (strengthen 2 existing tests + add 1 lifecycle test). NO
production source edits (defensive conditional below excepted).

### Success Criteria

- [ ] Runtime verification (re-run at impl time): `stagecoach config` help has NO "Subcommands:" block;
      `config upgrade` appears once in "Available Commands"; `stagecoach providers` help has NO
      "Subcommands:" block; `stagecoach config init --help` shows the 3 flags; `config upgrade --help`
      exits 0. (Commands in §Validation Loop Level 3.)
- [ ] `TestConfigGroup_NoSubcommandPrintsHelp` (strengthened) asserts: help contains `"upgrade"` AND
      `"Available Commands:"`; help does NOT contain `"Subcommands:"`.
- [ ] `TestProvidersGroup_NoSubcommandPrintsHelp` (strengthened) asserts: help contains
      `"Available Commands:"`; help does NOT contain `"Subcommands:"`.
- [ ] `TestConfigLifecycle_InitThenUpgrade` (new) asserts: `config init` writes a file containing
      `config_version = 2` (uncommented) and a populated block; `config upgrade` then reports the
      already-current message and leaves the file byte-identical.
- [ ] The strengthened/added tests FAIL if a manual "Subcommands:" block is re-added OR `configUpgradeCmd`
      registration is removed (proven by the assertions above — the implementer should reason about this,
      not necessarily mutate-then-revert).
- [ ] `go build ./... && go vet ./... && go test -race ./internal/cmd/...` green; `golangci-lint run`
      clean; `gofmt -l internal/` empty; go.mod/go.sum UNCHANGED; NO production source edits (unless the
      defensive conditional fires and is documented in the commit/PRP-acceptance).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ — YES. The exact current source state is quoted (the `init()` registrations, the two
`Long` strings, the `shouldSkipConfigLoad` membership). The exact existing test functions to strengthen
(named + line numbers) and the exact test-helper signatures (`setupNoRepo`, `writeConfigFile`,
`saveRootState`/`restoreRootState`, `Execute`, `exitcode.For`) are given. The exact assertion strategy
for FR-B6 (negative `"Subcommands:"` absent + positive `"Available Commands:"` present) is specified with
the reasoning for why counting leaf-name occurrences is NOT used (fragile — leaf names appear in flag
descriptions). The lifecycle test is fully specified with the temp-HOME isolation pattern. The defensive
conditional minimal-fix (the ONLY case where production source is touched) gives the exact one-line edit
per item. The runtime verification commands are executable as-is.

### Documentation & References

```yaml
# MUST READ — include these in your context window
- url: PRD.md §9.17 (FR-B5 config upgrade, FR-B6 help de-duplication) — the authoritative contract
  why: "FR-B6 is the SPECIFIC requirement this task locks in: 'The config and providers parent commands
        must not list their leaf commands twice. The manual \"Subcommands:\" block is removed from each
        parent's Long; cobra's auto-generated \"Available Commands\" is the single source.' FR-B5 is the
        config upgrade command (the registration this task verifies)."
  critical: "FR-B6 targets ONLY the config + providers PARENT commands' 'Subcommands:' blocks — NOT leaf
        commands. The config-init leaf's embedded prose 'Flags:' block is a separate, OUT-OF-SCOPE
        redundancy (do not touch it). The negative regression assertion is literally: help must NOT
        contain the string 'Subcommands:'."

- url: PRD.md §15.3 (Subcommands reference) + §9.8 FR38 (config init/path/upgrade)
  why: "The CLI surface contract: config has init/path/upgrade; providers has list/show. config upgrade
        rewrites to the current schema in place (FR-B5)."
  critical: "config init --provider/--force/--template flags (FR-B2). config upgrade is the remediation
        the FR-B4 load-time advisory points at."

# CODEBASE FILES — edit targets + consumed dependencies (all verified, paths exact)
- file: internal/cmd/config_test.go   # EDIT TARGET (strengthen 1 test + add 1 lifecycle test)
  why: "Home of TestConfigGroup_NoSubcommandPrintsHelp (line 554 — the test to strengthen) and all
        TestConfigUpgrade_*/TestConfigInit_* tests (the pattern to follow). The strengthened test must
        add the 'upgrade' presence assertion (the registration gap) + the FR-B6 assertions."
  pattern: "Copy the exact scaffold of TestConfigGroup_NoSubcommandPrintsHelp: saveRootState/restoreRootState
        around rootCmd mutation; setupNoRepo(t); rootCmd.SetOut(&buf)/SetErr(io.Discard)/SetArgs; Execute;
        assert on buf.String(). The new TestConfigLifecycle_InitThenUpgrade follows the scaffold of
        TestConfigUpgrade_AlreadyCurrent (which already does init-less 'already current' — the new test
        ADDS the config init step first) + TestConfigInit_Populated_WritesWorkingConfig (for the populated
        assertions)."
  gotcha: "setupNoRepo sets XDG_CONFIG_HOME=home (temp), so config.GlobalConfigPath() resolves to
        <home>/stagecoach/config.toml (globalDir = filepath.Join(home,'stagecoach')). Use config.GlobalConfigPath()
        to read the written file (NOT a hardcoded path). The existing tests read via os.ReadFile(config.GlobalConfigPath())."

- file: internal/cmd/providers_test.go   # EDIT TARGET (strengthen 1 test)
  why: "Home of TestProvidersGroup_NoSubcommandPrintsHelp (line 344 — the test to strengthen). Add the
        FR-B6 assertions (no 'Subcommands:', 'Available Commands:' present)."
  pattern: "Same scaffold as the config sibling (saveRootState/restoreRootState; setupRepo(t) — note
        providers uses setupRepo not setupNoRepo, because providers list/show go through config.Load and
        need a git repo; but the PARENT help test just needs Execute to print help, which works in either).
        KEEP setupRepo(t) to match the existing test (do not switch to setupNoRepo — minimize diff)."

- file: internal/cmd/config.go   # CONSUMED (read-only unless defensive conditional fires)
  why: "The production source. configCmd (Long = one prose line), configInitCmd (--provider/--force/
        --template registered in init()), configPathCmd, configUpgradeCmd (registered in init()).
        shouldSkipConfigLoad covers init/path/upgrade. runConfigInit uses config.GenerateBootstrapConfig.
        runConfigUpgrade uses config.GlobalConfigPath + upgradeConfigVersion + config.CurrentConfigVersion."
  pattern: "If the defensive conditional fires: (1) registration missing → add 'configCmd.AddCommand
        (configUpgradeCmd)' to init(); (2) a manual 'Subcommands:' block present → delete it from the
        Long string; (3) a config-init flag missing → add its 'configInitCmd.Flags().*(...)' line to
        init(). Each is a one-line edit mirroring the existing siblings."

- file: internal/cmd/providers.go   # CONSUMED (read-only unless defensive conditional fires)
  why: "providersCmd (Long = prose only). If a manual 'Subcommands:' block is present (defensive case),
        delete it from the Long string."
  gotcha: "providersCmd has NO RunE → bare 'stagecoach providers' prints help (cobra default). Same for
        configCmd. This is WHY the bare-command help test works without a RunE."

- file: internal/cmd/root.go   # CONSUMED (read-only)
  why: "shouldSkipConfigLoad(cmd) returns true for cmd.Name()=='init'||'path'||'upgrade' — confirms the
        three config leaves work OUTSIDE a git repo and never call config.Load. This is WHY config init's
        flags are LOCAL to configInitCmd (correct design) — do NOT move them to root's persistent flags."

- file: internal/config/config.go   # CONSUMED — CurrentConfigVersion + GlobalConfigPath
  why: "`const CurrentConfigVersion = 2` (line 18). `func GlobalConfigPath() string`. The lifecycle test
        asserts the written file contains 'config_version = 2' matching CurrentConfigVersion — reference
        the constant, do not hardcode '2' if you prefer (but the existing tests hardcode '2' for clarity;
        either is acceptable — match the existing TestConfigUpgrade_AddsVersion style)."

- file: internal/config/bootstrap.go   # CONSUMED — GenerateBootstrapConfig writes config_version
  why: "`func GenerateBootstrapConfig(prov string) string` (line 20) writes the POPULATED config including
        `fmt.Fprintf(&b, 'config_version = %d\n', CurrentConfigVersion)` (line 117). This is WHY the
        fresh-init'd config already has config_version=2 → upgrade reports 'already up to date'."
```

### Current Codebase tree (relevant subset)

```bash
internal/cmd/
  config.go              # CONSUMED (read-only unless defensive cond) — configCmd + 3 leaves + init() registration
  providers.go           # CONSUMED (read-only unless defensive cond) — providersCmd + list/show
  root.go                # CONSUMED — shouldSkipConfigLoad (init/path/upgrade skip config.Load)
  config_test.go         # EDIT — strengthen TestConfigGroup_NoSubcommandPrintsHelp + add lifecycle test
  providers_test.go      # EDIT — strengthen TestProvidersGroup_NoSubcommandPrintsHelp
  root_test.go           # CONSUMED — writeConfigFile, saveRootState, restoreRootState helpers
internal/config/
  config.go              # CONSUMED — CurrentConfigVersion=2, GlobalConfigPath
  bootstrap.go           # CONSUMED — GenerateBootstrapConfig (writes populated config + config_version)
pkg/stagecoach/*          # OWNED BY P4.M2.T1.S1 (parallel) — DO NOT TOUCH
```

### Desired Codebase tree (files this task EDITS — no production source, no new files)

```bash
internal/cmd/config_test.go     # +strengthen TestConfigGroup_NoSubcommandPrintsHelp  +TestConfigLifecycle_InitThenUpgrade
internal/cmd/providers_test.go  # +strengthen TestProvidersGroup_NoSubcommandPrintsHelp
```

### Known Gotchas of our codebase & Library Quirks

```go
// G-ALREADY-DONE (CRITICAL): The three contract code changes are ALREADY in the codebase (verified at
//   runtime + source — see research/VERIFIED_CURRENT_STATE.md). The PRIMARY deliverable is regression
//   TESTS, not source edits. If you start by editing config.go/providers.go, STOP — re-run the runtime
//   verification (§Validation Loop Level 3); the edits are almost certainly unnecessary and risk racing
//   the parallel tree / the already-complete P1.M4.* work. Touch production source ONLY via the defensive
//   conditional (G-DEFENSIVE-CONDITIONAL).

// G-FR-B6-SCOPE-IS-PARENTS-ONLY: FR-B6 removes the manual "Subcommands:" block from the config + providers
//   PARENT commands. It does NOT touch leaf commands. The config-init leaf's embedded prose "Flags:"
//   block (--provider/--force/--template in Long) is a separate, OUT-OF-SCOPE redundancy. DO NOT remove
//   it in this task (scope discipline; no test asserts on it, but changing Long text is unnecessary churn).

// G-FR-B6-ASSERTION-STRATEGY: Encode FR-B6 as: help does NOT contain "Subcommands:" (negative) AND help
//   DOES contain "Available Commands:" (positive, cobra's single source). Do NOT count leaf-name
//   occurrences to check "exactly once" — leaf names ("init", "path", "list", "show") appear in flag
//   descriptions and prose, making counts fragile. The "Subcommands:"-absent + "Available Commands:"-present
//   pair is the canonical, robust FR-B6 regression check.

// G-REGISTRATION-ASSERTION: The registration gap is that the EXISTING TestConfigGroup_NoSubcommandPrintsHelp
//   asserts "init"+"path" but NOT "upgrade". Strengthen it to assert "upgrade" is present too — this is the
//   explicit "verify registration" deliverable. (The TestConfigUpgrade_* tests execute the command, which
//   implicitly proves registration, but an explicit help-presence assertion is clearer + fails fast.)

// G-CONFIG-INIT-FLAGS-ARE-LOCAL: config init's --provider/--force/--template are LOCAL flags on
//   configInitCmd (registered via configInitCmd.Flags().*), NOT root persistent flags. This is CORRECT and
//   intentional: config init is in shouldSkipConfigLoad (root.go) and never runs config.Load (which needs a
//   git repo). Do NOT "wire them to root.go's flag set" — the contract's parenthetical "(or they're local
//   to configInitCmd)" is the chosen design. Verify they appear in `config init --help` "Flags:".

// G-SETUPNOREPO-XDG: setupNoRepo(t) sets BOTH HOME and XDG_CONFIG_HOME to the same temp home. So
//   config.GlobalConfigPath() = <home>/stagecoach/config.toml (XDG=home → $XDG_CONFIG_HOME/stagecoach/config.toml).
//   Always read the written file via os.ReadFile(config.GlobalConfigPath()), never a hardcoded path.

// G-ROOTCMD-IS-GLOBAL-MUTABLE: rootCmd is a package global mutated by SetOut/SetErr/SetArgs in every test.
//   ALWAYS wrap with saveRootState(t)/defer restoreRootState(t,…) (the existing pattern) so tests don't
//   poison each other. restoreRootState resets Out/Err/RunE; SetArgs is consumed by Execute so it need not
//   be restored, but the existing tests pass nil for the args slot — follow suit.

// G-PROVIDERS-USES-SETUPREPO: The existing TestProvidersGroup_NoSubcommandPrintsHelp uses setupRepo(t)
//   (not setupNoRepo) because the providers leaves (list/show) go through config.Load. The PARENT help
//   test itself just needs Execute to print help, but KEEP setupRepo(t) to match the existing test and
//   avoid surprising the providers-leaf test ordering. (setupRepo creates a temp git repo + chdir.)

// G-LIFECYCLE-ALREADY-CURRENT: A fresh `config init` writes config_version = 2 (== CurrentConfigVersion),
//   so the immediately-following `config upgrade` reports "already at version 2 (no changes)" and leaves
//   the file byte-identical — this IS the lifecycle assertion (init writes the version → upgrade is a
//   no-op). To ALSO cover the "older → upgraded" path, the existing TestConfigUpgrade_OlderUpdated already
//   does (it writes config_version=1 via writeConfigFile then upgrades); do not duplicate it. The new
//   lifecycle test's UNIQUE value is chaining init→upgrade on the POPULATED bootstrap output (proving the
//   bootstrap's config_version is correct + upgrade tolerates it).

// G-DEFENSIVE-CONDITIONAL (ONLY if an item is missing at impl time): Re-run §Validation Loop Level 3 first.
//   If (1) `config upgrade` is NOT in `config` help → add `configCmd.AddCommand(configUpgradeCmd)` to
//   config.go init(). If (2) `stagecoach config`/`providers` help CONTAINS "Subcommands:" → delete that
//   manual block from the respective Long string. If (3) a config-init flag is missing from `config init
//   --help` → add its `configInitCmd.Flags().*(name, default, usage)` line to config.go init(). Each is a
//   one-line edit mirroring the existing siblings. Document any such edit in the commit message. (Per the
//   2026-07-01 verification, NONE of these should fire.)
```

## Implementation Blueprint

### Data models and structure

No new data models. This task adds/strengthens TEST FUNCTIONS only. The consumed production types are
`config.CurrentConfigVersion` (const int = 2), `config.GlobalConfigPath() string`, and
`config.GenerateBootstrapConfig(provider string) string`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: VERIFY the current state (no edits — run the runtime gates first)
  - BUILD: go build -o /tmp/sh-verify ./cmd/stagecoach
  - RUN: /tmp/sh-verify config --help        → assert "Available Commands:" present, "Subcommands:" absent,
          "upgrade" listed once.
  - RUN: /tmp/sh-verify providers --help     → assert "Available Commands:" present, "Subcommands:" absent.
  - RUN: /tmp/sh-verify config init --help   → assert --provider/--force/--template in "Flags:".
  - RUN: /tmp/sh-verify config upgrade --help → assert exits 0, prints Long help.
  - IF ALL PASS (expected per 2026-07-01 verification): proceed to Task 1 (test hardening). NO source edits.
  - IF ANY FAILS: apply the G-DEFENSIVE-CONDITIONAL one-line fix for that item, re-verify, THEN Task 1.

Task 1: EDIT internal/cmd/config_test.go — strengthen TestConfigGroup_NoSubcommandPrintsHelp
  - IN the EXISTING TestConfigGroup_NoSubcommandPrintsHelp (line ~554), AFTER the current "init"/"path"
    Contains assertions, ADD:
        if !strings.Contains(got, "upgrade") {
            t.Error(`help output missing "upgrade" subcommand (registration — configCmd.AddCommand(configUpgradeCmd))`)
        }
        if strings.Contains(got, "Subcommands:") {
            t.Error(`help output must NOT contain a manual "Subcommands:" block (FR-B6: cobra "Available Commands" is the single source)`)
        }
        if !strings.Contains(got, "Available Commands:") {
            t.Error(`help output missing cobra "Available Commands:" (FR-B6 single source)`)
        }
  - KEEP the existing "init"/"path" assertions (do not remove them). KEEP setupNoRepo + the
    saveRootState/restoreRootState scaffold unchanged.
  - WHY: the existing test only checked init+path — it missed the upgrade registration (the contract's
    item 1) and had no FR-B6 assertions (the contract's item 2). These three additions lock both in.
  - NAMING/PLACEMENT: in-place additions to the existing test function (no new function for this part).

Task 2: EDIT internal/cmd/providers_test.go — strengthen TestProvidersGroup_NoSubcommandPrintsHelp
  - IN the EXISTING TestProvidersGroup_NoSubcommandPrintsHelp (line ~344), AFTER the current "list"/"show"
    Contains assertions, ADD:
        if strings.Contains(got, "Subcommands:") {
            t.Error(`help output must NOT contain a manual "Subcommands:" block (FR-B6)`)
        }
        if !strings.Contains(got, "Available Commands:") {
            t.Error(`help output missing cobra "Available Commands:" (FR-B6 single source)`)
        }
  - KEEP the existing "list"/"show" assertions + setupRepo scaffold unchanged.
  - WHY: locks FR-B6 for the providers parent (the contract's item 2, second half).

Task 3: EDIT internal/cmd/config_test.go — ADD TestConfigLifecycle_InitThenUpgrade
  - ADD a new test function (place it near the other TestConfigUpgrade_* tests, e.g. after
    TestConfigUpgrade_Idempotent or after the group help test):
        func TestConfigLifecycle_InitThenUpgrade(t *testing.T) {
            _, origOut, origErr, origRunE := saveRootState(t)
            defer restoreRootState(t, nil, origOut, origErr, origRunE)

            setupNoRepo(t) // temp HOME+XDG; chdir to a non-git dir; global = <home>/stagecoach/config.toml

            // (1) config init — populated bootstrap (no --template, no --provider → auto-detect/default "pi")
            rootCmd.SetOut(io.Discard)
            rootCmd.SetErr(io.Discard)
            rootCmd.SetArgs([]string{"config", "init"})
            if err := Execute(context.Background()); err != nil {
                t.Fatalf("config init err=%v, want nil", err)
            }

            // (2) the written config is POPULATED and carries the current schema version
            path := config.GlobalConfigPath()
            data, err := os.ReadFile(path)
            if err != nil {
                t.Fatalf("read written config: %v", err)
            }
            content := string(data)
            if !strings.Contains(content, "config_version = 2") {
                t.Errorf("populated config missing 'config_version = 2' (GenerateBootstrapConfig must write CurrentConfigVersion);\ngot:\n%s", content)
            }
            // populated (NOT inert): it has at least one uncommented [defaults] or [role. block
            if !strings.Contains(content, "[defaults]") && !strings.Contains(content, "[role.") {
                t.Errorf("populated config appears inert (no uncommented [defaults]/[role.*]);\ngot:\n%s", content)
            }

            // (3) config upgrade on the fresh-init'd config → already current, byte-identical
            preContent := content
            var out bytes.Buffer
            rootCmd.SetOut(&out)
            rootCmd.SetErr(io.Discard)
            rootCmd.SetArgs([]string{"config", "upgrade"})
            if err := Execute(context.Background()); err != nil {
                t.Fatalf("config upgrade err=%v, want nil (already-current is success)", err)
            }
            if !strings.Contains(out.String(), "no changes") && !strings.Contains(out.String(), "already") {
                t.Errorf("upgrade stdout=%q, want 'already up to date'/'no changes'", out.String())
            }
            afterContent, _ := os.ReadFile(path)
            if string(afterContent) != preContent {
                t.Errorf("config file changed after an already-current upgrade (must be byte-identical)")
            }
        }
  - WHY: proves the full init→upgrade lifecycle the contract's item 3/4 demands ("Run through the full
    config init → use → config upgrade lifecycle"). The UNIQUE value vs the existing TestConfigUpgrade_*
    tests is that it starts from the POPULATED bootstrap output (proving GenerateBootstrapConfig writes a
    correct config_version that upgrade tolerates), rather than a hand-written writeConfigFile.
  - GOTCHA: setupNoRepo sets XDG_CONFIG_HOME=home → GlobalConfigPath = <home>/stagecoach/config.toml. Use
    config.GlobalConfigPath() to read. config init with no installed provider auto-defaults to "pi" — fine,
    the test does not assert a specific provider, only that the file is populated + has config_version=2.
    Do NOT pass --template (that writes the INERT config, which has config_version COMMENTED OUT → the
    lifecycle assertion would fail by design).
  - COVERAGE: init populated write + config_version presence + upgrade already-current idempotence.

Task 4: VALIDATE (no edits)
  - go build ./... && go vet ./...
  - go test -race ./internal/cmd/...   # the strengthened + new tests must pass
  - golangci-lint run
  - gofmt -l internal/                 # must print nothing
  - Re-run §Validation Loop Level 3 (runtime gates) to confirm the user-visible behavior is unchanged.
```

### Implementation Patterns & Key Details

```go
// The FR-B6 regression assertion pair (the heart of this task):
//
//   // NEGATIVE — the manual block must be GONE (FR-B6):
//   if strings.Contains(help, "Subcommands:") {
//       t.Error(`help must NOT contain "Subcommands:" (FR-B6)`)
//   }
//   // POSITIVE — cobra's auto block is the single source (FR-B6):
//   if !strings.Contains(help, "Available Commands:") {
//       t.Error(`help must contain cobra "Available Commands:" (FR-B6)`)
//   }
//
// Why NOT count leaf-name occurrences: "init"/"path"/"list"/"show"/"upgrade" appear in flag descriptions
// and prose, so a count is fragile. The "Subcommands:"-absent + "Available Commands:"-present pair is the
// canonical FR-B6 check and is exactly what the requirement text describes ("cobra's auto-generated
// 'Available Commands' is the single source").
//
// The registration assertion (contract item 1):
//
//   if !strings.Contains(help, "upgrade") {
//       t.Error(`help missing "upgrade" (configCmd.AddCommand(configUpgradeCmd) registration)`)
//   }
//
// The lifecycle test chains the TWO already-unit-tested commands (init + upgrade) on the POPULATED
// bootstrap output — the integration seam neither existing test covers alone.
```

### Integration Points

```yaml
CONFIG CLI (internal/cmd/config.go): CONSUMED — configCmd + configInitCmd/configPathCmd/configUpgradeCmd
  are registered in init(). shouldSkipConfigLoad covers init/path/upgrade. No changes (unless defensive).
PROVIDERS CLI (internal/cmd/providers.go): CONSUMED — providersCmd + list/show. No changes (unless defensive).
ROOT (internal/cmd/root.go): CONSUMED — shouldSkipConfigLoad. No changes.
CONFIG PACKAGE (internal/config): CONSUMED — CurrentConfigVersion(=2), GlobalConfigPath,
  GenerateBootstrapConfig (writes populated config + config_version). No changes.
TESTS (internal/cmd/{config,providers}_test.go): EDIT — strengthen 2 tests + add 1 lifecycle test.
PARALLEL (pkg/stagecoach/*): NOT TOUCHED — owned by P4.M2.T1.S1.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After the test-file edits.
go build ./...                  # compile (the new/edited test funcs)
go vet ./...                    # shadowed vars, printf, unkeyed literals
gofmt -l internal/              # MUST print nothing
golangci-lint run               # repo linter (Makefile `make lint`)

# Scope-specific quick check:
go build ./internal/cmd/... && go vet ./internal/cmd/...

# Expected: zero errors. Verify the new test funcs are gofmt-clean (gofmt the file if needed).
# Verify NO production source file appears in `git diff --name-only` (only the two _test.go files),
# UNLESS the defensive conditional fired (then exactly the one config.go/providers.go line + its note).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The strengthened + new tests, plus the full cmd suite.
go test -race ./internal/cmd/... -v -run 'TestConfigGroup_NoSubcommandPrintsHelp|TestProvidersGroup_NoSubcommandPrintsHelp|TestConfigLifecycle_InitThenUpgrade'
go test -race ./internal/cmd/...          # full cmd suite — must stay green (no regressions)
go test -race ./...                        # whole repo (sanity — parallel P4.M2.T1.S1 pkg/stagecoach tests are independent)

# Expected: all pass. If TestConfigLifecycle_InitThenUpgrade fails on 'config_version = 2', the
# GenerateBootstrapConfig contract changed — investigate (do not relax the assertion without reason).
```

### Level 3: Integration Testing (System / Runtime Validation)

```bash
# Build a fresh binary and exercise the user-visible contract (re-verify; this is Task 0 + final gate).
go build -o /tmp/sh-verify ./cmd/stagecoach

# (1) config parent help — FR-B6 (config): NO "Subcommands:", upgrade listed once.
/tmp/sh-verify config --help > /tmp/cfg.txt 2>&1
grep -q "Available Commands:" /tmp/cfg.txt && echo "OK: Available Commands present"
! grep -q "Subcommands:" /tmp/cfg.txt && echo "OK: no manual Subcommands block (FR-B6)"
grep -E "^  upgrade " /tmp/cfg.txt && echo "OK: upgrade registered (appears once)"

# (2) providers parent help — FR-B6 (providers): NO "Subcommands:", list/show once.
/tmp/sh-verify providers --help > /tmp/prov.txt 2>&1
grep -q "Available Commands:" /tmp/prov.txt && echo "OK: Available Commands present"
! grep -q "Subcommands:" /tmp/prov.txt && echo "OK: no manual Subcommands block (FR-B6)"

# (3) config init flags wired — appear in local Flags section.
/tmp/sh-verify config init --help > /tmp/init.txt 2>&1
grep -q -- "--provider" /tmp/init.txt && grep -q -- "--force" /tmp/init.txt && grep -q -- "--template" /tmp/init.txt && echo "OK: init flags wired"

# (4) config upgrade reachable — exits 0.
/tmp/sh-verify config upgrade --help >/dev/null 2>&1 && echo "OK: upgrade --help exit 0"

# (5) Full lifecycle in an isolated HOME (mirrors TestConfigLifecycle_InitThenUpgrade at the shell):
export HOME=$(mktemp -d); export XDG_CONFIG_HOME="$HOME"
/tmp/sh-verify config init
grep -q "^config_version = 2" "$HOME/stagecoach/config.toml" && echo "OK: populated config_version=2"
/tmp/sh-verify config upgrade | grep -q "no changes" && echo "OK: upgrade already-current"

# Expected: every "OK:" line prints. If any (1)-(4) check fails, apply G-DEFENSIVE-CONDITIONAL.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Regression-proof spot check: temporarily (in a scratch branch / git stash) re-introduce a manual
# "Subcommands:" block into configCmd.Long and confirm the strengthened test FAILS, then revert.
# (Optional — confirms the test actually guards FR-B6. Do NOT commit the regression.)
#
# Example mental check (do not run unless verifying the guard):
#   append "\n\nSubcommands:\n  init ...\n  path ...\n  upgrade ...\n" to configCmd.Long
#   → go test -run TestConfigGroup_NoSubcommandPrintsHelp ./internal/cmd/... MUST fail with the FR-B6 msg.

# Similarly: comment out `configCmd.AddCommand(configUpgradeCmd)` → TestConfigGroup_NoSubcommandPrintsHelp
# MUST fail on the missing "upgrade" assertion. Revert.

# Expected: the strengthened tests FAIL under the regressed source (proving they lock the contract in).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 green: `go build ./... && go vet ./... && gofmt -l internal/` empty; `golangci-lint run` clean.
- [ ] Level 2 green: `go test -race ./internal/cmd/...` (and `./...`) all pass.
- [ ] Level 3 green: all six "OK:" runtime checks print (FR-B6 both parents + registration + init flags +
      upgrade reachable + lifecycle).
- [ ] `git diff --name-only` shows ONLY `internal/cmd/config_test.go` + `internal/cmd/providers_test.go`
      (NO production source), unless a defensive conditional edit is present + documented.

### Feature Validation

- [ ] `stagecoach config` help lists init/path/upgrade each ONCE, no "Subcommands:" block (FR-B6 config).
- [ ] `stagecoach providers` help lists list/show each ONCE, no "Subcommands:" block (FR-B6 providers).
- [ ] `config upgrade` registered + reachable (registration verified + asserted).
- [ ] `config init` flags (--provider/--force/--template) wired + asserted in help.
- [ ] Full init→upgrade lifecycle works (populated config_version=2 → upgrade already-current) —
      TestConfigLifecycle_InitThenUpgrade green + Level 3 step 5.
- [ ] The strengthened tests FAIL if FR-B6 is regressed or upgrade is unregistered (Level 4 reasoning).

### Code Quality Validation

- [ ] Strengthened tests follow the existing saveRootState/restoreRootState + setupNoRepo/setupRepo scaffold.
- [ ] No new test helpers introduced (reuse writeConfigFile/setupNoRepo/etc. — though the lifecycle test
      needs none beyond os.ReadFile + config.GlobalConfigPath).
- [ ] No production source churn (scope discipline); config-init leaf prose "Flags:" block left untouched.
- [ ] go.mod/go.sum UNCHANGED; no new imports needed (bytes/context/io/os/strings/config/exitcode already
      imported in config_test.go).

### Documentation & Deployment

- [ ] No docs changes required (FR-B6's fix IS the doc fix — the help text is the documentation; the
      contract explicitly says "DOCS: none — the help text IS the documentation; removing the manual block
      IS the doc fix").
- [ ] Commit message (if a defensive conditional edit occurred) documents the one-line fix + why.

---

## Anti-Patterns to Avoid

- ❌ Don't edit production source (config.go/providers.go/root.go) without first re-running Level 3 — the
  items are already satisfied; an unneeded edit risks racing the parallel P4.M2.T1.S1 tree.
- ❌ Don't "fix" the config-init leaf's prose "Flags:" block — it is OUT of FR-B6 scope (parents only).
- ❌ Don't count leaf-name occurrences for the FR-B6 check — use the "Subcommands:"-absent +
  "Available Commands:"-present pair (robust; leaf names appear in flag descriptions).
- ❌ Don't move config init's flags to root's persistent set "so config.Load can read them" — they are
  LOCAL to configInitCmd by design (config init skips config.Load via shouldSkipConfigLoad).
- ❌ Don't pass `--template` in the lifecycle test — it writes the INERT config (config_version COMMENTED
  OUT), which would fail the config_version assertion by design.
- ❌ Don't hardcode the global config path in tests — use `config.GlobalConfigPath()` (setupNoRepo sets
  XDG_CONFIG_HOME=home → path = <home>/stagecoach/config.toml).
- ❌ Don't touch `pkg/stagecoach/*` — owned by the parallel P4.M2.T1.S1 task.
- ❌ Don't add new exported symbols or change go.mod — this is a test-hardening task.
