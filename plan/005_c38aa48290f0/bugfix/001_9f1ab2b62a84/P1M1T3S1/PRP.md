name: "P1.M1.T3.S1 — Add exclude/format/locale/template/push commented lines to exampleConfigTemplate's [generation] block"
description: |
  Fixes Issue 3 (Minor): `config init --template` writes an inert reference config whose `[generation]` section
  documents only 8 of the 13 v2.1 keys — it is MISSING `exclude`, `format`, `locale`, `template`, and `push`.
  The template's own header claims it "documents every available option", and docs/configuration.md already
  documents all five, so the template is inconsistent with both. The fix appends 5 commented (`# `) lines to the
  `exampleConfigTemplate` `[generation]` block in `internal/cmd/config.go`, mirroring the docs wording with PRD
  §-refs and one-line descriptions, preserving the existing column alignment. All lines stay commented so the
  template remains INERT. PLUS one new positive test asserting all 5 keys render (commented) after
  `config init --template`. Existing template tests are self-updating (they compare to the same Go const) and
  need NO change. This is a Mode-A doc fix: the template IS the documentation.

---

## Goal

**Feature Goal**: `config init --template` produces a reference config whose `[generation]` section lists ALL 13
keys (8 existing + `exclude`, `format`, `locale`, `template`, `push`), every line commented out (inert), with the
docs-consistent wording and PRD section refs. The template's "documents every available option" header becomes
true.

**Deliverable**: Two edits in one package (`internal/cmd`):
1. `internal/cmd/config.go` — append 5 commented lines to `exampleConfigTemplate`'s `[generation]` block (after
   `binary_extensions`, before the `NOTE:` line).
2. `internal/cmd/config_test.go` — add `TestConfigInit_Template_GenerationKeys` asserting all 5 keys render
   commented after `config init --template --config <tmp>`.

**Success Definition**:
- `config init --template --config <f>` → the generated `[generation]` section contains, as commented lines:
  `exclude`, `format`, `locale`, `template`, `push` (values `[]`, `"auto"`, `""`, `""`, `false`).
- Every new line begins with `# ` (template stays inert — no behavioral change).
- The `push` line renders the literal `` `git push` `` code span correctly (backtick split wired right).
- `go build ./...`, `go vet ./internal/cmd/...`, `go test ./...`, and `gofmt` all pass; existing
  `TestConfigInit_Template_*` tests remain green (they compare to the same const → auto-update).

## User Persona (if applicable)

**Target User**: A developer bootstrapping Stagecoach who runs `config init --template` to learn the available
`[generation]` knobs without changing defaults (Mode-A documentation).

**Use Case**: "I want to see every `[generation]` option I could set, commented out, so I can copy/uncomment the
one I need."

**Pain Points Addressed**: Today the generated template hides 5 real options (`exclude`/`format`/`locale`/
`template`/`push`) that the rest of the docs describe — the user has no single inert reference listing all of them.

## Why

- **PRD refs**: §9.18 FR-X1 (`exclude`), §9.19 FR-F1 (`format`), §9.19 FR-F6 (`locale`), §9.19 FR-F8 (`template`),
  §9.22 FR-P1 (`push`); also §16.2 (config schema) and the template's own "documents every available option" claim.
- **Consistency**: docs/configuration.md already documents all five (L105–108, L131–134, L195). The template is
  the *other* user-facing surface for `[generation]`; this fix makes the two agree.
- **Lowest-risk fix**: a string-literal edit + one test. No logic, no config resolution, no precedence changes —
  the values are commented, so load behavior is byte-for-byte unchanged.

## What

No behavioral change — purely an addition of 5 commented documentation lines to the inert reference template. The
generated file remains inert (every key commented). When a user uncomments one, the existing config loader
(`internal/config/config.go`) already handles all five fields (they are shipped, working features since v2.1).

### Success Criteria

- [ ] `exampleConfigTemplate`'s `[generation]` block contains all 13 keys, the 5 new ones placed after
  `binary_extensions` and before the `NOTE:` line.
- [ ] Each new line is commented (`# `), preserves the column alignment of existing keys, and carries its PRD §-ref.
- [ ] The `push` line's `` `git push` `` code span renders correctly (uses the `+ "` + ` backtick-split idiom).
- [ ] New test `TestConfigInit_Template_GenerationKeys` passes and asserts all 5 keys render commented.
- [ ] `go test ./...` green; existing `TestConfigInit_Template_*` tests unchanged and still green.

## All Needed Context

### Context Completeness Check

_This PRP names the exact edit site (file + lines), the exact 5 lines to insert (copy-ready Go source including
the backtick split), the verified column-alignment rule, the confirmed default values (with struct/doc citations),
the test helper idiom (copied from an existing test), and the precise validation commands. An implementer with no
codebase knowledge can complete it in one pass._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/docs/issue_analysis.md
  why: §Issue 3 root-cause + fix design — provides the exact 5 lines to insert (authoritative wording + §-refs).
  section: "## Issue 3 (Minor): `config init --template` reference config omits v2.1 `[generation]` keys"
  critical: "Gives the exact commented lines AND the config-struct field→default mapping. Insert AFTER
             binary_extensions, BEFORE the NOTE: line."

- docfile: plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/P1M1T3S1/research/findings.md
  why: Verified line numbers, the column-alignment math, the backtick-split idiom citations, the defaults table,
       the self-updating-test proof, and the `--config` honoring evidence.
  critical: "The push line REQUIRES the `+ \"` + ` split (it contains a literal backtick). The other 4 lines do NOT.
             Existing template tests compare to the exampleConfigTemplate CONST — they auto-update, do NOT edit them."

- file: internal/cmd/config.go
  why: exampleConfigTemplate const (L497); [generation] block (L563–L574); insertion point (after L573, before L574).
       Also the two backtick-split PATTERNS to mirror: auto_stage_all (L560) and strip_code_fence (L571).
  pattern: |
    # existing lines (column-0, # prefix; `=` at col 24; value field width 8; `#` description col):
    # binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
    # NOTE: [generation] output/strip_code_fence override any per-provider [provider.<name>] values.
    # backtick-split idiom (auto_stage_all, L560) — the push line MUST follow this:
    # auto_stage_all = true     # run ` + "`git add -A`" + ` when nothing is staged
  gotcha: "Template is a Go RAW STRING LITERAL (backtick-delimited). Literal backticks are impossible inside it —
           split with ` + \"` + `. Lines live at COLUMN 0 (no Go indentation). gofmt does NOT reformat raw literals."

- file: internal/config/config.go
  why: Confirms the 5 fields, their toml tags, and defaults (Exclude L103, Format L85, Locale L89, Template L93,
       Push L120). Use ONLY for verifying default values — DO NOT edit this file.
  pattern: "Format string `toml:\"format\"` // ... \"auto\" (default)  |  Push bool `toml:\"push\"` // default false"
  gotcha: "This file is READ-ONLY for this task. The fix is purely a doc string in config.go's template."

- file: docs/configuration.md
  why: The wording source of truth. Commented [generation] example at L102–108; defaults table L131–134;
       git-config wording for push L195. Mirror its phrasing for consistency.
  pattern: |
    # exclude  = []      # UNIONS across layers ...
    # format   = "auto"  # auto|conventional|gitmoji|plain; unknown = hard error (exit 1)
    # locale   = ""      # free-form language name or BCP-47 tag; never validated
    # template = ""      # wrap every message; must contain literal $msg, e.g. "$msg (#205)"
    # push     = false   # (see git-config table L195 for full wording)
  gotcha: "The template lines should CARRY the PRD §-refs (issue_analysis wording) — docs/configuration.md omits
           some §-refs but the template convention (see max_commits line) includes them."

- file: internal/cmd/config_test.go
  why: (1) Existing template tests prove SELF-UPDATING (compare to the const): TestConfigInit_Template_WritesInert
       (L409), TestConfigInit_TemplateIsInert (L448), TestConfigInit_Force_OverwritesTemplate (L564) — DO NOT EDIT.
       (2) The test-helper idiom to copy for the NEW test (saveRootState/restoreRootState/resetFlags/setupNoRepo/
       SetOut/SetErr/SetArgs/Execute/ReadFile). (3) The --config precedent: TestConfigInit_ConfigFlag_WritesOverride (L198).
  pattern: |
    func TestConfigInit_Template_GenerationKeys(t *testing.T) {
        _, origOut, origErr, origRunE := saveRootState(t)
        defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()
        setupNoRepo(t)
        tmp := filepath.Join(t.TempDir(), "ref.toml")
        rootCmd.SetOut(io.Discard); rootCmd.SetErr(io.Discard)
        rootCmd.SetArgs([]string{"config", "init", "--template", "--config", tmp})
        if err := Execute(context.Background()); err != nil { t.Fatalf("Execute err=%v", err) }
        data, err := os.ReadFile(tmp); if err != nil { t.Fatalf("read: %v", err) }
        content := string(data)
        for _, key := range []string{"exclude", "format", "locale", "template", "push"} {
            needle := "# " + key
            if !strings.Contains(content, needle) { t.Errorf("template missing commented key %q", key) }
        }
        if !strings.Contains(content, "`git push`") { t.Error("push line backtick span did not render as `git push`") }
    }
  gotcha: "setupNoRepo chdir's into a temp dir AND isolates HOME/XDG — required so config discovery doesn't leak.
           The `# <key>` substring check proves BOTH presence AND that the line is commented. The `git push` check
           validates the backtick-split renders correctly (the most failure-prone part)."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/config.go        # exampleConfigTemplate const (L497); [generation] block L563–574; runConfigInit L432
internal/cmd/config_test.go   # TestConfigInit_Template_WritesInert (L409), _TemplateIsInert (L448), _Force_OverwritesTemplate (L564)
internal/config/config.go     # Config struct — Exclude/Format/Locale/Template/Push fields (READ-ONLY reference)
docs/configuration.md         # wording source of truth (L102–108, L131–134, L195)
```

### Desired Codebase tree with files to be added/changed

```bash
internal/cmd/config.go        # ~ exampleConfigTemplate [generation] block: +5 commented lines (after binary_extensions)
internal/cmd/config_test.go   # + TestConfigInit_Template_GenerationKeys
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: exampleConfigTemplate is a Go RAW STRING LITERAL (backtick-delimited, config.go L497). You CANNOT
// embed a literal backtick inside it. The `push` line's description contains the code span `git push` — it MUST
// be split with the  + "`" +  concatenation idiom, EXACTLY like the existing auto_stage_all line (L560):
//   # auto_stage_all = true     # run ` + "`git add -A`" + ` when nothing is staged
// and the strip_code_fence line (L571):
//   # strip_code_fence      = true    # strip ` + "`" + ` fences from agent output (all providers)

// CRITICAL: the other 4 new lines (exclude/format/locale/template) contain NO backticks → plain single segments,
// NO split. The `template` line uses $msg inside DOUBLE quotes — `$` is NOT special in a raw literal, so NO escape.

// CRITICAL: all template content lines sit at COLUMN 0 (no leading tabs/spaces) — verify against existing lines.
// gofmt does NOT touch raw string literals, so you must align manually.

// GOTCHA: column alignment. The `=` sign lands at 0-indexed column 24: pad each key LEFT-justified to width 21,
// then ` = ` (space, equals, space). The trailing `#` description marker follows a value field of width 8
// (left-justified). Pad widths for new keys: exclude 7→14, format 6→15, locale 6→15, template 8→13, push 4→17.

// GOTCHA: existing tests (TestConfigInit_Template_WritesInert L409, _TemplateIsInert L448,
// _Force_OverwritesTemplate L564) compare the written file to the exampleConfigTemplate CONST (exact match).
// They auto-update when you edit the const — DO NOT modify them, and do NOT introduce a hardcoded snapshot.

// GOTCHA: the insertion point is AFTER the `# binary_extensions = [] ...` line and BEFORE the
// `# NOTE: [generation] output/strip_code_fence ...` line. Do not reorder existing keys.
```

## Implementation Blueprint

### Data models and structure

None — this is a documentation string edit. No structs, no config schema, no behavior. The 5 keys already exist
in `internal/config/config.go` (Config struct: `Exclude`, `Format`, `Locale`, `Template`, `Push`) with the
defaults shown below; the template merely documents them as commented examples.

| key      | Go field           | toml tag  | example value in template | effective default |
|----------|--------------------|-----------|---------------------------|-------------------|
| exclude  | `Exclude []string` | `exclude` | `[]`                      | nil (none)        |
| format   | `Format string`    | `format`  | `"auto"`                  | `"auto"`          |
| locale   | `Locale string`    | `locale`  | `""`                      | `""`              |
| template | `Template string`  | `template`| `""`                      | `""`              |
| push     | `Push bool`        | `push`    | `false`                   | `false`           |

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/cmd/config.go — append 5 commented lines to exampleConfigTemplate's [generation] block
  - LOCATE: the [generation] block in the exampleConfigTemplate const (L563–L574). Find the line:
      # binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
    and the immediately-following line:
      # NOTE: [generation] output/strip_code_fence override any per-provider [provider.<name>] values.
  - INSERT, between those two lines (at COLUMN 0, matching existing lines), EXACTLY this Go source (note the push
    line uses the backtick-split idiom ` + "`git push`" + `):
      # exclude               = []      # gitignore-style globs; UNION across global+repo+flag (§9.18 FR-X1)
      # format                = "auto"  # auto|conventional|gitmoji|plain; unknown = hard error (exit 1) (§9.19 FR-F1)
      # locale                = ""      # free-form language name or BCP-47 tag; never validated (§9.19 FR-F6)
      # template              = ""      # wrap every message; must contain literal $msg, e.g. "$msg (#205)" (§9.19 FR-F8)
      # push                  = false   # run ` + "`git push`" + ` after a fully-successful run; on failure commits stand (§9.22 FR-P1)
  - VERIFY alignment: the `=` of each new line aligns with the `=` of `max_diff_bytes`/`binary_extensions`
    (column 24); the trailing `#` aligns with the existing description column.
  - VERIFY the push line compiles: it is `...run ` (raw seg) + "`git push`" (dbl-quoted backtick string) +
    ` after...` (raw seg), mirroring the auto_stage_all line (L560).
  - DO NOT touch any other line, any other const, or runConfigInit.

Task 2: CREATE/ADD test TestConfigInit_Template_GenerationKeys in internal/cmd/config_test.go
  - ADD the function shown in the config_test.go reference above (file: internal/cmd/config_test.go pattern block).
  - PLACE: near the other --template tests (after TestConfigInit_TemplateIsInert, ~L525, or adjacent to the
    TestConfigInit_TemplateFlag_CollisionSafe block).
  - FOLLOW pattern: saveRootState/restoreRootState/resetFlags(configInitCmd.Flags())/setupNoRepo/SetOut/SetErr/
    SetArgs/Execute/os.ReadFile — copy the scaffolding verbatim from TestConfigInit_TemplateIsInert (L448).
  - USE --config tmpPath (per work-item contract): tmp := filepath.Join(t.TempDir(), "ref.toml").
  - ASSERT: for each of exclude/format/locale/template/push, strings.Contains(content, "# "+key).
  - ASSERT: strings.Contains(content, "`git push`") — proves the backtick-split renders the code span.
  - NAMING: TestConfigInit_Template_GenerationKeys (matches the TestConfigInit_Template_* convention).
  - DO NOT modify TestConfigInit_Template_WritesInert / _TemplateIsInert / _Force_OverwritesTemplate.

Task 3: VERIFY — format, build, vet, test
  - gofmt -w internal/cmd/config.go internal/cmd/config_test.go   (confirm no diff after)
  - go build ./...
  - go vet ./internal/cmd/...
  - go test ./internal/cmd/... -run 'ConfigInit_Template' -v
  - go test ./...
  - golangci-lint run ./internal/cmd/...   (if configured)
```

### Implementation Patterns & Key Details

```go
// === Task 1: the exact edit in internal/cmd/config.go (exampleConfigTemplate [generation] block) ===
//
// BEFORE (L565–L574, unchanged context):
//   # [generation]
//   # max_diff_bytes        = 300000  # byte cap on the non-markdown diff section
//   # max_md_lines          = 100     # per-file line cap for markdown diffs
//   # max_duplicate_retries = 3       # re-generation attempts when the subject duplicates a recent commit
//   # subject_target_chars  = 50      # target subject-line length for truncation
//   # output                = "raw"   # agent output mode: "raw" | "json" — applies to parsing across ALL providers
//   # strip_code_fence      = true    # strip ` + "`" + ` fences from agent output (all providers)
//   # max_commits           = 12      # safety cap on auto-decompose (PRD §9.14 FR-M4); default 12
//   # binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
//   # NOTE: [generation] output/strip_code_fence override any per-provider [provider.<name>] values.
//
// AFTER — insert the 5 lines BETWEEN binary_extensions and NOTE: (column 0; push uses the backtick split):
//   # binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
//   # exclude               = []      # gitignore-style globs; UNION across global+repo+flag (§9.18 FR-X1)
//   # format                = "auto"  # auto|conventional|gitmoji|plain; unknown = hard error (exit 1) (§9.19 FR-F1)
//   # locale                = ""      # free-form language name or BCP-47 tag; never validated (§9.19 FR-F6)
//   # template              = ""      # wrap every message; must contain literal $msg, e.g. "$msg (#205)" (§9.19 FR-F8)
//   # push                  = false   # run ` + "`git push`" + ` after a fully-successful run; on failure commits stand (§9.22 FR-P1)
//   # NOTE: [generation] output/strip_code_fence override any per-provider [provider.<name>] values.

// === Why the push line is split (and the others are not) ===
// exampleConfigTemplate is ` ... ` (raw literal). A backtick cannot appear raw inside it. `git push` is wrapped
// in backticks (markdown code span) in the description, so that ONE line uses:
//     ...run ` + "`git push`" + ` after...
// which concatenates: raw "...run " + double-quoted "`git push`" + raw " after...". The other 4 lines have no
// backticks → no split. `$msg` in the template line is inside double quotes and `$` is inert in a raw literal.

// === Task 2: the new test (copy scaffolding from TestConfigInit_TemplateIsInert at config_test.go L448) ===
func TestConfigInit_Template_GenerationKeys(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	setupNoRepo(t)
	tmp := filepath.Join(t.TempDir(), "ref.toml") // parent (TempDir) exists — writeBootstrapFile MkdirAlls anyway
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--template", "--config", tmp})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("cannot read template at %s: %v", tmp, err)
	}
	content := string(data)

	// All 5 v2.1 keys must render as commented documentation lines in the [generation] section.
	for _, key := range []string{"exclude", "format", "locale", "template", "push"} {
		needle := "# " + key
		if !strings.Contains(content, needle) {
			t.Errorf("template [generation] missing commented key %q (needle %q not found)", key, needle)
		}
	}

	// The push line's `git push` code span must render correctly (validates the backtick-split concatenation).
	if !strings.Contains(content, "`git push`") {
		t.Errorf("template push line did not render the `git push` code span (backtick split mis-wired)")
	}
}
```

### Integration Points

```yaml
UPSTREAM/DOWNSTREAM: none — fully self-contained. The 5 keys are already implemented (Exclude/Format/Locale/
  Template/Push in internal/config/config.go) and already documented in docs/configuration.md. This task only
  fixes the inert reference template. No config loader, precedence, CLI flag, help-text, or behavior changes.
CALLSITES AFFECTED:
  - internal/cmd/config.go (runConfigInit → writeBootstrapFile): UNCHANGED. It writes exampleConfigTemplate
    verbatim; the 5 new lines ride along automatically.
  - No other code reads exampleConfigTemplate by content (the exact-match tests reference the same const → auto-update).
DOCS: Mode-A — the template IS the documentation. docs/configuration.md is ALREADY correct; do NOT edit it.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/cmd/config.go internal/cmd/config_test.go
git diff --stat internal/cmd/config.go internal/cmd/config_test.go   # confirm ONLY these 2 files changed
go build ./...
go vet ./internal/cmd/...
golangci-lint run ./internal/cmd/...   # if configured
# Expected: zero errors. gofmt must leave NO further diff (run `gofmt -d` to confirm). The push line's
# ` + "`git push`" + ` is valid Go (mirrors the existing auto_stage_all line); build proves the backtick split.
```

### Level 2: Unit Tests (the contract)

```bash
go test ./internal/cmd/... -run 'ConfigInit_Template' -v
# REQUIRED outcomes:
#  TestConfigInit_Template_WritesInert:        PASS (auto-updated — compares to the same const).
#  TestConfigInit_TemplateIsInert:             PASS (no uncommented header introduced; 5 lines are commented).
#  TestConfigInit_TemplateFlag_CollisionSafe:  PASS (unrelated).
#  TestConfigInit_Force_OverwritesTemplate:    PASS (auto-updated — compares to the same const).
#  TestConfigInit_Template_GenerationKeys:     PASS (NEW — all 5 keys render commented; `git push` span present).
go test ./...   # full suite green — no behavioral change anywhere
# Expected: all green. If TestConfigInit_TemplateIsInert fails with "uncommented TOML header", a new line lost
# its `# ` prefix — re-check Task 1 (every new line MUST start with `# `).
```

### Level 3: Integration (manual repro from the issue)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach
/tmp/stagecoach config init --template --config /tmp/ref.toml
echo "exit=$?"
# The [generation] section must now show all 13 keys. Grep the 5 new ones:
grep -nE '^# (exclude|format|locale|template|push) ' /tmp/ref.toml
# Expected: 5 matching lines, each commented, e.g.:
#   # exclude               = []      # gitignore-style globs; UNION across global+repo+flag (§9.18 FR-X1)
#   # format                = "auto"  # auto|conventional|gitmoji|plain; unknown = hard error (exit 1) (§9.19 FR-F1)
#   # locale                = ""      # free-form language name or BCP-47 tag; never validated (§9.19 FR-F6)
#   # template              = ""      # wrap every message; must contain literal $msg, e.g. "$msg (#205)" (§9.19 FR-F8)
#   # push                  = false   # run `git push` after a fully-successful run; on failure commits stand (§9.22 FR-P1)
# Confirm the push line rendered the backtick span (not a compile artifact):
grep -F '`git push`' /tmp/ref.toml   # expected: 1 match
# Confirm the file is still INERT (no uncommented key in [generation]):
awk '/^\[generation\]/{f=1;next} /^\[/{f=0} f && /^[a-z]/{print "UNCOMMENTED: "$0}' /tmp/ref.toml   # expected: (no output)
```

### Level 4: Cross-cutting / Regression

```bash
# Confirm no other code reads the template by content (only the exact-match tests, which auto-update):
grep -rn "exampleConfigTemplate" internal/   # expected: definition (config.go) + 2 exact-match test refs only
# Confirm docs/configuration.md and the template now agree on the 5 keys' presence:
for k in exclude format locale template push; do echo -n "$k: "; grep -c "^# $k " /tmp/ref.toml; done   # each = 1
# Expected: only the 2 intended files changed; docs already correct (Mode-A).
```

## Final Validation Checklist

### Technical
- [ ] `gofmt -d internal/cmd/config.go internal/cmd/config_test.go` shows no diff after formatting.
- [ ] `go build ./...`, `go vet ./internal/cmd/...`, `golangci-lint run ./internal/cmd/...` clean.
- [ ] `go test ./...` green (incl. the new `TestConfigInit_Template_GenerationKeys` and the auto-updated existing tests).

### Feature
- [ ] `config init --template --config <f>` → `[generation]` lists all 13 keys (8 existing + 5 new).
- [ ] Each new line is commented (`# `) and preserves column alignment (`,` at col 24, `#` desc col aligned).
- [ ] The `push` line renders `` `git push` `` correctly (backtick-split idiom wired right).
- [ ] The generated file remains INERT (no uncommented key added — load behavior unchanged).

### Code Quality
- [ ] Only `internal/cmd/config.go` and `internal/cmd/config_test.go` changed (no other files touched).
- [ ] Existing template tests were NOT edited (they auto-update against the const).
- [ ] Wording mirrors docs/configuration.md and carries the PRD §-refs per the issue_analysis contract.
- [ ] No new patterns introduced — the backtick split reuses the existing `auto_stage_all`/`strip_code_fence` idiom.

### Scope Boundaries (do NOT cross)
- [ ] Do NOT edit `internal/config/config.go` (the Config struct — already correct).
- [ ] Do NOT edit `docs/configuration.md` (already correct — Mode-A, the template IS the doc fix).
- [ ] Do NOT change `runConfigInit`, `writeBootstrapFile`, or any config-loading/precedence logic.
- [ ] Do NOT reorder existing `[generation]` keys or alter the `NOTE:` line.
- [ ] Do NOT add the 5 keys to the *populated* bootstrap config (`GenerateBootstrapConfig`) — only the inert template.

---

## Anti-Patterns to Avoid
- ❌ Don't write the push line as a single raw segment with a literal backtick — it won't compile (raw literals can't contain backticks). Use the ` + "`git push`" + ` split.
- ❌ Don't escape `$msg` in the template line — `$` is inert in a raw string literal; it must render verbatim as `$msg`.
- ❌ Don't indent the new lines with Go tabs/spaces — template content is at COLUMN 0 (gofmt won't fix raw literals).
- ❌ Don't misalign the columns — the `=` must land at col 24 and the description `#` at the existing desc column.
- ❌ Don't edit the existing `TestConfigInit_Template_*` tests to "add the new lines" — they compare to the CONST and auto-update; editing them is both unnecessary and risks masking a real regression.
- ❌ Don't edit `docs/configuration.md` or `internal/config/config.go` — out of scope (Mode-A; struct already correct).
- ❌ Don't uncomment any line or change a value — the template must stay 100% inert.

---

## Confidence Score

**9.5/10** for one-pass success. This is a surgical, two-file documentation fix: 5 commented lines appended to one
Go raw-string-literal const + one positive test. Every detail is pinned — the exact edit site (config.go L573→L574
gap), the exact Go source for all 5 lines (including the load-bearing ` + "`git push`" + ` backtick split that
mirrors the existing `auto_stage_all` line), the verified column-alignment rule, the confirmed default values
(cited from the Config struct + docs/configuration.md), the self-updating-test proof (existing tests compare to the
same const → no golden snapshot to break), and a copy-ready test that reuses the existing helper idiom. The only
residual risk is a column-alignment typo in the raw literal (gofmt won't catch it) — mitigated by the explicit
column math and the `grep`-based Level 3 check. No external deps, no logic change, no behavior change.
