# P2.M2.T1.S2 — Per-item PRD evidence (captured at HEAD `a6dbf1c`)

Read-only verification of `PRD.md` at HEAD. The agy re-verification was committed in
`2f77bd0` ("Re-verify and fix agy manifest against v1.1.0"), which touched PRD.md
(54 lines changed across 4 files). HEAD `a6dbf1c` is a descendant of `bb3cb3b`, itself a
descendant of `2f77bd0` — so the correction persists.

All line numbers below are absolute into `PRD.md` at HEAD, captured by direct
`awk`/`grep`/`read` inspection (2026-07-09).

---

## Baseline

| Check | Command | Observed | PASS? |
|---|---|---|---|
| re-verification commit exists | `git log --oneline -1 2f77bd0` | `2f77bd0 Re-verify and fix agy manifest against v1.1.0` | ✅ |
| commit touched PRD.md | `git show --stat 2f77bd0` | `PRD.md` listed; 4 files changed, 119 ins / 80 del | ✅ |
| HEAD is a descendant | `git rev-parse HEAD` | `a6dbf1c` (after `bb3cb3b`, after `2f77bd0`) | ✅ |

---

## Item (a) — §12.5.1 (h3.58) carries the corrected agy manifest

Heading at **PRD.md:947** (`### 12.5.1 Built-in provider: Antigravity CLI (agy) — the Gemini-CLI successor`).
Manifest TOML block spans **PRD.md:951–972**. Header comment **:952** dates it
(`# Antigravity CLI. --help + end-to-end verified 2026-07-08 (agy v1.1.0).`).

| field | line | value | expected | PASS? |
|---|---|---|---|---|
| `name` | 953 | `"agy"` | `"agy"` | ✅ |
| `prompt_delivery` | 957 | `"stdin"` | `"stdin"` | ✅ |
| `print_flag` | 958 | `""` (NON-NIL empty) | `""` | ✅ |
| `model_flag` | 960 | `"--model"` | `"--model"` (not `-m`) | ✅ |
| `default_model` | 961 | `"Gemini 3.5 Flash (Low)"` | display label verbatim | ✅ |
| `bare_flags` | 967 | `["--mode", "plan"]` | `["--mode","plan"]` (no `--approval-mode`) | ✅ |
| `experimental` | 972 | `true` | `true` | ✅ |

Rendered command at **:978**: `agy --model "Gemini 3.5 Flash (Low)" --mode plan   < <sys+user payload via stdin>`
— no `-p` flag, payload via stdin. Matches §12.5.1's stated render.

**Item (a) = PASS.**

---

## Item (b) — §12.5.1.1 (h4.0) marks items 1–3 RESOLVED, item 4 OPEN

Heading at **PRD.md:983** (`#### 12.5.1.1 Status (agy) — verified 2026-07-08 against agy v1.1.0`).

| # | line | label | state | expected | PASS? |
|---|---|---|---|---|---|
| 1 | 985 | non-TTY stdout drop (#76) | **RESOLVED** | RESOLVED | ✅ |
| 2 | 986 | Model flag (`--model`) | **RESOLVED** | RESOLVED | ✅ |
| 3 | 987 | Prompt delivery + read-only mode | **RESOLVED** | RESOLVED | ✅ |
| 4 | 988 | Tooled (stager) flags | **OPEN** | OPEN | ✅ |
| 5 | 989 | Print-mode timeout | (informational) | — | — |

Trailing summary at **:991**: "Items 1–3 are cleared; agy ships `experimental = true` (§12.7.2)
solely pending item 4." — corroborates the RESOLVED/OPEN split.

**Item (b) = PASS.**

---

## Item (c) — §22.1 risk table (h3.103) marks the #76 stdout drop RESOLVED 2026-07-08

Heading at **PRD.md:2135** (`### 22.1 Risks`). The agy row is at **PRD.md:2146**.

Row text (lead): `| **\`agy\` non-TTY stdout drop (issue #76; §12.5.1)** — \`agy -p\` may emit no
stdout when spawned as a subprocess, breaking all \`agy\` roles. **RESOLVED 2026-07-08
(agy v1.1.0):** no longer reproduces; agy reads piped stdin and returns stdout correctly.
Retained here for history. |`

Mitigation cell cites the re-verification: "re-verified end-to-end on v1.1.0 (stdin delivery,
no `-p`, `--mode plan`); agy stays `experimental` only for the unrelated §12.5.1.1 item 4
(stager flags)."

**Item (c) = PASS.**

---

## Item (d) — §12.5.2 (qwen-code) notes agy diverged from the gemini-cli lineage

Heading at **PRD.md:993** (`### 12.5.2 Built-in provider: qwen-code — the Qwen3-Coder CLI (a
Gemini-CLI fork)`). The divergence note is at **PRD.md:995**:

> "NOTE: do **not** assume it matches `agy` — `agy` (§12.5.1) **diverged** from this lineage
> in v1.1.0 (`--model`, value-taking `-p`, no `--approval-mode`); qwen-code's own flags remain
> `# TO CONFIRM` per FR-D5."

Also corroborated in §12.5.1's own prose at **:949** ("The Antigravity CLI has **diverged**
from the gemini-cli lineage it forked from") and §12.5.1.1 item 3 at **:987**.

**Item (d) = PASS.**

---

## Verdict (one line per item)

- (a) §12.5.1 corrected agy manifest (`print_flag=""`, `model_flag="--model"`,
      `bare_flags=["--mode","plan"]`, `default_model="Gemini 3.5 Flash (Low)"`,
      `experimental=true`, stdin delivery) — **PASS** (PRD.md:947–972).
- (b) §12.5.1.1 items 1–3 RESOLVED, item 4 OPEN — **PASS** (PRD.md:983–991).
- (c) §22.1 risk table `agy` #76 row marked RESOLVED 2026-07-08 — **PASS** (PRD.md:2146).
- (d) §12.5.2 qwen-code notes agy diverged from gemini-cli lineage — **PASS** (PRD.md:995).
