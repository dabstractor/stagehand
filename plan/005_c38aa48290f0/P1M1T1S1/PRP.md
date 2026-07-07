name: "P1.M1.T1.S1 — [generation].exclude config key + repeatable --exclude/-x flag with union merge"
description: |
  Add a new list-valued exclusion source to the resolved Config: the `[generation].exclude` TOML
  key (global + repo files) and a repeatable `--exclude <glob>` / `-x` CLI flag, all merged by
  UNION (accumulate) rather than the scalar override/replace cascade used by every other key today.
  This subtask produces `cfg.Exclude` — the resolved union of RAW (untranslated) gitignore-style
  glob patterns — and nothing else. Translation to `:(exclude,glob)` pathspecs (S2) and wiring into
  the diff paths (T2.S1) are downstream and OUT OF SCOPE here.

---

## Goal

**Feature Goal**: Extend the config resolver so exclusion glob patterns can be supplied via the
`[generation].exclude = ["…"]` config-file array (in BOTH the global and repo-local files) and via a
repeatable `--exclude <glob>` / `-x <glob>` CLI flag, with all sources combined by **union
(accumulation)** into a single resolved `Config.Exclude []string`. This is a *new merge behavior*
distinct from the non-zero scalar cascade and the list-REPLACE used by `BinaryExtensions`.

**Deliverable**:
1. `Config.Exclude []string` field (`internal/config/config.go`) + `Defaults()` initializer.
2. `fileGeneration.Exclude []string` decode field (`internal/config/file.go`).
3. Union merge logic in `materialize()` (single-file copy) and `overlay()` (cross-layer UNION) in
   `internal/config/file.go`.
4. Repeatable `--exclude` / `-x` pflag **StringArray** flag registered in `internal/cmd/root.go`
   `init()`, appended (union) to `cfg.Exclude` in `loadFlags()` (`internal/config/load.go`).
5. Table-driven precedence/union unit tests mirroring existing `load_test.go` / `file_test.go`.
6. Docs: `--exclude/-x` row in `docs/cli.md` global-flags table; `[generation].exclude` key +
   union-not-override note in `docs/configuration.md`.

**Success Definition**:
- `cfg.Exclude` contains the concatenation (union) of every glob from: global-file
  `[generation].exclude`, repo-file `[generation].exclude`, and every `--exclude/-x` occurrence.
- Patterns are stored **raw** (e.g. `*.lock`, `vendor/*`) — NOT translated to `:(exclude)` pathspecs.
- There is **no** `STAGECOACH_*` env var and **no** `stagecoach.*` git-config key for exclusions
  (FR-X1: the env-list quoting trap is deliberately avoided).
- `go build ./...`, `go test ./internal/config/... ./internal/cmd/...`, `go vet`, and `golangci-lint`
  all pass.

## Why

- **FR-X1 (PRD §9.18)** requires exclusion patterns to come from a *union* of four sources: the
  built-in denylist, `.stagecoachignore` (S2), the `[generation].exclude` config array, and the
  repeatable `--exclude/-x` flag. This subtask delivers the two config/flag sources plus the union
  machinery; `.stagecoachignore` and the pathspec translation land in S2, and application to the diff
  paths lands in T2.S1.
- Every existing list-valued key (`BinaryExtensions`) REPLACES across layers. Exclusions must
  ACCUMULATE — a repo cannot be forced to *lose* a globally-excluded pattern. This is the one place
  in the resolver where union semantics are correct, and the architecture note calls it out
  explicitly (see Gotchas).
- Output `cfg.Exclude` is the input contract for P1.M1.T1.S2 (the gitignore-glob → pathspec
  translator) and P1.M1.T2.S1 (threading pathspecs into the three diff paths). Getting the resolved
  set right here is a prerequisite for the entire §9.18 feature.

## What

A user can persist exclusion globs in config and/or pass them ad-hoc on the CLI:

```toml
# ~/.config/stagecoach/config.toml
[generation]
exclude = ["*.min.js", "dist/*"]
```
```toml
# ./.stagecoach.toml
[generation]
exclude = ["testdata/*"]
```
```bash
stagecoach -x '*.snap' --exclude 'coverage/*'
```
→ `cfg.Exclude == ["*.min.js", "dist/*", "testdata/*", "*.snap", "coverage/*"]` (order: global,
then repo, then flags — pure accumulation).

### Success Criteria

- [ ] `Config.Exclude []string` exists with `toml:"exclude"` tag; `Defaults()` sets it `nil`.
- [ ] `fileGeneration.Exclude []string` with `toml:"exclude"` decodes `[generation].exclude`.
- [ ] `materialize()` copies a file's `exclude` array into `Config.Exclude` (non-empty guard).
- [ ] `overlay()` **appends** (union) `src.Exclude` onto `dst.Exclude` — NOT replace.
- [ ] `--exclude` / `-x` is registered as a repeatable **StringArray** persistent flag.
- [ ] `loadFlags()` **appends** flag values to `cfg.Exclude` (union), gated on `fs.Changed("exclude")`.
- [ ] No env var and no git-config key are added for exclusions.
- [ ] Values are stored raw (untranslated), consumed later by S2.
- [ ] Table-driven tests cover: single file, global+repo union, flag append, all-three union, empty/absent.
- [ ] `docs/cli.md` and `docs/configuration.md` updated with union-not-override wording.

## All Needed Context

### Context Completeness Check

_This PRP names every file to edit, the exact struct/function to change, the existing sibling field
to copy from (and the ONE field NOT to copy — `BinaryExtensions`), the test helpers to extend, and
the precise merge semantics. An implementer with no prior codebase knowledge can complete it._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/architecture/system_context.md
  why: §3 "internal/config" bullet states the exact seam and the trap
  section: "internal/config" and "internal/git" package bullets
  critical: |
    "⚠ List merge is REPLACE today (BinaryExtensions). FR-X1 requires `exclude` to UNION across
    sources — a new merge behavior, do not copy the replace pattern." Also: the git Excludes seam
    already exists (StagedDiffOptions.Excludes) but is passed nil by every caller — that wiring is
    S2/T2, NOT this subtask. This subtask stops at cfg.Exclude (raw globs).

- file: internal/config/config.go
  why: Config struct + Defaults() — add Exclude field here following the [generation] block pattern
  pattern: BinaryExtensions field (lines 84-85) is the closest sibling (list-valued, [generation],
           toml snake_case tag) — copy its DECLARATION shape, but its MERGE semantics are the wrong
           model (it replaces; Exclude unions).
  gotcha: Config is flat/plain and NEVER decoded directly from the §16.2 file; the toml tag is
          documentation of the §16.2 leaf name only. Add `Exclude []string \`toml:"exclude"\`` to the
          [generation] group, and `Exclude: nil` to Defaults().

- file: internal/config/file.go
  why: fileGeneration decode struct (lines 49-58), materialize() (186-245), overlay() (258-348)
  pattern: |
    - fileGeneration.BinaryExtensions (line 57) → add `Exclude []string \`toml:"exclude"\`` twin.
    - materialize(): copy `if len(g.Exclude) > 0 { c.Exclude = g.Exclude }` (lines 228-230 show the
      BinaryExtensions copy — same shape).
    - overlay(): DO NOT copy the BinaryExtensions REPLACE line (346-347
      `dst.BinaryExtensions = src.BinaryExtensions`). Instead UNION:
      `if len(src.Exclude) > 0 { dst.Exclude = append(dst.Exclude, src.Exclude...) }`.
  gotcha: overlay() is invoked once per lower layer (global, repo-local, git-config) with cfg as
          dst — the append naturally accumulates global then repo. git-config never sets Exclude
          (no key), so its overlay pass is a no-op for this field. append(nil, …) allocates fresh,
          so there is no aliasing hazard into the fileGeneration slice.

- file: internal/config/load.go
  why: loadFlags() (239-306) is where the CLI flag layer overlays; loadEnv() (174-231) is where env
       vars overlay
  pattern: |
    - loadFlags(): add, gated on fs.Changed("exclude"):
        if vals, err := fs.GetStringArray("exclude"); err == nil {
            cfg.Exclude = append(cfg.Exclude, vals...)  // UNION — flags ADD to the union, not replace
        }
    - loadEnv(): add NOTHING. There is deliberately NO STAGECOACH_EXCLUDE (FR-X1 quoting trap).
  gotcha: For a UNION key "precedence" means accumulate, not override — the flag APPENDS to whatever
          the config files contributed; it does not replace. This is intentional and differs from
          every scalar flag in loadFlags (which set DIRECTLY).

- file: internal/cmd/root.go
  why: init() registers persistent flags (lines 101-151); flag package vars declared 27-65
  pattern: |
    - Declare a package var near the behavioral-flags block: `var flagExclude []string`.
    - Register in init(): pf.StringArrayVarP(&flagExclude, "exclude", "x", nil,
        "Exclude matching files from the agent payload (unions with .stagecoachignore and "+
        "[generation].exclude; never excluded from the commit)")
  gotcha: Use StringArrayVarP (NOT StringSliceVarP). StringSlice comma-splits each value, which
          would corrupt a glob that contains a comma; StringArray treats each -x occurrence as one
          literal value. -x is the shorthand (matches PRD §15.2). flagExclude is read only via
          fs.Changed/fs.GetStringArray in loadFlags — do not read the package var directly.

- file: internal/config/load_test.go
  why: newFlagSet() helper (lines 53-72) MUST register --exclude for Load() integration tests;
       table-driven env/overlay test patterns (lines 137+) to mirror
  pattern: |
    - Add to newFlagSet(): `fs.StringArray("exclude", nil, "")`
    - Mirror TestLoadEnv_* / setRole tests for a new TestOverlay/TestLoad exclude-union test.
  gotcha: newFlagSet is shared by many tests; adding a flag is behavior-neutral for tests that don't
          Set it. Tests that build a *pflag.FlagSet and call Load must have the flag registered or
          fs.Changed panics/errors.

- file: internal/config/file_test.go
  why: existing materialize()/overlay() table-driven unit tests to mirror for the union behavior
  pattern: unit-test overlay() directly with two *Config values to assert append/union, and
           materialize() with a fileConfig to assert single-file copy.

- file: internal/git/git.go
  why: shows the DOWNSTREAM consumer shape (StagedDiffOptions.Excludes, lines 33-42) — confirms this
       subtask stores RAW globs, and S2 translates them to `:!`/`:(exclude,glob)` pathspecs
  gotcha: Do NOT translate here. cfg.Exclude holds `*.lock`, NOT `:!*.lock`. The defaultExcludes in
          git.go are already-translated pathspecs and are a SEPARATE built-in source (FR-X1 item a).
```

### Current Codebase tree (relevant slice)

```bash
internal/
  cmd/
    root.go            # persistent flag registration (init) + PersistentPreRunE → config.Load
    root_test.go
  config/
    config.go          # Config struct (flat, resolved) + Defaults()
    file.go            # fileConfig/fileGeneration decode structs; materialize(); overlay()
    file_test.go
    load.go            # Load() layer pipeline; loadEnv(); loadFlags()
    load_test.go       # newFlagSet() helper; table-driven precedence tests
  git/
    git.go             # StagedDiffOptions.Excludes seam (downstream; nil today)
docs/
  cli.md               # ## Global flags table
  configuration.md     # ## File format / [generation] / ## Git-config keys
```

### Desired Codebase tree (files changed — no new files)

```bash
internal/config/config.go     # + Config.Exclude field; + Exclude: nil in Defaults()
internal/config/file.go       # + fileGeneration.Exclude; + materialize copy; + overlay UNION
internal/config/load.go       # + loadFlags exclude append (UNION); loadEnv unchanged (no env var)
internal/cmd/root.go          # + flagExclude var; + StringArrayVarP("exclude","x") registration
internal/config/load_test.go  # + newFlagSet registers --exclude; + union precedence tests
internal/config/file_test.go  # + materialize/overlay union unit tests
docs/cli.md                   # + --exclude/-x row in Global flags table
docs/configuration.md         # + [generation].exclude key + union-not-override note (per §16.1)
```

### Known Gotchas & Library Quirks

```go
// CRITICAL: overlay() list-merge default is REPLACE (BinaryExtensions). Exclude MUST UNION.
//   WRONG (copied from BinaryExtensions): dst.Exclude = src.Exclude
//   RIGHT:                                if len(src.Exclude) > 0 { dst.Exclude = append(dst.Exclude, src.Exclude...) }

// CRITICAL: pflag StringArray vs StringSlice. StringSlice splits "a,b" into ["a","b"] on every
// value — a comma-bearing glob would be silently split. Use StringArrayVarP so each -x is literal.

// CRITICAL: NO env var, NO git-config key for exclude (FR-X1). Do not touch loadEnv() or
// loadGitConfig()/git.go's config reader for this field. A colon/comma-joined env list is a
// documented quoting trap; config + flag cover the persistent and ad-hoc cases.

// CRITICAL: store RAW globs. This subtask does NOT translate to `:(exclude)` pathspecs — that is
// S2 (P1.M1.T1.S2). cfg.Exclude = ["*.lock"], never [":!*.lock"].

// GOTCHA: union "precedence". For a scalar, flags override env override files. For this UNION key,
// the flag ADDS to the file-contributed set (accumulate). loadFlags appends; it does not replace.

// GOTCHA: order. overlay is called global→repo→gitconfig; loadEnv; then loadFlags. So resolved
// order is [global globs..., repo globs..., flag globs...]. Duplicates are permitted (harmless:
// git dedupes identical pathspecs downstream; do NOT add dedup logic unless a test demands it —
// the contract says "accumulate").
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/config.go — add to the [generation] group of Config (after BinaryExtensions):
Exclude []string `toml:"exclude"` // §9.18 FR-X1 gitignore-style globs, RAW/untranslated; UNION
//                                    across global+repo files AND --exclude/-x (NOT replace). No env
//                                    var, no git-config key. Consumed by S2's translator. nil ⇒ none.

// internal/config/config.go — Defaults(): add
Exclude: nil, // §9.18 FR-X1: no built-in exclude globs at Layer 1 (denylist lives in git.go)

// internal/config/file.go — add to fileGeneration (after BinaryExtensions):
Exclude []string `toml:"exclude"` // V2.1 — §9.18 FR-X1 exclusion globs; UNION-merged in overlay()
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/config/config.go
  - IMPLEMENT: Config.Exclude []string field with toml:"exclude" in the [generation] block
    (place directly after BinaryExtensions, line ~85), with a doc comment noting UNION + raw + no-env.
  - IMPLEMENT: Exclude: nil in Defaults() (place after BinaryExtensions: nil).
  - FOLLOW pattern: BinaryExtensions declaration/default (DECLARATION only — not its merge).
  - PLACEMENT: [generation] group of Config; Defaults() return literal.

Task 2: MODIFY internal/config/file.go
  - IMPLEMENT: fileGeneration.Exclude []string `toml:"exclude"` (after BinaryExtensions, line ~57).
  - IMPLEMENT: materialize() single-file copy: `if len(g.Exclude) > 0 { c.Exclude = g.Exclude }`
    (place next to the BinaryExtensions copy at lines 228-230).
  - IMPLEMENT: overlay() UNION: `if len(src.Exclude) > 0 { dst.Exclude = append(dst.Exclude, src.Exclude...) }`
    — place it near the BinaryExtensions REPLACE (line 346), but with a comment stating it is the
    DELIBERATE exception (union, not replace, per FR-X1 / §16.1).
  - DEPENDENCIES: Task 1 (Config.Exclude must exist).
  - PLACEMENT: fileGeneration struct; materialize()'s [generation] block; overlay()'s V2 scalars block.

Task 3: MODIFY internal/cmd/root.go
  - IMPLEMENT: package var `flagExclude []string` (behavioral-flags var block, ~lines 60-65).
  - IMPLEMENT: pf.StringArrayVarP(&flagExclude, "exclude", "x", nil, "<help text>") in init().
  - FOLLOW pattern: existing BoolVarP(&flagAll, "all", "a", …) shorthand registration style.
  - PLACEMENT: init() alongside the §15.2 behavioral flags.

Task 4: MODIFY internal/config/load.go
  - IMPLEMENT: in loadFlags(), gated on fs.Changed("exclude"):
      if vals, err := fs.GetStringArray("exclude"); err == nil {
          cfg.Exclude = append(cfg.Exclude, vals...)   // UNION — flags accumulate onto the config set
      }
  - CONFIRM: loadEnv() gets NO STAGECOACH_EXCLUDE handling (FR-X1). Add a one-line comment there noting
    the deliberate omission so a future maintainer does not "helpfully" add it.
  - DEPENDENCIES: Tasks 1 & 3.
  - PLACEMENT: loadFlags() after the decompose-flags block.

Task 5: MODIFY internal/config/load_test.go
  - IMPLEMENT: register the flag in newFlagSet(): `fs.StringArray("exclude", nil, "")`.
  - IMPLEMENT: table-driven Load()/overlay union tests (see Validation Loop Level 2 for cases).
  - FOLLOW pattern: TestLoadEnv_* and setRole tests (t.Setenv/chdir/writeConfigFile helpers,
    loadEnvSetup for the global+repo layering).
  - PLACEMENT: new test funcs in load_test.go.

Task 6: MODIFY internal/config/file_test.go
  - IMPLEMENT: direct overlay() union unit test (two *Config, assert appended slice) and
    materialize() single-file copy test (fileConfig → *Config).
  - FOLLOW pattern: existing materialize/overlay table tests in file_test.go.

Task 7: MODIFY docs/cli.md and docs/configuration.md  [Mode A — rides WITH this subtask]
  - docs/cli.md: add a Global-flags table row after --max-commits (line ~36):
      | `--exclude <glob>`, `-x` | string (repeatable) | — | — | — | Exclude matching files from the
      agent payload (placeholder line instead; never excluded from the commit). Unions with
      `.stagecoachignore` and `[generation].exclude`. |
  - docs/configuration.md: document `[generation].exclude` in the File format / [generation] area
    and state plainly it UNIONS across the global and repo files (and with --exclude/-x) rather than
    overriding — cite §16.1 union-not-override semantics. Note there is NO env var / git-config key.
  - PLACEMENT: mirror the existing max_commits documentation rows.
```

### Implementation Patterns & Key Details

```go
// overlay() — THE load-bearing distinction of this subtask:
func overlay(dst, src *Config) {
    // ... existing scalar + BinaryExtensions REPLACE ...
    if len(src.BinaryExtensions) > 0 {
        dst.BinaryExtensions = src.BinaryExtensions // REPLACE (unchanged)
    }
    // §9.18 FR-X1: exclude UNIONS across layers (global → repo), the ONE list key that accumulates
    // rather than replaces (§16.1). A repo must not be able to DROP a globally-excluded glob.
    if len(src.Exclude) > 0 {
        dst.Exclude = append(dst.Exclude, src.Exclude...)
    }
}

// loadFlags() — the flag also UNIONS (accumulate), it does not override the config set:
if fs.Changed("exclude") {
    if vals, err := fs.GetStringArray("exclude"); err == nil {
        cfg.Exclude = append(cfg.Exclude, vals...)
    }
}
```

### Integration Points

```yaml
CONFIG STRUCT:
  - add to: internal/config/config.go Config (+ Defaults())
  - field:  Exclude []string `toml:"exclude"`  (nil default)

FILE DECODE + MERGE:
  - add to: internal/config/file.go fileGeneration, materialize(), overlay()
  - key:    [generation].exclude ; union in overlay()

CLI FLAG:
  - add to: internal/cmd/root.go init()
  - flag:   pf.StringArrayVarP(&flagExclude, "exclude", "x", nil, "...")
  - read:   internal/config/load.go loadFlags() via fs.GetStringArray + append

DOWNSTREAM (out of scope, do not implement):
  - S2 (P1.M1.T1.S2): translate cfg.Exclude globs → :(exclude,glob) pathspecs + .stagecoachignore.
  - T2.S1 (P1.M1.T2.S1): thread pathspecs into StagedDiff/WorkingTreeDiff/TreeDiff + [excluded] placeholders.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/config/config.go internal/config/file.go internal/config/load.go internal/cmd/root.go
go build ./...
go vet ./internal/config/... ./internal/cmd/...
golangci-lint run ./internal/config/... ./internal/cmd/...   # repo uses .golangci.yml

# Expected: zero errors. The `unused` linter is satisfied because flagExclude is referenced by
# &flagExclude in StringArrayVarP (same idiom as flagAll).
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/config/... -run 'Exclude|Overlay|Materialize|Load' -v
go test ./internal/cmd/... -v
go test ./internal/config/... ./internal/cmd/...

# Required test cases (table-driven where natural):
#  1. materialize: fileGeneration{Exclude:["*.lock"]} → Config.Exclude == ["*.lock"].
#  2. overlay union: dst.Exclude=["a"], src.Exclude=["b"] → dst.Exclude == ["a","b"] (append, not replace).
#  3. overlay nil src.Exclude → dst.Exclude unchanged.
#  4. Load global+repo: global [generation].exclude=["g1"], repo=["r1"] → cfg.Exclude==["g1","r1"].
#  5. Load with flags: fs.Set("exclude","f1") twice (or Set two values) + config → union in order
#     [global..., repo..., flag...].
#  6. No env var: STAGECOACH_EXCLUDE set in env is IGNORED (cfg.Exclude does not contain it).
#  7. Absent everywhere → cfg.Exclude == nil (or empty), no panic.

# Expected: all pass. Note: to Set a StringArray flag twice in a test, call fs.Set("exclude","a")
# then fs.Set("exclude","b") — StringArray appends per Set (matching repeated -x on the CLI).
```

### Level 3: Integration Testing (build the binary, exercise the flag)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach

# Confirm the flag is registered with the -x shorthand and repeatable:
/tmp/stagecoach --help 2>&1 | grep -E '\-x, --exclude'

# Confirm end-to-end resolution via --verbose on a throwaway repo (no commit needed — dry-run):
tmp=$(mktemp -d); cd "$tmp"; git init -q; printf 'a\n' > a.txt; git add a.txt
/tmp/stagecoach --dry-run -x '*.snap' --exclude 'dist/*' --verbose 2>&1 | head -30
# Expected: no crash; the two globs are part of the resolved exclusion set once S2/T2 consume them.
# (In THIS subtask they are only resolved into cfg.Exclude; visible plumbing arrives with T2.)
cd - >/dev/null; rm -rf "$tmp"
```

### Level 4: Cross-cutting / Regression

```bash
# Full suite — ensure the new overlay branch did not perturb BinaryExtensions or scalar merges:
go test ./...

# Docs lint (repo uses markdownlint config):
npx --yes markdownlint-cli docs/cli.md docs/configuration.md 2>/dev/null || \
  echo "markdownlint not available; verify table columns align manually"

# Expected: all Go tests pass; docs tables render (column count matches existing rows).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet` clean; `golangci-lint` clean.
- [ ] `go test ./...` passes (config + cmd + full suite).
- [ ] `gofmt` produces no diff.

### Feature Validation
- [ ] `cfg.Exclude` is the ordered union of global-file, repo-file, and `--exclude/-x` globs.
- [ ] Globs stored raw (untranslated) — no `:(exclude)` prefix added here.
- [ ] `--exclude`/`-x` is repeatable (StringArray) and each occurrence is a literal value.
- [ ] No `STAGECOACH_EXCLUDE` env var and no `stagecoach.*` git-config key added.
- [ ] `overlay()` UNIONS exclude while `BinaryExtensions` still REPLACES (regression-safe).

### Code Quality Validation
- [ ] Field declaration mirrors BinaryExtensions; merge deliberately does NOT (documented in comments).
- [ ] flagExclude read only via fs.Changed/GetStringArray (never the package var directly).
- [ ] Test helpers (newFlagSet) extended without breaking existing tests.
- [ ] docs/cli.md + docs/configuration.md state "payload-only, still committed" and union-not-override.

### Scope Boundaries (do NOT cross)
- [ ] No `.stagecoachignore` parsing (that is S2).
- [ ] No glob→pathspec translation (that is S2).
- [ ] No changes to StagedDiff/WorkingTreeDiff/TreeDiff or `[excluded]` placeholders (that is T2.S1).
- [ ] No env var, no git-config key (permanent per FR-X1).

---

## Anti-Patterns to Avoid
- ❌ Don't copy BinaryExtensions' REPLACE merge — Exclude must UNION (append).
- ❌ Don't use StringSlice (comma-splitting) — use StringArray for globs.
- ❌ Don't add STAGECOACH_EXCLUDE or a git-config key (FR-X1 forbids both).
- ❌ Don't translate globs to `:(exclude)` pathspecs here — S2 owns translation.
- ❌ Don't make the flag "override" the config set — it accumulates onto it.
- ❌ Don't add dedup logic unless a test requires it — the contract says accumulate.

---

## Confidence Score

**9/10** for one-pass implementation success. The seam is small, isolated, and explicitly flagged in
the architecture notes; every sibling pattern (BinaryExtensions declaration, per-role flag
registration, table-driven precedence tests) exists to copy. The single non-obvious decision — UNION
vs REPLACE — is called out repeatedly here and in `system_context.md §3`. The −1 is for the mild risk
of a test needing the exact StringArray double-Set idiom, which is documented in Level 2.
