name: "P1.M1.T1.S2 — Add per-role timeout field-merge to the overlay function (FR-R7)"
description: >
  Add the `if rc.Timeout != 0 { existing.Timeout = rc.Timeout }` branch to the per-role field-merge
  loop inside `overlay()` (internal/config/file.go) so a higher config layer's `[role.<role>].timeout`
  overrides a lower layer's without erasing that role's provider/model/reasoning. The branch mirrors
  the proven `Config.Timeout != 0` global overlay guard already in the SAME function. Consumes the
  `RoleConfig.Timeout time.Duration` field produced by sibling P1.M1.T1.S1. One production line + a
  focused regression test; internal merge logic only — no docs, no consumer, no default change.

---

## Goal

**Feature Goal**: Make the config-layer overlay correctly field-merge the per-role `Timeout` (a
`time.Duration`, 0 = inherit global) introduced by P1.M1.T1.S1, so that across the file/git config
layers a higher layer's `[role.planner].timeout = "480s"` overrides a lower layer's
`[role.planner].timeout = "300s"` — while preserving that role's lower-layer provider/model/reasoning
(per-role FIELD-MERGE, PRD §16.4 FR-R3), and an omitted/zero timeout inherits the lower layer.

**Deliverable**: (1) ONE production branch added to the per-role loop of `overlay()` in
`internal/config/file.go` (after the `Reasoning` branch, before `dst.Roles[role] = existing`); (2) a
focused regression test (`TestOverlayRolesFieldMerge_Timeout`) proving: higher-layer timeout wins,
zero/omitted timeout does NOT clobber a lower-layer timeout, and the timeout branch does not erase
sibling provider/model/reasoning fields.

**Success Definition**:
- `go build ./...` compiles clean (consumes S1's `RoleConfig.Timeout time.Duration`).
- `overlay(dst, src)` merges `src.Roles[role].Timeout` into `dst` when non-zero (`!= 0`), exactly
  mirroring the global `Config.Timeout` overlay guard in the same function.
- A higher layer setting ONLY `[role.planner].timeout` does NOT clobber a lower layer's
  `[role.planner].provider`/`.model`/`.reasoning` (regression guard — the FR-R3 field-merge invariant).
- A higher layer that OMITS timeout (zero) leaves a lower layer's non-zero timeout intact.
- `make test` + `make lint` pass.

## User Persona (if applicable)

**Target User**: Stagecoach maintainers (this is internal merge plumbing; no user-facing surface).

**Use Case**: A user sets `[role.planner].timeout = "300s"` globally and `timeout = "480s"` in a
per-repo `.stagecoach.toml`; the repo value wins for the planner role, while the global
`[role.planner].provider`/`.model` survive (they are not re-stated in the repo file).

**Pain Points Addressed**: Without S2, a per-role timeout in a higher config layer is SILENTLY DROPPED
at overlay (S1 parses it into the layer's `*Config`, but overlay never copies it onto the resolved
config). The user's explicit `timeout` is ignored — the same "only-non-zero-propagates gap" class the
overlay already closes for every other field.

## Why

- **FR-R7 / §9.15 / §16.1 / §16.4**: Per-role timeouts resolve across the standard 7-layer precedence.
  The file layers (global TOML → repo TOML) and the git-config layer all merge through `overlay()`.
  S1 made `[role.<role>].timeout` DECODABLE and PARSED into each layer's `*Config`; S2 makes that value
  SURVIVE the overlay into the final resolved `Config`. Until S2 lands, a repo-file per-role timeout is
  dropped at overlay — expected per S1's boundary, fixed by S2.
- **Why it is a tiny, surgical subtask**: `overlay()` already field-merges Provider/Model/Reasoning per
  role with non-empty-string guards, and already merges the global `Config.Timeout` with a `!= 0`
  guard. S2 is the per-role application of the EXACT same, already-proven discipline — one branch.
- **Complementary to S1, non-overlapping**: S1 owns the structs + materialize parse + signature; S2
  owns the overlay merge. Neither touches env/flag/git wiring (P1.M1.T2), resolution
  (P1.M2.T1), the default change (P1.M2.T2), the call sites (P1.M3), or docs (P1.M4.T2).

## What

**User-visible behavior**: None yet on its own (internal merge). Combined with the rest of the FR-R7
milestone, a `[role.<role>].timeout` set in a higher config layer overrides a lower layer's for that
role, field-by-field. This subtask's observable effect is at the unit-test level (overlay direct call).

**Technical change (one production line + test):**
1. In `overlay()`'s per-role loop, after the `if rc.Reasoning != "" { existing.Reasoning = rc.Reasoning }`
   branch and before `dst.Roles[role] = existing`, add:
   ```go
   if rc.Timeout != 0 {
       existing.Timeout = rc.Timeout
   }
   ```
2. Update the loop's preceding comment to mention the timeout field-merge (duration-zero = inherit,
   mirrors the global `Config.Timeout` overlay guard).
3. Add `TestOverlayRolesFieldMerge_Timeout` mirroring `TestOverlayRolesFieldMerge`.

### Success Criteria
- [ ] `overlay()` per-role loop has the `if rc.Timeout != 0 { existing.Timeout = rc.Timeout }` branch.
- [ ] Higher-layer timeout (non-zero) overrides lower-layer timeout (unit test).
- [ ] Higher layer setting ONLY timeout preserves lower-layer provider/model/reasoning (field-merge guard).
- [ ] Higher layer OMITTING timeout (zero) does NOT clobber a lower-layer non-zero timeout (`!= 0` guard).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l`, `make lint`, `make test` all pass.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact function, the exact loop, the exact branch to mirror (the global `Config.Timeout`
guard in the SAME function), the `!= 0` vs `!= nil` discipline rationale, the S1 line-drift caveat,
the test to mirror (with verbatim structure), and explicit scope boundaries against 5 sibling subtasks.

### Documentation & References

```yaml
- file: internal/config/file.go
  why: "THE change site. overlay(dst, src *Config) — the per-role field-merge loop. Add the Timeout
        branch after the Reasoning branch."
  pattern: "Per-role loop: `for role, rc := range src.Roles { existing := dst.Roles[role];
            if rc.Provider != \"\" {...} if rc.Model != \"\" {...} if rc.Reasoning != \"\" {...}
            dst.Roles[role] = existing }`. Global Config.Timeout mirror in the SAME function:
            `if src.Timeout != 0 { dst.Timeout = src.Timeout }`."
  critical: "ANCHOR ON STRUCTURE, NOT LINE NUMBERS. S1 (P1.M1.T1.S1) is being implemented IN PARALLEL
            right now and rewrites the materialize loop ABOVE overlay to field-by-field, so overlay +
            the per-role loop are a MOVING TARGET (observed drifting ~28 lines during this research).
            Do NOT trust any absolute line number. Grep `for role, rc := range src.Roles` to find the
            loop, then the `if rc.Reasoning != \"\"` branch; insert the Timeout branch immediately
            after it, before `dst.Roles[role] = existing`. The global Config.Timeout mirror guard is
            `grep -n 'if src.Timeout != 0' internal/config/file.go`."

- file: internal/config/file.go
  why: "The MIRROR pattern: the global Config.Timeout overlay guard (`!= 0`, duration-non-zero-wins).
        S2's per-role branch is byte-identical in discipline."
  pattern: "`if src.Timeout != 0 { dst.Timeout = src.Timeout }` and `if src.HookTimeout != 0 { ... }`
            — both value-type (time.Duration) overlay guards using != 0. Timeout is NOT a pointer."

- file: internal/config/file_test.go
  why: "TestOverlayRolesFieldMerge (the FR-R3 regression guard) is the test to MIRROR/EXTEND. It uses
        direct Config{...} literals + overlay(dst, src) and asserts field-merge preservation +
        override + src-only-role-add + untouched-survival."
  pattern: "dst := &Config{Roles: map[string]RoleConfig{...}}; src := &Config{Roles: ...};
            overlay(dst, src); assert dst.Roles[\"planner\"].{Provider,Model,...}."

- docfile: plan/015_b461e4720495/architecture/research_role_config.md
  why: "Already specifies S2 verbatim: 'FR-R7 adds a Timeout branch here: if rc.Timeout != 0
        { existing.Timeout = rc.Timeout } (duration-zero = inherit, mirrors the Config.Timeout overlay
        guard)'. §'Overlay (cross-layer field-merge)'."
  section: "1. The per-role config struct + TOML parsing → Overlay (cross-layer field-merge)"

- docfile: plan/015_b461e4720495/P1M1T1S1/PRP.md
  why: "S1 is the CONTRACT: it produces RoleConfig.Timeout time.Duration (0=inherit) + fileRoleConfig
        .Timeout string + the materialize parse. S2 consumes RoleConfig.Timeout. S1 explicitly defers
        the overlay branch to S2 (its Anti-Patterns + Integration Points name this exact line)."

- docfile: plan/015_b461e4720495/P1M1T1S2/research/findings.md
  why: "Verified line numbers, the != 0 vs != nil rationale, the S1 drift, the test cases, scope fences."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  file.go          # overlay() — the per-role field-merge loop (THE change site); global Config.Timeout guard (the mirror)
  file_test.go     # TestOverlayRolesFieldMerge (the test to mirror/extend — grep for it; line drifts with S1)
  config.go        # RoleConfig.Timeout time.Duration (from S1 — consumed, not modified here)
```

### Desired Codebase tree with files to be added

```bash
internal/config/file.go        # MODIFY: +1 branch in overlay's per-role loop (+ comment)
internal/config/file_test.go   # EXTEND: +TestOverlayRolesFieldMerge_Timeout (or extend the existing test)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (guard form — != 0, NOT != nil): RoleConfig.Timeout is a plain time.Duration (value type,
//   from S1), NOT *time.Duration. The overlay guard MUST be `if rc.Timeout != 0` — mirroring the
//   global Config.Timeout (file.go `if src.Timeout != 0`) and HookTimeout guards. Do NOT copy the
//   DiffContext/AutoStageAll `!= nil` discipline (those are POINTER types *int/*bool where *0/*false
//   are meaningful explicit values). For timeout, 0 ALWAYS means "inherit global" (FR-R7/S1) — there
//   is no meaningful "explicit 0" — so a plain Duration + != 0 is correct.

// CRITICAL (depends on S1): RoleConfig.Timeout only EXISTS after P1.M1.T1.S1 lands. rc.Timeout will
//   not compile pre-S1. Confirm S1 is merged (grep `Timeout   time.Duration` in config.go's RoleConfig)
//   before editing. The overlay function COMPILES FINE without this branch (it just doesn't merge
//   Timeout) — so this is a clean additive edit on S1's tree.

// CRITICAL (anchor on structure, not line numbers): S1 (P1.M1.T1.S1) is implemented IN PARALLEL and
//   its materialize rewrite (field-by-field, ABOVE overlay) shifts overlay + the per-role loop DOWN —
//   the drift was observed LIVE during this research (~28 lines and still moving). Treat ALL absolute
//   line numbers in this PRP as a stale snapshot. Locate the loop with grep, never by line number:
//     grep -n 'for role, rc := range src.Roles' internal/config/file.go   # the per-role loop
//     grep -n 'if rc.Reasoning != ""' internal/config/file.go              # the insert point (after it)
//     grep -n 'if src.Timeout != 0' internal/config/file.go                 # the global guard to mirror

// GOTCHA (zero-value existing is safe): when dst.Roles[role] is absent, `existing := dst.Roles[role]`
//   yields a zero RoleConfig (Provider/Model/Reasoning=="", Timeout==0). A higher layer setting ONLY
//   timeout on a new role leaves the siblings "" (inherit global) — correct composition of field-merge
//   + inherit-global. No special-casing needed.

// SCOPE: overlay is the SINGLE file/git-layer merge site (grep-confirmed — no second per-role merge).
//   env/flag per-role timeout (STAGECOACH_<ROLE>_TIMEOUT / --<role>-timeout) and git-config reading
//   (stagecoach.role.<role>.timeout) bypass overlay via a setRoleTimeout DIRECT-set helper — that is
//   sibling P1.M1.T2, NOT this subtask. Do NOT add setRoleTimeout / env / flag / git reading here.
```

## Implementation Blueprint

### Data models and structure
None. No new types, no struct changes (S1 owns those). S2 consumes `RoleConfig.Timeout time.Duration`
and adds one overlay branch. The `0`-duration "inherit" sentinel + `!= 0` guard mirrors the existing
`Config.Timeout` discipline exactly.

### Implementation Tasks (ordered by dependencies)

> **Hard prerequisite**: P1.M1.T1.S1 must be merged (`RoleConfig.Timeout time.Duration` must exist).
> Confirm with `grep -n "Timeout" internal/config/config.go | grep -i roleconfig` (or inspect the
> RoleConfig struct) before editing.

```yaml
Task 1: MODIFY internal/config/file.go — add the Timeout branch to overlay()'s per-role loop
  - LOCATE the per-role loop in overlay() by GREP (do NOT use line numbers — S1 is implemented in
    parallel and shifts this loop; it was already drifting during research): 
    `grep -n 'for role, rc := range src.Roles' internal/config/file.go`.
  - FIND the Reasoning branch inside it by GREP: `if rc.Reasoning != "" { existing.Reasoning = rc.Reasoning }`.
  - INSERT immediately AFTER the Reasoning branch and BEFORE `dst.Roles[role] = existing`:
        // FR-R7 — per-role timeout (time.Duration; 0 ⇒ inherit global). Mirrors the global
        // Config.Timeout overlay guard above (if src.Timeout != 0). Duration is a value type, so the
        // guard is != 0 (NOT != nil — that discipline is for *int/*bool pointer fields).
        if rc.Timeout != 0 {
            existing.Timeout = rc.Timeout
        }
  - VERIFY byte-identical discipline to the global guard: `if src.Timeout != 0 { dst.Timeout = src.Timeout }`
    (same function, a few dozen lines above).
  - DEPENDENCIES: S1 merged (RoleConfig.Timeout exists → rc.Timeout compiles).

Task 2: MODIFY internal/config/file_test.go — add TestOverlayRolesFieldMerge_Timeout
  - ADD a focused sibling test MIRRORING TestOverlayRolesFieldMerge (locate it by grep:
    `grep -n 'func TestOverlayRolesFieldMerge' internal/config/file_test.go`; it is ~641+ and drifting
    with S1). Direct
    Config{...} literals + overlay(dst, src), then assert. Cases:
    (a) higher-layer timeout OVERRIDES lower-layer timeout (non-zero-wins):
        dst.Roles["planner"].Timeout = 300*time.Second; src.Roles["planner"].Timeout = 480*time.Second
        → after overlay, dst.Roles["planner"].Timeout == 480*time.Second.
    (b) TIMEOUT field-merge does NOT erase sibling fields (THE regression guard — adding a branch
        must not clobber): src sets ONLY Timeout on planner (Provider/Model/Reasoning all "") →
        dst.Roles["planner"].Provider/Model/Reasoning SURVIVE from the lower layer.
    (c) zero/omitted timeout does NOT clobber (the != 0 guard): dst.Roles["planner"].Timeout = 300s;
        src.Roles["planner"] omits timeout (Timeout==0, maybe sets only Model) → dst stays 300s.
    (d) src-only role with timeout is added; untouched dst role survives (mirror the existing test's
        arbiter/message assertions for completeness).
  - Use time.Duration literals (300*time.Second, 480*time.Second) — NOT string parsing (this test
    exercises overlay, not materialize; materialize's string→Duration parse is S1's test).
  - PLACE next to TestOverlayRolesFieldMerge (keep the role overlay tests together).
  - DEPENDENCIES: Task 1.
  - ALTERNATIVE: instead of a sibling test, EXTEND TestOverlayRolesFieldMerge with Timeout fields on
    its existing planner dst/src + 2-3 new assertions. Either is acceptable; the focused sibling is
    clearer for a regression guard.

Task 3: VERIFY build + vet + format + tests
  - go build ./...
  - go vet ./internal/config/...
  - gofmt -l internal/config/file.go internal/config/file_test.go
  - go test ./internal/config/... -run 'OverlayRoles|Timeout' -v
  - make test && make lint
```

### Implementation Patterns & Key Details

```go
// PATTERN: the per-role overlay branch (byte-identical discipline to the global Config.Timeout guard)
for role, rc := range src.Roles {
    existing := dst.Roles[role] // zero RoleConfig if absent — fine (inherit-global sentinel)
    if rc.Provider != "" {
        existing.Provider = rc.Provider
    }
    if rc.Model != "" {
        existing.Model = rc.Model
    }
    if rc.Reasoning != "" {
        existing.Reasoning = rc.Reasoning
    }
    // FR-R7 — per-role timeout (time.Duration; 0 ⇒ inherit global). Mirrors the global
    // Config.Timeout overlay guard (if src.Timeout != 0). Value type ⇒ != 0 (NOT != nil).
    if rc.Timeout != 0 {
        existing.Timeout = rc.Timeout
    }
    dst.Roles[role] = existing
}

// PATTERN: the regression test (mirror TestOverlayRolesFieldMerge — grep 'func TestOverlayRolesFieldMerge')
func TestOverlayRolesFieldMerge_Timeout(t *testing.T) {
    dst := &Config{Roles: map[string]RoleConfig{
        "planner": {Provider: "agy", Model: "gemini-3.1-pro", Timeout: 300 * time.Second},
    }}
    src := &Config{Roles: map[string]RoleConfig{
        "planner": {Timeout: 480 * time.Second}, // higher layer sets TIMEOUT only
    }}
    overlay(dst, src)
    rc := dst.Roles["planner"]
    if rc.Timeout != 480*time.Second {
        t.Errorf("planner.timeout=%v want 480s (higher layer wins)", rc.Timeout)
    }
    if rc.Provider != "agy" || rc.Model != "gemini-3.1-pro" {
        t.Errorf("planner=%+v want provider=agy model=gemini-3.1-pro (timeout merge must not erase siblings)", rc)
    }
    // (c) the != 0 guard: a higher layer that OMITS timeout inherits the lower layer's
    dst2 := &Config{Roles: map[string]RoleConfig{"planner": {Timeout: 300 * time.Second}}}
    src2 := &Config{Roles: map[string]RoleConfig{"planner": {Model: "x"}}} // no timeout
    overlay(dst2, src2)
    if dst2.Roles["planner"].Timeout != 300*time.Second {
        t.Errorf("planner.timeout=%v want 300s (zero src must not clobber)", dst2.Roles["planner"].Timeout)
    }
}
```

### Integration Points

```yaml
NO database / routes / CLI / public-API / struct changes. One overlay branch + one test.

FUNCTION CHANGE:
  - overlay() (internal/config/file.go): per-role loop gains the `if rc.Timeout != 0` branch.

CONSUMED (from S1, must be merged first):
  - RoleConfig.Timeout time.Duration (config.go) — rc.Timeout reads it.

DOWNSTREAM (this subtask ENABLES but does NOT build — sibling subtasks):
  - P1.M1.T2.S1-S3: setRoleTimeout + STAGECOACH_<ROLE>_TIMEOUT env + --<role>-timeout flags +
    stagecoach.role.<role>.timeout git reading (DIRECT-set, bypass overlay — but consistent with it).
  - P1.M2.T1.S1: ResolveRoleTimeout + defaultRoleTimeouts{planner:480s} (reads the merged Timeout).
  - P1.M2.T2.S1: global default 480s→120s.
  - P1.M3: 13 provider.Execute call sites use the resolved per-role timeout.

UNCHANGED (do NOT touch): materialize (S1); Defaults().Timeout (stays 480s here — P1.M2.T2);
  Execute; all 13 call sites; ResolveRoleModel; env/flag/git loaders; docs.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build everything (RoleConfig.Timeout is consumed; the branch must compile across packages)
go build ./...
go vet ./internal/config/...
gofmt -l internal/config/file.go internal/config/file_test.go
# Expected: empty. If listed: gofmt -w them.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new timeout overlay test + the existing role overlay test (must still pass — no regression)
go test ./internal/config/... -run 'OverlayRoles|Timeout' -v

# Full config package
go test ./internal/config/... -v

# Whole suite (race) — overlay is on the load path of every config.Load; ensure no regressions
make test
# Expected: ALL pass. Global default still 480s (unchanged here) → 480s-pinning tests untouched.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary
make build

# Smoke: a global + repo TOML each setting [role.planner].timeout — after S1+S2 the REPO (higher)
# value survives overlay into the resolved config. (ResolveRoleTimeout is P1.M2.T1, not landed, so we
# can't yet OBSERVE the per-role timeout at generation — but we can confirm the load does not error and
# --verbose shows the resolved config. This is a load-level smoke, not a behavior assertion.)
mkdir -p /tmp/sc_s2 && cat > /tmp/sc_s2/global.toml <<'EOF'
[defaults]
provider = "pi"
[role.planner]
provider = "agy"
timeout = "300s"
EOF
cat > /tmp/sc_s2/.stagecoach.toml <<'EOF'
[role.planner]
timeout = "480s"
EOF
( cd /tmp/sc_s2 && git init -q && /home/dustin/projects/stagecoach/bin/stagecoach --config /tmp/sc_s2/global.toml --verbose --no-color 2>&1 | head -20 )
# Expected: loads WITHOUT a "[role.planner].timeout" parse error. (Pre-S2 the repo 480s would be
#           silently dropped at overlay; post-S2 it survives. Behavior observation lands with P1.M2/P1.M3.)
rm -rf /tmp/sc_s2
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: prove the per-role overlay loop now merges Timeout (exactly one new branch)
grep -n "rc.Timeout\|existing.Timeout" internal/config/file.go
# Expected: the one new branch (`if rc.Timeout != 0 { existing.Timeout = rc.Timeout }`).

# Grep guard: prove the guard form is != 0 (NOT != nil — Timeout is a value type, not a pointer)
grep -n "rc.Timeout" internal/config/file.go
# Expected: `if rc.Timeout != 0 {` (must NOT be `!= nil`).

# Scope-boundary guard: this subtask did NOT add env/flag/git/resolution/default changes
grep -rn "ResolveRoleTimeout\|setRoleTimeout\|defaultRoleTimeouts\|STAGECOACH_.*_TIMEOUT\|--planner-timeout" internal/config/
# Expected: empty (those are P1.M1.T2 / P1.M2.T1 — NOT this subtask).
grep -n "120 \* time.Second" internal/config/config.go
# Expected: empty (global default 480s→120s is P1.M2.T2; Defaults().Timeout must still be 480*time.Second).

# Confirm materialize (S1) was NOT re-touched here (S1 owns it)
git diff --stat -- internal/config/file.go
# Expected: only the overlay loop branch + the new test (no materialize/struct changes).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/config/...` clean
- [ ] `gofmt -l internal/config/file.go internal/config/file_test.go` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass, incl. new `TestOverlayRolesFieldMerge_Timeout`

### Feature Validation
- [ ] overlay() per-role loop has `if rc.Timeout != 0 { existing.Timeout = rc.Timeout }`
- [ ] Higher-layer non-zero timeout overrides lower-layer timeout (unit test case a)
- [ ] Timeout-only higher layer preserves lower-layer provider/model/reasoning (case b — the regression guard)
- [ ] Zero/omitted higher-layer timeout does NOT clobber lower-layer timeout (case c — the `!= 0` guard)
- [ ] Guard form is `!= 0` (value type), NOT `!= nil`

### Scope-Boundary Validation
- [ ] NO `setRoleTimeout` / env / flag / git-config per-role timeout added (P1.M1.T2)
- [ ] NO `ResolveRoleTimeout` / `defaultRoleTimeouts` added (P1.M2.T1)
- [ ] `Defaults().Timeout` STILL 480s; 480s-pinning tests UNCHANGED (P1.M2.T2)
- [ ] NO Execute call-site / docs changes (P1.M3 / P1.M4.T2)
- [ ] NO struct changes (S1 owns RoleConfig/fileRoleConfig) — only the overlay branch + test

### Code Quality & Docs
- [ ] Loop comment updated to mention the timeout field-merge + the `!= 0` (value-type) rationale
- [ ] New test placed next to TestOverlayRolesFieldMerge; uses time.Duration literals (not string parse)
- [ ] No stale comments claiming timeout "isn't merged yet"

---

## Anti-Patterns to Avoid

- ❌ Don't use `!= nil` for the Timeout guard — `RoleConfig.Timeout` is a plain `time.Duration` (value type from S1), not a pointer. Use `!= 0`, byte-identical to the global `Config.Timeout` overlay guard. The `!= nil` discipline is for `*int`/`*bool` pointer fields (DiffContext/AutoStageAll) where `*0`/`*false` are meaningful; timeout has no meaningful "explicit 0" (0 = inherit).
- ❌ Don't trust absolute line numbers — S1's materialize rewrite shifts overlay DOWN ~10-12 lines. Grep `for role, rc := range src.Roles` and insert after the `if rc.Reasoning != ""` branch.
- ❌ Don't add the branch anywhere except the per-role loop (e.g. not at the top-level scalar section). The global `Config.Timeout` guard already handles the flat timeout; the per-role loop is the only place `Roles[role].Timeout` is merged.
- ❌ Don't add `setRoleTimeout` / env / flag / git-config reading — those DIRECT-set the field (bypassing overlay) and are sibling P1.M1.T2's job.
- ❌ Don't change `Defaults().Timeout` (480s→120s) or any 480s-pinning test — that's P1.M2.T2.
- ❌ Don't touch materialize, the structs, `ResolveRoleModel`, the 13 Execute call sites, or docs — all out of scope (S1 / P1.M2 / P1.M3 / P1.M4.T2).
- ❌ Don't write the regression test using string-parsing of timeouts — overlay takes already-parsed `time.Duration` values; use `300*time.Second` literals. The string→Duration parse is materialize (S1)'s test, not overlay's.
- ❌ Don't omit the "zero src must not clobber" case (case c) — it is the assertion that proves the guard is `!= 0` rather than "always copy". Without it, a guard bug (e.g. unconditional `existing.Timeout = rc.Timeout`) would pass cases a/b but fail in production.

---

## Confidence Score: 10/10

One-pass success is essentially certain: the change is ONE branch mirroring an already-proven guard
(`Config.Timeout != 0`) in the SAME function, the architecture research specifies it verbatim, S1's
PRP explicitly defers it to this subtask, and the test pattern is a verbatim mirror of an existing
test (`TestOverlayRolesFieldMerge`). The only vigilance required is (1) the `!= 0` vs `!= nil`
discipline (called out as CRITICAL — Timeout is a value type), (2) anchoring on structure rather
than line numbers (S1 drifts them), and (3) including the "zero-src-must-not-clobber" regression case.
No external dependencies, no new patterns, no ambiguous inferences from the task description.
