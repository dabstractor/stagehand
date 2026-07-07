# Hooks Runner and All Callers — Research Findings

## Scope
Issues 1 (prepare-commit-msg argc), 2 (missing trailing newline), 4 (no empty-message guard after hooks).

## Issue 1: `runPrepareCommitMsg` argv (argc=2 → argc=1)

**File**: `internal/hooks/runner.go:195`
```go
return runHook(ctx, cfg.HookTimeout, hookPath, []string{msgPath, ""}, gitDir, workTree, nil, opts)
```
- The hook is invoked as `prepare-commit-msg <msgfile> ""` — **two** args. For a plain `git commit`
  (no `-m`/merge/squash/amend) git invokes it with **one** arg (`$#`=1, `$2` unset).
- The runner's comments at `runner.go:52` and `runner.go:178` claim **"VERIFIED argc=2 for a plain
  commit"** — this claim is false (git 2.54.0: `ARGC=1`).
- **Fix**: change `[]string{msgPath, ""}` → `[]string{msgPath}`. Correct the comments.
- **Single fix point** — only `runner.go:195`.

### Surrounding function (runner.go:183-195)
```go
func runPrepareCommitMsg(ctx context.Context, cfg config.Config, opts HookOpts,
    hooksDir, gitDir, workTree, msgPath string) error {
    hookPath := filepath.Join(hooksDir, "prepare-commit-msg")
    if !hookExecutable(hookPath) {
        return nil
    }
    if shouldSkipStagecoachPrepareCommitMsg(hooksDir) {
        if opts.Verbose != nil {
            opts.Verbose.VerboseWarn("skipping stagecoach's own prepare-commit-msg hook...")
        }
        return nil
    }
    return runHook(ctx, cfg.HookTimeout, hookPath, []string{msgPath, ""}, gitDir, workTree, nil, opts)
}
```

## Issue 2: Message file write — no trailing newline

**File**: `internal/hooks/runner.go:103`
```go
if _, werr := msgFile.WriteString(finalMsg); werr != nil {
```
- `finalMsg` written with NO trailing newline. git writes `msg\n` to `COMMIT_EDITMSG`.
- When a hook does `echo "Signed-off-by: …" >> "$1"`, the append concatenates onto the subject line.
- **Fix**: ensure file ends with `\n` if `finalMsg` does not.
```go
if !strings.HasSuffix(finalMsg, "\n") {
    finalMsg += "\n"
}
```
- `finalMsg` is reassigned from file read-back at line 131 (`stripCommentLines`), so the variable
  mutation is harmless — the `+= "\n"` only affects what's written to the temp file.
- **Single fix point** — only `runner.go:103` area.

### Message-file lifecycle (runner.go:91-131)
```go
msgFile, err := os.CreateTemp("", "stagecoach-hookmsg-*.txt")  // line 97
msgPath := msgFile.Name()
defer os.Remove(msgPath)
if _, werr := msgFile.WriteString(finalMsg); werr != nil {     // ← LINE 103 (no \n)
    ...
}
msgFile.Close()
// run prepare-commit-msg (line 108) then commit-msg (line 117) on msgPath...
// Final read-back + strip (line 126-131)
data, rErr := os.ReadFile(msgPath)
finalMsg = stripCommentLines(string(data), commentChar)  // line 131
```

## Issue 4: `RunCommitHooks` return contract + caller empty-check gap

### RunCommitHooks signature (runner.go:67-69)
```go
func RunCommitHooks(ctx context.Context, g git.Git, cfg config.Config, snapshotTree, parentSHA, msg string,
    opts HookOpts) (finalTree, finalMsg string, err error)
```
- Returns `(finalTree, finalMsg, nil)` on success. `finalMsg` can be EMPTY if a hook emptied the file.
  NO empty-message check inside `RunCommitHooks`.
- On error: `("", "", err)` — `*RescueError` or `ErrHookSweptConcurrentWork`.

### Caller (a): `generate.CommitStaged` — generate.go:427-439
```go
if deps.Hooks != nil {
    ft, fm, herr := deps.Hooks.RunCommitHooks(ctx, deps.Git, cfg, treeSHA, parentSHA, msg, false, deps.Verbose)
    if herr != nil {
        return Result{}, herr
    }
    treeSHA, msg = ft, fm // hook may have re-treed + annotated the msg
}
// ... CommitTree(ctx, treeSHA, parents, msg) at line 439
```
- **Empty-message check: NONE.** Empty `fm` → empty CommitTree.

### Caller (b): `pkg/stagecoach.runPipeline` — stagecoach.go:652-694
```go
if deps.Hooks != nil {
    ft, fm, herr := deps.Hooks.RunCommitHooks(ctx, deps.Git, cfg, treeSHA, parentSHA, msg, dryRun, deps.Verbose)
    if herr != nil {
        if dryRun {
            // FR-V8a: warn-and-print (exit 0) for RescueError under dry-run
        } else {
            return Result{}, herr // !dryRun → rescue
        }
    } else {
        treeSHA, msg = ft, fm
    }
}
// dry-run returns early (line 678). commit path: CommitTree at line 694.
```
- **Empty-message check: NONE.** Under dryRun, empty msg flows to Result.Message.

### Caller (c): `decompose.publishCommit` — message.go:230-235
```go
finalTree, finalMsg, herr := hooks.RunCommitHooks(ctx, deps.Git, deps.Config, tree, parentSHA, msg,
    hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
if herr != nil {
    return "", herr
}
newSHA, err := deps.Git.CommitTree(ctx, finalTree, parents, finalMsg)  // line 235
```
- **Empty-message check: NONE.** Empty `finalMsg` → empty commit.
- Calls `hooks.RunCommitHooks` DIRECTLY (not via the interface).

### Summary: no caller checks for emptiness
All three callers assign hook-adjusted msg directly to CommitTree. Only `EditMessage` (finalize.go:117)
guards against empty, but it runs BEFORE hooks.

## `ErrEmptyMessage` sentinel (existing, reusable)
**File**: `internal/generate/finalize.go:45`
```go
var ErrEmptyMessage = errors.New("stagecoach: empty commit message — aborted")
```
- Used at finalize.go:118 in `EditMessage`. Returns bare (non-rescue) → exit 1.
- `hooks` already imports `generate` (for `RescueError`), so it CAN reference `generate.ErrEmptyMessage`.

## `CommitHookRunner` interface (injected dep)
**File**: `internal/generate/generate.go:26-37`
```go
type CommitHookRunner interface {
    RunCommitHooks(ctx context.Context, g git.Git, cfg config.Config, snapshotTree, parentSHA, msg string,
        dryRun bool, verbose *ui.Verbose) (finalTree, finalMsg string, err error)
    RunPostCommit(ctx context.Context, g git.Git, cfg config.Config, dryRun bool, verbose *ui.Verbose) error
}
```
- Interface takes `(dryRun bool, verbose *ui.Verbose)` INLINED (not `hooks.HookOpts`) to break the
  `generate ↔ hooks` import cycle.
- `DefaultRunner` adapter at `internal/hooks/adapter.go:31` bridges the interface to the package function.
- `decompose.publishCommit` does NOT use this interface — it calls `hooks.RunCommitHooks` directly.

## Data flow diagram
```
                     ┌─────────────────────────────────────────┐
                     │         internal/hooks/runner.go        │
                     │  RunCommitHooks(ctx,g,cfg,tree,par,msg, │
                     │       opts HookOpts) (ft, fm, err)      │
                     │  1. pre-commit (scoped throwaway index) │
                     │  2. write msg → temp file (LINE 103)    │ ← Issue 2: no \n
                     │  3. runPrepareCommitMsg (LINE 195)      │ ← Issue 1: argc=2
                     │  4. runCommitMsg                        │
                     │  5. read-back + stripCommentLines       │
                     │  return (finalTree, finalMsg, nil)      │ ← Issue 4: no empty check
                     └──────────┬──────────────┬───────────────┘
                                │              │
                ┌───────────────┘              └────────────────┐
                │ (via DefaultRunner adapter)    (direct call)   │
                ▼                                                ▼
  ┌─────────────────────────────┐               ┌──────────────────────────────┐
  │ CommitHookRunner interface  │               │ hooks.RunCommitHooks(direct) │
  │  generate.go:26             │               │  decompose.publishCommit     │
  └──────────┬──────────────────┘               │  message.go:230              │
             │                                   │  → CommitTree(finalMsg)      │
             ├── CommitStaged generate.go:427    │     message.go:235           │
             │   → CommitTree(msg)  :439         │  NO empty check              │
             │   NO empty check                  └──────────────────────────────┘
             │
             └── runPipeline stagecoach.go:653
                 → CommitTree(msg)  :694
                 NO empty check
```

## Existing test gaps
- NO test asserts argc of prepare-commit-msg. → Issue 1 undetected.
- NO test asserts trailing newline or line-boundary on append-style hooks. → Issue 2 undetected.
- NO test covers a hook that empties the message file. → Issue 4 undetected.

## Fix placement analysis (Issue 4 — empty-message guard)

### Option A: single point inside `RunCommitHooks` (after line 131)
- Check `strings.TrimSpace(finalMsg) == ""` → return `generate.ErrEmptyMessage`.
- Covers all 3 callers automatically.
- **Dry-run interaction**: runPipeline's dry-run path checks `errors.As(herr, &re)` for `*RescueError`.
  `ErrEmptyMessage` is NOT `*RescueError` → falls to `return Result{}, herr` → exit non-zero. This
  means a dry-run with an emptied message aborts rather than warn-and-prints.

### Option B: check in each caller (3 sites)
- `CommitStaged` (generate.go:431), `runPipeline` (stagecoach.go:672), `publishCommit` (message.go:233).
- Each checks `strings.TrimSpace(msg) == ""` after the hook block.
- runPipeline can choose warn-and-print under dryRun instead of hard error.

### PRD recommendation
PRD says "check in CommitStaged at line 431, in runPipeline after the hooks block, and in decompose
publishCommit" → Option B (3-site check). Each returns `ErrEmptyMessage` (non-rescue, exit 1).
