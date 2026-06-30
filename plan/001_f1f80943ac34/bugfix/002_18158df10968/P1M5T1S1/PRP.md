---
name: "Sync README.md — config-path fail-fast, [generation] opt-in override, dry-run exit-1 (Mode B, bugfix-002)"
work_item: P1.M5.T1.S1 (bugfix-002)
kind: documentation (Mode B — changeset-level README sweep)
changeset: bugfix-002 (002_18158df10968) — second-pass QA bugfix
depends_on:
  - P1.M1.T1.S1 (Issue 1: explicit --config/STAGEHAND_CONFIG to a MISSING file → exit 1, not silent fallback)   ✅ Complete
  - P1.M2.T1.S1 (Issue 2: Config.Output → *string tri-state; stop defaulting Output/StripCodeFence in Defaults())  ✅ Complete
  - P1.M2.T1.S2 (Issue 2: buildDeps bridge overrides manifest only when [generation] explicitly set)              ✅ Complete
  - P1.M3.T1.S1 (Issue 3: merge-conflict clean message — NOT a README concern; owned by P1.M5.T1.S2 how-it-works) ✅ Complete
  - P1.M4.T1.S1 (Issue 4: dry-run generation failure → exit 1 + short message, no rescue recipe)                  ✅ Complete
mode_a_docs_already_synced:
  - docs/cli.md        (Issue 1: --config row line 20, prose line 30; Issue 4: --dry-run row line 26, exit-code note line 86)
  - docs/configuration.md (Issue 2: [generation] opt-in override, line ~83; git-config keys)
  - docs/providers.md  (manifest output/strip semantics)
  - internal/cmd/config.go exampleConfigTemplate (slightly loose NOTE — see Gotchas; out of scope to edit)
---

## Goal

**Feature Goal**: Reconcile `README.md`'s three cross-cutting blurbs with the **bugfix-002 shipped
behavior**, removing the **over-claims** that bugfix-001's README sweep introduced and that bugfix-002
now contradicts. Specifically: (a) the `[generation] output`/`strip_code_fence` blurb must stop saying
"overriding per-provider values" as if it always wins — it is now an **opt-in override** and the
per-provider manifest value wins by default (Issue 2); (b) the `--config` note must state that an
explicit path pointing at a **missing** file fails fast with exit 1 rather than silently invoking a
real agent (Issue 1); (c) `--dry-run` should note that a generation failure exits **1** with a short
message, not 3/124 + the recovery recipe (Issue 4).

**Deliverable**: An edited `README.md` (**prose only — no code changes anywhere**) in which three
specific blurbs no longer drift from shipped behavior, verified by (1) `markdownlint-cli2` passing
with 0 errors, and (2) a coherence check against the already-synced `docs/cli.md` /
`docs/configuration.md`. All edits stay minimal and accurate — **no over-claiming, no stale
"broader layer wins" / silent-fallback claims remain**.

**Success Definition**:

- `npx markdownlint-cli2 README.md` → `0 error(s)`.
- The string `overriding per-provider values` (the bugfix-001 over-claim at line 119) is GONE or
  corrected to the opt-in framing.
- The `--config` paragraph states a missing explicit path errors (exit 1) instead of silently
  falling back.
- `--dry-run` documents (concisely) that a generation failure exits 1 with a short message.
- The precedence list is UNCHANGED. Every README behavioral claim agrees with `docs/cli.md` /
  `docs/configuration.md`.

## Why

- `README.md` is the front door. The bugfix-001 sweep added two claims that bugfix-002 has now made
  **wrong**: "overriding per-provider values" (Issue 2 — manifest now wins by default) and an
  incomplete `--config` story (Issue 1 — a missing explicit path now errors). A README that over-claims
  is worse than one that omits, because it actively misleads users (e.g. "my manifest `output=json`
  must work — the README says `[generation]` overrides everything").
- These three fixes have no user-facing contract surface beyond the blurbs already present, so the
  reconciliation is **small and surgical** — do not re-derive the whole config story, mirror the
  already-correct Mode-A docs.
- This is the **final** bugfix-002 documentation task scoped to README. Issue 3 (merge-conflict
  wording) is owned by the sibling task **P1.M5.T1.S2** (`docs/how-it-works.md`) — do NOT touch it.

## What

Three prose edits to `README.md`, each mapped to a shipped fix. The **current text** is quoted
verbatim (post bugfix-001), the **drift** is named, and the **canonical wording** is quoted from the
already-synced Mode-A docs (mirror the substance; keep README phrasing compact).

### Drift map (line numbers verified against the current `README.md`)

| # | README location (line) | Current text (verbatim) | Issue | Drift | Type |
|---|------------------------|-------------------------|-------|-------|------|
| 1 | `## Configure your agent` — the `[generation]` NOTE (~119) | `> The template also documents a [generation] section: output ("raw" / "json") and strip_code_fence tune how Stagehand parses agent output across all providers (overriding per-provider values).` | 2 | "(overriding per-provider values)" is an **over-claim** bugfix-001 added and bugfix-002 reverses. `[generation]` is now an **opt-in override**; when omitted, the per-provider manifest value wins. | **Fix-inaccuracy** (reverses a stale claim) |
| 2 | `## Configure your agent` — the `--config` paragraph (~121) | `Point discovery at a specific file with stagehand --config path/to/config.toml. It is honored by every command — including the default commit action — so a provider declared under [provider.<name>] there is usable with --provider <name> directly.` | 1 | Implies discovery-style tolerance; does NOT state that an explicit path to a MISSING file errors. A typo silently invokes a real agent. | **Fix-inaccuracy** |
| 3 | `## Quick start` (~66) and/or a FAQ entry | `# 4. Preview the real message (full pipeline: snapshot→generate→parse→dedupe→retry), no commit` | 4 | Dry-run is framed as a safe preview but a generation failure now exits **1** with a short message (not 3/124 + recovery recipe). Not mentioned. | **Strengthen** (optional, keep concise) |

> The existing `> [!NOTE]` about a missing **provider command** failing fast with exit 1 (line 97,
> added in bugfix-001) is **still correct** and **must not change** — it is about the provider
> binary, not the config file. Do not confuse it with Issue 1 (config *file* missing).
>
> Issue 3 (merge-conflict wording) has **no README surface** — leave it to P1.M5.T1.S2.

## All Needed Context

### Context Completeness Check

**Pass** — this PRP quotes the exact current README lines, the exact canonical doc wording, the lint
command, and the gotchas. An agent who has never seen this repo can complete it from this file +
`README.md` + `docs/cli.md` + `docs/configuration.md`.

### Documentation & References

```yaml
# MUST READ — the file being edited
- file: README.md
  why: THE edit target. Read it fully; the three drift points are at lines ~66, ~119, ~121.
  pattern: GitHub-flavored Markdown with `> [!NOTE]` admonitions and HTML comments.
  gotcha: |
    - markdownlint config (.markdownlint.json) disables MD013 (line length) and MD033 (inline HTML);
      the existing `<!-- TODO -->` comments and long lines are fine — keep that style.
    - Reuse the existing `> [!NOTE]` admonition style for new notes; do not invent a new callout format.
    - Keep edits minimal (contract: "Keep changes minimal and accurate").

# MUST READ — canonical source of truth for wording (already synced in bugfix-002 Mode A)
- file: docs/cli.md
  why: the authoritative CLI reference, already aligned with shipped behavior.
  sections:
    - line 20: --config = "Path to a config file, overrides discovery. A path pointing at a **missing**
      file fails fast with exit 1 (like a malformed or directory path), rather than falling back to
      discovery."  → mirror for drift #2
    - line 26: --dry-run = "...Run the full snapshot→generate→parse→duplicate-check pipeline (same as a
      real commit, including the write-tree snapshot and retry) and print the message; do not commit. If
      generation fails (timeout or parse/duplicate-check exhaustion), exits **1** with a short stderr
      message instead of exit 3/124 + the full recovery recipe (since no commit was ever intended)"
      → mirror for drift #3
    - line 30: "An explicit `--config` (or `STAGEHAND_CONFIG`) pointing at a missing file errors with
      `config: config file not found: <path>` (exit 1) instead of silently falling back to provider
      auto-detection. Only the discovery default (no `--config` or `STAGEHAND_CONFIG`) tolerates a missing
      global file."  → mirror for drift #2
    - line 86: "With `--dry-run`, generation failures (timeout or parse/duplicate-check exhaustion) report
      exit **1** with a short stderr message (not 3/124 + the recovery recipe)"  → mirror for drift #3
  critical: Do not contradict docs/cli.md. If you can't phrase something shorter for the README, copy the docs/cli.md substance verbatim.

- file: docs/configuration.md
  why: canonical config reference; already aligned with shipped behavior.
  section: line ~83 (the opt-in override wording) — "These `[generation]` values are an **opt-in
    override**: when `[generation]` (and git-config) omit them, the per-provider `[provider.<name>]`
    value is honored, falling back to the §12.1 manifest defaults (`output = "raw"`,
    `strip_code_fence = true`). Set `output = "json"` here only to force JSON parsing across ALL
    providers."  → mirror for drift #1 (this is the EXACT framing that replaces "overriding per-provider values")
  critical: This wording is the antidote to the stale "overriding per-provider values" / "broader layer wins" claim. Use it.

# SUPPORTING — why each fix was made (root cause + seam)
- file: plan/001_f1f80943ac34/bugfix/002_18158df10968/architecture/system_context.md
  why: "Seam A" (Issue 1, config-file path resolution) and "Seam B" (Issue 2, the [generation]↔manifest
        bridge) explain WHY the README over-claims are now wrong. "UX wording (Issues 3 & 4)" explains
        the dry-run exit-1 CLI-layer change.
  section: "## The two seams this bugfix touches"

- file: plan/001_f1f80943ac34/bugfix/002_18158df10968/architecture/issue_analysis.md
  why: verified file:line evidence for all four issues (Issue 1: load.go:48-65; Issue 2: config.go:35-36,68-69 + stagehand.go:197-211; Issue 4: default_action.go:169-188).
```

### Current Codebase tree (documentation surface only)

```bash
README.md                      # ← THE edit target (Mode B, bugfix-002)
docs/
├── cli.md                     # Mode A — already synced (SOURCE OF TRUTH for wording)
├── configuration.md           # Mode A — already synced (opt-in override wording at ~line 83)
├── how-it-works.md            # ← owned by P1.M5.T1.S2 (Issue 3 merge-conflict); DO NOT EDIT here
├── providers.md
└── README.md
internal/cmd/config.go         # config init template — see Gotchas (slightly loose NOTE; OUT OF SCOPE)
.markdownlint.json             # lint config: default=true, MD013/MD033 off (MD060 listed but non-standard → ignored)
.github/workflows/ci.yml       # the CI badge in README points here (file exists ✓)
```

### Desired Codebase tree

```bash
# No files added, moved, or deleted.
README.md                      # ← EDITED (3 prose touches); everything else untouched
```

### Known Gotchas of our codebase & Library quirks

```text
# CRITICAL: markdownlint is NOT wired into the Makefile or CI (ci.yml).
# The ONLY way to validate is the manual command:
#   npx markdownlint-cli2 README.md
# (markdownlint-cli2 v0.22.1 / markdownlint v0.40.0 is cached and available via npx.)
# README currently passes (0 errors) — that baseline MUST be preserved. Do NOT assume CI catches it.

# CRITICAL — the bugfix-001 over-claim to REVERSE:
# README line ~119 currently ends "(overriding per-provider values)". That was added by bugfix-001's
# README sweep under the (then-correct) belief that [generation] always wins. Bugfix-002 Issue 2 made
# [generation] an OPT-IN override: the per-provider manifest value now wins by default. The phrase
# "overriding per-provider values" / "broader layer wins" MUST be removed or corrected. The contract
# explicitly requires: "no stale 'broader layer wins' / silent-fallback claims remain."

# CRITICAL — Issue 1 vs the existing provider-command NOTE:
# There are now TWO different "fails fast with exit 1" notes:
#   - README line 97 (bugfix-001): a missing provider COMMAND (binary not on $PATH) → exit 1. STILL CORRECT. Do not touch.
#   - README line 121 (this task): a missing config FILE (--config/STAGEHAND_CONFIG path) → exit 1. ADD this.
# Do NOT merge or confuse them; they are different failure modes (binary vs file). Keep both, distinct.

# GOTCHA: This is a PROSE-ONLY task. There are no Go unit tests for README content.
# Validation = lint + coherence vs docs/*.md + anchor/badge integrity. Do NOT edit any .go file.

# GOTCHA — the config init template (internal/cmd/config.go ~line 163) still says:
#   "# NOTE: [generation] output/strip_code_fence override any per-provider [provider.<name>] values."
# This is slightly loose (it omits the "only when set" nuance) but is NOT wrong (when SET it does
# override). It is CODE, not a doc, and is OUT OF SCOPE for this README task. Do NOT edit it. Your
# README wording must be the MORE PRECISE opt-in framing regardless — the README leads, the template
# is a known loose sibling. If you want, flag the template as a follow-up; do not fix it here.

# GOTCHA: The precedence ORDER in the README line (~123) is ALREADY CORRECT and matches
# docs/configuration.md. Do NOT reorder it.

# GOTCHA: --dry-run still RUNS the full pipeline (snapshot + dedupe + retry) — bugfix-002 did NOT
# change that. The ONLY change is the FAILURE outcome (exit 1 + short message instead of 3/124 +
# recovery recipe). Do not weaken the existing "full pipeline" wording at line 66; only ADD the
# failure-exit note.
```

## Implementation Blueprint

### Implementation Tasks (ordered — edit top-to-bottom so line numbers stay predictable)

```yaml
Task 1: FIX-INACCURACY — reword the [generation] NOTE to opt-in override  (drift #1, Issue 2)  ★ THE KEY EDIT
  - FILE: README.md, "## Configure your agent", the `> [!NOTE]` at line ~119.
  - CURRENT (verbatim):
        > The template also documents a `[generation]` section: `output` ("raw"|"json") and
        > `strip_code_fence` tune how Stagehand parses agent output across all providers
        > (overriding per-provider values).
  - CHANGE: Remove "(overriding per-provider values)" and replace with the OPT-IN framing. State that
    these are an OPT-IN OVERRIDE: when `[generation]` (and git-config) omit them, the per-provider
    `[provider.<name>]` value is honored (falling back to the raw/true defaults); set them here only
    to force the value across ALL providers. Mirror docs/configuration.md line ~83 substance.
  - EXAMPLE (compact, adjust phrasing to taste — keep it ONE admonition):
        > The template also documents a `[generation]` section: `output` ("raw"|"json") and
        > `strip_code_fence` are an **opt-in override** for how Stagehand parses agent output. When
        > unset, the per-provider `[provider.<name>]` value is used (defaulting to `raw` / `true`); set
        > them under `[generation]` only to force the value across ALL providers.
  - CRITICAL: the word "override" may stay, but ONLY as "opt-in override" / "when set". The phrase
    "overriding per-provider values" (unconditional) must be GONE.

Task 2: FIX-INACCURACY — clarify --config must point at an existing file  (drift #2, Issue 1)
  - FILE: README.md, "## Configure your agent", the --config paragraph at line ~121.
  - CURRENT (verbatim):
        Point discovery at a specific file with `stagehand --config path/to/config.toml`. It is
        honored by every command — including the default commit action — so a provider declared under
        `[provider.<name>]` there is usable with `--provider <name>` directly.
  - CHANGE: Keep the "honored by every command / default action" sentence (still correct). ADD one
    sentence: an explicit `--config` (or `STAGEHAND_CONFIG`) must point at an EXISTING file — a missing
    path fails fast with exit 1 (`config file not found`) rather than silently falling back to
    auto-detection. Mirror docs/cli.md line 20 + line 30 substance.
  - EXAMPLE (append/interleave, keep compact):
        Point discovery at a specific file with `stagehand --config path/to/config.toml`. It is
        honored by every command — including the default commit action — so a provider declared under
        `[provider.<name>]` there is usable with `--provider <name>` directly. The path must exist: an
        explicit `--config` (or `STAGEHAND_CONFIG`) pointing at a missing file fails fast with exit 1
        rather than silently falling back to auto-detection.
  - GOTCHA: distinguish from the provider-command NOTE at line 97 (binary vs file). Keep them separate.

Task 3: STRENGTHEN — note dry-run failure exits 1  (drift #3, Issue 4)
  - FILE: README.md, EITHER the Quick start `--dry-run` comment (line ~66) OR a short addition to the
          "## FAQ" section. Keep it CONCISE — "do not bloat the README".
  - CURRENT Quick start comment (line 66):
        # 4. Preview the real message (full pipeline: snapshot→generate→parse→dedupe→retry), no commit
        stagehand --dry-run
  - CHANGE (preferred — one short note after the Quick start block, or a compact FAQ line): note that
    if generation fails (timeout or parse/duplicate-check exhaustion), `--dry-run` exits **1** with a
    short message — it does NOT print the full recovery recipe or exit 3/124, since no commit was ever
    intended. Mirror docs/cli.md line 26 + line 86 substance.
  - OPTIONS (pick ONE, do not do both — avoid bloat):
    (a) A `> [!NOTE]` directly under the Quick start --dry-run block:
          > [!NOTE]
          > If generation fails, `--dry-run` exits 1 with a short message — not the full recovery
          > recipe or exit 3/124 — since no commit was ever intended.
    (b) A one-line FAQ answer (if a dry-run FAQ entry exists or fits naturally). Currently there is no
        dry-run FAQ; option (a) is the lower-friction choice. Prefer (a).
  - GOTCHA: Do NOT weaken the existing "full pipeline: snapshot→generate→parse→dedupe→retry" wording —
    that is still accurate. Only ADD the failure-exit behavior.

Task 4: NO-OP for Issue 3 (merge conflicts)
  - Issue 3 (clean "resolve merge conflicts first" message) has NO README surface. It is owned by
    P1.M5.T1.S2 (docs/how-it-works.md). Do NOT add anything for it to the README.
```

### Implementation Patterns & Key Details

```markdown
# PATTERN: keep README blurbs SHORT and a strict subset of docs/.
# Every behavioral claim in README should be worded identically (in substance) to docs/cli.md /
# docs/configuration.md. The README is a summary, never a new source of truth.

# PATTERN: reuse the existing `> [!NOTE]` admonition for new notes (lines 95, 113 already use it).
# markdownlint allows them under the current config. Do not introduce a new callout style.

# CRITICAL (reversal): the single most important edit is removing "overriding per-provider values".
# That phrase is the bugfix-001 over-claim that bugfix-002 Issue 2 made false. Verify with:
#   grep -n "overriding per-provider\|broader layer wins" README.md   # MUST return nothing.
```

### Integration Points

```yaml
NONE — this is a single-file prose edit. No config, routes, database, or code integration.
The only "integration" is textual consistency with the already-synced docs/*.md.
```

## Validation Loop

> This is a documentation task. There are no Go tests, no build step, no server. Validation is
> lint + coherence + integrity. **markdownlint is NOT in CI** — run the commands below manually.

### Level 1: Markdown lint (the primary gate)

```bash
# From the repo root. MUST show "Summary: 0 error(s)".
npx markdownlint-cli2 README.md

# (Optional) confirm you didn't need a docs/ change:
npx markdownlint-cli2 "**/*.md"
# If docs/*.md fail, that is NOT your task (P1.M5.T1.S2 owns docs/how-it-works.md) — flag it, do not fix.
```

### Level 2: Coherence + over-claim-removal check vs the already-synced docs

```bash
# THE KEY ASSERTION — the bugfix-001 over-claim is GONE (must return nothing):
grep -n "overriding per-provider\|broader layer wins" README.md

# [generation] is now framed as opt-in (Issue 2) — should match docs/configuration.md substance:
grep -n "opt-in\|opt in\|when unset\|only to force\|per-provider" README.md docs/configuration.md

# --config missing-file errors (Issue 1) — README and docs/cli.md agree:
grep -n "missing file\|fails fast\|exit 1\|config file not found\|silently falling back" README.md docs/cli.md

# dry-run failure exits 1 (Issue 4) — README and docs/cli.md agree:
grep -n "exits 1\|exit 1\|short message\|recovery recipe\|3/124" README.md docs/cli.md
# Expected: README and docs agree in substance; no stale silent-fallback / always-wins claim remains.
```

### Level 3: Anchor & badge integrity

```bash
# CI badge workflow path exists:
test -f .github/workflows/ci.yml && echo "badge target OK"

# In-README anchor targets still resolve to real headings:
grep -n "^## The snapshot workflow$" README.md   # anchor #the-snapshot-workflow
grep -n "^## Adding a new agent$"   README.md   # anchor #adding-a-new-agent
# Any NEW internal links added must also resolve (lowercase, spaces→hyphens, strip punctuation).
```

### Level 4: Build sanity (no code touched — confirm you didn't break the build by accident)

```bash
# If you ONLY edited README.md, this is a no-op confirmation. Run it to be safe.
go build ./... && go vet ./... && go test ./... 2>&1 | tail -5
# Expected: clean build/vet and all tests pass. (You should not have touched any .go file.)
```

## Final Validation Checklist

### Technical / Lint

- [ ] `npx markdownlint-cli2 README.md` → `0 error(s)`
- [ ] No other files modified (`git status --short` shows ONLY `README.md`)
- [ ] `go build ./... && go vet ./... && go test ./...` still pass (sanity — no code touched)

### Feature / Coherence

- [ ] Drift #1: the `[generation]` NOTE no longer says "overriding per-provider values"; it is framed as an **opt-in override** (per-provider value wins when unset)
- [ ] `grep -n "overriding per-provider\|broader layer wins" README.md` returns nothing
- [ ] Drift #2: the `--config` paragraph states a missing explicit path fails fast (exit 1) instead of silently falling back
- [ ] Drift #3: `--dry-run` documents (concisely) that a generation failure exits 1 with a short message
- [ ] The provider-command NOTE at line ~97 (binary missing → exit 1) is UNCHANGED and still distinct from the config-file note
- [ ] The "full pipeline" dry-run wording at line ~66 is preserved (not weakened)
- [ ] Config precedence list UNCHANGED (still matches `docs/configuration.md`)
- [ ] Every README behavioral claim agrees with `docs/cli.md` / `docs/configuration.md`

### Integrity

- [ ] CI badge workflow path (`.github/workflows/ci.yml`) still exists
- [ ] All in-README anchors (`#the-snapshot-workflow`, `#adding-a-new-agent`) resolve
- [ ] Any newly added internal links resolve

### Style / Discipline

- [ ] Minimal edits only — no padding, no whole-section rewrites (contract: "Keep changes minimal and accurate")
- [ ] Reuses existing `> [!NOTE]` admonition style; no new callout format
- [ ] Did NOT edit `docs/*.md` (how-it-works.md owned by P1.M5.T1.S2), `internal/cmd/config.go`, `PRD.md`, or any `.go`/code file
- [ ] Did NOT add anything for Issue 3 (merge conflicts) — it has no README surface

---

## Anti-Patterns to Avoid

- ❌ Don't leave "overriding per-provider values" / "broader layer wins" anywhere — that is the bugfix-001 over-claim bugfix-002 Issue 2 reverses. It MUST go.
- ❌ Don't merge the config-FILE-missing note (Issue 1) with the provider-COMMAND-missing note (line 97). Different failure modes; keep both distinct.
- ❌ Don't weaken the "full pipeline" dry-run wording — bugfix-002 did not change the pipeline, only the failure exit code.
- ❌ Don't invent behavioral claims not present in `docs/cli.md` / `docs/configuration.md` — the README is a subset/summary, never a new source of truth.
- ❌ Don't edit `docs/*.md`, `internal/cmd/config.go` (the config init template NOTE is loose but out of scope), `PRD.md`, `tasks.json`, or any `.go` file.
- ❌ Don't skip the manual `npx markdownlint-cli2 README.md` run assuming CI will catch it — markdownlint is NOT in CI.
- ❌ Don't pad or add a FAQ for Issue 3 — it has no README surface (owned by P1.M5.T1.S2).
