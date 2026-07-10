# Research Findings — P1.M1.T1.S2 (Add timeout field-merge to overlay function)

## 1. The change (single branch in overlay's per-role loop)

`overlay(dst, src *Config)` (internal/config/file.go) merges src→dst field-by-field. Its per-role
loop does field-merge for Provider/Model/Reasoning (non-empty-string guards). S2 adds ONE branch
for Timeout after the Reasoning branch:

```go
for role, rc := range src.Roles {
    existing := dst.Roles[role]
    if rc.Provider  != "" { existing.Provider  = rc.Provider }
    if rc.Model     != "" { existing.Model     = rc.Model }
    if rc.Reasoning != "" { existing.Reasoning = rc.Reasoning }
    if rc.Timeout   != 0  { existing.Timeout   = rc.Timeout }   // ← S2 ADDS THIS LINE
    dst.Roles[role] = existing
}
```

## 2. The mirror pattern (proven in the SAME function)

The global `Config.Timeout` overlay guard uses `!= 0` (duration-non-zero-wins). S2's per-role branch
uses the IDENTICAL `!= 0` discipline. Verified current line (pre-S1): file.go:348-350:
```go
if src.Timeout != 0 {
    dst.Timeout = src.Timeout
}
```
Same for HookTimeout (file.go:376 `if src.HookTimeout != 0`).

## 3. CRITICAL gotcha: `!= 0` NOT `!= nil` (Timeout is a value type, not a pointer)

The codebase has TWO overlay disciplines:
- **`!= nil`** for POINTER types (*int DiffContext, *bool AutoStageAll/MultiTurnFallback) — nil=inherit,
  non-nil (incl. zero) = override. Needed because *0/*false are meaningful explicit values.
- **`!= 0` / non-empty** for VALUE types (int, string, time.Duration) — zero/empty = inherit, non-zero = override.

`RoleConfig.Timeout` is a plain `time.Duration` (int64 value type, from S1) — NOT `*time.Duration`.
So the guard is `!= 0`. Do NOT mirror the DiffContext `!= nil` pattern. Rationale: for timeout, `0`
ALWAYS means "inherit global" (FR-R7/S1 design) — there is no meaningful "explicit 0" — so a plain
Duration + `!= 0` is correct and sufficient (no need for the *Duration pointer dance).

## 4. Exact current line numbers (pre-S1) + the S1 drift

Pre-S1 (the tree I inspected):
- Per-role loop: `for role, rc := range src.Roles` → file.go:447
- Reasoning branch: file.go:455-456
- `dst.Roles[role] = existing` → file.go:458
- Global Timeout mirror: file.go:348-350

**S1 SHIFTS overlay DOWN**: S1 rewrites the materialize loop (file.go:313-317 `RoleConfig(frc)`) to
field-by-field construction + parseTimeout (≈+10-12 lines), so overlay and the per-role loop move DOWN
by ~10-12 lines post-S1. The item description's line refs (440-457 / ~446-455 / 399) are approximate.
→ ANCHOR ON STRUCTURE, not absolute line numbers. Grep for `for role, rc := range src.Roles` and the
`if rc.Reasoning != ""` branch; insert the Timeout branch immediately after it.

## 5. The test to mirror: `TestOverlayRolesFieldMerge` (file_test.go:587-625)

The FR-R3 regression guard. Structure (direct Config literals + `overlay(dst, src)`):
- dst has planner={agy, codex-2.5-pro}, message={pi, gpt-5.4-nano}
- src has planner={Model only}, arbiter={new role}
- Asserts: planner.provider SURVIVES (lower-layer not clobbered); planner.model OVERRIDDEN;
  arbiter added; message untouched.

S2 EXTENDS this pattern with timeout assertions. Key cases:
1. **Higher-layer timeout overrides lower-layer timeout** (non-zero-wins): dst planner.Timeout=300s,
   src planner.Timeout=480s → dst == 480s.
2. **Timeout field-merge does NOT erase provider/model/reasoning** (THE regression guard): src sets
   ONLY timeout → dst's provider/model/reasoning survive. (Adding the branch must not clobber siblings.)
3. **Zero/omitted timeout does NOT clobber** (the `!= 0` guard): dst planner.Timeout=300s, src planner
   omits timeout (Timeout==0) → dst stays 300s (higher layer inherits lower layer's timeout).
4. (Optional) src-only role with timeout is added; untouched dst role survives.

Add as a focused sibling `TestOverlayRolesFieldMerge_Timeout` (cleaner) OR extend the existing test.

## 6. Contract from S1 (what exists when S2 begins — treat as given)

- `config.RoleConfig.Timeout time.Duration` (0 = inherit) — S1 adds it (config.go after Reasoning).
- `fileRoleConfig.Timeout string` (toml `timeout`, functional) — S1 adds it.
- `materialize` returns `(*Config, error)`, parses each role's timeout via `parseTimeout` — S1.
- The overlay function COMPILES FINE without the Timeout branch (rc.Timeout just isn't read) — S1's
  verification_deltas.md:104-107 confirms: "overlay COMPILES FINE without it after this subtask (it
  just doesn't merge Timeout yet) — but a per-role timeout in a REPO file won't survive into the
  resolved Config until S2 lands."
- So S2 is a clean, additive one-line (+test) change on top of S1's merged tree.

## 7. Scope boundaries (do NOT do — sibling subtasks)

- **P1.M1.T2.S1-S3**: `setRoleTimeout` helper + `STAGECOACH_<ROLE>_TIMEOUT` env + `--<role>-timeout`
  flags + `stagecoach.role.<role>.timeout` git-config reading. These set timeout DIRECTLY (bypass
  overlay), like the other setRole* helpers. NOT S2.
- **P1.M2.T1.S1**: `ResolveRoleTimeout` + `defaultRoleTimeouts{planner:480s}`. NOT S2.
- **P1.M2.T2.S1**: global default 480s→120s + fix pinning tests. NOT S2.
- **P1.M3**: the 13 `provider.Execute` call sites switch to per-role resolved timeout. NOT S2.
- **P1.M4.T2**: docs. NOT S2 (item point 5: "DOCS: none — internal merge logic").

S2 = ONLY the overlay branch + its test. The overlay function is the SINGLE file/git-layer merge site
(env/flag bypass it via setRoleTimeout in T2; there is no second per-role merge — grep-confirmed).

## 8. Validation

Config-package change only. Gates: `go build ./...`, `go vet ./...`, `gofmt -l internal/config/`,
`make lint`, `go test ./internal/config/...` (incl. new timeout overlay test), `make test`. No external
libs, no new patterns — the `!= 0` discipline is already used 3× in overlay (Timeout, HookTimeout,
plus all the int fields).
