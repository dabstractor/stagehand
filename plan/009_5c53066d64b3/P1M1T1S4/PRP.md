---
name: "P1.M1.T1.S4 — Set SessionMode=\"append\" on pi builtin + providers/pi.toml (FR-T9 verified)"
description: |
  Surgical value-setting subtask (R1's shipped value). Set `SessionMode: strPtr("append")` on the pi
  provider ONLY — the one provider whose append-turn rendering is FR-T9-VERIFIED (2026-07-05 live run,
  architecture/fr-t9-verification.md). Every other builtin ships absent (Resolve() defaults to "").

  ⚠️ THREE files must change in lockstep (NOT two — the work item names builtin.go + providers/pi.toml,
  but TWO reflect.DeepEqual decode-parity guards force a third edit):
    (1) internal/provider/builtin.go  — builtinPi(): add `SessionMode: strPtr("append"),` (with the
        FR-T9 VERIFIED inline comment) between ProviderFlag and BareFlags (PRD §12.1 ordering).
    (2) providers/pi.toml             — add `session_mode = "append"` (with `# VERIFIED 2026-07-05; FR-T9.`)
        between the `provider_flag` and `bare_flags` sections (PRD §12.1 ordering).
    (3) internal/provider/builtin_test.go — the `piTOML` const (line 16): add the SAME `session_mode = "append"`
        line. THIS IS MANDATORY: TestBuiltinManifests_DecodeParity does reflect.DeepEqual(builtinPi(),
        decode(piTOML)) — a 3rd-copy sync. (Omitting it is the #1 one-pass failure mode.)

  S1 LANDED (Manifest.SessionMode *string + Resolve "" default + Validate ""|"append" enum — verified).
  S3 (RenderMultiTurn, in parallel) reads *r.SessionMode == "append" as its capability gate; S4 is what
  makes that gate PASS for pi in production. NO docs (providers.md session_mode doc rides with S5).
---

## Goal

**Feature Goal**: Ship the FR-T9-VERIFIED `session_mode = "append"` value on the pi provider manifest (and
ONLY pi), so the multi-turn fallback's capability gate (FR-T1 condition d / S3's `RenderMultiTurn` gate
`*r.SessionMode == "append"`) is satisfied for pi and unsatisfied for every other provider — making
multi-turn available for pi and silently skipped (one-shot → rescue unchanged) for claude/opencode/codex/
cursor/agy/gemini/qwen-code until each is independently verified.

**Deliverable**: One new field assignment in `builtinPi()` (`internal/provider/builtin.go`), one new TOML
key in `providers/pi.toml`, and the matching line synced into the `piTOML` test-fixture const
(`internal/provider/builtin_test.go`). No other provider is touched. No new files.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green —
**critically both decode-parity guards pass**: `TestBuiltinManifests_DecodeParity` (builtinPi() ==
decode(piTOML)) AND `TestProviderReferenceFiles_DecodeParity` (decode(providers/pi.toml) == builtinPi()).
`builtinPi().SessionMode` is non-nil `*("append")`; `builtinClaude()`/`gemini()`/etc. `SessionMode` is nil
(unchanged). `git diff --stat` shows ONLY `internal/provider/builtin.go` +
`internal/provider/builtin_test.go` + `providers/pi.toml`.

## User Persona

**Target User**: The contributor implementing the multi-turn fallback protocol (P1.M1.T3.S2 — the N+1 turn
loop that calls S3's `RenderMultiTurn`) and the end user whose one-shot generation repeatedly fails on a
large diff with pi — S4 is what makes pi eligible for the lossless multi-turn fallback at all.

**Use Case**: A user with a 266K-token diff on pi hits one-shot exhaustion. The trigger gate (FR-T1)
checks condition (d): does the resolved pi manifest declare `session_mode = "append"`? After S4, yes →
multi-turn activates (lossless N+1-turn priming). Before S4, pi's SessionMode is nil/"", condition (d) is
false, and the run falls straight to rescue — multi-turn is dead code even though S1/S2/S3 landed.

**Pain Points Addressed**: Without S4, the entire SessionMode plumbing chain (S1 field → S2 merge → S3
renderer+gate) is internally complete but never fires for ANY provider — the shipped default is "" for all
eight. S4 is the single value-flip that turns the chain on for the one verified provider.

## Why

- **The value-flip that activates the whole chain.** S1 added the field (dead), S2 made it config-overridable
  (still dead at the shipped default), S3 added the renderer+gate (gate never passes — every built-in ships
  ""). S4 sets pi's shipped value to the FR-T9-VERIFIED "append", which is the ONLY change that makes the
  gate pass in production. It is the keystone value of R1.
- **FR-T9 is a verification DUTY, not a guess.** PRD §9.24 FR-T9: a manifest MUST NOT declare "append"
  speculatively; it requires a verified, reproducible append-turn rendering. That verification is DONE for
  pi (2026-07-05 live run, recorded in architecture/fr-t9-verification.md: same `--session-id` re-invoked
  recalls "BANANA"). S4 ships the verified value WITH the inline verification record (the established
  `# VERIFIED <date> via \`<cmd>\`` discipline, FR-D5 — see builtin.go ListModelsCommand). Every other
  provider stays "" because its append mechanism is NOT yet verified — FR-T9 forbids shipping "append"
  for them.
- **Lowest-risk value flip.** No logic, no rendering, no parsing, no precedence. Three text additions
  (one Go field, two TOML lines) plus the mandatory test-fixture sync. The only correctness gates are
  (a) the right value on the right provider and (b) the two decode-parity DeepEquals stay green.
- **Scope discipline: ONLY pi.** S4 deliberately does NOT set SessionMode on claude/opencode/codex/cursor/
  agy/gemini/qwen-code. For those, FR-T1 condition (d) stays false and multi-turn is skipped silently
  (one-shot → rescue unchanged). Setting "append" on an unverified provider would violate FR-T9.

## What

Three lockstep edits (the value, the reference doc, and the test fixture that mirrors them):

1. **`internal/provider/builtin.go` — `builtinPi()`**: add `SessionMode: strPtr("append"),` with the
   FR-T9 inline verification comment, placed between the `ProviderFlag:` and `BareFlags:` lines (PRD §12.1
   ordering: provider_flag → session_mode → bare_flags; matches S1's struct-slot decision). gofmt realigns
   the struct's value column automatically — do NOT hand-align the Go struct.
2. **`providers/pi.toml`**: add a `# --- session continuation (multi-turn fallback, §9.24) ---` section
   with `session_mode = "append"` and the `# VERIFIED 2026-07-05; FR-T9.` comment, between the
   `# --- sub-provider ---` section (provider_flag) and the `# --- bare mode ---` section (bare_flags).
   providers/pi.toml is hand-aligned reference doc (raw text, NOT gofmt'd) — match the existing inline-
   comment column.
3. **`internal/provider/builtin_test.go` — `piTOML` const (line 16)**: add the SAME `session_mode = "append"`
   line between `provider_flag = "--provider"` and `bare_flags = [`. This is MANDATORY (see Gotchas) — the
   `TestBuiltinManifests_DecodeParity` guard does `reflect.DeepEqual(builtinPi(), decode(piTOML))`.

*(Recommended, not required for green)* 4. **`TestBuiltinManifests_PiFields` (builtin_test.go:256)**: add a
`SessionMode` positive assertion (`assertStr(t, "SessionMode", m.SessionMode, "append")`) so the new field
is covered by the field-enumeration test. PiFields uses per-field asserts (NOT DeepEqual), so omitting this
does NOT fail the build — but adding it matches the existing per-field discipline and documents intent.

### Success Criteria

- [ ] `builtinPi().SessionMode` is non-nil and `*builtinPi().SessionMode == "append"` (with the VERIFIED
      inline comment).
- [ ] `builtinClaude()`, `builtinGemini()`, `builtinOpenCode()`, `builtinCodex()`, `builtinCursor()`,
      `builtinAgy()`, `builtinQwenCode()` each have `SessionMode == nil` (UNTOUCHED — no S4 edit).
- [ ] `providers/pi.toml` contains `session_mode = "append"` with the `# VERIFIED 2026-07-05; FR-T9.` comment,
      positioned between `provider_flag` and `bare_flags`.
- [ ] The `piTOML` const (builtin_test.go:16) contains the matching `session_mode = "append"` line.
- [ ] `TestBuiltinManifests_DecodeParity` PASSES (builtinPi() == decode(piTOML), incl. SessionMode).
- [ ] `TestProviderReferenceFiles_DecodeParity` PASSES (decode(providers/pi.toml) == builtinPi()).
- [ ] `TestBuiltinManifests_Validate` PASSES for all 8 builtins (pi's "append" is a valid enum value; S1).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] ONLY `internal/provider/builtin.go` + `internal/provider/builtin_test.go` + `providers/pi.toml` change.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the EXACT current `builtinPi()` struct literal (the insert point),
the EXACT target line for builtin.go (verbatim, with the FR-T9 comment from the verification record), the
EXACT target block for providers/pi.toml, the EXACT target line for the `piTOML` test const, and — the
load-bearing insight — names BOTH decode-parity guards and explains WHY the `piTOML` const is a mandatory
third edit (reflect.DeepEqual against builtinPi()). The S1-landed state (SessionMode *string + Resolve
default + Validate enum) is verified. The FR-T9 verification (the authority for the "append" value + the
VERIFIED-comment wording) is quoted. The "only pi, nowhere else" scope is reinforced.

### Documentation & References

```yaml
# MUST READ — the FR-T9 verification (the SOLE authority for the "append" value + the comment wording)
- docfile: plan/009_5c53066d64b3/architecture/fr-t9-verification.md
  why: "Records the 2026-07-05 LIVE pi run proving append-turn recall: turn 1 `pi ... --session-id
        stagecoach-frt9-probe -p \"remember the word BANANA\"` → \"Got it — BANANA\"; turn 2 (SAME session id)
        `-p \"What word...?\"` → \"BANANA\". Verdict: re-invoking the same --session-id appends a recallable
        turn. Gives the EXACT shipped lines (builtin.go SessionMode comment + pi.toml `# VERIFIED 2026-07-05;
        FR-T9.`) and the verified flag transformation RenderMultiTurn (S3) encodes."
  critical: "This file IS the FR-T9 verification. S4 ships the value it authorizes, with the inline comment
        it specifies. Do NOT reword the VERIFIED comment — it is the audit trail FR-T9 mandates (analogous
        to FR-D5's model-token verification duty). The value 'append' is pi-ONLY because only pi is verified
        here; every other provider is absent from this file ⇒ stays ''."

# MUST READ — the field S4 sets (S1 LANDED — verified in the working tree)
- docfile: plan/009_5c53066d64b3/P1M1T1S1/PRP.md
  why: "S1 added Manifest.SessionMode *string `toml:\"session_mode\"` (manifest.go), Resolve() default
        strPtr('') (so *r.SessionMode is always safe to deref — S3's gate depends on this), and Validate()
        enum ('' | 'append'; nil passes). S1's description literally names S4 as 'the pi append value' and
        S1's success criteria state 'NO builtin value set (S4's job)'. S4 fulfills that."
  critical: "SessionMode is *string (NOT plain string) — S4 sets it via strPtr('append'), exactly as S1's
        TestResolve_PreservesExplicitValues uses strPtr('append'). Validate accepts 'append' (S1 enum) so
        TestBuiltinManifests_Validate stays green for pi. Do NOT edit manifest.go (S1 owns it)."

# MUST READ — the parallel contract (S3's gate is what S4's value satisfies)
- docfile: plan/009_5c53066d64b3/P1M1T1S3/PRP.md
  why: "S3 (in parallel) adds RenderMultiTurn with the capability gate `if *r.SessionMode != \"append\" {
        return error }`. That gate reads the RESOLVED manifest's SessionMode. S4 is what makes the gate PASS
        for pi in production (before S4, every built-in ships '' ⇒ gate always errors ⇒ multi-turn dead).
        S3's unit tests set SessionMode in a literal (independent of S4); S4's value is what the real
        registry resolves for pi."
  critical: "S3 and S4 are CODE-INDEPENDENT (S3's tests don't depend on S4's builtin value), but they are
        SEMANTICALLY complementary: S3 = the gate, S4 = the value that opens it for pi. S4 does NOT edit
        render.go (S3) or manifest.go (S1) or merge.go (S2)."

# The research note that pinned builtinPi + the VERIFIED comment discipline
- docfile: plan/009_5c53066d64b3/architecture/research-provider.md
  why: "§3 (builtin.go builtinPi()): confirms SessionMode does NOT exist yet, slots between ProviderFlag
        (line ~60) and BareFlags (line ~63) per PRD §12.1, and — §3 'VERIFIED comment discipline (FR-D5)' —
        establishes the exact inline-comment format: `// VERIFIED <date> via \\`<cmd>\\` ...; <FR ref>`,
        citing builtin.go:47 ListModelsCommand as the precedent. §3 explicitly states: 'When pi's
        SessionMode: strPtr(\"append\") ships, the field SHOULD carry an inline comment recording the FR-T9
        verification ... Same discipline as # VERIFIED (mirrored in providers/pi.toml:34).'"
  critical: "§3 is the authority for (a) the insert point (between ProviderFlag and BareFlags), (b) the
        VERIFIED-comment format and precedent, and (c) the fact that the discipline is MIRRORED in
        providers/pi.toml (i.e. both files carry the VERIFIED audit trail)."

# The file under edit #1
- file: internal/provider/builtin.go
  why: "EDIT builtinPi() (lines ~30-96): add ONE line `SessionMode: strPtr(\"append\"), // VERIFIED ...`
        between the `ProviderFlag: strPtr(\"--provider\"),` line and the `BareFlags: []string{` line. This is
        the ONLY change in builtin.go. gofmt realigns the struct value column — do NOT hand-align."
  pattern: "Mirror the surrounding *string field assignments: `Field: strPtr(\"value\"), // comment`. The
        VERIFIED comment follows the ListModelsCommand precedent (builtin.go:47): inline trailing comment
        recording date + command + FR ref."
  gotcha: "(1) Insert BETWEEN ProviderFlag and BareFlags (PRD §12.1 ordering; S1's struct slot). (2) Use
        strPtr (the same-package helper, manifest.go) — NOT &s or a literal pointer. (3) gofmt handles Go
        struct alignment; do NOT touch other lines' spacing. (4) Do NOT add SessionMode to ANY other builtin
        function (builtinClaude/Gemini/etc.) — they ship nil."

# The file under edit #2 (the on-disk reference doc — read by a parity test)
- file: providers/pi.toml
  why: "EDIT: insert a `# --- session continuation (multi-turn fallback, §9.24) ---` section + the
        `session_mode = \"append\"` line between the `# --- sub-provider ---` block (provider_flag) and the
        `# --- bare mode ---` block (bare_flags). providers/pi.toml is reference documentation that mirrors
        builtinPi() byte-for-byte (modulo comments); TestProviderReferenceFiles_DecodeParity reads THIS file
        from disk and DeepEquals it against builtinPi()."
  pattern: "Match the existing section style: a `# --- <topic> (PRD ref) ---` header comment line, then the
        key=value line with an inline `# comment` aligned to the existing column (e.g. provider_flag's
        comment column). The VERIFIED comment is `# VERIFIED 2026-07-05; FR-T9.` per fr-t9-verification.md."
  gotcha: "(1) providers/pi.toml is hand-aligned RAW TEXT — gofmt does NOT touch it; match the inline-comment
        column manually. (2) Position is BETWEEN provider_flag and bare_flags (PRD §12.1). (3) This file is
        read by TestProviderReferenceFiles_DecodeParity — the decoded Manifest must equal builtinPi(), so the
        session_mode value here MUST be \"append\" (matching builtin.go) or that test fails. (4) Do NOT touch
        any other provider's .toml (claude.toml etc.) — they ship without session_mode."

# The file under edit #3 (MANDATORY — the test-fixture mirror of pi.toml)
- file: internal/provider/builtin_test.go
  why: "EDIT the `piTOML` const (line 16): add `session_mode = \"append\"` between `provider_flag =
        \"--provider\"` and `bare_flags = [`. This const is the decode-parity ORACLE: TestBuiltinManifests_
        DecodeParity does reflect.DeepEqual(builtinPi(), decode(piTOML)). It is a SEPARATE hand-maintained
        copy of pi's manifest (NOT an embed/read of providers/pi.toml) — so it must be synced by hand."
  pattern: "piTOML is a raw string literal `const piTOML = \\`...\\``. Add the line in the same terse
        one-key-per-line style as the surrounding fields (no comment needed in the test oracle, but a brief
        `# FR-T9` is fine). Position between provider_flag and bare_flags to mirror providers/pi.toml."
  gotcha: "CRITICAL — THIS IS THE #1 ONE-PASS FAILURE MODE. The work item names only builtin.go +
        providers/pi.toml, but omitting the piTOML sync makes TestBuiltinManifests_DecodeParity FAIL
        (builtinPi().SessionMode becomes non-nil 'append', but decode(piTOML).SessionMode stays nil →
        reflect.DeepEqual mismatch). The piTOML const ALSO contains a pre-existing stale
        `default_provider = \"\"` line (a v3-removed field) between provider_flag and bare_flags — LEAVE IT
        UNTOUCHED (out of scope; it decodes consistently today). Insert session_mode adjacent to it."

# RECOMMENDED (not required for green)
- file: internal/provider/builtin_test.go  # TestBuiltinManifests_PiFields (line 256)
  why: "RECOMMENDED: add `assertStr(t, \"SessionMode\", m.SessionMode, \"append\")` to the pi field-
        enumeration test. PiFields uses per-field asserts (NOT DeepEqual), so the build is green WITHOUT
        this — but adding it covers the new field and documents that pi is the lone append provider. Place
        it after the ProviderFlag assert (line ~265), before the BareFlags block."

# Verified landed state (READ-ONLY — do NOT edit in S4)
- file: internal/provider/manifest.go
  why: "READ-ONLY (S1 landed). SessionMode *string field + Resolve strPtr('') default + Validate ''|'append'
        enum. S4 only SETS the value on pi's builtin; it does not touch the schema. Confirm via:
        grep -n 'SessionMode' internal/provider/manifest.go"
- file: internal/provider/merge.go
  why: "READ-ONLY (S2). The MergeManifest SessionMode clause (config-overridable). S4's shipped 'append' is
        the BASE a user override merges onto; a user may set session_mode='' in [provider.pi] to disable
        multi-turn for pi (S2's semantics). S4 does not edit merge.go."
- file: internal/provider/render.go
  why: "READ-ONLY (S3, parallel). RenderMultiTurn's gate reads *r.SessionMode. S4's value is what makes it
        pass for pi. S4 does not edit render.go."

# PRD authority (already in the selected content)
- prd: PRD.md §9.24 FR-T8 (session_mode field: "" default | "append"; pi ships "append"), FR-T9 (verification
        duty — NEVER speculative; record the verified flag set), FR-T1 condition (d) (multi-turn activates
        only if resolved manifest session_mode=="append"); §12.1 (session_mode field position: between
        provider_flag and bare_flags), §12.3 (pi is the lone verified-append provider).
  why: "FR-T8/T9 is the authority that pi ships 'append' and every other provider ships '' until verified.
        §12.1 pins the field ORDERING (provider_flag → session_mode → bare_flags). §12.3 confirms pi is the
        sole session-capable builtin in this revision."
  critical: "FR-T9 is WHY only pi changes: setting 'append' on an unverified provider is a spec violation,
        not a shortcut. The VERIFIED comment is the audit trail FR-T9 mandates."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── internal/provider/
│   ├── builtin.go             # EDIT: builtinPi() +SessionMode: strPtr("append") (pi ONLY)
│   ├── builtin_test.go        # EDIT: piTOML const +session_mode="append" (MANDATORY sync); +PiFields assert (recommended)
│   ├── manifest.go            # READ-ONLY (S1 landed): SessionMode *string + Resolve + Validate
│   ├── merge.go               # READ-ONLY (S2): MergeManifest SessionMode clause
│   ├── render.go              # READ-ONLY (S3, parallel): RenderMultiTurn gate consumes the value
│   └── referencefiles_test.go # READ-ONLY: TestProviderReferenceFiles_DecodeParity reads providers/pi.toml
└── providers/
    └── pi.toml                # EDIT: +session_mode="append" between provider_flag and bare_flags
```

### Desired Codebase Tree After S4

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/provider/builtin.go        # builtinPi() +SessionMode (pi ONLY)
    internal/provider/builtin_test.go   # piTOML const +session_mode="append"; +PiFields assert (recommended)
    providers/pi.toml                   # +session_mode="append" (between provider_flag and bare_flags)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/provider/builtin.go` | MODIFY (1 line) | `builtinPi()`: add `SessionMode: strPtr("append"),` + VERIFIED comment, between ProviderFlag and BareFlags. |
| `providers/pi.toml` | MODIFY (1 section) | Add `session_mode = "append"` + `# VERIFIED 2026-07-05; FR-T9.` between provider_flag and bare_flags. |
| `internal/provider/builtin_test.go` | MODIFY (1 line + optional 1 assert) | `piTOML` const: add matching `session_mode = "append"`. *(Recommended)* PiFields: +SessionMode assert. |

**Explicitly NOT touched**: `manifest.go` (S1), `merge.go` (S2), `render.go`/`render_test.go` (S3), the
other 7 builtin functions in builtin.go (claude/gemini/opencode/codex/cursor/agy/qwen-code), the other 7
`*TOML` test consts (claudeTOML etc.), the other 7 `providers/*.toml` files, `referencefiles_test.go`,
docs/* (S5 owns providers.md/configuration.md session_mode doc), `multiturn.go`/`generate.go` (P1.M1.T3),
any other package, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (THREE copies, not two — the #1 one-pass failure mode): the work item names builtin.go +
// providers/pi.toml, but TestBuiltinManifests_DecodeParity (builtin_test.go:396) does
// reflect.DeepEqual(builtinPi(), decode(piTOML)) where piTOML is a SEPARATE hand-maintained const
// (builtin_test.go:16). Adding SessionMode to builtinPi() makes its SessionMode non-nil "append"; for the
// DeepEqual to pass, decode(piTOML).SessionMode must ALSO be non-nil "append" ⇒ piTOML MUST contain
// `session_mode = "append"`. OMITTING the piTOML sync is the single most likely way this task fails. The
// on-disk providers/pi.toml is covered by a SECOND guard (TestProviderReferenceFiles_DecodeParity) which
// reads the file at test time — that one is satisfied by editing providers/pi.toml itself. Sync ALL THREE.

// CRITICAL (pi ONLY — FR-T9): set SessionMode on builtinPi() ALONE. Do NOT add it to builtinClaude() /
// builtinGemini() / builtinOpenCode() / builtinCodex() / builtinCursor() / builtinAgy() / builtinQwenCode().
// Those ship nil (Resolve() ⇒ "") because their append-turn mechanisms are NOT verified (fr-t9-verification.md
// covers pi only). Setting "append" on an unverified provider violates FR-T9 and would silently enable
// multi-turn for a provider whose rendering is unconfirmed.

// CRITICAL (strPtr, not a bare pointer): SessionMode is *string (S1). Set it via `strPtr("append")` — the
// same-package helper (manifest.go). NOT `&"append"` (invalid Go) and NOT a plain string. This matches S1's
// own TestResolve_PreservesExplicitValues which uses strPtr("append").

// GOTCHA (placement = PRD §12.1 ordering): in builtin.go the line goes between ProviderFlag and BareFlags;
// in providers/pi.toml and piTOML it goes between provider_flag and bare_flags. S1's research (research-
// provider.md §1) pinned this slot; PRD §12.1 lists the field order as ...provider_flag, session_mode,
// bare_flags... Do NOT place it elsewhere (e.g. near output) — the ordering is part of the manifest contract.

// GOTCHA (the stale default_provider line in piTOML): the piTOML const (builtin_test.go:16) contains a
// pre-existing `default_provider = ""` line between provider_flag and bare_flags. This is a v3-REMOVED field
// (the inference backend is now the model slash-prefix, FR-R5b); it decodes consistently with builtinPi()
// TODAY (the parity test passes). LEAVE IT UNTOUCHED — removing/altering it is out of scope for S4. Insert
// session_mode adjacent to it (before or after; either decodes correctly since field order in TOML is not
// significant for the DeepEqual).

// GOTCHA (gofmt vs raw strings): gofmt realigns the Go struct in builtin.go automatically after you add the
// line — do NOT hand-align the struct's colon/value column. But providers/pi.toml and the piTOML const are
// RAW STRINGS — gofmt does NOT touch their interiors. Hand-match the inline-comment column in providers/pi.toml
// to the surrounding lines (cosmetic, but the file is reference documentation). The piTOML test oracle's
// formatting is irrelevant to the DeepEqual (only key=value decoding matters).

// GOTCHA (TestBuiltinManifests_Validate stays green): S1's Validate enum accepts "" | "append" (nil passes).
// pi's "append" is valid ⇒ TestBuiltinManifests_Validate (which calls Validate() on all 8 builtins) stays
// green. The other 7 builtins' SessionMode is nil ⇒ passes. No Validate change needed (S1 already landed it).

// GOTCHA (TestBuiltinManifests_PiFields stays green WITHOUT a new assert): PiFields uses per-field assertStr
// calls, NOT a DeepEqual. It does not currently assert SessionMode, so adding the field does NOT break it.
// Adding a positive SessionMode assert is RECOMMENDED (covers the field) but NOT required for green.

// GOTCHA (S2 merge makes the shipped value a BASE, not a lock): a user can override pi's session_mode to ""
// in [provider.pi] (S2's MergeManifest clause: non-nil override wins). S4 ships "append" as the default;
// S2 lets a user opt pi out of multi-turn. Do NOT add logic to prevent that — it's intended FR-37a semantics.
```

## Implementation Blueprint

### Data models and structure

None. S4 sets a value on an existing field (S1's `Manifest.SessionMode *string`). No new types, no schema
change. The "model" is three text lines (one Go field assignment, two TOML key lines).

### The EXACT target text (ready to paste)

**Edit 1 — `internal/provider/builtin.go`, `builtinPi()`** (insert between the `ProviderFlag:` and
`BareFlags:` lines):

```go
		ProviderFlag:      strPtr("--provider"),
		SessionMode:       strPtr("append"), // VERIFIED 2026-07-05 via `pi --session-id X <isolation-flags-minus-no-session> -p "remember BANANA"` then recall returns BANANA; FR-T9.
		BareFlags: []string{
```

> gofmt will realign the `:`/value column after insertion (the `SessionMode:` line's spacing above is
> approximate — run `gofmt -w` and it snaps to the file's column). The VERIFIED comment wording is the
> contract (matches fr-t9-verification.md + the item description + the ListModelsCommand FR-D5 precedent
> at builtin.go:47).

**Edit 2 — `providers/pi.toml`** (insert a new section between the `# --- sub-provider ---` block and the
`# --- bare mode ---` block):

```toml
# --- session continuation (multi-turn fallback, §9.24) ---
session_mode = "append"             # VERIFIED 2026-07-05; FR-T9. pi re-invoking the SAME --session-id
                                    # appends a recallable turn (one-shot → multi-turn available). Other
                                    # providers ship "" until their append mechanism is verified (FR-T9).
```

> Hand-align the `#` comment column to the surrounding lines (e.g. `provider_flag`'s column). The
> `# VERIFIED 2026-07-05; FR-T9.` token is the contract; the two trailing explanation lines mirror
> providers/pi.toml's existing multi-line comment style.

**Edit 3 — `internal/provider/builtin_test.go`, `piTOML` const** (insert between the `provider_flag` and
`bare_flags` lines — note the pre-existing stale `default_provider` line sits here too):

```toml
provider_flag = "--provider"
default_provider = ""
session_mode = "append"
bare_flags = [
```

> The `default_provider = ""` line is PRE-EXISTING and stale (v3-removed); leave it. Add only the
> `session_mode = "append"` line. No comment needed in the test oracle (it's a decode fixture, not docs);
> a trailing `# FR-T9` is acceptable but optional.

*(Recommended)* **Edit 4 — `TestBuiltinManifests_PiFields`** (insert after the `ProviderFlag` assert,
before the BareFlags block):

```go
	assertStr(t, "SessionMode", m.SessionMode, "append") // FR-T9 verified: pi is the lone session-capable builtin
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: builtin.go — add SessionMode to builtinPi() (pi ONLY)
  - LOCATE: internal/provider/builtin.go, func builtinPi() (~line 30). Find the `ProviderFlag: strPtr("--provider"),`
    line and the immediately-following `BareFlags: []string{` line.
  - INSERT (between them): `SessionMode:       strPtr("append"), // VERIFIED 2026-07-05 via \`pi --session-id X
    <isolation-flags-minus-no-session> -p "remember BANANA"\` then recall returns BANANA; FR-T9.` (exact text
    above; gofmt fixes the column).
  - VERIFY strPtr is the helper (manifest.go) — same package, no import needed.
  - DO NOT: add SessionMode to builtinClaude/Gemini/OpenCode/Codex/Cursor/Agy/QwenCode (FR-T9 — only pi is
    verified). DO NOT: edit any other line in builtinPi() or any other function. DO NOT: hand-align the
    struct (gofmt does it).

Task 2: providers/pi.toml — add the session_mode section (between provider_flag and bare_flags)
  - LOCATE: providers/pi.toml. Find the `# --- sub-provider ---` block (provider_flag = "--provider") and the
    next `# --- bare mode ---` block (bare_flags = [).
  - INSERT the 4-line block above (`# --- session continuation ---` header + session_mode + 2 explanation
    lines) between them.
  - HAND-ALIGN the `#` comment column to the surrounding lines (raw text — gofmt won't touch it).
  - DO NOT: edit any other provider's .toml. DO NOT: remove the `# default_provider removed in v3` comment
    (it's accurate documentation). DO NOT: touch the absent-fields comment block at the bottom.

Task 3: builtin_test.go — sync the piTOML const (MANDATORY)
  - LOCATE: internal/provider/builtin_test.go, `const piTOML = \`...\`` (line 16). Find `provider_flag =
    "--provider"` and the `bare_flags = [` line (with the pre-existing `default_provider = ""` between them).
  - INSERT `session_mode = "append"` between them (adjacent to default_provider — before or after, either
    decodes correctly).
  - DO NOT: remove/alter the stale `default_provider = ""` line (pre-existing, out of scope). DO NOT: touch
    the other *TOML consts (claudeTOML etc.). DO NOT: add session_mode to them.
  - WHY MANDATORY: TestBuiltinManifests_DecodeParity (line 396) does reflect.DeepEqual(builtinPi(),
    decode(piTOML)). After Task 1, builtinPi().SessionMode is non-nil "append"; without this sync,
    decode(piTOML).SessionMode is nil → DeepEqual FAILS.

Task 4 (RECOMMENDED, not required): builtin_test.go — add the SessionMode assert to PiFields
  - LOCATE: TestBuiltinManifests_PiFields (line 256). Find the `assertStr(t, "ProviderFlag", ...)` line.
  - INSERT after it: `assertStr(t, "SessionMode", m.SessionMode, "append") // FR-T9 verified: pi is the lone
    session-capable builtin`.
  - WHY RECOMMENDED: covers the new field in the enumeration test. NOT required for green (PiFields is
    per-field assertStr, not DeepEqual — omitting does not fail).

Task 5: VALIDATE
  - RUN: gofmt -w internal/provider/builtin.go internal/provider/builtin_test.go
  - RUN: go build ./... ; go vet ./... ; go test -race ./...
  - RUN the two parity guards specifically (THE green gate for this task):
        go test -race -run 'TestBuiltinManifests_DecodeParity|TestProviderReferenceFiles_DecodeParity' ./internal/provider/ -v
  - RUN the field/validate tests:
        go test -race -run 'TestBuiltinManifests_PiFields|TestBuiltinManifests_Validate' ./internal/provider/ -v
  - GREP: confirm pi has it, the other 7 don't, and the VERIFIED comments are present:
        grep -n "SessionMode" internal/provider/builtin.go                       # → 1 match (builtinPi ONLY)
        grep -n "session_mode" providers/pi.toml internal/provider/builtin_test.go # → 2 matches (pi.toml + piTOML)
        grep -n "VERIFIED 2026-07-05; FR-T9" providers/pi.toml                    # → 1 match
        grep -n "VERIFIED 2026-07-05 via" internal/provider/builtin.go            # → 1 match (the SessionMode line)
  - CONFIRM scope: git diff --stat -- internal/ pkg/ cmd/ docs/ providers/   # → 3 files ONLY.
  - FIX-FORWARD: if TestBuiltinManifests_DecodeParity fails on pi, the piTOML const (Task 3) was not synced.
    If TestProviderReferenceFiles_DecodeParity fails on pi, providers/pi.toml (Task 2) is missing/mismatched.
```

### Implementation Patterns & Key Details

```go
// === builtin.go — the EXACT insert (between ProviderFlag and BareFlags in builtinPi()) ===
// BEFORE (current):                       AFTER (S4):
//   ProviderFlag: strPtr("--provider"), //   ProviderFlag: strPtr("--provider"),
//   BareFlags: []string{               //   SessionMode:  strPtr("append"), // VERIFIED 2026-07-05 via `pi --session-id X <isolation-flags-minus-no-session> -p "remember BANANA"` then recall returns BANANA; FR-T9.
//                                        //   BareFlags: []string{
//
// (gofmt realigns the colon/value column after insertion — the struct is Go source, gofmt owns its layout.)

// === providers/pi.toml — the section insert (between the sub-provider and bare-mode blocks) ===
// # --- sub-provider ---
// provider_flag = "--provider"        # pi routes to backends ...
// # default_provider removed in v3: ...
//
// # --- session continuation (multi-turn fallback, §9.24) ---        ← NEW (4 lines)
// session_mode = "append"             # VERIFIED 2026-07-05; FR-T9. pi re-invoking the SAME --session-id  ← NEW
//                                     # appends a recallable turn (...). Other providers ship "" ...        ← NEW
//
// # --- bare mode (§12.7.1: explicit tool-disable switch) ---
// bare_flags = [

// === builtin_test.go — the piTOML const sync (MANDATORY) ===
// provider_flag = "--provider"
// default_provider = ""          ← PRE-EXISTING stale line; LEAVE IT
// session_mode = "append"        ← NEW (Task 3)
// bare_flags = [
```

```go
// === The VERIFIED comment is the FR-T9 audit trail (do NOT reword) ===
// The comment records WHAT was verified, WHEN, and HOW — exactly as fr-t9-verification.md specifies and as
// the FR-D5 discipline (builtin.go:47 ListModelsCommand) mandates. It is the proof FR-T9 requires that the
// "append" value is not speculative. Rewording it (or dropping the date/command/FR-ref) breaks the audit
// trail even though the code still compiles. Use the contract wording verbatim.
```

### Integration Points

```yaml
PROVIDER REGISTRY (internal/provider):
  - builtin.go builtinPi(): SessionMode now strPtr("append") — the shipped default for pi.
  - Resolve() (manifest.go, S1): nil → strPtr("") default. pi's non-nil "append" is preserved through Resolve.
  - MergeManifest (merge.go, S2): a user [provider.pi] override with session_mode="" wins (disables multi-turn
    for pi); session_mode absent inherits the shipped "append". S4's value is the BASE.
  - RenderMultiTurn (render.go, S3): gate `*r.SessionMode != "append"` → error. After S4, pi passes the gate.

REFERENCE DOC SYNC (the byte-faithfulness invariant):
  - providers/pi.toml mirrors builtinPi() (TestProviderReferenceFiles_DecodeParity reads it at test time).
  - piTOML const (builtin_test.go) mirrors builtinPi() (TestBuiltinManifests_DecodeParity decodes it).
  - BOTH must carry session_mode="append" or their reflect.DeepEqual against builtinPi() fails.

NO-TOUCH (explicitly — owned by sibling/later subtasks):
  - internal/provider/manifest.go            # S1 (LANDED): the SessionMode field + Resolve + Validate
  - internal/provider/merge.go               # S2: the MergeManifest clause
  - internal/provider/render.go + render_test.go   # S3 (parallel): RenderMultiTurn + its gate
  - the other 7 builtin functions in builtin.go    # claude/gemini/opencode/codex/cursor/agy/qwen-code (FR-T9: unverified)
  - the other 7 *TOML consts + providers/*.toml    # they ship WITHOUT session_mode
  - docs/* (providers.md, configuration.md)        # S5: the session_mode user-facing doc rides with S5
  - multiturn.go / generate.go                     # P1.M1.T3: the N+1 turn protocol that benefits from S4's value
  - any other package; PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S4):
  - P1.M1.T3.S2 (the turn protocol): its trigger gate (FR-T1 condition d) reads the resolved pi manifest's
    SessionMode; S4's "append" is what makes condition (d) true for pi. Before S4 the protocol never fires.
  - S5 (docs): documents session_mode in providers.md/configuration.md. S4 ships the VALUE + the VERIFIED
    audit comment; S5 writes the prose. Do not write the prose in S4 (contract point 5: "DOCS: none").
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/provider/builtin.go internal/provider/builtin_test.go   # realigns the Go struct
gofmt -l .                       # Expected: empty after the -w (providers/pi.toml is not Go — unaffected)
go vet ./internal/provider/...   # Expected: exit 0
go build ./...                   # Expected: exit 0
```

### Level 2: Unit Tests — the TWO parity guards are the green gate

```bash
cd /home/dustin/projects/stagecoach

# THE green gate for this task — the two reflect.DeepEqual decode-parity guards:
go test -race -run 'TestBuiltinManifests_DecodeParity|TestProviderReferenceFiles_DecodeParity' ./internal/provider/ -v
# Expected: PASS. If pi fails: piTOML const (Task 3) or providers/pi.toml (Task 2) is missing/mismatched
#           session_mode vs builtinPi()'s SessionMode. These are the load-bearing assertions.

# Field enumeration + Validate (pi's "append" is a valid enum value; the other 7 stay nil):
go test -race -run 'TestBuiltinManifests_PiFields|TestBuiltinManifests_Validate' ./internal/provider/ -v
# Expected: PASS. (PiFields stays green even without the recommended Task-4 assert.)

# Full provider suite (proves no other builtin/regression broke):
go test -race ./internal/provider/ -v
```

### Level 3: Whole-Repository Regression + the contract's grep verification

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages green
go vet ./...                     # Expected: exit 0

# === The contract's verification: pi declares append; every other provider ships "" ===
grep -n "SessionMode" internal/provider/builtin.go
#   Expected: 1 match — the builtinPi() line ONLY. (If >1 match, another builtin was edited — revert it.)

# === All THREE copies carry session_mode="append" ===
grep -n "session_mode" internal/provider/builtin_test.go providers/pi.toml
#   Expected: 2 matches — piTOML const + providers/pi.toml.

# === The VERIFIED audit comments are present (FR-T9 trail) ===
grep -n "VERIFIED 2026-07-05 via" internal/provider/builtin.go     # → 1 (the SessionMode line)
grep -n "VERIFIED 2026-07-05; FR-T9" providers/pi.toml             # → 1

# === The other 7 builtins are UNTOUCHED (SessionMode absent) ===
grep -n "SessionMode" internal/provider/builtin.go | wc -l         # → 1
# (builtinClaude/Gemini/OpenCode/Codex/Cursor/Agy/QwenCode must NOT appear in this grep.)

# === Confirm ONLY the 3 intended files changed ===
git diff --stat -- internal/ pkg/ cmd/ docs/ providers/
#   Expected: internal/provider/builtin.go + internal/provider/builtin_test.go + providers/pi.toml ONLY.
```

### Level 4: Behavioral Smoke (the gate S4's value opens — via S3's renderer, if S3 has landed)

```bash
cd /home/dustin/projects/stagecoach

# S4's value is observable ONLY through the capability gate S3 (parallel) added to RenderMultiTurn.
# If S3 has landed, this proves S4's value reaches the gate and passes it for pi (and only pi):
cat > internal/provider/zz_s4_smoke_test.go <<'EOF'
package provider
import "testing"
func TestZZ_S4_PiGateOpenOthersClosed(t *testing.T) {
	// pi: SessionMode == "append" ⇒ S3's RenderMultiTurn gate passes for pi.
	pi := builtinPi()
	if pi.SessionMode == nil || *pi.SessionMode != "append" {
		t.Fatalf("pi SessionMode = %v, want non-nil *\"append\"", pi.SessionMode)
	}
	// every other builtin: SessionMode nil ⇒ gate would error (multi-turn unavailable, FR-T1 cond d false).
	for _, name := range []string{"claude","gemini","opencode","codex","cursor","agy","qwen-code"} {
		m := BuiltinManifests()[name]
		if m.SessionMode != nil {
			t.Errorf("%s SessionMode = %v, want nil (FR-T9: unverified ⇒ ships \"\")", name, m.SessionMode)
		}
	}
	t.Log("S4 OK: pi gate open (append), all others closed (nil) ✅")
}
EOF
go test -run TestZZ_S4_PiGateOpenOthersClosed -v ./internal/provider/ ; rm -f internal/provider/zz_s4_smoke_test.go
# Expected: PASS. (This is the end-to-end proof S4's value is correct AND scoped to pi alone. If S3 has NOT
#           landed yet, this still passes — it asserts the shipped VALUES, not S3's renderer. Safe to run
#           regardless of S3's state.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.
- [ ] `TestBuiltinManifests_DecodeParity` PASSES (the piTOML const synced — Task 3).
- [ ] `TestProviderReferenceFiles_DecodeParity` PASSES (providers/pi.toml synced — Task 2).
- [ ] `grep -c "SessionMode" internal/provider/builtin.go` → 1 (builtinPi ONLY).

### Feature Validation

- [ ] `builtinPi().SessionMode` is non-nil `*("append")` with the `VERIFIED 2026-07-05 via ...` comment.
- [ ] `providers/pi.toml` has `session_mode = "append"` + `# VERIFIED 2026-07-05; FR-T9.` between
      `provider_flag` and `bare_flags`.
- [ ] The `piTOML` const (builtin_test.go) has the matching `session_mode = "append"` line.
- [ ] The other 7 builtins (`builtinClaude`…`builtinQwenCode`) have `SessionMode == nil` (UNTOUCHED).
- [ ] The other 7 `providers/*.toml` and `*TOML` consts have NO `session_mode` line (UNTOUCHED).

### Scope Discipline Validation

- [ ] ONLY `internal/provider/builtin.go` + `internal/provider/builtin_test.go` + `providers/pi.toml` change.
- [ ] Did NOT edit `manifest.go` (S1), `merge.go` (S2), `render.go`/`render_test.go` (S3).
- [ ] Did NOT add SessionMode to any builtin other than pi (FR-T9: only pi is verified).
- [ ] Did NOT remove the pre-existing stale `default_provider = ""` line in the piTOML const (out of scope).
- [ ] Did NOT write docs (providers.md/configuration.md session_mode prose rides with S5).
- [ ] Did NOT implement the turn protocol / `multiturn.go` (P1.M1.T3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] The VERIFIED comment uses the contract wording verbatim (the FR-T9 audit trail — date + command + FR ref).
- [ ] The `session_mode` line is positioned per PRD §12.1 (between provider_flag and bare_flags) in all 3 places.
- [ ] The providers/pi.toml section matches the file's existing `# --- <topic> ---` + inline-comment style.
- [ ] `strPtr("append")` is used (the same-package helper), not a bare pointer.
- [ ] *(Recommended)* `TestBuiltinManifests_PiFields` has a `SessionMode` positive assertion.

---

## Anti-Patterns to Avoid

- ❌ Don't sync only builtin.go + providers/pi.toml. The `piTOML` const (builtin_test.go:16) is a SEPARATE
  hand-maintained decode oracle; `TestBuiltinManifests_DecodeParity` does `reflect.DeepEqual(builtinPi(),
  decode(piTOML))`. Adding SessionMode to builtinPi() WITHOUT adding `session_mode = "append"` to piTOML
  makes that DeepEqual FAIL. This is the #1 one-pass failure mode — sync ALL THREE copies.
- ❌ Don't set SessionMode on any builtin other than pi. FR-T9 forbids a speculative "append": only pi's
  append-turn rendering is verified (fr-t9-verification.md). claude/gemini/opencode/codex/cursor/agy/
  qwen-code ship nil (Resolve ⇒ ""). Setting "append" on them would silently enable multi-turn for an
  unverified provider — a spec violation.
- ❌ Don't reword or drop the VERIFIED comment. It is the FR-T9 audit trail (date + command + FR ref),
  mandated by the FR-D5 verification-discipline precedent (builtin.go:47 ListModelsCommand). The value
  "append" is only shippable BECAUSE the comment records its proof. Use the contract wording verbatim.
- ❌ Don't use a bare pointer or plain string for SessionMode. It is `*string` (S1); set it via
  `strPtr("append")` (the same-package helper). `&"append"` is invalid Go; a plain `"append"` won't compile.
- ❌ Don't place session_mode outside the PRD §12.1 slot (between provider_flag and bare_flags). S1's
  research (research-provider.md §1) and the PRD both pin that ordering. Placing it near `output` or
  `bare_flags`'s tail deviates from the manifest contract.
- ❌ Don't remove the stale `default_provider = ""` line in the piTOML const. It is pre-existing (a
  v3-removed field) and decodes consistently today. "Cleaning it up" is out of scope and risks an unrelated
  test drift. Add session_mode adjacent to it; leave default_provider alone.
- ❌ Don't hand-align the Go struct in builtin.go. gofmt owns Go-source alignment — add the line, run
  `gofmt -w`, and the colon/value column snaps to the file's style. (providers/pi.toml and the piTOML const
  are raw strings — DO hand-match their comment columns cosmetically.)
- ❌ Don't edit `manifest.go` (S1), `merge.go` (S2), or `render.go` (S3). S4 only SETS the value on pi's
  builtin; the schema, the merge clause, and the renderer+gate are S1/S2/S3 respectively. S4 reads none of
  their internals — it just supplies the value their logic consumes.
- ❌ Don't write user-facing docs (providers.md/configuration.md) for session_mode. The contract (point 5)
  is explicit: "DOCS: none — the providers.md session_mode doc rides with P1.M1.T1.S5." S4 ships the VALUE
  + the inline VERIFIED audit comment; S5 writes the prose.
- ❌ Don't implement the multi-turn turn protocol, chunking, or the trigger gate — those are P1.M1.T3. S4
  is the single value-flip that makes P1.M1.T3's gate pass for pi. S4 produces no new logic.
- ❌ Don't add a RenderMultiTurn call or any multi-turn behavior test in S4. S3 owns the renderer; P1.M1.T3
  owns the protocol; P1.M1.T4 owns the integration tests. S4's validation is the decode-parity guards +
  the value-presence greps (and the optional Level-4 smoke that asserts the shipped VALUES, not S3's
  renderer).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a three-line value-flip with the EXACT target text quoted verbatim for all three edits
(one Go field assignment, two TOML lines), the insert points pinned to the surrounding verbatim context,
and the VERIFIED comment wording taken directly from the FR-T9 verification record + the FR-D5 precedent.
The single biggest risk — omitting the `piTOML` const sync (the work item names only two files, but TWO
`reflect.DeepEqual` decode-parity guards force the third edit) — is documented as the #1 failure mode in
FIVE places (description, Gotchas, Task 3, Validation, Anti-Patterns) with the exact reason (`builtinPi()`
becomes non-nil "append" while `decode(piTOML)` stays nil ⇒ mismatch). Four independent de-riskings: (1) S1
is ALREADY landed (SessionMode *string + Resolve default + Validate enum — verified), so the value compiles
and passes Validate TODAY; (2) the FR-T9 verification is DONE (2026-07-05 live run recorded), so "append"
is shippable with its audit comment — not speculative; (3) the sibling tasks' boundaries are explicit (S4
sets the VALUE; S1 schema / S2 merge / S3 renderer are untouched); (4) S3 is code-independent of S4 (S3's
unit tests set SessionMode in literals), so the parallel execution is not a risk. The only residual
uncertainty (not 10/10) is whether the implementer adds the RECOMMENDED PiFields assert (Task 4 — not
required for green) and the exact providers/pi.toml comment phrasing (cosmetic; the `# VERIFIED 2026-07-05;
FR-T9.` token is the contract, the surrounding explanation is taste). No logic, no rendering, no precedence
is touched — the blast radius is three text additions and the two parity guards that pin them.
