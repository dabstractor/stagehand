# Research: E2E Contention Scenarios (P1.M1.T2.S3)

> **Purpose:** Pin the exact test design for `internal/e2e/lock_scenarios_test.go` — the cross-process
> regression net for PRD §18.5 (FR52 per-repo run lock). Built on the e2e harness primitives
> (`newRepo`, `runStagecoach`, `waitForMarker`, `writeStubConfig`, `stubEnv`) and the ALREADY-LANDED
> S2 contention wiring. Verification environment: git 2.54.0, go1.26.4 linux/amd64, 2026-07-03.

---

## 1. S2 (the contention wiring) has ALREADY LANDED on disk — treat as complete

The plan_status says S2 is "Implementing", but the live code shows it landed:

```
internal/cmd/default_action.go:
  53:  g := git.New(repoDir)
  59:  locker, lockErr := lock.Acquire(repoDir)
  63:      return handleLockContention(stderr, held, g, ctx)
  67:  defer locker.Release()
  90:  if shouldDecompose(cfg, flagDryRun, flagNoAutoStage) {   // AFTER the lock
 235:  func handleLockContention(stderr io.Writer, heldErr *lock.HeldError, g git.Git, ctx context.Context) error {
```

**Implication:** the lock is acquired in `runDefault` (the default action's RunE) BEFORE `shouldDecompose`
and BEFORE the dry-run success path (line 197). Read-only subcommands (`providers`, `config`, `models`)
have their OWN `RunE` and never reach `runDefault` — they bypass the lock structurally (no code needed).
So all 5 scenarios have a real, wired target to exercise.

## 2. The EXACT contention messages (read from landed `handleLockContention`) — the assertion strings

**No-op fast path (exit 0):** writes to stderr, returns `exitcode.New(exitcode.Success, nil)`:
```
nothing to do — an in-progress run already covers your staged changes.
```
→ assert `strings.Contains(res.Stderr, "nothing to do")` + `res.ExitCode == 0`.

**Busy (exit 5):** writes to stderr, returns `exitcode.New(exitcode.Busy, nil)`:
```
stagecoach: another stagecoach run is already in progress on <repo> (pid <N> on <host>). Your newly-staged changes will remain staged — re-run stagecoach after it finishes. Lock: <path>.
```
→ assert `strings.Contains(res.Stderr, "already in progress")` + `res.ExitCode == 5`. The `<repo>` is the
canonical repo path, so also assert `strings.Contains(res.Stderr, repo)`.

`exitcode.Busy == 5` (internal/exitcode/exitcode.go:28). `exitcode.For()`'s `*ExitError` short-circuit
resolves the silent `New(code, nil)` returns to the right exit code at the binary level.

## 3. THE CRUX — Busy vs no-op is determined by the contender's INDEX, not the holder's existence

`handleLockContention` logic (verified at default_action.go:241):
```go
if snap := heldErr.Contents.Snapshot; snap != "" {
    contenderTree, werr := g.WriteTree(ctx)   // index-read-only probe
    if werr == nil && contenderTree == snap {
        // NO-OP: exit 0
    }
    // else fall through → Busy
}
// BUSY: exit 5
```

So once the holder has published `snapshot=` (it has, by marker time — see §4), the contender's outcome
depends ONLY on whether the contender's `write-tree` equals the holder's snapshot:

| Scenario | Contender's index vs holder's snapshot | Outcome |
|---|---|---|
| **A (Busy)** / **E (dry-run Busy)** | contender staged an EXTRA file → tree DIFFERS | **exit 5 (Busy)** |
| **B (no-op)** | contender staged NOTHING new → tree MATCHES | **exit 0 (no-op)** |

**This is the #1 design decision in the PRP.** If the implementer stages the SAME file for both #1 and #2
in scenario A (as the contract's loose "stage a change" prose might suggest), #2's tree will MATCH #1's
snapshot and #2 will exit 0 (no-op) — FAILING the Busy assertion. The Busy scenarios (A, E) MUST have #2
stage a genuine second batch (an extra file) so its tree differs from #1's frozen snapshot. The no-op
scenario (B) MUST have #2 stage nothing new.

## 4. The marker-timing invariant — why waitForMarker makes A/B/E deterministic

The stub agent (cmd/stubagent/main.go) sequence:
1. **drain stdin** (the prompt payload, sent by stagecoach after WriteTree)
2. **write STAGECOACH_STUB_MARKER** (readiness)
3. sleep `STAGECOACH_STUB_SLEEP_MS`
4. write `STAGECOACH_STUB_OUT`, exit

Stagecoach's single-commit path (generate.CommitStaged) BEFORE invoking the stub does:
1. `WriteTree` → treeSHA
2. `signal.SetSnapshot(treeSHA, ...)`
3. `lock.SetSnapshot(treeSHA)` ← **publishes the snapshot to the lock file**
4. build prompt → `provider.Execute` → stub drains stdin → **writes the readiness marker**

Therefore: **when `waitForMarker` returns, #1 has BOTH acquired the lock AND published its snapshot.**
This is the synchronization point. Launching #2 after `waitForMarker(readiness)` guarantees #2 sees a
non-empty `snapshot=` line — so the no-op/busy split is decided purely by #2's index (§3), deterministically.

(This is why the test does NOT need a fragile timing race between acquire and snapshot. The marker IS the
"snapshot published" signal.)

## 5. The blocking pattern — reuse the stub's timed sleep (NO new shell script needed)

The contract mentions "a shell script or Go binary that sleeps on a marker file." But the EXISTING stub
agent already supports `STAGECOACH_STUB_SLEEP_MS` (a timed sleep after the marker). This is the exact
pattern `scenarios_test.go`'s S3 (`ConcurrentFile_Excluded`) and S7 (`CASAbort_HeadMoved`) already use:

```go
resCh := make(chan e2eResult, 1)
go func() { resCh <- runStagecoach(t, bin, repo, cfg, env, "--provider", "stub") }()
waitForMarker(t, readiness, 10*time.Second)
// ... launch #2 / mutate state ...
res := <-resCh   // #1 finishes after its sleep
```

Use `STAGECOACH_STUB_SLEEP_MS=3000` (3s) for #1 — long enough that #2 runs during the sleep, short enough
to keep the suite fast. The goroutine + `resCh` lets the test assert #2's behavior mid-flight, then drain
#1's result. **No new shell script or gate-marker binary is required** — reusing the timed stub is simpler
and battle-tested. (The contract's "write the marker to release #1" is one option; the timed-sleep option
is equally valid and avoids a second helper.)

## 6. CLI subcommands confirmed (scenario D — read-only bypass)

Verified against internal/cmd:
- `providers list` → `runProvidersList` (providers.go:51). Group `providers` has NO RunE.
- `config path` → `runConfigPath` (config.go:86). `config init`/`path` skip config load.
- `models [<provider>]` → `runModels` (models.go:42); `models --help` is cobra help (exit 0).

All three have their OWN `RunE` and never reach `runDefault`, so they never acquire the lock. Against a
repo where #1 holds the lock, they must NOT exit 5. Assert `res.ExitCode != 5` and stderr lacks
"already in progress". (`runStagecoach` already prepends `--config cfg --no-color`, so just pass the
subcommand args.)

## 7. Dry-run goes through runDefault → acquires the lock (scenario E)

- `--dry-run` flag registered at root.go:159 (`pf.BoolVar(&flagDryRun, "dry-run", …)`).
- `runDefault` is the default action's RunE; the lock is acquired at line 59, BEFORE the dry-run+edit
  check (line 71) and the dry-run success path (line 197, `printDryRunMessage` + `return nil` exit 0).
- So a dry-run contender that hits a held lock returns at line 63 (`handleLockContention`) → Busy (5),
  BEFORE ever printing the dry-run message.
- Dry-run's normal "no commit created" line is printed to STDERR (default_action.go:204). So scenario E
  asserts the dry-run does NOT contain "no commit created" (it never got past the lock) AND exits 5.

## 8. The 5 scenarios — exact step design

All use: `cfg := writeStubConfig(t, stub, "")` (shared); per-subtest `repo := newRepo(t)` + `seedCommit`.
Both #1 and #2 inherit `os.Environ()` (same XDG lock dir) + same repo → same lock file → contention.

**A. BusyRefusal_GenuineSecondBatch** — #1 blocks (sleep 3s, snapshot=tree(a.txt)); after marker, stage
b.txt for #2; #2 → exit 5, stderr has "already in progress" + repo; #1 → exit 0, commitCount==2, HEAD="feat: a".

**B. NoOpFastPath_AccidentalDoubleRun** — #1 blocks (snapshot=tree(a.txt)); after marker, #2 stages
NOTHING new; #2 → exit 0, stderr has "nothing to do"; commitCount==2 (only #1 committed).

**C. NoStaleLock_AfterExit** — #1 runs to completion (no sleep); stage b.txt; #2 → exit 0 (NOT 5),
commitCount==3. Proves flock auto-released on #1's exit.

**D. ReadOnlyBypass** — #1 blocks (sleep 5s); run `providers list` / `config path` / `models --help`
against the locked repo; each → exit != 5, no "already in progress"; drain #1.

**E. DryRunAcquiresLock** — #1 blocks (snapshot=tree(a.txt)); after marker, stage b.txt; #2 `--dry-run`
→ exit 5 (Busy), stderr has "already in progress", NO "no commit created" (never passed the lock).

## 9. Scope boundaries (do NOT do)

- Do NOT modify ANY production code — S2's wiring is landed; S3 is TEST-ONLY (`internal/e2e/`).
- Do NOT touch `internal/lock/*`, `internal/exitcode/*`, `internal/cmd/*`, `internal/generate/*`,
  `internal/decompose/*`, the stub agent, the harness (`harness_test.go`).
- Do NOT create a new blocking-stub binary/script — reuse `STAGECOACH_STUB_SLEEP_MS`.
- Do NOT append to the large `scenarios_test.go` — create a NEW `lock_scenarios_test.go` (matches the
  `hook_scenarios_test.go` separation pattern; same `//go:build e2e`, same `package e2e`).
- Do NOT edit `PRD.md`, `tasks.json`, `prd_snapshot.md`, docs, or `plan/*`.

## 10. Decisions log

| # | Point | Decision | Why |
|---|---|---|---|
| D1 | New file vs append to scenarios_test.go | NEW `lock_scenarios_test.go` | Matches hook_scenarios_test.go separation; keeps the contention suite isolated; same package reuses all helpers. |
| D2 | Blocking mechanism | Reuse `STAGECOACH_STUB_SLEEP_MS` timed stub | Battle-tested (S3/S7); no second helper; deterministic via marker. The contract allows it ("or a Go binary"). |
| D3 | How to force Busy (not no-op) in A/E | Contender stages an EXTRA file (tree differs) | §18.5: "genuine second batch is staged (diff non-empty)" → Busy. Same-index → no-op. THE crux. |
| D4 | How to force no-op in B | Contender stages NOTHING new (tree matches snapshot) | §18.5: "path-diff is empty" → exit 0. |
| D5 | Synchronization point | `waitForMarker(readiness)` | Marker written by stub AFTER stdin drain ⟹ AFTER WriteTree+SetSnapshot. Guarantees snapshot is published before #2 runs. |
| D6 | Read-only assertion | `ExitCode != 5` + no "already in progress" | Bypass is structural; just assert they don't hit the lock. `config path`/`providers list`/`models --help` all exit 0. |
| D7 | Dry-run assertion | exit 5 + "already in progress" + NO "no commit created" | Proves dry-run acquires the lock (else it'd print the preview + exit 0). Extra file forces Busy not no-op. |
| D8 | XDG isolation | Rely on repo-key uniqueness (newRepo = unique t.TempDir) | Each repo hashes to a unique lock file → no cross-scenario contention. Both subprocesses inherit the same os.Environ → same lock dir → within-scenario contention works. No explicit XDG needed (matches existing scenarios_test.go). |
