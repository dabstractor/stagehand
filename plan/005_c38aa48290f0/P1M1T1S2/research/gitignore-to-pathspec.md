# Research: gitignore-style glob → `:(exclude,glob)` pathspec translation

Source of truth: `plan/005_c38aa48290f0/architecture/external_deps.md §4` (VERIFIED 2026-07-02
against gitglossary) and PRD §9.18 FR-X1..FR-X5, §16.1.

## The core problem (external_deps.md §4)

- Pathspec exclude spellings are equivalent: `:(exclude)pattern` == `:!pattern` == `:^pattern`.
- **Standalone excludes work**: with no non-exclude pathspec, git applies the exclusion to the whole
  result set. `git diff -- ':(exclude,glob)vendor/**'` is valid alone. The existing FR3 `defaultExcludes`
  path in `internal/git/git.go` already relies on this.
- **Default pathspec matching is `fnmatch` WITHOUT `FNM_PATHNAME`** — a `*` in a bare pathspec CAN cross
  `/`. That is NOT gitignore semantics. To get gitignore semantics (`*` stops at `/`, `**` spans
  components) you MUST add the `glob` magic word: `:(exclude,glob)<pattern>`.
- **Pathspecs have NO negation / re-include.** This is why FR-X2 forbids `!` lines: they are skipped
  with a `--verbose` warning, never an error.

## Translation rules (the contract for `TranslatePattern`)

A `.stagehandignore` / `[generation].exclude` / `--exclude` entry is a **gitignore-style glob relative
to the repo root** (FR-X2). Map it to a single `:(exclude,glob)<core>` pathspec:

Given the raw pattern `p` (loader already trimmed it and dropped blank/`#`/`!` lines):

1. `anchored` = `p` starts with `/`. If so, strip the leading `/`. (gitignore: a leading separator
   anchors to the root; under `:(glob)` a pattern is already root-relative, so stripping is correct.)
2. `dirOnly` = `p` ends with `/`. If so, strip the trailing `/` and remember it.
3. `hasInternalSlash` = the remaining `p` still contains a `/` (a separator in the *middle*).
   gitignore anchors a pattern to the root iff it has a leading OR middle separator; a *trailing*-only
   separator does NOT anchor.
4. Build `core`:
   - if `dirOnly`: `core = p + "/**"`  (the note's `dir/ → dir/**` mapping — exclude the directory's
     contents).
   - else `core = p`.
   - if NOT `anchored` AND NOT `hasInternalSlash`: prepend `**/` so the pattern matches at **any depth**
     (gitignore: a pattern with no leading/middle separator matches in every directory; git's `**/foo`
     "matches foo anywhere, including the root").
5. Return `":(exclude,glob)" + core`.

### Worked golden-table rows (author these as the unit test)

| input           | anchored | dirOnly | internalSlash | output                          | meaning                              |
|-----------------|----------|---------|---------------|---------------------------------|--------------------------------------|
| `*.lock`        | no       | no      | no            | `:(exclude,glob)**/*.lock`      | any-depth `.lock` files              |
| `/*.lock`       | yes      | no      | no            | `:(exclude,glob)*.lock`         | top-level `.lock` only               |
| `vendor/`       | no       | yes     | no            | `:(exclude,glob)**/vendor/**`   | any-depth `vendor/` dir contents     |
| `/build/`       | yes      | yes     | no            | `:(exclude,glob)build/**`       | root `build/` contents only          |
| `docs/*.md`     | no       | no      | yes           | `:(exclude,glob)docs/*.md`      | root-anchored (middle slash)         |
| `node_modules`  | no       | no      | no            | `:(exclude,glob)**/node_modules`| any-depth entry named node_modules   |
| `a/**/b.go`     | no       | no      | yes           | `:(exclude,glob)a/**/b.go`      | `**` passes through verbatim         |
| `**/foo.txt`    | no       | no      | yes           | `:(exclude,glob)**/foo.txt`     | leading `**/` already any-depth      |
| `src/gen*.ts`   | no       | no      | yes           | `:(exclude,glob)src/gen*.ts`    | embedded `*`, root-anchored          |

## KNOWN NUANCE to verify against real git (do NOT skip)

git pathspec `glob` magic matches the **full path**. A pathspec `**/node_modules` matches the path
`node_modules` but a naive reading suggests it may NOT match `node_modules/x.js`. Whether git recurses
into a directory matched by a glob pathspec is exactly the behavior the integration test must pin down.
`dir/` inputs sidestep this by emitting `dir/**` (contents). For **bareword directory names without a
trailing slash** (`node_modules`), if the integration test shows contents are not excluded, the fix is
to ALSO emit the `/**` contents form — but confirm empirically first; do not guess. The unit golden
table encodes the string mapping; the git integration test encodes the *semantics*.

## No-negation rule (FR-X2)

`!`-prefixed lines are **skipped** in the loader (never translated), each producing one
`--verbose`-visible warning (`ui.Verbose`). Never an error, never abort. Missing `.stagehandignore` =
no-op (nil, nil).

## Payload-only guarantee (FR-X5) — docs duty

Exclusion affects ONLY the agent payload, never the commit: "excluded from what the agent sees, still
committed." `docs/configuration.md` must state this in the new `.stagehandignore` section.
