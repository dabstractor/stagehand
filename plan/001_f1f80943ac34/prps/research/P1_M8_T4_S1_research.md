# P1.M8.T4.S1 — README.md §21.5 marketing surface — RESEARCH NOTES

## Task
Write the repo-root `README.md` per PRD §21.5 (10 sections). OUTPUT = Mode B
whole-feature doc. Deps: P1.M7.T2.S1 (CLI/providers UX), P1.M7.T3.S1,
P1.M8.T2.S1 (install paths). Core rule: **every command/example must actually
run against the built binary.**

## Verified facts (run against ./bin/stagehand, `make build` OK)
- Binary: `./bin/stagehand` (make output); a built `./stagehand` also exists at root.
- `./bin/stagehand --version` → `stagehand version 5e2fd64` (ldflags `-X main.version`).
- `./bin/stagehand --help` → cobra usage; default action = generate+commit; subcommands: `providers`, `config`, `completion`, `help`.
- `./bin/stagehand providers list` → table (claude/codex/cursor/gemini/opencode/pi) + `default provider: pi (model: ...)`.
- `./bin/stagehand providers show <name>` → fully-resolved manifest as TOML.
- `./bin/stagehand config path` → `~/.config/stagehand/config.toml`.
- `./bin/stagehand config init [--force]` → writes commented example config; offers to gitignore `./.stagehand.toml`.
- git-config keys (internal/config/file.go `gitConfigKeys`): `stagehand.provider`, `stagehand.model`, `stagehand.timeout`, `stagehand.autoStageAll`, `stagehand.verbose`, `stagehand.noColor`, `stagehand.maxDiffBytes`, `stagehand.maxMdLines`, `stagehand.maxDuplicateRetries`, `stagehand.subjectTargetChars`, `stagehand.output`, `stagehand.stripCodeFence`. (No provider-table keys in git-config — use `[provider.<name>]` in TOML.)

## Install paths — which curl|sh URL actually works?
- `.goreleaser.yaml` does NOT list `install.sh` in `release.extra_files`.
  => `releases/latest/download/install.sh` would 404.
  => The ONLY guaranteed-working curl|sh URL is the PRD §21.3 verbatim form:
     `curl -fsSL https://github.com/dustin/stagehand/raw/main/install.sh | bash`
     (install.sh IS committed at repo root.)
- `install.sh`'s own header comment suggests a `releases/latest/download/...` form,
  but that path is NOT wired into goreleaser. USE raw/main in the README.
- 4 install paths (PRD §21.3):
  - `brew install dustin/tap/stagehand`
  - `go install github.com/dustin/stagehand/cmd/stagehand@latest`
  - `curl -fsSL https://github.com/dustin/stagehand/raw/main/install.sh | bash`
  - `scoop install dustin/stagehand`

## Repo identity
- go.mod module: `github.com/dustin/stagehand` (use `dustin` everywhere, NOT local git remote `dabstractor`).
- tap: `dustin/homebrew-tap`; scoop bucket: `dustin/scoop-bucket`; AUR: `stagehand-bin`.
- `.goreleaser.yaml archives.files: [LICENSE*, README*]` → README.md must exist or release warns "no files matched". (No LICENSE file exists yet — flag, do not block.)

## Docs to link to (exist now, anchors verified)
- `docs/CONFIGURATION.md` — §1 precedence, §2 env vars, §3 git-config keys, §4 .stagehand.toml, §8 CLI flags, §9 exit codes.
- `docs/PROVIDERS.md` — `providers list`, `providers show`, field-merge, built-in providers.
README §21.5 item 8 = "Full CLI + config reference (link to docs)" → link these two.

## PRD source material for each §21.5 section
1. Hero one-sentence pitch → PRD §5 verbatim candidate (the blockquote).
2. 30-sec demo → asciinema/gif PLACEHOLDER (no asset exists; use a fenced placeholder + TODO).
3. "Why not opencommit/aicommits?" → §2.2 (coding-plan framing) + §4.3 (structural moat), 3 sentences.
4. Install → §21.3 four paths (above).
5. Quick start → one `stagehand` invocation (§15.5 line 1: bare `stagehand`).
6. Configure your agent → `stagehand providers list` → `git config stagehand.provider <name>`.
7. Snapshot workflow → §13.4 ASCII diagram (verbatim — it IS the "stage while it thinks" payoff).
8. CLI+config reference → link docs/CONFIGURATION.md + docs/PROVIDERS.md.
9. Adding a new agent → §12.8 `[provider.<name>]` TOML block (contributor hook).
10. FAQ / "Stagehand is not for you if…" → §7.4 anti-persona; add v2 multi-commit (§10.3) +
    permanent non-goals (§6.3: never an API-key tool / never a code reviewer).

## §5 hero pitch (verbatim candidate, PRD)
> **Stagehand writes your commit messages using the AI agent you already pay for.**
> No API key. No per-token billing. It shells out to Claude Code, Codex, Gemini CLI,
> pi, opencode, or Cursor — whatever you already have installed — and spends your
> existing coding-plan quota instead. Stage while it thinks; it commits only what was
> staged when it started, atomically, and can never corrupt your repo.

## §13.4 diagram (verbatim, the payoff visual) — reproduce in README.
```
Pane A (lazygit / shell)        Pane B (shell)
─────────────────────────       ───────────────────────
git add feature/login.js
stagehand                     ┐
  ↳ snapshotting…             │  (user is free to work here)
  ↳ generating with pi…       │  git add docs/login.md
  ↳ (10s pass)                │  git add tests/login.test.js
  ↳ created abc1234           │  (these stay staged — NOT in abc1234)
                              ┘
                                stagehand        # next run commits these
```

## §12.8 user-defined provider block (verbatim) — contributor hook.
```toml
# ~/.config/stagehand/config.toml
[provider.myagent]
command = "/opt/myagent/bin/agent"
prompt_delivery = "stdin"
print_flag = "--once"
model_flag = "--model"
default_model = "my-model-7b"
system_prompt_flag = "--system"
bare_flags = ["--no-mcp", "--ephemeral"]
output = "raw"
```

## External references (from knowledge; implementer verify links resolve)
- opencommit: https://github.com/di-sukharev/opencommit
- aicommits: https://github.com/Nutlope/aicommits
- shields.io badges: https://shields.io
- asciinema: https://asciinema.org
- Standard Readme spec: https://github.com/RichardLitt/standard-readme
- Conventional Commits: https://www.conventionalcommits.org
- makeareadme.com: https://www.makeareadme.com

## Gotchas
- Use `raw/main/install.sh` (NOT releases/download) — install.sh isn't a release asset.
- Org = `dustin` everywhere (module/tap/bucket/install.sh), NOT local `dabstractor` remote.
- Providers are six: pi, claude (Claude Code), gemini (Gemini CLI), opencode, codex, cursor.
- v1 = ONE commit per invocation; multi-commit is v2 (§10.3) — state plainly in FAQ.
- Demo is a PLACEHOLDER — do not fabricate an asciinema URL; use a fenced TODO/placeholder block.
- No LICENSE file yet; goreleaser `license: MIT` is set. README may note MIT; do not invent a LICENSE file (out of scope).
