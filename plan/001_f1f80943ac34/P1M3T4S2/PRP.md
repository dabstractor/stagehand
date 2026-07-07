---
name: "P1.M3.T4.S2 — CommitStaged orchestrator: full pipeline (snapshot → generate → parse → dedupe → commit) — PRD §13 / §9 / §18"
description: |

  Implement `generate.CommitStaged` — the synchronous, atomic, snapshot-based commit orchestrator that
  is Stagecoach's core IP (PRD §13). It wires together every upstream layer built so far — git plumbing
  (P1.M1.T2/T3), the provider pipeline (P1.M2.T1–T6), prompt construction (P1.M3.T1), dedupe
  (P1.M3.T2), and rescue formatting (P1.M3.T3) — into ONE function that NEVER calls `git add` (PRD
  §11.3), builds the commit from a FROZEN `TREE_SHA` via plumbing (`write-tree` → `commit-tree` →
  `update-ref` CAS), and is the ONLY code that advances HEAD (PRD §18.1). It is the keystone of the
  generation pipeline; the CLI default action (P1.M4.T1.S2) is `maybeAutoStage(); commitStaged()`, and
  v2's multi-commit is `for each partition { reset+stage; commitStaged() }` — so the §11.3 staging/
  commit decoupling this function enforces is what makes v2 trivial (design-decisions §0).

  Two deliverables, both NEW files, NO edits to existing code:
    1. **CREATE `internal/generate/generate.go`** (`package generate`) — the types + the orchestrator:
       `type Deps struct{ Git git.Git; Manifest provider.Manifest }`,
       `type Result struct{ CommitSHA, Subject, Message, Provider, Model string; Changes []git.FileChange }`,
       the typed errors (`ErrNothingToCommit`, `ErrTimeout`, `ErrRescue` sentinels; `ErrCASFailed`
       re-export of `git.ErrCASFailed`; `*RescueError` and `*CASError` context wrappers), and
       `func CommitStaged(ctx context.Context, deps Deps, cfg config.Config) (Result, error)`.
       Pipeline (PRD §13.3, §9): RevParseHEAD → StagedDiff(empty→ErrNothingToCommit) → WriteTree
       (fail→abort) → build system prompt (mature vs fallback on CommitCount) → GENERATION+DEDUPE
       LOOP (BuildUserPayload→Render→Execute→ParseOutput→ExtractSubject→IsDuplicate, with FR29
       retry_instruction on parse-fail and FR32 rejection-list on dup, bounded by MaxDuplicateRetries)
       → CommitTree → UpdateRefCAS (CAS-fail→CASError, never force) → DiffTree → Result.
    2. **CREATE `internal/generate/generate_test.go`** (`package generate`) — integration tests
       driving `CommitStaged` end-to-end with the **real** `provider.Execute` seam but a **stub** agent
       (`internal/stubtest`, P1.M3.T4.S1) against **real temp git repos**. Covers the five contract
       scenarios (success, dedupe-retry-then-success, parse-fail-rescue, CAS-failure via HEAD-moved-
       mid-run, root commit) + the idempotent-index / atomic-HEAD invariants (PRD §20.2).

  SCOPE BOUNDARY (load-bearing): this subtask is the ORCHESTRATOR + its integration tests only. It does
  NOT implement the CLI (P1.M4.T1), signal handling (P1.M4.T2 — the orchestrator is signal-AGNOSTIC;
  it observes a cancelled `ctx` but installs no handler), the public API wrapper (P1.M3.T5 — it will
  re-export `Result`/`Deps`/errors + add `Options`), dry-run (P1.M4.T4 — the CLI calls `CommitStaged`
  with a flag that... NO — dry-run is the CLI's job; `CommitStaged` always commits), property tests
  (P1.M5.T1 — they import `stubtest` too), or ANY staging logic (§11.3). It adds NO dependency.

  INPUT (upstream — already built, read-only): git plumbing `git.Git` (internal/git/git.go, P1.M1.T2/
  T3 — RevParseHEAD/WriteTree/CommitTree/UpdateRefCAS/DiffTree/StagedDiff/RecentMessages/RecentSubjects/
  CommitCount); the provider pipeline `provider.{Manifest.Render, Execute, ParseOutput}` (P1.M2.T4/T5/
  T6) + `git.ErrCASFailed`; prompt builders `prompt.{BuildSystemPrompt, BuildFallbackPrompt,
  BuildUserPayload, DetectMultiline}` (P1.M3.T1); dedupe `generate.{ExtractSubject, IsDuplicate}`
  (P1.M3.T2); rescue `generate.FormatRescue` (P1.M3.T3); config `config.Config` (P1.M1.T4); the stub
  provider `internal/stubtest` (P1.M3.T4.S1 — FROZEN `Build`/`Options`/`Manifest`/`NewScript`/`Env`).

  OUTPUT (downstream consumers): the CLI default action (P1.M4.T1.S2) calls
  `CommitStaged(ctx, Deps{Git: git.New(repo), Manifest: resolvedManifest}, cfg)` and maps the Result /
  typed errors to the success report (FR42) + exit codes (§15.4). The signal handler (P1.M4.T2) cancels
  `ctx` to abort an in-flight generation; the orchestrator's `context.Canceled` → `ErrRescue` path is
  the seam. The public API (P1.M3.T5) wraps `CommitStaged` with `Options` and re-exports `Result`/
  errors. The `CommitStaged`/`Deps`/`Result`/error API is the FROZEN surface these consume.

  ⚠️ **NEVER call `git add` (or `AddAll`/`StagedFileCount`).** PRD §11.3: "The core is
  `commitStaged(ctx, cfg)` that assumes the index is already in the desired state." Auto-stage-all is
  the CLI layer's job (P1.M4.T1.S2). Entangling staging with commit logic BREAKS v2's
  `for-partition{reset+stage; commitStaged()}` composition. `CommitStaged` reads the index (StagedDiff,
  WriteTree) but never mutates it. (design-decisions §0/§6)

  ⚠️ **The commit is built from the FROZEN `TREE_SHA`, not the live index (PRD §13.2/§13.3).**
  `WriteTree` (step 3) snapshots the index into an immutable tree object; `CommitTree` (step 7) builds
  the commit from THAT tree; `UpdateRefCAS` (step 8) is the ONLY ref mutation. Files the user stages
  AFTER `WriteTree` are NOT in the commit and remain staged. Any failure before/including step 8 leaves
  the repo byte-for-byte unchanged (PRD §18.1 — the atomic invariant). (design-decisions §7)

  ⚠️ **CAS failure → re-read HEAD, NEVER force-update (FR41).** `UpdateRefCAS` returns wrapped
  `git.ErrCASFailed` when HEAD moved since the snapshot. The orchestrator re-reads HEAD via
  `RevParseHEAD` to get the actual SHA (decision D5 in the git docstring), returns `*CASError`, and does
  NOT retry/force. (design-decisions §8)

  ⚠️ **Unified generation loop (FR29 + FR32 share one bounded counter).** `for attempt := 0; attempt <=
  cfg.MaxDuplicateRetries; attempt++` = maxRetries+1 attempts. Parse-failure (FR29) and duplicate
  (FR32) BOTH consume an attempt; the corrective signal differs (retry_instruction prepend vs
  rejection-list growth). Timeout (DeadlineExceeded) → immediate rescue (ErrTimeout), NO retry —
  retrying a killed agent would just time out again. (design-decisions §4/§5)

  ⚠️ **Generate-package tests need their OWN git fixtures.** `internal/git/*_test.go`'s helpers
  (`initRepo`, `makeEmptyCommit`, `writeFile`, `stageFile`, `headSHA`) are package-private AND in
  `_test.go` files → NOT importable. Copy the ~10-line `exec.Command("git","-C",dir,...)` wrappers into
  `generate_test.go`. (design-decisions §10)

  Deliverable: CREATE `internal/generate/generate.go` + `internal/generate/generate_test.go`. Imports
  only already-present internal packages + stdlib (`context`/`errors`/`fmt`). `go mod tidy` MUST be a
  no-op. Touches ONLY these two NEW files — NO go.mod/go.sum change, NO edit to any other file.

---

## Goal

**Feature Goal**: Implement the synchronous, atomic, snapshot-based commit orchestrator
(`generate.CommitStaged`) that fuses every upstream layer into PRD §13's core flow — capture the
parent + frozen-tree snapshot BEFORE generation, run a bounded generate→parse→dedupe retry loop, build
the commit object from the frozen tree via plumbing, and advance HEAD via a single compare-and-swap
`update-ref` (never `git commit`, never `git add`). The function returns a typed `Result` on success or
a typed error (`ErrNothingToCommit` / `ErrRescue` / `ErrTimeout` / `ErrCASFailed`) that the CLI maps to
an exit code + recovery message. It is the keystone of the v1 pipeline and the compositional unit v2
reuses verbatim.

**Deliverable** (two NEW files, nothing else touched):
1. **`internal/generate/generate.go`** — `package generate`. Types: `Deps`, `Result`, the error
   sentinels (`ErrNothingToCommit`/`ErrTimeout`/`ErrRescue`) + `ErrCASFailed` re-export, the context
   wrappers `RescueError`/`CASError` (with `Error()`+`Unwrap()`). Function:
   `func CommitStaged(ctx context.Context, deps Deps, cfg config.Config) (Result, error)` implementing
   the 10-step pipeline (PRD §13.3 / §9). Package doc comment updated to describe the orchestrator.
2. **`internal/generate/generate_test.go`** — `package generate`. Integration tests driving
   `CommitStaged` through the real `provider.Execute` seam with a stub agent (`internal/stubtest`)
   against real temp git repos. Five scenarios (success, dedupe-retry-then-success, parse-fail-rescue,
   CAS-failure, root commit) + the idempotent-index / atomic-HEAD invariants (PRD §20.2). Own minimal
   git fixture helpers (initRepo/writeFile/stageFile/headSHA/commitRaw/gitOut).

**Success Definition**: `go build ./...` succeeds; `go test -race ./internal/generate/` is green (new
orchestrator tests pass, existing dedupe/rescue tests still pass); `go test -race ./...` shows NO
regression; `go vet ./internal/generate/` clean; `gofmt -l internal/generate/` empty; `golangci-lint
run` (if available) clean; go.mod/go.sum byte-unchanged; the five contract scenarios + two invariants
all pass; every other file byte-unchanged.

## User Persona

**Target User**: The CLI implementer (P1.M4.T1.S2) and the public-API author (P1.M3.T5). They need a
single, well-typed entry point that does the whole commit atomically and tells them, via the error
type, exactly what to print and which exit code to use. Transitively: the end-user personas (PRD §7
"plan-holder"/"refusenik"/"tinkerer") get their `[<sha>] <subject>` + "what landed" report, safe
stage-while-generating overlap, and never a clobbered/aborted index — all of which live in this function.

**Use Case**: The CLI's default action does `maybeAutoStage(); CommitStaged(ctx, Deps{git.New(repo),
resolvedManifest}, cfg)`. On `Result` it prints `Result.Subject` + the `Changes` listing (FR42). On
`ErrNothingToCommit` it exits 2; on `*RescueError` it prints `FormatRescue(...)` + exits 3 (or 124 if
`ErrTimeout`); on `*CASError` it prints the §13.5 message + exits 1. The orchestrator does the rest.

**User Journey**: (internal, no end-user surface) CLI resolves config + manifest → calls CommitStaged →
CommitStaged snapshots, generates (calling the real agent), dedupes, commits via plumbing, returns →
CLI renders the report/exit. During the blocking generation the user `git add`s the next batch in
another pane (safe — the commit is built from the frozen tree).

**Pain Points Addressed**: (1) `git commit` couples what/when/whether-fail (§13.1) — solved by plumbing
+ frozen snapshot + CAS. (2) Stage-while-generating overlap — solved by never touching the index
between `write-tree` and `update-ref`. (3) Concurrent-commit races — solved by CAS (fails cleanly
instead of clobbering). (4) Indistinct failure reasons for the CLI — solved by typed errors carrying
recovery context. (5) v2 composability — solved by the §11.3 staging/commit decoupling.

## Why

- **It IS the core IP (PRD §13).** Everything else is scaffolding around this one function. The
  snapshot-then-CAS flow is "the thing Stagecoach does that no incumbent does" (§13).
- **Unblocks the CLI + public API + property tests.** P1.M4.T1.S2, P1.M3.T5, and P1.M5.T1 all wait on
  `CommitStaged`. It is the last vertical slice before the UX layer.
- **Faithful to commit-pi's proven model.** commit-pi (zsh, Appendix C) did write-tree→commit-tree→
  update-ref-CAS with a dedupe loop and a rescue trap. The Go port makes it typed, testable, atomic-by-
  construction, and agent-agnostic (via the injected manifest).
- **No new dependency, no new user-facing surface** (PRD "DOCS: none — internal orchestrator"). Imports
  only already-present internal packages + stdlib.

## What

A new orchestrator function in `internal/generate` that runs the full §13 pipeline synchronously and a
test suite that proves it against real git via a stub agent. The orchestrator is pure orchestration: it
calls git methods, the provider pipeline, the prompt builders, and the dedupe/rescue primitives; it owns
no new I/O of its own (no shelling out except via the injected `git.Git` and `provider.Execute`). It
returns a typed `Result` or a typed error; it never prints, never exits, never installs a signal handler
(those are the CLI's job). It never stages.

### Success Criteria

- [ ] `internal/generate/generate.go` exists, `package generate`, imports `context`/`errors`/`fmt` +
      `github.com/dustin/stagecoach/internal/{config,git,prompt,provider}` ONLY (NO new third-party).
      Exports `Deps`, `Result`, `ErrNothingToCommit`, `ErrTimeout`, `ErrRescue`, `ErrCASFailed`,
      `RescueError`, `CASError`, `CommitStaged`. Has an updated `// Package generate …` doc.
- [ ] `CommitStaged(ctx, deps, cfg)` runs the 10-step pipeline (design-decisions §7) and: returns
      `Result{...}` on success; `ErrNothingToCommit` when `StagedDiff==""`; `*RescueError{Kind:ErrTimeout}`
      on `context.DeadlineExceeded` from Execute (immediate, no retry); `*RescueError{Kind:ErrRescue}`
      when the loop exhausts (parse-fail/dup after all attempts); propagates `WriteTree`/`CommitTree`/
      `StagedDiff` infra errors; `*CASError` on CAS failure (with HEAD re-read for `Actual`).
- [ ] The orchestrator NEVER calls `git.AddAll`/`StagedFileCount`/any staging op (grep-verified:
      `grep -n 'AddAll\|StagedFileCount\|"add"' internal/generate/generate.go` → empty).
- [ ] On `isUnborn`: `parents=nil` for CommitTree (root commit), `expectedOld=all-zeros` for CAS,
      `recent=nil`, fallback prompt, `DiffTree(...,isRoot=true)`.
- [ ] The generation loop is `for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++`; parse-
      failure retries prepend `RetryInstruction`; duplicate retries grow `rejected`; success `break`s.
- [ ] `internal/generate/generate_test.go` exists, `package generate`, drives `CommitStaged` via the
      stub (`stubtest.Build`/`Manifest`/`NewScript`) against temp git repos, and passes: success;
      dedupe-retry-then-success; parse-fail-rescue; CAS-failure (HEAD moved mid-run); root commit;
      idempotent-index-on-failure; atomic-HEAD-on-CAS-failure.
- [ ] `go build ./...` succeeds; `go test -race ./...` green; `go vet ./internal/generate/` clean;
      `gofmt -l internal/generate/` empty; go.mod/go.sum byte-unchanged; every other file byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact upstream
signatures (the Git interface, `Manifest.Render`/`Execute`/`ParseOutput`, the prompt builders, the
dedupe/rescue functions — all quoted in the references), the design decisions (the 10 load-bearing calls
in research/design-decisions.md), the PRD §13/§9/§18 excerpts in `selected_prd_content`, the test
convention to mirror (`internal/provider/executor_test.go` + `internal/git/*_test.go` fixtures), and the
copy-ready Go skeletons in the Implementation Blueprint. No CLI/signal/public-API knowledge required —
the orchestrator is signal-agnostic and CLI-agnostic.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M3T4S2/research/design-decisions.md
  why: the SINGLE most important read — the 10 decisions specific to this subtask: scope = ORCHESTRATOR
       only + §11.3 never-stage (§0); why Deps{Git,Manifest} DI (§1); Result fields incl. Changes (§2);
       the typed-error design (sentinels + RescueError/CASError wrappers, errors.Is contract) (§3); the
       unified loop FR29+FR32 (§4); Execute-error branching (timeout→rescue-no-retry, exit→fall-through)
       (§5); nothing-to-commit via StagedDiff emptiness (§6); the exact 10-step pipeline (§7); CAS re-read
       + never-force (§8); root-commit handling (§9); test strategy + own git fixtures (§10).
  critical: §3 (error types — the CLI's exit-code mapping depends on errors.Is/As working), §4 (the
       loop bound + the two corrective signals), §5 (timeout must NOT retry), §7 (step ordering = the
       atomic invariant), §8 (CAS never forces). §10's fixture note (git's _test.go helpers are NOT
       importable) is the thing most likely to cause a compile error.

- file: internal/git/git.go   (P1.M1.T2/T3 — READ for the Git interface contract; do NOT edit)
  section: `type Git interface { … }` + `type FileChange struct{…}` + `type StagedDiffOptions struct{…}`
       + `var ErrCASFailed = errors.New(…)` + `func New(workDir string) Git`.
  why: THIS is the contract for every git call. The orchestrator calls (in order): RevParseHEAD → (sha,
       isUnborn, err); StagedDiff(ctx, StagedDiffOptions{MaxDiffBytes:cfg.MaxDiffBytes, MaxMdLines:
       cfg.MaxMdLines}) → (diff, err); WriteTree → (tree, err); CommitCount → (n, err) [if !isUnborn];
       RecentMessages(ctx, 20) → ([]string, err) [if n>1]; RecentSubjects(ctx, 50) → ([]string, err)
       [if !isUnborn]; CommitTree(ctx, tree, parents, msg) → (newSHA, err); UpdateRefCAS(ctx, "HEAD",
       newSHA, expectedOld) → err; DiffTree(ctx, newSHA, isUnborn) → ([]FileChange, err).
  pattern: every method takes ctx first; non-zero git exits are returned as (stdout, stderr, code, nil)
       INTERNALLY but the public methods translate to Go errors (WriteTree→"unresolved merge conflicts",
       UpdateRefCAS→wrapped ErrCASFailed, DiffTree/CommitTree/StagedDiff→wrapped exit errors). The
       orchestrator treats ANY non-nil err from these as a hard failure (return it; do not retry git).
  gotcha: UpdateRefCAS returns a WRAPPED `git.ErrCASFailed` (errors.Is works) carrying exit code+stderr
          but NOT the new HEAD — the orchestrator re-reads HEAD via RevParseHEAD to get `Actual` (§8).
          `isUnborn` comes ONLY from RevParseHEAD (exit 128 signal, NOT string emptiness). On a root
          commit pass `expectedOld` = 40 × '0' (matches TestUpdateRefCAS_RootCommit).

- file: internal/provider/render.go   (P1.M2.T4 — READ for the Render seam; do NOT edit)
  section: `func (m Manifest) Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error)`
       + `type CmdSpec struct{…}`.
  why: the orchestrator renders the manifest each attempt: `spec, err := deps.Manifest.Render(cfg.Model,
       cfg.Provider, sysPrompt, userPayload)`. Render calls Validate()+Resolve() internally (nil-pointer-
       safe on a copy). The userPayload (4th arg) is the prompt.BuildUserPayload output (+optional
       retry_instruction prepend); sysPrompt (3rd arg) is the mature/fallback system prompt; cfg.Model/
       cfg.Provider (1st/2nd) default to the manifest's when "".
  pattern: Render never spawns; it returns a *CmdSpec for Execute. Pass the resolved cfg.Model/cfg.Provider
           ("" is fine — Render falls back to manifest defaults).
  gotcha: for stdin-delivery manifests (all built-ins + the stub), Render puts the payload in spec.Stdin;
          the diff is NEVER a CLI arg (PRD §9.3 FR15 — avoids arg-length limits). No extra handling needed.

- file: internal/provider/executor.go   (P1.M2.T5 — READ for the Execute seam + error contract; do NOT edit)
  section: `func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration) (stdout, stderr string, err error)`.
  why: the orchestrator runs the agent: `out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout)`.
       cfg.Timeout is a time.Duration (default 120s). The error contract (check FIRST in this order):
       timeout → err IS context.DeadlineExceeded; parent/signal cancel → context.Canceled; non-zero exit →
       wrapped *exec.ExitError (stdout STILL captured + returned); start miss → wrapped LookPath; success → nil.
  pattern: timeout → return &RescueError{Kind:ErrTimeout} IMMEDIATELY (no retry — §5). context.Canceled →
           &RescueError{Kind:ErrRescue}. non-zero exit → fall through to ParseOutput (stdout may be partial-
           valid); record execErr as the rescue Cause if the loop later exhausts.
  gotcha: Execute sets up a process group (Setpgid) so ctx-cancel kills the whole tree — the orchestrator
          does NOT need its own kill logic (that's Execute's job). The orchestrator's only kill lever is
          the ctx (cancelled by the timeout Execute derives, or by the CLI's signal handler in P1.M4.T2).

- file: internal/provider/parse.go   (P1.M2.T6 — READ for ParseOutput's ok= lever; do NOT edit)
  section: `func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)`.
  why: the orchestrator parses the agent stdout: `msg, ok, _ := provider.ParseOutput(out, deps.Manifest)`.
       ok==false (empty after trim+normalize) ⇒ the attempt is a parse failure → retry with
       retry_instruction (FR29), or rescue if the loop is exhausted. fellback is a logging signal (ignore
       for control flow; could surface in verbose mode later).
  gotcha: ParseOutput calls m.Resolve() internally (nil-pointer-safe). An empty stub output (OUT="" or a
          blank script line) ⇒ ok=false ⇒ the parse-fail retry path — this is how the stub triggers the
          parse-fail-rescue scenario.

- file: internal/provider/manifest.go   (P1.M2.T1 — READ for the Manifest fields the orchestrator reads; do NOT edit)
  section: the `Manifest` struct + `Resolve()` + the `RetryInstruction`/`DefaultModel`/`Name` fields.
  why: the orchestrator reads `deps.Manifest.Resolve().RetryInstruction` (the FR29 corrective preamble;
       resolved default "Output ONLY the commit message. No preamble, no markdown, no quotes.") and the
       resolved DefaultModel (for Result.Model when cfg.Model==""). Result.Provider = deps.Manifest.Name.
  gotcha: Manifest's pointer fields (RetryInstruction etc.) are *string — ALWAYS Resolve() before
          deref. `strPtr`/`boolPtr` are UNEXPORTED (the orchestrator doesn't construct manifests, so this
          doesn't bite here — only stubtest did, in S1).

- file: internal/prompt/system.go   (P1.M3.T1.S1/S2 — READ for the system-prompt builders; do NOT edit)
  section: `func BuildSystemPrompt(examples []string, hasMultiline bool, subjectTarget int) string` +
       `func BuildFallbackPrompt(subjectTarget int) string` + `func DetectMultiline(examples []string) bool`.
  why: the orchestrator builds the system prompt ONCE before the loop. If isUnborn OR CommitCount<=1 →
       `BuildFallbackPrompt(cfg.SubjectTargetChars)`. Else → `BuildSystemPrompt(recentMsgs,
       prompt.DetectMultiline(recentMsgs), cfg.SubjectTargetChars)` where recentMsgs=git.RecentMessages
       (ctx, 20). The system prompt does NOT change per attempt.
  gotcha: DetectMultiline(nil/empty)→false (safe). RecentMessages on unborn returns (nil,nil) — but the
          orchestrator only calls it when CommitCount>1, so it's never called on unborn.

- file: internal/prompt/payload.go   (P1.M3.T1.S3 — READ for the user-payload builder; do NOT edit)
  section: `func BuildUserPayload(diff string, rejected []string) string`.
  why: the orchestrator rebuilds the user payload EACH attempt: `payload := prompt.BuildUserPayload(diff,
       rejected)`. On a parse-failure retry, prepend the retry_instruction: `payload = retryInstruction +
       "\n\n" + payload`. `rejected` grows by one element per duplicate attempt (the matched subject).
  gotcha: BuildUserPayload(nil/empty rejected) → the §17.3 NORMAL path (colon instruction); non-empty →
          the REJECTION path (period instruction + IMPORTANT block + list). The diff is appended VERBATIM.

- file: internal/generate/dedupe.go   (P1.M3.T2 — READ; same package, do NOT edit)
  section: `func ExtractSubject(message string) string` + `func IsDuplicate(subject string, recent []string) bool`.
  why: SAME PACKAGE — call as `ExtractSubject(msg)` / `IsDuplicate(subject, recent)` (no package prefix).
       ExtractSubject = trimmed first line (FR30). IsDuplicate = exact, case-sensitive, whole-subject set
       membership (FR32). On isUnborn, recent=nil → IsDuplicate always false (vacuous; no dup check).
  gotcha: both are pure functions; ExtractSubject("")==""; IsDuplicate(s, nil)==false. No defensive trim
          needed (inputs are pre-trimmed by ParseOutput/RecentSubjects).

- file: internal/generate/rescue.go   (P1.M3.T3 — READ; same package, do NOT edit)
  section: `func FormatRescue(treeSHA, parentSHA, candidateMsg string) string`.
  why: SAME PACKAGE. The CLI calls `FormatRescue(err.TreeSHA, err.ParentSHA, err.Candidate)` from a
       `*RescueError` to render PRD §18.3. The orchestrator does NOT call FormatRescue itself (it returns
       the structured RescueError; rendering is the CLI's job, P1.M4.T1) — BUT the test may call it to
       assert the rescue path produces a well-formed message. `candidateMsg==""` → no candidate note.
  gotcha: FormatRescue returns NO trailing newline (the CLI's Fprintln adds it). parentSHA=="" → omits
          `-p` (root repo). The candidate note is appended only when candidateMsg != "".

- file: internal/config/config.go   (P1.M1.T4.S1 — READ for the Config fields; do NOT edit)
  section: `type Config struct { … }` + `func Defaults() Config`.
  why: `cfg` carries the resolved tuning: `cfg.Timeout` (time.Duration, per-attempt Execute timeout);
       `cfg.MaxDuplicateRetries` (loop bound, default 3); `cfg.SubjectTargetChars` (prompt target);
       `cfg.MaxDiffBytes`/`cfg.MaxMdLines` (StagedDiffOptions); `cfg.Model`/`cfg.Provider` (Render args,
       "" → manifest default); `cfg.Output`/`cfg.StripCodeFence` (manifest output fields — the
       orchestrator does NOT set these; the manifest carries them; cfg is read-only here).
  gotcha: cfg is a resolved VALUE (not a pointer) — read fields directly, no deref. Defaults() gives the
          Layer-1 baseline for tests. AutoStageAll/Verbose/NoColor are CLI/UI concerns (ignored here).

- file: internal/stubtest/stubtest.go   (P1.M3.T4.S1 — READ for the FROZEN stub API; do NOT edit)
  section: `func Build(t testing.TB) string` + `type Options struct{…}` + `func Manifest(bin string, o
       Options) provider.Manifest` + `func NewScript(t testing.TB, bin string, responses []string)
       provider.Manifest` + `func Env(o Options) []string`.
  why: the integration tests' ONLY mock. `bin := stubtest.Build(t)` compiles the stub once. Success →
       `stubtest.Manifest(bin, Options{Out:"feat: x"})`. Dedupe-retry → `stubtest.NewScript(t, bin,
       []string{"<dup>","<fresh>"})`. Parse-fail → `stubtest.NewScript(t, bin, []string{"","<good>"})` OR
       `Options{Out:""}`. CAS race → `Options{Out:"x", SleepMS:400}` (gives the test a window to move HEAD).
  gotcha: stubtest.Manifest returns a provider.Manifest with PromptDelivery="stdin" — Render yields a
          CmdSpec whose Stdin is the payload and Env carries the STAGECOACH_STUB_* knobs. The stub is
          invoked through the REAL provider.Execute (so the test exercises the full pipeline). A blank
          NewScript line ⇒ empty stdout ⇒ ParseOutput ok=false ⇒ parse-fail path.

- file: internal/git/updateref_test.go + internal/git/committree_test.go   (P1.M1.T2 — READ for the
       TEST FIXTURE PATTERN to mirror in generate_test.go; do NOT edit)
  section: the fixture helpers `initRepo`/`makeEmptyCommit`/`writeFile`/`stageFile`/`headSHA`/`setIdentityConfig`
       + the CAS test pattern (`casCommit`/`casMoveHEAD`/`casHEAD`).
  why: the generate tests CANNOT import these (they're package-private + in _test.go). Copy the ~10-line
       `exec.Command("git","-C",dir,...)` wrappers into generate_test.go. The CAS test pattern
       (casMoveHEAD to simulate a race) is the template for the CAS-failure scenario.
  gotcha: identity must be set for commit-tree — either repo-local `git config user.name/email` (cleanest,
          no env pollution — setIdentityConfig pattern) OR GIT_AUTHOR_*/GIT_COMMITTER_* env (minGitEnv +
          makeEmptyCommit pattern). Use repo-local config in generate_test.go for robustness.

- url: (PRD §13 snapshot flow + §9 FRs + §18 error handling — already in context as selected_prd_content
       `h3.46`–`h3.50` / `h3.25` / `h2.9` / `h3.69`; ALSO plan/001_f1f80943ac34/prd_snapshot.md and
       architecture/system_context.md §3/§7)
  why: §13.2/§13.3 is the AUTHORITATIVE atomic-flow spec (write-tree freezes; commit-tree dangles;
       update-ref CAS is the only ref mutation). §13.5 is the edge-case list (rootless, conflicts, HEAD-
       moved, timeout, empty-diff). §9.2/§9.5/§9.6/§9.7/§9.9/§9.10 are the FRs this implements (FR6–FR9
       snapshot, FR21–FR29 generation/parse, FR30–FR33 dedupe, FR39–FR42 commit, FR43–FR45 rescue). §18.1
       is the invariant. §20.2 is the property-test contract (idempotent index, atomic HEAD).
  critical: §13.3's four simultaneous guarantees (committed content == snapshot; later-staged stays
            staged; atomic+safe; overlap-able) are the acceptance criteria. §13.5's CAS message + §18.3's
            rescue message are what CASError/RescueError must enable the CLI to render.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; go-toml/v2 + pflag  (UNCHANGED)
go.sum                          # unchanged
cmd/
  stagecoach/main.go             # stub (P1.M1.T1) — UNCHANGED
  stubagent/main.go             # P1.M3.T4.S1 (stub binary) — UNCHANGED (may not exist yet if S1 in-flight; it WILL)
internal/
  config/config.go              # P1.M1.T4 — Config struct (read-only ref)
  generate/
    dedupe.go                   # P1.M3.T2 — ExtractSubject/IsDuplicate (same-package ref)
    dedupe_test.go              # P1.M3.T2 — UNCHANGED
    rescue.go                   # P1.M3.T3 — FormatRescue (same-package ref)
    rescue_test.go              # P1.M3.T3 — UNCHANGED
    generate.go                 # NEW (this subtask) ← CommitStaged + Deps + Result + errors
    generate_test.go            # NEW (this subtask) ← integration tests (stub + temp repos)
  git/git.go                    # P1.M1.T2/T3 — Git interface + ErrCASFailed + FileChange + StagedDiffOptions (ref)
  git/*_test.go                 # P1.M1.T2/T3 — fixture pattern to MIRROR (NOT import)
  prompt/{system,payload}.go    # P1.M3.T1 — BuildSystemPrompt/BuildFallbackPrompt/BuildUserPayload/DetectMultiline (ref)
  provider/{manifest,render,executor,parse}.go  # P1.M2.T1/T4/T5/T6 — Manifest/Render/Execute/ParseOutput (ref)
  stubtest/stubtest.go          # P1.M3.T4.S1 — Build/Options/Manifest/NewScript/Env (FROZEN ref)
Makefile                        # build/test(-race)/coverage/lint/clean/help — UNCHANGED
```

### Desired Codebase tree with files to be added

```bash
internal/generate/generate.go       # NEW — Deps, Result, error sentinels (ErrNothingToCommit/ErrTimeout/
                                    #        ErrRescue) + ErrCASFailed re-export, RescueError/CASError
                                    #        wrappers, CommitStaged(ctx,deps,cfg). The 10-step §13 pipeline.
internal/generate/generate_test.go  # NEW — integration tests via stubtest + temp git repos. Own fixture
                                    #        helpers (initRepo/writeFile/stageFile/headSHA/commitRaw/gitOut).
                                    #        5 scenarios + idempotent-index/atomic-HEAD invariants.
# All other files UNCHANGED. go.mod/go.sum UNCHANGED. After this subtask: the CLI (P1.M4.T1.S2), the
# public API (P1.M3.T5), and the property tests (P1.M5.T1) all consume CommitStaged/Deps/Result/errors.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (§11.3 NEVER STAGE): CommitStaged MUST NOT call git.AddAll, git.StagedFileCount, or any
// "git add". It READS the index (StagedDiff, WriteTree) but never mutates it. Auto-stage-all is the CLI
// layer's job (P1.M4.T1.S2). Entangling staging breaks v2's for-partition composition. Grep-verify:
// `grep -nE 'AddAll|StagedFileCount|"add"' internal/generate/generate.go` MUST be empty. (design §0/§6)

// CRITICAL (snapshot-then-CAS atomicity, §13.2/§18.1): WriteTree (step 3) freezes the index into an
// immutable TREE_SHA; CommitTree (step 7) builds the commit from THAT tree (dangling); UpdateRefCAS
// (step 8) is the SOLE ref mutation. The orchestrator NEVER calls git commit (it would couple what/
// when/fail, §13.1). Any failure before/including step 8 leaves refs+index byte-for-byte unchanged.

// CRITICAL (CAS never forces, FR41): UpdateRefCAS returns wrapped git.ErrCASFailed on HEAD-moved. The
// orchestrator MUST: (a) NOT retry/force, (b) re-read HEAD via RevParseHEAD for `Actual`, (c) return
// *CASError. The 2-arg force form is NEVER used (would clobber a concurrent commit). (design §8)

// CRITICAL (timeout → immediate rescue, NO retry): Execute returning context.DeadlineExceeded means the
// agent was KILLED. Retrying would just time out again (120s × retries wasted). Return
// &RescueError{Kind:ErrTimeout} immediately. FR25/§13.5. (design §5)

// CRITICAL (unified loop bound): `for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++` =
// maxRetries+1 attempts (1 initial + up to maxRetries retries, default 3→4). Parse-fail AND duplicate
// both consume an attempt. Don't write a SEPARATE 1-retry loop for parse (FR29) — it shares the counter.

// CRITICAL (two corrective signals): parse-failure retry → prepend RetryInstruction to the NEXT payload
// (FR29). duplicate retry → append the matched subject to `rejected` so the NEXT payload's §17.3
// rejection block grows (FR32). Track the previous failure mode with a bool to apply the right signal.

// CRITICAL (system prompt ONCE, user payload EACH attempt): BuildSystemPrompt/BuildFallbackPrompt are
// called ONCE before the loop (they don't change). BuildUserPayload is called EACH attempt (the
// rejection list / retry_instruction change). RecentMessages/RecentSubjects/CommitCount fetched ONCE
// (the repo isn't committed until step 8, so they're stable across attempts).

// CRITICAL (isUnborn only from RevParseHEAD): isUnborn is the exit-128 signal from RevParseHEAD, NOT
// string emptiness of the SHA (commit-pi's latent bug, FINDING 1). On isUnborn: parents=nil (root
// commit), expectedOld=40×'0' (CAS), recent=nil (no dup check), fallback prompt, DiffTree(isRoot=true).

// GOTCHA (RescueError fires only AFTER WriteTree): the rescue path is reachable only after step 3
// (treeSHA is set). The nothing-to-commit (step 2) and WriteTree-failure (step 3) paths return BEFORE
// the snapshot — they are NOT rescues (nothing to recover; no TREE_SHA). So RescueError.TreeSHA is
// always non-empty when returned. (FR43: "TREE_SHA is set and NEW_SHA is not".)

// GOTCHA (generate tests can't import git's fixtures): internal/git/*_test.go helpers (initRepo etc.)
// are package-private AND in _test.go → unimportable. Copy minimal versions into generate_test.go.
// Set identity via repo-local `git config user.name/email` (cleanest) to make commit-tree work.

// GOTCHA (ParseOutput on a non-zero-exit stdout): Execute returns captured stdout EVEN on a non-zero
// exit. The orchestrator does NOT short-circuit on *exec.ExitError — it passes stdout to ParseOutput;
// if ok==true (partial-valid message) it proceeds, if ok==false it retries. Only timeout/cancel
// short-circuit (to rescue). (design §5)

// GOTCHA (Render takes cfg.Model/cfg.Provider, may be ""): Render(model, provider, ...) defaults ""
// params to the manifest's DefaultModel/DefaultProvider. Pass cfg.Model/cfg.Provider as-is. Result.Model
// = cfg.Model if non-empty else the resolved manifest DefaultModel. Result.Provider = deps.Manifest.Name.

// GOTCHA (Result.Changes is the home for DiffTree): step 9 calls DiffTree(newSHA, isUnborn) → []FileChange.
// Store in Result.Changes so the CLI prints FR42's listing WITHOUT re-querying git. May be nil/empty for
// a no-op commit (rare; not an error). DiffTree is called ONLY on the success path (after CAS succeeds).

// GOTCHA (don't print / don't exit / don't signal): the orchestrator is PURE orchestration. It returns
// Result/errors; it never writes to stdout/stderr, never calls os.Exit, never installs a signal handler.
// Rendering, exit codes, and signal handling are the CLI layer's job (P1.M4). A cancelled ctx is the
// orchestrator's only signal seam (context.Canceled → ErrRescue).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/generate/generate.go

// Deps carries the runtime collaborators that vary by environment/test. Injected (not resolved inside
// CommitStaged) so tests can pass a stub Manifest (stubtest.Manifest) and the real git.Git, while the
// CLI resolves the manifest via the registry. Manifest arrives UNRESOLVED — CommitStaged validates+
// resolves it before the first Render (mirrors provider.Render's guard).
type Deps struct {
	Git      git.Git            // the git boundary (real *gitRunner via git.New(repo) in prod+tests)
	Manifest provider.Manifest  // the provider manifest to Render+Execute (stub in tests)
}

// Result is the outcome of a successful CommitStaged. Carries everything the CLI (FR42) and the public
// API wrapper (P1.M3.T5) need to render the success report WITHOUT re-querying git. Changes is step 9's
// DiffTree output (the "what landed" file listing); nil/empty for a no-op commit (not an error).
type Result struct {
	CommitSHA string           // NEW_SHA from commit-tree (the published commit; HEAD now points here)
	Subject   string           // ExtractSubject(Message) — for the "[<short-sha>] <subject>" line (FR42)
	Message   string           // the full commit message (subject [+ body]) committed verbatim
	Provider  string           // deps.Manifest.Name — the concrete provider used
	Model     string           // resolved model (cfg.Model or the manifest DefaultModel)
	Changes   []git.FileChange // DiffTree(newSHA, isUnborn) — FR42's file listing
}

// ---- Typed errors (sentinels + context wrappers) ----

// ErrNothingToCommit: the staged diff is empty (nothing meaningful for the model). CLI → exit 2.
// Returned as a bare sentinel (no context needed). Reached BEFORE the snapshot (step 2) — no TREE_SHA.
var ErrNothingToCommit = errors.New("stagecoach: nothing staged to commit")

// ErrTimeout: generation exceeded cfg.Timeout (the agent was killed). CLI → exit 124 + FormatRescue.
// Returned wrapped in *RescueError{Kind: ErrTimeout}. Reached AFTER the snapshot — TREE_SHA is set.
var ErrTimeout = errors.New("stagecoach: generation timed out")

// ErrRescue: generation failed after exhausting retries (parse-fail / duplicate / non-zero exit / ctx
// cancel). CLI → exit 3 + FormatRescue. Returned wrapped in *RescueError{Kind: ErrRescue}.
var ErrRescue = errors.New("stagecoach: commit generation failed after retries")

// ErrCASFailed is git.ErrCASFailed re-exported so the CLI imports a single package. Returned wrapped in
// *CASError. Detected via errors.Is(err, generate.ErrCASFailed) (== errors.Is(err, git.ErrCASFailed)).
// CLI → exit 1 + the §13.5 HEAD-moved message (CASError.Error()).
var ErrCASFailed = git.ErrCASFailed

// RescueError carries the post-snapshot context for PRD §18.3's rescue message (FR43–FR44). Returned
// for BOTH ErrTimeout and ErrRescue (both render FormatRescue; the exit code differs). The CLI does:
//   var re *generate.RescueError; if errors.As(err, &re) { print(FormatRescue(re.TreeSHA, re.ParentSHA,
//       re.Candidate)); exit = errors.Is(err, generate.ErrTimeout) ? 124 : 3 }
type RescueError struct {
	Kind      error  // ErrTimeout or ErrRescue — Unwrap() returns this (enables errors.Is)
	TreeSHA   string // the frozen snapshot (always non-empty — rescue fires only after WriteTree)
	ParentSHA string // "" on a root commit (FormatRescue omits -p)
	Candidate string // the last generated message ("" if none) — FormatRescue appends the candidate note
	Cause     error  // underlying: context.DeadlineExceeded / *exec.ExitError / nil — for verbose/diag
}
func (e *RescueError) Error() string // names Kind + a short reason (e.g. "generation timed out")
func (e *RescueError) Unwrap() error { return e.Kind }

// CASError carries the §13.5 "HEAD moved" context. The orchestrator RE-READS HEAD via RevParseHEAD on
// CAS failure (git.ErrCASFailed docstring, decision D5) to obtain Actual. The CLI does:
//   var ce *generate.CASError; if errors.As(err, &ce) { print(ce.Error()); exit = 1 }
type CASError struct {
	TreeSHA  string // the snapshot tree (for the manual commit-tree recovery command)
	Expected string // the parentSHA captured at step 1 (the CAS expected-old)
	Actual   string // HEAD re-read after the CAS failed ("" if the re-read itself errored — rare)
	Message  string // the generated commit message (for the manual commit-tree -m)
}
func (e *CASError) Error() string  // the §13.5 message: "HEAD moved from <Expected> to <Actual>…"
func (e *CASError) Unwrap() error { return git.ErrCASFailed } // enables errors.Is(err, ErrCASFailed)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/generate/generate.go — types (Deps, Result, errors, RescueError, CASError)
  - FILE: NEW internal/generate/generate.go. PACKAGE: `package generate`. DOC: update the existing
      `// Package generate …` doc comment (currently in dedupe.go) — OR add a package-level doc here that
      supersedes it (Go allows ONE package doc; if dedupe.go has `// Package generate`, REMOVE that line
      from dedupe.go's top and put the canonical package doc in generate.go: `// Package generate implements
      Stagecoach's commit-generation pipeline (PRD §13). CommitStaged is the atomic, snapshot-based
      orchestrator: it captures the parent + a frozen write-tree snapshot, runs a bounded generate→parse→
      dedupe retry loop, builds the commit from the frozen tree via git plumbing, and advances HEAD via a
      single compare-and-swap update-ref (never git commit, never git add).`). NOTE: removing the doc
      from dedupe.go is a 1-line edit to an EXISTING file — this is the ONLY allowed edit; it's necessary
      to avoid duplicate package docs (Go vet/golint flags two `// Package` comments). If you prefer zero
      edits to existing files, instead generate.go's doc comment uses `// Package generate …` and you
      delete the one in dedupe.go. PREFER: move the package doc to generate.go (the orchestrator is the
      package's raison d'être).
  - IMPORT: `context`, `errors`, `fmt`, `github.com/dustin/stagecoach/internal/config`,
      `github.com/dustin/stagecoach/internal/git`, `github.com/dustin/stagecoach/internal/prompt`,
      `github.com/dustin/stagecoach/internal/provider`. NO third-party, NO new internal.
  - DEFINE the types in "Data models" above (Deps, Result, the 4 sentinels, RescueError, CASError) with
      doc comments citing PRD §/FR. ErrCASFailed = git.ErrCASFailed (re-export).
  - IMPLEMENT RescueError.Error() (names Kind + reason) + Unwrap() (returns Kind). IMPLEMENT
      CASError.Error() (the §13.5 message format: see research §8 / PRD §13.5: "HEAD moved from
      <Expected> to <Actual> while generating; aborting to avoid a non-fast-forward. Your generated
      message was: <Message>. To commit the snapshot manually: git commit-tree [-p <Expected>] -m \"…\"
      <TreeSHA> | xargs git update-ref HEAD".) + Unwrap() (returns git.ErrCASFailed).
  - NAMING: exported types CamelCase (Deps, Result, RescueError, CASError); error vars Err*; fields
      CamelCase (Go convention). PLACEMENT: all in internal/generate/generate.go.

Task 2: IMPLEMENT func CommitStaged(ctx, deps, cfg) (Result, error) — the 10-step §13 pipeline
  - SIGNATURE: `func CommitStaged(ctx context.Context, deps Deps, cfg config.Config) (Result, error)`.
  - STEP 1 (parent): parentSHA, isUnborn, err := deps.Git.RevParseHEAD(ctx); if err != nil { return
      Result{}, err } (propagate infra error; do NOT wrap into a typed error — it's an infrastructure
      failure, not a generation failure). On isUnborn, parentSHA=="" (the contract).
  - STEP 2 (diff / nothing-to-commit gate): diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
      MaxDiffBytes: cfg.MaxDiffBytes, MaxMdLines: cfg.MaxMdLines}); if err != nil { return Result{}, err }.
      if diff == "" { return Result{}, ErrNothingToCommit }. (design §6 — StagedDiff emptiness, NOT
      HasStagedChanges; reflects model-visible content; handles the all-excluded-files edge case.)
  - STEP 3 (snapshot / abort on conflict): treeSHA, err := deps.Git.WriteTree(ctx); if err != nil {
      return Result{}, err } (WriteTree fails on unresolved merge conflicts → wrapped "unresolved merge
      conflicts" error; the CLI exits 1. This is BEFORE generation — NOT a rescue; no TREE_SHA to print.)
      *** SNAPSHOT TAKEN — from here, HEAD & the committed content are frozen w.r.t. this run. ***
  - STEP 4 (system prompt + recent subjects, built ONCE):
      var sysPrompt string
      if isUnborn { sysPrompt = prompt.BuildFallbackPrompt(cfg.SubjectTargetChars) }
      else {
          n, err := deps.Git.CommitCount(ctx); if err != nil { return rescueOrErr? }
            — actually: if CommitCount errors, it's an infra failure → return Result{}, err.
          if n <= 1 { sysPrompt = prompt.BuildFallbackPrompt(cfg.SubjectTargetChars) }
          else {
              msgs, err := deps.Git.RecentMessages(ctx, 20); if err != nil { return Result{}, err }
              sysPrompt = prompt.BuildSystemPrompt(msgs, prompt.DetectMultiline(msgs), cfg.SubjectTargetChars)
          }
      }
      var recent []string
      if !isUnborn { recent, err = deps.Git.RecentSubjects(ctx, 50); if err != nil { return Result{}, err } }
      (On isUnborn, recent stays nil → IsDuplicate always false. RecentSubjects on unborn returns
      (nil,nil) but we skip the call per the interface docstring's short-circuit guidance.)
  - STEP 5 (GENERATION+DEDUPE LOOP — design §4):
      resolved := deps.Manifest.Resolve()  // nil-pointer-safe; for RetryInstruction + DefaultModel
      retryInstr := *resolved.RetryInstruction  // resolved default: "Output ONLY the commit message…"
      var rejected []string
      var candidate string       // last generated message (for RescueError.Candidate)
      var parseFail bool         // previous attempt failed parsing → prepend retryInstr next attempt
      var lastCause error        // last Execute error (for RescueError.Cause)
      var msg string             // the successful message (set on break)
      success := false
      for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
          payload := prompt.BuildUserPayload(diff, rejected)
          if parseFail { payload = retryInstr + "\n\n" + payload }  // FR29 corrective preamble
          spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
          if rerr != nil { return Result{}, fmt.Errorf("commit staged: render: %w", rerr) }  // malformed manifest
          out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout)
          if execErr != nil {
              if errors.Is(execErr, context.DeadlineExceeded) {
                  return Result{}, &RescueError{Kind: ErrTimeout, TreeSHA: treeSHA, ParentSHA: parentSHA,
                      Candidate: candidate, Cause: execErr}  // §5: immediate rescue, NO retry
              }
              if errors.Is(execErr, context.Canceled) {
                  return Result{}, &RescueError{Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
                      Candidate: candidate, Cause: execErr}  // interrupted (signal handler cancels ctx)
              }
              // non-zero exit (*exec.ExitError): fall through to ParseOutput; record the cause.
              lastCause = execErr
          } else { lastCause = nil }
          m, ok, _ := provider.ParseOutput(out, deps.Manifest)
          if !ok { parseFail = true; candidate = m; continue }  // FR29 retry (consumes an attempt)
          parseFail = false
          subject := ExtractSubject(m)               // same package (generate.ExtractSubject)
          if IsDuplicate(subject, recent) {          // same package (generate.IsDuplicate)
              rejected = append(rejected, subject); candidate = m; continue  // FR32 retry
          }
          msg = m; success = true; break             // SUCCESS — accept the message
      }
      if !success {
          return Result{}, &RescueError{Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
              Candidate: candidate, Cause: lastCause}  // loop exhausted (parse-fail/dup/non-zero-exit)
      }
  - STEP 7 (commit-tree — build the DANGLING commit object from the FROZEN tree):
      var parents []string
      if !isUnborn { parents = []string{parentSHA} }  // root commit ⇒ nil (no -p)
      newSHA, err := deps.Git.CommitTree(ctx, treeSHA, parents, msg)
      if err != nil { return Result{}, err }  // commit-tree failed (bad tree/parent) — infra error
  - STEP 8 (update-ref CAS — the SOLE ref mutation; design §8):
      expectedOld := parentSHA
      if isUnborn { expectedOld = strings.Repeat("0", 40) }  // all-zeros; CAS succeeds only if unborn
          // NOTE: import "strings" (for Repeat). Add to the import list.
      if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
          if errors.Is(err, git.ErrCASFailed) {
              actual, rerr := deps.Git.RevParseHEAD(ctx)  // re-read HEAD for the §13.5 message (D5)
              if rerr != nil { actual = "" }               // degrade gracefully (rare)
              return Result{}, &CASError{TreeSHA: treeSHA, Expected: parentSHA, Actual: actual, Message: msg}
          }
          return Result{}, err  // non-CAS infra error from update-ref (missing git etc.) — propagate
      }
  - STEP 9 (diff-tree — "what landed" for the report):
      changes, err := deps.Git.DiffTree(ctx, newSHA, isUnborn)  // isRoot=isUnborn → --root flag
      if err != nil { return Result{}, err }  // bad SHA (shouldn't happen post-commit) — infra error
  - STEP 10 (return Result):
      model := cfg.Model; if model == "" { model = *resolved.DefaultModel }  // resolved from step 5
      return Result{CommitSHA: newSHA, Subject: ExtractSubject(msg), Message: msg,
          Provider: deps.Manifest.Name, Model: model, Changes: changes}, nil
  - GOTCHA: the loop's `continue` on parse-fail/dup does NOT reset `candidate` — it accumulates the last
    message so the eventual RescueError.Candidate is meaningful. `parseFail` is set per-iteration.
  - GOTCHA: `strings` must be imported (for strings.Repeat("0",40)). Final import set: context, errors,
    fmt, strings + 4 internal packages.

Task 3: CREATE internal/generate/generate_test.go — integration tests (stub + temp git repos)
  - FILE: NEW internal/generate/generate_test.go. PACKAGE: `package generate` (white-box — can call
      unexported helpers if any; CommitStaged is exported anyway). IMPORT: `context`, `errors`, `os`,
      `os/exec`, `path/filepath`, `regexp`, `strings`, `testing`, `time`,
      `github.com/dustin/stagecoach/internal/config`, `github.com/dustin/stagecoach/internal/git`,
      `github.com/dustin/stagecoach/internal/stubtest`.
  - FIXTURE HELPERS (copy the ~10-line pattern from internal/git/*_test.go — they're unimportable):
      func initRepo(t *testing.T, dir string)       // git init + repo-local user.name/email config
      func writeFile(t *testing.T, dir, name, body) // os.WriteFile
      func stageFile(t *testing.T, dir, name)       // git -C dir add name
      func headSHA(t *testing.T, dir string) string // git -C dir rev-parse HEAD
      func commitRaw(t *testing.T, dir, msg string) // git -C dir commit --allow-empty -m msg (identity via config)
      func gitOut(t *testing.T, dir string, args ...string) string // generic git -C dir ... wrapper
      All with t.Helper(). Set identity via repo-local `git config user.name/email` (cleanest; mirrors
      committree_test.go's setIdentityConfig) so commit-tree works without env pollution.
  - SHARED cfg: `cfg := config.Defaults()` (timeout 120s, maxDup 3, subjectTarget 50, …). For the
      parse-fail-rescue test, override `cfg.MaxDuplicateRetries = 0` (so the loop runs once → blank →
      rescue). For the timeout scenario, override `cfg.Timeout` to a small value (e.g. 200ms) + stub
      SleepMS large.
  - TEST 1 (success): repo with `commitRaw(t,repo,"initial")` + stage a file; bin:=stubtest.Build(t);
      m:=stubtest.Manifest(bin, Options{Out:"feat: add login"}); res,err:=CommitStaged(ctx,
      Deps{Git:git.New(repo), Manifest:m}, cfg); assert err==nil; res.Subject=="feat: add login";
      res.CommitSHA matches `^[0-9a-f]{40,64}$`; headSHA(repo)==res.CommitSHA;
      gitOut(repo,"log","--format=%B","-n1",res.CommitSHA)=="feat: add login"; len(res.Changes)>0;
      res.Provider=="stub"; res.Model=="" OR the manifest default (stub has none → "").
  - TEST 2 (dedupe-retry-then-success): repo with `commitRaw(t,repo,"feat: existing")` (HEAD subject IS
      "feat: existing"); stage a file; m:=stubtest.NewScript(t,bin,[]string{"feat: existing",
      "feat: fresh"}); res,err:=CommitStaged(...); assert err==nil; res.Subject=="feat: fresh";
      headSHA(repo)==res.CommitSHA; the script's call1 ("feat: existing") was rejected as a dup, call2
      ("feat: fresh") accepted. (Pins FR30/FR32 + rejection-list growth.)
  - TEST 3 (parse-fail-rescue): repo with a commit + staged file; cfg.MaxDuplicateRetries=0;
      m:=stubtest.NewScript(t,bin,[]string{"","feat: good"}) (call1 blank → ok=false → loop exhausted);
      _,err:=CommitStaged(...); assert errors.As(err,&re) where re *RescueError; errors.Is(err,ErrRescue);
      re.TreeSHA != "" (non-empty — snapshot was taken); re.ParentSHA != ""; assert HEAD UNCHANGED
      (headSHA before==after) + index UNCHANGED (snapshot `git diff --cached --name-only` before/after
      byte-identical — the idempotent-index invariant, §20.2). Optionally: FormatRescue(re.TreeSHA,
      re.ParentSHA, re.Candidate) is non-empty (renders without panic).
  - TEST 4 (CAS-failure — HEAD moved mid-run): repo with a commit + staged file; parent:=headSHA(repo);
      m:=stubtest.Manifest(bin, Options{Out:"feat: x", SleepMS:400}); done:=make(chan error,1);
      go func(){ _,e:=CommitStaged(ctx, Deps{Git:git.New(repo), Manifest:m}, cfg); done<-e }();
      time.Sleep(150*time.Millisecond);  // let it snapshot + enter generation (stub sleeping)
      concurrent:=commitRaw(t,repo,"concurrent commit");  // move HEAD mid-generation
      err:=<-done; assert errors.As(err,&ce) where ce *CASError; errors.Is(err,git.ErrCASFailed) AND
      errors.Is(err,ErrCASFailed); ce.Expected==parent; ce.Actual==concurrent; ce.TreeSHA!="";
      headSHA(repo)==concurrent (the orchestrator's commit did NOT land — atomic HEAD, §20.2);
      ce.Error() contains "HEAD moved". NOTE: this races on wall-clock — keep the sleep margins generous
      (stub SleepMS 400 >> test's 150ms pre-move) so the move happens DURING generation, not after CAS.
  - TEST 5 (root commit): repo := t.TempDir(); initRepo(t,repo) (UNBORN — no commits); stage a file;
      m:=stubtest.Manifest(bin, Options{Out:"chore: initial"}); res,err:=CommitStaged(...); assert
      err==nil; res.Subject=="chore: initial"; headSHA(repo)==res.CommitSHA; the commit has NO parent
      (`git -C repo cat-file -p <sha>` lacks a "\nparent " line — mirror committree_test.go);
      len(res.Changes)>0 (DiffTree ran with isRoot=true → --root flag produced output).
  - TEST 6 (nothing-to-commit): repo with a commit, NOTHING staged; _,err:=CommitStaged(...); assert
      errors.Is(err,ErrNothingToCommit) (bare sentinel; NOT a *RescueError). HEAD/index unchanged.
  - TEST 7 (timeout → ErrTimeout, no retry): repo with commit + staged file; cfg.Timeout=150*time.
      Millisecond; m:=stubtest.Manifest(bin, Options{Out:"feat: x", SleepMS:2000}); _,err:=
      CommitStaged(...); assert errors.As(err,&re); errors.Is(err,ErrTimeout) (NOT ErrRescue);
      re.TreeSHA!="" (snapshot taken); returns within ~3s (Execute killed the process group); HEAD
      unchanged (atomic). (Optional — covers the timeout branch; cheap with a tiny timeout.)
  - PATTERN: mirror internal/provider/executor_test.go (construct deps, call, assert with errors.Is/As)
      + internal/git/*_test.go (temp repo fixtures, t.Helper, t.TempDir auto-cleanup). Each test is
      independent (own repo). NO real agent, NO network.
  - GOTCHA: for the CAS test, git.New(repo) is called ONCE and shared between the goroutine (CommitStaged)
    and the main thread (commitRaw moves HEAD). git.gitRunner is goroutine-safe (uses -C flag, not
    os.Chdir) so this is fine. The race is intentional — it's what's under test.
  - GOTCHA: the success/dedupe/root tests stage a REAL file so StagedDiff is non-empty (else
    ErrNothingToCommit short-circuits before generation). Use `writeFile`+`stageFile` with a small body.
  - GOTCHA: `config.Defaults()` gives MaxDuplicateRetries=3 — the dedupe test (2 responses) succeeds on
    call2 within the 4-attempt budget; the parse-fail test MUST set MaxDuplicateRetries=0 (or use a
    1-element all-blank script) so the blank exhausts the loop.

Task 4: VERIFY (no further file change beyond the dedupe.go package-doc move in Task 1)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. Every other file
      (all of internal/{git,provider,prompt,config}, cmd/*, pkg/*, Makefile) MUST be byte-unchanged
      EXCEPT the 1-line package-doc move in dedupe.go (if you chose that option). `go build ./...` MUST
      succeed. `go test -race ./...` MUST be green (new generate tests pass, nothing regresses).
      `go vet ./internal/generate/` + `gofmt -l internal/generate/` clean.
```

### Implementation Patterns & Key Details

```go
// The orchestrator's spine — the unified generation loop (design §4/§5). This is the densest part.
// (Pseudocode — see Task 2 for the exact, copy-ready version.)
func CommitStaged(ctx context.Context, deps Deps, cfg config.Config) (Result, error) {
	// 1. parent
	parentSHA, isUnborn, err := deps.Git.RevParseHEAD(ctx)
	if err != nil { return Result{}, err }

	// 2. diff / nothing-to-commit (StagedDiff emptiness — design §6)
	diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
		MaxDiffBytes: cfg.MaxDiffBytes, MaxMdLines: cfg.MaxMdLines,
	})
	if err != nil { return Result{}, err }
	if diff == "" { return Result{}, ErrNothingToCommit }

	// 3. snapshot (abort on merge-conflict failure — BEFORE generation; not a rescue)
	treeSHA, err := deps.Git.WriteTree(ctx)
	if err != nil { return Result{}, err } // "unresolved merge conflicts" — CLI exits 1

	// 4. system prompt (ONCE) + recent subjects (ONCE)
	sysPrompt := buildSystemPrompt(ctx, deps.Git, cfg, isUnborn)   // helper or inline (Task 2 step 4)
	recent := recentSubjectsOrNil(ctx, deps.Git, isUnborn)         // nil on unborn

	// 5. GENERATION+DEDUPE LOOP
	resolved := deps.Manifest.Resolve()
	retryInstr := *resolved.RetryInstruction
	var rejected []string
	var candidate string
	var parseFail, ok bool
	var lastCause error
	var msg string
	success := false
	for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
		payload := prompt.BuildUserPayload(diff, rejected)
		if parseFail {
			payload = retryInstr + "\n\n" + payload // FR29 corrective preamble
		}
		spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
		if rerr != nil { return Result{}, fmt.Errorf("commit staged: render: %w", rerr) }

		out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) {
				return Result{}, &RescueError{Kind: ErrTimeout, TreeSHA: treeSHA, ParentSHA: parentSHA, Candidate: candidate, Cause: execErr}
			}
			if errors.Is(execErr, context.Canceled) {
				return Result{}, &RescueError{Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA, Candidate: candidate, Cause: execErr}
			}
			lastCause = execErr // non-zero exit: fall through; stdout may be partial-valid
		} else {
			lastCause = nil
		}

		msg, ok, _ = provider.ParseOutput(out, deps.Manifest)
		if !ok {
			parseFail = true
			candidate = msg
			continue // FR29 retry (consumes an attempt)
		}
		parseFail = false
		if IsDuplicate(ExtractSubject(msg), recent) {
			rejected = append(rejected, ExtractSubject(msg))
			candidate = msg
			continue // FR32 retry
		}
		success = true
		break
	}
	if !success {
		return Result{}, &RescueError{Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA, Candidate: candidate, Cause: lastCause}
	}

	// 7. commit-tree (dangling object from the FROZEN tree)
	parents := []string(nil)
	if !isUnborn {
		parents = []string{parentSHA}
	}
	newSHA, err := deps.Git.CommitTree(ctx, treeSHA, parents, msg)
	if err != nil { return Result{}, err }

	// 8. update-ref CAS (SOLE ref mutation; never force)
	expectedOld := parentSHA
	if isUnborn {
		expectedOld = strings.Repeat("0", 40)
	}
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
		if errors.Is(err, git.ErrCASFailed) {
			actual, _ := deps.Git.RevParseHEAD(ctx) // re-read (D5); ignore err → actual=""
			return Result{}, &CASError{TreeSHA: treeSHA, Expected: parentSHA, Actual: actual, Message: msg}
		}
		return Result{}, err
	}

	// 9. diff-tree (what landed)
	changes, err := deps.Git.DiffTree(ctx, newSHA, isUnborn)
	if err != nil { return Result{}, err }

	// 10. result
	model := cfg.Model
	if model == "" {
		model = *resolved.DefaultModel
	}
	return Result{
		CommitSHA: newSHA,
		Subject:   ExtractSubject(msg),
		Message:   msg,
		Provider:  deps.Manifest.Name,
		Model:     model,
		Changes:   changes,
	}, nil
}

// CASError.Error() — the §13.5 "HEAD moved" message (PRD §13.5, research §8).
func (e *CASError) Error() string {
	return fmt.Sprintf("HEAD moved from %s to %s while generating; aborting to avoid a non-fast-forward. "+
		"Your generated message was: %s. To commit the snapshot manually: "+
		"git commit-tree -p %s -m %q %s | xargs git update-ref HEAD",
		e.Expected, e.Actual, e.Message, e.Expected, e.Message, e.TreeSHA)
}

// RescueError.Error() — a short reason (the CLI renders the FULL FormatRescue separately).
func (e *RescueError) Error() string {
	switch e.Kind {
	case ErrTimeout:
		return "stagecoach: generation timed out after the snapshot was taken"
	default:
		return "stagecoach: commit generation failed after retries"
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. generate.go imports only already-present internal packages + stdlib. `go mod tidy`
        MUST be a no-op. `git diff --exit-code go.mod go.sum` empty.

PACKAGE EDGES (import graph):
  - internal/generate/generate.go → (stdlib: context/errors/fmt/strings) +
        internal/{config,git,prompt,provider}. NO cycle (none of those import generate).
  - internal/generate/generate_test.go → (stdlib) + internal/{config,git,stubtest} + (same package).
        stubtest (P1.M3.T4.S1) → internal/provider (already present). No cycle.

UPSTREAM CONTRACT (the seams — already built, read-only):
  - git.Git (internal/git/git.go) — RevParseHEAD/StagedDiff/WriteTree/CommitCount/RecentMessages/
        RecentSubjects/CommitTree/UpdateRefCAS/DiffTree; git.ErrCASFailed; git.FileChange;
        git.StagedDiffOptions; git.New(workDir).
  - provider.Manifest.Render(model,provider,sys,userPayload) → *CmdSpec (render.go); provider.Execute(ctx,
        spec, timeout) → (stdout,stderr,err) (executor.go); provider.ParseOutput(raw,m) → (msg,ok,fellback)
        (parse.go); provider.Manifest.Resolve() (manifest.go).
  - prompt.BuildSystemPrompt/BuildFallbackPrompt/BuildUserPayload/DetectMultiline (system.go/payload.go).
  - generate.ExtractSubject/IsDuplicate (dedupe.go); generate.FormatRescue (rescue.go).
  - config.Config + config.Defaults() (config.go).
  - stubtest.Build/Options/Manifest/NewScript/Env (stubtest.go — P1.M3.T4.S1, FROZEN).

DOWNSTREAM CONTRACTS (the consumers — do NOT implement here, just honor the API):
  - P1.M4.T1.S2 (CLI default action): CommitStaged(ctx, Deps{Git: git.New(repo), Manifest: reg.Get(name)},
        cfg); map Result→FR42 report; map errors→exit codes (§15.4) + FormatRescue/§13.5 messages.
  - P1.M4.T2 (signal handler): cancel ctx → CommitStaged observes context.Canceled → &RescueError{ErrRescue}.
  - P1.M3.T5 (public API): wraps CommitStaged with Options; re-exports Result/Deps/errors.
  - P1.M5.T1 (property tests): import stubtest + CommitStaged for the full invariant suite.
  => The CommitStaged/Deps/Result/error API is FROZEN after this subtask.

FROZEN FILES (do NOT edit, with ONE exception):
  - All of internal/{git,provider,prompt,config}, internal/stubtest/*, cmd/*, pkg/*, Makefile, go.mod,
        go.sum. The ONE allowed edit: moving the `// Package generate` doc comment from dedupe.go to
        generate.go (1 line removed from dedupe.go's top) to avoid duplicate package docs. If you'd
        rather touch zero existing files, leave dedupe.go's package doc and give generate.go a NON-package
        leading comment (a plain `// CommitStaged …` file comment) — Go allows a file without a package
        doc as long as exactly one file in the package has it. PREFERRED: move it (cleaner).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the two new files (and the 1-line dedupe.go edit if you moved the package doc)
gofmt -w internal/generate/generate.go internal/generate/generate_test.go

# Vet the generate package
go vet ./internal/generate/

# Lint (if available) — MUST be clean
golangci-lint run ./internal/generate/ 2>/dev/null || echo "(golangci-lint not available — skip)"

# CRITICAL scope check: the orchestrator NEVER stages. This MUST be empty.
grep -nE 'AddAll|StagedFileCount|"add"' internal/generate/generate.go && echo "FAIL: orchestrator stages!" || echo "OK: never stages"

# Confirm generate.go imports only stdlib + the 4 internal packages (no third-party, no new internal)
go list -deps ./internal/generate | grep 'dustin/stagecoach' | sort -u
# Expected: .../internal/config, .../internal/git, .../internal/generate, .../internal/prompt, .../internal/provider

# Confirm exactly one `// Package generate` doc exists in the package (no duplicate-package-doc vet error)
grep -rl '^// Package generate' internal/generate/

# Confirm go.mod/go.sum unchanged
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged ✓"

# Expected: Zero errors. `go vet` clean. The never-stages grep is empty. One package doc.
```

### Level 2: Unit/Integration Tests (THE KEYSTONE — drive CommitStaged via the stub + temp repos)

```bash
# Run the generate suite verbosely (stubtest.Build compiles the stub once on first use)
go test -race -v ./internal/generate/

# Expected: ALL generate tests pass — the 5 contract scenarios + nothing-to-commit + timeout + the 2
# invariants. Specifically:
#   - success: Result.Subject == stub output, HEAD moved, commit message round-trips, Changes non-empty.
#   - dedupe-retry-then-success: call1 (dup subject) rejected, call2 (fresh) succeeds.
#   - parse-fail-rescue: *RescueError + ErrRescue, HEAD+index UNCHANGED (idempotent-index invariant).
#   - CAS-failure: *CASError + git.ErrCASFailed, Actual==concurrentSHA, HEAD==concurrentSHA (atomic).
#   - root commit: no parent line in the commit object, HEAD==CommitSHA, DiffTree(isRoot=true) non-empty.
#   - nothing-to-commit: errors.Is(err, ErrNothingToCommit) (bare sentinel, NOT *RescueError).
#   - timeout (if implemented): *RescueError + ErrTimeout, returns within ~3s, HEAD unchanged.
# Existing dedupe/rescue tests (dedupe_test.go, rescue_test.go) STILL PASS (no regression).

# Full module — confirm NOTHING else regressed (all P1.M1/M2/M3 suites still green; S1's stubtest too)
go test -race ./...

# Expected: all green. If S1 (stubtest) is also in-flight, its tests pass independently; both are additive.
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-module build — generate.go compiles as part of the module
go build ./...

# Manual trace (eyeball, not a test): confirm the stub drives a real commit in a throwaway repo.
TMP=$(mktemp -d); git init -C "$TMP" >/dev/null; git -C "$TMP" config user.name T; git -C "$TMP" config user.email t@e.c
echo "hello" > "$TMP/a.txt"; git -C "$TMP" add a.txt
BIN=$(mktemp); go build -o "$BIN" ./cmd/stubagent   # requires S1 done
# (A tiny throwaway main calling CommitStaged would be ideal, but the generate_test.go suite IS the
# integration validation — Level 2 covers it end-to-end with real git + the real Execute seam.)

# Expected: `go build ./...` succeeds; the generate suite (Level 2) is the authoritative integration proof.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Property/invariant spot-checks (the FULL suite is P1.M5.T1, but generate_test.go pins the core two):

# 1. Idempotent index on the rescue path (PRD §20.2): the parse-fail-rescue test snapshots
#    `git diff --cached --name-only` before AND after the failed CommitStaged and asserts byte-identical.
#    (This is the §18.1 invariant: any non-success path leaves the index unchanged.)

# 2. Atomic HEAD on CAS failure (PRD §20.2): the CAS-failure test asserts `git rev-parse HEAD` is the
#    concurrent commit's SHA (NOT the orchestrator's) — the CAS refused the non-fast-forward.

# 3. Snapshot immutability (§20.2, light): in the success test, after CommitStaged, stage an EXTRA file
#    and assert it is NOT in res.CommitSHA's tree (it landed after the snapshot) but IS still staged
#    (`git diff --cached --name-only` lists it). This pins the stage-while-generating guarantee (§13.3/§13.4).

# Coverage (the generate package is gated at ≥85% by P1.M5.T3.S3 eventually):
go test -cover ./internal/generate/  # informational; aim for high coverage of CommitStaged's branches

# Expected: all invariants hold; CommitStaged's branches (success/dedupe/parse-fail/timeout/CAS/root/
# nothing-to-commit) are each exercised by a named test.
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed successfully.
- [ ] `go build ./...` succeeds (generate.go compiles).
- [ ] All tests pass: `go test -race ./...` (new generate tests green; dedupe/rescue + all upstream suites
      still green; S1's stubtest green if present).
- [ ] No vet errors: `go vet ./internal/generate/`.
- [ ] No formatting issues: `gofmt -l internal/generate/` (empty output).
- [ ] No lint warnings: `golangci-lint run ./internal/generate/` (if available).
- [ ] Scope check: `grep -nE 'AddAll|StagedFileCount|"add"' internal/generate/generate.go` empty (never stages).
- [ ] go.mod/go.sum byte-unchanged: `git diff --exit-code go.mod go.sum`.

### Feature Validation

- [ ] Success: `CommitStaged` produces a real commit; HEAD moved to `Result.CommitSHA`; the message
      round-trips via `git log --format=%B`; `Result.Changes` is non-empty.
- [ ] Dedupe-retry-then-success: a duplicate subject is rejected (FR30/FR32) and the next attempt's fresh
      subject is accepted; the rejection list grew.
- [ ] Parse-fail-rescue: an empty/blank stub output → `*RescueError{ErrRescue}` with a non-empty `TreeSHA`;
      HEAD + index byte-unchanged (idempotent-index invariant).
- [ ] CAS-failure: HEAD moved mid-run → `*CASError` (errors.Is git.ErrCASFailed); `Actual` == the
      concurrent SHA; HEAD == the concurrent SHA (atomic-HEAD invariant); the orchestrator did NOT force.
- [ ] Root commit: unborn repo → success with a parentless commit; `DiffTree` ran with `isRoot=true`.
- [ ] Nothing-to-commit: empty staged diff → `errors.Is(err, ErrNothingToCommit)` (bare sentinel).
- [ ] Timeout (if tested): `cfg.Timeout` exceeded → `*RescueError{ErrTimeout}` (NOT ErrRescue), returned
      within seconds (process-group kill fired).
- [ ] Every typed error enables the CLI's exit-code mapping via `errors.Is`/`errors.As` (§15.4).

### Code Quality Validation

- [ ] Follows existing patterns: CmdSpec+Execute+assert (mirror executor_test.go); temp-repo fixtures +
      t.Helper + t.TempDir (mirror git/*_test.go); table-free where each scenario needs distinct repo setup.
- [ ] File placement matches the desired codebase tree (`internal/generate/{generate,generate_test}.go`).
- [ ] Anti-patterns avoided (see Anti-Patterns section).
- [ ] Imports properly managed: generate.go → stdlib + 4 internal; generate_test.go → stdlib + 3 internal
      + same package. No cycle. No new dependency.
- [ ] Doc comments cite PRD §/FR (the orchestrator IS §13; errors cite §15.4/§18.3/§13.5).
- [ ] `CommitStaged`/`Deps`/`Result`/error API matches the FROZEN contract for P1.M4/P1.M3.T5/P1.M5.T1.

### Documentation & Deployment

- [ ] Code is self-documenting with PRD-cited doc comments (the orchestrator IS PRD §13's core flow).
- [ ] No new environment variables (cfg is the input; the orchestrator reads none from the env directly).
- [ ] No new config keys (the orchestrator consumes the existing cfg fields; adds none).
- [ ] No new user-facing surface (PRD "DOCS: none — internal orchestrator"; the public docstring is P1.M3.T5).

---

## Anti-Patterns to Avoid

- ❌ Don't call `git add`/`AddAll`/`StagedFileCount` inside `CommitStaged` — PRD §11.3: staging is the CLI
  layer's job. Entangling it breaks v2's `for-partition{reset+stage; commitStaged()}` composition.
- ❌ Don't call `git commit` (the porcelain) — use the plumbing (`write-tree`→`commit-tree`→`update-ref`).
  `git commit` couples what/when/whether-fail (§13.1) and can leave the index/HEAD in surprising states.
- ❌ Don't force-update HEAD on CAS failure (no 2-arg `update-ref`) — FR41: abort + message, never clobber.
- ❌ Don't retry on timeout — the agent was killed; retrying wastes 120s × retries. Go straight to rescue.
- ❌ Don't build a SEPARATE 1-retry loop for parse failures (FR29) — it shares the MaxDuplicateRetries
  counter with the duplicate loop (FR32). One unified loop, two corrective signals.
- ❌ Don't rebuild the system prompt per attempt — build it ONCE (it doesn't change). Only the user
  payload (rejection list / retry_instruction) changes per attempt.
- ❌ Don't fetch RecentMessages/RecentSubjects/CommitCount per attempt — fetch ONCE (the repo isn't
  committed until step 8, so they're stable across the loop).
- ❌ Don't short-circuit ParseOutput on a non-zero agent exit — the captured stdout may be a valid partial
  message. Only timeout/cancel short-circuit (to rescue). Pass stdout to ParseOutput and let `ok` decide.
- ❌ Don't print / `os.Exit` / install a signal handler in the orchestrator — it's pure orchestration.
  Rendering, exit codes, and signals are the CLI layer's job (P1.M4). A cancelled ctx is the only seam.
- ❌ Don't re-query git for the file-changes report in the CLI — carry it in `Result.Changes` (step 9's
  DiffTree output) so the CLI renders FR42 from the Result alone.
- ❌ Don't import git's `_test.go` fixture helpers into generate's tests — they're unimportable (package-
  private + test-only). Copy the minimal ~10-line wrappers into generate_test.go.
- ❌ Don't forget the unborn/root path: `parents=nil`, `expectedOld=40×'0'`, `recent=nil`, fallback prompt,
  `DiffTree(isRoot=true)`. A naive `parents=[]string{parentSHA}` with `parentSHA==""` makes a root commit
  with a bogus empty-string parent (commit-tree errors).
- ❌ Don't leave duplicate `// Package generate` docs (Go vet flags it) — move the doc to generate.go.
- ❌ Don't implement the CLI, signal handler, public API wrapper, dry-run, or property tests here — those
  are P1.M4 / P1.M3.T5 / P1.M4.T4 / P1.M5.T1. This subtask is the orchestrator + its integration tests.
