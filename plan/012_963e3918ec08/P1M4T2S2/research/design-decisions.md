# P1.M4.T2.S2 — Design decisions (rename FUTURE_SPEC.md references: stagehand → stagecoach)

> Layer 5.4 of the stagehand→stagecoach project rename (rename_surface_map.md §5.4). Companion to the
> providers/*.toml rename (P1.M4.T2.S1, Layer 5.3, parallel). Single prose file, single mechanical pass.

## F1 — The surface, exactly (verified 2026-07-07)

`FUTURE_SPEC.md` (repo root, 7558 bytes — matches the item_description's "~7.5KB") contains exactly
**10 occurrences** of the project's old name, all standalone:

- **8 lowercase `stagehand`** + **2 capitalized `Stagehand`** + **0 `STAGEHAND`**.
- Across **10 lines**: 1 (the H1 title `# Stagehand — Future Spec`), 24, 27, 28, 34, 41 (the editor-
  integration family — prose + `stagehand --dry-run` command refs), 77 (`Stagehand's entire thesis`),
  93 & 94 (the §3 rejected-features table prose), 100 (the `--clipboard` row's
  `stagehand --dry-run --no-color | wl-copy` example).
- **No compound tokens.** A `grep -oE '[A-Za-z]*[Ss]tagehand[A-Za-z]*'` returns only the two distinct
  standalone words `stagehand` and `Stagehand` — there is no `stagehandignore`, `stagehandback`, no
  identifier with a suffix/prefix. The blanket `sed` is therefore SAFE: it cannot corrupt a larger token
  because there are none.

This is the proof that the contract's literal command (`sed -i 's/stagehand/stagecoach/g;
s/Stagehand/Stagecoach/g' FUTURE_SPEC.md`) is correct-by-construction: every match is the standalone
old name, and every replacement is the standalone new name.

## F2 — Pure prose, zero runtime/test impact (verified)

FUTURE_SPEC.md is a **companion document** (its own header: "Companion to `PRD.md`"). It is NOT loaded,
parsed, or referenced by:

- any Go source (`grep -rn 'FUTURE_SPEC' --include='*.go' .` → empty);
- any test;
- the Makefile, `.goreleaser.yaml`, `.github/` workflows, or `.gitignore`.

So the rename is **textual only**. `go build ./...` and `go test ./...` are regression-safety checks
(expected green, unchanged) — NOT feature tests. Validation is `grep` + `git diff`, exactly like the
providers/*.toml rename (S1). No test reads this file's contents; do not expect one to validate the rename.

## F3 — The rename is CORRECTNESS-RELEVANT, not just cosmetic

Four of the ten references are **command/invocation examples** that name the CLI binary:

- L24: `invoking \`stagehand --dry-run\`` → must become `stagecoach --dry-run`.
- L28: `\`stagehand --dry-run\`` → `stagecoach --dry-run`.
- L34: `shell out to the installed \`stagehand\` binary` → `stagecoach` binary.
- L100: `\`stagehand --dry-run --no-color | wl-copy\`` → `stagecoach --dry-run --no-color | wl-copy`.

The renamed binary is **`stagecoach`** — `cmd/stagecoach` exists (the directory was renamed in
P1.M1.T1.S2, Complete; `cmd/stubagent` is the unrelated test stub). Leaving these as `stagehand` would
document a command the renamed binary no longer answers to. Post-sed they correctly say `stagecoach`,
matching the actual binary + the already-renamed docs/providers/Go source. (Same accuracy argument as
S1's config-path comments: the doc must match the renamed reality.)

## F4 — The sed command, verified end-to-end (dry-run preview)

The contract's exact command is two case-disjoint arms:

```bash
sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' FUTURE_SPEC.md
```

- **Case-disjoint ⇒ order-safe.** sed is case-sensitive: `s/stagehand/.../g` matches only lowercase
  `stagehand` (initial `s`); it does NOT touch `Stagehand` (initial capital `S`). Then
  `s/Stagehand/Stagecoach/g` handles the two capitalized occurrences. Neither arm re-creates the other's
  pattern (`stagecoach` contains no `Stagehand`; `Stagecoach` contains no `stagehand`). Both arms are
  REQUIRED (dropping the `Stagehand` arm would leave lines 1 and 77 half-done).
- **`g` flag** handles multiple occurrences per line (none of the 10 lines has >1, but `g` is harmless
  and matches the sibling rename PRPs' contract — keep it).
- **Verified by dry-run**: `sed 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' FUTURE_SPEC.md |
  grep -c -i stagehand` → **0**. The sed fully clears the surface in one pass (every expected line —
  1, 24, 27, 28, 34, 41, 77, 93, 94, 100 — shows `stagecoach`/`Stagecoach` post-run).

## F5 — Scope boundary (Layer 5.4 ONLY; disjoint from siblings)

rename_surface_map.md pins this task as **Layer 5.4 = FUTURE_SPEC.md ("Any references to stagehand")** —
a single file, disjoint from every other rename surface:

- **Layer 5.2 (`docs/*.md`)** = P1.M4.T1.S2 (parallel, in progress) — different files, no conflict.
- **Layer 5.3 (`providers/*.toml`)** = P1.M4.T2.S1 (parallel — the previous PRP) — 8 different files.
- **`plan/` artifacts** = P1.M5.T1 (planned, later) — not touched here.
- **PRD.md / tasks.json / prd_snapshot.md** = orchestrator-owned, READ-ONLY — never touched.

The **final whole-repo zero-residue audit** (`grep -ri 'stagehand' ... | wc -l == 0`, rename_surface_map
verification gate 5) is **P1.M5.T2.S1**, NOT this task. This task closes ONLY the FUTURE_SPEC.md surface;
other surfaces are siblings' responsibility.

**COMPETITOR-ANALYSIS.md note:** FUTURE_SPEC.md references `COMPETITOR-ANALYSIS.md` in its prose (the
disposition-methodology preamble + §2.1). That file is NOT present in the repo today and is OUT OF SCOPE
for this task (it is not FUTURE_SPEC.md). If it existed and contained `stagehand`, it would be a separate
surface — not this one. Do not create or edit it.

## F6 — Platform sed vs the edit tool (determinism for a 0.5-point task)

The contract specifies `sed -i`. On **GNU sed** (Linux, the CI environment) `sed -i 's/.../.../g' file`
(no extension arg) works directly. On **macOS BSD sed**, `sed -i '' 's/.../.../g' file` (empty backup-ext
arg) is required — the GNU form fails. For a single small prose file, the **edit tool** with exact-text
replacements per line is platform-independent and fully deterministic (no BSD/GNU divergence), and is the
preferred mechanism for this 0.5-point task. The `sed` command remains the documented contract either way;
both produce byte-identical output (verified by the dry-run).

The 10 edits (if using the edit tool) are each a unique line substring — safe for exact-text replacement:

| Line | old (contains) | new |
|------|----------------|-----|
| 1  | `# Stagehand — Future Spec (deferred & rejected ideas)` | `# Stagecoach — Future Spec (deferred & rejected ideas)` |
| 24 | `invoking \`stagehand --dry-run\`` | `invoking \`stagecoach --dry-run\`` |
| 27 | `wrapping \`stagehand` /` | `wrapping \`stagecoach` /` |
| 28 | `\`stagehand --dry-run\`. The primary author` | `\`stagecoach --dry-run\`. The primary author` |
| 34 | `shell out to the installed \`stagehand\` binary` | `shell out to the installed \`stagecoach\` binary` |
| 41 | `custom-command facility to bind \`stagehand\` to.` | `… \`stagecoach\` to.` |
| 77 | `Stagehand's entire thesis` | `Stagecoach's entire thesis` |
| 93 | `stagehand never owns the model call.` | `stagecoach never owns the model call.` |
| 94 | `stagehand writes commit messages, nothing else.` | `stagecoach writes commit messages, nothing else.` |
| 100| `\`stagehand --dry-run --no-color \| wl-copy\`` | `\`stagecoach --dry-run --no-color \| wl-copy\`` |

(Each `old` substring is unique in the file — the edit tool's uniqueness requirement is satisfied. Line
27's `wrapping \`stagehand` /` spans a line wrap; include enough context to be unique.)
