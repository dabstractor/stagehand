# S1 Implementation Notes — line-anchor splitDiffSections (Shape B regex slice)

> Scope: P1.M1.T1.S1. Replace the un-anchored `strings.Split(diff, "diff --git ")` in splitDiffSections
> with a `(?m)^diff --git ` line-anchored regex slice (Shape B) so a content-embedded `diff --git ` literal
> no longer fragments a real file into bogus sections (which silently defeats token_limit truncation).
> Verified against live source + arch system_context.md §1-§4 (2026-07-04).

## 1. The bug (internal/git/truncatediff.go:76)

```go
parts := strings.Split(diff, "diff --git ")   // UN-ANCHORED — splits on ANY occurrence
```
A content line `+diff --git a/foo b/foo` (a documented/snapshot/fixture diff) contains the literal
`diff --git ` → treated as a section boundary → one real file fragments into many bogus tiny sections →
the water-fill sizes them tiny → truncates nothing → the full payload silently overflows token_limit
(measured ~7× at token_limit=2000). This is the ONLY un-anchored op in the truncation chain; every
sibling is `(?m)^`-anchored (diffSectionHeaderRe:27, diffSectionPlusPlusRe:33, atAtRe:40).

**One production caller**: applyWaterFillGate (tokengate.go:129), in the TokenLimit>0 branch of all three
diff functions (git.go:883/1360/1533). Default TokenLimit==0 path NEVER calls splitDiffSections (unaffected
— the regression anchor). Markdown files bypass it (immune).

## 2. The fix — Shape B (regex FindAllStringIndex slice) — arch §4 RECOMMENDED + verbatim

Shape B preserves byte-identical round-tripping AND all existing fixtures with zero newline bookkeeping.
Shape A (`"\ndiff --git "` split) is error-prone: it consumes the preceding `\n`, breaking the PREAMBLE
fixture. Use Shape B.

**Add the sibling-style regex (near diffSectionHeaderRe / atAtRe, lines 27-40):**
```go
// diffSectionBoundaryRe matches a real file-section header at a LINE START. (?m)^ anchors at column 0,
// so a content line carrying the literal `diff --git ` (always prefixed with a diff marker +/-/space/\)
// does NOT match. Mirrors the line-anchored siblings diffSectionHeaderRe/atAtRe. Pure; compiled once.
var diffSectionBoundaryRe = regexp.MustCompile(`(?m)^diff --git `)
```

**Rewrite splitDiffSections (verbatim from arch §4):**
```go
func splitDiffSections(diff string) []string {
	if strings.TrimSpace(diff) == "" {
		return nil
	}
	matches := diffSectionBoundaryRe.FindAllStringIndex(diff, -1)
	if len(matches) == 0 {
		return []string{diff} // single non-empty blob, no boundary
	}
	var sections []string
	if first := matches[0][0]; first > 0 {
		if leading := diff[:first]; strings.TrimSpace(leading) != "" {
			sections = append(sections, leading) // PREAMBLE preserved (TrimSpace-non-empty)
		}
	}
	for i, m := range matches {
		start := m[0]
		end := len(diff)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		sections = append(sections, diff[start:end]) // sliced AT header offset → prefix naturally present, NO re-prefix
	}
	return sections
}
```
KEY: Shape-B slices AT the header offset → the `diff --git ` prefix is naturally part of each section →
do NOT re-prefix (the old strings.Split re-prefixed because it consumed the separator; the regex slice
consumes nothing). This is what preserves byte-identical round-tripping.

## 3. Shape B verified byte-identical against ALL 6 existing TestSplitDiffSections fixtures (arch §4)

| Fixture | existing want | Shape-B got | match |
|---|---|---|---|
| `""` | nil | nil | ✅ |
| `"   \n  \t\n"` (ws-only) | nil | nil | ✅ |
| single canonical section `one` | `[]{one}` | `[]{one}` | ✅ |
| 3-file aggregate A/B/C | 3 self-contained sections | 3 (sliced at each header) | ✅ |
| PREAMBLE leading | `["PREAMBLE\n", section]` | same (leading[:first] preserved) | ✅ |
| trailing "extra" | `["...extra"]` (1 section) | same (last slice to len(diff)) | ✅ |

The existing assertions are `len(got)==len(want)` + per-section `got[i]==want[i]` (byte equality). Shape B
passes all unchanged → the rewrite is non-regressing for the existing pin points.

## 4. The 2 NEW pure table cases (the regression net)

Add to the `tests` slice in TestSplitDiffSections (truncatediff_test.go ~line 24, table-driven). Each
asserts len + per-section byte equality (same idiom as the existing cases).

**(i) single real file whose body contains `+diff --git a/foo b/foo`:**
```go
{
	desc: "content-embedded diff --git literal → ONE section (line-anchored, inert)",
	in:   "diff --git a/real.txt b/real.txt\n@@ -0,0 +1,1 @@\n+diff --git a/foo b/foo\n",
	want: []string{"diff --git a/real.txt b/real.txt\n@@ -0,0 +1,1 @@\n+diff --git a/foo b/foo\n"},
}
```
The `+diff --git...` line starts with `+` → `(?m)^diff --git ` does NOT match → 1 section byte-identical
to input. (Today's buggy code returns 2 sections → this case FAILS today, PASSES after the fix.)

**(ii) 2-file aggregate, FIRST file's body embeds `diff --git ` literal:**
```go
{
	desc: "first file body embeds diff --git literal → TWO sections (only real headers are boundaries)",
	in: "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,2 +1,2 @@\n-old\n+diff --git a/embed b/embed\n" +
		"diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -1 +1 @@\n-b\n+b2\n",
	want: []string{
		"diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,2 +1,2 @@\n-old\n+diff --git a/embed b/embed\n",
		"diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -1 +1 @@\n-b\n+b2\n",
	},
}
```
The embedded `+diff --git a/embed b/embed` (content line) is inert; only the 2 real headers (at line
start) are boundaries → 2 sections, each byte-identical to its real-file slice.

## 5. The doc-comment correction (Mode A — rides with the work)

The current splitDiffSections godoc (lines ~59-73) DEFENDS the un-anchored trailing-space split as "the
faithful section boundary that distinguishes the header from a content line that happens to start with
`diff --git`". That defense is now WRONG (it only covers a content line starting with `diff --git`
WITHOUT the trailing space). Rewrite the comment to explain LINE-ANCHORING: every content line is
diff-marker-prefixed (+/-/space/\), so `(?m)^diff --git ` matches only real headers at column 0; the
regex slice consumes nothing so each section is byte-identical to its source slice (no re-prefix).

Also: `regexp` is already imported (used by the sibling regexes) — no new import. The `strings` import
stays (TrimSpace). The function stays PURE (stdlib only). Signature FROZEN.

## 6. Scope discipline (S1 vs S2 / P1.M2)

S1 = the regex + splitDiffSections rewrite + the doc-comment correction + the 2 new pure table cases in
TestSplitDiffSections. That is ALL.
- NOT S1: E2E integration regression (content-embedded literal under a truncating token_limit across the
  3 diff functions, in difftokenlimit_test.go) = P1.M1.T2.S1.
- NOT S1: docs sweep (README/how-it-works/configuration) = P1.M2.T1.S1.
- DOCS: ONLY the internal godoc comment on splitDiffSections (Mode A rides with the work). No external
  doc file (docs/configuration.md's token_limit "always fits" contract describes the intended behavior
  the fix RESTORES — no edit needed; the P1.M2 sweep confirms).
- splitDiffSections signature FROZEN — applyWaterFillGate + the 3 diff functions consume it unchanged.
