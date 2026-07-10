name: "P1.M4.T1.S1 — Unit tests for ResolveRoleTimeout + config loading + default change (FR-R7 gap-fill + consolidation)"
description: >
  A TEST-COMPLETION task (no production code). The per-role-timeout config infrastructure (P1.M1) +
  resolution + default change (P1.M2) are LANDED, and they BUNDLED their own test coverage as they
  shipped — ~20 tests across roles_test.go / load_test.go / file_test.go / git_test.go / config_test.go
  already cover the contract's clauses (b) env, (b) flag, (c) overlay, (d) git, (e) Defaults().Timeout==120s
  almost entirely. So this item is NOT greenfield authoring: blindly writing the contract's named tests
  (TestEnvRoleTimeout/TestFlagRoleTimeout/TestFileRoleTimeout/TestOverlayRoleTimeout/TestGitConfigRoleTimeout
  /TestResolveRoleTimeout) would DUPLICATE coverage. This item's job is GAP-FILL + CONSOLIDATION, targeting
  the 4 genuine gaps the LANDED tests miss: (GAP1) NO test sets a NON-ZERO cfg.Roles["message"].Timeout and
  asserts ResolveRoleTimeout("message") returns it — the contract clause (a) "message with
  [role.message].timeout returns that" case, and the highest-value untested branch (message is the ONLY
  active role on the single-commit path); (GAP2) NO "all canonical roles" timeout table — the existing
  TestResolveRoleModel_AllCanonicalRoles IS this idiom for the model axis; the timeout axis is scattered
  across 6 ad-hoc tests and needs the consolidated twin; (GAP3) NO test calls ResolveRoleTimeout with the
  literal Defaults() — clause (e)'s "default change verified" is functionally proven (_PlannerBuiltinBeatsGlobal
  uses Defaults()+set Timeout=120s, identical) but not made explicit; (GAP4) TestMaterializeRoleTimeout
  covers planner+stager but NOT arbiter — clause (c) names [role.arbiter].timeout="200s" explicitly. ADD 4
  tests: roles_test.go +TestResolveRoleTimeout_MessagePerRoleOverride +TestResolveRoleTimeout_AllCanonicalRoles
  +TestResolveRoleTimeout_PlannerDefaultFromDefaults; file_test.go +TestMaterializeRoleTimeout_ArbiterRole.
  Touches ONLY internal/config test files. NO production code, NO decompose/generate edits (P1.M3.T2.S1 is
  parallel + owns those), NO docs (P1.M4.T2.S1). [Mode A] each new test has a godoc naming its FR-R7
  clause + why the LANDED tests did not cover it.

---

## Goal

**Feature Goal**: Close the 4 genuine test-coverage gaps for PRD §9.15 FR-R7 per-role generation timeouts
(ResolveRoleTimeout + the config-loading layers + the 480s→120s global default change) that the LANDED
P1.M1/P1.M2 implementation left open — WITHOUT duplicating the ~20 already-passing tests those subtasks
co-implemented. The result is a complete, non-redundant regression net so any future change to the
per-role-timeout resolution precedence (per-role override > planner 480s built-in > 120s global) or to the
config-loading layers (env/flag/file/git) is caught.

**Deliverable**: 4 NEW test functions across 2 files (additive; no production edit):
1. `internal/config/roles_test.go` — +`TestResolveRoleTimeout_MessagePerRoleOverride` (GAP1),
   +`TestResolveRoleTimeout_AllCanonicalRoles` (GAP2), +`TestResolveRoleTimeout_PlannerDefaultFromDefaults` (GAP3).
2. `internal/config/file_test.go` — +`TestMaterializeRoleTimeout_ArbiterRole` (GAP4).

**Success Definition**:
- The 4 new tests pass (`go test ./internal/config/... -run 'ResolveRoleTimeout|MaterializeRoleTimeout_Arbiter' -race -v`).
- They are NOT duplicates: each targets a branch/case the 20 LANDED tests do not cover (see findings §2 for
  the per-gap rationale), and each has a UNIQUE function name (no `func` collision).
- All 20 existing per-role-timeout tests STAY green (the additions are pure-additive; no edit to them).
- `go test ./internal/config/... -race` green; `go vet ./internal/config/...` clean; `gofmt -l` empty on the 2 files.
- The shared validation gates `go test ./internal/decompose/... ./internal/generate/... -race` stay green
  (this item writes ONLY config tests — it cannot affect them; the parallel P1.M3.T2.S1 owns any decompose fix).
- `git status --porcelain` == the 2 test files ONLY (scope guard). NO production code, NO docs.

## User Persona (if applicable)

**Target User**: The Stagecoach maintainer (developer). These are unit tests, not user-facing.
**Use Case**: A maintainer refactors `ResolveRoleTimeout` or a config loader and runs
`go test ./internal/config/...` for fast, deterministic confirmation that per-role timeouts resolve
correctly across all 4 roles and all precedence layers.
**User Journey**: change → `go test ./internal/config/... -race` → green/red signal on the FR-R7 matrix.
**Pain Points Addressed**: The LANDED tests leave the message-role per-role override (the single-commit
path's only active role), the all-4-roles matrix, the explicit Defaults() default-change proof, and the
arbiter materialize case untested. These 4 gaps are exactly the cases most likely to silently break in a
future refactor (e.g. someone "simplifies" ResolveRoleTimeout and accidentally drops the message branch, or
special-cases the planner and forgets the other 3 roles).

## Why

- **PRD §9.15 FR-R7 + §16.1**: "Each role resolves its own timeout independently" with precedence
  `[role.<role>].timeout > built-in (planner 480s) > [defaults].timeout (120s)`. The resolution function,
  the default change, and ALL config layers are LANDED + mostly tested — this item closes the last gaps so
  the FR is fully regression-guarded, not 90%-guarded.
- **Test-layer completeness (PRD §20.1 layer 1)**: §20.1 layer 1 is "Unit — pure functions: config
  precedence resolution." A 90%-covered resolution matrix is a future-bug magnet; the 4 gaps are the 10%.
- **Gap1 is the highest-value hole**: the `message` role is the ONLY active role on the single-commit path
  (the most common invocation). It is currently tested ONLY with Timeout==0 (inherit) — never with its own
  override. If a refactor broke the non-planner per-role-override branch, no LANDED test would catch it.
- **Gap2 matches an existing codebase idiom**: `TestResolveRoleModel_AllCanonicalRoles` (line 108) is the
  established "one table over roleNames proving the resolution matrix" pattern for the model axis. The
  timeout axis has no such twin — 6 scattered ad-hoc tests instead. The consolidated table is the missing
  canonical artifact and the single most readable proof of FR-R7.
- **Gap3 makes the FR-R7 default change explicit**: P1.M2.T2.S1 flipped the global 480s→120s precisely so
  the planner's 480s built-in would be LONGER than the global. That intent is proven functionally but not
  stated in any test name/body. Clause (e) wants it explicit; the one-liner makes "the planner still gets
  480s after the global dropped to 120s" a named, greppable assertion.
- **Bounded scope, no duplication**: 4 additive tests in 2 files. No production code, no docs, no overlap
  with the parallel decompose item or the LANDED tests.

## What

4 NEW unit tests in `internal/config/` (package `config` — same-package white-box, so `roleNames`,
`Defaults()`, `materialize`, `overlay` are all directly reachable). Each targets one genuine gap; none
duplicates the ~20 LANDED tests (verified in findings §1–§2).

### Success Criteria
- [ ] `internal/config/roles_test.go` adds `TestResolveRoleTimeout_MessagePerRoleOverride`: sets
      `cfg.Roles["message"] = RoleConfig{Timeout: 90s}` + `cfg.Timeout=120s`; asserts
      `ResolveRoleTimeout("message", cfg) == 90s` (GAP1 — the message per-role override, currently untested).
- [ ] `internal/config/roles_test.go` adds `TestResolveRoleTimeout_AllCanonicalRoles`: a table over
      `roleNames` with `cfg.Timeout=120s` + `cfg.Roles{planner:600s, stager:60s}` (message/arbiter absent);
      asserts planner→600s, stager→60s, message→120s, arbiter→120s (GAP2 — the consolidated matrix twin of
      `TestResolveRoleModel_AllCanonicalRoles`).
- [ ] `internal/config/roles_test.go` adds `TestResolveRoleTimeout_PlannerDefaultFromDefaults`: asserts
      `Defaults().Timeout == 120s` AND `ResolveRoleTimeout("planner", Defaults()) == 480s` (GAP3 — clause (e)
      explicit: the global is 120s yet the planner resolves to 480s built-in).
- [ ] `internal/config/file_test.go` adds `TestMaterializeRoleTimeout_ArbiterRole`: calls
      `materialize(&fileConfig{Role: map[string]fileRoleConfig{"arbiter": {Timeout: "200s"}}}, 0, 0)`; asserts
      no error + `cfg.Roles["arbiter"].Timeout == 200s` (GAP4 — clause (c) literal: the 4th role in materialize).
- [ ] All 4 are UNIQUE function names (no collision with the 20 LANDED tests — grep guard).
- [ ] The 20 existing per-role-timeout tests STAY GREEN (pure-additive; no edit to any existing test).
- [ ] `go test ./internal/config/... -race` green; `go vet ./internal/config/...` clean; `gofmt -l` empty.
- [ ] `go test ./internal/decompose/... ./internal/generate/... -race` green (shared gate; this item can't
      affect them, but confirm no collateral).
- [ ] `git status --porcelain` == the 2 files. NO production code, NO docs, NO decompose/generate edit.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the COMPLETE inventory of the 20 LANDED tests (with line numbers + what each covers) so the
implementer does NOT duplicate, the exact 4 gaps with per-gap rationale, the LANDED `ResolveRoleTimeout`
contract (precedence + the planner-only `defaultRoleTimeouts`), the same-package test-harness helpers
(`Defaults()`, `roleNames`, `materialize`, `overlay`, `fileRoleConfig`), the verbatim test bodies for all 4
new tests (drop-in), the scope fences (config-only; parallel item owns decompose; LANDED tests are read-only),
and the no-duplicate grep guards.

### Documentation & References

```yaml
# MUST READ — codebase-specific findings for THIS item (the inventory of what ALREADY exists + the 4 gaps).
- docfile: plan/015_b461e4720495/P1M4T1S1/research/findings.md
  why: "§0 THE HEADLINE — ~20 tests already exist (co-implemented in P1.M1/P1.M2); do NOT duplicate. §1 the
        COMPLETE inventory (roles_test.go 6, load_test.go 6, file_test.go 4, git_test.go 3, config_test.go 1)
        with line numbers + per-test coverage. §2 the 4 GENUINE gaps (message per-role override / all-roles
        table / explicit Defaults() / arbiter materialize) + the NOT-GAPS (env/flag/overlay/git already
        covered). §3 the LANDED impl under test. §4 the test-harness helpers. §5 parallel + scope awareness.
        §6 validation."
  critical: "The contract's named tests (TestEnvRoleTimeout/TestFlagRoleTimeout/TestFileRoleTimeout/
             TestOverlayRoleTimeout/TestGitConfigRoleTimeout/TestResolveRoleTimeout) are ALREADY COVERED by
             the LANDED tests under DIFFERENT names (TestLoadEnv_PerRoleTimeout etc.). Writing the contract's
             names would duplicate. Fill ONLY the 4 gaps."

# MUST READ — the function under test (ResolveRoleTimeout) + its contract (LANDED, P1.M2.T1.S1; read-only).
- file: internal/config/roles.go
  why: "ResolveRoleTimeout (line ~140): per-role override (cfg.Roles[role].Timeout != 0) > built-in
        (defaultRoleTimeouts[role]) > cfg.Timeout. defaultRoleTimeouts (line 8): map{'planner': 480s} — the
        ONLY built-in. A non-planner role with no override returns cfg.Timeout. Mirror ResolveRoleModel's
        structure. The 4 new tests assert exactly these branches."
  gotcha: "Do NOT modify roles.go — it is LANDED. This item CONSUMES it. defaultRoleTimeouts is unexported
           (same-package test can read it, but the tests use the observable ResolveRoleTimeout return, not
           the map directly — more robust)."

# MUST READ — the existing ResolveRoleTimeout tests (the 6 LANDED tests — append AFTER them, do NOT edit).
- file: internal/config/roles_test.go
  why: "Lines 195-252: the 6 TestResolveRoleTimeout_* tests. APPEND the 3 new tests after line 252 (after the
        last one, TestResolveRoleTimeout_RolesNilGlobalFallback). Do NOT edit any of the 6. The new tests
        REUSE the same setup idiom: cfg := Defaults(); cfg.Timeout = 120*time.Second; cfg.Roles = map[...]; 
        got := ResolveRoleTimeout(role, cfg); compare. roleNames is package-level (loop it for the table)."
  pattern: "Each existing test: Defaults() → set Timeout (distinct from 480s) → set Roles → ResolveRoleTimeout
            → t.Errorf on mismatch with a [bracketed rationale]. Clone this shape verbatim for the 3 new ones."
  critical: "GAP1: NONE of the 6 sets cfg.Roles['message'].Timeout to a non-zero value (line 231 uses Timeout:0).
             GAP2: NONE loops roleNames for timeout (unlike TestResolveRoleModel_AllCanonicalRoles at line 108).
             GAP3: NONE calls ResolveRoleTimeout with the literal Defaults() (line 211 sets cfg.Timeout=120s
             explicitly — functionally identical, but not the explicit clause-(e) proof)."

# MUST READ — the existing materialize test + the materialize seam (file.go) for GAP4.
- file: internal/config/file_test.go
  why: "TestMaterializeRoleTimeout (line 1282): table-tests [role.planner].timeout (duration/bare-int/2m/empty/
        omitted/malformed) + a two_roles planner/stager subtest. APPEND TestMaterializeRoleTimeout_ArbiterRole
        as a NEW standalone test after it (do NOT edit the existing table). The new test calls materialize
        DIRECTLY with a [role.arbiter] map — the SAME seam, the 4th role (currently only planner+stager)."
  pattern: "materialize(&fileConfig{Role: map[string]fileRoleConfig{<role>: {Timeout: '<dur>'}}}, 0, 0) →
            (cfg, err); assert err==nil + cfg.Roles[<role>].Timeout == want. fileRoleConfig.Timeout is a STRING
            (parsed by materialize via parseTimeout, which accepts '200s' AND bare '200')."

# CONTEXT — the RoleConfig struct + Defaults() (test setup correctness).
- file: internal/config/config.go
  why: "RoleConfig{Provider, Model, Reasoning string; Timeout time.Duration}; Config.Roles map[string]RoleConfig;
        Defaults() returns Roles==nil + Timeout==120s. In a test, cfg.Roles = map[string]RoleConfig{...} (assign
        the WHOLE map — Defaults() leaves Roles nil, and index-assign into a nil map panics)."
  critical: "Defaults().Roles is nil. The table test (GAP2) and override test (GAP1) MUST assign the whole map,
             never cfg.Roles['x'].Timeout = … on a Defaults() cfg."

# CONTEXT — the all-roles idiom to clone for GAP2 (the model-axis twin that already exists).
- file: internal/config/roles_test.go
  why: "TestResolveRoleModel_AllCanonicalRoles (line 108): the canonical 'one table over roleNames proving the
        resolution matrix' idiom. GAP2's TestResolveRoleTimeout_AllCanonicalRoles is its EXACT twin for the
        timeout axis — same loop `for _, role := range roleNames`, same want-map structure."

# CONTEXT — PRD §9.15 FR-R7 + §16.1 (the requirement) + §20.1 layer 1 (the test layer).
- docfile: plan/015_b461e4720495/prd_snapshot.md
  section: "§9.15 FR-R7 (per-role timeout) + §16.1 (resolution order: per-role > planner-480s-built-in > 120s-global) + §20.1 layer 1 (unit pure-fn config-precedence tests)"
  why: "Confirms the precedence this item regression-guards + the planner-480s-built-in asymmetry."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  roles.go            # READ-ONLY — ResolveRoleTimeout + defaultRoleTimeouts (LANDED P1.M2.T1.S1)
  config.go           # READ-ONLY — RoleConfig{Timeout}, Config.Roles, Defaults() (Timeout=120s, Roles=nil)
  load.go             # READ-ONLY — roleNames, setRoleTimeout, parseTimeout, env/flag _TIMEOUT branches (LANDED P1.M1.T2)
  file.go             # READ-ONLY — fileRoleConfig{Timeout}, materialize, overlay (LANDED P1.M1.T1)
  git.go              # READ-ONLY — stagecoach.role.<role>.timeout reading (LANDED P1.M1.T2.S3)
  roles_test.go       # EDIT (APPEND 3 tests after line 252)
  file_test.go        # EDIT (APPEND 1 test after TestMaterializeRoleTimeout)
  load_test.go        # READ-ONLY — TestLoadEnv/LoadFlags_PerRoleTimeout etc. already cover env/flag (NOT a gap)
  git_test.go         # READ-ONLY — TestLoadGitConfig_PerRoleTimeout already covers git (NOT a gap)
  config_test.go      # READ-ONLY — TestDefaults already asserts Timeout==120s (NOT a gap)
Makefile              # READ-ONLY — test=line 70 (-race); lint=line 103
```

### Desired Codebase tree with files to be added/modified

```bash
internal/config/roles_test.go   # EDIT — APPEND 3 tests (MessagePerRoleOverride, AllCanonicalRoles, PlannerDefaultFromDefaults)
internal/config/file_test.go    # EDIT — APPEND 1 test (MaterializeRoleTimeout_ArbiterRole)
# NOTHING ELSE. No production code (roles.go/config.go/load.go/file.go/git.go are READ-ONLY). No edit to
# load_test.go/git_test.go/config_test.go (their per-role-timeout coverage is ALREADY complete). No edit to
# any internal/decompose/* or internal/generate/* file (P1.M3.T2.S1 is parallel + owns those). No docs.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (do NOT duplicate the ~20 LANDED tests): the contract's named tests are ALREADY COVERED under
// different names — TestLoadEnv_PerRoleTimeout (env), TestLoadFlags_PerRoleTimeout (flag),
// TestOverlayRolesFieldMerge_Timeout (overlay), TestLoadGitConfig_PerRoleTimeout (git),
// TestResolveRoleTimeout_* (6 resolution tests), TestDefaults (Timeout==120s). Writing the contract's names
// duplicates. This item adds ONLY the 4 gap tests. (See findings §1 for the full inventory with line numbers.)

// CRITICAL (Defaults().Roles is nil — assign the WHOLE map): config.Defaults() returns Config{Roles: nil}.
// `cfg.Roles["message"].Timeout = 90s` on a Defaults() cfg PANICS (nil-map index-assign). Always:
//   cfg.Roles = map[string]config.RoleConfig{"message": {Timeout: 90 * time.Second}}

// CRITICAL (use cfg.Timeout=120s DISTINCT from the 480s built-in in assertions): so a pass/fail is
// unambiguous. If a test left cfg.Timeout at 0 and asserted planner==480s, a bug that returned cfg.Timeout
// (0) would also fail loudly — but the rationale in the error message should name the 480s-vs-120s contrast.
// The LANDED tests all set cfg.Timeout=120s explicitly; the new tests follow suit.

// GOTCHA (roleNames is package-level + same-package tests can loop it): `roleNames` is defined in load.go
// (line 24) as []string{"planner","stager","message","arbiter"}. roles_test.go is package config → it can
// loop roleNames directly (TestResolveRoleModel_AllCanonicalRoles at line 108 does exactly this). GAP2 reuses it.

// GOTCHA (fileRoleConfig.Timeout is a STRING, parsed by materialize): in file_test.go, the per-role timeout
// is a TOML string ("200s") that materialize parses via parseTimeout (accepts "200s" AND bare "200"). The
// test constructs fileRoleConfig{Timeout: "200s"} (string), NOT a time.Duration. Assert the PARSED result
// (cfg.Roles["arbiter"].Timeout == 200*time.Duration) is the Duration.

// GOTCHA (materialize signature is (fc *fileConfig, globalMaxDiff, globalMaxMd int)): the existing
// TestMaterializeRoleTimeout calls materialize(fc, 0, 0). The 2 int args are unrelated to timeout (they're
// max_diff_bytes/max_md_lines global fallbacks). Pass 0, 0 — same as the existing tests.

// GOTCHA (the planner is the ONLY role with a built-in): ResolveRoleTimeout returns 480s for planner even
// when cfg.Roles is nil; stager/message/arbiter have NO built-in → they return cfg.Timeout when no override.
// GAP2's table asserts exactly this asymmetry (planner absent from cfg.Roles still → would be 480s, but
// GAP2 SETS planner to 600s to test the override-beats-built-in branch; message/arbiter absent → 120s global).
```

## Implementation Blueprint

### Data models and structure

None NEW. The tests construct `config.Config` (via `Defaults()`) + `config.RoleConfig{Timeout: …}` +
`config.fileRoleConfig{Timeout: "…"}` (same-package) and call the LANDED `ResolveRoleTimeout` / `materialize`.
No new types, no production edits.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: APPEND 3 tests to internal/config/roles_test.go (after line 252, the last TestResolveRoleTimeout_*)
  - These APPEND to the existing "--- ResolveRoleTimeout tests (FR-R7 ...) ---" section. Do NOT edit the 6
    LANDED tests. Each new test reuses the existing setup idiom (Defaults() → set Timeout=120s → set Roles →
    ResolveRoleTimeout → t.Errorf with [rationale]).
  - STEP 1a — ADD TestResolveRoleTimeout_MessagePerRoleOverride (GAP1 — the clearest gap):
      // TestResolveRoleTimeout_MessagePerRoleOverride closes GAP1: NO LANDED test sets a NON-ZERO
      // cfg.Roles["message"].Timeout. (TestResolveRoleTimeout_FieldMergeTimeoutOnly uses message with
      // Timeout==0; _PerRoleOverride uses planner.) The message role is the ONLY active role on the
      // single-commit path, so its per-role override branch is the highest-value untested case (FR-R7).
      func TestResolveRoleTimeout_MessagePerRoleOverride(t *testing.T) {
      	cfg := Defaults()
      	cfg.Timeout = 120 * time.Second // distinct from any per-role value
      	cfg.Roles = map[string]RoleConfig{"message": {Timeout: 90 * time.Second}}
      	got := ResolveRoleTimeout("message", cfg)
      	if got != 90*time.Second {
      		t.Errorf("ResolveRoleTimeout(message) = %v, want 90s [message has no built-in; per-role override beats 120s global]", got)
      	}
      }
  - STEP 1b — ADD TestResolveRoleTimeout_AllCanonicalRoles (GAP2 — the consolidated matrix twin):
      // TestResolveRoleTimeout_AllCanonicalRoles closes GAP2: the timeout axis has no "all roles" table,
      // unlike TestResolveRoleModel_AllCanonicalRoles (line 108) for the model axis. One table over roleNames
      // proves the full FR-R7 resolution matrix: planner→per-role override (beats the 480s built-in);
      // stager→per-role override (beats the 120s global); message/arbiter (absent)→120s global.
      func TestResolveRoleTimeout_AllCanonicalRoles(t *testing.T) {
      	cfg := Defaults()
      	cfg.Timeout = 120 * time.Second // distinct from 480s so the planner built-in is unambiguous
      	cfg.Roles = map[string]RoleConfig{
      		"planner": {Timeout: 600 * time.Second}, // override beats the 480s built-in
      		"stager":  {Timeout: 60 * time.Second},  // override beats the 120s global
      		// message, arbiter: no entry → no built-in → global 120s
      	}
      	want := map[string]time.Duration{
      		"planner": 600 * time.Second, // per-role override
      		"stager":  60 * time.Second,  // per-role override
      		"message": 120 * time.Second, // no built-in, no override ⇒ global
      		"arbiter": 120 * time.Second, // no built-in, no override ⇒ global
      	}
      	for _, role := range roleNames { // roleNames: load.go package-level canonical list
      		got := ResolveRoleTimeout(role, cfg)
      		if got != want[role] {
      			t.Errorf("ResolveRoleTimeout(%s) = %v, want %v", role, got, want[role])
      		}
      	}
      }
  - STEP 1c — ADD TestResolveRoleTimeout_PlannerDefaultFromDefaults (GAP3 — clause (e) explicit):
      // TestResolveRoleTimeout_PlannerDefaultFromDefaults closes GAP3: clause (e) wants the FR-R7 default
      // change verified EXPLICITLY. Defaults() has Timeout=120s (P1.M2.T2.S1 flipped the global 480s→120s),
      // yet the planner resolves to its 480s BUILT-IN — NOT the 120s global. _PlannerBuiltinBeatsGlobal is
      // functionally identical (it sets cfg.Timeout=120s explicitly) but does not call ResolveRoleTimeout
      // with the literal Defaults(); this one-liner makes "the planner still gets 480s after the global
      // dropped to 120s" a named, greppable assertion.
      func TestResolveRoleTimeout_PlannerDefaultFromDefaults(t *testing.T) {
      	cfg := Defaults()
      	if cfg.Timeout != 120*time.Second {
      		t.Fatalf("Defaults().Timeout = %v, want 120s (the FR-R7 global default)", cfg.Timeout)
      	}
      	if got := ResolveRoleTimeout("planner", cfg); got != 480*time.Second {
      		t.Errorf("ResolveRoleTimeout(planner, Defaults()) = %v, want 480s (built-in beats 120s global)", got)
      	}
      }
  - NAMING: TestResolveRoleTimeout_<Scenario> (matches the existing _PerRoleOverride/_PlannerBuiltinBeatsGlobal
    convention). UNIQUE names — no collision with the 6 LANDED (grep guard).
  - PLACEMENT: immediately after TestResolveRoleTimeout_RolesNilGlobalFallback (the last one, line ~252),
    keeping the "--- ResolveRoleTimeout tests ---" section contiguous.

Task 2: APPEND 1 test to internal/config/file_test.go (after TestMaterializeRoleTimeout, ~line 1350)
  - ADD TestMaterializeRoleTimeout_ArbiterRole (GAP4 — clause (c) literal):
      // TestMaterializeRoleTimeout_ArbiterRole closes GAP4: TestMaterializeRoleTimeout covers planner (table)
      // + planner/stager (two_roles subtest) but NOT arbiter. materialize IS role-agnostic (loops the Role
      // map), but clause (c) names [role.arbiter].timeout="200s" explicitly, and pinning the 4th role is
      // cheap insurance against a future role-specific regression in the parse path.
      func TestMaterializeRoleTimeout_ArbiterRole(t *testing.T) {
      	fc := &fileConfig{Role: map[string]fileRoleConfig{"arbiter": {Timeout: "200s"}}}
      	cfg, err := materialize(fc, 0, 0)
      	if err != nil {
      		t.Fatalf("materialize([role.arbiter].timeout='200s'): %v", err)
      	}
      	if cfg == nil || cfg.Roles == nil {
      		t.Fatalf("materialize: cfg=%v, want non-nil Config with Roles populated", cfg)
      	}
      	if got := cfg.Roles["arbiter"].Timeout; got != 200*time.Second {
      		t.Errorf("Roles[arbiter].Timeout = %v, want 200s ([role.arbiter].timeout parsed via parseTimeout)", got)
      	}
      }
  - NAMING: TestMaterializeRoleTimeout_ArbiterRole (a NEW standalone test — do NOT edit the existing
    TestMaterializeRoleTimeout table; appending a subtest inside it risks touching the table's structure).
  - PLACEMENT: immediately after TestMaterializeRoleTimeout's closing brace.
  - GOTCHA: fileRoleConfig.Timeout is a STRING ("200s"); materialize parses it via parseTimeout. Pass
    materialize(fc, 0, 0) — the 2 int args are max_diff/max_md global fallbacks, unrelated to timeout.
    The "time" + "strings" imports are ALREADY present in file_test.go (used by the existing tests).

Task 3: VERIFY — build, vet, format, full regression, lint, no-duplicate + scope guards
  - go test ./internal/config/... -run 'ResolveRoleTimeout|MaterializeRoleTimeout_Arbiter' -race -v  # the 4 new + 6 LANDED
  - go test ./internal/config/... -race                          # full config regression (all ~20 existing + 4 new)
  - go vet ./internal/config/... ; gofmt -l internal/config/roles_test.go internal/config/file_test.go  # empty
  - go test ./internal/decompose/... ./internal/generate/... -race  # shared gate (this item can't affect; confirm)
  - make test ; make lint
  - grep guards (see Validation Loop Level 4): no-duplicate + scope.
```

### Implementation Patterns & Key Details

```go
// PATTERN (the 3 roles_test.go tests — clone of the LANDED TestResolveRoleTimeout_* idiom):
//   Defaults() → set cfg.Timeout=120s (DISTINCT from 480s) → set cfg.Roles (WHOLE map) → ResolveRoleTimeout → t.Errorf[rationale].
func TestResolveRoleTimeout_MessagePerRoleOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second
	cfg.Roles = map[string]RoleConfig{"message": {Timeout: 90 * time.Second}} // WHOLE map (Defaults leaves Roles nil)
	got := ResolveRoleTimeout("message", cfg)
	if got != 90*time.Second {
		t.Errorf("ResolveRoleTimeout(message) = %v, want 90s [...]", got)
	}
}

// PATTERN (the all-roles table — clone of TestResolveRoleModel_AllCanonicalRoles line 108):
for _, role := range roleNames { // package-level canonical list (load.go:24)
	got := ResolveRoleTimeout(role, cfg)
	if got != want[role] { t.Errorf("ResolveRoleTimeout(%s) = %v, want %v", role, got, want[role]) }
}

// PATTERN (the file_test.go materialize seam — clone of TestMaterializeRoleTimeout line 1282):
fc := &fileConfig{Role: map[string]fileRoleConfig{"arbiter": {Timeout: "200s"}}} // STRING (parsed by materialize)
cfg, err := materialize(fc, 0, 0) // (fc, globalMaxDiff, globalMaxMd) — the 2 ints are unrelated to timeout
if err != nil { t.Fatalf(...) }
if got := cfg.Roles["arbiter"].Timeout; got != 200*time.Second { t.Errorf(...) } // assert the PARSED Duration
```

### Integration Points

```yaml
TEST PACKAGES (internal/config — the ONLY package touched):
  - roles_test.go: APPEND 3 tests (MessagePerRoleOverride / AllCanonicalRoles / PlannerDefaultFromDefaults).
  - file_test.go: APPEND 1 test (MaterializeRoleTimeout_ArbiterRole).
NO production code. NO edit to roles.go/config.go/load.go/file.go/git.go (LANDED, read-only).
NO edit to load_test.go/git_test.go/config_test.go (their per-role-timeout coverage is ALREADY complete).
NO edit to any internal/decompose/* or internal/generate/* file (P1.M3.T2.S1 is parallel + owns those).
NO docs (P1.M4.T2.S1 owns README/docs sync; contract: "DOCS: none — tests are not user-facing docs").
SCOPE FENCES:
  - Touches ONLY: internal/config/roles_test.go + internal/config/file_test.go.
  - Does NOT touch: any .go non-test file, load_test.go/git_test.go/config_test.go, internal/decompose/*,
    internal/generate/*, root.go, cmd/*, go.mod, or any PRD/task file.
  - Adds NO production type/flag/import. The test files already import "time" + "testing" (roles_test.go)
    and "time" + "testing" + "strings" (file_test.go) — NO new import needed.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (the test files compile — the new tests use only LANDED, same-package symbols).
go build ./...
# Expected: clean.

# Vet the config package.
go vet ./internal/config/...
# Expected: clean.

# Format the 2 touched files.
gofmt -l internal/config/roles_test.go internal/config/file_test.go
# Expected: empty. If listed: gofmt -w <those files>.

# Lint.
make lint   # golangci-lint (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. (The 4 new tests all use their results; no unused-symbol findings.)

# Scope guard: ONLY the 2 test files changed.
git status --porcelain
# Expected: internal/config/roles_test.go + internal/config/file_test.go ONLY. ZERO production files,
#           ZERO other test files, ZERO docs.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 4 new tests + the 6 LANDED ResolveRoleTimeout tests + the materialize tests.
go test ./internal/config/... -run 'ResolveRoleTimeout|MaterializeRoleTimeout' -race -v
# Expected: ALL PASS —
#   NEW TestResolveRoleTimeout_MessagePerRoleOverride: message {Timeout:90s} → 90s (GAP1).
#   NEW TestResolveRoleTimeout_AllCanonicalRoles: planner→600s, stager→60s, message→120s, arbiter→120s (GAP2).
#   NEW TestResolveRoleTimeout_PlannerDefaultFromDefaults: Defaults().Timeout==120s + planner→480s (GAP3).
#   NEW TestMaterializeRoleTimeout_ArbiterRole: [role.arbiter].timeout="200s" → 200s (GAP4).
#   (6 LANDED TestResolveRoleTimeout_* + TestMaterializeRoleTimeout + the load/file/git/env tests stay GREEN.)

# Full config regression (all ~20 existing per-role-timeout tests + the 4 new + everything else).
go test ./internal/config/... -race
# Expected: green. The additions are pure-additive; no existing test is edited.

# Full race suite (prove no collateral outside config).
make test
# Expected: green.
```

### Level 3: Integration Testing (System Validation)

```bash
# This item is config-unit tests — no CLI/integration surface. The shared gate confirms the parallel
# decompose item's package (which this item does NOT touch) still builds+tests green alongside.
go test ./internal/decompose/... ./internal/generate/... -race
# Expected: green. (P1.M3.T2.S1 owns any decompose fix; this item writes ONLY config tests and cannot
#           affect decompose/generate. If this is RED, it is the PARALLEL item's in-flight edit, NOT this
#           item's — re-run after that item lands. This item's gate is ./internal/config/... only.)
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the 4 new tests EXIST with UNIQUE names (no collision with the 20 LANDED).
grep -nE 'func TestResolveRoleTimeout_MessagePerRoleOverride|func TestResolveRoleTimeout_AllCanonicalRoles|func TestResolveRoleTimeout_PlannerDefaultFromDefaults' internal/config/roles_test.go
grep -nE 'func TestMaterializeRoleTimeout_ArbiterRole' internal/config/file_test.go
# Expect: 1 hit each (3 in roles_test.go, 1 in file_test.go).

# Guard 2: NO duplicate of the contract's already-covered named tests (the LANDED tests cover them).
# (The contract's TestEnvRoleTimeout/TestFlagRoleTimeout/TestFileRoleTimeout/TestOverlayRoleTimeout/
#  TestGitConfigRoleTimeout must NOT be added — they're covered by TestLoadEnv_PerRoleTimeout etc.)
grep -nE 'func TestEnvRoleTimeout|func TestFlagRoleTimeout|func TestFileRoleTimeout\b|func TestOverlayRoleTimeout\b|func TestGitConfigRoleTimeout\b' internal/config/*_test.go && echo "FAIL: duplicated a contract-named test (already covered)" || echo "OK: no duplicated contract-named tests"
# Expect: OK (zero hits — the env/flag/file-overlay/git coverage is the LANDED tests' job).

# Guard 3: GAP1 is genuinely NEW — no LANDED test sets a non-zero message timeout via ResolveRoleTimeout.
# (Confirm the new test is the ONLY place cfg.Roles["message"] has a non-zero Timeout in a ResolveRoleTimeout call.)
grep -n 'Roles\["message"\].*Timeout\|"message": {Timeout' internal/config/roles_test.go
# Expect: the new TestResolveRoleTimeout_MessagePerRoleOverride (90s) + TestResolveRoleTimeout_AllCanonicalRoles
#         (message absent → global) + the LANDED _FieldMergeTimeoutOnly (Timeout:0). The 90s is the new branch.

# Guard 4: GAP2 loops roleNames (the all-roles idiom — mirrors TestResolveRoleModel_AllCanonicalRoles).
grep -n 'for _, role := range roleNames' internal/config/roles_test.go
# Expect: ≥2 hits (the LANDED TestResolveRoleModel_AllCanonicalRoles + the NEW TestResolveRoleTimeout_AllCanonicalRoles).

# Guard 5: GAP3 calls ResolveRoleTimeout with the literal Defaults() (clause (e) explicit).
grep -n 'ResolveRoleTimeout("planner", cfg)' internal/config/roles_test.go | grep -i default
# (Inspect TestResolveRoleTimeout_PlannerDefaultFromDefaults: it asserts Defaults().Timeout==120s THEN ResolveRoleTimeout.)

# Guard 6: NO production code edited.
git diff --name-only | grep -vE '_test\.go$' | grep -q . && echo "FAIL: non-test file edited" || echo "OK: only test files"
git diff --name-only | grep -qE '^internal/config/(roles|config|load|file|git)\.go$' && echo "FAIL: production config file edited" || echo "OK: production config untouched"

# Guard 7: scope — only the 2 test files.
git status --porcelain
# Expect: internal/config/roles_test.go + internal/config/file_test.go ONLY.
git diff --name-only | grep -vE '^internal/config/(roles_test|file_test)\.go$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./internal/config/...` clean; `gofmt -l` empty on the 2 files
- [ ] `make lint` zero errors (all 4 new tests use their results; no unused symbols)
- [ ] `go test ./internal/config/... -run 'ResolveRoleTimeout|MaterializeRoleTimeout' -race -v` green (4 new + LANDED)
- [ ] `go test ./internal/config/... -race` green (full config regression — ~20 existing + 4 new)
- [ ] `make test` (full race suite) green

### Feature Validation (the 4 gaps closed)
- [ ] GAP1: `TestResolveRoleTimeout_MessagePerRoleOverride` — message {Timeout:90s} → 90s (beats 120s global) (grep guard 3)
- [ ] GAP2: `TestResolveRoleTimeout_AllCanonicalRoles` — table over roleNames: planner→600s, stager→60s, message/arbiter→120s (grep guard 4)
- [ ] GAP3: `TestResolveRoleTimeout_PlannerDefaultFromDefaults` — Defaults().Timeout==120s + ResolveRoleTimeout("planner", Defaults())==480s (grep guard 5)
- [ ] GAP4: `TestMaterializeRoleTimeout_ArbiterRole` — [role.arbiter].timeout="200s" → 200s
- [ ] All 4 are UNIQUE names (no collision) (grep guard 1) and do NOT duplicate the contract's covered cases (grep guard 2)
- [ ] All 20 LANDED per-role-timeout tests stay GREEN (pure-additive; no edit to them)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/config/roles_test.go` + `internal/config/file_test.go` (grep guards 6–7)
- [ ] NO production code edit (roles.go/config.go/load.go/file.go/git.go read-only) (grep guard 6)
- [ ] NO edit to load_test.go/git_test.go/config_test.go (their coverage is ALREADY complete) (grep guard 2)
- [ ] NO edit to internal/decompose/* or internal/generate/* (P1.M3.T2.S1 is parallel + owns those)
- [ ] NO docs (P1.M4.T2.S1 owns README/docs; contract "DOCS: none")
- [ ] NO new import (time/testing already in roles_test.go; time/testing/strings already in file_test.go)

### Code Quality & Docs
- [ ] Each new test has a [Mode A] godoc naming its FR-R7 clause + WHY the LANDED tests did not cover it
      (GAP1: message is the single-commit path's only role; GAP2: the model-axis table's missing twin;
      GAP3: the explicit default-change proof; GAP4: the 4th role in materialize)
- [ ] Tests follow the existing `Defaults() → set Timeout=120s → set Roles (whole map) → ResolveRoleTimeout`
      idiom and the `materialize(fc, 0, 0)` seam — no new fixtures or helpers
- [ ] Contract honored: "DOCS: none — tests are not user-facing docs"

---

## Anti-Patterns to Avoid

- ❌ Don't duplicate the ~20 LANDED tests. The contract's named tests (TestEnvRoleTimeout / TestFlagRoleTimeout
  / TestFileRoleTimeout / TestOverlayRoleTimeout / TestGitConfigRoleTimeout / TestResolveRoleTimeout) are
  ALREADY COVERED by the LANDED tests under different names (TestLoadEnv_PerRoleTimeout,
  TestLoadFlags_PerRoleTimeout, TestOverlayRolesFieldMerge_Timeout, TestLoadGitConfig_PerRoleTimeout,
  TestResolveRoleTimeout_*, TestDefaults). Read findings §1 BEFORE writing. Fill ONLY the 4 gaps (grep guard 2).
- ❌ Don't index-assign into a nil Roles map. `config.Defaults()` returns `Config{Roles: nil}`. In a test,
  `cfg.Roles["message"].Timeout = 90s` PANICS. Assign the whole map: `cfg.Roles = map[string]RoleConfig{"message": {Timeout: 90s}}`.
- ❌ Don't edit any of the 6 LANDED `TestResolveRoleTimeout_*` tests or `TestMaterializeRoleTimeout`. APPEND
  the new tests after them. Editing a LANDED test risks breaking its existing assertion and is out of scope.
- ❌ Don't add the contract's named tests "for completeness." They are pure duplication (the LANDED tests are
  MORE thorough than the contract's one-liners — e.g. TestLoadEnv_PerRoleTimeout tests BOTH duration + bare-int
  forms + an unset role; the contract's "STAGECOACH_PLANNER_TIMEOUT=600s" is a strict subset). Adding them
  bloats the suite with no new coverage and risks future maintainers wondering why two tests cover the same thing.
- ❌ Don't modify production code. roles.go / config.go / load.go / file.go / git.go are LANDED (P1.M1/P1.M2)
  and READ-ONLY. This item CONSUMES them via tests only (grep guard 6). If a test FAILS, the bug is in the
  test's expectation, not the LANDED impl — fix the test.
- ❌ Don't touch internal/decompose/* or internal/generate/*. The contract's validation line
  "go test ./internal/decompose/... ./internal/generate/..." is the PARALLEL P1.M3.T2.S1 item's responsibility
  (it FIXes TestCallPlanner_Timeout + adds 3 behavioral proofs there). This item writes ONLY config tests and
  cannot affect those packages. If `go test ./internal/decompose/...` is RED, it's the parallel item's in-flight
  edit — not this item's.
- ❌ Don't conflate GAP2's "message/arbiter absent → global" with "the planner built-in". In GAP2's table,
  message and arbiter have NO cfg.Roles entry AND NO built-in → they resolve to cfg.Timeout (120s). Only the
  planner has a built-in (480s) — but GAP2 SETS planner to 600s to test override-beats-built-in. Do not assert
  message/arbiter == 480s (they have no built-in).
- ❌ Don't add docs. The contract says "DOCS: none — tests are not user-facing docs." The README/docs sync for
  per-role timeouts is P1.M4.T2.S1 (a separate, explicitly-planned task).
- ❌ Don't add a new import. roles_test.go already imports "time" + "testing"; file_test.go already imports
  "time" + "testing" + "strings". The 4 new tests use only those + same-package symbols (Defaults, roleNames,
  RoleConfig, ResolveRoleTimeout, fileConfig, fileRoleConfig, materialize). A stray new import is an
  unused-import compile error.
- ❌ Don't "improve" the existing tests while you're in the file. The 6 LANDED TestResolveRoleTimeout_* tests
  and TestMaterializeRoleTimeout are correct and tested. Append only. Any refactor of them is out of scope and
  risks the LANDED behavior the P1.M1/P1.M2 PRPs pinned.
