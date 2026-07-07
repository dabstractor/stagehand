# Capability → docs map for the README + index coherence sweep

Surveyed 2026-07-03 against the shipped `docs/` tree and README.md. This is the authoritative map
the README rewrite cites: every one of the six v2.1 capabilities is ALREADY documented in a `docs/`
Mode-A file — the README + docs/README.md job is DISCOVERY (surface + link), not authoring.

## The six v2.1 capabilities (M1–M6) and where each already lives

| # | Capability (milestone) | User surface | Documented at | README's job |
|---|---|---|---|---|
| 1 | Payload exclusions (M1, §9.18) | `.stagecoachignore`, `--exclude`/`-x`, `[generation].exclude` | `docs/configuration.md` §"Exclusion globs (`[generation].exclude`)" + "`.stagecoachignore`"; `docs/how-it-works.md` §"Payload exclusions (.stagecoachignore)" | capabilities list; link configuration.md |
| 2 | Message shaping (M2, §9.19) | `--format`/`-locale`/`-context`/`-template` (auto\|conventional\|gitmoji\|plain) | `docs/how-it-works.md` §"Format modes and locale"; `docs/configuration.md` ([generation] keys) | capabilities list; link how-it-works.md |
| 3 | Git hook mode (M3, §9.20) | `stagecoach hook install\|uninstall\|status`, `hook exec` | `docs/cli.md` §`hook *`; `docs/how-it-works.md` §"Hook mode vs the snapshot-based flow" / §"Trade-off inversion (FR-H7)" / §"When to use which" | capabilities list; FAQ entry (FR-H7 trade-off); link how-it-works.md |
| 4 | Tool integrations (M4, §9.21) | `stagecoach integrate install git-alias\|lazygit` (+ `list`/`remove`) | `docs/cli.md` §`integrate *` (incl. `git-alias`, `lazygit`, no-mangle protocol) | REPLACE the manual lazygit YAML snippet; keep YAML as collapsible alternative; mention gitui blocked (FUTURE_SPEC.md) |
| 5 | `--edit` / `--push` (M5, §9.22) | `--edit`, `--push`/`STAGECOACH_PUSH` | `docs/cli.md` (global flags table); `docs/how-it-works.md` "Stage-while-editing (FR-E2)"; `docs/configuration.md` (STAGECOACH_PUSH) | capabilities list (brief); link cli.md |
| 6 | Discovery (M6, §9.23) | `stagecoach models [<provider>]`, `config init --interactive` | `docs/cli.md` §`models [<provider>]` + §`config init` (`--interactive` per P1.M6.T2.S1) | capabilities list; "Configure your agent" section links both |

## Verified doc anchors (GitHub slug algorithm: lowercase, spaces→`-`, strip most punctuation)

These are the anchors the README/index will link to. Re-verify each renders correctly (the PRP's
Level-2 gate does this programmatically):

- `docs/how-it-works.md#trade-off-inversion-fr-h7` ← from "### Trade-off inversion (FR-H7)"
- `docs/how-it-works.md#hook-mode-vs-the-snapshot-based-flow`
- `docs/how-it-works.md#when-to-use-which`
- `docs/how-it-works.md#format-modes-and-locale`
- `docs/how-it-works.md#payload-exclusions-stagecoachignore` ← "### Payload exclusions (.stagecoachignore)" — the `(` strips, `.` strips
- `docs/how-it-works.md#multi-commit-decomposition`
- `docs/configuration.md#exclusion-globs-generationexclude` ← "### Exclusion globs (`[generation].exclude`)" — backticks strip
- `docs/configuration.md#stagecoachignore` ← "### `.stagecoachignore`" — backtick strips
- `docs/configuration.md#bootstrap-config-init` ← "### Bootstrap (`config init`)"
- `docs/cli.md#hook-install` ← "### `hook install`" — backtick strips
- `docs/cli.md#integrate-install-target` ← "### `integrate install <target>…`"
- `docs/cli.md#integrate-install-lazygit`-ish → NOT a heading; the lazygit target is `#### lazygit target` → `#lazygit-target`
- `docs/cli.md#models-provider` ← "### `models [<provider>]`" — brackets strip
- `docs/cli.md#config-init` ← "### `config init`"

## Shipped behavior snapshots (word the README against THESE, not the plan)

### lazygit customCommands (the manual-YAML alternative must match this — docs/cli.md ~L287)
```yaml
customCommands:
  - key: '<c-a>'                       # stagecoach-integration
    context: 'files'
    command: 'stagecoach'
    loadingText: 'Generating commit message…'
    output: 'none'
    description: 'stagecoach: AI commit'
```
Default key `<c-a>`; install cmd `stagecoach integrate install lazygit` (or `--key '<c-s>'`); remove
targets the `# stagecoach-integration` MARKER (not the key). Manual install = paste this YAML block.

### FR-H7 trade-off (docs/how-it-works.md §228–250) — the FAQ entry must match
- Snapshot flow (default `stagecoach`): atomic, stage-while-generating, rescue protocol, BUT bypasses
  pre-commit hooks (husky/lint-staged/`.pre-commit-config.yaml` do NOT run — built via plumbing).
- Hook mode (`stagecoach hook install` + `git commit`): pre-commit hooks honored, never-block contract
  (failure → exit 0, empty editor), BUT no snapshot/atomicity, latency inside the commit.
- The two COMPOSE: hook for `git commit`, flagship for the atomic path.

### Detection order (FR-D1) — README "Configure your agent" cites this
pi, opencode, cursor, agy, gemini, qwen-code, codex, claude. 8 built-ins. Auto-detect first on $PATH.

## FUTURE_SPEC.md — referenced, NEVER authored/modified
Repo-root file. README + docs/README.md must LINK to it wherever deferred/rejected ideas arise:
- gitui integration target (blocked upstream) → FUTURE_SPEC.md §1.2.
- "Why not PR generation / VS Code extension / GitHub Action / API keys?" → FUTURE_SPEC.md (§2.1
  GitHub Action blocked; §3 rejected table: API-key, PR generation, generate-N-and-pick, chunking,
  clipboard, self-update, config describe, locale i18n trees).
- The "Why not opencommit/aicommits?" README section is the natural discovery point.

## Scope fences (do NOT cross)
- **P1.M7.T1.S2 (stale-reference sweep)** owns the known-stale-string fix — most importantly the
  `exampleConfigTemplate` header string `This binary supports config_version = 2.` at
  `internal/cmd/config.go:511` (CODE, not docs). S1 touches ONLY `README.md` and `docs/README.md`.
  If S1 spots a stale string IN those two files, fix it (it is a docs file); do not touch code.
- **P1.M6.T2.S1 (interactive wizard)** is parallel/in-progress. It documents `--interactive` in
  `docs/cli.md` + `docs/configuration.md` itself. S1 only needs to SURFACE `config init --interactive`
  in the README "Configure your agent" section + index — assume its docs landing exists.

## Validation tooling available
- `.markdownlint.json` present: `default: true` (all rules on), MD013 (line length) / MD033 (inline
  HTML) / MD060 disabled. `<details>` and GitHub alert syntax (`> [!NOTE]`) are allowed & already used.
- No Makefile target for markdown lint (only `golangci-lint` for Go). Use `npx --yes markdownlint-cli2`
  (node+npx ARE on PATH) as best-effort; primary gate = the link/anchor + coverage audit script.
