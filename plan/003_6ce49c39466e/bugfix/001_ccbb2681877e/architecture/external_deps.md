# External Dependencies — Verified Provider Reasoning CLI Flags

> Research performed 2026-07-02 by running `pi --help` and `claude --help` against the
> installed binaries on this machine. **These are the VERIFIED flags** — the PRD's
> Suggested Fix text guessed `--thinking-effort` for claude, which is INCORRECT.

## pi (verified via `pi --help`)

```
--thinking <level>             Set thinking level: off, minimal, low, medium, high, xhigh
```

- Flag: `--thinking`
- Accepted values: `off, minimal, low, medium, high, xhigh`
- Stagecoach reasoning levels (`off|low|medium|high`) map cleanly:
  - `"high"` → `["--thinking", "high"]`
  - `"medium"` → `["--thinking", "medium"]`
  - `"low"` → `["--thinking", "low"]`
  - `"off"` → no tokens (natural zero value; not in the map)
- Shorthand: `pi --model sonnet:high` (model pattern supports `:thinking` suffix), but the
  manifest's `ReasoningLevels` mechanism appends standalone `--thinking` tokens, which is the
  correct approach for the Render append-after-model-flag design.

## claude (verified via `claude --help`)

```
--effort <level>               Effort level for the current session (low, medium, high)
```

- Flag: **`--effort`** — NOT `--thinking-effort` (the PRD Suggested Fix was wrong here)
- Accepted values: `low, medium, high`
- Stagecoach reasoning levels map:
  - `"high"` → `["--effort", "high"]`
  - `"medium"` → `["--effort", "medium"]`
  - `"low"` → `["--effort", "low"]`
  - `"off"` → no tokens

## Providers with NO known reasoning control (leave nil — graceful no-op)

These remain `nil` per FR-R6's honest per-provider no-op:
- `gemini`, `agy`, `qwen-code` (gemini-family; no thinking-effort flag verified)
- `opencode`, `codex`, `cursor` (no verified reasoning flag)

If a future investigation finds a flag for any of these, add a ReasoningLevels entry — the
Render guard and MergeManifest already handle it correctly.

## FR-D5 Caveat

The PRD repeatedly defers exact-token verification to FR-D5 ("populate reasoning_levels
tokens once verified"). This research IS that verification for pi and claude. The tokens
above are confirmed from live `--help` output on this machine.
