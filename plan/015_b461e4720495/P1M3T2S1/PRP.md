name: "P1.M3.T2.S1 — Wire per-role timeouts (ResolveRoleTimeout) at the 4 decompose Execute sites (planner/stager/message/arbiter) (FR-R7)"
description: >
  The decompose-path consumer-wiring subtask of P1.M3: make each of the 4 multi-commit decomposition
  roles resolve its OWN timeout via `config.ResolveRoleTimeout("<role>", deps.Config)` and pass it to
  `provider.Execute`, instead of all 4 sharing the flat `deps.Config.Timeout`. This is a SURGICAL,
  mechanical edit — ONE new resolve line + ONE arg swap PER role file — cloning the EXACT pattern S1
  (P1.M3.T1.S1) already LANDED in generate.go for the message role. The 4 sites (verified): planner.go:65
  (ResolveRoleModel) → add plannerTimeout at :65 → swap the :124 Execute arg; stager.go:96 → :110;
  message.go:127 → :155; arbiter.go:82 → :100. Each Execute is the ONLY `deps.Config.Timeout` reference
  in its file. NO new imports (config already imported by all 4; a time.Duration passthrough needs no
  `import "time"`). The dependency — `func ResolveRoleTimeout(role string, cfg Config) time.Duration` +
  the `defaultRoleTimeouts` map (planner=480s, the ONLY built-in) — is LANDED (P1.M2.T1.S1). CRITICAL
  ASYMMETRY: the planner is the ONLY role with a built-in (480s), so this wiring is a BEHAVIOR CHANGE
  for the planner (was `deps.Config.Timeout`=120s; now 480s built-in — INTENTIONAL per PRD §9.15/§16.1:
  "the planner needs more time than message/stager/arbiter"; P1.M2.T2.S1 flipped the global 480s→120s
  precisely so the planner 480s built-in would be longer). This BREAKS `TestCallPlanner_Timeout`
  (planner_test.go:337), which sets `cfg.Timeout=100ms` expecting it to bound the planner — it MUST be
  fixed by moving the 100ms to `cfg.Roles["planner"].Timeout` (and setting cfg.Timeout large so the test
  becomes a behavioral proof). For stager/message/arbiter there is NO built-in, so the wiring is
  BEHAVIOR-PRESERVING BY DEFAULT (ResolveRoleTimeout returns cfg.Timeout) — all their existing _Timeout
  tests (stager/message/arbiter/chain) stay GREEN as canaries. Add ONE new behavioral-proof test per
  non-planner role (clone the existing _Timeout test, flip global-LARGE/per-role-SMALL) to PROVE the
  per-role override bounds Execute — the S1 precedent (TestCommitStaged_MessageRoleTimeout). Each role's
  DISTINCT failure semantic is UNCHANGED (only the timeout VALUE changes): planner→ErrPlannerFailed
  non-rescue; stager→ErrStagerFailed; message→*generate.RescueError{ErrTimeout} exit 124; arbiter→graceful
  null. NOT in scope: generate.go (S1 DONE), multiturn/workdesc/hook (S2, parallel — different files),
  internal/config/* (LANDED), ResolveRoleTimeout itself, docs (P1.M4.T2 — contract: "DOCS: none —
  internal wiring"), the global 480s→120s flip (P1.M2.T2.S1, DONE), the per-role config TESTS for
  ResolveRoleTimeout itself (P1.M4.T1.S1).

---

## Goal

**Feature Goal**: Wire per-role generation timeouts (PRD §9.15 FR-R7, §16.1) into the multi-commit
decomposition pipeline so each of the 4 roles — planner, stager, message, arbiter — passes its OWN
resolved timeout (`config.ResolveRoleTimeout("<role>", deps.Config)`) to `provider.Execute`, instead of
all 4 sharing the flat `deps.Config.Timeout`. This makes `[role.<role>].timeout` / `--<role>-timeout` /
`STAGECOACH_<ROLE>_TIMEOUT` / `stagecoach.role.<role>.timeout` bound each decompose role's generation
independently, and realizes the planner's 480s built-in default (longer than the 120s global, because
open-ended decomposition planning needs more time than message/stager/arbiter).

**Deliverable**: A surgical edit set to 4 source files + 4 test files:
1. `internal/decompose/planner.go` — +1 resolve (`plannerTimeout`), 1 Execute arg swap (:124).
2. `internal/decompose/stager.go` — +1 resolve (`stagerTimeout`), 1 Execute arg swap (:110).
3. `internal/decompose/message.go` — +1 resolve (`messageTimeout`), 1 Execute arg swap (:155).
4. `internal/decompose/arbiter.go` — +1 resolve (`arbiterTimeout`), 1 Execute arg swap (:100).
5. `internal/decompose/planner_test.go` — FIX `TestCallPlanner_Timeout` (move 100ms global→per-role; set global large). Becomes the planner behavioral proof.
6. `internal/decompose/stager_test.go` — +1 test (`TestStageConcept_PerRoleTimeout`).
7. `internal/decompose/message_test.go` — +1 test (`TestGenerateMessage_PerRoleTimeout`).
8. `internal/decompose/arbiter_test.go` — +1 test (`TestRunArbiter_PerRoleTimeoutNull`).

**Success Definition**:
- Each of the 4 decompose Execute sites (planner:124, stager:110, message:155, arbiter:100) reads its
  resolved `<role>Timeout` (`config.ResolveRoleTimeout("<role>", deps.Config)`), not `deps.Config.Timeout`.
  The resolve is co-located RIGHT AFTER each file's existing `config.ResolveRoleModel("<role>", …)` line
  (the "model/timeout twins" idiom from S1's generate.go:264/269).
- The planner defaults to 480s (built-in) when no per-role override — a deliberate behavior change
  (was `deps.Config.Timeout`=120s). stager/message/arbiter default to `cfg.Timeout` (no built-in) —
  behavior-preserving.
- `TestCallPlanner_Timeout` is FIXED (it would otherwise break: planner now 480s, not cfg.Timeout=100ms)
  and turned into the planner behavioral proof (global 30s + per-role 100ms + 2000ms stub sleep → times
  out at 100ms → ErrPlannerFailed + DeadlineExceeded).
- 3 new behavioral-proof tests (stager/message/arbiter) prove the per-role override bounds Execute:
  global 30s (no timeout vs 2000ms sleep) + per-role 100ms (times out) → the role's distinct failure
  semantic fires. The EXISTING _Timeout tests (cfg.Timeout small) stay GREEN as
  behavior-preserving-by-default canaries.
- Each role's failure semantic is UNCHANGED (only the timeout VALUE differs): planner→ErrPlannerFailed;
  stager→ErrStagerFailed; message→*generate.RescueError{Kind:ErrTimeout}; arbiter→graceful null.
- `go build ./...` clean; `gofmt -l` empty; `go vet ./internal/decompose/...` clean;
  `go test ./internal/decompose/ -race` green (new tests + fixed planner test + all existing canaries);
  `make test` + `make lint` clean.
- NO new imports in any source file. Scope: `git status --porcelain` == the 8 files above. NO edit to
  internal/config/*, generate.go, multiturn.go/workdesc.go/hook (S2's), or any PRD/task file.

## User Persona (if applicable)

**Target User**: A developer running multi-commit decomposition (`stagecoach` with an un-staged dirty
tree) who wants to bound the PLANNER's open-ended planning time independently of the message/stager/
arbiter generations — e.g. setting `[role.planner].timeout = "600s"` for a huge refactor's planning
without also giving the (fast) message role 600s per concept. Also the operator whose planner times out
at the 120s global on a big tree and who needs the 480s built-in (now the default) or a higher override.

**Use Case**: User sets `[role.planner].timeout = "600s"`. Load() merges it into cfg.Roles["planner"].Timeout
(P1.M1 LANDED) → callPlanner resolves `plannerTimeout := ResolveRoleTimeout("planner", cfg)` = 600s →
the planner Execute is bounded at 600s. The message/stager/arbiter roles keep their own (cfg.Timeout or
their own overrides). Without this wiring, all 4 roles read `deps.Config.Timeout` directly and IGNORED
the per-role setting.

**User Journey**: `[role.planner] timeout = "600s"` → callPlanner :65 resolves 600s → :124 Execute
bounded at 600s → if planning exceeds, callPlanner returns ErrPlannerFailed (non-rescue; no commits yet).
The message/stager/arbiter roles resolve their OWN timeouts at their own sites.

**Pain Points Addressed**: FR-R7 — the per-role timeout was RESOLVABLE (P1.M2.T1.S1) and CONFIGURABLE
(P1.M1) and consumed on the single-commit path (S1), but NOT YET on the decompose path (all 4 roles
still read `deps.Config.Timeout`). This task closes that gap for all 4 decompose roles.

## Why

- **FR-R7 / §9.15 / §16.1**: "Each role resolves its own timeout independently." The accessor
  (`ResolveRoleTimeout`) + all config layers are LANDED; S1 consumed them for the single-commit message
  role. This task extends the same consumption to the 4 decompose roles — the last Execute sites still
  reading the flat `deps.Config.Timeout`.
- **Planner needs more time (PRD §9.15/§16.1)**: the planner does open-ended decomposition planning
  (analyze the full diff, decide count + partition) and most often needs longer than the message/stager/
  arbiter roles. P1.M2.T2.S1 set the global default to 120s AND shipped the planner's 480s built-in so
  the planner would default to longer — but that built-in only takes effect once the planner's Execute
  site reads `ResolveRoleTimeout` (this task). Today the planner still reads `deps.Config.Timeout` (120s),
  so the 480s built-in is dormant; this task ACTIVATES it.
- **Independent tunability**: a user can now give the planner 600s, the stager 60s (it just runs git add),
  the message 120s, and the arbiter 30s — each bounded independently, instead of one global knob.
- **Behavior-preserving for 3 of 4 roles**: stager/message/arbiter have NO built-in, so
  `ResolveRoleTimeout` returns `cfg.Timeout` when no override — invisible to every existing test and
  every default-config user. ONLY the planner's default changes (120s→480s, by design).
- **Bounded scope**: 4 one-line resolves + 4 one-word swaps + 1 test fix + 3 new tests. No new types, no
  new imports, no signature changes, no docs (contract: "DOCS: none").

## What

**User-visible behavior**: With no per-role timeouts configured, the planner now defaults to 480s
(longer; was 120s) and stager/message/arbiter are unchanged (120s global). With a per-role override set,
that role's Execute is bounded at the override (each independently). Failure semantics are unchanged per
role (planner/stager non-rescue; message rescue exit 124; arbiter graceful null).

**Technical change**: in each of the 4 role files, add `<role>Timeout := config.ResolveRoleTimeout("<role>",
deps.Config)` immediately after the existing `config.ResolveRoleModel("<role>", deps.Config)` line, and
replace the `deps.Config.Timeout` argument in the single `provider.Execute` call with `<role>Timeout`.

### Success Criteria
- [ ] `internal/decompose/planner.go`: `plannerTimeout := config.ResolveRoleTimeout("planner", deps.Config)`
      added after line 65's `ResolveRoleModel`; line 124's `provider.Execute(…, deps.Config.Timeout, …)`
      → `provider.Execute(…, plannerTimeout, …)`.
- [ ] `internal/decompose/stager.go`: `stagerTimeout := config.ResolveRoleTimeout("stager", deps.Config)`
      added after line 96; line 110 Execute → `stagerTimeout`.
- [ ] `internal/decompose/message.go`: `messageTimeout := config.ResolveRoleTimeout("message", deps.Config)`
      added after line 127; line 155 Execute → `messageTimeout`.
- [ ] `internal/decompose/arbiter.go`: `arbiterTimeout := config.ResolveRoleTimeout("arbiter", deps.Config)`
      added after line 82; line 100 Execute → `arbiterTimeout`.
- [ ] NO new `import` in any of the 4 source files (config already imported; time.Duration passthrough
      needs no `import "time"`). (grep guard.)
- [ ] `TestCallPlanner_Timeout` (planner_test.go) is FIXED: the 100ms moves from `cfg.Timeout` to
      `cfg.Roles["planner"].Timeout`; `cfg.Timeout` set to 30s (so the test proves the PER-ROLE, not
      global, bounds Execute). Still asserts `ErrPlannerFailed` + `DeadlineExceeded`.
- [ ] `TestStageConcept_PerRoleTimeout`, `TestGenerateMessage_PerRoleTimeout`,
      `TestRunArbiter_PerRoleTimeoutNull` added (clone of the existing _Timeout test + global 30s +
      per-role 100ms + 2000ms stub sleep), each asserting the role's distinct failure semantic.
- [ ] The EXISTING `TestStageConcept_Timeout`/`TestGenerateMessage_Timeout`/`TestRunArbiter_TimeoutNull`/
      `TestResolveArbiter_RescueErrorPropagation` stay GREEN unchanged (behavior-preserving canaries).
- [ ] `go build ./...` clean; `go vet ./internal/decompose/...` clean; `gofmt -l` empty on the 8 files.
- [ ] `go test ./internal/decompose/ -race` green; `make test` + `make lint` clean.
- [ ] `git status --porcelain` == the 8 files (4 role .go + 4 _test.go). ZERO changes to internal/config/,
      generate.go, multiturn.go/workdesc.go/hook, root.go, or any PRD/task file.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact line number of every ResolveRoleModel + Execute site in all 4 files (verified by grep),
the LANDED `ResolveRoleTimeout` signature + the planner-only `defaultRoleTimeouts` (480s) map (quoted in
full), the S1 canonical pattern to clone (generate.go:264/269/340, quoted), the CRITICAL planner-built-in
asymmetry and the ONE test it breaks (`TestCallPlanner_Timeout`) with the exact fix, the full pinning-test
inventory (5 tests, only 1 breaks), the distinct per-role failure semantics + their exact assertion
idioms (quoted from each existing _Timeout test), the stubtest harness (`stubtest.Build`/`Manifest`/
`Options{Out,SleepMS}` + the per-file `*Deps` helpers), the `config.RoleConfig{Timeout}` struct + the
`config.Defaults()` Roles==nil fact (so tests must init the map), the no-new-imports fact (time.Duration
passthrough), the scope fences (no overlap with S1/S2/config/docs), and 8 grep guards.

### Documentation & References

```yaml
# MUST READ — codebase-specific findings for THIS item (the planner-built-in gotcha + the test inventory)
- docfile: plan/015_b461e4720495/P1M3T2S1/research/findings.md
  why: "§0 ResolveRoleTimeout is LANDED + the CRITICAL planner-only built-in (480s) asymmetry — the
        wiring is behavior-preserving for stager/message/arbiter but a behavior CHANGE for the planner;
        §1 the exact line numbers of all 4 ResolveRoleModel+Execute sites (planner 65/124, stager 96/110,
        message 127/155, arbiter 82/100); §2 the S1 canonical pattern (generate.go:264/269/340); §3 no new
        imports; §4 the pinning-test inventory — ONLY TestCallPlanner_Timeout breaks + its exact fix
        (move 100ms to cfg.Roles['planner'].Timeout, set cfg.Timeout=30s); §5 the 3 new behavioral-proof
        tests + the per-role failure-semantics table; §6 the stubtest harness; §7 scope fences;
        §8 validation commands; §9 confirms the 480s planner default is DESIGNED (not a bug)."
  critical: "ResolveRoleTimeout('planner', cfg) returns 480s (built-in) when cfg.Roles has no planner
             entry — NOT cfg.Timeout. This BREAKS TestCallPlanner_Timeout (it sets cfg.Timeout=100ms
             expecting that to bound the planner). The fix is MANDATORY. The other 3 roles have no
             built-in → behavior-preserving → their tests stay green."

# MUST READ — the LANDED API being consumed (ResolveRoleTimeout + defaultRoleTimeouts) — read its contract
- file: internal/config/roles.go
  why: "ResolveRoleTimeout (line 96): per-role override > built-in (planner 480s) > cfg.Timeout. Its godoc
        states the planner is the ONLY built-in and a non-zero cfg.Roles[role].Timeout ALWAYS wins (even
        for the planner). defaultRoleTimeouts (line 8): map{'planner': 480s} — confirm planner-only.
        ResolveRoleModel (line 46) is the twin already called at each site — co-locate the timeout resolve
        right after it (the S1 idiom)."
  pattern: "ResolveRoleTimeout mirrors ResolveRoleModel's per-role-then-global structure. Read its godoc
            for the exact precedence + the 'returns cfg.Timeout for non-planner roles' fact."
  gotcha: "Do NOT modify internal/config/roles.go — it is LANDED (P1.M2.T1.S1). This item CONSUMES it."

# MUST READ — the S1 canonical pattern (LANDED in generate.go — clone it verbatim for each role)
- file: internal/generate/generate.go
  why: "generate.go:264 ResolveRoleModel('message', cfg) → :269 msgTimeout := ResolveRoleTimeout('message',
        cfg) (co-located, the 'model/timeout twins') → :340 provider.Execute(ctx, *spec, msgTimeout, …)
        (the swap). This is EXACTLY the shape to reproduce in each decompose role file. :431 also uses
        msgTimeout for a budget display (decompose has no analog — only the 1 Execute swap per file)."
  pattern: "Resolve the timeout RIGHT AFTER ResolveRoleModel (co-located); pass it to Execute. Local var
            named <role>Timeout. The resolve is a single line; the swap is a single arg."
  gotcha: "generate.go is S1's file (DONE) — do NOT edit it. It is the TEMPLATE, not a target. The decompose
           message site (message.go:155) is a DIFFERENT Execute call (generateMessage, not CommitStaged)."

# MUST READ — the 4 files being edited (read each ResolveRoleModel + Execute site + import block)
- file: internal/decompose/planner.go
  why: "Line 65: _, mdl, rsn := config.ResolveRoleModel('planner', deps.Config) — add plannerTimeout after.
        Line 124: out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose) — swap
        deps.Config.Timeout → plannerTimeout. Import block (line ~16): config already imported; NO 'time'."
  gotcha: "The planner is the ONLY role whose DEFAULT changes (120s→480s). This is intended (PRD §9.15)."
- file: internal/decompose/stager.go
  why: "Line 96: ResolveRoleModel('stager') — add stagerTimeout after. Line 110: the Execute (an `if … ;
        execErr != nil` form) — swap. Stager has NO built-in → behavior-preserving."
- file: internal/decompose/message.go
  why: "Line 127: ResolveRoleModel('message') — add messageTimeout after. Line 155: Execute — swap. Message
        has NO built-in → behavior-preserving (ResolveRoleTimeout returns cfg.Timeout). Failure semantic
        is *generate.RescueError{Kind:ErrTimeout} (rescue, exit 124) — UNCHANGED."
- file: internal/decompose/arbiter.go
  why: "Line 82: ResolveRoleModel('arbiter') — add arbiterTimeout after. Line 100: Execute — swap. Arbiter
        has NO built-in → behavior-preserving. Failure semantic is graceful null (§13.6.5 'when in doubt,
        null') — UNCHANGED."

# MUST READ — the test that BREAKS (the one mandatory test fix) + its fix
- file: internal/decompose/planner_test.go
  why: "TestCallPlanner_Timeout (line 337): sets cfg.Timeout = 100ms (line 345) + stub SleepMS 2000 +
        asserts ErrPlannerFailed + DeadlineExceeded. After the wiring the planner uses 480s built-in
        (cfg.Roles is nil) → NO timeout → test FAILS. FIX: move 100ms to cfg.Roles['planner'].Timeout,
        set cfg.Timeout = 30s. See Implementation Task 5 for the exact diff."
  critical: "This is the ONLY existing test that breaks. Forgetting it = `make test` red. The fix turns
             it into the planner behavioral proof (per-role bounds Execute, not global)."

# CONTEXT — the existing _Timeout tests (the canaries that stay green + the templates for the new tests)
- file: internal/decompose/stager_test.go
  why: "TestStageConcept_Timeout (line 139): cfg.Timeout=100ms + SleepMS 2000 → ErrStagerFailed +
        DeadlineExceeded. Stays GREEN (stager has no built-in). CLONE it for TestStageConcept_PerRoleTimeout
        (flip: cfg.Timeout=30s + cfg.Roles['stager'].Timeout=100ms). Uses stagerDepsWithConfig(t, repo, m, cfg)
        + tooledStubManifest."
- file: internal/decompose/message_test.go
  why: "TestGenerateMessage_Timeout (line ~178): cfg.Timeout=100ms + SleepMS 2000 → *generate.RescueError
        {Kind:ErrTimeout}. Stays GREEN. CLONE for TestGenerateMessage_PerRoleTimeout. Uses messageDeps +
        the msgInitRepo/msgCommitRaw/msgWriteFile/msgStageFile/msgGitOut fixtures + errors.As(err, &re)."
- file: internal/decompose/arbiter_test.go
  why: "TestRunArbiter_TimeoutNull (line 209): cfg.Timeout=100ms + SleepMS 2000 → err==nil + out.Target==nil
        (graceful null). Stays GREEN. CLONE for TestRunArbiter_PerRoleTimeoutNull. Uses arbDeps + arbCommits."
- file: internal/decompose/chain_test.go
  why: "TestResolveArbiter_RescueErrorPropagation (line 426): cfg.Timeout=100ms + SleepMS 2000 → generateMessage
        (message role, via resolveArbiter) times out → *generate.RescueError. Stays GREEN (message has no
        built-in). DO NOT TOUCH — it's a canary proving behavior-preserving for the message role under chain."

# CONTEXT — the RoleConfig struct + Defaults() (so test cfg setup is correct)
- file: internal/config/config.go
  why: "RoleConfig (line 38): struct{Provider, Model, Reasoning string; Timeout time.Duration} — the test
        sets cfg.Roles['<role>'] = config.RoleConfig{Timeout: 100*time.Millisecond}. Config.Roles (line 173):
        map[string]RoleConfig. Defaults() (line 195): returns Roles==nil (not initialized) — so a test MUST
        assign cfg.Roles = map[string]config.RoleConfig{…} (you cannot index-assign into a nil map)."
  critical: "cfg.Roles is nil from Defaults() — `cfg.Roles['planner'].Timeout = X` would PANIC (nil map
             index-assign). Assign the WHOLE map: cfg.Roles = map[string]config.RoleConfig{'planner': {Timeout: X}}."

# CONTEXT — PRD §9.15 FR-R7 (per-role timeouts) + §16.1 (precedence) — the requirement
- docfile: plan/015_b461e4720495/prd_snapshot.md
  section: "§9.15 (FR-R1–R6 per-role provider/model/reasoning; FR-R7 per-role timeout) + §16.1 (resolution order)"
  why: "FR-R7: each role resolves its own timeout independently. §16.1: [role.<role>].timeout > built-in
        (planner 480s) > [defaults].timeout (120s). Confirms the planner 480s default is by design."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  planner.go    # EDIT — +plannerTimeout resolve (after :65) + Execute swap (:124)
  stager.go     # EDIT — +stagerTimeout resolve (after :96) + Execute swap (:110)
  message.go    # EDIT — +messageTimeout resolve (after :127) + Execute swap (:155)
  arbiter.go    # EDIT — +arbiterTimeout resolve (after :82) + Execute swap (:100)
  planner_test.go  # EDIT — FIX TestCallPlanner_Timeout (move 100ms global→per-role; global=30s)
  stager_test.go   # EDIT — +TestStageConcept_PerRoleTimeout
  message_test.go  # EDIT — +TestGenerateMessage_PerRoleTimeout
  arbiter_test.go  # EDIT — +TestRunArbiter_PerRoleTimeoutNull
  chain_test.go    # READ-ONLY — TestResolveArbiter_RescueErrorPropagation stays GREEN (canary; do NOT touch)
  decompose_test.go / roles_test.go  # READ-ONLY — unaffected
internal/config/
  roles.go      # READ-ONLY — ResolveRoleTimeout (LANDED, P1.M2.T1.S1) is CONSUMED; do NOT edit
  config.go     # READ-ONLY — RoleConfig{Timeout} struct + Defaults() (Roles==nil)
internal/generate/
  generate.go   # READ-ONLY — the S1 TEMPLATE (msgTimeout at :269/:340); do NOT edit (S1 DONE)
Makefile        # test=line 70 (-race); lint=line 103; coverage-gate=line 77 (decompose NOT gated; generate IS)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/decompose/
  planner.go    # MODIFIED — +1 resolve, 1 Execute arg swap
  stager.go     # MODIFIED — +1 resolve, 1 Execute arg swap
  message.go    # MODIFIED — +1 resolve, 1 Execute arg swap
  arbiter.go    # MODIFIED — +1 resolve, 1 Execute arg swap
  planner_test.go  # MODIFIED — fix TestCallPlanner_Timeout (global→per-role)
  stager_test.go   # MODIFIED — +TestStageConcept_PerRoleTimeout
  message_test.go  # MODIFIED — +TestGenerateMessage_PerRoleTimeout
  arbiter_test.go  # MODIFIED — +TestRunArbiter_PerRoleTimeoutNull
# NOTHING ELSE. No edit to internal/config/*, generate.go, multiturn.go/workdesc.go/hook, root.go, go.mod.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the planner is the ONLY role with a built-in timeout): ResolveRoleTimeout("planner", cfg)
// returns 480s when cfg.Roles has no "planner" entry (defaultRoleTimeouts["planner"]=480s) — NOT cfg.Timeout.
// So the planner wiring is a BEHAVIOR CHANGE (was deps.Config.Timeout=120s; now 480s). This is INTENTIONAL
// (PRD §9.15: planner needs more time; P1.M2.T2.S1 set global=120s so planner 480s is longer). The other 3
// roles have NO built-in → ResolveRoleTimeout returns cfg.Timeout → behavior-preserving by default.

// CRITICAL (TestCallPlanner_Timeout BREAKS and MUST be fixed): planner_test.go:337 sets cfg.Timeout=100ms
// expecting it to bound the planner. After the wiring the planner uses 480s built-in (cfg.Roles==nil there)
// → stub SleepMS 2000 → NO timeout → callPlanner returns validMultiJSON success → the test's
// `if err == nil { t.Fatal }` fires. FIX: move 100ms to cfg.Roles["planner"].Timeout, set cfg.Timeout=30s.
// This is the ONLY existing test that breaks (verified: stager/message/arbiter/chain _Timeout tests stay green).

// CRITICAL (cfg.Roles is nil from config.Defaults() — do NOT index-assign into a nil map): Defaults()
// returns Config{Roles: nil}. In a test, `cfg.Roles["planner"].Timeout = X` PANICS (nil map index-assign).
// Assign the WHOLE map: cfg.Roles = map[string]config.RoleConfig{"planner": {Timeout: 100 * time.Millisecond}}.

// GOTCHA (NO new imports): all 4 role files already import internal/config (for ResolveRoleModel).
// ResolveRoleTimeout returns a time.Duration VALUE; the local <role>Timeout var holds it and passes it to
// Execute. You do NOT need `import "time"` to hold/pass a time.Duration (only to CONSTRUCT one via
// time.Millisecond, which the SOURCE files never do — only the TEST files, which already import "time").

// GOTCHA (co-locate the resolve with ResolveRoleModel — the S1 "model/timeout twins" idiom): place
// `<role>Timeout := config.ResolveRoleTimeout("<role>", deps.Config)` on the line IMMEDIATELY AFTER the
// existing `_, mdl, rsn := config.ResolveRoleModel("<role>", deps.Config)` line. This matches S1's
// generate.go:264(model)/269(timeout) and keeps the two role-resolution calls visually paired.

// GOTCHA (each Execute is the ONLY deps.Config.Timeout reference in its file): exactly one swap per file.
// Do not search-and-replace blindly — there is exactly one provider.Execute per role function; swap only
// its timeout arg. (grep guard: `grep -c deps.Config.Timeout <file>` == 1 before, 0 after.)

// GOTCHA (distinct failure semantics are UNCHANGED — only the timeout VALUE changes): planner→ErrPlannerFailed
// (non-rescue; errors.Is + DeadlineExceeded); stager→ErrStagerFailed (non-rescue; errors.Is +
// DeadlineExceeded); message→*generate.RescueError{Kind:ErrTimeout} (rescue, exit 124; errors.As); arbiter→
// graceful nil error + Target==nil (§13.6.5 "when in doubt, null"). Do NOT touch the error-handling branches
// — only the Execute timeout argument. The new behavioral tests reuse the SAME assertion idioms.
```

## Implementation Blueprint

### Data models and structure

None NEW. Each role resolves a `time.Duration` locally and passes it to the existing `provider.Execute`.
No new types, no struct fields, no package-level state, no signature changes. The `config.RoleConfig.Timeout`
field and `config.ResolveRoleTimeout` function are LANDED (P1.M1/P1.M2).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/decompose/planner.go — resolve plannerTimeout + swap the Execute arg
  - At line 65, the existing line is:
      _, mdl, rsn := config.ResolveRoleModel("planner", deps.Config)
    ADD immediately after it:
      // FR-R7 (§9.15/§16.1): the planner resolves its OWN timeout — the built-in 480s default (longer than
      // the 120s global) because open-ended decomposition planning needs more time than message/stager/arbiter.
      // A non-zero cfg.Roles["planner"].Timeout always wins. Behavior was deps.Config.Timeout (pre-FR-R7).
      plannerTimeout := config.ResolveRoleTimeout("planner", deps.Config)
  - At line 124, change:
      out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)
    to:
      out, _, execErr := provider.Execute(ctx, *spec, plannerTimeout, deps.Verbose)
  - NO import change (config already imported; no "time" needed).
  - GOTCHA: this is the ONLY role whose DEFAULT changes (120s→480s). Intended.

Task 2: EDIT internal/decompose/stager.go — resolve stagerTimeout + swap
  - At line 96 (`_, mdl, rsn := config.ResolveRoleModel("stager", deps.Config)`), ADD after it:
      stagerTimeout := config.ResolveRoleTimeout("stager", deps.Config)
  - At line 110, change `deps.Config.Timeout` → `stagerTimeout` in:
      if _, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose); execErr != nil {
  - NO import change. Stager has NO built-in → behavior-preserving.

Task 3: EDIT internal/decompose/message.go — resolve messageTimeout + swap
  - At line 127 (`_, mdl, rsn := config.ResolveRoleModel("message", deps.Config)`), ADD after it:
      messageTimeout := config.ResolveRoleTimeout("message", deps.Config)
  - At line 155, change `deps.Config.Timeout` → `messageTimeout` in:
      out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)
  - NO import change. Message has NO built-in → behavior-preserving.

Task 4: EDIT internal/decompose/arbiter.go — resolve arbiterTimeout + swap
  - At line 82 (`_, mdl, rsn := config.ResolveRoleModel("arbiter", deps.Config)`), ADD after it:
      arbiterTimeout := config.ResolveRoleTimeout("arbiter", deps.Config)
  - At line 100, change `deps.Config.Timeout` → `arbiterTimeout` in:
      out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)
  - NO import change. Arbiter has NO built-in → behavior-preserving.

Task 5: FIX internal/decompose/planner_test.go — TestCallPlanner_Timeout (MANDATORY; it breaks otherwise)
  - The test (line 337) currently does:
      cfg := config.Defaults()
      cfg.Timeout = 100 * time.Millisecond
    After the wiring, the planner uses 480s built-in (cfg.Roles==nil) → no timeout → test FAILS.
  - REPLACE those two lines with:
      cfg := config.Defaults()
      cfg.Timeout = 30 * time.Second // GLOBAL large — would NOT time out vs 2000ms stub sleep
      cfg.Roles = map[string]config.RoleConfig{
          "planner": {Timeout: 100 * time.Millisecond}, // PER-ROLE small → times out (proves ResolveRoleTimeout bounds Execute)
      }
    (cfg.Roles MUST be assigned as a whole map — Defaults() returns Roles==nil; index-assign panics.)
  - KEEP the rest of the test unchanged: it still asserts errors.Is(err, ErrPlannerFailed) +
    errors.Is(err, context.DeadlineExceeded). The fix turns it into the planner behavioral proof.
  - Optional: add a one-line comment that the planner's default is now 480s (which is WHY the per-role
    override is needed to make this test time out at all).

Task 6: ADD internal/decompose/stager_test.go — TestStageConcept_PerRoleTimeout
  - CLONE TestStageConcept_Timeout (line 139). In the clone:
      cfg := config.Defaults()
      cfg.Timeout = 30 * time.Second
      cfg.Roles = map[string]config.RoleConfig{"stager": {Timeout: 100 * time.Millisecond}}
      m := tooledStubManifest(t, bin, stubtest.Options{SleepMS: 2000})
      deps := stagerDepsWithConfig(t, repo, m, cfg)
  - Call stageConcept; assert errors.Is(err, ErrStagerFailed) + errors.Is(err, context.DeadlineExceeded)
    (the SAME assertions as TestStageConcept_Timeout).
  - The proof: with cfg.Timeout=30s the stub (2000ms) would NOT time out; only the per-role 100ms does →
    the assertion passing proves stagerTimeout (100ms), not cfg.Timeout (30s), reached Execute.

Task 7: ADD internal/decompose/message_test.go — TestGenerateMessage_PerRoleTimeout
  - CLONE TestGenerateMessage_Timeout (line ~178). In the clone:
      cfg := config.Defaults()
      cfg.Timeout = 30 * time.Second
      cfg.Roles = map[string]config.RoleConfig{"message": {Timeout: 100 * time.Millisecond}}
      m := stubtest.Manifest(bin, stubtest.Options{SleepMS: 2000})
      deps := messageDeps(t, repo, m); deps.Config = cfg
  - Call generateMessage; assert errors.As(err, &re) + re.Kind == generate.ErrTimeout +
    errors.Is(err, generate.ErrTimeout) (the SAME assertions as TestGenerateMessage_Timeout).
  - Reuse the SAME msgInitRepo/msgCommitRaw/msgWriteFile/msgStageFile/msgGitOut fixture sequence.

Task 8: ADD internal/decompose/arbiter_test.go — TestRunArbiter_PerRoleTimeoutNull
  - CLONE TestRunArbiter_TimeoutNull (line 209). In the clone:
      cfg := config.Defaults()
      cfg.Timeout = 30 * time.Second
      cfg.Roles = map[string]config.RoleConfig{"arbiter": {Timeout: 100 * time.Millisecond}}
      m := stubtest.Manifest(bin, stubtest.Options{Out: `{"target": null}`, SleepMS: 2000})
      deps := arbDeps(t, repo, m); deps.Config = cfg
  - Call runArbiter; assert err == nil + out.Target == nil (the SAME graceful-null assertions as
    TestRunArbiter_TimeoutNull — §13.6.5 "when in doubt, null").

Task 9: VERIFY — build, vet, format, full regression, lint, grep guards
  - go build ./... ; go vet ./internal/decompose/...
  - gofmt -l <the 8 files>   # empty
  - go test ./internal/decompose/ -run 'Timeout' -race -v   # fixed planner + 3 new + 4 existing canaries
  - go test ./internal/decompose/ -race                     # full decompose regression
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (clone of S1's generate.go:264/269/340 — the "model/timeout twins" idiom): resolve the timeout
// RIGHT AFTER ResolveRoleModel (co-located), then pass it to Execute. Shown for the planner; the other 3
// roles are identical with their own role string + var name.
func callPlanner(ctx context.Context, deps Deps, …) (prompt.PlannerOutput, error) {
	_, mdl, rsn := config.ResolveRoleModel("planner", deps.Config)            // line 65 (pre-existing)
	plannerTimeout := config.ResolveRoleTimeout("planner", deps.Config)       // ADD — the FR-R7 twin
	…
	for attempt := 0; attempt < maxAttempts; attempt++ {
		…
		out, _, execErr := provider.Execute(ctx, *spec, plannerTimeout, deps.Verbose)   // line 124 — SWAP deps.Config.Timeout → plannerTimeout
		…
	}
	…
}

// PATTERN (the behavioral-proof test — clone of S1's TestCommitStaged_MessageRoleTimeout): set the GLOBAL
// large (would NOT time out vs the stub sleep) + the PER-ROLE small (times out) → the role's distinct
// failure semantic firing proves the per-role timeout, not the global, reached Execute.
func TestCallPlanner_PerRoleTimeout(t *testing.T) {   // (or fix TestCallPlanner_Timeout in place — Task 5)
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.txt", "new content")

	cfg := config.Defaults()
	cfg.Timeout = 30 * time.Second // LARGE — 2000ms stub sleep would NOT time out here
	cfg.Roles = map[string]config.RoleConfig{
		"planner": {Timeout: 100 * time.Millisecond}, // SMALL → times out (proves ResolveRoleTimeout bounds Execute)
	}
	m := stubtest.Manifest(bin, stubtest.Options{Out: validMultiJSON, SleepMS: 2000})
	deps := plannerDeps(t, repo, m)
	deps.Config = cfg
	baseTree, tStart := freezeForPlanner(t, repo, false)

	_, err := callPlanner(context.Background(), deps, 0, false, baseTree, tStart)
	if err == nil {
		t.Fatal("expected error on per-role timeout, got nil")
	}
	if !errors.Is(err, ErrPlannerFailed) {
		t.Errorf("errors.Is(err, ErrPlannerFailed) = false, error = %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false, error = %v", err)
	}
}
```

### Integration Points

```yaml
CONSUMPTION (internal/decompose — the 4 Execute sites):
  - planner.go:124  provider.Execute(…, plannerTimeout, …)   where plannerTimeout = ResolveRoleTimeout("planner", deps.Config)
  - stager.go:110   provider.Execute(…, stagerTimeout,  …)   where stagerTimeout  = ResolveRoleTimeout("stager",  deps.Config)
  - message.go:155  provider.Execute(…, messageTimeout, …)   where messageTimeout = ResolveRoleTimeout("message", deps.Config)
  - arbiter.go:100  provider.Execute(…, arbiterTimeout, …)   where arbiterTimeout = ResolveRoleTimeout("arbiter", deps.Config)

IMPORTS:
  - NO change to any of the 4 source files' import blocks (config already imported; time.Duration passthrough).
  - NO change to any of the 4 test files' import blocks (time + config + errors already imported).

NO database / migration / routes / new types / new flag / config-struct change / signature change / docs.
  - The single-commit path (generate.go) is S1 (DONE) — NOT here.
  - The multi-turn/workdesc/hook message sites are S2 (parallel, different files) — NOT here.
  - internal/config/* (ResolveRoleTimeout, RoleConfig, defaultRoleTimeouts) is LANDED — NOT here.
  - Docs (README/configuration) are P1.M4.T2.S1 — NOT here (contract: "DOCS: none — internal wiring").
  - ResolveRoleTimeout's OWN unit tests + config-loading tests are P1.M4.T1.S1 — NOT here.

SCOPE FENCES:
  - Touches ONLY the 4 internal/decompose role .go files + their 4 _test.go files.
  - Does NOT edit internal/config/*, generate.go, multiturn.go, workdesc.go, internal/hook/exec.go,
    root.go, cmd/*, go.mod, or any PRD/task file.
  - Adds NO flag, NO exported type, NO third-party dependency, NO signature change.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (the 4 swaps link cleanly; no import cycle / unused-import error).
go build ./...
# Expected: clean. If it fails on an unused import, you accidentally added `import "time"` to a source
#           file — REMOVE it (the time.Duration passthrough needs no time import).

# Vet.
go vet ./internal/decompose/...
# Expected: clean.

# Format.
gofmt -l internal/decompose/planner.go internal/decompose/stager.go internal/decompose/message.go internal/decompose/arbiter.go \
       internal/decompose/planner_test.go internal/decompose/stager_test.go internal/decompose/message_test.go internal/decompose/arbiter_test.go
# Expected: empty. If listed: gofmt -w <those files>

# Lint.
make lint   # golangci-lint (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. (The new <role>Timeout vars are all used by their Execute call.)

# Scope guard: ONLY the 8 decompose files changed.
git status --porcelain
# Expected: the 4 role .go + 4 _test.go under internal/decompose/. ZERO changes to internal/config/,
#           generate.go, multiturn.go/workdesc.go/hook, root.go, go.mod, or any PRD/task file.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The timeout-related tests (the fixed planner test + 3 new + 4 existing canaries).
go test ./internal/decompose/ -run 'Timeout' -race -v
# Expected: ALL PASS.
#   - TestCallPlanner_Timeout (FIXED): ErrPlannerFailed + DeadlineExceeded (now via per-role 100ms).
#   - TestStageConcept_PerRoleTimeout (NEW): ErrStagerFailed + DeadlineExceeded (via per-role 100ms).
#   - TestGenerateMessage_PerRoleTimeout (NEW): *generate.RescueError{ErrTimeout} (via per-role 100ms).
#   - TestRunArbiter_PerRoleTimeoutNull (NEW): err==nil + Target==nil (via per-role 100ms).
#   - TestStageConcept_Timeout / TestGenerateMessage_Timeout / TestRunArbiter_TimeoutNull (canaries):
#     GREEN unchanged (cfg.Timeout small — behavior-preserving for no-built-in roles).
#   - TestResolveArbiter_RescueErrorPropagation (chain canary): GREEN (message role, no built-in).

# Full decompose regression (chain/roles/decompose + all the above).
go test ./internal/decompose/ -race
# Expected: green. The wiring is behavior-preserving for 3 of 4 roles; the planner's 480s default only
#           affects the (now-fixed) TestCallPlanner_Timeout.

# Full race suite.
make test
# Expected: green. (internal/generate's TestCommitStaged_MessageRoleTimeout — S1 — is unrelated and stays green.)
```

### Level 3: Integration Testing (System Validation)

```bash
# This item is internal wiring with no user-visible CLI surface change beyond the planner's 480s default.
# The decompose e2e (internal/e2e) exercises the full pipeline; it does NOT pin per-role timeouts via
# cfg.Timeout (verified — no cfg.Timeout pins in internal/e2e for these roles), so it stays green.
go test ./internal/e2e/ -race
# Expected: green (the planner's 480s default is LONGER than the prior 120s, so no e2e planner run that
#           previously succeeded will now time out; if anything, fewer timeouts).

# Manual sanity (optional): build + a decompose run with a per-role planner override.
make build
# (Set [role.planner].timeout in a test config, run stagecoach on an un-staged dirty tree, observe the
#  planner bounded at the override. The unit tests are the real proof; this is a smoke check.)
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: each role file resolves its OWN timeout right after ResolveRoleModel.
grep -n 'ResolveRoleModel("planner"\|ResolveRoleTimeout("planner"' internal/decompose/planner.go   # both present
grep -n 'ResolveRoleModel("stager"\|ResolveRoleTimeout("stager"'  internal/decompose/stager.go     # both present
grep -n 'ResolveRoleModel("message"\|ResolveRoleTimeout("message"' internal/decompose/message.go   # both present
grep -n 'ResolveRoleModel("arbiter"\|ResolveRoleTimeout("arbiter"' internal/decompose/arbiter.go   # both present
# Expect: 2 hits each (the model resolve + the timeout resolve).

# Guard 2: NO deps.Config.Timeout remains in any role file's Execute (the swap is complete).
grep -n 'deps.Config.Timeout' internal/decompose/planner.go internal/decompose/stager.go internal/decompose/message.go internal/decompose/arbiter.go
# Expect: ZERO hits (each Execute now uses <role>Timeout). (Before the change there was exactly 1 per file.)

# Guard 3: each Execute uses its <role>Timeout.
grep -n 'provider.Execute(ctx, \*spec, plannerTimeout' internal/decompose/planner.go   # 1 hit
grep -n 'provider.Execute(ctx, \*spec, stagerTimeout'  internal/decompose/stager.go     # 1 hit
grep -n 'provider.Execute(ctx, \*spec, messageTimeout' internal/decompose/message.go   # 1 hit
grep -n 'provider.Execute(ctx, \*spec, arbiterTimeout' internal/decompose/arbiter.go   # 1 hit

# Guard 4: NO new import in any source file (config was already imported; no "time" added).
for f in internal/decompose/planner.go internal/decompose/stager.go internal/decompose/message.go internal/decompose/arbiter.go; do
  echo "== $f =="; grep -n '"time"' "$f" || echo "  (no time import — correct)";
done
# Expect: "(no time import — correct)" for all 4.

# Guard 5: internal/config/ is UNCHANGED (ResolveRoleTimeout is consumed, not modified).
git diff --name-only | grep -q '^internal/config/' && echo "FAIL: config edited" || echo "OK: config untouched"

# Guard 6: generate.go (S1's file) is UNCHANGED.
git diff --name-only | grep -q '^internal/generate/generate\.go$' && echo "FAIL: generate.go edited" || echo "OK: generate.go untouched"

# Guard 7: TestCallPlanner_Timeout now uses the per-role field (the mandatory fix).
grep -A6 'func TestCallPlanner_Timeout' internal/decompose/planner_test.go | grep 'cfg.Roles\|"planner"'
# Expect: a hit (cfg.Roles = map[string]config.RoleConfig{"planner": …}).

# Guard 8: scope — only the 8 decompose files.
git status --porcelain
# Expect: 4 internal/decompose/*.go + 4 internal/decompose/*_test.go. NOTHING else.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (no unused-import error from a stray `import "time"`)
- [ ] `go vet ./internal/decompose/...` clean
- [ ] `gofmt -l` empty on the 8 files
- [ ] `make lint` zero errors (all `<role>Timeout` vars used by their Execute)
- [ ] `go test ./internal/decompose/ -race` green (fixed planner test + 3 new + all canaries)
- [ ] `make test` (full race suite) green

### Feature Validation
- [ ] Each of the 4 decompose Execute sites (planner:124, stager:110, message:155, arbiter:100) reads its
      `<role>Timeout` (`config.ResolveRoleTimeout`), not `deps.Config.Timeout` (grep guards 1–3)
- [ ] The planner defaults to 480s built-in (behavior change, intended); stager/message/arbiter default to
      cfg.Timeout (behavior-preserving)
- [ ] `TestCallPlanner_Timeout` FIXED (per-role 100ms + global 30s) — still asserts ErrPlannerFailed +
      DeadlineExceeded (grep guard 7)
- [ ] 3 new behavioral-proof tests prove the per-role override bounds Execute for stager/message/arbiter
- [ ] Each role's failure semantic is UNCHANGED (planner/stager non-rescue; message rescue ErrTimeout;
      arbiter graceful null)
- [ ] The 4 existing canary tests (stager/message/arbiter/chain _Timeout) stay GREEN unchanged

### Scope-Boundary Validation
- [ ] `git status` shows ONLY the 8 files under `internal/decompose/` (4 role .go + 4 _test.go)
- [ ] NO edit to internal/config/*, generate.go, multiturn.go, workdesc.go, internal/hook/exec.go, root.go,
      cmd/*, go.mod, or any PRD/task file (grep guards 5–6)
- [ ] NO new flag, NO new exported type, NO new import, NO signature change, NO third-party dependency
- [ ] NO single-commit-path change (S1 DONE), NO multiturn/workdesc/hook change (S2, parallel), NO docs
      (P1.M4.T2), NO ResolveRoleTimeout-own tests (P1.M4.T1)

### Code Quality & Docs
- [ ] Each `<role>Timeout` resolve is co-located with its `ResolveRoleModel` (the S1 "model/timeout twins")
- [ ] The planner resolve carries a brief comment noting the 480s built-in default (why it differs from the
      other 3 roles)
- [ ] Tests clone the existing `_Timeout` harness (stubtest + per-file *Deps helpers); no new fixtures
- [ ] Contract honored: "DOCS: none — internal wiring" (no README/docs edit)

---

## Anti-Patterns to Avoid

- ❌ Don't forget `TestCallPlanner_Timeout` BREAKS. The planner is the ONLY role with a built-in (480s), so
  after the swap it no longer reads `cfg.Timeout`. That test sets `cfg.Timeout=100ms` and expects a timeout
  — it will FAIL (planner now 480s, stub sleeps 2000ms → no timeout → success → `t.Fatal`). The fix (move
  100ms to `cfg.Roles["planner"].Timeout`) is MANDATORY. This is the single most likely way to leave
  `make test` red.
- ❌ Don't index-assign into a nil map. `config.Defaults()` returns `Config{Roles: nil}`. In a test,
  `cfg.Roles["planner"].Timeout = X` PANICS. Assign the whole map: `cfg.Roles = map[string]config.RoleConfig{"planner": {Timeout: X}}`.
- ❌ Don't add `import "time"` to a source file. The 4 role files hold a `time.Duration` VALUE (returned by
  ResolveRoleTimeout) and pass it to Execute — no `time.Second`/`time.Millisecond` construction happens in
  source. Only the TEST files construct durations (and they already import `"time"`). A stray source
  `import "time"` is an unused-import compile error (grep guard 4).
- ❌ Don't touch the error-handling branches. The contract is explicit: "only the timeout VALUE changes,
  not the error handling." Each role's distinct failure semantic (ErrPlannerFailed / ErrStagerFailed /
  *generate.RescueError{ErrTimeout} / graceful null) stays EXACTLY as-is. Swap the ONE Execute timeout
  argument; leave the `if execErr != nil { … }` / `errors.Is(DeadlineExceeded)` branches untouched.
- ❌ Don't edit internal/config/roles.go or generate.go. `ResolveRoleTimeout` + `defaultRoleTimeouts` are
  LANDED (P1.M2.T1.S1) — this item CONSUMES them. generate.go's `msgTimeout` is S1 (DONE) — it is the
  TEMPLATE to clone, not a target (grep guards 5–6).
- ❌ Don't conflate this with S2 (parallel). S2 wires the message role at multiturn.go/workdesc.go/hook
  (single-commit-path transports). This item wires ALL 4 roles in `internal/decompose/` (the decompose
  path). Different files entirely — no overlap, no shared edits.
- ❌ Don't search-and-replace `deps.Config.Timeout` blindly. Each role file has EXACTLY ONE such reference
  (its single Execute). Swap only that one. Run `grep -c deps.Config.Timeout <file>` → must be 1 before, 0
  after (grep guard 2). There are no other `deps.Config.Timeout` references to "find."
- ❌ Don't add docs. The contract says "DOCS: none — internal wiring." The README/configuration-docs sync is
  P1.M4.T2.S1 (a separate, explicitly-planned task). Adding doc edits here is out of scope and risks
  conflicting with that task.
- ❌ Don't defer the new tests to P1.M4.T1. P1.M4.T1.S1 is for `ResolveRoleTimeout` ITSELF + config-loading +
  the default-change unit tests (in internal/config). The CONSUMPTION behavioral proofs (proving each
  Execute site uses the per-role timeout) belong with the consumption subtask — the S1 precedent
  (TestCommitStaged_MessageRoleTimeout, LANDED) and S2 (adding 3 such tests) establish this. Each Execute
  site gets its own proof.
- ❌ Don't treat the planner 480s default as a bug to "fix back" to 120s. It is DESIGNED (PRD §9.15/§16.1;
  P1.M2.T2.S1 set global=120s precisely so the planner 480s built-in would be longer). If a test pins the
  OLD 120s planner behavior, fix the TEST (move the small timeout to the per-role field), do not weaken the
  wiring. The wiring's job is to make `ResolveRoleTimeout` the source of truth at each Execute site.
