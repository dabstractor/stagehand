# P1.M2.T1.S1 — Design decisions (locked)

Researched **2026-07-02** by reading the actual source (not summaries):
`internal/config/{config,file,load,git}.go`, `internal/cmd/root.go`,
`internal/config/{load,git}_test.go`, `docs/{cli,configuration}.md`,
and the sibling PRP `plan/005_c38aa48290f0/P1M1T1S1/PRP.md` (the `cfg.Exclude` plumbing —
the closest precedent, landed). Cross-checked against
`architecture/system_context.md` §3 and the PRD §9.19 / §9.8 / §15.2 selections.

## 0. The one-line summary

`Format` and `Locale` are TWO NEW SCALAR string fields on `Config` that flow through the
STANDARD 5-layer precedence (file → git-config → env → flag), validated exactly like every
other scalar — **NOT** the special union merge that `Exclude` needed. The only novel element
is the post-resolution enum validation of `Format`.

## 1. Format/locale are SCALARS — copy Provider/Model, NOT Exclude

| Aspect | Exclude (P1.M1.T1.S1) | **Format / Locale (THIS task)** |
|---|---|---|
| Field type | `[]string` (list) | `string` (scalar) |
| Merge in `overlay()` | **UNION** (append) — the one exception | **REPLACE** (non-zero wins) — the rule |
| File source | `[generation].exclude` | `[generation].format` / `[generation].locale` |
| Git-config key | **NONE** (deliberate) | `stagecoach.format` / `stagecoach.locale` |
| Env var | **NONE** (deliberate) | `STAGECOACH_FORMAT` / `STAGECOACH_LOCALE` |
| Flag | `--exclude`/`-x` (repeatable StringArray) | `--format` / `--locale` (single String) |
| Precedence sources | 2 (file + flag) | **4** (file + git + env + flag) |
| Validation | none (raw globs) | **format**: enum hard-error; **locale**: none (verbatim) |

**Implication**: the exact merge machinery is `Provider`/`Model`/`Reasoning`, which ALREADY do
file→git→env→flag scalar replace. Copy those, line-for-line. Do **not** copy `Exclude`'s union
branch or its "no env / no git-config" omissions — those were specific to the glob-quoting trap
(FR-X1) and do **not** apply to free-form strings.

## 2. The `Format` default is "auto" (non-empty) — the one subtle gotcha

Every other `[generation]` scalar defaults to the zero value (`""`, `0`, `nil`) and relies on
the non-zero overlay to leave absent fields untouched. `Format` is different: its shipped
default is **`"auto"`** (FR-F1), a non-empty string. This is safe but has two consequences the
implementer must understand:

- `Defaults()` sets `Format: "auto"` explicitly (do NOT leave it `""`). Without this, an
  unconfigured repo would resolve `Format == ""`, which `validateFormat` would reject — a
  boot-loop. `Locale` defaults to `""` (empty = no instruction) and needs no explicit default
  (zero value is correct), but we set it explicitly for documentation parity.
- A file/env/git/flag setting `format = "auto"` works fine ("auto" is non-empty → overlay
  copies it). The ONLY un-settable value is the empty string `""`, which is not a valid mode
  anyway. This is identical to how `Provider`/`Model` cannot be reset to `""` via a file —
  documented and accepted. No special-casing needed.

`loadFlags`/`loadEnv`/the flag registration ALL keep the flag/env **default as `""`** (zero),
so `fs.Changed`/`os.LookupEnv` presence semantics work unchanged: an unset flag is skipped,
and `cfg.Format` keeps the `"auto"` from `Defaults()`. This mirrors `Provider`/`Model`
(flag default `""`, Config default `""`) — only the Config-side default differs (auto vs "").

## 3. Validation is a SINGLE post-resolution check, NOT per-layer

**Decision**: `validateFormat(cfg.Format)` runs ONCE at the tail of `Load()`, after every
layer resolves and after the v3 in-memory migration, immediately before `return &cfg, nil`.

**Why not per-layer**: a bad value set at a LOW layer (e.g. `stagecoach.format = "emoji"` in
git-config) that is overridden by a HIGHER layer (`--format conventional`) is NOT an error —
the flag wins and the bad git-config value is moot. Per-layer validation would false-positive.
The contract is "validate the value that will actually be USED", which is the resolved value.
This matches how `configVersionNotice` and the `Commits==1 ⇒ Single` normalization both
operate on the final `cfg`.

**Where exactly** (load.go `Load()`, after the migration `if/else if` block, before return):
```go
if err := validateFormat(cfg.Format); err != nil {
    return nil, fmt.Errorf("format: %w", err)   // → PersistentPreRunE wraps as "config: format: ..."
}
```
The `fmt.Errorf("format: %w", err)` wrapper names the field (consistent with Load's other
`fmt.Errorf("global config: %w", …)` / `fmt.Errorf("env config: %w", …)` layer-naming).
`PersistentPreRunE` (root.go) then wraps the whole thing as `config: %w` and maps to
`exitcode.Error` (exit 1) — the correct "hard configuration error" surface (FR-F1).

**Error shape** (validateFormat): `invalid format "emoji" (valid: auto, conventional, gitmoji, plain)`
— names the offending value AND the valid set, exactly as the contract requires. Quote the
value (it may contain whitespace/special chars) and `strings.Join` the set with `", "`.

**Locale is deliberately NOT validated** (FR-F6: free-form name or BCP-47 tag, passed
verbatim, no i18n tables). A `validateLocale` call must NOT be added — add a code comment
stating the deliberate omission next to the `validateFormat` call.

## 4. `validFormats` is unexported, load.go-local

```go
var validFormats = []string{"auto", "conventional", "gitmoji", "plain"}
```
Placed in load.go next to `validateFormat` (validation-only; S3's prompt scaffolds are built
from static strings, not this slice). No export needed: help text and docs are static. If a
future task wants the list for dynamic help, export then — YAGNI now.

## 5. Git-config keys are single-word → no camelCase trap

`loadGitConfig` (git.go) uses CAMELCASE for multi-word keys (`stagecoach.maxDiffBytes`)
because `git config` rejects underscores (`invalid key`). `format` and `locale` are
single-word keys, identical in shape to the existing `stagecoach.provider` / `stagecoach.model` /
`stagecoach.output` (plain `gitConfigGet`, no `parseInt`/`gitConfigBool`). Copy the
`stagecoach.output` block exactly:
```go
if v, found, err := gitConfigGet(repoDir, "stagecoach.format"); err != nil {
    return nil, err
} else if found {
    c.Format = v
}
```
No validation here (raw copy; the final resolved value is validated in `Load()`).

## 6. Flag registration: single `StringVar`, zero default

`--format` and `--locale` are plain `pf.StringVar(&flagFormat, "format", "", "<help>")` in
root.go `init()` — NO shorthand (PRD §15.2 gives none for these two), single value (NOT
StringArray — they are not repeatable). Help text must disambiguate `--format` (message
shaping) from `config init --template` (system_context §6 watch-out #4) and name the 4 modes +
the env/git-config/`[generation]` sources. The package var (`flagFormat`/`flagLocale`) is read
ONLY via `fs.Changed`/`fs.GetString` in `loadFlags` — never directly (same discipline as
`flagProvider`/`flagModel`); the `&flagFormat` address is its use (satisfies the `unused` linter).

## 7. Placement is disjoint from the in-flight P1.M1.T2.S1 and the landed P1.M1.T1.S1

- **P1.M1.T2.S1** (implementing NOW, in parallel) touches `internal/git/*` (diff methods),
  `generate.Deps`/`decompose.Deps`, `pkg/stagecoach`, `cmd/stubagent`, `docs/how-it-works.md`.
  **THIS task touches NONE of those files** → zero merge conflict with the in-flight item.
- **P1.M1.T1.S1** (landed) touched the SAME files as this task (`config.go`, `file.go`,
  `load.go`, `root.go`, `load_test.go`, `docs/cli.md`, `docs/configuration.md`) but DIFFERENT
  regions: the `Exclude` field/struct-member/overlay-branch/loadFlags-append/flag-var/flag-row,
  vs THIS task's `Format`/`Locale` field/struct-members/overlay-branches/loadEnv+loadFlags
  blocks/git-config-block/flag-vars/flag-rows. They are sibling fields in the same struct; the
  edits land on distinct lines. Place `Format`/`Locale` in the `[generation]` SCALAR group
  (near `Output`/`MaxCommits`), NOT interleaved with the `Exclude` list handling.

## 8. Scope fence — what is OUT of scope (S2/S3 own it)

- **S2 (P1.M2.T1.S2)**: the compiled-in gitmoji reference table. Do NOT add it.
- **S3 (P1.M2.T1.S3)**: the format-mode prompt scaffolds (conventional/gitmoji/plain), the
  locale one-line append, and applying them at every message-production site. Do NOT touch
  `internal/prompt/*`, `generate.go`, `decompose/message.go`, `pkg/stagecoach` prompt calls.
- **bootstrap.go / `config init`**: the task contract is OUTPUT = `cfg.Format`/`cfg.Locale`
  plumbing + DOCS only. FR-B1 does not list format/locale in the bootstrap. Do NOT modify
  `GenerateBootstrapConfig` or `exampleConfigTemplate` (the stale "= 2" prose is P1.M7's job).
- The downstream consumer is S3 via `cfg.Format`/`cfg.Locale` — these fields are the deliverable.

## 9. Docs (Mode A) — exact placement

- **docs/cli.md**: (a) two rows in the **Global flags** table (after `--exclude`/`-x`, before
  the per-role block); (b) two rows in the **Flag ↔ env ↔ git-config map** table.
- **docs/configuration.md**: (a) `[generation].format` / `[generation].locale` in the File
  format `[generation]` area + the commented example block; (b) two rows in **Built-in
  defaults** (`format`→`auto`, `locale`→`""`); (c) two rows in **Environment variables**
  (`STAGECOACH_FORMAT` / `STAGECOACH_LOCALE`); (d) two rows in **Git-config keys**
  (`stagecoach.format` / `state.locale`). State the hard-error-on-unknown-format rule and the
  locale free-form/no-validation rule verbatim (FR-F1 / FR-F6).

## 10. Why no external-research subagent was spawned

This is a pure plumbing task over an in-codebase pattern with three live sibling scalars
(`Provider`/`Model`/`Reasoning`) and one live sibling list (`Exclude`) to copy from, plus an
enum-validation precedent (`configVersionNotice`). The pflag and go-toml surfaces are already
used identically upstream. External docs would add no signal; the codebase is the authoritative
reference and was read directly (more accurate than a subagent summary).
