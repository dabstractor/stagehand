---
name: "P1.M1.T1.S1 — Add TooledFlags + Experimental to Manifest struct; update Resolve + Validate"
description: |
  v2 manifest-schema foundation. Add two additive fields to `internal/provider/manifest.go`'s `Manifest`
  struct: `TooledFlags []string` (toml `tooled_flags`, slice regime — same as BareFlags) for the v2
  stager role's tooled mode (PRD §11.5/§12.1), and `Experimental *bool` (toml `experimental`,
  pointer-scalar regime — same as StripCodeFence) for docs-only providers (§12.7.2/§12.5.1). In Resolve(),
  default Experimental to `boolPtr(false)`; leave TooledFlags as-is (nil stays nil). No new Validate
  rules. Update inline struct doc comments. Existing code compiles unchanged (fields are nil/zero by
  default; builtins use named literals). Merge (S2), Render mode (T2), and agy/tooled-flag values (M2)
  are OUT OF SCOPE.
---

## Goal

**Feature Goal**: Extend the provider `Manifest` struct with the two v2 fields the rest of the
v2 provider/decompose work depends on — `TooledFlags` (the stager role's tool-enabled flag-set,
the tooled-mode analog of `bare_flags`) and `Experimental` (a quality/provenance marker for
docs-only providers) — using the struct's existing two field regimes, with correct Resolve() defaults
and inline documentation, so downstream subtasks (MergeManifest S2, Render-mode T2, agy M2) have a
stable schema to build on.

**Deliverable** (ONE production file, `internal/provider/manifest.go`, + its test file):
1. `Manifest` struct gains `TooledFlags []string` (`toml:"tooled_flags"`) and `Experimental *bool`
   (`toml:"experimental"`), with inline doc comments citing §11.5/§12.1 and §12.7.2/§12.5.1.
2. `Resolve()` adds `if out.Experimental == nil { out.Experimental = boolPtr(false) }`; `TooledFlags`
   is left as-is (nil stays nil), and the "left as-is" comment is extended to name it.
3. `Validate()` is UNCHANGED (no new rules).
4. The struct-level doc comment's slice enumeration ("Subcommand, BareFlags") is extended to include
   `TooledFlags`.
5. `manifest_test.go` extends the existing Resolve/unmarshal tests to prove the two new contracts.

**Success Definition**: `Manifest` carries the two new fields under the correct regimes; `Resolve()`
guarantees a non-nil `*Experimental == false` (default) while preserving an explicit `true`, and leaves
`TooledFlags` nil when absent; `Validate()` is unchanged; `go build/vet/gofmt` clean and
`go test -race ./...` green with no edits required to any other package (builtins, merge, render, parse
all compile unchanged).

## User Persona

**Target User**: The Stagehand contributor implementing the v2 provider/decompose subtasks that
immediately follow (S2 MergeManifest, T2 Render-mode, M2 agy + tooled-flags, P3 multi-commit
decomposition). This is a foundation/schema subtask — no end-user-visible behavior yet.

**Use Case**: Every downstream v2 subtask references `Manifest.TooledFlags` (stager rendering,
`providers show`) and `Manifest.Experimental` (experimental-provider marking in `providers list`,
agy). This subtask lands the field definitions + defaults so those subtasks can compile against them.

**Pain Points Addressed**: Removes the "where does the stager's flag-set live / how is a docs-only
provider marked" schema gap that would otherwise block the v2 stager pipeline and agy provider.

## Why

- **v2 multi-commit needs a tooled stager (PRD §11.5, §12.1, §13.6.2).** The stager is the one role
  that runs with tools ON (git-scoped). `bare_flags` is the bare-mode (tools-off) flag-set; the
  symmetric `tooled_flags` is required so the stager can express "tooled but safe" per provider. The
  PRD §12.1 manifest schema already specifies `tooled_flags`; this subtask adds the Go field.
- **Docs-only providers need an explicit provenance marker (§12.7.2).** A provider added from docs /
  issue-tracker research rather than a verified `--help` (e.g. `agy`, §12.5.1) must be marked
  `experimental = true` so `providers list` can flag it and users are warned. The `experimental`
  field is the schema home for that.
- **Additive, zero-blast-radius schema change.** Both fields are optional and nil/zero by default.
  Existing builtins use NAMED struct literals, so they compile unchanged. The pointer-scalar regime
  (Experimental) and slice regime (TooledFlags) are both already established on the struct, so the
  new fields follow proven patterns — no new design calls.
- **Unblocks the critical path.** This is the first subtask (P1.M1.T1.S1) of the v2 plan; MergeManifest
  (S2) and Render-mode (T2) depend on these fields existing.

## What

A purely additive change to the `Manifest` struct in `internal/provider/manifest.go`, plus a
default-filling line in `Resolve()`, plus inline doc-comment updates, plus natural test extensions.
No behavior change for any existing provider (the six builtins leave both fields nil; Resolve defaults
Experimental to false; TooledFlags stays nil).

### Success Criteria

- [ ] `Manifest` has `TooledFlags []string` with `toml:"tooled_flags"` and a §11.5/§12.1 doc comment.
- [ ] `Manifest` has `Experimental *bool` with `toml:"experimental"` and a §12.7.2/§12.5.1 doc comment.
- [ ] `Resolve()` sets `out.Experimental = boolPtr(false)` when nil; preserves an explicit non-nil value.
- [ ] `Resolve()` leaves `TooledFlags` as-is (nil stays nil); the "left as-is" comment names TooledFlags.
- [ ] `Validate()` is unchanged (no new rules for either field).
- [ ] The struct doc comment's slice enumeration includes TooledFlags.
- [ ] `manifest_test.go` asserts: TooledFlags stays nil through Resolve; Experimental defaults to
      non-nil `*false`; an explicit `Experimental: boolPtr(true)` survives Resolve.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] No edits to `merge.go`, `render.go`, `builtin.go`, `parse.go`, `executor.go`, or any docs/*.md.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact current `Manifest` struct, `Resolve()`, `Validate()`,
and the `strPtr`/`boolPtr` helpers; states the two field regimes (slice vs pointer-scalar) with the
existing fields each mirrors; gives the verbatim doc-comment text and placement from the architecture
delta; and lists the exact test extensions (with helper reuse). The architecture delta
(`manifest_v2_delta.md` §1) pre-resolved placement, regimes, and Resolve/Validate behavior.

### Documentation & References

```yaml
# MUST READ — the binding v2 manifest delta (do not re-litigate)
- docfile: plan/002_a17bb6c8dc1d/architecture/manifest_v2_delta.md
  why: "§1 gives the verbatim new-field declarations + doc comments, the exact Resolve() line (Experimental→boolPtr(false); TooledFlags left as-is), and confirms NO new Validate() rules. §2 (MergeManifest) and §3 (Render) are owned by S2/T2 — OUT OF SCOPE here, but §1 is fully S1's scope."
  critical: "§1 confirms TooledFlags follows the SLICE regime (same as BareFlags) and Experimental follows the POINTER-SCALAR regime (same as StripCodeFence). §4 (agy) is P1.M2 — do NOT add agy in S1. §5/§6 (priority reordering, pi default_model) are also P1.M2 — out of scope."

- file: internal/provider/manifest.go
  why: "THE edit target. Contains Manifest struct (two regimes documented in its doc comment), Resolve() (default-filling), Validate() (enum/required checks), and strPtr/boolPtr helpers."
  pattern: "Pointer scalars default in Resolve via `if out.X == nil { out.X = <ptr> }`; slices are left nil with a trailing comment. Experimental mirrors StripCodeFence (*bool); TooledFlags mirrors BareFlags ([]string)."
  gotcha: "Resolve's trailing comment `// Subcommand / BareFlags / Env: left as-is (nil stays nil).` MUST be extended to name TooledFlags. The struct doc comment's `Slices (Subcommand, BareFlags)` enumeration MUST be extended to include TooledFlags. Validate is UNCHANGED."

- file: internal/provider/manifest_test.go
  why: "EDIT TARGET (tests). Same-package tests use the strPtr/boolPtr helpers + named Manifest{} literals. Existing regime tests to EXTEND: TestResolve_SlicesLeftNil (:398), TestResolve_AppliesDefaultsToNilOptionals (:343), TestResolve_PreservesExplicitValues (:355)."
  pattern: "Assertion idiom: `r := m.Resolve(); if r.X != nil { ... }` (slice) / `if r.X == nil || *r.X != <want> { ... }` (pointer). No new test file; co-locate in manifest_test.go."

# Cross-references (read-only — do NOT edit in S1)
- file: internal/provider/builtin.go
  why: "Confirms every builtinXxx() uses NAMED struct literals (Manifest{Name:..., BareFlags:...}), so adding fields compiles cleanly with zero edits. Do NOT add agy or tooled_flags here (P1.M2)."
  gotcha: "If a builtin later needs TooledFlags/Experimental it is P1.M2.T1/T2 — S1 leaves all builtins nil."

- file: internal/provider/merge.go
  why: "MergeManifest handles the two new fields in S2 (P1.M1.T1.S2): TooledFlags via `if len(override.TooledFlags) > 0` (slice regime), Experimental via `if override.Experimental != nil` (pointer regime). NOT edited in S1."

- file: internal/provider/render.go
  why: "Render gains a variadic RenderMode param in T2 (P1.M1.T2.S1) to select bare vs tooled flags. NOT edited in S1 — existing Render uses BareFlags only and still compiles (TooledFlags simply unused for now)."

- docfile: plan/002_a17bb6c8dc1d/P1M1T1S1/research/s1_implementation_notes.md
  why: "Distilled S1 findings: the two regimes, exact placement + verbatim comments, the single Resolve line, the test extensions (with helper reuse), and the scope boundary vs S2/T2/M2."
```

### Current Codebase Tree (relevant slice)

```bash
stagehand/
└── internal/provider/
    ├── manifest.go        # EDIT TARGET (struct +2 fields; Resolve +1 line; doc comments)
    ├── manifest_test.go   # EDIT TARGET (extend 3 existing Resolve/unmarshal tests)
    ├── builtin.go         # read-only ref — named literals; UNCHANGED (agy/tooled-flags = P1.M2)
    ├── merge.go           # read-only ref — MergeManifest = S2 (NOT this subtask)
    ├── render.go          # read-only ref — RenderMode = T2 (NOT this subtask)
    ├── parse.go           # read-only ref — reads Output/StripCodeFence only
    └── executor.go        # read-only ref — unaffected
```

### Desired Codebase Tree After S1

```bash
stagehand/
└── (only existing files modified — no new files)
    internal/provider/manifest.go        # +2 fields, +1 Resolve line, +doc-comment updates
    internal/provider/manifest_test.go   # extend TestResolve_SlicesLeftNil / _AppliesDefaultsToNilOptionals / _PreservesExplicitValues
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/provider/manifest.go` | MODIFY | Add TooledFlags + Experimental fields (correct regimes) + doc comments; Resolve() default for Experimental; extend slice-regime comment. **Only production file touched.** |
| `internal/provider/manifest_test.go` | MODIFY | Extend existing Resolve tests to prove TooledFlags-nil-kept + Experimental-default-false + Experimental-explicit-preserved. |

**Explicitly NOT touched**: `merge.go` (S2), `render.go` (T2), `builtin.go`/`providers/*.toml`
(agy + tooled-flag values = P1.M2), `parse.go`, `executor.go`, any `docs/*.md` (contract: inline struct
comments only — no user-facing docs yet), `PRD.md`, `tasks.json`, `prd_snapshot.md`, anything under `plan/`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL — two field regimes, do not mix them. The Manifest doc comment pins this:
//   * POINTER SCALARS (*string/*bool): nil = absent (inherit/default), non-nil = explicit (even *false).
//     Experimental *bool FOLLOWS THIS (same as StripCodeFence). Default via Resolve: boolPtr(false).
//   * SLICES ([]string): nil = natural "absent" sentinel (absent → nil; present → non-nil even empty).
//     TooledFlags []string FOLLOWS THIS (same as BareFlags/Subcommand). Resolve does NOT touch it.
// Do NOT make Experimental a plain bool (loses the absent/explicit distinction the merge relies on in
// S2) and do NOT make TooledFlags a *[]string (no absent/explicit semantics needed; pointers on slices
// add dereference noise with no benefit — the struct doc comment says so explicitly).

// CRITICAL — go-toml/v2 has no omitempty (FINDING 5). The pointer-scalar design exists precisely so a
// user override's PRESENT-but-false Experimental decodes non-nil and overrides the built-in on merge.
// Plain bool could not distinguish "absent" from "false" — that is why Experimental is *bool.

// GOTCHA — Resolve's trailing comment and the struct doc comment both ENUMERATE the slice fields by name
// ("Subcommand, BareFlags"). After adding TooledFlags (a slice), BOTH comments must list it, or the
// documentation will be internally inconsistent and mislead the S2 merge implementer.

// GOTCHA — Validate is UNCHANGED. TooledFlags has no enum/required semantics at this layer (Render-time
// tooled-requires-flags check is T2, not here). Experimental is a free *bool (nil allowed; Resolve
// defaults it). Do NOT add a Validate rule for either field.

// GOTCHA — builtins use NAMED struct literals, so adding fields needs zero builtin edits. Do NOT add
// agy or set tooled_flags on pi/claude here — those are P1.M2. Leave all builtins' new fields nil.

// GOTCHA — Resolve returns `out := m` (a shallow copy: headers/pointers/slices/map). Setting
// out.Experimental = boolPtr(false) reassigns only the copy's pointer header — it does NOT mutate the
// caller's m.Experimental. This is the existing, correct behavior (same as every other Resolve default).
```

## Implementation Blueprint

### Data models and structure

No new types. The change extends the existing `Manifest` struct with two fields and adds one default
line to `Resolve()`. The relevant existing helpers/types (verbatim):

```go
// internal/provider/manifest.go (EXISTING — reused, unchanged)
func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

// Regime precedents already on the struct:
BareFlags      []string `toml:"bare_flags"`      // SLICE regime — TooledFlags mirrors this
StripCodeFence *bool   `toml:"strip_code_fence"` // POINTER regime — Experimental mirrors this
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD the two fields to the Manifest struct (internal/provider/manifest.go)
  - LOCATE: the `// --- bare mode (§12.1) ---` block ending with `BareFlags []string`.
  - INSERT immediately AFTER BareFlags (and before the `// --- output (§12.1) ---` block):
        // --- tooled mode (v2; §11.5, §12.1) ---
        // Flags for the STAGER role (tools on, git-scoped, non-interactive). nil/empty => this
        // provider does not support tooled mode and cannot serve as a stager. Used in place of
        // BareFlags when mode=="tooled" in Render.
        TooledFlags []string `toml:"tooled_flags"`

        // --- experimental (§12.7.2, §12.5.1) ---
        // true => provider ships from docs/issue-tracker research, not a verified --help.
        // `providers list` marks experimental providers distinctly.
        Experimental *bool `toml:"experimental"`
  - NAMING: `TooledFlags` / `Experimental` (exported, Go-camelCase; matches BareFlags/StripCodeFence).
  - TOML TAGS: exactly `tooled_flags` / `experimental` (snake_case, matching §12.1 + the delta).
  - REGIMES: TooledFlags is a plain []string (SLICE regime); Experimental is *bool (POINTER regime).
  - DO NOT: make Experimental a plain bool; do NOT make TooledFlags a *[]string.

Task 2: UPDATE the struct-level doc comment's slice enumeration
  - LOCATE: the DESIGN CALL paragraph in the Manifest doc comment containing
    `Slices (Subcommand, BareFlags) and the Env map stay plain`.
  - EDIT: change `Slices (Subcommand, BareFlags)` → `Slices (Subcommand, BareFlags, TooledFlags)`.
  - WHY: keeps the regime documentation internally consistent now that a third slice field exists.

Task 3: UPDATE Resolve() — default Experimental; leave TooledFlags as-is
  - LOCATE: Resolve()'s pointer-scalar default block (the `if out.X == nil { out.X = ... }` sequence).
  - ADD (logically next to the other *bool, StripCodeFence, or at the end of the pointer block):
        if out.Experimental == nil {
            out.Experimental = boolPtr(false) // §12.7.2 default: non-experimental unless explicitly set
        }
  - PRESERVE explicit values: the `if == nil` guard means a non-nil Experimental (incl. *true) is kept.
  - DO NOT touch TooledFlags in Resolve (nil stays nil — slice regime).
  - LOCATE: Resolve()'s trailing comment `// Subcommand / BareFlags / Env: left as-is (nil stays nil).`
  - EDIT: change to `// Subcommand / BareFlags / TooledFlags / Env: left as-is (nil stays nil).`
  - OPTIONAL: the Resolve doc comment says "The four PRD-defaulted fields take their §12.1 defaults" —
    Experimental is a §12.7.2 default (false); either keep "four" (strictly §12.1) or reword. Not load-bearing.

Task 4: CONFIRM Validate() needs no change (and make none)
  - Validate() enforces Name/Command required + PromptDelivery/Output enums. Neither TooledFlags
    (free flag slice) nor Experimental (free *bool) has enum/required semantics at this layer.
  - ACTION: do NOT edit Validate(). (A nil Experimental passes Validate; Resolve guarantees non-nil after.)
  - VERIFY: `go build ./internal/provider/` compiles with Validate untouched.

Task 5: EXTEND manifest_test.go to prove the two new contracts
  - REUSE same-package helpers strPtr/boolPtr and the existing `Manifest{Name:..., Command:...}` style.
  - EXTEND TestResolve_SlicesLeftNil (:398) — after the Subcommand/BareFlags nil assertions, ADD:
        if r.TooledFlags != nil {
            t.Errorf("TooledFlags = %v, want nil (left as-is, slice regime)", r.TooledFlags)
        }
  - EXTEND TestResolve_AppliesDefaultsToNilOptionals (:343) — ADD:
        if r.Experimental == nil || *r.Experimental != false {
            t.Errorf("Experimental = %v, want non-nil *false (default non-experimental)", r.Experimental)
        }
  - EXTEND TestResolve_PreservesExplicitValues (:355) — set `Experimental: boolPtr(true)` in the input
    and ADD: `if r.Experimental == nil || *r.Experimental != true { t.Errorf("Experimental not preserved") }`.
  - OPTIONAL: extend TestUnmarshal_FullManifest (:30) — the pi TOML has neither key → assert
    `m.TooledFlags == nil && m.Experimental == nil` (absent → nil for both regimes).
  - NAMING/PLACEMENT: extend the EXISTING test functions (no new Test* functions required); co-located.
  - DO NOT: add merge/render tests (S2/T2); do NOT touch builtin/registry/parse tests.

Task 6: VALIDATE (run all gates; all must pass before declaring done)
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: gofmt -l .            # must be empty
  - RUN: go test -race ./...   # provider tests green; full suite green (no other package touched)
  - FIX-FORWARD: read failures, fix, re-run. Do NOT skip.
```

### Implementation Patterns & Key Details

```go
// === Manifest struct — the new fields in context (after BareFlags, before Output) ===

	// --- bare mode (§12.1) ---
	BareFlags []string `toml:"bare_flags"` // appended verbatim; nil => none.

	// --- tooled mode (v2; §11.5, §12.1) ---
	// Flags for the STAGER role (tools on, git-scoped, non-interactive). nil/empty => this
	// provider does not support tooled mode and cannot serve as a stager. Used in place of
	// BareFlags when mode=="tooled" in Render.
	TooledFlags []string `toml:"tooled_flags"`

	// --- experimental (§12.7.2, §12.5.1) ---
	// true => provider ships from docs/issue-tracker research, not a verified --help.
	// `providers list` marks experimental providers distinctly.
	Experimental *bool `toml:"experimental"`

	// --- output (§12.1) ---
	Output         *string `toml:"output"`           // raw|json; nil => Resolve→"raw".
```

```go
// === Resolve() — the new default line + extended trailing comment ===

	if out.StripCodeFence == nil {
		out.StripCodeFence = boolPtr(DefaultStripCodeFence)
	}
	if out.Experimental == nil {
		out.Experimental = boolPtr(false) // §12.7.2 default: non-experimental unless explicitly set
	}
	if out.RetryInstruction == nil {
		out.RetryInstruction = strPtr(DefaultRetryInstruction)
	}
	// Subcommand / BareFlags / TooledFlags / Env: left as-is (nil stays nil).
	return out
```

```go
// === Test extensions (manifest_test.go, same package) ===

// in TestResolve_SlicesLeftNil, after the BareFlags nil-check:
	if r.TooledFlags != nil {
		t.Errorf("TooledFlags = %v, want nil (left as-is, slice regime)", r.TooledFlags)
	}

// in TestResolve_AppliesDefaultsToNilOptionals:
	if r.Experimental == nil || *r.Experimental != false {
		t.Errorf("Experimental = %v, want non-nil *false (default non-experimental)", r.Experimental)
	}

// in TestResolve_PreservesExplicitValues (add Experimental: boolPtr(true) to the input Manifest):
	if r.Experimental == nil || *r.Experimental != true {
		t.Errorf("Experimental = %v, want non-nil *true (explicit value preserved)", r.Experimental)
	}
```

### Integration Points

```yaml
MANIFEST STRUCT (internal/provider/manifest.go):
  - field added: "TooledFlags []string `toml:\"tooled_flags\"`"   # SLICE regime (v2 stager; §11.5/§12.1)
  - field added: "Experimental *bool `toml:\"experimental\"`"    # POINTER regime (§12.7.2/§12.5.1)
  - doc comments: inline on both fields (Mode A — no docs/*.md change)

RESOLVE (internal/provider/manifest.go):
  - line added: "if out.Experimental == nil { out.Experimental = boolPtr(false) }"
  - TooledFlags: untouched (nil stays nil); trailing "left as-is" comment extended to name it

VALIDATE: UNCHANGED (no new rules)

NO-TOUCH (explicitly — owned by other subtasks):
  - internal/provider/merge.go     # MergeManifest handles the 2 fields in S2 (P1.M1.T1.S2)
  - internal/provider/render.go    # RenderMode (bare/tooled) in T2 (P1.M1.T2.S1)
  - internal/provider/builtin.go   # agy + tooled_flags on pi/claude in P1.M2.T1/T2
  - internal/provider/parse.go, executor.go, registry.go   # read other fields; unaffected
  - providers/*.toml               # P1.M2 (agy.toml; tooled_flags values)
  - docs/*.md                      # contract: "No user-facing docs change yet"
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — implemented by LATER subtasks, NOT S1):
  - S2  (P1.M1.T1.S2): MergeManifest — TooledFlags via len(override)>0; Experimental via != nil
  - T2  (P1.M1.T2.S1): Render gains variadic RenderMode; tooled mode errors if TooledFlags empty
  - M2  (P1.M2):       agy builtin (Experimental=true); tooled_flags on pi/claude; priority reorder
  - P3:                decompose stager uses RenderTooled + TooledFlags
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand

gofmt -l .                       # Expected: empty (run `gofmt -w internal/provider/manifest.go manifest_test.go` if listed)
go vet ./internal/provider/...   # Expected: exit 0
go build ./...                   # Expected: exit 0 (builtins/merge/render/parse all compile unchanged)

# Expected: Zero output/errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagehand

# The provider package — the extended Resolve/unmarshal tests
go test -race ./internal/provider/ -v -run 'TestResolve_|TestUnmarshal_FullManifest'

# Full provider suite
go test -race ./internal/provider/ -v

# Expected: extended tests PASS — TooledFlags stays nil through Resolve; Experimental defaults to
# non-nil *false; explicit Experimental=true survives Resolve. No other test changes.
```

### Level 3: Whole-Repository Regression (No Behavior Change Elsewhere)

```bash
cd /home/dustin/projects/stagehand

go test -race ./...              # Expected: ALL packages pass (no other package edited)
go vet ./...                     # Expected: exit 0

# Confirm ONLY internal/provider/manifest.go (+ test) changed in production source
git diff --stat -- internal/ pkg/ cmd/ providers/
# Expected: only internal/provider/manifest.go + internal/provider/manifest_test.go appear.

# Confirm Validate() was NOT edited (no new rules)
git diff -- internal/provider/manifest.go | grep -E '^\+.*Validate|^\+.*return errors' || echo "OK: no Validate changes"
```

### Level 4: Schema Sanity (prove the fields decode + default correctly)

```bash
cd /home/dustin/projects/stagehand

# Inline behavioral check via a one-off test run (the extended TestResolve_* cover this; this is a
# manual cross-check). Decode a TOML with both keys and confirm round-trip through Resolve:
cat > /tmp/sh_schema_test.go <<'EOF'
package main
import ("fmt"; "github.com/dustin/stagehand/internal/provider"; "github.com/pelletier/go-toml/v2")
func main() {
  src := []byte(`name="x"
command="x"
tooled_flags=["--allowed-tools","git:*"]
experimental=true
`)
  var m provider.Manifest
  if err := toml.Unmarshal(src, &m); err != nil { fmt.Println("unmarshal err:", err); return }
  r := m.Resolve()
  fmt.Printf("TooledFlags=%v Experimental(explicit)=%v\n", r.TooledFlags, *r.Experimental)
  // expect: TooledFlags=[--allowed-tools git:*] Experimental(explicit)=true
}
EOF
go run /tmp/sh_schema_test.go && rm -f /tmp/sh_schema_test.go
# Expected: TooledFlags=[--allowed-tools git:*]  Experimental(explicit)=true
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages pass (provider tests extended + green; no other package touched).

### Feature Validation

- [ ] `Manifest` has `TooledFlags []string` (`toml:"tooled_flags"`) with a §11.5/§12.1 doc comment.
- [ ] `Manifest` has `Experimental *bool` (`toml:"experimental"`) with a §12.7.2/§12.5.1 doc comment.
- [ ] `Resolve()` defaults Experimental to non-nil `*false`; preserves an explicit `*true`.
- [ ] `Resolve()` leaves TooledFlags nil when absent (slice regime); "left as-is" comment names it.
- [ ] `Validate()` is unchanged (git diff shows no Validate edits).
- [ ] Struct doc comment's slice enumeration includes TooledFlags.

### Scope Discipline Validation

- [ ] ONLY `internal/provider/manifest.go` (+ `manifest_test.go`) modified (git diff --stat confirms).
- [ ] Did NOT edit `merge.go` (S2), `render.go` (T2), `builtin.go`/`providers/*.toml` (P1.M2).
- [ ] Did NOT add agy or set tooled_flags/experimental on any existing builtin.
- [ ] Did NOT add a Validate rule (none needed).
- [ ] Did NOT edit any `docs/*.md` (inline struct comments only — contract).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Field regimes match precedents (TooledFlags=slice like BareFlags; Experimental=*bool like StripCodeFence).
- [ ] Doc comments cite the PRD sections (§11.5/§12.1, §12.7.2/§12.5.1) per the architecture delta.
- [ ] Test extensions reuse existing helpers (strPtr/boolPtr) and assertion idioms; no new test file.
- [ ] go-toml omitempty constraint respected (Experimental is *bool so absent≠false).

---

## Anti-Patterns to Avoid

- ❌ Don't make `Experimental` a plain `bool` — go-toml/v2 has no omitempty, so plain bool can't
  distinguish "absent in override" (→ inherit) from "explicitly false" (→ override). It MUST be `*bool`
  (same reason StripCodeFence is). The S2 merge relies on this.
- ❌ Don't make `TooledFlags` a `*[]string` — slices already have a natural nil absent-sentinel
  (absent→nil, present→non-nil-even-empty), matching BareFlags/Subcommand. A pointer-on-slice adds
  dereference noise with zero benefit; the struct doc comment explicitly rejects it.
- ❌ Don't add a `Validate()` rule for either field — TooledFlags' "tooled mode requires non-empty"
  check is a Render-time concern (T2), and Experimental is a free *bool with nil allowed (Resolve
  defaults it). Adding a Validate rule here is out of scope and would break the "nil passes Validate"
  contract.
- ❌ Don't default `TooledFlags` in Resolve (e.g. to `[]string{}`) — nil MUST stay nil so the slice
  regime's "absent" sentinel is preserved and S2's `len(override) > 0` merge + T2's
  `len(TooledFlags) == 0 → tooled error` both work.
- ❌ Don't add agy, don't set tooled_flags/experimental on pi/claude, don't reorder preferredBuiltins,
  don't change pi's default_model — all of that is P1.M2 (separate subtasks).
- ❌ Don't edit `merge.go`/`render.go` — those are S2 and T2 respectively.
- ❌ Don't update `docs/*.md` — the contract says inline struct doc comments only (Mode A), no
  user-facing docs yet (those ride with the user-visible subtasks / the P4.M3 doc sweep).
- ❌ Don't forget to extend the two enumerated comments (struct doc `Slices (...)` and Resolve's
  `left as-is` line) — leaving them listing only Subcommand/BareFlags makes the docs inconsistent.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a two-field, single-production-file, additive schema change where both fields map
cleanly onto established regimes with named precedents (TooledFlags≡BareFlags slice regime;
Experimental≡StripCodeFence pointer regime). The architecture delta (`manifest_v2_delta.md` §1)
prescribes the verbatim field declarations, doc comments, the single Resolve line, and explicitly
confirms "No new validation rules." Existing builtins use named struct literals (verified), so zero
edits are required outside manifest.go(+test). The only residual uncertainty (not 10/10) is the minor
doc-comment wording on the Resolve "four PRD-defaulted fields" line (cosmetic, not load-bearing) and
the optional TestUnmarshal extension — both gated by the deterministic `go test -race` + `gofmt`
validation. The Merge/Render/agy boundaries are cleanly fenced off to S2/T2/M2 and cannot be broken by
S1 because the new fields are nil-by-default and unused until those subtasks wire them in.
