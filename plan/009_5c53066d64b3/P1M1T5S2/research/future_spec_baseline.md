# Baseline evidence: FUTURE_SPEC.md vs PRD §9.24 (captured at PRP-authoring time)

**Purpose:** This note is the audit trail proving FUTURE_SPEC.md is **already consistent**
with the shipped lossless multi-turn behavior (PRD §9.24, FR-T1–T12) at the time
subtask **P1.M1.T5.S2** was scoped. The subtask is a **confirmation**, not an expected
edit (per `plan/009_5c53066d64b3/delta_prd.md` line 79: *"Confirm (not edit):
FUTURE_SPEC.md already carries the lossless-multi-turn-graduation note and the revised
chunking rejection (lossy only). Verify it is consistent with the shipped behavior; no
edit expected."*).

The implementing agent should re-run these exact checks (they are reproduced as the
PRP's Level 1 validation gate) and compare against the expected values below.

---

## The single relevant line — FUTURE_SPEC.md:99

Located in **§3. Rejected — deliberate, with reasons** (the only place chunking or
multi-turn is mentioned in the entire file):

```
| **Large-diff chunking — lossy map-reduce form** (aicommits) | The *summarize-each-chunk-then-combine* flavor degrades message quality and is permanently rejected. NOTE: a **lossless** multi-turn priming form (full diff delivered across request-sized session turns) has graduated to the spec — see PRD §9.24 (FR-T1–T12). The rejection above applies only to the lossy form; the original premise ("agent contexts are 200k+; byte caps bound the payload") is withdrawn — a provider's per-request reliability ceiling can fall well below its advertised window, which is exactly what §9.24 addresses. |
```

---

## The three contract conditions — all PASS at baseline

### (a) Lossless multi-turn is NOT listed as a rejected/deferred idea ✅
- §3 table rows whose title cell names "multi-turn": **0** (only "chunking" appears, scoped to "lossy").
- §3 table rows whose title cell names "chunking": **1** — and its title is explicitly
  "**Large-diff chunking — lossy map-reduce form**" (the lossless variant is NOT the
  rejected subject).
- §1 "Deferred" subsection mentioning multi-turn/chunking: **0**.
- The lossless form appears in the row ONLY as a NOTE pointing to §9.24 ("graduated to
  the spec"), not as a deferred/rejected idea. This satisfies FUTURE_SPEC's own footer
  rule — *"an idea must live in exactly one of the two documents"* — because the
  lossless *spec* lives in PRD §9.24 while the lossy *rejection* lives here; the NOTE
  is a disambiguating pointer, not a second copy.

### (b) Lossy map-reduce chunk-summarize-combine form IS still rejected, with rationale ✅
- Row is in **§3 Rejected**.
- Rationale present verbatim: *"degrades message quality"* + *"permanently rejected"*.
- The rejected form is precisely scoped: *"The summarize-each-chunk-then-combine flavor"*
  (i.e. the lossy map-reduce / summarize-then-combine shape, not all chunking).

### (c) Language is consistent with the shipped §9.24 behavior ✅
Cross-walk of the NOTE's claims → §9.24 FRs (no contradictions; no mis-description):
| FUTURE_SPEC:99 claim | §9.24 source | Consistent? |
|---|---|---|
| "lossless multi-turn priming form" | FR-T2 ("Lossless — full diff, request-sized chunks… no truncation, no summarization") | ✅ |
| "full diff delivered across request-sized session turns" | FR-T2 (same captured payload, unmodified) + FR-T4 (N+1 turn protocol) + FR-T6 (session append) | ✅ |
| "graduated to the spec — see PRD §9.24 (FR-T1–T12)" | §9.24 exists at PRD.md:491; FR-T1–T12 are its requirements | ✅ (anchor verified) |
| "The rejection … applies only to the lossy form" | FR-T2 explicitly contrasts lossless multi-turn with the rejected lossy chunking | ✅ |
| original premise ("agent contexts are 200k+; byte caps bound the payload") withdrawn | §9.24 intro (169K-token request failed a 200K-window model; per-request ceiling ≠ window) | ✅ |

Note: the row does NOT re-state §9.24 internals (N+1, message-role-only FR-T10,
session_mode="append" gate FR-T1d/FR-T8). That is **correct**, not a gap — FUTURE_SPEC's
job is to describe the rejected idea + disambiguate, then point to §9.24 for the shipped
spec. Re-specifying those internals here would violate the footer's "exactly one
document" rule. The check is for *non-contradiction*, not for re-statement.

---

## PRD anchor cross-check
- `PRD.md:491` → `### 9.24 Multi-turn generation fallback (lossless large-diff priming) (P1, → G21)`
  (the heading FUTURE_SPEC:99 points at — verified present).

## git baseline
- `git diff --stat -- FUTURE_SPEC.md` → **empty** (no uncommitted changes; the file
  already reflects the post-§9.24 state). This is the expected end state of S2: the
  diff should remain empty (a confirmation), OR be a minimal single-row correction if
  the agent finds an inconsistency the baseline missed.

## Conclusion
**FUTURE_SPEC.md is consistent with PRD §9.24 as of PRP authoring.** Expected S2
outcome = confirmation note, no file edit. The PRP carries the exact verification
commands and a corrective-edit fallback (the canonical row text) for the unlikely case
the implementing agent's working tree diverges from this baseline.
