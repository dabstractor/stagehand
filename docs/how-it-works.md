# How Stagecoach works

Architecture overview of the Stagecoach pipeline: snapshot-based commit creation, stage-while-generating, the safety and rescue protocol, and prompt engineering. This is the cross-cutting "architecture overview" — it ties together the git plumbing, orchestrator, rescue protocol, and prompt assembly.

## The snapshot-based flow

### Why not `git commit`

Stagecoach does not use `git commit`. The standard `git commit` reads the **live** index and mutates `HEAD` — it locks the repo state for the duration. If you stage a file while a commit is in progress, that file may end up in the commit unexpectedly.

### The plumbing alternative

Instead, Stagecoach uses three low-level git plumbing commands:

1. `git write-tree` — freezes the current index into an immutable tree object (the **snapshot**). The index is never reset.
2. `git commit-tree` — creates a dangling commit object from the frozen tree (no ref mutation).
3. `git update-ref HEAD` (compare-and-swap) — advances `HEAD` to the new commit atomically. If `HEAD` changed meanwhile, the CAS fails and the commit is aborted.

### Snapshot invariants

These four invariants hold for every run (PRD §13.3):

1. **Frozen content** — the committed content is exactly what was staged at `write-tree` time. Nothing added afterward can affect it.
2. **Later-staged files stay staged** — the index is never reset. Files staged during generation remain staged for the next run.
3. **Atomic and safe** — `update-ref CAS` is the only ref mutation. A failed generation leaves the repo byte-for-byte unchanged (only orphan tree/commit objects are left for `git gc`).
4. **Overlap-able latency** — generation time is dead time only if the user does nothing. With the snapshot, the user can stage the next batch while the current message generates.

## Stage-while-generating

The snapshot decouples "what's committed" from "what's staged now." The user can keep working while Stagecoach generates:

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

Generation time is no longer dead time. The in-flight commit only ever contains what was staged when it started.

## Multi-commit decomposition

v2.0's headline feature: run `stagecoach` with a dirty working tree and nothing staged, and it automatically splits the changes into a sequence of logically-coherent commits — one per concept — using a four-role agent pipeline.

### Trigger

Decompose activates when **nothing is staged**, **auto-stage-all is on** (the default), and the user has **not opted out** (`--single`, `--no-decompose`, or `--commits 1`). If something is already staged, the single-commit path runs unchanged. `--dry-run` also forces the single-commit preview (decompose commits, so dry-run honors the single preview).

**Defense-in-depth (FR-M1e).** Decompose re-asserts the empty-index precondition at its entry: if a stale or buggy trigger ever routes to it with a non-empty index, it fails loudly — naming the offending staged paths and pointing to `git reset` (unstage, then re-run) or `stagecoach --single` (commit the hand-staged index as one) — instead of silently sweeping them into the start-of-run freeze. The single-commit escape hatch (`--single`, `--commits 1`) is checked first and is unaffected, since it handles a hand-staged index normally.

### The four roles

| Role | Mode | Job | Output |
|------|------|-----|--------|
| **planner** | bare | Analyze the full working-tree diff; decide how many commits and what each covers | JSON `{count, single, commits:[{title,description,files}], message?}` |
| **stager** | tooled | Stage one concept's subset of files (`git add`, hunk-level staging) | Mutates the index; exits 0 |
| **message** | bare | Generate a commit message from the concept diff | Raw commit message text |
| **arbiter** | bare | Decide which just-made commit any leftover changes belong to, or create a new commit | JSON `{target: "<sha>"\|null}` |

### Pipeline flow

```text
                 nothing staged + dirty working tree
                              │
                              ▼
            ┌────────────┐   T_start diff (binary placeholders)
            │  planner   │◀──── + style examples
            │ (bare)     │
            └─────┬──────┘   JSON: {count, single, commits:[…], message?}
                  │ single? ──yes──▶ commit T_start (planner's message) → done
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
     frozen leftover empty? ──yes──▶ done
              │ no
              ▼
            ┌────────────┐  commits made + leftover diff   target SHA or null
            │  arbiter   │◀───────────────────────────▶  (stagecoach does all git)
            │ (bare)     │
            └────────────┘
```

### Key design points

**Overlapped staging and generation.** `stager[i+1]` runs in parallel with `message[i]` — the stager prepares the next concept's index while the message agent generates the current commit message. This 1-deep overlap keeps latency low.

**Stage-while-editing (FR-E2).** With `--edit`, the snapshot is frozen *before* the editor opens. You can `git add` in another pane during the edit session — the in-flight commit is unaffected. This is the same stage-while-generating property, extended through the editor. This is the one thing `git commit -e`-style flows cannot offer on top of generation.

**Frozen tree snapshots.** After each stager returns, `write-tree` freezes the accumulated index into an immutable tree object (`tree[i]`). This is the SAME snapshot mechanism as the single-commit path, composed N times.

**Tree-to-tree diffs.** `message[i]` reasons over `diff(tree[i-1], tree[i])` — never `index-vs-HEAD`. This makes each concept diff immune to concurrent staging and to earlier commits landing.

**Serialized publication.** Even though generation overlaps, `commit-tree` + `update-ref` are serialized per concept (CAS). If `HEAD` moved externally, the CAS fails and prior commits stand.

**Start-of-run freeze (T_start).** The instant decomposition activates, the entire working-tree change set (every modified/added/deleted/untracked path and its byte content) is captured as an immutable tree object T_start. The planner partitions T_start's diff (never a fresh re-read of the live tree); every stager, the arbiter (its gate, its diff, and its leftover staging), and the one-file/single shortcuts draw strictly from T_start. A file created or modified after T_start is captured is invisible to the run.

**Freeze enforcement (defense-in-depth).** The freeze boundary is guarded at two layers, neither trusting its caller nor the external stager. At entry, decompose re-asserts its empty-index precondition (FR-M1e) — a stale or buggy trigger that routes here with a non-empty index fails loudly, naming the offending staged paths, instead of silently folding them into T_start (see [Trigger](#trigger)). Then, because the stager is an external agent running `git` against the live tree, after each staging step stagecoach verifies the resulting tree is a content-subset of T_start (only T_start paths, T_start content). Any deviation — a concurrent change swept in, or a stager that ran a bare `git add -A` — is a hard abort (non-rescue; already-landed commits stand per FR-M12), with an error that names the concept by title and explains the cause in plain language (concurrent working-tree changes were picked up by the stager; the abort is intentional freeze-boundary protection), so the user knows the run was protected and can re-run from a clean tree.

**One-file short-circuit.** In auto-decompose, if exactly one path changed, the planner is bypassed entirely: stage that file's T_start content, generate one message, create one commit (FR-M2b). Deterministic, not model judgment. `--commits N` (N≥2) overrides this shortcut.

**Mode-conditional planner rules.** The planner's `Rules:` block is mode-conditional. In auto-decompose (the default) it leans toward splitting unrelated changes — *lean toward SEVERAL* — tempered by a soft target of `max_commits / 2` (default 6) so an ordinary mixed tree lands at or below it rather than fanning into micro-commits; only the hard cap (`max_commits`, default 12) ever errors. Forced-count (`--commits N`) treats the count as fixed and omits the soft target. Every concept carries a `files` list naming each path it touches — a single file split across two concepts is named in both, with the description saying which part belongs where — so each stager knows where to look. After the planner returns, a deterministic coverage check logs (but never errors on) any changed path no concept claimed; the arbiter reconciles those leftovers.

**Arbiter leftover reconciliation.** After all N concepts are committed, stagecoach computes the **frozen leftover** = `diff-names(tipTree, T_start)` — the `T_start` content no stager claimed (`tipTree` is the last committed tree) — and runs the arbiter **iff it is non-empty**. The live working tree is never consulted for the gate, so a file written after `T_start` was captured cannot trigger the arbiter or enter any arbiter commit. Given `TreeDiff(tipTree, T_start)`, the arbiter decides whether the leftovers belong to an existing commit (a plumbing amend that rebuilds the chain from the frozen per-concept `tree[j]` and `T_start`) or warrant a new (N+1)th commit (committing `T_start` directly); stagecoach performs all git from frozen trees, then syncs the index to `T_start`, and the arbiter only decides.

**Output quality is bounded by stager-model discipline.** The freeze and arbiter logic guarantee a *correct* result: the final tree always equals `T_start`, the index is clean, and the repo is never corrupted. They do **not** guarantee *coherent intermediate history*. The stager is an LLM running `git` against the live accumulating index, and a weaker stager model may un-stage or re-add files that an earlier concept legitimately committed (e.g. concept 2 deleting `auth.go` that concept 1 added, forcing concept 3 to re-add it). Such thrashing is content-legal under the freeze (un-staging a `T_start` file is a subset of `T_start`), so it is not caught — only the *final* tree is reconciled. For clean, reviewable multi-commit history, prefer a strong stager model, or use `--single` / pre-stage each concept manually. The disjoint-files common case (the planner emits exact per-concept `files` lists) is the most robust against this.

### Safety

The same snapshot-based safety invariants from the single-commit path apply to every decompose iteration:

- **Atomic and safe** — `update-ref CAS` is the only ref mutation per commit; stagecoach owns all `commit-tree`, `update-ref`, and `push` operations. The stager is the ONE role that touches the index. Its scoping differs by provider: claude is structurally constrained to a staging-only git allowlist (`git add`/`apply`/`status`/`diff`); pi is constrained instructionally (its task prompt) plus a HEAD-movement guard that aborts the run if the stager moves a ref. See [providers.md](providers.md#tooled-mode-and-the-stager-role).
- **Frozen content** — `tree[i]` captures exactly what was staged at `write-tree` time. Nothing added afterward can affect it.
- **No index resets** — the index accumulates across concepts. After the final commit, HEAD.tree == tree[N-1] == full accumulated index, so the index is clean relative to HEAD.
- **Start-of-run freeze** — T_start captures the full working-tree change set at decompose activation; concurrent edits never enter any commit. The freeze boundary is held at three layers: the empty-index precondition is re-asserted at entry (FR-M1e); the stager is verified as a content-subset of T_start after each staging step (FR-M1c); and the arbiter — the third freeze surface — derives its gate, its diff, and every tree it commits strictly from T_start and tipTree, never a live re-read (FR-M1d).

See [configuration.md](configuration.md) for per-role provider/model/reasoning/**timeout** configuration and [cli.md](cli.md) for the decompose and per-role flags. Each role resolves its own generation timeout (FR-R7): the **planner defaults to 480s** (the heavy role), while stager/message/arbiter inherit the global `120s`; override with `--<role>-timeout`.

### Diff capture pipeline

Every diff payload Stagecoach builds — the staged diff, the multi-commit working-tree snapshot, and the per-concept tree-to-tree diff — goes through the same capture pipeline before it reaches the agent. Five transforms run, in order, in every path, alongside the binary/exclusion filtering described below:

1. **Compact change skeleton (FR3g).** A `git diff --numstat` summary is prepended to every payload, under a `Change summary (numstat: …)` header — one `added → deleted → path` line per changed file (binary files show as `-  -  <path>`). This is the completeness floor: the agent always sees the full shape of the change — every file, its add/delete magnitude, and its kind — even when individual bodies are truncated later. A file whose body is fully truncated still appears in the skeleton, so truncation never silently drops a file from view.

2. **Deterministic rename detection (FR3e).** Every `git diff` passes `-M`, so a rename (or a near-rename above the similarity threshold) is emitted compactly — a `rename from` / `rename to` pair plus any residual edit — instead of as a delete + add that duplicates the full file content. This is deterministic regardless of your `diff.renames` config or git version. (Copy detection `-C` is intentionally not enabled — it is expensive and adds little for message generation.)

3. **Reduced diff context (FR3f).** Diffs are captured with `-U1` by default — one anchor line per hunk — instead of git's `-U3` default, since unchanged surrounding lines are noise for message generation. Tune it with `diff_context`: `0` = changed lines only (maximal savings), `1` = one anchor line (the default), `3` = git's default.

4. **Index-line stripping (FR3h).** The `index <oid>..<oid> <mode>` line is stripped from each file diff — the blob OIDs are useless to the model and cost roughly 30 bytes per file. The `diff --git`, `---`, `+++`, and `@@` lines are retained (they carry file identity and hunk location).

5. **Size budget (FR3d / FR3i).** Two mutually-exclusive modes govern how large the payload is:
   - **Legacy caps (the default).** With `token_limit` unset (`0`), the markdown section is capped at `max_md_lines` per file (default 100) and the non-markdown aggregate at `max_diff_bytes` (default 300000); over-cap sections are marked `... [diff truncated at N bytes]` / `... [diff truncated at N lines]`.
   - **Holistic token budget.** Set `token_limit` (for example `120000`) to cap the *whole* payload — system prompt + style examples + the concatenated diff — to a token budget. Stagecoach reserves room for the prompt and examples, then allocates the remainder to the diff bodies with a **dynamic water-fill**: it sizes every file's body up front, and if they exceed the budget it finds a single water level `L` such that every file *smaller* than `L` is included whole and untouched, and every file *larger* than `L` is truncated to `L` (with a `... [truncated]` marker that ends the file's section on its own line, so the next file's `diff --git` begins fresh). Small files are never penalized for their size; large, substantive files receive the bulk of the budget; no single file can monopolize it; and nothing is wasted. The common case — a commit that fits — is left untouched. A non-zero `token_limit` **supersedes** both legacy caps for that run (they are mutually exclusive).

See [configuration.md](configuration.md#built-in-defaults) for the `token_limit`, `diff_context`, `max_diff_bytes`, and `max_md_lines` knobs.

### Binary and non-text file filtering

Binary files, lock files, snapshots, sourcemaps, and vendor directories are **excluded from every diff payload** — staged diff, working-tree snapshot, and concept diff. They are replaced with a `<status>\t[binary] <path>` placeholder so the agent sees *that* the file changed without the useless binary hunk. This applies identically in the single-commit and multi-commit paths.

### Payload exclusions (.stagecoachignore)

Exclusion patterns from `.stagecoachignore`, the `[generation] exclude` config key, or the `--exclude`/`-x` CLI flag hide a file's **diff body** from every payload while still committing the file exactly as it stands. Excluded files emit a `<status>\t[excluded] <path>` placeholder (same shape as the `[binary]` placeholder, distinguishable by tag) so the agent sees *that* the file changed without its contents.

**Payload-only guarantee (FR-X5):** Exclusion is payload-only — it never alters staging or commit content. The excluded file is committed exactly as staged, and `git diff-tree` of the resulting commit includes it. Only what the agent *sees* is affected.

The built-in noise denylist (lock files, snapshots, sourcemaps, vendor directories) always applies alongside any user exclusions — the two sets are unioned, never replaced. See [configuration.md](configuration.md) for `.stagecoachignore` syntax.

## Safety and the rescue protocol

### Per-repo run lock (FR52)

Stagecoach uses a **two-stage defense** against concurrent runs on the same repo:

1. **Per-repo run lock** (advisory `flock(LOCK_EX|LOCK_NB)`) — prevents the common local double-run (two terminals in the same repo). The lock is held on a file descriptor and **auto-releases on process death** (SIGKILL, crash, power loss) — the LOCK never goes stale. Orphaned lock FILES (left by exits that bypass the deferred cleanup) are reaped by pid-liveness on the next Acquire, and the signal path releases the file before exiting.
2. **§13.5 CAS** (`git update-ref HEAD` compare-and-swap) — the second, never-clobber-HEAD guarantee. Even if the lock somehow fails (shared/network FS, cross-host), the CAS ensures only one commit lands per run.

**Per-host limit.** The lock is a per-process advisory flock — it works on a single host. Cross-host contention (shared NFS, etc.) is the CAS's job.

**Never-in-repo location.** The lock file lives in a per-user runtime/cache directory (resolved via `XDG_RUNTIME_DIR` → `XDG_CACHE_HOME` → `~/.cache/stagecoach/locks`), keyed by a sha256 hash of the repo's canonical absolute path. It is **never inside the repo** — an in-repo lock would pollute `git status`, be committable, be ambiguous across worktrees, and be lost on clone.

**No-op fast path.** On the single-commit path (changes staged), the holder publishes its frozen index-tree SHA via `SetSnapshot()`, and a contender whose staged snapshot is byte-identical to it exits 0 immediately. On the decompose path (nothing staged, dirty working tree), an accidental double-run exits **5 (Busy)** instead — the holder publishes a working-tree snapshot (`T_start`) that a lock-free contender cannot reproduce from the index, so it conservatively refuses.

**Auto-release + file reaping.** The lock uses POSIX `flock` — it releases when the file descriptor or process closes, so the LOCK is never stale. The lock FILE, however, is orphaned by exits that bypass the deferred cleanup (SIGKILL, crash, signal-rescue `os.Exit`); on the next Acquire, stagecoach reaps every `*.lock` whose recorded pid is dead (`kill(pid,0)`→`ESRCH`), and the signal path releases the file before exiting. On Windows, `flock` is a no-op stub, reaping is a no-op too, and the §13.5 CAS is the guarantee there.

**Orphaned-but-alive — the launcher-closed case (FR-K1–K7).** The two states above (dead holder, file reaped; live holder, never touched) miss a third that arises from stagecoach's primary launch path (§9.21): a parent process — the lazygit TUI, an IDE, a detaching terminal — that **closes without killing its child**. The child is reparented to init (or a subreaper) and **keeps running**: it still holds the `flock`, its pid is still alive (so stale-file reaping never fires), and it is generating a commit nobody will see. Because the signal handler catches only delivered signals and the orphaning parent delivers none of them, the run never reaches the exit path — the lock file outlives the launcher until the orphan finishes (or forever, if its provider subprocess is wedged by the vanished terminal). This is the "the lock stays forever" report.

The remedy is **self-termination, never contender-side force-breaking** (FR-K1): on startup stagecoach records its parent pid and arms a parent-death watchdog that, on parent death, routes the process through the same rescue + lock-release exit path the signal handler uses — abandoning the in-flight commit is always safe (HEAD moves only at the final `update-ref`; the snapshot is a gc'able orphan whose SHA the rescue recipe prints). Detection is by **parent-pid change** (reparenting), not the brittle `getppid()==1` test (FR-K2) — subreaper-safe; Linux uses `prctl(PR_SET_PDEATHSIG)` as a best-effort fast path plus a ~1s `getppid()` poll, Darwin/other Unix poll only. The watchdog is **Unix-only** (FR-K7); on Windows `flock` is already a no-op and the §13.5 CAS is the guarantee.

`SIGHUP` joins the caught signals `{SIGINT, SIGTERM, SIGHUP}` (Unix; FR-K3): when the controlling terminal closes and the kernel hangs up the process group, stagecoach routes it through the rescue path (clean exit + lock-file removal) instead of Go's default disposition (terminate, skipping the deferred release and leaving the file for the next run's reaper). `SIGHUP` covers the terminal-hangup case where the signal *is* delivered; the parent-death watchdog covers the detach/orphan case where it is *not*. (Windows omits `SIGHUP`.)

`stagecoach lock status` (FR-K4) prints the holder's path/pid/hostname/repo/timestamp/snapshot, whether the holder process is alive, and — on Unix — whether it appears orphaned (reparented). It is **read-only**: it acquires no `flock` and never breaks/removes a lock; the **user** decides whether to `kill <pid>` or `rm <path>`. It never auto-breaks.

The opt-out `no_parent_watchdog` (FR-K6; default **off** — the watchdog runs by default) disables the parent-death watchdog for intentional-detach workflows (`nohup`/`setsid`/`systemd-run`, a service manager) that would otherwise trip it the moment the short-lived launcher exits. `SIGHUP` handling (FR-K3) and `lock status` (FR-K4) are independent of this flag and always on. FR52's "never force-break" guarantee is preserved unchanged: the guarantee is that *another* process never breaks a live lock, and the watchdog is the *same* process abandoning its own unwanted work.

See [CLI reference — lock status](cli.md#lock-status) and [Configuration — environment variables](configuration.md#environment-variables) (the opt-out keys).

### Safety invariant

No provider mutates the repository (PRD §18.1). Every built-in manifest constrains the agent to a read-only mode — either via explicit tool-disable flags (pi, claude) or read-only constraint flags (codex, cursor). The agent receives the diff via stdin/argv and writes the commit message to stdout — it never runs `git add`, `git commit`, or any write command.

### Failure modes and exit codes

| Failure | Exit code | Recovery |
|---------|-----------|----------|
| Agent missing on `$PATH` | 1 (Error) | Check the `[provider.<name>] command` path; install the agent |
| Unresolved merge conflicts in the index | 1 (Error) | Resolve the conflicts, then re-run `stagecoach` (caught before the snapshot) |
| Generation failed (parse/retry exhaustion) | 3 (Rescue) | Rescue message with tree SHA |
| Generation timed out | 124 (Timeout) | Rescue message with tree SHA |
| CAS failure (HEAD moved meanwhile) | 1 (Error) | HEAD-moved message |
| Nothing to commit (clean tree) | 2 (NothingToCommit) | Stage files and retry |
| Another stagecoach run holds the per-repo lock | 5 (Busy) | Wait for the in-progress run to finish, then re-run (see [Per-repo run lock](#per-repo-run-lock-fr52)) |
| General error | 1 (Error) | Inspect error message |

The rescue (3) and timeout (124) rows are the real-commit path; under `--dry-run`, a generation failure reports exit 1 instead — see [Rescue protocol](#rescue-protocol).

See [cli.md](cli.md#exit-codes) for the full exit-code table.

### Rescue protocol

When generation fails after the snapshot is taken on a real commit (exit 3 or 124), Stagecoach prints a recovery block to stderr with the frozen tree SHA and the exact `git commit-tree` command to commit manually:

```text
❌ Commit generation failed.
------------------------------------------------------------
Your staged files were safely snapshotted before generation.
Tree ID: <TREE_SHA>

To commit the originally staged files manually:
  git commit-tree -p <PARENT_SHA> -m "Your message" <TREE_SHA> | xargs git update-ref HEAD

(omit "-p <PARENT_SHA>" if this is the repository's first commit)
------------------------------------------------------------
```

If a candidate commit message was produced but rejected (duplicate subject or parse failure), it is appended to the rescue block so the user can paste it into the manual command.

Under `--dry-run`, the full pipeline still runs and the snapshot is still taken, but a generation failure (timeout or parse/duplicate-check exhaustion) exits **1** with a short stderr message and omits this recovery recipe — no commit was ever intended. The recipe and exit codes 3/124 apply to a real `stagecoach` commit.

## Prompt engineering

### System prompt (mature repos)

For repos with more than one commit, Stagecoach builds a system prompt from the last 20 commit messages:

- **Style learning** — the agent sees recent messages as examples of the project's conventions.
- **Anti-reuse** — a prohibition against copying the wording of any recent commit. Combined with a separate 50-subject dedupe check, this ensures every generated subject is unique.
- **Subject length** — the target is ~50 characters (configurable via `subject_target_chars`).
- **Multi-line rule** — if recent commits use multi-line messages, the agent is instructed to follow the same convention.

### System prompt (new repos)

For repos with zero or one commit (including unborn repos), Stagecoach falls back to a **conventional-commit** system prompt (PRD §17.2): "Use Conventional Commits format (type: description)."

### Format modes and locale

`--format` (default `auto`) controls how the system prompt shapes the commit message, and applies everywhere a message is produced: the message role, the planner's single-commit shortcut, and the arbiter's leftover-commit message.

- **`auto`** — the default described above: learn style from recent commit history.
- **`conventional`** — replaces the learned-style examples with an explicit `type(scope): description` contract (`feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`).
- **`gitmoji`** — replaces the examples with an instruction to begin the subject with one [gitmoji](https://gitmoji.dev) emoji, followed by the compiled-in emoji reference table (no network fetch).
- **`plain`** — replaces the examples with nothing: no learned style, no format contract, just the essence of the change.

For any mode other than `auto`, the recent-commit history examples are omitted entirely — useful for repos with an idiosyncratic or empty history. The multi-line rule and subject-length target still apply in every mode.

`--locale` (e.g. `--locale French`, `--locale ja`) appends one line — "Write the commit message in `<locale>`." — to the system prompt in every format mode. The value is passed through as-is with no translation or validation.

### User payload

The user payload combines the staged diff with the rejection list (previously rejected subjects). On a parse-failure retry, the retry instruction ("Output ONLY the commit message. No preamble, no markdown, no quotes.") is prepended as a corrective preamble.

### Why raw output, not JSON

Stagecoach requests raw text output from agents (`output = "raw"`) rather than structured JSON (PRD §17.4). Reasons:

- Agents that produce raw text are easier to invoke — no need to negotiate a JSON schema.
- A raw contract is more robust across different agent versions and providers.
- The parser handles code-fence stripping and newline normalization, which covers the common raw-output quirks.
- JSON mode is available as a fallback for agents that only produce structured output.

## Multi-turn generation fallback

For diffs too large for a single reliable request, stagecoach has an optional **multi-turn** generation
path (PRD §9.24). It exists because a provider's *per-request* reliability ceiling can lie well below its
advertised context window: a huge one-shot request may return empty or unparseable output even though the
model can handle the same content delivered in smaller pieces. Multi-turn runs on
every generation path — the snapshot commit flow, `--dry-run`, and hook mode (where
it composes with the never-block contract; see
[Hook mode](#hook-mode-vs-the-snapshot-based-flow) below).

**When it triggers.** Multi-turn runs ONLY when all four hold: (1) the normal one-shot path exhausted its
retries on empty/unparseable output; (2) the captured payload exceeds one chunk (`multi_turn_chunk_tokens`,
default 32000); (3) `multi_turn_fallback` is enabled (default `true`); and (4) the resolved provider
declares `session_mode = "append"` (the **pi** provider does; others ship `""` until verified). If any
condition fails, the run proceeds to the normal rescue protocol unchanged — multi-turn is strictly an
extra attempt, never a worse outcome.

**Lossless, not summarized.** Multi-turn is deliberately *not* the lossy "chunk-summarize-combine" pattern.
The full captured diff is re-delivered across N+1 session turns in request-sized pieces — the model sees
the entire diff in its session history — then writes one message at the end:

- **Turn 1:** the normal system prompt + a priming preamble + the first chunk.
- **Turns 2..N:** each remaining chunk, prefixed `PART i/N:`. Boundaries anchor to newlines so no diff line
  is fractured.
- **Turn N+1:** "Now write the commit message for the diff above." This turn's output runs through the
  normal parse + duplicate-rejection pipeline, then commits like any other message.

Each turn is a separate provider invocation bounded by the **message role's** resolved timeout (FR-R7/FR-T5); total wall-clock ≈ `message-timeout × (N+1)`,
surfaced on the progress line at fallback time. That progress line also reports the per-chunk token budget each chunk targets; with `--verbose`, each turn additionally prints its payload size and raw agent output (FR-T11).

**Failure handling.** If any turn errors, times out, or the final output fails to parse/dedupe, the
multi-turn attempt aborts and control passes to the standard rescue protocol — the snapshot is safe and
the run is no worse off than a one-shot failure.

**`token_limit` does not apply (FR-T12).** `token_limit` governs only the one-shot path (it truncates the
payload to fit one request). Multi-turn deliberately ignores it: the whole point is lossless delivery of a
large payload. So when `token_limit` is set, the multi-turn path re-captures the diff with `token_limit`
disabled and delivers the **untruncated** diff across the N+1 turns — the chunking itself never consults
`token_limit`. (The re-capture is skipped when `token_limit` is unset, since the one-shot payload is
already untruncated in that case.)

## Work-description mode (description-first, read-on-demand)

When you already have a description of the work (e.g. a task system that breaks work into named units and
commits each one), the default diff-first path makes the model reverse-engineer intent you already wrote.
**Work-description mode** inverts that: you supply the description, stagecoach leads the prompt with it
plus the file-change skeleton (the list of changed files), and the model pulls specific file diffs on
demand by writing `READ <path>` on its own line (PRD §9.26). It needs no tool-calling support and runs in
bare mode on every provider, because the only action is reading a staged file's diff.

Activate it with `--work-description "..."` (or `--work-description-file <path>`, which wins when both
are set; env `STAGECOACH_WORK_DESCRIPTION`). Message role only; never the default. Directing guidance
("phrase it this way") still uses `--context` — the two inputs are distinct: `--work-description` is the
*what*, `--context` is the *how*.

The read/answer exchange accumulates in one provider session via `session_mode = "append"` (the same
machinery multi-turn uses, §9.24), so provider support is identical (pi ships `"append"`; others ship
`""` until verified). The model has at most `work_desc_read_rounds` rounds (default 5) of READ requests;
after that, stagecoach refuses further reads and demands the commit message (FR-W6 guarantees
termination). A response with no `READ` line is the commit message, parsed through the normal
parse + duplicate-rejection pipeline (so `--format`, `--locale`, `--template`, `--edit` all apply).

If nothing is staged when `--work-description` is present, stagecoach auto-stages all (even with
`auto_stage_all` disabled), mirroring the default action's empty-tree behavior. The mode composes with
`--dry-run`, hooks, and the snapshot/CAS/rescue core. It does **not** cascade into multi-turn fallback —
a user who wants that degradation runs without `--work-description`.

## Commit hooks on the plumbing path

As of v2.4, the snapshot-based flow runs your repository's standard commit hooks itself — you no longer
need hook mode (§9.20) just to get `pre-commit`, `commit-msg`, or `post-commit` to fire on a `stagecoach`
commit. Hooks run in git's documented order around every commit produced by the plumbing path:
`pre-commit` → `prepare-commit-msg` → `commit-msg` before the commit object is created, and `post-commit`
after it is published.

The snapshot freeze still holds: `pre-commit` runs against a throwaway index primed from the frozen
`write-tree` snapshot, never the live index — so files you stage *while* the hook runs are never swept
into the in-flight commit (the core stage-while-generating guarantee). A `pre-commit` may modify paths
already in the snapshot (a formatter re-staging its output) and stagecoach includes those fixes, exactly
like `git commit`; a `pre-commit` that stages a brand-new path aborts the run (it would sweep in
concurrent work). After the commit lands, stagecoach **reconciles the live index** for exactly those
mutated snapshot paths to the committed tree, so `git status` is clean and the index holds the
formatter's output (not the pre-hook blob) — matching `git commit`. The reconcile is surgical: it
updates only the paths the hook changed, so any files you staged *while* generating stay staged
(the stage-while-generating guarantee holds).

`--no-verify` mirrors `git commit --no-verify`: it skips `pre-commit` and `commit-msg` only
(`prepare-commit-msg` and `post-commit` still run). A hook that exits non-zero or times out aborts the
run as a **rescue** (exit code 3) — no commit is created, HEAD and the index are byte-for-byte
unchanged, and the rescue recipe is printed. A `prepare-commit-msg` or `commit-msg` hook that empties
the message file (a rejection or force-re-edit pattern) aborts with **exit 1** and no commit created —
mirroring `git commit`'s "Aborting commit due to empty commit message." and the `--edit` path's
empty-result abort (exit 1, not a rescue). HEAD and the index are untouched at that point (no
`update-ref` has run). `post-commit` is best-effort: its exit code is logged as a
warning but cannot undo an already-landed commit (git itself disregards it).

See PRD §9.25 (FR-V1–V8) for the full specification, and [Hook mode vs the snapshot-based flow](#hook-mode-vs-the-snapshot-based-flow) below for how the two modes compose.

## Hook mode vs the snapshot-based flow

### Trade-off inversion (FR-H7)

Stagecoach offers two ways to generate commit messages, each with different trade-offs:

**Snapshot-based flow** (the default `stagecoach` command):

- **Atomic**: uses `git write-tree` to freeze the index, then `git commit-tree` + `git update-ref` to publish — the repo is byte-for-byte unchanged on failure (no orphan commits, no partial state).
- **Honors pre-commit hooks**: the repository's pre-commit → prepare-commit-msg → commit-msg → post-commit hooks run around every stagecoach commit, scoped to the frozen snapshot (so the stage-while-generating freeze holds). `--no-verify` skips pre-commit + commit-msg (mirrors `git commit --no-verify`). See [Commit hooks on the plumbing path](#commit-hooks-on-the-plumbing-path).
- **Stage-while-generating**: the snapshot decouples staged content from generation time, so you can keep staging while the message generates.
- **Rescue protocol**: if generation fails after the snapshot, the frozen tree SHA is printed so you can commit manually.

**Hook mode** (`stagecoach hook install` + `git commit`):

- **The bridge for plain `git commit`**: hook mode covers the case where you commit via `git commit` from an IDE or another tool instead of invoking `stagecoach`. Hooks run there too (real `git commit`), but there is no snapshot, no atomicity guarantee, and no stage-while-generating — generation latency happens inside the commit.
- **No snapshot guarantees**: the index is live during generation — if you stage more files while the hook runs, they may affect the commit. Generation latency is inside the commit flow (no overlap).
- **Never-block contract**: any failure leaves the message file untouched and exits 0, so the commit proceeds to an empty editor — the commit is never aborted by a model hiccup (unless `--strict` opts in).
- **No rescue protocol**: there is no frozen tree to recover — the commit simply proceeds without an AI message.

**Multi-turn fallback in hook mode.** The [multi-turn fallback](#multi-turn-generation-fallback) is available in hook mode too: on a large diff with an append-mode provider, the hook tries it as one extra attempt before the never-block exit. On success the generated message is written to the commit-message file; on any failure — a turn error, an empty final parse, or a duplicate subject — the hook still exits 0 with the message file untouched (FR-H5 preserved).

### When to use which

- Use **`stagecoach` directly** (the snapshot flow) for day-to-day commits: it's atomic, stage-while-generating, and — as of v2.4 — honors your repository's hooks (`--no-verify` for a one-off skip).
- Install **hook mode** only if you commit via plain `git commit` from an IDE or lazygit instead of invoking `stagecoach` — it fills the message without blocking, with hooks honored but no snapshot guarantees.
- The two **compose**: [Commit hooks on the plumbing path](#commit-hooks-on-the-plumbing-path) (§9.25) covers `stagecoach` commits; hook mode covers `git commit` commits.
