# P1.M5.T4.S1 README — verified context & gaps

Source of truth: the IMPLEMENTED code (not just the PRD). Every command/snippet the README shows must
match what the binary actually does. Verified 2026-06-29 against the working tree.

## 1. README does NOT exist yet
`ls README*` → nothing. This subtask CREATES `README.md` at repo root. No conflict with any sibling.

## 2. The CLI surface (P1.M4.T1.S2 — Complete) — what to document VERBATIM

From `internal/cmd/root.go`, `providers.go`, `config.go`, `default_action.go`:

- **Default action**: `stagecoach` (no subcommand) → commit staged changes; auto-stages all if nothing
  is staged and `auto_stage_all` is on (default true). Prints to **stdout** the success report:
  `[<7-char-sha>] <subject>` then one `STATUS  path` line per changed file. Notices/diagnostics → stderr.
- **Global flags** (all persistent, `stagecoach --help` lists them):
  `--provider`, `--model`, `--config`, `--timeout`, `--verbose`/`-v`, `--no-color`,
  `--all`/`-a`, `--no-auto-stage`, `--dry-run`. Plus cobra-builtin `--version` and `--help`/`-h`.
- **Subcommands**:
  - `stagecoach providers list` → `NAME  DETECTED  DEFAULT` table (`✓`/`✗`, `(default)`).
  - `stagecoach providers show <name>` → merged manifest as TOML (exit 1 if unknown).
  - `stagecoach config init` → writes a fully-commented example config to the GLOBAL path; **refuses to
    overwrite** an existing file (exit 1). THIS TEMPLATE IS THE CONFIG REFERENCE (Mode-A docs).
  - `stagecoach config path` → prints the global config path.
- **Dry run**: `stagecoach --dry-run` → stdout is the message ONLY; stderr gets `(no commit created)`.
- **Version**: `var version = "dev"` in `cmd/stagecoach/main.go`; injected via `-X main.version`. So
  `stagecoach --version` works (prints `dev` for a local build).

## 3. Providers (6 built-in, P1.M2.T2) + auto-detect order

From `internal/provider/registry.go` line 15:
`preferredBuiltins = ["pi", "claude", "gemini", "opencode", "codex", "cursor"]`
→ with no config, the **first installed** one is the default (pi first). User-defined §12.8 providers
are NEVER auto-selected. `providers list` shows which is `(default)`.

## 4. Config model (P1.M1.T4) — precedence + locations

From `internal/cmd/config.go` example template + `config.GlobalConfigPath()`:
- **Global file** = `$XDG_CONFIG_HOME/stagecoach/config.toml` (default `~/.config/stagecoach/config.toml`).
- **Repo-local file** = `./.stagecoach.toml` (gitignored by default, §19).
- **Git-config keys**: `git config stagecoach.provider <name>`, `stagecoach.model`, `stagecoach.timeout`,
  `stagecoach.auto_stage_all`.
- **Env**: `STAGECOACH_PROVIDER`, `STAGECOACH_MODEL`, `STAGECOACH_TIMEOUT`, `STAGECOACH_CONFIG`,
  `STAGECOACH_VERBOSE`, `STAGECOACH_NO_COLOR` (also honors `NO_COLOR`).
- **Precedence** (high→low): CLI flags > STAGECOACH_* env > repo git-config (stagecoach.*) >
  repo `.stagecoach.toml` > global config file > provider `default_*` > built-in defaults.

## 5. Install paths (PRD §21.3 + `.goreleaser.yaml`)

| Path        | Command                                                            |
|-------------|-------------------------------------------------------------------|
| Homebrew    | `brew install dustin/tap/stagecoach`                              |
| Go install  | `go install github.com/dustin/stagecoach/cmd/stagecoach@latest`     |
| curl\|sh    | `curl -fsSL https://github.com/dustin/stagecoach/raw/main/install.sh \| bash` |
| Scoop       | `scoop install dustin/stagecoach`                                  |

goreleaser wires: `release.github.owner=dustin`; Homebrew tap repo `dustin/homebrew-tap`; Scoop bucket
`dustin/scoop-bucket`. go.mod module path = `github.com/dustin/stagecoach`.

## 6. GAPS / DECISIONS the README author MUST handle (do NOT silently paper over)

### GAP A — namespace mismatch (CRITICAL for install URLs)
- git remote is `git@github.com:dabstractor/stagecoach` (origin).
- BUT module path, `.goreleaser.yaml`, and §21.3 all use `github.com/dustin/stagecoach`.
- `.goreleaser.yaml` already flags this: "before the first REAL tag the repo must be reachable at
  github.com/dustin/stagecoach (or the namespace is reconciled repo-wide)."
- **README DECISION**: use `dustin/stagecoach` in ALL install URLs (Homebrew/Scoop/go-install/curl|sh),
  matching goreleaser + §21.3. Do NOT use `dabstractor`. This is what the released artifacts will use.

### GAP B — `install.sh` does not exist yet
- `ls install.sh` → none. The curl|sh one-liner points at
  `https://github.com/dustin/stagecoach/raw/main/install.sh`, which is a RELEASE-TIME artifact
  (goreleaser's `before.hooks` only runs `go mod tidy`; nothing writes install.sh).
- **README DECISION**: still document the curl|sh path per §21.3 (it is the intended public path and
  lands with the first release), but add a short note that the script is published at first release.
  Do NOT invent a different URL.

### GAP C — no LICENSE file
- `ls LICENSE*` → none. `.goreleaser.yaml` sets `license: MIT` but comments "ADJUST to the repo's
  actual license if different."
- **README DECISION**: do NOT assert a license the repo lacks. Omit the license badge/line, OR add an
  MIT badge only if a human first drops a `LICENSE` file (out of this subtask's scope — humans own that).
  Preferred: a one-line `<!-- TODO: add LICENSE file and badge -->` note so it isn't forgotten.

### GAP D — `docs/` directory does not exist yet (the §21.5 #8 "Full reference" link)
- No `docs/` dir; `find . -name '*.md'` outside plan/ returns only `PRD.md`.
- P1.M5.T5.S1 ("Review and update docs/ overview") is **Planned** — it will create `docs/`.
- **README DECISION**: the "Full CLI + config reference" section must NOT dead-link. Use the
  LIVING references that already work — `stagecoach --help` (CLI) and `stagecoach config init` (writes
  the full commented config = the canonical config reference) — as the PRIMARY reference, and add a
  link to `docs/` (relative, to be populated by P1.M5.T5) as the secondary. If `docs/` is empty at
  author time, the `--help`/`config init` commands fully satisfy the reader — no broken link.

## 7. Markdown tooling available → use as a validation gate
`markdownlint-cli2 v0.22.1 (markdownlint v0.40.0)` IS installed. Use it as the Level-1 gate.
(GitHub-flavored markdown; fenced code blocks render fine. `git remote` confirmable.)

## 8. Verbatim PRD blocks the README MUST reproduce EXACTLY (do not paraphrase)
- **Hero pitch** (§5): the blockquote beginning "Stagecoach writes your commit messages using the AI
  agent you already pay for." (full text in the work-item / PRD §5).
- **Stage-while-generating diagram** (§13.4): the two-pane ASCII block (Pane A lazygit/shell, Pane B
  shell) — reproduce character-for-character.
- **Install block** (§21.3): the four commented commands above (§5 table), verbatim.
- **lazygit binding** (§15.5): the `customCommands` YAML snippet with `key: '<c-a>'`, `output: 'none'`.

## 9. "Why not opencommit/aicommits?" — 3 sentences (§4.3)
Distill §4.3's moat into 3 sentences: (1) incumbents own the HTTP call → they normalize providers but
CANNOT reach a coding-plan subscription (not reachable over the public API). (2) Stagecoach inverts it:
it shells out to your installed CLI agent, giving up provider normalization in exchange for spending
the quota you already bought. (3) That trade-off — give up control of the model call to access your
existing plan — is the entire product; the provider manifest makes the "give up normalization" part
tolerable. Keep to ~3 sentences per the contract.

## 10. Anti-persona (§7.4) — "not for you if…"
Plain-language: a developer with no coding-agent CLI installed and no desire to get one. Stagecoach is
useless to them; opencommit is the right tool. Say so plainly to avoid disappointing installs.

## 11. Mode B note (the contract's DOCS bullet)
This subtask IS the changeset-level documentation for README.md — the final doc sweep (per §5 Mode B).
It runs LAST, after all implementing subtasks (CLI/providers/config/manifests/install), so the README
reflects ACCURATE, shipped behavior — not aspirational features.
