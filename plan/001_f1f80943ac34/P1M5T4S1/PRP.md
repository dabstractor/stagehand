---
name: "P1.M5.T4.S1 — README.md (hero, install, quick start, snapshot workflow, config, contributor guide, FAQ)"
description: |

  CREATE the repo-root **README.md** — Stagecoach's primary marketing + onboarding surface — following
  PRD §21.5's exact 10-section structure, using the **shipped** behavior of the already-implemented CLI
  (P1.M4.T1.S2), provider manifests (P1.M2.T2 / P1.M5.T2.S1), config system (P1.M1.T4 / P1.M4.T1.S4),
  and release tooling (P1.M5.T3.S2). No Go source is written. This is a single new Markdown file.

  CONTRACT (P1.M5.T4.S1, verbatim):
    1. RESEARCH NOTE: "PRD §21.5 README structure (10 sections): (1) Hero pitch (§5 verbatim candidate),
       (2) 30-sec demo (asciinema/gif placeholder), (3) 'Why not opencommit/aicommits?' (§4.3 coding-plan
       paragraph), (4) Install (4 paths from §21.3), (5) Quick start (one `stagecoach` invocation),
       (6) Configure agent (providers list → git config), (7) Snapshot workflow (§13.4 diagram),
       (8) Full CLI+config reference link, (9) Adding a new agent (§12.8 contributor hook),
       (10) FAQ / 'not for you if' (§7.4 anti-persona)."
    2. INPUT: "CLI behavior (P1.M4.T1.S2), providers commands (S3), config commands (S4), reference
       manifests (P1.M5.T2.S1), install paths (P1.M5.T3.S2)."
    3. LOGIC: "Write README.md following the §21.5 structure. Use the §5 hero pitch verbatim. Include
       the §13.4 stage-while-generating diagram (ASCII). Document the 4 install paths (Homebrew, go
       install, curl|sh, Scoop). Include the quick-start sequence and lazygit binding example (§15.5).
       Document the 'not for you if' anti-persona plainly. This is the primary marketing + onboarding
       surface."
    4. OUTPUT: "A comprehensive README.md that onboards new users and positions the product."
    5. DOCS: "[Mode B] This IS the changeset-level documentation for the README. This subtask is the
       final doc sweep for README.md (per §5 Mode B). Dependencies on all implementing subtasks ensure
       it runs last with accurate information."

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `README.md` is the ONLY file this subtask creates/edits. ONE new file at repo root.
    - `docs/` directory + `docs/cli.md` / `docs/config.md` → P1.M5.T5.S1 (Planned). The README's
      §21.5 #8 "Full reference" link points at docs/ (secondary) but MUST additionally cite the living
      `stagecoach --help` + `stagecoach config init` (primary) so it never dead-links before docs/ exists.
    - `Makefile` (incl. `coverage-gate`) → P1.M5.T3.S3 (parallel). README may mention `make` build for
      contributors but does NOT own Makefile docs.
    - `.goreleaser.yaml`, `.github/workflows/*` → P1.M5.T3.S1/S2. README cites their install outputs
      but does not edit them.
    - `install.sh`, `LICENSE` → DO NOT EXIST YET (see GAPs B & C). README documents the intended paths
      but does not create these files. Adding them is a human/release-task decision, NOT this subtask.
    - `PRD.md`, `tasks.json`, `prd_snapshot.md`, `.gitignore`, all `*.go` → READ-ONLY / unchanged.

  DELIVERABLE (CREATE one new file):
    CREATE README.md   # repo-root. §21.5's 10 sections, §5 hero verbatim, §13.4 diagram verbatim,
                       # 4 install paths verbatim, §15.5 lazygit binding, §7.4 anti-persona. Plain
                       # GitHub-flavored Markdown; clean under markdownlint-cli2.

  SUCCESS: `markdownlint-cli2 README.md` → 0 errors; every command/snippet matches the real binary
  (cross-checked via `stagecoach --help` / `providers list` / `config path` after `make build`); all
10
  §21.5 sections present; the hero pitch, the §13.4 diagram, the 4 install commands, and the lazygit
  binding are reproduced VERBATIM; no dead links; `git status --short` shows ONLY `README.md`.

---

## Goal

**Feature Goal**: Ship a single, comprehensive `README.md` at the repo root that is Stagecoach's
**primary marketing + onboarding surface** — a visitor who reads only this file can decide in 60
seconds whether Stagecoach is for them, install it, configure their agent, make their first
snapshot-based commit, and know how to contribute a new agent. It follows PRD §21.5's exact 10-section
structure and reflects the **already-shipped** CLI/provider/config/release behavior (not aspirational
features), because this is the final Mode-B doc sweep running last over accurate information.

**Deliverable**: ONE new file: `README.md` at the repository root (`/home/dustin/projects/stagecoach/README.md`).
Plain GitHub-flavored Markdown. ~300–600 lines. No other files touched.

**Success Definition**:
- `markdownlint-cli2 README.md` exits 0 (zero lint errors). *Tooling confirmed present: v0.22.1.*
- The 10 §21.5 sections are all present and in order (see the grep checklist in Validation Loop L3).
- The §5 hero pitch blockquote is reproduced **character-for-character verbatim**.
- The §13.4 stage-while-generating ASCII diagram is reproduced **character-for-character verbatim**.
- The 4 install commands (Homebrew / go install / curl|sh / Scoop) are present and **verbatim** from §21.3.
- The §15.5 lazygit `customCommands` binding is present verbatim.
- Every CLI command shown in the README is **real** — verified against `bin/stagecoach --help`,
  `providers list`, `config path`, `--version` after `make build` (or against the stub agent where a
  real agent isn't installed).
- No dead links: external links point at `github.com/dustin/stagecoach` (matching goreleaser + go.mod);
  internal links to `docs/` are paired with always-working `stagecoach --help` / `config init` fallbacks.
- `git status --short` shows ONLY `README.md`.

## User Persona

**Target User**: the **plan-holder** (PRD §7.1) — a developer who already pays for a coding-agent CLI
(Claude Code, Codex, Gemini CLI, pi, opencode, Cursor) and is scrolling GitHub/README looking for a
commit-message tool that spends their existing quota instead of a new API key. Secondary: the
**multi-agent tinkerer** (§7.3) who wants to add a new agent via a manifest.

**Use Case**: A developer lands on the repo, reads the hero pitch, sees the "stage while it thinks"
diagram, installs via one of 4 paths, runs `stagecoach` once, and commits a real message using their
installed agent — all within five minutes, no API key.

**User Journey**:
1. Hero pitch (§5) → "this uses the plan I already pay for."
2. Snapshot diagram (§13.4) → "generation time is no longer dead time."
3. "Why not opencommit/aicommits?" (§4.3) → "they can't reach my coding plan; this can."
4. Install (one of 4 paths).
5. Quick start: `stagecoach` → first snapshot commit.
6. Configure: `stagecoach providers list` → `git config stagecoach.provider <name>`.
7. (Optional) Add a new agent (§12.8) or wire lazygit (§15.5).
8. "Not for you if…" (§7.4) → graceful exit for the no-CLI user.

**Pain Points Addressed**: API-key fatigue; dead time while an agent generates; fear of repo
corruption from a half-failed commit tool; rigid single-provider tools (aicommits); the inability of
HTTP-owning tools (opencommit/aicommits) to reuse a coding-plan subscription.

## Why

- **It is the marketing surface (§21.5).** The README is the single highest-leverage artifact for
  adoption: it is what `go install`, Homebrew, Scoop, and GitHub all surface first. Per §21.5 it must
  carry the pitch, the demo, the install, the quick start, and the honest "not for you if."
- **Mode B — accurate, not aspirational.** Because this subtask runs LAST (after the CLI, providers,
  config, manifests, and release tooling), the README documents **what shipped**, not a roadmap. This
  avoids the classic failure of a README that promises a flag or command that doesn't exist.
- **Positions the structural moat.** §4.3's trade-off ("give up control of the model call in exchange
  for access to the user's existing quota") is the entire product; the README must state it in 3
  sentences so a visitor understands why Stagecoach exists alongside opencommit/aicommits.
- **Onboards contributors.** §12.8's drop-a-manifest extensibility is the contributor hook; documenting
  it in the README (§21.5 #9) is how community support for new agents lands without a release.
- **Honesty protects the install base.** §7.4's anti-persona ("not for you if you have no agent CLI")
  stated plainly in the FAQ avoids disappointing installs — a PRD requirement ("The README should say so
  plainly, to avoid disappointing installs").

## What

A single `README.md` with exactly the **10 sections of PRD §21.5**, in order. Each section's required
content is specified below; the verbatim text blocks are given in "Implementation Blueprint" so the
author reproduces them exactly.

| § | Section                          | Source                      | Required content                                                                 |
|---|----------------------------------|-----------------------------|----------------------------------------------------------------------------------|
| 1 | Hero                             | §5 (verbatim)               | The §5 one-sentence-pitch blockquote, verbatim.                                  |
| 2 | 30-second demo                   | new (placeholder)           | An asciinema/GIF placeholder block (not yet recorded).                           |
| 3 | "Why not opencommit/aicommits?"  | §4.3                        | The structural-moat trade-off, in **3 sentences**.                               |
| 4 | Install                          | §21.3 (verbatim)            | The 4 install paths (Homebrew, go install, curl\|sh, Scoop), verbatim.           |
| 5 | Quick start                      | §15.5                       | One `stagecoach` invocation + the `-a` checkpoint + `--dry-run` note.             |
| 6 | Configure your agent             | §15.5 + S3/S4 CLI           | `providers list` → `git config stagecoach.provider <name>` (+ `config init`).     |
| 7 | The snapshot workflow            | §13.4 (verbatim diagram)    | The two-pane ASCII diagram, verbatim, + a one-paragraph payoff explanation.      |
| 8 | Full CLI + config reference      | new + S1/S4                 | `stagecoach --help` + `stagecoach config init` (primary); link to `docs/` (secondary). |
| 9 | Adding a new agent               | §12.8                       | The `[provider.<name>]` manifest example + `providers show` verification.        |
| 10| FAQ / "not for you if…"          | §7.4                        | Plain anti-persona + 4–6 FAQ entries (security, multi-commit, style learning).   |

### Gap-handling decisions (REQUIRED — do not silently paper over)

These are facts about the repo today; the README must handle each deliberately (full reasoning in
`research/README_context.md` §6):

- **GAP A — namespace.** `git remote` is `dabstractor/stagecoach`, but `go.mod`, `.goreleaser.yaml`,
  and §21.3 all use `github.com/dustin/stagecoach`. **Use `dustin/stagecoach` in every URL** (Homebrew
  tap `dustin/tap`, Scoop `dustin/stagecoach`, go-install path, curl|sh URL). This matches what the
  released artifacts will use.
- **GAP B — `install.sh` does not exist yet.** The curl|sh URL points at a release-time script that
  goreleaser does not yet generate. **Keep the §21.3 curl|sh command verbatim** (it is the intended
  public path) but add a one-line note: "(the install script is published with the first release)".
  Do not invent a different URL.
- **GAP C — no LICENSE file.** `.goreleaser.yaml` tentatively says MIT. **Do not assert a license the
  repo lacks.** Omit the license badge, or add an MIT badge ONLY alongside a real `LICENSE` file
  (human-owned, out of scope). Preferred: a `<!-- TODO: add LICENSE file + badge -->` HTML comment so
  it is not forgotten.
- **GAP D — `docs/` does not exist yet (P1.M5.T5.S1 owns it).** The §8 reference MUST NOT dead-link.
  **Primary reference = `stagecoach --help` and `stagecoach config init`** (both ship today and ARE the
  living CLI/config reference). **Secondary = a relative link to `docs/`** (to be populated by
  P1.M5.T5). If `docs/` is empty at author time, the commands fully satisfy the reader.

### Success Criteria

- [ ] All 10 §21.5 sections present and in order (L3 grep checklist passes).
- [ ] §5 hero pitch blockquote verbatim; §13.4 diagram verbatim; 4 install commands verbatim;
      §15.5 lazygit binding verbatim.
- [ ] `markdownlint-cli2 README.md` → 0 errors.
- [ ] Every `stagecoach …` command in the README is real (matches `bin/stagecoach` after `make build`).
- [ ] No dead links; namespace = `dustin/stagecoach`; reference section uses `--help`/`config init`.
- [ ] Anti-persona ("not for you if") stated plainly.
- [ ] `git status --short` shows ONLY `README.md`.

## All Needed Context

### Context Completeness Check

_Pass._ An author who has never seen this repo can implement this from: the exact 10-section spec
(§21.5, in the table above); the **copy-pasteable verbatim blocks** (hero, diagram, install commands,
lazygit binding) in "Implementation Blueprint"; the verified CLI surface (flags, subcommands, success
report, config paths, precedence, default-provider order) in the same section; the four gap decisions
above; and a runnable validation loop (markdownlint-cli2 + binary cross-check). No prior knowledge of
the codebase required beyond what is in this PRP.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M5T4S1/research/README_context.md
  why: THE primary context doc. Verified facts about the shipped CLI (flags, subcommands, success
       report), the 6 providers + auto-detect order, config precedence + locations, the 4 install
       paths, and — critically — the 4 GAPs (namespace mismatch, missing install.sh, missing LICENSE,
       missing docs/) with the exact README decision for each. Read this FIRST.
  critical: §2 (CLI surface to document verbatim), §3 (default-provider order), §5 (install paths),
            §6 (the 4 GAPs), §8 (verbatim PRD blocks to reproduce).

# --- the shipped code the README must match (READ to quote commands accurately) ---
- file: internal/cmd/root.go
  why: the global flags (--provider/--model/--config/--timeout/--verbose/-v/--no-color/--all/-a/
       --no-auto-stage/--dry-run) + cobra-builtin --version/--help. Quote `stagecoach --help` output.
  pattern: the `pf.StringVar*`/`BoolVar*` registration block in init() is the authoritative flag list.
  gotcha: --version prints the ldflags-injected `version` ("dev" for a local build) — do NOT claim a
          specific release number in the README.

- file: internal/cmd/providers.go
  why: `providers list` (NAME/DETECTED/DEFAULT table, ✓/✗, "(default)") and `providers show <name>`
       (merged manifest as TOML; exit 1 if unknown). These are the §6 commands.
  pattern: the `Long:` help strings ARE the user-facing descriptions — quote them for accuracy.

- file: internal/cmd/config.go
  why: `config init` (writes commented example, REFUSES to overwrite) + `config path` (prints global
       path). The `exampleConfigTemplate` const IS the canonical config reference (Mode-A docs) — the
       README §8 points users at `config init` to get it.
  pattern: the precedence comment block in exampleConfigTemplate (CLI > env > git-config > repo
           .stagecoach.toml > global file > provider defaults > built-in defaults) — quote it.

- file: internal/cmd/default_action.go
  why: the success report format `[<7-char-sha>] <subject>` + file list, and the auto-stage notice
       "Nothing staged — staging all changes (N files)." Quote these so the quick-start example matches.
  pattern: printCommitReport() and the FR18 notice string.

- file: internal/provider/registry.go   (line 15)
  why: `preferredBuiltins = ["pi","claude","gemini","opencode","codex","cursor"]` — the auto-detect
       order (first installed wins). §6 documents "the first detected built-in becomes the default."

- file: providers/*.toml   (6 files: pi, claude, gemini, opencode, codex, cursor)
  why: the shipped reference manifests. §9's "adding a new agent" example should mirror the field set
       these real manifests use (command, prompt_delivery, print_flag, model_flag, default_model,
       system_prompt_flag, bare_flags, output). providers/pi.toml is the cleanest template to copy.
  pattern: the `[provider.<name>]` override structure + the "HOW TO USE IT AS A CONFIG OVERRIDE"
           header comment is exactly the §12.8 contributor recipe.

- file: .goreleaser.yaml
  why: confirms the install-path namespaces (dustin/stagecoach, dustin/homebrew-tap, dustin/scoop-bucket)
       and the "ADJUST license" caveat. Anchors GAP A and GAP C.
  gotcha: its `release.github.owner: dustin` EXPLICITLY overrides the git-remote `dabstractor` — proof
          that `dustin/stagecoach` is the correct public namespace for the README.

- file: .gitignore
  why: confirms `./.stagecoach.toml` (repo-local config) is gitignored by default (§19) — relevant to
       the §6 "configure" note ("per-repo; not committed"). No edit needed.

# --- the PRD (authoritative spec — the verbatim blocks come from here) ---
- doc: PRD.md §21.5   (the 10-section structure — the spine of the README)
- doc: PRD.md §5      (the verbatim hero-pitch blockquote)
- doc: PRD.md §13.4   (the verbatim stage-while-generating ASCII diagram)
- doc: PRD.md §21.3   (the verbatim 4 install commands)
- doc: PRD.md §15.5   (the verbatim lazygit customCommands binding + example invocations)
- doc: PRD.md §4.3    (the "Why not opencommit/aicommits?" structural-moat paragraph → 3 sentences)
- doc: PRD.md §12.8   (the "Adding a new agent" [provider.<name>] manifest example)
- doc: PRD.md §7.4    (the anti-persona — "Stagecoach is not for you if…")
- doc: PRD.md §16.2   (the full config file example — source for the config-init template)

# --- external (Markdown conventions + GitHub rendering) ---
- url: https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax
  why: GitHub-flavored Markdown syntax (fenced code blocks with language hints, task lists, tables,
       alerts). Ensures the README renders correctly on github.com.
  critical: fenced code blocks need a language hint (```bash, ```toml, ```yaml) for syntax coloring;
            GitHub "alerts" (> [!NOTE]) are a clean way to render GAP B/D notes without dead links.
- url: https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md
  why: markdownlint rule set (the L1 gate uses markdownlint-cli2 v0.22.1). Common lint fixes: MD013
       (line length — disable for README prose), MD033 (no inline HTML — GAP C's <!-- TODO --> comment
       needs MD033 disabled or a config), MD024 (no duplicate headings), MD041 (first line = H1).
```

### Current Codebase tree (relevant slice)

```bash
README.md                      # ← DOES NOT EXIST YET. YOU CREATE THIS FILE.
PRD.md                         # the spec (READ-only; verbatim blocks sourced from §5/§13.4/§21.3/§15.5).
go.mod                         # module github.com/dustin/stagecoach; go 1.22.
Makefile                       # `make build` -> ./bin/stagecoach; `make help` lists targets.
.goreleaser.yaml               # install-path namespaces (dustin/*) + license caveat.
.github/workflows/             # ci.yml (S1), release.yml (S2). README cites outputs, doesn't edit.
cmd/stagecoach/main.go          # `var version = "dev"` (ldflags-injected) -> stagecoach --version.
internal/cmd/{root,providers,config,default_action}.go  # the CLI surface to document.
internal/provider/{builtin,registry}.go                 # 6 built-ins + auto-detect order.
providers/{pi,claude,gemini,opencode,codex,cursor}.toml # shipped reference manifests (§9 template).
docs/                          # DOES NOT EXIST — owned by P1.M5.T5.S1 (Planned). GAP D.
install.sh                     # DOES NOT EXIST — release-time artifact. GAP B.
LICENSE                        # DOES NOT EXIST — human-owned. GAP C.
.stagecoach.toml                # repo-local example override (gitignored). Not part of README.
```

### Desired Codebase tree with files to be added/changed

```bash
README.md                      # CREATE — repo root. §21.5's 10 sections; verbatim hero/diagram/install/
                               #          lazygit; gap-aware; markdownlint-clean.
# (NO other files. NO LICENSE, install.sh, or docs/ created by this subtask — see GAPs B/C/D.)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
# CRITICAL (#1) — VERBATIM MEANS VERBATIM. Three PRD blocks MUST be reproduced character-for-character:
#   (a) the §5 hero blockquote, (b) the §13.4 two-pane ASCII diagram, (c) the 4 §21.3 install commands,
#   (d) the §15.5 lazygit customCommands YAML. Do NOT "tidy" the diagram's box-drawing spacing, do NOT
#   rewrap the hero pitch, do NOT add/remove flags from the install commands. Copy from "Implementation
#   Blueprint" verbatim. These are contractual ("Use the §5 hero pitch verbatim", "Include the §13.4
#   diagram (ASCII)", "Document the 4 install paths").

# CRITICAL (#2) — NAMESPACE = dustin/stagecoach, NOT dabstractor. `git remote` says dabstractor, but
#   go.mod + .goreleaser.yaml + §21.3 all say dustin/stagecoach, and .goreleaser.yaml explicitly
#   overrides the remote (`release.github.owner: dustin`). Every URL in the README (go-install path,
#   curl|sh URL, Homebrew tap, Scoop bucket, repo/issue links) MUST use dustin/stagecoach. A README
#   that prints `go install github.com/dabstractor/stagecoach/...` will produce a broken install.

# CRITICAL (#3) — NO DEAD LINKS. (a) GAP D: docs/ doesn't exist — the §8 reference section's PRIMARY
#   citations are `stagecoach --help` and `stagecoach config init` (both ship today); the docs/ link is
#   SECONDARY and explicitly "growing". (b) GAP B: the curl|sh URL's install.sh doesn't exist yet —
#   keep the §21.3 URL but add the "published with the first release" note. (c) Do NOT link to a
#   specific release tag (none exists) — link to `releases/latest` or omit version pinning.

# GOTCHA (#4) — DON'T PROMISE UNSHIPPED FEATURES. v1 is SINGLE-COMMIT (§10.1). Multi-commit hunk
#   decomposition is v2 (§10.3). The FAQ may mention multi-commit as "planned for v2" but must NOT
#   document a `--split` flag or multi-commit workflow as if it works. Same for any v1.1 items (§10.2).

# GOTCHA (#5) --version PRINTS "dev" LOCALLY. `var version` defaults to "dev" and is only set by
#   goreleaser's -ldflags at release. The README must not assert a version number (e.g. "v1.0.0"). If
#   showing a version badge, use a shields.io "GitHub Tag" or "Go Reference" badge (dynamic), not a
#   hardcoded number.

# GOTCHA (#6) — MARKDOWNLINT WILL COMPLAIN BY DEFAULT. markdownlint-cli2 v0.22.1 is the L1 gate.
#   Common friction: MD013 (line length 80) is too tight for prose + the wide ASCII diagram;
#   MD033 forbids inline HTML (the <!-- TODO: LICENSE --> comment triggers it); MD041 requires the
#   first line to be an H1. Ship a `.markdownlint.json` (or a `<!-- markdownlint-disable -->` block
#   around the ASCII diagram) so the diagram and the HTML comment pass. See Validation Loop L1 for the
#   recommended config. (NOTE: adding a .markdownlint.json config is ALLOWED — it is doc tooling, not
#   source. If you prefer zero new files, use inline disable comments instead.)

# GOTCHA (#7) — THE ASCII DIAGRAM IS WIDE. The §13.4 diagram is ~70 columns. In a fenced ``` block it
#   renders fine on GitHub (horizontal scroll). Do NOT wrap or narrow it — wrapping breaks the
#   two-pane alignment that IS the diagram's point.

# GOTCHA (#8) — SCOPE. This subtask creates README.md ONLY. Do NOT create LICENSE, install.sh, docs/,
#   or edit .goreleaser.yaml / Makefile / *.go / .gitignore. Those are human- or sibling-owned. If a
#   gap tempts you to create a file, instead document it (GAPs B/C/D) and move on.

# GOTCHA (#9) — FENCED CODE LANGUAGE HINTS. Use ```bash for shell, ```toml for manifests/config,
#   ```yaml for the lazygit binding, ```text for the ASCII diagram (so it isn't syntax-colored into
#   ugliness). GitHub renders these with correct coloring; markdownlint MD040 (fenced needs language)
#   passes.

# GOTCHA (#10) — MODE B. This README IS the changeset-level documentation for the repo (contract DOCS
#   bullet). It must reflect shipped behavior. If you find the README WOULD document a flag/command
#   that the binary does not actually have, STOP and either (a) omit it, or (b) flag it in the FAQ as
#   "planned" — do not document fiction as fact.
```

## Implementation Blueprint

### Data models and structure

_N/A — this subtask produces a Markdown document, not code. There are no data models, schemas, or
types to create. The "structure" is the §21.5 section order (the table in "What").

### Verbatim PRD blocks (copy-paste these EXACTLY into README.md)

These four blocks are contractual ("verbatim"). Reproduce them character-for-character.

**Block 1 — Hero pitch (§5)** — the §21.5 #1 hero, as a blockquote:

```markdown
> **Stagecoach writes your commit messages using the AI agent you already pay for.**
> No API key. No per-token billing. It shells out to Claude Code, Codex, Gemini CLI, pi, opencode, or
> Cursor — whatever you already have installed — and spends your existing coding-plan quota instead.
> Stage while it thinks; it commits only what was staged when it started, atomically, and can never
> corrupt your repo.
```

**Block 2 — Install (§21.3)** — the §21.5 #4 install paths, verbatim (add GAP B note under the curl line):

```bash
# Homebrew (macOS / Linuxbrew)
brew install dustin/tap/stagecoach

# Go install (anywhere with Go)
go install github.com/dustin/stagecoach/cmd/stagecoach@latest

# Direct binary (curl|sh one-liner from GitHub Releases)
curl -fsSL https://github.com/dustin/stagecoach/raw/main/install.sh | bash

# Windows (Scoop)
scoop install dustin/stagecoach
```
_Add under the curl|sh block (GAP B):_ `> [!NOTE] The install.sh script is published with the first
release. Until then, use Homebrew, go install, or Scoop.`

**Block 3 — Snapshot workflow diagram (§13.4)** — the §21.5 #7 diagram, verbatim in a ```text fence:

```text
Pane A (lazygit / shell)        Pane B (shell)
─────────────────────────       ───────────────────────
git add feature/login.js
stagecoach                     ┐
  ↳ snapshotting…             │  (user is free to work here)
  ↳ generating with pi…       │  git add docs/login.md
  ↳ (10s pass)                │  git add tests/login.test.js
  ↳ created abc1234           │  (these stay staged — NOT in abc1234)
                              ┘
                                stagecoach        # next run commits these
```
_Follow with a one-paragraph payoff (your own words):_ snapshot-based commits mean generation time is
no longer dead time — the in-flight commit only ever contains what was staged when it started, so you
can stage the next batch freely while the current message generates.

**Block 4 — lazygit binding (§15.5)** — include in §5 Quick start or §6 Configure:

```yaml
# From lazygit config.yml:
#   customCommands:
#     - key: '<c-a>'
#       command: 'stagecoach'
#       loadingText: 'Generating commit message…'
#       output: 'none'
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: VERIFY inputs (RUN, no edit) — author the README against SHIPPED behavior, not memory
  - RUN: `make build`            # produces ./bin/stagecoach (the README's commands must match THIS)
  - RUN: `./bin/stagecoach --help`           # capture the real flag list + descriptions (§8, §5, §6)
  - RUN: `./bin/stagecoach --version`        # expect "dev" — do NOT hardcode a version in the README
  - RUN: `./bin/stagecoach providers list`   # capture the NAME/DETECTED/DEFAULT table (§6)
  - RUN: `./bin/stagecoach providers show pi` # capture a real merged manifest (§9 example)
  - RUN: `./bin/stagecoach config path`      # capture the global config path string (§6/§8)
  - RUN: `./bin/stagecoach config init` (in a throwaway dir / inspect the template) # the config reference (§8)
  - READ: providers/pi.toml (cleanest §12.8 template); PRD §5/§13.4/§21.3/§15.5/§4.3/§7.4/§12.8.
  - NOTE every captured string so the README quotes the binary, not a guess.

Task 1: CREATE README.md — §21.5 sections 1–3 (Hero, Demo placeholder, "Why not")
  - §1 Hero: paste Block 1 (§5 pitch) VERBATIM as the first content under the H1. Add a one-line
    tagline beneath it (your words) + badges row (Go Reference, GitHub Actions CI status, Go Report
    Card — all DYNAMIC shields.io badges, no hardcoded version/license; see GAP C).
  - §2 30-sec demo: a fenced ```text or an HTML comment placeholder: `<!-- TODO: record asciinema
    demo; for now see the §13.4 diagram below -->` + a short "what you'll see" sentence. Do NOT embed
    a non-existent GIF link (dead asset). A GitHub alert block works well here.
  - §3 "Why not opencommit/aicommits?": EXACTLY 3 sentences distilled from §4.3 (draft in
    research/README_context.md §9): (1) incumbents own the HTTP call so they can normalize providers
    but cannot reach a coding-plan subscription (not reachable over the public API); (2) Stagecoach
    inverts the architecture — it shells out to your installed CLI agent, trading provider
    normalization for quota reuse; (3) that trade-off is the entire product, and the provider manifest
    (§12) makes the "give up normalization" part tolerable. Optionally a 2-row comparison table
    (Incumbents: API-key, HTTP-owned, no stage-while-generating vs Stagecoach: no key, shells out,
    snapshot commits) — keep it to the §4.3 facts.

Task 2: CREATE README.md — §21.5 sections 4–6 (Install, Quick start, Configure)
  - §4 Install: paste Block 2 (§21.3) VERBATIM; add the GAP B note under curl|sh. Add a one-line
    "prerequisite: a coding-agent CLI already installed and on $PATH (pi, claude, gemini, opencode,
    codex, or cursor)" — because Stagecoach is useless without one (§7.4).
  - §5 Quick start: one `stagecoach` invocation (the happy path). Show: `git add -p` (or `git add`)
    → `stagecoach` → the success report `[abc1234] feat: add login flow`. Then the two one-liners from
    §15.5: `stagecoach -a` (stage everything + commit) and `stagecoach --dry-run` (preview, commit
    nothing). Keep it to ~6 lines — this is "one `stagecoach` invocation" per the contract.
  - §6 Configure your agent: `stagecoach providers list` (shows the table; first ✓ DETECTED + "(default)"
    is the auto-pick) → set a per-repo default: `git config stagecoach.provider pi` (+ optional
    `git config stagecoach.model glm-5.2`). Mention `stagecoach config init` writes a fully-commented
    global config to `$(stagecoach config path)` and the precedence (CLI > env > git-config > repo
    .stagecoach.toml > global). Quote the real `providers list` output you captured in Task 0.

Task 3: CREATE README.md — §21.5 sections 7–8 (Snapshot workflow, Full reference)
  - §7 Snapshot workflow: paste Block 3 (§13.4 diagram) VERBATIM in a ```text fence. Add the
    one-paragraph payoff (your words — generation time is no longer dead time; the commit only ever
    contains what was staged when it started). Optionally the Block 4 lazygit binding here OR in §5/§6.
  - §8 Full CLI + config reference (GAP D — no dead links): PRIMARY = `stagecoach --help` (every flag)
    and `stagecoach config init` (writes the full commented config = the canonical reference). SECONDARY
    = a relative link `See the [docs/](docs/) for the full reference (growing).` State plainly that
    `--help` and `config init` are the authoritative, always-available reference today.

Task 4: CREATE README.md — §21.5 sections 9–10 (Adding a new agent, FAQ)
  - §9 Adding a new agent (§12.8 contributor hook): show dropping a `[provider.myagent]` block into the
    config file (mirror providers/pi.toml's field set: command, prompt_delivery, print_flag,
    model_flag, default_model, system_prompt_flag, bare_flags, output). Then verify with
    `stagecoach providers show myagent` and use with `stagecoach --provider myagent`. Emphasize "no
    recompilation — community agents land via a manifest file" (§4.3 / §12.8). Point contributors at
    the 6 shipped `providers/*.toml` files as copy-paste templates.
  - §10 FAQ / "not for you if…": lead with the §7.4 anti-persona, PLAINLY: "Stagecoach is not for you
    if you don't have (and don't want) a coding-agent CLI installed — it has no model of its own.
    [opencommit](https://github.com/dlintw/opencommit) is the right tool for the no-CLI user." Then
    4–6 FAQ entries, each grounded in shipped behavior: (a) "Will it corrupt my repo?" → no;
    snapshot-based: write-tree + commit-tree + atomic update-ref, failed generation leaves the repo
    byte-for-byte unchanged (§13.2/§18.1). (b) "Does it send my code anywhere new?" → no; it shells
    out to YOUR agent under YOUR existing auth/billing (§19). (c) "Can it write multiple commits?" →
    not in v1 (single commit); multi-commit decomposition is planned for v2 (§10.3). (d) "How does it
    match my style?" → learns from the last 20 commits, prohibits reusing their wording, guarantees no
    subject duplicates the last 50 (§5 #4). (e) "Which agents are supported?" → the 6 built-ins +
    any [provider.<name>] you define. (f) "How do I see what command it runs?" → `stagecoach --verbose`.

Task 5: REVIEW for accuracy + Mode-B honesty (READ the rendered README, no edit unless fixing)
  - For EVERY `stagecoach …`, `git …`, `brew …`, `go install …`, `scoop …` line in the README: confirm
    it is real (matches Task 0 captures / §21.3 / §15.5). Fix any fiction.
  - Confirm the 4 verbatim blocks are byte-identical to the PRP's "Verbatim PRD blocks".
  - Confirm GAP A (dustin/stagecoach everywhere), GAP B (curl note), GAP C (no fabricated license),
    GAP D (--help/config-init primary, docs/ secondary).
  - Confirm no unshipped feature is documented as working (GOTCHA #4: v1 = single-commit).

Task 6: VALIDATE (run the Validation Loop L1–L3; fix until green; see Validation Loop section)
  - RUN: `markdownlint-cli2 README.md` → 0 errors. (Ship a .markdownlint.json OR inline disables for
    MD013/MD033/MD040 around the diagram+HTML comment — see L1.)
  - RUN: the L3 grep checklist → all 10 sections + 4 verbatim blocks present.
  - RUN: `git status --short` → ONLY `README.md`.
```

### Implementation Patterns & Key Details

```markdown
# PATTERN — section ordering IS the contract. §21.5 lists 10 sections in priority order; the README's
# heading order MUST follow it (Hero → Demo → Why-not → Install → Quick start → Configure → Snapshot
# workflow → Full reference → Adding an agent → FAQ). Do not reorder "for flow" — the order is the spec.

# PATTERN — quote the binary, not your memory. Before writing any `stagecoach` example, run it (Task 0).
# The flag is `--no-auto-stage` (not `--no-autostage`); the subcommand is `providers list` (not
# `provider list`); `config init` REFUSES to overwrite (exit 1) — these details must be exact.

# PATTERN — badges are DYNAMIC, never hardcoded. shields.io Go Reference, GitHub Workflow Status
# (ci.yml), Go Report Card. NO version badge with a number (GOTCHA #5); NO license badge unless a
# LICENSE file exists (GAP C). Example: ![CI](https://github.com/dustin/stagecoach/actions/workflows/ci.yml/badge.svg)

# PATTERN — use GitHub alerts for the GAP notes. `> [!NOTE]` / `> [!IMPORTANT]` render cleanly and are
# not dead links. This is the idiomatic way to say "install.sh lands at first release" (GAP B) and
# "docs/ is growing — use --help/config init today" (GAP D).

# PATTERN — the §9 manifest example mirrors a SHIPPED manifest. Copy the field set from
# providers/pi.toml (the cleanest). Do not invent fields that the Manifest struct doesn't have
# (internal/provider/manifest.go toml tags are the authoritative field list).

# PATTERN — every external link uses dustin/stagecoach. Homebrew tap, Scoop bucket, go-install path,
# curl|sh URL, repo/issues/releases links, badge URLs. (GAP A.)
```

### Integration Points

```yaml
NEW FILE (the ONLY artifact):
  - CREATE: README.md at repo root (/home/dustin/projects/stagecoach/README.md).

OPTIONAL CONFIG (doc tooling — allowed, not required):
  - IF markdownlint-cli2 fails on MD013/MD033/MD040 and you don't want inline disables: CREATE a
    minimal .markdownlint.json at repo root (e.g. {"default": true, "MD013": false, "MD033": false}).
    This is doc-tooling config, NOT source. If you'd rather not add a second file, use inline
    <!-- markdownlint-disable --> / <!-- markdownlint-enable --> comments around the diagram + HTML
    comment instead (zero new files). Either is acceptable; pick one and be consistent.

DEPENDENCIES (the inputs this README documents — all Complete/Ready, so facts are stable):
  - CLI behavior         → P1.M4.T1.S2 (Complete): flags, subcommands, default action, success report.
  - providers commands   → P1.M4.T1.S3 (Complete): `providers list` / `providers show`.
  - config commands      → P1.M4.T1.S4 (Complete): `config init` / `config path`.
  - reference manifests  → P1.M5.T2.S1 (Complete): the 6 providers/*.toml files (§9 templates).
  - install paths        → P1.M5.T3.S2 (Complete): .goreleaser.yaml namespaces (§4).

HANDOFFS (do NOT create/edit — owned elsewhere):
  - docs/ overview + cross-cutting docs → P1.M5.T5.S1 (Planned). README links to docs/ (secondary).
  - Makefile (coverage-gate etc.)        → P1.M5.T3.S3 (parallel). README may cite `make build`.
  - .goreleaser.yaml / release.yml       → P1.M5.T3.S1/S2. README cites their install outputs.
  - install.sh, LICENSE                  → DO NOT EXIST. Human/release-task decision. Document, don't create.
  - PRD.md, tasks.json, *.go, .gitignore → READ-ONLY / unchanged.
```

## Validation Loop

### Level 1: Markdown Lint (Immediate Feedback)

```bash
# markdownlint-cli2 v0.22.1 is INSTALLED (verified). This is the primary structural gate.
markdownlint-cli2 README.md
echo "exit=$?"        # expect 0

# IF it fails on MD013 (line length), MD033 (inline HTML), or MD040 (fenced-language) — the ASCII
# diagram and the <!-- TODO: LICENSE --> comment are the usual culprits — fix via ONE of:
#   (a) CREATE a minimal repo-root .markdownlint.json:
#         { "default": true, "MD013": false, "MD033": false, "MD040": true }
#   (b) OR wrap the diagram + HTML comment in inline disables:
#         <!-- markdownlint-disable MD013 MD033 -->
#         ```text
#         <diagram>
#         ```
#         <!-- markdownlint-enable MD013 MD033 -->
# Re-run until exit 0.

# Also sanity-check first line is an H1 (MD041) and no duplicate headings (MD024):
markdownlint-cli2 README.md   # a clean run covers both.

# Expected: exit 0, zero errors. Fix before proceeding.
```

### Level 2: Command Accuracy (the README's commands must be REAL)

```bash
# Build the binary the README's commands must match.
make build
BIN=./bin/stagecoach

# (a) Every `stagecoach` flag/subcommand shown in the README must exist on the binary:
$BIN --help | tee /tmp/help.txt
#   for each flag in README (--provider/--model/--config/--timeout/--verbose/-v/--no-color/--all/-a/
#   --no-auto-stage/--dry-run/--version): grep -c -- "<flag>" /tmp/help.txt  -> >= 1
#   for each subcommand (providers list, providers show, config init, config path): present in help.

# (b) The success-report + table formats the README claims must match the binary's actual output:
$BIN providers list            # NAME/DETECTED/DEFAULT table; ✓/✗; "(default)" on the auto-pick
$BIN config path               # the global config path string the README quotes
$BIN --version                 # "dev" (do NOT claim a release number)

# (c) The §9 manifest example fields must be real Manifest fields (cross-check):
grep -n 'toml:' internal/provider/manifest.go | sed 's/.*toml:"//; s/".*//' | sort -u
#   every field you used in the [provider.myagent] example must appear in this list.

# (d) The install commands must match §21.3 verbatim (diff README's install block against the PRP's
#     Block 2):
grep -nA1 'brew install dustin/tap/stagecoach\|go install github.com/dustin/stagecoach\|install.sh\|scoop install dustin/stagecoach' README.md

# Expected: every README command resolves to a real flag/subcommand/field; install block verbatim.
```

### Level 3: Completeness (all 10 §21.5 sections + 4 verbatim blocks present)

```bash
# (a) The 10 §21.5 sections as headings (adjust exact wording, but all 10 topics must appear):
for h in "Stagecoach writes your commit messages" "demo\|asciinema\|30-second" \
         "Why not\|opencommit\|aicommits" "Install" "Quick start\|Quickstart" \
         "Configure" "snapshot\|stage while\|Snapshot" "reference\|Reference\|--help" \
         "Adding a new agent\|new agent\|Extensib" "FAQ\|not for you"; do
  grep -qiE "$h" README.md && echo "FOUND: $h" || echo "MISSING: $h"
done
# Expected: all 10 FOUND.

# (b) The 4 verbatim blocks (hero, diagram, install, lazygit) — substring presence:
grep -q "No API key. No per-token billing" README.md               && echo "hero verbatim OK"     || echo "HERO MISSING"
grep -q "Pane A (lazygit / shell)" README.md                       && echo "diagram verbatim OK" || echo "DIAGRAM MISSING"
grep -q "brew install dustin/tap/stagecoach" README.md             && echo "install verbatim OK" || echo "INSTALL MISSING"
grep -q "customCommands" README.md && grep -q "output: 'none'" README.md && echo "lazygit OK"     || echo "LAZYGIT MISSING"
# Expected: all four OK.

# (c) Gap handling:
grep -q "dustin/stagecoach" README.md && ! grep -qi "dabstractor" README.md && echo "namespace OK" || echo "NAMESPACE CHECK"
grep -qiE "first release|install.sh" README.md                     && echo "GAP-B note OK"       || echo "GAP-B MISSING"
grep -qiE "stagecoach --help" README.md && grep -qiE "config init" README.md && echo "GAP-D primary OK" || echo "GAP-D MISSING"

# Expected: namespace = dustin/stagecoach (no dabstractor); GAP-B note present; §8 cites --help/config init.
```

### Level 4: Render & Honesty (GitHub rendering + Mode-B accuracy)

```bash
# (a) Render check — no broken fenced blocks, tables, or alerts. (If gh is available:)
gh api repos/{owner}/{repo}/contents/README.md >/dev/null 2>&1 || echo "(offline — eyeball the .md)"

# (b) Dead-link audit — every URL in the README must resolve or be explicitly placeholder:
grep -oE 'https?://[^ )"]+' README.md | sort -u
#   eyeball: all should be dustin/stagecoach or shields.io or a real doc site. No dabstractor. No
#   version-pinned release that doesn't exist. The curl|sh install.sh URL is OK ONLY with the GAP-B note.

# (c) Mode-B honesty sweep — no unshipped feature documented as working:
grep -niE '\-\-split|multi-commit|multiple commits|decompos' README.md
#   any hit must be qualified "planned for v2" (§10.3), NOT presented as a working command. v1 = single commit.
grep -niE 'v1\.0\.0|version 1\.0|release 1\.0' README.md
#   no hardcoded release number (GOTCHA #5); --version prints "dev" locally.

# (d) Scope discipline — ONLY README.md changed (+ optionally .markdownlint.json if you added one):
git status --short
# Expected: "?? README.md" (and "?? .markdownlint.json" if you chose config over inline disables).
#           NOTHING else (no .go, Makefile, .goreleaser.yaml, .gitignore, LICENSE, install.sh, docs/).
```

## Final Validation Checklist

### Technical Validation

- [ ] `markdownlint-cli2 README.md` exits 0 (L1).
- [ ] Every README `stagecoach`/`git`/`brew`/`go install`/`scoop` command is real (L2: matches `bin/stagecoach`
      + §21.3 + §15.5).
- [ ] All 10 §21.5 sections present (L3 grep checklist: 10× FOUND).
- [ ] 4 verbatim blocks present (L3: hero + diagram + install + lazygit all OK).
- [ ] Gap handling: namespace `dustin/stagecoach` (no `dabstractor`); GAP-B note; GAP-C no fabricated
      license; GAP-D `--help`/`config init` primary + docs/ secondary (L3 + L4).
- [ ] No dead links; no hardcoded release version; no unshipped feature documented as working (L4).

### Feature Validation

- [ ] The §5 hero pitch is reproduced verbatim as the first content block.
- [ ] The §13.4 diagram is reproduced verbatim (two-pane ASCII, ```text fence).
- [ ] The 4 §21.3 install paths are present verbatim.
- [ ] The §15.5 lazygit binding is present verbatim.
- [ ] "Why not opencommit/aicommits?" is exactly ~3 sentences, grounded in §4.3.
- [ ] "Stagecoach is not for you if…" (§7.4) is stated plainly, pointing the no-CLI user at opencommit.
- [ ] §9 "Adding a new agent" mirrors a shipped manifest's field set + shows `providers show` verification.
- [ ] Quick start shows one `stagecoach` invocation + the success report format.

### Code Quality & Scope Validation

- [ ] `git status --short` shows ONLY `README.md` (+ optionally `.markdownlint.json`).
- [ ] No `.go`, `Makefile`, `.goreleaser.yaml`, `.github/*`, `.gitignore`, `LICENSE`, `install.sh`, or
      `docs/` created/edited by this subtask.
- [ ] Follows GitHub-flavored Markdown conventions (language hints on all fences; H1 first line).
- [ ] Mode-B honest: documents shipped behavior; flags anything unshipped as "planned" (not as working).

### Documentation & Deployment

- [ ] README is self-contained: a new visitor can install, configure, commit, and contribute an agent
      from this file alone (+ the binary's `--help`/`config init`).
- [ ] External links all use `github.com/dustin/stagecoach`.
- [ ] The four GAPs are each handled deliberately (not silently papered over).

---

## Anti-Patterns to Avoid

- ❌ Don't paraphrase the four verbatim blocks (hero, diagram, install, lazygit) — the contract says
      "verbatim." Copy them from "Implementation Blueprint" character-for-character.
- ❌ Don't use `dabstractor/stagecoach` in any URL — the public namespace is `dustin/stagecoach`
      (go.mod + goreleaser + §21.3). A dabstractor URL is a broken install.
- ❌ Don't link to `docs/` as the sole "Full reference" — docs/ doesn't exist yet (GAP D). Cite
      `stagecoach --help` and `stagecoach config init` (which ship today) as the primary reference.
- ❌ Don't assert a license (MIT or otherwise) — there is no LICENSE file (GAP C). Omit the badge or
      gate it behind a real LICENSE file (human-owned).
- ❌ Don't document an unshipped feature (multi-commit, `--split`, a specific version) as working — v1
      is single-commit; `--version` prints "dev" locally.
- ❌ Don't invent a `stagecoach` flag/subcommand from memory — run the binary (Task 0) and quote it.
- ❌ Don't create LICENSE, install.sh, docs/, or edit source/Makefile/release files — this subtask is
      README.md (+ optionally a markdownlint config) ONLY.
- ❌ Don't skip markdownlint because "it's just prose" — markdownlint-cli2 is installed and is the L1
      gate; a lint error is a validation failure.
- ❌ Don't reorder §21.5's 10 sections "for better flow" — the order is the spec (Hero → Demo →
      Why-not → Install → Quick start → Configure → Snapshot → Reference → New agent → FAQ).
- ❌ Don't pad the README with roadmap/marketing fluff — §21.5 is specific; every section earns its place.
