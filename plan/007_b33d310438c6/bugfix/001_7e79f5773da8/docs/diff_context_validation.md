# Research: config-layer diff_context range validation + docs (P1.M2.T1.S1, bugfix Issue 2)

Verified against the live codebase. Source of truth for the validation + docs.

## The bug (Issue 2, Minor)

`buildDiffArgs` (`internal/git/git.go:689`) silently clamps `opts.DiffContext` to 1 when `<0 || >3`. It is a
PURE function ("Pure function; no I/O."), so a warning there would break purity AND spam per-diff-invocation
(3 diff functions × per-call). The config layer accepts ANY integer with no diagnostic — a user setting
`diff_context = 5` gets `-U1` with zero feedback. PRD §9.1 FR3f mandates "integer 0–3".

## Chosen fix (system_context.md §3.3 RECOMMENDED): config-layer ERROR, single helper

Add `func validateDiffContext(dc *int) error` in `internal/config/load.go` (next to `validateFormat` at :431
and `validateTemplate` at :444 — the EXISTING validation-helper pattern), invoked inline from `Load()`.
system_context §3.2 lists two PRD options: (a) validate at config layer → return error (cleanest), (b) stderr
warning when clamping fires (lower-risk but pollutes a pure function). §3.3 recommends (a) as a single
helper called from load — "clear, early, testable error and avoids I/O-in-pure-function." This matches the
contract's PREFERRED option and the existing `validateFormat`/`validateTemplate` convention (NOT a new
`Config.Validate()` method — none exists today; grep confirms; the standalone helper is the established style).

## The *int 0-vs-unset semantics are LOAD-BEARING — preserve them

`Config.DiffContext` is `*int` (config.go:82). Three cases, only ONE is invalid:
- `nil` ⇒ unset ⇒ default 1 (-U1). VALID. (materialize leaves it nil; Defaults() sets intPtr(1); overlay
  propagates non-nil across layers.)
- `*0` ⇒ explicit 0 ⇒ -U0 (changed-lines-only). VALID. This is THE key row — pinned by
  `file_test.go::TestMaterializeOverlay_DiffContext_TokenLimit` ("explicit_0").
- `*1`/`*2`/`*3` ⇒ valid.
- `*v` where `v < 0` (e.g. -1) or `v > 3` (e.g. 4, 5) ⇒ OUT OF RANGE ⇒ ERROR.

So the check is `if dc != nil && (*dc < 0 || *dc > 3)`. MUST guard on `dc != nil` FIRST (a nil deref would
panic; and nil is the valid "unset" case). `*0` MUST pass — do NOT write `*dc < 1` or `!= 0`.

## Exact placement in Load() (internal/config/load.go)

`Load()` (load.go:76) applies layers (global file ~100, repo file ~123, git config ~138, env ~142, flags
~148), normalizes (~Commits==1⇒Single), runs v3 migration, then VALIDATES post-overlay:
`validateFormat(cfg.Format)` (~176) → `validateTemplate(cfg.Template)` (~179) → self-hosting provider guard
(~201) → `return &cfg, nil` (208). Insert the diff_context check RIGHT AFTER validateTemplate (the validation
cluster), before the provider guard:
```go
if err := validateDiffContext(cfg.DiffContext); err != nil {
    return nil, fmt.Errorf("diff_context: %w", err)
}
```
This runs on the FULLY-MERGED *int (covers file + git-config sources — the only ones that can set it; there
is NO env/flag path for diff_context: grep of loadEnv/loadFlags finds none). The git-config resolver
(`internal/config/git.go:214`) parses the integer but does NOT range-check — validation at end of Load is the
single chokepoint.

## The helper body (mirror validateFormat/validateTemplate shape)

```go
// validateDiffContext rejects an out-of-range diff_context (PRD §9.1 FR3f: integer 0–3). nil ⇒ unset ⇒
// valid (default 1 applied by DiffContextValue); *0 ⇒ valid (-U0, changed-lines-only); only *v<0 or *v>3
// is an error. Pure (no I/O) so it is unit-testable directly. Called from Load after all overlays merge.
func validateDiffContext(dc *int) error {
    if dc == nil {
        return nil
    }
    if v := *dc; v < 0 || v > 3 {
        return fmt.Errorf("must be in range [0,3]: got %d", v)
    }
    return nil
}
```

## Test (pure, table-driven — mirror TestMaterializeOverlay_DiffContext_TokenLimit)

`internal/config/file_test.go::TestMaterializeOverlay_DiffContext_TokenLimit` (line 814) is the load-bearing
proof of the *int semantics, using `intp := func(v int) *int { return &v }` and table subtests. Mirror it
for `validateDiffContext`. Place in `load_test.go` (validateDiffContext lives in load.go; next to any
validateFormat test) — or file_test.go if that's where the diff_context tests cluster. Cases:
- `nil` → no error (unset; the valid default-1 case)
- `intp(0)` → no error (THE key row: explicit 0 stays valid)
- `intp(1)`, `intp(2)`, `intp(3)` → no error
- `intp(4)` → error (boundary just over 3)
- `intp(5)` → error (the contract's example)
- `intp(-1)` → error (the contract's example)
Assert `*v<0 || *v>3` cases return a non-nil error whose text names the range/got-value; valid cases return nil.

The pure unit test is the anchor (no I/O, deterministic — matches the contract's "no git, no I/O" guidance).
A Load-level wiring test (write `diff_context = 5` to a t.TempDir config, assert Load errors wrapping
"diff_context") is OPTIONAL — add it only if mirroring an existing load_test.go temp-config+Load pattern;
it's not required because the pure test + the inline Load call (verified by `go test ./...` staying green)
suffice.

## Call sites UNCHANGED; buildDiffArgs clamp stays as belt-and-suspenders

The 6 production call sites already consume `cfg.DiffContextValue()` / `deps.Config.DiffContextValue()`:
generate.go:184, decompose.go:624, message.go:96, planner.go:85, hook/exec.go:128, stagecoach.go:442. After
validation, DiffContextValue() returns a value in [0,3], so `buildDiffArgs`' clamp (git.go:689) is NEVER hit
for config-loaded runs — leave it as a defensive guard for programmatic Config construction (which bypasses
Load/Validate). Do NOT touch the call sites, DiffContextValue, buildDiffArgs, materialize, or overlay.

## Docs (Mode A — rides with the work)

- `docs/configuration.md`:
  - ~107 (comment block): `# diff_context = 1 # 0 = changed-lines-only, 1 = one anchor (default), 3 = git default; FR3f`
    → append "; valid range 0–3 — out-of-range rejected at config load".
  - ~131 (defaults table): the `diff_context` row → append "(range 0–3)" to the source/notes cell.
  - ~147 (prose, the **`diff_context`** bullet): add "Valid range is 0–3; an out-of-range value is rejected
    at config load with a clear error."
- `internal/config/bootstrap.go:291` (template comment): `# diff_context = 1 # ...` → append
  "; valid 0–3 (out-of-range rejected at config load)".

## Scope boundary (no conflict)

- **P1.M1.T2.S1 (parallel)** is TEST-ONLY in `internal/git/difftokenlimit_test.go` (E2E Issue-1 truncation
  regression). This task is config-layer + docs → DIFFERENT files, ZERO overlap.
- This task: `internal/config/load.go` (add validateDiffContext + the Load call), `docs/configuration.md`
  (3 spots), `internal/config/bootstrap.go` (1 comment), + the pure unit test. No git.go, no call sites,
  no materialize/overlay, no buildDiffArgs.
