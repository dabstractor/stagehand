# Delta PRD — Session 005: v2.1 Competitor Feature Parity + Tool Integrations

| Field | Value |
|---|---|
| **Delta from** | plan/004_136878664597 (PRD v2.0, config v3 — fully implemented) |
| **Current PRD** | PRD.md v2.1 (2026-07-02) |
| **Diff size** | ~201 lines added / ~31 removed. Six new requirement sections (§9.18–§9.23), six new goals (G15–G20), nine new user stories (US18–US26), ~33 new FRs, two new packages (`internal/hook/`, `internal/integrate/`). |
| **Sizing verdict** | **Large delta** — six independent, fully-specified feature areas. Full phase/milestone structure is warranted, but every feature builds on the existing implemented pipeline; nothing is re-architected. |

## 1. What actually changed (diff analysis)

The v2.1 revision adds competitor-parity features decided against a source-level review of aicommits and opencommit. The entire delta is **additive spec** plus bookkeeping:

**New requirement sections (all net-new work):**
1. **§9.18 Payload exclusions** (FR-X1–X5, P1) — `.stagecoachignore` + repeatable `--exclude`/`-x` + `[generation].exclude`, union semantics, `[excluded]` placeholders, applies to every diff path like binary filtering (FR3c).
2. **§9.19 Message shaping** (FR-F1–F8, P1) — `--format auto|conventional|gitmoji|plain` (non-auto replaces the style-examples block, §17.8), `--locale`, `--context` (flag-only, user payload), `--template '$msg …'` (post-generation substitution, before duplicate check). New env/git-config keys (FR35/FR36 updated).
3. **§9.20 Git hook mode** (FR-H1–H7, P1) — `stagecoach hook install|uninstall|status` (`prepare-commit-msg`, marker-based, refuses foreign hooks, no `--force`) + `hook exec` runtime (source-gated no-op, writes to msg-file top, **never blocks the commit** — exit 0 on any failure unless `--strict`).
4. **§9.21 Tool integrations** (FR-I1–I6, P1) — `stagecoach integrate list|install|remove` with targets `git-alias` (delegates to `git config`) and `lazygit` (comment-preserving YAML upsert of a `customCommands` entry). The **no-mangle write protocol** (FR-I3: parse-first → idempotent marker upsert → preview+confirm → backup → post-write validate with auto-restore → surgical scope) is the core deliverable.
5. **§9.22 `--edit` and `--push`** (FR-E1–E4, FR-P1–P3, P2) — `$EDITOR` gate before `commit-tree` (snapshot frozen first, so staging stays safe; per-commit in decompose mode; rejected in hook mode), and plain `git push` after a fully-clean run (push failure ≠ commit failure, exit 1 with commits standing).
6. **§9.23 Discovery** (FR-L1–L3, P2) — `stagecoach models [<provider>]` (manifest `list_models_command` argv, else curated FR-D4 table; **never HTTP**), new optional manifest field `list_models_command` (§12.1), and `config init --interactive` (TTY-gated wizard writing the same FR-B1 file).

**Supporting spec changes (ride with the above, no separate tasks):**
- §12.1 manifest schema gains `list_models_command = []` (part of item 6).
- §14 package layout adds `internal/hook/` (item 3) and `internal/integrate/` (item 4).
- §16 CLI/flag tables, config example (`[generation] exclude/format/locale/template/push`), env vars (FR35), git-config keys (FR36) — each rides with its feature.
- §17.8 prompt-template spec for format/locale/context (rides with item 2).
- §21 research directives 14–17: lazygit customCommands schema (gates FR-I5), hook script portability on git-for-windows sh + `core.hooksPath`/worktrees (gates FR-H1), gitmoji table currency compiled into the binary (gates FR-F3), per-provider `list_models_command` verification (gates FR-L1/L2). These are **implementation-time verification steps inside the corresponding tasks**, not standalone research tasks.

**Removed / superseded (awareness only — no tasks):**
- §6.2 N3/N4 rewritten: `--edit` and the hook installer graduated from non-goals into the spec; the GitHub Action stays rejected.
- §10 roadmap collapsed: v1.1 candidate list dispositioned; the speculative §10.4 list replaced by `FUTURE_SPEC.md` (already authored, staged in git — **no work**).
- Appendix examples corrected to config-v3 terminology (`--planner-provider`, `provider = "pi"` / `model = "zai/glm-5.2"`). The codebase already implements v3; this is the PRD catching up to shipped code. **Verify-only:** confirm shipped docs/help carry no stale `--planner-agent`-style wording (expected clean).

**Caveat:** the PRD cites `COMPETITOR-ANALYSIS.md` as the decision evidence base, but that file is **not present in the repo**. Not a blocker — every accepted feature is fully specified in PRD.md itself — but do not create tasks that depend on reading it.

## 2. Relationship to completed work

Session 004 was a one-line-scale delta (reasoning default flip); its architecture directory (`plan/004_136878664597/architecture/`) **no longer exists on disk** — there is no reusable research. What matters from 004's record: the **entire v2.0/v3 core is implemented and green** — pipeline (`internal/generate/`), decompose (`internal/decompose/`), config v3 + roles + bootstrap (`internal/config/`), provider manifests (`internal/provider/`), prompts (`internal/prompt/`), CLI (`internal/cmd/`), plumbing (`internal/git/`), and a full `docs/` tree (cli.md, configuration.md, how-it-works.md, providers.md, README.md).

Every v2.1 feature is a composition over those existing seams — reference, do not re-implement:

| New feature | Builds on (existing) |
|---|---|
| Exclusions (§9.18) | The FR3/FR3a pathspec-exclusion + binary-placeholder machinery in diff capture; extend the same `:(exclude)` pathspec plumbing and placeholder emitter with an `[excluded]` tag and new pattern sources. |
| Format/locale/context/template (§9.19) | `internal/prompt/` system-prompt builder (style-examples block becomes swappable per §17.8); `internal/generate/` cleanup pipeline (template substitution slots between parse-cleanup and the FR30 duplicate check); config precedence in `internal/config/` (new scalar keys follow the existing flag>env>git>file cascade). |
| Hook mode (§9.20) | Reuses diff capture, prompt build, message-role resolution (§9.15), generation + duplicate rejection as-is; **skips** snapshot/commit-tree/update-ref entirely. New package `internal/hook/`. |
| Integrations (§9.21) | New package `internal/integrate/`; `git-alias` target delegates the file edit to `git config` via the existing git exec wrapper. |
| `--edit` / `--push` (§9.22) | Hooks into the single point between duplicate rejection and `commit-tree` (single path and each decompose publication); `--push` runs after the existing success/exit-code determination. |
| Discovery (§9.23) | Manifest struct + registry in `internal/provider/` gains one optional field; `config init --interactive` wraps the existing FR-B1 writer in `internal/config/bootstrap.go`. |

## 3. Requirements (delta scope)

One phase. Milestones ordered so shared infrastructure lands first (§9.18/§9.19 touch the pipeline every later feature reuses via hook mode), then the independent command surfaces.

### R1 — Payload exclusions (§9.18, FR-X1–X5; P1, G15)
Union pattern sources (built-ins, `.stagecoachignore`, `[generation].exclude` across config layers, repeated `--exclude`/`-x`); gitignore-style globs → `:(exclude)` pathspecs; `!` skipped with `--verbose` warning; `[excluded]` placeholder per changed excluded file; applied on all three diff paths (staged, decompose snapshot, per-concept tree-to-tree). Exclusion is payload-only — the commit is never altered.
- **Docs (Mode A):** docs/cli.md (`--exclude` row), docs/configuration.md (`[generation].exclude`, `.stagecoachignore` syntax + "excluded from what the agent sees, still committed" — FR-X5), docs/how-it-works.md (diff-capture section).

### R2 — Message shaping (§9.19, FR-F1–F8, §17.8; P1, G16)
`--format` (unknown mode = hard error; non-auto replaces style examples per §17.8 and applies to message role, FR-M11 shortcut, and arbiter's N+1 message), gitmoji table compiled in (verify currency, record date — directive 16), `--locale` verbatim append, `--context` user-payload block (message + planner), `--template` with mandatory `$msg` (hard error), substituted post-cleanup / pre-duplicate-check, applied to every commit in a run. New env vars (`STAGECOACH_FORMAT/LOCALE/TEMPLATE`) and git-config keys per FR35/FR36.
- **Docs (Mode A):** docs/cli.md (four flag rows), docs/configuration.md (`[generation]` keys + env/git-config tables), docs/how-it-works.md (prompt-construction section notes format-mode substitution).

### R3 — Git hook mode (§9.20, FR-H1–H7; P1, G17)
New `internal/hook/` package + `hook` cobra subcommand tree. Install via `git rev-parse --git-path hooks`, POSIX-sh script with `# stagecoach prepare-commit-msg hook v1` marker, `exec stagecoach hook exec "$@"`; foreign-hook refusal (no `--force`), `--print`, `--strict`; uninstall/status marker-gated. `hook exec`: source-gated no-op (`message|template|merge|squash|commit`), empty-diff no-op, else standard pipeline (with R1 exclusions + R2 shaping) writing above git's comment block; **exit 0 on any failure** unless `--strict`; message-role config resolution; never decomposes. Verify hook-script portability (directive 15) in-task.
- **Docs (Mode A):** docs/cli.md (`hook` commands); **the FR-H7 FAQ is a spec requirement, not optional** — trade-off inversion (plumbing = atomic + stage-while-generating, hooks bypassed; hook mode = hooks honored, no snapshot) documented in README.md and/or docs/how-it-works.md.

### R4 — Tool integrations (§9.21, FR-I1–I6; P1, G18)
New `internal/integrate/` package + `integrate` subcommand (`list`/`install <target>…`/`remove <target>…`, detection-gated). The no-mangle protocol (FR-I3) as a reusable `protocol.go`: parse-first refusal, marker idempotency, unified-diff preview + `y/N` (`--yes`), timestamped backup, post-write re-parse with auto-restore, surgical scope, create-if-missing. `git-alias` target delegates to `git config --global alias.<name> '!stagecoach'` (still previewed/confirmed; conflicting alias surfaced). `lazygit` target: comment-preserving YAML node upsert (yaml.v3 Node API or equivalent), config dir via `lazygit --print-config-dir`, defaults `key '<c-a>'` / `context 'files'` / `output 'none'`, `# stagecoach-integration` marker; verify current customCommands schema in-task (directive 14). Uninstall symmetry.
- **Docs (Mode A):** docs/cli.md (`integrate` commands), README.md quick-start mention rides with R7.

### R5 — `--edit` and `--push` (§9.22, FR-E1–E4, FR-P1–P3; P2, G19)
`--edit`: message + commented summary → `.git/STAGECOACH_EDITMSG`, `$GIT_EDITOR`→`$VISUAL`→`$EDITOR`→`vi`, strip comments/trailing whitespace, empty = abort exit 1 (not a rescue); snapshot frozen before editor (staging stays safe); template applied before editor; edited message bypasses duplicate re-check; per-commit gate in decompose; warn-ignore with `--dry-run`; usage error on `hook exec`. `--push`: plain `git push` after a fully-clean run only (skip on dry-run/zero-commits/rescue/CAS-abort), never prompts, push failure leaves commits standing → "commits created; push failed", exit 1. `STAGECOACH_PUSH` / `stagecoach.push` / `[generation].push`.
- **Docs (Mode A):** docs/cli.md (both flags), docs/configuration.md (`push` key), docs/how-it-works.md (edit-while-staging property — FR-E2 says docs call it out).

### R6 — Discovery (§9.23, FR-L1–L3; P2, G20)
Manifest field `list_models_command` (optional argv, default empty) in `internal/provider/` schema + built-in manifests (populate only where verified — directive 17; opencode's `opencode models` is the known case). `stagecoach models [<provider>]` / `--all`: run the argv and print stdout, else curated FR-D4 table with verification date + `--help` pointer; never HTTP. `config init --interactive`: TTY-gated (non-TTY exit 1 → plain `config init`), provider pick (FR-D1 default highlighted), per-role model accept/edit (multi-backend prompts for `inference/` prefix, never guesses), writes the FR-B1 file, composes with `--force`.
- **Docs (Mode A):** docs/cli.md (`models` command, `config init --interactive`), docs/providers.md (`list_models_command` manifest field), docs/configuration.md (interactive bootstrap note).

### R7 — Sync changeset-level documentation (Mode B; depends on R1–R6)
Final coherence sweep once all features land: README.md feature list / hero section reflects the six v2.1 capabilities (exclusions, shaping, hook mode, integrate, `--edit`/`--push`, `models`); the FR-H7 trade-off FAQ is present and discoverable; docs/README.md index lists any new pages; no stale claims survive ("no hook installer", "v1.1 will add `--edit`", pre-v3 `--planner-agent`/`agent =` terminology — expected clean, verify by grep). FUTURE_SPEC.md already exists at the repo root — reference it from README where deferred ideas are mentioned; do not author it.

## 4. Explicitly out of scope for this delta
- Everything in FUTURE_SPEC.md (GitHub Action, PR generation, editor extensions, gitui, generate-N-and-pick, multiselect, chunking, clipboard, self-update, `config describe`, `--body`/`--scope`/`--amend`, fuzzy dedupe, branch-aware context, telemetry, `--background`).
- Re-touching the implemented v2.0/v3 core (decompose, per-role config, config v3 migration, bootstrap, binary filtering) except at the named extension seams.
- Authoring COMPETITOR-ANALYSIS.md (referenced by the PRD but absent; the spec is self-contained).

## 5. Suggested structure for breakdown
One phase ("v2.1 competitor parity + tool integrations"), milestones M1–M7 mapping to R1–R7. R1 and R2 first (pipeline seams that R3 reuses); R3–R6 are mutually independent; R7 last, depending on all. Research directives 14–17 are subtask-level verification steps inside R2 (gitmoji), R3 (hook portability), R4 (lazygit schema), R6 (list_models_command) — not separate tasks.
