---
name: "P1.M2.T2.S1 — Subset-check helper + re-tree-on-permitted-mutation logic (FR-V3 backstop)"
description: |
  The FR-V3 freeze-enforcement backstop for scoped `pre-commit` (PRD §9.25 FR-V3). After the hook runs
  against a throwaway index primed from the snapshot tree (P1.M2.T1.S2's `ReadTreeInto`/`WriteTreeFrom`),
  stagecoach must verify the hook introduced NO new paths: a formatter modifying/deleting an existing
  snapshot file is PERMITTED (the commit re-trees to the hook's output); a hook that ADDS a path not in the
  snapshot is a HARD ERROR (it would sweep concurrent work into the commit, violating the §5
  stage-while-generating freeze). Deliverable: `enforceSubset(ctx, g git.Git, snapshotTree, postTree string)
  error` + the `ErrHookSweptConcurrentWork` sentinel in a NEW `internal/hooks` package (policy, not a git
  primitive), mirroring the decompose stager's `ErrFreezeViolation`/`verifyFreezeSubset` FR-M1c twin.

  ⚠️ **THE central design call — `'A'` is the definitive new-path signal; NO `ListTreePaths` primitive is
  needed.** `DiffTreeNameStatus(snapshotTree, postTree)` runs `git diff-tree --no-commit-id --name-status -r`
  with NO `-M`/`-C` (verified git.go:1920). So an `'A'` status line is, BY DEFINITION, a path in postTree
  NOT in snapshotTree → a subset violation. `'M'`/`'D'`/`'T'` (modify/delete/typechange of an existing
  snapshot path) are permitted. A rename by the hook appears as `D\told`+`A\tnew` (no -M ⇒ no `R`) ⇒ the
  `'A'` triggers the hard error (correct: a rename stages a new path). The check is mathematically exact:
  postTree's path set ⊆ snapshotTree's iff the snapshot→post diff has no `'A'` lines. Parse: status =
  first tab-field's first byte; if `'A'`/`'C'`/`'R'` (defensive — C/R won't appear without -M/-C) → collect
  the LAST tab-field (the new/destination path).

  ⚠️ **THE second design call — mirror the FR-M1c twin's error-reporting shape EXACTLY.** The decompose
  stager already has the content-axis freeze-enforcement twin: `ErrFreezeViolation` (stager.go:75) +
  `verifyFreezeSubset` (stager.go:158), wrapped `fmt.Errorf("%w: …: %s", ErrFreezeViolation,
  strings.Join(extra, ", "))` so `errors.Is` works. THIS task's hook analogue: `ErrHookSweptConcurrentWork`
  + `fmt.Errorf("%w: pre-commit staged a path not in the snapshot: %s — … (FR-V3)",
  ErrHookSweptConcurrentWork, strings.Join(added, ", "))`. Both are NON-RESCUE hard errors (detected before
  the commit; HEAD/index untouched). The contract: "reuse the same error-reporting shape."

  ⚠️ **THE third design call — NEW `internal/hooks` package (policy, not a primitive); P1.M3.T1 extends
  it.** `internal/hooks` does NOT exist yet. The contract PREFERS it ("policy, not a primitive") and this
  mirrors the decompose precedent (`verifyFreezeSubset` lives in the CONSUMER package `internal/decompose`,
  NOT in internal/git). THIS task creates `internal/hooks` with `subset.go` (the helper + sentinel);
  P1.M3.T1 ADDS `runner.go` (the `RunPreCommit` sequence) to the same package. One-way dep: hooks → git
  (the Git interface); NO cycle. `enforceSubset` takes `g git.Git` (the interface — `DiffTreeNameStatus` is
  on it at L371), testable via the public `git.New(repo)`.

  ⚠️ **THE fourth design call — `enforceSubset` returns nil/error ONLY; the re-tree decision is the
  caller's.** The contract's OUTPUT clause: nil ⇒ permitted (caller uses postTree when postTree !=
  snapshotTree); error ⇒ abort. The re-tree is a one-line call-site decision (`tree := snapshotTree; if
  postTree != snapshotTree { tree = postTree }` gated by the check) — it does NOT warrant a separate
  function (the runner P1.M3.T1 owns the `commit-tree` call). `enforceSubset` is the testable deliverable.

  ⚠️ **PREREQUISITE (parallel, assume LANDED): P1.M2.T1.S2** (`ReadTreeInto`/`WriteTreeFrom`). THIS task
  consumes `WriteTreeFrom`'s `postTree`. The tests exercise the FULL scoped sequence (ReadTreeInto → mutate
  → WriteTreeFrom → enforceSubset) to prove the helper against real git output.

  Deliverable: `internal/hooks/subset.go` (NEW) — `enforceSubset` + `ErrHookSweptConcurrentWork`; and
  `internal/hooks/subset_test.go` (NEW) — the keystone matrix (no-mutation/permitted-M/permitted-D/
  forbidden-A/forbidden-rename/multi/git-failure). NO new git primitives, NO DiffTreeNames/NameStatus
  changes, NO runner/sequence/hook-discovery (P1.M3.T1), NO config/cli/docs. OUTPUT: the testable subset
  check the runner (P1.M3.T1) calls between WriteTreeFrom and commit-tree.
---

## Goal

**Feature Goal**: Implement the FR-V3 freeze backstop — `enforceSubset(ctx, g, snapshotTree, postTree)
error` in a new `internal/hooks` package — that returns nil when a scoped `pre-commit` hook's output
(`postTree`) introduces no new paths beyond the snapshot (permitted M/D/T mutation → re-tree), and a typed
hard error (`ErrHookSweptConcurrentWork`, mirroring the FR-M1c `ErrFreezeViolation` shape) naming the
offending path(s) when the hook added a path not in the snapshot (would sweep concurrent work in).

**Deliverable** (2 NEW files, NEW package):
1. **`internal/hooks/subset.go`** (`package hooks`) — `var ErrHookSweptConcurrentWork = errors.New(...)`;
   `func enforceSubset(ctx context.Context, g git.Git, snapshotTree, postTree string) error`. Body:
   `DiffTreeNameStatus(snapshotTree, postTree)` → parse lines → collect `'A'` (defensive also `'C'`/`'R'`)
   paths → if any, return `%w`-wrapped `ErrHookSweptConcurrentWork` naming them; else nil. Imports:
   `internal/git` + stdlib (`context`,`errors`,`fmt`,`strings`).
2. **`internal/hooks/subset_test.go`** (`package hooks` / `hooks_test`) — the keystone matrix (§Validation)
   building a real repo + scoped-index mutations via an independent oracle, capturing postTree via
   `WriteTreeFrom`, asserting `enforceSubset`'s verdict.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/hooks/` clean;
`go test -race ./internal/hooks/` green (the matrix); `go test -race ./...` green (additive package, no
regression). Concretely: a permitted M (formatter reformats a snapshot file) → nil; a permitted D (hook
removes a staged file) → nil; a forbidden A (new file) → `errors.Is(err, ErrHookSweptConcurrentWork)` with
the path named; a rename → forbidden (shows as A under no-`-M`); DiffTreeNameStatus git failure → wrapped
non-sentinel error. go.mod unchanged (no new dep — `internal/git` is the only stagecoach import).

## User Persona

**Target User**: The commit-hooks runner (P1.M3.T1 — `internal/hooks.RunPreCommit`) which calls
`enforceSubset` between `WriteTreeFrom` (capture postTree) and `commit-tree`. Transitively every user who
runs `stagecoach` on a repo with a `pre-commit` hook (US19, FR-V1): their hook fires on the snapshotted
content, and a hook that tries to sweep in unstaged concurrent work is caught and aborted rather than
silently committed.

**Use Case**: A user has a `pre-commit` formatter (prettier/gofmt). `stagecoach` snapshots the index, runs
the formatter scoped to T_start. The formatter reformats an existing file (permitted → re-tree, the commit
reflects the fix) — but if the formatter (or a misbehaving hook) stages a NEW file the user didn't stage,
enforceSubset aborts with a clear error rather than committing the user's half-finished work.

**User Journey**: (internal) snapshot tree → ReadTreeInto(T_start, tmp) → pre-commit runs (scoped) →
WriteTreeFrom(tmp) → **enforceSubset(snapshot, postTree)** (THIS task) → nil ⇒ commit-tree(postTree);
error ⇒ abort (FR-V7 rescue state). The §5 freeze holds: nothing staged DURING generation reaches the commit.

**Pain Points Addressed**: removes the "a hooked formatter swept my unstaged WIP into the commit" hazard
(the FR-V3 freeze backstop) — without breaking the common case (a formatter fixing an existing file).

## Why

- **FR-V3's safety backstop (P1).** The PRD accepts hook mutations to existing T_start paths (git-commit
  parity) but REQUIRES a hard error for new paths (sweep = freeze violation). `enforceSubset` IS that rule.
- **Mirrors the proven FR-M1c twin.** The decompose stager already enforces the same invariant
  (`verifyFreezeSubset`/`ErrFreezeViolation`). This is the hook-path analogue — same shape, same severity
  (non-rescue hard error), same `errors.Is`-able sentinel + named-path reporting.
- **Testable in isolation.** Factored as a pure function over two tree SHAs (via the read-only
  `DiffTreeNameStatus`), it's unit-testable without running a real hook — the matrix (§Validation) applies
  scoped-index mutations directly and asserts the verdict. The runner (P1.M3.T1) composes it.
- **Cheap + exact.** `'A'` is a mathematically exact subset-violation signal (no `ListTreePaths` needed);
  no new git primitive; one new small package.
- **No new surface.** DOCS: none (internal invariant; the FR-V3 behavior is documented via M3.T2.S1's
  Mode A subsection). No config/cli change.

## What

A new `internal/hooks` package exporting `enforceSubset` + `ErrHookSweptConcurrentWork`. The helper calls
`DiffTreeNameStatus(snapshotTree, postTree)`, parses the status lines, and returns nil (no `'A'` ⇒ subset
holds) or a `%w`-wrapped sentinel error naming the added path(s). The re-tree decision (use postTree when
nil and ≠ snapshot) is the caller's documented one-liner. No git-primitive/runner/config/cli/doc changes.

### Success Criteria

- [ ] `internal/hooks/subset.go` exists, `package hooks`, imports `internal/git` + stdlib only.
- [ ] `var ErrHookSweptConcurrentWork = errors.New("hooks: pre-commit swept concurrent work")` exists.
- [ ] `func enforceSubset(ctx context.Context, g git.Git, snapshotTree, postTree string) error`:
      calls `g.DiffTreeNameStatus(ctx, snapshotTree, postTree)`; on git failure returns a wrapped non-sentinel
      error; parses each line (tab-split; status = first byte of field[0]); for `'A'` (defensive `'C'`/`'R'`)
      collects the LAST tab-field (the new/destination path); if any collected → `fmt.Errorf("%w: pre-commit
      staged a path not in the snapshot: %s — refusing to sweep concurrent work into the commit (FR-V3)",
      ErrHookSweptConcurrentWork, strings.Join(added, ", "))`; else nil.
- [ ] `postTree == snapshotTree` (no mutation) → nil (trivially; the diff is empty).
- [ ] `'M'`/`'D'`/`'T'` lines do NOT trigger the error (permitted mutations).
- [ ] `internal/hooks/subset_test.go` has the keystone matrix (no-mutation, permitted-M, permitted-D,
      forbidden-A, forbidden-rename, multi-violation, git-failure), all passing.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/hooks/`, `go test -race ./internal/hooks/`,
      `go test -race ./...` clean/green; go.mod/go.sum unchanged; DiffTreeNameStatus/Names byte-unchanged;
      only the 2 new files touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact `enforceSubset`
body (quoted), the `'A'`-is-the-signal insight (DiffTreeNameStatus uses no `-M`), the FR-M1c twin to mirror
(`ErrFreezeViolation`'s shape), the parse rule (last tab-field), the placement (`internal/hooks`), and the
test matrix with an independent-oracle sketch. No git-internals knowledge beyond `git diff-tree --name-status`.

### Documentation & References

```yaml
# MUST READ — the authoritative research
- docfile: plan/010_49117f1f30ab/P1M2T2S1/research/subset-check-and-retree.md
  why: the `'A'`-is-the-signal logic (§1, with the no-`-M` proof + the rename⇒A consequence), the FR-M1c
       twin to mirror (§2 — ErrFreezeViolation/verifyFreezeSubset shape), the internal/hooks placement (§3),
       the re-tree decision (§4 — caller's one-liner), the resolved design (§5 — open_questions §1 +
       external_deps §8), the test matrix with sketches (§6), scope fences (§7), validation (§8).
  critical: §1 (the helper's whole logic — 'A' ⇒ hard error; M/D/T ⇒ nil; no ListTreePaths needed) and §2
       (mirror ErrFreezeViolation's sentinel + %w + named-paths shape).

# The contract — what the parallel scoped-primitive task produces (assume LANDED)
- docfile: plan/010_49117f1f30ab/P1M2T1S2/PRP.md
  why: P1.M2.T1.S2 added ReadTreeInto(ctx, tree, indexFile) + WriteTreeFrom(ctx, indexFile) (scoped via
       GIT_INDEX_FILE). THIS task consumes WriteTreeFrom's postTree. The tests exercise BOTH (prime →
       mutate → capture → check).
  critical: the tests use WriteTreeFrom to capture postTree after a scoped mutation — that primitive IS
       the input source. Do NOT re-implement the scoped variants.

# The FR-M1c twin (the error-reporting pattern to mirror — READ ONLY)
- file: internal/decompose/stager.go
  section: ErrFreezeViolation (L60-75) + verifyFreezeSubset (L140-180) + pathSet (L199).
  why: the EXACT shape to mirror — a package-level sentinel `var ErrXxx = errors.New("...")`; a
       `fmt.Errorf("%w: …: %s", ErrXxx, strings.Join(extra, ", "))` wrap (errors.Is-able); names offending
       paths; non-rescue hard error. verifyFreezeSubset uses DiffTreeNames + pathSet; enforceSubset uses
       DiffTreeNameStatus (status-aware). The DIFFERENCE: verifyFreezeSubset checks stager tree[i] ⊆ T_start
       (content + path); enforceSubset checks postTree ⊆ snapshotTree (path-only, via 'A' status).
  pattern: copy the sentinel + %w-wrap + Join style verbatim (just rename the sentinel + message).

# The git primitive consumed (READ ONLY — unchanged)
- file: internal/git/git.go
  section: DiffTreeNameStatus interface (L367-371) + impl (L1915-1930).
  why: the function enforceSubset calls. Signature `DiffTreeNameStatus(ctx, treeA, treeB string) (nameStatus
       string, err error)`; runs `git diff-tree --no-commit-id --name-status -r <treeA> <treeB>` (NO -M/-C);
       returns raw `A\tpath`/`M\tpath`/`D\tpath`/`T\tpath` lines (R/C only with -M/-C, which aren't passed).
  pattern: call `g.DiffTreeNameStatus(ctx, snapshotTree, postTree)`; on err wrap (non-sentinel); parse stdout.
  gotcha: it returns RAW stdout (incl. a trailing newline); split on "\n", skip empty lines. treeA=snapshot,
          treeB=post (so 'A' = added in post = not in snapshot = the violation).

# The resolved design (the scoped-index mechanism — READ ONLY)
- docfile: plan/010_49117f1f30ab/architecture/external_deps.md
  section: "8. The central design tension" + the "Recommended faithful sequence".
  why: the sequence (read-tree → hook → write-tree → subset check) THIS helper completes; and the §8
       rationale (the subset check is the backstop for working-tree-coupled hooks).
  critical: enforceSubset is the "subset check" step in the faithful sequence.

- docfile: plan/010_49117f1f30ab/open_questions.md   (or architecture/open_questions.md)
  section: "1. The scoped-index mechanism — RESOLVED approach"
  why: confirms DiffTreeNameStatus as the subset-check tool ("if any ADDED path is not in the snapshot's
       path set → hard error (FR-V3)") and the two-layer split (git primitives + hooks runner). enforceSubset
       is the factored-out, testable subset-check portion of the runner layer.

# The requirement
- file: PRD.md
  section: "9.25 Hook execution on the commit path" (h3.41) — FR-V3 ("pre-commit is scoped to T_start … A
       hook that stages a path not present in T_start is a hard error (it would sweep in concurrent work and
       violate the freeze — the same subset-enforcement discipline as the stager, FR-M1c)").
  why: FR-V3 IS the requirement. The "same discipline as FR-M1c" clause is why we mirror ErrFreezeViolation.
```

### Current Codebase tree (relevant slice — POST-P1.M2.T1.S2 assumed)

```bash
internal/git/
  git.go              # DiffTreeNameStatus (L1915, interface L371) + ReadTreeInto/WriteTreeFrom (P1.M2.T1.S2)  ← INPUT (consumed, UNCHANGED)
internal/decompose/
  stager.go           # ErrFreezeViolation + verifyFreezeSubset + pathSet  ← the twin to MIRROR (READ ONLY)
internal/hooks/       # NEW PACKAGE (THIS task creates it; P1.M3.T1 adds runner.go later)
  subset.go           # NEW — enforceSubset + ErrHookSweptConcurrentWork
  subset_test.go      # NEW — the keystone matrix
go.mod / go.sum       # unchanged (internal/git already a module dep; no external dep)
```

### Desired Codebase tree with files to be added

```bash
internal/hooks/       # NEW PACKAGE
  subset.go           # NEW — enforceSubset(ctx, g git.Git, snapshotTree, postTree) error + ErrHookSweptConcurrentWork
  subset_test.go      # NEW — TestEnforceSubset_* matrix (no-mutation/permitted-M/permitted-D/forbidden-A/forbidden-rename/multi/git-failure)
# go.mod/go.sum unchanged. internal/git + internal/decompose byte-unchanged.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the logic): DiffTreeNameStatus uses NO -M/-C (git.go:1920: "diff-tree --no-commit-id
// --name-status -r"). So 'A' is the EXACT, sufficient signal of a path in postTree NOT in snapshotTree.
// Do NOT compute the snapshot's full path set (no ListTreePaths) — the 'A' status IS the check. A rename by
// the hook shows as D+A (no -M) ⇒ the 'A' fires (correct — a rename stages a new path). M/D/T are permitted.

// CRITICAL (the error shape): mirror ErrFreezeViolation (stager.go:75) EXACTLY — a package-level sentinel
// var + fmt.Errorf("%w: …: %s", ErrHookSweptConcurrentWork, strings.Join(added, ", ")). errors.Is MUST work
// (the runner P1.M3.T1 + tests assert on it). It is a NON-RESCUE hard error (FR-V7: no update-ref ran).

// CRITICAL (the parse): --name-status lines are tab-split. field[0]=status (first byte is the letter;
// R/C lines carry a score suffix like "R100" — use status[0]). The NEW path (the violation) is the LAST
// tab-field (fields[len(fields)-1]): field[1] for A/M/D/T, field[2] for R/C. Skip empty lines (trailing \n).

// CRITICAL (placement): NEW internal/hooks package. It is POLICY (the FR-M1c twin lives in the consumer
// package internal/decompose, NOT internal/git). P1.M3.T1 ADDS runner.go to this package — creating it here
// is correct and expected. One-way dep hooks→git (the Git interface); NO cycle.

// CRITICAL (signature): take g git.Git (the INTERFACE), NOT *gitRunner. DiffTreeNameStatus is on the
// interface (L371); the interface makes enforceSubset testable via git.New(repo) and decouples hooks from
// *gitRunner internals. The runner P1.M3.T1 also takes git.Git.

// GOTCHA (the re-tree decision is the CALLER's): enforceSubset returns nil/error ONLY. The caller (runner)
// does `tree := snapshotTree; if postTree != snapshotTree && err == nil { tree = postTree }`. Do NOT add a
// resolvePostHookTree helper — it's a one-liner the runner owns (and it owns commit-tree). Document the rule.

// GOTCHA (test helpers): internal/git's test helpers (initRepo/writeFile/stageFile/writeTreeOf/execGit) are
// `package git` (internal) — NOT importable from `package hooks`. Re-create minimal helpers in subset_test.go
// (or use exec.Command("git", "-C", repo, ...) directly). Mirror the independent-oracle discipline from
// P1.M2.T1.S2 (apply scoped mutations via exec.Command with GIT_INDEX_FILE env, NOT via the runner).

// GOTCHA (test the FULL sequence): prime a throwaway index via g.ReadTreeInto(snapshotTree, tmp) (P1.M2.T1.S2),
// apply the mutation via an independent oracle (exec.Command "git update-index"/"git rm --cached" with
// GIT_INDEX_FILE=tmp), capture postTree via g.WriteTreeFrom(tmp), then call enforceSubset(ctx, g,
// snapshotTree, postTree). This exercises BOTH the scoped primitives AND the subset check end-to-end.

// GOTCHA: do NOT add -M to DiffTreeNameStatus (it's consumed unchanged; adding -M would change R/C handling
// and is out of scope). Do NOT touch DiffTreeNames/NameStatus, ReadTreeInto/WriteTreeFrom, or any runner file.
```

## Implementation Blueprint

### Data models and structure

One sentinel + one function. No new types.

```go
// internal/hooks/subset.go
package hooks

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dustin/stagecoach/internal/git"
)

// ErrHookSweptConcurrentWork is the sentinel for an FR-V3 FREEZE violation by a scoped pre-commit hook:
// the hook's output tree contains a path NOT present in the snapshot tree (an added file, or the
// destination of a rename/copy) — i.e. the hook tried to sweep concurrent/unstaged work into the commit.
// This is the hook-path twin of decompose.ErrFreezeViolation (the FR-M1c stager freeze guard): both are
// content-axis, NON-RESCUE hard errors (detected before commit-tree; HEAD and the index are untouched).
// Wrapped with %w so errors.Is(err, ErrHookSweptConcurrentWork) is true. Produced by enforceSubset.
var ErrHookSweptConcurrentWork = errors.New("hooks: pre-commit swept concurrent work")

// enforceSubset verifies the FR-V3 freeze backstop: postTree's path set must be a SUBSET of snapshotTree's
// (the hook introduced NO new paths). It is the "subset check" step in the scoped pre-commit sequence
// (external_deps.md §8): after ReadTreeInto(snapshot, tmp) → pre-commit → WriteTreeFrom(tmp) = postTree,
// the runner calls enforceSubset(snapshot, postTree). nil ⇒ the hook's mutations are permitted (M/D/T of
// existing snapshot paths); the caller uses postTree as the commit's tree (re-tree, git-commit parity).
// ErrHookSweptConcurrentWork ⇒ the hook added a path not in the snapshot (would sweep concurrent work in);
// the caller aborts the run (FR-V7 rescue state — no update-ref ran).
//
// The check uses DiffTreeNameStatus(snapshot, post), which runs WITHOUT -M/-C: an 'A' status line is,
// BY DEFINITION, a path in postTree not in snapshotTree (a subset violation). 'M'/'D'/'T' (modify/delete/
// typechange of an existing snapshot path) do NOT violate the subset and are permitted. A rename by the
// hook appears as D+A (no -M ⇒ no R line), so the 'A' correctly fires (a rename stages a new path).
// Defensively, 'C'/'R' status letters (which would only appear if -M/-C were ever added) are also flagged;
// the offending path is the LAST tab-field of the status line (the new/destination path).
//
// A git failure from DiffTreeNameStatus (e.g. a bad tree SHA) is a wrapped NON-sentinel error (not a sweep).
func enforceSubset(ctx context.Context, g git.Git, snapshotTree, postTree string) error {
	nameStatus, err := g.DiffTreeNameStatus(ctx, snapshotTree, postTree)
	if err != nil {
		return fmt.Errorf("hook subset check: diff-tree-name-status: %w", err)
	}
	var added []string
	for _, line := range strings.Split(nameStatus, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		status := fields[0]
		if len(status) == 0 {
			continue
		}
		// 'A' = added (not in snapshot); 'C'/'R' = copy/rename destination (defensive — no -M/-C today).
		// 'M'/'D'/'T' (modify/delete/typechange of an existing snapshot path) are permitted.
		switch status[0] {
		case 'A', 'C', 'R':
			added = append(added, fields[len(fields)-1]) // the new/destination path (last tab-field)
		}
	}
	if len(added) > 0 {
		return fmt.Errorf("%w: pre-commit staged a path not in the snapshot: %s — refusing to sweep concurrent work into the commit (FR-V3)",
			ErrHookSweptConcurrentWork, strings.Join(added, ", "))
	}
	return nil
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/hooks/subset.go — the sentinel + enforceSubset
  - FILE: internal/hooks/subset.go, `package hooks`. Imports: context, errors, fmt, strings + internal/git.
  - ADD ErrHookSweptConcurrentWork (the doc comment naming it the FR-V3 twin of decompose.ErrFreezeViolation).
  - ADD enforceSubset per the Data Models block: DiffTreeNameStatus(snapshot, post) → parse → collect 'A'/'C'/'R'
    last-fields → %w-wrap ErrHookSweptConcurrentWork with the comma-joined paths on any; nil otherwise.
  - GOTCHA: take g git.Git (the interface). status[0] is the letter (R100 ⇒ 'R'). Skip empty/short lines.
    Do NOT compute a snapshot path set; do NOT add a ListTreePaths primitive.

Task 2: CREATE internal/hooks/subset_test.go — the keystone matrix
  - FILE: internal/hooks/subset_test.go, `package hooks` (white-box) — so it can call the unexported
    enforceSubset. Imports: context, errors, os, os/exec, path/filepath, strings, testing + internal/git.
  - HELPERS (local — internal/git's are package-private): a minimal initRepo/writeFile/stageFile/makeEmptyCommit/
    writeTreeOf/execGit (or use exec.Command directly); plus a scopedUpdateIndex(repo, tmpIndex, op, path) that
    runs `git -C repo update-index --add/--remove <path>` with cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmp).
  - COMMON SETUP: repo := t.TempDir(); initRepo; build snapshotTree (writeFile a.go+b.go; stageFile; makeEmptyCommit;
    snapshotTree := writeTreeOf(repo)); tmpIndex := filepath.Join(t.TempDir(), "scoped.index"); g := git.New(repo);
    g.ReadTreeInto(ctx, snapshotTree, tmpIndex) (P1.M2.T1.S2 — prime the throwaway index).
  - TEST TestEnforceSubset_NoMutation: no scoped mutation; postTree := g.WriteTreeFrom(tmp); assert postTree ==
    snapshotTree; enforceSubset(ctx, g, snapshotTree, postTree) == nil.
  - TEST TestEnforceSubset_PermittedModify: scoped update-index --add a MODIFIED a.go (content change); postTree :=
    WriteTreeFrom; enforceSubset → nil (M status; caller would re-tree to postTree).
  - TEST TestEnforceSubset_PermittedDelete: scoped update-index --remove a.go (or git rm --cached -f a.go with
    GIT_INDEX_FILE); postTree := WriteTreeFrom; enforceSubset → nil (D status; subset holds).
  - TEST TestEnforceSubset_ForbiddenAddedFile (THE KEYSTONE): scoped update-index --add a NEW c.go; postTree :=
    WriteTreeFrom; err := enforceSubset; assert errors.Is(err, ErrHookSweptConcurrentWork) AND err string names "c.go".
  - TEST TestEnforceSubset_ForbiddenRename: scoped add a2.go (content of a.go) + remove a.go; postTree := WriteTreeFrom;
    err := enforceSubset; assert errors.Is(err, ErrHookSweptConcurrentWork) AND names "a2.go" (rename ⇒ A under no-`-M`).
  - TEST TestEnforceSubset_MultipleViolations: scoped add c.go + d.go; err names BOTH (comma-joined).
  - TEST TestEnforceSubset_DiffTreeFailure: enforceSubset(ctx, g, "0000...0", snapshotTree) → err != nil AND
    NOT errors.Is(err, ErrHookSweptConcurrentWork) (a wrapped git failure, not a sweep).
  - GOTCHA: use the INDEPENDENT-ORACLE pattern (exec.Command with GIT_INDEX_FILE env) for the scoped mutation —
    test the SUBSET CHECK, not the scoped primitives (those have their own tests in P1.M2.T1.S2). Capture postTree
    via g.WriteTreeFrom (exercises both the scoped capture AND the check). No t.Parallel() (shared tmp/repos).

Task 3: VERIFY (no further file change)
  - RUN `gofmt -w internal/hooks/`; `go vet ./internal/hooks/`; `go build ./...`; `go test -race ./internal/hooks/ -v`;
    `go test -race ./...` (additive package; no regression). go.mod/go.sum byte-unchanged.
  - internal/git (DiffTreeNameStatus/Names + ReadTreeInto/WriteTreeFrom) + internal/decompose (ErrFreezeViolation)
    byte-unchanged.
```

### Implementation Patterns & Key Details

```go
// THE check (the whole logic — 'A' ⇒ violation; M/D/T ⇒ permitted):
for _, line := range strings.Split(nameStatus, "\n") {
	if line == "" { continue }
	fields := strings.Split(line, "\t")
	if len(fields) < 2 { continue }
	if status := fields[0]; len(status) > 0 {
		switch status[0] {
		case 'A', 'C', 'R':               // a NEW path (added / copy-dest / rename-dest)
			added = append(added, fields[len(fields)-1])  // last tab-field = the new path
		// 'M'/'D'/'T' ⇒ permitted (modify/delete/typechange of an existing snapshot path) — no action
		}
	}
}
// THE error shape (mirror decompose.ErrFreezeViolation — sentinel + %w + named paths):
if len(added) > 0 {
	return fmt.Errorf("%w: pre-commit staged a path not in the snapshot: %s — refusing to sweep concurrent work into the commit (FR-V3)",
		ErrHookSweptConcurrentWork, strings.Join(added, ", "))
}
return nil

// THE re-tree decision (the CALLER's one-liner — NOT in enforceSubset):
// tree := snapshotTree
// if postTree != snapshotTree {
//     if err := enforceSubset(ctx, g, snapshotTree, postTree); err != nil { return err }
//     tree = postTree   // permitted mutation → re-tree (git-commit parity)
// }
```

```go
// subset_test.go — the independent-oracle scoped mutation (mirror P1.M2.T1.S2's discipline):
func scopedStage(t *testing.T, repo, tmpIndex, op, path string) {
	t.Helper()
	// op = "--add" / "--remove"; path resolved under repo
	cmd := exec.Command("git", "-C", repo, "update-index", op, path)
	cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpIndex)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("scoped update-index %s %s: %v\n%s", op, path, err, out)
	}
}
// Use: g.ReadTreeInto(ctx, snapshotTree, tmp) → scopedStage(t, repo, tmp, "--add", newPath) →
//      postTree, _ := g.WriteTreeFrom(ctx, tmp) → err := enforceSubset(ctx, g, snapshotTree, postTree).
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — internal/git is already a module dep; no external dep. go mod tidy no-op.

PACKAGE EDGES:
  - internal/hooks → internal/git (the Git interface). One-way; NO cycle (git does not import hooks).
  - internal/hooks → stdlib only (context, errors, fmt, strings).

FROZEN / NOT-EDITED:
  - internal/git/git.go (DiffTreeNameStatus L1915 / DiffTreeNames L1627 / ReadTreeInto / WriteTreeFrom —
    consumed via the interface, UNCHANGED).
  - internal/decompose/stager.go (ErrFreezeViolation + verifyFreezeSubset — the twin to MIRROR, not edit).
  - Any runner/sequence file (internal/hooks/runner.go is P1.M3.T1 — NOT this task).
  - config/cli (P1.M1), generate/decompose commit paths (P1.M3.T2/T3), docs (P1.M4).

DOWNSTREAM CONSUMER (do NOT implement here):
  - P1.M3.T1 (internal/hooks runner): RunPreCommit does ReadTreeInto → run pre-commit (scoped) →
    WriteTreeFrom → CALLS enforceSubset → on nil uses postTree (re-tree) else aborts (FR-V7). This task is
    the testable helper the runner calls; the runner owns the hook-discovery/env/timeout/sequence/--no-verify.

NO DATABASE / ROUTES / CONFIG / CLI / NEW GIT PRIMITIVES / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/hooks/subset.go internal/hooks/subset_test.go
test -z "$(gofmt -l internal/hooks/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/hooks/   # catches a bad interface call / unused import / wrong sentinel wrap.
go build ./...             # the new package compiles; no caller breaks (additive).
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm DiffTreeNameStatus/Names + ReadTreeInto/WriteTreeFrom are byte-unchanged (consumed, not modified):
git diff --exit-code internal/git/git.go && echo "internal/git UNCHANGED (expected)" || echo "(if P1.M2.T1.S2 landed in-flight, its scoped variants are the only expected git.go delta — confirm DiffTreeNameStatus/Names untouched)"
```

### Level 2: Unit Tests (Component Validation) — the keystone matrix

```bash
go test -race ./internal/hooks/ -v
# Expected PASS — verify each case:
#   TestEnforceSubset_NoMutation        → nil (postTree == snapshot)
#   TestEnforceSubset_PermittedModify   → nil (M status)
#   TestEnforceSubset_PermittedDelete   → nil (D status)
#   TestEnforceSubset_ForbiddenAddedFile → errors.Is(ErrHookSweptConcurrentWork) + names "c.go"  [KEYSTONE]
#   TestEnforceSubset_ForbiddenRename   → errors.Is(...) + names "a2.go" (rename ⇒ A under no-`-M`)
#   TestEnforceSubset_MultipleViolations → names BOTH (comma-joined)
#   TestEnforceSubset_DiffTreeFailure   → err != nil AND NOT errors.Is(ErrHookSweptConcurrentWork)
go test -race ./...   # full module — no regression (the new package is additive; only its tests import it).
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only the 2 new files changed:
git diff --name-only | grep -Ev '^internal/hooks/subset\.go$|^internal/hooks/subset_test\.go$' \
  && echo "UNEXPECTED file changed (unless P1.M2.T1.S2's scoped variants are in-flight)" || echo "only internal/hooks/{subset,subset_test}.go changed (good)"
# Confirm the sentinel is errors.Is-able (the FR-M1c twin shape) + the %w wrap:
grep -n 'ErrHookSweptConcurrentWork\|%w' internal/hooks/subset.go
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Property check (optional): for ANY postTree that is a pure subset-mutation of snapshotTree (only M/D/T,
# no A), enforceSubset MUST return nil; for ANY postTree with an 'A' path, it MUST return the sentinel.
# The keystone matrix covers the salient cases; a table-driven loop over {M-only, D-only, M+D, A, A+M,
# rename, copy-via-A} pins the property. Cross-check the rename case manually: `git diff-tree --name-status
# -r <snap> <post-with-rename>` shows "D\told" + "A\tnew" (confirming the no-`-M` ⇒ A reasoning). golangci-lint:
# `make lint` (project-wide gate — the new package's symbols are used by its tests, so no unused-export finding).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `gofmt -l internal/hooks/`, `go vet ./...`, `go build ./...`, `go mod tidy` no-op;
      go.mod/go.sum byte-unchanged; internal/git byte-unchanged (DiffTreeNameStatus/Names consumed, not modified).
- [ ] Level 2 green: `go test -race ./internal/hooks/` (the matrix) AND `go test -race ./...`.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only the 2 new files changed.

### Feature Validation

- [ ] `enforceSubset(ctx, g git.Git, snapshotTree, postTree) error` + `ErrHookSweptConcurrentWork` exist.
- [ ] `'A'` (and defensive `'C'`/`'R'`) ⇒ `errors.Is(err, ErrHookSweptConcurrentWork)` with the path(s) named.
- [ ] `'M'`/`'D'`/`'T'` + no-mutation (postTree==snapshot) ⇒ nil.
- [ ] A rename by the hook ⇒ forbidden (shows as `'A'` under no-`-M`).
- [ ] DiffTreeNameStatus git failure ⇒ wrapped NON-sentinel error.
- [ ] The re-tree decision is the caller's documented one-liner (enforceSubset returns nil/error only).

### Code Quality Validation

- [ ] Mirrors `decompose.ErrFreezeViolation`/`verifyFreezeSubset` (sentinel + `%w` + named paths + non-rescue).
- [ ] `internal/hooks` is a NEW policy package (not a git primitive); one-way dep hooks→git; takes `git.Git`.
- [ ] No `ListTreePaths` primitive (the `'A'` status is the check); no `-M` added to DiffTreeNameStatus.
- [ ] No scope creep into the scoped primitives (P1.M2.T1.S2), the runner (P1.M3.T1), config/cli (P1.M1),
      commit paths (P1.M3.T2/T3), or docs (P1.M4).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Doc comments on `ErrHookSweptConcurrentWork` (names the FR-M1c twin) + `enforceSubset` (names FR-V3,
      the no-`-M` ⇒ `'A'` logic, the faithful-sequence step, the re-tree rule).
- [ ] go.mod/go.sum byte-unchanged; 2 new files only.

---

## Anti-Patterns to Avoid

- ❌ Don't compute the snapshot's full path set or add a `ListTreePaths`/`ls-tree` primitive. `'A'` from
  `DiffTreeNameStatus(snapshot, post)` IS the subset-violation signal (no `-M` ⇒ rename/copy also surface as
  `'A'`). The check is one DiffTreeNameStatus call + a status-letter parse.
- ❌ Don't deviate from the FR-M1c twin's error shape. Mirror `decompose.ErrFreezeViolation`: package-level
  sentinel + `fmt.Errorf("%w: …: %s", ErrHookSweptConcurrentWork, strings.Join(added, ", "))`. `errors.Is`
  MUST work (the runner + tests assert on it). It's a NON-RESCUE hard error (FR-V7).
- ❌ Don't take `*gitRunner` — take `g git.Git` (the interface). `DiffTreeNameStatus` is on the interface;
  the interface makes the helper testable via `git.New(repo)` and decouples `internal/hooks` from git internals.
- ❌ Don't put the helper in `internal/git` (it's POLICY, not a primitive) or in `internal/decompose` (wrong
  consumer). NEW `internal/hooks` — mirroring the precedent that `verifyFreezeSubset` lives in its consumer
  package (`internal/decompose`). P1.M3.T1 adds `runner.go` to this package.
- ❌ Don't implement the re-tree decision inside `enforceSubset` (it returns nil/error only). The caller
  (runner P1.M3.T1) does the one-line `if postTree != snapshotTree { tree = postTree }` gated by the check.
  A `resolvePostHookTree` helper is too trivial and would overlap the runner's commit-tree ownership.
- ❌ Don't add `-M`/`-C` to DiffTreeNameStatus (consumed UNCHANGED). Adding `-M` would turn renames into `R`
  lines (handled defensively, but it's out of scope and changes the primitive's contract).
- ❌ Don't touch DiffTreeNames/DiffTreeNameStatus, ReadTreeInto/WriteTreeFrom, the runner, config/cli, or
  docs. Those are consumed (git), the twin to mirror (decompose), or other tasks' scope (M1/M3.T1/M3.T2/M4).
- ❌ Don't apply scoped mutations via the runner-under-test in subset_test.go. Use the INDEPENDENT-ORACLE
  pattern (`exec.Command("git", "-C", repo, "update-index", ...)` with `GIT_INDEX_FILE=<tmp>` env) — test the
  SUBSET CHECK against real git output, not the scoped primitives (those have their own tests in P1.M2.T1.S2).
- ❌ Don't forget the `'A'` empty-line / short-line guards in the parse (DiffTreeNameStatus returns raw stdout
  with a trailing newline; a malformed line must be skipped, not panic on `fields[0]`/`status[0]`).
- ❌ Don't change go.mod/go.sum. `internal/git` is already a module dep; the only new import is stdlib. An
  external dep is wrong; an unused import fails `go vet`.
- ❌ Don't skip `go vet`/`go build`/`go test -race ./internal/hooks/`/`go test -race ./...` — they catch a
  bad interface call, an unimportable package, a non-`errors.Is`-able wrap, and any regression from the new
  additive package.

---

## Confidence Score

**9/10** — a small, pure policy helper with a mathematically exact signal (`'A'` ⇒ subset violation, proven
by DiffTreeNameStatus's no-`-M` implementation), a directly-mirrorable FR-M1c twin for the error shape, a
resolved design (open_questions §1 + external_deps §8) confirming both the tool and the placement, and a
concrete keystone-matrix test plan using the independent-oracle discipline. Every fact (DiffTreeNameStatus
args/line, ErrFreezeViolation shape, scoped-primitive availability) is verified against the live codebase.
The -1 reserves for the test-helper re-creation (internal/git's helpers are package-private, so subset_test.go
rebuilds minimal ones) and the optional property-table expansion in Level 4.
