name: "P1.M1.T2.S1 — setRoleTimeout helper + per-role env _TIMEOUT branch (FR-R7 env layer)"
description: >
  Add `func (c *Config) setRoleTimeout(role string, d time.Duration)` to internal/config/load.go (the
  EXACT map-value-copy write-back idiom of setRoleReasoning — required for Go maps; sets ONE field so
  FR-R3 field-merge holds) and a `_TIMEOUT` branch in the per-role env loop (after `_REASONING`) that
  parses `STAGECOACH_<ROLE>_TIMEOUT` via parseTimeout ("480s" OR bare "480") and DIRECT-sets
  cfg.Roles[role].Timeout — mirroring the global STAGECOACH_TIMEOUT case. Plus 3 unit tests. Pure
  env-layer wiring; consumes S1's RoleConfig.Timeout (already landed at config.go:42). Does NOT touch
  file.go (S1/S2), flags (S2), git-config (S3), resolution (P1.M2.T1), the default change (P1.M2.T2),
  the 13 call sites (P1.M3), or docs (P1.M4.T2).

---

## Goal

**Feature Goal**: Wire the ENVIRONMENT layer of per-role generation timeouts (PRD §9.15 FR-R7, §9.8 FR35,
§16.1 layer 5) so `STAGECOACH_PLANNER_TIMEOUT=480s` (and stager/message/arbiter) is parsed and stored into
`cfg.Roles[role].Timeout` — using the same `parseTimeout` helper and DIRECT-set discipline the global
`STAGECOACH_TIMEOUT` already uses, and the same map-value-copy helper idiom the three existing
`setRole{Provider,Model,Reasoning}` helpers use.

**Deliverable**:
1. **internal/config/load.go** — (a) a new `func (c *Config) setRoleTimeout(role string, d time.Duration)`
   helper mirroring `setRoleReasoning` (load.go:57-65) 1:1 except it takes a `time.Duration` and sets
   `rc.Timeout`; (b) a new `if v, ok := os.LookupEnv(prefix + "_TIMEOUT"); ok && v != "" { ... }` branch
   in the per-role env loop (load.go:293-304), placed after the `_REASONING` branch, that `parseTimeout`s
   the value and calls `cfg.setRoleTimeout(role, d)`, returning a wrapped `STAGECOACH_<ROLE>_TIMEOUT`
   error on a bad value.
2. **internal/config/load_test.go** — 3 tests: (i) `setRoleTimeout` lazy-alloc + field-merge (mirror
   `TestSetRole_LazyAllocAndFieldMerge`), (ii) per-role env timeout parsing for ≥2 roles incl. the bare-int
   form + field-merge with `_PROVIDER` (mirror `TestLoadEnv_PerRole`), (iii) bad-value error wrapping
   (mirror `TestLoadEnv_BadTimeoutErrors`).

**Success Definition**:
- `STAGECOACH_PLANNER_TIMEOUT=480s` → `cfg.Roles["planner"].Timeout == 480*time.Second`.
- `STAGECOACH_STAGER_TIMEOUT=300` (bare int) → `cfg.Roles["stager"].Timeout == 300*time.Second` (proves
  `parseTimeout`, not `time.ParseDuration`).
- `STAGECOACH_PLANNER_TIMEOUT=not-a-dur` → `loadEnv` returns an error whose message contains
  `STAGECOACH_PLANNER_TIMEOUT`.
- Setting `STAGECOACH_PLANNER_TIMEOUT` does NOT clobber `STAGECOACH_PLANNER_PROVIDER` on the same role
  (FR-R3 field-merge — both survive in `cfg.Roles["planner"]`).
- `setRoleTimeout` on a `Config{}` (nil Roles) lazily allocates the map (mirrors the existing setters).
- `go build ./...`, `go vet ./internal/config/...`, `gofmt -l`, `make lint`, `make test` all pass.

## User Persona (if applicable)

**Target User**: A developer/CI author who wants to give the (slower) planner role a longer generation
budget than the message role, via an environment variable — e.g. `STAGECOACH_PLANNER_TIMEOUT=480s` in a
CI matrix or shell, the persistent/scriptable form of a per-role timeout.

**Use Case**: Multi-commit decomposition: the planner agent reasons over the whole diff and benefits from
a larger timeout; the message/stager agents are quick. Setting `STAGECOACH_PLANNER_TIMEOUT=480s` (while
leaving the global `STAGECOACH_TIMEOUT=120s`) gives the planner its own budget without per-invocation flags.

**User Journey**: `export STAGECOACH_PLANNER_TIMEOUT=480s` → `stagecoach` → load parses it into
`cfg.Roles["planner"].Timeout` → (after P1.M2.T1/P1.M3 land) the planner's `provider.Execute` call uses
480s while other roles use the global. (This subtask delivers the parse+store; consumption is downstream.)

**Pain Points Addressed**: Today every role shares the single flat `Config.Timeout`. There is no
`STAGECOACH_<ROLE>_TIMEOUT` (grep-confirmed — the env loop reads only `_PROVIDER/_MODEL/_REASONING`).
This task adds the per-role env source so a role can be budgeted independently.

## Why

- **FR-R7 / §9.15 / §16.1 layer 5 / §9.8 FR35**: Per-role timeouts resolve across the 7-layer precedence;
  the environment layer (5) is one of them, higher than file (3) and git-config (4). S1 (P1.M1.T1.S1)
  made `RoleConfig.Timeout` a typed field; this task makes `STAGECOACH_<ROLE>_TIMEOUT` actually populate it.
- **Why a tiny, mechanical subtask**: the env loop (load.go:293-304) already reads three per-role vars via
  three `setRole*` helpers; adding a fourth (`_TIMEOUT` + `setRoleTimeout`) is a 1:1 extension of the
  proven pattern. The global `STAGECOACH_TIMEOUT` case (load.go:260-266) is the exact parse+error+DIRECT-set
  template. parseTimeout already exists.
- **Complementary, non-overlapping**: S1 owns the struct + file materialize; S2 owns the file-layer overlay;
  THIS task owns the env layer. Flags (S2), git-config (S3), resolution (P1.M2.T1), default change
  (P1.M2.T2), 13 call sites (P1.M3), docs (P1.M4.T2) are all fenced out.

## What

**User-visible behavior**: None yet on its own (env parsing into the config). Combined with the rest of
FR-R7, `STAGECOACH_<ROLE>_TIMEOUT` budgets that role's generation independently. This subtask's observable
effect is at the unit-test level (loadEnv direct call → assert `cfg.Roles[role].Timeout`).

**Technical change (two small additions + tests):**
1. `setRoleTimeout(role string, d time.Duration)` — the map-value-copy write-back idiom (lazy alloc,
   copy out, set one field, write back). Setting timeout does NOT touch provider/model/reasoning.
2. The env-loop `_TIMEOUT` branch — `os.LookupEnv(prefix+"_TIMEOUT")` presence-semantic, `parseTimeout`
   (accepts `"480s"` and bare `"480"`), wrapped error `STAGECOACH_<ROLE>_TIMEOUT: %w` on bad value,
   DIRECT-set via `cfg.setRoleTimeout(role, d)`.

### Success Criteria
- [ ] `setRoleTimeout` exists, mirrors `setRoleReasoning`, takes `time.Duration`, sets `rc.Timeout` only.
- [ ] `STAGECOACH_PLANNER_TIMEOUT=480s` → `cfg.Roles["planner"].Timeout == 480*time.Second` (duration form).
- [ ] `STAGECOACH_STAGER_TIMEOUT=300` → `300*time.Second` (bare-int form proves parseTimeout used).
- [ ] `STAGECOACH_PLANNER_TIMEOUT=abc` → loadEnv error contains `STAGECOACH_PLANNER_TIMEOUT`.
- [ ] `_TIMEOUT` + `_PROVIDER` on the same role both survive in `cfg.Roles[role]` (FR-R3 field-merge).
- [ ] `setRoleTimeout` on a nil-Roles Config lazily allocates the map.
- [ ] `go build ./...`, `go vet ./internal/config/...`, `gofmt -l`, `make lint`, `make test` pass.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim helper to mirror, the verbatim env-loop branch to add (with the exact error format),
the global `STAGECOACH_TIMEOUT` mirror, the parseTimeout contract (both forms), the 3 tests to clone with
current line numbers, the prerequisite (S1 field, already landed), and the scope fences against 6 sibling
subtasks are all enumerated below.

### Documentation & References

```yaml
- file: internal/config/load.go
  why: "THE change site. setRoleReasoning @57-65 (the helper to mirror 1:1 — INSERT setRoleTimeout after
        its closing brace). The per-role env loop @293-304 (INSERT the _TIMEOUT branch after the _REASONING
        block @301-303). The global STAGECOACH_TIMEOUT case @260-266 (the parse+error+DIRECT-set mirror).
        roleNames @17 (the loop source: planner/stager/message/arbiter). parseTimeout @618 (reuse)."
  pattern: >
    // the helper idiom (setRoleReasoning @57):
    func (c *Config) setRoleReasoning(role, reasoning string) {
        if c.Roles == nil { c.Roles = make(map[string]RoleConfig) }
        rc := c.Roles[role]
        rc.Reasoning = reasoning
        c.Roles[role] = rc
    }
    // the env-loop branch idiom (after the _REASONING block @301-303):
    if v, ok := os.LookupEnv(prefix + "_REASONING"); ok && v != "" {
        cfg.setRoleReasoning(role, v)
    }
  critical: "Map-value-copy write-back is REQUIRED (Go forbids &map[k]; you cannot do c.Roles[role].X=d
             directly — copy out, mutate, write back). setRoleTimeout differs from setRoleReasoning ONLY
             in: param is time.Duration, field is rc.Timeout. The env _TIMEOUT branch differs from the
             global STAGECOACH_TIMEOUT case ONLY in: uses prefix (STAGECOACH_<ROLE>) in the LookupEnv AND
             the error, and calls setRoleTimeout instead of cfg.Timeout=d."

- file: internal/config/load.go
  why: "parseTimeout @618 — the single parse helper to reuse. Accepts BOTH '120s'/'2m' (time.ParseDuration)
        AND bare '120' (strconv.Atoi seconds). Unexported, same package → directly callable, no import."
  pattern: "parseTimeout(v) (time.Duration, error) — used by global STAGECOACH_TIMEOUT, --timeout, git
            stagecoach.timeout. Reuse it for the per-role var for cross-layer consistency."
  critical: "Do NOT use time.ParseDuration for the per-role var — it rejects bare '120'. parseTimeout is
             the consistent choice. The error is ALREADY wrapped by parseTimeout; the env branch wraps
             AGAIN with the var-name prefix (fmt.Errorf(\"%s_TIMEOUT: %w\", prefix, err))."

- file: internal/config/config.go
  why: "RoleConfig.Timeout time.Duration @42 (the field setRoleTimeout writes). ALREADY LANDED by S1
        (P1.M1.T1.S1) — prerequisite met. 0 is the 'inherit global' sentinel."
  pattern: "type RoleConfig struct { Provider string; Model string; Reasoning string; Timeout time.Duration }"
  critical: "This task CONSUMES RoleConfig.Timeout; it does NOT modify the struct. The 0='inherit' semantics
             matter for resolution (P1.M2.T1), not for this task — setRoleTimeout just stores whatever
             parseTimeout returns (always > 0 for a valid non-empty env value)."

- file: internal/config/load_test.go
  why: "The test patterns to clone. TestSetRole_LazyAllocAndFieldMerge @292 (helper lazy-alloc + field-merge).
        TestLoadEnv_PerRole @327 (env loop → cfg.Roles[role]). TestLoadEnv_BadTimeoutErrors @277 (error
        wrapping). TestLoad_TimeoutViaEnvInteger @1824 (full-Load bare-int timeout — proves parseTimeout form)."
  pattern: >
    cfg := Defaults(); t.Setenv("STAGECOACH_PLANNER_PROVIDER","agy"); loadEnv(&cfg)
    if rc := cfg.Roles["planner"]; rc.Provider != "agy" {...}
  critical: "Read cfg.Roles[role].Timeout (the PER-ROLE field), NOT cfg.Timeout (the global). They are
             different fields; the role→global fallback is P1.M2.T1's ResolveRoleTimeout, NOT this task.
             Use t.Setenv (Go 1.17+ auto-cleanup). Test ≥2 roles to prove the loop is general."

- docfile: plan/015_b461e4720495/architecture/research_role_config.md
  why: "§5 (Env-var parsing) specifies this task verbatim: 'FR-R7 adds: if v, ok := os.LookupEnv(prefix +
        \"_TIMEOUT\"); ok && v != \"\" { d, err := parseTimeout(v); ... cfg.setRoleTimeout(role, d) }' and
        'FR-R7 adds setRoleTimeout(role string, d time.Duration) following the identical idiom.'"
  section: "5. Env-var parsing — internal/config/load.go"

- docfile: plan/015_b461e4720495/P1M1T1S1/PRP.md
  why: "S1 is the CONTRACT: it produces RoleConfig.Timeout time.Duration (config.go:42). This task consumes it.
        S1 explicitly defers env/flag/git wiring to P1.M1.T2. Read it to confirm the field exists + semantics."

- docfile: plan/015_b461e4720495/P1M1T1S2/PRP.md
  why: "S2 (parallel) adds the file-layer overlay branch for Timeout in file.go. This task adds the env layer
        in load.go. NON-OVERLAPPING (different files, different layers). Both consume S1's field."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  load.go          # setRole{Provider,Model,Reasoning} @33/45/57 ← ADD setRoleTimeout after :57;
                   # per-role env loop @293-304 ← ADD _TIMEOUT branch; STAGECOACH_TIMEOUT @260 (mirror); parseTimeout @618
  load_test.go     # TestSetRole_LazyAllocAndFieldMerge @292; TestLoadEnv_PerRole @327; TestLoadEnv_BadTimeoutErrors @277 ← ADD 3 tests
  config.go        # RoleConfig.Timeout time.Duration @42 (from S1 — ALREADY LANDED; consumed, not modified)
  file.go          # materialize (S1) + overlay (S2) — NOT touched by this task
```

### Desired Codebase tree with files to be added/modified

```bash
internal/config/load.go        # MODIFY: +1 helper (setRoleTimeout) +1 env-loop branch
internal/config/load_test.go   # MODIFY: +3 tests (helper field-merge, env parsing, bad-value error)
# (no new files; no struct changes; no other package touched)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (map-value-copy write-back is REQUIRED): you CANNOT write c.Roles[role].Timeout = d directly —
//   Go forbids taking the address of a map element. The idiom (used by all 3 existing setters) is:
//   rc := c.Roles[role]; rc.Timeout = d; c.Roles[role] = rc. Mirror setRoleReasoning EXACTLY.

// CRITICAL (field-merge — set ONE field): setRoleTimeout must set rc.Timeout ONLY, leaving Provider/Model/
//   Reasoning untouched. The map-value-copy idiom does this naturally (you copy out the WHOLE RoleConfig,
//   change one field, write back). Do NOT construct a fresh RoleConfig{} (that would zero the siblings).

// CRITICAL (use parseTimeout, NOT time.ParseDuration): parseTimeout (load.go:618) accepts "480s" AND bare
//   "480". time.ParseDuration rejects bare ints. The global STAGECOACH_TIMEOUT, --timeout, and git
//   stagecoach.timeout ALL use parseTimeout — the per-role var must too (cross-layer consistency). A test
//   with the bare-int form (STAGECOACH_STAGER_TIMEOUT=300) is what PROVES parseTimeout was used.

// CRITICAL (error format — use prefix, not a hardcoded name): the env loop computes prefix := "STAGECOACH_"
//   + strings.ToUpper(role). The error MUST be fmt.Errorf("%s_TIMEOUT: %w", prefix, err) so it reads
//   STAGECOACH_PLANNER_TIMEOUT (role-specific), consistent with the global STAGECOACH_TIMEOUT: %w. Do NOT
//   hardcode "STAGECOACH_PLANNER_TIMEOUT" — the loop must work for all 4 roles.

// CRITICAL (DIRECT-set, not overlay): the env layer writes via setRoleTimeout (DIRECT), bypassing overlay().
//   This is correct — env (layer 5) is higher than file (3) and git-config (4), so a DIRECT set is the
//   escape hatch (same reason STAGECOACH_TIMEOUT DIRECT-sets cfg.Timeout at load.go:265). Do NOT route
//   through overlay (overlay is for file/git layer merges; env/flag DIRECT-set).

// CRITICAL (per-role field, NOT global): the branch sets cfg.Roles[role].Timeout, NOT cfg.Timeout. They are
//   DIFFERENT fields. The role→global fallback (Roles[role].Timeout==0 ⇒ use cfg.Timeout) is P1.M2.T1's
//   ResolveRoleTimeout — NOT this task. Tests must assert cfg.Roles[role].Timeout.

// CRITICAL (depends on S1 — ALREADY MET): RoleConfig.Timeout exists at config.go:42 (S1 landed). rc.Timeout
//   compiles. If somehow pre-S1, setRoleTimeout would not compile — confirm with grep first.

// COORDINATION (no conflict): S1 touches config.go+file.go; S2 touches file.go. NEITHER touches load.go.
//   load.go line numbers are STABLE. The only dependency (RoleConfig.Timeout) is already in the tree.

// SCOPE: do NOT add --<role>-timeout flags (P1.M1.T2.S2), stagecoach.role.<role>.timeout git reading
//   (P1.M1.T2.S3 — NEW infrastructure), ResolveRoleTimeout/defaultRoleTimeouts (P1.M2.T1), the 480s→120s
//   default change (P1.M2.T2), any of the 13 Execute call sites (P1.M3), or docs (P1.M4.T2).
```

## Implementation Blueprint

### Data models and structure
None. No new types, no struct changes. One new method on `*Config` (reuses `RoleConfig.Timeout` from S1)
and one new branch in the env loop (reuses `parseTimeout`). The `0`-duration "inherit" sentinel is S1's
concern; this task stores whatever parseTimeout returns (always a positive duration for a valid env value).

### Implementation Tasks (ordered by dependencies)

> **Prerequisite**: S1 (P1.M1.T1.S1) merged — `RoleConfig.Timeout time.Duration` must exist. CONFIRM
> (it does): `grep -n "Timeout   time.Duration" internal/config/config.go` → config.go:42. Then proceed.

```yaml
Task 1: MODIFY internal/config/load.go — add setRoleTimeout helper after setRoleReasoning
  - LOCATE func (c *Config) setRoleReasoning (search "func (c *Config) setRoleReasoning" — currently @57).
  - INSERT immediately AFTER its closing brace, BEFORE the "// Load resolves the full..." doc comment:
        // setRoleTimeout sets the Timeout field for a role in cfg.Roles, lazily allocating the map.
        // Map-value-copy write-back is REQUIRED (same idiom as setRoleReasoning).
        // Setting Timeout does NOT clobber existing Provider/Model/Reasoning — FR-R3 field-merge.
        func (c *Config) setRoleTimeout(role string, d time.Duration) {
            if c.Roles == nil {
                c.Roles = make(map[string]RoleConfig)
            }
            rc := c.Roles[role]
            rc.Timeout = d
            c.Roles[role] = rc
        }
  - VERIFY byte-identical idiom to setRoleReasoning (only the param type + field differ).
  - DEPENDENCIES: S1 (RoleConfig.Timeout exists → rc.Timeout compiles).

Task 2: MODIFY internal/config/load.go — add the _TIMEOUT branch to the per-role env loop
  - LOCATE the per-role env loop (search "for _, role := range roleNames" — the FIRST hit, @293).
  - FIND the _REASONING branch inside it:
        if v, ok := os.LookupEnv(prefix + "_REASONING"); ok && v != "" {
            cfg.setRoleReasoning(role, v)
        }
  - INSERT immediately AFTER that block, BEFORE the loop's closing `}`:
        // FR-R7 / §9.15 / §9.8 FR35 — per-role generation timeout via env (presence-semantic, DIRECT-set
        // via setRoleTimeout — bypasses overlay; mirrors the global STAGECOACH_TIMEOUT case). parseTimeout
        // accepts "480s" and bare "480" (cross-layer consistency with --timeout / stagecoach.timeout).
        if v, ok := os.LookupEnv(prefix + "_TIMEOUT"); ok && v != "" {
            d, err := parseTimeout(v)
            if err != nil {
                return fmt.Errorf("%s_TIMEOUT: %w", prefix, err)
            }
            cfg.setRoleTimeout(role, d)
        }
  - VERIFY the error uses `prefix` (so it reads STAGECOACH_PLANNER_TIMEOUT etc.), NOT a hardcoded name.
  - VERIFY it sets cfg.Roles[role].Timeout (via setRoleTimeout), NOT cfg.Timeout.
  - NO new imports: parseTimeout, os, fmt, strings all already imported in load.go.
  - DEPENDENCIES: Task 1 (setRoleTimeout must exist).

Task 3: MODIFY internal/config/load_test.go — add 3 tests
  - TEST A — TestSetRole_Timeout_LazyAllocAndFieldMerge (clone TestSetRole_LazyAllocAndFieldMerge @292):
        cfg := Config{} // Roles == nil
        cfg.setRoleTimeout("planner", 480*time.Second)
        if cfg.Roles == nil || cfg.Roles["planner"].Timeout != 480*time.Second { t.Fatalf(...) }
        // field-merge: setRoleTimeout must not have locked the role to timeout-only
        cfg.setRoleProvider("planner", "agy")
        rc := cfg.Roles["planner"]
        if rc.Timeout != 480*time.Second || rc.Provider != "agy" { t.Errorf("want both Timeout=480s and Provider=agy (field-merge), got %+v", rc) }
        // (and reverse order on a fresh cfg: setRoleProvider first, then setRoleTimeout → both survive)
  - TEST B — TestLoadEnv_PerRoleTimeout (clone TestLoadEnv_PerRole @327):
        cfg := Defaults()
        t.Setenv("STAGECOACH_PLANNER_TIMEOUT", "480s")   // duration form
        t.Setenv("STAGECOACH_STAGER_TIMEOUT", "300")     // bare-int form (proves parseTimeout)
        t.Setenv("STAGECOACH_PLANNER_PROVIDER", "agy")   // field-merge: same role, different field
        if err := loadEnv(&cfg); err != nil { t.Fatalf("loadEnv err=%v", err) }
        if rc := cfg.Roles["planner"]; rc.Timeout != 480*time.Second || rc.Provider != "agy" {
            t.Errorf("Roles[planner]=%+v want Timeout=480s Provider=agy", rc)
        }
        if rc := cfg.Roles["stager"]; rc.Timeout != 300*time.Second {
            t.Errorf("Roles[stager].Timeout=%v want 300s (bare int via parseTimeout)", rc.Timeout)
        }
        // unset role: no-op (message has no _TIMEOUT → absent or Timeout==0)
        if rc, ok := cfg.Roles["message"]; ok && rc.Timeout != 0 { t.Errorf("message timeout should be 0/absent, got %v", rc.Timeout) }
  - TEST C — TestLoadEnv_PerRoleTimeout_BadValueErrors (clone TestLoadEnv_BadTimeoutErrors @277):
        cfg := Defaults()
        t.Setenv("STAGECOACH_PLANNER_TIMEOUT", "not-a-dur")
        err := loadEnv(&cfg)
        if err == nil { t.Fatal("loadEnv err=nil, want error for bad per-role timeout") }
        if !strings.Contains(err.Error(), "STAGECOACH_PLANNER_TIMEOUT") {
            t.Errorf("err=%v, want it to contain 'STAGECOACH_PLANNER_TIMEOUT'", err)
        }
  - NAMING: Test{SetRole_Timeout..., LoadEnv_PerRoleTimeout, LoadEnv_PerRoleTimeout_BadValueErrors} —
    matches the file's Test<Area>_<Detail> convention. PLACE next to the mirrored tests.
  - USE t.Setenv (auto-cleanup). time.Duration literals (480*time.Second). DEPENDENCIES: Tasks 1+2.

Task 4: VERIFY — build, vet, format, targeted tests, full suite, grep guards
  - go build ./...
  - go vet ./internal/config/...
  - gofmt -l internal/config/load.go internal/config/load_test.go   # must list nothing
  - go test ./internal/config/... -run 'SetRole_Timeout|PerRoleTimeout' -v
  - make test && make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the setRoleTimeout helper (clone of setRoleReasoning @57 — map-value-copy write-back)
func (c *Config) setRoleTimeout(role string, d time.Duration) {
    if c.Roles == nil {                    // lazy alloc (mirrors all 3 setters)
        c.Roles = make(map[string]RoleConfig)
    }
    rc := c.Roles[role]                    // copy OUT (cannot &map[k])
    rc.Timeout = d                         // set ONE field (FR-R3 field-merge: siblings untouched)
    c.Roles[role] = rc                     // write BACK
}

// PATTERN: the env-loop _TIMEOUT branch (mirrors the global STAGECOACH_TIMEOUT case @260 + the _REASONING branch)
// Inside `for _, role := range roleNames { prefix := "STAGECOACH_" + strings.ToUpper(role); ... }`
if v, ok := os.LookupEnv(prefix + "_TIMEOUT"); ok && v != "" { // presence-semantic
    d, err := parseTimeout(v)                                 // "480s" OR bare "480"
    if err != nil {
        return fmt.Errorf("%s_TIMEOUT: %w", prefix, err)       // STAGECOACH_PLANNER_TIMEOUT: ...
    }
    cfg.setRoleTimeout(role, d)                               // DIRECT-set (env layer 5, bypasses overlay)
}

// PATTERN: the helper field-merge test (clone of TestSetRole_LazyAllocAndFieldMerge @292)
func TestSetRole_Timeout_LazyAllocAndFieldMerge(t *testing.T) {
    cfg := Config{} // Roles == nil
    cfg.setRoleTimeout("planner", 480*time.Second) // lazily allocs + sets Timeout
    if cfg.Roles == nil || cfg.Roles["planner"].Timeout != 480*time.Second {
        t.Fatalf("setRoleTimeout did not lazily alloc + set: %+v", cfg.Roles)
    }
    cfg.setRoleProvider("planner", "agy") // must NOT erase Timeout (FR-R3)
    rc := cfg.Roles["planner"]
    if rc.Timeout != 480*time.Second || rc.Provider != "agy" {
        t.Errorf("Roles[planner]=%+v want Timeout=480s Provider=agy (field-merge)", rc)
    }
}
```

### Integration Points

```yaml
NO database / routes / CLI / public-API / struct changes. One helper + one env branch + 3 tests.

LOAD PATH (internal/config/load.go):
  - +1 method: (c *Config) setRoleTimeout(role string, d time.Duration)
  - +1 branch: per-role env loop reads prefix+"_TIMEOUT" → parseTimeout → cfg.setRoleTimeout(role, d)

CONSUMED (from S1, already landed):
  - RoleConfig.Timeout time.Duration (config.go:42) — rc.Timeout reads/writes it.

DOWNSTREAM (this subtask ENABLES but does NOT build — sibling subtasks):
  - P1.M1.T2.S2: --<role>-timeout CLI flags + loadFlags branch (DIRECT-set, mirrors this).
  - P1.M1.T2.S3: stagecoach.role.<role>.timeout git-config reading (NEW loop in loadGitConfig).
  - P1.M2.T1.S1: ResolveRoleTimeout(role, cfg) + defaultRoleTimeouts{planner:480s} (reads Roles[role].Timeout).
  - P1.M2.T2.S1: global default 480s→120s + pinning-test fixes.
  - P1.M3: 13 provider.Execute call sites pass the resolved per-role timeout instead of cfg.Timeout.

PRECEDENCE (this task = layer 5, the env source):
  CLI flag --<role>-timeout (S2) > env STAGECOACH_<ROLE>_TIMEOUT (THIS) > [role.<role>].timeout TOML
    (S1+S2 file layer) > stagecoach.role.<role>.timeout git (S3) > global timeout > built-in role default.

UNCHANGED (do NOT touch): config.go structs (S1); file.go materialize/overlay (S1/S2); git.go (S3);
  Defaults().Timeout (stays 480s — P1.M2.T2); the 13 Execute call sites (P1.M3); docs (P1.M4.T2).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build everything (RoleConfig.Timeout is consumed; setRoleTimeout must compile across the package)
go build ./...
# Vet the changed package
go vet ./internal/config/...
# Format check
gofmt -l internal/config/load.go internal/config/load_test.go
# Expected: nothing listed. If listed: gofmt -w the file(s).
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new helper + env-loop tests (targeted)
go test ./internal/config/... -run 'SetRole_Timeout|PerRoleTimeout' -v
# Expected: all pass — helper lazy-alloc + field-merge; env duration form + bare-int form + field-merge;
#           bad-value error wrapping.

# Full config package (regression — existing setRole/env tests must stay green)
go test ./internal/config/... -v

# Whole suite (race) — load.go is on the load path of every config.Load
make test
# Expected: ALL pass. Global default still 480s (unchanged here) → 480s-pinning tests untouched.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary
make build

# Smoke: STAGECOACH_PLANNER_TIMEOUT loads cleanly (parse happens; no consumer uses it yet, but load must
# not error). After this task the value is STORED in cfg.Roles["planner"].Timeout; behavior observation
# (the planner actually using 480s) lands with P1.M2.T1/P1.M3. This smoke proves the env parse path:
mkdir -p /tmp/sc_role_env && cd /tmp/sc_role_env && git init -q
STAGECOACH_PLANNER_TIMEOUT=480s /home/dustin/projects/stagecoach/bin/stagecoach --dry-run --no-color 2>&1 | head -5
# Expected: loads WITHOUT a "STAGECOACH_PLANNER_TIMEOUT" parse error (it may exit for other reasons —
#           e.g. nothing staged — but NOT a timeout parse error). A malformed value
#           (STAGECOACH_PLANNER_TIMEOUT=abc) would print "STAGECOACH_PLANNER_TIMEOUT: ..." and exit 1.
cd / && rm -rf /tmp/sc_role_env
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: setRoleTimeout exists exactly once; the env-loop branch exists exactly once
grep -n 'func (c \*Config) setRoleTimeout' internal/config/load.go   # one hit
grep -n 'prefix + "_TIMEOUT"' internal/config/load.go               # one hit (the LookupEnv line)

# Grep guard: the error uses prefix (role-specific), not a hardcoded name
grep -n '_TIMEOUT: %w' internal/config/load.go
# Expected: fmt.Errorf("%s_TIMEOUT: %w", prefix, err) — NOT a hardcoded "STAGECOACH_PLANNER_TIMEOUT".

# Grep guard: parseTimeout (not time.ParseDuration) is used for the per-role var
grep -n 'parseTimeout\|ParseDuration' internal/config/load.go
# Expected: the new _TIMEOUT branch uses parseTimeout (the global STAGECOACH_TIMEOUT also uses parseTimeout;
#           time.ParseDuration should NOT appear in the per-role branch).

# Scope-boundary guard: this subtask added NO flags / git-config / resolution / default changes
grep -rn 'planner-timeout\|--.*-timeout\|ResolveRoleTimeout\|defaultRoleTimeouts\|stagecoach.role.' internal/config/load.go
# Expected: empty (those are P1.M1.T2.S2/S3, P1.M2.T1 — NOT this subtask).
grep -n '120 \* time.Second' internal/config/config.go
# Expected: empty (global default 480s→120s is P1.M2.T2; Defaults().Timeout must still be 480*time.Second).

# Scope-boundary guard: only load.go + load_test.go changed (NOT config.go structs, NOT file.go)
git diff --stat -- internal/config/
# Expected: only internal/config/load.go + internal/config/load_test.go (no config.go / file.go churn).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/config/...` clean
- [ ] `gofmt -l internal/config/load.go internal/config/load_test.go` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass, incl. the 3 new tests

### Feature Validation
- [ ] `setRoleTimeout(role, d)` mirrors `setRoleReasoning`; sets `rc.Timeout` only (field-merge)
- [ ] `setRoleTimeout` lazily allocates a nil `Roles` map (unit test)
- [ ] `STAGECOACH_PLANNER_TIMEOUT=480s` → `cfg.Roles["planner"].Timeout == 480*time.Second`
- [ ] `STAGECOACH_STAGER_TIMEOUT=300` (bare int) → `300*time.Second` (proves parseTimeout used)
- [ ] `STAGECOACH_PLANNER_TIMEOUT=abc` → loadEnv error contains `STAGECOACH_PLANNER_TIMEOUT`
- [ ] `_TIMEOUT` + `_PROVIDER` on the same role both survive in `cfg.Roles[role]` (FR-R3)
- [ ] Existing setRole / per-role env / global timeout tests still pass unchanged

### Scope-Boundary Validation
- [ ] NO `--<role>-timeout` flags / loadFlags branch added (P1.M1.T2.S2)
- [ ] NO `stagecoach.role.<role>.timeout` git-config reading added (P1.M1.T2.S3)
- [ ] NO `ResolveRoleTimeout` / `defaultRoleTimeouts` added (P1.M2.T1)
- [ ] `Defaults().Timeout` STILL 480s; 480s-pinning tests UNCHANGED (P1.M2.T2)
- [ ] NO config.go struct / file.go materialize-or-overlay / Execute call-site / docs changes
- [ ] Only `internal/config/load.go` + `internal/config/load_test.go` changed

### Code Quality & Docs
- [ ] `setRoleTimeout` doc comment mirrors the setRoleReasoning doc (lazy-alloc + field-merge note)
- [ ] env-loop branch comment cites FR-R7 / §9.15 / FR35 + the parseTimeout both-forms + DIRECT-set rationale
- [ ] Error uses `prefix` (role-specific), consistent with the global `STAGECOACH_TIMEOUT` discipline
- [ ] Tests use `t.Setenv`, read `cfg.Roles[role].Timeout` (per-role field, not global), test ≥2 roles

---

## Anti-Patterns to Avoid

- ❌ Don't write `c.Roles[role].Timeout = d` directly — Go forbids `&map[k]`. Use the map-value-copy write-back idiom (`rc := c.Roles[role]; rc.Timeout = d; c.Roles[role] = rc`), byte-identical to `setRoleReasoning`.
- ❌ Don't construct a fresh `RoleConfig{Timeout: d}` in setRoleTimeout — that would ZERO the sibling fields (Provider/Model/Reasoning) and break FR-R3 field-merge. Copy out the existing entry, change one field, write back.
- ❌ Don't use `time.ParseDuration` for the per-role env var — it rejects bare `"480"`. Use `parseTimeout` (load.go:618), consistent with the global `STAGECOACH_TIMEOUT` / `--timeout` / `stagecoach.timeout`. The bare-int test case is what PROVES parseTimeout was chosen.
- ❌ Don't hardcode `"STAGECOACH_PLANNER_TIMEOUT"` in the error — use `fmt.Errorf("%s_TIMEOUT: %w", prefix, err)` so the loop produces the right name for all 4 roles (planner/stager/message/arbiter).
- ❌ Don't set `cfg.Timeout` (the global) in the per-role branch — set `cfg.Roles[role].Timeout` via `setRoleTimeout`. They are DIFFERENT fields; the role→global fallback is P1.M2.T1's `ResolveRoleTimeout`, not this task.
- ❌ Don't route the env value through `overlay()` — env is layer 5 (DIRECT-set), higher than file (3) / git-config (4). DIRECT-set via setRoleTimeout is the escape hatch, exactly as `STAGECOACH_TIMEOUT` DIRECT-sets `cfg.Timeout`.
- ❌ Don't add the `--<role>-timeout` flag, the `stagecoach.role.<role>.timeout` git reader, `ResolveRoleTimeout`, the 480s→120s default change, or any Execute call-site change — all fenced to siblings (S2/S3, P1.M2.T1, P1.M2.T2, P1.M3).
- ❌ Don't touch `config.go` (struct) or `file.go` (materialize/overlay) — S1/S2 own those; this task is load.go-only. load.go line numbers are stable because no parallel sibling edits load.go.
- ❌ Don't read `cfg.Timeout` in the tests to verify the per-role env var — read `cfg.Roles[role].Timeout`. (A test that asserts on `cfg.Timeout` would pass even if the branch were missing, masking the bug.)
- ❌ Don't test only the planner role — test ≥2 (planner + stager) to prove the loop is general, not hardcoded to the first iteration.

---

## Confidence Score: 10/10

One-pass success is essentially certain: the helper is a 1:1 clone of an existing, field-tested setter
(`setRoleReasoning`) with one field changed; the env-loop branch is a 1:1 clone of the global
`STAGECOACH_TIMEOUT` case (parse + wrapped error + DIRECT-set) placed in the existing per-role loop; the
architecture research specifies both verbatim; the prerequisite field (`RoleConfig.Timeout`) is already in
the tree; and there is NO file-level conflict with the parallel siblings (S1/S2 edit config.go/file.go,
this task edits load.go). The three tests are verbatim-clones of three existing tests
(`TestSetRole_LazyAllocAndFieldMerge`, `TestLoadEnv_PerRole`, `TestLoadEnv_BadTimeoutErrors`). The only
vigilance points — map-value-copy idiom, parseTimeout (not ParseDuration), prefix-based error, per-role
field vs global, DIRECT-set vs overlay — are all enumerated as CRITICAL gotchas with grep guards.
