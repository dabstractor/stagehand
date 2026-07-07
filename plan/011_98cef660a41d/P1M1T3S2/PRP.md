---
name: "P1.M1.T3.S2 — E2e test for assembled-prompt-≤-token_limit invariant (FR3j)"
description: |
  TEST-ONLY (2 points). The INTEGRATION-level proof of the FR3j hard invariant (PRD §9.1 FR3j / §20.5):
  after the closed-loop gate runs, `EstimateTokens(assembledFullPrompt) ≤ token_limit`, ALWAYS — proven
  end-to-end against a REAL git repo (temp repo + stub provider, standard test pass, no real model).
  DISTINCT from the parallel S1 (P1.M1.T3.S1 — PURE unit tests of `closedLoopGate` with SYNTHETIC
  measures, internal/git/tokengate_test.go): S2 drives the FULL real path — real git diff capture → the
  REAL `MeasureAssembled` closure → `closedLoopGate` → `provider.Execute` (the stub) → capture the
  stub's received stdin → measure → assert it fits. TWO cases (the contract's PRIMARY + "Consider"):
  (1) MESSAGE-ROLE (PRIMARY): `TestCommitStaged_TokenLimitInvariant_AssembledPromptFits` in a NEW file
  `internal/generate/tokenlimit_invariant_test.go` (package generate) — stage a large diff (~1600 runes
  ≈ 390 tokens), set cfg.TokenLimit=200 (≪ untruncated ⇒ the gate MUST truncate), run CommitStaged with
  a stub provider whose stdin is captured via STAGEHAND_STUB_STDINFILE (the TestCommitStaged_ExcludedPayloadCapture
  pattern at generate_test.go:660), read the captured stdin, assert `git.EstimateTokens(stdin) ≤
  cfg.TokenLimit + assembledPromptSeparatorTokens` (assembledPromptSeparatorTokens = EstimateTokens("\n\n")
  = 1 — the Render prepend separator at render.go:158; FR3j's invariant is on the separator-free
  sysPrompt+payload the closure measures, so the +1 is the bounded artifact), AND assert the stdin
  contains "[truncated]" (the water-fill sentinel — proves the closed-loop ACTIVELY truncated). (2)
  PLANNER-ROLE (SECONDARY, "Consider"): `TestDecompose_TokenLimitInvariant_PlannerPromptFits` APPENDED
  to `internal/decompose/decompose_test.go` (package decompose — the dcm* harness's home) — drive
  Decompose() with cfg.TokenLimit set + a large working-tree diff; isolate the planner's stdin via
  role-specific Env (`plannerM.Env["STAGEHAND_STUB_STDINFILE"] = plannerFile` — ONLY the planner manifest
  carries it, so stager/message/arbiter invocations drain to io.Discard and don't overwrite it; cleaner
  than the tee-wrapper); assert the planner's captured stdin fits tokenLimit+separator + contains
  "[truncated]". Both tests run in the STANDARD test pass (in-process via CommitStaged/Decompose; NOT
  `//go:build e2e` — that's the subprocess harness). The closed-loop gate (tokengate.go:195) + the 6
  MeasureAssembled closures (T1/T2 — Complete) are READ-ONLY. NO production code. NO docs (P1.M3).
---

## Goal

**Feature Goal**: Land the end-to-end integration proof that FR3j's closed-loop token-budget guarantee
holds against a REAL git repo — the regression net PRD §20.5 mandates for the routing/sizing invariants
that "are easy to specify, easy to regress, and easy to break silently (unit tests with stub agents
cannot reach them)." The pure-unit proof (S1) covers `closedLoopGate`'s convergence with synthetic
measures; THIS task proves the FULL assembled path — the real `MeasureAssembled` closure measuring the
real sysPrompt + the real role payload wrapping the real gated diff — produces a stub-received stdin
whose `EstimateTokens` fits `token_limit`, on both the message-role (CommitStaged→StagedDiff) and the
planner-role (Decompose→TreeDiff) consumer paths.

**Deliverable** (TEST-ONLY — two files):
1. **CREATE** `internal/generate/tokenlimit_invariant_test.go` (`package generate`): the PRIMARY
   message-role test `TestCommitStaged_TokenLimitInvariant_AssembledPromptFits`.
2. **APPEND** to `internal/decompose/decompose_test.go` (`package decompose`): the SECONDARY planner-role
   test `TestDecompose_TokenLimitInvariant_PlannerPromptFits`.

No production code. No docs. No `//go:build e2e` tag (in-process).

**Success Definition**: `go build/vet/gofmt` clean; `go test -race ./...` green with both new tests
passing; the message-role test proves `EstimateTokens(capturedStdin) ≤ cfg.TokenLimit + 1` (the Render
separator allowance) when a ~390-token diff is gated to tokenLimit=200, AND the captured stdin contains
"[truncated]" (the gate actively ran); the planner-role test proves the same invariant on the planner's
captured stdin (isolated via role-specific Env) in a Decompose run; `git diff --stat` shows ONLY the two
test files (one new, one appended).

## User Persona

**Target User**: The contributor/maintainer who needs confidence that FR3j's `EstimateTokens(assembledFullPrompt)
≤ token_limit` guarantee survives the WHOLE real path — not just the pure gate function (S1) but the
actual git diff capture, the actual sysPrompt + payload assembly, the actual closed-loop re-measurement,
and the actual stdin delivered to the provider. Also the reviewer auditing FR3j before release, and the
user who sets `token_limit` and relies on the "payload always fits your context window" contract.

**Use Case**: A future change that breaks the closed loop — e.g., a refactor that drops the
`MeasureAssembled` closure at a consumer site, a change to `BuildUserPayload` that adds unaccounted
framing, or a water-fill tweak that leaves the assembled prompt over budget — is regression-caught by
this test (the captured stdin's `EstimateTokens` exceeds the bound, or the gate fails to truncate).

**User Journey**: `go test -race ./internal/generate/ -run TestCommitStaged_TokenLimitInvariant` → the
message-role test PASSes (captured stdin fits tokenLimit+separator, contains "[truncated]"). `go test
-race ./internal/decompose/ -run TestDecompose_TokenLimitInvariant` → the planner-role test PASSes
(planner's captured stdin fits). A regression that re-opens the budget loophole fails one of these.

**Pain Points Addressed**: Closes the silent-budget-overrun regression class at the integration layer.
Without these tests, a refactor that breaks the closed-loop wiring (e.g., nil-ing a closure, or adding
payload framing the closure doesn't measure) would ship undetected — the pure unit tests (S1) exercise
the gate with synthetic measures, not the real assembled prompt.

## Why

- **PRD §9.1 FR3j mandates the closed-loop guarantee** (`EstimateTokens(assembledFullPrompt) ≤ token_limit`,
  always; the loop re-trims until it fits). §20.5 (End-to-end scenario harness) is the mandate for
  exactly this kind of invariant — "easy to break silently (unit tests with stub agents cannot reach
  them)." The integration proof is the regression net.
- **The pure-unit proof (S1) is necessary but not sufficient.** S1 covers `closedLoopGate`'s convergence
  + termination + best-effort with SYNTHETIC measure callbacks. It does NOT exercise the REAL
  `MeasureAssembled` closure (which measures `sysPrompt + BuildUserPayload(gatedDiff)`), the REAL git
  diff capture, or the REAL stdin delivered to the provider. A wiring break (a nil closure, a missing
  field) is invisible to S1; THIS task catches it.
- **Two consumer paths, two tests.** The contract names the message-role (CommitStaged→StagedDiff,
  PRIMARY) and the planner-role (Decompose→TreeDiff, "Consider"). Both route through the same gate but
  via different closures (`BuildUserPayload` vs `BuildPlannerUserPayload`) and different diff functions
  (`StagedDiff` vs `TreeDiff`). Proving both catches a wiring break on either path.
- **The infrastructure exists.** `TestCommitStaged_ExcludedPayloadCapture` (generate_test.go:660) proves
  the STDINFILE-capture pattern; the dcm* decompose harness + the role-specific Env seam make the
  planner case feasible. No new mechanisms needed — just the assertion.

## What

Two integration tests, each driving the real closed-loop path against a temp git repo with a stub
provider, capturing the provider's received stdin, and asserting `EstimateTokens(stdin) ≤ tokenLimit +
separatorAllowance`.

### (1) Message-role (PRIMARY) — `TestCommitStaged_TokenLimitInvariant_AssembledPromptFits`

Stage a large diff (~1600 runes ≈ 390 tokens), set `cfg.TokenLimit = 200` (≪ 390 ⇒ the gate MUST
truncate to fit), run `CommitStaged` with a stub provider whose stdin is captured via
`STAGEHAND_STUB_STDINFILE`. Read the captured stdin. Assert:
- `git.EstimateTokens(capturedStdin) ≤ cfg.TokenLimit + assembledPromptSeparatorTokens` where
  `assembledPromptSeparatorTokens = git.EstimateTokens("\n\n")` (= 1). (FR3j guarantees the closure's
  measurement — `sysPrompt + payload`, no separator — is ≤ tokenLimit; the stub's stdin adds the
  `"\n\n"` Render separator (render.go:158), so +1 is the bounded artifact.)
- The captured stdin contains `"[truncated]"` (the water-fill sentinel — proves the closed-loop actively
  truncated; not a no-op).

### (2) Planner-role (SECONDARY) — `TestDecompose_TokenLimitInvariant_PlannerPromptFits`

Drive `Decompose()` with a large working-tree diff + `cfg.TokenLimit` set. Isolate the planner's stdin
via **role-specific Env** — set `STAGEHAND_STUB_STDINFILE` on ONLY the planner manifest's `Env` map (the
stager/message/arbiter manifests do NOT carry it ⇒ their invocations drain to io.Discard ⇒ never
overwrite the planner's file). Read the planner's captured stdin. Assert the same invariant +
`"[truncated]"` sentinel.

### Success Criteria

- [ ] `internal/generate/tokenlimit_invariant_test.go` exists (`package generate`) with
      `TestCommitStaged_TokenLimitInvariant_AssembledPromptFits`.
- [ ] The message-role test stages a diff whose untruncated assembled prompt exceeds `cfg.TokenLimit`,
      sets `STAGEHAND_STUB_STDINFILE`, runs `CommitStaged`, reads the captured stdin, and asserts
      `EstimateTokens(stdin) ≤ cfg.TokenLimit + assembledPromptSeparatorTokens` AND `stdin` contains
      `"[truncated]"`.
- [ ] `internal/decompose/decompose_test.go` has `TestDecompose_TokenLimitInvariant_PlannerPromptFits`.
- [ ] The planner test isolates the planner's stdin via role-specific `Env["STAGEHAND_STUB_STDINFILE"]`
      (only the planner manifest carries it), runs `Decompose`, and asserts the same invariant.
- [ ] `assembledPromptSeparatorTokens` is `git.EstimateTokens("\n\n")` (the Render separator allowance;
      documented, not a magic number).
- [ ] Both tests run in the STANDARD test pass (no `//go:build e2e`).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] `git diff --stat` shows ONLY the two test files (one new, one appended). NO production code touched.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim test bodies (copy-paste-ready), the exact
STDINFILE-capture pattern (from generate_test.go:660-731, quoted), the closedLoopGate invariant + the
closure bodies (generate.go / planner.go:81, quoted), the Render separator math (render.go:158
`sysPrompt + "\n\n" + payload`, with the EstimateTokens-difference proof), the role-specific Env
isolation for the planner case, the reusable helpers (`initRepo`/`writeFile`/`stageFile` in
generate_test.go; `dcm*` in decompose_test.go), and the hard scope fences. No inference required.

### Documentation & References

```yaml
# MUST READ — the FR3j spec, the gate, and this task's research
- file: PRD.md
  why: "§9.1 FR3j (the closed-loop budget guarantee: 'stagehand assembles the actual full prompt — the
        system prompt plus BuildUserPayload(gatedDiff) — measures it with the same EstimateTokens used
        for sizing, and if it exceeds token_limit, reduces the body budget and re-applies truncation.
        Invariant: EstimateTokens(assembledFullPrompt) ≤ token_limit, always'). §9.1 FR3i (the water-fill
        that emits the '... [truncated]' sentinel per over-budget file). §20.5 (the e2e harness mandate:
        invariants 'easy to break silently (unit tests with stub agents cannot reach them)')."
  critical: "FR3j's invariant is on EstimateTokens(sysPrompt + BuildUserPayload(gatedDiff)) — the
             separator-free assembled prompt the closure measures. The +1 separator allowance in the
             assertion is the Render stdin artifact, NOT a violation (D1)."

- docfile: plan/011_98cef660a41d/architecture/fr3j_closed_loop.md
  why: "The architecture spec: the loop structure, the bound (maxClosedLoopPasses), the invariant, the
        MeasureAssembled injection seam, and the 6 consumer wiring sites (3 message-role + 3 decompose-role)."
  critical: "Confirms the closure measures sysPrompt + <role payload>(gatedDiff) and the gate guarantees
             that quantity ≤ tokenLimit. The 6-site table confirms message-role (generate.go) +
             planner-role (planner.go) are both wired (T2.S1/T2.S2, Complete)."

- docfile: plan/011_98cef660a41d/P1M1T3S2/research/tokenlimit_invariant_e2e.md
  why: "THIS subtask's research: §2 the gate invariant + the closure bodies; §3 the Render separator
        math (EstimateTokens(stdin) ≤ closure_measurement + 1 ≤ tokenLimit + 1); §4 the STDINFILE
        capture pattern + the role-specific Env isolation for the planner case; §5/§6 the two test
        designs; §7 why the stub manifest has no SystemPromptFlag (so stdin = full assembled prompt);
        §8 decisions D1-D6. READ THIS FIRST."
  critical: "§3 (the +1 separator allowance — D1) and §4 (the role-specific Env isolation for the
             planner — D2) are the two non-obvious calls. §7 explains why the stub manifest MUST NOT
             set SystemPromptFlag (else stdin misses sysPrompt)."

- docfile: plan/011_98cef660a41d/P1M1T3S1/PRP.md
  why: "The parallel SIBLING (PURE unit tests of closedLoopGate with synthetic measures, internal/git/
        tokengate_test.go). Confirms S1 is PURE (no repo, no real closure) — DISTINCT from this
        integration test. No file overlap (S1 = internal/git/; S2 = internal/generate/ + internal/decompose/)."
  critical: "S1 proves the gate's CONVERGENCE logic; S2 (this) proves the INVARIANT holds on the real
             assembled path. Both are needed; they do not duplicate."

- docfile: plan/011_98cef660a41d/P1M1T2S1/PRP.md
  why: "The CONTRACT for the message-role MeasureAssembled closure (LANDED): generate.go's CommitStaged
        StagedDiffOptions passes `func(gatedDiff string) int { return git.EstimateTokens(sysPrompt +
        prompt.BuildUserPayload(gatedDiff, cfg.Context, nil)) }` when cfg.TokenLimit != 0. This is the
        closure whose invariant the PRIMARY test proves."
  critical: "The closure measures sysPrompt + payload (no separator). The stub's stdin = that + '\\n\\n'
             (render.go:158). The +1 assertion allowance is this separator."

- docfile: plan/011_98cef660a41d/P1M1T2S2/PRP.md
  why: "The CONTRACT for the decompose-role MeasureAssembled closures (LANDED): planner.go:81 passes
        `func(gatedDiff string) int { return git.EstimateTokens(sysPrompt + prompt.BuildPlannerUserPayload(
        gatedDiff, cfg.Context, forcedCount)) }`. This is the closure whose invariant the SECONDARY test
        proves."
  critical: "The planner's TreeDiff call (planner.go:89) carries MeasureAssembled + TokenLimit. The
             planner manifest's Env is the isolation seam for capturing its stdin (D2)."

# The pattern file (the template) + the code under test (READ-ONLY)
- file: internal/generate/generate_test.go
  why: "THE TEMPLATE. TestCommitStaged_ExcludedPayloadCapture (:660-731) is the exact STDINFILE-capture
        pattern: t.Setenv(STAGEHAND_STUB_STDINFILE, stdinFile) → stubtest.Manifest → CommitStaged →
        os.ReadFile(stdinFile). Reuse its helpers: initRepo (:23), writeFile (:31), stageFile (:39),
        commitRaw (:45). The doc comment at :741 documents 'provider.Render prepends the system prompt
        to the payload with a \\n\\n delimiter' — the separator this test accounts for."
  pattern: "stdinFile := filepath.Join(t.TempDir(), 'stdin.txt'); t.Setenv(STAGEHAND_STUB_STDINFILE,
            stdinFile); m := stubtest.Manifest(stub, Options{Out: ...}); CommitStaged(...); data, _ :=
            os.ReadFile(stdinFile)."
  gotcha: "The stub manifest has NO SystemPromptFlag ⇒ Render prepends sysPrompt+'\\n\\n'+payload to
           stdin ⇒ the captured file IS the full assembled prompt (+ separator). Do NOT set a
           SystemPromptFlag (that would send sysPrompt via the flag and stdin = payload only — missing
           sysPrompt, WRONG for this invariant). (Research §7.)"

- file: internal/git/tokengate.go
  why: "READ-ONLY (the gate under test). closedLoopGate (:195) — when measure != nil, guarantees
        measure(gatedDiff) ≤ tokenLimit. maxClosedLoopPasses (:83), closedLoopSlack (:90). applyWaterFillGate
        (:135) → truncateByWaterFill emits '... [truncated]' per over-budget file (the sentinel the test
        asserts). Confirms the gate is ACTIVE when MeasureAssembled is non-nil (which it is when
        cfg.TokenLimit != 0)."
  gotcha: "The gate is nil-safe (measure==nil → delegates to applyWaterFillGate, no loop). The test
           sets cfg.TokenLimit != 0 ⇒ the closure is non-nil ⇒ the closed loop runs."

- file: internal/generate/generate.go
  why: "READ-ONLY. CommitStaged (:158) — the entry point the PRIMARY test drives. The MeasureAssembled
        closure (T2.S1, LANDED) at the StagedDiff call site. The dedupe loop re-renders on retry but the
        DIFF is captured ONCE (before the loop) ⇒ the captured stdin's FIRST measurement reflects the
        gated diff with nil rejected — exactly the closure's measurement."
  gotcha: "The diff is captured ONCE before the dedupe loop. The stub's stdin (first attempt) =
           sysPrompt + '\\n\\n' + BuildUserPayload(gatedDiff, cfg.Context, nil) — exactly the closure's
           measurement + separator. (On a dedupe retry the stdin would carry a rejection block, but the
           stub's STAGEHAND_STUB_STDINFILE captures the LAST invocation — which is the successful one,
           no rejection block. So the captured stdin matches the closure's nil-rejected measurement.)"

- file: internal/decompose/planner.go
  why: "READ-ONLY. The planner's TreeDiff call (:89) with MeasureAssembled (:81) + TokenLimit (:94).
        Confirms the planner-role path is wired (T2.S2, Complete). The closure measures
        EstimateTokens(sysPrompt + BuildPlannerUserPayload(gatedDiff, cfg.Context, forcedCount))."
  gotcha: "The planner manifest's Env is the isolation seam (D2): set STAGEHAND_STUB_STDINFILE on ONLY
           the planner manifest so stager/message/arbiter invocations don't overwrite it."

- file: internal/git/tokens.go
  why: "READ-ONLY. EstimateTokens(s) = ceil(utf8.RuneCountInString(s) / 4) at :25. The SAME estimator the
        closure uses (single-estimator rule, FR3j). The test asserts with it. EstimateTokens('\\n\\n') =
        ceil(2/4) = 1 (the separator allowance)."

# External references
- url: https://pkg.go.dev/math#Ceil
  why: "(Background.) EstimateTokens is ceil(runeCount/4). Adding 2 runes ('\\n\\n') raises ceil by at
        most 1 — the mathematical basis for the +1 separator allowance (D1)."
  critical: "This is WHY the +1 is a hard upper bound on the stdin-vs-closure difference (not an
             arbitrary slack). ceil(n/4) − ceil((n−2)/4) ≤ 1 for all n ≥ 2."
```

### Current Codebase Tree (this task's scope)

```bash
stagehand/
├── internal/generate/
│   ├── generate_test.go                  # READ-ONLY (the template: TestCommitStaged_ExcludedPayloadCapture :660 + helpers)
│   └── tokenlimit_invariant_test.go      # NEW (PRIMARY message-role test)
├── internal/decompose/
│   └── decompose_test.go                 # EDIT/APPEND (SECONDARY planner-role test)
└── (internal/git/tokengate.go, generate.go, decompose/planner.go = READ-ONLY — the code under test)
```

### Desired Codebase Tree After This Subtask

```bash
stagehand/
├── internal/generate/
│   └── tokenlimit_invariant_test.go      # NEW — TestCommitStaged_TokenLimitInvariant_AssembledPromptFits
└── internal/decompose/
    └── decompose_test.go                 # +1 test — TestDecompose_TokenLimitInvariant_PlannerPromptFits
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/generate/tokenlimit_invariant_test.go` | CREATE | `package generate`. The PRIMARY message-role integration test: stage a large diff, set TokenLimit, run CommitStaged with STDINFILE capture, assert `EstimateTokens(stdin) ≤ tokenLimit+1` + `"[truncated]"` sentinel. Reuses generate_test.go's `initRepo`/`writeFile`/`stageFile`. |
| `internal/decompose/decompose_test.go` | MODIFY (append) | `package decompose`. The SECONDARY planner-role test: drive Decompose with TokenLimit + role-specific STDINFILE on the planner manifest, assert the planner's captured stdin fits. Reuses `dcm*` helpers. |

**Explicitly NOT touched**: any production code (`internal/git/tokengate.go`, `internal/generate/generate.go`,
`internal/decompose/planner.go`, `internal/git/git.go`, etc. — the gate + closures are LANDED by T1/T2),
the existing tests in generate_test.go (the template — read-only), S1's `internal/git/tokengate_test.go`
(parallel — pure unit tests), `internal/e2e/*` (the `//go:build e2e` subprocess harness — this is
in-process), any docs (P1.M3), `README.md`, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — the +1 separator allowance; do NOT assert the literal ≤ tokenLimit). The stub manifest
// has NO SystemPromptFlag ⇒ provider.Render prepends `sysPrompt + "\n\n" + userPayload` to stdin
// (render.go:158). The MeasureAssembled closure measures `EstimateTokens(sysPrompt + payload)` (Go `+`,
// NO separator). So capturedStdin = closure_measurement + "\n\n", and EstimateTokens(capturedStdin) ≤
// closure_measurement + 1 (ceil(runes/4) rises by ≤1 when 2 runes are appended). FR3j guarantees
// closure_measurement ≤ tokenLimit ⇒ EstimateTokens(capturedStdin) ≤ tokenLimit + 1. Asserting the
// literal `≤ tokenLimit` RISKS a 1-token failure when the closure lands at exactly tokenLimit and the
// separator pushes the stdin over a token boundary. Use `≤ cfg.TokenLimit + assembledPromptSeparatorTokens`
// where assembledPromptSeparatorTokens = git.EstimateTokens("\n\n") (= 1). DOCUMENTED, not a violation.
// (Research §3 / D1; generate_test.go:741 documents the same separator.)

// CRITICAL (G2 — the stub manifest MUST NOT set SystemPromptFlag). With no SystemPromptFlag, Render
// prepends sysPrompt to stdin ⇒ capturedStdin = the FULL assembled prompt (sysPrompt + separator +
// payload). Setting a SystemPromptFlag would send sysPrompt via the flag and stdin = payload ONLY
// (missing sysPrompt) ⇒ the assertion would be measuring the wrong thing (payload-alone, not the
// assembled prompt). stubtest.Manifest does NOT set SystemPromptFlag (correct); do NOT add one.
// (Research §7.)

// CRITICAL (G3 — force the gate to RUN: untruncated prompt ≫ tokenLimit). Set cfg.TokenLimit to a value
// the UNTRUNCATED assembled prompt far exceeds (e.g., stage ~1600 runes ≈ 390 tokens, tokenLimit=200).
// This forces the water-fill + closed-loop to truncate. AND assert the captured stdin contains
// "[truncated]" (truncateByWaterFill's sentinel) — proving the gate ACTIVELY truncated, not a no-op
// where the payload happened to fit without truncation. A test that sets tokenLimit above the payload
// size would pass trivially without exercising the gate. (D4.)

// CRITICAL (G4 — the planner's stdin isolation via role-specific Env, NOT t.Setenv). A decompose run
// invokes the stub multiple times (planner → stager → message → arbiter). `t.Setenv(STAGEHAND_STUB_STDINFILE,
// path)` would be inherited by ALL invocations ⇒ overwritten by the last (message/arbiter). Instead, set
// STAGEHAND_STUB_STDINFILE on ONLY the planner manifest's Env map: `plannerM.Env["STAGEHAND_STUB_STDINFILE"]
// = plannerFile`. Render builds spec.Env = os.Environ() + manifest.Env, so ONLY the planner invocation's
// stub sees it; the stager/message/arbiter manifests don't carry it ⇒ their stubs drain to io.Discard.
// (Research §4 / D2.)

// GOTCHA (G5 — STDINFILE captures the LAST invocation for the message-role case, which is correct).
// CommitStaged's dedupe loop may invoke the stub multiple times (on retry). STAGEHAND_STUB_STDINFILE
// captures the LAST invocation's stdin. The LAST invocation is the SUCCESSFUL one (the one that produced
// the committed message) — its stdin has NO rejection block (rejected is nil on success, or the final
// accepted attempt). The diff is captured ONCE (before the loop), so the gated diff is identical across
// attempts. The closure measures the nil-rejected assembled prompt; the captured (last/successful) stdin
// matches it (+ separator). So the assertion is faithful. (For the planner case, the LAST invocation is
// the arbiter/message — hence the role-specific Env isolation in G4.)

// GOTCHA (G6 — config.Defaults() + cfg.TokenLimit override; the stub needs Provider/Model only if
// CommitStaged resolves them — but deps.Manifest IS the stub, so cfg.Provider/Model are not strictly
// required). Mirror TestCommitStaged_ExcludedPayloadCapture: it sets Provider="stub", Model="stub" in the
// cfg literal. To be safe, use `cfg := config.Defaults(); cfg.TokenLimit = <small>` (Defaults has
// Timeout=120s — fine for the instant stub). If the stub fails to produce output (Out is the canned
// message), CommitStaged enters rescue — set `Out: "feat: ..."` so the run succeeds and the stdin is
// captured. (The dedupe loop's recentSubjects needs ≥1 commit for the mature-repo path, or the new-repo
// path is used — either is fine; the gate runs before the message.)

// GOTCHA (G7 — the message-role test's repo needs content that produces a DIFF, not just files). Stage
// a MODIFIED or ADDED file (write + git add). The diff body is what the gate truncates. A repo with only
// an empty commit + a staged new file produces an add-diff. Use strings.Repeat for a large deterministic
// body so EstimateTokens is predictable (≈390 for ~1600 runes). (Research §5.)

// GOTCHA (G8 — both tests run in the STANDARD test pass, NOT //go:build e2e). internal/e2e/* uses the
// `//go:build e2e` tag (subprocess harness, runs only with -tags e2e). THIS task is IN-PROCESS (calls
// CommitStaged/Decompose directly) and runs in `go test ./...`. Do NOT add a build tag. (D6.)

// GOTCHA (G9 — do NOT modify production code). The gate (tokengate.go), the closures (generate.go,
// planner.go, message.go, arbiter.go, stagehand.go, exec.go), and the diff functions (git.go) are
// LANDED by T1/T2. This task is TEST-ONLY. If a test FAILS, the bug is in the test's setup (wrong
// tokenLimit value, missing STDINFILE, wrong role manifest), NOT in the production code — debug the
// test, do not "fix" the gate. (If the production invariant genuinely fails, that's a real bug — but
// T1/T2 are Complete and S1's unit tests pass, so the gate logic is verified; the integration test
// should pass.)

// GOTCHA (G10 — the SECONDARY planner case is the "Consider" stretch; the PRIMARY message case alone
// satisfies the contract's core). The planner case adds the second consumer path but requires the
// decompose dcm* harness + the role-specific Env isolation. If the harness proves fiddly (Title-matching,
// 2-concept JSON), ship the PRIMARY message case first (it fully proves the invariant on the closed-loop
// path) and add the planner case as the documented stretch. Both are specified here.
```

## Implementation Blueprint

### Data models and structure

None. The tests consume existing types: `config.Config` (TokenLimit), `generate.Deps`, `decompose`'s
`dcm*` helpers, `git.EstimateTokens`, `stubtest.Manifest`/`Build`. No new production types. One
test-local constant: `assembledPromptSeparatorTokens`.

### (1) The PRIMARY message-role test (exact — CREATE `internal/generate/tokenlimit_invariant_test.go`)

```go
package generate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/stubtest"
)

// assembledPromptSeparatorTokens is the Render stdin-separator allowance: provider.Render prepends
// `sysPrompt + "\n\n" + userPayload` to the stub's stdin when the manifest has no system_prompt_flag
// (render.go:158). The FR3j MeasureAssembled closure measures `EstimateTokens(sysPrompt + payload)`
// (Go `+`, NO separator). So capturedStdin = closure_measurement + "\n\n", and EstimateTokens rises by
// ≤1 (ceil(runes/4) on +2 runes). FR3j guarantees closure_measurement ≤ tokenLimit ⇒ capturedStdin ≤
// tokenLimit + 1. The +1 is the bounded separator artifact, NOT a violation (FR3j's invariant is on the
// separator-free assembled prompt). Equal to git.EstimateTokens("\n\n").
const assembledPromptSeparatorTokens = 1 // == git.EstimateTokens("\n\n") == ceil(2/4)

// TestCommitStaged_TokenLimitInvariant_AssembledPromptFits (PRD §9.1 FR3j / §20.5) is the INTEGRATION
// proof that the closed-loop token-budget guarantee holds end-to-end on the message-role path: real git
// diff capture → the REAL MeasureAssembled closure → closedLoopGate → provider.Execute (the stub) →
// the captured stdin fits token_limit. Distinct from S1's PURE unit test (synthetic measures): this
// drives CommitStaged against a real temp repo with a stub provider and asserts on the stub's RECEIVED
// stdin (the closest observable to the assembled prompt).
//
// The stub manifest has NO system_prompt_flag ⇒ Render prepends sysPrompt + "\n\n" + payload to stdin
// (render.go:158) ⇒ the captured stdin IS the full assembled prompt (+ the 1-token separator allowance).
// The gate is forced to run by making the untruncated prompt far exceed tokenLimit (≈390 tokens gated
// to 200), and the "[truncated]" sentinel proves the closed-loop ACTIVELY truncated (not a no-op).
func TestCommitStaged_TokenLimitInvariant_AssembledPromptFits(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// A large staged diff: ~1600 runes of changes ⇒ EstimateTokens(diff) ≈ 390 ≫ tokenLimit=200.
	// The gate MUST truncate to fit (forcing the closed loop to run).
	body := strings.Repeat("change line content here\n", 60) // 23 runes/line × 60 ≈ 1380 runes ≈ 345 tokens
	writeFile(t, repo, "feature.go", "package main\n")
	stageFile(t, repo, "feature.go")
	writeFile(t, repo, "big.go", body) // the large body the gate truncates
	stageFile(t, repo, "big.go")

	// Capture the stub's received stdin (the assembled prompt). Mirrors TestCommitStaged_ExcludedPayloadCapture.
	stdinFile := filepath.Join(t.TempDir(), "stdin.txt")
	t.Setenv("STAGEHAND_STUB_STDINFILE", stdinFile)
	stub := stubtest.Build(t)
	m := stubtest.Manifest(stub, stubtest.Options{Out: "feat: add big feature"})

	cfg := config.Defaults()
	cfg.TokenLimit = 200 // ≪ 345 ⇒ the water-fill + closed-loop gate MUST truncate to fit

	deps := Deps{Git: git.New(repo), Manifest: m}
	if _, err := CommitStaged(context.Background(), deps, cfg); err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}

	// Read the captured assembled prompt (sysPrompt + "\n\n" + payload wrapping the gated diff).
	data, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("read captured stdin: %v (did the stub run? STAGEHAND_STUB_STDINFILE=%s)", err, stdinFile)
	}
	captured := string(data)

	// (FR3j invariant) EstimateTokens(assembled prompt) ≤ tokenLimit + separator allowance. The +1 is
	// the Render "\n\n" separator (render.go:158); the closure measures the separator-free prompt.
	measured := git.EstimateTokens(captured)
	if measured > cfg.TokenLimit+assembledPromptSeparatorTokens {
		t.Errorf("FR3j invariant violated: EstimateTokens(captured stdin) = %d, want ≤ %d (tokenLimit %d + %d separator allowance)\n"+
			"captured stdin (first 400 chars): %q", measured, cfg.TokenLimit+assembledPromptSeparatorTokens,
			cfg.TokenLimit, assembledPromptSeparatorTokens, truncForLog(captured, 400))
	}

	// (Gate-ran proof) the closed loop ACTIVELY truncated — the water-fill sentinel is present. A no-op
	// (payload fit without truncation) would lack it; with tokenLimit=200 ≪ 345, truncation is mandatory.
	if !strings.Contains(captured, "[truncated]") {
		t.Errorf("expected the water-fill '[truncated]' sentinel in the captured stdin (tokenLimit=%d ≪ untruncated≈%d), "+
			"got none — the closed-loop gate did not truncate (was it wired?)\ncaptured (first 400 chars): %q",
			cfg.TokenLimit, git.EstimateTokens(body), truncForLog(captured, 400))
	}
}

// truncForLog returns s truncated to the first n runes for a readable test failure message.
func truncForLog(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
```

> **Imports:** context, os, path/filepath, strings, testing, time (time only if needed; config/git/stubtest
> are the project imports). `initRepo`/`writeFile`/`stageFile` are reused from generate_test.go (same
> package — no redeclaration). `truncForLog` is a new helper local to this file. `CommitStaged`/`Deps`
> are same-package. If `time` is unused (config.Defaults sets Timeout), drop it.

### (2) The SECONDARY planner-role test (exact — APPEND to `internal/decompose/decompose_test.go`)

```go
// TestDecompose_TokenLimitInvariant_PlannerPromptFits (PRD §9.1 FR3j / §20.5) is the SECOND consumer
// path of the closed-loop invariant: the decompose PLANNER's assembled prompt (TreeDiff-gated) fits
// token_limit. The planner is the FIRST stub invocation in a decompose run; later invocations (stager/
// message/arbiter) would overwrite a plain STAGEHAND_STUB_STDINFILE, so the planner manifest's Env map
// carries it ALONE (the role-specific isolation seam) — the other roles' stubs drain to io.Discard and
// never touch the planner's file. Reuses the dcm* harness.
func TestDecompose_TokenLimitInvariant_PlannerPromptFits(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	// Large UNSTAGED working-tree diff (FR-M1 decompose trigger: nothing staged, tree dirty). The body
	// must exceed tokenLimit so the planner's TreeDiff gate truncates.
	body := strings.Repeat("change line content here\n", 60) // ≈345 tokens
	dcmWriteFile(t, repo, "a.go", "package a\n")
	dcmWriteFile(t, repo, "big.go", body)

	plannerJSON := `{"count":1,"single":true,"commits":[{"title":"add big","description":"big.go"}],"message":"feat: add big"}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	// Isolate the planner's stdin: ONLY the planner manifest carries STAGEHAND_STUB_STDINFILE. The
	// stager/message/arbiter manifests don't ⇒ their stubs see os.Getenv("") ⇒ drain to io.Discard.
	plannerStdin := filepath.Join(t.TempDir(), "planner-stdin.txt")
	plannerM.Env["STAGEHAND_STUB_STDINFILE"] = plannerStdin

	messageM := dcmMessageManifest(t, bin, "feat: add big")
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`)
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}

	cfg := config.Defaults()
	cfg.TokenLimit = 200 // ≪ 345 ⇒ the planner's TreeDiff gate MUST truncate
	deps := dcmDeps(t, repo, roles)

	// single:true ⇒ the planner's message is used directly (no separate message-agent call needed for
	// the invariant; the planner's TreeDiff is the gated path under test).
	if _, err := Decompose(context.Background(), deps); err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	data, err := os.ReadFile(plannerStdin)
	if err != nil {
		t.Fatalf("read planner stdin: %v (did the planner run? plannerM.Env had STDINFILE=%s)", err, plannerStdin)
	}
	captured := string(data)

	// (FR3j invariant) the planner's assembled prompt fits tokenLimit + separator allowance.
	measured := git.EstimateTokens(captured)
	if measured > cfg.TokenLimit+assembledPromptSeparatorTokens {
		t.Errorf("FR3j invariant violated (planner): EstimateTokens(planner stdin) = %d, want ≤ %d (tokenLimit %d + %d separator)\n"+
			"captured (first 400 chars): %q", measured, cfg.TokenLimit+assembledPromptSeparatorTokens,
			cfg.TokenLimit, assembledPromptSeparatorTokens, truncForLogD(captured, 400))
	}
	if !strings.Contains(captured, "[truncated]") {
		t.Errorf("expected '[truncated]' sentinel in the planner's stdin (tokenLimit=%d ≪ untruncated≈%d) — the gate did not truncate\n"+
			"captured (first 400 chars): %q", cfg.TokenLimit, git.EstimateTokens(body), truncForLogD(captured, 400))
	}
}

// truncForLogD is decompose_test.go's local truncation helper for readable failure messages (avoids
// colliding with any generate-package helper; the two packages are distinct).
func truncForLogD(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
```

> **Imports for decompose_test.go:** `os`, `path/filepath`, `strings`, `git`, `stubtest`, `config` —
> verify which are already imported (dcm* helpers use os/exec/strings; git is imported for DiffTreeNames
> in S1's tests). `assembledPromptSeparatorTokens` must be defined in decompose_test.go too (same
> constant; the two packages are distinct — do NOT cross-import). If a `truncForLog`-style helper
> already exists in decompose_test.go, reuse it instead of adding `truncForLogD`.
>
> **NOTE on the planner case:** the dcm* seam's planner JSON + the single-shortcut (single:true) path
> is the simplest decompose configuration that invokes the planner's TreeDiff. If the harness's
> Title/concept matching proves fiddly, the PRIMARY message-role case (file 1) alone satisfies the
> contract's core deliverable; this planner case is the documented "Consider" stretch (G10).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/generate/tokenlimit_invariant_test.go (the PRIMARY message-role test)
  - FILE: internal/generate/tokenlimit_invariant_test.go ; PACKAGE: generate (white-box — reuses
    generate_test.go's initRepo/writeFile/stageFile).
  - IMPORTS: context, os, path/filepath, strings, testing + internal/config, internal/git, internal/stubtest.
    (Drop time if unused.)
  - WRITE the file verbatim from §"(1) The PRIMARY message-role test": the assembledPromptSeparatorTokens
    const + truncForLog helper + TestCommitStaged_TokenLimitInvariant_AssembledPromptFits.
  - DO NOT: redeclare initRepo/writeFile/stageFile (they're in generate_test.go, same package); set a
    SystemPromptFlag on the stub manifest (G2); assert the literal ≤ tokenLimit (G1 — use +separator); add
    a //go:build e2e tag (G8); modify production code (G9).
  - VERIFY: go test -race ./internal/generate/ -run TestCommitStaged_TokenLimitInvariant -v → PASS.

Task 2: APPEND TestDecompose_TokenLimitInvariant_PlannerPromptFits to internal/decompose/decompose_test.go
  - FILE: internal/decompose/decompose_test.go ; PACKAGE: decompose (the dcm* harness's home).
  - IMPORTS: ensure os, path/filepath, strings, git, stubtest, config are imported (most already are);
    add the assembledPromptSeparatorTokens const (same value, distinct package — do NOT cross-import) +
    truncForLogD helper (or reuse an existing truncation helper if present).
  - APPEND the test verbatim from §"(2) The SECONDARY planner-role test".
  - DO NOT: use t.Setenv for the planner stdin (G4 — overwritten by later roles); use the tee-wrapper
    (D2 — role-specific Env is cleaner); touch production code; redeclare dcm* helpers.
  - VERIFY: go test -race ./internal/decompose/ -run TestDecompose_TokenLimitInvariant -v → PASS.
    (If the dcm* seam's planner invocation doesn't trigger the gate — e.g., the planner JSON doesn't
    produce a large-enough TreeDiff — increase `body` or lower cfg.TokenLimit. If the harness is
    uncooperative, ship Task 1 alone and mark Task 2 as a follow-up per G10.)

Task 3: VALIDATE — full gate set + scope discipline
  - RUN: gofmt -w internal/generate/tokenlimit_invariant_test.go internal/decompose/decompose_test.go ; gofmt -l .
  - RUN: go vet ./... ; go build ./...
  - RUN: go test -race ./internal/generate/ -run TestCommitStaged_TokenLimitInvariant -v  → PASS
  - RUN: go test -race ./internal/decompose/ -run TestDecompose_TokenLimitInvariant -v     → PASS
  - RUN: go test -race ./...   # whole repo green (additive tests; no production change)
  - RUN (scope): git status --porcelain → EXPECT EXACTLY:
        ?? internal/generate/tokenlimit_invariant_test.go
        M internal/decompose/decompose_test.go
  - RUN (no production diff): git diff --stat -- internal/git/ internal/generate/generate.go \
        internal/decompose/planner.go internal/decompose/message.go internal/decompose/arbiter.go → EMPTY.
  - RUN (no e2e build tag): grep -n 'go:build e2e' internal/generate/tokenlimit_invariant_test.go → NONE.
```

### Implementation Patterns & Key Details

```go
// === The +1 separator allowance (D1/G1) ===
// Render (render.go:157-158): when SystemPromptFlag == "" (the stub manifest), payload = sysPrompt +
// "\n\n" + userPayload. The MeasureAssembled closure measures EstimateTokens(sysPrompt + payload) (no
// separator). So:
//   capturedStdin            = sysPrompt + "\n\n" + payload
//   closure_measurement      = EstimateTokens(sysPrompt + payload)
//   EstimateTokens(captured) = ceil((runeCount(sysPrompt+payload) + 2) / 4)
//                            ≤ ceil(runeCount(sysPrompt+payload)/4) + 1   (ceil monotonicity on +2 runes)
//                            = closure_measurement + (0 or 1)
//   FR3j: closure_measurement ≤ tokenLimit
//   ⇒ EstimateTokens(captured) ≤ tokenLimit + 1
// Assert ≤ cfg.TokenLimit + assembledPromptSeparatorTokens (=1). DOCUMENTED.

// === Why the stub manifest has no SystemPromptFlag (G2) ===
// stubtest.Manifest sets PromptDelivery="stdin" but NOT SystemPromptFlag. So Render's prepend branch
// (render.go:157) fires ⇒ stdin = sysPrompt + "\n\n" + payload = the FULL assembled prompt (+ separator).
// This is what the test wants. Setting a SystemPromptFlag would route sysPrompt via the flag and leave
// stdin = payload only (missing sysPrompt) — the assertion would measure the wrong quantity.

// === Why role-specific Env isolates the planner's stdin (D2/G4) ===
// Render builds spec.Env = os.Environ() + manifest.Env (render.go). The planner manifest's Env carries
// STAGEHAND_STUB_STDINFILE=plannerFile; the stager/message/arbiter manifests do NOT. So:
//   - planner invocation: stub env has STDINFILE ⇒ writes stdin to plannerFile.
//   - stager/message/arbiter: stub env lacks STDINFILE ⇒ os.Getenv("") ⇒ drains to io.Discard.
// plannerFile holds ONLY the planner's stdin. (t.Setenv would be inherited by ALL invocations ⇒
// overwritten by the last. The per-role Env map is the isolation seam.)

// === Why force the gate to run (G3/D4) ===
// cfg.TokenLimit = 200 ≪ EstimateTokens(untruncated diff) ≈ 345. The water-fill MUST allocate < 345
// tokens ⇒ truncates the large file ⇒ emits "[truncated]". The closed-loop then re-measures the
// assembled prompt and re-trims if over 200. The "[truncated]" sentinel assertion proves the gate
// ACTIVELY ran (not a no-op where the payload fit without truncation).

// === Why capture the LAST invocation's stdin is correct for the message case (G5) ===
// CommitStaged's dedupe loop may retry, but the diff is captured ONCE (before the loop). The LAST stub
// invocation is the SUCCESSFUL one (the committed message) — its stdin has the gated diff + no rejection
// block (or the final accepted attempt). The closure measures the nil-rejected assembled prompt; the
// captured last/successful stdin matches it (+ separator). For the planner case, the LAST invocation is
// the arbiter/message — hence the role-specific Env isolation (G4).
```

### Integration Points

```yaml
TESTS (the two deliverables):
  - internal/generate/tokenlimit_invariant_test.go (NEW): TestCommitStaged_TokenLimitInvariant_AssembledPromptFits
  - internal/decompose/decompose_test.go (APPEND): TestDecompose_TokenLimitInvariant_PlannerPromptFits

CONSUMED (READ-ONLY — the code under test, all LANDED by T1/T2):
  - internal/git/tokengate.go: closedLoopGate (:195) — the gate; applyWaterFillGate (:135) → "[truncated]" sentinel
  - internal/generate/generate.go: CommitStaged (:158) + the message-role MeasureAssembled closure (T2.S1)
  - internal/decompose/planner.go: TreeDiff (:89) + the planner MeasureAssembled closure (:81, T2.S2)
  - internal/git/tokens.go: EstimateTokens (:25) — the single estimator (assert + the closure use it)
  - internal/provider/render.go:157-158: the sysPrompt + "\n\n" + payload prepend (the separator allowance)

REUSED HELPERS (do NOT redeclare):
  - generate_test.go: initRepo (:23), writeFile (:31), stageFile (:39)  [same package as file 1]
  - decompose_test.go: dcmInitRepo, dcmWriteFile, dcmPlannerManifest, dcmMessageManifest, dcmArbiterManifest,
    tooledStubManifest, dcmDeps  [same package as file 2]
  - stubtest: Build, Manifest

GATE: go test -race ./... → GREEN ; git status → ONLY the 2 test files (1 new, 1 appended)

NO-TOUCH (explicitly — owned by siblings / out of scope):
  - internal/git/tokengate.go + the 6 closure sites (generate.go, stagehand.go, hook/exec.go, planner.go,
    message.go, arbiter.go) — LANDED by T1/T2; read-only
  - internal/git/tokengate_test.go (S1's pure unit tests — parallel; no overlap)
  - internal/e2e/* (the //go:build e2e subprocess harness — this is in-process)
  - internal/generate/generate_test.go (the template — read-only; reuse its helpers)
  - docs/* (P1.M3); README.md; PRD.md, tasks.json, prd_snapshot.md, plan/*
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand

gofmt -w internal/generate/tokenlimit_invariant_test.go internal/decompose/decompose_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/generate/... ./internal/decompose/...   # Expected: exit 0
go build ./...                   # Expected: exit 0

# Expected: zero errors. If `undefined: initRepo` (file 1) or `undefined: dcmPlannerManifest` (file 2),
# the helper is in a same-package _test.go file — confirm the file is in the right package directory.
```

### Level 2: The Two Invariant Tests (the deliverable)

```bash
cd /home/dustin/projects/stagehand

go test -race ./internal/generate/ -v -run TestCommitStaged_TokenLimitInvariant
# Expected: PASS. Proves: EstimateTokens(captured stdin) ≤ tokenLimit(200) + 1; stdin contains "[truncated]".

go test -race ./internal/decompose/ -v -run TestDecompose_TokenLimitInvariant
# Expected: PASS. Proves: the planner's captured stdin ≤ tokenLimit + 1; contains "[truncated]".
```

### Level 3: Whole-Repository Regression + Scope Discipline

```bash
cd /home/dustin/projects/stagehand

go test -race ./...    # Expected: ALL packages green (additive tests; no production change)
go vet ./...           # Expected: exit 0

# Scope: ONLY the two test files.
git status --porcelain
# Expected EXACTLY:
#   ?? internal/generate/tokenlimit_invariant_test.go
#   M internal/decompose/decompose_test.go

# No production code touched (the gate + closures are LANDED):
git diff --stat -- internal/git/ internal/generate/generate.go internal/decompose/planner.go \
                   internal/decompose/message.go internal/decompose/arbiter.go internal/decompose/decompose.go
# Expected: EMPTY.

# No e2e build tag (this is in-process):
grep -rn 'go:build e2e' internal/generate/tokenlimit_invariant_test.go || echo "OK: no e2e build tag (standard test pass)"
```

### Level 4: Behavioral Cross-Check (manual repro of the invariant)

```bash
cd /home/dustin/projects/stagehand

# The PRIMARY test IS the proof (it asserts EstimateTokens(captured stdin) ≤ tokenLimit+1 against a real
# repo + the real closure + the real stub). For a manual cross-check of the separator math:
#   EstimateTokens("\n\n") should be 1 (ceil(2/4)):
cat > /tmp/sh_sep.go <<'EOF'
package main
import ("fmt";"github.com/dustin/stagehand/internal/git")
func main() { fmt.Printf("EstimateTokens(\"\\n\\n\") = %d (want 1)\n", git.EstimateTokens("\n\n")) }
EOF
go run /tmp/sh_sep.go ; rm -f /tmp/sh_sep.go
# Expected: EstimateTokens("\n\n") = 1 (want 1) — confirms the separator allowance.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.
- [ ] `go test -race ./internal/generate/ -run TestCommitStaged_TokenLimitInvariant` → PASS.
- [ ] `go test -race ./internal/decompose/ -run TestDecompose_TokenLimitInvariant` → PASS.

### Feature Validation
- [ ] The message-role test stages a large diff, sets TokenLimit ≪ untruncated, runs CommitStaged, and
      asserts `EstimateTokens(captured stdin) ≤ cfg.TokenLimit + assembledPromptSeparatorTokens`.
- [ ] The message-role test asserts the captured stdin contains `"[truncated]"` (the gate ran).
- [ ] The planner test isolates the planner's stdin via role-specific `Env["STAGEHAND_STUB_STDINFILE"]`
      (only the planner manifest), runs Decompose, and asserts the same invariant + sentinel.
- [ ] `assembledPromptSeparatorTokens` is `git.EstimateTokens("\n\n")` (= 1), documented as the Render
      separator allowance (not a magic number, not a violation).

### Scope Discipline Validation
- [ ] `git diff --stat` shows ONLY `internal/generate/tokenlimit_invariant_test.go` (new) +
      `internal/decompose/decompose_test.go` (appended).
- [ ] Did NOT touch any production code (tokengate.go, generate.go, planner.go, git.go, etc.).
- [ ] Did NOT edit S1's `internal/git/tokengate_test.go` (parallel pure unit tests).
- [ ] Did NOT edit the template `internal/generate/generate_test.go` (reused its helpers).
- [ ] Did NOT add a `//go:build e2e` tag (this is in-process; runs in `go test ./...`).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation
- [ ] Both tests run in the standard CI pass (stub agent, no real model, no e2e tag).
- [ ] The +1 separator allowance is documented (render.go:158; the closure's separator-free measurement).
- [ ] The "[truncated]" sentinel assertion proves the gate actively ran (not a no-op).
- [ ] The planner test uses role-specific Env (not t.Setenv, not the tee-wrapper) for stdin isolation.

---

## Anti-Patterns to Avoid

- ❌ Don't assert the literal `EstimateTokens(stdin) ≤ cfg.TokenLimit`. The stub's stdin includes the
  Render `"\n\n"` separator (render.go:158) that the closure's measurement lacks; the +1 allowance is
  PROVABLY the bound. The literal `≤` risks a 1-token flaky failure (G1/D1).
- ❌ Don't set a `SystemPromptFlag` on the stub manifest. With none, Render prepends sysPrompt to stdin ⇒
  the captured file IS the full assembled prompt. Setting one routes sysPrompt via the flag ⇒ stdin =
  payload only (missing sysPrompt) ⇒ the assertion measures the wrong quantity (G2).
- ❌ Don't set `cfg.TokenLimit` above the payload size. The gate must RUN (truncate) for the test to
  prove anything; a no-op fit is trivial. Make the untruncated prompt ≫ tokenLimit AND assert the
  "[truncated]" sentinel (G3/D4).
- ❌ Don't use `t.Setenv(STAGEHAND_STUB_STDINFILE, ...)` for the PLANNER case. A decompose run invokes
  the stub multiple times; the plain env var is inherited by all ⇒ overwritten by the last (arbiter/
  message). Set it on ONLY the planner manifest's `Env` map (role-specific isolation) (G4/D2).
- ❌ Don't add a `//go:build e2e` tag. This is an IN-PROCESS test (calls CommitStaged/Decompose directly);
  it runs in `go test ./...`. The `//go:build e2e` tag is the subprocess harness in internal/e2e/ (G8).
- ❌ Don't modify production code (the gate, the closures, the diff functions). They are LANDED by T1/T2
  and verified by S1's unit tests. If a test fails, debug the TEST setup (tokenLimit value, STDINFILE,
  role manifest), not the gate (G9).
- ❌ Don't duplicate S1's pure unit tests. S1 (internal/git/tokengate_test.go) covers closedLoopGate's
  convergence with synthetic measures; THIS task is the integration proof (real closure, real git, real
  stdin). Distinct layers (the S1 PRP's fence).
- ❌ Don't redeclare `initRepo`/`writeFile`/`stageFile` (generate_test.go) or `dcm*` (decompose_test.go).
  They're same-package helpers — reuse them.
- ❌ Don't cross-import between generate and decompose for the constant. `assembledPromptSeparatorTokens`
  is defined in EACH test file (same value, same doc) — the two packages are distinct (D3).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a focused, test-only change (one new file + one appended test), backed by a verified
template (`TestCommitStaged_ExcludedPayloadCapture` at generate_test.go:660 — the exact STDINFILE-capture
pattern, quoted verbatim), verified seam contracts (the closedLoopGate invariant, the closure bodies at
generate.go/planner.go:81, the Render separator at render.go:158 — all read directly), and the
role-specific Env isolation for the planner case (a clean, no-wrapper seam). Four independent
de-riskings: (1) the message-role PRIMARY case is a near-clone of an existing passing test
(ExcludedPayloadCapture) with the assertion swapped from content-substring to `EstimateTokens ≤ bound` —
the harness is proven; (2) the closed-loop gate + the 6 closures are CONFIRMED LANDED (T1/T2 Complete),
so the invariant the test asserts is the shipped behavior; (3) the `+1` separator allowance is
mathematically derived (`ceil(runes/4)` monotonicity on +2 runes), not heuristic — the assertion is
provably non-flaky; (4) the `"[truncated]"` sentinel assertion independently proves the gate ran (so a
wiring regression that nil-ed the closure fails on TWO assertions, not one). The CRITICAL gotchas
front-loaded — (G1) the +1 allowance, (G2) no SystemPromptFlag, (G3) force the gate to run, (G4)
role-specific Env for the planner — are the things an implementer would otherwise get wrong. The
residual uncertainty (not 10/10) is the SECONDARY planner case's dcm-harness cooperation (the planner
JSON / single-shortcut path / Title-matching is fiddly; if the harness doesn't trigger a large-enough
TreeDiff, the test needs a larger `body` or lower tokenLimit — mitigated by G10's "ship PRIMARY first"
guidance). No production-code risk (test-only); no parallel-edit risk (S1 is in internal/git/, which
this task doesn't touch).
