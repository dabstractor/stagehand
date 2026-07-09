# Research: P1.M2.T6.S1 — `--verbose` hint for `--model`/`--provider` shadowing (Issue 6)

All claims verified against current source with exact line numbers (repo at
`/home/dustin/projects/stagecoach`, 2026-07-09).

## 1. Issue 6 summary (PRD h3.5)

`--model X` sets the GLOBAL default (`cfg.Model`). A `[role.message] model = "Y"` in config takes
precedence (FR-R3) for the message role, so the rendered/generation command uses "Y", NOT "X" — and
the bare "X" is never even validated (FR-R5b). This is **correct per spec** but is a UX footgun: no
error, no warning, wrong model used silently. Suggested fix (PRD): a one-line `--verbose` hint when
an explicit `--model`/`--provider` is shadowed by a per-role override. **No behavioral change.**

## 2. The shadowing is REAL (not just a label cosmetic) — verified

`runDefault` (internal/cmd/default_action.go) computes `labelProvider`/`labelModel` for the progress
label + up-front validation, but the ACTUAL generation also resolves the per-role message config:

- `pkg/stagecoach/stagecoach.go:348` — `buildDeps`: `msgProvider, _, _ := config.ResolveRoleModel("message", cfg)`
  selects the provider MANIFEST from the message role (per-role provider beats `cfg.Provider`).
- `pkg/stagecoach/stagecoach.go:502` — `_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)`
  resolves the model used at the Render call site (per-role model beats `cfg.Model`).
- `internal/config/roles.go:34-57` — `ResolveRoleModel`: `if rc.Model != "" { model = rc.Model }`
  then `if model == "" { model = cfg.Model }` — per-role wins, global is FALLBACK only.

So when `cfg.Roles["message"].Model` is set, the user's `--model` value is genuinely ignored by the
generation pipeline. ⇒ A hint is warranted and the detection condition must reflect the RESOLVED
value differing from the explicit flag value.

## 3. CRITICAL CORRECTION to the architecture research

`architecture/research_provider_verbose.md` (Issue 6 section) says: *"Verbose sink available at
default_action.go:399."* **This is wrong for `runDefault`.** Line 399 (`verbose := ui.NewVerbose(stderr, cfg.Verbose)`)
is inside **`runDecompose`**, a DIFFERENT function. **`runDefault` does NOT create a `*ui.Verbose`
sink.** Confirmed by `grep -n "NewVerbose" internal/cmd/default_action.go` → only one hit (line 399,
inside `runDecompose`).

⇒ **The fix must CREATE a verbose sink in `runDefault`** before emitting the hint:
`verbose := ui.NewVerbose(stderr, cfg.Verbose)` — exactly mirroring runDecompose:399. `VerboseWarn`
already handles the nil/off/nil-writer no-op guard (verbose.go:101-108), so it is silent when
`--verbose` is off (no stderr noise for normal users — the key UX requirement).

## 4. Detection of an EXPLICIT flag (the footgun case)

We must warn ONLY when the user EXPLICITLY passed `--model`/`--provider` (the flag), not when
`cfg.Model`/`cfg.Provider` came from `[defaults]`/env (that is intentional config). The idiomatic
check is the flag's "changed" bit:

- `cmd.Flags().Changed("model")` / `cmd.Flags().Changed("provider")` — `cmd *cobra.Command` is the
  first arg of `runDefault(cmd *cobra.Command, args []string)`. Pattern already used in the package:
  `internal/cmd/hookexec.go:56` (`cmd.Flags().Changed("edit")`), `internal/cmd/hook.go:94`.
- When `Changed("model")` is true, `config.Load` wrote the flag value into `cfg.Model` (flag is the
  highest layer), so `cfg.Model` == the explicit `--model` value at this point.

## 5. The exact detection condition (chosen)

After `roleProvider, roleModel, _ := config.ResolveRoleModel("message", *cfg)` (default_action.go:176):

- Model shadow: `cmd.Flags().Changed("model") && roleModel != cfg.Model`
- Provider shadow: `cmd.Flags().Changed("provider") && roleProvider != cfg.Provider`

Why value-compare (`roleModel != cfg.Model`) rather than just `cfg.Roles["message"].Model != ""`:
- Under `Changed("model")`, `cfg.Model` is the explicit flag value. `roleModel` is what WILL be used
  (per-role if set, else the global). They differ IFF a per-role message model is set AND it is not
  identical to the flag value. This precisely captures "your explicit --model is being ignored" and
  AVOIDS a false positive when the user passes the SAME model as the per-role config (no real surprise).
- `roleModel != cfg.Model` ALSO implies the per-role model is set (given the flag is changed), so it
  is both necessary and sufficient. Safe map access: `cfg.Roles` may be nil/absent → `ResolveRoleModel`
  just falls back to global → `roleModel == cfg.Model` → no warning (correct).

Edge cases handled correctly:
- `--model X`, no per-role model → `roleModel = cfg.Model = X` → no warning. ✓
- `--model X`, `[role.message].model = Y` → `roleModel = Y ≠ X` → warning. ✓ (the PRD scenario)
- `--model X`, `[role.message].model = X` (same) → `roleModel = X = cfg.Model` → no warning. ✓
- No `--model`, `[role.message].model = Y` → `Changed("model")` false → no warning. ✓ (intentional config)

## 6. VerboseWarn contract (do not change)

`internal/ui/verbose.go:101-108`:
```go
func (v *Verbose) VerboseWarn(msg string) {
	if v == nil || v.w == nil || !v.on { return }
	fmt.Fprintln(v.w, "DEBUG: "+msg)
}
```
Format = `DEBUG: <msg>\n`. Silently no-op when off. Wording to use (mirrors PRD suggested fix):
- Model: `note: --model shadowed by [role.message].model; use --message-model to override`
- Provider: `note: --provider shadowed by [role.message].provider; use --message-provider to override`

## 7. Scope boundary

- The single-commit path (`runDefault`) uses ONLY the "message" role. This task is scoped to that
  path + the message role (per the subtask title: "...hint in default_action.go").
- The decompose path (`runDecompose`) resolves all four roles (planner/stager/message/arbiter) and
  has the same latent shadowing, but it is OUT OF SCOPE here (different function, four roles, more
  cases). Note as a future enhancement, do NOT implement it in this PRP.

## 8. Test design — why the stub provider makes clean tests possible

The stub provider is defined in tests via raw TOML (`[provider.stub]`) with `prompt_delivery="stdin"`,
NO `provider_flag`, NO `default_model`. `Manifest.ValidateModel` (manifest.go:136-154) only enforces
the FR-R5b slash rule when `*r.ProviderFlag != ""` — the stub's `ProviderFlag` resolves to `""`, so
`ValidateModel` returns nil for ANY model string. ⇒ Tests can set `[role.message] model = "<anything>"`
and pass `--model <other>` without hitting a validation error; the run reaches & emits the warning.

Existing test harness (internal/cmd/default_action_test.go): tests drive the FULL `Execute(context.Background())`
path with `rootCmd.SetArgs/SetOut/SetErr`, a real temp git repo (`setupStubRepo` / `setupStubRepoRaw`),
isolateHome, and the compiled stub binary. The hint lands in the stderr buffer (`errBuf`). Use
`--dry-run` so no commit is created (the warning is emitted in runDefault BEFORE GenerateCommit, so
`--dry-run` still reaches it) — simpler assertions, no commit-round-trip.

## 9. Mode A docs (ride with the work)

- `internal/cmd/root.go` ~L167: the `--model` flag `StringVar` description — add a SHORT precedence
  note (help text is terminal-width-wrapped; keep it terse).
- `docs/cli.md` ~L25 (`--model` row) + the §"Message-role resolution" note (~L120) — add a precedence
  gotcha: `--model` sets the global default; a `[role.<role>]` model wins for that role; use
  `--<role>-model` to override. A `--verbose` run now surfaces this as a hint.
- `docs/configuration.md` ~L241 (role-config section) — cross-reference the same precedence gotcha.

No change to PRD.md, tasks.json, prd_snapshot.md, or behavior/exit codes.
