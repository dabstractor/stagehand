# E2e test for assembled-prompt-≤-token_limit invariant — P1.M1.T3.S2 Research

> Verified against the live repo (HEAD = 35ae0ca, module `github.com/dustin/stagecoach`). All
> signatures/line numbers confirmed by direct read. No files modified — research only.

## 1. What this task is (the contract, restated)

An INTEGRATION-level proof of the FR3j hard invariant (PRD §9.1 FR3j): after the closed-loop gate
runs, `EstimateTokens(assembledFullPrompt) ≤ token_limit`, ALWAYS — proven end-to-end against a real
git repo (temp repo + stub provider, standard test pass, no real model). The pure-unit proof (S1,
P1.M1.T3.S1) covers `closedLoopGate`'s convergence with SYNTHETIC measures; THIS task drives the FULL
real path: real git diff capture → the REAL `MeasureAssembled` closure → `closedLoopGate` →
`provider.Execute` (the stub) → capture the stub's received stdin → measure it → assert it fits.

The contract names TWO consumer paths:
- **PRIMARY (message-role):** `CommitStaged` → `StagedDiff` (the gate's call site at generate.go).
- **SECONDARY ("Consider"):** the decompose **planner** → `TreeDiff` (planner.go:89).

## 2. The closedLoopGate invariant + what the closure measures (verified)

`closedLoopGate` (internal/git/tokengate.go:195, LANDED) guarantees: when `measure != nil`,
`measure(gatedDiff) ≤ tokenLimit`, where `gatedDiff` is the returned string. The `measure` callback is
the `MeasureAssembled` closure wired by T2.S1/T2.S2 (Complete):

- **Message-role closure** (generate.go, wired by T2.S1):
  `func(gatedDiff string) int { return git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, cfg.Context, nil)) }`
- **Planner closure** (planner.go:81, wired by T2.S2):
  `func(gatedDiff string) int { return git.EstimateTokens(sysPrompt + prompt.BuildPlannerUserPayload(gatedDiff, cfg.Context, forcedCount)) }`

So the invariant is on `EstimateTokens(sysPrompt + <role payload>(gatedDiff))` — the assembled prompt
MINUS any separator. This is the quantity the test must prove fits.

## 3. The Render separator (the one test-side subtlety)

`provider.Render` (render.go:157-158) prepends the system prompt to the stdin payload when the manifest
has no `system_prompt_flag` (the stub manifest from `stubtest.Manifest` does NOT set one):

```go
if *r.SystemPromptFlag == "" && sysPrompt != "" {
    payload = sysPrompt + "\n\n" + userPayload   // render.go:158
}
```

So the stub's **received stdin** = `sysPrompt + "\n\n" + <role payload>(gatedDiff)`, while the closure
measures `sysPrompt + <role payload>(gatedDiff)` (Go `+`, NO separator). The delta is the `"\n\n"`
(2 runes). `EstimateTokens = ceil(runeCount/4)` (tokens.go:25), so adding 2 runes raises the estimate
by **at most 1**:

```
EstimateTokens(stdin) − closure_measurement ∈ {0, 1}
⇒ EstimateTokens(stdin) ≤ closure_measurement + 1 ≤ tokenLimit + 1   (by FR3j)
```

**Decision D1 (the assertion):** assert `EstimateTokens(capturedStdin) ≤ cfg.TokenLimit + assembledPromptSeparatorTokens`
where `assembledPromptSeparatorTokens = git.EstimateTokens("\n\n")` (= 1). This is PROVABLY correct
(FR3j on the closure + the bounded separator) and not flaky. Asserting the literal `≤ tokenLimit`
RISKS a 1-token failure when the closure lands at exactly `tokenLimit` and the separator pushes the
stdin over a token boundary. The `+1` is the Render separator artifact, DOCUMENTED — not a violation of
FR3j (which is on the separator-free assembled prompt). (Existing tests document the same separator:
generate_test.go:741 "provider.Render prepends the system prompt to the payload with a '\n\n' delimiter".)

## 4. The STDINFILE capture pattern (verified — the template)

`TestCommitStaged_ExcludedPayloadCapture` (generate_test.go:660-731) is the exact template:

```go
stdinFile := filepath.Join(t.TempDir(), "stdin.txt")
t.Setenv("STAGECOACH_STUB_STDINFILE", stdinFile)   // stub writes its received stdin here
stub := stubtest.Build(t)
m := stubtest.Manifest(stub, stubtest.Options{Out: "feat: add feature"})
// ... cfg, deps, CommitStaged ...
data, err := os.ReadFile(stdinFile)   // the captured assembled prompt
payload := string(data)
```

The stub agent (cmd/stubagent) tees stdin to `STAGECOACH_STUB_STDINFILE` AFTER draining (deadlock guard).
`stubtest.Manifest` does NOT set `SystemPromptFlag` ⇒ Render prepends sysPrompt+"\n\n"+payload to stdin
⇒ the captured file IS the full assembled prompt (+ separator). This is the mechanism for BOTH cases.

**One overwrite caveat (the decompose case):** `STAGECOACH_STUB_STDINFILE` captures only the LAST
invocation's stdin (generate_multiturn_test.go:667 documents this). A decompose run invokes the stub
multiple times (planner → stager → message → arbiter), so a plain `t.Setenv` would be overwritten by
later invocations. TWO ways to isolate the planner's stdin:
- **(a) Role-specific Env (PREFERRED — clean):** set `STAGECOACH_STUB_STDINFILE` on ONLY the planner
  manifest's `Env` map (`plannerM.Env["STAGECOACH_STUB_STDINFILE"] = plannerFile`). The message/stager/
  arbiter manifests do NOT carry it ⇒ their stub invocations see `os.Getenv("")` ⇒ drain to io.Discard
  ⇒ never touch plannerFile. (Render builds `spec.Env = os.Environ() + manifest.Env`; the per-role Env
  is the isolation seam.)
- **(b) tee-wrapper** (generate_multiturn_test.go:684+): a `/bin/sh` wrapper that appends each
  invocation's stdin to a capture file; take the FIRST block (the planner). More complex; avoid here.

Decision D2: use (a) — role-specific Env — for the planner case. It's clean, no shell wrapper, and the
per-role Env map is the natural isolation seam.

## 5. The message-role case (PRIMARY) — design

File: a NEW focused test file `internal/generate/tokenlimit_invariant_test.go` (`package generate`).
(generate_test.go is already ~900 lines; the generate package convention is focused sibling test files
— multiturn_test.go, generate_multiturn_test.go, hooks_freeze_test.go, invariants_test.go. This groups
the invariant test coherently and keeps generate_test.go stable. The contract's "generate_test.go" is
honored in spirit — same package, same pattern.)

`TestCommitStaged_TokenLimitInvariant_AssembledPromptFits`:
1. `repo := t.TempDir(); initRepo(t, repo)` (reuse generate_test.go's `initRepo`).
2. Seed enough content that the untruncated assembled prompt EXCEEDS the tokenLimit: write a file with
   ~1600 runes of changes (`strings.Repeat("change line N\n", 120)` ≈ 1560 runes ⇒ EstimateTokens ≈ 390).
   Stage it.
3. `stdinFile := filepath.Join(t.TempDir(), "stdin.txt"); t.Setenv("STAGECOACH_STUB_STDINFILE", stdinFile)`.
4. `cfg := config.Defaults(); cfg.TokenLimit = 200` (≪ 390 ⇒ the gate MUST truncate to fit; forces the
   closed-loop to run). `m := stubtest.Manifest(stub, stubtest.Options{Out: "feat: big change"})`.
5. `deps := Deps{Git: git.New(repo), Manifest: m}`; `CommitStaged(ctx, deps, cfg)`.
6. Read `stdinFile` → `capturedStdin`.
7. **Assert (the invariant):** `git.EstimateTokens(capturedStdin) ≤ cfg.TokenLimit + assembledPromptSeparatorTokens`
   (assembledPromptSeparatorTokens = `git.EstimateTokens("\n\n")` = 1).
8. **Assert (the gate RAN):** the captured stdin contains a `"[truncated]"` sentinel — `truncateByWaterFill`
   emits `... [truncated]` per over-budget file. This proves the closed-loop ACTIVELY truncated (not a
   no-op where the payload happened to be small). AND `EstimateTokens(capturedStdin)` is well under the
   untruncated size (≈390 → gated ≤ 201).

## 6. The planner case (SECONDARY — "Consider") — design

File: APPEND to `internal/decompose/decompose_test.go` (`package decompose` — the natural home; the
dcm* harness lives there). Drives `Decompose()` with cfg.TokenLimit set + captures the planner's stdin
via role-specific Env (D2).

`TestDecompose_TokenLimitInvariant_PlannerPromptFits`:
1. `repo := t.TempDir(); dcmInitRepo(t, repo)`; write a large mixed working-tree diff (untracked files,
   nothing staged — the FR-M1 decompose trigger). Enough content that the planner's TreeDiff exceeds
   tokenLimit.
2. `plannerStdin := filepath.Join(t.TempDir(), "planner-stdin.txt")`.
3. Build the planner manifest: `plannerM := dcmPlannerManifest(t, bin, plannerJSON)` THEN
   `plannerM.Env["STAGECOACH_STUB_STDINFILE"] = plannerStdin` (isolate the planner's stdin — D2). The
   stager/message/arbiter manifests do NOT carry STDINFILE ⇒ they drain to io.Discard.
4. `cfg := config.Defaults(); cfg.TokenLimit = <small>`; `deps := dcmDeps(t, repo, roles)` with the
   planner manifest wired.
5. `Decompose(ctx, deps)` (the planner is invoked first; its stdin → plannerStdin).
6. Read `plannerStdin` → `capturedPlannerStdin`.
7. **Assert:** `git.EstimateTokens(capturedPlannerStdin) ≤ cfg.TokenLimit + assembledPromptSeparatorTokens`.
8. **Assert (gate ran):** contains `"[truncated]"`.

(If the decompose harness proves fiddly — the dcm* seam's Title-matching, the 2-concept planner JSON —
the message-role PRIMARY case alone satisfies the contract's core deliverable. The planner case is the
"Consider" stretch; the PRP specifies it fully but flags it as secondary.)

## 7. Why the stub manifest has no SystemPromptFlag (and why that's correct)

`stubtest.Manifest` builds a Manifest with `PromptDelivery="stdin"`, `Output="raw"`,
`StripCodeFence=true`, but does NOT set `SystemPromptFlag` (it's nil ⇒ Resolve defaults to ""). This is
EXACTLY what the invariant test needs: with no system_prompt_flag, Render prepends `sysPrompt + "\n\n" +
payload` to stdin (render.go:158), so the captured stdin = the FULL assembled prompt (sysPrompt + payload
+ separator) — the closest observable to the closure's measurement. Setting a SystemPromptFlag would
send sysPrompt via the flag and stdin = payload ONLY (missing sysPrompt) — WRONG for this invariant.

## 8. Decisions log

- **D1** Assert `EstimateTokens(stdin) ≤ tokenLimit + EstimateTokens("\n\n")` (= tokenLimit + 1). The
  +1 is the Render separator (render.go:158 `sysPrompt + "\n\n" + payload`); FR3j's invariant is on the
  separator-free `sysPrompt + payload` (the closure's measurement). Provable + non-flaky.
- **D2** Isolate the planner's stdin via role-specific `Env["STAGECOACH_STUB_STDINFILE"]` on ONLY the
  planner manifest (not the tee-wrapper). The per-role Env is the natural seam; later role invocations
  drain to io.Discard.
- **D3** PRIMARY case in a NEW file `internal/generate/tokenlimit_invariant_test.go` (focused sibling;
  generate_test.go is ~900 lines). SECONDARY case appended to `internal/decompose/decompose_test.go`
  (the dcm* harness's home). Each test lives with the code it drives — no cross-package dependency.
- **D4** Force the gate to run by making the untruncated prompt ≫ tokenLimit (stage ~1600 runes,
  tokenLimit=200). AND assert the `"[truncated]"` sentinel is present (proves the closed-loop actively
  truncated, not a no-op).
- **D5** `config.Defaults()` + `cfg.TokenLimit = <small>` (Defaults has Timeout=120s, which is fine for
  the instant stub). The stub manifest is `stubtest.Manifest(stub, Options{Out: "feat: ..."})`.
- **D6** Both tests run in the STANDARD test pass (no `//go:build e2e` tag — that's the subprocess
  harness in internal/e2e/; this is in-process via CommitStaged/Decompose). Stub agent, no real model.

## 9. Scope fence (the plan)

- **S1 (Implementing, in parallel)** = PURE unit tests for `closedLoopGate` (synthetic measures,
  internal/git/tokengate_test.go). DISTINCT layer — no overlap with this integration test.
- **S2 (this)** = the INTEGRATION proof (real closure, real git, stub provider, captured stdin).
- **T1.S1/T1.S2/T2.S1/T2.S2** = the gate + the 6 MeasureAssembled wirings (all Complete — the code
  under test). READ-ONLY for this task.

DO NOT: modify the gate (tokengate.go), the closures (generate.go/planner.go/etc.), or any production
code. Test-only. No docs (P1.M3). No `//go:build e2e` (this is in-process).
