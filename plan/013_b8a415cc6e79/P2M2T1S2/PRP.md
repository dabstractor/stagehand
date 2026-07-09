name: "P2.M2.T1.S2 — Confirm PRD §12.5.1/§12.5.1.1/§22.1 reflect the agy re-verification"
description: |
  Read-only PRD verification subtask (Mode A). Confirm — by direct `grep`/`read`/`sed` inspection
  of `PRD.md` at HEAD — that the committed state carries the agy re-verification
  (commit `2f77bd0`, "Re-verify and fix agy manifest against v1.1.0", verified 2026-07-08 against
  agy v1.1.0) across all four contract items:
    (a) §12.5.1 (h3.58) shows the corrected agy manifest (print_flag="", model_flag="--model",
        bare_flags=["--mode","plan"], default_model="Gemini 3.5 Flash (Low)", prompt_delivery="stdin",
        experimental=true);
    (b) §12.5.1.1 (h4.0) marks status items 1–3 RESOLVED and item 4 OPEN (tooled/stager flags);
    (c) §22.1 risk table (h3.103) marks the agy non-TTY stdout drop (issue #76) as RESOLVED 2026-07-08;
    (d) §12.5.2 (qwen-code) notes that agy diverged from the gemini-cli lineage.
  Produce a one-line PASS/FAIL per item (a)–(d), each backed by the cited PRD line number(s).
  No file is edited — `PRD.md` is human-owned READ-ONLY; the PRD sections themselves are this
  subtask's documentation surface (Mode A). This is the PRD-half of the parity proof; the
  code-half (S1) runs in parallel.

---

## Goal

**Feature Goal**: Verify — by direct read-only inspection of `PRD.md` at HEAD — that the agy
re-verification (commit `2f77bd0`, dated 2026-07-08, against agy v1.1.0) is faithfully recorded
across all four contract surfaces: the §12.5.1 corrected manifest TOML, the §12.5.1.1 status list,
the §22.1 risk-table row, and the §12.5.2 qwen-code divergence note.

**Deliverable**: A 4-line verification verdict — one PASS/FAIL line per contract item (a)–(d) —
each backed by the cited `PRD.md` line number(s) (heading + the specific field/row lines). Plus
confirmation that the re-verification commit `2f77bd0` touched `PRD.md` and that HEAD is a
descendant (so the correction persisted). No `PRD.md` (or any file) is edited.

**Success Definition**: All four items (a)–(d) report PASS; the deliverable names the exact
`PRD.md` line for each checked field/row/state; the legitimate `gemini` tokens (model family
"Gemini 3.5 Flash" + lineage prose "diverged from the gemini-cli lineage") are explicitly
recognized as intentional, NOT drift. This closes the PRD-side of the P2.M2 parity proof; the
code-side (S1) and the docs-drift fixes (P2.M3) depend on a confirmed PRD.

## User Persona (if applicable)

**Target User**: The stagecoach maintainer / verifying engineer (and the orchestrator consuming
the verification result to mark P2.M2.T1 done).

**Use Case**: Lock in confidence that the *spec* (`PRD.md`) — not just the *code* (S1) — carries
the corrected agy flag surface and the dated 2026-07-08 re-verification, before the provider-lineup
correction milestone (P2) is declared done and before the Mode B docs-drift fixes (P2.M3) ship.

**User Journey**: Confirm the re-verification commit touched `PRD.md` → read each contract
surface by line range → emit one PASS/FAIL line per item (a)–(d), citing the PRD lines → report.

**Pain Points Addressed**: Proves the PRD no longer carries the stale agy manifest (bare `-p`,
`-m`, `--approval-mode default`) or the old "OPEN/PTY-shim" status that the 2026-07-08
re-verification superseded. The PRD is the spec the code (S1) must match and the docs
(`docs/providers.md`, `docs/README.md` in P2.M3) must mirror; confirming it is the foundation
for the rest of the milestone.

## Why

- **Spec integrity after a re-verification:** agy v1.1.0 **diverged** from the gemini-cli lineage
  it forked from: it dropped `--approval-mode`, made `-p`/`--print`/`--prompt` **value-taking**
  (a bare `-p` fails), and uses `--model` (not `-m`). The re-verification (commit `2f77bd0`,
  2026-07-08) rewrote the §12.5.1 manifest, flipped §12.5.1.1 items 1–3 to RESOLVED, flipped the
  §22.1 #76 risk row to RESOLVED, and added the divergence note to §12.5.2. This subtask confirms
  all four persisted to HEAD.
- **Parity foundation (spec ↔ code ↔ docs):** S1 (code-half, running in parallel) confirms the
  *binary* ships the manifest the *spec* describes. This subtask (PRD-half) confirms the *spec*
  itself is correct. The two together close P2.M2. P2.M3 then mirrors both into the Mode B docs.
- **No regressions between commit and HEAD:** HEAD (`a6dbf1c`) is several commits past the
  re-verification (`2f77bd0`); this verification catches any accidental revert or partial
  overwrite of the PRD state.
- **Honest risk tracking (§22):** the §22.1 table retains the #76 row for history but must mark
  it RESOLVED 2026-07-08 so readers know the PTY-shim workaround is no longer needed and agy's
  remaining `experimental` status is solely about §12.5.1.1 item 4 (stager flags), not stdout.

## What

A pure read-only verification of `PRD.md` at HEAD. For each contract item, confirm the stated
invariant holds against the committed text, then emit one PASS/FAIL line. The exact expected
state (all pre-confirmed at HEAD = `a6dbf1c`):

**(a) §12.5.1 (h3.58) — corrected agy manifest.** Heading at **PRD.md:947**; TOML block spans
**:951–972**; the block header comment at **:952** dates it (`# Antigravity CLI. --help +
end-to-end verified 2026-07-08 (agy v1.1.0).`). The block MUST contain, at the cited lines:
- `:957` `prompt_delivery = "stdin"`
- `:958` `print_flag = ""`  (NON-NIL empty — agy's `-p` is value-taking, so NO print flag)
- `:960` `model_flag = "--model"`  (NOT `-m`)
- `:961` `default_model = "Gemini 3.5 Flash (Low)"`  (display label verbatim)
- `:967` `bare_flags = ["--mode", "plan"]`  (NO `--approval-mode`)
- `:972` `experimental = true`  (pending only §12.5.1.1 item 4)
The rendered example at **:978** MUST read `agy --model "Gemini 3.5 Flash (Low)" --mode plan`
with the payload via stdin and NO `-p`.

**(b) §12.5.1.1 (h4.0) — status list.** Heading at **PRD.md:983** (`#### 12.5.1.1 Status (agy) —
verified 2026-07-08 against agy v1.1.0`). The five numbered items at **:985–989** MUST show:
- `:985` item 1 — **RESOLVED** — non-TTY stdout drop (issue #76)
- `:986` item 2 — **RESOLVED** — Model flag (`--model`)
- `:987` item 3 — **RESOLVED** — Prompt delivery + read-only mode (`--mode plan`)
- `:988` item 4 — **OPEN** — Tooled (stager) flags
- `:989` item 5 — (informational) Print-mode timeout
The trailing summary at **:991** MUST corroborate: "Items 1–3 are cleared; agy ships
`experimental = true` … solely pending item 4."

**(c) §22.1 risk table (h3.103) — #76 row.** Heading at **PRD.md:2135** (`### 22.1 Risks`). The
agy row at **:2146** MUST mark the non-TTY stdout drop (issue #76; §12.5.1) as
**RESOLVED 2026-07-08 (agy v1.1.0)** — "no longer reproduces; agy reads piped stdin and returns
stdout correctly." The mitigation cell must tie the remaining `experimental` status to
§12.5.1.1 item 4 (stager flags), NOT to stdout.

**(d) §12.5.2 (qwen-code) — divergence note.** Heading at **PRD.md:993**. The divergence note at
**:995** MUST state that agy (§12.5.1) **diverged** from the gemini-cli lineage in v1.1.0
(`--model`, value-taking `-p`, no `--approval-mode`), and that qwen-code does NOT match agy.
(Corroborated by the §12.5.1 prose at `:949` and §12.5.1.1 item 3 at `:987`.)

**CONTRACT NOTE — legitimate `gemini` tokens are NOT drift.** The word `gemini` correctly
persists throughout these sections as a **model family name** ("Gemini 3.5 Flash (Low/High)") and
as **lineage prose** ("Gemini-CLI successor", "superseded `gemini` (Gemini CLI)", "diverged from
the gemini-cli lineage", qwen-code "fork of Google's Gemini CLI"). These are intentional and
MUST remain. Drift would be: the OLD flag surface (`-p`, `-m`, `--approval-mode default`)
appearing as active manifest values, §12.5.1.1 items 1–3 still marked OPEN, or the §22.1 #76 row
still marked OPEN/High — NONE of which exists at HEAD.

**OUT-OF-SCOPE (do NOT act on — reported only):**
1. The *code* half of the parity proof (compiled-in manifest, `providers/agy.toml`, role defaults)
   is the sibling task P2.M2.T1.S1, running in parallel. Do not duplicate it; this subtask is
   strictly the `PRD.md` surface.
2. The Mode B docs-drift fixes (`docs/providers.md`, `docs/README.md` agy table rows) are P2.M3.
   This subtask does NOT touch those files.
3. `PRD.md` is human-owned READ-ONLY — never edit it. The PRD sections themselves ARE this
   subtask's documentation surface (Mode A).

### Success Criteria

- [ ] Item (a) PASS — §12.5.1 manifest block (PRD.md:951–972) has `print_flag=""`, `model_flag
      ="--model"`, `bare_flags=["--mode","plan"]`, `prompt_delivery="stdin"`,
      `default_model="Gemini 3.5 Flash (Low)"`, `experimental=true`; rendered cmd (:978) has no `-p`.
- [ ] Item (b) PASS — §12.5.1.1 (PRD.md:985–988) shows items 1–3 RESOLVED and item 4 OPEN.
- [ ] Item (c) PASS — §22.1 risk table (PRD.md:2146) marks the `agy` #76 row RESOLVED 2026-07-08.
- [ ] Item (d) PASS — §12.5.2 (PRD.md:995) notes agy diverged from the gemini-cli lineage.
- [ ] Baseline confirmed: re-verification commit `2f77bd0` touched `PRD.md`; HEAD is a descendant.
- [ ] 4-line PASS/FAIL verdict emitted (a)–(d), each citing its PRD line(s).
- [ ] Legitimate model-family / lineage `gemini` tokens classified and confirmed non-drift.
- [ ] No file (`PRD.md` or any other) is modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to verify this
successfully?_ **Yes** — every check is pinned to an exact `PRD.md` line range, a copy-pasteable
grep/sed command, and the exact expected literal. No inference, no live `agy` binary, and no
external docs required (the flag surface was already verified live on 2026-07-08; this subtask
confirms the *recorded* PRD state matches that verification).

### Documentation & References

```yaml
# MUST READ — the single file under verification (READ-ONLY; human-owned — NEVER edit from this subtask)
- file: PRD.md
  why: The spec of record. §12.5.1 is the corrected agy manifest TOML; §12.5.1.1 is the verified
       status list; §22.1 is the risks table; §12.5.2 is the qwen-code provider note. These four
       surfaces ARE this subtask's documentation surface (Mode A) — confirming they carry the
       2026-07-08 re-verification is the deliverable.
  section: "§12.5.1 (h3.58, :947), §12.5.1.1 (h4.0, :983), §22.1 (h3.103, :2135), §12.5.2 (h3.59, :993)"
  gotcha: PRD.md is a LARGE markdown file (~2500 lines). Use anchored `grep`/`sed -n '<n>p'` against
          the cited absolute line numbers — do NOT eyeball the whole file. The manifest TOML block
          in §12.5.1 (:951–972) is distinct from the generic §12.1 schema example block (~:735–772)
          that shows the SAME field names with placeholder values; verify the AGY block, not the
          schema example.

# Baseline — the re-verification commit (read-only git history)
- file: (git history) commit 2f77bd0
  why: "Re-verify and fix agy manifest against v1.1.0" — the commit that rewrote the four PRD
       surfaces. `git show --stat 2f77bd0` confirms it touched PRD.md (54 lines changed across 4
       files). HEAD `a6dbf1c` is a descendant of `bb3cb3b`, itself a descendant of `2f77bd0`.

# Sibling task — the CODE half of this parity proof (CONTRACT; defines the binary that must match)
- docfile: plan/013_b8a415cc6e79/P2M2T1S1/PRP.md
  why: S1 verifies the compiled-in agy manifest (`internal/provider/builtin.go builtinAgy()`,
       `providers/agy.toml`, `internal/config/role_defaults.go`) matches PRD §12.5.1 byte-for-byte.
       This subtask (S2) is the PRD-half: it confirms the SPEC those code values are matched
       against is itself correct. The two close P2.M2 together. Do not duplicate S1's code checks.

# Prior PRD-side sibling (gemini removal) — context for the gemini→agy succession story
- docfile: plan/013_b8a415cc6e79/P2M1T1S2/PRP.md
  why: Established the PRD-side gemini-removal / agy-successor story is internally consistent.
       §12.5 carries the REMOVED stub for gemini; §12.5.1/§12.5.1.1 (this subtask) carry the agy
       successor's corrected, re-verified manifest. Together they form the provider-lineup fix.

# Per-item evidence captured at HEAD (this subtask's research output)
- docfile: plan/013_b8a415cc6e79/P2M2T1S2/research/per_item_evidence.md
  why: The exact observed-vs-expected table per item (a)–(d), with absolute PRD line numbers,
       captured by direct inspection at HEAD `a6dbf1c` on 2026-07-09. This PRP is the runbook;
       the evidence file is the pre-filled witness. An independent re-run should reproduce it.
```

### Current Codebase tree (relevant slice)

```bash
# Run from repo root: cd /home/dustin/projects/stagecoach
PRD.md                                  # ← the ONLY file under verification (READ-ONLY; ~2500 lines)
plan/013_b8a415cc6e79/
  P2M1T1S2/PRP.md                       # sibling: PRD-side gemini-removal verification (context)
  P2M2T1S1/
    PRP.md                              # sibling: CODE-side agy manifest verification (CONTRACT)
  P2M2T1S2/
    PRP.md                              # ← THIS file
    research/per_item_evidence.md       # per-item PRD evidence table (captured at HEAD)
  architecture/code_gemini_agy_audit.md # read-only audit (context; the code-side pre-confirmation)
# NOTE: no source file is touched by this subtask.
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
# NONE. This is a read-only PRD verification subtask. No files are created, modified, or deleted.
# The only artifacts are (1) this PRP, (2) research/per_item_evidence.md, and (3) the 4-line
# PASS/FAIL verdict reported back. PRD.md is inspected, never edited (Mode A).
```

### Known Gotchas of our codebase & Library Quirks

```text
# GOTCHA 1 — TWO manifest TOML blocks share the same field names; verify the AGY one.
#   PRD §12.1 contains a generic manifest-schema EXAMPLE block (≈PRD.md:735–772) showing the same
#   field names (print_flag, model_flag, bare_flags, …) with PLACEHOLDER values (e.g. print_flag
#   = "-p", bare_flags = [ … ]) to illustrate the schema. The §12.5.1 AGY block (PRD.md:951–972)
#   is the AUTHORITATIVE provider manifest under verification. When grepping for field values,
#   anchor to the §12.5.1 range (line ≥947, the "### 12.5.1 Built-in provider: Antigravity CLI"
#   heading) to avoid matching the schema example by mistake.

# GOTCHA 2 — print_flag="" is an EXPLICIT NON-NIL empty, by design.
#   agy v1.1.0's -p is value-taking, so a bare -p fails ("flag needs an argument: -p"); agy reads
#   stdin without -p. The PRD therefore records print_flag = "" (an explicit override to "no print
#   flag"), NOT print_flag = "-p" and NOT a missing line. The §12.5.1 comment at :958 explicitly
#   says "NON-NIL empty". This mirrors the code's strPtr("") design (see S1, manifest.go).

# GOTCHA 3 — "gemini" tokens in these sections are LEGITIMATE (model family + lineage), NOT drift.
#   agy RUNS the Gemini model family ("Gemini 3.5 Flash (Low/High/Medium)"), DESCENDS from the
#   gemini-cli lineage ("Gemini-CLI successor", "superseded `gemini` (Gemini CLI)", "diverged from
#   the gemini-cli lineage"), and qwen-code is a "fork of Google's Gemini CLI". These tokens are
#   CORRECT and MUST remain. Drift = the OLD flag surface (-p/-m/--approval-mode) as active values,
#   or items 1–3 / the #76 row still OPEN. None of that exists at HEAD.

# GOTCHA 4 — §22.1 is a WIDE markdown table; one agy row, many columns.
#   The risks table (heading :2135) has very wide columns. The agy #76 row is at :2146. When
#   grepping, match the row by its lead cell text ("`agy` non-TTY stdout drop (issue #76") AND by
#   the resolution token ("RESOLVED 2026-07-08") — both must appear in that one row. Do not confuse
#   the §22.1 row with the §12.5.1.1 item-1 line (:985) or the Appendix-E open-question line (:2296),
#   which also mention #76 (those are history/list entries, not the risk-table row).

# GOTCHA 5 — the dated heading is the status section's anchor.
#   §12.5.1.1's heading (:983) itself carries the date — "verified 2026-07-08 against agy v1.1.0".
#   That heading date is part of the re-verification record (it was rewritten in 2f77bd0). Confirm
#   it reads 2026-07-08, not an older date.
```

## Implementation Blueprint

### Verification approach (read-only — no implementation surface)

There are no data models to create. The "tasks" below are verification steps. Each step has an
exact command and an exact expected result; a step PASSES iff the observed result equals the
expected result. Emit the one-line PASS/FAIL verdict for the corresponding contract item. All
steps run from the repo root (`/home/dustin/projects/stagecoach`).

### Verification Tasks (ordered; each maps to a contract item)

```yaml
Task V0: CONFIRM baseline — the re-verification is committed and touched PRD.md
  - RUN: git log --oneline -1 2f77bd0
  - EXPECT: `2f77bd0 Re-verify and fix agy manifest against v1.1.0`.
  - RUN: git show --stat 2f77bd0 | grep PRD.md
  - EXPECT: a `PRD.md` line (the commit rewrote the four PRD surfaces).
  - RUN: git merge-base --is-ancestor 2f77bd0 HEAD && echo DESCENDANT
  - EXPECT: `DESCENDANT` (HEAD a6dbf1c is a descendant of 2f77bd0 → correction persisted).
  - VERDICT: baseline OK iff all three hold. If 2f77bd0 is absent or NOT an ancestor, the expected
            state is not present at HEAD — report immediately.

Task V1 → item (a): CONFIRM §12.5.1 carries the corrected agy manifest (PRD.md:947–978)
  - RUN: sed -n '947p' PRD.md                       # heading: "### 12.5.1 Built-in provider: Antigravity CLI (`agy`)"
  - RUN: sed -n '951,972p' PRD.md                    # the full manifest TOML block
  - RUN (pinpoint): sed -n '952p;957p;958p;960p;961p;967p;972p;978p' PRD.md
  - EXPECT:
      :952 = `# Antigravity CLI. --help + end-to-end verified 2026-07-08 (agy v1.1.0).`
      :957 = `prompt_delivery = "stdin" …`
      :958 = `print_flag = "" …`            (NON-NIL empty; comment notes -p is value-taking)
      :960 = `model_flag = "--model" …`     (comment: `-m` rejected)
      :961 = `default_model = "Gemini 3.5 Flash (Low)" …`
      :967 = `bare_flags = ["--mode", "plan"]`
      :972 = `experimental = true …`
      :978 = `agy --model "Gemini 3.5 Flash (Low)" --mode plan   < <sys+user payload via stdin>`
  - RUN (negative — old surface must NOT be active in the agy block):
      awk 'NR>=947 && NR<983' PRD.md | grep -nE 'print_flag *= *"-p"|model_flag *= *"-m"|approval-mode *=|bare_flags.*approval-mode'
      EXPECT: no match (the only `--approval-mode` mentions must be PROSE noting its removal, never
              a manifest field value).
  - VERDICT: item (a) PASS iff all pinpoint lines match AND the negative grep is clean.

Task V2 → item (b): CONFIRM §12.5.1.1 marks items 1–3 RESOLVED, item 4 OPEN (PRD.md:983–991)
  - RUN: sed -n '983p' PRD.md                       # heading: "#### 12.5.1.1 Status (agy) — verified 2026-07-08 against agy v1.1.0"
  - RUN: sed -n '985,989p' PRD.md                    # the five numbered status items
  - RUN: sed -n '991p' PRD.md                        # trailing summary
  - EXPECT:
      :983 heading ends with `verified 2026-07-08 against agy v1.1.0`
      :985 item 1 begins `1. **RESOLVED — non-TTY stdout drop (issue [#76]…`
      :986 item 2 begins `2. **RESOLVED — Model flag:**`
      :987 item 3 begins `3. **RESOLVED — Prompt delivery + read-only mode:**`
      :988 item 4 begins `4. **OPEN — Tooled (stager) flags:**`
      :991 summary: `Items 1–3 are cleared; agy ships `experimental = true` … solely pending item 4.`
  - RUN (negative): awk 'NR==985 || NR==986 || NR==987' PRD.md | grep -c RESOLVED  → expect 3
                    awk 'NR==988' PRD.md | grep -c '\*\*OPEN —'                     → expect 1
  - VERDICT: item (b) PASS iff items 1–3 are RESOLVED, item 4 is OPEN, and the heading/summary agree.

Task V3 → item (c): CONFIRM §22.1 risk table marks the #76 row RESOLVED 2026-07-08 (PRD.md:2135–2146)
  - RUN: sed -n '2135p' PRD.md                       # heading: "### 22.1 Risks"
  - RUN: sed -n '2146p' PRD.md                       # the agy #76 risk row
  - EXPECT:
      :2146 row lead cell contains: `**\`agy\` non-TTY stdout drop (issue #76; §12.5.1)**` AND
              the resolution token `**RESOLVED 2026-07-08 (agy v1.1.0):**`.
  - RUN (cross-check the whole table for the #76 row, in case line numbers shifted):
      awk '/^### 22.1 Risks/{f=1} f' PRD.md | grep -n 'non-TTY stdout drop' | head -1
      EXPECT: a single row whose line contains both `#76` and `RESOLVED 2026-07-08`.
  - RUN (negative): the row must NOT say OPEN / must NOT lack a resolution:
      sed -n '2146p' PRD.md | grep -qi 'RESOLVED 2026-07-08' && echo RESOLVED_OK || echo FAIL
  - VERDICT: item (c) PASS iff the #76 row is present in §22.1 and marked RESOLVED 2026-07-08.

Task V4 → item (d): CONFIRM §12.5.2 notes agy diverged from the gemini-cli lineage (PRD.md:993–995)
  - RUN: sed -n '993p' PRD.md                       # heading: "### 12.5.2 Built-in provider: qwen-code … (a Gemini-CLI fork)"
  - RUN: sed -n '995p' PRD.md                        # the divergence note
  - EXPECT: :995 contains `agy` + `diverged` + a reference to the v1.1.0 differences
            (`--model`, value-taking `-p`, no `--approval-mode`), and explicitly says qwen-code does
            NOT match agy.
  - RUN (cross-check — divergence is also stated in §12.5.1 prose and §12.5.1.1 item 3):
      grep -nE 'diverged? from (this|the) gemini-cli lineage' PRD.md
      EXPECT: matches at ~:949 (§12.5.1 prose) and ~:995 (§12.5.2 note); also `:987` (§12.5.1.1 item 3).
  - VERDICT: item (d) PASS iff §12.5.2 (:995) states agy diverged from the gemini-cli lineage.

Task V5: CLASSIFICATION sweep — legitimate gemini tokens, no active old-flag drift
  - RUN: awk 'NR>=947 && NR<=995' PRD.md | grep -ni 'gemini'
  - EXPECT: only legitimate tokens — model-family labels ("Gemini 3.5 Flash"), lineage prose
            ("Gemini-CLI successor", "superseded `gemini`", "diverged from the gemini-cli lineage",
            qwen-code "fork of Google's Gemini CLI"). NONE is the old flag surface as an active value.
  - VERDICT: informational — confirms the gemini tokens are intentional (Gotcha 3).
```

### Escalation procedure (only if an item UNEXPECTEDLY fails)

All four items were pre-confirmed PASS at HEAD (`a6dbf1c`) in `research/per_item_evidence.md`.
A failure means the committed state regressed, the re-verification commit (`2f77bd0`) was
partially reverted, OR a later edit overwrote the PRD section. Because **`PRD.md` is not edited
from this read-only subtask** (the PRD sections ARE the documentation surface, Mode A), a failure
is REPORTED, not patched:

```text
1. Re-run the failing item's grep/sed on a fresh checkout of HEAD to rule out a stale read.
2. If it still fails, inspect the re-verification diff and any later PRD edits:
     git show 2f77bd0 -- PRD.md                      # the re-verification change
     git log --oneline -- PRD.md | head -10          # later PRD edits that may have reverted it
   and locate where the expected state was lost.
3. REPORT the failing item, the observed vs expected literal, and the PRD line — do NOT fix it here.
   Editing PRD.md is a human decision (it is the product spec); a correction requires re-running
   the live agy v1.1.0 verification and re-committing.
```

### Integration Points

```yaml
# NONE. Read-only PRD verification — no DATABASE, CONFIG, ROUTES, or code integration.
# The only "integration" is consuming the verification result in the P2 milestone reporting and
# confirming parity with the CODE half (sibling P2.M2.T1.S1) before the provider-lineup correction
# is declared done, and unblocking the Mode B docs-drift fixes (P2.M3) which mirror the PRD.
```

## Validation Loop

### Level 1: Baseline (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
git log --oneline -1 2f77bd0                      # EXPECT: 2f77bd0 Re-verify and fix agy manifest against v1.1.0
git show --stat 2f77bd0 | grep PRD.md             # EXPECT: PRD.md present in the change set
git merge-base --is-ancestor 2f77bd0 HEAD && echo DESCENDANT   # EXPECT: DESCENDANT

# Expected: all three confirm the re-verification is committed, touched PRD.md, and persists to HEAD.
```

### Level 2: Per-Item Verification (the core gate)

```bash
cd /home/dustin/projects/stagecoach
# (a) §12.5.1 corrected agy manifest:
sed -n '947p' PRD.md                                  # heading
sed -n '952p;957p;958p;960p;961p;967p;972p;978p' PRD.md   # dated header + fields + rendered cmd
# (b) §12.5.1.1 status list:
sed -n '983p;985p;986p;987p;988p;991p' PRD.md         # heading + items 1-4 + summary
# (c) §22.1 risk table #76 row:
sed -n '2135p;2146p' PRD.md                           # heading + agy #76 row
# (d) §12.5.2 divergence note:
sed -n '993p;995p' PRD.md                             # heading + divergence note

# Expected: each command shows the exact expected literal cited in Tasks V1-V4.
```

### Level 3: Cross-Reference Sweep (System Validation)

```bash
cd /home/dustin/projects/stagecoach
# (1) No OLD flag surface as ACTIVE values in the §12.5.1 agy block (divergence markers):
awk 'NR>=947 && NR<983' PRD.md | grep -nE 'print_flag *= *"-p"|model_flag *= *"-m"|bare_flags *= *\[[^]]*approval-mode'
# EXPECT: no match. Prose mentioning --approval-mode's removal is fine; an active field value is drift.

# (2) §22.1 #76 row is RESOLVED 2026-07-08 (and the #76 references elsewhere are consistent history):
sed -n '2146p' PRD.md | grep -qi 'RESOLVED 2026-07-08' && echo "c:RESOLVED_OK" || echo "c:FAIL"
grep -n '#76' PRD.md                                   # EXPECT: :985 (RESOLVED item), :2146 (RESOLVED risk), :2296/:2321 (historical)

# (3) Divergence is stated in all the expected places:
grep -nE 'diverged? from (this|the) gemini-cli lineage' PRD.md   # EXPECT: ~:949, ~:995 (and :987 wording)

# (4) Legitimate gemini tokens only — model family + lineage (no provider drift):
awk 'NR>=947 && NR<=995' PRD.md | grep -ni 'gemini' | head -20
# EXPECT: model-family labels and lineage prose only (Gotcha 3).

# Expected: (1) clean; (2) c:RESOLVED_OK; (3) ≥2 hits; (4) legitimate tokens only.
```

### Level 4: Domain-Specific Validation (consistency with the parity proof)

```bash
cd /home/dustin/projects/stagecoach
# The PRD §12.5.1 manifest is the spec the CODE (S1) must match. Cross-check a few values against
# the compiled manifest to confirm the spec is internally the same as the binary (PARITY, not drift):
sed -n '958p;960p;967p' PRD.md                                   # PRD: print_flag/model_flag/bare_flags
grep -nE 'PrintFlag:|ModelFlag:|BareFlags:' internal/provider/builtin.go | grep -i agy -A2 -B2
# EXPECT: PRD print_flag="" ↔ code PrintFlag=strPtr(""); PRD model_flag="--model" ↔ code ModelFlag=
#         strPtr("--model"); PRD bare_flags=["--mode","plan"] ↔ code BareFlags=[]string{"--mode",
#         "plan"}. (This is a cross-reference sanity check; the authoritative code verification is
#         S1's job — here we only confirm the PRD half is self-consistent and matches the binary.)
# Expected: PRD values agree with the compiled manifest fields (parity confirmed).
```

## Final Validation Checklist

### Technical Validation

- [ ] Baseline confirmed: re-verification commit `2f77bd0` present, touched `PRD.md`, HEAD is a
      descendant (Level 1 all pass).

### Feature (Verification) Validation

- [ ] Item (a) PASS — §12.5.1 manifest block (PRD.md:951–972) has `print_flag=""`, `model_flag
      ="--model"`, `bare_flags=["--mode","plan"]`, `prompt_delivery="stdin"`,
      `default_model="Gemini 3.5 Flash (Low)"`, `experimental=true`; rendered cmd (:978) has no `-p`.
- [ ] Item (b) PASS — §12.5.1.1 (PRD.md:985–988) shows items 1–3 RESOLVED and item 4 OPEN; heading
      (:983) + summary (:991) dated/corroborated.
- [ ] Item (c) PASS — §22.1 risk table (PRD.md:2146) marks the `agy` #76 row RESOLVED 2026-07-08.
- [ ] Item (d) PASS — §12.5.2 (PRD.md:995) notes agy diverged from the gemini-cli lineage.
- [ ] 4-line PASS/FAIL verdict emitted (a)–(d), each citing its PRD line(s).
- [ ] Legitimate model-family / lineage `gemini` tokens classified and confirmed non-drift.

### Code Quality Validation

- [ ] No file (`PRD.md` or any other) modified.
- [ ] No `PRD.md` / `tasks.json` / `prd_snapshot.md` / `.gitignore` / source file touched.
- [ ] Out-of-scope items (the code-half S1; the docs-half P2.M3; live agy re-verification) reported,
      NOT acted on.

### Documentation & Deployment

- [ ] Per contract §5 (Mode A): the PRD sections themselves ARE the documentation surface — confirming
      §12.5.1 / §12.5.1.1 / §22.1 / §12.5.2 carry the 2026-07-08 re-verification IS the deliverable;
      no additional docs artifact is required beyond the 4-line verdict.
- [ ] Verification result recorded for the P2 milestone (the code-half S1 and the docs-drift P2.M3
      depend on a confirmed PRD-side agy re-verification).

---

## Anti-Patterns to Avoid

- ❌ Don't edit `PRD.md` — it is human-owned READ-ONLY and the product spec. A found regression is
  REPORTED, not fixed, from this read-only subtask (the PRD sections ARE the Mode A documentation
  surface; changing them requires re-running the live agy v1.1.0 verification and re-committing).
- ❌ Don't match the §12.1 generic manifest-schema EXAMPLE block (~PRD.md:735–772) by mistake — it
  uses the same field names with placeholder values (`print_flag = "-p"`). Anchor every check to the
  §12.5.1 AGY block (heading :947, lines :951–972). See Gotcha 1.
- ❌ Don't treat `print_flag = ""` as a missing field — it is an EXPLICIT non-nil empty, by design
  (agy's `-p` is value-taking; a bare `-p` fails). The §12.5.1 comment says "NON-NIL empty". See
  Gotcha 2.
- ❌ Don't treat model-family `gemini` tokens ("Gemini 3.5 Flash") or lineage prose ("Gemini-CLI
  successor", "diverged from the gemini-cli lineage", qwen-code "fork of Google's Gemini CLI") as
  drift — agy runs the Gemini family and descends from gemini-cli; those tokens are correct and
  MUST remain. See Gotcha 3.
- ❌ Don't confuse the §22.1 #76 *risk-table row* (:2146) with the §12.5.1.1 item-1 line (:985) or
  the Appendix-E open-question (:2296) — all mention #76, but only the §22.1 row is item (c)'s
  target. See Gotcha 4.
- ❌ Don't duplicate the code-half work (S1: `builtin.go`, `providers/agy.toml`, `role_defaults.go`)
  or the docs-drift work (P2.M3: `docs/providers.md`, `docs/README.md`) — this subtask is strictly
  the `PRD.md` surface.
- ❌ Don't re-run a live `agy --help` / end-to-end invocation — the flag surface was already verified
  live on 2026-07-08 (commit 2f77bd0); this subtask confirms the *recorded* PRD state matches that
  verification, not the live binary.

---

## Confidence Score

**One-pass success likelihood: 10/10.** This is a read-only verification of `PRD.md` at a
known-good committed state (`2f77bd0` re-verification, confirmed present and an ancestor of HEAD
`a6dbf1c`). Every check is pinned to an exact field value + an observed absolute PRD line,
pre-confirmed in `research/per_item_evidence.md`, and corroborated by consistency sweeps (no
old-flag-surface active; #76 references consistent; divergence stated in the expected places).
The deliverable is a deterministic 4-line PASS/FAIL verdict (a)–(d); there is no implementation
surface to get wrong, and the single hardest nuance (legitimate model-family/lineage `gemini`
tokens vs the §12.1 schema example vs provider drift) is spelled out with a full classification
guide (Gotchas 1–4).
