# StagedDiff Validation Research — P1.M1.T3.S1

> Empirically verified on **git 2.54.0 + Go 1.26.4** against throwaway temp repos. This document is
> the source of truth for the `StagedDiff` implementation. Every claim below was re-confirmed by
> running the actual git commands before writing this file.

## 1. Signature reconciliation

`StagedDiff`'s signature is **already fixed** by the landed `Git` interface (P1.M1.T2.S1) and the
`StagedDiffOptions` value type (also landed in S1, `internal/git/git.go`):

```go
type StagedDiffOptions struct {
	MaxDiffBytes int      // byte cap on the non-markdown section; 0/neg ⇒ commit-pi default
	MaxMDLines   int      // per-file line cap for markdown files; 0/neg ⇒ commit-pi default
	Excludes     []string // pathspec magic-prefix excludes; empty ⇒ commit-pi default set
}

// on Git interface (already declared):
StagedDiff(ctx context.Context, opts StagedDiffOptions) (diff string, err error)
```

The current disk stub is:
```go
func (g *gitRunner) StagedDiff(ctx context.Context, opts StagedDiffOptions) (string, error) {
	panic("gitRunner.StagedDiff: not yet implemented — see P1.M1.T3.S1")
}
```

**No interface change, no new type, no new method signature.** This subtask replaces ONLY the stub
body (plus declares two package-level default constants and one default-excludes `var`).

## 2. Empirically-pinned git diff behavior (the spec)

All commands run via `exec.CommandContext` with args as `[]string` (NO shell — PRD §19). Verified
against real temp repos initialized with `git init`.

### 2.1 Part 1 — markdown file list (`--name-only`)
```
git diff --cached --name-only -- '*.md' '*.markdown'
```
- **With staged .md files:** prints their paths, one per line, exit **0**.
- **With nothing staged:** prints nothing, exit **0** (NOT an error).
- **With no .md but .go staged:** prints nothing, exit 0.
- The globs `*.md` / `*.markdown` are interpreted by **git's pathspec matcher** (not the shell),
  because they are passed as literal `[]string` args — verified: `big.md`, `small.md` listed
  correctly; a `.txt` file was NOT listed.

### 2.2 Part 1 — per-file markdown diff
```
git diff --cached -- '<file>'
```
- Returns the full textual diff for that one file (e.g. 158 lines for a ~150-line content change).
- Each file's diff is a self-contained block starting with `diff --git a/<f> b/<f>`.
- The `--` ensures a filename beginning with `-` is treated as a path, not an option (safe).

### 2.3 Part 2 — non-markdown aggregate WITH excludes
```
git diff --cached -- ':!*.lock' ':!package-lock.json' ':!pnpm-lock.yaml' ':!yarn.lock' ':!*.snap' ':!*.map' ':!vendor/*' ':!*.md' ':!*.markdown'
```
- The `:!pattern` tokens are git **pathspec exclude magic**. Each is passed as a SEPARATE argv
  element (no shell, no quoting) — verified working via `exec.Command`.
- **Excludes are applied correctly:** with `app.go`, `package-lock.json`, `x.snap`, `x.map`,
  `vendor/lib.go`, `big.md` all staged, the output's `+++` lines showed **only** `app.go`.
- **CRITICAL (the double-count trap):** if `:!*.md` `:!*.markdown` are OMITTED from Part 2, markdown
  files appear in BOTH Part 1 (per-file) AND Part 2 (aggregate) — verified: a repo with `app.go` +
  `many.md` yielded **2** `+++` lines in Part 2 when `:!*.md` was absent (both files), vs **1**
  (`app.go` only) when present. **Conclusion:** the markdown excludes in Part 2 are STRUCTURAL
  (prevents duplication), NOT merely a noise filter, and must always be present.

### 2.4 Exit codes (the FINDING 6 cousin)
- `git diff --cached` **without** `--quiet` exits **0** on success, whether or not there are changes.
  It exits **128** (non-zero) only on a real error (bad pathspec, corrupt repo, etc.).
- This is DIFFERENT from `git diff --cached --quiet` (exit 1 = staged; FINDING 6, owned by
  P1.M1.T3.S2 HasStagedChanges). StagedDiff uses the **non-quiet** form, so `exitCode != 0` ⟺ real
  error. Branch on `code != 0` (stable), NOT `code == 128`.

### 2.5 Caps are POST-capture (FINDING 7)
- `git diff` has **no** `--max-bytes` or `--max-lines` flag. commit-pi pipes through `head -c` /
  `head -n`. In Go: capture `run()`'s stdout to a string, THEN truncate.
- Markdown: split on `"\n"`, keep the first `maxMDLines` lines, join with `"\n"`.
- Non-markdown: slice the byte string to `maxDiffBytes` (`s[:maxDiffBytes]`); `len(s)` is byte
  length (correct). A byte slice MAY split a multi-byte UTF-8 rune — this matches commit-pi's
  `head -c` behavior exactly; documented as an acceptable v1 tradeoff (the model tolerates it).
- A truncation **sentinel** is appended so the model knows the diff is partial.

## 3. Design decisions

### D1 — Defaults applied on zero/negative caps (NOT "0 = unlimited")
The `StagedDiffOptions` field comments (S1) say "0 = unlimited". This PRP **supersedes** that
aspiration for v1: a zero/negative cap applies the commit-pi default (`defaultMaxMDLines = 100`,
`defaultMaxDiffBytes = 300000`). Rationale: (a) the item description explicitly says "use constants
matching commit-pi defaults"; (b) the orchestrator (P1.M3.T4) builds opts from resolved config which
defaults to 100/300000 anyway; (c) unbounded capture is a footgun (a multi-MB staged diff would
blow an agent's context window — PRD §22.1 risk). An explicit "unlimited" opt-in is not needed for
v1. **If `opts.MaxMDLines <= 0` → use 100; if `opts.MaxDiffBytes <= 0` → use 300000.**

### D2 — Excludes: caller-overridable noise filters + STRUCTURAL markdown exclusion
- The noise-filter set (lock/snap/map/vendor) defaults to `defaultExcludes` but is **overridable**
  via `opts.Excludes` (if non-empty, it REPLACES the default set — enabling a future config knob).
- The markdown excludes (`:!*.md`, `:!!*.markdown`) are **always appended** to Part 2 regardless of
  `opts.Excludes`, because they are structural (§2.3 double-count trap). A caller CANNOT disable
  them via opts — doing so would duplicate markdown content in the payload.

### D3 — Two git invocations for Part 1 (list, then per-file), one for Part 2
Matches commit-pi exactly. Per-file diffing (rather than a single `git diff -- '*.md'` with
in-band parsing) keeps the per-file line cap trivial: `split on \n, take N`. The cost is N+2 git
invocations for N markdown files — acceptable (markdown-file counts in real repos are small; the
diff is the expensive part, not the process spawns). Alternative (single aggregate + parse hunk
boundaries) is more complex and fragile — rejected for v1.

### D4 — Sentinels (exact text, documented constants)
- Part 1 (per-file markdown, over line cap): append
  `"\n... [diff truncated at <N> lines]"` (N = effective maxMDLines).
- Part 2 (non-markdown, over byte cap): append
  `"\n... [diff truncated at <N> bytes]"` (N = effective maxDiffBytes).
Built via `fmt.Sprintf` (fmt already imported — NO new import). Nothing downstream parses the exact
text (the prompt builder P1.M3.T1.S3 treats the payload as opaque), so informative text is strictly
better. The item description's minimal `'\n... [diff truncated]'` is the seed; D4 enriches it with
the bound value for model clarity.

### D5 — Concatenation: markdown section FIRST, then non-markdown, no separator
Matches commit-pi (Appendix C: "staged-diff capture (md + other)"). Each per-file markdown diff is
written with a guaranteed trailing `"\n"`; the non-markdown section follows directly. No artificial
`---`/blank separator (commit-pi adds none; each hunk has its own `diff --git` header so the model
can tell them apart). Empty repo → empty string, no error.

### D6 — Zero new imports
`StagedDiff` uses `fmt` (Sprintf), `strings` (Builder, Split, Join, TrimSpace, HasSuffix) — BOTH
already imported in `git.go` (S1's import block: bytes, context, errors, fmt, io, os/exec, strings).
`run()` (S1) provides the exec. Adding an import would be an unused-import compile error. **Do NOT
add `strconv`** — use `fmt.Sprintf("%d", n)` for the sentinels.

### D7 — Inline the name-only split (no new helper function)
The file-list parse (`strings.Split(strings.TrimSpace(mdList), "\n")` + skip-empty) is trivial and
used once. S6 added `parseDiffTree` because the 3-field routing was non-trivial and worth testing
in isolation; here the parse is a one-liner, so inlining keeps the edit to a single contiguous
region (constants + method body). Decision: inline.

## 4. Edge cases (all verified or deducible from §2)

| Case | Behavior |
|---|---|
| Nothing staged | mdList empty, both diffs empty → return `""`, `nil` (caller gates on HasStagedChanges, but safe unconditionally) |
| Only markdown staged | Part 1 populated, Part 2 empty (markdown excluded) → return md section |
| Only non-markdown staged | Part 1 empty, Part 2 populated → return non-md section |
| Both | concatenated |
| Markdown file over line cap | truncated to N lines + sentinel (D4) |
| Non-md aggregate over byte cap | truncated to N bytes + sentinel (D4) |
| Lock/snap/map/vendor staged | EXCLUDED from Part 2 (D2 default set) |
| `opts.Excludes` non-empty | REPLACES default noise filters; markdown exclusion still appended (D2) |
| `opts` zero-value `StagedDiffOptions{}` | defaults applied: 100 lines, 300000 bytes (D1) |
| Bad pathspec / corrupt repo | `git diff` exits 128 → wrapped error (§2.4) |
| git binary missing | `run()` returns `err != nil` mentioning "git binary not found" → propagated |
| Context cancelled | `run()` returns `err = context.Canceled` → propagated (`errors.Is`) |
| Filename with spaces | handled: passed as ONE `[]string` arg (no shell); `--` guards leading-dash names |
| UTF-8 split at byte boundary | acceptable v1 tradeoff, matches `head -c` (D4/§2.5) |

## 5. Test design (mirrors S6/S4 structure: `package git`, `sd`-prefixed helpers)

`internal/git/stagediff_test.go`, `package git` (white-box — reaches `gitRunner`/`run()`). REUSE
existing helpers: `initRepo` (git_test.go), `writeFile` + `stageFile` (committree_test.go). NO
new fixture helpers strictly required (content generation inlined via `strings.Repeat`/a small
loop). Helper prefix `sd` if any are added (collides with nothing: S4=`setIdentityConfig`/`writeFile`
/`stageFile`/`writeTreeOf`/`headSHA`/`commitMessage`; S5=`cas*`/`gitIdentityEnv`; S6=`dtCommit`/
`dtRemove`; S2=`minGitEnv`/`makeEmptyCommit`).

Test matrix (9 functions):

| Test | Fixture | Key assertions |
|---|---|---|
| `TestStagedDiff_MarkdownAndCode` | stage `a.md` + `b.go` | payload contains the md hunk AND the go hunk |
| `TestStagedDiff_ExcludesLockSnapMapVendor` | stage `b.go` + `p.lock` + `x.snap` + `y.map` + `vendor/v.go` | `b.go` present; `.lock`/`.snap`/`.map`/`vendor/` ABSENT |
| `TestStagedDiff_MarkdownNotInNonMarkdownSection` | stage `a.md` (only) | payload contains `a.md` exactly ONCE (no double-count) |
| `TestStagedDiff_MarkdownLineCap` | stage `big.md` (>10 diff lines); `opts.MaxMDLines=10` | md hunk truncated; contains the `... [diff truncated at 10 lines]` sentinel; line count ≤ 11 |
| `TestStagedDiff_NonMarkdownByteCap` | stage `big.go` (>100-byte diff); `opts.MaxDiffBytes=100` | non-md section truncated; contains `... [diff truncated at 100 bytes]` sentinel; byte length bounded |
| `TestStagedDiff_NothingStaged` | fresh repo, nothing staged | returns `""`, `nil` |
| `TestStagedDiff_OnlyMarkdown` / `TestStagedDiff_OnlyCode` | single file type | only that type present |
| `TestStagedDiff_CustomExcludesOverride` | stage `keep.go` + `drop.go`; `opts.Excludes=[]{":!drop.go"}` | `drop.go` absent, `keep.go` present (override works; md exclusion still appended) |
| `TestStagedDiff_DefaultsOnZero` | `opts=StagedDiffOptions{}`; stage `big.md`+`big.go` | no panic; line cap 100 + byte cap 300000 applied (assert sentinel NOT present when under cap, present when over) |
| `TestStagedDiff_GitBinaryMissing` | `t.Setenv("PATH","")` | err contains "git binary not found" |
| `TestStagedDiff_ContextCancelled` | cancel ctx before call | `errors.Is(err, context.Canceled)` |
| `TestStagedDiff_MarkdownExtensions` | stage `a.md` + `b.markdown` | both listed/captured |

(Consolidate to ~9–11 functions; the `.markdown` extension case can fold into `MarkdownAndCode`
or stand alone.)

## 6. Concurrency with P1.M1.T2.S6 (DiffTree)

S6 (DiffTree) edits:
- `git.go`: replaces the `DiffTree` stub + adds `parseDiffTree` — a region ABOVE the `StagedDiff`
  stub. T3.S1 replaces the `StagedDiff` stub (immediately after) — **distinct, non-overlapping
  region**.
- `git_test.go`: removes the `DiffTree` line from `TestStubsPanic`. T3.S1 removes the `StagedDiff`
  line — **distinct line**.
- creates `difftree_test.go`. T3.S1 creates `stagediff_test.go` — **distinct file**.

No text overlap. Both use distinct helper prefixes (`dt` vs `sd`). On the current disk snapshot
S6 has already landed (DiffTree is real, `parseDiffTree` present, `TestStubsPanic` lists 6 stubs);
T3.S1 builds on that exact state. The parallel note is satisfied: T3.S1 consumes `run()` (S1) and
the `StagedDiffOptions` type (S1) only — it does NOT touch DiffTree/parseDiffTree.

## 7. Decisions log (quick reference)

- **D1** zero/neg cap ⇒ commit-pi default (100/300000), NOT unlimited.
- **D2** `opts.Excludes` overrides noise filters; markdown exclusion is structural (always appended).
- **D3** list + per-file for markdown; single aggregate for non-markdown (matches commit-pi).
- **D4** sentinels carry the bound value; built via `fmt.Sprintf` (no new import).
- **D5** concatenate md+other, no separator; empty repo ⇒ `""`.
- **D6** zero new imports (fmt, strings present).
- **D7** inline the name-only split (no new helper).
