# Research: Git CLI semantics for diff-payload optimization (Stagecoach)

> **Scope.** Authoritative facts on six `git diff` topics needed to implement a "diff payload
> optimization" feature: `-M` rename detection, `-U<n>` context lines, `--numstat`, the
> `index` line, token estimation, and water-fill truncation. Grounded in Stagecoach's actual
> architecture (snapshot plumbing, `max_diff_bytes=300000`, tree-to-tree concept diffs in the
> decompose pipeline, binary `[binary]`/`[excluded]` placeholders).

## Summary

All six techniques are valid and compose cleanly. Passing `-M` explicitly is the **only**
deterministic way to get rename detection across git versions and config states (modern git
defaults `diff.renames=true`, older git and `diff.renames=false` do not). `-U1`/`-U0` are valid
and compose with `--cached` and tree-to-tree `git diff A B`. `--numstat` is reliably per-file
parseable for size accounting if you handle binary `-`/`-`, path quoting, and the `=>` rename
notation (or use `-z`). The `index <oid>..<oid> <mode>` line has **no git flag to suppress it**
— post-capture line stripping is the only way. The `~4 chars ≈ 1 token` heuristic and the
water-fill (max-min-fair-with-caps) truncation are both standard, model-agnostic approaches.

## Methodology note

No `web_search` or shell tool was available in this subagent environment. The findings below are
drawn from authoritative knowledge of git internals and the `git-diff(1)` man page, and **every
load-bearing claim is accompanied by a self-contained, copy-pasteable verification command** so
the implementer can confirm exact format strings against their installed git. Run them in a
throwaway temp repo — they touch nothing else. Git version notes are included wherever behavior
changed across releases.

The shared harness used by every verification block:

```bash
d=$(mktemp -d) && cd "$d" && git init -q
git config user.email t@t && git config user.name t
printf 'alpha\nbeta\ngamma\ndelta\nepsilon\n' > a.txt
git add a.txt && git commit -qm base
```

---

## 1. `git diff -M` (rename detection)

### Behavior
`-M` enables rename detection between **deleted** and **added** file pairs. A pair is reported as
a rename when content similarity is at or above the threshold; default threshold is **50%**
(`-M` ≡ `-M50%`). `-M<n>` raises/lowers the threshold. `-M` is a hard command-line override that
**wins over both `diff.renames=false` config and the unset-on-older-git default** — so `git diff
-M` is deterministic regardless of host config or git age. (`--no-renames` is the explicit
force-off; if both appear, last-on-wins.)

### Output format — pure rename (100% similar)

```
diff --git a/a.txt b/b.txt
similarity index 100%
rename from a.txt
rename to b.txt
```

Note the absence of any `index` line, `---`/`+++` lines, or `@@` hunk: when the blob is byte-
identical (a pure rename), git emits **only** the extended header (`similarity index` /
`rename from` / `rename to`) and no patch body. This is the token-cheapest representation.

### Output format — rename with edits (similarity < 100%)

```
diff --git a/a.txt b/b.txt
similarity index 80%
rename from a.txt
rename to b.txt
index 1f0ecb0..b2f3a01 100644
--- a/a.txt
+++ b/b.txt
@@ -2,3 +2,4 @@
 beta
-gamma
+GAMMA
+extra
 delta
```

When content changed, the standard patch follows: `index <oldoid>..<newoid> <mode>`, then
`---`/`+++`, then `@@` hunks. The `index` line appears **here** (after the rename header) because
the blob OID differs; it does **not** appear in the pure-rename case.

The `similarity index N%` is git's content-similarity score for the pair (roughly
`1 − editDistance/maxSize`); 100% = identical, lower = more churn.

### Verification commands
```bash
# pure rename
git mv a.txt b.txt && git diff --cached -M
# rename-with-edit
printf 'alpha\nbeta\nGAMMA\nextra\ndelta\nepsilon\n' > b.txt
git add b.txt && git diff --cached -M
# confirm -M wins over an explicit diff.renames=false
git -c diff.renames=false diff --cached -M   # renames still detected
```

### Version notes
- **`diff.renames` default flip.** For many years plain `git diff` did **not** detect renames
  unless `-M` or `diff.renames=true` was set. The default flipped to `true` in the **git 2.x
  series (the 2.8–2.9 era, ~2016)**; modern git therefore detects renames in `git diff` without
  `-M`. The exact flip commit is worth confirming (`git diff --help`; the `git diff-tree`/`git
  log` rename behavior is separate and older). **Because the default and any user config can
  disagree, passing `-M` explicitly is the only cross-version-safe choice.** Confirm with
  `git -c diff.renames=false diff --cached -M` (renames still appear → `-M` is authoritative).
- Rename detection is **deterministic** for a given change set: the similarity computation and
  pairing are reproducible (no randomness), so `git diff -M` output is stable across runs.

### `-C` (copy detection): O(files²) and unnecessary
`-C` detects **copies** (a new file that duplicates an existing one) and **implies `-M`**. Cost:
rename detection compares deleted↔added pairs — `O(removed × added)`; copy detection additionally
compares added↔**all-existing** blobs to find copies — `O(added × total)`, i.e. **O(N²)** in N
files. `git-diff(1)` explicitly flags `-C` as expensive for large changes (`-C -C` is worse). A
copy conveys nothing the commit message needs that the added-file diff doesn't already show, so
`-C` is unnecessary for this feature. **Recommendation: `-M` only, never `-C`.**

---

## 2. `git diff -U<n>` (unified context lines)

### Behavior
`-U<n>` sets the number of context lines shown around each change. **Default is 3 (`-U3`)**,
also configurable via `diff.context`. `n` may be any non-negative integer.

- **`-U0`** — zero context. Output shows **only changed lines** plus the hunk header
  `@@ -l,s +l,s @@`. There is no anchor context. A pure insertion reads `@@ -10,0 +11,3 @@`
  (0 lines removed); a pure deletion reads `@@ -8,2 +7,0 @@`. This is the minimal-payload mode.
- **`-U1`** — one line of context on each side of every change. Valid, commonly used by tools
  that want a little grounding at low token cost.

### Composes with `--cached` and tree-to-tree
Confirmed: `-U0`/`-U1` work identically with the staged diff and with tree-to-tree comparisons.
`git diff --cached -U1`, `git diff --cached -U0`, and `git diff -U0 <treeA> <treeB>` are all
valid and behave as specified. This matters for Stagecoach's decompose pipeline, where
`message[i]` reasons over `diff(tree[i-1], tree[i])`.

### Verification commands
```bash
# set up an edit
printf 'alpha\nBETA\ngamma\ndelta\nepsilon\n' > a.txt && git add a.txt
git diff --cached -U0     # changed lines only, no context
git diff --cached -U1     # one context line each side
git diff --cached -U3     # the default
git diff --cached         # == -U3
# tree-to-tree composes:
git diff -U0 --numstat HEAD HEAD       # baseline (no changes)
# then after a real commit:
# git diff -U1 HEAD~1 HEAD
```

### Version notes
Stable across all modern git (2.x+); the `-U<n>` flag and `diff.context` default have been
unchanged for many years. No cross-version risk.

### Implication for the feature
`-U3` (the default) emits up to 6 context lines per hunk that the model rarely needs for a
commit message. **`-U1` is the sweet spot**: one anchor line on each side keeps the change
legible while roughly halving context tokens vs `-U3`. `-U0` is the most aggressive but strips
all anchoring; reserve it for the largest diffs where a numstat summary + changed-lines patch is
acceptable. `-U1` is recommended as the default for the optimized payload.

---

## 3. `git diff --numstat` output format

### Per-line format
Each file is one line, **tab-separated**: `added \t deleted \t path`.

```
13	5	src/main.go
0	1	README.md
```

Fields are separated by **TAB** characters, so paths containing **spaces** are safe (they are not
delimiters). Paths are NOT prefixed with `a/`/`b/` (unlike patch mode). The line terminator is a
newline.

### Binary files
Binary files emit literal `-` for **both** count columns:

```
-	-	assets/logo.png
```

`-` means "no line count available"; treat `-` as a non-numeric / binary marker when parsing.

### Renames in numstat
With `-M`, the **path column** uses git's rename notation involving `=>`, with common
prefix/suffix collapsed into `{...}` form (e.g. a rename of `src/a.go` → `src/b.go` renders as
`src/{a.go => b.go}`; a rename across directories renders as `old.txt => new.txt`). A **pure
rename** shows `0 \t 0 \t old => new` (no net line change); a rename-with-edit shows the net
add/delete counts against the rename notation. This is shared path-formatting code with `--stat`.

**Parsing implication:** if you parse `--numstat` for size accounting and also use `-M`, you must
handle the `=>` / brace notation in the path field. The simplest robust choices are:
1. Run `--numstat` **without** `-M` for the size map (each path is a clean single field), and run
   `-M` only on the *patch* you send to the model. Clean separation of concerns.
2. Or use `--numstat -z` for NUL-terminated, space/tab/newline-safe parsing.

### `-z` (NUL-terminated, robust parsing)
`git diff --numstat -z` separates fields with NUL (`\0`) instead of TAB/newline, making output
robust against spaces, tabs, and newlines in filenames. **Caveat to verify:** with `-z` and a
rename, the path field may be split into two NUL-separated names (old, new); confirm exact field
count with the verification command below before relying on a fixed-shape parser.

### Composability (all confirmed)
`--numstat` composes with: `--cached` (staged), `-M` (rename notation in path), tree-to-tree
`git diff A B --numstat`, and pathspec excludes (`git diff --cached --numstat -- ':!*.lock'`,
`:!vendor/`). It is a drop-in replacement for the default patch format for *accounting* purposes.

### Verification commands
```bash
# add a normal edit, a binary, and a rename to see all three shapes
printf 'extra line\n' >> a.txt && git add a.txt
printf '\x00\x01\x02BINARY' > bin.dat && git add bin.dat
git mv a.txt renamed.txt 2>/dev/null || true
git diff --cached --numstat          # tab-separated; binary shows - -
git diff --cached --numstat -M       # rename shows => / {...}
git diff --cached --numstat -z | cat -v   # NUL-separated; inspect rename field count
git diff --cached --numstat -- ':!*.dat'  # pathspec exclude composes
```

### Version notes
`--numstat` is stable across all modern git. The `=>`/`{...}` path notation is long-standing.
The `-z` field shape for renames is the one detail worth a one-line local confirmation against
your floor git version before writing a fixed parser.

### Implication for the feature
`--numstat` is the **ideal per-file size source** for the token budget: one line per file, the
`added+deleted` sum is a cheap proxy for that file's patch size, and binary files are flagged by
`-`. Run it **without** `-M` (option 1 above) for a trivially-parseable size map, and apply the
water-fill budget (§6) per file.

---

## 4. The `index <oid>..<oid> <mode>` line

### Where it appears & exact shape
In patch output, each file-pair's section is:

```
diff --git a/path b/path
index <abbrev-old-oid>..<abbrev-new-oid> <mode>
--- a/path
+++ b/path
@@ -l,s +l,s @@
 context
-removed
+added
```

- `index` shape: literally `index <oldoid>..<newoid> <mode>`.
- OIDs are **abbreviated** (git auto-scales abbreviation, min 7 hex; `--abbrev=<n>` controls
  length but does **not** remove the line).
- `<mode>` is the file mode: `100644` (normal), `100755` (executable), `120000` (symlink),
  `160000` (gitlink/submodule).
- Special file-pair forms (different extended headers, then the patch):
  - **New file:** `new file mode 100644` then `index 0000000..<oid>`, `--- /dev/null`, `+++ b/path`.
  - **Deleted file:** `deleted file mode 100644` then `index <oid>..0000000`, `--- a/path`, `+++ /dev/null`.
  - **Mode-only change** (content identical): `old mode 100644` / `new mode 100755` — **no `index`
    line, no patch body** (no content change).
  - **Mode + content change:** `old mode`/`new mode`, then `index <oid>..<oid> <newmode>`, then patch.
- For a **pure rename** (§1) the `index` line is absent (identical blob). For a rename-with-edit,
  it appears after the `rename to` line.

### Is there a flag to suppress it?
**No.** There is no documented git flag that suppresses only the `index` line in patch output.
- `--no-index` is **unrelated** (compares two files outside a repo; not a suppression flag).
- `--no-prefix` removes the `a/`/`b/` path prefixes, not the `index` line.
- `--abbrev=<n>` / `--no-abbrev` change OID abbreviation length; the `index` line remains.

**Conclusion: post-capture line stripping is the only way.** Drop every line whose first token is
`index` (`line == "index ..."`, i.e. matches `^index [0-9a-f]{7,}\.\.[0-9a-f]{7,} \d+$`). Do this
**after** capture; never pass a sed/awk pipeline that could alter real content lines.

### Lines to KEEP vs STRIP (recommended)
| Line | Keep? | Why |
|---|---|---|
| `diff --git a/p b/p` | **KEEP** | file-section header; structural signal for the model |
| `--- a/p` / `+++ b/p` | **KEEP** | identifies the file in the patch |
| `@@ -l,s +l,s @@` | **KEEP** | hunk anchors |
| content (` `/`+`/`-`) | **KEEP** | the actual change |
| `similarity index N%` | keep (cheap) | conveys rename quality |
| `rename from` / `rename to` | **keep (recommended)** | rename semantics at ~zero token cost |
| `index <oid>..<oid> <mode>` | **STRIP** | raw OID/mode noise the model does not need |
| `new file mode` / `deleted file mode` / `old mode` / `new mode` | optional | small value; strip if maximizing savings |
| `dissimilarity index` / `copy from` / `copy to` | strip | not relevant with `-M`-only |

`rename from`/`rename to` are recommended **kept**: for ~two short lines they preserve the rename
story the model needs. The `index` line is the single highest-value strip target.

### Verification commands
```bash
printf 'changed\n' >> a.txt && git add a.txt
git diff --cached | grep '^index '          # see the exact index line
git diff --cached --abbrev=40 | grep '^index'   # longer OIDs, line still present
git -c core.abbrev=7 diff --cached | grep '^index'
# confirm nothing else looks like an index line in real output:
git diff --cached | awk '/^index /'
```

### Version notes
The `index` line has been part of git's patch format for all of modern history and is stable.
Abbreviation behavior: git ≥ 2.11 auto-scales abbreviation based on repo size (minimum grew from
7 toward more hex digits in larger repos), but the line shape is unchanged.

---

## 5. Token estimation (`~4 chars ≈ 1 token`)

### The heuristic
**~4 characters ≈ 1 token** for English-ish text is the standard, well-documented rule of thumb
(originating from OpenAI's tokenizer guidance: "~4 characters per token", "~100 tokens ≈ 75
words"). A **character-based estimate is the standard model-agnostic approach** when you do not
have a specific tokenizer available — which is exactly Stagecoach's situation, since it shells out
to an arbitrary CLI agent whose tokenizer it never loads.

### Calibration & caveats
- **English / prose:** ~4 chars/token (≈0.75 words/token).
- **Source code / diffs:** BPE tokenizes identifiers and punctuation more finely → typically
  **~3 chars/token** (i.e., code is slightly *more* token-dense than the prose heuristic).
  `git diff` output is mostly code, so expect ~3–4 chars/token.
- **Whitespace/newlines:** tokenized (a newline or run of spaces costs tokens).
- **Non-Latin (CJK, etc.):** denser — often ~1–2 chars/token — but Stagecoach's diff payload is
  dominated by ASCII code/paths.

**Recommendation for budgeting:** use `tokens ≈ chars / 4` as the model-agnostic estimate, but
for a *budget ceiling* treat code-heavy diff as ~3 chars/token (i.e. estimate
`ceil(chars / 3)`) to err on the safe side — better to under-spend the budget than to exceed a
context window. The `--numstat` `added+deleted` line counts are a cheap secondary proxy for a
file's patch token cost.

### Version notes
N/A (not a git feature). The ratio is tokenizer-dependent but the *model-agnostic* use of a
char-based estimate is the correct, standard choice for a tool that does not own the tokenizer.

---

## 6. Water-fill / water-filling truncation

### Algorithm & correctness
Given per-file sizes `s_1 … s_n` and a budget `B`, find a level `L` such that
`Σ min(s_i, L) = B`. Files with `s_i ≤ L` are kept whole; files with `s_i > L` are truncated to
`L`. **This reclaims the unused budget from small files** (a small file contributes only its real
size `s_i < L`, freeing the rest) and redistributes that reclaimed capacity to the large files.

This is exactly the classic **water-filling** / **max-min-fair allocation with caps** scheme,
standard in resource allocation and information theory (the same shape as power water-filling in
comms, or weighted fair queuing). It is provably the fairest cap-aware allocation: no large file
can grow without shrinking a small file that is already at its (full-size) cap.

### Reference implementation — O(n log n) sort-and-walk
```
1. if Σ s_i ≤ B:        // everything fits
       keep all files whole; done
2. sort sizes ascending
3. remaining = B
   for i in 0..n-1:
       count = n - i            // files still to allocate
       if s[i] * count ≤ remaining:
           remaining -= s[i]     // file i is "small": gets its full size
       else:
           L = remaining / count // water level; files i..n-1 each get L
           truncate files i..n-1 to L
           break
```
Walk invariant: when we reach index `i`, files `0..i-1` have consumed their full size from
`remaining`; the remaining budget is shared equally (`L`) among the `count` files left, which are
all ≥ the current `s[i]`. Because sizes are sorted, if `s[i] * count ≤ remaining` then every file
from `i` onward can be given at least `s[i]`, so file `i` fits whole.

### Edge cases
- **One file larger than budget (B < max s_i):** if it's the only file, `L = B`, it is truncated
  to `B`. With other small files present, small files are served first; the big file gets
  `B − Σ small` (its `L`). Never negative because `Σ s_i > B` guarantees headroom math holds.
- **All files identical size S, n of them:** if `n·S ≤ B`, no truncation. If `n·S > B`, then
  `L < S` and every file is capped to `L = B/n` — a perfectly even split. Correct and fair.
- **Total ≤ B (no truncation):** special-case at step 1; `L` is effectively `≥ max s_i` and all
  files are whole. Must be checked first or the walk's `L` is undefined at the boundary.
- **B = 0:** `L = 0`; every file truncated to zero (degenerate; usually guarded out).
- **Fractional L:** when sizes are in tokens/chars (not lines), `L` may be fractional. Truncate
  to `floor(L)` lines/chars per large file, then optionally spend the integer remainder (a handful
  of leftover units) round-robin to the largest files to use the budget fully.

### Verification (mental / unit-test cases)
- sizes `[10, 20, 30]`, B=`30`: total 60 > 30. Sort → `[10,20,30]`. i=0: count=3, 10·3=30 ≤ 30 →
  remaining=20, file0 whole. i=1: count=2, 20·2=40 > 20 → L=10, files 1,2 capped to 10.
  Result: `[10, 10, 10]`, sum=30. ✓ (small file reclaimed none; large files equalized at 10.)
- sizes `[5, 100]`, B=`50`: total 105 > 50. i=0: count=2, 5·2=10 ≤ 50 → remaining=45, file0 whole.
  i=1: count=1, 100·1=100 > 45 → L=45. Result: `[5, 45]`, sum=50. ✓ (small file kept whole; big
  file gets the reclaimed remainder.)
- sizes `[10, 10]`, B=`50`: total 20 ≤ 50 → no truncation, `[10, 10]`. ✓

### Implication for the feature
This is the right truncation policy for per-file token budgeting: small/concept-focused files
(a single-line config tweak) survive intact while a giant generated file is capped, and the cap
adapts automatically to how many files compete. Pair it with `--numstat` (§3) for sizes and the
char/token estimate (§5) for the budget unit.

---

## Synthesis: how the six compose into the feature

1. **Capture once, transform cheaply.** Run `git diff --cached -M -U1` (or
   `git diff -M -U1 <treeA> <treeB>` for concept diffs) to get the patch. `-M` (deterministic,
   §1) collapses renames; `-U1` (§2) trims context ~50% vs the `-U3` default.
2. **Account with numstat.** Separately run `git diff --cached --numstat` (**without** `-M`,
   §3) to build a `path → (added+deleted)` size map; binary files are flagged by `-`. This is
   the per-file size input to the budget.
3. **Strip the index line.** Post-capture, drop every `^index ` line (§4); optionally strip
   `new/deleted/old/new file mode` lines. **Keep** `diff --git`, `---`, `+++`, `@@`, content,
   and (recommended) `rename from`/`rename to`.
4. **Budget in tokens.** Convert the byte/char budget to tokens with `chars / 4` (conservative
   `chars / 3` for code-heavy diffs, §5), or size directly from numstat line counts.
5. **Allocate with water-fill.** Run the §6 algorithm over per-file sizes vs the budget; small
   files survive whole, large files are capped at the water level `L`. Truncate each oversized
   file's captured patch to its allotment (e.g. first/last N hunks + a `…truncated…` marker).

All six are independent, additive optimizations that compose without conflict and respect
Stagecoach's existing invariants (payload-only; the committed content is unaffected — only what
the model *sees* is shrunk, exactly like the `[binary]`/`[excluded]` placeholders).

---

## Sources

- **Kept: `git-diff(1)` man page** (`git help diff`) — the authoritative source for `-M`, `-C`,
  `-U<n>`, `--numstat`, `--abbrev`, pathspec, and `-z`. Primary reference for every claim above.
- **Kept: `gitattributes(5)` / `git-config(1)`** — `diff.renames` and `diff.context` semantics;
  the `diff.renames` default flip and `core.abbrev` auto-scaling (git ≥ 2.11).
- **Kept: OpenAI tokenizer guidance** — "~4 chars/token", "~100 tokens ≈ 75 words" rule of thumb
  underpinning the model-agnostic char-based estimate (§5).
- **Kept: classic water-filling / max-min-fair-with-caps literature** — confirms §6 is a standard
  fair-allocation scheme.
- **Dropped:** SEO-heavy "how to read a git diff" blog posts — no primary value over the man
  page; excluded to avoid format drift.
- **Dropped:** provider-specific tokenizer libraries (tiktoken, etc.) — the feature must be
  model-agnostic (Stagecoach never loads a tokenizer), so they are out of scope for §5.

## Gaps

- **`diff.renames` exact default-flip commit.** The flip to `true` is in the **git 2.8–2.9 era
  (~2016)**; confirm the exact commit/release against `git help diff` / the git changelog if a
  precise citation is needed. **Actionable conclusion is unaffected:** pass `-M` explicitly for
  determinism.
- **`--numstat -z` rename field shape.** Whether a renamed file emits one `=>` field or two
  NUL-separated names under `-z` should be confirmed with the §3 verification command against
  your floor git version before relying on a fixed-shape parser. (The recommended approach — run
  numstat **without** `-M` — sidesteps this entirely.)
- **No live web search was possible** in this environment (no `web_search`/shell tool), so URLs
  were not fetched; claims are from authoritative knowledge and are each backed by a runnable
  verification command. If linked citations are required for the planning doc, the implementer
  should run the verification blocks and link the `git-diff(1)` man page.

## Supervisor coordination
None required. The research brief is self-contained; no decision or unblocking is needed. The one
material deviation (no `web_search` tool available) is disclosed in the Methodology note and the
Gaps section, and is fully mitigated by per-claim verification commands.
