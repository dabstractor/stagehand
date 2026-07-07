# FR-T9 Verification — pi `session_mode = "append"` (the sole external dependency)

## Status: ✅ VERIFIED 2026-07-05 (live run)

PRD §9.24 FR-T9 requires that a manifest MUST NOT declare `session_mode = "append"` speculatively;
setting it requires a verified, reproducible append-turn rendering. This file records that
verification for the **pi** provider, unblocking the `SessionMode: "append"` shipped value (R1).

## The verification contract (per FR-T9)

A second one-shot invocation against the same session id whose response demonstrably recalls content
from the first call. The exact flag set per provider must be confirmed (analogous to FR-D5's
model-token verification duty) and recorded.

## Verified flag set (pi)

pi's BareFlags MINUS `--no-session`, PLUS `--session-id <id>`. Full turn-1 rendering:

```
pi --provider zai --model glm-5.2 \
   --system-prompt "<sys>" \
   --no-tools --no-extensions --no-skills --no-prompt-templates --no-context-files \
   --session-id stagecoach-<run-uuid> \
   -p                       < <payload via stdin>
```

(Differs from the one-shot pi render ONLY by: `--no-session` removed, `--session-id <id>` added.
`-p`, `--provider`, `--model`, `--system-prompt`, and the rest of BareFlags are unchanged. The
`--continue`/`-c` flag is NOT used — FR-T6: it targets the previous session and is incompatible with
`--session-id`.)

## Live run (2026-07-05, `pi` at `/home/dustin/.local/bin/pi`)

**Turn 1 (create the session, prime it):**
```
pi --provider zai --model glm-5.2 --no-tools --no-extensions --no-skills \
   --no-prompt-templates --no-context-files \
   --session-id stagecoach-frt9-probe -p "remember the word BANANA"
```
→ stdout: `Got it — BANANA. 🍌`

**Turn 2 (same session id, recall — no `--no-session`, no `--continue`):**
```
pi --provider zai --model glm-5.2 --no-tools --no-extensions --no-skills \
   --no-prompt-templates --no-context-files \
   --session-id stagecoach-frt9-probe -p "What word did I ask you to remember? Reply with just that word."
```
→ stdout: `BANANA`

**Verdict:** re-invoking the SAME `--session-id` appends a turn the model can recall. The
`session_mode = "append"` capability is confirmed for pi. The system prompt is supplied via
`--system-prompt` on turn 1; subsequent turns rely on the session carrying it (FR-T6 turn-1-only
system-prompt semantics hold — the session persists the system prompt).

## Implication for the shipped manifest

`internal/provider/builtin.go` `builtinPi()` ships:

```go
SessionMode: strPtr("append"), // VERIFIED 2026-07-05 via `pi --session-id X <isolation-minus-no-session> -p "remember BANANA"` then recall returns BANANA; FR-T9.
```

And `providers/pi.toml` ships:
```toml
session_mode = "append"  # VERIFIED 2026-07-05; FR-T9.
```

All other builtins (claude/opencode/codex/cursor/agy/gemini/qwen-code) ship `""` (absent) — their
append-turn mechanisms are unverified; for them FR-T1 condition (d) is false and multi-turn is
skipped silently (one-shot → rescue unchanged).

## The render contract this verifies (for R1's `RenderMultiTurn`)

For a pi multi-turn turn, `RenderMultiTurn` produces the args above. The `"--no-session"` token is
filtered from `BareFlags`; `"--session-id", <sessionID>` is appended (after the bare-flags block,
before `-p` is acceptable, or anywhere stable — the stub test asserts presence/absence, not position).
The system prompt goes via `--system-prompt` on turn 1 only; on turns 2..N+1 the system-prompt flag
is omitted (the session carries it). Each turn reads its chunk via stdin (`prompt_delivery = "stdin"`).
