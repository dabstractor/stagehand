---
name: "P1.M1.T1.S1 — Add SessionMode field to Manifest struct + Resolve() default + Validate() enum"
description: |
  Pure schema addition (FR-T8 provider capability flag, step 1 of 5). Add `SessionMode *string`
  (`toml:"session_mode"`) to the Manifest struct between ProviderFlag and BareFlags (PRD §12.1 fixes the
  TOML ordering). `*string` pointer-scalar convention (go-toml/v2 has no omitempty): absent → nil → inherit
  on merge; present → override. Resolve() adds `if out.SessionMode == nil { out.SessionMode = strPtr("") }`
  (default "" = no session support). Validate() adds a nil-tolerant enum check (`""` | `"append"`; nil passes).
  Nothing reads it yet — consumed by S2 (MergeManifest), S3 (RenderMultiTurn capability gate), S4 (pi value).
  No docs in S1 (rides with S5). gofmt-aligns the struct block.
---

## Goal

**Feature Goal**: Land the `session_mode` manifest field — the FR-T8 provider capability flag that gates the
multi-turn generation fallback (§9.24) per provider. The field exists on the schema with correct TOML
ordering, pointer-scalar merge semantics, a Resolve default (`""`), and Validate enum enforcement, so the
downstream subtasks (S2 merge, S3 render+gate, S4 pi value) have a stable struct member to wire.

**Deliverable** (3 production edits in 1 file + 4 test extensions in 1 file):
1. `internal/provider/manifest.go` Manifest struct: add `SessionMode *string \`toml:"session_mode"\`` in a
   new `// --- session continuation (multi-turn fallback, §9.24) ---` block between `ProviderFlag` (sub-provider)
   and `BareFlags` (bare mode).
2. `internal/provider/manifest.go` `Resolve()`: add `if out.SessionMode == nil { out.SessionMode = strPtr("") }`
   (after the ProviderFlag default, before the Output block).
3. `internal/provider/manifest.go` `Validate()`: add the nil-tolerant enum check (`""` | `"append"`; nil passes)
   after the Output enum block.
4. `internal/provider/manifest_test.go`: extend `TestValidate` (+BadSessionMode_Errors), `TestResolve_AppliesDefaultsToNilOptionals`
   (+SessionMode non-nil `""`), `TestResolve_PreservesExplicitValues` (+SessionMode `"append"` preserved).

**Success Definition**: `Manifest.SessionMode` exists with the correct TOML tag + placement; `Resolve()`
guarantees `*r.SessionMode` is non-nil (safe deref for S3's render/gate); `Validate()` rejects bogus values
and accepts nil/`""`/`"append"`; all existing manifest tests stay green; the field is dead (unconsumed) until
S3/S4. `go build/vet/gofmt` clean; `go test -race ./...` green.

## User Persona

**Target User**: The contributor implementing the immediately-following provider-surface subtasks (S2
MergeManifest, S3 RenderMultiTurn capability gate, S4 the pi `"append"` value) and the eventual end user
who benefits from multi-turn fallback for large diffs on session-capable providers.

**Use Case**: A user opts into multi-turn fallback (default-on, FR-T1) on a provider whose manifest declares
`session_mode = "append"` (pi). S3's trigger gate checks `*r.SessionMode == "append"` (condition d of FR-T1);
S3's RenderMultiTurn renders the `--session-id` turns. S1 ships the field those consumers read.

**Pain Points Addressed**: Removes the "where does session_mode live on Manifest, and what are its
semantics?" gap that would block S2/S3/S4.

## Why

- **First step of a 5-step provider-surface chain.** The multi-turn fallback (§9.24) needs a per-provider
  capability flag. S1 is the schema foundation; S2 makes it config-overridable, S3 adds the render variant
  + capability gate, S4 sets pi's verified value, S5 docs it. Splitting keeps each subtask minimal.
- **Pure schema addition = lowest risk.** Nothing reads the field yet, so there is no behavior change. The
  field is dead until S3/S4. Existing tests are field-specific (not exhaustive DeepEqual), so they stay green.
- **PRD-mandated placement + convention.** §12.1 fixes the TOML ordering (between `provider_flag` and
  `bare_flags`); the pointer-scalar convention (manifest.go:14-24 doc block) is the established pattern for
  every optional scalar (ProviderFlag/PrintFlag/SystemPromptFlag/Output/StripCodeFence/Experimental/...).
  SessionMode follows it exactly.
- **Resolve guarantees safe deref.** S3's RenderMultiTurn capability gate does `*r.SessionMode == "append"`;
  the Resolve default ensures that deref is safe even for a manifest that never set the field (every built-in
  except pi, after S4).

## What

A purely additive schema change: one new `*string` field, one Resolve default, one Validate enum check, and
four test assertions. No logic that consumes the field, no caller change, no docs, no builtin value. The
field is dead (unconsumed) after S1.

### Success Criteria

- [ ] `Manifest` has `SessionMode *string \`toml:"session_mode"\`` in a new `// --- session continuation
      (multi-turn fallback, §9.24) ---` block between `ProviderFlag` and `BareFlags`.
- [ ] `Resolve()` sets `out.SessionMode = strPtr("")` when nil (after the ProviderFlag default).
- [ ] `Validate()` rejects non-nil SessionMode not in `{"", "append"}`; nil passes (absent case).
- [ ] `TestValidate_BadSessionMode_Errors` passes (bogus value → error).
- [ ] `TestResolve_AppliesDefaultsToNilOptionals` asserts `*r.SessionMode == ""` after Resolve.
- [ ] `TestResolve_PreservesExplicitValues` asserts `SessionMode: strPtr("append")` is preserved (not clobbered).
- [ ] All existing manifest tests stay green (no edit to them beyond the named extensions).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] NO builtin value set (S4's job), NO MergeManifest clause (S2), NO render/gate (S3), NO docs (S5).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact current struct region (ProviderFlag/BareFlags), the exact
Validate() body, the exact Resolve() tail, the verbatim new field/Resolve/Validate lines (with the doc
comment), the 4 test extensions (with the assertions), and the gofmt note. The pointer-scalar convention is
explained with its reason (go-toml no omitempty). The S1/S2/S3/S4/S5 boundary is explicit.

### Documentation & References

```yaml
# MUST READ — the authoritative placement + convention
- docfile: plan/009_5c53066d64b3/architecture/research-provider.md
  why: "§1 confirms SessionMode does NOT exist; §1 'Where SessionMode slots in' pins the placement (between ProviderFlag line 60-61 and BareFlags line 63-64, per PRD §12.1 TOML ordering PRD.md:726-733); §1 'Why *string' explains the pointer-scalar convention (go-toml no omitempty → absent=nil=inherit, present=override); §1 Resolve/Validate guidance."
  critical: "§1 is the authority that SessionMode is *string (NOT plain string), slots between ProviderFlag and BareFlags, and Resolve defaults to ''. The enum is '' | 'append' (PRD §12.1 / FR-T8)."

# The single production file under edit
- file: internal/provider/manifest.go
  why: "EDIT (3 spots). (a) Struct: insert SessionMode block between ProviderFlag (line 59) and BareFlags (line 62). (b) Resolve(): insert the nil→strPtr('') clause after the ProviderFlag default (line 162-164), before Output. (c) Validate(): insert the enum check after the Output block (line 110-112), before return nil."
  pattern: "Mirror ProviderFlag exactly: *string + toml snake_case tag + section doc-comment. Resolve mirrors `if out.ProviderFlag == nil { out.ProviderFlag = strPtr(\"\") }`. Validate mirrors the Output nil-tolerant enum check."
  gotcha: "SessionMode is *string (pointer-scalar), NOT plain string — the go-toml-no-omitempty convention (manifest.go:14-24). A plain string could not distinguish absent (inherit) from explicit '' (override) during the S2 field-merge."

- file: internal/provider/manifest_test.go
  why: "EDIT (4 extensions). TestValidate_BadOutput_Errors (:285) is the template for a new TestValidate_BadSessionMode_Errors. TestResolve_AppliesDefaultsToNilOptionals (:337) + TestResolve_PreservesExplicitValues (:352) get one new assertion each. TestValidate_NilEnumsAreOK (:294) already covers the nil-passes case (no edit)."
  pattern: "Same-package tests using strPtr/boolPtr + Manifest{...} literals. Error idiom: `if err == nil || !strings.Contains(err.Error(), \"...\")`. Resolve idiom: `r := m.Resolve(); if r.X == nil || *r.X != want { ... }`."

- docfile: plan/009_5c53066d64b3/P1M1T1S1/research/s1_sessionmode_field.md
  why: "Distilled S1 findings: the verbatim field/Resolve/Validate lines, the pointer-scalar rationale, the 4 test extensions, and the S1/S2/S3/S4/S5 scope boundary."

# PRD authority (already in the selected content)
- prd: PRD.md §9.24 FR-T8 (session_mode field, "" default | "append"); FR-T9 (verification duty); §12.1 manifest schema (session_mode between provider_flag and bare_flags).
  why: "The authoritative enum + placement + the 'never set speculatively' rule (FR-T9)."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/provider/
    ├── manifest.go        # EDIT: + SessionMode field (struct); + Resolve default; + Validate enum
    └── manifest_test.go   # EDIT: +4 test extensions (BadSessionMode, Resolve default, explicit-preserve)
```

### Desired Codebase Tree After S1

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/provider/manifest.go        # +SessionMode field + Resolve default + Validate enum
    internal/provider/manifest_test.go   # +4 test assertions
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/provider/manifest.go` | MODIFY | + `SessionMode *string` field (between ProviderFlag/BareFlags); Resolve `""` default; Validate `""\|"append"` enum. **Only production file.** |
| `internal/provider/manifest_test.go` | MODIFY | + `TestValidate_BadSessionMode_Errors`; extend `TestResolve_AppliesDefaultsToNilOptionals` + `TestResolve_PreservesExplicitValues`. |

**Explicitly NOT touched**: `merge.go` (S2 = P1.M1.T1.S2), `render.go` / any RenderMultiTurn (S3 =
P1.M1.T1.S3), `builtin.go` / `providers/pi.toml` (S4 = P1.M1.T1.S4), `docs/*` (S5 = P1.M1.T1.S5), any other
package, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (*string, not string): go-toml/v2 has NO omitempty (manifest.go:14-24 doc block). Optional
// SCALAR fields are *string/*bool so an ABSENT user override decodes to nil (→ inherit the built-in on
// merge) while a PRESENT value (even "") decodes non-nil (→ override). SessionMode MUST be *string — a
// plain string would make S2's field-merge unable to distinguish "absent" from "explicit ''".

// CRITICAL (placement): PRD §12.1 FIXES the TOML ordering — session_mode sits between provider_flag and
// bare_flags. Insert the new block BETWEEN ProviderFlag (line 59) and BareFlags (line 62), NOT elsewhere.
// (The struct field order is otherwise conventional; this one is PRD-mandated.)

// CRITICAL (Resolve default is "", NOT "append"): Resolve sets nil→strPtr(""). Only pi (S4) sets
// strPtr("append") (FR-T9 verified). S1 does NOT set any builtin value — that's S4's job. The default
// means multi-turn is UNAVAILABLE for every provider until S4 sets pi.

// GOTCHA (Validate nil-tolerant): mirror the Output/PromptDelivery pattern — `if m.SessionMode != nil`
// guard around the enum check, so a nil (absent) SessionMode PASSES Validate (the partial-override case).
// Non-nil must be "" or "append".

// GOTCHA (gofmt): the struct fields are column-aligned (type/tag column pads to the longest name).
// SessionMode (11 chars) is shorter than SystemPromptFlag/RetryInstruction; gofmt realigns the block.
// RUN `gofmt -w internal/provider/manifest.go` after the edit.

// GOTCHA (dead field): after S1, NOTHING reads SessionMode. MergeManifest (S2) has no clause for it yet;
// RenderMultiTurn (S3) doesn't exist; pi (S4) doesn't set it. So Config/manifest behavior is UNCHANGED.
// This is the intended "no behavior change yet" state.
```

## Implementation Blueprint

### Data models and structure

No new types — one new `*string` field. The relevant existing precedent (unchanged):

```go
// internal/provider/manifest.go (EXISTING pointer-scalar precedents — the model to mirror)
ProviderFlag    *string `toml:"provider_flag"`
SystemPromptFlag *string `toml:"system_prompt_flag"`
Experimental    *bool   `toml:"experimental"`
// Resolve:  if out.ProviderFlag == nil { out.ProviderFlag = strPtr("") }
// Validate: if m.Output != nil { if _, ok := validOutputs[*m.Output]; !ok { ... } }
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: manifest.go — add the SessionMode field (struct)
  - LOCATE: the `// --- sub-provider (§12.1) ---` block ending with `ProviderFlag *string \`toml:"provider_flag"\``
    (line 59), immediately before the `// --- bare mode (§12.1) ---` block (BareFlags, line 62).
  - INSERT between them (PRD §12.1 mandates this ordering):
        // --- session continuation (multi-turn fallback, §9.24) ---
        // "" (default): provider cannot append turns across one-shot calls → multi-turn fallback unavailable
        //   for this provider (one-shot → rescue, unchanged). "append": re-invoking the same session id
        //   appends a turn the model can recall (pi: `--session-id <id> ... -p`, repeated). REQUIRES a
        //   verified append rendering (FR-T9); never set speculatively. nil => Resolve→"".
        SessionMode *string `toml:"session_mode"`
  - TYPE: *string (pointer-scalar — see gotcha). TOML TAG: exactly `session_mode`.
  - DO NOT: place it elsewhere; use a plain string; set a builtin value (S4).

Task 2: manifest.go — Resolve() default
  - LOCATE: Resolve()'s `if out.ProviderFlag == nil { out.ProviderFlag = strPtr("") }` block (line 162-164),
    immediately before the `if out.Output == nil { ... }` block.
  - INSERT after the ProviderFlag block, before Output:
        if out.SessionMode == nil {
            out.SessionMode = strPtr("")
        }
  - DO NOT: default to "append" (the default is "" = no support; only pi/S4 sets "append").

Task 3: manifest.go — Validate() enum
  - LOCATE: Validate()'s Output enum block (line 109-112), immediately before `return nil` (line 114).
  - INSERT after the Output block, before return nil:
        if m.SessionMode != nil {
            if *m.SessionMode != "" && *m.SessionMode != "append" {
                return fmt.Errorf("provider manifest %q: session_mode %q must be \"\" or \"append\"", m.Name, *m.SessionMode)
            }
        }
  - DO NOT: reject nil (nil = absent = passes, the partial-override case); reject "" ("" is the default/no-op).

Task 4: manifest_test.go — extend the existing tests
  - ADD TestValidate_BadSessionMode_Errors (mirror TestValidate_BadOutput_Errors :285):
        func TestValidate_BadSessionMode_Errors(t *testing.T) {
            m := Manifest{Name: "x", Command: strPtr("x"), SessionMode: strPtr("bogus")}
            err := m.Validate()
            if err == nil { t.Fatal("Validate err = nil, want non-nil for bogus session_mode") }
            if !strings.Contains(err.Error(), "session_mode") {
                t.Errorf("err = %v, want it to mention session_mode", err)
            }
        }
  - EXTEND TestResolve_AppliesDefaultsToNilOptionals (:337) — add:
        if r.SessionMode == nil || *r.SessionMode != "" {
            t.Errorf("SessionMode = %v, want non-nil *\"\" (default no support)", r.SessionMode)
        }
  - EXTEND TestResolve_PreservesExplicitValues (:352) — add `SessionMode: strPtr("append")` to the input
    Manifest and assert:
        if r.SessionMode == nil || *r.SessionMode != "append" {
            t.Errorf("SessionMode = %v, want non-nil *\"append\" (explicit preserved)", r.SessionMode)
        }
  - TestValidate_NilEnumsAreOK (:294) already covers nil-SessionMode-passes (no edit needed — it builds a
    Manifest with no SessionMode, which is nil → passes).
  - DO NOT: add merge/render tests (S2/S3); touch other tests.

Task 5: VALIDATE
  - RUN: gofmt -w internal/provider/manifest.go internal/provider/manifest_test.go
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test -race ./internal/provider/ -v -run 'TestValidate|TestResolve'
  - RUN: go test -race ./...   # full suite green
  - FIX-FORWARD: read failures, fix, re-run.
```

### Implementation Patterns & Key Details

```go
// === manifest.go — the new struct block (between ProviderFlag and BareFlags) ===
	// --- sub-provider (§12.1) ---
	ProviderFlag *string `toml:"provider_flag"`

	// --- session continuation (multi-turn fallback, §9.24) ---
	// "" (default): provider cannot append turns across one-shot calls → multi-turn fallback unavailable
	//   for this provider (one-shot → rescue, unchanged). "append": re-invoking the same session id
	//   appends a turn the model can recall (pi: `--session-id <id> ... -p`, repeated). REQUIRES a
	//   verified append rendering (FR-T9); never set speculatively. nil => Resolve→"".
	SessionMode *string `toml:"session_mode"`

	// --- bare mode (§12.1) ---
	BareFlags []string `toml:"bare_flags"` // appended verbatim; nil => none.
```

```go
// === Resolve() — the new default clause (after ProviderFlag, before Output) ===
	if out.ProviderFlag == nil {
		out.ProviderFlag = strPtr("")
	}
	if out.SessionMode == nil {
		out.SessionMode = strPtr("")
	}

	if out.Output == nil {
		out.Output = strPtr(DefaultOutput)
	}
```

```go
// === Validate() — the new enum check (after Output, before return nil) ===
	if m.Output != nil {
		if _, ok := validOutputs[*m.Output]; !ok {
			return fmt.Errorf("provider manifest %q: output %q must be one of raw|json", m.Name, *m.Output)
		}
	}
	if m.SessionMode != nil {
		if *m.SessionMode != "" && *m.SessionMode != "append" {
			return fmt.Errorf("provider manifest %q: session_mode %q must be \"\" or \"append\"", m.Name, *m.SessionMode)
		}
	}
	return nil
```

### Integration Points

```yaml
MANIFEST STRUCT (internal/provider/manifest.go):
  - field added: "SessionMode *string `toml:\"session_mode\"`"  # pointer-scalar; between ProviderFlag and BareFlags

RESOLVE (internal/provider/manifest.go):
  - clause added: "if out.SessionMode == nil { out.SessionMode = strPtr(\"\") }"  # default "" (no support)

VALIDATE (internal/provider/manifest.go):
  - clause added: nil-tolerant enum check ("" | "append"; nil passes)

TESTS (internal/provider/manifest_test.go):
  - +TestValidate_BadSessionMode_Errors
  - +assertion in TestResolve_AppliesDefaultsToNilOptionals (*r.SessionMode == "")
  - +assertion in TestResolve_PreservesExplicitValues (*r.SessionMode == "append" preserved)

NO-TOUCH (explicitly — owned by sibling subtasks):
  - internal/provider/merge.go        # S2 (P1.M1.T1.S2): MergeManifest scalar clause for SessionMode
  - internal/provider/render.go       # S3 (P1.M1.T1.S3): RenderMultiTurn + capability gate (*r.SessionMode == "append")
  - internal/provider/builtin.go + providers/pi.toml  # S4 (P1.M1.T1.S4): pi SessionMode="append" (FR-T9 verified)
  - docs/*                            # S5 (P1.M1.T1.S5): manifest-schema doc / providers.md / configuration.md
  - any other package; PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — S2/S3/S4 own):
  - S2: MergeManifest adds `if override.SessionMode != nil { out.SessionMode = override.SessionMode }` (scalar regime).
  - S3: RenderMultiTurn errors if `*r.SessionMode != "append"` (capability gate, FR-T8/T9); renders --session-id turns.
  - S4: builtinPi sets `SessionMode: strPtr("append")`; providers/pi.toml adds `session_mode = "append"`.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/provider/manifest.go internal/provider/manifest_test.go   # realign the struct block
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/provider/...   # Expected: exit 0
go build ./...                   # Expected: exit 0 (new field; nothing reads it yet)
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

# The Validate + Resolve tests (incl. the new BadSessionMode + the Resolve extensions)
go test -race ./internal/provider/ -v -run 'TestValidate|TestResolve'

# Full provider suite
go test -race ./internal/provider/ -v

# Expected: TestValidate_BadSessionMode_Errors PASSES; TestResolve_AppliesDefaultsToNilOptionals asserts *r.SessionMode=="";
#           TestResolve_PreservesExplicitValues asserts *r.SessionMode=="append"; every other test UNCHANGED (green).
```

### Level 3: Whole-Repository Regression (no collateral)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages green (S1 adds a dead field; no behavior change)
go vet ./...                     # Expected: exit 0

# Confirm ONLY the 2 intended files changed
git diff --stat -- internal/ pkg/ cmd/ docs/ providers/
# Expected: internal/provider/manifest.go + internal/provider/manifest_test.go only.
```

### Level 4: Dead-Field Confirmation (the field is unconsumed)

```bash
cd /home/dustin/projects/stagecoach

# Nothing reads m.SessionMode / r.SessionMode yet (S3/S4 wire them). Confirm:
grep -rn "\.SessionMode" --include="*.go" internal/ pkg/ cmd/ | grep -v "_test.go" | grep -v "/plan/"
# Expected: only manifest.go's struct decl + Resolve clause + Validate clause (no consumer derefs it for behavior).
# (A config [provider.X] session_mode="..." override still has NO effect after S1 — by design. S2/S3 change that.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.

### Feature Validation

- [ ] `Manifest.SessionMode *string \`toml:"session_mode"\`` exists between ProviderFlag and BareFlags.
- [ ] `Resolve()` sets nil → `strPtr("")`.
- [ ] `Validate()` rejects non-nil not-in-{"","append"}; nil passes.
- [ ] `TestValidate_BadSessionMode_Errors` passes.
- [ ] `TestResolve_AppliesDefaultsToNilOptionals` asserts `*r.SessionMode == ""`.
- [ ] `TestResolve_PreservesExplicitValues` asserts `*r.SessionMode == "append"` preserved.

### Scope Discipline Validation

- [ ] ONLY `internal/provider/{manifest,manifest_test}.go` modified (git diff --stat confirms).
- [ ] Did NOT edit `merge.go` (S2), `render.go` (S3), `builtin.go`/`providers/*.toml` (S4), docs (S5).
- [ ] Did NOT set any builtin SessionMode value (S4's job — pi only, FR-T9 verified).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Field mirrors the pointer-scalar convention (ProviderFlag/PrintFlag/Experimental).
- [ ] Resolve/Validate clauses mirror the ProviderFlag/Output precedents exactly.
- [ ] gofmt run (struct block column-aligned automatically).

---

## Anti-Patterns to Avoid

- ❌ Don't make SessionMode a plain `string`. go-toml/v2 has no omitempty; the pointer-scalar convention
  (manifest.go:14-24) is the ONLY way S2's field-merge can distinguish absent (nil → inherit) from explicit
  `""` (non-nil → override). It MUST be `*string`.
- ❌ Don't default Resolve to `"append"`. The default is `""` (no session support → multi-turn unavailable).
  Only pi (S4) sets `"append"` (FR-T9 verified). A `"append"` default would silently enable multi-turn for
  providers whose append mechanism is unverified, violating FR-T9.
- ❌ Don't reject nil or `""` in Validate. nil = absent (the partial-override case) → passes; `""` = the
  default/no-support value → passes. Only non-nil values outside `{"","append"}` are errors. Mirror the
  Output nil-tolerant enum pattern exactly.
- ❌ Don't place the field outside the ProviderFlag/BareFlags gap. PRD §12.1 FIXES the TOML ordering
  (session_mode between provider_flag and bare_flags). The struct field order must match.
- ❌ Don't set a builtin value (pi's `"append"`) here — that's S4 (P1.M1.T1.S4), and it requires the FR-T9
  verification record. S1 is the struct/Resolve/Validate only; the field is dead until S3/S4.
- ❌ Don't add a MergeManifest clause (S2), a RenderMultiTurn method (S3), or docs (S5). S1 is the schema
  addition only.
- ❌ Don't hand-align the struct columns — run `gofmt -w`; it realigns the block.
- ❌ Don't break existing tests — they're field-specific (not exhaustive DeepEqual). Adding the field +
  extending the named tests is non-breaking. (Verified: the existing TestValidate_NilEnumsAreOK already
  covers nil-SessionMode-passes.)

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a 3-spot additive schema edit (struct field + Resolve clause + Validate clause) with the
exact current code quoted verbatim (the ProviderFlag/BareFlags region, the Resolve tail, the Validate body),
the verbatim new lines (field + doc comment + Resolve + Validate), and the PRD-mandated placement. Three
independent de-riskings: (1) the arch research-provider.md §1 confirms SessionMode is absent + pins the
placement + the *string convention; (2) the field mirrors two proven precedents exactly (ProviderFlag for
the pointer-scalar + Resolve default; Output for the Validate nil-tolerant enum); (3) the existing tests
are field-specific (not exhaustive DeepEqual), so they stay green — verified by the test-file grep. The field
is dead (unconsumed) after S1, so there is literally no behavior change to regress. The one residual
uncertainty (not 10/10) is purely mechanical gofmt alignment, which the `gofmt -w` gate catches immediately.
The S2/S3/S4/S5 consumers are cleanly fenced and untouched.
