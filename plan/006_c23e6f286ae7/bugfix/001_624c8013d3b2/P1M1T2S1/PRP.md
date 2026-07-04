---
name: "P1.M1.T2.S1 — Add scenario F (decompose accidental double-run → Busy) to lock_scenarios_test.go"
description: |
  TEST-ONLY regression (Issue 1, Major — pins the documented decompose-path behavior). Add a new
  `F_DecomposeAccidentalDoubleRun_Busy` subtest to `TestE2ELockContention` in
  `internal/e2e/lock_scenarios_test.go`. With ONE untracked (unstaged) file → decompose activates → the
  holder takes the FR-M2b one-file shortcut (MESSAGE role only, satisfied by the stub), publishes
  `lock.SetSnapshot(tStart)` (decompose.go:169) BEFORE the message call, then sleeps. A contender on the
  same dirty tree (still nothing staged) hits `handleLockContention`: its `WriteTree()` returns `baseTree`
  ≠ the holder's `T_start` → **Busy(5)**, never the no-op fast path's 0. Reuses the scenario-B two-process
  skeleton (goroutine + waitForMarker + channel drain) verbatim; uses the shared outer-scope
  `bin`/`stub`/`cfg`. Asserts: contender ExitCode==5, stderr contains "already in progress", stderr does
  NOT contain "nothing to do"; holder drains to ExitCode==0. The assertion is timing-proof (Busy either
  way). No code/production/docs change; no new binary.
---

## Goal

**Feature Goal**: Pin Issue 1's documented behavior with an automated e2e regression so it can never
silently flip again: on the **decompose path** (nothing staged, dirty working tree), an accidental
double-run of stagehand exits **5 (Busy)** — never the no-op fast path's 0 — because the holder publishes
a working-tree snapshot (`T_start`) that a lock-free contender cannot reproduce from the index.

**Deliverable**: ONE new `t.Run("F_DecomposeAccidentalDoubleRun_Busy", …)` subtest appended to
`TestE2ELockContention` in `internal/e2e/lock_scenarios_test.go`, reusing the existing harness helpers
and the scenario-B two-process skeleton. No production code, no docs, no new binary.

**Success Definition**: `go test -tags e2e -run 'TestE2ELockContention/F_DecomposeAccidentalDoubleRun_Busy' ./internal/e2e/...` passes: the contender exits 5 with the "already in progress" message and WITHOUT "nothing to do"; the holder drains to exit 0. The existing scenarios A–E remain green. `go vet -tags e2e ./internal/e2e/...` is clean. No file outside `internal/e2e/lock_scenarios_test.go` is touched.

## User Persona

**Target User**: The Stagehand maintainer/reviewer guarding the FR52 run-lock behavior against regressions, and any future contributor who might (well-meaningly) try to make the no-op fast path fire on the decompose path.

**Use Case**: CI (or a local contributor) runs `go test -tags e2e ./internal/e2e/...` before/after changes to the lock, decompose, or contention-handler code. Scenario F fails loudly if anyone changes the decompose path to exit 0 on an accidental double-run (which the architecture has deliberately decided NOT to do — Option 1 of issue_analysis.md qualified the docs instead).

**User Journey**: a user double-taps a keybind bound to `stagehand` on a dirty, un-staged tree → the second invocation exits 5 (Busy) with a clear "already in progress" message → the user re-runs after the first finishes. Scenario F automates the regression check for exactly this.

**Pain Points Addressed**: Closes the exact coverage gap that let Issue 1 survive validation — the e2e suite covered the no-op fast path ONLY on the single-commit (staged) path (scenario B); there was NO decompose-path contention scenario. Scenario F is that scenario.

## Why

- **Issue 1 (Major) recommended adding exactly this e2e scenario.** `issue_analysis.md` and the bug report (h3.0) both conclude: "add an e2e scenario (`internal/e2e/lock_scenarios_test.go`) that reproduces the decompose accidental-double-run and asserts the *documented* exit code, so this cannot regress silently again." P1.M1.T1 (S1/S2/S3, parallel/complete) qualified the docs (README/cli.md/how-it-works.md) to scope exit-0 to the single-commit path; this task is the test that pins the behavior those docs now describe.
- **The behavior is correct; only the doc over-promised (now fixed).** The decompose path exits Busy(5) on an accidental double-run because the holder publishes a working-tree `T_start` that a lock-free contender cannot reproduce from the index (its `write-tree` returns `baseTree == HEAD^{tree}`). This is safe defense-in-depth. The fix chosen (Option 1) is doc-qualification + this regression test — NOT a code change.
- **The gap was structural in the test suite.** Scenario B stages a file (single-commit path) so both runs share an index tree; no scenario ever exercised the decompose contention path. Scenario F is the missing piece.
- **Lowest-risk change possible.** Pure test addition, reuses every primitive (harness + scenario-B skeleton + the existing stub binary), needs no config extras (defaults enable decompose), and the assertion is timing-proof (Busy either way). No production code can be broken by adding a test.

## What

A single `t.Run` subtest added inside `TestE2ELockContention` (after scenario E), following the scenario-B
two-process skeleton. Setup: `newRepo` + `seedCommit('readme.md','init')` + `writeFile('feature.txt','new work')`
(one untracked, unstaged file). Holder: stub env with `STAGEHAND_STUB_OUT='feat: add feature'`,
`STAGEHAND_STUB_MARKER=readiness`, `STAGEHAND_STUB_SLEEP_MS='4000'`, launched in a goroutine;
`waitForMarker(readiness, 10s)`. Contender: same repo, `STAGEHAND_STUB_OUT='feat: add feature'`, run
synchronously. Assert contender `ExitCode==5`, stderr contains "already in progress", stderr does NOT
contain "nothing to do". Drain holder via `<-resCh`, assert `ExitCode==0`. No production code, docs, or
binary change.

### Success Criteria

- [ ] `TestE2ELockContention` has a new `F_DecomposeAccidentalDoubleRun_Busy` subtest (after E).
- [ ] Setup uses EXACTLY ONE untracked, UNSTAGED file (`writeFile`, NOT `stageFile`) → decompose activates.
- [ ] Holder uses the scenario-B skeleton (goroutine + `waitForMarker` + buffered channel), with
      `STAGEHAND_STUB_OUT`, `STAGEHAND_STUB_MARKER`, `STAGEHAND_STUB_SLEEP_MS='4000'`.
- [ ] Contender runs synchronously against the same repo; asserts `ExitCode == 5`.
- [ ] Contender stderr contains `"already in progress"`.
- [ ] Contender stderr does NOT contain `"nothing to do"`.
- [ ] Holder drains (`<-resCh`) to `ExitCode == 0`.
- [ ] Reuses the outer-scope `bin`/`stub`/`cfg` (does NOT redeclare them).
- [ ] `go test -tags e2e -run 'TestE2ELockContention/F_DecomposeAccidentalDoubleRun_Busy' ./internal/e2e/...` passes.
- [ ] Existing scenarios A–E remain green (`go test -tags e2e -run TestE2ELockContention ./internal/e2e/...`).
- [ ] `go vet -tags e2e ./internal/e2e/...` clean.
- [ ] No file outside `internal/e2e/lock_scenarios_test.go` is modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP provides the copy-paste-ready subtest (verified against the live
scenario-B skeleton), names every harness helper it reuses (with the file/line), states the five
code-verified facts that make the scenario feasible (config defaults enable decompose; one-file shortcut
uses only the message role; SetSnapshot runs before the message call; the stub writes the marker then
sleeps; the contention handler's exact strings), proves the assertion is timing-proof, and dispels the
contract's risk notes (no extras needed). The e2e package compiles clean today; only one subtest is added.

### Documentation & References

```yaml
# MUST READ — the authoritative scenario-F design + feasibility + the proof it's timing-proof
- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/e2e_test_strategy.md
  why: "The complete scenario F design: the harness-primitive table, the canonical scenario-B skeleton (quoted verbatim), the new scenario-F skeleton (ready to paste), the feasibility analysis (one-file shortcut + SetSnapshot-before-message-call + stub marker/sleep ordering), and the assertion-robustness proof (ExitCode==5 regardless of snapshot timing)."
  critical: "This doc IS the implementation spec. The ONLY correction: its skeleton redeclares `cfg := writeStubConfig(...)` inside F — DROP that line and reuse the outer-scope `cfg` (scenarios A–E do; the outer TestE2ELockContention already builds bin/stub/cfg). Shadowing it would still work but is pointless churn."

- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/architecture/issue_analysis.md
  why: "Issue 1 root cause: on the decompose path the holder publishes a working-tree snapshot (T_start, decompose.go:169) while the contender computes baseTree (index) via write-tree → never matches → Busy(5). Recommends Option 1 (qualify docs + add this e2e scenario) over the riskier code change."
  critical: "Confirms the decompose→Busy behavior is CORRECT and safe; only the docs over-promised (fixed by S1/S2/S3). This test PINS that behavior. Do NOT attempt Option 2 (holder/contender snapshot-axis code change) — out of scope."

- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M1T2S1/research/scenario_F_notes.md
  why: "EMPIRICALLY VERIFIED (live code, 2026-07-03): the five load-bearing facts (config defaults enable decompose; one-file shortcut at decompose.go:187 uses MESSAGE role only; SetSnapshot at :169 is before the message call; stub marker/sleep ordering; contention handler exact strings + the no-op/Busy branch logic). Plus the timing-proof proof and the decisions D1–D8. READ THIS FIRST."
  critical: "§2 (the five verified facts) and §5 (dispelling the contract's risk notes) prevent the implementer from adding unnecessary config extras or hunting for non-existent failure modes. §4 is the copy-paste subtest with the outer-scope-cfg correction."

- file: internal/e2e/lock_scenarios_test.go
  why: "THE edit target. Contains TestE2ELockContention (the outer func that builds bin/stub/cfg at the top) + scenarios A–E as t.Run subtests. Scenario B (B_NoOpFastPath_AccidentalDoubleRun) is the canonical two-process skeleton to mirror — F is its decompose-path counterpart (untracked-not-staged file → assert Busy instead of 0)."
  pattern: "Each subtest: newRepo → seedCommit → writeFile[/stageFile] → readiness marker path → holder stubEnv (OUT+MARKER+SLEEP_MS) → goroutine + waitForMarker → contender runStagehand → assert ExitCode + stderr substrings → <-resCh drain + assert holder. F mirrors this exactly; the ONLY deltas are: writeFile NOT stageFile (decompose), and the Busy assertions."
  gotcha: "Use the OUTER-SCOPE bin/stub/cfg (declared at the top of TestE2ELockContention: `bin := buildStagehand(t); stub := buildStub(t); cfg := writeStubConfig(t, stub, \"\")`). Do NOT redeclare cfg inside F. The file's line-1 build tag (`//go:build e2e`) and package decl are already correct — append only the subtest."

- file: internal/e2e/harness_test.go
  why: "READ-ONLY ref. Every helper F reuses lives here: buildStagehand/buildStub (cached sync.Once builds), newRepo (git init + identity), seedCommit (write+add+commit), writeFile (working-tree write, does NOT stage), stageFile (git add), writeStubConfig (base [provider.stub] TOML + extras), stubEnv (os.Environ + KNOBS), runStagehand (subprocess, 60s timeout, --config/--no-color), waitForMarker (20ms poll), commitCount/runGit."
  pattern: "writeStubConfig(t, stubBin, extras) writes config_version=3 + [provider.stub] (command, prompt_delivery=stdin, output=raw, strip_code_fence, default_model=stub, tooled_flags). stubEnv(map) returns os.Environ()+the STAGEHAND_STUB_* knobs. runStagehand returns e2eResult{Stdout,Stderr,ExitCode}."

- file: internal/cmd/default_action.go
  why: "READ-ONLY ref (the code under test). handleLockContention (lines 241-256): `if snap := heldErr.Contents.Snapshot; snap != \"\" { contenderTree = WriteTree(); if match → exit 0 (\"nothing to do\") }` else `→ Busy(5)` with the \"already in progress\" message. shouldDecompose (:329, called :90) returns true with defaults when nothing is staged + dirty tree."
  pattern: "The no-op arm requires `snap != \"\"` AND `contenderTree == snap`. On decompose: snap=T_start (working-tree), contenderTree=baseTree (index) → never equal → Busy. Even if snap=\"\" (race), the no-op arm is skipped → Busy. Either way → 5."
  gotcha: "Do NOT edit this file — it is the production code under test. F verifies its behavior; it does not change it."

- file: internal/decompose/decompose.go
  why: "READ-ONLY ref (the holder's code path). FreezeWorkingTree (:165) resets .git/index to baseTree; lock.SetSnapshot(tStart) (:169) publishes T_start; the one-file check (:184-189) → runOneFileShortcut → MESSAGE-role agent call. This ordering is WHY waitForMarker ⇒ T_start is already published."
  gotcha: "Do NOT edit. F relies on this exact ordering (SetSnapshot before the message call) for the contention window to be deterministic."

- file: cmd/stubagent/main.go
  why: "READ-ONLY ref (the stub's ordering contract). main() order: drain stdin → write marker (STAGEHAND_STUB_MARKER) → sleep (STAGEHAND_STUB_SLEEP_MS) → stderr → stdout (STAGEHAND_STUB_OUT) → exit. So waitForMarker returning ⇒ holder drained stdin (message gen in flight) + is sleeping with the lock held."
  gotcha: "The marker is written BEFORE the sleep — this is the deterministic-race primitive (G-MARKER-IS-DETERMINISTIC). The stub is a single-response fake agent; STAGEHAND_STUB_OUT satisfies the one-file shortcut's MESSAGE role."

- docfile: plan/006_c23e6f286ae7/bugfix/001_624c8013d3b2/P1M1T1S3/PRP.md
  why: "The parallel sibling (doc-only: docs/how-it-works.md:155). Confirms NO .go/test file overlap — S3 touches only a docs file, F touches only a test file. No conflict."
  critical: "S3 qualifies the how-it-works.md 'No-op fast path' prose to scope exit-0 to the single-commit path and document decompose→Busy. F is the test that pins exactly that behavior. The two subtasks are complementary: S3 = the doc, F = the regression test."

# External references
- url: https://pkg.go.dev/testing#T.Run
  why: "Confirm t.Run subtests share the parent's *t and can reference outer-scope variables (bin/stub/cfg). This is why F does NOT redeclare cfg — it closes over the outer-scope vars like A–E do."
```

### Current Codebase Tree (relevant slice)

```bash
stagehand/
└── internal/e2e/
    ├── harness_test.go            # READ-ONLY — helpers (buildStagehand/newRepo/writeStubConfig/runStagehand/waitForMarker/…)
    ├── lock_scenarios_test.go     # EDIT TARGET — append the F subtest to TestE2ELockContention
    ├── scenarios_test.go          # READ-ONLY — the S1–S7 decompose scenarios (real-agent gated)
    └── hook_scenarios_test.go     # READ-ONLY — hook-mode scenarios
# (cmd/stubagent, internal/cmd, internal/decompose, internal/config are READ-ONLY — the code under test)
```

### Desired Codebase Tree After This Subtask

```bash
stagehand/
└── (only one existing file modified — no new files)
    internal/e2e/lock_scenarios_test.go   # +F_DecomposeAccidentalDoubleRun_Busy subtest (after E)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/e2e/lock_scenarios_test.go` | MODIFY (append one subtest) | Add `F_DecomposeAccidentalDoubleRun_Busy` to `TestE2ELockContention`, mirroring scenario B's skeleton with a one-untracked-file setup and Busy(5) assertions. |

**Explicitly NOT touched**: `internal/e2e/harness_test.go` (helpers), `internal/e2e/scenarios_test.go` +
`hook_scenarios_test.go` (other suites), `cmd/stubagent/main.go` (the stub binary), any production code
(`internal/cmd`, `internal/decompose`, `internal/lock`, `internal/config`, `internal/exitcode`), any docs
(`README.md`, `docs/*` — S1/S2/S3 own those), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — build tag): every e2e file starts with `//go:build e2e` (line 1). Default
// `go test ./...` SKIPS the e2e package. To run F: `go test -tags e2e -run
// 'TestE2ELockContention/F_DecomposeAccidentalDoubleRun_Busy' ./internal/e2e/...`. `go vet -tags e2e`
// is the compile check. Do NOT remove the build tag.

// CRITICAL (G2 — ONE untracked file, NOT staged): use writeFile(t, repo, "feature.txt", "new work"),
// NOT stageFile. Decompose activates iff NOTHING is staged (FR-M1). Staging the file would route to the
// single-commit path (scenario B's territory) and F would assert the wrong exit code.

// CRITICAL (G3 — reuse the outer-scope bin/stub/cfg): TestE2ELockContention declares
// `bin := buildStagehand(t); stub := buildStub(t); cfg := writeStubConfig(t, stub, "")` at the top.
// Scenarios A–E reference these directly. F must too — do NOT redeclare cfg inside F (the architecture
// skeleton's local `cfg :=` line would shadow pointlessly). cfg is shared and correct (defaults enable
// decompose; no extras needed).

// CRITICAL (G4 — the assertion is timing-proof, do not weaken it): assert ExitCode==5 unconditionally.
// Whether T_start is published when the contender checks (snap=T_start → WriteTree=baseTree ≠ T_start →
// Busy) or not yet (snap="" → no-op arm skipped → Busy), the result is 5. Do NOT add a "lenient" branch
// that tolerates 0 — the no-op fast path is structurally impossible on the decompose path (Issue 1).

// GOTCHA (G5 — "nothing to do" must NOT appear): assert `!strings.Contains(res2.Stderr, "nothing to do")`.
// This is the negative half of pinning Issue 1: the no-op fast path (exit 0 + "nothing to do") must not
// fire on the decompose path. Robust for the same timing reason as G4.

// GOTCHA (G6 — no config extras needed): the contract's risk notes ("add auto_stage_all=true",
// "[role.planner] extras") are NOT triggered. Defaults: AutoStageAll=true, Commits=0, Single=false
// (config.go). The one-file shortcut uses ONLY the message role → no planner invocation → no role
// extras. Verify by running F; if the holder errors before sleeping, read its stderr — but per the
// verified code paths it should reach the shortcut and sleep.

// GOTCHA (G7 — holder SLEEP_MS=4000, contender no sleep): the holder sleeps 4000ms so the lock is held
// while the contender runs. The contender hits Busy immediately (fails Acquire → handleLockContention →
// exits 5) long before the holder wakes. runStagehand's 60s timeout covers the holder's `<-resCh` drain.
// Do NOT reduce SLEEP_MS below ~2000ms (risk of the contender running before the holder acquires/publishes).

// GOTCHA (G8 — stub marker timing is the deterministic gate): waitForMarker(readiness, 10s) returns ONLY
// after the stub has drained stdin + written the marker (cmd/stubagent/main.go ordering: drain → marker →
// sleep). Since the marker is written DURING the message-role call, and SetSnapshot(tStart) ran BEFORE
// that call (decompose.go:169), waitForMarker ⇒ T_start is published. Do not add extra sleeps/gates.

// GOTCHA (G9 — do not edit production code): F is a regression test that verifies the EXISTING behavior
// of handleLockContention + Decompose. Do NOT change default_action.go, decompose.go, lock, config, or
// exitcode to make F pass. If F fails, the failure is either a test-setup bug (read holder stderr) or a
// real regression in production code (which a human must decide how to fix — not this test-only subtask).
```

## Implementation Blueprint

### Data models and structure

No new types. F reuses `e2eResult{Stdout, Stderr, ExitCode}` (from harness_test.go) and the existing
helpers. The subtest is pure orchestration: build → seed → launch holder → wait → run contender → assert
→ drain holder.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: APPEND the F subtest to TestE2ELockContention in internal/e2e/lock_scenarios_test.go
  - FILE: internal/e2e/lock_scenarios_test.go
  - LOCATE: the closing `})` of the E_DryRunAcquiresLock subtest (the last t.Run inside TestE2ELockContention),
    immediately before the TestE2ELockContention func's closing `}`.
  - INSERT (paste verbatim — reuses the outer-scope bin/stub/cfg; mirrors scenario B's skeleton):
        t.Run("F_DecomposeAccidentalDoubleRun_Busy", func(t *testing.T) {
            repo := newRepo(t)
            seedCommit(t, repo, "readme.md", "init")
            writeFile(t, repo, "feature.txt", "new work\n") // ONE untracked file, NOT staged → decompose activates

            readiness := t.TempDir() + "/ready.marker"
            holderEnv := stubEnv(map[string]string{
                "STAGEHAND_STUB_OUT":      "feat: add feature",
                "STAGEHAND_STUB_MARKER":   readiness,
                "STAGEHAND_STUB_SLEEP_MS": "4000",
            })

            resCh := make(chan e2eResult, 1)
            go func() { resCh <- runStagehand(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
            waitForMarker(t, readiness, 10*time.Second) // holder: lock held, T_start published, message-gen sleep

            // Contender: same dirty tree, still nothing staged → handleLockContention:
            //   WriteTree() = baseTree ≠ snap(T_start) → Busy(5). "nothing to do" must NOT appear.
            contenderEnv := stubEnv(map[string]string{"STAGEHAND_STUB_OUT": "feat: add feature"})
            res2 := runStagehand(t, bin, repo, cfg, contenderEnv, "--provider", "stub")
            if res2.ExitCode != 5 {
                t.Fatalf("contender exit = %d, want 5 (Busy) — decompose no-op fast path is structurally impossible; stderr:\n%s", res2.ExitCode, res2.Stderr)
            }
            if !strings.Contains(res2.Stderr, "already in progress") {
                t.Errorf("stderr missing busy message; got:\n%s", res2.Stderr)
            }
            if strings.Contains(res2.Stderr, "nothing to do") {
                t.Errorf("decompose path must NOT hit the no-op fast path; got:\n%s", res2.Stderr)
            }

            res := <-resCh // drain holder
            if res.ExitCode != 0 {
                t.Fatalf("holder exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
            }
        })
  - NAMING: the subtest name is exactly `F_DecomposeAccidentalDoubleRun_Busy` (matches the run-filter in
    the contract's verification command). Place it AFTER the E subtest (letter order A–F).
  - DEPENDENCIES: uses only the outer-scope bin/stub/cfg + the harness helpers (newRepo/seedCommit/
    writeFile/stubEnv/runStagehand/waitForMarker) + the stdlib (strings, time — already imported).
  - DO NOT: redeclare bin/stub/cfg; do NOT stage the file; do NOT add config extras; do NOT edit any
    production file or other test.
  - VERIFY compile: go vet -tags e2e ./internal/e2e/...

Task 2: VALIDATE — run F + the full contention suite + vet
  - RUN (the targeted subtest — the contract's verification command):
        go test -tags e2e -run 'TestE2ELockContention/F_DecomposeAccidentalDoubleRun_Busy' ./internal/e2e/... -v
    EXPECTED: PASS. Contender ExitCode==5; stderr has "already in progress"; no "nothing to do"; holder drains to 0.
  - RUN (the whole contention suite — A–E must stay green):
        go test -tags e2e -run TestE2ELockContention ./internal/e2e/... -v
    EXPECTED: 6/6 PASS (A,B,C,D,E,F).
  - RUN: go vet -tags e2e ./internal/e2e/...    # EXPECTED: clean (compile + vet under the e2e tag)
  - RUN: go test ./...                           # EXPECTED: green (default build; e2e skipped — sanity that no production code was touched)
  - FIX-FORWARD: if F's contender ExitCode != 5, READ res2.Stderr (printed by Fatalf). If it is 0 with
    "nothing to do", decompose did NOT activate (was the file staged? is the config default wrong?) — fix
    the test setup, not production. If the holder errors before sleeping (res.ExitCode != 0), READ
    res.Stderr — per the verified code paths the one-file shortcut should be reached; a holder error
    indicates a real regression or a setup bug to diagnose from the stderr.
```

### Implementation Patterns & Key Details

```go
// === Why F is scenario B's mirror, not a new pattern ===
// Scenario B (single-commit no-op fast path): writeFile + STAGE the file → both runs share index tree(a.txt)
//   → contender WriteTree == holder snapshot → exit 0 ("nothing to do").
// Scenario F (decompose Busy): writeFile, do NOT stage (one untracked file) → decompose activates →
//   holder publishes T_start (working-tree); contender WriteTree == baseTree (index) ≠ T_start → exit 5 (Busy).
// Same skeleton (goroutine + waitForMarker + channel drain); the deltas are setup (stage vs not) + assertions (0 vs 5).

// === Why the assertion is timing-proof (do not weaken it) ===
// handleLockContention: if snap != "" { if WriteTree()==snap → exit 0 } → else Busy.
//   snap=T_start published: WriteTree()=baseTree ≠ T_start → Busy.
//   snap="" (race):         the `snap != ""` guard skips the no-op arm → Busy.
// Either way → 5. Assert ExitCode==5 unconditionally; do NOT tolerate 0.

// === Why no config extras are needed ===
// config.go Defaults(): AutoStageAll=true, Commits=0 (auto), Single=false.
// shouldDecompose(cfg,false,false) with nothing staged + dirty tree → true.
// One untracked file → DiffTreeNames(baseTree,tStart)==1 → runOneFileShortcut → MESSAGE role only
// (no planner call). The base writeStubConfig(t,stub,"") TOML is sufficient.

// === The stub marker is the deterministic gate ===
// stubagent main(): drain stdin → write marker → sleep. waitForMarker(readiness,10s) returns ⇒
// holder holds the lock, froze the tree, published SetSnapshot(tStart) (which ran BEFORE the message
// call at decompose.go:169), and is sleeping. No extra sleeps/gates needed.
```

### Integration Points

```yaml
TEST (internal/e2e/lock_scenarios_test.go):
  - +F_DecomposeAccidentalDoubleRun_Busy subtest in TestE2ELockContention (after E)
  - reuses outer-scope bin/stub/cfg + harness helpers; mirrors scenario-B skeleton
  - asserts contender ExitCode==5 + "already in progress" + NOT "nothing to do"; holder drains to 0

CODE UNDER TEST (READ-ONLY — do NOT edit):
  - internal/cmd/default_action.go:handleLockContention  # the no-op-vs-Busy branch F exercises
  - internal/decompose/decompose.go                       # FreezeWorkingTree/SetSnapshot/one-file shortcut (holder path)
  - internal/lock/*                                       # Acquire → *HeldError (the contention primitive)
  - internal/config/config.go:Defaults                    # AutoStageAll/Commits/Single defaults enable decompose
  - cmd/stubagent/main.go                                 # the stub's drain→marker→sleep ordering

NO-TOUCH (explicitly):
  - internal/e2e/harness_test.go, scenarios_test.go, hook_scenarios_test.go   # other test files
  - any production package (cmd, decompose, lock, config, exitcode, generate, git)   # code under test
  - README.md, docs/*                           # S1/S2/S3 (doc qualification) + P1.M3 (coherence sweep)
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational):
  - P1.M2 (lock primitive hardening, Issues 2/3/4): orthogonal; F does not cover those.
  - P1.M3.T1 (Mode B coherence sweep): docs only; F is unaffected.
```

## Validation Loop

### Level 1: Compile & Vet (under the e2e build tag)

```bash
cd /home/dustin/projects/stagehand

go vet -tags e2e ./internal/e2e/...    # Expected: clean (the new subtest compiles; helpers resolve)
go build -tags e2e ./...               # Expected: exit 0 (full e2e-tagged build)

# Expected: Zero errors. The `//go:build e2e` tag means default `go vet ./...` skips this package —
# the -tags e2e form is mandatory for the compile check.
```

### Level 2: The Targeted Subtest (the contract's verification command)

```bash
cd /home/dustin/projects/stagehand

go test -tags e2e -run 'TestE2ELockContention/F_DecomposeAccidentalDoubleRun_Busy' ./internal/e2e/... -v

# Expected: PASS.
#   - contender ExitCode==5
#   - contender stderr contains "already in progress"
#   - contender stderr does NOT contain "nothing to do"
#   - holder drains to ExitCode==0
# (Builds the stagehand + stubagent binaries once via sync.Once; ~10-15s wall clock incl. the 4s holder sleep.)
```

### Level 3: Full Contention Suite (A–E stay green; F is the 6th)

```bash
cd /home/dustin/projects/stagehand

go test -tags e2e -run TestE2ELockContention ./internal/e2e/... -v
# Expected: 6/6 PASS (A_BusyRefusal_GenuineSecondBatch, B_NoOpFastPath_AccidentalDoubleRun,
#           C_NoStaleLock_AfterExit, D_ReadOnlyBypass, E_DryRunAcquiresLock,
#           F_DecomposeAccidentalDoubleRun_Busy).

# Default-build sanity (e2e skipped — confirms no production code was touched)
go test ./...
# Expected: all green (e2e skipped by the build tag).
```

### Level 4: Behavioral Cross-Check (manual reproduction of Issue 1, if F is flaky)

```bash
cd /home/dustin/projects/stagehand

# If F ever fails, reproduce Issue 1 manually to diagnose. Build the binaries:
go build -o /tmp/stagehand ./cmd/stagehand && go build -o /tmp/stubagent ./cmd/stubagent

# In a temp repo with ONE untracked (unstaged) file:
TMP=$(mktemp -d); cd "$TMP"; git init -q; git config user.name t; git config user.email t@t
echo init > readme.md; git add readme.md; git commit -qm seed
echo "new work" > feature.txt        # ONE untracked file, NOT staged → decompose activates
cat > /tmp/cfg.toml <<EOF
config_version = 3
[provider.stub]
command = "/tmp/stubagent"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
EOF

# Holder (terminal 1): sleeps 6s holding the lock, publishes snapshot=T_start:
STAGEHAND_STUB_OUT="feat: add feature" STAGEHAND_STUB_SLEEP_MS=6000 /tmp/stagehand --config /tmp/cfg.toml --provider stub

# Contender (terminal 2, during the 6s sleep): same repo, same dirty tree, nothing staged:
STAGEHAND_STUB_OUT="feat: add feature" /tmp/stagehand --config /tmp/cfg.toml --provider stub; echo "EXIT=$?"
# Expected: EXIT=5 and stderr "stagehand: another stagehand run is already in progress on …".
# This is exactly what F asserts. (Issue 1's doc said exit 0; the corrected docs + F say exit 5.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go vet -tags e2e ./internal/e2e/...` clean (the subtest compiles).
- [ ] `go test -tags e2e -run 'TestE2ELockContention/F_DecomposeAccidentalDoubleRun_Busy' ./internal/e2e/...` PASS.
- [ ] `go test -tags e2e -run TestE2ELockContention ./internal/e2e/...` → 6/6 PASS (A–F).
- [ ] `go test ./...` green (default build; production code untouched).

### Feature Validation

- [ ] `F_DecomposeAccidentalDoubleRun_Busy` exists in `TestE2ELockContention` (after E).
- [ ] Setup uses `writeFile` (NOT `stageFile`) for exactly ONE untracked file → decompose activates.
- [ ] Holder uses the scenario-B skeleton (goroutine + `waitForMarker` + buffered channel) with OUT/MARKER/SLEEP_MS=4000.
- [ ] Contender asserts `ExitCode == 5`.
- [ ] Contender stderr contains `"already in progress"`.
- [ ] Contender stderr does NOT contain `"nothing to do"`.
- [ ] Holder drains (`<-resCh`) to `ExitCode == 0`.
- [ ] Reuses the outer-scope `bin`/`stub`/`cfg` (no redeclaration).

### Scope Discipline Validation

- [ ] ONLY `internal/e2e/lock_scenarios_test.go` modified (`git diff --stat` confirms; one subtest appended).
- [ ] Did NOT edit `harness_test.go`, `scenarios_test.go`, `hook_scenarios_test.go`, or any production package.
- [ ] Did NOT edit `cmd/stubagent/main.go` (the stub binary is reused as-is).
- [ ] Did NOT edit `README.md` or any `docs/*` (S1/S2/S3 + P1.M3 own those).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research note).

### Code Quality Validation

- [ ] Mirrors scenario B's skeleton (the canonical two-process contention pattern); no new pattern invented.
- [ ] Subtest name matches the contract's run-filter (`F_DecomposeAccidentalDoubleRun_Busy`).
- [ ] Assertions are timing-proof (ExitCode==5 unconditionally; no lenient 0-branch).
- [ ] The negative assertion (`!Contains "nothing to do"`) pins the Issue-1 behavior precisely.

---

## Anti-Patterns to Avoid

- ❌ Don't STAGE the file (`stageFile`) — decompose activates only when NOTHING is staged (FR-M1). Use
  `writeFile` only. Staging routes to the single-commit path and F would assert the wrong code (gotcha G2).
- ❌ Don't redeclare `bin`/`stub`/`cfg` inside F — reuse the outer-scope vars like A–E do. The architecture
  skeleton's local `cfg :=` line is pointless shadowing (gotcha G3).
- ❌ Don't weaken the assertion to tolerate `ExitCode == 0`. The no-op fast path is structurally impossible
  on the decompose path (Issue 1); the assertion is timing-proof at 5 either way (gotcha G4).
- ❌ Don't drop the `!Contains("nothing to do")` negative assertion — it is the precise pin for Issue 1
  (the no-op fast path must not fire on decompose) (gotcha G5).
- ❌ Don't add config extras (`auto_stage_all`, `[role.planner]`) prophylactically — the defaults enable
  decompose and the one-file shortcut uses only the message role. Add them ONLY if a real run shows the
  holder errors before sleeping (gotcha G6; per verified paths it will not).
- ❌ Don't reduce `STAGEHAND_STUB_SLEEP_MS` below ~2000ms — the contender must run while the holder holds
  the lock + has published T_start (gotcha G7).
- ❌ Don't add extra sleeps/gates beyond `waitForMarker` — the stub's drain→marker→sleep ordering IS the
  deterministic gate, and SetSnapshot runs before the message call (gotcha G8).
- ❌ Don't edit production code (`default_action.go`, `decompose.go`, `lock`, `config`, `exitcode`) to make
  F pass. F verifies existing behavior; a failure is a test-setup bug or a real regression for a human to
  triage (gotcha G9).
- ❌ Don't drop the `//go:build e2e` tag or run with the default build — e2e is tag-gated (gotcha G1).
- ❌ Don't edit other test files (`harness_test.go`, `scenarios_test.go`, `hook_scenarios_test.go`) or any
  docs (`README.md`, `docs/*`) — those are out of scope.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a single-subtest addition that reuses every primitive — the harness helpers
(harness_test.go), the canonical scenario-B two-process skeleton, and the existing stubagent binary — and
the scenario-F skeleton is supplied copy-paste-ready by the architecture `e2e_test_strategy.md` (with one
trivial correction: reuse the outer-scope `cfg` instead of shadowing). The five load-bearing feasibility
facts are verified against live code (not left as risks): config defaults enable decompose
(`AutoStageAll=true, Commits=0, Single=false`); the one-file shortcut at decompose.go:187 uses the MESSAGE
role only (no planner, no role extras); `SetSnapshot(tStart)` at :169 runs before the message call; the
stub writes the marker then sleeps; and `handleLockContention`'s exact branch logic + assertion strings
("already in progress" / "nothing to do") are quoted verbatim. The e2e package compiles clean today
(`go vet -tags e2e` exit 0). The assertion is provably timing-proof (Busy whether or not T_start is
published yet), so microsecond scheduling jitter cannot cause flakes. The residual uncertainty (not 10/10)
is the wall-clock nature of e2e (a 4s holder sleep + subprocess scheduling on a loaded CI box could, in
principle, need a longer `waitForMarker` timeout — but 10s already gives 6s of slack over the 4s sleep),
and the small possibility that an environment-specific stub/config quirk makes the holder error before
sleeping — mitigated by the explicit "read res.Stderr" fix-forward guidance and the verified code paths
showing the one-file shortcut is reached. No production code, docs, or other tests are in scope, so the
blast radius is one subtest in one file.
