---
name: "P1.M1.T1.S4 — Bootstrap config template + docs/configuration.md (token_limit + diff_context)"
description: |
  Docs-and-template subtask (final step of the P1.M1.T1 config-plumbing chain). Surface the two new
  diff-payload knobs (`token_limit`, `diff_context`) to the USER-FACING surfaces — the commented
  `[generation]` template emitted by `config init` and the docs reference — using the field semantics
  S1 landed (TokenLimit plain int, default 0) and S2 corrected (DiffContext *int, default 1).

  TWO files only:
  (1) `internal/config/bootstrap.go` — extend the `generationCommented` raw-string constant (the
      commented `[generation]` block appended to every `config init` / first-run bootstrap output).
      Add `# token_limit = 0` + `# diff_context = 1` lines with FR3d/FR3f explanations, and annotate
      the existing `max_diff_bytes` / `max_md_lines` lines with "ignored when token_limit is set (FR3d)".
  (2) `docs/configuration.md` — ⚠️ the work item writes "docs/CONFIGURATION.md" but the ACTUAL file on
      this Linux checkout is LOWERCASE `docs/configuration.md` (verified; no CONFIGURATION.md exists).
      Add the two keys to the "Built-in defaults" table + the "Git-config keys" table (S3 added
      `stagehand.tokenLimit`/`stagehand.diffContext` and explicitly deferred their docs to S4), add a
      focused explanation paragraph (FR3d/FR3f + mutual-exclusivity), and update the "Populated config"
      worked example that shows the `[generation]` block.

  S1/S2 are LANDED (verified: config.go:81 TokenLimit int, :82 DiffContext *int, :11 intPtr,
  Defaults() seeds :174/:175). S3 (git-config resolver keys) is being implemented IN PARALLEL — S4
  documents the keys S3 wires; treat S3's PRP as a contract. NO Config/struct/file/overlay/git.go
  edits (owned by S1/S2/S3). NO diff-function/consumer edits (P1.M1.T2 / P1.M2+).
---

## Goal

**Feature Goal**: Make the two new `[generation]` knobs (`token_limit`, `diff_context`) discoverable and
self-documenting on every user-facing config surface: (a) the commented `[generation]` template block that
`stagehand config init` (and the first-run bootstrap fallback) emits, and (b) `docs/configuration.md`'s
reference tables + worked example — so a user who opens a freshly-bootstrapped config or the docs sees the
two new keys with correct FR3d/FR3f semantics, correct defaults (0 / 1), and the mutual-exclusivity rule
(token_limit > 0 supersedes the legacy caps).

**Deliverable**: (1) An updated `generationCommented` constant in `internal/config/bootstrap.go`
(bootstrap.go:282-296) — two new commented lines + two annotated legacy-cap lines, matching the existing
column-alignment style. (2) An updated `docs/configuration.md` — two new rows in the "Built-in defaults"
table, two new rows in the "Git-config keys" table, one explanatory paragraph (token budget + diff
context), and the two new keys added to the "Populated config" worked example's `[generation]` block.
No new files.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green
(the existing bootstrap tests use `strings.Contains` + a TOML-validity parse — new *commented* lines do
not affect either, verified). `grep` of `GenerateBootstrapConfig("")` output (or `stagehand config init`
stdout) shows both `token_limit` and `diff_context`; `docs/configuration.md` greps show both keys in the
defaults table, the git-config table, and the worked example. `git diff --stat` shows ONLY
`internal/config/bootstrap.go` + `docs/configuration.md`.

## User Persona

**Target User**: The Stagehand user opening a freshly-bootstrapped `config.toml` (from `config init` or the
first-run auto-write) or reading `docs/configuration.md` to tune the diff payload — and the contributor
landing P1.M2+ (`-U<diff_context>`, the `token_limit` water-fill) who needs the user-facing knobs to match
the Config fields S1/S2 landed.

**Use Case**: A user wants the diff payload to always fit a 120k-context model. They open their config, see
`# token_limit = 0  # holistic token budget ... 0 = unset ⇒ use the legacy caps above ... (FR3d)`,
uncomment it, and set `token_limit = 120000`. They also see `# diff_context = 1  # 0 = changed lines only
... 3 = git's default (FR3f)` and leave it at the default. They never have to read the PRD.

**Pain Points Addressed**: Without S4, the two knobs S1/S2 added to `Config` (and S3 wired to git-config)
are **invisible in the shipped template and docs** — `config init` emits neither key, and
`docs/configuration.md` documents neither. A user has no way to discover them short of reading Go source.

## Why

- **Closes the config-plumbing chain's last gap (the USER-FACING layer).** S1 added the Config fields;
  S2 wired file→Config (materialize/overlay, with the `*int` correction that makes an explicit
  `diff_context = 0` survive); S3 wired the git-config layer (`stagehand.tokenLimit`/`stagehand.diffContext`,
  precedence layer 5). S4 is the bootstrap template + docs — the layer a *user* actually reads. Without it,
  the chain is internally complete but externally invisible.
- **S3's explicit downstream hook.** S3's PRP (Integration Points → DOWNSTREAM HOOKS) states verbatim:
  "S4 (bootstrap/docs): document `stagehand.tokenLimit` / `stagehand.diffContext` in the user-facing key
  reference + the bootstrap template. S3 is the internal resolver; S3 adds NO docs." S4 fulfills that hook
  (the Git-config keys table rows in configuration.md are precisely "the user-facing key reference" S3
  names). The work item's point 5 reinforces: "THIS subtask IS the docs update for these two keys ... No
  separate docs subtask for these keys" — i.e. ALL docs for `token_limit`/`diff_context` (file table +
  git-config table + worked example) land in S4.
- **The bootstrap template is the de-facto config reference for most users.** `config init` is the first
  thing a user runs (FR-B1/B3, incl. first-run auto-write). If the two knobs are absent from its output,
  they effectively don't exist for the majority of users.
- **Lowest-risk, docs-and-strings-only change.** No behavior, no parsing, no precedence logic touched.
  The bootstrap constant is a raw string (comments only); the docs are markdown. The only correctness
  gates are (a) the right key names/defaults/FR-refs and (b) `config init` still emitting valid TOML
  (trivially true — all new lines are `#` comments).

## What

### bootstrap.go (`generationCommented`, lines 282-296)

Extend the commented `[generation]` block. The block currently lists 8 keys (`max_diff_bytes`,
`max_md_lines`, `max_duplicate_retries`, `subject_target_chars`, `output`, `strip_code_fence`,
`max_commits`, `binary_extensions`). S4:

1. **Annotate** the existing `max_diff_bytes` and `max_md_lines` inline comments with
   `"; ignored when token_limit is set (FR3d)"` (append to each line's existing `# ...` comment).
2. **Add** `# token_limit = 0` immediately AFTER `max_md_lines` (so its "use the legacy caps *above*"
   wording reads correctly — the two legacy caps sit directly above it). Comment = the holistic token
   budget explanation (FR3d).
3. **Add** `# diff_context = 1` immediately after `token_limit`. Comment = the 0/1/3 context-width
   explanation (FR3f).
4. Preserve the existing column alignment (`=` aligned at column 25; the inline `#` value-comment aligned
   to 8-char value field). gofmt does NOT touch raw string literals, so the spacing must be hand-aligned
   to match the surrounding lines.

### docs/configuration.md (4 edits)

1. **"Built-in defaults" table** — add two rows (`token_limit` default `0`; `diff_context` default `1`),
   placed after the `max_md_lines` row, before `max_duplicate_retries`.
2. **Git-config keys table** — add two rows (`stagehand.tokenLimit`; `stagehand.diffContext`), placed
   after `stagehand.stripCodeFence` (the last generation-adjacent key) or grouped with the generation
   keys. (Satisfies S3's downstream hook; point 5 = "THE docs update for these two keys".)
3. **Explanatory paragraph** — add a focused "Token budget & diff context" note after the "Built-in
   defaults" table's output/strip_code_fence paragraph: token_limit (FR3d holistic budget, the
   mutual-exclusivity rule, no per-model registry), diff_context (FR3f 0/1/3).
4. **"Populated config" worked example** — the `[generation]` block snippet under "File format" currently
   shows `max_diff_bytes` (without `max_md_lines`) + `exclude`/`format`/`locale`/`template` + `# ...`.
   Add `max_md_lines`, `token_limit`, `diff_context` right after `max_diff_bytes` and annotate
   `max_diff_bytes`/`max_md_lines` with the FR3d note, so the worked example reflects the new template.

### Success Criteria

- [ ] `generationCommented` (bootstrap.go) contains `# token_limit           = 0` and
      `# diff_context          = 1` (commented, correct defaults).
- [ ] The `max_diff_bytes` and `max_md_lines` lines in `generationCommented` each carry the annotation
      `ignored when token_limit is set (FR3d)`.
- [ ] `token_limit`'s comment explains it is the holistic token budget and that `0 = unset ⇒ use the
      legacy caps above` (FR3d); `diff_context`'s comment explains `0`/`1`/`3` (FR3f).
- [ ] `docs/configuration.md` "Built-in defaults" table has `token_limit` (default `0`) and
      `diff_context` (default `1`) rows.
- [ ] `docs/configuration.md` "Git-config keys" table has `stagehand.tokenLimit` and
      `stagehand.diffContext` rows.
- [ ] `docs/configuration.md` has an explanatory paragraph covering both keys with FR3d/FR3f cross-refs
      and the mutual-exclusivity rule.
- [ ] `docs/configuration.md` "Populated config" worked example's `[generation]` block shows both new keys.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green (incl. the existing
      `TestBuildBootstrapConfig_*` and `TestGenerateBootstrapConfig_*` — all `strings.Contains`-based).
- [ ] `grep` of `GenerateBootstrapConfig("")` (or `config init` output) shows both keys.
- [ ] ONLY `internal/config/bootstrap.go` + `docs/configuration.md` change (git diff --stat confirms).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the EXACT current `generationCommented` body (verbatim, the
before-state), the EXACT target body (the after-state, ready to paste as a whole-constant replacement),
the exact column-alignment rule (column 25 for `=`; 8-char value field — derived byte-for-byte from the
existing lines), the EXACT docs edits (per-table row text + the explanatory paragraph + the worked-example
block), and the verified landed state of S1/S2 (TokenLimit/DiffContext/intPtr + Defaults seeds at the
cited line numbers). The "CONFIGURATION.md vs configuration.md" case gotcha is flagged explicitly
(verified: the file is lowercase on this checkout). The S3-in-parallel dependency is documented (S4
documents the git-config keys S3 wires; treat S3 as a contract).

### Documentation & References

```yaml
# MUST READ — the S3 contract S4 depends on (git-config keys S4 must document)
- docfile: plan/007_b33d310438c6/P1M1T1S3/PRP.md
  why: "S3 adds stagehand.tokenLimit / stagehand.diffContext to loadGitConfig (git.go). S3's DOWNSTREAM
        HOOK (Integration Points) states verbatim: 'S4 (bootstrap/docs): document
        stagehand.tokenLimit / stagehand.diffContext in the user-facing key reference + the bootstrap
        template. S3 is the internal resolver; S3 adds NO docs.' S4's Git-config keys table rows are the
        fulfillment of that hook."
  critical: "S3 is being implemented IN PARALLEL. Treat S3's PRP as a CONTRACT — assume stagehand.tokenLimit
        and stagehand.diffContext WILL exist in the resolver. If S3 is dropped, drop the two Git-config
        keys table rows (they'd document non-existent keys); the bootstrap + defaults-table + worked-example
        edits are independent of S3 and stay. Keys are CAMELCASE (tokenLimit/diffContext), matching the
        existing stagehand.maxDiffBytes / stripCodeFence convention."

# MUST READ — the field semantics S4 surfaces (S1/S2 LANDED — verified in working tree)
- docfile: plan/007_b33d310438c6/P1M1T1S1/PRP.md
  why: "S1 added the Config fields + Defaults seeds. S1's contract: TokenLimit plain int default 0 (FR3d
        unset ⇒ legacy caps); DiffContext default 1 (FR3f -U1). S2 later upgraded DiffContext to *int
        (nil = unset; *0 = changed-lines-only) — but the USER-FACING default is still 1 in both cases."
  critical: "The user-facing defaults S4 must emit are token_limit=0 and diff_context=1 (NOT *int — that's
        an internal Config concern; the template/docs show the concrete default value 1). The FR3d/FR3f
        cross-refs are mandatory in both the template comments and the docs."

# The architecture note that names this exact subtask
- docfile: plan/007_b33d310438c6/architecture/diff_capture_touchmap.md
  why: "§3 last block: 'config init template (internal/config/bootstrap.go:282-298)' + 'Add: commented
        token_limit and diff_context lines; annotate legacy caps as ignored when token_limit is set.'
        This IS the S4 contract (the touchmap's line range is slightly stale — actual is 282-296)."
  critical: "The touchmap cites bootstrap.go:282-298; the ACTUAL generationCommented constant is
        bootstrap.go:282-296 (verified via grep). Use the real range."

# The file under edit #1
- file: internal/config/bootstrap.go
  why: "EDIT generationCommented (lines 282-296), the commented [generation] raw-string constant appended
        to every bootstrap output (buildBootstrapConfig → b.WriteString(generationCommented) at line 229).
        It is the LAST thing written in a generated config."
  pattern: "Raw string literal `const generationCommented = \\`...\\``. Each key line is
        `# <key padded to col 25>= <value padded to 8-char field># <inline comment>`. gofmt does NOT
        reformat raw strings → hand-align to the neighbors."
  gotcha: "(1) Every line is a `#` comment — adding lines CANNOT break TOML validity (TestBuildBootstrapConfig_ValidTOML stays green). (2) The existing tests assert via strings.Contains (config_version, provider, reasoning, etc.) — NONE assert on [generation] key text, so adding lines is invisible to them. (3) Preserve the §16.2 header comment block above the keys verbatim."

# The file under edit #2 — NOTE THE LOWERCASE NAME
- file: docs/configuration.md
  why: "EDIT: (a) 'Built-in defaults' table (+2 rows), (b) 'Git-config keys' table (+2 rows), (c) explanatory
        paragraph after the defaults table, (d) 'Populated config' worked-example [generation] block."
  pattern: "Markdown tables are `| Option | Default | Source |` (defaults table) and
        `| Key | Type | Reads with | Description |` (git-config table). Match the existing row text shape."
  gotcha: "CRITICAL CASE GOTCHA: the work item and the touchmap say 'docs/CONFIGURATION.md' (UPPERCASE), but
        the ACTUAL file on this Linux checkout is LOWERCASE 'docs/configuration.md' (verified — no
        CONFIGURATION.md exists; `find` returns only ./docs/configuration.md). Edit the lowercase file. On a
        case-insensitive FS this is moot; on Linux it is load-bearing. Do NOT create a new CONFIGURATION.md."

# Verified landed state (READ-ONLY — do NOT edit in S4)
- file: internal/config/config.go
  why: "READ-ONLY. S1/S2 own it. Line 11 (intPtr), 81 (TokenLimit int), 82 (DiffContext *int), 174 (TokenLimit:0
        seed), 175 (DiffContext: intPtr(1) seed). S4 READS these to confirm the user-facing defaults (0 / 1);
        does NOT modify config.go."

# Bootstrap tests (READ-ONLY — confirm they stay green)
- file: internal/config/bootstrap_test.go
  why: "READ-ONLY. TestBuildBootstrapConfig_Pi/GeminiStagerFallback/OtherInstalledCommented/NoInstallFallback/
        ValidTOML + TestGenerateBootstrapConfig_AutoDetectPi/NamedProvider all use strings.Contains on
        non-[generation] substrings + TestBuildBootstrapConfig_ValidTOML parses the output as TOML. Adding
        commented lines to generationCommented changes NEITHER (comments are TOML-inert; no test asserts on
        [generation] key text). S4 adds NO new bootstrap test (the grep validation is sufficient — there is
        nothing behavioral to assert; the keys are inert comments)."
  gotcha: "If you DO add an assertion, use strings.Contains for the two new keys — do NOT assert exact
        full-output text (brittle, and the populated config varies by detected provider)."

# PRD authority
- prd: PRD.md §9.1 FR3d (token_limit, default 0 = unset ⇒ legacy per-section caps; non-zero supersedes BOTH
        legacy caps; model-agnostic — user sets their model's context window, no per-model registry) +
        FR3f (diff_context, integer 0–3, default 1; 0 = changed-lines-only; 3 = git default; applies in every
        diff path); §9.17 FR-B1/B2 (config init writes a populated config; --template retains the inert
        all-commented reference); §16.1 layer-1 defaults (token_limit 0, diff_context 1); §16.2 full config
        example (the authoritative wording for both keys' inline comments).
  why: "FR3d is the authority for the '0 = unset ⇒ legacy caps' + mutual-exclusivity wording and the
        'no per-model context registry' rationale. FR3f is the authority for the 0/1/3 wording. §16.2 is the
        canonical phrasing to mirror in the template comments (kept concise for the template; fuller in the
        docs paragraph)."
  critical: "The mutual-exclusivity rule (a non-zero token_limit supersedes BOTH max_diff_bytes AND
        max_md_lines for that run; the two modes are mutually exclusive) MUST appear in the docs paragraph —
        it is the single most-misunderstood aspect of FR3d and is why the legacy-cap annotations exist."
```

### Current Codebase Tree (relevant slice)

```bash
stagehand/
├── internal/config/
│   ├── bootstrap.go          # EDIT: generationCommented (282-296) +token_limit +diff_context lines
│   ├── config.go             # READ-ONLY (S1/S2): TokenLimit (81), DiffContext *int (82), intPtr (11), seeds (174-175)
│   ├── git.go                # READ-ONLY (S3, in parallel): +stagehand.tokenLimit +stagehand.diffContext
│   ├── file.go               # READ-ONLY (S2): materialize/overlay
│   └── bootstrap_test.go     # READ-ONLY: strings.Contains-based; stays green (new lines are comments)
└── docs/
    └── configuration.md      # EDIT (lowercase!): defaults table + git-config table + paragraph + worked example
```

### Desired Codebase Tree After S4

```bash
stagehand/
└── (only existing files modified — no new files)
    internal/config/bootstrap.go     # generationCommented +2 key lines +2 annotated legacy-cap lines
    docs/configuration.md            # +2 defaults-table rows +2 git-config-table rows +1 paragraph +worked-example keys
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/config/bootstrap.go` | MODIFY | Extend `generationCommented`: add `token_limit`/`diff_context` commented lines + annotate `max_diff_bytes`/`max_md_lines`. |
| `docs/configuration.md` | MODIFY | Document the two keys in the defaults table, git-config table, an explanatory paragraph, and the worked example. |

**Explicitly NOT touched**: `config.go` (S1/S2), `file.go` (S2), `git.go` (S3), `load.go`, `bootstrap_test.go`
(no new test needed — grep validation suffices), `StagedDiffOptions` + 6 call sites (P1.M1.T2), the diff
functions / `-U<diff_context>` (P1.M2+), any other doc file (`docs/cli.md`, `docs/providers.md`,
`docs/how-it-works.md` — those are P1.M5.T1), any other package, `PRD.md`, `tasks.json`, `prd_snapshot.md`,
`plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (case sensitivity): the work item + touchmap say "docs/CONFIGURATION.md" (UPPERCASE), but the
// ACTUAL file is LOWERCASE docs/configuration.md on this Linux checkout (verified: `find` returns only
// ./docs/configuration.md; no CONFIGURATION.md exists). Edit docs/configuration.md. Do NOT create a new
// uppercase file. (On macOS/Windows this is case-insensitive; on the Linux CI/build box it is not.)

// CRITICAL (raw-string alignment): generationCommented is a Go raw string literal (`...`). gofmt does NOT
// reformat the INSIDE of raw strings. The existing lines hand-align the `=` at column 25 and the inline `#`
// value-comment at an 8-char value field. New lines MUST match (hand-aligned) or the block looks ragged.
// Rule: `# ` (2) + key + N spaces so key+spaces = 22 chars (→ `=` at col 25) + `= ` + value + M spaces so
// value+spaces = 8 chars (→ `#` at col 35) + `# <comment>`. token_limit(11)→11 pad spaces; diff_context(12)
// →10. Values "0"/"1"(1 char)→7 pad spaces.

// CRITICAL (TOML validity is unaffected): every new/edited line in generationCommented is a `#` comment.
// Comments are TOML-inert. TestBuildBootstrapConfig_ValidTOML (parses bootstrap output as TOML) stays green
// by construction. Do NOT add any UNcommented key (that would change Defaults behavior — out of scope).

// CRITICAL (existing bootstrap tests stay green): bootstrap_test.go asserts via strings.Contains on
// non-[generation] substrings (config_version, `provider = "pi"`, `reasoning = "off"`, stager-fallback
// annotations, "=== claude (installed)", env-var header lines). NONE assert on [generation] key text.
// Adding commented lines to generationCommented is invisible to all of them. (Verified by reading the file.)

// GOTCHA (placement of token_limit AFTER max_md_lines): token_limit's comment says "0 = unset ⇒ use the
// legacy caps ABOVE". For "above" to be truthful, max_diff_bytes AND max_md_lines must both sit above
// token_limit. They do in the current block (max_diff_bytes 288, max_md_lines 289). Place token_limit at
// line 290 (current max_duplicate_retries position) and shift max_duplicate_retries down. Do NOT place
// token_limit above max_diff_bytes.

// GOTCHA (S3-in-parallel dependency for the Git-config table rows ONLY): S3 wires stagehand.tokenLimit /
// stagehand.diffContext into git.go's loadGitConfig. S4's Git-config keys table rows describe those keys.
// The bootstrap + defaults-table + worked-example edits are INDEPENDENT of S3. ONLY the 2 Git-config table
// rows depend on S3 landing. Treat S3's PRP as a contract (it WILL land). camelCase keys (tokenLimit /
// diffContext), NOT snake_case.

// GOTCHA (DiffContext is *int internally; the user-facing default is the concrete 1): S2 made Config.DiffContext
// a *int so overlay can tell "unset" from "explicit 0". That is an INTERNAL concern. The bootstrap template +
// docs show the concrete DEFAULT VALUE (diff_context = 1), NOT a pointer or nil. The *int is invisible to users.

// GOTCHA (the git-config table is SELECTIVE — do NOT retrofit): docs/configuration.md's Git-config keys table
// currently OMITS even stagehand.maxDiffBytes / stagehand.maxMdLines (a pre-existing doc gap). S4 adds ONLY
// the two new keys (S3's hook + point 5). Do NOT retrofit maxDiffBytes/maxMdLines rows — that is a separate
// pre-existing gap out of S4's scope (would be scope creep + risks touching unrelated docs).
```

## Implementation Blueprint

### Data models and structure

None. This is a docs-and-template subtask. No types, no fields, no parsing. The "model" is two strings: the
`generationCommented` raw-string constant and four markdown fragments.

### The EXACT before/after for `generationCommented` (the whole-constant replacement)

**BEFORE** (bootstrap.go:283-296, verbatim current state):

```go
const generationCommented = `
# ---------------------------------------------------------------------------
# [generation] — diff capture & output tuning (PRD §16.2)
# ---------------------------------------------------------------------------
# [generation]
# max_diff_bytes        = 300000  # byte cap on the non-markdown diff section
# max_md_lines          = 100     # per-file line cap for markdown diffs
# max_duplicate_retries = 3       # re-generation attempts when the subject duplicates a recent commit
# subject_target_chars  = 50      # target subject-line length for truncation
# output                = "raw"   # agent output mode: "raw" | "json" — applies to parsing across ALL providers
# strip_code_fence      = true    # strip ` + "`" + ` fences from agent output (all providers)
# max_commits           = 12      # safety cap on auto-decompose (PRD §9.14 FR-M4); default 12
# binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
`
```

**AFTER** (the target — replace the whole constant body; new/edited lines marked):

```go
const generationCommented = `
# ---------------------------------------------------------------------------
# [generation] — diff capture & output tuning (PRD §16.2)
# ---------------------------------------------------------------------------
# [generation]
# max_diff_bytes        = 300000  # byte cap on the non-markdown diff section; ignored when token_limit is set (FR3d)
# max_md_lines          = 100     # per-file line cap for markdown diffs; ignored when token_limit is set (FR3d)
# token_limit           = 0       # holistic token budget for the WHOLE payload (prompt+examples+diff); 0 = unset ⇒ use the legacy caps above. Set to your model's context window, e.g. 120000, so the payload always fits without a per-model registry (FR3d)
# diff_context          = 1       # unchanged context lines around each hunk: 0 = changed lines only (max savings), 1 = one anchor line (default), 3 = git's default (FR3f)
# max_duplicate_retries = 3       # re-generation attempts when the subject duplicates a recent commit
# subject_target_chars  = 50      # target subject-line length for truncation
# output                = "raw"   # agent output mode: "raw" | "json" — applies to parsing across ALL providers
# strip_code_fence      = true    # strip ` + "`" + ` fences from agent output (all providers)
# max_commits           = 12      # safety cap on auto-decompose (PRD §9.14 FR-M4); default 12
# binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
`
```

> **Alignment self-check (paste this exact text; the spacing below is pre-computed to match neighbors):**
> - `token_limit` (11 chars) + **11 spaces** → `=` at column 25. Value `0` (1 char) + **7 spaces** → `#` at column 35.
> - `diff_context` (12 chars) + **10 spaces** → `=` at column 25. Value `1` (1 char) + **7 spaces** → `#` at column 35.
> - The two annotated legacy lines append `; ignored when token_limit is set (FR3d)` to their existing comments.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: bootstrap.go — replace the generationCommented constant body
  - LOCATE: internal/config/bootstrap.go lines 282-296 (const generationCommented). Confirm via:
        grep -n "generationCommented\|max_diff_bytes\|max_md_lines" internal/config/bootstrap.go
  - REPLACE the whole constant body with the AFTER block above (cleanest: replace from the line
    `# max_diff_bytes        = 300000  # byte cap on the non-markdown diff section` through the
    `# binary_extensions ...` line, leaving the header (the 3 `# ---`/`# [generation]` lines) and the
    closing backtick untouched).
  - VERIFY alignment: open the file and confirm the `=` signs and the inline `#` comment markers line up
    with the existing max_duplicate_retries / subject_target_chars / output rows. (gofmt will NOT fix raw
    strings — eyeball it.)
  - DO NOT: uncomment any key (changes Defaults behavior — out of scope). DO NOT: edit bootstrapHeader
    (the precedence/env/git/CLI docs block — separate constant, unchanged). DO NOT: touch
    buildBootstrapConfig, GenerateBootstrapConfig, writeRoleBlock, or any function.

Task 2: docs/configuration.md — "Built-in defaults" table (+2 rows)
  - LOCATE the "## Built-in defaults" table (the `| Option | Default | Source |` table). Find the
    `| \`max_md_lines\` | \`100\` | \`config.Defaults()\` |` row.
  - INSERT two rows immediately after it (before the `max_duplicate_retries` row):
        | `token_limit` | `0` | `config.Defaults()` (§9.1 FR3d — unset ⇒ legacy caps) |
        | `diff_context` | `1` | `config.Defaults()` (§9.1 FR3f — `-U1`) |
  - MATCH the existing terse 3-column shape (Option | Default | Source). The FR-ref lives in the Source
    cell (brief) — the fuller explanation is Task 4's paragraph.

Task 3: docs/configuration.md — "Git-config keys" table (+2 rows)  [DEPENDS ON S3]
  - LOCATE the "## Git-config keys" table (`| Key | Type | Reads with | Description |`). Find the
    `| \`stagehand.stripCodeFence\` | ... |` row (the last generation-adjacent key).
  - INSERT two rows immediately after it:
        | `stagehand.tokenLimit` | int | `git config --get stagehand.tokenLimit` | Holistic token budget for the whole payload; `0` = unset ⇒ legacy `max_diff_bytes`/`max_md_lines` caps (§9.1 FR3d). Supersedes both legacy caps when >0 (mutually exclusive). |
        | `stagehand.diffContext` | int | `git config --get stagehand.diffContext` | Unchanged context lines per hunk: `0` = changed-lines-only, `1` = one anchor line (default), `3` = git default (§9.1 FR3f). An explicit `0` is honored (changed-lines-only is a first-class value). |
  - WHY here: S3 added these two resolver keys; S3's downstream hook defers their docs to S4 ("the
    user-facing key reference"). This table IS that reference. camelCase keys (matching
    stagehand.stripCodeFence / autoStageAll).
  - DO NOT: retrofit stagehand.maxDiffBytes / stagehand.maxMdLines rows (pre-existing gap, out of scope).
  - S3 DEPENDENCY: these rows are correct ONLY if S3 lands. S3's PRP is a contract — assume it lands. If a
    reviewer reports S3 was dropped, remove these 2 rows (the rest of S4 is S3-independent).

Task 4: docs/configuration.md — explanatory paragraph (the "explanations + cross-refs")
  - LOCATE the paragraph immediately AFTER the "Built-in defaults" table that discusses output /
    strip_code_fence (begins "`NoColor` is TTY-aware..." or the "The `output` and `strip_code_fence`
    settings apply to **parsing**..." paragraph). INSERT a new paragraph right after that paragraph:
        > **Token budget & diff context.** Two `[generation]` knobs size and shape the diff payload:
        > - **`token_limit`** (default `0` = unset) — a holistic token budget over the **whole** agent
        >   payload (system prompt + style examples + the concatenated diff). When set (e.g. `120000`),
        >   Stagehand reserves room for the prompt/examples and truncates the diff to fit using the ≈4
        >   chars/token estimate, so the payload always fits your model's context window **without
        >   Stagehand maintaining a per-model context registry** (§9.1 FR3d). A non-zero `token_limit`
        >   **supersedes** the legacy per-section caps `max_diff_bytes` and `max_md_lines` for that run;
        >   the two modes are mutually exclusive. When `0`/unset, the legacy caps apply unchanged.
        > - **`diff_context`** (default `1`) — unchanged context lines surrounding each diff hunk: `0` =
        >   changed lines only (maximal savings), `1` = one anchor line (default), `3` = git's default
        >   (§9.1 FR3f). Applies in every diff path (staged, multi-commit snapshot, per-concept tree diff).
  - WHY: this is where the "explanations + cross-refs to FR3d/FR3f" the contract requires live in full
    (the table rows are terse; the template comments are concise; this paragraph is the complete explainer
    + the mutual-exclusivity rule, which is the most-misunderstood aspect of FR3d).

Task 5: docs/configuration.md — "Populated config" worked example [generation] block
  - LOCATE the "## File format" section, the "**Populated config**" fenced ```toml block. Find its
    `[generation]` snippet (currently shows `# max_diff_bytes = 300000` then `# exclude = []` ...).
  - REPLACE the snippet's first lines so the `[generation]` block reads (keep the existing
    `# exclude`/`# format`/`# locale`/`# template`/`# ...` lines after):
        # [generation] — diff capture and output tuning (commented defaults)
        # [generation]
        # max_diff_bytes        = 300000  # ignored when token_limit is set (FR3d)
        # max_md_lines          = 100     # ignored when token_limit is set (FR3d)
        # token_limit           = 0       # holistic token budget (0 = unset ⇒ use the caps above); FR3d
        # diff_context          = 1       # 0 = changed-lines-only, 1 = one anchor (default), 3 = git default; FR3f
        # exclude               = []   # UNIONS across layers — see "Exclusion globs" below
        # format                = "auto"   # auto|conventional|gitmoji|plain; unknown = hard error (exit 1)
        # locale                = ""       # free-form language name or BCP-47 tag; never validated
        # template              = ""       # wrap every message; must contain literal $msg, e.g. "$msg (#205)"
        # ...
  - WHY: the worked example is the "worked example that shows the [generation] block" the contract names.
    Adding max_md_lines is required so token_limit's "use the caps above" wording is coherent in the
    example too. Keep the existing `# ...` truncation marker.

Task 6: VALIDATE (the contract's "verify by running the bootstrap and grepping the output")
  - RUN: gofmt -w internal/config/bootstrap.go ; gofmt -l .  (expect empty after -w)
  - RUN: go build ./... ; go vet ./...
  - RUN: go test -race ./internal/config/ -v   (all bootstrap tests green — strings.Contains-based)
  - RUN: go test -race ./...                    (whole repo green)
  - GREP the bootstrap output for both keys (the contract's required verification):
        go test -run TestGenerateBootstrapConfig_AutoDetectPi -v ./internal/config/  # prints nothing by default
        # Authoritative: a one-liner that prints the constant's content, OR grep the source directly:
        grep -n "token_limit\|diff_context" internal/config/bootstrap.go   # → 2 keys (the 2 new commented lines)
        grep -n "ignored when token_limit is set" internal/config/bootstrap.go  # → 2 (max_diff_bytes + max_md_lines annotations)
  - GREP the docs:
        grep -n "token_limit\|diff_context" docs/configuration.md           # → defaults table + git-config table + paragraph + worked example
        grep -n "tokenLimit\|diffContext" docs/configuration.md             # → 2 (git-config table rows)
        grep -n "mutually exclusive" docs/configuration.md                  # → 1 (the explanatory paragraph)
  - CONFIRM scope: git diff --stat -- internal/ pkg/ cmd/ docs/   # → bootstrap.go + configuration.md ONLY.
```

### Implementation Patterns & Key Details

```go
// === bootstrap.go — generationCommented: the column-alignment rule (hand-aligned; gofmt won't help) ===
// The existing block aligns `=` at column 25 and the inline `#` at column 35:
//   "# " (2) + <key> + <pad> (key+pad = 22 chars) + "=" (col 25) + " " + <value> + <pad> (value+pad = 8) + "# <cmt>"
// Examples from the EXISTING block (the model to match):
//   "# max_diff_bytes        = 300000  # ..."   (14-char key + 8 pad = 22; 6-char val + 2 pad = 8)
//   "# max_duplicate_retries = 3       # ..."   (21-char key + 1 pad = 22; 1-char val + 7 pad = 8)
// New lines (pre-computed):
//   "# token_limit           = 0       # ..."   (11-char key + 11 pad = 22; 1-char val + 7 pad = 8)
//   "# diff_context          = 1       # ..."   (12-char key + 10 pad = 22; 1-char val + 7 pad = 8)

// === docs/configuration.md — table-row shape (mirror the existing rows exactly) ===
// Built-in defaults table (Option | Default | Source):
//   | `max_md_lines` | `100` | `config.Defaults()` |
//   | `token_limit` | `0` | `config.Defaults()` (§9.1 FR3d — unset ⇒ legacy caps) |   ← NEW
//   | `diff_context` | `1` | `config.Defaults()` (§9.1 FR3f — `-U1`) |               ← NEW
//
// Git-config keys table (Key | Type | Reads with | Description) — camelCase keys:
//   | `stagehand.tokenLimit` | int | `git config --get stagehand.tokenLimit` | ... FR3d ... |   ← NEW (S3 hook)
//   | `stagehand.diffContext` | int | `git config --get stagehand.diffContext` | ... FR3f ... |  ← NEW (S3 hook)
```

```markdown
<!-- docs/configuration.md — the explanatory paragraph (the contract's "explanations + cross-refs") -->
> **Token budget & diff context.** Two `[generation]` knobs size and shape the diff payload:
> - **`token_limit`** (default `0` = unset) — a holistic token budget over the **whole** agent payload
>   (system prompt + style examples + the concatenated diff). When set (e.g. `120000`), Stagehand reserves
>   room for the prompt/examples and truncates the diff to fit using the ≈4 chars/token estimate, so the
>   payload always fits your model's context window **without Stagehand maintaining a per-model context
>   registry** (§9.1 FR3d). A non-zero `token_limit` **supersedes** the legacy per-section caps
>   `max_diff_bytes` and `max_md_lines` for that run; the two modes are mutually exclusive. When `0`/unset,
>   the legacy caps apply unchanged.
> - **`diff_context`** (default `1`) — unchanged context lines surrounding each diff hunk: `0` = changed
>   lines only (maximal savings), `1` = one anchor line (default), `3` = git's default (§9.1 FR3f). Applies
>   in every diff path (staged, multi-commit snapshot, per-concept tree diff).
```

### Integration Points

```yaml
BOOTSTRAP TEMPLATE (internal/config/bootstrap.go generationCommented):
  - emitted by: buildBootstrapConfig → b.WriteString(generationCommented) (bootstrap.go:229)
  - surfaces in: `stagehand config init` stdout AND the first-run auto-write (FR-B3 bootstrapWriteConfig →
    GenerateBootstrapConfig) — BOTH paths flow through generationCommented. ONE edit covers both.
  - TOML validity: unaffected (all new lines are `#` comments). TestBuildBootstrapConfig_ValidTOML green.

DOCS (docs/configuration.md):
  - "Built-in defaults" table: +2 rows (token_limit/diff_context). The table that documents every config
    key's default — the natural home for the two new keys' defaults (0 / 1).
  - "Git-config keys" table: +2 rows (stagehand.tokenLimit/diffContext). S3's downstream hook. camelCase.
  - Explanatory paragraph: the full FR3d/FR3f explainer + mutual-exclusivity (placed after the defaults table).
  - "Populated config" worked example: the [generation] block snippet updated with both new keys.

NO-TOUCH (explicitly — owned by sibling/later subtasks):
  - internal/config/config.go            # S1/S2: TokenLimit int (81), DiffContext *int (82), seeds (174-175)
  - internal/config/git.go               # S3 (in parallel): the git-config resolver keys
  - internal/config/file.go, load.go     # S2 / overlay flow
  - internal/config/bootstrap_test.go    # no new test (grep validation suffices; keys are inert comments)
  - StagedDiffOptions + 6 call sites     # P1.M1.T2
  - the 3 diff functions (-U<diff_context>, token-limit gate, water-fill)   # P1.M2 / P1.M3 / P1.M4
  - docs/how-it-works.md, README.md, docs/cli.md, docs/providers.md         # P1.M5.T1 (changeset-level docs)
  - any other package; PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM (informational — owned by LATER subtasks, NOT S4):
  - P1.M2.T2.S1 injects -U<diff_context> into the 3 diff functions — consumes cfg.DiffContext (the *int S4
    documents here). S4 is the user-facing surface; the consumer lands in P1.M2.
  - P1.M4.T3.S1 gates on token_limit > 0 (supersedes legacy caps) — consumes cfg.TokenLimit. S4 documents
    the gate's user-facing contract (the mutual-exclusivity rule) so the eventual behavior matches the docs.
  - P1.M5.T1.S1 (docs/how-it-works.md diff-capture section) + P1.M5.T1.S2 (README feature surface) are the
    CHANGESSET-level docs; S4 is the CONFIG-REFERENCE-level docs. They are distinct — do not conflate.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand

gofmt -w internal/config/bootstrap.go          # raw-string interior is untouched; safe
gofmt -l .                                     # Expected: empty after the -w
go vet ./internal/config/...                   # Expected: exit 0
go build ./...                                 # Expected: exit 0

# docs/configuration.md is markdown — no compiler. Sanity-check the tables render (optional):
#   (eyeball the 4 edits; no tooling gate.)
```

### Level 2: Unit Tests — the existing bootstrap suite stays green

```bash
cd /home/dustin/projects/stagehand

# The bootstrap tests — all strings.Contains-based + one TOML-validity parse. Adding commented lines to
# generationCommented is invisible to all of them (comments are TOML-inert; no test asserts [generation] text).
go test -race -run 'TestBuildBootstrapConfig|TestGenerateBootstrapConfig' ./internal/config/ -v

# Full config suite (proves S1/S2/S3 landed cleanly and S4 didn't regress anything)
go test -race ./internal/config/ -v

# Expected: ALL PASS. If TestBuildBootstrapConfig_ValidTOML fails, an UNcommented key was accidentally
# added — re-check that every new line starts with `#`.
```

### Level 3: Whole-Repository Regression + the contract's grep verification

```bash
cd /home/dustin/projects/stagehand

go test -race ./...                             # Expected: ALL packages green
go vet ./...                                    # Expected: exit 0

# === The contract's required verification: "verify by running the bootstrap and grepping the output" ===
# (1) The bootstrap source carries both keys + the FR3d annotations:
grep -n "token_limit\|diff_context" internal/config/bootstrap.go
#   Expected: 2 matches (the 2 new commented key lines: token_limit, diff_context)
grep -n "ignored when token_limit is set" internal/config/bootstrap.go
#   Expected: 2 matches (max_diff_bytes + max_md_lines annotations)

# (2) The docs carry both keys across all 4 surfaces:
grep -n "token_limit\|diff_context" docs/configuration.md
#   Expected: ≥6 matches (defaults table + git-config table + paragraph + worked example)
grep -n "tokenLimit\|diffContext" docs/configuration.md
#   Expected: 2 matches (the camelCase git-config table rows)
grep -n "mutually exclusive" docs/configuration.md
#   Expected: 1 match (the explanatory paragraph)

# (3) Confirm ONLY the 2 intended files changed:
git diff --stat -- internal/ pkg/ cmd/ docs/
#   Expected: internal/config/bootstrap.go + docs/configuration.md ONLY.
```

### Level 4: End-to-End Smoke (the contract's "run the bootstrap" — a real config init)

```bash
cd /home/dustin/projects/stagehand

# Build the binary, then run the ACTUAL user-facing command into a throwaway HOME (no clobber of the
# user's real config). This is the contract's "run the bootstrap and grep the output" at the CLI surface.
go build -o /tmp/stagehand-smoke . || { echo "build failed"; exit 1; }
export HOME_SMOKE="$(mktemp -d)"
XDG_CONFIG_HOME="$HOME_SMOKE/.config" /tmp/stagehand-smoke config init   # writes ~/.config/stagehand/config.toml
echo "--- generated [generation] block ---"
grep -A14 '\[generation\]' "$HOME_SMOKE/.config/stagehand/config.toml"
echo "--- key presence ---"
grep -E 'token_limit|diff_context' "$HOME_SMOKE/.config/stagehand/config.toml"
#   Expected: both `# token_limit           = 0` and `# diff_context          = 1` present (commented).
echo "--- legacy-cap annotations ---"
grep -E 'max_diff_bytes|max_md_lines' "$HOME_SMOKE/.config/stagehand/config.toml"
#   Expected: both lines carry "ignored when token_limit is set (FR3d)".
rm -rf "$HOME_SMOKE" /tmp/stagehand-smoke

# (If a CLI smoke is undesirable in CI, the authoritative check is the Level-3 grep of bootstrap.go +
# go test TestGenerateBootstrapConfig_AutoDetectPi — both prove generationCommented carries the keys.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green (the existing bootstrap tests stay green by construction).
- [ ] `grep "token_limit\|diff_context" internal/config/bootstrap.go` → 2 matches.
- [ ] `grep "ignored when token_limit is set" internal/config/bootstrap.go` → 2 matches.
- [ ] `grep "token_limit\|diff_context" docs/configuration.md` → ≥6 matches (4 surfaces).
- [ ] `grep "tokenLimit\|diffContext" docs/configuration.md` → 2 matches (git-config table).

### Feature Validation

- [ ] `generationCommented` emits `# token_limit = 0` with the FR3d holistic-budget + "use the legacy caps
      above" explanation.
- [ ] `generationCommented` emits `# diff_context = 1` with the FR3f 0/1/3 explanation.
- [ ] The `max_diff_bytes` and `max_md_lines` lines in `generationCommented` carry "ignored when
      token_limit is set (FR3d)".
- [ ] `docs/configuration.md` "Built-in defaults" table has `token_limit` (0) + `diff_context` (1) rows.
- [ ] `docs/configuration.md` "Git-config keys" table has `stagehand.tokenLimit` + `stagehand.diffContext`
      rows (S3's downstream hook fulfilled).
- [ ] `docs/configuration.md` has the explanatory paragraph with the mutual-exclusivity rule.
- [ ] `docs/configuration.md` "Populated config" worked example's `[generation]` block shows both keys.
- [ ] `config init` (smoke) emits both commented keys in the generated config (Level 4).

### Scope Discipline Validation

- [ ] ONLY `internal/config/bootstrap.go` + `docs/configuration.md` modified (git diff --stat confirms).
- [ ] Did NOT edit `config.go` (S1/S2), `git.go` (S3), `file.go`/`load.go`, `bootstrap_test.go`.
- [ ] Did NOT uncomment any key in `generationCommented` (would change Defaults behavior — out of scope).
- [ ] Did NOT touch `StagedDiffOptions`/call sites (P1.M1.T2) or the diff functions (P1.M2+).
- [ ] Did NOT retrofit `stagehand.maxDiffBytes`/`maxMdLines` rows into the git-config table (pre-existing gap).
- [ ] Did NOT edit other doc files (`docs/how-it-works.md`, `README.md` — P1.M5.T1).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] The `generationCommented` column alignment matches the existing rows (`=` at col 25; `#` at col 35).
- [ ] The docs table rows match the existing terse 3-/4-column shapes.
- [ ] FR3d/FR3f cross-refs appear in the template comments AND the docs (table + paragraph).
- [ ] The mutual-exclusivity rule (token_limit > 0 supersedes BOTH legacy caps) appears in the docs paragraph.
- [ ] The user-facing default for `diff_context` is the concrete `1` (the internal `*int` is invisible to users).

---

## Anti-Patterns to Avoid

- ❌ Don't edit a file named `docs/CONFIGURATION.md` (UPPERCASE) — it does NOT exist on this Linux checkout.
  The actual file is LOWERCASE `docs/configuration.md` (verified via `find`). Creating an uppercase file
  leaves the real docs unchanged and produces a confusing duplicate on case-insensitive filesystems.
- ❌ Don't uncomment any key in `generationCommented` — the block is intentionally an inert commented
  reference. Uncommenting `token_limit`/`diff_context` would inject active config into every bootstrap
  output and (worse) change behavior via Defaults — far out of S4's docs-and-template scope.
- ❌ Don't rely on gofmt to align the `generationCommented` lines — it is a raw string literal; gofmt does
  NOT reformat raw-string interiors. Hand-align the `=` (col 25) and the inline `#` (col 35) to match the
  existing rows, or the block will look ragged. (Correctness of the keys/values/FR-refs is the real gate;
  alignment is polish — but the PRP provides pre-computed spacing so there's no guesswork.)
- ❌ Don't place `token_limit` ABOVE `max_diff_bytes` — its comment says "use the legacy caps ABOVE", which
  is only truthful if both `max_diff_bytes` and `max_md_lines` sit above it. Insert it AFTER `max_md_lines`.
- ❌ Don't omit the "ignored when token_limit is set (FR3d)" annotations on `max_diff_bytes`/`max_md_lines` —
  the contract (point 3a) explicitly requires them; without them a user who sets `token_limit` won't know
  the legacy caps are silently superseded.
- ❌ Don't write the docs `diff_context` default as a pointer/nil/`*int` — that is an INTERNAL Config concern
  (S2). The user-facing default is the concrete value `1`. The `*int` is invisible to users.
- ❌ Don't retrofit `stagehand.maxDiffBytes`/`maxMdLines` rows into the Git-config keys table — that table
  is already selective (it omits them pre-existing-ly). S4 adds ONLY the two NEW keys (S3's hook). Fixing
  the pre-existing gap is scope creep and risks unrelated docs.
- ❌ Don't add a brittle full-output golden test for `generationCommented` — the populated config varies by
  detected provider (pi vs claude vs gemini…), so an exact-text assertion is fragile and unnecessary. The
  existing tests are `strings.Contains`-based; the grep validation (Level 3/4) is the authoritative check.
  If a test is added, assert `strings.Contains(content, "token_limit")` / `"diff_context"` only.
- ❌ Don't edit `bootstrapHeader` (the precedence/env/git/CLI docs block) — it is a SEPARATE constant; S4
  touches only `generationCommented`. (The header's git-keys list already omits maxDiffBytes etc.; leave it.)
- ❌ Don't touch `docs/how-it-works.md` or `README.md` for these keys — those are the CHANGESSET-level docs
  owned by P1.M5.T1. S4 is the CONFIG-REFERENCE-level docs (configuration.md + the bootstrap template).
  Conflating them causes merge conflicts with P1.M5.
- ❌ Don't edit `config.go`, `git.go`, `file.go`, `load.go`, `bootstrap_test.go`, `StagedDiffOptions`, or
  the diff functions — S1/S2/S3/P1.M1.T2/P1.M2+ own those. S4 is `bootstrap.go` + `configuration.md` ONLY.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a two-file docs-and-template change with NO behavioral logic. The `generationCommented`
before/after is quoted VERBATIM with pre-computed column alignment (ready to paste as a whole-constant
replacement), so there is no guesswork on the Go side. The docs edits are specified per-table with exact
row text + the full explanatory paragraph + the exact worked-example block. The "CONFIGURATION.md vs
configuration.md" case gotcha — the single most likely implementation error — is flagged at the top, in the
YAML context, in the gotchas, and in the anti-patterns (four reinforcements). The S1/S2 dependency is NOT a
risk: it has ALREADY LANDED (verified — TokenLimit/DiffContext/intPtr + Defaults seeds at the cited line
numbers). The S3 dependency is bounded: ONLY the 2 Git-config keys table rows depend on S3 landing, and
S3's PRP is treated as a contract; the other 3 docs edits + the bootstrap edit are fully S3-independent.
The existing bootstrap tests stay green by construction (new lines are TOML-inert comments; tests are
`strings.Contains`-based and assert no `[generation]` text). The validation is grep-based and executable
in-repo. The only residual uncertainty (not 10/10) is markdown-table-fidelity taste (column widths / exact
FR-ref placement in the terse table rows) and whether the reviewer wants the optional CLI smoke (Level 4)
vs source-grep-only — both specified as acceptable. No behavior, no parsing, no precedence is touched, so
the blast radius is two files' text content.
