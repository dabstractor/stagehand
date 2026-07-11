# Research Findings — P1.M2.T1.S2 (Add commented-pi-block test, Issue 2)

## 0. Task shape (one sentence)
Add a single table-free unit test `TestBuildBootstrapConfig_CommentedPiBlockBlanked` (plus a small
`extractCommentedProviderBlock` helper) to `internal/config/bootstrap_test.go` that asserts the
commented-out pi provider block — produced by the P1.M2.T1.S1 fix — ships blank models + a guidance
NOTE and contains NO bare gpt-5.4 model assignment. **Test-only: zero production-code changes.**

## 1. GROUND-TRUTH output (S1 already applied in the working tree)
The parallel S1 fix is live. A scratch run of `buildBootstrapConfig("claude", []string{"claude","pi"}, nil)`
emits this EXACT commented pi block (verified 2026-07-11):

```
# === pi (installed) — uncomment a [role.*] block to route that role to pi ===
# NOTE: pi is a multi-backend provider — prefix the model with your inference backend,
# e.g. model = "zai/gpt-5.4". A bare model (no '/') on pi is a config error (FR-R5b).
# [role.planner]
# provider = "pi"
# model = ""
# [role.stager]
# provider = "pi"
# model = ""
# [role.message]
# provider = "pi"
# model = ""
# [role.arbiter]
# provider = "pi"
# model = ""

# ---------------------------------------------------------------------------
# [generation] — diff capture & output tuning (PRD §16.2)
```

Observable facts used by the test:
- **Header** substring `# === pi (installed)` is present (target=claude → claude is the active
  provider; pi is installed and ≠ target → pi is the ONLY commented block).
- **NOTE** substring `multi-backend provider` is present (S1's 2-line guidance comment).
- **Each role line** is `# model = ""` (4 occurrences — planner, stager, message, arbiter).
- The block is bounded BELOW by the `\n# ----...----` generation separator (a blank line, then the
  dashed `[generation]` section header).

## 2. ⚠️ CRITICAL FINDING — the contract's literal assertion (d) CONFLICTS with S1's NOTE
The item CONTRACT says: *"(d) Negative: the pi block must NOT contain `gpt-5.4` anywhere."*

**This is INCOMPATIBLE with S1's output.** S1's guidance NOTE contains the EXAMPLE
`# e.g. model = "zai/gpt-5.4"` — which contains the literal substring `gpt-5.4`. A test asserting
`!strings.Contains(piBlock, "gpt-5.4")` would FAIL even with the S1 fix correctly applied (verified:
scratch test fired `(d) FAIL: has gpt-5.4` against the live, fixed code).

**Resolution (validated):** Refine assertion (d) from the literal substring to the **BARE MODEL
ASSIGNMENT form** `# model = "gpt-5.4`:
```go
if strings.Contains(piBlock, `# model = "gpt-5.4`) {   // bare assignment — the actual bug shape
    t.Errorf("commented pi block must not ship bare gpt-5.4* model assignments (FR-R5b)…")
}
```
Why this is correct AND faithful to the contract's INTENT:
- The OLD (pre-S1) bug wrote `# model = "gpt-5.4"`, `# model = "gpt-5.4-mini"`, `# model = "gpt-5.4-nano"`
  — ALL begin with `# model = "gpt-5.4`. The refined guard fires on every one. ✅ (verified via a
  simulated old-block string: the guard correctly reports the bug.)
- The NEW (S1) code writes `# model = ""` for every role. The refined guard does NOT fire. ✅
- S1's NOTE example `zai/gpt-5.4` lives in a `# e.g.` COMMENT line, NOT a `# model =` assignment, so it
  does NOT trip the refined guard. ✅
- The blank-presence check (b) + the 4-count check together ALREADY prove no bare model exists; the
  refined (d) is a belt-and-braces smoke alarm for the specific bug token, scoped to the assignment form.

This refinement is the single most important implementation decision. It is REQUIRED — without it the
test cannot pass against the very fix it is meant to validate. (The PRP spells it out in the test code
and the Anti-Patterns section.)

## 3. Existing test to follow + companion relationship
`TestBuildBootstrapConfig_OtherInstalledCommented` (bootstrap_test.go:153) is the MIRROR companion:
- It calls `buildBootstrapConfig("pi", []string{"pi","claude"}, nil)` (target=pi, claude installed).
- It asserts the commented **claude** block keeps its bare model `# model = "haiku"` (claude is NOT
  blanked, because claude has NO ProviderFlag — single-backend, bare models are legal).
- Our new test calls `buildBootstrapConfig("claude", []string{"claude","pi"}, nil)` (target=claude,
  pi installed) and asserts the commented **pi** block IS blanked (pi HAS a ProviderFlag → FR-R5b).
- Together they prove BOTH sides of the blanking rule: ProviderFlag providers (pi) → blanked; non-
  ProviderFlag providers (claude) → untouched. The existing test MUST stay green (S1 only adds a
  `name == "pi"` branch — claude is unaffected).

## 4. Extraction-helper design (validated against live output)
The test must scope its assertions to the pi commented block so the ACTIVE (claude) role models above
don't interfere. The idiom is already established by `extractStagerBlock` (bootstrap_test.go:276). A
parallel helper `extractCommentedProviderBlock(content, provider)`:

```go
func extractCommentedProviderBlock(content, provider string) string {
	header := "# === " + provider + " (installed)"
	start := strings.Index(content, header)
	if start < 0 { return "" }
	rest := content[start:]
	nextIdx := len(rest)
	for _, marker := range []string{"\n# ===", "\n# ---"} {  // next provider block OR generation separator
		if i := strings.Index(rest[1:], marker); i >= 0 && i+1 < nextIdx { nextIdx = i + 1 }
	}
	return rest[:nextIdx]
}
```
Validated: against the live `buildBootstrapConfig("claude",["claude","pi"],nil)` output it returns
exactly the 12 pi-block lines (header + NOTE×2 + 4 role blocks of 2 lines each) and stops before the
blank line + generation separator. The `rest[1:]` skip avoids re-matching the header itself; the two
markers correctly bound a single commented block even if multiple commented providers were present.

## 5. ProviderFlag distinction (the semantic core of the blanking rule)
- **pi**: `ProviderFlag: strPtr("--provider")` (builtin.go:53) — NON-EMPTY → multi-backend → a model
  on pi MUST carry its inference backend as a slash-prefix (`zai/gpt-5.4`); a BARE model is a hard
  FR-R5b error. → bootstrap BLANKS pi's models (active block AND commented block).
- **claude**: `ProviderFlag: strPtr("")` (builtin.go:121) — explicit-NIL-empty → single-backend → bare
  aliases (`haiku`, `opus`, `sonnet`) are LEGAL. → bootstrap does NOT blank claude.

This is why our test blanks pi and the companion test keeps claude's `haiku`.

## 6. Raw compiled-in defaults (role_defaults.go) — informs the negative guard
- `pi`: planner=`gpt-5.4`, stager=`gpt-5.4-mini`, message=`gpt-5.4-nano`, arbiter=`gpt-5.4-mini`
  → ALL contain `gpt-5.4`; ALL begin with `gpt-5.4` when quoted in `# model = "…"`. (This is exactly
  what the OLD commented block shipped — the bug.)
- `claude`: planner=`opus`, stager=`sonnet`, message=`haiku`, arbiter=`sonnet`
  → NONE contain `gpt-5.4`. (The active claude block in our test keeps real models — a bonus sanity
  assertion `strings.Contains(content, `model = "haiku"`)` confirms claude is NOT collateral damage.)

## 7. Old-vs-new behavior proof (the test must FAIL on old, PASS on new)
| Assertion                                 | OLD (pre-S1) pi block            | NEW (S1) pi block              |
|------------------------------------------|----------------------------------|--------------------------------|
| (a) header `# === pi (installed)` present | PASS (unchanged by S1)           | PASS                           |
| (b) contains `# model = ""`               | **FAIL** (has `# model = "gpt-5.4"`) | PASS                       |
| (count) exactly 4 × `# model = ""`        | **FAIL** (0 blanks)              | PASS                           |
| (c) NOTE `multi-backend provider` present | **FAIL** (no NOTE)               | PASS                           |
| (d) no `# model = "gpt-5.4` bare assign   | **FAIL** (bare models present)   | PASS                           |

Four independent failure modes on the old code; four independent passes on the new. Verified via
simulated old-block string + live new output (scratch tests, cleaned up).

## 8. Build / test / lint commands (Makefile-confirmed)
- `go test ./internal/config/ -v -run TestBuildBootstrapConfig_CommentedPi` — runs the NEW test only
  (regex matches `TestBuildBootstrapConfig_CommentedPiBlockBlanked`).
- `go test ./internal/config/ -run 'BuildBootstrapConfig' -v` — runs ALL bootstrap tests incl. the
  companion `TestBuildBootstrapConfig_OtherInstalledCommented` (claude-not-blanked regression).
- `make test` → `go test -race ./...` (full race suite).
- `make lint` → `golangci-lint run` (staticcheck/gosimple/govet/errcheck/ineffassign/unused).
- `gofmt -l internal/config/bootstrap_test.go` — must be empty (formatted).
- `go build ./...` and `go vet ./internal/config/...` — clean.

## 9. Scope fences (what NOT to do)
- NO production-code changes (bootstrap.go is S1's domain; it is treated as a CONTRACT here).
- NO docs changes (item DOCS: none — test-only).
- DO NOT touch the companion `TestBuildBootstrapConfig_OtherInstalledCommented` (it already covers the
  claude-not-blanked side; just keep it green).
- DO NOT use the literal `gpt-5.4` whole-block guard (§2 — it false-positives on S1's NOTE example).
- DO NOT add ValidateModel assertions here (commented TOML is inert → never decoded → ValidateModel
  can't reach it; that regression net is the parallel P1.M1.T2.S1, explicitly out of scope).
