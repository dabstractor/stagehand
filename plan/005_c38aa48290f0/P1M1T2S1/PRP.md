name: "P1.M1.T2.S1 — Thread user exclude pathspecs into StagedDiff / WorkingTreeDiff / TreeDiff; emit [excluded] placeholders; UNION with the built-in denylist"
description: |
  The final, end-to-end wiring of PRD §9.18 (Payload exclusions). It consumes the pathspec slice from
  P1.M1.T1.S2 (`exclude.ResolveExcludePathspecs`, ALREADY LANDED) and makes the THREE diff paths in
  internal/git actually honor it: (1) change the exclude-union behavior from REPLACE to UNION so the
  built-in denylist (lock/snap/map/vendor, FR3) ALWAYS applies alongside the user globs (FR-X1);
  (2) for each changed file the user excluded, emit the one-line placeholder `<status>\t[excluded]
  <path>` (mirroring the existing `[binary]` placeholder, FR-X4); (3) thread the resolved pathspecs from
  the two resolution sites (pkg/stagecoach internal; CLI runDecompose) through `generate.Deps` /
  `decompose.Deps` into the five `StagedDiffOptions` literals. Exclusion is PAYLOAD-ONLY — it never
  alters staging or commit content (FR-X5). Adds a stubagent stdin-capture knob + an e2e scenario + the
  docs/how-it-works.md diff-capture note. Hook exec (P1.M3.T2.S1) inherits this for free via StagedDiff.

---

## Goal

**Feature Goal**: A repo with a `.stagecoachignore` (or `[generation].exclude`, or `--exclude`) hides the
*bodies* of the matched files from every diff the agent sees (staged, working-tree snapshot, per-concept
tree-to-tree), while still (a) emitting a `<status>\t[excluded] <path>` placeholder so the agent knows the
file changed, and (b) committing that file exactly as it stands. The built-in noise denylist keeps
applying (UNION, not replace).

**Deliverable**:
1. `internal/git/binary.go` — two new primitives: `excludedPlaceholderLine(status, path) string` and
   `(*gitRunner).detectExcludedStatuses(ctx, allStatuses, excludes, diffArgs...) (map[string]string, error)`.
2. `internal/git/git.go` — the THREE diff methods (`StagedDiff`, `WorkingTreeDiff`, `TreeDiff`) each:
   (a) switch the Part-2 exclude assembly from REPLACE to UNION (`defaultExcludes ++ opts.Excludes`);
   (b) after the binary block, call `detectExcludedStatuses`, sort, and emit `[excluded]` placeholders
   for user-excluded changed files that are NOT already binary.
3. Threading: `generate.Deps` and `decompose.Deps` each gain `Excludes []string`; the five
   `StagedDiffOptions` literals set `Excludes: deps.Excludes`; `ResolveExcludePathspecs` is called at
   `pkg/stagecoach.{GenerateCommit,Decompose}` (repoDir from `resolveConfig`) and at CLI `runDecompose`.
4. `cmd/stubagent/main.go` — `STAGECOACH_STUB_STDINFILE` knob (tee stdin to a file; test-binary-only).
5. Tests: git-package integration tests (placeholder + body-absent + UNION) on all three methods; an
   end-to-end stubagent payload-capture test; an e2e scenario asserting the payload-only guarantee.
6. `docs/how-it-works.md` — diff-capture subsection: `[excluded]` placeholder format + FR-X5 guarantee.

**Success Definition**:
- On all three diff paths, a user-excluded changed file's diff HUNK is absent, its
  `<status>\t[excluded] <path>` placeholder is present, and a non-excluded sibling is present.
- The built-in denylist still applies when `opts.Excludes` is non-empty (UNION proof).
- An empty user-union (`opts.Excludes == nil`) changes NOTHING vs. today (no placeholders, no extra git
  call) — zero overhead in the common case.
- Exclusion never changes the committed tree (e2e proves the excluded file IS committed).
- `go build ./...`, `go test ./...` (incl. `-race`), `go vet`, `golangci-lint` all pass; e2e (`-tags e2e`) green.

## Why

- **FR-X3 (PRD §9.18)**: exclusions must apply to EVERY diff path exactly like binary filtering (FR3c).
  The `:(exclude,glob)` pathspecs produced upstream are useless until they reach the `git diff` calls.
- **FR-X1**: the pattern sources are a UNION — silently dropping the built-in denylist when the user adds
  one custom exclude would regress the noise filter (lock files would flood the payload again).
- **FR-X4**: silence is wrong. The planner must see that an excluded file changed (to group it into the
  right concept) and the message agent must see it exists (to avoid a half-picture) — hence the
  placeholder, identical in shape to `[binary]` so no prompt-side change is needed.
- **FR-X5**: the defining safety property of the feature — exclusion is payload-only, NEVER commit
  content. The docs and an e2e scenario state and prove this is the inverse of the "content loss" fear.
- Hook exec (P1.M3.T2.S1) reuses `StagedDiff` (FR-H4), so wiring exclusions into the diff layer once
  delivers the feature to the hook path for free.

## What

Given `.stagecoachignore` containing `secrets.env` and `cfg.Exclude == []`, a staged change set
`{feature.go, secrets.env}` produces (single-commit `StagedDiff`) a payload like:

```
diff --git a/feature.go b/feature.go
... full hunk ...
A	[excluded] secrets.env
```

— `feature.go`'s body present, `secrets.env`'s hunk absent, the placeholder present, and `git write-tree`
+ `commit-tree` commit `secrets.env` unchanged. The same happens in the decompose planner diff
(`TreeDiff(base, T_start)`) and each per-concept message diff (`TreeDiff(tree[i-1], tree[i])`).

### Success Criteria

- [ ] `excludedPlaceholderLine` + `detectExcludedStatuses` added to `internal/git/binary.go`.
- [ ] `StagedDiff`, `WorkingTreeDiff`, `TreeDiff` each: UNION excludes + emit `[excluded]` placeholders.
- [ ] `generate.Deps.Excludes` + `decompose.Deps.Excludes` fields; 5 call sites set `Excludes: deps.Excludes`.
- [ ] `ResolveExcludePathspecs` called in pkg/stagecoach (both entry points) + CLI `runDecompose`.
- [ ] `STAGECOACH_STUB_STDINFILE` added to stubagent (tee stdin, preserve deadlock guard).
- [ ] git-package tests on all three methods (placeholder present, hunk absent, UNION proof, empty=no-op).
- [ ] Stubagent payload-capture test (excluded body absent + placeholder present end-to-end).
- [ ] e2e scenario: excluded file IS committed (payload-only guarantee).
- [ ] `docs/how-it-works.md` gains the `[excluded]` placeholder + FR-X5 subsection.

## All Needed Context

### Context Completeness Check

_This PRP names the two helpers, the exact edit in each of the three diff methods (incl. the diffArgs per
method), the Deps fields, every call site (file:line), the two resolution sites, the stubagent knob, the
test breakage (3 existing tests MUST be updated because placeholders make the path reappear), and the
docs anchor. An implementer with no prior codebase knowledge can complete it._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/architecture/system_context.md
  why: §3 names the exact seams — StagedDiffOptions.Excludes (nil today), the binary.go placeholder
       template (detectBinaryFiles/fileStatuses/binaryPlaceholderLine), defaultExcludes, and the
       "Excludes is passed nil by every caller" gap this task plugs.
  section: "## 3. Package inventory & the v2.1 seams (internal/git ...)"
  critical: "opts.Excludes REPLACES defaultExcludes today (REPLACE→UNION is this task's call per FR-X1).
             binary.go is the EXACT template; mirror it for [excluded]."

- docfile: plan/005_c38aa48290f0/P1M1T2S1/research/design-decisions.md
  why: The seven locked decisions: REPLACE→UNION, the existing-test breakage gotcha, the
       set-difference placeholder detection, binary>excluded precedence, the Deps threading path, the
       stubagent STDINFILE knob, and the scope fence.
  critical: "Section 3 (CRITICAL GOTCHA) — 3 existing tests assert path-absence and WILL FAIL once
             placeholders emit the path; they must be rewritten to assert hunk-absent + placeholder-present."

- file: internal/git/binary.go
  why: THE TEMPLATE. detectBinaryFiles/fileStatuses/binaryPlaceholderLine + the doc comment's diffArgs
       table (--cached / none / treeA,treeB). Add the two new primitives here (co-locate; the file doc
       already says 'shared primitives').
  pattern: |
    func binaryPlaceholderLine(status, path string) string { return status + "\t[binary] " + path }
    // detectExcludedStatuses parallels detectBinaryFiles: one `git diff` call, returns path→status.
  gotcha: "fileStatuses keys by DESTINATION path; --name-only also emits destination paths (no -M → D+A
          pairs), so the set-difference against fileStatuses' keys aligns. Do NOT add -M."

- file: internal/git/git.go
  why: The three methods to edit (StagedDiff L579, TreeDiff L1011, WorkingTreeDiff L1128) and
       defaultExcludes (L552). Each has the identical binary block + the REPLACE line to change.
  pattern: |
    # REPLACE line (in all three):
    excludes := opts.Excludes
    if len(excludes) == 0 { excludes = defaultExcludes }
    # → UNION (in all three):
    excludes := make([]string, 0, len(defaultExcludes)+len(opts.Excludes))
    excludes = append(excludes, defaultExcludes...)
    excludes = append(excludes, opts.Excludes...)
  gotcha: "diffArgs differ: StagedDiff=\"--cached\", WorkingTreeDiff=(none), TreeDiff=treeA,treeB. Pass
          them to detectExcludedStatuses variadic, same as detectBinaryFiles(ctx, diffArgs...)."

- file: internal/git/stagediff_test.go  (also treediff_test.go, workingtreediff_test.go)
  why: The temp-repo test PATTERN (initRepo/writeFile/stageFile/runGit helpers) AND the 3 tests that
       BREAK (they assert `!Contains(out,"drop.go")` — placeholders now emit the path).
  pattern: "TestStagedDiff_BinaryFilePlaceholderAndExcluded asserts `Contains(out, \"A\\t[binary]
            logo.png\")` — copy this assertion shape for `[excluded]`."
  gotcha: "TestStagedDiff_CustomExcludesOverride / TestTreeDiff_ExcludesApplied /
          TestWorkingTreeDiff_ExcludesApplied MUST be rewritten (see research §3)."

- file: internal/exclude/exclude.go
  why: UPSTREAM CONTRACT (ALREADY LANDED). ResolveExcludePathspecs(cfg, repoRoot, v) returns the
       translated :(exclude,glob) union of .stagecoachignore ∪ cfg.Exclude. Consume it; do NOT modify it.
  pattern: "excludes, err := exclude.ResolveExcludePathspecs(cfg, repoDir, verbose)  // []string or nil"
  gotcha: "Returns (nil,nil) on missing .stagecoachignore (no-op). Only a genuine read failure errors —
          propagate it. Output is sources b+c+d ONLY; defaultExcludes (source a) is git.go's job (UNION here)."

- file: internal/generate/generate.go  (generate.Deps, L33) + internal/decompose/roles.go  (decompose.Deps, L55)
  why: The two Deps structs to extend with `Excludes []string`. Each StagedDiffOptions literal at the
       call sites (generate.go:143; decompose planner.go:69, message.go:71, decompose.go:595) sets it.
  pattern: "git.StagedDiffOptions{MaxDiffBytes: cfg.MaxDiffBytes, MaxMDLines: cfg.MaxMdLines,
            BinaryExtensions: cfg.BinaryExtensions, Excludes: deps.Excludes}"
  gotcha: "CommitStaged (generate.go:143) AND runPipeline (pkg/stagecoach/stagecoach.go:414) BOTH call
          StagedDiff — set Excludes in BOTH StagedDiffOptions literals (deps.Excludes is the same field)."

- file: pkg/stagecoach/stagecoach.go
  why: The two RESOLUTION SITES (GenerateCommit + Decompose). resolveConfig/resolveDecomposeConfig
       return (cfg, repoDir, err); deps.Verbose is created right after. Resolve excludes there.
  pattern: |
    cfg, repoDir, err := resolveConfig(ctx, opts)            // repoDir = os.Getwd()
    deps, err := buildDeps(cfg, repoDir)
    deps.Verbose = ui.NewVerbose(opts.Verbose, cfg.Verbose)
    deps.Excludes, err = exclude.ResolveExcludePathspecs(cfg, repoDir, deps.Verbose)   // ← add
    if err != nil { return Result{}, fmt.Errorf("resolve excludes: %w", err) }
  gotcha: "resolveDecomposeConfig (Decompose path) ALSO returns repoDir — resolve there too and set
          deps.Excludes before internal/decompose.Decompose. NO new Options field (resolution is internal)."

- file: internal/cmd/default_action.go
  why: CLI runDecompose builds decompose.Deps DIRECTLY (does NOT go through pkg/stagecoach.Decompose —
       per architecture note). It must resolve excludes itself. runDefault has repoDir (os.Getwd()).
  pattern: "Pass repoDir into runDecompose (add a param), then:
            excludes, err := exclude.ResolveExcludePathspecs(*cfg, repoDir, verbose)
            deps := decompose.Deps{..., Excludes: excludes}"
  gotcha: "The single-commit CLI path calls stagecoach.GenerateCommit (resolves internally) — do NOT
          double-resolve there. Only runDecompose needs manual resolution."

- file: cmd/stubagent/main.go
  why: Add STAGECOACH_STUB_STDINFILE. Currently `io.Copy(io.Discard, os.Stdin)` drains stdin (deadlock
       guard). Tee to a file when the knob is set so tests can assert the agent's received payload.
  pattern: |
    if f := os.Getenv("STAGECOACH_STUB_STDINFILE"); f != "" {
        var buf bytes.Buffer; io.Copy(&buf, os.Stdin); os.WriteFile(f, buf.Bytes(), 0o644)
    } else { io.Copy(io.Discard, os.Stdin) }
  gotcha: "Drain FULLY in both branches (the ~64KiB pipe deadlock guard must hold). Keep the order:
           drain FIRST, then MARKER, then ARGSFILE (existing) — do not reorder."

- file: internal/e2e/scenarios_test.go  +  internal/e2e/harness_test.go
  why: The e2e scenario PATTERN (buildStagecoach/buildStub/newRepo/seedCommit/runStagecoach/stubEnv/
       writeStubConfig/diffTreeNames). Add a scenario proving the payload-only guarantee.
  pattern: "S3_ConcurrentFile_Excluded shows the stub-marker + goroutine + assertion shape; a simpler
            synchronous scenario suffices here (no concurrency needed)."
  gotcha: "e2e can't inspect the agent payload — assert the EXCLUDED FILE IS COMMITTED (diffTreeNames
          includes it) to prove FR-X5. Set cfg.Exclude via .stagecoachignore or a config extra."
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go             # StagedDiff L579, WorkingTreeDiff L1128, TreeDiff L1011; defaultExcludes L552
  binary.go          # detectBinaryFiles, fileStatuses, binaryPlaceholderLine, isBinaryByExtension
  stagediff_test.go  # TestStagedDiff_*  (incl. _CustomExcludesOverride — REWRITE)
  treediff_test.go   # TestTreeDiff_*     (incl. _ExcludesApplied — REWRITE)
  workingtreediff_test.go # TestWorkingTreeDiff_* (incl. _ExcludesApplied — REWRITE)
internal/exclude/exclude.go   # ResolveExcludePathspecs — UPSTREAM, already landed (consume only)
internal/generate/generate.go # generate.Deps (L33) + CommitStaged StagedDiffOptions (L143)
internal/decompose/
  roles.go           # decompose.Deps (L55)
  planner.go         # callPlanner TreeDiff StagedDiffOptions (L69)
  message.go         # generateMessage TreeDiff StagedDiffOptions (L71)
  decompose.go       # arbiter-leftover TreeDiff StagedDiffOptions (L595)
pkg/stagecoach/stagecoach.go    # GenerateCommit/Decompose (resolve sites) + runPipeline StagedDiffOptions (L414)
internal/cmd/default_action.go# runDecompose (manual resolve) + runDefault (repoDir)
cmd/stubagent/main.go         # STAGECOACH_STUB_* knobs (add _STDINFILE)
internal/e2e/                 # harness_test.go + scenarios_test.go (add a scenario)
docs/how-it-works.md          # "### Binary and non-text file filtering" section (extend)
```

### Desired Codebase tree with files to be added/changed

```bash
internal/git/binary.go            # + excludedPlaceholderLine, + detectExcludedStatuses
internal/git/git.go               # ~ StagedDiff/WorkingTreeDiff/TreeDiff: UNION + [excluded] block
internal/git/stagediff_test.go    # ~ rewrite _CustomExcludesOverride; + placeholder/union/no-op tests
internal/git/treediff_test.go     # ~ rewrite _ExcludesApplied; + placeholder/union tests
internal/git/workingtreediff_test.go # ~ rewrite _ExcludesApplied; + placeholder/union tests
internal/generate/generate.go     # + Deps.Excludes; + Excludes in CommitStaged StagedDiffOptions
internal/decompose/roles.go       # + Deps.Excludes
internal/decompose/{planner,message,decompose}.go # + Excludes in the 3 TreeDiff StagedDiffOptions
pkg/stagecoach/stagecoach.go        # + resolve excludes in GenerateCommit + Decompose; + runPipeline Excludes
internal/cmd/default_action.go    # + resolve excludes in runDecompose (repoDir param)
cmd/stubagent/main.go             # + STAGECOACH_STUB_STDINFILE
internal/generate/generate_test.go (or pkg/stagecoach) # + stubagent payload-capture test
internal/e2e/scenarios_test.go    # + Sx_ExcludedFileStillCommitted scenario
docs/how-it-works.md              # + "### Payload exclusions" subsection (placeholder + FR-X5)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: REPLACE→UNION. The line `excludes := opts.Excludes; if len(excludes)==0 {excludes=defaultExcludes}`
// currently DROPS the built-in denylist when opts.Excludes is set. FR-X1 mandates UNION
// (defaultExcludes ++ opts.Excludes). Change in ALL THREE methods identically.

// CRITICAL: existing tests break. TestStagedDiff_CustomExcludesOverride / TestTreeDiff_ExcludesApplied /
// TestWorkingTreeDiff_ExcludesApplied assert the excluded PATH is absent from `out`. With a placeholder
// the path REAPPEARS (inside "A\t[excluded] drop.go"). Rewrite them to assert the HUNK is absent
// (!Contains "diff --git a/drop.go") AND the placeholder is present (Contains "[excluded] drop.go").

// CRITICAL: only USER excludes get placeholders. defaultExcludes (lock/snap/map/vendor) is NOT opts.Excludes
// — those files are dropped silently (no placeholder), exactly like today. detectExcludedStatuses must
// take opts.Excludes (the user slice), NOT the unioned excludes. Empty opts.Excludes ⇒ (nil,nil), NO git
// call, NO placeholders (zero overhead in the common case).

// CRITICAL: binary > excluded precedence. A file that is BOTH binary AND user-excluded gets `[binary]`
// only (binary filtering runs first, is more specific). When emitting `[excluded]`, skip paths already in
// the binary set (binPaths) to avoid a double placeholder.

// GOTCHA: never alias excludes. Build binExcludes as a SEPARATE slice (the existing code already does) —
// `excludes` may back the aggregate args; appending ":!path" to it would corrupt the union.

// GOTCHA: diffArgs per method. detectExcludedStatuses(ctx, statuses, opts.Excludes, diffArgs...) mirrors
// detectBinaryFiles(ctx, diffArgs...). StagedDiff passes ("--cached"); WorkingTreeDiff passes nothing;
// TreeDiff passes (treeA, treeB). Variadic-empty is the working-tree domain — correct.

// GOTCHA: set-difference, not pathspec conversion. Do NOT try to invert ":(exclude,glob)X"→":(glob)X" by
// hand. Query `git diff <diffArgs> --name-only -- <opts.Excludes>` for the SURVIVING set and complement
// against the fileStatuses map. Robust to ":!" and ":(exclude,glob)" spellings alike.

// GOTCHA: ResolveExcludePathspecs needs repoRoot. It is NOT on Deps (git.Git hides workDir). Resolve at
// pkg/stagecoach (repoDir from resolveConfig) and CLI runDecompose (repoDir from runDefault); thread the
// resulting []string via Deps.Excludes. Do NOT add repoRoot to Deps (leaks git internals).

// GOTCHA: stubagent deadlock guard. When adding STAGECOACH_STUB_STDINFILE, drain stdin FULLY in the tee
// branch (io.Copy into a buffer, then WriteFile) — never read a fixed prefix. The ~64KiB OS pipe would
// deadlock parent+child otherwise. Keep drain-first ordering.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/git/binary.go  (additions)

// excludedPlaceholderLine returns the FR-X4 one-line placeholder for a user-excluded file:
// "<status>\t[excluded] <path>". Mirrors binaryPlaceholderLine; distinguishable by tag ([excluded] vs [binary]).
func excludedPlaceholderLine(status, path string) string { return status + "\t[excluded] " + path }

// detectExcludedStatuses returns the subset of allStatuses (path→status) whose paths the USER exclude
// pathspecs remove from `git diff <diffArgs>`. It runs `git diff <diffArgs> --name-only -- <excludes>`
// for the SURVIVING paths, then returns allStatuses minus those (the excluded set, statuses preserved).
// Empty excludes ⇒ (nil, nil) with NO git call. diffArgs selects the domain, variadic, identical to
// detectBinaryFiles: "--cached" (staged), nothing (working tree), treeA treeB (tree-to-tree).
// Read-only w.r.t. refs/index. (PRD §9.18 FR-X4 placeholder source.)
func (g *gitRunner) detectExcludedStatuses(ctx context.Context, allStatuses map[string]string,
	excludes []string, diffArgs ...string) (map[string]string, error)
```

```go
// generate.Deps (generate.go) + decompose.Deps (roles.go) — add one field each:
Excludes []string // resolved user exclude pathspecs (from exclude.ResolveExcludePathspecs); nil ⇒ none
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/binary.go  — the two primitives
  - IMPLEMENT: excludedPlaceholderLine(status, path) → status+"\t[excluded] "+path (mirror binaryPlaceholderLine).
  - IMPLEMENT: (*gitRunner).detectExcludedStatuses(ctx, allStatuses map[string]string, excludes []string,
    diffArgs ...string) (map[string]string, error):
      if len(excludes)==0 { return nil,nil }                        // no-op, NO git call
      args := ["diff", diffArgs..., "--name-only", "--", excludes...]   // surviving paths
      run; on err return nil,err; on code!=0 return wrapped error (mirror fileStatuses error format)
      surviving := set of TrimSpace non-empty lines
      excluded := { path: allStatuses[path] for path in allStatuses if path not in surviving }
      return excluded,nil
  - FOLLOW pattern: detectBinaryFiles/fileStatuses (args assembly, run() error handling, SplitN/TrimSpace).
  - EXPAND the binary.go package doc comment to mention FR-X4 (excluded placeholders) alongside FR3a/b/c.
  - PLACEMENT: right after binaryPlaceholderLine (placeholder) and after fileStatuses (detector).

Task 2: MODIFY internal/git/git.go — StagedDiff (UNION + [excluded] block)
  - In the binary block (after binPaths/binExcludes are built and `statuses` is in scope), ADD:
      excluded, xerr := g.detectExcludedStatuses(ctx, statuses, opts.Excludes, "--cached")
      if xerr != nil { return "", xerr }
      exPaths := sorted paths in `excluded` NOT already in binSet (binary wins); for each:
          b.WriteString(excludedPlaceholderLine(excluded[path], path)); b.WriteByte('\n')
  - CHANGE Part-2 exclude assembly from REPLACE to UNION (defaultExcludes ++ opts.Excludes). Keep the
    separate ":!*.md",":!*.markdown" + binExcludes appends EXACTLY as today.
  - NOTE: emit `[excluded]` BEFORE the aggregate (Part 2) — same position as `[binary]` (both are
    "changed but body omitted" notices grouped before the real hunks).
  - FOLLOW the existing binary block byte-for-byte as the structural template.

Task 3: MODIFY internal/git/git.go — TreeDiff (L1011) and WorkingTreeDiff (L1128)
  - Apply the IDENTICAL change as Task 2, with diffArgs:
      TreeDiff:        detectExcludedStatuses(ctx, statuses, opts.Excludes, treeA, treeB)
      WorkingTreeDiff: detectExcludedStatuses(ctx, statuses, opts.Excludes)   // no diffArgs
  - Same REPLACE→UNION edit; same placeholder block; same binExcludes handling.
  - WHY also WorkingTreeDiff: FR-X3 (every diff path) + the method ships in the public Git interface;
    even though the freeze replaced its use in the planner, honoring excludes keeps it correct/ready.

Task 4: MODIFY generate.Deps + the two StagedDiff call sites (generate.go, pkg/stagecoach/stagecoach.go)
  - ADD `Excludes []string` to generate.Deps (generate.go ~L36).
  - generate.go CommitStaged (L143): add `Excludes: deps.Excludes` to the StagedDiffOptions literal.
  - pkg/stagecoach/stagecoach.go runPipeline (L414): add `Excludes: deps.Excludes` to its literal.

Task 5: MODIFY decompose.Deps + the three TreeDiff call sites
  - ADD `Excludes []string` to decompose.Deps (roles.go ~L60).
  - planner.go callPlanner (L69), message.go generateMessage (L71), decompose.go arbiter-leftover (L595):
    add `Excludes: deps.Excludes` to each StagedDiffOptions literal.

Task 6: MODIFY pkg/stagecoach/stagecoach.go — resolve excludes at both entry points
  - GenerateCommit: after `deps.Verbose = ui.NewVerbose(...)`, add:
      deps.Excludes, err = exclude.ResolveExcludePathspecs(cfg, repoDir, deps.Verbose)
      if err != nil { return Result{}, fmt.Errorf("resolve excludes: %w", err) }
    (cfg, repoDir come from resolveConfig; add the internal/exclude import.)
  - Decompose: same, after resolveDecomposeConfig returns (cfg, repoDir) and deps.Verbose is built, before
    internal/decompose.Decompose. Set deps.Excludes.
  - GOTCHA: no new Options field — resolution is internal (standalone library users get it for free).

Task 7: MODIFY internal/cmd/default_action.go — resolve excludes in runDecompose
  - Add a `repoDir string` param to runDecompose; update its single caller (runDefault, which has repoDir).
  - After building `verbose := ui.NewVerbose(stderr, cfg.Verbose)`, add:
      excludes, err := exclude.ResolveExcludePathspecs(*cfg, repoDir, verbose)
      if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("resolve excludes: %w", err)) }
    and set `Excludes: excludes` in the decompose.Deps literal. Add the internal/exclude import.
  - GOTCHA: do NOT touch the single-commit path (runDefault → stagecoach.GenerateCommit resolves internally).

Task 8: MODIFY cmd/stubagent/main.go — STAGECOACH_STUB_STDINFILE
  - Replace `io.Copy(io.Discard, os.Stdin)` with: if STAGECOACH_STUB_STDINFILE set, io.Copy into a
    bytes.Buffer then os.WriteFile; else io.Discard. Drain FULLY in both branches.
  - Keep the existing post-drain ordering (MARKER → ARGSFILE → sleep → stderr → out → exit) unchanged.
  - Add "bytes" import.

Task 9: MODIFY git-package tests (3 files) — rewrite + add
  - stagediff_test.go: rewrite TestStagedDiff_CustomExcludesOverride → assert hunk-absent +
    placeholder-present (Excludes uses ":(exclude,glob)**/drop.go" to match production; keep a raw
    ":!drop.go" variant for robustness). ADD: TestStagedDiff_ExcludedPlaceholder (user-excluded file
    placeholder present, sibling present, default-denylist file ALSO excluded → UNION proof),
    TestStagedDiff_ExcludedEmptyIsNoOp (opts.Excludes=nil ⇒ no [excluded] lines, no behavior change),
    TestStagedDiff_ExcludedBinaryPrecedence (a file that is both excluded AND binary → [binary] only).
  - treediff_test.go / workingtreediff_test.go: same rewrite + add for TreeDiff/WorkingTreeDiff.

Task 10: ADD stubagent payload-capture integration test
  - In internal/generate/generate_test.go (or pkg/stagecoach): build a stub Manifest (stubtest), temp repo
    with feature.go + secret.conf staged, cfg.Exclude=["*.conf"], STAGECOACH_STUB_STDINFILE set; run
    CommitStaged (or GenerateCommit); read the captured payload file; assert: secret.conf diff hunk
    ABSENT, "A\t[excluded] secret.conf" (or M) PRESENT, feature.go PRESENT. Proves end-to-end wiring.

Task 11: ADD internal/e2e scenario — payload-only guarantee
  - In scenarios_test.go add t.Run("Sx_ExcludedFileStillCommitted"): write a .stagecoachignore (or a
    [generation].exclude config extra), stage feature.go + excluded.txt, runStagecoach with stub; assert
    exit 0 AND diffTreeNames(head) INCLUDES excluded.txt (it was committed despite being excluded from
    the payload — FR-X5). Stub-only (no real agent needed).

Task 12: MODIFY docs/how-it-works.md — diff-capture subsection
  - After the "### Binary and non-text file filtering" section, add "### Payload exclusions
    (.stagecoachignore)": exclusion patterns (sources: built-in denylist, .stagecoachignore,
    [generation].exclude, --exclude) hide a file's DIFF BODY from every payload but emit a
    `<status>\t[excluded] <path>` placeholder (same shape as `[binary]`, distinguishable by tag) so the
    agent still sees the file changed. State FR-X5 verbatim: "excluded from what the agent sees, still
    committed." Cross-ref docs/configuration.md's `.stagecoachignore` section (added by P1.M1.T1.S2).
    Mode A (rides with this subtask).
```

### Implementation Patterns & Key Details

```go
// The [excluded] block — inserted in each diff method right after the [binary] block (binPaths/binExcludes
// built; `statuses` and `binSet` in scope). diffArgs is method-specific (see Task 3).
excluded, xerr := g.detectExcludedStatuses(ctx, statuses, opts.Excludes /*, diffArgs... */)
if xerr != nil {
	return "", xerr
}
exPaths := make([]string, 0, len(excluded))
for path := range excluded {
	if binSet[path] {
		continue // FR3b binary placeholder already covers this path; binary is the more specific signal
	}
	exPaths = append(exPaths, path)
}
sort.Strings(exPaths) // deterministic output (mirrors the sorted binPaths above)
for _, path := range exPaths {
	b.WriteString(excludedPlaceholderLine(excluded[path], path)) // "<status>\t[excluded] <path>"
	b.WriteByte('\n')
}

// The REPLACE→UNION edit (Part 2, all three methods):
excludes := make([]string, 0, len(defaultExcludes)+len(opts.Excludes))
excludes = append(excludes, defaultExcludes...) // FR3 / FR-X1 source (a) — ALWAYS
excludes = append(excludes, opts.Excludes...)    // user union, FR-X1 sources (b)+(c)+(d)
// … then nmArgs appends ":!*.md", ":!*.markdown" and binExcludes exactly as today.

// detectExcludedStatuses — set-difference detector.
func (g *gitRunner) detectExcludedStatuses(ctx context.Context, allStatuses map[string]string,
	excludes []string, diffArgs ...string) (map[string]string, error) {
	if len(excludes) == 0 {
		return nil, nil // no user exclusions ⇒ no placeholders, no git call (zero overhead)
	}
	args := []string{"diff"}
	args = append(args, diffArgs...)
	args = append(args, "--name-only", "--")
	args = append(args, excludes...)
	stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("git diff (exclude probe): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	surviving := make(map[string]bool, len(allStatuses))
	for _, line := range strings.Split(stdout, "\n") {
		if p := strings.TrimSpace(line); p != "" {
			surviving[p] = true
		}
	}
	excluded := make(map[string]string)
	for path, st := range allStatuses {
		if !surviving[path] { // present in all-changed but removed by the exclude pathspecs
			excluded[path] = st
		}
	}
	return excluded, nil
}
```

### Integration Points

```yaml
UPSTREAM (already landed — consume, do not modify):
  - internal/exclude/exclude.go: ResolveExcludePathspecs(cfg, repoRoot, v) → []string of :(exclude,glob)… .
  - internal/config/config.go: cfg.Exclude []string (S1).
DOWNSTREAM (inherits for free — do NOT implement here):
  - P1.M3.T2.S1 hook exec: calls StagedDiff (FR-H4) → gets exclusions + placeholders automatically.
THIS TASK'S TOUCHPOINTS:
  - internal/git/{binary,git}.go (3 methods, 2 helpers).
  - generate.Deps + decompose.Deps (+5 StagedDiffOptions literals).
  - pkg/stagecoach (2 resolve sites) + cmd/default_action.go (runDecompose resolve).
  - cmd/stubagent (STDINFILE).
  - tests + e2e + docs/how-it-works.md.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/git/binary.go internal/git/git.go internal/generate/generate.go \
  internal/decompose/roles.go internal/decompose/planner.go internal/decompose/message.go \
  internal/decompose/decompose.go pkg/stagecoach/stagecoach.go internal/cmd/default_action.go \
  cmd/stubagent/main.go docs 2>/dev/null
go build ./...
go vet ./internal/git/... ./internal/generate/... ./internal/decompose/... ./pkg/stagecoach/... ./cmd/...
golangci-lint run ./internal/git/... ./internal/generate/... ./internal/decompose/... ./pkg/stagecoach/... ./cmd/...
# Expected: zero errors.
```

### Level 2: Unit / Integration Tests (the load-bearing contract)

```bash
# All three diff methods: placeholder present, hunk absent, UNION, empty=no-op, binary-precedence.
go test ./internal/git/... -run 'Excluded|Excludes' -v
go test ./internal/git/... -v   # confirm the 3 rewritten tests + the rest stay green

# Stubagent payload capture (end-to-end single path).
go test ./internal/generate/... -run 'ExcludedPayload' -v   # (or pkg/stagecoach, per Task 10 placement)

# Resolution wiring sanity (no panic, deps.Excludes populated).
go test ./pkg/stagecoach/... ./internal/cmd/... -v
# Expected: all pass. REQUIRED cases:
#  StagedDiff/TreeDiff/WorkingTreeDiff: user-excluded file → "A\t[excluded] <path>" present, its
#    "diff --git a/<f>" hunk absent, a non-excluded sibling present.
#  UNION: a "*.lock" file is ALSO excluded when opts.Excludes is non-empty (defaultExcludes still applies).
#  EMPTY no-op: opts.Excludes nil ⇒ no "[excluded]" lines, identical to pre-change behavior.
#  BINARY precedence: a file both excluded and binary → "[binary]" only (no "[excluded]").
#  Stubagent payload: captured stdin has the placeholder, not the excluded body.
```

### Level 3: E2E (payload-only guarantee)

```bash
go test -tags e2e ./internal/e2e/... -run 'ExcludedFileStillCommitted' -v
# Scenario: .stagecoachignore (or [generation].exclude) hides excluded.txt; run stagecoach; assert
# exit 0 AND diffTreeNames(head) includes excluded.txt (it was COMMITTED — FR-X5). Stub-only.
# Expected: pass (excluded from payload, still committed).
```

### Level 4: Cross-cutting / Regression

```bash
go test ./...                       # nothing else changed behavior; full suite green
go test -race ./internal/git/... ./internal/decompose/...   # race detector (run() concurrency-safe)
# Expected: green; -race clean. The REPLACE→UNION change only affects the non-empty-opts.Excludes path
# (every existing caller passes nil), so regression risk is confined to the 3 rewritten tests.
npx --yes markdownlint-cli docs/how-it-works.md 2>/dev/null || echo "verify the new subsection renders"
```

## Final Validation Checklist

### Technical
- [ ] `go build ./...`, `go vet`, `golangci-lint` clean; `gofmt` no diff.
- [ ] `go test ./...` passes; `go test -race ./internal/git/... ./internal/decompose/...` clean.
- [ ] `go test -tags e2e ./internal/e2e/...` green (incl. the new exclusion scenario).

### Feature
- [ ] On all three diff paths: user-excluded changed file → `<status>\t[excluded] <path>` placeholder
      present, its diff hunk absent, non-excluded sibling present.
- [ ] Built-in denylist still applies when `opts.Excludes` is non-empty (UNION proof).
- [ ] Empty user-union (nil opts.Excludes) ⇒ no placeholders, no extra git call (zero-overhead no-op).
- [ ] Binary+excluded file → `[binary]` only (no double placeholder).
- [ ] e2e: excluded file IS committed (payload-only guarantee, FR-X5).
- [ ] Stubagent payload-capture test proves the agent receives placeholder, not the excluded body.

### Code Quality
- [ ] The `[excluded]` block mirrors the `[binary]` block structure (sorted, separate slice, before Part 2).
- [ ] REPLACE→UNION applied identically in all three methods.
- [ ] Deps.Excludes threaded (not repoRoot — no git-internals leak); resolution colocated with repoDir.
- [ ] stubagent STDINFILE preserves the drain-first deadlock guard.

### Scope Boundaries (do NOT cross)
- [ ] No changes to `internal/exclude/*` (S2 — consume only).
- [ ] No changes to `cfg.Exclude` resolution / config load (S1).
- [ ] No `[binary]` placeholder logic changes (already shipped — only ADD the parallel `[excluded]` path).
- [ ] No hook exec work (P1.M3.T2.S1 — it inherits via StagedDiff).
- [ ] No new pkg/stagecoach.Options field (resolution stays internal).

---

## Anti-Patterns to Avoid
- ❌ Don't fold defaultExcludes into ResolveExcludePathspecs — source (a) is git.go's job; UNION happens here.
- ❌ Don't emit `[excluded]` for default-denylist files (lock/snap/map/vendor) — only USER excludes.
- ❌ Don't double-placeholder a binary+excluded file — binary wins; skip it in the excluded loop.
- ❌ Don't convert pathspecs by hand (`:(exclude,glob)`→`:(glob)`) — use the set-difference probe.
- ❌ Don't leave the 3 existing exclude tests asserting path-absence — placeholders emit the path; rewrite them.
- ❌ Don't add repoRoot to Deps — resolve where it's known (pkg/stagecoach / runDecompose), thread the slice.
- ❌ Don't break the stubagent deadlock guard when teeing stdin — drain FULLY before WriteFile.
- ❌ Don't reorder the stubagent's post-drain steps (MARKER→ARGSFILE→…).

---

## Confidence Score

**8.5/10** for one-pass success. The change is a faithful, mechanical mirror of the already-shipped
`[binary]` filtering across three near-identical methods; every call site (file:line) and Deps field is
named, and the upstream contract (`ResolveExcludePathspecs`) is already landed. The −1.5 is two
tractable risks: (1) the three existing exclude tests WILL break once placeholders emit the path (clearly
flagged — they must be rewritten, not just re-run); and (2) the end-to-end resolution wiring spans two
packages (pkg/stagecoach + CLI runDecompose) where a missed call site would silently leave one path
unwired — mitigated by the per-method git-package tests AND the stubagent payload-capture test, which
together prove all three diff paths and the single-commit end-to-end path. The set-difference detector
avoids the fragile pathspec-conversion alternative entirely.
