name: "P1.M1.T1.S2 — Extend negative gpt-5.4 guard to stager-fallback cases + add opencode case (Issue 1)"
description: >
  Test-only completion of the Issue 1 regression net. S1 (in parallel) already fixed
  buildBootstrapConfig (blanks the bare pi stager model on fallback) AND over-delivered into the agy
  test case (model="" pin, gpt-5.4 negative guard, multi-backend guidance assertion are all present for
  agy). This subtask adds the REMAINING, non-overlapping coverage: a new table-driven test proving the
  fix GENERALIZES to opencode and qwen-code (the other empty-tooled_flags stager-fallback providers),
  with the gpt-5.4 negative guard extended to every stager-fallback case (the architecture doc's
  explicit ask). Pure test additions — no production code, no docs.

---

## Goal

**Feature Goal**: Lock the Issue 1 fix as a cross-provider regression guard so a future revert of S1's
stager-fallback blanking (or the same bug class on the opencode/qwen-code path) is caught
automatically — proving `buildBootstrapConfig` emits `model = ""` (never a bare `gpt-5.4*`) for the
stager role on EVERY non-pi, non-stager-capable target, not just agy.

**Deliverable**: (1) a NEW table-driven test `TestBuildBootstrapConfig_StagerFallbackProviders_NoBarePiModel`
covering {agy, opencode, qwen-code} asserting the generic stager-fallback invariants (stager→pi,
model="", no gpt-5.4 anywhere, multi-backend guidance, fallback annotation); (2) optional broadening
of `TestBuildBootstrapConfig_ValidTOML` to include opencode + qwen-code. NO edits to production code
and NO edits to S1's existing `TestBuildBootstrapConfig_AgyStagerFallback` (avoid merge conflict).

**Success Definition**:
- `buildBootstrapConfig("opencode", nil, nil)` (the explicit item-3 deliverable) is asserted to produce
  `[role.stager]` with `provider = "pi"`, `model = ""`, no `gpt-5.4` anywhere, and the multi-backend
  guidance — same as agy.
- The same holds for qwen-code (breadth).
- The new test FAILS if run against the OLD (pre-S1) buildBootstrapConfig (which wrote a bare
  `gpt-5.4-mini` → the negative guard fires). This is the load-bearing regression-guard property.
- `go test ./internal/config/ -v -run TestBuildBootstrapConfig` passes; `make test` + `make lint` pass.
- No production file is touched; S1's agy test is untouched.

## User Persona (if applicable)

**Target User**: Stagecoach maintainers / CI (this is a regression-guard test; no user-facing surface).

**Use Case**: A maintainer refactors `buildBootstrapConfig` or `StagerFallback`; the table test fails
loudly if any stager-fallback provider regresses to a bare pi model — before a release ships a config
that errors on first decompose.

**Pain Points Addressed**: Issue 1 was silent for agy/opencode/qwen-code users because the ONE test that
exercised the stager-fallback path (`TestBuildBootstrapConfig_AgyStagerFallback`) PINNED the buggy value
and the gpt-5.4 negative guard ran only for `target=="pi"`. This subtask closes that gap for all
stager-fallback providers.

## Why

- **Issue 1 (Critical)**: `config init --provider <X>` for agy/opencode/qwen-code wrote a bare pi stager
  model → FR-R5b hard error → decomposition (the DEFAULT action) failed on the first run. S1 fixed the
  emission (blanks the model). S2 proves the fix holds for ALL stager-fallback providers, not just the
  one (agy) S1 happened to test. PRD §9.15 FR-R5b, §9.17 FR-B1, §9.16 FR-D4, §9.14 FR-M1.
- **The architecture doc explicitly asked for this**: `bootstrap_pi_model_bug.md §Test That Pins the Bug`
  says the gpt-5.4 negative guard (lines 41-43) "should be extended to the stager-fallback test cases"
  (plural). S1 extended it to agy only; S2 extends it to opencode + qwen-code.
- **Consumed by P1.M1.T2.S1**: the post-bootstrap `ValidateModel` regression net (T2.S1) builds on this
  coverage; the table of stager-fallback providers established here is the foundation.

## What

**User-visible behavior**: None (test-only; item point 5: "DOCS: none").

**Technical change (pure test additions):**
1. NEW test `TestBuildBootstrapConfig_StagerFallbackProviders_NoBarePiModel` — table over
   {agy, opencode, qwen-code}; per target assert: `[role.stager]` provider="pi", model="", no "gpt-5.4"
   anywhere, "multi-backend provider" guidance present, "cannot serve as the stager" + "routed to pi".
2. (Recommended) Add {opencode, qwen-code} rows to `TestBuildBootstrapConfig_ValidTOML`.

### Success Criteria
- [ ] opencode case asserted (stager→pi, model="", no gpt-5.4, guidance) — the explicit item deliverable
- [ ] qwen-code case asserted (breadth)
- [ ] agy re-asserted at the generic-invariant level in the table test (defense in depth)
- [ ] New test would FAIL against the old (pre-S1) buildBootstrapConfig
- [ ] `go test ./internal/config/ -v -run TestBuildBootstrapConfig` passes
- [ ] No production file touched; S1's agy test untouched; `make test`/`make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the parallel-edit reality (what S1 already did), StagerFallback's exact mechanics for
opencode/qwen-code, the list of stager-fallback providers, the exact invariants to assert, the new-test
(non-conflicting) placement strategy, the regression-guard property to verify, and the scope fences.

### Documentation & References

```yaml
- file: internal/config/bootstrap_test.go
  why: "THE change site. TestBuildBootstrapConfig_AgyStagerFallback (line 74) is the S1-owned agy test
        — DO NOT EDIT IT (S1 already put model='' + gpt-5.4 guard + multi-backend assertion there;
        editing risks a parallel-merge conflict). assertContains helper (line ~227)."
  pattern: "assertContains(t, content, substrs...) checks all substrs present. The agy test's stager
            block (lines 86-93) is the EXACT assertion shape to mirror in the new table test."
  critical: "S1 OVER-DELIVERED: lines 87-93 already have model='' + the gpt-5.4 negative guard + the
            multi-backend assertion for AGY. The item description's line numbers (74-97, line 87 =
            gpt-5.4-mini, guard at 41-43 pi-only) are STALE (pre-S1). Do NOT re-add what is already
            there; ADD a new test for opencode/qwen-code."

- file: internal/config/bootstrap_test.go
  why: "TestBuildBootstrapConfig_ValidTOML (line 143) — the optional place to add opencode/qwen-code
        TOML-validity rows. Cases today: {pi, pi+claude, claude, claude+pi, agy}."
  pattern: "table of {target, installed}; toml.Unmarshal must not error."

- file: internal/config/bootstrap.go
  why: "StagerFallback (line 76) + buildBootstrapConfig. Confirms what opencode/qwen-code produce so
        the assertions are correct. READ-ONLY here (S1 owns the fix; S2 is test-only)."
  pattern: "StagerFallback returns ('pi', bare-model) for any target whose models['stager']=='' (empty
            tooled_flags). S1's guard (line 185-186) then blanks the model to ''. So the FIXED output
            for opencode/qwen-code is [role.stager] provider='pi', model='' (+ guidance annotation)."
  critical: "Do NOT modify bootstrap.go — S1's fix is complete and merged. S2 asserts its behavior; it
            does not change it."

- file: internal/provider/builtin.go
  why: "Confirms which providers are stager-fallback targets (empty TooledFlags ⇒ cannot be stager)."
  pattern: "agy: TooledFlags nil (lines 187-188, 216); qwen-code: nil (238-239, 263); opencode: empty
            (per Issue 1). pi is always stager-capable (the fallback destination)."

- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/bootstrap_pi_model_bug.md
  why: "§Test That Pins the Bug explicitly says the gpt-5.4 negative guard 'should be extended to the
        stager-fallback test cases' (plural). S1 did agy; S2 does opencode + qwen-code."
  section: "Issue 1 → Test That Pins the Bug"

- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M1T1S1/PRP.md
  why: "S1 is the CONTRACT. Its Task 3 flipped line 87 + (optionally) added the agy guidance assertion,
        and explicitly DEFERRED the gpt-5.4 negative-guard breadth to S2. Its Anti-Patterns name this
        subtask. READ to confirm S1's scope so S2 does not duplicate/conflict."

- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M1T1S2/research/findings.md
  why: "The parallel-edit reality, StagerFallback mechanics, the stager-fallback provider list, the
        non-conflicting new-test design, and scope fences."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  bootstrap_test.go  # TestBuildBootstrapConfig_AgyStagerFallback (74, S1-owned — DO NOT EDIT);
                     # TestBuildBootstrapConfig_ValidTOML (143 — optional broaden);
                     # assertContains helper (~227). ADD the new table test here.
  bootstrap.go       # S1's fix (COMPLETE — read-only for S2): guard at 185-186, annotation at 215-216
internal/provider/builtin.go  # confirms agy/qwen-code/opencode have empty TooledFlags (stager-fallback targets)
```

### Desired Codebase tree with files to be added

```bash
internal/config/bootstrap_test.go  # MODIFY (additive): +TestBuildBootstrapConfig_StagerFallbackProviders_NoBarePiModel;
                                   # optionally +2 rows in TestBuildBootstrapConfig_ValidTOML
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (parallel-edit / no-conflict): S1 is "Implementing" in parallel and has ALREADY edited
//   bootstrap_test.go (lines 87-93: model="", gpt-5.4 guard, multi-backend assertion for agy). Do NOT
//   edit TestBuildBootstrapConfig_AgyStagerFallback — you will conflict with S1's in-flight changes.
//   Add a NEW top-level test function for opencode/qwen-code.

// CRITICAL (test-only): S2 touches ONLY bootstrap_test.go. S1's bootstrap.go fix is complete and
//   merged (guard at 185-186, annotation at 215-216). Do NOT modify bootstrap.go, role_defaults.go,
//   or any production file. Item point 5: "DOCS: none — test-only changes."

// CRITICAL (regression-guard property): the new test MUST fail against the OLD buildBootstrapConfig.
//   The OLD code wrote [role.stager] model="gpt-5.4-mini" (bare) for opencode/qwen-code. So asserting
//   model="" AND "no gpt-5.4 anywhere" both fail under the old code. Verify this by reasoning (or by
//   temporarily reverting S1's guard locally if you want empirical proof) — it is the test's reason
//   to exist.

// GOTCHA (the negative guard checks the WHOLE content, not just the stager block): mirror S1's agy
//   guard — `if strings.Contains(content, "gpt-5.4")` scans the entire bootstrapped config. This
//   catches a bare stager model AND any other stray gpt-5.4 reference. Use the same whole-content
//   scope in the table test.

// GOTCHA (opencode/qwen-code ARE valid buildBootstrapConfig targets): DefaultModelsForProvider
//   handles them; buildBootstrapConfig will not crash. But assert valid TOML too (the optional
//   ValidTOML rows) — a blanked model + a long guidance comment must still parse.

// SCOPE: do NOT add the post-bootstrap ValidateModel net (that is P1.M1.T2.S1 — S2's tests are
//   CONSUMED BY it, not replaced). Do NOT touch the commented-out pi block (Issue 2 = P1.M2.T1).
```

## Implementation Blueprint

### Data models and structure
None. Pure test additions. No types, no production code, no fixtures beyond inline `buildBootstrapConfig`
calls. The new test mirrors the assertion shape S1 already used for agy.

### Implementation Tasks (ordered by dependencies)

> **Hard prerequisite**: P1.M1.T1.S1's code fix must be merged (bootstrap.go:185-186 guard). It IS
> merged in the current tree — confirm with `grep -n 'stagerName == "pi" && stagerName != target'
> internal/config/bootstrap.go` before writing the test (the test asserts the FIXED behavior).

```yaml
Task 1: VERIFY the S1 fix + the agy test state (read-only — confirm before adding)
  - grep -n 'stagerName == "pi" && stagerName != target' internal/config/bootstrap.go
    → expect the blanking guard (S1's fix is present).
  - grep -n 'gpt-5.4\|multi-backend provider\|model = ""' internal/config/bootstrap_test.go
    → expect S1 already added the agy-case guard + guidance assertion (lines ~87-93).
  - If the agy test LACKS the guard/guidance (S1 didn't over-deliver after all), the table test in
    Task 2 still covers agy at the generic level — no separate edit to S1's test needed.

Task 2: MODIFY internal/config/bootstrap_test.go — ADD the table-driven regression test
  - ADD a NEW top-level function (do NOT modify TestBuildBootstrapConfig_AgyStagerFallback):
        // TestBuildBootstrapConfig_StagerFallbackProviders_NoBarePiModel is the Issue 1 cross-provider
        // regression guard. S1 fixed the agy case (and its test); this proves the fix GENERALIZES to
        // every provider whose empty tooled_flags forces a stager fallback to pi (FR-D4). For each, the
        // stager is routed to pi with a BLANK model (never a bare gpt-5.4* — FR-R5b) plus the
        // multi-backend guidance. Would FAIL against the pre-S1 buildBootstrapConfig.
        func TestBuildBootstrapConfig_StagerFallbackProviders_NoBarePiModel(t *testing.T) {
            for _, target := range []string{"agy", "opencode", "qwen-code"} {
                t.Run(target, func(t *testing.T) {
                    content := buildBootstrapConfig(target, nil, nil)
                    // stager routed to pi, model BLANKED (the fix):
                    assertContains(t, content, "[role.stager]", `provider = "pi"`)
                    assertContains(t, content, "[role.stager]", `model = ""`)
                    // the negative guard, generalized to every stager-fallback case:
                    if strings.Contains(content, "gpt-5.4") {
                        t.Errorf("%s stager-fallback config must not ship a bare gpt-5.4* pi model (FR-R5b); got:\n%s", target, content)
                    }
                    // multi-backend guidance present on the stager annotation:
                    if !strings.Contains(content, "multi-backend provider") {
                        t.Errorf("%s stager-fallback config missing the pi multi-backend guidance", target)
                    }
                    // fallback annotation present:
                    if !strings.Contains(content, "cannot serve as the stager") || !strings.Contains(content, "routed to pi") {
                        t.Errorf("%s stager-fallback config missing the stager-fallback annotation", target)
                    }
                })
            }
        }
  - PLACE: near the other TestBuildBootstrapConfig_* tests (e.g. after TestBuildBootstrapConfig_AgyStagerFallback).
  - NOTE on agy overlap: S1's agy test asserts agy-SPECIFIC models (planner="Gemini 3.5 Flash (High)");
    this table asserts the GENERIC cross-provider invariant for agy. Complementary, not duplicative.
  - DEPENDENCIES: Task 1 (S1 fix present).

Task 3 (RECOMMENDED): MODIFY TestBuildBootstrapConfig_ValidTOML — add opencode + qwen-code rows
  - In the `cases` slice (line ~146), add:
        {"opencode", nil},
        {"qwen-code", nil},
  - This proves the bootstrapped config for those targets is valid TOML (blanked model + guidance
    comment parse correctly). Free sanity check; clean additive rows.
  - DEPENDENCIES: none (independent of Task 2).

Task 4: VERIFY build + vet + the exact item test command
  - go build ./...
  - go vet ./internal/config/...
  - gofmt -l internal/config/bootstrap_test.go
  - go test ./internal/config/ -v -run TestBuildBootstrapConfig     # the item's exact command
  - go test ./internal/config/... -v
  - make test && make lint
```

### Implementation Patterns & Key Details

```go
// PATTERN: the new table test (mirrors S1's agy stager assertions, generalized across providers)
func TestBuildBootstrapConfig_StagerFallbackProviders_NoBarePiModel(t *testing.T) {
	for _, target := range []string{"agy", "opencode", "qwen-code"} {
		t.Run(target, func(t *testing.T) {
			content := buildBootstrapConfig(target, nil, nil)
			assertContains(t, content, "[role.stager]", `provider = "pi"`)
			assertContains(t, content, "[role.stager]", `model = ""`) // the S1 fix
			if strings.Contains(content, "gpt-5.4") {                  // would FAIL on old code (wrote gpt-5.4-mini)
				t.Errorf("%s stager-fallback config must not ship a bare gpt-5.4* pi model (FR-R5b); got:\n%s", target, content)
			}
			if !strings.Contains(content, "multi-backend provider") {
				t.Errorf("%s stager-fallback config missing the pi multi-backend guidance", target)
			}
			if !strings.Contains(content, "cannot serve as the stager") || !strings.Contains(content, "routed to pi") {
				t.Errorf("%s stager-fallback config missing the stager-fallback annotation", target)
			}
		})
	}
}

// PATTERN: the optional ValidTOML row additions (clean, additive)
cases := []struct {
	target    string
	installed []string
}{
	// ... existing rows ...
	{"opencode", nil},
	{"qwen-code", nil},
}
```

### Integration Points

```yaml
NO database / routes / config-struct / production-code / docs changes. Test-only.

TEST FILE:
  - internal/config/bootstrap_test.go — +1 new test function (table over agy/opencode/qwen-code);
    optionally +2 rows in TestBuildBootstrapConfig_ValidTOML.

CONSUMED (read-only — S1's merged fix):
  - internal/config/bootstrap.go:185-186 (the blanking guard) + 215-216 (the guidance annotation).

DOWNSTREAM (consumes this coverage):
  - P1.M1.T2.S1: post-bootstrap ValidateModel regression net — builds on the stager-fallback provider
    set established here (the table is the foundation for the broader ValidateModel sweep).

UNCHANGED (do NOT touch): bootstrap.go (S1's fix); role_defaults.go; TestBuildBootstrapConfig_AgyStagerFallback
  (S1-owned, already correct); the commented-out pi block (Issue 2 = P1.M2.T1); docs.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                       # test files compile
go vet ./internal/config/...
gofmt -l internal/config/bootstrap_test.go
# Expected: empty (gofmt -w if listed).
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The item's EXACT command — runs all TestBuildBootstrapConfig_* including the new table test
go test ./internal/config/ -v -run TestBuildBootstrapConfig
# Expected: the new TestBuildBootstrapConfig_StagerFallbackProviders_NoBarePiModel/agy,
#           /opencode, /qwen-code subtests all PASS. S1's AgyStagerFallback still passes (untouched).

# Full config package
go test ./internal/config/... -v

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Test-only subtask — no binary behavior change to smoke. The within-scope proof is the unit test.
# Optional manual confirmation that the fix (S1's) is real for opencode (already proven by the test):
make build
TMPDIR=$(mktemp -d) && HOME=$TMPDIR ./bin/stagecoach config init --provider opencode
grep -A3 '\[role.stager\]' "$TMPDIR/.config/stagecoach/config.toml"
# Expected: provider = "pi" / model = "" / a "# ...multi-backend provider..." comment. NO gpt-5.4-mini.
rm -rf "$TMPDIR"
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: prove the new test exists and covers opencode + qwen-code (the unique S2 deliverable)
grep -n 'TestBuildBootstrapConfig_StagerFallbackProviders_NoBarePiModel\|"opencode"\|"qwen-code"' internal/config/bootstrap_test.go
# Expected: the new test function + the opencode/qwen-code table entries.

# Grep guard: prove NO production file was touched by this subtask
git diff --stat -- internal/config/bootstrap.go internal/config/role_defaults.go
# Expected: empty (S1 owns bootstrap.go; S2 is test-only).

# Grep guard: prove S1's agy test was NOT modified (no parallel-edit conflict)
git diff -- internal/config/bootstrap_test.go | grep -E '^-' | grep -i 'AgyStagerFallback\|gpt-5.4-mini'
# Expected: empty (no deletions in the agy test — S2 only ADDED a new function).

# Scope-boundary guard: this subtask did NOT add the ValidateModel net (P1.M1.T2.S1)
grep -rn 'ValidateModel' internal/config/bootstrap_test.go
# Expected: empty (T2.S1 owns it; S2's tests are consumed BY T2.S1).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/config/...` clean
- [ ] `gofmt -l internal/config/bootstrap_test.go` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass
- [ ] `go test ./internal/config/ -v -run TestBuildBootstrapConfig` passes (the item's exact command)

### Feature Validation
- [ ] opencode case asserted (stager→pi, model="", no gpt-5.4, guidance, annotation)
- [ ] qwen-code case asserted (breadth)
- [ ] agy re-asserted at the generic-invariant level in the table test
- [ ] New test would FAIL against the pre-S1 buildBootstrapConfig (regression-guard property)
- [ ] (Optional) opencode + qwen-code rows pass in TestBuildBootstrapConfig_ValidTOML

### Scope-Boundary Validation
- [ ] NO production file touched (bootstrap.go, role_defaults.go unchanged — S2 is test-only)
- [ ] NO edit to S1's TestBuildBootstrapConfig_AgyStagerFallback (parallel-edit conflict avoided)
- [ ] NO ValidateModel regression net added (P1.M1.T2.S1)
- [ ] NO commented-out pi block change (Issue 2 = P1.M2.T1)
- [ ] NO docs change (item point 5)

### Code Quality
- [ ] New test is a separate top-level function (not wedged into S1's agy test)
- [ ] Assertions mirror S1's agy stager-block shape (consistent style)
- [ ] Failure messages include the target name + the full content for fast triage

---

## Anti-Patterns to Avoid

- ❌ Don't edit `TestBuildBootstrapConfig_AgyStagerFallback` — S1 is implementing in parallel and already put the agy-case assertions (model="", gpt-5.4 guard, multi-backend guidance) there. Editing it risks a merge conflict AND duplicates S1's work. Add a NEW test function for opencode/qwen-code.
- ❌ Don't modify `bootstrap.go` — S1's fix (guard at 185-186, annotation at 215-216) is complete and merged. S2 is TEST-ONLY (item point 5). Asserting the behavior is the job; changing it is not.
- ❌ Don't trust the item description's line numbers (74-97, line 87 = gpt-5.4-mini, guard at 41-43) — they describe the PRE-S1 state. The current tree already has line 87 = `model = ""` and the agy gpt-5.4 guard. Grep for the anchors, don't edit by stale line number.
- ❌ Don't write a test that passes against the OLD buggy code — the whole point is the regression guard. Assert `model = ""` AND `no "gpt-5.4"`; both fail under the old code (which wrote `gpt-5.4-mini`).
- ❌ Don't scope the negative guard to just the `[role.stager]` substring — mirror S1's agy guard, which scans the WHOLE content (`strings.Contains(content, "gpt-5.4")`), catching a bare stager model or any stray reference.
- ❌ Don't add the post-bootstrap ValidateModel net here — that's P1.M1.T2.S1. S2's tests are CONSUMED BY T2.S1, not replaced by it.
- ❌ Don't touch the commented-out pi provider block (Issue 2 = P1.M2.T1) or role_defaults.go.
- ❌ Don't add docs — item point 5 says none (the fix already matches docs/configuration.md:40).
- ❌ Don't limit coverage to opencode alone — the architecture doc says extend the guard to the stager-fallback CASES (plural). Cover opencode AND qwen-code (agy too, for a complete cross-provider invariant table).

---

## Confidence Score: 9/10

One-pass success is very high: the change is a single NEW test function (no production code, no edits
to S1's in-flight test), the assertions mirror S1's already-working agy shape verbatim, and StagerFallback's
mechanics for opencode/qwen-code are confirmed (returns ("pi", bare) → S1 blanks it → assert model="" +
no gpt-5.4). The -1 is for the parallel-edit ambiguity: the item description assumed S1 would ONLY flip
line 87, but S1 over-delivered and already did the agy-case guard + guidance. An implementer who
blindly follows the item's literal "(a) change line 87, (b) add negative guard to the agy test" would
DUPLICATE S1's work and risk a conflict. This PRP resolves that by making the opencode/qwen-code
table test the PRIMARY (unique) deliverable and explicitly forbidding edits to S1's agy test — the
implementer must read the current tree first (Task 1) rather than trust the stale item line numbers.
