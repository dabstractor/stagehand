---
name: "P1.M2.T3.S1 — FR3h index-line stripping: post-capture '^index ' filter across the 3 diff functions"
description: |
  Implement PRD §9.1 **FR3h**: strip git's per-file `index <oid>..<oid> <mode>` header line from every
  captured diff body. The blob OIDs are useless to the model and cost ~30 bytes/file. No git flag
  suppresses the index line (`architecture/git_diff_semantics.md` §4 — `--no-index` is unrelated,
  `--no-prefix`/`--abbrev` don't remove it), so **post-capture line stripping is the only way.**

  ⚠️ **THE central design call — a POST-CAPTURE string transform, orthogonal to the parallel P1.M2.T2.S1.**
  T2.S1 injects `-M`/`-U<ctx>` on the **ARGV (pre-capture)** to shape what git EMITS. THIS task (T3.S1)
  shapes the **captured STRING (post-capture)** — it runs `stripIndexLines` on the bytes git already
  returned. The two compose (T2.S1 → git emits a compact patch → T3.S1 drops the index line → cap). Do NOT
  touch `buildDiffArgs`, the 9 argv sites, or `-M`/`-U` — that is T2.S1's scope. This task is a pure helper
  + 6 one-line insertions.

  ⚠️ **THE regex — `^index [0-9a-f]+\.\.[0-9a-f]+ ` (anchored; OID-form-disambiguated).** Only a line
  matching this is dropped. A content line that merely starts with the word "index" — a code comment
  `// index of items` or a bare `index of items` — does NOT match (it lacks the `<hex>..<hex> ` form; `of`
  is not hex) and is **preserved verbatim**. In a real diff body every content line also carries a leading
  `+`/`-`/space marker, so no content line can start with `index ` at all — the markers are a second
  protection layer; the regex is belt-and-suspenders. `diff --git`/`---`/`+++`/`@@`/`similarity index N%`/
  `rename from`/`rename to` all start with a different token → all KEPT. Implemented as a package-level
  compiled `regexp` + split/drop/join. See research §2.

  ⚠️ **THE ordering — capture → stripIndexLines → cap, at all 6 sites.** Each of the 3 diff functions
  (`StagedDiff`/`TreeDiff`/`WorkingTreeDiff`) has a markdown per-file loop (capture → line cap) AND a
  non-markdown aggregate (capture → byte cap) = **2 sites × 3 functions = 6 insertions**. At each, insert
  ONE line (`fileDiff = stripIndexLines(fileDiff)` / `nmDiff = stripIndexLines(nmDiff)`) **immediately
  after capture (after the exit-code check) and BEFORE the existing byte/line cap** — so the cap measures
  the stripped size (the whole point of FR3h: save the ~30 bytes/file against the cap). The cap code itself
  is untouched. See research §3.

  ⚠️ **THE parity — FR3c: all 3 diff paths, BOTH bodies.** The contract: "Apply to BOTH captured bodies
  (the markdown per-file diff AND the non-markdown aggregate diff), in all 3 functions (FR3c parity)."
  All 6 sites get the same one-liner. Placeholders (`[binary]`/`[excluded]`) are NEVER passed through the
  helper (they're synthesized directly to the `strings.Builder`) → the contract's "Do NOT strip inside
  placeholder lines" is satisfied by construction; no special-casing.

  ⚠️ **THE fixture reality — NO churn expected (verified).** Grepped every `internal/git/*_test.go` for
  `index [0-9a-f]`: **zero matches**. Every existing assertion is substring/structural (`Contains`,
  `Count("diff --git a/<file>")`, truncation sentinels, `[binary]`/`[excluded]` placeholders, file
  presence) — none assert on the index line. So FR3h breaks ZERO existing fixtures. The contract's
  "update golden fixtures that currently expect an index line" is a **run-driven no-op**: RUN the suites,
  fix only what breaks (nothing should). See research §5.

  ⚠️ **THE new import — `"regexp"` is NOT currently imported in git.go (stdlib; no go.mod change).** git.go
  imports `bytes/context/errors/fmt/io/os/exec/path/filepath/sort/strconv/strings`. ADD `"regexp"`. It is
  the only import change; regexp is stdlib so `go.mod`/`go.sum` are byte-unchanged.

  Deliverable: edits to `internal/git/git.go` (add `regexp` import + `indexLineRe` var + `stripIndexLines`
  func + 6 one-line insertions) + edits to `internal/git/stagediff_test.go` (2 new tests). NO new files,
  NO go.mod change, NO docs, NO argv/buildDiffArgs/binary.go/config/call-site changes. OUTPUT: index lines
  gone from every captured payload; ~30 bytes/file saved against the cap; every kept line intact.
---

## Goal

**Feature Goal**: Remove git's per-file `index <oid>..<oid> <mode>` header line from every captured diff
body (markdown per-file AND non-markdown aggregate, in all 3 diff functions) via a surgical post-capture
`^index [0-9a-f]+\.\.[0-9a-f]+ ` line filter applied BEFORE the byte/line cap — saving ~30 bytes/file
against the cap and dropping blob-OID/mode noise the model does not need, while preserving every other
line (`diff --git`/`---`/`+++`/`@@`/content/`similarity index`/`rename from`/`rename to`).

**Deliverable** (edits to existing files):
1. **`internal/git/git.go`** — add `"regexp"` to the import block; add a package-level
   `var indexLineRe = regexp.MustCompile(`^index [0-9a-f]+\.\.[0-9a-f]+ `)`; add
   `func stripIndexLines(s string) string` (split on `\n`, drop `indexLineRe` matches, rejoin; fast-path
   on `!strings.Contains(s, "index ")`); insert ONE `stripIndexLines` call at each of the 6 capture→cap
  sites (markdown `fileDiff` + non-markdown `nmDiff`, in StagedDiff/TreeDiff/WorkingTreeDiff), after the
   exit-code check and before the cap.
2. **`internal/git/stagediff_test.go`** — add `TestStripIndexLines` (table-driven unit test of the helper)
   and `TestStagedDiff_IndexLineStripped` (integration test through `StagedDiff`).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/git/` clean;
`go test -race ./internal/git/` green — the 2 new tests pass AND every existing golden test stays green
(no fixture references an index line, so no churn expected). `go test -race ./...` green (the agent payload
now lacks index lines; generate/decompose/hook tests are substring/output-shape — fix any break).
Concretely: a captured diff never contains a line matching `^index [0-9a-f]+\.\.[0-9a-f]+ `; a content line
starting with "index" (no OID form) is preserved; the cap measures the stripped size. go.mod/go.sum
unchanged; only `internal/git/git.go` + `internal/git/stagediff_test.go` touched.

## User Persona

**Target User**: The agent (model) consuming the diff payload — FR3h cuts ~30 bytes/file of pure noise
(blob OIDs + mode) that the model cannot use. Transitively PRD §9.1 FR3h (P0 diff-capture quality).
Also the NEXT subtasks (M3 numstat skeleton, M4 water-fill) which measure/truncate a now-index-free body.

**Use Case**: A user stages changes across N files and runs stagecoach. Each file's diff section currently
includes a useless `index 600d48a..62b056e 100644` line; after FR3h those lines are gone, the payload is
smaller and cleaner, and the byte/line cap budget goes further (the ~30 bytes/file are reclaimed).

**User Journey**: `StagedDiff`/`TreeDiff`/`WorkingTreeDiff` → `g.run` captures the patch →
`stripIndexLines(body)` drops each `index <oid>..<oid> <mode>` line → the byte/line cap measures the
stripped body → (prompt construction) → agent sees file identity (`diff --git`/`---`/`+++`) + hunks (`@@`)
+ content, with no OID noise.

**Pain Points Addressed**: removes the index-line token bloat (~30 bytes/file × N files) and lets the cap
budget cover more real content. Internal transform; no user-visible behavior change beyond a leaner payload.

## Why

- **FR3h is P0 (§9.1) and ALWAYS ON.** Like FR3e/FR3f (system_context §6 invariant 1), index-line stripping
  is unconditional — not gated on `token_limit`. It runs at every capture, even at `token_limit==0`.
- **Post-capture is the ONLY way.** No git flag suppresses the index line (`git_diff_semantics.md` §4).
  Stripping the captured string is the sole mechanism.
- **Pre-cap = the whole point.** Stripping BEFORE the byte/line cap means the cap measures the stripped
  size — the ~30 bytes/file are saved against the budget, not wasted on OID noise.
- **Surgical + safe.** The `^index <hex>..<hex> ` regex matches ONLY the index header; content lines (which
  also carry `+`/`-`/space markers in a diff body) cannot match. No real content is ever altered.
- **Composes cleanly with the parallel T2.S1.** T2.S1 shapes the argv (`-M`/`-U`); T3.S1 shapes the captured
  string. They are independent edits in the same 3 functions and do not conflict.
- **No new surface.** DOCS: none (P1.M5 owns the diff-capture doc sync); no option/config/CLI/argv change.

## What

A `stripIndexLines` helper + a package-level compiled regex in `internal/git/git.go`, applied at 6 sites
(2 per function × 3 functions), each capture→strip→cap. Two new tests. No argv/buildDiffArgs/binary.go/
config/call-site/doc changes.

### Success Criteria

- [ ] `internal/git/git.go` imports `"regexp"` (added); no other import change; go.mod/go.sum unchanged.
- [ ] `var indexLineRe = regexp.MustCompile(`^index [0-9a-f]+\.\.[0-9a-f]+ `)` is package-level (compiled once).
- [ ] `func stripIndexLines(s string) string`: fast-path `if !strings.Contains(s, "index ") { return s }`;
      else split on `\n`, drop lines where `indexLineRe.MatchString(line)`, rejoin with `\n`.
- [ ] 6 insertions, each ONE line placed AFTER the capture's exit-code check and BEFORE the existing cap:
      - StagedDiff: `fileDiff = stripIndexLines(fileDiff)` (md loop) + `nmDiff = stripIndexLines(nmDiff)` (nm section).
      - TreeDiff: same two insertions.
      - WorkingTreeDiff: same two insertions.
- [ ] A captured diff contains NO line matching `indexLineRe` (FR3h); `diff --git`/`---`/`+++`/`@@`/
      `similarity index`/`rename from`/`rename to`/content lines are all retained.
- [ ] A content line starting with "index" but lacking the `<hex>..<hex> ` form (e.g. `index of items`) is
      preserved (TestStripIndexLines negative case).
- [ ] `TestStripIndexLines` (unit, table-driven) + `TestStagedDiff_IndexLineStripped` (integration) added;
      all pass.
- [ ] Existing golden suites green (no fixture references an index line ⇒ no churn expected).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/git/`, `go test -race ./internal/git/`,
      `go test -race ./...` clean/green; go.mod/go.sum byte-unchanged; only `git.go` + `stagediff_test.go` touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact regex + helper
(quoted), the 6 pinned capture→cap sites (with before/after), the test sketches (unit + integration), and
the scope fences (no argv/buildDiffArgs/binary.go/config). No PRD/git-internals knowledge beyond "git's
patch format has an `index <oid>..<oid> <mode>` line per file that we drop."

### Documentation & References

```yaml
# MUST READ — the authoritative research
- docfile: plan/007_b33d310438c6/P1M2T3S1/research/index-line-stripping.md
  why: the index-line shape + why post-capture (§1), the regex + safety analysis (§2), the 6 pinned
       capture→cap sites (§3), the unit + integration test design (§4), the fixture reality (§5), scope
       fences (§6).
  critical: §2 (the regex is OID-form-disambiguated; a content line starting with "index" is kept) and
       §3 (capture → strip → cap ORDER; 6 sites) are the things most likely to be done wrong.

# The git semantics (verified, §4 is the index-line spec)
- docfile: plan/007_b33d310438c6/architecture/git_diff_semantics.md
  section: "4. The `index <oid>..<oid> <mode>` line"
  why: the exact line shape; proof NO git flag suppresses it (`--no-index` unrelated, `--no-prefix`/`--abbrev`
       don't remove it) → post-capture stripping is the only way; the KEEP-vs-STRIP table (strip ONLY the
       index line; keep diff --git / --- / +++ / @@ / similarity index / rename from / rename to).
  critical: a pure rename (after -M) has NO index line (identical blob) → the strip is a no-op there; a
       mode-only change has no index line either. The strip never removes anything but the index header.

# The regression invariant (always-on; fixture anchors)
- docfile: plan/007_b33d310438c6/architecture/system_context.md
  section: "6. Regression invariants" (FR3e/FR3f/FR3h ALWAYS ON, not token-gated; boundary-marker +
       truncation-sentinel fixtures are the stability anchors).
  why: FR3h is unconditional (do NOT gate on opts.TokenLimit). The boundary markers (`diff --git a/<file>`)
       and truncation sentinels that existing fixtures assert on are RETAINED by the strip.
  critical: the strip MUST be unconditional (M4 owns the token-limit gate; this task runs always).

# The parallel task (assume LANDED — do NOT duplicate)
- docfile: plan/007_b33d310438c6/P1M2T2S1/PRP.md
  why: T2.S1 extends `buildDiffArgs(opts, domain…) []string` to emit `-M`/`-U<ctx>` at the ARGV sites
       (pre-capture). THIS task (T3.S1) is a POST-CAPTURE string transform — it does NOT touch buildDiffArgs
       or the 9 argv sites. The two compose: T2.S1 → git emits → T3.S1 strips → cap.
  critical: do NOT edit buildDiffArgs, the 9 argv call sites, or -M/-U. Those are T2.S1's scope (landed).

# The file being edited
- file: internal/git/git.go
  section: imports (L3-14 — ADD "regexp"); buildDiffArgs (L688 — place stripIndexLines + indexLineRe
       nearby, the shared-diff-helper region); the 3 diff functions' capture→cap sites:
       StagedDiff md-loop fileDiff (capture L726 / line cap L734) + nmDiff (capture L805 / byte cap L814);
       TreeDiff md-loop (capture L1177 / line cap L1184) + nmDiff (capture L1245 / byte cap L1252);
       WorkingTreeDiff md-loop (capture L1312 / line cap L1319) + nmDiff (capture L1381 / byte cap L1388).
  why: the import block you add to; the 6 capture→cap gaps where stripIndexLines is inserted. Each gap is
       between the `if fcode/nmcode != 0 { … }` check and the `if lines := …` / `if len(nmDiff) > …` cap.
  pattern: at each md site `fileDiff = stripIndexLines(fileDiff)` then the existing line cap; at each nm
       site `nmDiff = stripIndexLines(nmDiff)` then the existing byte cap. One line per site; cap code untouched.
  gotcha: re-confirm line numbers at edit time (the parallel T2.S1's argv edits sit just above each capture
       and don't move the capture/cap lines, but verify). Insert AFTER the exit-code check, BEFORE the cap.

# The placeholder emitters (NOT touched — proof placeholders never hit the helper)
- file: internal/git/git.go
  section: binaryPlaceholderLine / excludedPlaceholderLine + the StagedDiff/TreeDiff/WorkingTreeDiff loops
       that `b.WriteString(binaryPlaceholderLine(...))` / `b.WriteString(excludedPlaceholderLine(...))`.
  why: PROOF the `[binary]`/`[excluded]` placeholder lines are synthesized directly to the strings.Builder
       and NEVER pass through stripIndexLines (which only runs on the captured `fileDiff`/`nmDiff`). So the
       contract's "Do NOT strip inside placeholder lines" is satisfied by construction.
  critical: do NOT route placeholders through stripIndexLines or add any placeholder special-case.

# The binary detection (NOT touched — numstat/name-status, no patch body)
- file: internal/git/binary.go
  section: detectBinaryFiles / fileStatuses (numstat / --name-status argv).
  why: PROOF binary.go builds its own argv and emits no patch body (no index line). Not routed through the
       strip. Untouched.
  gotcha: do NOT add stripIndexLines to binary.go or its call sites.

# The tests (the pattern for the new ones + the helpers to reuse)
- file: internal/git/stagediff_test.go
  section: the existing substring golden tests (TestStagedDiff_MarkdownAndCode, _MarkdownLineCap,
       _NonMarkdownByteCap, _BinaryFilePlaceholderAndExcluded, …) + the shared helpers.
  why: the new tests mirror this style. The shared helpers (initRepo/writeFile/stageFile/execGit) live across
       the test files (git_test.go/committree_test.go/revparsetree_test.go); `New(repo)` is the constructor.
  pattern: white-box `package git`; reuse initRepo(t,repo)/writeFile(t,repo,name,body)/stageFile(t,repo,name)/
       execGit(t,repo,args…)/New(repo). For the integration test, `StagedDiff(ctx, StagedDiffOptions{DiffContext:1})`.
  gotcha: reuse the package-level `indexLineRe` in the integration test (white-box access). context is imported
       in the test files; strings is imported. No new test import needed (regexp is accessed via indexLineRe).
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go              # imports (L3-14) + buildDiffArgs (L688) + StagedDiff/TreeDiff/WorkingTreeDiff (6 capture→cap sites) — EDIT
  binary.go           # detectBinaryFiles/fileStatuses (numstat/name-status) — NO edit
  stagediff_test.go   # ~23 substring golden tests + ADD TestStripIndexLines + TestStagedDiff_IndexLineStripped — EDIT
  treediff_test.go    # substring golden tests — EDIT only if a run-driven break (none expected)
  workingtreediff_test.go # substring golden tests — EDIT only if a run-driven break (none expected)
  (other _test.go)    # git_test/committree/etc helpers — NO edit
go.mod / go.sum       # unchanged (regexp is stdlib)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Edits to internal/git/git.go (regexp import + indexLineRe + stripIndexLines + 6 insertions)
# + internal/git/stagediff_test.go (2 new tests). treediff/workingtreediff _test.go only if a run-driven break.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (post-capture, NOT argv): stripIndexLines runs on the captured string AFTER g.run returns it.
// Do NOT touch buildDiffArgs or the 9 argv sites (that is the parallel T2.S1's -M/-U scope, LANDED). This
// task is a pure string transform + 6 one-line insertions between capture and cap.

// CRITICAL (the regex is OID-form-disambiguated): `^index [0-9a-f]+\.\.[0-9a-f]+ ` matches ONLY git's
// index header. A content line starting with "index" but lacking `<hex>..<hex> ` (e.g. "index of items")
// does NOT match ("of" is not hex) → KEPT. In a diff body, content lines also carry +/-/space markers, so
// they cannot start with "index " anyway. Never loosen the regex to `^index ` (that would strip a content
// line that starts with the word "index"); never tighten to require the mode (`\d+$`) — the contract regex
// stops at the trailing space and that is sufficient + authoritative.

// CRITICAL (capture → strip → cap ORDER): insert stripIndexLines AFTER the exit-code check and BEFORE the
// existing byte/line cap at all 6 sites, so the cap measures the stripped size (the FR3h savings point).
// Inserting AFTER the cap would leave the index bytes counted against the cap (defeats FR3h). The cap code
// itself is UNCHANGED.

// CRITICAL (all 6 sites, BOTH bodies, FR3c parity): md per-file fileDiff + nm aggregate nmDiff in EACH of
// StagedDiff/TreeDiff/WorkingTreeDiff. A missed site = an index line survives in one path (FR3c broken).
// StagedDiff-only is NOT enough; the decompose path uses TreeDiff/WorkingTreeDiff.

// GOTCHA (placeholders never hit the helper): [binary]/[excluded] placeholder lines are synthesized via
// binaryPlaceholderLine/excludedPlaceholderLine directly to the strings.Builder — they never pass through
// stripIndexLines and never contain an index line. Do NOT route them through the helper or add a special-case.

// GOTCHA (regexp is a NEW import in git.go): git.go imports bytes/context/errors/fmt/io/os/exec/path/filepath/
// sort/strconv/strings — NOT regexp. ADD "regexp" (stdlib; go.mod unchanged). Compile ONCE at package scope:
// var indexLineRe = regexp.MustCompile(`^index [0-9a-f]+\.\.[0-9a-f]+ `). Do NOT compile inside stripIndexLines
// (per-call compile wastes CPU).

// GOTCHA (fast path): `if !strings.Contains(s, "index ") { return s }` short-circuits only when the substring
// is entirely absent (→ no line can start with "index "). When a content line CONTAINS "index " the fast path
// proceeds to the per-line regex, which correctly keeps it. Safe; avoids a Split/Join alloc in the no-index case.

// GOTCHA (always-on): stripIndexLines is UNCONDITIONAL. Do NOT gate on opts.TokenLimit. FR3h is always-on
// (system_context §6), like FR3e/FR3f. M4 owns the token-limit gate; this task runs at every capture.

// GOTCHA (fixtures are substring-based + no test references an index line): do NOT rewrite golden fixtures
// prophylactically. RUN the suites; fix only a real break. Verified: zero tests reference `index [0-9a-f]`.
// Expect ~zero churn (the strip removes a line no test asserts on; kept lines are the assertion anchors).

// GOTCHA (pure rename / mode-only have no index line): after -M, a pure rename emits similarity index/rename
// from/rename to with NO `index <oid>..<oid>` line → stripIndexLines is a no-op there (correct; nothing to strip).
// Do NOT add special handling; the regex simply finds no match.
```

## Implementation Blueprint

### Data models and structure

No new types. One package-level regex var + one helper func:

```go
// (imports block — ADD "regexp":)
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// indexLineRe matches git's per-file `index <oid>..<oid> <mode>` patch header (PRD §9.1 FR3h). Anchored
// at start: "index ", a hex OID run, "..", another hex OID run, then a space (the file mode follows).
// A content line that merely starts with the word "index" (e.g. "// index of items" or "index of items")
// does NOT match — it lacks the "<hex>..<hex> " form — and is preserved. diff --git / --- / +++ / @@ /
// similarity index / rename from / rename to lines start with a different token and are all kept. In a
// real diff body content lines also carry a +//-/space marker, so they cannot start with "index " at all.
var indexLineRe = regexp.MustCompile(`^index [0-9a-f]+\.\.[0-9a-f]+ `)

// stripIndexLines removes git's per-file `index <oid>..<oid> <mode>` header lines from a captured diff
// (PRD §9.1 FR3h). The blob OIDs are useless to the model and cost ~30 bytes/file; stripping pre-cap means
// the byte/line cap measures the smaller, stripped size. Only a line matching indexLineRe is dropped; every
// other line is preserved verbatim. No git flag suppresses the index line (git_diff_semantics §4), so this
// post-capture filter is the only way. Applied in all 3 diff paths (FR3c parity) on BOTH the markdown
// per-file body and the non-markdown aggregate, immediately after capture and before the cap. Always-on
// (not token-limit-gated). A pure rename / mode-only change has no index line → this is a no-op there.
func stripIndexLines(s string) string {
	if !strings.Contains(s, "index ") {
		return s // fast path: substring absent → no line can start with "index "
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if indexLineRe.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
```

> **Placement:** put `indexLineRe` + `stripIndexLines` adjacent to `buildDiffArgs` (the other shared
> diff helper, ~L688), immediately before `StagedDiff`. **gofmt:** run `gofmt -w internal/git/git.go
> internal/git/stagediff_test.go`. **Imports:** only `"regexp"` is added (stdlib; go.mod unchanged).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: git.go — add the regexp import + indexLineRe + stripIndexLines
  - ADD "regexp" to the import block (alphabetical: after "path/filepath", before "sort").
  - ADD var indexLineRe = regexp.MustCompile(`^index [0-9a-f]+\.\.[0-9a-f]+ `) + the stripIndexLines func
    (per the Data Models block), placed adjacent to buildDiffArgs (~L688).
  - GOTCHA: compile ONCE at package scope (not per-call). Keep the fast-path Contains check. The regex is
      OID-form-disambiguated — do NOT loosen to `^index `.

Task 2: git.go — insert stripIndexLines at the 6 capture→cap sites
  - StagedDiff md-loop: after `if fcode != 0 { … }` and BEFORE `if lines := strings.Split(fileDiff, "\n")`,
      insert `fileDiff = stripIndexLines(fileDiff)`.
  - StagedDiff nm section: after `if nmcode != 0 { … }` and BEFORE `if len(nmDiff) > maxDiffBytes`,
      insert `nmDiff = stripIndexLines(nmDiff)`.
  - TreeDiff: the SAME two insertions (md-loop fileDiff; nm nmDiff), after each exit-code check, before each cap.
  - WorkingTreeDiff: the SAME two insertions.
  - GOTCHA: 6 insertions total (2 per function × 3). Insert AFTER the exit-code check, BEFORE the cap. The cap
      code is UNCHANGED. Placeholders are NOT touched (they don't pass through the helper). Re-confirm line
      numbers at edit time (T2.S1's argv edits sit above each capture; the capture/cap lines are stable).

Task 3: stagediff_test.go — ADD TestStripIndexLines (unit, table-driven)
  - ADD the test per the sketch below: 6 cases — index removed; content-line-starting-with-index kept
      (both a bare "index of items" line AND diff-marked " // index"/"-index"/"+index"); no-index → unchanged;
      multi-file; similarity index/rename from/rename to KEPT; empty string.
  - ASSERT exact-string equality (got == want) for determinism.
  - GOTCHA: this is a pure unit test (no git); it directly tests the helper logic incl. the negative case.

Task 4: stagediff_test.go — ADD TestStagedDiff_IndexLineStripped (integration)
  - ADD the test per the sketch below: initRepo + commit baseline a.go + edit + stage; StagedDiff(DiffContext:1);
      assert NO line matches indexLineRe (reuse the package-level regex, white-box); assert "diff --git a/a.go
      b/a.go" and "+++ b/a.go" are retained.
  - PATTERN: mirror the existing stagediff_test.go fixture style (initRepo/writeFile/stageFile/execGit/New).
  - GOTCHA: reuse indexLineRe (not a re-derived check); white-box package git. DiffContext:1 (the production
      default; matches the struct doc "callers MUST pass resolved default 1").

Task 5: VERIFY (run-driven fixture check + full suite)
  - RUN `gofmt -w internal/git/git.go internal/git/stagediff_test.go`; `go vet ./internal/git/`;
      `go build ./...`; `go test -race ./internal/git/ -v`; `go test -race ./...`.
  - EXPECTED: existing golden suites green unchanged (no test references an index line). If a test surprisingly
      breaks, fix ONLY that expectation (keep boundary-marker + truncation-sentinel anchors). go.mod/go.sum
      byte-unchanged. buildDiffArgs/binary.go/config/the 6 production call sites byte-unchanged.
```

### Test Specs (stagediff_test.go — 2 new tests)

```go
// TestStripIndexLines verifies FR3h's post-capture filter: the `index <oid>..<oid> <mode>` header line is
// removed; every other line is preserved verbatim — including a content line that starts with the word
// "index" but lacks the OID `..` form (the regex disambiguator), and the rename/similarity extended headers.
func TestStripIndexLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "index line removed, structural + content kept",
			input: "diff --git a/a.go b/a.go\nindex 600d48a..62b056e 100644\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
			want:  "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
		},
		{
			// THE contract negative case: a content line that starts with "index" but lacks the OID form is KEPT.
			// "index of items" → "of" is not hex → no match. The diff-marked variants (space/-/+) also kept.
			name:  "content line starting with index but no OID form is kept",
			input: "index of items in the list\n",
			want:  "index of items in the list\n",
		},
		{
			name:  "diff-marked content lines mentioning index are kept (markers protect them)",
			input: "diff --git a/a.go b/a.go\nindex 600d48a..62b056e 100644\n@@ -1,3 +1,3 @@\n // index of items\n-index of items\n+index of other\n",
			want:  "diff --git a/a.go b/a.go\n@@ -1,3 +1,3 @@\n // index of items\n-index of items\n+index of other\n",
		},
		{
			name:  "no index line → byte-identical (fast path)",
			input: "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
			want:  "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
		},
		{
			name:  "multiple files → all index lines removed, headers/content kept",
			input: "diff --git a/a.go b/a.go\nindex 1111111..2222222 100644\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-a\n+b\ndiff --git a/b.md b/b.md\nindex 3333333..4444444 100644\n--- a/b.md\n+++ b/b.md\n@@ -1 +1 @@\n-x\n+y\n",
			want:  "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-a\n+b\ndiff --git a/b.md b/b.md\n--- a/b.md\n+++ b/b.md\n@@ -1 +1 @@\n-x\n+y\n",
		},
		{
			// Composes with FR3e (-M): a pure rename has NO index line; similarity index / rename from / to
			// start with a different token → all KEPT. stripIndexLines is a no-op here (correct).
			name:  "rename path: similarity index / rename from / rename to KEPT (no index line present)",
			input: "diff --git a/old.go b/new.go\nsimilarity index 100%\nrename from old.go\nrename to new.go\n",
			want:  "diff --git a/old.go b/new.go\nsimilarity index 100%\nrename from old.go\nrename to new.go\n",
		},
		{
			name:  "empty string → empty string",
			input: "",
			want:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripIndexLines(tc.input); got != tc.want {
				t.Errorf("stripIndexLines mismatch:\n got=%q\nwant=%q", got, tc.want)
			}
		})
	}
}

// TestStagedDiff_IndexLineStripped is the FR3h integration test: a real captured StagedDiff payload
// contains NO `index <oid>..<oid> <mode>` line, while the structural identity lines (diff --git, +++) are
// retained. Proves the helper is wired into the capture→strip→cap path.
func TestStagedDiff_IndexLineStripped(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")
	execGit(t, repo, "commit", "-qm", "base")
	writeFile(t, repo, "a.go", "package main\nfunc a(){}\n")
	stageFile(t, repo, "a.go")

	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{DiffContext: 1})
	if err != nil {
		t.Fatalf("StagedDiff: %v", err)
	}
	// FR3h: no line in the captured payload matches the index-header regex.
	for _, line := range strings.Split(out, "\n") {
		if indexLineRe.MatchString(line) {
			t.Errorf("FR3h: index line present in StagedDiff output: %q\nfull output:\n%s", line, out)
		}
	}
	// Sanity: the kept structural lines are present.
	if !strings.Contains(out, "diff --git a/a.go b/a.go") {
		t.Errorf("diff --git header missing (should be KEPT):\n%s", out)
	}
	if !strings.Contains(out, "+++ b/a.go") {
		t.Errorf("+++ header missing (should be KEPT):\n%s", out)
	}
}
```

> **Test imports:** `context`, `strings`, `testing` are already imported in the `internal/git` test files;
> `indexLineRe` is the package-level var (white-box access, no `regexp` import needed in the test). No new
> test import.

### Implementation Patterns & Key Details

```go
// THE helper (the entire production logic in miniature):
var indexLineRe = regexp.MustCompile(`^index [0-9a-f]+\.\.[0-9a-f]+ `)

func stripIndexLines(s string) string {
	if !strings.Contains(s, "index ") {
		return s
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if indexLineRe.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// THE 6 insertions — each ONE line, capture → strip → cap (md-loop site shown; nm site identical shape):
//   fileDiff, fstderr, fcode, ferr := g.run(ctx, g.workDir, append(buildDiffArgs(opts, "--cached"), "--", file)...)
//   if ferr != nil { return "", ferr }
//   if fcode != 0 { return "", fmt.Errorf(...) }
//   fileDiff = stripIndexLines(fileDiff)   // ← INSERT FR3h: before the line cap so the cap measures stripped size
//   if lines := strings.Split(fileDiff, "\n"); len(lines) > maxMDLines { … }   // cap UNCHANGED
// (nm site: nmDiff = stripIndexLines(nmDiff) between the nmcode check and `if len(nmDiff) > maxDiffBytes`.)

// WHY the regex is safe (the disambiguator): `^index [0-9a-f]+\.\.[0-9a-f]+ ` requires the `<hex>..<hex> `
// OID form. "index of items" → "of" is not hex → no match → KEPT. "// index of items" → starts with "//" →
// no match → KEPT. In a diff body, content lines carry +/-/space markers → cannot start with "index " → KEPT.
// Only git's actual `index <oid>..<oid> <mode>` header matches. Surgical by construction.
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — "regexp" is stdlib. go mod tidy is a no-op.

PACKAGE EDGES: NONE added/removed (regexp is stdlib). internal/git stays a leaf.

FROZEN / NOT-EDITED:
  - buildDiffArgs + the 9 argv call sites (the parallel T2.S1's -M/-U scope; LANDED). This task is
        post-capture; it does NOT touch argv.
  - internal/git/binary.go (detectBinaryFiles/fileStatuses — numstat/name-status; no patch body, no index line).
  - The byte/line cap code (unchanged — stripIndexLines is inserted BEFORE the cap; the cap block is verbatim).
  - binaryPlaceholderLine / excludedPlaceholderLine (placeholders never pass through the helper).
  - StagedDiffOptions / config / the 6 production call sites (FR3h is always-on; needs no option).
  - docs/* (DOCS: none; P1.M5 owns the diff-capture doc sync).

DOWNSTREAM ENABLED (do NOT implement here):
  - P1.M3 (FR3g numstat skeleton): prepends a skeleton block; composes with the now-index-free body.
  - P1.M4 (FR3d/FR3i token-limit gate + water-fill): measures/truncates the post-strip body. THIS task's
        strip is unconditional (the token-limit gate wraps the cap, not the strip).
  - generate/decompose/hook consume a payload with no index lines (their tests are substring/output-shape).

NO DATABASE / ROUTES / CONFIG / CLI / NEW FILES / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/git/git.go internal/git/stagediff_test.go
test -z "$(gofmt -l internal/git/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/git/   # catches a missing regexp import, an unused var, a malformed regexp.MustCompile.
go build ./...           # whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm regexp is imported in git.go and indexLineRe/stripIndexLines are defined:
grep -n '"regexp"' internal/git/git.go
grep -n 'indexLineRe\|func stripIndexLines' internal/git/git.go
# Confirm the 6 insertions (each capture→cap gap has a stripIndexLines call):
grep -n 'stripIndexLines' internal/git/git.go   # expect: 1 func def + 6 call sites = 7 lines
# Confirm buildDiffArgs/binary.go are UNCHANGED (T2.S1's scope, not this task's):
git diff --exit-code internal/git/binary.go && echo "binary.go UNCHANGED (expected)"
```

### Level 2: Unit Tests (Component Validation)

```bash
# The git suite (golden suites unchanged-by-FR3h + 2 new tests):
go test -race ./internal/git/ -v
# Expected PASS — verify explicitly:
#   TestStripIndexLines (NEW, 7 sub-tests): index removed; content-starting-with-index kept; no-index
#       unchanged; multi-file; rename/similarity kept; empty.
#   TestStagedDiff_IndexLineStripped (NEW): real StagedDiff output has no indexLineRe match; diff --git / +++
#       retained.
#   TestStagedDiff_* / TestTreeDiff* / TestWorkingTreeDiff* (golden): UNCHANGED, still green (no test
#       references an index line; the strip removes only a line nothing asserts on).
#   binary/difftree/difftreenames/etc: unchanged, green.
go test -race ./...   # Full suite — generate/decompose/hook consume an index-free payload; fix any output-shape break.
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only internal/git/ changed:
git diff --name-only | grep -Ev '^internal/git/' && echo "UNEXPECTED file changed" || echo "only internal/git/ changed (good)"
# Confirm buildDiffArgs/binary.go/config byte-unchanged:
git diff --exit-code internal/git/binary.go && echo "binary.go UNCHANGED"
# Smoke (optional): in a temp repo, stage a one-line edit; stagecoach --dry-run; confirm the captured payload
# has no `index <oid>..<oid> <mode>` line (FR3h end-to-end through generate).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Determinism + the no-false-positive check: capture a real diff that includes a content line starting with
# the word "index" (e.g. a code comment), and confirm stripIndexLines drops ONLY the git index header, never
# the content line. (TestStripIndexLines covers this in-process; this is the cross-format belt-and-suspenders.)
# golangci-lint (project-wide gate):
make lint 2>/dev/null || golangci-lint run ./internal/git/ 2>/dev/null || echo "(golangci-lint optional in dev)"
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/git/`, `go mod tidy` no-op;
      `"regexp"` imported in git.go; `indexLineRe` + `stripIndexLines` defined; 6 call-site insertions
      (grep shows 7 `stripIndexLines` lines: 1 def + 6 calls); binary.go + go.mod/go.sum byte-unchanged.
- [ ] Level 2 green: `go test -race ./internal/git/` (2 new tests + all golden) AND `go test -race ./...`.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only `internal/git/` changed; binary.go byte-unchanged.

### Feature Validation

- [ ] `stripIndexLines(s)` drops lines matching `^index [0-9a-f]+\.\.[0-9a-f]+ `; preserves all else.
- [ ] 6 insertions (md fileDiff + nm nmDiff × StagedDiff/TreeDiff/WorkingTreeDiff), each AFTER the exit-code
      check, BEFORE the cap. FR3c parity (all 3 paths, both bodies).
- [ ] A captured diff has NO `indexLineRe` match; `diff --git`/`---`/`+++`/`@@`/content/`similarity index`/
      `rename from`/`rename to` retained.
- [ ] A content line starting with "index" but lacking the OID form is preserved (TestStripIndexLines).
- [ ] `TestStripIndexLines` (unit) + `TestStagedDiff_IndexLineStripped` (integration) pass.

### Code Quality Validation

- [ ] Package-level compiled regexp (compiled once, not per-call); fast-path Contains check.
- [ ] Mirrors existing stagediff_test.go conventions (white-box `package git`; initRepo/writeFile/stageFile/
      execGit/New; StagedDiffOptions{DiffContext:1}).
- [ ] No scope creep into buildDiffArgs/argv (T2.S1), binary.go, the cap code, config, call sites, M3/M4, docs.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] No docs edits (DOCS: none — P1.M5 owns the diff-capture doc sync).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ Don't touch `buildDiffArgs`, the 9 argv call sites, or `-M`/`-U`. That is the parallel T2.S1's scope
  (pre-capture argv). THIS task is a POST-CAPTURE string transform on the bytes git already returned.
- ❌ Don't loosen the regex to `^index ` (bare). That would strip a content line that starts with the word
  "index". The contract regex `^index [0-9a-f]+\.\.[0-9a-f]+ ` (OID `..` form + trailing space) is the
  disambiguator — keep it exactly.
- ❌ Don't insert `stripIndexLines` AFTER the cap. The cap must measure the STRIPPED size (the FR3h savings
  point). Insert AFTER the exit-code check, BEFORE the cap, at all 6 sites.
- ❌ Don't miss a site. FR3c parity requires all 6 (md fileDiff + nm nmDiff × 3 functions). StagedDiff-only
  leaves index lines in the decompose path (TreeDiff/WorkingTreeDiff).
- ❌ Don't route `[binary]`/`[excluded]` placeholders through `stripIndexLines` or add a placeholder
  special-case. They're synthesized directly to the strings.Builder and never contain an index line; the
  contract's "Do NOT strip inside placeholder lines" is satisfied by construction.
- ❌ Don't gate the strip on `opts.TokenLimit`. FR3h is ALWAYS ON (system_context §6), like FR3e/FR3f. M4
  owns the token-limit gate.
- ❌ Don't compile the regexp inside `stripIndexLines` (per-call compile wastes CPU). Compile ONCE at package
  scope (`var indexLineRe = regexp.MustCompile(…)`).
- ❌ Don't forget to ADD `"regexp"` to git.go's imports. It is NOT currently imported (git.go imports
  bytes/context/errors/fmt/io/os/exec/path/filepath/sort/strconv/strings). It's stdlib → go.mod unchanged.
- ❌ Don't rewrite the golden fixtures prophylactically. RUN the suites first — verified ZERO tests reference
  an `index [0-9a-f]` line, so ~all pass unchanged. Fix only a real break; keep boundary-marker +
  truncation-sentinel anchors.
- ❌ Don't edit the byte/line cap code. The cap block is UNCHANGED; `stripIndexLines` is inserted BEFORE it.
- ❌ Don't edit binary.go, config, the 6 production call sites, or docs. This task is internal to
  `internal/git/` (git.go + stagediff_test.go only).
- ❌ Don't change go.mod/go.sum or add files. One import + one var + one func + 6 one-line insertions + 2 tests.
- ❌ Don't skip `go test -race ./...` — it confirms generate/decompose/hook still pass with the index-free
  payload (their tests are substring/output-shape; fix any surprising break).

---

## Confidence Score

**9/10** — a surgical, single-helper post-capture transform (the regex is OID-form-disambiguated so it
cannot touch content; the contract pins it verbatim), 6 mechanical one-line insertions at pinned
capture→cap sites, a comprehensive table-driven unit test (incl. the contract's headline negative case) +
an integration test, and a VERIFIED golden-fixture reality (zero tests reference an index line ⇒ no churn).
It is cleanly orthogonal to the parallel T2.S1 (argv vs. string) with explicit scope fences. The -1
reserves for the run-driven fixture check (a test could surprisingly depend on an index line, though none
do) and re-confirming the 6 line numbers at edit time (T2.S1's argv edits sit just above each capture and
don't shift the capture/cap lines, but verify).
