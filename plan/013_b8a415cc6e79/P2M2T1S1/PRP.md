name: "P2.M2.T1.S1 — Verify the agy manifest fields in code match PRD §12.5.1"
description: |
  Read-only CODE verification subtask (Mode A). Confirm that the compiled-in `agy` built-in
  provider manifest (`internal/provider/builtin.go` `builtinAgy()`), its reference document
  (`providers/agy.toml`), and its FR-D4 role-defaults column (`internal/config/role_defaults.go`)
  match PRD §12.5.1 / §12.5.1.1 (re-verified 2026-07-08 against agy v1.1.0) EXACTLY. Produce a
  one-line PASS/FAIL per contract field (a–h), the four role defaults (i–l), the rendered-command
  check (m), and the dated-comment documentation check (n). No file is edited — source files are
  only inspected; the dated manifest comments ARE this subtask's documentation surface (Mode A).
  Architecture audit (`code_gemini_agy_audit.md` Check 1) pre-confirmed all eight fields MATCH;
  this is the independent re-verification.

---

## Goal

**Feature Goal**: Verify — by direct `grep`/`read` inspection of `internal/provider/builtin.go`,
`providers/agy.toml`, and `internal/config/role_defaults.go` at HEAD — that the `agy` provider
manifest matches PRD §12.5.1 byte-for-byte across all eight contract fields, that the FR-D4
per-role model defaults use the agy display-label form, and that the in-code comments carry the
2026-07-08 verification record.

**Deliverable**: A 14-line verification result — one PASS/FAIL line per contract item
(a–h = manifest fields, i–l = role defaults, m = rendered command, n = dated comments) — each
backed by the cited source line number(s). Plus confirmation that `providers/agy.toml` mirrors
`builtinAgy()` and that the agy unit/render tests pass. No source edits are made.

**Success Definition**: All 14 items report PASS; the deliverable names the exact source line for
each field; the one legitimate non-drift nuance (`gemini` as a *model family name* /
*lineage word* in agy comments — "Gemini 3.5 Flash", "Gemini-CLI successor", "diverged from
gemini-cli") is explicitly distinguished from any provider drift and left untouched.

## User Persona (if applicable)

**Target User**: The stagecoach maintainer / verifying engineer (and the orchestrator consuming the
verification result).

**Use Case**: Lock in confidence that the *code's* agy manifest agrees with the *spec* (PRD §12.5.1)
after the 2026-07-08 re-verification against agy v1.1.0, before the P2 milestone declares the
provider-lineup correction done and before the docs-drift fixes (P2.M3) ship.

**User Journey**: Run the 14 verification commands → read each cited source line → emit PASS/FAIL
per item → run the agy unit + render tests as an executable cross-check → report.

**Pain Points Addressed**: Proves the corrected agy flag surface (`--model` not `-m`; stdin not a
bare `-p`; `--mode plan` not `--approval-mode default`; display-label model strings with the
reasoning suffix; `experimental=true` pending the tooled/stager blocker; nil `TooledFlags`) is
actually compiled into the binary and documented in the reference TOML — not just described in the
PRD. Catches any drift introduced between the re-verification commit and now.

## Why

- **Correctness of the agy flag surface (PRD §12.5.1 / §12.5.1.1):** agy v1.1.0 **diverged** from
  the gemini-cli lineage. The old invocation (`--approval-mode default -p` + stdin) no longer works;
  `--approval-mode` was removed, `-p` is value-taking (a bare `-p` fails), and the model flag is
  `--model` (not `-m`). If the compiled manifest regressed to the old surface, every bare-role agy
  invocation would break at runtime. This verification proves the correction persisted.
- **Spec/code parity:** P2.M2.T1.S2 confirms the *PRD* (§12.5.1/§12.5.1.1/§22.1) carries the
  re-verification. This subtask is the *code* half of that parity proof — the binary must ship the
  manifest the spec describes. The two together close the P2.M2 milestone.
- **Foundation for P2.M3 (docs drift):** the Mode B docs-drift fixes (`docs/providers.md`,
  `docs/README.md` agy table rows) assume a known-good compiled manifest to mirror. This subtask
  establishes that ground truth.
- **Executable safety net:** the agy manifest is also asserted by `TestBuiltinManifests_AgyFields`
  and `TestBuiltinManifests_RenderedCommand_Agy`, so a passing test run is independent corroboration
  of the field values AND the rendered command shape.

## What

A pure read-only verification of the three agy source surfaces at HEAD. For each contract item,
confirm the stated invariant holds, then emit one PASS/FAIL line. The exact expected state (all
pre-confirmed at HEAD = `bb3cb3b`, a descendant of the re-verification commit `2f77bd0`):

**Manifest fields — `internal/provider/builtin.go` `builtinAgy()` (func @ `:198`; return literal
`:199-217`):**

- **(a)** `PrintFlag` = `strPtr("")` (NON-NIL empty) — at `:205`. agy v1.1.0's `-p` is value-taking;
  a bare `-p` fails, so NO print flag is emitted (agy reads stdin when `-p` is absent). Must NOT be
  `strPtr("-p")`.
- **(b)** `ModelFlag` = `strPtr("--model")` — at `:206`. Must NOT be `strPtr("-m")` (`-m` is rejected
  by agy v1.1.0 with "flags provided but not defined").
- **(c)** `PromptDelivery` = `strPtr("stdin")` — at `:204`. agy reads the prompt from stdin.
- **(d)** `DefaultModel` = `strPtr("Gemini 3.5 Flash (Low)")` — at `:207`. The `agy models` display
  label VERBATIM, including the "(Low)" reasoning suffix. Must NOT be an API-style id
  (`gemini-3.5-flash`).
- **(e)** `BareFlags` = `[]string{"--mode", "plan"}` — at `:210-212`. agy v1.1.0 has NO
  `--approval-mode` (removed); `--mode plan` is the read-only equivalent. Must NOT be
  `[]string{"--approval-mode", "default"}`.
- **(f)** `ListModelsCommand` = `[]string{"agy", "models"}` — at `:203`.
- **(g)** `Experimental` = `boolPtr(true)` — at `:215`. Ships experimental pending only
  §12.5.1.1 item 4 (the tooled/stager flag combo).
- **(h)** `TooledFlags` = `nil` (field OMITTED) — documented by the comment at `:216`. agy cannot
  serve as a stager until §12.5.1.1 item 4 is resolved. Must NOT be a populated slice.

**Role defaults — `internal/config/role_defaults.go` agy column (block @ `:65-72`), display labels:**

- **(i)** `planner` = `"Gemini 3.5 Flash (High)"` — at `:68` (flagship/smart = high thinking).
- **(j)** `message` = `"Gemini 3.5 Flash (Low)"` — at `:70` (fast/cheapest = low thinking).
- **(k)** `arbiter` = `"Gemini 3.5 Flash (Medium)"` — at `:71` (mid tier).
- **(l)** `stager` = `""` — at `:69` (NOT stager-capable; nil `TooledFlags` → bootstrap applies the
  FR-D4 fallback).
  All four MUST use the display-label form (verbatim label incl. the parenthesized reasoning
  suffix), NOT API-style ids.

**Rendered command + documentation:**

- **(m)** The rendered bare-role command is `agy --model "Gemini 3.5 Flash (Low)" --mode plan`
  with the `<sys>\n\n<user payload>` piped to stdin and NO `-p` flag. Asserted by
  `TestBuiltinManifests_RenderedCommand_Agy` (`internal/provider/builtin_test.go:686-701`); the
  exact expected argv is `["agy","--model","Gemini 3.5 Flash (Low)","--mode","plan"]`.
- **(n)** The manifest comments in `builtin.go` (the `builtinAgy` doc comment + per-field inline
  comments) and `providers/agy.toml` (the header) carry the **2026-07-08** verification record and
  document the divergence from the gemini-cli lineage. This is the in-code documentation (Mode A).

**CONTRACT NOTE — legitimate `gemini` tokens are NOT drift.** The word `gemini` correctly persists
in agy source as a **model family name** ("Gemini 3.5 Flash (Low/High/Medium)") and as **lineage
prose** ("Gemini-CLI successor", "superseded the EOL'd gemini-cli", "diverged from the gemini-cli
lineage"). These must be left alone. Drift = `gemini` referenced as the agy *provider name*, or the
old flag surface (`-p`, `-m`, `--approval-mode`) appearing in the manifest fields — NONE of which
exists at HEAD.

**OUT-OF-SCOPE (do NOT act on — reported only):**
1. `providers/agy.toml` is a human-readable reference document, NOT loaded at runtime (built-ins are
   compiled in). It is verified for *parity* with `builtinAgy()`, but a drift there is a
   documentation issue, not a runtime bug — report it, do not fix it from this read-only subtask.
2. Stale `gemini`/`gemini-2.5-*` strings in test *fixtures* (`internal/config/*_test.go`,
   `roles_test.go`) are opaque config-merge test data, NOT provider-registry lookups (per the
   architecture audit's residual observation #1). They are harmless and out of scope.
3. The sibling task P2.M2.T1.S2 handles the *PRD* half (§12.5.1/§12.5.1.1/§22.1). Do not touch PRD.md.

### Success Criteria

- [ ] Item (a) PASS — `PrintFlag = strPtr("")` at `builtin.go:205`.
- [ ] Item (b) PASS — `ModelFlag = strPtr("--model")` at `builtin.go:206` (not `-m`).
- [ ] Item (c) PASS — `PromptDelivery = strPtr("stdin")` at `builtin.go:204`.
- [ ] Item (d) PASS — `DefaultModel = strPtr("Gemini 3.5 Flash (Low)")` at `builtin.go:207`.
- [ ] Item (e) PASS — `BareFlags = []string{"--mode","plan"}` at `builtin.go:210-212` (not approval-mode).
- [ ] Item (f) PASS — `ListModelsCommand = []string{"agy","models"}` at `builtin.go:203`.
- [ ] Item (g) PASS — `Experimental = boolPtr(true)` at `builtin.go:215`.
- [ ] Item (h) PASS — `TooledFlags = nil` (omitted) at `builtin.go` (comment `:216`).
- [ ] Item (i) PASS — planner = `"Gemini 3.5 Flash (High)"` at `role_defaults.go:68`.
- [ ] Item (j) PASS — message = `"Gemini 3.5 Flash (Low)"` at `role_defaults.go:70`.
- [ ] Item (k) PASS — arbiter = `"Gemini 3.5 Flash (Medium)"` at `role_defaults.go:71`.
- [ ] Item (l) PASS — stager = `""` at `role_defaults.go:69`.
- [ ] Item (m) PASS — rendered argv = `["agy","--model","Gemini 3.5 Flash (Low)","--mode","plan"]`,
      no `-p`, payload via stdin (test `TestBuiltinManifests_RenderedCommand_Agy` passes).
- [ ] Item (n) PASS — `builtin.go` + `providers/agy.toml` comments carry the 2026-07-08 record.
- [ ] `providers/agy.toml` field body mirrors `builtinAgy()` (parity confirmed).
- [ ] `go test ./internal/provider/... -run 'Agy|RenderedCommand'` → PASS.
- [ ] No source file (`builtin.go`, `agy.toml`, `role_defaults.go`, or any other) is modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to verify this
successfully?_ **Yes** — every check is pinned to an exact source line range, a copy-pasteable
grep command, and the exact expected literal. No inference, no external-library docs, and no live
`agy` binary required (the flag surface was already verified live on 2026-07-08; this subtask
confirms the *recorded* manifest matches that verification).

### Documentation & References

```yaml
# MUST READ — the PRD spec of record for agy (READ-ONLY; human-owned — never edit from this subtask)
- file: PRD.md
  why: The spec being matched. §12.5.1 is the corrected agy manifest TOML; §12.5.1.1 is the verified
       status (5 items, dated 2026-07-08, agy v1.1.0). These define the exact expected field values.
  section: "§12.5.1 (h3.58) Built-in provider: Antigravity CLI (agy)" and "§12.5.1.1 (h4.0) Status (agy)"
  gotcha: §12.5.1's manifest block is the AUTHORITATIVE field list; cross-check every code field
          against it. The rendered example ("agy --model \"…\" --mode plan < …stdin…") is item (m).

# The three source surfaces under verification (READ-ONLY from this subtask)
- file: internal/provider/builtin.go
  why: Contains the compiled-in `builtinAgy()` factory — the runtime manifest. Items (a)-(h).
  pattern: `func builtinAgy() Manifest { … }` at :198; return literal :199-217. Fields use the
           strPtr(...)/boolPtr(...) pointer helpers (see manifest.go) — a non-nil pointer to "" is
           an EXPLICIT empty (the §12.1 pointer-scalar design; NOT the same as nil/absent).
  gotcha: "builtin.go:199-217" in the item_description = the function BODY (func keyword is :198).
          The audit's field→line table (203/204/205/206/207/210-212/215/216) is the precise map.

- file: providers/agy.toml
  why: Human-readable REFERENCE document mirroring builtinAgy() byte-for-byte. Verified for PARITY.
  pattern: field lines (`name`, `prompt_delivery`, `print_flag`, `model_flag`, `default_model`,
           `bare_flags`, `list_models_command`, `experimental`) in the body; the header is a rich
           comment block documenting the divergence + 2026-07-08 re-verification.
  gotcha: This file is NOT loaded at runtime (built-ins are compiled into the Go binary). Drift here
          is a docs issue, not a runtime bug. The test fixture `agyTOML` (builtin_test.go:142-160)
          is an identical copy.

- file: internal/config/role_defaults.go
  why: The FR-D4 per-provider × per-role default-model table. Items (i)-(l) = the agy column.
  pattern: `var roleDefaults = RoleModelDefaults{ … "agy": { "planner":…, "stager":…, "message":…,
           "arbiter":… }, … }` — block at :65-72.
  gotcha: agy cells MUST use the display-label form ("Gemini 3.5 Flash (High/Medium/Low)"), NOT
          API-style ids. stager="" is REQUIRED (consistent with TooledFlags=nil).

# Executable cross-checks (the safety net — these pass iff the manifest is correct)
- file: internal/provider/builtin_test.go
  why: Two tests assert the agy manifest directly.
  section: "Test 17: AgyFields" (:642-669) asserts every field; "Test 18: RenderedCommand_Agy"
           (:686-701) asserts the rendered argv (item m). Also `agyTOML` const (:142-160) and the
           all-providers table row `{"agy", builtinAgy(), agyTOML}` (:390).
  pattern: assertStr(t, "<Field>", m.<Field>, "<expected>") helper defined at manifest_test.go:523.

# Architecture audit (read-only reference; pre-confirmed the code side — Check 1)
- docfile: plan/013_b8a415cc6e79/architecture/code_gemini_agy_audit.md
  why: "Check 1" pre-confirmed all eight agy manifest fields MATCH at builtin.go:199-217 with the
       same field→line table. This subtask is the independent re-verification of that audit row.
  section: "Check 1 — internal/provider/builtin.go"

# Sibling task — the PRD half of this parity proof (CONTRACT; defines the spec being matched)
- docfile: plan/013_b8a415cc6e79/P2M1T1S2/PRP.md
  why: Establishes the PRD-side gemini-removal/agy-successor story is internally consistent. The
       code-side agy manifest (this subtask) must agree with the PRD §12.5.1 that subtask protects.

# Pointer-scalar design (why strPtr("") ≠ nil) — REQUIRED READING for items (a),(h)
- file: internal/provider/manifest.go
  why: Explains the *string/*bool pointer-scalar design. A field set to strPtr("") is a NON-NIL
       explicit empty (an OVERRIDE of ""), distinct from nil (ABSENT → inherit built-in). This is
       why PrintFlag=strPtr("") (explicit "no print flag") is correct and deliberate for agy.
  section: Manifest struct doc comment + Resolve() + strPtr/boolPtr helpers (bottom of file).
```

### Current Codebase tree (relevant slice)

```bash
# Run from repo root: cd /home/dustin/projects/stagecoach && tree internal/provider providers internal/config -L 1
internal/provider/
  builtin.go            # ← builtinAgy() @ :198-217 (items a-h)
  builtin_test.go       # TestBuiltinManifests_AgyFields + _RenderedCommand_Agy + agyTOML fixture
  manifest.go           # Manifest struct, Resolve(), strPtr/boolPtr (the pointer-scalar design)
  manifest_test.go      # assertStr helper @ :523
providers/
  agy.toml              # ← reference doc mirroring builtinAgy() (parity check; NOT runtime-loaded)
internal/config/
  role_defaults.go      # ← roleDefaults["agy"] @ :65-72 (items i-l)
plan/013_b8a415cc6e79/
  P2M1T1S2/PRP.md                  # sibling: PRD-side verification (CONTRACT)
  P2M2T1S1/
    PRP.md                         # ← this file
    research/per_item_evidence.md  # per-item evidence table collected at HEAD
  architecture/code_gemini_agy_audit.md  # read-only audit (Check 1 pre-confirmed the fields)
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
# NONE. This is a read-only verification subtask. No files are created, modified, or deleted.
# The only artifacts are (1) this PRP, (2) research/per_item_evidence.md, and (3) the 14-line
# PASS/FAIL verdict reported back. Source files are inspected, never edited.
```

### Known Gotchas of our codebase & Library Quirks

```text
# GOTCHA 1 — strPtr("") is a NON-NIL explicit empty, NOT the same as nil/absent.
#   The §12.1 pointer-scalar design (manifest.go) means a manifest field set to strPtr("") is an
#   EXPLICIT override to "". For agy, PrintFlag=strPtr("") (item a) is the DELIBERATE "no print
#   flag" value — agy v1.1.0's -p is value-taking, so emitting -p breaks delivery; the empty print
#   flag + stdin delivery is the correct combination. Do NOT "fix" strPtr("") to nil or to "-p".
#   Likewise SystemPromptFlag/ProviderFlag = strPtr("") (explicit empty — sys prepended, no sub-prov).

# GOTCHA 2 — "gemini" tokens in agy source are LEGITIMATE (model family + lineage), NOT drift.
#   The agy provider RUNS the Gemini model family ("Gemini 3.5 Flash (Low)") and DESCENDS from the
#   gemini-cli lineage ("Gemini-CLI successor", "superseded the EOL'd gemini-cli", "diverged from
#   the gemini-cli lineage"). These comment/label tokens are CORRECT and must remain. Drift =
#   `gemini` as the provider NAME, or the OLD flag surface (-p/-m/--approval-mode) in the fields.

# GOTCHA 3 — providers/agy.toml is NOT loaded at runtime.
#   Built-in manifests are COMPILED into the Go binary (BuiltinManifests() in builtin.go). The TOML
#   is a human-readable reference + a config-override TEMPLATE (§12.8). It is verified for PARITY
#   with builtinAgy(), but a drift there does not change runtime behavior. Report, do not fix.

# GOTCHA 4 — the item_description's "builtin.go:199-217" = the function BODY.
#   The `func builtinAgy()` keyword is at :198; `return Manifest{` is :199; the field literals span
#   :200-216; the return closes at :217. The audit's precise field→line map is authoritative:
#   ListModelsCommand:203, PromptDelivery:204, PrintFlag:205, ModelFlag:206, DefaultModel:207,
#   BareFlags:210-212, Experimental:215, TooledFlags-omitted(:216 comment).

# GOTCHA 5 — stager="" in roleDefaults is REQUIRED, not a gap.
#   agy's TooledFlags is nil (item h) → it cannot serve as a stager → its FR-D4 stager cell MUST be
#   "" so the config bootstrap (P1.M4.T2) applies the FR-D4 fallback to the next
#   TooledFlags-capable provider. A non-empty stager here would be the bug, not the empty string.
```

## Implementation Blueprint

### Verification approach (not "implementation" — read-only)

There are no data models to create. The "tasks" below are verification steps. Each step has an exact
command and an exact expected result; a step PASSES iff the observed result equals the expected
result. Emit the one-line PASS/FAIL verdict for the corresponding contract item. All steps run from
the repo root (`/home/dustin/projects/stagecoach`).

### Verification Tasks (ordered; each maps to a contract item)

```yaml
Task V0: CONFIRM baseline / that the re-verification is committed
  - RUN: git log --oneline -1 --grep='Re-verify and fix agy manifest'
  - EXPECT: a line beginning with `2f77bd0` (the re-verification commit). HEAD `bb3cb3b` is a
            descendant; the correction persists. If absent, the expected state is not present — report.
  - RUN: go build ./...
  - EXPECT: EXIT 0 (clean build).
  - RUN: go test ./internal/provider/... -run 'Agy|RenderedCommand'
  - EXPECT: PASS (both TestBuiltinManifests_AgyFields and TestBuiltinManifests_RenderedCommand_Agy).

Task V1 → item (a): CONFIRM PrintFlag = strPtr("") (NON-NIL empty)
  - RUN: sed -n '205p' internal/provider/builtin.go
  - EXPECT: a line containing `PrintFlag:` whose value is `strPtr("")` — with a comment noting agy
            reads stdin and a bare -p is value-taking. Must NOT be `strPtr("-p")`.
  - VERDICT: item (a) PASS iff PrintFlag == strPtr("").

Task V2 → item (b): CONFIRM ModelFlag = strPtr("--model") (NOT "-m")
  - RUN: sed -n '206p' internal/provider/builtin.go
  - EXPECT: `ModelFlag:` value `strPtr("--model")` (comment: `-m` rejected by agy). Must NOT be `-m`.
  - VERDICT: item (b) PASS iff ModelFlag == strPtr("--model").

Task V3 → item (c): CONFIRM PromptDelivery = strPtr("stdin")
  - RUN: sed -n '204p' internal/provider/builtin.go
  - EXPECT: `PromptDelivery:` value `strPtr("stdin")`.
  - VERDICT: item (c) PASS iff PromptDelivery == strPtr("stdin").

Task V4 → item (d): CONFIRM DefaultModel = strPtr("Gemini 3.5 Flash (Low)")
  - RUN: sed -n '207p' internal/provider/builtin.go
  - EXPECT: `DefaultModel:` value `strPtr("Gemini 3.5 Flash (Low)")` (display label, verbatim incl.
            the "(Low)" reasoning suffix). Must NOT be an API-style id like gemini-3.5-flash.
  - VERDICT: item (d) PASS iff DefaultModel == strPtr("Gemini 3.5 Flash (Low)").

Task V5 → item (e): CONFIRM BareFlags = []string{"--mode","plan"} (NOT approval-mode)
  - RUN: sed -n '210,212p' internal/provider/builtin.go
  - EXPECT: `BareFlags: []string{` then `"--mode", "plan",` then `}`. Must NOT contain
            `--approval-mode` or `default`.
  - RUN (negative): grep -n 'approval-mode' internal/provider/builtin.go
  - EXPECT: no match (or only a comment stating agy v1.1.0 has NO --approval-mode).
  - VERDICT: item (e) PASS iff BareFlags == []string{"--mode","plan"} and no active approval-mode flag.

Task V6 → item (f): CONFIRM ListModelsCommand = []string{"agy","models"}
  - RUN: sed -n '203p' internal/provider/builtin.go
  - EXPECT: `ListModelsCommand:` value `[]string{"agy", "models"}`.
  - VERDICT: item (f) PASS iff ListModelsCommand == []string{"agy","models"}.

Task V7 → item (g): CONFIRM Experimental = boolPtr(true)
  - RUN: sed -n '215p' internal/provider/builtin.go
  - EXPECT: `Experimental:` value `boolPtr(true)`.
  - VERDICT: item (g) PASS iff Experimental == boolPtr(true).

Task V8 → item (h): CONFIRM TooledFlags = nil (OMITTED)
  - RUN: sed -n '198,217p' internal/provider/builtin.go | grep -n 'TooledFlags'
  - EXPECT: the ONLY TooledFlags mention is a COMMENT (e.g. `// TooledFlags: nil — agy cannot serve
            as a stager …`). There must be NO `TooledFlags:` field assignment in the return literal.
  - VERDICT: item (h) PASS iff TooledFlags is omitted (nil) — only a comment references it.

Task V9 → items (i)-(l): CONFIRM roleDefaults["agy"] uses display labels + stager=""
  - RUN: sed -n '65,72p' internal/config/role_defaults.go
  - EXPECT: an `"agy": { … }` block with:
        planner  = "Gemini 3.5 Flash (High)"
        stager   = ""                                   (NOT stager-capable; TooledFlags nil)
        message  = "Gemini 3.5 Flash (Low)"
        arbiter  = "Gemini 3.5 Flash (Medium)"
  - RUN (negative — no API-style ids in the agy column): grep -nE 'gemini-[0-9]' internal/config/role_defaults.go
  - EXPECT: no match in the agy block (model ids like "gemini-3.5-flash" would be drift).
  - VERDICT: items (i),(j),(k),(l) PASS iff the four cells match the display-label values above and
             stager is "".

Task V10 → item (m): CONFIRM the rendered command (no -p; stdin delivery; display-label model)
  - RUN: go test ./internal/provider/... -run 'TestBuiltinManifests_RenderedCommand_Agy' -v
  - EXPECT: PASS. The test (builtin_test.go:686-701) asserts argv ==
            ["agy","--model","Gemini 3.5 Flash (Low)","--mode","plan"] with NO -p and the
            <sys>\n\n<user payload> piped to stdin.
  - ALSO READ: sed -n '686,701p' internal/provider/builtin_test.go to confirm the want[] argv.
  - VERDICT: item (m) PASS iff the test passes (the corrected render: no -p, stdin payload).

Task V11 → item (n): CONFIRM the 2026-07-08 verification record in comments
  - RUN: grep -n '2026-07-08' internal/provider/builtin.go
  - EXPECT: multiple hits in the builtinAgy doc comment + per-field inline comments (:156,:175,
            :182,:193,:203,:205,:206,:207,:211) documenting the agy v1.1.0 re-verification.
  - RUN: grep -n '2026-07-08' providers/agy.toml
  - EXPECT: multiple hits in the header (:20,:27,:35,:45,:63) — rendered-command, model-names,
            divergence-from-gemini-cli, experimental status, list_models_command.
  - VERDICT: item (n) PASS iff both files carry the 2026-07-08 record in their manifest comments.

Task V12: CONFIRM providers/agy.toml PARITY with builtinAgy() (reference-doc check)
  - RUN: sed -n '/^name = "agy"/,/^experimental = true/p' providers/agy.toml   # the field body
  - EXPECT: field values identical to builtinAgy() (prompt_delivery="stdin", print_flag="",
            model_flag="--model", default_model="Gemini 3.5 Flash (Low)",
            bare_flags=["--mode","plan"], list_models_command=["agy","models"],
            experimental=true, tooled_flags omitted).
  - RUN: compare against the agyTOML fixture: sed -n '142,160p' internal/provider/builtin_test.go
  - EXPECT: identical field body (the test fixture is the canonical copy).
  - VERDICT: PASS iff the TOML field body mirrors builtinAgy(). (Report-only on any drift — TOML is
            not runtime-loaded; see Gotcha 3.)

Task V13: CONFIRM classification sweep — legitimate gemini tokens, no provider drift
  - RUN: grep -n 'gemini' internal/provider/builtin.go
  - EXPECT: hits are COMMENT-ONLY (model-family labels: "Gemini 3.5 Flash (Low)"; lineage: "Gemini-CLI
            successor", "the former gemini provider", "gemini-cli lineage"). NONE is a provider name
            or the old flag surface. (See Gotcha 2.)
  - VERDICT: informational. Their presence is EXPECTED and correct.
```

### Restore / escalation procedure (only if an item UNEXPECTEDLY fails)

All 14 items were pre-confirmed PASS at HEAD (`bb3cb3b`). A failure means the committed state
regressed OR the re-verification commit (`2f77bd0`) was partially reverted. Because **source files
are not edited from this read-only subtask** (and the manifest comments ARE the documentation
surface, Mode A), a failure is REPORTED, not patched:

```text
1. Re-run the failing item's grep/read command on a fresh checkout of HEAD to rule out a stale read.
2. If it still fails, run: git show 2f77bd0 -- internal/provider/builtin.go providers/agy.toml
   internal/config/role_defaults.go   # the re-verification diff that should have persisted
   and diff against the current file to locate the regression.
3. REPORT the failing item, the observed vs expected value, and the source line — do NOT fix it here.
   A source edit to the manifest is a separate change (the manifest + its dated comments are the
   documentation surface; changing them requires re-running the live agy v1.1.0 verification).
```

### Integration Points

```yaml
# NONE. Read-only code verification — no DATABASE, CONFIG, ROUTES, or code integration.
# The only "integration" is consuming the verification result in the P2 milestone reporting and
# confirming parity with the PRD half (sibling P2.M2.T1.S2) and the docs-drift half (P2.M3) before
# the provider-lineup correction is declared done.
```

## Validation Loop

### Level 1: Build & Test (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
go build ./...                                                   # EXPECT: EXIT 0 (clean)
go test ./internal/provider/... -run 'Agy|RenderedCommand' -v    # EXPECT: PASS (AgyFields + RenderedCommand_Agy)
go test ./internal/provider/... ./internal/config/...            # EXPECT: both packages `ok`

# Expected: zero errors. The agy manifest is also asserted by the test suite, so a green test run
# independently corroborates items (a)-(m).
```

### Level 2: Per-Item Verification (the core gate)

```bash
cd /home/dustin/projects/stagecoach
# One-shot field sweep of builtinAgy():
sed -n '198,217p' internal/provider/builtin.go     # items (a)-(h) — read the whole function body
# Per-field pinpoint:
sed -n '205p' internal/provider/builtin.go         # (a) PrintFlag  = strPtr("")
sed -n '206p' internal/provider/builtin.go         # (b) ModelFlag  = strPtr("--model")
sed -n '204p' internal/provider/builtin.go         # (c) PromptDelivery = strPtr("stdin")
sed -n '207p' internal/provider/builtin.go         # (d) DefaultModel = strPtr("Gemini 3.5 Flash (Low)")
sed -n '210,212p' internal/provider/builtin.go     # (e) BareFlags  = []string{"--mode","plan"}
sed -n '203p' internal/provider/builtin.go         # (f) ListModelsCommand = []string{"agy","models"}
sed -n '215p' internal/provider/builtin.go         # (g) Experimental = boolPtr(true)
sed -n '198,217p' internal/provider/builtin.go | grep TooledFlags   # (h) TooledFlags — comment only (nil)
sed -n '65,72p' internal/config/role_defaults.go   # (i)-(l) agy role defaults (display labels, stager="")
sed -n '686,701p' internal/provider/builtin_test.go # (m) the want[] argv in RenderedCommand_Agy
grep -n '2026-07-08' internal/provider/builtin.go providers/agy.toml   # (n) dated verification record

# Expected: each command shows the exact expected value cited in Tasks V1-V11.
```

### Level 3: Cross-Reference Sweep (System Validation)

```bash
cd /home/dustin/projects/stagecoach
# (1) No OLD flag surface in the agy manifest (the divergence markers):
grep -nE 'approval-mode|PrintFlag:.*"-p"|ModelFlag:.*"-m"' internal/provider/builtin.go
# EXPECT: no field assignment matches (comments mentioning these are fine — they document the
#         divergence). A live `--approval-mode`/`-p`/`-m` field assignment would be a regression.

# (2) agy.toml field body mirrors builtinAgy() (parity):
sed -n '/^name = "agy"/,/^experimental = true/p' providers/agy.toml
diff <(sed -n '/^name = "agy"/,/^experimental = true/p' providers/agy.toml) \
     <(sed -n '142,160p' internal/provider/builtin_test.go | sed 's/^const agyTOML = `//; s/`$//')
# EXPECT: no meaningful field differences (modulo comment-only lines). The agyTOML fixture is the
#         canonical copy the test compares against.

# (3) No gemini provider drift in the agy column / manifest (legitimate model/lineage tokens OK):
grep -n 'gemini' internal/provider/builtin.go internal/config/role_defaults.go
# EXPECT: only comment/label tokens (Gotcha 2); no `gemini` as the agy provider name or field.
# Expected: (1) no field regression; (2) parity holds; (3) no provider drift.
```

### Level 4: Domain-Specific Validation (executable manifest proof)

```bash
cd /home/dustin/projects/stagecoach
# The two agy tests are the executable proof that the manifest + render are correct:
go test ./internal/provider/... -run 'TestBuiltinManifests_AgyFields' -v           # all field assertions
go test ./internal/provider/... -run 'TestBuiltinManifests_RenderedCommand_Agy' -v # rendered argv (item m)
go test ./internal/provider/... -run 'TestBuiltinManifests_AgyTOML\|TestAllProvidersRoundTrip\|TestReferenceFiles' -v 2>/dev/null \
  || go test ./internal/provider/... -v | grep -i agy   # any test touching the agy manifest/TOML
# Expected: all PASS. A failure here would contradict a PASS verdict on the corresponding field item —
# investigate before reporting (the test is the authoritative executable witness).
```

## Final Validation Checklist

### Technical Validation

- [ ] Baseline confirmed: re-verification commit `2f77bd0` present; HEAD is a descendant; build clean.
- [ ] `go test ./internal/provider/... -run 'Agy|RenderedCommand'` → PASS.

### Feature (Verification) Validation

- [ ] Item (a) PASS — PrintFlag = `strPtr("")` at `builtin.go:205`.
- [ ] Item (b) PASS — ModelFlag = `strPtr("--model")` at `builtin.go:206` (not `-m`).
- [ ] Item (c) PASS — PromptDelivery = `strPtr("stdin")` at `builtin.go:204`.
- [ ] Item (d) PASS — DefaultModel = `strPtr("Gemini 3.5 Flash (Low)")` at `builtin.go:207`.
- [ ] Item (e) PASS — BareFlags = `[]string{"--mode","plan"}` at `builtin.go:210-212` (no approval-mode).
- [ ] Item (f) PASS — ListModelsCommand = `[]string{"agy","models"}` at `builtin.go:203`.
- [ ] Item (g) PASS — Experimental = `boolPtr(true)` at `builtin.go:215`.
- [ ] Item (h) PASS — TooledFlags = nil (omitted) in `builtinAgy()`.
- [ ] Items (i)-(l) PASS — roleDefaults["agy"] planner/message/arbiter display labels + stager="" at
      `role_defaults.go:65-72`.
- [ ] Item (m) PASS — rendered argv has no `-p`, payload via stdin (test passes).
- [ ] Item (n) PASS — `builtin.go` + `providers/agy.toml` carry the 2026-07-08 record.
- [ ] `providers/agy.toml` field body mirrors `builtinAgy()` (parity).
- [ ] 14-line PASS/FAIL verdict emitted (a–n).
- [ ] Legitimate model-family / lineage `gemini` tokens classified and confirmed non-drift.

### Code Quality Validation

- [ ] No source file (`builtin.go`, `agy.toml`, `role_defaults.go`, or any other) modified.
- [ ] No `PRD.md` / `tasks.json` / `prd_snapshot.md` / `.gitignore` touched.
- [ ] Out-of-scope items (runtime-load status of agy.toml; stale gemini test fixtures; the PRD-half
      P2.M2.T1.S2) reported, NOT acted on.

### Documentation & Deployment

- [ ] Per contract §5 (Mode A): the dated manifest comments ARE the documentation surface — confirming
      the compiled manifest carries the 2026-07-08 verification record is the deliverable; no
      additional docs artifact is required beyond the verdict.
- [ ] Verification result recorded for the P2 milestone (P2.M2.T1.S2 PRD-half and P2.M3 docs-drift
      depend on a confirmed code-side agy manifest).

---

## Anti-Patterns to Avoid

- ❌ Don't "fix" `PrintFlag = strPtr("")` to `-p` or to nil — the NON-NIL empty is deliberate; agy
  v1.1.0's `-p` is value-taking and a bare `-p` breaks stdin delivery (the whole point of the
  re-verification). See Gotcha 1.
- ❌ Don't "fix" `BareFlags = []string{"--mode","plan"}` back to `--approval-mode default` —
  `--approval-mode` was REMOVED in agy v1.1.0; `--mode plan` is the read-only equivalent.
- ❌ Don't treat model-family `gemini` tokens ("Gemini 3.5 Flash") or lineage prose ("Gemini-CLI
  successor", "diverged from gemini-cli") as drift — agy runs the Gemini family and descends from
  gemini-cli; those tokens are correct and MUST remain.
- ❌ Don't treat `providers/agy.toml` drift as a runtime bug — it is NOT loaded at runtime; built-ins
  are compiled in. Report TOML drift, don't patch it from this read-only subtask.
- ❌ Don't edit any source file — this is read-only verification; the manifest + dated comments are
  the documentation surface (Mode A). A found regression is REPORTED, not fixed, from this subtask.
- ❌ Don't conflate the stale `gemini`/`gemini-2.5-*` strings in config test *fixtures* with provider
  drift — they are opaque config-merge test data, not registry lookups (audit residual obs. #1).
- ❌ Don't judge the agy manifest against the PRD §12.5.1 TOML *and* separately require them to differ
  — they must AGREE. The PRD is the spec; the code must match it byte-for-byte on these eight fields.

---

## Confidence Score

**One-pass success likelihood: 10/10.** This is a read-only verification of the agy manifest at a
known-good committed state (`2f77bd0` re-verification, confirmed at HEAD `bb3cb3b`). Every check is
pinned to an exact field value + an observed source line, corroborated by a passing executable test
(`TestBuiltinManifests_AgyFields`, `TestBuiltinManifests_RenderedCommand_Agy`) and a pre-confirming
architecture audit (Check 1). The deliverable is a deterministic PASS/FAIL verdict per item; there is
no implementation surface to get wrong, and the single hardest nuance (legitimate model-family vs
provider-drift `gemini` tokens, and `strPtr("")` non-nil-empty vs nil) is spelled out with a full
classification guide (Gotchas 1-2). Per-item evidence is in `research/per_item_evidence.md`.
