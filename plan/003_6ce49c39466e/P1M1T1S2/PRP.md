---
name: "P1.M1.T1.S2 — Rewrite the ~41 Render test call sites + FR-R5b (model-prefix) + reasoning render tests"
description: |
  Test-only completion of the v3 Render keystone. Rewrite every `Manifest.Render` test call site to the
  new arity (`Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode)` — drop the
  dead `""` provider arg; insert `"off"` reasoning), rework the FR-R5b tests to the model-prefix
  semantics (pi + `backend/model` → fold; bare model → error; single-backend → verbatim), add reasoning-
  token render tests (FR-R6: declared level appends tokens; absent/nil ⇒ silent no-op, never an error),
  and clear the `DefaultProvider` compile-breaks S1 introduces — so the `internal/provider` +
  `internal/stubtest` test packages are GREEN against v3 Render. S1 lands the production signature; S2
  is what makes the tests compile + pass again. No production code, no docs.
---

## Goal

**Feature Goal**: Bring every test that exercises `Manifest.Render` into the v3 world — mechanical
arity rewrites for the ~30 trivial sites, semantic reworks for the 5 sites whose behavior changes under
the FR-R5b model-prefix fold, three new FR-R6 reasoning-token render tests, and removal of the now-gone
`DefaultProvider` field references — so `go test ./internal/provider/ ./internal/stubtest/` is green and
permanently codifies the FR-R5b (model-prefix) + FR-R6 (reasoning) contracts.

**Deliverable** (test files only; no production code, no docs):
1. `internal/provider/render_test.go` — rewrite all ~22 Render call sites (mechanical for ~17; semantic
   for Test 2, Test 5→`ModelPrefixFold`, Test 8b→v3 FR-R5b matrix); add 3 reasoning render tests.
2. `internal/provider/builtin_test.go` — rewrite the 2 tooled Render sites (pi = model-prefix fold);
   delete the 8 `DefaultProvider` assertions.
3. `internal/provider/manifest_test.go` — delete the `DefaultProvider` assertions (3 sites).
4. `internal/provider/merge_test.go` — delete the `DefaultProvider` refs (sampleBase + 1 table entry).
5. `internal/stubtest/stubtest_test.go` — mechanical rewrite of ~10 Render call sites.
6. `internal/generate/realagent_test.go` — rewrite the 1 (gated) Render site + its DefaultProvider injection.

**Success Definition**: `go test ./internal/provider/ ./internal/stubtest/` is GREEN (compile + pass);
the FR-R5b fold (`zai/glm-5.2` → `--provider zai --model glm-5.2`) and no-slash error are asserted; the
verbatim single-backend path is asserted; FR-R6 reasoning tokens append when declared and no-op when
absent/nil (never error); no production file and no doc is modified; `go build ./...` stays green.

## User Persona

**Target User**: The Stagecoach contributor landing v3 (P1.M2 per-role reasoning + ResolveRoles v3, P1.3
config migration) and the reviewer verifying the FR-R5b/FR-R6 contracts are enforced. Test-only — no
end-user surface.

**Use Case**: After S1 changed the Render signature, the test tree does not compile. S2 is the
mechanical + semantic repair that restores a green test suite and adds the permanent regression nets for
the two new rendering contracts (model-prefix routing, reasoning effort).

**Pain Points Addressed**: Removes the post-S1 "tests don't compile" gap; codifies FR-R5b (so a future
regression to a silent bare `--model` is caught) and FR-R6 (so a missing reasoning level never silently
errors or silently drops tokens).

## Why

- **S1 is the keystone; S2 is its test mirror.** S1 changed the single command-emission chokepoint
  (`Render`) and removed `DefaultProvider`. `go build ./...` is green, but `go test` is red on test
  compilation. S2 restores green and locks in the new contracts — without it, the keystone has no
  executable verification.
- **FR-R5b and FR-R6 are safety-critical invariants.** FR-R5b prevents an unroutable command (bare model
  on a multi-backend agent); FR-R6 makes reasoning effort a graceful no-op. Both deserve dedicated render
  tests so a regression is caught at the chokepoint, not in production.
- **Low-risk, high-coverage test work.** ~30 of the ~41 sites are a mechanical arity transform (provider
  is the literal `""` everywhere — scout §A/§E confirmed); only 5 need semantic rework; 3 are net-new.
  The `renderArgs` golden helper stays unchanged (its outputs are still correct).

## What

A test-only change set: (1) mechanical arity rewrites, (2) five semantic FR-R5b reworks, (3) three new
reasoning render tests, (4) `DefaultProvider` compile-break cleanup across 4 internal/provider test
files. No production file, no doc, no config.

### Success Criteria

- [ ] `go test ./internal/provider/ ./internal/stubtest/` is GREEN (the S2 gate / contract OUTPUT).
- [ ] Every test `Render` call site uses the v3 arity (`model, sysPrompt, userPayload, reasoning, mode...`);
      no `provider` arg remains; existing cases pass `reasoning="off"`.
- [ ] FR-R5b fold: pi + `zai/glm-5.2` → Args contain `--provider zai` + `--model glm-5.2` (golden argv byte-identical to v1).
- [ ] FR-R5b error: pi + bare `glm-5.2` (no `/`) → returns an error; pi-shaped {DefaultModel="glm-5.2"} + `""` → error.
- [ ] FR-R5b verbatim: opencode + `openai/gpt-5.4` (no provider_flag) → `-m openai/gpt-5.4`, NOT split; claude + `sonnet` → `--model sonnet`.
- [ ] FR-R6: declared level → tokens appended (incl. in tooled mode); absent level / nil table → silent no-op, never an error.
- [ ] No `DefaultProvider` reference remains in any `internal/provider/*_test.go`.
- [ ] `go build ./...` stays green; no production file or doc modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives the exact mechanical rewrite rule, the per-test semantic reworks
(with before/after for all 5), the full code of the 3 new reasoning tests, the per-file `DefaultProvider`
cleanup table, the exact green-scope boundary (and why generate/decompose stay red), and the executable
validation (`go test ./internal/provider/ ./internal/stubtest/`). The call-site census + the stub-
manifest-is-safe finding are pre-resolved in the research note.

### Documentation & References

```yaml
# MUST READ — the v3 keystone contract (S1, in parallel)
- docfile: plan/003_6ce49c39466e/P1M1T1S1/PRP.md
  why: "Defines the new Render signature + FR-R5b (model-prefix fold + no-slash error) + FR-R6 (reasoning emit) + the DefaultProvider removal + ReasoningLevels addition. S2's tests must assert exactly these."
  critical: "Split on FIRST '/' only, only when *r.ProviderFlag != \"\". opencode (ProviderFlag=\"\") takes 'openai/gpt-5.4' VERBATIM. Reasoning is `reasoning != \"\" && len(r.ReasoningLevels[reasoning]) > 0` → append; else no-op."

- docfile: plan/003_6ce49c39466e/architecture/scout_render_callsites.md
  why: "§E enumerates the ~41 test call sites (render_test ~22, builtin_test 2, realagent_test 1, stubtest_test ~10); §A confirms provider is the literal \"\" at every non-test site (low-risk removal)."
  critical: "§E + §F: S2's scope is the test call sites. The DefaultProvider field removal ALSO breaks manifest_test/merge_test/builtin_test compile (the whole internal/provider package compiles together) — those cleanups are S2's too."

- docfile: plan/003_6ce49c39466e/P1M1T1S2/research/s2_implementation_notes.md
  why: "Distilled S2 findings: the mechanical rewrite RULE, the 5 semantic reworks (before/after), the per-file DefaultProvider cleanup table, the 3 new reasoning tests (full code), the stub-manifest-is-FR-R5b-safe finding, and the green-scope boundary (generate/decompose = P1.M2.T1.S2)."
  critical: "S2's gate is `go test ./internal/provider/ ./internal/stubtest/` — NOT `go test ./...` (generate/decompose tests stay red until P1.M2.T1.S2). realagent_test.go is GATED (//go:build integration_real) so it does not affect normal builds."

# The edit targets (test files)
- file: internal/provider/render_test.go
  why: "EDIT (largest). ~22 Render sites. Mechanical rewrite for Tests 1,3,4,7,8,9,11,12,13,14,15; SEMANTIC for Test 2 (model→zai/glm-5-turbo), Test 5 (→ModelPrefixFold), Test 8b (→v3 FR-R5b matrix). ADD 3 reasoning tests. Reuses containsPair/containsToken/dualModeManifest helpers."
  pattern: "v3 call: `m.Render(MODEL, SYS, USER, \"off\")` / `…, \"off\", RenderTooled)`. Golden wantArgs for the fold cases are byte-IDENTICAL to v1 (the prefix just changes HOW they're produced)."

- file: internal/provider/builtin_test.go
  why: "EDIT. 2 tooled Render sites (Test 19 pi = model-prefix fold to `zai/glm-5-turbo`; Test 20 claude = mechanical). DELETE 8 DefaultProvider assertions (L235-240 + 6 assertNilStr calls). `renderArgs` helper (L166) STAYS — it's a manual argv builder, no DefaultProvider ref, golden outputs still correct."
  gotcha: "Do NOT change renderArgs or its ~7 callers — they build EXPECTED argv manually and don't call Render. Only the 2 direct Render calls + the DefaultProvider assertions change."

- file: internal/provider/manifest_test.go
  why: "EDIT (compile-fix). DELETE the DefaultProvider assertions (L70-72 absent→nil; L139 assertNilStr; L394 Resolve table entry). Optional: assert ReasoningLevels stays nil through Resolve."
- file: internal/provider/merge_test.go
  why: "EDIT (compile-fix). DELETE `DefaultProvider: strPtr(\"\")` in sampleBase (L22) + the `{\"DefaultProvider\",…}` table entry (L63). Optional: a ReasoningLevels fresh-map merge test (S1 added the merge)."
- file: internal/stubtest/stubtest_test.go
  why: "EDIT (purely mechanical). ~10 sites `m.Render(\"\",\"\",\"\",\"payload\")` → `m.Render(\"\",\"\", \"payload\", \"off\")`. The stub manifest sets no ProviderFlag/DefaultModel → FR-R5b-safe (model=\"\" skips the split)."
- file: internal/generate/realagent_test.go
  why: "EDIT (gated //go:build integration_real). Rewrite the 1 Render site (L78) to v3 arity; the DefaultProvider injection (L124-127) becomes a model-prefix fold. Gated → excluded from normal `go test`."

# Read-only refs (do NOT edit in S2)
- file: internal/provider/render.go
  why: "READ-ONLY (S1 owns it). After S1 it implements FR-R5b/FR-R6. S2 writes tests against it; do NOT edit it."
- file: internal/provider/builtin.go
  why: "READ-ONLY. builtinPi (ProviderFlag=\"--provider\", DefaultModel=\"\", TooledFlags non-empty) + builtinClaude (ProviderFlag=\"\", DefaultModel=\"sonnet\"). Confirms which providers fold vs pass-verbatim."
- file: internal/stubtest/stubtest.go
  why: "READ-ONLY. `Manifest(bin, opts)` sets NO ProviderFlag/DefaultModel → the stub Render calls are FR-R5b-safe (verified)."

# PRD authority
- url: PRD.md §12.2 (rendering algorithm), §9.15 FR-R5b (model-prefix) + FR-R6 (reasoning)
  why: "The exact fold rule, the no-slash error wording, and the reasoning no-op contract the tests must assert."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── internal/provider/
│   ├── render.go          # READ-ONLY (S1) — v3 Render with FR-R5b fold/error + FR-R6 emit
│   ├── render_test.go     # EDIT — ~22 sites + 3 new reasoning tests
│   ├── builtin_test.go    # EDIT — 2 tooled sites + 8 DefaultProvider assertion deletions
│   ├── manifest_test.go   # EDIT — DefaultProvider assertion deletions (compile-fix)
│   ├── merge_test.go      # EDIT — sampleBase + table DefaultProvider deletions (compile-fix)
│   ├── builtin.go         # READ-ONLY — field shapes (fold vs verbatim)
│   └── registry_test.go   # READ-ONLY — uses DefaultProvider METHOD (not the field), unaffected
├── internal/stubtest/stubtest_test.go   # EDIT — ~10 mechanical rewrites
└── internal/generate/realagent_test.go  # EDIT (gated) — 1 site + DefaultProvider→model-prefix
```

### Desired Codebase Tree After S2

```bash
stagecoach/
└── (only TEST files modified — no production, no docs, no new files)
    internal/provider/{render,builtin,manifest,merge}_test.go
    internal/stubtest/stubtest_test.go
    internal/generate/realagent_test.go   # gated
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/provider/render_test.go` | MODIFY | v3 arity rewrites + 5 semantic FR-R5b reworks + 3 new reasoning tests. **Largest edit.** |
| `internal/provider/builtin_test.go` | MODIFY | 2 tooled Render rewrites (pi = model-prefix) + 8 DefaultProvider assertion deletions. |
| `internal/provider/manifest_test.go` | MODIFY | DefaultProvider assertion deletions (compile-fix). |
| `internal/provider/merge_test.go` | MODIFY | sampleBase + table DefaultProvider deletions (compile-fix). |
| `internal/stubtest/stubtest_test.go` | MODIFY | ~10 mechanical Render arity rewrites. |
| `internal/generate/realagent_test.go` | MODIFY | 1 gated Render rewrite + DefaultProvider→model-prefix. |

**Explicitly NOT touched**: `internal/provider/render.go` + all production files (S1's deliverable),
`internal/provider/{parse,executor,registry,merge,manifest,builtin}.go` (production — S1 owns the
schema/merge/builtin edits), `internal/generate/generate_test.go` + `internal/decompose/*_test.go`
(P1.M2.T1.S2 — these reference DefaultProvider/inferenceProvider and stay red after S2 by design),
`PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL — the mechanical rule is positional, NOT named. The new arity REORDERS args:
//   v1: Render(MODEL, provider, SYS, USER [, MODE])
//   v3: Render(MODEL, SYS, USER, reasoning [, MODE])
// The provider arg (always literal "" at test sites) is DROPPED; reasoning ("off") is INSERTED at the
// new 4th position. MODE stays LAST (variadic). Get the position wrong and a stray "" lands as the model.

// CRITICAL — pi FOLDS the model prefix; claude/gemini/opencode do NOT. The fold changes the INPUT for
// pi tests: v1 `Render("glm-5-turbo", "zai", …)` becomes v3 `Render("zai/glm-5-turbo", …)`. The GOLDEN
// argv (--provider zai --model glm-5-turbo …) is byte-identical — only the input model string changes.
// Single-backend tests (claude "sonnet", opencode "openai/gpt-5.4") pass the model VERBATIM (no fold).

// CRITICAL — DefaultProvider is GONE (S1). Any `m.DefaultProvider` / `DefaultProvider: strPtr(…)` /
// `assertNilStr("DefaultProvider",…)` in internal/provider/*_test.go is a COMPILE ERROR. The whole
// internal/provider package compiles together, so manifest_test/merge_test/builtin_test breaks block
// render_test too — fix ALL of them, not just the Render-call files.

// CRITICAL (scope): S2's green gate is `go test ./internal/provider/ ./internal/stubtest/`. Do NOT chase
// `go test ./...` — internal/generate/generate_test.go + internal/decompose/*_test.go reference the removed
// DefaultProvider/inferenceProvider and stay red until P1.M2.T1.S2. That red is EXPECTED, not an S2 bug.

// GOTCHA — `renderArgs` (builtin_test.go:166) is a MANUAL argv builder (does not call Render, does not
// reference DefaultProvider). Its golden outputs are still correct. Do NOT rewrite it or its ~7 callers —
// only the 2 direct Render calls + the DefaultProvider assertions in builtin_test.go change.

// GOTCHA — `registry_test.go` calls `r.DefaultProvider(installed)` — that's the auto-detect METHOD on
// Registry (unchanged by S1), NOT the removed Manifest field. LEAVE it.

// GOTCHA — reasoning="off" (not "") is the contract value for existing cases. It's a valid level that is
// a no-op on nil-ReasoningLevels builtins (len(nil)==0). Using "off" (not "") is semantically cleaner and
// matches the contract's "use 'off' for existing cases".

// GOTCHA — the stub manifest (stubtest.Manifest) sets NO ProviderFlag/DefaultModel, so its
// Render("","","payload","off") calls are FR-R5b-safe (model="" skips the split). Verified — purely mechanical.
```

## Implementation Blueprint

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: render_test.go — MECHANICAL arity rewrites (Tests 1,3,4,7,8,9,11,12,13,14,15)
  - RULE: Render(MODEL, "", SYS, USER) → Render(MODEL, SYS, USER, "off"); add ", RenderBare/RenderTooled" after "off" where a mode was present.
  - Test 1 GoldenPerProvider: ALSO drop the `provider string` field from the case struct + the call
    (tc.m.Render(tc.model, "<sys>", "<user>", "off")). wantArgs UNCHANGED for all 6 (verified: pi model=""
    skips the fold; others are provider_flag="" → verbatim).
  - Tests 3,4,7,9,10,11,12,13,14,15: apply the rule. Verify wantArgs/wantStdin are UNCHANGED
    (these tests use model="" or single-backend providers → no fold, no error).
  - Test 8 ValidateErrors (2 sites): apply the rule (error cases still error for the same Validate reasons).

Task 2: render_test.go — SEMANTIC rework: Test 2 (Pi byte-for-byte)
  - CHANGE the call: builtinPi().Render("zai/glm-5-turbo", "<sys>", "<user>", "off")
  - wantArgs UNCHANGED: ["--provider","zai","--model","glm-5-turbo","--system-prompt","<sys>", …, "-p"]
  - Update the comment: model-prefix fold (provider absorbed into the model string).

Task 3: render_test.go — SEMANTIC rework: Test 5 → TestRender_ModelPrefixFold
  - REPLACE the obsolete DefaultProvider-honored premise. New body:
      // pi + "zai/glm-5.2" (provider_flag) → fold to --provider zai --model glm-5.2
      s,_ := builtinPi().Render("zai/glm-5.2", "<sys>", "<user>", "off")
      if !containsPair(s.Args,"--provider","zai") || !containsPair(s.Args,"--model","glm-5.2") ||
         containsToken(s.Args,"zai/glm-5.2") { t.Errorf("fold: %v", s.Args) }
      // opencode (no provider_flag) + "openai/gpt-5.4" → VERBATIM, NOT split
      o,_ := builtinOpenCode().Render("openai/gpt-5.4", "<sys>", "<user>", "off")
      if !containsPair(o.Args,"-m","openai/gpt-5.4") || containsToken(o.Args,"--provider") {
          t.Errorf("opencode should pass model verbatim (no --provider): %v", o.Args) }
  - DELETE the Manifest{… DefaultProvider: strPtr("zai")} literal (line 164) — field gone.

Task 4: render_test.go — SEMANTIC rework: Test 8b → v3 FR-R5b matrix
  - RENAME intent stays (rejects bare model on a provider_flag provider). New body (all reasoning "off"):
      pi := builtinPi()
      // (1) bare model, no slash → ERROR
      if _, err := pi.Render("glm-5.2", "<sys>", "<user>", "off"); err == nil { t.Fatal("want no-slash error") }
      // (2) default_model path, no slash → ERROR (pi-shaped manifest)
      m := Manifest{Name:"pi", Command:strPtr("pi"), PromptDelivery:strPtr("stdin"),
          ProviderFlag:strPtr("--provider"), ModelFlag:strPtr("--model"), DefaultModel:strPtr("glm-5.2")}
      if _, err := m.Render("", "<sys>", "<user>", "off"); err == nil { t.Fatal("want no-slash error on default_model") }
      // (3) fold success
      s,err := pi.Render("zai/glm-5.2", "<sys>", "<user>", "off")
      if err != nil || !containsPair(s.Args,"--provider","zai") || !containsPair(s.Args,"--model","glm-5.2") {
          t.Errorf("fold: err=%v args=%v", err, s.Args) }
      // (4) no model → OK (skips the split)
      if _, err := pi.Render("", "<sys>", "<user>", "off"); err != nil { t.Errorf("no model should be OK: %v", err) }
      // (5) single-backend (claude) + bare model → OK (verbatim)
      if _, err := builtinClaude().Render("sonnet", "<sys>", "<user>", "off"); err != nil {
          t.Errorf("claude bare model should be OK: %v", err) }
  - DELETE the `DefaultProvider: strPtr("")` field ref (line 251).

Task 5: render_test.go — ADD 3 reasoning-token render tests (FR-R6)
  - TestRender_ReasoningTokensAppended: manifest {ReasoningLevels:{"high":{"--thinking","high"}}};
    "high" → containsPair(--thinking,high); "off" → no tokens; "medium" (undeclared) → no error.
  - TestRender_ReasoningNilTableNoOp: nil ReasoningLevels + "high" → no-op, no error.
  - TestRender_ReasoningTooledMode: dualModeManifest()+ReasoningLevels; RenderTooled + "high" → tokens appended.
  (Full code in the research note §4; reuse containsPair/containsToken/dualModeManifest.)

Task 6: builtin_test.go — 2 tooled Render rewrites + 8 DefaultProvider deletions
  - Test 19 (pi tooled): Render("zai/glm-5-turbo", "<sys>", "<user>", "off", RenderTooled); wantArgs UNCHANGED.
  - Test 20 (claude tooled): Render("sonnet", "<sys>", "<user>", "off", RenderTooled); wantArgs UNCHANGED.
  - DELETE: L235-240 (pi DefaultProvider non-nil-empty block) + the 6 assertNilStr("DefaultProvider",…) calls.
  - renderArgs helper + its callers: UNCHANGED (manual builder; golden outputs still correct).

Task 7: manifest_test.go + merge_test.go — DefaultProvider compile-fix
  - manifest_test.go: DELETE L70-72, L139, L394 (DefaultProvider assertions). (Optional: ReasoningLevels-nil assertion.)
  - merge_test.go: DELETE `DefaultProvider: strPtr("")` in sampleBase (L22) + the {"DefaultProvider",…} table entry (L63).

Task 8: stubtest_test.go — ~10 mechanical rewrites
  - RULE: m.Render("","","","payload") → m.Render("", "", "payload", "off") for all ~10 sites.
  - Stub manifest has no ProviderFlag/DefaultModel → FR-R5b-safe (verified). No assertion changes.

Task 9: realagent_test.go — gated (//go:build integration_real) rewrite
  - logResolvedCommand (L78): m.Render(cfg.Model, "<system prompt>", "<staged diff>", "off").
  - DefaultProvider injection (L124-127): fold the inference provider into the model string (ip+"/"+model)
    instead of m.DefaultProvider=&ip. Gated → excluded from normal `go test`; rewrite for tag-correctness.

Task 10: VALIDATE (S2's gate)
  - RUN: go test ./internal/provider/ ./internal/stubtest/      # GREEN (the contract OUTPUT)
  - RUN: go build ./...                                         # stays GREEN (no production touched)
  - RUN: gofmt -l internal/provider/*_test.go internal/stubtest/*_test.go internal/generate/realagent_test.go  # empty
  - EXPECT: go test ./... is RED in internal/generate(generate_test.go) + internal/decompose — P1.M2.T1.S2's scope.
  - FIX-FORWARD: read failures, fix, re-run. Do NOT expand into generate_test.go / decompose tests.
```

### Implementation Patterns & Key Details

```go
// === The mechanical arity transform (the rule for ~30 sites) ===
// v1 4-arg:   m.Render(MODEL, "", SYS, USER)            →  m.Render(MODEL, SYS, USER, "off")
// v1 5-arg:   m.Render(MODEL, "", SYS, USER, MODE)      →  m.Render(MODEL, SYS, USER, "off", MODE)
// Drop the "" provider arg (always literal "" at test sites); insert "off" reasoning; MODE stays last.

// === The model-prefix fold (pi only) — golden argv UNCHANGED, input model absorbs the provider ===
// v1: builtinPi().Render("glm-5-turbo", "zai", sys, user)        → "--provider zai --model glm-5-turbo …"
// v3: builtinPi().Render("zai/glm-5-turbo", sys, user, "off")    → "--provider zai --model glm-5-turbo …"  (IDENTICAL)

// === FR-R6 reasoning test skeleton (full code in research note §4) ===
m := Manifest{Name:"r", Command:strPtr("agent"), ModelFlag:strPtr("--model"),
	ReasoningLevels: map[string][]string{"high": {"--thinking", "high"}}}
s,_ := m.Render("m", "", "", "high")           // declared → tokens appended
if !containsPair(s.Args, "--thinking", "high") { t.Errorf("...") }
_, err := m.Render("m", "", "", "medium")      // undeclared → no-op, NEVER error
if err != nil { t.Errorf("undeclared level must not error: %v", err) }
```

### Integration Points

```yaml
TEST GATE (S2's deliverable):
  - "go test ./internal/provider/ ./internal/stubtest/"  → GREEN (compile + pass)
  - "go build ./..."                                     → stays GREEN (no production touched)

TEST FILES EDITED (test-only):
  - internal/provider/render_test.go       # arity rewrites + 5 semantic FR-R5b reworks + 3 reasoning tests
  - internal/provider/builtin_test.go      # 2 tooled rewrites + 8 DefaultProvider deletions
  - internal/provider/manifest_test.go     # DefaultProvider deletions (compile-fix)
  - internal/provider/merge_test.go        # sampleBase + table DefaultProvider deletions (compile-fix)
  - internal/stubtest/stubtest_test.go     # ~10 mechanical rewrites
  - internal/generate/realagent_test.go    # 1 gated rewrite + DefaultProvider→model-prefix

NO-TOUCH (explicitly — owned by other subtasks):
  - internal/provider/render.go + ALL production   # S1 (the keystone); S2 only writes tests against it
  - internal/generate/generate_test.go             # P1.M2.T1.S2 (ResolveRoles v3 + decompose callers)
  - internal/decompose/*_test.go                   # P1.M2.T1.S2 (DefaultProvider/inferenceProvider refs)
  - docs/*.md, PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by LATER subtasks):
  - P1.M2.T1.S2: fixes generate_test.go + decompose/*_test.go (DefaultProvider → model-prefix; RestoreRoles v3) → then `go test ./...` is fully green
  - P1.M2.T1.S1/S2: wire REAL per-role reasoning into the 6 production Render call sites (S1 leaves reasoning="")
```

## Validation Loop

### Level 1: Compile (the immediate S2 deliverable)

```bash
cd /home/dustin/projects/stagecoach

# S2's GREEN gate — the two in-scope packages compile + pass
go test ./internal/provider/ ./internal/stubtest/
# Expected: ok provider / ok stubtest. If compile fails, it is almost always a stray DefaultProvider
# reference or a mis-positioned "off" reasoning arg — read the line, fix, re-run.

# Production untouched → still green
go build ./...
# Expected: exit 0.

gofmt -l internal/provider/*_test.go internal/stubtest/*_test.go internal/generate/realagent_test.go
# Expected: empty (run gofmt -w on any listed file).
```

### Level 2: FR-R5b + FR-R6 contracts (the new assertions)

```bash
cd /home/dustin/projects/stagecoach

# Run the reworked + new tests by name
go test ./internal/provider/ -v -run 'TestRender_FR5b|TestRender_ModelPrefixFold|TestRender_Reasoning|TestRender_Pi_ByteForByte'
# Expected: PASS — fold produces --provider/--model; bare model errors; reasoning tokens append/ no-op.

# Confirm NO DefaultProvider reference survives in internal/provider tests
grep -rn "DefaultProvider" internal/provider/*_test.go
# Expected: NO output (registry_test.go uses the METHOD DefaultProvider( — that's in registry_test.go and is fine; verify it's only the method-call form).
```

### Level 3: Regression (all existing provider tests still pass)

```bash
cd /home/dustin/projects/stagecoach

go test ./internal/provider/ -v
# Expected: ALL provider tests PASS (golden-per-provider, prepend fallback, model default, env, flag
# delivery, validate, mode ternary, compat-with-renderArgs, non-mutate). The golden argv for the fold
# cases is byte-identical to v1.

go test ./internal/stubtest/ -v
# Expected: ALL stub tests PASS (the Execute seam is unaffected — CmdSpec unchanged).
```

### Level 4: Scope discipline (the expected-red boundary)

```bash
cd /home/dustin/projects/stagecoach

# These are EXPECTED to be RED after S2 — P1.M2.T1.S2 owns them. Do NOT chase.
go test ./internal/generate/ 2>&1 | grep -i "defaultprovider\|cannot" | head   # generate_test.go L421
go test ./internal/decompose/ 2>&1 | grep -i "defaultprovider\|inferenceprovider\|cannot" | head
# Expected: compile errors referencing DefaultProvider / inferenceProvider. This is the S2↔P1.M2 boundary.

# Confirm NO production file was modified by S2
git diff --stat -- internal/provider/render.go internal/provider/manifest.go internal/provider/merge.go \
   internal/provider/builtin.go internal/provider/executor.go internal/decompose/ internal/generate/generate.go \
   pkg/stagecoach/
# Expected: nothing (S2 is test-only).

# Confirm S2 did NOT touch generate_test.go / decompose tests (out of scope)
git diff --stat -- internal/generate/generate_test.go internal/decompose/*_test.go
# Expected: nothing.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go test ./internal/provider/ ./internal/stubtest/` is GREEN (S2's gate).
- [ ] `go build ./...` stays GREEN (no production touched).
- [ ] `gofmt -l` on edited test files reports nothing.

### Feature Validation

- [ ] Every test `Render` call uses v3 arity (`model, sys, user, "off"[, mode]`); no `provider` arg.
- [ ] FR-R5b fold: pi + `zai/glm-5.2` → `--provider zai --model glm-5.2` (golden byte-identical).
- [ ] FR-R5b error: pi + bare `glm-5.2` → error; pi-shaped {DefaultModel="glm-5.2"} + `""` → error.
- [ ] FR-R5b verbatim: opencode `openai/gpt-5.4` → `-m openai/gpt-5.4` (not split); claude `sonnet` → `--model sonnet`.
- [ ] FR-R6: declared level appends tokens (incl. tooled mode); absent/nil level → no-op, never error.
- [ ] No `DefaultProvider` reference in `internal/provider/*_test.go` (grep clean, except the registry METHOD call).

### Scope Discipline Validation

- [ ] ONLY the 6 test files in the edit list are modified (`git diff --stat` confirms).
- [ ] Did NOT edit any production file (render.go/manifest.go/merge.go/builtin.go/executor.go/decompose/generate).
- [ ] Did NOT edit `generate_test.go` or `internal/decompose/*_test.go` (P1.M2.T1.S2).
- [ ] Did NOT edit `docs/*.md`, `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.
- [ ] Did NOT rewrite `renderArgs` or its callers (manual builder; outputs unchanged).

### Code Quality Validation

- [ ] Mechanical rewrites use the exact positional rule (reasoning "off" at position 4; MODE last).
- [ ] The 5 semantic reworks preserve the golden argv (fold changes input, not output).
- [ ] New reasoning tests reuse `containsPair`/`containsToken`/`dualModeManifest` (no new helpers needed).
- [ ] DefaultProvider cleanup is complete across all 4 internal/provider test files (whole-package compile).

---

## Anti-Patterns to Avoid

- ❌ Don't mis-position the reasoning arg. The v3 arity is `(model, sys, user, reasoning, mode...)` —
  reasoning is the NEW 4th positional, MODE stays last (variadic). Dropping `""` provider and adding
  `"off"` in the wrong slot yields a stray empty model/sys.
- ❌ Don't change the golden `wantArgs` for the fold cases — the model-prefix fold changes the INPUT model
  string (`zai/glm-5-turbo`), not the OUTPUT argv (`--provider zai --model glm-5-turbo …` stays identical).
- ❌ Don't split the model for non-`provider_flag` providers. opencode `openai/gpt-5.4`, claude `sonnet`,
  gemini `gemini-2.5-pro` pass VERBATIM. Only pi (ProviderFlag="--provider") folds.
- ❌ Don't leave ANY `DefaultProvider` reference in `internal/provider/*_test.go` — the field is gone; the
  whole package compiles together, so one stray reference blocks every test file in the package.
- ❌ Don't rewrite `renderArgs` or its ~7 callers — it's a manual argv builder (no Render call, no
  DefaultProvider ref); its golden outputs are still correct. Only the 2 direct Render calls change.
- ❌ Don't chase `go test ./...` green. `generate_test.go` + `internal/decompose/*_test.go` reference the
  removed `DefaultProvider`/`inferenceProvider` and stay red until P1.M2.T1.S2 — that is EXPECTED. S2's
  gate is `go test ./internal/provider/ ./internal/stubtest/`.
- ❌ Don't expand into production (render.go etc.) — S1 owns the keystone; S2 only writes tests against it.
  If a test reveals a Render bug, that's an S1 issue, not an S2 edit.
- ❌ Don't make a missing reasoning level an error in the new tests — FR-R6 is a SILENT no-op. Assert
  `err == nil` for an undeclared level / nil table.
- ❌ Don't use `""` for the reasoning arg in existing cases — the contract says `"off"` (a valid level,
  cleaner than empty; both no-op on nil ReasoningLevels, but "off" is the documented convention).
- ❌ Don't touch `realagent_test.go`'s runtime — it's gated (`integration_real`); rewrite the Render site +
  the DefaultProvider→model-prefix injection for tag-correctness, but don't run/debug the real agents here.

---

## Confidence Score

**8.5/10** for one-pass implementation success.

Rationale: ~30 of the ~41 sites are a single mechanical positional transform (provider is literal `""`
everywhere — scout §A/§E confirmed; the stub manifest is FR-R5b-safe — verified). The 5 semantic reworks
are precisely specified with before/after, and the golden argv is byte-identical for the fold cases
(only the input model string changes). The 3 new reasoning tests are given as full code. The
DefaultProvider compile-break cleanup is enumerated per-file. The #1 risk — the S1/S2 green-scope
boundary (`go test ./...` stays red in generate/decompose until P1.M2.T1.S2) — is front-loaded as a
CRITICAL gotcha with the exact gate (`go test ./internal/provider/ ./internal/stubtest/`) so the
implementer does not waste effort chasing out-of-scope packages or editing production. The residual
uncertainty (not 9–10): the exact line numbers shift as S1 lands in parallel (mitigated: the PRP keys on
test NAMES + structural intent, not brittle line numbers), and the `realagent_test.go` DefaultProvider→
model-prefix injection is a small semantic call under a gated tag (low blast radius — excluded from
normal builds). Downstream subtasks (P1.M2.T1.S2 generate/decompose test rework) are cleanly fenced and
cannot be broken by S2 because S2 touches only test files in two packages.
