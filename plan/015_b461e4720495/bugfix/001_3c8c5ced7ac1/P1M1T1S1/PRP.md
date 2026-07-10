name: "P1.M1.T1.S1 — Stager-fallback blanking guard + guidance comment in buildBootstrapConfig (Issue 1)"
description: >
  Fix the Critical bootstrap bug: when `config init --provider <X>` targets a provider that cannot
  serve as the stager (agy/opencode/qwen-code/codex/cursor), `StagerFallback` routes the stager to pi
  but writes a BARE pi model (`gpt-5.4-mini`) — a hard FR-R5b error that breaks decomposition (the
  DEFAULT action) on the very first run. Add a guard that blanks the bare fallback model (pi stays the
  stager; only the MODEL is blanked) and append the multi-backend guidance to the stager annotation.
  Internal config-bootstrap fix only — no docs, no API change; makes the code match existing docs.

---

## Goal

**Feature Goal**: Make `buildBootstrapConfig` emit `model = ""` (blank) for the `[role.stager]`
block whenever the stager falls back to pi for a non-pi target, instead of the FR-R5b-violating bare
`gpt-5.4-mini` — so a freshly-bootstrapped agy/opencode/qwen-code config does not fail at role
resolution on its first decompose run.

**Deliverable**: Two edits in `internal/config/bootstrap.go::buildBootstrapConfig` — (1) a new guard
`if stagerName == "pi" && stagerName != target { stagerModel = "" }` placed after the existing
`piBlanked` guard and before `applyOverrides`; (2) the stager annotation extended with the
multi-backend guidance when the fallback model was blanked. Plus the one directly-broken test
assertion (`bootstrap_test.go:87`) flipped from `gpt-5.4-mini` to `""` so the gate stays green.

**Success Definition**:
- `buildBootstrapConfig("agy", nil, nil)` produces `[role.stager]` with `provider = "pi"` and
  `model = ""` (was `model = "gpt-5.4-mini"`), plus a stager annotation that includes the
  multi-backend guidance ("pi is a multi-backend provider — prefix the model ...").
- The same holds for opencode/qwen-code/codex/cursor (all empty-`tooled_flags` providers).
- target==pi and target==claude outputs are byte-unchanged (they were already correct).
- An explicit `overrides["stager"]` still wins (the guard blanks, then applyOverrides sets the value;
  no guidance annotation when a real model is supplied).
- `go build ./...`, `go test ./internal/config/...`, `make test`, `make lint` all pass.

## Why

- **Issue 1 (Critical)**: `config init` is the first-run experience. For the most common non-pi
  providers (agy, opencode), the bootstrap writes an invalid config that fails decomposition — the
  DEFAULT action when nothing is staged and the tree is dirty — immediately at role resolution,
  before the planner even runs. PRD §9.15 FR-R5b (bare model on a `provider_flag` provider is a hard
  error), §9.17 FR-B1 (bootstrap writes a WORKING config), §9.16 FR-D4 (stager fallback), §9.14 FR-M1.
- The root cause: `roleDefaults` stores pi's models as bare strings (a v2-era comment says "sub-provider
  set separately via --provider"), but v3 FR-R5b/FR-B7 made a bare model on pi a HARD ERROR. The
  bootstrap's multi-backend-blanking logic (correct for target==pi, per FR-D2) is NOT applied on the
  stager-FALLBACK path. This fix applies the same blanking to the fallback path — consistent with
  FR-D2 ("there is no universally-correct inference backend", so ship blank + guidance).

## What

**User-visible behavior**: A user who runs `stagecoach config init --provider agy` (or opencode/
qwen-code/codex/cursor) now gets a `[role.stager]` block with a blank model and a comment explaining
how to prefix the inference backend — instead of a config that errors on the first decompose. The
output already-matched docs (docs/configuration.md:40 says pi models are left EMPTY); this makes the
code match the docs.

**Technical change (small, surgical, two edits + one test pin):**
1. New guard after the existing `piBlanked` guard (before `applyOverrides`): blank the bare fallback
   stager model when `stagerName == "pi" && stagerName != target`.
2. Extend the stager annotation: append the multi-backend guidance when the fallback model was
   blanked (`stagerName == "pi" && stagerName != target && stagerModel == ""`).
3. Flip the directly-broken test assertion at `bootstrap_test.go:87`.

### Success Criteria
- [ ] `[role.stager]` has `model = ""` for target ∈ {agy, opencode, qwen-code, codex, cursor}
- [ ] `[role.stager]` still has `provider = "pi"` (routing unchanged — only the model is blanked)
- [ ] the stager annotation includes "multi-backend" guidance when the model is blanked
- [ ] target==pi and target==claude outputs unchanged
- [ ] `overrides["stager"]` still overrides (no guidance annotation when a real model supplied)
- [ ] `go build ./...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact bug location, the two edits with line numbers, the guard-placement reasoning (before
applyOverrides), the annotation-extension condition (with the override edge case handled), the test pin
to flip, and the scope boundaries are all enumerated below (verified by reading source).

### Documentation & References

```yaml
- file: internal/config/bootstrap.go
  why: "THE change site. buildBootstrapConfig (line 143); piBlanked (165); pi-blank loop (166-172); StagerFallback call (174); piBlanked re-blank guard (175-179); applyOverrides (182-183); big NOTE block (186-195, ONLY target==pi); stagerAnnotation (200-203); writeRoleBlock stager (204)."
  pattern: "pi-target path already blanks correctly: piBlanked blanks all roles (166-172), StagerFallback returns ('pi',''), the piBlanked guard re-blanks (175-179), the NOTE block emits guidance (186-195). Mirror that blanking on the FALLBACK path."
  gotcha: "StagerFallback (76-89) returns ('pi','gpt-5.4-mini') for non-pi non-stager targets — a FRESH bare model from DefaultModelsForProvider('pi'). The piBlanked guard does NOT fire (target!=pi). Place the new guard BEFORE applyOverrides (183) so an explicit override can still set a model."

- file: internal/config/bootstrap.go (writeRoleBlock, lines 108-116)
  why: "How the annotation renders. writeRoleBlock prints '\\n[role.<r>]\\n', then '# <annotation>\\n' (if annotation != ''), then 'provider = %q\\n' (if prov != ''), then 'model = %q\\n'. So appending guidance to stagerAnnotation = one '# ...' comment line inside the [role.stager] block."
  gotcha: "The annotation is a SINGLE comment line (writeRoleBlock prints it once with '# %s'). Appending the guidance sentence yields one long (valid-TOML) comment line — that matches the task's 'via writeRoleBlock's annotation parameter'."

- file: internal/config/bootstrap_test.go
  why: "Test patterns + the pinning assertion. TestBuildBootstrapConfig_AgyStagerFallback (74) PINS the buggy value at line 87: assertContains(... '[role.stager]' ... model = \"gpt-5.4-mini\"). The gpt-5.4 negative guard (41-43) runs ONLY for target==pi today (extending it is S2)."
  pattern: "assertContains(t, content, substrs...) at line 218. Tests are package config (internal test pkg). TestBuildBootstrapConfig_Pi (16) is the target==pi reference (all models blank, no gpt-5.4 anywhere)."
  gotcha: "Line 87 asserts the EXACT value S1 changes — it MUST flip to model = \"\" in S1 or the gate is red. (The broader negative-guard extension is S2.)"

- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/bootstrap_pi_model_bug.md
  why: "Full root-cause + the correct (target==pi) path for comparison + the test that pins the bug. §Issue 1."
  section: "Issue 1: Stager-Fallback Path (bootstrap.go:165-181)"

- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M1T1S1/research/verification_deltas.md
  why: "Exact line-number table, the guard-placement reasoning, the override edge case in the annotation condition, and the scope boundaries. READ THIS before editing."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  bootstrap.go        # buildBootstrapConfig (143); StagerFallback (76); writeRoleBlock (108); applyOverrides (121)
  role_defaults.go    # roleDefaults table — pi models stored BARE (the upstream cause); DefaultModelsForProvider (96)
  bootstrap_test.go   # TestBuildBootstrapConfig_AgyStagerFallback (74) — pins bug at line 87; assertContains (218)
docs/configuration.md # line 40 already says pi models are left EMPTY (code must match — no docs change)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (guard placement): the new guard `if stagerName == "pi" && stagerName != target { stagerModel = "" }`
//   MUST go AFTER the piBlanked guard (175-179) but BEFORE applyOverrides (183). Ordering matches the
//   pi-target path (blank first, then applyOverrides can override). If placed AFTER applyOverrides, an
//   explicit overrides["stager"] would be clobbered back to "" — wrong.

// CRITICAL (override edge case in the annotation): use `stagerModel == ""` (NOT just `stagerName != target`)
//   as the "model was blanked" condition. After applyOverrides, stagerModel == "" iff the guard blanked it
//   AND no override supplied a model. If an override set a valid prefixed model, stagerModel != "" → no
//   guidance annotation (correct: the user supplied one).

// GOTCHA (writeRoleBlock annotation = single line): the annotation prints as ONE '# <annotation>' comment
//   line. Appending the guidance sentence yields a long-but-valid TOML comment line. Do NOT try to emit a
//   separate multi-line NOTE here (the big NOTE block at 186-195 is target==pi-only and lives before the
//   role blocks; the stager-fallback case must NOT duplicate it).

// GOTCHA (routing unchanged): do NOT blank/alter stagerName — pi MUST remain the stager provider
//   (StagerFallback routes there because pi is stager-capable). Only the MODEL is blanked.

// GOTCHA (test pin): bootstrap_test.go:87 asserts the buggy `model = "gpt-5.4-mini"` for the agy case.
//   S1's code change breaks it. Flip it to `model = ""` in S1 (minimum to keep the gate green). The
//   broader gpt-5.4 negative-guard extension (lines 41-43, target==pi-only today) is S2's job.
```

## Implementation Blueprint

### Data models and structure

No struct/type changes. Pure control-flow edit inside `buildBootstrapConfig` (a string-returning pure
function) + one test-assertion flip. The `RoleConfig`/config structs are untouched.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/config/bootstrap.go — add the stager-fallback blanking guard
  - PLACE: after the existing `if piBlanked { ... stagerModel = "" }` block (closes at line 179) and
    BEFORE `piHasOverrides := ...` / `applyOverrides(...)` (lines 182-183).
  - ADD:
        // Stager fell back to pi for a non-pi target (agy/opencode/qwen-code/codex/cursor have empty
        // tooled_flags). pi is a multi-backend provider: a bare fallback model (gpt-5.4-mini) is a
        // hard FR-R5b error. Blank it so the user supplies their own backend/model. pi REMAINS the
        // stager (stager-capable) — only the MODEL is blanked. Placed before applyOverrides so an
        // explicit override can still set a model (mirrors the pi-target path's blank-then-override).
        if stagerName == "pi" && stagerName != target {
            stagerModel = ""
        }
  - DEPENDENCIES: none (the StagerFallback call at 174 must have run, which it has by this point).

Task 2: MODIFY internal/config/bootstrap.go — extend the stager annotation with guidance
  - PLACE: inside the stager-annotation computation (lines 200-203), AFTER the existing
    `if stagerName != target { stagerAnnotation = ... }` block and BEFORE writeRoleBlock (line 204).
  - ADD:
        // When the stager fell back to pi and no override supplied a model, the bare fallback was
        // blanked — append the multi-backend guidance (same wording as the target==pi NOTE at 187-188)
        // so the user knows to prefix their inference backend.
        if stagerName == "pi" && stagerName != target && stagerModel == "" {
            stagerAnnotation += " pi is a multi-backend provider — prefix the model with your inference backend, e.g. model = \"zai/glm-5.2\". A bare model (no '/') on pi is a config error (FR-R5b)."
        }
  - NOTE: the `stagerModel == ""` check handles the override edge case (applyOverrides at 183 may have
    set a real model; in that case stagerModel != "" → no guidance).
  - DEPENDENCIES: Task 1 (the guard must blank stagerModel for the annotation condition to be true).

Task 3: MODIFY internal/config/bootstrap_test.go — flip the directly-broken pin (line 87)
  - EDIT line 87 inside TestBuildBootstrapConfig_AgyStagerFallback:
        assertContains(t, content, "[role.stager]", `model = "gpt-5.4-mini"`)
    →
        assertContains(t, content, "[role.stager]", `model = ""`)
  - OPTIONALLY add a guidance-comment assertion in the same test (cheap, in-scope as proof):
        if !strings.Contains(content, "multi-backend provider") {
            t.Error("agy stager-fallback config should include the pi multi-backend guidance in the stager annotation")
        }
  - DO NOT (S2's job): extend the `gpt-5.4` negative guard (lines 41-43) to the stager-fallback cases.
  - DEPENDENCIES: Tasks 1-2 (the code must produce the blanked model before the test can assert it).

Task 4: VERIFY unaffected paths still pass (no edits — just confirm)
  - TestBuildBootstrapConfig_Pi (16) — target==pi: all models still blank, no gpt-5.4 (unchanged).
  - TestGenerateBootstrapConfig_NamedProvider (183) — claude: stager = (claude, "sonnet"), no fallback
    (claude is stager-capable), unaffected.
  - These must pass with zero edits (the new guard's condition `stagerName != target` is false for them).
```

### Implementation Patterns & Key Details

```go
// PATTERN: blank-then-override ordering (mirrors the existing pi-target path)
stagerName, stagerModel := StagerFallback(target, models)   // 174
if piBlanked { stagerModel = "" }                            // 175-179 (target==pi only)
if stagerName == "pi" && stagerName != target {              // NEW — fallback-to-pi blanking
    stagerModel = ""
}
applyOverrides(models, &stagerModel, overrides)             // 183 — explicit override can still set a model

// PATTERN: annotation guidance only when the model is actually blank (override-aware)
var stagerAnnotation string                                  // 200
if stagerName != target {                                    // 201
    stagerAnnotation = target + " cannot serve as the stager (no tooled_flags); routed to " + stagerName + " (the first stager-capable provider)."
}
if stagerName == "pi" && stagerName != target && stagerModel == "" { // NEW — blanked, no override
    stagerAnnotation += " pi is a multi-backend provider — prefix the model with your inference backend, e.g. model = \"zai/glm-5.2\". A bare model (no '/') on pi is a config error (FR-R5b)."
}
writeRoleBlock(&b, "stager", stagerName, stagerModel, stagerAnnotation) // 204
```

### Integration Points

```yaml
NO database / routes / config-struct / public-API changes. Pure string-output edit in one function.

CODE:
  - internal/config/bootstrap.go::buildBootstrapConfig — +1 guard (after line 179), +1 annotation branch (after line 203)
  - internal/config/bootstrap_test.go:87 — flip pin gpt-5.4-mini → ""

DOWNSTREAM (consumes this fix):
  - P1.M1.T1.S2: extends the gpt-5.4 negative guard to stager-fallback cases + guidance-comment assertions.
  - P1.M1.T2.S1: post-bootstrap ValidateModel regression net (catches Issue 1 & 2 class automatically).

UNCHANGED: StagerFallback (routing intact); role_defaults.go (upstream bare-model storage — out of
  scope; the fix is at the bootstrap emission point, consistent with FR-D2); the commented-out pi
  provider block (Issue 2 — different code path, P1.M2.T1).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...
go vet ./...
gofmt -l internal/config/
# Expected: empty (gofmt -w if listed).
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The directly-affected tests
go test ./internal/config/... -run 'Bootstrap' -v
# Expected: TestBuildBootstrapConfig_AgyStagerFallback passes with the flipped pin (model = "");
#           TestBuildBootstrapConfig_Pi + NamedProvider (claude) unchanged/pass.

# Full config package
go test ./internal/config/... -v

# Whole suite (race) — bootstrap is consumed by cmd config init; ensure no regression
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary
make build

# Reproduce-then-verify the Issue 1 scenario manually (the PRD reproduction):
#   1. Fresh config for a non-pi, non-stager provider:
TMPDIR=$(mktemp -d)
HOME=$TMPDIR ./bin/stagecoach config init --provider agy
#   2. Inspect the [role.stager] block — model MUST be "" (not gpt-5.4-mini), provider "pi",
#      and the annotation MUST mention the multi-backend guidance:
grep -A3 '\[role.stager\]' "$TMPDIR/.config/stagecoach/config.toml"
#   Expected: provider = "pi" / model = "" / a "# ...multi-backend provider..." comment line.

#   3. Functional proof: in a git repo with ≥2 dirty files + nothing staged, stagecoach must NOT
#      error at role resolution with "model gpt-5.4-mini on pi must be inference/model". (The stager
#      model is blank ⇒ the user must supply one for a real run, but the bootstrap output itself is no
#      longer an immediate FR-R5b hard error on load/role-resolution of the OTHER roles. A full
#      decompose run needs a real stager model + a real provider — out of scope for this unit fix;
#      the grep above is the within-scope proof.)

# Also confirm the pi path is byte-unchanged:
HOME=$TMPDIR ./bin/stagecoach config init --provider pi
grep -A3 '\[role.stager\]' "$TMPDIR/.config/stagecoach/config.toml"
# Expected: provider line ABSENT (pi is the default ⇒ stager inherits), model = "".
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: prove NO bare gpt-5.4* model is emitted for a non-pi stager-fallback target
for tgt in agy opencode qwen-code codex cursor; do
  out=$(go test ./internal/config/ -run TestBuildBootstrapConfig_AgyStagerFallback -v 2>/dev/null; true)
  # (programmatic check via a one-off: call buildBootstrapConfig directly is cleaner — see test)
done
# Simpler: the S1/S2 tests assert model = "" for the agy case; for the other targets, add a quick
# table check in a test (optional in S1; required in S2).

# Scope-boundary guard: the commented-out pi block (Issue 2) was NOT touched
grep -n 'writeCommentedRoleBlock' internal/config/bootstrap.go
# Expected: the commented-block loop (205-222) is unchanged (Issue 2 fix is P1.M2.T1).

# Confirm routing intact: stagerName is still "pi" for agy (only the model blanked)
grep -n 'stagerName' internal/config/bootstrap.go
# Expected: no change to StagerFallback or stagerName assignment — only stagerModel blanking added.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/config/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass

### Feature Validation
- [ ] `[role.stager]` has `model = ""` for target ∈ {agy, opencode, qwen-code, codex, cursor}
- [ ] `[role.stager]` still has `provider = "pi"` for those targets (routing unchanged)
- [ ] stager annotation includes "multi-backend" guidance when model is blanked
- [ ] target==pi output unchanged (TestBuildBootstrapConfig_Pi passes unedited)
- [ ] target==claude output unchanged (stager-capable, no fallback; NamedProvider passes unedited)
- [ ] `overrides["stager"]` overrides the blanking (stagerModel != "", no guidance annotation)

### Scope-Boundary Validation
- [ ] NO change to StagerFallback / stagerName routing (only stagerModel blanked)
- [ ] NO change to the commented-out pi provider block (Issue 2 = P1.M2.T1)
- [ ] NO gpt-5.4 negative-guard extension (that breadth is S2) — only the line-87 pin flipped
- [ ] NO ValidateModel regression net (P1.M1.T2.S1)
- [ ] NO docs change (code already matches docs/configuration.md:40)

### Code Quality
- [ ] Guard placed before applyOverrides (blank-then-override ordering)
- [ ] Annotation condition uses `stagerModel == ""` (override-aware)
- [ ] Comments explain WHY (FR-R5b bare-model error; blank-then-override mirror)

---

## Anti-Patterns to Avoid

- ❌ Don't place the new guard AFTER `applyOverrides` — it would clobber an explicit `overrides["stager"]` back to "". Put it before (blank-then-override, mirroring the pi-target path).
- ❌ Don't blank/alter `stagerName` — pi MUST stay the stager provider (it's stager-capable). Only the MODEL is blanked.
- ❌ Don't gate the annotation on `stagerName != target` alone — use `&& stagerModel == ""` so an explicit override (which sets a real model) suppresses the guidance.
- ❌ Don't leave `bootstrap_test.go:87` asserting `gpt-5.4-mini` — the code change breaks it; flip it to `""` in S1 or the gate is red.
- ❌ Don't extend the `gpt-5.4` negative guard (lines 41-43) to stager-fallback cases here — that breadth is S2. S1 flips only the one directly-broken pin.
- ❌ Don't duplicate/move the big NOTE block (186-195) — it's target==pi-only and lives before the role blocks; the stager-fallback guidance goes ONLY in the stager annotation.
- ❌ Don't touch the commented-out pi provider block (Issue 2, bootstrap.go:205-222) — different code path, P1.M2.T1.
- ❌ Don't change role_defaults.go's bare pi models — the fix is at the bootstrap emission point (consistent with FR-D2); upstream storage is out of scope.
- ❌ Don't add docs — the fix makes the code match docs/configuration.md:40 (already says pi models are EMPTY).

---

## Confidence Score: 9/10

One-pass success is very high: two surgical edits in one pure function, exact line numbers verified,
the guard-placement and override-edge-case reasoning worked out, and the one test pin identified.
The -1 is for the minor plan-label ambiguity (the task body says "P1.M1.T1.S3 (test updates)" while
the plan tree labels it "S2") — resolved here by treating S1 as the code fix + the single directly-
broken pin (line 87), with broader test coverage explicitly deferred to the sibling test subtask.
The implementer must flip line 87 in S1 (not defer it) or the `make test` gate will be red.
