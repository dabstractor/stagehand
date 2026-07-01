# Binary Filtering & Git V2 Plumbing — Detailed Design

## 1. Binary/Non-Text Filtering (§9.1 FR3a-c)

### Detection Strategy (FR3a)
Two signals, union for robustness:

**Primary: git numstat** — `git diff --cached --numstat` emits `-\t-\t<path>` for any file git classifies as binary. Content sniffing catches images, compiled binaries, archives, fonts, media.

**Supplemental: extension denylist** — covers files git may misclassify. Built-in set (FR3a):
```
png jpg jpeg gif webp bmp ico svgz pdf zip tar gz tgz bz2 7z rar
exe dll so dylib o class jar war wasm a mp3 mp4 mov avi mkv flac
ogg wav ttf otf woff woff2
```
Overridable via `binary_extensions` config (merges with built-in, does NOT replace).

### Placeholder Format (FR3b)
For each excluded file, emit ONE LINE instead of the binary diff body:
```
<status>\t[binary] <path>
```
Where `<status>` (A/M/D/R/T) is sourced from `git diff --cached --name-status`.

Examples:
```
M	[binary] assets/logo.png
A	[binary] public/trailer.mp4
```

### Application Points (FR3c)
Binary filtering applies in EVERY diff path:
1. **Staged diff** (FR1-4) — the v1 `StagedDiff` method must be updated
2. **Working-tree snapshot** (§13.6.2) — new `WorkingTreeDiff` for planner input
3. **Per-concept tree-to-tree diff** (§13.6.3) — new `TreeDiff` for message[i] input

The placeholder format is identical in all three.

### Implementation: `internal/git/binary.go`

```go
// NumStatEntry represents one file's numstat line.
type NumStatEntry struct {
    Added, Deleted int  // -1, -1 for binary files
    Path           string
}

// detectBinaryPaths returns the set of paths that git classifies as binary
// (numstat emits "-\t-\t") OR whose extension matches the denylist.
func (g *gitRunner) detectBinaryPaths(ctx context.Context, diffArgs ...string) (map[string]bool, error)

// binaryPlaceholderLine returns the FR3b placeholder for a binary file.
func binaryPlaceholderLine(status, path string) string

// defaultBinaryExtensions is the built-in denylist (FR3a).
var defaultBinaryExtensions = []string{
    "png", "jpg", "jpeg", "gif", "webp", "bmp", "ico", "svgz", "pdf",
    "zip", "tar", "gz", "tgz", "bz2", "7z", "rar",
    "exe", "dll", "so", "dylib", "o", "class", "jar", "war", "wasm", "a",
    "mp3", "mp4", "mov", "avi", "mkv", "flac", "ogg", "wav",
    "ttf", "otf", "woff", "woff2",
}

// isBinaryByExtension checks a path against the extension denylist.
func isBinaryByExtension(path string, extraExts []string) bool
```

### StagedDiff Integration
In `StagedDiff`, before capturing the non-markdown diff:
1. Run `git diff --cached --numstat` to get binary paths
2. Run `git diff --cached --name-status` to get statuses
3. For each binary file: emit placeholder line, add to pathspec excludes
4. Non-binary files: captured normally with existing caps/excludes
5. Binary placeholders are appended to the payload (after markdown section, before non-markdown section, or inline in the non-markdown section)

## 2. New Git Plumbing Methods

### RevParseTree
```go
// RevParseTree returns the tree SHA of a commit-ish (e.g. HEAD, or a commit SHA).
// Uses: git rev-parse <ref>^{tree}
// Returns "" and a nil error for an unborn repo when ref == "HEAD" (caller gates on isUnborn).
RevParseTree(ctx context.Context, ref string) (string, error)
```

### TreeDiff
```go
// TreeDiff returns the diff between two tree SHAs (concept diff for decompose).
// Uses: git diff <treeA> <treeB> with binary filtering applied.
// Never uses --cached (it's tree-to-tree, not index-vs-anything).
// Applies the same caps/excludes/binary-filtering as StagedDiff.
TreeDiff(ctx context.Context, treeA, treeB string, opts StagedDiffOptions) (string, error)
```

### ReadTree
```go
// ReadTree loads a tree object into the index (for chain rebuild in arbiter).
// Uses: git read-tree <tree>
// MUTATES THE INDEX — used only by the arbiter's mid-chain rebuild path.
ReadTree(ctx context.Context, tree string) error
```

### StatusPorcelain
```go
// StatusPorcelain returns `git status --porcelain` output.
// Used to detect leftovers after the decompose loop (arbiter trigger).
// Non-empty output → arbiter runs. Empty → perfect run.
StatusPorcelain(ctx context.Context) (string, error)
```

### WorkingTreeDiff
```go
// WorkingTreeDiff returns the unstaged working-tree diff (planner input).
// Uses: git diff (NO --cached flag) with binary filtering applied.
// Applies the same caps/excludes as StagedDiff.
WorkingTreeDiff(ctx context.Context, opts StagedDiffOptions) (string, error)
```

## 3. Testing Strategy for New Git Methods

All new methods are tested with a temp git repo + real git binary (matching v1 pattern in `internal/git/*_test.go`):
- `RevParseTree`: create commits, assert tree SHA matches `git cat-file -p HEAD^{tree}`
- `TreeDiff`: create two trees with known differences, assert diff content
- `ReadTree`: load a tree, assert index state matches
- `StatusPorcelain`: create dirty/clean states, assert porcelain output
- `WorkingTreeDiff`: modify files without staging, assert diff content
- Binary filtering: add binary files (e.g., a PNG), assert placeholder lines appear and bodies are excluded
