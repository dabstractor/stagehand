# P3.M2.T1.S1 Research Findings — Freeze enforcement (tree[i] ⊆ T_start, FR-M1c)

Empirical findings from reading the codebase (2026-07-02). The parallel task P3.M1.T1.S2 is ALREADY
reflected on disk (tStart captured at decompose.go:164; threaded into callPlanner/runSingleShortcut/
runArbiterPhase; runLoop does NOT yet take tStart — that's THIS task). Every claim is file:line-backed.

## 1. The contract (verbatim from the work item)

After each staging step (invokeStagerRetry → freezeSnapshot → tree[i]), VERIFY tree[i] is a CONTENT-
SUBSET of T_start: every path in diff(baseTree, tree[i]) must be in diff(baseTree, T_start) AND agree on
blob content (tree[i]'s blob == T_start's blob for that path). Any deviation (a concurrent working-tree
change the stager swept in, OR a stager that ran a bare `git add -A` staging a path/content not in
T_start) is a HARD ABORT: new sentinel ErrFreezeViolation (NON-RESCUE; already-landed commits stand,
mirroring the HEAD-movement guard). The orchestrator owns the freeze boundary, NOT the stager.
INPUT: P3.M1.T1.S2 (T_start captured + threaded) + P3.M1.T1.S1 (DiffTreeNames). DOCS: decompose.go /
stager.go doc comment (FR-M1c).

## 2. The loop anchor (where the check goes) — decompose.go runLoop

runLoop signature (decompose.go:291): `func runLoop(ctx, deps Deps, concepts []prompt.PlannerCommit,
baseTree, preRunHEAD string, isUnborn bool)`. Called at decompose.go:185 (does NOT take tStart yet).

Loop body (decompose.go:380–410, verbatim order):
```
for i, concept := range concepts {
    if err := invokeStagerRetry(concept); err != nil { drainMsg(inflight); return commits, nil, err }
    treeI, err := freezeSnapshot(ctx, deps)
    if err != nil { drainMsg(inflight); return commits, nil, fmt.Errorf("%w: freeze snapshot[%d]: %w", ErrDecomposeFailed, i, err) }
    skipped := treeI == prevTree
    if err := publish(inflight); err != nil { return commits, nil, err }   // publishes commit[i-1]
    inflight = nil
    if !skipped { signal.SetSnapshot(...); inflight = launch(i, prevTree, treeI); prevTree = treeI }
}
publish(inflight)  // final commit[N-1]
```

THE INSERTION POINT: between `freezeSnapshot` (treeI obtained) and `skipped := treeI == prevTree`.
On violation: `drainMsg(inflight); return commits, nil, <ErrFreezeViolation wrap>` — the EXACT pattern
the HEAD-movement guard + freezeSnapshot-error paths use (drainMsg + return partial).

## 3. ErrStagerMovedHEAD — the hard-abort precedent (stager.go:46)

stager.go defines ErrStagerMovedHEAD with a rich doc: "a stager SAFETY VIOLATION ... HARD (non-rescue)
error ... when the guard fires, there is no snapshot to restore ... The run aborts (exit 1)." The guard
LIVES in decompose.go invokeStagerRetry (decompose.go:353–378): snapshot HEAD pre-call, run stager,
re-read HEAD post-call, abort if moved; bypasses retry-once-then-empty via errors.Is short-circuits.

ErrFreezeViolation MIRRORS this exactly: sentinel in stager.go (next to ErrStagerMovedHEAD), guard LOGIC
in the orchestrator (a verifyFreezeSubset helper called from runLoop). Same non-rescue, partial-commits-
stand semantics. Same `errors.Is(err, ErrFreezeViolation)` test pattern (see TestDecompose_StagerMovedHEAD,
decompose_test.go:573).

## 4. The content-subset check — DONE WITH ONLY DiffTreeNames (no new git primitive)

DiffTreeNames (git.go:258 interface / :1240 impl) returns the SORTED, DEDUPED changed-path set between
two trees via `git diff-tree -r --name-only --no-commit-id`. Identical trees ⇒ (nil, nil). Read-only w.r.t.
refs+index. NOT unborn-aware (caller passes EmptyTreeSHA).

THE MATH (the load-bearing insight — verified by example in §5): tree[i] is a content-subset of T_start
(for tree[i]'s changed paths) IFF:
  (A) PATH check:  changedTreeI ⊆ changedTStart, where
        changedTreeI  = DiffTreeNames(baseTree, treeI)
        changedTStart = DiffTreeNames(baseTree, tStart)   [compute ONCE per run — invariant]
  (B) CONTENT check: changedTreeI ∩ DiffTreeNames(treeI, tStart) == ∅
        (equiv to the contract's "git diff tree[i] T_start -- <changed paths> is empty" — a path p is in
         DiffTreeNames(treeI, tStart) iff blob(treeI,p) != blob(tStart,p); intersecting with changedTreeI
         isolates tree[i]'s changed paths whose content differs from T_start ⇒ the violations.)

Proof CONTENT alone is complete: a path-violation p (p ∈ changedTreeI, p ∉ changedTStart) has
blob(tStart,p)==blob(base,p) [unchanged in T_start] != blob(treeI,p) [changed in treeI] ⇒ p ∈
DiffTreeNames(treeI, tStart) ⇒ caught by (B). So (B) subsumes (A) for DETECTION. KEEP BOTH anyway: (A)
gives the clearer "path not present in T_start" diagnostic, matches the contract's two-part framing, and
is cheap (set lookups). Cost: 2 DiffTreeNames calls/concept (changedTreeI + delta) + 1/run (changedTStart).
No new git primitive, no pathspec (the git.Git interface has NO path-restricted diff — StagedDiffOptions
has only `Excludes`, not includes — so DiffTreeNames is the only viable primitive; the intersection trick
is HOW you do the contract's path-restricted content check without a pathspec).

## 5. Worked example (proves §4)

base = {a=v0, b=v0}. working = {a=v1, b=v1}. tStart = base+{a=v1,b=v1}. changedTStart={a,b}.
- WELL-BEHAVED (git add a): treeI=base+{a=v1}. changedTreeI={a}. delta=DiffTreeNames(treeI,tStart)={b}
  (b=v0 in treeI vs v1 in tStart). contentMismatch={a}∩{b}=∅ ⇒ OK. ✓ (a's content matches tStart.)
- CONTENT ROGUE (modify a→v2, git add a): treeI=base+{a=v2}. changedTreeI={a}. path check passes (a∈
  changedTStart). delta=DiffTreeNames(treeI,tStart)={a,b} (a v2≠v1, b v0≠v1). contentMismatch={a}∩{a,b}=
  {a} ⇒ VIOLATION. ✓
- PATH ROGUE (git add a + concurrent sentinel): treeI=base+{a=v1,sentinel=v9}. changedTreeI={a,sentinel}.
  path check: sentinel∉changedTStart ⇒ VIOLATION (path). ✓ (content check would also catch it.)

## 6. What changes + where (the exact diff)

(a) stager.go — ADD `var ErrFreezeViolation = errors.New("decompose: freeze violation")` (next to
    ErrStagerMovedHEAD, :46) with a rich doc (FR-M1c: orchestrator owns the freeze boundary; non-rescue;
    partial commits stand). ADD `func verifyFreezeSubset(ctx, deps Deps, baseTree, tStart string,
    tStartPaths []string, i int, treeI string) error` (next to freezeSnapshot, :108) implementing §4:
    changedTreeI=DiffTreeNames(baseTree,treeI) → path check vs tStartPaths (set) → content check
    (DiffTreeNames(treeI,tStart) ∩ changedTreeI). Returns ErrFreezeViolation-wrapped on violation,
    ErrDecomposeFailed-wrapped on git error, nil if OK. ADD a private `pathSet` helper. Imports: +strings.
(b) decompose.go runLoop — signature +`tStart string` (after baseTree); compute
    `tStartPaths, err := deps.Git.DiffTreeNames(ctx, baseTree, tStart)` once before the loop (error →
    ErrDecomposeFailed wrap, return nil,nil,err); after freezeSnapshot + before `skipped :=`, insert
    `if err := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, treeI); err != nil {
    drainMsg(inflight); return commits, nil, err }`. Call site (decompose.go:185) +tStart.
(c) Doc comments: stager.go ErrFreezeViolation + verifyFreezeSubset (FR-M1c); decompose.go runLoop doc
    + the freeze-boundary doc note FR-M1c (the loop verifies tree[i] ⊆ T_start; the orchestrator owns it).

## 7. The test seam — deps.stager (roles.go Deps, decompose.go invokeStager)

Deps has an OPTIONAL `stager func(ctx, deps Deps, concept prompt.PlannerCommit) error` test seam
(roles.go). invokeStager(ctx, deps, concept) dispatches to deps.stager if non-nil else stageConcept. The
rogue-stager test (TestDecompose_StagerMovedHEAD, decompose_test.go:573) sets deps.stager to a closure
that runs git directly (dcmRunGit). For ErrFreezeViolation: a closure that stages the concept path AND a
sentinel file written AFTER the freeze (simulating `git add -A` sweeping a concurrent change) → treeI has
the sentinel ∉ T_start → ErrFreezeViolation. Helpers: dcmInitRepo/dcmCommitRaw/dcmWriteFile/dcmRunGit/
dcmPlannerManifest/dcmMessageScriptManifest/dcmAllRoles/dcmDeps/dcmLogCount (all in decompose_test.go,
package decompose — reuse, do NOT redefine).

## 8. Cohesion / scope (the arbiter-staging consideration)

The contract (point 3) scopes THIS task to the per-concept LOOP (invokeStagerRetry → freezeSnapshot →
tree[i]) — the EXTERNAL tooled stager, which is the trust boundary FR-M1c names ("the stager is an
external agent running git against the live tree"). The arbiter is BARE (stagecoach owns its git:
resolveNewCommit/resolveTipAmend/resolveMidChain in chain.go use AddAll/Add). P3.M1.T1.S2's note
("hardened by P3.M2.T1.S1") is AMBIGUOUS, but the operational contract is the loop. DECISION: implement
the loop check (contract point 3); FLAG the arbiter's stagecoach-owned AddAll as a SEPARATE freeze surface
(concurrent change COULD be swept by AddAll; closing it means staging from T_start, a chain.go change —
NOT this task). Under the no-concurrent-change invariant (the common case) the arbiter's AddAll already
yields a T_start subset. This is a conscious scoping decision, documented in the PRP, not an oversight.

## 9. The overlap subtlety (what "stands" on violation)

With runLoop's 1-deep overlap, publish(inflight) [commit i-1] happens AFTER freezeSnapshot[i]. So when
verifyFreezeSubset fires at concept i, commit[i-1] is IN-FLIGHT (drained, not landed); the LANDED commits
are 0..i-2. The contract's "already-landed commits 0..i-1 stand" reflects the logical concept count; the
overlap means the in-flight one's STAGING remains in the index (FR-M12 "remaining staged work is left in
the index"). The PRIMARY test uses count=1 (concept 0 violates ⇒ no prior commits, clean abort, HEAD
unchanged, sentinel excluded) — the partial-stands semantics are structurally identical to the HEAD guard
(same drainMsg+return partial). No need to re-prove partial-stands (already tested for the HEAD guard).

## 10. Validation gates (Go project)

`go build ./...` · `go test ./internal/decompose/... -v` · `go test ./...` (no regression) ·
`go vet ./...` · `golangci-lint run` (.golangci.yml: errcheck/gosimple/govet/ineffassign/staticcheck/
unused) · `gofmt -l internal/ pkg/` (empty). All changes are in-package (decompose): stager.go (+sentinel
+helper), decompose.go (runLoop sig + the verify call + call site + docs), + tests. No new files. No git.go
change (DiffTreeNames is consumed as-is). go.mod/go.sum UNCHANGED.
