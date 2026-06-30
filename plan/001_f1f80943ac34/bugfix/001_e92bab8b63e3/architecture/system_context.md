# System Context — Stagehand v1.0 Bugfix Pass

> Synthesis of the four codebase recon reports (`seam_config_handoff.md`,
> `seam_dryrun.md`, `seam_provider_preflight.md`, `seam_config_and_autostage.md`).
> This is the authoritative starting point for downstream PRP agents.

## 1. What this changeset fixes

An end-to-end QA pass found **no data-integrity or crash bugs**. The snapshot-based atomic
commit / "stage-while-generating" invariant (PRD §13.4 / §18.1) holds. The 7 issues to fix are
**spec/contract deviations and silent no-ops**, all clustered on four code seams:

| Issue | Severity | Seam | File(s) |
|-------|----------|------|---------|
| 1 — `--config` ignored by default action | Major | config handoff | `pkg/stagehand/stagehand.go` (`resolveConfig`), `internal/cmd/default_action.go` |
| 2 — `--dry-run` skips dup-check/retry | Major | dry-run pipeline | `pkg/stagehand/stagehand.go` (`runPipeline`) |
| 3 — missing provider command → exit 3 (not 1) | Major | provider pre-flight | `pkg/stagehand/stagehand.go` (`buildDeps`) |
| 4 — `[generation] output/strip_code_fence` never applied | Minor | cfg→manifest bridge | `pkg/stagehand/stagehand.go` (`buildDeps`) |
| 5 — §19 repo-local notice printed twice | Minor | config handoff (same as #1) | `internal/config/file.go` (side effect) |
| 6 — `--dry-run` skips `write-tree` snapshot | Minor | dry-run pipeline (same as #2) | `pkg/stagehand/stagehand.go` (`runPipeline`) |
| 7 — "(0 files)" notice on clean tree | Minor | auto-stage UX | `internal/cmd/default_action.go` |

## 2. The central architectural seam: the config double-load

Everything in Issues 1 and 5 flows from ONE line in `pkg/stagehand/stagehand.go:resolveConfig`:

```go
cfgPtr, err := config.Load(ctx, config.LoadOpts{RepoDir: repoDir, Flags: nil})
```

- **`ConfigPathOverride` is omitted** → defaults to `""` → the `--config` value is dropped; the
  second `Load` re-discovers the global file instead of the user's `--config` file. **(Issue 1.)**
- **`Flags: nil`** → Layer-7 (CLI flag overlay) is skipped. Provider/model/timeout survive only
  because `runDefault` re-injects them via `Options` (the documented "Options-as-flag-relay"
  workaround at `default_action.go:130-134`).
- **A second `config.Load` runs at all** → Layer-3 `loadRepoLocalConfig()` executes again → the
  §19 provider-redirect notice prints a second time. **(Issue 5.)**

The FIRST `config.Load` (in `internal/cmd/root.go` `PersistentPreRunE`) is correct — it passes
`ConfigPathOverride: flagConfig` and `Flags: cmd.Flags()`. But `runDefault` calls
`stagehand.GenerateCommit`, which **re-loads from scratch** and discards the CLI-resolved `cfg`.

**Data flow:**
```
CLI ──► root.go PersistentPreRunE ──► config.Load(--config honored, notice #1) ──► loadedCfg (correct)
                                                                                          │
        default_action.go runDefault ◄── Config() ─────────────────────────────────────────┘
                │
                └─► stagehand.GenerateCommit(Options{Provider,Model,Timeout,DryRun,Verbose,VerboseOn})
                                │  (Options has NO Config/ConfigPath field → --config lost)
                                ▼
                    resolveConfig ──► config.Load(RepoDir, Flags:nil)  ◄── BUG (notice #2, --config dropped)
```

## 3. The buildDeps chokepoint (Issues 3 & 4)

`buildDeps(cfg config.Config, repoDir string) (generate.Deps, error)` is the **single point** where
the provider `Manifest` is resolved from the registry. It feeds BOTH code paths
(`generate.CommitStaged` for the common path, `runPipeline` for dry-run/SystemExtra).

- **Issue 3:** `buildDeps` resolves & validates the manifest but never checks the command is on
  `$PATH`. The missing binary is only detected later inside `provider.Execute`'s `cmd.Start`
  (`executor.go:64`), **after** `WriteTree` already froze a snapshot (Step 3) and armed rescue.
  The start-error is neither `DeadlineExceeded` nor `Canceled`, so it falls through to
  `ParseOutput` → retries `MaxDuplicateRetries+1` times → returns `*RescueError{TreeSHA: non-empty}`
  → **exit 3** + the full §18.3 recovery block + a dangling tree object. The intended behavior
  (PRD §18.2 / §13.5) is a plain **exit 1** fail-fast *before* the snapshot.
  **Fix:** add `reg.IsInstalled(m)` (which calls `exec.LookPath(m.DetectCommand())`) right after
  `m.Validate()` and before `return generate.Deps{...}`. `Registry.IsInstalled` already exists
  (`registry.go:72-83`) and is tested.

- **Issue 4:** `cfg.Output` / `cfg.StripCodeFence` are populated by every loader (defaults, file,
  git-config) and asserted in `config/*_test.go`, but **no production code consumes them** —
  `provider.ParseOutput` reads only `deps.Manifest.Output/.StripCodeFence`. The cfg→manifest bridge
  for these two fields is simply missing in `buildDeps`.
  **Fix (chosen — see decisions.md):** apply `cfg.Output`/`cfg.StripCodeFence` onto the resolved
  manifest in `buildDeps`, after `m.Validate()`, before the pre-flight/return.

## 4. The dry-run divergence (Issues 2 & 6)

`GenerateCommit` dispatches:
- `!DryRun && SystemExtra==""` → `generate.CommitStaged` (frozen, fully-tested real loop).
- `DryRun || SystemExtra!=""` → `runPipeline` (a self-contained mirror).

Inside `runPipeline`, the dry-run branch is a **third, degraded implementation**:
1. `WriteTree` is gated behind `if !dryRun` → no snapshot in dry-run. **(Issue 6.)**
2. The dry-run branch is a **single generation attempt**: `Execute` → `ParseOutput` → if `!ok`,
   `return errors.New("dry run: model produced no valid message")`. No FR29 parse-retry, no FR30-33
   duplicate rejection, no bounded loop. **(Issue 2.)**
3. On timeout, dry-run returns the bare `ErrTimeout` sentinel (NOT a `*RescueError`, no TreeSHA),
   which `TestGenerateCommit_Timeout/dryrun` (`stagehand_test.go:224-250`) currently **locks in**.

The exact same loop body (generate→parse→dedupe with `rejected []string`, `parseFail`,
`retryInstr`, `IsDuplicate`) already exists in `runPipeline` for the SystemExtra **commit** path
(lines ~281-339). The fix is to take the snapshot unconditionally and run that loop for dry-run too,
then skip ONLY the `CommitTree`/`UpdateRefCAS` tail. The `signal.SetSnapshot`/`signal.SetCandidate`
plumbing is already present.

## 5. Dependency ordering between fixes

```
P1.M1 (config handoff: Issues 1 & 5)  ──┐
   fixes the cfg that buildDeps receives │
                                         ▼
P1.M4.T1 (Issue 4: apply cfg→manifest)  ── depends on M1 (so --config'd generation knobs are honored)

P1.M2 (Issue 3: pre-flight)  ── same function (buildDeps) as Issue 4; sequence after M1 to avoid rebase churn
P1.M3 (Issues 2 & 6: dry-run) ── independent of buildDeps; touches runPipeline only
P1.M4.T2 (Issue 7: auto-stage notice) ── independent; touches default_action.go only
```

**Issue 5 needs no separate implementation task** — it is eliminated by Issue 1's fix (the second
`Load` is no longer performed). It gets a regression assertion in M1's test subtask (notice count == 1).

## 6. Testing posture (TDD is implicit — per SOW §3, every subtask ships its failing test first)

Current gaps the breakdown must close:
- **No test passes `--config /custom.toml`** through the CLI default action.
- **No test asserts the §19 notice count** in the full CLI seam.
- **No `CommitStaged`/`GenerateCommit`-level test** for the missing-provider-command path (only an
  isolated `Execute`-level test at `executor_test.go:128-137`).
- **No dry-run dup-retry / parse-retry / snapshot-creation test.**
- **No cfg→manifest override test** asserting `ParseOutput` honors `cfg.Output`/`cfg.StripCodeFence`.
- `TestRunDefault_NothingStaged_FR17` does not assert the absence of the "(0 files)" notice.

Test harness primitives that already exist and should be reused:
- `internal/stubtest` — stub-agent scripting (multi-line outputs, sleeps for timeout).
- `pkg/stagehand/stagehand_test.go` `setupTestRepo` — temp repo + repo-local `.stagehand.toml`.
- `internal/cmd/default_action_test.go` `setupStubRepo` — full CLI seam via `rootCmd.SetArgs`.
- `internal/config/file.go` package-level `noticeOut io.Writer` — swappable notice sink for tests.

## 7. Documentation surface touched (drives the Mode-A / Mode-B plan, SOW §5)

Per-file (Mode A, ride with the work):
- `docs/cli.md` — `--config` flag table (Issue 1), dry-run description (Issues 2/6), exit-code table
  / failure modes (Issue 3: "agent missing → exit 1").
- `docs/configuration.md` — `[generation]` output/strip_code_fence (Issue 4), `--config`/STAGEHAND_CONFIG.
- `docs/how-it-works.md` — failure-modes table (Issue 3: agent-missing pre-generation exit 1), dry-run
  pipeline fidelity (Issues 2/6).

Changeset-level (Mode B, final task):
- `README.md` — config-precedence blurb, `--dry-run` feature line, "configure your agent".
- `internal/cmd/config.go` `exampleConfigTemplate` — the `[generation] output`/`strip_code_fence`
  comment lines (kept, now accurate; or note per-provider).
