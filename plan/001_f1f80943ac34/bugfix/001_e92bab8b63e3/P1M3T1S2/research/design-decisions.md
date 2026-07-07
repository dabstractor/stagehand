# P1.M3.T1.S2 — Replace dry-run single-pass with the bounded dedupe/retry loop (Issue 2)

Read `architecture/seam_dryrun.md` (the authoritative recon) and `architecture/decisions.md` **D3**
first — they are the binding design. This note records ONLY the concrete, current-state facts an
implementer needs (the recon's line numbers are slightly stale because **S1 / Issue 6 already
landed**). Cross-check against the live source.

## 0. Current state — S1 (Issue 6) is ALREADY COMPLETE

Do NOT re-do S1. As of this subtask, in `pkg/stagecoach/stagecoach.go` `runPipeline`:

- `WriteTree` is **unconditional** (the `if !dryRun` gate is already removed) — `treeSHA` is now
  ALWAYS set when execution reaches the dry-run block.
- `signal.SetSnapshot(treeSHA, parentSHA, "")` is called for BOTH paths (rescue is armed in dry-run).
- The dry-run block is STILL the single-pass short-circuit (that is what S2 replaces).

So S2 starts from: snapshot taken + rescue armed for dry-run, but dry-run still runs ONE attempt with
no dedupe / no retry / no `*RescueError`. S2 closes that gap.

## 1. The exact block to DELETE (current dry-run short-circuit)

In `runPipeline`, immediately after `model` resolution (`model := cfg.Model; if model == "" { model =
*resolved.DefaultModel }`) and before the commit-path loop, there is:

```go
// ---- DryRun: single pass, no commit. ----
if dryRun {
    payload := prompt.BuildUserPayload(diff, nil)
    spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
    if rerr != nil { return Result{}, fmt.Errorf("render: %w", rerr) }
    out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
    if execErr != nil {
        if errors.Is(execErr, context.DeadlineExceeded) { return Result{}, ErrTimeout } // bare sentinel
        return Result{}, fmt.Errorf("generate: %w", execErr)
    }
    msg, ok, _ := provider.ParseOutput(out, deps.Manifest)
    if !ok { return Result{}, errors.New("dry run: model produced no valid message") } // no FR29 retry
    return Result{CommitSHA: "", Subject: generate.ExtractSubject(msg), Message: msg,
        Provider: deps.Manifest.Name, Model: model}, nil
}
```

**Delete this entire block.** It is the third, degraded implementation of the loop. Everything it
needs (`diff`, `recent`, `sysPrompt`, `resolved`, `model`, `treeSHA`, `parentSHA`, `isUnborn`) is
already in scope for the loop that immediately follows it.

## 2. The loop that stays (already a faithful mirror of generate.CommitStaged)

The commit-path loop right after the deleted block is **already identical** to
`generate.CommitStaged`'s loop (rejected-subject list, `parseFail` + `retryInstr` preamble,
`DeadlineExceeded`/`Canceled` → `*RescueError`, non-zero-exit → `lastCause` fall-through,
`IsDuplicate` → append+continue, `MaxDuplicateRetries+1` attempts, exhaustion → `*RescueError`).
It uses `treeSHA`/`parentSHA`/`candidate` — all in scope. **It needs no body change**; it just needs
to run for BOTH `dryRun` and `!dryRun` (today it only runs for `!dryRun` because the dry-run
short-circuit returns first).

Note `retryInstr := *resolved.RetryInstruction` is declared just above this loop (keep it there).

## 3. THE change — add a dry-run success early-return AFTER the loop

After the `if !success { return *RescueError{...} }` block, INSERT (before the existing
`CommitTree` tail):

```go
// Dry-run: full pipeline ran (snapshot + dedupe + retry); skip ONLY commit-tree/update-ref.
// FR49: "full … pipeline, do not create the commit or move HEAD. Exit 0."
if dryRun {
    signal.ClearSnapshot() // disarm — no rescue on dry-run success (belt-and-suspenders)
    return Result{
        CommitSHA: "",
        Subject:   generate.ExtractSubject(msg),
        Message:   msg,
        Provider:  deps.Manifest.Name,
        Model:     model,
    }, nil
}
```

The existing `CommitTree → RestoreDefault → UpdateRefCAS → CASError → ClearSnapshot → DiffTree →
return` tail then runs ONLY for `!dryRun` (unchanged). This collapses the three near-duplicate code
paths into one loop with two tails — exactly what D3 item 2/3 and seam_dryrun.md "Start Here" prescribe.

## 4. The one locked-in test S2 MUST update (not optional — the suite would be RED otherwise)

`pkg/stagecoach/stagecoach_test.go`, `TestGenerateCommit_Timeout` / subtest `"dryrun"`, currently:

```go
if !errors.Is(err, ErrTimeout) { ... }            // STILL passes (*RescueError{Kind:ErrTimeout} satisfies Is)
var re *RescueError
if errors.As(err, &re) {                            // NOW FAILS — dry-run returns *RescueError after S2
    t.Error("DryRun timeout should return bare ErrTimeout, not *RescueError")
}
```

After S2, dry-run timeout returns `&generate.RescueError{Kind: ErrTimeout, TreeSHA: treeSHA, ...}`.
Flip the second assertion to mirror the `"commit_path"` subtest:

```go
var re *RescueError
if !errors.As(err, &re) { t.Fatalf("dryrun: error type = %T, want *RescueError", err) }
if !errors.Is(err, ErrTimeout) { t.Errorf("dryrun: errors.Is(err, ErrTimeout) = false") }
if re.TreeSHA == "" { t.Error("dryrun: RescueError.TreeSHA empty, want non-empty (snapshot taken)") }
```

**S2 owns this single assertion flip** (it is the test S2's own behavior change invalidates; leaving
it red between S2 and S3 would be a broken tree). S3 (the next subtask) owns the NET-NEW coverage
(dry-run dup-retry, dry-run parse-retry, dry-run exhaustion rescue, dry-run snapshot-exists) — S2
does not add those.

## 5. Signal disarm semantics — why `ClearSnapshot()` (not `RestoreDefault()`) on dry-run success

`internal/signal/signal.go` `handle(sig)`: reads `snapTree` under the lock; `if tree != ""` →
rescue-print + exit 3; else → plain signal-exit (130/143). So:

- `SetSnapshot` (S1) armed rescue for dry-run. On dry-run **success** we must **disarm** so a stray
  signal after `GenerateCommit` returns does not print a rescue block.
- `ClearSnapshot()` sets `snapTree=""` → disarms → no rescue. Idempotent, nil-safe. This is the
  correct call.
- `RestoreDefault()` is NOT needed in dry-run: its job is to neuter the handler for the
  `update-ref` window (§18.4 step 3), and dry-run never does `update-ref`. Dry-run success only
  needs `ClearSnapshot()`.

(Dry-run timeout/exhaustion returns `*RescueError` and does NOT disarm — by design; the CLI's
`handleGenError` prints the §18.3 rescue block from the `*RescueError`, and the armed handler is
consistent with that. The snapshot is left dangling in dry-run failure too, matching the commit
path's failure semantics. Out of scope to change.)

## 6. Primitives already imported in stagecoach.go (no new import, no new dep)

`generate.ExtractSubject`, `generate.IsDuplicate`, `generate.RescueError`, `generate.ErrTimeout`,
`generate.ErrRescue`, `prompt.BuildUserPayload`, `provider.Execute`, `provider.ParseOutput`,
`signal.SetSnapshot`/`SetCandidate`/`ClearSnapshot`/`RestoreDefault` — ALL already imported and used
by the existing commit-path loop. S2 deletes code and adds ~10 lines; the import block is UNCHANGED.
`go.mod`/`go.sum` UNCHANGED. `go vet`/`gofmt` clean expected.

## 7. Scope boundary — what S2 does NOT do (owned elsewhere)

- S1 (Issue 6, snapshot unconditionally): **DONE** — do not touch.
- S3: the net-new dry-run tests (dup-retry, parse-retry FR29, exhaustion rescue, snapshot-exists).
  S2 only flips the one locked-in timeout assertion it invalidates (§4).
- Issue 3 (missing-provider pre-flight): DONE (P1.M2). Issue 4 (config field application): P1.M4.
  Issue 7 (auto-stage UX): P1.M4. Issue 1/5 (config double-load): DONE (P1.M1).
- `generate.CommitStaged` is FROZEN — S2 does NOT add a no-commit flag to it (D3 rejected that;
  the dedup happens inside `runPipeline`, which already held a second copy of the loop).

## 8. FR coverage after S2

| FR | Before S2 (dry-run) | After S2 (dry-run) |
|----|---------------------|--------------------|
| FR49 full pipeline | partial (snapshot ✓ via S1, but single-pass, no dedupe) | ✓ full loop, no commit |
| FR29 parse-retry | ✗ plain error | ✓ `parseFail`+`retryInstr` |
| FR30 extract subject | ✓ (final result only) | ✓ (each attempt) |
| FR31 rejection list | ✗ `BuildUserPayload(diff, nil)` | ✓ `rejected []string` |
| FR32 dup exact-match retry | ✗ never called | ✓ `IsDuplicate` |
| FR33 bounded retries | ✗ 1 attempt | ✓ `MaxDuplicateRetries+1` |
| timeout error | bare `ErrTimeout` (no TreeSHA) | `*RescueError{Kind:ErrTimeout, TreeSHA}` |
| exhaustion error | N/A | `*RescueError{Kind:ErrRescue, TreeSHA}` |

## 9. Docs (Mode A) — minimal, two files

- `docs/cli.md:26` (`--dry-run` table row, currently "Generate and print the message; do not
  commit"): affirm it runs the FULL duplicate-check/retry pipeline. Expand the row's description or
  add a one-line note beneath the table.
- `docs/how-it-works.md`: grep found **no** dry-run mention, and the prompt/retry section (~line 102)
  documents the loop generically (not dry-run-specific) → **no edit needed**, only a verification that
  nothing implies dry-run diverges. If a divergence claim is found, correct it; else leave as-is.
