# Code Context — Dry-Run Path Divergence (PRD Issues 2 & 6)

Scope: how `pkg/stagehand`'s DryRun diverges from the real `generate.CommitStaged` pipeline, and the
exact reusable seams an implementer can lift to make DryRun run the full FR49 pipeline
(diff→snapshot→generate→parse→**duplicate-check**, no commit).

## Files Retrieved

1. `pkg/stagehand/stagehand.go` (full file, lines 1-355) — `GenerateCommit` dispatch, `Options`/`Result`,
   `runPipeline` (the DryRun/SystemExtra entrypoint). DryRun short-circuit is lines 259-280; commit
   path's duplicated loop is lines 281-339.
2. `internal/generate/generate.go` (full file, lines 1-end) — `CommitStaged` real pipeline; the
   authoritative generate→parse→dedupe loop is lines 144-225. `Deps`, `Result`, `RescueError`,
   `CASError`, sentinels.
3. `internal/generate/dedupe.go` (full file) — `ExtractSubject` + `IsDuplicate` (the FR30/FR32 helpers).
4. `internal/git/git.go` (lines 43-92 interface; 209-235 WriteTree impl) — `Git` interface incl.
   `WriteTree(ctx) (sha string, err error)` and `RecentSubjects(ctx, n) ([]string, error)`.
5. `internal/git/writetree_test.go` (full file) — confirms `WriteTree` returns 40/64-hex SHA, errors on
   merge conflicts / missing git / cancelled ctx.
6. `internal/provider/parse.go` (full file) — `ParseOutput(raw, m Manifest) (msg, ok, fellback)`.
7. `internal/provider/executor.go` (full file) — `Execute(ctx, spec, timeout, vb) (stdout, stderr, err)`.
8. `pkg/stagehand/stagehand_test.go` (lines 140-285) — DryRun test coverage.
9. `internal/config/config.go:29,45,62` — `MaxDuplicateRetries` field, default 3.
10. `plan/.../prd_snapshot.md` (lines 70-93, 160-185) — Issue 2 (FR49 dup-check) & Issue 6 (snapshot).

---

## 1. The real generate→parse→dedupe loop (`generate.CommitStaged`)

File: `internal/generate/generate.go`. The loop is bounded by `cfg.MaxDuplicateRetries`
(config: `max_duplicate_retries`, default **3** → **max_duplicate_retries+1 = 4 attempts**):

```go
// Step 5: GENERATION+DEDUPE LOOP (design §4 — FR29 + FR32 share one bounded counter).
resolved := deps.Manifest.Resolve()
retryInstr := *resolved.RetryInstruction // resolved default: "Output ONLY the commit message…"

var rejected []string
var candidate string // last generated message (for RescueError.Candidate)
var parseFail bool   // previous attempt failed parsing → prepend retryInstr next attempt
var lastCause error  // last Execute error (for RescueError.Cause)
var msg string       // the successful message (set on break)
success := false

for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
    // Build user payload each attempt (rejection list / retry_instruction change).
    payload := prompt.BuildUserPayload(diff, rejected)
    if parseFail {
        payload = retryInstr + "\n\n" + payload // FR29 corrective preamble
    }

    spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
    if rerr != nil {
        return Result{}, fmt.Errorf("commit staged: render: %w", rerr)
    }

    out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
    if execErr != nil {
        if errors.Is(execErr, context.DeadlineExceeded) {
            // §5: immediate rescue, NO retry — agent was killed.
            return Result{}, &RescueError{
                Kind: ErrTimeout, TreeSHA: treeSHA, ParentSHA: parentSHA,
                Candidate: candidate, Cause: execErr,
            }
        }
        if errors.Is(execErr, context.Canceled) {
            return Result{}, &RescueError{
                Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
                Candidate: candidate, Cause: execErr,
            }
        }
        // Non-zero exit (*exec.ExitError): fall through to ParseOutput.
        // stdout may be partial-valid. Record the cause for eventual rescue.
        lastCause = execErr
    } else {
        lastCause = nil
    }

    m, ok, _ := provider.ParseOutput(out, deps.Manifest)
    if !ok {
        parseFail = true
        candidate = m
        deps.Verbose.VerboseRetry(attempt+1, "parse failed (no valid commit message)")
        continue // FR29 retry (consumes an attempt)
    }
    parseFail = false
    signal.SetCandidate(m) // keep the §18.3 candidate note current

    subject := ExtractSubject(m) // same package — no prefix
    if IsDuplicate(subject, recent) {
        rejected = append(rejected, subject)
        candidate = m
        deps.Verbose.VerboseRetry(attempt+1, fmt.Sprintf("subject %q matches an existing commit", subject))
        continue // FR32 retry (consumes an attempt)
    }

    msg = m
    success = true
    break // SUCCESS — accept the message
}
if !success {
    return Result{}, &RescueError{
        Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
        Candidate: candidate, Cause: lastCause,
    }
}
```

Retry conditions (each consumes one attempt from the shared `MaxDuplicateRetries+1` budget):
- **FR29 parse-retry**: `ParseOutput(...)` returns `ok==false` → `parseFail=true`; the NEXT attempt
  prepends `retryInstr` (the manifest's `RetryInstruction`) to the payload. (No retry happens on the
  same attempt that failed — the corrective preamble applies on the *next* iteration.)
- **FR30–FR33 duplicate rejection**: `IsDuplicate(ExtractSubject(m), recent)` → append subject to
  `rejected`; the next call to `prompt.BuildUserPayload(diff, rejected)` includes the rejection list
  so the model avoids repeats.
- **Non-zero exit** (`*exec.ExitError`): recorded in `lastCause`, falls through to `ParseOutput`
  (partial stdout may still parse). Only becomes a hard failure if the loop exhausts.
- **DeadlineExceeded / Canceled**: immediate `*RescueError` (no further attempts).
- **Exhaustion** (`!success`): `*RescueError{Kind: ErrRescue, ..., Cause: lastCause}`.

`recent` is built ONCE before the loop:
```go
recent, err := recentSubjects(ctx, deps.Git, isUnborn) // nil on unborn → vacuous dup check
```
where `recentSubjects` calls `g.RecentSubjects(ctx, 50)`.

---

## 2. The DryRun code path (`pkg/stagehand.runPipeline`)

File: `pkg/stagehand/stagehand.go`. `GenerateCommit` dispatches at lines 94-110:

```go
// Common path: no DryRun, no SystemExtra → delegate to the frozen, tested orchestrator.
if !opts.DryRun && opts.SystemExtra == "" {
    res, gerr := generate.CommitStaged(ctx, deps, cfg)
    ...
}

// Advanced path: DryRun and/or SystemExtra → self-contained (CommitStaged can't honor these).
return runPipeline(ctx, deps, cfg, opts.SystemExtra, opts.DryRun)
```

Inside `runPipeline` (lines 206-355), the divergence is exactly three things:

### (a) `if !dryRun` gates WriteTree (PRD Issue 6 — snapshot skipped)

```go
// Step 3 (commit path only): snapshot. DryRun skips it (no commit → no object-store write).
var treeSHA string
if !dryRun {
    treeSHA, err = deps.Git.WriteTree(ctx)
    if err != nil {
        return Result{}, err
    }
    signal.SetSnapshot(treeSHA, parentSHA, "") // arm rescue (§18.4)
}
```

So in DryRun: `treeSHA == ""`, no `signal.SetSnapshot`, no dangling tree object created.

### (b) DryRun takes a SHORT-CIRCUIT single-pass branch (PRD Issue 2 — no dup-check, no retry)

```go
// ---- DryRun: single pass, no commit. ----
if dryRun {
    payload := prompt.BuildUserPayload(diff, nil)
    spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
    if rerr != nil {
        return Result{}, fmt.Errorf("render: %w", rerr)
    }
    out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
    if execErr != nil {
        if errors.Is(execErr, context.DeadlineExceeded) {
            return Result{}, ErrTimeout            // <-- bare sentinel, NOT *RescueError
        }
        return Result{}, fmt.Errorf("generate: %w", execErr)
    }
    msg, ok, _ := provider.ParseOutput(out, deps.Manifest)
    if !ok {
        return Result{}, errors.New("dry run: model produced no valid message")  // <-- no FR29 retry
    }
    return Result{
        CommitSHA: "",
        Subject:   generate.ExtractSubject(msg),
        Message:   msg,
        Provider:  deps.Manifest.Name,
        Model:     model,
    }, nil
}
```

### (c) No duplicate check anywhere on the dry-run branch

`generate.IsDuplicate` is **never called** when `dryRun` is true. `recent` (built at line 251-257) is
computed but unused on the dry-run branch — it is only consumed by the commit-path loop below.

Note: the same loop body IS duplicated (a second copy) at lines 281-339 for the SystemExtra commit
path — that copy does honor dedupe/retry/`*RescueError`. So the codebase already has TWO copies of
the loop; the DryRun branch is the only one that omits it.

---

## 3. Function signatures (the reusable seams)

```go
// internal/generate/generate.go
func CommitStaged(ctx context.Context, deps Deps, cfg config.Config) (Result, error)

type Deps struct {
    Git      git.Git
    Manifest provider.Manifest
    Verbose  *ui.Verbose
}
type Result struct {
    CommitSHA string
    Subject   string
    Message   string
    Provider  string
    Model     string
    Changes   []git.FileChange
}

// internal/generate/dedupe.go
func ExtractSubject(message string) string
func IsDuplicate(subject string, recent []string) bool

// internal/git/git.go (interface)
type Git interface {
    WriteTree(ctx context.Context) (sha string, err error)
    RecentSubjects(ctx context.Context, n int) (subjects []string, err error)
    RevParseHEAD(ctx context.Context) (sha string, isUnborn bool, err error)
    StagedDiff(ctx context.Context, opts StagedDiffOptions) (diff string, err error)
    CommitTree(ctx context.Context, tree string, parents []string, msg string) (sha string, err error)
    UpdateRefCAS(ctx context.Context, ref, newSHA, expectedOld string) error
    DiffTree(ctx context.Context, sha string, isRoot bool) ([]FileChange, error)
    // ...plus RecentMessages, CommitCount, AddAll, StagedFileCount, HasStagedChanges
}

// internal/provider/parse.go
func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)

// internal/provider/executor.go
func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout string, stderr string, err error)

// internal/provider/manifest (Render + Resolve)
func (m Manifest) Render(model, providerName, sysPrompt, payload string) (*CmdSpec, error)
func (m Manifest) Resolve() ResolvedManifest   // *RetryInstruction, *DefaultModel, etc. — nil-safe

// internal/prompt/payload.go
func BuildUserPayload(diff string, rejected []string) string
```

`generate.CommitStaged`'s loop is fully self-contained except for `buildSystemPrompt` /
`recentSubjects` (unexported in `internal/generate`). The exported primitives the runPipeline
author already imports (`generate.ExtractSubject`, `generate.IsDuplicate`, `generate.RescueError`,
`generate.CASError`, `provider.Execute`, `provider.ParseOutput`, `prompt.BuildUserPayload`) are
sufficient to reimplement the loop inline — which is exactly what the SystemExtra commit-path copy
at runPipeline lines 281-339 already does.

> **Implementation note (not a finding, just an option):** the cleanest fix for both Issue 2 and
> Issue 6 is to make `runPipeline`'s dry-run branch (a) take the snapshot unconditionally, and
> (b) run the same loop as the SystemExtra commit-path (lines 281-339), then skip ONLY the
> `CommitTree` + `UpdateRefCAS` step and return `CommitSHA: ""`. The `signal.SetSnapshot` /
> `signal.SetCandidate` plumbing is already there; the loop body is already duplicated once and can
> be shared. The loop's `*RescueError{TreeSHA: treeSHA, ...}` would then carry a real TreeSHA in
> dry-run too (today dry-run returns bare `ErrTimeout` with no TreeSHA — see test at
> stagehand_test.go:246).

---

## 4. FR49 / FR30-33 / FR29 — current implementation status in DryRun

| Requirement | Real commit path | DryRun path | Status |
|---|---|---|---|
| **FR49** "full diff→snapshot→generate→parse→duplicate-check pipeline, no commit, exit 0" | ✓ (CommitStaged) | ✗ single-pass; no snapshot, no dup-check | **Issue 2 + Issue 6** |
| **FR30** extract subject | ✓ `ExtractSubject` (used to populate `Result.Subject`) | ✓ (only on the final returned `Result`) | OK superficially |
| **FR31** build rejection list | ✓ `rejected []string` → `BuildUserPayload` | ✗ `BuildUserPayload(diff, nil)` — always nil | **Missing** |
| **FR32** exact-match dup retry | ✓ `IsDuplicate(subject, recent)` → `continue` | ✗ never called | **Missing (Issue 2)** |
| **FR33** bounded retries | ✓ `MaxDuplicateRetries+1` | ✗ single attempt | **Missing** |
| **FR29** parse-fail retry | ✓ `parseFail` flag + `retryInstr` preamble | ✗ returns plain `errors.New("dry run: model produced no valid message")` | **Missing** |
| Snapshot (write-tree) | ✓ step 3 | ✗ gated `if !dryRun` | **Missing (Issue 6)** |
| Rescue on timeout | ✓ `*RescueError{Kind:ErrTimeout, TreeSHA}` | bare `ErrTimeout` (no TreeSHA, no candidate) | **Divergent** |
| Rescue on exhaustion | ✓ `*RescueError{Kind:ErrRescue}` | N/A (never retries) | **Divergent** |

PRD FR49 verbatim (`PRD.md:314` / `plan/.../prd_snapshot.md:314`):
> **FR49.** `--dry-run` — run the full diff→snapshot→generate→parse→duplicate-check pipeline,
> print the resulting message, but **do not** create the commit or move HEAD. Exit 0.

The "do not create the commit or move HEAD" clause is honored; the "full … pipeline" clause is not.

---

## 5. DryRun tests in `pkg/stagehand/stagehand_test.go`

Only two test cases touch DryRun:

1. **`TestGenerateCommit_DryRun`** (lines 156-186) — stages a file, calls
   `GenerateCommit(ctx, Options{Provider: "stub", DryRun: true})` with stub output `"feat: preview"`.
   Asserts: `CommitSHA == ""`, `Message == "feat: preview"`, `Subject == "feat: preview"`, HEAD
   unchanged. **Happy-path only; uses a stub that always succeeds on the first attempt, so it cannot
   detect the missing dup-check / retry loop.**

2. **`TestGenerateCommit_Timeout`/`dryrun`** (lines 224-250) — stub sleeps 2000ms with a 150ms
   `Timeout`. Asserts `errors.Is(err, ErrTimeout)` AND that the error is **NOT** a `*RescueError`
   (`errors.As(err, &re)` must be false). This test **locks in** the current divergent behavior
   (bare `ErrTimeout`, no TreeSHA) — an implementer who makes dry-run return `*RescueError` on
   timeout will need to update this assertion.

**No test** covers: dry-run duplicate-retry, dry-run parse-retry (FR29), dry-run exhaustion rescue,
or dry-run snapshot creation. Any fix for Issue 2/6 should add these.

(For reference, the commit-path dup/retry behavior is tested elsewhere — the
`TestGenerateCommit_Timeout`/`commit_path` subtest at lines 251-269 asserts `*RescueError` with
non-empty `TreeSHA` via `SystemExtra` to force the runPipeline commit branch.)

---

## 6. Options / Result types

```go
// pkg/stagehand/stagehand.go
type Options struct {
    Provider    string        // manifest name; "" → resolved default
    Model       string        // "" → manifest default_model
    SystemExtra string        // appended to the built system prompt
    DryRun      bool          // if true, return the message WITHOUT committing (CommitSHA == "")
    Timeout     time.Duration // per-attempt generation timeout; 0 → config default (120s)
    Verbose     io.Writer     // diagnostics sink (CLI passes stderr); nil ⇒ silent
    VerboseOn   bool          // forces cfg.Verbose=true (CLI --verbose)
}

type Result struct {
    CommitSHA string // "" if DryRun or not committed
    Subject   string
    Message   string
    Provider  string
    Model     string
}
```

`generate.Result` (internal) additionally carries `Changes []git.FileChange` (FR42 file listing) —
`pkg/stagehand.GenerateCommit` deliberately drops `Changes` on the common delegation path (commented
"drop res.Changes (design §1)", stagehand.go:104). DryRun returns it empty today.

Re-exported typed errors in `pkg/stagehand` (stagehand.go:53-72):
`ErrNothingToCommit`, `ErrTimeout`, `ErrRescue`, `ErrCASFailed`; type aliases
`RescueError = generate.RescueError`, `CASError = generate.CASError`.

---

## Architecture (how the pieces connect)

```
GenerateCommit (pkg/stagehand)
  ├── resolveConfig   → config.Config (7-layer, opts override)
  ├── buildDeps       → generate.Deps{Git, Manifest, Verbose}
  └── dispatch:
        !DryRun && SystemExtra==""  →  generate.CommitStaged   [FROZEN, fully tested, real loop]
        DryRun || SystemExtra!=""   →  runPipeline             [self-contained mirror]
                                          ├── RevParseHEAD
                                          ├── StagedDiff  (empty → ErrNothingToCommit)
                                          ├── WriteTree        ← gated `if !dryRun`  (Issue 6)
                                          ├── buildSysPrompt (+ SystemExtra)
                                          ├── RecentSubjects(50)
                                          ├── if dryRun: SINGLE PASS  (Issue 2: no dup, no retry)
                                          │               ├── Execute
                                          │               ├── ParseOutput  (!ok → plain error)
                                          │               └── return Result{CommitSHA:""}
                                          └── else (commit path): DUPLICATE of CommitStaged's loop
                                                          (lines 281-339), then CommitTree + UpdateRefCAS
```

The duplication is the root smell: `CommitStaged` and `runPipeline`'s commit branch share an
identical loop body (copy-paste), while `runPipeline`'s dry-run branch is a *third*, degraded
implementation. Consolidating the loop (either by routing DryRun through `CommitStaged` with a
no-commit flag, or by extracting the loop into a shared helper) fixes Issue 2 and Issue 6 together.

## Start Here

Open **`pkg/stagehand/stagehand.go:206-355`** (`runPipeline`). The dry-run short-circuit at
**lines 259-280** is the single block to replace with the full loop (already present at lines
281-339 for the SystemExtra commit path — copy/share it, drop only the `CommitTree`/`UpdateRefCAS`
tail, keep `WriteTree` unconditional). Then update **`stagehand_test.go:224-250`**
(`TestGenerateCommit_Timeout`/`dryrun`) which currently asserts the bare-`ErrTimeout` behavior, and
add dry-run dup-retry / parse-retry tests mirroring the commit-path ones.

---

## Acceptance Report

This was a **research/scout** task: no source files were modified, no tests added, no commands run
beyond read-only inspection. The deliverable is this findings document at the authoritative path.

```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "Scoped research only — no code changes made. All 6 requested research questions answered with exact code quotes (loop in generate.go:144-225, dry-run branch in stagehand.go:259-280, WriteTree gate at stagehand.go:227, signatures for CommitStaged/WriteTree/ParseOutput/Execute/IsDuplicate/ExtractSubject, FR49/FR30-33/FR29 status table, dry-run tests at stagehand_test.go:156-250, Options/Result types). Findings written to the authoritative path."
    }
  ],
  "changedFiles": [],
  "testsAddedOrUpdated": [],
  "commandsRun": [
    {
      "command": "read pkg/stagehand/stagehand.go, internal/generate/generate.go, internal/generate/dedupe.go, internal/git/git.go (WriteTree), internal/git/writetree_test.go, internal/provider/parse.go, internal/provider/executor.go, pkg/stagehand/stagehand_test.go, internal/config/config.go, plan/.../prd_snapshot.md",
      "result": "passed",
      "summary": "All target files located and read; no writetree.go standalone file exists (WriteTree lives in internal/git/git.go:219)."
    }
  ],
  "validationOutput": [
    "Confirmed WriteTree implementation is in internal/git/git.go:219 (not internal/git/writetree.go as the task brief assumed — that path does not exist; writetree_test.go does).",
    "Confirmed MaxDuplicateRetries default = 3 (config.go:62), so the real loop runs up to 4 attempts.",
    "Confirmed the dry-run branch returns bare ErrTimeout on timeout (stagehand.go:269) and a plain errors.New on parse-failure (stagehand.go:274) — neither is a *RescueError and neither carries TreeSHA.",
    "Confirmed TestGenerateCommit_Timeout/dryrun (stagehand_test.go:246) actively asserts dry-run timeout is NOT a *RescueError — this assertion must change if the fix unifies the paths."
  ],
  "residualRisks": [
    "The task brief referenced internal/git/writetree.go which does not exist; WriteTree is defined in internal/git/git.go:219 (interface at :50). An implementer following the brief literally would not find the file.",
    "TestGenerateCommit_Timeout/dryrun encodes the CURRENT divergent error contract (bare ErrTimeout, no RescueError, no TreeSHA). Unifying dry-run with the real loop will break this test unless the assertion is updated — flag for the implementer.",
    "signal.SetSnapshot/RestoreDefault/ClearSnapshot side-effects are currently skipped entirely in dry-run (because WriteTree is gated). Taking the snapshot in dry-run will start arming/disarming the signal-rescue machinery on dry-run runs; verify no unintended rescue-message side effects in the CLI dry-run output path."
  ],
  "noStagedFiles": true,
  "diffSummary": "No diff — read-only scout; produced architecture/seam_dryrun.md findings document only.",
  "reviewFindings": [
    "no blockers (research-only deliverable)"
  ],
  "manualNotes": "Implementation hint for the parent: the loop body needed for dry-run is ALREADY duplicated at pkg/stagehand/stagehand.go:281-339 (the SystemExtra commit path). The minimal fix is to (1) move WriteTree out of the `if !dryRun` gate (stagehand.go:227), (2) delete the short-circuit dry-run branch (lines 259-280), and (3) make the existing loop (281-339) run for both dryRun and !dryRun, skipping only CommitTree+UpdateRefCAS+DiffTree when dryRun. That collapses three near-duplicate code paths into one and fixes Issue 2 + Issue 6 simultaneously. Update stagehand_test.go:246 (timeout dryrun assertion) and add dry-run dup/parse-retry tests."
}
```
