# Research Notes — P1.M3.T1.S1 (Issue 6: unconditional write-tree snapshot in dry-run)

Scope of this subtask: **remove the `if !dryRun` gate** around `WriteTree` + `signal.SetSnapshot` in
`runPipeline` so the snapshot (and signal arming) happen for **both** the commit and dry-run paths.
The dry-run single-pass branch itself stays intact (the dedupe/retry loop is **S2**; the
timeout→`*RescueError` change + new tests are **S3**).

## 1. The edit site (exact, verified)

`pkg/stagecoach/stagecoach.go`, inside `runPipeline`:

```go
244:	// Step 3 (commit path only): snapshot. DryRun skips it (no commit → no object-store write).
245:	var treeSHA string
246:	if !dryRun {
247:		treeSHA, err = deps.Git.WriteTree(ctx)
248:		if err != nil {
249:			return Result{}, err
250:		}
251:		signal.SetSnapshot(treeSHA, parentSHA, "") // arm rescue (§18.4)
252:	}
```

Target (unconditional; FR49 — dangling tree in dry-run is intentional/harmless):
```go
	// Step 3: snapshot (FR49 — dry-run runs the full diff→snapshot→… pipeline; the dangling tree in
	// dry-run is intentional and harmless — commit-tree/update-ref are skipped later for dry-run).
	treeSHA, err = deps.Git.WriteTree(ctx)
	if err != nil {
		return Result{}, err
	}
	signal.SetSnapshot(treeSHA, parentSHA, "") // arm rescue (§18.4) — both commit and dry-run paths
```
(`err` is already in scope from steps 1-2; `var treeSHA string` stays, or collapse to `treeSHA, err :=` —
either compiles since `treeSHA` is read later by the commit-path RescueError/CommitTree. No compile error:
the var is used in the commit branch of the same function, so Go's unused-var check doesn't fire.)

## 2. signal.SetSnapshot is nil-safe → dry-run arming is safe

`internal/signal/signal.go`: every package wrapper (`SetSnapshot`/`SetCandidate`/`ClearSnapshot`/
`RestoreDefault`) is a **no-op when no handler is installed** (`active.Load()` nil check). The
`pkg/stagecoach` tests **never call `signal.Install`** (confirmed: `grep signal. pkg/stagecoach/*_test.go`
is empty), so `SetSnapshot` is unobservable in the test suite. In the CLI, arming in dry-run is the
**intended** behavior (FR49: rescue arms for dry-run too). The dry-run single-pass returns before
`RestoreDefault`/`ClearSnapshot`, so the snapshot stays armed until process exit — harmless because the
process exits immediately after dry-run and a mid-generation Ctrl-C *should* fire rescue (that's the point).

## 3. Regression-safety proof — the entire existing suite stays green

`grep objectCount/count-objects` across `*_test.go` shows object-count guards live **only** in the
*missing-provider-command* tests:
- `pkg/stagecoach/stagecoach_test.go` `TestGenerateCommit_MissingProviderCommand_Issue3` (+ its `dryrun`
  subtest, lines ~350-421).
- `internal/cmd/default_action_test.go` `TestRunDefault_MissingProviderCommand_Issue3` (~620-670).

All of these use `command = "/nonexistent/path/agent"` → `buildDeps`'s `reg.IsInstalled(m)` pre-flight
(P1.M2.T1.S1) returns `false` and the function errors **before** `GenerateCommit` reaches `runPipeline`,
so `WriteTree` is never called → object count unchanged → unaffected by S1.

Other dry-run-touching tests that stay green:
- `TestGenerateCommit_DryRun` (stagecoach_test.go ~169): successful dry-run; asserts `CommitSHA==""`,
  Message/Subject, HEAD unchanged. Does **not** check object count → unaffected (HEAD still unchanged).
- `TestGenerateCommit_Timeout/dryrun` (~224): asserts `errors.Is(err, ErrTimeout)` AND `errors.As(err,&re)`
  is **false**. The dry-run single-pass still returns the bare `ErrTimeout` sentinel (S1 does not touch
  the dry-run branch) → assertion holds. (The flip to `*RescueError` is **S2/D3-item-4**, the test update
  is **S3** — out of S1 scope.)

## 4. DOCS — verify-and-skip (no edit required)

Contract DOCS clause is conditional ("if it states dry-run skips the snapshot, correct it"). Verified
neither doc makes such a claim:
- `grep -rn -i "dry.run" docs/ README.md | grep -i "snapshot|write.tree|skip"` → **empty**.
- `docs/how-it-works.md` has **no** dry-run description at all (its TOC: snapshot flow,
  stage-while-generating, safety/rescue, prompt engineering — nothing on dry-run).
- `docs/cli.md` `--dry-run` row: "Generate and print the message; do not commit." — accurate, does **not**
  imply the snapshot is skipped. Examples (`stagecoach --dry-run`) likewise only say "without committing".

→ **No doc edit is required for S1.** Any proactive dry-run narrative for how-it-works.md belongs to
**P1.M5.T1.S2** (docs sweep), not S1 — keeping S1 surgical avoids colliding with that future work item.
The PRP encodes this as a verification gate (grep must stay empty) rather than an edit task.

## 5. Binding decisions / seams referenced

- `architecture/decisions.md` **D3** item 1 (take WriteTree unconditionally + `signal.SetSnapshot` both
  paths) — the binding decision for Issue 6. Items 2-4 are explicitly S2/S3 scope.
- `architecture/seam_dryrun.md` §2(a) (the exact gated block), §"Start Here", Acceptance residual-risk #3
  (signal side-effects now armed in dry-run — addressed by nil-safety in §2 above).
