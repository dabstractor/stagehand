# Bugfix 001 — Architecture Findings: FR3i Truncation Formatting & Diff-Context Diagnostics

> Research artifact for the FR3d–FR3i diff-payload optimization bugfix delta (plan `007_b33d310438c6`).
> Scope: validate PRD feasibility, pin root causes, and ground the downstream PRP agents. All paths
> relative to `/home/dustin/projects/stagehand/`.

## 1. PRD feasibility verdict

| PRD Issue | Severity | Verdict | Action |
|-----------|----------|---------|--------|
| Issue 1 — truncated sentinel glued to next `diff --git` | **Major** | **CONFIRMED & reproduced** (all 3 diff fns) | FIX (single-point) |
| Issue 2 — `diff_context` out-of-range silently clamped | Minor | Confirmed (clamp at `git.go:689`) | FIX (config-layer diagnostic) |
| Issue 3 — `firstNRunes` rune-safety | Minor | PRD self-corrects: **no bug** (rune-boundary-safe) | NO ACTION |

## 2. Issue 1 — Root cause (THE MUST-FIX)

### 2.1 The defect

`internal/git/truncatediff.go`, function `truncateByWaterFill` (the per-file water-fill truncation
application). When a section's body exceeds its token allotment, the body is replaced with:

```go
// truncatediff.go — the truncation branch (current, BUGGY)
if EstimateTokens(body) > allotment {
    body = firstNRunes(body, allotment*4) + "\n" + truncatedSentinel
}
b.WriteString(headerBlock)
b.WriteString(body)
```

`const truncatedSentinel = "... [truncated]"` (truncatediff.go) has **NO trailing newline**. The loop
then immediately appends the next section, so a truncated NON-LAST section's sentinel is glued to the
next section's `diff --git` header on a single physical line:

```
... [truncated]diff --git a/b.go b/b.go
```

### 2.2 Reproduction (empirically verified in this research)

A scratch test calling `truncateByWaterFill` with two sections both capped under `allotment=10`
produced (confirmed FAIL):

```
"a\n... [truncated]diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ ...\n+new B content line pa\n... [truncated]"
```

The substring `[truncated]diff --git` is present — the bug signature. The last (trailing) section's
sentinel is fine (it ends the output); only INTERMEDIATE truncated sections are malformed.

### 2.3 Why existing tests missed it

Every case in `internal/git/truncatediff_test.go`'s `TestTruncateByWaterFill` truncates only the **last**
(or only) section:
- `item_sentinel_on_its_own_line` asserts `strings.HasSuffix(out, "\n... [truncated]")` — true for a
  trailing section regardless of the bug.
- `item_3file_one_over_budget_truncated_others_byte_identical` truncates only the MIDDLE section (B) but
  asserts via `strings.Contains`/`strings.Count` — both PASS even though B's sentinel is glued to C's
  header (`Contains` only checks the marker exists; `Count` only counts occurrences).

**The gap**: no test truncates a non-last section AND asserts the boundary BETWEEN two sections. The
E2E file `difftokenlimit_test.go` likewise caps a single large file. Multi-section concatenation in
`truncateByWaterFill` was never exercised.

### 2.4 Why it affects all three diff functions

`StagedDiff` / `TreeDiff` / `WorkingTreeDiff` (git.go:737 / 1226 / 1398) all route their
`opts.TokenLimit > 0` branch through ONE shared pure helper:

```
git.go (3 sites)  →  applyWaterFillGate(mdDiffs, nmDiff, skeleton, tokenLimit, promptReserve)  [tokengate.go:121]
                                →  truncateByWaterFill(sections, allotments)                  [truncatediff.go]
                                        ←  THE BUG IS HERE (single point)
```

`applyWaterFillGate` assembles `sections = mdDiffs + splitDiffSections(nmDiff)` and calls
`truncateByWaterFill` once. So a single fix in `truncateByWaterFill` covers the markdown path, the
non-markdown path, and all three diff functions. The PRD confirms: "The fix should be applied once in
`truncateByWaterFill` so all three diff functions and the markdown path are covered by a single change."

### 2.5 The fix

Append a trailing `\n` after the sentinel in the truncation branch:

```go
// FIXED
body = firstNRunes(body, allotment*4) + "\n" + truncatedSentinel + "\n"
```

This restores the line-shape discipline the legacy `token_limit==0` path already enforces (git.go:859 /
1338 / 1511: `if !strings.HasSuffix(fileDiff, "\n") { b.WriteByte('\n') }`).

**Regression-invariant check (must hold after the fix):**
- A truncated LAST section: output now ends with `... [truncated]\n` (was `... [truncated]`). The
  existing `item_sentinel_on_its_own_line` test asserts `HasSuffix(out, "\n... [truncated]")` — this
  BREAKS under the naive fix (extra `\n`). **The fix REQUIRES updating that existing assertion** to
  `HasSuffix(out, "\n... [truncated]\n")` OR to `strings.Contains(out, "\n... [truncated]\n")`. This is
  an EXPECTED fixture delta (system_context §6 invariant 3), not a regression.
- Within-budget sections: byte-identical (unaffected — no sentinel, no extra `\n`).
- Path-miss / pure-rename / zero-allotment: verbatim (unaffected).

**Alternative considered**: "ensure each recomposed section ends with a newline before appending the
next" (normalize at the section boundary rather than the sentinel). This is more uniform but would ALSO
alter the within-budget pass-through byte-identity (adding `\n` to sections that don't end in one). The
sentinel-only `+ "\n"` fix is surgical and preserves every byte-identical guarantee. **Recommend the
sentinel-only fix.**

## 3. Issue 2 — `diff_context` out-of-range (Minor)

### 3.1 Current behavior

`buildDiffArgs` (git.go:689) silently clamps:

```go
ctx := opts.DiffContext
if ctx < 0 || ctx > 3 {
    ctx = 1   // silent clamp, no diagnostic
}
```

The config layer (`DiffContextValue`, config.go:201) returns `*c.DiffContext` or default `1` with **no
range check**. `materialize`/`overlay` (file.go:226 / 340) copy the `*int` through verbatim. A user
setting `diff_context = 5` gets `-U1` with zero feedback.

### 3.2 Two PRD-sanctioned options

(a) **Validate at config materialize/overlay time → return an error.** Cleanest, surfaces bad config
early. BUT `materialize` returns `*Config` (no error); the validation must land in the `load.go` error
path (which DOES return errors) or a new `Config.Validate() error` called from load. More plumbing.

(b) **Emit a one-line stderr warning when clamping fires.** Lower-risk, but `buildDiffArgs` is a PURE
function ("Pure function; no I/O." per its doc comment) and is called per-diff-invocation, so a warning
there would (i) break purity and (ii) spam on every commit. The warning must instead be emitted ONCE
upstream — e.g. in the call sites that build `StagedDiffOptions`, or in `DiffContextValue`'s caller.

### 3.3 Recommended approach

Given Issue 2 is PRD-marked "Low priority," prefer **option (a) at the config layer** as a single
`Config.Validate() error` (or inline range check in the load path returning the existing error), which
gives a clear, early, testable error and avoids I/O-in-pure-function. The `*int` 0-vs-unset semantics
MUST be preserved: `nil` ⇒ unset (default 1), `*0` ⇒ valid (-U0), only values `<0` or `>3` are rejected.
Document the valid range `0–3` in `docs/configuration.md` and the bootstrap template comment.

If the error-return plumbing proves invasive, fall back to (b) a one-time stderr warning at the
`StagedDiffOptions` construction call sites (NOT inside `buildDiffArgs`).

## 4. Data flow & key locations (authoritative)

```
config.toml diff_context (*int, 0-vs-unset), token_limit (int, 0=unset)
   → config.Config.DiffContextValue()  [config.go:201]  → plain int
   → 6 call sites: cfg → git.StagedDiffOptions{TokenLimit, DiffContext, PromptReserveTokens, ...}
   → StagedDiff / TreeDiff / WorkingTreeDiff (git.go:737 / 1226 / 1398), ALL share StagedDiffOptions (git.go:37)
        if opts.TokenLimit > 0:
            b.WriteString(applyWaterFillGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens))
                                                  ↓  [tokengate.go:121]
            sections = mdDiffs + splitDiffSections(nmDiff)
            sizes[i] = EstimateTokens(sectionBody(sections[i]))
            allocs = allocByWaterFill(sizes, bodyBudget)        [waterfill.go]
            allotments[path] = allocs[i]   (path via diffSectionPath)
            return truncateByWaterFill(sections, allotments)    [truncatediff.go]  ← BUG
        else (token_limit==0):
            legacy per-file line cap + byte cap + "... [diff truncated at N ...]" sentinels
            (git.go:859/1338/1511 — HasSuffix newline guard, the regression anchor)
```

### 4.1 Exact file/line references

| Concern | File | Line |
|---------|------|------|
| **BUG (truncateByWaterFill truncation branch)** | `internal/git/truncatediff.go` | `firstNRunes(...) + "\n" + truncatedSentinel` (last write in the loop) |
| `truncatedSentinel` const | `internal/git/truncatediff.go` | `"... [truncated]"` |
| `applyWaterFillGate` | `internal/git/tokengate.go` | 121 |
| `StagedDiffOptions` struct (shared by all 3 fns) | `internal/git/git.go` | 37 |
| TokenLimit>0 call sites | `internal/git/git.go` | 883 / 1360 / 1533 |
| Legacy newline guard (regression anchor) | `internal/git/git.go` | 859 / 1338 / 1511 |
| `buildDiffArgs` (Issue 2 clamp) | `internal/git/git.go` | 689 |
| `DiffContextValue` (config resolve) | `internal/config/config.go` | 201 |
| Config `*int` overlay | `internal/config/file.go` | 226 / 340 |

## 5. Test conventions to follow (grounded in codebase)

- **Pure unit tests** (`truncatediff_test.go`, `tokengate_test.go`): table-driven + `t.Run` subtests,
  string-literal inputs, NO `t.TempDir`, NO I/O, NO testify. Canonical section shape:
  `"diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n"`. Shared helpers `itoa`, `tail`.
- **E2E integration tests** (`difftokenlimit_test.go`): `repo := t.TempDir()` → `initRepo(t, repo)` →
  `writeFile`/`stageFile` → `New(repo)` → `g.StagedDiff/TreeDiff/WorkingTreeDiff(ctx, opts)`. Assert via
  `strings.Contains`/`strings.Count`/`EstimateTokens(out)` bounds. NO testify.
- Helpers (same-package, `t.Helper()`): `initRepo` (git_test.go:13), `writeFile` (committree_test.go:31),
  `stageFile` (committree_test.go:39), `writeTreeOf` (committree_test.go:48), `headSHA`
  (committree_test.go:60), `commitAllowEmpty` (difftokenlimit_test.go:29), `sdManyLines`
  (stagediff_test.go:23), `tail` (truncatediff_test.go:489).
- Existing assertion that the fix WILL break (expected delta): `item_sentinel_on_its_own_line` in
  `truncatediff_test.go` asserts `HasSuffix(out, "\n... [truncated]")` — must update to account for the
  trailing `\n`.

## 6. Documentation surface

- **Issue 1**: changes internal output line-shape (adds `\n` after sentinel). No user-facing config/API
  surface change. `docs/how-it-works.md` mentions the `... [truncated]` marker (line 144) but does NOT
  specify line shape — no strict doc update required for Issue 1 alone. Verify in the Mode B sweep.
- **Issue 2**: if validation/warning is added, `docs/configuration.md` §diff_context (lines 107, 131,
  147) and the bootstrap template comment (`bootstrap.go:291`) should note valid range `0–3` and the
  out-of-range behavior — Mode A (rides with the implementing subtask).
