---
name: "P2.M1.T1.S1 — Binary detection (numstat + denylist) + placeholder line in internal/git/binary.go (PRD §9.1 FR3a/b)"
description: |

  Create `internal/git/binary.go` (package git) implementing the FR3a two-signal binary-detection
  primitives + the FR3b one-line placeholder generator. These are PURE functions + `*gitRunner` methods
  that are CONSUMED (not yet wired) by S2 (StagedDiff integration), P2.M2.T2.S2 (WorkingTreeDiff), and
  P2.M2.T1.S2 (TreeDiff). This subtask does NOT modify StagedDiff — that is S2's exclusive scope.

  CONTRACT (P2.M1.T1.S1, verbatim from the work item):
    1. RESEARCH: FR3a primary signal = `git diff --cached --numstat` emitting `-\t-\t<path>` for binary
       files. Supplemented by extension denylist (png jpg jpeg gif webp bmp ico svgz pdf zip tar gz tgz
       bz2 7z rar exe dll so dylib o class jar war wasm a mp3 mp4 mov avi mkv flac ogg wav ttf otf woff
       woff2). FR3b placeholder = `<status>\t[binary] <path>` where status is from
       `git diff --cached --name-status`. Existing StagedDiff currently emits the useless
       'Binary files a/… and b/… differ' hunks for binary files.
    2. INPUT: the existing `gitRunner.run()` helper and `StagedDiffOptions` struct in internal/git/git.go.
    3. LOGIC: Create internal/git/binary.go. Implement:
         (a) `defaultBinaryExtensions` — the denylist as a set.
         (b) `isBinaryByExtension(path string, extraExts []string) bool`.
         (c) `(g *gitRunner) detectBinaryFiles(ctx, diffArgs ...string) (map[string]bool, error)` — runs
             `git diff <diffArgs> --numstat`, parses lines where added=="-" && deleted=="-" as binary,
             UNIONS with the extension check.
         (d) `binaryPlaceholderLine(status, path string) string` returning "<status>\t[binary] <path>".
         (e) `(g *gitRunner) fileStatuses(ctx, diffArgs ...string) (map[string]string, error)` running
             `git diff <diffArgs> --name-status` to get A/M/D/R/T per path.
    4. OUTPUT: Pure functions and gitRunner methods for binary detection + placeholder generation.
    5. DOCS: [Mode A] Add a doc comment to binary.go explaining the two-signal detection strategy and the
       placeholder format per FR3a/b/c.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `internal/git/git.go` (StagedDiff, run(), StagedDiffOptions, gitRunner) — READ-ONLY. S2 (next
      subtask) owns the StagedDiff integration. This task only ADDS binary.go + binary_test.go.
    - The `Git` INTERFACE in git.go is NOT extended in S1 (these are unexported `*gitRunner` methods /
      package funcs; they are internal plumbing consumed by other git.go methods). Do NOT add methods to
      the exported `Git` interface.
    - P2.M2 (RevParseTree/TreeDiff/ReadTree/StatusPorcelain/WorkingTreeDiff) — NOT this task.

  DELIVERABLES (2 NEW files, 0 edits to existing source):
    CREATE internal/git/binary.go       — doc comment (FR3a/b/c) + defaultBinaryExtensions (set) +
                                          isBinaryByExtension + binaryPlaceholderLine +
                                          (*gitRunner).detectBinaryFiles + (*gitRunner).fileStatuses.
    CREATE internal/git/binary_test.go  — table tests for the pure funcs + temp-repo tests for the two
                                          gitRunner methods (binary-by-numstat, binary-by-extension,
                                          text-keeps-through, rename, empty, error paths).

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/` empty; go.mod/go.sum UNCHANGED; EXACTLY 2 new files, ZERO edits to existing files.

---

## Goal

**Feature Goal**: Implement PRD §9.1 FR3a/b as a set of reusable, well-tested primitives in
`internal/git/binary.go` that (a) detect binary/non-text files via git numstat UNIONED with an extension
denylist, (b) fetch the per-path change status (A/M/D/R/T) via name-status, and (c) format the FR3b
one-line placeholder that replaces the useless `Binary files … differ` hunk. These primitives are the
shared foundation for binary filtering across ALL THREE diff paths (FR3c: staged / working-tree /
tree-to-tree), but this subtask only CREATES them — it does NOT wire them into StagedDiff (that is S2).

**Deliverable** (2 NEW files, 0 edits):
1. `internal/git/binary.go` — package `git`. Contains: a Mode-A package/file doc comment explaining the
   two-signal detection strategy + the placeholder format (FR3a/b/c); `defaultBinaryExtensions`
   (the 36-entry denylist as a `map[string]bool` set); `isBinaryByExtension(path, extraExts) bool`;
   `binaryPlaceholderLine(status, path) string`; `(*gitRunner).detectBinaryFiles(ctx, diffArgs...)`;
   `(*gitRunner).fileStatuses(ctx, diffArgs...)`.
2. `internal/git/binary_test.go` — package `git` (internal test, like stagediff_test.go). Table tests for
   the two pure functions + temp-repo tests (reusing `initRepo`/`writeFile`/`stageFile`) for the two
   gitRunner methods, covering: real-binary numstat (`-`/`-`), extension-only binary (text content +
   `.png` ext — isolates the denylist signal from numstat), text passthrough, rename, empty repo, git
   binary missing, context cancelled, and exit-code≠0 error wrapping.

**Success Definition**:
- `detectBinaryFiles(ctx, "--cached")` on a repo with a staged real-binary PNG returns
  `map[string]bool{"logo.png": true}` and NO error; a staged text `.go` file is ABSENT from the map.
- A staged text-content file named `fake.png` (git treats as text ⇒ numstat `1/0`) IS detected via the
  extension denylist (proving the union: numstat misses it, the denylist catches it).
- `fileStatuses(ctx, "--cached")` returns `{"logo.png": "A", "notes.txt": "A"}` (and `"R100"` for a
  rename destination).
- `binaryPlaceholderLine("M", "assets/logo.png")` == `"M\t[binary] assets/logo.png"` (exact).
- `isBinaryByExtension("foo.PNG", nil)` == true (case-insensitive); `isBinaryByExtension("a.md", nil)`
  == false; `isBinaryByExtension("a.dat", []string{"dat"})` == true (extra ext, dot-tolerant).
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l internal/`
  empty; go.mod/go.sum UNCHANGED; `git status` shows EXACTLY 2 new files, ZERO edits to existing files.

## User Persona

**Target User**: the IMPLEMENTING AGENT for S2 (StagedDiff integration), WorkingTreeDiff
(P2.M2.T2.S2), and TreeDiff (P2.M2.T1.S2). These are developer-facing internal primitives, not end-user
facing. The "user" is the next subtask that consumes them.

**Use Case**: S2's StagedDiff will call `detectBinaryFiles(ctx, "--cached")` to learn which staged paths
are binary, `fileStatuses(ctx, "--cached")` to get each path's status, then for each binary path emit
`binaryPlaceholderLine(status, path)` and add the path to the pathspec excludes so its body is dropped.
The same three calls serve WorkingTreeDiff (`diffArgs = []`) and TreeDiff (`diffArgs = [treeA, treeB]`).

**Pain Points Addressed**: today a staged binary file produces a useless `Binary files a/… and b/… differ`
hunk that wastes the agent's prompt budget and conveys no useful information. The FR3b placeholder
instead preserves the filename + change type in one line — enough for decomposition grouping (an added
asset belongs with the feature that uses it; PRD §9.1 FR3b).

## Why

- **Closes PRD §9.1 FR3a + FR3b (P0)** at the primitive layer. Binary filtering is required in EVERY diff
  path (FR3c); centralizing detection + placeholder generation in ONE file avoids three copies drifting.
- **Two-signal robustness.** git's numstat content-sniffing is authoritative WHERE it fires, but it is
  content-based (a text-content `.png` is NOT flagged — verified, findings §1). The extension denylist
  covers files git may misclassify. The UNION is more robust than either alone — that is the explicit
  FR3a design.
- **Decoupled, testable foundation.** Pure functions (`isBinaryByExtension`, `binaryPlaceholderLine`)
  are table-testable with no git/repo; the gitRunner methods are thin `run()` wrappers with deterministic
  parsing. This makes S2 a small, low-risk integration rather than a detection+integration tangle.
- **Foundation for P2.M2.** TreeDiff/WorkingTreeDiff (FR3c application points 2 & 3) consume the SAME
  primitives — implementing them once here means P2.M2 is wiring, not re-derivation.

## What

A single new file `internal/git/binary.go` (package `git`) plus its test. No interface changes, no
StagedDiff edits, no config wiring (the `binary_extensions` override is wired by S2/config — S1's
`isBinaryByExtension` already accepts `extraExts` to make that trivial).

### Success Criteria

- [ ] `internal/git/binary.go` exists with a Mode-A doc comment explaining FR3a (two-signal: numstat
      primary + extension denylist supplemental, unioned) and FR3b (placeholder `<status>\t[binary] <path>`),
      and noting FR3c (applies in all diff paths via the variadic `diffArgs`).
- [ ] `defaultBinaryExtensions` is a `map[string]bool` containing EXACTLY the 36 FR3a extensions (no dot,
      lowercase): png jpg jpeg gif webp bmp ico svgz pdf zip tar gz tgz bz2 7z rar exe dll so dylib o
      class jar war wasm a mp3 mp4 mov avi mkv flac ogg wav ttf otf woff woff2.
- [ ] `isBinaryByExtension(path string, extraExts []string) bool`: extracts `filepath.Ext`, lowercases,
      strips the leading dot; returns true if in `defaultBinaryExtensions` OR matches a normalized
      `extraExts` entry (dot-tolerant + case-insensitive); false for empty extension.
- [ ] `binaryPlaceholderLine(status, path string) string` returns EXACTLY `status + "\t[binary] " + path`.
- [ ] `(*gitRunner).detectBinaryFiles(ctx, diffArgs...)` runs `git diff <diffArgs> --numstat`, parses each
      non-empty line via `SplitN("\t", 3)`, flags a path binary iff (`added=="-" && deleted=="-"`) OR
      `isBinaryByExtension(path, nil)`, returns `map[string]bool`. Propagates infrastructural `err`;
      wraps `code != 0` with exit code + trimmed stderr.
- [ ] `(*gitRunner).fileStatuses(ctx, diffArgs...)` runs `git diff <diffArgs> --name-status`, parses each
      non-empty line (`SplitN("\t", 3)`), maps the DESTINATION path (`fields[len-1]`) → status
      (`fields[0]`, e.g. "A"/"M"/"D"/"T"/"R100"). Same err/code handling as detectBinaryFiles.
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/` empty; go.mod/go.sum UNCHANGED; EXACTLY 2 new files, ZERO edits.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact wire formats
(numstat `-/-` for binary, name-status 2/3-field lines — findings §1/§2, empirically verified); the
exact `run()` contract (findings §6: non-zero exit ⇒ `(stdout,stderr,code,nil)`, err only for
infrastructural); the exact signatures + algorithms (the contract, restated precisely in What/Blueprint);
the reusable test helpers (`initRepo`/`writeFile`/`stageFile` in committree_test.go, same package —
findings §8); the key test-separation trick (text-content `.png` isolates the denylist signal from
numstat — findings §1/§8); and the rename `=>` gotcha + S2-coordination note (findings §4). No
prompt/decompose/config knowledge required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (wire formats verified on git 2.54.0)
- docfile: plan/002_a17bb6c8dc1d/P2M1T1S1/research/findings.md
  why: §1 numstat `-/-` binary format + the content-sniffing proof (why the denylist is needed);
       §2 name-status 2/3-field format; §4 the rename `=>` numstat-vs-name-status mismatch (S2 coord);
       §5 exit-code semantics (simple-branch form, NOT --quiet's exit-1 inversion); §6 run() contract;
       §7 why diffArgs is variadic (3 FR3c consumers); §8 test fixtures + the text-content-.png trick.
  critical: §1 (content-sniff ⇒ two-signal union is mandatory, not optional); §6 (run() invariant — err
       for infrastructural only; code!=0 wrapped); §8 (how to test each signal in isolation).

# MUST READ — the INPUT contract: run() + gitRunner + StagedDiffOptions (READ-ONLY, do NOT edit)
- file: internal/git/git.go   (READ-ONLY)
  section: `func (g *gitRunner) run(ctx, repo, args...) (stdout, stderr, exitCode, err)` (the ONLY exec
       helper — see its INVARIANT comment: non-zero git exit ⇒ err==nil, code carries the value); the
       `gitRunner` struct (`workDir string` — pass as `g.workDir` to run()); `New(workDir) Git`.
  why: detectBinaryFiles/fileStatuses are `*gitRunner` methods that call `g.run(ctx, g.workDir, args...)`
       exactly like StagedDiff/DiffTree do. Mirror their err-handling shape: `if err != nil {return err}`
       THEN `if code != 0 {return fmt.Errorf("... (exit %d): %s", code, strings.TrimSpace(stderr))}`.
  pattern: copy StagedDiff's/StagedFileCount's arg-building idiom: `args := make([]string,0,N);
           append "diff"; append diffArgs...; append "--numstat"`.
  gotcha: do NOT touch this file. do NOT add methods to the exported `Git` interface. do NOT modify
          StagedDiff (S2 owns that). do NOT use runWithInput (stdin not needed).

# MUST READ — the design reference (signatures + StagedDiff integration sketch)
- docfile: plan/002_a17bb6c8dc1d/architecture/binary_git_v2.md   (READ-ONLY)
  section: "1. Binary/Non-Text Filtering (§9.1 FR3a-c)" — the detection strategy, placeholder format,
           and the binary.go implementation sketch (NOTE: the doc shows defaultBinaryExtensions as a
           []slice and detectBinaryPaths; the WORK-ITEM CONTRACT overrides — use a map SET named
           defaultBinaryExtensions and detectBinaryFiles. Follow the CONTRACT, not the doc's sketch.)
  critical: confirms the two-signal union, the 36-entry list, the placeholder format, and the variadic
           diffArgs design serving all 3 FR3c points.

# MUST READ — test fixtures + an exemplar internal test (same package)
- file: internal/git/committree_test.go   (READ — fixture definitions)
  section: `writeFile(t, dir, name, body string)`, `stageFile(t, dir, name string)`, `initRepo(t, dir)`,
           `setIdentityConfig(t, dir)`, `writeTreeOf(t, dir) string` — all available to binary_test.go
           (same package git). Reuse them; do NOT redefine.
  pattern: see how stagediff_test.go composes initRepo(t,tmp)+writeFile+stageFile+New(tmp) per test.
- file: internal/git/stagediff_test.go   (READ — the closest test-pattern exemplar)
  section: the `New(repo)` + `context.Background()` + `g.StagedDiff(...)` + `strings.Contains`/`Count`
           assertion idiom; `TestStagedDiff_GitBinaryMissing` (PATH="" ⇒ LookPath fails) and
           `_ContextCancelled` (pre-cancelled ctx) — COPY these two error-path test shapes verbatim for
           detectBinaryFiles/fileStatuses.

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §9.1 FR3a / FR3b / FR3c   (internal to this repo)
  why: FR3a (numstat primary + 36-ext denylist, overridable via binary_extensions); FR3b (placeholder
       `<status>\t[binary] <path>`, status from name-status, A/M/D/R/T); FR3c (applies in EVERY diff path:
       staged §9.1, working-tree §13.6.2, tree-to-tree §13.6.3 — identical format).

# MUST READ — the parallel sibling PRP (coordination; S1 is independent of its outputs)
- docfile: plan/002_a17bb6c8dc1d/P1M4T4S1/PRP.md   (PARALLEL — first-run config bootstrap)
  why: it touches internal/config ONLY. S1 touches internal/git ONLY. NON-OVERLAPPING — no coordination
       needed beyond confirming no shared file. (Listed for completeness per parallel-context protocol.)
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go              # READ-ONLY: run(), runWithInput(), gitRunner{workDir}, New(), StagedDiff,
                      #   StagedDiffOptions, DiffTree/parseDiffTree, FileChange, defaultExcludes, etc.
  binary.go           # CREATE — the 5 symbols below + the FR3a/b/c doc comment.
  binary_test.go      # CREATE — table tests + temp-repo tests (reuses initRepo/writeFile/stageFile).
  committree_test.go  # READ — defines initRepo/writeFile/stageFile/writeTreeOf/setIdentityConfig.
  stagediff_test.go   # READ — closest test-pattern exemplar + the GitBinaryMissing/ContextCancelled shapes.
  (other *_test.go)   # READ — revparse/writetree/difftree/etc. exemplars (all same package git).
go.mod / go.sum       # UNCHANGED (no new deps: context/fmt/path/filepath/strings are stdlib).
.golangci.yml         # READ — enabled linters; binary_test.go is NOT in the exclude list ⇒ errcheck-clean.
```

### Desired Codebase tree with files to be added

```bash
internal/git/binary.go        # CREATE. Symbols:
                              #   var defaultBinaryExtensions map[string]bool         (the 36-ext set)
                              #   func isBinaryByExtension(path string, extraExts []string) bool
                              #   func binaryPlaceholderLine(status, path string) string
                              #   func (g *gitRunner) detectBinaryFiles(ctx, diffArgs ...string) (map[string]bool, error)
                              #   func (g *gitRunner) fileStatuses(ctx, diffArgs ...string) (map[string]string, error)
                              # + a Mode-A doc comment (package-level comment block atop the file) on FR3a/b/c.
internal/git/binary_test.go   # CREATE. Tests (package git):
                              #   TestIsBinaryByExtension_* (table: default hit, case-insensitive, miss,
                              #     empty ext, extra-ext with/without dot, extra-ext case-insensitive)
                              #   TestBinaryPlaceholderLine_* (table: A/M/D, status with score "R100", path w/ spaces)
                              #   TestDetectBinaryFiles_* (RealBinaryNumstat, ExtensionOnlyTextPng,
                              #     TextFileNotBinary, UnionBothSignals, EmptyRepo, Rename, GitBinaryMissing,
                              #     ContextCancelled)
                              #   TestFileStatuses_* (AddedModified, RenameDestination, EmptyRepo,
                              #     GitBinaryMissing, ContextCancelled)
# go.mod/go.sum UNCHANGED. git.go UNCHANGED. The Git interface UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (git CONTENT-SNIFFS — findings §1): numstat's `-/-` is decided by CONTENT, not extension. A
// file named fake.png with TEXT content emits `1\t0\tfake.png` (text!), NOT `-/-`. So the numstat signal
// ALONE would miss a text-content .png, and the extension signal ALONE would miss a binary that lacks a
// denylisted extension but git sniffs as binary. FR3a mandates the UNION — implement it as
// `(added=="-" && deleted=="-") || isBinaryByExtension(path, nil)`.

// CRITICAL (run() INVARIANT — findings §6): a NON-ZERO git exit is returned as (stdout, stderr, code, nil)
// — err is nil. Only infrastructural failures (LookPath miss, ctx cancel, start/I-O) return err != nil with
// code = -1. So: `if err != nil { return nil, err }` FIRST (propagate unwrapped), THEN `if code != 0 {
// return nil, fmt.Errorf("git diff --numstat: failed (exit %d): %s", code, strings.TrimSpace(stderr)) }`.
// Mirror StagedDiff/StagedFileCount EXACTLY — NOT HasStagedChanges' switch form (that exit-1 inversion is
// exclusive to --quiet; numstat/name-status have NO such inversion — findings §5).

// GOTCHA (NO --quiet, NO exit-1 inversion): detectBinaryFiles/fileStatuses use --numstat/--name-status
// WITHOUT --quiet. Without --quiet, `git diff` exits 0 whether or not changes exist, and emits the paths.
// (Adding --quiet would suppress stdout and silently break everything — same trap StagedFileCount's
// comment warns about. Do NOT add --quiet.)

// GOTCHA (rename `=>` in numstat vs 3-field in name-status — findings §4): git 2.54 has diff.renames ON
// by default, so a rename shows in numstat as `0\t0\told.png => new.png` (path contains ` => `, counts
// 0/0 — NOT -/-). detectBinaryFiles faithfully keys the map by that `=>` string; the extension check still
// fires because filepath.Ext("old.png => new.png")==".png". fileStatuses uses name-status, which separates
// renames into clean 3 fields (R100\tsrc\tdst) keyed by the DESTINATION. The numstat `=>` key ≠ the
// name-status destination key — S2 (StagedDiff) reconciles these. This is a DOCUMENTED coordination point
// for S2, NOT a bug in S1. Do NOT try to normalize renames in S1 (over-engineering; binary renames are
// rare; FR3b examples are A and M).

// GOTCHA (preserve paths — do NOT TrimSpace the whole numstat/name-status line): a path may contain
// leading/trailing spaces (rare but legal). Split on "\n", then SplitN(line, "\t", 3); skip len<2 (name-
// status) / len<3 (numstat) lines. Do NOT strings.TrimSpace the line (it would corrupt space-bearing
// paths); rely on the field-count guard to skip the empty trailing element. (filepath.Ext + the status
// field are unaffected by internal spaces.) Verified: a file "file with spaces.txt" → numstat emits it on
// one tab-delimited line intact (findings §1).

// GOTCHA (SplitN with limit 3 protects the path): use strings.SplitN(line, "\t", 3) so a path containing
// a literal tab (vanishingly rare) stays whole in fields[2]/fields[len-1]. Plain Split would over-split.

// GOTCHA (filepath.Ext edge case): filepath.Ext("archive.tar.gz") == ".gz" (only the LAST dot's suffix),
// and filepath.Ext("noext") == "". isBinaryByExtension lowercases + strips the leading dot, so "tar.gz"→
// ext "gz" (in the list ✓) and "gz" alone → ext "gz" too. A path ending in "." → ext "." → stripped to ""
// → not matched. All acceptable per FR3a (the denylist is by terminal extension).

// GOTCHA (extraExts normalization): config `binary_extensions` entries may arrive WITH or WITHOUT a
// leading dot and in any case. Normalize each: strings.ToLower + TrimSpace + TrimPrefix(e,"."). Compare
// against the normalized path extension. This makes S1's isBinaryByExtension trivially wireable to the
// config override without S1 knowing about config.

// GOTCHA (do NOT extend the exported Git interface): detectBinaryFiles/fileStatuses are UNEXPORTED
// *gitRunner methods (lowercase) consumed by other internal/git methods (StagedDiff in S2). They are NOT
// part of the public Git contract. Adding them to the interface would leak internal plumbing into pkg/
// stagecoach — do NOT. (StagedDiff is itself on the interface; its binary filtering is an internal detail.)

// GOTCHA (same package, no import cycle): binary.go is package git; it references gitRunner.run + the
// same stdlib imports git.go uses. NO new package imports. NO go.mod change.

// GOTCHA (errcheck on binary_test.go): .golangci.yml excludes ONLY stagediff_test.go from errcheck —
// binary_test.go is NOT excluded. Keep errcheck-clean there (handle/assign every returned error/Close).
```

## Implementation Blueprint

### Data models and structure

No new exported TYPES (the architecture doc's `NumStatEntry` struct is NOT required by the contract and
would be unused in S1 — omit it; detectBinaryFiles returns `map[string]bool`, fileStatuses returns
`map[string]string`). The five symbols are all the contract mandates.

```go
// internal/git/binary.go — NEW FILE (package git). Imports: context, fmt, path/filepath, strings.

// defaultBinaryExtensions is the FR3a built-in binary extension denylist, as a set for O(1) lookup.
// Entries are lowercase, WITHOUT a leading dot. Git's numstat content-sniffing is the PRIMARY binary
// signal; this list SUPPLEMENTS it for files git may misclassify. Overridable/extendable via the
// `binary_extensions` config (wired by StagedDiff/S2 + config); isBinaryByExtension accepts extraExts.
var defaultBinaryExtensions = map[string]bool{
	// images
	"png": true, "jpg": true, "jpeg": true, "gif": true, "webp": true, "bmp": true, "ico": true, "svgz": true,
	// documents / archives
	"pdf": true, "zip": true, "tar": true, "gz": true, "tgz": true, "bz2": true, "7z": true, "rar": true,
	// compiled / native
	"exe": true, "dll": true, "so": true, "dylib": true, "o": true, "class": true, "jar": true, "war": true, "wasm": true, "a": true,
	// media
	"mp3": true, "mp4": true, "mov": true, "avi": true, "mkv": true, "flac": true, "ogg": true, "wav": true,
	// fonts
	"ttf": true, "otf": true, "woff": true, "woff2": true,
}

// isBinaryByExtension reports whether path's terminal extension is in defaultBinaryExtensions or in the
// caller-supplied extraExts (the `binary_extensions` config override; entries dot-tolerant + case-
// insensitive). Extension lookup is case-insensitive and ignores a missing/empty extension. Pure: no I/O.
func isBinaryByExtension(path string, extraExts []string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if ext == "" {
		return false
	}
	if defaultBinaryExtensions[ext] {
		return true
	}
	for _, e := range extraExts {
		if strings.ToLower(strings.TrimSpace(strings.TrimPrefix(e, "."))) == ext {
			return true
		}
	}
	return false
}

// binaryPlaceholderLine returns the FR3b one-line placeholder for a binary file: "<status>\t[binary] <path>".
// status is the raw git name-status code (A/M/D/T, or "R100" for a rename). Pure: no I/O.
func binaryPlaceholderLine(status, path string) string {
	return status + "\t[binary] " + path
}

// detectBinaryFiles returns the set of binary paths among the files matched by `git diff <diffArgs>`.
// Detection is the FR3a two-signal UNION: (a) git numstat emits "-\t-\t<path>" for files it content-sniffs
// as binary (PRIMARY), (b) the extension denylist (isBinaryByExtension) catches files git may misclassify
// (SUPPLEMENTAL). diffArgs selects the diff domain and is forwarded verbatim: ["--cached"] (staged, S2),
// [] (working tree, P2.M2.T2.S2), or [treeA, treeB] (tree-to-tree, P2.M2.T1.S2) — serving all FR3c paths.
// Read-only w.r.t. refs/index. (PRD §9.1 FR3a.)
func (g *gitRunner) detectBinaryFiles(ctx context.Context, diffArgs ...string) (map[string]bool, error) {
	args := make([]string, 0, 1+len(diffArgs)+1)
	args = append(args, "diff")
	args = append(args, diffArgs...)
	args = append(args, "--numstat")
	stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return nil, err // infrastructural (git missing / ctx cancel / start failure) — propagate unwrapped
	}
	if code != 0 {
		return nil, fmt.Errorf("git diff --numstat: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	binary := make(map[string]bool)
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue // empty line (trailing "\n") or malformed — skip (do NOT TrimSpace the line; preserve paths)
		}
		added, deleted, path := fields[0], fields[1], fields[2]
		if (added == "-" && deleted == "-") || isBinaryByExtension(path, nil) {
			binary[path] = true // FR3a union: numstat content-sniff OR extension denylist
		}
	}
	return binary, nil
}

// fileStatuses returns a map of path → git change-status (A/M/D/T, or "R100"/"C80" for rename/copy) for
// the files matched by `git diff <diffArgs> --name-status`. The status is the source of the <status> in
// the FR3b placeholder (PRD §9.1 FR3b). For a rename/copy the map is keyed by the DESTINATION path
// (fields[len-1]); the source is dropped (the placeholder carries the new name). diffArgs is forwarded
// verbatim, same contract as detectBinaryFiles. Read-only w.r.t. refs/index.
func (g *gitRunner) fileStatuses(ctx context.Context, diffArgs ...string) (map[string]string, error) {
	args := make([]string, 0, 1+len(diffArgs)+1)
	args = append(args, "diff")
	args = append(args, diffArgs...)
	args = append(args, "--name-status")
	stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("git diff --name-status: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	statuses := make(map[string]string)
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 2 {
			continue
		}
		statuses[fields[len(fields)-1]] = fields[0] // destination path → status (R100 line: fields[2]→"R100")
	}
	return statuses, nil
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/git/binary.go (the 5 symbols + the Mode-A doc comment)
  - WRITE the file: `package git` + imports (context, fmt, path/filepath, strings); a Mode-A doc comment
    block atop the file explaining FR3a (two-signal: numstat primary + extension denylist supplemental,
    UNIONED — and WHY: git content-sniffs, so a text-content .png is not flagged by numstat; the denylist
    covers git's misses), FR3b (placeholder "<status>\t[binary] <path>", status from name-status), and FR3c
    (applies in every diff path via the variadic diffArgs: staged/working-tree/tree-to-tree).
  - IMPLEMENT the 5 symbols EXACTLY as in the Data Models section above.
  - NAMING: defaultBinaryExtensions (map set), isBinaryByExtension, binaryPlaceholderLine,
    detectBinaryFiles, fileStatuses (lowercase = unexported *gitRunner methods / package funcs).
  - GOTCHA: detectBinaryFiles/fileStatuses are *gitRunner methods (lowercase) — do NOT add to the Git
    interface. Mirror StagedDiff/StagedFileCount's err+code handling shape verbatim.
  - GOTCHA: SplitN(line,"\t",3); do NOT TrimSpace the line. Field-count guard skips empties.
  - PLACEMENT: internal/git/binary.go (one file, all 5 symbols).

Task 2: CREATE internal/git/binary_test.go (package git; reuse existing fixtures)
  - WRITE `package git` + imports (context, errors, os, path/filepath, strings, testing — match stagediff_test.go).
  - ADD TestIsBinaryByExtension (table-driven): ("logo.PNG",nil)→true [case-insensitive]; ("a.jpg",nil)→true;
    ("archive.tar.gz",nil)→true [terminal ext]; ("a.md",nil)→false; ("noext",nil)→false; ("a.",nil)→false;
    ("a.dat",nil)→false [not in default]; ("a.dat",[]string{"dat"})→true [extra, no dot];
    ("a.DAT",[]string{".dat"})→true [extra, with dot, case-insensitive]; ("a.bin",[]string{" bin "})→true
    [extra, whitespace-tolerant].
  - ADD TestBinaryPlaceholderLine (table): ("M","assets/logo.png")→"M\t[binary] assets/logo.png";
    ("A","public/trailer.mp4")→"A\t[binary] public/trailer.mp4"; ("D","old.bin")→"D\t[binary] old.bin";
    ("R100","new.png")→"R100\t[binary] new.png" [rename status verbatim].
  - ADD TestDetectBinaryFiles_RealBinaryNumstat: initRepo+writeFile a REAL binary (bytes with a NUL or PNG
    header "\x89PNG\r\n\x1a\n") as logo.png + a text notes.txt; stage both; detectBinaryFiles(ctx,"--cached")
    ⇒ {"logo.png":true} and NOT contain "notes.txt". (Proves the numstat -/- primary signal.)
  - ADD TestDetectBinaryFiles_ExtensionOnlyTextPng: writeFile TEXT content ("hello\n") to fake.png + stage;
    detectBinaryFiles ⇒ {"fake.png":true}. (Proves the denylist SUPPLEMENTAL signal — git treats it as
    text ⇒ numstat emits 1/0, ONLY the extension check catches it. This is the key two-signal test.)
  - ADD TestDetectBinaryFiles_TextFileNotBinary: a.go staged ⇒ detectBinaryFiles ⇒ empty map.
  - ADD TestDetectBinaryFiles_EmptyRepo: nothing staged ⇒ detectBinaryFiles ⇒ empty map, no error.
  - ADD TestDetectBinaryFiles_Rename (documentation test): commit a binary, `git mv` it, run
    detectBinaryFiles(ctx) on the working tree (no --cached) ⇒ the map contains a key with " => " (the
    numstat rename form) and is non-empty (extension check fires). ASSERT the key CONTAINS "=>" and the map
    is non-empty — do NOT assert an exact key (rename-score/format varies). (Pins findings §4 for S2.)
  - ADD TestDetectBinaryFiles_GitBinaryMissing (COPY stagediff_test.go shape): t.Setenv("PATH",""); New(tmp);
    detectBinaryFiles ⇒ err contains "git binary not found"; nil map.
  - ADD TestDetectBinaryFiles_ContextCancelled (COPY shape): pre-cancel ctx; detectBinaryFiles ⇒
    errors.Is(err, context.Canceled); nil map.
  - ADD TestFileStatuses_AddedModified: stage logo.png(real binary)+notes.txt ⇒ {"logo.png":"A","notes.txt":"A"}.
  - ADD TestFileStatuses_RenameDestination: commit old.txt, `git mv old.txt new.txt`, fileStatuses(ctx) ⇒
    {"new.txt":"R100"} (or a key starting "R" — assert HasPrefix "R" on the value). (Proves destination-keying.)
  - ADD TestFileStatuses_EmptyRepo ⇒ empty map, no error.
  - ADD TestFileStatuses_GitBinaryMissing + TestFileStatuses_ContextCancelled (COPY shapes).
  - PATTERN: reuse initRepo/writeFile/stageFile (committree_test.go). For renames use exec.Command("git","-C",repo,"mv",...).
    or commit + mv helpers; keep errcheck-clean (binary_test.go is NOT in the lint exclude list).
  - COVERAGE: every public symbol; both signals of detectBinaryFiles in isolation; rename; empty; both
    error paths (git-missing, ctx-cancel) for BOTH gitRunner methods.
  - PLACEMENT: internal/git/binary_test.go.

Task 3: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/git/binary.go internal/git/binary_test.go`
  - `go build ./... && go vet ./internal/git/`
  - `go test -race ./internal/git/ -run "TestIsBinaryByExtension|TestBinaryPlaceholderLine|TestDetectBinaryFiles|TestFileStatuses" -v`
  - `golangci-lint run ./internal/git/`   (errcheck/gosimple/govet/ineffassign/staticcheck/unused)
  - `go test ./...`   (full regression — no existing test should change; ZERO edits to existing files)
  - `git diff --exit-code go.mod go.sum` ⇒ empty.
  - `git status` ⇒ EXACTLY 2 new files (binary.go, binary_test.go), ZERO modified files.
```

### Implementation Patterns & Key Details

```go
// PATTERN (run() err+code shape — mirror StagedDiff/StagedFileCount): infrastructural err propagates
// UNWRAPPED first; then code != 0 is wrapped with the exit code + trimmed stderr. NEVER the HasStagedChanges
// switch form (no --quiet ⇒ no exit-1 inversion).
stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
if err != nil {
	return nil, err
}
if code != 0 {
	return nil, fmt.Errorf("git diff --numstat: failed (exit %d): %s", code, strings.TrimSpace(stderr))
}

// PATTERN (variadic diffArgs → arg slice — mirror StagedDiff's nmArgs building):
args := make([]string, 0, 1+len(diffArgs)+1)
args = append(args, "diff")
args = append(args, diffArgs...)
args = append(args, "--numstat") // flags last; diffArgs (e.g. "--cached" or two tree SHAs) sit between

// PATTERN (tab parse with field-count guard, NO full-line TrimSpace — preserve space-bearing paths):
for _, line := range strings.Split(stdout, "\n") {
	fields := strings.SplitN(line, "\t", 3)
	if len(fields) < 3 { // numstat needs 3; name-status needs 2 — adjust the threshold per caller
		continue
	}
	// ... fields[0]=added/status, fields[1]=deleted/src, fields[2]=path/dst ...
}

// CRITICAL (FR3a UNION — the whole point of two signals): a path is binary iff numstat says so OR the
// extension matches. git content-sniffs (findings §1), so neither signal alone suffices.
if (added == "-" && deleted == "-") || isBinaryByExtension(path, nil) {
	binary[path] = true
}

// CRITICAL (do NOT add to the Git interface): these are lowercase *gitRunner methods — internal plumbing
// consumed by StagedDiff (S2). The public Git surface is unchanged.
```

### Integration Points

```yaml
INTERNAL/GIT (internal/git/binary.go — NEW; consumed by sibling methods, NOT yet wired):
  - add: "defaultBinaryExtensions (map set, 36 FR3a exts); isBinaryByExtension(path, extraExts);
          binaryPlaceholderLine(status, path); (*gitRunner).detectBinaryFiles(ctx, diffArgs...);
          (*gitRunner).fileStatuses(ctx, diffArgs...)."
  - consumed-by (NOT this task): S2 StagedDiff (diffArgs=["--cached"]); P2.M2.T2.S2 WorkingTreeDiff
          (diffArgs=[]); P2.M2.T1.S2 TreeDiff (diffArgs=[treeA,treeB]).

S2 COORDINATION (StagedDiff integration — NEXT subtask, NOT this one):
  - handoff: "S2 calls detectBinaryFiles(ctx,'--cached') + fileStatuses(ctx,'--cached'); for each binary
          path emits binaryPlaceholderLine(status,path) and adds ':!'+path to the non-markdown excludes so
          the body is dropped. NOTE the rename key mismatch (numstat 'old => new' vs name-status 'new') —
          S2 reconciles by keying off name-status and looking up the binary set by destination (findings §4)."

GIT INTERFACE (internal/git/git.go — UNCHANGED):
  - no-change: "detectBinaryFiles/fileStatuses are unexported *gitRunner methods; NOT added to Git. S2's
          StagedDiff (already on the interface) calls them internally."

GO.MODULE: change NONE. stdlib only (context/fmt/path/filepath/strings). No new deps.

FROZEN/LEAVE (do NOT edit):
  - internal/git/git.go (run/gitRunner/StagedDiff/StagedDiffOptions/Git interface — READ-ONLY; S2 owns
    StagedDiff). All other *_test.go files. go.mod/go.sum. internal/config/* (parallel P1.M4.T4.S1).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
gofmt -w internal/git/binary.go internal/git/binary_test.go
go vet ./internal/git/
golangci-lint run ./internal/git/
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; golangci-lint clean (binary_test.go is NOT in the exclude list ⇒ errcheck-clean);
#           go.mod/go.sum unchanged.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Pure functions + both gitRunner methods, verbose:
go test -race ./internal/git/ -run "TestIsBinaryByExtension|TestBinaryPlaceholderLine|TestDetectBinaryFiles|TestFileStatuses" -v
# Expected: all green. Specifically:
#   TestDetectBinaryFiles_RealBinaryNumstat      ⇒ {logo.png} (numstat -/- primary signal)
#   TestDetectBinaryFiles_ExtensionOnlyTextPng   ⇒ {fake.png} (denylist supplemental signal — git said text!)
#   TestDetectBinaryFiles_TextFileNotBinary      ⇒ empty map
#   TestFileStatuses_RenameDestination           ⇒ {new.txt: "R100"} (destination-keyed)
#   TestDetectBinaryFiles_GitBinaryMissing       ⇒ err "git binary not found"
#   TestDetectBinaryFiles_ContextCancelled       ⇒ errors.Is(err, context.Canceled)

# Full internal/git suite (regression — ZERO edits to existing files ⇒ no existing test changes):
go test -race ./internal/git/ -v
```

### Level 3: Integration / Behavioral Proof (the two-signal contract)

```bash
# Reproduce the findings empirically inside a temp repo, calling the primitives via a tiny throwaway test
# or by inspecting git directly. The DEFINITIVE proof is TestDetectBinaryFiles_ExtensionOnlyTextPng:
T=$(mktemp -d); cd "$T"; git init -q .; git config user.email t@t.co; git config user.name t
printf '\x89PNG\r\n\x1a\n' > real.png        # genuine binary
echo "text but binary ext" > fake.png         # text content, denylisted ext
echo "code" > a.go                            # text, no denylist ext
git add real.png fake.png a.go
echo "--- numstat (real binary is -/-, fake.png is 1/0 because CONTENT is text) ---"
git diff --cached --numstat
# Expected:  real.png ⇒ "-\t-\t"; fake.png ⇒ "1\t0\t"; a.go ⇒ "1\t0\t"
echo "--- name-status ---"
git diff --cached --name-status
# Expected:  A\treal.png / A\tfake.png / A\ta.go
# ⇒ detectBinaryFiles MUST return {real.png, fake.png} (real by numstat, fake by extension); a.go excluded.
cd /; rm -rf "$T"

# (The above is exactly what TestDetectBinaryFiles_RealBinaryNumstat + _ExtensionOnlyTextPng assert in Go.)
```

### Level 4: Regression & Audit

```bash
cd /home/dustin/projects/stagecoach
go build ./...                # whole module compiles (binary.go is in package git)
go test ./...                 # FULL regression — no existing test changes (ZERO edits to existing files)
git status --short            # Expected: EXACTLY 2 files:
                              #   ?? internal/git/binary.go
                              #   ?? internal/git/binary_test.go
# Expected: build + full test green; only 2 new untracked files; no modified files; go.mod/go.sum unchanged.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l internal/` empty; `go vet ./internal/git/` clean; `golangci-lint run ./internal/git/` clean.
- [ ] Level 2: `go test -race ./internal/git/` green — incl. both two-signal tests (numstat + extension) in isolation.
- [ ] Level 3: empirical two-signal proof (real.png by numstat, fake.png by extension, a.go excluded).
- [ ] Level 4: `go build ./...` + `go test ./...` green; `git status` shows EXACTLY 2 new files, 0 modified; go.mod/go.sum unchanged.

### Feature Validation

- [ ] `defaultBinaryExtensions` is a `map[string]bool` with EXACTLY the 36 FR3a extensions (lowercase, no dot).
- [ ] `isBinaryByExtension` is case-insensitive, dot-tolerant for extraExts, returns false for empty extension.
- [ ] `binaryPlaceholderLine("M","assets/logo.png")` == `"M\t[binary] assets/logo.png"` (exact).
- [ ] `detectBinaryFiles` returns the FR3a UNION (numstat `-/-` OR extension); faithfully keys renames as
      `old => new`; empty repo ⇒ empty map; exit≠0 wrapped; infrastructural err propagated.
- [ ] `fileStatuses` maps destination path → raw status (A/M/D/T/R100); empty repo ⇒ empty map; same err shape.
- [ ] Mode-A doc comment explains FR3a (two-signal + why), FR3b (placeholder format), FR3c (all diff paths).

### Code Quality Validation

- [ ] detectBinaryFiles/fileStatuses are lowercase `*gitRunner` methods — NOT added to the exported Git interface.
- [ ] err/code handling mirrors StagedDiff/StagedFileCount (simple-branch form); NO --quiet, NO exit-1 inversion.
- [ ] Tab parse uses `SplitN(line,"\t",3)` with a field-count guard; no full-line `TrimSpace` (paths preserved).
- [ ] File placement matches the desired tree (only binary.go + binary_test.go); ZERO edits to existing files.
- [ ] Anti-patterns avoided (see below): no interface leak, no StagedDiff edit, no new deps, no over-engineered renames.

### Documentation & Deployment

- [ ] Mode-A doc comment atop binary.go names FR3a/b/c and the two-signal rationale (content-sniff ⇒ union).
- [ ] Each of the 5 symbols has a doc comment (godoc-compliant; the receiver methods name the PRD section).
- [ ] Implementation summary records: the 5 symbols, the two-signal proof, the rename S2-coordination note.

---

## Anti-Patterns to Avoid

- ❌ **Don't wire the primitives into StagedDiff.** S1 only CREATES binary.go + binary_test.go. StagedDiff
  integration is S2's exclusive scope. Editing git.go (StagedDiff) here = scope violation + a merge conflict
  with S2. Leave StagedDiff emitting the old `Binary files … differ` hunk for now; S2 replaces it.
- ❌ **Don't add detectBinaryFiles/fileStatuses to the exported `Git` interface.** They are internal
  `*gitRunner` plumbing consumed by other internal/git methods. Leaking them into the interface pollutes
  pkg/stagecoach's public surface with an implementation detail.
- ❌ **Don't rely on numstat alone OR the denylist alone.** FR3a is a UNION. git content-sniffs (verified:
  a text-content `.png` emits `1/0`, not `-/-`), so numstat misses it; a binary without a denylisted
  extension but sniffed by git is missed by the denylist. Implement `(added=="-"&&deleted=="-") || isBinaryByExtension(path,nil)`.
- ❌ **Don't use `--quiet` or the HasStagedChanges switch form.** numstat/name-status have NO exit-1
  inversion (that's exclusive to `--quiet`). Use the simple-branch form (`err` first, then `code != 0`).
- ❌ **Don't `strings.TrimSpace` the whole numstat/name-status line.** It corrupts paths with leading/trailing
  spaces. Split on `\n`, `SplitN("\t",3)`, skip by field-count. (The existing methods TrimSpace lines, but
  they don't return paths into a map keyed by exact bytes — here path fidelity matters.)
- ❌ **Don't over-engineer rename handling.** git 2.54 detects renames by default (numstat `old => new`;
  name-status `R100\tsrc\tdst`). detectBinaryFiles faithfully keys the `=>` form; fileStatuses keys the
  destination. Reconciling the two is S2's job. Don't add rename-normalization logic to S1.
- ❌ **Don't add the `NumStatEntry` struct from the architecture sketch.** The contract mandates
  `map[string]bool` (detectBinaryFiles) and `map[string]string` (fileStatuses) — no struct. An unused struct
  is dead code.
- ❌ **Don't add dependencies or touch go.mod/go.sum.** Only stdlib (context/fmt/path/filepath/strings) is used.
- ❌ **Don't edit existing files.** EXACTLY 2 new files. Any edit to git.go / an existing *_test.go / go.mod
  is out of scope (S2 owns git.go; the parallel config task owns internal/config).
