name: "P1.M2.T1.S1 — format + locale config/flag/env/git-config plumbing with hard-error validation"
description: |
  Add two new SCALAR string fields to the resolved Config — `Format` (default "auto") and `Locale`
  (default "") — and resolve each through the FULL 5-layer precedence (`[generation].format`/`[generation].locale`
  TOML → `stagecoach.format`/`stagecoach.locale` git-config → `STAGECOACH_FORMAT`/`STAGECOACH_LOCALE` env → `--format`/
  `--locale` flags). `Format` is an ENUM validated against {auto, conventional, gitmoji, plain} at the tail of
  `Load()` — an unknown mode is a HARD configuration error (exit 1) naming the offending value and the valid set
  (PRD §9.19 FR-F1). `Locale` is free-form, passed verbatim, NEVER validated, no i18n tables (FR-F6). This subtask
  produces `cfg.Format` / `cfg.Locale` and NOTHING ELSE: the gitmoji table (S2), the prompt scaffolds + locale
  line + application at every message site (S3), and bootstrap (`config init`) are all OUT OF SCOPE. It is the
  exact same seams as P1.M1.T1.S1 (`cfg.Exclude`), but SCALAR fields with the STANDARD replace overlay (copy
  Provider/Model/Reasoning, NOT Exclude's union) plus a single post-resolution enum check.

---

## Goal

**Feature Goal**: A user can select a commit-message format style and/or a target language through ANY of the
four standard config surfaces — config file (`[generation]`), per-repo git-config (`stagecoach.*`), environment
(`STAGECOACH_*`), or a CLI flag — and stagecoach resolves a single `cfg.Format` (one of `auto | conventional |
gitmoji | plain`, default `auto`) and a single `cfg.Locale` (free-form string, default `""`) following the
existing precedence (FR34). A typo'd format (e.g. `--format emoji`) fails the whole load with a clear,
exit-1 error before any generation; a locale value is accepted unconditionally.

**Deliverable**:
1. `Config.Format string` (`toml:"format"`, default `"auto"`) + `Config.Locale string` (`toml:"locale"`,
   default `""`) in `internal/config/config.go`, with `Defaults()` initializers.
2. `fileGeneration.Format` / `fileGeneration.Locale` decode fields + `materialize()` non-zero copy +
   `overlay()` standard REPLACE merge (non-zero wins) in `internal/config/file.go`.
3. `stagecoach.format` / `stagecoach.locale` reads (plain `gitConfigGet`) in `loadGitConfig()`
   (`internal/config/git.go`).
4. `STAGECOACH_FORMAT` / `STAGECOACH_LOCALE` presence-semantic copies in `loadEnv()`
   (`internal/config/load.go`).
5. `--format` / `--locale` `StringVar` persistent flags in `internal/cmd/root.go` `init()` + `loadFlags()`
   `fs.Changed`/`GetString` copies.
6. `validateFormat(string) error` pure helper + a `validFormats` slice + the SINGLE post-resolution call at
   the tail of `Load()` (after the v3 migration block).
7. Table-driven precedence + validation unit tests (config package) + `newFlagSet` flag registration.
8. Mode-A docs: two flag rows + two map rows in `docs/cli.md`; `[generation]` keys + built-in-defaults rows +
   env-var rows + git-config rows in `docs/configuration.md`.

**Success Definition**:
- `cfg.Format` resolves to the highest-precedence value across the 5 layers (flag > env > git > repo-file >
  global-file > `Defaults()` `"auto"`); `cfg.Locale` likewise (default `""`).
- `cfg.Format == "auto"` when nothing sets it (no boot-loop from validation).
- An unknown `Format` value from ANY layer (resolved) makes `Load()` return a non-nil error whose message
  names the offending value and the valid set → the CLI exits 1 with `config: format: …`.
- `cfg.Locale` accepts any string (`"日本語"`, `"en-US"`, `"nonsense"`) with no error and no normalization.
- No env var, no git-config key, no validation exists for locale; no `STAGECOACH_*`/`stagecoach.*` for the
  gitmoji TABLE or the prompt scaffolds (those are S2/S3 — not touched).
- `go build ./...`, `go test ./...`, `go vet`, `golangci-lint` all pass.

## User Persona (if applicable)

**Target User**: "the plan-holder" (primary persona) and "the API-key refusenik" — a CLI developer who wants
their commit messages in a fixed style (Conventional Commits / gitmoji / plain) and/or a non-English language,
persisted per-repo or passed ad-hoc.

**Use Case**: Pin `format = "conventional"` in `.stagecoach.toml` so every commit follows `type(scope): desc`;
or `--locale fr` for a one-off French message; or `STAGECOACH_FORMAT=gitmoji` for a session.

**User Journey**: set the value via whichever surface is convenient → `stagecoach` loads it → (S3 applies it to
the prompt). A typo is caught at load (exit 1, clear message), not mid-generation.

**Pain Points Addressed**: (1) incumbent tools ship 20 locale file-trees; stagecoach needs ONE free-form string
(FR-F6). (2) No way today to force a style independent of repo history (FR-F1). (3) Bad config should fail
loudly, not silently degrade.

## Why

- **FR-F1 (PRD §9.19)**: `--format` / `STAGECOACH_FORMAT` / `stagecoach.format` / `[generation].format`, mode ∈
  `{auto, conventional, gitmoji, plain}`, default `auto`. "An unknown mode is a hard configuration error."
  This subtask delivers the RESOLUTION + that HARD ERROR; the mode→prompt mapping is S3 (§17.8).
- **FR-F6 (PRD §9.19)**: `--locale` / `STAGECOACH_LOCALE` / `stagecoach.locale` / `[generation].locale`,
  default empty, "passed verbatim (the model understands both; stagecoach does not validate it and ships no
  i18n tables)." This subtask delivers the RESOLUTION and explicitly does NOT validate.
- **FR35 (§9.8)** lists `STAGECOACH_FORMAT` / `STAGECOACH_LOCALE` among the env vars; **FR36** lists
  `stagecoach.format` / `stagecoach.locale` among the git-config keys. Both are currently ABSENT (system_context
  §2 grep-verified). This subtask materializes all four entries on both lists.
- **§15.2 Global flags table** already documents the `--format` / `--locale` rows (default `auto` / `—`); the
  FLAG HELP and the code-side registration are what's missing. This subtask makes the binary match the spec.
- `cfg.Format` / `cfg.Locale` are the input contract for **S3** (P1.M2.T1.S3), which applies them at every
  message-production site. Getting the resolved values + the hard-error guard right here is the prerequisite
  for the entire §9.19 shaping feature.

## What

```toml
# ~/.config/stagecoach/config.toml  (or ./.stagecoach.toml)
[generation]
format = "conventional"   # auto | conventional | gitmoji | plain (unknown = hard error)
locale = "ja-JP"           # free-form language name or BCP-47 tag; never validated
```
```bash
git config stagecoach.format gitmoji
git config stagecoach.locale en-US
```
```bash
STAGECOACH_FORMAT=plain STAGECOACH_LOCALE=de stagecoach
stagecoach --format conventional --locale fr
```

All four surfaces compose by standard precedence (flag > env > git-config > repo-file > global-file > default).
`format` and `locale` resolve INDEPENDENTLY (you can set one without the other). An unknown `--format emoji`
aborts before generation:

```text
$ stagecoach --format emoji
config: format: invalid format "emoji" (valid: auto, conventional, gitmoji, plain)
# exit 1
```

### Success Criteria

- [ ] `Config.Format string` (`toml:"format"`) + `Config.Locale string` (`toml:"locale"`) exist; `Defaults()`
      sets `Format: "auto"`, `Locale: ""`.
- [ ] `fileGeneration.Format` / `fileGeneration.Locale` decode `[generation].format` / `[generation].locale`.
- [ ] `materialize()` copies each (non-empty guard); `overlay()` REPLACE-merges each (`if src.Format != ""`).
- [ ] `loadGitConfig()` reads `stagecoach.format` / `stagecoach.locale` via `gitConfigGet` (raw copy, no validation).
- [ ] `loadEnv()` copies `STAGECOACH_FORMAT` / `STAGECOACH_LOCALE` (presence-semantic).
- [ ] `loadFlags()` copies `--format` / `--locale` (gated on `fs.Changed`).
- [ ] `--format` / `--locale` registered as single `StringVar` persistent flags (zero default; no shorthand).
- [ ] `validateFormat(string) error` + `validFormats` slice; called ONCE at the tail of `Load()`.
- [ ] Bad resolved format → `Load()` error naming value + valid set → exit 1.
- [ ] Locale never validated; any string accepted verbatim.
- [ ] Table-driven tests cover: 5-layer precedence (per field), validation (4 valid + invalid), defaults,
      materialize/overlay replace, git-config read, env copy, flag copy, locale free-form.
- [ ] `docs/cli.md` + `docs/configuration.md` updated (FR-F1 hard-error + FR-F6 no-validation stated).

## All Needed Context

### Context Completeness Check

_This PRP names every file to edit, the exact struct field / function / line-region to change, the sibling
scalar to copy (`Provider`/`Model`/`Reasoning` — the precedent for file→git→env→flag replace), the one sibling
NOT to copy (`Exclude` — union + no env/git, wrong for scalars), the precise validation site (tail of `Load`),
the error shape, the flag-registration idiom, the test helpers to extend (`newFlagSet`, `setGitConfig`,
`loadEnvSetup`), and the exact doc table rows. An implementer with no prior codebase knowledge can complete it._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/architecture/system_context.md
  why: §3 "internal/config" names the seam (Config [generation] fields; the 5-layer cascade in Load();
       overlay() non-zero merge) and §2 grep-confirms --format/--locale/STAGECOACH_FORMAT/STAGECOACH_LOCALE/
       stagecoach.format/stagecoach.locale are all ABSENT today. §6 watch-out #4 (--template name collision).
  section: "## 3. Package inventory & the v2.1 seams" + "## 2. Command surface today"
  critical: "Scalar keys follow the standard cascade (replace/overlay); ONLY `exclude` unions. format/locale
             are SCALARS → copy Provider/Model, NOT Exclude. Needed [generation] keys: format, locale."

- docfile: plan/005_c38aa48290f0/P1M2T1S1/research/design-decisions.md
  why: The ten locked decisions for THIS task (scalar-not-union, the "auto" default gotcha, single
       post-resolution validation site + why-not-per-layer, validFormats placement, single-word git keys,
       flag idiom, scope fence, doc placement, why no external research).
  critical: "Section 2 (the "auto" default is NON-EMPTY — Defaults() MUST set it or validation boot-loops)
            and Section 3 (validation is ONE call at the Load() tail on the RESOLVED value; per-layer would
            false-positive on overridden low-layer typos)."

- file: internal/config/config.go
  why: Config struct (the [generation] block, ~L80-90) + Defaults() (~L150-180). Add Format/Locale here.
  pattern: "BinaryExtensions / Exclude are the [generation] list siblings; Output *string is the scalar
           sibling. For a STRING scalar, mirror Reasoning (string field, `toml:` tag, explicit Defaults entry).
           Place Format/Locale in the [generation] scalar group near Output/MaxCommits, NOT in the list group."
  gotcha: "Config is flat/plain and NEVER decoded directly from the §16.2 file; the toml tag documents the §16.2
          leaf name only. Defaults() MUST set Format:\"auto\" (non-empty) — an empty default would make
          validateFormat reject every unconfigured repo. Locale:\"\" is the correct empty default."

- file: internal/config/file.go
  why: fileGeneration decode struct (~L49-58) + materialize() (~L186-245) + overlay() (~L258-348).
  pattern: |
    - fileGeneration: add `Format string \`toml:"format"\`` + `Locale string \`toml:"locale"\`` (after Exclude).
    - materialize(): `if g.Format != "" { c.Format = g.Format }` + `if g.Locale != "" { c.Locale = g.Locale }`
      (next to the BinaryExtensions/Exclude copy).
    - overlay(): STANDARD replace, NOT union:
        if src.Format != "" { dst.Format = src.Format }
        if src.Locale != "" { dst.Locale = src.Locale }
  gotcha: "DO NOT copy Exclude's `append(...)` union branch — format/locale REPLACE (copy Provider/Model/Reasoning
          scalar pattern: `if src.X != \"\" { dst.X = src.X }`). overlay is called global→repo→gitconfig; the
          highest non-empty layer wins, exactly like Provider."

- file: internal/config/git.go
  why: loadGitConfig() (~L97-180) reads stagecoach.* keys. format/locale are SINGLE-WORD keys.
  pattern: "Copy the stagecoach.output block EXACTLY (single-word string key, plain gitConfigGet, raw copy):
            if v, found, err := gitConfigGet(repoDir, \"stagecoach.format\"); err != nil { return nil, err }
            else if found { c.Format = v }"
  gotcha: "Single-word keys have NO camelCase issue (unlike maxDiffBytes). Do NOT validate here — the final
          resolved value is validated once in Load(). Place the two new blocks near the existing string keys
          (stagecoach.provider/model/output)."

- file: internal/config/load.go
  why: Load() pipeline (the tail, ~L120-140, after the v3 migration if/else, before return) — the validation
       site. loadEnv() (~L120-175) + loadFlags() (~L180-260) — the env/flag copy sites.
  pattern: |
    - loadEnv(): presence-semantic copy (mirror STAGECOACH_PROVIDER):
        if v, ok := os.LookupEnv("STAGECOACH_FORMAT"); ok && v != "" { cfg.Format = v }
        if v, ok := os.LookupEnv("STAGECOACH_LOCALE"); ok && v != "" { cfg.Locale = v }
    - loadFlags(): gated copy (mirror provider):
        if fs.Changed("format") { if v, err := fs.GetString("format"); err == nil { cfg.Format = v } }
        if fs.Changed("locale") { if v, err := fs.GetString("locale"); err == nil { cfg.Locale = v } }
    - Load() tail (AFTER the `if fileLoaded && cfg.ConfigVersion < CurrentConfigVersion {…} else if …` migration
      block, BEFORE `return &cfg, nil`):
        if err := validateFormat(cfg.Format); err != nil {
            return nil, fmt.Errorf("format: %w", err)
        }
        // Locale is deliberately NOT validated (FR-F6): free-form, passed verbatim.
  gotcha: "Validation MUST be post-resolution (after ALL layers + migration), not per-layer — a low-layer typo
          overridden by a higher layer is NOT an error. The `fmt.Errorf(\"format: %w\", err)` wrapper names the
          field (PersistentPreRunE adds the outer 'config:'). Do NOT add validateLocale."

- file: internal/config/load.go  (validateFormat + validFormats — NEW, co-located with configVersionNotice)
  why: The pure validation helper + the valid-set slice, unit-testable in isolation (like configVersionNotice).
  pattern: |
    var validFormats = []string{"auto", "conventional", "gitmoji", "plain"}
    func validateFormat(format string) error {
        for _, m := range validFormats {
            if format == m { return nil }
        }
        return fmt.Errorf("invalid format %q (valid: %s)", format, strings.Join(validFormats, ", "))
    }
  gotcha: "Unexported (validation-only; S3 builds scaffolds from static strings). Quote the offending value with
          %q (it may contain spaces/special chars). strings.Join with \", \" (NOT \" | \") for the valid set."

- file: internal/cmd/root.go
  why: init() (~L100-160) registers persistent flags; package vars declared (~L27-65). Add --format/--locale.
  pattern: |
    - Declare package vars in the config-backed flags block (near flagProvider/flagModel):
        var ( … flagFormat string; flagLocale string )
    - Register in init() (single StringVar, zero default, NO shorthand — PRD §15.2 gives none):
        pf.StringVar(&flagFormat, "format", "",
            "Message format: auto|conventional|gitmoji|plain (env STAGECOACH_FORMAT; git stagecoach.format; "+
                "[generation].format; default auto). Unknown mode is a hard error.")
        pf.StringVar(&flagLocale, "locale", "",
            "Write the message in this language (free-form name or BCP-47 tag; env STAGECOACH_LOCALE; "+
                "git stagecoach.locale; [generation].locale; default empty)")
  gotcha: "Use StringVar (NOT StringArray — not repeatable). flagFormat/flagLocale read ONLY via
          fs.Changed/fs.GetString in loadFlags (never the package var directly); the &address is its use
          (satisfies `unused` linter). Disambiguate --format from `config init --template` in the help text."

- file: internal/config/load_test.go  (+ git_test.go)
  why: newFlagSet() (~L50-75) MUST register --format/--locale for Load() integration tests; table-driven
       precedence patterns (TestLoadEnv_*, overlay tests) + git-config patterns (TestLoadGitConfig_*).
  pattern: |
    - Add to newFlagSet(): fs.String("format", "", ""); fs.String("locale", "", "")
    - Mirror TestLoadGitConfig_ReadsValues (setGitConfig stagecoach.format/locale → assert c.Format/c.Locale).
    - Mirror a Load() precedence test for format across the 5 layers; add validateFormat table test.
  gotcha: "newFlagSet is shared; adding flags is behavior-neutral for tests that don't Set them. A Load() test
          that Sets a flag MUST have it registered or fs.Changed panics. For git-config tests use the existing
          setGitConfig(t, repo, 'stagecoach.format', 'gitmoji') helper + t.Setenv(\"HOME\", t.TempDir())."

- file: docs/cli.md
  why: Global flags table + Flag↔env↔git-config map table — add two rows each.
  pattern: "Mirror the --reasoning / --exclude rows (Flag | Type | Default | Env var | Git config | Description).
            For --format: Default 'auto', Env STAGECOACH_FORMAT, Git 'stagecoach.format', modes + hard-error note.
            For --locale: Default '—' (empty), Env STAGECOACH_LOCALE, Git 'stagecoach.locale', free-form note."
  gotcha: "Place the two rows AFTER --exclude/-x and BEFORE the per-role block (matches PRD §15.2 ordering).
           Keep table column counts identical to existing rows."

- file: docs/configuration.md
  why: Four tables/sections need the new keys: File format [generation] + commented example, Built-in defaults,
       Environment variables, Git-config keys.
  pattern: "Mirror existing rows (max_commits, exclude, stagecoach.reasoning). For [generation].format state the
            hard-error-on-unknown rule; for locale state the free-form/no-validation rule (FR-F6)."
  gotcha: "format/locale ARE git-config keys — add them to the Git-config keys TABLE (do NOT add them to the NOTE
           that lists what's absent). The 'no stagecoach.exclude' line stays correct (exclude has no git key)."
```

### Current Codebase tree (relevant slice)

```bash
internal/
  cmd/
    root.go            # persistent flag registration (init) + PersistentPreRunE → config.Load
  config/
    config.go          # Config struct (flat, resolved) + Defaults()  — Format/Locale live here
    file.go            # fileConfig/fileGeneration decode; materialize(); overlay()  — replace merge
    load.go            # Load() pipeline (validation site = tail); loadEnv(); loadFlags(); validateFormat
    git.go             # loadGitConfig() — stagecoach.format/locale via gitConfigGet
    load_test.go       # newFlagSet() helper + table-driven precedence tests
    git_test.go        # setGitConfig/initRepo helpers + TestLoadGitConfig_* patterns
    config_test.go     # Defaults() assertions
docs/
  cli.md               # ## Global flags table + ## Flag ↔ env ↔ git-config map
  configuration.md     # File format / [generation] + Built-in defaults + Env vars + Git-config keys
```

### Desired Codebase tree with files to be added/changed (no new files)

```bash
internal/config/config.go     # + Config.Format (default "auto"), + Config.Locale (default "")
internal/config/file.go       # + fileGeneration.Format/Locale; + materialize copy; + overlay REPLACE
internal/config/git.go        # + stagecoach.format / stagecoach.locale gitConfigGet reads
internal/config/load.go       # + validateFormat + validFormats; + loadEnv STAGECOACH_FORMAT/LOCALE;
                              #   + loadFlags --format/--locale; + Load() tail validation call
internal/cmd/root.go          # + flagFormat/flagLocale vars; + StringVar registration
internal/config/load_test.go  # + newFlagSet registers --format/--locale; + precedence/validation tests
internal/config/git_test.go   # + TestLoadGitConfig reads stagecoach.format/locale
internal/config/config_test.go# + Defaults() Format/Locale assertions (if Defaults is tested here)
docs/cli.md                   # + 2 Global-flags rows + 2 map rows
docs/configuration.md         # + [generation] keys + defaults rows + env rows + git-config rows
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: the "auto" default is NON-EMPTY. Defaults() MUST set Format: "auto". If left "" (zero),
// every unconfigured repo resolves Format=="" → validateFormat rejects it → boot-loop. Locale:"" is correct
// (empty = no locale instruction) but set it explicitly for clarity. This is the ONE field whose default
// is non-zero among the [generation] scalars — the others (Output=nil, MaxCommits=12 is also non-zero —
// mirror MaxCommits' explicit-default treatment).

// CRITICAL: format/locale are SCALARS — use the STANDARD replace overlay (`if src.X != "" { dst.X = src.X }`),
// the SAME machinery as Provider/Model/Reasoning. Do NOT copy Exclude's `append(...)` union branch.
// overlay is called global→repo→gitconfig; the highest non-empty layer wins. This is the documented rule;
// Exclude's union is the singular exception (FR-X1) and does NOT apply here.

// CRITICAL: validation is ONE call at the Load() TAIL on the RESOLVED value, NOT per-layer. A typo at a low
// layer overridden by a higher layer is NOT an error (the flag/env wins, the bad value is moot). Per-layer
// validation would false-positive. validateFormat is pure + unit-testable (mirror configVersionNotice).

// CRITICAL: locale is NEVER validated (FR-F6). Do NOT add validateLocale. Any string — "日本語", "en-US",
// "garbage!!!" — is accepted verbatim and stored raw. Add a code comment at the validateFormat call stating
// the deliberate omission so a future maintainer does not "helpfully" add it.

// GOTCHA: flag/env default is "" (zero), NOT "auto". fs.Changed/os.LookupEnv presence semantics rely on the
// zero default to mean "not set". When unset, loadFlags/loadEnv skip the field and cfg.Format keeps the
// "auto" from Defaults(). This mirrors Provider/Model (flag default "", Config default "") — only the
// Config-side default differs. Do NOT set the flag default to "auto".

// GOTCHA: single-word git-config keys have NO camelCase issue. `stagecoach.format` / `stagecoach.locale` are
// valid as-is (unlike `stagecoach.maxDiffBytes`). Copy the `stagecoach.output` gitConfigGet block verbatim.

// GOTCHA: the validation error surfaces as `config: format: invalid format "X" (valid: …)`. The outer
// `config:` is added by PersistentPreRunE (root.go); the `format:` wrapper is added in Load() to name the
// field. This is exit 1 (exitcode.Error) — the correct "hard configuration error" surface (FR-F1).

// GOTCHA: --format collides in NAME with `config init --template` (different commands — system_context §6 #4).
// Disambiguate in the flag help text ("Message format:" vs the template command). No code-level collision.

// GOTCHA: do NOT touch bootstrap.go / exampleConfigTemplate / GenerateBootstrapConfig. The task contract is
// cfg.Format/cfg.Locale PLUMBING + docs only. FR-B1 does not list format/locale in the bootstrap. The stale
// "= 2" prose in exampleConfigTemplate is P1.M7's job, not this task's.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/config.go — add to the [generation] block of Config (near Output/MaxCommits, NOT the list group):

// Format selects the commit-message style (PRD §9.19 FR-F1): "auto" (style learning, default),
// "conventional", "gitmoji", or "plain". Resolved through the standard 5-layer precedence (file → git →
// env → flag). Validated against validFormats at the tail of Load() — an unknown mode is a hard error (exit 1).
// Consumed by S3 (prompt scaffolds). Default "auto" (NON-empty — Defaults sets it explicitly).
Format string `toml:"format"`
// Locale is a free-form language name or BCP-47 tag appended to the system prompt (PRD §9.19 FR-F6).
// Resolved through the standard 5-layer precedence; NEVER validated, passed verbatim (no i18n tables).
// Empty = no locale instruction (in practice English or whatever the history models). Consumed by S3.
Locale string `toml:"locale"`

// internal/config/config.go — Defaults(): add (Format's "auto" default is load-bearing — see gotchas):
Format: "auto", // §9.19 FR-F1 default (NON-empty; validateFormat would reject "" — must be set here)
Locale: "",     // §9.19 FR-F6 default (empty = no locale instruction)

// internal/config/file.go — add to fileGeneration (after Exclude):
Format string `toml:"format"` // V2.1 — §9.19 FR-F1 message format (validated at Load)
Locale string `toml:"locale"` // V2.1 — §9.19 FR-F6 message locale (free-form, never validated)
```

```go
// internal/config/load.go — the pure validator + valid set (place near configVersionNotice):

// validFormats is the closed set of --format modes (PRD §9.19 FR-F1). Validation-only; S3 builds the
// prompt scaffolds from static strings, not this slice.
var validFormats = []string{"auto", "conventional", "gitmoji", "plain"}

// validateFormat returns nil iff format is one of validFormats, else an error naming the offending value
// and the valid set (PRD §9.19 FR-F1: "An unknown mode is a hard configuration error"). PURE (no I/O) so it
// is unit-testable; called ONCE at the tail of Load() on the FULLY RESOLVED cfg.Format (not per-layer — a
// low-layer typo overridden by a higher layer is not an error). Locale is deliberately NOT validated (FR-F6).
func validateFormat(format string) error {
	for _, m := range validFormats {
		if format == m {
			return nil
		}
	}
	return fmt.Errorf("invalid format %q (valid: %s)", format, strings.Join(validFormats, ", "))
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/config/config.go
  - IMPLEMENT: Config.Format string `toml:"format"` + Config.Locale string `toml:"locale"` in the [generation]
    scalar group (place near Output/MaxCommits — NOT in the list group with BinaryExtensions/Exclude), each
    with a doc comment citing FR-F1/FR-F6, the 5-layer resolution, validation status, and the S3 consumer.
  - IMPLEMENT: Defaults() — Format: "auto" (NON-empty — load-bearing; see gotchas) and Locale: "" (explicit
    for clarity), placed in the [generation] section of the return literal (near MaxCommits: 12).
  - FOLLOW pattern: Reasoning (string scalar, explicit Defaults entry) for DECLARATION; MaxCommits for a
    non-zero explicit default. Do NOT follow Exclude's list shape.
  - PLACEMENT: [generation] scalar group of Config; [generation] block of Defaults().

Task 2: MODIFY internal/config/file.go
  - IMPLEMENT: fileGeneration.Format string `toml:"format"` + fileGeneration.Locale string `toml:"locale"`
    (after the Exclude field, ~L58).
  - IMPLEMENT: materialize() — `if g.Format != "" { c.Format = g.Format }` and
    `if g.Locale != "" { c.Locale = g.Locale }` (next to the Exclude/BinaryExtensions copy, ~L228-235).
  - IMPLEMENT: overlay() — STANDARD REPLACE (NOT union):
      if src.Format != "" { dst.Format = src.Format }
      if src.Locale != "" { dst.Locale = src.Locale }
    place near the [generation] scalar block (NOT near Exclude's append branch); comment "standard replace
    (copy Provider/Model); NOT union — only Exclude unions (FR-X1)".
  - DEPENDENCIES: Task 1 (Config.Format/Locale must exist).
  - PLACEMENT: fileGeneration struct; materialize() [generation] block; overlay() [generation] scalars block.
  - CRITICAL: do NOT copy Exclude's `append(dst.Exclude, src.Exclude...)` union — that is the singular
    exception and is WRONG for scalars.

Task 3: MODIFY internal/config/git.go
  - IMPLEMENT: in loadGitConfig(), near the existing string keys (stagecoach.provider/model/output):
      if v, found, err := gitConfigGet(repoDir, "stagecoach.format"); err != nil { return nil, err } else if found { c.Format = v }
      if v, found, err := gitConfigGet(repoDir, "stagecoach.locale"); err != nil { return nil, err } else if found { c.Locale = v }
  - FOLLOW pattern: the stagecoach.output block EXACTLY (single-word string key, plain gitConfigGet, raw copy).
  - GOTCHA: single-word keys — no camelCase. Do NOT validate here (final value validated once in Load()).

Task 4: MODIFY internal/config/load.go — loadEnv + loadFlags + validateFormat + validFormats
  - IMPLEMENT: validateFormat + validFormats (see Data models above), placed near configVersionNotice.
  - IMPLEMENT: loadEnv() — presence-semantic copies (mirror STAGECOACH_PROVIDER), near the existing STAGECOACH_*
    string block:
      if v, ok := os.LookupEnv("STAGECOACH_FORMAT"); ok && v != "" { cfg.Format = v }
      if v, ok := os.LookupEnv("STAGECOACH_LOCALE"); ok && v != "" { cfg.Locale = v }
  - IMPLEMENT: loadFlags() — gated copies (mirror provider), near the existing fs.Changed("provider") block:
      if fs.Changed("format") { if v, err := fs.GetString("format"); err == nil { cfg.Format = v } }
      if fs.Changed("locale") { if v, err := fs.GetString("locale"); err == nil { cfg.Locale = v } }
  - IMPLEMENT: Load() — at the TAIL, AFTER the `if fileLoaded && cfg.ConfigVersion < CurrentConfigVersion {…}
    else if msg := configVersionNotice(…); msg != "" {…}` migration block and BEFORE `return &cfg, nil`:
      if err := validateFormat(cfg.Format); err != nil {
          return nil, fmt.Errorf("format: %w", err)
      }
      // Locale is deliberately NOT validated (FR-F6): free-form, passed verbatim, no i18n tables.
  - DEPENDENCIES: Tasks 1, 2, 3.
  - PLACEMENT: validateFormat/validFormats near configVersionNotice; loadEnv in the string block; loadFlags
    near the provider block; the validation call at the Load() tail.
  - CRITICAL: the validation call MUST be post-resolution (after ALL layers + migration). The fmt.Errorf
    "format:" wrapper names the field (PersistentPreRunE adds the outer "config:").

Task 5: MODIFY internal/cmd/root.go
  - IMPLEMENT: package vars flagFormat string + flagLocale string (config-backed flags var block, near
    flagProvider/flagModel).
  - IMPLEMENT: pf.StringVar(&flagFormat, "format", "", "<help>") + pf.StringVar(&flagLocale, "locale", "",
    "<help>") in init() (after --exclude/-x, before the per-role block). Help text: format names the 4 modes
    + "default auto" + "Unknown mode is a hard error" + the env/git/[generation] sources; locale names
    "free-form name or BCP-47 tag" + sources + "default empty".
  - FOLLOW pattern: pf.StringVar(&flagProvider, "provider", "", …) (single String, zero default, no shorthand).
  - GOTCHA: no shorthand (PRD §15.2 gives none). Disambiguate --format from `config init --template` in help.
    flagFormat/flagLocale read only via fs.Changed/fs.GetString in loadFlags (never the package var directly).

Task 6: MODIFY internal/config/load_test.go
  - IMPLEMENT: register flags in newFlagSet(): fs.String("format", "", "") + fs.String("locale", "", "").
  - IMPLEMENT: table-driven Load() precedence test for format across the 5 layers (global-file → repo-file →
    git-config → env → flag); assert flag > env > git > repo > global > "auto" default. Same for locale
    (default "" when nothing set). Mirror TestLoadEnv_* / loadEnvSetup (HOME isolation, chdir, writeConfigFile).
  - IMPLEMENT: validateFormat table test: each of {auto, conventional, gitmoji, plain} → nil; {"", "emoji",
    "Conventional", "AUTO", "gitmojii"} → non-nil error containing the value AND "auto, conventional, gitmoji,
    plain". Use strings.Contains on err.Error().
  - IMPLEMENT: locale free-form test: Load() with locale "日本語" / "en-US" / "weird!!!" → no error, cfg.Locale
    stored verbatim.
  - PLACEMENT: new test funcs in load_test.go; the two newFlagSet lines.
  - GOTCHA: a Load() test that fs.Set("format",…) MUST have newFlagSet registered it (or fs.Changed panics).

Task 7: MODIFY internal/config/git_test.go (+ config_test.go if Defaults is asserted there)
  - IMPLEMENT: extend TestLoadGitConfig_ReadsValues: setGitConfig stagecoach.format=gitmoji +
    stagecoach.locale=de; assert c.Format=="gitmoji", c.Locale=="de" (mirror the stagecoach.provider/model/output
    assertions). Also confirm TestLoadGitConfig_MissingKeysIgnored still passes (absent keys ⇒ unchanged).
  - IMPLEMENT: Defaults() assertions (config_test.go): cfg.Format=="auto", cfg.Locale=="" (if a Defaults test
    exists; else add one mirroring the existing Defaults assertions).
  - FOLLOW pattern: setGitConfig(t, repo, "stagecoach.X", "v") + t.Setenv("HOME", t.TempDir()) + initRepo.
  - GOTCHA: git-config is NOT validated in loadGitConfig — a bad format from git-config is only caught at the
    Load() tail (add a test: setGitConfig stagecoach.format=bad → Load() returns the validateFormat error).

Task 8: MODIFY docs/cli.md + docs/configuration.md  [Mode A — rides WITH this subtask]
  - docs/cli.md Global flags table (after --exclude/-x, before per-role): two rows:
      | `--format <mode>` | string | `auto` | `STAGECOACH_FORMAT` | `stagecoach.format` | Message format:
        `auto` (style learning) \| `conventional` \| `gitmoji` \| `plain`. An unknown mode is a hard error
        (exit 1). Also `[generation].format`. |
      | `--locale <lang>` | string | "" | `STAGECOACH_LOCALE` | `stagecoach.locale` | Write the message in
        this language (free-form name or BCP-47 tag; never validated). Also `[generation].locale`. |
  - docs/cli.md Flag↔env↔git-config map: two rows (--format / --locale).
  - docs/configuration.md File format [generation] + commented example: add `format` + `locale` with the
    hard-error / free-form notes.
  - docs/configuration.md Built-in defaults table: `format` → `auto` (`config.Defaults()`); `locale` → `""`.
  - docs/configuration.md Environment variables table: STAGECOACH_FORMAT (mirrors --format) + STAGECOACH_LOCALE.
  - docs/configuration.md Git-config keys table: stagecoach.format (string) + stagecoach.locale (string). Add
    them to the TABLE (they exist); do NOT touch the "no stagecoach.exclude" NOTE (still correct).
  - FOLLOW pattern: mirror the --reasoning / max_commits / exclude rows (column counts identical).
```

### Implementation Patterns & Key Details

```go
// overlay() — STANDARD replace for scalars (copy Provider/Model/Reasoning, NOT Exclude's union):
func overlay(dst, src *Config) {
	// ... existing scalars + Exclude UNION ...
	// §9.19 FR-F1/FR-F6 — format/locale are SCALARS: standard non-zero REPLACE (the rule), NOT union
	// (only Exclude unions, FR-X1). overlay is called global→repo→gitconfig; highest non-empty layer wins.
	if src.Format != "" {
		dst.Format = src.Format
	}
	if src.Locale != "" {
		dst.Locale = src.Locale
	}
}

// loadEnv() — presence-semantic copy (empty/unset is a no-op), exactly like STAGECOACH_PROVIDER:
if v, ok := os.LookupEnv("STAGECOACH_FORMAT"); ok && v != "" {
	cfg.Format = v
}
if v, ok := os.LookupEnv("STAGECOACH_LOCALE"); ok && v != "" {
	cfg.Locale = v
}

// loadFlags() — gated on fs.Changed (flag default "" ⇒ unset flags are skipped; cfg.Format keeps "auto"):
if fs.Changed("format") {
	if v, err := fs.GetString("format"); err == nil {
		cfg.Format = v
	}
}
if fs.Changed("locale") {
	if v, err := fs.GetString("locale"); err == nil {
		cfg.Locale = v
	}
}

// Load() tail — the SINGLE post-resolution validation (after every layer + the v3 migration):
// … existing migration if/else block …
// PRD §9.19 FR-F1: an unknown format mode is a HARD configuration error. Validate the RESOLVED value once
// (not per-layer — a low-layer typo overridden higher is not an error). Locale is deliberately NOT
// validated (FR-F6: free-form, verbatim, no i18n tables).
if err := validateFormat(cfg.Format); err != nil {
	return nil, fmt.Errorf("format: %w", err) // → "config: format: invalid format …" (exit 1)
}

// validateFormat — pure, unit-testable:
func validateFormat(format string) error {
	for _, m := range validFormats {
		if format == m {
			return nil
		}
	}
	return fmt.Errorf("invalid format %q (valid: %s)", format, strings.Join(validFormats, ", "))
}
```

### Integration Points

```yaml
CONFIG STRUCT:
  - add to: internal/config/config.go Config (+ Defaults())
  - fields: Format string (default "auto"); Locale string (default "")

FILE DECODE + MERGE:
  - add to: internal/config/file.go fileGeneration, materialize(), overlay()
  - keys:  [generation].format ; [generation].locale ; REPLACE overlay (standard)

GIT-CONFIG (layer 5):
  - add to: internal/config/git.go loadGitConfig()
  - keys:  stagecoach.format ; stagecoach.locale (gitConfigGet, raw copy, single-word — no camelCase)

ENV (layer 6):
  - add to: internal/config/load.go loadEnv()
  - vars:  STAGECOACH_FORMAT ; STAGECOACH_LOCALE (presence-semantic)

CLI FLAGS (layer 7):
  - add to: internal/cmd/root.go init()  (StringVar, zero default, no shorthand)
  - read:  internal/config/load.go loadFlags() via fs.Changed/fs.GetString

VALIDATION:
  - add to: internal/config/load.go (validateFormat + validFormats; called at Load() tail)
  - rule:  format ∈ {auto, conventional, gitmoji, plain} else hard error (exit 1, names value + set)
  - locale: NO validation (FR-F6)

DOWNSTREAM CONSUMER (out of scope — do NOT implement):
  - S2 (P1.M2.T1.S2): compiled-in gitmoji reference table (build-time constant).
  - S3 (P1.M2.T1.S3): format-mode prompt scaffolds + the locale one-line append, applied at every message
    site (message role, planner single-call shortcut, arbiter N+1) — consumes cfg.Format/cfg.Locale.
  - bootstrap.go / config init: NOT touched (FR-B1 doesn't list these; P1.M7 owns doc/stale sweeps).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/config/config.go internal/config/file.go internal/config/load.go \
  internal/config/git.go internal/cmd/root.go
go build ./...
go vet ./internal/config/... ./internal/cmd/...
golangci-lint run ./internal/config/... ./internal/cmd/...   # repo uses .golangci.yml

# Expected: zero errors. The `unused` linter is satisfied because flagFormat/flagLocale are referenced by
# &flagFormat/&flagLocale in StringVar (same idiom as flagProvider/flagModel/flagExclude).
```

### Level 2: Unit Tests (Component Validation)

```bash
# Validator (pure) + materialize/overlay (replace) + Defaults.
go test ./internal/config/... -run 'ValidateFormat|Format|Locale|Overlay|Materialize|Defaults' -v

# Full precedence (5 layers) + git-config + env + flag.
go test ./internal/config/... -run 'Load|LoadEnv|LoadGitConfig' -v
go test ./internal/config/... ./internal/cmd/...

# Required test cases (table-driven where natural):
#  1. validateFormat: each of {auto, conventional, gitmoji, plain} → nil.
#  2. validateFormat: {"", "emoji", "Conventional", "AUTO", "gitmojii", " auto"} → non-nil error whose
#     message Contains the quoted offending value AND "auto, conventional, gitmoji, plain".
#  3. Defaults(): cfg.Format == "auto"; cfg.Locale == "".
#  4. materialize: fileGeneration{Format:"conventional", Locale:"fr"} → Config.Format/Locale set.
#  5. overlay REPLACE: dst.Format="auto", src.Format="plain" → "plain" (replace, NOT union — contrast Exclude).
#  6. overlay empty src → dst unchanged.
#  7. Load precedence (format): global="auto" repo="conventional" git="gitmoji" env="plain" flag unset →
#     "gitmoji" (env unset path); then flag="conventional" → "conventional" (flag wins). Each layer beats
#     the one below; nothing set → "auto".
#  8. Load precedence (locale): same 5-layer ladder; nothing set → "".
#  9. Load bad format (each layer as the resolved source): --format emoji / STAGECOACH_FORMAT=emoji /
#     stagecoach.format=emoji / [generation].format=emoji → Load() returns error Contains "invalid format"
#     AND "emoji" AND the valid set; the higher-layer-good-override case (--format conventional over a
#     bad stagecoach.format=emoji) → NO error (proves post-resolution, not per-layer).
# 10. Load locale free-form: "日本語", "en-US", "weird!!!" → no error, stored verbatim (no normalization).
# 11. loadGitConfig: setGitConfig stagecoach.format=gitmoji + stagecoach.locale=de → c.Format/c.Locale set;
#     missing keys ⇒ unchanged (TestLoadGitConfig_MissingKeysIgnored still green).
# 12. CLI flag: fs.Set("format","plain") → cfg.Format=="plain"; fs.Set("locale","ja") → cfg.Locale=="ja".

# Expected: all pass. Note: a Load() test that fs.Set a flag MUST have newFlagSet registered it.
```

### Level 3: Integration Testing (build the binary, exercise the flag)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach

# Confirm both flags are registered (--help; no shorthand expected):
/tmp/stagecoach --help 2>&1 | grep -E '\-\-format|\-\-locale'

# Hard-error path — unknown format aborts at load (exit 1, before any generation):
tmp=$(mktemp -d); cd "$tmp"; git init -q; printf 'a\n' > a.txt; git add a.txt
/tmp/stagecoach --format emoji --dry-run; echo "exit=$?"
# Expected: stderr "config: format: invalid format \"emoji\" (valid: auto, conventional, gitmoji, plain)";
# exit=1. No generation, no commit.

# Valid path — accepted, stored (dry-run just to exercise load; S3 applies it later):
/tmp/stagecoach --format conventional --locale fr --dry-run --no-color 2>&1 | head -5; echo "exit=$?"
# Expected: proceeds to generation (or the normal nothing/exit path); NO config error. exit != 1-from-config.

# Env + git-config surfaces:
STAGECOACH_FORMAT=gitmoji /tmp/stagecoach --dry-run 2>&1 | head -3
git config stagecoach.locale de && /tmp/stagecoach --dry-run 2>&1 | head -3
# Expected: no config error from either surface.
cd - >/dev/null; rm -rf "$tmp"
```

### Level 4: Cross-cutting / Regression

```bash
# Full suite — the new overlay branches must not perturb Exclude's UNION, BinaryExtensions' REPLACE, or any
# scalar merge; the new Load() tail must not change the happy path for repos that never set format/locale:
go test ./...

# Docs lint (repo uses markdownlint config):
npx --yes markdownlint-cli docs/cli.md docs/configuration.md 2>/dev/null || \
  echo "markdownlint not available; verify table columns align manually (count columns vs existing rows)"

# Expected: all Go tests pass; docs tables render (column count matches existing rows). A repo with no
# format/locale set anywhere resolves Format=="auto" (no validation error) — the common case is untouched.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...`, `go vet`, `golangci-lint` clean; `gofmt` no diff.
- [ ] `go test ./...` passes (config + cmd + full suite).

### Feature Validation
- [ ] `cfg.Format` resolves through flag > env > git-config > repo-file > global-file > `"auto"` default.
- [ ] `cfg.Locale` resolves through the same 5 layers; default `""` when nothing set.
- [ ] `cfg.Format == "auto"` when nothing sets it (no validation boot-loop).
- [ ] Unknown resolved format → `Load()` error naming value + valid set → CLI exit 1 with
      `config: format: invalid format "X" (valid: auto, conventional, gitmoji, plain)`.
- [ ] A GOOD higher-layer value overrides a BAD lower-layer value with NO error (post-resolution, not per-layer).
- [ ] `cfg.Locale` accepts any string verbatim; never validated, never normalized (FR-F6).
- [ ] All four surfaces present: `[generation].format`/`locale`, `stagecoach.format`/`locale`,
      `STAGECOACH_FORMAT`/`LOCALE`, `--format`/`--locale`.

### Code Quality Validation
- [ ] Format/Locale declared as scalars in the `[generation]` group (not the list group); Defaults explicit.
- [ ] `overlay()` REPLACE for format/locale (standard) while Exclude still UNIONS and BinaryExtensions still
      REPLACES (regression-safe — the three merge behaviors are all correct).
- [ ] `validateFormat` pure + unexported; `validFormats` unexported; single call at Load() tail.
- [ ] flagFormat/flagLocale read only via fs.Changed/fs.GetString (never the package var directly).
- [ ] Help text disambiguates `--format` from `config init --template`.

### Scope Boundaries (do NOT cross)
- [ ] No gitmoji reference table (S2 — P1.M2.T1.S2).
- [ ] No format-mode prompt scaffolds / locale line / application at message sites (S3 — P1.M2.T1.S3).
- [ ] No changes to `internal/prompt/*`, `generate.go`, `decompose/*`, `pkg/stagecoach` prompt calls.
- [ ] No changes to `bootstrap.go` / `exampleConfigTemplate` / `GenerateBootstrapConfig` (P1.M7 owns doc sweeps).
- [ ] No `validateLocale` (FR-F6 forbids locale validation — permanent).

### Documentation & Deployment
- [ ] `docs/cli.md`: 2 Global-flags rows + 2 map rows (--format/--locale).
- [ ] `docs/configuration.md`: `[generation]` keys + defaults rows + env rows + git-config rows; FR-F1
      hard-error and FR-F6 no-validation stated verbatim.
- [ ] No new env vars or flags beyond the four specified (format/locale × file/git/env/flag).

---

## Anti-Patterns to Avoid
- ❌ Don't copy Exclude's UNION merge — format/locale are SCALARS; use standard REPLACE (copy Provider/Model).
- ❌ Don't leave `Format` default as `""` in `Defaults()` — it's NON-empty ("auto") or validation boot-loops.
- ❌ Don't validate per-layer — validate the RESOLVED value ONCE at the Load() tail (overridden typos aren't errors).
- ❌ Don't add `validateLocale` — FR-F6 forbids locale validation (free-form, verbatim, no i18n tables).
- ❌ Don't set the flag/env default to "auto" — keep "" (zero) so fs.Changed/LookupEnv presence semantics work.
- ❌ Don't add a shorthand to `--format`/`--locale` (PRD §15.2 gives none).
- ❌ Don't use StringArray for `--format`/`--locale` — they are single-value, not repeatable.
- ❌ Don't validate format inside loadGitConfig/materialize/loadEnv/loadFlags — raw copy only; one check in Load().
- ❌ Don't touch bootstrap.go / prompt scaffolds / message sites — that's S2/S3/P1.M7 (scope fence).
- ❌ Don't confuse `--format` (message shaping) with `config init --template` — disambiguate in help text.

---

## Confidence Score

**9/10** for one-pass implementation success. The change is a faithful, mechanical application of the
already-shipped scalar cascade (`Provider`/`Model`/`Reasoning`: file → git → env → flag, non-zero replace)
to two new string fields, with three live sibling scalars to copy line-for-line and the `configVersionNotice`
precedent for the pure post-resolution helper. Every file, struct field, function, and line-region is named;
the one novel element (enum validation) is small, pure, and fully specified (valid set, error shape, single
call site). The −1 is two tractable risks: (1) the "auto" default is non-empty and MUST be set in `Defaults()`
or validation boot-loops every unconfigured repo — clearly flagged but easy to miss; and (2) the four doc
tables must keep exact column counts — mitigated by the explicit row templates and the Level-4 markdownlint
check. The task is disjoint in FILES from the in-flight P1.M1.T2.S1 (git/diffs) and disjoint in REGIONS from
the landed P1.M1.T1.S1 (Exclude) — no merge conflict either way.
