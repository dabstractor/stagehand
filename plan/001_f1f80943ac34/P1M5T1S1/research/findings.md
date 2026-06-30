# P1.M5.T1.S1 — Research Findings
## Property/invariant tests (idempotent index, atomic HEAD, snapshot immutability)

---

## 0. Task contract (verbatim from item_description)

Three §20.2 invariants, asserted across §18.2 failure paths:

1. **Idempotent index** — after any failure path, `git diff --cached --name-only` is identical to
   before the run (no index mutation).
2. **Atomic HEAD** — after a CAS failure, `git rev-parse HEAD` is unchanged (by Stagehand).
3. **Snapshot immutability** — `git cat-file -p <TREE_SHA>` is stable across the run regardless of
   subsequent staging.

INPUT: `CommitStaged` (P1.M3.T4.S2, COMPLETE) + stub provider (P1.M3.T4.S1, COMPLETE).
Failure paths to drive: **parse fail, timeout, CAS fail, SIGINT**. Temp git repo. Real git binary.
OUTPUT: a property/invariant test suite that runs in CI and guards the repo-safety guarantee (§18.1).
DOCS: none — test infrastructure.

---

## 1. The orchestrator under test — `internal/generate/generate.go` (P1.M3.T4.S2)

`CommitStaged(ctx, deps, cfg)` is a 10-step pipeline. The safety boundary is at step 3:

```
1. RevParseHEAD        → parentSHA, isUnborn
2. StagedDiff          → diff payload; empty → ErrNothingToCommit (PRE-snapshot, no tree)
3. WriteTree           → treeSHA   *** SNAPSHOT TAKEN — from here on, HEAD/index are frozen w.r.t. this run ***
4. buildSystemPrompt   → (once)
5. generate→parse→dedupe LOOP (bounded by cfg.MaxDuplicateRetries)
7. CommitTree          → newSHA (DANGLING — no ref moved yet)
8. UpdateRefCAS        → *** SOLE ref mutation; CAS fail → CASError, never force ***
9. DiffTree            → FR42 report
10. return Result
```

**Failure → error type → TreeSHA mapping (the contract for invariant tests):**

| failure path (§18.2)       | how to trigger via stub            | error type          | `.TreeSHA` | `.Kind` / sentinel           |
|----------------------------|------------------------------------|---------------------|------------|------------------------------|
| parse fail                 | `NewScript(["",…])`, retries=0     | `*RescueError`      | non-empty  | `errors.Is(err, ErrRescue)`  |
| timeout                    | `Manifest(SleepMS=2000)`, `cfg.Timeout=150ms` | `*RescueError` | non-empty | `errors.Is(err, ErrTimeout)` |
| SIGINT (context cancel)    | `Manifest(SleepMS=3000)` + cancel ctx mid-run | `*RescueError` | non-empty | `errors.Is(err, ErrRescue)` (`context.Canceled` branch) |
| duplicate exhaustion       | `NewScript(["feat: existing"])` w/ matching HEAD | `*RescueError` | non-empty | `errors.Is(err, ErrRescue)` |
| agent non-zero exit        | `Manifest(Exit=2, Out="")`         | `*RescueError`      | non-empty  | `errors.Is(err, ErrRescue)` |
| CAS failure (HEAD moved)   | `Manifest(SleepMS=400)` + concurrent `commit` | `*CASError` | non-empty | `errors.Is(err, ErrCASFailed)` |

> The 4 NAMED failure modes in the contract are: **parse fail, timeout, CAS fail, SIGINT**.
> duplicate-exhaustion + agent-nonzero-exit are FREE bonus scenarios (same stub seam, same invariant
> shape) that strengthen the property; include them.

Source: `internal/generate/generate.go` lines:
- snapshot point: `treeSHA, err := deps.Git.WriteTree(ctx)` + `signal.SetSnapshot(...)`.
- timeout branch: `if errors.Is(execErr, context.DeadlineExceeded) { return ..., &RescueError{Kind: ErrTimeout,...} }`.
- SIGINT branch: `if errors.Is(execErr, context.Canceled) { return ..., &RescueError{Kind: ErrRescue,...} }`.
- loop-exhaustion branch: `if !success { return ..., &RescueError{Kind: ErrRescue,...} }`.
- CAS branch: `if errors.Is(err, git.ErrCASFailed) { ...; return ..., &CASError{...} }`.

---

## 2. The stub provider — `internal/stubtest` (P1.M3.T4.S1) + `cmd/stubagent`

- `stubtest.Build(t)` — compiles `cmd/stubagent` ONCE per test process (cached, `sync.Once`); skips if
  no `go` on PATH. Returns the binary path.
- `stubtest.Manifest(bin, stubtest.Options{...})` — returns a `provider.Manifest` with the stub wired.
  Knobs (→ `STAGEHAND_STUB_*` env):
  - `Out string` — single-response stdout (script=="" mode).
  - `Exit int` — non-zero exit (failed-agent sim).
  - `SleepMS int` — `time.Sleep` AFTER draining stdin (slow/timeout/async sim).
  - `Stderr string` — stderr text.
  - `Script`/`Counter` — call-varying mode via `NewScript(t, bin, []string{...})`.
- `stubtest.NewScript(t, bin, []string{...})` — successive stub invocations get successive responses;
  **blank entries are significant** (empty stdout → `ParseOutput` ok=false → orchestrator retries).
  After the list is exhausted the last response repeats.

`cmd/stubagent/main.go` drains stdin FIRST (deadlock guard), then sleeps, then writes stdout, then
`os.Exit(Exit)`. STDLIB ONLY. It is invoked through `provider.Execute` exactly like a real agent.

---

## 3. Reusable test helpers — `internal/generate/generate_test.go` is `package generate`

**CRITICAL — no helper duplication needed.** `generate_test.go` is an INTERNAL test
(`package generate`, NOT `package generate_test`). A new file
`internal/generate/invariants_test.go` declared `package generate` can call these directly:

| helper (already in generate_test.go) | signature / purpose |
|--------------------------------------|---------------------|
| `initRepo(t, dir)`                   | `git init` + repo-local `user.name`/`user.email` (no env pollution). |
| `writeFile(t, dir, name, body)`      | writes a worktree file. |
| `stageFile(t, dir, name)`            | `git add <name>`. |
| `commitRaw(t, dir, msg)`             | `git commit --allow-empty -m <msg>` (empty commit — does NOT touch the index). |
| `headSHA(t, dir)`                    | `git rev-parse HEAD` (trimmed). |
| `gitOut(t, dir, args...)`            | raw git, trimmed stdout (wraps runGit). |
| `runGit(t, dir, args...)`            | `git -C dir args...`, trimmed stdout, `t.Fatal` on error. |
| `shaRe`                              | `regexp` for hex SHA. |

→ DO NOT re-declare any of these in the new file (compile error: redeclared). Reuse them.

---

## 4. Existing invariant coverage (audit) — what already exists vs. what's MISSING

`internal/generate/generate_test.go` already has:

| test                                  | invariant touched | failure mode |
|---------------------------------------|-------------------|--------------|
| `TestCommitStaged_ParseFailRescue`    | idempotent index + HEAD unchanged | parse fail |
| `TestCommitStaged_IdempotentIndexOnFailure` | idempotent index (FULL byte-diff) | parse fail |
| `TestCommitStaged_CASFailure`         | atomic HEAD (HEAD == concurrent)  | CAS fail |
| `TestCommitStaged_Timeout`            | HEAD unchanged                    | timeout |

`internal/signal/signal_integration_test.go` has `TestSignalIntegration_SigintPostSnapshot` — the
FULL real-binary SIGINT→os.Exit(3) path, asserting HEAD + index unchanged + Tree ID is a real tree.
(CLI level; `//go:build !windows`; `testing.Short()` skip.)

**GAPS this task fills (the DELIVERABLE):**

1. **Snapshot immutability is tested NOWHERE.** This is the headline new invariant: stage extra
   content AFTER the run and re-`git cat-file -p <treeSHA>` → must be byte-identical. Proves the tree
   is a content-addressed immutable object that survives index mutations. (git's default `gc.auto`
   threshold ≈6700 loose objects is never hit in a test → the dangling tree persists.)
2. **No single SYSTEMATIC suite** covering all 3 invariants × all failure modes in one place. The
   existing tests scatter partial checks across separate functions. A table-driven "property" suite
   (`TestInvariants` with subtests) makes the §18.1 guarantee a first-class, auditable guard.
3. **SIGINT at the CommitStaged (orchestrator) level is not tested.** The integration test covers
   the CLI/os.Exit path; nothing drives a context-cancelled `CommitStaged` directly and asserts the
   repo invariants. (Context cancel → `context.Canceled` → `RescueError{Kind:ErrRescue}`.)

> NOTE on relationship to existing tests: this task ADDS a dedicated invariant suite; it does NOT
> delete the existing per-scenario tests (they also pin error types/`errors.Is` shapes, which the
> invariant suite treats as secondary). The two coexist. No production code changes.

---

## 5. SIGINT simulation at the orchestrator level — WHY context-cancel is correct

The signal package (`internal/signal/signal.go`) is **opt-in** (Install). In a test that calls
`CommitStaged` directly WITHOUT installing a handler, `signal.Active()` is nil and ALL signal wrappers
(`SetSnapshot`, `RestoreDefault`, `RegisterChild`, …) are **nil-safe no-ops** (verified: each checks
`active.Load()`). So calling CommitStaged directly in a test is safe — no global-state interference.

On a real SIGINT, `signal.handle` calls `h.cancel()` → the signal-aware ctx is cancelled →
`provider.Execute` returns `context.Canceled`. CommitStaged maps that to
`RescueError{Kind: ErrRescue, TreeSHA: <non-empty>}`.

→ **The faithful orchestrator-level SIGINT simulation = pass a cancellable context, cancel it during
   generation (stub sleeping).** Same error path, same repo invariants, no real signal/exit needed.
   (The CLI/os.Exit path stays covered by the existing `signal_integration_test.go`.)

Caveat handled: cancel AFTER the snapshot (step 3) so `.TreeSHA` is non-empty. WriteTree is
microseconds; we wait ~150ms before cancelling while the stub sleeps ~3000ms → snapshot is long done.

---

## 6. CAS scenario — the one async + externally-mutating case

Atomic-HEAD for CAS is SPECIAL: the test itself moves HEAD mid-run (to simulate concurrency), so
"HEAD unchanged" is read as "HEAD == the externally-moved commit" (proving the orchestrator did NOT
land/force). Pattern (proven by `TestCommitStaged_CASFailure`):

1. `parent := headSHA(repo)`.
2. stub `Manifest(SleepMS=400)`.
3. spawn `CommitStaged` in a goroutine.
4. `time.Sleep(150ms)` (let snapshot + generation start).
5. `commitRaw(repo, "concurrent commit")` (empty commit → HEAD moves, INDEX UNCHANGED).
6. `concurrent := headSHA(repo)`.
7. wait on `done`, get `*CASError`; assert `errors.Is(err, ErrCASFailed)`, `ce.TreeSHA != ""`.
8. atomic HEAD: `headSHA(repo) == concurrent` (NOT parent, NOT orchestrator's).
9. idempotent index: STILL HOLDS (empty commit doesn't touch the index) → before==after.
10. snapshot immutability: `cat-file -p ce.TreeSHA` stable across a subsequent `stageFile`.

---

## 7. Validation commands (verified against Makefile + go module)

```bash
# Targeted (fast feedback loop) — the new suite:
go test -race ./internal/generate/ -run TestInvariants -v

# Full generate package (ensure no regression in the existing tests that share helpers):
go test -race ./internal/generate/ -v

# Whole tree (the CI gate; safe — no production code changes):
make test            # == go test -race ./...
go vet ./internal/generate/
gofmt -l internal/generate/   # must be empty

# Coverage (PRD §20.3 ≥85% on internal/generate — this suite ADDS coverage, helps the gate):
make coverage       # go test -coverprofile=coverage.out ./... + go tool cover -func
```

`go.mod`: module `github.com/dustin/stagehand`, go 1.22. NO new deps (stdlib + existing internal
imports: `context`, `errors`, `strings`, `testing`, `time`, `config`, `git`, `stubtest`). The new
file imports only what `generate_test.go` already imports.

---

## 8. Confidence & risks

**Confidence: 9.5/10** for one-pass success. Rationale:
- Every seam exists and is COMPLETE: CommitStaged (T4.S2), stub (T4.S1), git binary, the reusable
  helpers in `generate_test.go`.
- The three invariants are directly observable via raw git commands the existing tests already use
  (`rev-parse HEAD`, `diff --cached [--name-only]`, `cat-file -p/-t`).
- The only genuinely NEW assertion (snapshot immutability) is a 2-line `cat-file -p` before/after a
  `stageFile` — git content-addressing makes this trivially true; the test is a guard, not a discovery.

**Risks (low):**
- **Helper name collision.** If the implementer re-declares `initRepo`/`headSHA`/etc. in the new
  file → compile error. Mitigated: PRP explicitly says REUSE them (§3 above).
- **Async flakiness (CAS/SIGINT).** Timing-based. Mitigated: generous stub sleeps (400ms/3000ms) and
  short pre-trigger waits (150ms); mirrors the already-green `TestCommitStaged_CASFailure`. Use
  `testing.Short()` skip for the slow SIGINT/timeout/CAS subtests to keep `go test -short` fast.
- **`t.Parallel()` + signal global state.** DO NOT call `t.Parallel()` — the signal package uses a
  process-global singleton. Existing generate tests run serially; match them.
