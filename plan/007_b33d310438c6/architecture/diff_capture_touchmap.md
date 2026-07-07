# Diff-Capture Touch Map — FR3d–FR3i (plan 007)

Handoff map for adding diff-payload optimizations (`token_limit` overlay, `-M` rename detection,
`-U<diff_context>` reduced context, numstat skeleton prepend, `index`-line stripping, dynamic
water-fill truncation) to the three sibling diff functions. Read-only scouting — no files changed.

---

## 1. The THREE sibling diff functions (FR3c parity)

All three live in `internal/git/git.go`. **They DUPLICATE the logic inline — there is NO shared
argv/cap helper.** Each function is a near-verbatim copy of the others; the ONLY difference is the
diff-domain positional args (`diffArgs`) threaded into `g.run(ctx, g.workDir, ...)`. This is the
central refactor risk: any new flag/filter/skeleton must be applied to all three, OR a shared
helper extracted first.

| Function | File:Lines | Domain positional args (`diffArgs`) |
|---|---|---|
| `StagedDiff` | `internal/git/git.go:642-764` | `--cached` (inserted after `diff`) |
| `TreeDiff` | `internal/git/git.go:1094-1207` | `treeA`, `treeB` (two tree SHAs after `diff`) |
| `WorkingTreeDiff` | `internal/git/git.go:1228-1341` | *(none)* — bare `git diff` |

### Shared three-part structure (identical in all three)
1. **Part 1 — markdown, per-file, line-capped.** Lists `.md`/`.markdown` files, diffs each individually,
   caps at `maxMDLines` lines.
2. **Binary filtering** (`detectBinaryFiles` + `fileStatuses` from `internal/git/binary.go`) → FR3b
   `[binary]` placeholders + pathspec excludes.
3. **User-exclude placeholders** (`detectExcludedStatuses`) → `[excluded]` placeholders.
4. **Part 2 — non-markdown aggregate, byte-capped.** Single `git diff` with excludes + `:!*.md` +
   bin excludes, capped at `maxDiffBytes` bytes.

### Current argv construction (per function, verbatim excerpts)

Each function builds `nmArgs` inline. The `-M` / `-U<diff_context>` flags (FR3e/FR3f) must be
inserted right after the leading `"diff"` token. Here is the exact current shape:

**StagedDiff** (`git.go:748-756`):
```go
nmArgs := []string{"diff", "--cached", "--"}
nmArgs = append(nmArgs, excludes...)
nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
nmArgs = append(nmArgs, binExcludes...)
```
Markdown per-file loop builds: `g.run(ctx, g.workDir, "diff", "--cached", "--", file)` (git.go:666) and
the md-list: `g.run(ctx, g.workDir, "diff", "--cached", "--name-only", "--", "*.md", "*.markdown")` (git.go:662).

**TreeDiff** (`git.go:1183-1187`):
```go
nmArgs := []string{"diff", treeA, treeB, "--"}
nmArgs = append(nmArgs, excludes...)
nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
nmArgs = append(nmArgs, binExcludes...)
```
md-list: `g.run(ctx, g.workDir, "diff", treeA, treeB, "--name-only", "--", "*.md", "*.markdown")` (git.go:1112);
per-file: `g.run(ctx, g.workDir, "diff", treeA, treeB, "--", file)` (git.go:1120).

**WorkingTreeDiff** (`git.go:1318-1322`):
```go
nmArgs := []string{"diff", "--"}
nmArgs = append(nmArgs, excludes...)
nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
nmArgs = append(nmArgs, binExcludes...)
```
md-list: `g.run(ctx, g.workDir, "diff", "--name-only", "--", "*.md", "*.markdown")` (git.go:1244);
per-file: `g.run(ctx, g.workDir, "diff", "--", file)` (git.go:1252).

> **FR3e/FR3f insertion point:** every `"diff"` argv (the two in Part 1 + Part 2's `nmArgs` =
> **3 argv sites × 3 functions = 9 sites total**) needs `-M` and `-U<diff_context>`. They go
> immediately after the domain positionals and before `--`/`--name-only`. A shared helper
> `appendDiffFlags(base, opts)` would eliminate the 9× duplication — recommended refactor.

### Shared helpers already in `internal/git/binary.go` (variadic `diffArgs` pattern — the model for a shared argv helper)
- `detectBinaryFiles(ctx, diffArgs ...string)` (`binary.go:98`) — appends `--numstat`.
- `fileStatuses(ctx, diffArgs ...string)` (`binary.go:130`) — appends `--name-status`.
- `detectExcludedStatuses` (in `git.go`) — also takes `diffArgs` as trailing varargs.

**FR3g note:** `detectBinaryFiles` already runs `git diff <diffArgs> --numstat` — the skeleton
numstat call (FR3g) partially overlaps with this existing call. The skeleton needs the FULL file
set (added/deleted/path), whereas `detectBinaryFiles` only keeps `-/-` rows; FR3g would need its own
capture or a refactor to return the parsed numstat rows.

### Cap-default constants (`git.go:594-595`)
```go
defaultMaxMDLines   = 100
defaultMaxDiffBytes = 300000
```
Each function re-applies these when `opts.MaxMDLines`/`opts.MaxDiffBytes <= 0`. This is the
`token_limit == 0` legacy fast-path that FR3i must leave byte-identical.

---

## 2. `StagedDiffOptions` struct + ALL call sites

### The struct (`internal/git/git.go:36-44`)
```go
type StagedDiffOptions struct {
	MaxDiffBytes     int      // byte cap on the non-markdown section; 0 = unlimited
	MaxMDLines       int      // per-file line cap for markdown; 0 = unlimited
	Excludes         []string // pathspec magic-prefix excludes
	BinaryExtensions []string // extra non-text extensions to filter beyond built-in denylist
}
```
**Add `TokenLimit int` and `DiffContext int` here** (per delta_prd.md §2). Every path receives them
from one place.

### Production call sites (6 total — ALL construct the struct inline, identically)

> There is NO central bridge function. Each call site manually maps `cfg.MaxDiffBytes` /
> `cfg.MaxMdLines` / `cfg.BinaryExtensions` / `deps.Excludes` into the struct. **All 6 must add
> `TokenLimit` and `DiffContext` (sourced from `cfg.TokenLimit` / `cfg.DiffContext`).**

| # | File:Line | Function | Method called |
|---|---|---|---|
| 1 | `internal/generate/generate.go:163-169` | `CommitStaged` | `StagedDiff` |
| 2 | `internal/hook/exec.go:104-110` | `Run` (hook path) | `StagedDiff` |
| 3 | `pkg/stagecoach/stagecoach.go:423-429` | `runPipeline` | `StagedDiff` |
| 4 | `internal/decompose/planner.go:69-75` | `callPlanner` | `TreeDiff` |
| 5 | `internal/decompose/message.go:71-77` | `generateMessage` | `TreeDiff` |
| 6 | `internal/decompose/decompose.go:608-614` | `runArbiter` caller (leftover diff) | `TreeDiff` |

Representative call site (identical shape at all 6):
```go
diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
    MaxDiffBytes:     cfg.MaxDiffBytes,
    MaxMDLines:       cfg.MaxMdLines,
    BinaryExtensions: cfg.BinaryExtensions,
    Excludes:         deps.Excludes,
})
```
> `deps.Excludes` comes from the FR-X1 union-resolved exclude list; `cfg.BinaryExtensions` from the
> `[generation]` table. The new `TokenLimit`/`DiffContext` map from `cfg.TokenLimit`/`cfg.DiffContext`.

**Note:** `WorkingTreeDiff` (the 3rd sibling) has NO production call site in the current code — only
test sites (`internal/git/workingtreediff_test.go`). It was wired in plan 002 but the live
planner uses `TreeDiff(baseTree, tStart)` (the FROZEN concept diff, `planner.go:69`), not
`WorkingTreeDiff`. So FR3c-parity changes to `WorkingTreeDiff` are exercised only by its own unit
tests today — keep them green.

### Test call sites
- `internal/git/stagediff_test.go` (21 calls) — all use `StagedDiffOptions{}` zero-value or single-field.
- `internal/git/treediff_test.go` (17 calls).
- `internal/git/workingtreediff_test.go` (19 calls).

---

## 3. Config layer

### `Config.Generation` fields (`internal/config/config.go`)
The `[generation]` fields are flat on `Config` (NOT a nested struct). Relevant block at **`config.go:77-93`**:
```go
// [generation] (PRD §16.2)
MaxDiffBytes        int `toml:"max_diff_bytes"`
MaxMdLines          int `toml:"max_md_lines"`
MaxDuplicateRetries int `toml:"max_duplicate_retries"`
SubjectTargetChars  int `toml:"subject_target_chars"`
...
MaxCommits       int      `toml:"max_commits"`
BinaryExtensions []string `toml:"binary_extensions"`
Exclude          []string `toml:"exclude"`
```
**Add `TokenLimit int toml:"token_limit"` (default 0) and `DiffContext int toml:"diff_context"`
(default 1) next to `MaxDiffBytes`/`MaxMdLines`.** `DiffContext` default 1 ≠ 0, so the standard
`!= 0` overlay guard needs care (see §3b note).

### `Defaults()` (`internal/config/config.go:155-190`)
```go
MaxDiffBytes: 300000,
MaxMdLines:   100,
```
**Add:** `TokenLimit: 0,` (unset ⇒ legacy caps) and `DiffContext: 1,` (FR3f reduced-context default).
> ⚠️ `DiffContext` default `1` is non-zero. The non-zero-overlay pattern (`if src.X != 0`) works
> for the user setting it, but the materialize/overlay code must distinguish "unset" from "explicit 0"
> (`-U0` is a valid FR3f value = changed lines only). Two options: (a) pointer `*int` (like
> `Output`/`StripCodeFence`), or (b) sentinel `-1` default. The pointer pattern has precedent
> (`Output *string`, `StripCodeFence *bool` at config.go:88-89). Flag for implementer decision.

### `fileGeneration` struct (`internal/config/file.go:49-62`)
```go
type fileGeneration struct {
	MaxDiffBytes        int      `toml:"max_diff_bytes"`
	MaxMdLines          int      `toml:"max_md_lines"`
	...
	BinaryExtensions    []string `toml:"binary_extensions"`
	Exclude             []string `toml:"exclude"`
	Format              string   `toml:"format"`
	...
	Push                bool     `toml:"push"`
}
```
**Add:** `TokenLimit int toml:"token_limit"` and `DiffContext int toml:"diff_context"`.

### Field-merge / precedence — THREE sites to wire (all next to MaxDiffBytes/MaxMdLines)

**(a) `materialize`** (`file.go:~205-215`) — single-file `*Config` builder:
```go
if g.MaxDiffBytes != 0 { c.MaxDiffBytes = g.MaxDiffBytes }
if g.MaxMdLines != 0   { c.MaxMdLines = g.MaxMdLines }
```
**Add:** the same `!= 0` guard for `TokenLimit`/`DiffContext` (with the `DiffContext` 0-vs-unset caveat above).

**(b) `overlay`** (`file.go:~308-318`) — cross-layer field-by-field merge:
```go
if src.MaxDiffBytes != 0 { dst.MaxDiffBytes = src.MaxDiffBytes }
if src.MaxMdLines != 0   { dst.MaxMdLines = src.MaxMdLines }
```
**Add:** the matching guards. Precedence order: global → repo → git-config → env → flag.

**(c) git-config resolver** (`internal/config/git.go:181-205`) — `resolveGitConfig`:
```go
if v, found, err := gitConfigGet(repoDir, "stagecoach.maxDiffBytes"); err != nil { ... } else if found {
    if err := parseInt(repoDir, "stagecoach.maxDiffBytes", v, &c.MaxDiffBytes); err != nil { return nil, err }
}
if v, found, err := gitConfigGet(repoDir, "stagecoach.maxMdLines"); err != nil { ... } else if found {
    if err := parseInt(repoDir, "stagecoach.maxMdLines", v, &c.MaxMdLines); err != nil { return nil, err }
}
```
**Add:** two parallel blocks for `stagecoach.tokenLimit` and `stagecoach.diffContext` (camelCase, per
the existing `maxDiffBytes`/`maxMdLines`/`stripCodeFence` convention). `parseInt` (`git.go:87`) and
`gitConfigGet` (`git.go:48`) are the helpers — copy the exact 4-line pattern.

### `config init` template (`internal/config/bootstrap.go:282-298`)
```go
const generationCommented = `
# max_diff_bytes = 300000  ...
# max_md_lines   = 100     ...
...`
```
**Add:** commented `token_limit` and `diff_context` lines; annotate legacy caps as
"ignored when token_limit is set".

> **No env/flag layer wiring needed** for FR3d/FR3f (config-file + git-config keys only per delta_prd).
> Confirm in M2 whether `--token-limit`/`--diff-context` flags are wanted (PRD specifies config-only).

---

## 4. Config → `StagedDiffOptions` bridge

**There is no bridge function.** The mapping is duplicated inline at all 6 production call sites
(§2 table). Each maps `cfg.{MaxDiffBytes,MaxMdLines,BinaryExtensions}` + `deps.Excludes` into the
struct literal. To add `TokenLimit`/`DiffContext`, add `TokenLimit: cfg.TokenLimit, DiffContext:
cfg.DiffContext` to each of the 6 struct literals. No signature changes to the diff methods are
needed (they already take `opts StagedDiffOptions`).

> **Refactor opportunity (optional):** a `config.DiffOpts(excludes)` helper returning
> `git.StagedDiffOptions` would collapse the 6 duplications and make future option additions
> single-site. Not required by the delta; flag for implementer.

---

## 5. Payload consumption + token estimation

### Where the diff string is consumed
The diff string from all three diff methods is the **verbatim tail** of the user payload. Two builders:

**(a) Message path** — `prompt.BuildUserPayload(diff, context, rejected)` (`internal/prompt/payload.go:131-172`).
Called by `generate.go:217`, `hook/exec.go` (after line 110), `stagecoach.go` (runPipeline). Assembly:
```
userInstruction + "\n\n" + [contextBlock + "\n\n"] + diff   // diff appended verbatim
```

**(b) Planner path** — `prompt.BuildPlannerUserPayload(diff, context, forcedCount)`
(`internal/prompt/planner.go:139-151`). Called by `decompose/planner.go:83`. Same pattern: diff is
the verbatim tail.

The diff's trailing bytes (incl. truncation sentinel) are preserved as-is by both builders — they
do NOT normalize. So any skeleton-prepend / index-strip / water-fill shaping happens **inside the
git layer** (before the string is handed to these builders), exactly as the delta_prd §2 seam note
states.

### Token estimation — DOES NOT EXIST
Grep across all `*.go` for `token`/`Token`/char-per-token heuristics finds **NO prompt-token
estimation anywhere.** All "token" hits are about CLI-arg tokens (provider flags) or JSON-contract
literal `<...>` placeholders — none estimate prompt/example token counts. **FR3i's
`≈4 chars/token` estimator is net-new.** The delta_prd §2 + M2.T2 specify threading the
prompt+example+margin reserve into the git layer via `StagedDiffOptions` (a new field) or a sibling
param — the git layer owns the diff body, so water-fill naturally lives there.

> **Open coupling question (flag for implementer):** the message path builds the payload per-attempt
> in a retry loop (`generate.go:218`), and the rejection-list grows across attempts — so the prompt
> size is NOT fixed when `StagedDiff` is called (it's called once, before the loop). FR3d's
> "holistic budget over prompt+examples+diff" therefore needs either (a) a worst-case reserve
> (measure instruction + max-rejection-block + max examples) passed into `StagedDiff`, or (b) a
> post-hoc truncation pass. The delta_prd leaves this to the implementer ("pick the lower-coupling
> option"). The cleanest seam is a `PromptReserveTokens` field on `StagedDiffOptions` measured
> upstream from `BuildUserPayload`'s constants + `RecentMessages` output.

---

## 6. Truncation sentinels + test fixtures to keep stable

### Sentinels (each appears 3× — once per function)
```
"\n... [diff truncated at %d bytes]"   // non-markdown byte cap (Part 2)
"\n... [diff truncated at %d lines]"   // markdown per-file line cap (Part 1)
```
Source sites: `git.go:679 & 758` (StagedDiff), `git.go:1128 & 1195` (TreeDiff), `git.go:1262 &
1330` (WorkingTreeDiff).

> **FR3i sentinel contract:** when `token_limit > 0`, the water-fill replaces the byte cap and emits
> `... [truncated]` (delta_prd uses this shorter form, NOT the `at %d bytes` form). When
> `token_limit == 0`, the EXISTING sentinels must remain byte-identical (regression guard).

### Test fixtures asserting on diff output (keep stable under `token_limit == 0`)
- `internal/git/stagediff_test.go:106` (MaxMDLines:10 → `... [diff truncated at 10 lines]`)
- `internal/git/stagediff_test.go:129` (MaxDiffBytes:100 → `... [diff truncated at 100 bytes]`)
- `internal/git/treediff_test.go:240` / `:265`
- `internal/git/workingtreediff_test.go:206` / `:229`
- Plus `diff --git a/<file>` count assertions (`stagediff_test.go:92`, `workingtreediff_test.go:185`,
  `treediff_test.go:217`) — these assert on the per-file boundary marker that FR3h index-stripping
  must PRESERVE and FR3i water-fill must split on.

### Regression guard (delta_prd §4 acceptance)
`token_limit == 0` (default) MUST be byte-identical to pre-delta output. The existing golden-shape
tests above are that guard. `diff_context` default `1` (≠ git's `3`) is the one always-on change that
ALTERS existing output — so the byte-cap/line-cap tests' fixture expectations (which currently assume
git's default `-U3`) will need updating to reflect `-U1` context. **Flag: the stagediff/treediff/
workingtreediff golden tests WILL change under FR3f even at `token_limit==0`.**

---

## Start Here
Open `internal/git/git.go:642` (`StagedDiff`). It is the canonical implementation that `TreeDiff`
(1094) and `WorkingTreeDiff` (1228) copy. Decide FIRST whether to extract a shared
`appendDiffFlags`/`buildDiffArgs` helper (recommended — eliminates the 9× flag-insertion sites and
makes FR3e/FR3f/FR3g/FR3h a single-site change) or to edit all three in lockstep. Then thread
`opts.TokenLimit`/`opts.DiffContext` from `StagedDiffOptions:36`, wire config at the 3 config sites
(file.go materialize `~210`, overlay `~312`, git.go resolver `~181`) + `Defaults()` + `bootstrap.go`,
and add the 2 new fields to the 6 production call-site struct literals.

## Architecture (data flow)
```
config.toml / git-config / flags
   → Defaults() + materialize() + overlay()  [file.go]  + resolveGitConfig() [git.go]
   → config.Config{MaxDiffBytes, MaxMdLines, TokenLimit(NEW), DiffContext(NEW), ...}
   → 6 call sites map cfg → git.StagedDiffOptions{...}        [generate/hook/stagecoach/decompose]
   → StagedDiff/TreeDiff/WorkingTreeDiff(opts)                [git.go: 642/1094/1228]
        ├─ Part 1: md per-file diff (+ -M/-U<n> NEW), line-capped
        ├─ binary/excluded placeholders (detectBinaryFiles/fileStatuses)
        └─ Part 2: non-md aggregate diff (+ -M/-U<n> NEW), byte-capped OR water-fill(NEW when token_limit>0)
              (+ index-line strip NEW, + numstat skeleton prepend NEW)
   → diff string (verbatim tail)
   → prompt.BuildUserPayload / BuildPlannerUserPayload       [payload.go / planner.go]
   → provider.Render → agent stdin
```

## Open questions / risks (for implementer)
1. **`DiffContext` 0-vs-unset:** default `1` but `0` is valid (`-U0`). Standard `!= 0` overlay guard
   conflates them. Use `*int` pointer (precedent: `Output`/`StripCodeFence`) or sentinel `-1`.
2. **No shared argv helper:** 9 flag-insertion sites (3 per function). Refactor risk; recommend
   extracting `buildDiffArgs(domain, opts) []string`.
3. **FR3g skeleton vs `detectBinaryFiles`:** both run `--numstat`; potential to share one parse.
4. **FR3i prompt-reserve coupling:** `StagedDiff` is called ONCE before the retry loop; the rejection
   list grows per-attempt, so prompt size isn't fixed. Needs worst-case reserve or post-hoc pass.
5. **FR3f alters existing golden tests** even at `token_limit==0` (default context 3→1). Update fixtures.
6. **`WorkingTreeDiff` has no live caller** — parity changes are test-only-exercised today.
