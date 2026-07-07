---
name: "P1.M5.T1.S1 — Property/invariant test suite (§20.2): idempotent index, atomic HEAD, snapshot immutability across all §18.2 failure paths"
description: |

  THIS IS A TEST-ONLY TASK. Deliver ONE new file: `internal/generate/invariants_test.go`
  (package generate). It is a table-driven property/invariant suite that drives
  `generate.CommitStaged` (P1.M3.T4.S2, COMPLETE) through every post-snapshot §18.2 failure path
  — parse fail, timeout, CAS fail, SIGINT (context-cancel), plus duplicate-exhaustion and
  agent-nonzero-exit as free bonus scenarios — using the stub provider (`internal/stubtest`,
  P1.M3.T4.S1, COMPLETE) against a real git binary in a temp repo, and asserts the THREE §20.2
  invariants via raw git queries before/after:

    (I1) Idempotent index  — `git diff --cached --name-only` AND `git diff --cached` byte-identical.
    (I2) Atomic HEAD       — `git rev-parse HEAD` unchanged by Stagecoach (CAS: == the externally-moved commit).
    (I3) Snapshot immutability — `git cat-file -p <TREE_SHA>` byte-identical AFTER staging extra content.

  CONTRACT (P1.M5.T1.S1, verbatim):
    1. RESEARCH: "PRD §20.2 — three invariants … system-level safety assertions using the stub provider."
    2. INPUT: "CommitStaged (P1.M3.T4.S2) + stub provider (P1.M3.T4.S1)."  ← both COMPLETE & present.
    3. LOGIC: "Create integration test files that drive CommitStaged through failure paths
       (parse fail, timeout, CAS fail, SIGINT) and assert the three invariants via git queries
       before/after. Use a temp git repo. … Mock: stub provider simulating each failure mode;
       real git binary."
    4. OUTPUT: "A property/invariant test suite that runs in CI and guards the repo-safety guarantee."
    5. DOCS: "none — test infrastructure."

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `internal/generate/generate.go` — CommitStaged orchestrator (P1.M3.T4.S2, Complete). READ ONLY.
    - `internal/stubtest/*`, `cmd/stubagent/main.go` — stub provider (P1.M3.T4.S1, Complete). READ ONLY.
    - `internal/git/*` — git wrapper (P1.M1.T2/T3, Complete). READ ONLY.
    - `internal/signal/*` — signal handler (P1.M4.T2, Complete). The wrappers are nil-safe no-ops when
      no handler is installed; this suite does NOT install one, so they no-op. READ ONLY.
    - The existing per-scenario tests in `internal/generate/generate_test.go` — they pin error types
      (`errors.Is` shapes) that the invariant suite treats as secondary. DO NOT delete/modify them.
      They coexist with the new suite.

  DELIVERABLE (ONE new file, ~150–220 LOC):
    CREATE internal/generate/invariants_test.go   (package generate)
      - `TestInvariants` table-driven: one subtest per failure mode.
      - A shared `assertInvariants` helper encoding I1+I2+I3 (the "property").
      - A `repoSnapshot` struct + `snapshotRepo` capture helper.
      - A `treeSHAFromErr` extractor (handles `*RescueError` AND `*CASError`).
      - Reuses `initRepo`/`writeFile`/`stageFile`/`commitRaw`/`headSHA`/`gitOut` from generate_test.go
        (same package — DO NOT redeclare them).
      - `//go:build !windows` is NOT needed (no real signals sent; stub+git are cross-platform).
        Slow async subtests (CAS/SIGINT/timeout) gate on `testing.Short()`.

  SUCCESS: `go test -race ./internal/generate/ -run TestInvariants -v` green; `go vet ./internal/generate/`
  clean; `gofmt -l internal/generate/` empty; `git status` shows ONLY the new file; `make test` green.
  NO production-code changes. NO new go.mod deps.

---

## Goal

**Feature Goal**: Give Stagecoach a first-class, CI-runnable property/invariant test suite that
**proves the §18.1 safety guarantee** ("any code path that does not reach a successful `update-ref`
leaves the repository byte-for-byte unchanged, modulo dangling objects") across **every** post-snapshot
§18.2 failure path — not as scattered per-test checks, but as one auditable, table-driven "property".

**Deliverable**: ONE new Go test file — `internal/generate/invariants_test.go` (`package generate`) —
containing a table-driven `TestInvariants` (one subtest per failure mode: parse-fail, timeout,
CAS-fail, SIGINT/context-cancel, + duplicate-exhaustion & agent-nonzero-exit bonuses) backed by a
shared `assertInvariants` helper that encodes all three §20.2 invariants (idempotent index, atomic
HEAD, snapshot immutability) via raw git queries against a temp repo, using the real git binary and
the `internal/stubtest` fake agent.

**Success Definition**:
- For EVERY failure-mode subtest, after `CommitStaged` returns its (non-nil) error:
  - **I1 (idempotent index):** `git diff --cached --name-only` AND the full `git diff --cached` are
    byte-identical to the pre-run snapshot.
  - **I2 (atomic HEAD):** `git rev-parse HEAD` is unchanged by Stagecoach. For CAS, HEAD == the
    externally-moved concurrent commit (proving the orchestrator neither landed nor forced).
  - **I3 (snapshot immutability):** the `TreeSHA` carried by the error (`*RescueError` or
    `*CASError`) resolves via `git cat-file -t` to `tree`, and `git cat-file -p <treeSHA>` is
    byte-identical BEFORE and AFTER staging additional content into the index.
- `go test -race ./internal/generate/ -run TestInvariants -v` is green.
- `go vet ./internal/generate/` clean; `gofmt -l internal/generate/` empty.
- `git status` shows ONLY `internal/generate/invariants_test.go` (no production-code edits).
- `make test` green (no regression in the existing tests that share the package's helpers).

## User Persona

**Target User**: the Stagecoach maintainer / release engineer (PRD §20). This is test infrastructure,
not a user-facing feature.

**Use Case**: run `go test -race ./internal/generate/ -run TestInvariants -v` (locally and in CI) to
PROVE the core repo-safety invariant before every merge/release. A failing subtest = a regression that
would corrupt a user's repository (move HEAD or mutate the index on a failure path) — a release blocker.

**Pain Points Addressed**: the §20.2 invariants were previously asserted only *piecemeal* across
separate tests (and snapshot-immutability not at all). This suite makes the guarantee a single,
greppable, auditable guard that a maintainer can read top-to-bottom and trust.

## Why

- **Realizes PRD §20.2 as a testable property.** The three invariants become one table-driven
  "for every failure mode F, invariants I1∧I2∧I3 hold" — exactly the §20.2 framing.
- **Closes the snapshot-immutability gap.** `git cat-file -p <TREE_SHA>` stability is asserted NOWHERE
  in the current tree; this suite adds it as the headline new guard.
- **Closes the orchestrator-level SIGINT gap.** Context-cancellation during generation is the
  orchestrator-level manifestation of SIGINT; it was only tested at the CLI/os.Exit level
  (`signal_integration_test.go`), never against `CommitStaged` directly with repo-invariant assertions.
- **Guards §18.1 cheaply forever.** The §18.1 "byte-for-byte unchanged" guarantee is the product's
  core safety promise; this suite is the regression net that catches any future refactor that
  accidentally mutates the index or moves HEAD on a failure path.
- **Avoids scope creep.** No production code changes, no new deps, no CI config (the suite is picked
  up automatically by the existing `go test ./...` / `make test`). The stub provider and CommitStaged
  already exist and are COMPLETE.

## What

A new `internal/generate/invariants_test.go` (`package generate`) with:

1. **A `repoSnapshot` struct** capturing the pre-run state: `head`, `indexNames`
   (`git diff --cached --name-only`), `indexFull` (`git diff --cached`).
2. **A `snapshotRepo(t, repo) repoSnapshot` helper** that runs the three git queries.
3. **A `treeSHAFromErr(t, err) string` extractor** that handles `*RescueError` (`.TreeSHA`) and
   `*CASError` (`.TreeSHA`), failing the test if neither matches or the SHA is empty.
4. **A shared `assertInvariants(t, repo, before repoSnapshot, treeSHA, wantHead string)` helper**
   encoding the "property": asserts I1 (index names + full byte-equal), I2 (HEAD == wantHead), and
   I3 (cat-file -t == "tree"; cat-file -p stable across a subsequent `stageFile` of NEW content).
5. **`TestInvariants`** — a table of `scenario{ name; run func(...) (err, wantHead) }`. Each `run`
   closure stages a known file, drives `CommitStaged` to the named failure via the stub, and returns
   the error + the expected post-run HEAD (`""` ⇒ use the pre-run HEAD; CAS returns its concurrent
   commit). The harness does the before-snapshot, calls `run`, extracts the treeSHA, and calls
   `assertInvariants`.

**Failure modes covered (subtests):**

| subtest name          | trigger (stub)                                      | expected error                    | wantHead             |
|-----------------------|-----------------------------------------------------|-----------------------------------|----------------------|
| `ParseFail`           | `NewScript([""])`, `cfg.MaxDuplicateRetries=0`      | `*RescueError` (`ErrRescue`)      | pre-run HEAD         |
| `Timeout`             | `Manifest(SleepMS=2000)`, `cfg.Timeout=150ms`       | `*RescueError` (`ErrTimeout`)     | pre-run HEAD         |
| `SigintContextCancel` | `Manifest(SleepMS=3000)` + cancel ctx after ~150ms  | `*RescueError` (`ErrRescue`)      | pre-run HEAD         |
| `CASFailure`          | `Manifest(SleepMS=400)` + concurrent commit         | `*CASError` (`ErrCASFailed`)      | the concurrent SHA   |
| `DuplicateExhaustion` (bonus) | `NewScript(["feat: existing"])` w/ HEAD=="feat: existing", retries=0 | `*RescueError` (`ErrRescue`) | pre-run HEAD |
| `AgentNonzeroExit` (bonus)   | `Manifest(Exit=2, Out="")`                  | `*RescueError` (`ErrRescue`)      | pre-run HEAD         |

The four named in the contract — **parse fail, timeout, CAS fail, SIGINT** — are required; the two
bonus rows are cheap (same stub seam, same invariant shape) and strengthen the property. Include all six.

### Success Criteria

- [ ] `internal/generate/invariants_test.go` exists, `package generate`, compiles cleanly.
- [ ] `TestInvariants` has a passing subtest for each of: ParseFail, Timeout, SigintContextCancel,
      CASFailure (the 4 named), plus DuplicateExhaustion & AgentNonzeroExit.
- [ ] Every subtest asserts all THREE invariants (I1, I2, I3) via the shared `assertInvariants`.
- [ ] I3 (snapshot immutability) is asserted for EVERY subtest — including CAS (the `*CASError.TreeSHA`).
- [ ] No production-code edits; `git status` shows ONLY the new test file.
- [ ] `go test -race ./internal/generate/ -run TestInvariants -v` green; `go vet` clean; `gofmt -l` empty.
- [ ] `make test` green (no regression in the existing shared-helper tests).

## All Needed Context

### Context Completeness Check

_Pass._ A Go test author with no prior knowledge of this repo can implement this from: the exact
CommitStaged failure→error→TreeSHA mapping (§"Implementation Patterns"), the stub provider's knobs
(§Documentation), the list of reusable helpers already in `generate_test.go` (so they are NOT
redeclared), the CAS async pattern (quoted from the already-green `TestCommitStaged_CASFailure`), and
the three git queries that constitute each invariant. No production-code reading beyond
`generate.go`'s error types is required.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M5T1S1/research/findings.md
  why: THE decisive doc. §1 failure→error→TreeSHA mapping table; §2 stub knobs; §3 the reusable
       helpers in generate_test.go (DO NOT redeclare); §4 the gap audit (snapshot-immutability
       untested, SIGINT-at-orchestrator untested); §5 why context-cancel is the SIGINT sim +
       nil-safe signal wrappers; §6 the CAS async pattern; §7 validation commands; §8 risks.
  critical: §1 (error types + .TreeSHA), §3 (reuse, don't redeclare), §6 (CAS timing).

- file: internal/generate/generate.go   (P1.M3.T4.S2 — READ only; the orchestrator under test)
  section: CommitStaged — the failure branches you will trigger:
       - snapshot: `treeSHA, err := deps.Git.WriteTree(ctx)` then `signal.SetSnapshot(...)` (step 3).
       - timeout: `if errors.Is(execErr, context.DeadlineExceeded) { return ..., &RescueError{Kind:ErrTimeout, TreeSHA:treeSHA, ...} }`.
       - SIGINT/cancel: `if errors.Is(execErr, context.Canceled) { return ..., &RescueError{Kind:ErrRescue, TreeSHA:treeSHA, ...} }`.
       - loop exhaustion (parse-fail / duplicate): `if !success { return ..., &RescueError{Kind:ErrRescue, TreeSHA:treeSHA, ...} }`.
       - CAS: `if errors.Is(err, git.ErrCASFailed) { actual,_ := deps.Git.RevParseHEAD(ctx); return ..., &CASError{TreeSHA:treeSHA, Expected:parentSHA, Actual:actual, ...} }`.
  why: these branches are EXACTLY what each subtest triggers; the returned error's `.TreeSHA` is the
       input to the I3 (snapshot-immutability) assertion. `RescueError`/`CASError` are in THIS package
       (generate) so the test can type-assert directly (`errors.As(err, &re)`).
  pattern: the test asserts on the SAME error types the existing generate_test.go already asserts on
       (TestCommitStaged_ParseFailRescue/Timeout/CASFailure) — mirror their errors.As/errors.Is usage.
  gotcha: a PRE-snapshot failure (ErrNothingToCommit / WriteTree merge-conflict) has NO TreeSHA and is
       OUT OF SCOPE — §20.2 invariants are about POST-snapshot failure paths only. Do not add a
       nothing-to-commit subtest (it has no tree to assert I3 on).

- file: internal/generate/generate_test.go   (P1.M3.T4.S2 — READ only; SOURCE OF REUSABLE HELPERS)
  section: the unexported helpers at the TOP of the file (package generate, internal test):
       `initRepo`, `writeFile`, `stageFile`, `headSHA`, `commitRaw`, `gitOut`, `runGit`, `shaRe`.
  why: your new file is ALSO `package generate` → it can call these DIRECTLY. DO NOT redeclare any of
       them (compile error: redeclared in this block). Also mine these existing tests for the proven
       CAS async pattern: TestCommitStaged_CASFailure (goroutine + time.Sleep(150ms) + commitRaw +
       done channel + errors.As(&ce)).
  pattern: t.Helper()+t.Fatalf/t.Errorf style; `runGit(t, dir, args...)` returns trimmed stdout;
       `gitOut` is its public alias. `commitRaw` uses --allow-empty so it moves HEAD WITHOUT touching
       the index (critical for the CAS idempotent-index assertion).
  gotcha: the existing tests do NOT use t.Parallel() for CommitStaged runs (the signal package uses a
       process-global singleton). Match them — do NOT add t.Parallel() to TestInvariants or its subtests.

- file: internal/stubtest/stubtest.go   (P1.M3.T4.S1 — READ only; the fake agent harness)
  section: `Build(t)` (compiles cmd/stubagent once, cached), `Manifest(bin, Options{...})`,
       `NewScript(t, bin, []string{...})`, and the `Options` knobs.
  why: this is HOW you trigger each failure mode. Knobs: Out (single response), Exit (non-zero),
       SleepMS (slow/timeout/async), Script/Counter (call-varying; blank entries ⇒ ParseOutput ok=false).
  pattern: `bin := stubtest.Build(t)` at the top of TestInvariants (NOT per-subtest — sync.Once caches it).
  gotcha: NewScript writes to t.TempDir() (auto-cleaned); blank entries in the script slice are
       SIGNIFICANT (empty stdout → parse fail → retry). After the list is exhausted the last response repeats.

- file: internal/git/git.go   (P1.M1.T2/T3 — READ only; the git boundary, confirms the queries)
  section: the methods the assertions shell out to (via raw `git -C repo` in runGit, NOT via the
       interface): WriteTree returns the immutable tree SHA; the tree is content-addressed & immutable.
  why: confirms I3 is sound — `git cat-file -p <treeSHA>` is deterministic for a fixed SHA; staging new
       content creates NEW objects but never mutates an existing tree object. git's default gc.auto
       (~6700 loose objects) is never hit in a test, so the dangling tree persists for cat-file.
  gotcha: use raw `gitOut(t, repo, "cat-file", "-p", treeSHA)` / `"cat-file", "-t", treeSHA` for I3 —
       do NOT add a new git-interface method (out of scope; the wrapper is frozen/complete).

- url: (PRD internal) PRD.md §18.1 (the invariant), §18.2 (failure modes table), §20.2 (the three
       property tests). AUTHORITATIVE spec for WHAT the suite must prove.
  why: §18.1 = "every code path that does not reach a successful update-ref leaves the repository
       byte-for-byte unchanged (modulo harmless dangling objects)"; §20.2 names the three invariants
       verbatim. The suite is the executable form of §20.2.
  critical: §20.2's I3 wording — "git cat-file -p <TREE_SHA> is stable across the run regardless of
       subsequent staging" — is EXACTLY what the stage-extra-content-then-re-cat-file check implements.
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  generate.go                # P1.M3.T4.S2 — CommitStaged orchestrator (READ only; under test)
  generate_test.go           # P1.M3.T4.S2 — package generate; HAS reusable helpers + per-scenario tests
  dedupe.go / rescue.go      # P1.M3.T2/T3 — ExtractSubject, IsDuplicate, FormatRescue (READ only)
  invariants_test.go         # ← NEW (this task): TestInvariants + assertInvariants + helpers
internal/stubtest/stubtest.go # P1.M3.T4.S1 — Build/Manifest/NewScript (READ only)
cmd/stubagent/main.go         # P1.M3.T4.S1 — the fake agent binary (READ only)
internal/git/git.go           # P1.M1.T2/T3 — git boundary (READ only)
internal/signal/signal.go     # P1.M4.T2 — nil-safe wrappers (no handler installed → no-op) (READ only)
Makefile                      # test / coverage / vet / lint targets (UNCHANGED)
go.mod                        # module github.com/dustin/stagecoach ; go 1.22 (UNCHANGED — no new deps)
```

### Desired Codebase tree with files to be added

```bash
internal/generate/invariants_test.go   # NEW — package generate; TestInvariants table-driven suite.
# ALL other files UNCHANGED. No production-code edits, no go.mod changes, no new packages.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (REUSE, DON'T REDECLARE): generate_test.go is `package generate` (an INTERNAL test, not
// `package generate_test`). Your new invariants_test.go MUST also be `package generate` so it can call
// the unexported helpers already declared there: initRepo, writeFile, stageFile, commitRaw, headSHA,
// gitOut, runGit, shaRe. If you re-declare ANY of them → "redeclared in this block" compile error.
// Mine generate_test.go for the proven patterns instead of reinventing them.

// CRITICAL (POST-SNAPSHOT ONLY): §20.2 invariants apply only to POST-SNAPSHOT failure paths (those
// that return a non-empty TreeSHA: *RescueError / *CASError). A PRE-snapshot failure like
// ErrNothingToCommit has NO tree → I3 is meaningless. Do NOT add a nothing-to-commit subtest.
// (generate.go returns ErrNothingToCommit BEFORE WriteTree; it is correctly out of scope.)

// CRITICAL (SIGINT SIM = CONTEXT CANCEL): do NOT send a real os.Signal from this suite. The signal
// package is opt-in (Install); we do NOT install it, so signal.* wrappers are nil-safe no-ops and the
// process stays alive. The orchestrator-level SIGINT path is `context.Canceled` → RescueError{ErrRescue}.
// Simulate by: cancellable ctx, stub SleepMS=3000, spawn CommitStaged in a goroutine, cancel after
// ~150ms (snapshot is microseconds; long done by then → TreeSHA is non-empty). The real SIGINT→os.Exit(3)
// CLI path is already covered by internal/signal/signal_integration_test.go (do not duplicate).

// GOTCHA (CAS IS THE SPECIAL CASE): for CAS, the TEST moves HEAD mid-run (commitRaw --allow-empty)
// to simulate concurrency. "Atomic HEAD" therefore reads as `headSHA(repo) == concurrentSHA` (the
// externally-moved value) — proving the orchestrator neither landed NOR forced. The idempotent-index
// invariant STILL HOLDS for CAS because --allow-empty does not touch the index. Mirror the EXACT
// timing of the already-green TestCommitStaged_CASFailure (stub SleepMS=400, sleep 150ms before the
// concurrent commit) to avoid flakiness.

// GOTCHA (NO t.Parallel): the signal package keeps a process-global singleton (active atomic.Pointer).
// Existing generate tests run CommitStaged serially. Do NOT call t.Parallel() in TestInvariants or its
// subtests — keep them serial to avoid global-state cross-talk.

// GOTCHA (testing.Short for slow subtests): Timeout/SigintContextCancel/CASFailure involve real
// time.Sleep (150ms–400ms) and (for SIGINT/CAS) goroutine sync. Gate them on !testing.Short() so
// `go test -short` stays fast; the simple synchronous subtests (ParseFail, DuplicateExhaustion,
// AgentNonzeroExit) can always run. Mirror signal_integration_test.go's `if testing.Short() { t.Skip(...) }`.

// GOTCHA (I3 needs the tree to PERSIST): `git cat-file -p <treeSHA>` must succeed AFTER the run and
// AFTER a subsequent stageFile. git's default gc.auto (~6700 loose objects) is never reached in a test,
// so the dangling snapshot tree is never collected. Do NOT run `git gc` or `git prune` in the test.
// Assert `git cat-file -t <treeSHA>` == "tree" (not "missing") as a guard before the -p comparison.

// GOTCHA (extract TreeSHA from BOTH error types): parse-fail/timeout/SIGINT/dup/nonzero → *RescueError
// (.TreeSHA); CAS → *CASError (.TreeSHA). treeSHAFromErr must errors.As both. Fail the test (t.Fatal)
// if neither matches OR the SHA is empty (an empty SHA would mean a pre-snapshot failure snuck in).
```

## Implementation Blueprint

### Data models and structure

No production data models. The test file declares small test-only types:

```go
package generate

// repoSnapshot captures the pre-run repo state for the §20.2 invariant comparison.
type repoSnapshot struct {
	head       string // git rev-parse HEAD
	indexNames string // git diff --cached --name-only  (I1: names)
	indexFull  string // git diff --cached              (I1: byte-for-byte)
}

// scenario is one row of the TestInvariants table: a named §18.2 failure mode + the closure that
// drives CommitStaged into it and reports the expected post-run HEAD.
type scenario struct {
	name string // subtest name (ParseFail, Timeout, SigintContextCancel, CASFailure, …)
	// run executes CommitStaged for this failure mode and returns its (non-nil) error plus the
	// expected post-run HEAD. wantHead=="" tells the harness to use the pre-run HEAD (the normal case);
	// CAS returns the concurrent commit it moved HEAD to.
	run func(t *testing.T, ctx context.Context, repo, bin string) (err error, wantHead string)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY the seams exist and are green (READ + RUN, no edit)
  - RUN: `go test -race ./internal/generate/ -v` → MUST be green (proves CommitStaged + helpers + stub
      are all present and working; this is the baseline you must not regress).
  - RUN: `go test -race ./internal/generate/ -run 'TestCommitStaged_(ParseFailRescue|Timeout|CASFailure)' -v`
      → green (these are the proven patterns you will generalize; they pin errors.Is shapes).
  - READ: internal/generate/generate.go — confirm the 5 failure branches (snapshot, timeout, cancel,
      loop-exhaustion, CAS) and that *RescueError/*CASError carry a non-empty .TreeSHA.
  - READ: internal/generate/generate_test.go TOP — confirm the reusable helpers (initRepo, writeFile,
      stageFile, commitRaw, headSHA, gitOut, runGit, shaRe) are `package generate` (internal test).
  - READ: internal/stubtest/stubtest.go — confirm Build/Manifest/NewScript + Options knobs.
  - GOTCHA: if any helper is MISSING or generate_test.go is `package generate_test` (external), STOP and
      report — the "reuse, don't redeclare" premise depends on it being an internal test. (It is internal.)

Task 2: CREATE internal/generate/invariants_test.go — scaffolding + helpers (the "property" layer)
  - FILE: CREATE internal/generate/invariants_test.go with `package generate` and the imports already
      used by generate_test.go (context, errors, strings, testing, time + config, git, stubtest). NO new
      imports beyond what generate_test.go uses (stdlib only; no new go.mod deps).
  - IMPLEMENT:
      func snapshotRepo(t *testing.T, repo string) repoSnapshot  // head + indexNames + indexFull via gitOut
      func treeSHAFromErr(t *testing.T, err error) string        // errors.As *RescueError AND *CASError; t.Fatal if neither/empty
      func assertInvariants(t *testing.T, repo string, before repoSnapshot, treeSHA, wantHead string)
  - assertInvariants body (the THREE invariants):
      // I1 — idempotent index (names + full byte-equal)
      if got := gitOut(t, repo, "diff", "--cached", "--name-only"); got != before.indexNames { t.Errorf("I1 names: got %q want %q", got, before.indexNames) }
      if got := gitOut(t, repo, "diff", "--cached");                got != before.indexFull  { t.Errorf("I1 full diff mutated (byte-for-byte mismatch)") }
      // I2 — atomic HEAD (wantHead=="" ⇒ use before.head)
      if wantHead == "" { wantHead = before.head }
      if got := headSHA(t, repo); got != wantHead { t.Errorf("I2 HEAD: got %q want %q", got, wantHead) }
      // I3 — snapshot immutability: cat-file -t == "tree"; cat-file -p stable across a subsequent stageFile
      if got := gitOut(t, repo, "cat-file", "-t", treeSHA); got != "tree" { t.Fatalf("I3 cat-file -t: got %q want tree (object missing?)", got) }
      treeBefore := gitOut(t, repo, "cat-file", "-p", treeSHA)
      writeFile(t, repo, "immutable_probe.txt", "this content must not change the frozen tree")
      stageFile(t, repo, "immutable_probe.txt")
      treeAfter := gitOut(t, repo, "cat-file", "-p", treeSHA)
      if treeAfter != treeBefore { t.Errorf("I3 snapshot mutated: cat-file -p changed after staging (immutability violated)") }
  - WHY: assertInvariants IS the property — every subtest funnels through it, so the three invariants
      are asserted uniformly and can never be silently dropped from a new scenario.
  - GOTCHA: t.Helper() at the top of each helper so failures point at the CALLING subtest, not the helper.

Task 3: CREATE the synchronous-failure scenarios (ParseFail, DuplicateExhaustion, AgentNonzeroExit)
  - These need NO goroutine/timing — straightforward stub config.
  - ParseFail:    stubtest.NewScript(t, bin, []string{""}); cfg.MaxDuplicateRetries = 0
                  → blank stdout → ParseOutput ok=false → loop exhausted → *RescueError{ErrRescue}. wantHead="".
  - DuplicateExhaustion: set HEAD subject to "feat: existing" first (commitRaw(t, repo, "feat: existing")),
                  then stubtest.NewScript(t, bin, []string{"feat: existing"}); cfg.MaxDuplicateRetries = 0
                  → subject matches → retry → exhausted → *RescueError{ErrRescue}. wantHead="".
  - AgentNonzeroExit: stubtest.Manifest(bin, stubtest.Options{Exit: 2, Out: ""}); cfg.MaxDuplicateRetries = 0
                  → non-zero exit → ParseOutput ok=false (empty stdout) → exhausted → *RescueError{ErrRescue}. wantHead="".
  - WHY: covers the parse/nonzero/dup §18.2 rows cheaply; they share the stub seam and the same invariant shape.
  - GOTCHA: these are FAST (no SleepMS) → do NOT gate on testing.Short(); run always.

Task 4: CREATE the Timeout scenario (needs a short cfg.Timeout)
  - Timeout: stubtest.Manifest(bin, stubtest.Options{Out: "feat: slow", SleepMS: 2000});
             cfg := config.Defaults(); cfg.Timeout = 150 * time.Millisecond.
             → Execute returns context.DeadlineExceeded → *RescueError{ErrTimeout}. wantHead="".
  - ASSERT (secondary, in run): errors.Is(err, ErrTimeout) (NOT ErrRescue) — pins the timeout branch.
  - WHY: the §18.2 timeout row + the ErrTimeout-specific branch.
  - GOTCHA: gate on `if testing.Short() { t.Skip(...) }` (2000ms stub sleep). cfg.Timeout MUST be <
      SleepMS so the context deadline fires before the stub finishes.

Task 5: CREATE the SIGINT (context-cancel) scenario — the orchestrator-level SIGINT sim
  - SigintContextCancel:
      ctx, cancel := context.WithCancel(context.Background())
      m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: hang", SleepMS: 3000})
      done := make(chan error, 1)
      go func() { _, e := CommitStaged(ctx, Deps{Git: git.New(repo), Manifest: m}, cfg); done <- e }()
      time.Sleep(150 * time.Millisecond)   // let snapshot (step 3) + generation start
      cancel()                             // ← simulate SIGINT: cancels ctx → Execute returns context.Canceled
      err := <-done
      → *RescueError{ErrRescue} (the context.Canceled branch). wantHead="".
  - ASSERT (secondary): errors.Is(err, ErrRescue); re.TreeSHA != "".
  - WHY: closes the orchestrator-level SIGINT gap (§4 of findings). The real SIGINT→os.Exit(3) path
      stays covered by signal_integration_test.go; this proves CommitStaged leaves the repo untouched.
  - GOTCHA: gate on testing.Short(). The 150ms wait is generous (WriteTree is microseconds → snapshot
      done → TreeSHA non-empty). Do NOT send a real os.Signal (signal package not installed; wrappers no-op).

Task 6: CREATE the CAS scenario — the async + externally-mutating case (mirror TestCommitStaged_CASFailure)
  - CASFailure:
      parent := headSHA(t, repo)
      m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: x", SleepMS: 400})
      done := make(chan error, 1)
      go func() { _, e := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg); done <- e }()
      time.Sleep(150 * time.Millisecond)        // let snapshot + generation start
      commitRaw(t, repo, "concurrent commit")   // --allow-empty → HEAD moves, INDEX UNCHANGED
      concurrent := headSHA(t, repo)
      err := <-done
      → *CASError; errors.Is(err, ErrCASFailed); ce.TreeSHA != ""; ce.Expected == parent; ce.Actual == concurrent.
      wantHead = concurrent   // ← I2 reads as "HEAD == the concurrent commit" (orchestrator did NOT land/force).
  - WHY: the §18.2 CAS row + the atomic-HEAD invariant's special case.
  - GOTCHA: gate on testing.Short(). commitRaw --allow-empty keeps the index stable so I1 still holds
      for CAS. Mirror the EXACT timing of the already-green TestCommitStaged_CASFailure.

Task 7: ASSEMBLE TestInvariants — the table + harness that funnels every scenario through assertInvariants
  - FILE: append `func TestInvariants(t *testing.T)` to invariants_test.go.
  - BODY:
      bin := stubtest.Build(t)   // ONCE (sync.Once caches it) — do not rebuild per subtest
      scenarios := []scenario{ {ParseFail...}, {DuplicateExhaustion...}, {AgentNonzeroExit...},
                               {Timeout...}, {SigintContextCancel...}, {CASFailure...} }
      for _, sc := range scenarios {
          sc := sc
          t.Run(sc.name, func(t *testing.T) {
              // shared fixture: born repo + initial commit + one staged file
              repo := t.TempDir(); initRepo(t, repo); commitRaw(t, repo, "initial")
              writeFile(t, repo, "staged.txt", "snapshotted content"); stageFile(t, repo, "staged.txt")
              before := snapshotRepo(t, repo)
              err, wantHead := sc.run(t, context.Background(), repo, bin)
              if err == nil { t.Fatal("expected a failure-path error, got nil") }
              treeSHA := treeSHAFromErr(t, err)
              assertInvariants(t, repo, before, treeSHA, wantHead)
          })
      }
  - WHY: one auditable table; adding a future failure mode = one scenario row (the property auto-applies).
  - GOTCHA: the DuplicateExhaustion scenario needs HEAD=="feat: existing" — its run closure must do its
      OWN commitRaw BEFORE snapshotRepo captures state (or accept the harness's "initial" commit and
      adjust). Cleanest: each run closure that needs a specific HEAD subject sets it up itself and the
      harness's "initial" commit is just a baseline parent. (See Implementation Patterns for the clean
      split: the harness seeds a born repo + staged.txt; the closure may add scenario-specific history.)
      NOTE `sc := sc` is NOT required when the loop body is `t.Run` with no closure capturing the loop
      variable after the iteration — but add it defensively (pre-go1.22 loop-var capture; go.mod is 1.22
      so it's optional). Prefer NOT capturing sc across goroutines inside run (run owns its own vars).

Task 8: FINAL VALIDATION (the gate)
  - RUN: `gofmt -w internal/generate/invariants_test.go`; `gofmt -l internal/generate/` (must be empty).
  - RUN: `go vet ./internal/generate/` (clean).
  - RUN: `go test -race ./internal/generate/ -run TestInvariants -v` → ALL subtests green; confirm each
      subtest ran (look for `=== RUN   TestInvariants/ParseFail` etc. and `--- PASS`).
  - RUN: `go test -race ./internal/generate/ -v` → green (no regression in the existing tests).
  - RUN: `go test -race -short ./internal/generate/ -run TestInvariants -v` → the fast subtests
      (ParseFail/DuplicateExhaustion/AgentNonzeroExit) run & pass; the slow ones SKIP cleanly.
  - RUN: `make test` → green (whole-tree; proves no cross-package regression).
  - RUN: `git status` → ONLY internal/generate/invariants_test.go changed/added.
```

### Implementation Patterns & Key Details

```go
// The property — assertInvariants (every subtest funnels through this):
func assertInvariants(t *testing.T, repo string, before repoSnapshot, treeSHA, wantHead string) {
	t.Helper()
	// I1: idempotent index — names + full byte-for-byte.
	if got := gitOut(t, repo, "diff", "--cached", "--name-only"); got != before.indexNames {
		t.Errorf("I1 (idempotent index) names: got %q, want %q", got, before.indexNames)
	}
	if got := gitOut(t, repo, "diff", "--cached"); got != before.indexFull {
		t.Errorf("I1 (idempotent index) full diff mutated: before=%q after=%q", before.indexFull, got)
	}
	// I2: atomic HEAD — unchanged by Stagecoach (CAS: == the externally-moved commit).
	if wantHead == "" {
		wantHead = before.head
	}
	if got := headSHA(t, repo); got != wantHead {
		t.Errorf("I2 (atomic HEAD): got %q, want %q", got, wantHead)
	}
	// I3: snapshot immutability — cat-file -t == tree; cat-file -p stable across a subsequent stage.
	if got := gitOut(t, repo, "cat-file", "-t", treeSHA); got != "tree" {
		t.Fatalf("I3 (snapshot immutability): cat-file -t %q = %q, want \"tree\" (object missing?)", treeSHA, got)
	}
	treeBefore := gitOut(t, repo, "cat-file", "-p", treeSHA)
	writeFile(t, repo, "immutable_probe.txt", "must not alter the frozen snapshot tree")
	stageFile(t, repo, "immutable_probe.txt")
	treeAfter := gitOut(t, repo, "cat-file", "-p", treeSHA)
	if treeAfter != treeBefore {
		t.Errorf("I3 (snapshot immutability): cat-file -p %q changed after staging (content-addressing violated)", treeSHA)
	}
}

// Extract TreeSHA from EITHER error type:
func treeSHAFromErr(t *testing.T, err error) string {
	t.Helper()
	var re *RescueError
	if errors.As(err, &re) {
		if re.TreeSHA == "" {
			t.Fatalf("RescueError.TreeSHA empty (pre-snapshot failure? out of scope): %v", err)
		}
		return re.TreeSHA
	}
	var ce *CASError
	if errors.As(err, &ce) {
		if ce.TreeSHA == "" {
			t.Fatalf("CASError.TreeSHA empty: %v", err)
		}
		return ce.TreeSHA
	}
	t.Fatalf("error is neither *RescueError nor *CASError (got %T): %v", err, err)
	return ""
}

// SIGINT sim (Task 5) — context cancel = the orchestrator-level SIGINT path:
func runSigintCancel(t *testing.T, ctx context.Context, repo, bin string) (error, string) {
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: hang", SleepMS: 3000})
	cctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { _, e := CommitStaged(cctx, Deps{Git: git.New(repo), Manifest: m}, config.Defaults()); done <- e }()
	time.Sleep(150 * time.Millisecond) // snapshot (step 3) is microseconds → long done → TreeSHA non-empty
	cancel()                           // ← SIGINT effect: ctx cancelled → Execute returns context.Canceled
	err := <-done
	if !errors.Is(err, ErrRescue) {
		t.Errorf("SigintContextCancel: errors.Is(err, ErrRescue) = false, got %v", err)
	}
	return err, "" // wantHead="" ⇒ assertInvariants uses the pre-run HEAD
}

// CAS (Task 6) — mirror the already-green TestCommitStaged_CASFailure timing EXACTLY:
func runCASFailure(t *testing.T, ctx context.Context, repo, bin string) (error, string) {
	if testing.Short() {
		t.Skip("skipping slow CAS scenario in -short mode")
	}
	parent := headSHA(t, repo)
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: x", SleepMS: 400})
	done := make(chan error, 1)
	go func() { _, e := CommitStaged(ctx, Deps{Git: git.New(repo), Manifest: m}, config.Defaults()); done <- e }()
	time.Sleep(150 * time.Millisecond)
	commitRaw(t, repo, "concurrent commit") // --allow-empty: HEAD moves, INDEX UNCHANGED (I1 still holds)
	concurrent := headSHA(t, repo)
	err := <-done
	var ce *CASError
	if !errors.As(err, &ce) || !errors.Is(err, ErrCASFailed) {
		t.Fatalf("CAS: want *CASError/ErrCASFailed, got %T %v", err, err)
	}
	if ce.Expected != parent || ce.Actual != concurrent {
		t.Errorf("CAS context: expected=%q actual=%q; want %q/%q", ce.Expected, ce.Actual, parent, concurrent)
	}
	return err, concurrent // ← I2 wants the concurrent commit (orchestrator did NOT land/force)
}
```

### Integration Points

```yaml
TEST SUITE (PRD §20.1 layer 3 + §20.2):
  - the new internal/generate/invariants_test.go is picked up AUTOMATICALLY by `go test ./...` and
    `make test` — no CI/Makefile change. It is CI-runnable on the full {linux,macos,windows} matrix
    (no //go:build constraint; no real signals; stub+git are cross-platform).

PRODUCTION CODE (frozen — read-only dependency):
  - the suite drives generate.CommitStaged (P1.M3.T4.S2) and consumes *RescueError/*CASError (same
    package). It does NOT modify generate.go, stubtest, git, or signal. If an assertion FAILS, the bug
    is in the orchestrator — but THIS task only writes the test; fixing orchestrator bugs is a separate
    work item. (Expect all-green: the invariants hold by construction; this is a guard, not a discovery.)

COVERAGE (PRD §20.3 ≥85% on internal/generate):
  - this suite ADDS coverage (more failure-path exercise of CommitStaged). It can only help the gate.

PARALLEL COORDINATION (P1.M5.T1.S2 — real-agent integration scaffold, Planned):
  - S2 adds a //go:build integration_real suite (opt-in, STAGECOACH_RUN_REAL=1). It does NOT touch
    internal/generate/invariants_test.go. Zero overlap. The two are complementary: S1 = stub invariants
    (CI), S2 = real-agent smoke (manual).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After creating the file — fix before proceeding.
gofmt -w internal/generate/invariants_test.go
go vet ./internal/generate/
gofmt -l internal/generate/   # must be empty

# Expected: zero errors. gofmt -l empty for internal/generate/. go vet clean.
# If go vet reports a redeclared-helper error → you re-declared a generate_test.go helper; REMOVE your copy.
```

### Level 2: The Suite (the core deliverable)

```bash
# Targeted — the new invariant suite (fast feedback loop):
go test -race ./internal/generate/ -run TestInvariants -v
# Expected: PASS for EVERY subtest:
#   TestInvariants/ParseFail, /DuplicateExhaustion, /AgentNonzeroExit (always run),
#   /Timeout, /SigintContextCancel, /CASFailure (run unless -short).
# Each subtest asserts I1 (idempotent index) + I2 (atomic HEAD) + I3 (snapshot immutability).

# Short mode — fast subtests run, slow ones SKIP cleanly:
go test -race -short ./internal/generate/ -run TestInvariants -v
# Expected: ParseFail/DuplicateExhaustion/AgentNonzeroExit PASS; Timeout/SigintContextCancel/CASFailure SKIP.
```

### Level 3: Regression & Whole-Tree (System Validation)

```bash
# The generate package (shared helpers + existing per-scenario tests must stay green):
go test -race ./internal/generate/ -v
# Expected: ALL green — the new suite + the existing TestCommitStaged_* tests. No helper collision,
# no global-state cross-talk (no t.Parallel added).

# Whole tree (the CI gate — proves no cross-package regression; safe, no production-code changes):
make test            # == go test -race ./...
# Expected: green.

# Build sanity (the test file compiles into the test binary):
go build ./...
# Expected: succeeds.
```

### Level 4: Coverage & Audit (confidence, no file change)

```bash
# Coverage on the core package (PRD §20.3 ≥85%; this suite ADDS coverage of CommitStaged failure paths):
make coverage        # go test -coverprofile=coverage.out ./... + go tool cover -func=coverage.out
# Expected: internal/generate coverage ≥ prior baseline (likely higher). Confirm CommitStaged's failure
# branches (timeout/cancel/CAS/loop-exhaustion) are now exercised by BOTH the old tests and the new suite.

# Audit — confirm the three invariants are uniformly asserted:
grep -n "I1\|I2\|I3\|assertInvariants" internal/generate/invariants_test.go
# Expected: assertInvariants called once per subtest (via the harness); I1/I2/I3 each asserted inside it.

# Audit — confirm scope (NO production-code edits):
git status --short
# Expected: ONLY `?? internal/generate/invariants_test.go` (untracked new file). Nothing else modified.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l internal/generate/` empty; `go vet ./internal/generate/` clean.
- [ ] Level 2: `go test -race ./internal/generate/ -run TestInvariants -v` green for ALL subtests.
- [ ] Level 2: `go test -race -short … -run TestInvariants -v` — fast subtests pass, slow ones SKIP.
- [ ] Level 3: `go test -race ./internal/generate/ -v` green (no regression in shared-helper tests).
- [ ] Level 3: `make test` green; `go build ./...` succeeds.
- [ ] Level 4: `make coverage` shows internal/generate ≥ prior baseline; `git status` shows ONLY the new file.
- [ ] No new `go.mod` dependencies (stdlib + existing internal imports only).

### Feature Validation

- [ ] `TestInvariants` has a passing subtest for each NAMED failure mode: ParseFail, Timeout,
      SigintContextCancel, CASFailure (+ bonus DuplicateExhaustion, AgentNonzeroExit).
- [ ] EVERY subtest asserts all THREE §20.2 invariants via the shared `assertInvariants` (I1+I2+I3).
- [ ] I3 (snapshot immutability) is asserted for EVERY subtest — including CAS (`*CASError.TreeSHA`):
      `cat-file -t == tree` and `cat-file -p` stable after staging extra content.
- [ ] CAS subtest: HEAD == the externally-moved concurrent commit (atomic-HEAD special case); index
      unchanged (empty commit doesn't touch it).
- [ ] SIGINT subtest: driven by context-cancel (NOT a real os.Signal); returns `*RescueError{ErrRescue}`.

### Code Quality Validation

- [ ] `package generate` (internal test) — REUSES initRepo/writeFile/stageFile/commitRaw/headSHA/gitOut/runGit
      from generate_test.go; NONE redeclared.
- [ ] `assertInvariants` is the single source of the property — every scenario funnels through it.
- [ ] NO `t.Parallel()` (signal package global singleton; match existing serial tests).
- [ ] Slow subtests (Timeout/SigintContextCancel/CASFailure) gate on `testing.Short()`.
- [ ] NO production-code edits; NO new packages; NO go.mod changes; NO CI/Makefile changes.

### Documentation & Deployment

- [ ] File-level package doc comment on `invariants_test.go` names the PRD refs (§18.1, §18.2, §20.2)
      and states this is the executable form of the §20.2 property.
- [ ] Each helper/scenario has a doc comment tying it to the §20.2 invariant / §18.2 failure mode it covers.
- [ ] No new env vars / config keys / CLI flags (test infrastructure only).

---

## Anti-Patterns to Avoid

- ❌ **Don't edit production code.** This is a TEST-ONLY task. generate.go / stubtest / git / signal are
  READ-ONLY and COMPLETE. If an assertion fails, the bug is in the orchestrator — but fixing it is a
  DIFFERENT work item. (Expect all-green: the invariants hold by construction.)
- ❌ **Don't redeclare the generate_test.go helpers.** That file is `package generate` (internal test).
  Re-declaring `initRepo`/`headSHA`/etc. → "redeclared in this block" compile error. Reuse them.
- ❌ **Don't add a pre-snapshot-failure subtest** (e.g. nothing-to-commit). §20.2 invariants are about
  POST-snapshot paths (those with a non-empty TreeSHA). A pre-snapshot failure has no tree → I3 is meaningless.
- ❌ **Don't send a real os.Signal to test SIGINT.** The signal package isn't installed in this suite
  (its wrappers no-op); the orchestrator-level SIGINT path is `context.Canceled` → `RescueError{ErrRescue}`.
  Simulate with a cancelled context. (The real SIGINT→os.Exit(3) CLI path is already covered by
  signal_integration_test.go — don't duplicate it.)
- ❌ **Don't use `t.Parallel()`.** The signal package keeps a process-global singleton; existing
  CommitStaged tests run serially. Parallel subtests risk global-state cross-talk.
- ❌ **Don't special-case I3 per scenario.** Funnel EVERY subtest through `assertInvariants`, which owns
  I1+I2+I3 uniformly. A scenario that skips I3 silently drops the snapshot-immutability guard.
- ❌ **Don't run `git gc`/`git prune` in the test.** The dangling snapshot tree must persist for the
  post-run `cat-file -p`; git's default gc.auto threshold is never hit in a test, so leave it alone.
- ❌ **Don't delete/modify the existing per-scenario tests** in generate_test.go. They pin error-type
  (`errors.Is`) shapes the invariant suite treats as secondary. The two suites coexist.
