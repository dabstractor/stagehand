---
name: "P1.M1.T1.S2 — Add MergeManifest scalar clause for SessionMode (config-overridable field merge)"
description: |
  Add the regime-1 scalar-merge clause for `SessionMode` to `MergeManifest` (`internal/provider/merge.go`)
  so a user's `[provider.<name>] session_mode = ...` override field-merges per FR-37a (global → repo →
  git-config). S1 already landed the `SessionMode *string` field + Resolve + Validate on Manifest; merge.go
  currently has ZERO SessionMode references, so an override is silently DROPPED today. The fix is ONE
  clause — `if override.SessionMode != nil { out.SessionMode = override.SessionMode }` — placed right after
  the ProviderFlag clause (regime-1). SessionMode is *string, so it merges IDENTICALLY to every other scalar
  (nil ⇒ inherit; non-nil incl. explicit "" ⇒ override). Enables: a user setting `session_mode = ""` on pi
  disables multi-turn (overrides the built-in "append"); omitting inherits; setting "append" on a non-append
  provider overrides up (user choice; FR-T9 duty is on SHIPPED defaults, not user config). +3 test edits
  (sampleBase, PartialOverride, ExplicitZeroPointerWins). NO docs (ride with S5).
---

## Goal

**Feature Goal**: Make `MergeManifest` field-merge `SessionMode` so a user's `[provider.<name>] session_mode`
override threads through the config-override merge (FR-37a) — critically, an explicit `session_mode = ""`
on pi overrides the built-in `"append"` (disabling multi-turn fallback for pi), while an omitted key
inherits the built-in. This closes the gap S1 left: the `SessionMode *string` field exists but merge.go
never copies an override's value, so it is silently dropped.

**Deliverable** (1 production clause + 3 test edits, all in `internal/provider/`):
1. `internal/provider/merge.go` — regime-1: add `if override.SessionMode != nil { out.SessionMode = override.SessionMode }` immediately after the ProviderFlag clause.
2. `internal/provider/merge_test.go` — `sampleBase()`: add `SessionMode: strPtr("append")` (pi is the append provider).
3. `internal/provider/merge_test.go` — `TestMergeManifest_PartialOverride_OnlyTouchedFieldChanges`: add a `{"SessionMode", ...}` row to the scalar table.
4. `internal/provider/merge_test.go` — `TestMergeManifest_ExplicitZeroPointerWins`: add `SessionMode: strPtr("")` override + assertion (the disable-multi-turn payoff).

**Success Definition**: A user override setting `[provider.pi] session_mode = ""` yields a merged manifest
whose `*SessionMode == ""` (not the built-in `"append"`); an omitted key inherits the built-in. `go build/
vet/gofmt` clean; `go test -race ./...` green; `grep SessionMode internal/provider/merge.go` returns the
one new clause. No edit to manifest.go (S1), render.go (S3), builtin.go/pi.toml (S4), or docs (S5).

## User Persona

**Target User**: The Stagehand user who wants to disable multi-turn fallback for a session-capable provider
(e.g. `session_mode = ""` in `[provider.pi]` to force one-shot + rescue), or to enable it on a provider
they've personally verified (FR-T9) — and the contributor wiring S3 (RenderMultiTurn capability gate) and
S4 (pi's `"append"` value).

**Use Case**: A user drops `[provider.pi] session_mode = ""` into `.stagehand.toml`. The registry merges
this override onto pi's built-in manifest; after S2 the merged `SessionMode` is `""` (was silently
`"append"` before S2). S3's gate then sees `*r.SessionMode != "append"` → multi-turn skipped.

**Pain Points Addressed**: Today the override is silently dropped (merge.go has no clause) — the user's
explicit "disable multi-turn for pi" config has no effect, contradicting FR-37a's field-merge promise.

## Why

- **Closes the S1 gap.** S1 landed the `SessionMode *string` field + Resolve + Validate, but the field is
  dead at the merge layer — `MergeManifest` never copies an override's value, so config cannot change it.
  S2 is the one-clause wiring that makes the field config-overridable.
- **FR-37a field-merge across layers.** A `[provider.<name>]` block merges field-by-field (global → repo →
  git-config): a field the user sets overrides that one field; omitted fields inherit. SessionMode is the
  one scalar S1 added that S2 hasn't yet merged. Without S2, the FR-37a promise is broken for this field.
- **Enables the disable-multi-turn use case.** The pointer-scalar design (S1) exists precisely so an
  explicit `""` (non-nil) overrides the built-in `"append"`. The merge clause is what honors that — a user
  can force one-shot+rescue on pi by writing `session_mode = ""`.
- **Lowest-risk change possible.** SessionMode is `*string`; it merges IDENTICALLY to ProviderFlag/Output/
  Experimental. The clause is a literal copy of the regime-1 rule. One line + tests.

## What

One scalar-merge clause in `MergeManifest`'s regime-1 block, plus three test edits that make the merge
MEANINGFULLY covered (sampleBase carries a non-nil SessionMode; the partial-override test proves it
survives an unrelated override; the explicit-zero test proves `""` overrides the built-in). No behavior
change beyond honoring the override.

### Success Criteria

- [ ] `merge.go` regime-1 has `if override.SessionMode != nil { out.SessionMode = override.SessionMode }` immediately after the ProviderFlag clause.
- [ ] `sampleBase()` sets `SessionMode: strPtr("append")` (after ProviderFlag).
- [ ] `TestMergeManifest_PartialOverride_OnlyTouchedFieldChanges` scalar table includes `{"SessionMode", merged.SessionMode, base.SessionMode}`.
- [ ] `TestMergeManifest_ExplicitZeroPointerWins` sets `SessionMode: strPtr("")` and asserts `*merged.SessionMode == ""`.
- [ ] An override `{SessionMode: strPtr("")}` on a base with `"append"` yields `*merged.SessionMode == ""`.
- [ ] An empty override leaves `merged.SessionMode == base.SessionMode` (inherit).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] NO edit to manifest.go (S1), render.go (S3), builtin.go/providers/*.toml (S4), docs (S5).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact regime-1 block (with the ProviderFlag insertion point),
the verbatim new clause, the three test edits (with the assertion idioms), and confirms S1 is already
landed (field/Resolve/Validate) while merge.go has ZERO SessionMode. The pointer-scalar regime-1 rule is
explained (nil ⇒ inherit; non-nil incl. "" ⇒ override) with the FR-37a + disable-multi-turn rationale.

### Documentation & References

```yaml
# MUST READ — the MergeManifest placement + the scalar regime rule
- docfile: plan/009_5c53066d64b3/architecture/research-provider.md
  why: "§4 confirms MergeManifest lives in internal/provider/merge.go (NOT registry.go); NewRegistry (registry.go:42-55) calls it per override key; regime-1 (scalar pointer fields) does `if override.X != nil { out.X = override.X }`. A plain *string SessionMode merges IDENTICALLY to every other scalar."
  critical: "§4 is the authority that SessionMode is a regime-1 scalar (like ProviderFlag/Output) — NOT a slice (regime-2) or a map (regime-3). The clause is `!= nil`, NOT `len > 0`."

- docfile: plan/009_5c53066d64b3/P1M1T1S1/PRP.md
  why: "S1's contract: SessionMode is *string (pointer-scalar, go-toml no omitempty), slots between ProviderFlag and BareFlags on the STRUCT, Resolve defaults to strPtr(''), Validate enforces ''|'append'. S1's downstream-hook note literally specifies S2's clause: `if override.SessionMode != nil { out.SessionMode = override.SessionMode }`."
  critical: "S1 is ALREADY LANDED in the working tree (manifest.go:66/121-123/177-178). S2 consumes S1's field as-is — it does NOT re-edit manifest.go. The merge clause goes in merge.go ONLY."

- docfile: plan/009_5c53066d64b3/P1M1T1S2/research/s2_implementation_notes.md
  why: "Distilled S2 findings: the single production edit (right after ProviderFlag), the FR-37a semantics it enables, the 3 test edits (sampleBase + PartialOverride + ExplicitZeroPointerWins), the sampleBase-safety proof, and the S1/S3/S4/S5 scope boundary."

# The files under edit
- file: internal/provider/merge.go
  why: "EDIT (1 clause). Regime-1 scalar block: insert `if override.SessionMode != nil { out.SessionMode = override.SessionMode }` immediately after the ProviderFlag clause (merge.go:56-58), before the blank line + Output block."
  pattern: "LITERAL copy of the regime-1 rule every other scalar follows: `if override.X != nil { out.X = override.X }`. ProviderFlag (lines 56-58) is the immediate precedent/neighbor. No doc-comment update — MergeManifest's regime-1 description (line 9) is GENERIC ('Scalar pointer fields ... override.Field != nil → result takes override.Field'), so it already covers SessionMode."
  gotcha: "SessionMode is *string ⇒ use `!= nil` (the scalar rule), NOT `len(override.SessionMode) > 0` (that's the slice rule) and NOT a map merge. Place it right after ProviderFlag (the contract specifies this; mirrors the struct's ProviderFlag→SessionMode adjacency)."

- file: internal/provider/merge_test.go
  why: "EDIT (3 spots). (a) sampleBase() (line 10): add `SessionMode: strPtr(\"append\")` after ProviderFlag (pi is the append provider — realistic non-nil so merge coverage is meaningful). (b) TestMergeManifest_PartialOverride_OnlyTouchedFieldChanges (line 39): add `{\"SessionMode\", merged.SessionMode, base.SessionMode}` to the scalar table (line ~62, after the ProviderFlag row). (c) TestMergeManifest_ExplicitZeroPointerWins (line 111): add `SessionMode: strPtr(\"\")` to the override + an assertion."
  pattern: "sampleBase already carries non-default scalar values (Experimental=boolPtr(true), ProviderFlag=strPtr('--provider')) — SessionMode=strPtr('append') follows that pattern. The PartialOverride scalar table iterates {name,got,want} *string fields. ExplicitZeroPointerWins already tests explicit *false/*\"\" overrides (StripCodeFence/PrintFlag/Experimental) — SessionMode=\"\" joins them."
  gotcha: "Adding SessionMode to sampleBase is SAFE for every existing test (verified): EmptyOverrideIsIdentity (DeepEqual holds — empty override keeps base.SessionMode); DoesNotMutateInputs (Env-only override → SessionMode clause doesn't fire → base untouched); MergedResultValidates ('append' passes Validate). sampleBase is a TEST FIXTURE (pi-shape) — setting 'append' here is NOT the shipped builtin value (that's S4's job on builtinPi)."

# Read-only refs (do NOT edit in S2)
- file: internal/provider/manifest.go
  why: "READ-ONLY (S1 landed). SessionMode *string (line 66); Resolve strPtr('') default (177-178); Validate ''|'append' enum (121-123). S2 consumes the field; it does NOT change it."
- file: internal/provider/registry.go
  why: "READ-ONLY. NewRegistry (lines 42-55) calls MergeManifest per [provider.<name>] override key — the caller that makes S2's clause effective. No edit."
- file: internal/provider/render.go
  why: "READ-ONLY (S3). RenderMultiTurn + the *r.SessionMode == 'append' capability gate land in S3. S2 only ensures the MERGED value reaches the registry; S3 reads it."

# PRD authority (already in the selected content)
- prd: PRD.md §16.1 / FR-37a (field-merge across layers); §9.24 FR-T8 (session_mode "" default | "append"); §12.1 (session_mode between provider_flag and bare_flags).
  why: "FR-37a is WHY the merge must be field-by-field (a session_mode override must not erase other fields). FR-T8 is the enum S1's Validate already enforces."
```

### Current Codebase Tree (relevant slice)

```bash
stagehand/
└── internal/provider/
    ├── manifest.go        # READ-ONLY (S1 landed: SessionMode field + Resolve + Validate)
    ├── merge.go           # EDIT: + regime-1 SessionMode clause (after ProviderFlag)
    ├── merge_test.go      # EDIT: sampleBase +SessionMode; PartialOverride +row; ExplicitZero +override
    ├── registry.go        # READ-ONLY — NewRegistry calls MergeManifest (the caller)
    └── render.go          # READ-ONLY (S3 — RenderMultiTurn capability gate)
```

### Desired Codebase Tree After S2

```bash
stagehand/
└── (only existing files modified — no new files)
    internal/provider/merge.go        # +1 regime-1 SessionMode clause
    internal/provider/merge_test.go   # sampleBase +1 field; PartialOverride +1 row; ExplicitZero +1 override+assertion
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/provider/merge.go` | MODIFY | + regime-1 `if override.SessionMode != nil { out.SessionMode = override.SessionMode }` (after ProviderFlag). **Only production file.** |
| `internal/provider/merge_test.go` | MODIFY | sampleBase +`SessionMode: strPtr("append")`; PartialOverride +SessionMode row; ExplicitZero +`SessionMode: strPtr("")` override + assertion. |

**Explicitly NOT touched**: `manifest.go` (S1 — landed), `render.go` (S3 = P1.M1.T1.S3), `builtin.go` /
`providers/pi.toml` (S4 = P1.M1.T1.S4 — the shipped pi `"append"` value), `docs/*` (S5 = P1.M1.T1.S5), any
other package, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (regime-1 scalar, NOT slice/map): SessionMode is *string ⇒ the clause is `if override.SessionMode != nil`
// (the scalar rule), exactly like ProviderFlag/Output/Experimental. Do NOT use `len(override.SessionMode) > 0`
// (the slice rule) or a fresh-map merge (the Env/ReasoningLevels rule). nil ⇒ inherit base; non-nil (incl.
// explicit "") ⇒ override. This is the whole point of S1's *string design (go-toml no omitempty).

// CRITICAL (placement): insert right AFTER the ProviderFlag clause (merge.go:56-58), per the contract.
// This mirrors the struct's ProviderFlag→SessionMode adjacency (S1 placed the field between them). The
// blank line at :59 then Output at :60 stay intact below the new clause.

// CRITICAL (sampleBase is a fixture, not the builtin): S2's sampleBase() sets SessionMode=strPtr("append")
// to make the merge tests meaningful — this is the pi-shape TEST FIXTURE, NOT the shipped builtinPi value.
// The shipped pi value is S4's job (builtin.go + providers/pi.toml). Do NOT conflate them; do NOT edit
// builtin.go here.

// GOTCHA (sampleBase safety): adding SessionMode to sampleBase is safe for every existing merge test:
// EmptyOverrideIsIdentity (reflect.DeepEqual holds — empty override keeps base.SessionMode);
// DoesNotMutateInputs (Env-only override → the SessionMode clause doesn't fire → base.SessionMode untouched);
// MergedResultValidates ("append" passes S1's Validate enum). Verified — no existing test breaks.

// GOTCHA (no doc-comment update): MergeManifest's regime-1 description (merge.go:9) is GENERIC —
// "Scalar pointer fields (*string / *bool): override.Field != nil → result takes override.Field" — it does
// NOT enumerate fields, so it already covers SessionMode. (Only regime-2 enumerates slice fields; unrelated.)

// GOTCHA (S1 is landed): manifest.go ALREADY has the field + Resolve + Validate. Do NOT re-edit manifest.go.
// The merge clause is the ONLY production gap. Confirmed: `grep SessionMode internal/provider/merge.go` →
// ZERO today; → 2 lines (the if + the assignment) after S2.
```

## Implementation Blueprint

### Data models and structure

No schema change — S1 landed `SessionMode *string` on Manifest. S2 adds one merge clause (regime-1 scalar)
and three test edits. The relevant existing precedent (the model to mirror — unchanged):

```go
// merge.go regime-1 — the rule every *string/*bool scalar follows (SessionMode joins verbatim)
if override.ProviderFlag != nil {
    out.ProviderFlag = override.ProviderFlag
}
// ... S2 adds, immediately after:
if override.SessionMode != nil {
    out.SessionMode = override.SessionMode
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: merge.go — add the regime-1 SessionMode clause
  - LOCATE: the ProviderFlag clause in regime-1 (merge.go:56-58):
        if override.ProviderFlag != nil {
            out.ProviderFlag = override.ProviderFlag
        }
  - INSERT immediately AFTER it (before the blank line + Output block):
        if override.SessionMode != nil {
            out.SessionMode = override.SessionMode // FR-37a field-merge: explicit "" disables multi-turn (overrides the built-in "append")
        }
  - GUARD: `!= nil` (scalar rule). NOT `len > 0`, NOT a map merge.
  - DO NOT: touch manifest.go (S1), render.go (S3), builtin.go (S4); update the regime-1 doc comment (generic already).

Task 2: merge_test.go — sampleBase() gains SessionMode
  - LOCATE: sampleBase() (line 10), the `ProviderFlag: strPtr("--provider"),` line (line 21).
  - ADD immediately after it (mirrors the struct's ProviderFlag→SessionMode order):
        SessionMode:      strPtr("append"), // pi is the FR-T8 "append" provider — realistic non-nil for merge tests
  - WHY: a non-nil base SessionMode makes the merge tests meaningful (nil→nil would pass trivially).
  - SAFE (verified): EmptyOverrideIsIdentity / DoesNotMutateInputs / MergedResultValidates all stay green.

Task 3: merge_test.go — extend TestMergeManifest_PartialOverride_OnlyTouchedFieldChanges
  - LOCATE: the scalar table (merge_test.go:50-72), the `{"ProviderFlag", merged.ProviderFlag, base.ProviderFlag}` row (line 62).
  - ADD immediately after the ProviderFlag row:
        {"SessionMode", merged.SessionMode, base.SessionMode},
  - WHY: proves an unrelated override (DefaultModel) leaves SessionMode untouched (survives == base "append").
  - The table's existing nil-safe compare (tc.got==nil && tc.want==nil → continue) handles the *string deref.

Task 4: merge_test.go — extend TestMergeManifest_ExplicitZeroPointerWins (the FR-37a payoff)
  - LOCATE: TestMergeManifest_ExplicitZeroPointerWins (line 111). The override Manifest literal (lines 113-117)
    currently sets StripCodeFence/PrintFlag/Experimental.
  - ADD to the override literal:
        SessionMode:    strPtr(""), // base has "append" → explicit "" must win (disable multi-turn for pi)
  - ADD an assertion (alongside the existing StripCodeFence/PrintFlag/Experimental assertions):
        if merged.SessionMode == nil || *merged.SessionMode != "" {
            t.Errorf("explicit session_mode=\"\" lost (got %v)", merged.SessionMode)
        }
  - WHY: this is THE contract test — a user setting `session_mode = ""` on pi (base "append") disables
    multi-turn. Without S2's clause, merged.SessionMode would stay "append" (the override silently dropped).

Task 5: VALIDATE
  - RUN: gofmt -w internal/provider/merge.go internal/provider/merge_test.go
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test -race ./internal/provider/ -v -run 'TestMergeManifest'
  - RUN: go test -race ./...   # full suite green
  - GREP: confirm the clause landed: `grep -n "override.SessionMode != nil" internal/provider/merge.go` → 1 match.
  - FIX-FORWARD: if ExplicitZeroPointerWins fails on SessionMode (got "append"), the clause wasn't added — re-check Task 1.
```

### Implementation Patterns & Key Details

```go
// === merge.go — the new clause in context (regime-1, after ProviderFlag) ===
	if override.ProviderFlag != nil {
		out.ProviderFlag = override.ProviderFlag
	}
	if override.SessionMode != nil {
		out.SessionMode = override.SessionMode // FR-37a field-merge: explicit "" disables multi-turn (overrides the built-in "append")
	}

	if override.Output != nil {
		out.Output = override.Output
	}
```

```go
// === merge_test.go — the 3 edits ===
// (a) sampleBase — after ProviderFlag:
		ProviderFlag:     strPtr("--provider"),
		SessionMode:      strPtr("append"), // pi is the FR-T8 "append" provider — realistic non-nil for merge tests
		Output:           strPtr("raw"),

// (b) PartialOverride scalar table — after the ProviderFlag row:
		{"ProviderFlag", merged.ProviderFlag, base.ProviderFlag},
		{"SessionMode", merged.SessionMode, base.SessionMode},

// (c) ExplicitZeroPointerWins — override + assertion:
	merged := MergeManifest(base, Manifest{
		StripCodeFence: boolPtr(false),
		PrintFlag:      strPtr(""),
		Experimental:   boolPtr(false),
		SessionMode:    strPtr(""), // base has "append" → explicit "" must win (disable multi-turn for pi)
	})
	...
	if merged.SessionMode == nil || *merged.SessionMode != "" {
		t.Errorf("explicit session_mode=\"\" lost (got %v)", merged.SessionMode)
	}
```

### Integration Points

```yaml
MERGE (internal/provider/merge.go MergeManifest regime-1):
  - clause added: "if override.SessionMode != nil { out.SessionMode = override.SessionMode }"

TESTS (internal/provider/merge_test.go):
  - sampleBase: +SessionMode strPtr("append")
  - TestMergeManifest_PartialOverride_OnlyTouchedFieldChanges: +{"SessionMode",...} row
  - TestMergeManifest_ExplicitZeroPointerWins: +SessionMode strPtr("") override + assertion

CONSUMED BY (read-only — the call path S2's clause enables):
  - registry.go NewRegistry (lines 42-55): calls MergeManifest per [provider.<name>] override key, across
    layers (global → repo → git-config). After S2, a session_mode override threads through.
  - render.go RenderMultiTurn (S3): reads *r.SessionMode == "append" (the capability gate). S2 ensures the
    MERGED value reaches the registry; S3 reads it.

NO-TOUCH (explicitly — owned by sibling subtasks):
  - internal/provider/manifest.go     # S1 (LANDED): SessionMode field + Resolve + Validate
  - internal/provider/render.go       # S3 (P1.M1.T1.S3): RenderMultiTurn + capability gate
  - internal/provider/builtin.go + providers/pi.toml   # S4 (P1.M1.T1.S4): pi SessionMode="append" (FR-T9)
  - docs/*                            # S5 (P1.M1.T1.S5): manifest-schema doc / providers.md / configuration.md
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S2):
  - S3: RenderMultiTurn errors if *r.SessionMode != "append" (FR-T8/T9); S2's merged value is what S3 reads.
  - S4: builtinPi sets SessionMode: strPtr("append"); providers/pi.toml adds session_mode = "append".
    (S2's sampleBase "append" is a TEST FIXTURE, independent of S4's shipped value.)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand

gofmt -w internal/provider/merge.go internal/provider/merge_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/provider/...   # Expected: exit 0
go build ./...                   # Expected: exit 0
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagehand

# The merge tests — the ExplicitZeroPointerWins SessionMode assertion is the FR-37a payoff
go test -race ./internal/provider/ -v -run 'TestMergeManifest'

# Full provider suite
go test -race ./internal/provider/ -v

# Expected: ALL merge tests PASS. ExplicitZeroPointerWins proves an explicit session_mode="" on a base
# with "append" yields *merged.SessionMode == "" (without S2's clause it would stay "append" → FAIL).
# PartialOverride proves an unrelated override leaves SessionMode == base ("append").
```

### Level 3: Whole-Repository Regression

```bash
cd /home/dustin/projects/stagehand

go test -race ./...              # Expected: ALL packages green
go vet ./...                     # Expected: exit 0

# Confirm the clause landed (1 match) and there's no stray slice/map guard for SessionMode
grep -n "override.SessionMode != nil" internal/provider/merge.go   # Expected: 1 match (the clause)
grep -n "len(override.SessionMode" internal/provider/merge.go      # Expected: ZERO (it's not a slice)

# Confirm ONLY the 2 intended files changed
git diff --stat -- internal/ pkg/ cmd/ docs/ providers/
# Expected: internal/provider/merge.go + internal/provider/merge_test.go only.
```

### Level 4: Behavior Smoke (the FR-37a end-to-end, via a throwaway in-package test)

```bash
cd /home/dustin/projects/stagehand

# Inline proof (delete after): a session_mode="" override on a pi-shape base disables multi-turn.
cat > internal/provider/zz_smoke_test.go <<'EOF'
package provider
import "testing"
func TestZZ_SessionModeMergeSmoke(t *testing.T) {
	base := Manifest{Name: "pi", Command: strPtr("pi"), ProviderFlag: strPtr("--provider"), SessionMode: strPtr("append")}
	merged := MergeManifest(base, Manifest{SessionMode: strPtr("")}) // user disables multi-turn
	if merged.SessionMode == nil || *merged.SessionMode != "" {
		t.Fatalf("session_mode=\"\" override lost: got %v (want *\"\")", merged.SessionMode)
	}
	inherit := MergeManifest(base, Manifest{}) // omitted ⇒ inherit
	if inherit.SessionMode == nil || *inherit.SessionMode != "append" {
		t.Fatalf("omitted key should inherit 'append': got %v", inherit.SessionMode)
	}
	t.Log("session_mode override merges correctly (\"\" disables; omit inherits) ✅")
}
EOF
go test -run TestZZ_SessionModeMergeSmoke -v ./internal/provider/ ; rm -f internal/provider/zz_smoke_test.go
# Expected: PASS. Without S2's clause the first check prints got 0x... / *"append" → FAIL.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green (incl. the extended merge tests).

### Feature Validation

- [ ] `merge.go` regime-1 has `if override.SessionMode != nil { out.SessionMode = override.SessionMode }` after the ProviderFlag clause.
- [ ] `sampleBase()` sets `SessionMode: strPtr("append")`.
- [ ] `TestMergeManifest_PartialOverride_OnlyTouchedFieldChanges` includes the `{"SessionMode", ...}` row.
- [ ] `TestMergeManifest_ExplicitZeroPointerWins` asserts `*merged.SessionMode == ""` for an explicit `""` override.
- [ ] An override `{SessionMode: strPtr("")}` on base `"append"` ⇒ `*merged.SessionMode == ""`.
- [ ] An empty override ⇒ `merged.SessionMode == base.SessionMode` (inherit).

### Scope Discipline Validation

- [ ] ONLY `internal/provider/{merge,merge_test}.go` modified (git diff --stat confirms).
- [ ] Did NOT edit `manifest.go` (S1 — landed), `render.go` (S3), `builtin.go`/`providers/*.toml` (S4), docs (S5).
- [ ] Did NOT conflate sampleBase's "append" with S4's shipped builtinPi value (sampleBase is a fixture).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] The clause is a literal copy of the regime-1 scalar rule (`!= nil`, not `len > 0`).
- [ ] Placement mirrors the struct's ProviderFlag→SessionMode adjacency (right after the ProviderFlag clause).
- [ ] The test edits reuse the existing idioms (sampleBase non-default values; the scalar table; the explicit-zero pattern).
- [ ] No doc-comment churn (the regime-1 description is generic).

---

## Anti-Patterns to Avoid

- ❌ Don't use `len(override.SessionMode) > 0` or a fresh-map merge — SessionMode is a regime-1 SCALAR
  (`*string`), not a slice or map. The clause is `if override.SessionMode != nil { out.SessionMode = override.SessionMode }`,
  identical to ProviderFlag/Output/Experimental. `len` won't compile on a `*string` anyway.
- ❌ Don't place the clause outside the regime-1 block, or far from ProviderFlag. The contract fixes it
  "right after the ProviderFlag clause" (mirrors the struct order; keeps scalar clauses grouped).
- ❌ Don't edit `manifest.go` — S1 is already landed (field + Resolve + Validate). The merge clause is the
  ONLY production gap. Re-editing manifest.go crosses the S1 boundary.
- ❌ Don't conflate `sampleBase()`'s `SessionMode: strPtr("append")` with the shipped `builtinPi` value.
  sampleBase is a TEST FIXTURE (the pi-shape used by every merge test); the shipped pi value is S4's job
  (builtin.go + providers/pi.toml). Do NOT edit builtin.go here.
- ❌ Don't skip the sampleBase edit — without it, base.SessionMode is nil and the PartialOverride check
  passes trivially (nil→nil), so the merge clause's "untouched field survives" path is never exercised.
  The non-nil "append" makes the coverage meaningful.
- ❌ Don't skip the ExplicitZeroPointerWins extension — that test is THE FR-37a contract (a user disabling
  multi-turn on pi). Without it, the clause could be a no-op and no test would catch it.
- ❌ Don't add a RenderMultiTurn method, a capability gate, or the pi builtin value — those are S3/S4.
  S2 is the merge clause + tests only.
- ❌ Don't update the MergeManifest doc comment — its regime-1 description is generic ("Scalar pointer
  fields ... override.Field != nil"), already covering SessionMode. (Only regime-2 enumerates fields.)
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a single-clause addition (one of ~15 identical regime-1 scalar clauses already in
MergeManifest) with the exact insertion point quoted (right after ProviderFlag, merge.go:56-58), the
verbatim clause, and three precise test edits reusing the file's existing idioms (sampleBase non-default
values; the PartialOverride scalar table; the ExplicitZeroPointerWins pattern). Three independent
de-riskings: (1) S1 is ALREADY landed (verified — manifest.go:66/121-123/177-178), so the field the clause
references exists and compiles; (2) SessionMode is `*string`, merging IDENTICALLY to the neighboring
ProviderFlag clause — no new pattern; (3) the sampleBase edit is verified safe for every existing merge
test (EmptyOverrideIsIdentity / DoesNotMutateInputs / MergedResultValidates all stay green). The ExplicitZero
extension is the load-bearing assertion — it FAILS without the clause (override silently dropped →
*"append") and PASSES with it (→ *""), so the test genuinely pins the FR-37a behavior. The only residual
uncertainty (not 10/10) is purely mechanical gofmt alignment, which the `gofmt -w` gate catches
immediately. S3/S4/S5 are cleanly fenced and untouched.
