name: "P1.M1.T3.S1 — Fix docs/configuration.md git-config key spelling (auto_stage_all → autoStageAll) and verify all multi-word git keys are camelCase"
description: >
  Documentation-only fix for PRD Issue 2 (§9.8 FR36). Git rejects underscores in the final segment of
  a config-key name, so the documented `stagecoach.auto_stage_all` is un-settable (`error: invalid
  key`). The implementation (`internal/config/git.go:158`) reads the camelCase key `stagecoach.autoStageAll`
  exclusively — every one of the 8 multi-word git keys is camelCase. Reconcile the DOCS to the code by
  fixing exactly two lines: the `[stagecoach]` INI example (`auto_stage_all` → `autoStageAll`) and the
  git-config table row (`stagecoach.auto_stage_all` → `stagecoach.autoStageAll` in all occurrences).
  Then verify (grep) that no other git-config key uses snake_case. NO Go code and NO test changes —
  this subtask IS the documentation deliverable (PRD "[Mode A]"). Optional stretch: add 6 missing
  git-config rows to the table for completeness.

---

## Goal

**Feature Goal**: Make every git-config key documented in `docs/configuration.md` match the spelling
actually read by `internal/config/git.go`, so a user who follows the docs can persistently disable
auto-stage-all via `git config stagecoach.autoStageAll false` (which works) instead of the documented
`stagecoach.auto_stage_all` (which git rejects with `error: invalid key`).

**Deliverable**: Two edits to `docs/configuration.md` (no other file touched):
1. The `[stagecoach]` INI example: `auto_stage_all = true` → `autoStageAll = true`.
2. The git-config table row: `stagecoach.auto_stage_all` → `stagecoach.autoStageAll` in both the Key
   column and the `git config --get --bool …` Reads-with column (Description prose unchanged).
Plus a grep verification that no other snake_case git-config key remains.

**Success Definition**:
- `grep -n 'auto_stage_all' docs/configuration.md` returns **nothing** (the only two occurrences are fixed).
- `grep -nE 'stagecoach\.[a-z]+_[a-z]' docs/configuration.md` returns **only line 229** (`... and no
  \`stagecoach.max_commits\`.` — intentional prose documenting a non-existent key; NOT a discrepancy).
- The `[stagecoach]` INI example block contains only camelCase / single-word keys.
- Every git-config key in the table is a key `git.go` actually reads (and is therefore settable via
  `git config`). Empirically: `git config stagecoach.autoStageAll false` succeeds (exit 0) and
  `git config stagecoach.auto_stage_all false` fails with `error: invalid key` (exit 1).
- No Go code or test file changed; `make test` stays green (no behavioral change).

## User Persona (if applicable)

**Target User**: A developer who wants to **persistently** disable auto-stage-all per-repo via git
config, by following the documentation.

**Use Case**: `git config stagecoach.autoStageAll false` in a repo so stagecoach never auto-stages
leftover files — the persistent, per-repo analog of the `--no-auto-stage` flag.

**User Journey**: Before this fix: the docs told users to run `git config stagecoach.auto_stage_all
false`, which git rejects (`error: invalid key`) — the key is never set and auto-stage silently stays
on. After this fix: the docs name the key the code reads (`stagecoach.autoStageAll`); the command
succeeds and correctly disables auto-stage.

**Pain Points Addressed**: PRD Issue 2 — "the method the docs tell users to use is un-settable." With
S1 (`*bool` overlay, done), S2 (e2e, done), T2 (env var, in-flight), and T3 (these docs), the full
FR34 precedence ladder for `auto_stage_all` is restored AND correctly documented.

## Why

- **Issue 2 (Major)**: `docs/configuration.md` lines 210 + 218 document `stagecoach.auto_stage_all`,
  but git forbids underscores in the final config-key component, so the command errors out and sets
  nothing. The implementation reads `stagecoach.autoStageAll` (`git.go:158`), confirmed by the existing
  `TestLoadGitConfig_CamelCaseKeysOnly` (`git_test.go:304`, which asserts a snake_case key yields
  "invalid key" at lines 326–328). The code is internally consistent (all 8 multi-word keys camelCase);
  the docs are the side that must change.
- **Complementary to S1/T2**: S1 made `false` survive the `*bool` overlay end-to-end; T2 adds the env
  source. Both are wasted if the docs still point users at a key that cannot be set. T3 closes the
  "correctly-documented" gap.
- **Bounded scope**: pure docs. Two edits + a grep. No compilation, no migration, no test. The optional
  6-row table completion improves completeness but is explicitly out of strict PRD scope.

## What

**User-visible behavior**: The git-config section of `docs/configuration.md` names only keys that git
will accept and that `git.go` will read. A user copy-pasting the INI example or the `git config …`
commands gets working results instead of `error: invalid key`.

**Technical change**: two text substitutions in `docs/configuration.md`, located by content (not by
line number, because the in-flight sibling P1.M1.T2.S1 inserts ~2 rows above and shifts line numbers):

| Anchor (content) | Before | After |
|------------------|--------|-------|
| `[stagecoach]` INI block, the `= true` key | `    auto_stage_all = true` | `    autoStageAll = true` |
| git-config table row (Key col + Reads-with col) | `stagecoach.auto_stage_all` (×2 in the row) | `stagecoach.autoStageAll` |

The Description column text ("Auto-stage all when nothing staged") is human prose and is UNCHANGED.

### Success Criteria
- [ ] `grep -n 'auto_stage_all' docs/configuration.md` → no output.
- [ ] The `[stagecoach]` INI example shows `autoStageAll = true` (camelCase), 4-space indented.
- [ ] The git-config table row shows `stagecoach.autoStageAll` in the Key column and in the
      `git config --get --bool stagecoach.autoStageAll` Reads-with column.
- [ ] `grep -nE 'stagecoach\.[a-z]+_[a-z]' docs/configuration.md` → only line 229 (`... no
      \`stagecoach.max_commits\`.`), which is intentional prose (a key that does NOT exist) and is left
      untouched.
- [ ] Empirical reproduction (scratch repo): `git config stagecoach.autoStageAll false` exits 0;
      `git config stagecoach.auto_stage_all false` exits 1 with `error: invalid key`.
- [ ] No Go file changed (`git diff --stat -- '*.go'` empty); `make test` green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — exact before/after text, the content-anchored locate strategy (grep), the two-layer rule
(git-config keys = camelCase; TOML config-file keys = snake_case, leave alone), the list of lines that
must NOT be touched, the empirical git reproduction commands, and the markdownlint baseline.

### Documentation & References

```yaml
# MUST READ — the authoritative research (verdict + the full key list)
- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/architecture/research_gitconfig_keys.md
  why: "§1 lists every git key git.go reads (all camelCase). §2 is the discrepancy table (exactly lines
        210 + 218). §3 is the optional '6 missing rows' finding. This PRP operationalizes §2."
  critical: "§2's 'NOT discrepancies' list (TOML keys at lines 87, 106–118, 133–151, 166) MUST be left
             alone — they are config-file keys, a different layer that legitimately uses snake_case."

- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M1T3S1/research/verification_notes.md
  why: "Implementation-time guardrails: exact current text of the 2 target lines, the grep design, the
        line-229 false-positive explanation, the markdownlint baseline (5 pre-existing errors, none at
        210/218), and the parallel-execution line-shift note (T2.S1 inserts ~2 rows above)."

# MUST READ — the file being edited
- file: docs/configuration.md
  why: "The git-config section ('## Git-config keys') holds both target lines: the `[stagecoach]` INI
        fenced block and the table immediately below it."
  pattern: "Table rows are 4 columns: '| Key | Type | Reads with | Description |'. The INI block is a
            ```ini fenced block under the `[stagecoach]` section header."
  gotcha: "T2.S1 (parallel) inserts ~2 rows in the env-var table ABOVE this section, shifting line
           numbers by +2. Locate edits by content via grep, NOT by hardcoded line number."

# MUST READ — the source of truth for key spelling
- file: internal/config/git.go
  why: "loadGitConfig (≈line 110–235) is the SOLE reader of the git-config layer. Line 158 reads
        `stagecoach.autoStageAll` (camelCase). Lines 102–105 comment 'KEY NAMES ARE CAMELCASE: git
        config rejects underscores (\"invalid key\").' All 8 multi-word keys are camelCase."
  pattern: "gitConfigBool(repoDir, \"stagecoach.autoStageAll\") / gitConfigGet(repoDir, \"stagecoach.maxDiffBytes\")."
  critical: "Do NOT 'fix' the code to match the docs — the code is correct (camelCase is forced by git).
             The DOCS are wrong; fix only docs/configuration.md."

# CONFIRMING — the test that pins the contract (no test change needed, but cite it)
- file: internal/config/git_test.go
  why: "TestLoadGitConfig_CamelCaseKeysOnly (≈line 304) sets `stagecoach.autoStageAll` (succeeds) and
        at ≈lines 326–328 asserts a snake_case key write produces 'invalid key' in stderr. This is the
        empirical proof behind this docs fix."
  critical: "This task changes NO .go file and NO test. If you feel compelled to edit git.go/git_test.go
             you have left scope — stop."

# CONTEXT — sibling task that touches the SAME file (parallel execution)
- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M1T2S1/PRP.md
  why: "T2.S1 inserts two env-var table rows into docs/configuration.md (~line 199–201, between
        STAGECOACH_PUSH and '## Git-config keys'). Different section from T3.S1's git-config edits, so
        no content conflict — but line numbers below shift by ~2."
  critical: "This is why T3.S1 MUST locate edits by content (grep), not by line number."
```

### Current Codebase tree (relevant slice)

```bash
docs/
  configuration.md     # EDIT HERE — 2 lines: [stagecoach] INI block + git-config table row
internal/config/
  git.go               # SOURCE OF TRUTH — reads stagecoach.autoStageAll @158 (camelCase). NOT EDITED.
  git_test.go          # TestLoadGitConfig_CamelCaseKeysOnly @304 pins the contract. NOT EDITED.
.markdownlint.json     # {default:true, MD013:false, MD033:false, MD060:false}
Makefile               # `make test` (race suite); `make lint` (golangci-lint) — .md not linted by these
```

### Desired Codebase tree with files to be added/modified

```bash
docs/configuration.md  # MODIFY: 2 text substitutions (snake_case → camelCase). No new files.
# (OPTIONAL, see Task 4: +6 rows to the git-config table for completeness — out of strict PRD scope)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (two layers, one rule): git-config keys (prefixed `stagecoach.` or inside a `[stagecoach]`
     INI block) MUST be camelCase — git rejects underscores in the final key segment. TOML config-file
     keys (under `[defaults]` / `[generation]`, or bare option names) legitimately use snake_case.
     Conflating the two is the #1 way to corrupt this fix. -->

<!-- CRITICAL (do NOT 'fix' line 229): `> ... and no \`stagecoach.max_commits\`.` documents a key that
     deliberately does NOT exist. It is prose, not a settable snake_case key. Leave it. After your fix,
     `grep -nE 'stagecoach\.[a-z]+_[a-z]'` SHOULD still match exactly this one line. -->

<!-- CRITICAL (locate by content, not line number): sibling T2.S1 inserts ~2 rows above this section,
     shifting lines 210→~212 and 218→~220. Use `grep -n 'auto_stage_all' docs/configuration.md` to find
     the exact lines; it returns exactly the two to fix regardless of upstream drift. -->

<!-- CRITICAL (Description prose is NOT a key): the table row's last column reads "Auto-stage all when
     nothing staged" — no underscore, human prose. Do not change it. Only the two `auto_stage_all`
     key-name tokens change to `autoStageAll`. -->

<!-- SCOPE FENCE (lines to leave alone): 87 (`# auto_stage_all = true` TOML), 106–118 (`[generation]`
     TOML block), 133–151 (Built-in defaults table — option names), 166 (prose already uses
     `autoStageAll`). None of these are git-config keys. -->

<!-- SCOPE FENCE (no code): this task edits docs/configuration.md ONLY. git.go is the source of truth
     and is already correct. git_test.go already pins the camelCase contract. Editing any .go file is
     out of scope. -->
```

## Implementation Blueprint

### Data models and structure
None. No types, no code. Pure documentation text edits.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: LOCATE the two edits by content (robust to parallel line-number drift)
  - RUN: grep -n 'auto_stage_all' docs/configuration.md
  - EXPECT: exactly two hits — one inside the ```ini [stagecoach] fenced block (the `    auto_stage_all = true`
    line) and one in the git-config table row (| `stagecoach.auto_stage_all` | ...).
  - NOTE: if sibling T2.S1 has already landed, these are ~line 212 and ~220 (not 210/218). The grep is
    authoritative; line numbers are advisory only.

Task 2: EDIT docs/configuration.md — the [stagecoach] INI example (mandatory)
  - FIND (inside the ```ini [stagecoach] block): the line `    auto_stage_all = true` (4-space indent).
  - REPLACE WITH: `    autoStageAll = true`
  - PRESERVE: the 4-space indentation and the surrounding keys (provider/model/timeout are single-word,
    unchanged). The fenced ```ini block stays fenced.

Task 3: EDIT docs/configuration.md — the git-config table row (mandatory)
  - FIND the row (4 columns: Key | Type | Reads with | Description):
        | `stagecoach.auto_stage_all` | bool | `git config --get --bool stagecoach.auto_stage_all` | Auto-stage all when nothing staged |
  - REPLACE WITH (only the two `auto_stage_all` tokens change → `autoStageAll`; Description unchanged):
        | `stagecoach.autoStageAll` | bool | `git config --get --bool stagecoach.autoStageAll` | Auto-stage all when nothing staged |
  - PRESERVE: the 4-column pipe layout, the backtick wrapping, the bool type, and the Description text.
    Do NOT touch any other table row.

Task 4: (OPTIONAL — out of strict PRD scope) ADD the 6 missing git-config rows for completeness
  - CONTEXT: the git-config table currently lists 11 keys git.go reads, omitting 6 (research §3). Adding
    them makes the table complete and is low-risk, but it is NOT required by PRD Issue 2 and may be
    SKIPPED without failing this task. If you do it, append these 6 rows to the existing table (after the
    `stagecoach.push` row, before the `> [!NOTE]` callout), matching the 4-column format:
        | `stagecoach.verbose` | bool | `git config --get --bool stagecoach.verbose` | Print resolved command and agent output (verbose mode) |
        | `stagecoach.noVerify` | bool | `git config --get --bool stagecoach.noVerify` | Skip pre-commit and commit-msg hooks (§9.25 FR-V5; mirrors `git commit --no-verify`) |
        | `stagecoach.maxDiffBytes` | int | `git config --get stagecoach.maxDiffBytes` | Legacy per-section diff cap in bytes; ignored when `tokenLimit` > 0 (§9.1 FR3d) |
        | `stagecoach.maxMdLines` | int | `git config --get stagecoach.maxMdLines` | Legacy per-section line cap; ignored when `tokenLimit` > 0 (§9.1 FR3d) |
        | `stagecoach.maxDuplicateRetries` | int | `git config --get stagecoach.maxDuplicateRetries` | Max retry attempts on duplicate commit messages |
        | `stagecoach.subjectTargetChars` | int | `git config --get stagecoach.subjectTargetChars` | Target subject-line length in characters |
  - VERIFY each is camelCase and is a key git.go actually reads (see architecture/research_gitconfig_keys.md §1).
  - DECISION RULE: if in doubt or under time pressure, SKIP Task 4 — Tasks 1–3 are the complete
    deliverable. Document in the PR whether you included it.

Task 5: VERIFY — grep guards + empirical git reproduction + lint baseline + no-code guard
  - grep -n 'auto_stage_all' docs/configuration.md                       # MUST be empty
  - grep -nE 'stagecoach\.[a-z]+_[a-z]' docs/configuration.md             # MUST be only line ~229 (max_commits prose)
  - (see Validation Loop Level 3/4 for the empirical git reproduction and the markdownlint baseline check)
  - git diff --stat -- '*.go'                                            # MUST be empty (docs-only)
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: content-anchored locate (immune to T2.S1's line shift) -->
$ grep -n 'auto_stage_all' docs/configuration.md
210:    auto_stage_all = true
218:| `stagecoach.auto_stage_all` | bool | `git config --get --bool stagecoach.auto_stage_all` | Auto-stage all when nothing staged |
# (post-T2.S1 these become ~212 and ~220 — grep finds them either way)

<!-- PATTERN: the exact two edits -->
# Edit A (INI block):
-     auto_stage_all = true
+     autoStageAll = true

# Edit B (table row — two tokens change, Description column unchanged):
- | `stagecoach.auto_stage_all` | bool | `git config --get --bool stagecoach.auto_stage_all` | Auto-stage all when nothing staged |
+ | `stagecoach.autoStageAll` | bool | `git config --get --bool stagecoach.autoStageAll` | Auto-stage all when nothing staged |

<!-- PATTERN: the post-fix grep expectation (the line-229 false positive is intentional) -->
$ grep -nE 'stagecoach\.[a-z]+_[a-z]' docs/configuration.md
229:> The git-config layer has **no** per-role keys (`stagecoach.role.*`), no `stagecoach.commits`, and no `stagecoach.max_commits`. Per-role ...
# ↑ the ONLY hit. "no `stagecoach.max_commits`" documents a non-existent key — leave it.
```

### Integration Points

```yaml
NO code / database / migration / routes. ONE markdown file edited (docs/configuration.md).

DOCS LAYER (docs/configuration.md):
  - INI example block: `auto_stage_all` → `autoStageAll` (camelCase, matches git.go:158).
  - git-config table row: `stagecoach.auto_stage_all` → `stagecoach.autoStageAll` (Key + Reads-with cols).

COORDINATION (parallel sibling P1.M1.T2.S1): edits are in DIFFERENT sections of the same file (T2 =
  env-var table ~line 199–201; T3 = git-config section ~line 207–226). No content overlap → no merge
  conflict beyond line-number drift, which the content-anchored locate strategy absorbs.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# No Go compilation is involved (docs-only). Confirm no .go file changed:
git diff --stat -- '*.go'           # Expected: empty
git diff --stat                     # Expected: only docs/configuration.md (+ optionally its rows)

# Markdown lint baseline (5 pre-existing errors, NONE at lines 210/218; the fix adds none).
npx --no-install markdownlint-cli2 docs/configuration.md
# Expected: the SAME 5 pre-existing errors (≈lines 156,162,164,166,280) and NO error referencing the
# INI block or the auto_stage_all/autoStageAll row. Do NOT fix the unrelated 5 (out of scope).
```

### Level 2: Unit Tests (Component Validation)

```bash
# No tests are authored by this task. Run the suite to PROVE no behavioral regression (docs can't
# break Go tests, but this confirms the working tree is otherwise clean).
make test                           # Expected: green (race detector). No .go changes => identical to baseline.
```

### Level 3: Integration Testing (System Validation)

```bash
# EMPIRICAL PROOF that the docs now name a key git accepts and the code reads.
# Scratch repo: camelCase key succeeds (exit 0); snake_case key fails (exit 1, "invalid key").
d=$(mktemp -d) && cd "$d" && git init -q
git config stagecoach.autoStageAll false; echo "camelCase exit=$? (expect 0)"
git config stagecoach.auto_stage_all false; echo "snake_case exit=$? (expect 1)"
git config stagecoach.auto_stage_all false 2>&1 | grep -q 'invalid key' && echo "OK: 'invalid key' present"
cd - && rm -rf "$d"
# Expected: camelCase exit=0; snake_case exit=1 with 'error: invalid key: stagecoach.auto_stage_all'.
# (This reproduces git_test.go:326–328 at the shell, confirming the docs fix points at the working key.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1: the bug is gone (no snake_case 'auto_stage_all' token anywhere).
grep -n 'auto_stage_all' docs/configuration.md
# Expected: NO output.

# Grep guard 2: the camelCase key is now present exactly where expected.
grep -n 'autoStageAll' docs/configuration.md
# Expected: 2 hits in the git-config section — the INI block (`    autoStageAll = true`) and the table
#           row (`stagecoach.autoStageAll` ×2 within that row). (Plus possibly the pre-existing line 166
#           prose "stagecoach.autoStageAll-style", which was already correct.)

# Grep guard 3: NO other snake_case git-config key leaked in (the two-layer rule).
grep -nE 'stagecoach\.[a-z]+_[a-z]' docs/configuration.md
# Expected: EXACTLY ONE hit — line ~229, the intentional "no `stagecoach.max_commits`" prose. If a
#           second hit appears, you introduced/left a snake_case git key — fix it.

# Grep guard 4: the [stagecoach] INI block is clean (no underscored keys).
sed -n '/^```ini/,/^```/p' docs/configuration.md | grep -nE '^\s+\w*_\w*\s*='
# Expected: NO output (the only multi-word key is now camelCase `autoStageAll`).

# Scope guard: docs-only change.
git diff --name-only
# Expected: docs/configuration.md (only).
```

## Final Validation Checklist

### Technical Validation
- [ ] `git diff --stat -- '*.go'` empty (docs-only)
- [ ] `make test` green (no behavioral regression)
- [ ] `npx --no-install markdownlint-cli2 docs/configuration.md` shows no NEW errors (still the 5 pre-existing)

### Feature Validation
- [ ] `grep -n 'auto_stage_all' docs/configuration.md` → no output
- [ ] INI example shows `    autoStageAll = true` (camelCase, 4-space indent)
- [ ] Table row shows `stagecoach.autoStageAll` in Key column AND in `git config --get --bool stagecoach.autoStageAll` (Reads-with column); Description column unchanged
- [ ] `grep -nE 'stagecoach\.[a-z]+_[a-z]' docs/configuration.md` → only line ~229 (`... no \`stagecoach.max_commits\`.`)
- [ ] Empirical (Level 3): `git config stagecoach.autoStageAll false` exit 0; `git config stagecoach.auto_stage_all false` exit 1 with "invalid key"

### Scope-Boundary Validation
- [ ] Only docs/configuration.md changed
- [ ] TOML config-file keys untouched (lines 87, 106–118, 133–151)
- [ ] Line 166 prose (already uses `autoStageAll`) untouched
- [ ] Line 229's `stagecoach.max_commits` (intentional "does not exist" prose) untouched
- [ ] No .go file edited; git.go is the source of truth and was already correct
- [ ] (If Task 4 done) the 6 added rows are all camelCase keys git.go reads; (if skipped) clearly noted

### Code Quality & Docs
- [ ] Both edits are content-anchored (work regardless of T2.S1's line shift)
- [ ] Table column layout / backtick wrapping preserved
- [ ] No accidental change to the Description column or any other table row

---

## Anti-Patterns to Avoid

- ❌ Don't "fix" `git.go` to read `stagecoach.auto_stage_all` — the code is correct; git FORCES camelCase (underscores are invalid in the final key segment). The docs are wrong; fix docs only.
- ❌ Don't change TOML config-file keys (lines 87, 106–118, 133–151). `[defaults].auto_stage_all` / `[generation].max_diff_bytes` etc. are a DIFFERENT layer and legitimately use snake_case. The two-layer rule (git-config = camelCase; TOML = snake_case) is the crux of this task.
- ❌ Don't touch line 229's `stagecoach.max_commits`. It is prose documenting a key that deliberately does NOT exist. After your fix it SHOULD remain the sole `stagecoach.<snake>` hit.
- ❌ Don't change the table row's Description column ("Auto-stage all when nothing staged"). It is human prose with no underscore; only the two `auto_stage_all` key tokens change.
- ❌ Don't locate edits by hardcoded line numbers (210/218). Sibling T2.S1 inserts ~2 rows above, shifting them to ~212/220. Use `grep -n 'auto_stage_all'` to find them.
- ❌ Don't try to fix the 5 pre-existing markdownlint errors (≈156/162/164/166/280). They predate this task and are out of scope; your change must not ADD any.
- ❌ Don't bundle unrelated docs rewrites. If you notice line 166's prose is awkward (it implies multi_turn_fallback is governable by a git key, which git.go does not read), note it but do NOT fix it here — that is a separate docs-accuracy issue, not a snake_case→camelCase spelling fix, and is out of scope for T3.S1.
- ❌ Don't skip the empirical git reproduction (Level 3). It is the only objective proof that the camelCase key the docs now name is actually settable — the whole point of Issue 2.

---

## Confidence Score: 10/10

This is a two-token documentation fix with the exact before/after text, the authoritative source of
truth (`git.go:158` reads `stagecoach.autoStageAll`; `git_test.go:304` pins camelCase-only), a
content-anchored locate strategy that absorbs the parallel T2.S1 line shift, an unambiguous two-layer
rule with an enumerated "do not touch" list, and an empirical shell reproduction that independently
proves correctness. No code, no tests, no migration. The only residual judgment call (whether to do
the optional 6-row table completion, Task 4) is explicitly fenced as out-of-scope-and-skippable, so it
cannot lower the one-pass success of the mandatory deliverable.
