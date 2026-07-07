---
name: "P1.M1.T1.S2 — Update MergeManifest for TooledFlags + Experimental"
description: |
  Extend `MergeManifest` in `internal/provider/merge.go` to field-merge the two v2 manifest fields S1
  added — `TooledFlags []string` (slice regime → `len(override) > 0` wholesale replace, same as
  `BareFlags`) and `Experimental *bool` (pointer regime → `override != nil` wins, same as
  `StripCodeFence`). Mirrors the existing regimes EXACTLY; no new semantics. Pure merge logic — does
  NOT Validate/Resolve, does NOT touch Render mode (T2), does NOT touch any builtin value (M2).
  Contract: a user override setting only `tooled_flags` preserves every other field from the built-in.
---

## Goal

**Feature Goal**: Make `MergeManifest` correctly field-merge the two manifest fields S1 introduced
(`TooledFlags`, `Experimental`) so that a user override (from `[provider.<name>]` in config) that sets
either field is no longer silently dropped — today the override is ignored and the built-in's value
(nil) wins. TooledFlags follows the slice regime (non-empty override replaces wholesale);
Experimental follows the pointer regime (non-nil override wins, explicit `*false` included).

**Deliverable** (ONE production file + its test file, both in `internal/provider/`):
1. `merge.go` — `MergeManifest` regime-1 block gains the `Experimental` (`!= nil`) entry; regime-2
   block gains the `TooledFlags` (`len > 0`) entry; the regime-2 doc-comment enumeration is updated to
   include `TooledFlags`. Six added lines + one doc token. Nothing else.
2. `merge_test.go` — `sampleBase()` gains non-nil/non-empty `TooledFlags` + `Experimental`; existing
   regime tests are extended to assert the two new fields; one new test for the TooledFlags wholesale
   replace (mirroring `TestMergeManifest_NonEmptySliceReplacesWholesale`).

**Success Definition**: After S2, `MergeManifest(base, Manifest{TooledFlags: []string{"--x"}})` yields a
result whose `TooledFlags == []string{"--x"}` with EVERY other field unchanged from `base`; and
`MergeManifest(base, Manifest{Experimental: boolPtr(false)})` against a base with `Experimental==*true`
yields `*false`. `go build/vet/gofmt` clean; `go test -race ./...` green; no edit outside
`internal/provider/merge.go` (+ test).

## User Persona

**Target User**: The Stagecoach contributor wiring v2 provider/decompose features (T2 Render-mode, M2 agy
+ tooled-flags, P3 multi-commit stager). This is internal merge plumbing — no end-user surface yet.

**Use Case**: When a user drops a `[provider.pi]` block in config setting only `tooled_flags` (to make pi
stager-capable) or `experimental` (to flag a docs-only agent), the registry merges that override onto the
pi built-in. Without S2 those overrides vanish; with S2 they take effect while every other built-in field
survives (PRD §16.1 field-by-field merge).

**Pain Points Addressed**: Closes the "my tooled_flags / experimental override is silently ignored" gap —
the only remaining un-merged manifest fields after S1.

## Why

- **PRD §16.1 mandates field-by-field manifest merge.** `MergeManifest` already honors this for every
  pre-S1 field across the three regimes. S1 added two fields; S2 is the mechanical completion of that
  contract for them. Leaving them un-merged is a latent correctness bug (override silently dropped).
- **Unblocks the v2 stager + agy path.** T2 (Render tooled mode) and M2 (agy `Experimental=true`;
  pi/claude `tooled_flags`) both rely on a merged manifest carrying the override's values. M2 sets these
  as BUILT-IN values (not overrides), but the user-override path must also work for community providers
  (§12.8) and per-provider config.
- **Zero new design.** Both fields map onto regimes with named precedents (TooledFlags≡BareFlags;
  Experimental≡StripCodeFence). The architecture delta (`manifest_v2_delta.md` §2) prescribes the exact
  merge lines verbatim. This is the lowest-risk change possible: apply two existing rules to two new fields.
- **No user-facing/docs surface** (contract: "internal merge logic, no user-facing/config/API surface
  change beyond what S1 documents").

## What

Two new override applications inside `MergeManifest`, each identical in shape to an existing entry:

```go
// regime 1 (scalar pointers) — placed after the StripCodeFence entry:
if override.Experimental != nil {
    out.Experimental = override.Experimental
}

// regime 2 (slices) — placed after the BareFlags entry:
if len(override.TooledFlags) > 0 {
    out.TooledFlags = override.TooledFlags
}
```

Plus the doc-comment enumeration update (line 13): `Slices (Subcommand, BareFlags)` →
`Slices (Subcommand, BareFlags, TooledFlags)` — the same consistency edit S1 made to the struct doc
comment. No change to `Name` handling, `Env` map merge, `Validate`, `Resolve`, or any builtin.

### Success Criteria

- [ ] `MergeManifest` applies `Experimental` via `if override.Experimental != nil` (pointer regime).
- [ ] `MergeManifest` applies `TooledFlags` via `if len(override.TooledFlags) > 0` (slice regime).
- [ ] The regime-2 doc-comment enumeration lists `TooledFlags` alongside `Subcommand, BareFlags`.
- [ ] `sampleBase()` sets non-empty `TooledFlags` and non-nil `Experimental` (so preserve/identity tests are meaningful).
- [ ] Tests prove: partial override preserves both new fields; TooledFlags wholesale-replaces; explicit
      `Experimental=false` overrides base `true`; nil/empty TooledFlags override preserves base; base.TooledFlags not mutated.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] No edit to `manifest.go`, `render.go`, `builtin.go`, `registry.go`, `parse.go`, or any docs/config/plan file.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the current `MergeManifest` body verbatim (the three regimes),
states the exact override signal per field type, gives the 3 surgical edits (with placement), provides
the full test changes (fixture + 4 extensions + 1 new test) reusing the existing helpers, and lists the
exact validation commands. The architecture delta §2 prescribes the merge lines; S1 is already landed
(verified), so the fields exist and the baseline is green.

### Documentation & References

```yaml
# MUST READ — the binding delta (prescribes the verbatim merge lines)
- docfile: plan/002_a17bb6c8dc1d/architecture/manifest_v2_delta.md
  why: "§2 (MergeManifest Updates) gives the exact two code blocks to add (TooledFlags len>0 in regime 2; Experimental !=nil in regime 1). §1 is S1 (done); §3 is T2 (out of scope)."
  critical: "§2 confirms TooledFlags=SLICE regime (len>0, like BareFlags) and Experimental=POINTER regime (!=nil, like StripCodeFence). Do NOT invent a new regime."

- docfile: plan/002_a17bb6c8dc1d/P1M1T1S2/research/s2_implementation_notes.md
  why: "Distilled S2 findings: baseline is GREEN; S1 already landed; merge.go has ZERO handling for the two fields (the gap); the exact 3-edit diff; the test taxonomy to mirror; the sampleBase() fixture change + why it is safe; aliasing rationale."
  critical: "The #1 implementation trap is using the WRONG override signal: TooledFlags MUST use `len(override.TooledFlags) > 0` (slice), Experimental MUST use `override.Experimental != nil` (pointer). Swapping them is the failure mode."

- file: internal/provider/merge.go
  why: "THE edit target. Contains MergeManifest with the three regimes fully documented in its doc comment. The override signal for each field is dictated by its TYPE, proven by the existing fields of the same type."
  pattern: "Pointer field → `if override.X != nil { out.X = override.X }`. Slice field → `if len(override.X) > 0 { out.X = override.X }`. Map (Env) → fresh-map key-by-key (UNCHANGED by S2)."
  gotcha: "Line 13 doc comment enumerates `Slices (Subcommand, BareFlags)` — MUST add TooledFlags for doc consistency (same edit S1 made to the struct doc comment). Do NOT touch the Env fresh-map block or the `out := base` shallow copy."

- file: internal/provider/merge_test.go
  why: "EDIT TARGET (tests). Same-package tests; reuses strPtr/boolPtr + sampleBase() + reflect.DeepEqual. The existing test taxonomy (one focused test per regime/behavior) is the template to EXTEND, not replace."
  pattern: "sampleBase() is the shared fully-populated manifest. Extend the partial-override / explicit-zero / empty-slice / no-mutation tests; ADD one TooledFlags wholesale-replace test mirroring TestMergeManifest_NonEmptySliceReplacesWholesale."
  gotcha: "sampleBase() currently sets NEITHER new field → add non-empty TooledFlags + non-nil Experimental=true so 'preserves base'/'identity' assertions are real (verified safe; no existing assertion breaks)."

- file: internal/provider/manifest.go
  why: "READ-ONLY ref (S1 already landed). Confirms field types/regimes: `TooledFlags []string`, `Experimental *bool`, and the strPtr/boolPtr helpers + Resolve's Experimental→boolPtr(false) default. Do NOT edit (S1 owns it)."
  pattern: "Experimental is *bool (pointer regime); TooledFlags is []string (slice regime). Resolve defaults Experimental to *false and leaves TooledFlags nil."

# Cross-references (read-only — do NOT edit in S2)
- file: internal/provider/builtin.go
  why: "Confirms builtins use NAMED literals and leave both new fields nil (agy/tooled_flags VALUES land in P1.M2, NOT here). Merge must handle the USER-OVERRIDE path regardless."
- file: internal/provider/registry.go
  why: "The merge caller: `merged := MergeManifest(builtin, override); merged.Validate(); merged.Resolve()`. S2 changes no signature; the caller is unaffected."
- file: internal/provider/render.go
  why: "Render's tooled-mode (mode==tooled → TooledFlags) and its 'tooled requires non-empty tooled_flags' error are T2 (P1.M1.T2.S1) — NOT a merge concern. S2 only ensures the value reaches the merged manifest."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/provider/
    ├── manifest.go        # READ-ONLY (S1 landed): has TooledFlags []string + Experimental *bool
    ├── merge.go           # EDIT TARGET (regime 1 +Experimental; regime 2 +TooledFlags; doc line 13)
    ├── merge_test.go      # EDIT TARGET (sampleBase +2 fields; 4 test extensions; 1 new test)
    ├── builtin.go         # read-only ref — named literals; new fields nil (agy/tooled_flags = M2)
    ├── render.go          # read-only ref — RenderMode/tooled-mode = T2 (NOT this subtask)
    ├── registry.go        # read-only ref — MergeManifest caller; no signature change
    └── parse.go, executor.go  # read-only ref — unaffected
```

### Desired Codebase Tree After S2

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/provider/merge.go        # +Experimental (regime 1), +TooledFlags (regime 2), +doc token
    internal/provider/merge_test.go   # sampleBase +2 fields; 4 extensions; 1 new test
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/provider/merge.go` | MODIFY | Apply the two existing merge regimes to the two new fields; update the regime-2 doc enumeration. **Only production file touched.** |
| `internal/provider/merge_test.go` | MODIFY | Extend `sampleBase()` + the regime tests to cover both fields; add TooledFlags wholesale-replace test. |

**Explicitly NOT touched**: `manifest.go` (S1), `render.go` (T2), `builtin.go`/`providers/*.toml`
(agy + tooled-flags values = P1.M2), `registry.go` (unaffected caller), `parse.go`, `executor.go`, any
`docs/*.md` (contract: no docs change), `PRD.md`, `tasks.json`, `prd_snapshot.md`, anything under `plan/`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL — match the override signal to the field TYPE, not to the field NAME.
//   TooledFlags is []string  → SLICE regime   → `if len(override.TooledFlags) > 0 { out.TooledFlags = override.TooledFlags }`
//   Experimental is *bool    → POINTER regime → `if override.Experimental != nil { out.Experimental = override.Experimental }`
// Getting this backwards (len on a *bool, or != nil on a slice) is the single failure mode. The
// existing fields prove each rule: BareFlags/Subcommand use len>0; StripCodeFence/Output/etc. use != nil.

// CRITICAL — go-toml/v2 has no omitempty (FINDING 5). This is WHY Experimental is *bool: a user override
// setting `experimental = false` must decode to a NON-NIL *false and OVERRIDE the built-in's value on
// merge. The merge's `!= nil` check is what honors that explicit false. A plain bool could not do this.

// CRITICAL — an EMPTY/nil TooledFlags override must PRESERVE base (the slice "absent" sentinel).
// `len(override.TooledFlags) > 0` correctly treats both nil and `[]string{}` as "not overridden".
// Do NOT use `if override.TooledFlags != nil` (that would replace base with an empty slice on a `[]` override).

// GOTCHA — `out := base` is a SHALLOW copy: slice headers and pointers are shared with base until reassigned.
// Regime 2 REASSIGNS the header (`out.X = override.X`) — it never writes the backing array — so base.X is
// never mutated. (Same reason BareFlags/Subcommand are safe; no fresh allocation needed for slices.)
// ONLY the Env map (regime 3) needs a fresh allocation; S2 adds no map handling, so that invariant is untouched.

// GOTCHA — update the doc-comment enumeration. Line 13 says `Slices (Subcommand, BareFlags)`; after adding
// TooledFlags (a slice) it MUST read `Slices (Subcommand, BareFlags, TooledFlags)` or the doc is internally
// inconsistent — the exact consistency edit S1 made to the struct's own doc comment.

// GOTCHA — MergeManifest does NOT Validate or Resolve (its doc comment says so). Experimental may be nil
// at merge time (builtin unset + override unset) → Resolve defaults it to *false afterward, downstream.
// Leave it nil in that case; only APPLY a non-nil override. Same for TooledFlags (stays nil → Render's
// tooled-requires-flags check in T2 handles it; NOT a merge concern).
```

## Implementation Blueprint

### Data models and structure

No new types, no new helpers. S2 reuses `strPtr`/`boolPtr` (manifest.go) and the existing `Manifest`
struct unchanged. The only "model" fact is the type→regime mapping (above). The relevant existing merge
skeleton (verbatim, current):

```go
func MergeManifest(base, override Manifest) Manifest {
	out := base
	// --- regime 1: scalar pointer fields — non-nil override WINS (explicit "" / false included) ---
	// ... Detect ... StripCodeFence ... RetryInstruction ...   ← ADD Experimental here (after StripCodeFence)
	// --- regime 2: slices — non-empty override REPLACES wholesale (no element merge) ---
	// ... Subcommand, BareFlags ...                            ← ADD TooledFlags here (after BareFlags)
	// --- regime 3: Env map — key-by-key merge into a FRESH map (UNCHANGED) ---
	// Name: NOT merged — out.Name == base.Name.
	return out
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT merge.go regime 1 — apply Experimental (pointer regime)
  - LOCATE: the regime-1 block, the `StripCodeFence` entry:
        if override.StripCodeFence != nil {
            out.StripCodeFence = override.StripCodeFence
        }
  - INSERT immediately AFTER it (groups the two *bool fields):
        if override.Experimental != nil {
            out.Experimental = override.Experimental
        }
  - SIGNAL: `!= nil` (Experimental is *bool → POINTER regime; explicit *false is a real override).
  - DO NOT: use `len()` (that is the slice signal); do NOT touch Resolve/Validate.

Task 2: EDIT merge.go regime 2 — apply TooledFlags (slice regime)
  - LOCATE: the regime-2 block, the `BareFlags` entry:
        if len(override.BareFlags) > 0 {
            out.BareFlags = override.BareFlags
        }
  - INSERT immediately AFTER it (groups the two flag slices):
        if len(override.TooledFlags) > 0 {
            out.TooledFlags = override.TooledFlags
        }
  - SIGNAL: `len(override.X) > 0` (TooledFlags is []string → SLICE regime; nil AND [] both preserve base).
  - DO NOT: use `!= nil` (that would replace base with [] on an empty-slice override — WRONG).

Task 3: EDIT merge.go doc comment — regime-2 enumeration (line 13)
  - LOCATE: `//  2. Slices (Subcommand, BareFlags): len(override.Slice) > 0 → result REPLACES base's slice`
  - CHANGE to: `//  2. Slices (Subcommand, BareFlags, TooledFlags): len(override.Slice) > 0 → result REPLACES base's slice`
  - WHY: doc consistency (mirror the edit S1 made to the struct doc comment). The ONLY doc change.

Task 4: EDIT merge_test.go sampleBase() — add the two fields (meaningful, non-zero values)
  - LOCATE: the `sampleBase()` func (top of merge_test.go).
  - ADD to the returned Manifest literal (alongside BareFlags/Env):
        TooledFlags:  []string{"--allowed-tools", "git:*", "--approval-mode", "auto"},
        Experimental: boolPtr(true),
  - WHY: non-empty TooledFlags + non-nil Experimental=true make the "preserves base" / "identity" /
        "explicit false wins" assertions MEANINGFUL (nil would pass them trivially).
  - SAFETY (verified): adding two non-zero fields breaks no existing assertion — every existing test
        either asserts "merged==base" (still true) or a specific override (unaffected).

Task 5: EXTEND merge_test.go existing tests to cover both new fields
  - TestMergeManifest_PartialOverride_OnlyTouchedFieldChanges — after the BareFlags/Subcommand/Env checks, ADD:
        if !reflect.DeepEqual(merged.TooledFlags, base.TooledFlags) {
            t.Errorf("TooledFlags = %v, want %v", merged.TooledFlags, base.TooledFlags)
        }
        if merged.Experimental == nil || base.Experimental == nil || *merged.Experimental != *base.Experimental {
            t.Errorf("Experimental = %v, want %v", merged.Experimental, base.Experimental)
        }
  - TestMergeManifest_ExplicitZeroPointerWins — add Experimental to the override + an assertion:
        merged := MergeManifest(base, Manifest{
            StripCodeFence: boolPtr(false),
            PrintFlag:      strPtr(""),
            Experimental:   boolPtr(false), // base has true → explicit false must win
        })
        ... existing StripCodeFence/PrintFlag assertions ...
        if merged.Experimental == nil || *merged.Experimental != false {
            t.Errorf("explicit experimental=false lost (got %v)", merged.Experimental)
        }
  - TestMergeManifest_EmptyOrNilSlicePreservesBase — extend both sub-cases to TooledFlags:
        // (a) nil override
        if !reflect.DeepEqual(mergedNil.TooledFlags, base.TooledFlags) {
            t.Errorf("nil override: TooledFlags = %v, want %v", mergedNil.TooledFlags, base.TooledFlags)
        }
        // (b) non-nil empty slice — add TooledFlags: []string{} to the override, then:
        if !reflect.DeepEqual(mergedEmpty.TooledFlags, base.TooledFlags) {
            t.Errorf("empty override: TooledFlags = %v, want %v", mergedEmpty.TooledFlags, base.TooledFlags)
        }
  - TestMergeManifest_DoesNotMutateInputs — snapshot + assert base.TooledFlags unmutated:
        tooledBefore := append([]string(nil), base.TooledFlags...)
        ... existing MergeManifest call ...
        if !reflect.DeepEqual(base.TooledFlags, tooledBefore) {
            t.Errorf("MergeManifest mutated base.TooledFlags: got %v, want %v", base.TooledFlags, tooledBefore)
        }

Task 6: ADD one new test — TestMergeManifest_TooledFlagsReplacedWholesale
  - PLACE: alongside TestMergeManifest_NonEmptySliceReplacesWholesale (mirror its shape).
  - BODY:
        func TestMergeManifest_TooledFlagsReplacedWholesale(t *testing.T) {
            base := sampleBase()
            override := Manifest{TooledFlags: []string{"--yolo"}}
            merged := MergeManifest(base, override)
            if !reflect.DeepEqual(merged.TooledFlags, []string{"--yolo"}) {
                t.Errorf("TooledFlags = %v, want [\"--yolo\"] (wholesale replace)", merged.TooledFlags)
            }
            // The OTHER flag slice must be untouched.
            if !reflect.DeepEqual(merged.BareFlags, base.BareFlags) {
                t.Errorf("BareFlags = %v, want %v (untouched)", merged.BareFlags, base.BareFlags)
            }
        }

Task 7: VALIDATE (run all gates; all must pass before declaring done)
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: gofmt -l .            # must be empty
  - RUN: go test -race ./...   # provider suite green; full repo green (no other package touched)
  - RUN targeted: go test -race ./internal/provider/ -run 'TestMergeManifest'
  - FIX-FORWARD: read failures, fix, re-run. Do NOT skip.
```

### Implementation Patterns & Key Details

```go
// === The two edits in context (merge.go, AFTER S2) ===

	// --- regime 1: scalar pointer fields — non-nil override WINS (explicit "" / false included) ---
	// ... [Detect ... DefaultProvider ... Output ... JsonField] ...
	if override.StripCodeFence != nil {
		out.StripCodeFence = override.StripCodeFence
	}
	if override.Experimental != nil {              // ← NEW (S2): pointer regime, same as StripCodeFence
		out.Experimental = override.Experimental
	}
	if override.RetryInstruction != nil {
		out.RetryInstruction = override.RetryInstruction
	}

	// --- regime 2: slices — non-empty override REPLACES wholesale (no element merge) ---
	if len(override.Subcommand) > 0 {
		out.Subcommand = override.Subcommand
	}
	if len(override.BareFlags) > 0 {
		out.BareFlags = override.BareFlags
	}
	if len(override.TooledFlags) > 0 {              // ← NEW (S2): slice regime, same as BareFlags
		out.TooledFlags = override.TooledFlags
	}
```

```go
// === sampleBase() after S2 (the two added lines) ===
func sampleBase() Manifest {
	return Manifest{
		// ... existing fields ...
		BareFlags:      []string{"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session"},
		TooledFlags:    []string{"--allowed-tools", "git:*", "--approval-mode", "auto"}, // NEW (S2)
		Experimental:   boolPtr(true),                                                   // NEW (S2)
		Env:            map[string]string{"A": "1", "B": "2"},
	}
}
```

### Integration Points

```yaml
MERGEMANIFEST (internal/provider/merge.go):
  - regime 1 +entry: "if override.Experimental != nil { out.Experimental = override.Experimental }"
  - regime 2 +entry: "if len(override.TooledFlags) > 0 { out.TooledFlags = override.TooledFlags }"
  - doc comment: regime-2 enumeration adds "TooledFlags"

NO-TOUCH (explicitly — owned by other subtasks):
  - internal/provider/manifest.go   # S1 (DONE) — struct + Resolve; do not re-edit
  - internal/provider/render.go     # RenderMode (bare/tooled) = P1.M1.T2.S1; tooled-requires-flags error is RENDER-time, not merge
  - internal/provider/builtin.go    # agy + tooled_flags/experimental VALUES = P1.M2.T1/T2
  - internal/provider/registry.go   # calls MergeManifest; no signature/behavior change needed
  - internal/provider/parse.go, executor.go   # read other fields; unaffected
  - docs/*.md                       # contract: internal merge logic, no docs change

DOWNSTREAM HOOKS (informational — implemented by LATER subtasks, NOT S2):
  - T2  (P1.M1.T2.S1): Render reads merged.TooledFlags in tooled mode; errors if empty
  - M2  (P1.M2):       agy sets Experimental=true as a BUILTIN; pi/claude set tooled_flags as BUILTINS
  - registry:          runs MergeManifest → Validate → Resolve (S2 is the merge step only)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l .                      # Expected: empty (run `gofmt -w internal/provider/merge.go merge_test.go` if listed)
go vet ./internal/provider/...  # Expected: exit 0
go build ./...                  # Expected: exit 0 (no signature change; callers unaffected)

# Expected: Zero output/errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

# Targeted: all merge tests (the changed + new)
go test -race ./internal/provider/ -v -run 'TestMergeManifest'

# Full provider suite
go test -race ./internal/provider/ -v

# Expected: ALL merge tests PASS — partial override preserves TooledFlags+Experimental; TooledFlags
# wholesale-replaces; explicit Experimental=false overrides base true; nil/empty TooledFlags preserves base;
# base.TooledFlags not mutated; empty-override is identity.
```

### Level 3: Whole-Repository Regression (No Behavior Change Elsewhere)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...             # Expected: ALL packages pass (no other package edited)
go vet ./...                    # Expected: exit 0

# Confirm ONLY internal/provider/merge.go (+ test) changed in production source
git diff --stat -- internal/ pkg/ cmd/ providers/
# Expected: only internal/provider/merge.go + internal/provider/merge_test.go appear.

# Confirm manifest.go was NOT re-edited by S2 (S1 owns it)
git diff --stat -- internal/provider/manifest.go || echo "OK: manifest.go untouched by S2"
```

### Level 4: Merge-Behavior Sanity (prove the override is no longer dropped)

```bash
cd /home/dustin/projects/stagecoach

# Inline behavioral cross-check: override setting ONLY tooled_flags preserves every other field,
# and the override value actually lands on the merged manifest. (The extended tests cover this; this
# is a manual cross-check via a throwaway main.)
cat > /tmp/sh_merge_test.go <<'EOF'
package main
import ("fmt"; "reflect"; "github.com/dustin/stagecoach/internal/provider")
func boolPtr(b bool) *bool { return &b }
func main() {
  base := provider.Manifest{}        // stand-in built-in (zero); resolve not needed for merge
  // Give base a Command so a downstream Validate would pass (not required for the merge assertion).
  override := provider.Manifest{TooledFlags: []string{"--allowed-tools", "git:*"}, Experimental: boolPtr(true)}
  merged := provider.MergeManifest(base, override)
  fmt.Printf("TooledFlags=%v Experimental=%v\n", merged.TooledFlags, merged.Experimental)
  wantT := []string{"--allowed-tools", "git:*"}
  if !reflect.DeepEqual(merged.TooledFlags, wantT) || merged.Experimental == nil || *merged.Experimental != true {
    fmt.Println("FAIL: override dropped"); return
  }
  fmt.Println("PASS: override applied")
}
EOF
go run /tmp/sh_merge_test.go && rm -f /tmp/sh_merge_test.go
# Expected: TooledFlags=[--allowed-tools git:*] Experimental=0xc... ; PASS: override applied
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages pass (provider merge tests extended + green; no other package touched).

### Feature Validation

- [ ] `MergeManifest` applies `Experimental` via `if override.Experimental != nil` (pointer regime).
- [ ] `MergeManifest` applies `TooledFlags` via `if len(override.TooledFlags) > 0` (slice regime).
- [ ] The regime-2 doc-comment enumeration includes `TooledFlags`.
- [ ] A partial override (only `DefaultModel` set) leaves `TooledFlags` AND `Experimental` equal to base.
- [ ] An explicit `Experimental: boolPtr(false)` overrides a base `*true`.
- [ ] A non-empty `TooledFlags` override replaces wholesale; `BareFlags` is untouched.
- [ ] nil and empty (`[]string{}`) `TooledFlags` overrides preserve base.

### Scope Discipline Validation

- [ ] ONLY `internal/provider/merge.go` (+ `merge_test.go`) modified (`git diff --stat` confirms).
- [ ] Did NOT edit `manifest.go` (S1), `render.go` (T2), `builtin.go`/`providers/*.toml` (P1.M2).
- [ ] Did NOT edit `registry.go` (the merge caller — no signature change).
- [ ] Did NOT add a `Validate`/`Resolve` rule (merge does neither).
- [ ] Did NOT edit any `docs/*.md` (contract: no docs change).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Override signal matches field TYPE (TooledFlags=len>0 slice; Experimental=!=nil pointer) — not swapped.
- [ ] Placement mirrors the matching precedent (TooledFlags after BareFlags; Experimental after StripCodeFence).
- [ ] Test extensions reuse `strPtr`/`boolPtr`/`sampleBase()`/`reflect.DeepEqual`; one new focused test.
- [ ] `sampleBase()` now sets non-nil/non-empty values for both new fields (meaningful preserve/identity assertions).

---

## Anti-Patterns to Avoid

- ❌ Don't swap the override signals — TooledFlags is a SLICE (`len(override) > 0`); Experimental is a
  POINTER (`override != nil`). Using `!= nil` on TooledFlags would replace base with `[]string{}` on an
  empty-slice override; using `len()` on Experimental won't compile (`*bool` has no `len`).
- ❌ Don't add a NEW merge regime — both fields map onto EXISTING regimes with named precedents
  (TooledFlags≡BareFlags; Experimental≡StripCodeFence). S2 applies two existing rules to two new fields.
- ❌ Don't touch the Env fresh-map block (regime 3) or the `out := base` shallow copy — S2 adds no map
  handling and no new aliasing surface; slices are safe by header reassignment.
- ❌ Don't Validate or Resolve inside MergeManifest (its doc comment forbids it; the registry does that
  after). Experimental may legitimately be nil at merge time (Resolve defaults it later).
- ❌ Don't forget the doc-comment enumeration update (line 13 `Slices (...)`) — leaving it listing only
  `Subcommand, BareFlags` makes the doc inconsistent (the exact edit S1 made to the struct comment).
- ❌ Don't leave `sampleBase()` without the two new fields — nil values make "preserves base"/"identity"
  assertions pass trivially without exercising the new merge code paths.
- ❌ Don't edit `manifest.go` (S1 owns it), `render.go` (T2 owns tooled-mode rendering + the
  "tooled requires non-empty tooled_flags" error), or `builtin.go` (M2 owns the agy/tooled-flag VALUES).
- ❌ Don't add a `TestMergeManifest_Experimental*` test that duplicates the
  `TestMergeManifest_ExplicitZeroPointerWins` extension — fold Experimental into that existing
  pointer-regime-payoff test instead (one focused test per behavior, matching the file's taxonomy).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is the smallest possible change that completes a documented contract — apply two
EXISTING merge regimes (each with a named precedent on the same struct) to two new fields, exactly as
prescribed verbatim by the architecture delta (§2). The override signal per field is dictated by its TYPE
(not a judgment call), and S1 already landed the fields (verified: baseline `go test ./internal/provider/`
is green; `manifest.go` has both fields + Resolve default). The #1 failure mode — swapping the slice vs
pointer override signals — is front-loaded as a CRITICAL gotcha with the compile-time safety net that
`len(*bool)` won't even compile. The test plan reuses the existing taxonomy and `sampleBase()` fixture
(verified safe to extend), with the `sampleBase()` non-nil-values requirement called out so the new merge
paths are actually exercised. The only residual uncertainty (not 10/10) is doc-comment wording precision
on line 13 and the exact ordering of the two insertion points — both cosmetic, not load-bearing, and both
gated by the deterministic `go test -race ./...` + `gofmt -l .` validation. The Render/builtin/agy
boundaries are cleanly fenced to T2/M2 and cannot be broken by S2 (S2 only changes how two already-existing
fields propagate through merge; it adds no field, no signature, no builtin value).
