name: "P1.M5.T2.S1 — --push plumbing, streaming push, and skip conditions"
description: |
  Add the `--push` workflow convenience (PRD §9.22 FR-P1/P2/P3, → G19): a config-surface (full 5-layer
  precedence) `push` knob (default false) that, AFTER THE ENTIRE RUN PUBLISHES CLEANLY, runs plain
  `git push` (NO arguments) STREAMING its stdout/stderr verbatim to the user's terminal. Never prompts
  (FR-P1). On push failure the COMMITS STAND (FR-P2): stagecoach does NOT auto-`--set-upstream`
  (publishing a new branch is the user's call); it shows git's stderr verbatim, prints the closing
  note "commits created; push failed" to stderr, and EXITS 1. Push is SKIPPED ENTIRELY (FR-P3) on
  `--dry-run`, on the zero-commits exit-2 path, and on any rescue/CAS abort — push happens ONLY after a
  fully-clean run.

  IMPLEMENTATION STRATEGY (see research/streaming-push-design.md):
  (1) A NET-NEW streaming `git.Git.Push(ctx, stdout, stderr io.Writer) error` method on the git runner —
      the existing `run()`/`runWithInput()` helpers both CAPTURE stdout/stderr into `bytes.Buffer` and
      CANNOT stream verbatim. `Push` wires `cmd.Stdout`/`cmd.Stderr` directly to the passed writers
      (the CLI passes `os.Stdout`/`os.Stderr`), targets the repo via `-C` (goroutine-safe convention),
      runs `git push` with NO args, never adds `--set-upstream` (FR-P2), and on non-zero exit returns a
      wrapped error carrying git's exit code.
  (2) A small CLI helper `runPush(ctx, stderr, g, cfg) error` invoked at the TWO success returns in
      `internal/cmd/default_action.go` (single path: after `printCommitReport`, before `return nil`;
      decompose path: after the commit-print loop, before `return nil`). It is gated by `cfg.Push`
      (default false → no-op, byte-identical to today). FR-P3's three skip cases are STRUCTURALLY
      UNREACHABLE at the success return (dry-run early-returns; exit-2 returns an error; rescue/CAS
      returns an error) — placing push at the success return makes the skip conditions hold by
      construction. The closing note "commits created; push failed" is printed to stderr BEFORE the
      error is returned (so it always lands); a plain `fmt.Errorf("git push failed: %w", err)` flows
      to `exitcode.For()` → exit 1 (the default tail — NO new sentinel, NO new mapping).
  (3) Full config precedence for `push` (FR-P1: `--push` / `STAGECOACH_PUSH` / `stagecoach.push` /
      `[generation].push`, default false) — mirrors `Template` (P1.M2.T2.S2), NOT the flag-only `Context`.
  (4) Docs: docs/cli.md `--push` row (incl. the no-auto-`--set-upstream` stance + failure semantics),
      docs/configuration.md `push` key.

  RESEARCH NOTE (architecture/external_deps.md §8, VERIFIED empirically on git 2.54.0): a no-upstream
  `git push` exits 128 with stderr containing `has no upstream branch` + the `--set-upstream` hint.
  `push.autoSetupRemote=true` (present in the developer's real global config) MASKS this — tests MUST
  run git with `GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null` and assert on stable SUBSTRINGS
  (`has no upstream branch`, `--set-upstream`), not full text. stagecoach streams git's stderr verbatim
  and never auto-sets upstream (FR-P2).

  CONSUMES: the existing `git.Git` boundary (adds ONE read-ish method that runs a network-mutating git
  command — the ONLY such method; all other methods are local-repo plumbing); the two success returns
  in `default_action.go`; the standard 5-layer config resolver (file → git → env → flag). PROVIDES:
  `--push`, `Config.Push`, `Git.Push`, `runPush`, docs. NO edits to the generate/decompose orchestrator,
  the prompt package, hook exec, or P1.M5.T1.S1's `--edit` (the two conveniences are independent
  post-run stages — `--edit` is pre-publish per-commit; `--push` is post-publish once; they compose).
  The non-push default path is UNTOUCHED (cfg.Push == false ⇒ runPush is a no-op short-circuit).

---

## Goal

**Feature Goal**: `stagecoach --push` (or `STAGECOACH_PUSH=1`, or `stagecoach.push=true`, or
`[generation].push = true`) runs plain `git push` (no arguments) after a fully-successful run, streaming
its stdout/stderr verbatim to the user's terminal. Push failure does NOT roll back the commits (FR-P2):
stagecoach shows git's stderr verbatim (including the no-upstream hint — stagecoach does NOT auto-
`--set-upstream`), prints the closing note "commits created; push failed", and exits 1. Push is skipped
on `--dry-run`, the zero-commits exit-2 path, and any rescue/CAS abort (FR-P3). Never prompts (FR-P1).

**Deliverable**:
1. `internal/git/git.go` (EXTEND the `Git` interface + impl) — `Push(ctx context.Context, stdout, stderr
   io.Writer) error` (a NET-NEW streaming method; runs `git push` with NO args, wires stdout/stderr
   directly to the passed writers, never adds `--set-upstream`, returns a wrapped error on non-zero exit).
2. `internal/config/config.go` (EDIT) — `Push bool \`toml:"push"\`` under `[generation]` (a config-file
   key, NOT flag-only) + `Defaults()` `Push: false`.
3. `internal/config/load.go` (EDIT) — `loadEnv` reads `STAGECOACH_PUSH` (bool, presence-semantic);
   `loadFlags` reads `--push` (DIRECT set).
4. `internal/config/git.go` (EDIT) — `loadGitConfig` reads `stagecoach.push` (bool via `gitConfigBool`).
5. `internal/cmd/root.go` (EDIT) — `flagPush bool` + `pf.BoolVar(&flagPush, "push", false, "...")`
   (global persistent).
6. `internal/cmd/default_action.go` (EDIT) — `runPush(ctx, stderr, g, cfg) error` helper + invocation at
   the TWO success returns (single path post-`printCommitReport`; decompose path post-commit-print-loop).
7. `docs/cli.md` (EDIT) — `--push` global-flags row (incl. no-auto-`--set-upstream` stance + failure
   semantics); `docs/configuration.md` (EDIT) — the `push` key.
8. Tests — `Git.Push`: clean push to a temp bare remote (exit 0, output streamed); no-upstream failure
   (exit 128, `has no upstream branch` + `--set-upstream` substrings, with `GIT_CONFIG_GLOBAL=/dev/null`);
   push config precedence across flag/env/git-config/file; the CLI skip-on-dry-run + the FR-P2 exit-1 +
   "commits created; push failed" note.

**Success Definition**:
- `--push` unset (and no env/config/git-config source) → every commit path byte-identical to today
  (`runPush` is a `cfg.Push`-gated no-op; all existing tests pass unchanged — the regression guard).
- `stagecoach --push` on a single-commit run with a configured upstream → `git push` runs, its output
  streams verbatim to the terminal, exit 0.
- `stagecoach --push` on a decompose run → push runs ONCE after ALL commits + arbiter resolution land.
- A no-upstream `git push` → stagecoach prints git's stderr verbatim (incl. `has no upstream branch` +
  `--set-upstream`), then "commits created; push failed" to stderr, exits 1; the commits STAND (HEAD
  unchanged by the push failure — verify `git rev-parse HEAD` is the post-commit SHA).
- `stagecoach --dry-run --push` → NO push runs (the dry-run early-return fires before the push site),
  exit 0.
- `stagecoach --push` on a clean-tree exit-2 run → NO push runs (the NothingToCommit return fires before
  the push site), exit 2.
- `stagecoach --push` on a rescue/CAS-aborted run → NO push runs (the error return fires before the push
  site), exit 3 / 1 / 124 as appropriate.
- `STAGECOACH_PUSH=1`, `stagecoach.push=true`, and `[generation].push = true` each enable push without the
  flag (full precedence: flag > env > git-config > file > default false).
- `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l` all green.

## User Persona

**Target User**: the "plan-holder" (PRD §7.1) whose final step after a commit is always `git push`.
Today they run `stagecoach && git push` as two commands (or an alias). `--push` collapses it to one —
the same convenience opencommit offers, but WITHOUT opencommit's interactive prompt (which contradicts
stagecoach's non-interactive design, FR-P1). Their fear: "I committed but forgot to push, and now my
branch is behind." `--push` makes push the run's tail step, opt-in.

**Use Case**: `stagecoach --push` (or `git stagecoach --push`, or the lazygit keybind with `--push` baked
in, or `STAGECOACH_PUSH=1` in their shell rc for an always-push workflow) → generation → commit(s) land →
`git push` streams to the terminal → done. If push fails (no upstream, network, rejected non-fast-
forward), the commits are still there; stagecoach says so and exits 1 so a script catches it.

**User Journey**: `stagecoach --push` → generation → `[<sha>] <subject>` (the FR42 report) → `git push`
output streams verbatim → exit 0. On no-upstream: `git push` stderr streams verbatim → "commits created;
push failed" → exit 1 (the user runs `git push -u origin HEAD` themselves — stagecoach never auto-sets
upstream, FR-P2).

**Pain Points Addressed**: incumbents (opencommit) prompt on push (interactive — bad for scripts/CI);
aicommits has no push. `--push` delivers a NON-INTERACTIVE push that streams git's real output (so the
user sees rejects, no-upstream hints, etc. exactly as git prints them) and never silently invents an
upstream (the user's call — FR-P2).

## Why

- **FR-P1 (PRD §9.22)**: `--push` config surface (full precedence), runs plain `git push` (no args)
  streaming output, after the ENTIRE run publishes, never prompts.
- **FR-P2**: push failure is NOT commit failure — the commits stand; git's stderr is shown verbatim
  (incl. the no-upstream hint — stagecoach does NOT auto-`--set-upstream`); closing note "commits created;
  push failed"; exit 1.
- **FR-P3**: skip conditions — no push on `--dry-run`, on the zero-commits exit-2 path, or on any run
  failure (rescue/CAS abort); push happens ONLY after a fully-clean run.
- **architecture/external_deps.md §8 (VERIFIED)**: the no-upstream `git push` exits 128 with stable
  substrings `has no upstream branch` + `--set-upstream`; `push.autoSetupRemote=true` masks it → tests
  MUST use `GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null`.
- **Scope fences**: CONSUMES the existing `git.Git` boundary + the two success returns in
  `default_action.go` + the standard config resolver. Does NOT touch the generate/decompose orchestrator,
  the prompt package, hook exec, or P1.M5.T1.S1's `--edit`. The non-push default path is untouched
  (`cfg.Push` gates everything).

## What

A full-precedence `push` config knob (default false) that runs plain `git push` streaming verbatim after
a clean run. The streaming `Push` git method is net-new (the existing runners only capture). The CLI
helper is placed at the two success returns, which makes FR-P3's skip conditions hold by construction.

### Success Criteria

- [ ] `internal/git/git.go`: `Push(ctx, stdout, stderr io.Writer) error` added to the `Git` interface +
      implemented on `*gitRunner` (streams `cmd.Stdout`/`cmd.Stderr` to the passed writers; `git push`
      with NO args + `-C <repo>`; never `--set-upstream`; wrapped error on non-zero exit; ctx-aware).
- [ ] `internal/config/config.go`: `Push bool \`toml:"push"\`` under `[generation]` + `Defaults()`
      `Push: false`.
- [ ] `internal/config/load.go`: `loadEnv` `STAGECOACH_PUSH` (bool, presence-semantic, DIRECT set — can
      be false); `loadFlags` `--push` (`fs.Changed("push")` → DIRECT set).
- [ ] `internal/config/git.go`: `loadGitConfig` reads `stagecoach.push` via `gitConfigBool` (camelCase
      key convention — `stagecoach.push` is already lowercase, no camelCase needed; mirrors the bool
      block near `stagecoach.verbose`).
- [ ] `internal/cmd/root.go`: `flagPush bool` + `pf.BoolVar(&flagPush, "push", false, "...")`.
- [ ] `internal/cmd/default_action.go`: `runPush(ctx, stderr, g, cfg) error` + invocation at BOTH
      success returns (single post-`printCommitReport`; decompose post-commit-print-loop). Gated by
      `cfg.Push`; prints "commits created; push failed" to stderr on failure BEFORE returning the error.
- [ ] `--dry-run --push` → no push (dry-run early-returns before the site); exit 2/3/1/124 paths → no
      push (they return errors before the site).
- [ ] No-upstream push → git's stderr streamed verbatim + "commits created; push failed" + exit 1;
      commits STAND (`git rev-parse HEAD` unchanged by the push failure).
- [ ] docs/cli.md `--push` row (no-auto-`--set-upstream` stance + failure semantics); docs/configuration.md
      `push` key.
- [ ] Tests: `Git.Push` (clean push to a temp bare remote; no-upstream failure with
      `GIT_CONFIG_GLOBAL=/dev/null` + substring asserts); config precedence; CLI skip-on-dry-run +
      FR-P2 exit-1 + note.
- [ ] Full build/test/vet/lint/fmt green; `--push` unset → byte-identical to today.

## All Needed Context

### Context Completeness Check

_This PRP names the EXACT net-new streaming method (with the `cmd.Stdout`/`cmd.Stderr` wiring + the
`-C repo` convention + the no-`--set-upstream` stance), the EXACT two success-return insertion points
(with the surrounding anchor calls), the `runPush` helper signature + the closing-note-before-error
ordering, the full-precedence config plumbing (mirroring `Template` — toml tag, env bool, git-config
bool, flag DIRECT set), the FR-P3 structural-unreachability proof (the three skip cases all return
before the success site), the no-upstream test contract (`GIT_CONFIG_GLOBAL=/dev/null` + substring
asserts, external_deps.md §8), the exit-1-via-default-tail proof (no new sentinel/mapping), the
temp-bare-remote E2E setup, and the docs rows. An implementer with no prior codebase knowledge can
build it from this document + codebase access._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/P1M5T2S1/research/streaming-push-design.md
  why: THE design playbook — §1 (push point in the CLI layer, not the orchestrator), §2 (FR-P3 skip
       conditions are STRUCTURALLY UNREACHABLE at the success return — the proof), §3 (the net-new
       streaming Push method + why run()/runWithInput() can't do it), §4 (the no-upstream test contract),
       §5 (exit-1 via exitcode.For's default tail — no new sentinel), §6 (the temp-bare-remote E2E),
       §8 (full-precedence config surface — mirrors Template, NOT Context).
  section: all
  critical: |
    The push site is at the TWO success returns in default_action.go. FR-P3's three skip cases (dry-run,
    exit-2, rescue/CAS) ALL return BEFORE the success site, so placing push there makes the skip
    conditions hold by construction — no explicit skip guard beyond `cfg.Push` is required (a `flagDryRun`
    short-circuit is belt-and-suspenders + the documented contract).

- docfile: plan/005_c38aa48290f0/architecture/external_deps.md
  why: §8 (`git push` no-upstream failure, VERIFIED empirically on git 2.54.0) — exit 128, stderr
       contains `has no upstream branch` + `--set-upstream`; `push.autoSetupRemote=true` (in the dev's
       real global config) MASKS this → tests MUST run git with `GIT_CONFIG_GLOBAL=/dev/null
       GIT_CONFIG_SYSTEM=/dev/null` and assert on stable SUBSTRINGS, not full text. stagecoach streams
       git's stderr verbatim and never auto-sets upstream (FR-P2).
  section: "## 8. `git push` no-upstream failure (gates FR-P2) — VERIFIED (empirical, git 2.54.0)"
  critical: |
    WITHOUT `GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null`, the no-upstream test will PASS
    (push silently succeeds due to autoSetupRemote) — a FALSE GREEN. Every no-upstream test MUST set
    these env vars on the git subprocess (or via t.Setenv for the test process). Assert on substrings
    `has no upstream branch` and `--set-upstream`, NEVER the full stderr line (wording varies).

- docfile: plan/005_c38aa48290f0/prd_snapshot.md   (and PRD.md §9.22 / §15.2 / §15.4)
  why: §9.22 FR-P1 (the config surface + plain `git push` + streaming + never-prompts), FR-P2 (push
       failure ≠ commit failure; commits stand; git stderr verbatim; no auto-`--set-upstream`; closing
       note; exit 1), FR-P3 (skip on dry-run / exit-2 / rescue-CAS); §15.2 (the --push global flag row);
       §15.4 (exit codes — push failure maps to 1, the existing generic-error code).
  section: "§9.22 (FR-P1/P2/P3), §15.2, §15.4"

- file: internal/git/git.go   (EDIT — the Git interface + *gitRunner impl)
  why: ADD the net-new streaming Push method. The existing `run()` (L~300) and `runWithInput()` (L~340)
       BOTH capture stdout/stderr into `bytes.Buffer` and return strings — they CANNOT stream verbatim.
       Push wires `cmd.Stdout`/`cmd.Stderr` directly to the passed `io.Writer`s. It is the ONLY method
       that runs a network-mutating git command (all others are local-repo plumbing); read/write w.r.t.
       REMOTE refs (not local — local refs are untouched by `git push`; it is the remote that moves).
       Target the repo via `-C` (the goroutine-safe convention every method uses via `g.workDir`).
  pattern: |
    // Push (interface doc):
    // Push runs plain `git push` (NO arguments — FR-P1) streaming its stdout/stderr VERBATIM to the
    // passed writers (the CLI passes os.Stdout/os.Stderr so the user sees git's real output, incl. the
    // no-upstream hint, rejected non-fast-forwards, progress, etc.). It NEVER adds `--set-upstream`
    // (FR-P2: publishing a new branch is the user's call). On a non-zero exit (128 = no upstream /
    // rejected; 1 = network) it returns a wrapped error carrying git's exit code; the COMMITS STAND
    // (push failure does not roll back local commits — the caller prints "commits created; push failed"
    // and exits 1). ctx-aware (timeout/signal cancel the push). Targets the repo via -C (goroutine-safe).
    Push(ctx context.Context, stdout, stderr io.Writer) error

    // Push (*gitRunner impl):
    func (g *gitRunner) Push(ctx context.Context, stdout, stderr io.Writer) error {
        gitPath, lerr := exec.LookPath("git")
        if lerr != nil {
            return fmt.Errorf("git binary not found in PATH: %w", lerr)
        }
        // `git -C <repo> push` — NO args after `push` (plain push, FR-P1); NEVER --set-upstream (FR-P2).
        cmd := exec.CommandContext(ctx, gitPath, "-C", g.workDir, "push")
        cmd.Stdout = stdout // STREAM VERBATIM (not a bytes.Buffer)
        cmd.Stderr = stderr // STREAM VERBATIM
        runErr := cmd.Run()
        if runErr == nil {
            return nil
        }
        if cerr := ctx.Err(); cerr != nil { // context cancelled (timeout/signal) — not a git exit
            return cerr
        }
        var exitErr *exec.ExitError
        if errors.As(runErr, &exitErr) { // non-zero git exit → wrapped error (code carried for diagnostics)
            return fmt.Errorf("git push failed (exit %d)", exitErr.ExitCode())
        }
        return fmt.Errorf("git push: %w", runErr) // start / I/O failure
    }
  gotcha: |
    Add Push to the Git INTERFACE (the type, near HooksPath's doc ~L58) AND to *gitRunner (impl). EVERY
    existing test double of Git (search `git.Git` in *_test.go — stubtest, fakeGit, decompose/stagecoach
    test doubles, etc.) must gain a no-op/stub Push impl or the package won't compile. Run
    `go build ./...` immediately after the interface edit to enumerate them all. Push is the ONLY method
    that takes io.Writer params — stubs can ignore them (`func (f *fakeGit) Push(ctx context.Context, _, _ io.Writer) error { return nil }`).
    Do NOT pass `--porcelain`, `--quiet`, or any arg — FR-P1 is PLAIN `git push` (the user sees real output).

- file: internal/cmd/default_action.go   (EDIT — the push site, BOTH success returns)
  why: THE two success returns. (1) SINGLE path: the `// Commit path: FR42 report` block ends with
       `printCommitReport(stdout, res, changes)` then `return nil` — insert runPush BETWEEN them.
       (2) DECOMPOSE path: `runDecompose` prints each landed commit via `printDecomposeCommit`, then
       `if derr != nil { return handleDecomposeError(derr) }; return nil` — insert runPush BEFORE the
       `return nil` (after the commit-print loop, only on the derr==nil success branch). BOTH sites are
       naturally after the dry-run early-return, the exit-2 returns, and the rescue/CAS error returns —
       so FR-P3 holds by construction.
  pattern: |
    // SINGLE path — replace `printCommitReport(stdout, res, changes); return nil` with:
    printCommitReport(stdout, res, changes)
    if err := runPush(ctx, stderr, g, *cfg); err != nil { // §9.22 FR-P1/P2 — no-op unless cfg.Push
        return exitcode.New(exitcode.Error, err) // exit 1; commits already stand (FR-P2)
    }
    return nil

    // DECOMPOSE path — in runDecompose, replace `if derr != nil { ... }; return nil` with:
    if derr != nil {
        return handleDecomposeError(derr)
    }
    if err := runPush(ctx, stderr, g, *cfg); err != nil { // §9.22 FR-P1/P2
        return exitcode.New(exitcode.Error, err)
    }
    return nil

    // runPush helper (add near handleGenError):
    // runPush runs `git push` (plain, streaming) after a fully-clean run, iff cfg.Push is true (§9.22
    // FR-P1). It is a no-op when push is disabled (the default — byte-identical to the pre-feature path).
    // On push failure the COMMITS STAND (FR-P2): git's stderr was already streamed verbatim by Push, so
    // print the closing note "commits created; push failed" to stderr and return a wrapped error (the
    // caller maps it to exit 1 via exitcode.For's default tail). Never prompts; never auto-sets upstream.
    func runPush(ctx context.Context, stderr io.Writer, g git.Git, cfg config.Config) error {
        if !cfg.Push {
            return nil // THE no-op guard — the byte-identity regression invariant
        }
        if err := g.Push(ctx, os.Stdout, stderr); err != nil { // stream git's stdout/stderr verbatim
            fmt.Fprintln(stderr, "commits created; push failed") // FR-P2 closing note (stderr; BEFORE the err)
            return fmt.Errorf("git push: %w", err)
        }
        return nil
    }
  gotcha: |
    The closing note MUST print to stderr BEFORE returning the error (so it always lands — main's generic
    path would otherwise print only the wrapped "git push: ..." line). Push's stdout is os.Stdout (FR51:
    the FR42 report is on stdout; git push's own stdout — progress, etc. — is fine on stdout too; git
    push writes its human output to STDERR, so the verbatim hint lands on stderr either way). Use *cfg
    (Config() returns *config.Config; pass a VALUE to runPush so it can't be nil-derefed). The dry-run
    path returns BEFORE this site (no explicit dry-run guard needed in runPush — but adding one is
    harmless: `if flagDryRun { return nil }` as belt-and-suspenders; the structural guarantee is the
    early-return in runDefault). Do NOT push on the partial-landing FR-M12 path (runDecompose returns
    via handleDecomposeError on derr != nil — push is only on the derr==nil branch).

- file: internal/config/config.go   (EDIT — Config.Push field, under [generation])
  why: ADD `Push bool \`toml:"push"\`` near Template (L~93) — it is a `[generation]` config-file key
       (full precedence, FR-P1), NOT flag-only. Add to Defaults(): `Push: false`.
  pattern: |
    // Push is the §9.22 FR-P1 --push workflow convenience (full 5-layer precedence: --push /
    // STAGECOACH_PUSH / stagecoach.push / [generation].push, default false). When true, a plain `git push`
    // (no args, streaming) runs AFTER a fully-clean run. Push failure does NOT roll back commits (FR-P2):
    // git's stderr is streamed verbatim, "commits created; push failed" prints, exit 1. Skipped on
    // --dry-run, the exit-2 path, and any rescue/CAS abort (FR-P3). See cmd.runPush + git.Git.Push.
    Push bool `toml:"push"`
    // ...in Defaults(): Push: false,
  gotcha: "Mirror Template (full precedence, toml:\"push\") — NOT Context (flag-only, toml:\"-\"). FR-P1
           names all four sources explicitly."

- file: internal/config/load.go   (EDIT — loadEnv STAGECOACH_PUSH + loadFlags --push)
  why: loadEnv (L~184) reads STAGECOACH_* bool vars (VERBOSE/NO_COLOR block L~201). Add STAGECOACH_PUSH
       (presence-semantic, strconv.ParseBool, DIRECT set — can be false). loadFlags (L~259) reads
       Config-backed flags; add a `--push` block (fs.Changed("push") → DIRECT set, like --verbose).
  pattern: |
    // in loadEnv, after the STAGECOACH_NO_COLOR block:
    if v, ok := os.LookupEnv("STAGECOACH_PUSH"); ok && v != "" {
        b, err := strconv.ParseBool(v)
        if err != nil {
            return fmt.Errorf("STAGECOACH_PUSH: %w", err)
        }
        cfg.Push = b // DIRECT set — can be false (escape hatch)
    }

    // in loadFlags, near the --verbose/--no-color block:
    if fs.Changed("push") {
        if v, err := fs.GetBool("push"); err == nil {
            cfg.Push = v // DIRECT set
        }
    }
  gotcha: "ParseBool accepts 1/0/true/false/TRUE/etc. Presence-semantic (empty string ⇒ not set ⇒ fall
           through), mirroring STAGECOACH_VERBOSE. DIRECT set (not overlay) so --push=false / STAGECOACH_PUSH=0
           work as escape hatches."

- file: internal/config/git.go   (EDIT — loadGitConfig reads stagecoach.push)
  why: loadGitConfig (L~110) reads `stagecoach.*` keys. The bool block (L~156, near stagecoach.verbose)
       uses `gitConfigBool(repoDir, "stagecoach.<key>")`. Add `stagecoach.push` there. NOTE: unlike
       `stagecoach.autoStageAll`/`maxDiffBytes` (which this codebase stores in camelCase), `push` is a
       single word — the key is literally `stagecoach.push` (no camelCase transformation).
  pattern: |
    // in the booleans block (after stagecoach.verbose):
    if v, found, err := gitConfigBool(repoDir, "stagecoach.push"); err != nil {
        return nil, err
    } else if found {
        c.Push = v
    }
  gotcha: "gitConfigBool uses `--bool` (canonicalizes to true/false — FINDING C in this file). The key
           is `stagecoach.push` (lowercase, single word — do NOT write `stagecoach.Push`)."

- file: internal/cmd/root.go   (EDIT — register --push global persistent flag)
  why: flagFormat/flagTemplate/flagContext are at L~74-81; their BoolVar/StringVar registrations follow
       (L~155-167). Add `flagPush bool` to the var block + a `pf.BoolVar(&flagPush, "push", false, "...")`.
  pattern: |
    // near L74-81 (the flagFormat/flagTemplate/flagContext var block): add
    var flagPush bool
    // after the --context StringVar (L~167):
    pf.BoolVar(&flagPush, "push", false,
        "Run plain `git push` (streaming) after a fully-successful run. Never prompts; never auto-sets "+
            "upstream. On push failure the commits stand — git's stderr is shown verbatim, "+
            "\"commits created; push failed\" prints, and stagecoach exits 1. Skipped on --dry-run, the "+
            "nothing-to-commit exit, and any rescue/CAS abort. (env STAGECOACH_PUSH, git stagecoach.push, "+
            "config [generation].push; default false.) (§9.22 FR-P1)")
  gotcha: "GLOBAL persistent flag — inherited by subcommands. hook exec should IGNORE it silently (push "+
            "is meaningless in hook mode — git owns the commit; a push belongs in the user's post-commit "+
            "flow, not the hook). Unlike --edit (which hookexec.go REJECTS as a usage error, FR-E4), --push "+
            "in hook exec is a silent no-op (cfg.Push is only CHECKED at the default-action success return; "+
            "hook exec never reaches that site). No hookexec.go edit needed."

- file: internal/git/git_test.go   (REF + EXTEND — the git test harness + Push tests)
  why: initRepo(t, dir) (L~14) is the test-repo bootstrap (git init + user.name/email). EXTEND with Push
       tests. The clean-push E2E needs a temp BARE remote (`git init --bare <bare>`), `git remote add
       origin <bare>`, an initial `git push -u origin HEAD` (TEST SETUP — not stagecoach's job), then a
       new commit, then `g.Push(ctx, os.Stdout, os.Stderr)` → assert exit nil + the remote advanced. The
       no-upstream test MUST set `GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null` on the push
       subprocess (or via t.Setenv) and assert on substrings.
  pattern: |
    // clean push to a temp bare remote:
    func TestPush_CleanToBareRemote(t *testing.T) {
        repo := t.TempDir(); initRepo(t, repo)
        bare := t.TempDir()
        if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil { t.Fatalf(...) }
        mustRun(t, repo, "remote", "add", "origin", bare)
        writeAndCommit(t, repo, "a.txt", "a") // helper: write file, git add, git commit
        mustRun(t, repo, "push", "-u", "origin", "HEAD") // TEST SETUP: establish upstream
        // now add a NEW commit and push via the runner:
        writeAndCommit(t, repo, "b.txt", "b")
        g := New(repo)
        var out, errb bytes.Buffer
        if err := g.Push(context.Background(), &out, &errb); err != nil { t.Fatalf("Push err = %v", err) }
        // assert the remote advanced: git -C <bare> log --oneline shows 2 commits
    }

    // no-upstream failure (the FR-P2 contract — external_deps.md §8):
    func TestPush_NoUpstreamFails128(t *testing.T) {
        t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")      // MASKS autoSetupRemote — CRITICAL
        t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")      // (external_deps.md §8)
        repo := t.TempDir(); initRepo(t, repo)
        bare := t.TempDir(); exec.Command("git", "init", "--bare", bare).Run()
        mustRun(t, repo, "remote", "add", "origin", bare)
        writeAndCommit(t, repo, "a.txt", "a")
        // NOTE: deliberately NO `git push -u origin HEAD` — no upstream.
        g := New(repo)
        var errb bytes.Buffer
        err := g.Push(context.Background(), io.Discard, &errb)
        if err == nil { t.Fatal("Push err = nil, want non-nil (no upstream)") }
        if !strings.Contains(errb.String(), "has no upstream branch") { t.Errorf("stderr missing 'has no upstream branch': %q", errb.String()) }
        if !strings.Contains(errb.String(), "--set-upstream") { t.Errorf("stderr missing '--set-upstream': %q", errb.String()) }
        // commits STAND: HEAD is unchanged by the push failure
        // (already committed before Push; Push touches only the remote)
    }
  gotcha: |
    WITHOUT t.Setenv("GIT_CONFIG_GLOBAL","/dev/null"), the no-upstream test PASSES (push silently
    succeeds due to the dev's autoSetupRemote) — a FALSE GREEN. This is the #1 test pitfall
    (external_deps.md §8). Assert on SUBSTRINGS, never full stderr. The `git push -u origin HEAD` in the
    clean-push test is TEST SETUP (establishing upstream) — stagecoach NEVER does this (FR-P2). For
    stub/fake Git impls: Push(ctx, _, _ io.Writer) error { return nil } (no-op stub).

- file: internal/cmd/default_action_test.go   (EXTEND — CLI skip + FR-P2 exit-1 + note)
  why: The CLI-level tests. (1) skip-on-dry-run: `stagecoach --dry-run --push` with staged changes →
       message printed, "(no commit created)", NO push (assert the bare remote is unchanged / a spy Git
       captures no Push call), exit 0. (2) FR-P2 exit-1 + note: a no-upstream repo + `--push` → "commits
       created; push failed" on stderr + exit 1 + the commit landed (HEAD advanced). Use a temp bare
       remote + GIT_CONFIG_GLOBAL=/dev/null for the no-upstream case. (3) byte-identity: `stagecoach`
       WITHOUT --push → no Push call (a spy Git asserts Push was never called).
  pattern: "Use a spy/fake Git that records Push calls for the skip + byte-identity assertions; use a real
            git.New on a temp repo + bare remote for the FR-P2 integration assertion. Mirror the existing
            default_action_test.go harness (it already builds a temp repo + staged changes)."
  gotcha: "The dry-run skip is STRUCTURAL (runDefault early-returns before the push site) — the test
           asserts the OBSERVABLE consequence (no remote advance / no spy Push call), not an internal flag."

- file: docs/cli.md   (EDIT — Mode A, --push global-flags row)
  why: Global-flags table. Add a `--push` row after `--edit` (P1.M5.T1.S1) / near `--dry-run`. Note the
       no-auto-`--set-upstream` stance, the failure semantics (commits stand, exit 1), and the skip
       conditions.
  critical: |
    | `--push` | bool | false | `STAGECOACH_PUSH` | `stagecoach.push` | Run plain `git push` (no arguments,
      streaming its output) after a fully-successful run. Never prompts. On push failure the commits
      stand — git's stderr is shown verbatim (including the no-upstream hint; stagecoach does NOT auto-
      `--set-upstream`), "commits created; push failed" prints, and stagecoach exits 1. Skipped on
      `--dry-run`, the nothing-to-commit exit, and any rescue/CAS abort. Also `[generation].push`. (§9.22
      FR-P1) |

- file: docs/configuration.md   (EDIT — Mode A, the push key)
  why: The config-file reference. Add `push` to the `[generation]` key list + the env/git-config table.
  critical: "Document all four sources (flag/env/git-config/file) + the default false + the failure
             semantics (commits stand, exit 1). Cross-reference docs/cli.md's --push row."

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go           # EDIT — Git interface + *gitRunner: +Push (net-new streaming method)
  git_test.go      # EXTEND — Push clean-push-to-bare-remote + no-upstream-fails-128 (GIT_CONFIG_GLOBAL=/dev/null)
internal/config/
  config.go        # EDIT — Config.Push field ([generation], toml:"push") + Defaults()
  load.go          # EDIT — loadEnv STAGECOACH_PUSH + loadFlags --push
  git.go           # EDIT — loadGitConfig reads stagecoach.push (gitConfigBool)
  load_test.go / git_test.go / file_test.go  # EXTEND — push precedence across flag/env/git/file
internal/cmd/
  root.go          # EDIT — flagPush + --push BoolVar (global persistent)
  default_action.go# EDIT — runPush helper + invocation at BOTH success returns (single + decompose)
  default_action_test.go # EXTEND — skip-on-dry-run + FR-P2 exit-1 + "commits created; push failed" + byte-identity
docs/
  cli.md           # EDIT — --push global-flags row
  configuration.md # EDIT — push key ([generation] + env + git-config tables)
```

### Desired Codebase tree with files to be added and responsibility of file

No NEW files. All work is EXTENSIONS to existing files. `internal/git/git.go` gains the net-new streaming
`Push` method (the only io.Writer-taking method on Git). `internal/cmd/default_action.go` gains the
`runPush` helper + two call sites. The config plumbing mirrors the existing `Template` knob (full
precedence). Every change is additive + guarded by `cfg.Push`.

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the no-upstream test contract — external_deps.md §8, VERIFIED): `git push` on a no-upstream
// branch exits 128 with stderr containing `has no upstream branch` + `--set-upstream`. BUT the developer's
// real global git config sets push.autoSetupRemote=true, which makes the push SILENTLY SUCCEED — masking
// the FR-P2 failure path entirely. Tests MUST run git with GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
// (t.Setenv on the test process propagates to the git subprocess) and assert on stable SUBSTRINGS, never
// full stderr text. WITHOUT this isolation the no-upstream test is a FALSE GREEN.

// CRITICAL (Push is NET-NEW streaming — run()/runWithInput() CANNOT do it): both existing exec helpers
// capture stdout/stderr into bytes.Buffer and return strings. Push wires cmd.Stdout/cmd.Stderr DIRECTLY
// to the passed io.Writers (the CLI passes os.Stdout/os.Stderr → verbatim streaming). Do NOT try to reuse
// run() for push — it would buffer the output and the user would see nothing until push completes (and a
// slow/large push would be invisible). Streaming is the FR-P1 contract ("streaming its output").

// CRITICAL (the Git interface edit ripples to ALL test doubles): adding Push to git.Git means EVERY
// fake/stub Git in the repo (search `git.Git` in *_test.go: stubtest, internal/git fakes, decompose/stagecoach
// test doubles) must gain a Push impl or the package won't compile. Push is the ONLY io.Writer-taking
// method — stubs: `func (f *fakeGit) Push(ctx context.Context, _, _ io.Writer) error { return nil }`.
// Run `go build ./...` immediately after the interface edit to enumerate them all.

// CRITICAL (FR-P3 skip conditions are STRUCTURAL — do not add explicit guards beyond cfg.Push): the
// three skip cases (dry-run, exit-2, rescue/CAS) ALL return from runDefault/runDecompose BEFORE the
// success return where runPush lives. Placing runPush at the success return makes the skip conditions
// hold by construction. A `if flagDryRun { return nil }` inside runPush is harmless belt-and-suspenders
// but NOT required — the structural guarantee is the early-return in runDefault. Do NOT add push to the
// dry-run path, the exit-2 path, or the error paths.

// CRITICAL (the closing note prints BEFORE the error — FR-P2): on push failure, runPush prints "commits
// created; push failed" to stderr, THEN returns the wrapped error. If it returned the error first, main's
// generic path would print only "git push: ..." and the FR-P2 closing note would be lost. The note is the
// user-facing contract ("commits stand; push failed"); the wrapped error is the exit-code vehicle.

// CRITICAL (push failure = exit 1 via the DEFAULT TAIL — no new sentinel/mapping): exitcode.For() already
// maps a generic non-nil error → exitcode.Error (1) as the default tail. A plain fmt.Errorf("git push: %w",
// err) returned from runDefault/runDecompose flows to main → exitcode.For → exit 1. Do NOT add a new
// exitcode constant or a new For() branch for push failure (it is NOT rescue/timeout/CAS/nothing — it is
// a generic post-run error). The COMMITS STAND: push failure does not roll back local commits (git push
// moves the REMOTE; local HEAD is untouched by a push failure).

// CRITICAL (NEVER auto---set-upstream — FR-P2): Push runs `git push` with NO args after `push`. Do NOT
// add `--set-upstream`, `-u`, or any arg. Publishing a new branch is the user's call (FR-P2: "stagecoach
// does not auto---set-upstream; publishing a new branch is the user's call"). The no-upstream failure
// surfaces git's own hint verbatim (the user runs `git push -u origin HEAD` themselves).

// GOTCHA (push stdout to os.Stdout is fine — git writes human output to STDERR): git push's progress/
// status human output goes to STDERR (git convention); its stdout is usually empty (or porcelain output
// only with --porcelain, which we do NOT pass). So passing os.Stdout as Push's stdout writer is harmless
// (git writes little to stdout) and matches the FR51 stream discipline. The FR42 commit report on stdout
// (already printed before runPush) is unaffected — push runs AFTER printCommitReport.

// GOTCHA (push is full-precedence, NOT flag-only — mirror Template, not Context): Config.Push has
// toml:"push" (a config-file key under [generation]); STAGECOACH_PUSH env; stagecoach.push git-config; --push
// flag. All four sources, default false. This is the SAME shape as Template (P1.M2.T2.S2), NOT Context
// (flag-only, toml:"-"). FR-P1 names all four sources explicitly.

// GOTCHA (the git-config key is stagecoach.push — lowercase, single word): unlike stagecoach.autoStageAll
// / maxDiffBytes (which this codebase stores in camelCase because the prose names are multi-word),
// `push` is one word — the key is literally `stagecoach.push`. Do NOT write `stagecoach.Push`.

// GOTCHA (hook exec silently ignores --push — no hookexec.go edit): --push is a GLOBAL persistent flag,
// inherited by hook exec. But cfg.Push is only CHECKED at the default-action success return (runPush),
// which hook exec never reaches (hook exec writes the message file and exits). So --push in hook exec is
// a silent no-op — do NOT add a rejection (unlike --edit, which FR-E4 explicitly rejects as a usage error).
// Push belongs in the user's post-commit flow, not the prepare-commit-msg hook.

// GOTCHA (decompose partial-landing FR-M12 does NOT push): runDecompose returns via handleDecomposeError
// when derr != nil (a concept-i failure leaves already-landed commits standing but aborts the run).
// runPush is ONLY on the derr==nil branch (the fully-clean run). Do NOT push on a partial landing —
// FR-P3: "push happens only after a fully-clean run."

// GOTCHA (--edit and --push are independent — they compose): --edit (P1.M5.T1.S1) gates EACH commit's
// message PRE-publish (inside the orchestrator); --push runs ONCE POST-publish (in the CLI). Neither
// touches the other's code. `stagecoach --edit --push` edits each message, publishes all, then pushes.
// Both are cfg.<Flag>-gated no-ops when off → byte-identity when both unset.
```

## Implementation Blueprint

### Data models and structure

```go
// === internal/git/git.go (EXTEND the Git interface, near HooksPath's doc ~L58, + *gitRunner impl) ===
// Add to the Git interface:
	// Push runs plain `git push` (NO arguments — §9.22 FR-P1) streaming its stdout/stderr VERBATIM to the
	// passed writers (the CLI passes os.Stdout/os.Stderr so the user sees git's real output: progress,
	// the no-upstream hint, rejected non-fast-forwards, etc.). It NEVER adds `--set-upstream` (FR-P2:
	// publishing a new branch is the user's call — stagecoach surfaces git's own hint verbatim instead).
	// On a non-zero exit (128 = no upstream / rejected; 1 = network) it returns a wrapped error carrying
	// git's exit code; the COMMITS STAND (push failure does not roll back local commits — push moves the
	// REMOTE, not local HEAD; the caller prints "commits created; push failed" and exits 1). ctx-aware
	// (timeout/signal cancel the push). Targets the repo via -C (the goroutine-safe convention). Push is
	// the ONLY method on Git that runs a network-mutating command and the ONLY one taking io.Writer params.
	Push(ctx context.Context, stdout, stderr io.Writer) error

// Add to *gitRunner (impl):
func (g *gitRunner) Push(ctx context.Context, stdout, stderr io.Writer) error {
	gitPath, lerr := exec.LookPath("git")
	if lerr != nil {
		return fmt.Errorf("git binary not found in PATH: %w", lerr)
	}
	cmd := exec.CommandContext(ctx, gitPath, "-C", g.workDir, "push") // NO args after "push"; NEVER -u/--set-upstream
	cmd.Stdout = stdout                                               // STREAM VERBATIM (not a bytes.Buffer)
	cmd.Stderr = stderr                                               // STREAM VERBATIM
	runErr := cmd.Run()
	if runErr == nil {
		return nil
	}
	if cerr := ctx.Err(); cerr != nil { // context cancelled (timeout/signal) — not a git exit
		return cerr
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return fmt.Errorf("git push failed (exit %d)", exitErr.ExitCode())
	}
	return fmt.Errorf("git push: %w", runErr) // start / I/O failure
}

// === internal/config/config.go (EDIT — Config.Push field, near Template L~93) ===
	// Push is the §9.22 FR-P1 --push workflow convenience (full 5-layer precedence: --push /
	// STAGECOACH_PUSH / stagecoach.push / [generation].push, default false). When true, a plain `git push`
	// (no args, streaming) runs AFTER a fully-clean run. Push failure does NOT roll back commits (FR-P2):
	// git's stderr is streamed verbatim, "commits created; push failed" prints, exit 1. Skipped on
	// --dry-run, the exit-2 path, and any rescue/CAS abort (FR-P3). See cmd.runPush + git.Git.Push.
	Push bool `toml:"push"`
// ...in Defaults(): Push: false,

// === internal/config/load.go (EDIT — loadEnv + loadFlags) ===
// in loadEnv, after the STAGECOACH_NO_COLOR block:
	if v, ok := os.LookupEnv("STAGECOACH_PUSH"); ok && v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("STAGECOACH_PUSH: %w", err)
		}
		cfg.Push = b // DIRECT set — can be false (escape hatch)
	}
// in loadFlags, near the --verbose/--no-color block:
	if fs.Changed("push") {
		if v, err := fs.GetBool("push"); err == nil {
			cfg.Push = v // DIRECT set
		}
	}

// === internal/config/git.go (EDIT — loadGitConfig, bool block near stagecoach.verbose) ===
	if v, found, err := gitConfigBool(repoDir, "stagecoach.push"); err != nil {
		return nil, err
	} else if found {
		c.Push = v
	}

// === internal/cmd/root.go (EDIT — flagPush + BoolVar, near --context) ===
	var flagPush bool   // (add to the flagFormat/flagContext var block)
	// after the --context StringVar:
	pf.BoolVar(&flagPush, "push", false,
		"Run plain `git push` (streaming) after a fully-successful run. Never prompts; never auto-sets "+
			"upstream. On push failure the commits stand — git's stderr is shown verbatim (including the "+
			"no-upstream hint), \"commits created; push failed\" prints, and stagecoach exits 1. Skipped "+
			"on --dry-run, the nothing-to-commit exit, and any rescue/CAS abort. (env STAGECOACH_PUSH, "+
			"git stagecoach.push, config [generation].push; default false.) (§9.22 FR-P1)")

// === internal/cmd/default_action.go (EDIT — runPush helper + BOTH success-return invocations) ===
// runPush runs `git push` (plain, streaming) after a fully-clean run, iff cfg.Push (§9.22 FR-P1). No-op
// when disabled (default — byte-identical to the pre-feature path). On push failure the COMMITS STAND
// (FR-P2): git's stderr was already streamed verbatim by Push; print the closing note + return a wrapped
// error (caller maps to exit 1 via exitcode.For's default tail). Never prompts; never auto-sets upstream.
func runPush(ctx context.Context, stderr io.Writer, g git.Git, cfg config.Config) error {
	if !cfg.Push {
		return nil // THE no-op guard — the byte-identity regression invariant
	}
	if err := g.Push(ctx, os.Stdout, stderr); err != nil { // stream git's stdout/stderr verbatim
		fmt.Fprintln(stderr, "commits created; push failed") // FR-P2 closing note (stderr; BEFORE the err)
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}
// SINGLE path — replace `printCommitReport(stdout, res, changes); return nil`:
	printCommitReport(stdout, res, changes)
	if err := runPush(ctx, stderr, g, *cfg); err != nil {
		return exitcode.New(exitcode.Error, err) // exit 1; commits already stand (FR-P2)
	}
	return nil
// DECOMPOSE path (runDecompose) — replace `if derr != nil { ... }; return nil`:
	if derr != nil {
		return handleDecomposeError(derr)
	}
	if err := runPush(ctx, stderr, g, *cfg); err != nil {
		return exitcode.New(exitcode.Error, err)
	}
	return nil
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD git.Git.Push (interface + *gitRunner impl + ALL test doubles)
  - EDIT internal/git/git.go: add Push(ctx, stdout, stderr io.Writer) error to the Git interface
    (near HooksPath's doc ~L58) + implement on *gitRunner (the net-new streaming method — wires
    cmd.Stdout/cmd.Stderr to the passed writers; `git -C <repo> push` with NO args; NEVER --set-upstream;
    wrapped error on non-zero exit; ctx-aware via ctx.Err()).
  - RUN `go build ./...` immediately to enumerate EVERY fake/stub git.Git in the repo; add no-op/stub
    Push impls (`func (...) Push(_ context.Context, _, _ io.Writer) error { return nil }`) to each
    (search: `git.Git` in *_test.go — stubtest, internal/git fakes, decompose/stagecoach test doubles).

Task 2: ADD Config.Push + loadEnv + loadFlags + loadGitConfig + root.go --push flag
  - EDIT internal/config/config.go: `Push bool \`toml:"push"\`` near Template (L~93) + Defaults() Push: false.
  - EDIT internal/config/load.go: loadEnv STAGECOACH_PUSH block (presence-semantic, ParseBool, DIRECT set)
    + loadFlags --push block (fs.Changed("push") → DIRECT set).
  - EDIT internal/config/git.go: loadGitConfig bool block — `stagecoach.push` via gitConfigBool.
  - EDIT internal/cmd/root.go: flagPush var + pf.BoolVar(&flagPush, "push", false, "...").

Task 3: ADD runPush helper + the TWO success-return invocations
  - EDIT internal/cmd/default_action.go: add runPush(ctx, stderr, g, cfg) error near handleGenError.
  - SINGLE path: insert `if err := runPush(ctx, stderr, g, *cfg); err != nil { return exitcode.New(exitcode.Error, err) }`
    BETWEEN printCommitReport and `return nil`.
  - DECOMPOSE path (runDecompose): insert the SAME runPush call on the derr==nil branch, before `return nil`.
  - NOTE: runPush is the cfg.Push-gated no-op guard (byte-identity when push is off).

Task 4: TESTS — Git.Push (clean + no-upstream) + config precedence + CLI skip/FR-P2
  - internal/git/git_test.go: TestPush_CleanToBareRemote (temp bare remote, `git push -u origin HEAD`
    SETUP, new commit, g.Push → nil; assert remote advanced). TestPush_NoUpstreamFails128
    (t.Setenv GIT_CONFIG_GLOBAL+SYSTEM=/dev/null, NO upstream setup, g.Push → err; assert stderr contains
    `has no upstream branch` + `--set-upstream`; HEAD unchanged by the push failure).
  - internal/config (load_test.go / git_test.go / file_test.go): push precedence across --push /
    STAGECOACH_PUSH / stagecoach.push / [generation].push; default false; DIRECT-set escape hatch
    (--push=false / STAGECOACH_PUSH=0).
  - internal/cmd/default_action_test.go: skip-on-dry-run (--dry-run --push → no push, exit 0; assert via
    spy Git or unchanged remote); FR-P2 exit-1 + "commits created; push failed" note (no-upstream repo +
    --push → exit 1, note on stderr, commit landed); byte-identity (no --push → Push never called).

Task 5: EDIT docs/cli.md + docs/configuration.md
  - docs/cli.md: --push global-flags row (the Documentation & References critical block — no-auto-
    --set-upstream stance, failure semantics, skip conditions).
  - docs/configuration.md: `push` key in the [generation] list + the env/git-config tables (all four
    sources, default false, failure semantics, cross-reference docs/cli.md).
```

### Implementation Patterns & Key Details

```go
// The net-new streaming Push method (the ONLY io.Writer-taking method on Git):
func (g *gitRunner) Push(ctx context.Context, stdout, stderr io.Writer) error {
	gitPath, lerr := exec.LookPath("git")
	if lerr != nil {
		return fmt.Errorf("git binary not found in PATH: %w", lerr)
	}
	cmd := exec.CommandContext(ctx, gitPath, "-C", g.workDir, "push") // plain push, NO args, NEVER -u
	cmd.Stdout = stdout                                               // VERBATIM (not bytes.Buffer)
	cmd.Stderr = stderr                                               // VERBATIM
	runErr := cmd.Run()
	if runErr == nil {
		return nil
	}
	if cerr := ctx.Err(); cerr != nil { return cerr }
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return fmt.Errorf("git push failed (exit %d)", exitErr.ExitCode())
	}
	return fmt.Errorf("git push: %w", runErr)
}

// runPush — the cfg.Push-gated no-op guard + the closing-note-before-error ordering (FR-P2):
func runPush(ctx context.Context, stderr io.Writer, g git.Git, cfg config.Config) error {
	if !cfg.Push {
		return nil // byte-identity when push is off (the regression invariant)
	}
	if err := g.Push(ctx, os.Stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "commits created; push failed") // BEFORE the err (FR-P2)
		return fmt.Errorf("git push: %w", err)               // → exitcode.For default tail → exit 1
	}
	return nil
}

// The no-upstream test contract (external_deps.md §8 — CRITICAL):
t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")  // masks autoSetupRemote — WITHOUT this the test FALSELY PASSES
t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
// ... assert stderr contains "has no upstream branch" + "--set-upstream" (substrings, NOT full text)
```

### Integration Points

```yaml
GIT INTERFACE (1 new method — net-new streaming):
  - Push: `git -C <repo> push` (NO args; NEVER --set-upstream); cmd.Stdout/cmd.Stderr wired to passed
    io.Writers (the CLI passes os.Stdout/os.Stderr → verbatim streaming). ALL existing fake/stub git.Git
    impls MUST gain a no-op Push (go build ./... enumerates them).

CLI (the push site — BOTH success returns):
  - internal/cmd/default_action.go: runPush helper + invocation at the SINGLE path (post-printCommitReport)
    and the DECOMPOSE path (post-commit-print-loop, derr==nil branch). Gated by cfg.Push.
  - FR-P3 skip conditions are STRUCTURAL (dry-run/exit-2/rescue-CAS all return before the success site).

CONFIG (full 5-layer precedence — mirrors Template, NOT Context):
  - Config.Push `toml:"push"` ([generation] key) + Defaults() Push: false.
  - loadEnv: STAGECOACH_PUSH (bool, presence-semantic, DIRECT set).
  - loadGitConfig: stagecoach.push (gitConfigBool; lowercase single-word key).
  - loadFlags: --push (fs.Changed → DIRECT set). root.go: pf.BoolVar.

EXIT MAPPING:
  - NO new sentinel/mapping. Push failure → plain fmt.Errorf → exitcode.For default tail → exit 1.
  - The "commits created; push failed" note is printed by runPush to stderr BEFORE returning the error.

DOCS (Mode A):
  - docs/cli.md (--push row), docs/configuration.md (push key).

OUT OF SCOPE:
  - The generate/decompose orchestrator (push is a CLI-layer post-run convenience).
  - The prompt package, hook exec (push is a silent no-op there — no hookexec.go edit), P1.M5.T1.S1 --edit.
  - Any new exitcode constant or For() branch (push failure is generic exit 1).
  - Auto-setting upstream (FR-P2 explicitly forbids it).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/git/git.go internal/config/config.go internal/config/load.go \
        internal/config/git.go internal/cmd/root.go internal/cmd/default_action.go
go build ./...   # the interface edit must compile across ALL fake/stub git.Git impls
go vet ./...
golangci-lint run
# Expected: zero errors. The FIRST `go build ./...` after the Git interface edit surfaces every test
# double needing the new Push method — fix them all before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/git/... -v        # Push clean-to-bare-remote + no-upstream-fails-128 (GIT_CONFIG_GLOBAL=/dev/null)
go test ./internal/config/... -v     # push precedence across flag/env/git-config/file; default false
go test ./internal/cmd/... -v        # runPush no-op + skip-on-dry-run + FR-P2 exit-1 + note + byte-identity
# Expected: all pass. The byte-identity guard: every pre-existing test (cfg.Push never set) passes UNCHANGED.
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach
# Flag exists + help:
/tmp/stagecoach --help 2>&1 | grep -A2 -- '--push'
# Clean push to a temp bare remote (manual smoke):
rm -rf /tmp/pushe2e && mkdir -p /tmp/pushe2e && cd /tmp/pushe2e
git init && git config user.email t@t && git config user.name t
git remote add origin /tmp/pushe2e-bare.git && git init --bare /tmp/pushe2e-bare.git
echo a > a.txt && git add a.txt && git commit -m init && git push -u origin HEAD  # upstream SETUP
echo b > b.txt && /tmp/stagecoach --push   # → commit + `git push` streams → exit 0
# No-upstream failure (manual smoke — isolate the dev's autoSetupRemote):
GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null /tmp/stagecoach --push
#   → "commits created; push failed" + git's no-upstream stderr → exit 1; commits stand
# Skip-on-dry-run:
/tmp/stagecoach --dry-run --push   # → message + "(no commit created)"; NO push; exit 0
```

### Level 4: Regression & Cross-cutting

```bash
go test -race ./...     # full suite
golangci-lint run ./...
# Byte-identity guard: cfg.Push==false (default) → every commit path byte-identical to today — all
# pre-existing generate/decompose/cmd tests MUST pass UNCHANGED (they never set cfg.Push). The push site
# is unreachable when cfg.Push is false (runPush short-circuits on line 1).
# FR-P3 guard: the three skip cases (dry-run, exit-2, rescue/CAS) return BEFORE the push site — verify
# via the default_action_test.go assertions (no spy Push call / unchanged remote on each skip path).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` compiles (all fake/stub git.Git impls updated for the new Push method).
- [ ] `go test ./...` green; pre-existing tests pass UNCHANGED (cfg.Push never set → byte-identity).
- [ ] `go vet ./...` + `golangci-lint run` clean; `gofmt` no diff.

### Feature Validation
- [ ] `--push` unset → every commit path byte-identical to today (runPush no-op).
- [ ] `--push` (single, with upstream) → `git push` runs, output streams verbatim, exit 0.
- [ ] `--push` (decompose) → push runs ONCE after ALL commits + arbiter resolution land.
- [ ] No-upstream push → git's stderr streamed verbatim (incl. `has no upstream branch` + `--set-upstream`),
      "commits created; push failed" on stderr, exit 1; commits STAND (HEAD unchanged by the push failure).
- [ ] `--dry-run --push` → NO push (dry-run early-returns before the site), exit 0.
- [ ] Exit-2 (nothing to commit) + `--push` → NO push (exit-2 returns before the site), exit 2.
- [ ] Rescue/CAS abort + `--push` → NO push (error returns before the site), exit 3/1/124.
- [ ] `STAGECOACH_PUSH=1` / `stagecoach.push=true` / `[generation].push = true` each enable push (precedence).
- [ ] stagecoach NEVER auto-sets upstream (FR-P2) — `git push` runs with NO args after `push`.

### Code Quality Validation
- [ ] Push is the net-new streaming method (cmd.Stdout/cmd.Stderr wired to io.Writers, NOT bytes.Buffer).
- [ ] runPush prints the closing note BEFORE returning the error (FR-P2).
- [ ] Push failure → exit 1 via exitcode.For's DEFAULT TAIL (no new sentinel/mapping).
- [ ] cfg.Push is full-precedence (toml:"push" + env + git-config + flag; mirrors Template, NOT Context).
- [ ] The git-config key is `stagecoach.push` (lowercase single word, not camelCase).
- [ ] FR-P3 skip conditions hold STRUCTURALLY (push site is at the success return).
- [ ] hook exec silently ignores --push (no hookexec.go edit; cfg.Push only checked at the default-action site).

### Documentation & Deployment
- [ ] docs/cli.md `--push` row (no-auto-`--set-upstream` stance, failure semantics, skip conditions).
- [ ] docs/configuration.md `push` key (all four sources, default false, failure semantics).
- [ ] The no-upstream test contract (GIT_CONFIG_GLOBAL=/dev/null + substring asserts) is documented in tests.

---

## Anti-Patterns to Avoid

- ❌ Don't reuse `run()`/`runWithInput()` for push — they CAPTURE stdout/stderr into bytes.Buffer; push
  must STREAM verbatim (FR-P1). Push is a net-new method wiring cmd.Stdout/cmd.Stderr to io.Writers.
- ❌ Don't add `--set-upstream` / `-u` / any arg to `git push` — FR-P2 explicitly forbids auto-setting
  upstream; stagecoach surfaces git's own hint verbatim instead.
- ❌ Don't write the no-upstream test WITHOUT `GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null` —
  the dev's `push.autoSetupRemote=true` makes the push silently SUCCEED → a FALSE GREEN (external_deps.md §8).
- ❌ Don't assert on full stderr text in the no-upstream test — git wording varies; assert on the stable
  substrings `has no upstream branch` + `--set-upstream`.
- ❌ Don't add a new exitcode constant or For() branch for push failure — it's generic exit 1 via the
  default tail. The COMMITS STAND (push moves the REMOTE, not local HEAD).
- ❌ Don't return the push error BEFORE printing "commits created; push failed" — the note is the
  user-facing contract (FR-P2); print it to stderr first, THEN return the wrapped error.
- ❌ Don't give `push` flag-only precedence (toml:"-") — FR-P1 names all four sources (flag/env/git-config/
  file); mirror Template, NOT Context.
- ❌ Don't write the git-config key as `stagecoach.Push` (camelCase) — `push` is a single word; the key is
  literally `stagecoach.push`.
- ❌ Don't add explicit skip guards for dry-run/exit-2/rescue-CAS inside runPush beyond `cfg.Push` —
  FR-P3's skip conditions hold STRUCTURALLY (those paths return before the success site). A `flagDryRun`
  short-circuit is harmless belt-and-suspenders but NOT required.
- ❌ Don't reject `--push` on hook exec (unlike `--edit`, FR-E4) — push is a silent no-op there (cfg.Push
  is only checked at the default-action success return, which hook exec never reaches). No hookexec.go edit.
- ❌ Don't push on the decompose partial-landing FR-M12 path — runPush is ONLY on the derr==nil branch.
- ❌ Don't forget the fake/stub git.Git impls — adding Push to the interface ripples to EVERY test double;
  `go build ./...` after the interface edit enumerates them (stubs: `Push(_, _, _ io.Writer) error { return nil }`).

---

## Confidence Score

**9/10** for one-pass implementation success. The design is tightly scoped and the contracts are pinned:
the net-new streaming `Push` method is a small, self-contained exec with explicit `cmd.Stdout`/`cmd.Stderr`
wiring (the only novelty vs the capturing `run()`), the two success-return insertion points are explicit
with surrounding anchor calls, the FR-P3 skip conditions are PROVEN structurally unreachable (placing push
at the success return makes them hold by construction — no guard logic needed), the exit-1 mapping reuses
`exitcode.For`'s default tail (no new sentinel), the full-precedence config plumbing mirrors the already-
shipped `Template` knob exactly, and the `cfg.Push==false` no-op is the byte-identity regression guard.
The −1 is residual risk in the no-upstream test harness: the `GIT_CONFIG_GLOBAL=/dev/null` isolation is
non-obvious and a careless implementer could ship a false-green test (mitigated by the explicit anti-
pattern + the external_deps.md §8 citation + the test-pattern block). The feature is transparent when off
and the implementation touches no orchestrator/prompt/hook code, so blast radius is minimal.
