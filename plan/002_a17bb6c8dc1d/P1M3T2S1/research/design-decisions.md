# P1.M3.T2.S1 — Design Decisions

**Subtask**: Add per-role + decompose env vars to `loadEnv()` and flag handling to `loadFlags()`
**Files**: `internal/config/load.go` (EDIT) + `internal/config/load_test.go` (EDIT) + `internal/cmd/config.go`
(EDIT — header env/flag docs only). `internal/config/config.go` is FROZEN (S1 shipped it).
**PRD**: §9.15 FR-R1–R5 (per-role), §16.4, §9.14 FR-M2 (decompose flags), §15.2 (global flags table), §9.8 FR35 (env).
**Authoritative spec**: architecture/config_v2_delta.md §3 (Env/Flag Resolution) — quoted & refined below.
**Scope**: the ENV (layer 5) and CLI-flag (layer 7) layers for the v2 fields. `ResolveRoleModel` is S2
(P1.M3.T2.S2); the file/git layers are S2 of M3.T1; flag REGISTRATION is P4.M1.T1.S1 (root.go).

---

## §0 — The boundary: load.go + load_test.go + cmd/config.go header docs

This subtask makes the ENV and CLI-FLAG layers populate the v2 `Config` fields S1 declared. Concretely:

- `loadEnv()`: read `STAGECOACH_<ROLE>_PROVIDER`/`_MODEL` (4 roles) → `cfg.Roles`; `STAGECOACH_COMMITS` → `cfg.Commits`.
- `loadFlags()`: read `--<role>-provider`/`--<role>-model` (4 roles) → `cfg.Roles`; `--commits` → `cfg.Commits`;
  `--single`/`--no-decompose` → `cfg.Single`; `--max-commits` → `cfg.MaxCommits`.
- TWO helper methods (`setRoleProvider`/`setRoleModel`) + a `roleNames` slice, all in load.go (NOT config.go).
- ONE consistency normalization in `Load()`: `Commits==1 ⇒ Single=true` (FR-M2c/§15.2).
- `load_test.go`: extend `newFlagSet` + new tests.
- `internal/cmd/config.go`: extend the HEADER env/flag docs of `exampleConfigTemplate` (Mode A).

NOT this subtask: `ResolveRoleModel` (S2), the file/git layers (S2 of M3.T1), flag registration (P4.M1.T1),
the bootstrap `config init` rewrite (P1.M4.T2), the config_version advisory (P1.M4.T1). Do NOT implement them.

## §1 — setRoleProvider / setRoleModel: lazily-allocate + map-value-copy

The delta §3 names these helpers (the work-item allows `setRole(role,provider,model)` or inline). Use the
two-field form (`setRoleProvider`, `setRoleModel`) — it matches delta §3 exactly and reads cleanly in the
per-field env/flag loops (you set ONE field per env var / flag). Define them in load.go (methods on
`*Config` may live in any file of `package config`; config.go stays frozen).

```go
func (c *Config) setRoleProvider(role, provider string) {
	if c.Roles == nil {
		c.Roles = make(map[string]RoleConfig)
	}
	rc := c.Roles[role]   // map returns a VALUE COPY
	rc.Provider = provider
	c.Roles[role] = rc     // write back — required (assigning to the copy's field would be lost)
}
```

CRITICAL — the map-value-copy idiom: Go maps return a COPY of the value type. `c.Roles[role].Provider = v`
would not compile (can't address a map element); and even `rc := c.Roles[role]; rc.Provider = v` alone
mutates only the local copy. You MUST write `c.Roles[role] = rc` back. This is the same reason S2's
overlay() uses `existing := dst.Roles[role]; ...; dst.Roles[role] = existing`. Mirror it.

Lazy allocation: `if c.Roles == nil { c.Roles = make(...) }` in each helper. Defaults() leaves Roles nil
(FR-R2: "no per-role overrides → all roles use the global"); the FIRST per-role env/flag/file to fire
allocates it. Unexported (lowercase) — internal to the config package.

## §2 — loadEnv: per-role loop + STAGECOACH_COMMITS (NEW import: "strings")

Append to loadEnv(), before `return nil`. The per-role loop uses `strings.ToUpper(role)` to build the env
var name (`STAGECOACH_PLANNER_PROVIDER`, …). load.go currently imports context/fmt/os/strconv/time/pflag —
NOT `strings`. ADD `"strings"` (stdlib, go.mod-neutral). Presence-semantic (`ok && v != ""`) matches the
existing STAGECOACH_PROVIDER/MODEL handling.

```go
for _, role := range roleNames {
	if v, ok := os.LookupEnv("STAGECOACH_"+strings.ToUpper(role)+"_PROVIDER", ); ok && v != "" {
		cfg.setRoleProvider(role, v)
	}
	if v, ok := os.LookupEnv("STAGECOACH_" + strings.ToUpper(role) + "_MODEL"); ok && v != "" {
		cfg.setRoleModel(role, v)
	}
}
if v, ok := os.LookupEnv("STAGECOACH_COMMITS"); ok && v != "" {
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("STAGECOACH_COMMITS: %w", err)   // fail-at-load, like STAGECOACH_TIMEOUT
	}
	cfg.Commits = n
}
```

DECISION — STAGECOACH_COMMITS errors on a non-integer (delta §3 shows silent-ignore). Rationale: loadEnv
ALREADY errors on a bad STAGECOACH_TIMEOUT (parseTimeout) and a bad STAGECOACH_VERBOSE/NO_COLOR (ParseBool).
Erroring on a bad STAGECOACH_COMMITS is CONSISTENT (fail-at-load with a wrapped, layer-named error) and
catches typos early. The delta's silent-ignore is a sketch, not a contract; the work-item just says
"handle STAGECOACH_COMMITS (int)". A wrapped error is the better, consistent choice. (A test pins the error.)

## §3 — loadFlags: per-role loop + commits/single/no-decompose/max-commits (NO new import)

Append to loadFlags(), before the closing brace. loadFlags has NO error return (it is
`func loadFlags(cfg *Config, fs *pflag.FlagSet)`); bad values are silently skipped (the `if err == nil`
guards), matching the existing provider/model/timeout flag handling. Only `fs.Changed(name)` flags override
(an unset flag's default value is NOT an override) — the established Changed-only semantics.

```go
for _, role := range roleNames {
	if fs.Changed(role + "-provider") {
		if v, err := fs.GetString(role + "-provider"); err == nil { cfg.setRoleProvider(role, v) }
	}
	if fs.Changed(role + "-model") {
		if v, err := fs.GetString(role + "-model"); err == nil { cfg.setRoleModel(role, v) }
	}
}
if fs.Changed("commits") {
	if v, err := fs.GetInt("commits"); err == nil { cfg.Commits = v }
}
if fs.Changed("single") || fs.Changed("no-decompose") {
	cfg.Single = true   // FR-M2c: --single and --no-decompose are aliases; either forces the bypass
}
if fs.Changed("max-commits") {
	if v, err := fs.GetInt("max-commits"); err == nil { cfg.MaxCommits = v }
}
```

DECISION — `--single` / `--no-decompose` BOTH set `cfg.Single = true` (they are aliases, PRD §15.2/§9.14
FR-M2c). `Changed("single") || Changed("no-decompose")` — either flag, if explicitly passed, forces the
bypass. Only sets TRUE (a user cannot pass `--single=false` to "un-bypass"; that's not a meaningful
operation — the bypass is opt-in). Note `--no-decompose` is a positive-name bool (passing it = true =
bypass); do NOT invert.

## §4 — The Commits==1 ⇒ Single normalization (in Load, after flags)

PRD §9.14 FR-M2c + §15.2: "`--commits 1` ≡ `--single`" — a forced count of 1 IS the single-commit escape
hatch. The delta §3 does NOT normalize this; but leaving `cfg.Commits==1, cfg.Single==false` is an
INCONSISTENT state that a downstream consumer (the decompose orchestrator, P3.M4) would have to re-derive.
Normalize in `Load()` AFTER both env and flags so it covers BOTH sources (`STAGECOACH_COMMITS=1` AND
`--commits 1`):

```go
// after: if opts.Flags != nil { loadFlags(&cfg, opts.Flags) }
if cfg.Commits == 1 {
	cfg.Single = true   // FR-M2c/§15.2: commits 1 == single, regardless of source
}
```

Only sets TRUE (never forces false) → preserves the escape-hatch philosophy. One place, both sources, cfg
self-consistent. The orchestrator MAY also defensively check `cfg.Single || cfg.Commits==1`, but it does
not HAVE to — cfg is guaranteed consistent. (A test pins `STAGECOACH_COMMITS=1 → cfg.Single==true` via Load.)

## §5 — Coordination: flag REGISTRATION is P4.M1.T1.S1 (loadFlags is defensive)

loadFlags reads `fs.Changed("<role>-provider")`, `fs.Changed("commits")`, etc. These flags must be
REGISTERED on the FlagSet (in root.go, P4.M1.T1.S1) for `Changed`/`GetString`/`GetInt` to return meaningful
values. BUT loadFlags is DEFENSIVE: pflag's `fs.Changed("unknownflag")` returns FALSE for an unregistered
flag, so the block is simply skipped — loadFlags is correct and panic-free BEFORE P4.M1.T1.S1 wires the
flags (it just won't find them) AND after. This is the SAME relationship the existing loadFlags has with
provider/model/timeout/verbose/no-color (also registered by the CLI layer). No hard dependency; no stubbing.

For TESTS: the test FlagSet must register the v2 flags so `Changed`/`Set`/`GetInt` work → extend `newFlagSet`
(§8). load_test.go is entirely this subtask's (S2 of M3.T1 does NOT touch it).

## §6 — DOCS conflict with S2 (parallel): disjoint regions in exampleConfigTemplate

BOTH this subtask AND S2 (P1.M3.T1.S2, implementing NOW) edit `internal/cmd/config.go`'s `exampleConfigTemplate`
constant. S2 edits: (a) the [generation] section (adds `# max_commits` / `# binary_extensions`), (b) APPENDS
a `# [role.*]` TOML-config section at the END of the template. THIS subtask edits the HEADER region:
extends the existing `# Environment variables` block (adds `# STAGECOACH_<ROLE>_*` + `# STAGECOACH_COMMITS`)
and adds a new `# CLI flags` reference block — both placed in the header, BEFORE `# [defaults]`.

These regions are DISJOINT (header vs [generation]-middle vs [role.*]-end), so a 3-way git merge should
resolve cleanly. IF a conflict arises: keep BOTH edits verbatim (they do not overlap). The implementing
agent MUST NOT delete S2's [generation]/[role.*] additions, and S2 must not delete these header additions.
Both are PROVISIONAL reference text (P1.M4.T2.S1 later rewrites `config init` to a populated bootstrap,
superseding both). Flag this clearly.

DECISION — keep ALL added template lines `#`-commented (INERT), matching the existing convention. The
`config_test.go` equality test compares the WRITTEN file to the CONSTANT (editing the constant is safe).
Do NOT add uncommented keys.

## §7 — roleNames: one canonical slice (DRY)

```go
var roleNames = []string{"planner", "stager", "message", "arbiter"}
```

Package-level, unexported, in load.go. Used by BOTH loadEnv and loadFlags (and the test newFlagSet). One
source of truth for the four roles (PRD §13.6.2/§9.15 FR-R1) — avoids a duplicated literal that could drift.
If ResolveRoleModel (S2) or the orchestrator wants it exported later, promote `roleNames`→`RoleNames` (no
behavior change). Keep it unexported for now (tight scope).

## §8 — Test strategy: extend newFlagSet (safe) + new tests

load_test.go's `newFlagSet(t)` currently registers 5 flags (provider/model/timeout/verbose/no-color). It is
used by ~15 existing tests (TestLoadFlags_*, TestLoad_*). EXTEND it to also register the v2 flags:

```go
for _, role := range roleNames {
	fs.String(role+"-provider", "", "")
	fs.String(role+"-model", "", "")
}
fs.Int("commits", 0, "")
fs.Bool("single", false, "")
fs.Bool("no-decompose", false, "")
fs.Int("max-commits", 0, "")
```

SAFE: registering flags with defaults is BEHAVIOR-NEUTRAL for the existing tests (they don't `Set` the v2
flags, so `Changed` stays false → cfg unchanged → they stay green). New tests `Set` the v2 flags to exercise
the new code. This is the DRY choice (one "Config-backed flag set") over a separate `newV2FlagSet`.

New tests (all `package config` white-box):
- `TestLoadEnv_PerRole` — set STAGECOACH_PLANNER_PROVIDER/_MODEL + STAGECOACH_STAGER_MODEL → cfg.Roles populated.
- `TestLoadEnv_PerRolePartial` — set only STAGECOACH_PLANNER_MODEL → cfg.Roles["planner"]={Provider:"",Model:X}
  (field-level, not whole-block; mirrors the FR-R3 field-merge the file layer does).
- `TestLoadEnv_Commits` — STAGECOACH_COMMITS=3 → cfg.Commits=3.
- `TestLoadEnv_CommitsBadIntErrors` — STAGECOACH_COMMITS=abc → wrapped error containing "STAGECOACH_COMMITS".
- `TestLoadEnv_PerRoleOverridesGlobal` — (precedence sanity) STAGECOACH_PLANNER_MODEL beats a pre-set global.
- `TestLoadFlags_PerRole` — fs.Set("planner-provider","agy"), fs.Set("planner-model","gemini-2.5-pro") → cfg.Roles.
- `TestLoadFlags_Decompose` — fs.Set("commits","3")/("single")/("no-decompose")/("max-commits","5") → cfg fields.
- `TestLoad_CommitsOneNormalizesSingle` — via Load(): STAGECOACH_COMMITS=1 → cfg.Commits==1 && cfg.Single==true
  (the §4 normalization). Also `--commits 1` via fs → Single==true.
- `TestLoad_PerRoleFlagBeatsEnv` — env sets planner-model=X, flag sets planner-model=Y → cfg.Roles["planner"].Model==Y
  (flag > env, the FR-R3 precedence; mirrors TestLoad_CLIOverridesEnv for the per-role case).
- `TestSetRole_LazyAlloc` — setRoleProvider on a nil-Roles Config → map allocated, field set; setRoleModel on
  same → second field added (map-value-copy correctness).

## §9 — No go.mod change; config.go frozen; imports

load.go += `"strings"` (stdlib only — for strings.ToUpper in the per-role env loop). No third-party, no
internal/*. `go mod tidy` MUST be a no-op; `git diff --exit-code go.mod go.sum` MUST be empty. config.go
(S1) is FROZEN — the helpers are methods on *Config defined in load.go, not config.go. file.go/git.go (S2 +
others) untouched. The only files this subtask edits: load.go, load_test.go, internal/cmd/config.go.
