# S2 Implementation Notes — Render test-call-site rewrite + FR-R5b + reasoning render tests

> Scope: P1.M1.T1.S2 — rewrite every `Manifest.Render` test call site to the v3 arity
> (`Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode)`), add the FR-R5b
> model-prefix tests, add reasoning-token render tests, and clear the `DefaultProvider` compile-breaks
> S1 introduces — so `internal/provider` + `internal/stubtest` test packages are green against v3 Render.
> Verified against live source 2026-07-01. **S1 is being implemented in parallel** (treat as a contract).

## 0. What S1 changes (the input contract)

- **Render signature**: `Render(model, provider, sysPrompt, userPayload, mode...)` →
  `Render(model, sysPrompt, userPayload, reasoning, mode...)`. The `provider` param is GONE; the
  inference provider is now the slash-PREFIX on `model` (FR-R5b). The `reasoning` param is NEW.
- **Manifest schema**: `DefaultProvider *string` REMOVED; `ReasoningLevels map[string][]string` ADDED.
- **FR-R5b at Render**: a `provider_flag` provider (pi) + `backend/model` → splits to
  `--provider <backend> --model <rest>`; a bare model (no `/`) on such a provider → HARD ERROR.
  Non-`provider_flag` providers (claude, gemini, opencode, …) pass the model VERBATIM (opencode's
  `openai/gpt-5.4` is its own combined form — NOT split).
- **FR-R6 at Render**: `reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0` → append tokens;
  else SILENT no-op (never an error; nil map reads are safe).
- S1's gate is `go build ./...` (production). S1 does NOT touch `_test.go`. So after S1, `go test` is
  RED on test compilation — **making it green is S2's whole job**.

## 1. The mechanical rewrite RULE (applies to ~30 of the ~41 sites)

Every existing call site passes the literal `""` for `provider` (scout §A/§E confirmed; verified).
The mechanical transform is:

```
4-arg v1:  Render(MODEL, "", SYS, USER)            →  Render(MODEL, SYS, USER, "off")
5-arg v1:  Render(MODEL, "", SYS, USER, MODE)      →  Render(MODEL, SYS, USER, "off", MODE)
```

- Drop the `""` provider arg (2nd positional).
- Insert `"off"` as the NEW reasoning arg (4th positional, after userPayload). "off" is a valid level
  that is a no-op on nil-ReasoningLevels builtins (len(nil)==0) — semantically cleaner than "".
- MODE (RenderBare/RenderTooled) stays LAST (variadic).

This rule covers: render_test.go Tests 1,3,4,7,8,9,11,12,13,14,15 (~20 sites), builtin_test.go claude
tooled (1), stubtest_test.go (~10). The stub manifest has NO ProviderFlag/DefaultModel → `Render("","","payload","off")`
is FR-R5b-safe (model="" skips the split). Verified.

## 2. The SEMANTIC reworks (5 tests — behavior changes under the model-prefix fold)

These tests' INTENT or ASSERTIONS change because pi's provider now comes from the model slash-prefix:

### (a) render_test.go Test 2 — `TestRender_Pi_ByteForByteCommitPi`
- OLD: `builtinPi().Render("glm-5-turbo", "zai", "<sys>", "<user>")` → `--provider zai --model glm-5-turbo …`
- NEW: `builtinPi().Render("zai/glm-5-turbo", "<sys>", "<user>", "off")` → SAME wantArgs
  (`--provider zai --model glm-5-turbo …`). The model string absorbs the provider; the GOLDEN argv is
  byte-identical. Update the comment (model-prefix, not provider param).

### (b) render_test.go Test 5 — `TestRender_ProviderDefaultFallback` → REPURPOSE to `TestRender_ModelPrefixFold`
- OBSOLETE premise: `DefaultProvider` field honored (`m.DefaultProvider = strPtr("zai")`). Field is GONE.
- REPURPOSE to prove the FR-R5b fold + verbatim:
  - pi + `"zai/glm-5.2"` → Args contain `--provider zai` + `--model glm-5.2` (fold).
  - opencode + `"openai/gpt-5.4"` → Args contain `-m openai/gpt-5.4` VERBATIM (NOT split — opencode has
    ProviderFlag=""); NO `--provider` token.
- REMOVE the `DefaultProvider: strPtr("zai")` Manifest literal (line 164) — field gone.

### (c) render_test.go Test 8b — `TestRender_FR5b_RejectsBareModelOnMultiProvider` → REWORK to v3 semantics
- OLD asserted the "no default_provider ⇒ bare model error" backstop (DefaultProvider-keyed). GONE.
- NEW v3 matrix (all use reasoning `"off"`):
  1. pi + `"glm-5.2"` (bare, no slash) → ERROR.
  2. pi-shaped manifest {ProviderFlag="--provider", DefaultModel="glm-5.2"} + model `""` (→ default_model,
     no slash) → ERROR. (REMOVE the `DefaultProvider: strPtr("")` field ref at line 251.)
  3. pi + `"zai/glm-5.2"` → OK; Args contain `--provider zai` + `--model glm-5.2`.
  4. pi + `""` (no model) → OK (no error; no --provider/--model).
  5. claude + `"sonnet"` (ProviderFlag="") → OK (verbatim, single-backend exempt).

### (d) builtin_test.go Test 19 — `TestBuiltinManifests_RenderedCommand_Pi_Tooled`
- OLD: `builtinPi().Render("glm-5-turbo", "zai", "<sys>", "<user>", RenderTooled)`.
- NEW: `builtinPi().Render("zai/glm-5-turbo", "<sys>", "<user>", "off", RenderTooled)` → SAME wantArgs
  (`--provider zai --model glm-5-turbo …` + tooled flags). Model-prefix fold; golden argv unchanged.

### (e) realagent_test.go `logResolvedCommand` (gated `//go:build integration_real`)
- OLD: `m.Render(cfg.Model, "", "<system prompt>", "<staged diff>")`.
- NEW: `m.Render(cfg.Model, "<system prompt>", "<staged diff>", "off")`.
- ALSO: lines 124-127 set `m.DefaultProvider = &ip` to inject the inference provider. Under v3 the
  provider is the model prefix — the test must fold `ip` into the model string instead (e.g.
  `cfg.Model = ip + "/" + baseModel`) OR drop the injection. Gated → does NOT affect normal `go test`.
  Rewrite for correctness under the tag; do not chase its runtime here.

## 3. The DefaultProvider COMPILE-BREAK cleanup (S1 removes the field — these won't compile)

S2 must remove/replace every `DefaultProvider` reference in the **internal/provider** test files (the
internal/provider package won't compile otherwise — Go compiles the whole package together):

| file | refs | action |
|------|------|--------|
| `manifest_test.go` | L70-72, L139, L394 (assert absent→nil; Resolve table entry) | DELETE the DefaultProvider assertions; (optional) add a ReasoningLevels-absent→nil assertion |
| `merge_test.go` | L22 `sampleBase` sets `DefaultProvider: strPtr("")`; L63 partial-override table entry | DELETE both (field gone; merge no longer has the block) |
| `builtin_test.go` | L235-240 (pi non-nil-empty assertion); L304/458/497/569/613/685 `assertNilStr("DefaultProvider",…)` | DELETE all; (optional) assert ReasoningLevels==nil for pi/claude (S1 leaves it nil+TODO) |
| `render_test.go` | L152/157/164 (Test 5), L239/251 (Test 8b) | Handled by the semantic reworks in §2 |

`registry_test.go` references `Registry.DefaultProvider(` — that's the auto-detect METHOD (unchanged by
S1), NOT the field. LEAVE it. `renderArgs` (builtin_test.go:166) is a manual argv builder that does NOT
reference DefaultProvider and does NOT call Render — it STAYS unchanged; its golden outputs are still
correct (`--provider zai --model glm-5-turbo`). `parse_test.go`/`executor_test.go` unaffected.

## 4. The NEW reasoning-token render tests (FR-R6) — add to render_test.go

```go
func TestRender_ReasoningTokensAppended(t *testing.T) {
	m := Manifest{Name:"r", Command:strPtr("agent"), ModelFlag:strPtr("--model"),
		ReasoningLevels: map[string][]string{"high": {"--thinking", "high"}}}
	// declared level → tokens appended after the model flag
	s, err := m.Render("m", "", "", "high")
	if err != nil { t.Fatalf("high: %v", err) }
	if !containsPair(s.Args, "--thinking", "high") { t.Errorf("reasoning tokens missing: %v", s.Args) }
	// "off" → no tokens, no error (off has no entry → nil slice → len 0)
	so, _ := m.Render("m", "", "", "off")
	if containsToken(so.Args, "--thinking") { t.Errorf("off should append no tokens: %v", so.Args) }
	// undeclared level → silent no-op, NEVER an error
	if _, err := m.Render("m", "", "", "medium"); err != nil { t.Errorf("undeclared level errored: %v", err) }
}

func TestRender_ReasoningNilTableNoOp(t *testing.T) {
	// nil ReasoningLevels + any level → no-op, no error (FR-R6 graceful; nil map reads are safe)
	m := Manifest{Name:"n", Command:strPtr("agent"), ModelFlag:strPtr("--model")}
	if _, err := m.Render("m", "", "", "high"); err != nil { t.Errorf("nil table + high errored: %v", err) }
}

func TestRender_ReasoningTooledMode(t *testing.T) {
	// reasoning tokens append in TOOLED mode too (RenderTooled path)
	m := dualModeManifest()
	m.ReasoningLevels = map[string][]string{"high": {"--reason", "high"}}
	s, err := m.Render("", "", "U", "high", RenderTooled)
	if err != nil { t.Fatalf("tooled+reasoning: %v", err) }
	if !containsPair(s.Args, "--reason", "high") { t.Errorf("reasoning tokens missing in tooled mode: %v", s.Args) }
}
```

## 5. Scope boundary — what is NOT S2's job (EXPECTED-red after S2)

- **`internal/generate/generate_test.go`** (TestCommitStaged_ResolvesSubProviderFromManifest, L421 sets
  `m.DefaultProvider`) and **`internal/decompose/*_test.go`** (roles/planner/stager/message/arbiter/
  decompose — all reference the removed `DefaultProvider` + the removed `inferenceProvider`/ResolveRoles
  guard). These packages will NOT compile under `go test` until **P1.M2.T1.S2** (ResolveRoles v3 +
  decompose callers). After S2, `go test ./...` is RED in `internal/generate` + `internal/decompose` —
  this is EXPECTED, not an S2 failure.
- **S2's gate is `go test ./internal/provider/ ./internal/stubtest/` GREEN** (the contract OUTPUT:
  "all internal/provider + stubtest tests green"). `go build ./...` stays green (S1's deliverable).

## 6. The stub manifest is FR-R5b-safe (verified)

`stubtest.Manifest(bin, opts)` (stubtest.go:102) sets NEITHER ProviderFlag NOR DefaultModel → the
`Render("","","payload","off")` calls in stubtest_test.go hit model="" → FR-R5b split is skipped
(modelToUse==""), no error. The ~10 sites are PURELY mechanical rewrites.

## 7. Sources

- `scout_render_callsites.md` §E (~41 census) + §A (provider is literal `""` everywhere).
- `P1M1T1S1/PRP.md` — the v3 Render signature + FR-R5b fold/error + FR-R6 emit contract.
- PRD §12.2 (rendering algorithm), §9.15 FR-R5b (model-prefix) + FR-R6 (reasoning).
- `internal/provider/{render,render_test,builtin_test,manifest_test,merge_test}.go`, `internal/stubtest/stubtest_test.go`.
