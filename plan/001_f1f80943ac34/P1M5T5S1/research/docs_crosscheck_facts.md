# P1.M5.T5.S1 — docs/ cross-cutting sync: VERIFIED FACTS

Source of truth = the IMPLEMENTED code + the live README.md + `.goreleaser.yaml` + PRD.md (root).
Every doc snippet must match these facts. Verified 2026-06-29.

## 1. docs/ DOES NOT EXIST yet — this task CREATES the whole set

- `ls docs/` → no such directory. There is NO `plan/.../docs/`-style content in the repo; the only
  repo `.md` files outside `plan/` are `README.md` and `PRD.md`.
- There are NO "Mode A" docs subtasks in this codebase (the contract's `docs/CONFIGURATION.md` /
  `docs/PROVIDERS.md` "if created by Mode A" clause does NOT apply — none were created). Therefore
  P1.M5.T5.S1 is NOT a "review-and-tweak" task; it MUST author the docs/ set from scratch.
- README.md (P1.M5.T4.S1) was authored in parallel and NOW EXISTS. It links to docs/ verbatim:
  `See the [docs/](docs/) for the full reference (growing).` — so docs/ must deliver a "full reference".

## 2. PRD location DISCREPANCY (CRITICAL — do NOT act on the contract's assumption)

- The work-item contract says: "The PRD itself lives in docs/PRD.md and is READ-ONLY."
- REALITY: `PRD.md` lives at the REPO ROOT (`/home/dustin/projects/stagehand/PRD.md`), NOT in docs/.
- DECISION: do NOT move PRD.md into docs/ (it is human-owned, READ-ONLY). Do NOT create a
  `docs/PRD.md` (it would duplicate/contradict the canonical root file). docs/README.md will LINK to
  the root PRD as "the authoritative product & technical specification" and note it is read-only.

## 3. README.md (P1.M5.T4.S1 — now EXISTS) — what it promises docs/ must provide

README "Full CLI and config reference" section (verbatim):
```
The authoritative, always-available reference lives in the binary itself:
  stagehand --help          # every flag, subcommand, and option
  stagehand config init     # writes a fully-commented config file (the canonical config reference)
  stagehand config path     # shows where the global config lives
See the docs/ for the full reference (growing).
```
→ docs/ must provide the FULL, browsable reference (deeper than --help): cli.md, configuration.md,
providers.md, + an architecture/how-it-works overview. README cites --help/config init as PRIMARY
(always-available) and docs/ as SECONDARY (growing) — docs/ must not contradict the binary.

### README env-var coverage (contract says "verify env-var sections are accurate")
- The README does NOT have a dedicated env-var section. Env vars appear ONLY inline, in the config
  precedence line: "CLI flags > STAGEHAND_* env vars > repo git config ...". This is ACCURATE but not
  exhaustive. docs/configuration.md will carry the FULL env-var table (§3.5). No README edit is
  warranted (it is accurate; depth belongs in docs/). Flag this as a "no-change-needed" finding.

## 4. markdownlint tooling (the L1 gate) — MUST pass the EXISTING config

- `.markdownlint.json` EXISTS at repo root (created by P1.M5.T4.S1):
  `{ "default": true, "MD013": false, "MD033": false, "MD060": false }`
  → line length (MD013) OFF, inline HTML (MD033) OFF, MD060 OFF. Other rules ON (MD041 first-line H1,
  MD024 dup headings, MD040 fenced-language, MD009 trailing spaces, etc.).
- `markdownlint-cli2 v0.22.1 (markdownlint v0.40.0)` is available via `npx markdownlint-cli2` (NOT on
  bare $PATH — `which` fails; use `npx`). Invoke: `npx markdownlint-cli2 'docs/**/*.md' README.md`.
- All new docs MUST pass this config with zero errors. Use language hints on every fenced block
  (```toml/```bash/```text/```go). First line of each doc = an H1. No duplicate headings.

## 5. CLI surface to document (docs/cli.md) — VERIFIED against internal/cmd/*.go

### Global flags (root.go init()) — ALL eleven
| Flag | Type | Default | Notes |
|------|------|---------|-------|
| `--provider <name>` | string | "" (auto-detect) | env STAGEHAND_PROVIDER; git stagehand.provider |
| `--model <name>` | string | "" (manifest default) | env STAGEHAND_MODEL; git stagehand.model |
| `--config <path>` | string | "" | overrides discovery; env STAGEHAND_CONFIG. NOT a Config field. |
| `--timeout <dur>` | string | "120s" | "120s" or bare seconds; env STAGEHAND_TIMEOUT; git stagehand.timeout |
| `--verbose`, `-v` | bool | false | resolved cmd + raw output + retries; env STAGEHAND_VERBOSE |
| `--no-color` | bool | TTY-aware | env STAGEHAND_NO_COLOR; also honors NO_COLOR |
| `--all`, `-a` | bool | false | `git add -A` before snapshotting even if something staged |
| `--no-auto-stage` | bool | false | nothing staged → exit 2 instead of auto-staging |
| `--dry-run` | bool | false | print message, do not commit (exit 0) |
| `--version` | — | "dev" locally | auto-registered by cobra (Version field); prints BEFORE config load |
| `--help`, `-h` | — | — | cobra builtin |

### Subcommands (§15.3)
- `stagehand providers list` → table: `NAME  DETECTED  DEFAULT` (✓/✗, `(default)` on resolved pick).
- `stagehand providers show <name>` → fully-resolved manifest as TOML; exit 1 if unknown.
- `stagehand config init` → writes commented example to GLOBAL path; REFUSES overwrite (exit 1).
- `stagehand config path` → prints resolved global config path.

### Exit codes (internal/exitcode/exitcode.go — AUTHORITATIVE, §15.4)
| Code | Const | Meaning |
|------|-------|---------|
| 0 | Success | commit created, or dry-run message printed |
| 1 | Error | general (generation/parse/agent-missing/CAS/flag-usage) |
| 2 | NothingToCommit | clean tree after auto-stage, or nothing staged with --no-auto-stage |
| 3 | Rescue | snapshot taken, commit NOT created — manual recovery printed (§18.3) |
| 124 | Timeout | generation exceeded --timeout (mirrors GNU `timeout`) |

NOTE: exitcode.For() ordering — explicit *ExitError → its Code; then ErrNothingToCommit→2; timeout
checked BEFORE rescue (a timeout IS a *RescueError with Kind=ErrTimeout → 124); ErrRescue→3;
DeadlineExceeded→124; CAS→1; else 1.

### Default action output (default_action.go)
- SUCCESS report to STDOUT: `[<7-char-sha>] <subject>` then one `STATUS  path` line per changed file
  (rename/copy shown as `R100  old → new`). Notices/diagnostics → STDERR.
- AUTO-STAGE notice (stderr): `Nothing staged — staging all changes (N files).` (FR18, verbatim w/ em-dash).
- DRY RUN: stdout = message ONLY; stderr = `(no commit created)`; exit 0.
- RESCUE (§18.3) → stderr block; main prints nothing extra (silent ExitError); exit 3 (or 124 if timeout).
- CAS failure → stderr "HEAD moved…" message; exit 1 (silent).

## 6. Config model (docs/configuration.md) — VERIFIED against config.go / config.go template / load.go

### 7-layer precedence (§16.1), HIGH → LOW (matches exampleConfigTemplate header):
1. CLI flags
2. STAGEHAND_* env vars
3. repo git-config (`stagehand.*`)
4. repo-local `./.stagehand.toml`
5. GLOBAL config file
6. provider `default_*` (manifest)
7. built-in defaults

### Env vars (FULL list)
STAGEHAND_PROVIDER, STAGEHAND_MODEL, STAGEHAND_TIMEOUT, STAGEHAND_CONFIG, STAGEHAND_VERBOSE,
STAGEHAND_NO_COLOR, plus the universal NO_COLOR (honored by --no-color resolution).

### Git-config keys (§16.3)
`stagehand.provider`, `stagehand.model`, `stagehand.timeout`, `stagehand.auto_stage_all`.

### Built-in defaults (config.go Defaults())
timeout 120s; auto_stage_all true; max_diff_bytes 300000; max_md_lines 100; max_duplicate_retries 3;
subject_target_chars 50; output "raw"; strip_code_fence true. Provider/Model = "" (auto-detect /
manifest default). NoColor is TTY-aware (UI layer, not a file field — `toml:"-"`).

### Paths
- GLOBAL file: `$XDG_CONFIG_HOME/stagehand/config.toml` (default `~/.config/stagehand/config.toml`).
- REPO-LOCAL file: `./.stagehand.toml` (gitignored by default).
- `config init` REFUSES to overwrite an existing file (exit 1); creates parent dirs; writes the
  commented exampleConfigTemplate (which IS the Mode-A config reference — every line `#`-commented).

### Config struct fields (config.go) — the resolved value type (NOT the file shape)
[defaults] Provider, Model, Timeout(time.Duration), AutoStageAll, Verbose, NoColor(toml:"-");
[generation] MaxDiffBytes, MaxMdLines, MaxDuplicateRetries, SubjectTargetChars, Output, StripCodeFence;
[provider.<name>] raw map `Providers map[string]map[string]any` (toml:"-").
NOTE: the FILE uses [defaults]/[generation] subtables + string durations; Config is the resolved form.

## 7. Provider manifests (docs/providers.md) — VERIFIED against manifest.go + builtin.go + providers/*.toml

### Manifest schema — all 18 toml fields (manifest.go grep)
name, detect, command, subcommand, prompt_delivery, prompt_flag, print_flag, model_flag,
default_model, system_prompt_flag, provider_flag, default_provider, bare_flags, output, json_field,
strip_code_fence, retry_instruction, env ([env] subtable).

### Defaults of the schema itself (when omitted): prompt_delivery→"stdin", output→"raw",
strip_code_fence→true, retry_instruction→"Output ONLY the commit message. No preamble, no markdown,
no quotes."; flags/print_flag/model_flag/etc default "" (no flag emitted).

### 6 built-ins (auto-detect order, registry.go preferredBuiltins):
pi, claude, gemini, opencode, codex, cursor. First DETECTED on $PATH (no config) = default.
User-defined [provider.<name>] are NEVER auto-selected.

### per-provider facts (for the manifest table) — from PRD §12.3–12.7 + builtin.go
- pi: stdin, -p, --model, --system-prompt, --provider, default_model glm-5-turbo; explicit --no-* tool-disable.
- claude: stdin, -p, --model, --system-prompt, `--tools ""`/`--setting-sources ""`/`--no-session-persistence`; sonnet.
- gemini: positional, no print_flag, -m, NO system_prompt_flag (prepend), --approval-mode default; gemini-2.5-pro.
- opencode: positional (subcommand "run"), -m (provider/model), no sys flag, default_model "" (user must set).
- codex: positional (subcommand "exec"), -m, no sys flag, --sandbox read-only --ask-for-approval never; default_model "".
- cursor: positional, -p (--print), --model, no sys flag, --mode ask --trust; default_model "".
- TOOLS-DISABLE ASYMMETRY (§12.7.1): explicit-switch (pi, claude) vs read-only-constraint (codex/cursor/gemini).

### Command rendering (§12.2) + output parsing (§12.9) — document the algorithm.

## 8. Install paths (docs/overview + cross-ref README) — §21.3 + .goreleaser.yaml (VERIFIED)
- Homebrew: `brew install dustin/tap/stagehand` (goreleaser homebrew tap repo dustin/homebrew-tap).
- Go install: `go install github.com/dustin/stagehand/cmd/stagehand@latest`.
- curl|sh: `curl -fsSL https://github.com/dustin/stagehand/raw/main/install.sh | bash` (NOTE: install.sh
  does NOT exist yet — published at first release; README already carries this note).
- Scoop: `scoop install dustin/stagehand` (bucket dustin/scoop-bucket).
- NAMESPACE = dustin/stagehand everywhere (goreleaser owner:dustin WINS over git-remote dabstractor).
  go.mod module path = github.com/dustin/stagehand.

## 9. How-it-works / architecture facts (docs/how-it-works.md) — §13 + §18 + §17

### Snapshot flow (§13.2/§13.3): write-tree → commit-tree → update-ref CAS. Invariants:
- committed content = exactly what was staged at write-tree time;
- later-staged files remain staged (index NEVER reset);
- atomic + safe — failed generation leaves repo byte-for-byte unchanged (only a tree/commit object
  left for git gc to reap);
- generation latency is overlap-able (stage-while-generating, §13.4 diagram).
### Rescue protocol (§18.2/§18.3): TREE_SHA set, NEW_SHA not → print recovery block w/ TREE_SHA +
PARENT_SHA + `git commit-tree -p <PARENT> -m "msg" <TREE_SHA> | xargs git update-ref HEAD`; candidate
message printed if available. Safety invariant §18.1 holds for all 6 providers.
### Prompt engineering (§17): mature-repo system prompt (style learn from last 20, anti-reuse, ~50-char
subject, multi-line rule); new-repo conventional-commit fallback; raw output contract (§17.4).

## 10. Files this task CREATES (the coherent docs/ set)
1. docs/README.md — overview + navigation index; links to the 4 below + the root PRD.
2. docs/cli.md — §15 in full (synopsis, all flags, subcommands, exit codes, examples, env-var map).
3. docs/configuration.md — §16 in full (precedence, file format, git-config, env vars, defaults, paths).
4. docs/providers.md — §12 in full (schema, rendering, 6 built-ins, asymmetry, extensibility, parsing).
5. docs/how-it-works.md — architecture overview (snapshot §13, safety/rescue §18, prompt §17).

## 11. SCOPE — do NOT touch (READ-ONLY / owned elsewhere)
- PRD.md (root) — read-only; link to it, never move/copy/edit.
- README.md — owned by P1.M5.T4.S1 (parallel). Do NOT edit; verify only (see §3 finding).
- All *.go, Makefile, .goreleaser.yaml, .github/*, providers/*.toml, .gitignore, .markdownlint.json,
  tasks.json, prd_snapshot.md — unchanged.
- install.sh, LICENSE — do NOT exist; do NOT create (human-owned).
