# P1.M1.T2.S1 — Design decisions & research notes

Surveyed 2026-07-02 on `competitor-feature-parity`. The `internal/exclude` package (P1.M1.T1.S2) is
ALREADY LANDED (`ResolveExcludePathspecs`, `TranslatePattern`, `LoadStagecoachIgnore` all present and
matching the S2 PRP contract). This task consumes it.

## 1. The three diff methods are near-identical ports (the template)

`internal/git/git.go` has THREE methods that each build `StagedDiffOptions`-driven payloads and share
the SAME structure: Part 1 (markdown per-file) → binary filtering → Part 2 (non-markdown aggregate).
They differ ONLY in the `git diff` domain args:

| Method           | diffArgs (the variadic passed to detectBinaryFiles/fileStatuses) | call sites                                            |
|------------------|------------------------------------------------------------------|-------------------------------------------------------|
| `StagedDiff`     | `"--cached"`                                                     | pkg/stagecoach/stagecoach.go:414 (runPipeline); internal/generate/generate.go:143 (CommitStaged) |
| `WorkingTreeDiff`| *(none)*                                                          | *(not called in current flow — freeze replaced it; still honor excludes for FR-X3 parity)* |
| `TreeDiff`       | `treeA, treeB`                                                    | internal/decompose/planner.go:69; message.go:71; decompose.go:595 |

The binary filtering block in each is the EXACT template to mirror for `[excluded]`:
```go
binSet, _ := g.detectBinaryFiles(ctx, diffArgs...)      // map[path]bool
statuses, _ := g.fileStatuses(ctx, diffArgs...)          // map[path]status (ALL changed)
binPaths := sorted paths where binSet[path] || isBinaryByExtension(...)
var binExcludes []string                                   // SEPARATE slice (never alias excludes)
for _, path := range binPaths { b.WriteString(binaryPlaceholderLine(...)); binExcludes = append(binExcludes, ":!"+path) }
excludes := opts.Excludes; if len(excludes)==0 { excludes = defaultExcludes }   // ← THIS is the line to change
nmArgs := ["diff", diffArgs..., "--", excludes..., ":!*.md", ":!*.markdown", binExcludes...]
```

## 2. DECISION: REPLACE → UNION (FR-X1)

Current line (`excludes := opts.Excludes; if len(excludes)==0 { excludes = defaultExcludes }`) means a
non-empty `opts.Excludes` REPLACES the built-in denylist. FR-X1 ("Pattern sources (union)") requires
the UNION: built-in denylist (source a) ∪ user globs (sources b+c+d). New code in each method:
```go
excludes := make([]string, 0, len(defaultExcludes)+len(opts.Excludes))
excludes = append(excludes, defaultExcludes...)   // FR3/FR-X1 source (a) — ALWAYS
excludes = append(excludes, opts.Excludes...)      // user union (FR-X1 sources b+c+d)
```
Every caller passes `opts.Excludes == nil` today (architecture note), so the ONLY behavioral change is
when user exclusions are active — exactly the new feature. ✓

## 3. CRITICAL GOTCHA: existing exclude tests assert path ABSENCE → they break under placeholders

Three existing tests assert the excluded path is ENTIRELY absent from `out`:
- `TestStagedDiff_CustomExcludesOverride` (stagediff_test.go:189) — `:!drop.go`, asserts `!Contains(out,"drop.go")`
- `TestTreeDiff_ExcludesApplied` (treediff_test.go:166) — `:!drop.go`, asserts `!Contains(out,"drop.go")`
- `TestWorkingTreeDiff_ExcludesApplied` (workingtreediff_test.go:137) — `:!drop.go`, asserts `!Contains(out,"drop.go")`

With `[excluded]` placeholders, `out` WILL contain the path (inside `A\t[excluded] drop.go`), so these
FAIL. They MUST be updated to assert: (a) the diff HUNK absent (`!Contains(out, "diff --git a/drop.go")`),
(b) the placeholder present (`Contains(out, "[excluded] drop.go")`), (c) keep.go present. Rename
"Override"/"Applied" → "ExcludedPlaceholder" and ADD a UNION-proof test (a `.lock` + a custom-excluded
file both excluded when opts.Excludes is non-empty, proving defaultExcludes still applies).

NOTE the path-format difference: these tests use the raw legacy spelling `:!drop.go`. The real flow
passes `:(exclude,glob)**/drop.go` from ResolveExcludePathspecs. Keep ONE raw-spelling test for
robustness; add `:(exclude,glob)`-spelling tests matching production.

## 4. DECISION: placeholder detection via set-difference (no pathspec conversion)

To know which changed files the USER excludes matched (for the `[excluded]` placeholder), the robust
approach is a git-native SET DIFFERENCE (handles ANY pathspec spelling — `:!` or `:(exclude,glob)`):
1. `statuses = fileStatuses(ctx, diffArgs...)` — ALL changed paths → status (ALREADY computed for binary).
2. Run `git diff <diffArgs> --name-only -- <opts.Excludes>` → the SURVIVING (non-excluded) paths.
3. `excluded = { path ∈ statuses : path ∉ surviving }` (with its status).

This mirrors detectBinaryFiles (one extra `git diff` call) and avoids fragile `:(exclude,glob)`→`:(glob)`
string conversion. Empty `opts.Excludes` ⇒ skip the call entirely (returns nil,nil) — ZERO overhead in
the common case (no .stagecoachignore, no cfg.Exclude).

Placeholder PRECEDENCE: a file that is BOTH binary AND user-excluded gets `[binary]` only (more
informative; binary filtering runs first). Skip paths already in the binary set when emitting
`[excluded]` to avoid double placeholders.

## 5. DECISION: threading — `Excludes []string` on both Deps structs; resolve where repoDir is known

`ResolveExcludePathspecs(cfg, repoRoot, v)` needs repoRoot (to read `.stagecoachignore`). repoDir is
NOT on generate.Deps/decompose.Deps (they carry `git.Git`, which hides workDir). It IS available at:
- `pkg/stagecoach.GenerateCommit`/`Decompose` — `resolveConfig` returns `(cfg, repoDir, err)` via os.Getwd().
- CLI `runDefault` — has `repoDir` from os.Getwd().

Threading:
- `generate.Deps` += `Excludes []string`; `decompose.Deps` (roles.go:55) += `Excludes []string`.
- pkg/stagecoach: after `resolveConfig` + creating `deps.Verbose`, resolve once → `deps.Excludes`
  (covers BOTH GenerateCommit and Decompose public paths — standalone library users get exclusions free).
- CLI `runDecompose` (calls internal/decompose DIRECTLY, not pkg/stagecoach.Decompose — per architecture
  note): pass `repoDir` into runDecompose, resolve there → `deps.Excludes`.
- The 5 StagedDiffOptions literals: add `Excludes: deps.Excludes`.
- NO new pkg/stagecoach.Options field needed (resolution is internal). No import cycle
  (internal/exclude imports only config+ui; pkg/stagecoach already imports those).

## 6. Stub-agent payload capture needs a new env knob

cmd/stubagent/main.go DRAINS stdin to io.Discard (deadlock guard). It captures argv via
`STAGECOACH_STUB_ARGSFILE` but NOT the stdin payload. The work item requires a payload-capture test, so
add `STAGECOACH_STUB_STDINFILE`: when set, tee stdin to that file instead of Discard (still drain fully —
the deadlock guard must hold). Minimal, test-binary-only, follows the existing STAGECOACH_STUB_* pattern.
This lets an integration test assert the agent RECEIVES the placeholder + NOT the excluded body.

## 7. Scope fence

- UPSTREAM (done): `cfg.Exclude` (S1), `internal/exclude.ResolveExcludePathspecs` (S2). Consume only.
- THIS TASK: git.go (3 methods UNION+placeholder) + binary.go (2 helpers) + Deps threading (2 structs,
  5 call sites, 2 resolution sites) + stubagent STDINFILE + tests + e2e + docs/how-it-works.md.
- NOT THIS TASK: hook exec wiring (P1.M3.T2.S1 inherits via StagedDiff for free — do NOT touch hook);
  config `exclude` key (S1); `.stagecoachignore` parsing/translation (S2); `[binary]` placeholder logic
  itself (already shipped — we only ADD the parallel `[excluded]` path).
