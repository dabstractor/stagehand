# Research Notes — P1.M6.T2.S1 (signal.go)

## Governing contract (from task definition)
- `installSignalHandler(ctxCancel func(), rescueFn func())` — signal.Notify on SIGINT,SIGTERM.
  On receipt: call ctxCancel (executor kills process group via Setpgid + grace SIGKILL),
  then if TREE!="" run rescueFn, then exit with Rescue code (ui.ExitRescue == 3).
- `restoreSignalHandler()` — signal.Reset(SIGINT,SIGTERM) back to default.
  CommitStaged calls this immediately BEFORE UpdateRefCAS.
- OUTPUT: signal integration wired into CommitStaged (P1.M6.T1.S1).

## Key findings / gotchas

### 1. Ordering reconciliation (CRITICAL)
- reference_impl.md §1 places `trap - INT TERM` as the LAST line (after update-ref + diff-tree).
- BUT work-item contract + PRD §18.4 step 3 + rescue.go all say: restore IMMEDIATELY BEFORE
  UpdateRefCAS. Reason: a SIGINT during the atomic CAS would otherwise fire rescue ("commit
  failed"), but the CAS either fully landed (rescue is a lie) or fully failed (HEAD unchanged →
  CAS-failure path prints its own message, exits 1). PRD §18.4 §7 lists this as NEW PRD work that
  deliberately improves on the reference (which "just trap'd").
- decisions.md §3 pseudo-code lists the restore comment AFTER the UpdateRefCAS line — the line
  POSITION is misleading; the comment TEXT ("restore BEFORE update-ref") is correct.
  => FOLLOW: restoreSignalHandler() immediately before git.UpdateRefCAS(...). Document this.

### 2. TREE gating lives in the rescueFn closure
- Signature is fixed: (ctxCancel func(), rescueFn func()). The "if TREE!=" " guard therefore
  lives inside the rescueFn closure (captures TREE/PARENT/*ui.Output), not in the handler.
- Handler is only armed AFTER WriteTree (post-snapshot) => TREE is always "" when it fires; the
  guard is defensive. candidate="" on the SIGINT path (rescue.go: "the SIGINT/SIGTERM path never
  has one").

### 3. Exit code semantics
- Handler fires only post-snapshot => exits ui.ExitRescue (3).
- Pre-snapshot SIGINT: NO custom handler installed => Go default => exit 130. (This is the
  "else just exit" branch of PRD §18.4 step 2 — handled by NOT arming a handler before snapshot.)

### 4. ctxCancel seam = executor context cancel func
- Executor.Run(ctx, ...) (M2.T4.S1) observes ctx.Done(); on cancel it SIGTERM+SIGKILLs the whole
  process group (Setpgid). The SAME cancel func is passed to installSignalHandler as ctxCancel.
  No stored context needed — ctx parameter is the seam (see executor.go doc comment).

### 5. Testability: os.Exit in a signal goroutine
- Rescue (rescue.go) deliberately does NOT os.Exit (caller's job). But the signal handler MUST
  os.Exit from its goroutine. To make this unit-testable:
  (a) Split a pure core `handleSignal(ctxCancel, rescueFn func()) int` (no os.Exit) that unit
      tests assert against directly (ctxCancel called, rescueFn called, returns 3).
  (b) Injectable exit seam `var exit = os.Exit` so the goroutine does `exit(handleSignal(...))`.
- For the REAL signal-delivery + exit-code assertions (MOCKING: "send SIGINT to the process →
  rescue printed + exit code"), use the canonical Go SUBPROCESS re-exec pattern (same real-process
  philosophy as executor_test.go): child mode via env var (e.g. STAGEHAND_SIGNAL_TEST) installs the
  handler and blocks; parent execs `os.Args[0] -test.run=...` with the env, sends SIGINT, asserts
  exit code 3 + stderr contains the rescue block + tree.
- Scenario 2 ("after restore, signal during mocked update-ref does NOT trigger rescue"):
  child installs handler, calls restoreSignalHandler(), does a mocked update-ref (prints a sentinel),
  exits 0; parent sends SIGINT; assert stderr has NO rescue block and exit code != 3.
- Stdlib only (no testify), `package generate` white-box — matches rescue_test.go / dedupe_test.go.

### 6. Scope boundary (do NOT overstep)
- CommitStaged (P1.M6.T1.S1) does NOT exist yet (internal/generate/ has only dedupe.go, rescue.go).
  T2.S1's deps are Rescue (T1.S3) + Executor (M2.T4.S1), NOT T1.S1.
- => signal.go must be SELF-CONTAINED and fully testable on its own. Wiring call-sites into
  generate.go are applied ONLY IF CommitStaged already exists at impl time; otherwise provide the
  exact 2-call-site snippet for T1.S1. Do NOT create a stub CommitStaged.

### 7. Docs
- Task has NO DOCS line (Mode A). Defers to Mode B (changeset-level doc sync in M8 per
  plan_overview.md). No per-item doc for internal/generate/signal.go.

## Verified validation commands (all pass on current tree)
- go build ./...            (exit 0)
- go vet ./internal/generate/
- gofmt -l internal/generate/   (empty)
- go test ./internal/generate/  (ok)
