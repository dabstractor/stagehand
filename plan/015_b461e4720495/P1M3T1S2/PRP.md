name: "P1.M3.T1.S2 — Wire message-role timeout (ResolveRoleTimeout) at the multi-turn, work-description, and hook Execute sites (FR-R7, FR-T5)"
description: >
  The SECOND consumer-wiring subtask of P1.M3: make the multi-turn transport, the work-description
  transport, and the hook runtime use the MESSAGE role's resolved timeout
  (`config.ResolveRoleTimeout("message", cfg)`) instead of the flat `cfg.Timeout` at every message-role
  Execute site. S1 (P1.M3.T1.S1) ALREADY did this for the canonical path — `internal/generate/generate.go`
  CommitStaged — and is ALREADY LANDED in the working tree (generate.go:269 resolves msgTimeout; :340
  Execute + :431 budget-display use it; generate_test.go:495 TestCommitStaged_MessageRoleTimeout is
  present). S2 mirrors S1's exact pattern across the 3 REMAINING message-role files:
  (a) `internal/generate/multiturn.go` Run — resolve `msgTimeout` at the top of Run; swap the 3 per-turn
  Execute sites (:165 turn-1, :176 turns-2..N, :187 turn-N+1) `cfg.Timeout` → `msgTimeout`; update the
  :132 doc comment;
  (b) `internal/generate/workdesc.go` RunWorkDescription — resolve `msgTimeout` at the top of the
  function; swap the 3 Execute sites (:75 turn-1, :106 forced-conclusion FR-W6, :122 answer-turn — note
  :122 uses `=` not `:=`) `cfg.Timeout` → `msgTimeout`;
  (c) `internal/hook/exec.go` Run — resolve `msgTimeout` immediately after the EXISTING :162
  `config.ResolveRoleModel("message", cfg)` (co-locate the timeout twin, exactly like generate.go:264/269);
  swap the :182 one-shot Execute AND the :252 multi-turn budget-display `cfg.Timeout` → `msgTimeout`
  (BOTH are message-role FR-T5 sites — :252 is the IDENTICAL budget line S1 changed in generate.go:431;
  the contract text named only :182, an oversight — see Anti-Patterns). NO new imports for the 3 source
  files (config is already imported by all three; multiturn.go + hook/exec.go also import time; workdesc.go
  needs neither for the source change). Run's + RunWorkDescription's signatures are UNCHANGED (both already
  receive `cfg`; S2 resolves locally — same as S1 left the Run CALL at generate.go:436 passing `cfg`).
  THREE new behavioral-proof tests (one per file), each cloning an existing harness and flipping WHICH
  field carries the small timeout (large global cfg.Timeout + small message-role Timeout + stub SleepMS)
  to prove the MESSAGE-role timeout bounds Execute:
  - multiturn_test.go TestRun_MessageRoleTimeoutBoundsTurn (needs `"time"` import added);
  - generate_workdesc_test.go TestCommitStaged_WorkDescription_MessageRoleTimeout (via CommitStaged's
    workDescActive branch — cleanly isolatable; needs `"time"` import added);
  - hook/exec_test.go TestRun_MessageRoleTimeoutNeverBlock (clones TestRun_TimeoutNeverBlock; time already
    imported). The dependency — `func ResolveRoleTimeout(role string, cfg Config) time.Duration` — is
  LANDED (P1.M2.T1.S1): for "message" it returns cfg.Roles["message"].Timeout if non-zero ELSE cfg.Timeout
  (the message role has NO built-in — only planner=480s). This makes every change BEHAVIOR-PRESERVING BY
  DEFAULT: with no [role.message].timeout override, msgTimeout == cfg.Timeout, so all existing tests
  (TestRun_HappyPath, TestCommitStaged_WorkDescription_*, TestRun_TimeoutNeverBlock, …) stay GREEN
  unchanged. NOT in scope: generate.go (S1 DONE), config/roles.go (LANDED), planner/stager/arbiter
  (P1.M3.T2.S1 decompose path), docs (P1.M4.T2 — contract: "DOCS: none — internal wiring"), the global
  480s→120s flip (P1.M2.T2.S1, independent — ResolveRoleTimeout returns cfg.Timeout whatever its value).

---

## Goal

**Feature Goal**: Wire the message-role per-role timeout (FR-R7) into the three remaining message-role
transports — the multi-turn fallback (`multiturn.go` Run), the work-description read loop (`workdesc.go`
RunWorkDescription), and the hook runtime (`hook/exec.go` Run) — so that `[role.message].timeout` /
`--message-timeout` / `STAGECOACH_MESSAGE_TIMEOUT` / `stagecoach.role.message.timeout` bound the message
agent's per-turn generation at ALL message-role Execute sites (and the hook's multi-turn total-budget
progress line, FR-T5), instead of every site silently inheriting the flat `cfg.Timeout`. This completes
the message-role consumer wiring for P1.M3.T1 (S1 wired the canonical single-commit one-shot + budget in
generate.go; S2 wires the multi-turn, work-description, and hook paths).

**Deliverable**: A surgical edit set to 3 source files + 3 test files (1 new test per source file):
1. `internal/generate/multiturn.go` — +1 resolve (`msgTimeout`), 3 Execute arg swaps, 1 doc-comment update.
2. `internal/generate/workdesc.go` — +1 resolve (`msgTimeout`), 3 Execute arg swaps.
3. `internal/hook/exec.go` — +1 resolve (`msgTimeout`, co-located with :162 ResolveRoleModel), 1 Execute swap (:182), 1 budget-display swap (:252).
4. `internal/generate/multiturn_test.go` — +1 test (TestRun_MessageRoleTimeoutBoundsTurn) + `"time"` import.
5. `internal/generate/generate_workdesc_test.go` — +1 test (TestCommitStaged_WorkDescription_MessageRoleTimeout) + `"time"` import.
6. `internal/hook/exec_test.go` — +1 test (TestRun_MessageRoleTimeoutNeverBlock; no import — time already imported).

**Success Definition**:
- Every message-role Execute site in multiturn.go (×3), workdesc.go (×3), and hook/exec.go (:182) reads
  `msgTimeout` (`config.ResolveRoleTimeout("message", cfg)`), not `cfg.Timeout`. The hook's :252 multi-turn
  budget display also reads `msgTimeout` (FR-T5 — same as S1's generate.go:431).
- A `[role.message].timeout = "150ms"` (with `cfg.Timeout = 30s`) makes the multi-turn turn-1 Execute,
  the work-description turn-1 Execute, AND the hook one-shot Execute time out at 150ms (not 30s) — proven
  by the 3 new tests (stub SleepMS exceeds 150ms; Execute returns DeadlineExceeded).
- **Behavior-preserving by default**: with no message-role override, `msgTimeout == cfg.Timeout`, so the
  full `make test` suite (incl. TestRun_HappyPath, TestCommitStaged_WorkDescription_HappyPath,
  TestRun_TimeoutNeverBlock, the multi-turn/workdesc/hook coverage) stays GREEN unchanged.
- `go build ./...` + cross-build clean; `gofmt -l` empty; `make lint` + `make coverage-gate` green.
- Scope: `git diff --name-only` == the 6 files above. Run's signature (multiturn.go:145) + RunWorkDescription's
  signature (workdesc.go:63) UNCHANGED. generate.go / config/* UNCHANGED (grep-guarded).

## User Persona (if applicable)

**Target User**: A developer whose message-role generation is slow on a large diff (multi-turn fallback)
or who uses `--work-description` for description-first runs or the git hook — and who wants to bound JUST
the message agent's per-turn time without lowering the global timeout other roles may need. Also the
operator who finds the 120s global too tight for big-repo multi-turn runs and sets
`[role.message].timeout = "300s"` (which must now bound EVERY message-role turn, including the hook path
and the work-description loop, not just the one-shot).

**Use Case**: User sets `[role.message].timeout = "300s"`. A large-diff run falls back to multi-turn (5
turns); each turn is now bounded at 300s (and the hook's `~Mm total` progress line reflects 300s×(N+1)),
instead of the 120s global. Without this wiring, the per-turn Execute in Run/RunWorkDescription/hook.Run
read `cfg.Timeout` directly and IGNORED the 300s setting.

**User Journey**: `[role.message] timeout = "300s"` → Load() merges into cfg.Roles["message"].Timeout
(P1.M1 LANDED) → each transport resolves `msgTimeout := ResolveRoleTimeout("message", cfg)` = 300s →
per-turn Execute bounded at 300s → if a turn exceeds, the transport returns cause=DeadlineExceeded → the
caller's rescue/never-block path (FR-T7 / FR-H5).

**Pain Points Addressed**: FR-R7/FR-T5 — the message per-role timeout was RESOLVABLE (P1.M2.T1.S1) and
CONFIGURABLE (P1.M1) and consumed on the one-shot single-commit path (S1), but NOT YET on the multi-turn
fallback, the work-description loop, or the hook path. This task closes that gap for all three.

## Why

- **FR-R7 / §9.15 / §16.1**: "Each role resolves its own timeout independently." The accessor + all config
  layers are LANDED; S1 consumed them for CommitStaged's one-shot + budget. S2 extends the same consumption
  to the multi-turn/work-description/hook message-role Execute sites — the last message-role call sites.
- **FR-T5 / §9.24**: "Each turn is a separate provider invocation with its own timeout equal to the MESSAGE
  role's resolved timeout … Total wall-clock budget = message-timeout × (N+1)." multiturn.go's per-turn
  Execute (:165/176/187) and the hook's budget display (:252) are EXACTLY these sites; they must read the
  message-role timeout. workdesc.go's read-loop turns (:75/106/122) are message-role turns by the same logic
  (FR-W4 reuses §9.24's session machinery).
- **FR-H6 / §9.20**: "Hook mode resolves provider/model/reasoning exactly like the single-commit path (the
  MESSAGE role) and honors --timeout semantics via the same config keys." The hook's one-shot Execute (:182)
  is the message-role call site for hook mode; it must read the message-role timeout.
- **Behavior-preserving by default**: `ResolveRoleTimeout("message", cfg)` returns `cfg.Timeout` when no
  override (no message built-in). The wiring is invisible to every existing test and every default-config
  user; it ONLY activates under a per-role message timeout — precisely the new capability.
- **Bounded scope**: 3 source files (1 resolve + ≤3 arg swaps each + 1 doc comment) + 3 tests. No new types,
  no new imports for source, no signature changes, no docs (contract: "DOCS: none").

## What

**User-visible behavior**: With no message-role timeout configured, nothing changes (msgTimeout ==
cfg.Timeout). With `[role.message].timeout` / `--message-timeout` set, the message-role per-turn
generation is bounded by that value across the multi-turn, work-description, and hook paths, and the hook's
multi-turn progress line reports the correct total budget.

**Technical change**: 3 source files (resolve `msgTimeout` once per function + swap the `cfg.Timeout` args)
+ 3 tests (clone an existing harness, flip which field carries the small timeout). See the Implementation
Blueprint for verbatim before/after anchored by STRING.

### Success Criteria
- [ ] `multiturn.go` Run resolves `msgTimeout := config.ResolveRoleTimeout("message", cfg)` at its top;
      Execute sites :165/:176/:187 use `msgTimeout`; :132 doc comment updated.
- [ ] `workdesc.go` RunWorkDescription resolves `msgTimeout` at its top; Execute sites :75/:106/:122 use
      `msgTimeout` (preserving the `=` assignment at :122).
- [ ] `hook/exec.go` Run resolves `msgTimeout` immediately after the :162 ResolveRoleModel; Execute :182
      AND budget display :252 use `msgTimeout`.
- [ ] NEW `TestRun_MessageRoleTimeoutBoundsTurn` (multiturn_test.go): cfg.Timeout=30s, role=150ms,
      SleepMS=2000 → cause != nil && DeadlineExceeded.
- [ ] NEW `TestCommitStaged_WorkDescription_MessageRoleTimeout` (generate_workdesc_test.go): via
      CommitStaged workDescActive branch, cfg.Timeout=30s, role=150ms, SleepMS=2000 → `*RescueError{ErrTimeout}`.
- [ ] NEW `TestRun_MessageRoleTimeoutNeverBlock` (hook/exec_test.go): cfg.Timeout=30s, role=50ms,
      SleepMS=5000 → "hook generation timed out" + msg-file UNTOUCHED.
- [ ] EXISTING regression canaries GREEN unchanged (TestRun_HappyPath, TestRun_TurnError,
      TestCommitStaged_WorkDescription_HappyPath, TestRun_TimeoutNeverBlock, …).
- [ ] `go build ./...` + GOOS=windows + GOOS=linux clean; `gofmt -l` empty; `make lint` + `make coverage-gate` green.
- [ ] Scope: `git diff --name-only` == the 6 files; Run/RunWorkDescription signatures + generate.go + config/* UNCHANGED.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim before/after for every edit anchored by STRING (with verified line numbers), the
S1 in-tree template to mirror (generate.go:264/269/340/431, ALREADY LANDED — S2 is a mechanical replication),
the dependency signature (`ResolveRoleTimeout(role string, cfg Config) time.Duration`, LANDED) + the proof
it is behavior-preserving by default (no message built-in ⇒ msgTimeout==cfg.Timeout), the import situation
for each source file (config already imported by all three; no new source imports) and each test file
(multiturn_test.go + generate_workdesc_test.go need `"time"` added; exec_test.go already has it), the
workdesc :122 `=`-vs-`:=` gotcha, the hook :252 BOTH-sites decision (justified), the doc-comment update
(multiturn.go:132), the 3 test harnesses (with the exact existing tests to clone and the stub SleepMS +
Execute WithTimeout mechanism that makes the flip a valid proof), and the grep guards.

### Documentation & References

```yaml
# MUST READ — the authoritative codebase findings (exact edit sites, the S1 template, the test harnesses)
- docfile: plan/015_b461e4720495/P1M3T1S2/research/findings.md
  why: "§1 the EXACT edit sites per file (verified line numbers + unique strings); §2 the S1 in-tree template
        (generate.go:264/269/340/431 — the mechanical pattern to replicate); §3 ResolveRoleTimeout signature +
        the no-message-built-in proof; §4 the behavior-preserving-by-default proof + the regression canaries;
        §5 the 3 test harnesses (the stub SleepMS + Execute WithTimeout mechanism, which existing test to clone);
        §6 the hook :252 BOTH-sites decision; §7 the multiturn.go:132 doc-comment update; §8 scope fences."
  critical: "§1 line numbers are VERIFIED (multiturn 165/176/187 + 132 doc; workdesc 75/106/122 [note :122 `=`];
             hook 182 Execute + 252 budget + 162 anchor). §2 S1 is ALREADY in the tree — replicate, don't invent.
             §5 the workdesc test is cleanly isolatable via CommitStaged's workDescActive branch. §6 hook/exec.go
             has TWO cfg.Timeout sites — update BOTH (resolve a local), not just the contract's :182."

# MUST READ — the S1 PRP (the pattern this task replicates + the S1/S2 boundary contract)
- docfile: plan/015_b461e4720495/P1M3T1S1/PRP.md
  why: "S1 wired generate.go CommitStaged identically (resolve msgTimeout after ResolveRoleModel, swap Execute
        + budget sites). Its PRP defines the S1/S2 boundary: S1 = generate.go; S2 = multiturn.go/workdesc.go/
        hook-exec.go. S1 explicitly LEFT the Run call at generate.go:436 passing cfg (S2 wires Run's internals).
        The new-test pattern (flip which field carries the small timeout) is S1's, reused here per-file."
  critical: "S1 is the template — the msgTimeout resolution comment, the Execute swap, the budget swap, and the
             test shape ALL come from S1. Read it to mirror the exact comment wording + test assertion style."

# MUST READ — the dependency contract (ResolveRoleTimeout; LANDED by P1.M2.T1.S1 — consume, don't rebuild)
- docfile: plan/015_b461e4720495/P1M2T1S1/PRP.md
  why: "Defines `func ResolveRoleTimeout(role string, cfg Config) time.Duration` + defaultRoleTimeouts {planner:480s}.
        For 'message' returns cfg.Roles['message'].Timeout if non-zero ELSE cfg.Timeout (NO message built-in).
        This is THE function all 3 files call."
  critical: "Do NOT change ResolveRoleTimeout/defaultRoleTimeouts (LANDED). The no-message-built-in fact is WHY
             every change is behavior-preserving by default."

# MUST EDIT — the 3 source files (edit sites anchored by STRING)
- file: internal/generate/multiturn.go
  why: "Run (signature :145; receives cfg). Top-of-Run msgTimeout resolve. 3 Execute sites (:165 turn-1, :176
        turns-2..N, :187 turn-N+1) cfg.Timeout→msgTimeout. :132 doc comment update. config+time ALREADY imported."
  pattern: "S1's generate.go:269 (msgTimeout resolve after ResolveRoleModel) — but Run has NO ResolveRoleModel
            (it receives msgModel/msgReasoning as params), so resolve msgTimeout once at the top of Run with a
            FR-R7/FR-T5 comment, then reuse at the 3 Execute sites."
  gotcha: ":132 doc comment ('Per-turn timeout = cfg.Timeout') becomes inaccurate — update it. Anchor edits by
            STRING (provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose) appears 3× — edit each in context)."

- file: internal/generate/workdesc.go
  why: "RunWorkDescription (signature :63; receives cfg). Top-of-function msgTimeout resolve. 3 Execute sites
        (:75 turn-1, :106 forced-conclusion, :122 answer-turn) cfg.Timeout→msgTimeout. config ALREADY imported;
        time NOT imported but NOT NEEDED for the source change."
  pattern: "Same as multiturn.go: resolve msgTimeout once at the top of RunWorkDescription (no ResolveRoleModel
            call there either — msgModel/msgReasoning are params), reuse at the 3 Execute sites."
  gotcha: ":122 uses `=` (assignment to the existing `out` from :75), NOT `:=`. Swap ONLY the cfg.Timeout arg;
            do not touch the assignment operator. The 3 Execute lines are textually near-identical — anchor each
            by its surrounding context (the forced-conclusion `out2` at :106, the loop-tail `out` at :122)."

- file: internal/hook/exec.go
  why: "Run (the hook runtime). :162 `config.ResolveRoleModel('message', cfg)` is the co-location ANCHOR — add
        msgTimeout right after it (exactly like generate.go:264/269). :182 one-shot Execute + :252 budget display
        cfg.Timeout→msgTimeout. config+time ALREADY imported."
  pattern: "generate.go:264/269 verbatim — the hook HAS the ResolveRoleModel line, so co-locate the timeout twin
            immediately below it. Both :182 and :252 then read msgTimeout."
  gotcha: "TWO cfg.Timeout sites (:182 Execute, :252 budget display) — update BOTH (see Anti-Patterns). The :252
            line is `int((cfg.Timeout * time.Duration(turns)).Minutes())` — swap only the duration source, keep
            the .Minutes()/int()/clamp shape (S1 generate.go:431 did exactly this)."

# MUST EDIT — the 3 test files (1 new test each; clone + flip the small-timeout field)
- file: internal/generate/multiturn_test.go
  why: "TestRun_TurnError (:233) + TestRun_HappyPath (:210) are the Run-test harness (stubtest.Manifest/appendScript
        + Run(ctx, Deps{}, cfg, m, …)). Clone for TestRun_MessageRoleTimeoutBoundsTurn: cfg.Timeout=30s +
        cfg.Roles['message']={Timeout:150ms} + stub SleepMS=2000 + SessionMode=append + MultiTurnChunkTokens=1
        ⇒ turn-1 Execute killed at 150ms ⇒ cause != nil && errors.Is(cause, DeadlineExceeded)."
  pattern: "stubtest.Manifest(bin, stubtest.Options{Out:'ok', SleepMS:2000}); appendMode:='append'; m.SessionMode=&appendMode;
            cfg:=config.Defaults(); cfg.MultiTurnChunkTokens=1; cfg.Timeout=30*time.Second;
            cfg.Roles=map[string]config.RoleConfig{'message':{Timeout:150*time.Millisecond}}."
  gotcha: "multiturn_test.go does NOT import 'time' today — ADD it. stubtest.Manifest DOES support RenderMultiTurn
            (proven by TestRun_TurnError). Deps{} is fine for Run (nil-safe deps.Verbose)."

- file: internal/generate/generate_workdesc_test.go
  why: "TestCommitStaged_WorkDescription_HappyPath is the harness (initRepo/commitRaw/writeFile/stageFile +
        appendScriptManifest + CommitStaged with cfg.WorkDescription). The workdesc path is cleanly isolatable
        (generate.go:282 workDescActive := cfg.WorkDescription != '' ⇒ runs RunWorkDescription, SKIPS one-shot).
        Clone for TestCommitStaged_WorkDescription_MessageRoleTimeout: cfg.WorkDescription='add x' +
        cfg.Timeout=30s + cfg.Roles['message']={Timeout:150ms} + stub SleepMS=2000 + SessionMode=append
        ⇒ RunWorkDescription turn-1 Execute killed at 150ms ⇒ CommitStaged returns *RescueError{Kind:ErrTimeout}."
  pattern: "Use stubtest.Manifest(bin, {Out:'feat: never reached', SleepMS:2000}) + SessionMode=append (the turn-1
            RenderMultiTurn gate). cfg.WorkDescription='add x' activates the workdesc branch. Assert errors.As(err,&re)
            && errors.Is(err, ErrTimeout) (the same shape as S1's TestCommitStaged_MessageRoleTimeout)."
  gotcha: "generate_workdesc_test.go does NOT import 'time' today — ADD it. SessionMode=append is REQUIRED (else the
            turn-1 RenderMultiTurn errors before Execute is reached — you'd get a render-error cause, not a timeout)."

- file: internal/hook/exec_test.go
  why: "TestRun_TimeoutNeverBlock (:269) is the template — stubtest.Manifest(stubBin,{SleepMS:5000}) +
        config.Config{Timeout:50ms, MaxDuplicateRetries:2} + real repo + msgFile; hook one-shot Execute times out
        ⇒ 'hook generation timed out' + msg-file untouched. Clone for TestRun_MessageRoleTimeoutNeverBlock: FLIP the
        small timeout from cfg.Timeout to cfg.Roles['message'] — cfg.Timeout=30s + cfg.Roles['message']={Timeout:50ms}
        + SleepMS=5000 ⇒ the 50ms message-role timeout (NOT 30s) bounds the one-shot Execute."
  pattern: "config.Config{Timeout:30*time.Second, MaxDuplicateRetries:2, Roles:map[string]config.RoleConfig{'message':{Timeout:50*time.Millisecond}}}.
            The hook one-shot uses deps.Manifest.Render (NOT RenderMultiTurn) ⇒ no SessionMode needed (it times out
            before multi-turn). Assert err contains 'timed out' AND msg-file == orig (never-block)."
  gotcha: "exec_test.go ALREADY imports 'time' (TestRun_TimeoutNeverBlock uses it) — NO new import. Note config.Config
            (not config.Defaults()) is used in the hook tests — match that. The :252 budget display is NOT exercised
            here (one-shot times out before multi-turn) — it is grep-guarded instead."

# CONTEXT — the in-tree S1 implementation (the concrete template; read it to mirror comment wording)
- file: internal/generate/generate.go
  why: "S1 is LANDED: :264 ResolveRoleModel, :269 msgTimeout:=ResolveRoleTimeout('message',cfg) [with the 5-line
        FR-R7/FR25 comment 265–268], :340 Execute uses msgTimeout, :431 budget uses msgTimeout. S2 replicates :269's
        resolve + comment in each of the 3 files, and the :340/:431 swaps."
  critical: "READ-ONLY. Do NOT edit generate.go (S1 DONE). The Run call at :436 still passes cfg — UNCHANGED."

# CONTEXT — ResolveRoleTimeout + RoleConfig (LANDED; READ-ONLY)
- file: internal/config/roles.go   # ResolveRoleTimeout (:128) + defaultRoleTimeouts (:12, planner-only)
- file: internal/config/config.go  # RoleConfig.Timeout (:42, time.Duration, 0⇒inherit) + Config.Timeout (:71, default 120s)
  why: "Confirms 'message' has NO built-in ⇒ ResolveRoleTimeout('message',cfg)==cfg.Timeout unless a per-role override."
  critical: "READ-ONLY. Consume ResolveRoleTimeout; do not edit it or the map."

# CONTEXT — Execute's timeout mechanism (the test-proof foundation)
- file: internal/provider/executor.go   # Execute (:44): if timeout>0 { ctx,cancel=WithTimeout(ctx,timeout) }; on ctx.Err() returns DeadlineExceeded
  why: "Explains WHY a stub SleepMS>timeout ⇒ Execute returns DeadlineExceeded (the turn is killed at the timeout).
        This is the mechanism all 3 new tests rely on (and S1's test relied on)."

# CONTEXT — the sibling NOT in scope
- docfile: (P1.M3.T2.S1 PRP, when written)
  why: "P1.M3.T2.S1 wires planner/stager/arbiter (decompose path) to their per-role timeouts. NOT this task — do
        not touch decompose.go/planner.go/stager.go/message.go/arbiter.go Execute sites."
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  generate.go          # READ-ONLY (S1 DONE) — the template: :269 msgTimeout resolve, :340 Execute, :431 budget
  generate_test.go     # READ-ONLY — TestCommitStaged_MessageRoleTimeout (:495) + TestCommitStaged_Timeout (:449)
  multiturn.go         # EDIT — Run: +msgTimeout (top), 3 Execute swaps (:165/:176/:187), :132 doc comment
  multiturn_test.go    # EDIT — +TestRun_MessageRoleTimeoutBoundsTurn + "time" import
  workdesc.go          # EDIT — RunWorkDescription: +msgTimeout (top), 3 Execute swaps (:75/:106/:122)
  generate_workdesc_test.go  # EDIT — +TestCommitStaged_WorkDescription_MessageRoleTimeout + "time" import
  (other *_test.go)    # READ-ONLY — regression net (invariants_test.go:205, etc.)
internal/hook/
  exec.go              # EDIT — Run: +msgTimeout (after :162 ResolveRoleModel), Execute swap (:182), budget swap (:252)
  exec_test.go         # EDIT — +TestRun_MessageRoleTimeoutNeverBlock (time already imported)
internal/config/
  roles.go             # READ-ONLY — ResolveRoleTimeout + defaultRoleTimeouts (LANDED; consume)
  config.go            # READ-ONLY — RoleConfig.Timeout, Config.Timeout (LANDED)
internal/provider/
  executor.go          # READ-ONLY — Execute WithTimeout mechanism (the test-proof foundation)
internal/stubtest/
  stubtest.go          # READ-ONLY — Manifest/NewScript/Options{SleepMS} (the test harness)
go.mod / Makefile      # READ-ONLY — no new dep; test=line70, lint=line103, coverage-gate=line77
```

### Desired Codebase tree with files to be modified

```bash
# MODIFIED (no new files). EXACTLY 6 files:
internal/generate/multiturn.go               # +1 resolve + 3 Execute arg swaps + 1 doc-comment update
internal/generate/workdesc.go                # +1 resolve + 3 Execute arg swaps
internal/hook/exec.go                        # +1 resolve (after :162) + 1 Execute swap (:182) + 1 budget swap (:252)
internal/generate/multiturn_test.go          # +1 test + "time" import
internal/generate/generate_workdesc_test.go  # +1 test + "time" import
internal/hook/exec_test.go                   # +1 test (no import)
# (NOT touched: generate.go [S1 DONE], config/* [LANDED], planner/stager/arbiter [P1.M3.T2.S1], docs/* [P1.M4.T2])
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (S1 is the in-tree template — REPLICATE, don't invent): generate.go:269 already does
//   msgTimeout := config.ResolveRoleTimeout("message", cfg)
// followed by Execute(..., msgTimeout, ...) at :340 and the budget line at :431. S2 is a mechanical
// replication of this exact pattern into the 3 remaining message-role files. Read generate.go:260-270 + :340
// + :431 to mirror the comment wording + the swap mechanics.

// CRITICAL (the change is BEHAVIOR-PRESERVING BY DEFAULT — do not "fix" existing tests): ResolveRoleTimeout
// ("message", cfg) returns cfg.Roles["message"].Timeout if non-zero, ELSE cfg.Timeout (the message role has
// NO built-in — only the planner does). So with no [role.message].timeout override, msgTimeout == cfg.Timeout
// byte-for-byte. Every existing test that sets only cfg.Timeout (no Roles["message"]) STAYS GREEN unchanged.
// Do NOT modify the regression canaries. The ONLY new behavior is under a per-role message override — that
// is what the 3 NEW tests exercise.

// CRITICAL (workdesc.go:122 uses `=`, NOT `:=`): line 122 is `out, _, execErr = provider.Execute(...)` — an
// ASSIGNMENT to the `out`/`execErr` declared at line 75 (the loop reuses them). Swap ONLY the cfg.Timeout arg;
// do not change `=` to `:=` (that would be a compile error — redeclaration). Anchor by the surrounding loop tail.

// CRITICAL (hook/exec.go has TWO cfg.Timeout message-role sites — update BOTH): :182 (one-shot Execute) AND
// :252 (multi-turn budget display `int((cfg.Timeout * time.Duration(turns)).Minutes())`). The contract text
// named only :182 (an oversight). :252 is the IDENTICAL FR-T5 budget site S1 changed in generate.go:431. If you
// inline only :182 and leave :252 on cfg.Timeout, the hook's printed ~Mm total would be WRONG under a message
// override (inconsistent with generate.Run's actual per-turn bound). Resolve ONE local msgTimeout (after :162
// ResolveRoleModel) and use it at BOTH. Do NOT follow the contract's "inline at :182" suggestion literally.

// GOTCHA (NO new source imports): multiturn.go + hook/exec.go already import BOTH "time" and internal/config
// (multiturn uses time for newSessionID; hook uses time for the budget line + config for ResolveRoleModel).
// workdesc.go imports config (yes) but NOT time — and does NOT need it (ResolveRoleTimeout returns time.Duration,
// stored in a local; no time-literal in the source). So NO source file gets a new import.

// GOTCHA (the 2 test files that NEED "time"): multiturn_test.go + generate_workdesc_test.go do NOT import "time"
// today; the new tests use time.Millisecond/time.Second ⇒ ADD "time" to each import block. exec_test.go ALREADY
// imports "time" (TestRun_TimeoutNeverBlock) ⇒ no import change there.

// GOTCHA (Execute's 3rd arg is a plain time.Duration): provider.Execute(ctx, spec, timeout time.Duration, vb).
// msgTimeout is time.Duration — drops in directly. No wrapper, no conversion.

// GOTCHA (the workdesc test is cleanly isolatable): CommitStaged's workDescActive branch (generate.go:282) runs
// RunWorkDescription and SKIPS the one-shot/multi-turn default loop. So a small message-role timeout times out
// RunWorkDescription's turn-1 Execute WITHOUT touching S1's one-shot msgTimeout path. The test goes through
// CommitStaged (the established workdesc-test pattern — there are NO direct RunWorkDescription tests).

// GOTCHA (SessionMode=append is REQUIRED for the multiturn + workdesc timeout tests): Run and RunWorkDescription
// call RenderMultiTurn, whose session_mode gate errors BEFORE Execute if SessionMode != "append". Without it you'd
// get a render-error cause (contains "session_mode"), not a timeout. Set m.SessionMode = &appendMode (appendMode:="append").
// The HOOK one-shot test does NOT need it (hook one-shot uses Render, not RenderMultiTurn, and times out before multi-turn).

// GOTCHA (the hook budget display :252 is a DISPLAY computation — keep the unit math identical): it computes
// totalMin := int((<timeout> * time.Duration(turns)).Minutes()) then clamps totalMin<1 → 1. Swapping cfg.Timeout
// → msgTimeout changes ONLY the source duration; the .Minutes(), int(), clamp are UNCHANGED (S1 generate.go:431).
```

## Implementation Blueprint

### Data models and structure

None. No new types, no new fields, no signature changes. One new local variable (`msgTimeout time.Duration`)
per function (Run, RunWorkDescription, hook Run), consumed at the existing Execute + budget sites.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/generate/multiturn.go — ADD msgTimeout resolve at the top of Run
  - ANCHOR (the first statement inside Run, after the signature — the existing comment block at :148-156):
        func Run(ctx context.Context, deps Deps, cfg config.Config, manifest provider.Manifest,
            sysPrompt, payload, msgModel, msgReasoning string) (msg string, ok bool, cause error) {

            // (1) Chunk the captured payload ...
  - ADD at the very top of Run's body (BEFORE the "(1) Chunk" block), the message-role timeout resolution:
        // FR-R7/FR-T5: resolve the message role's per-turn timeout so [role.message].timeout / --message-timeout
        // bound each multi-turn turn (and the caller's total-budget display) instead of the flat cfg.Timeout.
        // With no per-role override ResolveRoleTimeout returns cfg.Timeout (the message role has no built-in) —
        // behavior-preserving by default.
        msgTimeout := config.ResolveRoleTimeout("message", cfg)
  - GOTCHA: Run has NO ResolveRoleModel call (it receives msgModel/msgReasoning as params), so resolve msgTimeout
    standalone at the top (do NOT look for a ResolveRoleModel line to co-locate with — there isn't one here).
  - NAMING: msgTimeout (mirrors S1's generate.go:269). config is ALREADY imported.

Task 2: EDIT internal/generate/multiturn.go — swap the 3 per-turn Execute sites
  - OLD (:165, turn 1):
        if _, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose); execErr != nil {
  - NEW:
        if _, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose); execErr != nil {
  - OLD (:176, turns 2..N loop):
        if _, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose); execErr != nil {
  - NEW:
        if _, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose); execErr != nil {
  - OLD (:187, turn N+1 final):
        out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
  - NEW:
        out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
  - GOTCHA: the 3 lines are textually near-identical (all `provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)`).
    The :165 and :176 lines are byte-identical (`if _, _, execErr := provider.Execute(...)`) — disambiguate by
    surrounding context (turn-1 is the standalone if; turns-2..N is inside the `for i := 2; i <= N; i++` loop).
    The :187 line differs (assignment to `out`, no `if`). Swap cfg.Timeout → msgTimeout in each.

Task 3: EDIT internal/generate/multiturn.go — update the :132 doc comment
  - OLD (:132):
        // Per-turn timeout = cfg.Timeout (FR-T5; Execute shadows ctx with WithTimeout). Intermediate turns'
  - NEW:
        // Per-turn timeout = msgTimeout = ResolveRoleTimeout("message", cfg) (FR-R7/FR-T5; Execute shadows ctx
        // with WithTimeout). Intermediate turns'
  - GOTCHA: this is a multi-line comment; update ONLY the first line (the one citing cfg.Timeout). Keep the rest.

Task 4: EDIT internal/generate/workdesc.go — ADD msgTimeout resolve at the top of RunWorkDescription
  - ANCHOR (the first statement inside RunWorkDescription — the existing sessionID mint):
        func RunWorkDescription(ctx context.Context, deps Deps, cfg config.Config, manifest provider.Manifest,
            sysPrompt, payload, skeleton, msgModel, msgReasoning string) (msg string, ok bool, cause error) {

            // Mint a fresh, one-run-scope session id (FR-T6 parity — never resumed on a later run).
            sessionID := newSessionID()
  - ADD at the very top of RunWorkDescription's body (BEFORE the sessionID mint):
        // FR-R7: resolve the message role's per-turn timeout so [role.message].timeout / --message-timeout
        // bound each work-description read-loop turn instead of the flat cfg.Timeout. With no per-role
        // override ResolveRoleTimeout returns cfg.Timeout (the message role has no built-in) — behavior-
        // preserving by default.
        msgTimeout := config.ResolveRoleTimeout("message", cfg)
  - GOTCHA: RunWorkDescription has NO ResolveRoleModel call (params) — resolve msgTimeout standalone at the top.
  - config is ALREADY imported; time is NOT imported but NOT needed.

Task 5: EDIT internal/generate/workdesc.go — swap the 3 Execute sites
  - OLD (:75, turn 1):
        out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
  - NEW:
        out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
  - OLD (:106, forced-conclusion turn inside the `if st.rounds >= st.N` block):
            out2, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
  - NEW:
            out2, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
  - OLD (:122, answer turn at the loop tail — NOTE `=`, not `:=`):
        out, _, execErr = provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
  - NEW:
        out, _, execErr = provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
  - GOTCHA: :122 uses `=` (assignment to the `out`/`execErr` declared at :75) — swap ONLY the cfg.Timeout arg,
    PRESERVE the `=`. :106 assigns to `out2` (distinct var) and is indented (inside the if-block). Disambiguate
    :75 (`:=`, `out`) from :106 (`:=`, `out2`, indented) from :122 (`=`, `out`).

Task 6: EDIT internal/hook/exec.go — ADD msgTimeout resolve after the :162 ResolveRoleModel
  - ANCHOR (the Step F message-role resolution, :162):
        // Step F: resolve the message role (FR-H6 — exactly like the single-commit path).
        _, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
  - ADD immediately AFTER it (co-locate the timeout twin — EXACTLY like generate.go:264/269):
        // FR-R7/FR-H6: resolve the message role's timeout so [role.message].timeout / --message-timeout bound
        // the hook one-shot generation (and the multi-turn total budget, FR-T5) instead of the flat cfg.Timeout.
        // With no per-role override ResolveRoleTimeout returns cfg.Timeout (no message built-in) — behavior-
        // preserving by default.
        msgTimeout := config.ResolveRoleTimeout("message", cfg)
  - GOTCHA: the hook HAS the ResolveRoleModel line (unlike Run/RunWorkDescription), so co-locate msgTimeout right
    below it — the verbatim generate.go:264/269 pattern. config + time ALREADY imported.

Task 7: EDIT internal/hook/exec.go — swap the :182 one-shot Execute AND the :252 budget display
  - OLD (:182, one-shot Execute in the dedupe loop):
        out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
  - NEW:
        out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
  - OLD (:252, multi-turn budget display inside the FR-T1 fallback block):
            totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
  - NEW:
            totalMin := int((msgTimeout * time.Duration(turns)).Minutes())
  - GOTCHA: update BOTH :182 and :252 (see Anti-Patterns — do NOT inline only :182). For :252 keep the rest of
    the expression identical (the .Minutes(), int(), and the `if totalMin < 1 { totalMin = 1 }` clamp below it).

Task 8: ADD internal/generate/multiturn_test.go — TestRun_MessageRoleTimeoutBoundsTurn + "time" import
  - ADD "time" to the import block (multiturn_test.go imports config + stubtest today; NO "time").
  - CLONE TestRun_TurnError (:233) / TestRun_HappyPath (:210) harness, FLIP the small-timeout field:
        func TestRun_MessageRoleTimeoutBoundsTurn(t *testing.T) {
            bin := stubtest.Build(t)
            m := stubtest.Manifest(bin, stubtest.Options{Out: "ok", SleepMS: 2000}) // slow on EVERY turn
            appendMode := "append"
            m.SessionMode = &appendMode // REQUIRED: RenderMultiTurn's session_mode gate must pass to reach Execute

            cfg := config.Defaults()
            cfg.MultiTurnChunkTokens = 1 // "aaaa\nbbbb\n" ⇒ 2 chunks ⇒ 3 turns (turn 1 is the one that times out)
            cfg.Timeout = 30 * time.Second                                                 // LARGE global (would NOT time out)
            cfg.Roles = map[string]config.RoleConfig{"message": {Timeout: 150 * time.Millisecond}} // SMALL role → times out

            _, _, cause := Run(context.Background(), Deps{}, cfg, m, "sys", "aaaa\nbbbb\n", "zai/glm-5.2", "")
            if cause == nil {
                t.Fatal("Run cause = nil, want non-nil (message-role 150ms should bound turn-1 Execute, not the 30s global)")
            }
            if !errors.Is(cause, context.DeadlineExceeded) {
                t.Errorf("cause = %v, want context.DeadlineExceeded (the 150ms message-role timeout)", cause)
            }
        }
  - WHY this proves the wiring: with cfg.Timeout=30s the OLD code would NOT time out (30s > 2000ms sleep); only
    because msgTimeout (150ms) is now the bound does turn-1 Execute time out → DeadlineExceeded cause.
  - GOTCHA: check `errors` is imported in multiturn_test.go (it uses `strings`; ADD "errors" if missing — grep first).
            stubtest.Manifest supports RenderMultiTurn (TestRun_TurnError proves it). Deps{} is nil-safe for Run.

Task 9: ADD internal/generate/generate_workdesc_test.go — TestCommitStaged_WorkDescription_MessageRoleTimeout + "time" import
  - ADD "time" to the import block (generate_workdesc_test.go imports config/git/prompt/stubtest today; NO "time").
  - CLONE TestCommitStaged_WorkDescription_HappyPath harness (initRepo/commitRaw/writeFile/stageFile), FLIP the field:
        func TestCommitStaged_WorkDescription_MessageRoleTimeout(t *testing.T) {
            bin := stubtest.Build(t)
            repo := t.TempDir()
            initRepo(t, repo)
            commitRaw(t, repo, "initial")
            writeFile(t, repo, "feature.go", "package main\n\nfunc F() {}\n")
            stageFile(t, repo, "feature.go")

            // turn-1 Execute sleeps 2000ms; the 150ms message-role timeout kills it before any READ/message.
            m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: never reached", SleepMS: 2000})
            appendMode := "append"
            m.SessionMode = &appendMode // REQUIRED: RunWorkDescription's turn-1 RenderMultiTurn gate

            cfg := config.Defaults()
            cfg.WorkDescription = "add F"                                                  // activates the workdesc branch (skips one-shot)
            cfg.Timeout = 30 * time.Second                                                 // LARGE global (would NOT time out)
            cfg.Roles = map[string]config.RoleConfig{"message": {Timeout: 150 * time.Millisecond}} // SMALL role → times out

            _, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
            if err == nil {
                t.Fatal("expected *RescueError on message-role timeout, got nil")
            }
            var re *RescueError
            if !errors.As(err, &re) {
                t.Fatalf("err = %T, want *RescueError (workdesc turn-1 timeout → rescue)", err)
            }
            if !errors.Is(err, ErrTimeout) {
                t.Errorf("errors.Is(err, ErrTimeout) = false, want true (the 150ms message-role timeout bounded turn-1 Execute)")
            }
        }
  - WHY: workDescActive (generate.go:282) runs RunWorkDescription and SKIPS one-shot, so the 150ms message-role
    timeout times out RunWorkDescription's turn-1 Execute (NOT S1's one-shot msgTimeout path) → CommitStaged
    returns *RescueError{Kind:ErrTimeout} (generate.go:308). errors + RescueError + ErrTimeout already used in
    this file (TestCommitStaged_WorkDescription_NoCascadeToMultiTurn). git already imported.

Task 10: ADD internal/hook/exec_test.go — TestRun_MessageRoleTimeoutNeverBlock (clone TestRun_TimeoutNeverBlock)
  - NO import change (exec_test.go already imports "time").
  - CLONE TestRun_TimeoutNeverBlock (:269), FLIP the small-timeout field from cfg.Timeout to cfg.Roles["message"]:
        func TestRun_MessageRoleTimeoutNeverBlock(t *testing.T) {
            stubBin := stubtest.Build(t)
            repoDir, g := initTempRepo(t)

            changePath := filepath.Join(repoDir, "new.txt")
            mustWriteFile(t, changePath, []byte("new content\n"))
            runGit(t, repoDir, "add", "new.txt")

            msgFile := filepath.Join(t.TempDir(), "msg")
            orig := "# original comments\n"
            mustWriteFile(t, msgFile, []byte(orig))

            m := stubtest.Manifest(stubBin, stubtest.Options{SleepMS: 5000})
            cfg := config.Config{
                Timeout:           30 * time.Second, // LARGE global (would NOT time out under the old cfg.Timeout read)
                MaxDuplicateRetries: 2,
                Roles: map[string]config.RoleConfig{"message": {Timeout: 50 * time.Millisecond}}, // SMALL role → times out
            }

            err := Run(context.Background(), generate.Deps{Git: g, Manifest: m}, cfg, msgFile, "")
            if err == nil {
                t.Error("expected timeout error, got nil")
            }
            if err != nil && !strings.Contains(err.Error(), "timed out") {
                t.Errorf("expected timeout error, got: %v", err)
            }

            data, _ := os.ReadFile(msgFile)
            if string(data) != orig {
                t.Errorf("msg-file was modified on timeout; got:\n%s", string(data))
            }
        }
  - WHY: cfg.Timeout=30s ⇒ the OLD code (cfg.Timeout at :182) would NOT time out (30s > 5000ms sleep); only
    because msgTimeout (50ms) is now the bound does the hook one-shot Execute time out → "hook generation timed
    out" + msg-file untouched (never-block, FR-H5). config.Config + Roles map shape matches TestRun_TimeoutNeverBlock's
    config.Config literal. The hook one-shot uses Render (not RenderMultiTurn) ⇒ no SessionMode needed.

Task 11: VERIFY — build (native+cross), vet, format, focused + full tests, lint, coverage, grep guards
  - go build ./... ; GOOS=windows go build ./... ; GOOS=linux go build ./...
  - go vet ./internal/generate/... ./internal/hook/...
  - gofmt -l internal/generate/multiturn.go internal/generate/workdesc.go internal/hook/exec.go \
            internal/generate/multiturn_test.go internal/generate/generate_workdesc_test.go internal/hook/exec_test.go   # must be empty
  - go test ./internal/generate/ -run 'TestRun_MessageRoleTimeoutBoundsTurn|TestCommitStaged_WorkDescription_MessageRoleTimeout|TestRun_HappyPath|TestRun_TurnError|TestCommitStaged_WorkDescription' -v
  - go test ./internal/hook/    -run 'TestRun_MessageRoleTimeoutNeverBlock|TestRun_TimeoutNeverBlock|TestRun_MultiTurn' -v
  - make test ; make lint ; make coverage-gate
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (S1 in-tree template — generate.go:264/269): resolve the message-role model + timeout together:
_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)   // hook/exec.go:162 HAS this line
msgTimeout := config.ResolveRoleTimeout("message", cfg)                // ADD right after (hook); standalone at top (Run/RunWorkDescription)

// PATTERN (the per-turn Execute now reads the role-resolved timeout — 1 token swap, 7 sites total):
provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)   // was cfg.Timeout — multiturn ×3, workdesc ×3, hook ×1

// PATTERN (the FR-T5 budget display reads the role-resolved timeout — hook :252, mirrors generate.go:431):
totalMin := int((msgTimeout * time.Duration(turns)).Minutes())   // was cfg.Timeout — display-only

// PATTERN (the 3 new tests flip WHICH field carries the small timeout — proves the message-role timeout is consumed):
cfg.Timeout = 30 * time.Second                                                      // global is LARGE (old code wouldn't time out)
cfg.Roles = map[string]config.RoleConfig{"message": {Timeout: 150 * time.Millisecond}} // role is SMALL → times out
// → Execute killed at the message-role timeout (DeadlineExceeded / ErrTimeout / "timed out") proves msgTimeout reached Execute.
```

### Integration Points

```yaml
CODE (3 source files):
  - multiturn.go Run:         +msgTimeout resolve (top); Execute :165/:176/:187 → msgTimeout; :132 doc comment.
  - workdesc.go RunWorkDescription: +msgTimeout resolve (top); Execute :75/:106/:122 → msgTimeout.
  - hook/exec.go Run:         +msgTimeout resolve (after :162 ResolveRoleModel); Execute :182 → msgTimeout; budget :252 → msgTimeout.

NO-CHANGE (scope fences):
  - Run signature (multiturn.go:145) + RunWorkDescription signature (workdesc.go:63): UNCHANGED (both receive cfg).
  - generate.go (S1 DONE) + the Run call at generate.go:436 (still passes cfg): UNCHANGED.
  - ResolveRoleTimeout / defaultRoleTimeouts (roles.go) + Config layers / --message-timeout (P1.M1): UNCHANGED (LANDED).
  - planner/stager/arbiter Execute sites (P1.M3.T2.S1): UNCHANGED.

CONSUMERS OF THIS CHANGE:
  - The [role.message].timeout / --message-timeout / STAGECOACH_MESSAGE_TIMEOUT / stagecoach.role.message.timeout
    settings now take effect on the multi-turn fallback (every turn), the work-description read loop (every turn),
    and the hook one-shot path — previously resolved into cfg.Roles["message"].Timeout but read by no call site
    in these 3 files (they read cfg.Timeout). S1 already wired the single-commit one-shot; S2 completes the message role.

DOWNSTREAM (sibling items, NOT this task):
  - P1.M3.T2.S1 wires planner/stager/arbiter (decompose path) to their per-role timeouts.
  - P1.M4.T1.S1 adds unit tests for ResolveRoleTimeout + config loading (this task adds the CONSUMPTION tests).
  - P1.M4.T2.S1 syncs docs (this task: "DOCS: none — internal wiring").

NO database / migration / routes / new types / new source imports / new flag / config change / signature change.
  - The 2 test files (multiturn_test.go, generate_workdesc_test.go) add the "time" import (the source files add none).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross build (no platform tags in the 3 files; arg swaps + 1 local must build everywhere).
go build ./...
GOOS=windows go build ./...
GOOS=linux   go build ./...
# Expected: all clean. A failure means an edit strayed (e.g. a typo in msgTimeout, or workdesc :122's `=` became `:=`,
#           or a test used time without importing it).

# Vet.
go vet ./internal/generate/... ./internal/hook/...
# Expected: clean.

# Format.
gofmt -l internal/generate/multiturn.go internal/generate/workdesc.go internal/hook/exec.go \
       internal/generate/multiturn_test.go internal/generate/generate_workdesc_test.go internal/hook/exec_test.go
# Expected: empty. If listed: gofmt -w the file(s).

# Lint.
make lint      # golangci-lint (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. msgTimeout is used at every swap site (no `unused`); cfg.Timeout's direct reads in these
#           files drop to ZERO after the edit — that's FINE (cfg is still used; a struct field being unread is not a lint error).

# Scope guard: ONLY the 6 expected files changed.
git diff --name-only
# Expected: exactly:
#   internal/generate/multiturn.go
#   internal/generate/multiturn_test.go
#   internal/generate/workdesc.go
#   internal/generate/generate_workdesc_test.go
#   internal/hook/exec.go
#   internal/hook/exec_test.go
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 3 NEW behavioral proofs (message-role timeout bounds the per-turn/one-shot Execute).
go test ./internal/generate/ -run 'TestRun_MessageRoleTimeoutBoundsTurn' -v
# Expected: PASS — Run turn-1 Execute killed at 150ms (cfg.Timeout=30s would NOT have timed out).
go test ./internal/generate/ -run 'TestCommitStaged_WorkDescription_MessageRoleTimeout' -v
# Expected: PASS — RunWorkDescription turn-1 Execute killed at 150ms → *RescueError{ErrTimeout}.
go test ./internal/hook/    -run 'TestRun_MessageRoleTimeoutNeverBlock' -v
# Expected: PASS — hook one-shot Execute killed at 50ms → "hook generation timed out" + msg-file untouched.

# The REGRESSION canaries (behavior-preserving by default — must stay GREEN unchanged).
go test ./internal/generate/ -run 'TestRun_HappyPath|TestRun_TurnError|TestRun_FinalParseEmpty|TestRun_NonAppendManifest' -v
go test ./internal/generate/ -run 'TestCommitStaged_WorkDescription' -v
go test ./internal/hook/    -run 'TestRun_TimeoutNeverBlock|TestRun_MultiTurnSuccess_WritesMessageFile|TestRun_MultiTurnFailure_NeverBlock|TestRun_MultiTurnSkipped_NonAppend' -v
# Expected: all PASS (msgTimeout==cfg.Timeout under their configs — no Roles["message"] set).

# Full race suite + coverage gate (PRD §20.3: ≥85% on internal/{git,provider,generate,config}).
make test
make coverage-gate
# Expected: green / passes. The 3 new tests ADD coverage to the msgTimeout paths; nothing is removed.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary (the wiring links into the binary; proves no compile/link break).
make build

# Manual sanity (OPTIONAL — the unit tests are the deterministic proof): a --message-timeout override bounds a
# real multi-turn / work-description / hook run. A full per-role-timeout e2e is P1.M4.T1.S1's deliverable, NOT
# this subtask. The 3 new unit tests ARE the within-scope proof.
```

> **Note**: this subtask is the call-site wiring for the 3 remaining message-role transports. The within-scope
> proof is: clean build/vet/lint/gofmt + the 3 new unit tests + the full regression green + the grep guards.
> The decompose (planner/stager/arbiter) wiring is P1.M3.T2.S1; the docs sync is P1.M4.T2.S1; the full
> per-role-timeout e2e is P1.M4.T1.S1.

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: NO cfg.Timeout remains in the 3 source files' Execute/budget sites (all swapped to msgTimeout).
grep -n 'cfg.Timeout' internal/generate/multiturn.go internal/generate/workdesc.go internal/hook/exec.go
# Expected: ZERO hits (the :132 doc comment was updated; :182/:252 swapped; :165/176/187/:75/106/122 swapped).
# (If any remain, a swap was missed. The ONLY cfg.Timeout references left in the package are in generate.go's
#  comments [S1] + the test files' cfg.Timeout= assignments.)

# Guard 2: msgTimeout is resolved once per function (3 resolves total).
grep -n 'msgTimeout := config.ResolveRoleTimeout("message", cfg)' internal/generate/multiturn.go internal/generate/workdesc.go internal/hook/exec.go
# Expected: 3 hits (1 per file: multiturn Run top, workdesc RunWorkDescription top, hook after :162 ResolveRoleModel).

# Guard 3: the Execute sites use msgTimeout (7 total: 3 multiturn + 3 workdesc + 1 hook).
grep -c 'provider.Execute(ctx, \*spec, msgTimeout, deps.Verbose)' internal/generate/multiturn.go internal/generate/workdesc.go internal/hook/exec.go
# Expected: multiturn.go:3, workdesc.go:3, hook/exec.go:1.

# Guard 4: the hook budget display uses msgTimeout (FR-T5, mirrors generate.go:431).
grep -n 'msgTimeout \* time.Duration(turns)' internal/hook/exec.go
# Expected: 1 hit (:252). And:
grep -n 'cfg.Timeout \* time.Duration(turns)' internal/hook/exec.go
# Expected: ZERO hits.

# Guard 5: Run + RunWorkDescription signatures UNCHANGED (S2 resolves locally; no signature change).
grep -n 'func Run(ctx context.Context, deps Deps, cfg config.Config' internal/generate/multiturn.go
grep -n 'func RunWorkDescription(ctx context.Context, deps Deps, cfg config.Config' internal/generate/workdesc.go
# Expected: 1 hit each, UNCHANGED (cfg still a param; S2 adds a local, not a param).

# Guard 6: NO edits to generate.go / config/* (S1 DONE + LANDED).
git diff --name-only | grep -E 'generate/generate.go|config/roles.go|config/config.go'
# Expected: EMPTY (S1 owns generate.go; config/* LANDED).

# Guard 7: the 3 new tests exist and set a LARGE global + a SMALL message-role Timeout.
grep -n 'TestRun_MessageRoleTimeoutBoundsTurn' internal/generate/multiturn_test.go
grep -n 'TestCommitStaged_WorkDescription_MessageRoleTimeout' internal/generate/generate_workdesc_test.go
grep -n 'TestRun_MessageRoleTimeoutNeverBlock' internal/hook/exec_test.go
# Expected: 1 hit each. And each test has cfg.Timeout = 30*time.Second (or 30) near cfg.Roles["message"]={Timeout: ...}.

# Guard 8: the 2 test files added the "time" import.
grep -n '"time"' internal/generate/multiturn_test.go internal/generate/generate_workdesc_test.go
# Expected: 1 hit each. (exec_test.go already had it.)

# Guard 9: the existing regression canaries are UNCHANGED.
git diff internal/generate/multiturn_test.go | grep -E '^\-.*TestRun_HappyPath|^\-.*TestRun_TurnError'
git diff internal/hook/exec_test.go | grep -E '^\-.*TestRun_TimeoutNeverBlock'
git diff internal/generate/generate_workdesc_test.go | grep -E '^\-.*TestCommitStaged_WorkDescription_HappyPath'
# Expected: EMPTY (existing tests are not modified — only the new tests are added).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=windows/linux go build ./...` clean
- [ ] `go vet ./internal/generate/... ./internal/hook/...` clean
- [ ] `gofmt -l` empty on the 6 changed files
- [ ] `make lint` zero errors (msgTimeout used at every swap site)
- [ ] `make test` (race) green — incl. the 3 new tests + all unchanged regression canaries
- [ ] `make coverage-gate` ≥85% on internal/{git,provider,generate,config} (the new tests add coverage)

### Feature Validation
- [ ] multiturn.go Run: msgTimeout resolved at top; Execute :165/:176/:187 use msgTimeout; :132 doc updated
- [ ] workdesc.go RunWorkDescription: msgTimeout resolved at top; Execute :75/:106/:122 use msgTimeout (`=` preserved at :122)
- [ ] hook/exec.go Run: msgTimeout resolved after :162 ResolveRoleModel; Execute :182 + budget :252 use msgTimeout
- [ ] TestRun_MessageRoleTimeoutBoundsTurn proves Run's per-turn bound is the message-role timeout
- [ ] TestCommitStaged_WorkDescription_MessageRoleTimeout proves RunWorkDescription's per-turn bound is the message-role timeout
- [ ] TestRun_MessageRoleTimeoutNeverBlock proves the hook one-shot bound is the message-role timeout (+ never-block)
- [ ] All regression canaries GREEN unchanged

### Scope-Boundary Validation
- [ ] `git diff --name-only` == the 6 files
- [ ] Run + RunWorkDescription signatures UNCHANGED; generate.go + the Run call at :436 UNCHANGED
- [ ] config/roles.go (ResolveRoleTimeout) + config layers + --message-timeout flag UNCHANGED (all LANDED)
- [ ] planner/stager/arbiter Execute sites UNCHANGED (P1.M3.T2.S1)
- [ ] NO new source import (the 2 test files add "time"); NO new type/flag; NO docs change (P1.M4.T2)
- [ ] Grep guards 1–9 (Level 4) all pass

### Code Quality & Docs
- [ ] msgTimeout comments cite FR-R7/FR-T5 (FR-H6 in the hook) + the behavior-preserving-by-default note
- [ ] msgTimeout resolved ONCE per function (co-located with ResolveRoleModel in the hook; standalone at top in Run/RunWorkDescription)
- [ ] All edits anchored by STRING (grep), not by line number alone (lines verified but may drift in parallel work)

---

## Anti-Patterns to Avoid

- ❌ Don't inline ONLY hook/exec.go:182 and leave :252 on cfg.Timeout. The contract text named only :182, but
  hook/exec.go has TWO message-role cfg.Timeout sites: :182 (one-shot Execute) AND :252 (the multi-turn budget
  display `int((cfg.Timeout * time.Duration(turns)).Minutes())`). :252 is the IDENTICAL FR-T5 budget site S1
  changed in generate.go:431. If you swap only :182, the hook's printed `~Mm total` would be WRONG under a
  message override (it would compute cfg.Timeout×(N+1) while generate.Run's actual per-turn bound — wired by THIS
  same task in multiturn.go — uses msgTimeout). Resolve ONE local `msgTimeout` (after the :162 ResolveRoleModel)
  and use it at BOTH. The contract's "inline at :182" was an oversight (one of two sites); do not follow it literally.
- ❌ Don't change the `=` to `:=` at workdesc.go:122. Line 122 is `out, _, execErr = provider.Execute(...)` — an
  ASSIGNMENT to the `out`/`execErr` declared at :75 (the read loop reuses them). Swap ONLY the cfg.Timeout arg.
  Changing `=` to `:=` is a compile error (redeclaration of `out`/`execErr` in the same scope) or a shadowing bug.
- ❌ Don't add a "time" import to workdesc.go (the SOURCE file). workdesc.go doesn't import "time" today and
  doesn't need it: `msgTimeout := config.ResolveRoleTimeout("message", cfg)` returns a `time.Duration` stored in a
  local — no `time.` symbol is referenced in the source. (The workdesc TEST file, generate_workdesc_test.go, DOES
  need "time" added — that's a test-file change, separate.) Adding "time" to workdesc.go is an unused-import lint error.
- ❌ Don't co-locate msgTimeout with a ResolveRoleModel line in multiturn.go or workdesc.go. Those functions do NOT
  call ResolveRoleModel (they receive msgModel/msgReasoning as PARAMETERS). Resolve msgTimeout standalone at the top
  of each function (with an FR-R7/FR-T5 comment). Only hook/exec.go HAS the ResolveModel line (:162) to co-locate with.
- ❌ Don't re-resolve msgTimeout at each Execute site. Resolve it ONCE per function (Task 1/4/6) and reuse the local
  at all the Execute + budget sites. Re-resolving per site is harmless (the function is pure) but needlessly verbose.
- ❌ Don't change Run's or RunWorkDescription's signature to accept a msgTimeout param. Both already receive `cfg`;
  resolve msgTimeout LOCALLY inside each (the contract: "they can call config.ResolveRoleTimeout("message", cfg)
  internally"). Changing the signature crosses the S1/S2 boundary (S1 left the Run call at generate.go:436 passing
  cfg, expecting S2 to wire Run's internals) and creates a coordination hazard.
- ❌ Don't modify the existing regression canaries (TestRun_HappyPath, TestRun_TurnError, TestRun_NonAppendManifest,
  TestCommitStaged_WorkDescription_HappyPath/_RoundBudgetForcesConclusion/_NoCascadeToMultiTurn/_NonAppendProviderRescues,
  TestRun_TimeoutNeverBlock, TestRun_MultiTurn*). They set cfg via config.Defaults()/config.Config{} with NO
  cfg.Roles["message"], so after this task msgTimeout==cfg.Timeout → identical behavior → they stay GREEN unchanged.
  They ARE the behavior-preserving proof. The ONLY new behavior is under a per-role message override — proven by the
  3 NEW tests. If a canary fails, a swap was wired wrong (e.g. wrong role string, or a message built-in accidentally added).
- ❌ Don't add a "message built-in" to defaultRoleTimeouts. The message role has NO built-in — only planner (480s).
  ResolveRoleTimeout("message", cfg) returns cfg.Timeout when no override; that IS the intended behavior-preserving
  default. Adding a message built-in would silently change every default-config user's timeout and conflict with
  P1.M2.T2.S1's global-120s flip.
- ❌ Don't forget SessionMode=append in the multiturn + workdesc timeout TESTS. Run and RunWorkDescription call
  RenderMultiTurn, whose session_mode gate errors BEFORE Execute if SessionMode != "append". Without it the test gets
  a render-error cause (contains "session_mode"), not a DeadlineExceeded timeout — the test would fail its assertion.
  Set `m.SessionMode = &appendMode` (the harness pattern from TestRun_TurnError/stubAppendManifest). The HOOK test
  does NOT need it (the hook one-shot uses Render, not RenderMultiTurn, and times out before multi-turn).
- ❌ Don't couple this to the global 480s→120s flip (P1.M2.T2.S1, in-flight parallel). This task reads
  ResolveRoleTimeout("message", cfg), which returns cfg.Timeout when no message override — WHATEVER that value is
  (480s today, 120s after the sibling lands). The two are independent.
- ❌ Don't write a test for the hook budget DISPLAY line (:252) specifically. The hook test times out on the one-shot
  (:182) before multi-turn is reached. The :252 correctness is ensured by using the same `msgTimeout` local as :182
  (grep-guarded) and mirrors S1's generate.go:431 handling. Over-testing the display line is low-value and brittle.
- ❌ Don't edit generate.go, config/roles.go, config/config.go, or any planner/stager/arbiter file. generate.go is
  S1 (DONE); config/* is LANDED; the decompose roles are P1.M3.T2.S1. The grep guards (6) enforce the scope.

---

## Confidence Score: 9/10

This is a mechanical replication of S1's ALREADY-LANDED pattern (generate.go:269/340/431) into 3 more message-role
files, plus 3 tests that clone existing harnesses and flip which field carries the small timeout. Every edit site is
verified by grep with its unique string (multiturn 165/176/187 + 132 doc; workdesc 75/106/122 [note :122 `=`]; hook
182 Execute + 252 budget + 162 anchor). The dependency (ResolveRoleTimeout) is LANDED; the no-message-built-in fact
makes every change behavior-preserving by default (full regression stays green). The import situation is mapped
exactly (no source imports; 2 test files add "time"). The one subtlety the implementer must honor beyond the literal
contract: hook/exec.go has TWO cfg.Timeout message-role sites (:182 AND :252), and BOTH must use msgTimeout (the
contract named only :182 — an oversight; :252 is the FR-T5 budget line S1 changed in generate.go:431). The grep guards
(catch a missed :252 swap) + the workdesc :122 `=`-vs-`:=` gotcha + the test-file "time"-import requirement are the
implementation traps; all are documented above. The -1 from 10/10 reflects the parallel-work line-drift risk (S1 may
still be settling in the tree) — anchor every edit by STRING, not by the line numbers cited here (which are verified
as of this research but may shift).
