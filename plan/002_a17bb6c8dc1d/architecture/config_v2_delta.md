# Config System V2 Delta — Per-Role, Bootstrap & Versioning

## 1. Config Struct Changes

### Current Config (config.go)
```go
type Config struct {
    Provider     string        // [defaults]
    Model        string        // [defaults]
    Timeout      time.Duration
    AutoStageAll bool
    Verbose      bool
    NoColor      bool          `toml:"-"`
    MaxDiffBytes        int    // [generation]
    MaxMdLines          int
    MaxDuplicateRetries int
    SubjectTargetChars  int
    Output              *string
    StripCodeFence      *bool
    Providers map[string]map[string]any `toml:"-"`
}
```

### New Fields to Add
```go
type Config struct {
    // ... existing fields ...

    // V2: per-role provider/model (§16.4, FR-R1–R5)
    Roles map[string]RoleConfig `toml:"-"` // keyed by role name: "planner","stager","message","arbiter"

    // V2: schema version (§9.17 FR-B4)
    ConfigVersion int `toml:"config_version"`

    // V2: safety cap on auto-decompose (§9.14 FR-M4)
    MaxCommits int `toml:"max_commits"` // default 12

    // V2: additional binary extensions to filter (§9.1 FR3a)
    BinaryExtensions []string `toml:"binary_extensions"` // merged with built-in denylist

    // V2: decompose flags (behavioral — set by CLI, not in file)
    Commits  int  `toml:"-"` // --commits N; 0 = auto
    Single   bool `toml:"-"` // --single/--no-decompose
}

// RoleConfig holds per-role provider/model overrides (§16.4).
type RoleConfig struct {
    Provider string `toml:"provider"`
    Model    string `toml:"model"`
}
```

### Defaults() Updates
```go
func Defaults() Config {
    return Config{
        // ... existing ...
        Roles:           nil,      // no per-role overrides → all use global
        ConfigVersion:   CurrentConfigVersion,
        MaxCommits:      12,       // §9.14 FR-M4 default
        BinaryExtensions: nil,     // nil → built-in denylist only
        Commits:         0,        // auto-decompose
        Single:          false,
    }
}

const CurrentConfigVersion = 2  // bumped on any breaking config change
```

## 2. File Decode Changes (file.go)

### New decode structs
```go
type fileConfig struct {
    ConfigVersion int                        `toml:"config_version"` // V2
    Defaults      fileDefaults               `toml:"defaults"`
    Generation    fileGeneration             `toml:"generation"`
    Role          map[string]fileRoleConfig  `toml:"role"`  // V2: [role.<role>]
    Provider      map[string]map[string]any  `toml:"provider"`
}

type fileRoleConfig struct {
    Provider string `toml:"provider"`
    Model    string `toml:"model"`
}

type fileGeneration struct {
    // ... existing ...
    MaxCommits       int      `toml:"max_commits"`        // V2
    BinaryExtensions []string `toml:"binary_extensions"`  // V2
}
```

### materialize() updates
Copy `fc.Role` → `c.Roles`, `fc.ConfigVersion` → `c.ConfigVersion`, `fc.Generation.MaxCommits` → `c.MaxCommits`, `fc.Generation.BinaryExtensions` → `c.BinaryExtensions`.

### overlay() updates
Add per-role field-merge (same pattern as Providers):
```go
if len(src.Roles) > 0 {
    if dst.Roles == nil {
        dst.Roles = make(map[string]config.RoleConfig)
    }
    for role, rc := range src.Roles {
        existing := dst.Roles[role]
        if rc.Provider != "" { existing.Provider = rc.Provider }
        if rc.Model != ""    { existing.Model = rc.Model }
        dst.Roles[role] = existing
    }
}
if src.MaxCommits != 0 { dst.MaxCommits = src.MaxCommits }
if len(src.BinaryExtensions) > 0 { dst.BinaryExtensions = src.BinaryExtensions }
```

## 3. Env/Flag Resolution (load.go)

### New env vars (loadEnv)
```go
// Per-role: STAGEHAND_<ROLE>_PROVIDER / STAGEHAND_<ROLE>_MODEL
for _, role := range []string{"planner", "stager", "message", "arbiter"} {
    envProv := "STAGEHAND_" + strings.ToUpper(role) + "_PROVIDER"
    envModel := "STAGEHAND_" + strings.ToUpper(role) + "_MODEL"
    if v, ok := os.LookupEnv(envProv); ok && v != "" {
        cfg.setRoleProvider(role, v)
    }
    if v, ok := os.LookupEnv(envModel); ok && v != "" {
        cfg.setRoleModel(role, v)
    }
}

// STAGEHAND_COMMITS
if v, ok := os.LookupEnv("STAGEHAND_COMMITS"); ok && v != "" {
    if n, err := strconv.Atoi(v); err == nil { cfg.Commits = n }
}
```

### New CLI flags (loadFlags)
```go
for _, role := range []string{"planner", "stager", "message", "arbiter"} {
    flagName := role + "-provider"
    if fs.Changed(flagName) {
        if v, err := fs.GetString(flagName); err == nil { cfg.setRoleProvider(role, v) }
    }
    flagName = role + "-model"
    if fs.Changed(flagName) {
        if v, err := fs.GetString(flagName); err == nil { cfg.setRoleModel(role, v) }
    }
}
if fs.Changed("commits") {
    if v, err := fs.GetInt("commits"); err == nil { cfg.Commits = v }
}
if fs.Changed("single") || fs.Changed("no-decompose") {
    cfg.Single = true
}
if fs.Changed("max-commits") {
    if v, err := fs.GetInt("max-commits"); err == nil { cfg.MaxCommits = v }
}
```

## 4. Role Resolution Function

```go
// ResolveRoleModel returns the (provider, model) for a given role, applying
// the 5-layer precedence: flag/env (already in cfg.Roles) > [role.<role>] config >
// [defaults] (global) > built-in manifest default.
// provider=="" means "use global/default"; model=="" means "use manifest default_model".
func ResolveRoleModel(role string, cfg Config) (provider, model string) {
    // Layer: per-role config (includes flag/env which were already overlaid)
    if rc, ok := cfg.Roles[role]; ok {
        if rc.Provider != "" { provider = rc.Provider }
        if rc.Model != ""    { model = rc.Model }
    }
    // Layer: global default (fall back)
    if provider == "" { provider = cfg.Provider }
    if model == ""    { model = cfg.Model }
    return provider, model
}
```

## 5. Config Bootstrap (§9.17)

### config init — Populated (FR-B1/B2)
Instead of the inert commented template, `config init`:
1. Runs cascading detection (FR-D1) → finds highest-priority installed provider
2. Writes `[defaults] provider = <detected>`
3. Writes that provider's `[role.*]` per-role default models (FR-D4) **uncommented**
4. Writes other installed providers as **commented-out** `[role.*]` blocks
5. Parent dirs created; existing file NOT overwritten unless `--force`
6. Always prints the written path

New flags: `--provider <name>` (target specific), `--force` (overwrite), `--template` (inert reference, the old behavior).

### config upgrade (FR-B5)
Rewrites existing config to CurrentConfigVersion in place:
- Preserves user values for keys that still exist
- Comments out removed/renamed keys with a note
- Simple, idempotent, future-extensible

### First-run fallback (FR-B3)
If stagehand starts with no global config and no STAGEHAND_CONFIG:
- Auto-write bootstrap config once
- Print notice with path
- Continue — tool is never "unconfigured"

### Help de-duplication (FR-B6)
Remove the manual "Subcommands:" block from `config` and `providers` parent commands' Long help. Cobra's auto-generated "Available Commands" is the single source.

## 6. Config Version Advisory (FR-B4)

On load: compare file's `config_version` to `CurrentConfigVersion`:
- Missing/older → warn naming mismatch + remediation (`config upgrade` or `config init --force`)
- Newer → warn file is ahead of binary
- Advisory only — no automatic migration
