# Research: Map cfg → StagedDiffOptions at the 6 Production Call Sites

> **Purpose:** Pin the exact 6-call-site edits + the `*int` resolution logic for P1.M1.T2.S2.
> Built on S1 (LANDED — `StagedDiffOptions` has `TokenLimit int` / `DiffContext int` /
> `PromptReserveTokens int`) and P1.M1.T1 (COMPLETE — `config.TokenLimit int` + `config.DiffContext *int`).
> All code quoted from the live tree on 2026-07-04.

---

## 1. Baseline state (verified)

### 1.1 S1 LANDED — `StagedDiffOptions` (`internal/git/git.go:37-72`) already has the 3 fields
```go
type StagedDiffOptions struct {
    MaxDiffBytes     int
    MaxMDLines       int
    Excludes         []string
    BinaryExtensions []string
    // ... v2.1 group header ...
    TokenLimit          int  // FR3d; plain int; 0 = unset sentinel
    DiffContext         int  // FR3f; PLAIN int (the RESOLVED value); 0 is VALID (-U0)
    PromptReserveTokens int  // FR3i; 0 = unset (wired in M4.T1.S2, NOT this task)
}
```
The `DiffContext` doc comment explicitly states: **"the call site dereferences with a default-1
fallback before constructing this struct."** That sentence IS this task's central instruction.

### 1.2 P1.M1.T1 COMPLETE — config source fields
- `config.TokenLimit int` (`config.go:81`, plain int; `Defaults()` → 0). **Maps directly** (int→int).
- `config.DiffContext *int` (`config.go:82`, POINTER; `Defaults()` → `intPtr(1)`).
  - `nil` ⇒ user omitted the key ⇒ default 1 (-U1).
  - `*0` ⇒ explicit 0 (-U0 = changed lines only) — MUST be preserved, NOT collapsed to the default.
  - `*n` ⇒ explicit n.
  - The `*int` exists so `materialize`/`overlay` can distinguish "unset" (nil) from "explicit 0"
    (`file.go:226` `if g.DiffContext != nil`, `file.go:340` `if src.DiffContext != nil` — both guard
    on `!= nil`, NEVER `!= 0`). Verified by `TestMaterializeOverlay_DiffContext_TokenLimit` (file_test.go:814).

### 1.3 The type mismatch (THE gotcha)
`config.DiffContext` is `*int`; `StagedDiffOptions.DiffContext` is plain `int`. A literal
`DiffContext: cfg.DiffContext,` (as the contract shorthand wrote) is a **COMPILE ERROR** (`cannot use
cfg.DiffContext (value of type *int) as int value in struct literal`). The faithful realization
requires dereferencing the pointer with a default-1 nil-guard.

## 2. The 6 production call sites (exact current code — verified)

All 6 construct `git.StagedDiffOptions{...}` inline with the SAME 4 fields. TWO variable shapes:

### Shape A — local `cfg config.Config` (sites 1-3)
**Site 1 — `internal/generate/generate.go:163` (`CommitStaged` → `StagedDiff`):**
```go
diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
    MaxDiffBytes:     cfg.MaxDiffBytes,
    MaxMDLines:       cfg.MaxMdLines,
    BinaryExtensions: cfg.BinaryExtensions,
    Excludes:         deps.Excludes,
})
```
**Site 2 — `internal/hook/exec.go:104` (`Run` hook path → `StagedDiff`):** identical shape, `cfg`.
**Site 3 — `pkg/stagecoach/stagecoach.go:423` (`runPipeline` → `StagedDiff`):** identical shape, `cfg`.

### Shape B — `deps.Config` (sites 4-6)
**Site 4 — `internal/decompose/planner.go:69` (`callPlanner` → `TreeDiff`):**
```go
diff, err := deps.Git.TreeDiff(ctx, baseTree, tStart, git.StagedDiffOptions{
    MaxDiffBytes:     deps.Config.MaxDiffBytes,
    MaxMDLines:       deps.Config.MaxMdLines,
    BinaryExtensions: deps.Config.BinaryExtensions,
    Excludes:         deps.Excludes,
})
```
**Site 5 — `internal/decompose/message.go:71` (`generateMessage` → `TreeDiff`):** identical, `deps.Config`.
**Site 6 — `internal/decompose/decompose.go:608` (`runArbiterPhase` → `TreeDiff`):** identical, `deps.Config`.

> Note: sites 4-6 call `TreeDiff`, but they pass `git.StagedDiffOptions` (the SAME struct — there is no
> separate TreeDiffOptions). S1's 3 fields are therefore already present for all 6 sites. Confirmed by
> touchmap §2 + the live code.

## 3. The resolver (the ONE piece of real new logic)

Because `cfg.DiffContext` is `*int` and the nil→1 default must be identical at all 6 sites, the
dereference is centralized as a method on `Config` (the type whose field is being resolved). This is
DRY (one nil-rule, 6 callers), defensive (handles `config.Config{}` constructed without `Defaults()`
in tests/library use), and idiomatic (method-on-type). It is added to `internal/config/config.go`
alongside the existing `intPtr` helper (line 11).

```go
// DiffContextValue resolves the *int DiffContext to the plain int the git diff functions consume
// (StagedDiffOptions.DiffContext is a plain int — the RESOLVED value). Returns the FR3f default 1
// (-U1) when the user omitted the key (nil); a non-nil pointer (incl. *0) is returned verbatim, so
// an explicit 0 (-U0 = changed-lines-only) is preserved. Used by the 6 StagedDiffOptions call sites.
func (c Config) DiffContextValue() int {
    if c.DiffContext != nil {
        return *c.DiffContext
    }
    return 1
}
```
Value receiver — `Config` is passed by value at all 6 sites (`cfg config.Config` / `deps.Config`), so
`cfg.DiffContextValue()` and `deps.Config.DiffContextValue()` both compile. No existing Config methods
to clash with (config.go currently has only free functions: `boolPtr`/`strPtr`/`intPtr`/`Defaults`).

### The 6-site mapping (exact additions)
At each literal, append two lines (keep the existing 4 fields byte-identical):
- Shape A (sites 1-3): `TokenLimit: cfg.TokenLimit,` + `DiffContext: cfg.DiffContextValue(),`
- Shape B (sites 4-6): `TokenLimit: deps.Config.TokenLimit,` + `DiffContext: deps.Config.DiffContextValue(),`

`PromptReserveTokens` is NOT set (contract: leave zero; M4.T1.S2 wires it). Go's zero-value handles it.

## 4. Why this is behavior-free (the regression guarantee)

The 3 new `StagedDiffOptions` fields are **UNREAD** by the three diff functions (`StagedDiff`,
`TreeDiff`, `WorkingTreeDiff`) until M2 (`DiffContext` → `-U<n>`) and M4 (`TokenLimit`/
`PromptReserveTokens` → gate + water-fill). S1 landed them as unread seam-threaders. So populating
`TokenLimit`/`DiffContext` at the 6 literals changes ZERO diff output — the values flow into the
struct and die there (unread). Therefore:
- Every existing diff test (stagediff/treediff/workingtreediff golden fixtures, ~57 test literals)
  passes UNCHANGED.
- `go test ./internal/{generate,hook,decompose}` + `./pkg/stagecoach` + `./internal/git` all green.
- The only NEW logic is `DiffContextValue` (nil→1, *0→0, *n→n) — a focused 3-case unit test covers it.

## 5. Scope boundaries (do NOT do)

- Do NOT set `PromptReserveTokens` at any site (M4.T1.S2 owns it; leave zero).
- Do NOT edit the 3 diff functions / `Git` interface / `StagedDiffOptions` struct (S1/M2/M4 territory).
- Do NOT add `-M`/`-U<n>` flags, numstat skeleton, index-strip, or water-fill (M2/M3/M4).
- Do NOT add a `config.DiffOpts()` bridge function that returns the whole struct (touchmap §4 flags it
  as an OPTIONAL future refactor; the contract scoped THIS task to mapping the 2 fields at 6 literals).
- Do NOT touch `internal/config` materialize/overlay/Defaults/git-config (P1.M1.T1 COMPLETE) — the only
  config edit is the additive `DiffContextValue` method.
- Do NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

## 6. Decisions log

| # | Point | Decision | Why |
|---|---|---|---|
| D1 | `DiffContext: cfg.DiffContext` (contract shorthand) is a type error — how to map? | Add `Config.DiffContextValue() int` resolver; map `DiffContext: <cfg>.DiffContextValue()` | `cfg.DiffContext` is `*int`; the field is plain `int`. S1's struct doc explicitly mandates "the call site dereferences with a default-1 fallback." Centralizing in one method is DRY + defensive + idiomatic. |
| D2 | Resolver as method vs free function vs inline ×6 | Method `func (c Config) DiffContextValue() int` | Method-on-type reads best at the call site (`cfg.DiffContextValue()`); Config is passed by value everywhere (value receiver works); avoids duplicating the nil-guard 6×. |
| D3 | Where does the resolver live? | `internal/config/config.go` (next to `intPtr`) | The `*int` semantics are config-owned; the resolver belongs with the type. Additive, non-breaking — doesn't disturb P1.M1.T1's completed materialize/overlay work. |
| D4 | TokenLimit mapping | Direct: `TokenLimit: cfg.TokenLimit` (and `deps.Config.TokenLimit`) | Both sides are plain `int`; 0 IS the unset sentinel (FR3d). No resolution needed. |
| D5 | Set PromptReserveTokens? | NO — leave zero | Contract: "Do NOT set PromptReserveTokens here; M4.T1.S2 sets it." Go zero-value suffices. |
| D6 | New test? | Yes — `TestDiffContextValue` (nil→1, *0→0, *3→3) | The resolver is the only new logic; a 3-case test pins the nil-default + explicit-0 preservation. The 6 mappings need no test (mechanical, fields unread → no behavior). |
| D7 | Two variable shapes (cfg vs deps.Config)? | Handle both explicitly in the task list | Sites 1-3 use `cfg`; sites 4-6 use `deps.Config`. The edit per site differs only in the receiver name. |
