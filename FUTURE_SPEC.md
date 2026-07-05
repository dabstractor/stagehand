# Stagehand — Future Spec (deferred & rejected ideas)

Companion to `PRD.md` (v2.1). The PRD carries **no stubs or placeholders**: if a capability is
described there, it is in scope. Everything else — ideas we like but haven't specified, ideas
blocked on external factors, and ideas we have deliberately rejected — lives here, each with its
reasoning, so future revisions don't re-litigate them from scratch.

Dispositions were made against the source-level competitor review in `COMPETITOR-ANALYSIS.md`
under three rules: features both incumbents (aicommits, opencommit) share are accepted as far as
they agree with each other; anything contradicting the core feature set is disqualified even if
both have it; the rest judged on simplicity and value.

---

## 1. Deferred — we want this, it isn't specified yet

### 1.1 Editor integration family

The `prepare-commit-msg` hook (PRD §9.20) already gives every editor that commits through real
git — VS Code, JetBrains, neovim `:Git commit` via fugitive — a pre-filled commit-message box for
free. The extensions below are UX sugar on top of that (a button, a re-generate command, inline
status), not a new capability, which is why they're deferred rather than specified.

- **VS Code extension.** A ✨ button in the Source Control panel invoking `stagehand --dry-run`
  into the message box (aicommits ships one). Separate TypeScript artifact, marketplace
  publishing pipeline.
- **neovim / vim-fugitive.** A small plugin (or doc'd `:Git` mapping) wrapping `stagehand` /
  `stagehand --dry-run`. The primary author is a neovim user; likely the first one built.
- **Zed.** Has a git panel with commit-message box and runs git hooks; watch its extension API
  for a first-class hook point.
- **JetBrains.** Still a huge population. Its commit dialog executes git hooks, so hook mode
  covers it today; a plugin would add a toolbar action.

Common design note for all of these: they must shell out to the installed `stagehand` binary
(the quota/auth story lives in the user's shell environment), never re-implement generation.

### 1.2 More `integrate` targets (PRD §9.21)

- **gitui — blocked upstream.** Verified 2026-07-02 against the gitui changelog: `key_bindings.ron`
  can only remap built-in actions; there is no custom/external-command facility to bind
  `stagehand` to. Revisit if upstream ships custom commands.
- **tig.** `~/.tigrc` external-command keybind; line-oriented format, easy to edit safely.
- **magit / emacs.** Print-only (`integrate --print` style): never auto-edit a user's elisp.
- **Sublime Merge.** Custom command via a `.sublime-commands` JSON file.

All future targets inherit the FR-I3 no-mangle write protocol unchanged.

### 1.3 CLI candidates (carried from the old v1.1 list)

- **`--body` / `--no-body`** — force multi-line/single-line regardless of history detection.
- **`--scope` / `--type`** — hint conventional-commit scope/type (composes with `--format
  conventional`).
- **`--amend`** — regenerate the previous commit's message via the same plumbing (`commit-tree`
  against the same tree, CAS `update-ref`).
- **Fuzzy duplicate detection** — reject subjects within a configurable edit distance of a recent
  subject, not just exact matches (§9.7 today is exact-match).

### 1.4 Other roadmap ideas (carried from the old §10.4)

- **Branch-aware context** — include branch name / PR title in the prompt (pairs naturally with
  `--context`, PRD FR-F7).
- **Conventional-commit validation / auto-fixup** — lint mode over existing history.
- **Opt-in, anonymous telemetry** — never on by default (PRD N6 stands).
- **`--background` daemon mode** — fire-and-forget async generate-and-commit with notification;
  the snapshot model makes this safe in principle, but signal/rescue semantics need a design pass.
- **Interactive setup beyond `config init --interactive`** — e.g. a first-run wizard that also
  offers `hook install` and `integrate`.

---

## 2. Blocked — rejected until the world changes

### 2.1 GitHub Action (CI message rewriting)

opencommit's Action rewrites lazy commit messages on push, on a GitHub-hosted runner. It works
**only because opencommit is an API-key tool**: the key sits in a repo secret and the runner
bills per token. Stagehand's entire thesis is spending a coding-plan quota through a locally
authenticated agent CLI, and an ephemeral headless runner cannot do that without exporting OAuth
credentials into repo-level secrets — per-provider, refresh-fragile, ToS-hostile, and it makes
one person's personal plan pay for every contributor's pushes.

One partial exception exists: Anthropic sanctions `claude setup-token` for CI use by Pro/Max
subscribers. That is one provider's bespoke blessing, not a generalizable architecture across
pi/opencode/gemini/codex. **Revisit only if providers standardize sanctioned headless
plan-tokens.** Until then: rejected, not deferred-by-laziness.

---

## 3. Rejected — deliberate, with reasons

| Feature (competitor) | Why rejected |
|---|---|
| **API-key HTTP providers, token limits, proxy/custom headers** (both) | PRD N2 is a permanent boundary: stagehand never owns the model call. Both competitors have these; contradiction beats parity. |
| **PR title/body generation** (aicommits, beta) | PRD §6.3, permanent: stagehand writes commit messages, nothing else. Ranked #2 in the analysis; scope discipline wins anyway. |
| **Interactive confirm loop as the default** (both) | Contradicts the non-interactive atomic design (lazygit `output: none` flows). The opt-in `--edit` (PRD §9.22) is the accepted form. |
| **Generate N messages + pick** (aicommits) | Interactive selection contradicts the non-interactive default; also no parity (opencommit only has a regenerate loop). |
| **Interactive file multiselect when nothing staged** (opencommit) | Superseded by multi-commit decomposition (PRD §13.6), which solves the same problem without a picker. |
| **Push-after-commit *prompt*** (opencommit) | The prompt is the objection, not the push: accepted as the non-interactive `--push` flag (PRD FR-P1). |
| **Large-diff chunking — lossy map-reduce form** (aicommits) | The *summarize-each-chunk-then-combine* flavor degrades message quality and is permanently rejected. NOTE: a **lossless** multi-turn priming form (full diff delivered across request-sized session turns) has graduated to the spec — see PRD §9.24 (FR-T1–T12). The rejection above applies only to the lossy form; the original premise ("agent contexts are 200k+; byte caps bound the payload") is withdrawn — a provider's per-request reliability ceiling can fall well below its advertised window, which is exactly what §9.24 addresses. |
| **`--clipboard` mode** (aicommits) | `stagehand --dry-run --no-color \| wl-copy` (or `pbcopy`) is the same feature without per-platform clipboard dependencies. |
| **Self-update command** (aicommits) | Distribution is Homebrew/AUR/Scoop/`go install`; a self-updating binary fights its package manager and breaks checksums. |
| **`config describe`** (opencommit) | The populated bootstrap (FR-B1) plus `config init --template` already document every key in place. |
| **Locale i18n file trees** (opencommit: 20 files) | The *feature* shipped (`--locale`, FR-F6) — as one prompt line. The model is the translator; maintaining 20 locale files is incumbent baggage. |

---

*When promoting anything out of this file, move it into `PRD.md` as numbered FRs and delete it
here — an idea must live in exactly one of the two documents.*
