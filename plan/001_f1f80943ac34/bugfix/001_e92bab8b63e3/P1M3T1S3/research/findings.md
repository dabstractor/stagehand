# P1.M3.T1.S3 — Research Findings (test-only: pin S1+S2 dry-run fixes)

Scope: ADD net-new dry-run tests + refine the locked-in timeout subtest. S1 (Issue 6, unconditional
`WriteTree`) and S2 (Issue 2, full dedupe/retry loop) ALREADY SHIPPED the behavior; S3 only PINS/PROVES
it with tests. **No source changes, no docs** (contract OUTPUT: test-only). The new tests should pass
immediately (they verify already-working behavior); if one fails, it surfaces an S1/S2 regression.

## 0. State of the tree BEFORE S3 (verified)

- `git log --oneline -3`: `f8db87e replace dry-run single-pass with full dedupe/retry loop` (S2),
  `fb73011 unconditionally take write-tree snapshot in dry-run path` (S1), `0b38ce6 …Issue3 tests`.
- Baseline GREEN: `go test -race -run 'TestGenerateCommit_Timeout|TestGenerateCommit_DryRun' ./pkg/stagecoach/`
  → PASS (incl. `TestGenerateCommit_Timeout/dryrun` and `…/commit_path`).
- `pkg/stagecoach/stagecoach.go runPipeline` already has: unconditional `WriteTree` + `signal.SetSnapshot`
  (S1); ONE shared generate→parse→dedupe loop serving BOTH dryRun and !dryRun; dry-run success
  early-return (`if dryRun { signal.ClearSnapshot(); return Result{CommitSHA:""} }`) after `if !success`;
  dry-run timeout/exhaustion returns `&generate.RescueError{Kind, TreeSHA, …}` (real TreeSHA from S1).
  → S3 does NOT touch this file.

## 1. Part (a) — TestGenerateCommit_Timeout/"dryrun": ALREADY flipped by S2, but needs REFINEMENT

S2's commit `f8db87e` already changed the dryrun subtest from "must NOT be *RescueError" to "must BE
*RescueError with non-empty TreeSHA" (it passes today). Remaining gaps vs. the S3 contract
("Kind==ErrTimeout … and exitcode.For==124, mirroring commit-path"):

- STALE leading comment: `// DryRun path: ErrTimeout (bare sentinel, no TreeSHA).` → now WRONG.
- REDUNDANT duplicate `errors.Is(err, ErrTimeout)` check (appears twice; harmless but sloppy).
- MISSING: `re.Kind == ErrTimeout` (or `errors.Is(re.Kind, ErrTimeout)`) — contract requires it.
- MISSING: `exitcode.For(err) == exitcode.Timeout` (124) — contract requires it; `exitcode` is already
  imported in this test file (used by `TestGenerateCommit_MissingProviderCommand_Issue3`).

→ S3 cleans the comment, dedupes the redundant check, and ADDS `re.Kind` + `exitcode.For` assertions.
The `exitcode.For` mapping is proven: `exitcode.go:65` `errors.Is(err, generate.ErrTimeout) → Timeout
(124)`, and `*RescueError.Unwrap()==Kind` (generate.go:93) chains `errors.Is` through. exitcode_test.go
already pins `RescueError{Kind:ErrTimeout} → 124`.

## 2. Parts (b)/(c)/(d) — net-new tests (NONE exist today)

Existing tests in stagecoach_test.go: Success, DryRun (happy-path), NothingStaged, ProviderOverride,
Timeout{dryrun,commit_path}, SystemExtra, MissingProviderCommand_Issue3, ResolveConfig_InjectedConfig.
NO dry-run dup-retry / parse-retry / snapshot test. These are S3's deliverables.

## 3. The harness gap — setupTestRepo does NOT support stub SCRIPT mode

- `setupTestRepo(t, stubtest.Options)` only emits `STAGECOACH_STUB_OUT` and `STAGECOACH_STUB_SLEEP_MS`
  into `[provider.stub.env]`. It has NO path for call-varying (script) mode.
- dup-retry (b) and parse-retry (c) REQUIRE script mode (call 1 ≠ call 2).
- Resolution: add a sibling helper `setupScriptedRepo(t, headSubject, responses)` that writes a
  script file + counter file in `t.TempDir()` and emits `STAGECOACH_STUB_SCRIPT`/`STAGECOACH_STUB_COUNTER`
  into the `[provider.stub.env]` block. Mirrors `setupTestRepo`'s chdir/cleanup/TOML pattern and the
  inline-TOML precedent in `TestGenerateCommit_MissingProviderCommand_Issue3`. No `filepath` import
  needed (use `dir+"/script.txt"` like setupTestRepo's `repo+"/.stagecoach.toml"`).

## 4. stub script semantics (cmd/stubagent/main.go selectScripted)

- `STAGECOACH_STUB_SCRIPT` = path to a file; `lines = split(content, "\n")`; `STAGECOACH_STUB_COUNTER`
  = path to a counter file (ABSENT ⇒ reads 0). Each stub process: `index=readCounter()`,
  returns `lines[index]` (clamped to last when exhausted), writes `index+1`. Blank line ⇒ empty stdout
  ⇒ `provider.ParseOutput(out, m)` returns `ok==false` ⇒ loop treats it as a parse failure (FR29 retry).
- Per-call: each stub invocation is a FRESH process, so the file-backed counter is the ONLY state that
  carries between attempts. Serial callers ⇒ no race. Each test gets its own t.TempDir counter ⇒ no
  cross-test interference.

## 5. The dedupe target — how dup-retry detects a duplicate

- `runPipeline` builds `recent = deps.Git.RecentSubjects(ctx, 50)` ONCE (nil on unborn ⇒ vacuous).
  `RecentSubjects` returns the SUBJECTS of recent commits (first line of each message).
- `generate.IsDuplicate(subject, recent)` is an EXACT subject match.
- ⇒ For attempt 1 to be rejected as a dup, the repo MUST contain a commit whose subject == attempt 1.
- `setupTestRepo`/`setupScriptedRepo` commit one initial commit → its subject is the dup target.
- MIRROR the canonical `internal/generate/generate_test.go:TestCommitStaged_DedupeRetryThenSuccess`
  (commits "feat: existing", script ["feat: existing", "feat: fresh"], asserts "feat: fresh"): use
  conventional "feat:"-prefixed subjects so ParseOutput has no edge case (don't rely on bare "initial").

## 6. Loop budget — ample for the retry tests

- `config.Defaults().MaxDuplicateRetries == 3` (config.go:62) ⇒ loop runs up to 4 attempts.
- pkg/stagecoach tests load config via `.stagecoach.toml` + defaults (GenerateCommit → resolveConfig) ⇒
  MaxDuplicateRetries = 3 (not overridden). dup-retry needs 2 attempts; parse-retry needs 2. ✓.

## 7. Snapshot detection (Issue 6) — prove a tree object was created in dry-run

- WriteTree (`internal/git/git.go:219`) writes the root tree (a loose object) to the object store.
  `git add <file>` writes the BLOB first (during setup); WriteTree writes the TREE during GenerateCommit.
- Capture object set AFTER staging, BEFORE GenerateCommit; diff against AFTER GenerateCommit. The new
  object(s) are the WriteTree tree(s) (dry-run skips commit-tree/update-ref ⇒ no commit/blob added).
- Helper `looseObjectTypes(t, dir)` runs `git cat-file --batch-all-objects --batch-check`
  (output `<sha> <type> <size>` per line) → map[sha]type. Assert `after \ before` contains ≥1 `tree`.
  This DIRECTLY proves "a tree object IS created in dry-run" (the contract's words) and is symmetric to
  the `objectCountLine` guard in MissingProviderCommand (which asserts NO new object). `--batch-all-
  objects` covers loose + packed (robust even if git auto-packs; fresh temp repos are all-loose anyway).

## 8. exitcode + RescueError type facts (for part (a) assertions)

- `internal/exitcode/exitcode.go`: `Timeout = 124`; `For(*RescueError{Kind:ErrTimeout}) == 124`
  (checked before generic rescue→3, via `errors.Is(err, generate.ErrTimeout)`).
- `internal/generate/generate.go:76` `type RescueError struct { Kind error; TreeSHA, ParentSHA,
  Candidate string; Cause error }`; `Unwrap() error { return e.Kind }` (enables errors.Is).
- Sentinels: `ErrTimeout` (generate.go:54), `ErrRescue` (generate.go:59). pkg/stagecoach re-exports
  them as type aliases (`ErrTimeout`, `RescueError`).

## 9. Test naming + placement

All edits in `pkg/stagecoach/stagecoach_test.go` (the ONLY file S3 touches). Co-locate helpers next to
existing fixture helpers (after `objectCountLine`, ~line 161). New tests after the existing
`TestGenerateCommit_Timeout` (mirror the commit-path subtests they parallel):
- (a) UPDATE `TestGenerateCommit_Timeout`/`"dryrun"` (refine assertions + comment).
- (b) ADD `TestGenerateCommit_DryRun_DedupeRetry`.
- (c) ADD `TestGenerateCommit_DryRun_ParseRetry`.
- (d) ADD `TestGenerateCommit_DryRun_Snapshot` (Issue 6).

## 10. Anti-patterns / scope guards

- ❌ Do NOT edit `pkg/stagecoach/stagecoach.go` or any internal/* — S1/S2 shipped; S3 is test-only.
- ❌ Do NOT edit docs (contract: DOCS none). ❌ Do NOT change go.mod/go.sum.
- ❌ Do NOT use a `MaxDuplicateRetries=0` trick for the parse/dup RETRY tests (that's for RESCUE tests,
  e.g. `TestCommitStaged_ParseFailRescue`); the retry tests need the DEFAULT budget so the loop can
  retry and SUCCEED.
- ❌ Do NOT assert on stub output ordering beyond what the loop guarantees — assert the FINAL message.
