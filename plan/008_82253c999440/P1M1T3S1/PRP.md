---
name: "P1.M1.T3.S1 — Concurrent-across-arbiter-gate + T_start completeness integration tests"
description: |
  TEST-ONLY task (no production code). The permanent FR-M1d regression net: prove a post-`T_start`
  concurrent working-tree change can NEVER enter an arbiter commit, and the arbiter is SKIPPED when the
  frozen leftover is empty. The freeze-safe arbiter (P1.M1.T2.S1) is LANDED on disk (gate =
  `DiffTreeNames(tipTree, tStart)` at decompose.go:223; `resolveArbiter` 7-arg at chain.go:50; paths A/B
  `treePrime := tStart`; path C `OverlayTreePaths`; `ReadTree(tStart)` syncs). These tests PASS against
  the landed code — they encode the post-fix invariant so a future regression is caught. (1) UPGRADE the
  existing `TestDecompose_ConcurrentChangeExclusion` (decompose_test.go:819-882) — its fixture IS the
  empty-frozen-leftover case (2-concept unborn repo; stagers cover all of `T_start`; the
  `concurrentSentinelSeam` writes `sentinel.txt` UNSTAGED post-freeze). Add: sentinel in NO commit across
  the whole run (`git log --name-only --format=`); sentinel REMAINS untracked (`?? sentinel.txt`); EXACTLY
  2 commits via `dcmLogCount` (arbiter skipped — empty frozen leftover). Trim the message script 4→2
  (arbiter no longer calls for a message); remove the stale loophole-admitting comments (L838, L880-882).
  (2) ADD `TestDecompose_TStartCompleteness` (PRD §20.2 "T_start completeness" invariant, contract 3c) on a
  BORN repo: 2-concept run whose stagers cover all of `T_start` + a post-freeze sentinel; assert every
  frozen path ⊆ `DiffTreeNames(baseTree, HEAD^{tree})` via `git.New(repo).DiffTreeNames`; `git status
  --porcelain` shows ONLY `?? sentinel.txt` (no `T_start` path left dirty). Reuses the `dcm*` helpers +
  `concurrentSentinelSeam` + `stubtest.Manifest/NewScript`; real git against temp repos; stub agents (no
  real model). DOES NOT touch production code, chain_test.go (S1+S2), git/*, docs/* (P1.M1.T2.S2), or
  PRD/tasks/snapshot. Sibling S2 (§5.2 legitimate-leftover "arbiter folds only T_start" + chain_test unit
  proof) is the clean fence: this S1 is empty-leftover (arbiter skipped) + exclusion + completeness ONLY.
---

## Goal

**Feature Goal**: Land the permanent regression net for FR-M1d (arbiter freeze parity) at the full
`Decompose()` integration level — the layer that proves, against real git on temp repos with stub agents,
that (a) a working-tree file written after `T_start` capture appears in NO commit of the run (including
across the arbiter gate), (b) the arbiter is SKIPPED when the frozen leftover `DiffTreeNames(tipTree,
tStart)` is empty (so a concurrent change cannot even provoke an arbiter run), and (c) after a
fully-successful run every `T_start` path landed in HEAD and the only live-tree dirt is the out-of-
`T_start` sentinel. These are the PRD §20.5 "must-cover" concurrency scenarios and the §20.2
"Start-of-run freeze" / "Arbiter freeze parity" / "`T_start` completeness" invariants, encoded as
deterministic in-process integration tests.

**Deliverable** (TEST-ONLY — one file modified):
1. **UPGRADE** `TestDecompose_ConcurrentChangeExclusion` (decompose_test.go:819-882) — strengthen its
   assertions to cover the arbiter (sentinel in no commit; sentinel remains untracked; exactly 2 commits
   ⇒ arbiter skipped); trim the message script 4→2; remove the stale loophole-admitting comments; rewrite
   the doc comment.
2. **ADD** `TestDecompose_TStartCompleteness` — the §20.2 "`T_start` completeness" invariant on a born
   repo (every frozen path landed; status clean except the out-of-`T_start` sentinel).

No new files; no production code; no helpers (reuses `dcm*` + `concurrentSentinelSeam`).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green;
the upgraded `TestDecompose_ConcurrentChangeExclusion` asserts sentinel-in-no-commit (whole-run `git log
--name-only`) + `?? sentinel.txt` remains + `dcmLogCount == 2` (arbiter skipped); the new
`TestDecompose_TStartCompleteness` asserts every frozen path ∈ `DiffTreeNames(baseTree, HEAD^{tree})` and
`dcmStatusPorcelain` == `?? sentinel.txt` only; `git diff --stat` shows ONLY `decompose_test.go` changed.

## User Persona

**Target User**: Stagehand contributors/maintainers — this is the regression net PRD §20.5 mandates
("Every bug found in the wild becomes a scenario here"). The end-user behavior it guards is "a concurrent
editor save / coding-agent write during a decompose run never lands in one of my commits, and never
provokes a spurious extra arbiter commit."

**Use Case**: A field report that "a file another tool wrote mid-decompose ended up in an arbiter commit"
or "stagehand created an extra commit for a concurrent change" becomes a scenario here. The two initial
scenarios cover the §18.5/§20.2 spec for the empty-leftover + exclusion + completeness cases.

**User Journey**: `go test -race ./internal/decompose/` → the upgraded exclusion test + the new
completeness test PASS, proving the freeze-safe arbiter (a) excludes a post-`T_start` sentinel from every
commit, (b) skips the arbiter when the frozen leftover is empty, and (c) commits every frozen path while
leaving only out-of-`T_start` dirt. A future regression that re-opens the arbiter loophole fails one of
these assertions.

**Pain Points Addressed**: Closes the silent-concurrency-loophole regression class for FR-M1d at the
integration layer (unit tests with stub agents cannot reach the full `Decompose()` → gate → arbiter path).
Without these tests, a refactor that re-introduces a live `StatusPorcelain` gate or live `AddAll` staging
would ship undetected.

## Why

- **PRD §20.5 (End-to-end scenario harness) is the mandate:** the concurrency invariants "are easy to
  specify, easy to regress, and — as repeated field discoveries have shown — easy to break silently (unit
  tests with stub agents cannot reach them)." The "must-cover" set explicitly includes: *"a file
  created/modified by a concurrent process mid-run → excluded from every commit, left in the working tree
  (FR-M1b/M1c), including across the arbiter gate — concurrent file + loop commits all of `T_start` →
  arbiter skips, file stays untracked (FR-M1d)."* That bullet IS the upgraded exclusion test.
- **PRD §20.2 (Property/invariant tests) names three invariants this task encodes:** "Start-of-run freeze"
  (sentinel in no commit, remains in the working tree), "Arbiter freeze parity" (the gate is
  `diff-names(tipTree, T_start)`, never `git status`; no arbiter commit when the frozen leftover is empty),
  and "`T_start` completeness" (every frozen path landed; live status non-empty only outside `T_start`).
- **PRD §9.14 FR-M1d is the feature under test:** the arbiter's gate/diff/staging are all frozen. The
  freeze-safe arbiter (P1.M1.T2.S1) LANDED the code; this task lands the acceptance proof. The contract's
  "TDD: fail before / pass after" note is historical (the fix is in) — these tests are the permanent net.
- **Closes P1.M1.T3.S1 (the empty-leftover half of the acceptance proof).** The sibling P1.M1.T3.S2 covers
  the legitimate-leftover half (arbiter RUNS, folds only `T_start`). Together they exhaust §20.2's
  "Arbiter freeze parity" paired cases. This task's fence: `DiffTreeNames(tipTree, tStart) == []` only.

## What

Two integration tests in `internal/decompose/decompose_test.go`, both driving the full `Decompose()` path
via the `dcm*` helpers with `stubtest` stub agents and real git on temp repos. No production code, no new
helpers.

**Test 1 — UPGRADE `TestDecompose_ConcurrentChangeExclusion`** (the existing empty-frozen-leftover
fixture): keep the existing "sentinel in no LOOP commit" diff-tree check; ADD (a) sentinel in NO commit
across the whole run (`dcmGitOut(t, repo, "log", "--name-only", "--format=")` does not contain
`sentinel.txt`); ADD (a) sentinel REMAINS untracked (`dcmStatusPorcelain` contains `?? sentinel.txt`);
ADD (b) exactly 2 commits (`dcmLogCount(t, repo) == 2` — arbiter skipped because the frozen leftover is
empty); trim the message script from 4 entries to 2 (the arbiter no longer calls for a message); remove
the stale loophole-admitting comments (L838, L880-882); rewrite the doc comment to state this is the
post-FR-M1d regression net.

**Test 2 — ADD `TestDecompose_TStartCompleteness`** (§20.2 "`T_start` completeness", contract 3c) on a
born repo: seed commit → `baseTree = HEAD^{tree}`; two dirty files (x.go, y.go) ⇒ `T_start` = {x.go,
y.go}; 2-concept planner; stagers cover both; `concurrentSentinelSeam` writes `sentinel.txt` unstaged.
Assert: every frozen path `∈ {x.go, y.go}` is in `git.New(repo).DiffTreeNames(ctx, baseTree, headTree)`
where `headTree = HEAD^{tree}` post-run; `dcmStatusPorcelain` shows ONLY `?? sentinel.txt`; (cross-check)
sentinel in no commit; `len(result.Commits) == 2`.

### Success Criteria

- [ ] `TestDecompose_ConcurrentChangeExclusion` upgraded: sentinel in no commit (whole-run `git log
      --name-only --format=`); `?? sentinel.txt` remains; `dcmLogCount == 2`; message script trimmed 4→2;
      stale comments removed; doc comment updated.
- [ ] `TestDecompose_TStartCompleteness` added (born repo): every frozen path `⊆ DiffTreeNames(baseTree,
      HEAD^{tree})`; `dcmStatusPorcelain == "?? sentinel.txt"` (only); sentinel in no commit; 2 commits.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] `git diff --stat` shows ONLY `internal/decompose/decompose_test.go`.
- [ ] NO legitimate-leftover / arbiter-RUNS scenario added (that is S2's §5.2 territory — D6).
- [ ] NO production code, chain_test.go, git/*, docs/*, or PRD/tasks/snapshot touched.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim current body of `TestDecompose_ConcurrentChangeExclusion`
(decompose_test.go:819-882, including the loophole-admitting comments at L838/L880-882 and the 4-entry
message script), the verbatim `concurrentSentinelSeam` semantics (L795), the full `dcm*` helper inventory
with line numbers, the exact new assertions with the exact git oracles (`dcmLogCount`, `dcmStatusPorcelain`,
`dcmGitOut(... "log" "--name-only" "--format=")`), the completeness check via `git.New(repo).DiffTreeNames`
(already imported/used), the §20.5/§20.2 PRD mapping, and the hard S2 fence. The freeze-safe arbiter is
confirmed LANDED (gate/decompose.go:223, resolveArbiter/chain.go:50). No inference required.

### Documentation & References

```yaml
# MUST READ — the feature spec, the acceptance-proof plan, and this task's research
- file: PRD.md
  why: "§9.14 FR-M1d (the arbiter gate/diff/staging are frozen — gate = diff-names(tipTree, T_start), never
        git status; arbiter skipped when frozen leftover empty). §13.6.5 (the arbiter; 'If the frozen
        leftover is empty after the loop, the arbiter does not run'). §20.2 ('Start-of-run freeze',
        'Arbiter freeze parity', 'T_start completeness' invariants — the exact assertions). §20.5 (the
        e2e harness mandate + the 'concurrent file + loop commits all of T_start → arbiter skips' bullet)."
  critical: "§20.2's three invariants ARE the assertion spec. §20.5's 'arbiter skips' bullet IS the
             upgraded exclusion test. FR-M1d is WHY the arbiter no longer sweeps the sentinel."

- docfile: plan/008_82253c999440/docs/architecture/arbiter_freeze_parity.md
  why: "§5.1 (upgrade TestDecompose_ConcurrentChangeExclusion with the (a) sentinel-in-no-commit /
        (b) sentinel-remains / (c) no-arbiter-commit-when-empty assertions — the fixture is the
        empty-frozen-leftover case). §5.3 (T_start completeness: DiffTreeNames(baseTree, tStart) ⊆
        DiffTreeNames(baseTree, HEAD^{tree}); live status non-empty only outside T_start). §5.2 (the
        SIBLING S2 case — arbiter folds only T_start content; legitimate leftover — NOT this task)."
  critical: "§5.1 + §5.3 ARE this task's spec. §5.2 is the S2 fence (do NOT duplicate). §1.1/§1.5 pin
             the existing test's loophole-admitting comments (L838/L880-882) — the upgrade target."

- docfile: plan/008_82253c999440/P1M1T3S1/research/concurrent_arbiter_gate_tests.md
  why: "THIS subtask's research: §1 the scope/boundary table (S1 vs S2); §2 the verbatim current
        TestDecompose_ConcurrentChangeExclusion state + the (a)+(b) merge rationale; §3 the new
        TestDecompose_TStartCompleteness design + the line-395 inline-assertion note; §4 the S2 fence;
        §5 the reusable-helper inventory; §6 decisions D1–D7. READ THIS FIRST."
  critical: "§2 (the merge — the existing fixture IS the empty-leftover case) and §4 (the S2 fence:
             no legitimate-leftover scenario) are the two decisions most likely to be gotten wrong. §3
             explains why the line-395 assertion is NOT deleted (it's an inline shortcut check, not a
             standalone test)."

- docfile: plan/008_82253c999440/P1M1T2S1/PRP.md
  why: "The CONTRACT for the freeze-safe arbiter under test (LANDED): gate = DiffTreeNames(tipTree, tStart)
        (decompose.go:223); resolveArbiter 7-arg (chain.go:50); paths A/B treePrime:=tStart; path C
        OverlayTreePaths; ReadTree(tStart) syncs. Confirms the behavior these tests assert is the shipped
        behavior."
  critical: "The tests PASS against this landed code. Do NOT attempt to demonstrate a pre-fix failure
             (the pre-fix code is gone). These are the permanent regression net (D7)."

# The edit site + the reusable helpers (all in decompose_test.go)
- file: internal/decompose/decompose_test.go
  why: "EDIT (the only file). TestDecompose_ConcurrentChangeExclusion at :819-882 (upgrade); the new
        TestDecompose_TStartCompleteness (add near the exclusion test — they share the seam/concern).
        concurrentSentinelSeam at :795 (reuse). dcm* helpers at :28-227 (reuse — do NOT redeclare)."
  pattern: "Each dcm test: bin:=stubtest.Build(t); repo:=t.TempDir(); dcmInitRepo; plannerJSON via
            dcmPlannerManifest; messageM via dcmMessageScriptManifest; stagerM via tooledStubManifest;
            arbiterM via dcmArbiterManifest; roles:=RoleManifests{...}; deps:=dcmDeps(t,repo,roles);
            deps.stager = <seam>; result,err:=Decompose(ctx,deps); assert via dcm* oracles."
  gotcha: "concurrentSentinelSeam(t,repo,conceptFiles,sentinel) writes sentinel UNSTAGED on the FIRST
           concept only (post-freeze ⇒ excluded from T_start). The conceptFiles map keys MUST match the
           planner's concept Titles exactly (e.g. 'add a' / 'add b') or the seam stages nothing."

# Read-only refs (do NOT edit)
- file: internal/decompose/decompose.go
  why: "READ-ONLY. The gate at :213-230 (leftoverPaths := DiffTreeNames(tipTree, tStart); arbiter runs iff
        len(leftoverPaths)>0 && len(commits)>0). DecomposeResult{Commits []CommitResult, Amended int} at :72
        — Amended is 0 BOTH when the arbiter did not run AND when it ran+null, so dcmLogCount (not Amended)
        is the 'arbiter skipped' proof."
- file: internal/decompose/chain.go
  why: "READ-ONLY. resolveArbiter 7-arg (:50); the three paths (resolveNewCommit/resolveTipAmend/
        resolveMidChain) are tree-only-from-T_start. Confirms no arbiter path can sweep a live-tree file."
- file: internal/git/git.go
  why: "READ-ONLY. DiffTreeNames(ctx, treeA, treeB) at :288 (the completeness superset check + the gate's
        primitive). EmptyTreeSHA at :641 (the unborn-repo baseTree). git.New(repo) constructs the Git used
        by dcmDeps and available directly in the test."

# External references
- url: https://git-scm.com/docs/git-log#Documentation/git-log.txt---name-only
  why: "`git log --name-only --format=` prints only the file names touched per commit (the empty --format
        suppresses the subject line). The whole-run sentinel-exclusion oracle (reused from
        TestDecompose_StagerFreezeViolation:781)."
  critical: "This is the 'sentinel in NO commit (including arbiter)' check. --format= (empty) is what
             leaves only file-name lines."
- url: https://git-scm.com/docs/git-diff-tree#Documentation/git-diff-tree.txt---name-only
  why: "`git diff-tree --no-commit-id --name-only -r <treeA> <treeB>` lists paths differing between two
        trees — the oracle for the per-loop-commit check AND the DiffTreeNames completeness basis."
```

### Current Codebase Tree (this task's scope)

```bash
stagehand/
└── internal/decompose/
    └── decompose_test.go   # EDIT (only file): upgrade TestDecompose_ConcurrentChangeExclusion (:819); add TestDecompose_TStartCompleteness
# (chain.go/decompose.go/arbiter.go = the LANDED freeze-safe arbiter — read-only; chain_test.go = S1+S2 territory; git/* = read-only)
```

### Desired Codebase Tree After This Subtask

```bash
stagehand/
└── internal/decompose/
    └── decompose_test.go   # one test upgraded, one test added (both reuse dcm* + concurrentSentinelSeam)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/decompose/decompose_test.go` | MODIFY | Upgrade `TestDecompose_ConcurrentChangeExclusion` (sentinel-in-no-commit + remains + exactly-2-commits + trim script + comment cleanup); add `TestDecompose_TStartCompleteness` (every frozen path landed + status-clean-except-sentinel). |

**Explicitly NOT touched**: `internal/decompose/chain.go`, `decompose.go`, `arbiter.go` (the LANDED
freeze-safe arbiter — production), `internal/decompose/chain_test.go` (S1's canonical resolveArbiter
tests + S2's §5.2 unit proof), `internal/decompose/{planner,message,stager,roles}.go` (unaffected),
`internal/git/*` (read-only oracles), `docs/*` (P1.M1.T2.S2 Mode A arbiter narrative — in flight),
`README.md` (P3.M1.T1.S1), `PRD.md`, `tasks.json`, `prd_snapshot.md`, anything under `plan/`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — the existing fixture IS the empty-frozen-leftover case; merge (a)+(b)). The current
// TestDecompose_ConcurrentChangeExclusion writes a.txt+b.txt BEFORE the run (both in T_start) and the seam
// stages BOTH, so tipTree == T_start ⇒ DiffTreeNames(tipTree, tStart) == [] ⇒ arbiter SKIPPED under the
// landed fix. So contract (b) "exactly 2 commits / arbiter skipped" is a natural strengthening of (a)'s
// upgrade — do NOT write a redundant second test with the same fixture. Add the (b) assertion
// (dcmLogCount == 2) to the upgraded (a) test. (D1.)

// CRITICAL (G2 — dcmLogCount, NOT result.Amended, proves "arbiter skipped"). DecomposeResult.Amended is 0
// BOTH when the arbiter did not run AND when it ran with a null target (decompose.go:74). A null-target
// arbiter would create a 3rd commit. So the authoritative "arbiter did not run" proof is
// dcmLogCount(t, repo) == 2 (the two loop commits; no N+1 arbiter commit). Do NOT assert Amended == 0 as
// the skip proof. (D2.)

// CRITICAL (G3 — trim the message script 4→2; the arbiter no longer calls for a message). Post-fix the
// arbiter does not run, so the message script's entries 2/3 ("feat: add sentinel" ×2) are never consumed.
// Trim to []string{"feat: add a", "feat: add b"} (loop only) so the test's intent is explicit. KEEP a
// stub Arbiter manifest (the RoleManifests struct requires all four roles) — it is never invoked. (D3.)

// CRITICAL (G4 — remove the stale loophole-admitting comments). The comments at decompose_test.go:838
// ("the arbiter picks up the sentinel via AddAll (not yet frozen — P3.M2.T1.S1)") and L880-882 ("the
// arbiter's STAGING picks up sentinel.txt… we verify only the loop commits") describe the PRE-FIX
// behavior. They are now FALSE and must be removed/rewritten — leaving them misrepresents the test. The
// rewritten doc comment states this is the post-FR-M1d regression net (arbiter frozen; sentinel cannot
// cross the gate).

// CRITICAL (G5 — the S2 fence: NO legitimate-leftover / arbiter-RUNS scenario). S2 (§5.2) owns the case
// where a REAL frozen leftover exists (a stager no-ops) → the arbiter RUNS → its tree is exactly T_start.
// This S1 is EXCLUSIVELY the empty-leftover case (arbiter SKIPPED) + exclusion + completeness. Do NOT add
// any test where DiffTreeNames(tipTree, tStart) != [] with an arbiter run — that collides with S2. (D6.)

// GOTCHA (G6 — the sentinel is UNSTAGED, never staged). concurrentSentinelSeam writes sentinel.txt via
// os.WriteFile (NO git add) on the first concept. STAGING it would trip ErrFreezeViolation
// (TestDecompose_StagerFreezeViolation:736 covers that path). This test's sentinel MUST stay unstaged so
// the run succeeds and the freeze-exclusion property (not the violation) is exercised. Reuse
// concurrentSentinelSeam as-is.

// GOTCHA (G7 — conceptFiles map keys MUST match planner concept Titles). concurrentSentinelSeam and
// dcmStagerSeam look up conceptFiles[concept.Title]. The planner JSON's "title" fields ("add a", "add b")
// must EXACTLY match the map keys, or the seam stages nothing and the run breaks. Keep the titles
// consistent with the existing test.

// GOTCHA (G8 — TestDecompose_TStartCompleteness uses a BORN repo for a meaningful baseTree). The
// completeness invariant is DiffTreeNames(baseTree, HEAD^{tree}) ⊇ DiffTreeNames(baseTree, tStart). On an
// unborn repo baseTree == EmptyTreeSHA (still valid, but the born-repo variant with a real baseTree is the
// clearer §20.2 proof). Seed an "initial" commit, capture baseTree = HEAD^{tree} BEFORE the run, then
// assert each frozen path ∈ git.New(repo).DiffTreeNames(ctx, baseTree, headTree). (D4.)

// GOTCHA (G9 — use git.New(repo).DiffTreeNames for the completeness check, not a hand-rolled git diff).
// DiffTreeNames is the SAME primitive the production gate uses (decompose.go:223) — already imported in
// decompose_test.go (L18) and used via git.New at L142/153/232. Calling it directly pins the invariant
// with the identical oracle. Alternatively the git CLI oracle `git diff-tree --no-commit-id --name-only
// -r baseTree headTree` works; prefer the in-process primitive for symmetry with the gate. (D5.)

// GOTCHA (G10 — the line-395 "loop-index-cleanliness" assertion is NOT deleted). decompose_test.go:395 is
// an INLINE assertion (status == "" after a single-shortcut run) inside a shortcut test — NOT a standalone
// "Loop index cleanliness" test. The §20.2 rename ("T_start completeness" replaces "Loop index
// cleanliness") is narrative; that inline shortcut assertion is a valid check and stays. This task ADDS
// the decompose-loop completeness proof (which that assertion does not cover). (D4.)

// GOTCHA (G11 — the tests PASS against the landed code; do NOT chase a "before" failure). The freeze-safe
// arbiter (P1.M1.T2.S1) is LANDED. The contract's "TDD: fail before / pass after" note is historical
// (the pre-fix code is gone). These tests are the PERMANENT regression net — write them to encode the
// post-fix invariant and confirm they pass against HEAD. (D7.)

// GOTCHA (G12 — this is TEST-ONLY; the build/test gate is the whole deliverable). No production file is in
// scope. `go test -race ./...` must stay green (it is at baseline). The two named tests are the
// `go test -race ./internal/decompose/ -run 'TestDecompose_ConcurrentChangeExclusion|TestDecompose_TStartCompleteness'`
// gate. `git diff --stat` must show ONLY decompose_test.go.
```

## Implementation Blueprint

### Data models and structure

None. The tests consume existing types: `DecomposeResult{Commits []CommitResult, Amended int}`
(decompose.go:72), the `dcm*` helpers, `concurrentSentinelSeam`, and `git.New(repo).DiffTreeNames`. No
new production types, no new helpers.

### The upgraded `TestDecompose_ConcurrentChangeExclusion` (exact assertion additions)

The fixture is UNCHANGED (unborn repo; a.txt + b.txt; 2-concept planner; `concurrentSentinelSeam` with
`sentinel.txt`). The changes are: trim the message script, remove stale comments, and ADD three
assertions. The shape (pseudocode, mirroring the existing test's style):

```go
func TestDecompose_ConcurrentChangeExclusion(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	// NO initial commit (unborn repo) — mirrors TestDecompose_AutoMultiCommit_HappyPath.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"add a","description":"a.txt"},{"title":"add b","description":"b.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// LOOP-ONLY message responses: under FR-M1d the arbiter does NOT run (the frozen leftover is empty —
	// the stagers cover all of T_start), so no arbiter→resolveNewCommit message call occurs.
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b"})

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`) // never invoked under FR-M1d (gate skips)
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	deps.stager = concurrentSentinelSeam(t, repo,
		map[string][]string{"add a": {"a.txt"}, "add b": {"b.txt"}},
		"sentinel.txt",
	)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(concurrent exclusion): %v", err)
	}
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want exactly 2 (loop commits; arbiter must be skipped)", len(result.Commits))
	}

	// (a)+(b) The arbiter is SKIPPED: the frozen leftover DiffTreeNames(tipTree, tStart) is empty (the
	// stagers covered all of T_start), so FR-M1d's frozen gate does not run the arbiter. Exactly 2 commits.
	if got := dcmLogCount(t, repo); got != 2 {
		t.Errorf("commit count = %d, want exactly 2 (arbiter skipped — empty frozen leftover; FR-M1d)", got)
	}

	// (existing) sentinel in no LOOP commit's diff-tree (FR-M1b/M1c freeze).
	for _, sha := range []string{/* the two loop commit SHAs from dcmLogOneline */} {
		treeOut := dcmGitOut(t, repo, "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
		if strings.Contains(treeOut, "sentinel.txt") {
			t.Errorf("commit %s contains sentinel.txt — freeze should exclude post-freeze changes\ntree: %s", sha, treeOut)
		}
	}

	// (a) NEW: sentinel in NO commit across the WHOLE run — including any arbiter commit (FR-M1d). The
	// arbiter's gate/diff/staging are frozen, so a post-T_start file cannot cross the gate.
	logNames := dcmGitOut(t, repo, "log", "--name-only", "--format=")
	if strings.Contains(logNames, "sentinel.txt") {
		t.Errorf("sentinel.txt appears in a commit (incl. arbiter) — FR-M1d freeze should exclude it:\n%s", logNames)
	}

	// (a) NEW: sentinel REMAINS in the working tree, untracked (FR-M1b/M1d). The run left it untouched.
	status := dcmStatusPorcelain(t, repo)
	if !strings.Contains(status, "?? sentinel.txt") {
		t.Errorf("status = %q, want it to contain '?? sentinel.txt' (concurrent change left untouched)", status)
	}
}
```

> The implementer fills the loop-commit-SHA slice from `dcmLogOneline(t, repo)` (split the reversed log;
> the first two entries are the loop commits) exactly as the existing test does (L859-878). The three
> NEW assertions (the `dcmLogCount == 2` check, the whole-run `git log --name-only` check, the
> `?? sentinel.txt` check) are the FR-M1d upgrade. Remove the comments at L838 and L880-882 (now false).

### The new `TestDecompose_TStartCompleteness` (exact shape)

```go
// TestDecompose_TStartCompleteness (PRD §20.2 "T_start completeness" invariant, FR-M1d contract 3c):
// after a fully-successful decompose run, EVERY T_start path landed in HEAD, and the live working tree
// is non-empty ONLY from paths OUTSIDE T_start (the post-freeze sentinel), which are intentionally left
// unstaged. Born repo so baseTree is a real tree (makes the DiffTreeNames superset check meaningful).
func TestDecompose_TStartCompleteness(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial") // BORN repo — real baseTree
	dcmWriteFile(t, repo, "x.go", "package x\n")
	dcmWriteFile(t, repo, "y.go", "package y\n")
	baseTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}") // capture BEFORE the run

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"add x","description":"x.go"},{"title":"add y","description":"y.go"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add x", "feat: add y"})
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`) // arbiter skipped (stagers cover all T_start)
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	deps.stager = concurrentSentinelSeam(t, repo,
		map[string][]string{"add x": {"x.go"}, "add y": {"y.go"}},
		"sentinel.txt", // post-freeze, OUTSIDE T_start
	)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(completeness): %v", err)
	}
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2", len(result.Commits))
	}

	// (c) T_start completeness: every frozen path landed in HEAD. DiffTreeNames(baseTree, HEAD^{tree})
	// ⊇ DiffTreeNames(baseTree, tStart) == {x.go, y.go}. Use the SAME primitive the production gate uses.
	headTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	g := git.New(repo)
	landed, err := g.DiffTreeNames(context.Background(), baseTree, headTree)
	if err != nil {
		t.Fatalf("DiffTreeNames(baseTree, headTree): %v", err)
	}
	landedSet := map[string]bool{}
	for _, p := range landed {
		landedSet[p] = true
	}
	for _, frozen := range []string{"x.go", "y.go"} {
		if !landedSet[frozen] {
			t.Errorf("frozen path %q did NOT land in HEAD — T_start completeness violated (landed=%v)", frozen, landed)
		}
	}

	// (c) Live status is non-empty ONLY from paths OUTSIDE T_start: the sole entry is the sentinel.
	if status := dcmStatusPorcelain(t, repo); !strings.Contains(status, "?? sentinel.txt") || strings.TrimSpace(status) != "?? sentinel.txt" {
		t.Errorf("status = %q, want exactly '?? sentinel.txt' (only out-of-T_start dirt remains)", status)
	}

	// Cross-check (FR-M1d): the sentinel is in no commit; exactly 2 commits (initial + 2 loop = 3 total).
	if got := dcmLogCount(t, repo); got != 3 {
		t.Errorf("commit count = %d, want 3 (initial + 2 loop; arbiter skipped)", got)
	}
	if logNames := dcmGitOut(t, repo, "log", "--name-only", "--format="); strings.Contains(logNames, "sentinel.txt") {
		t.Errorf("sentinel.txt appears in a commit — FR-M1d freeze should exclude it:\n%s", logNames)
	}
}
```

> `git.New(repo).DiffTreeNames` is the in-process oracle (same primitive as the production gate). The
> `landedSet` membership check is the operational form of §20.2's superset. The `dcmStatusPorcelain ==
// "?? sentinel.txt"` (exactly) check proves no `T_start` path is left dirty.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: UPGRADE TestDecompose_ConcurrentChangeExclusion (decompose_test.go:819-882)
  - OPEN internal/decompose/decompose_test.go; locate TestDecompose_ConcurrentChangeExclusion (:819).
  - EDIT the message script line: trim []string{"feat: add a","feat: add b","feat: add sentinel","feat: add sentinel"}
    → []string{"feat: add a","feat: add b"} (loop only; arbiter no longer runs — D3/G3).
  - REPLACE the doc comment + the inline comments at :838 and :880-882: remove the loophole-admitting
    language ("the arbiter picks up the sentinel via AddAll…"; "we verify only the loop commits"). State
    this is the post-FR-M1d regression net: the arbiter is frozen, the sentinel cannot cross the gate,
    the run produces exactly 2 commits. (G4.)
  - CHANGE `if len(result.Commits) < 2` → `if len(result.Commits) != 2` (exactly 2; arbiter skipped).
  - ADD after the loop-commit checks (the three NEW assertions from §"The upgraded test"):
      (a) whole-run sentinel exclusion: logNames := dcmGitOut(t,repo,"log","--name-only","--format="); !contains sentinel.txt
      (a) sentinel remains: status := dcmStatusPorcelain(t,repo); strings.Contains(status, "?? sentinel.txt")
      (b) arbiter skipped: dcmLogCount(t,repo) == 2
  - KEEP the existing loop-commit diff-tree sentinel check (the FR-M1b/M1c proof) — just stop gating on
    `loopCount` only; the whole-run check subsumes it but keep both for layered diagnostics.
  - DO NOT: change the fixture (repo/files/planner/seam); add a legitimate-leftover case (S2 — G5); touch
    any other test or production file.
  - VERIFY: go test -race ./internal/decompose/ -run TestDecompose_ConcurrentChangeExclusion → PASS.

Task 2: ADD TestDecompose_TStartCompleteness (decompose_test.go, near the exclusion test)
  - ADD the function verbatim from §"The new test" (born repo; x.go+y.go; baseTree capture;
    concurrentSentinelSeam with sentinel.txt; DiffTreeNames superset check via git.New(repo);
    dcmStatusPorcelain == "?? sentinel.txt"; dcmLogCount == 3; sentinel-in-no-commit cross-check).
  - PLACE it immediately AFTER TestDecompose_ConcurrentChangeExclusion (shared concern: freeze + sentinel).
  - DO NOT redeclare dcm*/concurrentSentinelSeam (reuse); do NOT add a legitimate-leftover case (G5).
  - VERIFY: go test -race ./internal/decompose/ -run TestDecompose_TStartCompleteness → PASS.

Task 3: VALIDATE — full gate set + scope discipline
  - RUN: gofmt -w internal/decompose/decompose_test.go ; gofmt -l .   (expect empty after -w)
  - RUN: go vet ./...                                                   (expect exit 0)
  - RUN: go build ./...                                                 (expect exit 0)
  - RUN: go test -race ./internal/decompose/ -run 'TestDecompose_ConcurrentChangeExclusion|TestDecompose_TStartCompleteness' -v
        → expect both PASS.
  - RUN: go test -race ./...                                            (expect all packages green)
  - RUN: git diff --stat -- internal/decompose/decompose_test.go        → expect ONLY this file.
  - RUN: git diff --stat -- internal/decompose/chain.go internal/decompose/decompose.go internal/decompose/arbiter.go
         internal/decompose/chain_test.go internal/git/ docs/ README.md → expect EMPTY (untouched).
  - RUN (S2 fence): grep -n 'leftover' internal/decompose/decompose_test.go | grep -i 'legitimate\|arbiter.*run\|non-empty.*leftover'
        → expect NONE (no legitimate-leftover/arbiter-runs scenario — that is S2's §5.2).
```

### Implementation Patterns & Key Details

```go
// === The "arbiter skipped" proof is dcmLogCount, NOT result.Amended (G2) ===
// DecomposeResult.Amended == 0 both when the arbiter did not run AND when it ran with target==nil
// (a null-target arbiter creates an (N+1)-th commit but Amended stays 0). So Amended can't distinguish
// "skipped" from "ran+null". dcmLogCount == 2 (unborn) or == 3 (born + seed) CAN: a null-target arbiter
// would add a commit. Assert the COUNT, not Amended.

// === The whole-run sentinel oracle (reused from TestDecompose_StagerFreezeViolation:781) ===
// logNames := dcmGitOut(t, repo, "log", "--name-only", "--format=")
// `--format=` (empty) suppresses the subject line, leaving only file names (one block per commit).
// `strings.Contains(logNames, "sentinel.txt")` ⇒ the sentinel is in SOME commit (incl. arbiter) ⇒ FAIL.
// This is strictly stronger than the per-loop-commit diff-tree check (it also covers any arbiter commit).

// === The completeness oracle = the production gate's primitive (G9) ===
// g := git.New(repo); landed, _ := g.DiffTreeNames(ctx, baseTree, headTree)
// DiffTreeNames is EXACTLY what decompose.go:223 calls for the gate. Asserting the frozen set ⊆ landed
// pins the same invariant the gate relies on. headTree = `git rev-parse HEAD^{tree}` post-run.

// === Why concurrentSentinelSeam writes the sentinel UNSTAGED (G6) ===
// The seam uses os.WriteFile (NO `git add`) on the first concept. The sentinel is post-freeze (the seam
// runs during the loop, after FreezeWorkingTree captured T_start) ⇒ NOT in T_start ⇒ excluded from every
// commit + left untracked. STAGING it would trip ErrFreezeViolation (TestDecompose_StagerFreezeViolation
// covers that). Reuse the seam as-is; do not stage the sentinel.

// === Why the message script is trimmed 4→2 (G3) ===
// Pre-fix, the arbiter ran and called resolveNewCommit → generateMessage (entries 2/3). Post-fix, the
// frozen gate skips the arbiter → no arbiter message call. Trimming to the 2 loop entries makes the
// test's intent explicit and would FAIL loudly (script exhausted) if a regression re-ran the arbiter.
```

### Integration Points

```yaml
TEST FILE (internal/decompose/decompose_test.go):
  - UPGRADE: TestDecompose_ConcurrentChangeExclusion (:819) — +3 assertions, -stale comments, message script 4→2
  - ADD: TestDecompose_TStartCompleteness — born repo, DiffTreeNames superset, status-clean-except-sentinel

CONSUMED (READ-ONLY — the landed freeze-safe arbiter under test):
  - internal/decompose/decompose.go: gate (:213-230) DiffTreeNames(tipTree, tStart); DecomposeResult (:72)
  - internal/decompose/chain.go: resolveArbiter 7-arg (:50); 3 paths tree-only-from-T_start
  - internal/git/git.go: DiffTreeNames (:288), EmptyTreeSHA (:641), git.New

REUSED HELPERS (decompose_test.go — do NOT redeclare):
  - dcm*: InitRepo/WriteFile/StageFile/CommitRaw/RunGit/GitOut/HeadSHA/LogOneline/LogCount/StatusPorcelain
  - dcm*: PlannerManifest/MessageScriptManifest/ArbiterManifest/Deps/AllRoles
  - concurrentSentinelSeam (:795), tooledStubManifest, stubtest.Build/Manifest/NewScript

GATE: go test -race ./... → GREEN ; git diff --stat → ONLY decompose_test.go

NO-TOUCH (explicitly — owned by siblings / out of scope):
  - internal/decompose/{chain,decompose,arbiter,planner,message,stager,roles}.go  # production (landed)
  - internal/decompose/chain_test.go    # S1 canonical resolveArbiter tests + S2's §5.2 unit proof
  - internal/git/*                      # read-only oracles
  - docs/* (P1.M1.T2.S2 Mode A arbiter narrative — in flight); README.md (P3.M1.T1.S1)
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOK (informational — owned by the SIBLING, NOT this task):
  - P1.M1.T3.S2 (§5.2): the legitimate-leftover "arbiter folds only T_start" paired integration case +
    the chain_test.go unit proof (resolveArbiter direct call with a post-freeze sentinel). This S1 is the
    complementary empty-leftover case; do NOT overlap.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand

gofmt -w internal/decompose/decompose_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/decompose/...  # Expected: exit 0
go build ./...                   # Expected: exit 0

# Expected: zero errors. The build confirms no helper was redeclared and the new test compiles.
```

### Level 2: The Two Named Tests (the deliverable)

```bash
cd /home/dustin/projects/stagehand

go test -race ./internal/decompose/ -v -run 'TestDecompose_ConcurrentChangeExclusion|TestDecompose_TStartCompleteness'
# Expected: BOTH PASS, exit 0.
#   TestDecompose_ConcurrentChangeExclusion:
#     - sentinel in no commit (whole-run git log --name-only)         [FR-M1d, contract 3a]
#     - "?? sentinel.txt" remains in the working tree                 [FR-M1b/M1d, contract 3a]
#     - dcmLogCount == 2 (arbiter SKIPPED — empty frozen leftover)    [FR-M1d, contract 3b]
#   TestDecompose_TStartCompleteness:
#     - every frozen path (x.go, y.go) ∈ DiffTreeNames(baseTree, HEAD^{tree})  [§20.2, contract 3c]
#     - dcmStatusPorcelain == "?? sentinel.txt" (only out-of-T_start dirt)     [§20.2, contract 3c]
#     - sentinel in no commit; dcmLogCount == 3 (initial + 2 loop)             [FR-M1d cross-check]
```

### Level 3: Whole-Repository Regression + Scope Discipline

```bash
cd /home/dustin/projects/stagehand

go test -race ./...              # Expected: ALL packages green (the new/edited tests are the only change)
go vet ./...                     # Expected: exit 0

# Scope: ONLY decompose_test.go changed.
git diff --stat -- internal/decompose/decompose_test.go
# Expected: internal/decompose/decompose_test.go | <n> +-<m>

git diff --stat -- internal/decompose/chain.go internal/decompose/decompose.go internal/decompose/arbiter.go \
                   internal/decompose/chain_test.go internal/git/ docs/ README.md
# Expected: EMPTY (production code, chain_test.go, git/*, docs, README all untouched).

# S2 fence: NO legitimate-leftover / arbiter-RUNS scenario leaked in.
git grep -nE 'legitimate.*leftover|arbiter.*runs|non-empty.*leftover' internal/decompose/decompose_test.go || true
# Expected: no matches (the legitimate-leftover case is S2's §5.2 territory).
```

### Level 4: Behavioral Smoke (manual repro of the freeze property)

```bash
cd /home/dustin/projects/stagehand

# The two tests ARE the smoke test (they drive the full Decompose() against real git on a temp repo with
# stub agents). For a manual cross-check, build a tiny repo and confirm a post-freeze untracked file is
# invisible to DiffTreeNames(tipTree, tStart):
tmp=$(mktemp -d); cd "$tmp"; git init -q; git config user.name T; git config user.email t@t
printf 'a\n' > a.go; printf 'b\n' > b.go
base=$(git rev-parse HEAD^{tree} 2>/dev/null || printf '4b825dc642cb6eb9a060e54bf8d69288fbee4904')
# (T_start capture is internal to Decompose; this smoke just confirms the oracle semantics:)
git add -A; tstart=$(git write-tree); git rm --cached -q a.go b.go
tip=$(git write-tree)
echo "DiffTreeNames(tip, tStart) (should be a.go, b.go):"; git diff-tree --no-commit-id --name-only -r "$tip" "$tstart"
printf 'sentinel\n' > sentinel.txt  # post-freeze, untracked
echo "status (should show ?? sentinel.txt):"; git status --porcelain
cd /; rm -rf "$tmp"
# This confirms the oracle mechanics the tests rely on (DiffTreeNames excludes the untracked sentinel;
# status shows only the out-of-T_start file).
```

## Final Validation Checklist

### Technical Validation

- [ ] `gofmt -l .` reports nothing.
- [ ] `go vet ./...` exits 0.
- [ ] `go build ./...` exits 0.
- [ ] `go test -race ./...` — all packages green.
- [ ] `go test -race ./internal/decompose/ -run 'TestDecompose_ConcurrentChangeExclusion|TestDecompose_TStartCompleteness' -v` → both PASS.

### Feature Validation

- [ ] `TestDecompose_ConcurrentChangeExclusion` (upgraded): sentinel in NO commit (whole-run `git log
      --name-only --format=`); `?? sentinel.txt` remains; `dcmLogCount == 2` (arbiter skipped); message
      script trimmed 4→2; stale comments removed.
- [ ] `TestDecompose_TStartCompleteness` (new): every frozen path ∈ `git.New(repo).DiffTreeNames(ctx,
      baseTree, headTree)`; `dcmStatusPorcelain == "?? sentinel.txt"` (only); sentinel in no commit;
      `dcmLogCount == 3` (seed + 2 loop).

### Scope Discipline Validation

- [ ] `git diff --stat` shows ONLY `internal/decompose/decompose_test.go`.
- [ ] Did NOT edit production code (`chain.go`, `decompose.go`, `arbiter.go`, `planner/message/stager/roles.go`).
- [ ] Did NOT edit `chain_test.go` (S1 canonical tests + S2's §5.2 unit proof) or `internal/git/*`.
- [ ] Did NOT edit `docs/*` (P1.M1.T2.S2) or `README.md` (P3.M1.T1.S1).
- [ ] Did NOT add any legitimate-leftover / arbiter-RUNS scenario (S2's §5.2 territory — G5/D6).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Reuses `dcm*` + `concurrentSentinelSeam` (no redeclarations).
- [ ] The upgraded test's doc comment reflects the post-FR-M1d reality (no loophole-admitting language).
- [ ] The completeness check uses `git.New(repo).DiffTreeNames` (the production gate's primitive).
- [ ] "Arbiter skipped" is proven via `dcmLogCount`, not `result.Amended` (G2).

---

## Anti-Patterns to Avoid

- ❌ Don't add a redundant separate test for contract (b). The existing `TestDecompose_ConcurrentChangeExclusion`
  fixture IS the empty-frozen-leftover case (stagers cover all of T_start). Merge (b)'s "exactly 2 commits"
  assertion into the (a) upgrade. A second test with an identical fixture is churn (D1/G1).
- ❌ Don't prove "arbiter skipped" with `result.Amended == 0`. `Amended` is 0 for BOTH "arbiter did not
  run" and "arbiter ran + null target" (decompose.go:74). Use `dcmLogCount == 2` — a null-target arbiter
  would create a 3rd commit (G2/D2).
- ❌ Don't leave the message script at 4 entries. Post-fix the arbiter doesn't run, so entries 2/3 are
  never consumed. Trim to 2 (loop only) so the test's intent is explicit and a regression that re-runs
  the arbiter fails loudly (script exhausted) (G3/D3).
- ❌ Don't leave the stale loophole-admitting comments (L838, L880-882). They describe PRE-FIX behavior
  and now misrepresent the test. Remove/rewrite them (G4).
- ❌ Don't add a legitimate-leftover / arbiter-RUNS scenario. That is S2's (§5.2) exclusive territory
  (a stager no-ops → real leftover → arbiter runs → tree == T_start). This S1 is empty-leftover
  (arbiter skipped) + exclusion + completeness ONLY (G5/D6).
- ❌ Don't STAGE the sentinel. `concurrentSentinelSeam` writes it UNSTAGED (os.WriteFile, no `git add`).
  Staging it trips `ErrFreezeViolation` (covered by `TestDecompose_StagerFreezeViolation`). Reuse the
  seam as-is (G6).
- ❌ Don't mismatch `conceptFiles` map keys and planner Titles. The seam looks up `conceptFiles[concept.Title]`;
  the JSON "title" fields must EXACTLY match the map keys or nothing stages (G7).
- ❌ Don't delete the line-395 "loop-index-cleanliness" inline assertion. It's a valid shortcut-path
  check inside a shortcut test, not a standalone test. The §20.2 rename is narrative; ADD the decompose-
  loop completeness proof separately (G10/D4).
- ❌ Don't chase a "pre-fix failure" demonstration. The freeze-safe arbiter (P1.M1.T2.S1) is LANDED; the
  pre-fix code is gone. These tests are the PERMANENT regression net — write them to pass against HEAD
  (G11/D7).
- ❌ Don't edit production code, chain_test.go, git/*, docs/*, README.md, PRD.md, tasks.json,
  prd_snapshot.md, or plan/*.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a self-contained, test-only change to one file, backed by an exceptionally detailed
architecture doc (`arbiter_freeze_parity.md` §5.1/§5.3) that IS the assertion spec, and the freeze-safe
arbiter under test is CONFIRMED LANDED on disk (gate/decompose.go:223, resolveArbiter/chain.go:50, paths
A/B `treePrime := tStart`, path C `OverlayTreePaths`). Four independent de-riskings: (1) the existing
`TestDecompose_ConcurrentChangeExclusion` fixture IS the empty-frozen-leftover case, so the (a)+(b)
upgrade is pure assertion-strengthening on a fixture that already passes (no new wiring to debug); (2)
all helpers (`dcm*`, `concurrentSentinelSeam`, `stubtest.*`, `git.New`) are reusable and already in scope
in decompose_test.go — no redeclarations, no new binaries; (3) the three NEW assertions use exact git
oracles already proven by `TestDecompose_StagerFreezeViolation:781` (`git log --name-only --format=`) and
the production gate's own primitive (`git.New(repo).DiffTreeNames`); (4) the S2 fence is crisp
(empty-leftover vs legitimate-leftover) so there is no overlap risk. The two CRITICAL gotchas front-loaded
— (G2) `dcmLogCount` not `Amended` for the skip proof, and (G4) removing the now-false loophole-admitting
comments — are the two things an implementer would otherwise get wrong. The residual uncertainty (not
10/10) is the `concurrentSentinelSeam` Title-map-key matching (G7) and the born-repo `baseTree` capture
bookkeeping in the new test — fiddly plumbing the implementer must get right for the assertions to hold,
mitigated by the verbatim test bodies and the reusable helpers. No production-code risk (test-only); no
parallel-edit risk (only decompose_test.go, which no sibling touches — S2 is chain_test.go + a separate
decompose_test case that doesn't exist yet).
