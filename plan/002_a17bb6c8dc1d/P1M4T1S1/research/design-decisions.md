# P1.M4.T1.S1 — Design Decisions & Research Notes

> Research backing `PRP.md`: a load-time advisory that warns when a loaded config file's `config_version`
> is missing / older / newer than `CurrentConfigVersion` (PRD §9.17 FR-B4), routed through the existing
> `noticeOut` sink for testability. Advisory only — no auto-migration.

## 0. `Defaults()` MUST change from `CurrentConfigVersion` → 0 (the load-bearing decision)

The task's stated logic is: *"If cfg.ConfigVersion == 0 (missing) or < CurrentConfigVersion → print
warning."* But the SHIPPED `Defaults()` (P1.M3.T1.S1, `internal/config/config.go`) pins
`ConfigVersion: CurrentConfigVersion` (== 2). Because `materialize()` and `overlay()` use NON-ZERO
guards (`if fc.ConfigVersion != 0` / `if src.ConfigVersion != 0`), a resolved `cfg.ConfigVersion` is
**always ≥ 2** after `Load()` — `== 0` is UNREACHABLE. So the "missing" branch is dead code unless
`Defaults()` leaves the field at its zero value.

**Resolution: `Defaults()` sets `ConfigVersion: 0`** (just remove the `ConfigVersion: CurrentConfigVersion`
line). Then `cfg.ConfigVersion == 0` genuinely means "no source declared a schema version", which is the
exact "missing" sentinel FR-B4 + the task require. `0` is the natural zero-value sentinel for an int,
matching how every other non-zero-overlay field in this package works (Timeout/MaxDiffBytes/etc. all use
0 = unset). This is the ONE edit to a "shipped" file, and it is squarely in P1.M4.T1.S1's scope: this
subtask is what makes `ConfigVersion` meaningful, so it owns the sentinel semantics. Verified no consumer
besides the new advisory reads `ConfigVersion`, and no test outside `internal/config/` references it.

State table under the new design:

| scenario                                     | fileLoaded | cfg.ConfigVersion | advisory         |
|----------------------------------------------|------------|-------------------|------------------|
| no config file at all (pure Defaults)        | false      | 0                 | none (guard)     |
| file(s) loaded, none set config_version      | true       | 0                 | "missing"        |
| file(s) set config_version == current (2)    | true       | 2                 | none (current)   |
| file(s) set config_version < current (1)     | true       | 1                 | "older"          |
| file(s) set config_version > current (3)     | true       | 3                 | "ahead"          |

## 1. The `fileLoaded` guard — no spurious warning on the no-file path

With `Defaults() = 0`, the "no config file" case also yields `cfg.ConfigVersion == 0`. Without a guard it
would fire a misleading "config file has no config_version" warning when there is NO file at all. So the
advisory is gated on "did ANY config file load?". `Load()` already knows this: the global file (`g != nil`
after `loadTOML`) and the repo-local file (`r != nil` after `loadRepoLocalConfig`). Add a `var fileLoaded
bool`, set it true in both `else if != nil` branches, and pass it to the advisory. Git config (Layer 4)
never carries `config_version` (verified: `loadGitConfig` sets no such key — `config_version` is a
file-only metadata concept, PRD §16.1), so `fileLoaded` is purely about FILES. Two extra lines in `Load()`,
clearly worth the correctness.

## 2. Reusing `noticeOut` is SAFE — existing notice tests never assert its content

The task says "Use the existing noticeOut pattern from file.go for testability." `noticeOut` (file.go,
`var noticeOut io.Writer = os.Stderr`, with `SetNoticeOut`/`NoticeOut` accessors) is the §19 repo-provider
notice sink. Reusing it for the config_version advisory is the intended pattern. VERIFIED safe: the only
tests that capture `noticeOut` (load_test.go:494, :515, :920 — `TestLoad_RepoFileOverridesGlobal`,
`TestLoad_GitOverridesRepoFile`, `TestLoad_FullPrecedenceMatrix`) redirect it to a `strings.Builder`
"so it doesn't pollute test output" and then assert on `cfg.Provider` etc. — they NEVER read the builder's
content. So even though those repo files lack `config_version` (→ the new "missing" advisory now also
writes to the captured builder), those tests still PASS (they ignore the builder). No test anywhere
asserts `noticeOut` is empty or exact-matches its content. ✓

## 3. EXACTLY 2 shipped assertions break under `Defaults() = 0` — both are bounded, trivial updates

Grepped every `ConfigVersion`/`config_version` reference in `internal/config/*_test.go`:

| location | current assertion | breaks? | fix |
|---|---|---|---|
| config_test.go:68-69 | `Defaults().ConfigVersion != CurrentConfigVersion` → expect 2 | YES | expect `0` ("Defaults leaves it unset") |
| file_test.go:594-595 (TestOverlay_V2Scalars b) | after overlay with zero-src, `dst.ConfigVersion != CurrentConfigVersion` → expect 2 | YES | expect `0` (Defaults pins 0; nil src doesn't clobber) |
| config_test.go:71-72 | `CurrentConfigVersion != 2` | no | — (const unchanged) |
| config_test.go:116 | `config_version` in the marshal-exclusion key list | no | — |
| file_test.go:491,509-510 | loads a file WITH `config_version = 2` → expect 2 | no | — (file sets it to 2 regardless of Defaults) |
| file_test.go:577-581 | overlay src `ConfigVersion:3` → expect 3 | no | — (src non-zero wins; Defaults value irrelevant) |

No reference outside `internal/config/`. So the `Defaults()` change costs exactly 2 one-line assertion
updates + their messages. The PRP lists them verbatim. The cmd template tests (config_test.go:137/178/184/
187) use `len(got)==len(template)` (tautology — got IS written from the template) and `Contains` checks, so
template additions are safe.

## 4. `configVersionNotice` is a pure function mirroring `repoProviderNotice`

`repoProviderNotice(cfg *Config) string` (file.go) is the established pattern: a PURE function returning
the notice text (or "") with NO I/O, called from the loader which does `fmt.Fprint(noticeOut, msg)`. Mirror
it: `configVersionNotice(fileLoaded bool, version int) string` — pure, takes the resolved facts (not cfg),
trivially unit-testable without constructing a `Config`. Returns "" when `!fileLoaded` or `version ==
CurrentConfigVersion`; otherwise the per-case advisory text. Lives in `load.go` (next to `Load`, its only
caller); `noticeOut` is package-level so `load.go` reaches it (load.go already imports "fmt").

## 5. Placement in `Load()` — end of the happy path, after all overlays

The advisory runs AFTER every overlay layer (so the highest-layer version wins, per the task) and only on a
SUCCESSFUL load (a loadEnv/loadFlags hard error returns before it — no point warning about a discarded
cfg). Concretely: right before `return &cfg, nil`, after the `Commits==1 → Single` normalization
(normalization doesn't touch ConfigVersion; ordering is immaterial). On any earlier `return nil, err`, the
advisory is skipped (correct — we're failing, not advising).

## 6. The three advisory messages (match the task wording + FR-B4)

The task gives the older-case text verbatim: *"stagecoach: config file uses schema version X; current is Y.
Run 'stagecoach config upgrade' or 'stagecoach config init --force'."* Adapt per case (all `\n`-terminated,
matching `repoProviderNotice`):

- **missing** (`version == 0`, fileLoaded): `stagecoach: config file has no config_version; current is 2.
  Run 'stagecoach config upgrade' or 'stagecoach config init --force'.\n` (no "version 0" — clearer).
- **older** (`0 < version < CurrentConfigVersion`): the task's text with X=version, Y=CurrentConfigVersion.
- **ahead** (`version > CurrentConfigVersion`): `stagecoach: config file uses schema version 3; this binary
  supports up to 2. Upgrade stagecoach, or run 'stagecoach config init --force' to regenerate.\n`

## 7. DOCS (Mode A) — update the `config init` template header

The `config init` template is `exampleConfigTemplate` in `internal/cmd/config.go` (the inert, fully-
commented config — the Mode-A user-facing config documentation). ADD a header block (commented `#` lines,
so the file stays inert and the cmd `Contains`/uncommented-header tests stay green) documenting:
`config_version` is top-level metadata (NOT a precedence layer, §16.1), `CurrentConfigVersion`, the
missing/older/ahead advisory behavior, and that `stagecoach config upgrade` / `config init --force` remedy
staleness. PLACE it in the header near the precedence section. NOTE: P1.M4.T2 will rewrite `config init`
to a POPULATED bootstrap (FR-B1) and retain this inert template behind `--template` (FR-B2); the
config_version doc note should be carried into P1.M4.T2's populated output too — flag it for that task.

## 8. Scope boundaries / no conflicts

- **Parallel P1.M3.T3.S1** (running now) creates `internal/config/role_defaults.go` (NEW) + edits
  `docs/providers.md`. It does NOT touch `config.go`, `config_test.go`, `file_test.go`, `load.go`,
  `load_test.go`, or `internal/cmd/config.go`. No overlap. ✓
- **Future P1.M4.T2** (populated config init) runs AFTER this; it will see the template's new
  config_version doc and should carry it into its populated output. No conflict (sequential).
- **Future P1.M4.T3** (first-run fallback) ensures a file always exists post-install; until then the
  `fileLoaded` guard (§1) keeps the no-file path quiet.
- This subtask does NOT implement `config upgrade` (P1.M4.T3), populated `config init` (P1.M4.T2), or
  auto-migration (FR-B4: advisory only — "there are no existing users to migrate"). It only WARNS.

## Sources
- `internal/config/config.go` — `CurrentConfigVersion = 2` const, `Config.ConfigVersion` field,
  `Defaults()` (pins it → this subtask changes that). READ-then-edit.
- `internal/config/file.go` — `fileConfig.ConfigVersion`, `materialize()` (non-zero copy), `overlay()`
  (non-zero merge), `noticeOut`/`SetNoticeOut`/`NoticeOut`, `repoProviderNotice` (the pure-function
  pattern to mirror). READ-then-edit(only Defaults-via-config.go).
- `internal/config/load.go` — `Load()` (where the advisory + fileLoaded guard go), `loadGitConfig` call
  (Layer 4 — no config_version). READ-then-edit.
- `internal/config/{config,file,load}_test.go` — the test conventions + the 2 breaking assertions.
- `internal/cmd/config.go` — `exampleConfigTemplate` (the DOCS target).
- PRD §9.17 FR-B4 (the advisory contract), §16.1 (config_version is metadata, not a precedence layer).
