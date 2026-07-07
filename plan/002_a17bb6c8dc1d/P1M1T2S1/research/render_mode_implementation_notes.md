# Research: RenderMode + Variadic Render Signature (P1.M1.T2.S1)

> **Purpose:** Pin the exact edit to `internal/provider/render.go` for the bare/tooled mode
> parameter, validated against the live v1 codebase (baseline `go test ./internal/provider/`
> GREEN as of 2026-07-01), the architecture delta (`manifest_v2_delta.md` §3), and PRD §12.2/§11.5.

---

## 1. Environment & Baseline

| Item | Value / Evidence |
|---|---|
| Module | `github.com/dustin/stagecoach`, `go 1.22` |
| Edit target | `internal/provider/render.go` (exists; 1 method `Render` + type `CmdSpec`) |
| S1 (TooledFlags) status | **LANDED & VERIFIED** — `manifest.go` has `TooledFlags []string` + `Experimental *bool`; `Resolve()` leaves TooledFlags as-is (nil stays nil) and defaults Experimental→`*false`. |
| S2 (MergeManifest) status | Parallel (assumed landed per the contract — does NOT touch render.go; safe either way). |
| Baseline test | `go test -race ./internal/provider/` → **ok** (1.419s). |
| git / go | git 2.54.0 / go1.26.4 (irrelevant to this pure-Go edit). |

**Implication:** `TooledFlags` already exists on the resolved manifest `r`; Render just needs to read
`r.TooledFlags` in the new tooled branch. No struct change, no Resolve change (S1 owns those).

---

## 2. Caller Inventory — the backward-compatibility proof

Every existing call to `Render` passes **exactly 4 positional args** (no mode). A variadic
`mode ...RenderMode` final parameter makes ALL of them compile and behave identically (zero variadic
args → default bare). This is why the contract mandates variadic over a required positional.

| File:Line | Call (4 args) | Status after change |
|---|---|---|
| `internal/generate/generate.go:191` | `deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)` | UNCHANGED (bare) |
| `pkg/stagecoach/stagecoach.go:304` | `deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)` | UNCHANGED (bare) |
| `internal/generate/realagent_test.go:70` | `m.Render(cfg.Model, cfg.Provider, "<system prompt>", "<staged diff>")` | UNCHANGED (bare) |
| `internal/stubtest/stubtest_test.go` (×11) | `m.Render("", "", "", "payload…")` | UNCHANGED (bare) |
| `internal/provider/render_test.go` (10 tests) | `…Render(model, provider, "<sys>", "<user>")` | UNCHANGED (bare) — byte-identical output |
| (future) `internal/decompose/stager.go` (P3.M2.T3) | `…Render(model, provider, sys, payload, provider.RenderTooled)` | NEW — the only tooled caller |

**The variadic change is purely additive for callers.** No caller needs editing. This is the keystone
of why a 2-point subtask can land the mode hook without touching the v1 pipeline.

---

## 3. The Exact Replacement Point in render.go (current → new)

### 3.1 Current code (the SINGLE line that changes)

```go
	if *r.SystemPromptFlag != "" && sysPrompt != "" {
		args = append(args, *r.SystemPromptFlag, sysPrompt)
	}
	args = append(args, r.BareFlags...)      // ← REPLACE THIS ONE LINE with the mode switch
	if *r.PrintFlag != "" {
		args = append(args, *r.PrintFlag) // LAST per §12.2
	}
```

The line sits **after the system-prompt flag block** and **before the print_flag block** — exactly the
"after system-prompt block, before print_flag" anchor the contract names. Everything else in Render
(Validate → Resolve → model/provider default fallback → token order → payload prepend → delivery
switch → Env) is byte-identical.

### 3.2 New code (architecture delta §3, verbatim — proven against PRD §12.2 ternary)

```go
	// §11.5 / §12.2 mode: bare (tools off, planner/message/arbiter) vs tooled (git tools, stager).
	// Defaults to bare when no mode is passed (keeps every v1 caller unchanged). Tooled with empty
	// tooled_flags is an error — that provider cannot serve as a stager (§12.1 tooled_flags doc).
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
	default: // RenderBare (also the fallback for "" / any unrecognized mode — PRD §12.2 ternary)
		args = append(args, r.BareFlags...)
	}
	if *r.PrintFlag != "" {
		args = append(args, *r.PrintFlag) // LAST per §12.2
	}
```

### 3.3 Why `default: RenderBare` (not an error for unknown modes)

PRD §12.2 is AUTHORITATIVE for the rendering algorithm:
```
args += (mode == "tooled") ? m.tooled_flags : m.bare_flags
```
This is a strict ternary: **only `"tooled"` triggers tooled; everything else (bare, empty, even an
unrecognized string) → bare_flags.** The `default` arm faithfully implements this. An unknown mode
is a programming error from an internal caller (the only modes are the two exported constants), and
silently rendering bare is both spec-faithful and non-crashing. Erroring on unknown modes would
DIVERGE from the PRD ternary; we follow the spec.

---

## 4. The RenderMode Type + Constants (NEW, placed in render.go)

```go
// RenderMode selects which flag-set Manifest.Render appends: bare (tools off) or tooled (git tools on).
// See PRD §11.5 (two invocation modes) and §12.2 (rendering algorithm's mode ternary).
type RenderMode string

const (
	// RenderBare appends BareFlags — tools off, session-less, ephemeral. The DEFAULT mode (used when
	// Render's variadic mode is omitted). Serves the planner / message / arbiter roles.
	RenderBare RenderMode = "bare"

	// RenderTooled appends TooledFlags — tools on, git-scoped, non-interactive. Serves the stager role
	// (the only role that mutates the index). Errors if TooledFlags is empty (that provider cannot stager).
	RenderTooled RenderMode = "tooled"
)
```

**Placement:** in `render.go`, immediately BEFORE the `Render` method (after the `CmdSpec` type).
Rationale: RenderMode is Render's parameter type, so co-locating them is cohesive. It is NOT a manifest
field (do not put it in manifest.go). It MUST be exported (`RenderTooled`) so `internal/decompose`
(P3.M2.T3) can pass `provider.RenderTooled`.

**Type choice — `RenderMode` (typed) vs `string` (system_context.md's sketch):** the system_context
suggested `mode ...string`, but the **CONTRACT is authoritative** and specifies `mode ...RenderMode`
(a named string type). The named type is strictly safer (callers write `provider.RenderTooled`, not a
magic `"tooled"` literal) at zero runtime cost (a named string type IS a string under the hood). Follow
the contract.

---

## 5. Variable Naming in the Error Message — `m.Name` (not `r.Name`)

Render's value receiver is `m`; the resolved copy is `r := m.Resolve()`. The EXISTING error messages
in render.go all use `m.Name`:
- `return nil, fmt.Errorf("provider render %q: %w", m.Name, err)` (Validate propagation)
- `return nil, fmt.Errorf("provider render %q: unsupported prompt_delivery %q", m.Name, *r.PromptDelivery)`

The contract's error string is `provider %q: tooled mode requires non-empty tooled_flags` with `m.Name`.
Follow the existing convention: **`m.Name`**. (Resolve copies Name unchanged, so `m.Name == r.Name`
here, but consistency with the existing two error sites matters.) Note the error does NOT carry the
`provider render %q:` prefix the other two use — the contract's exact wording is
`provider %q: tooled mode requires non-empty tooled_flags`. Use the contract's wording verbatim.

---

## 6. Test Patterns to Mirror (render_test.go taxonomy)

The existing `render_test.go` is **same-package** (`package provider`) — it can call `builtinPi()`,
`strPtr`, `containsPair`, and the unexported `renderArgs` directly. Test style:

- **Table-driven** with `cases []struct{...}` + `t.Run(tc.name, …)` (see `TestRender_GoldenPerProvider`).
- **Golden args** asserted via `reflect.DeepEqual(spec.Args, tc.wantArgs)`.
- **Pair membership** via the `containsPair(args, flag, val)` helper (already defined at file bottom).
- **Error cases** via `if err == nil { t.Error("want error …") }` (see `TestRender_ValidateErrors`).

`renderArgs` (in `builtin_test.go:137`) is a **test-only bare-mode argv builder** — it does NOT take a
mode and only ever emits BareFlags. The mode change does NOT touch it, so `TestRender_CompatWithRenderArgs`
continues to pass unchanged (it compares Render's bare output to renderArgs's bare output).

### New tests to ADD (one focused test per new behavior, matching the taxonomy)

1. **`TestRender_DefaultModeIsBare`** — a provider with both BareFlags and TooledFlags; `Render(...)`
   with NO mode arg → Args contain BareFlags, NOT TooledFlags. (Proves backward compat + default.)
2. **`TestRender_ExplicitBareMode`** — same provider; `Render(..., RenderBare)` → identical to #1.
   (Proves explicit bare == default.)
3. **`TestRender_TooledModeAppendsTooledFlags`** — provider with non-empty TooledFlags;
   `Render(..., RenderTooled)` → Args contain TooledFlags, NOT BareFlags. (The stager path.)
4. **`TestRender_TooledModeEmptyFlagsErrors`** — provider with TooledFlags=nil (e.g. `builtinPi()` or
   a `Manifest{}`-style fixture with only BareFlags); `Render(..., RenderTooled)` → non-nil error
   whose message contains `tooled mode requires non-empty tooled_flags` AND the provider name.
5. (Optional belt-and-suspenders) **`TestRender_AllGoldenProvidersStillBareDefault`** — loop all 6
   builtins; assert each `Render(model, provider, sys, user)` (no mode) is byte-identical to before.
   (Guards against any accidental change to the bare path.)

A single helper manifest fixture with BOTH flag-sets set keeps tests #1–#4 tight:
```go
func dualModeManifest() Manifest {
	return Manifest{
		Name:       "dual",
		Command:    strPtr("agent"),
		BareFlags:  []string{"--no-tools"},
		TooledFlags: []string{"--allowed-tools", "git:*", "--approval-mode", "auto"},
	}
}
```

---

## 7. Doc-Comment Updates (contract: Mode A)

The contract requires updating the Render doc comment (and CmdSpec's, "if needed"). The two changes:

1. **Render doc comment** — add a paragraph documenting:
   - the new `mode ...RenderMode` parameter and its bare default;
   - that bare (the default) keeps every v1 caller unchanged;
   - that tooled appends TooledFlags and ERRORS if TooledFlags is empty (the "cannot stager" rule).
2. **Token-order comment block** — the existing comment lists `+ bare_flags...`; update it to
   `+ (mode==tooled ? tooled_flags : bare_flags)...` to reflect the mode ternary (PRD §12.2).

CmdSpec's doc comment needs NO change — CmdSpec is pure data and is unaware of how Args were chosen.
The contract's "if needed" caveat resolves to "not needed."

---

## 8. Decisions Log

| # | Question | Decision | Rationale |
|---|---|---|---|
| D1 | Variadic vs required positional `mode`? | **Variadic `mode ...RenderMode`** | Keeps all 16+ existing 4-arg callers unchanged (caller inventory §2). Contract-mandated. |
| D2 | Typed `RenderMode` vs plain `string`? | **Typed `RenderMode`** | Contract-mandated; type-safe (`provider.RenderTooled` vs magic string); zero runtime cost. system_context.md's `...string` was a sketch the contract overrides. |
| D3 | Where do RenderMode/constants live? | **`render.go`** (before Render) | It's Render's parameter type, not a manifest field. Cohesive; exported for `internal/decompose`. |
| D4 | Unknown/empty mode → error or bare? | **Bare (`default` arm)** | PRD §12.2 ternary `mode=="tooled" ? tooled : bare` is authoritative; only `"tooled"` triggers tooled. |
| D5 | Tooled + empty TooledFlags → ? | **Error** `provider %q: tooled mode requires non-empty tooled_flags` | Contract + PRD §12.1 ("nil/empty => cannot serve as stager"). Fail fast at render, not at exec. |
| D6 | `m.Name` or `r.Name` in the error? | **`m.Name`** | Matches the two existing error sites in render.go (consistency). Resolve copies Name unchanged. |
| D7 | Does Resolve need a TooledFlags change? | **No** | S1 already made Resolve leave TooledFlags as-is (nil→nil); `len(r.TooledFlags)==0` catches nil AND `[]`. |
| D8 | Does renderArgs (test helper) need a mode? | **No** | It's a bare-only test scaffold; `TestRender_CompatWithRenderArgs` compares bare↔bare, unaffected. |
| D9 | Touch CmdSpec doc? | **No** | CmdSpec is mode-agnostic pure data. Contract's "if needed" → not needed. |
