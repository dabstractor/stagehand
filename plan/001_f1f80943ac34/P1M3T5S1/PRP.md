---
name: "P1.M3.T5.S1 — GenerateCommit public API (Options + Result + thin wrapper) — PRD §14.1 / Appendix E item 6"
description: |

  Implement Stagecoach's PUBLIC library surface in `pkg/stagecoach/stagecoach.go` (PRD §14.1): the
  `Options` and `Result` structs and the `GenerateCommit(ctx, opts) (Result, error)` entry point that an
  integrator (git GUI, pre-commit hook, CI step) imports as
  `import "github.com/dustin/stagecoach/pkg/stagecoach"` (US12). The surface is intentionally tiny
  (PRD §14.1): the point is to let an integrator call the core without reimplementing it, NOT to be a
  rich library.

  ONE deliverable, a NEW file, NO edits to existing code:
  **CREATE `pkg/stagecoach/stagecoach.go`** (`package stagecoach`) — the public types + the entry point +
  the typed-error re-exports + Go-doc comments marking the API `// Stable as of v1.0` (Appendix E item 6,
  additive-only Options). Plus **CREATE `pkg/stagecoach/stagecoach_test.go`** (`package stagecoach`) —
  integration tests driving `GenerateCommit` at the public boundary with the stub provider
  (`internal/stubtest`) against real temp git repos (mirrors `internal/generate/generate_test.go`).

  CONTRACT (PRD §14.1, verbatim):
    - `type Options struct { Provider string; Model string; SystemExtra string; DryRun bool; Timeout
      time.Duration }`.
    - `type Result struct { CommitSHA string; Subject string; Message string; Provider string; Model
      string }` (NO `Changes` — the public surface drops the internal `[]git.FileChange`).
    - `func GenerateCommit(ctx context.Context, opts Options) (Result, error)`.

  GENERATECOMMIT LOGIC:
    1. Resolve config: `config.Load(ctx, LoadOpts{RepoDir: os.Getwd(), Flags: nil})`, then apply the
       opts that ARE config fields (Provider/Model/Timeout overwrite cfg when non-zero — caller intent
       wins over file/env/git-config).
    2. Resolve the manifest from the registry (cfg.Provider, else auto-detect via
       `Registry.DefaultProvider`), `Validate()` it, construct `generate.Deps{Git: git.New(repoDir),
       Manifest: manifest}`.
    3. **Common path** (`!opts.DryRun && opts.SystemExtra == ""`): delegate to
       `generate.CommitStaged(ctx, deps, cfg)`, map `generate.Result` → `stagecoach.Result` (drop
       `Changes`). This honors "calls CommitStaged" for the primary path and reuses the tested atomic
       commit with zero duplication.
    4. **Advanced path** (`opts.DryRun || opts.SystemExtra != ""`): the frozen `generate.CommitStaged`
       ALWAYS commits and builds its OWN system prompt with NO `SystemExtra`/`DryRun` seam (P1.M3.T4.S2
       contract, READ-ONLY). So GenerateCommit drives a self-contained `runPipeline` (unexported) that
       reuses the SAME exported primitives CommitStaged uses (`git.Git`, `prompt.*`,
       `provider.{Render,Execute,ParseOutput}`, `generate.{ExtractSubject,IsDuplicate,RescueError,
       CASError}`) — differing only in that (a) `SystemExtra` is appended to the system prompt and
       (b) `DryRun` runs ONE generation pass and returns `CommitSHA=""` WITHOUT committing.

  SCOPE BOUNDARY (load-bearing): this subtask is the PUBLIC WRAPPER + its tests only. It does NOT
  implement the CLI (P1.M4.T1), signal handling (P1.M4.T2 — GenerateCommit is signal-agnostic; it
  observes a cancelled `ctx` only), the `--dry-run` FLAG plumbing (P1.M4.T4 — the CLI just sets
  `opts.DryRun` and calls GenerateCommit), property tests (P1.M5.T1), or ANY change to
  `internal/generate`, `internal/config`, `internal/provider`, `internal/git`, or `internal/prompt`. It
  MODIFIES NOTHING under `internal/`; it only ADDS `pkg/stagecoach/{stagecoach.go,stagecoach_test.go}`. It
  adds NO dependency (`go mod tidy` is a no-op — stdlib + same-module internals only).

  INPUT (upstream — READ-ONLY contracts, already built): `generate.CommitStaged`/`Deps`/`Result`/
  errors (internal/generate/generate.go, P1.M3.T4.S2); `config.Load`/`LoadOpts`/`Config`/`Defaults`
  (internal/config, P1.M1.T4); `provider.NewRegistry`/`DecodeUserOverrides`/`Registry.{Get,List,
  IsInstalled,DefaultProvider}`/`Manifest.{Validate,Resolve}` (P1.M2.T1/T3); `git.New` + the `Git`
  interface (P1.M1.T2/T3); `prompt.{BuildSystemPrompt,BuildFallbackPrompt,DetectMultiline,
  BuildUserPayload}` (P1.M3.T1); `internal/stubtest` (P1.M3.T4.S1 — FROZEN `Build`/`Options`/`Manifest`/
  `NewScript`/`Env`).

  OUTPUT (downstream consumers): the CLI default action (P1.M4.T1.S2) is "parse flags → maybe auto-stage
  → `stagecoach.GenerateCommit(ctx, opts)` → print result"; the `--dry-run` flag (P1.M4.T4) just sets
  `opts.DryRun`. Library consumers (US12) import `pkg/stagecoach` directly. The `Options`/`Result`/
  `GenerateCommit`/error surface is the FROZEN v1.0 public API (Appendix E item 6: additive-only Options).

  ⚠️ **Do NOT modify `generate.CommitStaged` / `Deps` / `config.Config` to add a DryRun or SystemExtra
  seam.** P1.M3.T4.S2 is a frozen, READ-ONLY contract running in parallel; touching `internal/generate`
  or `internal/config` would conflict. The DryRun + SystemExtra logic lives ENTIRELY in
  `pkg/stagecoach/stagecoach.go` (see design-decisions §0). (design §0)
  ⚠️ **`stagecoach.Result` has NO `Changes` field** (PRD §14.1 — "intentionally tiny"). The delegation
  path maps `generate.Result` → `stagecoach.Result` by DROPPING `Changes`. Do not expose the internal
  `[]git.FileChange` on the public surface. (design §1)
  ⚠️ **Tests can't import the git/generate fixture helpers** — they're package-private in `_test.go`.
  Copy the ~25-line `initRepo`/`writeFile`/`stageFile`/`headSHA`/`commitRaw`/`gitOut`/`runGit` set into
  `stagecoach_test.go` (same approach P1.M3.T4.S2 took). (design §8)

  Deliverable: CREATE `pkg/stagecoach/stagecoach.go` + `pkg/stagecoach/stagecoach_test.go`. Imports only
  stdlib + same-module `internal/*`. `go mod tidy` MUST be a no-op. Touches ONLY these two NEW files.

---

## Goal

**Feature Goal**: Ship Stagecoach's public library surface (PRD §14.1) — a tiny, stable, v1.0 Go API that
an integrator imports as `github.com/dustin/stagecoach/pkg/stagecoach` and calls `GenerateCommit(ctx, opts)`
to generate (and, unless `DryRun`, create) a commit from the currently-staged index, reusing the full
internal pipeline (config resolution → manifest resolution → `generate.CommitStaged`, or a self-contained
path when `DryRun`/`SystemExtra` are set). Mark it `// Stable as of v1.0` with additive-only `Options`
(Appendix E item 6).

**Deliverable** (two NEW files, nothing else touched):
1. **`pkg/stagecoach/stagecoach.go`** — `package stagecoach`. The public `Options` and `Result` structs
   (PRD §14.1 shapes), `GenerateCommit(ctx, opts) (Result, error)`, the typed-error re-exports
   (`ErrNothingToCommit`/`ErrTimeout`/`ErrRescue`/`ErrCASFailed` + `RescueError`/`CASError` aliases), an
   unexported `runPipeline` for the DryRun/SystemExtra path, an unexported `resolveConfig` +
   `resolveManifest`/`buildDeps`, and a package doc comment. Go-doc comments on every exported symbol.
2. **`pkg/stagecoach/stagecoach_test.go`** — `package stagecoach`. Integration tests driving `GenerateCommit`
   end-to-end with the stub provider (`stubtest`) against real temp git repos. Own git fixture helpers.

**Success Definition**: `go build ./...` succeeds; `go test -race ./pkg/stagecoach/` is green; `go test
-race ./...` shows NO regression; `go vet ./pkg/stagecoach/` clean; `gofmt -l pkg/stagecoach/` empty;
`golangci-lint run` (if available) clean; go.mod/go.sum byte-unchanged; every other file byte-unchanged.
A library consumer can `import "github.com/dustin/stagecoach/pkg/stagecoach"` and call `GenerateCommit`.

## User Persona

**Target User**: The library integrator (PRD §7 "the plan-holder" extending their toolchain; US12) — a
git GUI, a pre-commit hook, or a CI step that wants Stagecoach's commit-generation WITHOUT reimplementing
it or shelling out to the CLI. Transitively: the CLI itself (P1.M4.T1.S2) is a thin shell over this API
("parse flags → maybe auto-stage → `GenerateCommit` → print result").

**Use Case**: An integrator writes `res, err := stagecoach.GenerateCommit(ctx, stagecoach.Options{
Provider: "claude", Timeout: 60*time.Second })` and, on success, reads `res.Subject` / `res.CommitSHA`.
For a pre-commit preview they add `DryRun: true` and read `res.Message` without committing.

**User Journey**: (internal) integrator imports `pkg/stagecoach` → constructs `Options` → calls
`GenerateCommit` → the API resolves config + manifest + repo, drives the pipeline, returns `Result` (or a
typed error the integrator maps to their own UX). No CLI, no flags, no subprocess orchestration required.

**Pain Points Addressed**: (1) Reimplementing the snapshot→CAS atomic commit is hard — solved by reusing
the tested `CommitStaged`. (2) Integrators who only want a generated message (hooks, previews) — solved by
`DryRun`. (3) Custom instructions per integration — solved by `SystemExtra`. (4) Version-stability fear —
solved by the `// Stable as of v1.0` doc + additive-only `Options` promise.

## Why

- **It IS the library surface (PRD §14.1).** The CLI is "essentially: parse flags → maybe auto-stage →
  `stagecoach.GenerateCommit(ctx, opts)` → print result." Keeping the CLI a thin shell over the library
  GUARANTEES v2 can reuse `GenerateCommit` in a loop (multi-commit decomposition, PRD §10.3).
- **Unblocks the CLI + closes the pipeline.** P1.M4.T1.S2 (default action), P1.M4.T4 (dry-run flag), and
  US12 (library consumers) all wait on this entry point. It is the last slice before the UX layer.
- **Honors Appendix E item 6.** The recommendation was: "ship it, mark it `// Stable as of v1.0`, keep
  `Options` additive-only." This subtask does exactly that.
- **No new dependency, additive only.** Two NEW files under `pkg/stagecoach/`; `internal/*` untouched.

## What

A public `GenerateCommit` that resolves config + manifest + repo and drives the commit-generation
pipeline, plus a `runPipeline` self-contained path for `DryRun`/`SystemExtra` (which the frozen
`CommitStaged` cannot honor). The public API is pure orchestration over the internal packages: it never
shells out except via the injected `git.Git`/`provider.Execute`, never prints, never calls `os.Exit`, never
installs a signal handler (those are the CLI's job). It returns a typed `Result` or a typed error.

### Success Criteria

- [ ] `pkg/stagecoach/stagecoach.go` exists, `package stagecoach`, imports `context`/`errors`/`fmt`/`os`/
      `strings` + `github.com/dustin/stagecoach/internal/{config,generate,git,prompt,provider}` ONLY (NO
      third-party). Exports `Options`, `Result`, `GenerateCommit`, `ErrNothingToCommit`, `ErrTimeout`,
      `ErrRescue`, `ErrCASFailed`, `RescueError`, `CASError`. Has a `// Package stagecoach …` doc.
- [ ] `Options`/`Result` match PRD §14.1 VERBATIM (field names, types, order). `Result` has NO `Changes`.
- [ ] `GenerateCommit` resolves config (`config.Load` + Provider/Model/Timeout overrides), resolves the
      manifest (registry + auto-detect + `Validate`), constructs `Deps`, and: in the common path delegates
      to `generate.CommitStaged` and maps the result; in the DryRun/SystemExtra path calls `runPipeline`.
- [ ] DryRun returns `Result{CommitSHA:""}` and creates NO commit (test: HEAD unchanged after a DryRun
      call). SystemExtra is appended to the system prompt in `runPipeline`.
- [ ] Every exported symbol has a Go-doc comment; `GenerateCommit`/`Options`/`Result` carry the
      `// Stable as of v1.0` note (Appendix E item 6) and the "caller must stage first" contract.
- [ ] `pkg/stagecoach/stagecoach_test.go` exists, `package stagecoach`, drives `GenerateCommit` via
      `stubtest` against temp git repos, and passes: commit-success; DryRun; nothing-staged; provider
      override; timeout.
- [ ] `go build ./...` succeeds; `go test -race ./...` green; `go vet ./pkg/stagecoach/` clean;
      `gofmt -l pkg/stagecoach/` empty; go.mod/go.sum byte-unchanged; every other file byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact upstream
signatures (all quoted below), the central design decision (design-decisions §0 — why DryRun/SystemExtra
can't be a plain `CommitStaged` call and how `runPipeline` resolves it), the PRD §14.1 contract (in
`selected_prd_content`), the test convention to mirror (`internal/generate/generate_test.go` + the fixture
note), and the copy-ready Go skeletons in the Implementation Blueprint. No CLI/signal/property-test
knowledge required.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M3T5S1/research/design-decisions.md
  why: the SINGLE most important read — the 10 decisions specific to this subtask. §0 is the central
       tension (frozen CommitStaged can't honor DryRun/SystemExtra → delegate for common path, self-
       contain via runPipeline for advanced). §1 Result drops Changes; §2 error re-export; §3 config +
       opts overrides; §4 manifest resolution; §5 DryRun single-pass; §6 SystemExtra in runPipeline;
       §7 runPipeline reuses generate's errors; §8 test fixtures unimportable; §9 go mod tidy no-op.
  critical: §0 (read BEFORE writing any logic — it explains why GenerateCommit is NOT a 3-line wrapper),
       §1 (Result shape), §8 (fixtures must be copied, not imported — the #1 compile-error trap).

- file: internal/generate/generate.go   (P1.M3.T4.S2 — READ for the CommitStaged contract; do NOT edit)
  section: `func CommitStaged(ctx, deps, cfg) (Result, error)` + `type Deps` + `type Result` + the error
       sentinels (`ErrNothingToCommit`/`ErrTimeout`/`ErrRescue`/`ErrCASFailed`) + `RescueError`/`CASError`
       (with `Error()`/`Unwrap()`).
  why: THIS is what the common path delegates to AND what runPipeline mirrors. `Deps{Git git.Git;
       Manifest provider.Manifest}` is constructed by GenerateCommit. `Result{CommitSHA, Subject, Message,
       Provider, Model, Changes}` is mapped to the public Result (Changes dropped). The errors are
       re-exported verbatim. runPipeline's commit branch mirrors steps 1-10 EXACTLY (read the body).
  pattern: CommitStaged is signal/CLI-agnostic (returns Result/errors, never prints/exits). GenerateCommit
           inherits that. The 10-step pipeline (RevParseHEAD→StagedDiff→WriteTree→prompts→loop→
           CommitTree→UpdateRefCAS→DiffTree→Result) is the reference runPipeline mirrors.
  gotcha: CommitStaged ALWAYS commits and builds its OWN system prompt (unexported buildSystemPrompt).
          There is NO DryRun flag and NO SystemExtra parameter on it, Deps, or Config. Do NOT try to add
          one (frozen contract). The advanced path uses runPipeline instead. `Result.Changes` exists ONLY
          on the internal Result; the public Result omits it.

- file: internal/config/load.go   (P1.M1.T4 — READ for Load + LoadOpts; do NOT edit)
  section: `type LoadOpts struct{ ConfigPathOverride string; RepoDir string; Flags *pflag.FlagSet }` +
       `func Load(ctx, opts) (*Config, error)`.
  why: GenerateCommit calls `config.Load(ctx, config.LoadOpts{RepoDir: os.Getwd(), Flags: nil})`.
       RepoDir is the CWD (Options has no RepoDir by PRD §14.1 design); `loadGitConfig(repoDir)` runs
       `git -C repoDir config`, which resolves the repo root by walking up, so CWD is correct from any
       subdir. Flags is nil (no CLI in the library path). Load returns a fully-resolved *Config (Layer 1
       defaults through Layer 7 flags — but Layer 7 is skipped since Flags==nil).
  pattern: apply opts.Provider/Model/Timeout onto the returned cfg AFTER Load (they are the caller's
           explicit intent — highest precedence). cfg is a *Config; copy to a local value before mutating
           (do not mutate the caller-visible pointer needlessly — though Load returns a fresh *Config each
           call, so in-place mutation is safe here).
  gotcha: a zero opts.Timeout (time.Duration(0)) means "not set" → do NOT overwrite cfg.Timeout with 0.
          Guard each override with `!= ""` (strings) / `!= 0` (Duration). opts.Provider/Model "" = inherit.

- file: internal/config/config.go   (P1.M1.T4 — READ for Config fields; do NOT edit)
  section: `type Config struct { Provider, Model string; Timeout time.Duration; AutoStageAll, Verbose,
       NoColor bool; MaxDiffBytes, MaxMdLines, MaxDuplicateRetries, SubjectTargetChars int; Output string;
       StripCodeFence bool; Providers map[string]map[string]any }` + `func Defaults() Config`.
  why: cfg carries the resolved tuning GenerateCommit passes to CommitStaged / uses in runPipeline:
       cfg.Timeout (per-attempt Execute timeout), cfg.MaxDuplicateRetries (runPipeline loop bound),
       cfg.SubjectTargetChars (prompt target), cfg.MaxDiffBytes/cfg.MaxMdLines (StagedDiffOptions),
       cfg.Model/cfg.Provider (Render args, "" → manifest default), cfg.Providers (the raw map the
       registry decodes). There is NO SystemExtra/DryRun field (confirmed) — those are Options-only.
  gotcha: cfg.Providers is `toml:"-"` (raw map). Hand it to provider.DecodeUserOverrides (nil-safe).

- file: internal/provider/registry.go   (P1.M2.T3 — READ for manifest resolution; do NOT edit)
  section: `func NewRegistry(userOverrides map[string]Manifest) *Registry` + `func DecodeUserOverrides(
       raw map[string]map[string]any) (map[string]Manifest, error)` + `(r *Registry) Get(name)` +
       `List()` + `IsInstalled(m)` + `DefaultProvider(installed []string) string`.
  why: GenerateCommit resolves the manifest: `overrides, _ := provider.DecodeUserOverrides(cfg.Providers)`;
       `reg := provider.NewRegistry(overrides)`; if cfg.Provider != "" use it, else `name =
       reg.DefaultProvider(installed)` where `installed` = the names in `reg.List()` for which
       `reg.IsInstalled(m)` is true; `m, ok := reg.Get(name)`; if !ok → error; `m.Validate()`.
  pattern: DefaultProvider returns "" if NO preferred built-in is installed (pi/claude/gemini/opencode/
           codex/cursor). In that case GenerateCommit returns a clear error ("no provider configured/
           installed"). Only built-in names are auto-selected (§12.8 user providers never auto-pick).
  gotcha: IsInstalled probes `m.DetectCommand()` via exec.LookPath (cursor's Detect is "agent", not the
          name). Do NOT call Resolve() here — CommitStaged/Render resolve internally; Validate() is enough
          to fail fast on a malformed manifest.

- file: internal/provider/manifest.go   (P1.M2.T1 — READ for Manifest.{Validate,Resolve}; do NOT edit)
  section: `func (m Manifest) Validate() error` + `func (m Manifest) Resolve() Manifest`.
  why: GenerateCommit calls `m.Validate()` after Get (fail fast: non-empty Name, non-nil Command, valid
       enums). runPipeline calls `m.Resolve()` once (nil-pointer-safe) to read `*resolved.RetryInstruction`
       and `*resolved.DefaultModel` — same as CommitStaged does.
  gotcha: pointer fields (RetryInstruction, DefaultModel, …) are *string — ALWAYS Resolve() before deref.
          Render also Validate()+Resolve()s internally, so a manifest passed to CommitStaged need only be
          Get()-valid (Validate is belt-and-suspenders for a good error before any git/agent work).

- file: internal/git/git.go   (P1.M1.T2/T3 — READ for git.New + the Git interface; do NOT edit)
  section: `func New(workDir string) Git` + `type Git interface { … }` + `type StagedDiffOptions` +
       `var ErrCASFailed`.
  why: GenerateCommit constructs `git.New(repoDir)` for Deps.Git. runPipeline (advanced path) calls the
       SAME git methods CommitStaged does, in the same order: RevParseHEAD → (sha,isUnborn,err);
       StagedDiff(ctx, StagedDiffOptions{MaxDiffBytes,MaxMdLines}) → (diff,err); WriteTree → (tree,err)
       [commit path only]; RecentSubjects(ctx,50) [if !isUnborn]; CommitTree(ctx,tree,parents,msg);
       UpdateRefCAS(ctx,"HEAD",newSHA,expectedOld); DiffTree is NOT needed (public Result has no Changes).
  pattern: every method takes ctx first; a non-nil err is a hard failure (propagate, do not retry git).
  gotcha: isUnborn comes ONLY from RevParseHEAD (exit 128). On isUnborn: parents=nil for CommitTree,
          expectedOld=strings.Repeat("0",40) for CAS. UpdateRefCAS returns WRAPPED git.ErrCASFailed
          (errors.Is works); re-read HEAD via RevParseHEAD for CASError.Actual (mirror CommitStaged §8).

- file: internal/provider/render.go + executor.go + parse.go   (P1.M2.T4/T5/T6 — READ; do NOT edit)
  section: `(m Manifest) Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error)` +
       `func Execute(ctx, spec CmdSpec, timeout time.Duration) (stdout, stderr string, err error)` +
       `func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)`.
  why: runPipeline renders + executes + parses exactly as CommitStaged does (read generate.go step 5).
       `spec, err := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)`;
       `out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout)`;
       `msg, ok, _ := provider.ParseOutput(out, deps.Manifest)`. Error contract: timeout → err IS
       context.DeadlineExceeded; cancel → context.Canceled; non-zero exit → *exec.ExitError (stdout still
       captured).
  gotcha: cfg.Model/cfg.Provider may be "" (Render/manifest default them). Result.Model = cfg.Model if
          non-empty else *resolved.DefaultModel. Result.Provider = deps.Manifest.Name.

- file: internal/prompt/system.go + payload.go   (P1.M3.T1 — READ; do NOT edit)
  section: `BuildSystemPrompt(examples []string, hasMultiline bool, subjectTarget int) string` +
       `BuildFallbackPrompt(subjectTarget int) string` + `DetectMultiline(examples []string) bool` +
       `BuildUserPayload(diff string, rejected []string) string`.
  why: runPipeline builds the system prompt (the common delegation path does NOT — CommitStaged builds
       its own). If isUnborn OR CommitCount<=1 → `BuildFallbackPrompt(cfg.SubjectTargetChars)`; else
       `BuildSystemPrompt(msgs, DetectMultiline(msgs), cfg.SubjectTargetChars)` where msgs=
       git.RecentMessages(ctx,20). Append `"\n\n"+SystemExtra` when SystemExtra != "" (design §6).
       `payload := prompt.BuildUserPayload(diff, rejected)` each attempt; on parse-fail retry prepend
       `*resolved.RetryInstruction+"\n\n"`.
  gotcha: generate.buildSystemPrompt is UNEXPORTED — pkg/stagecoach writes its OWN equivalent (a few lines
          calling the prompt builders). This is NOT duplication of IP — it reuses the same builder funcs.

- file: internal/generate/dedupe.go + rescue.go   (P1.M3.T2/T3 — READ for the EXPORTED helpers; do NOT edit)
  section: `func ExtractSubject(message string) string` + `func IsDuplicate(subject string, recent
       []string) bool` (generate package — call as `generate.ExtractSubject` / `generate.IsDuplicate`).
  why: runPipeline's commit branch REUSES these (exported) — NOT re-implemented. ExtractSubject(msg) for
       the subject + dup check; IsDuplicate(subject, recent) for FR32. The DryRun single-pass also calls
       ExtractSubject for Result.Subject (no dup check — moot when not committing).
  gotcha: FormatRescue (rescue.go) is NOT called by GenerateCommit — the integrator renders the rescue
          message if they want (they have RescueError.TreeSHA/ParentSHA/Candidate). GenerateCommit just
          returns the structured RescueError.

- file: internal/stubtest/stubtest.go   (P1.M3.T4.S1 — READ for the FROZEN stub API; do NOT edit)
  section: `func Build(t testing.TB) string` + `type Options struct{…}` + `func Manifest(bin string, o
       Options) provider.Manifest` + `func NewScript(t, bin, responses []string) provider.Manifest`.
  why: the integration tests' ONLY mock. BUT — the tests call the PUBLIC GenerateCommit, which resolves
       the manifest via the REGISTRY, not via stubtest.Manifest directly. So the stub must be REGISTERED:
       the test sets `cfg.Providers` (or uses an override) so the registry yields the stub manifest, OR
       the test calls an internal seam. SIMPLEST: test sets `opts.Provider` + injects the stub via a
       config.Providers override whose `command` points at the compiled stub binary — BUT a cleaner path
       is to expose the Deps construction so the test can pass a stub Manifest. SEE design §8 / Blueprint:
       GenerateCommit's manifest resolution uses the registry; the test registers a "stub" provider entry
       (cfg.Providers["stub"] = {command: <binPath>, prompt_delivery:"stdin", output:"raw"}) and sets
       opts.Provider="stub". stubtest.Build(t) gives binPath; stubtest.Options{Out:"feat: x"} drives it.
  gotcha: the stub binary is invoked through the REAL provider.Execute (full pipeline). A blank script
          line ⇒ empty stdout ⇒ ParseOutput ok=false. stubtest.Options{SleepMS:400}+short opts.Timeout ⇒
          timeout. Verify the provider-override TOML shape decodes to a valid Manifest (DecodeUserOverrides).

- file: internal/generate/generate_test.go   (P1.M3.T4.S2 — READ for the TEST PATTERN + fixtures; do NOT edit)
  section: the fixture helpers `initRepo`/`writeFile`/`stageFile`/`headSHA`/`commitRaw`/`gitOut`/`runGit`
       + the scenario structure (success, dedupe, parse-fail, CAS, root).
  why: stagecoach_test.go MIRRORS this file's approach (real git + stub provider + temp repos) at the
       public boundary. The fixtures are package-private + in _test.go → UNIMPORTABLE — copy them.
  gotcha: copy the helpers verbatim (initRepo sets repo-local user.name/user.email so commit-tree works).
          Use t.TempDir() for the repo. chdir is NOT needed — pass the repo dir to git via the fixtures'
          `git -C <dir>` pattern (GenerateCommit uses os.Getwd(), so the test must `t.Chdir(dir)` or set
          the process CWD — SEE design §8 note on CWD).

- url: (PRD §14.1 + §14 layout + Appendix E item 6 — already in context as selected_prd_content `h3.51`/
       `h2.14`/`h2.28`; ALSO plan/001_f1f80943ac34/prd_snapshot.md and architecture/system_context.md)
  why: §14.1 is the AUTHORITATIVE public-surface spec (Options/Result/GenerateCommit verbatim). §14 shows
       pkg/stagecoach/stagecoach.go as the PUBLIC API file. Appendix E item 6 mandates "Stable as of v1.0"
       + additive-only Options.
  critical: §14.1's "The CLI's main.go is essentially: parse flags → maybe auto-stage →
            stagecoach.GenerateCommit(ctx, opts) → print result" — GenerateCommit must be reusable by both
            the CLI and direct library callers. Result.CommitSHA is "empty if DryRun or not committed."
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; go-toml/v2 + pflag  (UNCHANGED)
go.sum                          # unchanged
cmd/stagecoach/main.go           # stub (P1.M1.T1) — UNCHANGED
cmd/stubagent/main.go           # P1.M3.T4.S1 (stub binary) — UNCHANGED
internal/
  config/{config,load,git,file}.go   # P1.M1.T4 — Config/Load/LoadOpts (read-only ref)
  generate/generate.go          # P1.M3.T4.S2 — CommitStaged/Deps/Result/errors (read-only ref + runPipeline mirror)
  generate/{dedupe,rescue}.go   # P1.M3.T2/T3 — ExtractSubject/IsDuplicate/FormatRescue (read-only ref)
  generate/*_test.go            # P1.M3.T2/T3/T4 — TEST PATTERN to mirror (NOT import)
  git/git.go                    # P1.M1.T2/T3 — Git interface + New + ErrCASFailed (read-only ref)
  git/*_test.go                 # P1.M1.T2/T3 — fixture pattern (NOT import)
  prompt/{system,payload}.go    # P1.M3.T1 — Build* prompt builders (read-only ref)
  provider/{manifest,registry,render,executor,parse}.go  # P1.M2 — Manifest/Registry/Render/Execute/ParseOutput (ref)
  stubtest/stubtest.go          # P1.M3.T4.S1 — Build/Options/Manifest/NewScript (FROZEN ref)
pkg/
  stagecoach/                    # EMPTY dir — this subtask creates stagecoach.go + stagecoach_test.go here
Makefile                        # build/test(-race)/coverage/lint/clean/help — UNCHANGED
```

### Desired Codebase tree with files to be added

```bash
pkg/stagecoach/stagecoach.go         # NEW — package stagecoach. Options, Result, GenerateCommit(ctx,opts),
                                   #        typed-error re-exports (ErrNothingToCommit/ErrTimeout/ErrRescue/
                                   #        ErrCASFailed + RescueError/CASError aliases), unexported
                                   #        resolveConfig + buildDeps + runPipeline. Go-doc comments, v1.0.
pkg/stagecoach/stagecoach_test.go    # NEW — integration tests via stubtest + temp git repos. Own fixture
                                   #        helpers (initRepo/writeFile/stageFile/headSHA/commitRaw/gitOut/runGit).
                                   #        Scenarios: commit-success, DryRun, nothing-staged, provider-override, timeout.
# All other files UNCHANGED. go.mod/go.sum UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (frozen CommitStaged, design §0): generate.CommitStaged ALWAYS commits and builds its OWN
// system prompt. It has NO DryRun flag and NO SystemExtra parameter (on it, Deps, or Config). Do NOT add
// one — P1.M3.T4.S2 is a read-only parallel contract; modifying internal/generate/internal/config would
// conflict. DryRun + SystemExtra are honored by runPipeline (unexported) in pkg/stagecoach, which reuses
// the SAME exported primitives (git.Git, prompt.*, provider.{Render,Execute,ParseOutput},
// generate.{ExtractSubject,IsDuplicate,RescueError,CASError}). The common path (!DryRun && SystemExtra=="")
// delegates to CommitStaged.

// CRITICAL (Result shape, design §1): stagecoach.Result is PRD §14.1's shape — {CommitSHA, Subject,
// Message, Provider, Model}. NO Changes. The delegation path maps generate.Result → stagecoach.Result by
// DROPPING the internal []git.FileChange. runPipeline constructs stagecoach.Result directly.

// CRITICAL (opts override precedence, design §3): apply opts.Provider/Model/Timeout AFTER config.Load,
// guarding non-zero (opts.Timeout==0 means "not set" → keep cfg.Timeout; opts.Provider/Model "" → inherit).
// These three are the caller's explicit intent — highest precedence over file/env/git-config.

// CRITICAL (DryRun is single-pass, design §5): DryRun previews ONE generation (no dedupe loop — dedupe is
// a commit concern, moot when not committing). DryRun does NOT call WriteTree (no snapshot — nothing to
// commit). Returns Result{CommitSHA:""}. Errors: ErrNothingToCommit (nothing staged), ErrTimeout (bare —
// no RescueError since there's no TreeSHA), or a descriptive fmt.Errorf (model produced no valid message).

// CRITICAL (SystemExtra forces runPipeline, design §6): SystemExtra != "" takes the runPipeline path even
// when !DryRun — there it runs the FULL generate→dedupe→commit flow (mirroring CommitStaged) with the
// extended prompt (sysPrompt + "\n\n" + SystemExtra). This is the only place with logic that mirrors
// CommitStaged's loop; keep it faithful to generate.go (read it as the reference).

// GOTCHA (manifest resolution via registry, design §4): GenerateCommit resolves the manifest from
// cfg.Provider (or auto-detect via DefaultProvider over IsInstalled built-ins), NOT from a direct
// stubtest.Manifest call. Tests register a "stub" provider via cfg.Providers override + opts.Provider.
// Validate() the manifest before constructing Deps (fail fast). Do NOT Resolve() here (CommitStaged/Render
// do it internally).

// GOTCHA (CWD is the repo, design §3): Options has no RepoDir (PRD §14.1). GenerateCommit uses os.Getwd()
// for BOTH config.Load(RepoDir) and git.New(repoDir). loadGitConfig runs `git -C <cwd> config` which walks
// up to the repo root, so CWD is correct from any subdir. Tests must run with CWD inside the temp repo
// (t.Chdir, Go 1.24+) OR the test sets up the repo in a dir and chdir's into it — SEE design §8.

// GOTCHA (fixtures unimportable, design §8): the git/generate _test.go fixture helpers are package-private.
// Copy the ~25-line set into stagecoach_test.go (initRepo sets repo-local user.name/email for commit-tree).

// GOTCHA (no third-party import, design §9): pkg/stagecoach imports stdlib (context/errors/fmt/os/strings)
// + same-module internal/* ONLY. go mod tidy is a no-op. Do NOT import pflag/go-toml directly (config.Load
// and provider.DecodeUserOverrides handle those internally).

// GOTCHA (don't print/exit/signal): GenerateCommit is pure orchestration — returns Result/errors; never
// writes stdout/stderr, never calls os.Exit, never installs a signal handler. A cancelled ctx is the only
// signal seam (runPipeline maps context.Canceled → *generate.RescueError{Kind:ErrRescue} on the commit path).
```

## Implementation Blueprint

### Data models and structure

```go
// pkg/stagecoach/stagecoach.go
package stagecoach

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/generate"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/prompt"
	"github.com/dustin/stagecoach/internal/provider"
)

// Options configures a GenerateCommit call. All fields are optional (zero value ⇒ inherit the resolved
// default). This struct is ADDITIVE-ONLY for future versions (Appendix E item 6): new fields may be
// added, existing fields will not be removed or repurposed.
//
// Stable as of v1.0.
type Options struct {
	Provider    string        // manifest name; "" → resolved default (auto-detect installed built-ins)
	Model       string        // "" → manifest default_model
	SystemExtra string        // appended to the built system prompt (extra integrator instructions)
	DryRun      bool          // if true, return the message WITHOUT committing (CommitSHA == "")
	Timeout     time.Duration // per-attempt generation timeout; 0 → config default (120s)
}

// Result is the outcome of GenerateCommit. On DryRun (or any non-committing outcome) CommitSHA is "".
//
// Stable as of v1.0.
type Result struct {
	CommitSHA string // the published commit SHA; "" if DryRun or not committed
	Subject   string // the commit subject (first line)
	Message   string // the full commit message (subject [+ body])
	Provider  string // the resolved provider name
	Model     string // the resolved model
}

// ---- Typed-error re-exports (so library consumers import only pkg/stagecoach) ----
// These ARE the generate-package symbols (alias / same sentinel), so errors.Is / errors.As work
// uniformly whether the error came from the delegation path (CommitStaged) or runPipeline.

var (
	ErrNothingToCommit = generate.ErrNothingToCommit // nothing staged (caller should stage first)
	ErrTimeout         = generate.ErrTimeout         // generation exceeded the timeout
	ErrRescue          = generate.ErrRescue          // generation failed after retries
	ErrCASFailed       = generate.ErrCASFailed       // HEAD moved since the snapshot (non-fast-forward)
)

// RescueError carries the post-snapshot context for a rescue (see generate.RescueError). Returned
// wrapped for BOTH ErrTimeout and ErrRescue. type alias — interchangeable with generate.RescueError.
type RescueError = generate.RescueError

// CASError carries the "HEAD moved" context (see generate.CASError). type alias.
type CASError = generate.CASError
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE pkg/stagecoach/stagecoach.go — public types + error re-exports + package doc
  - FILE: NEW pkg/stagecoach/stagecoach.go. PACKAGE: `package stagecoach`.
  - DOC: `// Package stagecoach is Stagecoach's public library surface (PRD §14.1). The entry point is
      GenerateCommit, which generates (and, unless Options.DryRun, creates) a commit from the
      currently-staged index. The surface is intentionally tiny: an integrator imports this package
      instead of reimplementing the pipeline or shelling out to the CLI. // Stable as of v1.0.`
  - IMPORT: context, errors, fmt, os, strings, time + internal/{config,generate,git,prompt,provider}.
  - DEFINE Options + Result (PRD §14.1 verbatim — see "Data models") with the `// Stable as of v1.0`
      doc comment on each + the "caller must stage first" note on GenerateCommit. DEFINE the error
      re-exports (4 vars + 2 type aliases — see "Data models").
  - NAMING: exported types CamelCase (Options, Result, RescueError, CASError); error vars Err*;
      unexported helpers lowerCamelCase (resolveConfig, buildDeps, runPipeline). PLACEMENT: all in
      pkg/stagecoach/stagecoach.go.

Task 2: IMPLEMENT resolveConfig(ctx, opts) (config.Config, string, error)
  - SIGNATURE: `func resolveConfig(ctx context.Context, opts Options) (config.Config, string, error)`.
      Returns (cfg, repoDir, err). repoDir = os.Getwd() (Options has no RepoDir).
  - BODY: repoDir, err := os.Getwd(); if err != nil { return config.Config{}, "", fmt.Errorf("stagecoach:
      getwd: %w", err) }. cfgPtr, err := config.Load(ctx, config.LoadOpts{RepoDir: repoDir, Flags: nil});
      if err != nil { return config.Config{}, "", fmt.Errorf("stagecoach: load config: %w", err) }.
      cfg := *cfgPtr (copy to a value). Apply opts overrides (HIGHEST precedence — caller intent):
      if opts.Provider != "" { cfg.Provider = opts.Provider }; if opts.Model != "" { cfg.Model =
      opts.Model }; if opts.Timeout != 0 { cfg.Timeout = opts.Timeout }. return cfg, repoDir, nil.
  - GOTCHA: guard each override with non-zero (opts.Timeout==0 ⇒ keep cfg.Timeout). SystemExtra/DryRun
      are NOT applied here (they're Options-only, flow into runPipeline).

Task 3: IMPLEMENT buildDeps(cfg, repoDir) (generate.Deps, error)
  - SIGNATURE: `func buildDeps(cfg config.Config, repoDir string) (generate.Deps, error)`.
  - BODY: overrides, err := provider.DecodeUserOverrides(cfg.Providers); if err != nil { return
      generate.Deps{}, fmt.Errorf("stagecoach: provider overrides: %w", err) }. reg :=
      provider.NewRegistry(overrides). name := cfg.Provider. if name == "" { installed := []string{};
      for _, m := range reg.List() { if reg.IsInstalled(m) { installed = append(installed, m.Name) } };
      name = reg.DefaultProvider(installed) }. if name == "" { return generate.Deps{},
      fmt.Errorf("stagecoach: no provider configured and none of the built-ins (%s) are installed",
      strings.Join([]string{"pi","claude","gemini","opencode","codex","cursor"}, ", ")) }. m, ok :=
      reg.Get(name); if !ok { return generate.Deps{}, fmt.Errorf("stagecoach: unknown provider %q",
      name) }. if err := m.Validate(); err != nil { return generate.Deps{}, fmt.Errorf("stagecoach:
      provider %q: %w", name, err) }. return generate.Deps{Git: git.New(repoDir), Manifest: m}, nil.
  - GOTCHA: do NOT Resolve() here (CommitStaged/Render resolve). DefaultProvider returns "" if no
      preferred built-in is installed → clear error. (design §4)

Task 4: IMPLEMENT GenerateCommit(ctx, opts) (Result, error) — the dispatcher
  - SIGNATURE: `func GenerateCommit(ctx context.Context, opts Options) (Result, error)`.
  - DOC: `// GenerateCommit generates and (unless Options.DryRun) creates a commit from the
      currently-staged index. It does NOT decide what to stage: the caller stages first (or the CLI
      layer uses its auto-stage-all). Repo = the current working directory. Returns a typed error
      (ErrNothingToCommit / ErrTimeout / ErrRescue / ErrCASFailed, or a *RescueError / *CASError) the
      caller maps to their own UX. // Stable as of v1.0.`
  - BODY:
      cfg, repoDir, err := resolveConfig(ctx, opts); if err != nil { return Result{}, err }
      deps, err := buildDeps(cfg, repoDir);              if err != nil { return Result{}, err }
      // Common path: no DryRun, no SystemExtra → delegate to the frozen, tested orchestrator.
      if !opts.DryRun && opts.SystemExtra == "" {
          res, gerr := generate.CommitStaged(ctx, deps, cfg)
          if gerr != nil { return Result{}, gerr }
          return Result{CommitSHA: res.CommitSHA, Subject: res.Subject, Message: res.Message,
              Provider: res.Provider, Model: res.Model}, nil   // drop res.Changes (design §1)
      }
      // Advanced path: DryRun and/or SystemExtra → self-contained (CommitStaged can't honor these).
      return runPipeline(ctx, deps, cfg, opts.SystemExtra, opts.DryRun)
  - GOTCHA: the delegation maps generate.Result → stagecoach.Result DROPPING Changes. The advanced path
      returns a stagecoach.Result directly. Both paths return the SAME typed errors (runPipeline reuses
      generate's errors — design §7).

Task 5: IMPLEMENT runPipeline(ctx, deps, cfg, systemExtra, dryRun) (Result, error) — the advanced path
  - SIGNATURE: `func runPipeline(ctx context.Context, deps generate.Deps, cfg config.Config,
      systemExtra string, dryRun bool) (Result, error)`.
  - BODY (mirror generate.CommitStaged; read generate.go as the reference):
      // Step 1: parent + isUnborn.
      parentSHA, isUnborn, err := deps.Git.RevParseHEAD(ctx); if err != nil { return Result{}, err }
      // Step 2: diff; nothing → ErrNothingToCommit.
      diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{MaxDiffBytes: cfg.MaxDiffBytes,
          MaxMdLines: cfg.MaxMdLines}); if err != nil { return Result{}, err }
      if diff == "" { return Result{}, ErrNothingToCommit }
      // Step 3 (commit path only): snapshot. DryRun skips it (no commit → no object-store write).
      var treeSHA string
      if !dryRun { treeSHA, err = deps.Git.WriteTree(ctx); if err != nil { return Result{}, err } }
      // Step 4: system prompt (+ SystemExtra) + recent subjects (built ONCE).
      sysPrompt, err := buildSysPrompt(ctx, deps.Git, cfg, isUnborn); if err != nil { return Result{}, err }
      if systemExtra != "" { sysPrompt += "\n\n" + systemExtra }
      var recent []string
      if !isUnborn { recent, err = deps.Git.RecentSubjects(ctx, 50); if err != nil { return Result{}, err } }
      resolved := deps.Manifest.Resolve()
      model := cfg.Model; if model == "" { model = *resolved.DefaultModel }
      // ---- DryRun: single pass, no commit. ----
      if dryRun {
          payload := prompt.BuildUserPayload(diff, nil)
          spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
          if rerr != nil { return Result{}, fmt.Errorf("stagecoach: render: %w", rerr) }
          out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout)
          if execErr != nil {
              if errors.Is(execErr, context.DeadlineExceeded) { return Result{}, ErrTimeout }
              return Result{}, fmt.Errorf("stagecoach: generate: %w", execErr)
          }
          msg, ok, _ := provider.ParseOutput(out, deps.Manifest)
          if !ok { return Result{}, errors.New("stagecoach: dry run: model produced no valid message") }
          return Result{CommitSHA: "", Subject: generate.ExtractSubject(msg), Message: msg,
              Provider: deps.Manifest.Name, Model: model}, nil
      }
      // ---- Commit path (SystemExtra set): full generate→dedupe loop + commit. Mirror CommitStaged. ----
      retryInstr := *resolved.RetryInstruction
      var rejected []string; var candidate, msg string; var parseFail, success bool
      for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
          payload := prompt.BuildUserPayload(diff, rejected)
          if parseFail { payload = retryInstr + "\n\n" + payload }
          spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
          if rerr != nil { return Result{}, fmt.Errorf("stagecoach: render: %w", rerr) }
          out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout)
          if execErr != nil {
              if errors.Is(execErr, context.DeadlineExceeded) {
                  return Result{}, &generate.RescueError{Kind: generate.ErrTimeout, TreeSHA: treeSHA,
                      ParentSHA: parentSHA, Candidate: candidate, Cause: execErr}
              }
              if errors.Is(execErr, context.Canceled) {
                  return Result{}, &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: treeSHA,
                      ParentSHA: parentSHA, Candidate: candidate, Cause: execErr}
              }
              // non-zero exit: fall through to ParseOutput (stdout may be partial-valid).
          }
          m, ok, _ := provider.ParseOutput(out, deps.Manifest)
          if !ok { parseFail = true; candidate = m; continue }
          parseFail = false
          subject := generate.ExtractSubject(m)
          if generate.IsDuplicate(subject, recent) { rejected = append(rejected, subject); candidate = m; continue }
          msg = m; success = true; break
      }
      if !success {
          return Result{}, &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: treeSHA,
              ParentSHA: parentSHA, Candidate: candidate}
      }
      // Commit (mirror CommitStaged steps 7-8).
      var parents []string
      if !isUnborn { parents = []string{parentSHA} }
      newSHA, err := deps.Git.CommitTree(ctx, treeSHA, parents, msg); if err != nil { return Result{}, err }
      expectedOld := parentSHA
      if isUnborn { expectedOld = strings.Repeat("0", 40) }
      if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
          if errors.Is(err, git.ErrCASFailed) {
              actual, _ := deps.Git.RevParseHEAD(ctx)
              return Result{}, &generate.CASError{TreeSHA: treeSHA, Expected: parentSHA,
                  Actual: actual, Message: msg}
          }
          return Result{}, err
      }
      return Result{CommitSHA: newSHA, Subject: generate.ExtractSubject(msg), Message: msg,
          Provider: deps.Manifest.Name, Model: model}, nil
  - GOTCHA: this mirrors CommitStaged but (a) appends SystemExtra, (b) DryRun branch returns early.
      DiffTree is NOT called (public Result has no Changes). Reuse generate.ExtractSubject/
      IsDuplicate/RescueError/CASError (exported). (design §0/§5/§6/§7)

Task 6: IMPLEMENT buildSysPrompt(ctx, g, cfg, isUnborn) (string, error) — unexported helper
  - SIGNATURE: `func buildSysPrompt(ctx context.Context, g git.Git, cfg config.Config, isUnborn bool)
      (string, error)`.
  - BODY: if isUnborn { return prompt.BuildFallbackPrompt(cfg.SubjectTargetChars), nil }. n, err :=
      g.CommitCount(ctx); if err != nil { return "", err }. if n <= 1 { return
      prompt.BuildFallbackPrompt(cfg.SubjectTargetChars), nil }. msgs, err := g.RecentMessages(ctx, 20);
      if err != nil { return "", err }. return prompt.BuildSystemPrompt(msgs,
      prompt.DetectMultiline(msgs), cfg.SubjectTargetChars), nil.
  - NOTE: this mirrors generate.buildSystemPrompt (unexported — can't import). It reuses the prompt
      builders; NOT IP duplication. (design §6 gotcha)

Task 7: CREATE pkg/stagecoach/stagecoach_test.go — integration tests (stub + temp git repos)
  - FILE: NEW pkg/stagecoach/stagecoach_test.go. PACKAGE: `package stagecoach`.
  - FIXTURES: copy the ~25-line helper set from internal/generate/generate_test.go (initRepo/writeFile/
      stageFile/headSHA/commitRaw/gitOut/runGit) — they're package-private + unimportable. Use
      t.TempDir() for the repo. Set repo-local identity (initRepo does `git config user.name/email`).
  - CWD NOTE: GenerateCommit uses os.Getwd(). The test must run with CWD INSIDE the temp repo. Use
      `t.Chdir(repoDir)` (Go 1.24+) OR `wd, _ := os.Getwd(); os.Chdir(repoDir); t.Cleanup(func(){
      os.Chdir(wd) })` (Go 1.22 — this repo's version). PREFER the Chdir+Cleanup form for go 1.22.
  - REGISTER THE STUB: bin := stubtest.Build(t). The test resolves the manifest via the REGISTRY (not
      stubtest.Manifest directly), so register a "stub" provider via a config override. Two options:
      (a) build a config.Config with cfg.Providers["stub"] pointing command at bin (must decode via TOML
      shape — DecodeUserOverrides re-encodes the map), OR (b) SIMPLER — set cfg via config.Defaults()
      and inject the stub through opts... but opts has no manifest field. CLEANEST for the test: bypass
      the registry by testing runPipeline directly is NOT public. So register the stub via cfg.Providers.
      IMPLEMENTATION: build the stub manifest via stubtest.Manifest(bin, stubtest.Options{Out:"feat: x"}),
      marshal it to a map[string]map[string]any (or construct the raw map by hand: {"command": bin,
      "prompt_delivery":"stdin", "output":"raw"}), set cfg.Providers["stub"] = that map, then call
      GenerateCommit with opts.Provider="stub". (NOTE: because GenerateCommit calls config.Load which
      DISCARDS a hand-built cfg, the test must instead construct cfg via config.Defaults() + set
      cfg.Providers, then call resolveConfig/buildDeps... but those are unexported. RESOLUTION: the test
      calls the PUBLIC GenerateCommit and relies on a repo-local .stagecoach.toml or env to set the
      provider — OR expose a thin test seam. SIMPLEST WORKABLE: write a temp .stagecoach.toml with a
      [provider.stub] table pointing at the stub binary into the temp repo, set STAGECOACH_CONFIG or rely
      on repo-local discovery, and call GenerateCommit with opts.Provider="stub".) — SEE the dedicated
      note below; pick the approach that compiles against the frozen config.Load.
  - SCENARIOS:
      * TestGenerateCommit_Success: repo with ≥1 commit + staged change; stub Out="feat: add x";
        GenerateCommit → Result.CommitSHA non-empty, matches headSHA after; Result.Subject contains the
        stub output; HEAD advanced.
      * TestGenerateCommit_DryRun: same setup, opts.DryRun=true, stub Out="feat: preview"; before-sha =
        headSHA; GenerateCommit → Result.CommitSHA=="", Result.Message non-empty; after-sha = headSHA;
        assert before-sha == after-sha (NO commit created).
      * TestGenerateCommit_NothingStaged: repo with nothing staged; GenerateCommit → errors.Is(err,
        ErrNothingToCommit).
      * TestGenerateCommit_ProviderOverride: opts.Provider selects a registered provider (the stub);
        asserts Result.Provider == "stub".
      * TestGenerateCommit_Timeout: stub SleepMS=500 + opts.Timeout=50ms; GenerateCommit → errors.Is(err,
        ErrTimeout) (DryRun path) OR errors.As(err, &RescueError{})+ErrTimeout (commit path).
  - COVERAGE: all 5 Options fields exercised (Provider, Model, SystemExtra, DryRun, Timeout). DryRun +
      nothing-staged + timeout are the error-contract assertions.
  - PLACEMENT: pkg/stagecoach/stagecoach_test.go.
```

**NOTE on Task 7 stub registration (read carefully):** `GenerateCommit` calls `config.Load`, which
reads from disk (TOML/git-config/env) — a hand-built `config.Config` is NOT accepted by the public
signature. To drive the stub through the registry at the PUBLIC boundary, the test registers the stub as
a repo-local provider. The cleanest go-1.22 approach:
1. `bin := stubtest.Build(t)`; `repo := t.TempDir()`.
2. Write `repo/.stagecoach.toml` containing a `[provider.stub]` table whose `command = "<bin>"`,
   `prompt_delivery = "stdin"`, `output = "raw"` (the shape `loadRepoLocalConfig` decodes — see
   `internal/config/file.go` `loadRepoLocalConfig`, which reads CWD `.stagecoach.toml`).
3. `chdir` into `repo` (Chdir+Cleanup); `initRepo`; seed a commit; stage a change.
4. `res, err := stagecoach.GenerateCommit(ctx, stagecoach.Options{Provider: "stub", DryRun: true})`.
Because `config.Load`'s Layer 3 reads CWD `.stagecoach.toml`, the `[provider.stub]` override is merged into
`cfg.Providers`, `DecodeUserOverrides` yields the stub manifest, and `opts.Provider="stub"` selects it.
Verify against `loadRepoLocalConfig`'s exact decode shape (read `internal/config/file.go`); if the raw
`command` path with backslashes on Windows is an issue, use forward slashes or `filepath.ToSlash`. If the
repo-local TOML approach proves brittle, FALL BACK to setting `STAGECOACH_PROVIDER` + a global
`$XDG_CONFIG_HOME/stagecoach/config.toml` — but prefer repo-local (self-contained per test).

### Implementation Patterns & Key Details

```go
// The dispatcher — GenerateCommit is NOT a 3-line wrapper. It dispatches between the frozen CommitStaged
// (common path) and runPipeline (DryRun/SystemExtra). Read design-decisions §0 before writing it.
func GenerateCommit(ctx context.Context, opts Options) (Result, error) {
	cfg, repoDir, err := resolveConfig(ctx, opts)
	if err != nil { return Result{}, err }
	deps, err := buildDeps(cfg, repoDir)
	if err != nil { return Result{}, err }
	if !opts.DryRun && opts.SystemExtra == "" {
		res, err := generate.CommitStaged(ctx, deps, cfg)   // delegate — tested atomic commit
		if err != nil { return Result{}, err }
		return Result{CommitSHA: res.CommitSHA, Subject: res.Subject, Message: res.Message,
			Provider: res.Provider, Model: res.Model}, nil   // DROP res.Changes
	}
	return runPipeline(ctx, deps, cfg, opts.SystemExtra, opts.DryRun)   // DryRun / SystemExtra
}

// Manifest resolution — GenerateCommit resolves via the REGISTRY (auto-detect or opts.Provider), then
// Validate()s. Tests register a stub via a repo-local [provider.stub] TOML override + opts.Provider.
func buildDeps(cfg config.Config, repoDir string) (generate.Deps, error) {
	overrides, err := provider.DecodeUserOverrides(cfg.Providers) // nil-safe
	if err != nil { return generate.Deps{}, fmt.Errorf("stagecoach: provider overrides: %w", err) }
	reg := provider.NewRegistry(overrides)
	name := cfg.Provider
	if name == "" {
		var installed []string
		for _, m := range reg.List() {
			if reg.IsInstalled(m) { installed = append(installed, m.Name) }
		}
		name = reg.DefaultProvider(installed) // "" if no preferred built-in installed
	}
	if name == "" {
		return generate.Deps{}, fmt.Errorf("stagecoach: no provider configured and none of the built-ins are installed")
	}
	m, ok := reg.Get(name)
	if !ok { return generate.Deps{}, fmt.Errorf("stagecoach: unknown provider %q", name) }
	if err := m.Validate(); err != nil { return generate.Deps{}, fmt.Errorf("stagecoach: provider %q: %w", name, err) }
	return generate.Deps{Git: git.New(repoDir), Manifest: m}, nil
}

// runPipeline's DryRun branch returns CommitSHA="" WITHOUT calling WriteTree/CommitTree/UpdateRefCAS.
// Its commit branch mirrors CommitStaged (read generate.go) and returns the SAME typed errors so the
// caller's error handling is uniform regardless of which path ran. (design §5/§7)
```

### Integration Points

```yaml
CONFIG (read-only — GenerateCommit consumes, never writes):
  - reads: config.Load(ctx, LoadOpts{RepoDir: os.Getwd(), Flags: nil}) → the 7-layer resolved Config.
  - applies: opts.Provider/Model/Timeout as the highest-precedence override AFTER Load.

REGISTRY (read-only):
  - reads: provider.NewRegistry(DecodeUserOverrides(cfg.Providers)); Get/List/IsInstalled/DefaultProvider.

GIT (read + write via CommitStaged / runPipeline):
  - reads: RevParseHEAD, StagedDiff, CommitCount, RecentMessages, RecentSubjects.
  - writes (commit path only): WriteTree, CommitTree, UpdateRefCAS. DryRun writes NOTHING.

DOWNSTREAM (the public API is the FROZEN v1.0 surface):
  - CLI (P1.M4.T1.S2): parse flags → maybe auto-stage → stagecoach.GenerateCommit(ctx, opts) → print.
  - CLI --dry-run (P1.M4.T4): sets opts.DryRun = true; calls GenerateCommit; prints Message.
  - Library consumers (US12): import "github.com/dustin/stagecoach/pkg/stagecoach".
  - Options is ADDITIVE-ONLY (Appendix E item 6): new fields OK; never remove/rename existing.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after creating each file — fix before proceeding.
gofmt -w pkg/stagecoach/stagecoach.go pkg/stagecoach/stagecoach_test.go
go vet ./pkg/stagecoach/
go build ./...

# Project-wide
make build            # go build -ldflags … -o bin/stagecoach ./cmd/stagecoach (must succeed)
make lint             # golangci-lint run (if available) — must be clean

# Expected: Zero errors. gofmt -l pkg/stagecoach/ must be EMPTY (go-doc comments + struct formatting).
```

### Level 2: Unit / Integration Tests (Component Validation)

```bash
# Test the public API as created.
go test -race ./pkg/stagecoach/ -v

# Full suite — must show NO regression in the internal packages.
go test -race ./...

# Coverage (if desired)
go test -coverprofile=coverage.out ./pkg/stagecoach/ && go tool cover -func=coverage.out

# Expected: all stagecoach tests pass; the 5 scenarios (success, DryRun, nothing-staged, provider-
# override, timeout) green; existing internal/* tests still green.
```

### Level 3: Integration Testing (Library Surface Validation)

```bash
# Build the CLI + library together.
go build ./...

# Smoke-test the public import compiles from an external-style call (in-repo):
go vet ./pkg/stagecoach/

# Manual library-shape check (the types/options compile and are addressable):
cat <<'EOF' | go run /dev/stdin
package main
import (
  "context"
  "fmt"
  "time"
  "github.com/dustin/stagecoach/pkg/stagecoach"
)
func main() {
  opts := stagecoach.Options{Provider: "pi", Model: "", SystemExtra: "refs ticket #42", DryRun: false, Timeout: 30*time.Second}
  _ = opts
  // We do NOT call GenerateCommit here (no staged changes / no git repo in this throwaway main);
  // this only verifies the public types + import path compile.
  fmt.Println("stagecoach public surface OK")
  _ = context.Background()
}
EOF
# Expected: "stagecoach public surface OK" — proves the import path + Options/Result types are usable.

# DryRun round-trip in a real temp repo (manual, optional):
#   mkdir -p /tmp/shtest && cd /tmp/shtest && git init -q && git config user.name t && git config user.email t@t
#   echo hi > a.txt && git add a.txt
#   # (with a provider configured) call GenerateCommit({Provider:"pi", DryRun:true}); assert no commit.

# Expected: import compiles; DryRun leaves HEAD unchanged; commit path advances HEAD.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Stability-promise check (Appendix E item 6): grep the Go-doc comments.
grep -n "Stable as of v1.0" pkg/stagecoach/stagecoach.go
# Expected: matches on Package doc, Options, Result, GenerateCommit.

# Additive-only sanity: Options/Result match PRD §14.1 field-for-field.
grep -nA8 "type Options struct" pkg/stagecoach/stagecoach.go
grep -nA7 "type Result struct"  pkg/stagecoach/stagecoach.go
# Expected: Options has exactly {Provider, Model, SystemExtra, DryRun, Timeout};
#           Result has exactly {CommitSHA, Subject, Message, Provider, Model} (NO Changes).

# Anti-rename guard: the public symbols exist and are exported.
go doc ./pkg/stagecoach
# Expected: lists Options, Result, GenerateCommit, ErrNothingToCommit, ErrTimeout, ErrRescue,
#           ErrCASFailed, RescueError, CASError.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go test -race ./pkg/stagecoach/ -v` green (5 scenarios).
- [ ] `go test -race ./...` green (NO regression in internal/*).
- [ ] `go vet ./pkg/stagecoach/` clean; `go build ./...` succeeds.
- [ ] `gofmt -l pkg/stagecoach/` empty; `golangci-lint run` clean (if available).
- [ ] go.mod/go.sum byte-UNCHANGED (`go mod tidy` is a no-op); every other file byte-unchanged.

### Feature Validation

- [ ] `Options`/`Result` match PRD §14.1 verbatim (field names, types, order); `Result` has NO `Changes`.
- [ ] Common path delegates to `generate.CommitStaged` and maps the result (drops `Changes`).
- [ ] DryRun returns `CommitSHA==""` and creates NO commit (test asserts HEAD unchanged).
- [ ] `SystemExtra` is appended to the system prompt in `runPipeline` (forces the advanced path).
- [ ] Provider/Model/Timeout opts overrides take precedence over file/env/git-config.
- [ ] Manifest resolved via registry (cfg.Provider or auto-detect); `Validate()`d before use.
- [ ] Typed errors re-exported; `errors.Is(err, stagecoach.ErrCASFailed)` /
      `errors.As(err, &stagecoach.RescueError{})` work across both paths.

### Code Quality Validation

- [ ] Every exported symbol has a Go-doc comment; `// Stable as of v1.0` on Package/Options/Result/
      GenerateCommit (Appendix E item 6).
- [ ] Follows existing codebase patterns (Deps DI, ctx-first, typed errors, no printing/exiting).
- [ ] File placement matches the desired tree; only `pkg/stagecoach/{stagecoach.go,stagecoach_test.go}` added.
- [ ] Anti-patterns avoided (no os.Exit, no signal handler, no shelling out except via git.Git/Execute).
- [ ] No third-party import; only stdlib + same-module internal/*.

### Documentation & Deployment

- [ ] Go-doc comments self-document the contract (stability promise, "caller stages first", DryRun).
- [ ] The `runPipeline` duplication vs `CommitStaged` is documented (design §0) — a reviewer can see WHY.
- [ ] No new env vars or config keys introduced (Options is the only new surface).

---

## Anti-Patterns to Avoid

- ❌ Don't add a DryRun/SystemExtra seam to `generate.CommitStaged`/`Deps`/`config.Config` — P1.M3.T4.S2
  is a frozen, read-only parallel contract. Honor them via `runPipeline` in `pkg/stagecoach` instead.
- ❌ Don't make `GenerateCommit` a 3-line wrapper that always calls `CommitStaged` — that would COMMIT on
  DryRun (defeating it) and DROP SystemExtra (a v1-stable defect). Dispatch (design §0).
- ❌ Don't expose `Changes` on the public `Result` — PRD §14.1 is "intentionally tiny."
- ❌ Don't Resolve() the manifest in `buildDeps` — `CommitStaged`/`Render` do it internally; `Validate()`
  is enough (fail fast on a malformed manifest).
- ❌ Don't overwrite `cfg.Timeout` with `opts.Timeout==0` — guard non-zero (0 means "not set").
- ❌ Don't call `WriteTree`/`CommitTree`/`UpdateRefCAS` in the DryRun branch — DryRun writes nothing.
- ❌ Don't catch all exceptions — be specific (mirror CommitStaged's `errors.Is(…, DeadlineExceeded)` /
  `context.Canceled` / `git.ErrCASFailed` branching).
- ❌ Don't import the git/generate `_test.go` fixture helpers — copy them (package-private + unimportable).
- ❌ Don't introduce a third-party dependency — stdlib + same-module internals only (`go mod tidy` no-op).

---

## Confidence Score

**8 / 10** for one-pass implementation success.

Rationale: the upstream contracts are fully quoted (CommitStaged, Load, Registry, Git, prompt builders,
stubtest) and the central design decision (delegate-for-common + runPipeline-for-advanced) is documented
in depth (design-decisions §0). The two residual risks that keep this from a 9-10: (1) the **stub
registration at the public boundary** (Task 7) — `GenerateCommit` resolves the manifest via `config.Load`
+ the registry, so the test must register the stub through a repo-local `.stagecoach.toml` `[provider.stub]`
override rather than a direct `stubtest.Manifest` call; the exact TOML shape that `loadRepoLocalConfig`
decodes must be verified against `internal/config/file.go` (read it). (2) `runPipeline`'s commit branch
**mirrors `CommitStaged`** — a faithful re-implementation is required; the implementer should read
`internal/generate/generate.go` end-to-end and match its error branching (RescueError for timeout/loop,
CASError for CAS) exactly. Both are tractable with the references provided.
