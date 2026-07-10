# Research: P1.M1.T2.S1 — setRoleTimeout helper + per-role env `_TIMEOUT` branch

**Scope**: Add `func (c *Config) setRoleTimeout(role string, d time.Duration)` to internal/config/load.go
+ a `_TIMEOUT` branch in the per-role env loop + unit tests. All line numbers verified against the
working tree (2026-07-10). S1's `RoleConfig.Timeout time.Duration` field is **ALREADY LANDED**
(config.go:42) — prerequisite met.

## 1. The two-part deliverable

**Part A — the helper (mirror setRoleReasoning at load.go:57-65 EXACTLY):**
```go
func (c *Config) setRoleTimeout(role string, d time.Duration) {
	if c.Roles == nil {
		c.Roles = make(map[string]RoleConfig)
	}
	rc := c.Roles[role]
	rc.Timeout = d
	c.Roles[role] = rc
}
```
The map-value-copy write-back idiom is **REQUIRED for Go maps** (you cannot take the address of a map
element, so `c.Roles[role].Timeout = d` does not compile — you must copy out, mutate, write back).
The three existing helpers (setRoleProvider:33, setRoleModel:45, setRoleReasoning:57) all use it.
`setRoleTimeout` differs ONLY in: takes `time.Duration` (not string), sets `rc.Timeout`. Setting timeout
does NOT clobber provider/model/reasoning (FR-R3 field-merge) — each setter touches one field.

**Part B — the env-loop branch (after the `_REASONING` branch, load.go:301-303):**
```go
if v, ok := os.LookupEnv(prefix + "_TIMEOUT"); ok && v != "" {
	d, err := parseTimeout(v)
	if err != nil {
		return fmt.Errorf("%s_TIMEOUT: %w", prefix, err)
	}
	cfg.setRoleTimeout(role, d)
}
```
- `prefix` is `STAGECOACH_<ROLE>` (e.g. `STAGECOACH_PLANNER`), so the error reads `STAGECOACH_PLANNER_TIMEOUT: <err>`
  — consistent with the global `STAGECOACH_TIMEOUT: %w` discipline (load.go:263).
- `parseTimeout(v)` (load.go:618) accepts BOTH `"480s"`/`"2m"` (Go duration) AND bare `"480"` (seconds).
- DIRECT-set via setRoleTimeout (bypasses overlay — env is layer 5, higher than file/git). This matches how
  the global STAGECOACH_TIMEOUT DIRECT-sets `cfg.Timeout` (load.go:265).

## 2. Exact current anchors (load.go — STABLE, no parallel work in this file)

| Symbol | Line | Notes |
|---|---|---|
| `var roleNames = []string{"planner","stager","message","arbiter"}` | 17 | the loop iterates this (4 roles) |
| `func (c *Config) setRoleProvider` | 33 | idiom source #1 |
| `func (c *Config) setRoleModel` | 45 | idiom source #2 |
| `func (c *Config) setRoleReasoning` | 57-65 | **the idiom to mirror 1:1**; INSERT setRoleTimeout after its closing brace |
| global `STAGECOACH_TIMEOUT` env case | 260-266 | **the parse+error+DIRECT-set mirror** |
| per-role env loop | 293-304 | INSERT the `_TIMEOUT` branch after `_REASONING` (301-303), before loop close (304) |
| `func parseTimeout(s string) (time.Duration, error)` | 618 | reuse; handles "480s" + "480" |

**Placement**: setRoleTimeout goes immediately after setRoleReasoning (load.go:65). The env-loop branch
goes immediately after the `if v, ok := os.LookupEnv(prefix + "_REASONING")` block.

## 3. COORDINATION WITH PARALLEL SIBLINGS (NO conflict)

- **P1.M1.T1.S1** (Implementing in parallel): touches `config.go` (RoleConfig struct) + `file.go`
  (materialize rewrite). Its `RoleConfig.Timeout time.Duration` field is ALREADY in the tree (config.go:42)
  — that's my only dependency, and it's satisfied. S1 does NOT touch load.go.
- **P1.M1.T1.S2** (Implementing in parallel): touches `file.go` (overlay per-role loop). Does NOT touch load.go.
- **This task (T2.S1)**: touches `load.go` (helper + env loop) + `load_test.go` ONLY.

=> NO file-level merge conflict. load.go line numbers are STABLE (neither sibling edits load.go). The only
cross-task dependency is the RoleConfig.Timeout field, which has landed.

## 4. Test patterns to mirror (load_test.go)

| Existing test | Line | What it proves | Mirror for setRoleTimeout / env loop |
|---|---|---|---|
| `TestSetRole_LazyAllocAndFieldMerge` | 292 | setRoleProvider lazily allocs nil map; setRoleModel does NOT erase Provider | setRoleTimeout lazily allocs; does NOT erase Provider/Model/Reasoning |
| `TestLoadEnv_PerRole` | 327 | STAGECOACH_PLANNER_PROVIDER/MODEL → cfg.Roles["planner"]; partial stager | STAGECOACH_PLANNER_TIMEOUT=480s → cfg.Roles["planner"].Timeout |
| `TestLoadEnv_BadTimeoutErrors` | 277 | STAGECOACH_TIMEOUT=abc → error containing "STAGECOACH_TIMEOUT" | STAGECOACH_PLANNER_TIMEOUT=abc → error containing "STAGECOACH_PLANNER_TIMEOUT" |
| `TestLoad_TimeoutViaEnvInteger` | 1824 | full Load, STAGECOACH_TIMEOUT=45 (bare int) → cfg.Timeout==45s | proves parseTimeout bare-int form works (extend the env test) |

**KEY assertions**:
- The env test reads `cfg.Roles["planner"].Timeout` (the per-role field), NOT `cfg.Timeout` (the global).
  These are DIFFERENT fields. The role→global fallback is P1.M2.T1's ResolveRoleTimeout, NOT this task.
- Test BOTH the duration form (`"480s"`) and the bare-int form (`"480"`) to prove parseTimeout is used
  (not time.ParseDuration, which rejects bare ints).
- Test ≥2 roles (planner + stager) to prove the loop applies to all roles, not just the first.
- The field-merge assertion: set `_TIMEOUT` AND `_PROVIDER` for the same role → both survive in
  `cfg.Roles[role]` (FR-R3).

## 5. Test cases to implement (all in load_test.go)

1. `TestSetRole_Timeout_LazyAllocAndFieldMerge` — mirror TestSetRole_LazyAllocAndFieldMerge:
   - `cfg := Config{}` (Roles==nil); `cfg.setRoleTimeout("planner", 480*time.Second)` → Roles non-nil,
     Roles["planner"].Timeout==480s.
   - then `cfg.setRoleProvider("planner","agy")` → Roles["planner"] has BOTH Timeout==480s AND Provider=="agy"
     (field-merge: setRoleTimeout must not have locked the role to timeout-only).
   - (reverse order too: setRoleProvider first, then setRoleTimeout → both survive.)
2. `TestLoadEnv_PerRoleTimeout` — mirror TestLoadEnv_PerRole:
   - `STAGECOACH_PLANNER_TIMEOUT=480s` → `cfg.Roles["planner"].Timeout == 480*time.Second`.
   - `STAGECOACH_STAGER_TIMEOUT=300` (bare int) → `cfg.Roles["stager"].Timeout == 300*time.Second` (parseTimeout bare form).
   - unset role (e.g. message) → `Roles["message"]` either absent OR `.Timeout==0` (no-op; presence-semantic).
   - field-merge: also set `STAGECOACH_PLANNER_PROVIDER=agy` → Roles["planner"].Provider=="agy" AND .Timeout==480s.
3. `TestLoadEnv_PerRoleTimeout_BadValueErrors` — mirror TestLoadEnv_BadTimeoutErrors:
   - `STAGECOACH_PLANNER_TIMEOUT=abc` → `loadEnv` returns error; `strings.Contains(err.Error(), "STAGECOACH_PLANNER_TIMEOUT")`.
   - (optional) confirm the error does NOT contain a bare "STAGECOACH_TIMEOUT" (no global-prefix confusion).

## 6. What this task does NOT do (scope fences)

- Does NOT touch file.go (S1 materialize / S2 overlay) — those handle the FILE layer's per-role timeout.
- Does NOT add `--<role>-timeout` CLI flags or the loadFlags branch (that's P1.M1.T2.S2).
- Does NOT add `stagecoach.role.<role>.timeout` git-config reading (NEW infrastructure — P1.M1.T2.S3).
- Does NOT add ResolveRoleTimeout / defaultRoleTimeouts / planner-480s default (P1.M2.T1).
- Does NOT change the global default 480s→120s (P1.M2.T2).
- Does NOT touch the 13 provider.Execute call sites (P1.M3) or docs (P1.M4.T2).

So after this task: `STAGECOACH_PLANNER_TIMEOUT=480s` is PARSED + STORED in `cfg.Roles["planner"].Timeout`,
but nothing CONSUMES it yet (resolution + call-site wiring land in P1.M2/P1.M3). The unit tests are the
proof of storage; behavior observation is downstream.

## 7. Validation commands

- `go build ./...` (consumes RoleConfig.Timeout — must compile).
- `go vet ./internal/config/...`.
- `gofmt -l internal/config/load.go internal/config/load_test.go` → empty.
- `go test ./internal/config/... -run 'SetRole_Timeout|PerRoleTimeout' -v`.
- `make test && make lint`.
- Grep guards: `grep -n 'func (c \*Config) setRoleTimeout' internal/config/load.go` (one hit); 
  `grep -n 'prefix + "_TIMEOUT"' internal/config/load.go` (one hit); scope-fence: no flag/git/resolution symbols added.
