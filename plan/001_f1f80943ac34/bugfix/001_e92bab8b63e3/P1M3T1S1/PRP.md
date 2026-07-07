# PRP — P1.M3.T1.S1: Take the `write-tree` snapshot unconditionally in dry-run (Issue 6)

> **Scope discipline.** This subtask is the **surgical gate-removal** for PRD Issue 6: in
> `pkg/stagecoach.runPipeline`, `git write-tree` (`deps.Git.WriteTree`) and the
> `signal.SetSnapshot(treeSHA, parentSHA, "")` rescue-arming call are currently wrapped in
> `if !dryRun { … }`, so a dry-run writes **no** snapshot object and never arms rescue. FR49 requires the
> full `diff→snapshot→generate→…` pipeline. **S1 removes the gate** so both run for the dry-run path too
> (the harmless dangling tree in dry-run is intentional, per FR49 / decision **D3**).
>
> S1 **only** un-conditions the snapshot + signal-arming. It deliberately **does NOT**:
> - replace the dry-run single-pass branch with the bounded dedupe/retry loop (that is **S2** /
>   decision D3 item 2),
> - flip the dry-run timeout/parse-fail errors to `*RescueError` carrying `treeSHA` (that is **S2** /
>   D3 item 4),
> - write any new tests (the snapshot/timeout/dup-retry tests are **S3**).
>
> After S1, dry-run behaves exactly as before *up to and including the snapshot*: it still runs the
> single-pass generation, still returns `CommitSHA:""` / bare `ErrTimeout`, and the dangling tree + armed
> signal are the only new side-effects. This keeps the change **regression-safe** (proof in §Validation).

---

## Goal

**Feature Goal**: `runPipeline` takes the `write-tree` snapshot and calls `signal.SetSnapshot` on
**both** the commit path and the dry-run path, so dry-run faithfully mirrors the real pipeline up to
(but not including) `commit-tree`/`update-ref` — satisfying FR49's "full … pipeline" clause and the
P1.M4.T4.S1 contract ("the snapshot is still taken (write-tree runs) but commit-tree/update-ref are
skipped").

**Deliverable**:
1. One source edit in `pkg/stagecoach/stagecoach.go` (`runPipeline`, lines ~244-252): remove the
   `if !dryRun { … }` wrapper so `deps.Git.WriteTree(ctx)` + `signal.SetSnapshot(treeSHA, parentSHA, "")`
   run unconditionally; update the now-misleading "commit path only … DryRun skips it" comment.
2. A docs **verification** (Mode A): confirm via `grep` that no doc under `docs/` or `README.md` claims
   dry-run skips the snapshot; correct one **only if** such a claim is found. (Verified at planning time:
   none exists → no doc edit is required for S1; see "What".)

**Success Definition**:
- A dry-run that **succeeds** now writes one new tree object (a dangling tree) to the object store and
  arms the signal-rescue path — yet still returns `CommitSHA:""`, leaves HEAD unchanged, and prints the
  preview message (exit 0). A dry-run that **times out** still returns the bare `ErrTimeout` sentinel
  (unchanged by S1 — the `*RescueError` flip is S2).
- `go build ./...`, `go vet ./...`, `gofmt -l` clean, and the **entire existing** `go test -race ./...`
  suite stays green (the change is behavior-preserving for every existing assertion — see the regression
  proof in Validation Level 2).

---

## Why

- **PRD §9.12 FR49** (verbatim): *"`--dry-run` — run the full diff→**snapshot**→generate→parse→
  duplicate-check pipeline, print the resulting message, but **do not** create the commit or move HEAD.
  Exit 0."* The "do not create the commit / move HEAD" clause is honored today; the "**snapshot**" clause
  is not — `WriteTree` is gated behind `if !dryRun` (`stagecoach.go:246`).
- **P1.M4.T4.S1 work-item contract** (referenced by the bug report): *"the snapshot is still taken
  (write-tree runs) but commit-tree/update-ref are skipped."*
- **Decision D3** (binding, `architecture/decisions.md`): item 1 — *"Take the `WriteTree` snapshot
  unconditionally (remove the `if !dryRun` gate) and call `signal.SetSnapshot` for both paths."* (Items
  2-4 are explicitly deferred to S2/S3.)
- **Root cause** (full trace: `architecture/seam_dryrun.md` §2(a)): in `runPipeline`, the snapshot block
  at `stagecoach.go:244-252` is the only thing that differs structurally between the two paths at the
  snapshot step. Removing the gate makes dry-run create the same immutable tree object the commit path
  does, and arms the same §18.4 rescue machinery — so a Ctrl-C during a *dry-run* generation now produces
  the correct rescue guidance (tree SHA + manual recovery) instead of a bare exit 130/143.
- **Impact of the fix**: functionally tiny (a harmless dangling tree object + an armed-but-harmless-on-
  exit signal), but it removes a contract/spec deviation and unblocks S2 (the dedupe/retry loop needs a
  real `treeSHA` to populate `RescueError.TreeSHA` on dry-run timeout/exhaustion).

---

## What

In `pkg/stagecoach/stagecoach.go`, function `runPipeline`, replace the gated snapshot block (lines
**244-252**) with an unconditional one. Concretely, today:

```go
	// Step 3 (commit path only): snapshot. DryRun skips it (no commit → no object-store write).
	var treeSHA string
	if !dryRun {
		treeSHA, err = deps.Git.WriteTree(ctx)
		if err != nil {
			return Result{}, err
		}
		signal.SetSnapshot(treeSHA, parentSHA, "") // arm rescue (§18.4)
	}
```

becomes:

```go
	// Step 3: snapshot (FR49 — dry-run runs the full diff→snapshot→… pipeline; the dangling tree in
	// dry-run is intentional and harmless — commit-tree/update-ref are skipped later for dry-run).
	treeSHA, err = deps.Git.WriteTree(ctx)
	if err != nil {
		return Result{}, err
	}
	signal.SetSnapshot(treeSHA, parentSHA, "") // arm rescue (§18.4) — both the commit and dry-run paths
```

That is the **entire** source change. Notes for the implementer:
- `err` is already in scope from steps 1-2 (`RevParseHEAD`, `StagedDiff`), and `treeSHA` is read later by
  the commit-path `*RescueError`/`CommitTree` — so keeping `var treeSHA string` above + `treeSHA, err = …`
  compiles, as does collapsing to `treeSHA, err := deps.Git.WriteTree(ctx)` and dropping the `var`. Either
  is fine; pick whichever `gofmt`/`go vet` accept. **Do not** reorder surrounding steps.
- The dry-run single-pass branch at `// ---- DryRun: single pass, no commit. ----` (~line 277) is
  **untouched** by S1. After this change `treeSHA` is populated in dry-run but not yet referenced by the
  single-pass branch (the `*RescueError{TreeSHA: treeSHA}` wiring is S2). That is expected and temporary —
  it is **not** an unused-variable compile error because `treeSHA` is read in the commit-path branch of
  the same function.
- `signal.SetSnapshot` is **nil-safe** (`internal/signal/signal.go`: a no-op when no handler is installed
  via `signal.Install`). The `pkg/stagecoach` tests never install a handler, so the new call is
  unobservable there. In the CLI it is the *intended* FR49 behavior (rescue arms for dry-run too).

### Docs (Mode A) — verify-and-skip

The contract's doc clause is conditional: *"correct docs/how-it-works.md if it states dry-run skips the
snapshot; affirm docs/cli.md `--dry-run` row if it implies the snapshot is skipped."* Verified at planning
time that **neither condition holds**, so **no doc edit is required for S1**:

- `docs/how-it-works.md` has **no** dry-run description at all (TOC: snapshot flow, stage-while-generating,
  safety/rescue, prompt engineering — nothing on dry-run). Nothing to correct.
- `docs/cli.md` `--dry-run` row reads *"Generate and print the message; do not commit."* — accurate; it
  does **not** imply the snapshot is skipped. The `--dry-run` examples likewise say only "without
  committing".
- Project-wide grep `grep -rn -i "dry.run" docs/ README.md | grep -i "snapshot\|write.tree\|skip"` returns
  **empty**.

The implementer must re-run that grep as a **gate** (Validation Level 4) and, *if and only if* a doc now
claims dry-run skips the snapshot, correct it to *"dry-run runs the full pipeline including the snapshot,
omitting only the commit."* Do **not** proactively add a new dry-run narrative to `docs/how-it-works.md`
— that is **P1.M5.T1.S2** (docs sweep) scope; keeping S1 surgical avoids colliding with that future work
item.

### Success Criteria

- [ ] `runPipeline` calls `deps.Git.WriteTree(ctx)` and `signal.SetSnapshot(treeSHA, parentSHA, "")`
      **unconditionally** (no `if !dryRun` gate around either).
- [ ] A successful dry-run writes exactly one new tree object (the dangling snapshot) and leaves HEAD
      unchanged, returning `CommitSHA:""` + the preview message.
- [ ] The dry-run single-pass branch is otherwise byte-for-byte unchanged (S2 owns the loop replacement).
- [ ] Every currently-green test stays green (no new tests added in S1).
- [ ] Docs grep gate passes (no doc claims dry-run skips the snapshot).

---

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the exact function, exact line range, before/after block, the nil-safety proof
for the signal call, the regression-safety proof (which tests guard object counts and why they're
unaffected), the binding decision, and the verify-and-skip docs gate are all below.

### Documentation & References

```yaml
# MUST READ — binding architecture (root-cause trace + decision)
- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/decisions.md
  why: D3 item 1 is the binding decision for THIS subtask — remove the `if !dryRun` gate, call
       signal.SetSnapshot for both paths. (D3 items 2-4 are S2/S3 — out of scope here.)
  section: "D3 — Fix Issues 2 & 6 by routing dry-run through the full loop"
  critical: D3 item 4 explicitly flags that the timeout→*RescueError flip + the
            TestGenerateCommit_Timeout/dryrun assertion update are a LATER step (S2/S3) — do NOT do
            them in S1.

- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/seam_dryrun.md
  why: §2(a) quotes the exact gated block; the "Start Here" / Acceptance sections name the edit site
       (stagecoach.go:244-252) and the residual risk that signal side-effects now fire in dry-run.
  section: "2. The DryRun code path (runPipeline) → (a) if !dryRun gates WriteTree (Issue 6)"

# The edit site + the surrounding pipeline
- file: pkg/stagecoach/stagecoach.go
  why: runPipeline (the DryRun/SystemExtra entrypoint). The snapshot block is lines 244-252 (Step 3).
       The dry-run single-pass branch it arms into starts at ~line 277 ("---- DryRun: single pass").
  pattern: Step 3 sits between Step 2 (StagedDiff → ErrNothingToCommit) and Step 4 (buildSysPrompt).
       Keep that ordering — the snapshot must follow the empty-diff check (don't write a tree when there
       is nothing to commit) and precede generation.
  gotcha: `err` is shared across steps 1-3 (RevParseHEAD/StagedDiff/WriteTree); assign, don't re-declare
          with `:=` unless `treeSHA` is the only new LHS. `parentSHA` (from Step 1) is the snapshot parent.

# Why signal.SetSnapshot is safe to call in dry-run
- file: internal/signal/signal.go
  why: SetSnapshot (and every package wrapper) is nil-safe — `if h := active.Load(); h != nil { … }`. No
       handler installed (library/CLI-without-Install) ⇒ no-op. The pkg/stagecoach tests never Install.
  section: "SetSnapshot arms the rescue path" + package doc comment "nil-safe no-ops when no handler"

# Why WriteTree is safe/idempotent in dry-run (it just makes a dangling tree)
- file: internal/git/git.go
  why: WriteTree "writes a tree object to the object store but does NOT modify the index or HEAD"
       (read-only-w.r.t.-refs). A dry-run tree is dangling until/unless commit-tree publishes it — which
       dry-run never does. Hence "harmless dangling tree".
  section: WriteTree doc comment (~line 209)

# Regression-proof anchor: the only object-count guards are missing-provider tests (fail pre-WriteTree)
- file: pkg/stagecoach/stagecoach_test.go
  why: TestGenerateCommit_MissingProviderCommand_Issue3 (+ its dryrun subtest, ~350-421) assert object
       count unchanged, BUT use a non-existent command → buildDeps pre-flight errors before runPipeline →
       WriteTree never runs → unaffected by S1. TestGenerateCommit_DryRun (~169) checks HEAD/message only
       (not object count) → unaffected. TestGenerateCommit_Timeout/dryrun (~224) asserts bare ErrTimeout
       (the dry-run branch is unchanged by S1) → unaffected.
  gotcha: Do NOT add an object-count-unchanged assertion for a SUCCESSFUL dry-run — after S1 a successful
          dry-run DOES write a dangling tree, so such an assertion would (correctly) be WRONG. The
          snapshot-creation test belongs to S3.

# Docs gate (Mode A) — verify, don't blindly edit
- file: docs/cli.md
  why: `--dry-run` row ("do not commit") + examples already say nothing about skipping the snapshot.
       No edit unless a grep finds a contradicting statement.
  section: flags table (~line 26) + "## Examples"
- file: docs/how-it-works.md
  why: Has no dry-run section at all. No edit (adding a dry-run narrative is P1.M5.T1.S2 scope).
```

### Current Codebase tree (relevant slice)

```bash
pkg/stagecoach/stagecoach.go          # runPipeline — THE EDIT SITE (snapshot block L244-252)
internal/signal/signal.go           # SetSnapshot — nil-safe; called unconditionally is safe
internal/git/git.go                 # Git.WriteTree (L209) — read-only-w.r.t.-refs; dangling tree ok
pkg/stagecoach/stagecoach_test.go     # DryRun/Timeout/MissingProvider tests — regression anchors
docs/cli.md                         # --dry-run row/examples — verify-only (no skip-snapshot claim)
docs/how-it-works.md                # no dry-run section — verify-only (no edit)
```

### Desired Codebase tree with files changed

```bash
pkg/stagecoach/stagecoach.go   # MODIFIED — runPipeline Step 3: un-gate WriteTree + signal.SetSnapshot
# (no new files; no other source files touched; no new tests in S1)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: Keep the dry-run single-pass branch UNCHANGED. S1 only un-conditions the snapshot + signal
//   arming. The dedupe/retry loop (S2), the timeout→*RescueError flip (S2/D3-item-4), and the new tests
//   (S3) are deliberately out of scope. If you "tidy up" by merging the dry-run branch into the commit
//   loop now, you will collide with S2 and break the locked-in TestGenerateCommit_Timeout/dryrun assertion.

// CRITICAL: signal.SetSnapshot is nil-safe, but it IS observable in the CLI (signal.Install runs in
//   main.go). After S1, a Ctrl-C during a dry-run generation will print the §18.3 rescue block (tree SHA +
//   manual recovery) and exit 3 — this is the INTENDED FR49 behavior, not a regression. The pkg/stagecoach
//   unit tests never Install, so they cannot observe it.

// CRITICAL: A SUCCESSFUL dry-run now writes a dangling tree object. Do NOT add (or assume) any test that
//   asserts the object count is unchanged after a successful dry-run — that would be wrong post-fix.
//   The missing-provider object-count guards stay valid only because buildDeps errors pre-WriteTree.

// GOTCHA: `err` is reused across runPipeline steps 1-3; assign with `=` (not `:=`) unless `treeSHA` is the
//   only new LHS. `parentSHA` comes from Step 1 (RevParseHEAD) and is the snapshot's parent — pass it
//   through to signal.SetSnapshot unchanged (it is "" on a root/unborn commit, which FormatRescue handles).
```

---

## Implementation Blueprint

### The exact edit (Task 1)

In `pkg/stagecoach/stagecoach.go`, `runPipeline`, **replace lines 244-252** (the `// Step 3 (commit path
only) …` comment + `var treeSHA string` + the `if !dryRun { … }` block) with the unconditional version
shown in the "What" section above. No other line in the function (or file) changes.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY pkg/stagecoach/stagecoach.go :: runPipeline, Step 3 (snapshot)
  - REPLACE: the gated block at lines 244-252 (`var treeSHA string` + `if !dryRun { WriteTree; SetSnapshot }`)
             with the unconditional version (WriteTree + signal.SetSnapshot always run).
  - UPDATE COMMENT: the old "Step 3 (commit path only): snapshot. DryRun skips it …" is now FALSE —
             replace with a comment stating the snapshot runs for both paths per FR49 (dangling tree in
             dry-run is intentional/harmless; commit-tree/update-ref are skipped later for dry-run).
  - DO NOT TOUCH: the dry-run single-pass branch (~line 277), the commit-path loop, CommitStaged,
             signal.go, git.go, executor.go, or any *_test.go file.
  - NAMING/PLACEMENT: keep `treeSHA`/`err`/`parentSHA` exactly as the surrounding steps use them.
  - DEPENDENCIES: none new (WriteTree + signal.SetSnapshot already imported/called here).

Task 2: VERIFY docs (Mode A — gate, not an edit task)
  - RUN: `grep -rn -i "dry.run" docs/ README.md | grep -i "snapshot\|write.tree\|skip\|no.*object"`
  - EXPECT: empty output (no doc claims dry-run skips the snapshot). If empty → NO doc edit for S1.
  - IF NON-EMPTY (a doc does claim dry-run skips the snapshot): correct that single statement to
             "dry-run runs the full pipeline including the snapshot, omitting only the commit."
  - DO NOT: proactively add a dry-run section to docs/how-it-works.md — that is P1.M5.T1.S2 scope.
```

### Implementation Patterns & Key Details

```go
// PATTERN: runPipeline already calls signal.SetSnapshot exactly once, right after a successful WriteTree,
// on the commit path. S1 makes that single call run for both paths — it does not add a second call, nor
// does it touch the later signal.RestoreDefault()/signal.ClearSnapshot() calls (those stay commit-path
// only; dry-run returns before them, leaving the snapshot armed until process exit — harmless).
//
// GOTCHA: the "dangling tree in dry-run is harmless" claim rests on WriteTree being read-only w.r.t. refs
// (internal/git/git.go doc: "writes a tree object … does NOT modify the index or HEAD"). A dry-run never
// calls CommitTree, so the tree stays dangling and HEAD/index are untouched — exactly FR49.
//
// WHY NOT also wire treeSHA into the dry-run RescueError now: because the dry-run single-pass still
// returns the BARE ErrTimeout sentinel (its branch is unchanged in S1). Changing that error shape is S2
// (D3 item 4), and the test that locks the bare-sentinel behavior (TestGenerateCommit_Timeout/dryrun) is
// updated in S3. Doing it now would break that locked-in test prematurely.
```

### Integration Points

```yaml
CODE: one block in pkg/stagecoach/stagecoach.go::runPipeline (no new imports, exports, or API change).
DATABASE/OBJECT STORE: dry-run now writes one dangling tree object (git object store) — intentional.
CONFIG: none.
ROUTES: none.
SIGNALS: signal.SetSnapshot now arms rescue in dry-run too (nil-safe; intended FR49 behavior in the CLI).
```

---

## Validation Loop

### Level 1: Syntax & Style (run after Task 1)

```bash
go build ./...
go vet ./pkg/stagecoach/...
gofmt -l pkg/stagecoach/stagecoach.go     # expect: no output
# If gofmt lists the file, run: gofmt -w pkg/stagecoach/stagecoach.go
```

### Level 2: Existing tests (regression guard — S1 writes NO new tests)

```bash
# Whole suite with -race (the Makefile `make test` target is exactly this).
go test -race ./...

# Fast targeted re-run of the dry-run-relevant package:
go test -race ./pkg/stagecoach/...
# Expected: all PASS. Reasoning (why each dry-run-touching test stays green):
#   * TestGenerateCommit_DryRun        — checks CommitSHA==""/Message/HEAD, NOT object count.
#   * TestGenerateCommit_Timeout/dryrun — still returns bare ErrTimeout (dry-run branch unchanged).
#   * TestGenerateCommit_MissingProviderCommand_Issue3 (+dryrun subtest) — buildDeps errors before
#     runPipeline, so WriteTree never runs → object-count-unchanged guard still holds.
#   * No pkg/stagecoach test installs a signal handler → signal.SetSnapshot is an unobservable no-op.
```

> **If a previously-green test now fails**: it is almost certainly an object-count assertion firing on a
> **successful** dry-run (which now correctly writes a dangling tree). That assertion would itself be wrong
> post-fix — but per the proof above, no such assertion exists in the current suite, so a failure means you
> accidentally changed the dry-run branch or commit path. Revert to the single-block edit. Do NOT weaken
> any missing-provider object-count guard (those stay valid because `buildDeps` errors pre-`WriteTree`).

### Level 3: Manual / end-to-end (proves the snapshot is now taken in dry-run)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach

TMP=$(mktemp -d) && cd "$TMP"
git init -q && git config user.email t@e.com && git config user.name t
git commit -q --allow-empty -m init
echo hi > a.txt && git add a.txt

before=$(git count-objects -v | awk '/^count:/{print $2}')
/tmp/stagecoach --provider <an-installed-stub-or-builtin> --dry-run; echo "EXIT=$?"
after=$(git count-objects -v | awk '/^count:/{print $2}')

# EXPECT: EXIT=0, a preview message printed, HEAD UNCHANGED, and `after` == `before + 1`
#         (exactly ONE new object — the dangling write-tree snapshot). Before S1, before==after.
echo "objects: before=$before after=$after  (expect after == before+1)"
[ "$after" -eq "$((before+1))" ] && echo "OK: dry-run wrote the snapshot tree" || echo "FAIL: no snapshot written"
git rev-parse HEAD >/dev/null && echo "HEAD intact"
```

(If you don't have a stub/built-in handy, the same proof runs at the library level against
`GenerateCommit(ctx, Options{Provider:"stub", DryRun:true})` — assert `git count-objects` count increments
by exactly 1 and HEAD is unchanged. See `TestGenerateCommit_DryRun` in `pkg/stagecoach/stagecoach_test.go`
for the repo/stub setup helpers: `setupTestRepo`, `writeFile`, `stageFile`, `headSHA`.)

### Level 4: Docs gate (verify-and-skip)

```bash
# MUST be empty — confirms no doc claims dry-run skips the snapshot.
grep -rn -i "dry.run" docs/ README.md | grep -i "snapshot\|write.tree\|skip\|no.*object" || echo "OK: no skip-snapshot doc claim"
# If non-empty, correct ONLY the offending statement (see Task 2). Do not add new dry-run prose here.
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean.
- [ ] `go vet ./pkg/stagecoach/...` clean (or `go vet ./...`).
- [ ] `gofmt -l pkg/stagecoach/stagecoach.go` reports nothing.
- [ ] `go test -race ./...` — all previously-green tests still PASS (no new tests added in S1).
- [ ] Manual repro (Level 3): successful dry-run → exit 0, preview message, HEAD unchanged, and
      `git count-objects` count increases by **exactly 1** (the dangling snapshot tree).
- [ ] Docs grep gate (Level 4) is empty (no doc claims dry-run skips the snapshot).

### Feature Validation
- [ ] `WriteTree` runs for **both** the commit and dry-run paths (the `if !dryRun` gate is gone).
- [ ] `signal.SetSnapshot(treeSHA, parentSHA, "")` runs for **both** paths.
- [ ] The dry-run single-pass branch is otherwise unchanged (still returns `CommitSHA:""`; still returns
      bare `ErrTimeout` on timeout — the `*RescueError` flip is S2).
- [ ] `parentSHA` is passed through unchanged (root/unborn repo → `""`, handled by FormatRescue).

### Code Quality Validation
- [ ] Single, surgical block edit in `runPipeline`; no edits to CommitStaged, the dry-run branch body,
      signal.go, git.go, executor.go, or any `*_test.go`.
- [ ] The Step-3 comment no longer claims dry-run skips the snapshot.
- [ ] No speculative doc additions (proactive dry-run narrative stays in P1.M5.T1.S2 scope).

### Documentation
- [ ] Docs gate run and recorded (empty = no edit required; non-empty = the single offending line corrected).

---

## Anti-Patterns to Avoid

- ❌ **Don't replace the dry-run single-pass with the dedupe/retry loop here** — that is S2 (D3 item 2).
  S1 only un-gates the snapshot.
- ❌ **Don't change the dry-run timeout/parse-fail errors to `*RescueError`** — that is S2 (D3 item 4),
  and the `TestGenerateCommit_Timeout/dryrun` assertion that locks the bare `ErrTimeout` is updated in S3.
- ❌ **Don't write new tests in S1** — the snapshot/dup-retry/timeout tests are S3.
- ❌ **Don't add an object-count-unchanged assertion for a successful dry-run** — post-fix a successful
  dry-run correctly writes a dangling tree, so such an assertion would be wrong.
- ❌ **Don't reorder runPipeline steps** — the snapshot must stay after the empty-diff check and before
  generation.
- ❌ **Don't touch the commit-path `signal.RestoreDefault()`/`ClearSnapshot()` calls** — dry-run returns
  before them by design; the armed snapshot disarms naturally at process exit.
- ❌ **Don't proactively add dry-run prose to `docs/how-it-works.md`** — that is P1.M5.T1.S2 scope; keep
  S1's docs work to the verify-and-skip gate.

---

## Confidence Score

**9/10** — The fix is a single, precisely-located block edit (lines 244-252) with an exact before/after,
the nil-safety of the newly-unconditional `signal.SetSnapshot` is proven, the regression-safety of the
entire existing suite is proven (object-count guards live only in missing-provider tests that fail
pre-`WriteTree`), and the docs work is a verified no-op gate. The -1 reserves for the manual-repro
dependency on an installed stub/built-in provider in Level 3 (which has a library-level fallback noted),
and the judgment call of collapsing vs. keeping the `var treeSHA string` declaration — both non-blocking.
