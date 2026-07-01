# PRP — P1.M3.T2.S1: Add `rereadFinalCommits` and call it after a successful arbiter in `Decompose`

> **Scope discipline.** This subtask **consumes** the `git.Git.LogRange` primitive shipped by
> P1.M3.T1.S1 (now Complete) to close Issue 3's "post-arbiter gap": after the arbiter
> amends/rebuilds/creates, `Decompose` re-reads git for the final commits so `DecomposeResult.Commits`
> carries **accurate, resolvable SHAs/subjects/file-lists** (and includes the null-path (N+1)-th
> commit). It touches ONLY `internal/decompose/decompose.go` (new helper + one call site + two doc
> comments) and `internal/decompose/decompose_test.go` (update one existing assertion + add tip/mid/
> happy-path tests). It does NOT touch `chain.go` (resolution logic), `default_action.go` (printer),
> `git.go`, or the arbiter itself. The success-report format is UNCHANGED — only data accuracy improves.

> ### ⚠️ CRITICAL gotchas that WILL break a one-pass impl if missed
> 1. **ADD `"strings"` to decompose.go's imports** — it is NOT currently imported (needed for
>    `strings.Repeat("0", 40)`). See Task 1.
> 2. **UPDATE the existing `TestDecompose_ArbiterWiring` test** — its `len(result.Commits) != 1`
>    assertion MUST become `!= 2` (the null-path commit is now re-read & included). See Task 6. Two
>    OTHER full-Decompose arbiter tests leave a clean tree (arbiter does not run) and are UNAFFECTED.
> 3. **baseSHA for LogRange** is `strings.Repeat("0", 40)` when `isUnborn`, else `preRunHEAD`.
>    LogRange's all-zeros sentinel already correctly branches to the no-range `HEAD` form (T1.S1).
> 4. **Best-effort on reread error**: log via `deps.Verbose` (nil-safe), KEEP the loop's pre-arbiter
>    `commits`, do NOT fail. The commits are already published; stale SHAs beat erroring.

---

## Goal

**Feature Goal**: After the arbiter phase succeeds in `Decompose`, re-read git for the final commits
in the range `preRunHEAD..HEAD` and replace `DecomposeResult.Commits` with accurate post-arbiter
entries (resolvable SHAs, correct subjects, correct file-lists), closing the §G-RESULT
"post-arbiter gap." Tip/mid-chain amends produce new SHAs (no longer dangling); the null-path
(N+1)-th commit is included.

**Deliverable**:
1. A `rereadFinalCommits(ctx, deps, preRunHEAD, isUnborn) ([]CommitResult, error)` helper in
   `internal/decompose/decompose.go` that pairs `LogRange` + `DiffTree` to rebuild `[]CommitResult`.
2. One call site in `Decompose`'s arbiter block (after `runArbiterPhase` returns nil), best-effort
   (reread error logged via `deps.Verbose`, falls back to the loop's commits).
3. Updated `CommitResult` + `DecomposeResult` (`§G-RESULT`) doc comments (Mode A — gap closed).
4. Updated `TestDecompose_ArbiterWiring` (1→2) + new tip-amend / mid-chain / happy-path-unchanged
   tests + a shell-script-arbiter test helper + a `shaResolves` helper.

**Success Definition**:
- On a null-path decompose (arbiter creates a (N+1)-th commit): `len(res.Commits) == N+1`, and EVERY
  `res.Commits[i].SHA` resolves via `git rev-parse --verify <sha>^{commit}` (exit 0, not dangling),
  and `res.Commits[last].SHA == git rev-parse HEAD`.
- On a tip-amend decompose: `len(res.Commits) == N`, `res.Commits[last].SHA` resolves and equals the
  repo's real tip; the pre-amend SHA is NOT present.
- On a mid-chain decompose: ALL `res.Commits[*].SHA` resolve and match `git log --reverse --format=%H`.
- On a happy-path decompose (arbiter does NOT run): `res.Commits` is UNCHANGED from the loop's entries.
- `go build ./...`, `go vet ./...`, `gofmt -l`, and `go test ./...` all pass.

---

## Why

- **PRD §13.6.5 / FR-M9–M10** (arbiter reconciliation) + **§15.4 / FR42** (success report) + Appendix
  B.1: after the arbiter amends/rebuilds/creates, the SHAs the loop recorded become dangling and an
  arbiter-created commit is omitted entirely (`architecture/issue3_post_arbiter_output.md`). The user
  is shown SHAs that no longer resolve and is missing commits — the most common decompose outcomes
  (leftovers reconciled). Root cause: `runArbiterPhase`/`resolveArbiter` move HEAD but return only an
  error/count — the new SHAs never flow back to `DecomposeResult.Commits`.
- **The fix**: stagehand already owns every ref mutation, so after a successful arbiter the final
  commits are exactly the range `preRunHEAD..HEAD`. Re-read them with the `LogRange` primitive
  (P1.M3.T1.S1) + `DiffTree` and replace the loop's pre-arbiter entries.
- **Why now / sequencing**: `LogRange` shipped in T1.S1 (Complete). This subtask (T2.S1) is the FIRST
  and ONLY consumer. It does NOT change the success-report printer (`default_action.go` already
  iterates `res.Commits`) or any resolution logic. Downstream work (P1.M6 docs sync) notes the fix.

---

## What

### The `rereadFinalCommits` helper (new, decompose.go — place near `runArbiterPhase`)

```go
// rereadFinalCommits re-reads the FINAL commits this run produced (post-arbiter) by listing the range
// preRunHEAD..HEAD via LogRange and pairing each entry's SHA with DiffTree, rebuilding accurate
// []CommitResult for DecomposeResult.Commits. It closes the §G-RESULT post-arbiter gap: after the
// arbiter amends/rebuilds/creates, the loop's pre-arbiter SHAs are stale (dangling) and the null-path
// (N+1)-th commit is missing from the loop's slice.
//
// baseSHA: preRunHEAD (captured at Decompose step (2) BEFORE the loop/arbiter mutated HEAD), or the
// all-zeros sentinel strings.Repeat("0",40) when the repo was originally unborn (isUnborn) — LogRange
// branches the all-zeros sentinel to the no-range HEAD form (lists ALL commits created this run).
//
// isRoot (per DiffTree): true ONLY for the FIRST entry when isUnborn (concept 0 is the repo's root
// commit). Message is set to "" — printDecomposeCommit prints only SHA+Subject+Files (the full message
// is not part of the success report), so it is not re-fetched.
//
// Best-effort: callers log a non-nil error and fall back to the loop's pre-arbiter commits rather than
// failing the run (the commits are already published; stale SHAs beat erroring). Returns (nil, err) on
// any git read failure (the caller decides the fallback).
func rereadFinalCommits(ctx context.Context, deps Deps, preRunHEAD string, isUnborn bool) ([]CommitResult, error) {
	baseSHA := preRunHEAD
	if isUnborn {
		baseSHA = strings.Repeat("0", 40) // LogRange's all-zeros unborn sentinel → no-range HEAD form
	}
	entries, err := deps.Git.LogRange(ctx, baseSHA)
	if err != nil {
		return nil, fmt.Errorf("%w: log range: %w", ErrDecomposeFailed, err)
	}
	out := make([]CommitResult, 0, len(entries))
	for i, entry := range entries {
		isRoot := isUnborn && i == 0 // concept 0 is the root commit only on an originally-unborn repo
		files, err := deps.Git.DiffTree(ctx, entry.SHA, isRoot)
		if err != nil {
			return nil, fmt.Errorf("%w: diff-tree %s: %w", ErrDecomposeFailed, entry.SHA, err)
		}
		out = append(out, CommitResult{SHA: entry.SHA, Subject: entry.Subject, Message: "", Files: files})
	}
	return out, nil
}
```

### The call site (Decompose, arbiter block — INSERT after `runArbiterPhase` success, before step 9)

The arbiter block is currently (~L178-189):
```go
if status != "" && len(commits) > 0 {
    arbiterCommits := buildArbiterCommits(ctx, deps, commits, isUnborn)
    amended, err = runArbiterPhase(ctx, deps, arbiterCommits, chainData)
    if err != nil {
        return DecomposeResult{}, err
    }
}
// (9) Return.
return DecomposeResult{Commits: commits, Amended: amended}, nil
```
Insert the best-effort reread inside the `if`, right after the error check:
```go
    if err != nil {
        return DecomposeResult{}, err
    }
    // §G-RESULT gap closed: re-read the FINAL commits (post-arbiter) for accurate, resolvable SHAs.
    finalCommits, rerr := rereadFinalCommits(ctx, deps, preRunHEAD, isUnborn)
    if rerr != nil {
        // Best-effort: the commits are already published. Log and keep the loop's pre-arbiter commits.
        deps.Verbose.VerboseRawOutput(fmt.Sprintf("decompose: reread final commits failed (best-effort, keeping loop commits): %v", rerr))
    } else {
        commits = finalCommits
    }
}
```
Step (9) is UNCHANGED — it already returns `DecomposeResult{Commits: commits, Amended: amended}`; `commits`
is now the accurate post-arbiter slice on success (or the loop's fallback on reread error).

### Doc-comment updates (Mode A — the gap is closed)

**`CommitResult` doc** (~L40-42) — remove the stale PRE-amend claim:
```go
// CommitResult is one commit produced by a Decompose run (mirrors generate.Result's commit-relevant
// fields). Ordered oldest-first in DecomposeResult.Commits. On the happy path (arbiter did not run) the
// SHA/Subject/Message/Files are the published commit's. When the arbiter RAN, Decompose re-reads git
// post-arbiter (rereadFinalCommits) so Commits carry the FINAL, resolvable SHAs/subjects/file-lists;
// Message is "" in those rebuilt entries (the success report prints only SHA+Subject+Files).
type CommitResult struct { ... }
```

**`DecomposeResult` §G-RESULT doc** (~L55-67) — rewrite to reflect the closed gap:
```go
// §G-RESULT — the post-arbiter gap is CLOSED: when the arbiter runs, Decompose calls
// rereadFinalCommits (LogRange(preRunHEAD..HEAD) + DiffTree) AFTER runArbiterPhase succeeds and
// replaces Commits with the FINAL, resolvable post-arbiter entries (tip/mid-chain new SHAs; the
// null-path (N+1)-th commit is included). The re-read is best-effort: on a git-read error it logs via
// Verbose and KEEPS the loop's pre-arbiter Commits (commits are already published — stale SHAs beat
// erroring). When the arbiter did NOT run (clean tree → StatusPorcelain "" → arbiter skipped, or
// len(commits)==0), Commits are the loop's accurate entries, unchanged.
```
Update the `Amended` field comment (~L67) to drop "for the post-arbiter gap":
`// arbiter rewrite count (0/1/N-i); 0 if the arbiter did not run or made a new commit (null).`

### Success Criteria

- [ ] `rereadFinalCommits(ctx, deps, preRunHEAD, isUnborn) ([]CommitResult, error)` exists in decompose.go.
- [ ] It is called after `runArbiterPhase` success; reread errors are logged via `deps.Verbose` and do NOT fail.
- [ ] `DecomposeResult.Commits` carries accurate post-arbiter SHAs when the arbiter ran; unchanged otherwise.
- [ ] `CommitResult` + `DecomposeResult` doc comments updated (gap closed).
- [ ] `TestDecompose_ArbiterWiring` updated (1→2); tip/mid/happy-path tests added; all green.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l`, `go test ./...` pass.

---

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed?_ **Yes** — the
exact helper (copy-ready), the exact insertion point, the import to add, the exact doc rewrites, the
existing test to update, the shell-script-arbiter technique for the dynamic-SHA tests (with the
prompt-format facts that make it robust), and the dcm* test helpers are all below.

### Documentation & References

```yaml
# MUST READ — binding architecture (root cause + design)
- file: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue3_post_arbiter_output.md
  why: Names rereadFinalCommits as Step 2 of the Issue-3 fix; explains the 3 SHA-change paths
       (tip/mid/null) and the re-read strategy. NOTE: its all-zeros-range claim was corrected by
       T1.S1 (already shipped) — LogRange now handles the sentinel correctly, so this task just PASSES
       preRunHEAD or all-zeros.
  section: "### Step 2: Re-read commits after arbiter in Decompose"

# The prerequisite primitive (already shipped — T1.S1 Complete) you CONSUME
- file: internal/git/git.go :: LogEntry (L24) + Git.LogRange (L209) + gitRunner.LogRange (L783)
  why: LogRange(ctx, baseSHA) returns []LogEntry{SHA, Subject} oldest-first for baseSHA..HEAD. The
       all-zeros sentinel strings.Repeat("0",40) branches to the no-range HEAD form (lists ALL commits
       created this run on an originally-unborn repo). Empty/unborn → (nil,nil).
  pattern: "deps.Git.LogRange(ctx, preRunHEAD)" (or all-zeros when isUnborn).

- file: internal/git/git.go :: DiffTree (L434)
  why: DiffTree(ctx, sha, isRoot) returns []FileChange (the FR42 file-list). isRoot true ONLY for a
       root commit (concept 0 on an unborn repo) — same isRoot semantics the loop uses in buildCommitResult.

# The edit site + the patterns to mirror
- file: internal/decompose/decompose.go
  why: (a) ADD \"strings\" to the import block (L23-30 — currently NO strings import). (b) rereadFinalCommits
        goes near runArbiterPhase (~L415). (c) The call site is inside the arbiter if-block (~L178-189).
        (d) buildCommitResult (~L470) is the DiffTree→CommitResult precedent to mirror (isRoot handling).
        (e) preRunHEAD/isUnborn captured at step (2) (~L114).
  pattern: Mirror buildCommitResult's DiffTree call + CommitResult construction. Mirror runArbiterPhase's
           ErrDecomposeFailed-wrapped error style.

# Verbose sink (best-effort logging — nil-safe)
- file: internal/ui/verbose.go
  why: Deps.Verbose is *ui.Verbose (nullable). VerboseRawOutput(string) is the ONLY free-form Verbose
        method; it is a no-op when v==nil || v.w==nil || !v.on (dcmDeps sets Verbose: nil → no-op in tests).
        VerboseCommand/VerboseRetry are arg-shaped (command/retry); use VerboseRawOutput for the diagnostic.

# Tests to mirror + UPDATE
- file: internal/decompose/decompose_test.go :: TestDecompose_ArbiterWiring (~L718)
  why: The ONLY existing full-Decompose test where the arbiter RUNS with a leftover (null path). Its
        `len(result.Commits) != 1` assertion (currently ~L744) MUST become `!= 2`. result.Amended==0 stays.
  pattern: dcm* helpers (dcmInitRepo/dcmWriteFile/dcmDeps/dcmStagerSeam/dcmHeadSHA/dcmLogCount/
           dcmStatusPorcelain/dcmArbiterManifest/dcmMessageScriptManifest), stubtest.Build/Manifest.

- file: internal/decompose/chain_test.go :: TestResolveArbiter_TipAmend / _MidChainRebuild (L174/L237)
  why: Unit tests for resolveArbiter that assert the SHA changes in git (the resolution correctness is
        ALREADY covered). These prove tip amend produces a new tip SHA and mid-chain rebuilds [i..N-1].
        The NEW thing THIS task adds is the re-READ wiring — assert it via full Decompose (Tasks 7-8).

# How the stub agent works (for the shell-script-arbiter test technique)
- file: cmd/stubagent/main.go
  why: The stub emits fixed responses (STAGEHAND_STUB_OUT/SCRIPT) — it CANNOT read git. For tip/mid-chain
        tests the arbiter must return a DYNAMIC SHA, so override Manifest.Command with a shell script that
        parses SHAs from its STDIN prompt (see "Known Gotchas" + Tasks 7-8).
  critical: The agent subprocess cwd is the USER's CWD, NOT the repo (executor.go L25). Do NOT have the
            script run git — parse the SHAs from STDIN (the arbiter payload lists each commit's SHA on
            its own 40-hex line; prompt/arbiter.go BuildArbiterUserPayload).
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/decompose.go   # Decompose (L108), arbiter block (L178), runArbiterPhase (L415), buildCommitResult (L470)
internal/decompose/decompose_test.go  # dcm* helpers (L40-210); TestDecompose_ArbiterWiring (L718); ArbiterSkippedOnCleanTree (L677)
internal/git/git.go               # LogRange (L783, shipped T1.S1) + DiffTree (L434) — CONSUMED, not modified
internal/cmd/default_action.go    # runDecompose/printDecomposeCommit (L270/L308) — NOT modified
```

### Desired Codebase tree with files changed

```bash
internal/decompose/decompose.go        # MODIFIED — +strings import, +rereadFinalCommits, +call site, doc comments
internal/decompose/decompose_test.go   # MODIFIED — UPDATE ArbiterWiring (1→2); +dcmShaResolves, +dcmScriptArbiter helpers;
                                       #            +tip-amend, +mid-chain, +happy-path-unchanged tests
# (git.go, chain.go, default_action.go, arbiter.go — UNCHANGED)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: decompose.go does NOT import "strings" today (imports: context/errors/fmt + 4 internal
//   pkgs). rereadFinalCommits uses strings.Repeat("0", 40) → ADD "strings" to the import block. A
//   compile error ("undefined: strings") is the #1 way this task fails one-pass if missed.

// CRITICAL: baseSHA for LogRange is preRunHEAD (captured at step 2) for a BORN repo, and the all-zeros
//   sentinel strings.Repeat("0",40) for an originally-UNBORN repo (isUnborn). LogRange (shipped T1.S1)
//   already branches the all-zeros sentinel to the no-range HEAD form — do NOT re-implement that here.

// CRITICAL (test): the EXISTING TestDecompose_ArbiterWiring asserts len(result.Commits)==1. After this
//   fix the null-path (N+1)-th commit is re-read & included → it MUST be ==2. If you only ADD new tests
//   and forget this, `go test` FAILS. Two other full-Decompose arbiter tests leave a CLEAN tree after
//   the loop (arbiter does NOT run, status=="") so they stay len==N: TestDecompose_RoleResolvesSubProvider
//   (~L1500) and the stager-retry test (~L1105) — DO NOT touch them.

// GOTCHA: DiffTree's isRoot is true ONLY for concept 0 on an originally-unborn repo. In rereadFinalCommits
//   that is `isUnborn && i==0` (the FIRST LogRange entry). The loop's buildCommitResult uses the SAME rule.

// GOTCHA: Message="" in rebuilt CommitResult. printDecomposeCommit prints ONLY [<short-sha>] <subject>
//   + per-file lines — it never reads Message. Re-fetching the full message would be wasted git work.

// GOTCHA: best-effort logging via deps.Verbose.VerboseRawOutput — Deps.Verbose is *ui.Verbose (nullable);
//   every method is a no-op when nil/off. dcmDeps sets Verbose: nil → the log is silent in most tests
//   (correct — it's a diagnostic). Do NOT fail the run on a reread error; keep the loop's `commits`.

// GOTCHA (tests): the stub agent (cmd/stubagent) emits FIXED responses and CANNOT read git. For tip
//   amend / mid-chain the arbiter must return a DYNAMIC SHA. The agent cwd is the USER's CWD (NOT the
//   repo). BUT the arbiter prompt is on STDIN and lists each commit's SHA on its own 40-hex line
//   (prompt/arbiter.go: "SHA\nSubject\nfiles...\n\n"). A shell-script arbiter parses STDIN → emits
//   {"target":"<sha>"}. Override stubtest.Manifest(...).Command with the script path. The extracted SHA
//   is in the run's valid set → passes runArbiter's targetInRun check.
//   - Bare 40-hex lines appear ONLY in the commit-list section (leftover diff lines are +/-/space-prefixed)
//     → the sed pattern is robust.
//   - resolveTipAmend / resolveMidChain REUSE the original messages (NO extra message-agent call); only
//     resolveNewCommit (null) calls generateMessage → the message-script needs an extra entry for null.
```

---

## Implementation Blueprint

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/decompose/decompose.go :: add the "strings" import
  - ADD: "strings" to the import block (L23-30), grouped with the stdlib imports (context/errors/fmt).
  - WHY: rereadFinalCommits uses strings.Repeat("0", 40) — strings is NOT currently imported.
  - DEPENDENCIES: none. (Do this FIRST — it is the #1 one-pass failure mode if forgotten.)

Task 2: MODIFY internal/decompose/decompose.go :: add rereadFinalCommits helper
  - ADD: the rereadFinalCommits function (code in "What" above), placed near runArbiterPhase (~L415).
  - FOLLOW pattern: buildCommitResult (~L470) for DiffTree→CommitResult construction + isRoot handling.
  - REUSE: deps.Git.LogRange (all-zeros sentinel via strings.Repeat("0",40) when isUnborn) + deps.Git.DiffTree.
  - ERROR STYLE: wrap with ErrDecomposeFailed (mirror runArbiterPhase: "%w: log range: %w", "%w: diff-tree %s: %w").
  - NAMING: rereadFinalCommits (unexported; same package). Message field = "".
  - DEPENDENCIES: "strings" import (Task 1); LogRange/DiffTree (shipped T1.S1, present).

Task 3: MODIFY internal/decompose/decompose.go :: call rereadFinalCommits after arbiter success
  - FIND: the arbiter if-block (status != "" && len(commits) > 0), the `amended, err = runArbiterPhase(...)`
          + its `if err != nil { return ... }` (~L181-184).
  - INSERT (code in "What" above) immediately after the error check, INSIDE the if-block: call
          rereadFinalCommits; on rerr != nil log via deps.Verbose.VerboseRawOutput and KEEP `commits`;
          else commits = finalCommits.
  - PRESERVE: the loop's `commits` fallback (do not clobber on error); step (9) return is UNCHANGED.
  - GOTCHA: use a plain block (finalCommits, rerr := ...) not the if-init form, so `commits = finalCommits`
            is in scope. `rerr` must NOT shadow/conflict with the outer `err`.
  - DEPENDENCIES: rereadFinalCommits (Task 2); preRunHEAD + isUnborn (captured at step 2, in scope).

Task 4: MODIFY internal/decompose/decompose.go :: update doc comments (Mode A)
  - REWRITE: CommitResult doc (~L40-42) — drop the PRE-amend claim; note post-arbiter re-read + Message="".
  - REWRITE: DecomposeResult §G-RESULT doc (~L55-67) — gap is CLOSED when arbiter ran; unchanged when not.
  - UPDATE: Amended field comment (~L67) — drop "for the post-arbiter gap".
  - DOC MODE: Mode A (the doc rides with the work — these comments describe THIS type's semantics).
  - DEPENDENCIES: Tasks 2-3 (so the comments match the code).

Task 5: MODIFY internal/decompose/decompose_test.go :: add test helpers
  - ADD: dcmShaResolves(t, repo, sha) bool — runs `git -C <repo> rev-parse --verify <sha>^{commit}`;
         exit 0 ⇒ true (resolvable, not dangling). Use exec.Command + CombinedOutput (mirror dcmRunGit style).
  - ADD: dcmScriptArbiter(t, mode) provider.Manifest — writes a shell script to t.TempDir(), chmod 0755,
         builds stubtest.Manifest(bin, Options{Out:""}) and OVERRIDES .Command to the script path:
           - mode "tip":      `#!/bin/sh` + `sha=$(sed -n 's/^\([0-9a-f]\{40\}\)$/\1/p' | tail -n 1)` + `printf '{"target": "%s"}\n' "$sha"`
           - mode "mid":      same but `... | sed -n '2p'` (the 2nd SHA = concept[1] for N≥3)
         (bin from stubtest.Build(t); .Command is *string — take addr of the path var.)
  - WHY: tip/mid-chain arbiter must return a DYNAMIC SHA created at runtime by the loop; the stub binary
         can't read git, so parse the SHA from the arbiter's STDIN prompt (each commit's SHA is a 40-hex line).
  - DEPENDENCIES: stubtest.Build, stubtest.Manifest, provider.Manifest (Command is *string).

Task 6: MODIFY internal/decompose/decompose_test.go :: UPDATE TestDecompose_ArbiterWiring (null path)
  - FIND: `if len(result.Commits) != 1` (~L744) → CHANGE to `!= 2` (the null-path commit is now included).
  - ADD assertions: result.Commits[1].Subject == "feat: add leftover"; dcmShaResolves(repo, result.Commits[1].SHA);
          result.Commits[1].SHA == dcmHeadSHA(repo) (the new commit is the tip).
  - KEEP: result.Amended == 0 (null target → 0); dcmLogCount == 2; clean tree.
  - CRITICAL: this is the existing test that BREAKS without the assertion change.

Task 7: CREATE/ADD tests :: tip amend (full Decompose) — decompose_test.go
  - SETUP: 2 concepts (c1:a.txt, c2:b.txt) + a leftover (c.txt) the stager seam leaves un-staged.
  - arbiter = dcmScriptArbiter(t, "tip") (returns the tip SHA). message-script: ["feat: add a","feat: add b"]
          (resolveTipAmend REUSES messages — no extra message call).
  - ASSERT: len(result.Commits) == 2; dcmShaResolves for BOTH SHAs; result.Commits[1].SHA == dcmHeadSHA(repo)
          (post-amend tip); result.Commits[1].SHA != the pre-amend tip (capture via the script writing the
          chosen SHA to a side-channel file, OR assert the tip resolves + matches HEAD which implies it's new);
          result.Commits[1].Files contains BOTH b.txt and c.txt (leftover folded into the tip); clean tree.
  - NAMING: TestDecompose_ArbiterTipAmend_RereadsFinalSHA. Reuse dcm* helpers + dcmStagerSeam.

Task 8: CREATE/ADD tests :: mid-chain + happy-path-unchanged — decompose_test.go
  - MID-CHAIN: 3 concepts (c1:a.txt, c2:b.txt, c3:c.txt) + leftover (d.txt). arbiter = dcmScriptArbiter(t,"mid")
          (2nd SHA = concept[1]). message-script: 3 entries (resolveMidChain reuses messages). ASSERT:
          len(result.Commits)==3; dcmShaResolves for ALL 3; the SHAs match `git log --reverse --format=%H`
          (split on "\n"); result.Commits[0].SHA matches the UNCHANGED concept[0] (dcmGitOut rev-parse of the
          first log SHA); clean tree. NAMING: TestDecompose_ArbiterMidChain_AllSHAsResolve.
  - HAPPY-PATH-UNCHANGED: reuse the EXISTING TestDecompose_ArbiterSkippedOnCleanTree (~L677) — it already
          proves the arbiter does NOT run on a clean tree. ADD a focused assertion that result.Commits[0].SHA
          resolves (dcmShaResolves) and == dcmHeadSHA (the loop's entries are accurate, unchanged).
          NAMING: either extend the existing test or add TestDecompose_HappyPath_CommitsAccurate.
  - DEPENDENCIES: helpers from Task 5.
```

### Implementation Patterns & Key Details

```go
// PATTERN (DiffTree→CommitResult): mirror buildCommitResult (decompose.go ~L470):
//   files, err := deps.Git.DiffTree(ctx, sha, isRoot)  // isRoot = (concept 0 && unborn)
//   CommitResult{SHA, Subject, Message, Files}
// rereadFinalCommits iterates LogRange entries instead of a single sha, and sets Message="".

// PATTERN (best-effort, keep-loop-commits): the arbiter block's error already returns; the reread is
//   strictly post-success and must NOT turn a published-commit success into a failure. Mirror the
//   codebase's "best-effort read" convention (dupCheckMessage returns false on read failure rather
//   than erroring): on rerr != nil, log + keep the loop `commits`; on success, commits = finalCommits.

// PATTERN (ErrDecomposeFailed-wrapped errors): runArbiterPhase wraps "%w: leftover diff: %w".
//   rereadFinalCommits wraps "%w: log range: %w" and "%w: diff-tree %s: %w" for consistency.

// SHELL-SCRIPT ARBITER (test technique): the arbiter prompt lists each commit as SHA\nSubject\nfiles\n\n.
//   A bare 40-hex line appears ONLY as a commit SHA (the leftover diff is +/-/space-prefixed). So:
//     sed -n 's/^\([0-9a-f]\{40\}\)$/\1/p' | tail -n 1   # tip (last SHA)
//     sed -n 's/^\([0-9a-f]\{40\}\)$/\1/p' | sed -n '2p' # 2nd SHA = concept[1] for a 3-concept run
//   reliably extracts the desired SHA from STDIN. The script emits {"target":"<sha>"}; the SHA is in
//   the run's valid set → passes runArbiter's targetInRun; resolveTipAmend/resolveMidChain reuse messages.
```

### Integration Points

```yaml
CODE: internal/decompose/decompose.go (+ "strings" import) + decompose_test.go ONLY.
GIT (read-only): LogRange (baseSHA..HEAD, oldest-first) + DiffTree — both pure reads; no ref/index mutation.
INTERFACE CONTRACT: no change to git.Git (LogRange/DiffTree shipped in T1.S1). No change to Deps.
ARBITER / CHAIN: UNCHANGED (runArbiterPhase/resolveArbiter/computeAmended untouched).
PRINTER: UNCHANGED (default_action.go runDecompose/printDecomposeCommit iterate res.Commits as-is).
DOCS: Mode A — in-source doc comments only (CommitResult + DecomposeResult §G-RESULT). No user-facing
      docs change (success-report format is unchanged; P1.M6.T1.S2 verifies docs consistency later).
```

---

## Validation Loop

### Level 1: Syntax & Style (run after Tasks 1-4)

```bash
cd /home/dustin/projects/stagehand
go build ./...                              # MUST pass — catches the missing "strings" import first.
go vet ./internal/decompose/...
gofmt -l internal/decompose/decompose.go    # expect: no output (else gofmt -w it)
# If `go build` errors "undefined: strings" → Task 1 was skipped; add the import.
```

### Level 2: Unit / integration tests (run after Tasks 5-8)

```bash
# The updated + new decompose tests (all must pass).
go test ./internal/decompose/... -run 'TestDecompose_Arbiter|TestDecompose_HappyPath|TestDecompose_ArbiterSkippedOnCleanTree' -v

# Full package + full suite (nothing else regresses — git.go/chain.go/default_action.go are untouched).
go test ./...
# Expected: all PASS. Reasoning (why nothing else breaks):
#   - rereadFinalCommits is a NEW function; the only existing behavior change is that res.Commits is now
#     ACCURATE post-arbiter (more commits on the null path; resolvable SHAs on tip/mid). The ONLY existing
#     assertion that this invalidates is TestDecompose_ArbiterWiring's `len != 1` → updated to `!= 2` (Task 6).
#   - default_action.go/chain.go/arbiter.go are UNCHANGED → their tests are unaffected.
#   - The two other full-Decompose arbiter tests (RoleResolvesSubProvider ~L1500, stager-retry ~L1105)
#     leave a clean tree → arbiter does NOT run → res.Commits is the loop's entries (unchanged) → unaffected.
```

> **If a previously-green test now fails**: it MUST be a test that asserted on the COUNT or SHAs of
> `res.Commits` after an arbiter RUN. Only `TestDecompose_ArbiterWiring` (null path) does so — updated in
> Task 6. Any other failure means rereadFinalCommits ran when it shouldn't (arbiter gate) or mis-counted:
> re-check the call is INSIDE `if status != "" && len(commits) > 0` and that baseSHA is preRunHEAD (or
> all-zeros when isUnborn), and that isRoot is `isUnborn && i==0`.

### Level 3: Manual / empirical (proves the git semantics end-to-end via the CLI binary)

```bash
# Build the CLI; exercise a real decompose where the arbiter folds a leftover into the tip.
# (Requires a configured default provider / stub; the unit tests in Level 2 are the primary gate.)
go build -o /tmp/stagehand ./cmd/stagehand
# In a throwaway repo with an un-staged dirty tree the planner partitions, observe the success report's
# [<short-sha>] lines now MATCH `git log --oneline` after the arbiter ran (no dangling SHAs), and a
# null-path run prints the (N+1)-th commit. (See architecture/issue3_post_arbiter_output.md reproduction.)
git log --oneline   # the printed SHAs must all appear here (resolvable, correct order/count).
```

### Level 4: Doc-comment consistency

```bash
grep -n "§G-RESULT\|PRE-amend\|post-arbiter gap\|rereadFinalCommits" internal/decompose/decompose.go
# Expect: §G-RESULT rewritten to say the gap is CLOSED; no remaining "PRE-amend" claim on CommitResult;
# rereadFinalCommits defined and referenced. The Amended field comment drops "post-arbiter gap".
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (verifies the `"strings"` import was added).
- [ ] `go vet ./internal/decompose/...` clean.
- [ ] `gofmt -l internal/decompose/decompose.go internal/decompose/decompose_test.go` reports nothing.
- [ ] `go test ./internal/decompose/...` passes (incl. updated ArbiterWiring + new tip/mid/happy tests).
- [ ] `go test ./...` — all previously-green tests still PASS.

### Feature Validation
- [ ] `rereadFinalCommits(ctx, deps, preRunHEAD, isUnborn)` exists; calls LogRange + DiffTree; Message="".
- [ ] It is called after `runArbiterPhase` success; reread errors are logged via `deps.Verbose` and do NOT fail.
- [ ] Null path: `len(res.Commits) == N+1`, last SHA resolves & == HEAD; the (N+1)-th commit IS printed.
- [ ] Tip amend: `len(res.Commits) == N`; `res.Commits[last].SHA` resolves & == HEAD (post-amend, not dangling).
- [ ] Mid-chain: ALL `res.Commits[*].SHA` resolve & match `git log --reverse --format=%H`.
- [ ] Happy path (arbiter skipped): `res.Commits` is the loop's accurate entries (unchanged).
- [ ] `CommitResult` + `DecomposeResult` §G-RESULT doc comments reflect the closed gap (Mode A).

### Code Quality Validation
- [ ] Follows existing patterns: `buildCommitResult` (DiffTree→CommitResult), `runArbiterPhase` (ErrDecomposeFailed wrap),
      `dupCheckMessage` (best-effort read → fall back, don't error).
- [ ] `isRoot` rule (`isUnborn && i==0`) matches the loop's `buildCommitResult`; baseSHA = all-zeros only when isUnborn.
- [ ] Files touched: ONLY `decompose.go` + `decompose_test.go` (git.go/chain.go/arbiter.go/default_action.go untouched).
- [ ] Test helpers (`dcmShaResolves`, `dcmScriptArbiter`) reuse `dcm*`/`stubtest` conventions; shell-script arbiter parses
      STDIN (does not rely on agent cwd == repo, which it is NOT).
- [ ] Anti-patterns avoided (see below).

### Documentation
- [ ] In-source doc comments (Mode A) accurately describe post-arbiter re-read + best-effort fallback + Message="".

---

## Anti-Patterns to Avoid

- ❌ **Don't forget the `"strings"` import** in decompose.go — `strings.Repeat("0",40)` won't compile without it (the #1 one-pass failure).
- ❌ **Don't fail the run on a reread error** — it's best-effort. Log via `deps.Verbose` and KEEP the loop's pre-arbiter `commits` (commits are already published).
- ❌ **Don't skip updating `TestDecompose_ArbiterWiring`** — its `len != 1` assertion breaks (null-path commit is now included). Change to `!= 2`.
- ❌ **Don't touch the two clean-tree full-Decompose arbiter tests** (RoleResolvesSubProvider ~L1500, stager-retry ~L1105) — the arbiter doesn't run there (status==""), so their `len == N` assertions are still correct.
- ❌ **Don't re-fetch the full Message** — `printDecomposeCommit` prints only SHA+Subject+Files; set Message="".
- ❌ **Don't have the shell-script arbiter run `git`** — the agent cwd is the USER's CWD, not the repo. Parse SHAs from the STDIN prompt (each commit's SHA is a 40-hex line).
- ❌ **Don't re-implement the all-zeros LogRange special-case** — LogRange (T1.S1) already branches the sentinel to the no-range form; just PASS preRunHEAD (or all-zeros when isUnborn).
- ❌ **Don't shadow the outer `err`** — use `rerr` for the reread error; keep `amended`/`err` from `runArbiterPhase` intact.
- ❌ **Don't modify `chain.go`, `arbiter.go`, `default_action.go`, or `git.go`** — out of scope (resolution logic, printer, and the primitive are all correct/unchanged).
- ❌ **Don't set isRoot wrong** — it's `isUnborn && i==0` (first entry only on an unborn repo), matching `buildCommitResult`.

---

## Confidence Score

**9/10** — The helper is copy-ready and verified against the shipped `LogRange`/`DiffTree` API; the
exact insertion point (after `runArbiterPhase` success), the import to add (`strings`), the exact doc
rewrites, the existing test to update (`TestDecompose_ArbiterWiring` 1→2), and the unaffected tests
(two clean-tree full-Decompose arbiter tests) are all pinned to exact line numbers. The
shell-script-arbiter technique for the dynamic tip/mid-chain SHAs is grounded in the real prompt format
(each commit's SHA is a bare 40-hex line on STDIN) and the real executor fact (agent cwd ≠ repo, but
the prompt carries the SHAs). The -1 reserves for the test-time detail of capturing the pre-amend SHA
for the explicit negative assertion in the tip test — the core assertions (tip resolves + matches HEAD)
are robust regardless; the side-channel capture is a nice-to-have the implementer can add or omit.
