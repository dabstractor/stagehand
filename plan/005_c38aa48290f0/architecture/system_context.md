# System context — stagecoach codebase state for the v2.1 delta

Surveyed **2026-07-02** on branch `competitor-feature-parity` (clean; only `plan/005_c38aa48290f0/` untracked).
Read this together with `external_deps.md` (verified external surfaces) and `../delta_prd.md` (authoritative scope).

## 0. Headline

The **entire v2.0/v3 core is implemented and green**: single-commit pipeline, multi-commit decompose
(planner→stager→message→arbiter with `T_start` freeze and 1-deep overlap), config v3 + per-role resolution +
bootstrap + v2→v3 migration, 8 built-in provider manifests, e2e harness, CI, and a full lowercase `docs/` tree.
**None of the six v2.1 features (§9.18–§9.23) exist in code.** Every v2.1 feature is a composition over existing
seams — reference, do not re-implement.

## 1. Build & dependencies

- Module `github.com/dustin/stagecoach`, Go 1.22. Deps: `spf13/cobra v1.10.2`, `spf13/pflag`, `pelletier/go-toml/v2 v2.4.2`.
- **No YAML dependency** — the lazygit target (FR-I5) must add `gopkg.in/yaml.v3` (archived upstream; see external_deps.md §2).
- Makefile: `build`, `test` (race), `coverage-gate` (≥85% on `internal/{git,provider,generate,config}`), `lint`. goreleaser present.
- CI: `.github/workflows/ci.yml` — {ubuntu, macos, macos-13, windows} × go {1.22, 1.23}, `-race`, golangci-lint, govulncheck, coverage.

## 2. Command surface today (`internal/cmd/`)

- Entrypoint `cmd/stagecoach/main.go` → `internal/cmd.Execute(ctx)`; signals via `internal/signal.Install`; exit codes via `internal/exitcode.For` (`Success=0, Error=1, NothingToCommit=2, Rescue=3, Timeout=124`).
- Subcommands: **root** (default action, `root.go` + `default_action.go`: `runDefault`, `shouldDecompose`, `runDecompose`, `printCommitReport`), **config** (`init`, `path`, `upgrade` — `config.go`), **providers** (`list`, `show` — `providers.go`). No `hook`/`integrate`/`models`.
- Root persistent flags: `--provider --model --reasoning --config --timeout --verbose/-v --no-color --all/-a --no-auto-stage --dry-run --commits --single --no-decompose --max-commits` + per-role `--{planner,stager,message,arbiter}-{provider,model,reasoning}`.
- Confirmed ABSENT (grep-verified): `--exclude/-x`, `--format`, `--locale`, `--context`, message `--template`, `--edit`, `--push`, `.stagecoachignore`, `hook`, `integrate`, `models`, `config init --interactive`.
- ⚠ `config init --template` EXISTS (writes the inert reference config, `exampleConfigTemplate` const in `cmd/config.go`). The v2.1 message-shaping `--template` is a different flag on the root command — disambiguate help text.

## 3. Package inventory & the v2.1 seams

### `internal/config` (config.go, load.go, file.go, git.go, bootstrap.go, migrate.go, roles.go, role_defaults.go)
- `Config` fields today: Provider, Model, Reasoning, Timeout, AutoStageAll, Verbose, NoColor, Commits, Single, MaxDiffBytes, MaxMdLines, MaxDuplicateRetries, SubjectTargetChars, Output, StripCodeFence, MaxCommits, BinaryExtensions, Providers map, Roles map, ConfigVersion. **`CurrentConfigVersion = 3`.**
- `[generation]` keys present: max_diff_bytes, max_md_lines, max_duplicate_retries, subject_target_chars, output, strip_code_fence, max_commits, binary_extensions. **Needed for v2.1: `exclude`, `format`, `locale`, `template`, `push`.**
- `Load()` (load.go) applies layers lowest→highest: Defaults → global TOML → repo `.stagecoach.toml` → git `stagecoach.*` → `STAGECOACH_*` env → flags (`fs.Changed` only). `overlay(dst, src)` in file.go is the non-zero merge.
- ⚠ **List merge is REPLACE today** (`BinaryExtensions`). FR-X1 requires `exclude` to **UNION** across sources — a new merge behavior, do not copy the replace pattern.
- Role resolution: `ResolveRoleModel(role, cfg) (provider, model, reasoning)` (roles.go). **FR-D4 curated tier table already exists as `DefaultModelsForProvider(name)`** (role_defaults.go) — reuse it as the `models` fallback (FR-L1b). Bootstrap: `GenerateBootstrapConfig(provider)` (bootstrap.go), first-run auto-write inside `Load`.
- ⚠ Stale string: `exampleConfigTemplate` header says "supports config_version = 2" while the const is 3 — fix in the final docs sweep.

### `internal/provider` (manifest.go, render.go, executor.go, parse.go, merge.go, registry.go, builtin.go)
- `Manifest` has: Name, Detect, Command, Subcommand, PromptDelivery/PromptFlag, PrintFlag, ModelFlag, DefaultModel, SystemPromptFlag, ProviderFlag, BareFlags, TooledFlags, Experimental, Output, JsonField, StripCodeFence, RetryInstruction, Env, ReasoningLevels. **`list_models_command` does NOT exist** — the one new §12.1 field (FR-L2).
- `Render(model, sys, user, reasoning string, mode ...RenderMode) (*CmdSpec, error)` — FR-R5b prefix-splitting chokepoint. `Execute(ctx, spec, timeout, vb)` — context timeout + process-group kill. `ParseOutput(raw, m) (msg, ok, fellback)`.
- Registry: `NewRegistry / Get / List / IsInstalled / MarshalTOML / DefaultProvider(installed) / FirstTooledProvider / DecodeUserOverrides`; `preferredBuiltins = [pi, opencode, cursor, agy, gemini, qwen-code, codex, claude]`. `MergeManifest` in merge.go.
- 8 manifests in `providers/*.toml`, parity-asserted against `builtin.go` by `referencefiles_test.go` — **any Manifest field addition must touch all three + the merge + MarshalTOML**.

### `internal/git` (git.go ~1268 lines; single `Git` interface, `*gitRunner`; exec seams `run`/`runWithInput`, `-C repo`, never `sh -c`)
- Methods: RevParseHEAD, WriteTree, CommitTree, UpdateRefCAS (ErrCASFailed), DiffTree, StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll, Add, StagedFileCount, RevParseTree, ReadTree, TreeDiff, StatusPorcelain, WorkingTreeDiff, LogRange, FreezeWorkingTree, DiffTreeNames.
- **Exclusions seam:** `StagedDiffOptions{MaxDiffBytes, MaxMDLines, Excludes []string, BinaryExtensions []string}` is accepted by all three diff paths (`StagedDiff`, `WorkingTreeDiff`, `TreeDiff`) and `:(exclude)`-style pathspecs are already used internally (`defaultExcludes` `:!*.lock` etc.), **but `Excludes` is passed `nil` by every caller** — §9.18 plugs in here; no new diff machinery.
- **Placeholder template:** binary filtering in `binary.go` (`detectBinaryFiles`, `fileStatuses`, `isBinaryByExtension`, `binaryPlaceholderLine`) emits `<status>\t[binary] <path>` and appends per-file excludes — mirror it for `[excluded]` (FR-X4).
- ⚠ **`rev-parse --git-path` is not wrapped** — hook install (FR-H1) needs a new `HooksPath()` helper (returns cwd-relative paths; resolve to absolute — see external_deps.md §3). Push (FR-P1) needs a **streaming** exec variant (existing `run` captures output).

### `internal/prompt` (system.go, payload.go, planner.go, stager.go, arbiter.go)
- `BuildSystemPrompt(examples, hasMultiline, subjectTarget)` (§17.1) / `BuildFallbackPrompt(subjectTarget)` (§17.2); `BuildUserPayload(diff, rejected []string)` (§17.3 + rejection block); planner: `BuildPlannerSystemPrompt`/`BuildPlannerUserPayload`/`ParsePlannerOutput`.
- ⚠ **No format/locale/context seams exist.** The style-examples block is unconditional; §17.8 requires it to become swappable (conventional/gitmoji/plain replace it entirely), locale is a one-line append, and `--context` needs a slot in `BuildUserPayload` + `BuildPlannerUserPayload` (before the rejection block when both present).

### `internal/generate` + `pkg/stagecoach` + `internal/decompose`
- `CommitStaged(ctx, deps, cfg)`: RevParseHEAD → StagedDiff → WriteTree (arms `signal.SetSnapshot`) → prompt build → generate→parse→dedupe loop (`ResolveRoleModel("message")`) → CommitTree → UpdateRefCAS → DiffTree → Result. Dedupe: `ExtractSubject`/`IsDuplicate`. Rescue: `FormatRescue`/`FormatRescueMulti`.
- ⚠ **No post-generation transform seam**: the accepted message flows straight into `CommitTree` in all THREE commit paths — `internal/generate/generate.go` (~msg accept →~CommitTree), `pkg/stagecoach/stagecoach.go` `runPipeline`, `internal/decompose/message.go` `generateMessage`/`publishCommit` (plus the FR-M11 planner-shortcut in `planner.go`/`runSingleShortcut` and the arbiter N+1). §9.19 `--template` (post-cleanup, pre-duplicate-check) and §9.22 `--edit` (post-dedupe, pre-CommitTree) must introduce ONE shared message-finalization pipeline used by all three.
- `pkg/stagecoach` public API: `Options{Provider, Model, SystemExtra, DryRun, Timeout, Verbose, VerboseOn, Config}`, `Result`, `DecomposeOptions`, `DecomposeResult`, `GenerateCommit`, `Decompose`. Additive-only extension for new knobs.

### `internal/ui`, `internal/exitcode`, `internal/signal`
- `UI` writes progress to **stderr** (stdout stays pipeable); `ProgressLabel(verb, model, provider)` renders "«verb» with «model» in «provider»…". Verbose helpers in `verbose.go`. Follow these conventions for all new output (FR-X5 notices, hook warnings, integrate previews).

## 4. Testing patterns to follow

- **Stub agent**: `cmd/stubagent/main.go`, driven by `STAGECOACH_STUB_*` env (`_OUT`, `_SCRIPT`+`_COUNTER`, `_EXIT`, `_SLEEP_MS`, `_STDERR`, `_MARKER`, `_ARGSFILE`), built via `internal/stubtest.Build`, injected through test-only `Manifest.Env`.
- **Temp-repo git tests**: every `internal/git` method has a real-git temp-repo test — copy that shape for `HooksPath`, streaming push, exclusion pathspecs.
- **E2E harness**: `internal/e2e/` (`//go:build e2e`) runs the compiled binary against a temp repo with a stub-provider TOML (`writeStubConfig`, config_version 3); dual-mode real-agent via `STAGECOACH_RUN_REAL=1`. §20.5 says every new user-visible behavior gets a scenario (hook never-block, exclusion placeholders, push skip-conditions belong here).
- **Push tests caveat**: run git with `GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null` — the developer's global `push.autoSetupRemote=true` otherwise masks the no-upstream failure (external_deps.md §8).
- Coverage gate ≥85% on the four core packages; new packages (`internal/hook`, `internal/integrate`) should aim for the same bar.

## 5. Feature → seam map (the whole delta at a glance)

| v2.1 feature | Existing seam | Net-new |
|---|---|---|
| §9.18 exclusions | `StagedDiffOptions.Excludes` (nil today) + `binary.go` placeholder pattern + `defaultExcludes` pathspecs | `.stagecoachignore` loader, glob→`:(exclude,glob)` translator, UNION config key, `--exclude/-x`, `[excluded]` placeholders |
| §9.19 shaping | `BuildSystemPrompt`/`BuildFallbackPrompt`/`BuildUserPayload`; config cascade; dedupe loop | swappable examples block + 3 scaffolds (§17.8), gitmoji constant, locale line, context block, `$msg` template + message-finalization seam |
| §9.20 hook | diff capture, prompt build, `ResolveRoleModel("message")`, Render/Execute/Parse, dedupe — reused as-is; NO snapshot/commit-tree/update-ref | `internal/hook/` package, `HooksPath()` git helper, POSIX script + marker, cobra `hook` tree, never-block runtime |
| §9.21 integrate | git exec wrapper (alias target); `internal/ui` confirm/output | `internal/integrate/` package (protocol.go engine + targets), yaml.v3 dep, cobra `integrate` tree |
| §9.22 edit/push | snapshot data (tree SHA, DiffTree); run outcome in `default_action.go`; exitcode | message-finalization editor gate, `.git/STAGECOACH_EDITMSG`, `git var GIT_EDITOR` shell-out, streaming `git push` |
| §9.23 discovery | `Registry`/`MarshalTOML`/`referencefiles_test.go`; `DefaultModelsForProvider` (FR-D4 table exists!); `GenerateBootstrapConfig` | `list_models_command` manifest field, `models` command, TTY wizard on `config init --interactive` |

## 6. Contradictions & watch-outs (vs the PRD's assumed baseline)

1. v2.0 decompose **is** implemented (PRD hedges are stale) — plan only the §9.18–§9.23 delta.
2. `config_version` is **3**; only remnant is the stale "= 2" prose in `exampleConfigTemplate` (fix in docs sweep).
3. All four roles already expose provider/model/reasoning flags — nothing missing there.
4. `--template` collides in name with `config init --template` (different commands; disambiguate help text).
5. **`COMPETITOR-ANALYSIS.md` does not exist** — never depend on reading it; the PRD is self-contained.
6. No YAML library in go.mod; gopkg.in/yaml.v3 is archived — pin it, wrap it behind the no-mangle protocol.
7. Docs are lowercase (`docs/cli.md`, `docs/configuration.md`, `docs/how-it-works.md`, `docs/providers.md`, `docs/README.md` index) — Mode-A doc updates target these exact files.
8. goreleaser builds only `cmd/stagecoach` — `cmd/stubagent` is test-only; keep it that way.
