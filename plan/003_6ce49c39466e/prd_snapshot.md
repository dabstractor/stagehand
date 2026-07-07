# Stagecoach — Product Requirements & Technical Specification

| Field | Value |
|---|---|
| **Project name** | stagecoach |
| **Language / runtime** | Go (single static binary) |
| **Status** | v2.0 specification (draft) |
| **Author** | dustin |
| **Last updated** | 2026-07-01 |
| **Origin** | `commit-pi` / `commit-claude` (zsh), `~/projects/git-scripts` |
| **Document purpose** | Comprehensive PRD + technical specification. Defines the product surface, architecture, provider model, configuration model, git plumbing, testing, and distribution. Supersedes ad-hoc design discussion. |
| **This revision** | Promotes **multi-commit decomposition** from the deferred v2 roadmap into the core spec (§13.6), adds **per-role agent/model configuration** (§16.4), adds **binary/non-text filtering** (§9.1), adds the **Antigravity CLI (`agy`)** provider (§12.5.1), adds a **cascading provider priority + tier-based default models** that are **decoupled from the author's z.ai subscription** (§9.16), and adds a **populated config bootstrap + schema versioning** (§9.17). The v1 single-commit core (§9, §13.1–§13.5) is implemented. **[config v3]** Folds the inference provider into the model string: `provider` is the agent platform (its original meaning), and multi-backend providers (pi, opencode) take the model as `inference/model` (e.g. `zai/glm-5.2`); pi splits the prefix into its `--provider` flag at render. No separate inference-provider field exists — the prefix IS the field (§12, §9.15, FR-R5b, FR-B7). |

---

## 0. How to read this document

This is two documents fused into one:

- **Part I — Product Requirements (PRD):** sections 1–10. What we are building, for whom, why it is different, and what is explicitly out of scope. Read this to understand the *why* and the *what*.
- **Part II — Technical Specification:** sections 11–22. How it is built: package layout, provider manifest schema, git internals, CLI reference, config model, testing, release. Read this to implement it.

Appendices (A–F) contain reference material: full prompt templates, example terminal sessions, and a line-by-line porting map from `commit-pi`.

The single most important section for understanding the product's defensibility is **§5 (Unique Value Proposition)** and the single most important section for understanding the engineering is **§13 (The snapshot-based generation flow)**. If you read only two sections, read those. The most important section for understanding the **multi-commit** feature — the headline addition in this revision — is **§13.6 (Multi-commit decomposition)**; read it alongside §13.1–§13.5, which it composes.

---

# PART I — PRODUCT REQUIREMENTS

## 1. Executive summary

**Stagecoach** is a command-line tool that writes your git commit messages for you using the AI coding agent you already have installed and already pay for.

It is **not** another "API key in your config file, pay-per-token" commit generator. The incumbent tools in this space — `opencommit`, `aicommits`, `cz-git` — all talk directly to provider APIs (OpenAI, Anthropic, local Ollama) and require you to provision an API key and spend tokens against your API billing. Stagecoach refuses to do that. Instead, it shells out to whatever coding-agent CLI you already run — **Claude Code, Codex, Gemini CLI, pi, opencode, Cursor CLI** — and lets the generation count against the **coding-plan quota** you have already paid for (Claude Max, Codex Pro/Plus, Gemini Advanced, a self-hosted pi/opencode setup, etc.).

This is a structural gap the incumbents cannot close without ceasing to be what they are. The coding-plan subscriptions are deliberately billed against the *CLI product* via proprietary OAuth flows, not the public API. There is no API key that draws down a Claude Max allotment. Stagecoach is built entirely around that fact.

Stagecoach also brings three workflow properties that no incumbent offers:

1. **Snapshot-based atomic commits.** Before generation begins, Stagecoach snapshots the staged index into the git object store with `git write-tree`. The eventual commit is created with plumbing (`commit-tree` + `update-ref`) against that frozen snapshot, never against the live index. A failed or slow generation can therefore never corrupt your repository, and — critically — **you can keep staging the *next* batch of files while the *current* message is still being generated.** Those newly staged files are not swept into the in-flight commit; they stay staged, ready for the next run. `git commit` cannot do this, and neither can any tool that ends with `git commit`.
2. **Style learning with anti-duplicate guarantee.** Stagecoach reads the last 20 commit messages in the repository, detects whether the project uses single-line subjects or multi-line subjects-with-bodies, instructs the model to match the *style* while forbidding it from reusing the *wording*, and runs a post-generation check that rejects any subject that already exists in the last 50 commits, retrying with an explicit "this was already used, write something different" instruction.
3. **Multi-commit decomposition (new).** When you run `stagecoach` with a dirty working tree and *nothing* staged, it does not blindly squash everything into one commit. It sends a snapshot of all the changes to a reasoning agent that decides — or is told, via `--commits N` — how many commits the changes warrant and what belongs in each, then stages and commits each logical unit in sequence, reusing the snapshot machinery so staging the next group overlaps generating the current message. Any leftovers are reconciled by a final arbiter pass that amends the correct just-made commit (or makes a new one). One large batch becomes a clean, reviewable history with no extra typing.

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
| **Stagecoach** | Go | **None — uses your installed CLI agent** | **Yes** | **Yes** (auto-decompose when nothing is staged, or force N via `--commits`; §13.6) | Yes (last 20, anti-duplicate) | **Yes** (`write-tree` + `commit-tree` + `update-ref`) | **Yes** |

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
6. **Right model for the right job (new).** Commit-message generation, decomposition planning, per-concept staging, and leftover reconciliation are different tasks, so they can be bound to different agents and models — a large-context model for planning, a fast model for messages. One global model still covers everything if you don't care to tune.

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
- **G11.** Implement **multi-commit decomposition** (§13.6): when nothing is staged, decide (or accept a forced `--commits N`) how many commits the working-tree changes warrant, stage and commit each logical unit in sequence via the snapshot machinery with overlapped staging/generation, and reconcile leftovers via an arbiter pass. Provide `--single`/`--no-decompose` as an escape hatch to the one-commit behavior.
- **G12.** Implement **per-role provider/model configuration** (§16.4): a global default that applies to all agent roles, overridable per role (planner / stager / message / arbiter).
- **G13.** **Filter binary and other non-text filetypes** out of every diff payload sent to an agent, replacing each with a filename + change-status placeholder (§9.1).
- **G14.** Ship a built-in manifest for **Antigravity CLI (`agy`)** — Google's Gemini-CLI successor, whose coding-plan quota is reachable only through `agy` (§12.5.1).

### 6.2 Non-goals (v1 — explicitly deferred)

- **N1.** Multi-commit decomposition on a **pre-staged index**. Decomposition activates only when nothing is staged (§13.6.1). If the user has already staged files, the single-commit primitive (§13.1–§13.5) runs unchanged — stagecoach never re-partitions a hand-staged index. (Decomposition itself, formerly deferred, is now in scope; see §13.6.)
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

A developer who runs several agents (pi for open models, Claude Code for hard problems, Gemini for long context) and wants to choose per-repository or per-invocation which one writes commits. The `--provider` flag and per-repo git-config key (`stagecoach.provider`) are for this user.

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
- **US13.** As a plan-holder with a large, mixed working tree and nothing staged, I want `stagecoach` to split my changes into a sequence of logically-coherent commits automatically, so that my history stays clean without manual `git add -p` archaeology.
- **US14.** As a plan-holder who already knows the answer, I want `stagecoach --commits 3` to skip the "how many?" decision and go straight to partitioning into three commits, so that I save a round-trip and stay in control of the count.
- **US15.** As a power user, I want to route decomposition planning to a large-context model and commit-message generation to a fast model, so that each task uses the agent best suited to it — while still being able to set one model for everything.
- **US16.** As a plan-holder, I want any changes the staging agents failed to claim to be reconciled automatically (folded into the correct just-made commit, or a new one) rather than left dangling, so that `git status` is clean after a decompose run.
- **US17.** As a Gemini / Antigravity subscriber, I want to point stagecoach at `agy` so that commit generation spends my Antigravity coding-plan quota, which is unreachable over the public API.

---

## 9. Functional requirements

Each requirement has an ID (FR-n), a priority (P0 = must for v1, P1 = should for v1, P2 = nice for v1), and a mapping to the goals in §6.1.

### 9.1 Diff capture (P0, → G1, G3)

- **FR1.** Capture the staged diff via `git diff --cached`.
- **FR2.** Markdown files (`.md`, `.markdown`): include full diff capped at N lines per file (default 100, configurable via `max_md_lines`).
- **FR3.** Non-markdown files: include diff with pathspec exclusions for lock files (`*.lock`, `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`), snapshots (`*.snap`), sourcemaps (`*.map`), and vendored code (`vendor/*`), capped at N bytes total (default 300,000, configurable via `max_diff_bytes`).
- **FR3a.** Detect **non-text (binary) files** and exclude their bodies from the payload. Primary signal: `git diff --cached --numstat`, which emits `-\t-\t<path>` for any file git classifies as binary (content sniffing catches images, compiled binaries, archives, fonts, media). Supplemented by an **extension denylist** (`png jpg jpeg gif webp bmp ico svgz pdf zip tar gz tgz bz2 7z rar exe dll so dylib o class jar war wasm a mp3 mp4 mov avi mkv flac ogg wav ttf otf woff woff2`), overridable via `binary_extensions`. (Git's numstat detection is authoritative where it fires; the denylist covers files git may misclassify.)
- **FR3b.** For each excluded non-text file, emit a **one-line placeholder that preserves the filename and a description of the change**, instead of the useless `Binary files a/… and b/… differ` hunk: `"<status>\t[binary] <path>"` (e.g. `M\t[binary] assets/logo.png`, `A\t[binary] public/trailer.mp4`). The `<status>` (A/M/D/R/T) is sourced from `git diff --cached --name-status` so the agent knows *what kind* of change it was — which matters for decomposition grouping (an added asset usually belongs with the feature that uses it).
- **FR3c.** Binary filtering applies in **every diff path**: the staged diff (FR1–FR4), the multi-commit working-tree snapshot (§13.6.2), and the per-concept tree-to-tree concept diff (§13.6.3). The placeholder format is identical in all three.
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
- **FR22.** Render the agent's command: base command, **inference-provider flag + value** (the agent's `--provider`, emitted for multi-backend agents like pi/opencode/agy when an inference provider is resolved — §12), model flag + value, system-prompt flag + value (if the agent supports one), bare-mode flags, and the print-mode flag.
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
- **FR35.** Environment variables use the `STAGECOACH_` prefix. The model-invocation knobs (§12) are `STAGECOACH_PROVIDER` (the agent platform), `STAGECOACH_MODEL`, and `STAGECOACH_REASONING`, each with per-role variants `STAGECOACH_<ROLE>_PROVIDER` / `_MODEL` / `_REASONING`. Others: `STAGECOACH_TIMEOUT`, `STAGECOACH_CONFIG`, `STAGECOACH_VERBOSE`, `STAGECOACH_NO_COLOR`.
- **FR36.** Git config keys live under the `stagecoach.` section (`stagecoach.provider`, `stagecoach.model`, `stagecoach.reasoning`, `stagecoach.timeout`, `stagecoach.auto_stage_all`, etc.), with per-role `stagecoach.role.<role>.{provider,model,reasoning}`. Read via `git config --get`.
- **FR37.** A config file may define provider overrides (`[provider.<name>]`), defaults (`[defaults]`), and generation tuning (`[generation]`).
- **FR37a. `[provider.<name>]` blocks field-merge across layers.** A `[provider.<name>]` block merges **field-by-field across every config layer** (global → repo file → git config), exactly like scalar fields: a field a higher layer sets overrides that one field only; fields the higher layer omits are inherited from lower layers. A repo `[provider.pi]` setting only `default_model` must **not** erase other fields set in the global file. Field-merge onto the *built-in manifest* remains the registry's separate job. (The inference provider is NOT a `[provider.*]` field in v3 — it is the slash-prefix on `model`, FR-R5b.)
- **FR38.** `stagecoach config init` bootstraps a **populated** global config via cascading detection (§9.17, FR-B1); `stagecoach config path` prints the resolved config path; `stagecoach config upgrade` refreshes an existing config to the current schema version (FR-B5).

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
- **FR48.** User-defined agents in config files override built-ins of the same name; new names add new agents.

### 9.12 Dry run (P1)

- **FR49.** `--dry-run` — run the full diff→snapshot→generate→parse→duplicate-check pipeline, print the resulting message, but **do not** create the commit or move HEAD. Exit 0.

### 9.13 Verbosity & color (P1)

- **FR50.** `--verbose` / `-v` / `STAGECOACH_VERBOSE=1` — print the resolved provider command, the raw agent stdout, and each retry attempt to stderr.
- **FR51.** Color output when stdout is a TTY; disable with `--no-color` or `NO_COLOR`. Progress messages go to stderr so stdout stays clean for piping.
- **FR51b. Progress label shows the resolved model and provider.** The `↳ Generating…` / `↳ Decomposing…` progress line (stderr, FR51) names the resolved model invocation: `<Verb> with <model> in <provider>…`. The model string already carries the inference provider where relevant (FR-R5b), so no special formatting is needed — e.g. `Generating with sonnet in claude…`, `Generating with zai/glm-5.2 in pi…`, `Decomposing with anthropic/claude-sonnet-4 in opencode…`. When `model` is empty (the provider's own default), show `<provider>` alone. On the single-commit path the role is `message`; for decompose the label surfaces the **planner** role's resolved config, and `--verbose` prints all four roles (planner/stager/message/arbiter).

### 9.14 Multi-commit decomposition (P0, → G11, references §13.6)

- **FR-M1. Trigger.** Decomposition activates **iff** nothing is staged (`HasStagedChanges` false) **and** the working tree has changes. If anything is staged, the single-commit primitive (§13.1–§13.5) runs unchanged; stagecoach never re-partitions a hand-staged index.
- **FR-M1b. Start-of-run working-tree freeze (the concurrency invariant).** The instant decomposition activates, stagecoach captures an immutable snapshot of the entire working-tree change set — every modified/added/deleted/untracked path **and its byte content at that instant** — as a tree object `T_start` (the index is empty per FR-M1, so the change set is captured cleanly against HEAD). The planner partitions `T_start`'s diff (never a fresh re-read of the live tree), and every stager, the arbiter's leftover staging, the one-file short-circuit (FR-M2b), and the single shortcuts (FR-M11) stage and commit content drawn **strictly** from `T_start`. **Any file created or modified in the working tree after `T_start` is captured is invisible to the run** — never staged, never diffed, never committed — and is left untouched in the working tree for the user to handle separately. This generalizes v1's per-commit `write-tree` freeze (§13.1) to the whole run: the run commits *exactly* the working-tree state as it existed when the run began, so a long decomposition running alongside another tool (an editor save, a concurrent coding agent) can never sweep that tool's output into the commits.
- **FR-M1c. Freeze enforcement (defense-in-depth).** The stager is an external agent running git against the live tree, so stagecoach does not trust it to honor `T_start`. After each staging step, stagecoach verifies the resulting tree is a subset of `T_start` — only paths present in `T_start`, with `T_start`'s content. Any staged path or content not traceable to `T_start` (a concurrent working-tree change the stager picked up, or a stager that ran a bare `git add -A`) is a hard error: stagecoach aborts the run (non-rescue; already-landed commits stand, per FR-M12) rather than letting a concurrent change into a commit. The orchestrator owns the freeze boundary, not the stager — mirroring the HEAD-movement guard (§19). The orchestrator therefore never commits a bare `git add -A` against the live tree; staging is always restricted to `T_start` paths/content (the capture may transiently `add -A` to *build* `T_start`, then restore the index).
- **FR-M2. Modes.** (a) **Auto-decompose (default):** the planner decides the count *and* the partition; if it judges one commit is correct it emits the message in the same call (single-call shortcut, FR-M11). (b) **Forced count** `--commits N` (N ≥ 2): the count question is skipped; the planner only partitions into exactly N. (c) **Single (escape hatch)** `--single` / `--no-decompose` / `--commits 1`: the planner is bypassed entirely; v1 behavior (`git add -A` → one `CommitStaged`).
- **FR-M2b. One-file short-circuit (auto mode).** In auto-decompose mode, if the working tree has **exactly one** changed file (a single path in `git status --porcelain`), the planner is bypassed entirely: stagecoach stages that file's `T_start` content (FR-M1b), generates one commit message via the message role, and commits — the same outcome as the FR-M11 single shortcut but with **no planner agent call at all**. A single file cannot be sensibly partitioned into multiple commits, so invoking the planner is pure churn; this check is deterministic (changed-path count), not model judgment. An explicit `--commits N` (N ≥ 2) overrides this short-circuit — a forced count is honored even for one file (hunk-level staging may still partition).
- **FR-M3. Planner agent (bare).** Receives the full working-tree diff snapshot (with binary placeholders per FR3c) plus the style examples from §9.3. Returns a structured partition as JSON (the planner output is structured, so JSON is justified here — unlike free-form commit messages, §17.4): `{"count": N, "single": bool, "commits": [{"title": "…", "description": "what belongs in this commit"}, …], "message"?: "…"}`. `message` is present only when `single == true`.
- **FR-M4. Safety cap.** Refuse to create more than `max_commits` commits in one run (default 12) unless the user explicitly sets a higher `--commits` / `--max-commits`. Guards against a runaway planner producing dozens of micro-commits.
- **FR-M5. Stager agent (tooled).** For concept *i*, invoke a **tooled** agent bound to the repo (tools on, git allowed, non-interactive; §12.2 tooled mode) with the concept's title + description as its task. It finds *all* changes related to that concept and stages them (`git add <path>`, and hunk-staging via `git apply --cached` / the agent's patch application). It **must not commit, move refs, or push**; stagecoach owns all ref mutations. This is the single exception to stagecoach's "agent never touches git" rule, scoped strictly to staging.
- **FR-M6. Per-concept snapshot + overlapped generation.** After stager *i* returns, freeze `tree[i] = git write-tree` **before** stager *i+1* is allowed to start; then start the **message** agent for concept *i* using the concept diff `git diff tree[i-1] tree[i]` (tree-to-tree; never index-vs-HEAD). Stager *i+1* may run in parallel with that generation. All staging in this loop draws from `T_start` (FR-M1b) — the live working tree is never read for content after the run begins.
- **FR-M7. Serialized publication.** Commit *i* = `commit-tree -p newSHA[i-1] tree[i] msg[i]`; then `update-ref HEAD newSHA[i] newSHA[i-1]` (CAS). Commits land in strict order (each CAS requires HEAD == the previous newSHA); generation may overlap, publication may not.
- **FR-M8. Empty-concept skip.** If `tree[i] == tree[i-1]` (the stager staged nothing new), skip commit *i* — never create an empty commit — and log it. The concept is considered handled (nothing to commit).
- **FR-M9. Arbiter agent (bare).** After the loop, if `git status --porcelain` is non-empty (changes no stager claimed), invoke the arbiter with the SHAs, messages, and file-lists (`diff-tree`) of every commit made this run, plus a diff of the remaining changes. It returns `{"target": "<sha>"}` (one of this run's commits) or `{"target": null}` (→ new commit).
- **FR-M10. Arbiter resolution (stagecoach owns all git).** `null` → stage leftovers, snapshot, message-agent, `commit-tree` + `update-ref` as an (N+1)-th commit. `target == HEAD` → stage leftovers, `write-tree` → tree', `commit-tree -p <tip's parent> tree'` reusing the tip's message, `update-ref` (a plumbing amend of the tip). `target == an earlier commit[i]` → rebuild the linear chain: `read-tree` each base, fold the leftovers in at *i*, `write-tree` + `commit-tree` against the rebuilt parent, `update-ref` (deterministic reconstruction; never interactive rebase; HEAD only). **Ambiguous → default to `null` (new commit).** Amend is restricted to commits stagecoach made in *this* run.
- **FR-M11. Single-call shortcut.** In auto-decompose mode, if the planner returns `single: true`, stagecoach uses the planner's `message` directly: stage `T_start` (FR-M1b) → snapshot → `commit-tree` → `update-ref`. No separate message-agent call. If that message fails the duplicate check (§9.7), fall back to the standard message agent to regenerate.
- **FR-M12. Per-concept failure isolation.** A failed generation for concept *i* enters the rescue path (§18.3) for *that* concept; already-published commits 0..i-1 stand, and remaining staged work is left in the index with manual recovery printed. A CAS failure on commit *i* aborts the run with the §13.5 message; prior commits stand. A stager that exits non-zero is retried once, then treated as empty (FR-M8).

### 9.15 Per-role provider/model configuration (P0, → G12, references §16.4)

Each role resolves its **provider** (the agent platform), its **model** (which carries the inference provider as a prefix for multi-backend providers, §12/FR-R5b), and its **reasoning** level (FR-R6) — each independently configurable per role with cascading defaults. There is no separate inference-provider field; the model string is the single source of truth for both model and (where needed) inference backend.

- **FR-R1. Roles.** Four roles: **planner**, **stager**, **message**, **arbiter** (§13.6.2). Each resolves its provider, model, and reasoning independently.
- **FR-R2. Global default.** `[defaults].provider` / `[defaults].model` / `[defaults].reasoning` — surfaced as `--provider` / `--model` / `--reasoning`, `STAGECOACH_PROVIDER` / `STAGECOACH_MODEL` / `STAGECOACH_REASONING`, and `stagecoach.provider` / `stagecoach.model` / `stagecoach.reasoning` — is the fallback applied to **any** role that does not override that field. On the single-commit path the only active role is `message`, so setting just the global is equivalent to v1 (back-compatible).
- **FR-R3. Per-role overrides.** For each role and each field: `[role.<role>].{provider,model,reasoning}` in config; `STAGECOACH_<ROLE>_{PROVIDER,MODEL,REASONING}` in env; `--<role>-provider` / `--<role>-model` / `--<role>-reasoning` as flags (e.g. `--planner-model`, `--stager-provider`). **Every role exposes all three flags, including `message`** — no role is a special case that omits a flag (corrects the v2 gap that had no `--message-*` flags). Precedence (highest wins), applied independently per field: flag > env > per-role config > global config > built-in manifest default.
- **FR-R4. One setting for everything.** Setting only the global default (FR-R2) covers all roles and all three fields. Per-role overrides are opt-in granularity.
- **FR-R5. Model strings are provider-specific.** Because `gpt-5.4`, `anthropic/claude-sonnet-4`, `gemini-3.1-pro` belong to different inference backends, a per-role `model` is interpreted by that role's resolved **provider**'s manifest. For multi-backend providers the model is `inference/model` (FR-R5b); changing a role's provider without updating its model is a configuration error stagecoach surfaces (not silently ignores).
- **FR-R6. Reasoning level (per role).** Each role accepts a normalized `reasoning` level — `off | low | medium | high` — controlling the model's reasoning/thinking effort for that role's invocation. Resolution is per-field like `provider`/`model`: `--<role>-reasoning` / `STAGECOACH_<ROLE>_REASONING` / `[role.<role>].reasoning` / the global `[defaults] reasoning` (`--reasoning` / `STAGECOACH_REASONING` / `stagecoach.reasoning`). **Shipped defaults: `planner = high` (decomposition reasoning benefits from it); `stager = message = arbiter = off`** (speed/cost — reasoning rarely helps bare message generation or tool-call staging). The level is rendered via each provider's `reasoning_levels` manifest table (§12.1), appended at `Render` (§12.2). **Graceful no-op:** if the resolved provider/model has no reasoning control — the manifest declares no tokens for that level (or omits the table), or the model is not a reasoning model — the level is silently not applied (logged at `--verbose`), **never an error**. This lets a user pin a non-reasoning model on the planner (or a reasoning model on the message role) without configuring around it.
- **FR-R5b. Multi-backend models carry their inference provider as a prefix.** For providers that route to a choice of inference backend — **pi** (separate `--provider` flag) and **opencode** (`backend/model` token) — the `model` string MUST be in `inference-provider/model` form (e.g. `zai/glm-5.2`, `openai/gpt-5.4`). At `Render` (§12.2), a provider whose manifest declares a `provider_flag` (pi — the only one today) splits the model on the first `/` and emits `--provider <prefix> --model <rest>`; providers without one (opencode, and every single-backend provider) pass the model verbatim. A model without a `/` on a `provider_flag` provider is a **hard configuration error** (e.g. `model = "glm-5.2"` on pi is rejected with "include the inference provider, e.g. `zai/glm-5.2`") — never a silent bare `--model` that returns empty output. There is no separate inference-provider field to forget: **the prefix IS the field.** Authoritative enforcement lives at `Render` (the single command-emission chokepoint shared by every call path), and role resolution re-checks it earlier for a role-named error. Single-backend providers (claude, codex, cursor, gemini, agy, qwen-code) take a bare model.
- **FR-R6. Reasoning level (per role).** Each role accepts a normalized `reasoning` level — `off | low | medium | high` — controlling the model's reasoning/thinking effort for that role's invocation. Resolution is per-field like `agent`/`provider`/`model`: `--<role>-reasoning` / `STAGECOACH_<ROLE>_REASONING` / `[role.<role>].reasoning` / the global `[defaults] reasoning` (`--reasoning` / `STAGECOACH_REASONING` / `stagecoach.reasoning`). **Shipped defaults: `planner = high` (decomposition reasoning benefits from it); `stager = message = arbiter = off`** (speed/cost — reasoning rarely helps bare message generation or tool-call staging). The level is rendered via each agent's `reasoning_levels` manifest table (§12.1), appended at `Render` (§12.2). **Graceful no-op:** if the resolved agent/model has no reasoning control — the manifest declares no tokens for that level (or omits the table), or the model is not a reasoning model — the level is silently not applied (logged at `--verbose`), **never an error**. This lets a user pin a non-reasoning model on the planner (or a reasoning model on the message role) without configuring around it.

### 9.16 Default provider & per-role model defaults (P0, → G12, G14)

- **FR-D1. Cascading provider priority.** The auto-default provider is the highest-priority built-in whose command is found on `$PATH`, in this order: **pi, opencode, cursor, agy, gemini, qwen-code, codex, claude.** (Rationale: open / self-hostable harnesses first; closed subscription CLIs last. This is the maintainer's stated preference and lives in one slice in the registry — trivial to reorder.) User-defined providers (§12.8) are never auto-selected. Implemented as `Registry.DefaultProvider(installed)` over `preferredBuiltins`.
- **FR-D2. Decoupled from any one subscription.** No built-in default assumes a specific account or inference backend — notably **pi does NOT ship `glm-*` / `zai` as its default** (that was the original author's personal z.ai Max subscription). Because there is no universally-correct inference backend for a multi-backend provider, pi's shipped `default_model` is **blank** (the user supplies the `inference/model` prefix, FR-R5b). The personal z.ai/GLM setup is a documented *override* (`model = "zai/glm-5.2"`), not the default. The bootstrap (FR-B1) surfaces this explicitly rather than guessing a backend.
- **FR-D3. Universal role→tier strategy.** Out of the box each role is sized to its job: **planner = flagship/smart** (decomposition reasoning — one call per run, so a gated/rate-limited model like Gemini Pro is tolerable even on a free tier), **stager = mid** (reliable tool-use for git staging — deliberately *not* the fastest tier, because tooled staging needs dependable tool calls), **message = fast** (bare text generation — the **cheapest / free-tier-eligible** model is ideal, since message is the highest-volume role and many users are on free tiers), **arbiter = mid** (leftover judgment). Concretely, message defaults to Flash-Lite (gemini/agy), Haiku/nano (claude/cursor), etc. — the tier that stays available and cheap on a free plan. Users can override any role (§16.4); these are just the shipped defaults.
- **FR-D4. Per-provider default-model table.** The bootstrap config (§9.17) materializes one provider's column. Exemplars are current as of 2026-07; **see FR-D5 — they MUST be re-verified at implementation.**

| Provider | planner (smart) | stager (mid, tooled) | message (fast) | arbiter (mid) |
|---|---|---|---|---|
| **pi** | OpenAI flagship (e.g. `gpt-5.4`) via the user's pi sub-provider | `gpt-5.4-mini` | `gpt-5.4-nano` | `gpt-5.4-mini` |
| **opencode** | `openai/gpt-5.4` | `openai/gpt-5.4-mini` | `openai/gpt-5.4-nano` | `openai/gpt-5.4-mini` |
| **cursor** | flagship (e.g. `gpt-5.4`) | mid | nano | mid |
| **agy** | `gemini-3.1-pro` | `gemini-3.5-flash` | `gemini-3.1-flash-lite` | `gemini-3.5-flash` |
| **gemini** | `gemini-3.1-pro` | `gemini-3.5-flash` | `gemini-3.1-flash-lite` | `gemini-3.5-flash` |
| **qwen-code** | `qwen3-coder-plus` | `qwen3-coder-flash` | `qwen3-coder-flash` | `qwen3-coder-plus` |
| **codex** | `gpt-5.1-codex-max` | `gpt-5.1-codex-mini` | `gpt-5.4-nano` | `gpt-5.1-codex-mini` |
| **claude** | `opus` (4.8) | `sonnet` (5) | `haiku` | `sonnet` (5) |

  Notes: pi's `default_model` is intentionally **blank** in the shipped manifest — there is no universally-correct backend for pi, so the user supplies the `backend/model` prefix (FR-R5b); the bootstrap must not silently pick one. cursor's/codex's/qwen-code's exact model tokens are resolved during implementation by reading each CLI's live model list (FR-D5). A platform whose `tooled_flags` is empty (agy, opencode, and qwen-code today — §12.5.1.1, Appendix D) cannot serve as the **stager**; for those, the stager role falls back to the next-priority agent that *can*, and the bootstrap config annotates the fallback.
- **FR-D5. Research directive (planning / implementation).** Model lineups change fast (Sonnet 5 shipped 2026-07-01; Gemini and OpenAI iterate roughly monthly). The implementing agent MUST verify, per provider, the *current* flagship / mid / fast model names against each provider's live docs / `--help` before pinning any default, and record the verified names + verification date in the manifest source. A periodic refresh process (an automated check against each provider's model list) keeps them current over time — that process is **out of scope for this document** — but the defaults must be authored to be trivially refreshable (one table / one constant set per provider).

### 9.17 Config bootstrap & versioning (P0, → G8)

- **FR-B1. `config init` writes a populated, working config** (not an inert commented template). It runs cascading detection (FR-D1), writes `[defaults] provider = <detected>`, and writes that provider's `[role.*]` per-role default models (FR-D4) **uncommented** so the tool works immediately. For a multi-backend detected provider (pi, opencode) the written models are in `inference/model` form (FR-R5b); because there is no universally-correct inference backend (FR-D2), the bootstrap cannot invent the prefix — it either detects it (e.g. pi's configured backend) or leaves those models blank with guidance, never silently guessing. Other *installed* providers are written as commented-out `[role.*]` blocks so switching platforms is a one-line uncomment. Parent dirs are created; an existing file is not overwritten unless `--force`. The written path is always printed.
- **FR-B2.** `config init --provider <name>` targets a specific provider instead of auto-detecting. `config init --force` regenerates (overwrites) an existing file. The v1 "all-commented inert template" behavior is retained behind `config init --template` for users who want the bare reference.
- **FR-B3. Bootstrap on install, fallback on first run.** Where the install method permits a post-install step (Homebrew `post_install`, the curl\|sh installer, Scoop), stagecoach runs the equivalent of `config init` so a config exists immediately after install. First-run fallback: if stagecoach starts with no global config and no `STAGECOACH_CONFIG`, it auto-writes the bootstrap config once, prints a notice with the path, and continues — the tool is never "unconfigured."
- **FR-B4. Config schema version.** Every config file carries `config_version = <int>`; the binary knows `CurrentConfigVersion` (compile-time constant, bumped on any breaking config change). On load: if the file's version is missing or **older**, print a clear warning naming the mismatch and the remediation (`stagecoach config upgrade`, or `config init --force`); if **newer**, warn that the file is ahead of the binary. **Config v3** (this revision) folds the inference provider into the model string (FR-B7) and is auto-migrated on load for older files.
- **FR-B5.** `stagecoach config upgrade` rewrites an existing config to `CurrentConfigVersion` in place: preserving user values for keys that still exist, commenting out removed/renamed keys with a note. Simple, idempotent, future-extensible. FR-B7 defines the →v3 rewrite.
- **FR-B7. Config →v3: inference provider folds into the model string (breaking).** v3 keeps `provider` as the agent platform (the original meaning) and removes the separate inference-provider concept entirely: the inference backend becomes a slash-prefix on `model` for multi-backend providers (§12, FR-R5b). On load of a `config_version < 3` file, stagecoach auto-migrates **in memory** and emits a one-time deprecation notice pointing at `config upgrade`. The mapping: (a) `[provider.<name>]` blocks and the `[defaults] provider`/`[role.*] provider` fields are **unchanged** (they name the platform); (b) for a multi-backend provider, the former `[provider.<name>] default_provider = "X"` is **prepended** to its model as a prefix — `model = "Y"` + `default_provider = "X"` → `model = "X/Y"` (per-role and global); (c) the `default_provider` field is removed. Single-backend providers are untouched. **No value is invented:** a v2 file whose pi model has no resolvable prefix stays bare and becomes an FR-R5b error the user resolves by writing `zai/<model>`. `config upgrade` performs the same rewrite on disk. (Files that went through the abandoned intermediate `agent`/`[agent.*]` terminology are mapped back to `provider`/`[provider.*]` first, then step (b) applies.)
- **FR-B6. Help de-duplication.** The `config` and `agents` parent commands must not list their leaf commands twice. The manual "Subcommands:" block is removed from each parent's `Long`; cobra's auto-generated "Available Commands" is the single source. (The v1 `stagecoach config` output showed `init`/`path` both in the prose *and* in Available Commands — redundant.)

---

## 10. v1 scope, v1.1, and v2 roadmap

### 10.1 v1.0 (shipped — the single-commit core)

Everything in §9 marked P0 or P1 *other than* the §9.14/§9.15 additions. Concretely: diff capture, snapshot, prompt construction, auto-stage-all, generation via provider manifest, raw/json parsing with robust fallback, duplicate rejection, atomic commit, rescue protocol, config precedence, `providers list/show`, `config init/path`, `--dry-run`, `--verbose`, color. Built-in manifests for pi, Claude Code, Gemini CLI, opencode; documented (possibly stubbed) manifests for Codex and Cursor CLI. This is the implemented baseline against which the v2.0 additions below compose.

### 10.2 v1.1 (likely quick follow-ons)

- **`--edit`**: drop into `$EDITOR` with the generated message before committing.
- **`--body` / `--no-body`**: force multi-line or single-line regardless of history detection.
- **`--scope` / `--type`**: hint conventional-commit scope/type to the model.
- **`--amend`**: amend the previous commit's message via generation.
- **Fuzzy duplicate detection**: reject subjects within a Levenshtein distance of N of a recent subject (configurable), not just exact matches.
- **Per-provider `model` overrides in config** beyond the single global default.

### 10.3 v2.0 (this revision) — multi-commit decomposition + per-role models

The headline feature, **formerly deferred to "v2" and now specified**: when nothing is staged, decompose a dirty working tree into N logically-coherent commits via the snapshot machinery, with overlapped staging/generation and an arbiter pass for leftovers. Fully specified in **§13.6** (flow), **§9.14** (requirements), **§16.4** (config), **§17.5–§17.7** (prompts). This revision also adds per-role provider/model config (§9.15, §16.4), binary/non-text filtering (§9.1), and the Antigravity `agy` provider (§12.5.1).

This is exactly why the snapshot/atomic-commit foundation was built so carefully in v1: v2 reuses it N times in a loop. The old v1 auto-stage-all "one commit" behavior survives as the `--single` / `--no-decompose` escape hatch (and as the default when something is already staged).

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

The flow is deliberately linear and synchronous for the **single-commit** path. Concurrency (stage-while-generating) is achieved not by backgrounding Stagecoach, but by the user running `git add` in another terminal/pane during the blocking generation call — which is safe precisely because the commit is built from the frozen `TREE_SHA`, not the live index. See §13 for the full mechanics. The **multi-commit** path (§13.6) layers additional, *internal* concurrency on top of the same invariant: stage[i+1] overlaps generate[i], safe for the identical reason.

### 11.2 Process model

Stagecoach is a single process. It shells out to git (multiple times) and to the agent CLI (once per attempt). All subprocesses inherit Stagecoach's working directory (the repo root) and environment, with a controlled, minimal set of extra env vars passed to the agent only if the manifest requests them. Stagecoach owns signal handling: SIGINT/SIGTERM propagates to the currently-running child and then triggers the rescue path.

### 11.3 Design constraints that protect v2 (now realized)

The v2 multi-commit feature needs to: (a) partition the diff, (b) for each partition, stage exactly that subset, snapshot, generate, commit, repeat. The v1 architecture was built to make this trivially composable, and v2 now realizes it (§13.6). Concretely:

- The core is a function `commitStaged(ctx, cfg) error` (or `(sha, error)`) that assumes the index is already in the desired state. It does not decide *what* to stage; it commits *whatever is staged*.
- The single-commit path is: `maybeAutoStage(); commitStaged()`.
- The multi-commit path (§13.6) is: `plan() → for each concept { stageConcept(); snapshot(); generate() ‖ stageNextConcept(); commit() }; arbitrate()`. It composes `commitStaged`'s primitives (snapshot/commit-tree/update-ref) per concept, but drives staging from the *planner's* concepts rather than from a user `git add`.

The keystone discipline: **staging policy is never entangled with commit logic.** §14's package layout enforces this.

### 11.4 Multi-commit pipeline (data flow)

```
                 nothing staged + dirty working tree
                              │
                              ▼
            ┌────────────┐   full working-tree diff (binary placeholders)
            │  planner   │◀──── + style examples
            │ (bare)     │
            └─────┬──────┘   JSON: {count, single, commits:[…], message?}
                  │ single? ──yes──▶ git add -A → CommitStaged (one call) → done
                  ▼ no (N concepts)
         for i in 0..N-1:
            ┌────────────┐  concept[i] description        ┌────────────┐
            │  stager[i] │──────────────────────▶ index   │            │
            │ (tooled)   │   (mutates index; no commit)   │            │
            └─────┬──────┘                                │            │
                  ▼ tree[i]=write-tree (FROZEN)            │            │
            ┌────────────┐  diff(tree[i-1],tree[i])  ═══▶ │  message[i]│ (bare)
            │            │                                │ (overlaps) │
            │            │  ‖ stager[i+1] runs here       │            │
            └─────┬──────┘                                └─────┬──────┘
                  ▼ msg[i]                                      │
            commit-tree -p newSHA[i-1] tree[i] msg[i] ◀──────────┘
            update-ref HEAD newSHA[i] newSHA[i-1]   (serialized)
                  ▼
         git status clean? ──yes──▶ done
                  │ no
                  ▼
            ┌────────────┐  commits made + leftover diff   target SHA or null
            │  arbiter   │◀───────────────────────────▶  (stagecoach does all git)
            │ (bare)     │
            └────────────┘
```

The two invariants that make `stager[i+1] ∥ message[i]` safe are: (1) **tree[i] is frozen before stager[i+1] begins**, so it captures exactly concept[i]; and (2) **the concept diff is `tree[i-1]→tree[i]`, never index-vs-HEAD**, so message[i] is immune to concurrent staging and to commits landing. See §13.6.3.

### 11.5 Two invocation modes: bare and tooled

Stagecoach invokes agents in one of two modes, selected by role:

- **Bare mode** (existing; §12.1 `bare_flags`): tools off, session-less, chrome-less, ephemeral. A pure text-in/text-out call. Used by the **planner**, **message**, and **arbiter** roles — none of which touch git; they reason over a diff/context stagecoach hands them and return text/JSON.
- **Tooled mode** (new; §12.1 `tooled_flags`): tools on, constrained to staging-relevant tools (git/read/edit, per provider), non-interactive, repo-scoped. Used **only** by the **stager** role, which must mutate the index. This is the single deliberate exception to stagecoach's "agent never touches git" rule.

Both modes reuse the manifest's command/model/provider/subcommand/print_flag/delivery fields; only the flag-set that makes the call "bare" vs "tooled" differs. §12.1 adds `tooled_flags` and §12.2 adds a `mode` to the rendering algorithm. The safety properties in §12.7.1 still hold for bare roles; the stager's safety is enforced by `tooled_flags` (git-only toolset) plus the hard rule that it never runs `commit`/`update-ref`/`push` (stagecoach intercepts by instruction and, defensively, by not granting commit-capable surfaces where a provider allows scoping).

---

## 12. The provider system

Stagecoach's provider system is the heart of its agent-agnosticism: given a logical intent ("call an agent with this system prompt and this user payload, bare and ephemeral, with this model"), produce a concrete command line for a specific **provider** (the agent platform), run it, and parse the result.

**Terminology — two concepts.** Stagecoach configures a model invocation with TWO concepts. There is no separate "inference provider" field — it folds into the model string (below), which is what eliminates the term overload that caused repeated routing bugs.

| Concept | What it is | Examples | Config field | Flag | Env | Git key |
|---|---|---|---|---|---|---|
| **provider** | the agent platform / CLI stagecoach shells out to | pi, opencode, claude, codex, cursor, gemini, agy, qwen-code | `provider` | `--provider` | `STAGECOACH_PROVIDER` | `stagecoach.provider` |
| **model** | the model identifier | `zai/glm-5.2`, `openai/gpt-5.4`, `sonnet`, `gemini-3.1-pro` | `model` | `--model` | `STAGECOACH_MODEL` | `stagecoach.model` |

**The inference provider lives in the model string, not a separate field.** Some providers route to a choice of upstream inference backend — **pi** (via a separate `--provider <backend>` flag) and **opencode** (via a `backend/model` token like `openai/gpt-5.4`). For these, the model string carries the inference provider as a **slash-prefixed namespace**: `zai/glm-5.2`, `openai/gpt-5.4`, `anthropic/claude-sonnet-4`. Providers with a fixed backend (claude, codex, cursor, gemini, agy, qwen-code) take a bare model (`sonnet`, `gemini-3.1-pro`).

- **pi renders the prefix as a separate flag; opencode passes it whole.** At `Render` (§12.2), if the provider's manifest declares a `provider_flag` (pi — the only one today), stagecoach splits the model on the first `/` and emits `--provider <prefix> --model <rest>` (so `zai/glm-5.2` → `pi --provider zai --model glm-5.2`). Providers without a `provider_flag` (opencode, and every single-backend provider) pass the model string verbatim.
- **A bare model on a `provider_flag` provider is a hard error** (FR-R5b): `model = "glm-5.2"` on pi is rejected with "include the inference provider, e.g. `zai/glm-5.2`" — never silently rendered as an unroutable `pi --model glm-5.2`. This is precisely the bug class that motivated the design: there is no separate inference-provider field to forget, because **the prefix IS the field**.
- The manifest block is **`[provider.<name>]`**; the former `default_provider` field is **removed** — the model prefix replaces it.

### 12.1 The manifest schema

Each provider (agent platform) is described by a manifest. Manifests are TOML (human-editable, no quoting hell for flag lists). Built-in manifests are compiled into the binary (so the tool works with zero config); user manifests in config files override or extend them.

```toml
# An agent manifest. All fields except `name` and `command` are optional
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
# Default model if the user specifies none. Empty in the shipped pi default
# (decoupled from any one subscription, §9.16 FR-D2); config init fills per-role.
default_model = ""

# --- system prompt -------------------------------------------------------
# If the agent supports a system prompt. Empty means "prepend to the user
# payload" (fallback for agents with no system-prompt flag).
system_prompt_flag = "--system-prompt"

# --- sub-provider (the inference backend) -------------------------------
# pi has a --provider flag; per FR-R5b/§12.2 the inference backend is the slash-PREFIX
# on `model` (e.g. model "zai/glm-5.2" → pi --provider zai --model glm-5.2). There is
# NO `default_provider` field in v3 — the prefix on `model` IS the provider. opencode
# has no provider_flag and takes `backend/model` verbatim instead.
provider_flag = "--provider"

# --- bare mode -----------------------------------------------------------
# Flags appended verbatim to make the call tool-less, session-less,
# extension-less, chrome-less, and ephemeral. These are agent-specific.
# Used by the bare roles: planner, message, arbiter (§11.5).
bare_flags = [
  "--no-tools",
  "--no-extensions",
  "--no-skills",
  "--no-prompt-templates",
  "--no-context-files",
  "--no-session",
]

# --- tooled mode (v2; §11.5) ---------------------------------------------
# Flags appended verbatim to make the call TOOL-ENABLED, non-interactive, and
# scoped to staging-relevant tools (git / read / edit), for the stager role —
# the ONLY role that touches git. Each provider expresses "tooled but safe" in
# its own idiom (an allowlist, a sandbox, an approval-mode). nil/empty => this
# provider does not support tooled mode and cannot serve as a stager.
tooled_flags = [
  # e.g. for an agent with an allowlist + auto-approve:
  # "--allowed-tools", "Bash(git:*),Read,Edit",
  # "--approval-mode", "auto",
]

# --- reasoning level (optional; FR-R6) ----------------------------------
# Per-level flag tokens appended to express the model's reasoning/thinking
# effort. Omit the table entirely (or declare only `off = []`) for agents or
# models with no reasoning control — a non-`off` level set on such an agent is
# a silent no-op (never an error). Token lists are agent-specific; verify per
# FR-D5. Shape shown for illustration only.
[reasoning_levels]
off = []
# low    = ["--thinking", "low"]
# medium = ["--thinking", "medium"]
# high   = ["--thinking", "high"]

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

Given a manifest `m`, a resolved model `model` (which, for a multi-backend provider, is in `inference/model` form — §12/FR-R5b), a system prompt `sys`, a user payload `user`, and a **mode** (`"bare"` | `"tooled"`; default `"bare"`):

```
args = [m.subcommand...]
# FR-R5b: a provider with a provider_flag (pi) takes "inference/model"; split it.
# The prefix becomes the agent's --provider; the rest is the model. A bare model
# (no "/") on such a provider is a hard error, not a silent bare --model.
if m.provider_flag != "":
    if model == "":
        pass                                   # no model → emit neither flag
    elif "/" in model:
        inf, model = split(model, "/", 1)
        args += [m.provider_flag, inf]
    else:
        error("model %q on %s must be inference/model, e.g. zai/glm-5.2", model, m.name)
if m.model_flag and model != "":
    args += [m.model_flag, model]               # verbatim for non-provider_flag providers (opencode, single-backend)
# reasoning level (FR-R6): append the resolved level's tokens if the provider
# declares them; absent/empty => silent no-op (provider or model lacks reasoning control).
if reasoning != "" and len(m.reasoning_levels[reasoning]) > 0:
    args += m.reasoning_levels[reasoning]
if m.system_prompt_flag and sys != "":
    args += [m.system_prompt_flag, sys]
# mode selects the flag-set: bare (tools off) for planner/message/arbiter,
# tooled (git tools on, scoped) for the stager (§11.5). tooled with no
# tooled_flags defined => error (provider cannot serve as a stager).
args += (mode == "tooled") ? m.tooled_flags : m.bare_flags
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

**FR-R5b (enforced at this chokepoint):** if `m.provider_flag` is set (a multi-backend provider) and `model` is non-empty but contains no `/`, `Render` returns an error rather than emitting a bare `--model` — it splits `backend/model` into `--provider <backend> --model <model>` (§9.15). This is the single command-emission gate every call path flows through, so no path can produce an unroutable command.

**Reasoning level (FR-R6):** `Render` also receives the role's resolved `reasoning` level (`off|low|medium|high`) and appends `m.reasoning_levels[level]` after the model flag. A level the agent does not declare (no tokens) is a silent no-op — never an error — so a non-reasoning model can be pinned anywhere without configuration.

### 12.3 Built-in provider: pi

Captured from `pi --help` on the author's machine (2026-06-29). pi is a **harness** that routes to model backends via its own sub-providers. Its shipped `default_model` is **deliberately empty** — it is populated per-role by `config init` (§9.17), and because pi is multi-backend the model is supplied in `backend/model` form (FR-R5b), e.g. `zai/glm-5-turbo`. The shipped default does **not** assume the author's personal z.ai/GLM subscription (FR-D2); that is shown as a personal *override* below.

```toml
name = "pi"
detect = "pi"
command = "pi"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "--model"
default_model = ""            # empty in the shipped default; config init fills per-role (§9.16/§9.17)
system_prompt_flag = "--system-prompt"
provider_flag = "--provider"          # FR-R5b: the inference backend is the slash-PREFIX on `model`; no default_provider field in v3
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

Rendered (model `<backend>/<m>` — stagecoach splits the prefix per FR-R5b):

```
pi --provider <backend> --model <m> --system-prompt "<sys>" \
   --no-tools --no-extensions --no-skills --no-prompt-templates \
   --no-context-files --no-session -p            < <user payload via stdin>
```

**Personal-override example (NOT the shipped default).** The original `commit-pi` script — and the author's daily setup — routes pi to z.ai GLM: `model = "zai/glm-5-turbo"` (the invocation above with `<backend>=zai`, `<m>=glm-5-turbo`, byte-for-byte the `commit-pi` call). That is the author's *subscription-specific* override, kept here as the reference shape; it is not a default anyone else would inherit.

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
default_model = "gemini-3.1-pro"
system_prompt_flag = ""             # gemini-cli has no first-class --system flag at present
provider_flag = ""
bare_flags = [
  "--approval-mode", "default",     # don't auto-run tools
]
output = "raw"
strip_code_fence = true
# fallback: system prompt prepended to the positional payload (see §12.2)
```

Rendered (model `gemini-3.1-pro`):

```
gemini -m gemini-3.1-pro --approval-mode default "<sys>\n\n<user payload>"
```

Caveats (to verify at integration time): the `-p/--prompt` flag is deprecated in favor of the positional `query`, and the help notes stdin is appended to the prompt — so `prompt_delivery = "stdin"` may also work and is preferable for large diffs (avoids arg-length limits). The manifest should default to whichever is verified to handle a ~300 KB payload; candidates are `stdin` first, `positional` as fallback. Gemini CLI's lack of a system-prompt flag means the system prompt is prepended to the payload per §12.2.

### 12.5.1 Built-in provider: Antigravity CLI (`agy`) — the Gemini-CLI successor

Antigravity CLI (`agy`) is Google's terminal coding agent; it **superseded `gemini` (Gemini CLI) on 2026-06-18** and is the Gemini lineage's current surface. It matters to stagecoach for the same structural reason every provider does: **the Antigravity coding-plan quota is reachable only through `agy`**, never over the public API. Flag surface below is assembled from Antigravity's published docs and issue tracker (2026-06); it carries the PRD's heaviest `# TO CONFIRM` load of any built-in (see the block beneath the manifest) and should be marked `experimental` until a real end-to-end run clears the items.

```toml
# Antigravity CLI. Researched from docs + issue tracker (not yet `--help`-verified).
name = "agy"
detect = "agy"
command = "agy"
subcommand = []
prompt_delivery = "stdin"          # agy appends stdin to the prompt (gemini lineage); avoids arg limits
print_flag = "-p"                  # `-p` / `--print`: run one prompt non-interactively, print response
model_flag = "-m"                  # `-m` / `--model`; gemini-lineage. # TO CONFIRM exact token
default_model = "gemini-3.1-pro"   # agy runs the Gemini family; user overrides per account
system_prompt_flag = ""            # none first-class (like gemini-cli) → prepend to payload (§12.2). # TO CONFIRM
provider_flag = ""
# bare mode: read-only, no tool execution (planner / message / arbiter roles).
# `--approval-mode default` mirrors gemini-cli; # TO CONFIRM agy's equivalent read-only mode.
bare_flags = ["--approval-mode", "default"]
# tooled mode: the STAGER role needs git tool execution, non-interactive and scoped.
# agy exposes `--sandbox` and `--dangerously-skip-permissions`; neither is the right primitive
# alone. The intended combo is a scoped tool allowlist + a non-interactive approval mode that is
# NOT the unscoped bypass. Exact flags: # TO CONFIRM at integration (see §12.5.1.1).
tooled_flags = []                  # intentionally empty until verified; agy cannot stager until set
output = "raw"
strip_code_fence = true
```

Rendered, bare (model `gemini-3.1-pro`):

```
agy -m gemini-3.1-pro --approval-mode default -p            < <sys+user payload via stdin>
```

#### 12.5.1.1 TO CONFIRM (agy) — block until a real run clears these

1. **CRITICAL — non-TTY stdout drop (issue [#76](https://github.com/google-antigravity/antigravity-cli/issues/76)):** `agy --print`/`-p` **silently drops stdout when invoked from a non-TTY** (pipe / subprocess / redirect) — which is exactly how stagecoach spawns agents. Until upstream fixes it, stagecoach's subprocess model cannot reliably read `agy`'s output. Candidate workaround under evaluation: allocate a PTY for the `agy` child (so it sees a TTY) while still capturing its bytes. This must be solved (or PTY-shimmed) before `agy` is usable for any role. This is the single biggest open item for the provider.
2. **Model flag:** confirm `-m` vs `--model` vs both; set `model_flag` accordingly.
3. **System-prompt flag:** confirm whether `agy` gained a first-class `--system`/`--system-prompt` (gemini-cli never had one); if so, populate `system_prompt_flag` and stop prepending.
4. **Tooled (stager) flags:** determine the exact combination that yields *non-interactive, git-scoped, auto-approved* tool execution **without** the unscoped `--dangerously-skip-permissions` (which §19 forbids). Candidates: a `--allowed-tools`/`--allowed-tools-pattern` allowlist restricted to `git`/`Read`/`Edit`, paired with the least-permissive approval mode that still executes non-interactively. Until this is known, `tooled_flags = []` and `agy` **cannot serve as a stager** (it can still serve the bare roles once item 1 is resolved).
5. **Print-mode timeout:** `agy` exposes `--print-timeout` (default 5m); stagecoach's own `--timeout` (§9.5) governs the kill, but a shorter `--print-timeout` makes `agy` exit cleanly rather than hang — wire it to the same budget.

Until items 1–4 clear, `agy` ships `experimental = true` (§12.7.2) with the bare-roles path gated on item 1 and the stager path gated on item 4.

### 12.5.2 Built-in provider: qwen-code — the Qwen3-Coder CLI (a Gemini-CLI fork)

`qwen-code` (npm `@qwen-code/qwen-code`; GitHub `QwenLM/qwen-code`) is Alibaba/Qwen's terminal coding agent. It is a **fork of Google's Gemini CLI** and behaves identically to it at the invocation/API level — same flag surface (`-m`/`--model`, `--approval-mode`, `-p`/`--prompt`, stdin delivery, no first-class system-prompt flag) — so its manifest mirrors `gemini`/`agy` (§12.5, §12.5.1). What differs is the **model line and capabilities**: qwen-code is tuned for the **Qwen3-Coder** family, reached via Alibaba Cloud Model Studio / DashScope (`DASHSCOPE_API_KEY`, or `qwen-code login` for the free coding-plan quota). It is **single-backend** (Qwen/DashScope), so `provider` is meaningless and `provider_flag` is empty. Exact default model tokens (`qwen3-coder-plus` et al.) and the `reasoning_levels` mapping are **# TO CONFIRM** per FR-D5 — the model lineup differs materially from Gemini's despite the shared CLI lineage. Mark `experimental` until a real end-to-end run clears it.

```toml
# qwen-code — Gemini-CLI fork for Qwen3-Coder. Researched from docs (not yet `--help`-verified).
name = "qwen-code"
detect = "qwen-code"
command = "qwen-code"
subcommand = []
prompt_delivery = "stdin"          # gemini-lineage: stdin appended to the prompt; avoids arg limits
print_flag = "-p"                  # `-p` / `--prompt`: one-shot non-interactive
model_flag = "-m"                  # gemini-lineage `-m` / `--model`. # TO CONFIRM exact token
default_model = "qwen3-coder-plus" # Qwen3-Coder family; user overrides per account. # TO CONFIRM
system_prompt_flag = ""            # none first-class (gemini lineage) → prepend to payload (§12.2). # TO CONFIRM
provider_flag = ""                 # single-backend (Qwen/DashScope)
bare_flags = ["--approval-mode", "default"]   # read-only, no tool execution. # TO CONFIRM gemini-equivalent
tooled_flags = []                  # empty until verified → cannot stager until set (FR-D4 fallback)
output = "raw"
strip_code_fence = true
# [reasoning_levels]               # Qwen3-Coder thinking-effort flags: # TO CONFIRM (FR-R6/FR-D5)
```

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

There is a real architectural split across our built-in agents in **how they become "bare"**:

- **Explicit tool-disable flags:** pi (`--no-tools`), Claude Code (`--tools ""`). These agents offer a literal "turn tools off" switch, so the call is a pure text-in/text-out with no agent loop. Fast and clean.
- **Read-only constraint instead:** Codex (`--sandbox read-only`), Cursor (`--mode ask`), Gemini (`--approval-mode default`). These agents have **no** global "disable all tools" switch — tools are intrinsic to their loop. We constrain them to a read-only, never-ask profile so they *cannot* mutate the repo or block on a prompt, but the model may still internally reason with tools.

Consequences, stated plainly:

1. **Safety is preserved either way.** Read-only sandbox/mode + never-ask means no provider in the default set can touch the working tree. The repo-integrity invariant (§18.1) holds for all six.
2. **Latency varies.** The read-only-constrained agents may be slightly slower (they run an agent loop the model can choose to use). Acceptable for a one-shot message.
3. **Output is still just the message.** Whichever path the model takes, the final assistant text is what Stagecoach parses; a model that "reads a file" before answering still ends with a commit message on stdout.

This is not a defect to paper over — it is the honest cost of supporting heterogeneous agent CLIs through one manifest schema. The `bare_flags` field exists precisely so each provider expresses "bare" in its own idiom.

**The stager role inverts this (v2; §11.5).** The per-concept staging agent is the one place stagecoach *wants* tools on — it must run `git add` and apply hunks. There the `tooled_flags` field (§12.1) takes over, expressing "tooled but safe" (a git-scoped allowlist or a read/write sandbox) in each provider's idiom. The safety invariant for the stager is therefore not "no tools" but "tools scoped to staging, and never `commit`/`update-ref`/`push`" — enforced by `tooled_flags` plus the hard rule that ref mutations are stagecoach's alone (§13.6.2, §19). A provider with empty `tooled_flags` simply cannot serve as a stager; it can still serve the bare roles.

#### 12.7.2 On stubbing and progressive verification

We do not pretend to know everything up front. The implementing agent will do its own comprehensive research per task. The contract here is: **the manifest *schema* and the built-in default manifests are fixed by this document; the exact behavior of each manifest is confirmed by a real end-to-end run during implementation.** Two explicit `# TO CONFIRM` notes are carried above (codex stdout-on-success; cursor `ask`-wins-over-`-p`). Any manifest field that cannot be confirmed is left at a safe default and marked with a `# TO CONFIRM` comment, never silently assumed. The `experimental` flag remains available (see §22.1) for any *future* provider added from docs alone rather than a verified `--help`.

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

### 13.6 Multi-commit decomposition (the v2 core, now specified)

§13.1–§13.5 describe the single-commit primitive. This subsection specifies how stagecoach composes that primitive N times to turn one large, *un-staged* working tree into a sequence of logically-coherent commits. It is the feature formerly deferred to "v2" (old §10.3); it is now in scope. The snapshot machinery in §13.1–§13.5 is exactly what makes it possible — and the reason the v1 foundation was built so deliberately.

Functional requirements live in §9.14; prompts in §17.5–§17.7; config in §16.4. This section is the *flow*.

#### 13.6.1 When it activates (the trigger model)

Decomposition activates **iff** nothing is staged (`git diff --cached --quiet` reports empty) **and** the working tree has changes. This replaces v1's "nothing staged → auto-stage-all into one commit" behavior (FR16) as the default for *that* state. **The first action on activation is to freeze the entire working-tree change set into `T_start` (FR-M1b)** — the planner, every stager, the arbiter, and all shortcuts then operate on that frozen snapshot, so files created or modified by a concurrent process during the (potentially long) run are excluded from every commit. Three modes:

| Mode | Trigger | Planner's job |
|---|---|---|
| **Auto-decompose (default)** | nothing staged, no `--commits` | decide the count **and** partition; if it judges one commit is correct, also emit the message (single-call shortcut, §13.6.4) |
| **Forced count** | `--commits N` (N ≥ 2) | skip the count decision; partition into **exactly N** |
| **Single (escape hatch)** | `--single` / `--no-decompose`, or `--commits 1` | planner bypassed entirely; old v1 behavior (`git add -A` → one `CommitStaged`) |

If something is already staged, the single-commit primitive (§13.1–§13.5) runs **unchanged** — decomposition never re-partitions a hand-staged index.

`--commits N` is the critical user control: it asserts the answer to "how many commits?" so the planner is never asked to count (it only partitions into N). This both saves a reasoning round-trip and keeps the user in control of commit granularity. `--commits 1` is equivalent to `--single`.

**One-file short-circuit (FR-M2b).** In auto mode, if exactly one file is changed, the planner is skipped entirely — stagecoach stages that one file and generates a single commit message directly (no planner round-trip). One file cannot sensibly decompose, so the planner call is pure churn. `--commits N` (N ≥ 2) overrides this and is honored even for a single file.

#### 13.6.2 The four agent roles

Decomposition is a multi-agent pipeline. Each role is a distinct invocation, independently bindable to its own provider and model (§16.4); all default to the global `provider`/`model`.

| Role | Mode | Job | Output contract |
|---|---|---|---|
| **planner** | bare | analyze the full working-tree diff; decide count (unless forced) + partition into concepts; if single, also write the message | JSON `{count, single, commits:[{title,description}], message?}` (§17.5) |
| **stager** | **tooled** (runs git in the repo) | for one concept, find **all** related changes and stage them (`git add`, hunk-stage via `git apply --cached`) | exits 0; mutates the index; returns a short confirmation |
| **message** | bare | generate one commit message for one frozen snapshot — this **is** the §13.1–§13.5 agent, unchanged | raw text (the message) |
| **arbiter** | bare | after all commits, if changes remain, decide which just-made commit (by SHA) the leftovers belong to, or "new" | JSON `{target: "<sha>" \| null}` (§17.7) |

**Only the stager breaks bare mode.** This is the one architectural departure from §12.7.1: the stager must actually mutate the index, so it runs as a full agent with git tool access in the repo (§11.5 tooled mode). Every other role is a pure text-in/text-out call. The manifest system models this with one new field, `tooled_flags` (§12.1), used in place of `bare_flags` for stager invocations. The stager **never** commits, moves refs, or pushes — stagecoach owns all ref mutations (FR-M5, §19).

#### 13.6.3 The pipeline (sequential staging, overlapped generation)

```
planner  ──▶  concepts[0..N-1]      (one call; single-shortcut → done, §13.6.4)
   │
   ▼
for i in 0..N-1:
   stager[i]     ──▶  index now holds concepts[0..i]
   snapshot[i]   ──▶  tree[i] = write-tree   ◀── FROZEN here, before stager[i+1]
   ┌── message[i] : diff(tree[i-1], tree[i])  ──▶ msg[i]
   │        ‖   (parallel; safe — see invariants)
   └── stager[i+1]  (only if i+1 < N)        ──▶ index now holds concepts[0..i+1]
   commit[i]   =  commit-tree -p newSHA[i-1] tree[i] msg[i]
   update-ref HEAD newSHA[i] newSHA[i-1]        ◀── serialized, in order
arbiter   ──▶  (only if working tree ≠ clean) amend-or-new (§13.6.5)
```

Three invariants make `stager[i+1] ∥ message[i]` safe, all consequences of the snapshot design:

1. **`tree[i]` is frozen before `stager[i+1]` starts.** write-tree is a pure, ref/index-read-only operation (§13.2) that captures the current index. Because the orchestrator snapshots *immediately* after stager[i] returns and *before* launching stager[i+1], `tree[i]` records exactly concept[i] on top of `tree[i-1]` — whatever stager[i+1] does to the live index afterward cannot reach `tree[i]`.
2. **The concept diff is computed tree-to-tree, never index-vs-HEAD.** `message[i]` reasons over `git diff tree[i-1] tree[i]`, which is exactly what stager[i] added. This is independent of where HEAD points, so it is immune to concurrent staging *and* to earlier commits landing. (The single-commit path's `StagedDiff` is index-vs-HEAD; the loop deliberately does not reuse it, for this reason.)
3. **`update-ref`s serialize.** commit[i] parents to `newSHA[i-1]` and CAS-moves HEAD only if `HEAD == newSHA[i-1]`. So commits publish in strict order even though their *generation* overlapped. Two `commit-tree` calls may build dangling objects concurrently, but the chain of `update-ref`s is strictly sequential.

**Index model: accumulate, never reset.** Stagecoach does not reset the index between concepts. The index grows: after stager[i] it holds concepts[0..i]; `tree[i]` is that full accumulation. After commit[N-1] lands, `HEAD.tree == tree[N-1] ==` the full accumulated index, so the index is clean relative to HEAD. Any un-committed residue therefore lives **only in the working tree** (changes no stager claimed) — that is the arbiter's input (§13.6.5), not a staged-area artifact.

**Base cases.** `tree[-1]` is the original parent tree (`git rev-parse HEAD^{tree}`, or the empty tree for an unborn repo). For an unborn repo, commit[0] is a root commit and subsequent commits chain normally. Per-concept "what landed" is `diff-tree newSHA[i]` vs `newSHA[i-1]` = exactly concept[i] (FR42 reporting, per commit).

#### 13.6.4 The single-commit shortcut

If the planner (in auto-decompose mode) judges that one commit is the correct call, it returns `single: true` **plus the message in the same response**. Stagecoach then: `git add -A` → snapshot → `commit-tree` → `update-ref`, using the planner's message. **No separate message-generation call is made** — the trivial case stays a single agent round-trip. (If the planner's message fails the duplicate check §9.7, the standard message agent is invoked as a fallback to regenerate, then the normal commit path.)

This is why the planner's output contract carries an optional `message` field (§17.5): present iff `single == true`. It lets the planner, which has already read the whole diff, produce the message for free when N=1, instead of forcing a second call.

#### 13.6.5 The arbiter (leftover reconciliation)

After the loop, if `git status --porcelain` is non-empty (some changes were not claimed by any stager), the arbiter runs. It receives: the SHAs, messages, and file-lists (`diff-tree`) of every commit made *this run*, plus a diff of the remaining changes (with binary placeholders). It returns either a target SHA (one of this run's commits) or `null` (→ new commit). **Stagecoach performs ALL git; the arbiter only decides** (FR-M9/M10):

- **`null` / "new":** stage the leftovers, snapshot, run the message agent, `commit-tree` + `update-ref` as an (N+1)-th commit.
- **`target == HEAD` (the tip):** stage the leftovers, `write-tree` → tree', `commit-tree -p <tip's parent> tree'` reusing the tip's message, `update-ref HEAD`. A plumbing amend of the tip — no `git commit --amend`.
- **`target == an earlier commit[i]` (mid-chain):** stagecoach **rebuilds the linear chain**. Because stagecoach built the whole chain and holds every `tree[j]` and `msg[j]`, this is a deterministic reconstruction: for each j, `read-tree` the appropriate base, fold the leftovers in at j==i, `write-tree`, `commit-tree` against the rebuilt parent, `update-ref`. This is **never** an interactive rebase and never touches refs other than HEAD. (Exact plumbing is detailed in the implementation plan; the contract here is the contract above.)
- **Ambiguous → default to `null` (new commit).** Stagecoach never amends outside the just-made set, and never force-updates a ref.

If `git status --porcelain` is empty after the loop, the arbiter does not run — the perfect run.

#### 13.6.6 Failure handling within the loop

Each concept is independently recoverable (extends §18):

- **Stager stages nothing** (`tree[i] == tree[i-1]`): skip commit[i] — no empty commits (FR-M8); log and continue.
- **Stager exits non-zero:** retry once; on second failure treat as empty (FR-M8) and continue, so one bad concept cannot poison the run.
- **message[i] generation fails** (parse / duplicate-exhausted / timeout): enter the rescue path (§18.3) **for concept i only**. Already-published commits 0..i-1 stand. The frozen `tree[i]` and manual recovery are printed; remaining staged work stays in the index for the user to finish by hand. (The overlapped stager[i+1], if already running, is allowed to complete so its staging is not lost — it remains staged for the user.)
- **CAS failure on commit[i]** (HEAD moved externally): abort the run with the §13.5 "HEAD moved" message; prior commits stand; the in-flight tree[i] recovery command is printed.
- **Planner fails / returns unparseable output:** no commits have been made yet (planning precedes all staging); surface the error and exit non-rescue (nothing was snapshotted).

#### 13.6.7 Why this is safe (the one-paragraph proof)

Every commit is built from a frozen `tree[i]` captured *before* the next concept's staging begins, and its message is generated from a tree-to-tree diff that never consults the live index or HEAD. Refs move only at `update-ref`, serialized in order, each a CAS that refuses to clobber a moved HEAD. The only agent that mutates the repo is the stager, and it is scoped to `git add`-class operations — it cannot commit, amend, or push. Therefore: a failed, slow, or mis-behaving agent can never corrupt history, never lose staged work, and never produce a commit containing changes meant for a different concept. The worst case is a rescue message pointing at a frozen tree the user commits by hand — the same guarantee v1 makes, extended across a loop.

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
│   │   ├── manifest.go            # Manifest struct (+ tooled_flags), Render(mode) → exec.Cmd spec
│   │   ├── builtin.go             # compiled-in manifests (pi, claude, gemini, agy, ...)
│   │   ├── registry.go            # name → manifest, with override merge
│   │   ├── executor.go            # run manifest, feed stdin, capture stdout, timeout
│   │   ├── parse.go               # parseOutput() pipeline (§12.9)
│   │   └── *_test.go
│   ├── prompt/
│   │   ├── system.go              # buildSystemPrompt() (style learn, anti-reuse)
│   │   ├── examples.go            # fetch last 20, multi-line detection
│   │   ├── payload.go             # assemble user payload (instruction + diff)
│   │   ├── planner.go             # planner system prompt + JSON contract (§17.5)
│   │   ├── stager.go              # stager task prompt (§17.6)
│   │   ├── arbiter.go             # arbiter prompt + JSON contract (§17.7)
│   │   └── *_test.go
│   ├── git/
│   │   ├── git.go                 # Git wrapper interface
│   │   ├── plumbing.go            # WriteTree, CommitTree, UpdateRefCAS, RevParseHEAD
│   │   ├── diff.go                # StagedDiff() with caps, exclusions, binary filtering (FR3a–c)
│   │   ├── binary.go              # detectNonText() via numstat + extension denylist; placeholder line
│   │   ├── tree.go                # RevParseTree, TreeDiff (tree-to-tree concept diff), ReadTree, StatusPorcelain (v2)
│   │   ├── log.go                 # RecentMessages(), RecentSubjects(), CommitCount()
│   │   ├── stage.go               # AddAll(), HasStagedChanges(), StagedFileCount()
│   │   └── *_test.go              # uses a temp repo + real git binary
│   ├── generate/
│   │   ├── generate.go            # CommitStaged(ctx, cfg) — the single-commit orchestrator (§13.1–5)
│   │   ├── dedupe.go              # duplicate-subject check + retry
│   │   ├── rescue.go              # rescue protocol (FR43–FR45)
│   │   └── *_test.go              # integration with a stub provider
│   ├── decompose/                 # v2 multi-commit pipeline (§13.6)
│   │   ├── decompose.go           # Decompose(ctx, cfg) — orchestrates plan→stage→gen→commit→arbitrate
│   │   ├── roles.go               # per-role provider/model resolution (§16.4, FR-R1–R5)
│   │   ├── planner.go             # planner agent call + JSON parse/retry
│   │   ├── stager.go              # tooled stager agent call (mode=tooled); snapshot/overlap scheduling
│   │   ├── arbiter.go             # arbiter agent call + amend/new/rebuild resolution (stagecoach does git)
│   │   ├── chain.go               # linear-chain rebuild for mid-chain amend (FR-M10)
│   │   └── *_test.go              # integration with stub planner/stager/arbiter + a temp repo
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
│   ├── agy.toml                   # Antigravity CLI (experimental; §12.5.1)
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
// stages first (or uses AutoStageAll in the CLI layer). Used for the
// single-commit path and as the per-concept primitive inside Decompose.
func GenerateCommit(ctx context.Context, opts Options) (Result, error)

// DecomposeOptions configures the multi-commit pipeline (§13.6).
type DecomposeOptions struct {
    Options                  // embedded: Provider/Model/DryRun/Timeout apply to the MESSAGE role
    Count          int       // 0 => auto-decompose (planner decides); >0 => force exactly Count commits
    Single         bool      // true => bypass planner, force one CommitStaged (--single)
    MaxCommits     int       // safety cap (default 12); refuses more unless Count forces it
    Planner        RoleModel // planner role provider/model (zero => global default)
    Stager         RoleModel // stager role provider/model (zero => global default)
    Arbiter        RoleModel // arbiter role provider/model (zero => global default)
}

type RoleModel struct { Provider, Model string }

// DecomposeResult is the outcome of Decompose: the ordered commits created this run.
type DecomposeResult struct {
    Commits []Result // one per concept that produced a commit (empty concepts skipped)
    Amended int      // number of those commits the arbiter folded leftovers into
    Provider string   // resolved MESSAGE provider (for display)
}

// Decompose turns a dirty, un-staged working tree into N logically-coherent
// commits (§13.6). It activates the planner→stager→message→arbiter pipeline;
// it is a NO-OP (delegates to GenerateCommit) when Single is true or Count==1.
// Caller must ensure nothing is staged (the CLI gates on HasStagedChanges).
func Decompose(ctx context.Context, opts DecomposeOptions) (DecomposeResult, error)
```

The CLI's `main.go` is essentially: parse flags → decide path (`GenerateCommit` if something staged or `--single`; else `Decompose`) → print result. The single-commit path stays a thin shell over `GenerateCommit`; the multi-commit path composes `GenerateCommit`'s primitives per concept (§13.6).

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
| `--provider <name>` | `STAGECOACH_PROVIDER` | `stagecoach.provider` | auto-detected | Provider (agent platform) to use — **global default for all roles** (§16.4). |
| `--model <name>` | `STAGECOACH_MODEL` | `stagecoach.model` | per-manifest `default_model` | Model override — `inference/model` form for multi-backend providers (FR-R5b) — **global default for all roles** (§16.4). |
| `--reasoning <level>` | `STAGECOACH_REASONING` | `stagecoach.reasoning` | off (planner: high) | Reasoning level (`off\|low\|medium\|high`) — **global default for all roles** (FR-R6). |
| `--config <path>` | `STAGECOACH_CONFIG` | — | resolved path | Path to a config file (overrides discovery). |
| `--timeout <dur>` | `STAGECOACH_TIMEOUT` | `stagecoach.timeout` | `120s` | Generation timeout. |
| `--all`, `-a` | — | — | — | `git add -A` before snapshotting, even if something is staged. |
| `--no-auto-stage` | — | — | — | If nothing is staged, exit instead of auto-staging. |
| `--commits <N>` | — | `stagecoach.commits` | `0` (auto) | Force exactly N commits when nothing is staged (skips the planner's count decision; §13.6.1). `1` ≡ `--single`. |
| `--single`, `--no-decompose` | — | — | — | Bypass decomposition; force the v1 single-commit auto-stage-all behavior. |
| `--max-commits <N>` | — | `stagecoach.max_commits` | `12` | Safety cap on auto-decompose commit count. |
| `--planner-provider <p>` / `--planner-model <m>` | `STAGECOACH_PLANNER_PROVIDER` / `_MODEL` | `stagecoach.role.planner.*` | global | Per-role override for the decomposition planner (§16.4). |
| `--stager-provider <p>` / `--stager-model <m>` | `STAGECOACH_STAGER_PROVIDER` / `_MODEL` | `stagecoach.role.stager.*` | global | Per-role override for the (tooled) staging agent. |
| `--arbiter-provider <p>` / `--arbiter-model <m>` | `STAGECOACH_ARBITER_PROVIDER` / `_MODEL` | `stagecoach.role.arbiter.*` | global | Per-role override for the leftover arbiter. |
| `--dry-run` | — | — | `false` | Generate and print the message; do not commit. |
| `--verbose`, `-v` | `STAGECOACH_VERBOSE` | — | `false` | Print resolved command, raw output, retries. |
| `--no-color` | `STAGECOACH_NO_COLOR` | — | TTY-aware | Disable color. Respects `NO_COLOR`. |
| `--version` | — | — | — | Print version and exit. |
| `--help`, `-h` | — | — | — | Help. |

### 15.3 Subcommands

- **`stagecoach providers list`** — List all known providers (built-in + user). Mark detected (on `$PATH`) vs not. Show the resolved default (highest-priority *installed* built-in per FR-D1's order: pi, opencode, cursor, agy, gemini, qwen-code, codex, claude).
- **`stagecoach providers show <name>`** — Print the fully-resolved manifest as TOML.
- **`stagecoach config init`** — Bootstrap a **populated, working** config (auto-detects the default provider and writes its per-role models); `--provider <name>` to target one, `--force` to overwrite, `--template` for the inert reference (§9.17).
- **`stagecoach config path`** — Print the resolved global config path.
- **`stagecoach config upgrade`** — Rewrite an existing config to the current schema version in place (§9.17, FR-B5).

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

# Use a specific provider + model for one commit (claude is single-backend; bare model).
stagecoach --provider claude --model sonnet

# A multi-backend provider carries its inference backend as a model prefix (FR-R5b).
stagecoach --provider pi --model zai/glm-5.2

# Set a per-repo default (persisted in the repo's git config).
git config stagecoach.provider pi
git config stagecoach.model zai/glm-5.2

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

# --- multi-commit decomposition (v2; §13.6) ---
# Dirty tree, nothing staged: auto-decompose into as many commits as warranted.
stagecoach

# You know it's three logical changes — skip the "how many?" step.
stagecoach --commits 3

# Force the old one-commit behavior (or equivalently --commits 1).
stagecoach --single

# Route planning to a big context, keep messages on the fast default.
stagecoach --planner-model gemini-3.1-pro --planner-agent agy

# Per-repo: plan with Antigravity's quota, messages with pi's.
#   .stagecoach.toml:
#     [defaults]
#       agent    = "pi"
#       provider = "zai"
#     [role.planner]
#       agent    = "agy"
#       model    = "gemini-3.1-pro"
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

Higher wins. Agent-platform manifests merge field-by-field (a user override that sets only `default_model` leaves all other fields from the built-in manifest intact).

**`config_version` is metadata, not a precedence layer.** Every config file carries `config_version = <int>`; on load, stagecoach compares it to its compile-time `CurrentConfigVersion` and emits an advisory staleness warning (or points to `config upgrade`) per §9.17 FR-B4/B5. It does not participate in value resolution.

### 16.2 Full config file example

```toml
# ~/.config/stagecoach/config.toml  (config_version 3)

[defaults]
provider = "pi"          # the AGENT PLATFORM (pi, claude, opencode, …)
model    = "zai/glm-5.2"  # inference provider is a slash-PREFIX for multi-backend providers (FR-R5b); bare for single-backend
reasoning = "off"         # off|low|medium|high; planner defaults to high (FR-R6)
timeout  = "120s"
auto_stage_all = true
verbose  = false

[generation]
max_diff_bytes      = 300000
max_md_lines        = 100
max_duplicate_retries = 3
output              = "raw"     # raw | json
strip_code_fence    = true
subject_target_chars = 50
binary_extensions   = []        # extra non-text extensions to filter (FR3a; merges with built-in denylist)
max_commits         = 12        # safety cap on auto-decompose (FR-M4)

# Override a built-in provider/agent platform (field-merged with the built-in manifest).
[provider.pi]
default_model = "gpt-5.4-mini"        # the platform's default model (no default_provider field in v3 — the prefix lives on `model`)
# tooled_flags let this provider serve the STAGER role; omit to exclude it.
# tooled_flags = ["--allowed-tools", "Bash(git:*),Read,Edit", "--approval-mode", "auto"]

# Define a brand-new provider/agent platform (§12.8).
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
    provider = pi          # the agent platform
    model = zai/glm-5.2    # inference provider is the slash-prefix (FR-R5b)
    timeout = 90
    autoStageAll = true
```

Read with `git config --get stagecoach.provider`, etc. Booleans via `git config --bool`. This composes naturally with the author's existing `git commit-pi` alias habit and with `git config --local` vs `--global`.

### 16.4 Per-role provider/model configuration (v3; → G12, FR-R1–R6)

The four roles — **planner, stager, message, arbiter** (§13.6.2) — each resolve their **provider** (agent platform), **model** (inference-provider-prefixed for multi-backend providers, FR-R5b), and **reasoning** level (FR-R6) independently. A single global default covers all of them; per-role tables override the fields you care about.

**Resolution for a role's provider/model/reasoning (highest wins), applied independently per field:** CLI flag → env → `[role.<role>]` config → `[defaults]` config (the global) → built-in manifest default. The globals are `[defaults].provider` / `[defaults].model` / `[defaults].reasoning` (i.e. `--provider`/`--model`/`--reasoning`, `STAGECOACH_PROVIDER`/`STAGECOACH_MODEL`/`STAGECOACH_REASONING`). On the single-commit path the only active role is `message`, so setting just the globals is exactly equivalent to v1 — back-compatible.

```toml
# One setting for everything: set only [defaults].
[defaults]
provider = "pi"
model    = "zai/glm-5.2"   # multi-backend: inference provider is the prefix (FR-R5b)
reasoning = "off"       # global; planner overrides to high below (FR-R6)

# Granular: route planning to a large-context provider, leave the rest on the global.
[role.planner]
provider = "agy"        # Antigravity quota for the big-context reasoning (single-backend → bare model)
model    = "gemini-3.1-pro"
reasoning = "high"      # override the global off — planning benefits from reasoning (FR-R6)

[role.stager]           # tooled provider that runs git; needs tooled_flags in its manifest
provider = "agy"
model    = "gemini-3.5-flash"

[role.message]          # bare commit-message role — inherits [defaults] (pi)
# (omit to inherit)

[role.arbiter]          # bare leftover arbiter — inherits [defaults]
# (omit to inherit)
```

Env: `STAGECOACH_<ROLE>_{PROVIDER,MODEL,REASONING}` (e.g. `STAGECOACH_PLANNER_MODEL`). Flags: `--<role>-provider` / `--<role>-model` / `--<role>-reasoning` (**all four roles, including `message`**). **Model strings are provider-specific** (FR-R5): a role's `model` is interpreted by *that role's* resolved provider's manifest; for multi-backend providers it is `inference/model` (FR-R5b). A role routed to a provider whose manifest has empty `tooled_flags` cannot serve as the **stager** (it lacks a safe tooled profile); stagecoach rejects that combination up front. **A bare model (no `/`) on a `provider_flag` provider like pi is a hard error** (FR-R5b) — never silently rendered as a bare `--model`.

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

### 17.5 Planner prompt (v2; §13.6.2, FR-M3)

The planner is **bare** and receives the full working-tree diff (with binary placeholders) plus the §17.1 style examples. Its job: decide whether this changeset is one commit or many, partition accordingly, and — only if one — produce the message. Because the output is structured (a list), a **JSON contract** is justified here (unlike free-form commit messages, §17.4), with a robust parse + one retry.

System prompt (sketch):
```
You are a commit-planning assistant. Given a diff of un-staged changes, decide whether they
form ONE coherent commit or SEVERAL, and partition them into logical units.

Rules:
- Prefer FEWER commits. A single commit is correct unless the changes clearly span
  unrelated concerns. Do not manufacture tiny commits.
- Each commit must be independently meaningful and reviewable. Group tightly-coupled
  changes (a function + its test, a refactor + its callers) together.
- Respect dependencies: if change B depends on change A, A comes first.
- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.

Respond with ONLY JSON, no prose, no code fences:
{"count": <int>, "single": <bool>, "commits": [{"title": "<short concept>", "description": "<precisely which files/hunks belong here, by path>"}, ...]}
- If single is true, set count=1 and ALSO include "message": "<the full commit message>".
- The "description" must be specific enough that a staging agent can find the exact changes.

<style examples>
```

User payload: `"Decompose these un-staged changes into commits:\n\n<diff>"`. Forced-count mode prepends: `"Produce EXACTLY N commits from these changes (do not reconsider the count):"`. Retry instruction (unparseable JSON): `"Respond with ONLY the JSON object described, no other text."`

### 17.6 Stager task prompt (v2; §13.6.2, FR-M5)

The stager is **tooled** (git access, repo-scoped). It receives one concept's title + description (from the planner) as a *task*, not a system-prompt-and-diff. It must stage exactly that concept's changes and stop.

Task prompt (delivered as the user payload; system prompt minimal/empty):
```
Stage, but do NOT commit, all changes in this repository that match this concept:

<title>
<description>

Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply
only the relevant hunks via `git apply --cached`). Stage ONLY changes belonging to this
concept; leave unrelated changes unstaged. Do not commit, do not amend, do not push, do not
modify file contents — only update the index. When done, reply with the list of paths you
staged and stop.
```

The hard guardrails (no commit/amend/push/ref-mutation) are restated in the prompt AND enforced structurally: the stager runs with a git-scoped tool profile (`tooled_flags`, §12.1) and stagecoach performs every ref operation itself. A stager that nevertheless attempts a commit is a best-effort concern — it cannot move stagecoach's refs (stagecoach owns those via `update-ref`), and the user-visible HEAD only advances through stagecoach's CAS.

### 17.7 Arbiter prompt (v2; §13.6.5, FR-M9)

The arbiter is **bare** and runs only if the working tree is non-empty after the loop. It receives the commits made this run (SHA + subject + file list each) and a diff of the remaining changes. It returns a target SHA or null.

System prompt (sketch):
```
You reconcile leftover changes into commits that were just made. You are given the commits
created this run (with their messages and changed files) and a diff of changes that were not
included in any of them.

Decide: do these leftovers logically belong WITH one of those commits, or do they warrant a
NEW commit?
- Choose an existing commit only if the leftovers are part of the SAME logical change.
- When in doubt, prefer a NEW commit (return null) — never force a fit.
- You may only target a commit from the provided list.

Respond with ONLY JSON: {"target": "<sha from the list>"} or {"target": null}.
```

User payload: the commit list + the leftover diff. Stagecoach performs all resulting git (FR-M10); the arbiter only returns the decision.

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
| Planner unparseable / fails (v2) | pre-staging | surface error; nothing snapshotted yet | 1 |
| Decompose would exceed `max_commits` (v2) | pre-staging | error: raise `--max-commits` / `--commits` | 1 |
| Stager stages nothing / exits non-zero twice (v2) | mid-loop | skip concept (no empty commit); log; continue | 0 |
| `message[i]` fails mid-loop (v2) | mid-loop | rescue **for concept i only** (§13.6.6); prior commits stand | 3 |
| Arbiter returns invalid/unknown target (v2) | post-loop | default to a NEW commit (null) | 0 |

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

**Multi-commit variant (v2; §13.6.6).** When a single concept fails mid-loop, the rescue is scoped to that concept's frozen `tree[i]`: print `tree[i]`, its parent (`newSHA[i-1]`), and the same `commit-tree | update-ref` recipe. Already-published commits 0..i-1 are final and untouched; any concepts whose staging completed remain staged for the user to finish. The arbiter is not run when the loop aborts via rescue.

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
- **The stager is the one tooled exception (v2).** The per-concept staging agent runs with git tools on (§11.5, §13.6.2). Its toolset is **scoped** — a git/read/edit allowlist expressed via `tooled_flags` — and it is instructed (and structurally constrained) to only update the index (`git add`-class ops); it cannot commit, amend, or push, because stagecoach owns every ref mutation via `update-ref`. The unscoped `--dangerously-skip-permissions` bypass is **never** used to achieve this; a provider whose only non-interactive tool-execution path is the unscoped bypass cannot serve as a stager (its `tooled_flags` stays empty).

---

## 20. Testing and QA strategy

### 20.1 Layers

1. **Unit — pure functions.** `parseOutput` (table-driven: raw, fenced, json, json-in-prose, fallback), command rendering (per provider, golden files), prompt construction (style-learning, multi-line detection, anti-reuse text), duplicate detection, config precedence resolution.
2. **Unit — git wrapper, with a real git binary.** Each `internal/git/*` test creates a temp directory, `git init`, stages known content, and asserts on `WriteTree`/`CommitTree`/`UpdateRefCAS`/`StagedDiff`/`RecentMessages`. These are fast (git is fast) and catch real plumbing regressions.
3. **Integration — full flow with a stub provider.** A fake agent: a tiny Go binary (or shell script) that reads stdin and writes a canned message to stdout. Drives `generate.CommitStaged` end-to-end and asserts the resulting commit exists in the repo with the right tree, parent, and message. Covers: success, duplicate-retry-then-success, parse-failure-then-rescue, timeout, CAS failure (simulate by moving HEAD mid-test), root commit, auto-stage-all. **v2 adds a parallel stub suite for `decompose.Decompose`**: stub planner (canned JSON partition), stub stager (a scripted `git add` of named paths — no real tooled agent in CI), stub arbiter (canned target/null). Covers: auto-decompose into N, `--commits N` forced count, single-shortcut, empty-concept skip, mid-loop rescue, arbiter new/tip-amend/mid-chain-rebuild, binary-placeholder propagation, the `stager[i+1] ∥ message[i]` overlap (assert tree[i] is frozen before stager[i+1] runs via interleaving checks), and **concurrent-change exclusion**: the harness writes a new file to the working tree mid-run and asserts it lands in no commit and remains in the working tree post-run (FR-M1b/M1c).
4. **Integration — real agents (opt-in, not in CI).** A `//go:build integration_real` suite that invokes the actual `pi`/`claude`/etc. if installed and `STAGECOACH_RUN_REAL=1`. Used manually before releases; skipped in CI.

### 20.2 Property/invariant tests

- **Idempotent index:** after any failure path, `git status` output is identical to before the run (no index mutation). Asserted by snapshotting `git diff --cached --name-only` before and after.
- **Atomic HEAD:** after a CAS failure, `git rev-parse HEAD` is unchanged.
- **Snapshot immutability:** `git cat-file -p <TREE_SHA>` is stable across the run regardless of subsequent staging.
- **Concept isolation (v2):** for every commit in a decompose run, `diff-tree <newSHA[i]>` (vs its parent) equals exactly the concept's files — no leakage from sibling concepts. Asserted by comparing each commit's file set to the stager's recorded paths.
- **Loop index cleanliness (v2):** after a fully-successful decompose run, `git status --porcelain` is empty (arbiter not needed) OR the arbiter has reconciled every remaining entry (arbiter ran) — never a partial leftover.
- **Mid-chain amend fidelity (v2):** after an arbiter-driven mid-chain rebuild, the rebuilt chain's non-target commits are byte-identical (same tree, same message) to the originals, and only the target commit's tree grew by the leftover set.
- **Start-of-run freeze (v2):** a file created or modified in the working tree *after* decomposition begins appears in no commit of the run and remains in the working tree afterward. Asserted by writing a sentinel file between decompose start and completion and checking it is untracked/unchanged across every produced commit (FR-M1b/M1c).

### 20.3 Coverage target

≥85% on `internal/git`, `internal/provider`, `internal/generate`, `internal/config`. Lower bar for `internal/ui` (hard to test, low risk). Enforced in CI with a coverage gate.

### 20.4 CI matrix

GitHub Actions: build + test on `{linux, macos, windows} × {amd64, arm64}`, Go `1.22` and `1.23`. `golangci-lint`. `govulncheck`. Release on tag via goreleaser.

### 20.5 End-to-end scenario harness (strongly encouraged)

The concurrency and routing invariants above are easy to specify, easy to regress, and — as repeated field discoveries have shown — easy to break silently (unit tests with stub agents cannot reach them). Maintain a throwaway-repository harness (a script or a `//go:build e2e` test) that, **per scenario, creates a fresh `git init` temp repo, seeds it, runs `stagecoach`, and asserts the resulting history** — driving the real agent where feasible (the `integration_real` suite) or a stub. **Every bug found in the wild becomes a scenario here.** The current must-cover set:

- nothing staged, N unrelated files → N commits (auto *and* `--commits N`);
- exactly one file changed → single commit, **no planner call** (FR-M2b);
- a file created/modified by a concurrent process mid-run → excluded from every commit, left in the working tree (FR-M1b/M1c);
- a model pinned on a multi-backend agent with no inference provider → **hard error**, not silent empty output (FR-R5b);
- arbiter leftover reconciliation (new commit / tip amend / mid-chain rebuild);
- rescue mid-loop; CAS abort (HEAD moved concurrently).

This harness is the regression net for the behaviors that only manifest against a real repo and (ideally) a real agent — the gap that let the planner-empty-output and concurrent-file bugs ship.

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
6. Configure your agent (`providers list` → set `stagecoach.provider`; for multi-backend providers prefix the model, e.g. `stagecoach.model zai/glm-5.2`).
7. The snapshot workflow (§13.4 diagram) — the "stage while it thinks" payoff.
8. Full CLI + config reference (link to docs).
9. Adding a new agent (§12.8) — the contributor hook.
10. FAQ / "Stagecoach is not for you if…"

---

## 22. Risks, assumptions, dependencies

### 22.1 Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Agent CLI surfaces change (flags renamed/removed). | Medium | Medium (an agent breaks). | Manifests are config-overridable without a release; `providers show` aids debugging; pin known-good manifest versions in docs; community can ship fixes. |
| An agent's raw output is unreliable (preambles, fences). | Medium | Low (retry handles it). | Robust parse pipeline + retry; JSON fallback per-provider. |
| Large diffs exceed an agent's context or arg limits. | Low | Medium. | Diff cap (300 KB default, configurable); stdin delivery avoids arg limits; surface a clear "diff truncated" notice. |
| `update-ref` CAS semantics misunderstood. | Low | High (data integrity). | Exhaustive tests (§20.2); never use force-update; rescue on failure. |
| Users expect multi-commit in v1. | Medium | Low (disappointment). | README states v1 = single commit clearly; roadmap links to v2. |
| Agent invokes tools despite bare flags (e.g., a model "reads" a file). | Low | Low (slower, maybe wrong message). | Bare flags disable tools; output is still just a message; worst case is a slightly slower or odd commit, never repo damage. |
| Codex / Cursor manifests drift or the two `# TO CONFIRM` items fail (§12.7). | Low–Medium | Low (safe read-only profile either way). | Flag surfaces verified against real `--help`; two residual checks carried inline. `experimental` flag remains available for any future docs-only provider; manifests are config-overridable without a release. |
| **`agy` non-TTY stdout drop (issue #76; §12.5.1)** — `agy -p` may emit no stdout when spawned as a subprocess, breaking all `agy` roles. | High (for `agy` specifically) | High (provider unusable until resolved). | Block `agy` behind `experimental` + a PTY-shim workaround; gate the bare-roles path on a verified fix. No other provider is affected. |
| **Stager mutates the working tree/index (v2)** — the only tooled agent could stage the wrong hunks or touch unrelated files. | Medium | Medium (messy index, not history corruption — it cannot commit). | Scoped git toolset (`tooled_flags`); instruction guardrails; stagecoach owns all refs; arbiter + empty-concept skip contain mistakes; user can always inspect `git status` before any commit lands. |
| **Mid-chain amend rebuild is wrong (v2)** — reconstructing the chain for a non-tip arbiter target could misorder or drop a commit. | Low–Medium | High (rewrites just-made history). | Deterministic `read-tree`/`write-tree`/`commit-tree` reconstruction owned entirely by stagecoach (no interactive rebase); covered by the §20.2 "mid-chain amend fidelity" invariant test; ambiguous arbiter output defaults to a safe new commit. |
| **Planner context overflow (v2)** — a very large working-tree diff exceeds the planner's context window. | Low | Medium (planner fails pre-staging). | Same diff cap as v1 + binary filtering reduces payload; surface a clear "diff too large; use `--commits` or stage manually" error; no partial state (planning precedes all staging). |
| **Concurrency race on the index (v2)** — stager[i+1] and snapshot[i] could overlap incorrectly. | Low | High (wrong commit contents). | Enforced ordering: snapshot[i] is taken synchronously before stager[i+1] starts (§13.6.3 invariant 1); concept diffs are tree-to-tree, not index-vs-HEAD, so they are immune to the race by construction. |

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
- **Decomposition** — the v2 flow (§13.6) that turns a dirty, un-staged working tree into N coherent commits via the snapshot machinery.
- **Concept** — one logical unit output by the planner (a title + a description of which changes belong together); becomes one commit.
- **Planner / stager / message / arbiter** — the four agent roles in decomposition (§13.6.2): plan the partition; stage one concept (tooled); write one commit message; reconcile leftovers.
- **Bare mode vs tooled mode** — bare = tools off, text-in/text-out (planner/message/arbiter); tooled = git tools on, repo-scoped (stager only). (§11.5)
- **`tooled_flags`** — the manifest field (§12.1) that makes a provider able to serve as the stager; empty means the provider is bare-roles-only.

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
↳ Generating with zai/glm-5.2 in pi…
↳ Created abc1234  feat(auth): accept SAML tokens for enterprise login
   M  src/login.go
   A  src/login_test.go
```

### B.2 Stage-while-generating

```
# pane A
$ git add src/a.go && stagecoach
↳ Generating with sonnet in claude…   (takes 8s)

# pane B, during those 8s
$ git add src/b.go src/c.go          # these are NOT in the commit below

# pane A resumes
↳ Created def5678  refactor: extract auth helper
# git status now shows src/b.go, src/c.go as staged-for-next-commit
```

### B.3 Dry run

```
$ stagecoach --dry-run
↳ Generating with zai/glm-5.2 in pi…
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
↳ Generating with gemini-3.1-pro in gemini…
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
| **agy** | `agy` | stdin | `-p` | `-m` | *(prepend)* | `--approval-mode default` | raw (experimental; §12.5.1) |
| opencode | `opencode run` | positional | — | `-m` (`provider/model`) | *(prepend)* | — | raw |
| codex | `codex exec` | positional | (exec) | `-m` | *(prepend)* | `--sandbox read-only --ask-for-approval never` | raw |
| cursor | `agent` | positional | `-p` | `--model` | *(prepend)* | `--mode ask --trust` | raw |

> **`tooled_flags` (v2, not shown above):** each provider's stager profile is provider-specific and carried as `tooled_flags` (§12.1) — a git/read/edit allowlist + a non-interactive approval mode. Providers with empty `tooled_flags` (notably **`agy`** until §12.5.1.1 item 4 clears, and **opencode**) cannot serve as the stager; they can still serve the bare roles (planner/message/arbiter).

## Appendix E — Open questions (to resolve before/during v1 implementation)

1. **Gemini delivery:** confirm `stdin` accepts a ~300 KB payload without truncation; if not, fall back to positional and document the diff cap as the mitigation.
2. **Claude tools-disable:** confirm `--tools ""` fully suppresses tool use in `-p` mode for current Claude Code versions; if a model still "thinks" about tools, add `--disallowed-tools "*"` (verify syntax).
3. **opencode system prompt:** decide whether to (a) prepend to payload (simple, v1), or (b) document an `--agent` workflow where users define a `stagecoach` agent persona in `opencode.json` (nicer, v1.1).
4. **Codex / Cursor (mostly resolved):** flag surfaces verified against real `--help` (§12.7). Two residual confirmations carried as inline `# TO CONFIRM`: (a) `codex exec` writes the final answer to stdout and exits 0; (b) cursor `--mode ask` wins over `-p`'s default full-tools profile. Both are expected from the docs and are quick to confirm during the first real run.
5. **`.stagecoach.toml` trust:** finalize the v1.1 hardening (restrict repo-local overrides to non-`command` fields unless explicitly trusted).
6. **Public API stability:** decide whether `pkg/stagecoach.GenerateCommit` is v1-stable or marked experimental until v1.1. Recommendation: ship it, mark it `// Stable as of v1.0`, keep the `Options` struct additive-only.
7. **`agy` non-TTY stdout (§12.5.1.1 item 1, blocking):** confirm whether a PTY-shim makes `agy -p` emit stdout reliably under stagecoach's subprocess model, or wait for upstream issue [#76](https://github.com/google-antigravity/antigravity-cli/issues/76). Gates all `agy` roles.
8. **`agy` tooled (stager) flags (§12.5.1.1 item 4):** determine the exact non-interactive, git-scoped, non-bypass flag combination. Gates `agy` (and any provider) as a stager.
9. **Mid-chain amend plumbing (§13.6.5):** finalize the exact `read-tree`/`write-tree`/`commit-tree` reconstruction sequence (what `read-tree` base for each j, how leftovers fold in at the target) during implementation planning; prove via the §20.2 fidelity invariant.
10. **Stager toolset scope per provider:** pin the minimal allowlist each tooled profile needs (git add, read, edit, apply) so no provider's stager can do more than stage.
11. **Verify current model names per provider (FR-D5, blocking for defaults):** confirm the live flagship/mid/fast model token for each of pi, opencode, cursor, agy, gemini, qwen-code, codex, claude (e.g. is it `gpt-5.4` or newer? `gemini-3.1-pro` or newer? Claude Opus/Sonnet/Haiku current versions? `qwen3-coder-plus` current?). Record names + verification date in the manifest source.
12. **pi OpenAI routing:** determine which of pi's current sub-providers routes to an OpenAI model (openrouter? a native openai sub-provider?) so pi's shipped `backend/model` default is wired end-to-end; if none is universal, ship pi's model empty and let `config init` prompt for the prefix.
13. **Config `upgrade` mechanics:** finalize how `config upgrade` preserves user values vs. comments-out renamed keys (FR-B5) — keep it simple (no value-type migration) until a real rename occurs.

## Appendix F — Decision log (key calls and why)

- **Shell out to agents, not call APIs.** Because coding-plan quotas are unreachable over the public API. This is the product. (§2.2, §4.3)
- **Go, not TS.** Distribution fit (Homebrew/binary) matches the lazygit/gh audience; zero runtime dependency. (§2.3)
- **Raw output default, not JSON.** Removes the double-quote constraint and fragile parsing; JSON remains an option per-provider. (§17.4)
- **Shells out to real `git`, no go-git.** Matches the proven reference; identical semantics; smaller dependency surface. (§22.3)
- **v1 = single commit; multi-commit is v2.** Keeps v1 shippable; the snapshot foundation makes v2 a loop over v1. (§10.1, §11.3)
- **Auto-stage-all on by default in v1.** Per author's explicit request; the quickest path to a checkpoint commit; `--no-auto-stage` escapes it. (§9.4, FR16–FR20)
- **Multi-commit promoted from v2 into the core spec (this revision).** The snapshot foundation made it a loop over v1; the dirty-tree-and-nothing-staged state is the natural trigger; `--commits N` / `--single` give the user count control + an escape hatch. (§13.6, §10.3)
- **Only the stager is tooled; everything else stays bare.** One new manifest field (`tooled_flags`) instead of a second schema; keeps the bare-mode safety story intact for planner/message/arbiter. (§11.5, §12.1)
- **Concept diffs are tree-to-tree, not index-vs-HEAD.** This is what makes `stager[i+1] ∥ message[i]` safe — the diff is immune to concurrent staging and to commits landing. (§13.6.3 invariant 2)
- **Per-role models with a global fallback.** Different tasks warrant different agents, but one global model must still cover everything; `[defaults]` is the fallback, `[role.*]` is opt-in granularity, and it stays back-compatible with v1. (§16.4)
- **Binaries replaced with filename+status placeholders, never dropped silently.** The decomposition planner needs to know a binary asset changed to group it correctly. (§9.1, FR3b)
- **`agy` ships experimental behind its `# TO CONFIRM` block.** Honest about the non-TTY stdout bug (#76) rather than pretending the manifest is verified; matches the §12.7.2 progressive-verification ethos. (§12.5.1)
- **Decoupled defaults from the author's z.ai subscription.** pi no longer ships `glm-*`/`zai`; defaults are account-agnostic and the z.ai setup is a documented personal override. (§9.16 FR-D2, §12.3)
- **Cascading provider priority (pi → opencode → cursor → agy → gemini → codex → claude).** Open/self-hostable harnesses first; closed subscription CLIs last; highest-priority *installed* one wins. (§9.16 FR-D1)
- **Tier-based per-role defaults (smart/mid/fast), materialized by a populated `config init`.** Each role is sized to its job (stager mid not fast — it needs tool-use; message fast — it's bare), and the bootstrap config writes them uncommented so it works out of the box. (§9.16, §9.17)
- **Model defaults are research-driven and refreshable, not pinned from stale knowledge.** The implementing agent verifies current names per provider; a future automated refresh process keeps them current. (§9.16 FR-D5)
- **Config schema versioning + advisory staleness warning.** Simple integer version + a warning + `config upgrade`; no auto-migration (no existing users). (§9.17 FR-B4/B5)
- **Manifests are config-overridable, compiled-in as defaults.** Decouples "support a new agent" from "cut a release." (§12.1, §12.8)

---

*End of document. Target length: comprehensive PRD + technical specification exceeding 20,000 tokens, covering product positioning, competitive analysis, functional requirements, architecture, the provider-manifest system, the snapshot-based atomic-commit core, CLI/config reference, prompt engineering, error/rescue design, security, testing, distribution, risks, and appendices including a porting map from the originating `commit-pi` script.*
