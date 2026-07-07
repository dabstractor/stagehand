# P1.M5.T1.S1 — Research Findings
## docs/how-it-works.md diff-capture section (describe FR3d–FR3i)

---

## 0. Task contract (verbatim from item_description)

Mode B changeset-level docs update. The diff-capture section of `docs/how-it-works.md` "currently describes
only the markdown/non-markdown caps + binary filtering; it predates FR3d-i." Update it (user-facing prose)
to describe the implemented M2/M3/M4 diff-capture pipeline, accurate to the implementation, cross-linking to
`docs/CONFIGURATION.md` for the knobs:

- (a) compact numstat skeleton prepended to every payload (**FR3g**);
- (b) deterministic rename detection via `-M` (**FR3e**);
- (c) reduced `-U1` context by default, configurable via `diff_context` 0–3 (**FR3f**);
- (d) index-line stripping for ~30 bytes/file (**FR3h**);
- (e) the optional `token_limit` holistic budget with water-fill truncation (**FR3d/FR3i**), and that
  `token_limit==0` preserves the legacy `max_diff_bytes`/`max_md_lines` behavior.

DOCS: this IS the changeset-level docs update for how-it-works (Mode B). OUTPUT: docs/how-it-works.md
reflects the new diff-capture pipeline.

---

## 1. The file being edited — `docs/how-it-works.md` (266 lines, ~21KB)

Current top-level structure:
1. `# How Stagecoach works`
2. `## The snapshot-based flow` (why not git commit / plumbing / invariants)
3. `## Stage-while-generating`
4. `## Multi-commit decomposition` (trigger / four roles / pipeline / key design points / safety)
   - `### Binary and non-text file filtering`  ← diff-capture content lives here (FR3a–c)
   - `### Payload exclusions (.stagecoachignore)`  ← FR-X
5. `## Safety and the rescue protocol`
6. `## Prompt engineering`
7. `## Hook mode vs the snapshot-based flow`

**Diff-capture content today**: ONLY the `### Binary and non-text file filtering` subsection (binary/lock/
snapshot/sourcemap/vendor exclusion + the `[binary] <status>\t<path>` placeholder, FR3a–c) + the
`### Payload exclusions (.stagecoachignore)` subsection (FR-X). **There is NO description of FR3d–FR3i**
(skeleton, `-M`, `-U1`, index-strip, token_limit water-fill). That is the gap.

**Placement decision**: add a NEW `### Diff capture pipeline` subsection IMMEDIATELY BEFORE the existing
`### Binary and non-text file filtering` subsection. This forms a coherent diff-capture cluster at the end
of `## Multi-commit decomposition`: **pipeline (FR3d–i) → binary filtering (FR3a–c) → exclusions (FR-X)**.
(The existing binary/exclusions subsections already apply to "every diff payload — staged diff, working-tree
snapshot, and concept diff", so they are correctly scoped even though they sit under the decompose heading;
the new pipeline subsection uses the same scope. Do NOT restructure the doc — the contract says "update the
diff-capture section", not "restructure".)

---

## 2. Verified implementation facts (the docs MUST match these exactly)

Sourced from the implementation files (READ-ONLY; do NOT edit code):

| FR | implementation fact | source |
|----|---------------------|--------|
| FR3g | Skeleton header is literally `Change summary (numstat: added\tdeleted\tpath):` followed by one `<added>\t<deleted>\t<path>` line per changed file, then a blank line. Binary files render as `-\t-\t<path>`. Prepended to EVERY diff payload (StagedDiff/TreeDiff/WorkingTreeDiff). Empty change set ⇒ no skeleton (preserves the FR5 empty-payload check). | `internal/git/skeleton.go:23` (`numstatSkeletonHeader`), `:36` (`renderNumstatSkeleton`) |
| FR3e | `-M` is appended to EVERY `git diff` invocation (deterministic rename detection). Copy detection `-C` is intentionally NOT enabled. | `internal/git/git.go:695` (`args = append(args, "-M")`), also `:756`, `:1228` |
| FR3f | `-U<diff_context>` is appended to every diff. `diff_context` ∈ [0,3]; 0 = `-U0` (changed lines only, VALID), 1 = one anchor (the DEFAULT, resolved upstream nil⇒1), 3 = git default. | `internal/git/git.go:690-696` (`buildDiffArgs`), `StagedDiffOptions.DiffContext` |
| FR3h | `stripIndexLines` removes the `index <oid>..<oid> <mode>` line from each file diff. The `diff --git`, `---`, `+++`, `@@` lines are RETAINED. Runs at capture, before any cap. | `internal/git/git.go:709` (doc), `:717` (`stripIndexLines`), called `:836`, `:863`, `:1299`, `:1323` |
| FR3d/FR3i | Gated on `opts.TokenLimit`. `==0` ⇒ legacy caps byte-identical: `... [diff truncated at N lines]` (md, `max_md_lines`) and `... [diff truncated at N bytes]` (non-md, `max_diff_bytes`). `>0` ⇒ water-fill REPLACES both caps; per-file `... [truncated]` (SHORTER form) sentinel on truncated files; the `at N bytes/lines` sentinels NEVER appear. body_budget = token_limit − skeleton − promptReserve − margin. | `internal/git/git.go:840,868` (legacy sentinels); `internal/git/truncatediff.go:57` (`truncatedSentinel = "... [truncated]"`); `internal/git/tokengate.go` (`applyWaterFillGate`) |

**Water-fill semantics (FR3i) — for accurate prose**: size each file's body up front; if `Σ size_i ≤ budget`,
include every file whole (the common case); else find water level `L` with `Σ min(size_i, L) = budget` —
files smaller than `L` whole and untouched, files larger than `L` truncated to exactly `L` (+ `... [truncated]`).
Guarantees: small files never penalized; large files get the bulk; no file monopolizes; budget fully used.

**Two mutually-exclusive modes (FR3d)**: `token_limit == 0`/unset ⇒ legacy `max_md_lines`/`max_diff_bytes`
caps apply unchanged. `token_limit > 0` ⇒ supersedes BOTH legacy caps for that run. The skeleton (FR3g),
`-M` (FR3e), `-U<n>` (FR3f), index-strip (FR3h) apply in BOTH modes (they are cap-independent capture steps).

---

## 3. Cross-link target — `docs/configuration.md` (verified anchors + exact knob names)

The `[generation]` knobs are documented in TWO places in configuration.md:
- **File format** block (lines 102-115): the commented `[generation]` template with `max_diff_bytes`,
  `max_md_lines`, `token_limit`, `diff_context`.
- **Built-in defaults** (lines 117-147): the defaults table (`max_diff_bytes=300000`, `max_md_lines=100`,
  `token_limit=0`, `diff_context=1`) + the detailed `> **token_limit**` (line 146) and `> **diff_context**`
  (line 147) prose descriptions.

**Exact knob names (use these verbatim in the docs)**:
- `max_diff_bytes` (default 300000) — non-markdown aggregate byte cap (legacy).
- `max_md_lines` (default 100) — per-file markdown line cap (legacy).
- `token_limit` (default 0) — holistic token budget; 0 = unset ⇒ legacy caps; >0 supersedes both.
- `diff_context` (default 1) — 0/1/3 unchanged-context-lines per hunk.
- (binary: `binary_extensions`; exclusions: `[generation].exclude` / `.stagecoachignore` — owned by the
  existing binary/exclusions subsections, NOT this task.)

**Cross-link**: link to `configuration.md#built-in-defaults` (the section with the detailed token_limit /
diff_context prose + the defaults table). GitHub-style auto-anchor from the `## Built-in defaults` heading.

NOTE: the file is `docs/configuration.md` (lowercase). The contract writes `docs/CONFIGURATION.md`; the
actual on-disk name is lowercase — use `configuration.md` (the existing how-it-works.md links already use
`configuration.md`, e.g. the decompose section's "See [configuration.md](configuration.md)").

---

## 4. What NOT to touch (scope boundary)

- The `### Binary and non-text file filtering` and `### Payload exclusions (.stagecoachignore)` subsections
  STAY (they cover FR3a–c and FR-X correctly). The new pipeline subsection COMPLEMENTS them, placed before
  them. Do not duplicate binary/exclusion content.
- No other section of how-it-works.md (snapshot flow, decompose pipeline, safety, prompt engineering, hook
  mode) is in scope.
- P1.M5.T1.S2 owns README.md + providers.md/cli.md consistency — do NOT edit those here.
- NO source-code changes (this is Mode B docs-only). NO tests. The implementation (M2/M3/M4) is the input.

---

## 5. Validation (docs-only — lightweight)

```bash
# 1. The new subsection + the five FR3d-i concepts are present:
grep -nE 'Diff capture pipeline|skeleton|numstat|rename|`-M`|`-U1`|diff_context|index.line|token_limit|water.fill|max_diff_bytes|max_md_lines' docs/how-it-works.md
# 2. The cross-link resolves (target heading exists in configuration.md):
grep -n '^## Built-in defaults' docs/configuration.md
# 3. configuration.md referenced with the correct (lowercase) filename:
grep -n 'configuration.md' docs/how-it-works.md
# 4. Markdown sanity (if a linter is available; else visual review):
markdownlint docs/how-it-works.md 2>/dev/null || echo "(no markdownlint — visual review)"
# 5. No code changed:
git status --porcelain   # expect ONLY docs/how-it-works.md
```

No `go build`/`go test` (no code change). The gate is: the five FR3d-i concepts are described accurately
(matching §2's implementation facts), the cross-link resolves, the markdown is well-formed, and only the one
docs file changed.

---

## 6. Confidence & risks

**Confidence: 9.5/10.** Pure prose addition to one docs file. All implementation facts verified against source
(§2). The cross-link target exists (§3). The placement is decided (§1). No code/test risk.

**Risks (low):**
- **Accuracy drift.** The docs must match the implementation (§2), not the PRD's FR3 wording alone — e.g.
  the skeleton header literal, the two distinct sentinel forms, the `==0` byte-identical contract. The PRP
  quotes these verbatim so the implementer doesn't paraphrase them wrong.
- **Over-scoping.** Temptation to also document FR3a-c (binary) / FR-X (exclusions) — those already have
  their own subsections; the new subsection must NOT duplicate them. The PRP scopes to FR3d-i only.
- **Filename casing.** Contract says `CONFIGURATION.md`; the real file + existing links are `configuration.md`
  (lowercase). Use lowercase or the link breaks on case-sensitive FS.
