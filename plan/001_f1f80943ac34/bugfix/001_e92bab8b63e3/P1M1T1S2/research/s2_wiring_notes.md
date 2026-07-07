# S2 Research Notes — Wire the CLI's loaded cfg through runDefault into Options

Scope: the **one-line wiring** that closes the CLI↔`pkg/stagecoach` config-handoff seam.
S1 (commit `13225e9`) already added `Options.Config *config.Config` and made
`resolveConfig` skip `config.Load` when `opts.Config != nil`. S2 sets that field from the CLI.

## 1. Current state verification (git + grep, 2026-06-30)

- `git log --oneline -3` → `13225e9 add resolved-config injection to bypass duplicate config load` (= S1, DONE).
- `grep -c "Config \*config.Config" pkg/stagecoach/stagecoach.go` → `1` (field present).
- `grep -c "opts.Config != nil" pkg/stagecoach/stagecoach.go` → `1` (resolveConfig branch present).
- `grep -c "Config:  cfg\|Config: cfg" internal/cmd/default_action.go` → `0` (**S2 NOT done** — confirms this is greenfield wiring).

## 2. The single edit locus — `internal/cmd/default_action.go`

`runDefault` resolves `cfg := Config()` at the top (line ~56) and currently calls (lines ~147-154):

```go
res, err := stagecoach.GenerateCommit(ctx, stagecoach.Options{
    Provider:  cfg.Provider,
    Model:     cfg.Model,
    Timeout:   cfg.Timeout,
    DryRun:    flagDryRun,
    Verbose:   stderr,
    VerboseOn: cfg.Verbose,
})
```

S2 adds ONE field to this literal: `Config: cfg,`. `cfg` is already a `*config.Config` in scope
(`Config()` returns `*config.Config` per `root.go:108`), so the type matches `Options.Config` exactly —
no `&cfg`, no deref. The remaining Provider/Model/Timeout/VerboseOn fields STAY (redundant-but-correct:
they re-assert the highest-precedence Options-override contract on top of the injected cfg; they are
what the standalone-library `Options.Config == nil` precedence story documents).

## 3. The comment that becomes stale — `default_action.go:130-134`

```go
// §3: re-apply the CLI-resolved provider/model/timeout (Layer-7 flags already applied by
// PersistentPreRunE) as Options — GenerateCommit re-loads config with Flags:nil, so opts is how the
// CLI flags take effect (opts override is highest precedence in resolveConfig).
```

After S2, `GenerateCommit` does **NOT** re-load config (it consumes `Options.Config`). The clause
"GenerateCommit re-loads config with Flags:nil" is now FALSE and must be rewritten to describe the new
injection path (one Load total, via PersistentPreRunE; --config honored; §19 notice fires once).
Behavior is unchanged by the comment edit; it is doc-of-code accuracy only.

## 4. Why this is SAFE — no existing test breaks

Confirmed via `grep -rn "repo-local config\|sets provider\|noticeOut" internal/ pkg/`:
- **No CLI test asserts on the §19 notice text/count.** Every notice assertion lives in
  `internal/config/{file_test.go,load_test.go}` and tests `loadRepoLocalConfig`/`Load` **directly**
  (counting one notice per `Load` call). None observe the cross-package double-call through
  `Execute()`/`runDefault` stderr.
- `internal/cmd/default_action_test.go` captures stderr into buffers but never asserts on the notice.
- `pkg/stagecoach/stagecoach_test.go` uses the repo-local `.stagecoach.toml` (Layer-3) + `Options{}`
  with `Config == nil`; S2 changes nothing about that path.
- `stagecoach.resolveConfig` keeps the `opts.Config == nil` branch byte-for-byte → standalone path unchanged.

Net: `go test -race ./...` stays green after the wiring (the bug it fixes was previously invisible to
the suite; S3 will add the assertions that make it visible).

## 5. Docs touch-points — `docs/cli.md` ([Mode A], "adjust prose only if wording implies subcommand-only")

- **Line 20** (`--config <path>` table row): `Path to a config file, overrides discovery` — accurate, NO CHANGE
  needed (it never claimed subcommand-only). The contract's "line ~20" is the *reference anchor*, not an edit.
- **Line 30** (prose after the global-flags table): currently
  `The --config flag is a path override for config-file discovery — it is not itself a Config field.
   The behavioral flags (...) have no env-var or git-config analogs.`
  This is accurate but SILENT on "every command, incl. the default action". Add ONE affirming sentence
  stating `--config` is honored by all commands including the default commit action (the bug fix).
- **Line 97** (`--config | STAGECOACH_CONFIG | —` map row): accurate — keep as-is.

The current wording does NOT imply subcommand-only, so the edit is a minimal positive affirmation
(1 sentence appended to the line-30 prose), NOT a rewrite.

## 6. Scope boundary (per plan_status)

- S2 = wiring (`Config: cfg`) + stale-comment fix + docs affirmation. **NO tests** (S3 owns:
  "Regression tests: --config honored by default action + §19 notice printed exactly once").
- S2 must NOT edit `internal/config/*`, `pkg/stagecoach/*` (S1 owns the field/branch — already done),
  `GenerateCommit` signature, or `tasks.json`/`PRD.md`/`prd_snapshot.md`.
