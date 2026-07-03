name: "P1.M1.T1.S2 — .stagehandignore parser + gitignore-glob → :(exclude,glob) pathspec translator"
description: |
  Add the second and third exclusion sources of PRD §9.18: a `.stagehandignore` loader (one glob per
  line, `#`/blank ignored, `!` skipped with a --verbose warning, missing file = no-op) and a PURE
  translation function that turns gitignore-style globs into git `:(exclude,glob)<pattern>` pathspecs.
  Ship `ResolveExcludePathspecs(cfg, repoRoot, v) ([]string, error)` returning the translated union of
  `.stagehandignore` ∪ `cfg.Exclude` (the raw glob set S1 already resolved). Consumed by P1.M1.T2.S1,
  which threads the result into `StagedDiffOptions.Excludes` on all three diff paths. This subtask does
  NOT touch any diff path, does NOT emit `[excluded]` placeholders, and does NOT resolve the config
  union (that is S1, already landed).

---

## Goal

**Feature Goal**: A repo can carry a `.stagehandignore` file of gitignore-style globs, and every user
exclusion glob (from `.stagehandignore` and from `cfg.Exclude`) is faithfully translated into a git
`:(exclude,glob)` pathspec that behaves like gitignore (`*` stops at `/`, `**` spans components,
anchoring and `dir/` honored). The single entry point returns the ready-to-use pathspec union.

**Deliverable**: A new package `internal/exclude` exposing:
1. `TranslatePattern(glob string) string` — pure gitignore-glob → `:(exclude,glob)<core>` translator
   (golden-table tested; the load-bearing logic).
2. `LoadStagehandIgnore(repoRoot string, v *ui.Verbose) ([]string, error)` — reads `<repoRoot>/.stagehandignore`,
   returns the raw (untranslated) globs; skips blank/`#` lines; skips `!` lines with a `--verbose`
   warning; missing file ⇒ `(nil, nil)`.
3. `ResolveExcludePathspecs(cfg config.Config, repoRoot string, v *ui.Verbose) ([]string, error)` —
   translated union of `LoadStagehandIgnore(...)` ∪ `cfg.Exclude`, each run through `TranslatePattern`.

Plus one small `internal/ui` addition: `func (v *Verbose) VerboseWarn(msg string)` for the `!`-line
warning (same nil/off no-op guard idiom as the other `Verbose*` methods).

Plus docs: a new `## .stagehandignore` section in `docs/configuration.md` (Mode A — rides WITH this
subtask): syntax, the no-negation rule, and the FR-X5 sentence "excluded from what the agent sees,
still committed."

**Success Definition**:
- `TranslatePattern` maps every golden-table row in `research/gitignore-to-pathspec.md` exactly.
- `LoadStagehandIgnore` parses per FR-X2 (blank/`#` ignored, `!` skipped+warned, missing ⇒ nil,nil).
- `ResolveExcludePathspecs` returns `translate(.stagehandignore globs) ++ translate(cfg.Exclude)`.
- A real-git integration test confirms the emitted pathspecs actually exclude the intended paths
  (anchored, any-depth, `dir/`, embedded `*`, `**`) when handed to `git diff -- <pathspec…>`.
- `go build ./...`, `go test ./internal/exclude/... ./internal/ui/...`, `go vet`, `golangci-lint` pass.

## Why

- **FR-X1 (PRD §9.18)** unions four exclusion sources. S1 (already landed) delivered sources (c)
  `[generation].exclude` and (d) `--exclude/-x` as raw globs in `cfg.Exclude`, plus the union machinery.
  THIS subtask delivers source (b) `.stagehandignore` AND the translation step that every user source
  needs before it can reach git. Source (a) the built-in denylist stays separate — it is already
  pre-translated in `internal/git/git.go` (`defaultExcludes`) and is NOT this function's concern.
- **FR-X2** fixes the exact `.stagehandignore` syntax and the no-negation rule; the translation is
  non-obvious because git's default pathspec matching is `fnmatch` WITHOUT `FNM_PATHNAME` (`*` crosses
  `/`), so gitignore semantics REQUIRE the `:(glob)` magic word (architecture/external_deps.md §4).
- Output is the input contract for **P1.M1.T2.S1**, which sets `StagedDiffOptions.Excludes` from this
  slice on the staged / working-tree / tree-to-tree diff paths. Getting the pathspec strings right here
  is a prerequisite for the whole §9.18 feature actually hiding files from the agent.

## What

`ResolveExcludePathspecs` produces a `[]string` of `:(exclude,glob)…` pathspecs. Given:

```
# .stagehandignore (repo root)
*.min.js          # any-depth
/dist/            # root dist/ dir contents only
!keep.min.js      # SKIPPED — pathspecs cannot un-exclude (warned at --verbose)
                  # (blank line above ignored)
```
and `cfg.Exclude == ["testdata/*", "vendor/"]` (from S1),
→ result (order: ignore-file first, then cfg.Exclude):
```
[":(exclude,glob)**/*.min.js", ":(exclude,glob)dist/**",
 ":(exclude,glob)testdata/*", ":(exclude,glob)**/vendor/**"]
```
and stderr (only with `--verbose`) shows one `DEBUG: ...` warning for the skipped `!keep.min.js` line.

### Success Criteria

- [ ] `internal/exclude/exclude.go` created with the three exported functions + the `.stagehandignore`
      filename constant.
- [ ] `TranslatePattern` implements the 5 rules (anchor strip, `dir/`→`/**`, any-depth `**/` prefix,
      `:(exclude,glob)` wrap) and passes the golden table verbatim.
- [ ] `LoadStagehandIgnore` skips blank + `#` lines, skips `!` lines with a `VerboseWarn`, returns raw
      globs, `(nil,nil)` on missing file, real error only on non-ENOENT read failure.
- [ ] `ResolveExcludePathspecs` = translate(ignore-file) ++ translate(cfg.Exclude); order preserved.
- [ ] `ui.Verbose.VerboseWarn` added with the standard `v==nil || v.w==nil || !v.on` no-op guard.
- [ ] Golden-table unit tests for `TranslatePattern`; loader tests (blank/comment/`!`/missing/CRLF);
      one real-git integration test proving the semantics.
- [ ] `docs/configuration.md` gains the `## .stagehandignore` section (syntax + no-negation + FR-X5).
- [ ] NO diff-path changes, NO `[excluded]` placeholder work, NO config-resolution changes.

## All Needed Context

### Context Completeness Check

_This PRP names the new package + every signature, the exact translation rules with a worked golden
table, the sibling patterns to copy (git temp-repo tests, Verbose no-op guard), the one ui method to
add, the import-cycle reasoning, and the precise scope fence against S1 (upstream) and T2.S1
(downstream). An implementer with no prior codebase knowledge can complete it._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/architecture/external_deps.md
  why: §4 is the VERIFIED (gitglossary, 2026-07-02) spec for pathspec exclusion + glob magic — the
       authority for why translation is needed and how anchoring/glob behave.
  section: "## 4. git pathspec exclusion (gates FR-X2/FR-X3)"
  critical: |
    Default matching is fnmatch WITHOUT FNM_PATHNAME (`*` crosses `/`) → gitignore semantics REQUIRE
    the `glob` magic: `:(exclude,glob)<pattern>` with leading-`/` anchor mapping and `dir/`→`dir/**`.
    Standalone excludes are valid (no non-exclude pathspec needed). Pathspecs have NO negation → `!`
    lines are skipped with a --verbose warning, never an error.

- docfile: plan/005_c38aa48290f0/P1M1T1S2/research/gitignore-to-pathspec.md
  why: The full translation rule set + the GOLDEN TABLE (author it as the unit test verbatim) + the
       one directory-recursion nuance to verify against real git.
  section: "Translation rules" and "Worked golden-table rows"
  critical: Emit ONE pathspec per input pattern; the golden table is the string contract; the git
            integration test is the semantic contract.

- file: plan/005_c38aa48290f0/P1M1T1S1/PRP.md
  why: UPSTREAM CONTRACT (implemented in parallel). It delivers cfg.Exclude — the resolved UNION of RAW
       (untranslated) globs from [generation].exclude (global+repo) and --exclude/-x. Assume it exists
       exactly as specified.
  pattern: cfg.Exclude is []string of raw globs like ["*.lock","vendor/*"] — NEVER pre-translated.
  gotcha: Do NOT re-resolve config or touch loadFlags/overlay/materialize — S1 owns all of that.

- file: internal/config/config.go
  why: Config.Exclude ALREADY EXISTS (S1 landed it) — lines ~85-90 (field) and ~146 (Defaults nil).
  pattern: |
    Exclude []string `toml:"exclude"`  // raw gitignore-style globs, UNION-merged; consumed here.
  gotcha: This subtask only READS cfg.Exclude. If for some reason S1 has not landed the field yet at
          implementation time, add it exactly as S1's PRP specifies — but do not duplicate its merge logic.

- file: internal/git/git.go
  why: Shows the DOWNSTREAM shape this feeds and the SEPARATE built-in denylist that is NOT ours.
  pattern: |
    - StagedDiffOptions.Excludes []string (lines 33-42): the consumer field, e.g. []string{":!*.lock"}.
      Comment already says entries are pathspec magic-prefix excludes — our output is exactly this shape
      (`:(exclude,glob)…` is an equivalent, richer spelling of `:!…`).
    - defaultExcludes (lines ~551-553): the built-in noise filter, ALREADY translated (`:!*.lock` …).
      This is FR-X1 source (a); it lives in git.go and is applied there. Do NOT reproduce or merge it
      into ResolveExcludePathspecs (our output is sources (b)+(c)+(d) only).
  gotcha: |
    git.go currently REPLACES defaultExcludes when opts.Excludes is non-empty (line ~656). How the
    user union combines with the denylist is a T2.S1 decision — NOT this subtask. We only produce the
    user pathspec slice. The `:!` vs `:(exclude,glob)` spellings are interchangeable to git.

- file: internal/ui/verbose.go
  why: Add VerboseWarn here; copy the EXACT no-op guard used by VerboseCommand/VerboseRawOutput.
  pattern: |
    func (v *Verbose) VerboseWarn(msg string) {
        if v == nil || v.w == nil || !v.on { return }
        fmt.Fprintln(v.w, "DEBUG: "+msg)
    }
  gotcha: The writer/on fields are unexported; the method must live in package ui. Follow the "DEBUG: "
          prefix convention (commit-pi parity) already used by every Verbose method.

- file: internal/git/stagediff_test.go
  why: The temp-repo git-integration test PATTERN to copy for the semantic (real-git) validation.
  pattern: |
    TestStagedDiff_ExcludesLockSnapMapVendor / TestStagedDiff_CustomExcludesOverride create a temp
    repo, write files, `git add`, then assert a diff with Excludes hides the right paths. Reuse the
    same temp-repo+runGit helper style to feed our translated pathspecs to `git diff -- <pathspec…>`
    and assert the excluded paths are absent while a sibling non-matching path survives.
  gotcha: These tests exec the real git binary (present in CI). Guard with a t.Skip if git is absent,
          matching the existing git-package test convention.

- file: internal/prompt/payload.go
  why: Example of a small, single-responsibility internal package (pure functions + one type) to mirror
       for file/package structure and doc-comment density.
  pattern: package-level doc comment citing the PRD FR, exported funcs with FR-referencing comments.
```

### Current Codebase tree (relevant slice)

```bash
internal/
  config/
    config.go        # Config.Exclude []string ALREADY present (S1); read-only here
  git/
    git.go           # StagedDiffOptions.Excludes (consumer), defaultExcludes (separate denylist)
    stagediff_test.go# temp-repo + git-diff-exclude test PATTERN to copy
  ui/
    verbose.go       # Verbose sink; ADD VerboseWarn here
    verbose_test.go  # add a VerboseWarn test
  prompt/            # exemplar small pure-function package
docs/
  configuration.md   # ADD ## .stagehandignore section (Mode A)
```

### Desired Codebase tree (files added / changed)

```bash
internal/exclude/exclude.go         # NEW — TranslatePattern, LoadStagehandIgnore, ResolveExcludePathspecs, filename const
internal/exclude/exclude_test.go    # NEW — golden-table + loader tests + real-git integration test
internal/ui/verbose.go              # + VerboseWarn method
internal/ui/verbose_test.go         # + VerboseWarn no-op/on test
docs/configuration.md               # + ## .stagehandignore section (syntax, no-negation, FR-X5)
```

### Known Gotchas & Library Quirks

```go
// CRITICAL: gitignore ≠ default pathspec. A bare pathspec `*.log` uses fnmatch WITHOUT FNM_PATHNAME —
// `*` crosses `/`. You MUST prepend the glob magic word: ":(exclude,glob)". Without it the translation
// is silently wrong (external_deps.md §4).

// CRITICAL: any-depth prefix. gitignore `*.log` matches at EVERY depth. Under :(glob) you must emit
// "**/*.log" (git's "**/foo matches foo anywhere, incl. root"). Only patterns with NO leading/middle
// slash get the "**/" prefix; anchored (leading `/`) or middle-slash patterns do NOT.

// CRITICAL: dir/ → dir/** . A trailing slash means "directory + contents"; append "/**". A trailing
// slash does NOT anchor (it is not a leading/middle separator), so `vendor/` still gets the any-depth
// "**/" prefix → "**/vendor/**".

// CRITICAL: NO negation. A line starting with "!" is SKIPPED (never translated) + one VerboseWarn.
// Never return an error for it. Missing .stagehandignore ⇒ (nil, nil), also never an error.

// CRITICAL: output is sources (b)+(c)+(d) ONLY. Do NOT fold in git.go's defaultExcludes (source (a)) —
// that denylist is applied separately inside git.go. ResolveExcludePathspecs returns the USER union
// (.stagehandignore ∪ cfg.Exclude), translated.

// GOTCHA: import cycle. internal/exclude imports internal/config (for Config) and internal/ui (for
// Verbose). config imports NEITHER; ui imports neither. So the graph is acyclic. Do NOT import
// internal/git here (repoRoot is a plain string param — no git dependency needed).

// GOTCHA: CRLF + trailing whitespace. Strip a trailing "\r" (Windows-authored .stagehandignore) and
// trailing spaces before classifying a line. A "#" only starts a comment at column 0 (after trim) —
// gitignore treats "#" mid-pattern literally; keep it simple: trim, then check prefix.

// GOTCHA: leading-"#"/"!" escape. gitignore allows "\#" / "\!" to mean a literal leading # / !.
// This is an edge case; handle it if trivial (strip one leading backslash before the # / !), else
// document it as unsupported in the section. Do not block on it.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/exclude/exclude.go
package exclude

// StagehandIgnoreFile is the fixed repo-root filename (PRD §9.18 FR-X1b/FR-X2).
const StagehandIgnoreFile = ".stagehandignore"

// TranslatePattern converts one gitignore-style glob (relative to the repo root) into a single git
// `:(exclude,glob)<core>` pathspec (PRD §9.18 FR-X2; architecture/external_deps.md §4).
func TranslatePattern(glob string) string

// LoadStagehandIgnore reads <repoRoot>/.stagehandignore and returns the raw (untranslated) globs.
// Blank and `#`-comment lines are ignored; `!` (negation) lines are skipped with a VerboseWarn
// (FR-X2: pathspecs cannot un-exclude). A missing file returns (nil, nil). Only a non-ENOENT read
// error is returned as err.
func LoadStagehandIgnore(repoRoot string, v *ui.Verbose) ([]string, error)

// ResolveExcludePathspecs returns the translated UNION of the .stagehandignore globs and cfg.Exclude
// (the S1-resolved raw union), each run through TranslatePattern. Order: ignore-file globs first, then
// cfg.Exclude. Consumed by P1.M1.T2.S1 as StagedDiffOptions.Excludes.
func ResolveExcludePathspecs(cfg config.Config, repoRoot string, v *ui.Verbose) ([]string, error)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/ui/verbose.go
  - IMPLEMENT: func (v *Verbose) VerboseWarn(msg string) printing "DEBUG: "+msg+"\n".
  - FOLLOW pattern: VerboseCommand's exact guard `if v == nil || v.w == nil || !v.on { return }`.
  - PLACEMENT: next to VerboseRetry; add a doc comment citing FR-X2 (the .stagehandignore `!` warning).

Task 2: MODIFY internal/ui/verbose_test.go
  - IMPLEMENT: test VerboseWarn writes "DEBUG: <msg>" when on, writes nothing when off / nil writer.
  - FOLLOW pattern: existing VerboseCommand/VerboseRetry table tests (bytes.Buffer sink).

Task 3: CREATE internal/exclude/exclude.go
  - IMPLEMENT: StagehandIgnoreFile const; TranslatePattern (the 5 rules); LoadStagehandIgnore; ResolveExcludePathspecs.
  - TranslatePattern rules (research/gitignore-to-pathspec.md):
      1. strip a leading "/" (record anchored=true).
      2. strip a trailing "/" (record dirOnly=true).
      3. hasInternalSlash = remaining contains "/".
      4. core = dirOnly ? p+"/**" : p ; if !anchored && !hasInternalSlash: core = "**/"+core.
      5. return ":(exclude,glob)"+core.
  - LoadStagehandIgnore: os.ReadFile(filepath.Join(repoRoot, StagehandIgnoreFile)); on os.IsNotExist ⇒
      return nil,nil; split lines; per line: strip trailing "\r", TrimSpace; skip "" and "#..."; if
      HasPrefix "!" ⇒ v.VerboseWarn("skipping unsupported negation in .stagehandignore: "+line) and
      continue; else append raw.
  - ResolveExcludePathspecs: globs,err := LoadStagehandIgnore(...); if err return; out := make([]string,0);
      for each ignore glob append TranslatePattern(g); for each cfg.Exclude glob append TranslatePattern(g);
      return out,nil. (Empty union ⇒ empty/nil slice, nil err — a valid no-exclusions result.)
  - FOLLOW pattern: internal/prompt/payload.go (package doc comment, FR-referencing func comments).
  - DEPENDENCIES: Task 1 (VerboseWarn).

Task 4: CREATE internal/exclude/exclude_test.go
  - IMPLEMENT: (a) golden-table TestTranslatePattern covering every row in research + at least: `*.lock`,
      `/*.lock`, `vendor/`, `/build/`, `docs/*.md`, `node_modules`, `a/**/b.go`, `**/foo.txt`, `src/gen*.ts`.
    (b) TestLoadStagehandIgnore: blank+comment ignored; `!` skipped and warns (assert buffer contains
      DEBUG); CRLF handled; missing file ⇒ nil,nil; a genuine read error path (e.g. repoRoot is a file).
    (c) TestResolveExcludePathspecs: ignore-file globs THEN cfg.Exclude, all translated, order preserved;
      cfg.Exclude-only (no file); file-only (empty cfg); both empty ⇒ empty.
    (d) TestPathspecsBehaveLikeGitignore (real git): temp repo; create top-level + nested files matching
      anchored/any-depth/dir/embedded-* patterns; run `git diff --cached --name-only -- <pathspecs...>`
      (or reuse the git-package helper) and assert the intended paths are excluded while a sibling path
      that must NOT match survives. Verify the directory-recursion nuance flagged in research.
  - FOLLOW pattern: internal/git/stagediff_test.go temp-repo + runGit helper; t.Skip if git missing.

Task 5: MODIFY docs/configuration.md  [Mode A — rides WITH this subtask]
  - ADD a "## .stagehandignore" section: it lives at the repo root; one gitignore-style glob per line;
    blank + `#` lines ignored; **negation (`!`) is NOT supported** (patterns become git `:(exclude)`
    pathspecs, which cannot un-exclude — `!` lines are skipped with a `--verbose` warning); missing
    file = no-op. Unions with `[generation].exclude` and `--exclude/-x` (cross-ref §9.18 / the existing
    exclude docs S1 added). State the FR-X5 guarantee verbatim: "excluded from what the agent sees,
    still committed." Note there is NO env var / git-config key for exclusions.
  - PLACEMENT: near the exclusion material S1 added; mirror the existing section heading style.
```

### Implementation Patterns & Key Details

```go
// TranslatePattern — the load-bearing pure function.
func TranslatePattern(glob string) string {
    p := glob
    anchored := strings.HasPrefix(p, "/")
    if anchored {
        p = p[1:]
    }
    dirOnly := strings.HasSuffix(p, "/")
    if dirOnly {
        p = strings.TrimSuffix(p, "/")
    }
    hasInternalSlash := strings.Contains(p, "/")

    core := p
    if dirOnly {
        core += "/**" // FR-X2 note: dir/ → dir/** (exclude the directory's contents)
    }
    if !anchored && !hasInternalSlash {
        core = "**/" + core // any-depth: gitignore pattern w/o leading/middle slash matches everywhere
    }
    return ":(exclude,glob)" + core
}

// LoadStagehandIgnore — parse loop.
data, err := os.ReadFile(filepath.Join(repoRoot, StagehandIgnoreFile))
if errors.Is(err, os.ErrNotExist) {
    return nil, nil // FR-X2: missing file = no-op
}
if err != nil {
    return nil, fmt.Errorf("read %s: %w", StagehandIgnoreFile, err)
}
for _, raw := range strings.Split(string(data), "\n") {
    line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
    if line == "" || strings.HasPrefix(line, "#") {
        continue
    }
    if strings.HasPrefix(line, "!") {
        v.VerboseWarn("ignoring unsupported negation pattern in .stagehandignore: " + line)
        continue // FR-X2: pathspecs have no un-exclude
    }
    globs = append(globs, line)
}
```

### Integration Points

```yaml
UPSTREAM (already done — consume, do not modify):
  - internal/config/config.go: cfg.Exclude []string (raw union from S1).

THIS SUBTASK:
  - internal/exclude/exclude.go: TranslatePattern, LoadStagehandIgnore, ResolveExcludePathspecs.
  - internal/ui/verbose.go: VerboseWarn.

DOWNSTREAM (out of scope — do NOT implement):
  - P1.M1.T2.S1: call ResolveExcludePathspecs at the diff call sites (default_action.go / generate /
    decompose), pass repoRoot (os.Getwd() today) + the *ui.Verbose, set StagedDiffOptions.Excludes,
    decide how the user union combines with git.go defaultExcludes, and emit `[excluded]` placeholders.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagehand-competitor-feature-parity
gofmt -w internal/exclude/exclude.go internal/exclude/exclude_test.go internal/ui/verbose.go
go build ./...
go vet ./internal/exclude/... ./internal/ui/...
golangci-lint run ./internal/exclude/... ./internal/ui/...
# Expected: zero errors.
```

### Level 2: Unit Tests

```bash
go test ./internal/exclude/... -run 'Translate|Load|Resolve' -v
go test ./internal/ui/... -run 'VerboseWarn' -v

# Required cases:
#  Translate golden table: *.lock→:(exclude,glob)**/*.lock ; /*.lock→:(exclude,glob)*.lock ;
#    vendor/→:(exclude,glob)**/vendor/** ; /build/→:(exclude,glob)build/** ;
#    docs/*.md→:(exclude,glob)docs/*.md ; node_modules→:(exclude,glob)**/node_modules ;
#    a/**/b.go→:(exclude,glob)a/**/b.go ; **/foo.txt→:(exclude,glob)**/foo.txt ;
#    src/gen*.ts→:(exclude,glob)src/gen*.ts
#  Loader: blank/# ignored ; ! skipped + VerboseWarn emitted ; CRLF stripped ; missing⇒nil,nil ;
#    read error surfaced (repoRoot points at a file, not a dir).
#  Resolve: [ignore-globs...]++[cfg.Exclude...] translated, order preserved ; each source alone ; both empty⇒empty.
# Expected: all pass.
```

### Level 3: Integration (real git — the SEMANTIC contract)

```bash
go test ./internal/exclude/... -run 'BehaveLikeGitignore' -v
# The test builds a temp repo with, e.g.:
#   top.lock, sub/nested.lock, dist/x.js, keep.js, docs/readme.md, sub/docs/other.md
# Translates .stagehandignore {*.lock, /dist/, docs/*.md} and asserts (via git diff --name-only with
# the pathspecs) that top.lock, sub/nested.lock, dist/x.js, docs/readme.md are EXCLUDED while keep.js
# and sub/docs/other.md (docs/*.md is root-anchored) SURVIVE. Pin the dir-recursion nuance here.
# Expected: pass; if the nuance test fails for a bareword dir, apply the research-documented fix.
```

### Level 4: Cross-cutting / Regression

```bash
go test ./...   # no other package changed behavior; ui + exclude green, everything else untouched
npx --yes markdownlint-cli docs/configuration.md 2>/dev/null || echo "verify section renders manually"
# Expected: full suite green; docs section renders.
```

## Final Validation Checklist

### Technical
- [ ] `go build ./...`, `go vet`, `golangci-lint` clean.
- [ ] `go test ./...` passes; `gofmt` no diff.

### Feature
- [ ] `TranslatePattern` matches the golden table exactly (anchor, `dir/`, `**`, embedded `*`, any-depth).
- [ ] `LoadStagehandIgnore`: blank/`#` ignored; `!` skipped + `--verbose` warning; missing⇒nil,nil; real error surfaced.
- [ ] `ResolveExcludePathspecs` = translate(.stagehandignore) ++ translate(cfg.Exclude), order preserved.
- [ ] Real-git integration test proves the pathspecs exclude the intended paths.
- [ ] `ui.Verbose.VerboseWarn` no-ops when off / nil (guard identical to siblings).
- [ ] `docs/configuration.md` `## .stagehandignore` states syntax, no-negation, and FR-X5 payload-only sentence.

### Scope Boundaries (do NOT cross)
- [ ] No config-resolution changes (cfg.Exclude is S1's; read-only here).
- [ ] No diff-path changes / no `StagedDiffOptions.Excludes` wiring (that is T2.S1).
- [ ] No `[excluded]` placeholder emission (T2.S1).
- [ ] No merging with git.go `defaultExcludes` (source (a) is applied separately).
- [ ] No `internal/git` import from `internal/exclude` (repoRoot is a string param).

---

## Anti-Patterns to Avoid
- ❌ Don't emit bare `:!pattern` / `:(exclude)pattern` without `glob` — you lose gitignore semantics.
- ❌ Don't forget the `**/` any-depth prefix for unanchored, no-slash patterns.
- ❌ Don't turn a `!` line into an error or a re-include — skip + warn only.
- ❌ Don't fold in the built-in denylist here — that is git.go's job (source (a)).
- ❌ Don't re-resolve config or duplicate S1's union merge.
- ❌ Don't import internal/git (no dependency needed; repoRoot is passed in).
- ❌ Don't skip the real-git integration test — the golden table checks strings, not semantics.

---

## Confidence Score

**8.5/10** for one-pass success. The package is small, pure, and dependency-light; every sibling
pattern (Verbose no-op guard, git temp-repo tests, small pure-function package) exists to copy, and the
translation rules + golden table are spelled out. The −1.5 is the one genuine unknown: git's exact
recursion behavior for a bareword-directory glob pathspec (`**/node_modules` vs its contents), which the
mandated real-git integration test is designed to pin down and correct if needed — the string-level
golden table is fully deterministic regardless.
