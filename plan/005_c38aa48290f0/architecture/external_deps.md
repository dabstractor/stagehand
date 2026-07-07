# External dependencies & integration surfaces (v2.1)

Research date: **2026-07-02**. Verification level noted per item. This document discharges
PRD Appendix E open questions **14–17** (lazygit schema, hook portability, gitmoji currency,
`list_models_command`) to the extent possible pre-implementation; residual in-task checks are flagged.

## 1. lazygit `customCommands` schema (gates FR-I5) — VERIFIED (web + local lazygit v0.62.2)

- Current field is **`output`** with values `none` | `terminal` | `log` | `logWithPty` | `popup`.
  The older `subprocess` / `showOutput` fields are **gone** (deprecated ~v0.42, mid-2024). Use `output: 'none'` (US8).
- **`loadingText` is valid** ("Text to display while waiting for command to finish").
- Full per-entry field set: `key`, `command`, `context`, `prompts`, `loadingText`, `description`, `output`, `outputTitle`, `after`, `commandMenu`.
- Files-panel context value is exactly **`files`**.
- **`lazygit --print-config-dir`** (short `-cd`) exists; verified locally → `/home/dustin/.config/lazygit`.
  There is no `--config-dir` flag. Default config paths: Linux `~/.config/lazygit/config.yml` (XDG),
  macOS `~/Library/Application Support/lazygit/config.yml`, Windows `%LOCALAPPDATA%\lazygit\config.yml`
  (fallback `%APPDATA%\lazygit\config.yml`).
- Sources: lazygit `docs/Custom_Command_Keybindings.md`, `docs/Config.md` (github.com/jesseduffield/lazygit).
- Record in code: verified 2026-07-02 against lazygit v0.62.2 (FR-D5 discipline).

## 2. Comment-preserving YAML in Go (gates FR-I3/FR-I5) — VERIFIED (empirical, yaml.v3 v3.0.1)

- `yaml.Node` carries `HeadComment` / `LineComment` / `FootComment` + `Style`/`Anchor`/`Alias`.
- **Byte-identity outside the edited node is NOT guaranteed** — yaml.v3 re-encodes the whole document.
  Empirical zero-edit round-trip of a lazygit-style config still (a) dropped a blank line between sections,
  (b) normalized inline-comment spacing. Quote style and comment text/placement were preserved.
- Default encoder indent is **4**; call `enc.SetIndent(2)` to match conventional configs.
- Surgical insert works: unmarshal the new entry into its own `yaml.Node`, append `entry.Content[0]`
  to the `customCommands` sequence node's `Content`; comments elsewhere survive.
- go-yaml is archived (2025) — no upstream fixes coming; design around it.
- **Consequence for FR-I3/FR-I5:** the PRD's "preserved outside the edited node" must be satisfied via the
  protocol, not the library: parse-first refusal → node edit → `SetIndent(2)` encode → re-parse validate →
  timestamped backup → atomic write (temp+rename) → unified-diff preview so any incidental normalization is
  visible and confirmed by the user before writing. This IS the no-mangle guarantee.
- Source: pkg.go.dev/gopkg.in/yaml.v3#Node.

## 3. `prepare-commit-msg` hook semantics (gates FR-H1/FR-H4) — VERIFIED (git-scm.com + git 2.54.0)

- Args: (1) message-file path, (2) source ∈ `message` (`-m`/`-F`) | `template` (`-t`/`commit.template`) |
  `merge` | `squash` | `commit` (`-c`/`-C`/`--amend`), (3) SHA only when source=`commit`.
- **Plain `git commit` invokes the hook with ONLY arg 1 — source is absent.** FR-H4's "fill only the empty
  case" gate is therefore: generate iff argc==1 (or source empty), no-op-exit-0 for every named source.
- Non-zero exit **aborts the commit** and the hook is **NOT suppressed by `--no-verify`** — this is why
  FR-H5's never-block (exit 0 on any failure) contract is load-bearing.
- **`git rev-parse --git-path hooks` verified empirically**: honors `core.hooksPath`; from a subdirectory
  returns a RELATIVE path (resolve against cwd before use); from a linked worktree returns the common dir's
  hooks path (correct). This is the right install-location command (FR-H1).
- Residual in-task check (Appendix E #15): script runs under git-for-windows sh — keep the script strict POSIX.

## 4. git pathspec exclusion (gates FR-X2/FR-X3) — VERIFIED (gitglossary)

- Spellings: `:(exclude)pattern` == `:!pattern` (== `:^pattern`). Always shell-quote when documenting.
- **Standalone excludes work**: "When there is no non-exclude pathspec, the exclusion is applied to the
  result set as if invoked without any pathspec." `git diff -- ':(exclude)vendor'` is valid alone; the
  existing FR3 code path already relies on this.
- Wildcards: default pathspec matching is fnmatch WITHOUT FNM_PATHNAME (`*` may cross `/`). For
  gitignore-style semantics (`**`, `*` not crossing `/`) use the `:(glob)` magic — combine as
  `:(exclude,glob)<pattern>`. `.stagecoachignore` patterns are "gitignore-style globs relative to repo root"
  (FR-X2) → translate to `:(exclude,glob)` (with a leading-`/` anchor mapping and `dir/` → `dir/**`).
- **No negation/re-include exists in pathspecs** — confirms FR-X2's rule: `!` lines are skipped with a
  `--verbose` warning, never an error.

## 5. gitmoji canonical list (gates FR-F3, Appendix E #16) — VERIFIED (web, 2026-07-02)

- Authoritative source: carloscuesta/gitmoji, `packages/gitmojis/src/gitmojis.json` (rendered by gitmoji.dev).
  Raw: https://raw.githubusercontent.com/carloscuesta/gitmoji/master/packages/gitmojis/src/gitmojis.json
- Current count: **75 entries**. Stable fields: `emoji`, `entity`, `code` (`:art:`), `description`, `name`, `semver`.
- Compile `emoji` + `description` (optionally `name`) into the binary as a Go constant table; record
  "verified 2026-07-02, 75 entries" in the source comment (FR-D5 discipline). No network fetch, ever (FR-F3).

## 6. Editor resolution (gates FR-E1) — VERIFIED (git-var docs)

- Git's own order: `$GIT_EDITOR` → `core.editor` → `$VISUAL` → `$EDITOR` → `vi`. The value is
  shell-interpreted (may contain args/quotes) → invoke via `sh -c '<editor> "$@"' -- <file>`, not bare exec.
- **Preferred implementation: shell out to `git var GIT_EDITOR`** — it performs the exact resolution
  (including repo-local `core.editor` and the dumb-terminal edge case). The PRD's FR-E1 chain omits
  `core.editor`; using `git var GIT_EDITOR` supersets it faithfully and is the recommended reading of FR-E1.

## 7. git alias mechanics (gates FR-I4) — VERIFIED (web + local)

- `git config --global alias.stagecoach '!stagecoach'`: `!` = shell command, executed from repo **toplevel**,
  `GIT_PREFIX` set to invocation subdir, extra args appended.
- Read-back: `git config --global --get alias.stagecoach` → prints `!stagecoach` (the `!` is part of the stored
  value — strip when comparing "is it ours"); exit 1/empty when unset. Use `--get` (portable), not the 2.46+
  `config get` subcommand form.

## 8. `git push` no-upstream failure (gates FR-P2) — VERIFIED (empirical, git 2.54.0)

- Exit code **128**, stderr `fatal: The current branch X has no upstream branch.` + the `--set-upstream` hint.
- Caveats: with `push.autoSetupRemote=true` (Dustin's global config HAS it) the push silently succeeds —
  tests MUST run with `GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null`. Match stable substrings
  (`has no upstream branch`, `--set-upstream`), not full text. stagecoach streams git's stderr verbatim and
  never auto-sets upstream (FR-P2).

## 9. `list_models_command` availability (gates FR-L1/L2, Appendix E #17) — PARTIAL

- Known-good case: **opencode** exposes `opencode models` (PRD-cited). Other providers must be checked
  against their live `--help` at implementation time; populate the manifest field ONLY where verified,
  everyone else falls back to the curated FR-D4 tier table annotated with its verification date.
