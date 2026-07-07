---
name: "P1.M1.T2.S3 — E2E contention scenarios: held lock → Busy, accidental double-run → exit 0, no stale lock, dry-run/read-only bypass"
description: |
  Add the cross-process regression net for PRD §18.5 (FR52 per-repo run lock) as a NEW
  `//go:build e2e` test file `internal/e2e/lock_scenarios_test.go` (package e2e). It exercises — against
  REAL compiled `stagecoach` subprocesses driving a REAL temp git repo — every contention behavior the
  landed S2 wiring produces: (A) held lock + genuine second batch → exit 5 (Busy) naming the holder;
  (B) accidental double-run with an identical index → exit 0 "nothing to do" no-op fast path; (C) a run
  after a normal exit → NOT Busy (flock auto-released, no stale lock); (D) read-only subcommands
  (`providers list` / `config path` / `models --help`) bypass the lock entirely; (E) `--dry-run` acquires
  the lock → exit Busy. Reuses the harness primitives (`newRepo`, `runStagecoach`, `waitForMarker`,
  `writeStubConfig`, `stubEnv`) and the stub agent's `STAGECOACH_STUB_MARKER`+`STAGECOACH_STUB_SLEEP_MS`
  blocking pattern (NO new binary). TEST-ONLY: no production code, docs, config, or API change.
---

## Goal

**Feature Goal**: Provide the PRD §20.5 "throwaway-repo harness" regression coverage for the FR52
per-repo run lock — the cross-process behaviors that the in-process unit tests (S2's
`handleLockContention` cases) structurally cannot reach: two real `stagecoach` subprocesses racing on one
repo, the no-op fast path comparing a contender's `write-tree` to a holder's published snapshot, flock
auto-release across process death, read-only-subcommand bypass, and dry-run lock acquisition. Every
contention scenario in §18.5's "Contention behavior" + "Scope" paragraphs gets a deterministic,
stub-driven e2e test that builds the binary once, creates a fresh `git init` repo, and asserts the
resulting exit code + history.

**Deliverable**: ONE new file — `internal/e2e/lock_scenarios_test.go` (`//go:build e2e`, `package e2e`) —
containing a `TestE2ELockContention` test with 5 `t.Run` subtests (A–E) plus a tiny `contains`-style
helper if not already in scope. No production code touched. No docs touched (test-only).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green
(the new file is `//go:build e2e` so it is excluded from the default suite — zero impact); `go test
-tags e2e ./internal/e2e/ -run TestE2ELockContention` passes all 5 subtests, asserting: A → contender
exit 5 + "already in progress" + repo path, holder exit 0 + commit landed; B → contender exit 0 +
"nothing to do", only holder commits; C → second run exit 0 (NOT 5); D → each read-only subcommand exit
≠ 5 + no "already in progress"; E → dry-run exit 5 + "already in progress" + NO "no commit created".

## User Persona

**Target User**: Stagecoach contributors/maintainers — this is the regression net PRD §20.5 mandates
("Every bug found in the wild becomes a scenario here"). The end-user behavior it guards is "safe to
double-invoke stagecoach in two terminals."

**Use Case**: A field report that "two stagecoach runs collided and one clobbered HEAD" or "a stale lock
hung my terminal" becomes a new subtest here. The 5 initial scenarios cover the §18.5 spec exhaustively.

**User Journey**: `go test -tags e2e ./internal/e2e/` → compiles stagecoach once → for each scenario:
`git init` a temp repo → seed → run real subprocess(es) → assert exit code + git history. Runs in CI
behind the `e2e` build tag (opt-in, like the existing `scenarios_test.go`/`hook_scenarios_test.go`).

**Pain Points Addressed**: Catches regressions in the lock wiring that unit tests miss — e.g. a future
refactor that accidentally makes a read-only subcommand reach `runDefault` (D would fail), or breaks
flock auto-release (C would fail), or mis-routes dry-run around the lock (E would fail), or breaks the
no-op/busy split (A/B would fail).

## Why

- **PRD §20.5 (End-to-end scenario harness) is the mandate:** *"The concurrency and routing invariants
  … are easy to specify, easy to regress, and — as repeated field discoveries have shown — easy to break
  silently (unit tests with stub agents cannot reach them). Maintain a throwaway-repository harness …
  Every bug found in the wild becomes a scenario here."* The lock's cross-process behavior is precisely
  such an invariant. This subtask IS that harness for FR52.
- **PRD §18.5 (FR52) is the spec under test:** the "Contention behavior" paragraph (no-op fast path →
  exit 0; genuine second batch → exit Busy naming holder; never force-break) and the "Scope" paragraph
  (commit-producing actions acquire; read-only subcommands bypass) are the exact assertions A–E encode.
  §18.5 "Mechanism" (advisory flock, auto-released on process death, no stale locks) is scenario C.
- **Defense in depth with §13.5:** the lock is the FIRST line of defense; the CAS is the second. This
  harness proves the first line holds end-to-end (two processes cannot both reach `update-ref`).
- **Closes P1.M1.T2:** S1 built the primitive, S2 wired it + unit-tested `handleLockContention`, S3
  (this) proves it at the subprocess level — the only layer that exercises real flock across real
  processes. Without S3, a silent regression in the lock wiring (e.g. acquire placed after `shouldDecompose`)
  would ship undetected.

## What

A single `//go:build e2e` test file with 5 subtests. Each builds on the harness's primitives and the
stub agent's blocking knobs. The two non-obvious design rules (front-loaded as CRITICAL gotchas):

1. **Busy vs no-op is decided by the contender's INDEX, not by the mere existence of a holder.** Once
   the holder has published `snapshot=` (it has, by `waitForMarker` time), the contender's
   `handleLockContention` runs its own `write-tree` and compares. **Busy scenarios (A, E) stage an EXTRA
   file for the contender** so its tree DIFFERS from the holder's snapshot. The **no-op scenario (B)
   stages nothing new** so its tree MATCHES. Staging the same file for both in scenario A would make #2
   exit 0 (no-op) and FAIL the Busy assertion.
2. **`waitForMarker` is the "snapshot published" synchronization point.** The stub writes the readiness
   marker AFTER draining stdin, which is AFTER stagecoach did `WriteTree` + `lock.SetSnapshot`. So when
   `waitForMarker` returns, #1 holds the lock AND has published its snapshot — making A/B/E fully
   deterministic (no timing race between acquire and snapshot).

### Success Criteria

- [ ] `internal/e2e/lock_scenarios_test.go` exists with `//go:build e2e` + `package e2e`.
- [ ] `TestE2ELockContention` has 5 `t.Run` subtests: `A_BusyRefusal_GenuineSecondBatch`,
      `B_NoOpFastPath_AccidentalDoubleRun`, `C_NoStaleLock_AfterExit`, `D_ReadOnlyBypass`,
      `E_DryRunAcquiresLock`.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean.
- [ ] `go test -race ./...` green (the e2e file is excluded by build tag — no impact on the default suite).
- [ ] `go test -tags e2e ./internal/e2e/ -run TestE2ELockContention -v` → all 5 subtests PASS.
- [ ] A: contender exit 5 + stderr "already in progress" + repo path; holder exit 0 + commitCount==2.
- [ ] B: contender exit 0 + stderr "nothing to do"; commitCount==2 (only holder committed).
- [ ] C: second run exit 0 (NOT 5); commitCount==3.
- [ ] D: `providers list` / `config path` / `models --help` each exit ≠ 5 + no "already in progress".
- [ ] E: `--dry-run` contender exit 5 + "already in progress" + stderr lacks "no commit created".
- [ ] NO production code, docs, config, harness, or stub-agent changes (test-only).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes: the exact harness primitives to call (with signatures) and the
exact stub-agent knobs to set; the EXACT landed assertion strings (no-op + Busy messages, read from
`handleLockContention`); the complete step-by-step for each of the 5 subtests (which file to stage when,
which env to pass to #1 vs #2, what to assert); the two CRITICAL design rules (Busy-vs-no-op crux +
marker-timing invariant); and the exact validation commands (`go test -tags e2e`). No inference required.

### Documentation & References

```yaml
# MUST READ — the binding contract + the authoritative seams
- file: PRD.md
  why: "§18.5 (FR52) is the spec under test — 'Contention behavior' (no-op fast path → exit 0
        'nothing to do — an in-progress run already covers your staged changes'; genuine second batch
        → exit Busy naming pid/host/repo; never force-break) and 'Scope' (commit actions acquire;
        read-only subcommands bypass) and 'Mechanism' (advisory flock auto-released on process death —
        no stale locks). §20.5 mandates this very harness. §15.4 defines exit Busy."
  critical: "This subtask is TEST-ONLY (internal/e2e/). It consumes the COMPLETE, landed lock wiring
             (S1 primitive + S2 contention logic). Do NOT modify any production code, docs, or config."

- docfile: plan/006_c23e6f286ae7/architecture/integration_seams.md
  why: "§7 is the e2e contention-test sketch this subtask implements (newRepo → seed → blocking stub →
        waitForMarker → contender → assert exit/holder → release holder → assert commit). §1 confirms the
        lock acquire is in runDefault after g:=git.New, BEFORE shouldDecompose, and that read-only
        subcommands bypass structurally."
  critical: "§7's prose 'write the marker to release #1' is ONE option; this PRP uses the stub's
             STAGECOACH_STUB_SLEEP_MS timed sleep instead (simpler, battle-tested by S3/S7 in
             scenarios_test.go) — both are valid per the item contract's MOCKING clause."

- docfile: plan/006_c23e6f286ae7/P1M1T2S2/PRP.md
  why: "The CONTRACT for the wiring under test: runDefault acquires after g:=git.New; handleLockContention
        (no-op fast path: snapshot non-empty AND contender WriteTree matches → exit 0; else exit Busy 5);
        generate.go SetSnapshot(treeSHA) + decompose.go SetSnapshot(tStart). S2 is LANDED on disk."
  critical: "The exact handleLockContention messages (read from the live code in this PRP's research §2)
             are the assertion strings. S2's G6 (read-only/hook bypass is structural) is what scenario D
             proves. S2's G3 (silent exits) is why stderr carries the message and the exit code carries
             the code."

- docfile: plan/006_c23e6f286ae7/P1M1T2S3/research/e2e_contention_scenarios.md
  why: "THIS subtask's research: confirms S2 is landed on disk (lock.Acquire at default_action.go:59,
        handleLockContention at 235); pins the exact no-op/busy messages; explains the Busy-vs-no-op crux
        (§3) and the marker-timing invariant (§4); confirms the CLI subcommands and dry-run routing; and
        gives the exact 5-scenario step design (§8)."
  critical: "§3 (the crux) and §5 (reuse STAGECOACH_STUB_SLEEP_MS, no new binary) are the two decisions
             most likely to be gotten wrong without this doc."

# The code under test + the harness primitives (all READ-ONLY — do not edit)
- file: internal/e2e/harness_test.go
  why: "THE harness primitives to reuse: buildStagecoach(t), buildStub(t), newRepo(t), seedCommit(t,repo,
        name,body), writeFile(t,repo,name,body), stageFile(t,repo,name), runGit(t,repo,args...),
        headSHA(t,repo), commitCount(t,repo), runStagecoach(t,bin,repo,cfg,env,args...) → e2eResult{Stdout,
        Stderr,ExitCode}, writeStubConfig(t,stub,extras) → cfg path, stubEnv(knobs map) → []string,
        waitForMarker(t,path,timeout). The file's package doc explains the STAGECOACH_RUN_REAL dual mode."
  pattern: "runStagecoach prepends '--config cfg --no-color' and sets cmd.Dir=repo + cmd.Env=env; it
            captures stdout/stderr separately and returns exitCode via errors.As(*exec.ExitError).
            waitForMarker polls os.Stat(marker) every 20ms up to timeout."
  gotcha: "Both #1 and #2 MUST get the SAME env (so they resolve the same XDG lock dir) AND the same repo
           (cmd.Dir) — the lock key is sha256(canonical(repo)), so same repo ⟹ same lock file ⟹ contention.
           stubEnv(os.Environ()) gives both the inherited env; newRepo's unique t.TempDir() keeps different
           scenarios from colliding (distinct repo ⟹ distinct lock file)."

- file: internal/e2e/scenarios_test.go
  why: "THE pattern to mirror for blocking-stub concurrency: S3 (ConcurrentFile_Excluded) and S7
        (CASAbort_HeadMoved) both launch #1 in a goroutine with STAGECOACH_STUB_MARKER+STAGECOACH_STUB_SLEEP_MS,
        waitForMarker, mutate state, launch/assert #2, then <-resCh. Copy that EXACT structure for A/B/D/E."
  pattern: "resCh := make(chan e2eResult, 1); go func(){ resCh <- runStagecoach(...) }(); waitForMarker(...);
            <contender or mutation>; res := <-resCh."
  gotcha: "S7 stages the file BEFORE launching (#1) — but for the lock, the contender's INDEX at contender
           time is what matters, so stage the contender's extra file AFTER waitForMarker (so it is NOT in
           #1's frozen snapshot). See §Implementation Patterns."

- file: internal/e2e/hook_scenarios_test.go
  why: "THE pattern to mirror for a SEPARATE e2e file: it is its own //go:build e2e file in package e2e
        that adds helpers (runGitCommit, prependPath) and a TestE2EHookScenarios — distinct from
        scenarios_test.go. lock_scenarios_test.go follows the same separation (keeps the contention suite
        isolated, reuses all harness helpers via the same package)."
  critical: "Same package (e2e) ⟹ the new file reuses buildStagecoach/newRepo/runStagecoach/etc. WITHOUT
             redeclaring them. A redeclaration is a compile error."

- file: cmd/stubagent/main.go
  why: "THE stub the blocking pattern drives. Sequence: drain stdin → write STAGECOACH_STUB_MARKER → sleep
        STAGECOACH_STUB_SLEEP_MS → write STAGECOACH_STUB_OUT → exit STAGECOACH_STUB_EXIT. The marker is written
        AFTER stdin drain (which is AFTER stagecoach did WriteTree+SetSnapshot and sent the prompt)."
  critical: "This ordering is WHY waitForMarker is the 'snapshot published' point (research §4). The stub
             is built once via buildStub(t) (stubtest.Build); do NOT modify it."

- file: internal/cmd/default_action.go   # READ-ONLY — the wiring under test
  why: "Confirms (already landed): line 59 lock.Acquire(repoDir); line 63 handleLockContention on
        *lock.HeldError; line 67 defer locker.Release(); line 90 shouldDecompose (AFTER lock); line 197
        dry-run success path (AFTER lock); line 241 handleLockContention body with the exact messages."
  critical: "handleLockContention's no-op branch is `snap != \"\" && contenderTree == snap` → exit 0;
        everything else → exit 5. That is the entire behavior matrix A/B/E exercise."

- file: internal/exitcode/exitcode.go   # READ-ONLY
  why: "exitcode.Busy == 5 (line 28). exitcode.For() resolves the silent exitcode.New(code,nil) returns
        to the binary exit code via the *ExitError short-circuit."
  critical: "So at the subprocess level the contender exits 5 (Busy) / 0 (Success). Assert ExitCode
        directly on e2eResult."

- file: internal/lock/lock.go   # READ-ONLY
  why: "lock.Acquire reads the holder's lock-file contents into HeldError.Contents (Pid/Hostname/Repo/
        Timestamp/Snapshot) at Acquire time. The contention message interpolates Repo/Pid/Hostname/Path."
  critical: "The Busy message includes the repo's CANONICAL path (EvalSymlinks) — so assert
        strings.Contains(stderr, repo) where repo is the t.TempDir() path (it is already canonical)."

# External references (exact, anchor-level)
- url: https://man7.org/linux/man-pages/man2/flock.2.html
  why: "Documents LOCK_EX|LOCK_NB (fail-fast, non-blocking) and that flock is released automatically when
        the fd is closed (process exit, including SIGKILL) — the 'no stale lock' property scenario C tests."
  critical: "Confirms scenario C's invariant: after #1's process exits, the fd closes ⟹ flock releases ⟹
             #2 acquires without contention. No reaping needed."
- url: https://git-scm.com/docs/git-write-tree
  why: "Confirms write-tree reads the index and writes ONE tree object (no ref mutation) — the reason the
        contender's WriteTree probe in handleLockContention is safe WITHOUT holding the lock (PRD §18.5)."
- url: https://pkg.go.dev/testing#T.Run
  why: "t.Run subtests with func(t *testing.T) closures — each gets its own t.TempDir() (unique repo +
        unique readiness-marker path), so subtests never collide."
```

### Current Codebase Tree (relevant slice — S1 COMPLETE, S2 LANDED on disk)

```bash
stagecoach/
├── cmd/
│   ├── stagecoach/main.go        # the binary buildStagecoach compiles
│   └── stubagent/main.go        # the stub buildStub compiles (blocking knobs)
└── internal/
    ├── cmd/
    │   └── default_action.go    # LANDED (S2): lock.Acquire:59, handleLockContention:241
    ├── e2e/
    │   ├── harness_test.go      # //go:build e2e — buildStagecoach/newRepo/runStagecoach/waitForMarker/...
    │   ├── hook_scenarios_test.go   # //go:build e2e — the separate-file pattern to mirror
    │   └── scenarios_test.go    # //go:build e2e — the blocking-stub goroutine pattern (S3/S7)
    ├── exitcode/exitcode.go     # Busy = 5
    ├── generate/generate.go     # LANDED (S2): lock.SetSnapshot(treeSHA)
    └── lock/lock.go             # COMPLETE (S1): Acquire/Release/SetSnapshot/HeldError
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/
    └── e2e/
        └── lock_scenarios_test.go   # NEW — //go:build e2e, package e2e; TestE2ELockContention (A–E)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/e2e/lock_scenarios_test.go` | CREATE | `//go:build e2e` + `package e2e`. `TestE2ELockContention` with 5 `t.Run` subtests exercising every §18.5 contention behavior via real subprocesses. Reuses ALL harness helpers (same package). |

**Explicitly NOT created/modified:** any production code (`internal/cmd`, `internal/lock`,
`internal/exitcode`, `internal/generate`, `internal/decompose`, `internal/git`); the stub agent
(`cmd/stubagent`); the harness (`internal/e2e/harness_test.go`); the existing scenarios
(`scenarios_test.go`, `hook_scenarios_test.go`); any docs (`docs/`, `README.md`); `PRD.md`,
`tasks.json`, `prd_snapshot.md`; anything under `plan/`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — Busy vs no-op is decided by the contender's INDEX): handleLockContention's no-op branch
// is `snap != "" && contenderTree == snap` → exit 0; EVERYTHING else → exit 5. So once the holder has
// published snapshot= (it has, by waitForMarker time), the contender's exit depends ONLY on whether its
// own write-tree equals the holder's snapshot.
//   - Busy scenarios (A, E): the contender MUST stage an EXTRA file (e.g. b.txt) AFTER waitForMarker,
//     so its tree (a.txt+b.txt) DIFFERS from the holder's frozen snapshot (a.txt) → exit 5.
//   - No-op scenario (B): the contender stages NOTHING new, so its tree (a.txt) MATCHES the holder's
//     snapshot (a.txt) → exit 0.
// If you stage the SAME file for both #1 and #2 in scenario A, #2 will exit 0 (no-op) and FAIL the
// Busy assertion. This is the #1 way the implementation goes wrong.

// CRITICAL (G2 — waitForMarker is the "snapshot published" point): the stub writes STAGECOACH_STUB_MARKER
// AFTER draining stdin, which is AFTER stagecoach did WriteTree + lock.SetSnapshot + sent the prompt. So
// when waitForMarker returns, #1 holds the lock AND has published snapshot=. Launch the contender ONLY
// after waitForMarker — this makes A/B/E deterministic (no timing race between lock acquire and snapshot
// publish). For B this is MANDATORY: if #2 ran before #1 published snapshot, #2 would see snap=="" → Busy
// (not the expected no-op).

// CRITICAL (G3 — stage the contender's extra file AFTER waitForMarker, not before): in Busy scenarios,
// b.txt must be staged AFTER #1 has snapshotted (after waitForMarker). If you stage it before launching
// #1, it would be IN #1's snapshot too → #1 commits it → trees match → no-op, not Busy. The contender's
// "genuine second batch" must be genuinely NEW relative to the holder's frozen snapshot.

// GOTCHA (G4 — reuse the stub's timed sleep; NO new blocking binary): the contract mentions "a shell
// script or Go binary that sleeps on a marker file" — but the EXISTING stub already supports
// STAGECOACH_STUB_SLEEP_MS (a timed sleep after the marker). scenarios_test.go S3/S7 use exactly this.
// Set STAGECOACH_STUB_SLEEP_MS=3000 for #1 (long enough for #2 to run mid-flight; short enough to keep
// the suite fast). Do NOT write a new shell script or gate-marker binary.

// GOTCHA (G5 — both subprocesses need the SAME env + repo to contend): the lock key is
// sha256(canonical(repo path)). #1 and #2 MUST run with cmd.Dir = repo (same repo) and inherit the same
// env (same XDG lock dir) so they resolve to the SAME lock file. runStagecoach sets cmd.Dir=repo and
// cmd.Env=env; pass the same `repo` and compatible `env` to both. newRepo's unique t.TempDir() means
// different subtests hash to different lock files (no cross-scenario contention) — so you do NOT need
// explicit XDG isolation (matching the existing scenarios_test.go).

// GOTCHA (G6 — the contender should use a NON-blocking env): give #2 an env WITHOUT the marker/sleep
// knobs (just STAGECOACH_STUB_OUT). #2 exits Busy/no-op BEFORE ever invoking the stub, so blocking knobs
// are irrelevant to #2 — but passing them would be misleading. Use stubEnv(map{"STAGECOACH_STUB_OUT":
// "feat: b"}) for contenders. (It is functionally harmless to reuse #1's env for #2, but the clean
// separation makes intent obvious.)

// GOTCHA (G7 — read-only subcommands bypass via their OWN RunE, not via a flag): providers/config/models
// each have their own RunE and never reach runDefault (where the lock lives). So scenario D just asserts
// they don't exit 5 / don't print "already in progress". No special bypass flag exists or is needed.
// `runStagecoach` already prepends `--config cfg --no-color`, so pass args like []string{"providers","list"}.

// GOTCHA (G8 — dry-run acquires the lock at default_action.go:59, BEFORE the dry-run success path):
// --dry-run goes through runDefault (it's the default action). The lock acquire (line 59) is BEFORE the
// dry-run message print (line 197) and the dry-run+edit check (line 71). So a dry-run contender that hits
// a held lock returns Busy at line 63 BEFORE printing "no commit created". Scenario E asserts exit 5 AND
// that stderr LACKS "no commit created" (proving dry-run never passed the lock). Force Busy (not no-op)
// by staging an extra file for the dry-run (G1).

// GOTCHA (G9 — drain #1's resCh, don't leak the goroutine): after asserting #2, do `res := <-resCh` and
// assert #1's outcome. For scenario D (where #2 are read-only commands and #1 is still sleeping), still
// drain resCh at the end so #1's 5s sleep completes and the goroutine exits. The harness's 60s per-run
// timeout bounds any hang. (Leaked goroutines die when the test binary exits, but draining is cleaner and
// matches S3/S7.)

// GOTCHA (G10 — the e2e file is EXCLUDED from the default suite): `//go:build e2e` means `go test ./...`
// does NOT compile or run it. So adding this file has ZERO impact on `go test -race ./...`. The gate is
// `go test -tags e2e ./internal/e2e/`. This is why S3 is safe to land in parallel with anything — it
// cannot break the default suite.

// GOTCHA (G11 — the file is package e2e, reusing helpers; do NOT redeclare): lock_scenarios_test.go is
// `package e2e` (same as harness_test.go / scenarios_test.go / hook_scenarios_test.go). buildStagecoach,
// buildStub, newRepo, seedCommit, writeFile, stageFile, runGit, headSHA, commitCount, runStagecoach,
// writeStubConfig, stubEnv, waitForMarker are ALL in scope. Redeclaring any is a compile error. The
// `contains` helper lives in scenarios_test.go — reuse it; do NOT redeclare.
```

## Implementation Blueprint

### Data models and structure

None. The test consumes existing types: `e2eResult{Stdout, Stderr string; ExitCode int}` (from the
harness) and the harness's helper functions. No new production types.

### The file skeleton (exact — copy as the starting point)

```go
//go:build e2e

// lock_scenarios_test.go is the PRD §20.5 cross-process regression net for the FR52 per-repo run lock
// (PRD §18.5). It exercises the LANDED S2 contention wiring (internal/cmd/default_action.go:
// lock.Acquire + handleLockContention) against REAL stagecoach subprocesses on REAL temp git repos —
// the layer unit tests cannot reach (real flock across real processes). Reuses the harness primitives
// (buildStagecoach/newRepo/runStagecoach/waitForMarker/writeStubConfig/stubEnv) and the stub agent's
// STAGECOACH_STUB_MARKER + STAGECOACH_STUB_SLEEP_MS blocking pattern (NO new binary). Test-only.
package e2e

import (
	"strings"
	"testing"
	"time"
)

// TestE2ELockContention exercises every PRD §18.5 contention behavior end-to-end.
func TestE2ELockContention(t *testing.T) {
	bin := buildStagecoach(t)
	stub := buildStub(t)
	cfg := writeStubConfig(t, stub, "") // shared; each subtest makes its own repo

	t.Run("A_BusyRefusal_GenuineSecondBatch", func(t *testing.T) { /* §A */ })
	t.Run("B_NoOpFastPath_AccidentalDoubleRun", func(t *testing.T) { /* §B */ })
	t.Run("C_NoStaleLock_AfterExit", func(t *testing.T) { /* §C */ })
	t.Run("D_ReadOnlyBypass", func(t *testing.T) { /* §D */ })
	t.Run("E_DryRunAcquiresLock", func(t *testing.T) { /* §E */ })
}
```

### The 5 subtests (exact step-by-step — copy the bodies)

**§A — Busy refusal (genuine second batch).** #1 blocks holding the lock (snapshot=tree(a.txt)); the
contender stages an EXTRA file so its tree differs → exit 5 (Busy) naming the holder.

```go
func(t *testing.T) {
	repo := newRepo(t)
	seedCommit(t, repo, "readme.md", "init")
	writeFile(t, repo, "a.txt", "a\n") // first batch — covered by #1's snapshot
	stageFile(t, repo, "a.txt")

	readiness := t.TempDir() + "/ready.marker"
	holderEnv := stubEnv(map[string]string{
		"STAGECOACH_STUB_OUT":      "feat: a",
		"STAGECOACH_STUB_MARKER":   readiness,
		"STAGECOACH_STUB_SLEEP_MS": "3000", // #1 holds the lock during generation
	})

	resCh := make(chan e2eResult, 1)
	go func() { resCh <- runStagecoach(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
	waitForMarker(t, readiness, 10*time.Second) // #1 holds lock + published snapshot=tree(a.txt)

	// GENUINE SECOND BATCH: stage b.txt AFTER #1 snapshotted → not in #1's snapshot → tree differs → Busy.
	writeFile(t, repo, "b.txt", "b\n")
	stageFile(t, repo, "b.txt")

	contenderEnv := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: b"})
	res2 := runStagecoach(t, bin, repo, cfg, contenderEnv, "--provider", "stub")
	if res2.ExitCode != 5 {
		t.Fatalf("contender exit = %d, want 5 (Busy); stderr:\n%s", res2.ExitCode, res2.Stderr)
	}
	if !strings.Contains(res2.Stderr, "already in progress") {
		t.Errorf("stderr missing 'already in progress'; got:\n%s", res2.Stderr)
	}
	if !strings.Contains(res2.Stderr, repo) {
		t.Errorf("stderr missing repo path %q (holder must be named); got:\n%s", repo, res2.Stderr)
	}

	res := <-resCh // #1 finishes after its 3s sleep
	if res.ExitCode != 0 {
		t.Fatalf("holder exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
	}
	if n := commitCount(t, repo); n != 2 {
		t.Errorf("commit count = %d, want 2 (seed + #1's a.txt; b.txt stays staged)", n)
	}
	if msg := runGit(t, repo, "log", "-1", "--format=%s"); msg != "feat: a" {
		t.Errorf("HEAD subject = %q, want 'feat: a'", msg)
	}
}
```

**§B — No-op fast path (accidental double-run).** Same index as the holder's snapshot → exit 0 "nothing to do".

```go
func(t *testing.T) {
	repo := newRepo(t)
	seedCommit(t, repo, "readme.md", "init")
	writeFile(t, repo, "a.txt", "a\n")
	stageFile(t, repo, "a.txt")

	readiness := t.TempDir() + "/ready.marker"
	holderEnv := stubEnv(map[string]string{
		"STAGECOACH_STUB_OUT":      "feat: a",
		"STAGECOACH_STUB_MARKER":   readiness,
		"STAGECOACH_STUB_SLEEP_MS": "3000",
	})

	resCh := make(chan e2eResult, 1)
	go func() { resCh <- runStagecoach(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
	waitForMarker(t, readiness, 10*time.Second) // #1 snapshot = tree(a.txt)

	// #2 stages NOTHING NEW → its write-tree (tree(a.txt)) == #1's snapshot → no-op fast path → exit 0.
	contenderEnv := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: a"})
	res2 := runStagecoach(t, bin, repo, cfg, contenderEnv, "--provider", "stub")
	if res2.ExitCode != 0 {
		t.Fatalf("contender exit = %d, want 0 (no-op fast path); stderr:\n%s", res2.ExitCode, res2.Stderr)
	}
	if !strings.Contains(res2.Stderr, "nothing to do") {
		t.Errorf("stderr missing 'nothing to do'; got:\n%s", res2.Stderr)
	}

	res := <-resCh
	if res.ExitCode != 0 {
		t.Fatalf("holder exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
	}
	if n := commitCount(t, repo); n != 2 {
		t.Errorf("commit count = %d, want 2 (only #1 committed; #2 was a no-op)", n)
	}
}
```

**§C — No stale lock (flock auto-released on #1's exit).** Two sequential runs; the second must NOT contend.

```go
func(t *testing.T) {
	repo := newRepo(t)
	seedCommit(t, repo, "readme.md", "init")
	writeFile(t, repo, "a.txt", "a\n")
	stageFile(t, repo, "a.txt")

	env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: a"})

	// #1 runs to completion and its process exits → flock auto-released (no stale lock).
	res1 := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")
	if res1.ExitCode != 0 {
		t.Fatalf("#1 exit = %d, want 0; stderr:\n%s", res1.ExitCode, res1.Stderr)
	}

	// After #1's exit, #2 must acquire without contention (flock released).
	writeFile(t, repo, "b.txt", "b\n")
	stageFile(t, repo, "b.txt")
	res2 := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")
	if res2.ExitCode == 5 {
		t.Fatalf("#2 exited Busy (5) — stale lock! flock must auto-release on #1's exit; stderr:\n%s", res2.Stderr)
	}
	if res2.ExitCode != 0 {
		t.Fatalf("#2 exit = %d, want 0; stderr:\n%s", res2.ExitCode, res2.Stderr)
	}
	if n := commitCount(t, repo); n != 3 {
		t.Errorf("commit count = %d, want 3 (seed + #1 + #2)", n)
	}
}
```

**§D — Read-only bypass.** `providers list` / `config path` / `models --help` against a locked repo must not exit Busy.

```go
func(t *testing.T) {
	repo := newRepo(t)
	seedCommit(t, repo, "readme.md", "init")
	writeFile(t, repo, "a.txt", "a\n")
	stageFile(t, repo, "a.txt")

	readiness := t.TempDir() + "/ready.marker"
	holderEnv := stubEnv(map[string]string{
		"STAGECOACH_STUB_OUT":      "feat: a",
		"STAGECOACH_STUB_MARKER":   readiness,
		"STAGECOACH_STUB_SLEEP_MS": "5000", // hold the lock while we poke the read-only commands
	})

	resCh := make(chan e2eResult, 1)
	go func() { resCh <- runStagecoach(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
	waitForMarker(t, readiness, 10*time.Second) // #1 holds the lock

	baseEnv := stubEnv(nil)
	for _, args := range [][]string{
		{"providers", "list"},
		{"config", "path"},
		{"models", "--help"},
	} {
		res := runStagecoach(t, bin, repo, cfg, baseEnv, args...)
		if res.ExitCode == 5 {
			t.Errorf("%v exited Busy (5) — read-only subcommands must bypass the lock; stderr:\n%s", args, res.Stderr)
		}
		if strings.Contains(res.Stderr, "already in progress") {
			t.Errorf("%v hit the lock; stderr:\n%s", args, res.Stderr)
		}
	}

	if res := <-resCh; res.ExitCode != 0 { // drain #1 (lets its 5s sleep finish)
		t.Fatalf("holder exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
	}
}
```

**§E — Dry-run acquires the lock.** `--dry-run` contender → exit 5 (Busy), never prints its preview.

```go
func(t *testing.T) {
	repo := newRepo(t)
	seedCommit(t, repo, "readme.md", "init")
	writeFile(t, repo, "a.txt", "a\n")
	stageFile(t, repo, "a.txt")

	readiness := t.TempDir() + "/ready.marker"
	holderEnv := stubEnv(map[string]string{
		"STAGECOACH_STUB_OUT":      "feat: a",
		"STAGECOACH_STUB_MARKER":   readiness,
		"STAGECOACH_STUB_SLEEP_MS": "3000",
	})

	resCh := make(chan e2eResult, 1)
	go func() { resCh <- runStagecoach(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
	waitForMarker(t, readiness, 10*time.Second) // #1 snapshot = tree(a.txt)

	// Dry-run contender stages an EXTRA file → tree differs → Busy (proves dry-run goes through runDefault
	// and acquires the lock; if it bypassed, it would print "no commit created" and exit 0).
	writeFile(t, repo, "b.txt", "b\n")
	stageFile(t, repo, "b.txt")
	contenderEnv := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: b"})
	res2 := runStagecoach(t, bin, repo, cfg, contenderEnv, "--dry-run", "--provider", "stub")
	if res2.ExitCode != 5 {
		t.Fatalf("dry-run contender exit = %d, want 5 (Busy — dry-run acquires the lock); stderr:\n%s", res2.ExitCode, res2.Stderr)
	}
	if !strings.Contains(res2.Stderr, "already in progress") {
		t.Errorf("stderr missing 'already in progress'; got:\n%s", res2.Stderr)
	}
	if strings.Contains(res2.Stderr, "no commit created") {
		t.Errorf("dry-run proceeded past the lock (printed 'no commit created'); stderr:\n%s", res2.Stderr)
	}

	if res := <-resCh; res.ExitCode != 0 {
		t.Fatalf("holder exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
	}
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/e2e/lock_scenarios_test.go
  - FILE: internal/e2e/lock_scenarios_test.go ; build tag line 1: `//go:build e2e` ; PACKAGE: e2e.
  - IMPORTS: strings, testing, time. (All harness helpers are in-scope via package e2e — do NOT import
    internal/...; the e2e package is a leaf test package that only uses the compiled binaries.)
  - WRITE the file skeleton (§"file skeleton") + the 5 subtest bodies (§A–§E), verbatim.
  - REUSE (do NOT redeclare): buildStagecoach, buildStub, newRepo, seedCommit, writeFile, stageFile,
    runGit, headSHA, commitCount, runStagecoach, writeStubConfig, stubEnv, waitForMarker, contains.
  - VERIFY it compiles under the tag: go build -tags e2e ./internal/e2e/ → exit 0.

Task 2: VALIDATE — the e2e gate + the no-impact-on-default-suite gate
  - RUN (the gate): go test -tags e2e ./internal/e2e/ -run TestE2ELockContention -v
    → expect all 5 subtests PASS.
  - RUN (no-impact): go test -race ./... → green (the new file is //go:build e2e, excluded by default).
  - RUN: go vet ./... ; gofmt -l . → clean.
  - RUN: git status --porcelain → expect ONLY internal/e2e/lock_scenarios_test.go (new).
```

### Implementation Patterns & Key Details

```go
// === The blocking-stub concurrency idiom (mirror scenarios_test.go S3/S7) ===
// resCh := make(chan e2eResult, 1)
// go func() { resCh <- runStagecoach(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
// waitForMarker(t, readiness, 10*time.Second)   // #1 holds lock + published snapshot
// <launch contender / mutate index>
// res := <-resCh                                  // #1 finishes after STAGECOACH_STUB_SLEEP_MS
// The buffered chan (cap 1) lets the goroutine complete even if the test asserts on #2 first.

// === Why the contender's extra file is staged AFTER waitForMarker (G1/G3) ===
// #1 freezes its snapshot (tree(a.txt)) BEFORE the stub runs (WriteTree → SetSnapshot → prompt → stub).
// The marker appears only after the stub drains stdin — so after waitForMarker, #1's snapshot is frozen
// at tree(a.txt). Staging b.txt NOW guarantees it is NOT in #1's snapshot → #1's commit excludes it, and
// the contender's tree(a.txt+b.txt) ≠ snapshot(tree(a.txt)) → Busy. Stage it BEFORE launching #1 and it
// would land in #1's snapshot (trees match → no-op, breaking the Busy assertion).

// === Why scenario B stages NOTHING for #2 (G1) ===
// The no-op branch needs contenderTree == snapshot. #1's snapshot = tree(a.txt) (a.txt was staged before
// #1 launched). #2's index is still a.txt (nothing new staged) → #2's write-tree = tree(a.txt) == snapshot
// → exit 0 "nothing to do". This is the accidental-double-run: same staged set, second run is redundant.

// === Why scenario C needs no goroutine/marker (G-on-stale) ===
// C proves flock auto-release across PROCESS DEATH. #1 runs to completion (no sleep) and exits; its fd
// closes → flock releases. #2 then acquires cleanly. Sequential runs, no concurrency primitive needed.
// If C fails with exit 5, the lock is NOT being released on exit (a real bug — the §18.5 "no stale locks"
// property is violated).

// === Why read-only bypass needs no special flag (G7) ===
// providers/config/models are separate cobra commands with their own RunE. root.go routes them there;
// they never call runDefault (where lock.Acquire lives). So against a locked repo they simply never
// touch the lock. Scenario D only asserts the absence of Busy — there is no "bypass" code path to test,
// the bypass IS the routing.

// === Why dry-run contends at all (G8) ===
// --dry-run is a FLAG on the default action, not a separate command. runDefault is its RunE. The lock
// acquire (default_action.go:59) runs before the dry-run message path (line 197). So a dry-run into a
// locked repo hits handleLockContention → Busy, never reaching printDryRunMessage. Scenario E's "no 'no
// commit created'" assertion proves dry-run did NOT slip past the lock.
```

### Integration Points

```yaml
HARNESS (consumed — internal/e2e/harness_test.go, READ-ONLY):
  - buildStagecoach(t) → bin ; buildStub(t) → stub
  - newRepo(t) → repo (git init + identity) ; seedCommit/writeFile/stageFile/runGit/headSHA/commitCount
  - runStagecoach(t, bin, repo, cfg, env, args...) → e2eResult{Stdout, Stderr, ExitCode}
  - writeStubConfig(t, stub, extras) → cfg path ; stubEnv(knobs) → []string
  - waitForMarker(t, path, timeout)
  - contains(ss, s) (defined in scenarios_test.go — reuse, do NOT redeclare)

WIRING UNDER TEST (consumed — already LANDED by S1+S2, READ-ONLY):
  - internal/cmd/default_action.go: lock.Acquire(repoDir):59; handleLockContention:241; defer Release:67
  - internal/generate/generate.go: lock.SetSnapshot(treeSHA)   [single-commit path]
  - internal/decompose/decompose.go: lock.SetSnapshot(tStart)  [decompose path — not exercised here]
  - internal/lock/lock.go: Acquire/Release/SetSnapshot/HeldError (COMPLETE)
  - internal/exitcode/exitcode.go: Busy = 5

BUILD TAG (the isolation boundary):
  - //go:build e2e → excluded from `go test ./...`; run via `go test -tags e2e ./internal/e2e/`
  - This is why S3 cannot break the default suite and is safe to land in parallel with anything.

NO-TOUCH (explicitly — owned by siblings / out of scope):
  - All production code (internal/cmd, internal/lock, internal/exitcode, internal/generate,
    internal/decompose, internal/git), the stub agent, the harness, the existing e2e scenarios, docs,
    PRD.md, tasks.json, prd_snapshot.md, plan/*
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/e2e/lock_scenarios_test.go    # Expected: empty (run gofmt -w if listed).
go vet ./internal/e2e/...                         # Expected: exit 0 (no unused import, no shadowing).
go build -tags e2e ./internal/e2e/                # Expected: exit 0 (compiles under the e2e tag).

# Expected: zero output/errors. If `undefined: waitForMarker` (or any harness helper), you are NOT in
# package e2e — the file MUST start with `//go:build e2e` then `package e2e`. If `contains redeclared`,
# you redeclared a helper that already lives in scenarios_test.go — delete your copy.
```

### Level 2: The E2E Gate (the actual deliverable test)

```bash
cd /home/dustin/projects/stagecoach

go test -tags e2e ./internal/e2e/ -run TestE2ELockContention -v
# Expected: all 5 subtests PASS, exit 0:
#   A_BusyRefusal_GenuineSecondBatch   — contender exit 5 + "already in progress" + repo; holder exit 0
#   B_NoOpFastPath_AccidentalDoubleRun — contender exit 0 + "nothing to do"; only holder commits
#   C_NoStaleLock_AfterExit            — second run exit 0 (NOT 5); 3 commits
#   D_ReadOnlyBypass                   — providers list / config path / models --help none exit 5
#   E_DryRunAcquiresLock               — --dry-run exit 5 + "already in progress"; no "no commit created"
#
# The whole suite takes ~15-20s (the 3s/5s stub sleeps dominate). The harness builds stagecoach once.

# Run the full e2e suite to confirm no collateral on the existing scenarios:
go test -tags e2e ./internal/e2e/ -v
# Expected: TestE2EScenarios, TestE2EHookScenarios, AND TestE2ELockContention all pass.
```

### Level 3: No-Impact-on-Default-Suite (the build-tag guarantee)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...      # Expected: ALL packages green. The new file is //go:build e2e, so it is NOT
                         # compiled or run here — zero impact on the default suite. (If this fails, the
                         # failure is pre-existing or in another package, NOT caused by S3.)

go vet ./...             # Expected: exit 0.
gofmt -l .               # Expected: empty.

# Confirm ONLY the new file changed:
git status --porcelain
# Expected EXACTLY:
#   ?? internal/e2e/lock_scenarios_test.go

# Confirm sibling/production territories UNTOUCHED:
git diff --stat -- internal/cmd/ internal/lock/ internal/exitcode/ internal/generate/ internal/decompose/ \
                   cmd/stubagent/ internal/e2e/harness_test.go internal/e2e/scenarios_test.go \
                   internal/e2e/hook_scenarios_test.go docs/ README.md
# Expected: EMPTY.
```

### Level 4: Behavioral Smoke Test (manual repro of the contention matrix)

```bash
cd /home/dustin/projects/stagecoach

# Build the binary + a blocking stub config and reproduce the two outcomes by hand. This mirrors what the
# e2e asserts, against the real flock, to confirm the box behaves as the tests expect.
bin=$(mktemp -d)/stagecoach; go build -o "$bin" ./cmd/stagecoach
tmp=$(mktemp -d); cd "$tmp"; git init -q; git config user.name T; git config user.email t@t
git config --global --get init.defaultBranch >/dev/null 2>&1 || true
echo a > a.txt; git add a.txt; git commit -q -m seed
echo change > a.txt; git add a.txt    # the staged change

cfg=$(mktemp); cat > "$cfg" <<EOF
[provider.stub]
command = "$(go build -o /tmp/stubagent ./cmd/stubagent && echo /tmp/stubagent)"
prompt_delivery = "stdin"
output = "raw"
EOF

# #1 holds the lock (stub sleeps 5s after writing the marker):
( "$bin" --config "$cfg" --no-color --provider stub & )  # writing a marker env requires the stub; for a
# quick smoke, just observe: a second immediate invocation against the same repo exits Busy or no-op.

# The authoritative proof is the e2e test (Level 2); this smoke is optional. Clean up:
cd /; rm -rf "$tmp" "$bin" /tmp/stubagent
```

## Final Validation Checklist

### Technical Validation

- [ ] `gofmt -l internal/e2e/lock_scenarios_test.go` reports nothing.
- [ ] `go vet ./...` exits 0.
- [ ] `go build -tags e2e ./internal/e2e/` exits 0.
- [ ] `go test -tags e2e ./internal/e2e/ -run TestE2ELockContention -v` → all 5 subtests PASS.
- [ ] `go test -race ./...` green (new file excluded by build tag — no impact).

### Feature Validation

- [ ] A: contender exit 5; stderr has "already in progress" + repo path; holder exit 0; commitCount==2; HEAD=="feat: a".
- [ ] B: contender exit 0; stderr has "nothing to do"; commitCount==2 (only holder committed).
- [ ] C: second run exit 0 (NOT 5); commitCount==3.
- [ ] D: `providers list` / `config path` / `models --help` each exit ≠ 5 and lack "already in progress".
- [ ] E: `--dry-run` contender exit 5; stderr has "already in progress"; stderr lacks "no commit created".

### Scope Discipline Validation

- [ ] ONLY `internal/e2e/lock_scenarios_test.go` (new) is changed (`git status --porcelain`).
- [ ] Did NOT touch ANY production code (`internal/cmd`, `internal/lock`, `internal/exitcode`,
      `internal/generate`, `internal/decompose`, `internal/git`).
- [ ] Did NOT touch the stub agent, the harness, or the existing e2e scenarios.
- [ ] Did NOT create a new blocking-stub binary/script (reused `STAGECOACH_STUB_SLEEP_MS`).
- [ ] Did NOT edit docs, `README.md`, `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] File starts with `//go:build e2e` then `package e2e` (reuses all harness helpers; no redeclaration).
- [ ] Follows the blocking-stub goroutine + `waitForMarker` + `<-resCh` idiom from `scenarios_test.go` S3/S7.
- [ ] Subtests named `A_BusyRefusal_GenuineSecondBatch`, `B_NoOpFastPath_AccidentalDoubleRun`,
      `C_NoStaleLock_AfterExit`, `D_ReadOnlyBypass`, `E_DryRunAcquiresLock`.
- [ ] Busy scenarios (A, E) stage the contender's extra file AFTER `waitForMarker` (G1/G3).
- [ ] Contender envs omit the blocking knobs (G6).

---

## Anti-Patterns to Avoid

- ❌ Don't stage the SAME file for both #1 and #2 in scenario A and expect Busy — once the holder
  publishes `snapshot=`, a same-index contender takes the NO-OP fast path (exit 0), not Busy. Busy scenarios
  (A, E) MUST stage an EXTRA file for the contender so its tree differs from the holder's frozen snapshot
  (G1). This is the #1 implementation trap.
- ❌ Don't stage the contender's extra file BEFORE launching #1 — it would land in #1's snapshot (trees
  match → no-op). Stage it AFTER `waitForMarker` so it is genuinely new relative to the frozen snapshot (G3).
- ❌ Don't run #2 before `waitForMarker` in scenario B — #1 may not have published `snapshot=` yet, so #2
  would see `snap==""` → Busy (not the expected no-op). The marker IS the "snapshot published" signal (G2).
- ❌ Don't create a new blocking shell-script or gate-marker binary — the existing stub's
  `STAGECOACH_STUB_SLEEP_MS` timed sleep is the blocking mechanism, exactly as `scenarios_test.go` S3/S7 use
  it (G4).
- ❌ Don't forget `<-resCh` at the end of each concurrency subtest — it lets #1's sleep finish and asserts
  #1's outcome (the holder should commit successfully). Skipping it leaks the goroutine and misses the
  "holder commits" half of the assertion (G9).
- ❌ Don't pass the marker/sleep env to the contender (#2) — #2 exits at the lock BEFORE invoking the stub,
  so blocking knobs are irrelevant to it; use a clean `stubEnv` with just `STAGECOACH_STUB_OUT` for contenders
  to keep intent clear (G6).
- ❌ Don't modify production code, the stub agent, the harness, or the existing scenarios — S3 is TEST-ONLY.
  The wiring under test is already landed (S1+S2); this subtask only observes it.
- ❌ Don't redeclare `buildStagecoach`/`newRepo`/`runStagecoach`/`waitForMarker`/`contains`/etc. — the file is
  `package e2e`; they are all in scope. A redeclaration is a compile error.
- ❌ Don't write the file without `//go:build e2e` — without it, `runStagecoach`/`waitForMarker` (which live
  in `//go:build e2e` files) are undefined and the default `go test ./...` breaks. The build tag is the
  isolation boundary (G10).
- ❌ Don't assert exact exit 0 for read-only subcommands in scenario D — assert `!= 5` (Busy) and absence of
  "already in progress". A read-only command could legitimately exit non-zero for an unrelated reason (e.g.
  a config quirk) on some boxes; the property under test is "it does not hit the LOCK", not "it exits 0".
- ❌ Don't edit `PRD.md`, `tasks.json`, `prd_snapshot.md`, docs, or `plan/*`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a self-contained test file that reuses an established harness and an established
blocking-stub idiom (scenarios_test.go S3/S7), and the wiring under test is ALREADY LANDED on disk (S2's
`lock.Acquire` + `handleLockContention` verified at default_action.go:59/241). The exact assertion strings
("nothing to do …", "already in progress on <repo> (pid … )") are read from the live `handleLockContention`
implementation. The two design rules that would otherwise cause silent failures — (a) Busy-vs-no-op is
decided by the contender's INDEX, so Busy scenarios stage an extra file and the no-op scenario stages
nothing (G1), and (b) the extra file must be staged AFTER `waitForMarker` so it is not in the holder's
frozen snapshot (G3) — are front-loaded as CRITICAL gotchas with verbatim subtest bodies. The marker-timing
invariant (G2) makes the concurrency deterministic. The build tag (G10) guarantees zero impact on the
default suite. The residual uncertainty (not 10/10): the e2e suite depends on the real `flock` syscall and
real `git` against real temp repos — if the box's kernel/git or the stub's sleep timing is unusual, a
subtest could flake (e.g. #2 runs so fast it beats #1's marker; mitigated by `waitForMarker`'s 10s poll).
The 3s/5s sleeps give wide margins. No production-code risk (test-only); no parallel-edit risk (new file
under a build tag no sibling touches).
