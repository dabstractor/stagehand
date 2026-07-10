# Research Findings — P1.M3.T2.S1: Wire per-role timeouts at the 4 decompose Execute sites

## 0. Dependency status — `config.ResolveRoleTimeout` is LANDED (CONSUME, don't re-implement)

P1.M2.T1.S1 is **Complete**: `ResolveRoleTimeout` + `defaultRoleTimeouts` exist in `internal/config/roles.go`.
This item CONSUMES them; it does NOT touch `internal/config/`. Confirmed by direct read:

```go
// internal/config/roles.go:96 (LANDED)
func ResolveRoleTimeout(role string, cfg Config) time.Duration {
	if rc, ok := cfg.Roles[role]; ok && rc.Timeout != 0 {
		return rc.Timeout           // per-role override wins (even for planner)
	}
	if d, ok := defaultRoleTimeouts[role]; ok {
		return d                    // BUILT-IN: planner → 480s (the ONLY entry)
	}
	return cfg.Timeout              // global fallback (stager/message/arbiter land here)
}

// internal/config/roles.go:8 (LANDED) — planner is the ONLY built-in
var defaultRoleTimeouts = map[string]time.Duration{
	"planner": 480 * time.Second,
}
```

**CRITICAL asymmetry (the whole item's risk):** the planner is the ONLY role with a built-in timeout.
- `ResolveRoleTimeout("planner", cfg)` → 480s when no per-role override (NOT cfg.Timeout!).
- `ResolveRoleTimeout("stager"|"message"|"arbiter", cfg)` → cfg.Timeout when no per-role override (no built-in).

So the wiring is **behavior-preserving for stager/message/arbiter** but a **behavior CHANGE for the planner**
(was `deps.Config.Timeout` = 120s global default; now 480s built-in). This is INTENTIONAL (PRD §9.15/§16.1:
"the planner needs more time than message/stager/arbiter"; P1.M2.T2.S1 flipped the global 480s→120s
specifically so the planner built-in 480s would be LONGER than the global 120s). The item description
confirms: OUTPUT "The planner gets 480s by default (built-in), others get 120s (global)."

## 1. The exact 4 wiring sites (verified by grep — exact line numbers)

Each role file has the IDENTICAL twin structure: a `config.ResolveRoleModel("<role>", deps.Config)` line,
then later a `provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)` line. The change is
mechanical: add `<role>Timeout := config.ResolveRoleTimeout("<role>", deps.Config)` right after the
ResolveRoleModel line, and swap the Execute arg `deps.Config.Timeout` → `<role>Timeout`.

| File | ResolveRoleModel line | Execute line (the swap) | role string |
|------|----------------------|--------------------------|-------------|
| `internal/decompose/planner.go` | 65 (`_, mdl, rsn := config.ResolveRoleModel("planner", deps.Config)`) | 124 (`out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)`) | "planner" |
| `internal/decompose/stager.go` | 96 (`_, mdl, rsn := config.ResolveRoleModel("stager", deps.Config)`) | 110 (`if _, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose); …`) | "stager" |
| `internal/decompose/message.go` | 127 (`_, mdl, rsn := config.ResolveRoleModel("message", deps.Config)`) | 155 (`out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)`) | "message" |
| `internal/decompose/arbiter.go` | 82 (`_, mdl, rsn := config.ResolveRoleModel("arbiter", deps.Config)`) | 100 (`out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)`) | "arbiter" |

Each Execute is the ONLY `deps.Config.Timeout` reference in its file (one swap per file). The
`provider.Execute` signature (verified): `func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose)`.

## 2. The S1 canonical pattern (LANDED in generate.go — the template to clone)

P1.M3.T1.S1 is **Complete** and LANDED in `internal/generate/generate.go`. It wired the MESSAGE role
on the single-commit path using EXACTLY this pattern — clone it verbatim for each decompose role:

```go
// generate.go:264 (ResolveRoleModel — pre-existing)
_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
// generate.go:267-269 (the S1 addition — resolve the timeout twin, co-located)
msgTimeout := config.ResolveRoleTimeout("message", cfg)
...
// generate.go:340 (the S1 swap — cfg.Timeout → msgTimeout)
out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
```

The decompose wiring is the SAME shape: resolve the timeout RIGHT AFTER the ResolveRoleModel line
(co-located — the "model/timeout twins" idiom), then pass it to Execute. Naming: `<role>Timeout`
(plannerTimeout/stagerTimeout/messageTimeout/arbiterTimeout) — matches the item description + S1's
`msgTimeout`. Each is a LOCAL var in the role function (no package-level state).

## 3. NO new imports for any source file

All 4 role files already import `github.com/dustin/stagecoach/internal/config` (for ResolveRoleModel).
`ResolveRoleTimeout` returns a `time.Duration` VALUE — the local var holds it and passes it to Execute.
You do NOT need to `import "time"` to hold/pass a `time.Duration` (only to CONSTRUCT one via
`time.Second`/`time.Millisecond`, which the source files never do — only the test files do, and they
already import `"time"`). So: **zero import changes across all 4 source files.** (Verified each file's
import block: planner/stager/arbiter import {context, errors, fmt, config, …}; message adds
{generate, git, hooks, prompt}. None import "time" — none need to.)

## 4. The pinning-test inventory — ONLY `TestCallPlanner_Timeout` breaks

Every decompose role has an existing `_Timeout` test that sets `cfg.Timeout` small + stub sleeps 2000ms
+ asserts the role's distinct failure semantic. After the wiring:
- **stager/message/arbiter tests stay GREEN** (no built-in → ResolveRoleTimeout returns cfg.Timeout →
  the small cfg.Timeout still bounds Execute; behavior-preserving by default).
- **`TestCallPlanner_Timeout` (planner_test.go:337) BREAKS**: it sets `cfg.Timeout = 100ms` expecting
  that to bound the planner. After the wiring, the planner uses ResolveRoleTimeout("planner", cfg) =
  480s built-in (cfg.Roles is nil in that test) → stub sleeps 2000ms → NO timeout → callPlanner returns
  validMultiJSON success → the test's `if err == nil { t.Fatal }` fires.

The full inventory (verified by grep `cfg.Timeout = ` in internal/decompose/*_test.go):

| Test (file:line) | Role exercised | Built-in? | After-wiring status |
|------------------|-----------------|-----------|---------------------|
| `TestCallPlanner_Timeout` (planner_test.go:345) | planner | **YES (480s)** | **BREAKS — must fix** |
| `TestStageConcept_Timeout` (stager_test.go:146) | stager | no | GREEN (canary) |
| `TestGenerateMessage_Timeout` (message_test.go:194) | message | no | GREEN (canary) |
| `TestRunArbiter_TimeoutNull` (arbiter_test.go:220) | arbiter | no | GREEN (canary) |
| `TestResolveArbiter_RescueErrorPropagation` (chain_test.go:428) | message (via resolveArbiter→generateMessage) | no | GREEN (canary) |

**The fix for `TestCallPlanner_Timeout`** (the one mandatory test edit): move the small timeout from
the global to the per-role field, and set the global LARGE so the test becomes a behavioral PROOF that
the per-role (not global) bounds the planner Execute:
```go
cfg := config.Defaults()
cfg.Timeout = 30 * time.Second                                   // LARGE — would NOT time out vs 2000ms sleep
cfg.Roles = map[string]config.RoleConfig{
    "planner": {Timeout: 100 * time.Millisecond},                // per-role SMALL → times out (proves ResolveRoleTimeout bounds Execute)
}
```
This both FIXES the test (planner now times out at 100ms via the per-role override) AND proves the
wiring. `config.RoleConfig{Timeout: time.Duration}` is the struct (config.go:38; Timeout is a field).
`config.Defaults()` returns Roles==nil, so the test must initialize the map (above).

## 5. The behavioral-proof tests (clone S1's `TestCommitStaged_MessageRoleTimeout` pattern)

To PROVE the wiring for the 3 non-planner roles (stager/message/arbiter), add ONE new test per role that
clones the existing `_Timeout` test and FLIPS which field is small (global LARGE + per-role SMALL).
`config.Defaults().Timeout` is 120s — set it to 30s (clearly > 2000ms stub sleep) and the per-role to
100ms. If the role still times out, ResolveRoleTimeout("<role>") (the per-role 100ms), not cfg.Timeout
(30s), reached Execute. The existing `_Timeout` tests (cfg.Timeout small) stay as the
behavior-preserving-by-default canaries.

The distinct failure semantics per role (verified by reading each existing `_Timeout` test — these are
the assertions the new tests reuse verbatim):

| Role | Failure semantic on timeout | Assertion in the test |
|------|------------------------------|----------------------|
| planner | `ErrPlannerFailed` (non-rescue) | `errors.Is(err, ErrPlannerFailed)` + `errors.Is(err, context.DeadlineExceeded)` |
| stager | `ErrStagerFailed` (non-rescue) | `errors.Is(err, ErrStagerFailed)` + `errors.Is(err, context.DeadlineExceeded)` |
| message | `*generate.RescueError{Kind: ErrTimeout}` (rescue, exit 124) | `errors.As(err, &re)` + `re.Kind == generate.ErrTimeout` |
| arbiter | graceful `nil` error + `Target == nil` (the §13.6.5 "when in doubt, null") | `err == nil` + `out.Target == nil` |

So the new tests:
- `TestStageConcept_PerRoleTimeout` (stager_test.go) — clone TestStageConcept_Timeout, flip global/per-role.
- `TestGenerateMessage_PerRoleTimeout` (message_test.go) — clone TestGenerateMessage_Timeout, flip.
- `TestRunArbiter_PerRoleTimeoutNull` (arbiter_test.go) — clone TestRunArbiter_TimeoutNull, flip.
- (planner) `TestCallPlanner_Timeout` is FIXED IN PLACE (§4) — it becomes the planner behavioral proof.

## 6. The test harness (verified — reuse, don't reinvent)

The decompose tests use `internal/stubtest` to stub the provider:
- `bin := stubtest.Build(t)` — builds a stub agent binary (once per test).
- `stubtest.Manifest(bin, stubtest.Options{Out: "<json/text>", SleepMS: <n>})` — a bare-mode manifest
  whose stub sleeps SleepMS ms then prints Out. (Stager uses `tooledStubManifest(t, bin, opts)` — tooled mode.)
- `plannerDeps(t, repo, m)` / `messageDeps` / `stagerDepsWithConfig(t, repo, m, cfg)` / `arbDeps(t, repo, m)`
  — per-file helpers that build a minimal `Deps{Git, Config: config.Defaults(), Roles: RoleManifests{…}, Verbose: nil}`.
  Each test then overrides `deps.Config = cfg` to inject its timeout config.
- Repo fixtures: `initRepo`/`commitRaw`/`writeFile` (planner), `msgInitRepo`/`msgCommitRaw`/`msgWriteFile`/
  `msgStageFile`/`msgGitOut` (message), `stgInitRepo`/`stgCommitRaw` (stager), `arbInitRepo`/`arbCommits`
  (arbiter) — own per-file copies (the package's _test.go helpers aren't shared across files). REUSE the
  SAME fixture the cloned test uses; do not invent new ones.
- `time` is already imported by all 4 test files (they set `cfg.Timeout = … * time.Millisecond`).

## 7. Scope boundaries (no overlap with siblings)

- **P1.M2.T1.S1** (DONE) — provides `ResolveRoleTimeout` + `defaultRoleTimeouts`. My item CONSUMES them.
- **P1.M2.T2.S1** (DONE) — flipped the global default 480s→120s + fixed pinning tests for THAT change.
  It did NOT touch `TestCallPlanner_Timeout` (that test sets cfg.Timeout explicitly, overriding the
  default, so the global flip didn't affect it). The planner-built-in breakage is NEW to MY item (the
  planner still used `deps.Config.Timeout` until now) — so fixing `TestCallPlanner_Timeout` is mine.
- **P1.M3.T1.S1** (DONE) — wired the MESSAGE role on the SINGLE-COMMIT path (`generate.CommitStaged`).
  My item wires ALL 4 roles on the DECOMPOSE path (separate call sites). The decompose `generateMessage`
  (message.go:155) is a DIFFERENT Execute site from generate.go:340 — not redundant.
- **P1.M3.T1.S2** (Implementing, parallel) — wires the message role at multiturn/workdesc/hook. NO overlap
  with decompose. Different files entirely.
- **P1.M4.T1.S1** (planned) — tests for `ResolveRoleTimeout` ITSELF + config loading + the default change
  (unit tests in internal/config). NOT the consumption-wiring behavioral proofs (those are part of P1.M3,
  per the S1/S2 precedent). My item's tests are the decompose consumption proofs.
- **P1.M4.T2.S1** (planned) — README/docs sync. My item: DOCS none (internal wiring; contract says so).
- This item touches ONLY: the 4 role files (1-line resolve + 1-word swap each) + `TestCallPlanner_Timeout`
  (fix) + 3 new tests (stager/message/arbiter). NO edit to internal/config/*, generate.go,
  multiturn.go/workdesc.go/hook (S2's), root.go, or any PRD/task file.

## 8. Validation commands (verified against Makefile + existing tests)

```bash
go build ./...                                  # the 4 swaps link cleanly
go vet ./internal/decompose/...
gofmt -l internal/decompose/planner.go internal/decompose/stager.go internal/decompose/message.go internal/decompose/arbiter.go \
       internal/decompose/planner_test.go internal/decompose/stager_test.go internal/decompose/message_test.go internal/decompose/arbiter_test.go   # empty
go test ./internal/decompose/ -run 'Timeout' -race -v     # the fixed planner test + 3 new + 4 existing canaries all PASS
go test ./internal/decompose/ -race                        # full decompose regression (chain/roles/decompose tests green)
make test                                                  # full race suite
make lint                                                  # golangci-lint
git status --porcelain                                     # ONLY the 4 role files + their 4 test files
```

`internal/decompose` is NOT in the coverage-gate list (Makefile:77 gates `internal/{git,provider,generate,config}`
only) — no coverage-threshold pressure. (internal/generate IS gated, but my item does not touch generate.go.)

## 9. The "planner gets 480s by default" behavior change — confirm it is desired, not a bug

Re-reading ResolveRoleTimeout's godoc (roles.go): "The planner is the ONLY role with a shipped built-in
timeout (480s)… A non-zero cfg.Roles[role].Timeout ALWAYS wins — even for the planner." And the PRD §9.15
FR-R7 + §16.1: per-role timeouts, planner longer. P1.M2.T2.S1's whole PURPOSE was to set the global to
120s so the planner's 480s built-in would be meaningfully longer. So the planner's default going from
120s (old `deps.Config.Timeout`) to 480s (new ResolveRoleTimeout) is the DESIGNED behavior — this item
REALIZES it at the planner Execute site. The test fix (§4) is the only consequence.
