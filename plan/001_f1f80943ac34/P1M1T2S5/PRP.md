---
name: "P1.M1.T2.S5 — UpdateRefCAS (3-arg compare-and-swap, never force)"
description: |
  Replace the `(*gitRunner).UpdateRefCAS` panic-stub (landed by P1.M1.T2.S1) with the real
  implementation — the atomic HEAD-publish primitive that is the FINAL step of Stagecoach's
  snapshot-based commit flow (PRD §13.2 step 3, §18.1 "the invariant"). Signature is fixed by the
  `Git` interface (already landed): `UpdateRefCAS(ctx, ref, newSHA, expectedOld string) error` —
  note the `ref` parameter (the interface is authoritative over the work-item prose's
  `(ctx, newSHA, expectedOld)`, exactly as S4's `parents []string` was authoritative over its prose
  `parentSHA`). It delegates to S1's `run()` helper (NOT S4's `runWithInput` — update-ref takes no
  stdin) with args `update-ref <ref> <newSHA> <expectedOld>` (the 3-arg compare-and-swap form; the
  2-arg force form is FORBIDDEN — PRD §13.2, §18.1). On exit 0 → return nil (HEAD atomically
  advanced). On exit ≠ 0 (CAS mismatch: HEAD moved concurrently, or all-zeros-expected on a repo
  that already has commits, or — impossibly in flow — a bad newSHA) → return a wrapped sentinel
  `ErrCASFailed` so the orchestrator can detect it via `errors.Is` and emit PRD §13.5's "HEAD moved
  from <expected> to <actual>" message. Infrastructural failures (missing git binary / cancelled
  context) are returned UNWRAPPED (not ErrCASFailed) so they stay distinguishable. Introduces ONE
  exported package-level `var ErrCASFailed` (S5 is the first subtask to add a sentinel error) and
  ZERO new imports (`errors`, `fmt`, `strings` all already present in git.go). Adds ONE new test
  file `internal/git/updateref_test.go` (package git) covering CAS success, stale-expected failure
  (the core race — simulated by moving HEAD between capture and update), root-commit publish via
  all-zeros on an unborn repo, all-zeros-on-born failure, git-missing, and context-cancelled, plus
  four `cas`-prefixed fixture helpers (distinct names so they don't collide with S4's planned
  `committree_test.go` helpers when both land). Also removes the single `UpdateRefCAS` line from
  `git_test.go`'s `TestStubsPanic` (required consequence of making the method real — mirrors S2/S3/S4).
  Touches ONLY `internal/git/`; no interface, struct, `run()`, `runWithInput`, RevParseHEAD,
  WriteTree, or CommitTree changes.
---

## Goal

**Feature Goal**: Implement the fourth real git plumbing method on `*gitRunner` — `UpdateRefCAS` —
the atomic compare-and-swap ref-update that is the **sole** point at which Stagecoach mutates a ref
(PRD §18.1: *"The repository's refs and index are modified only at the final `update-ref` step, and
only if HEAD is unchanged since the snapshot"*). It takes a `ref` ("HEAD"), a `newSHA` (the dangling
commit object SHA from `CommitTree`, P1.M1.T2.S4), and an `expectedOld` (the `PARENT_SHA` captured
before generation by `RevParseHEAD`, P1.M1.T2.S2 — or the all-zeros hash when `isUnborn`), and runs
`git -C <repo> update-ref <ref> <newSHA> <expectedOld>` (the 3-arg CAS form; git takes the ref lock,
reads the current value, and writes `<newSHA>` only if current == `<expectedOld>`). On success HEAD is
atomically advanced to `newSHA` and the method returns nil. On any non-zero exit (the CAS did not
match — HEAD moved concurrently) it returns a wrapped sentinel error `ErrCASFailed` that the
generate orchestrator (P1.M3.T4) detects via `errors.Is` to print PRD §13.5's abort message and exit
1. The 2-arg force form is NEVER used (PRD §13.2, §18.1, §18.2).

**Deliverable**:
1. **MODIFY** `internal/git/git.go`: (a) add the exported package-level `var ErrCASFailed` sentinel
   (with doc comment); (b) replace the `UpdateRefCAS` panic-stub body with the ~10-line body that
   delegates to `run()` and branches `err`-first then `code != 0` (exact body in §Blueprint). NO
   import changes — `errors`, `fmt`, `strings` are all already imported.
2. **CREATE** `internal/git/updateref_test.go` (`package git`): six test functions
   (`TestUpdateRefCAS_Success`, `TestUpdateRefCAS_StaleExpected`, `TestUpdateRefCAS_RootCommit`,
   `TestUpdateRefCAS_AllZerosOnBornRepo`, `TestUpdateRefCAS_GitBinaryMissing`,
   `TestUpdateRefCAS_ContextCancelled`) plus four `cas`-prefixed fixture helpers (`casCommit`,
   `casHEAD`, `casMoveHEAD`, `casOut`) and a `gitIdentityEnv` helper.
3. **MODIFY** `internal/git/git_test.go`: remove the single
   `assertPanics(t, "UpdateRefCAS", …)` line from `TestStubsPanic` (required now that UpdateRefCAS
   is real — the test would otherwise fail expecting a panic that no longer occurs; mirrors S2/S3/S4).

No other files touched. No new dependencies. `go.mod`/`go.sum` unchanged.

**Success Definition**: `go build ./...`, `go vet ./...`, and `gofmt -l internal/git/` are clean;
`go test -race ./internal/git/` exits 0 with the 6 new `UpdateRefCAS` cases passing (plus S1's
`run()` tests, S2's `RevParseHEAD` tests, S3's `WriteTree` tests, S4's `CommitTree` tests, and the
now-trimmed `TestStubsPanic` all still green); on a correct `expectedOld`, `UpdateRefCAS` returns
nil and `git rev-parse HEAD` afterwards equals `newSHA`; on a stale `expectedOld` (HEAD moved
between capture and update), it returns an error for which `errors.Is(err, ErrCASFailed)` is true,
its message contains `"(exit 128)"`, and HEAD is byte-for-byte unchanged; an all-zeros `expectedOld`
on an unborn repo publishes a root commit (nil), while the same all-zeros `expectedOld` on a born
repo returns `ErrCASFailed`; a missing git binary surfaces as a non-nil error mentioning "git binary
not found" for which `errors.Is(err, ErrCASFailed)` is **false** (NOT misreported as a CAS failure);
`run()`, `runWithInput`, the `Git` interface, `RevParseHEAD`, `WriteTree`, and `CommitTree` are
byte-identical to their landed forms.

## User Persona

**Target User**: The P1.M3.T4 `CommitStaged` orchestrator (the primary and sole caller), and —
transitively — the rescue protocol (P1.M3.T3), which fires when this method returns `ErrCASFailed`
(or when generation failed before reaching it).

**Use Case**: After snapshotting the index (`WriteTree` → `TREE_SHA`), capturing the parent
(`RevParseHEAD` → `parentSHA`, `isUnborn`), generating a message, and creating the commit object
(`CommitTree` → `NEW_SHA`), the orchestrator publishes it atomically:
`err := g.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld)`. `expectedOld` is `parentSHA` when born,
or the all-zeros hash when `isUnborn`. On `err == nil` the commit is landed (FR40). On
`errors.Is(err, git.ErrCASFailed)` the orchestrator re-reads HEAD via `RevParseHEAD` to obtain
`<actual>`, prints PRD §13.5's abort message, and exits 1 (FR41/§18.2). On any other `err`
(infrastructural), it propagates.

**User Journey**: `g := git.New(repoPath)` → `parent, isUnborn, _ := g.RevParseHEAD(ctx)` →
`tree, _ := g.WriteTree(ctx)` → (generate `msg`) →
`parents := []string(nil); if !isUnborn { parents = []string{parent} }` →
`newSHA, _ := g.CommitTree(ctx, tree, parents, msg)` →
`expectedOld := parent; if isUnborn { expectedOld = strings.Repeat("0", 40) }` →
`err := g.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld)`.

**Pain Points Addressed**: Atomicity + concurrency safety — the central design argument of PRD §13.1.
A `git commit` would move HEAD non-atomically (racing with concurrent terminal commits and silently
clobbering them). The 3-arg CAS `update-ref` fails cleanly if HEAD moved during the (potentially
tens-of-seconds) generation window, leaving the repo untouched (§18.1) and letting the user recover
manually. The sentinel error lets the orchestrator distinguish this specific, recoverable failure
from all others and emit the precise, actionable message PRD §13.5/§18.2 require.

## Why

- **PRD §13.2 (The plumbing alternative, step 3):** *"`git update-ref HEAD <new-sha> <expected-old-sha>`
  — the two-argument (CAS) form atomically updates `HEAD` to `<new-sha>` only if its current value
  equals `<expected-old-sha>` (i.e., `PARENT_SHA`). If HEAD has moved in the meantime (the user
  committed in another terminal), the update fails cleanly and the repository is untouched."* (Git
  calls this the "three-arg" form because `update-ref <ref> <new> <old>` has three operands; the PRD
  prose's "two-argument" counts the value operands `<new>`/`<old>`. Both mean the CAS form — never
  force.) This subtask IS that primitive.
- **PRD §9.9 / FR40 (Commit creation):** *"Advance HEAD atomically: `git update-ref HEAD <NEW_SHA>
  <PARENT_SHA>` (the two-arg form refuses to move HEAD if its current value is not `<PARENT_SHA>`)."*
  This method implements that refusal: exit ≠ 0 ⇒ `ErrCASFailed`.
- **PRD §9.9 / FR41:** *"If `update-ref` fails (HEAD moved concurrently), abort with a clear message
  and a manual recovery command. Do not force-update."* The sentinel `ErrCASFailed` is what lets the
  orchestrator detect the failure and emit that message; the implementation never falls back to the
  2-arg force form.
- **PRD §18.1 (The invariant):** *"Every code path that does not reach a successful `update-ref`
  leaves the repository byte-for-byte unchanged (modulo harmless dangling objects)."* `UpdateRefCAS`
  is the ONLY mutation site. A CAS failure leaves HEAD untouched (verified empirically: after a
  failed CAS, `git rev-parse HEAD` is unchanged).
- **PRD §18.2 (Failure modes):** the table row *"`update-ref` CAS failure (HEAD moved) | commit |
  print message + manual recovery (do NOT force) | Exit 1"* — this method produces the signal
  (`ErrCASFailed`) that row depends on.
- **PRD §13.5 (Edge cases):** *"Rootless repo (no commits yet): `PARENT_SHA` is empty. … `update-ref
  HEAD <new>` is called without the expected-old argument. Handled."* The all-zeros-hash formulation
  (FINDING 3 / git_plumbing_reference §3) is the *uniform* way to express "expected-old = unborn":
  pass `expectedOld = 000…0` (40 zeros for sha-1); the CAS succeeds only if HEAD is truly unborn.
  This avoids a special-cased "drop the 3rd arg" code path — one CAS code path handles both born and
  unborn (decision D7). Verified empirically: all-zeros + unborn ⇒ exit 0; all-zeros + born ⇒ exit 128.
- **`critical_findings.md` FINDING 3:** *"CAS failure messages vary by version/scenario — do NOT
  substring-match. Treat exit ≠ 0 as the CAS-failure signal."* Verified (see research §2): the
  exit code is **128** (NOT 1 as `git_plumbing_summary.md`'s cheat-sheet claims — a documented
  discrepancy), and there are ≥3 distinct stderr phrasings. The implementation branches on `code != 0`
  and embeds (but never matches) the stderr.
- **`git_plumbing_reference.md` §3:** the canonical CAS pattern — 3-arg form, all-zeros = unborn,
  atomic under `.git/<ref>.lock`, never force.
- **`git_plumbing_summary.md` Atomic Commit Sequence:** step 6 = `git update-ref HEAD NEW PARENT` →
  atomic publish (fails if HEAD moved); the final, sole mutation in the 7-step atomic core.
- **Foundation for P1.M3.T4 / P1.M3.T3:** the `CommitStaged` orchestrator (P1.M3.T4) calls this
  method and branches on `ErrCASFailed`; the rescue protocol (P1.M3.T3) is triggered by it. Both are
  blocked on a correct `UpdateRefCAS` + a detectable sentinel. This is the fourth of the 11 interface
  methods to leave stub-status.

## What

`UpdateRefCAS` builds `update-ref <ref> <newSHA> <expectedOld>`, delegates to `run()`, and translates
the four-tuple into `error`:

- `run()` returns `err != nil` (LookPath miss / context cancelled / start failure; `exitCode == -1`)
  → return `err` **unchanged** (NOT wrapped in `ErrCASFailed`). This is the ONLY path that returns a
  non-sentinel error — so a missing git binary is never misreported as a CAS failure.
- `exitCode != 0` (128 on git 2.x — CAS mismatch / all-zeros-on-born / bad newSHA) → return
  `fmt.Errorf("%w (exit %d): %s", ErrCASFailed, code, strings.TrimSpace(stderr))`. Detection is by
  `errors.Is(err, ErrCASFailed)`; the wrapped string carries the exit code + git's own stderr for
  diagnostics (verbose mode / logs), but is NEVER substring-matched.
- `exitCode == 0` → return `nil`. HEAD atomically advanced to `newSHA`.

No porcelain, no `git commit`, no 2-arg force form, no re-reading of HEAD inside the method (the
orchestrator does that for the `<actual>` in the abort message — decision D5), no SHA-format
validation in production code.

### Success Criteria

- [ ] `internal/git/git.go` defines exported `var ErrCASFailed = errors.New(...)` with the doc
      comment in §Blueprint (the ONE new symbol this subtask introduces).
- [ ] `(*gitRunner).UpdateRefCAS` body matches §Implementation Blueprint verbatim (no `panic`);
      delegates to `run()` (NOT `runWithInput`); branches `err`-first, then `code != 0`.
- [ ] `ErrCASFailed` is the ONLY thing wrapped on the `code != 0` branch; infrastructural errors
      (`run` err != nil) are returned UNWRAPPED.
- [ ] NO new imports in git.go (`errors`, `fmt`, `strings` already present).
- [ ] `run()`, `runWithInput`, `New`, `gitRunner`, the `Git` interface, `FileChange`,
      `StagedDiffOptions`, `RevParseHEAD`, `WriteTree`, `CommitTree`, and the other 7 stubs are
      byte-identical to their landed forms.
- [ ] `internal/git/updateref_test.go` exists in `package git` with the 6 named test functions and
      the 4 `cas`-prefixed helpers + `gitIdentityEnv`.
- [ ] `git_test.go`'s `TestStubsPanic` no longer contains the `UpdateRefCAS` line (removed; 7 stubs
      remain).
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go test -race ./internal/git/` exits 0; all 6 `TestUpdateRefCAS_*` pass and S1/S2/S3/S4's
      tests still pass.
- [ ] `TestUpdateRefCAS_Success` asserts `err==nil` AND `git rev-parse HEAD == newSHA` afterwards.
- [ ] `TestUpdateRefCAS_StaleExpected` asserts `errors.Is(err, ErrCASFailed)`, the message contains
      `"(exit 128)"`, AND HEAD is unchanged after the failed call.
- [ ] `TestUpdateRefCAS_GitBinaryMissing` asserts `!errors.Is(err, ErrCASFailed)` (infrastructural
      failure NOT misreported as CAS failure).
- [ ] NO shell execution and NO `cmd.Dir`/`os.Chdir` in production code (inherited from S1).
- [ ] `go.mod`/`go.sum` unchanged; no new dependencies.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives: the exact module path; the exact three files to touch (and
the exact single line to remove from `git_test.go`); the exact `run()` contract (signature + the
`err==nil`-for-non-zero-exits invariant that the `code != 0` branch relies on); the exact
`ErrCASFailed` declaration and the exact `UpdateRefCAS` body (verified-equivalent to throwaway
invocations run against git 2.54.0); the empirically-pinned `update-ref` behavior (success ⇒ exit 0;
every failure ⇒ exit **128**, NOT 1 — a documented discrepancy with `git_plumbing_summary.md`'s
cheat-sheet; ≥3 distinct stderr phrasings; all-zeros + unborn ⇒ exit 0, all-zeros + born ⇒ exit 128;
2-arg force always wins); the exact 6 test cases with verified assertions and the 4 helpers; and the
exact validation commands with expected results. No inference required.

### Documentation & References

```yaml
# MUST READ — the authoritative context
- file: PRD.md
  why: "§13.2 step 3 (update-ref <new> <old> CAS — refuses if HEAD moved; this method IS it);
        §9.9/FR40 (advance HEAD atomically via the CAS form); §9.9/FR41 (on failure, abort + manual
        recovery, do NOT force); §13.5 (rootless repo: expected-old omitted / all-zeros); §18.1
        (the invariant: refs modified ONLY at update-ref); §18.2 (CAS-failure row → exit 1);
        §19 (security: args as []string, never sh -c) is inherited and must not be violated."
  critical: "This subtask owns ONLY the ErrCASFailed var + UpdateRefCAS body + its tests + the
             one-line TestStubsPanic edit. Do NOT implement DiffTree (S6), the rescue protocol
             (P1.M3.T3), the orchestrator (P1.M3.T4), or any other method. Do NOT change the Git
             interface (already correct from S1). Do NOT modify run() or runWithInput (S2/S3/S4
             forbid it)."

- docfile: plan/001_f1f80943ac34/architecture/critical_findings.md
  why: "FINDING 3 — THE finding for this subtask: CAS failure messages vary by version AND scenario
        (HEAD-moved vs all-zeros-on-born vs bad-sha all differ); do NOT substring-match; treat exit
        ≠ 0 as the CAS-failure signal. FINDING 1 (unborn HEAD) is the sibling finding that explains
        why the all-zeros expected-old represents 'unborn' and composes with RevParseHEAD's isUnborn."
  critical: "FINDING 3 is THE reason UpdateRefCAS branches on `code != 0` (not `code == 1` and not a
             stderr match) and wraps a sentinel rather than returning a plain fmt.Errorf. Research §2
             proves the real exit is 128 and enumerates the stderr variants."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_reference.md
  why: "§3 (git update-ref <ref> <new> [<expected-old>]) documents: 2-arg = unconditional force
        (DANGEROUS, forbidden); 3-arg = CAS (lock + compare inside one git process under .git/ref.lock
        → atomic); all-zeros SHA = 'ref does not exist yet' (unborn), so root commit uses expected-old
        = all-zeros and succeeds only if HEAD is truly unborn. The canonical Go pattern
        (exec.CommandContext + update-ref HEAD new old; on exit != 0 do NOT force)."
  critical: "§3 establishes the 3-arg CAS contract, the all-zeros-as-unborn semantics, and the
             'never force-update' rule. Confirms atomicity is guaranteed by git's own ref lock —
             Stagecoach adds no locking of its own."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_summary.md
  why: "The Atomic Commit Sequence (step 6 = update-ref HEAD NEW PARENT → atomic publish, fails if
        HEAD moved) and the Exit-Code Cheat Sheet row for update-ref."
  critical: "The cheat-sheet claims CAS mismatch = exit 1. THIS IS WRONG on git 2.54.0 — it is exit
             128 (research §2.1, decision D2). Do NOT branch on code == 1. Branch on code != 0. The
             cheat-sheet is otherwise useful for the sequence framing; treat its exit-1 cell as a typo."

- docfile: plan/001_f1f80943ac34/P1M1T2S1/PRP.md
  why: "The CONTRACT for everything UpdateRefCAS consumes: the gitRunner struct; the run() helper
        (exact signature and verified body — which this subtask does NOT modify, only calls); the
        UpdateRefCAS panic-stub being replaced; the Git interface (signature already correct:
        ctx, ref, newSHA, expectedOld string); New(); the git_test.go initRepo(t,dir) helper; and the
        assertPanics helper."
  critical: "Treat as already-landed. run() returns (stdout, stderr, exitCode, err) with err==nil
             for non-zero exits and exitCode==-1 for infrastructural failures. S1 gotcha G2 (non-zero
             exit is not a Go error) is the foundation UpdateRefCAS's `code != 0` branch relies on."

- docfile: plan/001_f1f80943ac34/P1M1T2S4/PRP.md
  why: "The CONTRACT for the IMMEDIATE upstream producer: CommitTree returns the NEW_SHA that
        UpdateRefCAS publishes, and it introduced runWithInput + the 'io' import (both landed).
        Confirms S4's body and the TestStubsPanic one-line-removal pattern that S5 mirrors for
        UpdateRefCAS. Also confirms UpdateRefCAS delegates to run() (NOT runWithInput) because
        update-ref takes no stdin — S4's stdin need is specific to commit-tree's -F -."
  critical: "S4 is landing/landed concurrently. S4's git.go edit is the CommitTree body + runWithInput
             + the io import; S5's git.go edits are (a) the ErrCASFailed var, (b) the UpdateRefCAS
             body — DISTINCT regions. S4's git_test.go edit removes the CommitTree line; S5's removes
             the UpdateRefCAS line — distinct lines. S4's test helpers (setIdentityConfig, writeFile,
             stageFile, writeTreeOf, headSHA, commitMessage) are name-collision risks for S5's
             helpers — S5 uses a `cas` prefix throughout to avoid them (research §8, decision D10)."

- docfile: plan/001_f1f80943ac34/P1M1T2S2/PRP.md
  why: "The CONTRACT for the upstream that produces expectedOld: RevParseHEAD returns parentSHA +
        isUnborn. S2's revparse_test.go defines minGitEnv() and makeEmptyCommit(t,dir,msg), which
        UpdateRefCAS's tests reuse without redeclaring. S2 also established the err-first / code-branch
        method pattern that UpdateRefCAS mirrors."
  critical: "S2's Level-2 note documents the EXACT TestStubsPanic-edit pattern (remove the now-real
             method's line) — apply it for UpdateRefCAS. makeEmptyCommit creates commits via
             `git commit --allow-empty` with identity in ITS OWN command's env (not process env) —
             S5's casCommit helper follows the same explicit-env pattern for the dangling commit-tree
             objects it creates."

- docfile: plan/001_f1f80943ac34/P1M1T2S5/research/updateref_cas_validation.md
  why: "THIS subtask's own research: the interface-vs-prose signature reconciliation (§1), the
        empirically-pinned update-ref behavior on git 2.54.0 incl. the exit-128-not-1 discrepancy
        (§2), the typed-error design (sentinel ErrCASFailed, why not a struct, §3), the all-zeros
        handling (§4), the run()-not-runWithInput decision (§5), the empirical transcript (§6), the
        test design matrix (§7), the helper-name collision avoidance with S4 (§8), and the decisions
        log D1–D10 (§9)."
  critical: "§2 (exit 128 + stderr variants) and §3 (the sentinel design + the 'do not re-read HEAD
             here' decision D5) are the two non-obvious design calls an implementing agent would
             otherwise guess at. §8 (cas-prefix naming) is what keeps the package compiling when S4
             and S5 land together."

# External references (exact, anchor-level)
- url: https://git-scm.com/docs/git-update-ref#_description
  why: "Documents the three forms: `git update-ref <ref> <new-value>` (force/unconditional),
        `git update-ref <ref> <new-value> <old-value>` (CAS — 'without <old-value>, it is equivalent
        to ... a forced update'); and the all-zeros semantics ('a <new-value> of 000000000... sets the
        ref to be deleted ... and <old-value> of 0000... checks for an unborn ref'). Confirms the 3-arg
        CAS contract and the all-zeros-as-unborn rule."
  critical: "Establishes the 3-arg CAS form is the safe primitive and that all-zeros expected-old ⟺
             'ref must be unborn' (root commit). The 2-arg form is documented as a FORCED update —
             exactly what PRD §13.2/§18.1 forbid."
- url: https://git-scm.com/docs/git-update-ref#_options
  why: "Documents that update-ref takes the ref lock under .git/<ref>.lock and performs the
        read-compare-write atomically within one process — the atomicity guarantee Stagecoach relies on
        (it adds no locking of its own)."
  critical: "Confirms atomicity w.r.t. concurrent git writers is git's responsibility, not Stagecoach's."
- url: https://pkg.go.dev/errors#Is
  why: "errors.Is unwraps wrapped errors; fmt.Errorf with the %w verb creates a wrapping error whose
        Is/As chain includes the wrapped value. This is HOW the orchestrator detects ErrCASFailed via
        errors.Is(err, git.ErrCASFailed) even though the returned error also carries exit-code/stderr
        context."
  critical: "Use `%w` (NOT `%s`/`%v`) when wrapping ErrCASFailed, or errors.Is will return false and
             the orchestrator's branch will silently miss CAS failures."
- url: https://pkg.go.dev/fmt#Errorf
  why: "The %w verb (Go 1.13+) wraps an error for errors.Is/errors.As while still formatting the
        message string. UpdateRefCAS uses `fmt.Errorf(\"%w (exit %d): %s\", ErrCASFailed, code, stderr)`."
  critical: "%w must be the LAST verb mapping to the error argument; the formatted message is human-
             readable and carries the exit code + trimmed stderr for diagnostics."
```

### Current Codebase Tree (after S1 + S2 + S3 + S4 have landed — verified on disk)

```bash
stagecoach/
├── PRD.md
├── go.mod                # module github.com/dustin/stagecoach, go 1.22, NO deps
├── Makefile              # build/test/lint/coverage/install/clean (test = go test -race ./...)
├── cmd/stagecoach/main.go # stub
├── internal/
│   └── git/
│       ├── git.go        # S1: interface+gitRunner+run()+New()+stubs; S2: RevParseHEAD real;
│       │                 #   S3: WriteTree real; S4: runWithInput+CommitTree real+io import
│       │                 # imports: bytes, context, errors, fmt, io, os/exec, strings  ← ALL S5 needs present
│       ├── git_test.go   # S1: TestNew/TestRun_*/TestStubsPanic(8 stubs, incl UpdateRefCAS) + initRepo + assertPanics
│       ├── revparse_test.go  # S2: 4 TestRevParseHEAD_* + minGitEnv + makeEmptyCommit
│       ├── writetree_test.go # S3: 5 TestWriteTree_* + makeMergeConflict
│       └── committree_test.go # S4 (landing concurrently): 6 TestCommitTree_* + setIdentityConfig/writeFile/...
└── (other empty internal/ dirs, pkg/, providers/, docs/ — untouched by this subtask)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/
    └── git/
        ├── git.go              # MODIFIED — +var ErrCASFailed; UpdateRefCAS stub → real body. NO import change.
        ├── git_test.go         # MODIFIED — remove the ONE `UpdateRefCAS` line from TestStubsPanic
        ├── revparse_test.go    # UNCHANGED (S2's file; minGitEnv/makeEmptyCommit reused, not edited)
        ├── writetree_test.go   # UNCHANGED (S3's file)
        ├── committree_test.go  # UNCHANGED (S4's file; landing concurrently — distinct helper names)
        └── updateref_test.go   # NEW — package git; 6 TestUpdateRefCAS_* + casCommit/casHEAD/casMoveHEAD/casOut + gitIdentityEnv
```

**File responsibilities:**

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | MODIFY | (1) Add exported `var ErrCASFailed` (with doc comment). (2) Replace the `UpdateRefCAS` panic-stub with the `run()`-delegating body. No import changes. |
| `internal/git/git_test.go` | MODIFY | Remove the single `assertPanics(t, "UpdateRefCAS", …)` line from `TestStubsPanic`. Nothing else. |
| `internal/git/updateref_test.go` | CREATE | `package git` tests for `UpdateRefCAS`: success / stale-expected / root-commit / all-zeros-on-born / git-missing / ctx-cancelled, plus the `cas`-prefixed fixtures. |

**Explicitly NOT created/modified:** `run()`, `runWithInput`, `New`, `gitRunner`, the `Git`
interface, `FileChange`, `StagedDiffOptions`, `RevParseHEAD` (S2), `WriteTree` (S3), `CommitTree`
(S4), the other 7 method stubs (DiffTree, StagedDiff, HasStagedChanges, RecentMessages,
RecentSubjects, CommitCount, AddAll), `revparse_test.go` (S2), `writetree_test.go` (S3),
`committree_test.go` (S4), `go.mod`/`go.sum`, the `Makefile`, anything under `cmd/`/`pkg/`/other
`internal/*`, `plan/`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — the CAS-failure exit code is 128, NOT 1): git_plumbing_summary.md's cheat-sheet
// lists "CAS mismatch (Exit 1)" for update-ref. Empirically (git 2.54.0, research §2) EVERY failure
// mode returns 128: stale-expected ("is at <actual> but expected <expected>"), all-zeros-on-born
// ("reference already exists"), and bad-newSHA ("nonexistent object"). The cheat-sheet's exit-1 cell
// is a typo (likely by analogy with `diff --quiet`, which genuinely uses exit 1). This is EXACTLY the
// trap FINDING 3 warns about. Resolution: branch on `code != 0`. Do NOT write `code == 1` or
// `code == 128`. (Decision D2.)

// CRITICAL (G2 — do NOT substring-match the stderr): FINDING 3 + research §2 show ≥3 distinct
// stderr phrasings that also vary by git version. DETECTION is `code != 0` (stable). The trimmed
// stderr is still EMBEDDED in the error message (for verbose/logs), but the orchestrator must detect
// via errors.Is(err, ErrCASFailed), NEVER strings.Contains(stderr, ...). (Decision D3.)

// CRITICAL (G3 — wrap with %w, NOT %s/%v): the orchestrator detects CAS failure via
// errors.Is(err, git.ErrCASFailed). This works ONLY if the failure branch wraps the sentinel with
// the %w verb: `fmt.Errorf("%w (exit %d): %s", ErrCASFailed, code, stderr)`. Using %s/%v would lose
// the Is/As chain and the orchestrator would silently treat every CAS failure as a generic error
// (wrong exit code, wrong message). TestUpdateRefCAS_StaleExpected guards this via errors.Is.

// CRITICAL (G4 — err checked BEFORE code, infrastructural errors UNWRAPPED): run() guarantees
// err != nil ⟹ exitCode == -1 (LookPath / context / start failure) and err == nil for every real git
// exit (0, 128, …). So `if err != nil { return err }` is the authoritative infrastructural-failure
// guard and MUST come first. The returned err there is NOT wrapped in ErrCASFailed — a missing git
// binary is not a CAS failure and the orchestrator must distinguish them. Only when err == nil does
// `code != 0` decide CAS-failure-vs-success. TestUpdateRefCAS_GitBinaryMissing guards against
// regressing this (it asserts !errors.Is(err, ErrCASFailed)). (Decision D6.)

// CRITICAL (G5 — the interface is `ref, newSHA, expectedOld`, NOT `(newSHA, expectedOld)`): the
// work-item CONTRACT prose writes UpdateRefCAS(ctx, newSHA, expectedOld). But the Git interface
// (S1, ALREADY LANDED) is UpdateRefCAS(ctx, ref, newSHA, expectedOld string) error. The interface is
// authoritative (mirrors S4's parents []string over prose parentSHA). Forward `ref` verbatim as the
// first update-ref operand; do NOT hardcode "HEAD". The orchestrator passes "HEAD". (Decision D1.)

// CRITICAL (G6 — NEVER the 2-arg force form): the 2-arg `git update-ref HEAD <new>` unconditionally
// overwrites HEAD — it would silently clobber a concurrent commit (exactly the §13.1 hazard). PRD
// §13.2/§18.1/§18.2 forbid it. The implementation ALWAYS appends <expectedOld> as the 3rd operand.
// There is no fallback path. (Verified empirically: 2-arg force always exits 0 even when HEAD moved.)

// CRITICAL (G7 — the TestStubsPanic edit): git_test.go's TestStubsPanic (after S2+S3+S4) STILL
// includes `assertPanics(t, "UpdateRefCAS", func() { _ = g.UpdateRefCAS(ctx, "ref", "new", "old") })`.
// Once UpdateRefCAS is real (no panic), assertPanics fails with "expected panic, but did not panic".
// Resolution (mirrors S2's RevParseHEAD removal, S3's WriteTree removal, S4's CommitTree removal):
// DELETE that one line. After removal, TestStubsPanic covers the remaining 7 stubs (DiffTree,
// StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll). This is the
// ONLY edit to git_test.go. Document it in the commit message.

// CRITICAL (G8 — ZERO new imports): UpdateRefCAS uses g.run (S1), errors.New (errors — present),
// fmt.Errorf (fmt — present), strings.TrimSpace (strings — present). ErrCASFailed = errors.New(...).
// git.go's import block (bytes, context, errors, fmt, io, os/exec, strings) already has everything.
// Do NOT add an import. The compiler will complain about an unused import if you add one erroneously.

// GOTCHA (G9 — delegate to run(), NOT runWithInput): update-ref reads nothing from stdin, unlike
// commit-tree's -F -. So UpdateRefCAS calls g.run(ctx, g.workDir, "update-ref", ref, newSHA,
// expectedOld). runWithInput exists only for commit-tree's stdin need (S4). Using runWithInput here
// would compile but is semantically wrong (passes a nil-ish reader) and wasteful. (Decision D9.)

// GOTCHA (G10 — the all-zeros hash is the CALLER's responsibility, not UpdateRefCAS's): UpdateRefCAS
// has no isUnborn parameter and no knowledge of unborn repos. It forwards expectedOld to git verbatim.
// When expectedOld is the all-zeros hash, git's CAS treats it as "ref must be unborn" (verified: exit
// 0 on unborn, exit 128 on born). The orchestrator (P1.M3.T4) constructs the all-zeros string when
// RevParseHEAD reported isUnborn. Do NOT special-case all-zeros inside UpdateRefCAS; do NOT add a
// ZeroSHA exported constant (sha-1-specific; would mislead on sha-256 — decision D8).

// GOTCHA (G11 — do NOT re-read HEAD inside UpdateRefCAS): PRD §13.5 wants "<actual>" in the abort
// message. Considered capturing it here (a CASFailedError struct). REJECTED (decision D5): the git
// package is a plumbing layer and must not construct user messages; any captured <actual> is
// immediately stale; and it would couple this unit test to RevParseHEAD. The orchestrator re-reads
// HEAD via RevParseHEAD when it sees ErrCASFailed. UpdateRefCAS just returns the sentinel (wrapped).

// GOTCHA (G12 — test helper names must not collide with S4's): S4 (committree_test.go, landing
// concurrently) plans setIdentityConfig, writeFile, stageFile, writeTreeOf, headSHA, commitMessage.
// S5's helpers use a `cas` prefix: casCommit, casHEAD, casMoveHEAD, casOut, plus gitIdentityEnv.
// All DISTINCT — the package compiles when both land. Do NOT name an S5 helper headSHA/setIdentity/etc.
// (Research §8, decision D10.) REUSE (do not redeclare) initRepo (git_test.go), minGitEnv and
// makeEmptyCommit (revparse_test.go).

// GOTCHA (G13 — simulating the CAS-failure race in a test): a CAS failure requires HEAD to have
// MOVED between capturing expectedOld and calling UpdateRefCAS. Capture expected = casHEAD(dir),
// then casMoveHEAD(dir, otherSHA) [raw 2-arg force update-ref], THEN call UpdateRefCAS with the stale
// expected. Verify err is ErrCASFailed AND casHEAD(dir) is still otherSHA (the failed CAS left it
// untouched — the §18.1 invariant). Do NOT just pass a wrong expected without moving HEAD first —
// that tests the same code path but is less faithful to the real race.

// GOTCHA (G14 — creating a dangling commit object in a fixture needs identity): the success and
// root-commit tests need a NEW_SHA that is a valid but UNPUBLISHED commit object (makeEmptyCommit
// publishes via `git commit`, moving HEAD — unsuitable). The casCommit helper creates one via raw
// `git commit-tree` with identity supplied through the command's env (mirrors S2's makeEmptyCommit
// pattern: append(minGitEnv(), GIT_AUTHOR_*/GIT_COMMITTER_*)). The identity lives ONLY on that
// command — it does not pollute the process env or other tests. (Research §8.)

// GOTCHA (G15 — test file is package git, white-box): UpdateRefCAS is on *gitRunner and ErrCASFailed
// is package-level; the fixture work needs `package git` to call New(), access ErrCASFailed, and
// reuse initRepo/minGitEnv/makeEmptyCommit. Match S1/S2/S3/S4's package (carried from S2 G7 / S3 G9).

// GOTCHA (G16 — no shell, no cmd.Dir in PRODUCTION code): UpdateRefCAS inherits S1's §19 guarantees
// (run() uses exec.CommandContext + []string args + -C repo flag, NOT cmd.Dir / os.Chdir). Do NOT
// introduce exec.Command / os.Chdir / sh -c in git.go. The test fixtures DO use exec.Command directly
// (parallel to S1's initRepo, S2's makeEmptyCommit, S3's makeMergeConflict, S4's fixtures) — that is
// acceptable test-fixture usage ([]string args + cmd.Env, never a shell). The Level-1 grep for sh -c
// / cmd.Dir covers PRODUCTION code (git.go) only.

// GOTCHA (G17 — ErrCASFailed is exported and package-level, not a method-local var): it must be
// visible to the orchestrator in another package (errors.Is(err, git.ErrCASFailed)). Declare it at
// package scope in git.go (place it immediately before the UpdateRefCAS method, after CommitTree).
// It is the FIRST exported symbol the git package adds that isn't a type/method/constructor.
```

## Implementation Blueprint

### Data models and structure

No structs added. The ONE new symbol is an exported sentinel error variable:

```go
// ErrCASFailed is returned by UpdateRefCAS when git's compare-and-swap did not match — i.e. HEAD
// moved concurrently since the snapshot (or expectedOld was the all-zeros hash on a repo that
// already has commits). The orchestrator detects it via errors.Is(err, ErrCASFailed) to emit PRD
// §13.5's "HEAD moved from <expected> to <actual>" message and exit 1 (FR41/§18.2). It is NOT
// returned for infrastructural failures (missing git binary, cancelled context); those propagate the
// underlying error unchanged so they remain distinguishable. The <actual> SHA is re-read by the
// orchestrator via RevParseHEAD when it observes this error (it is deliberately NOT captured here —
// see P1.M1.T2.S5 research §3 / decision D5).
var ErrCASFailed = errors.New("git update-ref: compare-and-swap failed (ref moved since snapshot)")
```

No new options type, no new struct. `run()`'s return shape `(stdout, stderr, exitCode, err)` is
already defined by S1; `UpdateRefCAS` discards `stdout` (update-ref prints nothing on success).

### The `UpdateRefCAS` body (exact — copy verbatim)

This replaces S1's panic-stub of the same name and signature. Place the `ErrCASFailed` var
immediately above it.

```go
// UpdateRefCAS atomically moves ref to newSHA only if ref's current value equals expectedOld — the
// 3-arg compare-and-swap form of git update-ref (git takes the ref lock, reads the current value, and
// writes newSHA only if current == expectedOld, all under .git/<ref>.lock in one process). It is the
// SOLE point at which Stagecoach mutates a ref (PRD §18.1: "refs are modified only at the final
// update-ref step, and only if HEAD is unchanged since the snapshot"). The 2-arg force form is NEVER
// used — it would silently clobber a concurrent commit (PRD §13.1/§13.2/§18.2).
//
// For a root commit (unborn repo), the caller passes expectedOld = the all-zeros hash (40 zeros for
// sha-1); the CAS then succeeds only if HEAD is truly unborn (UpdateRefCAS itself has no isUnborn
// knowledge — the caller, via RevParseHEAD, decides). On any non-zero exit the CAS did not match
// (HEAD moved, or all-zeros-expected on a repo that already has commits): return ErrCASFailed
// (wrapped, so errors.Is works) carrying the exit code + git's own stderr for diagnostics. FINDING 3:
// stderr varies by scenario/version — detection is by exit code (code != 0), NEVER by matching the
// stderr string.
func (g *gitRunner) UpdateRefCAS(ctx context.Context, ref, newSHA, expectedOld string) error {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "update-ref", ref, newSHA, expectedOld)
	_ = stdout // update-ref prints nothing on success; referenced to silence unused-var linters
	if err != nil {
		return err // git binary missing / context cancelled / start failure (run sets code=-1) — UNWRAPPED
	}
	if code != 0 {
		// CAS did not match. Branch on code (!= 0), NOT on a specific exit or stderr text (FINDING 3,
		// gotcha G1/G2). Wrap with %w so errors.Is(err, ErrCASFailed) is true (gotcha G3).
		return fmt.Errorf("%w (exit %d): %s", ErrCASFailed, code, strings.TrimSpace(stderr))
	}
	return nil
}
```

> **Verified:** the args shape (`update-ref <ref> <new> <old>`) and the branch order are confirmed by
> this subtask's research §2/§6 (success ⇒ exit 0 ⇒ nil; every failure ⇒ exit 128 ⇒ ErrCASFailed;
> all-zeros+unborn ⇒ exit 0 ⇒ nil; all-zeros+born ⇒ exit 128 ⇒ ErrCASFailed), re-verified empirically
> on this box (git 2.54.0).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go (two surgical edits — NO import change)
  - EDIT 1 — add the ErrCASFailed sentinel:
      INSERT the `var ErrCASFailed = errors.New(...)` declaration (with its doc comment, verbatim from
      §"Data models and structure") immediately AFTER the CommitTree method's closing brace and BEFORE
      the UpdateRefCAS stub. Keep it package-level and exported (capital E). (gotcha G17.)
  - EDIT 2 — replace the UpdateRefCAS panic-stub:
      FIND the stub:
        func (g *gitRunner) UpdateRefCAS(ctx context.Context, ref, newSHA, expectedOld string) error {
            panic("gitRunner.UpdateRefCAS: not yet implemented — see P1.M1.T2.S5")
        }
      REPLACE with the body in §"The UpdateRefCAS body" above (keep the same signature, add the doc
      comment, delegate to run(), branch err-first then code != 0).
  - DO NOT touch: the import block (all needed symbols already present — gotcha G8), run(),
    runWithInput, New, gitRunner, Git interface, FileChange, StagedDiffOptions, RevParseHEAD (real
    from S2), WriteTree (real from S3), CommitTree (real from S4), or any of the other 7 method stubs
    (DiffTree, StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll).
  - VERIFY: go build ./internal/git/ → exit 0.

Task 2: MODIFY internal/git/git_test.go (one-line removal)
  - FIND inside TestStubsPanic:
      assertPanics(t, "UpdateRefCAS", func() { _ = g.UpdateRefCAS(ctx, "ref", "new", "old") })
  - DELETE that single line. After removal TestStubsPanic covers the remaining 7 stubs: DiffTree,
    StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll.
  - DO NOT touch anything else in git_test.go (initRepo, TestNew, TestRun_*, assertPanics helper,
    the other assertPanics lines).
  - WHY: once UpdateRefCAS is real it no longer panics; assertPanics would fail (gotcha G7). Mirrors
    S2/S3/S4's removals.
  - VERIFY: go test -race -run TestStubsPanic ./internal/git/ → exit 0 (7 stubs still panic).

Task 3: CREATE internal/git/updateref_test.go (package git — white-box)
  - FILE: internal/git/updateref_test.go
  - PACKAGE line: `package git`  (NOT git_test — gotcha G15; matches the other test files)
  - IMPORTS: context, errors, os, os/exec, strings, testing  (all stdlib)
  - WRITE the fixture helpers (exact bodies in research §8; all DISTINCT `cas`-prefixed names so they
    do not collide with S4's committree_test.go helpers when both land — gotcha G12):
      casOut(t, dir string, args ...string) string:
        - raw exec.Command("git", append([]string{"-C", dir}, args...)...); default env; return
          strings.TrimSpace(stdout); on non-zero exit t.Fatalf with CombinedOutput. t.Helper().
          (The shared raw-git stdout helper used by the others.)
      gitIdentityEnv() []string:
        - return append(minGitEnv(),
            "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
            "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com").
          REUSES S2's minGitEnv() (do NOT redeclare). Distinct name from any S4 helper.
      casCommit(t, dir string, parents []string, msg string) string:
        - resolve a tree: if len(parents)>0 → casOut(t, dir, "rev-parse", parents[0]+"^{tree}");
          else → the sha-1 empty-tree SHA "4b825dc642cb6eb9a060e54bf8d69288fbee4904".
        - build args: ["commit-tree"]; for each p: append "-p", p; append "-m", msg; append <tree>.
          (Test messages are trivial/safe; -m is fine in a fixture — the -F - requirement is a
          PRODUCTION concern for user messages, gotcha G14.)
        - run via a raw exec.Command with cmd.Env = gitIdentityEnv() (identity on THIS command only);
          return trimmed stdout (the dangling commit SHA); on error t.Fatalf. t.Helper().
          (Creates an UNPUBLISHED commit object — does NOT move any ref.)
      casHEAD(t, dir string) string:
        - return casOut(t, dir, "rev-parse", "HEAD").  (Distinct from S4's planned `headSHA`.)
      casMoveHEAD(t, dir, sha string):
        - casOut(t, dir, "update-ref", "HEAD", sha).  (Raw 2-arg FORCE update-ref — used ONLY to
          simulate the concurrent HEAD move in the stale-expected test. This is test-fixture usage of
          the force form; PRODUCTION code never uses it — gotcha G6/G16.)
  - WRITE the 6 test functions (assertions in §"Test cases" below):
      TestUpdateRefCAS_Success:
        repo := t.TempDir(); initRepo(t, repo)
        makeEmptyCommit(t, repo, "initial"); makeEmptyCommit(t, repo, "second")  // HEAD = C1
        c1 := casHEAD(t, repo)
        newSHA := casCommit(t, repo, []string{c1}, "feat: third")  // dangling child of C1
        g := New(repo)
        err := g.UpdateRefCAS(context.Background(), "HEAD", newSHA, c1)  // expected == current HEAD
        assert err == nil
        assert casHEAD(t, repo) == newSHA   // HEAD atomically advanced to newSHA
      TestUpdateRefCAS_StaleExpected:
        repo := t.TempDir(); initRepo(t, repo)
        makeEmptyCommit(t, repo, "initial"); makeEmptyCommit(t, repo, "second")  // HEAD = C1
        c0 := casOut(t, repo, "rev-parse", "HEAD~1"); c1 := casHEAD(t, repo)
        newSHA := casCommit(t, repo, []string{c1}, "feat: third")
        // SIMULATE the race: capture expected=c1, then a concurrent commit moves HEAD to c0
        casMoveHEAD(t, repo, c0)
        g := New(repo)
        err := g.UpdateRefCAS(context.Background(), "HEAD", newSHA, c1)  // stale expected (HEAD is c0)
        assert err != nil && errors.Is(err, ErrCASFailed)                 // ← detectable sentinel
        assert strings.Contains(err.Error(), "(exit 128)")                // ← real exit (NOT 1)
        assert casHEAD(t, repo) == c0                                     // ← repo UNCHANGED (§18.1)
      TestUpdateRefCAS_RootCommit:
        repo := t.TempDir(); initRepo(t, repo)                            // unborn (zero commits)
        const zeros = "0000000000000000000000000000000000000000"          // sha-1 all-zeros (40)
        newSHA := casCommit(t, repo, nil, "feat: root")                   // dangling ROOT commit
        g := New(repo)
        err := g.UpdateRefCAS(context.Background(), "HEAD", newSHA, zeros) // all-zeros + unborn
        assert err == nil
        assert casHEAD(t, repo) == newSHA                                 // root commit published
      TestUpdateRefCAS_AllZerosOnBornRepo:
        repo := t.TempDir(); initRepo(t, repo)
        makeEmptyCommit(t, repo, "initial")                              // HEAD exists (born)
        const zeros = "0000000000000000000000000000000000000000"
        c0 := casHEAD(t, repo)
        g := New(repo)
        err := g.UpdateRefCAS(context.Background(), "HEAD", c0, zeros)    // all-zeros + born
        assert err != nil && errors.Is(err, ErrCASFailed)                 // "reference already exists"
        assert casHEAD(t, repo) == c0                                     // unchanged
      TestUpdateRefCAS_GitBinaryMissing:
        t.Setenv("PATH", "")                                              // makes run()'s LookPath("git") fail
        g := New(t.TempDir())                                             // dir need not be a repo
        err := g.UpdateRefCAS(context.Background(), "HEAD", "new", "old")
        assert err != nil && strings.Contains(err.Error(), "git binary not found")
        assert !errors.Is(err, ErrCASFailed)                              // ← NOT misreported as CAS failure
      TestUpdateRefCAS_ContextCancelled:
        ctx, cancel := context.WithCancel(context.Background()); cancel()  // cancel BEFORE call
        g := New(t.TempDir())
        err := g.UpdateRefCAS(ctx, "HEAD", "new", "old")
        assert err != nil && errors.Is(err, context.Canceled)
        assert !errors.Is(err, ErrCASFailed)                              // ← NOT misreported as CAS failure
  - NAMING: TestUpdateRefCAS_<Scenario>; helpers casCommit/casHEAD/casMoveHEAD/casOut/gitIdentityEnv
    (distinct from S1/S2/S3/S4 helpers — gotcha G12).
  - DO NOT redeclare initRepo / minGitEnv / makeEmptyCommit (they live in git_test.go /
    revparse_test.go).
  - VERIFY: go test -race -run 'TestUpdateRefCAS' ./internal/git/ → exit 0, all 6 pass.

Task 4: VALIDATE — full gate set + scope discipline
  - RUN: go build ./... ; go vet ./... ; gofmt -l internal/git/ ; go test -race ./internal/git/
  - RUN: git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go   (expect: no matches)
  - RUN: git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                    (expect: no matches)
  - RUN: git grep -n 'panic.*UpdateRefCAS' internal/git/git.go                   (expect: no matches — stub gone)
  - RUN: git grep -n 'var ErrCASFailed' internal/git/git.go                      (expect: exactly 1 match)
  - RUN: git grep -n 'func (g \*gitRunner) UpdateRefCAS' internal/git/git.go     (expect: exactly 1 match)
  - RUN: git grep -n 'runWithInput' internal/git/git.go | grep -i updateref      (expect: no matches — uses run())
  - RUN: git status --porcelain → expect EXACTLY:
        M internal/git/git.go
        M internal/git/git_test.go
        ?? internal/git/updateref_test.go
```

### Test cases (the assertion matrix)

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestUpdateRefCAS_Success` | initRepo + makeEmptyCommit×2 → HEAD=C1; `casCommit([C1])`=dangling newSHA | `err==nil`; `casHEAD==newSHA` | CAS success path (FR40); HEAD atomically advanced |
| `TestUpdateRefCAS_StaleExpected` | initRepo + makeEmptyCommit×2 → HEAD=C1; capture expected=C1; `casMoveHEAD(C0)`; call with expected=C1 | `errors.Is(err, ErrCASFailed)`; msg contains `"(exit 128)"`; `casHEAD==C0` (unchanged) | **The core race**: HEAD moved during generation → clean abort, repo untouched (§18.1/§18.2) |
| `TestUpdateRefCAS_RootCommit` | initRepo (unborn) + `casCommit(nil)`=dangling root; expected=`000…0` | `err==nil`; `casHEAD==rootSHA` | all-zeros + unborn ⇒ publish succeeds (§13.5 root edge) |
| `TestUpdateRefCAS_AllZerosOnBornRepo` | initRepo + makeEmptyCommit → HEAD=C0; expected=`000…0` | `errors.Is(err, ErrCASFailed)`; `casHEAD==C0` | all-zeros + born ⇒ fails ("reference already exists") |
| `TestUpdateRefCAS_GitBinaryMissing` | `t.Setenv("PATH","")` | err contains "git binary not found"; `!errors.Is(err, ErrCASFailed)` | infrastructural failure NOT misreported as CAS failure (gotcha G4) |
| `TestUpdateRefCAS_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)`; `!errors.Is(err, ErrCASFailed)` | ctx.Err() surfaced, not wrapped as CAS failure |

### Implementation Patterns & Key Details

```go
// === Why a sentinel var, not a plain fmt.Errorf (G3) ===
// The orchestrator must branch: "was this a CAS failure (print §13.5 message, exit 1) or some other
// error (propagate)?" A plain fmt.Errorf("...CAS failed...") is undetectable except by string match,
// which FINDING 3 forbids (stderr varies). A sentinel wrapped with %w is detectable via errors.Is
// AND still carries context. This is the idiomatic Go pattern for "a category of error callers match
// on" (io.EOF, sql.ErrNoRows, os.ErrNotExist).

// === Why err is checked BEFORE code, and infrastructural errors are UNWRAPPED (G4) ===
// run() guarantees: err != nil ⟹ exitCode == -1 (LookPath / context / start failure).
//                  err == nil  for every real git exit (0, 128, …).
// So `if err != nil { return err }` is the authoritative infrastructural-failure guard, and it MUST
// run before the `code != 0` branch. The err returned there is NOT wrapped — a missing git binary is
// a different failure category than a CAS mismatch, and TestUpdateRefCAS_GitBinaryMissing asserts
// !errors.Is(err, ErrCASFailed) to lock that in. This mirrors S2/S3/S4's branch ordering.

// === Why branch on `code != 0` (not `code == 1`, not `code == 128`) (G1) ===
// The cheat-sheet in git_plumbing_summary.md claims CAS mismatch = exit 1. That is WRONG on git
// 2.54.0 — it is 128 (research §2.1). Worse, the exit could differ across versions. The STABLE signal
// is "non-zero exit ⇒ CAS did not match." Branching on `code != 0` is correct for all observed and
// future failure modes (stale-expected, all-zeros-on-born, bad-newSHA all return 128 today). The
// message embeds the actual code ("(exit 128)") so a future change is visible in diagnostics.

// === Why the stderr is included but NEVER matched (G2) ===
// On git 2.54.0 the stderr is one of: "cannot lock ref 'HEAD': is at <actual> but expected <expected>",
// "cannot lock ref 'HEAD': reference already exists", or "trying to write ref ... with nonexistent
// object ...". These vary by scenario AND version. Including the trimmed stderr in the error string
// gives the user/debugger git's exact phrasing (useful in --verbose). DETECTION is solely via
// errors.Is(err, ErrCASFailed). Never write strings.Contains(stderr, ...).

// === Why run() and not runWithInput (G9) ===
// update-ref reads nothing from stdin (unlike commit-tree's -F - message). runWithInput (S4) exists
// solely to feed stdin. Using it here would be semantically wrong (nil reader) and wasteful. run() is
// the correct, already-landed helper. (Decision D9.)

// === Why no isUnborn parameter / no all-zeros special-casing (G10) ===
// UpdateRefCAS is a pure CAS primitive: forward expectedOld to git, branch on exit. When expectedOld
// is all-zeros, git's CAS itself enforces "ref must be unborn" (verified). The orchestrator decides
// what expectedOld to pass (the real parentSHA, or all-zeros when RevParseHEAD said isUnborn). This
// keeps one code path for born and unborn (no `if isUnborn` branch), mirroring how CommitTree (S4)
// doesn't know about isUnborn — the caller drives it via parents.

// === Why HEAD is NOT re-read inside the method (G11) ===
// PRD §13.5 wants "<actual>" in the abort message. Capturing it here (a CASFailedError struct) was
// rejected (decision D5): (1) the git package is plumbing, not message-formatting; (2) any captured
// <actual> is immediately stale; (3) it would couple this test to RevParseHEAD. The orchestrator
// re-reads HEAD when it observes ErrCASFailed. UpdateRefCAS stays a 1-git-call primitive.

// === Why cas-prefixed helper names (G12) ===
// S4 (committree_test.go, landing concurrently) plans setIdentityConfig/writeFile/stageFile/writeTreeOf/
// headSHA/commitMessage. If S5 reuses those names, the package won't compile when both land. The `cas`
// prefix (casCommit, casHEAD, casMoveHEAD, casOut, gitIdentityEnv) is collision-free. REUSE (don't
// redeclare) initRepo, minGitEnv, makeEmptyCommit from the existing files.
```

### Integration Points

```yaml
MODULE (consumed, not modified):
  - module path: "github.com/dustin/stagecoach" → package import path "github.com/dustin/stagecoach/internal/git"
  - go directive: 1.22 → context, errors, fmt, strings, os/exec, testing.T.TempDir/Setenv all available
  - deps: NONE added (errors/fmt/strings already imported; test helpers use only stdlib)

INTERNAL/GIT PACKAGE (modified here):
  - NEW exported symbol: `var ErrCASFailed` (consumed cross-package by the P1.M3.T4 orchestrator via
    errors.Is(err, git.ErrCASFailed)). This is the package's first exported non-type/non-method symbol.
  - NEW real method: `(*gitRunner).UpdateRefCAS` (satisfies the already-declared Git interface method).

CALLERS (future, NOT built here — documented so the contract is clear):
  - P1.M3.T4 (CommitStaged orchestrator): `if err := g.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld);
    err != nil { if errors.Is(err, git.ErrCASFailed) { actual, _, _ := g.RevParseHEAD(ctx);
    printAbortMessage(parent, actual, msg, treeSHA); exit 1 } else { /* infrastructural */ ... } }`
  - P1.M3.T3 (rescue protocol): fires on ErrCASFailed (or on pre-update-ref generation failure).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after each file creation - fix before proceeding
gofmt -w internal/git/git.go internal/git/updateref_test.go   # format the new/edited files
go vet ./internal/git/                                        # vet the package

# Project-wide validation
go build ./...
go vet ./...
gofmt -l internal/git/

# Expected: Zero errors and empty gofmt -l output. If errors exist, READ output and fix before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Test the new component in isolation
go test -race -run 'TestUpdateRefCAS' ./internal/git/ -v

# The stubs test (must still pass with the UpdateRefCAS line removed — 7 stubs remain)
go test -race -run TestStubsPanic ./internal/git/ -v

# Full package suite (S1 run() + S2 RevParseHEAD + S3 WriteTree + S4 CommitTree + S5 UpdateRefCAS)
go test -race ./internal/git/ -v

# Expected: All tests pass. If failing, debug root cause and fix implementation.
# Specifically: TestUpdateRefCAS_StaleExpected must show err wraps ErrCASFailed and HEAD unchanged.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the whole module (no main changes expected — just confirms the package compiles in-tree)
go build ./...

# Manual end-to-end CAS sanity check against a throwaway repo (mirrors research §6):
TMP=$(mktemp -d); cd "$TMP"; git init -q; git config user.name T; git config user.email t@e.com
echo a>a.txt; git add a.txt
C0=$(printf root|git commit-tree -F - $(git write-tree)); git update-ref HEAD "$C0"
echo b>b.txt; git add b.txt
C1=$(printf child|git commit-tree -p "$C0" -F - $(git write-tree))
EXPECTED=$C0; git update-ref HEAD "$C1" >/dev/null   # simulate the race (move HEAD to C1)
git update-ref HEAD "$C1" "$EXPECTED"; echo "EXIT=$? (expect 128 = CAS failure)"
cd /; rm -rf "$TMP"

# Expected: the manual command prints EXIT=128, confirming git's CAS-failure exit code matches what
# UpdateRefCAS branches on (code != 0). No network, no external services.

# Scope-discipline greps (production code only — git.go)
git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go   # expect: no matches
git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                    # expect: no matches
git grep -n 'panic.*UpdateRefCAS' internal/git/git.go                   # expect: no matches (stub gone)
git grep -n 'var ErrCASFailed' internal/git/git.go                      # expect: exactly 1 match
git grep -n 'runWithInput' internal/git/git.go | grep -i updateref      # expect: no matches
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Prove the sentinel is detectable end-to-end via a tiny throwaway program (optional, ad-hoc):
# cat > /tmp/cas_probe_test.go <<'EOF'
# package main
# import ("context"; "errors"; "fmt"; "github.com/dustin/stagecoach/internal/git")
# func main() {
#   g := git.New("/tmp") // path irrelevant; will fail fast, but exercise the error path
#   err := g.UpdateRefCAS(context.Background(), "HEAD", "0".Lol, "old")  // (illustrative)
#   fmt.Println(errors.Is(err, git.ErrCASFailed))
# }
# EOF
# (Not committed — just confirms errors.Is(err, git.ErrCASFailed) compiles cross-package.)

# Race-detector confidence: every test runs under -race (Makefile `test` target = go test -race ./...).
# The CAS primitive itself is a single git subprocess under git's own ref lock; -race validates the
# surrounding Go (run()'s buffer handling, no shared mutable state).

# Expected: optional probe compiles and prints `false` for an infrastructural failure / `true` for a
# CAS failure (the whole point of the sentinel). All -race tests pass.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go test -race ./internal/git/` exits 0 (all 6 new cases + S1/S2/S3/S4 cases + trimmed TestStubsPanic).
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l internal/git/` reports nothing.

### Feature Validation

- [ ] On a correct `expectedOld`, `UpdateRefCAS` returns nil and `git rev-parse HEAD == newSHA`.
- [ ] On a stale `expectedOld` (HEAD moved), returns an error with `errors.Is(err, ErrCASFailed)==true`,
      message contains `"(exit 128)"`, and HEAD is byte-for-byte unchanged afterwards.
- [ ] All-zeros `expectedOld` on an unborn repo publishes a root commit (nil); on a born repo returns
      `ErrCASFailed`.
- [ ] A missing git binary returns an error mentioning "git binary not found" with
      `errors.Is(err, ErrCASFailed)==false` (NOT misreported as a CAS failure).
- [ ] A cancelled context returns `errors.Is(err, context.Canceled)==true` with
      `errors.Is(err, ErrCASFailed)==false`.
- [ ] The 2-arg force form is NEVER used in production code (grep confirms only the 3-arg form).

### Code Quality Validation

- [ ] `run()`, `runWithInput`, `New`, `gitRunner`, the `Git` interface, `FileChange`,
      `StagedDiffOptions`, `RevParseHEAD`, `WriteTree`, `CommitTree` are byte-identical to landed forms.
- [ ] ZERO new imports in git.go (`errors`/`fmt`/`strings` already present).
- [ ] `ErrCASFailed` is exported, package-level, wrapped with `%w` (not `%s`/`%v`).
- [ ] Helper names use the `cas` prefix (no collision with S4's `committree_test.go` helpers).
- [ ] File placement matches the desired codebase tree (only `internal/git/` touched).
- [ ] Anti-patterns avoided (check against Anti-Patterns section).
- [ ] `go.mod`/`go.sum` unchanged; no new dependencies.

### Documentation & Deployment

- [ ] `ErrCASFailed` has a doc comment explaining detection semantics and the "re-read HEAD in the
      orchestrator" contract.
- [ ] `UpdateRefCAS` has a doc comment citing PRD §13.2/§18.1 and FINDING 3.
- [ ] No new environment variables or config (internal method; PRD §13/§18 are the spec).
- [ ] The TestStubsPanic one-line removal is noted in the commit message (mirrors S2/S3/S4).

---

## Anti-Patterns to Avoid

- ❌ Don't branch on `code == 1` or `code == 128` — branch on `code != 0` (the cheat-sheet's exit-1 is
  wrong; FINDING 3 says exit ≠ 0 is the signal).
- ❌ Don't substring-match the stderr for CAS detection — it varies by scenario/version; use
  `errors.Is(err, ErrCASFailed)`.
- ❌ Don't wrap with `%s`/`%v` — use `%w` or `errors.Is` breaks (the orchestrator's whole detection
  mechanism depends on it).
- ❌ Don't use the 2-arg force form (`git update-ref HEAD <new>`) — it silently clobbers concurrent
  commits (PRD §13.1/§18.1 forbid it).
- ❌ Don't re-read HEAD inside UpdateRefCAS to populate `<actual>` — that's the orchestrator's job
  (separation of concerns; the value would be stale anyway).
- ❌ Don't wrap infrastructural errors (missing git / cancelled ctx) in `ErrCASFailed` — they are a
  different failure category and must stay distinguishable.
- ❌ Don't modify `run()`, `runWithInput`, the interface, or any sibling method — S5 owns only
  `ErrCASFailed` + `UpdateRefCAS` + its tests + the one-line `TestStubsPanic` edit.
- ❌ Don't add an import — `errors`/`fmt`/`strings` are all already present.
- ❌ Don't name test helpers `headSHA`/`setIdentityConfig`/etc. — collide with S4; use the `cas` prefix.
- ❌ Don't catch all exceptions / use a bare `return err` for the success path — check `code == 0`
  explicitly and return nil; only non-zero wraps the sentinel.
