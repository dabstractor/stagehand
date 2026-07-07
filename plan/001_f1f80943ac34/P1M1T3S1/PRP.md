---
name: "P1.M1.T3.S1 — StagedDiff with markdown caps, exclusions, and byte truncation"
description: |
  Replace the `(*gitRunner).StagedDiff` panic-stub (landed by P1.M1.T2.S1) with the real
  implementation — the staged-diff capture that feeds prompt construction and the provider executor
  (PRD §9.1/FR1–FR4, Appendix C, FINDING 7). Signature is fixed by the already-landed `Git`
  interface: `StagedDiff(ctx, opts StagedDiffOptions) (string, error)`. It performs commit-pi's
  two-part capture: (1) `git diff --cached --name-only -- '*.md' '*.markdown'` lists markdown files,
  then each is diffed individually (`git diff --cached -- '<file>'`) and capped at `max_md_lines`
  lines (split on `\n`, take first N; commit-pi default 100); (2) a single
  `git diff --cached -- '<excludes>'` for non-markdown with pathspec magic-prefix excludes for
  lock/snap/map/vendor noise PLUS the STRUCTURAL `:!*.md` `:!*.markdown` excludes (preventing
  markdown double-count — verified: without them markdown appears in BOTH sections), capped at
  `max_diff_bytes` bytes (commit-pi default 300000). Both caps are POST-capture (FINDING 7: git has
  no `--max-bytes`/`--max-lines`); over-cap sections get a `... [diff truncated at <N> lines/bytes]`
  sentinel. The two parts are concatenated (markdown first). Zero/negative caps in opts apply the
  commit-pi default constants (`defaultMaxMDLines=100`, `defaultMaxDiffBytes=300000`); a non-empty
  `opts.Excludes` REPLACES the default noise-filter set (markdown exclusion is always appended).
  Delegates to S1's `run()` helper for ALL exec (NOT `runWithInput` — diff reads no stdin); branches
  `err`-first (infrastructural, unwrapped), then `code != 0` (bad pathspec/corrupt repo ⇒ exit 128 ⇒
  wrapped `"git diff ...: failed"` error), then success (truncate + concatenate). `git diff` WITHOUT
  `--quiet` exits 0 on success whether or not changes exist — distinct from HasStagedChanges'
  `--quiet` exit-1-means-staged (FINDING 6, P1.M1.T3.S2). Introduces TWO package-level constants, ONE
  package-level `var`, and ZERO new imports (`fmt`, `strings` already present in git.go). Adds ONE
  new test file `internal/git/stagediff_test.go` (package git) covering: markdown+code, excludes
  (lock/snap/map/vendor absent), markdown-not-double-counted, markdown line cap + sentinel,
  non-markdown byte cap + sentinel, nothing-staged (empty), only-markdown / only-code, custom
  excludes override, zero-value defaults, markdown extensions (.md+.markdown), git-missing,
  ctx-cancelled. Helper prefix `sd` (collides with nothing: S4/S5/S6 use setIdentityConfig/cas/dt).
  Also removes the single `StagedDiff` line from `git_test.go`'s `TestStubsPanic` (required
  consequence of making the method real — mirrors S2/S3/S4/S5/S6). Touches ONLY `internal/git/`;
  no interface, struct, `run()`, `runWithInput`, `FileChange`, `StagedDiffOptions`, RevParseHEAD,
  WriteTree, CommitTree, UpdateRefCAS, DiffTree, or parseDiffTree changes. This is the FIRST of the
  six P1.M1.T3 methods (StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects, CommitCount,
  AddAll) to leave stub-status.
---

## Goal

**Feature Goal**: Implement the first P1.M1.T3 git method — `StagedDiff` — the staged-diff capture
that produces the single payload string consumed by prompt construction (P1.M3.T1.S3) and delivered
to the provider executor's stdin (P1.M2.T5). After the orchestrator (P1.M3.T4) confirms staged
changes via `HasStagedChanges` (P1.M1.T3.S2), it calls `diff, _ := g.StagedDiff(ctx, opts)` to obtain
the concatenated diff text, then embeds it in the user prompt and pipes it to the agent CLI. The
method reproduces commit-pi's exact capture semantics (PRD Appendix C: "Caps/exclusions identical,
configurable"): markdown files are captured per-file with a per-file line cap; all other files are
captured in one aggregate diff with pathspec excludes for lock/snapshot/sourcemap/vendor noise,
capped at a total byte budget — both caps applied post-capture because git has no native cap flag
(FINDING 7).

**Deliverable**:
1. **MODIFY** `internal/git/git.go`: (a) declare the two commit-pi default constants
   (`defaultMaxMDLines = 100`, `defaultMaxDiffBytes = 300000`) and the one default-excludes `var`
   immediately above the `StagedDiff` method; (b) replace the `StagedDiff` panic-stub body with the
   real two-part capture body (exact body in §Blueprint). NO import changes — `fmt`, `strings` are
   both already imported; `StagedDiffOptions` is already declared (S1); `run()` is already real (S1).
2. **CREATE** `internal/git/stagediff_test.go` (`package git`): the test matrix below (9–11 test
   functions) plus any `sd`-prefixed fixture helpers.
3. **MODIFY** `internal/git/git_test.go`: remove the single `assertPanics(t, "StagedDiff", …)` line
   from `TestStubsPanic` (required now that StagedDiff is real — mirrors S2/S3/S4/S5/S6).

No other files touched. No new dependencies. `go.mod`/`go.sum` unchanged.

**Success Definition**: `go build ./...`, `go vet ./...`, and `gofmt -l internal/git/` are clean;
`go test -race ./internal/git/` exits 0 with the new cases passing (plus S1's `run()` tests, S2–S5's
plumbing tests, and S6's DiffTree tests all still green); staging a `.md` + `.go` yields a payload
containing BOTH hunks; staging `.lock`/`.snap`/`.map`/`vendor/` files yields a payload with NONE of
them; a markdown file appears EXACTLY ONCE (never double-counted); a markdown file over the line cap
is truncated to N lines with the `... [diff truncated at <N> lines]` sentinel; a non-markdown
aggregate over the byte cap is truncated to N bytes with the `... [diff truncated at <N> bytes]`
sentinel; a nothing-staged repo returns `("", nil)`; a zero-value `StagedDiffOptions{}` applies the
100/300000 defaults; a custom `opts.Excludes` overrides the noise filters while keeping the markdown
exclusion; a missing git binary surfaces as a non-nil error mentioning "git binary not found"; a
cancelled context surfaces as `errors.Is(err, context.Canceled)`; `run()`, `runWithInput`, the `Git`
interface, `FileChange`, `StagedDiffOptions`, `RevParseHEAD`, `WriteTree`, `CommitTree`,
`UpdateRefCAS`, `DiffTree`, and `parseDiffTree` are byte-identical to their landed forms.

## User Persona

**Target User**: The P1.M3.T4 `CommitStaged` orchestrator (the primary caller), and — transitively —
the prompt builder (P1.M3.T1.S3) that embeds the diff in the user payload and the provider executor
(P1.M2.T5) that pipes it to the agent CLI's stdin.

**Use Case**: After `HasStagedChanges` (P1.M1.T3.S2) confirms the index differs from HEAD, the
orchestrator captures the diff: `diff, err := g.StagedDiff(ctx, cfg.DiffOpts())`. The returned
string is embedded verbatim into the user prompt (P1.M3.T1.S3: "instruction + diff + rejection
block") and/or piped to the agent's stdin (P1.M2.T5). The caps keep the payload within an agent's
context/arg limits (PRD §22.1 risk: "Large diffs exceed an agent's context or arg limits" →
mitigated by the 300 KB default + "diff truncated" notice); the excludes keep lock/snapshot noise
out of the model's view; the per-file markdown line cap keeps huge changelog-style `.md` files from
monopolizing the budget.

**User Journey**: `g := git.New(repoPath)` → `staged, _ := g.HasStagedChanges(ctx)` → (if staged)
`diff, _ := g.StagedDiff(ctx, opts)` → (prompt builder) `payload := instruction + "\n\n" + diff +
rejectionBlock` → (executor) `cmd.Stdin = strings.NewReader(payload)` (or the diff is embedded in
the `-p` prompt arg). The payload string is opaque to the builder/executor — they never parse it.

**Pain Points Addressed**: (1) Without caps, a large staged change would overflow the agent's context
window or exceed argv length limits — the byte/line caps + truncation sentinel bound the payload and
signal incompleteness to the model. (2) Without excludes, lock files (often 10k+ lines of JSON) and
snapshots would drown the actual code change. (3) Without the structural markdown exclusion, markdown
would be duplicated (captured both per-file and in the aggregate), wasting budget and confusing the
model. (4) Configurability (opts + future config P1.M1.T4) lets power users tune the caps without a
release.

## Why

- **PRD §9.1 / FR1–FR4 (Diff capture, P0 → G1, G3):** *"FR1. Capture the staged diff via
  `git diff --cached`. FR2. Markdown files: include full diff capped at N lines per file (default 100,
  configurable via `max_md_lines`). FR3. Non-markdown files: include diff with pathspec exclusions for
  lock files, snapshots, sourcemaps, vendored code, capped at N bytes total (default 300000,
  configurable via `max_diff_bytes`). FR4. Concatenate markdown diff and other diff into a single
  payload."* This method IS FR1–FR4, exactly.
- **PRD Appendix C (Line-by-line porting map):** the row *"staged-diff capture (md + other) |
  `internal/git/diff.go` `StagedDiff()` | Caps/exclusions identical, configurable."* — the caps and
  exclusions must match commit-pi verbatim; only the configurability is new.
- **PRD §22.1 (Risks):** *"Large diffs exceed an agent's context or arg limits (Low/Medium). Diff
  cap (300 KB default, configurable); stdin delivery avoids arg limits; surface a clear 'diff
  truncated' notice."* The byte cap + sentinel are the explicit mitigation; the truncation notice is
  the sentinel.
- **PRD §13 / §11.1 (Core IP):** StagedDiff is step 0' (the diff is captured AFTER the
  nothing-staged gate and BEFORE `write-tree`, feeding the generation step). It is read-only with
  respect to refs/objects — it mutates nothing.
- **Foundation for P1.M3.T1.S3 / P1.M2.T5:** the prompt builder and executor are blocked on a
  correct, bounded `StagedDiff`. This is the first of the six P1.M1.T3 methods to leave stub-status
  (the remaining five — HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll — are
  P1.M1.T3.S2–S5).

## What

`StagedDiff` performs two `run()`-delegated captures and concatenates them:

- **Part 1 (markdown, per-file, line-capped):** `git diff --cached --name-only -- '*.md' '*.markdown'`
  lists staged markdown files (git's pathspec globs, NOT shell globs — passed as literal `[]string`).
  For each listed file, `git diff --cached -- '<file>'` returns the full per-file diff; it is capped
  at `maxMDLines` lines (`strings.Split(diff, "\n")`, keep first N, re-`Join`) and, if truncated, a
  `"\n... [diff truncated at <N> lines]"` sentinel is appended. Each file's (possibly truncated)
  diff is written to a `strings.Builder` with a guaranteed trailing newline.
- **Part 2 (non-markdown, aggregate, byte-capped, excluded):** a single
  `git diff --cached -- '<excludes>'` where `<excludes>` = (`opts.Excludes` if non-empty, else the
  `defaultExcludes` noise-filter set) **+ always** `:!*.md`, `:!*.markdown` (structural — prevents
  double-count, verified in research §2.3). The aggregate stdout is capped at `maxDiffBytes` bytes
  (`s[:maxDiffBytes]`); if truncated, a `"\n... [diff truncated at <N> bytes]"` sentinel is appended.
  The result is appended to the builder.
- **Caps:** if `opts.MaxMDLines <= 0` → `defaultMaxMDLines` (100); if `opts.MaxDiffBytes <= 0` →
  `defaultMaxDiffBytes` (300000). (Decision D1: zero applies the commit-pi default, NOT unlimited —
  the field's aspirational "0 = unlimited" comment from S1 is superseded for v1.)
- **Branch order (mirrors S2–S6):** `err != nil` (LookPath miss / context cancelled / start failure;
  `exitCode == -1`) → return `"", err` unchanged. `code != 0` (bad pathspec / corrupt repo = exit 128
  — `git diff` WITHOUT `--quiet` exits 0 on success regardless of changes, §2.4) → return
  `"", fmt.Errorf("git diff ...: failed (exit %d): %s", ...)`. success → truncate + concatenate.
- **Empty repo (nothing staged):** both parts empty → return `("", nil)` (NOT an error).

No `git diff --quiet` (that's HasStagedChanges, P1.M1.T3.S2), no porcelain, no shell, no
`cmd.Dir`/`os.Chdir` (inherited from `run()`), no new types, no new error sentinels.

### Success Criteria

- [ ] `(*gitRunner).StagedDiff` body matches §Implementation Blueprint verbatim (no `panic`);
      delegates to `run()` (NOT `runWithInput`); branches `err`-first, then `code != 0`, then success.
- [ ] The two commit-pi default constants (`defaultMaxMDLines = 100`, `defaultMaxDiffBytes = 300000`)
      and the `defaultExcludes` `var` are declared immediately above the method.
- [ ] Part 1 args are EXACTLY `["diff", "--cached", "--name-only", "--", "*.md", "*.markdown"]`;
      per-file args are EXACTLY `["diff", "--cached", "--", file]`.
- [ ] Part 2 args are `["diff", "--cached", "--"]` + (excludes) + `":!*.md", ":!*.markdown"`; the
      markdown excludes are ALWAYS present regardless of `opts.Excludes`.
- [ ] Line cap: `strings.Split` on `"\n"`, keep first `maxMDLines`, `strings.Join` with `"\n"`, append
      the `... [diff truncated at <N> lines]` sentinel IFF the original exceeded the cap.
- [ ] Byte cap: `len(diff) > maxDiffBytes` ⇒ `diff[:maxDiffBytes]` + the
      `... [diff truncated at <N> bytes]` sentinel.
- [ ] Zero/negative `opts.MaxMDLines`/`opts.MaxDiffBytes` apply the defaults; non-empty
      `opts.Excludes` REPLACES the default noise-filter set.
- [ ] NO new imports in git.go (`fmt`, `strings` already present; sentinels via `fmt.Sprintf`).
- [ ] `run()`, `runWithInput`, `New`, `gitRunner`, the `Git` interface, `FileChange`,
      `StagedDiffOptions`, `RevParseHEAD` (S2), `WriteTree` (S3), `CommitTree` (S4),
      `UpdateRefCAS`/`ErrCASFailed` (S5), `DiffTree`/`parseDiffTree` (S6), and the other 5 method
      stubs (HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll) are
      byte-identical to their landed forms.
- [ ] `internal/git/stagediff_test.go` exists in `package git` with the test matrix below.
- [ ] `git_test.go`'s `TestStubsPanic` no longer contains the `StagedDiff` line (removed; 5 stubs
      remain: HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll).
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go test -race ./internal/git/` exits 0; all new cases pass and S1/S2/S3/S4/S5/S6's tests still
      pass.
- [ ] NO shell execution and NO `cmd.Dir`/`os.Chdir` in production code (inherited from S1).
- [ ] `go.mod`/`go.sum` unchanged; no new dependencies.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives: the exact module path; the exact three files to touch (and
the exact single line to remove from `git_test.go`); the exact `run()` contract (signature + the
`err==nil`-for-non-zero-exits invariant that the `code != 0` branch relies on); the exact constants,
default-excludes `var`, and the full `StagedDiff` body (verified-equivalent to throwaway git
invocations run against git 2.54.0); the empirically-pinned `git diff` behaviors (name-only lists md
globs via git pathspec; per-file diff; `:!` excludes as separate argv elements; WITHOUT `:!*.md` in
Part 2 markdown is double-counted; `git diff` sans `--quiet` exits 0 on success, 128 on error); the
exact test matrix with verified assertions and the reused helpers; and the exact validation commands
with expected results. No inference required.

### Documentation & References

```yaml
# MUST READ — the authoritative context
- file: PRD.md
  why: "§9.1/FR1–FR4 (the diff-capture functional requirements this method implements verbatim:
        git diff --cached; markdown capped at N lines/file default 100; non-markdown capped at N bytes
        default 300000 with pathspec excludes for lock/snap/map/vendor; concatenate into one payload);
        §22.1 (the 'Large diffs exceed context/arg limits' risk → mitigated by the cap + 'diff
        truncated' notice); Appendix C ('staged-diff capture (md + other) | StagedDiff() | Caps/
        exclusions identical, configurable'); §13/§11.1 (StagedDiff is the read-only diff capture
        feeding generation — mutates no ref/object)."
  critical: "This subtask owns ONLY the StagedDiff body + its constants/var + its tests + the one-line
             TestStubsPanic edit. Do NOT implement HasStagedChanges/RecentMessages/RecentSubjects/
             CommitCount/AddAll (those are P1.M1.T3.S2–S5), the prompt builder (P1.M3.T1.S3), the
             executor (P1.M2.T5), or the orchestrator (P1.M3.T4). Do NOT change the Git interface or
             StagedDiffOptions (already correct from S1). Do NOT modify run()/runWithInput."

- docfile: plan/001_f1f80943ac34/architecture/critical_findings.md
  why: "FINDING 7 (Diff byte-capping has no built-in git flag — capture then truncate; append a
        '\n... [diff truncated at N bytes/lines]' sentinel) is THE spec for both caps. FINDING 6
        (git diff --cached --quiet exit 1 = staged) is owned by P1.M1.T3.S2, NOT this subtask — but
        it clarifies why StagedDiff uses the NON-quiet form (exit 0 on success regardless of changes)."
  critical: "FINDING 7's two-part structure (markdown per-file line cap; non-markdown aggregate byte
             cap with the exact pathspec exclude list) is the direct ancestor of this implementation.
             The sentinel text in FINDING 7 is the seed for decision D4's enriched sentinel."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_summary.md
  why: "The 'Diff Capture (§9.1, matching commit-pi)' Go-pattern block: (1) markdown per-file
        `git diff --cached -- '<file>'` → head -n max_md_lines; (2) non-markdown single
        `git diff --cached -- ':!*.lock' ':!package-lock.json' ':!pnpm-lock.yaml' ':!yarn.lock'
        ':!*.snap' ':!*.map' ':!vendor/*' ':!*.md' ':!*.markdown'` → cap at max_diff_bytes + sentinel;
        (3) concatenate. Also the Cross-Platform Notes (git -C repo; args as []string NEVER sh -c)."
  critical: "Confirms: the pathspec excludes are passed as SEPARATE argv elements (safe with exec —
             no shell), and the caps are POST-capture. The exact exclude list here is the source of
             defaultExcludes (plus the structural md excludes)."

- docfile: plan/001_f1f80943ac34/P1M1T2S1/PRP.md
  why: "The CONTRACT for everything StagedDiff consumes: the gitRunner struct; the run() helper (exact
        signature and verified body — which this subtask does NOT modify, only calls); the StagedDiff
        panic-stub being replaced; the Git interface (signature already correct:
        StagedDiff(ctx, StagedDiffOptions) (string, error)); the StagedDiffOptions type (MaxDiffBytes,
        MaxMDLines int; Excludes []string — already declared); New(); the git_test.go initRepo helper;
        and the assertPanics helper."
  critical: "Treat as already-landed. run() returns (stdout, stderr, exitCode, err) with err==nil for
             non-zero exits and exitCode==-1 for infrastructural failures. S1 gotcha G2 (non-zero exit
             is not a Go error) is the foundation StagedDiff's `code != 0` branch relies on."

- docfile: plan/001_f1f80943ac34/P1M1T2S6/PRP.md
  why: "The CONTRACT for the concurrently-landing DiffTree: confirms S6's git.go edit (DiffTree body +
        parseDiffTree) and git_test.go edit (remove DiffTree line from TestStubsPanic) are DISTINCT
        regions/lines from T3.S1's edits (StagedDiff body immediately after parseDiffTree; remove the
        StagedDiff line from TestStubsPanic). S6's helpers (dtCommit, dtRemove) use a `dt` prefix;
        T3.S1's use `sd` — no collision. S6's parseDiffTree precedent (a private helper tested in
        isolation) informed decision D7 (inline the trivial name-only split instead)."
  critical: "S6 has ALREADY LANDED on the current disk snapshot (DiffTree is real, parseDiffTree
             present, TestStubsPanic lists 6 stubs). T3.S1 builds on that exact state. No text overlap
             with S6's edits — the StagedDiff stub sits immediately after parseDiffTree."

- docfile: plan/001_f1f80943ac34/P1M1T3S1/research/stagediff_validation.md
  why: "THIS subtask's own research: the signature reconciliation (§1, none needed — all types
        already landed); the empirically-pinned git diff behavior on git 2.54.0 incl. the markdown
        double-count trap (§2.3), the exit-code semantics (§2.4: non-quiet diff exits 0/128), and the
        post-capture cap mechanics (§2.5); the seven design decisions D1–D7 (§3); the edge-case table
        (§4); and the test matrix (§5)."
  critical: "§2.3 (the double-count trap — `:!*.md` MUST be in Part 2) and §2.4 (git diff sans
             --quiet exits 0 on success) are the two non-obvious calls an implementing agent would
             otherwise guess wrong. §3 D1 (zero ⇒ default, NOT unlimited) and D2 (md exclusion is
             structural, always appended) resolve the two design tensions in the S1 field comments."

# External references (exact, anchor-level)
- url: https://git-scm.com/docs/git-diff#_description
  why: "Documents `git diff --cached` (diff of staged changes vs HEAD) and that without --quiet it
        writes the patch to stdout and exits 0 on success. Confirms the non-quiet exit semantics
        StagedDiff relies on (§2.4)."
  critical: "Establishes that StagedDiff's diff calls exit 0 whether or not there are changes — so
             `code != 0` ⟺ real error (bad pathspec/corrupt repo). Distinct from --quiet (exit 1 =
             staged), which is HasStagedChanges' concern."
- url: https://git-scm.com/docs/gitglossary#Documentation/gitglossary.txt-aiddefpathspecapathspec
  why: "Documents the pathspec magic syntax: `:(exclude)pattern` (short form `:!pattern`) excludes
        matches; `*` is a glob. Confirms the `:!*.lock` / `:!vendor/*` excludes and the `*.md` /
        `*.markdown` globs are git-interpreted (NOT shell), so passing them as []string args is
        correct and shell-free."
  critical: "Establishes that each `:!pattern` and `*.glob` is ONE argv element (no shell quoting
             needed). The double-count trap (§2.3) is why `:!*.md` must appear in Part 2."
- url: https://pkg.go.dev/strings#Builder
  why: "strings.Builder is the efficient concatenation target for the payload (markdown hunks +
        non-markdown aggregate). WriteString/WriteByte/String are the only methods used."
  critical: "Builder avoids the O(n²) cost of `+`-concatenating many markdown hunks. The final
             String() is the return value."
- url: https://pkg.go.dev/strings#Split
  why: "strings.Split(s, '\n') splits the diff into lines for the markdown cap; Split('', '\n')
        returns [''] (one empty element), which is why the file-list loop skips empties."
  critical: "The skip-empty guard (for the name-only file list) handles the nothing-staged case
             (empty mdList) cleanly — no spurious empty-string file diff."
```

### Current Codebase Tree (after S1 + S2 + S3 + S4 + S5 + S6 have landed — verified on disk)

```bash
stagecoach/
├── PRD.md
├── go.mod                # module github.com/dustin/stagecoach, go 1.22, NO deps
├── Makefile              # build/test/lint/coverage/install/clean (test = go test -race ./...)
├── cmd/stagecoach/main.go # stub
├── internal/
│   └── git/
│       ├── git.go        # S1: interface+gitRunner+run()+New()+FileChange+StagedDiffOptions+stubs;
│       │                 #   S2: RevParseHEAD real; S3: WriteTree real; S4: runWithInput+CommitTree
│       │                 #   real+io import; S5: ErrCASFailed+UpdateRefCAS real; S6: DiffTree real+
│       │                 #   parseDiffTree. imports: bytes, context, errors, fmt, io, os/exec, strings
│       │                 #   ← ALL T3.S1 needs present (fmt, strings)
│       ├── git_test.go   # S1: TestNew/TestRun_*/TestStubsPanic + initRepo + assertPanics
│       │                 #   TestStubsPanic currently lists 6 stubs: StagedDiff, HasStagedChanges,
│       │                 #   RecentMessages, RecentSubjects, CommitCount, AddAll
│       ├── revparse_test.go   # S2: minGitEnv + makeEmptyCommit + 4 tests
│       ├── writetree_test.go  # S3: makeMergeConflict + 5 tests
│       ├── committree_test.go # S4: setIdentityConfig/writeFile/stageFile/writeTreeOf/headSHA/
│       │                       #   commitMessage + 6 tests
│       ├── updateref_test.go  # S5: cas*/gitIdentityEnv + 6 tests
│       └── difftree_test.go   # S6: dtCommit/dtRemove + 9 tests
└── (other empty internal/ dirs, pkg/, providers/, docs/ — untouched by this subtask)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── internal/
    └── git/
        ├── git.go              # MODIFIED — StagedDiff stub → constants+var+real body. NO import change.
        ├── git_test.go         # MODIFIED — remove the ONE `StagedDiff` line from TestStubsPanic
        ├── revparse_test.go    # UNCHANGED (S2's file; minGitEnv/makeEmptyCommit reused, not edited)
        ├── writetree_test.go   # UNCHANGED (S3's file)
        ├── committree_test.go  # UNCHANGED (S4's file; writeFile/stageFile reused, not redeclared)
        ├── updateref_test.go   # UNCHANGED (S5's file)
        ├── difftree_test.go    # UNCHANGED (S6's file)
        └── stagediff_test.go   # NEW — package git; the test matrix below (+ any sd-prefixed helpers)
```

**File responsibilities:**

| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | MODIFY | (1) Declare `defaultMaxMDLines`, `defaultMaxDiffBytes` constants and `defaultExcludes` var immediately above `StagedDiff`. (2) Replace the `StagedDiff` panic-stub with the two-part capture body. No import changes. |
| `internal/git/git_test.go` | MODIFY | Remove the single `assertPanics(t, "StagedDiff", …)` line from `TestStubsPanic`. Nothing else. |
| `internal/git/stagediff_test.go` | CREATE | `package git` tests for `StagedDiff`: markdown+code, excludes applied, no double-count, line cap + sentinel, byte cap + sentinel, nothing-staged, only-markdown / only-code, custom excludes override, zero-value defaults, markdown extensions, git-missing, ctx-cancelled. |

**Explicitly NOT created/modified:** `run()`, `runWithInput`, `New`, `gitRunner`, the `Git`
interface, `FileChange`, `StagedDiffOptions`, `RevParseHEAD` (S2), `WriteTree` (S3), `CommitTree`
(S4), `UpdateRefCAS`/`ErrCASFailed` (S5), `DiffTree`/`parseDiffTree` (S6), the other 5 method stubs
(HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll), `revparse_test.go` (S2),
`writetree_test.go` (S3), `committree_test.go` (S4), `updateref_test.go` (S5), `difftree_test.go`
(S6), `go.mod`/`go.sum`, the `Makefile`, anything under `cmd/`/`pkg/`/other `internal/*`, `plan/`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — the markdown double-count trap): git's pathspec `:!pattern` EXCLUDES matches. If
// Part 2's non-markdown aggregate does NOT include `:!*.md` `:!*.markdown`, then markdown files
// (already captured per-file in Part 1) appear AGAIN in Part 2 — duplicating them in the payload.
// VERIFIED (research §2.3): a repo with app.go + many.md yielded TWO `+++` lines in Part 2 without
// `:!*.md`, ONE (app.go) with it. RESOLUTION: the markdown excludes are ALWAYS appended to Part 2,
// regardless of opts.Excludes — they are STRUCTURAL (prevent duplication), not a noise filter. A
// caller cannot disable them via opts. (Decision D2.)

// CRITICAL (G2 — git diff WITHOUT --quiet exits 0 on success, 128 on error): unlike
// `git diff --cached --quiet` (exit 1 = staged, FINDING 6 — owned by P1.M1.T3.S2 HasStagedChanges),
// the plain `git diff --cached [--name-only] [-- <pathspecs>]` form exits 0 whether or not there are
// changes. It exits 128 (non-zero) ONLY on a real error (bad pathspec, corrupt repo, etc.). So
// StagedDiff branches on `code != 0` ⟺ error. Do NOT treat empty output (nothing staged) as an error.
// (Research §2.4.)

// CRITICAL (G3 — caps are POST-capture, FINDING 7): git has no `--max-bytes` or `--max-lines` flag.
// commit-pi pipes through `head -c` / `head -n`. In Go: capture run()'s full stdout to a string,
// THEN truncate. Markdown: strings.Split(diff, "\n"), keep first maxMDLines, strings.Join with "\n".
// Non-markdown: if len(diff) > maxDiffBytes, take diff[:maxDiffBytes]. A byte slice MAY split a
// multi-byte UTF-8 rune — this matches `head -c` exactly; acceptable v1 tradeoff (model tolerates it).

// CRITICAL (G4 — zero/negative cap applies the DEFAULT, NOT unlimited): the StagedDiffOptions field
// comments (S1) say "0 = unlimited". This PRP SUPERSEDES that for v1: opts.MaxMDLines <= 0 ⇒
// defaultMaxMDLines (100); opts.MaxDiffBytes <= 0 ⇒ defaultMaxDiffBytes (300000). Rationale: the item
// description mandates commit-pi-default constants; unbounded capture is a context-window footgun
// (PRD §22.1); the config system (P1.M1.T4) populates 100/300000 anyway. An explicit "unlimited"
// opt-in is not needed for v1. (Decision D1.)

// CRITICAL (G5 — delegate to run(), NOT runWithInput): diff reads nothing from stdin (unlike
// commit-tree's -F - message). So StagedDiff calls g.run(ctx, g.workDir, args...) for ALL THREE
// captures (the name-only list, each per-file diff, and the non-markdown aggregate). runWithInput
// (S4) exists solely to feed commit-tree's stdin. Using it here would be semantically wrong (nil
// reader) and wasteful. (Mirrors S5's and S6's run()-not-runWithInput decision.)

// CRITICAL (G6 — opts.Excludes REPLACES the default set; markdown exclusion is additive): if
// opts.Excludes is non-empty, it REPLACES defaultExcludes (the lock/snap/map/vendor noise filters) —
// enabling a future config knob to customize the noise set. But `:!*.md` `:!*.markdown` are ALWAYS
// appended afterward (structural — see G1), so a caller who supplies opts.Excludes cannot
// accidentally re-introduce the double-count. Build Part 2 args as: ["diff","--cached","--"] +
// (opts.Excludes if non-empty else defaultExcludes) + [":!*.md", ":!*.markdown"]. (Decision D2.)

// GOTCHA (G7 — the TestStubsPanic edit): git_test.go's TestStubsPanic (after S2–S6 removed their
// lines) STILL includes
//   assertPanics(t, "StagedDiff", func() { _, _ = g.StagedDiff(ctx, StagedDiffOptions{}) }).
// Once StagedDiff is real (no panic), assertPanics fails with "expected panic, but did not panic".
// Resolution (mirrors S2/S3/S4/S5/S6): DELETE that one line. After removal, TestStubsPanic covers
// the remaining 5 stubs (HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll).
// This is the ONLY edit to git_test.go.

// GOTCHA (G8 — ZERO new imports): StagedDiff uses g.run (S1), fmt.Errorf/Sprintf (fmt — present),
// strings.Builder/Split/Join/TrimSpace/HasSuffix (strings — present). StagedDiffOptions is already
// declared in git.go (S1). git.go's import block (bytes, context, errors, fmt, io, os/exec, strings)
// already has everything. Do NOT add an import (strconv NOT needed — use fmt.Sprintf("%d", n) for the
// sentinels). The compiler errors on an unused import if you add one erroneously.

// GOTCHA (G9 — test helper names must not collide with S2/S3/S4/S5/S6's): S2 defines minGitEnv,
// makeEmptyCommit; S4 defines setIdentityConfig, writeFile, stageFile, writeTreeOf, headSHA,
// commitMessage; S5 defines cas*, gitIdentityEnv; S6 defines dtCommit, dtRemove. T3.S1's NEW helpers
// (if any) use an `sd` prefix. REUSE (do not redeclare) initRepo (git_test.go) and writeFile +
// stageFile (committree_test.go). For staged-diff tests NO commits are needed (only staged files),
// so setIdentityConfig/makeEmptyCommit are not required — but if a test stages into a repo that
// already has a HEAD, that's fine too (diff --cached works against HEAD or empty tree).

// GOTCHA (G10 — filename globs are GIT pathspecs, not shell globs): pass "*.md", "*.markdown", and
// every ":!pattern" as LITERAL []string args. exec.CommandContext does NOT invoke a shell, so the
// asterisks and colons reach git unmodified and git's pathspec matcher interprets them. Do NOT quote
// or escape them. VERIFIED (research §2.1/§2.3): the globs and excludes work via direct exec args.

// GOTCHA (G11 — the `--` separator precedes the pathspecs): every diff invocation places `--`
// between the options and the pathspecs to guard pathspec-like filenames:
//   name-only: ["diff","--cached","--name-only","--","*.md","*.markdown"]
//   per-file:  ["diff","--cached","--",file]
//   aggregate: ["diff","--cached","--"] + excludes + [":!*.md",":!*.markdown"]
// The `--` ensures a staged file literally named `-foo.md` is treated as a path, not an option.

// GOTCHA (G12 — test file is package git, white-box): StagedDiff is on *gitRunner and the constants/
// var are package-private; the fixtures reuse unexported helpers. The test MUST be `package git`.
// Match S1/S2/S3/S4/S5/S6's package (carried from S1 G9).

// GOTCHA (G13 — no shell, no cmd.Dir in PRODUCTION code): StagedDiff inherits S1's §19 guarantees
// (run() uses exec.CommandContext + []string args + -C repo flag, NOT cmd.Dir / os.Chdir). Do NOT
// introduce exec.Command / os.Chdir / sh -c in git.go. The test fixtures DO use exec.Command directly
// (parallel to S1's initRepo, S4's writeFile/stageFile helpers) — that is acceptable test-fixture
// usage ([]string args + cmd.Env, never a shell). The Level-1 grep for sh -c / cmd.Dir covers
// PRODUCTION code (git.go) only.

// GOTCHA (G14 — memory: full stdout captured before capping): run() captures the ENTIRE stdout into
// a bytes.Buffer before StagedDiff truncates. A pathological multi-MB markdown file is fully
// materialized in memory before the line cap. This matches commit-pi's `git diff | head -n` (the
// pipe still buffers). Acceptable for v1: diffs are bounded by repo size and the orchestrator gates
// on HasStagedChanges. An io.LimitedReader approach would require restructuring run() — out of scope.
// (FINDING 7 explicitly says cap post-capture.)

// GOTCHA (G15 — nothing-staged returns ("", nil), NOT an error): when nothing is staged, the
// name-only list is empty (Part 1 skipped), and the non-markdown aggregate is empty (Part 2 appends
// ""). The builder's String() returns "". Return ("", nil). The orchestrator gates on
// HasStagedChanges BEFORE calling StagedDiff, but StagedDiff must still be safe unconditionally.
// (Research §4.)

// GOTCHA (G16 — the truncation sentinel is appended AFTER the truncated content, not replacing it):
// for the markdown line cap: keep lines[:N], Join with "\n", then append "\n... [diff truncated at
// <N> lines]". For the byte cap: take diff[:maxDiffBytes], then append "\n... [diff truncated at
// <N> bytes]". The sentinel length itself is NOT counted against the cap (the cap bounds the diff
// content; the sentinel is a small annotation). This means the final string can be slightly longer
// than maxDiffBytes — acceptable and intended (the model needs the notice). (Decision D4.)
```

## Implementation Blueprint

### Data models and structure

No new types. `StagedDiffOptions` is already declared by S1 (`MaxDiffBytes`, `MaxMDLines int`;
`Excludes []string`) and is the input this method consumes. The THREE new symbols are package-level
constants/var:

```go
// commit-pi defaults for staged-diff capture (PRD §9.1 / FINDING 7). Applied when the caller passes
// a zero/negative cap in StagedDiffOptions — guaranteeing commit-pi parity for any caller (the
// config system P1.M1.T4 populates these from resolved config; here we enforce the floor).
const (
	defaultMaxMDLines   = 100   // per-file line cap for markdown (commit-pi default)
	defaultMaxDiffBytes = 300000 // byte cap on the non-markdown aggregate (commit-pi default)
)

// defaultExcludes is the commit-pi noise-filter pathspec set (lock files, snapshots, sourcemaps,
// vendored code). Used when StagedDiffOptions.Excludes is empty; a non-empty opts.Excludes REPLACES
// it. The structural markdown excludes (":!*.md", ":!*.markdown") are appended SEPARATELY in the
// non-markdown section (always, regardless of opts.Excludes) because markdown is captured per-file in
// Part 1 — omitting them would duplicate markdown in the payload (the double-count trap, G1).
var defaultExcludes = []string{
	":!*.lock", ":!package-lock.json", ":!pnpm-lock.yaml", ":!yarn.lock",
	":!*.snap", ":!*.map", ":!vendor/*",
}
```

No new options type, no new struct, no new error sentinel. `run()`'s return shape
`(stdout, stderr, exitCode, err)` is already defined by S1.

### The `StagedDiff` body (exact — copy verbatim)

This replaces S1's panic-stub of the same name and signature. Place the constants + var + method
immediately AFTER `parseDiffTree` (S6) and BEFORE the `HasStagedChanges` stub.

```go
// StagedDiff returns the concatenated staged-diff payload for prompt construction and stdin delivery
// to the agent CLI (PRD §9.1/FR1–FR4, Appendix C, FINDING 7). It mirrors commit-pi's two-part
// capture:
//
//  1. Markdown files (.md, .markdown): `git diff --cached --name-only -- '*.md' '*.markdown'` lists
//     them (git pathspec globs, NOT shell globs — passed as []string), then each is diffed
//     individually (`git diff --cached -- '<file>'`) and capped at max_md_lines lines (split on
//     "\n", take the first N). A per-file truncation sentinel marks over-cap files so the model knows
//     the diff is partial.
//  2. Non-markdown files: a single `git diff --cached -- <excludes>` with pathspec magic-prefix
//     excludes for lock/snapshot/sourcemap/vendor noise (defaultExcludes, overridable via
//     opts.Excludes) PLUS the structural markdown excludes (":!*.md", ":!*.markdown") so markdown is
//     not double-counted (verified: without them markdown appears in BOTH sections). The aggregate is
//     capped at max_diff_bytes bytes.
//
// Caps are POST-capture (FINDING 7: git has no --max-bytes/--max-lines). Zero/negative caps apply the
// commit-pi defaults (100/300000). The two parts are concatenated (markdown first). An empty repo
// (nothing staged) yields "" with no error — the caller gates on HasStagedChanges first, but
// StagedDiff is safe to call unconditionally.
//
// `git diff` WITHOUT --quiet exits 0 on success whether or not there are changes (distinct from
// HasStagedChanges' --quiet exit-1-means-staged); a non-zero exit (128) is a real error (bad
// pathspec, corrupt repo) and is surfaced as a wrapped error.
func (g *gitRunner) StagedDiff(ctx context.Context, opts StagedDiffOptions) (string, error) {
	maxMDLines := opts.MaxMDLines
	if maxMDLines <= 0 {
		maxMDLines = defaultMaxMDLines
	}
	maxDiffBytes := opts.MaxDiffBytes
	if maxDiffBytes <= 0 {
		maxDiffBytes = defaultMaxDiffBytes
	}

	var b strings.Builder

	// ---- Part 1: markdown, per-file, line-capped ----
	// "*.md" / "*.markdown" are git pathspec globs (interpreted by git, not the shell — G10); the "--"
	// guards pathspec-like filenames (G11).
	mdList, stderr, code, err := g.run(ctx, g.workDir,
		"diff", "--cached", "--name-only", "--", "*.md", "*.markdown")
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		return "", fmt.Errorf("git diff (markdown list): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	for _, file := range strings.Split(strings.TrimSpace(mdList), "\n") {
		if file == "" {
			continue // nothing-staged ⇒ mdList is "" ⇒ Split yields [""] ⇒ skipped (G15)
		}
		fileDiff, fstderr, fcode, ferr := g.run(ctx, g.workDir, "diff", "--cached", "--", file)
		if ferr != nil {
			return "", ferr
		}
		if fcode != 0 {
			return "", fmt.Errorf("git diff --cached -- %s: failed (exit %d): %s", file, fcode, strings.TrimSpace(fstderr))
		}
		// Per-file line cap (post-capture, FINDING 7/G3). Split on "\n", keep first maxMDLines.
		if lines := strings.Split(fileDiff, "\n"); len(lines) > maxMDLines {
			fileDiff = strings.Join(lines[:maxMDLines], "\n") +
				fmt.Sprintf("\n... [diff truncated at %d lines]", maxMDLines)
		}
		b.WriteString(fileDiff)
		if !strings.HasSuffix(fileDiff, "\n") {
			b.WriteByte('\n') // ensure a clean boundary before the next hunk / Part 2
		}
	}

	// ---- Part 2: non-markdown, aggregate, byte-capped, excluded ----
	// opts.Excludes REPLACES the noise-filter default if non-empty (G6); the markdown excludes are
	// ALWAYS appended (structural — prevents the double-count trap, G1).
	excludes := opts.Excludes
	if len(excludes) == 0 {
		excludes = defaultExcludes
	}
	nmArgs := []string{"diff", "--cached", "--"}
	nmArgs = append(nmArgs, excludes...)
	nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
	nmDiff, nmstderr, nmcode, nmerr := g.run(ctx, g.workDir, nmArgs...)
	if nmerr != nil {
		return "", nmerr
	}
	if nmcode != 0 {
		return "", fmt.Errorf("git diff (non-markdown): failed (exit %d): %s", nmcode, strings.TrimSpace(nmstderr))
	}
	// Byte cap (post-capture, FINDING 7/G3). len() is byte length; the slice may split a UTF-8 rune —
	// matches `head -c` (G3). The sentinel is appended AFTER the cap and is not counted against it.
	if len(nmDiff) > maxDiffBytes {
		nmDiff = nmDiff[:maxDiffBytes] +
			fmt.Sprintf("\n... [diff truncated at %d bytes]", maxDiffBytes)
	}
	b.WriteString(nmDiff)

	return b.String(), nil
}
```

> **Verified:** the args shapes, the exclude list, the structural md exclusion, the post-capture cap
> mechanics, and the exit-code semantics are confirmed by this subtask's research §2 (re-verified
> empirically against git 2.54.0): name-only lists md globs; per-file diff; `:!` excludes as separate
> argv elements; WITHOUT `:!*.md` markdown is double-counted; non-quiet diff exits 0/128.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go (constants + var + method body — NO import change)
  - EDIT — replace the StagedDiff panic-stub:
      FIND the stub:
        func (g *gitRunner) StagedDiff(ctx context.Context, opts StagedDiffOptions) (string, error) {
            panic("gitRunner.StagedDiff: not yet implemented — see P1.M1.T3.S1")
        }
      REPLACE with: the `defaultMaxMDLines`/`defaultMaxDiffBytes` const block + the `defaultExcludes`
      var + the `StagedDiff` method (with its doc comment), verbatim from §"The StagedDiff body"
      above. Keep the same signature. Place this block immediately AFTER `parseDiffTree` (S6) and
      BEFORE the `HasStagedChanges` stub.
  - DO NOT touch: the import block (all needed symbols already present — gotcha G8), run(),
    runWithInput, New, gitRunner, Git interface, FileChange, StagedDiffOptions, RevParseHEAD (real
    from S2), WriteTree (real from S3), CommitTree (real from S4), UpdateRefCAS/ErrCASFailed (real
    from S5), DiffTree/parseDiffTree (real from S6), or any of the other 5 method stubs
    (HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll).
  - VERIFY: go build ./internal/git/ → exit 0.

Task 2: MODIFY internal/git/git_test.go (one-line removal)
  - FIND inside TestStubsPanic:
      assertPanics(t, "StagedDiff", func() { _, _ = g.StagedDiff(ctx, StagedDiffOptions{}) })
  - DELETE that single line. After removal TestStubsPanic covers the remaining 5 stubs:
    HasStagedChanges, RecentMessages, RecentSubjects, CommitCount, AddAll.
  - DO NOT touch anything else in git_test.go (initRepo, TestNew, TestRun_*, assertPanics helper,
    the other assertPanics lines).
  - WHY: once StagedDiff is real it no longer panics; assertPanics would fail (gotcha G7). Mirrors
    S2/S3/S4/S5/S6's removals.
  - VERIFY: go test -race -run TestStubsPanic ./internal/git/ → exit 0 (5 stubs still panic).

Task 3: CREATE internal/git/stagediff_test.go (package git — white-box)
  - FILE: internal/git/stagediff_test.go
  - PACKAGE line: `package git`  (NOT git_test — gotcha G12; matches the other test files)
  - IMPORTS: context, errors, strings, testing  (all stdlib; os/os/exec only if a helper needs them)
  - REUSE (do NOT redeclare): initRepo (git_test.go), writeFile + stageFile (committree_test.go).
  - WRITE the test functions (assertions in §"Test cases" below). Use `sd`-prefixed names for any
    NEW helper (gotcha G9). For generating many-line markdown content, inline a loop or
    strings.Repeat — e.g. a helper:
      sdManyLines(n int) string:
        var b strings.Builder; for i := 0; i < n; i++ { fmt.Fprintf(&b, "line %d\n", i) }; return b.String()
      (or inline; if a helper is added, prefix it `sd` and put fmt in the test imports.)
  - TEST MATRIX (11 functions — consolidate where natural):
      TestStagedDiff_MarkdownAndCode:
        repo := t.TempDir(); initRepo(t, repo)
        writeFile(t, repo, "a.md", "# Title\n\nbody\n"); stageFile(t, repo, "a.md")
        writeFile(t, repo, "b.go", "package main\n"); stageFile(t, repo, "b.go")
        g := New(repo)
        out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
        assert err == nil
        assert strings.Contains(out, "a.md")   // markdown hunk present
        assert strings.Contains(out, "b.go")   // code hunk present
      TestStagedDiff_ExcludesLockSnapMapVendor:
        repo := t.TempDir(); initRepo(t, repo)
        writeFile(t, repo, "keep.go", "package main\n"); stageFile(t, repo, "keep.go")
        writeFile(t, repo, "pkg.lock", "{}\n"); stageFile(t, repo, "pkg.lock")
        writeFile(t, repo, "package-lock.json", "{}\n"); stageFile(t, repo, "package-lock.json")
        writeFile(t, repo, "x.snap", "snap\n"); stageFile(t, repo, "x.snap")
        writeFile(t, repo, "y.map", "{}\n"); stageFile(t, repo, "y.map")
        os.MkdirAll(filepath.Join(repo, "vendor"), 0o755)
        writeFile(t, repo, "vendor/lib.go", "package vendor\n"); stageFile(t, repo, "vendor/lib.go")
        g := New(repo)
        out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
        assert err == nil
        assert strings.Contains(out, "keep.go")               // code kept
        assert !strings.Contains(out, "pkg.lock")             // .lock excluded
        assert !strings.Contains(out, "package-lock.json")    // package-lock excluded
        assert !strings.Contains(out, "x.snap")               // .snap excluded
        assert !strings.Contains(out, "y.map")                // .map excluded
        assert !strings.Contains(out, "vendor/lib.go")        // vendor/* excluded
      TestStagedDiff_MarkdownNotDoubleCounted:
        repo := t.TempDir(); initRepo(t, repo)
        writeFile(t, repo, "only.md", "# Hi\n\ncontent\n"); stageFile(t, repo, "only.md")
        g := New(repo)
        out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
        assert err == nil
        // "only.md" should appear (in the md section) but the file should NOT be duplicated.
        assert strings.Contains(out, "only.md")
        assert strings.Count(out, "diff --git a/only.md b/only.md") == 1   // EXACTLY ONE hunk (G1)
      TestStagedDiff_MarkdownLineCap:
        repo := t.TempDir(); initRepo(t, repo)
        // markdown content that yields > 10 diff lines
        writeFile(t, repo, "big.md", sdManyLines(50)); stageFile(t, repo, "big.md")
        g := New(repo)
        out, err := g.StagedDiff(context.Background(), StagedDiffOptions{MaxMDLines: 10})
        assert err == nil
        assert strings.Contains(out, "... [diff truncated at 10 lines]")   // sentinel present
        // the big.md hunk portion (everything before any non-md content) has ≤ 11 lines (10 + sentinel)
        mdPart := out // (no non-md files staged, so out is purely the md hunk)
        assert strings.Count(mdPart, "\n") <= 11
      TestStagedDiff_NonMarkdownByteCap:
        repo := t.TempDir(); initRepo(t, repo)
        writeFile(t, repo, "big.go", "package main\n"+strings.Repeat("// line\n", 2000)); stageFile(t, repo, "big.go")
        g := New(repo)
        out, err := g.StagedDiff(context.Background(), StagedDiffOptions{MaxDiffBytes: 100})
        assert err == nil
        assert strings.Contains(out, "... [diff truncated at 100 bytes]")  // sentinel present
        // the non-md section is capped: the diff content before the sentinel is ≤ 100 bytes; total
        // length is bounded (cap + sentinel). Assert len(out) is small and the truncation occurred.
        assert len(out) < 200   // well under the untruncated ~10KB+ diff
      TestStagedDiff_NothingStaged:
        repo := t.TempDir(); initRepo(t, repo)   // fresh repo, nothing staged
        g := New(repo)
        out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
        assert err == nil
        assert out == ""                                  // empty payload, no error (G15)
      TestStagedDiff_OnlyMarkdown:
        repo := t.TempDir(); initRepo(t, repo)
        writeFile(t, repo, "a.md", "# x\n"); stageFile(t, repo, "a.md")
        g := New(repo)
        out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
        assert err == nil && strings.Contains(out, "a.md")
      TestStagedDiff_OnlyCode:
        repo := t.TempDir(); initRepo(t, repo)
        writeFile(t, repo, "a.go", "package x\n"); stageFile(t, repo, "a.go")
        g := New(repo)
        out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
        assert err == nil && strings.Contains(out, "a.go")
      TestStagedDiff_CustomExcludesOverride:
        repo := t.TempDir(); initRepo(t, repo)
        writeFile(t, repo, "keep.go", "package main\n"); stageFile(t, repo, "keep.go")
        writeFile(t, repo, "drop.go", "package drop\n"); stageFile(t, repo, "drop.go")
        g := New(repo)
        // opts.Excludes REPLACES the default set — so ":!drop.go" is honored (G6); md exclusion still
        // appended (no .md staged, so just verify drop.go is gone and keep.go remains).
        out, err := g.StagedDiff(context.Background(), StagedDiffOptions{Excludes: []string{":!drop.go"}})
        assert err == nil
        assert strings.Contains(out, "keep.go")
        assert !strings.Contains(out, "drop.go")
      TestStagedDiff_DefaultsOnZero:
        repo := t.TempDir(); initRepo(t, repo)
        writeFile(t, repo, "small.md", "# tiny\n"); stageFile(t, repo, "small.md")
        writeFile(t, repo, "small.go", "package main\n"); stageFile(t, repo, "small.go")
        g := New(repo)
        out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})   // zero-value
        assert err == nil                                   // no panic; defaults applied (D1)
        assert !strings.Contains(out, "truncated")          // small diffs under default caps → no sentinel
        assert strings.Contains(out, "small.md") && strings.Contains(out, "small.go")
      TestStagedDiff_MarkdownExtensions:
        repo := t.TempDir(); initRepo(t, repo)
        writeFile(t, repo, "a.md", "# md\n"); stageFile(t, repo, "a.md")
        writeFile(t, repo, "b.markdown", "# markdown\n"); stageFile(t, repo, "b.markdown")
        g := New(repo)
        out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
        assert err == nil
        assert strings.Contains(out, "a.md") && strings.Contains(out, "b.markdown")  // both globs match
      TestStagedDiff_GitBinaryMissing:
        t.Setenv("PATH", "")                               // makes run()'s LookPath("git") fail
        g := New(t.TempDir())                              // dir need not be a repo
        out, err := g.StagedDiff(context.Background(), StagedDiffOptions{})
        assert err != nil && strings.Contains(err.Error(), "git binary not found")
        assert out == ""
      TestStagedDiff_ContextCancelled:
        ctx, cancel := context.WithCancel(context.Background()); cancel()  // cancel BEFORE call
        g := New(t.TempDir())
        out, err := g.StagedDiff(ctx, StagedDiffOptions{})
        assert err != nil && errors.Is(err, context.Canceled)
        assert out == ""
  - NAMING: TestStagedDiff_<Scenario>; helper sdManyLines (sd prefix — gotcha G9).
  - DO NOT redeclare initRepo / writeFile / stageFile (they live in git_test.go / committree_test.go).
  - VERIFY: go test -race -run TestStagedDiff ./internal/git/ → exit 0, all 11 pass.

Task 4: VALIDATE — full gate set + scope discipline
  - RUN: go build ./... ; go vet ./... ; gofmt -l internal/git/ ; go test -race ./internal/git/
  - RUN: git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go   (expect: no matches)
  - RUN: git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                    (expect: no matches)
  - RUN: git grep -n 'panic.*StagedDiff' internal/git/git.go                    (expect: no matches — stub gone)
  - RUN: git grep -n 'defaultMaxMDLines\|defaultMaxDiffBytes\|defaultExcludes' internal/git/git.go
        (expect: defaultMaxMDLines once, defaultMaxDiffBytes once, defaultExcludes once as var + reads)
  - RUN: git grep -n 'func (g \*gitRunner) StagedDiff' internal/git/git.go      (expect: exactly 1 match)
  - RUN: git grep -n 'runWithInput' internal/git/git.go | grep -i stagediff     (expect: no matches — uses run())
  - RUN: git grep -n 'strconv' internal/git/git.go                              (expect: no matches — fmt.Sprintf used)
  - RUN: git status --porcelain → expect EXACTLY:
        M internal/git/git.go
        M internal/git/git_test.go
        ?? internal/git/stagediff_test.go
        (If difftree_test.go / updateref_test.go / other files appear as M/??, S5/S6 landed and are
        already committed or in-flight — coordinate; T3.S1 itself touches only the three files above.)
```

### Test cases (the assertion matrix)

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestStagedDiff_MarkdownAndCode` | stage `a.md` + `b.go` | err nil; payload has both `a.md` and `b.go` hunks | the two-part concatenation (FR4) |
| `TestStagedDiff_ExcludesLockSnapMapVendor` | stage `keep.go` + `.lock` + `package-lock.json` + `.snap` + `.map` + `vendor/lib.go` | `keep.go` present; all 5 noise types ABSENT | the pathspec excludes (FR3) |
| `TestStagedDiff_MarkdownNotDoubleCounted` | stage `only.md` | exactly ONE `diff --git a/only.md` hunk | the structural `:!*.md` exclude (G1) |
| `TestStagedDiff_MarkdownLineCap` | `big.md` (50 lines); `MaxMDLines:10` | sentinel `... [diff truncated at 10 lines]`; ≤11 lines | the post-capture line cap (FR2/D4) |
| `TestStagedDiff_NonMarkdownByteCap` | `big.go` (large); `MaxDiffBytes:100` | sentinel `... [diff truncated at 100 bytes]`; bounded len | the post-capture byte cap (FR3/D4) |
| `TestStagedDiff_NothingStaged` | fresh repo | `("", nil)` | empty-payload success (G15) |
| `TestStagedDiff_OnlyMarkdown` | only `a.md` | md present | Part 2 empty when only md staged |
| `TestStagedDiff_OnlyCode` | only `a.go` | code present | Part 1 empty when only code staged |
| `TestStagedDiff_CustomExcludesOverride` | `keep.go` + `drop.go`; `Excludes:[":!drop.go"]` | `keep.go` present, `drop.go` absent | opts.Excludes replaces default set (G6) |
| `TestStagedDiff_DefaultsOnZero` | small `a.md` + `a.go`; zero opts | no panic; no "truncated" sentinel | defaults applied on zero (D1) |
| `TestStagedDiff_MarkdownExtensions` | `a.md` + `b.markdown` | both captured | both globs match (FR2) |
| `TestStagedDiff_GitBinaryMissing` | `t.Setenv("PATH","")` | err has "git binary not found"; `out==""` | run()'s LookPath branch |
| `TestStagedDiff_ContextCancelled` | cancel ctx before call | `errors.Is(err, context.Canceled)`; `out==""` | run()'s ctx.Err() branch |

### Implementation Patterns & Key Details

```go
// === The run() invariant in action (mirrors S2–S6) ===
// StagedDiff does, for each capture:
//   stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
//   if err != nil { return "", err }                       // LookPath/start/ctx failure only
//   if code != 0 { return "", fmt.Errorf("git diff ...: failed (exit %d): %s", code, ...) }
//   // success — process stdout
// Note err is nil at code != 0 — that is the whole point of the four-tuple return (S1 gotcha G2).

// === Why opts.Excludes REPLACES (not appends to) the default set (G6/D2) ===
// A future config knob may want to customize the ENTIRE noise-filter set. Appending would make
// the defaults un-removable. So: empty opts.Excludes ⇒ use defaultExcludes; non-empty ⇒ use the
// caller's slice wholesale. The markdown excludes are ALWAYS appended afterward (structural — G1).

// === Why the byte cap uses len() and a byte slice (G3) ===
// Go strings are byte sequences; len(s) is the byte length; s[:n] slices by bytes. This matches
// `head -c 300000` exactly. A multi-byte UTF-8 rune may be split at the boundary — acceptable
// (commit-pi has the same behavior; the model tolerates a partial final line).

// === Why the sentinel is appended AFTER the cap, not counted against it (G16) ===
// The cap bounds the DIFF CONTENT; the sentinel is a small annotation the model needs. So the final
// string can be slightly longer than maxDiffBytes (by the sentinel's length). This is intended —
// the alternative (counting the sentinel against the cap) would silently drop diff content to make
// room for the notice, which defeats the purpose.

// === Why a strings.Builder (not +=) ===
// Part 1 may write many markdown hunks; += on strings is O(n²) over the hunks. Builder is O(n).
// The final b.String() is the return value.
```

### Integration Points

```yaml
MODULE (consumed, not modified):
  - module path: "github.com/dustin/stagecoach" → package import path "github.com/dustin/stagecoach/internal/git"
  - go directive: 1.22 → context, os/exec, errors.As, strings.Builder, t.Setenv (1.17+) all available
  - deps: NONE added (stdlib only)

INTERNAL/GIT PACKAGE (modified here):
  - file: internal/git/git.go           # constants + var + StagedDiff body (stub → real)
  - file: internal/git/git_test.go      # remove the StagedDiff line from TestStubsPanic
  - file: internal/git/stagediff_test.go # NEW — package git white-box tests

CALLERS (informational — do NOT implement now; each consumes StagedDiff's string return):
  - P1.M1.T3.S2: HasStagedChanges — the gate called BEFORE StagedDiff (exit 1 ⇒ staged)
  - P1.M3.T1.S3: prompt builder — embeds the diff in the user payload (instruction + diff + rejection)
  - P1.M2.T5:    provider executor — pipes the payload to the agent CLI's stdin
  - P1.M3.T4:    CommitStaged orchestrator — calls HasStagedChanges then StagedDiff
  - P1.M1.T4:    config system — will populate StagedDiffOptions{MaxMDLines, MaxDiffBytes} (and
                 optionally Excludes) from resolved config (defaults 100/300000); until it lands,
                 callers pass StagedDiffOptions{} and the method applies the commit-pi defaults.

LATER-SUBTASK HOOKS (the remaining 5 P1.M1.T3 stubs — do NOT implement now):
  - P1.M1.T3.S2: HasStagedChanges (git diff --cached --quiet; exit 1 ⇒ true)
  - P1.M1.T3.S3: RecentMessages + CommitCount (NUL-delimited; rev-list --count)
  - P1.M1.T3.S4: RecentSubjects (%s one-per-line)
  - P1.M1.T3.S5: AddAll (git add -A)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/git/          # Expected: no output (run `gofmt -w internal/git/` if it lists files)
go vet ./internal/git/...        # Expected: exit 0, no warnings
go build ./internal/git/         # Expected: exit 0 (package compiles standalone)
go build ./...                   # Expected: exit 0 (whole module compiles)

# Expected: Zero output/errors. gofmt is the formatting gate (golangci-lint is absent locally —
# see gotcha / S2's PRP); `go vet` catches shadowed vars, unreachable code, etc.
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

go test -race -v -run TestStagedDiff ./internal/git/   # Expected: all StagedDiff tests PASS, exit 0
# Must see: TestStagedDiff_MarkdownAndCode, _ExcludesLockSnapMapVendor, _MarkdownNotDoubleCounted,
#           _MarkdownLineCap, _NonMarkdownByteCap, _NothingStaged, _OnlyMarkdown, _OnlyCode,
#           _CustomExcludesOverride, _DefaultsOnZero, _MarkdownExtensions, _GitBinaryMissing,
#           _ContextCancelled — all ok.

go test -race -run TestStubsPanic ./internal/git/      # Expected: exit 0 (5 stubs still panic)

go test -race ./internal/git/                          # Expected: exit 0 (full package green)
# Or via the Makefile target:
make test                                              # Expected: exit 0 (runs go test -race ./...)

# Expected: all pass. If TestStagedDiff_ExcludesLockSnapMapVendor fails because a noise file appears,
# re-check the pathspec spelling (each ":!pattern" is one arg; the "--" precedes them all).
```

### Level 3: Security & Structural Invariants (the §19 enforcement)

```bash
cd /home/dustin/projects/stagecoach

# PRD §19: NO shell execution anywhere in the git wrapper.
git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go
# Expected: NO output (no matches). A match is a §19 violation.

# No os.Chdir / cmd.Dir — repo is targeted via the -C flag only (goroutine-safe).
git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go
# Expected: NO output (no matches).

# StagedDiff delegates to run() (NOT runWithInput).
git grep -n 'runWithInput' internal/git/git.go | grep -i stagediff
# Expected: NO output (no matches).

# No strconv import (sentinels use fmt.Sprintf).
git grep -n 'strconv' internal/git/git.go
# Expected: NO output (no matches).

# The stub is gone and the constants/var/method exist.
git grep -n 'panic.*StagedDiff' internal/git/git.go        # expect: no matches
git grep -n 'defaultMaxMDLines' internal/git/git.go        # expect: 1 match (the const)
git grep -n 'defaultMaxDiffBytes' internal/git/git.go      # expect: 1 match (the const)
git grep -n 'var defaultExcludes' internal/git/git.go      # expect: 1 match
git grep -n 'func (g \*gitRunner) StagedDiff' internal/git/git.go  # expect: 1 match
```

### Level 4: Runtime Smoke Test (prove the capture works against a real repo)

```bash
cd /home/dustin/projects/stagecoach

# Build a tiny throwaway program that exercises StagedDiff exactly as the tests do, to confirm the
# real-binary path end-to-end (mirrors the research verification):
cat > /tmp/smoke_stagediff.go <<'EOF'
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// (imports the package under test via go run from the module — see below)
func main() {
	dir, _ := os.MkdirTemp("", "smoke")
	defer os.RemoveAll(dir)
	for _, args := range [][]string{
		{"init"}, {"config", "user.email", "t@t.com"}, {"config", "user.name", "t"},
	} {
		exec.Command("git", append([]string{"-C", dir}, args...)...).Run()
	}
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# t\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "p.lock"), []byte("{}\n"), 0o644)
	exec.Command("git", "-C", dir, "add", "-A").Run()

	// Invoke the real git diff commands StagedDiff runs, to eyeball the payload:
	out, _ := exec.Command("git", "-C", dir, "diff", "--cached", "--name-only", "--", "*.md", "*.markdown").Output()
	fmt.Printf("md list: %q\n", string(out))
	out2, _ := exec.Command("git", "-C", dir, "diff", "--cached", "--",
		":!*.lock", ":!*.snap", ":!*.map", ":!vendor/*", ":!*.md", ":!*.markdown").Output()
	fmt.Printf("non-md agg (len=%d):\n%s\n", len(out2), string(out2))
	_ = context.Background()
}
EOF
go run /tmp/smoke_stagediff.go
# Expected: md list = "a.md\n"; non-md agg shows b.go only (p.lock excluded), NOT a.md (structural md exclude).
rm -f /tmp/smoke_stagediff.go
# This proves the pathspec globs, the excludes, and the structural md exclusion end-to-end.
```

## Final Validation Checklist

### Technical Validation

- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go vet ./...` exits 0.
- [ ] `go build ./...` exits 0.
- [ ] `go test -race ./internal/git/` exits 0 (all StagedDiff tests + S1–S6's tests pass).
- [ ] `make test` exits 0 (the S2 target runs the same `-race` suite).

### Feature Validation

- [ ] `(*gitRunner).StagedDiff` body matches §Blueprint verbatim (no `panic`); delegates to `run()`.
- [ ] `defaultMaxMDLines` (100), `defaultMaxDiffBytes` (300000) constants and `defaultExcludes` var
      declared immediately above the method.
- [ ] Part 1 uses `--name-only` + per-file diff; Part 2 uses the excludes + structural md exclusion.
- [ ] Markdown over the line cap is truncated with the `... [diff truncated at <N> lines]` sentinel.
- [ ] Non-markdown over the byte cap is truncated with the `... [diff truncated at <N> bytes]` sentinel.
- [ ] Zero-value `StagedDiffOptions{}` applies the 100/300000 defaults.
- [ ] `opts.Excludes` (non-empty) REPLACES the default noise set; markdown exclusion always appended.
- [ ] Nothing-staged returns `("", nil)`.
- [ ] Lock/snap/map/vendor files are excluded from the payload.
- [ ] Markdown appears EXACTLY ONCE (never double-counted).
- [ ] git-binary-missing → err contains "git binary not found".
- [ ] context-cancelled → `errors.Is(err, context.Canceled)`.

### Security & Scope Discipline Validation

- [ ] NO `sh -c` / `zsh -c` / `bash -c` / `cmd /c` anywhere (`git grep` Level 3 → no matches).
- [ ] NO `cmd.Dir` field set, NO `os.Chdir` call (`git grep` Level 3 → no matches).
- [ ] NO `strconv` import; sentinels use `fmt.Sprintf`.
- [ ] No new dependencies; `go.mod` unchanged; no `go.sum` generated.
- [ ] Created ONLY `internal/git/stagediff_test.go`; MODIFIED only `internal/git/git.go` and
      `internal/git/git_test.go` (`git status --porcelain`).
- [ ] Did NOT implement any of the other 5 method stubs (those are P1.M1.T3.S2–S5).
- [ ] Did NOT touch `run()`/`runWithInput`/the interface/`StagedDiffOptions`/`FileChange`/the
      S2–S6 methods or their test files.
- [ ] Did NOT touch `go.mod`, `Makefile`, `cmd/`, other `internal/*`, `pkg/`, `providers/`, or `plan/`.

---

## Anti-Patterns to Avoid

- ❌ Don't OMIT `:!*.md` `:!*.markdown` from Part 2 — markdown would be captured twice (per-file in
  Part 1 AND in the aggregate). The markdown excludes are STRUCTURAL, always appended (gotcha G1).
- ❌ Don't turn a non-zero `git diff` exit into a Go `err` — `git diff` (without `--quiet`) exits 0
  on success whether or not there are changes; only LookPath/context/start failures set `err`
  (gotcha G2). Branch on `code != 0` for real errors.
- ❌ Don't use `git diff --cached --quiet` here — that's HasStagedChanges' concern (P1.M1.T3.S2,
  FINDING 6: exit 1 = staged). StagedDiff captures the patch text (non-quiet form).
- ❌ Don't add a `--max-bytes`/`--max-lines` git flag — it doesn't exist. Caps are POST-capture
  (FINDING 7, gotcha G3).
- ❌ Don't treat `opts.MaxMDLines == 0` as "unlimited" — apply the commit-pi default (100). Zero
  caps are a footgun (unbounded capture overflows context windows); the config system populates the
  defaults anyway (gotcha G4, decision D1).
- ❌ Don't append `opts.Excludes` to `defaultExcludes` — REPLACE the default set when opts.Excludes is
  non-empty (so a future config can fully customize the noise filters). The markdown exclusion is the
  only always-appended part (gotcha G6, decision D2).
- ❌ Don't count the truncation sentinel against the byte cap — the cap bounds the diff content; the
  sentinel is appended after (gotcha G16, decision D4).
- ❌ Don't delegate to `runWithInput` — diff reads no stdin; use `run()` (gotcha G5).
- ❌ Don't add `strconv` (or any import) — `fmt.Sprintf("%d", n)` builds the sentinels and `fmt`/
  `strings` are already imported (gotcha G8, decision D6).
- ❌ Don't quote/escape the pathspec globs (`*.md`, `:!*.lock`) — they are git-interpreted, passed as
  literal `[]string` args; no shell is involved (gotcha G10).
- ❌ Don't forget the `--` separator before pathspecs — it guards pathspec-like filenames (gotcha G11).
- ❌ Don't redeclare `initRepo`/`writeFile`/`stageFile` in the test — reuse them (gotcha G9); use an
  `sd` prefix for any new helper.
- ❌ Don't implement HasStagedChanges/RecentMessages/RecentSubjects/CommitCount/AddAll, the prompt
  builder, the executor, or the orchestrator here — those are P1.M1.T3.S2–S5 / P1.M3 / P1.M2.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: The `StagedDiff` body, the constants/var, and the args shapes are verified-equivalent to
throwaway git invocations executed against the exact installed git (2.54.0) — the two-part capture,
the `:!` pathspec excludes as separate argv elements, the structural markdown exclusion (the
double-count trap, empirically confirmed in research §2.3), the post-capture line/byte caps, and the
non-quiet exit semantics (exit 0/128, §2.4) are all pinned. The `run()` contract (S1), the
`StagedDiffOptions` type (S1), and the established S2–S6 method pattern (err-first → code-branch →
success) mean there is exactly one idiomatic way to write the body, and it is specified verbatim.
The test matrix is tied to empirically-confirmed behaviors. The only residual uncertainty (not 10/10)
is environmental: the exact per-file diff line count depends on git's hunk formatting, so the
line-cap test uses an over-provisioned fixture (50 content lines → well over a 10-line cap) with a
loose bound (`≤ 11` lines) rather than an exact count; and the byte-cap test uses a generous
`< 200` upper bound rather than an exact value, to avoid brittleness across git versions. The
"no-shell / no-cmd.Dir / no-strconv / no-runWithInput" invariants are enforced by greppable Level-3
gates. The parallel-execution note (S6 DiffTree also landing) is a non-conflict: T3.S1 edits the
StagedDiff stub (immediately after S6's parseDiffTree) and removes a distinct TestStubsPanic line.
