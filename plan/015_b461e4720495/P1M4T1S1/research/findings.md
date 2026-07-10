# P1.M4.T1.S1 — Unit tests for ResolveRoleTimeout + config loading + default change: findings

## §0 — THE HEADLINE: most of this task's contract is ALREADY DONE (co-implemented in P1.M1/P1.M2)

The implementation subtasks P1.M1 (config infra) and P1.M2 (resolution + default change) BUNDLED
their own test coverage as they landed. A grep of `internal/config/*_test.go` for `RoleTimeout` /
`_TIMEOUT` / `role.*timeout` / `setRoleTimeout` finds **~20 LANDED tests** that already cover the
contract's clauses (b)–(e) almost entirely. **Blindly writing the contract's named tests
(`TestEnvRoleTimeout`, `TestFlagRoleTimeout`, `TestFileRoleTimeout`, `TestOverlayRoleTimeout`,
`TestGitConfigRoleTimeout`, `TestResolveRoleTimeout`) would DUPLICATE coverage** (wasteful) and, if the
exact contract names were reused, risk nothing (they differ from the LANDED names) — but it is still the
wrong move. This task's real job is **GAP-FILLING + CONSOLIDATION**, not greenfield authoring.

The implementer MUST read this inventory before writing anything, or they will waste effort.

## §1 — COMPLETE INVENTORY of LANDED tests (DO NOT DUPLICATE — verified line numbers)

### roles_test.go (ResolveRoleTimeout — 6 tests, all LANDED in P1.M2.T1.S1)
| Test (line) | Covers |
|---|---|
| `TestResolveRoleTimeout_PerRoleOverride` (197) | planner per-role override 600s beats built-in 480s + global 120s |
| `TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal` (207) | planner → 480s built-in when Roles nil (cfg.Timeout=120s) |
| `TestResolveRoleTimeout_NonPlannerGlobalFallback` (217) | stager/message/arbiter → 120s global (loop over the 3) |
| `TestResolveRoleTimeout_FieldMergeTimeoutOnly` (228) | message {Provider:pi, Timeout:0} → 120s (Timeout 0 inherits) |
| `TestResolveRoleTimeout_UnknownRoleGlobalFallback` (238) | "palnner" typo → 120s |
| `TestResolveRoleTimeout_RolesNilGlobalFallback` (247) | Roles nil, message → 120s |

### load_test.go (env + flag — 6 tests, LANDED in P1.M1.T2)
| Test (line) | Covers |
|---|---|
| `TestSetRole_Timeout_LazyAllocAndFieldMerge` (324) | setRoleTimeout lazy-allocs nil Roles map + field-merge w/ setRoleProvider |
| `TestLoadEnv_PerRoleTimeout` (368) | STAGECOACH_PLANNER_TIMEOUT=480s + STAGECOACH_STAGER_TIMEOUT=300 (bare int) |
| `TestLoadEnv_PerRoleTimeout_BadValueErrors` (390) | malformed STAGECOACH_PLANNER_TIMEOUT → wrapped error |
| `TestLoadFlags_PerRoleTimeout` (520) | --planner-timeout 600s + --stager-timeout 300 + --planner-provider field-merge |
| `TestLoadFlags_PerRoleTimeout_MalformedIgnored` (548) | malformed --planner-timeout silently ignored (loadFlags no error return) |
| `TestLoad_PerRoleTimeout_FlagBeatsEnv` (1126) | FULL Load() path: flag 600s > env 480s |

### file_test.go (materialize + overlay — 4 tests, LANDED in P1.M1.T1.S1/S2)
| Test (line) | Covers |
|---|---|
| `TestLoadTOMLRoleTimeoutMalformed` (546) | malformed [role.planner].timeout → load-time error naming the key |
| `TestLoadTOMLRoleTimeoutValid` (571) | valid [role.planner].timeout=480s → loadTOML→materialize → Roles[planner].Timeout |
| `TestOverlayRolesFieldMerge_Timeout` (676) | overlay field-merge for Timeout (4 sub-cases: higher-wins / siblings-survive / zero-no-clobber / src-only-add) |
| `TestMaterializeRoleTimeout` (1282) | materialize parses [role.planner].timeout (table: duration/bare-int/2m/empty/omitted/malformed + two_roles planner/stager subtest) |

### git_test.go (layer 4 — 3 tests, LANDED in P1.M1.T2.S3)
| Test (line) | Covers |
|---|---|
| `TestLoadGitConfig_PerRoleTimeout` (262) | stagecoach.role.planner.timeout=600s + stager=300 (bare int) |
| `TestLoadGitConfig_PerRoleTimeout_BadValue` (291) | malformed → HARD error naming the key |
| `TestLoadGitConfig_PerRoleTimeout_FieldMergeViaOverlay` (315) | git Timeout merged onto file-layer provider (FR-R3) |

### config_test.go (default change — 1 test)
| Test (line) | Covers |
|---|---|
| `TestDefaults` (~30) | `c.Timeout != 120*time.Second` assertion (the P1.M2.T2.S1 global 480s→120s flip) |

## §2 — THE GENUINE GAPS (what the contract asks for that is NOT yet covered)

Cross-referencing the contract's clauses against §1's inventory:

### GAP 1 (roles_test.go — contract clause (a), the clearest gap): NO test sets a NON-ZERO
`cfg.Roles["message"].Timeout` and asserts `ResolveRoleTimeout("message")` returns it.
- The contract clause (a) explicitly requires: *"message with [role.message].timeout returns that."*
- §1 covers planner's per-role override (`_PerRoleOverride`) and message with Timeout==0
  (`_FieldMergeTimeoutOnly`), but NEVER a message per-role override (non-zero). The message role is the
  ONLY active role on the single-commit path, so this is the highest-value untested branch.
- → ADD `TestResolveRoleTimeout_MessagePerRoleOverride` (message {Timeout:90s} → 90s, beating the 120s global).

### GAP 2 (roles_test.go — CONSOLIDATION, mirroring the existing model-axis idiom): there is NO
"all canonical roles" table for timeout. `TestResolveRoleModel_AllCanonicalRoles` (line 108) IS the
canonical idiom for the model axis — a single table over `roleNames` proving the resolution matrix.
The timeout axis is scattered across 6 ad-hoc tests. A consolidated table is the missing twin and the
clearest "completion" artifact (one readable matrix: planner→override/built-in; others→global/override).
- → ADD `TestResolveRoleTimeout_AllCanonicalRoles` (table over roleNames: planner override 600s,
  stager override 60s, message/arbiter → 120s global). Proves the full matrix in one place.

### GAP 3 (roles_test.go — contract clause (e), explicit): the contract clause (e) wants
`ResolveRoleTimeout("planner", Defaults()) == 480s` verified explicitly as the "FR-R7 default change"
proof. §1's `_PlannerBuiltinBeatsGlobal` is FUNCTIONALLY identical (Defaults()+set Timeout=120s, since
Defaults().Timeout IS 120s), so the behavior is proven — but no test calls `ResolveRoleTimeout` with the
literal `Defaults()`. A one-liner makes the "default change verified" intent explicit + readable.
- → ADD `TestResolveRoleTimeout_PlannerDefaultFromDefaults` (Defaults().Timeout==120s assert + ResolveRoleTimeout("planner", Defaults())==480s).

### GAP 4 (file_test.go — contract clause (c), literal): `TestMaterializeRoleTimeout` tests planner
(table) + planner/stager (two_roles subtest). NO `[role.arbiter]` case. The contract clause (c) explicitly
names *"TestFileRoleTimeout — [role.arbiter] timeout='200s' parsed correctly."* materialize IS role-agnostic
(loops the Role map), so this is belt-and-suspenders — but the contract names arbiter (the 4th role)
explicitly, and pinning all 4 roles in materialize is cheap insurance.
- → ADD a focused `TestMaterializeRoleTimeout_ArbiterRole` (materialize [role.arbiter].timeout="200s" → 200s).

### NOT GAPS (already covered — do NOT add):
- Contract (b) "TestEnvRoleTimeout STAGECOACH_PLANNER_TIMEOUT" → covered by `TestLoadEnv_PerRoleTimeout`.
- Contract (b) "TestFlagRoleTimeout --stager-timeout 300s" → covered by `TestLoadFlags_PerRoleTimeout`.
- Contract (c) "TestOverlayRoleTimeout" → covered by `TestOverlayRolesFieldMerge_Timeout` (4 sub-cases).
- Contract (d) "TestGitConfigRoleTimeout stagecoach.role.planner.timeout" → covered by `TestLoadGitConfig_PerRoleTimeout`.
- Contract (e) "Defaults().Timeout==120s" → covered by `TestDefaults`.
- Contract (e) "ResolveRoleTimeout('planner',…)==480s" → covered (functionally) by `_PlannerBuiltinBeatsGlobal`; GAP 3 makes it explicit.

## §3 — The implementation under test (LANDED — read-only, do NOT modify)

- `internal/config/roles.go`: `ResolveRoleTimeout(role, cfg)` — per-role override (`cfg.Roles[role].Timeout != 0`)
  > built-in (`defaultRoleTimeouts["planner"]=480s`, planner-ONLY) > `cfg.Timeout`. `defaultRoleTimeouts`
  is `map{"planner": 480s}` (the ONLY built-in). Mirrors `ResolveRoleModel`'s per-role-then-global structure.
- `internal/config/config.go`: `RoleConfig{Provider, Model, Reasoning string; Timeout time.Duration}`;
  `Config.Roles map[string]RoleConfig`; `Defaults()` returns `Roles==nil` (assign the WHOLE map in tests,
  never index-assign into nil).
- `internal/config/load.go`: `roleNames = []string{"planner","stager","message","arbiter"}` (package-level,
  reusable in same-package tests — `TestResolveRoleModel_AllCanonicalRoles` loops it). `setRoleTimeout`,
  `parseTimeout` (accepts "480s" AND bare "480"), the env `_TIMEOUT` branch, the flag `-timeout` branch.
- `internal/config/file.go`: `fileRoleConfig{Timeout string}` + `materialize` (parses via parseTimeout) +
  `overlay` (non-zero-wins field-merge for Timeout).
- `internal/config/git.go`: `stagecoach.role.<role>.timeout` → `gitConfigGet` → `parseTimeout` →
  `setRoleTimeout` (loop over roleNames; the bare-int "300" form proves parseTimeout).

## §4 — Test-harness helpers (same-package `config` — reuse, do NOT re-invent)

- `Defaults()` — returns a `Config` with `Roles==nil`, `Timeout==120s`.
- `roleNames` — package-level `[]string{"planner","stager","message","arbiter"}` (loop it for all-roles tables).
- `newFlagSet(t)` (load_test.go:53) — a `*pflag.FlagSet` with ALL flags pre-registered, INCLUDING
  `fs.String(role+"-timeout", "", "")` for every role (line 65) — so a flag test just `fs.Set("stager-timeout","300")`.
- `writeTempTOML(t, body)` (file_test.go) — writes a TOML body to a temp file, returns the path.
- `materialize(fc, 0, 0)` — direct call (NOT loadTOML→overlay); the unit-test seam for the parse step.
- `overlay(dst, src)` — direct call; the field-merge seam.
- `t.TempDir()` + `initRepo(t, repo)` + `setGitConfig(t, repo, key, val)` (git_test.go) — temp git repo
  pattern for git-config tests (isolates `HOME` via `t.Setenv`).

## §5 — Parallel-execution + scope awareness

- **P1.M3.T2.S1** (parallel, decompose) edits `internal/decompose/{planner,stager,message,arbiter}.go` +
  their `_test.go` (FIXes `TestCallPlanner_Timeout`, adds 3 per-role behavioral proofs). DIFFERENT package
  (`internal/decompose`) — ZERO overlap with this item's `internal/config` tests. The contract's validation
  "go test ./internal/decompose/... passes" is the PARALLEL item's responsibility; this item only must NOT
  break it (pure-additive config tests can't).
- **P1.M4.T2.S1** (docs) owns the README + docs sync. Contract: "DOCS: none — tests are not user-facing docs."
- **P1.M1/P1.M2** (LANDED) own the source AND the ~20 co-implemented tests. This item CONSUMES both + fills gaps.
- This item touches ONLY test files in `internal/config/`. NO production code, NO decompose/generate edits.

## §6 — Validation

- Primary gate: `go test ./internal/config/... -race` green (the new tests + all 20 existing).
- Targeted: `go test ./internal/config/... -run 'ResolveRoleTimeout|MaterializeRoleTimeout' -race -v`.
- Shared gate (don't break): `go test ./internal/decompose/... ./internal/generate/... -race` (parallel item's).
- `go vet ./internal/config/...`; `gofmt -l <the 2 edited files>`; `make test`; `make lint`.
- NO-duplicate guard: `grep -c '^func TestResolveRoleTimeout\|^func TestMaterializeRoleTimeout'` — the new
  func names must be UNIQUE (no collision with the 20 existing).
- Scope guard: `git status --porcelain` == the 2 files (roles_test.go, file_test.go) — UNLESS the implementer
  also adds the message case to load/git tests (optional, see PRP).
