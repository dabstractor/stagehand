# S2 Implementation Notes — Env (STAGECOACH_NO_VERIFY) + git-config + flag resolution for NoVerify

> Scope: P1.M1.T1.S2 — wire `NoVerify` through the ambient config layers (env, flag, git-config) in
> `load.go` + `git.go`, mirroring `Push` byte-for-byte. **HookTimeout has NO ambient layers** (FR-V6:
> file+default only). **S1 is fully landed** (Config/fileGeneration/materialize/overlay + Defaults;
> baseline `go test ./internal/config/` GREEN). Verified 2026-07-05.

## 0. Baseline + S1 state (confirmed)

- `config.go`: `NoVerify bool toml:"no_verify"` (line 134) + `HookTimeout time.Duration toml:"hook_timeout"`
  (line 138); `Defaults()` NoVerify=false (205) + HookTimeout=10*time.Minute (206). **S1 landed.**
- `file.go`: fileGeneration fields (67-68); loadTOML HookTimeout parse (188-193); `materialize(fc, timeout,
  hookTimeout time.Duration)` (207) with `c.NoVerify = true` only-true (291-292); overlay NoVerify only-true
  (352-353) + HookTimeout `!= 0` (356-357). **S1 landed — DO NOT touch file.go.**
- `load.go` + `git.go`: **ZERO** NoVerify/HookTimeout references → the ambient layers S2 owns are the gap.
- `go test ./internal/config/` → **GREEN** (2.317s). `load_test.go` / `git_test.go` untouched (clean for S2).

## 1. The three mirror edits (all mirror Push exactly)

### (a) load.go loadEnv() — STAGECOACH_NO_VERIFY (DIRECT set)
Template = STAGECOACH_PUSH (load.go:294-301). Insert immediately AFTER the STAGECOACH_PUSH block, before
`return nil` (~line 302):
```go
// §9.25 FR-V5 — no_verify via env (presence-semantic, DIRECT set — can be false, the escape hatch).
if v, ok := os.LookupEnv("STAGECOACH_NO_VERIFY"); ok && v != "" {
    b, err := strconv.ParseBool(v)
    if err != nil {
        return fmt.Errorf("STAGECOACH_NO_VERIFY: %w", err)
    }
    cfg.NoVerify = b // DIRECT set — can be false (escape hatch, mirrors STAGECOACH_PUSH)
}
```
DIRECT set (not only-true): `STAGECOACH_NO_VERIFY=false` forces NoVerify=false, overriding any file/git-config
`true`. This is the high-precedence escape hatch (env is layer 6 > git-config 5 > file 4). NO
STAGECOACH_HOOK_TIMEOUT (FR-V6).

### (b) load.go loadFlags() — --no-verify reader (DIRECT set)
Template = --push (load.go:423-427). Insert immediately AFTER the --push block, before the closing `}`
(~line 428):
```go
// §9.25 FR-V5 — --no-verify flag (DIRECT set; mirrors --push).
if fs.Changed("no-verify") {
    if v, err := fs.GetBool("no-verify"); err == nil {
        cfg.NoVerify = v // DIRECT set
    }
}
```
DIRECT set: the flag (layer 7, highest) can force true OR false. NO --hook-timeout flag (FR-V6).

**CRITICAL (T2 dependency):** the `--no-verify` FLAG VAR is registered in **P1.M1.T2.S1** (root.go
BoolVar) — NOT here. S2 writes only the READER. Pre-T2, `fs.Changed("no-verify")` returns false for an
unregistered flag (pflag lookup → not found → false; GetBool is never called → no error), so the reader
is **inert and safe** until T2 registers the flag. Once T2 lands, the reader activates. This is the
correct parallel-execution ordering; do NOT register the flag in S2.

### (c) git.go loadGitConfig() — stagecoach.no_verify (mirror stagecoach.push)
Template = stagecoach.push (git.go:174-179) + the `gitConfigBool` helper (git.go:63-78). Insert immediately
AFTER the stagecoach.push block (~line 180):
```go
// §9.25 FR-V5 — no_verify via git config (lowercase single-word key — no camelCase needed).
if v, found, err := gitConfigBool(repoDir, "stagecoach.no_verify"); err != nil {
    return nil, err
} else if found {
    c.NoVerify = v
}
```
`gitConfigBool` runs `git config --bool --get` (canonicalizes on/off/yes/no/1/0 → true/false; ParseBool
never fails). loadGitConfig sets `c.NoVerify = v` DIRECTLY (true OR false). **The only-true-propagates
limitation applies DOWNSTREAM in overlay** (`if src.NoVerify { dst.NoVerify = true }`, S1's overlay) — so
a git-config `stagecoach.no_verify = false` is a no-op through overlay (can't undo a lower layer's true via
git-config; same accepted limitation as Push). This is consistent and documented. NO stagecoach.hook_timeout.

## 2. The full 5-layer precedence for NoVerify (the contract's OUTPUT)

| layer | source | mechanism | can set false? |
|-------|--------|-----------|----------------|
| 7 flag | `--no-verify` | loadFlags DIRECT (`cfg.NoVerify = v`) | YES (escape hatch) |
| 6 env | `STAGECOACH_NO_VERIFY` | loadEnv DIRECT (`cfg.NoVerify = b`) | YES (escape hatch) |
| 5 git-config | `stagecoach.no_verify` | gitConfigBool → `c.NoVerify=v`, then overlay only-true | NO (only-true) |
| 4/3 file | `[generation] no_verify` | materialize/overlay only-true (S1) | NO (only-true) |
| 1 default | Defaults() | `NoVerify: false` (S1) | — |

Precedence flag > env > git-config > file > default. The DIRECT set at layers 6/7 is the escape hatch
(force false); the only-true at layers 3/4/5 is the accepted v1 limitation (can't undo a default-false via
those layers) — identical to Push. `STAGECOACH_NO_VERIFY=false` overrides a `[generation] no_verify=true`.

## 3. HookTimeout — DO NOT add ambient layers (FR-V6)

HookTimeout resolves through **file + default ONLY** (arch codebase_reality.md §2: "Decision: HookTimeout
is file + default only (no env/flag/git-config) — simplest"). S1 wired layers 1+4 (Defaults 10m +
fileGeneration + loadTOML parse + materialize + overlay). S2 adds NOTHING for HookTimeout — no
STAGECOACH_HOOK_TIMEOUT, no --hook-timeout, no stagecoach.hook_timeout. This is the #1 anti-pattern to avoid.

## 4. Tests (mirror the Push test patterns)

### load_test.go (env layer)
- Extend `TestLoadEnv_StringsTimeoutBools` (load_test.go:144): add `t.Setenv("STAGECOACH_NO_VERIFY",
  "true")` + assert `cfg.NoVerify == true`.
- Extend `TestLoadEnv_BoolFalseEscape` (load_test.go:172 — the DIRECT-set proof): add
  `t.Setenv("STAGECOACH_NO_VERIFY", "false")` + assert `cfg.NoVerify == false` (the escape hatch: env can
  force false, unlike the file layer).
- (flag reader) Add a focused `TestLoadFlags_NoVerify`: build a `pflag.FlagSet`, register `"no-verify"`
  via `fs.BoolVar(&fv, "no-verify", false, "")`, `fs.Set("no-verify", "true")`, call `loadFlags(&cfg, fs)`,
  assert `cfg.NoVerify == true`. This is SELF-CONTAINED (does not depend on root.go/T2's registration) —
  it proves the reader works once a flag exists. (Mirror any existing loadFlags --push test if present.)

### git_test.go (git-config layer)
- Extend `TestLoadGitConfig_ReadsValues` (git_test.go:71): add `setGitConfig(t, repo, "stagecoach.no_verify",
  "true")` + assert `cfg.NoVerify == true` (mirrors the stagecoach.push assertion at lines 87/128-129).
- Extend `TestLoadGitConfig_BoolNormalization` (git_test.go:166): add `setGitConfig(t, repo,
  "stagecoach.no_verify", "false")` + assert `cfg.NoVerify == false` (mirrors stagecoach.push=false at
  184/189-190). NOTE: this tests loadGitConfig's DIRECT output (c.NoVerify=v); the overlay only-true
  behavior is a separate concern (file_test.go, S1's territory).

## 5. Scope discipline — what S2 does NOT do

- NOT file.go (S1 — landed: fileGeneration/materialize/overlay).
- NOT config.go (S1 — landed: fields/Defaults).
- NOT the --no-verify flag REGISTRATION (P1.M1.T2.S1 — root.go BoolVar + help + cli.md row). S2 = reader only.
- NOT HookTimeout env/flag/git-config (FR-V6 — none exist).
- NOT any consumer (M3 RunCommitHooks reads cfg.NoVerify/HookTimeout).
- NOT PRD.md / tasks.json / prd_snapshot.md / plan/*.

## 6. Sources

- `architecture/codebase_reality.md` §2 (NoVerify → mirror Push; HookTimeout → file+default only).
- `P1M1T1S1/PRP.md` (S1 — the fields + file-layer plumbing; S2 = the ambient layers).
- PRD §9.25 FR-V5 (--no-verify, env STAGECOACH_NO_VERIFY, git stagecoach.no_verify, default false) + FR-V6
  (hook_timeout file+default only); §15.2 flag table (--no-verify row); §16.1 precedence.
- `internal/config/load.go` (STAGECOACH_PUSH :294-301, --push :423-427) + `git.go` (gitConfigBool :63-78,
  stagecoach.push :174-179).
