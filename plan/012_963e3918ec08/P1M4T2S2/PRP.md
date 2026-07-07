---
name: "P1.M4.T2.S2 — Rename FUTURE_SPEC.md references: stagehand → stagecoach (project rename Layer 5.4)"
description: |

  Rename every `stagehand`/`Stagehand` reference in the repo-root `FUTURE_SPEC.md` companion document to
  `stagecoach`/`Stagecoach`. This is Layer 5.4 of the stagehand→stagecoach project rename
  (architecture/rename_surface_map.md §5.4). The Go source (M1–M2), build system (M3), and README
  (M4.T1.S1) are already renamed; docs/*.md (M4.T1.S2) and providers/*.toml (M4.T2.S1) are being renamed
  in parallel. This task closes the FUTURE_SPEC.md surface.

  CONTRACT (item_description §3–4, verbatim): run `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g'
  FUTURE_SPEC.md`. OUTPUT: FUTURE_SPEC.md uses 'stagecoach' throughout. DOCS: Mode A — this IS the docs update.

  SCOPE — what this IS: a TEXTUAL rename in ONE existing prose file. Verified pre-rename (research F1): the
  file (7558 bytes, matches "~7.5KB") contains exactly 10 occurrences — 8 lowercase `stagehand` + 2
  capitalized `Stagehand` + 0 `STAGEHAND` — across 10 lines (1, 24, 27, 28, 34, 41, 77, 93, 94, 100). Every
  match is the STANDALONE word (no compound tokens — `grep -oE '[A-Za-z]*[Ss]tagehand[A-Za-z]*'` returns
  only `stagehand`/`Stagehand`), so the blanket sed cannot corrupt a larger identifier.

  FUTURE_SPEC.md is a COMPANION doc ("Companion to PRD.md") — NOT loaded/parsed/referenced by any Go code,
  test, Makefile, .goreleaser.yaml, CI workflow, or .gitignore (research F2). A textual rename has ZERO
  runtime/test impact: `go build ./... && go test ./...` is a regression-safety check only (expected green,
  unchanged). Validation is textual (grep + git diff), exactly like the providers/*.toml rename (S1).

  The rename is CORRECTNESS-RELEVANT, not cosmetic (research F3): 4 of the 10 refs are CLI command/invocation
  examples (`stagehand --dry-run`, `stagehand --dry-run --no-color | wl-copy`, "shell out to the installed
  stagehand binary", "bind stagehand to"). The renamed binary is now `stagecoach` (cmd/stagecoach, renamed
  P1.M1.T1.S2 Complete). Post-rename these examples are ACCURATE; left as `stagehand` they would document a
  command the renamed binary no longer answers to.

  The sed's two arms are CASE-DISJOINT and order-safe (research F4): `s/stagehand/.../g` matches only
  lowercase (initial `s`), leaving `Stagehand` for the second arm; neither re-creates the other's pattern.
  Both arms are REQUIRED. Verified by dry-run: `sed '...' FUTURE_SPEC.md | grep -c -i stagehand` → 0.

  DELIVERABLE (1 file MODIFIED; nothing else): `FUTURE_SPEC.md` — one blanket case-variant sed, then a
  zero-residue grep + a diff check.

  SCOPE BOUNDARY (owned by siblings — do NOT touch): `docs/*.md` (P1.M4.T1.S2, Layer 5.2 — parallel,
  disjoint); `providers/*.toml` (P1.M4.T2.S1, Layer 5.3 — parallel, disjoint); `plan/` artifacts
  (P1.M5.T1); `internal/provider/builtin.go` + all Go source (renamed M1–M2); PRD.md / tasks.json /
  prd_snapshot.md (orchestrator-owned, READ-ONLY). COMPETITOR-ANALYSIS.md is referenced in FUTURE_SPEC.md
  prose but is NOT present in the repo and is OUT OF SCOPE (not this surface). The final whole-repo
  zero-residue audit is P1.M5.T2.S1, NOT this task.

  Deliverable: 1 modified `FUTURE_SPEC.md`. `grep -in 'stagehand\|Stagehand\|STAGEHAND' FUTURE_SPEC.md`
  returns empty (exit 1); `git diff FUTURE_SPEC.md` shows exactly 10 lines changed (stagehand→stagecoach /
  Stagehand→Stagecoach); `go build ./... && go test ./...` unaffected (pure docs, not runtime-loaded).

---

## Goal

**Feature Goal**: Complete the stagehand→stagecoach rename on the FUTURE_SPEC.md surface (Layer 5.4) — every
`stagehand`/`Stagehand` reference in the repo-root companion document becomes `stagecoach`/`Stagecoach`,
with the file's prose, command examples, and table entries all consistent with the renamed binary.

**Deliverable** (1 file MODIFIED; nothing else): `FUTURE_SPEC.md` — one blanket case-variant sed, then a
zero-residue grep + a diff check.

**Success Definition**: `grep -in 'stagehand\|Stagehand\|STAGEHAND' FUTURE_SPEC.md` returns nothing (exit 1);
`git diff FUTURE_SPEC.md` shows exactly the 10 expected lines changed (the old-name → new-name swap, no
other prose altered); the 4 command-example references now read `stagecoach --dry-run` etc. (matching the
renamed binary, `cmd/stagecoach`); `go build ./... && go test ./...` green (pure docs — regression check).

## User Persona

**Target User**: A developer (or the primary author) reading `FUTURE_SPEC.md` to recall why a feature was
deferred or rejected, or to promote an idea into the PRD. They should see a single, consistent `stagecoach`
identity — not a mix of `stagehand` (stale) and `stagecoach` (current).

**Use Case**: A reader opens FUTURE_SPEC.md §1.1 (editor integration) and copies the VS Code extension's
invocation example (`stagecoach --dry-run`) into a config — the example must match the renamed binary.

**User Journey**: reader opens FUTURE_SPEC.md → title + prose + command examples all say `stagecoach` → the
`--clipboard` row's `stagecoach --dry-run --no-color | wl-copy` example runs against the actual binary → no
confusion about which name is current.

**Pain Points Addressed**: (1) stale `stagehand` references in the deferred/rejected-ideas doc that
contradict the renamed binary; (2) command examples naming a CLI (`stagehand`) the renamed binary no longer
answers to — fixed to `stagecoach`.

## Why

- **Closes the FUTURE_SPEC.md rename surface (Layer 5.4).** With Go source (M1–M2), build (M3), README
  (M4.T1.S1) already renamed, and docs/*.md (M4.T1.S2) + providers/*.toml (M4.T2.S1) renaming in parallel,
  FUTURE_SPEC.md is the remaining spec-doc surface still saying `stagehand`.
- **Makes the command examples TRUE.** 4 of the 10 references are CLI invocations (`stagehand --dry-run`,
  `stagehand --dry-run --no-color | wl-copy`, "the installed stagehand binary", "bind stagehand to"). The
  renamed binary is `stagecoach` (cmd/stagecoach); the rename restores accuracy.
- **Zero risk to runtime.** FUTURE_SPEC.md is a companion doc, not loaded by any code/test/build. A textual
  rename cannot affect behavior or tests.
- **One mechanical pass.** All 10 matches are the standalone old name (no compound tokens); the blanket sed
  handles them in one shot with a trivial verification.

## What

Run the contract's exact sed over `FUTURE_SPEC.md`, then verify (a) no `stagehand`/`Stagehand`/`STAGEHAND`
remains in the file, and (b) the `git diff` touches exactly the 10 expected lines (the old→new name swap,
no other prose altered). No edits to `docs/`, `providers/`, any Go source, or any other file.

### Success Criteria

- [ ] `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' FUTURE_SPEC.md` run (Linux/CI; macOS BSD
      sed: `sed -i '' '...' FUTURE_SPEC.md`). OR equivalent exact-text edits via the edit tool (platform-
      independent — see Implementation Tasks Task 2).
- [ ] `grep -in 'stagehand\|Stagehand\|STAGEHAND' FUTURE_SPEC.md` returns empty (exit 1) — zero residue.
- [ ] `git diff FUTURE_SPEC.md` shows exactly 10 changed lines (1, 24, 27, 28, 34, 41, 77, 93, 94, 100); the
      ONLY token changed in each is `stagehand`→`stagecoach` or `Stagehand`→`Stagecoach`. No other prose altered.
- [ ] The 4 command-example references now read `stagecoach` (e.g. line 24 `invoking \`stagecoach --dry-run\``;
      line 100 `stagecoach --dry-run --no-color | wl-copy`) — matching the renamed binary `cmd/stagecoach`.
- [ ] `go build ./... && go test ./...` green (pure docs, not runtime-loaded; regression-safety check only).
- [ ] ONLY `FUTURE_SPEC.md` differs (`git status`); `docs/`, `providers/`, all Go source, go.mod/go.sum,
      Makefile, .goreleaser.yaml UNCHANGED.

## All Needed Context

### Context Completeness Check

_Pass._ A developer with no prior repo knowledge can implement this from: the exact sed command (verbatim
above), the single target file (repo-root `FUTURE_SPEC.md`), the verified fact that all 10 matches are the
standalone old name with no compound tokens (research F1 + the pre-check command), the two verification
gates (zero-residue grep + 10-line diff), and the scope boundary (do not touch docs/providers/Go source).
No Go/provider/git knowledge required — it is a textual rename with a safety check.

### Documentation & References

```yaml
# MUST READ — the verified findings (the 10 refs, no-compound-token proof, sed dry-run, scope boundary)
- docfile: plan/012_963e3918ec08/P1M4T2S2/research/design-decisions.md
  why: F1 (exactly 10 occurrences: 8 stagehand + 2 Stagehand + 0 STAGEHAND, on lines 1/24/27/28/34/41/77/
       93/94/100; ALL standalone — no compound tokens, so the blanket sed is safe), F2 (pure-prose companion
       doc, NOT loaded by any Go/test/build — zero runtime/test impact; validation is grep+diff), F3 (the
       rename is accuracy-relevant: 4 refs are CLI command examples; cmd/stagecoach is the renamed binary),
       F4 (the two sed arms are case-disjoint + order-safe; both REQUIRED; dry-run yields 0 residue), F5
       (Layer 5.4 = FUTURE_SPEC.md ONLY; disjoint from docs/ 5.2 and providers/ 5.3; final whole-repo audit
       is P1.M5.T2.S1), F6 (GNU vs BSD sed -i; the edit tool as the platform-independent alternative + the
       per-line edit table).
  critical: F1 (the no-compound-token proof — the safety guarantee for the blanket sed), F4 (both sed arms
       required — dropping the Stagehand arm leaves lines 1 + 77 half-done).

# MUST READ — the rename surface map (this task IS Layer 5.4)
- docfile: plan/012_963e3918ec08/architecture/rename_surface_map.md
  section: "### 5.4 FUTURE_SPEC.md — Any references to stagehand." + the "Execution Order" + "Verification
       Gates" blocks (gate 5: whole-repo `grep -ri stagehand ... | wc -l == 0`).
  why: confirms this task's scope is exactly FUTURE_SPEC.md, and that Layer 5.2 (docs/) and 5.3
       (providers/*.toml) are SEPARATE parallel tasks — disjoint file sets, no conflict. Also confirms the
       FINAL whole-repo zero-residue audit is a later verification gate (P1.M5.T2.S1), not this task.
  critical: Layer 5.4 = FUTURE_SPEC.md ONLY. Do NOT cross into docs/ (5.2) or providers/ (5.3).

# MUST READ — the target file itself (the exact text the sed transforms)
- file: FUTURE_SPEC.md   (EDIT — repo root, 7558 bytes)
  section: line 1 `# Stagehand — Future Spec (deferred & rejected ideas)`; lines 24/27/28/34/41 (§1.1 editor
       integration — prose + `stagehand --dry-run` command refs); line 77 (§2.1 `Stagehand's entire
       thesis`); lines 93/94 (§3 rejected-features table prose); line 100 (`--clipboard` row's
       `stagehand --dry-run --no-color | wl-copy` example).
  why: shows the exact text the sed transforms. 10 lines, 10 standalone old-name occurrences. The §1.1
       command examples and the §3 `--clipboard` example are the accuracy-critical refs (they name the CLI).
  pattern: every match is the standalone word `stagehand` or `Stagehand` (verified — no compound tokens).
  gotcha: line 1 + line 77 are CAPITALIZED (`Stagehand`) — the sed's second arm (`s/Stagehand/Stagecoach/g`)
       handles them; the first arm (lowercase) does NOT (sed is case-sensitive). Both arms are required.

# READ — confirm the renamed binary (so the command-example update is accurate, not just cosmetic)
- file: cmd/   (READ ONLY — confirms cmd/stagecoach is the renamed binary)
  section: `cmd/stagecoach` (renamed from cmd/stagehand in P1.M1.T1.S2, Complete). `cmd/stubagent` is an
       unrelated test stub.
  why: the 4 command-example refs in FUTURE_SPEC.md (`stagehand --dry-run`, etc.) must become `stagecoach`
       to match the actual binary the user invokes. Verified — cmd/stagecoach exists.
  gotcha: do NOT edit cmd/ — it is already renamed (M1.T1.S2). This task only updates the COMMAND EXAMPLES
       in FUTURE_SPEC.md's prose to match.

# READ — the PRD rename note (the governing directive)
- docfile: PRD.md (heading h2.30 — in context as selected_prd_content)
  section: "## Note: this project was originally named 'stagehand' and has been renamed. All references to
       'stagehand' must be replaced with 'stagecoach'."
  why: the authoritative project-wide rename directive this task executes on its surface (Layer 5.4).

# READ — the sibling PRP (Layer 5.3, parallel — establishes the identical pattern this task follows)
- docfile: plan/012_963e3918ec08/P1M4T2S1/PRP.md
  section: the "What" + "Validation Loop" — the providers/*.toml rename uses the same contract sed
       (`sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' providers/*.toml`), the same zero-
       residue grep gate, and the same "pure docs ⇒ go test is a regression check only" rationale.
  why: confirms the rename pattern + verification gates this task mirrors (single file instead of 8). The
       S1 PRP explicitly lists FUTURE_SPEC.md as OUT OF ITS SCOPE ("Layer 5.4 — not this task") — no overlap.
  critical: S1 is the parallel sibling; it does NOT touch FUTURE_SPEC.md. This task (S2) does NOT touch
       providers/*.toml. Disjoint file sets.
```

### Current Codebase tree (relevant slice)

```bash
FUTURE_SPEC.md                  # 7558 bytes, 10 stagehand/Stagehand refs (lines 1,24,27,28,34,41,77,93,94,100). EDIT.
cmd/stagecoach/                 # the renamed binary (P1.M1.T1.S2 Complete) — what the command examples must name. READ ONLY.
docs/*.md                       # P1.M4.T1.S2 (Layer 5.2, parallel). UNCHANGED by THIS task.
providers/*.toml                # P1.M4.T2.S1 (Layer 5.3, parallel). UNCHANGED by THIS task.
internal/provider/builtin.go    # compiled-in manifests (Go source, renamed M1.T2). UNCHANGED.
go.mod / go.sum                 # UNCHANGED.
```

### Desired Codebase tree with files to be added/changed

```bash
FUTURE_SPEC.md                  # MODIFIED — 10 textual refs: stagehand→stagecoach / Stagehand→Stagecoach.
# NO other files changed. docs/, providers/, all Go source, go.mod/go.sum, Makefile, .goreleaser.yaml UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (no compound tokens — research F1): all 10 matches are the STANDALONE word stagehand/Stagehand.
#   Verify BEFORE trusting the blanket sed: `grep -oE '[A-Za-z]*[Ss]tagehand[A-Za-z]*' FUTURE_SPEC.md |
#   sort -u` → must print only `stagehand` and `Stagehand` (no suffixed/prefixed identifiers). If it prints
#   anything else (e.g. a hypothetical `stagehandignore`), STOP — hand-edit those occurrences (the blanket
#   sed would corrupt the larger token). Verified: only the two standalone words appear.

# CRITICAL (both sed arms REQUIRED — research F4): `s/stagehand/stagecoach/g` matches ONLY lowercase
#   (initial `s`); it does NOT touch `Stagehand` (capital S). The second arm `s/Stagehand/Stagecoach/g`
#   handles lines 1 + 77. Dropping it leaves those two capitalized refs stale. The arms are case-disjoint
#   and order-safe (neither re-creates the other's pattern).

# GOTCHA (2 CAPITALIZED refs on lines 1 + 77 — research F1): line 1 is the H1 title
#   `# Stagehand — Future Spec`; line 77 is `Stagehand's entire thesis` (possessive — the `'s` stays after
#   the swap → `Stagecoach's`). Both become `Stagecoach` via the second sed arm.

# GOTCHA (the rename is accuracy-relevant — research F3): 4 refs are CLI command examples (lines 24, 28,
#   34, 100) that name the binary. The renamed binary is `stagecoach` (cmd/stagecoach). Post-sed they
#   correctly say `stagecoach --dry-run` etc. Do NOT "preserve stagehand for backwards compat" — the
#   renamed binary does not answer to `stagehand`.

# GOTCHA (FUTURE_SPEC.md is NOT runtime-loaded — research F2): it is a companion doc, not parsed by any
#   Go code/test/build. A textual rename has ZERO runtime/test effect — `go test ./...` is unaffected. Do
#   NOT expect a test to validate this; validation is textual (grep + diff).

# GOTCHA (macOS sed vs GNU sed — research F6): `sed -i 's/.../.../g' file` (no extension arg) works on GNU
#   sed (Linux CI). On macOS BSD sed, `sed -i '' 's/.../.../g' file` (empty backup ext) is required. The
#   CI is Linux; if running locally on macOS, use `sed -i ''`. Alternatively, avoid the platform issue
#   entirely: the edit tool with exact-text replacements per line is deterministic and platform-independent
#   (preferred for a 0.5-point single-file task — see Implementation Tasks Task 2 + the per-line table in
#   research/design-decisions.md F6).

# GOTCHA (do NOT touch other surfaces): docs/*.md (Layer 5.2, P1.M4.T1.S2) and providers/*.toml (Layer 5.3,
#   P1.M4.T2.S1) are PARALLEL sibling tasks — disjoint files. plan/ is P1.M5.T1 (later). PRD.md/tasks.json/
#   prd_snapshot.md are orchestrator-owned READ-ONLY. This task is FUTURE_SPEC.md ONLY.

# GOTCHA (COMPETITOR-ANALYSIS.md is OUT OF SCOPE): FUTURE_SPEC.md references COMPETITOR-ANALYSIS.md in its
#   prose (the disposition preamble + §2.1). That file is NOT present in the repo and is NOT this task's
#   surface. Do not create or edit it.
```

## Implementation Blueprint

### Data models and structure

_None._ This is a textual rename in one existing prose file. No data models, no code, no new types. The
only "structure" is the 10 old-name → new-name substring swaps.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY the no-compound-token invariant BEFORE the rename (the safety gate)
  - RUN the rigorous pre-check (research F1): extract every token containing stagehand and confirm they are
      all the standalone word. Command:
        grep -oE '[A-Za-z]*[Ss]tagehand[A-Za-z]*' FUTURE_SPEC.md | sort -u
      Expected output: exactly two lines — `stagehand` and `Stagehand`. If ANY other token prints (a
      hypothetical compound like `stagehandignore`), do NOT run the blanket sed — hand-edit only the
      standalone occurrences (the sed would corrupt the compound token).
  - ALSO capture the baseline counts: `grep -oc 'stagehand' FUTURE_SPEC.md` → 8; `grep -oc 'Stagehand'
      FUTURE_SPEC.md` → 2; `grep -oc 'STAGEHAND' FUTURE_SPEC.md` → 0 (10 total).
  - WHY: this is the proof that the blanket sed is safe. Skip it and you are trusting the rename blindly.

Task 2: RENAME — apply the blanket case-variant sed (OR per-line edit-tool replacements)
  - OPTION A (sed — the contract's literal command, Linux/CI):
        sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' FUTURE_SPEC.md
    (macOS BSD sed: `sed -i '' 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' FUTURE_SPEC.md`.)
  - OPTION B (edit tool — platform-independent, PREFERRED for determinism on a 0.5-point single-file task):
      use the edit tool with 10 exact-text replacements (one per line; each oldText is unique in the file —
      the per-line table is in research/design-decisions.md F6). Concretely:
        - line 1:  "# Stagehand — Future Spec (deferred & rejected ideas)" → "# Stagecoach — Future Spec (deferred & rejected ideas)"
        - line 24: "invoking `stagehand --dry-run`" → "invoking `stagecoach --dry-run`"
        - line 27: "wrapping `stagehand` /" → "wrapping `stagecoach` /"
        - line 28: "`stagehand --dry-run`. The primary author" → "`stagecoach --dry-run`. The primary author"
        - line 34: "shell out to the installed `stagehand` binary" → "shell out to the installed `stagecoach` binary"
        - line 41: "custom-command facility to bind `stagehand` to." → "custom-command facility to bind `stagecoach` to."
        - line 77: "Stagehand's entire thesis" → "Stagecoach's entire thesis"
        - line 93: "stagehand never owns the model call." → "stagecoach never owns the model call."
        - line 94: "stagehand writes commit messages, nothing else." → "stagecoach writes commit messages, nothing else."
        - line 100: "`stagehand --dry-run --no-color | wl-copy`" → "`stagecoach --dry-run --no-color | wl-copy`"
  - GOTCHA (Option A): the `g` flag is required (matches the sibling rename PRPs; harmless here — no line
      has >1 occurrence, but `g` is the documented contract). KEEP both arms (lowercase + capitalized).
  - GOTCHA (Option B): each oldText must be unique in the file — the substrings above are unique (verified
      by line numbers). For line 27 (which wraps across a newline), include enough surrounding context
      (`wrapping \`stagehand\` /`) to be unique.
  - RUN (either option): `grep -in 'stagehand\|Stagehand\|STAGEHAND' FUTURE_SPEC.md` → expect NO output (exit 1).

Task 3: VERIFY — zero residue + 10-line diff + build/test unaffected
  - ZERO RESIDUE: `grep -in 'stagehand\|Stagehand\|STAGEHAND' FUTURE_SPEC.md` returns nothing (exit 1).
      (Case-insensitive `-i` catches any straggler regardless of case. If it prints anything, a ref was
      missed — re-run the sed / fix the edit.)
  - 10-LINE DIFF: `git diff FUTURE_SPEC.md` — exactly 10 changed lines (the `-`/`+` pairs for lines 1, 24,
      27, 28, 34, 41, 77, 93, 94, 100). The ONLY token changed in each hunk is `stagehand`→`stagecoach` or
      `Stagehand`→`Stagecoach`; no other prose is altered. Eyeball-confirm.
      Count check: `git diff FUTURE_SPEC.md | grep -cE '^-[^-]'` → 10 removed lines; `git diff FUTURE_SPEC.md
      | grep -cE '^\+[^+]'` → 10 added lines.
  - ACCURACY: confirm the command examples now read `stagecoach`:
        grep -n 'stagecoach --dry-run' FUTURE_SPEC.md   → lines 24, 28, 100.
        grep -n 'installed .stagecoach. binary' FUTURE_SPEC.md   → line 34.
  - BUILD/TEST UNAFFECTED: `go build ./... && go test ./...` → green (pure docs, not runtime-loaded; no
      behavior change). Regression-safety check, not a feature test.
  - SCOPE: `git status` → ONLY FUTURE_SPEC.md modified. `git diff --exit-code docs providers
      internal/provider/builtin.go go.mod go.sum Makefile .goreleaser.yaml` → empty (frozen files UNCHANGED).
```

### Implementation Patterns & Key Details

```bash
# PATTERN: the blanket case-variant sed (the contract's exact command). One pass, one file.
sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' FUTURE_SPEC.md
# (macOS: add the empty backup-ext arg: sed -i '' '...')

# PATTERN: verify no compound tokens BEFORE trusting the sed (the safety gate, research F1).
grep -oE '[A-Za-z]*[Ss]tagehand[A-Za-z]*' FUTURE_SPEC.md | sort -u
# Expected: exactly `stagehand` and `Stagehand` (standalone). Anything else ⇒ hand-edit, do NOT blanket-sed.

# PATTERN: verify zero residue AFTER the sed (case-insensitive — catches any straggler).
grep -in 'stagehand\|Stagehand\|STAGEHAND' FUTURE_SPEC.md   # Expected: no output (exit 1).

# GOTCHA: both sed arms are required (2 CAPITALIZED refs on lines 1 + 77). Case-disjoint, order-safe.
# GOTCHA: the rename is accuracy-relevant — 4 command-example refs (lines 24/28/34/100) must name `stagecoach`.
# GOTCHA: do NOT touch docs/providers/Go source — this task is FUTURE_SPEC.md ONLY (Layer 5.4).
# GOTCHA: FUTURE_SPEC.md is a companion doc, NOT runtime-loaded — `go test ./...` is a regression check only.
```

### Integration Points

```yaml
FUTURE_SPEC.md (companion doc — repo root):
  - change: "1 file — 10 textual refs: stagehand→stagecoach (8) + Stagehand→Stagecoach (2). Lines 1, 24,
    27, 28, 34, 41, 77, 93, 94, 100."
  - runtime: "NONE — not loaded by any Go code/test/Makefile/goreleaser/CI. A companion prose doc."

COMMAND.EXAMPLES (accuracy — cmd/stagecoach, P1.M1.T1.S2):
  - match: "the 4 CLI invocation refs (lines 24, 28, 34, 100) now read `stagecoach --dry-run` etc.,
    matching the renamed binary (cmd/stagecoach). Post-rename the examples are ACCURATE (they were stale
    before — naming `stagehand`, which the renamed binary does not answer to)."

SIBLING.TASKS (no conflict — disjoint file sets):
  - docs/*.md: "P1.M4.T1.S2 (Layer 5.2, parallel) — disjoint; no merge conflict."
  - providers/*.toml: "P1.M4.T2.S1 (Layer 5.3, parallel) — disjoint; no merge conflict."
  - plan/: "P1.M5.T1 (later) — not this task."
  - final whole-repo audit: "P1.M5.T2.S1 — not this task (this task closes only FUTURE_SPEC.md)."

GO.MODULE / BUILD / TEST: change NONE. The rename is textual in a non-runtime companion doc.
`go build ./... && go test ./...` is a regression-safety check (expected green, unchanged).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Confirm zero residue (case-insensitive — catches stagehand/Stagehand/STAGEHAND of any case):
grep -in 'stagehand\|Stagehand\|STAGEHAND' FUTURE_SPEC.md   # expect: NO matches (exit 1)
# Confirm go.mod/go.sum unchanged:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: zero stagehand residue; go.mod/go.sum byte-unchanged.
```

### Level 2: The 10-line diff gate (the core verification)

```bash
# The diff must touch EXACTLY the 10 expected lines, changing only the name token:
git diff FUTURE_SPEC.md
# Expected: 10 `-`/`+` pairs (lines 1, 24, 27, 28, 34, 41, 77, 93, 94, 100). The only token changed in each
# hunk is `stagehand`→`stagecoach` or `Stagehand`→`Stagecoach`. Sample expected hunks:
#   -# Stagehand — Future Spec (deferred & rejected ideas)
#   +# Stagecoach — Future Spec (deferred & rejected ideas)
#   -- **VS Code extension.** A ✨ button ... invoking `stagehand --dry-run`
#   +- **VS Code extension.** A ✨ button ... invoking `stagecoach --dry-run`
#   -| **`--clipboard` mode** (aicommits) | `stagehand --dry-run --no-color \| wl-copy` ...
#   +| **`--clipboard` mode** (aicommits) | `stagecoach --dry-run --no-color \| wl-copy` ...

# Count check (10 removed, 10 added):
git diff FUTURE_SPEC.md | grep -cE '^-[^-]'   # expect: 10
git diff FUTURE_SPEC.md | grep -cE '^\+[^+]'  # expect: 10
# If the counts differ from 10, a ref was missed OR unrelated prose was altered — investigate.

# Confirm FUTURE_SPEC.md is the only changed file:
git status --porcelain
# Expected: exactly ` M FUTURE_SPEC.md`. Nothing else.
```

### Level 3: Build/test regression check (the file is a companion doc, not runtime-loaded)

```bash
go build ./...     # Expect clean (no Go source touched).
go test ./...      # Expect all PASS — FUTURE_SPEC.md is NOT parsed at runtime; no test reads it.
# This is a regression-safety check (the rename should be a no-op for the suite), not a feature test.
```

### Level 4: Scope + accuracy (the rename is complete + correct on this surface)

```bash
# SCOPE: only FUTURE_SPEC.md changed; frozen files byte-unchanged.
git diff --exit-code docs providers internal/provider/builtin.go go.mod go.sum Makefile \
  .goreleaser.yaml .github README.md PRD.md && echo "frozen files UNCHANGED (expected)"

# ACCURACY: the 4 command-example refs now name the renamed binary.
grep -n 'stagecoach --dry-run' FUTURE_SPEC.md          # expect: lines 24, 28, 100
grep -n 'installed .stagecoach. binary' FUTURE_SPEC.md # expect: line 34
grep -n "bind .stagecoach. to" FUTURE_SPEC.md          # expect: line 41

# WHOLE-REPO RESIDUE (informational — the FINAL zero-residue audit is P1.M5.T2.S1, not this task):
# This task closes the FUTURE_SPEC.md surface; other surfaces (docs/, providers/, plan/) are siblings'.
grep -rln 'stagehand\|Stagehand\|STAGEHAND' FUTURE_SPEC.md   # expect: empty (this task's surface is clean)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go test ./...` green (regression-safety; the file is not runtime-loaded).
- [ ] `git diff --exit-code go.mod go.sum` empty.
- [ ] `git status` shows EXACTLY `FUTURE_SPEC.md` modified; every frozen file byte-unchanged (`docs/`,
      `providers/`, `internal/provider/builtin.go`, all Go source, Makefile, .goreleaser.yaml).

### Feature Validation
- [ ] `grep -in 'stagehand\|Stagehand\|STAGEHAND' FUTURE_SPEC.md` returns nothing (zero residue).
- [ ] `git diff FUTURE_SPEC.md` shows exactly 10 changed lines (1, 24, 27, 28, 34, 41, 77, 93, 94, 100);
      only the name token changed in each.
- [ ] The 4 command-example refs now read `stagecoach` (lines 24, 28, 34, 100) — accurate post-rename.

### Code Quality Validation
- [ ] The blanket sed was verified compound-token-free BEFORE application (Task 1 pre-check).
- [ ] No prose other than the 10 name tokens was altered (the diff is name-only).
- [ ] The rename is consistent with the project-wide directive (PRD h2.30) and the sibling surfaces
      (Go source M1–M2, build M3, README M4.T1.S1, docs M4.T1.S2, providers M4.T2.S1).
- [ ] Anti-patterns avoided (see below); no out-of-scope churn.

### Documentation
- [ ] FUTURE_SPEC.md now presents a single, consistent `stagecoach` identity (title, prose, command
      examples, table entries).
- [ ] No new env vars / config keys / CLI surface (DOCS clause: "Mode A — this IS the docs update").

---

## Anti-Patterns to Avoid

- ❌ **Don't blanket-sed without the no-compound-token pre-check.** Run Task 1 first (`grep -oE
  '[A-Za-z]*[Ss]tagehand[A-Za-z]*' | sort -u`). If a compound token existed, the blanket sed would corrupt
  it. (Verified: only standalone `stagehand`/`Stagehand` — but prove it, don't assume it.)
- ❌ **Don't drop the `Stagehand` arm of the sed.** Two refs are CAPITALIZED (lines 1 + 77). The lowercase
  arm does not touch them (sed is case-sensitive); without the second arm they stay stale. Both arms required.
- ❌ **Don't alter prose "while you're in there."** The diff must be name-only — 10 lines, each changing only
  `stagehand`→`stagecoach` / `Stagehand`→`Stagecoach`. No rewording, no reflow, no "improvements."
- ❌ **Don't touch other rename surfaces.** `docs/*.md` is P1.M4.T1.S2 (Layer 5.2, parallel);
  `providers/*.toml` is P1.M4.T2.S1 (Layer 5.3, parallel); `plan/` is P1.M5.T1 (later). This task is
  `FUTURE_SPEC.md` ONLY (Layer 5.4).
- ❌ **Don't "preserve stagehand for backwards compat."** The renamed binary does NOT answer to `stagehand`
  (it is `stagecoach`, cmd/stagecoach). The command examples must say `stagecoach` to be accurate.
- ❌ **Don't expect a test to validate this.** FUTURE_SPEC.md is a companion doc, not runtime-loaded; no test
  reads it. Validation is textual (grep + diff). `go test ./...` is a regression check only.
- ❌ **Don't edit COMPETITOR-ANALYSIS.md.** It is referenced in FUTURE_SPEC.md prose but is NOT present in
  the repo and is NOT this task's surface. Do not create or edit it.
- ❌ **Don't conflate this with the final whole-repo audit.** This task closes ONLY FUTURE_SPEC.md. The
  project-wide `grep -ri stagehand ... == 0` gate (rename_surface_map verification gate 5) is P1.M5.T2.S1.
