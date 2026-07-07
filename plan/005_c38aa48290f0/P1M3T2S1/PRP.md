name: "P1.M3.T2.S1 — `hook exec` runtime: source-gated generation with never-block contract"
description: |
  Implement the `prepare-commit-msg` runtime: `internal/hook/exec.go` (`Run`) runs the message
  generation pipeline — diff capture (with exclusions + binary filtering, §9.1/§9.18) → system prompt
  (mature/fallback, §9.3/§9.19) → message-role generation → duplicate rejection (§9.7) — **WITHOUT**
  any commit plumbing (NO snapshot/WriteTree, NO commit-tree, NO update-ref, NO rescue/signal: git owns
  this commit). It writes the message at the TOP of `<msg-file>`, preserving git's comment block beneath.
  Source-gated: no-op exit 0 when source ∈ {message,template,merge,squash,commit} or the staged diff is
  empty (FR-H4). Never-block: any failure → one stderr line, `<msg-file>` UNTOUCHED, exit 0; `--strict`
  inverts to non-zero (FR-H5). Resolves the message role exactly like the single-commit path (FR-H6).
  Wires the `hook exec <msg-file> [<source> [<sha>]]` cobra leaf (documented as called-by-git, not by
  users) onto S2's `hookCmd`, and ships [Mode A] docs (docs/cli.md `hook exec` + docs/how-it-works.md
  FR-H7 FAQ) + e2e scenarios (real `git commit` in a temp repo via the stub agent).

---

## Goal

**Feature Goal**: Ship the runtime half of stagecoach's git hook mode (PRD §9.20): when git's
`prepare-commit-msg` hook (installed by S2) fires `stagecoach hook exec "$@"`, stagecoach generates a
commit message for the STAGED diff and prepends it to git's message file — so a plain `git commit` in an
IDE/lazygit gets an AI message with **zero ceremony and zero risk of blocking the commit**. The runtime
reuses the single-commit generation pipeline but **strips the snapshot/commit plumbing entirely** (git
owns this commit) and inverts the failure contract (never exit non-zero unless `--strict`).

**Deliverable**:
1. `internal/hook/exec.go` — `Run(ctx, deps, cfg, msgFile, source) error` (the source-gated,
   never-block runtime) + `ErrNoOp` sentinel + the pure helpers `NoOpSource(source)` and
   `WriteMessageFile(path, msg) error`. Mirrors `generate.CommitStaged`'s generation loop using the
   EXPORTED primitives, minus the snapshot/commit steps.
2. `internal/hook/exec_test.go` — in-process tests (stub `provider.Manifest`, temp git repo): source
   gate, empty-diff no-op, happy-path message write (comment block preserved), parse-fail retry,
   duplicate rejection, timeout/stub-exit-1 never-block (msg-file UNTOUCHED).
3. `internal/cmd/hookexec.go` — the `hook exec` cobra leaf (NEW file; attaches to S2's `hookCmd` via
   `init()`; does NOT edit S2's `hook.go`) + `runHookExec` (loads config itself, resolves the message
   manifest mirroring `runDefault`, applies the never-block exit-code mapping) + `internal/cmd/hookexec_test.go`.
4. `docs/cli.md` — a `### hook exec` subcommand block (Mode A). `docs/how-it-works.md` — the FR-H7 FAQ
   entry (the trade-off inversion: plumbing vs hook mode).
5. `internal/e2e/hook_scenarios_test.go` (`//go:build e2e`) — real `git commit` in a temp repo via the
   stub agent: happy path (hook fills the message), failure-injection (stub exit 1 → never-block, commit
   proceeds), `-m` no-op (source=message → hook no-ops).

**Success Definition**:
- A stagecoach-installed hook, on a plain `git commit` with staged changes, writes the generated message
  into `.git/COMMIT_EDITMSG` at the top (comment block intact) and the commit lands with that message.
- `git commit -m "x"` / `-t` / merge / squash / `--amend` → hook exits 0 having done nothing (source
  gated). Empty staged diff → hook exits 0 having done nothing.
- Any generation failure (agent missing/timeout/parse/dup exhaustion, even a config-load error) → the
  message file is byte-identical to before, ONE line on stderr, exit 0 (so the commit proceeds to an
  empty editor). With `--strict` baked into the script, the same failure exits non-zero (aborts).
- `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt` all green;
  `go test -tags e2e ./internal/e2e/...` green.

## User Persona

**Target User**: the "plan-holder" (PRD §7.1) who commits via `git commit` in their IDE/lazygit and wants
the message auto-filled — without learning a new command, and without ever being blocked by a model hiccup.

**Use Case**: after `stagecoach hook install` (S2), a plain `git commit` fires the hook → `stagecoach hook
exec <msgfile>` generates from the staged diff and writes the message → git opens the editor pre-filled
(or proceeds in a non-interactive flow). The user never types `hook exec` themselves.

**User Journey**: stage changes → `git commit` → hook runs generation in-process → message appears in the
editor → save/close → commit lands. On any failure the editor opens empty (as if stagecoach weren't
installed) and the commit still works — the "invisible assistant" contract.

**Pain Points Addressed**: incumbents' hooks can ABORT a commit on a model timeout (and
`--no-verify` does NOT skip `prepare-commit-msg` — architecture §3), trapping the user. Stagecoach's
never-block (FR-H5) guarantees the commit always proceeds; `--strict` is the opt-in for users who want
generation to be mandatory.

## Why

- **PRD §9.20 FR-H4**: the runtime runs the standard pipeline (capture → prompt → generate → dedupe) and
  writes the message at the TOP of `<msg-file>` preserving git's comment block; NO snapshot/commit-tree/
  update-ref; source-gated no-op for {message,template,merge,squash,commit} and empty staged diff.
- **PRD §9.20 FR-H5**: never-block — any failure leaves `<msg-file>` untouched, one stderr line, exit 0;
  `--strict` (baked by install) inverts to non-zero.
- **PRD §9.20 FR-H6**: resolves provider/model/reasoning exactly like the single-commit path (the
  `message` role, §9.15) via the same config/timeout keys; NEVER decomposes.
- **PRD §9.20 FR-H7**: the trade-off inversion is a first-class FAQ (plumbing = atomic + bypass
  pre-commit hooks; hook = pre-commit hooks honored, no snapshot guarantees).
- **architecture/external_deps.md §3 (VERIFIED, git 2.54.0)**: plain `git commit` invokes the hook with
  ONLY arg 1 (source absent); non-zero exit aborts AND `--no-verify` does NOT skip this hook — which is
  why FR-H5's exit-0 contract is load-bearing.
- **Scope fences**: consumes S1 (`Marker`, `hookScript`) and S2 (`hookCmd` with its no-op
  `PersistentPreRunE`); does NOT touch install/uninstall/status, does NOT add the FR-E4 `--edit`
  rejection (P1.M5.T1.S1), does NOT surface README (P1.M7.T1.S1), does NOT decompose.

## What

A generation-only runtime + a cobra leaf + docs + e2e. The runtime is a faithful, **plumbing-stripped**
mirror of `generate.CommitStaged`'s steps 1/2/4/5 (no step 3 snapshot, no 7/8/9 commit/ref) — it reuses
the same exported primitives but constructs no `*RescueError`/`*CASError` and arms no signal handler.

### Success Criteria

- [ ] `internal/hook/exec.go`: `Run(ctx, deps generate.Deps, cfg config.Config, msgFile, source string) error`;
      `ErrNoHookGeneration`/`ErrNoOp` sentinel; `NoOpSource(source string) bool`; `WriteMessageFile(path, msg string) error`.
- [ ] `Run` no-ops (`ErrNoOp`) when `NoOpSource(source)` (the 5 named sources) OR `StagedDiff == ""`; else
      runs capture→prompt→generate→dedupe and on success calls `WriteMessageFile`; on ANY failure returns a
      descriptive error **without writing the file**.
- [ ] `Run` contains NO `WriteTree`/`CommitTree`/`UpdateRefCAS`/`signal.*`/`DiffTree` calls (grep-enforced).
- [ ] `internal/cmd/hookexec.go`: `hook exec` leaf (`cobra.RangeArgs(1,3)`, local `--strict` bool),
      registered via `hookCmd.AddCommand(hookExecCmd)` in `init()`; `runHookExec` loads config itself
      (`config.Load` mirroring root's PersistentPreRunE), resolves the message manifest (mirror
      `runDefault`/`buildDeps`), resolves excludes, builds `generate.Deps`, calls `hook.Run`, and maps the
      result: nil/`ErrNoOp`→exit 0; other error→one stderr line + exit 0 (non-strict) / `exitcode.Error` (strict).
- [ ] `internal/hook/exec_test.go` + `internal/cmd/hookexec_test.go`: source gate, empty-diff no-op,
      happy-path write (comment block preserved byte-for-byte beneath the message), parse retry, dup
      rejection, timeout/exit-1 never-block (msg-file unchanged), `--strict` → non-zero.
- [ ] `docs/cli.md` `### hook exec` + `docs/how-it-works.md` FR-H7 FAQ.
- [ ] `internal/e2e/hook_scenarios_test.go`: the three real-`git commit` scenarios.
- [ ] `go build ./...` + `go test ./...` + `go vet ./...` + `golangci-lint run` + `gofmt -l` clean.

## All Needed Context

### Context Completeness Check

_This PRP names the exact generation steps to mirror (CommitStaged 1/2/4/5, with the loop body written
out), the exact exported primitives + their signatures, the S2 config-load contract (hook exec loads
config itself because `hookCmd`'s no-op PersistentPreRunE skips it), the never-block exit-code mapping,
the message-file write format, the cobra wiring that AVOIDS editing S2's `hook.go`, and the e2e env recipe
(PATH + STAGECOACH_CONFIG + GIT_EDITOR). An implementer with no prior codebase knowledge can complete it
from this document + codebase access._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/P1M3T2S1/research/codebase-patterns.md
  why: Condensed research — the "no plumbing" design call, the mirror-vs-extract decision (with the
       pkg/stagecoach precedent), the reusable primitive list with signatures, the config-load contract
       from S2, the source-gate facts, the write format, the never-block mapping, and the e2e recipe.
  section: all
  critical: |
    hook exec CANNOT call GenerateCommit/CommitStaged (they snapshot+commit). It MIRRORS the generation
    loop from exported primitives (same pattern pkg/stagecoach already uses for buildSysPrompt/runPipeline).
    hookCmd's no-op PersistentPreRunE (S2) means hook exec must config.Load ITSELF in RunE.

- file: internal/generate/generate.go
  why: CommitStaged — THE pipeline to MIRROR. Steps 1 (RevParseHEAD), 2 (StagedDiff; "" ⇒ nothing), 4
       (buildSystemPrompt + recentSubjects — both unexported, ~15 lines each, copy them), 5 (the
       generate→parse→dedupe LOOP, ~35 lines — the authoritative loop body). Steps 3/7/8/9 are FORBIDDEN
       in hook mode (snapshot/commit/ref/difftree).
  pattern: |
    Loop body to mirror (CommitStaged step 5): per attempt — BuildUserPayload(diff, cfg.Context, rejected)
    [+ prepend retryInstr if prev parse failed]; Manifest.Render(msgModel, sysPrompt, payload, msgReasoning);
    provider.Execute(ctx, *spec, cfg.Timeout, vb); on DeadlineExceeded → bail (hook: return timeout err,
    NO retry); ParseOutput(out, manifest); if !ok → parseFail=true, VerboseRetry, continue; else
    FinalizeMessage(m, cfg); ExtractSubject; IsDuplicate(subject, recent) → append rejected, VerboseRetry,
    continue; else accept. Bounded by `for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++`.
  gotcha: |
    CommitStaged wraps failures in *RescueError{treeSHA,...} and calls signal.SetSnapshot/SetCandidate —
    hook exec must NOT (no snapshot exists). On success CommitStaged commits; hook exec WRITES THE FILE
    instead. On timeout CommitStaged returns immediately (no retry) — hook exec does the same (return err).

- file: pkg/stagecoach/stagecoach.go
  why: buildDeps (L325) is THE manifest-resolution sequence to mirror in the cmd layer
       (DecodeUserOverrides→NewRegistry→ResolveRoleModel("message")→DefaultProvider if ""→Get→Validate→
       IsInstalled→apply cfg.Output/StripCodeFence→generate.Deps). buildSysPrompt (L393) + runPipeline
       (L415) are the PRECEDENT for mirroring generate's unexported internals ("This mirrors generate.X
       (unexported — can't import). It reuses the prompt builders; NOT IP duplication.").
  critical: |
    Do NOT import pkg/stagecoach from internal/hook or internal/cmd (internal→pkg is an anti-pattern).
    Copy the ~15-line resolution inline into hookexec.go (it is already duplicated between runDefault
    and buildDeps — a third copy is the accepted status quo).

- file: internal/cmd/default_action.go
  why: runDefault shows (a) the inline manifest resolution + labelProvider resolution to copy into
       runHookExec, (b) the exitcode.New(code, nil) silent-exit idiom for self-printed messages, (c)
       ui.New + u.Progress(ProgressLabel) usage, (d) exclude.ResolveExcludePathspecs + ui.NewVerbose.
  pattern: "DecodeUserOverrides→NewRegistry→(installed loop)→ResolveRoleModel('message')→DefaultProvider
            →Get→Validate→IsInstalled; verbose := ui.NewVerbose(stderr, cfg.Verbose); excludes :=
            exclude.ResolveExcludePathspecs(*cfg, repoDir, verbose)."

- file: internal/cmd/root.go
  why: THE config.Load call to mirror inside runHookExec (since hookCmd's no-op PersistentPreRunE skips it):
       `config.Load(cmd.Context(), config.LoadOpts{ConfigPathOverride: flagConfig, RepoDir: repoDir,
       Flags: cmd.Flags()})`. flagConfig is the root persistent flag var (inherited by hook exec). Also:
       Execute() sets the ctx on rootCmd (read via cmd.Context()).
  critical: |
    hook exec INHERITS hookCmd's no-op PersistentPreRunE (S2) → Config() is nil for it → it MUST load
    config itself. This is by design (fits FR-H5: a config error is a failure → never-block). Do NOT
    remove hookCmd's no-op PersistentPreRunE (S2 owns it; it protects install/uninstall/status).

- file: internal/cmd/hook.go   (S2 — CONTRACT, do NOT edit)
  why: S2 defines the package-level `hookCmd` var (with the no-op PersistentPreRunE) and registers
       install/uninstall/status on it. hook exec attaches as a SIBLING leaf from a NEW file
       (internal/cmd/hookexec.go) via `hookCmd.AddCommand(hookExecCmd)` in init(). Because hookCmd is a
       package var (initialized before any init() func), this is safe regardless of init() ordering.
  critical: "Do NOT edit hook.go (S2 is being implemented in parallel — avoid merge collisions). Put the
             exec leaf + its init() in the new hookexec.go."

- file: internal/hook/script.go   (S1 — CONTRACT, consume)
  why: `Marker` const + `hookScript(strict)` confirm the installed script is `exec stagecoach hook exec
       [--strict] "$@"` — i.e. `--strict` precedes the positional args, and the runtime receives
       `<msg-file> [<source> [<sha>]]` as positionals. This fixes the cobra Args shape (RangeArgs(1,3))
       and the local `--strict` flag.

- file: internal/provider/render.go
  why: `Manifest.Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error)`
       (L89) — the command-emission chokepoint; v3 splits an `inference/model` slash-prefix into
       `--provider <inference>` for provider_flag providers (FR-R5b). hook exec uses the message role's
       resolved model.
- file: internal/provider/executor.go
  why: `Execute(ctx, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout, stderr string, err error)`
       (L44). On timeout returns err wrapping context.DeadlineExceeded (hook: bail, never retry). Non-zero
       agent exit → *exec.ExitError with partial stdout (ParseOutput still runs).
- file: internal/provider/parse.go
  why: `ParseOutput(raw, m Manifest) (msg string, ok bool, fellback bool)` (L20) — the 5-step cleanup.
       ok==false ⇒ retry with retry_instruction (FR29).
- file: internal/generate/dedupe.go
  why: `ExtractSubject(message) string` (first line) + `IsDuplicate(subject, recent) bool` (exact,
       case-sensitive set over the 50 recent subjects). FR30/FR32.
- file: internal/generate/finalize.go
  why: `FinalizeMessage(msg, cfg) string` = ApplyTemplate (§9.19 FR-F8). Call AFTER ParseOutput, BEFORE
       ExtractSubject (so dedupe judges the templated subject). Also `Manifest.Resolve().RetryInstruction`
       for the corrective preamble (FR29).
- file: internal/prompt/system.go
  why: `BuildSystemPrompt(examples, hasMultiline, subjectTarget, format, locale)` (L190) +
       `BuildFallbackPrompt(subjectTarget, format, locale)` (L176) + `DetectMultiline` (search this file).
- file: internal/prompt/payload.go
  why: `BuildUserPayload(diff, context string, rejected []string)` (L97) — appends the rejection list for
       FR32 retries and the user --context (FR-F7).
- file: internal/config/roles.go
  why: `ResolveRoleModel(role, cfg) (provider, model, reasoning)` — hook exec resolves "message"
       (provider is discarded — the manifest is selected in the cmd layer; model/reasoning feed Render).
- file: internal/exclude/exclude.go
  why: `ResolveExcludePathspecs(cfg, repoRoot, v *ui.Verbose) ([]string, error)` (L98) — unions the
       .stagecoachignore + [generation].exclude + built-in denylist; passed into deps.Excludes → StagedDiff.
- file: internal/git/git.go
  why: the Git interface methods consumed: RevParseHEAD (isUnborn), StagedDiff(opts) (capture; "" ⇒ empty),
       CommitCount, RecentMessages(20), RecentSubjects(50). (HasStagedChanges NOT needed — StagedDiff==""
       is the gate.) HooksPath is S1's (not used by the runtime, only by install).
- file: internal/exitcode/exitcode.go
  why: Success=0 / Error=1 / New(code,err) / ExitError.Error()=="" when err==nil (silent-exit idiom for
       self-printed stderr). Never-block non-strict = exit 0 (return nil); strict = exitcode.New(Error, nil)
       AFTER printing the one stderr line (silent so main doesn't double-print).
- file: internal/e2e/harness_test.go + scenarios_test.go
  why: THE e2e harness to extend (`//go:build e2e`). buildStagecoach/buildStub (cached), newRepo,
       seedCommit, writeFile, runGit, headSHA, writeStubConfig, stubEnv. Add hook-specific helpers
       (runGitCommitWithHook) + a TestE2EHookScenarios with the three subtests.
  gotcha: "the hook script runs `stagecoach` from $PATH → prepend the built bin's dir to PATH in git's env;
           set GIT_EDITOR=true so git doesn't open a real editor on the hook-filled message; set
           STAGECOACH_CONFIG + STAGECOACH_STUB_* in git's env (the hook subprocess inherits them)."
- file: internal/stubtest/stubtest.go
  why: in-process stub for internal/hook/exec_test.go — `stubtest.Manifest(opts)`/`NewScript` return a
       provider.Manifest wired to cmd/stubagent; Env knobs (Out/Exit/SleepMS) drive failure injection.
- url: https://git-scm.com/docs/githooks#_prepare_commit_msg
  why: authoritative prepare-commit-msg arg contract (msg-file, source, sha) + "non-zero aborts, not
       skipped by --no-verify".
- docfile: plan/005_c38aa48290f0/architecture/external_deps.md
  why: §3 VERIFIED — plain git commit ⇒ only arg 1 (source absent); named sources; non-zero aborts;
       --no-verify does not skip. Underpins the source gate + the never-block rationale.
  section: "## 3."
- docfile: plan/005_c38aa48290f0/prd_snapshot.md
  why: §9.20 FR-H4/H5/H6/H7 (the runtime contract), §9.1 (capture), §9.7 (dedupe), §9.15 (message role),
       §9.18 (exclusions), §9.19 (format/locale/template/context). §15.3 (hook exec synopsis).
  section: "§9.20, §9.1, §9.7, §9.15, §9.18, §9.19, §15.3"
```

### Current Codebase tree (relevant slice)

```bash
internal/hook/
  script.go        # S1 (done): Marker, ScriptMode, hookScript(strict) — CONSUMED
  hook.go          # S2 (in flight): Status/Detect/Install/Uninstall + hookCmd is NOT here — see cmd/
  exec.go          # NEW — Run (source-gated never-block runtime) + ErrNoOp + NoOpSource + WriteMessageFile
  exec_test.go     # NEW — in-process (stub manifest + temp repo)
internal/cmd/
  root.go          # UNCHANGED — rootCmd, PersistentPreRunE/config.Load pattern, flagConfig, Execute()
  default_action.go# pattern ref — inline manifest resolution, exitcode silent idiom, ui/exclude
  hook.go          # S2 (in flight): hookCmd var (no-op PersistentPreRunE) + install/uninstall/status — DO NOT EDIT
  hookexec.go      # NEW — `hook exec` leaf + runHookExec (config.Load self, resolve manifest, never-block map) + init()
  hookexec_test.go # NEW — cobra-leaf tests (args, source gate, --strict exit code)
internal/e2e/
  harness_test.go        # UNCHANGED — buildStagecoach/buildStub/newRepo/runGit/writeStubConfig/stubEnv
  scenarios_test.go      # UNCHANGED
  hook_scenarios_test.go # NEW (//go:build e2e) — real git commit via the hook (happy/failure/-m no-op)
docs/
  cli.md           # EDIT — add ### hook exec (after ### hook status, before ### providers list)
  how-it-works.md  # EDIT — add the FR-H7 FAQ (plumbing vs hook mode trade-off)
```

### Desired Codebase tree

```bash
# Two NEW source files (internal/hook/exec.go, internal/cmd/hookexec.go) + their tests + one e2e file +
# two docs edits. NO edit to S2's internal/cmd/hook.go or S1's internal/hook/script.go. No new deps.
# hook exec attaches to S2's hookCmd via init() in the new file (no root.go edit, no hook.go edit).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (no plumbing): hook exec MUST NOT call WriteTree/CommitTree/UpdateRefCAS/DiffTree or touch
// internal/signal. git owns this commit (FR-H4). A grep test asserts exec.go has none of these calls.
// This is the single most important invariant — a hook that snapshots+commits would double-commit.

// CRITICAL (config-load): hookCmd's no-op PersistentPreRunE (S2) means Config()==nil for `hook exec`.
// runHookExec MUST call config.Load itself (mirror root.go's PersistentPreRunE). A config error is a
// FAILURE → never-block (exit 0 non-strict). Do NOT weaken or remove hookCmd's no-op PersistentPreRunE.

// CRITICAL (never-block, FR-H5): Run returns ErrNoOp for intended no-ops (source gate / empty diff) and
// a descriptive error for generation failures — WITHOUT having written the file. WriteMessageFile is
// called ONLY on a fully-accepted message. The cmd layer: nil/ErrNoOp → exit 0; other err → ONE stderr
// line ("stagecoach: <err>") + exit 0 (non-strict) / exitcode.New(Error, nil) (strict). Never print more
// than one line; never touch the message file on a failure.

// CRITICAL (source absent): plain `git commit` invokes prepare-commit-msg with ONLY the msg-file arg —
// source is ABSENT (architecture §3). So `NoOpSource("")` must be FALSE (empty/absent source ⇒ proceed,
// the empty case we fill). NoOpSource returns true ONLY for the 5 named sources. argc can be 1, 2, or 3.

// GOTCHA (mirror, not import): generate.buildSystemPrompt/recentSubjects and the loop are unexported.
// Copy them into exec.go (the pkg/stagecoach precedent — "NOT IP duplication"). Do NOT refactor
// generate.CommitStaged to extract a shared helper (behavior-sensitive core; error semantics differ).

// GOTCHA (timeout = no retry): provider.Execute returns context.DeadlineExceeded on timeout. Mirror
// CommitStaged: bail IMMEDIATELY (return a timeout error), do NOT consume a retry attempt. Hook then
// never-blocks (exit 0 non-strict). context.Canceled likewise → bail (return error), never-block.

// GOTCHA (write format): git's msg-file (empty case) holds the comment block (# lines). Prepend msg,
// ensure ONE blank separator, then the original bytes VERBATIM (do not strip/trim orig — git strips #
// lines itself). File mode 0o644 (it already exists; we rewrite in place). msg already newline-normalized
// by ParseOutput; ensure the file ends with the original trailing content (no truncation).

// GOTCHA (silent exit): the strict-abort prints its own one-line stderr message, then returns
// exitcode.New(exitcode.Error, nil) — nil err so main does NOT double-print (ExitError.Error()=="").
// Same idiom as default_action.go's foreign-refusal / handleGenError silent paths.

// GOTCHA (flag --strict placement): S1's script is `exec stagecoach hook exec --strict "$@"` — --strict
// is a LOCAL flag on hookExecCmd, BEFORE the positionals. cobra parses it correctly with RangeArgs(1,3).
// It is distinct from `hook install --strict` (S2, local to installCmd) — no collision.

// GOTCHA (e2e PATH): the installed hook runs `stagecoach` from $PATH (S1 script is not absolute). The e2e
// must prepend the built stagecoach binary's directory to PATH in `git commit`'s environment, and set
// STAGECOACH_CONFIG (stub) + STAGECOACH_STUB_* + GIT_EDITOR=true (so git doesn't open a real editor).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/hook/exec.go  (NEW — package hook; same package as S1's script.go)
package hook

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/generate" // FinalizeMessage, ExtractSubject, IsDuplicate
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/prompt"
	"github.com/dustin/stagecoach/internal/provider"
	"github.com/dustin/stagecoach/internal/ui"
)

// ErrNoOp indicates Run declined to generate (FR-H4): a named message source was present, or the
// staged diff was empty. The caller exits 0 silently — this is the intended no-op, NOT a failure.
var ErrNoOp = errors.New("stagecoach: hook no-op (message source present or nothing staged)")

// noOpSources are the prepare-commit-msg sources where a message already exists (architecture §3).
// A plain `git commit` passes NO source (absent) — that is the empty case stagecoach fills.
var noOpSources = map[string]struct{}{
	"message": {}, "template": {}, "merge": {}, "squash": {}, "commit": {},
}

// NoOpSource reports whether source names a prepare-commit-msg path that already has a message
// (FR-H4). Empty/absent source (plain `git commit`) ⇒ false (proceed).
func NoOpSource(source string) bool {
	_, ok := noOpSources[source]
	return ok
}

// WriteMessageFile prepends msg to git's prepare-commit-msg file at <path>, preserving git's comment
// block beneath it verbatim (FR-H4). The message goes first; a single blank line separates it from the
// original content. git strips `#` comment lines itself on commit. Called ONLY on a fully-accepted msg.
func WriteMessageFile(path, msg string) error {
	orig, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read message file: %w", err)
	}
	var b strings.Builder
	b.WriteString(msg)
	b.WriteByte('\n')
	if len(orig) > 0 {
		b.WriteByte('\n') // exactly one blank separator line
		b.Write(orig)     // git's comment block, VERBATIM
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/hook/exec.go — NoOpSource + WriteMessageFile + ErrNoOp (pure helpers)
  - IMPLEMENT the data-models block above verbatim. NoOpSource over the 5 named sources; "" ⇒ false.
  - WriteMessageFile: read orig (tolerate absent), write msg+"\n" + ("\n"+orig if orig non-empty), mode 0o644.
  - PLACEMENT: internal/hook/exec.go (new file, package hook).

Task 2: CREATE internal/hook/exec.go — buildSystemPrompt + recentSubjects (MIRROR generate.go)
  - COPY generate.buildSystemPrompt (unexported) and recentSubjects verbatim into exec.go (the
    pkg/stagecoach precedent). They call the exported prompt.BuildSystemPrompt/BuildFallbackPrompt +
    prompt.DetectMultiline + git.CommitCount/RecentMessages/RecentSubjects. Rename to hookSystemPrompt/
    hookRecentSubjects to avoid any future same-package clash (hook is a different package, but be explicit).

Task 3: CREATE internal/hook/exec.go — Run (the source-gated never-block runtime)
  - SIGNATURE: func Run(ctx, deps generate.Deps, cfg config.Config, msgFile, source string) error
  - STEP A (source gate, FR-H4): if NoOpSource(source) → return ErrNoOp.
  - STEP B (capture): diff := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{MaxDiffBytes, MaxMdLines,
    BinaryExtensions, Excludes: deps.Excludes}); on err → return err; if diff=="" → return ErrNoOp.
  - STEP C (parent/unborn): _, isUnborn, err := deps.Git.RevParseHEAD(ctx); on err → return err.
  - STEP D (prompts): sysPrompt via hookSystemPrompt(ctx, deps.Git, cfg, isUnborn); recent via
    hookRecentSubjects(ctx, deps.Git, isUnborn); on err → return err.
  - STEP E (role/model): _, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg).
    retryInstr := *deps.Manifest.Resolve().RetryInstruction.
  - STEP F (LOOP — mirror CommitStaged step 5, bounded by cfg.MaxDuplicateRetries): per attempt build
    payload (BuildUserPayload; prepend retryInstr+"\n\n" if prev parse failed) → Manifest.Render(msgModel,
    sysPrompt, payload, msgReasoning) [on err return] → provider.Execute(ctx,*spec,cfg.Timeout,deps.Verbose)
    [on DeadlineExceeded/Canceled → return fmt.Errorf("generation timed out"/"cancelled")] → ParseOutput
    [!ok → parseFail, candidate, VerboseRetry, continue] → FinalizeMessage → ExtractSubject → IsDuplicate
    [dup → rejected append, VerboseRetry, continue] → else WRITE + return nil.
  - STEP G (exhaustion): after the loop with no success → return fmt.Errorf("commit generation failed
    after %d retries", cfg.MaxDuplicateRetries). (Caller never-blocks.) NO RescueError, NO signal.

Task 4: CREATE internal/hook/exec_test.go — in-process (stubtest.Manifest + temp git repo)
  - TestNoOpSource: the 5 named sources true; ""/"chat"/"foo" false.
  - TestWriteMessageFile_PreservesComments: seed a msg-file with "# comment\n# more\n"; write "feat: x";
    assert output == "feat: x\n\n# comment\n# more\n" (comment block verbatim, one blank separator).
  - TestRun_SourceGateNoOp: source="message"/"merge"/"commit" → ErrNoOp; msg-file UNCHANGED.
  - TestRun_EmptyDiffNoOp: repo with no staged changes → StagedDiff=="" → ErrNoOp.
  - TestRun_HappyPath: stub Out="feat: add x\n\nbody"; stage a file; Run(nil) → msg-file starts with the
    message; original comment block preserved beneath.
  - TestRun_ParseFailRetryThenOK: stub script returning empty then valid (stubtest NewScript counter) →
    success on retry; VerboseRetry observed.
  - TestRun_DuplicateRejected: seed a recent commit whose subject the stub emits → retry with a different
    one → success (or exhaustion → descriptive error, msg-file UNTOUCHED).
  - TestRun_StubExit1_NeverBlock: stub Exit=1 → Run returns non-nil non-ErrNoOp error; msg-file UNCHANGED
    (byte-for-byte equal to pre-run snapshot).
  - TestRun_TimeoutNeverBlock: stub SleepMS large + cfg.Timeout tiny → Run returns timeout error;
    msg-file UNCHANGED.
  - TestRun_NoPlumbing: a source-grep test (or build-time assertion) that exec.go references none of
    WriteTree/CommitTree/UpdateRefCAS/DiffTree/signal. (Use `go vet` + a grep in the test via //go:generate
    or a simple strings.Contains over the source read at test time.)

Task 5: CREATE internal/cmd/hookexec.go — the `hook exec` cobra leaf + runHookExec + init()
  - VARS: flagHookExecStrict bool; hookExecCmd (&cobra.Command{Use:"exec <msg-file> [<source> [<sha>]]",
    Short, Long (documented called-by-git), Args: cobra.RangeArgs(1,3), SilenceErrors, SilenceUsage,
    RunE: runHookExec}).
  - init(): hookExecCmd.Flags().BoolVar(&flagHookExecStrict, "strict", false, "Abort the commit on
    generation failure (default: never block — exit 0 and leave the message empty)");
    hookCmd.AddCommand(hookExecCmd).  // hookCmd is S2's var; NO edit to hook.go.
  - runHookExec: repoDir:=os.Getwd(); g:=git.New(repoDir); stderr:=cmd.ErrOrStderr(); neverBlock :=
    func(err error) — prints ONE stderr line "stagecoach: <err>" and returns exitcode mapping
    (nil if !strict, exitcode.New(exitcode.Error,nil) if strict). Then:
      (1) cfg, err := config.Load(cmd.Context(), config.LoadOpts{ConfigPathOverride: flagConfig,
          RepoDir: repoDir, Flags: cmd.Flags()}); on err → neverBlock(err).
      (2) resolve manifest (mirror runDefault: DecodeUserOverrides→NewRegistry→msgProvider via
          ResolveRoleModel("message") or DefaultProvider(installed)→Get→Validate→IsInstalled→apply
          cfg.Output/StripCodeFence); on err → neverBlock(err).
      (3) verbose := ui.NewVerbose(stderr, cfg.Verbose); excludes := exclude.ResolveExcludePathspecs(*cfg,
          repoDir, verbose); on err → neverBlock(err).
      (4) msgFile:=args[0]; source:="" ; if len(args)>=2 { source=args[1] }.
      (5) u := ui.New(stdout, stderr, color); u.Progress(ui.ProgressLabel("Generating", msgModel,
          labelProvider))  // best-effort stderr line; message goes to msgFile, NOT stdout.
      (6) err := hook.Run(cmd.Context(), generate.Deps{Git:g, Manifest:m, Verbose:verbose, Excludes:
          excludes}, *cfg, msgFile, source).
      (7) err==nil || errors.Is(err, hook.ErrNoOp) → return nil (exit 0). else → neverBlock(err).
  - GOTCHA: wrap the WHOLE body so ANY error (incl. config.Load / manifest / excludes) funnels through
    neverBlock. Do NOT os.Exit; return *exitcode.ExitError.

Task 6: CREATE internal/cmd/hookexec_test.go — cobra-leaf tests
  - TestHookExec_SourceGateExit0: run `hook exec <msgfile> message` in a temp repo; assert exit 0 and
    msg-file unchanged (Run returns ErrNoOp → nil). (Config still loads — give it a stub config or rely
    on the no-bootstrap path; source gate short-circuits before generation.)
  - TestHookExec_RangeArgs: too few/many args → cobra usage error (exit 1 via exitcode path or cobra).
  - TestHookExec_StrictFailureNonZero: stub failing (Exit=1) + --strict → exit code 1 (errors.As
    *exitcode.ExitError, Code==1); one stderr line; msg-file unchanged.
  - TestHookExec_NonStrictFailureExit0: same failure without --strict → exit 0; msg-file unchanged.
  - Use the providers_test.go harness: SetArgs/SetOut/SetErr; assert exit code via errors.As *ExitError.

Task 7: EDIT docs/cli.md — add ### hook exec (Mode A)
  - After "### hook status" (before "### providers list"): document `hook exec <msg-file> [<source>
    [<sha>]]` — called by the installed prepare-commit-msg hook (not by users); source-gated no-op for
    message/template/merge/squash/commit and empty staged diff; writes the message atop the file
    preserving git's comment block; never-blocks (exit 0) on any failure; --strict aborts; resolves the
    message role like the single-commit path; never decomposes. Cross-link hook install/uninstall/status.

Task 8: EDIT docs/how-it-works.md — add the FR-H7 FAQ entry
  - Add a "## Hook mode vs the snapshot-based flow" (or FAQ) section: plumbing path = atomic +
    stage-while-generating, but pre-commit hooks bypassed; hook mode = pre-commit hooks honored
    (husky/lint-staged), but no snapshot guarantees and generation latency is inside the commit. The two
    compose: hook for `git commit`, flagship `stagecoach` for the atomic path.

Task 9: CREATE internal/e2e/hook_scenarios_test.go (//go:build e2e) — real git commit
  - Helper runHookE2E(t, repo, cfg, stubKnobs, gitArgs...) : env = os.Environ() + PATH prepend
    (dir of buildStagecoach(t)) + STAGECOACH_CONFIG=cfg + stubKnobs + GIT_EDITOR=true (+ HOME);
    run `git -C repo commit <gitArgs>`; return (stdout, stderr, exitCode).
  - Setup: newRepo(t); seedCommit(t, repo,"readme.md","init"); `stagecoach hook install` via runGit-like
    subprocess (buildStagecoach) with --config cfg (so hook exec resolves the stub provider); stage a change.
  - t.Run("happy_path"): stub Out="feat: generated\n"; GIT_EDITOR=true git commit → headSHA message ==
    "feat: generated" (hook filled the file; git accepted it).
  - t.Run("failure_never_block"): stub Exit=1; GIT_EDITOR='sh -c "echo fallback > $1"' git commit →
    commit lands with "fallback" (hook exit-0'd; git continued) — proves never-block. Assert exit 0.
  - t.Run("m_flag_noop"): stub Out="SHOULD NOT APPEAR"; git commit -m "explicit" → headSHA message ==
    "explicit" (source=message → hook no-op'd; stub output never written).
  - Skip gracefully if go toolchain/git absent (mirror harness t.Skipf conventions).
```

### Implementation Patterns & Key Details

```go
// internal/hook/exec.go — Run (the plumbing-stripped generation runtime). MIRRORS
// generate.CommitStaged steps 1/2/4/5; FORBIDS steps 3/7/8/9 (no snapshot/commit/ref).
func Run(ctx context.Context, deps generate.Deps, cfg config.Config, msgFile, source string) error {
	if NoOpSource(source) { // FR-H4: message/template/merge/squash/commit → a message already exists
		return ErrNoOp
	}
	diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
		MaxDiffBytes: cfg.MaxDiffBytes, MaxMdLines: cfg.MaxMdLines,
		BinaryExtensions: cfg.BinaryExtensions, Excludes: deps.Excludes,
	})
	if err != nil {
		return err
	}
	if diff == "" { // FR-H4: empty staged diff → no-op
		return ErrNoOp
	}
	_, isUnborn, err := deps.Git.RevParseHEAD(ctx)
	if err != nil {
		return err
	}
	sysPrompt, err := hookSystemPrompt(ctx, deps.Git, cfg, isUnborn) // mirror generate.buildSystemPrompt
	if err != nil {
		return err
	}
	recent, err := hookRecentSubjects(ctx, deps.Git, isUnborn) // mirror generate.recentSubjects
	if err != nil {
		return err
	}
	_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg) // FR-H6
	retryInstr := *deps.Manifest.Resolve().RetryInstruction

	var rejected []string
	var parseFail bool
	for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
		payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
		if parseFail {
			payload = retryInstr + "\n\n" + payload // FR29 corrective preamble
		}
		spec, rerr := deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)
		if rerr != nil {
			return fmt.Errorf("hook render: %w", rerr)
		}
		out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) {
				return errors.New("stagecoach: hook generation timed out") // no retry; never-block
			}
			if errors.Is(execErr, context.Canceled) {
				return errors.New("stagecoach: hook generation cancelled")
			}
			// non-zero exit: fall through to ParseOutput (partial stdout may be valid)
		}
		m, ok, _ := provider.ParseOutput(out, deps.Manifest)
		if !ok {
			parseFail = true
			deps.Verbose.VerboseRetry(attempt+1, "parse failed (no valid commit message)")
			continue // FR29 retry
		}
		parseFail = false
		m = generate.FinalizeMessage(m, cfg) // §9.19 FR-F8 template, BEFORE dedupe
		subject := generate.ExtractSubject(m)
		if generate.IsDuplicate(subject, recent) {
			rejected = append(rejected, subject)
			deps.Verbose.VerboseRetry(attempt+1, fmt.Sprintf("subject %q matches an existing commit", subject))
			continue // FR32 retry
		}
		return WriteMessageFile(msgFile, m) // SUCCESS — write atop the file, preserve comments
	}
	return fmt.Errorf("stagecoach: hook generation failed after %d retries", cfg.MaxDuplicateRetries)
}
```

```go
// internal/cmd/hookexec.go — the cobra leaf. Loads config ITSELF (hookCmd's no-op PersistentPreRunE
// skips root's), resolves the message manifest (mirror runDefault), and applies the never-block map.
var flagHookExecStrict bool

var hookExecCmd = &cobra.Command{
	Use:   "exec <msg-file> [<source> [<sha>]]",
	Short: "Generate a commit message into git's prepare-commit-msg file (called by the installed hook)",
	Long: `Called by stagecoach's prepare-commit-msg hook — not by users. Generates a message for the
staged diff and writes it at the top of <msg-file>, preserving git's comment block. No-op (exit 0)
when a message source is present (message/template/merge/squash/commit) or nothing is staged. Any
generation failure leaves the file untouched and exits 0 (never block) unless --strict aborts. (PRD §9.20)`,
	Args:          cobra.RangeArgs(1, 3),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runHookExec,
}

func init() {
	hookExecCmd.Flags().BoolVar(&flagHookExecStrict, "strict", false,
		"Abort the commit on generation failure (default: never block — exit 0 and leave the message empty)")
	hookCmd.AddCommand(hookExecCmd) // S2's hookCmd; NO edit to hook.go
}

func runHookExec(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	stderr := cmd.ErrOrStderr()
	repoDir, err := os.Getwd()
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: getwd: %w", err))
	}
	g := git.New(repoDir)
	msgFile := args[0]
	source := ""
	if len(args) >= 2 {
		source = args[1]
	}

	// neverBlock is the FR-H5 contract: ONE stderr line; exit 0 unless --strict (then exit 1, silent).
	neverBlock := func(err error) error {
		fmt.Fprintf(stderr, "stagecoach: %s\n", err)
		if flagHookExecStrict {
			return exitcode.New(exitcode.Error, nil) // silent non-zero → aborts the commit
		}
		return nil // exit 0 → commit proceeds to an empty editor
	}

	cfg, err := config.Load(ctx, config.LoadOpts{ConfigPathOverride: flagConfig, RepoDir: repoDir, Flags: cmd.Flags()})
	if err != nil {
		return neverBlock(fmt.Errorf("config: %w", err)) // hookCmd's no-op PreRun skipped root's load
	}
	// Resolve the message-role manifest (mirror runDefault / buildDeps).
	overrides, oerr := provider.DecodeUserOverrides(cfg.Providers)
	if oerr != nil {
		return neverBlock(fmt.Errorf("provider overrides: %w", oerr))
	}
	reg := provider.NewRegistry(overrides)
	msgProvider, msgModel, _ := config.ResolveRoleModel("message", cfg)
	name := msgProvider
	if name == "" {
		var installed []string
		for _, m := range reg.List() {
			if reg.IsInstalled(m) {
				installed = append(installed, m.Name)
			}
		}
		name = reg.DefaultProvider(installed)
	}
	m, ok := reg.Get(name)
	if !ok {
		return neverBlock(fmt.Errorf("unknown provider %q", name))
	}
	if verr := m.Validate(); verr != nil {
		return neverBlock(fmt.Errorf("provider %q: %w", name, verr))
	}
	if !reg.IsInstalled(m) {
		return neverBlock(fmt.Errorf("provider %q: command %q not found", name, m.DetectCommand()))
	}
	if cfg.Output != nil {
		m.Output = cfg.Output
	}
	if cfg.StripCodeFence != nil {
		m.StripCodeFence = cfg.StripCodeFence
	}

	verbose := ui.NewVerbose(stderr, cfg.Verbose)
	excludes, eerr := exclude.ResolveExcludePathspecs(*cfg, repoDir, verbose)
	if eerr != nil {
		return neverBlock(fmt.Errorf("resolve excludes: %w", eerr))
	}

	// Best-effort progress line (stderr; the message itself goes to msgFile, never stdout).
	labelProvider := name
	u := ui.New(cmd.OutOrStdout(), stderr, ui.ResolveColor(cfg.NoColor, ui.IsTerminal(os.Stdout)))
	u.Progress(ui.ProgressLabel("Generating", msgModel, labelProvider))

	if rerr := hook.Run(ctx, generate.Deps{Git: g, Manifest: m, Verbose: verbose, Excludes: excludes},
		*cfg, msgFile, source); rerr != nil && !errors.Is(rerr, hook.ErrNoOp) {
		return neverBlock(rerr) // generation failure → never-block (or strict abort)
	}
	return nil // success OR intended no-op → exit 0
}
```

### Integration Points

```yaml
CONSUMES (do NOT re-implement):
  - internal/hook/script.go (S1): Marker, hookScript (confirms the exec line shape).
  - internal/cmd/hook.go (S2): the package-level hookCmd var (hook exec attaches via AddCommand).
  - git.HooksPath (S1): NOT used by the runtime (only by install/uninstall).
  - EXPORTED primitives: prompt.Build{System,Fallback}Prompt/BuildUserPayload/DetectMultiline;
    provider.{DecodeUserOverrides,NewRegistry,Manifest.Render,Execute,ParseOutput};
    config.{Load,ResolveRoleModel}; generate.{FinalizeMessage,ExtractSubject,IsDuplicate};
    exclude.ResolveExcludePathspecs; git.StagedDiff/RevParseHEAD/CommitCount/RecentMessages/RecentSubjects.

REGISTERS:
  - hookCmd.AddCommand(hookExecCmd) from internal/cmd/hookexec.go init() — NO edit to hook.go or root.go.

PROVIDES (to later work):
  - hook.Run / hook.WriteMessageFile / hook.NoOpSource — reusable runtime surface.
  - P1.M5.T1.S1 adds the FR-E4 `--edit` usage-error rejection ON TOP of hookExecCmd (this subtask does NOT).

DOCS:
  - docs/cli.md: ### hook exec (after ### hook status). docs/how-it-works.md: FR-H7 FAQ section.

OUT OF SCOPE (do NOT touch):
  - hook install/uninstall/status (S2); HooksPath/hookScript/Marker/ScriptMode (S1); root.go;
    generate.CommitStaged / pkg/stagecoach (consumed via MIRROR, not modified); the FR-E4 --edit rejection
    (P1.M5.T1.S1); README surfacing (P1.M7.T1.S1); decomposition (hook mode never decomposes).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/hook/exec.go internal/hook/exec_test.go internal/cmd/hookexec.go internal/cmd/hookexec_test.go \
  internal/e2e/hook_scenarios_test.go
go build ./...        # hook exec leaf + runtime must compile against S1/S2 + exported primitives
go vet ./...
golangci-lint run
# Expected: zero errors. Confirm exec.go has NO WriteTree/CommitTree/UpdateRefCAS/DiffTree/signal refs:
! grep -nE 'WriteTree|CommitTree|UpdateRefCAS|DiffTree|signal\.' internal/hook/exec.go && echo "no-plumbing OK"
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/hook/... -run 'NoOpSource|WriteMessageFile|Run' -v   # source gate, write, never-block, no-plumbing
go test ./internal/cmd/...  -run HookExec -v                            # cobra leaf: RangeArgs, source gate, strict exit
go test ./internal/hook/... ./internal/cmd/...                          # ensure existing hook(S2)/providers/config tests still pass
go test -race ./internal/hook/... ./internal/cmd/...
# Expected: all pass. On stub-exit-1 / timeout: msg-file byte-identical to pre-run; comment block preserved
# beneath the message on the happy path.
```

### Level 3: Integration Testing (System Validation)

```bash
# Drive the real binary + a real git commit through the installed hook.
go build -o /tmp/stagecoach ./cmd/stagecoach
stub=$(go build -o /tmp/stubagent ./cmd/stubagent)
cat > /tmp/hookcfg.toml <<EOF
config_version = 3
[provider.stub]
command = "/tmp/stubagent"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
EOF
tmp=$(mktemp -d); cd "$tmp"
git init -q && git config user.name T && git config user.email t@e.co
echo a > a.txt && git add a.txt && git commit -q -m seed
PATH="/tmp:$PATH" STAGECOACH_CONFIG=/tmp/hookcfg.toml /tmp/stagecoach hook install   # installs the hook
echo b >> a.txt; echo c > c.txt; git add a.txt c.txt
# happy path: stub emits a message; GIT_EDITOR=true accepts it as-is
STAGECOACH_STUB_OUT='feat: add c' STAGECOACH_CONFIG=/tmp/hookcfg.toml GIT_EDITOR=true \
  PATH="/tmp:$PATH" git commit -q 2>err.txt; cat err.txt    # progress/never-block line on stderr
git log -1 --format=%s   # → feat: add c   (hook filled the file)
# -m no-op: source=message → hook exits 0 having done nothing
STAGECOACH_STUB_OUT='SHOULD NOT APPEAR' STAGECOACH_CONFIG=/tmp/hookcfg.toml GIT_EDITOR=true \
  PATH="/tmp:$PATH" git commit -q -m "explicit message" 2>/dev/null
git log -1 --format=%s   # → explicit message   (stub output never landed)
# Expected: happy-path commit message == stub output; -m commit keeps the explicit message; hook never aborts.
```

### Level 4: E2E Scenario Harness (System Validation)

```bash
go test -tags e2e ./internal/e2e/... -run TestE2EHookScenarios -v
# Expected: happy_path (HEAD == stub msg), failure_never_block (HEAD == "fallback"; commit proceeded),
# m_flag_noop (HEAD == "explicit"; stub never ran) all pass. Skips cleanly if git/go absent.
# Optional real-agent run (wired via STAGECOACH_RUN_REAL=1 + STAGECOACH_E2E_PROVIDER) validates against a
# live agent CLI — not required for green.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...`; `go test ./...`; `go test -tags e2e ./internal/e2e/...` green; existing S1/S2 tests unchanged.
- [ ] `go vet ./...` + `golangci-lint run` clean; `gofmt -l` empty.
- [ ] `grep -nE 'WriteTree|CommitTree|UpdateRefCAS|DiffTree|signal\.' internal/hook/exec.go` → no matches (no plumbing).

### Feature Validation
- [ ] Source gate: message/template/merge/squash/commit → exit 0, msg-file unchanged; absent/empty source → proceeds.
- [ ] Empty staged diff → exit 0, msg-file unchanged (ErrNoOp).
- [ ] Happy path: generated message written at TOP of msg-file; git's comment block preserved verbatim beneath.
- [ ] Never-block: stub exit 1 / timeout / parse-exhaust / dup-exhaust → ONE stderr line, msg-file UNCHANGED, exit 0.
- [ ] `--strict`: same failure → exit non-zero (aborts the commit); still only one stderr line.
- [ ] `-m`/`-t`/merge/squash/`--amend`: hook no-ops; the explicit message wins.
- [ ] Resolves the message role like the single-commit path (--message-* / [role.message] / env / config); never decomposes.
- [ ] hook exec loads config itself (Config() is nil under hookCmd's no-op PersistentPreRunE); a config error never-blocks.

### Code Quality Validation
- [ ] Runtime is a MIRROR of CommitStaged's generation steps (no refactor of generate.go / pkg/stagecoach).
- [ ] No edit to S1's script.go or S2's hook.go (exec leaf in a NEW file; attaches via hookCmd.AddCommand).
- [ ] No edit to root.go; no new third-party dependencies; no os.Exit in RunE (returns *exitcode.ExitError).
- [ ] WriteMessageFile called ONLY on a fully-accepted message (never on a failure path).
- [ ] Output via cmd.OutOrStdout()/ErrOrStderr() (tests can capture); the message goes to msgFile, never stdout.

### Documentation & Deployment
- [ ] docs/cli.md `### hook exec` (Mode A) documents args, source gate, never-block/--strict, message-role resolution, no-decompose.
- [ ] docs/how-it-works.md FR-H7 FAQ: plumbing (atomic, bypass pre-commit) vs hook (honored, no snapshot) — they compose.

---

## Anti-Patterns to Avoid

- ❌ Don't call `pkg/stagecoach.GenerateCommit` or `generate.CommitStaged` — they snapshot+commit; hook mode must NOT (FR-H4). Mirror the generation steps only.
- ❌ Don't add WriteTree/CommitTree/UpdateRefCAS/DiffTree or touch internal/signal in the runtime — git owns this commit.
- ❌ Don't construct `*RescueError`/`*CASError` or print the §18.3 recovery recipe — there is no snapshot to recover; a failure is one stderr line + exit 0.
- ❌ Don't write the message file on any failure path — WriteMessageFile runs ONLY after an accepted message (the never-block invariant is "msg-file UNTOUCHED on failure").
- ❌ Don't rely on `Config()` — hookCmd's no-op PersistentPreRunE (S2) means it's nil for `hook exec`; load config yourself.
- ❌ Don't edit S2's `internal/cmd/hook.go` or S1's `internal/hook/script.go` — consume them; put the exec leaf in a new file.
- ❌ Don't treat `NoOpSource("")` as true — absent source (plain `git commit`) is the empty case we FILL (architecture §3).
- ❌ Don't retry on timeout — mirror CommitStaged: bail immediately (return error → never-block).
- ❌ Don't add the FR-E4 `--edit` rejection here — that's P1.M5.T1.S1.
- ❌ Don't print more than one stderr line on failure, and don't print to stdout (the message belongs in the file).
- ❌ Don't import `pkg/stagecoach` from `internal/` — copy the ~15-line manifest resolution inline (runDefault/buildDeps already duplicate it).

---

## Confidence Score

**8.5/10** for one-pass implementation success. The runtime is a faithful mirror of an existing, tested
pipeline (CommitStaged) with the plumbing stripped — every primitive it needs is already exported with
stable signatures, and the loop body is written out verbatim. The never-block contract and source gate are
small and fully specified. The −1.5 is concentrated in three places: (1) the e2e env recipe (PATH +
STAGECOACH_CONFIG + GIT_EDITOR plumbing for a real `git commit` driving the hook — the most likely spot for
a subprocess/env surprise, neutralized by the Level-3 manual recipe that mirrors it); (2) the
config-load-self step (a subtle S2 contract — documented with its mechanism, but easy to get wrong if an
implementer assumes `Config()` works); and (3) the message-file write format edge cases (orig already
starting with `\n`). All three are covered by explicit tests in the blueprint. The mirror-not-extract
decision is the established codebase pattern (pkg/stagecoach) and keeps the behavior-sensitive core untouched.
