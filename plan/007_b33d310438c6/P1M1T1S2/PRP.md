---
name: "P1.M1.T1.S2 — materialize + overlay field-merge (DiffContext *int) [CONTRACT CORRECTION]"
description: |
  Wire `TokenLimit` + `DiffContext` from the `fileGeneration` decode struct through `materialize()` and
  `overlay()` into `config.Config`. ⚠️ CONTRACT CORRECTION: the prescribed overlay guard
  `if src.DiffContext != 0` is PROVABLY BROKEN — `overlay()` sits between EVERY file/git-config layer and
  the final config (load.go:82→100→123→138; git.go:106 confirms the non-zero overlay), so an explicit
  `diff_context=0` would fail `0 != 0` and be silently clobbered back to the default 1. The contract's
  OWN end-to-end test ("explicit 0⇒0") would FAIL. The ONLY correct fix is `Config.DiffContext = *int`
  (nil = unset), extending S1's `*int` from the file struct to the resolved Config (precedent:
  `Output *string`, `StripCodeFence *bool`). TokenLimit stays plain int + `!= 0` everywhere (FR3d: 0 IS
  the unset sentinel — no meaningful "explicit 0"). S2 re-edits the 3 config.go DiffContext spots S1
  adds in parallel (field → *int, Defaults → intPtr(1), TestDefaults → *DiffContext==1); see Coordination.
---

## Goal

**Feature Goal**: Make `TokenLimit` (FR3d) and `DiffContext` (FR3f) propagate from a config file through
`materialize()` (file → Config) and `overlay()` (Config → Config, the cross-layer merge) with CORRECT
unset-vs-explicit semantics — critically, an explicit `diff_context = 0` (-U0, changed-lines-only) set in
a file MUST survive the overlay chain and yield `*cfg.DiffContext == 0` end-to-end, while an omitted key
MUST inherit the `-U1` default. TokenLimit uses the standard non-zero convention (0 = unset).

**Deliverable** (the corrected design):
1. `internal/config/config.go` — `Config.DiffContext`: plain `int` → `*int` (nil = unset); add an `intPtr` helper; `Defaults()` seeds `DiffContext: intPtr(1)` (non-nil). `TokenLimit` stays plain `int` (S1's field).
2. `internal/config/config_test.go` — `TestDefaults` DiffContext assertion → `*DiffContext == 1` (nil-safe).
3. `internal/config/file.go` — `fileGeneration.DiffContext`: plain `int` → `*int`; `materialize()` adds TokenLimit (`!= 0`) + DiffContext (`!= nil`) guards; `overlay()` mirrors them.
4. `internal/config/file_test.go` — a table-driven test (unset⇒1, explicit 1⇒1, explicit 0⇒0, explicit 3⇒3, across global-only / repo-only / global+repo overlay).

**Success Definition**: `go build/vet/gofmt` clean; `go test -race ./...` green; the table-driven test
passes (incl. explicit-0 ⇒ 0 through the overlay chain); `grep` confirms no `!= 0` overlay guard remains
on DiffContext. An explicit `diff_context = 0` in a global OR repo file yields `*cfg.DiffContext == 0`;
an omitted key yields `*cfg.DiffContext == 1` (the default). S3's git-config layer has a clear `*int`
contract to follow.

## User Persona

**Target User**: The contributor implementing S3 (git-config `stagehand.diffContext`), S4 (bootstrap/docs),
P1.M1.T2 (StagedDiffOptions), and P1.M2+ (the `-U<diff_context>` diff functions) — every downstream
subtask reads `cfg.DiffContext` / `cfg.TokenLimit`. And the user who sets `diff_context = 0` expecting
changed-lines-only diffs (FR3f).

**Use Case**: A user writes `[generation] diff_context = 0` in `.stagehand.toml` to maximize diff
savings; the resolved `cfg.DiffContext` must be `0`, not silently reverted to `1`.

**Pain Points Addressed**: Without the `*int` correction, `diff_context = 0` is silently impossible to
configure via any layer (file/git-config) — a violation of FR3f and of the contract's own end-to-end
requirement. The non-zero overlay convention (a documented v1 limitation for fields where 0 is invalid)
is structurally incapable of expressing "override to 0" for a plain int.

## Why

- **FR3f requires diff_context=0 to be configurable.** `0 = changed lines only (maximal savings)` is a
  first-class value, not an "unset". The contract's end-to-end test demands it survive the overlay chain.
- **The contract's overlay guard is broken (proven).** `overlay()` is between every layer and the final
  config (load.go:82 `Defaults()` → :100/:123/:138 `overlay(&cfg, materialize/file/gitconfig)`). With
  `if src.DiffContext != 0`, an explicit 0 fails the guard (`0 != 0` is false) and the default 1 survives.
  The `*int` on `fileGeneration` alone is insufficient — the unset/explicit-0 bit is lost at materialize's
  `*int → int` collapse, and overlay (Config→Config, plain int) cannot recover it. `git.go:106` confirms
  even the git-config layer uses the non-zero overlay, so NO layer could set 0 under the contract's design.
- **`*int` is the established nullable-scalar precedent.** `Output *string` and `StripCodeFence *bool`
  (config.go:88-89,97) exist precisely so overlay's `!= nil` guard can distinguish "unset" (nil) from an
  explicit zero-value. DiffContext needs the same treatment; it is the cleanest, most consistent fix.
- **TokenLimit is different (and the contract is right about it).** FR3d: `token_limit` default `0` =
  unset ⇒ legacy caps; a non-zero value supersedes them. There is no meaningful "explicit 0", so plain
  `int` + `!= 0` in both materialize and overlay is correct (matches MaxDiffBytes/MaxMdLines).
- **Fulfills S1's stated intent.** S1 explicitly planned `*int` on `fileGeneration.DiffContext` "to
  disambiguate 0 (changed-lines-only) from unset." That intent is only achievable by extending `*int` to
  `Config.DiffContext`; S1 scoped it to the file struct and did not trace through overlay. S2 completes
  the intent correctly.

## What

A field-merge wiring with one structural correction: `Config.DiffContext` becomes `*int` (nil = unset) so
the overlay chain can carry an explicit 0. `TokenLimit` stays plain `int` with the standard `!= 0` guard.
`materialize` and `overlay` each get two new guards. A table-driven test pins all four values × three
layer scenarios.

### Success Criteria

- [ ] `Config.DiffContext` is `*int` (nil ⇒ unset); `Config.TokenLimit` stays plain `int`.
- [ ] `fileGeneration.DiffContext` is `*int`; `fileGeneration.TokenLimit` stays plain `int`.
- [ ] `Defaults()` seeds `DiffContext: intPtr(1)` (non-nil) and `TokenLimit: 0`.
- [ ] `materialize`: `if g.TokenLimit != 0 { c.TokenLimit = g.TokenLimit }` + `if g.DiffContext != nil { c.DiffContext = g.DiffContext }`.
- [ ] `overlay`: `if src.TokenLimit != 0 { dst.TokenLimit = src.TokenLimit }` + `if src.DiffContext != nil { dst.DiffContext = src.DiffContext }`.
- [ ] An explicit `diff_context = 0` in a global OR repo file ⇒ `*cfg.DiffContext == 0` end-to-end (through overlay).
- [ ] An omitted `diff_context` ⇒ `*cfg.DiffContext == 1` (default inherited through overlay).
- [ ] `TestDefaults` asserts `cfg.DiffContext != nil && *cfg.DiffContext == 1` + `cfg.TokenLimit == 0`.
- [ ] The table-driven test passes (unset/1/0/3 × global-only/repo-only/global+repo).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] NO `!= 0` overlay/materialize guard remains on DiffContext (grep-verified).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim load flow proving the contract's overlay guard is
broken, the exact `*int` correction (with the nullable-scalar precedent), the precise edit sites
(config.go/file.go/config_test.go/file_test.go with line numbers), and a complete table-driven test
whose rows PASS under `*int` and FAIL under the literal `!= 0` (the proof the correction is necessary).
The S1 parallel-edit coordination and the S3/future-consumer ripple are spelled out.

### Documentation & References

```yaml
# MUST READ — the contract correction (do NOT use the literal overlay guard)
- docfile: plan/007_b33d310438c6/P1M1T1S2/research/s2_implementation_notes.md
  why: "§1 proves (via the load.go flow + git.go:106) that the contract's `if src.DiffContext != 0` overlay guard silently clobbers an explicit diff_context=0 back to 1 — the contract's own end-to-end test would FAIL. §2 gives the only correct fix: Config.DiffContext = *int (nil = unset). §3 lists the exact edits; §4 the S1 coordination; §5 the S3/future ripple."
  critical: "The #1 failure mode is blindly implementing the contract's `!= 0` overlay guard for DiffContext — the table-driven test's explicit-0 rows will FAIL. DiffContext MUST use *int + `!= nil` in BOTH materialize and overlay. TokenLimit stays plain int + `!= 0` (FR3d: 0 IS unset)."

- docfile: plan/007_b33d310438c6/architecture/diff_capture_touchmap.md
  why: "§3 confirms config fields are FLAT on Config; fileGeneration is the [generation] decode struct; MaxDiffBytes/MaxMdLines is the pair to mirror. The materialize/overlay non-zero convention is documented here."
  critical: "The touchmap's `!= 0` guard is correct for TokenLimit/MaxDiffBytes/MaxMdLines (0 = invalid/unset) but NOT for DiffContext (0 is valid). The *int override is the documented exception."

- docfile: plan/007_b33d310438c6/P1M1T1S1/PRP.md
  why: "S1's contract: adds TokenLimit (plain int) + DiffContext (plain int on BOTH Config and fileGeneration in S1) + Defaults seeds (0, 1). S1's stated INTENT: 'fileGeneration.DiffContext becomes *int in S2 to disambiguate 0 from unset.' S2 extends that *int to Config.DiffContext (S1 scoped it to the file struct only)."
  critical: "S2 RE-EDITS the 3 config.go DiffContext spots S1 adds in parallel (field: int→*int; Defaults: 1→intPtr(1); TestDefaults: ==1 → *==1). See Coordination. TokenLimit is untouched (stays S1's plain int)."

# The files under edit
- file: internal/config/config.go
  why: "EDIT (3 spots). (1) Add `func intPtr(i int) *int { return &i }` next to boolPtr/strPtr (lines 7-9). (2) Config struct: `DiffContext int` → `DiffContext *int` (TokenLimit stays int). (3) Defaults(): `DiffContext: 1,` → `DiffContext: intPtr(1),`."
  pattern: "Mirror the existing nullable-scalar precedent: Output *string + StripCodeFence *bool use *T so overlay's `!= nil` distinguishes unset from explicit-zero. DiffContext joins them as *int."
  gotcha: "TokenLimit stays PLAIN int (FR3d: 0 = unset). Only DiffContext becomes *int. Defaults() DiffContext MUST be intPtr(1) (non-nil) so 'unset' (nil) is distinguishable from the real default 1."

- file: internal/config/file.go
  why: "EDIT (3 spots). (1) fileGeneration.DiffContext: int → *int (TokenLimit stays int). (2) materialize (~line 212, next to MaxDiffBytes/MaxMdLines): add TokenLimit `!= 0` + DiffContext `!= nil`. (3) overlay (~line 314, next to MaxDiffBytes/MaxMdLines): mirror — TokenLimit `!= 0` + DiffContext `!= nil`."
  pattern: "materialize's `c` starts fresh (`&Config{Timeout: timeout}`) so c.DiffContext is nil until the guard sets it; overlay merges Config→Config. The DiffContext guard is `!= nil` in BOTH (NOT `!= 0`)."
  gotcha: "Do NOT use `if src.DiffContext != 0` in overlay — that is the broken contract guard. Use `if src.DiffContext != nil`. (The materialize doc comment 'a file cannot override a field to its zero value' is the v1 limitation DiffContext now escapes via *int.)"

- file: internal/config/config_test.go
  why: "EDIT (1 assertion). TestDefaults DiffContext assertion: `c.DiffContext != 1` → `c.DiffContext == nil || *c.DiffContext != 1` (nil-safe deref). TokenLimit assertion `c.TokenLimit != 0` unchanged."
- file: internal/config/file_test.go
  why: "EDIT (new table-driven test). Add TestMaterializeOverlay_DiffContext_TokenLimit: unset⇒1, explicit 1⇒1, explicit 0⇒0, explicit 3⇒3, across global-only/repo-only/global+repo overlay. Reuses the existing `overlay(&dst, src)` + materialize-via-loadTOML test idiom."

# Read-only refs (do NOT edit in S2)
- file: internal/config/load.go
  why: "READ-ONLY. Lines 82/100/123/138 are the proof overlay is in every layer's path. S2 does NOT edit load.go — the *int design works WITH the existing overlay-based flow (that's the point)."
- file: internal/config/git.go
  why: "READ-ONLY (S3 owns it). Line 106 'designed for NON-ZERO overlay' confirms the git-config layer also uses overlay. S3 must set `c.DiffContext = intPtr(v)` (nil when absent) to participate in the *int semantics — S2's *int design is the contract S3 follows."

# PRD authority
- prd: PRD.md §9.1 FR3d (token_limit, 0=unset ⇒ legacy caps) + FR3f (diff_context 0–3, 0=changed-lines-only, 1=-U1 default); §16.1 layer-1 defaults (token_limit 0, diff_context 1).
  why: "FR3f is WHY 0 must be configurable (it's a meaningful value). FR3d is WHY TokenLimit can stay plain int (0 = unset, no meaningful explicit-0)."
```

### Current Codebase Tree (relevant slice)

```bash
stagehand/
└── internal/config/
    ├── config.go        # EDIT: intPtr helper + Config.DiffContext → *int + Defaults intPtr(1)
    ├── config_test.go   # EDIT: TestDefaults DiffContext assertion → nil-safe *==1
    ├── file.go          # EDIT: fileGeneration.DiffContext → *int; materialize +overlay guards
    ├── file_test.go     # EDIT: + table-driven materialize/overlay test
    ├── load.go          # READ-ONLY — the overlay flow (proof; unchanged)
    └── git.go           # READ-ONLY (S3) — git-config layer; must follow the *int contract
```

### Desired Codebase Tree After S2

```bash
stagehand/
└── (only existing files modified — no new files)
    internal/config/config.go        # intPtr + Config.DiffContext *int + Defaults intPtr(1)
    internal/config/config_test.go   # TestDefaults nil-safe DiffContext assertion
    internal/config/file.go          # fileGeneration.DiffContext *int; materialize + overlay guards
    internal/config/file_test.go     # + TestMaterializeOverlay_DiffContext_TokenLimit
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/config/config.go` | MODIFY | `intPtr` helper; `Config.DiffContext` → `*int`; `Defaults()` → `intPtr(1)`. **Re-edits S1's plain-int DiffContext (see Coordination).** |
| `internal/config/config_test.go` | MODIFY | `TestDefaults` DiffContext assertion → `*DiffContext == 1` nil-safe. |
| `internal/config/file.go` | MODIFY | `fileGeneration.DiffContext` → `*int`; `materialize` + `overlay` TokenLimit (`!= 0`) + DiffContext (`!= nil`) guards. |
| `internal/config/file_test.go` | MODIFY | + table-driven test (4 values × 3 layer scenarios). |

**Explicitly NOT touched**: `load.go` (the flow is correct; `*int` works with it), `git.go` (S3 — but
S2's `*int` is the contract S3 follows), bootstrap template + `docs/CONFIGURATION.md` (S4),
`StagedDiffOptions`/6 call sites (P1.M1.T2), the diff functions (P1.M2+), `PRD.md`, `tasks.json`,
`prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (contract correction): do NOT use `if src.DiffContext != 0` in overlay. overlay() is between
// EVERY layer and the final config (load.go:82 Defaults → :100/:123/:138 overlay). With `!= 0`, an
// explicit diff_context=0 fails the guard (0 != 0 is false) and is silently clobbered to the default 1.
// The contract's own end-to-end test ("explicit 0⇒0") FAILS. Config.DiffContext MUST be *int and the
// guard MUST be `!= nil` in BOTH materialize and overlay. (TokenLimit is the exception: 0 IS its unset
// sentinel per FR3d, so plain int + `!= 0` is correct for TokenLimit everywhere.)

// CRITICAL (S1 boundary override): S1 adds Config.DiffContext as a PLAIN int. S2 changes it to *int.
// This re-edits S1's config.go (field + Defaults + TestDefaults). It is MANDATORY for correctness —
// S1's stated 0-vs-unset intent is unachievable with plain int on Config. See Coordination for the
// parallel-edit reconciliation. TokenLimit is NOT changed (stays S1's plain int).

// CRITICAL (Defaults must be non-nil): Defaults() MUST seed `DiffContext: intPtr(1)` (non-nil). If it
// seeded nil, "unset" (nil) and "default 1" would be indistinguishable and the -U1 default would never
// apply. The non-nil default is what makes nil mean "user omitted the key" vs the real -U1 value.

// GOTCHA (nullable-scalar precedent): Output *string (config.go:88) + StripCodeFence *bool (:97) are
// already *T so overlay's `!= nil` distinguishes unset from explicit-zero. DiffContext joins them. The
// intPtr helper mirrors boolPtr/strPtr (lines 7-9).

// GOTCHA (materialize's fresh config): materialize starts `c := &Config{Timeout: timeout}` — c.DiffContext
// is nil until the guard copies g.DiffContext (also nil if the file omits the key). overlay then sees
// nil ⇒ inherit lower layer. Correct.

// GOTCHA (S3 ripple): git.go's loadGitConfig (S3) currently returns a Config "designed for NON-ZERO
// overlay" (all fields zero). For DiffContext it MUST instead set c.DiffContext = intPtr(v) when
// stagehand.diffContext is found (nil when absent). S2 does NOT edit git.go — but S2's *int design is
// the contract S3 implements. Flag this in the S3 handoff.
```

## Implementation Blueprint

### Data models and structure

No new types. Two field-merge guards per function + one nullable-scalar correction. The relevant existing
precedents (the models to mirror):

```go
// config.go — the nullable-scalar precedent DiffContext joins (EXISTING — unchanged)
Output         *string `toml:"output"`           // *string so overlay `!= nil` distinguishes unset from ""
StripCodeFence *bool   `toml:"strip_code_fence"` // *bool so overlay `!= nil` distinguishes unset from false

// config.go — helpers (EXISTING boolPtr/strPtr; S2 ADDS intPtr)
func boolPtr(b bool) *bool { return &b }
func strPtr(s string) *string { return &s }
// S2 adds:
func intPtr(i int) *int { return &i }
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: config.go — intPtr helper + Config.DiffContext → *int + Defaults intPtr(1)
  - ADD (next to boolPtr/strPtr, lines 7-9): `func intPtr(i int) *int { return &i }`
  - CONFIG STRUCT: change `DiffContext int \`toml:"diff_context"\`` →
        DiffContext *int `toml:"diff_context"` // FR3f reduced context (0–3); *int — nil ⇒ unset (default 1/-U1); non-nil incl. *0 ⇒ explicit (0 = changed-lines-only). *int not plain int so overlay distinguishes unset from explicit 0.
    - KEEP `TokenLimit int` PLAIN (FR3d: 0 = unset).
  - DEFAULTS(): change `DiffContext: 1,` → `DiffContext: intPtr(1), // FR3f -U1 default (non-nil: nil ⇒ user omitted the key)`.
    - KEEP `TokenLimit: 0,` unchanged.
  - WHY *int: see the research note §1–§2 (the contract's `!= 0` overlay guard is broken; *int is the only fix).

Task 2: config_test.go — TestDefaults nil-safe DiffContext assertion
  - LOCATE TestDefaults (S1 added `c.DiffLimit != 0` / `c.DiffContext != 1`).
  - CHANGE the DiffContext assertion to: `if c.DiffContext == nil || *c.DiffContext != 1 { t.Errorf("DiffContext = %v, want non-nil *1 (-U1 default)", c.DiffContext) }`.
  - KEEP the TokenLimit assertion `c.TokenLimit != 0` unchanged.

Task 3: file.go — fileGeneration.DiffContext → *int
  - LOCATE the fileGeneration struct (S1 added TokenLimit int + DiffContext int).
  - CHANGE `DiffContext int \`toml:"diff_context"\`` → `DiffContext *int \`toml:"diff_context"\` // FR3f — *int (0-vs-unset); nil ⇒ user omitted`.
  - KEEP `TokenLimit int` PLAIN.

Task 4: file.go — materialize() guards (next to MaxDiffBytes/MaxMdLines, ~line 212-216)
  - ADD (after the MaxMdLines guard):
        if g.TokenLimit != 0 {
            c.TokenLimit = g.TokenLimit
        }
        if g.DiffContext != nil {
            c.DiffContext = g.DiffContext // *int: nil ⇒ unset; non-nil (incl. *0) ⇒ override
        }
  - GUARD: DiffContext uses `!= nil` (NOT `!= 0`); TokenLimit uses `!= 0`.

Task 5: file.go — overlay() guards (next to MaxDiffBytes/MaxMdLines, ~line 314-318)
  - ADD (after the MaxMdLines guard):
        if src.TokenLimit != 0 {
            dst.TokenLimit = src.TokenLimit
        }
        if src.DiffContext != nil {
            dst.DiffContext = src.DiffContext
        }
  - GUARD: DiffContext uses `!= nil` (NOT `!= 0` — the contract's `!= 0` is the bug being fixed);
    TokenLimit uses `!= 0`.

Task 6: file_test.go — table-driven test (the contract's required verification)
  - ADD TestMaterializeOverlay_DiffContext_TokenLimit (reuses the existing overlay(&dst,src) idiom;
    materialize via loadTOML or a direct fileGeneration). Table rows (DiffContext): unset⇒*1, 1⇒*1,
    0⇒*0, 3⇒*3. Scenarios: (a) materialize-only (file → Config); (b) global-only (Defaults → overlay
    global); (c) repo-only (Defaults → overlay repo); (d) global+repo (Defaults → overlay global →
    overlay repo, incl. repo-explicit-0 overrides global-3, and repo-unset inherits global-3).
  - ALSO assert TokenLimit propagation: unset⇒0, 120000⇒120000 (plain int, `!= 0`).
  - The explicit-0 rows PASS under *int and would FAIL under the contract's literal `!= 0` overlay
    (the proof the correction is necessary).

Task 7: VALIDATE
  - RUN: gofmt -w internal/config/{config,config_test,file,file_test}.go
  - RUN: go build ./... ; go vet ./... ; go test -race ./...
  - GREP: confirm NO `DiffContext != 0` / `DiffContext) != 0` overlay/materialize guard remains:
        grep -n "DiffContext != 0\|DiffContext) != 0" internal/config/file.go   # → ZERO matches
  - FIX-FORWARD: if the explicit-0 test row fails, the overlay guard is still `!= 0` — fix to `!= nil`.
```

### Implementation Patterns & Key Details

```go
// === config.go — the intPtr helper + the *int field + the non-nil default ===
func boolPtr(b bool) *bool { return &b }
func strPtr(s string) *string { return &s }
func intPtr(i int) *int { return &i }   // NEW (S2) — mirrors boolPtr/strPtr

// Config struct (TokenLimit plain int; DiffContext *int):
	TokenLimit          int   `toml:"token_limit"`    // FR3d holistic token cap (0 = unset ⇒ legacy caps)
	DiffContext         *int  `toml:"diff_context"`   // FR3f reduced context (0–3); *int — nil ⇒ unset (default 1); *0 ⇒ changed-lines-only

// Defaults():
		TokenLimit:          0,         // FR3d: 0 = unset ⇒ legacy caps
		DiffContext:         intPtr(1), // FR3f: -U1 default (non-nil: nil ⇒ user omitted the key)
```

```go
// === file.go — materialize + overlay guards (DiffContext uses != nil, NOT != 0) ===
// materialize (next to MaxDiffBytes/MaxMdLines):
	if g.TokenLimit != 0 {
		c.TokenLimit = g.TokenLimit
	}
	if g.DiffContext != nil {
		c.DiffContext = g.DiffContext // *int: nil ⇒ unset; non-nil (incl. *0) ⇒ override
	}
// overlay (next to MaxDiffBytes/MaxMdLines):
	if src.TokenLimit != 0 {
		dst.TokenLimit = src.TokenLimit
	}
	if src.DiffContext != nil {
		dst.DiffContext = src.DiffContext // *int: nil ⇒ inherit lower layer; non-nil ⇒ override (incl. *0)
	}
```

```go
// === file_test.go — the table-driven proof (passes under *int; the explicit-0 row is the key) ===
// Scenario: Defaults() (DiffContext=intPtr(1)) → overlay(&cfg, materialize(file)).
//   file omits diff_context  → materialize DiffContext=nil  → overlay skips → cfg *1  ✅
//   file diff_context = 0    → materialize DiffContext=*0   → overlay copies → cfg *0  ✅ (the contract's end-to-end test)
//   file diff_context = 3    → materialize DiffContext=*3   → overlay copies → cfg *3  ✅
// global+repo: Defaults → overlay(global=3) → *3; overlay(repo=0) → *0 (repo explicit-0 wins) ✅
//              Defaults → overlay(global=3) → *3; overlay(repo unset) → *3 (repo inherits global) ✅
```

### Integration Points

```yaml
CONFIG SCHEMA (internal/config/config.go):
  - Config.DiffContext: int → *int (nil ⇒ unset); TokenLimit: plain int (unchanged)
  - Defaults(): DiffContext intPtr(1) (non-nil); TokenLimit 0
  - helper: intPtr (new), mirroring boolPtr/strPtr

FILE DECODE + MERGE (internal/config/file.go):
  - fileGeneration.DiffContext: int → *int; TokenLimit: plain int
  - materialize: + TokenLimit `!= 0` + DiffContext `!= nil`
  - overlay:     + TokenLimit `!= 0` + DiffContext `!= nil`   # NOT `!= 0` for DiffContext (the contract bug)

TESTS:
  - config_test.go TestDefaults: DiffContext nil-safe *==1; TokenLimit ==0
  - file_test.go: + table-driven materialize/overlay test (unset/1/0/3 × global/repo/global+repo)

COORDINATION (S1 parallel edit — IMPORTANT):
  - S2 re-edits config.go's 3 DiffContext spots S1 adds (field int→*int; Defaults 1→intPtr(1); TestDefaults ==1→*==1).
  - Preferred: fold the *int into S1 (S1 ships Config.DiffContext as *int directly — a one-field refinement
    that makes S1's stated 0-vs-unset intent work). Fallback: S2 re-edits after S1 lands (orchestrator
    sequences S2 after S1 to avoid a merge conflict on the DiffContext field line).

NO-TOUCH (explicitly):
  - internal/config/load.go           # the flow is correct; *int works WITH it (that's the point)
  - internal/config/git.go            # S3 — but S2's *int is the contract S3 implements (see ripple)
  - bootstrap.go + docs/CONFIGURATION.md   # S4
  - StagedDiffOptions + 6 call sites  # P1.M1.T2
  - the diff functions (-U<diff_context>)  # P1.M2+
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S2):
  - S3 (git.go): set c.DiffContext = intPtr(v) when stagehand.diffContext found (nil when absent) — the *int contract.
  - P1.M1.T2/P1.M2 (consumers): deref *cfg.DiffContext (dc := 1; if cfg.DiffContext != nil { dc = *cfg.DiffContext }).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand

gofmt -w internal/config/config.go internal/config/config_test.go internal/config/file.go internal/config/file_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/config/...     # Expected: exit 0
go build ./...                   # Expected: exit 0
```

### Level 2: Unit Tests — the table-driven proof (the contract's required verification)

```bash
cd /home/dustin/projects/stagehand

# The new materialize/overlay test — the explicit-0 row is the load-bearing assertion
go test -race -run 'TestMaterializeOverlay_DiffContext_TokenLimit|TestDefaults' ./internal/config/ -v

# Full config suite
go test -race ./internal/config/ -v

# Expected: ALL PASS. The explicit diff_context=0 rows yield *cfg.DiffContext == 0 through the overlay
# chain (global-only, repo-only, global+repo). Under the contract's literal `!= 0` overlay these would
# FAIL (clobbered to *1) — the test passing is the proof the *int correction is correct.
```

### Level 3: Whole-Repository Regression

```bash
cd /home/dustin/projects/stagehand

go test -race ./...              # Expected: ALL packages green
go vet ./...                     # Expected: exit 0

# Confirm NO broken `!= 0` overlay/materialize guard remains on DiffContext
grep -n "DiffContext != 0\|DiffContext) != 0" internal/config/file.go   # Expected: ZERO matches
grep -n "DiffContext != nil" internal/config/file.go                    # Expected: 2 matches (materialize + overlay)

# Confirm ONLY the 4 intended config files changed
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/config/{config,config_test,file,file_test}.go only.
```

### Level 4: End-to-End Behavior Smoke (the contract's "verify" clause)

```bash
cd /home/dustin/projects/stagehand

# Inline proof via a throwaway in-package test (delete after) — proves diff_context=0 survives overlay:
cat > internal/config/zz_smoke_test.go <<'EOF'
package config
import "testing"
func TestZZ_DiffContextZeroSmoke(t *testing.T) {
	cfg := Defaults()                       // DiffContext = intPtr(1)
	g := materialize(&fileConfig{Generation: fileGeneration{DiffContext: intPtr(0)}}, 0)
	overlay(&cfg, g)                        // the load.go step that the contract's guard broke
	if cfg.DiffContext == nil || *cfg.DiffContext != 0 {
		t.Fatalf("explicit diff_context=0 lost through overlay: got %v", cfg.DiffContext)
	}
	t.Log("explicit diff_context=0 ⇒ *cfg.DiffContext == 0 (end-to-end through overlay) ✅")
}
EOF
go test -run TestZZ_DiffContextZeroSmoke -v ./internal/config/ ; rm -f internal/config/zz_smoke_test.go
# Expected: PASS (got *0). Under the contract's literal `!= 0` overlay this prints got 0x... / *1 → FAIL.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green (incl. the table-driven materialize/overlay test).
- [ ] `grep "DiffContext != 0" internal/config/file.go` → ZERO; `grep "DiffContext != nil"` → 2 (materialize + overlay).

### Feature Validation

- [ ] `Config.DiffContext` is `*int`; `fileGeneration.DiffContext` is `*int`; `TokenLimit` stays plain `int` on both.
- [ ] `Defaults()` seeds `DiffContext: intPtr(1)` (non-nil) + `TokenLimit: 0`.
- [ ] `materialize` + `overlay` use `!= nil` for DiffContext and `!= 0` for TokenLimit.
- [ ] An explicit `diff_context = 0` in a global OR repo file ⇒ `*cfg.DiffContext == 0` end-to-end (through overlay).
- [ ] An omitted `diff_context` ⇒ `*cfg.DiffContext == 1` (default inherited).
- [ ] The table-driven test passes (unset/1/0/3 × global-only/repo-only/global+repo).

### Scope Discipline Validation

- [ ] ONLY `internal/config/{config,config_test,file,file_test}.go` modified (git diff --stat confirms).
- [ ] Did NOT edit `load.go` (the flow is correct), `git.go` (S3), bootstrap/docs (S4), consumers (P1.M1.T2/P1.M2+).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] DiffContext `*int` mirrors the `Output *string` / `StripCodeFence *bool` nullable-scalar precedent.
- [ ] `intPtr` helper mirrors `boolPtr`/`strPtr`.
- [ ] The contract-correction (overlay `!= nil`, not `!= 0`) is documented in the code comments + the research note.
- [ ] The S1 parallel-edit coordination and the S3/future-consumer ripple are documented.

---

## Anti-Patterns to Avoid

- ❌ Don't use `if src.DiffContext != 0` in overlay — that is the contract's BROKEN guard. overlay() sits
  between every layer and the final config (load.go:82→100→123→138; git.go:106), so an explicit
  diff_context=0 fails `0 != 0` and is silently clobbered to 1. The contract's own end-to-end test fails.
  Use `if src.DiffContext != nil` with `Config.DiffContext` as `*int`.
- ❌ Don't keep `Config.DiffContext` as plain int "to respect S1's boundary." S1's plain-int scoping makes
  the contract's end-to-end explicit-0 requirement impossible (the non-zero overlay convention cannot
  express "override to 0" for a plain int). The `*int` override is MANDATORY and fulfills S1's stated
  0-vs-unset intent. Document the S1 re-edit; don't silently keep a broken plain int.
- ❌ Don't make `TokenLimit` a `*int`. FR3d: `0` IS TokenLimit's unset sentinel (no meaningful "explicit
  0"). Plain int + `!= 0` in both materialize and overlay is correct (matches MaxDiffBytes/MaxMdLines).
- ❌ Don't seed `Defaults()` with `DiffContext: nil` — that erases the -U1 default (nil would mean "user
  omitted", not "default 1"). Seed `intPtr(1)` (non-nil) so nil stays the "omitted" signal.
- ❌ Don't forget the `intPtr` helper — `Defaults()` needs a non-nil `*int` literal, and `&1` is not a
  valid Go addressable literal in a struct literal context (use `intPtr(1)`, mirroring boolPtr/strPtr).
- ❌ Don't edit `load.go` to "fix" the flow — the flow is correct; the `*int` design works WITH it (that's
  the point of the correction). The bug was the field type + guard, not the flow.
- ❌ Don't edit `git.go` (S3) — but DO ensure S2's `*int` design is the contract S3 follows (S3 must set
  `c.DiffContext = intPtr(v)`). Flag it in the handoff, don't implement it here.
- ❌ Don't write the table-driven test without the explicit-0 row — that row is the proof the `*int`
  correction works AND the proof the contract's literal guard would have failed. It is the load-bearing
  assertion.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

---

## Confidence Score

**8.5/10** for one-pass implementation success.

Rationale: The edits are small and precisely specified (intPtr helper + `*int` field + two `!= nil`
guards + a table-driven test), with the nullable-scalar precedent (`Output *string`/`StripCodeFence *bool`)
already in the codebase. The load-bearing insight — that the contract's `!= 0` overlay guard is broken
(proven via load.go:82→100→123→138 + git.go:106) and the only fix is `Config.DiffContext = *int` — is
documented with a trace and an inline smoke test whose explicit-0 row passes under `*int` and fails under
the literal guard. The residual uncertainty (not 9.5–10) is the **S1 parallel-edit coordination**: S2
re-edits the three config.go DiffContext spots S1 is adding in parallel (field, Defaults, TestDefaults),
so the orchestrator must either fold the `*int` into S1 or sequence S2 after S1 to avoid a merge conflict.
There is also a documented ripple to S3 (git.go must produce `*int`) and future consumers (deref
`*cfg.DiffContext`) — not S2's edits, but contracts S2 establishes. A faithful literal implementation of
the contract's `!= 0` overlay would have guaranteed a failing test; the correction is what makes one-pass
success possible.
