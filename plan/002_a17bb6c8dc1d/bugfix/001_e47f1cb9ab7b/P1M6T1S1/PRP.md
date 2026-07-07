---
name: "Update README.md to reflect all five fixes (changeset-level docs sync)"
work_item: P1.M6.T1.S1
changeset: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b (Stagecoach v2.0 QA bug pass)
issue: all five (1 Critical, 2/3/4 Major, 5 Minor — all implementing subtasks are ✅ Complete)
kind: documentation (Markdown ONLY — no Go code, no Go tests)
mode: B (changeset-level docs sync — this task spans the entire P1 changeset and runs LAST)
depends_on:  # ALL implementing subtasks (this is the final docs task)
  - P1.M1.T1 (Issue 1, Critical: provider/sub-provider conflation — callers now pass "" so Render emits --provider <default_provider>, never the manifest name)  ✅
  - P1.M2.T1 (Issue 2, Major: stager toolset — claude structurally scoped via staging-only allowlist; pi honestly documented as NOT flag-scoped + HEAD-movement guard added)  ✅
  - P1.M3.T1/T2 (Issue 3, Major: post-arbiter output — LogRange + rereadFinalCommits; printed SHAs are now accurate/resolvable)  ✅
  - P1.M4.T1 (Issue 4, Major: config subcommands now honor --config / STAGECOACH_CONFIG via ResolveConfigPath)  ✅
  - P1.M5.T1 (Issue 5, Minor: bootstrap blanks pi per-role models — pi picks its own backend default + sub-provider NOTE annotation)  ✅
  - P1.M6.T1.S2 (sibling task: docs/ directory consistency — OUT OF SCOPE here; do NOT touch docs/*.md)
---

## Goal

**Feature Goal**: Synchronize the top-level `README.md` with the **shipped behavior** after all five
P1 bug fixes so that **no stale claim remains** and every user-facing behavior the README describes is
accurate. README.md is the project's front door (GitHub landing page + quick start); it mentions the
provider setup (`git config stagecoach.provider pi`), the bootstrap experience (`config init`), the
config-file override (`--config` / `STAGECOACH_CONFIG`), and the multi-commit four-role pipeline. All
five fixes touch behavior the README either describes or implies.

**Deliverable**: An edited `README.md` (the only file this task touches) in which:
1. The provider/sub-provider distinction is accurate — for multi-provider agents like **pi**, the
   rendered `--provider <backend>` comes from `[provider.<name>] default_provider`, **not** the
   manifest name (Issue 1).
2. The stager safety language is honest — README must **not** repeat a §19-style "structurally
   constrained, cannot commit/amend/push" claim without qualifying it for pi (Issue 2).
3. The multi-commit output fidelity is not misrepresented (Issue 3 — README makes no specific SHA
   claim today, so this is a verification pass).
4. `config init` / `config path` / `config upgrade` are documented as honoring the
   `--config` / `STAGECOACH_CONFIG` override (Issue 4).
5. The bootstrap description ("populated, working config … writes per-role model defaults") is
   adjusted to reflect that for **pi** the per-role models are blanked (pi picks its own backend
   default) until the user sets a `default_provider` (Issue 5).

**Success Definition**:
- Every one of the five behaviors above is accurately reflected (or, where README made no claim, is
  verified to contain no stale claim).
- `go build ./...` and `go test ./...` remain green (docs edits must not touch Go; this is a
  regression guard, not a functional change).
- `README.md` is the ONLY modified file under `git status` for this subtask.
- No edits to `docs/*.md` (that is sibling task **P1.M6.T1.S2**), no edits to `PRD.md`,
  no edits to `providers/*.toml`, no edits to any `.go` file.

## Why

- **Docs must track the binary.** The `docs/README.md` note states "If anything here disagrees with
  `stagecoach --help`, the binary is authoritative." README.md is read first by every new user; a stale
  claim about the default provider (pi), the stager's safety model, or config override behavior
  misleads the exact people these fixes were for.
- **Issue 1 (Critical) is user-visible in setup.** The recommended setup (`git config
  stagecoach.provider pi`) now actually works (pi no longer receives a bogus `--provider pi`). README
  should not imply the manifest name is the sub-provider.
- **Issue 2 is a safety/trust claim.** README is the marketing surface; if it (or a reader's memory
  of it) implies the stager *cannot* commit, that must be qualified honestly for pi (which is only
  instructionally constrained + HEAD-guarded).
- **Issue 4 is a footgun README already half-documents.** README line ~189 says `--config` is
  "honored by every command — including the default commit action"; after Issue 4 that is now
  *actually* true for the config subcommands too, and README should say so explicitly.
- **Issue 5 changes the bootstrap output.** README's "writes per-role model defaults" is now only
  fully accurate for non-pi agents; for pi the models are blanked.
- **Scope discipline**: this is **README.md ONLY**. The `docs/` directory is a separate subtask
  (**P1.M6.T1.S2**); do not duplicate or preempt that work.

## What

This is a **text-only** task: a handful of precise, localized edits to `README.md`. No code, no
tests, no schema. Five behavioral areas map to concrete README locations:

| Fix | README location(s) today | Required action |
|-----|--------------------------|-----------------|
| Issue 1 (sub-provider) | `git config stagecoach.provider pi` block (line ~167); `--provider <name>` note (line ~189) | Add a concise clarifying note: for pi the backend comes from `[provider.pi] default_provider`, not the manifest name. |
| Issue 2 (stager safety) | four-role pipeline mention (lines ~32, ~114); FAQ "Will it corrupt my repo?" (line ~268) | **Verify** README makes no §19 "structurally constrained" claim; optionally add ONE honest sentence about the stager's defense-in-depth. |
| Issue 3 (post-arbiter SHAs) | multi-commit demo output (lines ~116–118) | **Verify** no stale SHA/output claim; no edit required unless a stale claim is found. |
| Issue 4 (config override) | "Point discovery at a specific file" paragraph (line ~189) | **Edit**: state that `config init` / `config path` / `config upgrade` honor `--config` / `STAGECOACH_CONFIG`. |
| Issue 5 (bootstrap models) | "bootstrap a populated, working config" (line ~172) | **Edit**: note pi per-role models are blanked (pi picks its own backend default); set `default_provider` to pin a backend. |

### Success Criteria

- [ ] README's provider-setup text does not imply the manifest name (`pi`) is passed to the agent as
      the sub-provider; the `default_provider` → `--provider <backend>` mapping is stated or linked.
- [ ] README contains **no unqualified** "stager cannot commit/amend/push" / "structurally
      constrained" claim. (If such language is added or exists, it must be qualified for pi.)
- [ ] README explicitly documents that `config init` / `config path` / `config upgrade` honor the
      `--config` flag and `STAGECOACH_CONFIG` env var.
- [ ] README's bootstrap description accurately reflects that pi's per-role models are blanked
      (pi picks its own backend default) pending a configured `default_provider`.
- [ ] No stale claim about post-arbiter output / SHAs remains (Issue 3 — verification).
- [ ] `go build ./...` and `go test ./...` are green and **unchanged** by this task.
- [ ] `git status --short` shows ONLY `README.md` modified (plus any plan/ artifacts).

## All Needed Context

### Context Completeness Check

**Pass** — this PRP quotes the exact shipped behavior for each of the five fixes (from the committed
code + architecture docs), maps each fix to the exact README line range, gives before/after wording
for the required edits, and lists the verification commands. An agent who has never seen this repo
can complete it from this file + `README.md`.

### Documentation & References

```yaml
# MUST EDIT — the single file this task touches
- file: README.md
  why: The top-level project overview (GitHub landing page + quick start). ~305 lines.
  critical: |
    This is the ONLY file you may modify for P1.M6.T1.S1. Target the four sections identified in the
    "What" table: provider-setup block (~L167), bootstrap line (~L172), config-override paragraph
    (~L189), and the four-role pipeline / FAQ safety language (~L32, ~L114, ~L268). Keep edits tight
    and in the README's existing voice (terse, confident, `[!NOTE]` callouts).

# MUST READ — the shipped behavior each fix delivered (authoritative, since all subtasks are Complete)
- file: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue1_provider_conflation.md
  why: Explains cfg.Provider (manifest name) vs DefaultProvider (sub-provider). After the fix, callers
        pass "" so Render falls back to *r.DefaultProvider — emits --provider <sub-provider>, or omits
        the flag entirely when default_provider is "".
  critical for README: the value rendered to --provider is default_provider (e.g. "zai"/"openrouter"),
        NEVER "pi". When default_provider is unset (the pi shipped default), pi is invoked with NO
        --provider and routes the model on its own default backend ("google").

- file: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue2_stager_toolset.md
  why: Three-pronged fix. (a) claude TooledFlags tightened to Bash(git add:*,git apply:*,git
        status:*,git diff:*),Read,Edit — structurally excludes commit/amend/push. (b) pi honestly
        documented as NOT flag-scoped. (c) HEAD-movement guard added.
  critical for README: PRD §19's "structurally constrained … cannot commit/amend/push" claim holds
        for CLAUDE but does NOT hold for PI. pi's stager is instructionally constrained + HEAD-guarded.
        README must not state an unqualified structural guarantee.

- file: providers/pi.toml
  why: The reference doc for pi's shipped safety model — the honest "SAFETY MODEL — HONEST" comment
        block. This is the canonical wording to mirror/echo in README's optional stager sentence.
  critical: quotes the exact distinction — "claude's stager IS structurally constrained … pi's stager
        is NOT … INSTRUCTIONALLY constrained (§17.6 stager task prompt) + BEST-EFFORT guarded
        (HEAD-movement defense-in-depth check). THE SAFETY NET IS THIS GUARD, NOT FLAG-SCOPING."

- file: providers/claude.toml
  why: Shows the tightened claude tooled_flags allowlist (staging-only git subcommands). Reference.

- file: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue4_config_path_override.md
  why: ResolveConfigPath(flagConfig) honors --config > STAGECOACH_CONFIG > GlobalConfigPath(). The
        config subcommands init/upgrade/path now use it.
  critical for README: `stagecoach --config X config upgrade` upgrades X (not the global file);
        `config path` prints the resolved (overridden) path. This was previously BROKEN (always
        global) — README must document the now-correct behavior.

- file: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue5_bootstrap_config.md
  why: Option A shipped — pi's per-role models blanked (model = "") so pi picks its own backend
        default, with a sub-provider NOTE annotation in the generated TOML.
  critical for README: README's "writes per-role model defaults" is only fully accurate for non-pi.
        For pi the bootstrap writes empty models (pi picks its own backend); user sets
        [provider.pi] default_provider to pin a backend.

- file: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue3_post_arbiter_output.md
  why: After a successful arbiter, final commits are re-read via LogRange(preRunHEAD..HEAD); printed
        SHAs are now accurate and the arbiter's new (N+1)-th commit is reported.
  critical for README: README makes no specific SHA-output claim today, so this is a VERIFICATION
        pass (confirm no stale claim). No edit expected.

# SUPPORTING — sibling task boundary (do NOT cross it)
- file: docs/README.md
  why: The docs/ index. Note its rule: "If anything here disagrees with stagecoach --help, the binary
        is authoritative." README inherits the same posture.
  critical: docs/*.md (cli.md, configuration.md, providers.md, how-it-works.md) are owned by
        P1.M6.T1.S2. Do NOT edit them here. If you find a docs/ inconsistency, leave it for S2.

# SUPPORTING — markdown style rules in force
- file: .markdownlint.json
  why: markdownlint config. default:true with MD013 (line-length) OFF, MD033 (inline-HTML) OFF,
        MD060 (no-blockquote) OFF.
  critical: README legitimately uses `> [!NOTE]`/`> [!TIP]` callouts (needs MD060 off) and
        `<details>`/inline badges (needs MD033 off), and long lines are fine (MD013 off). Match this
        style. markdownlint is NOT in Makefile or CI and may not be installed (see Validation L1).
```

### Current Codebase tree (relevant slice)

```bash
# Only this file is in scope:
README.md                       # ← EDIT (the deliverable)
# Reference docs (READ-ONLY for this task):
providers/pi.toml               # pi safety model — canonical honest wording to echo
providers/claude.toml           # claude tightened allowlist
docs/README.md                  # docs/ index — boundary with P1.M6.T1.S2
docs/{cli,configuration,providers,how-it-works}.md  # OUT OF SCOPE (sibling task S2)
PRD.md                          # READ-ONLY (owned by humans)
.markdownlint.json              # markdown style rules
```

### Desired Codebase tree

```bash
# No files added or deleted. Exactly ONE file edited:
README.md                       # five behavioral areas synced (Issues 1–5)
```

### Known Gotchas of our codebase & Library quirks

```markdown
<!-- CRITICAL — README is Markdown, the only deliverable; do not touch any .go / .toml / docs/* file.
     git status --short after your work must show ONLY `README.md` (plus plan/ PRP/research artifacts). -->

<!-- GOTCHA — do not over-claim the stager's safety (Issue 2). README today makes NO §19 "structurally
     constrained" claim. The job is to (a) confirm none sneaks in, and (b) optionally add ONE honest
     sentence distinguishing claude (structurally scoped) from pi (instructional + HEAD guard). Do NOT
     write "the stager cannot commit" without qualifying it for pi — pi is NOT flag-scoped. -->

<!-- GOTCHA — Issue 3 is a verification pass, not an edit, unless you find a stale claim. README's
     multi-commit demo prints `[<sha>] <subject>` lines but makes no promise about SHA accuracy, so
     the Issue-3 fix (accurate post-arbiter SHAs) does NOT contradict anything currently in README.
     Do not invent a new output-fidelity section; that belongs in docs/how-it-works.md (task S2). -->

<!-- GOTCHA — Issue 1 detail that's easy to misstate: `--provider <name>` AT THE STAGECOACH CLI
     selects the agent/manifest (correct, keep README's line ~189 wording). The DIFFERENT rendered
     flag `pi --provider <backend>` selects pi's backend and comes from default_provider. These are
     two different "--provider" tokens in two different programs. Don't conflate them; a one-line
     clarification for pi is enough (link to docs/providers.md rather than inlining the full schema). -->

<!-- GOTCHA — markdownlint style (config in .markdownlint.json): MD013/MD033/MD060 are OFF, so long
     lines, inline HTML (`<details>`, badge `<img>`), and `> [!NOTE]` callouts are all fine. Match
     the existing README voice; reuse `> [!NOTE]` for the new clarifications. -->

<!-- GOTCHA — scope boundary: docs/*.md is task P1.M6.T1.S2. If the same stale claim exists in
     docs/configuration.md or docs/providers.md, DO NOT fix it here — that's S2's job. Stay in README.md. -->
```

## Implementation Blueprint

### Data models and structure

None — this is a documentation task. No types, schemas, or data structures are involved.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: VERIFY current README claims against shipped behavior (read-first; no edits yet)
  - READ: README.md in full (~305 lines).
  - GREP to confirm the absence of stale §19-style claims before editing:
      grep -n -iE "cannot commit|cannot amend|cannot push|structurally constrained|structurally unable|never commit" README.md
      grep -n -iE "default_provider|sub-provider|backend" README.md
      grep -n -iE "config init|config upgrade|config path|--config|STAGECOACH_CONFIG" README.md
  - EXPECT: the first grep returns NOTHING (README makes no structural-constraint claim today — this
    is the Issue-2 baseline). Record the exact line numbers the other two greps return; those are your
    edit anchors (provider-setup block ~L167, bootstrap ~L172, config-override ~L189).
  - DECISION RULE: if the first grep DID hit a line (a stale §19 claim), that line is the #1 priority
    edit — qualify it for pi per the pi.toml "SAFETY MODEL — HONEST" wording. If it hit nothing, proceed.

Task 1: EDIT README — Issue 4 (config subcommands honor --config / STAGECOACH_CONFIG)  [the clearest edit]
  - LOCATE: the paragraph beginning "Point discovery at a specific file with" (line ~189). Current text:
      "Point discovery at a specific file with `stagecoach --config path/to/config.toml`. It is honored
      by every command — including the default commit action — so a provider declared under
      `[provider.<name>]` there is usable with `--provider <name>` directly. The path must exist: an
      explicit `--config` (or `STAGECOACH_CONFIG`) pointing at a missing file fails fast with exit 1
      rather than silently falling back to auto-detection."
  - CHANGE: after the first sentence (or appended to it), make the config-subcommand honoring explicit.
    Suggested wording (keep terse; reuse `> [!NOTE]` if cleaner as a callout):
      "It is honored by every command — including the default commit action **and the `config init`,
      `config path`, and `config upgrade` subcommands** (e.g. `stagecoach --config X config upgrade`
      upgrades file `X`, and `config path` prints the resolved path) — so a provider declared under
      `[provider.<name>]` there is usable with `--provider <name>` directly."
  - PRESERVE: the "path must exist … fails fast with exit 1" sentence (unchanged, still accurate).
  - RATIONALE: Issue 4 made the subcommands override-aware; README previously implied only the default
    action honored the override. This is the highest-value, most concrete edit.

Task 2: EDIT README — Issue 5 (bootstrap blanks pi per-role models)
  - LOCATE: the line "Or bootstrap a **populated, working config** (auto-detects your agent and writes
    per-role model defaults):" (line ~172), immediately above the `config init` code block.
  - CHANGE: adjust "writes per-role model defaults" so it is accurate for pi (the default). Suggested:
      "Or bootstrap a **populated, working config** (auto-detects your agent and writes per-role model
      defaults — for **pi**, the default, per-role models are left empty so pi picks its own backend
      model; set `[provider.pi] default_provider` to pin a specific backend):"
  - PRESERVE: the `config init` / `config path` / `config upgrade` code block and the `[generation]`
    `> [!NOTE]` that follow (unchanged).
  - RATIONALE: Issue 5 blanked pi's models; "writes per-role model defaults" now understates the pi
    nuance. The "working" claim stays TRUE (empty models ⇒ pi picks its own default), but the text
    should say so and point at default_provider.

Task 3: EDIT README — Issue 1 (sub-provider comes from default_provider, not the manifest name)
  - LOCATE: the provider-setup block (lines ~165–168):
      "Set a per-repo default with git config:
        git config stagecoach.provider pi
        # Optionally pin a model (overrides the per-provider default):
        git config stagecoach.model sonnet"
  - CHANGE: add a short clarifying note (a `> [!NOTE]` or 1–2 sentences) right after this block, before
    the "Or bootstrap" paragraph. Suggested:
      "> [!NOTE]
      > `pi` is a multi-provider agent: the `--provider` flag it receives selects a *backend*
      > (`zai`, `openrouter`, `anthropic`, …). That backend comes from `[provider.pi] default_provider`
      > in your config — **not** the manifest name. If `default_provider` is unset, pi is invoked with
      > no `--provider` and routes the model on its own default backend. See
      > [Provider manifests](docs/providers.md) for the full schema."
  - PRESERVE: the existing `git config stagecoach.provider pi` example (still the recommended setup and
    now actually correct post-Issue-1).
  - RATIONALE: Issue 1 is the Critical fix; the README must not let a reader infer that
    `--provider pi` (manifest name) is passed to pi. The Stagecoach-CLI `--provider <name>` (agent
    selection, line ~189) is a DIFFERENT token — don't conflate; keep that wording intact.

Task 4: VERIFY + (optional) light EDIT — Issue 2 (stager safety honesty) and Issue 3 (output fidelity)
  - STEP A (Issue 2 — mandatory verification): run the Task-0 grep for structural-constraint language.
    - If it HITS a stale line → EDIT that line to qualify for pi using the pi.toml "SAFETY MODEL —
      HONEST" distinction (claude structurally scoped; pi instructional + HEAD-movement guard).
    - If it HITS NOTHING (expected) → README is already honest; no mandatory edit.
  - STEP B (Issue 2 — optional, only if it reads naturally): near the four-role pipeline mention
    (line ~114, "planner → stager → message → arbiter"), you MAY add ONE honest sentence, e.g. as a
    `> [!NOTE]`:
      "> [!NOTE]
      > The stager is constrained to staging operations: claude via a staging-only git allowlist
      > (`git add`/`apply`/`status`/`diff`); pi instructionally (its task prompt) plus a HEAD-movement
      > guard that aborts the run if the stager moves a ref. Either way, stagecoach still owns every
      > commit via git plumbing."
    Keep this OPTIONAL and terse — README is an overview, not a security spec. Do NOT add a structural
    guarantee that doesn't hold for pi.
  - STEP C (Issue 3 — verification only): confirm README's multi-commit demo output (lines ~116–118,
    the `[<sha>] <subject>` + file-list lines) makes no accuracy/staleness promise that Issue 3
    contradicts. It does not. NO edit required. If (unexpectedly) you find an output claim, soften it
    to "the final commits (re-read after the arbiter reconciles leftovers)".
  - RATIONALE: README does not today carry the §19 claim or a SHA-output claim, so these are primarily
    verification passes with one optional honest note. Do NOT expand README into a stager/security spec.

Task 5: FINAL pass — consistency + link integrity
  - Re-read the edited README end-to-end; confirm voice/tense matches the rest of the file.
  - Confirm the `[Provider manifests](docs/providers.md)` link you added (Task 3) points at an existing
    file (it does: docs/providers.md).
  - Confirm no claim in README now contradicts the shipped behavior summarized in the architecture docs.
  - Confirm you touched ONLY README.md.
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: reuse the README's existing callout style (`> [!NOTE]` / `> [!TIP]`) for new notes.
     It's already used at lines ~104, ~163, ~187 and is markdownlint-clean (MD060 off). -->

<!-- PATTERN: keep edits surgical. README is a curated overview; deep schema/precedence detail belongs
     in docs/ (S2). In README, state the accurate behavior in one or two sentences + a docs/ link. -->

<!-- PATTERN: when naming two different "--provider" tokens, disambiguate in prose:
       - Stagecoach CLI `--provider <name>`    → selects the AGENT/manifest (README L189 is correct).
       - pi's rendered `--provider <backend>` → selects the BACKEND, from default_provider (new note).
     Don't let a reader conflate them. -->

<!-- KEY DETAIL (Issue 1): when default_provider is "" (pi's shipped default), pi gets NO --provider
     flag at all — not `--provider ""`. State it as "pi is invoked with no --provider". -->

<!-- KEY DETAIL (Issue 4): both override forms work in subcommands: `--config X` (flag) AND
     STAGECOACH_CONFIG=X (env). Flag beats env (matches config.Load precedence). State both. -->

<!-- KEY DETAIL (Issue 5): "working config" stays TRUE — empty models are valid (pi picks a backend
     default). Don't weaken FR-B1's "works immediately"; clarify WHY it still works. -->
```

### Integration Points

```yaml
NONE. This is a standalone Markdown edit. It does not:
  - change any Go source, import, or interface;
  - change any providers/*.toml (already updated by P1.M2.T1.S1/S2);
  - change any docs/*.md (that is P1.M6.T1.S2);
  - change PRD.md (read-only, human-owned);
  - change any build/test/lint wiring.

The only cross-reference is the new [Provider manifests](docs/providers.md) link (Task 3), which
already resolves. No anchors needed (the docs/providers.md page exists; link to the page, not a
specific heading, to avoid anchor rot).
```

## Validation Loop

### Level 1: Markdown hygiene (immediate)

```bash
# Confirm the file still parses as sane Markdown and links resolve. markdownlint is configured
# (.markdownlint.json) but NOT wired into the Makefile or CI, and may not be installed locally.
# Run it ONLY if available; if npx offers to download it, that's fine but optional.
npx --yes markdownlint-cli2 README.md 2>/dev/null || \
npx --yes markdownlint README.md 2>/dev/null || \
echo "markdownlint not available — skipped (it is optional per .markdownlint.json; not in CI)."
# Expected (if run): zero errors. NOTE: MD013 (line-length), MD033 (inline HTML), MD060 (blockquote)
# are DISABLED in .markdownlint.json, so long lines, <details>/<img>, and > [!NOTE] are all fine.

# Lightweight link/target check (no network; local anchors + docs/ links):
grep -oE '\]\([^)]+\)' README.md | grep -v 'http' | sort -u
# Eyeball that every `](path)` target exists (notably docs/providers.md, docs/cli.md, docs/how-it-works.md).
```

### Level 2: Regression guard — Go still builds and tests (docs must not break it)

```bash
# Docs changes must NOT affect the build or tests. This is the contract's stated test gate.
go build ./...
go test ./...
# Expected: BUILD clean, all tests PASS (same as before the docs edit). If anything regresses, you
# accidentally edited a .go file — revert it; this task is README-only.
```

### Level 3: Behavioral accuracy spot-check (manual, against the shipped binary)

```bash
# Optional but recommended — prove the README claims match the binary. Build first:
make build 2>/dev/null || go build -o bin/stagecoach ./cmd/stagecoach
SH=$(pwd)/bin/stagecoach

# Issue 4 — config path honors --config:
TMP=$(mktemp -d); mkdir -p "$TMP"
STAGECOACH_CONFIG="$TMP/my.toml" $SH config path   # Expected: prints "$TMP/my.toml" (not global)
# Issue 4 — config init honors --config:
STAGECOACH_CONFIG="$TMP/my.toml" $SH config init   # Expected: "Wrote config to $TMP/my.toml"
grep -c '^\[role' "$TMP/my.toml"                   # Expected: 4 role blocks
# Issue 5 — pi bootstrap blanks models:
grep -E '^model' "$TMP/my.toml"                    # Expected: four `model = ""` lines + a NOTE about default_provider
# Issue 1 — pi rendered command omits --provider when default_provider unset:
$SH providers show pi 2>/dev/null | grep -i 'default_provider' || true
# (The above confirm the README statements you wrote/verified. If any disagrees with the README,
# fix the README, not the binary — the binary is authoritative per docs/README.md.)
```

### Level 4: Consistency read-through

```bash
# Confirm only README.md changed (plus plan/ artifacts):
git status --short
# Expected: ` M README.md` and plan/002_*/.../P1M6T1S1/{PRP.md,research/} only. NO docs/, NO *.go, NO *.toml.

# Final read of the edited sections for voice/accuracy:
sed -n '150,200p' README.md   # provider-setup + bootstrap + config-override paragraphs
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` clean (unchanged by this task).
- [ ] `go test ./...` green (unchanged by this task).
- [ ] markdownlint clean IF run (MD013/MD033/MD060 are disabled; callouts/inline-HTML/long lines OK).
- [ ] All in-file links/anchors in README resolve (notably `docs/providers.md`).

### Feature Validation (all five fixes reflected)

- [ ] **Issue 1**: README does not imply the manifest name is the pi sub-provider; the
      `default_provider` → rendered `--provider <backend>` mapping is stated (or clearly linked).
- [ ] **Issue 2**: README contains NO unqualified "stager cannot commit/amend/push" / "structurally
      constrained" claim; any safety language is honest about claude (scoped) vs pi (instructional +
      HEAD guard).
- [ ] **Issue 3**: README has no stale SHA/output claim (verification pass — no edit unless found).
- [ ] **Issue 4**: README explicitly states `config init` / `config path` / `config upgrade` honor
      `--config` and `STAGECOACH_CONFIG`.
- [ ] **Issue 5**: README's bootstrap text reflects that pi's per-role models are blanked (pi picks
      its own backend default; set `default_provider` to pin a backend).

### Code Quality / Scope Validation

- [ ] `git status --short` shows ONLY `README.md` (plus plan/ artifacts) — no `.go`, no `.toml`, no
      `docs/*.md`, no `PRD.md`.
- [ ] Edits reuse the README's existing voice and `> [!NOTE]` callout style.
- [ ] Scope boundary respected: `docs/*.md` left untouched (sibling task **P1.M6.T1.S2**).
- [ ] No new structural/stale claims introduced; README remains a curated overview (not a security spec).

### Documentation

- [ ] The README's stated behaviors match the shipped binary (validated in Level 3).
- [ ] Where a behavior is too detailed for the overview, README links to the relevant `docs/` page.

---

## Anti-Patterns to Avoid

- ❌ Don't edit anything other than `README.md` — no `.go`, no `providers/*.toml`, no `docs/*.md`, no `PRD.md`.
  (`docs/` is task P1.M6.T1.S2; PRD and source are read-only.)
- ❌ Don't add an unqualified "the stager cannot commit/amend/push" claim — it does NOT hold for pi.
  Qualify it (claude scoped; pi instructional + HEAD guard) or leave the §19 claim out entirely.
- ❌ Don't conflate the two `--provider` tokens — Stagecoach's CLI `--provider <name>` (agent selection)
  is different from pi's rendered `--provider <backend>` (from default_provider). Disambiguate in prose.
- ❌ Don't state pi receives `--provider ""` — when default_provider is unset, pi gets NO `--provider`
  flag at all. Say "no `--provider`".
- ❌ Don't weaken or remove FR-B1's "works immediately / working config" promise — it's still true
  (empty models ⇒ pi picks its own backend default). Clarify *why*, don't delete it.
- ❌ Don't invent a new post-arbiter output-fidelity section (Issue 3) — README makes no SHA claim
  today; that detail lives in docs/how-it-works.md (S2). Just verify nothing is stale.
- ❌ Don't skip the Go build/test gate — docs edits must not break the build; if they do, you edited
  code by accident.
- ❌ Don't rely on markdownlint being installed — it's configured but not in Makefile/CI. Run it only
  if present; the real gates are `go build ./...`, `go test ./...`, and a manual accuracy read.
