---
name: "P4.M1.T1.S2 — End-to-end scenario harness (PRD §20.5): a //go:build e2e throwaway-repo regression
  harness that drives the compiled stagecoach binary against a fresh git repo per scenario, plus the
  concurrent-change-exclusion assertion extended into the v2 stub decompose suite"
description: |

  CREATE `internal/e2e/harness_test.go` + `internal/e2e/scenarios_test.go` (both `//go:build e2e`) — a
  throwaway-repo harness that, per scenario, `git init`s a temp repo, seeds it, runs the COMPILED
  `stagecoach` binary as a subprocess (real agent when STAGECOACH_RUN_REAL=1, else a `stub` provider wired
  via `--config` + `cmd/stubagent`), and asserts the resulting history/exit-code. EDIT
  `internal/decompose/decompose_test.go` to add the deterministic in-process concurrent-change-exclusion
  happy path (FR-M1b/M1c) using the existing stager seam.

  CONTRACT (P4.M1.T1.S2, verbatim from the work item):
    1. RESEARCH NOTE: PRD §20.5. The concurrency/routing invariants are easy to specify, easy to regress,
       and — per the §G bug history — easy to break SILENTLY (unit tests with stub agents cannot reach
       them). internal/git tests use real git in a temp repo; internal/stubtest provides a stub agent
       (cmd/stubagent). The v2 decompose suite is internal/decompose/decompose_test.go.
    2. INPUT: P3.M2.T1.S2 (freeze + short-circuit), P1.M3.T1.S2 (migration), P2.M1.T1.S2 (qwen-code). The
       internal/stubtest stub agent + the v2 decompose stub suite.
    3. LOGIC: Build a //go:build e2e throwaway-repo harness: per scenario, `git init` a temp repo → seed
       known content → run stagecoach (real agent where STAGECOACH_RUN_REAL=1, else the stub) → assert the
       resulting history. MUST-COVER set: (1) nothing staged + N unrelated files → N commits (auto AND
       --commits N); (2) exactly one file → single commit, NO planner call (FR-M2b); (3) a file
       created/modified by a concurrent process mid-run → excluded from EVERY commit, left in the working
       tree (FR-M1b/M1c); (4) a model pinned on a multi-backend agent with no inference-provider prefix →
       HARD ERROR, not empty output (FR-R5b); (5) arbiter reconciliation (new commit / tip amend /
       mid-chain rebuild); (6) rescue mid-loop; (7) CAS abort (HEAD moved concurrently). ALSO extend the
       v2 stub decompose suite (internal/decompose/decompose_test.go) with the concurrent-change-exclusion
       assertion (write a sentinel file mid-run, assert it lands in no commit and remains in the working
       tree).
    4. OUTPUT: a regression net for the behaviors that only manifest against a real repo; every bug found
       in the wild becomes a scenario here.
    5. DOCS: none — test-only.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/decompose/*.go (NON-test) — CONSUMED READ-ONLY. Decompose/ResolveRoles/Deps/FreezeWorkingTree/
      runOneFileShortcut/ErrFreezeViolation are the SYSTEM UNDER TEST. The parallel sibling P3.M2.T1.S2
      owns decompose.go's one-file short-circuit (already shipped); P4.M1.T1.S1 owns ui/* + cmd progress
      label. NEITHER touches the e2e package or decompose_test.go (no file overlap).
    - internal/git/git.go, internal/config/*, internal/provider/*, internal/stubtest/*, cmd/stubagent —
      CONSUMED. (If a tiny stubagent enhancement were needed it would be in-scope, but this PRP achieves
      all stub-reachable scenarios WITHOUT modifying stubagent — see Gotchas.)
    - PRD.md, tasks.json, prd_snapshot.md, .gitignore — NEVER modify.

  DELIVERABLES (2 NEW e2e files + 1 EDIT; 0 production-code changes):
    CREATE internal/e2e/harness_test.go      — //go:build e2e; shared helpers (binary builder, repo
                                               factory, git assertion helpers, config writer, stub runner).
    CREATE internal/e2e/scenarios_test.go    — //go:build e2e; the 7 §20.5 scenarios as t.Run subtests,
                                               STAGECOACH_RUN_REAL-gated for stager-dependent ones.
    EDIT   internal/decompose/decompose_test.go — +TestDecompose_ConcurrentChangeExclusion (the
                                               deterministic in-process FR-M1b/M1c happy path via the
                                               stager seam; the success-path sibling of the existing
                                               TestDecompose_StagerFreezeViolation failure path).

  SUCCESS: `go build ./...` + `go vet ./...` green (e2e package excluded without -tags); `go test -race
  ./...` (make test) green; the new decompose test passes under the normal suite;
  `golangci-lint run` clean; `gofmt -l internal/` empty; `go test -tags e2e ./internal/e2e/ -v` runs the
  stub-reachable scenarios (2,3,4,7 + single-rescue 6) green and SKIPS stager-only ones (1,5,loop-6) with a
  clear message; `STAGECOACH_RUN_REAL=1 go test -tags e2e ./internal/e2e/ -v` runs all 7 against a real
  agent; `make coverage-gate` unaffected (test-only changes). go.mod/go.sum UNCHANGED.

---

## Goal

**Feature Goal**: Implement PRD **§20.5** — a regression net for the concurrency/routing invariants that
only manifest against a real git repo (and, ideally, a real agent): the behaviors that let the
planner-empty-output (FR-R5b) and concurrent-file (FR-M1b/M1c) bugs ship silently. Concretely: (A) a
`//go:build e2e` throwaway-repo harness that drives the compiled `stagecoach` binary as a **subprocess** in
a fresh `git init` temp repo per scenario, asserting on the resulting history + exit code (real agent when
`STAGECOACH_RUN_REAL=1`, else a `stub` provider wired via `--config` + `cmd/stubagent`); and (B) the
deterministic, CI-able concurrent-change-exclusion happy path added to the in-process v2 stub decompose
suite.

**Deliverable** (2 NEW e2e files + 1 EDIT; 0 production changes):
1. `internal/e2e/harness_test.go` (`//go:build e2e`) — shared helpers: `buildStagecoach(t)`,
   `buildStub(t)`, `newRepo(t)`, seed/git helpers, `writeStubConfig(t,...)`, `runStagecoach(t,...)`,
   `waitForMarker(t,...)`, `skipIfNotReal(t)`.
2. `internal/e2e/scenarios_test.go` (`//go:build e2e`) — 7 `t.Run` scenarios covering the §20.5 must-cover
   set, each routed to stub-reachable and/or real-only execution as appropriate.
3. `internal/decompose/decompose_test.go` — `+TestDecompose_ConcurrentChangeExclusion` (in-process
   multi-concept FR-M1b/M1c happy path via the stager seam).

**Success Definition**:
- **E2E harness (subprocess, stub mode `go test -tags e2e ./internal/e2e/`)**:
  - **S2 (one-file, no planner):** a repo with exactly one un-staged change → `stagecoach` (auto-mode
    decompose) produces exactly ONE commit whose tree contains that file; a CANARY planner provider
    (whose command touches a marker file if ever invoked) leaves the marker ABSENT → the planner was never
    called (FR-M2b).
  - **S3 (concurrent file excluded):** one-file change; the test waits for the stub's readiness
    `STAGECOACH_STUB_MARKER`, THEN writes a `sentinel.txt`; the one commit does NOT contain `sentinel.txt`,
    and `git status` still shows `?? sentinel.txt` post-run (it remains in the working tree) (FR-M1b/M1c).
  - **S4 (FR-R5b hard error):** a custom `[provider.testmulti]` (multi-backend: `provider_flag` set) with
    `--model bare` (no `/`) → non-zero exit (1) and stderr contains `must be inference/model` — a HARD
    ERROR, never silent empty output.
  - **S6 (rescue, single path):** empty `STAGECOACH_STUB_OUT` → unparseable output → exit code 3 (Rescue)
    + the rescue message on stderr.
  - **S7 (CAS abort):** the stub writes its MARKER then SLEEPs; the test moves HEAD (`git commit`) while
    generation is in-flight → exit 1 (CAS) and `git rev-parse HEAD` is unchanged (still the test's
    concurrent commit).
  - **S1 / S5 / loop-S6:** `t.Skip` with a clear message (need a tooled stager; set
    `STAGECOACH_RUN_REAL=1` or see `internal/decompose/decompose_test.go` for the in-process stub coverage).
- **E2E harness (real mode `STAGECOACH_RUN_REAL=1`):** all 7 scenarios run against a real configured agent
  (provider/model via `STAGECOACH_E2E_PROVIDER`/`STAGECOACH_E2E_MODEL`, default `pi`), asserting the
  general invariants (commit count, clean tree, resolvable SHAs) where exact agent behavior is
  nondeterministic.
- **v2 suite extension:** `TestDecompose_ConcurrentChangeExclusion` — a 2-concept in-process run whose
  stager seam writes an UNSTAGED `sentinel.txt` during concept 0; both commits succeed; `sentinel.txt` is
  in NEITHER commit's `DiffTree` and REMAINS in the working tree (`?? sentinel.txt`) post-run. Distinct
  from `TestDecompose_StagerFreezeViolation` (which STAGES the sentinel → `ErrFreezeViolation`).
- **No regressions:** `make test` (`go test -race ./...`) green; `go build ./...` green (e2e excluded
  without `-tags`); `make lint` clean; `gofmt -l internal/` empty; `make coverage-gate` unaffected;
  go.mod/go.sum UNCHANGED.

## User Persona

**Target User**: the Stagecoach maintainer (the developer shipping v2). This is a TEST-ONLY internal
deliverable — no end-user surface.

**Use Case**: before every release (and after touching routing/freeze/CAS code), the maintainer runs the
e2e harness (`go test -tags e2e ./internal/e2e/`) to confirm the concurrency/routing invariants hold
against a real repo+binary — the gap that let the planner-empty-output and concurrent-file bugs ship.
"Every bug found in the wild becomes a scenario here."

**User Journey**: (1) `make test` runs the normal suite (incl. the new in-process concurrent-exclusion
test); (2) `go test -tags e2e ./internal/e2e/ -v` runs the stub-mode subprocess scenarios; (3) before
release, `STAGECOACH_RUN_REAL=1 go test -tags e2e ./internal/e2e/ -v -timeout 30m` exercises a real agent.

**Pain Points Addressed**: unit tests with stub agents CANNOT reach the real-binary routing/CAS/freeze
behaviors; this harness is the dedicated regression net for them.

## Why

- **Business value**: §20.5 is "strongly encouraged" and closes the highest-leverage QA gap in v2 — the
  silent regressions that only appear against a real repo. Directly defends G10 (safety/idempotent index)
  and the FR-M1b/M1c/M2b/R5b invariants that prior field discoveries broke.
- **Integration with existing features**: consumes the freeze (P3.M1/M2), the one-file short-circuit
  (P3.M2.T1.S2), the v3 config/model-prefix (P1.M3/P1.M2), and the qwen-code provider (P2.M1) — asserting
  them end-to-end rather than in isolation. Reuses `internal/stubtest` (`cmd/stubagent`) and the v2 stub
  suite's helper idioms.
- **Problems this solves and for whom**: gives the maintainer a fast (stub) + thorough (real) regression
  net for the exact behaviors that broke in the field; turns each future bug into a permanent scenario.

## What

**Maintainer-visible behavior** (test output):
- `go test -tags e2e ./internal/e2e/ -v`: one `--- PASS` per stub-reachable scenario (S2,S3,S4,S6-single,S7)
  and one `--- SKIP` per stager-dependent scenario (S1,S5,loop-S6) explaining the two ways to run them.
- `go test -race ./internal/decompose/ -run TestDecompose_ConcurrentChangeExclusion -v`: PASS.
- `make test` / `make lint` / `go build ./...`: green (unaffected — e2e is build-tagged test-only).

**Technical requirements**: two `//go:build e2e` test files in a NEW `internal/e2e` package (subprocess:
build `./cmd/stagecoach` + `./cmd/stubagent` once, run the binary with `--config`, assert via raw `git`);
one added in-process test in `internal/decompose`. No new packages beyond `internal/e2e`; no config/provider
schema changes; no go.mod changes; no production-code edits.

### Success Criteria

- [ ] `internal/e2e/{harness,scenarios}_test.go` exist, both `//go:build e2e`, package `e2e`, lint+vet+fmt clean.
- [ ] `go test -tags e2e ./internal/e2e/` runs S2/S3/S4/S6-single/S7 (stub) green and SKIPS S1/S5/loop-S6.
- [ ] S2 asserts one commit + planner canary marker ABSENT (FR-M2b).
- [ ] S3 asserts sentinel excluded from the commit + remains in the working tree (FR-M1b/M1c).
- [ ] S4 asserts FR-R5b hard error (exit 1 + `must be inference/model`).
- [ ] S7 asserts CAS (exit 1 + HEAD unchanged) via the stub MARKER+SLEEP.
- [ ] `STAGECOACH_RUN_REAL=1 go test -tags e2e ./internal/e2e/` runs all 7 against a real agent.
- [ ] `TestDecompose_ConcurrentChangeExclusion` added to `decompose_test.go`; passes under `go test -race`.
- [ ] `go build ./...` + `go vet ./...` green; `make test` green; `make lint` clean; `gofmt -l internal/` empty;
      `make coverage-gate` unaffected; go.mod/go.sum UNCHANGED.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed?_ — YES. The exact build
commands (`go build -o <tmp> ./cmd/stagecoach`, reuse `stubtest.Build`), the exact `--config` flag +
`[provider.<name>]` TOML shape + field names, the exact env-flow fact (`Render` = `os.Environ()+manifest.Env`,
so per-scenario stub knobs go on the stagecoach PROCESS env), the exact routing predicate (`shouldDecompose`:
un-staged dirty tree + `AutoStageAll` default-true → decompose), the exact freeze/one-file-shortcut control
flow (where to inject the sentinel post-freeze), the exact stub MARKER semantics (purpose-built for
deterministic concurrent races), the exact FR-R5b error substring + exit codes, the exact assertion
primitives (`git diff-tree`, `git status --porcelain`, `git rev-parse`), the exact existing helpers to reuse
(`dcmStagerSeam`, `dcmAllRoles`, `tooledStubManifest`, `dcmLogCount`), and the exact stager-wall reason each
scenario is stub-reachable vs real-only — all named with file + symbol + line, with the non-obvious
gotchas explained.

### Documentation & References

```yaml
# MUST READ
- url: PRD.md §20.5 (End-to-end scenario harness) + §20.1 layer 3/4 + §20.2 invariants
  why: "Authoritative must-cover set + the 'stub where feasible, real otherwise' dual-mode contract +
        the Start-of-run-freeze property (write a sentinel mid-run → in no commit + remains in the tree)."
  critical: "The harness drives the COMPILED binary as a SUBPROCESS in a fresh git init per scenario —
        NOT an in-process library call (that is §20.1 layer 3, already covered by decompose_test.go)."

# CODEBASE FILES — the system under test + the patterns to mirror (all verified; paths exact)
- file: internal/stubtest/stubtest.go   # CONSUMED — Build/Manifest/NewScript/Env/Options
  why: "stubtest.Build(t) compiles ./cmd/stubagent ONCE (cached) → path. stubtest.Manifest(bin,Options)
        builds a stub manifest. Options{Out,Exit,SleepMS,Stderr,Script,Counter}. The harness REUSES
        stubtest.Build for the stub binary; it does NOT need stubtest.Manifest (the binary loads the stub
        via a config [provider.stub], not a Go manifest) — but the OPTIONS/knob names are the contract."
  pattern: "Mirror stubtest.Build's sync.Once + exec.LookPath('go') + t.TempDir + import-path build for
        buildStagecoach(t) (build ./cmd/stagecoach). For the stub, CALL stubtest.Build(t) directly."

- file: cmd/stubagent/main.go   # CONSUMED — the stub binary's behavior contract (NO edit needed)
  why: "Knobs (all STAGECOACH_STUB_*): OUT (single response), SCRIPT+COUNTER (call-varying), EXIT, SLEEP_MS,
        STDERR, MARKER. MARKER semantics (verbatim): 'tells the test harness stdin drained + generation is
        in-flight; must happen BEFORE the sleep so the test can race HEAD movement DETERMINISTICALLY.' This
        is the primitive for S3 (concurrent sentinel) and S7 (CAS race)."
  gotcha: "stubagent is stdlib-only and CANNOT run `git add` → it cannot serve the tooled STAGER via the
        binary. This is WHY scenarios needing a stager (S1/S5/loop-S6) are REAL-only in the e2e harness.
        Do NOT try to make the stub stage — its deterministic multi-concept coverage lives in-process."

- file: internal/provider/executor.go   # CONSUMED (env-flow fact)
  why: "L58-59: `if len(spec.Env) > 0 { cmd.Env = spec.Env }` and the doc (L24): 'Render populates
        os.Environ()+manifest env; nil Env ⇒ inherit parent env'. So: stub knobs set on the STAGECOACH
        PROCESS ENV (the test's exec.Command Env) inherit to the stub subprocess via Render's os.Environ()."
  pattern: "ONE base [provider.stub] config (command/output/etc., NO env knobs); set per-scenario
        STAGECOACH_STUB_* knobs on the stagecoach subprocess Env (= os.Environ() + knobs)."

- file: internal/cmd/root.go   # CONSUMED — flag surface
  why: "--config (flagConfig→ConfigPathOverride, L88/L106); --provider/--model (L104-105);
        --commits N (L115); --single/--no-decompose (L117-120); --all/-a, --no-auto-stage, --dry-run
        (L111-113); per-role --<role>-provider/--<role>-model (L123-129). The harness passes these as argv."

- file: internal/cmd/default_action.go   # CONSUMED — routing
  why: "shouldDecompose(cfg,dryRun,noAutoStage) (L253): true iff (nothing staged) + cfg.AutoStageAll + NOT
        cfg.Single + cfg.Commits!=1 + NOT --dry-run. runDefault (L33): --all→AddAll first; else
        HasStagedChanges; if !hasStaged && shouldDecompose → runDecompose (planner gets working-tree diff)."
  pattern: "To trigger DECOMPOSE in a scenario: leave files UN-staged in the working tree (do NOT git add)
        + default config (AutoStageAll=true). To force single-commit: --single. To force N: --commits N."

- file: internal/decompose/decompose.go   # CONSUMED (SUT) — freeze + shortcuts
  why: "Decompose (L140): mode-routing → RevParseHEAD/baseTree → FreezeWorkingTree(baseTree)→T_start (L~220,
        BEFORE the planner) → [one-file short-circuit L208-216: DiffTreeNames(baseTree,tStart)==1 path ⇒
        runOneFileShortcut, planner BYPASSED] → callPlanner → runLoop → arbiter. runOneFileShortcut (L280):
        generateMessage(baseTree,tStart) + publishCommit(tStart) — commits the FROZEN tree directly, NO
        stager, NO planner. The escape-hatch (--single/--commits 1) does NOT freeze (v1 behavior)."
  critical: "S2 (one-file) and S3 (concurrent) go through runOneFileShortcut: it FREEZES, calls the MESSAGE
        agent (stub: SLEEP+MARKER), then commits tStart. The sentinel written after the MARKER is
        post-freeze ⇒ excluded. This path is fully stub-reachable."

- file: internal/git/git.go   # CONSUMED — assertion primitives
  why: "DiffTree(ctx,sha,isRoot) []FileChange (L483); FileChange{Status,SrcPath,Path} (L18). CommitCount
        (L814); LogRange(baseSHA) (L836); StatusPorcelain (L1107) — sentinel untracked = '?? sentinel.txt';
        RevParseHEAD (L362); FreezeWorkingTree (L1223); DiffTreeNames (L1243)."
  pattern: "In the SUBPROCESS harness prefer RAW `git -C <repo> …` (mirror decompose_test.go's dcmRunGit)
        — it needs no Go import and is clearest for history assertions: `git rev-list --count HEAD`,
        `git diff-tree --no-commit-id --name-only -r <sha>`, `git status --porcelain`, `git rev-parse HEAD`."

- file: internal/decompose/decompose_test.go   # EDIT TARGET + pattern source (v2 stub suite)
  why: "The deterministic in-process stager-seam idiom. dcmStagerSeam(repo, conceptFiles map[string][]string)
        (L183) does real `git add` of named paths — the NEW test injects the sentinel here (UNSTAGED).
        dcmAllRoles/dcmPlannerManifest/dcmMessageScriptManifest/tooledStubManifest build the 4 roles.
        TestDecompose_AutoMultiCommit_HappyPath (L394) is the 2-3-concept template. TestDecompose_StagerFreezeViolation
        (L615) is the FAILURE-path sibling (stages a post-freeze sentinel → ErrFreezeViolation) — the NEW
        test is its SUCCESS-path counterpart (writes the sentinel UNSTAGED ⇒ excluded, run succeeds)."
  pattern: "Copy TestDecompose_AutoMultiCommit_HappyPath's structure: initRepo+commitRaw seed; write a.txt
        + b.txt (un-staged); planner JSON with 2 concepts {a.txt},{b.txt}; message script with 2 msgs;
        dcmAllRoles(tooled stager seam); a CUSTOM seam (not dcmStagerSeam) that, for concept 0, stages a.txt
        AND writes sentinel.txt UNSTAGED (os.WriteFile, NO git add). Run Decompose; assert 2 commits,
        sentinel in neither DiffTree, status shows '?? sentinel.txt'."

- file: internal/config/config.go   # CONSUMED — config shape + Defaults
  why: "Config.AutoStageAll defaults TRUE (L67/L135) ⇒ decompose triggers by default on an un-staged dirty
        tree. Config.Providers map[string]map[string]any (L95) ← [provider.X] TOML sections. CurrentConfigVersion=3."

- file: internal/provider/registry.go   # CONSUMED — user-provider merge
  why: "NewRegistry(overrides) (L36): built-ins ⊕ user overrides. A brand-new §12.8 provider (name not a
        built-in) is added verbatim from the table key (L49-50); a built-in name is field-merged (L45).
        DecodeUserOverrides (L154) bridges config.Providers → manifests. So [provider.stub] and
        [provider.testmulti] become usable providers via --config."

- file: internal/provider/manifest.go   # CONSUMED — TOML field names for [provider.X]
  why: "Field toml tags: command (L40), prompt_delivery (L44), output (L75), strip_code_fence (L77),
        default_model (L52), model_flag (L51), provider_flag (L58), tooled_flags (L67), detect (L39),
        env (L83, map[string]string). Defines the [provider.stub] / [provider.testmulti] config bodies."

- file: internal/generate/realagent_test.go   # PATTERN SOURCE — the dual-mode gating convention
  why: "//go:build integration_real; runs ONLY when STAGECOACH_RUN_REAL=1; t.Skip otherwise; builds no binary
        (in-process). The e2e harness mirrors the STAGECOACH_RUN_REAL gate but drives a SUBPROCESS binary
        and ALSO runs stub-mode scenarios by default. envOr(key,def) helper for provider/model env."

- file: internal/decompose/roles.go   # CONSUMED — FR-R5b error text (for the S4 assertion)
  why: "ResolveRoles (L90) + the FR-R5b guard (~L155): `isMultiProvider(m) && mdl != \"\" &&
        !strings.Contains(mdl, \"/\")` ⇒ error `role %q: model %q on %s must be inference/model, e.g.
        \"zai/glm-5.2\"`. isMultiProvider = ProviderFlag != \"\" (L178). So a [provider.testmulti] with
        provider_flag set + --model bare reproduces FR-R5b deterministically (no real pi install needed)."

- file: internal/exitcode/exitcode.go   # CONSUMED — exit-code expectations
  why: "0=Success; 1=Error (CAS/FR-R5b/gen-fail/usage); 2=NothingToCommit; 3=Rescue; 124=Timeout. S4→1,
        S6-rescue→3, S7-CAS→1."
```

### Current Codebase tree (relevant subset)

```bash
internal/decompose/
  decompose.go            # SUT: Decompose/freeze/one-file-shortcut (CONSUMED)
  roles.go                # SUT: ResolveRoles/FR-R5b (CONSUMED)
  stager.go               # SUT: verifyFreezeSubset/ErrFreezeViolation (CONSUMED)
  decompose_test.go       # EDIT — +TestDecompose_ConcurrentChangeExclusion
internal/e2e/             # (does not exist yet — NEW package)
internal/stubtest/stubtest.go  # CONSUMED — Build/Manifest/Options
cmd/stubagent/main.go          # CONSUMED — the stub binary (NO edit)
internal/git/git.go            # CONSUMED — DiffTree/StatusPorcelain/RevParseHEAD/...
internal/provider/{executor,registry,manifest}.go  # CONSUMED — env-flow/merge/TOML fields
internal/config/config.go      # CONSUMED — Defaults(AutoStageAll=true)/Providers
internal/cmd/{root,default_action}.go  # CONSUMED — --config flag + shouldDecompose routing
internal/generate/realagent_test.go    # PATTERN — STAGECOACH_RUN_REAL gate convention
```

### Desired Codebase tree (files this task ADDS/EDITS — 0 production changes)

```bash
internal/e2e/harness_test.go      # NEW //go:build e2e — shared helpers (binary builder, repo factory,
                                  #  git assertions, config writer, runStagecoach, waitForMarker, skipIfNotReal)
internal/e2e/scenarios_test.go    # NEW //go:build e2e — the 7 §20.5 scenarios (t.Run), dual-mode
internal/decompose/decompose_test.go  # EDIT — +TestDecompose_ConcurrentChangeExclusion (in-process)
```

### Known Gotchas of our codebase & Library Quirks

```go
// G-SUBPROCESS-NOT-INPROCESS: §20.5 says "run stagecoach". The e2e harness drives the COMPILED binary as a
//   SUBPROCESS (go build ./cmd/stagecoach; exec it with --config). The in-process library angle is §20.1
//   layer 3, already covered by decompose_test.go. Do NOT collapse them — the subprocess angle is what
//   catches CLI-routing + config-load bugs.

// G-STAGER-WALL: cmd/stubagent is stdlib-only and CANNOT `git add`. So multi-concept decompose (needs a
//   tooled stager) CANNOT run in stub mode via the binary. S1/S5/loop-S6 are therefore REAL-only in the
//   e2e harness (skipIfNotReal + a pointer to the in-process suite). Do NOT try to make the stub stage.

// G-ENV-FLOW: executor.go L58 `cmd.Env = spec.Env` (Render populates os.Environ()+manifest.Env). So set
//   per-scenario STAGECOACH_STUB_* knobs on the STAGECOACH PROCESS Env (test's exec.Command Env =
//   os.Environ()+knobs); they inherit to the stub via Render's os.Environ(). ONE base [provider.stub]
//   config; no per-scenario config for env.

// G-CONFIG-OVERRIDE: pass `--config <tmp.toml>`. The TOML defines [provider.stub] (command=stubbin,
//   output=raw, strip_code_fence=true, default_model=stub) + (optionally) [provider.testmulti]
//   (provider_flag="--provider", model_flag="--model") for S4. Do NOT rely on repo-local discovery.

// G-DECOMPOSE-TRIGGER: to route to decompose leave files UN-STAGED (no git add) in a dirty tree with the
//   default config (AutoStageAll=true). --single ⇒ v1 single-commit. --commits N ⇒ forced count.
//   Staging a file (--all or git add) ⇒ single-commit path.

// G-ONE-FILE-PATH-FREEZES: runOneFileShortcut (auto-mode + exactly 1 changed path) FREEZES, calls the
//   MESSAGE agent (stub SLEEP+MARKER), commits the frozen tStart. Planner is BYPASSED (FR-M2b). This is
//   the stub-reachable path for S2 + S3.

// G-MARKER-IS-DETERMINISTIC: STAGECOACH_STUB_MARKER is written by the stub AFTER draining stdin (generation
//   in-flight) and BEFORE the SLEEP — purpose-built for deterministic concurrent races. waitForMarker(t,path)
//   (poll for the file) ⇒ the action after it is guaranteed post-freeze / mid-generation. Use for S3
//   (write sentinel) and S7 (move HEAD).

// G-NO-PLANNER-CALL-CANARY: to assert "planner bypassed" (FR-M2b) in subprocess, give the PLANNER role a
//   CANARY provider (a tiny script that touches a marker file + exits 0) via --planner-provider canary /
//   [role.planner]. If decompose called the planner, the marker would exist (and the run would then fail
//   on empty planner output). Assert the marker is ABSENT ⇒ planner never ran. (The unit-level pin is
//   TestDecompose_OneFileShortcut_PlannerBypassed; this is the binary-level belt-and-suspenders.)

// G-FR5B-WITHOUT-PI: do NOT depend on `pi` being installed to test FR-R5b. Define [provider.testmulti]
//   (command=stubbin so IsInstalled passes; provider_flag="--provider" ⇒ isMultiProvider=true) +
//   --provider testmulti --model bare ⇒ ResolveRoles FR-R5b error. Deterministic on any machine.

// G-EXIT-CODES: S4 FR-R5b ⇒ exit 1 (Error). S6 single-rescue ⇒ exit 3 (Rescue). S7 CAS ⇒ exit 1. S2/S3
//   success ⇒ exit 0. Assert the exit code AND a stable stderr substring, not the full message.

// G-RACE-WINDOW-SLEEP: S7 needs the message agent in-flight when HEAD moves. Give the message stub a
//   STAGECOACH_STUB_SLEEP_MS (e.g. 1500) so waitForMarker + `git commit` lands inside the window. S3 same
//   idea (sleep gives a window, though the MARKER alone guarantees post-freeze). Keep sleeps modest (CI).

// G-DECOMPOSE-TEST-HELPER-REUSE: the new in-process test REUSES dcmInitRepo/dcmWriteFile/dcmCommitRaw/
//   dcmRunGit/dcmPlannerManifest/dcmMessageScriptManifest/dcmAllRoles/tooledStubManifest/dcmLogCount —
//   do NOT reinvent them. Build a CUSTOM stager seam (not dcmStagerSeam) that writes the sentinel UNSTAGED.

// G-FREEZE-VIOLATION-IS-THE-SIBLING: TestDecompose_StagerFreezeViolation STAGES a post-freeze sentinel ⇒
//   ErrFreezeViolation (failure path). The NEW test writes the sentinel UNSTAGED ⇒ happy path (excluded,
//   run succeeds). Same sentinel, opposite staging decision, opposite outcome. Keep them adjacent + cross-
//   reference each other in comments.
```

## Implementation Blueprint

### Data models and structure

No domain data models (test-only). Two small e2e-package helper types + one stager-seam closure:

```go
// internal/e2e/harness_test.go (//go:build e2e)
// e2eResult bundles a stagecoach subprocess run's observable outputs for assertion.
type e2eResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// stubEnv builds the stagecoach process env for one scenario: os.Environ() + the STAGECOACH_STUB_* knobs.
// (executor.go: Render = os.Environ()+manifest.Env ⇒ these inherit to the stub subprocess.)
func stubEnv(knobs map[string]string) []string
```

```go
// internal/decompose/decompose_test.go — the custom stager seam for the concurrent-exclusion test.
// Like dcmStagerSeam but, for concept 0, ALSO writes an UNSTAGED sentinel to the working tree mid-run
// (after FreezeWorkingTree captured T_start). Returns the standard stager signature.
func concurrentSentinelSeam(t *testing.T, repo string, conceptFiles map[string][]string, sentinel string) func(context.Context, Deps, prompt.PlannerCommit) error
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/e2e/harness_test.go (//go:build e2e) — shared helpers
  - FILE HEADER: `//go:build e2e` + package doc citing PRD §20.5 + the dual-mode contract.
  - IMPLEMENT buildStagecoach(t *testing.T) string: sync.Once + exec.LookPath("go") (t.Skipf if absent) +
    t.TempDir + `go build -o <tmp>/stagecoach(-.exe) github.com/dustin/stagecoach/cmd/stagecoach`; cache the
    path (mirror stubtest.Build exactly — import-path build so cwd-independent; windows .exe suffix).
  - IMPLEMENT buildStub(t *testing.T) string: return stubtest.Build(t) (REUSE — do not rebuild).
  - IMPLEMENT newRepo(t *testing.T) string: t.TempDir + `git init -q` + repo-local identity
    (`git config user.email/name`) — mirror dcmInitRepo/initRepo.
  - IMPLEMENT seed/git helpers (raw `git -C <repo>`): seedCommit(t,repo,name,body) (write+add+commit),
    writeFile(t,repo,name,body), stageFile(t,repo,name), gitOut(t,repo,args...) string, headSHA(t,repo),
    commitCount(t,repo) (git rev-list --count HEAD), diffTreeNames(t,repo,sha) []string
    (`git diff-tree --no-commit-id --name-only -r <sha>`), statusPorcelain(t,repo) string.
  - IMPLEMENT writeStubConfig(t *testing.T, stubBin string, extras string) string: write a TOML to
    t.TempDir/stagecoach.toml with `config_version = 3` + `[provider.stub]` (command=stubBin,
    prompt_delivery="stdin", output="raw", strip_code_fence=true, default_model="stub") + the `extras`
    string verbatim (for [provider.testmulti] / [provider.canary] / [role.planner] blocks). Return the path.
  - IMPLEMENT runStagecoach(t *testing.T, bin, repo, cfgPath string, env []string, args ...string) e2eResult:
    exec.Command(bin, append([]string{"--config", cfgPath, "--no-color"}, args...)...); cmd.Dir=repo;
    cmd.Env=env; capture stdout/stderr; Run(); map ExitError→code. Use a per-run context timeout
    (e.g. 60s) so a hung agent fails the test (not the suite).
  - IMPLEMENT waitForMarker(t *testing.T, path string, timeout time.Duration): poll (every ~20ms) for the
    marker file's existence up to timeout; t.Fatal on timeout (the stub never reached generation).
  - IMPLEMENT skipIfNotReal(t *testing.T, why string): if os.Getenv("STAGECOACH_RUN_REAL")!="1" → t.Skipf
    ("%s (needs a tooled stager; set STAGECOACH_RUN_REAL=1 + STAGECOACH_E2E_PROVIDER for the real run; see
    internal/decompose/decompose_test.go for the in-process stub coverage: %s)", why, pointer).
  - IMPLEMENT realAgent(t *testing.T) (provider, model string): if STAGECOACH_RUN_REAL!="1" t.Skip; read
    STAGECOACH_E2E_PROVIDER (default "pi") + STAGECOACH_E2E_MODEL (default "" → manifest default). Mirror
    realagent_test.go's envOr.
  - NAMING: exported only within package (lowercase is fine — test package). snake_case none; Go conventions.
  - FOLLOW pattern: stubtest.Build (sync.Once build), dcmRunGit (raw git), realagent_test.go (envOr/skip).

Task 2: CREATE internal/e2e/scenarios_test.go (//go:build e2e) — the 7 scenarios
  - FILE HEADER: `//go:build e2e` + package doc. One TestE2EScenarios(t) with t.Run subtests (or one
    top-level Test per scenario — either is fine; t.Run keeps one binary run).
  - COMMON SETUP per subtest: bin:=buildStagecoach(t); stub:=buildStub(t); cfg:=writeStubConfig(t,stub,extras).
  - S1_NothingStagedNFiles_NCommits (REAL-only):
      skipIfNotReal(t,"S1 needs a tooled stager to stage N concepts"); prov,model:=realAgent(t);
      repo:=newRepo(t); seedCommit(t,repo,"readme","init"); write N (e.g. 3) unrelated files UN-STAGED;
      run A: runStagecoach(..., stubEnv(nil), "--provider",prov,(model flag)); assert commitCount==N (auto).
      run B (fresh repo, same seed): runStagecoach(..., "--commits","3", ...); assert commitCount==3 (forced).
      Assert each commit's diffTreeNames == exactly its concept files (concept isolation) + status clean.
  - S2_OneFile_NoPlannerCall (stub + real):
      extras = `[provider.canary]` command=a canary script (write $CANARY_MARKER; exit 0) +
               `[role.planner] provider="canary"` (so the planner role uses the canary).
      repo: seedCommit; write ONE file (e.g. solo.txt) UN-STAGED. canaryMarker := t.TempDir+"/planner_called".
      env := stubEnv({OUT:"feat: solo file", MARKER: <msgMarker>}) + CANARY_MARKER=canaryMarker.
      res := runStagecoach(..., env) (auto-mode; one-file ⇒ runOneFileShortcut, planner bypassed).
      assert res.ExitCode==0; commitCount==1; diffTreeNames(head) contains solo.txt.
      assert canaryMarker does NOT exist (planner never called — FR-M2b).
      (REAL variant: prov/model:=realAgent(t); same seed; assert 1 commit; canary absent.)
  - S3_ConcurrentFile_Excluded (stub + real, one-file path):
      repo: seedCommit; write ONE file (kept.txt) UN-STAGED. sentinel := repo+"/intruder.txt".
      msgMarker := t.TempDir+"/msg.marker". env := stubEnv({OUT:"feat: keep", MARKER:msgMarker, SLEEP_MS:800}).
      Launch runStagecoach in a goroutine (capture res via a channel/ptr). In the main goroutine:
      waitForMarker(t, msgMarker, 10s); os.WriteFile(sentinel, []byte("concurrent\n"), 0o644); then wait for res.
      assert res.ExitCode==0; commitCount==1; diffTreeNames(head)=={kept.txt} (NO intruder.txt);
      statusPorcelain contains "?? intruder.txt" (it REMAINS in the working tree — FR-M1b/M1c).
      (REAL variant: same; the real agent is slower — waitForMarker may be replaced by a short sleep/poll,
       but the invariant is identical: intruder excluded + remains.)
  - S4_MultiBackendBareModel_HardError (stub + real):
      extras = `[provider.testmulti]` (command=stub, prompt_delivery="stdin", output="raw",
        strip_code_fence=true, model_flag="--model", provider_flag="--provider", default_model="x").
      repo: seedCommit; write a file UN-STAGED (to route to decompose). env:=stubEnv({OUT:"feat: x"}).
      res := runStagecoach(..., env, "--provider","testmulti","--model","bare").
      assert res.ExitCode==1 (Error); res.Stderr contains "must be inference/model" (FR-R5b hard error,
      not silent empty output). assert commitCount unchanged (no commit created).
  - S5_ArbiterReconciliation (REAL-only; specific modes in-process):
      skipIfNotReal(t,"S5 needs a tooled stager"); prov,model:=realAgent(t);
      Seed N files with intentional overlap so leftovers are plausible; run; assert the general invariants:
      commitCount==N, all SHAs resolvable, status clean OR fully reconciled (no partial leftover).
      (The new/tip/mid SPECIFIC modes are pinned by TestDecompose_ArbiterWiring + dcmScriptArbiter
       in-process — assert the general property here, cross-reference there.)
  - S6_Rescue (stub single + real loop):
      repo: seedCommit; stage a file (single-commit path) OR write one file UN-STAGED (one-file path).
      env := stubEnv({OUT:"", MARKER:msgMarker}) — empty output ⇒ ParseOutput ok=false ⇒ rescue.
      res := runStagecoach(..., env). assert res.ExitCode==3 (Rescue); res.Stderr contains the rescue text.
      assert HEAD unchanged (no commit). (Loop-rescue mid-concept is TestDecompose_MessageRescuePartial
       in-process — cross-reference.)
  - S7_CASAbort_HeadMoved (stub + real):
      repo: seedCommit; stage a file (single path). msgMarker:=t.TempDir+"/msg.marker".
      env := stubEnv({OUT:"feat: x", MARKER:msgMarker, SLEEP_MS:1500}).
      Launch runStagecoach in a goroutine. Main: waitForMarker(t,msgMarker,10s); capture preHEAD:=headSHA;
      gitOut(t,repo,"commit","--allow-empty","-m","concurrent"); concurrent:=headSHA.
      Wait for res. assert res.ExitCode==1 (CAS); headSHA==concurrent (HEAD unchanged by stagecoach —
      UpdateRefCAS aborted); the staged file is still staged (index idempotent — optional extra assert).
  - CLEANUP: each subtest uses t.TempDir (auto-cleaned). Goroutine-launched runs MUST be awaited (no leak).
  - FOLLOW pattern: realagent_test.go (skip gate + envOr), dcmRunGit (raw git), TestCommitStaged_CASFailure
    (the marker+sleep+move-HEAD race, in-process — mirror its timing as a subprocess).
  - GOTCHA: a goroutine-launched runStagecoach must use a context with a timeout so a hang fails the test
    (not the suite). Await it before the subtest returns (t.Go / a done channel + select on timeout).

Task 3: EDIT internal/decompose/decompose_test.go — +TestDecompose_ConcurrentChangeExclusion
  - ADD TestDecompose_ConcurrentChangeExclusion (place it ADJACENT to TestDecompose_StagerFreezeViolation
    and cross-reference it in the doc comment — they are the success/failure-path siblings).
  - STRUCTURE (mirror TestDecompose_AutoMultiCommit_HappyPath, L394):
      bin:=stubtest.Build(t); repo:=t.TempDir(); dcmInitRepo(t,repo); dcmCommitRaw(t,repo,"initial");
      dcmWriteFile(t,repo,"a.txt","aaa\n"); dcmWriteFile(t,repo,"b.txt","bbb\n"); // UN-STAGED, dirty tree
      plannerJSON := 2 concepts: [{title:"add a", files:["a.txt"]}, {title:"add b", files:["b.txt"]}].
      plannerM := dcmPlannerManifest(t,bin,plannerJSON);
      messageM := dcmMessageScriptManifest(t,bin,[]string{"feat: add a","feat: add b"});
      roles := dcmAllRoles(t,bin,stubtest.Options{Out:""}); // tooled stub stager (can't run git) — overridden
      deps := dcmDeps(t,repo,roles); deps.Config.Commits=0; // auto
      deps.stager = concurrentSentinelSeam(t, repo,
          map[string][]string{"add a":{"a.txt"},"add b":{"b.txt"}}, "sentinel.txt");
      res, err := Decompose(ctx, deps); assert err==nil; assert len(res.Commits)==2.
  - IMPLEMENT concurrentSentinelSeam (the closure): for the concept, `git add` its files (like dcmStagerSeam);
    ADDITIONALLY, when concept.Title=="add a" (the FIRST concept), os.WriteFile(repo+"/sentinel.txt",
    []byte("concurrent\n"),0o644) — UNSTAGED. (Freeze already ran before concept 0 ⇒ sentinel is post-freeze.)
  - ASSERTIONS:
      log := dcmLogOneline(t,repo); assert 2 new commits.
      for each commit SHA: assert "sentinel.txt" NOT IN dcmGitOut(t,repo,"diff-tree","--no-commit-id",
        "--name-only","-r",sha) (sentinel in NO commit).
      assert dcmStatusPorcelain(t,repo) contains "?? sentinel.txt" (sentinel REMAINS in the working tree).
      assert the file still exists on disk with its content (os.ReadFile).
  - FOLLOW pattern: dcmStagerSeam (the git-add seam), TestDecompose_AutoMultiCommit_HappyPath (2-concept
    setup), TestDecompose_StagerFreezeViolation (the sentinel idiom + adjacent placement).
  - GOTCHA: write sentinel UNSTAGED (os.WriteFile, NOT git add) — staging it would turn this into the
    failure-path freeze-violation test. The seam runs AFTER FreezeWorkingTree (which is before concept 0),
    so the sentinel is guaranteed post-freeze ⇒ excluded from every concept tree ⇒ in no commit. The
    sentinel stays untracked ⇒ remains in the working tree. This is DETERMINISTIC (no goroutine, no timing).
```

### Implementation Patterns & Key Details

```go
// buildStagecoach (harness_test.go) — mirror stubtest.Build for the stagecoach binary (import-path build):
var (
	stagecoachOnce sync.Once
	stagecoachBin  string
)
func buildStagecoach(t *testing.T) string {
	t.Helper()
	stagecoachOnce.Do(func() {
		goPath, err := exec.LookPath("go")
		if err != nil { t.Skipf("go toolchain not on PATH: %v", err); return }
		dir := t.TempDir()
		name := "stagecoach"; if runtime.GOOS == "windows" { name = "stagecoach.exe" }
		stagecoachBin = filepath.Join(dir, name)
		out, err := exec.Command(goPath, "build", "-o", stagecoachBin,
			"github.com/dustin/stagecoach/cmd/stagecoach").CombinedOutput()
		if err != nil { t.Fatalf("go build stagecoach: %v\n%s", err, out) }
	})
	return stagecoachBin
}

// writeStubConfig — ONE base config; per-scenario knobs ride the process env (G-ENV-FLOW).
func writeStubConfig(t *testing.T, stubBin, extras string) string {
	t.Helper()
	body := `config_version = 3
[provider.stub]
command = ` + fmt.Sprintf("%q", stubBin) + `
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
` + extras + "\n"
	p := filepath.Join(t.TempDir(), "stagecoach.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil { t.Fatalf("write config: %v", err) }
	return p
}

// runStagecoach — drive the binary; capture stdout/stderr/exit.
func runStagecoach(t *testing.T, bin, repo, cfg string, env []string, args ...string) e2eResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second); defer cancel()
	cmd := exec.CommandContext(ctx, append([]string{bin, "--config", cfg, "--no-color"}, args...)...)
	cmd.Dir = repo; cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	r := e2eResult{Stdout: out.String(), Stderr: errb.String()}
	if err := cmd.Run(); err != nil {
		if ee := (*exec.ExitError)(nil); errors.As(err, &ee) { r.ExitCode = ee.ExitCode() } else { t.Fatalf("run: %v", err) }
	}
	return r
}

// S3 / S7 concurrent race — goroutine + waitForMarker (deterministic via the stub MARKER).
//   resCh := make(chan e2eResult, 1)
//   go func() { resCh <- runStagecoach(t, bin, repo, cfg, stubEnv(map[string]string{
//       "STAGECOACH_STUB_OUT": "feat: keep", "STAGECOACH_STUB_MARKER": msgMarker, "STAGECOACH_STUB_SLEEP_MS": "800",
//   })) }()
//   waitForMarker(t, msgMarker, 10*time.Second)
//   os.WriteFile(filepath.Join(repo, "intruder.txt"), []byte("concurrent\n"), 0o644)  // S3
//   //   — or for S7: gitOut(t, repo, "commit", "--allow-empty", "-m", "concurrent")
//   res := <-resCh

// The in-process concurrent-exclusion seam (decompose_test.go) — UNSTAGED sentinel mid-loop.
func concurrentSentinelSeam(t *testing.T, repo string, conceptFiles map[string][]string, sentinel string) func(context.Context, Deps, prompt.PlannerCommit) error {
	return func(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error {
		t.Helper()
		for _, f := range conceptFiles[concept.Title] { dcmRunGit(t, repo, "add", f) }
		if concept.Title == "add a" { // FIRST concept ⇒ write the sentinel UNSTAGED, post-freeze
			if err := os.WriteFile(filepath.Join(repo, sentinel), []byte("concurrent\n"), 0o644); err != nil {
				t.Fatalf("write sentinel: %v", err)
			}
		}
		return nil
	}
}
```

### Integration Points

```yaml
NEW PACKAGE internal/e2e (//go:build e2e):
  - harness_test.go: buildStagecoach/buildStub/newRepo/seed+git helpers/writeStubConfig/runStagecoach/
    waitForMarker/skipIfNotReal/realAgent/stubEnv. Stdlib + internal/stubtest only.
  - scenarios_test.go: 7 t.Run scenarios. Stdlib + the harness helpers.
  - NO new imports beyond stdlib + internal/stubtest (+ internal/git types ONLY if needed; prefer raw git).
EDIT internal/decompose/decompose_test.go:
  - +TestDecompose_ConcurrentChangeExclusion + concurrentSentinelSeam closure. Uses EXISTING helpers
    (dcm*/stubtest/tooledStubManifest); adds os + filepath (already imported in the file — verify).
NO production-code changes. NO config/provider schema changes. NO go.mod changes. NO new env vars in prod
  (STAGECOACH_RUN_REAL / STAGECOACH_E2E_PROVIDER / STAGECOACH_E2E_MODEL are TEST-ONLY knobs, like realagent_test.go).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Normal build/vet (e2e package EXCLUDED without -tags — must stay green):
go build ./...
go vet ./...
gofmt -l internal/                       # MUST print nothing

# e2e package compiles + vets under its tag:
go vet -tags e2e ./internal/e2e/
gofmt -l internal/e2e/

# Lint (Makefile `make lint`):
golangci-lint run
# (If golangci-lint skips the e2e test files, additionally: golangci-lint run --build-tags e2e ./internal/e2e/)

# Expected: zero errors. Verify: every e2e file starts with `//go:build e2e` + a blank line. Verify
# buildStagecoach uses the import-path form (github.com/dustin/stagecoach/cmd/stagecoach). Verify the new
# decompose test's seam writes the sentinel UNSTAGED (os.WriteFile, no git add).
```

### Level 2: Unit / Component Tests (the v2 suite extension)

```bash
# The new in-process concurrent-exclusion test (runs under the NORMAL suite — no tag):
go test -race ./internal/decompose/ -run TestDecompose_ConcurrentChangeExclusion -v

# Full decompose suite (regression — all existing tests stay green):
go test -race ./internal/decompose/ -v

# Expected: PASS. If it fails with ErrFreezeViolation, the seam STAGED the sentinel (bug — write UNSTAGED).
# If the sentinel appears in a commit, the freeze did not exclude it (re-check runOneFileShortcut/runLoop
# commit the FROZEN tStart/tree[i], not a live AddAll). If status lacks "?? sentinel.txt", the sentinel was
# staged or committed (bug).
```

### Level 3: Integration / E2E (the harness — the core deliverable)

```bash
# Stub mode (default — the CI-able subset; builds stagecoach + stubagent once):
go test -tags e2e ./internal/e2e/ -v
# Expected: S2/S3/S4/S6-single/S7 PASS; S1/S5/loop-S6 SKIP with the clear pointer message.

# Targeted scenario debugging:
go test -tags e2e ./internal/e2e/ -run 'TestE2EScenarios/S3' -v
go test -tags e2e ./internal/e2e/ -run 'TestE2EScenarios/S7' -v

# Real mode (manual, pre-release — drives a real agent):
STAGECOACH_RUN_REAL=1 STAGECOACH_E2E_PROVIDER=pi go test -tags e2e ./internal/e2e/ -v -timeout 30m
# Expected: all 7 PASS against the real agent (general invariants where behavior is nondeterministic).

# Expected on failure: READ res.Stderr (it carries stagecoach's full stderr — progress/rescue/CAS/error).
# S3/S7 timing flake ⇒ bump STAGECOACH_STUB_SLEEP_MS; a hang ⇒ the 60s run context fired (check waitForMarker
# didn't time out → the stub MARKER was never written → the message agent didn't run → routing/stub wiring bug).
```

### Level 4: Creative & Domain-Specific Validation (the §20.5 invariants)

```bash
# Each scenario encodes one §20.5 / §20.2 invariant; the assertions ARE the validation:
#   S1: concept isolation — each commit's diff-tree == its concept files (§20.2 Concept isolation).
#   S2: FR-M2b planner bypass — canary marker ABSENT.
#   S3 + the decompose test: Start-of-run freeze — sentinel in NO commit + remains in tree (§20.2).
#   S4: FR-R5b hard error — exit 1 + "must be inference/model" (never silent empty output).
#   S6: rescue — exit 3 + HEAD unchanged.
#   S7: Atomic HEAD / idempotent index — CAS abort leaves HEAD at the concurrent commit (§20.2).

# Optional belt-and-suspenders for S7 (idempotent index, §20.2): snapshot `git diff --cached --name-only`
# BEFORE the run and assert it's identical AFTER (no index mutation on the CAS failure path).
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` green (e2e excluded without `-tags`).
- [ ] `go vet ./...` + `go vet -tags e2e ./internal/e2e/` clean.
- [ ] `go test -race ./...` green (make test) — incl. the new decompose test.
- [ ] `golangci-lint run` clean (make lint); `gofmt -l internal/` empty.
- [ ] `make coverage-gate` unaffected (test-only changes; no gated-package regression).
- [ ] go.mod/go.sum UNCHANGED (stdlib + internal/stubtest only).

### Feature Validation

- [ ] `go test -tags e2e ./internal/e2e/ -v`: S2/S3/S4/S6-single/S7 PASS (stub); S1/S5/loop-S6 SKIP clearly.
- [ ] S2: one commit + planner canary marker ABSENT (FR-M2b).
- [ ] S3: sentinel excluded from the commit + `?? sentinel.txt` remains (FR-M1b/M1c).
- [ ] S4: exit 1 + `must be inference/model` (FR-R5b hard error).
- [ ] S7: exit 1 (CAS) + HEAD unchanged (atomic HEAD).
- [ ] `STAGECOACH_RUN_REAL=1 go test -tags e2e ./internal/e2e/ -v` runs all 7 against a real agent.
- [ ] `TestDecompose_ConcurrentChangeExclusion`: 2 commits, sentinel in NEITHER, remains in tree.

### Code Quality Validation

- [ ] e2e helpers reuse `stubtest.Build` (no duplicate stub build) + raw-git idiom (mirror dcmRunGit).
- [ ] Per-scenario stub knobs ride the stagecoach PROCESS env (G-ENV-FLOW); ONE base [provider.stub] config.
- [ ] FR-R5b tested via a custom [provider.testmulti] (no dependence on `pi` being installed).
- [ ] The new decompose test REUSES existing dcm*/stubtest helpers + a custom UNSTAGED-sentinel seam.
- [ ] New decompose test placed adjacent to + cross-referencing `TestDecompose_StagerFreezeViolation`.
- [ ] Goroutine-launched runs are awaited (no leak); each has a context timeout (a hang fails the test).

### Documentation & Deployment

- [ ] e2e package doc + each scenario doc comment cite PRD §20.5 + the invariant it pins.
- [ ] skipIfNotReal messages point to `STAGECOACH_RUN_REAL=1` AND the in-process suite (two ways to run).
- [ ] No production docs (DOCS: none — test-only). Changeset-level README/docs sync is P4.M2.T1.S1 (NOT this task).

---

## Anti-Patterns to Avoid

- ❌ Don't make the e2e harness in-process — §20.5 says "run `stagecoach`"; the subprocess angle is the whole
  point (catches CLI-routing + config-load bugs the library tests can't). The in-process angle is §20.1
  layer 3 (already covered by decompose_test.go).
- ❌ Don't try to make `cmd/stubagent` stage (run `git add`) — it's stdlib-only and can't. Multi-concept
  decompose (S1/S5/loop-S6) is REAL-only in the e2e harness; its deterministic stub coverage lives in the
  in-process v2 suite. Trying to stage via the stub WILL fail and wastes the one-pass budget.
- ❌ Don't depend on `pi` being installed to test FR-R5b — define a `[provider.testmulti]`
  (`provider_flag="--provider"`) so `isMultiProvider` is true on any machine; `--model bare` reproduces the
  guard deterministically.
- ❌ Don't put per-scenario `STAGECOACH_STUB_*` knobs in a per-scenario config file — `Render` builds
  `os.Environ()+manifest.Env` (executor.go L58-59), so set them on the stagecoach PROCESS env (one base config).
- ❌ Don't write the in-process sentinel STAGED — that's the FAILURE path
  (`TestDecompose_StagerFreezeViolation` → `ErrFreezeViolation`). The new test writes it UNSTAGED (os.WriteFile,
  no `git add`) so the run SUCCEEDS and the sentinel is merely excluded + remaining.
- ❌ Don't assert full stderr/commit-message text in the subprocess tests — agent text is nondeterministic
  (even the stub's). Assert EXIT CODE + a stable substring + the GIT HISTORY (commit count, diff-tree file
  set, `git status`). The git history is the deterministic ground truth.
- ❌ Don't skip `waitForMarker` for S3/S7 and use a fixed sleep alone — the MARKER is purpose-built for
  deterministic post-freeze/mid-generation races; a bare sleep is CI-flaky. (A modest SLEEP_MS widens the
  window; the MARKER guarantees the ordering.)
- ❌ Don't leak goroutines in S3/S7 — await the `runStagecoach` result before the subtest returns, and give
  every run a context timeout so a hang fails the test (not the suite).
- ❌ Don't duplicate existing v2-suite coverage in the e2e harness as if new — the in-process suite already
  pins S1/S5/S6-loop/S7 at the unit level; the e2e harness ADDS the subprocess/real-repo angle (and clearly
  SKIPS stager-dependent scenarios in stub mode with a pointer to the in-process suite).
- ❌ Don't modify production code, PRD.md, tasks.json, go.mod, or .gitignore — this is test-only.
