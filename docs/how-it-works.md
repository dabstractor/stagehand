# How Stagehand works

Architecture overview of the Stagehand pipeline: snapshot-based commit creation, stage-while-generating, the safety and rescue protocol, and prompt engineering. This is the cross-cutting "architecture overview" — it ties together the git plumbing, orchestrator, rescue protocol, and prompt assembly.

## The snapshot-based flow

### Why not `git commit`

Stagehand does not use `git commit`. The standard `git commit` reads the **live** index and mutates `HEAD` — it locks the repo state for the duration. If you stage a file while a commit is in progress, that file may end up in the commit unexpectedly.

### The plumbing alternative

Instead, Stagehand uses three low-level git plumbing commands:

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

The snapshot decouples "what's committed" from "what's staged now." The user can keep working while Stagehand generates:

```text
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

Generation time is no longer dead time. The in-flight commit only ever contains what was staged when it started.

## Safety and the rescue protocol

### Safety invariant

No provider mutates the repository (PRD §18.1). Every built-in manifest constrains the agent to a read-only mode — either via explicit tool-disable flags (pi, claude) or read-only constraint flags (codex, cursor, gemini). The agent receives the diff via stdin/argv and writes the commit message to stdout — it never runs `git add`, `git commit`, or any write command.

### Failure modes and exit codes

| Failure | Exit code | Recovery |
|---------|-----------|----------|
| Generation failed (parse/retry exhaustion) | 3 (Rescue) | Rescue message with tree SHA |
| Generation timed out | 124 (Timeout) | Rescue message with tree SHA |
| CAS failure (HEAD moved meanwhile) | 1 (Error) | HEAD-moved message |
| Nothing to commit (clean tree) | 2 (NothingToCommit) | Stage files and retry |
| General error | 1 (Error) | Inspect error message |

See [cli.md](cli.md#exit-codes) for the full exit-code table.

### Rescue protocol

When generation fails after the snapshot is taken (exit 3 or 124), Stagehand prints a recovery block to stderr with the frozen tree SHA and the exact `git commit-tree` command to commit manually:

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

## Prompt engineering

### System prompt (mature repos)

For repos with more than one commit, Stagehand builds a system prompt from the last 20 commit messages:

- **Style learning** — the agent sees recent messages as examples of the project's conventions.
- **Anti-reuse** — a prohibition against copying the wording of any recent commit. Combined with a separate 50-subject dedupe check, this ensures every generated subject is unique.
- **Subject length** — the target is ~50 characters (configurable via `subject_target_chars`).
- **Multi-line rule** — if recent commits use multi-line messages, the agent is instructed to follow the same convention.

### System prompt (new repos)

For repos with zero or one commit (including unborn repos), Stagehand falls back to a **conventional-commit** system prompt (PRD §17.2): "Use Conventional Commits format (type: description)."

### User payload

The user payload combines the staged diff with the rejection list (previously rejected subjects). On a parse-failure retry, the retry instruction ("Output ONLY the commit message. No preamble, no markdown, no quotes.") is prepended as a corrective preamble.

### Why raw output, not JSON

Stagehand requests raw text output from agents (`output = "raw"`) rather than structured JSON (PRD §17.4). Reasons:

- Agents that produce raw text are easier to invoke — no need to negotiate a JSON schema.
- A raw contract is more robust across different agent versions and providers.
- The parser handles code-fence stripping and newline normalization, which covers the common raw-output quirks.
- JSON mode is available as a fallback for agents that only produce structured output.
