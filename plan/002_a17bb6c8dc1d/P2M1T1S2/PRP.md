---
name: "P2.M1.T1.S2 — Integrate binary filtering into StagedDiff (PRD §9.1 FR3a/b/c, staged path)"
description: |

  MODIFY `internal/git/git.go`'s `StagedDiff` so staged binary/non-text files emit the FR3b one-line
  placeholder (`<status>\t[binary] <path>`) and their useless `Binary files … differ` bodies are
  EXCLUDED from the non-markdown aggregate via pathspec `:!<path>`. This is the FIRST of the three FR3c
  application points (staged diff); the same `binary.go` primitives will later serve WorkingTreeDiff
  (P2.M2.T2.S2) and TreeDiff (P2.M2.T1.S2). Also ADD `BinaryExtensions []string` to `StagedDiffOptions`
  and wire it from the two existing `StagedDiff` call sites.

  CONTRACT (P2.M1.T1.S2, verbatim from the work item):
    1. RESEARCH: StagedDiff currently captures markdown per-file (Part 1) + non-markdown aggregate
       (Part 2) with caps/excludes. Binary files slip through as 'Binary files differ' hunks in Part 2.
       The integration must: (a) detect binary files among staged changes, (b) emit placeholder lines,
       (c) exclude their bodies from the non-markdown capture via pathspec excludes. FR3c: binary
       filtering applies in every diff path (this task = staged).
    2. INPUT: the binary detection + placeholder functions from P2.M1.T1.S1 (internal/git/binary.go —
       ALREADY IMPLEMENTED, see findings). The existing StagedDiff method.
    3. LOGIC: In StagedDiff, AFTER the markdown section and BEFORE the non-markdown capture: call
       detectBinaryFiles(ctx, "--cached") → binary path set; call fileStatuses(ctx, "--cached") →
       statuses. For each binary file: append binaryPlaceholderLine(status, path) to the builder;
       add `:!'+path to the non-markdown pathspec excludes so its body doesn't appear. Placeholders
       appear between the markdown and non-markdown sections. Accept BinaryExtensions from
       StagedDiffOptions (new field) to merge with the built-in denylist.
    4. OUTPUT: StagedDiff produces a payload where binary files appear as one-line placeholders instead
       of 'Binary files differ' hunks. Existing markdown/cap/exclude behavior is UNCHANGED for
       non-binary files.
    5. DOCS: none — internal diff-capture change; the agent payload format is documented by the
       placeholder line itself.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `internal/git/binary.go` + `binary_test.go` — S1's (ALREADY IMPLEMENTED). CONSUME ONLY.
    - The exported `Git` INTERFACE — StagedDiff's SIGNATURE is unchanged; BinaryExtensions is a new
      struct FIELD, not a new interface method. Do NOT touch the interface.
    - P2.M2 (RevParseTree/TreeDiff/ReadTree/StatusPorcelain/WorkingTreeDiff) — NOT this task. Those will
      reuse the SAME binary.go primitives for FR3c points 2 & 3.
    - go.mod/go.sum — UNCHANGED (stdlib `sort` only).

  DELIVERABLES (4 files MODIFIED, 0 new files):
    MODIFY internal/git/git.go            — add `BinaryExtensions []string` to StagedDiffOptions;
                                           insert the binary section into StagedDiff (detect + placeholder
                                           + exclude); add `"sort"` import.
    MODIFY internal/git/stagediff_test.go — ADD integration tests for binary filtering (existing 12
                                           tests stay GREEN unchanged).
    MODIFY internal/generate/generate.go  — forward `BinaryExtensions: cfg.BinaryExtensions` at the
                                           StagedDiff call site (CommitStaged, line ~143).
    MODIFY pkg/stagecoach/stagecoach.go     — forward `BinaryExtensions: cfg.BinaryExtensions` at the
                                           StagedDiff call site (public Commit, line ~247).

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all 12 existing stagediff tests still pass;
  new binary-integration tests pass; a staged real-binary PNG emits `A\t[binary] logo.png` and its body
  is absent from the payload; a staged text file is unaffected.

---

## Goal

**Feature Goal**: Wire S1's binary-detection primitives into `StagedDiff` so that PRD §9.1 FR3a/b/c are
realized on the staged-diff path: staged binary/non-text files are detected (numstat UNION extension
denylist, SUPPLEMENTED by the user's `binary_extensions`), each emits the FR3b one-line placeholder
(`<status>\t[binary] <path>`, status from name-status), and its useless `Binary files … differ` hunk is
excluded from the non-markdown aggregate via a pathspec `:!<path>` exclude. Non-binary files are
completely unaffected (markdown per-file capture, caps, lock/snap/map/vendor excludes all unchanged).

**Deliverable** (4 files MODIFIED, 0 new):
1. `internal/git/git.go` — (a) `BinaryExtensions []string` field added to `StagedDiffOptions`; (b) a new
   binary section inserted into `StagedDiff` between Part 1 (markdown) and Part 2 (non-markdown) that
   calls `detectBinaryFiles(ctx, "--cached")` + `fileStatuses(ctx, "--cached")`, emits sorted
   placeholders, and collects `:!path` excludes into a SEPARATE slice (never mutating `defaultExcludes`);
   (c) Part 2's `nmArgs` appends the binary excludes; (d) `import "sort"` added.
2. `internal/git/stagediff_test.go` — NEW test functions proving: a real-binary PNG → placeholder +
   body-excluded; a text file alongside it is unaffected; a user `BinaryExtensions` catches an
   extension not in the built-in denylist; a binary in a subdirectory; mixed markdown+binary+code;
   nothing-staged still yields ""; the two error-path shapes (GitBinaryMissing / ContextCancelled) still
   hold.
3. `internal/generate/generate.go` — one line: add `BinaryExtensions: cfg.BinaryExtensions,` to the
   `git.StagedDiffOptions{…}` literal at the `CommitStaged` call site.
4. `pkg/stagecoach/stagecoach.go` — one line: add `BinaryExtensions: cfg.BinaryExtensions,` to the
   `git.StagedDiffOptions{…}` literal at the public `Commit` call site.

**Success Definition**:
- Staging a real binary (`printf '\x89PNG\r\n\x1a\n\x00' > logo.png; git add logo.png`) plus a text file
  (`echo code > a.go; git add a.go`) and calling `StagedDiff(ctx, StagedDiffOptions{})` yields a payload
  that: CONTAINS the exact line `A\t[binary] logo.png`; does NOT contain `Binary files` or the `logo.png`
  diff hunk body; STILL contains the `a.go` hunk.
- Staging `data.dat` (real-binary content) with `StagedDiffOptions{BinaryExtensions: []string{"dat"}}`
  yields `A\t[binary] data.dat` (proving the user override merges with the built-in denylist — `.dat` is
  NOT in the 36-entry built-in set).
- Staging ONLY text files yields a payload with ZERO `[binary]` lines (binary section is a no-op).
- All 12 existing `TestStagedDiff_*` tests pass UNCHANGED (no edits to them required).
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status` shows EXACTLY the 4 modified files.

## User Persona

**Target User**: the COMMIT-GENERATION agent (and, downstream, the human reading the generated commit).
The binary placeholder is consumed by the agent's prompt: instead of burning prompt budget on a useless
`Binary files … differ` hunk, the agent sees `A\t[binary] assets/logo.png` — enough to group an added
asset with the feature that uses it (PRD §9.1 FR3b rationale) during decomposition.

**Use Case**: a user stages a feature branch that adds code (`feature.go`) AND a binary asset
(`assets/logo.png`). `stagecoach` calls `StagedDiff`; the payload now contains the `feature.go` diff plus
`A\t[binary] assets/logo.png`, not a multi-line binary hunk.

**Pain Points Addressed**: today a staged binary produces a useless, prompt-budget-wasting
`Binary files /dev/null and b/<path> differ` hunk that conveys only "something binary changed". The FR3b
placeholder preserves the filename + change type (A/M/D/R/T) in ONE line.

## Why

- **Closes PRD §9.1 FR3a + FR3b + FR3c (staged path) at the integration layer.** S1 built the primitives;
  S2 is the first wiring (the staged diff, FR1–FR4). FR3c's other two paths (working-tree §13.6.2,
  tree-to-tree §13.6.3) reuse the SAME primitives in P2.M2.
- **Prompt-budget hygiene.** Binary hunks are pure noise to an LLM; the placeholder is signal (filename +
  change kind) at ~1 line vs. a 4-line useless hunk. Matters most for repos with assets.
- **Completes the config→diff path for `binary_extensions`.** The config field already exists
  (`config.Config.BinaryExtensions`, default nil) and is plumbed through file.go, but until S2 forwards
  it into `StagedDiffOptions` AND StagedDiff consumes it, the override is dead. S2 is the last hop.
- **Low-risk, well-isolated change.** S1's primitives are pure/thin and tested; S2 is stitching +
  excludes-append + 2 one-line call-site edits. Existing behavior for non-binary files is byte-identical
  (the 2 extra read-only git calls return empty when no binaries are staged).

## What

A surgical modification to `StagedDiff` (insert a binary section + append excludes) plus a new
`StagedDiffOptions.BinaryExtensions` field threaded through to the two call sites. No new files, no
interface changes, no new dependencies.

### Success Criteria

- [ ] `StagedDiffOptions` has a new `BinaryExtensions []string` field with a doc comment tying it to
      PRD §9.1 FR3a (extra non-text exts to filter; nil ⇒ built-in denylist only).
- [ ] `StagedDiff` calls `detectBinaryFiles(ctx, "--cached")` and `fileStatuses(ctx, "--cached")` AFTER
      the markdown section and BEFORE the non-markdown capture; both errors propagate (`return "", err`).
- [ ] For each binary path (detected OR matched by `isBinaryByExtension(path, opts.BinaryExtensions)`),
      `StagedDiff` writes `binaryPlaceholderLine(status, path)` + `"\n"` to the builder, where `status`
      comes from `fileStatuses`; binary paths are SORTED before emission (deterministic output).
- [ ] Each binary path is added to the non-markdown excludes as `":!" + path` via a SEPARATE slice
      (`binExcludes`) appended to `nmArgs` — NEVER appended to `excludes` (which may alias
      `defaultExcludes`; mutating it corrupts the package var).
- [ ] A staged binary's `Binary files … differ` body is ABSENT from the payload; a staged text file
      alongside it is fully present.
- [ ] All 12 existing `TestStagedDiff_*` tests pass unchanged; new binary-integration tests pass.
- [ ] `generate.go` and `stagecoach.go` forward `BinaryExtensions: cfg.BinaryExtensions` at their
      `StagedDiffOptions{…}` literals.
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact `binary.go`
interface to consume (findings §1, the file is already implemented — signatures verbatim); the exact
StagedDiff structure to modify (git.go, the markdown/non-markdown two-part capture); the verified
`:!<path>` exclude behavior (findings §3); the rename-key reconciliation rule (findings §4: key off
`fileStatuses` destinations); the `defaultExcludes`-mutation trap (findings §5: use a separate
`binExcludes` slice); the placement rule (findings §6: between Part 1 and Part 2); the deterministic-sort
note (findings §7); the test fixtures (findings §9: reuse `initRepo`/`writeFile`/`stageFile`); and the
exact call-site locations (findings §2). No prompt/decompose/registry knowledge required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (wire formats + integration traps, all verified)
- docfile: plan/002_a17bb6c8dc1d/P2M1T1S2/research/findings.md
  why: §1 the EXACT binary.go interface to consume + WHY detectBinaryFiles can't carry extraExts (S2 must
       supplement via isBinaryByExtension); §2 the two call-site locations; §3 the verified :!<path>
       exclude; §4 the rename-key reconciliation (key off fileStatuses); §5 the defaultExcludes-mutation
       TRAP (separate binExcludes slice); §6 placement (between Part 1 and Part 2); §7 sort for determinism;
       §8 trace proving all 12 existing tests pass; §9 test fixtures; §10 lint/scope facts.
  critical: §1 (detectBinaryFiles hardcodes nil extraExts ⇒ S2 supplements), §5 (mutating defaultExcludes
       corrupts the package var across calls/tests).

# MUST READ — S1's PRP (the CONTRACT for the primitives S2 consumes; binary.go is already implemented to it)
- docfile: plan/002_a17bb6c8dc1d/P2M1T1S1/PRP.md
  section: "Data models and structure" (the 5 symbol signatures) + "S2 COORDINATION" (the handoff: S2 calls
       detectBinaryFiles(ctx,"--cached") + fileStatuses(ctx,"--cached"); reconciles renames by keying off
       name-status destinations).
  critical: confirms detectBinaryFiles/fileStatuses are UNEXPORTED *gitRunner methods (same package ⇒ S2 in
       git.go calls them directly, no interface change); the rename `=>` coordination note.

# MUST READ — the FILE TO MODIFY: StagedDiff + StagedDiffOptions + run() + defaultExcludes (READ then EDIT)
- file: internal/git/git.go
  section: `StagedDiffOptions` struct (add BinaryExtensions); `defaultExcludes` var (DO NOT MUTATE);
       `StagedDiff` (insert binary section between the markdown loop and the `// ---- Part 2` block;
       append binExcludes to nmArgs); the `run()` INVARIANT comment (err for infrastructural only,
       code!=0 wrapped — mirror StagedDiff's existing err shape for the 2 new calls).
  why: StagedDiff is THE method being modified. Its two-part structure (markdown per-file, then
       non-markdown aggregate with excludes + byte cap) is the integration target.
  pattern: copy the existing err+code shape (`if err != nil { return "", err }; if code != 0 { return "",
       fmt.Errorf(...) }`) — the new detectBinaryFiles/fileStatuses calls already implement that shape
       INTERNALLY and return a clean error, so StagedDiff just does `if err != nil { return "", err }`.
  gotcha: `excludes := opts.Excludes; if len(excludes)==0 { excludes = defaultExcludes }` ALIASES the
       package var — appending to `excludes` mutates it. Collect binary excludes in a SEPARATE slice.

# MUST READ — the primitives file (ALREADY IMPLEMENTED by S1; CONSUME, do not edit)
- file: internal/git/binary.go   (READ-ONLY)
  section: `isBinaryByExtension(path, extraExts) bool`; `binaryPlaceholderLine(status, path) string`;
       `(*gitRunner).detectBinaryFiles(ctx, diffArgs...) (map[string]bool, error)`;
       `(*gitRunner).fileStatuses(ctx, diffArgs...) (map[string]string, error)`.
  why: these are the exact symbols StagedDiff calls. Same package `git` ⇒ direct calls, no import.
  pattern: `g.detectBinaryFiles(ctx, "--cached")` returns the binary SET (numstat + built-in denylist);
       `g.fileStatuses(ctx, "--cached")` returns dest-path → status.
  gotcha: detectBinaryFiles applies the BUILT-IN denylist ONLY (nil extraExts). The user override
       (opts.BinaryExtensions) must be applied SEPARATELY in StagedDiff via isBinaryByExtension.

# MUST READ — the test file to EXTEND (existing tests must stay green; add new ones)
- file: internal/git/stagediff_test.go   (READ then EDIT — add functions only)
  section: the 12 existing TestStagedDiff_* functions (the patterns to mirror: initRepo+writeFile+stageFile
       + New(repo) + StagedDiff + strings.Contains/Count assertions); TestStagedDiff_GitBinaryMissing
       (t.Setenv("PATH","")) and _ContextCancelled (pre-cancel ctx) shapes.
  why: S2 adds binary-integration tests HERE (same package, same fixtures). errcheck is DISABLED for this
       file in .golangci.yml, but keep gosimple/govet/staticcheck/unused clean.
  pattern: real binary = writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00"); git sniffs it ⇒
       numstat -/-. user-override isolation = ".dat" (NOT in built-in 36) with binary content.

# MUST READ — the config field (ALREADY EXISTS; S2 forwards it from the 2 call sites)
- file: internal/config/config.go   (READ-ONLY)
  section: `Config.BinaryExtensions []string toml:"binary_extensions"` (line ~81; default nil ⇒ built-in
       denylist only).
  why: confirms the field exists and is named `BinaryExtensions`; S2 forwards `cfg.BinaryExtensions` into
       `StagedDiffOptions.BinaryExtensions` at the 2 call sites. No config work needed.

# MUST READ — the design reference (signatures + integration sketch)
- docfile: plan/002_a17bb6c8dc1d/architecture/binary_git_v2.md   (READ-ONLY)
  section: "1. Binary/Non-Text Filtering" → "StagedDiff Integration" (steps 1–5). NOTE: the doc's sketch
       uses the old names (detectBinaryPaths/NumStatEntry) and omits the user-override supplement — the
       WORK-ITEM CONTRACT + S1's actual binary.go OVERRIDE both. Follow the CONTRACT + binary.go, not the
       doc's sketch.
  critical: confirms the 5-step integration (numstat → name-status → placeholder → exclude → normal
       capture) and that the SAME primitives serve all 3 FR3c paths.

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §9.1 FR3a / FR3b / FR3c
  why: FR3a (numstat primary + 36-ext denylist, overridable via binary_extensions); FR3b (placeholder
       `<status>\t[binary] <path>`, status A/M/D/R/T from name-status); FR3c (applies in EVERY diff path;
       this task = staged, FR1–FR4).
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go              # MODIFY: StagedDiffOptions (add BinaryExtensions) + StagedDiff (binary section) + import "sort".
  binary.go           # READ-ONLY (S1, already implemented): the 5 primitives StagedDiff consumes.
  binary_test.go      # READ-ONLY (S1): primitive tests.
  stagediff_test.go   # MODIFY: ADD binary-integration tests (12 existing tests stay green).
  git_test.go         # READ: defines initRepo(t, dir).
  committree_test.go  # READ: defines writeFile/stageFile/setIdentityConfig/writeTreeOf/headSHA/commitMessage.
  (*_test.go)         # READ: other exemplars (same package git).
internal/generate/generate.go   # MODIFY: StagedDiff call site (~143) — add BinaryExtensions: cfg.BinaryExtensions.
pkg/stagecoach/stagecoach.go      # MODIFY: StagedDiff call site (~247) — add BinaryExtensions: cfg.BinaryExtensions.
internal/config/config.go       # READ-ONLY: Config.BinaryExtensions already exists (default nil).
go.mod / go.sum      # UNCHANGED (stdlib "sort" only).
.golangci.yml        # READ: stagediff_test.go is errcheck-EXEMPT (but other linters still apply).
```

### Desired Codebase tree with files to be MODIFIED

```bash
internal/git/git.go              # MODIFY — StagedDiffOptions gains `BinaryExtensions []string`;
                                 #   StagedDiff gains a binary section (detect + placeholder + exclude)
                                 #   between Part 1 and Part 2; `import "sort"` added. NO interface change.
internal/git/stagediff_test.go   # MODIFY — ADD (do not alter existing) test functions:
                                 #   TestStagedDiff_BinaryFilePlaceholderAndExcluded
                                 #   TestStagedDiff_BinaryKeepsTextCompanion
                                 #   TestStagedDiff_BinaryExtensionsUserOverride
                                 #   TestStagedDiff_BinaryInSubdirectory
                                 #   TestStagedDiff_MixedMarkdownBinaryCode
                                 #   TestStagedDiff_NoBinaryWhenOnlyText (regression guard)
internal/generate/generate.go    # MODIFY — 1 line: `BinaryExtensions: cfg.BinaryExtensions,` in the
                                 #   git.StagedDiffOptions{…} literal at CommitStaged.
pkg/stagecoach/stagecoach.go       # MODIFY — 1 line: `BinaryExtensions: cfg.BinaryExtensions,` in the
                                 #   git.StagedDiffOptions{…} literal at the public Commit.
# go.mod/go.sum UNCHANGED. binary.go/binary_test.go UNCHANGED. The Git interface UNCHANGED. 0 new files.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (detectBinaryFiles hardcodes nil extraExts — findings §1): S1's detectBinaryFiles applies the
// BUILT-IN denylist only (its internal union is `(added=="-"&&deleted=="-") || isBinaryByExtension(path,
// nil)`). The user's binary_extensions override CANNOT reach detection through detectBinaryFiles. ⇒ S2
// SUPPLEMENTS in StagedDiff: after calling detectBinaryFiles, ALSO test every path in fileStatuses via
// isBinaryByExtension(path, opts.BinaryExtensions). The S2 union predicate is:
//   binSet[path] || isBinaryByExtension(path, opts.BinaryExtensions)

// CRITICAL (defaultExcludes ALIASING — findings §5): the existing code does
//   `excludes := opts.Excludes; if len(excludes)==0 { excludes = defaultExcludes }` — when opts.Excludes
// is empty, `excludes` ALIASES the package-level `defaultExcludes` var. Appending binary excludes to
// `excludes` directly would MUTATE `defaultExcludes` (corrupting it for every later call/test — flaky
// tests, wrong payloads). ⇒ collect binary excludes in a SEPARATE `binExcludes []string` slice and append
// it to `nmArgs` (which copies), NEVER to `excludes`. This mirrors why the existing code appends
// `:!*.md`/`:!*.markdown` to `nmArgs`, not to `excludes`.

// CRITICAL (rename key mismatch — findings §4): detectBinaryFiles keys renames as the numstat `old => new`
// string; fileStatuses keys by the clean destination `new`. They DIFFER for renames. ⇒ S2 iterates over
// FILESTATUSES (destination keys) and looks the binary set up BY destination: `binSet[<dest>]` matches for
// non-rename A/M/D (same path in both); for a rename the dest-keyed lookup misses the `=> ` key, BUT
// isBinaryByExtension(<dest>, …) catches .png/.jpg/etc. via the denylist. So the union still catches
// binary renames via the extension signal. The orphaned `=> ` key in binSet is never read (harmless).
// Accepted edge case (document, do NOT over-engineer): a renamed binary with NO denylisted extension AND
// not in BinaryExtensions that git sniffs as binary — missed by the dest-keyed lookup. Vanishingly rare.

// GOTCHA (placement — findings §6): the binary section goes AFTER the markdown loop (Part 1) and BEFORE
// the `// ---- Part 2` non-markdown block. This keeps the markdown-list call as StagedDiff's FIRST git
// invocation, so TestStagedDiff_GitBinaryMissing / _ContextCancelled (which fail on the first call) keep
// their exact error shape — NO edit to those two tests needed.

// GOTCHA (byte cap does NOT count placeholders): the existing `max_diff_bytes` cap is applied to `nmDiff`
// ONLY (the non-markdown body). Binary placeholders are tiny metadata lines that REPLACE binary bodies;
// they are written to the builder BEFORE the capped `nmDiff` and are NOT counted against the cap. Do not
// move the cap to cover the whole builder.

// GOTCHA (deterministic output — findings §7): fileStatuses returns a map ⇒ non-deterministic iteration.
// SORT the collected binary paths (sort.Strings) before emitting placeholders so the payload (and tests)
// are reproducible. Requires `import "sort"` (stdlib; git.go does not currently import it — ADD it).

// GOTCHA (pathspec :!<path> is glob-interpreted — findings §3): `:!` content is treated as a glob by git
// (that's why `:!*.lock` works). For a literal binary path WITHOUT glob metacharacters (the common case:
// logo.png, assets/img.png), `:!" + path` matches exactly that path. A binary path containing literal
// `*`/`?`/`[` would be misinterpreted — an accepted limitation (binary asset paths almost never contain
// glob metacharacters; FR3b examples are assets/logo.png, public/trailer.mp4). Do NOT add escaping.

// GOTCHA (do NOT edit binary.go / binary_test.go): S1 owns them (already implemented). S2 only CONSUMES
// the package-level symbols. Editing them = scope violation + a conflict with S1.

// GOTCHA (do NOT change the Git interface): StagedDiff's SIGNATURE is unchanged. BinaryExtensions is a new
// STRUCT FIELD, not a new method. Adding a method to the interface would be out of scope.

// GOTCHA (2 extra read-only git calls per StagedDiff): the binary section adds detectBinaryFiles +
// fileStatuses (both `git diff --cached --numstat` / `--name-status`, read-only). StagedDiff is NOT hot
// (once per commit generation), so the cost is negligible. When no binaries are staged both return empty
// maps and the section is a no-op (placeholders loop writes nothing).
```

## Implementation Blueprint

### Data models and structure

No new TYPES. The only data-model change is one new FIELD on the existing `StagedDiffOptions` struct:

```go
// StagedDiffOptions configures staged-diff capture (commit-pi parity, PRD §9.1 / FINDING 7).
type StagedDiffOptions struct {
	MaxDiffBytes int      // byte cap on the non-markdown section (commit-pi default 300000); 0 = unlimited
	MaxMDLines   int      // per-file line cap for markdown files (commit-pi default 100); 0 = unlimited
	Excludes     []string // pathspec magic-prefix excludes, e.g. []string{":!*.lock", ":!vendor/*"}
	BinaryExtensions []string // NEW (FR3a): extra non-text extensions to filter BEYOND the built-in
	                          // denylist (png jpg … woff2 in internal/git/binary.go). Entries are
	                          // dot-tolerant + case-insensitive (see isBinaryByExtension). nil ⇒ the
	                          // built-in denylist only. Sourced from config `binary_extensions`.
}
```

The integration consumes S1's existing symbols (no new types, no new methods):

```go
// from internal/git/binary.go (S1, already implemented — CONSUME):
//   func isBinaryByExtension(path string, extraExts []string) bool
//   func binaryPlaceholderLine(status, path string) string
//   func (g *gitRunner) detectBinaryFiles(ctx context.Context, diffArgs ...string) (map[string]bool, error)
//   func (g *gitRunner) fileStatuses(ctx context.Context, diffArgs ...string) (map[string]string, error)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/git/git.go — add `import "sort"` + the BinaryExtensions field
  - EDIT the import block: add `"sort"` to the stdlib import group (between "os/exec" and "strconv", or
    anywhere alphabetically correct within the stdlib group). Keep gofmt-happy grouping.
  - EDIT the `StagedDiffOptions` struct: add `BinaryExtensions []string` with the doc comment above
    (FR3a; dot-tolerant + case-insensitive; nil ⇒ built-in denylist only; sourced from config).
  - NAMING: `BinaryExtensions` (matches config.Config.BinaryExtensions + the PRD `binary_extensions` key).
  - GOTCHA: this is a struct FIELD, NOT an interface method. The `Git` interface is UNCHANGED.
  - PLACEMENT: internal/git/git.go (the StagedDiffOptions struct is ~line 30-40; the import block is top).

Task 2: MODIFY internal/git/git.go — insert the binary section into StagedDiff + append excludes to Part 2
  - LOCATE the boundary: the end of the markdown loop (the `for _, file := range ...` block, after the
    `b.WriteByte('\n')` boundary-ensuring append) and the `// ---- Part 2: non-markdown ...` comment.
  - INSERT (between them) the binary section EXACTLY as in "Implementation Patterns" below:
      (a) `binSet, berr := g.detectBinaryFiles(ctx, "--cached")` ; `if berr != nil { return "", berr }`.
      (b) `statuses, serr := g.fileStatuses(ctx, "--cached")` ; `if serr != nil { return "", serr }`.
      (c) collect binary paths into a sorted slice: for each dest-path in `statuses`, if
          `binSet[path] || isBinaryByExtension(path, opts.BinaryExtensions)` → append to `binPaths`;
          then `sort.Strings(binPaths)`.
      (d) for each sorted path: `b.WriteString(binaryPlaceholderLine(statuses[path], path))`;
          `b.WriteByte('\n')`; `binExcludes = append(binExcludes, ":!"+path)`.
      - `binExcludes` is a SEPARATE `[]string` (declared with `var binExcludes []string`) — NEVER append
        to `excludes` (which may alias defaultExcludes — findings §5).
  - EDIT the Part-2 `nmArgs` build: AFTER `nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")`, ADD
      `nmArgs = append(nmArgs, binExcludes...)`. Leave the rest of Part 2 (run, err check, byte cap,
      `b.WriteString(nmDiff)`) UNCHANGED.
  - PATTERN: mirror StagedDiff's existing err shape for the 2 new calls (`if err != nil { return "", err }`).
      detectBinaryFiles/fileStatuses already wrap code!=0 internally, so StagedDiff only propagates.
  - GOTCHA: placement is AFTER Part 1, BEFORE Part 2 (findings §6). The markdown-list call stays first.
  - GOTCHA: do NOT touch the byte cap (it covers nmDiff only — findings §6).
  - PLACEMENT: internal/git/git.go, inside `(g *gitRunner) StagedDiff`.

Task 3: MODIFY internal/git/stagediff_test.go — ADD binary-integration tests (existing 12 stay green)
  - ADD `import "strings"` is already present; ensure no new imports needed beyond what's there.
  - ADD TestStagedDiff_BinaryFilePlaceholderAndExcluded: initRepo; writeFile a REAL binary
      (`"\x89PNG\r\n\x1a\n\x00\x00\x00"`) as logo.png + text a.go; stage both; StagedDiff({}) ⇒ assert
      `strings.Contains(out, "A\t[binary] logo.png")`; assert `!strings.Contains(out, "Binary files")`;
      assert `!strings.Contains(out, "logo.png\n")` style binary hunk markers (the body is gone); assert
      `strings.Contains(out, "a.go")` (text companion present). Use a helper to write real-binary bytes.
  - ADD TestStagedDiff_BinaryExtensionsUserOverride: writeFile real-binary content to `data.dat`
      (.dat is NOT in the 36-entry built-in denylist); stage; StagedDiff({BinaryExtensions:[]string{"dat"}})
      ⇒ assert `strings.Contains(out, "A\t[binary] data.dat")` AND `!strings.Contains(out, "Binary files")`.
      (Proves the user override merges with — extends — the built-in denylist.)
  - ADD TestStagedDiff_BinaryExtensionsNilDoesNotCatchDat: same data.dat staged; StagedDiff({}) (nil override)
      ⇒ assert `!strings.Contains(out, "[binary] data.dat")` (without the override, .dat is NOT filtered —
      it's not in the built-in denylist; its body appears as a normal binary hunk OR is sniffed by numstat).
      NOTE: since data.dat has real-binary CONTENT, numstat -/- WILL flag it via detectBinaryFiles even
      with nil override — so this test must use TEXT content for data.dat to truly isolate the extension
      signal. REVISE: writeFile TEXT ("hello\n") to data.dat; StagedDiff({}) ⇒ NOT caught (text content,
      non-denylist ext); StagedDiff({BinaryExtensions:[]string{"dat"}}) ⇒ caught via extension. This
      isolates the extension signal from numstat (mirrors S1's fake.png test).
  - ADD TestStagedDiff_BinaryInSubdirectory: writeFile real binary to `assets/logo.png`; stage; StagedDiff({})
      ⇒ assert `strings.Contains(out, "A\t[binary] assets/logo.png")`; body excluded.
  - ADD TestStagedDiff_MixedMarkdownBinaryCode: stage a.md (markdown) + logo.png (real binary) + b.go
      (text); StagedDiff({}) ⇒ assert a.md hunk present (Part 1), `A\t[binary] logo.png` present, b.go hunk
      present (Part 2), NO `Binary files` anywhere. (End-to-end: all three sections coexist.)
  - ADD TestStagedDiff_NoBinaryWhenOnlyText: stage only a.go + b.go (text); StagedDiff({}) ⇒ assert
      `!strings.Contains(out, "[binary]")` (regression guard: binary section is a no-op for text-only).
  - PATTERN: reuse initRepo/writeFile/stageFile (git_test.go + committree_test.go). For real-binary bytes
      use a string literal with NUL/PNG header. Assert via strings.Contains/Count (order-independent,
      matching existing tests — even though output is sorted, Contains is the established idiom).
  - COVERAGE: numstat signal (real binary), extension signal (user override, isolated via text content),
      subdirectory paths, mixed payload, no-binary regression. (Error paths GitBinaryMissing/
      ContextCancelled are ALREADY covered by existing tests — no change needed.)
  - PLACEMENT: internal/git/stagediff_test.go (append new functions; do NOT alter existing 12).

Task 4: MODIFY internal/generate/generate.go + pkg/stagecoach/stagecoach.go — forward BinaryExtensions
  - EDIT generate.go (~line 143, inside CommitStaged's `git.StagedDiffOptions{ … }` literal): add
      `BinaryExtensions: cfg.BinaryExtensions,` alongside the existing `MaxDiffBytes`/`MaxMDLines` lines.
  - EDIT stagecoach.go (~line 247, inside the public Commit's `git.StagedDiffOptions{ … }` literal): add
      the same line.
  - GOTCHA: `cfg.BinaryExtensions` defaults to nil (config Defaults()) ⇒ StagedDiff treats nil as
      "built-in denylist only" ⇒ byte-identical behavior to today for default config (back-compatible).
  - GOTCHA: cfg is `config.Config` (passed into CommitStaged/deps) — verify the field name is exactly
      `BinaryExtensions` (config.go:81). It is.
  - PLACEMENT: the two StagedDiff call sites (1 line each).

Task 5: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/git/git.go internal/git/stagediff_test.go internal/generate/generate.go pkg/stagecoach/stagecoach.go`
  - `go build ./... && go vet ./...`
  - `go test -race ./internal/git/ -run "TestStagedDiff" -v`   (all 12 existing + new binary tests)
  - `go test -race ./internal/generate/ ./pkg/stagecoach/`      (call-site wiring doesn't break consumers)
  - `golangci-lint run ./...`   (errcheck/gosimple/govet/ineffassign/staticcheck/unused)
  - `go test ./...`             (FULL regression)
  - `git diff --exit-code go.mod go.sum` ⇒ empty.
  - `git status --short` ⇒ EXACTLY 4 modified files (git.go, stagediff_test.go, generate.go, stagecoach.go);
    0 new files; binary.go/binary_test.go UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// === Task 1+2: the StagedDiff binary section (insert between Part 1 and Part 2) ===
//
// Imports: ADD "sort" to git.go's stdlib import group.

// StagedDiffOptions — add the field:
type StagedDiffOptions struct {
	MaxDiffBytes     int
	MaxMDLines       int
	Excludes         []string
	BinaryExtensions []string // NEW (FR3a): extra non-text exts beyond the built-in denylist; nil ⇒ built-in only
}

// Inside StagedDiff, AFTER the markdown loop (Part 1) and BEFORE `// ---- Part 2`:
//
// ---- Binary filtering (PRD §9.1 FR3a/b/c, staged path) ----
// detectBinaryFiles applies numstat + the BUILT-IN denylist (S1 hardcodes nil extraExts); supplement
// with the user's BinaryExtensions below. Key off fileStatuses (destination paths) to reconcile renames
// (numstat `old => new` vs name-status `new` — findings §4).
binSet, berr := g.detectBinaryFiles(ctx, "--cached")
if berr != nil {
	return "", berr
}
statuses, serr := g.fileStatuses(ctx, "--cached")
if serr != nil {
	return "", serr
}

// Collect binary paths (FR3a union: detected-by-numstat/denylist OR matched by user BinaryExtensions),
// SORT for deterministic output, emit FR3b placeholders, and gather pathspec excludes.
binPaths := make([]string, 0, len(statuses))
for path := range statuses {
	if binSet[path] || isBinaryByExtension(path, opts.BinaryExtensions) {
		binPaths = append(binPaths, path)
	}
}
sort.Strings(binPaths)
var binExcludes []string // SEPARATE slice — never append to `excludes` (it may alias defaultExcludes, §5)
for _, path := range binPaths {
	b.WriteString(binaryPlaceholderLine(statuses[path], path)) // "<status>\t[binary] <path>"
	b.WriteByte('\n')
	binExcludes = append(binExcludes, ":!"+path)
}

// ---- Part 2: non-markdown (existing block, UNCHANGED except binExcludes appended to nmArgs) ----
excludes := opts.Excludes
if len(excludes) == 0 {
	excludes = defaultExcludes
}
nmArgs := []string{"diff", "--cached", "--"}
nmArgs = append(nmArgs, excludes...)
nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
nmArgs = append(nmArgs, binExcludes...) // NEW — drop binary bodies from the aggregate
nmDiff, nmstderr, nmcode, nmerr := g.run(ctx, g.workDir, nmArgs...)
// ... (existing err check + byte cap + b.WriteString(nmDiff) UNCHANGED) ...

// PATTERN (err shape): detectBinaryFiles/fileStatuses already wrap code!=0 internally (S1), so StagedDiff
// only does `if err != nil { return "", err }` — mirroring how it already handles the markdown per-file
// run() calls. NO code!=0 branch needed at the StagedDiff layer for these two calls.

// CRITICAL (defaultExcludes aliasing — §5): when opts.Excludes is empty, `excludes` POINTS AT the
// package-level defaultExcludes. `binExcludes` is a fresh local slice; appending it to `nmArgs` COPIES
// (nmArgs = append(nmArgs, binExcludes...) never mutates binExcludes' backing array beyond it, and never
// touches defaultExcludes). This is exactly why the existing code appends `:!*.md` to nmArgs, not excludes.

// CRITICAL (union predicate — §1): `binSet[path]` covers numstat-sniffed binaries + built-in-denylist
// matches (detectBinaryFiles' internal union). `isBinaryByExtension(path, opts.BinaryExtensions)` adds
// the USER override. Together: the full FR3a union. For a rename, binSet holds the `=> ` key (miss on
// dest lookup) but isBinaryByExtension(<dest>) catches .png etc. via the denylist.
```

### Integration Points

```yaml
INTERNAL/GIT (internal/git/git.go — MODIFY):
  - StagedDiffOptions: "+ BinaryExtensions []string (FR3a; nil ⇒ built-in denylist only)."
  - StagedDiff: "+ binary section between Part 1 and Part 2 (detectBinaryFiles + fileStatuses + sorted
          placeholders + binExcludes); nmArgs appends binExcludes. Existing markdown/cap/exclude logic
          for non-binary files is byte-identical."
  - imports: "+ \"sort\" (stdlib)."

INTERNAL/GIT BINARY.GO (CONSUME — UNCHANGED):
  - StagedDiff calls: isBinaryByExtension, binaryPlaceholderLine, (*gitRunner).detectBinaryFiles,
          (*gitRunner).fileStatuses (all same package git; S1 already implemented them).

CONFIG WIRING (the 2 call sites — MODIFY, 1 line each):
  - internal/generate/generate.go (~143): "+ BinaryExtensions: cfg.BinaryExtensions," in the
          git.StagedDiffOptions{…} literal at CommitStaged.
  - pkg/stagecoach/stagecoach.go (~247): "+ BinaryExtensions: cfg.BinaryExtensions," in the
          git.StagedDiffOptions{…} literal at the public Commit.
  - config.Config.BinaryExtensions ALREADY EXISTS (config.go:81, default nil) and is plumbed through
          file.go. No config-layer work.

GIT INTERFACE (internal/git/git.go Git interface — UNCHANGED):
  - no-change: "StagedDiff's signature is unchanged; BinaryExtensions is a struct field, not a method."

GO.MODULE: change NONE. stdlib "sort" only. No new deps.

FROZEN/LEAVE (do NOT edit):
  - internal/git/binary.go + binary_test.go (S1 — CONSUME only).
  - internal/config/* (the field exists; no config work).
  - The Git interface (StagedDiff signature unchanged).
  - P2.M2 methods (RevParseTree/TreeDiff/ReadTree/StatusPorcelain/WorkingTreeDiff) — they will reuse the
    SAME binary.go primitives for FR3c points 2 & 3; not this task.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
gofmt -w internal/git/git.go internal/git/stagediff_test.go internal/generate/generate.go pkg/stagecoach/stagecoach.go
go vet ./...
golangci-lint run ./...
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; golangci-lint clean (stagediff_test.go is errcheck-EXEMPT; other linters apply);
#           go.mod/go.sum unchanged. NOTE: run `go vet ./...` (not just ./internal/git/) since generate.go
#           and stagecoach.go are also modified.
```

### Level 2: Unit Tests (Component Validation)

```bash
# All StagedDiff tests (existing 12 + new binary-integration tests), verbose:
go test -race ./internal/git/ -run "TestStagedDiff" -v
# Expected: all green. Specifically the NEW tests:
#   TestStagedDiff_BinaryFilePlaceholderAndExcluded  ⇒ contains "A\t[binary] logo.png"; NO "Binary files"; a.go present
#   TestStagedDiff_BinaryExtensionsUserOverride      ⇒ contains "A\t[binary] data.dat" (text-content .dat + user override)
#   TestStagedDiff_BinaryInSubdirectory              ⇒ contains "A\t[binary] assets/logo.png"
#   TestStagedDiff_MixedMarkdownBinaryCode           ⇒ a.md + placeholder + b.go all present; NO "Binary files"
#   TestStagedDiff_NoBinaryWhenOnlyText              ⇒ NO "[binary]" anywhere
# And the EXISTING 12 (MarkdownAndCode, ExcludesLockSnapMapVendor, MarkdownNotDoubleCounted,
#   MarkdownLineCap, NonMarkdownByteCap, NothingStaged, OnlyMarkdown, OnlyCode, CustomExcludesOverride,
#   DefaultsOnZero, MarkdownExtensions, GitBinaryMissing, ContextCancelled) all still pass UNCHANGED.

# Call-site consumers still build + test (wiring doesn't break them; cfg.BinaryExtensions defaults nil):
go test -race ./internal/generate/ ./pkg/stagecoach/

# Full regression:
go test ./...
# Expected: GREEN across all packages (binary.go untouched; the only behavior change is additive
#           placeholders + excludes, which the new tests pin and the existing tests are unaffected by).
```

### Level 3: Integration / Behavioral Proof (the binary-body exclusion)

```bash
# Reproduce empirically: a staged real binary MUST produce a placeholder and DROP its body.
T=$(mktemp -d); cd "$T"; git init -q .; git config user.email t@t.co; git config user.name t
printf '\x89PNG\r\n\x1a\n\x00\x00\x00' > logo.png   # genuine binary (git sniffs ⇒ numstat -/-)
echo "package main" > a.go
git add logo.png a.go
echo "--- numstat (logo.png is -/-) ---"
git diff --cached --numstat
echo "--- the useless hunk that StagedDiff used to emit ---"
git diff --cached -- logo.png
echo "--- what StagedDiff now produces (a.go body + :!logo.png exclude ⇒ no binary hunk) ---"
git diff --cached -- a.go ':!logo.png'
echo "--- the placeholder line StagedDiff emits (status from name-status) ---"
git diff --cached --name-status | awk -F'\t' '$1!=""{print $1"\t[binary] "$NF}'
# Expected placeholder: "A	[binary] logo.png"
cd /; rm -rf "$T"
# (The Go tests in Level 2 assert exactly this end-to-end through StagedDiff.)
```

### Level 4: Regression & Audit

```bash
cd /home/dustin/projects/stagecoach
go build ./...                 # whole module compiles (git.go, generate.go, stagecoach.go all modified)
go test ./...                  # FULL regression
git status --short             # Expected: EXACTLY 4 modified files:
                               #    M internal/git/git.go
                               #    M internal/git/stagediff_test.go
                               #    M internal/generate/generate.go
                               #    M pkg/stagecoach/stagecoach.go
                               # 0 new files; binary.go/binary_test.go UNCHANGED.
git diff --stat internal/git/binary.go internal/git/binary_test.go   # Expected: empty (S1's files untouched)
# Expected: build + full test green; only 4 modified files; S1's files untouched; go.mod/go.sum unchanged.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l internal/ pkg/` empty; `go vet ./...` clean; `golangci-lint run ./...` clean.
- [ ] Level 2: `go test -race ./internal/git/ -run TestStagedDiff` green — 12 existing + new binary tests.
- [ ] Level 2: `go test -race ./internal/generate/ ./pkg/stagecoach/` green (call-site wiring safe).
- [ ] Level 3: empirical proof — staged real binary ⇒ placeholder `A\t[binary] logo.png`, body absent.
- [ ] Level 4: `go build ./...` + `go test ./...` green; `git status` shows EXACTLY 4 modified files;
      binary.go/binary_test.go + go.mod/go.sum UNCHANGED.

### Feature Validation

- [ ] `StagedDiffOptions.BinaryExtensions []string` exists with an FR3a doc comment; nil ⇒ built-in only.
- [ ] A staged real binary (PNG header) emits `A\t[binary] logo.png` and its `Binary files … differ` body
      is ABSENT from the payload.
- [ ] A staged text file alongside the binary is fully present (its hunk survives).
- [ ] A user `BinaryExtensions: []string{"dat"}` catches a text-content `.dat` (isolating the extension
      signal from numstat) — proving the override MERGES with (extends) the built-in denylist.
- [ ] A binary in a subdirectory (`assets/logo.png`) emits `A\t[binary] assets/logo.png`.
- [ ] Markdown per-file capture (Part 1), the byte cap, and the lock/snap/map/vendor excludes are all
      UNCHANGED for non-binary files (existing 12 tests prove this).
- [ ] `generate.go` + `stagecoach.go` forward `BinaryExtensions: cfg.BinaryExtensions` (default nil ⇒
      back-compatible).

### Code Quality Validation

- [ ] Binary paths are SORTED before placeholder emission (deterministic output).
- [ ] Binary excludes collected in a SEPARATE `binExcludes` slice — `defaultExcludes` is NEVER mutated.
- [ ] The union predicate is `binSet[path] || isBinaryByExtension(path, opts.BinaryExtensions)`
      (detectBinaryFiles' built-in union + the user supplement).
- [ ] Binary section placed AFTER Part 1, BEFORE Part 2 (markdown-list call stays first ⇒ existing error-
      path tests keep their shape).
- [ ] The `Git` interface is UNCHANGED; binary.go/binary_test.go UNCHANGED; go.mod/go.sum UNCHANGED.
- [ ] Anti-patterns avoided (see below): no defaultExcludes mutation, no interface change, no binary.go
      edit, no byte-cap-on-placeholders, no rename over-engineering.

### Documentation & Deployment

- [ ] `BinaryExtensions` field has a doc comment (FR3a; dot-tolerant + case-insensitive; nil ⇒ built-in).
- [ ] The inserted binary section has a comment naming PRD §9.1 FR3a/b/c + the staged-path scope.
- [ ] Implementation summary records: the field, the binary section, the call-site wiring, the
      defaultExcludes-aliasing trap, and the rename-key reconciliation.

---

## Anti-Patterns to Avoid

- ❌ **Don't append binary excludes to `excludes`.** When `opts.Excludes` is empty, `excludes` ALIASES the
  package-level `defaultExcludes` var; appending to it mutates shared state and corrupts later calls/tests
  (flaky tests, wrong payloads). Use a SEPARATE `binExcludes` slice and append it to `nmArgs` (which copies).
- ❌ **Don't rely on `detectBinaryFiles` alone for the user override.** S1's `detectBinaryFiles` hardcodes
  `nil` extraExts (built-in denylist only). The user's `binary_extensions` MUST be applied separately via
  `isBinaryByExtension(path, opts.BinaryExtensions)`. The S2 union is
  `binSet[path] || isBinaryByExtension(path, opts.BinaryExtensions)`.
- ❌ **Don't key binary detection off `detectBinaryFiles`' map for renames.** It keys renames as the
  numstat `old => new` string; `fileStatuses` keys the clean destination `new`. Iterate over `fileStatuses`
  (destinations) and look the binary set up BY destination; the extension signal catches binary renames.
- ❌ **Don't edit `binary.go` / `binary_test.go`.** S1 owns them (already implemented). S2 CONSUMES the
  package-level symbols only. Editing them = scope violation + a conflict with S1.
- ❌ **Don't change the `Git` interface.** `StagedDiff`'s signature is unchanged; `BinaryExtensions` is a
  struct FIELD. Adding a method to the interface is out of scope and leaks an implementation detail.
- ❌ **Don't count binary placeholders against `max_diff_bytes`.** The byte cap covers the non-markdown
  diff body (`nmDiff`) only. Placeholders are tiny metadata lines that REPLACE binary bodies; write them
  to the builder before the capped `nmDiff` and leave the cap untouched.
- ❌ **Don't over-engineer rename handling.** Binary renames are rare (FR3b examples are A and M). The
  destination-keyed lookup + extension signal cover the `.png`-rename case. Don't add rename-normalization
  or `=> `-key reconciliation logic.
- ❌ **Don't escape the `:!<path>` pathspec.** `:!` content is glob-interpreted; a literal path without
  glob metacharacters (the common case: `assets/logo.png`) matches exactly. Escaping is unnecessary and
  risks breaking the common case. Binary asset paths almost never contain `*`/`?`/`[`.
- ❌ **Don't move the binary section before Part 1.** Placement is AFTER markdown, BEFORE non-markdown
  (work item + findings §6). Putting it first would change which git call fails first in the
  GitBinaryMissing/ContextCancelled tests (they'd still pass via Contains, but the markdown section must
  remain first to preserve the existing payload ordering: markdown → binary placeholders → non-markdown).
- ❌ **Don't add dependencies or touch go.mod/go.sum.** Only stdlib `sort` is added. Any new dep is out of
  scope. Editing S1's files, the config layer, or the interface is out of scope.
