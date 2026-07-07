# Research note — README multi-turn blurb placement & links

## Where the blurb goes
`README.md` has a `## Features` section (line 59) with a capabilities table
(lines 61–72). Each row is `| Capability | Description + docs links |`.
This table **is** the "feature-list / how-it works" surface named in the contract.

Row order (current):
1. Multi-commit decomposition
2. Payload exclusions
3. Payload optimization  ← large-diff / payload concern
4. Message shaping
5. Git hook mode
6. Tool integrations
7. `--edit` / `--push`
8. Discovery

**Decision:** insert the Multi-turn row **immediately after "Payload optimization"**
(row 3). Rationale: multi-turn fallback is also a large-diff mechanism, so the two
diff/payload rows cluster together; it then precedes the shaping/HOW row group
logically. (Both are about *what gets delivered to the model on a big diff* —
payload optimization trims/budgets it; multi-turn is the reliability fallback when
even the one-shot of that payload fails.)

## Exact doc anchors (verified against headings on disk)
- `docs/how-it-works.md` line 262: `## Multi-turn generation fallback`
  → anchor `#multi-turn-generation-fallback` (GitHub slugify: lower, spaces→`-`).
- `docs/configuration.md` line 121: `## Built-in defaults`
  → anchor `#built-in-defaults`. **This is where** `multi_turn_fallback` (default
  `true`) and `multi_turn_chunk_tokens` (default `32000`) are documented (the
  table at lines 137–138 + the note at lines 155–157).

These two anchors are exactly the "pointer to docs" the contract allows. They map
to the feature's only user surfaces: the progress line (how-it-works) and the two
config keys (configuration.md).

## Voice match — the "Payload optimization" row is the template
The row to mirror (its link style is the canonical README voice):

```
| Payload optimization | The diff sent to your agent is trimmed and budgeted …
  ([how it works](docs/how-it-works.md#diff-capture-pipeline)
  · [knobs](docs/configuration.md#built-in-defaults)). |
```

So the multi-turn row should use the same `[how it works](…) · [knobs](…)` link pair.

## Wording
Contract suggested wording (verbatim): "Lossless multi-turn fallback: when a
one-shot generation of a large diff fails, stagecoach re-delivers the full diff
across session turns so a single message still lands (no truncation, no extra
commits)."

Adapted to the table voice (one sentence, capability name in col 1, body in col 2,
em-dash instead of parens for the trailing clause to match other rows, and the
contract's parenthetical folded in):

```
| Multi-turn fallback | Lossless multi-turn fallback: when a one-shot generation of a large diff fails, stagecoach re-delivers the full diff across session turns so the message still lands — no truncation, no extra commits ([how it works](docs/how-it-works.md#multi-turn-generation-fallback) · [knobs](docs/configuration.md#built-in-defaults)). |
```

This stays to ONE sentence, names zero CLI flags, and names zero config keys in
the body (the "knobs" link is the allowed pointer to docs). It reuses the existing
link pair so no new convention is introduced.

## Scope guards (from the contract + §9.24)
- This is a **1-row addition to one table** in README.md. Nothing else.
- Do NOT add CLI flags, `session_mode`, `multi_turn_fallback=false` caveats, or
  the progress-line text into README — those live in docs/ (and how-it-works.md
  already has the full §9.24 treatment at line 262).
- The FUTURE_SPEC.md lossy-chunking rejection (line 99) is **unchanged** — the
  blurb says "lossless" and points to §9.24; it does not touch FUTURE_SPEC.
  (FUTURE_SPEC consistency is the SIBLING task P1.M1.T5.S2, not this one.)
- No `.gitignore`, no PRD.md, no tasks.json, no source changes (Mode B doc-only).

## Validation reality
`Makefile` has only a Go `lint` target (golangci-lint); **no markdownlint make
target**. `.markdownlint.json` exists (MD013/MD033/MD060 off, default true).
Validation is therefore: (a) optional `npx markdownlint-cli2 README.md` if the
runner has it, (b) a grep that the new row + both anchors are present, and
(c) a render check that the table still parses (column count). All low-risk.
