---
name: "P3.M4.T1.S2 — Implement per-concept failure isolation (FR-M12) + multi-commit rescue variant (§13.6.6 / §18.3): edit internal/decompose/decompose.go + internal/generate/rescue.go + roles.go"
description: |

  EDIT `internal/decompose/decompose.go` (S1's file — add FR-M12 isolation to `runLoop` + `Decompose` +
  signal arming), EDIT `internal/decompose/roles.go` (add `Out io.Writer` to `Deps`), EDIT
  `internal/generate/rescue.go` (add `FormatRescueMulti`), EDIT `internal/decompose/decompose_test.go`
  (add FR-M12 tests), and EDIT `internal/generate/rescue_test.go` (add `FormatRescueMulti` tests).

  S2 wraps S1's structurally-propagating loop with per-concept failure isolation (PRD §13.6.6 / §18.2 /
  §18.3, §9.14 FR-M12) so that ONE bad concept cannot poison a multi-commit run:

  CONTRACT (P3.M4.T1.S2, verbatim from the work item):
    1. RESEARCH NOTE: FR-M12 / §13.6.6: (a) message[i] generation fails → rescue for concept i ONLY;
       already-published commits 0..i-1 stand; frozen tree[i] + manual recovery printed; remaining
       staged work left in index. (b) CAS failure on commit[i] → abort with §13.5 HEAD-moved message;
       prior commits stand; tree[i] recovery printed. (c) stager stages nothing → skip commit[i]
       (FR-M8). (d) stager exits non-zero twice → treat as empty. The multi-commit rescue variant
       (§18.3) prints tree[i], its parent (newSHA[i-1]), and the same commit-tree|update-ref recipe.
       The overlapped stager[i+1], if running, is allowed to complete so its staging is not lost.
    2. INPUT: The Decompose orchestrator from P3.M4.T1.S1.
    3. LOGIC: In decompose/decompose.go, add error handling to the loop: catch message-generation
       errors (RescueError) → print multi-commit rescue (tree[i], parent, recipe), return partial
       DecomposeResult with commits 0..i-1. Catch CAS errors (CASError) → print §13.5 message, return
       partial result. Catch stager errors after retry → skip concept (log), continue. The multi-commit
       rescue message format: extend generate.FormatRescue or create a decompose-specific variant that
       names the concept and its position in the chain.
    4. OUTPUT: Per-concept failures are isolated — prior commits stand, remaining work is preserved,
       and the user gets actionable recovery instructions.
    5. DOCS: none — internal error handling.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/decompose/{planner,stager,message,arbiter,chain}.go — SHIPPED + S1-consumed. CONSUMED
      VERBATIM: generateMessage (returns *RescueError DIRECTLY), publishCommit (returns *CASError
      DIRECTLY), stageConcept (ErrStagerFailed-wrapped; no retry), freezeSnapshot. DO NOT EDIT.
    - internal/decompose/roles.go — EDIT (this task): add ONE `Out io.Writer` field to Deps (S1
      already added the unexported `stager` seam; S2 adds `Out` — safe, sequential after S1).
    - internal/decompose/decompose.go — EDIT (this task): S1's file. S2 wraps runLoop's publish()
      closure + the stager call with FR-M12 isolation, adds signal arming, adds DecomposeRescueError,
      and updates Decompose to return partial results + skip the arbiter on partial failure.
    - internal/decompose/decompose_test.go — EDIT (this task): S1's file. S2 ADDS TestDecompose_*
      functions + dcm* helpers (no rename of S1's — additive).
    - internal/generate/rescue.go — EDIT (this task): add FormatRescueMulti (reuses rescueSep; same
      package). generate is shipped v1, NOT parallel-owned this cycle → safe to add a function.
    - internal/generate/generate.go — CONSUMED (read-only): RescueError, CASError, ErrTimeout,
      ErrRescue. DO NOT EDIT.
    - internal/signal/signal.go — CONSUMED (read-only): SetSnapshot, ClearSnapshot (nil-safe no-ops
      without Install). DO NOT call RestoreDefault (ONE-SHOT/PERMANENT — §5 gotcha). signal imports NO
      stagehand packages → decompose→signal is NOT an import cycle.
    - internal/git/git.go — CONSUMED (read-only). internal/exitcode — CONSUMED (For mapping).
    - internal/cmd/*, pkg/stagehand/ — UNCHANGED. CLI decompose routing (the double-print suppression
      + partial-commit success reporting) is P4 (P4.M1.T1.S1). S2 is INTERNAL error handling.

  DELIVERABLES (5 file EDITS — no new files):
    EDIT internal/generate/rescue.go — add FormatRescueMulti(treeSHA, parentSHA, candidateMsg,
      conceptTitle string, index, count int) string (§18.3 multi-commit variant; reuses rescueSep).
    EDIT internal/decompose/roles.go — add `Out io.Writer` field to Deps (+ "io" import) — the rescue
      destination (stderr in prod, *bytes.Buffer in tests).
    EDIT internal/decompose/decompose.go — add DecomposeRescueError type; wrap runLoop's stager call
      with retry-once-then-empty (FR-M12d); rewrite the publish() closure to catch *RescueError →
      print FormatRescueMulti + return *DecomposeRescueError, and *CASError → print ce.Error() +
      return it; runLoop returns partial commits on error (not nil); arm/disarm signal per-concept
      (SetSnapshot/ClearSnapshot); Decompose returns (DecomposeResult{Commits:partial}, typedErr) on
      partial failure and SKIPS the arbiter (§18.3).
    EDIT internal/decompose/decompose_test.go — add TestDecompose_MessageRescuePartial,
      TestDecompose_CASAbortPartial, TestDecompose_StagerRetryThenEmpty (FR-M12d),
      TestDecompose_RescueArbiterSkipped + dcm* helpers (dcmOutBuffer).
    EDIT internal/generate/rescue_test.go — add TestFormatRescueMulti_* (header/recipe/candidate/position).

  SUCCESS: `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; `make coverage-gate` PASS (generate IS gated → FormatRescueMulti
  MUST have tests; decompose is NOT gated); go.mod/go.sum UNCHANGED; all S1 tests still pass;
  message[i] failure returns a partial DecomposeResult (commits 0..i-1) + a *DecomposeRescueError that
  errors.As to *generate.RescueError AND errors.Is to generate.ErrRescue/ErrTimeout (→ exit 3/124) and
  prints FormatRescueMulti(tree[i], newSHA[i-1], candidate, title, i, N) to deps.Out; the overlapped
  stager[i+1]'s work remains staged in the index; the arbiter does NOT run; CAS failure returns partial
  + *generate.CASError (→ exit 1) and prints ce.Error() (the §13.5 message); a stager that fails twice
  is treated as empty (concept skipped, ≤N commits, loop continues); FR-M8 empty-skip still works.

---

## Goal

**Feature Goal**: Harden S1's `Decompose`/`runLoop` with per-concept failure isolation (PRD §13.6.6 /
§18.2 / §18.3, §9.14 FR-M12) so that a single concept's generation failure, a CAS (HEAD-moved) failure
on one commit, or a twice-failing stager cannot corrupt a multi-commit run. Specifically: (a) a failed
`message[i]` prints the §18.3 multi-commit rescue scoped to concept i (its frozen `tree[i]`, parent
`newSHA[i-1]`, and the `commit-tree | update-ref` recipe — naming the concept + its position), returns
a PARTIAL `DecomposeResult` carrying the already-published commits 0..i-1, leaves the overlapped
stager[i+1]'s staging in the index, and does NOT run the arbiter; (b) a CAS failure on `commit[i]`
prints the §13.5 "HEAD moved" message (which includes the `tree[i]` recovery command), returns a
partial result, and aborts; (c/d) a twice-failing stager is treated as an empty concept (FR-M8 skip +
continue). S2 also arms the signal handler around each concept's message-generation window (Ctrl-C →
the §18.3 rescue with the correct `tree[i]` + `newSHA[i-1]` parent).

**Deliverable** (5 file EDITS — no new files):
1. `internal/generate/rescue.go` (EDIT) — `FormatRescueMulti` (the §18.3 multi-commit variant).
2. `internal/decompose/roles.go` (EDIT) — `Out io.Writer` field on `Deps`.
3. `internal/decompose/decompose.go` (EDIT) — `DecomposeRescueError` type; FR-M12 isolation in
   `runLoop` (retry-once-then-empty stager + rescue-for-concept-i + CAS abort); per-concept signal
   arming; `Decompose` returns partial results + skips the arbiter on partial failure.
4. `internal/decompose/decompose_test.go` (EDIT) — FR-M12 integration tests + `dcm*` helpers.
5. `internal/generate/rescue_test.go` (EDIT) — `FormatRescueMulti` tests.

**Success Definition**:
- **FR-M12a (message[i] rescue)**: in a 3-concept run where the message agent TIMES OUT for concept 1
  (index 1), `Decompose` returns `(DecomposeResult{Commits: [commit0]}, *DecomposeRescueError)` where
  commit0 is real (HEAD advanced once); `errors.As(err, &*generate.RescueError{})` is TRUE; the error
  `errors.Is(generate.ErrTimeout)` (→ exit 124) or `errors.Is(generate.ErrRescue)` (→ exit 3) per the
  kind; `deps.Out` (a `*bytes.Buffer` in the test) contains `FormatRescueMulti(tree[1], newSHA[0],
  candidate, concept[1].Title, 1, 3)` — i.e. it names "concept 2 of 3" and shows the recipe with
  `-p <newSHA[0]>`; concept 2's stager had already run (synchronously) so its files remain staged
  (`git status --porcelain` non-empty, index holds concepts[0..2]); the arbiter stub's Execute is
  NEVER called (the loop aborted before the StatusPorcelain gate / arbiter phase).
- **FR-M12b (CAS abort)**: in a run where the stager seam (or an external HEAD move) makes HEAD !=
  newSHA[i-1] at `publishCommit`'s CAS for commit i, `Decompose` returns
  `(DecomposeResult{Commits: [0..i-1]}, *generate.CASError)`; `errors.As(err, &*generate.CASError{})`
  is TRUE → `errors.Is(git.ErrCASFailed)` → exit 1; `deps.Out` contains `ce.Error()` (the §13.5
  "HEAD moved from <E> to <A>… git commit-tree -p <E> -m … <tree[i]> | xargs git update-ref HEAD");
  the arbiter does NOT run.
- **FR-M12d (stager retry-then-empty)**: a stager seam that returns an error TWICE for concept i
  (and succeeds for others) → concept i is SKIPPED (no commit, no message launch); the loop continues;
  the final commit count is ≤ N (the non-failing concepts land); `deps.Verbose` (or a captured log)
  records the retry. A stager that fails ONCE then succeeds on retry proceeds normally (concept committed).
- **FR-M8 empty-skip preserved**: S1's `tree[i] == prevTree` skip still works AND is the mechanism a
  twice-failed stager falls into (it staged nothing → tree[i]==prevTree → skipped).
- **Partial commits are real**: on any FR-M12a/b partial failure, the returned `DecomposeResult.Commits`
  are the actually-published commits (HEAD's log contains them in order); they "stand".
- **Signal arming**: after `freezeSnapshot` for a non-skipped concept, the loop calls
  `signal.SetSnapshot(tree[i], newSHA[i-1], "")` and clears it in `publish()` before the CAS — nil-safe
  no-ops in tests without `signal.Install`. The loop NEVER calls `signal.RestoreDefault` (one-shot).
- `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; `make coverage-gate` PASS; go.mod/go.sum UNCHANGED.

## User Persona

**Target User**: the CLI default-action router (P4.M1.T1.S1) and the public `pkg/stagehand` API
(P4.M2.T1.S1), and through them the end user running `stagehand` on an un-staged dirty tree. S2 is
internal error handling — the user-visible artifacts are the §18.3 multi-commit rescue message and the
§13.5 CAS message on stderr, plus the partial commits that landed.

**Use Case**: the user runs `stagehand` on a messy working tree; the planner partitions it into N
concepts; the stager/message pipeline runs. If the message agent times out on concept i (or HEAD moves
during commit i), the user is NOT left with a broken half-run: the commits that DID land (0..i-1) are
final, the remaining staged work is preserved in the index, and an actionable `git commit-tree … |
xargs git update-ref HEAD` recipe is printed for the failed concept.

**User Journey**: (1) user runs `stagehand`; (2) concepts 0..i-1 commit cleanly; (3) concept i's
message gen fails → stderr shows "❌ Commit generation failed for concept i+1 of N: <title>…" + the
recipe + "Concepts already published are final…"; (4) user inspects `git log` (0..i-1 landed) +
`git status` (concept i+1's files staged); (5) user commits concept i manually with the printed recipe.

**Pain Points Addressed**: (a) one misbehaving concept can no longer poison an otherwise-good multi-
commit run; (b) the user never loses staged work or lands an empty/cross-contaminated commit; (c) the
recovery recipe is scoped to the EXACT failed concept (right tree, right parent) — not a generic v1
rescue that assumes a single snapshot.

## Why

- **Business value**: FR-M12 is the multi-commit analogue of v1's §18 rescue guarantee (§13.6.7's
  one-paragraph proof: "a failed, slow, or mis-behaving agent can never corrupt history, never lose
  staged work… the same guarantee v1 makes, extended across a loop"). Without S2, S1's loop aborts the
  WHOLE run on the first concept failure — discarding the context (which concept, which tree, which
  parent) and forcing the user to re-run from scratch. S2 turns a hard abort into an isolated, recoverable
  partial success.
- **Integration with existing features**: consumes S1's `Decompose`/`runLoop`/`publish` closure +
  `generateMessage`/`publishCommit`/`stageConcept`/`freezeSnapshot` (their error contracts are
  *RescueError*/*CASError*/*ErrStagerFailed*); reuses `generate.FormatRescue`'s recipe + `rescueSep`
  (via the new `FormatRescueMulti`); reuses `signal.SetSnapshot`/`ClearSnapshot` (the §18.4 handler,
  already wired in `main.go`). It is the error-isolation layer §13.6.6/§18.2 specify for v2.
- **Problems this solves and for whom**: the plan-holder persona (§7.1) wants reliability: a transient
  agent timeout on one concept must not waste the run. S2 delivers the §13.6.6 per-concept recovery
  contract: prior commits stand, remaining work is preserved, the user gets the exact recipe.

## What

**User-visible behavior**: on a multi-commit run, if concept i's message generation fails, stagehand
prints a concept-scoped rescue to stderr ("concept i+1 of N", its frozen tree, its parent
`newSHA[i-1]`, the `commit-tree | update-ref` recipe, and the "already-published commits stand /
remaining staged work is safe" note) and exits 3 (or 124 on timeout). The commits that landed (0..i-1)
are in `git log`; concept i+1's staged files are in `git status`. If commit i's CAS fails (HEAD moved
externally), stagehand prints the §13.5 "HEAD moved" message (with the tree[i] recovery command) and
exits 1; prior commits stand. If a stager fails twice, that concept is silently skipped (logged) and
the run continues — no empty commit. The arbiter never runs after a mid-loop rescue/abort (§18.3).

**Technical requirements**: S2 EDITS `runLoop`'s `publish()` closure to catch `*generate.RescueError`
→ print `generate.FormatRescueMulti(...)` + return a new `*DecomposeRescueError` (wrapping the
RescueError + concept title + index + count), and `*generate.CASError` → print `ce.Error()` + return
it; `runLoop` returns the PARTIAL commits on such errors (not `nil`); the stager call is wrapped with
retry-once-then-empty (FR-M12d); `Decompose` returns `(DecomposeResult{Commits: partial}, typedErr)`
on partial failure and does NOT run the arbiter. Signal is armed per-concept via `SetSnapshot`/`ClearSnapshot`
(never `RestoreDefault`). `Deps.Out io.Writer` is the rescue destination.

### Success Criteria

- [ ] `FormatRescueMulti(treeSHA, parentSHA, candidateMsg, conceptTitle, index, count)` produces the
      §18.3 multi-commit variant: a concept-naming header ("concept i+1 of N"), the multi-commit
      reassurance line, the SAME recipe lines as `FormatRescue` (`git commit-tree [-p <parent>] -m
      "Your message" <tree> | xargs git update-ref HEAD`, parent omitted when ""), the first-commit
      hint, and the optional candidate note. It reuses `rescueSep` (60 '-') and has no trailing newline.
- [ ] `Decompose` on a message[i] `*RescueError` returns `(DecomposeResult{Commits: 0..i-1},
      *DecomposeRescueError)`; `errors.As(err, &*generate.RescueError{})` TRUE; `errors.Is` to
      `generate.ErrRescue`/`ErrTimeout` TRUE (→ exit 3/124); `deps.Out` contains the FormatRescueMulti
      output; the arbiter does NOT run.
- [ ] `Decompose` on a commit[i] `*generate.CASError` returns `(DecomposeResult{Commits: 0..i-1},
      *generate.CASError)`; `errors.As(err, &*generate.CASError{})` TRUE → exit 1; `deps.Out` contains
      `ce.Error()`; the arbiter does NOT run.
- [ ] `Decompose` with a stager seam that fails TWICE for concept i skips concept i (no commit i,
      no message launch) and continues; final commit count ≤ N. A stager that fails ONCE then succeeds
      commits normally.
- [ ] The overlapped stager[i+1]'s work (already complete when msg[i] fails, since stage[i+1] runs
      synchronously before publish(msg[i])) remains in the index after a FR-M12a partial failure.
- [ ] `runLoop` arms `signal.SetSnapshot(tree[i], newSHA[i-1], "")` after freeze (non-skipped concepts)
      and `signal.ClearSnapshot()` in `publish()` before the CAS; NEVER calls `signal.RestoreDefault`.
- [ ] On the happy path (no failures), S2 changes NOTHING observable vs S1 (N commits, arbiter runs if
      leftovers, exit 0); all S1 tests still pass.
- [ ] `go build ./... && go vet ./... && go test -race ./...` green; `golangci-lint run` clean;
      `gofmt -l internal/ pkg/` empty; `make coverage-gate` PASS; go.mod/go.sum UNCHANGED.

## All Needed Context

### Context Completeness Check

_Before writing this PRP, validate: "If someone knew nothing about this codebase, would they have
everything needed to implement this successfully?"_ — YES. Every consumed symbol (exact signature,
file, error contract), the S1 `runLoop`/`publish` structure (quoted verbatim), the exact FR-M12
diffs to make (stager wrapper, publish-closure rewrite, runLoop partial-return, Decompose arbiter-skip),
the `FormatRescueMulti` byte-level spec, the `DecomposeRescueError` Unwrap chain (for exit-code
mapping), the one-shot `RestoreDefault` gotcha, the "stager[i+1] already complete" insight, the
`Deps.Out` decision, the exit-code semantics from §18.2, and the validation gates are all specified
below with exact paths and line-level patterns. The subtle points — that `RescueError.ParentSHA`
ALREADY equals `newSHA[i-1]` (no re-read needed), that the empty-skip is the twice-failed-stager
mechanism, that the loop prints while the CLI double-print is P4, and that signal uses Set/Clear not
RestoreDefault — are explained, not just named.

### Documentation & References

```yaml
# MUST READ — include these in your context window
- url: PRD.md §13.6.6 (failure handling within the loop)
  why: "The authoritative per-concept failure contract. (a) message[i] fails → rescue for concept i
        ONLY; prior commits stand; frozen tree[i]+recovery printed; remaining staged work stays in
        index; overlapped stager[i+1] allowed to complete. (b) CAS failure on commit[i] → abort with
        §13.5 message; prior commits stand; tree[i] recovery printed. (c) stager stages nothing →
        skip (FR-M8). (d) stager non-zero twice → treat as empty."
  critical: "The arbiter is NOT run when the loop aborts via rescue (§18.3 last ¶). The overlapped
        stager[i+1] 'if already running, is allowed to complete' is AUTOMATICALLY satisfied by S1's
        loop ordering (stage[i+1] runs synchronously BEFORE publish(msg[i]) drains msg[i]) — S2 must
        only avoid resetting the index (it never does)."

- url: PRD.md §18.3 (the rescue message) — LAST PARAGRAPH (multi-commit variant)
  why: "Defines FormatRescueMulti's content: 'print tree[i], its parent (newSHA[i-1]), and the same
        commit-tree|update-ref recipe. Already-published commits 0..i-1 are final and untouched; any
        concepts whose staging completed remain staged.'"
  critical: "Reuse FormatRescue's recipe lines VERBATIM (same `git commit-tree [-p <parent>] -m
        \"Your message\" <tree> | xargs git update-ref HEAD`, parent omitted when parentSHA==\"\"). Add
        a concept-naming header + a multi-commit reassurance line. Do NOT change FormatRescue's
        signature (FROZEN — main.go wires signal.Options.RescueFormat = generate.FormatRescue)."

- url: PRD.md §18.2 (failure modes table) — the v2 rows
  why: "Exit-code semantics: message[i] fails mid-loop → exit 3 (Rescue); CAS failure → exit 1;
        stager stages-nothing/exits-non-zero-twice → skip+continue (exit 0 eventually)."
  critical: "S2's DecomposeRescueError MUST errors.Is to generate.ErrRescue/ErrTimeout so exitcode.For
        maps to 3/124; the raw *CASError errors.Is to git.ErrCASFailed → exit 1. No new exit codes."

- url: PRD.md §18.4 (signal handling) + §9.10 FR45
  why: "Ctrl-C post-snapshot → rescue. S2 arms the snapshot per-concept so a Ctrl-C during message[i]
        generation rescues with tree[i] + newSHA[i-1]."
  critical: "signal.RestoreDefault is ONE-SHOT + PERMANENT (signal.Stop+close). The loop CANNOT call
        it per-concept — use SetSnapshot/ClearSnapshot toggling. RestoreDefault stays single-commit-
        only (generate.CommitStaged's internal §18.4 step 3)."

# CODEBASE FILES — pattern sources + consumed dependencies (all verified, paths exact)
- file: internal/decompose/decompose.go   # S1's file — EDIT TARGET (S2 wraps runLoop + Decompose)
  why: "S1's runLoop (1-deep overlap) + the publish() CLOSURE + Decompose entry point. S2 EDITS these.
        The publish closure is `res := <-ch; if res.err != nil { return res.err }; publishCommit(...)`.
        S2 rewrites it to catch *RescueError → FormatRescueMulti + *DecomposeRescueError, and
        *CASError → ce.Error() + return ce. S2 also wraps the `invokeStager` call with retry-once."
  pattern: "S1's runLoop returns nil,nil,err on any error. S2 changes it to return `commits, nil, err`
        (the PARTIAL commits) so Decompose can surface them. Decompose: on err != nil return
        (DecomposeResult{Commits: commits}, err) and SKIP the arbiter (only run arbiter on err==nil)."
  gotcha: "msgOut{conceptIdx, treeA, treeB, msg, err} is UNEXPORTED + local to runLoop (S1). The
        publish closure captures `concepts`, `commits`, `prevSHA` — so FormatRescueMulti can index
        concepts[res.conceptIdx].Title. Keep publish a closure (do not extract it without passing all
        captures). S2 ADDS `import \"github.com/dustin/stagehand/internal/signal\"` + \"errors\" (if not present)."

- file: internal/decompose/message.go   # CONSUMED (read-only)
  why: "generateMessage returns *generate.RescueError DIRECTLY (gen failure) — its ParentSHA field =
        RevParseHEAD() at call time = newSHA[i-1] (because commit[i-1] was published before msg[i]
        launched). publishCommit returns *generate.CASError DIRECTLY (CAS failure) — ce.Error() IS the
        §13.5 message WITH the tree[i] recovery command."
  pattern: "errors.As(res.err, &re) detects message failure; errors.As(publishErr, &ce) detects CAS."
  gotcha: "generateMessage ALSO returns ErrMessageFailed-wrapped errors for INFRA failures (TreeDiff /
        RevParseHEAD / render / empty-diff). Those are HARD failures (not rescue) — S2 propagates them
        (do NOT catch as rescue). Only *RescueError is a partial-rescue. Likewise publishCommit's
        ErrPublicationFailed-wrapped (CommitTree) errors are HARD — only *CASError is partial-CAS."

- file: internal/decompose/stager.go   # CONSUMED (read-only)
  why: "stageConcept returns ErrStagerFailed-wrapped on ANY failure; nil on success; NO retry (the
        orchestrator owns FR-M8/M12). freezeSnapshot(ctx, deps) (string, error)."
  pattern: "S2 wraps invokeStager: on error, retry once; on second error, log (deps.Verbose) + return
        nil (fall through to freezeSnapshot → tree[i]==prevTree → empty-skip)."
  gotcha: "A stager that stages SOME files then exits non-zero: on retry it may stage more (git add is
        idempotent). The TRUTH is tree[i]==prevTree — if anything was staged, it is NOT skipped. The
        empty-skip is authoritative; S2's retry wrapper only decides retry-vs-skip, the skip itself is
        S1's existing tree comparison."

- file: internal/generate/rescue.go   # EDIT TARGET (add FormatRescueMulti)
  why: "FormatRescue(treeSHA, parentSHA, candidateMsg) is the §18.3 base rescue (FROZEN signature).
        rescueSep = 60 '-' (package-private const). S2 adds FormatRescueMulti in the SAME package
        (reuses rescueSep, mirrors the recipe lines)."
  pattern: "Mirror FormatRescue's Builder structure: header line → rescueSep → reassurance → 'Tree ID:
        <tree>' → blank → recipe header → '  git commit-tree [-p <parent>] -m \"Your message\" <tree>
        | xargs git update-ref HEAD' → blank → first-commit hint → rescueSep → optional candidate note.
        No trailing newline. FormatRescueMulti's header names 'concept <index+1> of <count>' + title;
        its reassurance is the multi-commit line."
  gotcha: "index is 0-based internally; print index+1 (1-based, human-readable 'concept 2 of 3'). When
        conceptTitle=='' omit the ': <title>' (defensive). parentSHA=='' omits ' -p <parent>' (root/
        unborn — mirrors FormatRescue + git.CommitTree). Do NOT change FormatRescue's signature/body."

- file: internal/generate/generate.go   # CONSUMED (read-only)
  why: "RescueError{Kind, TreeSHA, ParentSHA, Candidate, Cause} (Error + Unwrap→Kind; Kind∈{ErrTimeout,
        ErrRescue}). CASError{TreeSHA, Expected, Actual, Message} (Error IS §18.5 message; Unwrap→
        git.ErrCASFailed). DecomposeRescueError holds a *RescueError field + Unwrap()→it so errors.As
        to *RescueError AND errors.Is to ErrRescue/ErrTimeout traverse the chain (exitcode.For)."
  pattern: "type DecomposeRescueError struct { Rescue *generate.RescueError; ConceptTitle string;
        Index, Count int; Commits []CommitResult }. Error() delegates to Rescue.Error(); Unwrap()
        returns Rescue (nil-safe). Do NOT embed (promotion would shadow Error/Unwrap)."

- file: internal/decompose/roles.go   # EDIT TARGET (add Out io.Writer to Deps)
  why: "Deps{Git, Registry, Config, Roles, Verbose *ui.Verbose, stager seam}. S2 ADDS `Out io.Writer`
        — the rescue-message destination. ui.Verbose.w is UNEXPORTED (no public writer) so Deps.Out is
        the clean injection point. Add `\"io\"` to imports."
  pattern: "Field with a doc comment: '// Out is where the loop prints the §18.3 multi-commit rescue +
        the §13.5 CAS message (stderr in prod via cmd.ErrOrStderr; *bytes.Buffer in tests). nil →
        rescue messages are skipped (library-safe).' NO behavior change for nil (the loop guards nil)."
  gotcha: "roles.go was EDITED by S1 (added the unexported `stager` seam). S2 runs AFTER S1 → just add
        the Out field alongside. Production CLI builds Deps with Out = cmd.ErrOrStderr() (P4 wires it;
        S2's tests pass a *bytes.Buffer)."

- file: internal/signal/signal.go   # CONSUMED (read-only) — SetSnapshot, ClearSnapshot only
  why: "signal.SetSnapshot(treeSHA, parentSHA, candidate) arms (mutex-protected; nil-safe no-op without
        Install). signal.ClearSnapshot() disarms. The handler on Ctrl-C calls opts.RescueFormat (=
        generate.FormatRescue, the BASE form) → correct for multi-commit (parent is right)."
  pattern: "In runLoop: after freezeSnapshot for a non-skipped concept, before launch:
        signal.SetSnapshot(treeI, prevSHA, \"\"). In publish(): `res := <-ch; signal.ClearSnapshot();
        ...` (clear before the CAS — the CAS is atomic/instant; a Ctrl-C during it → pre-snapshot →
        exit 130, acceptable)."
  gotcha: "DO NOT call signal.RestoreDefault in the loop — it is ONE-SHOT+PERMANENT (would kill the
        handler after concept 0). SetSnapshot/ClearSnapshot toggle is the loop's mechanism. The message
        goroutine is signal-FREE (it never calls Set/Clear) — only the main loop goroutine touches
        signal, so no concurrency hazard (the handlers' mutex serializes). candidate stays \"\" in the
        signal path (the goroutine can't update it) — acceptable; the EXPLICIT error path (FormatRescueMulti)
        carries the real candidate from the RescueError."

- file: internal/exitcode/exitcode.go   # CONSUMED (read-only) — For(err) mapping
  why: "exitcode.For: errors.Is(err, generate.ErrRescue)→3; errors.Is(err, generate.ErrTimeout)→124;
        errors.Is(err, generate.ErrCASFailed)→1. S2's DecomposeRescueError.Unwrap→*RescueError→Unwrap→Kind
        makes errors.Is traverse correctly. The raw *CASError.Unwrap→git.ErrCASFailed→1."
  pattern: "No exitcode changes. S2 relies on the existing errors.Is traversal. The CLI (P4) calls
        exitcode.For on Decompose's returned error."

- file: internal/decompose/decompose_test.go   # S1's file — EDIT TARGET (S2 ADDS tests)
  why: "S1 created dcm* fixtures (dcmInitRepo/dcmWriteFile/dcmStageFile/dcmCommitRaw/dcmRunGit/dcmDeps)
        + stubtest.Build/Manifest/NewScript. S2 ADDS TestDecompose_MessageRescuePartial,
        TestDecompose_CASAbortPartial, TestDecompose_StagerRetryThenEmpty, TestDecompose_RescueArbiterSkipped
        + a dcmOutBuffer() helper (dcmDeps with Out=*bytes.Buffer)."
  pattern: "Message timeout → *RescueError: stubtest.Manifest with SleepMS > Config.Timeout. CAS failure:
        inject a stager seam (deps.stager) that runs `git commit --allow-empty -m moved` (moves HEAD)
        for concept i+1, so publishCommit[i]'s CAS (expected newSHA[i-1]) fails. Stager-fail-twice: a
        stager seam that returns an error for the first 2 calls on concept i then succeeds."
  gotcha: "dcmDeps must set Out (a *bytes.Buffer) so the test can assert the printed rescue. The
    FR-M12a test's message stub must time out ONLY for concept i (use stubtest.NewScript with a
    SleepMS-varying response, or a counter-based seam). The arbiter stub MUST record its call count
    (assert 0 on partial-failure tests)."

- file: internal/generate/rescue_test.go   # EDIT TARGET (add FormatRescueMulti tests)
  why: "PATTERN for byte-exact rescue assertions (TestFormatRescue_RootedNoCandidate etc.). S2 adds
        TestFormatRescueMulti_* (header with title + position, recipe with/without -p, candidate note,
        the multi-commit reassurance line, no trailing newline)."
  pattern: "Table-driven; assert the FULL string via %q diff (like TestFormatRescue_RootedNoCandidate).
        Add a TestFormatRescueMulti_Properties row to the existing structural-invariant table if convenient."
  gotcha: "generate IS coverage-gated (make coverage-gate ≥85%). FormatRescueMulti MUST be tested or it
        lowers generate's coverage. Cover: titled concept, empty title, root (parentSHA==\"\"), with
        candidate, index/count rendering."
```

### Current Codebase tree (relevant subset — post-S1)

```bash
internal/
  generate/
    generate.go      # CONSUMED — RescueError, CASError, ErrTimeout, ErrRescue
    rescue.go        # EDIT (S2): add FormatRescueMulti (reuses rescueSep)
    rescue_test.go   # EDIT (S2): add TestFormatRescueMulti_*
  decompose/
    roles.go         # EDIT (S2): add Out io.Writer to Deps (S1 added the stager seam)
    planner.go       # CONSUMED (read-only)
    stager.go        # CONSUMED (read-only) — stageConcept, freezeSnapshot, ErrStagerFailed
    message.go       # CONSUMED (read-only) — generateMessage→*RescueError, publishCommit→*CASError
    arbiter.go       # CONSUMED (read-only)
    chain.go         # CONSUMED (read-only)
    decompose.go     # EDIT (S2): S1's file — DecomposeRescueError + FR-M12 isolation in runLoop +
                     #   Decompose + signal arming
    decompose_test.go# EDIT (S2): S1's file — ADD FR-M12 tests + dcmOutBuffer
  signal/signal.go   # CONSUMED (read-only) — SetSnapshot, ClearSnapshot (NOT RestoreDefault)
  exitcode/exitcode.go # CONSUMED (read-only) — For(err) mapping
  git/git.go         # CONSUMED (read-only)
pkg/stagehand/       # UNCHANGED (P4.M2.T1.S1 adds the public Decompose API)
```

### Desired Codebase tree (files S2 EDITS — no new files)

```bash
internal/generate/rescue.go        # +FormatRescueMulti(treeSHA, parentSHA, candidateMsg, conceptTitle, index, count)
internal/generate/rescue_test.go   # +TestFormatRescueMulti_* (header/recipe/candidate/position/root)
internal/decompose/roles.go        # +Out io.Writer field on Deps (+ "io" import)
internal/decompose/decompose.go    # +DecomposeRescueError; runLoop stager-retry + publish-closure FR-M12 catch +
                                   #  partial-commit return + per-concept signal arming; Decompose arbiter-skip on partial
internal/decompose/decompose_test.go # +TestDecompose_MessageRescuePartial / _CASAbortPartial /
                                   #  _StagerRetryThenEmpty / _RescueArbiterSkipped + dcmOutBuffer
```

### Known Gotchas of our codebase & Library Quirks

```go
// G-PARENT-ALREADY-CORRECT (CRITICAL): generateMessage sets RescueError.ParentSHA = RevParseHEAD() at
//   call time. In S1's loop, msg[i] is launched AFTER publish(msg[i-1]) advanced HEAD to newSHA[i-1].
//   So RescueError.ParentSHA == newSHA[i-1] ALREADY — the §18.3 multi-commit variant's "parent
//   (newSHA[i-1])" needs NO re-read. FormatRescueMulti(re.TreeSHA, re.ParentSHA, re.Candidate, ...)
//   gives tree[i] + newSHA[i-1]. Do NOT pass prevSHA from the loop for the parent — use re.ParentSHA
//   (it's the value captured inside generateMessage, consistent with the failure moment).

// G-STAGER-OVERLAP-AUTO (CRITICAL): §13.6.6 "the overlapped stager[i+1], if already running, is allowed
//   to complete" is AUTOMATIC in S1's loop. Order: iter i+1 → stage[i+1] (synchronous, msg[i] in flight)
//   → freeze tree[i+1] → publish(msg[i]) [drains msg[i], sees the error]. So stage[i+1] has ALREADY
//   completed by the time msg[i]'s error is observed; its staging is in the live index (frozen into
//   tree[i+1]). S2 does NOT need a wait/drain for stager[i+1] — only must not reset the index (never does).

// G-EMPTY-SKIP-IS-THE-MECHANISM (FR-M12d): a twice-failed stager stages NOTHING → after freezeSnapshot
//   tree[i]==prevTree → S1's existing empty-skip handles it (skip launch + skip publish for concept i).
//   S2's retry wrapper returns nil on the second failure so the loop FALLS THROUGH to freezeSnapshot +
//   the empty-skip. Do NOT add a separate "skip" code path — reuse S1's tree comparison.

// G-RESCUE-VS-INFRA (CRITICAL): generateMessage returns *RescueError for GEN failures (timeout/parse/
//   dup-exhausted/non-zero-exit/cancel) AND ErrMessageFailed-wrapped for INFRA failures (TreeDiff/
//   RevParseHEAD/render/empty-diff). ONLY *RescueError is a partial-rescue (FR-M12a). ErrMessageFailed-
//   wrapped is a HARD failure — propagate (return nil-nil-err style, partial commits still returned but
//   it's exit 1, no rescue printed). Distinguish via errors.As(res.err, &re). Likewise publishCommit:
//   *CASError = partial-CAS (FR-M12b); ErrPublicationFailed-wrapped = hard.

// G-RESTOREDEFAULT-ONESHOT (CRITICAL): signal.RestoreDefault is ONE-SHOT + PERMANENT (signal.Stop +
//   close(ch); stopped CAS). Calling it per-concept kills the handler after concept 0. The loop uses
//   SetSnapshot/ClearSnapshot toggling ONLY. RestoreDefault stays in generate.CommitStaged (single-commit).

// G-DECOMPOSERESCUE-UNWRAP: DecomposeRescueError must Unwrap() to its *RescueError field (NOT embed it —
//   embedding promotes RescueError.Error/Unwrap and shadows S2's). Then: errors.As(err, &dre) ✓,
//   errors.As(err, &re) traverses Unwrap→Rescue ✓, errors.Is(err, generate.ErrRescue) traverses
//   Unwrap→Rescue→Unwrap→Kind ✓. exitcode.For maps 3/124. The raw *CASError already Unwraps→git.ErrCASFailed→1.

// G-LOOP-PRINTS-CLI-MAPS: S2's loop PRINTS the rescue/CAS message to deps.Out (it owns the concept
//   context: title, index, count). Decompose returns (DecomposeResult{Commits: partial}, typedErr).
//   The CLI (P4.M1.T1.S1) maps the exit code via exitcode.For and must NOT re-print (handleGenError
//   is single-commit-specific; P4 adds a decompose branch that sees the partial result + suppresses
//   reprint). S2 is INTERNAL; the double-print suppression is a P4 concern (flagged, not blocking).

// G-OUT-NILSAFE: deps.Out may be nil (library use without CLI wiring). The loop guards:
//   `if deps.Out != nil { fmt.Fprintln(deps.Out, ...) }`. Production CLI passes cmd.ErrOrStderr();
//   tests pass *bytes.Buffer. A nil Out never panics.

// G-MSGOUT-UNEXPORTED: S1's msgOut{conceptIdx, treeA, treeB, msg, err} is local to runLoop. S2 keeps
//   publish a CLOSURE inside runLoop (it captures concepts, commits, prevSHA, deps) — do not extract
//   it to a package-level func without threading all captures. The FR-M12 catch lives INSIDE the closure.

// G-COVERAGE-GATE: generate IS coverage-gated (make coverage-gate ≥85%). FormatRescueMulti (in
//   generate/rescue.go) MUST have tests or it lowers generate's coverage → gate FAILS. decompose is
//   NOT gated. Write TestFormatRescueMulti_* FIRST (or alongside) and confirm `make coverage-gate` PASS.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/decompose/decompose.go (S2 additions)

// DecomposeRescueError carries the partial result + concept context when message[i] generation fails
// mid-loop (PRD §13.6.6 / §18.3 multi-commit variant / §9.14 FR-M12a). It wraps *generate.RescueError
// (held as a field, NOT embedded — embedding would shadow Error/Unwrap) so errors.As to *RescueError
// and errors.Is to generate.ErrRescue/ErrTimeout traverse the Unwrap chain for exit-code mapping
// (exitcode.For → Rescue=3 / Timeout=124). Commits holds the already-published commits 0..i-1 (they
// stand). The loop prints generate.FormatRescueMulti(...) to deps.Out BEFORE returning this.
type DecomposeRescueError struct {
	Rescue       *generate.RescueError // the concept-i failure: TreeSHA=tree[i], ParentSHA=newSHA[i-1], Candidate, Kind
	ConceptTitle string                // concepts[i].Title — for the rescue header
	Index        int                   // concept index i (0-based)
	Count        int                   // total concept count N
	Commits      []CommitResult        // partial commits 0..i-1 (already published — they stand)
}

func (e *DecomposeRescueError) Error() string {
	if e.Rescue != nil {
		return e.Rescue.Error()
	}
	return "decompose: concept generation failed"
}

// Unwrap returns the underlying *RescueError so errors.As(&re) + errors.Is(ErrRescue/ErrTimeout)
// (→ exitcode 3/124) traverse the chain. nil-safe.
func (e *DecomposeRescueError) Unwrap() error {
	if e.Rescue != nil {
		return e.Rescue
	}
	return nil
}
```

```go
// internal/generate/rescue.go (S2 addition)

// FormatRescueMulti implements PRD §18.3's multi-commit variant (last ¶): when a single concept fails
// mid-loop, the rescue is scoped to that concept's frozen tree[i]. It prints tree[i], its parent
// (newSHA[i-1]), and the same commit-tree|update-ref recipe as FormatRescue — plus a concept-naming
// header ("concept i+1 of N: <title>") and a multi-commit reassurance line. index is 0-based (printed
// 1-based); count is N. conceptTitle=="" omits the title. parentSHA=="" omits " -p <parent>" (root).
// candidateMsg!="" appends the §18.3 candidate note. No trailing newline (the caller's Fprintln adds it).
// Reuses rescueSep (60 '-') + FormatRescue's recipe lines verbatim.
func FormatRescueMulti(treeSHA, parentSHA, candidateMsg, conceptTitle string, index, count int) string
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/generate/rescue.go — add FormatRescueMulti
  - ADD FormatRescueMulti(treeSHA, parentSHA, candidateMsg, conceptTitle string, index, count int) string.
  - STRUCTURE (mirror FormatRescue; reuse rescueSep):
      Line 1 (header): "❌ Commit generation failed for concept <index+1> of <count>" + (conceptTitle!="" ?
        ": <conceptTitle>" : "") + ".\n"
      rescueSep + "\n"
      reassurance: "Concepts already published are final and untouched. Remaining staged changes are
        safe in your index.\n"
      "Tree ID: <treeSHA>\n\n"
      "To commit this concept's staged files manually:\n"
      "  git commit-tree" + (parentSHA!="" ? " -p <parentSHA>" : "") + ` -m "Your message" ` + treeSHA
        + " | xargs git update-ref HEAD\n\n"
      `(omit "-p <PARENT_SHA>" if this is the repository's first commit)` + "\n"
      rescueSep
      (candidateMsg!="" ? "\n\nA candidate message was produced but rejected: \"<candidateMsg>\". You
        can use it manually in the command above." : "")
  - FOLLOW pattern: internal/generate/rescue.go FormatRescue (the Builder + no-trailing-newline style +
    the exact recipe lines + rescueSep). strconv.Itoa for index+1/count.
  - NAMING: FormatRescueMulti (exported; sibling of FormatRescue).
  - PLACEMENT: internal/generate/rescue.go (same package — reuses rescueSep).
  - PRESERVE: FormatRescue UNCHANGED (FROZEN signature + body).

Task 2: EDIT internal/generate/rescue_test.go — add FormatRescueMulti tests (coverage gate)
  - ADD TestFormatRescueMulti_TitledRooted (concept 2 of 3, titled, rooted → header + recipe with -p +
    reassurance; byte-exact %q diff like TestFormatRescue_RootedNoCandidate).
  - ADD TestFormatRescueMulti_RootlessNoTitle (parentSHA=="", conceptTitle=="" → no -p, no title).
  - ADD TestFormatRescueMulti_WithCandidate (candidate note appended after closing sep).
  - ADD a row or two to TestFormatRescue_Properties (structural invariants: contains "concept",
    contains the tree, contains "update-ref HEAD", no trailing newline) if convenient.
  - FOLLOW pattern: internal/generate/rescue_test.go (table-driven %q assertions).
  - COVERAGE: header (titled + untitled), recipe (rooted -p + rootless no -p), candidate present/absent,
    position rendering (index+1/count). MUST keep `make coverage-gate` green.

Task 3: EDIT internal/decompose/roles.go — add Out io.Writer to Deps
  - ADD to Deps (after Verbose): 
      // Out is where the loop prints the §18.3 multi-commit rescue + the §13.5 CAS message (stderr in
      // prod via cmd.ErrOrStderr; *bytes.Buffer in tests). nil → rescue/CAS messages are skipped
      // (library-safe; the loop guards nil). S2 (P3.M4.T1.S2).
      Out io.Writer
  - ADD "io" to the import block (roles.go imports context/config/git/prompt/provider/ui/fmt; add "io").
  - FOLLOW pattern: Deps is the injectable-collaborators struct (S1 added the stager seam the same way).
  - PRESERVE: every other field + ResolveRoles + helpers UNCHANGED. (S1's stager seam stays.)

Task 4: EDIT internal/decompose/decompose.go — DecomposeRescueError + FR-M12 isolation + signal arming
  - ADD: DecomposeRescueError type (+ Error + Unwrap) — see Data models.
  - ADD import "github.com/dustin/stagehand/internal/signal" (signal imports no stagehand pkgs → no cycle)
    + ensure "errors" is imported.
  - MODIFY runLoop (S1's version): wrap the invokeStager call with retry-once-then-empty; rewrite the
    publish() closure to catch *RescueError/*CASError; return PARTIAL commits on those errors; arm/
    disarm signal per-concept. See "runLoop changes" below.
  - MODIFY Decompose: on runLoop error, return (DecomposeResult{Commits: commits}, err) (partial) and
    SKIP the arbiter (only run the arbiter on err==nil). See "Decompose changes" below.
  - FOLLOW pattern: internal/decompose/message.go (errors.As to typed errors; direct propagation) +
    S1's runLoop closure style.
  - NAMING: DecomposeRescueError (exported — eventual public-API surface); the retry helper can be an
    inline closure or invokeStagerWithRetry (unexported).
  - PLACEMENT: internal/decompose/decompose.go.

Task 5: EDIT internal/decompose/decompose_test.go — FR-M12 integration tests
  - ADD dcmOutBuffer() (a dcmDeps variant with Out=*bytes.Buffer) — or extend dcmDeps to take an Out.
  - ADD TestDecompose_MessageRescuePartial: 3 concepts; message stub TIMES OUT for concept 1 (index 1,
    e.g. NewScript or SleepMS-varying); stager seam stages real files per concept. Assert: returns
    (DecomposeResult{Commits:[commit0]}, *DecomposeRescueError); errors.As to *RescueError TRUE;
    errors.Is(generate.ErrTimeout) TRUE; deps.Out contains "concept 2 of 3" + the recipe with
    -p <newSHA[0]>; git log shows 1 commit (commit0); git status non-empty (concept 2 staged); arbiter
    stub Execute count == 0.
  - ADD TestDecompose_CASAbortPartial: stager seam for concept i+1 runs `git commit --allow-empty -m x`
    (moves HEAD) so publishCommit[i]'s CAS fails. Assert: returns (*generate.CASError); deps.Out contains
    "HEAD moved"; partial commits 0..i-1 in git log; arbiter count == 0.
  - ADD TestDecompose_StagerRetryThenEmpty: stager seam returns error twice for concept i (succeeds for
    others). Assert: concept i skipped (≤N commits, no empty commit); loop continued; deps.Verbose (or
    a captured log) shows the retry. Variant: fails once then succeeds → concept committed normally.
  - ADD TestDecompose_RescueArbiterSkipped: asserts the arbiter does NOT run after a FR-M12a rescue
    (covered by TestDecompose_MessageRescuePartial's arbiter-count==0; can be a focused sub-assertion).
  - FOLLOW pattern: S1's dcm* fixtures + stubtest.NewScript (call-varying message responses) +
    message_test.go (script stubs) + the stager seam (deps.stager for real git staging / HEAD moves).
  - COVERAGE: FR-M12a (message rescue), FR-M12b (CAS abort), FR-M12d (stager retry-then-empty +
    retry-then-success), arbiter-skipped, the Out-buffer assertion. All via the real temp git repo.
  - NAMING: dcm* helpers + TestDecompose_* (additive to S1's set — no rename).
  - PLACEMENT: internal/decompose/decompose_test.go.
```

### runLoop changes (Task 4 detail) — the FR-M12 core

```go
// runLoop (S2 version). Changes vs S1: (1) stager retry-once-then-empty; (2) publish() catches
// *RescueError → FormatRescueMulti + *DecomposeRescueError, *CASError → ce.Error(); (3) returns PARTIAL
// commits on those errors; (4) per-concept signal Set/Clear. Skeleton (consumes S1's launch/buildCommitResult):

func runLoop(ctx context.Context, deps Deps, concepts []prompt.PlannerCommit, baseTree, preRunHEAD string, isUnborn bool) ([]CommitResult, []ChainEntry, error) {
	type msgOut struct { conceptIdx int; treeA, treeB, msg string; err error }
	var commits []CommitResult
	var chainData []ChainEntry
	prevTree := baseTree
	prevSHA := preRunHEAD

	launch := func(i int, treeA, treeB string) chan msgOut { /* S1 unchanged: buffered(1), goroutine calls generateMessage */ }

	// isPartial reports whether err is a FR-M12 partial failure (rescue/CAS) — used to decide
	// partial-commits-vs-nil on return.
	isPartial := func(err error) bool {
		var dre *DecomposeRescueError
		var ce *generate.CASError
		return errors.As(err, &dre) || errors.As(err, &ce)
	}

	publish := func(ch chan msgOut) error {
		if ch == nil {
			return nil
		}
		res := <-ch
		signal.ClearSnapshot() // disarm before the CAS (§18.4 analog; RestoreDefault is one-shot — see G-RESTOREDEFAULT)
		if res.err != nil {
			var re *generate.RescueError
			if errors.As(res.err, &re) {
				// FR-M12a: message[i] failed → rescue for concept i ONLY. re.ParentSHA == newSHA[i-1]
				// (generateMessage captured RevParseHEAD after commit[i-1] landed). Print the §18.3
				// multi-commit variant (names the concept + position). The overlapped stager[i+1] has
				// already completed (synchronous-before-drain) → its staging stays in the index.
				title := ""
				if res.conceptIdx < len(concepts) {
					title = concepts[res.conceptIdx].Title
				}
				if deps.Out != nil {
					fmt.Fprintln(deps.Out, generate.FormatRescueMulti(re.TreeSHA, re.ParentSHA, re.Candidate, title, res.conceptIdx, len(concepts)))
				}
				return &DecomposeRescueError{Rescue: re, ConceptTitle: title, Index: res.conceptIdx, Count: len(concepts), Commits: commits}
			}
			return res.err // HARD (ErrMessageFailed-wrapped infra) — propagate
		}
		newSHA, err := publishCommit(ctx, deps, res.treeB, prevSHA, res.msg)
		if err != nil {
			var ce *generate.CASError
			if errors.As(err, &ce) {
				// FR-M12b: CAS failed → §13.5 message (ce.Error() has tree[i] recovery). Prior commits stand.
				if deps.Out != nil {
					fmt.Fprintln(deps.Out, ce.Error())
				}
				return ce // partial; DecomposeResult.Commits = commits (0..i-1)
			}
			return err // HARD (ErrPublicationFailed-wrapped CommitTree)
		}
		cr, err := buildCommitResult(ctx, deps, newSHA, res.msg, res.conceptIdx == 0 && isUnborn)
		if err != nil {
			return fmt.Errorf("%w: diff-tree[%d]: %w", ErrDecomposeFailed, res.conceptIdx, err)
		}
		commits = append(commits, cr)
		chainData = append(chainData, ChainEntry{SHA: newSHA, Tree: res.treeB, Message: res.msg, Parent: prevSHA})
		prevSHA = newSHA
		return nil
	}

	// invokeStagerRetry: FR-M12d — stager exits non-zero → retry once; on second failure treat as empty
	// (return nil → fall through to freezeSnapshot → tree[i]==prevTree → S1's empty-skip). Logs via Verbose.
	// A cancelled ctx propagates (abort the loop) so the run doesn't silently skip every remaining concept.
	invokeStagerRetry := func(concept prompt.PlannerCommit) error {
		if cerr := ctx.Err(); cerr != nil {
			return cerr // ctx cancelled/moved on → abort (drainMsg + return partial), not skip-everything
		}
		err := invokeStager(ctx, deps, concept)
		if err == nil {
			return nil
		}
		deps.Verbose.VerboseRetry(1, fmt.Sprintf("stager failed for %q; retrying once", concept.Title))
		if err2 := invokeStager(ctx, deps, concept); err2 == nil {
			return nil
		}
		deps.Verbose.VerboseRetry(2, fmt.Sprintf("stager failed twice for %q; treating concept as empty (FR-M8)", concept.Title))
		return nil // empty: freezeSnapshot will yield tree[i]==prevTree → S1's empty-skip
	}

	var inflight chan msgOut
	for i, concept := range concepts {
		// FR-M12d: stager retry-once-then-empty (replaces S1's single invokeStager + propagate).
		if err := invokeStagerRetry(concept); err != nil {
			drainMsg(inflight) // ctx cancellation (only non-nil return) — abort; partial commits stand
			return commits, nil, err
		}
		treeI, err := freezeSnapshot(ctx, deps)
		if err != nil {
			drainMsg(inflight)
			return commits, nil, fmt.Errorf("%w: freeze snapshot[%d]: %w", ErrDecomposeFailed, i, err)
		}
		skipped := treeI == prevTree // FR-M8 (a): empty-skip (also the twice-failed-stager path)
		if err := publish(inflight); err != nil {
			// FR-M12: partial failures return the PARTIAL commits (0..i-1); hard failures too (they landed).
			return commits, nil, err
		}
		inflight = nil
		if !skipped {
			signal.SetSnapshot(treeI, prevSHA, "") // arm rescue during msg[i] (§18.4; nil-safe without Install)
			inflight = launch(i, prevTree, treeI)
			prevTree = treeI
		}
	}
	if err := publish(inflight); err != nil {
		return commits, nil, err
	}
	return commits, chainData, nil
}
```

> **NOTE on drainMsg / the in-flight channel**: keep S1's `drainMsg(inflight)` (or inline `<-inflight`)
> at the two early-return sites (stager hard-error, freeze error) to avoid a goroutine leak. The
> buffered(1) channel guarantees the receive completes. On the FR-M12a rescue path, `publish` already
> drained `inflight` (the failing msg[i]) BEFORE returning — so no extra drain is needed there; but the
> NEXT concept's stager (stage[i+1]) had already run, so there is no in-flight goroutine to leak. If you
> restructure, ensure every error path that follows a `launch` drains its channel.

### Decompose changes (Task 4 detail)

```go
// Decompose (S2): the only change is the runLoop error branch. S1 did:
//   commits, chainData, err := runLoop(...); if err != nil { return DecomposeResult{}, err }
// S2 returns the PARTIAL commits AND skips the arbiter on any loop error (§18.3: "the arbiter is not
// run when the loop aborts via rescue"). Replace the error branch with:

	commits, chainData, err := runLoop(ctx, deps, out.Commits, baseTree, preRunHEAD, isUnborn)
	if err != nil {
		// FR-M12: partial failures (rescue/CAS) AND hard failures return the partial commits that
		// already landed (0..i-1). The arbiter does NOT run on a loop abort (§18.3).
		return DecomposeResult{Commits: commits}, err
	}

	// Happy path ONLY: arbiter gate (StatusPorcelain != "" → runArbiterPhase). Unchanged from S1.
	amended := 0
	status, err := deps.Git.StatusPorcelain(ctx)
	// ... (S1's arbiter phase, unchanged) ...
	return DecomposeResult{Commits: commits, Amended: amended}, nil
```

### Implementation Patterns & Key Details

```go
// FormatRescueMulti — reuses rescueSep (same package); mirrors FormatRescue's recipe lines VERBATIM.
func FormatRescueMulti(treeSHA, parentSHA, candidateMsg, conceptTitle string, index, count int) string {
	var b strings.Builder
	b.WriteString("❌ Commit generation failed for concept ")
	b.WriteString(strconv.Itoa(index + 1)) // 1-based for humans
	b.WriteString(" of ")
	b.WriteString(strconv.Itoa(count))
	if conceptTitle != "" {
		b.WriteString(": ")
		b.WriteString(conceptTitle)
	}
	b.WriteString(".\n")
	b.WriteString(rescueSep)
	b.WriteByte('\n')
	b.WriteString("Concepts already published are final and untouched. Remaining staged changes are safe in your index.\n")
	b.WriteString("Tree ID: ")
	b.WriteString(treeSHA)
	b.WriteString("\n\nTo commit this concept's staged files manually:\n  git commit-tree")
	if parentSHA != "" {
		b.WriteString(" -p ")
		b.WriteString(parentSHA)
	}
	b.WriteString(` -m "Your message" `)
	b.WriteString(treeSHA)
	b.WriteString(" | xargs git update-ref HEAD\n\n")
	b.WriteString(`(omit "-p <PARENT_SHA>" if this is the repository's first commit)`)
	b.WriteByte('\n')
	b.WriteString(rescueSep)
	if candidateMsg != "" {
		b.WriteString("\n\nA candidate message was produced but rejected: \"")
		b.WriteString(candidateMsg)
		b.WriteString("\". You can use it manually in the command above.")
	}
	return b.String()
}

// DecomposeRescueError — field (not embed) + delegate Error/Unwrap (see Data models + G-DECOMPOSERESCUE-UNWRAP).
// exitcode.For traverses: DecomposeRescueError.Unwrap → *RescueError → RescueError.Unwrap → Kind (ErrRescue/ErrTimeout).

// CAS path: return the RAW *generate.CASError (not wrapped). exitcode.For: errors.Is(err, git.ErrCASFailed) → 1.
// ce.Error() IS the §13.5 message (with tree[i] recovery). The loop prints it via fmt.Fprintln(deps.Out, ce.Error()).
```

### Integration Points

```yaml
GENERATE (internal/generate/rescue.go): EDIT — add FormatRescueMulti (reuses rescueSep). generate.go UNCHANGED.
DECOMPOSE (internal/decompose):
  - roles.go: EDIT — add Out io.Writer to Deps (+ "io" import).
  - decompose.go: EDIT — DecomposeRescueError type; runLoop stager-retry + publish-closure FR-M12 catch +
    partial-commit return + per-concept signal Set/Clear; Decompose arbiter-skip on partial. Add internal/signal import.
  - decompose_test.go: EDIT — ADD FR-M12 tests + dcmOutBuffer.
SIGNAL (internal/signal): CONSUMED — SetSnapshot/ClearSnapshot ONLY. NEVER RestoreDefault in the loop.
EXITCODE (internal/exitcode): CONSUMED — For(err) maps 3/124/1 via errors.Is traversal. No changes.
GIT (internal/git): CONSUMED read-only. CONFIG/PROMPT: CONSUMED read-only.
CALLER WIRING (P4 — NOT this task): the CLI decompose handler (P4.M1.T1.S1) wires Deps.Out =
  cmd.ErrOrStderr(), maps the exit code via exitcode.For, prints the partial commits' FR42 success lines,
  and SUPPRESSES re-printing the rescue (the loop already printed it). The public API (P4.M2.T1.S1)
  surfaces DecomposeResult.Commits + the typed error. No caller wiring in S2.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after each file edit — fix before proceeding.
go build ./...                       # compile (DecomposeRescueError + FormatRescueMulti + Deps.Out + signal import)
go vet ./...                         # catches shadowed vars, printf issues, unkeyed struct lits
gofmt -l internal/ pkg/              # MUST print nothing
golangci-lint run                    # the repo's linter (Makefile `make lint`)

# Scope-specific quick check:
go build ./internal/decompose/... ./internal/generate/... && go vet ./internal/decompose/... ./internal/generate/...

# Expected: zero errors. Verify the new internal/signal import in decompose.go compiles (no import cycle —
# signal imports no stagehand packages). Verify roles.go's new "io" import is USED (Deps.Out io.Writer).
```

### Level 2: Unit & Integration Tests (Component Validation)

```bash
# FormatRescueMulti (coverage-gated generate package):
go test -race ./internal/generate/... -run 'TestFormatRescueMulti' -v

# The FR-M12 decompose tests:
go test -race ./internal/decompose/... -run 'TestDecompose_MessageRescuePartial|TestDecompose_CASAbortPartial|TestDecompose_StagerRetryThenEmpty|TestDecompose_RescueArbiterSkipped' -v

# Full packages (regression — S1's tests + siblings unchanged):
go test -race ./internal/decompose/... ./internal/generate/...

# The whole suite (Makefile `make test`):
go test -race ./...

# Expected: all pass. If TestDecompose_MessageRescuePartial fails with "arbiter count != 0", the loop
# ran the arbiter after the rescue — verify Decompose's error branch returns BEFORE the StatusPorcelain
# gate (§18.3: arbiter not run on loop abort). If the rescue message is missing from deps.Out, verify
# the message stub actually times out for concept i (SleepMS > Config.Timeout) AND deps.Out is non-nil
# (dcmOutBuffer). If TestDecompose_CASAbortPartial never triggers CAS, the stager seam's HEAD move
# must happen for concept i+1 (before publishCommit[i]'s CAS) — verify the seam runs `git commit` in
# the repo. If TestDecompose_StagerRetryThenEmpty creates an empty commit, the retry wrapper returned
# non-nil (propagating) instead of nil (empty-skip) on the second failure.
```

### Level 3: Integration Testing (System Validation)

```bash
# Coverage gate — generate IS gated (FormatRescueMulti must be covered); decompose is NOT gated.
make coverage-gate      # enforces >=85% on internal/{git,provider,generate,config}

# Expected: PASS. If it FAILS on generate, FormatRescueMulti lacks test coverage — add/extend
# TestFormatRescueMulti_* (Task 2) until the gate is green. Run `make coverage` to see per-function.

# Manual sanity (optional): build a 3-concept repo, inject a stager seam that stages each concept's
# files + a message stub that times out on concept 1, run Decompose with Deps.Out=os.Stderr, then:
#   git log --oneline   → 1 commit (concept 0)
#   git status          → concept 2's files staged (concept 1's tree frozen, rescue printed)
#   stderr              → "❌ Commit generation failed for concept 2 of 3: <title>…" + recipe with -p <newSHA[0]>
```

### Level 4: Creative & Domain-Specific Validation

```bash
# FR-M12a invariant (TestDecompose_MessageRescuePartial): assert (a) errors.As(err, &dre) TRUE;
# (b) errors.As(err, &*generate.RescueError{}) TRUE (traverses Unwrap); (c) errors.Is(err,
# generate.ErrTimeout) TRUE (→ exitcode.For == 124) or errors.Is(err, generate.ErrRescue) (→ 3);
# (d) dre.Commits == [commit0] and git log --count == 1; (e) the rescue in deps.Out names
# "concept 2 of 3" + contains the tree + "-p "+newSHA[0]; (f) arbiter stub Execute count == 0.

# FR-M12b invariant (TestDecompose_CASAbortPartial): assert (a) errors.As(err, &*generate.CASError{}) TRUE;
# (b) errors.Is(err, git.ErrCASFailed) TRUE (→ exitcode.For == 1); (c) deps.Out contains "HEAD moved";
# (d) partial commits 0..i-1 in git log; (e) arbiter count == 0.

# FR-M12d invariant (TestDecompose_StagerRetryThenEmpty): assert (a) concept i created NO commit
# (final count ≤ N); (b) the stager seam was called exactly TWICE for concept i (retry);
# (c) deps.Verbose (captured) logged the retry; (d) the loop CONTINUED (concepts after i committed).

# Exit-code mapping invariant: build the typed errors and assert exitcode.For:
#   exitcode.For(&DecomposeRescueError{Rescue:&generate.RescueError{Kind:generate.ErrRescue}}) == 3
#   exitcode.For(&DecomposeRescueError{Rescue:&generate.RescueError{Kind:generate.ErrTimeout}}) == 124
#   exitcode.For(&generate.CASError{...}) == 1
# (This is the proof the Unwrap chain works end-to-end without touching the CLI.)

# Signal arming (best-effort / optional): a test that calls signal.Install (with a test Exit recorder)
# + a slow message stub + sends os.Interrupt during msg[i] → asserts the handler printed generate.
# FormatRescue (base form, with tree[i]+newSHA[i-1]) to its Out. This is complex; if time-boxed, the
# nil-safe SetSnapshot/ClearSnapshot calls are sufficient (they're no-ops without Install) — verify via
# go test -race that the loop's signal calls never panic and don't leak/race.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` compiles (DecomposeRescueError + FormatRescueMulti + Deps.Out + signal import; no import cycle).
- [ ] `go vet ./...` clean.
- [ ] `go test -race ./...` green (Makefile `make test`) — no goroutine leaks/races in runLoop's FR-M12 paths.
- [ ] `golangci-lint run` clean (Makefile `make lint`).
- [ ] `gofmt -l internal/ pkg/` empty.
- [ ] `make coverage-gate` PASS (generate covered by TestFormatRescueMulti_*; decompose not gated).
- [ ] go.mod/go.sum UNCHANGED (generate/decompose/signal/exitcode/git/config/prompt/provider/ui all already imported).

### Feature Validation

- [ ] FormatRescueMulti produces the §18.3 multi-commit variant (concept header + reassurance + same recipe + candidate note; no trailing newline).
- [ ] message[i] `*RescueError` → Decompose returns `(DecomposeResult{Commits: 0..i-1}, *DecomposeRescueError)`; errors.As to *RescueError + errors.Is to ErrRescue/ErrTimeout (→ 3/124); deps.Out has FormatRescueMulti(tree[i], newSHA[i-1], candidate, title, i, N); arbiter NOT run.
- [ ] commit[i] `*CASError` → Decompose returns `(DecomposeResult{Commits: 0..i-1}, *generate.CASError)` (→ exit 1); deps.Out has ce.Error() (§13.5); arbiter NOT run.
- [ ] stager fails TWICE → concept skipped (≤N commits, no empty commit); stager fails once then succeeds → committed.
- [ ] overlapped stager[i+1]'s work remains staged in the index after a FR-M12a partial (it ran synchronously before publish(msg[i])).
- [ ] loop arms signal.SetSnapshot(tree[i], newSHA[i-1], "") after freeze + signal.ClearSnapshot() in publish() before CAS; NEVER RestoreDefault.
- [ ] happy path (no failures) UNCHANGED vs S1 (N commits, arbiter runs if leftovers, exit 0); all S1 tests pass.

### Code Quality Validation

- [ ] runLoop's publish() closure catches *RescueError (partial) vs ErrMessageFailed-wrapped (hard), and *CASError (partial) vs ErrPublicationFailed-wrapped (hard), via errors.As.
- [ ] DecomposeRescueError uses a FIELD (not embed) + delegates Error/Unwrap (no promotion shadowing); the Unwrap chain yields exit 3/124 via exitcode.For.
- [ ] FormatRescueMulti reuses rescueSep + mirrors FormatRescue's recipe verbatim; FormatRescue UNCHANGED.
- [ ] deps.Out is nil-safe (loop guards `if deps.Out != nil`); production wires stderr (P4).
- [ ] signal.RestoreDefault is NOT called in the loop (one-shot); only SetSnapshot/ClearSnapshot.
- [ ] planner.go/stager.go/message.go/arbiter.go/chain.go/git.go/generate.go UNCHANGED (only rescue.go + roles.go + decompose.go + the two _test.go are edited).
- [ ] msgOut stays unexported/local to runLoop; publish stays a closure capturing concepts/commits/prevSHA/deps.

### Documentation & Deployment

- [ ] Doc comments on FormatRescueMulti, DecomposeRescueError (Error/Unwrap), the runLoop FR-M12 branches, the stager-retry helper, and the signal Set/Clear sites (the package's doc-first style).
- [ ] The §13.6.6/§18.3 contract is cited in the rescue/CAS branches; the §18.4 one-shot RestoreDefault constraint is cited at the signal sites.
- [ ] No new environment variables or config keys.
- [ ] The S2/P4 boundary is documented in code comments (loop prints; CLI double-print suppression + Deps.Out wiring = P4).

---

## Anti-Patterns to Avoid

- ❌ Don't catch `ErrMessageFailed`-wrapped (infra) errors as a rescue — ONLY `*generate.RescueError` is a partial-rescue (FR-M12a). Infra failures are HARD (exit 1, propagate). Same for `ErrPublicationFailed` vs `*CASError` (§G-RESCUE-VS-INFRA).
- ❌ Don't re-read HEAD or pass `prevSHA` for the rescue parent — `RescueError.ParentSHA` ALREADY == newSHA[i-1] (generateMessage captured it post-commit[i-1]). Use `re.ParentSHA` in FormatRescueMulti (§G-PARENT-ALREADY-CORRECT).
- ❌ Don't add a separate "skip" code path for the twice-failed stager — return nil from the retry wrapper and let S1's existing `tree[i]==prevTree` empty-skip handle it (§G-EMPTY-SKIP-IS-THE-MECHANISM).
- ❌ Don't call `signal.RestoreDefault` in the loop — it's ONE-SHOT+PERMANENT and would kill the handler after concept 0. Use SetSnapshot/ClearSnapshot toggling (§G-RESTOREDEFAULT-ONESHOT).
- ❌ Don't embed `*generate.RescueError` in DecomposeRescueError (promotion shadows Error/Unwrap). Hold it as a field + delegate (§G-DECOMPOSERESCUE-UNWRAP).
- ❌ Don't run the arbiter after a loop abort (rescue/CAS/hard) — §18.3: "the arbiter is not run when the loop aborts via rescue." Decompose returns on the runLoop error BEFORE the StatusPorcelain gate.
- ❌ Don't return `nil` commits on a partial failure — return the PARTIAL commits 0..i-1 (they're real, landed, "stand"). S1 returned nil,nil,err; S2 returns commits,nil,err (§runLoop changes).
- ❌ Don't change `FormatRescue`'s signature/body — it's FROZEN (main.go wires signal.Options.RescueFormat = generate.FormatRescue). Add FormatRescueMulti as a sibling.
- ❌ Don't print to os.Stderr directly from the loop — print to `deps.Out` (nil-safe; injectable for tests; production wires stderr in P4). ui.Verbose.w is unexported (§G-OUT-NILSAFE).
- ❌ Don't forget the FormatRescueMulti tests — generate IS coverage-gated; an untested FormatRescueMulti FAILS `make coverage-gate` (§G-COVERAGE-GATE).
- ❌ Don't extract runLoop's `publish` closure to a package-level func without threading all its captures (concepts, commits, prevSHA, deps) — keep it a closure (§G-MSGOUT-UNEXPORTED).
- ❌ Don't lose the in-flight message goroutine on the hard-error early returns — `drainMsg(inflight)` (or `<-inflight`) at the stager-hard-error and freeze-error sites (buffered(1) guarantees completion).
- ❌ Don't wire CLI behavior (exit-code printing, Deps.Out=stderr, double-print suppression) in S2 — that's P4 (P4.M1.T1.S1). S2 is INTERNAL error handling: the loop prints + returns the typed error.
