# Research Notes — P1.M1.T2.S3 (per-role git-config timeout)

## The change site: internal/config/git.go `loadGitConfig`

`loadGitConfig(repoDir string) (*Config, error)` is the git-config layer (PRD §16.1 layer 4). It returns
a PARTIAL `*Config` (all fields zero unless found). The global `stagecoach.timeout` block is the EXACT
template to mirror:

```go
// git.go:147-156 (current)
// --- timeout: accepts both "90" (seconds) and "90s" (Go duration) forms. ---
if v, found, err := gitConfigGet(repoDir, "stagecoach.timeout"); err != nil {
    return nil, err
} else if found {
    d, perr := parseTimeout(v) // parseTimeout handles both "90" and "90s"
    if perr != nil {
        return nil, fmt.Errorf("git config stagecoach.timeout: %w", perr)
    }
    c.Timeout = d
}
```

**INSERTION POINT**: immediately AFTER this block's closing `}` (line 156), BEFORE the booleans comment
`// --- booleans (--bool canonicalizes; FINDING C) ---` (line 158). The per-role loop goes here.

**CRITICAL**: `loadGitConfig` HAS an error return (`*Config, error`). So a malformed per-role timeout
is a HARD ERROR (`return nil, fmt.Errorf(...)`), NOT a silent-ignore. This is the OPPOSITE of loadFlags
(S2), which has no error return and silently ignores. This matches the global `stagecoach.timeout`
block exactly (it also errors on bad value).

## Available dependencies (all in `package config`, all directly accessible from git.go)

- `roleNames` — load.go:17: `var roleNames = []string{"planner", "stager", "message", "arbiter"}`. Same
  package → no import needed.
- `gitConfigGet(repo, key)` — git.go:71: returns `(value string, found bool, err error)`. exit 0 → found;
  exit 1 → missing (found=false, NOT error); else → wrapped error.
- `parseTimeout(s)` — load.go:640: accepts `"600s"`/`"2m"` (time.ParseDuration) AND bare `"600"`
  (strconv.Atoi seconds). Returns `(time.Duration, error)`.
- `c.setRoleTimeout(role, d)` — load.go:66-78 (from S1, ALREADY LANDED): map-value-copy write-back,
  sets `Roles[role].Timeout` only (FR-R3 field-merge).
- `fmt` — ALREADY imported in git.go (line 5). NO new imports needed.

## The contract loop (verbatim from item description)

```go
for _, role := range roleNames {
    key := "stagecoach.role." + role + ".timeout"
    if v, found, err := gitConfigGet(repoDir, key); err != nil {
        return nil, err
    } else if found {
        d, perr := parseTimeout(v)
        if perr != nil {
            return nil, fmt.Errorf("git config %s: %w", key, perr)
        }
        c.setRoleTimeout(role, d)
    }
}
```

NOTE: this adds per-role git-config for TIMEOUT ONLY (not provider/model/reasoning — those remain
file/env/flag only; Finding 2 confirms git.go reads NO per-role keys today).

## Overlay correctness (file.go:467-496) — verified, NO change needed

`overlay(dst, src)` merges `src.Roles` into `dst.Roles` per-FIELD with non-zero-wins guards:

```go
for role, rc := range src.Roles {
    existing := dst.Roles[role]                  // zero value if absent
    if rc.Provider != ""  { existing.Provider = rc.Provider }
    if rc.Model != ""     { existing.Model = rc.Model }
    if rc.Reasoning != "" { existing.Reasoning = rc.Reasoning }
    if rc.Timeout != 0    { existing.Timeout = rc.Timeout }   // <-- the FR-R7 guard
    dst.Roles[role] = existing
}
```

The git-config partial Config has `Roles[role] = {Timeout: d}` (Provider/Model/Reasoning all zero, since
git.go only reads timeout). When Load calls `overlay(&cfg, gc)` at load.go:138, the timeout merges in
WITHOUT clobbering any file-layer per-role provider/model/reasoning. This is the FR-R3 field-merge
guarantee and it works with NO overlay change. The `!= 0` guard is correct (timeout has no meaningful
"explicit 0"; 0 ⇒ inherit global).

## Git-config multi-dot keys — VERIFIED load-bearing assumption

`stagecoach.role.planner.timeout` is valid git config syntax: section=`stagecoach`,
subsection=`role.planner`, key=`timeout`. Verified in a temp repo:
- `git config stagecoach.role.planner.timeout 600s` → WRITE succeeds
- `git config --get stagecoach.role.planner.timeout` → returns `600s`, exit 0 (found)
- `git config --get stagecoach.role.stager.timeout` (unset) → exit 1 (found=false, NOT error)
- `gitConfigGet` handles exit-1 as `(value="", found=false, err=nil)` — matches git.go:80 case 1.

The git_test.go helper `setGitConfig(t, dir, "stagecoach.role.planner.timeout", "600s")` works (it runs
`git -C dir config <key> <value>`).

## Test patterns (internal/config/git_test.go)

Existing helpers + conventions:
- `initRepo(t, dir)` — `git -C dir init` + sets user.name/user.email.
- `setGitConfig(t, dir, key, value)` — `git -C dir config <key> <value>`.
- `t.Setenv("HOME", t.TempDir())` — isolates global ~/.gitconfig (FINDING E — prevents a stray global
  stagecoach.* key from leaking into the test).
- Tests call `loadGitConfig(repo)` and assert on `cfg.*` fields.
- Error tests assert `strings.Contains(err.Error(), "<key>")` AND `strings.Contains(err.Error(), "<phrase>")`.
- `TestLoadGitConfig_BadTimeout` (Test D): bad value → error contains `stagecoach.timeout` + `invalid timeout`.
- `TestLoadGitConfig_TimeoutDurationForm` (Test D2): both `"90"` and `"2m30s"` forms.
- `TestLoadGitConfig_OverlaysWithDefaults` (Test H): proves partial overlay (git overrides Defaults but
  unset git fields keep Defaults) — calls `overlay(&cfg, gc)` directly.

For per-role, assert on `cfg.Roles[role].Timeout` (the PER-ROLE field), NOT `cfg.Timeout` (global).

## Docs (docs/configuration.md) — git-config section

- `## Git-config keys` at line 211. Keys table at lines 225-237.
- Line 227: `stagecoach.timeout` row (the global — sibling of the new per-role key).
- **Line 240 NOTE is now STALE**: "The git-config layer has **no** per-role keys (`stagecoach.role.*`),
  ... Per-role configuration is available via CLI flags, env vars, and config-file `[role.*]` blocks only."
  This MUST be UPDATED — after this task, `stagecoach.role.<role>.timeout` IS read. Provider/model/
  reasoning remain NOT read via git-config (still file/env/flag only). The NOTE must be rewritten to
  scope the "no per-role" claim to provider/model/reasoning only, and acknowledge the new timeout key.

## Coordination with parallel S2 (P1.M1.T2.S2)

S2 owns: `internal/cmd/root.go`, `internal/config/load.go` (loadFlags branch), `docs/cli.md`.
S3 owns: `internal/config/git.go`, `internal/config/git_test.go`, `docs/configuration.md`.
**DIFFERENT files → NO merge conflict.** S2 consumes the same `setRoleTimeout` (S1) but via loadFlags;
S3 consumes it via loadGitConfig. Both append to different functions / files.

**Docs/cli.md coordination note**: S2's PRP adds 4 `--<role>-timeout` rows to docs/cli.md with the
git-config column = "—" (because, when S2 is written, git.go does not read per-role keys). S2's PRP
explicitly anticipates: "When [S3] lands, it can flip the docs/cli.md git-config column from — to
stagecoach.role.<role>.timeout." S3's CONTRACTED docs scope is docs/configuration.md ONLY. To avoid a
parallel-merge conflict on docs/cli.md (S2's file), S3 does NOT touch docs/cli.md; the "—" → key flip
is deferred (S2's PRP documents this as the downstream item). docs/configuration.md is the AUTHORITATIVE
git-config reference and is fully updated by S3.

## Dependency status

- `setRoleTimeout` (S1) — LANDED (load.go:66-78, confirmed by reading the file).
- `RoleConfig.Timeout time.Duration` (S1 grandparent) — LANDED (config.go:42).
- `parseTimeout` — exists (load.go:640).
- Everything S3 needs is in-tree. No prerequisite unmet.
