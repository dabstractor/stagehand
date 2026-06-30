# Bug Fix Requirements

## Overview

End-to-end QA / bug-hunting pass against the Stagehand v1.0 implementation vs. `PRD.md`. The
implementation is in strong shape overall: it builds and vets cleanly, the **entire test suite
passes with `-race`**, and the product's most important property — the **snapshot-based atomic
commit / "stage-while-generating" invariant (PRD §13.4 / §18.1)** — was verified end-to-end with a
real git repo and a stub agent: a commit built from a frozen `write-tree` snapshot contains *only*
the files staged before generation, while files staged *during* generation remain staged for the
next run. Rescue (exit 3), timeout (exit 124), duplicate-retry-then-success, root-commit (unborn
repo), code-fence stripping, and JSON output parsing were all exercised and behave correctly.

The issues below are **not** data-integrity or crash bugs. They are **spec/contract deviations and
silent no-ops** found by driving the actual binary through the documented user journeys
(`--config`, `--dry-run`, a misconfigured provider, config-file `[generation]` tuning, repo-local
config). They center on one architectural seam: **the CLI default action delegates to
`pkg/stagehand.GenerateCommit`, which re-loads configuration from scratch instead of receiving the
already-loaded config**, and the dry-run code path is a *separate, simplified* implementation rather
than the real pipeline.

- **Build:** `go build ./...` clean; `go vet ./...` clean.
- **Tests:** `go test -race ./...` — all packages pass.
- **Binaries used:** `bin/stagehand` (rev `dev`) + `bin/stubagent`.

## Critical Issues (Must Fix)

None. No data corruption, no crashes, no broken data-integrity invariants. The happy path, rescue,
timeout, CAS-style atomicity, and root-commit paths all work correctly.

## Major Issues (Should Fix)

### Issue 1: The `--config` flag is silently ignored by the default commit action

**Severity**: Major
**PRD Reference**: §15.2 (`--config <path>` — "Path to a config file (overrides discovery)"), §16.1
layer resolution, §9.8 FR34, §12.8 (user-defined providers via config files).
**Expected Behavior**: `stagehand --config <file>` should read `<file>` as the global config layer
for **every** command, including the default commit action. In particular, a user-defined provider
declared in `[provider.<name>]` inside `<file>` must be usable with `--provider <name>`.
**Actual Behavior**: The `--config` flag works only for the `providers list` / `providers show`
subcommands. For the **default commit action**, a provider defined only in the `--config` file is
reported as `unknown provider` — *after* the CLI has already validated the provider and printed
`↳ Generating with <provider>…`. Root cause: `runDefault` (internal/cmd/default_action.go) calls
`pkg/stagehand.GenerateCommit`, whose `resolveConfig` re-runs `config.Load` with
`LoadOpts{ConfigPathOverride: ""}` (no `--config` override) and `Flags: nil`. The CLI-loaded
`cfg.Providers` map (which *did* honor `--config`) is discarded and never passed through. The
`STAGEHAND_CONFIG` *env var* does work, because `config.Load` reads it directly via `os.Getenv` on
every call — only the `--config` *flag* (held in the CLI-only `flagConfig`) is lost.
**Steps to Reproduce**:
```bash
# config.toml defines a [provider.stub] pointing at any command
echo "b" > b.txt && git add b.txt
# Subcommand path: WORKS — stub is detected and shown
stagehand --config config.toml providers list      # stub  ✓
stagehand --config config.toml providers show stub # full manifest printed
# Default action path: FAILS
stagehand --config config.toml --provider stub
# ↳ Generating with stub…
# stagehand: unknown provider "stub"        (exit 1)
# Same thing via the env var WORKS:
STAGEHAND_CONFIG=config.toml stagehand --provider stub --dry-run   # (exit 0)
```
**Suggested Fix**: Either (a) pass the already-resolved `*config.Config` (and the resolved manifest
`generate.Deps`) from the CLI into `GenerateCommit` instead of re-loading (eliminates the double
load entirely), or (b) add a `ConfigPath` / `Providers` field to `pkg/stagehand.Options` and forward
`flagConfig` through `runDefault` so `resolveConfig` honors it. Option (a) also fixes Issues 5 and
the duplicated side-effects of the double load.

### Issue 2: `--dry-run` does not run the duplicate-check / retry pipeline (FR49 violation)

**Severity**: Major
**PRD Reference**: §9.12 **FR49** ("run the full diff→snapshot→generate→parse→**duplicate-check**
pipeline, print the resulting message, but do not create the commit"); US9 (dry-run is the
"judge quality before trusting it" preview). Also P1.M4.T4.S1 contract.
**Expected Behavior**: `--dry-run` should preview the **exact** message a real commit would produce,
including the duplicate-rejection retry loop (FR30–FR33) and the parse-retry (FR29).
**Actual Behavior**: The dry-run path (`pkg/stagehand.runPipeline` with `dryRun=true`) performs a
**single generation attempt with no duplicate check and no retry**. Consequently `--dry-run` and a
real commit can return **different messages**: when the model's first attempt duplicates a recent
subject, dry-run shows the duplicate (which a real run would reject and retry past).
**Steps to Reproduce** (stub script: attempt 1 = duplicate of existing subject, attempt 2 = unique):
```bash
git commit -q -m "feat: init"           # repo history
# stub script:  line0="feat: init"  line1="feat: unique after retry"
stagehand --provider stub --dry-run      # prints:  feat: init          (the DUPLICATE)
stagehand --provider stub                # commits: feat: unique after retry
```
A new user evaluating quality via `--dry-run` judges a message that would never actually be
committed.
**Suggested Fix**: Make the dry-run path reuse the same bounded generate→parse→dedupe loop as
`generate.CommitStaged`, then stop *before* `commit-tree`/`update-ref`. (See also Issue 6: dry-run
should still take the `write-tree` snapshot per FR49's "full … pipeline".)

### Issue 3: A missing/uninstalled provider command triggers the rescue path (exit 3) instead of the documented pre-generation fail-fast (exit 1)

**Severity**: Major
**PRD Reference**: §18.2 failure table ("Agent missing on $PATH … **pre-generation** … exit **1**")
and §13.5 ("on direct use, fail fast with 'provider 'X' not found: is <command> installed?' …
exit non-zero").
**Expected Behavior**: When the resolved provider's `command` is not on `$PATH` / does not exist,
Stagehand should fail fast **before** the snapshot with exit code **1** and a message naming the
missing command.
**Actual Behavior**: `CommitStaged` takes the `write-tree` snapshot (step 3) **before** attempting
generation (step 5). The first `Execute` call fails at `cmd.Start` (command not found). That error
is neither `DeadlineExceeded` nor `Canceled`, so the loop treats it as "fall through to
`ParseOutput`", retries identically `max_duplicate_retries+1` times, then returns a `*RescueError` →
**exit 3** and the full §18.3 rescue block (tree SHA + `git commit-tree … | xargs git update-ref`
manual-recovery command). The user sees a misleading "commit generation failed" with a recovery
recipe, never the intended "provider 'X': command 'Y' not found. Is the agent installed?".
Additionally, a dangling tree object is left in the object store for each such run.
**Steps to Reproduce**:
```bash
# [provider.missing] command = "/nonexistent/path/agent"
echo "b" > b.txt && git add b.txt
stagehand --provider missing
# ❌ Commit generation failed.   ...   Tree ID: <sha>   ...   (exit 3)
```
**Suggested Fix**: Add a pre-snapshot existence check (`exec.LookPath` on the resolved manifest's
`command`) in `buildDeps`/`CommitStaged` that returns a plain exit-1 error ("provider %q: command
%q not found. Is the agent installed?") *before* `WriteTree`. (The `Registry.IsInstalled` LookPath
already exists for `providers list`; reuse it as a pre-flight.) Failing that, at minimum treat a
`cmd.Start` failure distinctly from a non-zero exit so it short-circuits to exit 1 rather than the
parse-retry→rescue path.

## Minor Issues (Nice to Fix)

### Issue 4: The `[generation]` `output` / `strip_code_fence` config fields (file **and** git-config) are loaded but never applied

**Severity**: Minor
**PRD Reference**: §16.2 (`[generation] output`, `[generation] strip_code_fence`), §16.1 (layer-1
defaults "output raw, strip_code_fence true"), §16.3 git-config keys, §12.9 (parse uses the
**manifest's** `output`/`strip_code_fence`). The `config init` template (internal/cmd/config.go)
also documents these as usable options.
**Expected Behavior**: A user setting `output = "json"` / `strip_code_fence = false` (or
`git config stagehand.output json` / `stagehand.stripCodeFence false`) reasonably expects the
parsing pipeline to honor it.
**Actual Behavior**: `config.Config.Output` / `.StripCodeFence` are populated by every loader
(file, git-config) and asserted on in `config/*_test.go`, but **no code path consumes them** —
`provider.ParseOutput` reads only `deps.Manifest.Output` / `.StripCodeFence`. Verified: with
`[generation] output = "json"` set and the manifest at `output = "raw"`, a non-JSON raw blob is
accepted verbatim (no JSON parse attempted). Same for `git config stagehand.output json`.
**Suggested Fix**: Either (a) apply `cfg.Output`/`cfg.StripCodeFence` onto the resolved manifest
before parsing (override the manifest's values), or (b) remove these fields from the documented
config schema / `config init` template and the git-config loader, documenting that
output/fence-stripping is a **per-manifest** setting only (per §12.1/§12.9). Pick one to remove the
silent no-op.

### Issue 5: The §19 repo-local provider-redirect notice is printed twice

**Severity**: Minor
**PRD Reference**: §19 (one-line notice when a repo-local config sets the provider).
**Expected Behavior**: The notice appears once.
**Actual Behavior**: Because config is loaded twice (CLI `PersistentPreRunE`, then again inside
`GenerateCommit.resolveConfig`), `loadRepoLocalConfig` runs twice and writes the notice to stderr
both times whenever `.stagehand.toml` sets `provider`. Verified: `grep -c "repo-local config"` on
the run output returns `2`.
**Suggested Fix**: Resolved for free by Issue 1's fix (pass the already-loaded config through
instead of re-loading). Alternatively, make the notice idempotent / emit it from only one layer.

### Issue 6: `--dry-run` skips the `write-tree` snapshot

**Severity**: Minor
**PRD Reference**: §9.12 FR49 ("full diff→**snapshot**→generate→…" pipeline); P1.M4.T4.S1 contract
("The snapshot is still taken (write-tree runs) but commit-tree/update-ref are skipped").
**Expected Behavior**: Dry-run runs `write-tree` (creating the harmless dangling tree) to faithfully
mirror the real pipeline.
**Actual Behavior**: `runPipeline` gates `WriteTree` behind `if !dryRun`, so no snapshot object is
created in dry-run. Functionally harmless, but it is a deviation from the documented pipeline and
from the work-item contract (and pairs with Issue 2's broader dry-run divergence).
**Suggested Fix**: Take the snapshot in dry-run too (then simply skip `commit-tree`/`update-ref`),
or update the spec/contract to state dry-run omits the snapshot.

### Issue 7: Cosmetic — "Nothing staged — staging all changes (0 files)." on an already-clean tree

**Severity**: Minor
**PRD Reference**: §9.4 FR16–FR18.
**Expected Behavior**: On a clean tree, the auto-stage notice should read sensibly.
**Actual Behavior**: With nothing staged and `auto_stage_all` on, a clean tree prints
`Nothing staged — staging all changes (0 files).` immediately before `Nothing to commit.` The
"(0 files)" phrasing is slightly odd since nothing was staged. Matches FR18's literal `N` template
but is a rough UX in the empty case.
**Suggested Fix**: Skip/shorten the FR18 notice when the post-`AddAll` file count is 0 (go straight
to the exit-2 "Nothing to commit." message).

## Testing Summary

- **Total distinct scenarios exercised**: ~20 (happy path, root commit/unborn repo, dry-run,
  nothing-staged `--no-auto-stage`, clean-tree auto-stage, auto-stage-all with staged + unstaged,
  duplicate→rescue, duplicate→retry→success, parse-fail→rescue, timeout→rescue, code-fence
  stripping, JSON output mode, missing-provider-command, `--config` vs `STAGEHAND_CONFIG`,
  repo-local `.stagehand.toml`, `[generation]` output/strip_code_fence, git-config output,
  stage-while-generating overlap invariant, `--model`/`--timeout`/`--verbose`, `--version`,
  `config path`, `providers list/show`, unknown subcommand).
- **Passing**: all happy-path, data-integrity, rescue, timeout, root-commit, parsing, and
  stage-while-generating scenarios.
- **Failing / deviating**: Issues 1–3 (Major) and 4–7 (Minor) above.
- **Areas with good coverage**: snapshot/atomic-commit core (§13), git plumbing exit-code semantics,
  rescue/timeout/CAS error mapping, prompt assembly, provider render + parse, config precedence
  resolution, signal/process-group handling.
- **Areas needing more attention**: the **CLI ↔ `pkg/stagehand` config-handoff seam** (root cause of
  Issues 1 and 5), the **dry-run code path** being a divergent single-pass implementation (Issues 2
  and 6), the **agent-missing pre-flight check** (Issue 3), and the **dead `[generation]`
  output/strip_code_fence config** (Issue 4).
