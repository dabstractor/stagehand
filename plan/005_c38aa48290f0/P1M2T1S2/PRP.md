name: "P1.M2.T1.S2 — Compiled-in gitmoji reference table"
description: |
  A leaf DATA subtask (INPUT: none). Author `internal/prompt/gitmoji.go` containing the canonical gitmoji
  reference table (PRD §9.19 FR-F3 / §17.8) as a COMPILE-TIME Go constant — NO network fetch at runtime, ever.
  Deliverables: (1) the `Gitmoji` struct (Emoji, Description, Name — name kept as the stable `:shortcode:` key),
  (2) the exported `GitmojiTable` slice of all 75 canonical entries (verbatim from carloscuesta/gitmoji's
  `gitmojis.json`), (3) a `RenderGitmojiTable() string` convenience renderer (the §17.8 "emoji + meaning"
  block, one line per entry), (4) `gitmojiVerifiedCount`/`gitmojiVerifiedDate` drift-check consts + a header
  comment recording the source URL/count/date (FR-D5 discipline), and (5) table-driven property tests
  (non-empty, count, unique emojis, unique names, every entry complete, render sanity). This subtask produces
  the DATA + the table renderer and NOTHING ELSE: the format-mode prompt scaffold / locale line / swap wiring
  into `Build*` is S3 (P1.M2.T1.S3); the `cfg.Format` enum ("gitmoji" already a valid mode) is S1
  (P1.M2.T1.S1, already landed). No config/flag/env/git-config changes. No docs (Mode A: internal constant).

---

## Goal

**Feature Goal**: The `prompt` package exposes a stable, compile-time, offline gitmoji reference table that a
future consumer (S3's `gitmoji` format-mode scaffold) can embed verbatim into a system prompt, with the data
fidelity (every canonical emoji + its meaning), currency (a recorded verification date + a self-checking count
constant), and integrity (unique emojis, unique keys, no empty fields) all machine-verified by unit tests.

**Deliverable**: One new file, `internal/prompt/gitmoji.go`, plus `internal/prompt/gitmoji_test.go`. The file
exports `Gitmoji` (struct), `GitmojiTable` (`[]Gitmoji`, 75 entries), and `RenderGitmojiTable() string`; and
declares two unexported drift consts (`gitmojiVerifiedCount = 75`, `gitmojiVerifiedDate = "2026-07-02"`) plus a
header comment naming the source URL, count, and date. The test asserts the contract invariants. No other file
is touched.

**Success Definition**:
- `prompt.GitmojiTable` has exactly `gitmojiVerifiedCount` (75) entries, each a complete `{Emoji, Description, Name}` triple (no empty field).
- Every `Emoji` in the table is unique; every `Name` is unique.
- `prompt.RenderGitmojiTable()` returns a non-empty block of exactly `len(GitmojiTable)` lines, each `<emoji> - <description>`, and contains a known emoji (🎨) + a known description.
- The header comment records the source URL, the entry count, and the verification date (FR-D5 / Appendix E #16).
- The table is a compile-time constant: there is NO `http`/`net` fetch, NO `//go:embed`, NO `os.ReadFile` of the JSON at runtime (FR-F3).
- `go build ./...`, `go test ./internal/prompt/...`, `go vet ./internal/prompt/...`, `golangci-lint run ./internal/prompt/...` all pass; `gofmt` produces no diff.

## User Persona (if applicable)

**Target User**: The implementing agent for **S3 (P1.M2.T1.S3)** — the consumer of this table — and, one layer
out, "the plan-holder" / "the API-key refusenik" who runs `stagecoach --format gitmoji` (PRD §7). This subtask
is invisible to end users (Mode A: no user-facing docs); it exists so S3 has a tested, offline, canonical data
source to render.

**Use Case**: `--format gitmoji` (FR-F3) makes stagecoach emit commit subjects prefixed with a single gitmoji.
The model needs the reference table (emoji + meaning) in its system prompt to choose the right emoji; that
table must be byte-stable, offline, and current — which is exactly what `GitmojiTable` + `RenderGitmojiTable()`
provide.

**User Journey**: (S3) resolves `cfg.Format == "gitmoji"` → builds the §17.8 scaffold → embeds
`prompt.RenderGitmojiTable()` (or iterates `prompt.GitmojiTable`) → the model sees the table → emits
`🎨 Refactor foo`-style subjects. S2 is the data under that flow.

**Pain Points Addressed**: (1) Incumbents that fetch the gitmoji list at runtime are fragile/offline-hostile
(stagecoach's whole thesis, PRD §5); a compiled-in constant removes the network dependency entirely (FR-F3).
(2) Hand-transcribed tables drift / typo; generating the literal from the canonical JSON + a self-checking
count const removes transcription risk and makes drift detectable.

## Why

- **FR-F3 (PRD §9.19)**: "`gitmoji`. … The prompt embeds the canonical gitmoji reference table (emoji + meaning,
  from the gitmoji spec) **compiled into the binary** — no network fetch, ever. The table is a build-time
  constant refreshed like model defaults (FR-D5 discipline: verify at implementation, record the date)."
  This subtask IS that compiled-in constant.
- **§17.8 (PRD)**: the gitmoji scaffold is *"Begin the subject with exactly ONE emoji … followed by a space and
  the description"* — **followed by the compiled-in gitmoji reference table (emoji + meaning)**. S2 provides the
  "emoji + meaning" table; S3 provides the instruction + placement. This split keeps the 75-entry rendering in
  one auditable place (the package convention — see `system.go`'s `BuildSystemPrompt`, `payload.go`'s
  `BuildUserPayload`).
- **FR-D5 / Appendix E #16**: "verify at implementation, record the date." `architecture/external_deps.md §5`
  (verified 2026-07-02) is the canonical reference: source URL, 75 entries, stable fields. The drift consts +
  the count test make "record the date" an executable, self-checking discipline rather than a prose comment.
- **Scope boundary**: S2 is a LEAF data subtask (INPUT: none). It does NOT touch config (S1 owns `cfg.Format`,
  where "gitmoji" is already a validated mode — confirmed landed in `internal/config/load.go:350`
  `validFormats`) and does NOT build the scaffold (S3). Keeping it leaf lets it land independently and be
  consumed unchanged by S3.

## What

A new `internal/prompt/gitmoji.go` whose public surface is:

```go
package prompt

// Gitmoji is one entry in the canonical gitmoji reference table (PRD §9.19 FR-F3 / §17.8).
type Gitmoji struct {
	Emoji       string // the emoji character (e.g. "🎨")
	Description string // its meaning (e.g. "Improve structure / format of the code.")
	Name        string // the stable :shortcode: key (e.g. "art"); stable across versions; never rendered
}

// GitmojiTable is the compiled-in canonical gitmoji reference table (75 entries).
// READ-ONLY: treat as immutable; iterate/render only.
var GitmojiTable = []Gitmoji{
	{Emoji: "🎨", Description: "Improve structure / format of the code.", Name: "art"},
	// … 74 more, verbatim from gitmojis.json …
	{Emoji: "🦖", Description: "Code that adds backwards compatibility.", Name: "t-rex"},
}

// RenderGitmojiTable renders the §17.8 "emoji + meaning" reference block: one line per entry,
// "<emoji> - <description>". Pure; no trailing newline.
func RenderGitmojiTable() string { /* iterate GitmojiTable, join lines */ }
```

Plus two unexported drift consts (`gitmojiVerifiedCount = 75`, `gitmojiVerifiedDate = "2026-07-02"`) in the
header block, and `internal/prompt/gitmoji_test.go` asserting the invariants.

### Success Criteria

- [ ] `internal/prompt/gitmoji.go` exists with exported `Gitmoji`, `GitmojiTable`, `RenderGitmojiTable`.
- [ ] `GitmojiTable` has exactly `gitmojiVerifiedCount` (75) entries; the header comment records the source
      URL (`https://raw.githubusercontent.com/carloscuesta/gitmoji/master/packages/gitmojis/src/gitmojis.json`),
      the count, and `gitmojiVerifiedDate`.
- [ ] Every entry has non-empty `Emoji`, `Description`, and `Name`.
- [ ] All `Emoji` values are unique; all `Name` values are unique.
- [ ] `RenderGitmojiTable()` returns exactly `len(GitmojiTable)` lines (`strings.Count == len-1` newlines), each
      `<emoji> - <description>`, containing a known emoji + description.
- [ ] No runtime network/embed/file-read of the JSON (FR-F3); pure compile-time literals.
- [ ] `gitmoji_test.go` (package prompt) covers: non-empty, count==const, unique emoji, unique name, every-field-
      non-empty, and `RenderGitmojiTable()` shape.
- [ ] `go build ./...`, `go test ./internal/prompt/...`, `go vet`, `golangci-lint` clean; `gofmt` no diff.

## All Needed Context

### Context Completeness Check

_This PRP names the one new file + its test, the exact public surface (3 exported symbols + 2 unexported consts),
the canonical data source + a paste-ready generated literal, the package conventions to match (constants carry no
trailing newline; `Build*`/`Render*` own assembly; defensive pure functions; comments cite the PRD section), the
test invariants, the linter constraint (export to dodge `unused` pre-S3), and the exact S1/S3 scope fence. An
implementer with no prior codebase knowledge can complete it by pasting the generated table and writing the small
renderer + tests._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/architecture/external_deps.md
  why: §5 "gitmoji canonical list (gates FR-F3, Appendix E #16) — VERIFIED (web, 2026-07-02)" is the authoritative
       source: the raw URL, the 75-entry count, and the stable field set (emoji/entity/code/description/name/semver).
       It also states the compile discipline: "Compile emoji + description (optionally name) into the binary as a Go
       constant table; record 'verified 2026-07-02, 75 entries' in the source comment (FR-D5 discipline). No network
       fetch, ever (FR-F3)." This subtask executes exactly that.
  section: "## 5. gitmoji canonical list"
  critical: "Source URL + count (75) + the 'keep name' instruction are all here. The implementation-time re-verify
             (Appendix E #16) is satisfied by regenerating from this URL via research/regenerate_table.py and updating
             gitmojiVerifiedCount/gitmojiVerifiedDate if the count drifted."

- docfile: plan/005_c38aa48290f0/architecture/system_context.md
  why: §3 "internal/prompt" inventories the package (system.go/payload.go/planner.go/stager.go/arbiter.go) and flags
       "No format/locale/context seams exist" — confirming S2 must ADD a new file (gitmoji.go), not edit an existing
       scaffold (that is S3). §5 feature→seam map rows §9.19 shaping's "gitmoji constant" as Net-new. §1 nails the
       toolchain: Go 1.22; golangci-lint enables `unused`; no new dep should be added for a static table.
  section: "## 3. Package inventory" + "## 5. Feature → seam map" + "## 1. Build & dependencies"
  critical: "No format/locale/context seam exists today → gitmoji.go is a brand-new self-contained data file. Do NOT
             edit system.go/payload.go/planner.go (S3 owns the scaffold wiring). Go 1.22; the `unused` linter means
             GitmojiTable/RenderGitmojiTable/Gitmoji MUST be EXPORTED (S3 is a later task — unexported + unconsumed
             = 'declared but not used' build/lint failure today)."

- docfile: plan/005_c38aa48290f0/P1M2T1S2/research/gitmoji_table.go.txt
  why: The PASTE-READY Go slice literal for all 75 entries, generated programmatically from the canonical JSON
       (research/gitmojis_canonical.json). Eliminates all hand-transcription risk — copy the `var GitmojiTable =
       []Gitmoji{ … }` body verbatim into gitmoji.go.
  pattern: "One struct literal per line: {Emoji: \"🎨\", Description: \"…\", Name: \"art\"}. Fields already
            Go-escaped (none of the 75 descriptions contain quotes/backslashes — verified). Emojis with U+FE0F
            variation selectors (⚡️ 🚑️ ✈️ …, 19 of them) are valid in Go UTF-8 string literals — leave verbatim."
  critical: "Use this generated body AS-IS; do not retype. After pasting, re-run research/regenerate_table.py against
             a fresh fetch and diff — if the upstream count changed, update the data + gitmojiVerifiedCount +
             gitmojiVerifiedDate (Appendix E #16 re-verify)."

- docfile: plan/005_c38aa48290f0/P1M2T1S2/research/regenerate_table.py
  why: The refresh generator (FR-D5). `python3 regenerate_table.py <gitmojis.json> > out` regenerates the slice
       literal from a freshly-fetched JSON. Run it at implementation time to discharge Appendix E #16.
  critical: "It prints `// count=N` as the first line — use N to set gitmojiVerifiedCount and confirm it is still 75
             (or update the const + header date if drifted)."

- docfile: plan/005_c38aa48290f0/P1M2T1S2/research/design-decisions.md
  why: The eight locked decisions for THIS task (hardcoded-literal-not-embed; 3 fields; export-to-dodge-unused;
       RenderGitmojiTable boundary vs S3; drift consts; the invariants; the scope fence; why no external-research
       subagent).
  critical: "§3 (EXPORT to avoid the `unused` linter failing before S3 lands) and §4 (S2 owns data + table renderer;
             S3 owns the instruction sentence + locale line + Build* swap wiring — do not cross)."

- file: internal/prompt/system.go
  why: The package-convention precedent. Constants (maturePromptHeader, antiReuseProhibition, multilineRuleAllow…)
       carry NO trailing newline; the Build*/Render* functions own ALL inter-block newline placement so topology
       lives in one auditable place. Pure functions; defensive on nil/empty; doc comments cite the PRD section
       (§17.x) verbatim. Match this style for gitmoji.go.
  pattern: |
    - Declare exported types/vars/funcs with a doc comment citing PRD §9.19 FR-F3 / §17.8.
    - RenderGitmojiTable() returns a string with NO trailing newline (callers/S3 append separators); build it with
      a strings.Builder, one "<emoji> - <description>\n" per entry, then strings.TrimRight(b.String(), "\n")
      (mirrors how Build* functions own placement).
    - Cite the verification discipline in the header comment (FR-D5).
  gotcha: "The ONLY non-ASCII bytes in the package today are the em-dash (—) in antiReuseProhibition and now the
          emoji glyphs in GitmojiTable. Ensure the file is saved UTF-8 (Go source is UTF-8 by spec; gofmt preserves
          multibyte runes). Run `gofmt -w` — it must report no change."

- file: internal/prompt/system_test.go  (and payload_test.go)
  why: The test-pattern precedent. Tests are `package prompt` (INTERNAL test — confirmed: `head -1 …_test.go` shows
       `package prompt`), so unexported consts (multilineRuleAllow, gitmojiVerifiedCount) are directly referenceable.
       Table-driven subtests; `strings.Contains` / `strings.Count` assertions; clear subtest names.
  pattern: |
    - `package prompt` (internal) — you MAY reference gitmojiVerifiedCount/gitmojiVerifiedDate directly.
    - Table-driven: build a map[string]struct{} over Emoji (and Name), assert len(map)==len(GitmojiTable) (uniqueness).
    - Loop over GitmojiTable with t.Run per index (or a single loop with a collected failure list) to assert no empty
      Emoji/Description/Name.
    - RenderGitmojiTable(): strings.Count(out, "\n") == len(GitmojiTable)-1; strings.Contains(out, "🎨"); no trailing
      "\n" (assert !strings.HasSuffix).
  gotcha: "Use `package prompt` (NOT `package prompt_test`) so the unexported drift consts are visible. A test that
          fs.Set/env is irrelevant here — this is pure-data, no I/O."

- file: internal/config/load.go  (READ-ONLY — do NOT edit)
  why: Confirms S1 already landed: `validFormats = []string{"auto","conventional","gitmoji","plain"}` (line ~350) and
       `cfg.Format` exist; "gitmoji" is ALREADY a validated mode. S2 does not touch config — it only provides the data
       S3 will render WHEN cfg.Format=="gitmoji".
  pattern: "n/a — reference only."
  gotcha: "Do NOT add anything to config. S1 owns the enum; S2 is INPUT-less. Touching config here duplicates/conflicts
          with S1 (in-flight) and S3."
```

### Current Codebase tree (relevant slice)

```bash
internal/prompt/
  system.go         # BuildSystemPrompt/BuildFallbackPrompt (§17.1/§17.2); constants-no-trailing-newline convention
  payload.go        # BuildUserPayload (§17.3); defensive pure-function + verbatim-fidelity conventions
  planner.go        # BuildPlannerSystemPrompt/... (§17.5) — S3 will add format/locale/context swaps here too
  stager.go         # §17.6
  arbiter.go        # §17.7
  *_test.go         # all `package prompt` (internal tests); table-driven; strings.Contains/Count assertions
# No gitmoji.go exists today (grep-verified: zero "gitmoji" hits under internal/prompt/).
plan/005_c38aa48290f0/P1M2T1S2/research/
  gitmojis_canonical.json   # the raw upstream JSON snapshot (75 entries)
  gitmoji_table.go.txt      # PASTE-READY generated Go slice literal (75 entries)
  regenerate_table.py       # refresh generator (FR-D5 / Appendix E #16)
  design-decisions.md       # the 8 locked decisions
```

### Desired Codebase tree with files to be added

```bash
internal/prompt/gitmoji.go        # NEW — Gitmoji struct + GitmojiTable (75) + RenderGitmojiTable() + drift consts + header
internal/prompt/gitmoji_test.go   # NEW — property tests (non-empty, count, unique emoji, unique name, completeness, render)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: GitmojiTable / RenderGitmojiTable / Gitmoji MUST be EXPORTED. The `unused` linter
// (.golangci.yml enables it) does NOT flag exported symbols, but it DOES fail the build on unexported
// symbols that no current code references. S3 (the consumer) is a LATER task — so at S2 land-time nothing
// references the table except the tests. Exporting is what keeps `go build`/`golangci-lint` green pre-S3.
// (The contract OUTPUT name "prompt.GitmojiTable" is already exported, so this aligns.)

// CRITICAL: NO runtime fetch of the JSON. FR-F3 is absolute ("no network fetch, ever"). Do NOT use net/http,
// os.ReadFile, or //go:embed of gitmojis.json. The table is a `var … = []Gitmoji{{…}}` of string LITERALS —
// a true compile-time constant. (The research/regenerate_table.py generator is a BUILD/MAINTENANCE action,
// run by a human at refresh time, never at runtime.)

// CRITICAL: paste the GENERATED table from research/gitmoji_table.go.txt — do NOT hand-transcribe 75 entries.
// Hand transcription is the single largest source of defect here; the generator eliminates it. After pasting,
// re-run regenerate_table.py against a fresh curl and diff; update gitmojiVerifiedCount/gitmojiVerifiedDate if
// the count drifted (Appendix E #16 re-verify at implementation time).

// GOTCHA: tests MUST be `package prompt` (internal), NOT `package prompt_test`. The drift consts
// (gitmojiVerifiedCount/gitmojiVerifiedDate) are UNEXPORTED and are only reachable from an internal test. Every
// existing *_test.go in internal/prompt is `package prompt` (verified) — match them. (If you used prompt_test,
// you'd have to export the consts or add a test-only accessor — unnecessary; just use package prompt.)

// GOTCHA: emoji glyphs with a U+FE0F VARIATION SELECTOR (⚡️ 🚑️ ✈️ 🔒 ⬇️ ⬆️ ♻️ … — 19 of the 75) are multi-rune
// but valid UTF-8 Go string literals; leave them VERBATIM (do not "normalize" by stripping the selector — that
// changes the glyph, e.g. ⚡→⚡️ differ in rendering). gofmt preserves multibyte runes; run `gofmt -w` and confirm
// no diff. Uniqueness is by Go STRING equality (byte-wise) — the 19 VS emojis are still unique strings (verified).

// GOTCHA: RenderGitmojiTable() must return NO trailing newline (package convention: constants/renderers carry no
// trailing newline; the assembler — here S3's scaffold — owns inter-block placement). Build with strings.Builder
// ("emoji - desc\n" per entry) then strings.TrimRight(…, "\n"). A test pins !strings.HasSuffix(out, "\n").

// GOTCHA: do NOT edit system.go / payload.go / planner.go / stager.go / arbiter.go or any Build* function. The
// gitmoji MODE SCAFFOLD (the "Begin the subject with exactly ONE emoji…" instruction + locale line + the swap of
// the style-examples block) is S3 (P1.M2.T1.S3). S2 ships DATA + a table RENDERER only. Editing Build* here
// collides with S3's in-flight scope.

// GOTCHA: do NOT touch internal/config/* or internal/cmd/root.go. S1 (P1.M2.T1.S1, already landed) owns cfg.Format
// and the "gitmoji" enum value; S2 is INPUT-less (a leaf data subtask). Any config change here duplicates S1.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/prompt/gitmoji.go

package prompt

import "strings"

// gitmojiSourceURL is the canonical upstream JSON (carloscuesta/gitmoji). Recorded for the FR-D5 refresh
// process (Appendix E #16: re-verify currency at implementation time); NEVER fetched at runtime (FR-F3).
const (
	gitmojiSourceURL       = "https://raw.githubusercontent.com/carloscuesta/gitmoji/master/packages/gitmojis/src/gitmojis.json"
	gitmojiVerifiedCount   = 75    // FR-D5: entry count at last verification — TestGitmojiTableCount pins len(GitmojiTable) == this.
	gitmojiVerifiedDate    = "2026-07-02" // FR-D5 / Appendix E #16: the date the table was last verified against gitmojiSourceURL.
)

// Gitmoji is a single entry in the canonical gitmoji reference table (PRD §9.19 FR-F3 / §17.8). The prompt
// renders Emoji + Description; Name (the :shortcode: minus colons) is the STABLE key kept for maintenance —
// it is stable across gitmoji versions (emoji glyphs / description wording drift; the name does not) and makes
// the data self-documenting. It is never rendered into a prompt.
type Gitmoji struct {
	Emoji       string // the emoji character, e.g. "🎨"
	Description string // its meaning, e.g. "Improve structure / format of the code."
	Name        string // the stable :shortcode: key, e.g. "art"
}

// GitmojiTable is the canonical gitmoji reference table compiled into the binary (PRD §9.19 FR-F3 / §17.8).
//
// Source: <gitmojiSourceURL> (carloscuesta/gitmoji, packages/gitmojis/src/gitmojis.json).
// Verified <gitmojiVerifiedDate> — <gitmojiVerifiedCount> entries (FR-D5 discipline; Appendix E #16).
// NO network fetch, ever (FR-F3): this is a build-time constant. To refresh, re-fetch the JSON, regenerate
// the literal (see research/regenerate_table.py), and update gitmojiVerifiedCount/gitmojiVerifiedDate.
//
// READ-ONLY: treat as an immutable constant. Callers iterate / render (RenderGitmojiTable); do not mutate.
var GitmojiTable = []Gitmoji{
	{Emoji: "🎨", Description: "Improve structure / format of the code.", Name: "art"},
	// … (PASTE the remaining 74 lines verbatim from research/gitmoji_table.go.txt) …
	{Emoji: "🦖", Description: "Code that adds backwards compatibility.", Name: "t-rex"},
}

// RenderGitmojiTable renders the PRD §17.8 "emoji + meaning" reference block: one line per entry,
// "<emoji> - <description>". It is the recommended way for the gitmoji format-mode scaffold (S3,
// P1.M2.T1.S3) to embed the table; S3 may also iterate GitmojiTable directly if a different separator
// is wanted (the two are additive). PURE (no I/O); returns a string with NO trailing newline (package
// convention — the caller owns inter-block newline placement, mirroring BuildSystemPrompt/BuildUserPayload).
func RenderGitmojiTable() string {
	var b strings.Builder
	for i, g := range GitmojiTable {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(g.Emoji)
		b.WriteString(" - ")
		b.WriteString(g.Description)
	}
	return b.String()
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/prompt/gitmoji.go
  - IMPLEMENT: the header const block (gitmojiSourceURL, gitmojiVerifiedCount=75, gitmojiVerifiedDate="2026-07-02").
  - IMPLEMENT: the exported Gitmoji struct {Emoji, Description, Name string} with the doc comment above.
  - IMPLEMENT: the exported GitmojiTable slice — PASTE the body from research/gitmoji_table.go.txt VERBATIM
    (the `var GitmojiTable = []Gitmoji{ … }` with all 75 struct literals). Keep the doc comment naming the
    source URL, count, date, the READ-ONLY rule, and the refresh procedure.
  - IMPLEMENT: the exported RenderGitmojiTable() string per the Data-models snippet (strings.Builder, "<emoji>
    - <description>" per line, strings.TrimRight not needed if you gate the leading "\n" on i>0 — either shape
    is fine; the TEST pins no trailing newline).
  - FOLLOW pattern: internal/prompt/system.go (constants no trailing newline; Build*/Render* own placement;
    doc comments cite PRD §9.19 FR-F3 / §17.8; pure + defensive).
  - NAMING: Gitmoji (type), GitmojiTable (var), RenderGitmojiTable (func) — EXPORTED (contract OUTPUT + dodges
    the `unused` linter pre-S3); gitmojiSourceURL/gitmojiVerifiedCount/gitmojiVerifiedDate — unexported (internal
    drift-check metadata, reachable from the internal test).
  - PLACEMENT: brand-new file internal/prompt/gitmoji.go (no existing scaffold to extend — system_context §3).
  - CRITICAL: paste the GENERATED table; do not hand-transcribe. NO net/http, NO os.ReadFile, NO //go:embed
    (FR-F3 — compile-time literals only).

Task 2: CREATE internal/prompt/gitmoji_test.go
  - PACKAGE: `package prompt` (INTERNAL test — required to reference the unexported drift consts; matches every
    existing *_test.go in the dir).
  - IMPLEMENT TestGitmojiTable_NonEmpty: len(GitmojiTable) > 0.
  - IMPLEMENT TestGitmojiTable_Count: len(GitmojiTable) == gitmojiVerifiedCount (the drift self-check; fails if a
    refresh updated the data but forgot the const).
  - IMPLEMENT TestGitmojiTable_UniqueEmojis: build map[string]struct{} over g.Emoji; assert len(map)==len(table).
  - IMPLEMENT TestGitmojiTable_UniqueNames: same over g.Name (name is the stable KEY — a dup is a real defect).
  - IMPLEMENT TestGitmojiTable_EveryEntryComplete: loop over GitmojiTable; assert each Emoji/Description/Name != ""
    (use t.Run per index, or collect failures; cite the contract: "every entry has emoji+description").
  - IMPLEMENT TestRenderGitmojiTable: out:=RenderGitmojiTable(); assert out!="" ; strings.Count(out,"\n")==len-1 ;
    !strings.HasSuffix(out,"\n") ; strings.Contains(out,"🎨") ; strings.Contains(out,"Improve structure / format
    of the code.") ; and every table Emoji appears in out (loop + strings.Contains).
  - FOLLOW pattern: internal/prompt/system_test.go (table-driven subtests; strings.Contains/Count; `package prompt`).
  - NAMING: TestGitmojiTable_<Invariant>, TestRenderGitmojiTable.
  - PLACEMENT: internal/prompt/gitmoji_test.go.
  - GOTCHA: do NOT assert the literal string "75" or "2026-07-02" in prose — assert against the CONSTS so a refresh
    only edits one place (the const) and the test stays green / fails-on-drift correctly.

Task 3: VERIFY (implementation-time re-verify — Appendix E #16)
  - RUN: `curl -s <gitmojiSourceURL> -o /tmp/g.json && python3 plan/005_c38aa48290f0/P1M2T1S2/research/regenerate_table.py /tmp/g.json > /tmp/fresh.txt`
  - DIFF the regenerated body against the pasted GitmojiTable; confirm the `// count=N` line == 75 (== gitmojiVerifiedCount).
  - IF DRIFTED: replace the GitmojiTable body with the regenerated literal AND update gitmojiVerifiedCount +
    gitmojiVerifiedDate (today's date). IF STILL 75: no edit; the existing consts stand.
  - This is a BUILD/MAINTENANCE action (a human/agent refresh), NOT runtime behavior — FR-F3's "no network fetch,
    ever" applies to the RUNNING BINARY, not to the authoring step.
```

### Implementation Patterns & Key Details

```go
// RenderGitmojiTable — pure; no trailing newline (package convention — caller/S3 owns placement):
func RenderGitmojiTable() string {
	var b strings.Builder
	for i, g := range GitmojiTable {
		if i > 0 {
			b.WriteByte('\n') // separator between lines, not a terminator
		}
		b.WriteString(g.Emoji)
		b.WriteString(" - ")
		b.WriteString(g.Description)
	}
	return b.String()
}

// Uniqueness test shape (copy for both Emoji and Name):
func TestGitmojiTable_UniqueEmojis(t *testing.T) {
	seen := make(map[string]struct{}, len(GitmojiTable))
	for _, g := range GitmojiTable {
		if _, dup := seen[g.Emoji]; dup {
			t.Errorf("duplicate emoji %q", g.Emoji)
		}
		seen[g.Emoji] = struct{}{}
	}
	if len(seen) != len(GitmojiTable) {
		t.Errorf("unique emojis %d != table len %d", len(seen), len(GitmojiTable))
	}
}

// Count drift-check (the executable FR-D5 record):
func TestGitmojiTable_Count(t *testing.T) {
	if len(GitmojiTable) != gitmojiVerifiedCount {
		t.Errorf("len(GitmojiTable)=%d != gitmojiVerifiedCount=%d (refresh the const + date per Appendix E #16)",
			len(GitmojiTable), gitmojiVerifiedCount)
	}
}
```

### Integration Points

```yaml
NEW FILE (no edits to existing code):
  - create: internal/prompt/gitmoji.go        (Gitmoji, GitmojiTable, RenderGitmojiTable, drift consts)
  - create: internal/prompt/gitmoji_test.go   (package prompt)

DOWNSTREAM CONSUMER (out of scope — do NOT implement here):
  - S3 (P1.M2.T1.S3): the gitmoji format-mode scaffold — the §17.8 instruction sentence ("Begin the subject with
    exactly ONE emoji …") + embedding prompt.RenderGitmojiTable() (or iterating prompt.GitmojiTable) + the locale
    line + swapping the style-examples block in BuildSystemPrompt/BuildFallbackPrompt (and the planner single-call
    shortcut). S3 keys off cfg.Format=="gitmoji" (S1's resolved enum).

UPSTREAM (already landed — do NOT re-touch):
  - S1 (P1.M2.T1.S1): cfg.Format ∈ {auto,conventional,gitmoji,plain} (internal/config/load.go validFormats) —
    "gitmoji" is already a validated mode; S2 needs no config change.

NO DEPENDENCY / CONFIG CHANGES:
  - go.mod: unchanged (no net/http embed lib, no JSON lib — pure string literals).
  - internal/config/*, internal/cmd/*: unchanged.
  - docs/*: unchanged (Mode A: internal constant — user-facing gitmoji docs ride with S3).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/prompt/gitmoji.go internal/prompt/gitmoji_test.go
go build ./...
go vet ./internal/prompt/...
golangci-lint run ./internal/prompt/...   # repo uses .golangci.yml (enables `unused` — export sidesteps it)

# Expected: zero errors. gofmt MUST report no diff (multibyte emoji runes preserved verbatim). The `unused`
# linter is satisfied because Gitmoji/GitmojiTable/RenderGitmojiTable are EXPORTED (API surface), and the
# unexported drift consts are referenced by gitmoji_test.go (same package).
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/prompt/... -run 'Gitmoji|RenderGitmoji' -v

# Required cases (all must pass):
#  1. NonEmpty:        len(GitmojiTable) > 0.
#  2. Count:           len(GitmojiTable) == gitmojiVerifiedCount (75) — the drift self-check.
#  3. UniqueEmojis:    map over Emoji; len(map) == len(table) (verified live: 75/75 unique).
#  4. UniqueNames:     map over Name;   len(map) == len(table) (name is the stable KEY).
#  5. EveryEntryComplete: each Emoji/Description/Name != "" (verified live: 0 empty of each).
#  6. RenderGitmojiTable: out != ""; strings.Count(out,"\n")==len-1; !HasSuffix(out,"\n");
#     Contains(out,"🎨"); Contains(out,"Improve structure / format of the code."); every table Emoji ∈ out.

# Full package suite — the new file must not perturb existing BuildSystemPrompt/BuildUserPayload/planner tests:
go test ./internal/prompt/...

# Expected: all pass. gitmoji.go adds only NEW exported symbols + a NEW test file — no existing symbol changes.
```

### Level 3: Integration / Offline guarantee (System Validation)

```bash
# (a) OFFLINE guarantee (FR-F3): assert the running binary has NO network/embed code path for gitmoji.
grep -nE 'net/http|os\.ReadFile|go:embed' internal/prompt/gitmoji.go || echo "OK: no runtime fetch/embed (FR-F3)"
# Expected: "OK: no runtime fetch/embed (FR-F3)" — gitmoji.go must contain NONE of those.

# (b) Public surface is reachable + renders: a tiny smoke check via `go run` (or a one-liner test).
cat > /tmp/smoke_test.go <<'EOF'
package main
import ("fmt"; "github.com/dustin/stagecoach/internal/prompt")
func main() {
  fmt.Printf("entries=%d first=%q render_head=%q\n", len(prompt.GitmojiTable), prompt.GitmojiTable[0], prompt.RenderGitmojiTable()[:40])
}
EOF
go run /tmp/smoke_test.go
# Expected: entries=75 first="{🎨 Improve structure / format of the code. art}" render_head="🎨 - Improve structure / format"
rm -f /tmp/smoke_test.go

# (c) Re-verify currency (Appendix E #16) — optional but recommended at land time:
curl -s "https://raw.githubusercontent.com/carloscuesta/gitmoji/master/packages/gitmojis/src/gitmojis.json" -o /tmp/g.json
python3 plan/005_c38aa48290f0/P1M2T1S2/research/regenerate_table.py /tmp/g.json | head -1
# Expected: "// count=75". If != 75 → update GitmojiTable + gitmojiVerifiedCount + gitmojiVerifiedDate.
```

### Level 4: Cross-cutting / Regression

```bash
# Whole repo — the new prompt symbols must not break any other package (nothing imports them yet; that's fine —
# exported symbols are exempt from `unused`):
go test ./...
golangci-lint run ./...
go vet ./...

# Expected: all green. No file outside internal/prompt/ is touched, so cross-package regressions are impossible
# by construction (additive-only: one new file + one new test file).
```

## Final Validation Checklist

### Technical Validation
- [ ] `gofmt -w internal/prompt/gitmoji.go internal/prompt/gitmoji_test.go` → no diff (emoji runes preserved).
- [ ] `go build ./...` clean.
- [ ] `go vet ./internal/prompt/...` clean.
- [ ] `golangci-lint run ./internal/prompt/...` clean (exported symbols dodge `unused`; consts used by tests).
- [ ] `go test ./...` passes (new tests green; existing prompt tests unaffected).

### Feature Validation
- [ ] `prompt.GitmojiTable` has exactly `gitmojiVerifiedCount` (75) complete `{Emoji, Description, Name}` entries.
- [ ] All `Emoji` unique; all `Name` unique; no empty field in any entry.
- [ ] `prompt.RenderGitmojiTable()` returns `len(GitmojiTable)` lines, `<emoji> - <description>`, no trailing newline.
- [ ] Header comment records source URL + count + `gitmojiVerifiedDate` (FR-D5 / Appendix E #16).
- [ ] OFFLINE: `grep net/http|os.ReadFile|go:embed internal/prompt/gitmoji.go` → none (FR-F3).
- [ ] Appendix E #16 re-verify run: regenerated count == 75 (or consts updated if drifted).

### Code Quality Validation
- [ ] Matches `internal/prompt` conventions: constants/tables carry no trailing newline; the renderer owns line
      topology; doc comments cite PRD §9.19 FR-F3 / §17.8; pure + defensive.
- [ ] `Gitmoji`/`GitmojiTable`/`RenderGitmojiTable` EXPORTED (contract OUTPUT + `unused`-safe pre-S3); drift
      consts UNEXPORTED (internal metadata, tested via `package prompt` internal test).
- [ ] The 75-entry literal is the GENERATED body (research/gitmoji_table.go.txt), not hand-transcribed.
- [ ] File placement: two NEW files only (`gitmoji.go`, `gitmoji_test.go`); no edits elsewhere.

### Scope Boundaries (do NOT cross)
- [ ] No config/flag/env/git-config changes (S1 owns `cfg.Format`; "gitmoji" already a valid mode).
- [ ] No prompt scaffold / locale line / `Build*` swap wiring (S3 — P1.M2.T1.S3); no edits to system.go /
      payload.go / planner.go / stager.go / arbiter.go.
- [ ] No `go.mod` change (no new dependency; no `//go:embed`, no JSON lib).
- [ ] No docs changes (Mode A: internal constant; user-facing gitmoji docs ride with S3).
- [ ] No runtime network/file/embed fetch (FR-F3).

### Documentation & Deployment
- [ ] Self-documenting: `Gitmoji` field doc comments + `GitmojiTable` header (source/count/date/refresh procedure).
- [ ] No new env vars / flags / config keys (this subtask introduces none).

---

## Anti-Patterns to Avoid

- ❌ Don't hand-transcribe the 75 entries — paste the generated body from `research/gitmoji_table.go.txt`
  (regenerated from the canonical JSON; eliminates transcription defects).
- ❌ Don't use `//go:embed` or `net/http` or `os.ReadFile` for the JSON — FR-F3 forbids ANY runtime fetch/embed;
  the table is compile-time string literals only.
- ❌ Don't leave `Gitmoji`/`GitmojiTable`/`RenderGitmojiTable` UNEXPORTED — the `unused` linter fails the build
  on unconsumed unexported symbols, and S3 (the consumer) is a later task. Export them (they're the contract
  OUTPUT anyway).
- ❌ Don't write the test as `package prompt_test` — the unexported drift consts (`gitmojiVerifiedCount`,
  `gitmojiVerifiedDate`) are only reachable from `package prompt` (internal test; matches every existing
  `*_test.go` in the dir).
- ❌ Don't hardcode the literal `75` or `"2026-07-02"` in tests — assert against the CONSTS so a refresh edits
  one place and the drift test fails correctly.
- ❌ Don't strip U+FE0F variation selectors from the 19 VS emojis (⚡️ 🚑️ ✈️ …) — they change the glyph; leave
  them verbatim (gofmt preserves multibyte runes; uniqueness is byte-wise and still holds).
- ❌ Don't add a trailing newline to `RenderGitmojiTable()`'s output — package convention: renderers carry no
  trailing newline; the assembler (S3) owns inter-block placement.
- ❌ Don't edit `system.go`/`payload.go`/`planner.go`/`arbiter.go`/`stager.go` or any `Build*` function, and
  don't touch `internal/config/*` or `internal/cmd/*` — those are S3's and S1's scopes respectively.
- ❌ Don't add user-facing docs — Mode A: this is an internal constant; gitmoji docs ride with S3.

---

## Confidence Score

**9/10** for one-pass implementation success. The task is a self-contained data file: one new `gitmoji.go`
(exported struct + a paste-ready 75-entry generated literal + a ~10-line pure renderer + 3 drift consts) and one
new `gitmoji_test.go` (six small property tests). The canonical data is already fetched and generated
(`research/gitmoji_table.go.txt`, zero transcription risk), the source/count/date are pinned in
`external_deps.md §5`, the package conventions are documented from `system.go`/`payload.go`, and the linter
constraint (export → `unused`-safe pre-S3) + the test-package constraint (`package prompt` for unexported consts)
are both explicitly flagged. The −1 is two small risks: (1) the Appendix E #16 re-verify could find the upstream
count has drifted from 75 by land-time — fully mitigated by the documented regenerate-and-update-consts flow and
the self-checking `TestGitmojiTable_Count`; and (2) emoji variation-selector bytes are easy to "helpfully"
normalize — mitigated by the explicit gotcha + the uniqueness test (byte-wise). The task is disjoint in FILES
from the in-flight S1 (config/cmd) and S3 (prompt scaffolds) — no merge conflict either way.
