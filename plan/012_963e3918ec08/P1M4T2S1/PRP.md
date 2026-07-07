---
name: "P1.M4.T2.S1 — Rename providers/*.toml comment references: stagehand → stagecoach (project rename Layer 5.3)"
description: |

  Rename every `stagehand`/`Stagehand` reference in the eight provider reference-manifest TOML files
  (`providers/{pi,claude,gemini,agy,opencode,codex,cursor,qwen-code}.toml`) to `stagecoach`/`Stagecoach`.
  This is Layer 5.3 of the stagehand→stagecoach project rename (rename_surface_map.md §5.3). The Go source
  (M1–M2), build system (M3), and README (M4.T1.S1) are already renamed; docs/*.md is being renamed in
  parallel (M4.T1.S2). This task closes the provider-manifest documentation surface.

  CONTRACT (item_description §3–4, verbatim): run `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g'
  providers/*.toml`, then VERIFY no functional manifest values were changed (only comments). OUTPUT: all
  provider TOML comments reference 'stagecoach'; no functional manifest values changed.

  SCOPE — what this IS: a COMMENT-ONLY textual rename across 8 files. Verified pre-rename (research F1):
  all 18 `stagehand` matches are on `#`-comment lines (a rigorous "strip comments, grep" check returns
  empty for the non-comment remainder). No TOML key/value/flag/command/model/reasoning-level/env contains
  `stagehand`. The blanket sed therefore cannot change a functional value. These TOML files are NOT loaded
  at runtime (built-ins are compiled into `internal/provider/builtin.go`); they are human-readable REFERENCE
  manifests, so a comment-only rename has zero runtime effect and zero test impact.

  The rename is also CORRECTNESS-RELEVANT, not just cosmetic: the boilerplate comments reference the config
  loader's real paths (`.stagehand.toml`, `~/.config/stagehand/config.toml`), which were renamed to
  `.stagecoach.toml` / `~/.config/stagecoach/config.toml` in P1.M2.T2 (Complete). Leaving the comments as
  `stagehand` would document paths the renamed binary no longer reads.

  DELIVERABLE (8 files MODIFIED; nothing else): the eight `providers/*.toml` files — one global case-variant
  sed, then a comment-only diff verification + a zero-residue grep.

  SCOPE BOUNDARY (owned by siblings — do NOT touch): `internal/provider/builtin.go` (compiled-in manifests,
  Go source — renamed in M1.T2); `docs/*.md` (P1.M4.T1.S2, Layer 5.2 — parallel, disjoint file set);
  FUTURE_SPEC.md (P1.M4.T2.S2); plan/ artifacts (P1.M5.T1); PRD.md / tasks.json / prd_snapshot.md
  (orchestrator-owned, READ-ONLY). Do NOT change any functional TOML value.

  Deliverable: 8 modified `providers/*.toml` (comments only). `grep -rn 'stagehand\|Stagehand\|STAGEHAND'
  providers/*.toml` returns empty; `git diff providers/*.toml` shows only `#`-comment lines; `go test ./...`
  unaffected (these files are not runtime-loaded).

---

## Goal

**Feature Goal**: Complete the stagehand→stagecoach rename on the provider reference-manifest surface
(Layer 5.3) — every `stagehand`/`Stagehand` reference in the eight `providers/*.toml` files becomes
`stagecoach`/`Stagecoach`, with ZERO functional manifest values changed (comments only).

**Deliverable** (8 files MODIFIED; nothing else): `providers/{pi,claude,gemini,agy,opencode,codex,cursor,
qwen-code}.toml` — one blanket case-variant sed, then a comment-only diff verification + zero-residue grep.

**Success Definition**: `grep -rn 'stagehand\|Stagehand\|STAGEHAND' providers/*.toml` returns nothing (exit
1); `git diff providers/*.toml` shows ONLY `#`-comment lines changed (no functional key/value/flag/model/
`[reasoning_levels]`/`[env]` line altered); the build + test suite is unaffected (`go build ./... &&
go test ./...` green — these files are reference docs, not runtime-loaded).

## User Persona

**Target User**: A developer reading a provider manifest (e.g. `providers/pi.toml`) to understand how
stagecoach invokes `pi`, or copy Field lines into their config as an override. They should see a single,
consistent `stagecoach` identity — not a mix of `stagehand` (stale) and `stagecoach` (current).

**Use Case**: A user opens `providers/pi.toml` to find the `--provider` flag shape; the header's config-path
example (`~/.config/stagecoach/config.toml`) matches the renamed binary's actual config path.

**User Journey**: user inspects a provider manifest → comments reference `stagecoach` consistently → the
config-path example matches `stagecoach config path` output → no confusion about which name is current.

**Pain Points Addressed**: (1) stale `stagehand` references in provider docs that contradict the renamed
binary; (2) the config-path example pointing at `.stagehand.toml` (a path the renamed binary no longer
reads) — fixed to `.stagecoach.toml`.

## Why

- **Closes the provider-docs rename surface (Layer 5.3).** With Go source (M1–M2), build (M3), and README
  (M4.T1.S1) already renamed, these 8 reference manifests are the remaining provider-documentation surface
  still saying `stagehand`.
- **Makes the comments TRUE.** The boilerplate references real config paths (`.stagehand.toml`,
  `~/.config/stagehand/config.toml`) renamed in P1.M2.T2. The comment rename restores accuracy.
- **Zero risk to runtime.** These TOMLs are reference docs ("NOT loaded at runtime — built-ins are compiled
  into the Go binary"); a comment-only rename cannot affect behavior or tests.
- **One mechanical pass.** All 18 matches are lowercase `stagehand` in comments; the blanket sed handles
  them in one shot with a trivial verification.

## What

Run the contract's exact sed over the eight files, then verify (a) no `stagehand`/`Stagehand`/`STAGEHAND`
remains in `providers/*.toml`, and (b) the `git diff` touches ONLY `#`-comment lines (no functional value
changed). No edits to `internal/provider/builtin.go`, `docs/`, `FUTURE_SPEC.md`, or any Go source.

### Success Criteria

- [ ] `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' providers/*.toml` run over all 8 files.
- [ ] `grep -rn 'stagehand\|Stagehand\|STAGEHAND' providers/*.toml` returns empty (exit 1) — zero residue.
- [ ] `git diff providers/*.toml` shows ONLY `#`-comment lines changed (every changed line begins with `#`
      after the `+`/`-`); NO functional TOML line (keys, values, flags, models, `[reasoning_levels]`,
      `[env]`) is altered. (Pre-checked: all 18 matches are comment-only — research F1.)
- [ ] The boilerplate config-path comments now read `.stagecoach.toml` and `~/.config/stagecoach/config.toml`
      (matching the renamed binary's real paths, P1.M2.T2).
- [ ] `go build ./... && go test ./...` green (these files are not runtime-loaded; no behavior change).
- [ ] ONLY the 8 `providers/*.toml` files differ (`git status`); `internal/provider/builtin.go`, `docs/`,
      `FUTURE_SPEC.md`, all Go source, go.mod/go.sum UNCHANGED.

## All Needed Context

### Context Completeness Check

_Pass._ A developer with no prior repo knowledge can implement this from: the exact sed command (verbatim
above), the 8 target files (listed), the verified fact that all 18 matches are comment-only (research F1 +
the pre-check command), the two verification gates (zero-residue grep + comment-only diff), and the
scope boundary (do not touch builtin.go/docs/FUTURE_SPEC.md/Go source). No Go/TOML/provider knowledge
required — it is a textual rename with a safety check.

### Documentation & References

```yaml
# MUST READ — the verified findings (the 18 refs, comment-only proof, the breakdown)
- docfile: plan/012_963e3918ec08/P1M4T2S1/research/design-decisions.md
  why: F1 (all 18 refs are comments — the rigorous strip-comments-then-grep proof that the blanket sed is
       comment-only by construction), F2 (the breakdown: boilerplate lines 9+16 in every file + pi.toml's
       unique line 112), F3 (the rename is accuracy-relevant: the comments reference real renamed config
       paths), F4 (no conflict with the parallel docs/*.md task), F5 (the TOMLs are reference docs, not
       runtime-loaded — zero test impact).
  critical: F1 (the comment-only proof — this is the safety guarantee), F3 (why it matters beyond cosmetics).

# MUST READ — the rename surface map (this task IS Layer 5.3)
- docfile: plan/012_963e3918ec08/architecture/rename_surface_map.md
  section: "### 5.3 providers/*.toml (8 files) — Comments referencing stagehand in each manifest file".
  why: confirms this task's scope is exactly the 8 provider TOMLs' comments, and that Layer 5.2 (docs/) is
       a SEPARATE task (P1.M4.T1.S2) — disjoint file set, no conflict.
  critical: Layer 5.3 = providers/*.toml ONLY. Do NOT cross into docs/ (5.2) or FUTURE_SPEC.md (5.4).

# READ — one of the 8 target files (the boilerplate header shape + pi.toml's unique line)
- file: providers/pi.toml   (EDIT — representative; the boilerplate lines 9/16 are shared by all 8)
  section: line 9 `#   the Go binary. (The config loader reads .stagehand.toml, not this directory.)`;
       line 16 `#       # ~/.config/stagehand/config.toml  (or a repo-local .stagehand.toml)` (2 refs);
       line 112 `# off has no entry ⇒ graceful no-op (FR-R6); minimal/xhigh have no stagehand level.`.
  why: shows the exact text the sed transforms. Lines 9 + 16 are the shared header (identical in all 8
       files); line 112 is pi.toml-only. The other 7 files have only the lines-9/16 boilerplate.
  pattern: every match begins with `#` (a comment) — confirmed for all 18 matches across all 8 files.
  gotcha: line 16 has TWO `stagehand` occurrences (`stagehand/config.toml` AND `.stagehand.toml`); the
       blanket `g` flag handles both per-line. Do NOT hand-edit — the sed is the single source of truth.

# READ — confirm the renamed config paths (so the comment update is accurate, not just cosmetic)
- file: internal/config/file.go   (READ ONLY — confirms the renamed paths the comments should match)
  section: line 92-94 `$XDG_CONFIG_HOME/stagecoach/config.toml` / `~/.config/stagecoach/config.toml`;
       line 125 `./.stagecoach.toml`.
  why: the boilerplate comments reference these EXACT paths; post-rename they must say `stagecoach` to
       match the renamed binary (P1.M2.T2). Verified — the Go code already uses stagecoach.
  gotcha: do NOT edit file.go — it is already renamed (M1.T2/M2.T2). This task only updates the COMMENTS
       in providers/*.toml to match.

# READ — the PRD rename note (the governing directive)
- docfile: PRD.md (heading h2.30 — in context as selected_prd_content)
  section: "## Note: this project was originally named 'stagehand' and has been renamed. All references to
       'stagehand' must be replaced with 'stagecoach'."
  why: the authoritative project-wide rename directive this task executes on its surface (Layer 5.3).
```

### Current Codebase tree (relevant slice)

```bash
providers/
  pi.toml          # 3 refs: lines 9, 16(×2... actually 9, 16, 112), 112 (unique). EDIT.
  claude.toml      # 2 refs: lines 9, 16 (boilerplate). EDIT.
  codex.toml       # 2 refs: lines 9, 16 (boilerplate). EDIT.
  cursor.toml      # 2 refs: lines 9, 16 (boilerplate). EDIT.
  gemini.toml      # 2 refs: lines 9, 16 (boilerplate). EDIT.
  opencode.toml    # 2 refs: lines 9, 16 (boilerplate). EDIT.
  agy.toml         # 3 refs: lines 9, 16 (boilerplate) [+1]. EDIT.
  qwen-code.toml   # 2 refs: lines 9, 16 (boilerplate). EDIT.
internal/provider/builtin.go   # compiled-in manifests (Go source, renamed M1.T2). UNCHANGED.
docs/*.md                     # P1.M4.T1.S2 (Layer 5.2, parallel). UNCHANGED by THIS task.
FUTURE_SPEC.md                # P1.M4.T2.S2 (Layer 5.4). UNCHANGED by THIS task.
go.mod / go.sum               # UNCHANGED.
```

### Desired Codebase tree with files to be added/changed

```bash
providers/pi.toml              # MODIFIED — comments only (stagehand → stagecoach).
providers/claude.toml          # MODIFIED — comments only.
providers/codex.toml           # MODIFIED — comments only.
providers/cursor.toml          # MODIFIED — comments only.
providers/gemini.toml          # MODIFIED — comments only.
providers/opencode.toml        # MODIFIED — comments only.
providers/agy.toml             # MODIFIED — comments only.
providers/qwen-code.toml       # MODIFIED — comments only.
# NO other files changed. internal/provider/builtin.go, docs/, FUTURE_SPEC.md, all Go source UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (comment-only by construction — research F1): all 18 `stagehand` matches in providers/*.toml are
#   on `#`-comment lines. Verify BEFORE trusting the blanket sed: `for f in providers/*.toml; do grep -vE '^\s*#' "$f" |
#   grep -i stagehand && echo "FUNCTIONAL REF in $f"; done` → must print nothing. If it prints anything, STOP — a
#   functional value contains stagehand and the blanket sed would change behavior (hand-edit that line instead).

# GOTCHA (line 16 has TWO occurrences): `# ~/.config/stagehand/config.toml (or a repo-local .stagehand.toml)`
#   → both `stagehand` substrings become `stagecoach`. The `g` (global) flag in `s/stagehand/stagecoach/g`
#   handles multiple occurrences per line — do NOT run the sed without `g`.

# GOTCHA (no Stagehand/STAGEHAND matches — research F1): only lowercase `stagehand` appears. The
#   `s/Stagehand/Stagecoach/g` arm is a no-op but KEEP it (symmetry with the sibling rename PRPs; harmless).

# GOTCHA (these TOMLs are NOT runtime-loaded — F5): built-ins are compiled into internal/provider/builtin.go.
#   providers/*.toml are human-readable REFERENCE manifests. A comment rename has ZERO runtime/test effect —
#   `go test ./...` is unaffected. Do NOT expect a test to validate this; validation is textual (grep + diff).

# GOTCHA (the rename is accuracy-relevant — F3): the boilerplate references the config loader's REAL paths
#   (.stagehand.toml, ~/.config/stagehand/config.toml), renamed in P1.M2.T2. Post-sed they correctly say
#   .stagecoach.toml / ~/.config/stagecoach/config.toml — matching internal/config/file.go. Do NOT "preserve
#   the old paths for backwards compat" — the renamed binary does not read .stagehand.toml.

# GOTCHA (do NOT touch builtin.go): the compiled-in manifests live in internal/provider/builtin.go (Go source,
#   renamed in M1.T2). providers/*.toml MIRROR builtin.go byte-for-byte (modulo comments). If a comment in a
#   .toml references a field value, leave the VALUE alone — only the word "stagehand" in comments changes.

# GOTCHA (macOS sed vs GNU sed): `sed -i 's/.../.../g' file` (no extension arg) works on GNU sed (Linux CI).
#   On macOS BSD sed, `sed -i '' 's/.../.../g' file` (empty backup ext) is required. The Makefile/CI is Linux;
#   if running locally on macOS, use `sed -i ''`. Alternatively, avoid the platform issue entirely: the edit
#   tool with exact-text replacements per file is deterministic and platform-independent (preferred for a
#   1-point task — see Implementation Tasks Task 2).
```

## Implementation Blueprint

### Data models and structure

_None._ This is a textual rename of comment lines in 8 existing TOML files. No data models, no code, no
new types. The only "structure" is the per-file set of comment substrings `stagehand` → `stagecoach`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY the comment-only invariant BEFORE the rename (the safety gate)
  - RUN the rigorous pre-check (research F1): for each providers/*.toml, strip `#`-comment lines and grep
      the remainder for stagehand. Command:
        for f in providers/*.toml; do grep -vE '^\s*#' "$f" | grep -iE 'stagehand|Stagehand|STAGEHAND' \
          && echo "  ^^ FUNCTIONAL REF in $f — STOP, hand-edit"; done
      Expected: NO output (every match is a comment). If ANY line prints, a functional value contains
      stagehand — do NOT run the blanket sed; hand-edit only the comment occurrences in that file.
  - ALSO capture the baseline: `grep -rc 'stagehand\|Stagehand' providers/*.toml | grep -v ':0'` → expect
      agy=3, claude=2, codex=2, cursor=2, gemini=2, opencode=2, pi=3, qwen-code=2 (18 lines total).
  - WHY: this is the proof that the blanket sed in Task 2 is comment-only by construction. Skip it and you
      are trusting the rename blindly on a surface you haven't audited.

Task 2: RENAME — apply the blanket case-variant sed (OR per-file edit-tool replacements)
  - OPTION A (sed — the contract's literal command, Linux/CI):
        sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' providers/*.toml
    (macOS BSD sed: `sed -i '' 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' providers/*.toml`.)
  - OPTION B (edit tool — platform-independent, preferred for determinism on a 1-point task): for each of
      the 8 files, the comment occurrences are identical boilerplate (lines 9 + 16) except pi.toml (line 112).
      Use the edit tool's multi-edit per file:
        - Every file: replace "# The config loader reads .stagehand.toml, not this directory." → "...stagecoach..."
          AND "# ~/.config/stagehand/config.toml  (or a repo-local .stagehand.toml)" → "...stagecoach...stagecoach...".
        - pi.toml ONLY: additionally replace "minimal/xhigh have no stagehand level." → "...stagecoach level.".
  - GOTCHA: if using sed, the `g` flag is REQUIRED (line 16 has two `stagehand` per line). If using the edit
      tool, each oldText must be unique in its file (the boilerplate lines are unique per file — safe).
  - RUN: `grep -rn 'stagehand\|Stagehand\|STAGEHAND' providers/*.toml` → expect NO output (exit 1).

Task 3: VERIFY — zero residue + comment-only diff + build/test unaffected
  - ZERO RESIDUE: `grep -rn 'stagehand\|Stagehand\|STAGEHAND' providers/*.toml` returns nothing (exit 1).
      (If it prints anything, a ref was missed — re-run the sed / fix the edit.)
  - COMMENT-ONLY DIFF: `git diff providers/*.toml` — every changed line begins with `+ #` or `- #` (a
      comment). NO functional TOML line (a key=value, a flag in bare_flags/tooled_flags, a model, a
      [reasoning_levels] / [env] entry) appears in the diff. Eyeball-confirm: the only token changed in
      each hunk is `stagehand` → `stagecoach` inside a comment.
      Rigorous variant: `git diff providers/*.toml | grep -E '^[+-]' | grep -vE '^[+-]\s*#' | grep -v '^[+-]{3}'
      → expect EMPTY (no changed line is a non-comment). (The `^[+-]{3}` excludes the diff hunk headers.)
  - ACCURACY: confirm the boilerplate now reads `.stagecoach.toml` and `~/.config/stagecoach/config.toml`
      (matching internal/config/file.go lines 92-94, 125): `grep -n 'stagecoach' providers/pi.toml | head`.
  - BUILD/TEST UNAFFECTED: `go build ./... && go test ./...` → green (these files are reference docs, not
      runtime-loaded; no behavior change). This is a regression-safety check, not a feature test.
  - SCOPE: `git status` → ONLY the 8 providers/*.toml modified. `git diff --exit-code internal/provider/builtin.go
      docs FUTURE_SPEC.md go.mod go.sum` → empty (frozen files UNCHANGED).
```

### Implementation Patterns & Key Details

```bash
# PATTERN: the blanket case-variant sed (the contract's exact command). One pass, all 8 files.
sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' providers/*.toml
# (macOS: add the empty backup-ext arg: sed -i '' '...')

# PATTERN: verify comment-only BEFORE trusting the sed (the safety gate, research F1).
for f in providers/*.toml; do grep -vE '^\s*#' "$f" | grep -iE 'stagehand|Stagehand|STAGEHAND' \
  && echo "FUNCTIONAL REF in $f — hand-edit, do NOT blanket-sed"; done
# Expected: no output. Every match is a comment ⇒ the blanket sed is safe.

# PATTERN: verify comment-only AFTER the sed (the diff gate).
git diff providers/*.toml | grep -E '^[+-]' | grep -vE '^[+-]\s*#' | grep -v '^[+-]{3}'
# Expected: empty (no non-comment line changed).

# GOTCHA: the `g` flag is mandatory (line 16 has two `stagehand` per line).
# GOTCHA: keep the `Stagehand` arm even though there are no matches (symmetry with sibling PRPs; harmless).
# GOTCHA: do NOT touch builtin.go / docs / FUTURE_SPEC.md / Go source — this task is providers/*.toml ONLY.
# GOTCHA: these TOMLs are reference docs, NOT runtime-loaded — `go test ./...` is a regression check only.
```

### Integration Points

```yaml
PROVIDER.MANIFESTS (reference docs — providers/*.toml):
  - change: "8 files — comment-only stagehand → stagecoach (boilerplate lines 9/16 + pi.toml line 112)."
  - runtime: "NONE — built-ins are compiled into internal/provider/builtin.go (unchanged). The TOMLs are
    human-readable references mirroring builtin.go byte-for-byte (modulo comments)."

CONFIG.PATHS (comment accuracy — internal/config/file.go, P1.M2.T2):
  - match: "the boilerplate comments now reference .stagecoach.toml + ~/.config/stagecoach/config.toml,
    matching the renamed binary's real config paths (file.go lines 92-94, 125). Post-rename the comments
    are ACCURATE (they were stale before — pointing at .stagehand.toml, which the binary no longer reads)."

SIBLING.TASKS (no conflict):
  - docs/*.md: "P1.M4.T1.S2 (Layer 5.2, parallel) — disjoint file set; no merge conflict."
  - FUTURE_SPEC.md: "P1.M4.T2.S2 (Layer 5.4) — not this task."
  - internal/provider/builtin.go: "renamed in M1.T2 (Complete) — UNCHANGED here."

GO.MODULE / BUILD / TEST: change NONE. The rename is comment-only in non-runtime reference files.
`go build ./... && go test ./...` is a regression-safety check (expected green, unchanged).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Confirm the 8 TOML files are still valid TOML (a comment-only rename cannot break this, but verify).
for f in providers/*.toml; do grep -vE '^\s*#' "$f" | grep -E '^[a-z_]+\s*=' >/dev/null || true; done
# (There is no toml CLI in this repo; the runtime parser is Go's go-toml/v2. Validity is guaranteed by the
#  comment-only diff — no key/value line is touched. The Level-3 build/test is the real validity check.)

# Confirm zero residue:
grep -rn 'stagehand\|Stagehand\|STAGEHAND' providers/*.toml   # expect: NO matches (exit 1)
# Confirm go.mod/go.sum unchanged:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: zero stagehand residue; go.mod/go.sum byte-unchanged.
```

### Level 2: The comment-only diff gate (the core verification)

```bash
# The diff must touch ONLY # lines:
git diff providers/*.toml | grep -E '^[+-]' | grep -vE '^[+-]\s*#' | grep -v '^[+-]{3}'
# Expected: EMPTY (no non-comment line changed). If anything prints, a functional value was altered — revert
# and hand-edit the comment occurrences only.

# Eyeball the hunks (every changed token is stagehand → stagecoach inside a comment):
git diff providers/*.toml | head -60
# Expected: lines like:
#   -#   the Go binary. (The config loader reads .stagehand.toml, not this directory.)
#   +#   the Go binary. (The config loader reads .stagecoach.toml, not this directory.)
#   -#       # ~/.config/stagehand/config.toml  (or a repo-local .stagehand.toml)
#   +#       # ~/.config/stagecoach/config.toml  (or a repo-local .stagecoach.toml)
# (pi.toml also): -# ... minimal/xhigh have no stagehand level. ...  →  stagecoach level.

# Confirm the 8 files are the only changed ones:
git status --porcelain providers/
# Expected: 8 modified files (M providers/*.toml). Nothing else under providers/ (no new/deleted files).
```

### Level 3: Build/test regression check (the files are reference docs, not runtime-loaded)

```bash
go build ./...     # Expect clean (no Go source touched).
go test ./...      # Expect all PASS — providers/*.toml are NOT parsed at runtime (built-ins are compiled
                   # into internal/provider/builtin.go); the manifest tests exercise builtin.go, not the TOML.
# This is a regression-safety check (the rename should be a no-op for the suite), not a feature test.
```

### Level 4: Scope + accuracy (the rename is complete + correct on this surface)

```bash
# SCOPE: only the 8 providers/*.toml changed; frozen files byte-unchanged.
git diff --exit-code internal/provider/builtin.go docs FUTURE_SPEC.md go.mod go.sum Makefile \
  .goreleaser.yaml .github README.md PRD.md && echo "frozen files UNCHANGED (expected)"

# ACCURACY: the boilerplate config-path comments now match the renamed binary's real paths.
grep -n 'stagecoach' providers/pi.toml | head
# Expect lines containing: ".stagecoach.toml" and "~/.config/stagecoach/config.toml" — matching
# internal/config/file.go (lines 92-94: stagecoach/config.toml; line 125: ./.stagecoach.toml).

# WHOLE-REPO RESIDUE (informational — the FINAL zero-residue audit is P1.M5.T2.S1, not this task):
# This task closes the providers/ surface; other surfaces (docs/, FUTURE_SPEC.md, plan/) are siblings'.
grep -rln 'stagehand\|Stagehand\|STAGEHAND' providers/   # expect: empty (this task's surface is clean)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go test ./...` green (regression-safety; the TOMLs are not runtime-loaded).
- [ ] `git diff --exit-code go.mod go.sum` empty.
- [ ] `git status` shows EXACTLY the 8 `providers/*.toml` modified; every frozen file byte-unchanged
      (`internal/provider/builtin.go`, `docs/`, `FUTURE_SPEC.md`, all Go source, Makefile, .goreleaser.yaml).

### Feature Validation
- [ ] `grep -rn 'stagehand\|Stagehand\|STAGEHAND' providers/*.toml` returns nothing (zero residue).
- [ ] `git diff providers/*.toml` touches ONLY `#`-comment lines (the comment-only gate, Level 2).
- [ ] The boilerplate comments now read `.stagecoach.toml` and `~/.config/stagecoach/config.toml`
      (accurate post-rename — matching `internal/config/file.go`).
- [ ] pi.toml's unique line 112 reads `no stagecoach level`.

### Code Quality Validation
- [ ] The blanket sed was verified comment-only BEFORE application (Task 1 pre-check) — no functional value
      was at risk.
- [ ] No functional TOML value (name/command/detect/flags/models/`[reasoning_levels]`/`[env]`/print_flag/
      model_flag/provider_flag/etc.) altered.
- [ ] The rename is consistent with the project-wide directive (PRD h2.30) and the sibling surfaces
      (Go source M1–M2, build M3, README M4.T1.S1, docs M4.T1.S2).
- [ ] Anti-patterns avoided (see below); no out-of-scope churn.

### Documentation
- [ ] The provider reference manifests now present a single, consistent `stagecoach` identity.
- [ ] No new env vars / config keys / CLI surface (DOCS clause: "Mode A — provider docs reference manifests"
      — this IS the doc sync for this surface).

---

## Anti-Patterns to Avoid

- ❌ **Don't blanket-sed without the comment-only pre-check.** Run Task 1 first (strip `#` lines, grep the
  remainder). If a functional value contained `stagehand`, the blanket sed would silently change behavior.
  (Verified: all 18 matches are comments — but prove it, don't assume it.)
- ❌ **Don't forget the `g` flag.** Line 16 has TWO `stagehand` occurrences per line (`stagehand/config.toml`
  AND `.stagehand.toml`). Without `g`, only the first per line is replaced → stale residue + a half-renamed
  path that's neither valid nor consistent.
- ❌ **Don't touch functional values "while you're in there."** The flag lists (`bare_flags`/`tooled_flags`),
  models, `[reasoning_levels]`, `[env]`, and the `name`/`command`/`detect`/`*_flag` keys are AGENT-specific
  (pi, claude, etc.) — none contain `stagehand`, and none should be edited. Only the word `stagehand` in
  comments changes.
- ❌ **Don't edit `internal/provider/builtin.go`.** That's the compiled-in manifest (Go source, renamed in
  M1.T2). The TOML files MIRROR it; if a comment refers to a field value, the value stays — only the comment
  word `stagehand` → `stagecoach`.
- ❌ **Don't cross into other rename surfaces.** `docs/*.md` is P1.M4.T1.S2 (Layer 5.2, parallel);
  `FUTURE_SPEC.md` is P1.M4.T2.S2 (Layer 5.4); `plan/` is P1.M5.T1. This task is `providers/*.toml` ONLY
  (Layer 5.3).
- ❌ **Don't "preserve old paths for backwards compat."** The renamed binary does NOT read `.stagehand.toml`
  (it reads `.stagecoach.toml` per P1.M2.T2). The comments must say `stagecoach` to be accurate.
- ❌ **Don't expect a test to validate this.** These TOMLs are reference docs, not runtime-loaded; no test
  reads their comments. Validation is textual (grep + diff). `go test ./...` is a regression check only.
- ❌ **Don't drop the `Stagehand` arm of the sed.** There are no `Stagehand` matches today, but keeping the
  arm matches the sibling rename PRPs and is harmless — dropping it creates an inconsistency for no gain.
