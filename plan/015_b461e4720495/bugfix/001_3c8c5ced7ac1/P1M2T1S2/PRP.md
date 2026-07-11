name: "P1.M2.T1.S2 — Test the commented-out pi block ships blank models + guidance NOTE (Issue 2, FR-R5b/FR-B1)"
description: >
  TEST-ONLY subtask. Add `TestBuildBootstrapConfig_CommentedPiBlockBlanked` (plus a small
  `extractCommentedProviderBlock` helper) to `internal/config/bootstrap_test.go`. It calls
  `buildBootstrapConfig("claude", []string{"claude", "pi"}, nil)` (target=claude, pi installed) and
  asserts the commented-out pi provider block — produced by the P1.M2.T1.S1 fix — (a) has its
  `# === pi (installed)` header, (b) ships `# model = ""` for all four roles, (c) carries the
  multi-backend guidance NOTE, and (d) contains NO bare `# model = "gpt-5.4…"` assignment. It is the
  commented-block analogue of `TestBuildBootstrapConfig_Pi` (which covers the ACTIVE pi block) and the
  mirror companion of `TestBuildBootstrapConfig_OtherInstalledCommented` (which proves the non-pi
  claude block is NOT blanked). NO production-code changes; NO docs changes; does not touch the active
  block paths, the parallel ValidateModel regression net, or exampleConfigTemplate.

---

## Goal

**Feature Goal**: Lock in the P1.M2.T1.S1 fix (Issue 2) with a permanent, deterministic regression
test that PROVES the commented-out pi provider block in every `config init` output is FR-R5b-valid:
blank per-role models (`# model = ""`) + a multi-backend guidance NOTE, and ZERO bare `gpt-5.4*` model
assignments — so uncommenting the block (the documented FR-B1 workflow) yields a valid config, not a
hard `model "gpt-5.4-nano" on pi must be inference/model` error.

**Deliverable**:
1. **internal/config/bootstrap_test.go** — ONE new test `TestBuildBootstrapConfig_CommentedPiBlockBlanked`
   plus ONE new helper `extractCommentedProviderBlock(content, provider string) string`. No other file
   changes.

**Success Definition**:
- `go test ./internal/config/ -v -run TestBuildBootstrapConfig_CommentedPi` PASSES against the S1-fixed
  `buildBootstrapConfig`.
- The same test FAILS against the pre-S1 `buildBootstrapConfig` (4 independent assertions all fire on
  the old bare-`gpt-5.4` block — see §Findings §7).
- The companion `TestBuildBootstrapConfig_OtherInstalledCommented` stays GREEN (claude commented block
  keeps `# model = "haiku"` — claude is NOT blanked; S1's `name == "pi"` branch never touches claude).
- `go build ./...`, `make test` (race), `make lint`, `gofmt -l` all clean.

## User Persona (if applicable)

**Target User**: The maintainer (and future contributor) who needs assurance that the commented pi
block — the most common uncomment target for FR-B1 — is always shipped FR-R5b-valid.

**Use Case**: Regression guard. After S1 lands, this test ensures a future refactor of the commented-
provider loop cannot silently reintroduce bare `gpt-5.4*` models into the pi block (which would break
uncommenting for every claude/agy/opencode/qwen-code default user who adds pi).

**Pain Points Addressed**: Before S1 there was NO test asserting the commented pi block's model values
(S1's research §5 confirmed this gap). This test closes that gap permanently.

## Why

- **Issue 2 (Major) / FR-R5b / FR-B1**: The commented-out pi block is the ONLY commented block that
  produced a hard error on uncomment (opencode is `openai/`-prefixed; claude/agy/etc. are bare-but-
  legal single-backend models). S1 fixed it by blanking; this test PROVES the fix and guards it.
- **Symmetry of coverage**: `TestBuildBootstrapConfig_Pi` covers the ACTIVE pi block (target=pi);
  `TestBuildBootstrapConfig_OtherInstalledCommented` covers the commented CLAUDE block (not blanked).
  This test fills the missing cell: the commented PI block (blanked). With all three, the blanking rule
  is fully specified: "blank pi (active + commented); leave everything else alone."
- **Bounded scope**: one test + one helper, test-only. The production fix is S1's contract; the
  ValidateModel regression net is parallel (P1.M1.T2.S1) and structurally cannot see commented TOML.

## What

**User-visible behavior**: None (test-only). The internal effect: the test suite now fails loudly if
the commented pi block ever ships a bare `gpt-5.4*` model again.

**Technical change** (additions to `internal/config/bootstrap_test.go` ONLY):

```go
// TestBuildBootstrapConfig_CommentedPiBlockBlanked guards Issue 2 (FR-R5b/FR-B1): when a non-pi
// target is generated with pi ALSO installed, the commented-out pi provider block must ship BLANK
// models + a multi-backend guidance NOTE — NOT the bare gpt-5.4* defaults that are a hard FR-R5b
// error when uncommented. Mirror of TestBuildBootstrapConfig_OtherInstalledCommented (which proves
// the non-pi claude block is NOT blanked). See findings §1/§2/§7.
func TestBuildBootstrapConfig_CommentedPiBlockBlanked(t *testing.T) {
	content := buildBootstrapConfig("claude", []string{"claude", "pi"}, nil)

	// (a) commented pi block header present
	if !strings.Contains(content, "# === pi (installed)") {
		t.Fatalf("missing commented pi block header; content:\n%s", content)
	}

	piBlock := extractCommentedProviderBlock(content, "pi") // scope assertions to the pi block only

	// (b) ships BLANK models (`# model = ""`)
	if !strings.Contains(piBlock, `# model = ""`) {
		t.Errorf("commented pi block missing blank `# model = \"\"`; pi block:\n%s", piBlock)
	}
	// all FOUR roles blanked
	if got := strings.Count(piBlock, `# model = ""`); got != 4 {
		t.Errorf("commented pi block: want 4 blank `# model = \"\"` (planner/stager/message/arbiter), got %d; pi block:\n%s", got, piBlock)
	}

	// (c) multi-backend guidance NOTE present
	if !strings.Contains(piBlock, "multi-backend provider") {
		t.Errorf("commented pi block missing the multi-backend guidance NOTE; pi block:\n%s", piBlock)
	}

	// (d) NEGATIVE: no BARE gpt-5.4* model ASSIGNMENT (the actual bug shape).
	// NOTE: the literal substring "gpt-5.4" ALSO appears inside S1's NOTE example
	// (`# e.g. model = "zai/gpt-5.4"`) — a slash-PREFIXED model in a `# e.g.` comment, which is
	// CORRECT and must NOT trip the guard. So we assert on the bare ASSIGNMENT form
	// `# model = "gpt-5.4`, which matches the OLD bug and only the old bug. See findings §2.
	if strings.Contains(piBlock, `# model = "gpt-5.4`) {
		t.Errorf("commented pi block must not ship bare gpt-5.4* model assignments (FR-R5b); pi block:\n%s", piBlock)
	}

	// Companion sanity: the ACTIVE claude role models are NOT blanked (claude has no ProviderFlag →
	// bare aliases are legal). Confirms the pi-blank didn't collateral-damage the active block.
	if !strings.Contains(content, `model = "haiku"`) {
		t.Errorf("active claude block unexpectedly missing `model = \"haiku\"` (claude must NOT be blanked)")
	}
}

// extractCommentedProviderBlock returns the commented-provider block for `provider`: from the
// `# === <provider> (installed)` header up to (not including) the next section boundary (another
// commented-provider block, the [generation] dashed separator, or EOF). Idiom mirrors
// extractStagerBlock. Scopes assertions to a single commented provider so active role models
// elsewhere don't interfere. See findings §4.
func extractCommentedProviderBlock(content, provider string) string {
	header := "# === " + provider + " (installed)"
	start := strings.Index(content, header)
	if start < 0 {
		return ""
	}
	rest := content[start:]
	nextIdx := len(rest)
	for _, marker := range []string{"\n# ===", "\n# ---"} {
		if i := strings.Index(rest[1:], marker); i >= 0 && i+1 < nextIdx {
			nextIdx = i + 1
		}
	}
	return rest[:nextIdx]
}
```

### Success Criteria
- [ ] New test `TestBuildBootstrapConfig_CommentedPiBlockBlanked` exists in `bootstrap_test.go`.
- [ ] New helper `extractCommentedProviderBlock` exists in `bootstrap_test.go` (Helpers section, near
      `extractStagerBlock`).
- [ ] Test PASSES against the S1-fixed code (all 4 assertions + count + companion sanity).
- [ ] Test FAILS against pre-S1 code (assertions b, count, c, d all fire — verified by simulation).
- [ ] Companion `TestBuildBootstrapConfig_OtherInstalledCommented` stays GREEN.
- [ ] `go build ./...`, `make test` (race), `make lint`, `gofmt -l` all clean.
- [ ] ZERO production-code files changed (`git diff --name-only` == only `bootstrap_test.go`).

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim test + helper code is above; the exact ground-truth output the test asserts
against is in findings §1; the one non-obvious conflict (literal `gpt-5.4` vs S1's NOTE example) and
its resolution are spelled out in findings §2 and re-explained inline in the test comment; the
old-vs-new behavior matrix (findings §7) proves the test fails-on-old/passes-on-new; and the scope
fences (findings §9) prevent touching production code or the companion test.

### Documentation & References

```yaml
# MUST READ — the authoritative research (ground-truth output, the (d) conflict resolution, validated
# helper, old-vs-new matrix, scope fences)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M2T1S2/research/findings.md
  why: "§1 = exact commented pi block emitted by S1 (assert against THIS). §2 = the CRITICAL refinement
        of assertion (d) from literal `gpt-5.4` substring to bare-assignment form `# model = \"gpt-5.4`
        (S1's NOTE example `zai/gpt-5.4` would otherwise false-positive). §4 = the validated
        extractCommentedProviderBlock helper. §7 = old-vs-new matrix proving fail-on-old/pass-on-new."
  critical: "§2 is the make-or-break detail. A literal `!strings.Contains(piBlock,\"gpt-5.4\")` guard
             FAILS against the fixed code because S1's NOTE contains `zai/gpt-5.4`. The guard MUST be
             `strings.Contains(piBlock, \"# model = \\\"gpt-5.4\")` (bare assignment form)."

# MUST READ — the S1 PRP (the CONTRACT this test validates; running in parallel)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M2T1S1/PRP.md
  why: "Defines the exact S1 output this test asserts against: the commented-provider loop's
        `piCommented` branch blanks `other` and writes the 2-line NOTE with wording
        `# NOTE: pi is a multi-backend provider … # e.g. model = \"zai/gpt-5.4\" …`. The NOTE wording
        is why assertion (d) had to be refined (see findings §2)."
  critical: "Treat S1 as a CONTRACT — assume it lands EXACTLY as specified. This test must PASS against
             that output. Do NOT modify bootstrap.go (that is S1's exclusive domain)."

# MUST READ — the file being edited (the test file + the patterns/helpers to mirror)
- file: internal/config/bootstrap_test.go
  why: "LOCATE the companion by content: grep -n 'OtherInstalledCommented' (≈line 153) — it is the
        MIRROR (target=pi, claude installed → asserts claude keeps `# model = \"haiku\"`). Our new test
        is target=claude, pi installed → asserts pi IS blanked. The `extractStagerBlock` helper
        (≈line 276) is the IDIOM to mirror for `extractCommentedProviderBlock`. `assertContains`
        (≈line 293) is the existing substring helper. PLACE the new test right after
        TestBuildBootstrapConfig_OtherInstalledCommented; PLACE the new helper in the Helpers section
        right after `extractStagerBlock`."
  pattern: "Companion test: `content := buildBootstrapConfig(\"pi\", []string{\"pi\",\"claude\"}, nil)` →
            `strings.Contains(content, \"=== claude (installed)\")` + `# model = \"haiku\"`. New test:
            same shape, swapped providers, inverted expectation (blank instead of haiku)."
  gotcha: "Line numbers DRIFT (S1 is editing bootstrap.go in parallel; P1.M1.T1.S1/S2 already shifted
           bootstrap_test.go). Locate by content via grep, not by the contract's 101-127 / 153 numbers.
           The contract cites 'bootstrap_test.go:101-127' for the claude test — that range is STALE;
           the claude companion is now at ≈153 (locate by `grep -n 'OtherInstalledCommented'`)."

# MUST READ — the ground-truth output source (the production code under test; READ-ONLY here)
- file: internal/config/bootstrap.go
  why: "The commented-provider loop (`for _, name := range preferredBuiltins`, locate via
        `grep -n preferredBuiltins`) + `writeCommentedRoleBlock` (locate via
        `grep -n 'func writeCommentedRoleBlock'`) is what the test exercises via buildBootstrapConfig.
        S1 adds the `piCommented` branch there. This test does NOT edit bootstrap.go — it only calls
        buildBootstrapConfig and inspects the string."
  pattern: "writeCommentedRoleBlock renders `# model = %q\n` → for a blank model \"\" this is exactly
            `# model = \"\"` (the string the test asserts). For S1's blanked pi block that is the output."
  critical: "READ-ONLY. S1 owns this file. Do not edit it. If S1 has not yet landed when you implement,
             your test will FAIL on assertions b/c/d (that is EXPECTED — the test is gated on S1)."

# CONTEXT — the ProviderFlag distinction (semantic core: why pi is blanked, claude is not)
- file: internal/provider/builtin.go
  why: "pi: ProviderFlag = strPtr(\"--provider\") (NON-empty → multi-backend → bare model = FR-R5b error
        → blanked). claude: ProviderFlag = strPtr(\"\") (explicit-empty → single-backend → bare aliases
        legal → NOT blanked). This is why our test blanks pi and the companion keeps claude's haiku."
  critical: "Do not change builtin.go. This is reference-only context for the test's rationale."

# CONTEXT — test conventions (the doc map)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/test_patterns.md
  why: "§'Bootstrap Tests' lists the existing bootstrap test roster incl. the companion
        TestBuildBootstrapConfig_OtherInstalledCommented. §'Config/Provider Decoupling Invariant' notes
        internal/config MUST NOT import internal/provider — our test stays in package `config` and calls
        only buildBootstrapConfig (a pure string function), so no cross-package import is needed."
  critical: "The test is in `package config` (internal) so it can call the unexported buildBootstrapConfig
             — mirroring every existing Test* in bootstrap_test.go. Do NOT switch to `package config_test`."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  bootstrap.go        # READ-ONLY (S1's domain) — the commented-provider loop + writeCommentedRoleBlock
  bootstrap_test.go   # EDIT — add TestBuildBootstrapConfig_CommentedPiBlockBlanked + extractCommentedProviderBlock
  role_defaults.go    # READ-ONLY — DefaultModelsForProvider source (pi defaults = gpt-5.4*; claude = opus/sonnet/haiku)
internal/provider/
  builtin.go          # READ-ONLY — ProviderFlag distinction (pi=--provider; claude="") — rationale only
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
internal/config/bootstrap_test.go   # +TestBuildBootstrapConfig_CommentedPiBlockBlanked +extractCommentedProviderBlock
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the (d) refinement — do NOT use the literal gpt-5.4 substring): S1's guidance NOTE contains
//   the EXAMPLE `# e.g. model = "zai/gpt-5.4"` — a slash-PREFIXED model in a `# e.g.` comment line. A
//   `!strings.Contains(piBlock, "gpt-5.4")` guard FALSE-POSITIVES on the fixed code. The guard MUST be
//   scoped to the bare ASSIGNMENT form `# model = "gpt-5.4` (which matches ONLY the old bug's
//   `# model = "gpt-5.4"` / `# model = "gpt-5.4-mini"` / `# model = "gpt-5.4-nano"` lines). See findings §2.

// CRITICAL (scope assertions to the pi block): use extractCommentedProviderBlock to isolate the pi
//   commented block. Without it, a whole-content guard would also see the ACTIVE claude role models
//   (opus/sonnet/haiku) and — more importantly — could not cleanly express "the pi block contains X".
//   The active claude blocks above carry real models, so `# model = ""` would ONLY appear in the
//   commented pi block anyway, but isolating is correct, readable, and robust to future multi-provider
//   installed lists (e.g. target=claude, installed=[claude,pi,opencode]).

// GOTCHA (target=claude ⇒ claude is the ACTIVE provider, NOT a commented block): with
//   buildBootstrapConfig("claude", ["claude","pi"], nil), claude is the target so its [role.*] blocks
//   are UNCOMMENTED with real models; pi is installed and ≠ target so it is the ONLY commented block.
//   Therefore the companion sanity `strings.Contains(content, `model = "haiku"`)` checks the ACTIVE
//   claude [role.message] block (writeRoleBlock → `model = "haiku"`, no `#`), NOT a commented one.

// GOTCHA (line numbers drift — locate by content): S1 is editing bootstrap.go in parallel and prior
//   subtasks shifted bootstrap_test.go line numbers. The contract's "bootstrap_test.go:101-127" for the
//   claude test is STALE (it is now ≈153). Locate anchors via grep -n, never by hardcoded line numbers.

// GOTCHA (the count check is the strongest guard): asserting exactly 4 × `# model = ""` proves ALL four
//   roles are blanked in one shot. The old code emits ZERO blanks, so this alone fails-on-old; pair it
//   with (b)/(d) for clear failure messages.

// GOTCHA (gating on S1): this test is CONTRACTUALLY dependent on S1. If S1 has NOT landed when the
//   implementer runs the test, assertions b/count/c/d will FAIL — that is the CORRECT signal that S1 is
//   missing, not a bug in the test. Do NOT weaken the test to accommodate pre-S1 code.
```

## Implementation Blueprint

### Data models and structure
None. No types, no production code, no new packages. One test function + one string-helper function,
both in `package config` (internal, to call the unexported `buildBootstrapConfig`).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD the extractCommentedProviderBlock helper to internal/config/bootstrap_test.go
  - PLACE: in the "Helpers" section, immediately AFTER the existing `extractStagerBlock` helper
    (locate via `grep -n 'func extractStagerBlock' internal/config/bootstrap_test.go`).
  - IMPLEMENT: the function exactly as shown in the "What" section's code block (header search +
    `strings.Index(rest[1:], marker)` for `\n# ===` and `\n# ---`, take the min, return the slice).
  - ADD a doc comment explaining: isolates a single commented-provider block; mirrors extractStagerBlock;
    bounds on the next provider block OR the [generation] dashed separator OR EOF.
  - NO new imports (strings already imported in bootstrap_test.go).
  - VERIFY: `gofmt -l internal/config/bootstrap_test.go` empty after adding.

Task 2: ADD TestBuildBootstrapConfig_CommentedPiBlockBlanked to internal/config/bootstrap_test.go
  - PLACE: immediately AFTER `TestBuildBootstrapConfig_OtherInstalledCommented` (locate via
    `grep -n 'func TestBuildBootstrapConfig_OtherInstalledCommented'`).
  - IMPLEMENT: the function exactly as shown in the "What" section's code block:
      content := buildBootstrapConfig("claude", []string{"claude", "pi"}, nil)
      (a) assert "# === pi (installed)" present (Fatalf if missing — can't isolate without it)
      piBlock := extractCommentedProviderBlock(content, "pi")
      (b) assert piBlock contains `# model = ""`
      (count) assert strings.Count(piBlock, `# model = ""`) == 4
      (c) assert piBlock contains "multi-backend provider"
      (d) assert piBlock does NOT contain `# model = "gpt-5.4`   ← REFINED (see findings §2)
      (companion) assert content contains `model = "haiku"` (active claude not blanked)
  - ADD a doc comment explaining: guards Issue 2; mirror of OtherInstalledCommented; the (d) refinement
    rationale (literal gpt-5.4 appears in S1's NOTE example).
  - NO new imports.
  - NAMING: TestBuildBootstrapConfig_CommentedPiBlockBlanked (EXACT — the -run regex
    `TestBuildBootstrapConfig_CommentedPi` must match it).

Task 3: VERIFY — build, vet, format, the new test, the companion, full race suite, lint
  - go build ./...
  - go vet ./internal/config/...
  - gofmt -l internal/config/bootstrap_test.go   # empty
  - go test ./internal/config/ -v -run TestBuildBootstrapConfig_CommentedPi   # NEW test PASS
  - go test ./internal/config/ -run 'BuildBootstrapConfig' -v                 # companion + all bootstrap PASS
  - make test ; make lint
  - scope guard: `git diff --name-only` == only internal/config/bootstrap_test.go
```

### Implementation Patterns & Key Details

```go
// PATTERN: mirror the existing companion (TestBuildBootstrapConfig_OtherInstalledCommented) with
// providers SWAPPED and expectation INVERTED.
//   Companion: buildBootstrapConfig("pi",  []string{"pi","claude"}, nil) → claude commented, NOT blanked (`# model = "haiku"`).
//   New test:  buildBootstrapConfig("claude",[]string{"claude","pi"}, nil) → pi commented,     blanked    (`# model = ""`).

// PATTERN: isolate the block before asserting (mirror extractStagerBlock).
piBlock := extractCommentedProviderBlock(content, "pi")  // header → next \n# === / \n# --- / EOF

// PATTERN: the (d) guard MUST target the bare ASSIGNMENT, not the literal token.
//   WRONG: if strings.Contains(piBlock, "gpt-5.4") { ... }     // false-positive on S1's NOTE `zai/gpt-5.4`
//   RIGHT: if strings.Contains(piBlock, `# model = "gpt-5.4`) { ... }  // matches only the old bug's assignments
```

### Integration Points

```yaml
NO database / migration / routes / new types / new imports / production-code edits / docs edits.

TEST FILE (internal/config/bootstrap_test.go):
  - +extractCommentedProviderBlock helper (Helpers section, after extractStagerBlock)
  - +TestBuildBootstrapConfig_CommentedPiBlockBlanked (after TestBuildBootstrapConfig_OtherInstalledCommented)

SCOPE FENCES: NO bootstrap.go edit (S1's domain); NO docs edit (item DOCS: none); NO touch to the
  companion TestBuildBootstrapConfig_OtherInstalledCommented (keep it green, don't rewrite it); NO
  ValidateModel assertions (commented TOML is inert → can't reach ValidateModel; that net is the
  parallel P1.M1.T2.S1); NO exampleConfigTemplate/internal/cmd/config.go edit.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet (the test compiles against the S1-fixed bootstrap.go).
go build ./...
go vet ./internal/config/...

# Format.
gofmt -l internal/config/bootstrap_test.go
# Expected: empty. If listed: gofmt -w internal/config/bootstrap_test.go.

# Lint.
make lint      # golangci-lint v1.61 (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors (the new test + helper are trivially clean).

# Scope guard: ONLY the test file changed.
git diff --name-only
# Expected: internal/config/bootstrap_test.go  (exactly this 1; NO bootstrap.go, NO docs).
```

### Level 2: Unit Tests (Component Validation)

```bash
# THE new test (the primary deliverable) — runs ONLY TestBuildBootstrapConfig_CommentedPiBlockBlanked.
go test ./internal/config/ -v -run TestBuildBootstrapConfig_CommentedPi
# Expected: PASS — (a) header present, (b) `# model = ""` present, (count) 4 blanks, (c) NOTE present,
#           (d) no bare `# model = "gpt-5.4` assignment, (companion) active claude `model = "haiku"` present.

# All bootstrap tests — incl. the companion claude-not-blanked regression.
go test ./internal/config/ -run 'BuildBootstrapConfig|GenerateBootstrapConfig' -v
# Expected: ALL PASS — especially:
#   TestBuildBootstrapConfig_OtherInstalledCommented (claude commented block keeps `# model = "haiku"`)
#   TestBuildBootstrapConfig_ValidTOML (the {claude,[claude,pi]} case parses — blanked + # comments are inert TOML)
#   TestBuildBootstrapConfig_Pi (active pi block — unaffected)

# Full race suite.
make test
# Expected: green (race detector).
```

### Level 3: Integration Testing (System Validation)

```bash
# Regression-equivalence proof: generate a real config with claude target + pi installed and eyeball
# the commented pi block (deterministic — same code path the unit test exercises).
# (The unit test above IS the deterministic proof; this is the human-inspection corroboration.)
cat > /tmp/sc_inspect.go <<'EOF'
package main
import ("fmt"; "os"; "github.com/dustin/stagecoach/internal/config")
func main() {
	_ = os.Args
	fmt.Print(config.GenerateBootstrapConfig("claude"))
	// NOTE: GenerateBootstrapConfig auto-detects installed providers via $PATH; to FORCE the
	// [claude,pi] installed set deterministically, use the unexported buildBootstrapConfig via a
	// scratch _test.go (as the unit test does). The unit test is the authoritative deterministic check.
}
EOF
# (Optional corroboration only — the unit test in Level 2 is the binding proof.)
rm -f /tmp/sc_inspect.go

# Confirm the new test name is matched by the contract's -run regex.
go test ./internal/config/ -run 'TestBuildBootstrapConfig_CommentedPi' -v -list '.*' 2>/dev/null | grep CommentedPi || \
  go test ./internal/config/ -v -run TestBuildBootstrapConfig_CommentedPi 2>&1 | grep -E 'RUN|PASS|FAIL'
# Expected: TestBuildBootstrapConfig_CommentedPiBlockBlanked is RUN and PASS.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1: the new test exists and is correctly named.
grep -n 'func TestBuildBootstrapConfig_CommentedPiBlockBlanked' internal/config/bootstrap_test.go
# Expected: exactly 1 match.

# Grep guard 2: the helper exists and is correctly named.
grep -n 'func extractCommentedProviderBlock' internal/config/bootstrap_test.go
# Expected: exactly 1 match.

# Grep guard 3: the (d) guard uses the REFINED bare-assignment form, NOT the literal substring.
grep -n '# model = "gpt-5.4' internal/config/bootstrap_test.go
# Expected: exactly 1 match (inside the `strings.Contains(piBlock, ...)` assertion).
grep -n 'strings.Contains(piBlock, "gpt-5.4")' internal/config/bootstrap_test.go
# Expected: ZERO matches (the literal-substring form is FORBIDDEN — it false-positives on S1's NOTE).

# Grep guard 4: the companion test is UNMODIFIED (still asserts claude's haiku).
grep -n 'func TestBuildBootstrapConfig_OtherInstalledCommented' internal/config/bootstrap_test.go
grep -n '# model = "haiku"' internal/config/bootstrap_test.go
# Expected: the companion function is present and still asserts `# model = "haiku"`.

# Grep guard 5: NO production code changed.
git diff --name-only | grep -v 'bootstrap_test.go'
# Expected: empty (only bootstrap_test.go changed).

# Old-code simulation (optional confidence): temporarily check the test would catch the bug by
# confirming the refined (d) substring matches the known old output shape.
go test ./internal/config/ -run TestBuildBootstrapConfig_CommentedPiBlockBlanked -v
# (Full fail-on-old proof is in findings §7 — the refined (d) + count + (b) + (c) all fire on a
#  bare-gpt-5.4 block; verified by simulated old-block string.)

# Regression: all bootstrap tests still green.
go test ./internal/config/ -run 'BuildBootstrapConfig|GenerateBootstrapConfig' -v
# Expected: all PASS.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/config/...` clean
- [ ] `gofmt -l internal/config/bootstrap_test.go` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) green — full suite

### Feature Validation
- [ ] `TestBuildBootstrapConfig_CommentedPiBlockBlanked` PASS via `go test ./internal/config/ -v -run TestBuildBootstrapConfig_CommentedPi`
- [ ] (a) commented pi header `# === pi (installed)` asserted present
- [ ] (b) commented pi block contains `# model = ""`
- [ ] (count) exactly 4 × `# model = ""` (all roles blanked)
- [ ] (c) commented pi block contains the `multi-backend provider` NOTE
- [ ] (d) commented pi block contains NO `# model = "gpt-5.4` bare assignment (REFINED — see findings §2)
- [ ] (companion) active claude block still has `model = "haiku"` (claude NOT blanked)

### Scope-Boundary Validation
- [ ] `git diff --name-only` == only `internal/config/bootstrap_test.go`
- [ ] NO change to `internal/config/bootstrap.go` (S1's domain)
- [ ] NO change to `docs/*` (item DOCS: none — test-only)
- [ ] `TestBuildBootstrapConfig_OtherInstalledCommented` UNMODIFIED and still GREEN
- [ ] NO ValidateModel assertions added (out of scope — commented TOML is inert)
- [ ] NO `internal/cmd/config.go` / `exampleConfigTemplate` change

### Code Quality & Docs
- [ ] New test + helper follow the existing `package config` internal-test idiom
- [ ] `extractCommentedProviderBlock` mirrors the `extractStagerBlock` idiom
- [ ] Doc comments explain the Issue-2 guard AND the (d) refinement rationale
- [ ] Anchors located by content (grep), not stale line numbers

---

## Anti-Patterns to Avoid

- ❌ Don't use the literal `gpt-5.4` substring for assertion (d). S1's NOTE contains the EXAMPLE
  `# e.g. model = "zai/gpt-5.4"` — a CORRECT, slash-prefixed model in a `# e.g.` comment. A literal
  `!strings.Contains(piBlock, "gpt-5.4")` guard FAILS against the fixed code. Scope (d) to the bare
  ASSIGNMENT form `# model = "gpt-5.4` (findings §2). This is the single most important rule.
- ❌ Don't weaken the test to pass against pre-S1 code. The test is CONTRACTUALLY gated on S1. If it
  fails on b/count/c/d before S1 lands, that is the CORRECT signal — do NOT relax the assertions.
- ❌ Don't assert on the WHOLE content without isolating the pi block. Use `extractCommentedProviderBlock`
  so the active claude models (and any future multi-provider installed list) don't interfere. The
  `extractStagerBlock` idiom exists for exactly this reason — mirror it.
- ❌ Don't anchor to the contract's line numbers (bootstrap_test.go:101-127). They are STALE (the claude
  companion is now ≈153; S1/prior subtasks shifted everything). Locate anchors with `grep -n`.
- ❌ Don't modify `bootstrap.go`. That is S1's exclusive domain (running in parallel). This subtask is
  test-only — it CALLS `buildBootstrapConfig` and inspects the string.
- ❌ Don't rewrite or touch the companion `TestBuildBootstrapConfig_OtherInstalledCommented`. It already
  covers the claude-not-blanked side (target=pi, claude installed → `# model = "haiku"`). Just keep it
  green. Your new test is its mirror (target=claude, pi installed → `# model = ""`).
- ❌ Don't add ValidateModel assertions. Commented TOML is inert — it is never decoded, so ValidateModel
  can never reach the commented pi models. That regression net is the parallel P1.M1.T2.S1 and is
  structurally incapable of covering commented blocks; this string-inspection test is the ONLY way.
- ❌ Don't switch to `package config_test`. The existing tests are in `package config` (internal) so they
  can call the unexported `buildBootstrapConfig`. Switching packages would break compilation (the symbol
  is unexported) and violate the file's established convention.
- ❌ Don't add a `claude commented block is not blanked` assertion to THIS test. With target=claude,
  claude is the ACTIVE provider (not a commented block) — there is no commented claude block to assert
  against. The commented-claude-not-blanked case is the companion test's job (target=pi). The companion
  sanity assertion here (`model = "haiku"`) checks the ACTIVE claude block, which is a different (but
  still useful) confirmation.

---

## Confidence Score: 10/10

This is a single test + one string helper, test-only, with the verbatim code spelled out in the "What"
section. The ground-truth output it asserts against was captured live from the S1-fixed working tree
(findings §1). The one non-obvious trap — the literal-`gpt-5.4` guard false-positiving on S1's NOTE
example — was discovered, validated, and resolved (findings §2: use the bare-assignment form
`# model = "gpt-5.4`). The extraction helper was validated against live output (findings §4). The
old-vs-new matrix (findings §7) proves the test fails-on-old (4 independent assertions) and
passes-on-new. The companion-test relationship and scope fences are explicit. One-pass success is
essentially guaranteed provided S1 has landed (the test is correctly gated on it).
