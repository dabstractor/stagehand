# Research Findings — P4.M1.T1.S2 (E2E scenario harness §20.5)

## 1. The design split (resolved)

The contract names TWO deliverables:

| Deliverable | Location | Mode | What it uniquely catches |
|---|---|---|---|
| **E2E harness** (NEW) | `internal/e2e/` (`//go:build e2e`) | SUBPROCESS (drives compiled `stagecoach` binary) | CLI routing + config-load + real-repo + real-agent bugs — the §20.5 "gap that let the bugs ship" |
| **v2 stub suite EXTENSION** | `internal/decompose/decompose_test.go` (+1 test) | IN-PROCESS (stager seam) | Deterministic, CI-able multi-concept concurrent-change-exclusion (FR-M1b/M1c happy path) |

The in-process v2 stub suite **already covers** scenarios 1, 5, 6, 7 (loop/arbiter/rescue/CAS) via the
stager seam + `dcmScriptArbiter`. So the e2e harness's UNIQUE value is the subprocess/real-repo angle.

## 2. Why subprocess (not in-process) for the e2e harness

§20.5 literally says "run `stagecoach`" + "else the stub" (the stub = `cmd/stubagent`, which must be wired
via a provider config to be usable by the binary). The cited bugs (planner-empty-output FR-R5b,
concurrent-file FR-M1b) are ROUTING/real-repo behaviors the in-process library tests don't reach.

**Stager wall:** `cmd/stubagent` is stdlib-only and CANNOT run `git add` (the tooled-stager behavior). So
multi-concept decompose scenarios (1, 5, loop-6) **cannot run in stub mode via the binary** — the stub
stager would produce no staged content → empty concepts. These scenarios in the e2e harness are therefore
**REAL-only** (gated on `STAGECOACH_RUN_REAL=1`); their deterministic stub coverage lives in the in-process
v2 suite (already present + the new concurrent-exclusion test).

## 3. Scenarios × reachability

| # | Scenario | Stub-reachable (subprocess)? | Mechanism |
|---|---|---|---|
| 1 | nothing staged + N files → N commits (auto & --commits N) | NO (needs tooled stager) → REAL-only | real agent stages |
| 2 | exactly one file → single commit, NO planner call (FR-M2b) | YES | one-file shortcut bypasses planner; "no planner call" via a CANARY planner provider (touches a marker if run → assert marker ABSENT) |
| 3 | concurrent file mid-run → excluded + remains (FR-M1b/M1c) | YES (via one-file path) | freeze; sentinel written AFTER the stub `STAGECOACH_STUB_MARKER` appears (post-freeze, deterministic) |
| 4 | multi-backend agent, bare model → HARD ERROR (FR-R5b) | YES | custom `[provider.testmulti]` (provider_flag set) + `--model bare` → ResolveRoles/Render error, exit 1 |
| 5 | arbiter reconciliation (new/tip/mid) | NO (needs stager) → REAL-only (specific modes pinned in-process via `dcmScriptArbiter`) | real smoke + in-process authoritative |
| 6 | rescue mid-loop | partial: single/one-file rescue stub-reachable (empty stub output → exit 3); loop-rescue in-process | empty `STAGECOACH_STUB_OUT` |
| 7 | CAS abort (HEAD moved) | YES (single/one-file path) | stub MARKER + SLEEP → test moves HEAD → UpdateRefCAS fails → exit 1 |

## 4. Critical codebase facts (all verified)

### 4.1 Subprocess wiring
- `--config <path>` flag → `flagConfig` → `config.LoadOpts.ConfigPathOverride` (root.go:88,106). **The
  harness writes a TOML config and passes `--config`.** No repo-local discovery needed.
- `[provider.<name>]` TOML section → `config.Providers` map → `DecodeUserOverrides` → brand-new §12.8
  provider (table key = name) or field-merge onto a built-in (`registry.go:42-50`).
- Provider manifest TOML fields (manifest.go): `command`, `prompt_delivery`, `output`, `strip_code_fence`,
  `default_model`, `model_flag`, `provider_flag`, `tooled_flags`, `env` (sub-table `map[string]string`),
  `detect`.
- **Env flow:** `executor.go:58-59` — `cmd.Env = spec.Env` (non-empty); `Render` builds
  `os.Environ() + manifest.Env`. So stub knobs set on the **stagecoach process env** inherit to the stub
  subprocess (one base config reused; per-scenario knobs on the process env).

### 4.2 Routing (default_action.go)
- `shouldDecompose(cfg, dryRun, noAutoStage)` (L253): nothing staged (caller-guaranteed) + `cfg.AutoStageAll`
  (default TRUE) + not `cfg.Single` + not `--commits 1` + not `--dry-run` → `runDecompose`.
- So: un-staged dirty tree (no `git add`) + default config → decompose. `--single` → v1. `--commits N` →
  forced count. `--all`/staged → single-commit.

### 4.3 The stub (`cmd/stubagent`, `internal/stubtest`)
- `stubtest.Build(t)` compiles `./cmd/stubagent` ONCE per process (cached) → path. `stubtest.Manifest(bin, opts)`
  / `stubtest.NewScript(t, bin, responses)` build manifests.
- Knobs (all `STAGECOACH_STUB_*`): `OUT` (single response), `SCRIPT`+`COUNTER` (call-varying), `EXIT`,
  `SLEEP_MS`, `STDERR`, **`MARKER`** — "tells the test harness stdin drained + generation in-flight; must
  happen BEFORE the sleep so the test can race HEAD movement **deterministically**." Purpose-built for
  scenarios 3 & 7.
- Build the stagecoach binary: `go build -o <tmp> ./cmd/stagecoach` (mirror stubtest.Build; resolve via import
  path so cwd-independent).

### 4.4 The freeze (the heart of scenarios 2,3 + the v2 extension)
- `Decompose` → `FreezeWorkingTree(baseTree) → T_start` ONCE, before the planner (decompose.go ~L220).
  Planner/shortcut/arbiter draw from T_start; escape-hatch (`--single`/`--commits 1`) does NOT freeze.
- One-file shortcut `runOneFileShortcut` (L280): when auto-mode + exactly 1 changed path (DiffTreeNames
  baseTree→tStart) → **planner bypassed**, message generated from baseTree→tStart, commits tStart directly.
  This is the FR-M2b path — freezes, no stager, no planner. **Stub-reachable.**
- `verifyFreezeSubset` (stager.go:158): after each staging step, tree[i] must be a content-subset of
  T_start → hard abort `ErrFreezeViolation` (the FAILURE path; already tested by
  `TestDecompose_StagerFreezeViolation` which STAGES a post-freeze sentinel).

### 4.5 Assertion primitives
- `git.DiffTree(ctx, sha, isRoot) []FileChange` — files in a commit. `FileChange{Status,SrcPath,Path}`.
- `git.CommitCount`, `git.LogRange(baseSHA)`, `git.StatusPorcelain` (sentinel untracked = `?? sentinel.txt`),
  `git.RevParseHEAD`. In the subprocess harness use raw `git -C <repo> …` (mirror `dcmRunGit`).
- Exit codes (exitcode.go): 0=Success, 1=Error(CAS/FR-R5b/gen-fail), 2=NothingToCommit, 3=Rescue, 124=Timeout.

### 4.6 FR-R5b error text (roles.go:~155)
`role %q: model %q on %s must be inference/model, e.g. "zai/glm-5.2"`. Assert a stable substring
(`must be inference/model`) in stderr + exit 1.

### 4.7 Existing in-process v2 suite coverage (decompose_test.go) — DO NOT duplicate, EXTEND
- `TestDecompose_AutoMultiCommit_HappyPath` (S1 stub), `TestDecompose_OneFileShortcut_PlannerBypassed` (S2
  stub — already pins "no planner call" at unit level), `TestDecompose_StagerFreezeViolation` (freeze
  FAILURE path), `TestDecompose_ArbiterWiring` + `dcmScriptArbiter` (S5 tip/mid stub),
  `TestDecompose_MessageRescuePartial` (S6 stub), `TestDecompose_CASAbortPartial` (S7 stub).
- Helpers: `dcmInitRepo/dcmWriteFile/dcmStageFile/dcmCommitRaw/dcmRunGit/dcmGitOut/dcmHeadSHA/dcmLogOneline/
  dcmLogCount/dcmStatusPorcelain/dcmPlannerManifest/dcmArbiterManifest/dcmMessageManifest/dcmStagerSeam/
  dcmAllRoles/tooledStubManifest`.
- `dcmStagerSeam(repo, conceptFiles map[string][]string)` — the seam that does real `git add` of named
  paths (the deterministic in-process stager). The NEW concurrent-exclusion test injects the sentinel here.

## 5. Parallel-work-item safety (P4.M1.T1.S1)
P4.M1.T1.S1 edits `internal/ui/*` + `internal/cmd/default_action.go` (progress label FR51b) — **zero file
overlap** with this task (`internal/e2e/*` NEW + `internal/decompose/decompose_test.go` EDIT). The e2e
harness OBSERVES progress lines only incidentally (it asserts on git history + exit codes, not label text),
so the FR51b label format change cannot break it. Safe to run in parallel.

## 6. Build/test/lint commands (verified in Makefile)
- `make test` = `go test -race ./...` (NO build tag → e2e package excluded; decompose_test.go addition runs).
- `make lint` = `golangci-lint run`.
- `make coverage-gate` = ≥85% on `internal/{git,provider,generate,config}` (e2e + decompose_test are test-
  only → no production-package coverage regression).
- E2E: `go test -tags e2e ./internal/e2e/ -v` (stub mode); `STAGECOACH_RUN_REAL=1 go test -tags e2e
  ./internal/e2e/ -v -timeout 30m` (real mode).
- `go build ./...` (no tag) MUST stay green (e2e package is build-tagged test-only → excluded).
