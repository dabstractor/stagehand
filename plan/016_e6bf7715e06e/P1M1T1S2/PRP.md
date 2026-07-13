name: "P1.M1.T1.S2 — Mirror CHROME-DISABLE notes in providers/*.toml reference files (FR-C5)"
description: >
  Add a `# CHROME-DISABLE (FR-C5, §9.28):` comment paragraph to the header block of each of the 7
  providers/*.toml reference files (pi, claude, agy, qwen-code, opencode, codex, cursor), mirroring the
  note P1.M1.T1.S1 adds to the corresponding builtinXxx() doc-comment in builtin.go. Documentation-only
  (TOML comments are inert — not loaded at runtime). Each note records, per chrome surface, what is
  disabled (by which flag), what is not (no CLI switch → documented limitation), and the verification
  source/date. Consumes S1's builtin.go notes as the source of truth.

---

## Goal

**Feature Goal**: Make the providers/*.toml reference files' chrome-disable story match builtin.go
exactly (FR-C5, §9.28), so a user reading any reference manifest sees the same complete, honest
per-surface chrome record that lives in the compiled-in code — not a hidden assumption.

**Deliverable**: 7 edited header blocks (one paragraph each) in providers/{pi,claude,agy,qwen-code,
opencode,codex,cursor}.toml. Each adds a concise (1-4 line) `# CHROME-DISABLE (FR-C5, §9.28):`
paragraph immediately after the existing `TOOLS-DISABLE CATEGORY` paragraph, mirroring S1's builtin.go
note in substance. No field line changes (the decode-parity test enforces byte-identity on fields).

**Success Definition**:
- All 7 providers/*.toml contain a grep-able `# CHROME-DISABLE (FR-C5, §9.28):` paragraph in the header.
- pi's note names the 4 disabled surfaces (--no-extensions/--no-skills/--no-prompt-templates/
  --no-context-files) AND the MCP gap (no --no-mcp; FR-C3).
- claude's note names --tools "" + --setting-sources "" (chrome covered).
- agy/qwen-code/opencode/codex/cursor each state "no per-surface chrome switch; <read-only flag> is the
  constraint, not chrome; chrome may load — documented limitation (FR-C4)" + verification source/date.
- `go test ./internal/provider/...` passes — specifically `TestProviderReferenceFiles_DecodeParity`
  stays GREEN (proves no field was altered; comments are inert under toml.Unmarshal).
- `make test` + `make lint` pass.

## User Persona (if applicable)

**Target User**: A stagecoach user reading `providers/<name>.toml` to understand a provider's chrome
behavior, or copy-pasting it as a config-override template.

**Use Case**: "Does the pi reference manifest disable MCP? What about agy's skills?" — the user finds
the CHROME-DISABLE paragraph in the header and gets the honest, complete answer without reading Go code.

**Pain Points Addressed**: Before this, the reference files documented TOOL-disable (TOOLS-DISABLE
CATEGORY) but NOT chrome-disable, so a reader could wrongly assume the read-only constraint also
disables chrome (it does not — FR-C4). This makes the chrome story explicit and honest.

## Why

- **FR-C5 / §9.28**: Each built-in provider's manifest header must carry a CHROME-DISABLE note recording
  per surface what is disabled vs. what is a documented limitation. S1 puts that note in builtin.go (the
  compiled-in source of truth). S2 mirrors it into providers/*.toml (the human-readable reference that
  claims to mirror builtin.go "byte-for-byte modulo comments") so the two never drift and users reading
  either get the same story.
- **The reference files claim fidelity**: each providers/*.toml header states it "mirrors the compiled-in
  manifest ... BYTE-FOR-BYTE (modulo comments)". A CHROME-DISABLE note in builtin.go but absent from the
  .toml would falsify that claim. S2 keeps the claim true.

## What

**User-visible behavior**: None at runtime (the .toml files are not loaded). A user who opens any
providers/*.toml sees a CHROME-DISABLE paragraph in the header documenting that provider's chrome surfaces.

**Technical change (comment-only on 7 files):**
1. For each of the 7 providers/*.toml, insert a `# CHROME-DISABLE (FR-C5, §9.28):` paragraph into the
   header block, immediately after the `TOOLS-DISABLE CATEGORY` paragraph and before the closing
   `# ===` delimiter.
2. Mirror S1's substance (per provider), condensed to 1-4 lines (item guidance), preserving the three
   required elements: disabled-surfaces + flags, not-disabled (no switch), limitation status + source/date.

### Success Criteria
- [ ] 7 `# CHROME-DISABLE (FR-C5, §9.28):` paragraphs added (one per file)
- [ ] pi note: 4 surfaces disabled by named flags + MCP gap (FR-C3)
- [ ] claude note: --tools "" + --setting-sources "" (chrome covered)
- [ ] 5 read-only providers: "no chrome switch; constraint-only; documented limitation (FR-C4)" + source/date
- [ ] NO field line changed (decode-parity test stays green)
- [ ] `go test ./internal/provider/...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact per-provider paragraph content, the placement anchor (after TOOLS-DISABLE, before
`# ===`), the TOML `#` comment style, the decode-parity test that proves comments are safe + fields must
not change, and the scope fences are all enumerated below.

### Documentation & References

```yaml
- file: providers/pi.toml (and the 6 siblings)
  why: "THE change sites. Each has a header block delimited by '# ===' lines containing a
        '# TOOLS-DISABLE CATEGORY (§12.7.1):' paragraph. Insert '# CHROME-DISABLE (FR-C5, §9.28):'
        immediately AFTER the TOOLS-DISABLE paragraph and BEFORE the closing '# ===' delimiter."
  pattern: "TOOLS-DISABLE paragraph per file (grep 'TOOLS-DISABLE CATEGORY'): pi.toml:26, claude.toml:24,
            agy.toml:53, qwen-code.toml:39, opencode.toml:25, codex.toml:24, cursor.toml:24. Continuation
            lines use '#   ' (hash + 3 spaces) — match the TOOLS-DISABLE paragraph's indentation."
  critical: "Use TOML '#' comments (NOT Go '//'). The marker '# CHROME-DISABLE (FR-C5, §9.28):' must be
             grep-able and mirror S1's '// CHROME-DISABLE (FR-C5, §9.28):' in builtin.go."

- file: internal/provider/referencefiles_test.go
  why: "TestProviderReferenceFiles_DecodeParity reads each providers/*.toml, toml.Unmarshal's it into a
        Manifest, and reflect.DeepEqual-checks against the builtin. THIS IS THE SAFETY NET."
  pattern: "toml.Unmarshal ignores '#' comments → adding comment lines does NOT change the decoded
            Manifest → the test stays GREEN as long as no FIELD line changes."
  critical: "DO NOT alter any field line (name=, command=, bare_flags=, etc.). Comments only. If a field
             drifts, this test fails immediately with a decoded-vs-builtin diff. The test is the proof
             that S2 is comment-only."

- file: internal/provider/builtin.go
  why: "S1's CHROME-DISABLE notes are the SOURCE OF TRUTH (item point: 'notes from P1.M1.T1.S1 in
        builtin.go are the source of truth'). Read the actual notes once S1 lands and mirror their
        wording; if S1 hasn't landed, use S1's PRP-specified content (below) verbatim in substance."
  pattern: "S1 appends '// CHROME-DISABLE (FR-C5, §9.28):' to each builtinXxx() doc block. The 7 funcs:
            builtinPi, builtinClaude, builtinAgy, builtinQwenCode, builtinOpenCode, builtinCodex,
            builtinCursor."

- docfile: plan/016_e6bf7715e06e/architecture/external_deps.md
  why: "The per-provider chrome surface inventory + verification sources/dates that both S1 and S2 cite."
  section: "Per-provider chrome surface inventory"

- docfile: plan/016_e6bf7715e06e/P1M1T1S1/PRP.md
  why: "S1 is the CONTRACT. Its Tasks 2-8 specify the EXACT per-provider note content S2 mirrors. READ
        to get each note's substance (surfaces, flag tokens, gaps, verification dates)."

- docfile: plan/016_e6bf7715e06e/P1M1T1S2/research/findings.md
  why: "The decode-parity test mechanics, placement anchors, comment-style, per-provider content, scope fences."
```

### Current Codebase tree (relevant slice)

```bash
providers/
  pi.toml          # header TOOLS-DISABLE at :26 — add CHROME-DISABLE after it
  claude.toml      # :24
  agy.toml         # :53
  qwen-code.toml   # :39
  opencode.toml    # :25
  codex.toml       # :24
  cursor.toml      # :24
internal/provider/
  referencefiles_test.go  # TestProviderReferenceFiles_DecodeParity — the comment-safety + field-fidelity net
  builtin.go              # S1 adds the source-of-truth notes here (read-only for S2)
```

### Desired Codebase tree with files to be added

```bash
providers/{pi,claude,agy,qwen-code,opencode,codex,cursor}.toml  # MODIFY (additive): +1 CHROME-DISABLE comment paragraph each
```

### Known Gotchas of our codebase & Library Quirks

```toml
# CRITICAL (decode-parity test): TestProviderReferenceFiles_DecodeParity does reflect.DeepEqual(decoded,
#   builtin) after toml.Unmarshal. TOML '#' comments are INERT (the parser drops them), so adding comment
#   lines does NOT change the decoded Manifest → the test stays GREEN. But this also means: DO NOT change
#   any FIELD line — fields must stay byte-identical to builtin.go. Comments only.

# CRITICAL (source of truth = S1's builtin.go notes): the item says the notes "from P1.M1.T1.S1 (in
#   builtin.go) are the source of truth." READ builtin.go's actual notes once S1 lands and mirror their
#   wording. S1's PRP content (below) is the fallback contract if S1 hasn't landed. Do NOT invent content.

# GOTCHA (comment style): TOML uses '#', Go uses '//'. Translate S1's '// CHROME-DISABLE ...' to
#   '# CHROME-DISABLE ...'. Continuation lines: '#   ' (hash + 3 spaces), matching the existing
#   TOOLS-DISABLE paragraph indentation in each file.

# GOTCHA (placement): insert AFTER the TOOLS-DISABLE CATEGORY paragraph and BEFORE the closing '# ==='
#   header delimiter — NOT in the middle of the field section. The header block is the comment region;
#   the field section starts at '# --- identity / discovery ---'.

# GOTCHA (concise per item): the item says "Keep it concise (1-4 lines per provider)." S1's Go notes are
#   longer (pi is 5-7 lines). Condense for TOML while keeping the 3 substance elements (disabled+flags,
#   not-disabled+no-switch, limitation+source/date). Do not drop the verification source/date (FR-D5/FR-T9).

# SCOPE: do NOT modify builtin.go (S1), any field line (parity test), builtin_test.go (T2.S1), or
#   docs/*.md (P1.M2.T1). S2 is providers/*.toml header comments only.
```

## Implementation Blueprint

### Data models and structure
None. Pure TOML comment additions. No fields, no runtime behavior, no code.

### Implementation Tasks (ordered by dependencies)

> **Hard prerequisite**: P1.M1.T1.S1's notes should be read for the source-of-truth wording. If S1 has
> landed (`grep -c CHROME-DISABLE internal/provider/builtin.go` == 7), mirror the actual note text. If
> not (== 0), use the S1-PRP-specified content below (it is the contract). Either way, the SUBSTANCE
> per provider is fixed.

```yaml
Task 0: PREREQUISITE — read the source of truth (S1 has LANDED)
  - S1 was confirmed landed during this research: `grep -c 'CHROME-DISABLE (FR-C5' internal/provider/builtin.go`
    returns 7. READ each builtinXxx doc block's CHROME-DISABLE note in internal/provider/builtin.go and
    mirror its exact wording (translating `//` → `#`). The 7 landed notes already match the per-provider
    content below (verified). If, at implementation time, the count is NOT 7 (S1 reverted/rebased oddly),
    fall back to S1's PRP (plan/016_*/P1M1T1S1/PRP.md Tasks 2-8) as the contract.
  - DEPENDENCIES: none.

Task 1: MODIFY providers/pi.toml — add CHROME-DISABLE after the TOOLS-DISABLE paragraph (:26)
  - INSERT after the TOOLS-DISABLE paragraph (before the closing '# ==='):
        #
        # CHROME-DISABLE (FR-C5, §9.28): verified vs `pi --help` (2026-06-29). Extensions/skills/
        #   prompt-templates/context-files (AGENTS.md/CLAUDE.md) disabled by --no-extensions/--no-skills/
        #   --no-prompt-templates/--no-context-files (all in bare_flags). MCP: NOT disabled — pi has NO
        #   --no-mcp; --no-tools suppresses MCP tool USE but configured servers may still connect at
        #   startup. Documented, tracked LIMITATION (FR-C3), never an assumption MCP is off.
  - DEPENDENCIES: Task 0.

Task 2: MODIFY providers/claude.toml — add CHROME-DISABLE after TOOLS-DISABLE (:24)
  - INSERT:
        #
        # CHROME-DISABLE (FR-C5, §9.28): verified vs `claude --help`. Chrome COVERED: --tools "" (in
        #   bare_flags) disables ALL built-in tools (MCP surfaces as tools), and --setting-sources ""
        #   blocks the settings files where MCP servers, skills, and extensions are configured. No
        #   per-surface gap.
  - DEPENDENCIES: Task 0.

Task 3: MODIFY providers/agy.toml — add CHROME-DISABLE after TOOLS-DISABLE (:53)
  - INSERT:
        #
        # CHROME-DISABLE (FR-C5, §9.28): verified vs `agy --help` (agy v1.1.0, 2026-07-08). NO per-surface
        #   chrome-disable switch exists. --mode plan (bare_flags) is the read-only, never-ask CONSTRAINT
        #   (mutation safety, §12.7.1) — NOT a chrome substitute. Chrome MAY load; the call stays
        #   read-only. Documented LIMITATION (FR-C4). Re-check at the next agy --help re-verification.
  - DEPENDENCIES: Task 0.

Task 4: MODIFY providers/qwen-code.toml — add CHROME-DISABLE after TOOLS-DISABLE (:39)
  - INSERT:
        #
        # CHROME-DISABLE (FR-C5, §9.28): flag surface assembled from docs (NOT yet --help-verified; # TO
        #   CONFIRM per FR-D5). NO known per-surface chrome-disable switch. --approval-mode default
        #   (bare_flags) is the read-only CONSTRAINT, not chrome. Chrome surface is unverified —
        #   documented LIMITATION (FR-C4). Re-verify at the FR-D5 token refresh.
  - NOTE: drop the "(S2)" cross-reference S1's PRP used (it refers to an unrelated plan's S2, ambiguous
    here); "Re-verify at the FR-D5 token refresh" is unambiguous.
  - DEPENDENCIES: Task 0.

Task 5: MODIFY providers/opencode.toml — add CHROME-DISABLE after TOOLS-DISABLE (:25)
  - INSERT:
        #
        # CHROME-DISABLE (FR-C5, §9.28): verified vs `opencode run --help` (opencode 1.1.23, 2026-07-08).
        #   `run` is inherently read-only by design and exposes NO per-surface chrome-disable switch.
        #   bare_flags is empty because `run` is already a read-only one-shot — mutation safety, NOT
        #   chrome. Chrome MAY load; the call stays read-only. Documented LIMITATION (FR-C4).
  - DEPENDENCIES: Task 0.

Task 6: MODIFY providers/codex.toml — add CHROME-DISABLE after TOOLS-DISABLE (:24)
  - INSERT:
        #
        # CHROME-DISABLE (FR-C5, §9.28): verified vs `codex exec --help` (codex-cli 0.143.0, 2026-07-08).
        #   NO per-surface chrome-disable switch for MCP/AGENTS.md/skills. --sandbox read-only +
        #   --ephemeral (bare_flags) are the read-only, session-clean CONSTRAINT (mutation safety,
        #   §12.7.1), NOT chrome. Chrome MAY load; the call stays read-only. Documented LIMITATION (FR-C4).
  - DEPENDENCIES: Task 0.

Task 7: MODIFY providers/cursor.toml — add CHROME-DISABLE after TOOLS-DISABLE (:24)
  - INSERT:
        #
        # CHROME-DISABLE (FR-C5, §9.28): verified vs `agent --help`. NO per-surface chrome-disable switch.
        #   --mode ask + --trust (bare_flags) are the read-only Q&A CONSTRAINT (mutation safety, §12.7.1),
        #   NOT chrome. Chrome MAY load; the call stays read-only. Documented LIMITATION (FR-C4).
  - DEPENDENCIES: Task 0.

Task 8: VERIFY — build, decode-parity, lint
  - go build ./...
  - go test ./internal/provider/... -run 'DecodeParity|AllBuiltinsCovered' -v   # the safety net
  - go test ./internal/provider/... -v
  - make test && make lint
```

### Implementation Patterns & Key Details

```toml
# PATTERN: the CHROME-DISABLE paragraph in each providers/*.toml header (after TOOLS-DISABLE, before '# ===')
#
# TOOLS-DISABLE CATEGORY (§12.7.1): <existing text ...>
#   <existing continuation>
# ============================================================================     ← closing delimiter (BEFORE this)
#
# Insert CHROME-DISABLE BETWEEN the TOOLS-DISABLE paragraph and the '# ===' delimiter:
#
# CHROME-DISABLE (FR-C5, §9.28): <source/date>. <disabled surfaces + flags | "no switch">.
#   <limitation status: COVERED | documented LIMITATION (FR-C3/FR-C4)>.
# ============================================================================

# PATTERN: comment-only safety. The decode-parity test proves fields are unchanged:
#   toml.Unmarshal drops '#' lines → decoded Manifest == builtin → reflect.DeepEqual passes.
#   If you accidentally edit a FIELD line, the test fails with a decoded-vs-builtin diff.
```

### Integration Points

```yaml
NO code / struct / runtime / public-API changes. TOML header comments only.

FILES:
  - providers/{pi,claude,agy,qwen-code,opencode,codex,cursor}.toml — +1 comment paragraph each

CONSUMED (read-only — S1's output is the source of truth):
  - internal/provider/builtin.go CHROME-DISABLE doc-notes (S1) — mirror their substance

DOWNSTREAM (do NOT implement in S2):
  - P1.M1.T2.S1: chrome-contract assertions in builtin_test.go (asserts the named flag tokens in BareFlags)
  - P1.M2.T1.*: docs/providers.md Chrome-disable column + docs/how-it-works.md + README.md

UNCHANGED: builtin.go (S1); every field line in providers/*.toml (decode-parity); builtin_test.go (T2.S1);
  docs/*.md (P1.M2.T1).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Comments don't affect compilation, but verify nothing else drifted
go build ./...
go vet ./...

# The provider reference files are TOML — validate they still parse (toml.Unmarshal in the parity test
# does this; run it explicitly as the gate).
go test ./internal/provider/... -run 'TestProviderReferenceFiles_DecodeParity' -v
# Expected: PASS for all 7 (comments are inert; fields unchanged).

make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# THE safety net: decode-parity proves fields are byte-identical to builtin.go (comments only)
go test ./internal/provider/... -run 'TestProviderReferenceFiles_DecodeParity' -v
# Expected: all 7 subtests PASS.

# Whole provider package (existing tests unchanged)
go test ./internal/provider/... -v

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# No runtime behavior change (the .toml files are not loaded). The within-scope proof is the
# decode-parity test + the grep guard. Optional: eyeball one rendered header:
sed -n '/TOOLS-DISABLE/,/^# ===/p' providers/pi.toml
# Expected: TOOLS-DISABLE paragraph immediately followed by the new CHROME-DISABLE paragraph, then '# ==='.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: prove all 7 files got a CHROME-DISABLE paragraph
grep -rl 'CHROME-DISABLE (FR-C5, §9.28):' providers/ | wc -l
# Expected: 7

# Grep guard: per-file presence
for f in pi claude agy qwen-code opencode codex cursor; do
  grep -q 'CHROME-DISABLE (FR-C5, §9.28):' providers/$f.toml || echo "MISSING: $f"
done
# Expected: no MISSING lines.

# Grep guard: pi records the MCP gap (FR-C3) — the one real limitation
grep -c 'no-mcp\|NO --no-mcp\|FR-C3' providers/pi.toml
# Expected: ≥1

# Scope-boundary guard: NO field line changed (the diff is ALL comment lines, '#' or blank)
git diff providers/ | grep -E '^\+[^# ]' | grep -v '^\+\+\+'
# Expected: empty (every added line starts with '#' or is blank). A non-comment added line means a field
#           was touched — the decode-parity test would also catch it, but fix it before pushing.

# Scope-boundary guard: builtin.go was NOT touched by S2 (S1 owns it)
git diff --stat -- internal/provider/builtin.go
# Expected: empty (S2 is providers/*.toml only).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `make lint` zero errors
- [ ] `go test ./internal/provider/...` passes — `TestProviderReferenceFiles_DecodeParity` GREEN (fields unchanged)
- [ ] `make test` (race) all pass

### Feature Validation
- [ ] All 7 providers/*.toml contain a `# CHROME-DISABLE (FR-C5, §9.28):` paragraph
- [ ] pi note: 4 disabled surfaces by named flags + MCP gap (FR-C3)
- [ ] claude note: --tools "" + --setting-sources "" (chrome covered)
- [ ] agy/qwen-code/opencode/codex/cursor: "no chrome switch; constraint-only; documented limitation (FR-C4)" + source/date
- [ ] Each note placed after TOOLS-DISABLE, before the closing `# ===` header delimiter

### Scope-Boundary Validation
- [ ] NO field line changed in any providers/*.toml (decode-parity proves it)
- [ ] NO builtin.go change (S1 owns it)
- [ ] NO builtin_test.go change (T2.S1)
- [ ] NO docs/providers.md / README.md change (P1.M2.T1)
- [ ] NO new providers/*.toml file (only the 7 existing ones edited)

### Code Quality
- [ ] TOML `#` comments (not `//`); continuation lines use `#   ` (matches TOOLS-DISABLE indentation)
- [ ] Each note carries the verification source/date (FR-D5/FR-T9 discipline)
- [ ] Limitations stated as limitations (FR-C3/FR-C4), never assumptions
- [ ] Substance mirrors S1's builtin.go note (read it once S1 lands to catch wording drift)

---

## Anti-Patterns to Avoid

- ❌ Don't change any FIELD line in providers/*.toml — `TestProviderReferenceFiles_DecodeParity` does `reflect.DeepEqual(decoded, builtin)`; any field drift fails it. Comments only.
- ❌ Don't use Go `//` comments — these are TOML files; use `#`. The marker is `# CHROME-DISABLE (FR-C5, §9.28):`.
- ❌ Don't place the paragraph in the field section — it goes in the HEADER block, after `TOOLS-DISABLE CATEGORY` and before the closing `# ===` delimiter.
- ❌ Don't invent content or flags (FR-C4) — mirror S1's builtin.go note substance. If S1 landed, read the actual notes; if not, use S1's PRP-specified content (the contract).
- ❌ Don't drop the verification source/date — FR-C5 requires "the same discipline as FR-D5/FR-T9" (every existing note carries a date/source).
- ❌ Don't phrase the read-only constraint as if it disables chrome — FR-C4: mutation safety is NOT chrome. State "chrome may load; the call stays read-only" plainly.
- ❌ Don't modify builtin.go (S1), builtin_test.go (T2.S1), or docs/*.md (P1.M2.T1) — S2 is providers/*.toml header comments only.
- ❌ Don't reword pi's MCP gap vaguely — it is the ONE real gap; name it (no --no-mcp; --no-tools suppresses use not discovery) and tag it FR-C3.
- ❌ Don't make the notes longer than needed — the item says "concise (1-4 lines)"; condense S1's longer Go form while keeping the 3 substance elements.

---

## Confidence Score: 9/10

One-pass success is very high: the task is 7 comment-only insertions with fully-specified per-provider
content (S1's PRP gives every note's exact substance; external_deps.md confirms the verification dates),
the placement anchor is grep-verified per file, and the decode-parity test is both the safety net (proves
fields are unchanged) and the validation gate. Comments are inert under toml.Unmarshal, so there is zero
runtime/compile risk. The -1 is for the parallel-edit dependency on S1: S1's actual builtin.go wording
may drift slightly from its PRP spec, so the implementer must read S1's landed notes (Task 0) and mirror
the actual text rather than blindly copying the PRP's pre-specified wording — and S1 may not have landed
yet when S2 starts (the PRP gives the fallback contract for that case). The qwen-code "(S2)"
cross-reference ambiguity is also flagged (Task 4 simplifies it).
