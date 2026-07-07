# PRP — P1.M2.T1.S1: Change `Config.Output` to `*string`; stop defaulting Output/StripCodeFence in `Defaults()`

> **Scope discipline.** This subtask is the **internal config-package plumbing** for PRD Issue 2
> (manifest-level `output`/`strip_code_fence` silently clobbered by `[generation]` defaults). S1 makes
> `Config.Output` a tri-state `*string` and stops seeding both `Output`/`StripCodeFence` in `Defaults()`,
> then fixes the config-package loaders + tests. **The user-facing behavior** (the `buildDeps` bridge
> that copies `cfg.Output`/`cfg.StripCodeFence` onto the manifest ONLY when explicitly set) **is S2's
> scope** (`P1.M2.T1.S2`). Do NOT touch `pkg/stagecoach/stagecoach.go`.
>
> **No external research needed** — this is a mechanical pointer-ification inside the existing Go
> config package, reusing the project's established `*bool`/`boolPtr` pattern. All required context is
> the binding architecture doc + the compiler error list.

---

## Goal

**Feature Goal**: Make `config.Config.Output` a tri-state `*string` (nil ⇒ honor the manifest; non-nil ⇒
override) and make BOTH `Output` and `StripCodeFence` default to `nil` (not "raw"/`true`), so that
`[generation]` becomes a genuine opt-in override instead of an always-on default. This is the
type/default prerequisite for S2's `buildDeps` bridge.

**Deliverable**:
1. `internal/config/config.go`: `Output string` → `Output *string`; add `strPtr`; in `Defaults()`
   remove the `Output:` and `StripCodeFence:` initializers (→ both nil).
2. `internal/config/file.go`: pointer-ify the `materialize` and `overlay` `Output` handling (symmetric
   with the existing `StripCodeFence` pointer handling). `fileGeneration.Output` stays a plain `string`.
3. `internal/config/git.go`: `loadGitConfig` sets `c.Output = &v`.
4. Updated config-package tests that compile and reflect the new nil-default contract.

**Success Definition**: `go build ./internal/config/... && go vet ./internal/config/... &&
go test ./internal/config/...` is GREEN, and `Config.Output` is `*string` with BOTH `Output` and
`StripCodeFence` equal to `nil` in `Defaults()`.

---

## Why

- **PRD Issue 2 / §12.1 / §12.9**: `output` and `strip_code_fence` are **per-manifest** settings, and
  `parseOutput` reads the **manifest's** values. A `[provider.X] output = "json"` (+ `json_field`) must
  be honored without the user also repeating it under `[generation]`.
- **Root cause** (full trace: `architecture/issue_analysis.md` ISSUE 2): the bugfix-001 "Issue 4"
  bridge in `buildDeps` copies `cfg.Output`/`cfg.StripCodeFence` onto the resolved manifest, but its
  guards (`cfg.Output != ""` / `cfg.StripCodeFence != nil`) ALWAYS pass because `Defaults()` seeds
  non-empty/non-nil values. So the manifest's own `output`/`strip_code_fence` are always overwritten →
  manifest-level JSON is dead, `strip_code_fence = false` is ignored, and `providers show` *lies*
  (prints `output = 'json'` while parsing uses `raw`).
- **This subtask's role**: remove the always-on defaults so the S2 bridge's nil-check means what it
  says ("only override when the user explicitly set `[generation]` …"). S1 is pure internal plumbing;
  no user-facing behavior change of its own.

---

## What

### The type change

`Config.Output` becomes `*string` (nil ⇒ no `[generation]` override ⇒ manifest wins; non-nil ⇒ override).
This mirrors the existing `Config.StripCodeFence *bool` model exactly. `Defaults()` stops setting both,
so they are `nil` unless a file/git-config layer explicitly provides them. The manifest's own `Resolve()`
already supplies the §12.1 `raw`/`true` fallbacks (`internal/provider/manifest.go:138-148`,
`DefaultOutput`/`DefaultStripCodeFence`), so removing the config-layer defaults loses nothing.

### Success Criteria

- [ ] `Config.Output` is `*string` (`internal/config/config.go`).
- [ ] `Defaults()` leaves BOTH `Output` and `StripCodeFence` as `nil`.
- [ ] `file.go` `materialize`/`overlay` handle `Output` as a pointer (symmetric with `StripCodeFence`).
- [ ] `git.go` `loadGitConfig` sets `c.Output = &v`.
- [ ] `fileGeneration.Output` remains a plain `string`.
- [ ] `go build ./internal/config/... && go vet ./internal/config/... && go test ./internal/config/...` GREEN.

---

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — exact file:line refs, the before/after code for every edit, the compiler-driven
test-edit list, and the precise (narrowed) validation gate are all below.

### Documentation & References

```yaml
# MUST READ — binding architecture (root cause + blast radius, verified SAFE + mechanical)
- file: plan/001_f1f80943ac34/bugfix/002_18158df10968/architecture/issue_analysis.md
  why: ISSUE 2 — proves Defaults() seeds the always-on defaults that defeat the buildDeps nil-guards;
       lists the exact config.go/file.go/git.go edits and the compiler-driven test-edit sites.
  section: "ISSUE 2 (Major) — ... Fix — Part 1 (config package — type + defaults)" and "Blast radius"

# The type + defaults (EDIT SITE 1)
- file: internal/config/config.go
  why: Config.Output (line 35 string→*string); boolPtr (line 7, add strPtr next to it); Defaults()
       (lines 67-70, remove the Output:"raw" + StripCodeFence:boolPtr(true) initializers).
  pattern: StripCodeFence is ALREADY *bool with a boolPtr helper — Output follows the IDENTICAL pattern.
  gotcha: Do NOT remove other Defaults() entries (Timeout/AutoStageAll/Max*/SubjectTargetChars stay).

# file loaders (EDIT SITES 2 & 3)
- file: internal/config/file.go
  why: materialize (159-160) and overlay (210-211) must pointer-ify Output; the StripCodeFence lines
       (162-163, 213-214) are the exact pointer pattern to mirror and are LEFT UNCHANGED.
  gotcha: fileGeneration.Output (line 41) STAYS a plain string — go-toml decodes it fine; only the
          resolved Config.Output becomes *string.

# git-config loader (EDIT SITE 4)
- file: internal/config/git.go
  why: loadGitConfig (line 127) `c.Output = v` → `c.Output = &v`.
  gotcha: each `if v, found, err := ...` declares a FRESH v (not a loop var), so &v is safe — exactly
          like the existing `c.StripCodeFence = &v` at lines 152-155 (unchanged).

# The manifest-side fallback that makes removing the defaults SAFE
- file: internal/provider/manifest.go
  why: Resolve() (138-148) fills nil Output→DefaultOutput("raw") and nil StripCodeFence→DefaultStripCodeFence
       (true). This is why Defaults() no longer needs to supply them. (READ-ONLY — do not edit.)

# Tests to update (compiler-driven)
- file: internal/config/config_test.go   # TestDefaults (46-47), TestTOMLMarshalKeysAndNoColorExclusion (47-72)
- file: internal/config/file_test.go      # TestLoadTOMLValid (82-83), TestOverlayPartial (114,119-120),
                                          # TestOverlayStripCodeFenceFalse (407)
- file: internal/config/git_test.go       # 111-112, 133, 345-346
```

### Current Codebase tree (relevant slice)

```bash
internal/config/config.go     # Config struct (Output L35), boolPtr (L7), Defaults() (L67-70) — EDIT
internal/config/file.go       # materialize (L159-160), overlay (L210-211) — EDIT; fileGeneration.Output (L41) KEEP string
internal/config/git.go        # loadGitConfig (L127) — EDIT
internal/config/config_test.go # TestDefaults, TestTOMLMarshalKeysAndNoColorExclusion — EDIT
internal/config/file_test.go   # TestLoadTOMLValid, TestOverlayPartial, TestOverlayStripCodeFenceFalse — EDIT
internal/config/git_test.go    # 3 Output assertion sites — EDIT
internal/provider/manifest.go  # Resolve() L138-148 — READ-ONLY (provides the raw/true fallback)
pkg/stagecoach/stagecoach.go     # L206-208 — DO NOT TOUCH (S2 scope; transiently non-compiling after S1)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (scope): S1 edits ONLY internal/config/*. S2 owns pkg/stagecoach/stagecoach.go:206-208.
//   After S1: `go build ./internal/config/...` is GREEN, but `go build ./...` FAILS at
//   pkg/stagecoach/stagecoach.go:206 (`if cfg.Output != ""` on a now-*string). This is EXPECTED and is
//   resolved by S2 (`if cfg.Output != nil { m.Output = cfg.Output }`). Do NOT "fix" pkg/stagecoach here.

// CRITICAL (validation gate): the S1 success gate is scoped to ./internal/config/... — NOT go build ./... .
//   Running `go build ./...` after S1 WILL error at pkg/stagecoach; that is not an S1 failure.

// GOTCHA (go-toml/v2 nil pointers): go-toml OMITS nil *string/*bool keys on marshal (free omitempty) —
//   this is the SAME behavior provider.Registry.MarshalTOML already relies on in prod. So marshaling
//   Defaults() (now nil Output/StripCodeFence) NO LONGER emits `output =`/`strip_code_fence =`. The
//   TestTOMLMarshalKeysAndNoColorExclusion test must marshal a Config with EXPLICIT values (see Task 5).

// GOTCHA (loop-variable safety): in git.go each `if v, found, err := gitConfigGet(...)` is a fresh
//   short-variable declaration, so `&v` is a distinct address each time — NOT the classic loop-var
//   aliasing bug. Mirror the existing `c.StripCodeFence = &v`.
```

---

## Implementation Blueprint

### Data models and structure

No new models. The only structural change is the field type:

```go
// internal/config/config.go
type Config struct {
    // ... unchanged ...
    Output         *string `toml:"output"`           // WAS: string. nil ⇒ honor manifest (S2 bridge); non-nil ⇒ override
    StripCodeFence *bool   `toml:"strip_code_fence"` // UNCHANGED type; only its Defaults() initializer is removed
    // ... unchanged ...
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/config/config.go
  - EDIT field: `Output string` → `Output *string` (struct, ~line 35).
  - ADD helper: `func strPtr(s string) *string { return &s }` immediately under the existing
    `func boolPtr(b bool) *bool { return &b }` (line 7).
  - EDIT Defaults() (~lines 68-69): DELETE the two lines `Output: "raw",` and
    `StripCodeFence: boolPtr(true),`. Leave every other initializer (Timeout, AutoStageAll, Max*,
    SubjectTargetChars, Provider/Model "") untouched. Update the Defaults() doc comment: change the
    "output \"raw\", strip_code_fence true" clause to note these are now nil (deferred to the manifest).
  - NAMING: `strPtr` mirrors `boolPtr` (same package, same style).
  - GUARDRAIL: do NOT change StripCodeFence's TYPE (it is already *bool); only remove its initializer.

Task 2: MODIFY internal/config/file.go :: materialize (~159-160)
  - EDIT: replace
        if g.Output != "" {
            c.Output = g.Output
        }
    with
        if g.Output != "" {
            o := g.Output
            c.Output = &o
        }
  - WHY local copy: g.Output is a string on the fileGeneration value; c.Output is now *string.
    (fileGeneration.Output at line 41 STAYS string — do not change it.)
  - MIRROR: the immediately-following StripCodeFence block (162-163) is the reference; leave it unchanged.

Task 3: MODIFY internal/config/file.go :: overlay (~210-211)
  - EDIT: replace
        if src.Output != "" {
            dst.Output = src.Output
        }
    with
        if src.Output != nil {
            dst.Output = src.Output
        }
  - WHY: src.Output is now *string; pointer copy is symmetric with the StripCodeFence overlay (213-214).

Task 4: MODIFY internal/config/git.go :: loadGitConfig (~line 127)
  - EDIT: in the `stagecoach.output` block, replace `c.Output = v` with `c.Output = &v`.
  - SAFETY: v is a fresh per-if local; &v is distinct each time (same as c.StripCodeFence = &v at 152-155).
  - GUARDRAIL: the StripCodeFence block (152-155) is unchanged.

Task 5: MODIFY internal/config/config_test.go
  - TestDefaults (46-47): replace the two Output/StripCodeFence assertions with nil assertions:
        if c.Output != nil { t.Errorf("Output = %v, want nil", c.Output) }
        if c.StripCodeFence != nil { t.Errorf("StripCodeFence = %v, want nil", c.StripCodeFence) }
  - TestTOMLMarshalKeysAndNoColorExclusion (47-72): marshaling Defaults() no longer emits output/
    strip_code_fence (nil pointers are omitted by go-toml). Marshal a Config with EXPLICIT values so the
    key-presence assertions still validate the toml tags:
        c := Defaults()
        c.Output = strPtr("raw")
        c.StripCodeFence = boolPtr(true)
        data, err := toml.Marshal(c)
    Keep the full key-list loop (provider/model/timeout/.../output/strip_code_fence) and the NoColor
    exclusion sub-assertion unchanged.

Task 6: MODIFY internal/config/file_test.go
  - TestLoadTOMLValid (82-83): `if cfg.Output != "json"` → `if cfg.Output == nil || *cfg.Output != "json"`.
  - TestOverlayPartial (114): struct literal `Output: "json"` → `Output: strPtr("json")`.
  - TestOverlayPartial (119-120): `if dst.Output != "json"` → `if dst.Output == nil || *dst.Output != "json"`.
  - TestOverlayPartial StripCodeFence assertion: Defaults() now leaves it nil and src (Timeout+Output)
    doesn't set it, so after overlay dst.StripCodeFence is nil — change the assertion to assert nil
    (the untouched default): `if dst.StripCodeFence != nil { t.Errorf("... want nil") }`.
  - TestOverlayStripCodeFenceFalse Case 2 (407): `src = &Config{Output: "json"}` → `Output: strPtr("json")`.
    Preserve the test's INTENT ("nil src must not clobber") by pre-setting the dst default explicitly
    before overlay: `dst := Defaults(); dst.StripCodeFence = boolPtr(true)` then overlay src (nil SCF),
    then assert `*dst.StripCodeFence == true`. (This keeps the "nil src must not clobber a set dst"
    property meaningful now that Defaults() no longer sets it.)
    NOTE: Case 1 of the same test (src sets &false) needs NO logic change — overlay still copies &false;
    only verify the leading comment "dst := Defaults() // StripCodeFence = true" is updated if misleading.

Task 7: MODIFY internal/config/git_test.go
  - (111-112): `if cfg.Output != "json"` → `if cfg.Output == nil || *cfg.Output != "json"`.
  - (133) TestLoadGitConfig_MissingKeysIgnored: `cfg.Output != ""` → `cfg.Output != nil` (a missing key
    now leaves Output nil).
  - (345-346) the "default preserved" assertion: `if cfg.Output != "raw"` → `if cfg.Output != nil`
    (want nil — the default is now nil, not "raw"); update the message to "want nil (default preserved)".
```

### Implementation Patterns & Key Details

```go
// PATTERN: tri-state pointer field with a *Ptr helper — the EXACT pattern Config.StripCodeFence already
// follows. Output becomes its twin:
//   - type: *string
//   - helper: strPtr(s) (twin of boolPtr(b))
//   - Defaults(): nil (unset ⇒ defer to the manifest's Resolve())
//   - loaders: set only when the source explicitly provides a value (materialize/overlay/loadGitConfig)

// GOTCHA: non-zero overlay semantics mean a file STILL cannot set Output to "" (empty string is the
// fileGeneration.Output zero value → skipped). That v1 limitation is pre-existing and out of scope; the
// git-config loader CAN set any value. Do not try to "fix" the empty-string-skip here.
```

### Integration Points

```yaml
CODE (config package): the 4 edit sites above (config.go, file.go ×2, git.go) + 3 test files.
DATABASE: none.
CONFIG: none (no new config keys; the [generation] output/strip_code_fence TOML keys are unchanged).
ROUTES: none.
DOWNSTREAM (NOT this task): pkg/stagecoach/stagecoach.go buildDeps (L206-208) is S2's bridge — it will be
  changed to `if cfg.Output != nil { m.Output = cfg.Output }` (drop the local copy). S1 must NOT touch it.
```

---

## Validation Loop

> **The S1 gate is scoped to `./internal/config/...`.** `go build ./...` (whole repo) is EXPECTED to
> fail at `pkg/stagecoach/stagecoach.go:206` until S2 lands — that is NOT an S1 failure.

### Level 1: Syntax & Type (run after Tasks 1-4)

```bash
# Build + vet the config package ONLY. Expected: clean.
go build ./internal/config/...
go vet ./internal/config/...

# If there are compile errors INSIDE internal/config, follow them (every cfg.Output != "" / Output: "..."
# site is a pointer edit). If the only error is pkg/stagecoach/stagecoach.go:206, that is EXPECTED — stop.
```

### Level 2: Config-package tests (run after Tasks 5-7)

```bash
# All config tests must pass with the new nil-default contract.
go test ./internal/config/...
# Expected: PASS. Specifically:
#   TestDefaults ................................ Output==nil AND StripCodeFence==nil
#   TestTOMLMarshalKeysAndNoColorExclusion ...... output=/strip_code_fence= present (explicit marshal)
#   TestLoadTOMLValid ........................... cfg.Output == strPtr("json") equivalent
#   TestOverlayPartial .......................... dst.Output deref == "json"; SCF untouched (nil)
#   TestOverlayStripCodeFenceFalse .............. nil src does not clobber a set dst
#   TestLoadGitConfig_* ......................... deref / != nil / nil-default-preserved
```

### Level 3: Confirm the transient repo-wide break is ONLY at the S2 seam (sanity, not a fix)

```bash
# OPTIONAL sanity check: confirm the ONLY whole-repo compile break is the S2 site.
go build ./... 2>&1 | grep -v '^#' | sed 's/:.*//;s/^# //' | sort -u
# Expected: the only failing file is pkg/stagecoach/stagecoach.go (and only the cfg.Output site at ~206).
# If ANY internal/config file appears here, that is a real S1 bug — fix it. Do NOT edit pkg/stagecoach.
```

### Level 4: Behavioral spot-check of the new default (proves the plumbing)

```bash
# Quick programmatic proof that Defaults() now leaves both fields nil.
go test ./internal/config/... -run TestDefaults -v
# Expected log: TestDefaults PASS (asserts Output==nil, StripCodeFence==nil).
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./internal/config/...` clean.
- [ ] `go vet ./internal/config/...` clean.
- [ ] `gofmt -l internal/config/` reports nothing (run `gofmt -w` if it does).
- [ ] `go test ./internal/config/...` — all PASS with the new nil-default contract.
- [ ] (Sanity) the only whole-repo compile break is `pkg/stagecoach/stagecoach.go` (the S2 seam).

### Feature Validation
- [ ] `Config.Output` is `*string`; `Config.StripCodeFence` is unchanged (`*bool`).
- [ ] `Defaults()` returns `Output == nil` AND `StripCodeFence == nil`.
- [ ] `fileGeneration.Output` is still a plain `string` (unchanged).
- [ ] `materialize`/`overlay` treat `Output` as a pointer, symmetric with `StripCodeFence`.
- [ ] `loadGitConfig` sets `c.Output = &v` only when the key is found.

### Code Quality Validation
- [ ] Reuses the existing `boolPtr`/`*bool` pattern (`strPtr` is its twin) — no new pattern invented.
- [ ] No edits outside `internal/config/` (pkg/stagecoach untouched — it is S2's scope).
- [ ] Test edits preserve each test's original INTENT (partial-overlay, nil-src-no-clobber,
      default-preserved), updated only for the new nil default.
- [ ] No silent behavior change for installed providers (the manifest's Resolve() still yields raw/true).

### Documentation
- [ ] Defaults() doc comment updated to note Output/StripCodeFence are now nil (deferred to manifest).
- [ ] (No user-facing docs in S1 — the `[generation]`/manifest-semantics doc update rides with S2, per
      the contract's DOCS note: "internal type change with no user-facing/config surface change of its own".)

---

## Anti-Patterns to Avoid

- ❌ **Don't run/require `go build ./...` as the S1 gate** — it will fail at `pkg/stagecoach` (S2 seam).
  The gate is `./internal/config/...`.
- ❌ **Don't edit `pkg/stagecoach/stagecoach.go`** to "make it compile" — that is S2's scope; doing it here
  duplicates S2 and breaks the task boundary.
- ❌ **Don't change `Config.StripCodeFence`'s TYPE** — it is already `*bool`; only remove its
  `Defaults()` initializer.
- ❌ **Don't change `fileGeneration.Output` to a pointer** — keep it `string` (go-toml decodes it fine;
  only the resolved `Config.Output` is a pointer).
- ❌ **Don't use a loop-variable address** — in git.go each `v` is a fresh `if`-local; `&v` is safe and
  distinct (mirror the existing `c.StripCodeFence = &v`).
- ❌ **Don't drop the `TestTOMLMarshalKeysAndNoColorExclusion` output/strip_code_fence key checks
  silently** — marshal a Config with explicit `strPtr`/`boolPtr` values so the toml-tag coverage stays.
- ❌ **Don't try to fix the file-loader empty-string-skip** (`if g.Output != ""`) — that v1 non-zero
  overlay limitation is pre-existing and out of scope.

---

## Confidence Score

**9/10** — A mechanical, compiler-driven pointer-ification that mirrors an existing, in-package pattern
(`StripCodeFence`/`boolPtr`), with every edit site and test update enumerated and verified against the
current source. The -1 reserves for the `TestOverlayStripCodeFenceFalse` Case 2 intent-preservation
judgment call (pre-set `dst.StripCodeFence = boolPtr(true)` to keep "nil src must not clobber"
meaningful), which is spelled out explicitly. The one cross-task hazard (the transient
`pkg/stagecoach` compile break) is flagged prominently so the implementer does not over-reach into S2.
