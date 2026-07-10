# Research Findings — P1.M1.T1.S2 (Extend negative gpt-5.4 guard + opencode case for stager-fallback)

## 1. CRITICAL PARALLEL-EDIT REALITY (read first)

S1 ("Implementing" in parallel) has ALREADY landed its full code fix AND over-delivered into S2's
agy-case territory. Current working tree (2026-07-10):

**bootstrap.go — S1's fix is COMPLETE:**
- bootstrap.go:185-186 — the blanking guard: `if stagerName == "pi" && stagerName != target { stagerModel = "" }`
- bootstrap.go:215-216 — the annotation guidance: `if stagerName == "pi" && stagerName != target && stagerModel == "" { stagerAnnotation += " pi is a multi-backend provider — prefix the model ..." }`

**bootstrap_test.go — S1 went BEYOND its PRP's stated scope (it marked these optional/S2):**
- Line 87: `assertContains(t, content, "[role.stager]", \`model = ""\`)` — DONE (S1 required)
- Lines 88-90: `if strings.Contains(content, \`gpt-5.4\`) { t.Errorf("agy stager-fallback ...") }` — DONE (S1's PRP said "DO NOT ... that breadth is S2")
- Lines 91-93: `if !strings.Contains(content, "multi-backend provider") { t.Error(...) }` — DONE (S1's PRP said "OPTIONAL")

So the agy-case parts of S2's contract (item 3a/3b/3c) are ALREADY SATISFIED by S1's over-reach. The
item description's line numbers (74-97, line 87 = gpt-5.4-mini, guard at 41-43 pi-only) are STALE —
they describe the pre-S1 state.

## 2. S2's UNIQUE, NON-CONFLICTING remaining value

The ONLY S2 contract item NOT yet done is item 3's tail: **"add a test case for
`buildBootstrapConfig("opencode", nil, nil)` with the same assertions"**. Plus the architecture doc's
explicit ask: "extend [the negative guard] to the stager-fallback test CASES" (plural — opencode AND
qwen-code, not just agy).

→ S2 must NOT re-edit S1's `TestBuildBootstrapConfig_AgyStagerFallback` (merge-conflict risk; it is
already correct). S2 adds NEW test coverage for the other stager-fallback providers.

## 3. StagerFallback mechanics (bootstrap.go:76-89) — what opencode/qwen-code produce

```go
func StagerFallback(target string, models map[string]string) (string, string) {
    if m := models["stager"]; m != "" { return target, m }   // target IS stager-capable
    for _, name := range preferredBuiltins {                  // target is NOT — find first stager-capable
        if col := DefaultModelsForProvider(name); col != nil && col["stager"] != "" {
            return name, col["stager"]                        // returns ("pi", "gpt-5.4-mini") — BARE
        }
    }
    return target, models["stager"]
}
```
For opencode/qwen-code/agy (empty `tooled_flags` ⇒ `models["stager"]==""`), this returns
`("pi", "gpt-5.4-mini")` — a BARE pi model. S1's guard then blanks the model to `""`. So:
- `buildBootstrapConfig("opencode", nil, nil)` → `[role.stager]` provider="pi", model="" (+ guidance). FIXED.
- OLD buildBootstrapConfig (no guard) → `[role.stager]` model="gpt-5.4-mini" (FR-R5b hard error). BUG.

## 4. Stager-fallback targets (empty tooled_flags — confirmed in builtin.go)

- agy — `TooledFlags: nil` (builtin.go:187-188, 216: "agy CANNOT serve as a stager")
- qwen-code — `TooledFlags: nil` (builtin.go:238-239, 263: "qwen-code cannot stager")
- opencode — empty tooled_flags (per Issue 1: "agy, opencode, qwen-code — every provider lacking tooled_flags")
- (codex, cursor also per S1's PRP — verified providers; the canonical built-in cases are agy/opencode/qwen-code)

The S2 table test should cover at least {agy, opencode, qwen-code}. agy re-asserts the generic
invariant (defense-in-depth; S1's agy test asserts agy-SPECIFIC models, the table asserts the
GENERIC cross-provider invariant — complementary, not duplicative).

## 5. The cleanest non-conflicting design: a NEW table-driven test

Add `TestBuildBootstrapConfig_StagerFallbackProviders_NoBarePiModel` (NEW function — zero merge
conflict with S1's agy test). Iterate {agy, opencode, qwen-code}, assert the GENERIC invariants:
- `[role.stager]` `provider = "pi"` (routing unchanged)
- `[role.stager]` `model = ""` (the blanked fallback)
- NO `gpt-5.4` ANYWHERE in the content (the generalized negative guard — would FAIL with old version)
- `multi-backend provider` guidance present (the stager annotation)
- `cannot serve as the stager` + `routed to pi` annotation present

This is the architecture doc's "extend the negative guard to the stager-fallback test cases",
materialized as ONE robust regression guard that would fail with the old buggy buildBootstrapConfig.

## 6. Complementary (optional, recommended): broaden TestBuildBootstrapConfig_ValidTOML

TestBuildBootstrapConfig_ValidTOML (line 143) has cases {pi, pi+claude, claude, claude+pi, agy} — it
does NOT include opencode or qwen-code. Adding {opencode, qwen-code} proves the bootstrapped config is
valid TOML for those targets (a free sanity check; the stager model="" + guidance comment must parse).
Clean additive table-row additions; no conflict.

## 7. Scope boundaries (do NOT do)
- Do NOT re-edit `TestBuildBootstrapConfig_AgyStagerFallback` (S1 owns it; already correct; conflict risk).
- Do NOT modify bootstrap.go (S1's fix is complete — S2 is TEST-ONLY per item point 5).
- Do NOT add the post-bootstrap ValidateModel regression net — that's P1.M1.T2.S1 (S2's tests are
  CONSUMED BY T2.S1, not replaced by it).
- Do NOT touch the commented-out pi block (Issue 2 = P1.M2.T1) or role_defaults.go.
- Do NOT change docs (item point 5: "DOCS: none — test-only changes").

## 8. Validation

Test-only. Gates: `go test ./internal/config/ -v -run TestBuildBootstrapConfig` (the item's exact
command), `go test ./internal/config/...`, `go build ./...`, `go vet`, `gofmt -l`, `make lint`,
`make test`. No external libs.
