---
name: "P1.M3.T2.S1 — per-role + decompose env vars in loadEnv() and flag handling in loadFlags(): the ENV (layer 5) and CLI-flag (layer 7) plumbing for the v2 Config fields — PRD §9.15 FR-R3 / §9.14 FR-M2 / §15.2 / §9.8 FR35"
description: |

  Land the FIRST subtask of Per-Role Config Schema & Model Resolution's resolution half (P1.M3.T2): make
  the ENV and CLI-FLAG precedence layers populate the v2 `Config` fields S1 (SHIPPED) declared. S1 added
  `Roles map[string]RoleConfig`, `Commits int`, `Single bool`, `MaxCommits int` to config.go — but they
  stay zero/nil from the env/flag layers because `loadEnv()`/`loadFlags()` (in load.go) do not yet read the
  per-role or decompose env vars/flags. THIS subtask IS that wiring. (The FILE/git layers are S2 of M3.T1,
  implementing in PARALLEL; `ResolveRoleModel` is S2 of M3.T2; flag REGISTRATION is P4.M1.T1.S1.)

  THREE FILES, all EDITS (no new files):
    1. internal/config/load.go — loadEnv() + loadFlags() + 2 helper methods + roleNames + a Load() normalization.
    2. internal/config/load_test.go — extend newFlagSet + new tests.
    3. internal/cmd/config.go — exampleConfigTemplate HEADER env/flag docs (Mode A).

  THE load.go CHANGES (authoritative source: architecture/config_v2_delta.md §3 — matches the item contract):
    A. NEW `var roleNames = []string{"planner", "stager", "message", "arbiter"}` — one canonical role list (DRY).
    B. NEW `func (c *Config) setRoleProvider(role, provider string)` + `setRoleModel(role, model string)` —
       lazily allocate c.Roles; map-value-copy idiom (rc := c.Roles[role]; rc.X = v; c.Roles[role] = rc).
    C. loadEnv() += per-role loop (`STAGECOACH_<ROLE>_PROVIDER/_MODEL` via strings.ToUpper) → setRoleProvider/Model;
       += `STAGECOACH_COMMITS` (int; ERROR on non-integer, consistent with STAGECOACH_TIMEOUT). NEW import "strings".
    D. loadFlags() += per-role loop (`--<role>-provider/--<role>-model` via fs.Changed+GetString) → setRoleProvider/Model;
       += `--commits` (GetInt), `--single`/`--no-decompose` (→ Single=true), `--max-commits` (GetInt).
    E. Load() += `if cfg.Commits == 1 { cfg.Single = true }` AFTER loadFlags (FR-M2c/§15.2: commits 1 ≡ single;
       normalizes BOTH env and flag sources so cfg is self-consistent).

  THE load_test.go CHANGES: EXTEND `newFlagSet` to register the v2 flags (commits/single/no-decompose/
  max-commits + the 8 per-role flags) — SAFE (registration is behavior-neutral for the ~15 existing tests
  that don't Set them) — and ADD per-role/decompose env+flag tests + a commits==1⇒Single normalization test +
  a per-role flag>env precedence test + a setRole lazy-alloc/map-value-copy test.

  THE exampleConfigTemplate CHANGES (Mode A — internal/cmd/config.go): extend the HEADER `# Environment
  variables` block (add `# STAGECOACH_<ROLE>_PROVIDER/_MODEL` for the 4 roles + `# STAGECOACH_COMMITS`) and add
  a new `# CLI flags` reference block (per-role flags + --commits/--single/--no-decompose/--max-commits). ALL
  added lines `#`-commented (INERT). ⚠️ PLACED IN THE HEADER REGION (before `# [defaults]`) — DISJOINT from
  S2's [generation]/[role.*] regions (see design-decisions.md §6).

  ⚠️ **THE #1 scope boundary — config.go (S1, DONE) is FROZEN.** S1 declared the Config fields + RoleConfig;
  this subtask WRITES INTO them via load.go. The setRoleProvider/setRoleModel helpers are methods on *Config
  DEFINED IN load.go (a method may live in any file of package config — config.go stays byte-unchanged). file.go/
  git.go (S2 + others) are also FROZEN. ResolveRoleModel is S2 (P1.M3.T2.S2). Flag REGISTRATION is P4.M1.T1.S1.
  See research design-decisions.md §0/§9.

  ⚠️ **THE #2 design call — per-role field-merge via setRoleProvider/setRoleModel (map-value-copy).** A
  per-role env/flag sets ONE field (Provider OR Model) at a time; the helper does `rc := c.Roles[role];
  rc.Provider = v; c.Roles[role] = rc`. The write-back is REQUIRED (Go maps return a value COPY — assigning
  to the copy's field is lost). This is the env/flag analog of S2's overlay() Roles field-merge (FR-R3): a
  STAGECOACH_PLANNER_MODEL alone must NOT erase a `[role.planner].provider` from a lower file layer. Test it.
  See design-decisions.md §1.

  ⚠️ **THE #3 design call — STAGECOACH_COMMITS errors on a non-integer (NOT silent-ignore).** delta §3 sketches
  silent-ignore; this PRP OVERRIDES it for CONSISTENCY: loadEnv already errors on a bad STAGECOACH_TIMEOUT
  (parseTimeout) and bad STAGECOACH_VERBOSE/NO_COLOR (ParseBool) — erroring on a bad STAGECOACH_COMMITS is the
  same fail-at-load discipline and catches typos. A test pins the error. See design-decisions.md §2.

  ⚠️ **THE #4 design call — Commits==1 ⇒ Single=true normalized in Load() (covers env AND flags).** PRD §9.14
  FR-M2c + §15.2 define `--commits 1 ≡ --single`. delta §3 omits this; leaving `Commits==1, Single==false` is
  an inconsistent state downstream consumers must re-derive. Normalize ONCE in Load() after both layers so
  `STAGECOACH_COMMITS=1` AND `--commits 1` both yield Single==true. Only sets TRUE. See design-decisions.md §4.

  ⚠️ **THE #5 coordination note — flag REGISTRATION is P4.M1.T1.S1; loadFlags is DEFENSIVE.** loadFlags reads
  fs.Changed("<role>-provider") etc.; an UNREGISTERED flag reports Changed==false (skipped) — so loadFlags is
  correct and panic-free before AND after P4.M1.T1.S1 wires the flags (same relationship as the existing
  provider/model flag handling). TESTS register the v2 flags via the extended newFlagSet. See design-decisions.md §5.

  ⚠️ **THE #6 conflict flag — S2 (parallel) ALSO edits exampleConfigTemplate.** S2 edits the [generation]
  section + appends a [role.*] TOML-config section at the END. THIS subtask edits the HEADER region (env-vars
  block + new CLI-flags block, before # [defaults]). DISJOINT regions — keep both on merge. See design-decisions.md §6.

  ⚠️ **NO new dependency — stdlib `strings` only.** load.go += "strings" (for strings.ToUpper in the per-role
  env loop). No third-party, no internal/*. `go mod tidy` MUST be a no-op. See design-decisions.md §9.

  Deliverable: MODIFIED internal/config/load.go (roleNames + setRoleProvider/setRoleModel + loadEnv + loadFlags
  + Load normalization) + MODIFIED internal/config/load_test.go (newFlagSet extension + new tests) + MODIFIED
  internal/cmd/config.go (exampleConfigTemplate header env/flag docs). NO other file touched. OUTPUT: per-role
  env/flag overrides resolve into cfg.Roles; decompose flags (Commits/Single/MaxCommits) resolve from env/flags;
  `go build ./... && go test ./...` green.

---

## Goal

**Feature Goal**: Wire the ENV (PRD §16.1 layer 5) and CLI-flag (layer 7) precedence layers so the v2
`Config` fields S1 declared actually populate from `STAGECOACH_<ROLE>_PROVIDER`/`_MODEL` (4 roles, §9.15 FR-R3),
`STAGECOACH_COMMITS` (§9.14 FR-M2), `--<role>-provider`/`--<role>-model` (4 roles), and `--commits`/`--single`/
`--no-decompose`/`--max-commits` (§9.14). After S1, these fields are zero/nil from env/flags; this subtask
makes them resolve, with the per-role field-merge semantics (FR-R3) and the `commits==1≡single` consistency
normalization (FR-M2c). Downstream consumers (`ResolveRoleModel` S2, the decompose orchestrator P3) can then
read a self-consistent `cfg.Roles`/`cfg.Commits`/`cfg.Single`/`cfg.MaxCommits`.

**Deliverable** (all EDITS — no new files):
1. `internal/config/load.go`:
   - ADD `var roleNames = []string{"planner", "stager", "message", "arbiter"}`.
   - ADD `func (c *Config) setRoleProvider(role, provider string)` and `func (c *Config) setRoleModel(role,
     model string)` (lazily allocate `c.Roles`; map-value-copy write-back).
   - EXTEND `loadEnv()`: per-role `STAGECOACH_<ROLE>_PROVIDER`/`_MODEL` loop → `setRoleProvider`/`setRoleModel`;
     `STAGECOACH_COMMITS` (int; wrapped error on non-integer). ADD `"strings"` import.
   - EXTEND `loadFlags()`: per-role `--<role>-provider`/`--<role>-model` loop (fs.Changed+GetString);
     `--commits`/`--max-commits` (GetInt); `--single`/`--no-decompose` (→ `Single=true`).
   - EXTEND `Load()`: `if cfg.Commits == 1 { cfg.Single = true }` after loadFlags.
2. `internal/config/load_test.go`:
   - EXTEND `newFlagSet` to register the v2 flags (8 per-role + commits/single/no-decompose/max-commits).
   - ADD per-role/decompose env+flag tests, a commits==1⇒Single test, a per-role flag>env precedence test,
     and a setRole lazy-alloc/map-value-copy test.
3. `internal/cmd/config.go`:
   - EXTEND `exampleConfigTemplate`: header `# Environment variables` block (add per-role vars + COMMITS) and a
     new `# CLI flags` reference block — both in the header region, `#`-commented (INERT).

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `STAGECOACH_PLANNER_PROVIDER`/
`_MODEL` populate `cfg.Roles["planner"]`; `--stager-model` (Changed) populates `cfg.Roles["stager"].Model`
without erasing a lower-layer `cfg.Roles["stager"].Provider`; `STAGECOACH_COMMITS=3` → `cfg.Commits==3`;
`STAGECOACH_COMMITS=abc` → wrapped error; `--single`/`--no-decompose` → `cfg.Single==true`; `STAGECOACH_COMMITS=1`
AND `--commits 1` → `cfg.Single==true` (the §4 normalization); `--max-commits` → `cfg.MaxCommits`; a per-role
flag beats a per-role env var; go.mod/go.sum byte-unchanged; config.go (S1) + file.go/git.go + every non-target
file byte-unchanged.

## User Persona

**Target User**: The downstream v2 consumers that read these resolved fields:
- **P1.M3.T2.S2 (`ResolveRoleModel`)** — reads `cfg.Roles[role]` (now populated by BOTH the file layer S2-of-
  M3.T1 AND these env/flag layers) then falls back to `cfg.Provider`/`cfg.Model`. This subtask makes the env/
  flag per-role overrides appear in `cfg.Roles` so ResolveRoleModel's "flag > env > [role.*] config > global"
  precedence holds (the flag/env layers write DIRECTLY into `cfg.Roles`, the highest layers).
- **P3.M2/M4 (decompose orchestrator)** — reads `cfg.Single` (bypass planner), `cfg.Commits` (forced count),
  `cfg.MaxCommits` (safety cap). This subtask makes `--commits`/`--single`/`--max-commits`/`STAGECOACH_COMMITS`
  populate them, and normalizes `Commits==1⇒Single` so the orchestrator reads a consistent cfg.
End-user persona is "the multi-agent tinkerer" (PRD §7.3) who routes different decomposition roles to different
agents (`stagecoach --planner-model gemini-2.5-pro --planner-provider agy`, §15.5) or forces a commit count
(`stagecoach --commits 3`).

**Use Case**: A user runs `STAGECOACH_PLANNER_MODEL=gemini-2.5-pro stagecoach --planner-provider agy` in a dirty
repo. loadEnv sets `cfg.Roles["planner"].Model="gemini-2.5-pro"`; loadFlags (higher layer) sets
`cfg.Roles["planner"].Provider="agy"`. Both write into the SAME `cfg.Roles["planner"]` entry (field-merge —
FR-R3), so the planner resolves to `{agy, gemini-2.5-pro}` — neither field clobbers the other. Separately,
`stagecoach --commits 1` sets `cfg.Commits=1`, and Load() normalizes `cfg.Single=true` → the planner is bypassed.

**User Journey**: (internal) `flag.Parse` (P4.M1.T1) → `Load()` → `Defaults()` → file/git overlays (S2-of-M3.T1)
→ `loadEnv` (per-role env → `cfg.Roles`; `STAGECOACH_COMMITS` → `cfg.Commits`) → `loadFlags` (per-role/decompose
flags → `cfg.Roles`/`cfg.Commits`/`cfg.Single`/`cfg.MaxCommits`) → `Commits==1⇒Single` normalize → resolved
`*Config` → `ResolveRoleModel` / orchestrator.

**Pain Points Addressed**: (1) v2 env/flags being silently ignored (S1 declared the fields; without this they
stay zero/nil from env/flags) — a user's `--planner-model` would do nothing. (2) A per-role override
clobbering its sibling field — the field-merge helper prevents `--planner-model` from erasing a `[role.planner]
.provider`. (3) `--commits 1` not behaving as `--single` — the Load() normalization makes the equivalence hold.

## Why

- **Activates the env/flag half of the v2 Config schema.** S1 declared the fields; the file/git layers (S2-of-
  M3.T1) populate them from files; THIS subtask populates them from env vars and CLI flags (the two HIGHEST
  precedence layers). Without it, `--planner-model`/`STAGECOACH_COMMITS`/`--single` are no-ops.
- **Satisfies PRD §9.15 FR-R3 (per-role env/flags), §9.14 FR-M2 (decompose flags), §15.2 (global flags table),
  §9.8 FR35 (env prefix).** The per-role loop IS FR-R3's env/flag half; `--commits`/`--single`/`--max-commits`
  IS FR-M2; `STAGECOACH_<ROLE>_*` IS FR35's per-role extension.
- **Faithful to the existing load.go design.** The per-role env loop mirrors the existing `STAGECOACH_PROVIDER`/
  `MODEL` handling (presence-semantic, direct set); the per-role flag loop mirrors the existing `provider`/
  `model` handling (Changed-only, silent on bad values); the `setRole*` helpers' map-value-copy mirrors S2's
  overlay() Roles field-merge. This subtask adds NO new pattern — it extends the existing ones.
- **Zero behavior change for v1 env/flags.** A v1 invocation (no v2 env/flags) leaves `cfg.Roles` nil and
  `cfg.Commits`/`Single`/`MaxCommits` at Defaults() — back-compatible by construction.

## What

Modified `internal/config/load.go` (roleNames + 2 helpers + loadEnv/loadFlags/Load extensions), modified
`internal/config/load_test.go` (newFlagSet extension + new tests), and modified `internal/cmd/config.go`
(exampleConfigTemplate header docs). No new files, no new dependency (stdlib `strings` only), no
ResolveRoleModel, no flag registration, no bootstrap rewrite.

### Success Criteria

- [ ] `load.go` defines `var roleNames = []string{"planner", "stager", "message", "arbiter"}` and the two
      unexported methods `func (c *Config) setRoleProvider(role, provider string)` / `setRoleModel(role, model string)`
      (lazy `c.Roles` allocation + map-value-copy write-back).
- [ ] `loadEnv` reads `STAGECOACH_<ROLE>_PROVIDER`/`_MODEL` for each role (via `strings.ToUpper`) into
      `cfg.Roles`, and `STAGECOACH_COMMITS` into `cfg.Commits` (ERROR on non-integer, wrapped as
      `"STAGECOACH_COMMITS: %w"`). load.go imports `"strings"`.
- [ ] `loadFlags` reads `--<role>-provider`/`--<role>-model` (Changed+GetString) into `cfg.Roles`, `--commits`
      (GetInt) into `cfg.Commits`, `--single` OR `--no-decompose` into `cfg.Single=true`, `--max-commits`
      (GetInt) into `cfg.MaxCommits`. No error return (silent, matching existing flag handling).
- [ ] `Load` sets `cfg.Single=true` when `cfg.Commits==1` (after loadFlags), so `STAGECOACH_COMMITS=1` AND
      `--commits 1` both yield Single==true.
- [ ] A per-role field set does NOT clobber its sibling: setting only the Model leaves Provider intact
      (`setRoleModel` on an entry with a Provider preserves it) — pinned by a test.
- [ ] A per-role CLI flag beats a per-role env var (flag > env, FR-R3 precedence) — pinned by a test.
- [ ] `newFlagSet` registers the v2 flags; existing tests remain GREEN (registration is behavior-neutral).
- [ ] `exampleConfigTemplate` header lists the per-role env vars + `STAGECOACH_COMMITS` and a `# CLI flags`
      reference block; every added line is `#`-commented (INERT).
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/` clean.
- [ ] go.mod/go.sum byte-unchanged; config.go (S1) + file.go + git.go + every non-{load.go, load_test.go,
      internal/cmd/config.go} file byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact load.go additions
(quoted verbatim in the Blueprint), the authoritative delta §3 (quoted in research design-decisions.md
§1–§4), the S1 Config fields (the targets: Roles/Commits/Single/MaxCommits/RoleConfig — enumerated), the
existing loadEnv/loadFlags patterns (the templates to extend — read load.go fully), the load_test.go idioms
(newFlagSet + the Changed/Set/GetInt test style), the exampleConfigTemplate header region (the DOCS target),
the S2 disjoint-region coordination note, and the LEAVE list (config.go/file.go/git.go frozen). No decompose/
git/prompt knowledge required — this subtask is env-var reads + flag reads + map helpers + template text.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE load.go spec (matches the item contract verbatim)
- docfile: plan/002_a17bb6c8dc1d/architecture/config_v2_delta.md
  section: "§3. Env/Flag Resolution (load.go)" — the EXACT loadEnv per-role loop (STAGECOACH_<ROLE>_PROVIDER/
           _MODEL via strings.ToUpper), STAGECOACH_COMMITS, and the loadFlags per-role loop + commits/single/
           no-decompose/max-commits. Also §4 (ResolveRoleModel — S2's job, NOT this subtask; read only to
           confirm the cfg.Roles shape S2 reads).
  critical: §3 is the source of truth for the env/flag loops + the setRole* helper names. NOTE this PRP
       OVERRIDES two delta-§3 sketch details for the better: (a) STAGECOACH_COMMITS ERRORS on bad int (delta
       silent-ignores — see design-decisions §2); (b) Commits==1⇒Single normalized in Load() (delta omits it
       — see design-decisions §4). Implement the PRP, not the sketch, for those two.

# MUST READ — the design calls (the non-obvious decisions)
- docfile: plan/002_a17bb6c8dc1d/P1M3T2S1/research/design-decisions.md
  why: the 9 non-obvious calls — scope (load.go+load_test.go+cmd/config.go header; §0), setRole* lazy-alloc +
       map-value-copy (§1), loadEnv per-role + COMMITS-error (§2), loadFlags per-role + single/no-decompose
       aliases (§3), Commits==1⇒Single in Load (§4), flag-registration-is-P4.M1.T1 (§5), the S2 DOCS conflict
       + disjoint-region resolution (§6), roleNames DRY slice (§7), test strategy (§8), no go.mod change (§9).
  critical: §1 (map-value-copy write-back is REQUIRED — easy to get wrong), §2 (COMMITS errors — overrides the
       delta sketch), §4 (the Load normalization — overrides the delta sketch), §6 (don't clobber S2's template
       edits) are the things most likely to go wrong.

# MUST READ — the S1 CONTRACT (the Config fields this subtask WRITES INTO; S1 is SHIPPED, FROZEN)
- file: internal/config/config.go   (S1 — read for the field/type targets; do NOT edit)
  section: `RoleConfig` (struct {Provider string; Model string}); the Config fields `Roles map[string]RoleConfig
           toml:"-"`, `Commits int toml:"-"`, `Single bool toml:"-"`, `MaxCommits int toml:"max_commits"`;
           `Defaults()` sets Commits=0, Single=false, MaxCommits=12, Roles=nil.
  why: this subtask WRITES INTO these fields via load.go. Confirms Roles is `map[string]RoleConfig` (so the
       setRole* helpers' map-value-copy is the correct idiom); confirms Commits/Single are `toml:"-"` runtime-
       only (set ONLY by env/flags here — never a file key); confirms MaxCommits is also a [generation] file
       key (the file layer S2-of-M3.T1 populates it; --max-commits here is the higher layer override).
  critical: do NOT edit config.go. The setRole* helpers are methods on *Config DEFINED IN load.go (a method may
       live in any file of package config).

# THE FILE BEING MODIFIED — READ FULLY before editing
- file: internal/config/load.go
  section: `loadEnv(cfg *Config) error` (presence-semantic os.LookupEnv; DIRECT set for bools/NoColor; wrapped
           errors for bad STAGECOACH_TIMEOUT/VERBOSE/NO_COLOR), `loadFlags(cfg *Config, fs *pflag.FlagSet)` (NO
           error return; Changed-only; silent `if err == nil` guards), `Load()` (Defaults → file/git overlays →
           loadEnv → loadFlags → return).
  why: the EXACT current state this subtask edits. loadEnv returns error (so a bad STAGECOACH_COMMITS can error
       consistently); loadFlags returns nothing (bad flag values are silently skipped). Imports are
       context/fmt/os/strconv/time/pflag — `strings` is NOT yet imported (ADD it for the per-role ToUpper loop).
  critical: mirror the existing presence-semantic (`ok && v != ""`) for the per-role env vars; mirror the
       existing Changed-only + `if err == nil` pattern for the per-role/decompose flags. The Commits==1⇒Single
       normalization goes in Load() AFTER the `if opts.Flags != nil { loadFlags(...) }` block.

# THE TEST FILE BEING MODIFIED — mirror its idioms
- file: internal/config/load_test.go
  section: `newFlagSet(t)` (registers the 5 Config-backed flags), TestLoadEnv_* (t.Setenv + loadEnv(&cfg) +
           assertions), TestLoadFlags_* (fs.Set + loadFlags + assertions), TestLoad_* (LoadOpts + chdir +
           precedence assertions).
  why: the test STYLE — white-box `package config`, t.Setenv for env, fs.Set for flags, one t.Errorf per
       assertion. `newFlagSet` is shared by ~15 tests; EXTEND it (safe) rather than duplicating.
  critical: extending newFlagSet to register the v2 flags is behavior-neutral for existing tests (they don't
       Set them → Changed==false → cfg unchanged → green). Reuse `roleNames` in the test loop (same package).

# THE DOCS FILE BEING MODIFIED — the Mode-A template (HEADER region ONLY)
- file: internal/cmd/config.go
  section: `const exampleConfigTemplate` — the HEADER `# Environment variables (...)` block and the
           `# Git config keys (...)` block (both near the top, BEFORE `# [defaults]`). The [generation] section
           (middle) and the end are S2's regions (do NOT touch).
  why: the template this subtask extends with per-role env vars + STAGECOACH_COMMITS (in the env-vars block) and
       a new `# CLI flags` reference block (after the git-config block). Confirms every option line is
       `#`-commented (INERT) and the header-block style.
  critical: place ALL added lines in the HEADER region (before `# [defaults]`) — DISJOINT from S2's [generation]
       / [role.*] edits. Keep every added line `#`-commented. config_test.go compares the written file to the
       CONSTANT (editing the constant is safe). If a merge conflict with S2 arises, keep BOTH (disjoint regions).

# The PRD basis (already in your context as selected_prd_content)
- file: PRD.md (or plan/002_a17bb6c8dc1d/prd_snapshot.md)
  section: "9.15 Per-role provider/model configuration" (h3.31) — FR-R1 (4 roles), FR-R3 (per-role env/flag
           overrides + precedence flag>env>[role.*]>global>manifest).
  section: "15.2 Global flags" (h3.61) — the exact flag names (--<role>-provider/--<role>-model, --commits,
           --single/--no-decompose, --max-commits) and that --commits 1 ≡ --single.
  section: "9.14 FR-M2" (h3.30) — decompose modes (auto / --commits N / --single≡--no-decompose≡--commits 1).
  section: "16.4" (h3.68) — per-role resolution precedence + that "" ⇒ inherit global.
  critical: §15.2 is the authoritative flag-name table (the per-role flags are `--<role>-provider`/`--<role>-model`).
       §9.14 FR-M2c is the basis for the Commits==1⇒Single normalization. Note §15.2 lists only planner/stager/
       arbiter per-role flags (message inherits global) — but FR-R1 names all FOUR roles; loadEnv/loadFlags loop
       ALL FOUR (a --message-* flag/env is honored if set; the flag REGISTRATION in P4.M1.T1 may omit it, in
       which case Changed==false and it's skipped — harmless). Loop all four (FR-R1); registration is P4.M1.T1's call.
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go        # S1 SHIPPED (FROZEN) — RoleConfig + Roles/Commits/Single/MaxCommits/ConfigVersion. UNCHANGED here.
  config_test.go   # S1 SHIPPED. UNCHANGED here.
  file.go          # S2-of-M3.T1 (parallel) edits this — UNCHANGED here (the FILE layer for Roles/MaxCommits/BinaryExtensions).
  file_test.go     # S2-of-M3.T1. UNCHANGED here.
  load.go          # loadEnv/loadFlags/Load — EDIT (this subtask's core: env/flag layers for the v2 fields)
  load_test.go     # newFlagSet + load/loadEnv/loadFlags tests — EDIT (extend newFlagSet + new tests)
  git.go           # git-config reader — UNCHANGED
internal/cmd/
  config.go        # configCmd + exampleConfigTemplate — EDIT (Mode-A HEADER env/flag docs ONLY)
  config_test.go   # config init/path tests — UNCHANGED (equality test stays green)
go.mod / go.sum    # UNCHANGED (load.go += stdlib "strings" only)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. All edits are in-place modifications to internal/config/load.go + load_test.go + internal/cmd/config.go.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — scope: load.go + load_test.go + internal/cmd/config.go header ONLY): do NOT touch config.go
//   (S1 SHIPPED the fields — this subtask only WRITES INTO them via load.go), file.go/git.go (S2-of-M3.T1 +
//   others own the FILE/git layers), ResolveRoleModel (S2 of M3.T2), or flag registration (P4.M1.T1.S1, root.go).
//   The setRole* helpers are methods on *Config DEFINED IN load.go (legal — any file of package config).
//   (design-decisions §0/§9)

// CRITICAL (#2 — setRole* map-value-copy write-back is REQUIRED): Go maps return a VALUE COPY. You CANNOT do
//   `c.Roles[role].Provider = v` (compile error — can't address a map element), and `rc := c.Roles[role];
//   rc.Provider = v` alone mutates only the local copy (lost). You MUST write back:
//     rc := c.Roles[role]; rc.Provider = v; c.Roles[role] = rc
//   This is why setRoleProvider/setRoleModel exist (and why S2's overlay() does the same existing:=dst.Roles[role]
//   pattern). The write-back is the single most error-prone line. (design-decisions §1)

// CRITICAL (#3 — per-role FIELD-MERGE: setting one field must NOT clobber the sibling): setRoleModel on an
//   entry that already has a Provider (from a lower file layer OR a setRoleProvider call) MUST preserve it.
//   The map-value-copy idiom guarantees this (rc := c.Roles[role] reads the existing Provider; you overwrite
//   only Model; write back the whole struct). This is the env/flag analog of FR-R3/FR37a field-merge. A test
//   pins it (setRoleModel after setRoleProvider → both fields present). NEVER do `c.Roles[role] = RoleConfig{...}`
//   (whole-entry replace drops the sibling). (design-decisions §1/§2)

// CRITICAL (#4 — STAGECOACH_COMMITS ERRORS on non-integer, overriding the delta §3 sketch): loadEnv already
//   errors on bad STAGECOACH_TIMEOUT/VERBOSE/NO_COLOR; error on bad STAGECOACH_COMMITS for consistency
//   (`return fmt.Errorf("STAGECOACH_COMMITS: %w", err)`). The delta's silent-ignore is a sketch, not a contract.
//   (design-decisions §2)

// CRITICAL (#5 — Commits==1 ⇒ Single=true normalized in Load(), overriding the delta §3 sketch): do it AFTER
//   loadFlags so it covers BOTH `STAGECOACH_COMMITS=1` and `--commits 1`. Only sets TRUE. Without it,
//   cfg is inconsistent (Commits==1, Single==false) and downstream must re-derive. (design-decisions §4)

// GOTCHA (flag REGISTRATION is P4.M1.T1.S1; loadFlags is DEFENSIVE): fs.Changed("commits")/("<role>-provider")
//   returns FALSE for an unregistered flag, so the block is skipped — loadFlags is correct/panic-free before
//   P4.M1.T1.S1 wires the flags. Tests register the v2 flags via the extended newFlagSet. (design-decisions §5)

// GOTCHA (loadFlags has NO error return): bad flag values are silently skipped (`if err == nil` guards),
//   matching the existing provider/model/timeout flag handling. --commits/--max-commits are registered as
//   pflag Int (validates at parse time), so GetInt won't fail in practice — the guards are belt-and-suspenders.

// GOTCHA (--single / --no-decompose are ALIASES → Single=true): `if fs.Changed("single") || fs.Changed("no-decompose")
//   { cfg.Single = true }`. Either flag forces the bypass (FR-M2c). --no-decompose is a positive-name bool
//   (passing it = bypass); do NOT invert. Only sets TRUE. (design-decisions §3)

// GOTCHA (loop ALL FOUR roles in loadEnv/loadFlags, even though §15.2 lists only planner/stager/arbiter flags):
//   FR-R1 names all four roles (planner, stager, message, arbiter). Loop roleNames (all four). If P4.M1.T1
//   registers only three per-role flag pairs, Changed("message-provider")==false → skipped (harmless). Env has
//   no registration, so STAGECOACH_MESSAGE_* always works. Looping four is correct + future-proof. (§7)

// GOTCHA (exampleConfigTemplate: HEADER region ONLY, disjoint from S2): S2 (parallel) edits the [generation]
//   section + appends [role.*] at the END. THIS subtask edits the HEADER (env-vars block + new CLI-flags block,
//   before # [defaults]). Disjoint regions — keep both on merge. ALL added lines `#`-commented (INERT).
//   (design-decisions §6)

// GOTCHA (load.go += "strings" import, stdlib only): the per-role env loop uses strings.ToUpper(role). No
//   third-party, no internal/*. `go mod tidy` MUST be a no-op. (design-decisions §9)
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/load.go — NO new types. roleNames (a slice) + two unexported methods on *Config.
// (RoleConfig is already defined in config.go by S1; this subtask only writes into cfg.Roles.)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/load.go — roleNames + setRoleProvider/setRoleModel
  - ADD `var roleNames = []string{"planner", "stager", "message", "arbiter"}` (package-level, unexported).
  - ADD `func (c *Config) setRoleProvider(role, provider string)` and `func (c *Config) setRoleModel(role,
      model string)` — each: `if c.Roles == nil { c.Roles = make(map[string]RoleConfig) }; rc := c.Roles[role];
      rc.Provider = provider; c.Roles[role] = rc` (write-back REQUIRED).
  - GOTCHA: map-value-copy write-back (can't address map element; can't mutate the local copy alone).
  - GOTCHA: define these in load.go (NOT config.go, which is frozen).

Task 2: EDIT internal/config/load.go — loadEnv() per-role loop + STAGECOACH_COMMITS
  - ADD `"strings"` to the import block.
  - APPEND (before `return nil`): the per-role loop over roleNames — `os.LookupEnv("STAGECOACH_" +
      strings.ToUpper(role) + "_PROVIDER"/"_MODEL")` with presence-semantic (`ok && v != ""`) → setRoleProvider/
      setRoleModel; then STAGECOACH_COMMITS (`strconv.Atoi`; on error `return fmt.Errorf("STAGECOACH_COMMITS: %w", err)`).
  - GOTCHA: COMMITS errors (NOT silent) — overrides delta §3 sketch for consistency with STAGECOACH_TIMEOUT.

Task 3: EDIT internal/config/load.go — loadFlags() per-role loop + decompose flags
  - APPEND (before the closing brace): the per-role loop over roleNames — `fs.Changed(role+"-provider"/+"-model")`
      + `fs.GetString` → setRoleProvider/setRoleModel; then `--commits`/`--max-commits` (GetInt);
      `--single`/`--no-decompose` (`||` → `cfg.Single = true`).
  - GOTCHA: NO error return (silent `if err == nil` guards, matching existing flag handling).
  - GOTCHA: --single/--no-decompose are aliases → Single=true (do NOT invert --no-decompose).

Task 4: EDIT internal/config/load.go — Load() Commits==1⇒Single normalization
  - ADD (after the `if opts.Flags != nil { loadFlags(&cfg, opts.Flags) }` block, before `return &cfg, nil`):
      `if cfg.Commits == 1 { cfg.Single = true }` + a comment citing FR-M2c/§15.2.
  - GOTCHA: after loadFlags so it covers BOTH env and flag sources. Only sets TRUE.

Task 5: EDIT internal/config/load_test.go — extend newFlagSet + new tests
  - EXTEND `newFlagSet`: register the 8 per-role flags (roleNames loop, String "") + commits (Int 0) +
      single (Bool false) + no-decompose (Bool false) + max-commits (Int 0).
  - ADD: TestLoadEnv_PerRole, TestLoadEnv_PerRolePartial, TestLoadEnv_Commits, TestLoadEnv_CommitsBadIntErrors,
      TestLoadFlags_PerRole, TestLoadFlags_Decompose, TestLoad_CommitsOneNormalizesSingle (env AND flag paths),
      TestLoad_PerRoleFlagBeatsEnv, TestSetRole_LazyAllocAndFieldMerge.
  - GOTCHA: white-box `package config`; reuse roleNames + t.Setenv/fs.Set; one t.Errorf per assertion.
  - GOTCHA: extending newFlagSet is behavior-neutral for the existing ~15 tests (they stay green).

Task 6: EDIT internal/cmd/config.go — exampleConfigTemplate HEADER env/flag docs (Mode A)
  - EXTEND the `# Environment variables (...)` header block: add `# STAGECOACH_PLANNER_PROVIDER/_MODEL`,
      `# STAGECOACH_STAGER_PROVIDER/_MODEL`, `# STAGECOACH_MESSAGE_PROVIDER/_MODEL`, `# STAGECOACH_ARBITER_PROVIDER/_MODEL`,
      and `# STAGECOACH_COMMITS`.
  - ADD a new `# CLI flags (...)` reference block (after the `# Git config keys` block, before `# [defaults]`):
      --provider/--model (global), --<role>-provider/--<role>-model (per-role), --commits, --single/--no-decompose,
      --max-commits.
  - GOTCHA: HEADER region ONLY (disjoint from S2's [generation]/[role.*]); ALL lines `#`-commented (INERT).

Task 7: VERIFY (no further edits)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. config.go (S1) + file.go +
      git.go + every non-target file byte-unchanged. `go build ./... && go test ./...` green.
```

### Implementation Patterns & Key Details

```go
// THE setRole* helpers (map-value-copy write-back is REQUIRED — the load-bearing idiom):
func (c *Config) setRoleProvider(role, provider string) {
	if c.Roles == nil {
		c.Roles = make(map[string]RoleConfig)
	}
	rc := c.Roles[role] // value copy (zero value if absent — fine: "" = inherit global)
	rc.Provider = provider
	c.Roles[role] = rc // WRITE-BACK required (mutating the local copy alone is lost)
}

func (c *Config) setRoleModel(role, model string) {
	if c.Roles == nil {
		c.Roles = make(map[string]RoleConfig)
	}
	rc := c.Roles[role]
	rc.Model = model
	c.Roles[role] = rc
}

// roleNames — the canonical four roles (PRD §13.6.2 / §9.15 FR-R1). One source for loadEnv/loadFlags/tests.
var roleNames = []string{"planner", "stager", "message", "arbiter"}

// loadEnv additions (presence-semantic; COMMITS errors on bad int):
for _, role := range roleNames {
	if v, ok := os.LookupEnv("STAGECOACH_" + strings.ToUpper(role) + "_PROVIDER"); ok && v != "" {
		cfg.setRoleProvider(role, v)
	}
	if v, ok := os.LookupEnv("STAGECOACH_" + strings.ToUpper(role) + "_MODEL"); ok && v != "" {
		cfg.setRoleModel(role, v)
	}
}
if v, ok := os.LookupEnv("STAGECOACH_COMMITS"); ok && v != "" {
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("STAGECOACH_COMMITS: %w", err)
	}
	cfg.Commits = n
}

// loadFlags additions (Changed-only; silent on bad values; single/no-decompose are aliases):
for _, role := range roleNames {
	if fs.Changed(role + "-provider") {
		if v, err := fs.GetString(role + "-provider"); err == nil {
			cfg.setRoleProvider(role, v)
		}
	}
	if fs.Changed(role + "-model") {
		if v, err := fs.GetString(role + "-model"); err == nil {
			cfg.setRoleModel(role, v)
		}
	}
}
if fs.Changed("commits") {
	if v, err := fs.GetInt("commits"); err == nil {
		cfg.Commits = v
	}
}
if fs.Changed("single") || fs.Changed("no-decompose") {
	cfg.Single = true
}
if fs.Changed("max-commits") {
	if v, err := fs.GetInt("max-commits"); err == nil {
		cfg.MaxCommits = v
	}
}

// Load() normalization (after loadFlags; covers env AND flag sources):
if cfg.Commits == 1 {
	cfg.Single = true // FR-M2c / §15.2: --commits 1 ≡ --single
}
```

```go
// === internal/config/load_test.go — newFlagSet extension + representative new tests ===

// EXTEND newFlagSet (behavior-neutral for existing tests):
func newFlagSet(t *testing.T) *pflag.FlagSet {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("provider", "", "")
	fs.String("model", "", "")
	fs.String("timeout", "", "")
	fs.Bool("verbose", false, "")
	fs.Bool("no-color", false, "")
	for _, role := range roleNames { // V2 per-role flags
		fs.String(role+"-provider", "", "")
		fs.String(role+"-model", "", "")
	}
	fs.Int("commits", 0, "")      // V2 decompose
	fs.Bool("single", false, "")
	fs.Bool("no-decompose", false, "")
	fs.Int("max-commits", 0, "")
	return fs
}

// TestLoadEnv_PerRole — per-role env vars populate cfg.Roles (field-level).
func TestLoadEnv_PerRole(t *testing.T) {
	cfg := Defaults()
	t.Setenv("STAGECOACH_PLANNER_PROVIDER", "agy")
	t.Setenv("STAGECOACH_PLANNER_MODEL", "gemini-2.5-pro")
	t.Setenv("STAGECOACH_STAGER_MODEL", "gemini-2.5-flash")
	if err := loadEnv(&cfg); err != nil {
		t.Fatalf("loadEnv err=%v", err)
	}
	if rc := cfg.Roles["planner"]; rc.Provider != "agy" || rc.Model != "gemini-2.5-pro" {
		t.Errorf("Roles[planner]=%+v want {agy gemini-2.5-pro}", rc)
	}
	if rc := cfg.Roles["stager"]; rc.Provider != "" || rc.Model != "gemini-2.5-flash" {
		t.Errorf("Roles[stager]=%+v want {\"\" gemini-2.5-flash} (partial — field-level)", rc)
	}
}

// TestLoadEnv_CommitsBadIntErrors — COMMITS errors on non-integer (consistent with STAGECOACH_TIMEOUT).
func TestLoadEnv_CommitsBadIntErrors(t *testing.T) {
	cfg := Defaults()
	t.Setenv("STAGECOACH_COMMITS", "abc")
	err := loadEnv(&cfg)
	if err == nil {
		t.Fatal("loadEnv err=nil, want error for bad STAGECOACH_COMMITS")
	}
	if !strings.Contains(err.Error(), "STAGECOACH_COMMITS") {
		t.Errorf("err=%v, want it to contain 'STAGECOACH_COMMITS'", err)
	}
}

// TestLoadFlags_PerRole — per-role flags populate cfg.Roles (Changed-only).
func TestLoadFlags_PerRole(t *testing.T) {
	cfg := Defaults()
	fs := newFlagSet(t)
	if err := fs.Set("planner-provider", "agy"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Set("planner-model", "gemini-2.5-pro"); err != nil {
		t.Fatal(err)
	}
	loadFlags(&cfg, fs)
	if rc := cfg.Roles["planner"]; rc.Provider != "agy" || rc.Model != "gemini-2.5-pro" {
		t.Errorf("Roles[planner]=%+v want {agy gemini-2.5-pro}", rc)
	}
}

// TestLoadFlags_Decompose — commits/single/no-decompose/max-commits flags.
func TestLoadFlags_Decompose(t *testing.T) {
	cfg := Defaults()
	fs := newFlagSet(t)
	fs.Set("commits", "3")
	fs.Set("max-commits", "8")
	loadFlags(&cfg, fs)
	if cfg.Commits != 3 {
		t.Errorf("Commits=%d want 3", cfg.Commits)
	}
	if cfg.MaxCommits != 8 {
		t.Errorf("MaxCommits=%d want 8", cfg.MaxCommits)
	}
	if cfg.Single {
		t.Errorf("Single=true want false (no single/no-decompose set)")
	}

	// --no-decompose alias → Single=true
	cfg2 := Defaults()
	fs2 := newFlagSet(t)
	fs2.Set("no-decompose", "true")
	loadFlags(&cfg2, fs2)
	if !cfg2.Single {
		t.Errorf("Single=false want true (--no-decompose alias)")
	}
}

// TestLoad_CommitsOneNormalizesSingle — STAGECOACH_COMMITS=1 AND --commits 1 both → cfg.Single==true (the §4
// normalization in Load, after env+flags).
func TestLoad_CommitsOneNormalizesSingle(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)

	// env path
	t.Setenv("STAGECOACH_COMMITS", "1")
	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Commits != 1 {
		t.Errorf("Commits=%d want 1", cfg.Commits)
	}
	if !cfg.Single {
		t.Errorf("Single=false want true (STAGECOACH_COMMITS=1 must normalize to Single)")
	}

	// flag path
	os.Unsetenv("STAGECOACH_COMMITS")
	fs := newFlagSet(t)
	fs.Set("commits", "1")
	cfg2, err := Load(context.Background(), LoadOpts{RepoDir: repo, Flags: fs})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if !cfg2.Single {
		t.Errorf("Single=false want true (--commits 1 must normalize to Single)")
	}
}

// TestLoad_PerRoleFlagBeatsEnv — flag > env for a per-role override (FR-R3 precedence).
func TestLoad_PerRoleFlagBeatsEnv(t *testing.T) {
	_, repo, _ := loadEnvSetup(t)
	chdir(t, repo)
	t.Setenv("STAGECOACH_PLANNER_MODEL", "env-model")
	fs := newFlagSet(t)
	fs.Set("planner-model", "flag-model")
	cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo, Flags: fs})
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if rc := cfg.Roles["planner"]; rc.Model != "flag-model" {
		t.Errorf("Roles[planner].Model=%q want flag-model (flag > env)", rc.Model)
	}
}

// TestSetRole_LazyAllocAndFieldMerge — setRole* lazily allocates and field-merges (map-value-copy correctness).
func TestSetRole_LazyAllocAndFieldMerge(t *testing.T) {
	cfg := Config{} // Roles == nil
	cfg.setRoleProvider("planner", "agy")
	if cfg.Roles == nil || cfg.Roles["planner"].Provider != "agy" {
		t.Fatalf("setRoleProvider did not lazily alloc + set: %+v", cfg.Roles)
	}
	cfg.setRoleModel("planner", "gemini-2.5-pro") // must NOT erase Provider
	rc := cfg.Roles["planner"]
	if rc.Provider != "agy" || rc.Model != "gemini-2.5-pro" {
		t.Errorf("Roles[planner]=%+v want {agy gemini-2.5-pro} (field-merge: Model must not erase Provider)", rc)
	}
}
```

```go
// === internal/cmd/config.go — exampleConfigTemplate HEADER additions (Mode A). Insert the per-role env-var
//     lines at the END of the existing `# Environment variables (...)` block (before the blank line that
//     precedes `# Git config keys`); insert the `# CLI flags (...)` block AFTER the `# Git config keys (...)`
//     block and BEFORE `# [defaults]`. EVERY added line is # -commented (INERT). HEADER region ONLY — do NOT
//     touch the [generation] section or append anything at the end (those are S2's regions). ===

// (1) Append to the `# Environment variables (...)` header block (after the existing STAGECOACH_NO_COLOR line):
#   STAGECOACH_PLANNER_PROVIDER / _MODEL   per-role override: decomposition planner (PRD §16.4, §9.15)
#   STAGECOACH_STAGER_PROVIDER  / _MODEL   per-role override: (tooled) staging agent
#   STAGECOACH_MESSAGE_PROVIDER / _MODEL   per-role override: bare commit-message agent
#   STAGECOACH_ARBITER_PROVIDER / _MODEL   per-role override: leftover arbiter
#   STAGECOACH_COMMITS                    force exactly N commits when nothing is staged (PRD §9.14); 1 == --single

// (2) NEW block, placed AFTER the `# Git config keys (...)` block and BEFORE the `# [defaults]` section:
# ---------------------------------------------------------------------------
# CLI flags (PRD §15.2) — highest precedence; only an EXPLICITLY-passed flag overrides lower layers
# ---------------------------------------------------------------------------
# --provider / --model                       global default for ALL roles (§16.4)
# --<role>-provider / --<role>-model         per-role override (role = planner|stager|message|arbiter)
# --commits <N>                              force exactly N commits (N>=2); --commits 1 == --single (§9.14)
# --single / --no-decompose                  bypass decomposition; force the single-commit path (§9.14)
# --max-commits <N>                          safety cap on auto-decompose (default 12; §9.14 FR-M4)
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): change NONE. load.go += stdlib "strings" only (for strings.ToUpper in the per-role
      env loop). No third-party, no internal/*. `go mod tidy` MUST be a no-op. `git diff --exit-code go.mod
      go.sum` MUST be empty.

PACKAGE EDGES: NONE added. config stays a leaf (RoleConfig is plain; load.go already imports os/strconv/fmt/
      pflag; +"strings"). cmd already imports config (the template edit is a string change).

UPSTREAM CONTRACT (the inputs — do NOT implement, just consume):
  - S1 (config.go, SHIPPED): RoleConfig{Provider,Model}, Config.Roles/Commits/Single/MaxCommits. This subtask
        WRITES INTO them via the setRole* helpers (methods defined in load.go) and direct field sets.
  - P4.M1.T1.S1 (root.go, FUTURE): REGISTERS the --<role>-*/--commits/--single/--no-decompose/--max-commits
        flags. loadFlags is DEFENSIVE (unregistered flag → Changed==false → skipped), so it works before and
        after P4.M1.T1.S1 with no hard dependency.

DOWNSTREAM CONTRACTS (the consumers — do NOT implement here, just honor the cfg shape):
  - P1.M3.T2.S2 (ResolveRoleModel): reads cfg.Roles[role] (now populated by file layer S2-of-M3.T1 AND these
        env/flag layers — the env/flag layers are the HIGHER precedence, written directly into cfg.Roles) then
        falls back to cfg.Provider/cfg.Model. Because loadEnv/loadFlags write DIRECTLY into cfg.Roles, the
        flag>env>[role.*]>global precedence holds (the orchestrator loads env THEN flags, so flags overwrite).
  - P3.M2/M4 (decompose orchestrator): reads cfg.Single (bypass), cfg.Commits (forced count), cfg.MaxCommits
        (cap). This subtask makes the env/flags populate them and normalizes Commits==1⇒Single so cfg is
        self-consistent (the orchestrator need not re-derive).

FROZEN/LEAVE (do NOT edit):
  - internal/config/config.go (+_test.go) — S1 SHIPPED.
  - internal/config/file.go (+_test.go), git.go (+_test.go) — S2-of-M3.T1 (parallel) + others.
  - internal/provider/*, internal/git/*, internal/prompt/*, internal/generate/*, internal/ui/*,
    internal/cmd/root.go (+_test.go), internal/cmd/config_test.go, pkg/*, cmd/*.
  - PRD.md, Makefile, providers/*.toml, docs/*.

NO NEW DATABASE / ROUTES / CLI COMMANDS / FILES.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/config/load.go internal/config/load_test.go internal/cmd/config.go
go vet ./internal/config/ ./internal/cmd/
# Confirm load.go gained the "strings" import + the new artifacts:
grep -n '"strings"' internal/config/load.go
grep -n 'roleNames\|setRoleProvider\|setRoleModel\|STAGECOACH_COMMITS\|cfg.Commits == 1' internal/config/load.go
# Confirm go.mod/go.sum unchanged:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; "strings" imported; roleNames/setRole*/COMMITS/normalization present; go.mod/go.sum byte-unchanged.
```

### Level 2: Config-package unit tests (the new tests + no regression)

```bash
go test ./internal/config/ -v
# Expected PASS — verify explicitly:
#   TestLoadEnv_PerRole ................ per-role STAGECOACH_<ROLE>_* populate cfg.Roles (field-level)
#   TestLoadEnv_PerRolePartial ......... a model-only env var leaves Provider "" (field-merge)
#   TestLoadEnv_Commits ................ STAGECOACH_COMMITS=3 → cfg.Commits=3
#   TestLoadEnv_CommitsBadIntErrors .... STAGECOACH_COMMITS=abc → wrapped "STAGECOACH_COMMITS" error
#   TestLoadFlags_PerRole .............. --<role>-provider/--<role>-model populate cfg.Roles
#   TestLoadFlags_Decompose ............ --commits/--max-commits (Int) + --no-decompose → Single
#   TestLoad_CommitsOneNormalizesSingle  STAGECOACH_COMMITS=1 AND --commits 1 → cfg.Single==true
#   TestLoad_PerRoleFlagBeatsEnv ....... per-role flag > per-role env (FR-R3 precedence)
#   TestSetRole_LazyAllocAndFieldMerge . setRole* lazy-alloc + Model does NOT erase Provider (map-value-copy)
#   TestLoadEnv_* / TestLoadFlags_* / TestLoad_* (the ~25 existing tests) ... STILL GREEN (newFlagSet extension
#       is behavior-neutral — they don't Set the v2 flags).
# If TestSetRole_LazyAllocAndFieldMerge fails on "Provider erased", the helper does a whole-entry replace —
# use the map-value-copy write-back (rc := c.Roles[role]; rc.X=v; c.Roles[role]=rc).
# If TestLoad_CommitsOneNormalizesSingle fails, the Load() normalization is missing/misplaced (must be AFTER
# loadFlags).
```

### Level 3: Whole-repo build/test + cmd-package tests + frozen-file check

```bash
go build ./...     # Expect clean (this subtask only writes into S1's fields).
go test ./...      # Expect all PASS — incl. internal/cmd (the exampleConfigTemplate equality test stays green
                   # because it compares the written file to the constant; the template is still all-commented).
# Confirm the LEAVE files are byte-unchanged:
git diff --exit-code internal/config/config.go internal/config/config_test.go internal/config/file.go \
  internal/config/file_test.go internal/config/git.go internal/config/git_test.go \
  internal/provider internal/git internal/prompt internal/generate internal/ui \
  internal/cmd/root.go internal/cmd/config_test.go cmd pkg Makefile go.mod go.sum PRD.md \
  && echo "frozen files UNCHANGED (expected)"
# Confirm ONLY the three target files changed:
git diff --name-only | grep -E 'internal/config/load\.go|internal/config/load_test\.go|internal/cmd/config\.go' \
  && echo "target files modified (expected)"
```

### Level 4: Correctness reasoning (no runtime to start)

```bash
# This subtask is env-var reads + flag reads + map helpers + a one-line normalization + template text — no
# server/DB/subprocess. Verify by reasoning + the tests:
#   1. loadEnv: STAGECOACH_<ROLE>_PROVIDER/_MODEL (4 roles, strings.ToUpper) → cfg.Roles via setRole*;
#      STAGECOACH_COMMITS → cfg.Commits (error on bad int). (TestLoadEnv_PerRole/Commits/CommitsBadIntErrors)
#   2. loadFlags: --<role>-provider/--<role>-model (Changed+GetString) → cfg.Roles; --commits/--max-commits
#      (GetInt); --single/--no-decompose → Single=true. (TestLoadFlags_PerRole/Decompose)
#   3. Field-merge: setRoleModel does NOT erase a Provider (map-value-copy). (TestSetRole_LazyAllocAndFieldMerge)
#   4. Precedence: per-role flag > per-role env (Load runs env THEN flags). (TestLoad_PerRoleFlagBeatsEnv)
#   5. Normalization: Commits==1 ⇒ Single=true in Load, for BOTH env and flag sources.
#      (TestLoad_CommitsOneNormalizesSingle)
#   6. Template: header env-vars block lists the per-role vars + COMMITS; new CLI-flags block lists the flags;
#      all # -commented; disjoint from S2's regions; cmd equality test green.
# (No Level-4 commands beyond Levels 1–3 — the tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l internal/` clean.
- [ ] `go test ./...` PASS (config suite incl. the new tests + no repo-wide regression; cmd suite green).
- [ ] go.mod/go.sum byte-unchanged.
- [ ] config.go (S1) + file.go + git.go (+tests) + every non-target file byte-unchanged; PRD.md byte-unchanged.

### Feature Validation
- [ ] `loadEnv` reads `STAGECOACH_<ROLE>_PROVIDER/_MODEL` (4 roles) → `cfg.Roles`; `STAGECOACH_COMMITS` →
      `cfg.Commits` (error on bad int).
- [ ] `loadFlags` reads `--<role>-provider/--<role>-model` → `cfg.Roles`; `--commits`/`--max-commits` →
      `cfg.Commits`/`cfg.MaxCommits`; `--single`/`--no-decompose` → `cfg.Single=true`.
- [ ] `Load` normalizes `Commits==1 ⇒ Single=true` (env AND flag sources).
- [ ] Per-role field-merge: setting Model does NOT erase Provider (map-value-copy).
- [ ] Per-role flag beats per-role env (FR-R3 precedence).
- [ ] `exampleConfigTemplate` header lists per-role env vars + COMMITS + a CLI-flags block; all `#`-commented.
- [ ] The new tests pass; the ~25 existing load tests stay green.

### Code Quality Validation
- [ ] Follows existing conventions: presence-semantic loadEnv; Changed-only loadFlags; wrapped errors for bad
      env values; map-value-copy helpers (mirror S2's overlay()); white-box `package config` tests mirroring
      TestLoadEnv_*/TestLoadFlags_*/TestLoad_*; commented-everywhere template.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (LEAVE files untouched).

### Documentation
- [ ] load.go doc comments cite PRD §9.15 FR-R3 / §9.14 FR-M2 / §15.2 / §9.8 FR35 and the FRs; the setRole*
      helpers document the map-value-copy requirement; the Load normalization cites FR-M2c/§15.2.
- [ ] The exampleConfigTemplate header sections document the per-role env vars + decompose env/flags (Mode A —
      the template IS the user-facing config reference until P1.M4.T2 rewrites it to a populated bootstrap).

---

## Anti-Patterns to Avoid

- ❌ **Don't touch config.go / file.go / git.go.** config.go is S1 (SHIPPED — this subtask only WRITES INTO its
  fields via load.go); file.go is S2-of-M3.T1 (the FILE layer); git.go is untouched. The setRole* helpers are
  methods on *Config DEFINED IN load.go (legal in any file of package config). ResolveRoleModel is S2 of M3.T2;
  flag REGISTRATION is P4.M1.T1.S1 (root.go). (design-decisions §0)
- ❌ **Don't skip the map-value-copy write-back in setRole*.** `rc := c.Roles[role]; rc.Provider = v` alone
  mutates a LOCAL COPY (lost). You MUST `c.Roles[role] = rc`. Never `c.Roles[role] = RoleConfig{...}` (whole-
  entry replace drops the sibling field — violates FR-R3 field-merge). (§1/§3)
- ❌ **Don't silently ignore a bad STAGECOACH_COMMITS.** Error on non-integer (consistent with STAGECOACH_TIMEOUT)
  — overrides the delta §3 sketch. (§2)
- ❌ **Don't forget the Commits==1⇒Single normalization in Load().** Put it AFTER loadFlags so it covers env AND
  flags. Without it cfg is inconsistent. (§4) — overrides the delta §3 sketch.
- ❌ **Don't register the flags in load.go.** Flag registration is P4.M1.T1.S1 (root.go); loadFlags only READS
  them (defensively — unregistered ⇒ Changed==false ⇒ skipped). (§5)
- ❌ **Don't invert `--no-decompose`.** It's a positive-name bool: passing it = bypass = Single=true. `--single`
  and `--no-decompose` are aliases (`||`). Only set TRUE. (§3)
- ❌ **Don't edit exampleConfigTemplate's [generation] section or append at the end.** Those are S2's (parallel)
  regions. Place your additions in the HEADER region (env-vars block + new CLI-flags block), disjoint from S2.
  Keep all lines `#`-commented. (§6)
- ❌ **Don't add a third-party import.** load.go += stdlib "strings" only. `go mod tidy` must be a no-op. (§9)
- ❌ **Don't loop only 3 roles.** FR-R1 names FOUR (planner, stager, message, arbiter). Loop `roleNames` (all
  four) in both loadEnv and loadFlags — a --message-* flag/env is honored if set; if P4.M1.T1 omits registering
  it, Changed==false ⇒ skipped (harmless). (§7)
