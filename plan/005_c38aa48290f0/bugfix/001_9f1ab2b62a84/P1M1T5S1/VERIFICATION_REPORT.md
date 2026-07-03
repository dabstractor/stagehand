# Verification Report — P1.M1.T5.S1 (Changeset Doc-Sync)

**Date**: 2026-07-03
**Scope**: README.md + docs/*.md — audit against post-fix behavior of P1.M1.T1–T4
**Method**: Per-fix deterministic drift probes (grep/read) independently executed and recorded.

## Headline Result

**ZERO documentation drift.** Every fix brought the *code* into compliance with docs that were already
accurate. No doc file required changes. README.md and docs/*.md are byte-identical to their pre-task
state. No source file under `cmd/` or `internal/` was touched.

```
$ git diff --stat README.md docs/
(empty)
$ git diff --stat cmd/ internal/
(empty)
```

---

## Per-Fix Drift Probes

### Fix T1 — Issue 1 (Major): `integrate install lazygit` foreign-key WARNING

| Item | Detail |
|------|--------|
| **Doc files inspected** | `docs/cli.md` L288–L290, L326–L334; `README.md` L170–L189 |
| **Claim audited** | `docs/cli.md` L328: "If an **unmarked** entry already binds your target key (e.g. `<c-a>`), `install` prints a `WARNING` to stderr noting that a duplicate `customCommands` entry will be created, then proceeds through the normal no-mangle preview/confirm flow (outcome: *Updated*)." |
| **Probe** | `grep -niE "WARNING\|conflicting\|unmarked\|duplicate" docs/cli.md` → hit at L328 (exact match) |
| **Probe (README)** | `grep -niE "mangle\|silent\|foreign\|<c-a>" README.md` → L173 `<c-a>` (benign default-key reference), L188 `<c-a>` (manual YAML block), L251 "silently" (about `--config` discovery, unrelated). Zero claims about no-mangle/foreign-key/WARNING. |
| **Verdict** | **ACCURATE — no drift.** docs/cli.md L328 already documents the T1 WARNING verbatim. README makes no contradicting claim. |

### Fix T2 — Issue 2 (Minor): `hook exec` progress emission deferred past no-op gates

| Item | Detail |
|------|--------|
| **Doc files inspected** | `docs/cli.md` L109–L134 (### hook exec, esp. L113) |
| **Claim audited** | L113: "exits 0 having done nothing when a message source is present (`message`/`template`/`merge`/`squash`/`commit`) or nothing is staged." No doc claims the "Generating…" progress line prints on no-op sources. |
| **Probe** | `grep -niE "Generating\|no-op\|FR-H4\|having done nothing" docs/cli.md` → L113 FR-H4 no-op guarantee (exact), L121 no-op example, L128 no-op source description, L301 loadingText (lazygit, unrelated), L455 reasoning no-op (unrelated) |
| **Probe (cross-doc)** | `grep -niE "Generating with" README.md docs/cli.md docs/how-it-works.md` → README L265 (snapshot diagram, NOT hook exec), how-it-works L38 (same diagram). Zero `hook exec` "Generating with" hits. |
| **Verdict** | **ACCURATE — no drift.** The FR-H4 no-op prose says "having done nothing" — after T2 this is literally true (no noise on no-op paths). No doc claims the pre-T2 noisy behavior. |

### Fix T3 — Issue 3 (Minor): `config init --template` now includes 5 v2.1 `[generation]` keys

| Item | Detail |
|------|--------|
| **Doc files inspected** | `docs/configuration.md` L99–L112 (populated config + inert template note), L125–L134 (defaults table), L210+ (Exclusion globs section) |
| **Claim audited** | L112: "documents every available option" — claim that the inert template covers all config keys. The defaults table must list all 5 v2.1 `[generation]` keys. |
| **Probe** | Defaults table contains: `format` L131, `locale` L132, `template` L133, `push` L134. `exclude` at L105 (populated-config example) + L210 "Exclusion globs (`[generation].exclude`)" (dedicated section). All 5 keys present. |
| **Probe** | `grep -niE "documents every available option\|inert" docs/configuration.md` → L112 (exact match). |
| **Verdict** | **ACCURATE — no drift.** docs/configuration.md already lists all 5 keys. T3 fixed the CODE template (`internal/cmd/config.go exampleConfigTemplate`) to match the doc — the fix arrow is code→doc, not doc→code. No doc edit needed. |

### Fix T4 — Issue 4 (Minor): `ui.IsTerminal` true isatty probe (parallel, PRP P1M1T4S1)

| Item | Detail |
|------|--------|
| **Doc files inspected** | `docs/cli.md` L188 (--interactive flag row), L190 (wizard paragraph); `docs/configuration.md` L52 (config init --interactive) |
| **Claim audited** | docs/cli.md L188: "Non-TTY → exit 1 (use plain `config init`)." L190: "Non-TTY stdin exits 1 pointing at plain `config init`." docs/configuration.md L52: "Non-TTY stdin exits 1 pointing at plain `config init`." |
| **Probe** | `grep -nE "Non-TTY\|isatty\|/dev/null\|char device\|terminal on stdin" docs/cli.md docs/configuration.md` → L188, L190, config L52 (all generic FR-L3 prose). |
| **Probe (cross-doc)** | `grep -rliE "isatty\|/dev/null\|char.device" docs/ README.md` → zero hits. No doc mentions isatty internals or `/dev/null`. |
| **Verdict** | **ACCURATE — no drift.** The FR-L3 / FR-I3c prose describes external behavior (Non-TTY gate, auto-decline). After T4, `/dev/null` correctly trips this gate — the docs described the *intended* behavior and need no change. |

---

## Cross-Cutting Sweep

| Doc file | Sweep result |
|----------|-------------|
| `docs/README.md` (documentation index) | Per-page "v2.1 additions" notes (L33–L36) list features (hook, integrate, models, shaping, edit, push). None added/removed by the four internal-behavior bug fixes. L33 mentions "no-mangle protocol" as a v2.1 addition — accurate (T1 *strengthens* it with a WARNING, doesn't change the protocol). **No drift.** |
| `docs/how-it-works.md` (architecture) | L33 "lazygit" is in a diagram pane label (benign). L248 mentions hook mode for lazygit IDE use (unrelated to T1's foreign-key WARNING). Zero mentions of isatty, /dev/null, TTY gates, config templates, or hook-exec progress lines. **No drift.** |
| `docs/providers.md` (manifest schema) | L77: "`agy` is **experimental** (PRD §12.5.1) due to a non-TTY stdout drop bug (issue #76)" — this is a PROVIDER bug, NOT Issue 4's IsTerminal fix. No conflation. Zero mentions of the other three fixes. **No drift.** |

---

## Scope Guardrails

| Check | Result |
|-------|--------|
| `git diff --stat README.md docs/` | **EMPTY** (zero doc changes) |
| `git diff --stat cmd/ internal/` | **EMPTY** (zero code changes — docs-only task) |
| No `plan/005_*/**/tasks.json` modified | Confirmed (not touched) |
| No `PRD.md` modified | Confirmed (not touched) |
| No `.gitignore` modified | Confirmed (not touched) |

---

## Conclusion

All four fixes (T1–T4) are **internal-behavior bug fixes**, not new features. The documentation was
written to the PRD's intended behavior; the fixes reconcile the code to that intent. Every drift probe
independently confirms the docs already describe the post-fix behavior accurately.

Per work item 3(d): "If NO documentation drift is found, document this finding in the subtask completion
and make zero file changes. Do NOT invent documentation changes." — **zero doc changes made; zero code
changes made; this report is the deliverable.**
