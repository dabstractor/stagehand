---
name: "P1.M1.T4.S2 — Integration: mid-turn failure → rescue; small-payload skip; non-append provider skip"
description: |
  Add the multi-turn FAILURE-PATH integration tests (PRD §9.24 FR-T7 + the FR-T1 gate's NEGATIVE
  conditions b/d). Three scenarios, each asserting the FULL rescue invariant (Kind, TreeSHA==frozen,
  ParentSHA, atomic-HEAD, idempotent-index §20.2) AND a stub-counter call-count that proves whether
  the multi-turn phase was entered:
    (a) MID-TURN FAILURE → RESCUE: SessionMode="append" + large payload + GLOBAL STAGECOACH_STUB_EXIT=1
        ⇒ one-shot exits-1-but-parses-empty (exhausts ⇒ gate fires) ⇒ Run's first turn exits 1 ⇒ Run
        aborts (FR-T7) ⇒ *RescueError{Kind:ErrRescue}. Counter == 2 (1 one-shot + 1 turn). Cause != nil.
    (b) SMALL-PAYLOAD SKIP: SessionMode="append" + TINY diff + default chunkTokens(32000) ⇒ cond (b)
        false ⇒ gate skips Run ⇒ existing rescue. Counter == 1 (one-shot ONLY; multi-turn NEVER entered).
    (c) NON-APPEND PROVIDER SKIP: SessionMode unset (claude-shaped) + large payload ⇒ cond (d) false ⇒
        gate skips Run ⇒ existing rescue. Counter == 1.
  PRD §9.24 FR-T1 (b/d negative), FR-T7 (failure is pure upside); §18.1 (atomic-HEAD); §20.2
  (idempotent-index); research-tests-ui.md §2 (rescue + idempotent templates); research-generate-config.md
  §5 (multi-turn failure ⇒ &RescueError{Kind:ErrRescue}, byte-identical to one-shot; use ErrRescue NOT
  ErrTimeout).

  ⚠️ **NON-NEGOTIABLE: do NOT duplicate T3.S3.** `internal/generate/generate_test.go` ALREADY contains
  (COMPLETE, lines 868-941): `TestCommitStaged_MultiTurnFallbackSuccess` (happy path),
  `TestCommitStaged_MultiTurnSkipped_NonAppend` (line 895), `TestCommitStaged_MultiTurnSkipped_SmallPayload`
  (line 918), and `TestCommitStaged_MultiTurnDuplicateRescue` (line 941). Those four assert ONLY
  `errors.As(err,&re) && re.Kind==ErrRescue` — they do NOT assert the idempotent-index invariant, do NOT
  assert TreeSHA==frozen, and do NOT prove (via the stub counter) that multi-turn was/wasn't entered.
  T4.S1 (parallel sibling) owns the HAPPY-PATH RENDER CONTRACT in a NEW file `generate_multiturn_test.go`.
  ⇒ T4.S2's UNIQUE contribution is the FAILURE/INVARIANT depth: (a) the mid-turn Execute-error path
  (entirely uncovered — the duplicate test is a FINAL-turn dedupe failure, a different code branch), and
  (b)/(c) the idempotent-index + counter-not-invoked strengthenings of the skip paths. Put the tests in a
  NEW, distinctly-named file `internal/generate/generate_multiturn_failure_test.go` (avoids file-level
  collision with T4.S1's `generate_multiturn_test.go`). REUSE generate_test.go helpers (same package).

  ⚠️ **THE mechanism for a mid-turn failure — GLOBAL `STAGECOACH_STUB_EXIT=1`, NO stub change.** The stub
  (`cmd/stubagent/main.go`) has ONE exit knob — `STAGECOACH_STUB_EXIT` (envInt, applied to EVERY call);
  `STAGECOACH_STUB_SCRIPT` varies OUTPUT only, never exit code. There is NO per-call exit without modifying
  the stub, which T4.S1's PRP forbids and this item's "existing harness" constraint discourages. So set
  `m.Env["STAGECOACH_STUB_EXIT"]="1"` GLOBALLY: the one-shot exits 1 BUT its stdout is still `""` (script[0])
  ⇒ CommitStaged's non-zero-exit branch (generate.go ~line 258: `lastCause=execErr`, fall through) runs
  ParseOutput("") ⇒ ok=false ⇒ exhausts (MaxDuplicateRetries=0) ⇒ the FR-T1 gate fires (all 4 conds hold)
  ⇒ Run's turn 1 exits 1 ⇒ `provider.Execute` returns a wrapped `*exec.ExitError` (executor.go:77) ⇒ Run
  returns `cause != nil` (multiturn.go turn-1 branch) ⇒ the gate sets `lastCause=cause` and falls through
  to the byte-identical rescue (generate.go:289, `Kind:ErrRescue`). The failure lands on turn 1 (Run aborts
  at the FIRST error — same `return "",false,execErr` branch as a later turn), which fully exercises FR-T7.
  The contract's "exit 1 on turn 2" was illustrative; turn-1 failure is equivalent coverage.

  ⚠️ **THE counter is the discriminator between "gate fired + Run aborted" and "gate skipped".**
  `stubtest.NewScript`/`appendScriptManifest` create a call-counter file
  (`m.Env["STAGECOACH_STUB_COUNTER"]`); the stub increments it once per process (cmd/stubagent selectScripted
  reads N, writes N+1). So after a run: counter=="1" ⇒ ONLY the one-shot ran (gate SKIPPED Run — conds b/d
  false); counter=="2" ⇒ one-shot + exactly 1 multi-turn turn (gate FIRED, Run aborted at turn 1). This is
  the assertion T3.S3's skip tests LACK — they cannot prove multi-turn was never entered.

  ⚠️ **Use ErrRescue, NOT ErrTimeout.** research-generate-config.md §5 + the gate (generate.go ~318-326):
  a multi-turn turn error maps to `&RescueError{Kind:ErrRescue}` (exit 3), byte-identical to one-shot-
  exhaustion. `ErrTimeout` (exit 124) is reserved for the ONE-shot kill (DeadlineExceeded). Asserting
  `re.Kind==ErrRescue` IS the proof that "failure is never worse than one-shot-exhausted" (FR-T7) — same
  Kind, same TreeSHA, same ParentSHA ⇒ same FormatRescue message + exit 3.

  Deliverable: ONE new file `internal/generate/generate_multiturn_failure_test.go` (`package generate`,
  white-box) with a shared helper `assertMultiTurnRescue(t,repo,m,cfg,wantCalls)` (the full invariant:
  Kind/TreeSHA/ParentSHA/atomic-HEAD/idempotent-index/counter) + THREE tests (a)/(b)/(c). INPUT = wired
  CommitStaged with the FR-T1 gate (P1.M1.T3.S3 COMPLETE) + pi-style SessionMode (the tests set it on the
  stub manifest directly via appendScriptManifest). Touches ONLY the new test file. Test-only; no production
  code, no stub change, no go.mod, no docs. Non-overlapping with T3.S3 (Kind-only skip tests), T3.S4
  (multiturn_test.go unit tests), and T4.S1 (render-contract test in generate_multiturn_test.go).
---

## Goal

**Feature Goal**: Prove, via three end-to-end integration tests against the stub agent, that the
multi-turn fallback's FAILURE and SKIP paths satisfy PRD §9.24 FR-T7 ("failure is pure upside — never
worse than one-shot-exhausted") and the FR-T1 gate's NEGATIVE conditions (b small-payload, d non-append).
Each test asserts the FULL rescue invariant — `*RescueError{Kind:ErrRescue}`, `TreeSHA` == the frozen
snapshot, `ParentSHA` == pre-run HEAD, atomic-HEAD (§18.1), idempotent-index (§20.2) — PLUS a stub-counter
call-count that proves whether the multi-turn phase was entered (the assertion T3.S3's weak skip tests lack).

**Deliverable**: ONE new file `internal/generate/generate_multiturn_failure_test.go` (`package generate`,
white-box) containing:
- a shared helper `assertMultiTurnRescue(t *testing.T, repo string, m provider.Manifest, cfg config.Config, wantCalls int) *RescueError` — snapshots HEAD + staged index (names + full) + the frozen tree (`git write-tree`), runs `CommitStaged`, and asserts the full FR-T7/idempotent invariant + `stub counter == wantCalls`; returns the `*RescueError` for scenario-specific extra assertions.
- `TestCommitStaged_MultiTurnMidTurnFailureRescue` (scenario a).
- `TestCommitStaged_MultiTurnSmallPayloadSkip_RescueInvariant` (scenario b).
- `TestCommitStaged_MultiTurnNonAppendSkip_RescueInvariant` (scenario c).

It REUSES `generate_test.go`'s package-level helpers (`initRepo`, `writeFile`, `stageFile`, `headSHA`,
`commitRaw`, `gitOut`, `appendScriptManifest`) — no duplication, no redeclaration.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/generate/` clean;
`go test -race ./internal/generate/ -run 'TestCommitStaged_MultiTurn(MidTurnFailureRescue|SmallPayloadSkip_RescueInvariant|NonAppendSkip_RescueInvariant)' -v` PASSES; the full suite `go test -race ./internal/generate/...` stays green (no regression to T3.S3's tests, T4.S1's render-contract test, or any other). `git diff --stat` shows ONLY the one new file.

## User Persona

**Target User**: The Stagecoach contributor/maintainer (PRD §20 QA). Transitively US9.24 / G21 (multi-turn
fallback) — this test is the regression net proving a future refactor of the FR-T1 gate or `multiturn.Run`'s
failure handling cannot (a) leave the repo mutated after a mid-turn failure, nor (b) accidentally invoke
multi-turn when a gate condition is false, nor (c) surface a NEW exit code/message on a multi-turn failure.

**Use Case**: CI runs these tests on every change to `internal/generate/generate.go` (the gate),
`internal/generate/multiturn.go` (`Run`'s per-turn error abort), or `internal/generate/rescue.go`. A
regression (e.g. the gate forgetting to check cond (b), or `Run` not aborting on a turn error, or the
rescue mutating the index) turns one of these tests red.

**User Journey**: (internal test) build stub → init temp repo + seed commit → stage a file → configure a
stub manifest (append/non-append, chunkTokens, EXIT) → run `CommitStaged` → assert the rescue invariant +
the counter-proves-(no-)multi-turn call count.

**Pain Points Addressed**: Closes the coverage gap left by T3.S3 (which proves the gate fires/skips and
the rescue Kind, but never inspects TreeSHA/the index/the call count). Without these tests: a mid-turn
failure that mutated the index, or a gate that invoked multi-turn on a small payload, would ship silently.

## Why

- **FR-T7 is the safety promise — it must be tested at the invariant level, not just the Kind level.**
  "Multi-turn can never leave the run in a worse state than one-shot-exhausted" is only truly proven by
  asserting HEAD + index are byte-unchanged AND the rescue carries the frozen TreeSHA/ParentSHA (so the
  manual recovery command in FormatRescue is correct). T3.S3 asserts only the Kind.
- **The mid-turn Execute-error path is uncovered.** T3.S3's `..._DuplicateRescue` exercises a FINAL-turn
  dedupe failure (Run completes N+1 turns, the parsed message duplicates); NOTHING exercises a turn's
  `Execute` returning an error mid-run (Run aborting at turn 1..N). That abort path (multiturn.go
  `return "",false,execErr`) is FR-T7's core — only scenario (a) reaches it.
- **The counter proves the gate's negative conditions bite.** "Multi-turn skipped on small payload /
  non-append provider" is only meaningful if you can prove Run was NEVER called. The stub counter (== 1)
  is that proof; T3.S3's skip tests can't make it.
- **Non-overlapping with T3.S3/S4 and T4.S1.** T3.S3 owns gate-wiring + Kind-only skip/duplicate tests;
  T3.S4 owns chunk-math unit tests; T4.S1 owns the happy-path render contract. T4.S2 owns the
  FAILURE/INVARIANT depth — a distinct, narrow, high-value assertion set.
- **Cheap, fast, deterministic.** Reuses the compiled-once stub, a real temp git repo, and the existing
  fixture helpers. No real provider, no network (PRD §20.1 layer 3). No stub modification.
- **No user-facing surface change** (PRD "DOCS: none — test-only").

## What

Three white-box integration tests + one shared helper. Each test sets up a stub manifest + config that
targets exactly one FR-T1 condition/failure mode, then calls `assertMultiTurnRescue` (which asserts the
full invariant + counter). Scenario (a) additionally asserts `RescueError.Cause != nil` (the multi-turn
turn error propagated). No production code changes. No stub changes. No new dependencies.

### Success Criteria

- [ ] New file `internal/generate/generate_multiturn_failure_test.go`, `package generate`, white-box.
- [ ] ONE shared helper `assertMultiTurnRescue` (full invariant: Kind/TreeSHA/ParentSHA/atomic-HEAD/
      idempotent-index-names/idempotent-index-full/counter); reuses generate_test.go helpers (no redeclaration).
- [ ] Exactly THREE tests (a)/(b)/(c); no second happy-path test; no re-test of T3.S3's Kind-only asserts.
- [ ] (a) `m.Env["STAGECOACH_STUB_EXIT"]="1"` (global) + append + chunkTokens=4 + large file ⇒ counter==2,
      `re.Kind==ErrRescue`, `re.TreeSHA==frozen`, `re.ParentSHA==beforeHEAD`, `re.Cause != nil`, index unchanged.
- [ ] (b) append + tiny file + default chunkTokens(32000) ⇒ counter==1 (multi-turn NOT entered), full invariant.
- [ ] (c) non-append (raw `stubtest.NewScript`) + large file + chunkTokens=4 ⇒ counter==1, full invariant.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/generate/` clean; full generate suite green.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the helper skeleton + assertion
table below, the three test skeletons, and the cited code sites (generate.go gate ~290-330, multiturn.go
Run turn-1 abort, executor.go:77, generate_test.go:491 idempotent template, generate_test.go:857
appendScriptManifest). The global-EXIT mechanism and the counter discriminator are fully explained.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- file: internal/generate/generate_test.go
  why: (1) REUSE these package-level helpers — do NOT redeclare: initRepo, writeFile, stageFile, headSHA,
       commitRaw, gitOut, runGit (lines 22-66), appendScriptManifest (line 857). (2) Lines 868-941 are the
       T3.S3 multi-turn tests — READ them to AVOID duplicating their Kind-only assertions. (3) Line 491
       (TestCommitStaged_IdempotentIndexOnFailure) is the canonical idempotent-index template — mirror its
       beforeHEAD/beforeIndex/beforeIndexFull + after-compare pattern. (4) Line 210 (TestCommitStaged_ParseFailRescue)
       is the rescue template.
  pattern: stubtest.Build → initRepo → commitRaw → writeFile → stageFile → appendScriptManifest/NewScript →
           CommitStaged → assert *RescueError + git index/HEAD invariants.
  gotcha: "same package = same test binary" — helpers are visible to generate_multiturn_failure_test.go
          WITHOUT import; redeclaring any ⇒ compile error. appendScriptManifest sets SessionMode=&"append".

- file: internal/generate/generate.go
  section: "CommitStaged" — the FR-T1 trigger gate (search "FR-T1 multi-turn fallback trigger gate", ~line 290)
  why: confirms (1) the gate fires only when MultiTurnFallback && EstimateTokens(payload)>MultiTurnChunkTokens
       && resolved.SessionMode!="append" (after one-shot !success); (2) Run's cause!=nil OR ok2==false ⇒
       lastCause=cause / candidate=msg2 ⇒ falls through to the byte-identical rescue at the `if !success`
       return (`Kind:ErrRescue`, ~line 327); (3) the non-zero-exit one-shot branch (~line 258) sets lastCause
       and falls through to ParseOutput (does NOT early-return) ⇒ a global EXIT=1 one-shot still EXHAUSTS
       (parse-fail) and reaches the gate.
  critical: this is why global STAGECOACH_STUB_EXIT=1 works — the one-shot exit-1 is swallowed as a parse-fail,
            and the multi-turn turn-1 exit-1 becomes the rescue Cause.

- file: internal/generate/multiturn.go
  section: "Run" — the turn-1 Execute abort (search "FR-T7: any turn error/timeout/cancel/non-zero-exit aborts")
  why: confirms Run returns ("", false, execErr) on the FIRST turn's Execute error — the abort branch
       scenario (a) exercises. Intermediate turns' stdout is discarded; Run does NOT parse partial output.
       A non-append manifest would fail at RenderMultiTurn (session_mode gate) — but scenario (c) never
       reaches Run (the CommitStaged gate's cond d is false first).
  pattern: `if _, _, execErr := provider.Execute(...); execErr != nil { return "", false, execErr }`.

- file: internal/provider/executor.go
  section: "Execute" (line 77 — the non-zero-exit return)
  why: PROVES a non-zero stub exit surfaces as a non-nil wrapped *exec.ExitError: `return out.String(),
       errb.String(), fmt.Errorf("provider %q: %w", spec.Command, werr)`. So scenario (a)'s
       `re.Cause != nil` assertion holds (the gate sets lastCause=cause=that wrapped error).
  critical: Execute captures+returns stdout EVEN on error (partial output for the rescue path) — but
            scenario (a)'s one-shot stdout is "" (script[0]) regardless.

- file: cmd/stubagent/main.go
  section: "main" (the exit knob) + "selectScripted" (the counter)
  why: confirms (1) exit is `os.Exit(envInt("STAGECOACH_STUB_EXIT", 0))` — a SINGLE global value applied to
       EVERY call (no per-call exit; the script varies OUTPUT only); (2) selectScripted reads counter N,
       writes N+1, returns lines[N] (clamp-to-last) — so the counter file is the authoritative call count.
  critical: stdout is written BEFORE os.Exit, so exit=1 still yields the scripted line ("" for call 0).

- file: internal/stubtest/stubtest.go
  why: confirms (1) NewScript writes a script file + counter file in t.TempDir() and sets
       m.Env["STAGECOACH_STUB_COUNTER"]=counter; (2) Manifest's Env is `optsEnvMap(o)` — a MUTABLE map, so
       `m.Env["STAGECOACH_STUB_EXIT"]="1"` after appendScriptManifest is persistent and reaches the stub
       via CmdSpec.Env→cmd.Env; (3) NewScript does NOT set SessionMode (nil) ⇒ cond (d) false.
  gotcha: appendScriptManifest(t,bin,responses) = NewScript + m.SessionMode=&"append". For scenario (c)
          use RAW stubtest.NewScript (no SessionMode).

- file: plan/009_5c53066d64b3/architecture/research-generate-config.md
  section: "5. RescueError type — location & multi-turn failure contract (FR-T7)"
  why: confirms the multi-turn failure returns `&RescueError{Kind:ErrRescue,...}` BYTE-IDENTICALLY to the
       one-shot-exhaustion path (generate.go ~327); ErrTimeout is reserved for the one-shot kill. This is
       the spec for the Kind assertion and the "byte-identical rescue" claim.
  pattern: assert re.Kind==ErrRescue (NOT ErrTimeout); re.TreeSHA non-empty; re.Cause = the turn error.

- file: plan/009_5c53066d64b3/architecture/research-tests-ui.md
  section: "2. CommitStaged end-to-end integration template" (rescue/idempotent-index + counter)
  why: the rescue + idempotent-index template (TestCommitStaged_ParseFailRescue +
       TestCommitStaged_IdempotentIndexOnFailure) and the rendered-argv/counter knobs. Confirms the
       counter + Env-injection seam used here.

- prd: PRD.md §9.24 (FR-T1 gate conditions a-d, FR-T7 failure handling) + §18.1 (atomic-HEAD invariant)
      + §20.2 (idempotent-index property test)
  why: FR-T1 defines the four gate conditions (a exhausted, b payload>chunk, c fallback on, d append);
       FR-T7 defines "any turn error/timeout/final-parse-fail ⇒ existing rescue, pure upside"; §18.1/§20.2
       define the HEAD/index invariants the helper asserts.
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  generate.go                 # CommitStaged + the FR-T1 gate (calls multiturn.Run) — calls rescue at ~line 327
  multiturn.go                # Run (N+1 protocol) — per-turn Execute; returns cause on ANY turn error (FR-T7)
  generate_test.go            # helpers + appendScriptManifest + T3.S3 multi-turn tests (lines 868-941) + idempotent template (line 491)
  multiturn_test.go           # T3.S4 unit tests (chunk math, truth table)
  rescue.go                   # FormatRescue (the message assembler; RescueError type lives in generate.go)
internal/provider/executor.go # Execute → wrapped *exec.ExitError on non-zero exit (line 77)
internal/stubtest/stubtest.go # Build, NewScript (counter), Manifest (mutable Env), optsEnvMap
cmd/stubagent/main.go         # the fake agent: STAGECOACH_STUB_EXIT (global), STAGECOACH_STUB_COUNTER (per-call)
```

### Desired Codebase tree with files to be added

```bash
internal/generate/
  generate_multiturn_failure_test.go  # NEW — assertMultiTurnRescue helper + 3 tests (a)/(b)/(c)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: do NOT redeclare generate_test.go helpers (initRepo, writeFile, stageFile, headSHA, commitRaw,
// gitOut, runGit, appendScriptManifest). Same package ⇒ visible. Redeclaring ⇒ "redeclared in this package".

// CRITICAL: do NOT duplicate T3.S3's tests. generate_test.go:868-941 already has the happy path + two
// Kind-only skip tests + the final-turn-dedupe-rescue test. T4.S2 adds (a) the mid-turn Execute-error path
// and (b)/(c) the idempotent-index + counter strengthenings. Do NOT re-assert only Kind==ErrRescue.

// CRITICAL: the stub has ONE global exit knob (STAGECOACH_STUB_EXIT); the script varies OUTPUT only. To
// trigger a mid-turn Execute error WITHOUT modifying the stub (forbidden by T4.S1 + "existing harness"),
// set m.Env["STAGECOACH_STUB_EXIT"]="1" GLOBALLY. The one-shot exit-1 is swallowed (parse-fail ⇒ exhaust ⇒
// gate fires); the first multi-turn turn exit-1 ⇒ Run aborts ⇒ rescue. The failure lands on turn 1 (Run
// aborts at the FIRST error) — equivalent FR-T7 coverage to a later turn.

// CRITICAL: MaxDuplicateRetries=0 ⇒ the one-shot loop runs EXACTLY 1 attempt ⇒ the counter math is clean:
//   scenario (a): 1 one-shot + 1 turn-1 = 2  (gate FIRED, Run aborted at turn 1)
//   scenarios (b)/(c): 1 one-shot only    = 1  (gate SKIPPED Run)
// Any other MaxDuplicateRetries muddies the counter assertion.

// CRITICAL: use ErrRescue (exit 3), NOT ErrTimeout (exit 124). A multi-turn turn error maps to
// &RescueError{Kind:ErrRescue} byte-identically to one-shot-exhaustion. ErrTimeout is reserved for the
// one-shot DeadlineExceeded kill. Asserting re.Kind==ErrRescue IS the FR-T7 "never worse" proof.

// CRITICAL: chunkTokens discriminates cond (b). scenario (b) uses the DEFAULT 32000 (tiny diff ⇒ cond b
// FALSE ⇒ skip); scenarios (a)/(c) use 4 (normal file ⇒ EstimateTokens(payload) > 4 ⇒ cond b TRUE). Mirror
// T3.S3's chunkTokens choices so the gate behaves as intended.

// CRITICAL: scenario (c) must use RAW stubtest.NewScript (SessionMode nil ⇒ cond d false), NOT
// appendScriptManifest (which sets SessionMode="append"). And it needs a LARGE file + chunkTokens=4 so
// cond (b) is TRUE — isolating cond (d) as the ONLY failing condition.

// MINOR: `git write-tree` is safe to call before the run (writes the index as a tree object, does NOT
// mutate index/refs) — its output IS the frozen TreeSHA CommitStaged will compute (same index ⇒ same tree).
// Use it to assert re.TreeSHA == frozen without inspecting CommitStaged internals.

// MINOR: name the file generate_multiturn_failure_test.go (NOT generate_multiturn_test.go — that is T4.S1's
// file, being added in parallel). Both compile in package generate; keep helper names distinct from T4.S1's
// sessionIDRe (use assertMultiTurnRescue — no collision).
```

## Implementation Blueprint

### Data models and structure

No production data models. The tests build:
- A **stub manifest** = `appendScriptManifest(t, bin, script)` (scenarios a/b) for SessionMode="append",
  OR raw `stubtest.NewScript(t, bin, []string{""})` (scenario c) for SessionMode unset.
- For scenario (a) ONLY: `m.Env["STAGECOACH_STUB_EXIT"] = "1"` (global non-zero exit → mid-turn failure).
- A **config** = `config.Defaults()` with `MaxDuplicateRetries=0`; `MultiTurnChunkTokens` per scenario
  (4 for a/c; default 32000 for b); `MultiTurnFallback=true` (the default; explicit optional).
- A **staged file** sized to control cond (b): normal ("hello world\n", scenarios a/c) or tiny ("hi\n", b).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/generate/generate_multiturn_failure_test.go (package generate, white-box)
  - IMPORTS: context, errors, os, strconv, strings, testing +
             internal/config, internal/git, internal/provider, internal/stubtest.
    (NO bytes/regexp/fmt needed — the helper uses t.Errorf + strconv.Itoa + os.ReadFile only.)
  - DO NOT redeclare generate_test.go helpers — reuse initRepo/writeFile/stageFile/headSHA/commitRaw/
    gitOut/appendScriptManifest directly (same package).
  - HELPER: assertMultiTurnRescue(t, repo, m, cfg, wantCalls) *RescueError — see the Pattern block.
  - TESTS: TestCommitStaged_MultiTurnMidTurnFailureRescue (a),
           TestCommitStaged_MultiTurnSmallPayloadSkip_RescueInvariant (b),
           TestCommitStaged_MultiTurnNonAppendSkip_RescueInvariant (c).
  - NAMING/PLACEMENT: file generate_multiturn_failure_test.go; helper + 3 exported Test funcs.

Task 2: VERIFY (no file change)
  - RUN the Validation Loop (Level 1 + Level 2). Fix until green. `git diff --stat` ⇒ only the new file.
```

### Implementation Patterns & Key Details

```go
// generate_multiturn_failure_test.go — the multi-turn FAILURE/INVARIANT integration tests (PRD §9.24
// FR-T7 + FR-T1 negative conditions b/d).
//
// UNIQUE vs T3.S3 (generate_test.go:868-941): T3.S3's skip tests assert ONLY Kind==ErrRescue; the duplicate
// test exercises a FINAL-turn dedupe failure. THESE tests add: (a) the mid-turn Execute-error abort path
// (entirely new), and (b)/(c) the full rescue invariant (TreeSHA==frozen, ParentSHA, atomic-HEAD,
// idempotent-index) + the stub-counter proof that Run was/wasn't entered.

// assertMultiTurnRescue runs CommitStaged expecting a *RescueError{Kind:ErrRescue} and asserts the FULL
// FR-T7 / idempotent invariant: TreeSHA == the frozen snapshot (git write-tree before the run), ParentSHA
// == pre-run HEAD, HEAD unchanged (§18.1), staged index unchanged both name-set and full diff (§20.2), and
// the stub counter == wantCalls (1 ⇒ one-shot only / gate skipped Run; 2 ⇒ one-shot + 1 multi-turn turn /
// gate fired, Run aborted at turn 1). Returns the RescueError for scenario-specific extra assertions.
// Template: research-tests-ui.md §2 (TestCommitStaged_ParseFailRescue + TestCommitStaged_IdempotentIndexOnFailure).
func assertMultiTurnRescue(t *testing.T, repo string, m provider.Manifest, cfg config.Config, wantCalls int) *RescueError {
	t.Helper()
	beforeHEAD := headSHA(t, repo)
	beforeIndex := gitOut(t, repo, "diff", "--cached", "--name-only")
	beforeIndexFull := gitOut(t, repo, "diff", "--cached")
	frozen := strings.TrimSpace(gitOut(t, repo, "write-tree")) // safe: writes a tree object, no index/refs mutation

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	var re *RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want *RescueError (FR-T7 rescue)", err)
	}
	if re.Kind != ErrRescue {
		t.Fatalf("Kind = %v, want ErrRescue (FR-T7: byte-identical to one-shot-exhaustion ⇒ exit 3; NOT ErrTimeout/124)", re.Kind)
	}
	if re.TreeSHA != frozen {
		t.Errorf("TreeSHA = %q, want frozen snapshot %q (WriteTree taken before generation)", re.TreeSHA, frozen)
	}
	if re.ParentSHA != beforeHEAD {
		t.Errorf("ParentSHA = %q, want %q (pre-run HEAD = the rescue parent)", re.ParentSHA, beforeHEAD)
	}
	if got := headSHA(t, repo); got != beforeHEAD {
		t.Errorf("HEAD moved %q → %q (atomic-HEAD §18.1 — rescue must not move refs)", beforeHEAD, got)
	}
	if got := gitOut(t, repo, "diff", "--cached", "--name-only"); got != beforeIndex {
		t.Errorf("staged file set changed (idempotent-index §20.2):\nbefore: %q\nafter:  %q", beforeIndex, got)
	}
	if got := gitOut(t, repo, "diff", "--cached"); got != beforeIndexFull {
		t.Errorf("staged index content changed (idempotent-index §20.2 full diff)")
	}
	cf := m.Env["STAGECOACH_STUB_COUNTER"]
	if cf == "" {
		t.Fatalf("manifest Env lacks STAGECOACH_STUB_COUNTER (use stubtest.NewScript/appendScriptManifest)")
	}
	raw, rerr := os.ReadFile(cf)
	if rerr != nil {
		t.Fatalf("read stub counter: %v", rerr)
	}
	if got := strings.TrimSpace(string(raw)); got != strconv.Itoa(wantCalls) {
		t.Errorf("stub invocations = %s, want %d (1 = one-shot only / multi-turn skipped; 2 = one-shot + 1 turn / Run aborted)", got, wantCalls)
	}
	return re
}

// (a) MID-TURN FAILURE → RESCUE (FR-T7). Global STAGECOACH_STUB_EXIT=1 ⇒ the one-shot exits 1 but its
// stdout is "" (script[0]) ⇒ parse-fail ⇒ exhaust ⇒ gate fires (conds a/b/c/d all hold) ⇒ Run's turn 1
// exits 1 ⇒ Execute returns a wrapped *exec.ExitError ⇒ Run aborts ⇒ rescue. Counter == 2.
func TestCommitStaged_MultiTurnMidTurnFailureRescue(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "hello world\n") // payload ≫ chunkTokens(4) ⇒ cond (b) true; N≥2
	stageFile(t, repo, "new.txt")

	// Script[0]="" ⇒ one-shot parse-fail ⇒ exhaust ⇒ gate. Global EXIT=1 ⇒ EVERY call exits non-zero.
	// The one-shot's exit-1 is swallowed (non-zero-exit branch falls through to ParseOutput("") ⇒ ok=false);
	// Run's turn-1 exit-1 ⇒ Run aborts (FR-T7) ⇒ rescue. (No per-call exit knob without a stub change;
	// turn-1 failure == later-turn failure coverage — Run aborts at the first error.)
	m := appendScriptManifest(t, bin, []string{"", "ok", "ok", "feat: unreachable"})
	m.Env["STAGECOACH_STUB_EXIT"] = "1" // mutable Env map (optsEnvMap); applies to every stub call

	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0   // exactly 1 one-shot call ⇒ counter math clean
	cfg.MultiTurnChunkTokens = 4  // tiny ⇒ cond (b) true; N≥2
	cfg.MultiTurnFallback = true  // cond (c) (default true; explicit)

	re := assertMultiTurnRescue(t, repo, m, cfg, 2) // 1 one-shot + 1 multi-turn turn (aborted at turn 1)
	// FR-T7: the multi-turn turn error supersedes one-shot's lastCause and propagates as RescueError.Cause.
	if re.Cause == nil {
		t.Errorf("Cause = nil, want the wrapped *exec.ExitError from the failed multi-turn turn (executor.go:77)")
	}
}

// (b) SMALL-PAYLOAD SKIP (FR-T1b negative). SessionMode="append" but a TINY diff + default chunkTokens
// ⇒ EstimateTokens(payload) ≤ 32000 ⇒ cond (b) FALSE ⇒ gate skips Run ⇒ existing rescue. Counter == 1.
// DISTINCT from T3.S3's TestCommitStaged_MultiTurnSkipped_SmallPayload (Kind-only): adds idempotent-index
// + the counter proof that multi-turn was NEVER entered.
func TestCommitStaged_MultiTurnSmallPayloadSkip_RescueInvariant(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "tiny.txt", "hi\n") // tiny diff ⇒ EstimateTokens(payload) ≤ 32000 ⇒ cond (b) false
	stageFile(t, repo, "tiny.txt")

	m := appendScriptManifest(t, bin, []string{""}) // SessionMode="append" (cond d true); script[0]="" ⇒ exhaust
	cfg := config.Defaults()                        // MultiTurnChunkTokens=32000 (default) ⇒ cond (b) false
	cfg.MaxDuplicateRetries = 0

	assertMultiTurnRescue(t, repo, m, cfg, 1) // one-shot ONLY; Run never entered
}

// (c) NON-APPEND PROVIDER SKIP (FR-T1d negative). SessionMode UNSET (claude-shaped) + large payload ⇒
// cond (d) FALSE ⇒ gate skips Run silently ⇒ existing rescue. Counter == 1. The large file + chunkTokens=4
// keep cond (b) TRUE so cond (d) is the ONLY failing condition (isolates the non-append skip).
func TestCommitStaged_MultiTurnNonAppendSkip_RescueInvariant(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "hello world\n") // payload ≫ chunkTokens(4) ⇒ cond (b) true
	stageFile(t, repo, "new.txt")

	m := stubtest.NewScript(t, bin, []string{""}) // RAW NewScript ⇒ SessionMode nil ⇒ cond (d) false
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0
	cfg.MultiTurnChunkTokens = 4 // cond (b) true ⇒ ONLY cond (d) fails

	assertMultiTurnRescue(t, repo, m, cfg, 1) // one-shot ONLY; Run never entered
}
```

> If `TestCommitStaged_MultiTurnMidTurnFailureRescue` fails with counter==1 (not 2), the gate did NOT fire
> — check that `MultiTurnChunkTokens=4` + a non-tiny file actually make `EstimateTokens(payload) > 4`
> (cond b), and that SessionMode="append" (cond d). If it fails with Kind==ErrTimeout, the one-shot's
> exit-1 was misrouted into the DeadlineExceeded branch (it must hit the non-zero-exit fall-through).

### Integration Points

```yaml
TEST WIRING (the ONLY integration):
  - scenario (a): appendScriptManifest(t,bin,["","ok","ok","feat: unreachable"]) + m.Env["STAGECOACH_STUB_EXIT"]="1";
                  cfg.MaxDuplicateRetries=0, MultiTurnChunkTokens=4, MultiTurnFallback=true; wantCalls=2.
  - scenario (b): appendScriptManifest(t,bin,[""]); cfg=Defaults() (chunkTokens 32000), MaxDuplicateRetries=0; wantCalls=1.
  - scenario (c): stubtest.NewScript(t,bin,[""]) (NO SessionMode); cfg.MaxDuplicateRetries=0, MultiTurnChunkTokens=4; wantCalls=1.
  - drive: CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg) via assertMultiTurnRescue.

NO PRODUCTION CODE / NO STUB CHANGE / NO go.mod / NO DOCS (test-only, PRD "DOCS: none — test-only").
NO overlap with T3.S3 (generate_test.go:868-941 Kind-only tests), T3.S4 (multiturn_test.go unit tests),
or T4.S1 (generate_multiturn_test.go render-contract test).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                              # Compiles incl. the new test file. Expect exit 0.
go vet ./internal/generate/                 # (and `go vet ./...`) Expect zero diagnostics.
gofmt -w internal/generate/generate_multiturn_failure_test.go
test -z "$(gofmt -l internal/generate/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
# Expected: all clean. If go vet complains about an unused import (e.g. strconv if you dropped Itoa), trim it.
```

### Level 2: Unit/Integration Tests (Component Validation)

```bash
# The three new tests (white-box, real git + stub agent):
go test -race ./internal/generate/ -run 'TestCommitStaged_MultiTurn(MidTurnFailureRescue|SmallPayloadSkip_RescueInvariant|NonAppendSkip_RescueInvariant)' -v
# Expected: PASS — all three assert the full rescue invariant + the counter call-count.

# Full generate suite must stay green (T3.S3 multi-turn tests, T4.S1 render-contract test, all prior):
go test -race ./internal/generate/...
# Expected: all PASS — no regression.

# Whole module (defensive — no cross-package fallout):
go test -race ./...
# Expected: all PASS.
```

### Level 3: Integration Testing (System Validation)

```bash
# Confirm each test EXERCISES the intended path (not a silent misconfiguration):
#  - (a) must show counter==2 in its assertion (gate FIRED + Run aborted). If it shows 1, cond (b)/(d) failed.
#  - (b)/(c) must show counter==1 (gate SKIPPED Run). If 2, the gate wrongly fired.
go test ./internal/generate/ -run 'TestCommitStaged_MultiTurnMidTurnFailureRescue|TestCommitStaged_MultiTurnSmallPayloadSkip_RescueInvariant|TestCommitStaged_MultiTurnNonAppendSkip_RescueInvariant' -v -count=1
# Expected: PASS; each internally verified TreeSHA==frozen, idempotent-index, and the counter value.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# (Optional) Lint the new file if golangci-lint is installed:
golangci-lint run ./internal/generate/ 2>/dev/null || echo "golangci-lint not installed (Makefile lint is project-wide; run \`make lint\` in CI)."
# Mutation checks (manual, confirm the tests BITE):
#  (1) Edit the FR-T1 gate to DROP the cond (b) check ⇒ scenario (b)'s counter becomes 2 (Run invoked on a
#      small payload) ⇒ TestCommitStaged_MultiTurnSmallPayloadSkip_RescueInvariant goes red. Revert.
#  (2) Edit multiturn.Run to NOT abort on a turn error (continue instead) ⇒ scenario (a)'s counter grows
#      beyond 2 AND/or the rescue path changes ⇒ TestCommitStaged_MultiTurnMidTurnFailureRescue goes red. Revert.
#  (3) Edit the gate's rescue fall-through to return ErrTimeout ⇒ scenario (a) Kind assertion goes red. Revert.
# Expected: each mutation turns the relevant test red, proving the tests guard the contract.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/generate/`.
- [ ] Level 2 green: the three new tests AND `go test -race ./internal/generate/...`.
- [ ] `git diff --stat` shows ONLY `internal/generate/generate_multiturn_failure_test.go` (no production/stub/go.mod/docs changes).

### Feature Validation

- [ ] (a) `m.Env["STAGECOACH_STUB_EXIT"]="1"` + append + chunkTokens=4 ⇒ `*RescueError{Kind:ErrRescue}`,
      TreeSHA==frozen, ParentSHA==beforeHEAD, Cause != nil, idempotent-index, counter==2.
- [ ] (b) append + tiny file + default chunkTokens ⇒ full invariant + counter==1 (Run NOT entered).
- [ ] (c) raw NewScript (SessionMode unset) + large file + chunkTokens=4 ⇒ full invariant + counter==1.
- [ ] All three assert Kind==ErrRescue (NOT ErrTimeout) — the FR-T7 "never worse than one-shot-exhausted" proof.

### Code Quality Validation

- [ ] Reuses generate_test.go helpers (no redeclaration); `package generate` white-box.
- [ ] Does NOT duplicate T3.S3's Kind-only skip tests or T4.S1's render-contract test.
- [ ] File name `generate_multiturn_failure_test.go` (distinct from T4.S1's `generate_multiturn_test.go`).
- [ ] No stub/production change; no new deps; no docs.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Helper + test doc-comments explain the UNIQUE value (failure/invARIANT depth) vs T3.S3 and the
      global-EXIT + counter mechanism.
- [ ] No env vars / no user-facing docs (test-only).

---

## Anti-Patterns to Avoid

- ❌ Don't duplicate T3.S3's `TestCommitStaged_MultiTurnSkipped_NonAppend` / `..._SmallPayload` /
  `..._DuplicateRescue` / `..._FallbackSuccess` (generate_test.go:868-941). T4.S2 adds the INVARIANT depth
  (TreeSHA/ParentSHA/idempotent-index/counter) and the mid-turn-failure path — distinct assertions.
- ❌ Don't redeclare `initRepo`/`headSHA`/`gitOut`/`appendScriptManifest` — same package, reuse them.
- ❌ Don't modify `cmd/stubagent` to add a per-call exit knob — T4.S1 forbids stub changes and the "existing
  harness" constraint discourages it. Use GLOBAL `m.Env["STAGECOACH_STUB_EXIT"]="1"`; the one-shot exit-1 is
  swallowed (parse-fail ⇒ exhaust ⇒ gate fires) and the first multi-turn turn exit-1 aborts Run.
- ❌ Don't assert `Kind==ErrTimeout` for a multi-turn failure — use `ErrRescue` (exit 3, byte-identical to
  one-shot-exhaustion). `ErrTimeout` is reserved for the one-shot DeadlineExceeded kill.
- ❌ Don't set `MaxDuplicateRetries!=0` — it muddies the counter math (one-shot would make >1 call).
  `MaxDuplicateRetries=0` ⇒ exactly 1 one-shot call ⇒ counter == 1 (skip) or 2 (gate fired + turn-1 abort).
- ❌ Don't use `appendScriptManifest` for scenario (c) — it sets SessionMode="append" (cond d TRUE), defeating
  the non-append skip. Use RAW `stubtest.NewScript` (SessionMode nil ⇒ cond d false).
- ❌ Don't use a tiny file for scenario (a) or (c) — cond (b) would be false and the gate would skip (a) or
  conflate conditions (c). Use a normal file ("hello world\n") + chunkTokens=4 so cond (b) is TRUE.
- ❌ Don't name the file `generate_multiturn_test.go` — that is T4.S1's file (parallel). Use
  `generate_multiturn_failure_test.go`.
- ❌ Don't add a `//go:build integration_real` tag — this is a CI stub test (PRD §20.1 layer 3), not the
  opt-in real-agent suite.
- ❌ Don't re-derive TreeSHA by inspecting CommitStaged internals — call `git write-tree` before the run
  (same index ⇒ same tree) and assert `re.TreeSHA ==` that. Safe (writes a tree object, no index/refs mutation).
