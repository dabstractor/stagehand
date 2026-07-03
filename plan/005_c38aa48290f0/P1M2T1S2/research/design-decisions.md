# Design decisions — P1.M2.T1.S2 (compiled-in gitmoji reference table)

Research date: **2026-07-02**. Canonical source verified live: 75 entries, all unique emojis + names.
Raw JSON saved as `gitmojis_canonical.json`; the ready-to-paste Go slice literal is `gitmoji_table.go.txt`
(generated from that JSON — zero transcription risk); `regenerate_table.py` re-runs the generation for a
future refresh (FR-D5 discipline).

## 1. Hardcoded Go slice literal — NOT `//go:embed`

The work-item contract says "a Go **constant table** … compiled into the binary." A `var GitmojiTable =
[]Gitmoji{…}` of literal struct values is the most direct reading: it is a true compile-time constant, has
zero runtime parse cost, adds no extra asset file, and needs no new dependency (go.mod has no JSON-in-binary
story today and should not grow one for a static table). `//go:embed` of the JSON would compile into the
binary too (so it also satisfies FR-F3's "no network fetch, ever"), but it (a) re-parses 75 entries at every
process start, (b) ships a separate JSON asset that can drift from the code, and (c) contradicts "constant
table." Decision: **hardcoded Go literals**, generated from the canonical JSON to eliminate hand-transcription
errors (the generator + paste-ready output live in this research dir).

## 2. Three fields: Emoji, Description, Name

The contract: "emoji + description, keep `name` as the stable key." `Name` (the `:shortcode:` minus colons,
e.g. `art`, `t-rex`) is kept even though the PROMPT renders only emoji + description, because:
- it is the STABLE identifier across gitmoji versions (emoji glyphs and description wording drift; the
  `name` key does not) — the right anchor for the refresh process and for any future per-entry reference;
- it makes the data self-documenting (a reader of `gitmoji.go` sees `Name: "t-rex"` and instantly knows which
  entry is which, even before parsing the emoji glyph).

`Entity` (HTML entity) and `code` (`:name:`) and `semver` are dropped — none are rendered and none are needed
as keys (name already covers the identifier role). Dropping them keeps the constant minimal and the file small.

## 3. Exported `GitmojiTable` + `RenderGitmojiTable()` + `Gitmoji` struct

All three are EXPORTED for two reasons:
- the contract OUTPUT is "`prompt.GitmojiTable` (or equivalent)" — it is the public surface S3 consumes;
- **the `unused` linter (`.golangci.yml` enables it) does NOT flag exported symbols.** S2 ships BEFORE S3
  consumes the table; if `GitmojiTable`/`RenderGitmojiTable`/`Gitmoji` were unexported and referenced only by
  a future S3, `go build` + `golangci-lint` would fail today with "declared but not used." Exporting sidesteps
  that entirely (exported = part of the package API = exempt from `unused`). This is the load-bearing reason
  to export, independent of the contract name.

`GitmojiTable` is a package-level `var` slice. It is READ-ONLY by convention (documented on the declaration);
callers iterate / render, never mutate. A defensive copy-returning accessor is YAGNI for a static 75-entry
table rendered ~once per generation inside an internal package — `var` is the idiomatic Go shape for static
config/lookup data (matches how the repo treats other constant tables).

## 4. `RenderGitmojiTable()` — where S2 ends and S3 begins

PRD §17.8 says the gitmoji scaffold is *"Begin the subject with exactly ONE emoji …"* **followed by the
compiled-in gitmoji reference table (emoji + meaning)**. S2 owns the **table** (data + the canonical
"emoji - description" one-line-per-entry renderer); S3 owns the **scaffold** (the instruction sentence, the
locale line, and wiring the swap into `BuildSystemPrompt`/`BuildFallbackPrompt`/`BuildUserPayload`).

`RenderGitmojiTable()` produces the §17.8 "emoji + meaning" block in its recommended form — one line per
entry, `<emoji> - <description>` (e.g. `🎨 - Improve structure / format of the code.`). This is:
- the natural, token-cheap, LLM-readable rendering of "emoji + meaning";
- a PURE, independently-testable function (S3 does not need to unit-test table rendering — it just embeds it);
- consistent with the package convention that assembly/newline-placement lives in exactly one auditable place
  (system.go's `BuildSystemPrompt`, payload.go's `BuildUserPayload`): here `RenderGitmojiTable` owns the
  table's line topology, so the 75-entry rendering is not scattered across S3's scaffold assembly.

S3 is FREE to ignore `RenderGitmojiTable()` and iterate `GitmojiTable` directly if it wants a different
separator — the two are additive, never conflicting. The recommended path is `RenderGitmojiTable()`.

## 5. Drift-check constants — `gitmojiVerifiedCount` / `gitmojiVerifiedDate`

Two unexported consts in the header block: `gitmojiVerifiedCount = 75` and `gitmojiVerifiedDate = "2026-07-02"`.
The test `TestGitmojiTableCount` asserts `len(GitmojiTable) == gitmoviVerifiedCount`. This is the FR-D5
"record the verification date" + Appendix E #16 "re-verify currency at implementation time" made executable:
on a refresh, the implementer updates the data AND both consts; if they update the data but forget the const,
the test fails — a self-checking drift signal. They are unexported (internal-only) and reachable from tests
because every `*_test.go` in `internal/prompt` is `package prompt` (internal test — verified; same shape as
the existing `multilineRuleAllow` const tested directly in system_test.go).

## 6. The uniqueness + completeness invariants (the contract tests)

Contract: "asserts non-empty, unique emojis, and that every entry has emoji+description." Implemented as
table-driven property tests over `GitmojiTable`:
- non-empty: `len(GitmojiTable) > 0` AND `== gitmojiVerifiedCount`;
- unique emojis: `len(set(emoji)) == len(GitmojiTable)` (verified live: 75/75 unique);
- unique names: same check on `Name` (name is the stable KEY — a duplicate key is a real defect, caught here);
- completeness: every entry has non-empty Emoji, Description, AND Name (verified live: 0 empty of each);
- `RenderGitmojiTable()`: non-empty, line count == `len(GitmojiTable)`, contains a known emoji (🎨) and a
  known description substring, and every table emoji appears in the rendered output.

## 7. Scope fence — what S2 does NOT do

- NO config/flag/env/git-config changes (S1 owns `cfg.Format`; "gitmoji" is already in S1's `validFormats` —
  confirmed landed in `internal/config/load.go:350` and tested in `git_test.go`). S2 is INPUT-less (a leaf
  data subtask) and must not re-touch the config cascade.
- NO prompt scaffold / locale line / swap wiring (S3 — P1.M2.T1.S3). S2 ships data + a table renderer; it
  does not modify `system.go`/`payload.go`/`planner.go`/`stager.go`/`arbiter.go` or any `Build*` function.
- NO docs (Mode A: none — internal constant; user-facing gitmoji docs ride with S3).
- NO network fetch at runtime (FR-F3); the table is a compile-time literal. (Implementation-time re-verify
  via `regenerate_table.py` is a build/maintenance action, not a runtime behavior.)
- NO `go.mod` change (no new dependency; no `//go:embed`, no JSON lib).

## 8. Why no external-research subagent was needed

`architecture/external_deps.md §5` (verified 2026-07-02) already nails the canonical source URL, the field
set, and the count; the live fetch confirmed 75 entries + the three invariants; the gitmoji spec is a single
flat JSON array with no subtle protocol/semantics to research. The only research artifact worth materializing
was the generated Go literal + the regenerator — done in this directory.
