---
name: "P1.M1.T2.S1 — Add RenderMode type + update Render signature with variadic mode parameter"
description: |
  Extend `internal/provider/render.go`: add an exported `RenderMode` type (`RenderBare`/`RenderTooled`),
  change `Manifest.Render` to a variadic `mode ...RenderMode` final parameter (default bare), and replace
  the unconditional `args = append(args, r.BareFlags...)` with a mode switch that appends TooledFlags in
  tooled mode (erroring if TooledFlags is empty). The variadic keeps every v1 caller (generate.go,
  stagecoach.go, 16+ test sites) byte-for-byte unchanged. This lands the render-side hook the v2
  decompose stager (P3.M2.T3) will call as `Render(model, provider, sys, payload, provider.RenderTooled)`.
  Pure additive change to ONE production file + its test file. S1 (TooledFlags field) is already landed.
---

## Goal

**Feature Goal**: Give `Manifest.Render` an optional, type-safe mode selector so a single render entry
point can produce EITHER a bare (tools-off) command for the planner/message/arbiter roles OR a tooled
(git-tools-on) command for the stager role — selected by the last variadic argument — without breaking
any existing caller and without duplicating the rendering algorithm.

**Deliverable** (ONE production file + its test file, both in `internal/provider/`):
1. `render.go` — NEW exported `type RenderMode string` + constants `RenderBare`/`RenderTooled`; `Render`
   signature gains a trailing `mode ...RenderMode`; the single `args = append(args, r.BareFlags...)`
   line becomes a mode switch (bare → BareFlags; tooled → TooledFlags, error if empty); the Render doc
   comment + token-order comment updated. ~25 added lines.
2. `render_test.go` — ~5 new focused tests (default-is-bare, explicit-bare, tooled-appends-tooled-flags,
   tooled-empty-errors, golden-providers-still-bare) reusing the existing table/`containsPair`/`strPtr`
   patterns + one small `dualModeManifest()` fixture.

**Success Definition**: After S1, `Render(model, provider, sys, user)` (no mode) produces byte-identical
`*CmdSpec` to before (all 10 existing render tests + all v1 callers unchanged); `Render(..., RenderBare)`
is identical to the no-mode call; `Render(..., RenderTooled)` on a manifest with non-empty TooledFlags
appends TooledFlags instead of BareFlags; `Render(..., RenderTooled)` on a manifest with nil/empty
TooledFlags returns a non-nil error containing the provider name and "tooled mode requires non-empty
tooled_flags". `go build/vet/gofmt` clean; `go test -race ./...` green across the WHOLE repo.

## User Persona

**Target User**: The Stagecoach contributor implementing the v2 decompose pipeline (P3.M2.T3 stager) and
the provider/tooled-flags builtins (P1.M2.T2). This is internal rendering plumbing — no end-user surface.

**Use Case**: The decompose stager (P3.M2.T3) must invoke an agent WITH git tools enabled (tooled mode)
to stage per-concept hunks, while the planner/message/arbiter and the entire v1 single-commit pipeline
keep calling Render in bare mode. One Render method, one mode argument, selects the flag-set.

**User Journey**: `spec, err := manifest.Render(model, provider, sys, payload, provider.RenderTooled)`
→ executor runs the tooled CmdSpec. The v1 path `manifest.Render(model, provider, sys, payload)` is
untouched and still bare.

**Pain Points Addressed**: Avoids duplicating the §12.2 rendering algorithm into a second "tooled
renderer"; gives the stager a typed mode constant instead of a magic string; makes "this provider can't
stager" fail at render time (clear error) rather than silently emitting a tool-less command.

## Why

- **PRD §12.2 is the binding algorithm:** `args += (mode == "tooled") ? m.tooled_flags : m.bare_flags`.
  The v1 renderer hardcodes the `bare_flags` branch; this subtask implements the ternary's `mode`
  dimension so the same code path serves both modes.
- **PRD §11.5 (two invocation modes):** bare (tools off) for planner/message/arbiter; tooled (git tools
  on, scoped) for the stager — "the single deliberate exception to stagecoach's 'agent never touches git'
  rule." Both reuse the manifest's command/model/provider/print/delivery fields; only the flag-set differs.
  Render is where that flag-set selection happens.
- **PRD §12.1 tooled_flags contract:** "nil/empty => this provider does not support tooled mode and
  cannot serve as a stager." Render enforces this at render time (tooled + empty TooledFlags → error),
  so a misconfigured stager role fails fast with a named-provider message instead of producing a
  tool-less command that silently does the wrong thing.
- **Unblocks P3.M2.T3 (stager) and P1.M2.T2 (tooled-flags builtins):** the stager's tooled invocation
  has no path until Render accepts a mode; the pi/claude tooled-flags values (M2) have no consumer until
  Render reads them. This subtask is the render-side half of that pair.
- **Zero v1 regression by construction:** the variadic `mode ...RenderMode` is purely additive for
  callers — every existing 4-arg call compiles and behaves identically (default bare). This is why a
  2-point subtask can land the hook without touching generate.go/stagecoach.go.

## What

A new exported `RenderMode` type + two constants in `render.go`; a trailing variadic parameter on
`Manifest.Render`; and the bare-flags append line replaced by a 3-way switch (resolve selectedMode →
tooled/error/bare). No struct change (S1 added TooledFlags), no Resolve change, no caller change, no
builtin value change, no docs/config/CLI change.

### Success Criteria

- [ ] `render.go` declares `type RenderMode string` and exported constants `RenderBare = "bare"`,
      `RenderTooled = "tooled"`.
- [ ] `Render` signature is exactly
      `func (m Manifest) Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode) (*CmdSpec, error)`.
- [ ] The `args = append(args, r.BareFlags...)` line is replaced by a switch that: resolves
      `selectedMode` (default `RenderBare` when `mode` is empty or `mode[0]==""`); in `RenderTooled`
      appends `r.TooledFlags` OR returns `fmt.Errorf("provider %q: tooled mode requires non-empty tooled_flags", m.Name)` when `len(r.TooledFlags)==0`; default arm appends `r.BareFlags`.
- [ ] All other Render logic (Validate, Resolve, model/provider defaults, token order, sys-prepend
      fallback, delivery switch, Env) is byte-identical.
- [ ] The Render doc comment documents the new `mode` parameter, the bare default, and the tooled-empty
      error; the token-order comment reflects the mode ternary.
- [ ] `Render(model, provider, sys, user)` with NO mode produces byte-identical `*CmdSpec` to the v1
      output (existing tests + callers unchanged).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean.
- [ ] `go test -race ./...` green across the WHOLE repo (provider render tests extended; no other
      package's behavior changes).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the current `Render` body verbatim around the single edit point,
states the exact replacement (the verified mode switch), gives the exact RenderMode type/constants and
their placement, lists the full caller inventory proving backward compatibility, provides 5 new tests
(with a fixture) reusing the existing helpers, and pins the exact validation commands. S1 is confirmed
landed (TooledFlags exists; baseline green). No inference required.

### Documentation & References

```yaml
# MUST READ — the binding algorithm + the field contract
- file: PRD.md
  why: "§12.2 (Command rendering algorithm) is AUTHORITATIVE for the mode ternary: `args += (mode == \"tooled\") ? m.tooled_flags : m.bare_flags`. §11.5 (two invocation modes) defines bare vs tooled + which roles use each. §12.1 (manifest schema) defines tooled_flags: 'nil/empty => cannot serve as a stager'."
  critical: "§12.2's ternary is why the switch's default arm is bare (only \"tooled\" triggers tooled). §12.1's 'nil/empty => cannot stager' is why tooled+empty TooledFlags must ERROR. This subtask owns ONLY render.go + render_test.go; do NOT touch manifest.go (S1), merge.go (S2), or builtin.go (M2)."

- docfile: plan/002_a17bb6c8dc1d/architecture/manifest_v2_delta.md
  why: "§3 (Render Mode Support) prescribes the verbatim signature, the RenderMode type/constants, AND the exact mode-switch code block (selectedMode resolution + switch + the tooled-empty error). This is the source-of-truth implementation sketch."
  critical: "§3's code block is the implementation. Note it uses `r.TooledFlags` (the resolved copy) and `m.Name` in the error — both verified against the current render.go variable names in research §5."

- docfile: plan/002_a17bb6c8dc1d/P1M1T2S1/research/render_mode_implementation_notes.md
  why: "EMPIRICALLY VERIFIED facts: the baseline is GREEN; S1 landed (TooledFlags exists on the resolved manifest); the FULL caller inventory (16+ sites, all 4-arg → backward compatible); the exact single-line replacement point; why default→bare matches PRD §12.2; the m.Name-vs-r.Name convention; the test taxonomy + the dualModeManifest() fixture. READ THIS FIRST."
  critical: "§2 (caller inventory) is the proof that the variadic change is purely additive. §3.2 is the verbatim replacement block. §8 (Decisions D1–D9) resolves every design question an implementer would otherwise guess at."

- docfile: plan/002_a17bb6c8dc1d/architecture/system_context.md
  why: "§'Render Signature' confirms the current 4-arg signature and that 'all existing callers pass bare mode'. NOTE: its suggestion of `mode ...string` is OVERRIDDEN by the contract's typed `mode ...RenderMode` (research D2)."
  critical: "The contract is authoritative over this planning sketch — use the TYPED RenderMode, not plain string."

- file: internal/provider/render.go
  why: "THE edit target. Contains CmdSpec + the Render method. The single line `args = append(args, r.BareFlags...)` (after the system-prompt block, before the print_flag block) is what gets replaced by the mode switch. Validate→Resolve→token-order→payload-prepend→delivery-switch→Env are all UNCHANGED."
  pattern: "Render resolves `r := m.Resolve()` then derefs `*r.X`; the mode switch reads `r.TooledFlags` / `r.BareFlags` and errors with `m.Name` (matching the two existing error sites that use `m.Name`)."
  gotcha: "The replacement MUST preserve print_flag as LAST (it appends AFTER the flag-set block). Do NOT move the print_flag append."

- file: internal/provider/manifest.go
  why: "READ-ONLY ref (S1 landed). Confirms `TooledFlags []string` exists and Resolve() leaves it as-is (nil→nil), so `len(r.TooledFlags)==0` catches BOTH nil and `[]string{}`. Do NOT edit (S1 owns it)."
  pattern: "TooledFlags is a plain []string (slice regime): nil = absent, non-nil-even-if-empty = present. Resolve does NOT default it (unlike Experimental→*false)."

- file: internal/provider/render_test.go
  why: "EDIT TARGET (tests). Same-package tests (`package provider`); reuses builtinPi()/builtinClaude()/…, strPtr, the containsPair helper, and reflect.DeepEqual golden-args. Table-driven with t.Run subtests. The existing 10 tests ALL pass 4 args (no mode) → they exercise the bare path and stay green unchanged."
  pattern: "One focused test per behavior (see TestRender_GoldenPerProvider / _ValidateErrors). Add 5 new tests mirroring this taxonomy + a dualModeManifest() fixture that sets BOTH flag-sets."
  gotcha: "renderArgs (builtin_test.go:137) is a bare-only TEST scaffold — it does NOT take a mode and only emits BareFlags. TestRender_CompatWithRenderArgs compares bare↔bare and is UNAFFECTED; do NOT add a mode to renderArgs."

# External references (exact, anchor-level)
- url: https://go.dev/ref/spec#Function_types
  why: "Go variadic parameters: a final `mode ...RenderMode` accepts ZERO or more args, so `Render(a,b,c,d)` (4 args) is valid and passes an empty `mode` slice. This is the language basis for the backward-compatible signature change."
  critical: "This is WHY no caller needs editing — a variadic final parameter is optional at every call site."
- url: https://go.dev/ref/spec#Type_declarations
  why: "A named type `type RenderMode string` has the same underlying type as string but is assignment-incompatible with untyped string literals only via explicit conversion — `RenderTooled` is a typed constant. Callers pass `provider.RenderTooled` (type-safe), not a magic `\"tooled\"`."
```

### Current Codebase Tree (relevant slice — v1 fully implemented)

```bash
stagecoach/
└── internal/provider/
    ├── manifest.go        # S1 LANDED: has TooledFlags []string + Experimental *bool + Resolve (TooledFlags left nil)
    ├── render.go          # EDIT TARGET: CmdSpec + Render (4-arg; unconditional r.BareFlags append)
    ├── render_test.go     # EDIT TARGET (tests): 10 existing tests, all 4-arg bare
    ├── merge.go           # S2 (parallel): MergeManifest — does NOT touch render.go
    ├── merge_test.go      # S2 (parallel)
    ├── builtin.go         # READ-ONLY: 6 builtins; agy + tooled_flags VALUES = M2 (not here)
    ├── builtin_test.go    # READ-ONLY: renderArgs helper (bare-only test scaffold)
    ├── registry.go, executor.go, parse.go, procgroup_*.go   # unaffected
└── (callers, unchanged)
    internal/generate/generate.go:191   # deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
    pkg/stagecoach/stagecoach.go:304      # deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
    internal/generate/realagent_test.go:70, internal/stubtest/stubtest_test.go (×11)  # 4-arg bare calls
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/provider/render.go        # +RenderMode type/constants; Render +variadic mode; mode switch; doc updates
    internal/provider/render_test.go   # +5 tests + dualModeManifest() fixture
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/provider/render.go` | MODIFY | Add RenderMode type + constants; variadic `mode` param; replace the BareFlags append with the mode switch; update doc comments. **Only production file touched.** |
| `internal/provider/render_test.go` | MODIFY | Add 5 focused tests + a dualModeManifest() fixture for the new mode behaviors. Existing 10 tests unchanged. |

**Explicitly NOT touched**: `manifest.go` (S1 — TooledFlags already there), `merge.go`/`merge_test.go`
(S2 — parallel), `builtin.go`/`builtin_test.go`/`providers/*.toml` (agy + tooled-flags VALUES = P1.M2),
`registry.go`/`executor.go`/`parse.go` (unaffected), `generate.go`/`stagecoach.go` (callers — unchanged by
the variadic), any `docs/*.md` (contract: Mode A = code doc comments only), `PRD.md`, `tasks.json`,
`prd_snapshot.md`, anything under `plan/`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — backward compat is THE requirement): the variadic `mode ...RenderMode` MUST be the
// LAST parameter and every existing caller passes exactly 4 args (no mode). Go variadics accept zero
// trailing args, so `Render(a,b,c,d)` compiles and yields mode==nil (→ default bare). Do NOT reorder
// parameters or make mode required — that would break 16+ call sites. (Caller inventory: research §2.)

// CRITICAL (G2 — the edit is ONE line): replace ONLY `args = append(args, r.BareFlags...)` (the line
// after the system-prompt block, before the print_flag block). Do NOT touch Validate, Resolve, the
// model/provider default fallback, the token order, the sys-prepend fallback, the delivery switch, or
// the Env build. The print_flag append MUST stay AFTER the flag-set block (it is LAST per §12.2).

// CRITICAL (G3 — tooled + empty TooledFlags is an ERROR, not silent bare): when selectedMode==RenderTooled
// and len(r.TooledFlags)==0, return (nil, error). The error message is the contract's verbatim:
//   fmt.Errorf("provider %q: tooled mode requires non-empty tooled_flags", m.Name)
// Do NOT fall through to bare in this case — that would silently produce a tool-less "stager" command.
// len(r.TooledFlags)==0 catches BOTH nil and []string{} because Resolve leaves TooledFlags as-is (S1).

// GOTCHA (G4 — use m.Name, not r.Name, in the error): the value receiver is `m`; `r := m.Resolve()` is
// the resolved copy. The two EXISTING error sites in render.go both use m.Name. Resolve copies Name
// unchanged so they're equal, but match the existing convention for consistency. (Research §5.)

// GOTCHA (G5 — default arm is bare, NOT an error for unknown modes): PRD §12.2 is a strict ternary
// `mode=="tooled" ? tooled : bare`. Only "tooled" triggers tooled; RenderBare, "", and any unrecognized
// mode all fall to the `default` arm → BareFlags. Erroring on unknown modes would DIVERGE from the spec.
// The only modes are the two exported constants; there is no untrusted mode input.

// GOTCHA (G6 — RenderMode must be EXPORTED and is NOT a manifest field): place it in render.go (Render's
// parameter type), NOT manifest.go. It must be exported (RenderBare/RenderTooled) so internal/decompose
// (P3.M2.T3) can call `…Render(model, provider, sys, payload, provider.RenderTooled)`. Use the TYPED
// RenderMode per the contract, NOT plain `...string` (system_context.md's sketch is overridden).

// GOTCHA (G7 — selectedMode resolution guards the empty-string case): resolve as
//   selectedMode := RenderBare
//   if len(mode) > 0 && mode[0] != "" { selectedMode = mode[0] }
// The `mode[0] != ""` guard means an explicitly-passed RenderMode("") is treated as bare (default),
// matching the variadic "no arg" case. Without it, mode[0]=="" would hit the default arm anyway (since
// "" != "tooled"), so it is belt-and-suspenders — but use the architecture-delta form verbatim.

// GOTCHA (G8 — do NOT add a mode to renderArgs): renderArgs (builtin_test.go:137) is a bare-only TEST
// scaffold. TestRender_CompatWithRenderArgs compares Render's bare output to renderArgs's bare output.
// Adding a mode parameter to renderArgs is out of scope and would break that compat test for no benefit.

// GOTCHA (G9 — Resolve already handles TooledFlags; do NOT change Resolve): S1 made Resolve leave
// TooledFlags as-is (nil→nil). The switch's `len(r.TooledFlags)==0` check relies on this. Do NOT add a
// Resolve default for TooledFlags (that would make a nil TooledFlags non-empty and break the error path).

// GOTCHA (G10 — this is stdlib-only; no deps): Render already imports only fmt and os. RenderMode is a
// named string type — no new imports. Do NOT run go get or edit go.mod/go.sum.
```

## Implementation Blueprint

### Data models and structure

No new structs. The single new type + constants (place in `render.go`, immediately before the `Render`
method, after the `CmdSpec` type):

```go
// RenderMode selects which flag-set Manifest.Render appends after the system-prompt block (PRD §11.5,
// §12.2). It is the v2 "mode" dimension of the §12.2 rendering ternary
// `args += (mode == "tooled") ? m.tooled_flags : m.bare_flags`.
//
// Render's `mode ...RenderMode` parameter is VARIADIC and defaults to RenderBare when omitted, so every
// v1 caller (generate.CommitStaged, pkg/stagecoach.runPipeline, all tests) is unchanged. The decompose
// stager (P3.M2.T3) passes RenderTooled.
type RenderMode string

const (
	// RenderBare appends BareFlags — tools off, session-less, chrome-less, ephemeral (PRD §12.1).
	// The DEFAULT mode. Serves the planner / message / arbiter roles and the entire v1 single-commit path.
	RenderBare RenderMode = "bare"

	// RenderTooled appends TooledFlags — tools on, git-scoped, non-interactive (PRD §12.1 tooled_flags).
	// Serves the stager role (the only role that mutates the index, §11.5). Errors at render time if
	// TooledFlags is nil/empty — that provider cannot serve as a stager.
	RenderTooled RenderMode = "tooled"
)
```

### The single edit to `Render` (exact — current → new)

The current signature + the flag-set block (verbatim from render.go):

```go
func (m Manifest) Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error) {
	// ... Validate, Resolve, model/provider defaults ...
	if *r.SystemPromptFlag != "" && sysPrompt != "" {
		args = append(args, *r.SystemPromptFlag, sysPrompt)
	}
	args = append(args, r.BareFlags...)                 // ← THE ONE LINE THAT CHANGES
	if *r.PrintFlag != "" {
		args = append(args, *r.PrintFlag) // LAST per §12.2
	}
	// ... payload prepend, delivery switch, Env ...
}
```

**After the edit** (signature + the block; everything else byte-identical):

```go
func (m Manifest) Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode) (*CmdSpec, error) {
	// ... Validate, Resolve, model/provider defaults UNCHANGED ...
	if *r.SystemPromptFlag != "" && sysPrompt != "" {
		args = append(args, *r.SystemPromptFlag, sysPrompt)
	}
	// §11.5 / §12.2 mode: bare (tools off, planner/message/arbiter + all v1 callers) vs tooled
	// (git tools on, stager). Defaults to bare when mode is omitted (variadic) — keeps every v1
	// caller byte-for-byte unchanged. Tooled with empty tooled_flags is an error (§12.1: that
	// provider cannot serve as a stager).
	selectedMode := RenderBare
	if len(mode) > 0 && mode[0] != "" {
		selectedMode = mode[0]
	}
	switch selectedMode {
	case RenderTooled:
		if len(r.TooledFlags) == 0 {
			return nil, fmt.Errorf("provider %q: tooled mode requires non-empty tooled_flags", m.Name)
		}
		args = append(args, r.TooledFlags...)
	default: // RenderBare — also the fallback for "" / any unrecognized mode (PRD §12.2 ternary)
		args = append(args, r.BareFlags...)
	}
	if *r.PrintFlag != "" {
		args = append(args, *r.PrintFlag) // LAST per §12.2
	}
	// ... payload prepend, delivery switch, Env UNCHANGED ...
}
```

### Doc-comment updates (contract Mode A — code comments only)

1. **Render doc comment — add a paragraph** after the existing "System-prompt + payload" paragraph:
   > `mode ...RenderMode` (variadic, default `RenderBare`) selects the flag-set appended after the
   > system-prompt block: `RenderBare` (the default) appends `BareFlags` (tools off — planner/message/
   > arbiter + the entire v1 single-commit path); `RenderTooled` appends `TooledFlags` (git tools on —
   > the stager role, §11.5). `RenderTooled` on a manifest with nil/empty `TooledFlags` returns an
   > error — that provider cannot serve as a stager (§12.1). The variadic default keeps every v1 caller
   > unchanged.

2. **Token-order comment block — update the `+ bare_flags...` line** to reflect the ternary:
   > `+ (mode==tooled ? tooled_flags : bare_flags)...    # §11.5/§12.2 mode ternary; default bare`

3. **CmdSpec doc comment — NO change** (CmdSpec is mode-agnostic pure data; contract's "if needed" →
   not needed).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD the RenderMode type + constants to render.go
  - FILE: internal/provider/render.go
  - PLACE: immediately BEFORE the `func (m Manifest) Render` method (after the `type CmdSpec` block).
  - WRITE: the `type RenderMode string` + the two constants EXACTLY as in "Data models and structure" above.
  - NAMING: exported — `RenderMode`, `RenderBare`, `RenderTooled` (capitalized; needed by internal/decompose).
  - DO NOT: place it in manifest.go (it is Render's parameter, not a manifest field).
  - VERIFY: `go build ./internal/provider/` → exit 0.

Task 2: CHANGE the Render signature + replace the BareFlags append with the mode switch
  - FILE: internal/provider/render.go
  - EDIT 1 (signature): change
        func (m Manifest) Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error) {
    to
        func (m Manifest) Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode) (*CmdSpec, error) {
  - EDIT 2 (the switch): replace the single line
        args = append(args, r.BareFlags...)
    with the selectedMode resolution + switch block EXACTLY as in "The single edit to Render" above
    (tooled→TooledFlags-or-error; default→BareFlags). Preserve the print_flag append IMMEDIATELY after.
  - PRESERVE: Validate, Resolve, model/provider default fallback, token order, sys-prepend fallback,
    delivery switch, Env build — all byte-identical.
  - VERIFY: `go build ./...` → exit 0 (all 16+ existing callers compile unchanged).
  - VERIFY: `go test -race ./internal/provider/ -run TestRender` → existing 10 tests still PASS (bare path unchanged).

Task 3: UPDATE the Render doc comment + token-order comment (Mode A)
  - EDIT the Render doc comment to add the mode paragraph (text in "Doc-comment updates" #1).
  - EDIT the token-order comment block's `+ bare_flags...` line to the ternary form (#2).
  - DO NOT: edit the CmdSpec doc comment (#3 — no change needed).

Task 4: ADD tests to render_test.go (one focused test per new behavior)
  - FILE: internal/provider/render_test.go (package provider — same package; can use builtinPi/strPtr/containsPair)
  - ADD helper fixture near the top (after imports / before TestRender_GoldenPerProvider):
        func dualModeManifest() Manifest {
            return Manifest{
                Name:        "dual",
                Command:     strPtr("agent"),
                BareFlags:   []string{"--no-tools"},
                TooledFlags: []string{"--allowed-tools", "git:*", "--approval-mode", "auto"},
            }
        }
  - ADD TestRender_DefaultModeIsBare:
        m := dualModeManifest()
        spec, err := m.Render("", "", "", "U")           // NO mode arg
        // assert err==nil; assert containsPair(spec.Args,"--no-tools"? ) — actually check BareFlags present,
        //   TooledFlags absent. Use: argsContain(spec.Args, "--no-tools")==true && argsContain(spec.Args,"--allowed-tools")==false.
  - ADD TestRender_ExplicitBareMode:
        spec, _ := m.Render("", "", "", "U", RenderBare)  // explicit bare
        // assert identical to the no-mode call (same Args/Stdin). DeepEqual against the no-mode spec.
  - ADD TestRender_TooledModeAppendsTooledFlags:
        spec, err := m.Render("", "", "", "U", RenderTooled)
        // assert err==nil; assert "--allowed-tools" present AND "--no-tools" ABSENT (bare not mixed in).
  - ADD TestRender_TooledModeEmptyFlagsErrors:
        bareOnly := Manifest{Name: "stager", Command: strPtr("agent"), BareFlags: []string{"--no-tools"}}  // TooledFlags nil
        _, err := bareOnly.Render("", "", "", "U", RenderTooled)
        // assert err != nil; assert strings.Contains(err.Error(), "tooled mode requires non-empty tooled_flags")
        // assert strings.Contains(err.Error(), `"stager"`)  (the provider name is quoted in the message)
  - ADD TestRender_AllGoldenProvidersStillBareDefault (regression guard):
        for _, b := range []Manifest{builtinPi(), builtinClaude(), builtinGemini(), builtinOpenCode(), builtinCodex(), builtinCursor()} {
            spec, err := b.Render(modelFor(b), providerFor(b), "<sys>", "<user>")  // no mode
            // assert err==nil (the no-mode path is unchanged); spot-check Command != "".
        }
        (modelFor/providerFor: reuse the per-provider values already in TestRender_GoldenPerProvider's table,
         or just pass "" and rely on defaults — the point is only that no-mode renders without error.)
  - NOTE: add a tiny argsContain helper (or reuse a loop) since containsPair checks a (flag,value) PAIR,
    not a single token. Simplest: inline `slices.Contains(spec.Args, "--no-tools")` (slices is Go 1.21+;
    module is go 1.22 ✓) or a 3-line loop.
  - VERIFY: `go test -race ./internal/provider/ -v -run TestRender` → ALL render tests (old + new) PASS.

Task 5: VALIDATE (run all gates; all must pass before declaring done)
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: gofmt -l .            # must be empty
  - RUN: go test -race ./...   # WHOLE repo green (no other package's behavior changes)
  - RUN targeted: go test -race ./internal/provider/ -run TestRender
  - RUN regression proof: go test -race ./internal/generate/ ./pkg/stagecoach/ ./internal/stubtest/  # callers unchanged
  - FIX-FORWARD: read failures, fix, re-run. Do NOT skip.
```

### Implementation Patterns & Key Details

```go
// === Why the switch's default is bare (PRD §12.2 ternary) ===
// The PRD rendering algorithm is: `args += (mode == "tooled") ? m.tooled_flags : m.bare_flags`.
// That is a strict two-way branch on "tooled"; EVERYTHING ELSE (bare, empty, unrecognized) → bare_flags.
// So `default: args = append(args, r.BareFlags...)` is the faithful implementation. Erroring on an
// unrecognized mode would diverge from the spec; the only modes are the two exported constants anyway.

// === Why tooled + empty TooledFlags errors (PRD §12.1) ===
// §12.1: "tooled_flags ... nil/empty => this provider does not support tooled mode and cannot serve as
// a stager." Rendering a tool-less "stager" command would silently do the wrong thing (the stager MUST
// mutate the index). Fail fast at render with a named-provider message. len(r.TooledFlags)==0 catches
// nil AND []string{} because Resolve leaves TooledFlags as-is (S1).

// === Why variadic keeps callers unchanged (Go spec) ===
// A final `mode ...RenderMode` parameter accepts ZERO trailing args: `Render(a,b,c,d)` is valid and
// passes mode==nil. The selectedMode resolution (`if len(mode) > 0 …`) then defaults to RenderBare.
// So generate.go:191, stagecoach.go:304, and all 14 test call sites compile and behave identically.

// === The edit is surgically one line + its replacement block ===
// Before:  args = append(args, r.BareFlags...)
// After:   selectedMode := RenderBare; if len(mode) > 0 && mode[0] != "" { selectedMode = mode[0] }
//          switch selectedMode { case RenderTooled: …; default: args = append(args, r.BareFlags...) }
// The print_flag append (`if *r.PrintFlag != "" { args = append(args, *r.PrintFlag) }`) stays
// IMMEDIATELY after — it is LAST per §12.2 in BOTH modes.
```

### Integration Points

```yaml
RENDER (internal/provider/render.go):
  - signature: + trailing `mode ...RenderMode` (variadic; default RenderBare)
  - flag-set block: mode switch replaces the unconditional r.BareFlags append
  - error: tooled + len(r.TooledFlags)==0 → "provider %q: tooled mode requires non-empty tooled_flags"
  - docs: Render doc comment + token-order comment updated (Mode A)

NO-TOUCH (explicitly — owned by other subtasks):
  - internal/provider/manifest.go    # S1 (DONE) — TooledFlags field + Resolve; do not re-edit
  - internal/provider/merge.go       # S2 (parallel) — MergeManifest; does not touch render
  - internal/provider/builtin.go     # agy + tooled_flags VALUES = P1.M2.T1/T2
  - internal/provider/builtin_test.go # renderArgs is bare-only; do NOT add a mode
  - internal/provider/registry.go, executor.go, parse.go   # read other fields; unaffected
  - internal/generate/generate.go, pkg/stagecoach/stagecoach.go  # callers — UNCHANGED (variadic)
  - docs/*.md                        # contract: Mode A = code doc comments only

DOWNSTREAM HOOKS (informational — implemented by LATER subtasks, NOT this one):
  - P1.M2.T2.S2: sets pi/claude tooled_flags as BUILTINS (gives the tooled branch real flags to append)
  - P3.M2.T3.S1: the stager calls `manifest.Render(model, provider, sys, payload, provider.RenderTooled)`
  - P3.M2.T1.S1 (roles): resolves which provider serves the stager role (must have non-empty tooled_flags)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l .                       # Expected: empty (run `gofmt -w internal/provider/render.go render_test.go` if listed)
go vet ./internal/provider/...   # Expected: exit 0
go build ./...                   # Expected: exit 0 (variadic keeps all 16+ callers compiling)

# Expected: Zero output/errors. The build passing is itself the proof that the variadic change broke
# no caller (a required positional mode would fail to compile here).
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

# Targeted: all render tests (existing 10 unchanged + 5 new)
go test -race ./internal/provider/ -v -run TestRender

# Full provider suite
go test -race ./internal/provider/ -v

# Expected: ALL render tests PASS.
#   - Existing 10 (GoldenPerProvider, Pi_ByteForByte, SystemPromptPrependFallback, ModelDefaultFallback,
#     ProviderDefaultFallback, Env, FlagDelivery, ValidateErrors, DoesNotMutateManifest,
#     CompatWithRenderArgs) — UNCHANGED, still green (bare path byte-identical).
#   - New 5 (DefaultModeIsBare, ExplicitBareMode, TooledModeAppendsTooledFlags,
#     TooledModeEmptyFlagsErrors, AllGoldenProvidersStillBareDefault) — green.
```

### Level 3: Whole-Repository Regression (No Behavior Change Elsewhere)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages pass (callers unchanged; only render.go + test edited)

# Caller regression proof — the 3 packages that call Render must be green unchanged
go test -race ./internal/generate/ ./pkg/stagecoach/ ./internal/stubtest/

# Confirm ONLY internal/provider/render.go (+ test) changed in production source
git diff --stat -- internal/ pkg/ cmd/ providers/
# Expected: only internal/provider/render.go + internal/provider/render_test.go appear.

# Confirm manifest.go / merge.go were NOT re-edited by this subtask
git diff --stat -- internal/provider/manifest.go internal/provider/merge.go || echo "OK: untouched by T2.S1"
```

### Level 4: Behavioral Cross-Check (prove the mode ternary end-to-end)

```bash
cd /home/dustin/projects/stagecoach

# Throwaway main: exercises all three branches (bare default, explicit bare, tooled, tooled-error)
# exactly as the tests do, against the real Render.
cat > /tmp/sh_mode_check.go <<'EOF'
package main
import ("fmt"; "github.com/dustin/stagecoach/internal/provider")
func strPtr(s string) *string { return &s }
func main() {
  dual := provider.Manifest{Name: "dual", Command: strPtr("agent"),
    BareFlags: []string{"--no-tools"}, TooledFlags: []string{"--allowed-tools", "git:*"}}
  bareOnly := provider.Manifest{Name: "stager", Command: strPtr("agent"), BareFlags: []string{"--no-tools"}}

  s1, e1 := dual.Render("", "", "", "U")                              // no mode → bare
  s2, e2 := dual.Render("", "", "", "U", provider.RenderBare)         // explicit bare
  s3, e3 := dual.Render("", "", "", "U", provider.RenderTooled)       // tooled → tooled flags
  _, e4  := bareOnly.Render("", "", "", "U", provider.RenderTooled)   // tooled + empty → error

  fmt.Printf("default-bare:  err=%v hasNoTools=%v\n", e1, contains(s1.Args,"--no-tools"))
  fmt.Printf("explicit-bare: err=%v sameAsDefault=%v\n", e2, fmt.Sprint(s1.Args)==fmt.Sprint(s2.Args))
  fmt.Printf("tooled:        err=%v hasAllowedTools=%v hasNoTools=%v\n", e3, contains(s3.Args,"--allowed-tools"), contains(s3.Args,"--no-tools"))
  fmt.Printf("tooled-empty:  err=%v\n", e4)
}
func contains(a []string, s string) bool { for _, x := range a { if x==s {return true} }; return false }
EOF
go run /tmp/sh_mode_check.go && rm -f /tmp/sh_mode_check.go
# Expected output:
#   default-bare:  err=<nil> hasNoTools=true
#   explicit-bare: err=<nil> sameAsDefault=true
#   tooled:        err=<nil> hasAllowedTools=true hasNoTools=false
#   tooled-empty:  err=provider "stager": tooled mode requires non-empty tooled_flags
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages pass (provider render tests extended + green; callers unchanged).

### Feature Validation

- [ ] `render.go` declares `type RenderMode string` + exported `RenderBare`/`RenderTooled` constants.
- [ ] `Render` signature ends in `mode ...RenderMode` (variadic; default `RenderBare`).
- [ ] The mode switch: tooled→`r.TooledFlags` (or the verbatim error if empty); default→`r.BareFlags`.
- [ ] `Render(a,b,c,d)` (no mode) and `Render(a,b,c,d,RenderBare)` produce byte-identical `*CmdSpec`.
- [ ] `Render(...,RenderTooled)` with non-empty TooledFlags appends TooledFlags and NOT BareFlags.
- [ ] `Render(...,RenderTooled)` with nil/empty TooledFlags returns an error quoting the provider name.
- [ ] The print_flag append remains LAST (immediately after the flag-set block) in both modes.
- [ ] Render doc comment + token-order comment document the mode parameter and ternary.

### Backward-Compatibility & Scope Discipline Validation

- [ ] All existing 10 render tests pass UNCHANGED (no test-body edit to the existing tests).
- [ ] `go test -race ./internal/generate/ ./pkg/stagecoach/ ./internal/stubtest/` green (callers untouched).
- [ ] ONLY `internal/provider/render.go` (+ `render_test.go`) modified (`git diff --stat` confirms).
- [ ] Did NOT edit `manifest.go` (S1), `merge.go` (S2), `builtin.go`/`builtin_test.go`/`providers/*.toml` (P1.M2).
- [ ] Did NOT add a mode to `renderArgs` (bare-only test scaffold).
- [ ] Did NOT change `Resolve` (S1 already leaves TooledFlags nil; the empty-check relies on it).
- [ ] Did NOT edit any `docs/*.md` (contract: Mode A = code doc comments only), `PRD.md`, `tasks.json`,
      `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] RenderMode is a TYPED named string (not plain `...string`); constants are exported.
- [ ] Error uses `m.Name` (matches the two existing error sites in render.go).
- [ ] `default` arm is bare (PRD §12.2 ternary) — unknown modes do not error.
- [ ] New tests reuse `strPtr`/`containsPair`/builtins and follow the table-driven + one-test-per-behavior taxonomy.

---

## Anti-Patterns to Avoid

- ❌ Don't make `mode` a REQUIRED positional parameter — that breaks 16+ call sites. It MUST be variadic
  (`mode ...RenderMode`) so `Render(a,b,c,d)` still compiles and defaults to bare (gotcha G1).
- ❌ Don't use plain `...string` — the contract mandates the typed `RenderMode` (system_context.md's
  `...string` sketch is overridden). Type-safety (`provider.RenderTooled`) at zero runtime cost (gotcha G6).
- ❌ Don't fall through to bare when tooled is requested but TooledFlags is empty — that silently produces
  a tool-less "stager" command. It MUST error with the verbatim message (gotcha G3).
- ❌ Don't error on an unrecognized mode — PRD §12.2 is a strict ternary (`=="tooled" ? tooled : bare`);
  the `default` arm is bare. Only `"tooled"` triggers tooled (gotcha G5).
- ❌ Don't touch anything other than the `args = append(args, r.BareFlags...)` line (+ signature + docs).
  Validate/Resolve/token-order/prepend/delivery/Env are byte-identical; the print_flag append stays LAST (gotcha G2).
- ❌ Don't move the print_flag append — it must follow the flag-set block in BOTH modes (§12.2: print_flag LAST).
- ❌ Don't use `r.Name` in the error — use `m.Name` to match the existing two error sites (gotcha G4).
- ❌ Don't place RenderMode in manifest.go — it's Render's parameter type, not a manifest field; it goes
  in render.go before Render (gotcha G6).
- ❌ Don't add a Resolve default for TooledFlags — S1 leaves it nil precisely so `len(r.TooledFlags)==0`
  detects "cannot stager"; defaulting it would break the error path (gotcha G9).
- ❌ Don't add a mode to `renderArgs` (builtin_test.go) — it's a bare-only test scaffold; the compat test
  compares bare↔bare and is unaffected (gotcha G8).
- ❌ Don't edit `manifest.go` (S1), `merge.go` (S2), `builtin.go` (M2 values), the callers, or any docs/config.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a surgically small, fully-prescribed change to ONE production line (+ its replacement
block) plus a typed parameter and doc tweaks. The implementation is dictated verbatim by the architecture
delta (§3) and the contract; the baseline is GREEN (verified); S1 already landed the `TooledFlags` field
the switch reads (verified in manifest.go); and the variadic signature is provably backward-compatible
(the full 16+ caller inventory, all 4-arg, is enumerated in research §2 — `go build ./...` passing IS the
regression proof). The mode switch's semantics are pinned by PRD §12.2's ternary (`=="tooled" ? tooled :
bare`) and §12.1's "nil/empty ⇒ cannot stager" → the error. The #1 failure mode — making `mode` required
or erroring on the wrong branch — is front-loaded as CRITICAL gotchas (G1/G3/G5) with the build + a
behavioral cross-check (Level 4) as deterministic gates. The only residual uncertainty (not 10/10) is
doc-comment wording precision (cosmetic, gated by gofmt) and the exact helper style for the new tests
(`containsPair` checks pairs, so a single-token check needs `slices.Contains` or a tiny loop — called out
in Task 4). The S2/M2 boundaries are cleanly fenced: S2 changes only merge.go (never render.go), and M2
sets tooled_flags VALUES as builtins (consumed by this switch but not edited here) — neither can be broken
by this subtask.
