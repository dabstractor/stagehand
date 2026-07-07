---
name: "P1.M3.T1.S3 — Update locked-in dry-run timeout test + add dry-run dup-retry/parse-retry/snapshot tests (pin S1+S2 fixes for PRD Issues 2 & 6)"
description: |

  THE SITUATION: PRD Issues 2 & 6 (the dry-run pipeline divergence) are **ALREADY FIXED in source** by
  the two prior subtasks in this module: S1 (`fb73011`) made `WriteTree` + `signal.SetSnapshot`
  **unconditional** in `runPipeline` (Issue 6 — snapshot now runs in dry-run), and S2 (`f8db87e`)
  **replaced the dry-run single-pass short-circuit with the shared bounded generate→parse→dedupe loop**
  (Issue 2 — dry-run now runs FR29 parse-retry, FR30–FR33 duplicate rejection, FR33 bounded retries;
  dry-run timeout now returns `*RescueError{Kind:ErrTimeout, TreeSHA}` instead of bare `ErrTimeout`).
  Baseline is GREEN (`go test -race ./pkg/stagecoach/` passes today).

  THE GAP (this subtask): the dry-run pipeline is fixed but the TEST COVERAGE still locks in / under-
  specifies the contract. Specifically: (a) `TestGenerateCommit_Timeout`/`"dryrun"` was flipped by S2
  to keep the tree green, but its comment is now stale, it has a redundant duplicate assertion, and it
  does NOT yet assert `re.Kind == ErrTimeout` or `exitcode.For(err) == 124` (both required by the S3
  contract and by "mirror the commit-path subtest"); (b) there is **no** dry-run duplicate-retry test;
  (c) there is **no** dry-run parse-retry (FR29) test; (d) there is **no** dry-run snapshot-creation test
  (Issue 6). None of these net-new tests exist in `pkg/stagecoach/stagecoach_test.go` today.

  THE DELIVERABLE (TEST-ONLY — contract OUTPUT: "test-only"; DOCS: none): EDIT
  `pkg/stagecoach/stagecoach_test.go` ONLY — refine the `"dryrun"` timeout subtest and ADD three net-new
  tests + two small fixture helpers. **No source edits, no docs, no go.mod/go.sum changes.** Because the
  behavior already shipped in S1/S2, the net-new tests should PASS as soon as they are written (they are
  characterization/regression tests that PIN the fix). A failure means an S1/S2 regression — report it,
  do NOT "fix" it by editing source in this subtask.

  ⚠️ **THE one harness gap**: the existing `setupTestRepo(t, stubtest.Options)` helper only emits
  `STAGECOACH_STUB_OUT` / `STAGECOACH_STUB_SLEEP_MS` into `[provider.stub.env]`. It has NO path for stub
  SCRIPT (call-varying) mode, which dup-retry (b) and parse-retry (c) REQUIRE (call 1 ≠ call 2).
  Resolution: add a sibling helper `setupScriptedRepo(t, headSubject, responses)` (copy-ready in
  §Implementation Blueprint) that writes a script file + counter file and emits
  `STAGECOACH_STUB_SCRIPT`/`STAGECOACH_STUB_COUNTER`. Mirrors `setupTestRepo`'s chdir/cleanup/TOML pattern
  and the inline-TOML precedent already used by `TestGenerateCommit_MissingProviderCommand_Issue3`.
  (§3, §4.)

  ⚠️ **MIRROR the canonical commit-path tests** in `internal/generate/generate_test.go`:
  `TestCommitStaged_DedupeRetryThenSuccess` (lines 119-141 — commits "feat: existing", script
  ["feat: existing","feat: fresh"], asserts "feat: fresh") and `TestCommitStaged_ParseFailRescue`
  (lines 145-187). The dry-run dup/parse-RETRY tests are the SUCCESS counterparts (do NOT set
  `MaxDuplicateRetries=0` — that trick forces exhaustion/rescue, the opposite of what (b)/(c) prove).
  (§5, §10.)

  ⚠️ **THE snapshot test (d)** proves Issue 6 by diffing the git object set before/after a dry run:
  `git add` writes the blob (before), `WriteTree` writes the tree (during the dry run), commit-tree/
  update-ref are skipped (after). A small `looseObjectTypes(t, dir)` helper runs
  `git cat-file --batch-all-objects --batch-check` and asserts ≥1 NEW `tree` object appeared — symmetric
  to the `objectCountLine` guard in `MissingProviderCommand` (which asserts NO new object). (§7.)

  Deliverable: EDIT `pkg/stagecoach/stagecoach_test.go` only — refine `TestGenerateCommit_Timeout`/
  `"dryrun"`; ADD `setupScriptedRepo` + `looseObjectTypes` helpers; ADD
  `TestGenerateCommit_DryRun_DedupeRetry`, `TestGenerateCommit_DryRun_ParseRetry`,
  `TestGenerateCommit_DryRun_Snapshot`. `gofmt -l`, `go vet ./pkg/stagecoach/`, and `go test -race ./...`
  all green. No other files.

---

## Goal

**Feature Goal**: Pin/verify the already-shipped S1+S2 dry-run fixes (PRD Issues 2 & 6) with net-new
tests + a refined timeout subtest, so the test suite (not just the source) proves that `--dry-run` runs
the **full** FR49 pipeline — diff→**snapshot**→generate→parse→**duplicate-check** with FR29 parse-retry
and FR30–FR33 bounded retries — and returns the **same** message a real commit would produce, while
still creating no commit (HEAD unchanged, `CommitSHA:""`, exit 0). Concretely: (a) the dry-run timeout
subtest asserts `*RescueError{Kind:ErrTimeout}` + non-empty `TreeSHA` + `exitcode.For==124`; (b) a
dry-run whose FIRST attempt duplicates a recent subject retries to the UNIQUE message; (c) a dry-run
whose first attempt is unparseable retries (FR29) to a valid message; (d) a dry-run creates a dangling
**tree** object (snapshot taken) without moving HEAD.

**Deliverable** (TEST-ONLY — `pkg/stagecoach/stagecoach_test.go` is the only file touched):
1. **REFINE** `TestGenerateCommit_Timeout` / subtest `"dryrun"`: fix the stale comment, drop the
   redundant duplicate `errors.Is` check, and ADD `re.Kind == ErrTimeout` +
   `exitcode.For(err) == exitcode.Timeout` (124) assertions (mirror + strengthen vs. `"commit_path"`).
2. **ADD** helper `setupScriptedRepo(t *testing.T, headSubject string, responses []string) string` —
   stub SCRIPT (call-varying) mode via a repo-local `.stagecoach.toml`, sibling to `setupTestRepo`.
3. **ADD** helper `looseObjectTypes(t *testing.T, dir string) map[string]string` — `git cat-file
   --batch-all-objects --batch-check` → `sha→type`, for the snapshot test.
4. **ADD** `TestGenerateCommit_DryRun_DedupeRetry` — script `["feat: existing","feat: fresh"]` over a
   repo whose HEAD subject is `"feat: existing"`; assert dry-run returns `"feat: fresh"` (dup rejected).
5. **ADD** `TestGenerateCommit_DryRun_ParseRetry` — script `["","feat: good"]`; assert dry-run returns
   `"feat: good"` (parse-fail retried, NOT a plain error).
6. **ADD** `TestGenerateCommit_DryRun_Snapshot` — single-response stub; assert a NEW `tree` object is
   created in dry-run AND HEAD is unchanged AND `CommitSHA == ""` (Issue 6 + Issue 2).

No source changes. No docs. No new deps (`go.mod`/`go.sum` byte-unchanged).

**Success Definition**: `gofmt -l pkg/stagecoach/`, `go vet ./pkg/stagecoach/`, and `go test -race ./...`
all green, including the four (a–d) dry-run test cases. `git diff --stat` touches ONLY
`pkg/stagecoach/stagecoach_test.go`. A reader of these tests can see, without reading `runPipeline`, that
`--dry-run` runs the full dedupe/retry pipeline and takes the snapshot.

## User Persona

**Target User**: A Stagecoach maintainer / future contributor who needs **test-level proof** that the
dry-run path matches the real commit path (and a regression net if anyone re-introduces the divergent
single-pass). Transitively the end user from PRD US9 ("judge quality before trusting it") whose
`--dry-run` preview is now faithful because S2 shipped — S3 makes that faithfulness enforceable.

**Use Case**: Run `go test -race ./pkg/stagecoach/` and see green for the dry-run dup-retry / parse-retry
/ snapshot / timeout tests — proof that `--dry-run` previews the EXACT message a real commit produces.

**User Journey**: (contributor) edit `runPipeline` → `go test -race ./pkg/stagecoach/` → if the dry-run
loop or snapshot is accidentally reverted, `TestGenerateCommit_DryRun_DedupeRetry` /
`_ParseRetry` / `_Snapshot` / `_Timeout` fail and name the broken contract.

**Pain Points Addressed**: Before S3, the dry-run path had strong SOURCE coverage (the loop is a copy of
`CommitStaged`) but weak TEST coverage at the `pkg/stagecoach` boundary — only a happy-path DryRun test
and a timeout test existed. A future refactor that re-introduced the divergent single-pass (or re-gated
`WriteTree` behind `if !dryRun`) would NOT be caught. S3 closes that hole.

## Why

- **Pin S1+S2 (regression net).** S1 (Issue 6) and S2 (Issue 2) shipped real behavior changes with only
  a single flipped assertion as their test footprint. S3 adds the net-new tests those subtasks deferred
  (S2's PRP §4/§7 explicitly states "S3 owns the net-new coverage").
- **FR49 acceptance bar (PRD §9.12).** FR49 requires the "full … duplicate-check pipeline" in dry-run.
  A test that feeds a duplicate first attempt and asserts the UNIQUE second attempt is the most direct
  proof FR49 is honored at the public `GenerateCommit` boundary.
- **Faithful preview (PRD US9).** The dup-retry test encodes PRD Issue 2's exact Steps-to-Reproduce
  (attempt 1 = dup of existing subject, attempt 2 = unique) and asserts dry-run no longer shows the dup.
- **Issue 6 proof.** The snapshot test asserts a `tree` object IS created in dry-run — the literal
  contract clause "git cat-file finds the snapshotted tree".
- **Consistent rescue contract.** The refined timeout subtest asserts dry-run timeout is
  `*RescueError{Kind:ErrTimeout}` mapping to exit 124 (mirroring the commit path), not a bare sentinel.
- **Boundaries respected.** S3 touches ONLY the test file. It does NOT re-do S1/S2 source work, does NOT
  add docs, does NOT change any frozen file (`generate.go`, `signal.go`, `provider/*`, `git/*`,
  `exitcode/*`, `config/*`).

## What

Additive edits to `pkg/stagecoach/stagecoach_test.go`: refine one existing subtest's assertions/comment,
add two small fixture helpers (`setupScriptedRepo`, `looseObjectTypes`), and add three net-new test
functions. All four behaviors are already implemented by S1/S2 in `runPipeline`; the tests are
characterization/regression tests that should pass immediately. No new types, no new imports beyond
what the file already has (everything used — `errors`, `os`, `strings`, `strconv`, `context`, `testing`,
`exitcode`, `stubtest` — is already imported).

### Success Criteria

- [ ] `TestGenerateCommit_Timeout`/`"dryrun"` asserts: `errors.As(err,&re)` true; `errors.Is(err,
      ErrTimeout)` true (exactly ONCE — no duplicate check); `re.Kind == ErrTimeout`; `re.TreeSHA !=
      ""`; `exitcode.For(err) == exitcode.Timeout` (124). Its leading comment no longer claims "bare
      sentinel, no TreeSHA".
- [ ] `TestGenerateCommit_DryRun_DedupeRetry`: repo HEAD subject = `"feat: existing"`; stub script
      `["feat: existing","feat: fresh"]`; `GenerateCommit(DryRun:true)` returns `err==nil`,
      `Message == "feat: fresh"`, `Subject == "feat: fresh"`, `CommitSHA == ""`.
- [ ] `TestGenerateCommit_DryRun_ParseRetry`: stub script `["","feat: good"]`; returns `err==nil`,
      `Message == "feat: good"` (proving the blank first attempt was retried past, not a plain error).
- [ ] `TestGenerateCommit_DryRun_Snapshot`: single-response stub; a NEW `tree`-typed object appears in
      `git cat-file --batch-all-objects --batch-check` after the dry run; HEAD unchanged; `CommitSHA == ""`.
- [ ] `setupScriptedRepo` + `looseObjectTypes` helpers co-located with the existing fixture helpers and
      reused by the new tests.
- [ ] `gofmt -l pkg/stagecoach/` empty; `go vet ./pkg/stagecoach/` clean; `go test -race ./...` green.
- [ ] `git diff --stat` shows ONLY `pkg/stagecoach/stagecoach_test.go`; `git diff --exit-code go.mod
      go.sum` empty; every `internal/*` and `pkg/stagecoach/stagecoach.go` file byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact
helpers to add (copy-ready in §Implementation Blueprint), the exact canonical tests to mirror
(quoted from `internal/generate/generate_test.go`), the stub script-mode contract (quoted from
`cmd/stubagent/main.go`), the `exitcode`/`RescueError` type facts, and the four copy-ready test
bodies. No external research needed — pure in-repo test additions against already-shipped behavior.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- file: pkg/stagecoach/stagecoach_test.go   (THE ONLY file you EDIT)
  section: fixture helpers (initRepo/writeFile/stageFile/headSHA/commitRaw/gitOut/runGit/setupTestRepo/
           objectCountLine) and TestGenerateCommit_Timeout (the "dryrun"+"commit_path" subtests).
  why: this is the entire change surface. setupTestRepo is the pattern setupScriptedRepo mirrors;
       objectCountLine is the pattern looseObjectTypes mirrors; TestGenerateCommit_Timeout/"dryrun" is
       the subtest you refine; TestGenerateCommit_MissingProviderCommand_Issue3 is the precedent for
       writing a custom .stagecoach.toml inline (the exitcode.For assertion pattern lives here too).
  pattern: every pkg/stagecoach test calls setupTestRepo/GenerateCommit(ctx, Options{...}) and operates
           on repoDir,_:=os.Getwd() after the helper chdir's. The new tests follow this exactly.
  gotcha: setupTestRepo commits "initial" and registers the stub in single-response (STAGECOACH_STUB_OUT)
          mode ONLY. dup/parse-retry need SCRIPT mode → use the new setupScriptedRepo helper.

- file: internal/stubtest/stubtest.go   (READ ONLY — the stub harness)
  section: Options{Out,Exit,SleepMS,Stderr,Script,Counter,...}, NewScript(t,bin,responses), optsEnvMap.
  why: NewScript is the canonical way to build a call-varying Manifest in the internal/generate tests,
       but pkg/stagecoach tests can't pass a Manifest directly (GenerateCommit builds deps from config).
       So setupScriptedRepo reproduces NewScript's file layout (script.txt = join(responses,"\n") +
       counter file absent⇒0) INSIDE a repo-local .stagecoach.toml's [provider.stub.env].
  critical: STAGECOACH_STUB_SCRIPT = path to a file whose lines are the per-call responses (\n-joined);
            STAGECOACH_STUB_COUNTER = path to a counter file (ABSENT ⇒ stub reads 0). Blank line ⇒ empty
            stdout ⇒ ParseOutput ok=false ⇒ FR29 retry. Clamp-to-last after exhaustion.

- file: cmd/stubagent/main.go   (READ ONLY — confirms the script/counter contract)
  section: selectScripted(scriptFile) — reads STAGECOACH_STUB_SCRIPT, splits on "\n", indexes by the
           STAGECOACH_STUB_COUNTER file (read→write index+1), clamps to last when exhausted.
  why: proves blank lines are significant (empty output ⇒ parse failure) and that each stub invocation
       is a fresh process whose only cross-call state is the counter FILE. Serial attempts ⇒ no race;
       each test's own t.TempDir counter ⇒ no cross-test interference.
  gotcha: the stub drains stdin BEFORE sleeping (deadlock guard) — irrelevant to these tests, but
          confirms SleepMS-based timeout tests are stable (the existing "dryrun"/"commit_path" timeout
          subtests already rely on this).

- file: internal/generate/generate_test.go   (READ ONLY — the canonical tests to MIRROR)
  section: TestCommitStaged_DedupeRetryThenSuccess (lines 119-141) and TestCommitStaged_ParseFailRescue
           (lines 145-187).
  why: these are the authoritative dup-retry / parse-failure patterns. The dry-run dup-retry test is the
       DryRun SUCCESS counterpart of DedupeRetryThenSuccess; the dry-run parse-retry test is the DryRun
       SUCCESS counterpart of ParseFailRescue (which uses MaxDuplicateRetries=0 to FORCE rescue — do NOT
       copy that trick; the retry tests need the DEFAULT budget so the loop can retry and SUCCEED).
  pattern: DedupeRetryThenSuccess — commitRaw("feat: existing"); NewScript(["feat: existing","feat: fresh"]);
           assert Subject=="feat: fresh". The dry-run version swaps CommitStaged→GenerateCommit(DryRun:true)
           and setupScriptedRepo for NewScript, and additionally asserts CommitSHA=="".

- file: internal/generate/generate.go   (READ ONLY — RescueError + sentinels + the loop)
  section: lines 46-101 (ErrNothingToCommit/ErrTimeout/ErrRescue sentinels; RescueError struct
           {Kind error; TreeSHA,ParentSHA,Candidate string; Cause error}; Unwrap()==Kind) and the loop
           at ~144-225 (the authoritative generate→parse→dedupe loop runPipeline mirrors).
  why: confirms `errors.Is(err, ErrTimeout)` chains through *RescueError via Unwrap()==Kind, and that
       `re.Kind == ErrTimeout` is a valid direct comparison (Kind holds the sentinel var).
  gotcha: do NOT edit this file (frozen). pkg/stagecoach re-exports the sentinels/types as aliases
          (ErrTimeout, RescueError) — use those names in the test.

- file: internal/exitcode/exitcode.go   (READ ONLY — the exit-code mapping)
  section: const Timeout = 124; For(err) — `errors.Is(err, generate.ErrTimeout) → Timeout (124)`,
           checked BEFORE the generic rescue→3.
  why: the refined "dryrun" subtest asserts `exitcode.For(err) == exitcode.Timeout`. `exitcode` is
       already imported in stagecoach_test.go (TestGenerateCommit_MissingProviderCommand_Issue3 uses it).

- file: internal/git/git.go   (READ ONLY — confirms WriteTree writes a tree object)
  section: WriteTree impl (~line 219) — runs `git write-tree`, returns the 40/64-hex tree SHA; the
           object written is a loose tree object in the object store.
  why: the snapshot test asserts a NEW tree-typed object appears after a dry run. `git add` writes the
       BLOB first (during setup); WriteTree writes the TREE during GenerateCommit; commit-tree/update-ref
       are skipped in dry-run ⇒ no commit/blob added ⇒ the only new object is the tree.

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/seam_dryrun.md
  section: "## 5. DryRun tests in pkg/stagecoach/stagecoach_test.go" + "## Start Here".
  why: the recon that scoped S2/S3. §5 enumerates EXACTLY the test gaps S3 fills (only happy-path DryRun
       + the locking timeout test existed; no dup/parse-retry/snapshot coverage).
  critical: §5 notes the timeout test "locks in the current divergent behavior" — S2 already flipped it;
       S3 refines + strengthens it (adds Kind + exitcode assertions) and adds the three net-new tests.

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/P1M3T1S2/PRP.md
  section: "## What" Success Criteria + "## Anti-Patterns to Avoid" (last bullet: "S3 owns net-new tests").
  why: confirms S2 deferred the net-new dup/parse-retry/snapshot tests to S3 and that S2 already flipped
       the single timeout assertion. S3 picks up exactly where S2 left off.

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/P1M3T1S3/research/findings.md
  why: this subtask's own research note — current tree state (§0), part-(a) gaps (§1), the harness gap
       and setupScriptedRepo resolution (§3), stub script semantics (§4), the dedupe target (§5), loop
       budget (§6), snapshot detection (§7), exitcode/RescueError facts (§8), naming/placement (§9).
```

### Current Codebase tree (relevant slice)

```bash
go.mod / go.sum                              # UNCHANGED — S3 adds NO dep (stdlib only)
pkg/stagecoach/
  stagecoach.go          # FROZEN (S1+S2 shipped) — READ ONLY for S3
  stagecoach_test.go     # EDIT — refine TestGenerateCommit_Timeout/"dryrun"; ADD 2 helpers + 3 tests
internal/
  stubtest/stubtest.go      # READ ONLY — Options/NewScript (the script-mode contract setupScriptedRepo mirrors)
  generate/generate.go      # FROZEN — RescueError/sentinels/loop (READ ONLY)
  generate/generate_test.go # READ ONLY — TestCommitStaged_DedupeRetryThenSuccess / _ParseFailRescue (templates)
  git/git.go                # READ ONLY — WriteTree writes a loose tree object
  exitcode/exitcode.go      # READ ONLY — Timeout=124; For(*RescueError{ErrTimeout})==124
  config/config.go          # READ ONLY — Defaults().MaxDuplicateRetries==3 (4 attempts)
  cmd/stubagent/main.go     # READ ONLY — selectScripted (script/counter/blank-line contract)
cmd/stubagent/main.go       # (same — built once per process by stubtest.Build)
Makefile                # UNCHANGED
```

### Desired Codebase tree with files to be added

```bash
pkg/stagecoach/
  stagecoach_test.go     # EDITED — +setupScriptedRepo, +looseObjectTypes, +3 tests, refined "dryrun" subtest
# NO new files. NO source changes. NO docs. go.mod/go.sum UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (S1+S2 ALREADY SHIPPED — S3 is test-only): runPipeline already (a) takes WriteTree
// unconditionally, (b) runs ONE shared generate→parse→dedupe loop for dryRun AND !dryRun, (c) returns
// Result{CommitSHA:""} via a dry-run success early-return after `if !success`, (d) returns
// *RescueError{Kind:ErrTimeout,TreeSHA} on dry-run timeout. The net-new tests should PASS immediately.
// If one FAILS, it is an S1/S2 regression — report it; do NOT edit source to "fix" it in this subtask.

// CRITICAL (setupTestRepo has NO script mode): it only emits STAGECOACH_STUB_OUT / STAGECOACH_STUB_SLEEP_MS.
// dup-retry (b) and parse-retry (c) need call-varying responses → use the new setupScriptedRepo helper,
// which emits STAGECOACH_STUB_SCRIPT + STAGECOACH_STUB_COUNTER. Do NOT try to shoehorn script mode through
// setupTestRepo (extend-via-sibling-helper, mirroring the inline-TOML precedent in MissingProviderCommand).

// CRITICAL (the dup target must be a REAL commit subject): runPipeline builds `recent =
// Git.RecentSubjects(ctx,50)` ONCE and IsDuplicate is an EXACT subject match. For attempt 1 to be
// rejected, the repo MUST contain a commit whose subject == attempt 1. setupScriptedRepo commits
// `headSubject` as the repo's only commit → pass headSubject=="feat: existing" + script[0]=="feat: existing".
// Mirror TestCommitStaged_DedupeRetryThenSuccess: use "feat:"-prefixed subjects (don't rely on bare
// "initial" — avoids any ParseOutput edge case on non-conventional messages).

// CRITICAL (do NOT set MaxDuplicateRetries=0 for the RETRY tests): that trick (used by
// TestCommitStaged_ParseFailRescue) forces loop EXHAUSTION → *RescueError. The dup/parse-RETRY tests
// need the DEFAULT budget (3 → 4 attempts) so the loop can retry and SUCCEED on attempt 2. The
// .stagecoach.toml does not set max_duplicate_retries → config.Defaults() → 3. Do not override it.

// CRITICAL (blank script line ⇒ parse failure, not empty success): a "" entry in the responses slice
// produces empty stub stdout ⇒ provider.ParseOutput returns ok==false ⇒ the loop sets parseFail=true and
// CONTINUEs (FR29 retry). So parse-retry script is ["", "feat: good"]: attempt 1 fails parse, attempt 2
// succeeds. Assert Message=="feat: good" (NOT an error) — the OLD single-pass returned a plain error here.

// CRITICAL (snapshot detection ordering): capture the object set AFTER `git add` (so the staged blob is
// already counted) and BEFORE GenerateCommit. WriteTree (inside GenerateCommit) writes the tree. Diff
// before/after → the new object(s) are the tree(s). Dry-run skips commit-tree/update-ref ⇒ no commit/blob
// added ⇒ after\before ⊇ {one tree}. Use `git cat-file --batch-all-objects --batch-check` (covers loose
// AND packed; fresh temp repos are all-loose). Assert ≥1 NEW object with type=="tree".

// GOTCHA (no new import needed): everything used — errors, os, strings, strconv, context, testing,
// exitcode, stubtest — is ALREADY imported in stagecoach_test.go. Do NOT add "path/filepath"; use string
// concatenation (dir+"/script.txt") exactly like setupTestRepo's repo+"/.stagecoach.toml".

// GOTCHA (TOML env block reads fine): setupTestRepo already writes [provider.stub.env] with
// STAGECOACH_STUB_OUT when Out is set, and those tests pass → config.Load decodes nested [provider.X.env]
// into the manifest Env. STAGECOACH_STUB_SCRIPT / STAGECOACH_STUB_COUNTER decode identically.

// GOTCHA (errors.Is appears once in the refined "dryrun" subtest): S2 left a duplicate
// `if !errors.Is(err, ErrTimeout)` check (once before `var re`, once after). Drop the redundancy; keep
// one `errors.Is(err, ErrTimeout)` plus the new `re.Kind == ErrTimeout` + `exitcode.For(err)==exitcode.Timeout`.
```

## Implementation Blueprint

### Data models and structure

```go
// NO new types. The existing test-file types drive everything:
//   Options{Provider,Model,SystemExtra,DryRun,Timeout,...}  (pkg/stagecoach.GenerateCommit input)
//   Result{CommitSHA,Subject,Message,Provider,Model}        (pkg/stagecoach.GenerateCommit output)
//   *RescueError{Kind,TreeSHA,ParentSHA,Candidate,Cause}    (pkg/stagecoach type alias for generate.RescueError)
//   exitcode.Timeout == 124                                  (internal/exitcode)
//   stubtest.Options{Out,SleepMS,Script,Counter,...}        (only Out/SleepMS used here via setupTestRepo)
// The two new helpers return plain values (string bin; map[string]string sha→type).
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD helper setupScriptedRepo to pkg/stagecoach/stagecoach_test.go
  - PLACEMENT: immediately after setupTestRepo (co-locate with the other fixture helpers, ~line 140).
  - SIGNATURE: func setupScriptedRepo(t *testing.T, headSubject string, responses []string) string
      (returns the stub bin path, like setupTestRepo; callers usually ignore it).
  - BODY (copy-ready — see "Implementation Patterns & Key Details" §A): build a repo-local
      .stagecoach.toml registering [provider.stub] with command=bin, prompt_delivery="stdin",
      output="raw", strip_code_fence=true, and a [provider.stub.env] table emitting
      STAGECOACH_STUB_SCRIPT="<t.TempDir>/script.txt" + STAGECOACH_STUB_COUNTER="<t.TempDir>/counter".
      Write the script file as strings.Join(responses,"\n"). initRepo + commitRaw(headSubject) + chdir
      (repo) + t.Cleanup(restore wd). Mirror setupTestRepo's exact structure; only the env block differs.
  - GOTCHA: use dir+"/script.txt" / dir+"/counter" (string concat, NOT filepath.Join — matches
      setupTestRepo's repo+"/.stagecoach.toml" and avoids a new import). Use a SEPARATE t.TempDir() for
      the script/counter files from the repo dir (keeps them out of git's view; harmless either way).
  - GOTCHA: commit headSubject (NOT a hardcoded "initial") so the dup-retry test can pass
      "feat: existing" as both the head subject and script[0].

Task 2: ADD helper looseObjectTypes to pkg/stagecoach/stagecoach_test.go
  - PLACEMENT: immediately after objectCountLine (co-locate, ~line 161).
  - SIGNATURE: func looseObjectTypes(t *testing.T, dir string) map[string]string
  - BODY (copy-ready — see §B): gitOut(t, dir, "cat-file","--batch-all-objects","--batch-check") →
      split on "\n" → for each line, strings.Fields → objs[f[0]]=f[1] (sha→type). Return the map.
  - WHY: symmetric to objectCountLine (MissingProviderCommand asserts NO new object); this asserts a
      NEW tree object. --batch-all-objects covers loose+packed.

Task 3: REFINETestGenerateCommit_Timeout/"dryrun" in pkg/stagecoach/stagecoach_test.go
  - PLACEMENT: the existing "dryrun" subtest inside TestGenerateCommit_Timeout (~lines 196-243).
  - EDITS (copy-ready — see §C):
      (1) Replace the stale leading comment "// DryRun path: ErrTimeout (bare sentinel, no TreeSHA)."
          with "// DryRun path: *RescueError{Kind:ErrTimeout} with a real TreeSHA (S1 snapshot + S2 loop)."
      (2) Keep the FIRST `if !errors.Is(err, ErrTimeout) { t.Errorf(...) }` guard.
      (3) Keep `var re *RescueError; if !errors.As(err,&re) { t.Fatalf(...) }` and `if re.TreeSHA=="" {...}`.
      (4) DELETE the now-redundant SECOND `if !errors.Is(err, ErrTimeout)` block.
      (5) ADD: `if re.Kind != ErrTimeout { t.Errorf("dryrun: re.Kind = %v, want ErrTimeout", re.Kind) }`.
      (6) ADD: `if code := exitcode.For(err); code != exitcode.Timeout { t.Errorf("dryrun: exitcode.For = %d, want %d (Timeout)", code, exitcode.Timeout) }`.
  - GOTCHA: `exitcode` is already imported. `ErrTimeout`/`RescueError` are the pkg/stagecoach aliases.

Task 4: ADD TestGenerateCommit_DryRun_DedupeRetry to pkg/stagecoach/stagecoach_test.go
  - PLACEMENT: immediately after TestGenerateCommit_Timeout (group the dry-run dup/parse/snapshot tests).
  - BODY (copy-ready — see §D): setupScriptedRepo(t, "feat: existing", []string{"feat: existing","feat: fresh"});
      repoDir,_:=os.Getwd(); writeFile+stageFile a new file; res,err:=GenerateCommit(ctx,
      Options{Provider:"stub",DryRun:true}); assert err==nil, Message=="feat: fresh",
      Subject=="feat: fresh", CommitSHA==""; optionally assert HEAD unchanged.
  - WHY: proves a dry-run retries PAST a duplicate first attempt (Issue 2 / FR32). OLD single-pass would
      have returned "feat: existing" (the dup).

Task 5: ADD TestGenerateCommit_DryRun_ParseRetry to pkg/stagecoach/stagecoach_test.go
  - PLACEMENT: immediately after Task 4's test.
  - BODY (copy-ready — see §E): setupScriptedRepo(t, "initial", []string{"","feat: good after parse retry"});
      stage a file; res,err:=GenerateCommit(DryRun:true); assert err==nil, Message=="feat: good after parse retry".
  - WHY: proves a dry-run retries PAST an unparseable first attempt (FR29). OLD single-pass returned a
      plain errors.New("dry run: model produced no valid message"). The head subject is irrelevant here
      ("initial" is fine) — only the script's blank-then-valid sequence matters.

Task 6: ADD TestGenerateCommit_DryRun_Snapshot to pkg/stagecoach/stagecoach_test.go
  - PLACEMENT: immediately after Task 5's test.
  - BODY (copy-ready — see §F): setupTestRepo(t, stubtest.Options{Out:"feat: snapshot taken"});
      repoDir,_:=os.Getwd(); writeFile+stageFile; before:=looseObjectTypes(t,repoDir); beforeHEAD:=headSHA;
      res,err:=GenerateCommit(DryRun:true); assert err==nil, CommitSHA==""; after:=looseObjectTypes;
      count new trees (after[sha]=="tree" && !in before); assert >=1; assert HEAD unchanged.
  - WHY: proves Issue 6 (write-tree runs in dry-run) + Issue 2 (no commit, HEAD unchanged).

Task 7: VERIFY (no further file change)
  - RUN the Validation Loop (Levels 1–2). `git diff --stat` MUST show ONLY stagecoach_test.go.
      `git diff --exit-code go.mod go.sum` empty. Every internal/* and stagecoach.go byte-unchanged.
      `go test -race ./...` green (incl. the 4 dry-run cases + the full suite).
```

### Implementation Patterns & Key Details

```go
// §A — setupScriptedRepo (sibling to setupTestRepo; script/call-varying mode). Mirrors setupTestRepo
// line-for-line except the [provider.stub.env] block emits SCRIPT/COUNTER instead of OUT/SLEEP_MS.
func setupScriptedRepo(t *testing.T, headSubject string, responses []string) string {
	t.Helper()
	bin := stubtest.Build(t)
	repo := t.TempDir()
	aux := t.TempDir() // script + counter live outside the repo (harmless; keeps git's view clean)
	script := aux + "/script.txt"
	if err := os.WriteFile(script, []byte(strings.Join(responses, "\n")), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	counter := aux + "/counter" // absent ⇒ stub reads 0

	var sb strings.Builder
	sb.WriteString("[provider.stub]\n")
	sb.WriteString("command = \"" + bin + "\"\n")
	sb.WriteString("prompt_delivery = \"stdin\"\n")
	sb.WriteString("output = \"raw\"\n")
	sb.WriteString("strip_code_fence = true\n")
	sb.WriteString("\n[provider.stub.env]\n")
	sb.WriteString("STAGECOACH_STUB_SCRIPT = \"" + script + "\"\n")
	sb.WriteString("STAGECOACH_STUB_COUNTER = \"" + counter + "\"\n")
	if err := os.WriteFile(repo+"/.stagecoach.toml", []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("write .stagecoach.toml: %v", err)
	}

	initRepo(t, repo)
	commitRaw(t, repo, headSubject) // headSubject is the dup target for the dup-retry test

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir %s: %v", repo, err)
	}
	t.Cleanup(func() { os.Chdir(wd) })
	return bin
}

// §B — looseObjectTypes (sibling to objectCountLine; proves a NEW object of a given type appeared).
func looseObjectTypes(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := gitOut(t, dir, "cat-file", "--batch-all-objects", "--batch-check")
	objs := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line) // "<sha> <type> <size>"
		if len(f) >= 2 {
			objs[f[0]] = f[1] // sha → type
		}
	}
	return objs
}

// §C — the REFINED "dryrun" subtest body (inside TestGenerateCommit_Timeout). Keeps S2's flipped
// errors.As/TreeSHA assertions, drops the redundant duplicate errors.Is, and adds Kind + exitcode.
	t.Run("dryrun", func(t *testing.T) {
		setupTestRepo(t, stubtest.Options{Out: "feat: slow", SleepMS: 2000})
		repoDir, _ := os.Getwd()
		writeFile(t, repoDir, "z.txt", "data")
		stageFile(t, repoDir, "z.txt")

		ctx := context.Background()
		_, err := GenerateCommit(ctx, Options{Provider: "stub", DryRun: true, Timeout: 150 * time.Millisecond})
		if err == nil {
			t.Fatal("expected error on timeout, got nil")
		}
		// DryRun path: *RescueError{Kind:ErrTimeout} with a real TreeSHA (S1 snapshot + S2 loop).
		if !errors.Is(err, ErrTimeout) {
			t.Errorf("errors.Is(err, ErrTimeout) = false, error = %v", err)
		}
		var re *RescueError
		if !errors.As(err, &re) {
			t.Fatalf("dryrun: error type = %T, want *RescueError", err)
		}
		if re.Kind != ErrTimeout {
			t.Errorf("dryrun: re.Kind = %v, want ErrTimeout", re.Kind)
		}
		if re.TreeSHA == "" {
			t.Error("dryrun: RescueError.TreeSHA empty, want non-empty (snapshot was taken — S1)")
		}
		if code := exitcode.For(err); code != exitcode.Timeout {
			t.Errorf("dryrun: exitcode.For(err) = %d, want %d (Timeout/124)", code, exitcode.Timeout)
		}
	})

// §D — TestGenerateCommit_DryRun_DedupeRetry (mirror TestCommitStaged_DedupeRetryThenSuccess, DryRun).
func TestGenerateCommit_DryRun_DedupeRetry(t *testing.T) {
	setupScriptedRepo(t, "feat: existing", []string{"feat: existing", "feat: fresh"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "a.txt", "data")
	stageFile(t, repoDir, "a.txt")

	beforeHEAD := headSHA(t, repoDir)
	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun dedupe-retry: %v", err)
	}
	if res.Message != "feat: fresh" {
		t.Errorf("Message = %q, want %q (duplicate first attempt should have been rejected & retried past)",
			res.Message, "feat: fresh")
	}
	if res.Subject != "feat: fresh" {
		t.Errorf("Subject = %q, want %q", res.Subject, "feat: fresh")
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun must not commit)", res.CommitSHA)
	}
	if got := headSHA(t, repoDir); got != beforeHEAD {
		t.Errorf("HEAD changed from %q to %q, want unchanged (DryRun)", beforeHEAD, got)
	}
}

// §E — TestGenerateCommit_DryRun_ParseRetry (FR29: blank first attempt → retry → valid message).
func TestGenerateCommit_DryRun_ParseRetry(t *testing.T) {
	setupScriptedRepo(t, "initial", []string{"", "feat: good after parse retry"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "p.txt", "data")
	stageFile(t, repoDir, "p.txt")

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun parse-retry: %v (the unparseable first attempt should have been retried, not surfaced as an error)", err)
	}
	if res.Message != "feat: good after parse retry" {
		t.Errorf("Message = %q, want %q (blank first attempt should have triggered FR29 retry to the valid message)",
			res.Message, "feat: good after parse retry")
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun)", res.CommitSHA)
	}
}

// §F — TestGenerateCommit_DryRun_Snapshot (Issue 6: write-tree runs in dry-run; Issue 2: no commit).
func TestGenerateCommit_DryRun_Snapshot(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: snapshot taken"})
	repoDir, _ := os.Getwd()
	writeFile(t, repoDir, "snap.txt", "data")
	stageFile(t, repoDir, "snap.txt") // writes the blob (counted in `before`)

	before := looseObjectTypes(t, repoDir) // captured AFTER staging, BEFORE GenerateCommit
	beforeHEAD := headSHA(t, repoDir)

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit DryRun snapshot: %v", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun must not commit)", res.CommitSHA)
	}

	// Issue 6: WriteTree ran in dry-run → a NEW tree-typed object exists in the object store.
	after := looseObjectTypes(t, repoDir)
	newTrees := 0
	for sha, typ := range after {
		if _, ok := before[sha]; !ok && typ == "tree" {
			newTrees++
		}
	}
	if newTrees == 0 {
		t.Error("dry-run snapshot: no new tree object created; WriteTree was skipped (Issue 6 regression)")
	}

	// Issue 2: dry-run skips commit-tree/update-ref → HEAD unchanged.
	if got := headSHA(t, repoDir); got != beforeHEAD {
		t.Errorf("HEAD changed from %q to %q, want unchanged (DryRun)", beforeHEAD, got)
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. S3 uses only already-imported stdlib (errors, os, strings, strconv, context, testing)
        + already-imported internal packages (exitcode, stubtest). `go mod tidy` is a no-op;
        `git diff --exit-code go.mod go.sum` is empty.

PACKAGE EDGES (import graph):
  - pkg/stagecoach (test) → (internal: config, exitcode, stubtest; stdlib). UNCHANGED. No new import edge.

UPSTREAM (already in place — the behavior S3 verifies, do NOT rebuild):
  - S1 (P1.M3.T1.S1): WriteTree + signal.SetSnapshot are UNCONDITIONAL in runPipeline → treeSHA always
        set in dry-run; rescue armed. The snapshot test (Task 6) proves the object is written.
  - S2 (P1.M3.T1.S2): the dry-run short-circuit is GONE; the shared loop runs for dryRun; dry-run
        success returns Result{CommitSHA:""} via the early-return; dry-run timeout/exhaustion return
        *RescueError{Kind,TreeSHA}. Tasks 3–6 verify all of this.
  - config.Defaults().MaxDuplicateRetries == 3 (4 attempts): bounds the loop; ample for 2-attempt
        retry tests. The .stagecoach.toml does not override it.

DOWNSTREAM (contracts S3 preserves):
  - The pkg/stagecoach public API (GenerateCommit, Options, Result, ErrTimeout, RescueError) is
        UNCHANGED. S3 only exercises it. No CLI/config/exitcode change.
  - The test names (TestGenerateCommit_DryRun_DedupeRetry / _ParseRetry / _Snapshot) become the
        canonical regression identifiers for Issues 2 & 6 at the pkg/stagecoach boundary.

FROZEN FILES (do NOT edit — S3 is test-only):
  - pkg/stagecoach/stagecoach.go, internal/generate/*, internal/signal/*, internal/provider/*,
        internal/prompt/*, internal/git/*, internal/cmd/*, internal/config/*, internal/exitcode/*,
        cmd/stubagent/*, go.mod, go.sum, Makefile, README.md, docs/*.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the edited file
gofmt -w pkg/stagecoach/stagecoach_test.go

# Vet the package (compiles stagecoach.go + the test file)
go vet ./pkg/stagecoach/

# Confirm ONLY the test file changed; go.mod/go.sum + source untouched
git diff --stat
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged ✓"
git diff --exit-code pkg/stagecoach/stagecoach.go internal/ && echo "source/frozen files untouched ✓"

# Expected: Zero errors. `git diff --stat` lists ONLY pkg/stagecoach/stagecoach_test.go.
```

### Level 2: Unit Tests (THE KEYSTONE — the refined + net-new tests + full suite)

```bash
# The four S3 test cases specifically (refined timeout + 3 net-new)
go test -race -v ./pkg/stagecoach/ -run 'TestGenerateCommit_Timeout|TestGenerateCommit_DryRun_DedupeRetry|TestGenerateCommit_DryRun_ParseRetry|TestGenerateCommit_DryRun_Snapshot'

# The whole stagecoach package (happy-path DryRun, SystemExtra, MissingProvider, ResolveConfig, …)
go test -race -v ./pkg/stagecoach/

# Full repo regression (S3 must not break any other package — e.g. internal/generate dup/parse tests)
go test -race ./...

# Expected: ALL green.
#   - TestGenerateCommit_Timeout/{dryrun,commit_path} PASS (refined dryrun asserts *RescueError+Kind+124).
#   - TestGenerateCommit_DryRun_DedupeRetry PASS (Message=="feat: fresh" — dup rejected).
#   - TestGenerateCommit_DryRun_ParseRetry PASS (Message=="feat: good after parse retry" — FR29 retried).
#   - TestGenerateCommit_DryRun_Snapshot PASS (≥1 new tree object; HEAD unchanged; CommitSHA=="").
#   - If a net-new test FAILS, it signals an S1/S2 regression — investigate runPipeline; do NOT edit
#     source in this subtask without surfacing the finding.
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-module build + vet (test-only change must not break the build)
go build ./... && go vet ./...

# Expected: clean. (No binary behavior change — S3 adds tests only.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Direct proof the snapshot test is meaningful (not a vacuous ≥0): in a quick manual dry run, confirm
# `git cat-file --batch-all-objects --batch-check` enumerates a `tree` that did not exist before the run.
# (The unit test in Task 6 already encodes this assertion deterministically — this is just a sanity lens.)

# Confirm the stub script/counter contract end-to-end at the pkg boundary (already covered by Task 4/5):
#   attempt 1 = responses[0], attempt 2 = responses[1], blank ⇒ parse failure ⇒ retry. (cmd/stubagent
#   selectScripted + internal/stubtest/stubtest_test.go TestStub_ScriptCallVarying already pin this.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `gofmt -l pkg/stagecoach/` empty; `go vet ./pkg/stagecoach/` clean.
- [ ] `go test -race ./...` green (refined `TestGenerateCommit_Timeout/"dryrun"` + the 3 net-new tests +
      every pre-existing test across all packages).
- [ ] `git diff --stat` lists ONLY `pkg/stagecoach/stagecoach_test.go`.
- [ ] `git diff --exit-code go.mod go.sum` empty; `pkg/stagecoach/stagecoach.go` and every `internal/*`
      file byte-unchanged.

### Feature Validation

- [ ] `TestGenerateCommit_Timeout/"dryrun"` asserts `*RescueError` with `re.Kind==ErrTimeout`,
      `re.TreeSHA!=""`, and `exitcode.For(err)==exitcode.Timeout` (124); stale comment fixed; redundant
      `errors.Is` removed.
- [ ] `TestGenerateCommit_DryRun_DedupeRetry` returns `"feat: fresh"` (dup `"feat: existing"` rejected).
- [ ] `TestGenerateCommit_DryRun_ParseRetry` returns `"feat: good after parse retry"` (blank first
      attempt retried, no plain error).
- [ ] `TestGenerateCommit_DryRun_Snapshot` shows ≥1 NEW `tree` object + HEAD unchanged + `CommitSHA==""`.
- [ ] `setupScriptedRepo` + `looseObjectTypes` helpers added and reused by the new tests.

### Code Quality Validation

- [ ] New helpers mirror `setupTestRepo` / `objectCountLine` (no new pattern invented).
- [ ] New tests mirror `TestCommitStaged_DedupeRetryThenSuccess` (commit-path counterparts).
- [ ] Conventional `"feat:"`-prefixed subjects used (no ParseOutput edge cases).
- [ ] No `MaxDuplicateRetries=0` trick in the retry tests (default budget preserved so retries SUCCEED).
- [ ] No new import; no new type; no source/doc/deps change.

### Documentation & Deployment

- [ ] No docs changed (contract: DOCS none). No new env vars / config / exit codes.
- [ ] Test names are self-documenting and grouped with the existing dry-run/timeout tests.

---

## Anti-Patterns to Avoid

- ❌ Don't edit ANY source file (`pkg/stagecoach/stagecoach.go`, `internal/*`, `cmd/*`) — S1/S2 shipped;
  S3 is test-only. A failing net-new test is a finding, not a mandate to change source here.
- ❌ Don't edit docs, README, go.mod/go.sum, or Makefile (contract: test-only; DOCS none).
- ❌ Don't extend `setupTestRepo` to support script mode by changing its signature (risk to all existing
  callers) — add the `setupScriptedRepo` sibling helper instead.
- ❌ Don't set `MaxDuplicateRetries=0` in the dup/parse-RETRY tests — that forces exhaustion→rescue (the
  opposite of proving retry-then-success). Use the default budget (3 → 4 attempts).
- ❌ Don't rely on a bare non-conventional subject (e.g. "initial") as the dup target — use `"feat:"`-
  prefixed subjects to mirror the canonical test and avoid ParseOutput edge cases.
- ❌ Don't assert the snapshot via `objectCountLine`'s `count:` increment alone if you can use
  `looseObjectTypes` (the latter proves the new object is a `tree`, matching the contract's "git cat-file
  finds the snapshotted tree"; `count:` only proves "some object" appeared).
- ❌ Don't leave the redundant duplicate `errors.Is(err, ErrTimeout)` check in the refined "dryrun"
  subtest — drop it; keep one `errors.Is` + the new `re.Kind` + `exitcode.For` assertions.
- ❌ Don't add a new import (`path/filepath`) — use string concat (`dir+"/script.txt"`) to match
  `setupTestRepo`'s existing style.
- ❌ Don't pin the loop's INTERNAL attempt count (e.g. "exactly 2 stub invocations") — assert the FINAL
  message; the loop budget is an implementation detail.
