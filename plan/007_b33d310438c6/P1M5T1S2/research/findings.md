# P1.M5.T1.S2 Research Findings — README.md payload-optimization surface + providers/cli consistency

> Research for the README marketing-surface update (PRD §21.5) covering the FR3d–FR3i diff-payload
> optimization work (M2/M3/M4), PLUS a consistency verification of docs/cli.md and docs/providers.md.
> All findings verified against the live tree on 2026-07-04. This is a MODE-B docs-only task: ONE file
> MODIFIED (README.md), TWO files VERIFIED (no change).

---

## 1. The contract, restated (item_description)

(a) **README.md** — add a concise mention of payload optimization to the feature list / 'how it works'
blurb (e.g. "diffs are trimmed and budgeted: rename-aware, reduced-context, skeleton-headed, and
optionally capped to your model's context window via token_limit"); **keep the hero pitch intact**.
(b) **docs/cli.md** — verify the global-flags / config tables are consistent with the new `[generation]`
keys (`token_limit`, `diff_context`). They should already be (the keys are config-only with NO new CLI
flag — confirm no flag was added and the docs don't claim one).
(c) **docs/providers.md** — verify it needs no change (it doesn't — the transforms are provider-agnostic).
(d) **Do NOT duplicate the per-key reference** — that lives in docs/configuration.md (P1.M1.T1.S4).

## 2. README.md structure + the insertion point (the ONE edit)

`README.md` (~20KB) is the marketing surface. Relevant structure:
- **Hero pitch** (lines 1–6): the top blockquote ("Stagecoach writes your commit messages…") + the
  "v2.1 adds payload exclusions, message shaping, …" line. **MUST STAY INTACT** — do not touch.
- **`## Features` table** (6 rows): Payload exclusions, Message shaping, Git hook mode, Tool
  integrations, `--edit`/`--push`, Discovery. Each row is `| <Capability> | <one-line desc> ([docs](anchor)) |`.
  **This table has NO payload-optimization row — that is THE gap.** Insert one row here.
- `## The snapshot workflow` — the concurrency/snapshot model; NOT about the diff pipeline (don't touch).

**Placement**: insert the new "Payload optimization" row immediately AFTER "Payload exclusions" — they
are thematically adjacent (exclusions HIDE files from the payload; optimization TRIMS the payload), so the
two rows read coherently together. Existing row shape (mirror exactly):

```
| Payload exclusions | `.stagecoachignore` / `--exclude` hide a file's diff from the model — never from the commit ([docs](docs/configuration.md#exclusion-globs-generationexclude)). |
```

**Proposed new row (concise, accurate, hero-pitch-safe, dual cross-link):**

```
| Payload optimization | The diff sent to your agent is trimmed and budgeted — rename-aware (`-M`), reduced-context (`-U1`), led by a compact file skeleton, and optionally capped to your model's context window via `token_limit` ([how it works](docs/how-it-works.md#diff-capture-pipeline) · [knobs](docs/configuration.md#built-in-defaults)). |
```

Rationale: matches the table's one-line-with-docs-link idiom; names the user-facing knob (`token_limit`)
inline without duplicating configuration.md's reference; links to BOTH the pipeline explainer (S1) and the
knob reference. "Keep the hero pitch intact" = do not edit lines 1–6.

## 3. Cross-link targets — BOTH verified to EXIST

- `docs/how-it-works.md#diff-capture-pipeline` — **ALREADY EXISTS** (how-it-works.md:130, `### Diff capture
  pipeline`). S1 (P1.M5.T1.S1) is implemented/present; the anchor is guaranteed valid. This section explains
  WHAT the optimization does (skeleton, `-M`, `-U1`, index-strip, token_limit water-fill).
- `docs/configuration.md#built-in-defaults` — **EXISTS** (configuration.md `## Built-in defaults`). Holds the
  defaults table (`token_limit=0`, `diff_context=1`, `max_diff_bytes=300000`, `max_md_lines=100`) AND the
  `> **Token budget & diff context.**` callout (the authoritative per-knob reference, from P1.M1.T1.S4).

Both links follow README's existing relative-link style (`docs/<file>.md#<anchor>`).

## 4. Verified implementation facts (the README claims must be accurate)

| README claim | Source-of-truth | Status |
|---|---|---|
| `token_limit` is the user-facing knob (config-only) | config.go:81 `TokenLimit int` (toml `token_limit`); NO flag in internal/cmd/ or cmd/ | ✓ confirmed config-only |
| `diff_context` / `-U1` default | config.go:82 `DiffContext *int` default `intPtr(1)` → `-U1`; git.go:695-696 `-M` + `-U<n>` | ✓ confirmed |
| rename-aware `-M` always-on | git.go:695 `args = append(args, "-M")` (buildDiffArgs) | ✓ confirmed |
| compact file skeleton | internal/git/skeleton.go `numstatSkeletonHeader` (FR3g) | ✓ confirmed (S1 §2) |
| `token_limit > 0` caps to model's context window | FR3d holistic budget; water-fill (FR3i) | ✓ confirmed |
| NO CLI flag for token_limit/diff_context | grep internal/cmd/ + cmd/ for flag registration → none | ✓ confirmed |

So the proposed row wording is accurate: every literal (`-M`, `-U1`, `token_limit`, "file skeleton",
"capped to your model's context window") matches the shipped implementation. No `diff_context` mention in
the row is needed (it's a secondary knob; `token_limit` is the headline; configuration.md owns the detail).

## 5. docs/cli.md consistency check — PASS, NO edit (task b)

`grep -niE 'token.?limit|diff.?context' docs/cli.md` → **zero matches**. Correct:
- The new `[generation]` keys (`token_limit`, `diff_context`) are **config-only** — there is no
  `--token-limit` / `--diff-context` flag (confirmed: no flag registration in internal/cmd/ or cmd/).
- cli.md's **Global flags** table (lines ~20–43) lists only flags that EXIST; it correctly OMITS
  token_limit/diff_context. No row claims a flag for them.
- cli.md's **Flag ↔ env ↔ git-config map** (line 381+) likewise has NO token_limit/diff_context entry
  (lines 399–400 show `--max-commits` and `--exclude` — neither is a diff-pipeline knob).
- Conclusion: cli.md is **already consistent**. The verification is the deliverable for task (b); no file
  edit. (Do NOT add a token_limit/diff_context row to cli.md — they have no flag/env/git-config triple to
  document, and the per-key reference lives in configuration.md per the contract.)

## 6. docs/providers.md consistency check — PASS, NO edit (task c)

`grep` for token_limit/diff_context/skeleton/water-fill/numstat/`-M`/`-U1`/index-line/max_diff_bytes/
max_md_lines → **zero matches** (the only `--model`/`model_flag` hits are manifest SCHEMA fields, unrelated).
Correct: the diff-pipeline transforms are **provider-agnostic** — they run in `internal/git/` before any
provider renders/executes, identically for all 8 providers. providers.md covers the manifest schema,
rendering, the 8 providers, tools-disable, tooled mode, per-role models, and output parsing — none of which
intersect diff capture/truncation. **No change needed.** The verification is the deliverable for task (c).

## 7. Scope boundary vs S1 (P1.M5.T1.S1) and P1.M1.T1.S4 (parallel coordination)

- **S1** owns `docs/how-it-works.md` (the `### Diff capture pipeline` explainer — DONE, present at line 130).
  S2 does NOT edit how-it-works.md; it LINKS to it.
- **P1.M1.T1.S4** owns `docs/configuration.md` (the per-knob reference: the defaults table + the
  `> **Token budget & diff context.**` callout). S2 does NOT edit configuration.md; it LINKS to it and does
  NOT duplicate the per-key reference (contract §d).
- **S2** owns `README.md` (the Features row) + the cli.md/providers.md verification. ONLY README.md is edited.
- No code, no tests, no other docs files. Zero overlap with S1 or P1.M1.T1.S4.

## 8. Validation (docs-only — no build/test involved)

```bash
# The new row exists + all the literals are present + hero pitch untouched:
grep -nE 'Payload optimization' README.md                                  # → the new row
grep -nE 'token_limit|`-M`|`-U1`|skeleton' README.md                       # → the literals in that row
sed -n '1,6p' README.md                                                    # → hero pitch unchanged

# Both cross-link targets exist:
grep -n '^### Diff capture pipeline' docs/how-it-works.md                  # → line ~130 (the #diff-capture-pipeline anchor)
grep -n '^## Built-in defaults' docs/configuration.md                      # → the #built-in-defaults anchor

# Consistency verification (tasks b/c) — expected ZERO matches (already consistent):
grep -niE 'token.?limit|diff.?context' docs/cli.md                         # → none
grep -niE 'token.?limit|diff.?context|skeleton|water|numstat' docs/providers.md  # → none

# Scope audit:
git status --porcelain                                                     # → ONLY README.md modified
```

## 9. Risks / edge cases

- **Anchor drift**: if S1's heading text differs from `### Diff capture pipeline` the anchor changes. VERIFIED
  present verbatim at how-it-works.md:130 — no drift. (If a future S1 edit renames it, update the README link.)
- **Casing**: README already uses lowercase `docs/<file>.md` links (e.g. `docs/configuration.md#…`); the new
  row matches. Don't use `CONFIGURATION.md` (404s on case-sensitive FS).
- **Over-statement**: the row must not claim `token_limit` is on by default (it's `0`/unset ⇒ legacy caps).
  The proposed wording says "optionally capped" — accurate. Don't add "by default" or imply auto-enabling.
- **Don't touch the "v2.1 adds" line**: the diff optimization is a changeset-level quality improvement, not a
  version-flagged v2.1 feature set. Adding it to that line would be inaccurate. The Features table is the home.
