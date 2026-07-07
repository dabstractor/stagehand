---
name: "P1.M1.T1.S5 — Render unit tests (golden/table) + Mode A provider docs (providers.md, configuration.md)"
description: |
  Two deliverables, both landing in S5 (S4's contract point 5 deferred ALL docs here; S3 landed the
  renderer + 4 of its tests, leaving one coverage gap + the table-driven idiom).

  (A) TESTS — internal/provider/render_test.go: S3 (COMPLETE) already shipped 4 individual-function tests
  covering item cases (a) turn-1, (b) turn-2-no-sys, (d) non-append-errors, (e) immutability
  (render_test.go:503/537/562/589, all reusing the mtPiManifest() helper at :487). S5 adds ONLY:
    (1) TestRenderMultiTurn_GoldenTable — a cases-slice table-driven test (the §5 idiom + the item TITLE's
        "golden/table") consolidating turn-1 + turn-2 as cases, asserting PRESENCE via containsPair/
        containsToken (NOT byte-exact DeepEqual — that would duplicate S3; the item says "assert presence +
        the system-prompt turn-1-only distinction", "Do NOT assert exact arg position").
    (2) TestRenderMultiTurn_SessionIDStableAcrossTurns — the ONE genuine coverage gap: S3 uses the same
        sessionID literal per turn but never EXPLICITLY asserts the id renders identically across turns.
        Call RenderMultiTurn for turn 1 AND turn 2 (and turn 3) with one sessionID; assert
        containsPair(spec.Args, "--session-id", sessionID) is true for EVERY turn (case c).
  Do NOT re-add (a)/(b)/(d)/(e) — S3 has them. Do NOT widen Render. NO new files unless preferring
  render_multiturn_test.go (render_test.go is recommended — same package, reuses mtPiManifest()).

  (B) DOCS — Mode A, the larger deliverable (entirely undone; S4 deferred it here):
    (i) docs/providers.md manifest-schema section is STALE: lists "21-field schema" (line 3) + "21 fields"
        (line 13) + a schema table with NO session_mode row (S1 added SessionMode to manifest.go:66 → the
        struct now has 22 fields). Edit: fix both counts to 22; insert a `| session_mode |` row between
        `provider_flag` (line 28) and `bare_flags` (line 29, PRD §12.1 ordering); add a prose note that
        ONLY pi declares "append" today (VERIFIED 2026-07-05 per FR-T9 — architecture/fr-t9-verification.md),
        every other built-in ships "", and adding another provider requires the FR-T9 empirical
        append-turn verification bar; cross-link §9.24.
    (ii) docs/configuration.md `[provider.<name>]` surface: configuration.md does NOT have a per-field row
        table for providers (that lives in providers.md); it surfaces MERGE SEMANTICS at line 22
        ("fields are merged onto the built-in manifest ... present values override, absent values inherit").
        Add a short note near line 22 that session_mode is config-overridable per S2: an explicit
        session_mode = "" on pi DISABLES multi-turn fallback (overrides the built-in "append"); omitting
        inherits the built-in; setting "append" on a non-append provider is a user override at their own
        FR-T9 risk. Cross-link providers.md#the-schema. Mirrors how output/strip_code_fence override
        semantics are discussed (configuration.md:143).

  NO production Go code changes (render.go is S3/LANDED; manifest.go is S1/LANDED; merge.go is S2/LANDED;
  builtin.go/pi.toml is S4/parallel). S5 = tests + docs ONLY.
---

## Goal

**Feature Goal**: (1) Complete the `RenderMultiTurn` unit-test coverage in the codebase's documented
"golden/table" idiom — adding the table-driven cases-slice test the item title requests AND filling the
one genuine coverage gap S3 left (cross-turn session-ID stability, item case c) — WITHOUT duplicating the
4 individual tests S3 already shipped (cases a/b/d/e). (2) Land the deferred Mode A documentation for the
`session_mode` manifest field: fix the stale providers.md schema (21→22 fields, add the missing
`session_mode` row + the pi-only-verified-append note + the FR-T9 verification bar + §9.24 cross-link) and
document the S2 config-override semantics in configuration.md.

**Deliverable**:
- **Tests**: 2 new test functions appended to `internal/provider/render_test.go`
  (`TestRenderMultiTurn_GoldenTable`, `TestRenderMultiTurn_SessionIDStableAcrossTurns`), reusing the
  existing `mtPiManifest()` helper (render_test.go:487) and `containsPair`/`containsToken` helpers (:606/:616).
- **Docs**: edits to `docs/providers.md` (2 count fixes + 1 schema-table row + 1 prose note) and
  `docs/configuration.md` (1 override-semantics note + cross-link). No new files.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green
— specifically the 2 new tests pass AND S3's 4 `TestRenderMultiTurn_*` tests stay green (proving no
regression). `grep -n "session_mode" docs/providers.md` returns the new schema row + note (≥2 matches);
`grep -n "session_mode" docs/configuration.md` returns the override note (≥1 match); providers.md says
"22-field"/"22 fields" (not 21). S3's `RenderMultiTurn` (render.go:203), S1's manifest field
(manifest.go:66), S2's merge clause (merge.go), and S4's pi value (builtin.go/providers/pi.toml) are all
UNTOUCHED. `git diff --stat` shows ONLY `internal/provider/render_test.go` + `docs/providers.md` +
`docs/configuration.md`.

## User Persona

**Target User**: (1) The contributor who next touches the multi-turn render path (P1.M1.T3.S2 the N+1
turn protocol; P1.M1.T4 the integration tests) — the table test is the readable contract surface they'll
scan first, and the cross-turn stability test is the regression net for "the session id must be identical
on every turn." (2) The end user / contributor reading the docs to understand which providers support the
lossless multi-turn fallback (§9.24) and how to disable it for pi via config.

**Use Case**: A reader opens providers.md to see the manifest schema and immediately understands
`session_mode` exists, that only pi is verified-append today, and that adding another provider is gated on
a real FR-T9 verification — not a guess. A pi user drops `[provider.pi] session_mode = ""` into
`.stagecoach.toml` to force one-shot+rescue; configuration.md now tells them that works (S2 semantics).

**Pain Points Addressed**: Without S5, providers.md claims "21 fields" and omits `session_mode` entirely
(stale since S1 landed) — a reader has no idea the field exists or that only pi supports multi-turn.
configuration.md never mentions session_mode is overridable. And the only RenderMultiTurn tests are 4
individual functions with no consolidated golden-table view and no explicit cross-turn-id assertion.

## Why

- **Closes the docs gap S4 deliberately deferred.** S4's contract point 5 is explicit: "DOCS: none — the
  providers.md session_mode doc rides with P1.M1.T1.S5." S4 shipped the VALUE + the inline VERIFIED comment;
  S5 writes the user-facing PROSE. Without S5 the field is undocumented in the reference docs.
- **Fixes the providers.md staleness introduced by S1.** S1 added `Manifest.SessionMode` (manifest.go:66),
  making the struct 22 fields; providers.md still says 21 and has no `session_mode` row. A schema table
  that omits a real field is a documentation defect. S5 is the docs sync.
- **Completes the test surface the item title requests ("golden/table").** research-tests-ui.md §5 names
  `TestRender_GoldenPerProvider` (render_test.go:30) as the table-driven idiom. S3 shipped individual
  functions; S5 ships the consolidated cases-slice form. It is complementary (presence-based, not
  byte-exact), not duplicative.
- **Fills the one real coverage gap.** S3's turn-1 and turn-2 tests each pin a single turn's args; neither
  EXPLICITLY asserts the session-ID string is identical across turns (case c). That invariant — "the
  orchestrator mints ONE id and reuses it every turn" (FR-T6) — deserves its own regression test.
- **Lowest-risk change possible.** No production Go code. Two tests (pure data transformation —
  `RenderMultiTurn` performs no spawning; the sole side effect is `os.Environ()` for Env) and prose/doc
  edits. The tests reuse existing helpers; the docs reuse existing section/table conventions.

## What

### Tests (`internal/provider/render_test.go`)

Two new test functions appended after S3's `TestRenderMultiTurn_DoesNotMutateManifest` (render_test.go:589,
before the `// helpers` divider at :601). Both reuse `mtPiManifest()` (:487) and the `containsPair`/`containsToken`
helpers. Neither asserts byte-exact args (that is S3's job; the item says "assert presence").

### Docs (`docs/providers.md` + `docs/configuration.md`)

- **providers.md**: line 3 "21-field" → "22-field"; line 13 "21 fields" → "22 fields"; insert a
  `| session_mode |` table row after the `provider_flag` row (line 28, before `bare_flags` line 29);
  add a short prose note/subsection stating only pi declares `"append"` (VERIFIED 2026-07-05, FR-T9),
  others ship `""`, and adding a provider requires the FR-T9 verification bar, cross-linking §9.24.
- **configuration.md**: near line 22 (the `[provider.<name>]` merge-semantics sentence), add a note that
  `session_mode` is config-overridable (explicit `""` disables multi-turn for pi; omit inherits; per S2),
  cross-linking providers.md#the-schema.

### Success Criteria

- [ ] `TestRenderMultiTurn_GoldenTable` exists in render_test.go, is a `cases` slice run under `t.Run`,
      asserts PRESENCE (containsPair/containsToken) for turn-1 (session-id present, no-session absent,
      system-prompt present, -p present) and turn-2 (system-prompt ABSENT, session-id present).
- [ ] `TestRenderMultiTurn_SessionIDStableAcrossTurns` exists and asserts `containsPair(args, "--session-id", id)`
      is true for turn 1 AND turn 2 (and turn 3) using ONE shared sessionID literal.
- [ ] S3's 4 `TestRenderMultiTurn_*` tests stay green (no re-add, no edit).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] providers.md schema table has a `session_mode` row between `provider_flag` and `bare_flags`.
- [ ] providers.md says "22-field schema" (line 3) and "Each manifest has 22 fields" (line 13).
- [ ] providers.md has a prose note: only pi = `"append"` (VERIFIED 2026-07-05, FR-T9), others `""`,
      adding a provider requires FR-T9 verification, cross-link §9.24.
- [ ] configuration.md has a note near line 22 documenting the session_mode override semantics (S2).
- [ ] ONLY `internal/provider/render_test.go` + `docs/providers.md` + `docs/configuration.md` change.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the EXACT current state: S3 already landed `RenderMultiTurn`
(render.go:203) + 4 tests (render_test.go:503/537/562/589) + the `mtPiManifest()` helper (:487) + the
`containsPair`/`containsToken` helpers (:606/:616) — so the implementer reuses them verbatim rather than
re-deriving. The two new tests are specified with their exact assertion shapes (presence-based, per the
item's "assert presence" guidance; cross-turn-id for case c). The docs insertion points are pinned to
exact line numbers (providers.md:3/13/28; configuration.md:22) with the exact replacement text. The
FR-T9 authority (the "VERIFIED 2026-07-05" wording) is quoted from architecture/fr-t9-verification.md.
The S2 override semantics (explicit "" disables, omit inherits) is quoted from P1M1T1S2/PRP.md. The
"don't duplicate S3 / don't touch production Go" boundary is stated in five places.

### Documentation & References

```yaml
# MUST READ — the test idiom S5 must follow (the §5 golden/table template)
- docfile: plan/009_5c53066d64b3/architecture/research-tests-ui.md
  why: "§5: 'internal/provider/render_test.go is golden/table-driven. TestRender_GoldenPerProvider (lines
        36–90) and TestRender_Pi_ByteForByteCommitPi (94–108) are the templates; helpers containsPair/
        containsToken (396–416) assert a flag-value pair or single token; TestRender_DoesNotMutateManifest
        (311–322) is the non-mutation regression guard. The pattern: a cases slice of {name, m, model,
        wantCmd, wantArgs, wantStdin} under t.Run.' This is the authority for S5's table-driven test shape."
  critical: "§5 is WHY S5 adds a cases-slice table test (the documented idiom) rather than another
        individual function. It is also the authority for using containsPair/containsToken (presence), NOT
        reflect.DeepEqual (byte-exact) in the table — matching the item's 'assert presence' + 'Do NOT
        assert exact arg position' guidance. (S3's individual tests already do byte-exact; S5's table is
        the complementary presence-based surface.)"

# MUST READ — the FR-T9 authority (the docs "VERIFIED 2026-07-05" wording)
- docfile: plan/009_5c53066d64b3/architecture/fr-t9-verification.md
  why: "Records the 2026-07-05 LIVE pi run proving append-turn recall (turn 1 'remember BANANA' → turn 2
        'BANANA'). The verified flag set: BareFlags MINUS --no-session, PLUS --session-id <id>;
        --system-prompt turn-1-only; -p kept; --continue/-c NOT used. Gives the EXACT wording for the
        providers.md note ('VERIFIED 2026-07-05; FR-T9') and the table test's expected presence."
  critical: "This file IS the FR-T9 verification the providers.md note cites. It is ALSO the authority that
        ONLY pi is verified — every other provider is absent from this file ⇒ ships ''. The note MUST state
        that adding another provider requires reproducing this kind of verification (the FR-T9 bar)."

# MUST READ — the S2 override semantics S5 documents in configuration.md
- docfile: plan/009_5c53066d64b3/P1M1T1S2/PRP.md
  why: "S2 LANDED the MergeManifest regime-1 clause `if override.SessionMode != nil {
        out.SessionMode = override.SessionMode }`. The semantics S5 documents: nil ⇒ inherit the built-in;
        non-nil (incl. explicit '') ⇒ override. The payoff use case: an explicit `[provider.pi]
        session_mode = ''` DISABLES multi-turn fallback for pi (overrides the built-in 'append'); omitting
        the key inherits 'append'."
  critical: "configuration.md's note MUST state these semantics verbatim (explicit '' disables for pi;
        omit inherits). This is the user-visible contract S2's clause implements. Do NOT understate it."

# MUST READ — the S3 contract (what is ALREADY landed; do NOT duplicate)
- docfile: plan/009_5c53066d64b3/P1M1T1S3/PRP.md
  why: "S3 LANDED RenderMultiTurn (render.go:203) + 4 tests: TestRenderMultiTurn_PiTurn1_Golden (case a,
        byte-exact + presence), TestRenderMultiTurn_PiTurn2_NoSysPromptFlag_NoPrepend (case b),
        TestRenderMultiTurn_NonAppendProviderErrors (case d), TestRenderMultiTurn_DoesNotMutateManifest
        (case e). S5 does NOT re-add a/b/d/e. S5 adds the table-driven consolidation (the item title's
        'golden/table') + case c (cross-turn session-ID stability — S3 never explicitly asserts it)."
  critical: "S3 is COMPLETE. S5's risk is DUPLICATION: re-asserting a/b/d/e as a table would be redundant
        (and a byte-exact table would brittlely duplicate S3's wantArgs). S5's table uses PRESENCE
        assertions only (complementary surface). S5 does NOT edit render.go or S3's 4 tests."

# The S4 contract (parallel) — the shipped pi value S5 documents
- docfile: plan/009_5c53066d64b3/P1M1T1S4/PRP.md
  why: "S4 (parallel) ships `SessionMode: strPtr('append')` on builtinPi + `session_mode = \"append\"` in
        providers/pi.toml + the VERIFIED comment. S5's docs reference this (pi is the lone append provider).
        S5 does NOT edit builtin.go/pi.toml/builtin_test.go."
  critical: "S5's providers.md note says 'only pi declares append today' — that is TRUE because of S4's
        shipped value. If S4 has not landed when S5 runs, the doc note is still correct as a statement of
        INTENT (the note describes the post-S4 state; the tests set SessionMode in literals independent of
        S4). Do NOT block S5 on S4."

# The file under edit #1 (tests)
- file: internal/provider/render_test.go
  why: "EDIT (2 new tests, append-only). Reuse mtPiManifest() (:487 — returns the pi-shape Manifest with
        SessionMode: strPtr('append') + the full --no-session-bearing BareFlags). Reuse containsPair (:606)
        and containsToken (:616). Append the 2 new tests after TestRenderMultiTurn_DoesNotMutateManifest
        (:589), before the `// helpers` divider (:601). Do NOT edit S3's 4 tests or the helpers."
  pattern: "Mirror the existing TestRenderMultiTurn_* style: `spec, err := mtPiManifest().RenderMultiTurn(...);
        if err != nil { t.Fatal(err) }; ...assertions`. For the table: a `cases := []struct{...}{...}` slice
        + `for _, tc := range cases { t.Run(tc.name, func(t *testing.T){...}) }`, mirroring
        TestRender_GoldenPerProvider (:30). For cross-turn: a loop over turns 1..3 asserting the id is present."
  gotcha: "(1) Use PRESENCE assertions (containsPair/containsToken) in the table — NOT reflect.DeepEqual
        (S3 owns byte-exact; the item says 'assert presence', 'Do NOT assert exact arg position'). (2) The
        cross-turn test must use ONE sessionID literal across all turns and assert it appears in EACH turn's
        args (case c). (3) Reuse mtPiManifest() — do NOT redefine a parallel pi literal. (4) Do NOT assert
        the exact POSITION of --session-id (the P1.M1.T4 stub integration tests assert order; S5 asserts
        presence)."

# The file under edit #2 (providers.md schema)
- file: docs/providers.md
  why: "EDIT (2 count fixes + 1 table row + 1 prose note). Line 3 'the 21-field schema' → 'the 22-field
        schema'. Line 13 'Each manifest has 21 fields' → '...22 fields...'. After the provider_flag table
        row (line 28), before bare_flags (line 29), insert a `| session_mode |` row. Add a short prose
        note (a new subsection or a paragraph under '## The schema') on the FR-T9 verification bar."
  pattern: "The schema table rows follow `| \`field\` | type | default | purpose |`. Match that exactly.
        The note follows the file's prose style (short paragraphs, PRD §-cross-links, the
        `# VERIFIED <date>; FR-T9.` token from fr-t9-verification.md). Mirror how experimental/reasoning
        nuance is footnoted elsewhere in the file."
  gotcha: "(1) The row goes BETWEEN provider_flag and bare_flags (PRD §12.1 ordering — matches manifest.go
        where SessionMode:66 sits between ProviderFlag:59 and BareFlags:69). (2) The count is 22, not 21:
        manifest.go has the SessionMode field (S1). (3) State the FR-T9 bar: a provider MUST NOT declare
        'append' speculatively; it requires a verified reproducible append-turn rendering (cross-link §9.24).
        (4) ONLY pi is verified today (fr-t9-verification.md covers pi alone)."

# The file under edit #3 (configuration.md override surface)
- file: docs/configuration.md
  why: "EDIT (1 note near line 22). Line 22 is the merge-semantics sentence ('When a [provider.<name>]
        section appears ... fields are merged onto the built-in manifest ... present values override,
        absent values inherit'). Add a short note immediately after it documenting session_mode is
        config-overridable per S2. configuration.md has NO per-field provider row table (that is in
        providers.md); the override SURFACE here is the merge semantics — so a note is the right shape,
        not a row."
  pattern: "Mirror how output/strip_code_fence override semantics are discussed (configuration.md:143 — a
        prose paragraph clarifying the opt-in override + a cross-link). Use a short paragraph + a cross-link
        to providers.md#the-schema. State the S2 semantics: explicit `session_mode = ''` disables multi-turn
        for pi; omitting inherits; setting 'append' on a non-append provider is a user override at their
        own FR-T9 risk."
  gotcha: "(1) The item says 'IF provider-level override is surfaced there' — it IS surfaced (as merge
        semantics at line 22), so add the note. Do NOT invent a per-field row table here (that belongs in
        providers.md). (2) The disable-multi-turn-for-pi use case is the load-bearing sentence (S2's payoff).
        (3) Cross-link providers.md, not architecture/fr-t9-verification.md (user-facing doc)."

# Read-only refs (do NOT edit in S5)
- file: internal/provider/render.go
  why: "READ-ONLY (S3 LANDED). RenderMultiTurn at :203. S5 adds NO render code. The tests call the landed
        method; they do not modify it."
- file: internal/provider/manifest.go
  why: "READ-ONLY (S1 LANDED). SessionMode *string at :66 (between ProviderFlag:59 and BareFlags:69); the
        providers.md schema row + count fix document THIS field. Resolve default '' (:177-178); Validate
        ''|'append' enum (:121-123). S5 adds NO schema code."
- file: internal/provider/merge.go
  why: "READ-ONLY (S2 LANDED). The regime-1 SessionMode clause is what makes the override work — S5 only
        DOCUMENTS its semantics in configuration.md."
- file: internal/provider/builtin.go + providers/pi.toml + builtin_test.go
  why: "READ-ONLY (S4, parallel). The shipped pi 'append' value + VERIFIED comment + piTOML fixture. S5's
        docs reference pi as the lone append provider but do NOT edit these."

# PRD authority (already in the selected content)
- prd: PRD.md §9.24 FR-T8 (session_mode field: "" default | "append"; pi ships "append"), FR-T9 (verification
        duty — NEVER speculative; record the verified flag set), FR-T6 (turn protocol: turn-1 sys prompt,
        --no-session dropped, --session-id added, session carries the prompt after turn 1); §12.1 (session_mode
        field position: between provider_flag and bare_flags); §16.1/FR-37a (field-merge across layers — the
        override semantics S2 implements and S5 documents).
  why: "FR-T8/T9 is the authority for the providers.md note (only pi verified; adding a provider requires
        the FR-T9 bar). §12.1 pins the schema-table row position. FR-37a is WHY an explicit '' overrides
        (S2/S5 configuration.md). FR-T6 is the turn contract the table + cross-turn tests pin."
  critical: "FR-T9 is WHY the providers.md note must say 'do not set append speculatively' — it is a spec
        requirement, not editorial caution."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── internal/provider/
│   ├── render.go            # READ-ONLY (S3 LANDED): RenderMultiTurn at :203
│   ├── render_test.go       # EDIT: + TestRenderMultiTurn_GoldenTable, + TestRenderMultiTurn_SessionIDStableAcrossTurns
│   │                        #      (reuse mtPiManifest :487, containsPair :606, containsToken :616)
│   ├── manifest.go          # READ-ONLY (S1 LANDED): SessionMode field :66 (the field the docs document)
│   ├── merge.go             # READ-ONLY (S2 LANDED): the override clause (semantics documented in configuration.md)
│   └── builtin.go           # READ-ONLY (S4 parallel): pi SessionMode="append" (referenced in providers.md note)
└── docs/
    ├── providers.md         # EDIT: line 3 + line 13 (21→22); + session_mode row after line 28; + FR-T9 prose note
    └── configuration.md     # EDIT: + session_mode override-semantics note near line 22 (cross-link providers.md)
```

### Desired Codebase Tree After S5

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/provider/render_test.go  # +2 tests (table-driven golden + cross-turn session-ID stability)
    docs/providers.md                 # schema 21→22, +session_mode row, +FR-T9/pi-only prose note
    docs/configuration.md             # +session_mode override-semantics note (S2)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/provider/render_test.go` | MODIFY (append only) | Add `TestRenderMultiTurn_GoldenTable` (cases slice, presence-based) + `TestRenderMultiTurn_SessionIDStableAcrossTurns` (case c). Reuse `mtPiManifest()`/helpers. **S3's 4 tests + helpers unchanged.** |
| `docs/providers.md` | MODIFY | Line 3 + line 13: `21` → `22`. Insert `session_mode` schema row after `provider_flag` (line 28). Add FR-T9 verification-bar prose note (only pi verified; cross-link §9.24). |
| `docs/configuration.md` | MODIFY | Add session_mode override-semantics note near line 22 (explicit `""` disables multi-turn for pi; omit inherits; per S2). Cross-link `providers.md#the-schema`. |

**Explicitly NOT touched**: `render.go` (S3 — landed), `manifest.go` (S1 — landed), `merge.go` (S2 — landed),
`builtin.go`/`providers/pi.toml`/`builtin_test.go` (S4 — parallel), `merge_test.go` (S2), `manifest_test.go`
(S1), S3's 4 `TestRenderMultiTurn_*` tests + the helpers in render_test.go, `multiturn.go`/`generate.go`
(P1.M1.T3 — the N+1 protocol), any integration/stub tests (P1.M1.T4), any other package, `PRD.md`,
`tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (S3 is LANDED — do NOT duplicate a/b/d/e): render.go:203 already has RenderMultiTurn and
// render_test.go already has TestRenderMultiTurn_PiTurn1_Golden (case a), _PiTurn2_NoSysPromptFlag_NoPrepend
// (case b), _NonAppendProviderErrors (case d), _DoesNotMutateManifest (case e). S5 adds the TABLE-DRIVEN
// consolidation (the item title's "golden/table" + the §5 idiom) + case c ONLY. Re-asserting a/b/d/e as a
// byte-exact table would be redundant and brittle. The table uses PRESENCE (containsPair/containsToken).

// CRITICAL (presence, NOT byte-exact, in the table): the item explicitly says "assert presence + the
// system-prompt turn-1-only distinction" and "Do NOT assert exact arg position for --session-id". So the
// table asserts containsPair(args, "--session-id", id) / containsToken(args, "--system-prompt") etc. — NOT
// reflect.DeepEqual(args, wantArgs). S3's individual tests own byte-exact; S5's table owns the contract
// surface. (P1.M1.T4's stub integration tests assert order; S5 does not.)

// CRITICAL (case c is the one real gap): S3's turn-1 and turn-2 tests each pin ONE turn's args with the
// SAME literal sessionID, but neither EXPLICITLY asserts the id renders identically across turns. The
// orchestrator mints ONE id and reuses it every turn (FR-T6); a regression that, say, appended a turn
// counter to the id would NOT be caught by S3's tests. TestRenderMultiTurn_SessionIDStableAcrossTurns
// closes that: one id, multiple turns, containsPair true for each.

// GOTCHA (reuse mtPiManifest — do NOT redefine): render_test.go:487 already returns the pi-shape Manifest
// (SessionMode strPtr("append"), the full --no-session-bearing BareFlags). Reuse it. A parallel literal
// would drift from S3's fixture and invite a future where the helper and the table disagree.

// GOTCHA (RenderMultiTurn performs NO spawning): like Render, its sole side effect is os.Environ() for Env.
// The tests need NO stubs/processes — Manifest literals + presence assertions suffice. Do NOT add mocking.

// GOTCHA (docs count is 22, not 21): manifest.go has the SessionMode field (S1, :66) — the struct now has
// 22 fields. providers.md line 3 ("21-field schema") and line 13 ("21 fields") are BOTH stale. Fix BOTH.

// GOTCHA (schema row position = PRD §12.1): insert the session_mode row BETWEEN provider_flag and
// bare_flags in the providers.md table — mirroring manifest.go where SessionMode:66 sits between
// ProviderFlag:59 and BareFlags:69. Do NOT place it near output/reasoning.

// GOTCHA (configuration.md has NO provider row table): the [provider.<name>] override SURFACE in
// configuration.md is the merge-SEMANTICS sentence (line 22), NOT a per-field row table (that lives in
// providers.md). The item's "if provider-level override is surfaced there" ⇒ yes, as semantics ⇒ add a
// NOTE, not a row. Do NOT invent a row table in configuration.md.

// GOTCHA (the FR-T9 wording is the audit trail): the providers.md note must cite "VERIFIED 2026-07-05;
// FR-T9" (from fr-t9-verification.md) and state the bar (no speculative "append"). This mirrors the
// VERIFIED-comment discipline S4 ships in builtin.go/pi.toml (FR-D5 precedent). Do NOT soften it to
// "pi supports sessions" — the verification duty is the point.

// GOTCHA (S2 semantics are the load-bearing configuration.md sentence): the note MUST state that an
// explicit `session_mode = ""` on pi DISABLES multi-turn (overrides the built-in "append"), and omitting
// inherits. This is S2's payoff use case. A note that only says "session_mode is overridable" understates it.
```

## Implementation Blueprint

### Data models and structure

None. No production Go code. The "models" are: a `cases` slice struct (the table test) and prose/markdown
(the docs). The tests exercise the LANDED `RenderMultiTurn(model, sysPrompt, userPayload, reasoning,
sessionID string, turn int) (*CmdSpec, error)` (render.go:203).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: render_test.go — TestRenderMultiTurn_GoldenTable (the §5 golden/table consolidation)
  - LOCATE: internal/provider/render_test.go. Append AFTER TestRenderMultiTurn_DoesNotMutateManifest
    (render_test.go:589), BEFORE the `// helpers` divider (render_test.go:601). Reuse mtPiManifest() (:487).
  - SHAPE: a `cases` slice mirroring TestRender_GoldenPerProvider (:30), run under t.Run. Each case carries
    {name, turn, wantSysPromptPresent bool} (the two key distinctions: turn-1 has the system-prompt flag,
    turn-2 does not; BOTH turns have --session-id and lack --no-session).
  - ASSERT (PRESENCE, per the item — NOT reflect.DeepEqual):
      * spec, err := mtPiManifest().RenderMultiTurn("zai/glm-5.2", "<sys>", "<payload>", "", tc.sessionID, tc.turn)
      * if err != nil { t.Fatalf(...) }
      * if !containsPair(spec.Args, "--session-id", tc.sessionID) { t.Errorf("--session-id <id> missing") }
      * if containsToken(spec.Args, "--no-session") { t.Errorf("--no-session should be filtered") }
      * if containsToken(spec.Args, "--system-prompt") != tc.wantSysPromptPresent { t.Errorf("sys-prompt presence") }
      * if !containsToken(spec.Args, "-p") { t.Errorf("-p (print_flag) missing") }
      * if spec.Stdin != "<payload>" { t.Errorf("Stdin should be payload only") }
  - CASES: {"pi turn 1 — session-id + sys-prompt + no --no-session", turn 1, wantSysPromptPresent: true},
           {"pi turn 2 — session-id present, sys-prompt ABSENT",      turn 2, wantSysPromptPresent: false},
           (optionally turn 3 with wantSysPromptPresent: false to reinforce the turn-1-only rule).
  - DO NOT: use reflect.DeepEqual (that is S3's job); assert exact position of --session-id; redefine a pi
    Manifest literal (use mtPiManifest()); edit S3's tests or the helpers.

Task 2: render_test.go — TestRenderMultiTurn_SessionIDStableAcrossTurns (case c — the real gap)
  - LOCATE: immediately after Task 1's test (still before the helpers divider).
  - SHAPE: mint ONE sessionID literal (e.g. "stagecoach-stability-probe"); loop over turns 1, 2, 3; for each,
    call RenderMultiTurn and assert the SAME id appears after --session-id.
  - ASSERT:
      * const sid = "stagecoach-stability-probe"
      * for turn := 1; turn <= 3; turn++ {
            spec, err := mtPiManifest().RenderMultiTurn("zai/glm-5.2", "<sys>", "<p>", "", sid, turn)
            if err != nil { t.Fatalf("turn %d: %v", turn, err) }
            if !containsPair(spec.Args, "--session-id", sid) {
                t.Errorf("turn %d: --session-id %q not present (got %v)", turn, sid, spec.Args)
            }
        }
  - WHY case c: S3's turn-1 and turn-2 tests use the same literal but never EXPLICITLY assert cross-turn
    equality. This test pins "the orchestrator's single minted id renders identically on every turn"
    (FR-T6). A regression that mutated the id per turn (e.g. appending a counter) would fail here.
  - DO NOT: use different ids per turn (that defeats the purpose); assert byte-exact args; skip turn 3
    (the N+1 protocol has a final turn — covering ≥3 turns is more representative than 2).

Task 3: docs/providers.md — fix the schema count + add the session_mode row + FR-T9 note
  - EDIT line 3: "the 21-field schema" → "the 22-field schema".
  - EDIT line 13: "Each manifest has 21 fields (matching the TOML tags in `internal/provider/manifest.go`):"
    → "Each manifest has 22 fields (matching the TOML tags in `internal/provider/manifest.go`):".
  - INSERT a table row after the `provider_flag` row (line 28), before the `bare_flags` row (line 29):
        | `session_mode` | string | `""` | Multi-turn fallback capability (§9.24). `""` (default) = the provider cannot append turns across one-shot calls → multi-turn unavailable; `"append"` = re-invoking the same session id appends a recallable turn. **Only pi ships `"append"`** (VERIFIED 2026-07-05; FR-T9). Setting `"append"` requires a verified, reproducible append-turn rendering — see the note below. |
  - ADD a prose note (a short subsection "### Multi-turn capability (`session_mode`)" under "## The schema",
    OR a paragraph immediately after the table) stating:
      * A provider supports the lossless multi-turn fallback (§9.24) iff re-invoking the SAME session id
        appends a turn the model can recall.
      * ONLY `pi` declares `session_mode = "append"` today — VERIFIED 2026-07-05 via a live run
        (`pi --session-id X <isolation-flags-minus-no-session> -p "remember BANANA"` then a recall turn
        returning "BANANA"; see the FR-T9 verification record).
      * Every other built-in ships `""` → multi-turn is skipped silently for them (one-shot → rescue, unchanged).
      * **FR-T9 verification bar**: a manifest MUST NOT declare `"append"` speculatively. Adding another
        provider requires a verified, reproducible append-turn rendering (the exact flag set confirmed,
        analogous to FR-D5). Cross-link §9.24.
  - DO NOT: add a session_mode COLUMN to the "8 built-in providers" table (it is already wide; a note is
    clearer). DO NOT edit other rows. DO NOT change the markdown table alignment elsewhere.

Task 4: docs/configuration.md — add the session_mode override-semantics note
  - LOCATE: line 22 — the sentence "When a `[provider.<name>]` section appears in a config file, its fields
    are **merged onto** the built-in manifest of the same name (field-by-field: present values override,
    absent values inherit)."
  - INSERT a short note immediately after it (a new paragraph or a blockquote), e.g.:
      > `session_mode` is one such overridable field. An explicit `session_mode = ""` on a provider that
      > ships `"append"` (pi) **disables the multi-turn fallback** for that provider (one-shot → rescue,
      > unchanged); omitting the key inherits the built-in. Setting `session_mode = "append"` on a provider
      > that ships `""` is a user override at their own FR-T9 verification risk (the shipped default stays
      > `""` until a reproducible append-turn rendering is confirmed — see [providers.md](providers.md#the-schema)
      > and §9.24).
  - DO NOT: add a per-field row table here (providers.md owns the schema table). DO NOT edit the precedence
    list or other sections. Cross-link providers.md (NOT architecture/fr-t9-verification.md — user-facing).

Task 5: VALIDATE
  - RUN: gofmt -w internal/provider/render_test.go  (the docs are markdown — not gofmt'd)
  - RUN: go build ./... ; go vet ./... ; go test -race ./...
  - RUN the new + existing RenderMultiTurn tests specifically:
        go test -race -run 'TestRenderMultiTurn' ./internal/provider/ -v
        # Expected: 6 PASS (S3's 4 + S5's 2). The 4 S3 tests MUST stay green (no regression).
  - RUN the broader render regression: go test -race -run 'TestRender' ./internal/provider/ -v
  - GREP the docs:
        grep -n "22-field\|22 fields" docs/providers.md                 # → 2 matches (line 3 + line 13)
        grep -n "session_mode" docs/providers.md                        # → ≥2 (the row + the note)
        grep -n "VERIFIED 2026-07-05" docs/providers.md                 # → ≥1 (the FR-T9 wording)
        grep -n "session_mode" docs/configuration.md                    # → ≥1 (the override note)
        grep -n "§9.24\|9.24" docs/providers.md docs/configuration.md   # → cross-links present
  - CONFIRM scope: git diff --stat -- internal/ pkg/ cmd/ docs/ providers/
        # → ONLY internal/provider/render_test.go + docs/providers.md + docs/configuration.md.
  - FIX-FORWARD:
      * if TestRenderMultiTurn_GoldenTable fails on turn-2 sys-prompt presence, RenderMultiTurn is leaking
        --system-prompt on turn>1 (an S3 bug, not S5 — but the table catches it; report it, the table is
        correct). If it fails on --no-session present, the filter regressed (S3).
      * if providers.md count grep shows only 1 match, one of line 3 / line 13 was missed.
      * if configuration.md grep is empty, the note was not added near line 22.
```

### Implementation Patterns & Key Details

```go
// === render_test.go — TestRenderMultiTurn_GoldenTable (presence-based cases slice, the §5 idiom) ===
func TestRenderMultiTurn_GoldenTable(t *testing.T) {
	// The §5 golden/table consolidation. PRESENCE-based (containsPair/containsToken), NOT byte-exact:
	// S3's individual tests (TestRenderMultiTurn_PiTurn1_Golden etc.) own byte-exact; this table owns the
	// contract surface in the codebase's documented table idiom. Per the work item: "assert presence +
	// the system-prompt turn-1-only distinction"; "Do NOT assert exact arg position for --session-id".
	cases := []struct {
		name                  string
		turn                  int
		sessionID             string
		wantSysPromptPresent  bool
	}{
		{"pi_turn1_session_id_and_sys_prompt_no_no_session", 1, "stagecoach-gt-t1", true},
		{"pi_turn2_session_id_present_sys_prompt_absent",    2, "stagecoach-gt-t1", false},
		{"pi_turn3_session_id_still_present_sys_prompt_absent", 3, "stagecoach-gt-t1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := mtPiManifest().RenderMultiTurn("zai/glm-5.2", "<sys>", "<payload>", "", tc.sessionID, tc.turn)
			if err != nil {
				t.Fatalf("turn %d: %v", tc.turn, err)
			}
			if !containsPair(spec.Args, "--session-id", tc.sessionID) {
				t.Errorf("turn %d: --session-id %q not present as a pair: %v", tc.turn, tc.sessionID, spec.Args)
			}
			if containsToken(spec.Args, "--no-session") {
				t.Errorf("turn %d: --no-session should be filtered out: %v", tc.turn, spec.Args)
			}
			if containsToken(spec.Args, "--system-prompt") != tc.wantSysPromptPresent {
				t.Errorf("turn %d: --system-prompt presence = %v, want %v",
					tc.turn, containsToken(spec.Args, "--system-prompt"), tc.wantSysPromptPresent)
			}
			if !containsToken(spec.Args, "-p") {
				t.Errorf("turn %d: -p (print_flag) missing: %v", tc.turn, spec.Args)
			}
			if spec.Stdin != "<payload>" {
				t.Errorf("turn %d: Stdin = %q, want <payload> (no sys prepend via stdin)", tc.turn, spec.Stdin)
			}
		})
	}
}

// === render_test.go — TestRenderMultiTurn_SessionIDStableAcrossTurns (case c — the gap S3 left) ===
func TestRenderMultiTurn_SessionIDStableAcrossTurns(t *testing.T) {
	// FR-T6: the orchestrator mints ONE session id and re-invokes it every turn. S3's per-turn tests use a
	// shared literal but never EXPLICITLY assert the id renders identically across turns. This test pins
	// that invariant: a single id must appear after --session-id on turn 1, 2, AND 3.
	const sid = "stagecoach-stability-probe"
	for turn := 1; turn <= 3; turn++ {
		spec, err := mtPiManifest().RenderMultiTurn("zai/glm-5.2", "<sys>", "<payload>", "", sid, turn)
		if err != nil {
			t.Fatalf("turn %d: %v", turn, err)
		}
		if !containsPair(spec.Args, "--session-id", sid) {
			t.Errorf("turn %d: expected --session-id %q in args, got %v", turn, sid, spec.Args)
		}
	}
}
```

```markdown
<!-- === docs/providers.md — the schema count fix + the new row (between provider_flag and bare_flags) === -->
Line 3:  "...the 21-field schema..."   →   "...the 22-field schema..."
Line 13: "Each manifest has 21 fields (matching the TOML tags in `internal/provider/manifest.go`):"
       → "Each manifest has 22 fields (matching the TOML tags in `internal/provider/manifest.go`):"

After the `provider_flag` row, before the `bare_flags` row:
| `provider_flag` | string | `""` | Flag for sub-provider selection (e.g. `"--provider"`). |
| `session_mode` | string | `""` | Multi-turn fallback capability (§9.24). `""` (default) = the provider cannot append turns across one-shot calls → multi-turn unavailable; `"append"` = re-invoking the same session id appends a recallable turn. **Only pi ships `"append"`** (VERIFIED 2026-07-05; FR-T9). Requires a verified, reproducible append-turn rendering — see below. |
| `bare_flags` | list of string | `[]` (none) | Extra flags appended verbatim before `print_flag` in bare mode. |
```

```markdown
<!-- === docs/providers.md — the prose note (new subsection under "## The schema") === -->
### Multi-turn capability (`session_mode`)

A provider supports Stagecoach's **lossless multi-turn fallback** (§9.24 — used when a one-shot generation
repeatedly fails on a diff too large for a single reliable request) if and only if re-invoking the SAME
session id appends a turn the model can recall. The `session_mode` manifest field declares this:

- `"append"` — re-invoking the same session id appends a recallable turn (multi-turn available).
- `""` (default) — the provider cannot append turns across one-shot calls (multi-turn unavailable; the run
  proceeds one-shot → rescue, unchanged).

**Only `pi` ships `session_mode = "append"` today** — VERIFIED 2026-07-05 via a live run
(`pi --session-id X <isolation-flags-minus-no-session> -p "remember BANANA"`, then a same-`--session-id`
recall turn returning "BANANA"). Every other built-in (claude, opencode, codex, cursor, agy, gemini,
qwen-code) ships `""`.

**FR-T9 verification bar.** A manifest MUST NOT declare `"append"` speculatively. Setting it requires a
verified, reproducible append-turn rendering — the exact flag set confirmed per provider (analogous to
FR-D5's model-token verification duty). Until a provider's append mechanism is verified, its `session_mode`
stays `""` and multi-turn is silently skipped for it. See §9.24 (FR-T8/FR-T9) for the full contract.
```

```markdown
<!-- === docs/configuration.md — the override-semantics note (after line 22) === -->
When a `[provider.<name>]` section appears in a config file, its fields are **merged onto** the built-in
manifest of the same name (field-by-field: present values override, absent values inherit).

> **`session_mode` override.** `session_mode` is one such overridable field. An explicit `session_mode = ""`
> on a provider that ships `"append"` (pi) **disables the multi-turn fallback** for that provider (the run
> proceeds one-shot → rescue, unchanged); omitting the key inherits the built-in `"append"`. Setting
> `session_mode = "append"` on a provider that ships `""` is a user override at their own FR-T9
> verification risk — the shipped default stays `""` until a reproducible append-turn rendering is
> confirmed (see [providers.md](providers.md#the-schema) and §9.24).
```

### Integration Points

```yaml
TESTS (internal/provider/render_test.go):
  - + TestRenderMultiTurn_GoldenTable        # the §5 golden/table consolidation (presence-based)
  - + TestRenderMultiTurn_SessionIDStableAcrossTurns  # case c (cross-turn id stability)
  - REUSES: mtPiManifest() (render_test.go:487), containsPair (:606), containsToken (:616)

DOCS (docs/):
  - providers.md: schema count 21→22 (line 3 + line 13); + session_mode row (after provider_flag line 28);
                  + FR-T9 verification-bar prose note (only pi verified; cross-link §9.24)
  - configuration.md: + session_mode override-semantics note (after line 22; explicit "" disables for pi;
                       omit inherits; cross-link providers.md#the-schema)

CONSUMED BY (read-only — what these tests/docs describe):
  - render.go RenderMultiTurn (S3, LANDED, :203) — the method the 2 new tests exercise
  - manifest.go SessionMode field (S1, LANDED, :66) — the field the docs document
  - merge.go MergeManifest SessionMode clause (S2, LANDED) — the override the configuration.md note describes
  - builtin.go builtinPi + providers/pi.toml (S4, parallel) — the shipped "append" value the providers.md
    note references ("only pi ships append today")

NO-TOUCH (explicitly — owned by sibling/prior/later subtasks):
  - internal/provider/render.go            # S3 (LANDED): the renderer; S5 adds NO render code
  - internal/provider/manifest.go          # S1 (LANDED): the field; S5 adds NO schema code
  - internal/provider/merge.go             # S2 (LANDED): the clause; S5 only DOCUMENTS it
  - internal/provider/builtin.go + providers/pi.toml + builtin_test.go   # S4 (parallel): the value
  - internal/provider/{manifest,merge,builtin}_test.go                    # S1/S2/S4 tests
  - S3's 4 TestRenderMultiTurn_* tests + the containsPair/containsToken/mtPiManifest helpers  # unchanged
  - multiturn.go / generate.go             # P1.M1.T3: the N+1 turn protocol (not in scope)
  - internal/stubtest + internal/e2e + internal/generate integration tests  # P1.M1.T4: stub/e2e tests
  - any other package; PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S5):
  - P1.M1.T4 (integration tests): exercises RenderMultiTurn end-to-end via the stub agent, asserting
    --session-id present, --no-session dropped, final parsed+deduped, commit lands. S5's unit tests are the
    pure-data-transformation layer beneath it.
  - P1.M1.T3.S4 (Mode A how-it-works.md): documents the multi-turn PROTOCOL; S5 documents the session_mode
    FIELD + override. The two doc edits are complementary (field/capability vs protocol).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/provider/render_test.go     # realign the new tests (markdown docs are not gofmt'd)
gofmt -l .                                    # Expected: empty after the -w
go vet ./internal/provider/...                # Expected: exit 0
go build ./...                                # Expected: exit 0
```

### Level 2: Unit Tests — the 2 new tests + the S3 regression

```bash
cd /home/dustin/projects/stagecoach

# The 6 RenderMultiTurn tests: S3's 4 (a/b/d/e) MUST stay green + S5's 2 new ones pass.
go test -race -run 'TestRenderMultiTurn' ./internal/provider/ -v
# Expected: 6 PASS. If S3's 4 fail, S5 regressed them (check for an accidental edit to the helpers/tests).
#           If S5's GoldenTable fails on turn-2 sys-prompt, RenderMultiTurn leaked --system-prompt on turn>1
#           (an S3 renderer bug the table correctly catches — the table assertion is right; report it).

# The broader render suite (proves the table/helper reuse didn't break Render's tests).
go test -race -run 'TestRender' ./internal/provider/ -v

# Full provider suite.
go test -race ./internal/provider/ -v

# Expected: ALL PASS. The table uses presence (containsPair/containsToken); the cross-turn test loops
# turns 1..3 with one id. No mocking — RenderMultiTurn is pure data transformation.
```

### Level 3: Whole-Repository Regression + the docs grep verification

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages green
go vet ./...                     # Expected: exit 0

# === The docs verification: session_mode is documented in BOTH reference docs ===
grep -n "22-field\|22 fields" docs/providers.md          # Expected: 2 matches (line 3 + line 13)
grep -n "session_mode" docs/providers.md                 # Expected: ≥2 (the schema row + the prose note)
grep -n "VERIFIED 2026-07-05" docs/providers.md          # Expected: ≥1 (the FR-T9 audit-trail wording)
grep -n "§9.24\|9.24" docs/providers.md                  # Expected: ≥1 (the multi-turn cross-link)
grep -n "session_mode" docs/configuration.md             # Expected: ≥1 (the override-semantics note)
grep -n "providers.md" docs/configuration.md             # Expected: the cross-link is present

# === The count fix: providers.md no longer claims 21 ===
grep -c "21-field\|21 fields" docs/providers.md          # Expected: 0 (both occurrences fixed to 22)

# === Confirm ONLY the 3 intended files changed ===
git diff --stat -- internal/ pkg/ cmd/ docs/ providers/
#   Expected: internal/provider/render_test.go + docs/providers.md + docs/configuration.md ONLY.
```

### Level 4: Docs Render Smoke (verify the markdown is well-formed + the schema row parses)

```bash
cd /home/dustin/projects/stagecoach

# Confirm the schema table still has a consistent row count (the new row inserted cleanly between
# provider_flag and bare_flags — not appended at the end, not splitting another row):
awk '/^## The schema/,/^## /' docs/providers.md | grep -cE '^\| `'   # Expected: 22 rows (one per field)

# Confirm the schema row ordering: provider_flag → session_mode → bare_flags (PRD §12.1):
awk '/^## The schema/,/^## /' docs/providers.md | grep -nE '^\| `(provider_flag|session_mode|bare_flags)`'
#   Expected: provider_flag on line N, session_mode on line N+1, bare_flags on line N+2 (contiguous, ordered).

# Confirm the configuration.md note landed right after the merge-semantics sentence (line 22) and cites S2:
sed -n '20,30p' docs/configuration.md | grep -i "session_mode"
#   Expected: the override note is visible in the line-22 neighborhood.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green (incl. the 2 new tests + S3's 4 RenderMultiTurn tests).
- [ ] `go test -race -run 'TestRenderMultiTurn' ./internal/provider/ -v` → 6 PASS.

### Feature Validation

- [ ] `TestRenderMultiTurn_GoldenTable` is a `cases` slice under `t.Run`, asserting PRESENCE
      (containsPair/containsToken) for turn-1 (sys-prompt present) and turn-2/3 (sys-prompt absent), with
      `--session-id` present and `--no-session` absent on every turn.
- [ ] `TestRenderMultiTurn_SessionIDStableAcrossTurns` uses ONE sessionID across turns 1..3 and asserts
      `containsPair(args, "--session-id", id)` is true for each (case c).
- [ ] providers.md schema table has a `session_mode` row between `provider_flag` and `bare_flags`.
- [ ] providers.md says "22-field schema" (line 3) and "Each manifest has 22 fields" (line 13).
- [ ] providers.md prose note states: only pi = `"append"` (VERIFIED 2026-07-05; FR-T9), others `""`, and
      adding a provider requires the FR-T9 verification bar, with a §9.24 cross-link.
- [ ] configuration.md has an override-semantics note near line 22 (explicit `""` disables for pi; omit
      inherits; per S2) with a providers.md cross-link.

### Scope Discipline Validation

- [ ] ONLY `internal/provider/render_test.go` + `docs/providers.md` + `docs/configuration.md` change
      (`git diff --stat` confirms).
- [ ] Did NOT edit `render.go` (S3), `manifest.go` (S1), `merge.go` (S2), `builtin.go`/`pi.toml`/`builtin_test.go` (S4).
- [ ] Did NOT re-add or edit S3's 4 `TestRenderMultiTurn_*` tests or the `mtPiManifest`/`containsPair`/`containsToken` helpers.
- [ ] Did NOT use `reflect.DeepEqual` in the new table test (presence-only, per the item).
- [ ] Did NOT add mocking/stubbing (RenderMultiTurn is pure data transformation).
- [ ] Did NOT add a `session_mode` column to the providers.md "8 built-in providers" table (a note is clearer).
- [ ] Did NOT invent a per-field provider row table in configuration.md (providers.md owns the schema table).
- [ ] Did NOT implement the turn protocol / `multiturn.go` / integration tests (P1.M1.T3 / P1.M1.T4).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] The 2 new tests reuse `mtPiManifest()` (no parallel pi literal that could drift).
- [ ] The table test follows the `TestRender_GoldenPerProvider` cases-slice idiom (§5).
- [ ] The cross-turn test covers ≥3 turns (representative of the N+1 protocol's final turn).
- [ ] The docs edits match each file's existing conventions (table-row format; prose + cross-link style).
- [ ] The FR-T9 wording in providers.md cites "VERIFIED 2026-07-05" (the audit trail, matching S4's discipline).
- [ ] The configuration.md note states the S2 disable-multi-turn-for-pi payoff explicitly (not just "overridable").

---

## Anti-Patterns to Avoid

- ❌ Don't re-add item cases (a)/(b)/(d)/(e) as new tests. S3 LANDED them
  (`TestRenderMultiTurn_PiTurn1_Golden` / `_PiTurn2_NoSysPromptFlag_NoPrepend` / `_NonAppendProviderErrors`
  / `_DoesNotMutateManifest`, render_test.go:503/537/562/589). S5 adds the **table-driven consolidation**
  (the item title's "golden/table" + the §5 idiom) + case **c** ONLY. Duplicating a/b/d/e is redundant and
  fails review.
- ❌ Don't make the table test byte-exact (`reflect.DeepEqual`). S3's individual tests own byte-exact; the
  item explicitly says "assert presence + the system-prompt turn-1-only distinction" and "Do NOT assert
  exact arg position for --session-id". Use `containsPair`/`containsToken`. A byte-exact table would brittlely
  duplicate S3's `wantArgs`.
- ❌ Don't skip case c (`TestRenderMultiTurn_SessionIDStableAcrossTurns). It is the ONE genuine coverage
  gap: S3's per-turn tests never EXPLICITLY assert the session-ID renders identically across turns (FR-T6's
  "one minted id, reused every turn"). Without it, a regression that mutated the id per turn is invisible.
- ❌ Don't redefine a pi Manifest literal in the new tests. Reuse `mtPiManifest()` (render_test.go:487) — a
  parallel literal would drift from S3's fixture.
- ❌ Don't edit `render.go` (S3), `manifest.go` (S1), `merge.go` (S2), or `builtin.go`/`pi.toml` (S4). S5 is
  tests + docs ONLY. S3/S1/S2 are LANDED; S4 is parallel. S5 produces no production Go code.
- ❌ Don't edit S3's 4 `TestRenderMultiTurn_*` tests or the `containsPair`/`containsToken`/`mtPiManifest`
  helpers. Append the 2 new tests after `TestRenderMultiTurn_DoesNotMutateManifest` (:589), before the
  helpers divider (:601).
- ❌ Don't leave providers.md saying "21-field"/"21 fields". S1 added the `SessionMode` field
  (manifest.go:66) — the struct now has **22** fields. Fix BOTH the line-3 and line-13 counts.
- ❌ Don't place the `session_mode` schema row outside the PRD §12.1 slot (between `provider_flag` and
  `bare_flags`). manifest.go has SessionMode:66 between ProviderFlag:59 and BareFlags:69; the doc table must
  mirror that ordering.
- ❌ Don't add a `session_mode` COLUMN to the providers.md "8 built-in providers" table. That table is
  already wide; a prose note ("only pi ships append today") is clearer and matches the file's footnote style.
- ❌ Don't invent a per-field provider row table in configuration.md. The `[provider.<name>]` override
  SURFACE in configuration.md is the merge-SEMANTICS sentence (line 22); the schema row table lives in
  providers.md. The item's "if provider-level override is surfaced there" ⇒ yes, as semantics ⇒ a NOTE.
- ❌ Don't soften the providers.md FR-T9 note to "pi supports sessions." The verification DUTY is the point:
  cite "VERIFIED 2026-07-05; FR-T9" and state the bar (no speculative "append"; adding a provider requires a
  verified, reproducible append-turn rendering). This mirrors S4's VERIFIED-comment discipline (FR-D5).
- ❌ Don't understate the configuration.md note. It MUST state that an explicit `session_mode = ""` on pi
  DISABLES multi-turn (S2's payoff use case), and that omitting inherits the built-in. "session_mode is
  overridable" alone is insufficient.
- ❌ Don't add mocking/stubbing/process-spawning to the unit tests. `RenderMultiTurn` performs NO spawning
  (the sole side effect is `os.Environ()` for Env, matching `Render`). Manifest literals + presence
  assertions suffice. Stub/integration tests are P1.M1.T4's job.
- ❌ Don't implement the turn protocol, chunking, the trigger gate, `multiturn.go`, or any integration test —
  those are P1.M1.T3 / P1.M1.T4. S5 is the unit-test + docs layer beneath them.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: The two tests are pure data-transformation assertions (no spawning, no mocking) reusing three
already-landed helpers (`mtPiManifest` :487, `containsPair` :606, `containsToken` :616) whose exact shapes
are quoted verbatim, with the table following the §5 `TestRender_GoldenPerProvider` idiom (render_test.go:30,
quoted in research-tests-ui.md). The docs edits are pinned to exact line numbers (providers.md:3/13/28;
configuration.md:22) with the exact replacement/insertion text provided ready-to-paste, and the FR-T9 wording
("VERIFIED 2026-07-05") + the S2 override semantics are quoted from their authority files. Five independent
de-riskings: (1) S3 is LANDED — `RenderMultiTurn` (render.go:203) + its 4 tests exist and compile, so the 2
new tests exercise a real method; (2) the "don't duplicate S3 / use presence not byte-exact" rule is stated
in five places (description, Gotchas, Task 1, Validation, Anti-Patterns) so the implementer won't write a
redundant brittle table; (3) case c is the one real gap and its test shape is fully specified (one id, turns
1..3, `containsPair` per turn); (4) the docs count fix is mechanical (21→22, two spots) and the schema-row
ordering is pinned to manifest.go's struct layout; (5) the FR-T9 + S2 wording is quoted verbatim from
fr-t9-verification.md and P1M1T1S2/PRP.md — no invention. The only residual uncertainty (not 10/10) is the
exact prose phrasing of the two doc notes (taste — the load-bearing facts/wording are pinned, the surrounding
sentences are editorial), and whether the implementer remembers to fix BOTH line 3 and line 13 of providers.md
(the grep gate catches a single-spot miss). No production Go code is touched, so the blast radius is two tests
+ three markdown edits, and S3's renderer/tests are provably untouched (the Level-2 gate asserts all 6
`TestRenderMultiTurn_*` tests pass).
