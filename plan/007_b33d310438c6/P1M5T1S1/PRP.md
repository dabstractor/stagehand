---
name: "P1.M5.T1.S1 (Mode B docs) — docs/how-it-works.md diff-capture section: describe the FR3d–FR3i pipeline (skeleton, -M, -U1, index-strip, token_limit water-fill)"
description: |

  Mode B changeset-level documentation update. docs/how-it-works.md (~21KB) documents the snapshot flow and
  diff capture, but its diff-capture content currently covers ONLY binary/non-text filtering (FR3a–c) and
  payload exclusions (FR-X) — it predates the M2/M3/M4 diff-pipeline work (FR3d–FR3i). Add a new
  `### Diff capture pipeline` subsection that describes, in user-facing prose, the five always-on / opt-in
  transforms every diff payload now goes through, accurate to the IMPLEMENTATION (not just the PRD wording),
  cross-linking to docs/configuration.md for the knobs.

  CONTRACT (item_description §1–§5; PRD §9.1 FR3d–FR3i):
    1. RESEARCH NOTE: "the diff-capture section currently describes only the markdown/non-markdown caps +
       binary filtering; it predates FR3d-i."
    2. INPUT: "the implemented behavior from M2/M3/M4 (always-on -M/-U1/skeleton/index-strip;
       token_limit⇒water-fill)."
    3. LOGIC: "Update the diff-capture section to describe, in user-facing prose: (a) the compact numstat
       skeleton prepended to every payload (FR3g); (b) deterministic rename detection via -M (FR3e);
       (c) reduced -U1 context by default, configurable via diff_context 0-3 (FR3f); (d) index-line stripping
       for ~30 bytes/file (FR3h); (e) the optional token_limit holistic budget with water-fill truncation
       (FR3d/FR3i), and that token_limit==0 preserves the legacy max_diff_bytes/max_md_lines behavior. Keep
       it accurate to the implementation; cross-link to docs/CONFIGURATION.md for the knobs."
    4. OUTPUT: "docs/how-it-works.md reflects the new diff-capture pipeline."
    5. DOCS: "[Mode B] THIS is the changeset-level docs update for how-it-works."

  DELIVERABLE (1 file MODIFIED; no code, no tests):
    MODIFY docs/how-it-works.md — insert a new `### Diff capture pipeline` subsection IMMEDIATELY BEFORE the
    existing `### Binary and non-text file filtering` subsection (inside `## Multi-commit decomposition`),
    describing FR3g/FR3e/FR3f/FR3h/FR3d-FR3i in user-facing prose + a cross-link to configuration.md.

  SCOPE BOUNDARY (do NOT edit):
    - The existing `### Binary and non-text file filtering` and `### Payload exclusions (.stagehandignore)`
      subsections — they cover FR3a–c / FR-X correctly; the new subsection COMPLEMENTS (not duplicates) them.
    - Every other section of how-it-works.md (snapshot flow, stage-while-generating, decompose pipeline,
      safety/rescue, prompt engineering, hook mode).
    - docs/configuration.md, docs/cli.md, docs/providers.md, README.md — P1.M5.T1.S2 owns consistency there.
    - ANY source code / tests (Mode B docs-only). The M2/M3/M4 implementation is the INPUT, read-only.

  SUCCESS: docs/how-it-works.md has a `### Diff capture pipeline` subsection describing all five FR3d-i
  concepts accurately (matching the implementation: skeleton header literal, -M always-on, -U1 default with
  diff_context 0/1/3, index-line strip, and the two mutually-exclusive token_limit modes with the
  `... [truncated]` vs `... [diff truncated at N bytes/lines]` distinction); a working cross-link to
  configuration.md; well-formed markdown; only docs/how-it-works.md changed.

---

## Goal

**Feature Goal**: Bring docs/how-it-works.md's diff-capture coverage current with the shipped M2/M3/M4
diff pipeline (PRD §9.1 FR3d–FR3i). Today the doc describes only binary/exclusion filtering; add a focused,
user-facing `### Diff capture pipeline` subsection that explains the five transforms every payload now goes
through — the numstat skeleton, deterministic `-M` renames, reduced `-U1` context, index-line stripping, and
the optional `token_limit` water-fill budget — accurate to the implementation, with a cross-link to the knob
reference in configuration.md.

**Deliverable** (1 modified file; no code, no tests):
- `docs/how-it-works.md` — a new `### Diff capture pipeline` subsection inserted immediately before the
  existing `### Binary and non-text file filtering` subsection, covering FR3g / FR3e / FR3f / FR3h /
  FR3d–FR3i in prose + a cross-link to `configuration.md#built-in-defaults`.

**Success Definition**:
- A reader of how-it-works.md can understand, end-to-end, what stagehand does to a diff before the agent
  sees it: the skeleton completeness floor, compact deterministic renames, trimmed context, stripped index
  lines, and the two size-budget modes.
- Every implementation claim matches the actual code (the skeleton header literal, `-M`/`-U<n>` always-on,
  index-line strip, the two distinct truncation sentinels, the `==0` legacy / `>0` water-fill exclusivity).
- The cross-link to configuration.md resolves (correct lowercase filename + `#built-in-defaults` anchor).
- Markdown is well-formed; only `docs/how-it-works.md` is modified.

## User Persona

**Target User**: the stagehand user reading "How it works" to understand what the tool sends to their model
— especially a user deciding whether to set `token_limit` (their diff exceeds their model's context window),
or wondering why their payload looks different (compact renames, `-U1`, no `index` lines, a numstat header).

**Use Case**: user opens how-it-works.md → reads the diff-capture pipeline → understands (a) every payload
leads with a numstat skeleton so no file is silently dropped; (b) renames are compact; (c) context is
trimmed to `-U1` by default and is tunable; (d) they can set `token_limit` to fit their model, and it
water-fills fairly (small files whole, large files capped); (e) leaving `token_limit` unset keeps the
familiar `max_diff_bytes`/`max_md_lines` caps.

**User Journey**: reader scans the new subsection → clicks through to configuration.md for the exact knobs
→ sets `token_limit = 120000` (or `diff_context = 3`, etc.) informedly.

**Pain Points Addressed**: the docs currently silently omit the entire modern diff pipeline; a user
inspecting `--verbose` output (a numstat skeleton header, `... [truncated]` markers, no `index` lines) has
no doc explaining any of it. This subsection closes that gap.

## Why

- **It IS the P1.M5.T1.S1 contract.** Mode B docs sync: how-it-works.md must reflect the M2/M3/M4
  implementation. The diff-capture section "predates FR3d-i" — this adds exactly that.
- **Accuracy over PRD paraphrase.** The docs must match the IMPLEMENTATION (skeleton.go's literal header,
  the two sentinel forms, the `==0` byte-identical contract), not just restate the PRD FR wording — so a
  user reading `--verbose` output recognizes what they see.
- **Discovery of the knobs.** The `token_limit` / `diff_context` knobs are the user-facing surface of this
  work; the subsection cross-links to configuration.md rather than duplicating the reference.
- **Complements, doesn't duplicate.** The existing binary (FR3a–c) and exclusions (FR-X) subsections stay;
  this subsection sits before them, forming a coherent pipeline → filter → exclude cluster.
- **Cheap and isolated.** One prose subsection in one docs file. No code, no tests, no cross-file churn.

## What

Insert ONE new `### Diff capture pipeline` subsection into `docs/how-it-works.md`, immediately before the
existing `### Binary and non-text file filtering` subsection (which is the first diff-capture subsection at
the end of `## Multi-commit decomposition`). The subsection describes five transforms, in pipeline order:

1. **Compact change skeleton (FR3g)** — a `git diff --numstat` summary prepended to every payload; one
   `added\tdeleted\tpath` line per changed file; the completeness floor so truncation never silently drops a
   file.
2. **Deterministic rename detection (FR3e)** — `-M` on every `git diff`; compact renames instead of
   delete+add; independent of the user's `diff.renames` config / git version.
3. **Reduced diff context (FR3f)** — `-U1` default; `diff_context` ∈ {0,1,3}.
4. **Index-line stripping (FR3h)** — the `index <oid>..<oid> <mode>` line removed (~30 bytes/file); the
   `diff --git`/`---`/`+++`/`@@` lines retained.
5. **Size budget (FR3d / FR3i)** — two mutually-exclusive modes: `token_limit == 0` (default) keeps the
   legacy `max_md_lines` / `max_diff_bytes` caps; `token_limit > 0` supersedes both with a holistic
   water-fill (small files whole, large files capped at a shared level `L` with a `... [truncated]` marker).

Close with a one-line cross-link to `configuration.md#built-in-defaults` for the `token_limit`,
`diff_context`, `max_diff_bytes`, and `max_md_lines` knobs.

### Success Criteria

- [ ] `docs/how-it-works.md` contains a new `### Diff capture pipeline` subsection placed immediately before
      `### Binary and non-text file filtering`.
- [ ] The subsection describes ALL FIVE: FR3g (numstat skeleton), FR3e (`-M`), FR3f (`-U1` + `diff_context`
      0/1/3), FR3h (index-line strip), FR3d/FR3i (`token_limit` water-fill + the `==0` legacy-back-compat).
- [ ] Implementation facts are accurate: skeleton header phrasing matches `Change summary (numstat: …)`;
      `-M`/`-U1` described as always-on defaults; `diff_context` default is 1; index line named
      `index <oid>..<oid> <mode>`; the water-fill sentinel is `... [truncated]` and the legacy sentinels are
      `... [diff truncated at N bytes/lines]`; `token_limit > 0` SUPERSEDES (not adds to) the legacy caps.
- [ ] A working cross-link to `configuration.md#built-in-defaults` (lowercase filename).
- [ ] Well-formed markdown (valid headings, code-fences, link syntax); no duplication of the binary/exclusion
      subsections' content.
- [ ] ONLY `docs/how-it-works.md` is modified.

## All Needed Context

### Context Completeness Check

_Pass._ A technical writer with no prior repo knowledge can implement this from: the exact subsection
placement (before `### Binary and non-text file filtering`), the five concepts to cover with their verified
implementation facts (quoted below), the exact knob names + cross-link anchor, and the ready-to-adapt prose
skeleton in the Implementation Blueprint. No code reading beyond the verified-fact table is required.

### Documentation & References

```yaml
# MUST READ — THE decisive doc (verified facts + placement + cross-link)
- docfile: plan/007_b33d310438c6/P1M5T1S1/research/findings.md
  why: §1 the current how-it-works.md structure + the placement decision (new subsection before
       `### Binary and non-text file filtering`); §2 the VERIFIED implementation facts table (the docs MUST
       match these — skeleton header literal, -M/-U sources, stripIndexLines, the two sentinel forms, the
       ==0/>0 exclusivity); §3 the exact knob names + the configuration.md cross-link target; §4 scope
       (don't touch binary/exclusions subsections, other sections, or other docs files); §5 validation.
  critical: §2 (accuracy — the sentinel distinction + the skeleton header literal), §1 (placement), §3
       (knob names + lowercase filename).

# MUST READ — the file being EDITED
- file: docs/how-it-works.md   (EDIT — insert ONE new subsection)
  section: `## Multi-commit decomposition` → its trailing diff-capture cluster: `### Binary and non-text file
       filtering` (currently the first diff-capture subsection) and `### Payload exclusions (.stagehandignore)`.
       INSERT the new `### Diff capture pipeline` subsection IMMEDIATELY BEFORE `### Binary and non-text file
       filtering`.
  why: this is THE insertion point. The existing binary/exclusions subsections already scope themselves to
       "every diff payload — staged diff, working-tree snapshot, and concept diff" — the new pipeline
       subsection uses the SAME scope, so it reads coherently as pipeline → filter → exclude.
  pattern: match the file's existing prose style — short H3 subsections, a lead sentence stating the scope,
           sparing code formatting for literals (e.g. `-M`, `-U1`, `index <oid>..<oid> <mode>`), and a
           trailing cross-link line like the existing "See [configuration.md](configuration.md) for …".
  gotcha: do NOT duplicate the binary (FR3a–c) or exclusions (FR-X) content — those subsections stay. The new
          subsection covers FR3d–i ONLY. Do NOT restructure the doc (no moving subsections out from under
          `## Multi-commit decomposition`) — the contract says "update the diff-capture section".

# MUST READ — the cross-link TARGET (verify the anchor + knob names; do NOT edit)
- file: docs/configuration.md   (READ ONLY)
  section: `## Built-in defaults` (heading → anchor `#built-in-defaults`): the defaults table
       (`max_diff_bytes=300000`, `max_md_lines=100`, `token_limit=0`, `diff_context=1`) + the detailed
       `> **token_limit**` and `> **diff_context**` prose. ALSO the `[generation]` block under `## File format`.
  why: the cross-link destination. The four knobs (`token_limit`, `diff_context`, `max_diff_bytes`,
       `max_md_lines`) are documented there; how-it-works.md should POINT there, not re-document the values.
  gotcha: the filename is LOWERCASE (`configuration.md`), not `CONFIGURATION.md`. The contract writes the
          latter but the on-disk file + every existing link in how-it-works.md use lowercase — use lowercase
          or the link 404s on case-sensitive filesystems.

# READ — the implementation sources (confirm the docs match; do NOT edit)
- file: internal/git/skeleton.go   (READ ONLY)
  section: `numstatSkeletonHeader = "Change summary (numstat: added\\tdeleted\\tpath):"` (line 23) +
       `renderNumstatSkeleton` (binary rows render `-\t-\t<path>`; empty change set ⇒ no skeleton).
  why: the FR3g prose must reference the skeleton accurately (header phrasing, per-file line shape, the
       completeness-floor guarantee) without pasting code.
- file: internal/git/git.go   (READ ONLY)
  section: `buildDiffArgs` appends `-M` (line 695) and `-U<diff_context>` (line 696); `stripIndexLines`
       removes the `index <oid>..<oid> <mode>` line (lines 709/717); the legacy `==0` sentinels at lines
       840 (`... [diff truncated at %d lines]`) and 868 (`... [diff truncated at %d bytes]`).
  why: confirms FR3e (`-M` always-on), FR3f (`-U1` default; `diff_context` 0/1/3), FR3h (the exact line
       stripped + what's retained), and the legacy-side of the FR3d two-mode contract.
- file: internal/git/truncatediff.go + internal/git/tokengate.go   (READ ONLY)
  section: `truncatedSentinel = "... [truncated]"` (truncatediff.go:57); `applyWaterFillGate`
       (tokengate.go) computes body_budget = token_limit − skeleton − promptReserve − margin and water-fills.
  why: confirms the `>0` sentinel is the SHORTER `... [truncated]` form (distinct from the legacy
       `... [diff truncated at N bytes/lines]`) and the water-fill semantics (small whole, large capped at L).

- url: (PRD internal) PRD.md §9.1 FR3d–FR3i (in context as selected_prd_content h3.17). AUTHORITATIVE FR
       wording — use it to structure the prose, but RESOLVE any wording ambiguity against the implementation
       (§2 of findings) when they differ (they don't materially, but the sentinel forms / header literal are
       implementation-accurate details the PRD leaves implicit).
```

### Current Codebase tree (relevant slice)

```bash
docs/
  how-it-works.md        # EDIT — insert `### Diff capture pipeline` before `### Binary and non-text file filtering`.
  configuration.md       # READ ONLY — cross-link target (#built-in-defaults; the 4 knobs). NOT edited here.
  cli.md / providers.md / README.md  # NOT edited here (P1.M5.T1.S2 owns consistency).
internal/git/
  skeleton.go            # READ ONLY — numstatSkeletonHeader (FR3g accuracy).
  git.go                 # READ ONLY — buildDiffArgs (-M/-U<n)), stripIndexLines (FR3h), legacy sentinels (FR3d ==0).
  truncatediff.go        # READ ONLY — truncatedSentinel "... [truncated]" (FR3i >0).
  tokengate.go           # READ ONLY — applyWaterFillGate water-fill (FR3d/FR3i >0).
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. 1 MODIFIED docs file only:
docs/how-it-works.md     # + `### Diff capture pipeline` subsection (FR3d–i prose + configuration.md cross-link).
# NO code changes. NO tests. NO other docs files.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (accuracy — two DISTINCT sentinels): the FR3d `==0` legacy path emits `... [diff truncated at N bytes]`
     (non-md) / `... [diff truncated at N lines]` (md); the `>0` water-fill path emits the SHORTER `... [truncated]`
     per truncated file. The docs must NOT conflate these (a user grepping `--verbose` output will see one or the
     other depending on whether token_limit is set). State both, and that `token_limit > 0` makes the legacy
     forms disappear for that run. (findings §2.) -->

<!-- CRITICAL (the two modes are MUTUALLY EXCLUSIVE — FR3d): `token_limit > 0` SUPERSEDES (replaces) both
     `max_diff_bytes` and `max_md_lines` for that run; it does NOT add to them. And `token_limit == 0`/unset
     keeps the legacy caps UNCHANGED (byte-identical). Say "supersedes"/"replaces", never "adds to" or "caps at". -->

<!-- CRITICAL (skeleton header literal): the FR3g skeleton begins with the literal line
     `Change summary (numstat: added\tdeleted\tpath):`. Reference it recognizably (e.g. "a `Change summary
     (numstat: …)` header") so a user seeing it in `--verbose` knows what it is. Don't paste the raw tab bytes. -->

<!-- CRITICAL (`-M` and `-U1` are ALWAYS-ON defaults, not opt-in): FR3e (`-M`) and FR3h (index-strip) are
     unconditional; FR3f (`-U1`) is the default with `diff_context` tunable to 0/1/3. Only the SIZE BUDGET
     (FR3d) is opt-in via `token_limit`. Do not imply `-M` or index-strip are configurable — they aren't. -->

<!-- GOTCHA (cross-link casing): the file is `docs/configuration.md` (LOWERCASE). The contract writes
     `CONFIGURATION.md` but the on-disk file + existing how-it-works.md links use lowercase. Use
     `configuration.md` or the link 404s on case-sensitive FS. Anchor: `#built-in-defaults`. -->

<!-- GOTCHA (scope — don't duplicate): the binary (FR3a–c) and exclusions (FR-X) subsections ALREADY exist
     immediately after the insertion point. The new subsection covers FR3d–i ONLY. Mention binary/excluded
     placeholders at most in one passing clause ("alongside the binary/exclusion filtering described below")
     and let those subsections own the detail. -->

<!-- GOTCHA (scope — don't restructure): the diff-capture subsections sit under `## Multi-commit decomposition`
     but apply to ALL paths (single + decompose). That's a pre-existing structural quirk. Do NOT promote them
     to a top-level section — the contract is "update the diff-capture section", a focused prose addition. -->

<!-- GOTCHA (diff_context 0 is VALID): `diff_context = 0` (changed lines only) is a first-class value, not
     "unset". The default is 1. Document the 0/1/3 scale explicitly so a user doesn't think 0 disables the cap. -->
```

## Implementation Blueprint

### Data models and structure

```markdown
<!-- No data models — this is a prose subsection. The "structure" is: one H3, a 1-sentence scope lead,
     a numbered list of the 5 transforms (pipeline order), a short paragraph on the two size-budget modes,
     and a trailing cross-link line. Mirror the file's existing H3 subsections (e.g. the lead style of
     `### Binary and non-text file filtering`). -->
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: LOCATE the insertion point in docs/how-it-works.md (READ, no edit yet)
  - FIND the heading `### Binary and non-text file filtering` (inside `## Multi-commit decomposition`,
    near the end of that section, just before `### Payload exclusions (.stagehandignore)` and the
    `## Safety and the rescue protocol` top-level section).
  - CONFIRM the surrounding prose: that subsection opens "Binary files, lock files, snapshots, sourcemaps,
    and vendor directories are **excluded from every diff payload** — staged diff, working-tree snapshot,
    and concept diff." The new subsection will use the SAME scope ("every diff payload") and sit ABOVE it.
  - NOTE the file's cross-link style (e.g. the decompose section ends with "See [configuration.md]
      (configuration.md) for per-role model configuration and [cli.md](cli.md) for …") — mirror it.
  - GOTCHA: if the heading text differs slightly from `### Binary and non-text file filtering`, locate it by
      its unique opening sentence ("Binary files, lock files … are **excluded from every diff payload**").

Task 2: INSERT the new `### Diff capture pipeline` subsection (THE deliverable)
  - FILE: docs/how-it-works.md. INSERT immediately BEFORE the `### Binary and non-text file filtering`
      heading (i.e. after the last line of the preceding decompose content — the "Freeze enforcement" /
      "One-file short-circuit" / "Arbiter leftover reconciliation" / Safety bullet cluster ends, then the
      binary subsection begins; the new subsection goes in that gap, right above `### Binary and non-text…`).
  - CONTENT (adapt this proven skeleton — keep it user-facing, accurate to §2 of findings, no code pasting):

        ### Diff capture pipeline

        Every diff payload Stagehand builds — the staged diff, the multi-commit working-tree snapshot, and
        the per-concept tree-to-tree diff — goes through the same capture pipeline before it reaches the
        agent. Five transforms run, in order, in every path:

        1. **Compact change skeleton (FR3g).** A `git diff --numstat` summary is prepended to every payload,
           under a `Change summary (numstat: …)` header — one `added → deleted → path` line per changed file
           (binary files show as `-  -  <path>`). This is the completeness floor: the agent always sees the
           full shape of the change — every file, its add/delete magnitude, and its kind — even when
           individual bodies are truncated later. A file whose body is fully truncated still appears in the
           skeleton, so truncation never silently drops a file from view.

        2. **Deterministic rename detection (FR3e).** Every `git diff` passes `-M`, so a rename (or a
           near-rename above the similarity threshold) is emitted compactly — a `rename from` / `rename to`
           pair plus any residual edit — instead of as a delete + add that duplicates the full file content.
           This is deterministic regardless of your `diff.renames` config or git version. (Copy detection
           `-C` is intentionally not enabled — it is expensive and adds little for message generation.)

        3. **Reduced diff context (FR3f).** Diffs are captured with `-U1` by default — one anchor line per
           hunk — instead of git's `-U3` default, since unchanged surrounding lines are noise for message
           generation. Tune it with `diff_context`: `0` = changed lines only (maximal savings), `1` = one
           anchor line (the default), `3` = git's default.

        4. **Index-line stripping (FR3h).** The `index <oid>..<oid> <mode>` line is stripped from each file
           diff — the blob OIDs are useless to the model and cost roughly 30 bytes per file. The
           `diff --git`, `---`, `+++`, and `@@` lines are retained (they carry file identity and hunk
           location).

        5. **Size budget (FR3d / FR3i).** Two mutually-exclusive modes govern how large the payload is:
           - **Legacy caps (the default).** With `token_limit` unset (`0`), the markdown section is capped
             at `max_md_lines` per file (default 100) and the non-markdown aggregate at `max_diff_bytes`
             (default 300000); over-cap sections are marked `... [diff truncated at N bytes]` /
             `... [diff truncated at N lines]`.
           - **Holistic token budget.** Set `token_limit` (for example `120000`) to cap the *whole* payload
             — system prompt + style examples + the concatenated diff — to a token budget. Stagehand
             reserves room for the prompt and examples, then allocates the remainder to the diff bodies with
             a **dynamic water-fill**: it sizes every file's body up front, and if they exceed the budget it
             finds a single water level `L` such that every file *smaller* than `L` is included whole and
             untouched, and every file *larger* than `L` is truncated to `L` (with a `... [truncated]`
             marker). Small files are never penalized for their size; large, substantive files receive the
             bulk of the budget; no single file can monopolize it; and nothing is wasted. The common case —
             a commit that fits — is left untouched. A non-zero `token_limit` **supersedes** both legacy
             caps for that run (they are mutually exclusive).

        See [configuration.md](configuration.md#built-in-defaults) for the `token_limit`, `diff_context`,
        `max_diff_bytes`, and `max_md_lines` knobs.

  - WHY: this is the entire deliverable — user-facing prose for FR3g/e/f/h/d-i, accurate to the
      implementation (skeleton header, -M/-U1 always-on, index line, two sentinels, mutual exclusivity),
      with the cross-link.
  - GOTCHA: keep the bullet literals exact (`-M`, `-U1`, `index <oid>..<oid> <mode>`,
      `... [diff truncated at N bytes]`, `... [truncated]`). Use "supersedes"/"replaces" for the >0 mode
      (never "adds to"). State `diff_context` default is 1 and 0 is valid. Lowercase `configuration.md`.

Task 3: VERIFY (docs-only gates)
  - RUN the grep checks in "Validation Loop → Level 1" (all five concepts present; cross-link target exists;
      lowercase filename used; no duplication of binary/exclusion content).
  - VISUAL review: the subsection reads coherently as the lead-in to the `### Binary and non-text file
      filtering` subsection that follows it (pipeline → filter → exclude).
  - MARKDOWN sanity: valid H3, valid numbered list, valid link `(configuration.md#built-in-defaults)`.
  - `git status --porcelain` → ONLY docs/how-it-works.md modified.
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: mirror the file's existing H3 subsection style. `### Binary and non-text file filtering` is
     one H3 + a short paragraph scoped to "every diff payload" + cross-links. The new subsection is the same
     shape: H3 + scope lead + numbered transforms + a trailing cross-link line. -->

<!-- PATTERN: cross-link like the file already does. The decompose section ends with "See
     [configuration.md](configuration.md) for per-role model configuration and [cli.md](cli.md) for …".
     The new subsection's trailing line is the same idiom, anchored: "[configuration.md](configuration.md#built-in-defaults)". -->

<!-- CRITICAL (accuracy): the five bullets must match the implementation, not just the PRD. Concretely:
       - skeleton header phrasing ≈ "Change summary (numstat: …)" (skeleton.go:23).
       - -M is ALWAYS on (git.go:695); -U1 is the default with diff_context 0/1/3 (git.go:690-696).
       - index line is `index <oid>..<oid> <mode>`; the `diff --git`/`---`/`+++`/`@@` lines stay (git.go:709).
       - legacy sentinels `... [diff truncated at N bytes/lines]` (git.go:840/868) vs water-fill
         `... [truncated]` (truncatediff.go:57); >0 makes the legacy forms disappear (FR3d exclusivity). -->

<!-- CRITICAL (scope): cover FR3d–i ONLY. Binary (FR3a–c) and exclusions (FR-X) are the NEXT two subsections
     — reference them at most in one passing clause and let them own the detail. Do not restate the
     `[binary]`/`[excluded]` placeholder format here. -->

<!-- GOTCHA (do NOT restructure): leave the subsections under `## Multi-commit decomposition`. The contract is
     a focused prose addition ("update the diff-capture section"), not a doc restructuring. -->
```

### Integration Points

```yaml
DOCS.HOW_IT_WORKS (the only file edited):
  - insert: "### Diff capture pipeline subsection immediately before ### Binary and non-text file filtering"
  - content: "FR3g skeleton + FR3e -M + FR3f -U1/diff_context + FR3h index-strip + FR3d/FR3i token_limit
    water-fill (two mutually-exclusive modes) + cross-link"

DOCS.CONFIGURATION (cross-link target — READ ONLY):
  - link: "configuration.md#built-in-defaults (the token_limit / diff_context / max_diff_bytes / max_md_lines knobs)"
  - do-not-edit: "configuration.md is NOT modified by this task (P1.M5.T1.S2 owns cross-doc consistency)."

FROZEN/LEAVE (do NOT edit):
  - The ### Binary and non-text file filtering + ### Payload exclusions subsections (they stay; the new
    subsection complements them).
  - Every other section of how-it-works.md.
  - docs/configuration.md, docs/cli.md, docs/providers.md, README.md (P1.M5.T1.S2).
  - ALL source code and tests (Mode B docs-only).
```

## Validation Loop

### Level 1: Content & cross-link checks (docs-only — the real gate)

```bash
# 1. The new subsection + all five FR3d-i concepts are present:
grep -nE 'Diff capture pipeline|numstat|skeleton|rename|`-M`|`-U1`|diff_context|index.line|token_limit|water.fill|max_diff_bytes|max_md_lines' docs/how-it-works.md
# Expected: matches in the new subsection for EACH of: skeleton/numstat, -M, -U1/diff_context, index-line,
#           token_limit/water-fill, max_diff_bytes/max_md_lines.

# 2. The two sentinel forms are BOTH referenced (the accuracy contract):
grep -nE '\[\.\.\. \[truncated\]\]|\[\.\.\. \[diff truncated at' docs/how-it-works.md
# (If grep escaping is awkward, visually confirm both `... [truncated]` and `... [diff truncated at N bytes]`
#  appear, and that >0 supersedes the legacy caps.)

# 3. The cross-link target EXISTS in configuration.md:
grep -n '^## Built-in defaults' docs/configuration.md   # expect a match (the #built-in-defaults anchor source)

# 4. configuration.md is referenced with the correct LOWERCASE filename:
grep -n 'configuration.md' docs/how-it-works.md | grep -i 'built-in-defaults'   # expect the new cross-link

# Expected: all five concepts present; both sentinels referenced; cross-link target exists; lowercase link.
```

### Level 2: Markdown well-formedness

```bash
# If a markdown linter is available:
markdownlint docs/how-it-works.md 2>/dev/null && echo "markdownlint clean" || echo "(no markdownlint — do a visual review)"
# Visual review checklist:
#   - `### Diff capture pipeline` is a valid H3 (exactly one line, `### ` prefix).
#   - The numbered list (1.–5.) renders (blank line before it; consistent indentation).
#   - The two size-budget sub-bullets (Legacy caps / Holistic token budget) render as a nested bullet list.
#   - The cross-link `[configuration.md](configuration.md#built-in-defaults)` is valid inline link syntax.
#   - No raw tab characters leaked from the skeleton header description (describe it, don't paste `\t`).
```

### Level 3: Scope & accuracy audit

```bash
# ONLY docs/how-it-works.md changed (no code, no other docs):
git status --porcelain
# Expected: exactly one entry — docs/how-it-works.md (modified). Nothing under internal/, no other docs.

# The new subsection does NOT duplicate the binary/exclusion subsections' content:
sed -n '/^### Diff capture pipeline/,/^### Binary and non-text file filtering/p' docs/how-it-works.md | \
  grep -iE '\[binary\]|\[excluded\]|stagehandignore|lock file|sourcemap' && \
  echo "WARNING: new subsection duplicates binary/exclusion content (should only reference it in passing)" || \
  echo "OK: no binary/exclusion duplication"

# Accuracy spot-check against the implementation (the docs must match these literals):
grep -n 'Change summary (numstat' internal/git/skeleton.go          # the skeleton header source-of-truth
grep -n 'args = append(args, "-M")' internal/git/git.go             # -M always-on
grep -n 'truncatedSentinel = "... [truncated]"' internal/git/truncatediff.go   # the >0 sentinel
# Expected: each grep matches — the docs reference these concepts consistently with the code.
```

### Level 4: Render check (optional confidence)

```bash
# If a markdown renderer / preview is available, confirm the new subsection renders cleanly and the
# cross-link navigates to configuration.md's "Built-in defaults" section. (No automated gate; visual.)
# Expected: the pipeline → binary → exclusions cluster reads top-to-bottom as one coherent diff-capture
# narrative; the cross-link jumps to the knobs table.
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1: all five FR3d-i concepts present (skeleton, `-M`, `-U1`/`diff_context`, index-line strip,
      `token_limit` water-fill); both sentinel forms referenced; cross-link target exists; lowercase link.
- [ ] Level 2: markdown well-formed (valid H3, numbered list, nested bullets, valid link).
- [ ] Level 3: ONLY `docs/how-it-works.md` modified; no binary/exclusion duplication; docs literals match
      the implementation (`Change summary (numstat`, `-M`, `... [truncated]`).
- [ ] No source code or test files touched; no other docs files touched.

### Feature Validation
- [ ] A new `### Diff capture pipeline` subsection exists immediately before `### Binary and non-text file filtering`.
- [ ] All five transforms described: FR3g (numstat skeleton + completeness floor), FR3e (`-M` deterministic
      renames), FR3f (`-U1` default + `diff_context` 0/1/3), FR3h (index-line strip, ~30 bytes/file), FR3d/FR3i
      (`token_limit` water-fill; `==0` keeps `max_diff_bytes`/`max_md_lines`; `>0` supersedes both).
- [ ] Accuracy: skeleton header phrasing, always-on `-M`/`-U1`, the two distinct sentinels, and the
      mutual-exclusivity ("supersedes", not "adds to") all match the implementation.
- [ ] A working cross-link to `configuration.md#built-in-defaults` for the four knobs.

### Code Quality Validation
- [ ] The subsection mirrors the file's existing H3 prose style and cross-link idiom.
- [ ] No duplication of the binary (FR3a–c) or exclusions (FR-X) subsections' content.
- [ ] No doc restructuring (subsection stays under `## Multi-commit decomposition`); focused addition only.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment
- [ ] User-facing prose; no code pasted; literals rendered readably (no raw `\t`).
- [ ] The four knobs are named exactly (`token_limit`, `diff_context`, `max_diff_bytes`, `max_md_lines`) and
      point to configuration.md rather than re-documenting their full precedence.
- [ ] No new env vars / flags invented; the subsection describes existing shipped behavior only.

---

## Anti-Patterns to Avoid

- ❌ **Don't conflate the two sentinels.** `... [diff truncated at N bytes/lines]` is the LEGACY `==0` form;
  `... [truncated]` is the `>0` water-fill form. Document both, and that `token_limit > 0` makes the legacy
  forms disappear for that run. (gotcha)
- ❌ **Don't say token_limit "adds to" or "caps at" the legacy caps.** It SUPERSEDES (replaces) both — the two
  modes are mutually exclusive (FR3d). (gotcha)
- ❌ **Don't paraphrase the skeleton header inaccurately.** It begins `Change summary (numstat: …)`. Reference
  it recognizably so a user seeing it in `--verbose` recognizes it. Don't paste raw tab bytes. (gotcha)
- ❌ **Don't imply `-M` or index-stripping are configurable.** They are always-on (FR3e/FR3h). Only the SIZE
  BUDGET (`token_limit`) and `diff_context` are user-tunable. (gotcha)
- ❌ **Don't duplicate the binary/exclusion subsections.** They follow the new subsection and own FR3a–c / FR-X.
  Cover FR3d–i only; reference the others in at most one passing clause. (gotcha)
- ❌ **Don't use `CONFIGURATION.md` (uppercase).** The file + all existing links are lowercase
  `configuration.md`; uppercase 404s on case-sensitive FS. (gotcha)
- ❌ **Don't restructure the doc** (e.g. promote the diff subsections to a top-level section). The contract is
  a focused prose addition ("update the diff-capture section"). Leave the subsections under
  `## Multi-commit decomposition`. (gotcha)
- ❌ **Don't edit code, tests, configuration.md, cli.md, providers.md, or README.md.** This is Mode B docs-only
  on the ONE file; P1.M5.T1.S2 owns cross-doc consistency. (scope)
