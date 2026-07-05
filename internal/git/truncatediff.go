// FR3i per-file water-fill TRUNCATION APPLICATION (PRD §9.1 FR3i; architecture/git_diff_semantics.md
// §6; system_context.md §6 invariant 2).
//
// Three pure, git-independent functions that apply a per-file token allotment (built by the M4.T3
// token-limit gate from allocByWaterFill (S1) + EstimateTokens(body) sizes, keyed by numstat
// destination path) to a list of per-file diff sections captured upstream:
//
//   - splitDiffSections splits the captured non-markdown aggregate on `diff --git ` boundaries
//     (FR3i: "split on `diff --git` boundaries to apply the per-file level without extra git
//     invocations").
//   - diffSectionPath extracts the destination (b/) path so a section can be matched against the
//     allotments map (the key is numstat's destination path, resolveNumstatPath — they must AGREE).
//   - truncateByWaterFill applies the per-file level: for each section whose BODY (the first `@@`
//     onward) exceeds its allotment, the body is cut to its first allotment×4 runes (allotment
//     tokens under the chars/4 estimator) + the SHORTER `... [truncated]` sentinel; the header
//     block (diff --git/extended/---/+++) is always preserved; within-budget sections pass through
//     byte-identical (NO sentinel); sections are recomposed in input order.
//
// PURE: stdlib `regexp` + `strings` only (no context, no I/O, no git). The `index` line is already
// stripped upstream (FR3h, stripIndexLines at capture); do NOT re-strip. S2 calls ONLY EstimateTokens
// (in-package) for the "needs truncation?" check; it does NOT call the solver (allocByWaterFill/
// waterFillLevel — the gate does) or numstatRows (the gate builds the path keying). The three public
// signatures are FROZEN — consumed by M4.T3 (P1.M4.T3.S1, the token-limit gate). The same
// truncateByWaterFill serves the markdown per-file sections (each is a self-contained `diff --git`
// section) — uniform handling per the item_description.

package git

import (
	"regexp"
	"strings"
)

// diffSectionHeaderRe matches a section's leading `diff --git a/<src> b/<dst>` line and captures the
// DESTINATION path <dst> (group 2). (?m)^ anchors at line start. For a rename <dst> is the NEW path —
// matching resolveNumstatPath's destination resolution (numstat.go), so the path agrees with the
// allotments map key. Pure; compiled once.
var diffSectionHeaderRe = regexp.MustCompile(`(?m)^diff --git a/(.*) b/(.*)$`)

// diffSectionPlusPlusRe is the FALLBACK destination extractor: `+++ b/<dst>` (group 1). Reached only
// when the `diff --git` header is absent/malformed. A deletion's `+++ /dev/null` does not match `b/`.
// Pure.
var diffSectionPlusPlusRe = regexp.MustCompile(`(?m)^\+\+\+ b/(.*)$`)

// atAtRe matches a hunk header line `@@ -l,s +l,s @@ …` (FR3h keep-table: a hunk anchor). Used to split
// a section into its header block (lines before the first @@) and body (the first @@ onward). (?m)^@@
// anchors at line start; a content line carrying a leading @@ (vanishingly rare, and would need a
// `/`+`/`-` marker) does not match because content lines start with a diff marker. Pure.
var atAtRe = regexp.MustCompile(`(?m)^@@`)

// diffSectionBoundaryRe matches a real file-section header at a LINE START. (?m)^ anchors at
// column 0, so a content line carrying the literal `diff --git ` (always prefixed with a diff
// marker +/-/space/\) does NOT match. Mirrors the line-anchored siblings
// diffSectionHeaderRe/atAtRe. Pure; compiled once.
var diffSectionBoundaryRe = regexp.MustCompile(`(?m)^diff --git `)

// truncatedSentinel is the FR3i per-file truncation sentinel — the SHORTER `... [truncated]` form
// (system_context.md §6 invariant 2: "The `... [truncated]` sentinel (shorter form, per PRD FR3i) is
// emitted per truncated file; the `at N bytes` sentinels do NOT appear"). DISTINCT from the legacy
// aggregate sentinels (`... [diff truncated at N bytes/lines]`, git.go L840/L868) which S2 must NEVER
// emit. Appended on its OWN line (leading `\n`), matching the legacy sentinels' line shape, ONLY when
// content was actually removed.
const truncatedSentinel = "... [truncated]"

// splitDiffSections splits the captured non-markdown aggregate diff on `diff --git ` boundaries (PRD
// §9.1 FR3i: "the aggregate non-markdown diff is split on `diff --git` boundaries to apply the
// per-file level without extra git invocations"). Each returned section is self-contained — its
// first line is `diff --git a/<p> b/<p>` — because sections are SLICED AT the header byte offset
// returned by diffSectionBoundaryRe.FindAllStringIndex: the regex consumes NOTHING, so the
// `diff --git ` prefix is NATURALLY present at the start of each section (NO re-prefixing, NO
// newline bookkeeping — byte-identical split→join round-trip).
//
// Line-anchoring: diffSectionBoundaryRe is `(?m)^diff --git ` — it matches ONLY at column 0
// (a line start). Every diff CONTENT line is prefixed with a diff marker (`+`/`-`/` `/`\`), so a
// content line carrying the literal `diff --git ` (e.g. an added fixture/snapshot/doc line
// `+diff --git a/foo b/foo`) does NOT match — it is INERT. Only real file-section headers at a
// line start are boundaries, so each real file is exactly one section (the FR3d contract: each
// real file is sized and truncated as a unit, restoring "the payload always fits").
//
// Leading element: text before the first `diff --git` is dropped if empty (TrimSpace), preserved
// if non-empty (defensive — should not occur for a clean non-md aggregate, which always starts
// with `diff --git`). Empty/whitespace-only input → nil. A non-empty input with NO `diff --git `
// header line → a single verbatim element (safe default; should not occur for a clean aggregate).
// The input bytes are otherwise preserved verbatim (the trailing "\n" of the last section is NOT
// stripped — only the leading element is trimmed when deciding whether to drop it) so a round-trip
// split → join is byte-identical for a clean aggregate.
//
// PURE: string manipulation only (stdlib `regexp` + `strings`); no git, no I/O, no context. The
// input is the ALREADY-captured, ALREADY FR3h-index-stripped non-markdown aggregate (the
// `index <oid>..<oid> <mode>` line is gone upstream — do NOT re-strip). Signature FROZEN —
// consumed by M4.T3 (the token-limit gate).
func splitDiffSections(diff string) []string {
	// Whitespace-only input (incl. "") → nil. We do NOT TrimSpace the whole input: that would
	// strip the trailing "\n" of the last section (legitimate git output), breaking byte-identical
	// round-tripping. Only the leading element (text before the first `diff --git`) is trimmed when
	// deciding whether to drop it as empty.
	if strings.TrimSpace(diff) == "" {
		return nil
	}
	// Find every real file-section header at a line start (column 0). A content line carrying the
	// literal `diff --git ` is prefixed with a diff marker, so (?m)^diff --git does NOT match it.
	matches := diffSectionBoundaryRe.FindAllStringIndex(diff, -1)
	if len(matches) == 0 {
		// No section boundary (no line starts with `diff --git `) → the non-empty input is a single
		// blob. Preserve verbatim (should not occur for a clean non-md aggregate, which always starts
		// with `diff --git`).
		return []string{diff}
	}
	var sections []string
	// Leading content before the first boundary: drop if empty (TrimSpace), preserve if non-empty
	// (defensive — a stray placeholder/comment would be preserved, not lost).
	if first := matches[0][0]; first > 0 {
		if leading := diff[:first]; strings.TrimSpace(leading) != "" {
			sections = append(sections, leading)
		}
	}
	// Each section is sliced AT its header offset, so the `diff --git ` prefix is naturally present
	// (NO re-prefixing — the regex consumes nothing). Byte-identical split→join round-trip.
	for i, m := range matches {
		start := m[0]
		end := len(diff)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		sections = append(sections, diff[start:end])
	}
	return sections
}

// diffSectionPath extracts the DESTINATION (b/) path from a section so it can be matched against the
// allotments map (keyed by numstat destination path — resolveNumstatPath, numstat.go). Preference:
//  1. the `diff --git a/<src> b/<dst>` line → <dst> (group 2);
//  2. fallback `+++ b/<dst>` (group 1).
//
// One surrounding pair of `"` is stripped (basic mitigation for git-quoted paths with spaces/special
// chars — full core.quotePath unquoting is out of scope; a residual mismatch degrades to safe
// pass-through in truncateByWaterFill). ok is false when neither line matches (e.g. a non-diff string,
// or a binary placeholder line that leaked in). PURE.
//
// For a deletion, `+++ /dev/null` does not match `b/`, but `diff --git a/x b/x` already supplied the
// path (case 1). For a rename, <dst> is the NEW path — agreeing with resolveNumstatPath's destination.
func diffSectionPath(section string) (path string, ok bool) {
	if m := diffSectionHeaderRe.FindStringSubmatch(section); m != nil {
		return stripOneQuotePair(m[2]), true
	}
	if m := diffSectionPlusPlusRe.FindStringSubmatch(section); m != nil {
		return stripOneQuotePair(m[1]), true
	}
	return "", false
}

// stripOneQuotePair removes a single leading+trailing `"` pair from s if both are present (basic
// mitigation for git-quoted diff paths). Pure helper for diffSectionPath.
func stripOneQuotePair(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// firstNRunes returns s's first n runes, rune-boundary-safe, WITHOUT allocating a full []rune (it
// iterates byte offsets via `for i := range s`, which UTF-8-decodes and yields each rune's start byte,
// stopping at the n-th rune's start). n <= 0 → "". Fewer than n runes → s whole. Used by
// truncateByWaterFill to cut a body to allotment×4 runes (allotment tokens under the chars/4
// estimator; the faithful inverse of EstimateTokens = ceil(runes/4)). PURE.
func firstNRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s { // i = byte offset of each rune start (utf8-decoded by the range clause)
		if count == n {
			return s[:i]
		}
		count++
	}
	return s // fewer than n runes
}

// truncateByWaterFill applies per-file token allotments (PRD §9.1 FR3i; git_diff_semantics.md §6) to
// a list of per-file diff sections and returns the recomposed diff body under token_limit. For each
// section (in INPUT order):
//
//  1. extract its destination path (diffSectionPath); if not found OR absent from allotments → pass
//     through verbatim (path-miss ⇒ safe over-include; never a wrong truncation). Robust to
//     git-emission-order vs numstat-sorted-by-path drift — the map is keyed by PATH, not by index.
//  2. split the section into a HEADER BLOCK (the `diff --git`/extended-header/`---`/`+++` lines —
//     everything before the first `@@` line) and a BODY (the first `@@` onward). A section with no
//     `@@` (pure rename / mode-only) has an empty body ⇒ no truncation (pass through verbatim).
//  3. if EstimateTokens(body) > allotment: replace the body with its first allotment×4 RUNES
//     (allotment tokens under the chars/4 estimator; rune-boundary-safe) + "\n" + truncatedSentinel
//     (the SHORTER `... [truncated]` form, on its own line, ONLY when content was removed). Else:
//     body unchanged (within budget ⇒ byte-identical, NO sentinel).
//  4. append headerBlock + (possibly truncated) body.
//
// Sections are recomposed in ORIGINAL INPUT ORDER (NOT sorted — FR3i: "Recompose the sections in
// original order"). The byte-identical pass-through guarantee relies on (a) within-budget sections
// returned verbatim AND (b) join order == input order. The same function serves the markdown per-file
// sections (each is a self-contained `diff --git` section) — uniform handling per the item_description.
// allotments is a map[string]int (numstat destination path → token allotment), built by the M4.T3 gate
// from allocByWaterFill (S1) + EstimateTokens(body) sizes.
//
// PURE: no git, no I/O, no context. Calls ONLY EstimateTokens (in-package). The `index` line is
// already stripped upstream (FR3h); do NOT re-strip. Signature FROZEN — consumed by M4.T3
// (P1.M4.T3.S1, the token-limit gate). Empty sections → "".
func truncateByWaterFill(sections []string, allotments map[string]int) string {
	if len(sections) == 0 {
		return ""
	}
	var b strings.Builder
	for _, section := range sections {
		path, ok := diffSectionPath(section)
		allotment, found := 0, false
		if ok {
			allotment, found = allotments[path]
		}
		// Path-miss (no diff --git/+++ b/, OR path absent from allotments, OR non-positive allotment):
		// pass through verbatim — safe over-include, never a wrong truncation.
		if !ok || !found || allotment <= 0 {
			b.WriteString(section)
			continue
		}
		// Split into header block (before first @@) + body (first @@ onward).
		loc := atAtRe.FindStringIndex(section)
		if loc == nil {
			// No hunk (pure rename / mode-only) → no body to truncate → pass through.
			b.WriteString(section)
			continue
		}
		headerBlock := section[:loc[0]]
		body := section[loc[0]:]
		if EstimateTokens(body) > allotment {
			// allotment tokens ⟺ allotment×4 runes. Rune-boundary slice + sentinel on its own line,
			// with a trailing "\n" so the next section's `diff --git` begins at a line start (FR3i).
			body = firstNRunes(body, allotment*4) + "\n" + truncatedSentinel + "\n"
		}
		b.WriteString(headerBlock)
		b.WriteString(body)
	}
	return b.String()
}
