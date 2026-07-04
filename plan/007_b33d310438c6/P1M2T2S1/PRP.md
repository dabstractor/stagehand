---
name: "P1.M2.T2.S1 — Inject -M + -U<diff_context> via buildDiffArgs; update golden fixtures (FR3e/FR3f)"
description: |
  Implement FR3e (deterministic rename detection) + FR3f (reduced unified context) at the SINGLE shared
  argv site. The parallel P1.M2.T1.S1 created `buildDiffArgs(domain ...string) []string` (returns
  `["diff", domain…]`) and routed the 9 patch-argv sites (3 per function: md-list / per-file / nmArgs across
  StagedDiff/TreeDiff/WorkingTreeDiff) through it, byte-identical. THIS task EXTENDS the helper to take
  `opts StagedDiffOptions` and appends `-M` (always on) + `-U<ctx>` (effective context) after the domain,
  so both flags land in all 3 patch paths of all 3 functions from ONE edit. Then update golden fixtures +
  add rename (-M) and -U0 positive tests.

  ⚠️ **THE central design call — extend the helper signature to `(opts StagedDiffOptions, domain ...string)`;
  inject `-M` + `-U<ctx>` AFTER the domain, BEFORE the caller's trailing tokens.** The helper returns
  `["diff", domain…, "-M", "-U<ctx>"]`; each of the 9 callers appends its existing trailing tokens
  (`--name-only -- *.md *.markdown` / `-- <file>` / `-- excludes :!*.md :!*.markdown binExcludes`).
  `-M` is ALWAYS ON (FR3e: the only deterministic cross-version/config rename detector; wins over
  `diff.renames=false`; never `-C` — O(files²), rejected by FR3e). `-U<ctx>` is `opts.DiffContext`
  clamped to [0,3] (out-of-range → 1, defensive). The 9 call sites change from `buildDiffArgs("--cached")`
  to `buildDiffArgs(opts, "--cached")` (`opts` is in scope in all 3 functions). `fmt` is already imported.

  ⚠️ **THE second design call — the -U0 / default-1 resolution is DEFINITIVE (3 sources agree).**
  `StagedDiffOptions.DiffContext` is a PLAIN int carrying the RESOLVED value (struct doc: "0 is VALID
  (-U0); callers MUST pass the resolved default 1, NEVER a 0-means-unset sentinel"). `system_context.md`
  L91: "DiffContext default is 1, but 0 is VALID (-U0)." `config.DiffContextValue()` resolves the config
  `*int` (nil⇒1, *0⇒0); `config.Defaults()` sets `intPtr(1)`; the 6 call sites all use `DiffContextValue()`.
  So the helper maps `0→-U0`, `1→-U1`, `2→-U2`, `3→-U3`, out-of-range→1. Production ALWAYS passes the
  resolved value (default 1); only TESTS passing bare `StagedDiffOptions{}` get 0→-U0. This is why the
  item's two test clauses coexist: golden fixtures pass `DiffContext:1` → show -U1 ("context shrinks
  -U3→-U1"); the NEW -U0 test passes `DiffContext:0` → changed-lines-only.

  ⚠️ **THE third design call — FR3e/FR3f are ALWAYS ON (system_context §6 invariant 1); NOT gated on
  token_limit.** The helper emits -M/-U UNCONDITIONALLY (it does NOT read opts.TokenLimit). At
  token_limit==0 (the default), the ONLY output change vs today is -U3→-U1 context (and -M, which is a
  no-op unless a rename is staged). This matches the item's "FR3e/FR3f are ALWAYS ON." M3 (skeleton) /
  M4 (water-fill) / FR3h (index-strip) are SEPARATE tasks — their opts fields are UNREAD here.

  ⚠️ **THE golden-fixture reality (verified by reading all 3 test files): every assertion is
  substring/structural — NONE assert on exact context-line shape.** -M won't trigger in any fixture (no
  delete+add-similar >50% pair is staged). -U changes only context-line COUNT, but the assertions check
  file presence (`Contains`), boundary markers (`Count "diff --git a/<file>"`), binary/excluded
  placeholders, and truncation sentinels — all retained at every -U level. So this task's fixture churn is
  expected to be ~ZERO; the item's "update golden fixtures" + system_context §6 L158 ("fixtures WILL
  change under FR3f") anticipate the CUMULATIVE M2+M3+M4 changes. Update strategy is RUN-DRIVEN: apply the
  helper, run the suites, fix only what breaks (keep boundary-marker + truncation-sentinel anchors).

  ⚠️ **-M scope = 3 PATCH paths ONLY; binary.go UNTOUCHED.** The item's "all three" = md-list/per-file/
  nmArgs (the patch argv). `binary.go`'s `detectBinaryFiles`/`fileStatuses` build their OWN numstat/
  name-status argv and are NOT routed through buildDiffArgs (P1.M2.T1.S1 left them alone; this task does
  too). WHY no -M on numstat: git_diff_semantics §3 — "-M makes numstat's path column use `=>`/`{...}`
  rename notation, harder to parse." Keep sizing/status parsing simple.

  Deliverable: edit `internal/git/git.go` (extend buildDiffArgs signature+body; update the 9 call sites to
  pass `opts`) + edit the 3 golden test files (run-driven: update only broken body-shape assertions; add
  `DiffContext: 1` to default-path body-shape tests per the struct doc) + add 2 new tests
  (`TestStagedDiff_RenameDetectedCompact` for -M; `TestStagedDiff_DiffContextZero_ChangedLinesOnly` for
  -U0; optionally a -U1 default-shape test). NO new files, NO new imports (`fmt` already imported), NO
  go.mod change, NO docs, NO binary.go, NO call-site edits. OUTPUT: every captured patch diff uses
  `-M -U<diff_context>`; renames are compact; context is reduced. Consumed as the baseline by M3/M4.
---

## Goal

**Feature Goal**: Make every captured `git diff` patch (StagedDiff/TreeDiff/WorkingTreeDiff, all 3 argv
paths each) pass `-M` (FR3e deterministic rename detection) and `-U<diff_context>` (FR3f reduced context,
default 1) by injecting both at the single shared `buildDiffArgs` helper — so a rename emits
`similarity index`/`rename from`/`rename to` (not a full-content delete+add) and unchanged context noise
is cut from git's -U3 default to -U1. Both are ALWAYS ON (not gated on token_limit).

**Deliverable** (edits to existing files):
1. **`internal/git/git.go`** — extend `buildDiffArgs(domain ...string)` → `buildDiffArgs(opts StagedDiffOptions, domain ...string)[]string`;
   body appends `-M` + `fmt.Sprintf("-U%d", ctx)` (ctx = `opts.DiffContext` clamped to [0,3], out-of-range→1)
   after the domain; update the 9 call sites (3 per function) to pass `opts`.
2. **`internal/git/{stagediff,treediff,workingtreediff}_test.go`** — run-driven: update any body-shape
   assertion that breaks (keep `diff --git` boundary-marker + truncation-sentinel anchors); set
   `DiffContext: 1` on default-path body-shape tests per the struct doc.
3. **NEW tests** in `stagediff_test.go` — `TestStagedDiff_RenameDetectedCompact` (asserts `rename from`/
   `rename to`, FR3e) and `TestStagedDiff_DiffContextZero_ChangedLinesOnly` (DiffContext:0 → changed-lines-
   only, FR3f); optionally a -U1 default-shape test.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/git/` clean;
`go test -race ./internal/git/` green (existing golden suites pass — most unchanged because they're
substring-based; any body-shape break updated) AND `go test -race ./...` green (no regression in
generate/decompose/hook). Concretely: every patch argv is `["diff", domain…, "-M", "-U<ctx>", …trailing]`;
a staged pure rename yields `rename from`/`rename to` (no delete+add body); DiffContext:1 → one anchor
context line; DiffContext:0 → changed lines only. go.mod/go.sum unchanged; only `internal/git/` touched.

## User Persona

**Target User**: The agent (model) consuming the diff payload — FR3e/FR3f cut its token cost and noise
(renames no longer duplicate full content; unchanged context lines trimmed from 3 to 1). Transitively PRD
§9.1 FR3e/FR3f (P0 diff-capture quality). Also the NEXT subtasks (M3 skeleton, M4 water-fill) which build
on this centralized argv site.

**Use Case**: A user renames a file (`git mv`) and edits a few lines across several files, then runs
stagehand. The diff payload shows the rename compactly (`rename from`/`rename to` + residual edit) and
each edit with one anchor context line — not a full delete+add of the renamed file plus 3-line context windows.

**User Journey**: `StagedDiff`/`TreeDiff`/`WorkingTreeDiff` → each `g.run` builds argv via
`buildDiffArgs(opts, domain…)` + trailing tokens → `git diff … -M -U1 …` → compact, low-noise patch →
(prompt construction) → agent.

**Pain Points Addressed**: removes the rename-duplication token bloat (a 500-line rename cost ~500 lines
without -M; ~3 lines with -M) and the unchanged-context-line noise (~28-44% payload reduction per FR3f).

## Why

- **FR3e/FR3f are P0 (§9.1) and ALWAYS ON (system_context §6).** -M is the only deterministic cross-version/
  config rename detector; -U1 is the recommended sweet spot (git_diff_semantics §2). This task lands both.
- **Single-site edit (the payoff of P1.M2.T1.S1).** Without the helper, -M/-U would land in 9 sites across
  3 near-verbatim functions — divergence-prone. The helper makes it ONE edit (+ 9 mechanical `opts` passes).
- **Deterministic + safe.** -M wins over `diff.renames=false` (verified git_diff_semantics §1); -U0/-U1
  compose with `--cached` and tree-to-tree (§2). Neither affects the snapshot/commit (payload-only,
  system_context §6 invariant 4). -C is rejected (O(files²), FR3e).
- **Back-compatible at the patch level.** -M is a no-op unless a rename is staged; -U only trims context.
  The commit is untouched; only what the agent SEES changes.
- **No new surface.** DOCS: none — the user-facing `diff_context` knob was documented in P1.M1.T1.S4.

## What

`buildDiffArgs` emits `-M -U<ctx>` (always on, ctx clamped to [0,3]) in all 3 patch argv paths of all 3
diff functions; the 9 call sites pass `opts`; renames render compactly; context is reduced. Golden fixtures
updated run-driven (keep boundary-marker + truncation anchors); 2 new positive tests added. No numstat/
name-status/-C/token-limit/index-strip/skeleton/call-site/doc changes.

### Success Criteria

- [ ] `buildDiffArgs(opts StagedDiffOptions, domain ...string) []string` returns
      `["diff", domain…, "-M", "-U<ctx>"]` where ctx = `opts.DiffContext` if ∈ [0,3] else 1 (defensive clamp).
- [ ] All 9 call sites pass `opts`: StagedDiff `buildDiffArgs(opts, "--cached")`; TreeDiff
      `buildDiffArgs(opts, treeA, treeB)`; WorkingTreeDiff `buildDiffArgs(opts)`. Each appends its EXACT
      current trailing tokens (byte-identical except the new -M/-U).
- [ ] -M/-U are UNCONDITIONAL (the helper does NOT read opts.TokenLimit) — always on at token_limit==0.
- [ ] `binary.go`'s `detectBinaryFiles`/`fileStatuses` UNCHANGED (no -M/-U on numstat/name-status).
- [ ] Existing golden suites pass: `go test ./internal/git/ -run 'TestStagedDiff|TestTreeDiff|TestWorkingTreeDiff'`
      green (substring/structural assertions unaffected; any body-shape break updated; boundary-marker +
      truncation-sentinel anchors kept).
- [ ] NEW `TestStagedDiff_RenameDetectedCompact`: a staged pure rename → output contains `rename from` +
      `rename to` (FR3e).
- [ ] NEW `TestStagedDiff_DiffContextZero_ChangedLinesOnly`: `DiffContext: 0` → changed-lines-only (an
      unchanged anchor line present at -U1 is absent at -U0); `diff_context:1` retains one anchor (FR3f).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/git/`, `go test -race ./internal/git/`,
      `go test -race ./...` clean/green; go.mod/go.sum unchanged; only `internal/git/` touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact helper before/after
(quoted), the -U0/default-1 resolution (3-source definitive), the run-driven fixture-update strategy, the
two new-test sketches, and the scope fences (binary.go untouched, no -C, always-on not token-gated). No
PRD/git-internals knowledge beyond "git diff -M -U<n>".

### Documentation & References

```yaml
# MUST READ — the authoritative research
- docfile: plan/007_b33d310438c6/P1M2T2S1/research/inject-M-and-U-context.md
  why: the helper before/after (§1), the DEFINITIVE -U0/default-1 resolution from 3 sources (§2), the
       golden-fixture reality (substring-based ⇒ ~zero churn for M2.T2; §3), the -M scope (3 patch paths
       only; binary.go untouched; §4), the 2+ new-test sketches (§5), system_context §6 invariants (§6),
       scope fences (§7), validation commands (§8).
  critical: §2 (0→-U0, 1→-U1 default, out-of-range→1) and §3 (run-driven fixture updates; don't blindly
       rewrite — most tests pass unchanged) are the two things most likely to be done wrong.

# The contract — what the parallel helper-refactor produced (assume LANDED)
- docfile: plan/007_b33d310438c6/P1M2T1S1/PRP.md
  why: P1.M2.T1.S1 created buildDiffArgs(domain ...string) []string and routed the 9 sites through it,
       byte-identical. THIS task extends that helper (signature + injection) — it is the "T2" that PRP
       repeatedly names as the -M/-U extender.
  critical: the 9 call sites currently are buildDiffArgs("--cached") / buildDiffArgs(treeA,treeB) /
       buildDiffArgs(); THIS task changes them to buildDiffArgs(opts, …). opts is in scope in all 3 functions.

# The git semantics (verified, with copy-pasteable repro commands)
- docfile: plan/007_b33d310438c6/architecture/git_diff_semantics.md
  section: "1. git diff -M" (pure-rename format: similarity index 100% / rename from / rename to, NO index
       line, NO patch body; -M wins over diff.renames=false; -C is O(files²), rejected) + "2. git diff -U<n>"
       (-U0 = changed lines only; -U1 = one anchor each side; compose with --cached + tree-to-tree) +
       "3. --numstat" (WHY no -M on numstat: the => / brace rename notation).
  critical: §1 — a pure rename emits NO patch body (just the extended header); §3 — keep -M OFF numstat.

# The regression invariant (acceptance criteria)
- docfile: plan/007_b33d310438c6/architecture/system_context.md
  section: "6. Regression invariants" (L151-159) + L91 (DiffContext default 1, 0 valid).
  why: FR3e/FR3f are ALWAYS ON (not token-gated); FR3f -U1 "CHANGES existing golden tests even at
       token_limit==0"; boundary-marker + truncation-sentinel fixtures are the stability anchors.
  critical: the helper MUST emit -M/-U unconditionally (do NOT gate on opts.TokenLimit).

# The file being edited
- file: internal/git/git.go
  section: StagedDiffOptions.DiffContext (L60-66, the "0 is VALID / callers MUST pass resolved default 1"
       doc); buildDiffArgs (immediately before StagedDiff, ~L668 — added by P1.M2.T1.S1); the 9 argv sites
       (StagedDiff md-list ~L686 / per-file ~L697 / nmArgs ~L771; TreeDiff ~L1136/1147/1210;
       WorkingTreeDiff ~L1270/1281/1345).
  why: the helper you extend + the 9 sites that gain an `opts` arg. opts is the function's parameter.
  pattern: each site swaps buildDiffArgs(domain…) → buildDiffArgs(opts, domain…); trailing tokens unchanged.
  gotcha: per-file site is in a loop — build fresh each iteration (append(buildDiffArgs(opts, domain…), trailing…)…);
       do NOT hoist+mutate. fmt is already imported (verify) — no new import.

# The golden test files (run-driven updates + 2 new tests)
- file: internal/git/stagediff_test.go   (+ treediff_test.go + workingtreediff_test.go)
  why: the golden suites. ALL assertions are substring/structural (Contains / Count / != "" / count != 1) —
       NONE on exact context-line shape (verified). So -M (no renames in fixtures) + -U (context-count only)
       leave them green; update only a body-shape break if one appears. Helpers: initRepo/writeFile/stageFile/execGit.
  pattern: add NEW TestStagedDiff_RenameDetectedCompact + TestStagedDiff_DiffContextZero_ChangedLinesOnly
       mirroring the existing fixture style (t.TempDir + initRepo + writeFile + stageFile + execGit).

- file: internal/git/binary.go
  section: detectBinaryFiles (~L98) + fileStatuses (~L130).
  why: PROOF binary.go is untouched — they build their own ["diff", diffArgs…, "--numstat"/"--name-status"]
       and are NOT routed through buildDiffArgs. No -M on numstat/name-status (git_diff_semantics §3).
  gotcha: do NOT add -M/-U here or route through buildDiffArgs. The 3 functions' calls to
       g.detectBinaryFiles(ctx, "--cached") etc. stay verbatim.

# The default-resolution proof (already DONE by P1.M1.T2.S2 — read-only)
- file: internal/config/config.go
  section: DiffContext *int (L82); Defaults() intPtr(1) (L175); DiffContextValue() (L201-205 — nil⇒1, *0⇒0).
  why: PROOF production always passes the resolved DiffContext (1 default) via the 6 call sites
       (generate.go:169, decompose.go:614, message.go:77, planner.go:75, hook/exec.go:110, …). So the helper's
       opts.DiffContext is 1 (default) or explicit [0,3] in production; only bare StagedDiffOptions{} tests get 0.
  critical: this is why the helper maps 0→-U0 literally (no sentinel hijack) — the resolution already happened upstream.
```

### Current Codebase tree (relevant slice — POST-P1.M2.T1.S1 assumed)

```bash
internal/git/
  git.go              # buildDiffArgs (pre-StagedDiff) + StagedDiff/TreeDiff/WorkingTreeDiff (9 sites) — EDIT (helper + 9 sites)
  binary.go           # detectBinaryFiles/fileStatuses (numstat/name-status) — NO edit
  stagediff_test.go   # ~23 substring golden tests + ADD 2 new tests — EDIT (run-driven + new)
  treediff_test.go    # ~12 substring golden tests — EDIT (run-driven, likely no change)
  workingtreediff_test.go # substring golden tests — EDIT (run-driven, likely no change)
  (other _test.go)    # difftree/difftreenames/binary — NO edit
internal/config/config.go   # DiffContext *int + DiffContextValue() — INPUT (read-only; already resolves default 1)
go.mod / go.sum      # unchanged (fmt already imported)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Edits to internal/git/git.go (helper + 9 sites) + the 3 _test.go files (run-driven + 2 new tests).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the -U0/default-1 resolution): opts.DiffContext is a PLAIN int carrying the RESOLVED value.
// 0 → -U0 (VALID, changed-lines-only); 1 → -U1 (the production default); 2/3 → -U2/-U3; out-of-range → 1
// (defensive). Do NOT hijack 0 as an "unset⇒default" sentinel — config already resolved nil⇒1 upstream
// (DiffContextValue). The helper's clamp is belt-and-suspenders for a malformed opts, not the default path.

// CRITICAL (always-on): emit -M and -U UNCONDITIONALLY. Do NOT gate on opts.TokenLimit. system_context §6:
// FR3e/FR3f are ALWAYS ON, even at token_limit==0. (M4 owns the token-limit gate; M3 owns the skeleton.)

// CRITICAL (-M on the 3 PATCH paths only): buildDiffArgs feeds md-list/per-file/nmArgs. binary.go's
// detectBinaryFiles/fileStatuses (numstat/name-status) are NOT routed through it — do NOT add -M there
// (git_diff_semantics §3: -M corrupts numstat's path column with => notation). Their call sites in the 3
// functions (g.detectBinaryFiles(ctx, "--cached") etc.) stay verbatim.

// CRITICAL (signature ripple): buildDiffArgs gains an `opts` param. ALL 9 call sites must pass it (opts is
// in scope in StagedDiff/TreeDiff/WorkingTreeDiff — it's the function's parameter). A missed site is a
// compile error (wrong arg count) — go vet/build catches it immediately.

// GOTCHA (per-file loop): build the argv fresh each iteration (append(buildDiffArgs(opts, domain…), "--", file)…);
// do NOT hoist a shared base and append into it (buildDiffArgs may return a slice with spare capacity).

// GOTCHA (fixtures are substring-based): do NOT blindly rewrite the golden tests. RUN them; fix only what
// breaks. The stability anchors (diff --git boundary markers, truncation sentinels, binary/excluded
// placeholders, file-presence Contains) are RETAINED at every -U level and unaffected by -M (no renames in
// fixtures). Expect ~zero churn for M2.T2 alone.

// GOTCHA (no -C): never add -C (copy detection). FR3e explicitly rejects it (O(files²), no value).

// GOTCHA (fmt import): the helper uses fmt.Sprintf("-U%d", ctx). fmt is almost certainly already imported
// in git.go (88KB; Fprintf/Sprintf used throughout) — verify; if not, add it (stdlib, no go.mod change).

// GOTCHA (test helpers): stagediff_test.go has initRepo/writeFile/stageFile/execGit (and sdManyLines).
// treediff/workingtreediff have their own equivalent helpers. Mirror them in the new tests — do NOT invent
// a new helper layer.
```

## Implementation Blueprint

### Data models and structure

No new types. The helper signature extension + body:

```go
// buildDiffArgs returns the leading argv for a `git diff`: ["diff", domain…, "-M", "-U<ctx>"].
// -M (FR3e) is ALWAYS ON — the only deterministic cross-version/config rename detector (wins over
// diff.renames=false; -C is rejected as O(files²)). -U<ctx> (FR3f) is the effective unified context:
// opts.DiffContext ∈ [0,3] used verbatim (0 = -U0 changed-lines-only; 1 = the production default);
// an out-of-range value defensively clamps to 1. Both are UNCONDITIONAL (system_context §6 invariant 1:
// always-on, not token-limit-gated). domain is the diff domain ("--cached" / treeA treeB / none). The
// caller appends its trailing tokens (--name-only / -- / pathspecs). Shared by StagedDiff/TreeDiff/
// WorkingTreeDiff (3 patch argv paths each); binary.go's numstat/name-status build their own argv
// (kept -M-free — git_diff_semantics §3).
func buildDiffArgs(opts StagedDiffOptions, domain ...string) []string {
	ctx := opts.DiffContext
	if ctx < 0 || ctx > 3 {
		ctx = 1
	}
	args := append([]string{"diff"}, domain...)
	args = append(args, "-M")
	args = append(args, fmt.Sprintf("-U%d", ctx))
	return args
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: git.go — extend buildDiffArgs (signature + body) and update the 9 call sites
  - EDIT buildDiffArgs: signature (opts StagedDiffOptions, domain ...string); body appends -M + -U<ctx>
    (ctx clamped) per the Data Models block; refresh the doc comment (FR3e/FR3f always-on; -C rejected;
    binary.go kept -M-free; T2 = this task).
  - EDIT the 9 call sites to pass opts:
      StagedDiff md-list:   append(buildDiffArgs(opts, "--cached"), "--name-only", "--", "*.md", "*.markdown")…
      StagedDiff per-file:  append(buildDiffArgs(opts, "--cached"), "--", file)…                  (fresh per iteration)
      StagedDiff nmArgs:    nmArgs := buildDiffArgs(opts, "--cached"); nmArgs = append(nmArgs, "--"); … (rest unchanged)
      TreeDiff:             buildDiffArgs(opts, treeA, treeB) at the 3 sites (md-list/per-file/nmArgs).
      WorkingTreeDiff:      buildDiffArgs(opts) at the 3 sites (empty domain).
  - VERIFY (mentally) each site's token order: ["diff", domain…, "-M", "-U<ctx>", …trailing] == today's
    argv + the two new flags before the trailing tokens.
  - GOTCHA: per-file loop builds fresh; all 9 sites pass opts; binary.go calls UNCHANGED; no -C; fmt imported.

Task 2: RUN the golden suites; update only body-shape breaks (run-driven)
  - RUN: go test ./internal/git/ -run 'TestStagedDiff|TestTreeDiff|TestWorkingTreeDiff' -v
  - EXPECTED: ~all pass unchanged (substring/structural assertions; no renames in fixtures; -U only trims
    context which the assertions don't check). If a test breaks (it implicitly depended on -U3 context),
    update its expectation — KEEP the `diff --git a/<file>` boundary-marker assertions + truncation-sentinel
    assertions (the stability anchors per system_context §6 L158). For a default-path body-shape test, set
    DiffContext: 1 (per the struct doc "callers MUST pass resolved default 1") so it exercises -U1.
  - DO NOT churn tests that pass (binary/excludes/truncation tests are fine at {} → -U0).

Task 3: ADD the rename (-M) positive test to stagediff_test.go
  - TestStagedDiff_RenameDetectedCompact: initRepo + commit a baseline old.go; stage a pure rename to new.go
    (identical content) via execGit add -A (delete old + add new); StagedDiff(DiffContext:1); assert output
    Contains "rename from" AND "rename to" (FR3e). (A pure rename = similarity index 100%, no patch body.)
  - Mirror the existing helpers (initRepo/writeFile/stageFile/execGit).

Task 4: ADD the -U0 (FR3f) positive test to stagediff_test.go
  - TestStagedDiff_DiffContextZero_ChangedLinesOnly: commit a.go with 3 funcs; edit the MIDDLE line only;
    stage. StagedDiff(DiffContext:0) → assert an unchanged anchor line ("func a()") is ABSENT (changed-lines-
    only). StagedDiff(DiffContext:1) → assert the SAME anchor line IS present (one context line each side).
    This contrast pins -U0 vs -U1 (FR3f) deterministically.
  - (Optional) TestStagedDiff_DiffContextOne_DefaultShape: pin the -U1 production default explicitly.

Task 5: VERIFY (no further edits)
  - RUN the full Validation Loop. go.mod/go.sum byte-unchanged. binary.go byte-unchanged. The 6 production
    call sites byte-unchanged (they already pass DiffContextValue()). generate/decompose/hook stay green
    (the patch they receive now has -M/-U1; their tests are substring/output-shape — fix any break).
```

### Implementation Patterns & Key Details

```go
// THE helper (the entire production change in miniature):
func buildDiffArgs(opts StagedDiffOptions, domain ...string) []string {
	ctx := opts.DiffContext
	if ctx < 0 || ctx > 3 {
		ctx = 1
	}
	args := append([]string{"diff"}, domain...)
	args = append(args, "-M", fmt.Sprintf("-U%d", ctx))
	return args
}
// StagedDiff nmArgs (before: nmArgs := []string{"diff","--cached","--"}):
nmArgs := buildDiffArgs(opts, "--cached")
nmArgs = append(nmArgs, "--")
nmArgs = append(nmArgs, excludes...)
nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
nmArgs = append(nmArgs, binExcludes...)
// → argv: ["diff","--cached","-M","-U1","--",excludes…,":!*.md",":!*.markdown",binExcludes…]

// THE always-on invariant — do NOT gate on token_limit:
// WRONG: if opts.TokenLimit > 0 { args = append(args, "-M", ...) }
// RIGHT: unconditional (system_context §6 — FR3e/FR3f always on, even at token_limit==0).
```

```go
// stagediff_test.go — the rename (-M) test (FR3e):
func TestStagedDiff_RenameDetectedCompact(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "old.go", "package main\nfunc a(){}\n")
	stageFile(t, repo, "old.go")
	execGit(t, repo, "commit", "-qm", "base") // baseline
	writeFile(t, repo, "new.go", "package main\nfunc a(){}\n") // identical content → pure rename
	stageFile(t, repo, "new.go")
	os.Remove(filepath.Join(repo, "old.go"))
	execGit(t, repo, "add", "-A") // stage delete-old + add-new (≥50% similar ⇒ -M sees a rename)
	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{DiffContext: 1})
	if err != nil {
		t.Fatalf("StagedDiff: %v", err)
	}
	if !strings.Contains(out, "rename from") || !strings.Contains(out, "rename to") {
		t.Fatalf("FR3e (-M): expected compact rename (rename from/to), got:\n%s", out)
	}
}

// stagediff_test.go — the -U0 (FR3f) test:
func TestStagedDiff_DiffContextZero_ChangedLinesOnly(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "package main\nfunc a(){}\nfunc b(){}\nfunc c(){}\n")
	stageFile(t, repo, "a.go")
	execGit(t, repo, "commit", "-qm", "base")
	writeFile(t, repo, "a.go", "package main\nfunc a(){}\nfunc B(){}\nfunc c(){}\n") // edit middle line
	stageFile(t, repo, "a.go")
	g := New(repo)
	out0, err := g.StagedDiff(context.Background(), StagedDiffOptions{DiffContext: 0})
	if err != nil {
		t.Fatalf("StagedDiff -U0: %v", err)
	}
	out1, err := g.StagedDiff(context.Background(), StagedDiffOptions{DiffContext: 1})
	if err != nil {
		t.Fatalf("StagedDiff -U1: %v", err)
	}
	if !strings.Contains(out1, "func a()") {
		t.Fatalf("DiffContext:1 (-U1) should retain an anchor context line, got:\n%s", out1)
	}
	if strings.Contains(out0, "func a()") {
		t.Fatalf("DiffContext:0 (-U0) should drop unchanged context (changed-lines-only), got:\n%s", out0)
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — no new dep; fmt already imported. go mod tidy MUST be a no-op.

PACKAGE EDGES: NONE added/removed. internal/git stays a leaf.

FROZEN / NOT-EDITED:
  - internal/git/binary.go (detectBinaryFiles/fileStatuses — numstat/name-status; kept -M-free per
    git_diff_semantics §3). Their call sites in the 3 functions stay verbatim.
  - internal/config/* (DiffContextValue already resolves nil⇒1; the 6 call sites already pass it).
  - The 6 production call sites (generate/hook/stagehand/decompose) — they already map cfg→opts.
  - opts.TokenLimit / opts.PromptReserveTokens — UNREAD here (M4 owns the token-limit gate + water-fill).
  - docs/* (DOCS: none; diff_context documented in P1.M1.T1.S4).

DOWNSTREAM ENABLED (do NOT implement here):
  - P1.M3 (FR3g numstat skeleton): builds on the centralized patch argv; uses a SEPARATE numstat git call
    (deliberately -M-free, §3).
  - P1.M4 (FR3d/FR3i token-limit gate + water-fill): consumes opts.TokenLimit/PromptReserveTokens; gates on
    token_limit>0. THIS task's -M/-U are unconditional (the gate wraps THIS, not the reverse).
  - P1.M2.T3.S1 (FR3h index-strip): post-capture ^index line filter across the 3 functions.

NO DATABASE / ROUTES / CONFIG / CLI / NEW FILES / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/git/git.go internal/git/stagediff_test.go internal/git/treediff_test.go internal/git/workingtreediff_test.go
test -z "$(gofmt -l internal/git/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/git/   # catches a missed opts arg (compile), an unused param, a broken append, a missing fmt import.
go build ./...           # whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm -M/-U live ONLY in buildDiffArgs (not scattered) and binary.go is -M-free:
grep -n '"-M"\|"-U' internal/git/git.go        # expect: only inside buildDiffArgs (+ doc)
grep -n '"-M"' internal/git/binary.go          # expect: no matches (numstat/name-status kept -M-free)
```

### Level 2: Unit Tests (Component Validation)

```bash
# The golden suites (run-driven — most pass unchanged; fix any body-shape break):
go test ./internal/git/ -run 'TestStagedDiff|TestTreeDiff|TestWorkingTreeDiff' -v
# Expected: PASS. Verify the 2 new tests:
#   TestStagedDiff_RenameDetectedCompact ......... output has "rename from"/"rename to" (FR3e)
#   TestStagedDiff_DiffContextZero_ChangedLinesOnly  -U0 drops anchor / -U1 keeps it (FR3f)
go test -race ./internal/git/      # full git suite (incl. binary/difftree/difftreenames)
go test -race ./...                # generate/decompose/hook consume the (now -M/-U1) patch — fix any output-shape break.
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagehand ./cmd/stagehand && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only internal/git/ changed:
git diff --name-only | grep -Ev '^internal/git/' && echo "UNEXPECTED file changed" || echo "only internal/git/ changed (good)"
# Confirm binary.go byte-unchanged:
git diff --exit-code internal/git/binary.go && echo "binary.go UNCHANGED (expected)"
# Smoke (optional): in a temp repo, stage a rename + an edit; stagehand --dry-run; confirm the payload
# shows "rename from/to" + 1-line context (FR3e/FR3f end-to-end through generate).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Determinism + cross-config check (git_diff_semantics §1 verification): in a temp repo, set
# `diff.renames=false` and confirm stagehand's captured diff STILL shows the compact rename (proves -M
# wins over the user's git config). Also confirm -U1 composes with tree-to-tree (TreeDiff) by staging a
# 2-commit decompose-style change. The unit tests above cover the common path; this is the cross-config
# belt-and-suspenders. golangci-lint: `make lint` (project-wide gate).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/git/`, `go mod tidy` no-op;
      -M/-U only in buildDiffArgs; binary.go -M-free; go.mod/go.sum byte-unchanged.
- [ ] Level 2 green: `go test -race ./internal/git/` (golden suites + 2 new tests) AND `go test -race ./...`.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only `internal/git/` changed; binary.go byte-unchanged.

### Feature Validation

- [ ] `buildDiffArgs(opts, domain…) []string` emits `["diff", domain…, "-M", "-U<ctx>"]` (ctx clamped [0,3]→1).
- [ ] All 9 call sites pass `opts`; trailing tokens byte-identical; -M/-U unconditional (not token-gated).
- [ ] `TestStagedDiff_RenameDetectedCompact`: pure rename → `rename from`/`rename to` (FR3e).
- [ ] `TestStagedDiff_DiffContextZero_ChangedLinesOnly`: DiffContext:0 drops anchor / DiffContext:1 keeps it (FR3f).
- [ ] binary.go's numstat/name-status unchanged (no -M); -C never added.

### Code Quality Validation

- [ ] Mirrors P1.M2.T1.S1's variadic helper + per-file-fresh-in-loop pattern; fmt already imported.
- [ ] Golden-fixture updates are run-driven (keep boundary-marker + truncation anchors); no blind rewrite.
- [ ] No scope creep into M3 (skeleton), M4 (token-limit/water-fill), M2.T3 (index-strip), binary.go, -C,
      call sites, or docs.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] No docs edits (DOCS: none — `diff_context` documented in P1.M1.T1.S4; P1.M5 owns the diff-capture doc sync).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ Don't gate -M/-U on `opts.TokenLimit`. FR3e/FR3f are ALWAYS ON (system_context §6 invariant 1), even at
  token_limit==0. M4 owns the token-limit gate; THIS task emits the flags unconditionally.
- ❌ Don't hijack `opts.DiffContext == 0` as an "unset⇒default-1" sentinel. 0 is VALID (-U0, changed-lines-only);
  the default-1 resolution already happened upstream (config.DiffContextValue, nil⇒1). The helper maps 0→-U0
  literally; only OUT-OF-RANGE values clamp to 1.
- ❌ Don't add `-M` to `binary.go`'s numstat/name-status (detectBinaryFiles/fileStatuses) or route them
  through buildDiffArgs. git_diff_semantics §3: -M corrupts numstat's path column (`=>`/`{...}` notation).
  The 3 functions' calls to g.detectBinaryFiles(ctx, "--cached") etc. stay verbatim.
- ❌ Don't add `-C` (copy detection). FR3e explicitly rejects it (O(files²), no value for message generation).
- ❌ Don't blindly rewrite the golden fixtures. RUN them first — they're substring/structural (no context-line
  assertions, no renames in fixtures) so ~all pass unchanged. Fix only a body-shape break; keep the
  `diff --git` boundary-marker + truncation-sentinel anchors (system_context §6 L158).
- ❌ Don't forget to pass `opts` at all 9 call sites. A missed site is a compile error (wrong arg count) —
  but verify all 3 functions' md-list/per-file/nmArgs are updated, not just one.
- ❌ Don't hoist a shared `buildDiffArgs` base outside the per-file loop and append into it — build fresh per
  iteration (`append(buildDiffArgs(opts, domain…), "--", file)…`).
- ❌ Don't change the trailing tokens at any site (md-list `--name-only -- *.md *.markdown`; per-file `-- <file>`;
  nmArgs `-- excludes :!*.md :!*.markdown binExcludes`). Only the base changes (gains `opts` + the flags).
- ❌ Don't edit the 6 production call sites, config, or docs. The cfg→opts mapping (DiffContextValue) is done;
  this task is function-internal to internal/git.
- ❌ Don't change go.mod/go.sum or add files. One helper extension + 9 site updates + run-driven test fixes +
  2 new tests, all in internal/git/.
- ❌ Don't skip `go vet`/`go build`/`go test -race ./...` — they catch a missed `opts` arg, a missing fmt
  import, and any generate/decompose/hook output-shape regression from the now--M/-U1 patch.

---

## Confidence Score

**9/10** — a single-site flag injection (the payoff of the parallel helper refactor) with a definitively
resolved -U0/default-1 design question (3 independent sources agree), a verified golden-fixture reality
(substring-based ⇒ ~zero churn for M2.T2; run-driven updates), concrete sketches for the 2 new tests, and
clear scope fences (binary.go untouched, no -C, always-on not token-gated, M3/M4/M2.T3 deferred). Every edit
site is pinned with the exact before/after. The -1 reserves for the run-driven fixture updates (a test could
surprisingly depend on -U3 context and need a small expectation tweak) and the optional -U1 default-shape test.
