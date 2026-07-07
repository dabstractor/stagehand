# Stagecoach — Product Requirements & Technical Specification

| Field | Value |
|---|---|
| **Project name** | stagecoach |
| **Language / runtime** | Go (single static binary) |
| **Status** | v1.0 specification (draft) |
| **Author** | dustin |
| **Last updated** | 2026-06-29 |
| **Origin** | `commit-pi` / `commit-claude` (zsh), `~/projects/git-scripts` |
| **Document purpose** | Comprehensive PRD + technical specification. Defines the v1 product surface, architecture, provider model, configuration model, git plumbing, testing, and distribution. Supersedes ad-hoc design discussion. |

---

## 0. How to read this document

This is two documents fused into one:

- **Part I — Product Requirements (PRD):** sections 1–10. What we are building, for whom, why it is different, and what is explicitly out of scope. Read this to understand the *why* and the *what*.
- **Part II — Technical Specification:** sections 11–22. How it is built: package layout, provider manifest schema, git internals, CLI reference, config model, testing, release. Read this to implement it.

Appendices (A–F) contain reference material: full prompt templates, example terminal sessions, and a line-by-line porting map from `commit-pi`.

The single most important section for understanding the product's defensibility is **§5 (Unique Value Proposition)** and the single most important section for understanding the engineering is **§13 (The snapshot-based generation flow)**. If you read only two sections, read those.

---

# PART I — PRODUCT REQUIREMENTS

## 1. Executive summary

**Stagecoach** is a command-line tool that writes your git commit messages for you using the AI coding agent you already have installed and already pay for.

It is **not** another "API key in your config file, pay-per-token" commit generator. The incumbent tools in this space — `opencommit`, `aicommits`, `cz-git` — all talk directly to provider APIs (OpenAI, Anthropic, local Ollama) and require you to provision an API key and spend tokens against your API billing. Stagecoach refuses to do that. Instead, it shells out to whatever coding-agent CLI you already run — **Claude Code, Codex, Gemini CLI, pi, opencode, Cursor CLI** — and lets the generation count against the **coding-plan quota** you have already paid for (Claude Max, Codex Pro/Plus, Gemini Advanced, a self-hosted pi/opencode setup, etc.).

This is a structural gap the incumbents cannot close without ceasing to be what they are. The coding-plan subscriptions are deliberately billed against the *CLI product* via proprietary OAuth flows, not the public API. There is no API key that draws down a Claude Max allotment. Stagecoach is built entirely around that fact.

Stagecoach also brings two workflow properties that no incumbent offers:

1. **Snapshot-based atomic commits.** Before generation begins, Stagecoach snapshots the staged index into the git object store with `git write-tree`. The eventual commit is created with plumbing (`commit-tree` + `update-ref`) against that frozen snapshot, never against the live index. A failed or slow generation can therefore never corrupt your repository, and — critically — **you can keep staging the *next* batch of files while the *current* message is still being generated.** Those newly staged files are not swept into the in-flight commit; they stay staged, ready for the next run. `git commit` cannot do this, and neither can any tool that ends with `git commit`.
2. **Style learning with anti-duplicate guarantee.** Stagecoach reads the last 20 commit messages in the repository, detects whether the project uses single-line subjects or multi-line subjects-with-bodies, instructs the model to match the *style* while forbidding it from reusing the *wording*, and runs a post-generation check that rejects any subject that already exists in the last 50 commits, retrying with an explicit "this was already used, write something different" instruction.

The result is a tool that is faster to adopt than the incumbents (no API key, no new billing relationship — you already have the agent), safer than `git commit` (atomic, snapshot-based), and better at matching your repository's voice than a one-shot model call.

The name is **stagecoach**: the thing behind the curtain that moves the set into place while you keep working.

---

## 2. Background and motivation

### 2.1 The originating tool: `commit-pi`

Stagecoach is a generalization of `commit-pi`, a zsh script in the author's `~/projects/git-scripts` directory, exposed to git as the alias `git commit-pi` (and `git commit-claude`), and bound to `<c-a>` in lazygit. The script:

1. Captures the staged diff (markdown files capped at 100 lines each, other files capped at 300 KB total, with lock files / snapshots / sourcemaps / vendor excluded).
2. Snapshots the index with `git write-tree` and captures the parent SHA.
3. Builds a system prompt that includes the last 20 commit messages as style examples, with a hard rule against reusing their wording, plus a JSON output contract (`{"commit_message": "..."}`).
4. Pipes the diff + a short instruction into `pi --provider zai --model glm-5-turbo --system-prompt ... --no-tools --no-extensions --no-skills --no-prompt-templates --no-context-files --no-session -p` (bare, ephemeral, no tools, no session).
5. Parses the JSON, retries on malformed output, and runs a duplicate-rejection loop (up to 3 retries) against the last 50 commit subjects.
6. Creates the commit with `git commit-tree -p <parent> -m <msg> <tree>` and atomically advances HEAD with `git update-ref HEAD <new> <parent>` (the two-argument form that refuses to move HEAD if it has changed underneath us).
7. On any failure after the snapshot was taken, prints the tree SHA and the exact manual recovery commands so the user can finish the commit by hand.

`commit-pi` works and is used daily. It has two problems that motivate Stagecoach:

- **It is welded to one agent.** A near-identical `commit-claude` exists as a fork. Every new agent (codex, gemini, opencode, cursor) would require another fork. There is no abstraction.
- **It is not distributable.** It is zsh, uses zsh-specific array syntax (`${(f)"$()"}`), cannot be `import`ed, has no package-manager story, and is installed by cloning a repo and hand-writing git aliases. That is fine for the author and unacceptable for anyone else.

Stagecoach extracts the agent-agnostic kernel (everything except the agent invocation), replaces the agent invocation with a **provider manifest** that normalizes the differences between agents, and packages it as a single Go binary with first-class distribution.

### 2.2 Why "use your own CLI agent" is the right framing

A commit message is a trivial generation task: one system prompt, one diff in, one short string out. It is so simple that hitting a provider API directly is technically straightforward — roughly 100 lines covers both the OpenAI-compatible (`/v1/chat/completions`) and Anthropic (`/v1/messages`) shapes, which between them cover OpenAI, Anthropic, OpenRouter, Groq, Together, DeepSeek, Ollama, LM Studio, and a dozen others.

But "technically straightforward" is the wrong axis. The right axis is: **whose quota does this spend?**

The users Stagecoach is for already have a coding plan. They pay a flat monthly fee for Claude Max, or Codex Pro, or Gemini Advanced, or they run a self-hosted pi/opencode pointed at a model they already pay for. They have *already bought the tokens*. The incumbents ask them to *also* provision an API key and pay per token on top of that, for a task their existing agent does in under a second. That is the friction Stagecoach removes.

There is no way to spend a coding-plan subscription against the public API. The providers enforce this on purpose. So the only path to "use the quota you already have" is to invoke the CLI product the quota is attached to. Stagecoach is, at its core, a very nice wrapper around that invocation.

### 2.3 Why Go

- **Single static binary**, no runtime dependency. The user already has enough dependencies (the agent itself). Stagecoach should be one file in `$PATH`.
- **Distribution matches the audience.** The target user lives in the same ecosystem as `lazygit`, `gh`, `ripgrep`, `fd`, `bat`, `delta`, `fzf`. Those are all Go or Rust binaries distributed via Homebrew, AUR, Scoop, `go install`, and GitHub Releases. Stagecoach fits that mold exactly.
- **Cross-compilation is trivial.** `goreleaser` produces Linux/macOS/Windows × amd64/arm64 in one CI job.
- **Subprocess + git plumbing is Go's comfort zone.** `os/exec`, signal handling, stdin/stdout piping, structured error types — all stdlib, all ergonomic for exactly this workload.
- **Optionally importable as a library.** Go modules mean `pkg/stagecoach` can be `import`ed by anyone building a git GUI, a CI hook, or a pre-commit integration. This is a freebie, not the primary artifact (see §6, non-goals), but it costs nothing to expose.

TypeScript/npm was the only serious alternative (`npx` zero-friction trial is compelling). Go wins on distribution fit and on the zero-runtime-dependency property that matters for a tool people invoke from lazygit and shell aliases.

---

## 3. Problem statement

### 3.1 The problem, in one sentence

Developers who already pay for an AI coding plan are forced to provision and pay for a *separate* API key — and accept per-token billing — just to generate commit messages, because every existing tool in the category talks to provider APIs directly and none of them will invoke the agent the developer already runs.

### 3.2 The secondary problem

Even developers who are fine with API-key tools get a worse workflow than they should, because those tools end with `git commit`, which commits *whatever is in the index at commit time*. If the message generation takes ten seconds and the developer stages more files during that window — which is the natural thing to do — those files get swept into a commit they were not meant to be in, or the developer has to sit idle waiting for generation to finish before staging the next batch. There is no way to overlap staging with generation safely.

### 3.3 The tertiary problem

Existing tools either ignore the repository's commit-message conventions (producing generic messages that clash with a project's voice) or learn them naively (producing messages that are near-duplicates of recent commits). Neither is good enough to leave on autopilot.

### 3.4 What Stagecoach is not solving

Stagecoach is **not** an AI pair-programmer, a code-reviewer, a PR-description writer, a changelog generator, or a release manager. It writes commit messages. Keeping the scope narrow is a feature: it is one thing, done well, that composes with everything else.

---

## 4. Competitive landscape

### 4.1 Direct competitors

| Tool | Language | Auth model | Invokes a CLI agent? | Multi-commit? | Style learning? | Atomic / snapshot? | Stage-while-generating? |
|---|---|---|---|---|---|---|---|
| **opencommit** (`oco`) | Node/TS | API key (OpenAI/Anthropic/Ollama/OpenAI-compatible) | **No** | Yes (partitions hunks) | Configurable (per-type templates) | No (ends with `git commit`) | **No** |
| **aicommits** | Node/TS | API key (OpenAI only) | **No** | No | Minimal | No (ends with `git commit`) | **No** |
| **cz-git** (+ AI plugin) | Node/TS | API key (plugin) | **No** | No | Conventional templates | No | **No** |
| **gitmoji-cli** | Node/TS | None (interactive picker) | N/A | No | N/A | No | **No** |
| **commitizen** | Python | None (interactive) | N/A | No | Conventional | No | **No** |
| **Stagecoach** | Go | **None — uses your installed CLI agent** | **Yes** | v2 (v1 = single commit) | Yes (last 20, anti-duplicate) | **Yes** (`write-tree` + `commit-tree` + `update-ref`) | **Yes** |

### 4.2 What each incumbent does well (so we respect them, not dismiss them)

- **opencommit** is the feature leader. It supports many providers, hunk-level multi-commit splitting, config files, per-type message templates, and a polished CLI. It is the tool to beat on *configuration depth*. We should not try to out-feature it on v1; we should beat it on *the one thing it structurally cannot do* (use your coding plan) and on *workflow safety* (snapshot commits).
- **aicommits** is the simplicity leader. One command, one provider, no config. It proved the category has demand (very high install counts). Its limitation is its rigidity (OpenAI only).
- **commitizen / cz-git** own the *conventional-commits-as-process* niche. They are interactive wizards, not AI generators (though cz-git has an AI plugin). Stagecoach is not competing for the "enforce a commit convention via a wizard" use case; it competes for "write the message for me."

### 4.3 The structural moat

The cell that reads **"Invokes a CLI agent? — No"** for every incumbent is not an oversight on their part. It is a consequence of their architecture: they own the HTTP call to the model, so they can normalize providers, handle retries, and abstract auth. Once you own the HTTP call, you cannot use a coding-plan subscription, because that subscription is not reachable over the public API.

Stagecoach inverts the architecture. It does **not** own the model call. It hands the prompt to an external process (the user's agent) and reads its stdout. This is strictly worse for provider normalization (we cannot abstract auth; we cannot retry at the HTTP layer; we are at the mercy of each agent's CLI surface) and strictly better for quota reuse (the agent brings its own auth, its own billing relationship, its own model routing). The provider manifest (§12) exists to make the "strictly worse" part tolerable.

That trade-off — *give up control of the model call in exchange for access to the user's existing quota* — is the entire product. Every design decision downstream follows from it.

---

## 5. Unique value proposition

Stagecoach's positioning, in priority order:

1. **Use the coding plan you already pay for.** No API key. No new billing relationship. The agent you already run does the work, against the quota you already bought. Works with Claude Code, Codex, Gemini CLI, pi, opencode, Cursor CLI, or any agent that exposes a non-interactive prompt interface.
2. **Keep staging while it thinks.** Snapshot-based commits mean generation time is no longer dead time. Stage the next batch while the current message generates; the in-flight commit only ever contains what was staged when it started.
3. **Never corrupt your repo.** A failed generation leaves the repository byte-for-byte unchanged. The index and HEAD are touched only at the final, atomic `update-ref` step, and only if HEAD hasn't moved underneath us.
4. **Match your project's voice.** Style is learned from the last 20 commits, with an explicit prohibition on reusing their wording, and a hard guarantee that no generated subject duplicates one of the last 50.
5. **Agent-agnostic by construction.** Switching from Claude Code to pi to Gemini is one config line or one `--provider` flag, not a reinstall. New agents are added by dropping a manifest file, not by forking the tool.

The README hero pitch, verbatim candidate:

> **Stagecoach writes your commit messages using the AI agent you already pay for.**
> No API key. No per-token billing. It shells out to Claude Code, Codex, Gemini CLI, pi, opencode, or Cursor — whatever you already have installed — and spends your existing coding-plan quota instead. Stage while it thinks; it commits only what was staged when it started, atomically, and can never corrupt your repo.

---

## 6. Goals and non-goals

### 6.1 Goals (v1)

- **G1.** Generate a commit message from staged changes by invoking a user-selected CLI agent, with zero API-key configuration.
- **G2.** Support at least four agents out of the box: **pi, Claude Code, Gemini CLI, opencode**. Document **Codex** and **Cursor CLI** manifests as best-effort (verify flags at integration time; see §12.5).
- **G3.** Implement the snapshot-based atomic commit flow (write-tree → generate → commit-tree → update-ref) faithfully ported from `commit-pi`.
- **G4.** Implement stage-while-generating: the commit is created against the frozen snapshot, not the live index.
- **G5.** Implement auto-stage-all when invoked with nothing staged (the v1 simplification requested by the author).
- **G6.** Implement style learning (last 20 commits) with multi-line detection and an anti-duplicate retry loop (last 50 subjects).
- **G7.** Ship a provider-manifest system that lets users override built-in agents and define new ones via config files, without recompiling.
- **G8.** Ship a configuration model with clear precedence: flag > env > repo git-config > repo file > global file > built-in defaults.
- **G9.** Distribute as a single static binary via Homebrew, `go install`, GitHub Releases (Linux/macOS/Windows × amd64/arm64), and AUR.
- **G10.** Ship a rescue protocol: on any failure after the snapshot, print the tree SHA and exact manual recovery commands.

### 6.2 Non-goals (v1 — explicitly deferred)

- **N1.** Multi-commit decomposition (splitting one staging batch into N logical commits). This is the headline v2 feature. v1 always produces exactly one commit per invocation. (See §10 roadmap.)
- **N2.** Direct HTTP API calls to providers. Stagecoach will never grow an `--api-key` flag for model access. This is a deliberate, permanent architectural boundary, not a limitation to be lifted later. (A user who wants direct API access should use opencommit.)
- **N3.** Interactive commit-message editing / a TUI editor. v1 prints the message and commits. A `--edit` flag that drops into `$EDITOR` between generation and commit is a likely v1.1 addition but not required for v1.0.
- **N4.** Pre-commit hook / CI integration as a shipped artifact. Users can wire Stagecoach into hooks themselves; we do not ship a hook installer in v1.
- **N5.** Managing the agent's authentication. Stagecoach never sees, stores, or touches the agent's tokens. If the agent isn't authenticated, Stagecoach surfaces the agent's error and exits.
- **N6.** Telemetry / analytics in v1. Possibly opt-in later; never on by default.

### 6.3 Non-goals (permanent)

- Stagecoach will never be an AI coding assistant, code reviewer, or PR writer. It writes commit messages. Scope discipline is the product.

---

## 7. Target users and personas

### 7.1 Primary persona — "the plan-holder"

**Dustin (the author).** Pays for Claude Max (or equivalent). Runs `claude`, `pi`, `gemini`, and `opencode` daily. Already uses `commit-pi` via lazygit `<c-a>`. Wants to (a) stop maintaining per-agent forks, (b) share the tool with colleagues, (c) keep the snapshot-based workflow he already relies on. He will install via Homebrew and configure via a TOML file or git config.

### 7.2 Secondary persona — "the API-key refusenik"

A developer who looked at `opencommit`/`aicommits`, saw "paste your OpenAI key," and closed the tab. Reasons vary: cost control (doesn't want surprise token spend on a subscription they already pay for), security policy (employer forbids pasting API keys into third-party tools), or simple annoyance (already authenticated to Claude Code via OAuth, doesn't want a second credential). This user is delighted by "it uses what you already have."

### 7.3 Tertiary persona — "the multi-agent tinkerer"

A developer who runs several agents (pi for open models, Claude Code for hard problems, Gemini for long context) and wants to choose per-repository or per-invocation which one writes commits. The `--provider` flag and per-repo git-config keys (`stagecoach.provider`) are for this user.

### 7.4 Anti-persona — "the no-CLI user"

A developer who does not have any coding-agent CLI installed and does not want one. Stagecoach is useless to this person. We do not optimize for them; opencommit is the right tool for them. The README should say so plainly, to avoid disappointing installs.

---

## 8. User stories

Format: *As a &lt;persona&gt;, I want &lt;capability&gt;, so that &lt;benefit&gt;.*

- **US1.** As a plan-holder, I want to run `stagecoach` and get a commit message generated by my default agent against my existing quota, so that I don't have to configure or pay for an API key.
- **US2.** As a plan-holder, I want to stage files for the *next* commit while the *current* message is generating, so that generation latency is not dead time.
- **US3.** As a plan-holder, I want a failed generation to leave my repository completely untouched, so that I never have to recover from a half-committed state.
- **US4.** As a multi-agent tinkerer, I want to switch agents with `stagecoach --provider gemini`, so that I can route commit generation to whichever agent I prefer at the moment.
- **US5.** As a multi-agent tinkerer, I want a per-repo default (`git config stagecoach.provider pi`), so that each repository remembers its preferred agent.
- **US6.** As a plan-holder, I want Stagecoach to match my project's commit-message style (length, tone, subject-vs-body), so that generated messages don't stick out in `git log`.
- **US7.** As a plan-holder, I want Stagecoach to guarantee no duplicate subjects, so that `git log` doesn't contain the same line twice.
- **US8.** As a lazygit user, I want to bind `stagecoach` to a key (`<c-a>`) with `output: none`, so that the commit happens silently and I stay in the lazygit UI.
- **US9.** As a new-user evaluator, I want to run `stagecoach --dry-run` to see the generated message without committing, so that I can judge quality before trusting it.
- **US10.** As a new-user evaluator, I want `stagecoach providers list` to show which agents I have installed and which is the default, so that I can understand what will happen before I run it.
- **US11.** As a plan-holder with nothing staged, I want `stagecoach` to stage all changes and commit them in one message (v1 behavior), so that I don't have to pre-stage for a quick checkpoint commit.
- **US12.** As an integration builder, I want a stable Go API (`pkg/stagecoach.GenerateCommit`) so that I can embed commit-message generation in a larger tool.

---

## 9. Functional requirements

Each requirement has an ID (FR-n), a priority (P0 = must for v1, P1 = should for v1, P2 = nice for v1), and a mapping to the goals in §6.1.

### 9.1 Diff capture (P0, → G1, G3)

- **FR1.** Capture the staged diff via `git diff --cached`.
- **FR2.** Markdown files (`.md`, `.markdown`): include full diff capped at N lines per file (default 100, configurable via `max_md_lines`).
- **FR3.** Non-markdown files: include diff with pathspec exclusions for lock files (`*.lock`, `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`), snapshots (`*.snap`), sourcemaps (`*.map`), and vendored code (`vendor/*`), capped at N bytes total (default 300,000, configurable via `max_diff_bytes`).
- **FR4.** Concatenate markdown diff and other diff into a single payload.
- **FR5.** If the combined diff is empty after capture, follow the nothing-staged path (§9.4).

### 9.2 Snapshot (P0, → G3, G4)

- **FR6.** Capture `PARENT_SHA = git rev-parse HEAD` (may be empty on a rootless repo).
- **FR7.** Snapshot the index with `TREE_SHA = git write-tree`.
- **FR8.** If `write-tree` fails (e.g., unresolved merge conflicts in the index), abort with a clear error before any generation.
- **FR9.** Store `(PARENT_SHA, TREE_SHA)` for the commit and rescue steps.

### 9.3 Prompt construction (P0, → G6)

- **FR10.** Count commits (`git rev-list --count HEAD`).
- **FR11.** For repos with >1 commit: fetch the last 20 full commit messages (`git log --format="---%n%B" -20`), trimmed, capped at 100 lines.
- **FR12.** Detect whether the history contains multi-line commits (subject + body) by scanning the examples.
- **FR13.** Construct the system prompt with: role ("commit message generator"), output contract (raw text = the message, nothing else), essence-not-filenames instruction, style examples with an explicit anti-reuse prohibition, multi-line rule conditioned on FR12, and a subject-length target (~50 chars).
- **FR14.** For repos with ≤1 commit: use a conventional-commit fallback prompt (`type(scope): description`, ~50 chars).
- **FR15.** The user-facing instruction is a short, stable string (e.g., "Generate a commit message for these changes:") followed by the diff payload. The diff is delivered via **stdin**, never as a command-line argument (avoids arg-length limits and shell injection).

### 9.4 Nothing-staged / auto-stage-all (P0, → G5)

- **FR16.** If `git diff --cached --quiet` reports no staged changes (FR5 path): if `auto_stage_all` is enabled (default: **true**), run `git add -A`, then re-check for changes.
- **FR17.** If after auto-stage there are still no changes (clean working tree), exit with a friendly "nothing to commit" message and exit code 2.
- **FR18.** Print a transparent notice when auto-staging occurs, e.g. `Nothing staged — staging all changes (3 files).`
- **FR19.** Provide `--no-auto-stage` to disable auto-staging for a single invocation (exit with "nothing staged" instead).
- **FR20.** Provide `--all` / `-a` to force `git add -A` even when something is already staged (stages additional files before snapshotting).

### 9.5 Generation (P0, → G1, G2)

- **FR21.** Resolve the provider (manifest) per the precedence in §9.8.
- **FR22.** Render the provider's command: base command, model flag + value, system-prompt flag + value (if the provider supports one), provider flag + value (for agents like pi that have sub-providers), bare-mode flags, and the print-mode flag.
- **FR23.** Deliver the prompt payload to the agent's stdin (or positional/flag, per the manifest's `prompt_delivery`).
- **FR24.** Capture the agent's stdout.
- **FR25.** Impose a configurable generation timeout (default 120s; `STAGECOACH_TIMEOUT` / `--timeout`). On timeout, kill the agent process and enter the rescue path.

### 9.6 Output parsing (P0, → G1)

- **FR26.** Default output mode: **raw** — the agent's stdout, stripped of leading/trailing whitespace and (optionally) surrounding markdown code fences, is the commit message.
- **FR27.** Alternative output mode: **json** — parse stdout as JSON, extract the configured field (default `result` for Claude Code's `--output-format json`; configurable via `json_field`).
- **FR28.** Robust extraction pipeline: (a) strip whitespace; (b) if the output begins with a code fence (``` or ~~~), unwrap one layer; (c) if `output=json`, attempt `json.Unmarshal` on the whole, then on the first `{...}` block found; (d) fall back to treating the entire (cleaned) stdout as the message if all structured attempts fail.
- **FR29.** On empty/failed parse, retry generation once with a corrective instruction (per the manifest's `retry_instruction`, default: "Output only the commit message, with no preamble, no markdown, and no quotes.").

### 9.7 Duplicate rejection (P0, → G6)

- **FR30.** Extract the generated subject (first line of the message).
- **FR31.** Fetch the last 50 commit subjects (`git log --format=%s -50`).
- **FR32.** If the subject exactly matches one of the 50, retry generation with an explicit rejection list appended to the user prompt: "The following messages were rejected because they already exist; generate something completely different: …". Up to `max_duplicate_retries` (default 3) retries.
- **FR33.** On exhausting duplicate retries, enter the rescue path (the snapshot still exists; the user can commit manually).

### 9.8 Configuration & precedence (P0, → G8)

- **FR34.** Precedence, highest to lowest: **CLI flags > environment variables > per-repo git config (`stagecoach.*`) > per-repo file (`.stagecoach.toml`) > global file (`$XDG_CONFIG_HOME/stagecoach/config.toml`) > built-in provider defaults > built-in defaults.**
- **FR35.** Environment variables use the `STAGECOACH_` prefix (`STAGECOACH_PROVIDER`, `STAGECOACH_MODEL`, `STAGECOACH_TIMEOUT`, `STAGECOACH_CONFIG`, `STAGECOACH_VERBOSE`, `STAGECOACH_NO_COLOR`).
- **FR36.** Git config keys live under the `stagecoach.` section (`stagecoach.provider`, `stagecoach.model`, `stagecoach.timeout`, `stagecoach.auto_stage_all`, etc.). Read via `git config --get`.
- **FR37.** A config file may define provider overrides (`[provider.<name>]`), defaults (`[defaults]`), and generation tuning (`[generation]`).
- **FR38.** `stagecoach config init` writes a commented example config to the global config path. `stagecoach config path` prints the resolved config path.

### 9.9 Commit creation (P0, → G3, G4)

- **FR39.** Create the commit object: if `PARENT_SHA` is non-empty, `git commit-tree -p <PARENT_SHA> -m <MSG> <TREE_SHA>`; else `git commit-tree -m <MSG> <TREE_SHA>` (root commit).
- **FR40.** Advance HEAD atomically: `git update-ref HEAD <NEW_SHA> <PARENT_SHA>` (the two-arg form refuses to move HEAD if its current value is not `<PARENT_SHA>`).
- **FR41.** If `update-ref` fails (HEAD moved concurrently), abort with a clear message and a manual recovery command. Do **not** force-update.
- **FR42.** On success, print `[<short-sha>] <subject>` and `git diff-tree --no-commit-id --name-status -r <NEW_SHA>` so the user sees what landed.

### 9.10 Rescue protocol (P0, → G10)

- **FR43.** Define a rescue condition: generation or parsing failed after the snapshot was taken (`TREE_SHA` is set and `NEW_SHA` is not).
- **FR44.** On rescue, print: a failure notice, the `TREE_SHA`, and the exact manual recovery command (`git commit-tree [-p <PARENT>] -m "…" <TREE> | xargs git update-ref HEAD`).
- **FR45.** Install a SIGINT/SIGTERM handler that triggers the rescue path if interrupted after the snapshot.

### 9.11 Provider management (P1, → G7)

- **FR46.** `stagecoach providers list` — list built-in providers, mark which are detected on `$PATH`, show the resolved default.
- **FR47.** `stagecoach providers show <name>` — print the fully-resolved manifest for a provider (built-in merged with user overrides), as TOML.
- **FR48.** User-defined providers in config files override built-ins of the same name; new names add new providers.

### 9.12 Dry run (P1)

- **FR49.** `--dry-run` — run the full diff→snapshot→generate→parse→duplicate-check pipeline, print the resulting message, but **do not** create the commit or move HEAD. Exit 0.

### 9.13 Verbosity & color (P1)

- **FR50.** `--verbose` / `-v` / `STAGECOACH_VERBOSE=1` — print the resolved provider command, the raw agent stdout, and each retry attempt to stderr.
- **FR51.** Color output when stdout is a TTY; disable with `--no-color` or `NO_COLOR`. Progress messages go to stderr so stdout stays clean for piping.

---

## 10. v1 scope, v1.1, and v2 roadmap

### 10.1 v1.0 (the ship list)

Everything in §9 marked P0 or P1. Concretely: diff capture, snapshot, prompt construction, auto-stage-all, generation via provider manifest, raw/json parsing with robust fallback, duplicate rejection, atomic commit, rescue protocol, config precedence, `providers list/show`, `config init/path`, `--dry-run`, `--verbose`, color. Built-in manifests for pi, Claude Code, Gemini CLI, opencode; documented (possibly stubbed) manifests for Codex and Cursor CLI.

### 10.2 v1.1 (likely quick follow-ons)

- **`--edit`**: drop into `$EDITOR` with the generated message before committing.
- **`--body` / `--no-body`**: force multi-line or single-line regardless of history detection.
- **`--scope` / `--type`**: hint conventional-commit scope/type to the model.
- **`--amend`**: amend the previous commit's message via generation.
- **Fuzzy duplicate detection**: reject subjects within a Levenshtein distance of N of a recent subject (configurable), not just exact matches.
- **Per-provider `model` overrides in config** beyond the single global default.

### 10.3 v2 (the big one) — multi-commit decomposition

The headline v2 feature: given a large staging batch, partition the diff into logically coherent groups (by file proximity, by hunk coherence, by an LLM-assisted grouping pass) and produce **N commits** instead of one, each with its own generated message, applied sequentially via the snapshot machinery.

This is why the snapshot/atomic-commit foundation is being built so carefully in v1: v2 reuses it N times in a loop. The v1 auto-stage-all "one commit" behavior is explicitly the placeholder until v2 lands. Design notes for v2 are out of scope for this document but the v1 architecture must not preclude it (see §11.3).

### 10.4 v2+ (speculative)

- Pre-commit hook installer (`stagecoach hook install`).
- Branch-aware context (include PR title / branch name in the prompt).
- Conventional-commit validation and auto-fixup.
- Gitmoji support.
- Opt-in, anonymous usage telemetry.
- A `--background` daemon mode that generates and commits asynchronously, notifying on completion (turns generation latency into true fire-and-forget).

---

# PART II — TECHNICAL SPECIFICATION

## 11. Architecture overview

### 11.1 High-level data flow

```
                         ┌──────────────┐
   git index (staged) ──▶│  diff capture │── diff payload ──┐
                         └──────────────┘                   │
                                                            ▼
                         ┌──────────────┐          ┌────────────────┐
                         │  snapshot    │          │ prompt builder │
                         │ write-tree   │          │ (style learn)  │
                         └──────┬───────┘          └────────┬───────┘
                                │ TREE_SHA, PARENT  system+user prompt
                                │                           │
                                │                           ▼
                                │                 ┌──────────────────┐
                                │                 │ provider executor│──▶ external CLI agent (stdin→stdout)
                                │                 └────────┬─────────┘
                                │                          │ raw/json output
                                │                          ▼
                                │                 ┌──────────────────┐
                                │                 │  parse + dedupe  │── retry loop ──┐
                                │                 └────────┬─────────┘               │
                                │                          │ commit message          │
                                ▼                          ▼
                         ┌──────────────────────────────────────┐
                         │ commit-tree -p PARENT -m MSG TREE     │──▶ NEW_SHA
                         │ update-ref HEAD NEW PARENT (atomic)   │
                         └──────────────────────────────────────┘
                                │ on failure ──▶ rescue protocol (print TREE_SHA + recovery cmd)
```

The flow is deliberately linear and synchronous in v1. Concurrency (stage-while-generating) is achieved not by backgrounding Stagecoach, but by the user running `git add` in another terminal/pane during the blocking generation call — which is safe precisely because the commit is built from the frozen `TREE_SHA`, not the live index. See §13 for the full mechanics.

### 11.2 Process model

Stagecoach is a single process. It shells out to git (multiple times) and to the agent CLI (once per attempt). All subprocesses inherit Stagecoach's working directory (the repo root) and environment, with a controlled, minimal set of extra env vars passed to the agent only if the manifest requests them. Stagecoach owns signal handling: SIGINT/SIGTERM propagates to the currently-running child and then triggers the rescue path.

### 11.3 Design constraints that protect v2

The v2 multi-commit feature will need to: (a) partition the diff, (b) for each partition, stage exactly that subset, snapshot, generate, commit, repeat. The v1 architecture must make this trivially composable. Concretely:

- The core is a function `commitStaged(ctx, cfg) error` (or `(sha, error)`) that assumes the index is already in the desired state. It does not decide *what* to stage; it commits *whatever is staged*.
- v1's `main` is: `maybeAutoStage(); commitStaged()`.
- v2's `main` will be: `for each partition { reset+stage partition; commitStaged() }`.

This means v1 must **not** entangle staging policy with commit logic. §11.4's package layout enforces this.

---

## 12. The provider system

The provider system is the heart of Stagecoach's agent-agnosticism. Its job: given a logical intent ("call an agent with this system prompt and this user payload, bare and ephemeral, with this model"), produce a concrete command line for a specific agent, run it, and parse the result.

### 12.1 The manifest schema

Each provider is described by a manifest. Manifests are TOML (human-editable, no quoting hell for flag lists). Built-in manifests are compiled into the binary (so the tool works with zero config); user manifests in config files override or extend them.

```toml
# A provider manifest. All fields except `name` and `command` are optional
# with sensible defaults; shown here fully expanded for pi.

name = "pi"

# --- discovery -----------------------------------------------------------
# Command to look up on $PATH to decide if this provider is "installed".
# If absent, `command` is used.
detect = "pi"

# The executable to run. Resolved via exec.LookPath; may be an absolute path.
command = "pi"

# Optional subcommand tokens inserted between `command` and the flags
# (e.g. opencode uses ["run"], codex uses ["exec"]).
subcommand = []

# --- prompt delivery -----------------------------------------------------
# How the user payload (system-built prompt + diff) reaches the agent.
#   "stdin"      → pipe to the process stdin (DEFAULT; avoids arg-length limits)
#   "positional" → append as the final positional argument
#   "flag"       → append after `prompt_flag`
prompt_delivery = "stdin"
prompt_flag = ""          # used only when prompt_delivery = "flag"

# --- non-interactive / print mode ---------------------------------------
# Token(s) that put the agent into non-interactive "print and exit" mode.
print_flag = "-p"

# --- model ---------------------------------------------------------------
model_flag = "--model"
# Default model if the user specifies none. Overridable per-invocation.
default_model = "glm-5-turbo"

# --- system prompt -------------------------------------------------------
# If the agent supports a system prompt. Empty means "prepend to the user
# payload" (fallback for agents with no system-prompt flag).
system_prompt_flag = "--system-prompt"

# --- sub-provider (agents that route to multiple backends) ---------------
# pi has --provider zai|anthropic|google|...; opencode uses provider/model
# in the model string instead. Optional.
provider_flag = "--provider"
default_provider = ""     # e.g. "zai" for pi

# --- bare mode -----------------------------------------------------------
# Flags appended verbatim to make the call tool-less, session-less,
# extension-less, chrome-less, and ephemeral. These are agent-specific.
bare_flags = [
  "--no-tools",
  "--no-extensions",
  "--no-skills",
  "--no-prompt-templates",
  "--no-context-files",
  "--no-session",
]

# --- output --------------------------------------------------------------
#   "raw"  → stdout (cleaned) IS the message            [DEFAULT]
#   "json" → stdout is JSON; extract `json_field`
output = "raw"
json_field = ""           # e.g. "result" when output = "json"

# Strip a single layer of markdown code fence (``` or ~~~) if present.
strip_code_fence = true

# --- retry ---------------------------------------------------------------
# Instruction prepended on a parse-retry (empty/invalid output).
retry_instruction = "Output ONLY the commit message. No preamble, no markdown, no quotes."

# --- environment ---------------------------------------------------------
# Extra env vars to set ONLY for the agent subprocess (never global).
[env]
# PI_OFFLINE = "1"   # example; commented out by default
```

### 12.2 Command rendering algorithm

Given a manifest `m`, a resolved model `model`, a sub-provider `provider`, a system prompt `sys`, and a user payload `user`:

```
args = [m.subcommand...]
if m.provider_flag and provider != "":
    args += [m.provider_flag, provider]
if m.model_flag and model != "":
    args += [m.model_flag, model]
if m.system_prompt_flag and sys != "":
    args += [m.system_prompt_flag, sys]
args += m.bare_flags
if m.print_flag != "":
    args += [m.print_flag]
switch m.prompt_delivery:
  case "stdin":       (prompt goes to stdin; nothing appended)
  case "positional":  args += [user]
  case "flag":        args += [m.prompt_flag, user]

cmd = exec.Command(m.command, args...)
cmd.Stdin = (m.prompt_delivery == "stdin") ? strings.NewReader(sys? + user) : /dev/null
cmd.Env   = os.Environ() + m.env
```

**Note on system prompt + stdin:** when delivery is `stdin` and a system-prompt flag exists, the system prompt goes via the flag and only the user payload goes via stdin (matching `commit-pi`). If the agent has *no* system-prompt flag (`system_prompt_flag = ""`), the system prompt is prepended to the stdin payload as a fallback.

### 12.3 Built-in provider: pi

Captured from `pi --help` on the author's machine (2026-06-29).

```toml
name = "pi"
detect = "pi"
command = "pi"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "--model"
default_model = "glm-5-turbo"
system_prompt_flag = "--system-prompt"
provider_flag = "--provider"
default_provider = ""        # user sets e.g. "zai" if they want GLM
bare_flags = [
  "--no-tools",
  "--no-extensions",
  "--no-skills",
  "--no-prompt-templates",
  "--no-context-files",
  "--no-session",
]
output = "raw"
strip_code_fence = true
```

Rendered (model `glm-5-turbo`, provider `zai`):

```
pi --provider zai --model glm-5-turbo --system-prompt "<sys>" \
   --no-tools --no-extensions --no-skills --no-prompt-templates \
   --no-context-files --no-session -p            < <user payload via stdin>
```

This is byte-for-byte the invocation `commit-pi` uses today.

### 12.4 Built-in provider: Claude Code

Captured from `claude --help`.

```toml
name = "claude"
detect = "claude"
command = "claude"
prompt_delivery = "stdin"           # claude -p reads stdin when no positional given
print_flag = "-p"                   # also enables non-interactive mode
model_flag = "--model"
default_model = "sonnet"            # alias; user can override with a full name
system_prompt_flag = "--system-prompt"
provider_flag = ""                  # n/a
bare_flags = [
  "--tools", "",                   # disable ALL built-in tools (per --help)
  "--setting-sources", "",         # load no settings sources
  "--no-session-persistence",      # ephemeral
]
output = "raw"                      # could use "json" with json_field="result"
strip_code_fence = true
```

Rendered (model `sonnet`):

```
claude -p --model sonnet --system-prompt "<sys>" \
       --tools "" --setting-sources "" --no-session-persistence   < <user payload>
```

Notes:
- `--tools ""` is documented (`Use "" to disable all tools`).
- `--system-prompt` *replaces* the default; `--append-system-prompt` *adds to* it. We use the replacing form for a clean, bare call. (Configurable: a user who wants CC's default persona retained can switch the flag to `--append-system-prompt`.)
- `--output-format json` + `json_field = "result"` is an alternative if raw mode proves unreliable with a given model.

### 12.5 Built-in provider: Gemini CLI

Captured from `gemini --help`.

```toml
name = "gemini"
detect = "gemini"
command = "gemini"
prompt_delivery = "positional"      # positional `query`; stdin is appended if present
print_flag = ""                     # no separate print flag; positional implies one-shot
model_flag = "-m"
default_model = "gemini-2.5-pro"
system_prompt_flag = ""             # gemini-cli has no first-class --system flag at present
provider_flag = ""
bare_flags = [
  "--approval-mode", "default",     # don't auto-run tools
]
output = "raw"
strip_code_fence = true
# fallback: system prompt prepended to the positional payload (see §12.2)
```

Rendered (model `gemini-2.5-pro`):

```
gemini -m gemini-2.5-pro --approval-mode default "<sys>\n\n<user payload>"
```

Caveats (to verify at integration time): the `-p/--prompt` flag is deprecated in favor of the positional `query`, and the help notes stdin is appended to the prompt — so `prompt_delivery = "stdin"` may also work and is preferable for large diffs (avoids arg-length limits). The manifest should default to whichever is verified to handle a ~300 KB payload; candidates are `stdin` first, `positional` as fallback. Gemini CLI's lack of a system-prompt flag means the system prompt is prepended to the payload per §12.2.

### 12.6 Built-in provider: opencode

Captured from `opencode run --help`.

```toml
name = "opencode"
detect = "opencode"
command = "opencode"
subcommand = ["run"]
prompt_delivery = "positional"      # `opencode run [message..]`
print_flag = ""
model_flag = "-m"                   # format: provider/model, e.g. "anthropic/claude-sonnet-4"
default_model = ""                  # opencode has no single sensible default; require user set
system_prompt_flag = ""             # not exposed as a flag on `run`; use --agent or config
provider_flag = ""                  # provider is part of the model string
bare_flags = []
output = "raw"
strip_code_fence = true
```

Rendered (model `anthropic/claude-sonnet-4`):

```
opencode run -m anthropic/claude-sonnet-4 "<sys>\n\n<user payload>"
```

Caveats: opencode's `run` subcommand is non-interactive and prints the final message to stdout (good). It has no system-prompt flag; the system prompt is prepended to the payload. For finer control of agent persona, opencode supports `--agent <name>` against a user-defined agent in `opencode.json` — Stagecoach can expose this via an `extra_args` passthrough or a dedicated `agent_flag` field in a later revision. `default_model` is intentionally empty: opencode's model space is huge and user-specific, so we require the user to set `model` (via flag/env/config) rather than guess.

### 12.7 Verified providers: Codex, Cursor Agent

Both were verified against their real `--help` output. They are **not** marked `experimental` — the flag surfaces below are confirmed. Two residual details to confirm at integration time are called out inline; neither blocks the manifest shape.

```toml
# Codex (OpenAI). Verified from `codex --help`.
name = "codex"
detect = "codex"
command = "codex"
subcommand = ["exec"]               # `codex exec` (alias `e`) = non-interactive runner
prompt_delivery = "positional"      # positional [PROMPT] to `codex exec`
print_flag = ""                     # `exec` is already non-interactive; no extra token
model_flag = "-m"                   # `-m` / `--model <MODEL>` (confirmed both forms)
default_model = ""                  # codex reads model from ~/.codex/config.toml; user sets if overriding
system_prompt_flag = ""             # NO system-prompt flag exists → prepend to payload (§12.2)
provider_flag = ""
# Codex has no "disable all tools" switch. Tools are intrinsic to its agent loop.
# We constrain it to a safe, non-interactive, non-mutating profile instead:
#   -s read-only      → sandbox forbids writes / network mutations
#   -a never          → never block waiting for human approval (required for non-interactive)
bare_flags = ["--sandbox", "read-only", "--ask-for-approval", "never"]
output = "raw"
strip_code_fence = true
# TO CONFIRM (integration): that `codex exec` writes the assistant's final answer
# to stdout and exits 0 on success (expected; verify against a real run).
```

Rendered (model e.g. `gpt-5`):

```
codex exec -m gpt-5 --sandbox read-only --ask-for-approval never "<sys>\n\n<user payload>"
```

```toml
# Cursor Agent. Verified from `agent --help` (the Cursor Agent CLI).
name = "cursor"
detect = "agent"                    # the standalone binary is `agent`
command = "agent"                   # NOTE: some installs expose this as `cursor agent`
                                   # (the `agent [prompt...]` subcommand). If `agent`
                                   # is not on $PATH, set command="cursor" subcommand=["agent"].
subcommand = []
prompt_delivery = "positional"      # positional [prompt] to the agent
print_flag = "-p"                   # `-p` / `--print` = non-interactive (writes answer to stdout)
model_flag = "--model"              # e.g. gpt-5, sonnet-4-thinking; bracket overrides supported
default_model = ""                  # user sets; cursor has per-account model availability
system_prompt_flag = ""             # NO system-prompt flag exists → prepend to payload (§12.2)
provider_flag = ""
# Cursor's `-p` print mode defaults to FULL tool access ("all tools, including write
# and shell"). We override to a read-only Q&A profile so it cannot mutate the repo:
#   --mode ask   → "Q&A style, read-only" (no edits) — the right semantic for msg gen
#   --trust      → skip the workspace-trust prompt that would otherwise block `-p`
bare_flags = ["--mode", "ask", "--trust"]
output = "raw"                      # could use "json" with json_field from --output-format json
strip_code_fence = true
# We deliberately DO NOT set --force / --yolo (those force-allow commands).
# TO CONFIRM (integration): that `--mode ask` wins over `-p`'s default full-tools
# behavior — i.e. the combo is genuinely read-only. Expected, since `ask` is defined
# as read-only; verify against a real run.
```

Rendered (model e.g. `gpt-5`):

```
agent -p --mode ask --trust --model gpt-5 "<sys>\n\n<user payload>"
```

#### 12.7.1 The tools-disable asymmetry (important, documented honestly)

There is a real architectural split across our six providers in **how they become "bare"**:

- **Explicit tool-disable flags:** pi (`--no-tools`), Claude Code (`--tools ""`). These agents offer a literal "turn tools off" switch, so the call is a pure text-in/text-out with no agent loop. Fast and clean.
- **Read-only constraint instead:** Codex (`--sandbox read-only`), Cursor (`--mode ask`), Gemini (`--approval-mode default`). These agents have **no** global "disable all tools" switch — tools are intrinsic to their loop. We constrain them to a read-only, never-ask profile so they *cannot* mutate the repo or block on a prompt, but the model may still internally reason with tools.

Consequences, stated plainly:

1. **Safety is preserved either way.** Read-only sandbox/mode + never-ask means no provider in the default set can touch the working tree. The repo-integrity invariant (§18.1) holds for all six.
2. **Latency varies.** The read-only-constrained agents may be slightly slower (they run an agent loop the model can choose to use). Acceptable for a one-shot message.
3. **Output is still just the message.** Whichever path the model takes, the final assistant text is what Stagecoach parses; a model that "reads a file" before answering still ends with a commit message on stdout.

This is not a defect to paper over — it is the honest cost of supporting heterogeneous agent CLIs through one manifest schema. The `bare_flags` field exists precisely so each provider expresses "bare" in its own idiom.

#### 12.7.2 On stubbing and progressive verification

We do not pretend to know everything up front. The implementing agent will do its own comprehensive research per task. The contract here is: **the manifest *schema* and the six default manifests are fixed by this document; the exact behavior of each manifest is confirmed by a real end-to-end run during implementation.** Two explicit `# TO CONFIRM` notes are carried above (codex stdout-on-success; cursor `ask`-wins-over-`-p`). Any manifest field that cannot be confirmed is left at a safe default and marked with a `# TO CONFIRM` comment, never silently assumed. The `experimental` flag remains available (see §22.1) for any *future* provider added from docs alone rather than a verified `--help`.

### 12.8 Extensibility: user-defined providers

A user can define a provider unknown to Stagecoach by dropping a `[provider.<name>]` block into any config file. This is how community support for new agents (or future versions of existing ones) lands without a release:

```toml
# ~/.config/stagecoach/config.toml
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

Then `stagecoach --provider myagent` (or `stagecoach.provider = myagent` in git config) uses it. No recompilation.

### 12.9 Output parsing pipeline (detailed)

`parseOutput(raw string, m Manifest) (msg string, ok bool)`:

1. `s = strings.TrimSpace(raw)`.
2. If `m.strip_code_fence` and `s` starts with ```` ``` ```` or `~~~`: remove the first line (the fence opener, including any language tag) and everything from the last fence closer onward. Re-trim.
3. Switch on `m.output`:
   - **raw**: `msg = s`.
   - **json**: attempt `json.Unmarshal([]byte(s), &obj)`; if it fails, find the first `{` and the matching last `}` (brace-balanced substring) and retry. Extract `obj[m.json_field]` as a string. If anything fails, fall through to raw (treat `s` as the message) and set a "parse-fallback" flag for logging.
4. Normalize newlines: convert `\r\n` → `\n`; collapse 3+ consecutive newlines to 2.
5. `msg = strings.TrimSpace(msg)`. `ok = msg != ""`.

This pipeline is the reason the v1 commit-prompt uses a **raw** contract ("output only the commit message") rather than the JSON contract `commit-pi` used. JSON required the fragile `sed` extraction and the "no double quotes inside the message" constraint; raw + robust cleanup removes both. JSON remains available for agents (like Claude Code) where `--output-format json` is more reliable than raw stdout.

---

## 13. The snapshot-based generation flow (the core IP)

This section is the most important in the document. It is the thing Stagecoach does that no incumbent does, and it is the foundation v2 builds on.

### 13.1 Why `git commit` is the wrong primitive

`git commit` reads the **index at commit time**, packages it into a tree, and advances HEAD. This couples three things that should be decoupled:

1. *What gets committed* (the index contents at commit time).
2. *When the commit happens* (synchronously, right now).
3. *Whether the commit can fail safely* (a `git commit` that errors mid-way can leave the index and HEAD in surprising states, especially with hooks).

For an AI-commit tool, this coupling is actively harmful: the "what" was decided when the user staged files and we snapshotted, but the "when" is whenever the model finishes — potentially tens of seconds later, during which the user has every reason to keep staging. With `git commit`, the user must either sit idle (losing the overlap) or risk sweeping unintended files into the commit.

### 13.2 The plumbing alternative

Stagecoach never calls `git commit`. It uses three plumbing commands:

1. **`git write-tree`** — serializes the *current index* into a tree object and prints its SHA. Crucially, this **does not modify the index or HEAD**. It is a pure, read-only-with-respect-to-refs operation that freezes a copy of the staging area into the object store. After this call, `TREE_SHA` refers to a permanent, immutable record of "what was staged at time T", regardless of what the user does to the index afterward.

2. **`git commit-tree (-p <parent>) -m <msg> <tree>`** — creates a commit object with the given tree, parent, and message, and prints its SHA. This also **does not touch any ref**. The commit object exists in the object store but is "dangling" (unreferenced) until step 3. `PARENT_SHA` is captured *before* `write-tree` (actually before generation) for consistency.

3. **`git update-ref HEAD <new-sha> <expected-old-sha>`** — the two-argument (CAS) form atomically updates `HEAD` to `<new-sha>` **only if** its current value equals `<expected-old-sha>` (i.e., `PARENT_SHA`). If HEAD has moved in the meantime (the user committed in another terminal), the update fails cleanly and the repository is untouched.

### 13.3 The resulting workflow

Because the commit is built from `TREE_SHA` (frozen) and applied via CAS `update-ref`, the following all hold simultaneously:

- **The committed content is exactly what was staged when `write-tree` ran.** Files the user stages *after* that point are not in `TREE_SHA` and therefore not in the commit.
- **Those later-staged files remain staged.** The index is never reset by Stagecoach. After `update-ref`, HEAD's tree equals the snapshot (so the originally-staged files show as clean/committed), while the later-staged files are in the index but not in HEAD's tree — so `git status` shows them as "changes to be committed," ready for the next run.
- **The operation is atomic and safe.** If generation fails, we never reach `update-ref`; HEAD and the index are byte-for-byte unchanged. If `update-ref` fails (HEAD moved), same thing. The only artifacts left behind are a tree object and possibly a dangling commit object in the object store, which `git gc` will eventually reap (they are harmless).
- **Generation latency is overlap-able.** The user can `git add` the next batch in another pane during the blocking model call; the in-flight commit is unaffected.

### 13.4 Stage-while-generating: the user's mental model

```
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

This is the workflow the author already uses with `commit-pi` and that lazygit's `output: none` binding makes frictionless. v1 preserves it exactly; the implementation simply never touches the index between `write-tree` and `update-ref`.

### 13.5 Edge cases and their handling

- **Rootless repo (no commits yet):** `PARENT_SHA` is empty. `commit-tree` is called without `-p` (creates a root commit). `update-ref HEAD <new>` is called without the expected-old argument. Handled.
- **Unresolved merge conflicts in the index:** `write-tree` fails. Stagecoach aborts before any generation with "resolve merge conflicts first."
- **HEAD moved during generation (user committed elsewhere):** the CAS `update-ref` fails. Stagecoach prints: "HEAD moved from <PARENT> to <actual> while generating; aborting to avoid a non-fast-forward. Your generated message was: <msg>. To commit the snapshot manually: `git commit-tree -p <PARENT> -m \"<msg>\" <TREE> | xargs git update-ref HEAD`." Exit non-zero.
- **Generation timeout / SIGINT:** kill the agent, enter rescue path (print `TREE_SHA` + manual recovery).
- **Empty diff after auto-stage-all:** exit 2, "nothing to commit."
- **Agent not on `$PATH`:** `providers list` would have shown it as absent; on direct use, fail fast with "provider 'X' not found: is <command> installed?"

---

## 14. Package layout (Go)

```
stagecoach/
├── cmd/
│   └── stagecoach/
│       └── main.go                # entrypoint: arg parsing, wiring, exit codes
├── internal/
│   ├── config/
│   │   ├── config.go              # Config struct, Load(), precedence resolution
│   │   ├── defaults.go            # built-in defaults
│   │   ├── file.go                # TOML read (global + repo), git-config read
│   │   └── config_test.go
│   ├── provider/
│   │   ├── manifest.go            # Manifest struct, Render() → exec.Cmd spec
│   │   ├── builtin.go             # compiled-in manifests (pi, claude, gemini, ...)
│   │   ├── registry.go            # name → manifest, with override merge
│   │   ├── executor.go            # run manifest, feed stdin, capture stdout, timeout
│   │   ├── parse.go               # parseOutput() pipeline (§12.9)
│   │   └── *_test.go
│   ├── prompt/
│   │   ├── system.go              # buildSystemPrompt() (style learn, anti-reuse)
│   │   ├── examples.go            # fetch last 20, multi-line detection
│   │   ├── payload.go             # assemble user payload (instruction + diff)
│   │   └── *_test.go
│   ├── git/
│   │   ├── git.go                 # Git wrapper interface
│   │   ├── plumbing.go            # WriteTree, CommitTree, UpdateRefCAS, RevParseHEAD
│   │   ├── diff.go                # StagedDiff() with caps & exclusions
│   │   ├── log.go                 # RecentMessages(), RecentSubjects(), CommitCount()
│   │   ├── stage.go               # AddAll(), HasStagedChanges()
│   │   └── *_test.go              # uses a temp repo + real git binary
│   ├── generate/
│   │   ├── generate.go            # CommitStaged(ctx, cfg) — the core orchestrator
│   │   ├── dedupe.go              # duplicate-subject check + retry
│   │   ├── rescue.go              # rescue protocol (FR43–FR45)
│   │   └── *_test.go              # integration with a stub provider
│   └── ui/
│       ├── output.go              # progress messages, color, TTY detect
│       └── exitcode.go            # canonical exit codes
├── pkg/
│   └── stagecoach/
│       └── stagecoach.go           # PUBLIC API: GenerateCommit(ctx, opts) (Result, error)
│                                  # thin wrapper over internal/generate (for library use)
├── providers/                     # shipped reference manifests (TOML), human-readable
│   ├── pi.toml
│   ├── claude.toml
│   ├── gemini.toml
│   ├── opencode.toml
│   ├── codex.toml
│   └── cursor.toml
├── docs/
│   └── PRD.md                     # this document
├── .goreleaser.yaml
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### 14.1 The public library surface (`pkg/stagecoach`)

Intentionally tiny. The point is not to be a rich library; it is to let an integrator (a git GUI, a pre-commit hook, a CI step) call the core without reimplementing it.

```go
package stagecoach

type Options struct {
    Provider    string  // manifest name; "" → resolved default
    Model       string  // "" → manifest default_model
    SystemExtra string  // appended to the built system prompt
    DryRun      bool    // if true, return the message without committing
    Timeout     time.Duration
}

type Result struct {
    CommitSHA string    // empty if DryRun or not committed
    Subject   string
    Message   string    // full message (subject [+ body])
    Provider  string    // resolved provider name
    Model     string    // resolved model
}

// GenerateCommit generates and (unless DryRun) creates a commit from the
// currently-staged index. It does NOT decide what to stage. The caller
// stages first (or uses AutoStageAll in the CLI layer).
func GenerateCommit(ctx context.Context, opts Options) (Result, error)
```

The CLI's `main.go` is essentially: parse flags → maybe auto-stage → `stagecoach.GenerateCommit(ctx, opts)` → print result. This keeps the CLI a thin shell over the library and guarantees v2 can reuse `GenerateCommit` in a loop.

---

## 15. CLI reference

### 15.1 Synopsis

```
stagecoach [flags]
stagecoach <command> [flags]
```

With no command, runs the default action: commit staged changes (auto-staging all if nothing is staged and `auto_stage_all` is on).

### 15.2 Global flags

| Flag | Env | Git config | Default | Description |
|---|---|---|---|---|
| `--provider <name>` | `STAGECOACH_PROVIDER` | `stagecoach.provider` | auto-detected | Provider/agent to use. |
| `--model <name>` | `STAGECOACH_MODEL` | `stagecoach.model` | per-manifest `default_model` | Model override. |
| `--config <path>` | `STAGECOACH_CONFIG` | — | resolved path | Path to a config file (overrides discovery). |
| `--timeout <dur>` | `STAGECOACH_TIMEOUT` | `stagecoach.timeout` | `120s` | Generation timeout. |
| `--all`, `-a` | — | — | — | `git add -A` before snapshotting, even if something is staged. |
| `--no-auto-stage` | — | — | — | If nothing is staged, exit instead of auto-staging. |
| `--dry-run` | — | — | `false` | Generate and print the message; do not commit. |
| `--verbose`, `-v` | `STAGECOACH_VERBOSE` | — | `false` | Print resolved command, raw output, retries. |
| `--no-color` | `STAGECOACH_NO_COLOR` | — | TTY-aware | Disable color. Respects `NO_COLOR`. |
| `--version` | — | — | — | Print version and exit. |
| `--help`, `-h` | — | — | — | Help. |

### 15.3 Subcommands

- **`stagecoach providers list`** — List all known providers (built-in + user). Mark detected (on `$PATH`) vs not. Show the resolved default.
- **`stagecoach providers show <name>`** — Print the fully-resolved manifest as TOML.
- **`stagecoach config init`** — Write a commented example config to the global path.
- **`stagecoach config path`** — Print the resolved global config path.

### 15.4 Exit codes

| Code | Meaning |
|---|---|
| `0` | Success (commit created, or dry-run message printed). |
| `1` | General error (generation failed, parse failed after retries, agent missing, etc.). |
| `2` | Nothing to commit (clean tree after auto-stage, or nothing staged with `--no-auto-stage`). |
| `3` | Rescue condition (snapshot taken, commit not created — manual recovery printed). |
| `124` | Timeout (generation exceeded `--timeout`). |

### 15.5 Example invocations

```bash
# Default: commit staged changes with the default provider.
stagecoach

# Use a specific agent + model for one commit.
stagecoach --provider claude --model sonnet

# Set a per-repo default (persisted in the repo's git config).
git config stagecoach.provider pi
git config stagecoach.model glm-5.2

# Dry run: see what it would write, commit nothing.
stagecoach --dry-run

# Quick checkpoint: stage everything and commit in one shot.
stagecoach -a

# From lazygit config.yml:
#   customCommands:
#     - key: '<c-a>'
#       command: 'stagecoach'
#       loadingText: 'Generating commit message…'
#       output: 'none'

# Pipe the generated message elsewhere (dry-run, stdout = message only).
stagecoach --dry-run --no-color | tee /tmp/msg.txt
```

---

## 16. Configuration model (full)

### 16.1 Resolution order (FR34), lowest to highest

1. **Built-in defaults** (`internal/config/defaults.go`): timeout 120s, auto_stage_all true, max_diff_bytes 300000, max_md_lines 100, max_duplicate_retries 3, output raw, strip_code_fence true.
2. **Built-in provider defaults** (`internal/provider/builtin.go`): the manifests in §12.3–12.7.
3. **Global config file** (`$XDG_CONFIG_HOME/stagecoach/config.toml`, default `~/.config/stagecoach/config.toml`).
4. **Per-repo config file** (`./.stagecoach.toml`, if present; not committed by default — added to a generated `.gitignore` only on `config init` if the user confirms).
5. **Per-repo git config** (`stagecoach.*` keys; read via `git config --get`).
6. **Environment variables** (`STAGECOACH_*`).
7. **CLI flags.**

Higher wins. Provider manifests merge field-by-field (a user override that sets only `default_model` leaves all other fields from the built-in manifest intact).

### 16.2 Full config file example

```toml
# ~/.config/stagecoach/config.toml

[defaults]
provider = "pi"            # default agent
model   = ""               # "" → use the manifest's default_model
timeout = "120s"
auto_stage_all = true
verbose = false

[generation]
max_diff_bytes      = 300000
max_md_lines        = 100
max_duplicate_retries = 3
output              = "raw"     # raw | json
strip_code_fence    = true
subject_target_chars = 50

# Override a built-in provider (field-merged with the built-in manifest).
[provider.pi]
default_model = "glm-5.2"
default_provider = "zai"

# Define a brand-new provider (§12.8).
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

### 16.3 Git-config keys (alternative to a file)

For users who prefer to keep config with the repo and don't want a `.stagecoach.toml`:

```ini
[stagecoach]
    provider = pi
    model = glm-5.2
    timeout = 90
    autoStageAll = true
```

Read with `git config --get stagecoach.provider`, etc. Booleans via `git config --bool`. This composes naturally with the author's existing `git commit-pi` alias habit and with `git config --local` vs `--global`.

---

## 17. Prompt engineering

### 17.1 The system prompt (mature repo, >1 commit)

Ported and refined from `commit-pi`. The structure:

```
You are a commit message generator.

Output ONLY the commit message. No preamble, no markdown, no code fences,
no quoting. If a body is warranted, use a blank line between subject and body.

Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.

Match the tone and style of these recent commits from this repository:
---
<commit 1 full message>
---
<commit 2 full message>
...
(up to 20, ≤100 lines total)

CRITICAL: You MUST NOT copy or reuse ANY phrasing from the examples above.
They show the STYLE to match — format, tone, length, conventions. Producing
the same text you have seen is STRICTLY FORBIDDEN. Your output must be
entirely original wording describing THIS specific change. Reusing example
text is a critical failure.

<multi-line rule>
Target ~50 characters for the subject line.
```

Where `<multi-line rule>` is one of:
- If history has multi-line commits: *"Only add a body (blank line + description) if the history shows multi-line commits AND these changes truly warrant detailed explanation. Otherwise, use a single-line subject only."*
- Else: *"Only output a single-line subject (no body)."*

### 17.2 The system prompt (new repo, ≤1 commit)

```
You are a commit message generator.

Output ONLY the commit message. No preamble, no markdown, no code fences.

Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.

Target ~50 characters (~7 words). Format: type(scope): description
```

### 17.3 The user payload

Delivered via stdin (or positional/flag per manifest). Structure:

```
Generate a commit message for these changes:

<diff payload (markdown section, then other-files section)>
```

On a duplicate-rejection retry, a rejection block is inserted after the instruction:

```
Generate a commit message for these changes.

IMPORTANT: The following messages were REJECTED because they already exist
in git history. You MUST generate something COMPLETELY DIFFERENT:
- <rejected subject 1>
- <rejected subject 2>

Create an entirely new message with different wording.

<diff payload>
```

### 17.4 Why raw output, not JSON (the v1 design call)

`commit-pi` used `{"commit_message": "..."}` and parsed it with `sed`. This required (a) telling the model never to use double quotes inside the message (a real constraint that produced awkward messages), and (b) a fragile regex. Go's `json.Unmarshal` removes (b), but (a) remains — JSON string escaping is a footgun for free-form prose, and models frequently emit invalid JSON when the message contains quotes or newlines.

Raw output ("output only the message") is more robust for this use case: there is nothing to escape, nothing to parse structurally, and the robust cleanup pipeline (§12.9) handles the rare case of a model wrapping output in a code fence. The only failure mode raw introduces is "the model added a preamble sentence," which the retry instruction corrects, and which is strictly less common than JSON-parse failures.

JSON mode remains available (`output = "json"`, `json_field = "result"`) for agents like Claude Code whose `--output-format json` is specifically designed to be machine-parsed and may be more reliable than raw for certain models. The default is raw; the option exists.

---

## 18. Error handling, rescue protocol, and safety

### 18.1 The invariant

**The repository's refs and index are modified only at the final `update-ref` step, and only if HEAD is unchanged since the snapshot.** Every code path that does not reach a successful `update-ref` leaves the repository byte-for-byte unchanged (modulo harmless dangling objects).

### 18.2 Failure modes and responses

| Failure | When | Response | Exit |
|---|---|---|---|
| Nothing staged, `--no-auto-stage` | pre-snapshot | "Nothing staged." | 2 |
| Nothing staged, auto-stage on, but clean tree | pre-snapshot | "Nothing to commit." | 2 |
| Merge conflicts in index | `write-tree` | "Resolve merge conflicts first." | 1 |
| Agent missing on `$PATH` | pre-generation | "Provider 'X': command 'Y' not found. Is the agent installed?" | 1 |
| Generation timeout | executor | kill agent, rescue | 124/3 |
| SIGINT/SIGTERM | any time post-snapshot | kill agent, rescue | 3 |
| Empty/invalid output after retries | parse | rescue | 3 |
| Duplicate after all retries | dedupe | rescue | 3 |
| `update-ref` CAS failure (HEAD moved) | commit | print message + manual recovery (do NOT force) | 1 |

### 18.3 The rescue message (FR43–FR45)

When `TREE_SHA` is set and `NEW_SHA` is not:

```
❌ Commit generation failed.
------------------------------------------------------------
Your staged files were safely snapshotted before generation.
Tree ID: <TREE_SHA>

To commit the originally staged files manually:
  git commit-tree -p <PARENT_SHA> -m "Your message" <TREE_SHA> | xargs git update-ref HEAD

(omit "-p <PARENT_SHA>" if this is the repository's first commit)
------------------------------------------------------------
```

If the failure was a duplicate-exhaustion or parse failure *with a candidate message in hand*, additionally print: *"A candidate message was produced but rejected: \"<msg>\". You can use it manually in the command above."* — so the user's wait wasn't wasted.

### 18.4 Signal handling

Stagecoach installs a `signal.Notify` handler for SIGINT and SIGTERM. On receipt:
1. If a child process (agent) is running, send it SIGTERM (then SIGKILL after a grace period) via its process group (`SysProcAttr.Setpgid = true` so we can kill the whole tree).
2. If the snapshot has been taken, run the rescue path; else just exit.
3. Restore the default signal handler before the final `update-ref` so a Ctrl-C at the very last instant isn't mistaken for a failure (matching `commit-pi`'s `trap - INT TERM` before commit).

---

## 19. Security considerations

- **No shell interpolation.** Commands are built as `[]string` and run via `exec.Command` directly, never via `sh -c` / `zsh -c`. The diff payload is delivered via stdin, never interpolated into an argument. This eliminates the entire class of shell-injection bugs that a naive port could introduce. (The original `commit-pi` ran under `zsh -c` because of the git-alias mechanism; Stagecoach execs directly and is safer.)
- **No secret handling.** Stagecoach never reads, logs, or transmits the agent's credentials. The agent owns its own auth; Stagecoach only spawns it with the inherited environment (plus any manifest-declared `[env]` additions). Logs in `--verbose` print the command and flags but never stdin contents unless `STAGECOACH_VERBOSE=2`.
- **Diff content is local.** The diff never leaves the machine except via the user's own agent over the user's own authenticated channel. Stagecoach makes no network calls itself.
- **Config file trust.** Config files are user-owned (`~/.config` and repo-local). A repo-local `.stagecoach.toml` could be committed by an attacker to change a user's provider — but it can only redirect commit generation to another *installed* agent the user already trusts; it cannot exfiltrate credentials or run arbitrary commands (manifests specify a `command` + flags, not arbitrary shell). Still, Stagecoach will print a one-line notice when a repo-local config is loaded that overrides the provider, so the redirection is visible. (Hardening for v1.1: restrict repo-local configs to non-`command` fields unless `STAGECOACH_TRUST_REPO_CONFIG=1`.)
- **`--dangerously-*` flags never auto-set.** Stagecoach will not pass `--dangerously-skip-permissions` or equivalent to any agent. Bare mode means "no tools, no session, no chrome" — not "skip safety checks." For agents where disabling tools requires an empty allowlist (Claude's `--tools ""`), we use that; we never use the bypass-permissions flags.

---

## 20. Testing and QA strategy

### 20.1 Layers

1. **Unit — pure functions.** `parseOutput` (table-driven: raw, fenced, json, json-in-prose, fallback), command rendering (per provider, golden files), prompt construction (style-learning, multi-line detection, anti-reuse text), duplicate detection, config precedence resolution.
2. **Unit — git wrapper, with a real git binary.** Each `internal/git/*` test creates a temp directory, `git init`, stages known content, and asserts on `WriteTree`/`CommitTree`/`UpdateRefCAS`/`StagedDiff`/`RecentMessages`. These are fast (git is fast) and catch real plumbing regressions.
3. **Integration — full flow with a stub provider.** A fake agent: a tiny Go binary (or shell script) that reads stdin and writes a canned message to stdout. Drives `generate.CommitStaged` end-to-end and asserts the resulting commit exists in the repo with the right tree, parent, and message. Covers: success, duplicate-retry-then-success, parse-failure-then-rescue, timeout, CAS failure (simulate by moving HEAD mid-test), root commit, auto-stage-all.
4. **Integration — real agents (opt-in, not in CI).** A `//go:build integration_real` suite that invokes the actual `pi`/`claude`/etc. if installed and `STAGECOACH_RUN_REAL=1`. Used manually before releases; skipped in CI.

### 20.2 Property/invariant tests

- **Idempotent index:** after any failure path, `git status` output is identical to before the run (no index mutation). Asserted by snapshotting `git diff --cached --name-only` before and after.
- **Atomic HEAD:** after a CAS failure, `git rev-parse HEAD` is unchanged.
- **Snapshot immutability:** `git cat-file -p <TREE_SHA>` is stable across the run regardless of subsequent staging.

### 20.3 Coverage target

≥85% on `internal/git`, `internal/provider`, `internal/generate`, `internal/config`. Lower bar for `internal/ui` (hard to test, low risk). Enforced in CI with a coverage gate.

### 20.4 CI matrix

GitHub Actions: build + test on `{linux, macos, windows} × {amd64, arm64}`, Go `1.22` and `1.23`. `golangci-lint`. `govulncheck`. Release on tag via goreleaser.

---

## 21. Distribution and release

### 21.1 Build

Go modules. `make build` → `./bin/stagecoach`. `make test`, `make lint`, `make coverage`. Version injected via `-ldflags "-X main.version=…"` at release.

### 21.2 goreleaser

`.goreleaser.yaml` produces:
- Archives + standalone binaries for `linux/darwin/windows × amd64/arm64`.
- Homebrew formula to a tap repo (`dustin/homebrew-tap`).
- AUR `stagecoach` + `stagecoach-bin` (via a maintained PKGBUILD; possibly community).
- Scoop manifest (Windows) to a bucket.
- Checksums + a changelog.
- `go install github.com/dustin/stagecoach/cmd/stagecoach@latest` works from the tagged commit.

### 21.3 Install paths

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

### 21.4 Versioning

Semantic versioning. v1.0.0 = feature-complete against this PRD's P0/P1 set. Provider-manifest additions (new agents) are minor bumps if built-in, patch bumps if docs-only. Breaking changes to the manifest schema or public `pkg/stagecoach` API are major bumps.

### 21.5 README structure (the marketing surface)

1. Hero: the one-sentence pitch (§5).
2. The 30-second demo (asciinema/gif).
3. "Why not opencommit/aicommits?" — the coding-plan paragraph, in 3 sentences.
4. Install (the four paths above).
5. Quick start (one `stagecoach` invocation).
6. Configure your agent (`providers list` → set `stagecoach.provider`).
7. The snapshot workflow (§13.4 diagram) — the "stage while it thinks" payoff.
8. Full CLI + config reference (link to docs).
9. Adding a new agent (§12.8) — the contributor hook.
10. FAQ / "Stagecoach is not for you if…"

---

## 22. Risks, assumptions, dependencies

### 22.1 Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Agent CLI surfaces change (flags renamed/removed). | Medium | Medium (a provider breaks). | Manifests are config-overridable without a release; `providers show` aids debugging; pin known-good manifest versions in docs; community can ship fixes. |
| An agent's raw output is unreliable (preambles, fences). | Medium | Low (retry handles it). | Robust parse pipeline + retry; JSON fallback per-provider. |
| Large diffs exceed an agent's context or arg limits. | Low | Medium. | Diff cap (300 KB default, configurable); stdin delivery avoids arg limits; surface a clear "diff truncated" notice. |
| `update-ref` CAS semantics misunderstood. | Low | High (data integrity). | Exhaustive tests (§20.2); never use force-update; rescue on failure. |
| Users expect multi-commit in v1. | Medium | Low (disappointment). | README states v1 = single commit clearly; roadmap links to v2. |
| Agent invokes tools despite bare flags (e.g., a model "reads" a file). | Low | Low (slower, maybe wrong message). | Bare flags disable tools; output is still just a message; worst case is a slightly slower or odd commit, never repo damage. |
| Codex / Cursor manifests drift or the two `# TO CONFIRM` items fail (§12.7). | Low–Medium | Low (safe read-only profile either way). | Flag surfaces verified against real `--help`; two residual checks carried inline. `experimental` flag remains available for any future docs-only provider; manifests are config-overridable without a release. |

### 22.2 Assumptions

- The user has at least one supported coding-agent CLI installed and authenticated. (Anti-persona §7.4 is explicitly unsupported.)
- Git ≥ 2.20 (for `write-tree`/`commit-tree`/`update-ref` CAS semantics — all ancient, but we state a floor).
- A POSIX-ish environment for the curl|sh installer; Homebrew/Scoop/Go-install paths cover the rest.
- The agent's non-interactive mode writes the answer to stdout and exits non-zero on failure (true for pi, claude, gemini, opencode per their `--help`).

### 22.3 Dependencies

- **Go 1.22+** (stdlib `os/exec`, `os/signal`, `encoding/json`, `flag` or cobra).
- **cobra** (recommended) for CLI/subcommands, or `urfave/cli/v3`; or bare `flag` to minimize deps. (Recommendation: cobra, for `providers`/`config` subcommands and familiar UX.)
- **pelletier/go-toml/v2** for config parsing.
- **No git library dependency.** Stagecoach shells out to the real `git` binary (matching `commit-pi`). go-git is tempting but adds a large dependency and re-implements plumbing we trust the real binary for. Shelling out is simpler, matches the reference implementation, and guarantees identical semantics to the user's git.

---

## 23. Glossary

- **Coding plan** — a flat-fee subscription (Claude Max, Codex Pro, Gemini Advanced) whose usage is billed against the CLI product via proprietary OAuth, not the public API. Stagecoach's reason to exist.
- **Manifest** — a TOML description of how to invoke one agent (§12.1).
- **Provider** — a named manifest; roughly synonymous with "agent" in the UI.
- **Snapshot** — the tree object produced by `git write-tree`, freezing the index at a point in time.
- **CAS update-ref** — `git update-ref HEAD <new> <expected-old>`; updates HEAD only if unchanged since expected-old. Stagecoach's atomicity primitive.
- **Rescue** — the protocol triggered when generation fails after the snapshot (§18.3): print the tree SHA and manual recovery command.
- **Bare mode** — invoking an agent with no tools, no session, no extensions/skills/chrome, for a pure ephemeral text generation.
- **Stage-while-generating** — the workflow property (§13.4) whereby the user can stage the next batch during the current generation without affecting the in-flight commit.

---

# Appendices

## Appendix A — Full v1 system prompt templates

(See §17. These are the canonical strings to be committed verbatim to `internal/prompt/system.go` as Go string constants, with the diff/examples/rejection-list interpolated at runtime.)

## Appendix B — Example terminal sessions

### B.1 Happy path

```
$ git add src/login.go src/login_test.go
$ stagecoach
↳ Snapshotting 2 staged files…  (tree 9f3a1c…)
↳ Generating with pi (glm-5.2)…
↳ Created abc1234  feat(auth): accept SAML tokens for enterprise login
   M  src/login.go
   A  src/login_test.go
```

### B.2 Stage-while-generating

```
# pane A
$ git add src/a.go && stagecoach
↳ Generating with claude (sonnet)…   (takes 8s)

# pane B, during those 8s
$ git add src/b.go src/c.go          # these are NOT in the commit below

# pane A resumes
↳ Created def5678  refactor: extract auth helper
# git status now shows src/b.go, src/c.go as staged-for-next-commit
```

### B.3 Dry run

```
$ stagecoach --dry-run
↳ Generating with pi (glm-5.2)…
feat(auth): accept SAML tokens for enterprise login

(no commit created)
```

### B.4 Duplicate retry

```
$ stagecoach -v
↳ Attempt 1: subject "fix: handle null user" matches an existing commit — retrying.
↳ Attempt 2: "fix: guard against missing user record" — accepted.
↳ Created ghi9012  fix: guard against missing user record
```

### B.5 Rescue

```
$ stagecoach
↳ Generating with gemini (gemini-2.5-pro)…
^C
❌ Commit generation failed (interrupted).
------------------------------------------------------------
Your staged files were safely snapshotted before generation.
Tree ID: 9f3a1c...

To commit the originally staged files manually:
  git commit-tree -p abc1234 -m "Your message" 9f3a1c... | xargs git update-ref HEAD
------------------------------------------------------------
```

## Appendix C — Line-by-line porting map from `commit-pi`

| `commit-pi` section | Stagecoach location | Notes |
|---|---|---|
| `handle_error()` rescue | `internal/generate/rescue.go` | Identical message; richer (includes candidate message on dedupe fail). |
| `trap 'handle_error' INT TERM` | `main.go` signal handler (§18.4) | Process-group kill of child; rescue if snapshot taken. |
| staged-diff capture (md + other) | `internal/git/diff.go` `StagedDiff()` | Caps/exclusions identical, configurable. |
| `PARENT_SHA=$(git rev-parse HEAD)` | `git.RevParseHEAD()` | Empty allowed (root repo). |
| `TREE_SHA=$(git write-tree)` | `git.WriteTree()` | Abort on conflict-in-index. |
| commit_count / examples / multi-line detect | `internal/prompt/examples.go` | Same heuristics, Go. |
| system_prompt construction | `internal/prompt/system.go` | Raw-output contract (not JSON) per §17.4. |
| `pi --no-tools … -p` invocation | `internal/provider` + manifest | Manifest-driven; pi manifest reproduces this exactly (§12.3). |
| JSON sed parse | `internal/provider/parse.go` | Replaced by raw + robust pipeline (§12.9). JSON still available. |
| duplicate-retry loop | `internal/generate/dedupe.go` | Last 50 subjects; up to 3 retries; rejection list appended. |
| `commit-tree` + `update-ref` | `git.CommitTree` / `git.UpdateRefCAS` | CAS form preserved. |
| `git diff-tree --name-status` success print | `main.go` | Identical UX. |
| `trap - INT TERM` before commit | signal handler restore | Same intent. |

## Appendix D — Built-in manifest quick reference

| Provider | command | delivery | print | model flag | sys-prompt flag | bare essentials | output |
|---|---|---|---|---|---|---|---|
| pi | `pi` | stdin | `-p` | `--model` | `--system-prompt` | `--no-tools --no-extensions --no-skills --no-prompt-templates --no-context-files --no-session` | raw |
| claude | `claude` | stdin | `-p` | `--model` | `--system-prompt` | `--tools "" --setting-sources "" --no-session-persistence` | raw (json optional) |
| gemini | `gemini` | positional/stdin | (positional) | `-m` | *(prepend)* | `--approval-mode default` | raw |
| opencode | `opencode run` | positional | — | `-m` (`provider/model`) | *(prepend)* | — | raw |
| codex | `codex exec` | positional | (exec) | `-m` | *(prepend)* | `--sandbox read-only --ask-for-approval never` | raw |
| cursor | `agent` | positional | `-p` | `--model` | *(prepend)* | `--mode ask --trust` | raw |

## Appendix E — Open questions (to resolve before/during v1 implementation)

1. **Gemini delivery:** confirm `stdin` accepts a ~300 KB payload without truncation; if not, fall back to positional and document the diff cap as the mitigation.
2. **Claude tools-disable:** confirm `--tools ""` fully suppresses tool use in `-p` mode for current Claude Code versions; if a model still "thinks" about tools, add `--disallowed-tools "*"` (verify syntax).
3. **opencode system prompt:** decide whether to (a) prepend to payload (simple, v1), or (b) document an `--agent` workflow where users define a `stagecoach` agent persona in `opencode.json` (nicer, v1.1).
4. **Codex / Cursor (mostly resolved):** flag surfaces verified against real `--help` (§12.7). Two residual confirmations carried as inline `# TO CONFIRM`: (a) `codex exec` writes the final answer to stdout and exits 0; (b) cursor `--mode ask` wins over `-p`'s default full-tools profile. Both are expected from the docs and are quick to confirm during the first real run.
5. **`.stagecoach.toml` trust:** finalize the v1.1 hardening (restrict repo-local overrides to non-`command` fields unless explicitly trusted).
6. **Public API stability:** decide whether `pkg/stagecoach.GenerateCommit` is v1-stable or marked experimental until v1.1. Recommendation: ship it, mark it `// Stable as of v1.0`, keep the `Options` struct additive-only.

## Appendix F — Decision log (key calls and why)

- **Shell out to agents, not call APIs.** Because coding-plan quotas are unreachable over the public API. This is the product. (§2.2, §4.3)
- **Go, not TS.** Distribution fit (Homebrew/binary) matches the lazygit/gh audience; zero runtime dependency. (§2.3)
- **Raw output default, not JSON.** Removes the double-quote constraint and fragile parsing; JSON remains an option per-provider. (§17.4)
- **Shells out to real `git`, no go-git.** Matches the proven reference; identical semantics; smaller dependency surface. (§22.3)
- **v1 = single commit; multi-commit is v2.** Keeps v1 shippable; the snapshot foundation makes v2 a loop over v1. (§10.1, §11.3)
- **Auto-stage-all on by default in v1.** Per author's explicit request; the quickest path to a checkpoint commit; `--no-auto-stage` escapes it. (§9.4, FR16–FR20)
- **Manifests are config-overridable, compiled-in as defaults.** Decouples "support a new agent" from "cut a release." (§12.1, §12.8)

---

*End of document. Target length: comprehensive PRD + technical specification exceeding 20,000 tokens, covering product positioning, competitive analysis, functional requirements, architecture, the provider-manifest system, the snapshot-based atomic-commit core, CLI/config reference, prompt engineering, error/rescue design, security, testing, distribution, risks, and appendices including a porting map from the originating `commit-pi` script.*
