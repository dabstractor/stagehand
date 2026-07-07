---
name: "P1.M1.T3.S4 — Unit tests (chunk math, trigger truth table, token_limit non-interaction) + Mode A how-it-works.md"
description: |
  Add the EXHAUSTIVE multi-turn unit tests to `internal/generate/multiturn_test.go` (S1/S2's file) and a
  concise "Multi-turn generation fallback" subsection to `docs/how-it-works.md`. This is the unit-test +
  docs companion to S3 (which wired the gate into CommitStaged + added focused end-to-end tests in
  generate_test.go). S4 OWNS the exhaustive coverage the item contract names:
    (a) chunk-count ceil math — N grows with payload size (VERIFIED exact-N payloads: 'abcd\n'×4/CT5→1,
        ×8→2, ×12→3; 'ab\n'×33/CT10→3 = the 2.5×→3 case). ⚠️ The forward-newline anchor means N ≠ naive
        ceil(ET/CT) — the EXACT payloads below are empirically pinned; inventing "chunkTokens+1→N=2" with
        an arbitrary payload FAILS (anchoring absorbs the overage). This is the #1 one-pass risk.
    (b) newline-anchored boundaries — a payload where the naive rune boundary lands mid-line ⇒ every chunk
        boundary falls on '\n' (no fractured diff line) + lossless round-trip (VERIFIED).
    (c) PART i/N prefix — each chunk.text matches ^PART i/N:\n with correct, monotonic 1..N i/total.
    (d) trigger-gate truth table — a table test driving CommitStaged (the gate's sole call site; the gate
        is INLINE in generate.go:297 — there is NO standalone predicate fn, so this is an integration-style
        unit test, explicitly allowed by the item "assert it was NOT called"). Each of the 4 FR-T1
        conditions flips to false ⇒ multi-turn skipped ⇒ the gate's VerboseWarn("one-shot exhausted →
        multi-turn fallback") trigger is ABSENT from the verbose buffer (the clean, uniform proof Run was
        NOT entered) + the result is *RescueError{Kind:ErrRescue}. Row 4 = success-already (gate not
        reached). Row 5 (control) = all-true ⇒ trigger PRESENT.
    (e) token_limit non-interaction (FR-T12) — pure-helper + gate level: chunkPayload's signature has NO
        TokenLimit parameter (N is a function of (payload, chunkTokens) only), and the gate expression has
        no TokenLimit term. ⚠️ FR-T12 TENSION (flagged, not hidden): S3's D2 passes the CAPTURED payload
        (StagedDiff already applied TokenLimit), so in the diff-exceeds-TokenLimit case multi-turn receives
        a TRUNCATED payload — diverging from FR-T12's literal "untruncated." S4 CANNOT fix this (generate.go
        is S3's territory; S3 forbids the recompute). S4 tests the TRUE non-interaction (chunking never
        consults TokenLimit), records the gap in how-it-works.md, and defers the full-diff re-capture fix.
  DOCS: Mode A — add "## Multi-turn generation fallback" to docs/how-it-works.md immediately before
  "## Hook mode vs the snapshot-based flow" (line 262), covering the 4 triggers, lossless (not lossy
  map-reduce) chunking, the N+1 protocol, final-turn parse+dedupe reuse, failure→rescue (snapshot safe),
  and token_limit non-applicability (FR-T12, with the S3 caveat noted honestly). Cross-links PRD §9.24.
  Touches ONLY internal/generate/multiturn_test.go + docs/how-it-works.md. NON-OVERLAPPING with S3 (which
  edits generate.go + generate_test.go). NO production-code change; ZERO new deps.
---

## Goal

**Feature Goal**: Provide the exhaustive unit-test coverage and the user-facing Mode A documentation for
the multi-turn generation fallback (PRD §9.24, FR-T1/T3/T12). S3 wired the gate and added focused
end-to-end tests; S4 locks in (a) the `chunkPayload` sizing math (exact-N, ceil growth, newline-anchored
boundaries, PART i/N prefix), (b) the 4-condition FR-T1 trigger truth table (each condition independently
gating multi-turn OFF), and (c) FR-T12's token_limit non-interaction — plus the `how-it-works.md`
subsection that explains the feature to users.

**Deliverable** (two files MODIFIED, zero new files):
1. **MODIFY** `internal/generate/multiturn_test.go`: ADD the unit-test matrix — (a) a `TestChunkPayload_CeilMath` table with the VERIFIED exact-N payloads; (b) a `TestChunkPayload_NoFracturedBoundaries` (every boundary on `\n` + round-trip); (c) a `TestChunkPayload_PartPrefixMonotonic` (exact `^PART i/N:\n` + 1..N); (d) a `TestMultiTurnTriggerGate_TruthTable` driving `CommitStaged` across the 4 skip rows + the success row + the all-true control, asserting the `VerboseWarn` trigger's presence/absence; (e) a `TestChunkPayload_TokenLimitNonInteraction` (pure-helper) + a `TestMultiTurnGate_TokenLimitNotATerm` (gate-level) documenting FR-T12 and the S3 caveat.
2. **MODIFY** `docs/how-it-works.md`: INSERT a `## Multi-turn generation fallback` section before `## Hook mode vs the snapshot-based flow`.

No production code touched. No new dependencies. `go.mod` unchanged.

**Success Definition**: `go build/vet/gofmt` clean; `go test -race ./internal/generate/...` green with the
new tests + all S1/S2/S3 tests passing; the ceil-math table asserts the VERIFIED exact N (1/2/3/3) for
the pinned payloads; the truth table proves each FR-T1 condition independently suppresses the
`VerboseWarn` trigger (Run not entered) and yields rescue (or commit for the success row); the
token_limit test proves chunkPayload's N is TokenLimit-independent and documents the S3↔FR-T12 caveat;
`how-it-works.md` has the new subsection at the specified anchor, cross-linking PRD §9.24; `git diff
--stat` shows ONLY the two files.

## User Persona

**Target User**: (1) The maintainer regression-protecting the multi-turn sizing/gate logic (the tests);
(2) the end user / contributor reading `how-it-works.md` to understand when and why multi-turn fires.

**Use Case**: Tests (a)–(c) guard `chunkPayload`'s lossless/no-fracture/labeling invariants against
refactor regressions. Test (d) guards the FR-T1 gate — a future change that accidentally drops a condition
(or inverts one) is caught by the truth table. Test (e) documents that token_limit is architecturally
separate from multi-turn chunking. The docs subsection tells a user with a 266K-token diff why stagecoach
silently re-delivers it across N+1 turns instead of rescuing.

**User Journey**: A maintainer edits `multiturn.go` or the gate in `generate.go` → `go test -race
./internal/generate/...` runs the S4 matrix → any regression in sizing/boundaries/prefix/gating/token-
independence fails loudly with a precise assertion. A user reads `how-it-works.md` → finds the
"Multi-turn generation fallback" section → understands the 4 triggers, that chunking is lossless, and
that token_limit does not apply.

**Pain Points Addressed**: (1) Without the ceil-math table, a refactor to `chunkPayload`'s windowing
could silently change N (and thus turn count / total budget) with no test catching it. (2) Without the
truth table, a gate-condition regression (e.g. dropping the `session_mode` check) ships silently — S3's
focused tests cover only 2 of the 4 conditions. (3) Without the FR-T12 test + docs caveat, a reader
assumes multi-turn always uses the full diff, when S3's captured-payload decision means a TokenLimit-
truncated diff reaches multi-turn. (4) Without the docs subsection, the feature is invisible to users.

## Why

- **PRD §9.24 FR-T3 (Chunk sizing):** N = ceil(payload_tokens / chunk_size), boundaries anchored forward
  to the next newline, "PART i/N:" outside the budget. Tests (a)/(b)/(c) lock these in.
- **PRD §9.24 FR-T1 (gated trigger):** multi-turn activates ONLY when ALL FOUR conditions hold; any one
  false ⇒ rescue. Test (d) is the exhaustive truth table S3's focused tests deliberately defer to S4.
- **PRD §9.24 FR-T12 (no token_limit interaction):** multi-turn deliberately ignores token_limit. Test
  (e) proves the architectural separation and documents the S3 caveat.
- **PRD §20.1 (Unit — pure functions) / §20.2 (property/invariant tests):** chunk math + boundary
  anchoring are exactly the "pure function" + "lossless round-trip" invariants the QA strategy names.
- **PRD §21.5 (README/docs surface) + the Mode A doc duty:** the multi-turn feature must be explained in
  `how-it-works.md` (Mode A = rides with the implementing work).

## What

### Test (a) — `TestChunkPayload_CeilMath` (table, VERIFIED exact N)

A `cases` slice driving `chunkPayload(payload, ct)`; each row asserts `len(chunks) == wantN`. Uses the
**pinned payloads** from research §2 — these are the ONLY payloads whose N is clean (the forward-newline
anchor distorts arbitrary payloads):

| name | payload | ct | wantN | note |
|---|---|---|---|---|
| `one_chunk_exact_CT` | `"abcd\n"×4` | 5 | 1 | ET = CT |
| `two_chunks_2x_CT` | `"abcd\n"×8` | 5 | 2 | ET = 2×CT |
| `three_chunks_3x_CT` | `"abcd\n"×12` | 5 | 3 | ET = 3×CT |
| `two_half_x ceil` | `"ab\n"×33` | 10 | 3 | ET = 2.5×CT → N=3 (ceil) |

PLUS a monotonicity sub-assertion: for the `'abcd\n'`/CT=5 family, `wantN` is non-decreasing across
×4/×8/×12 (more content never reduces N).

### Test (b) — `TestChunkPayload_NoFracturedBoundaries`

Payload `"aaaaaa\nbbbbbb\ncccccc\n"` (6-rune lines), CT=1 (window=4 ⇒ the naive boundary lands mid-line
inside "aaaa"). Assert: (1) every non-last chunk body **ends on `\n`** (no fractured diff line); (2) the
concatenated bodies (prefix-stripped) **equal the original payload** byte-for-byte (lossless, FR-T2).

### Test (c) — `TestChunkPayload_PartPrefixMonotonic`

For a multi-chunk payload (e.g. `'abcd\n'×12`, CT=5 ⇒ N=3), assert for every chunk: (1) `chunk.text`
matches `^PART i/N:\n` where i = `chunk.index`, N = `chunk.total`; (2) `chunk.index` is strictly
monotonic 1..N; (3) `chunk.total == len(chunks)` for all; (4) the body (after the first `\n`) is
non-empty for non-empty payloads. Use `regexp.MustCompile(`^PART (\d+)/(\d+):\n`)` to extract i/N and
compare to the struct fields.

### Test (d) — `TestMultiTurnTriggerGate_TruthTable` (drives CommitStaged)

A `cases` slice; each row builds a repo + stub manifest + cfg, calls `CommitStaged` with
`Deps{Verbose: ui.NewVerbose(&buf, true)}`, then asserts on `buf.String()` (trigger present/absent) +
the result (rescue vs commit). The trigger string is `"one-shot exhausted → multi-turn fallback"`
(generate.go:311, the `VerboseWarn` arg). See §"Test cases" for the full table + per-row setup.

### Test (e) — token_limit non-interaction (two tests)

- `TestChunkPayload_TokenLimitNonInteraction` (pure helper): assert `len(chunkPayload(payload, ct))` is
  deterministic and that `cfg.TokenLimit` is **not a parameter** of `chunkPayload` (documented via
  comment + a behavioral re-invocation stability check). Proves multi-turn chunking is structurally
  TokenLimit-free.
- `TestMultiTurnGate_TokenLimitNotATerm` (gate-level, drives CommitStaged): with `cfg.TokenLimit` set to
  a value that does NOT truncate the small test diff (so StagedDiff passes it through), set
  `MultiTurnChunkTokens` small so multi-turn triggers, and assert the verbose trigger IS present (the
  gate fired) and the turn count reflects the FULL payload. A second invocation with a DIFFERENT
  `cfg.TokenLimit` (still non-truncating) yields the SAME trigger/turn count ⇒ TokenLimit did not alter
  multi-turn. Documents the S3↔FR-T12 caveat in a comment (see §"Implementation Patterns").

### Docs — `## Multi-turn generation fallback` in how-it-works.md

~25–40 lines inserted before `## Hook mode vs the snapshot-based flow`. Covers: the 4 FR-T1 triggers
(one-shot exhausted; payload > one chunk; multi_turn_fallback on; provider session_mode="append");
lossless chunking (full diff re-delivered across N+1 turns, NOT summarized — contrast the rejected lossy
map-reduce); the N+1 protocol (priming preamble, PART i/N chunks, final "write the message" turn); the
final turn reuses the normal parse + duplicate-rejection pipeline; failure falls back to the standard
rescue (snapshot safe); token_limit does not apply to multi-turn (FR-T12) — with an honest note that the
captured payload is what multi-turn chunks. Cross-links PRD §9.24.

### Success Criteria

- [ ] `TestChunkPayload_CeilMath` exists and asserts the 4 VERIFIED exact-N rows + monotonicity.
- [ ] `TestChunkPayload_NoFracturedBoundaries` asserts every non-last body ends on `\n` + lossless round-trip.
- [ ] `TestChunkPayload_PartPrefixMonotonic` asserts `^PART i/N:\n` + strict 1..N monotonicity + total==len.
- [ ] `TestMultiTurnTriggerGate_TruthTable` has the 4 skip rows + success row + all-true control; skip rows
      assert the `VerboseWarn` trigger is ABSENT + rescue; the control asserts it is PRESENT.
- [ ] `TestChunkPayload_TokenLimitNonInteraction` + `TestMultiTurnGate_TokenLimitNotATerm` exist and
      document the FR-T12 architectural separation + the S3 caveat.
- [ ] `how-it-works.md` has `## Multi-turn generation fallback` before `## Hook mode vs the snapshot-based flow`.
- [ ] The new tests PASS alongside all existing S1/S2/S3 tests (`go test -race ./internal/generate/...`).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean.
- [ ] `git diff --stat` shows ONLY `internal/generate/multiturn_test.go` + `docs/how-it-works.md`.
- [ ] NO production-code change; NO edits to `generate.go`, `generate_test.go`, `multiturn.go` (S1–S3's).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives: the VERIFIED exact-N payloads (the #1 risk — anchored N is not
naive ceil); the verbatim `chunkPayload`/`Run`/`chunk` contracts; the inline-gate testing strategy
(verbose-buffer trigger absence); the exact trigger string; the docs insertion anchor (line 262); the
import additions (`bytes`/`errors`/`ui`); the FR-T12 tension and how to handle it honestly; and the exact
validation commands. No inference required.

### Documentation & References

```yaml
# MUST READ — the FR spec, the seam contracts, and this task's research
- file: PRD.md
  why: "§9.24 FR-T1 (the 4-condition gate — the truth table's rows); FR-T3 (chunk sizing: N=ceil,
        newline-anchored, PART i/N outside budget — tests a/b/c); FR-T12 (no token_limit interaction —
        test e + the caveat); §20.1/§20.2 (unit pure-fn + lossless-round-trip invariants); §17.3/§13.3
        (the generation path the docs subsection summarizes)."
  critical: "FR-T3's 'N=ceil' is the IDEAL; the actual chunkPayload anchor makes N depend on line
             structure (research §2). Use the VERIFIED payloads. FR-T12's 'untruncated payload' is NOT
             fully realized by S3's captured-payload decision (D2) — document the caveat, do not test the
             full-diff case as 'passes'."

- docfile: plan/009_5c53066d64b3/architecture/research-tests-ui.md
  why: "§1 item 9 (CommitStaged test template + the NewScript call-indexed + clamping mechanism); §1 item
        10 (provider.Execute emits per-turn VerboseCommand/Payload/RawOutput/Stderr — the verbose buffer
        captures every turn); §2 (initRepo/writeFile/stageFile/headSHA/commitRaw/gitOut helpers + the
        verbose-capture pattern `ui.NewVerbose(&buf, true)` at generate_test.go:647)."
  critical: "§1 item 10 is WHY the verbose-buffer trigger-absence assertion works: the gate's VerboseWarn
             writes to the same *ui.Verbose sink. §2's NewScript clamps to the last line after exhaustion,
             so a script [''] forces one-shot exhaustion (cond a) deterministically."

- docfile: plan/009_5c53066d64b3/architecture/research-generate-config.md
  why: "§1c (EstimateTokens is NOT yet called directly on the payload except via the gate — the gate's
        condition b is its first direct payload-token call; the ceil-math tests reason about ET =
        ceil(runes/4))."
  critical: "EstimateTokens = ceil(runes/4) (internal/git/tokens.go:25). The ceil-math table's ET column
             is computed with this formula. runesPerWindow = chunkTokens*4 is chunkPayload's budget unit."

- docfile: plan/009_5c53066d64b3/P1M1T3S4/research/s4_tests_docs.md
  why: "THIS subtask's research: §2 the VERIFIED exact-N payloads (READ FIRST — the #1 one-pass risk);
        §3 the newline-anchoring proof; §4 the prefix format; §5 the truth-table strategy (verbose-
        absence); §6 the FR-T12 tension + S3 caveat; §7 the docs placement; §8 the reused helpers/imports."
  critical: "§2 is THE anchor: the implementing agent MUST use the pinned payloads (inventing arbitrary
             'chunkTokens+1→N=2' payloads FAILS). §6 says: do NOT write a passing test for the diff-
             exceeds-TokenLimit case — it diverges from FR-T12 under S3 and is integration territory."

- docfile: plan/009_5c53066d64b3/P1M1T3S3/PRP.md
  why: "The CONTRACT for the gate S4 tests against (S3 landed it inline in generate.go): the gate
        expression (4 conditions at generate.go:297), the VerboseWarn trigger string (generate.go:311:
        'one-shot exhausted → multi-turn fallback'), the captured-payload decision (D2 — the FR-T12
        tension source), and the byte-identical rescue return (FR-T7)."
  critical: "The gate is INLINE — there is NO standalone predicate fn to unit-test in isolation. S4 drives
             CommitStaged (the gate's sole call site). The trigger string at :311 is the assertion target.
             S3's D2 (pass captured payload) is the FR-T12 caveat S4 must document."

- docfile: plan/009_5c53066d64b3/P1M1T3S1/PRP.md
  why: "The CONTRACT for chunkPayload (LANDED): `func chunkPayload(payload string, chunkTokens int)
        []chunk` + the `chunk{index,total,text}` struct + the runesPerWindow=chunkTokens*4 budget + the
        forward-newline anchor. S4 tests this pure helper."
  critical: "chunkPayload is UNEXPORTED but same-package (generate) — accessible from multiturn_test.go.
             Do NOT modify it. The anchor semantics (research §2) are why the test payloads are pinned."

- docfile: plan/009_5c53066d64b3/P1M1T3S2/PRP.md
  why: "The CONTRACT for Run (LANDED): `func Run(ctx, deps, cfg, manifest, sysPrompt, payload, msgModel,
        msgReasoning) (msg, ok, cause)`. The truth table asserts Run was NOT entered (via the gate trigger
        absence) for the skip rows."
  critical: "Run is same-package; the truth table does NOT call Run directly — it observes whether the
             gate called it (via the VerboseWarn trigger the gate emits immediately before Run)."

# The seams (READ-ONLY — consumed by the tests; none modified)
- file: internal/generate/multiturn.go
  why: "READ-ONLY (S1/S2's file; HARD reference). chunkPayload + Run + chunk{index,total,text} +
        preambleFmt/finalInstruction. The tests assert on chunkPayload's output and (via CommitStaged) on
        whether Run was entered."
  gotcha: "Do NOT edit multiturn.go — S1/S2 own it. The tests live in multiturn_test.go (same package)."

- file: internal/generate/generate.go
  why: "READ-ONLY (S3's file; the gate). The inline FR-T1 gate at :297; the VerboseWarn trigger at :311
        (string 'one-shot exhausted → multi-turn fallback'); the rescue return (RescueError{Kind:ErrRescue}).
        CommitStaged + Deps{Git,Manifest,Verbose,Excludes} + Result are the test entry points."
  gotcha: "Do NOT edit generate.go. The gate is INLINE — test it via CommitStaged, not a predicate fn."

- file: internal/generate/multiturn_test.go
  why: "EDIT (file 1 of 2). S1/S2's existing chunkPayload + Run tests. S4 APPENDS the new tests. Has
        stripPartPrefix helper (reuse for body extraction) + stubAppendManifest helper (reuse for the
        SessionMode='append' stub). Imports: context, strings, testing, unicode/utf8, config, git,
        provider, stubtest."
  pattern: "Match the existing test style (plain if/t.Errorf, no testify; table-driven where natural).
            Reuse stripPartPrefix/stubAppendManifest. ADD imports bytes, errors, ui."
  gotcha: "DO NOT redeclare stripPartPrefix/stubAppendManifest (compile error). They already exist. ADD
           bytes/errors/ui to the import block (gofmt-sorted)."

- file: internal/generate/generate_test.go
  why: "READ-ONLY (S3's file + the fixture source). initRepo/writeFile/stageFile/commitRaw/headSHA/gitOut
        are `package generate` and IN SCOPE for multiturn_test.go (same package) — reuse, do NOT redeclare."
  gotcha: "S3 is editing generate_test.go in parallel. S4 must NOT touch it. The helpers are shared
           (same-package); adding tests to multiturn_test.go that CALL them is safe and non-conflicting."

- file: internal/ui/verbose.go
  why: "READ-ONLY. VerboseWarn(msg) at :103 (nil-safe; writes 'DEBUG: '+msg to the sink). NewVerbose(w,
        on) constructs the sink. The truth table captures the sink via `ui.NewVerbose(&buf, true)`."
  gotcha: "VerboseWarn is nil-safe (v==nil ⇒ no-op). For the truth table you MUST pass a non-nil Verbose
           with on=true so the trigger is captured into buf. Import internal/ui (NEW for multiturn_test.go)."

- file: docs/how-it-works.md
  why: "EDIT (file 2 of 2). The user-facing Mode A doc. Sections end at '## Hook mode vs the snapshot-based
        flow' (:262). INSERT the new '## Multi-turn generation fallback' section immediately BEFORE that
        heading (after the prompt-engineering section)."
  pattern: "Match the existing markdown style (## headings, concise prose, PRD cross-links like '(PRD
            §9.24)'). ~25–40 lines."
  gotcha: "Insert BEFORE '## Hook mode' (line 262), NOT inside another section. Cross-link PRD §9.24. Note
           the token_limit caveat honestly (S3's captured-payload decision)."

# External references
- url: https://pkg.go.dev/regexp#MustCompile
  why: "regexp.MustCompile(`^PART (\\d+)/(\\d+):\\n`) extracts i/N from a chunk's text for the prefix test.
        FindStringSubmatch yields [full, i, N] to compare against chunk.index/total."
  critical: "Use regexp (not string split) so the test asserts the EXACT prefix FORMAT (digits, slash,
            colon, newline) — a regression to 'Part 1 of 3' would be caught."
- url: https://pkg.go.dev/testing#hdr-Subtests
  why: "t.Run(name, func(t *testing.T){...}) for the table-driven ceil-math + truth-table cases. Named
        subtests pin WHICH row failed (essential for the truth table's 6 rows)."
  critical: "Use t.Run subtests for every table so a failure names the exact case. Do NOT collapse the
            truth table into one function with no subtest names."
```

### Current Codebase Tree (this task's scope — multiturn_test.go + how-it-works.md)

```bash
stagecoach/
├── internal/generate/
│   ├── multiturn.go          # READ-ONLY (S1/S2 — chunkPayload, Run, chunk, preambleFmt, finalInstruction)
│   ├── multiturn_test.go     # EDIT (file 1): + ceil-math / boundaries / prefix / truth-table / token-limit tests
│   ├── generate.go           # READ-ONLY (S3 — the inline gate at :297, the VerboseWarn trigger at :311)
│   ├── generate_test.go      # READ-ONLY (S3 + the fixture helpers initRepo/writeFile/stageFile/...)
│   └── (rescue/finalize/dedupe/invariants/realagent — READ-ONLY siblings)
└── docs/
    └── how-it-works.md       # EDIT (file 2): + "## Multi-turn generation fallback" before "## Hook mode" (:262)
# internal/stubtest/*, internal/ui/*, internal/config/*, internal/git/tokens.go = READ-ONLY seams
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
├── internal/generate/
│   └── multiturn_test.go     # + TestChunkPayload_CeilMath, _NoFracturedBoundaries, _PartPrefixMonotonic,
│                             #   TestMultiTurnTriggerGate_TruthTable, TestChunkPayload_TokenLimitNonInteraction,
│                             #   TestMultiTurnGate_TokenLimitNotATerm  (+ imports bytes/errors/ui)
└── docs/
    └── how-it-works.md       # + "## Multi-turn generation fallback" section (before "## Hook mode")
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/generate/multiturn_test.go` | MODIFY | Append the 6 unit tests (a–e) + add `bytes`/`errors`/`ui` imports. Reuse `stripPartPrefix`/`stubAppendManifest`/`initRepo`/`writeFile`/`stageFile`/`commitRaw`/`headSHA`. No production change. |
| `docs/how-it-works.md` | MODIFY | Insert the `## Multi-turn generation fallback` subsection before `## Hook mode vs the snapshot-based flow`. |

**Explicitly NOT touched**: `internal/generate/multiturn.go` (S1/S2), `internal/generate/generate.go` (S3), `internal/generate/generate_test.go` (S3), `internal/generate/{rescue,finalize,dedupe,invariants,realagent}*`, `internal/stubtest/*`, `internal/ui/*`, `internal/config/*`, `internal/git/*`, `internal/provider/*`, `cmd/*`, `README.md` (S5), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`, every other doc file.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — anchored N ≠ naive ceil; use the PINNED payloads). chunkPayload advances runesPerWindow
// runes then anchors FORWARD to the next '\n', so a chunk is runesPerWindow runes + up-to-one-line
// overshoot. A no-newline payload collapses to ONE chunk; a small overage is absorbed into chunk 1. So
// "chunkTokens+1 → N=2" is NOT achievable with an arbitrary payload. Use the VERIFIED payloads in research
// §2 ('abcd\n' family at CT=5 for N=1/2/3; 'ab\n'×33 at CT=10 for the 2.5×→3 case). Inventing your own
// "ET=CT+1" payload will FAIL the assertion. This is the #1 one-pass risk.

// CRITICAL (G2 — the gate is INLINE; there is NO predicate fn to unit-test in isolation). S3 landed the
// FR-T1 gate inside CommitStaged (generate.go:297). S4 tests it by driving CommitStaged and observing the
// gate's VerboseWarn trigger ("one-shot exhausted → multi-turn fallback", generate.go:311) in the verbose
// buffer. Trigger ABSENT ⇒ the gate did not pass ⇒ Run was NOT entered (the item's "assert it was NOT
// called"). Do NOT try to extract a predicate fn (that would edit generate.go — S3's territory).

// CRITICAL (G3 — the truth-table "was NOT called" assertion = verbose-buffer trigger ABSENCE). Build
// Deps{Verbose: ui.NewVerbose(&buf, true)} (the generate_test.go:647 pattern). After CommitStaged, assert
// !strings.Contains(buf.String(), "multi-turn fallback"). This is clean + uniform across all skip rows.
// Corroborate with errors.As(err, &re) && re.Kind == ErrRescue for the !success rows.

// CRITICAL (G4 — FR-T12 tension: S3 passes the CAPTURED payload, which StagedDiff may have TokenLimit-
// truncated). PRD §9.24 FR-T12 says multi-turn uses the UNTRUNCATED payload. S3's D2 passes the captured
// `payload` variable (built from diff = StagedDiff(...TokenLimit...)). So in the diff-exceeds-TokenLimit
// case, multi-turn receives a TRUNCATED payload. S4 CANNOT fix this (generate.go is S3's; S3 forbids the
// recompute). S4 tests the TRUE non-interaction (chunkPayload/gate never read TokenLimit) and DOCUMENTS
// the caveat in how-it-works.md. Do NOT write a passing test asserting the full-diff chunk count when
// TokenLimit would truncate — it would FAIL against S3 and is integration territory (P1.M1.T4).

// GOTCHA (G5 — non-overlap with S3 via FILE separation). S3 edits generate.go + generate_test.go; S4
// edits multiturn_test.go + how-it-works.md. ZERO file overlap ⇒ no merge conflict. multiturn_test.go is
// package generate, so it can call CommitStaged + reuse generate_test.go's helpers (same package). Do NOT
// add S4's tests to generate_test.go (would collide with S3's parallel edits).

// GOTCHA (G6 — reuse, do NOT redeclare). multiturn_test.go ALREADY has stripPartPrefix + stubAppendManifest
// (S1/S2). generate_test.go has initRepo/writeFile/stageFile/commitRaw/headSHA/gitOut. ALL are package
// generate, in scope for multiturn_test.go. Redeclaring any ⇒ compile error. Call them directly.

// GOTCHA (G7 — imports to ADD: bytes, errors, ui). multiturn_test.go currently imports context, strings,
        testing, unicode/utf8, config, git, provider, stubtest. ADD bytes (for the verbose buffer),
        errors (for errors.As into *RescueError), ui (for ui.NewVerbose). gofmt-sort them. Do NOT remove
        existing imports. (unicode/utf8 may become unused if you remove the CeilRounding reference — it is
        currently used by the existing _RuneBasedCJK test's RuneCountInString keepalive line; LEAVE the
        existing tests untouched so utf8 stays used.)

// GOTCHA (G8 — the stub NewScript CLAMPS to the last line after exhaustion). A script [""] forces the
// one-shot path to exhaust (call 1 = "" ⇒ parse-fail ⇒ cond a true). For the all-true control row, use
// ["", "ok", "ok", "feat: multi-turn win"] so one-shot exhausts on "" then multi-turn's turns get ok/ok/
// msg. For the success row (cond a false), use ["feat: win"] so one-shot SUCCEEDS on call 1 (loop breaks;
// gate not reached).

// GOTCHA (G9 — SessionMode is *string; the stubAppendManifest helper sets it). For rows needing cond (d)
// TRUE, use stubAppendManifest(t, bin, responses, false) (false ⇒ appendMode set). For cond (d) FALSE,
// use stubtest.NewScript directly (SessionMode unset ⇒ "") OR stubAppendManifest(..., true).

// GOTCHA (G10 — chunkTokens in tests must keep N bounded but ≥2 for the control). For the all-true
// control row, set cfg.MultiTurnChunkTokens small (e.g. 2–4) so EstimateTokens(payload) > chunkTokens
// (cond b true) AND N is small (a tiny diff ⇒ N≈2–4). Default 32000 ⇒ cond b false (the small-payload
// skip row). Do NOT use chunkTokens=1 with a real-ish diff (N explodes → slow).

// GOTCHA (G11 — the docs insertion anchor). how-it-works.md's "## Hook mode vs the snapshot-based flow"
// is at line 262. INSERT "## Multi-turn generation fallback" IMMEDIATELY BEFORE it (after the prompt-
// engineering section ends). A 2-line blank-line separator before the new ## heading, per markdown norm.

// GOTCHA (G12 — EstimateTokens = ceil(runes/4); runesPerWindow = chunkTokens*4). The ceil-math table's
// ET column uses ceil(runes/4). 'abcd\n' = 5 runes ⇒ ET=2; ×4 ⇒ 20 runes ⇒ ET=5=CT(5). Verified.
```

## Implementation Blueprint

### Data models and structure

None added or changed. The tests reuse `chunk`/`chunkPayload`/`Run`/`CommitStaged`/`Deps`/`Result`/
`RescueError`/`ErrRescue` (all same-package). No new types, constants, or production code. Two new test
imports (`bytes`, `errors`, `ui`).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/generate/multiturn_test.go (imports — add bytes, errors, ui)
  - EDIT the import block: add "bytes", "errors" (stdlib, gofmt-sorted) and "github.com/dustin/stagecoach/
    internal/ui" (internal group). Result includes the existing context/strings/testing/unicode-utf8 +
    config/git/provider/stubtest + the three new.
  - DO NOT remove any existing import (unicode/utf8 stays — used by the existing _RuneBasedCJK test).
  - VERIFY: go build ./internal/generate/ → exit 0 (imports unused until Task 2; build fails on unused —
    EXPECTED; proceed; Task 2 uses them). OR add the imports in the SAME edit as the first test that uses
    them (preferred — avoids a transient unused-import break).

Task 2: MODIFY internal/generate/multiturn_test.go (chunkPayload pure-helper tests — a/b/c/e-pure)
  - APPEND (after the existing chunkPayload tests, BEFORE the Run tests or after them — co-locate with the
    other chunkPayload tests for readability):
      TestChunkPayload_CeilMath          — table; 4 VERIFIED rows + monotonicity (research §2).
      TestChunkPayload_NoFracturedBoundaries — every non-last body ends on '\n' + round-trip (research §3).
      TestChunkPayload_PartPrefixMonotonic   — ^PART i/N:\n via regexp + strict 1..N (research §4).
      TestChunkPayload_TokenLimitNonInteraction — pure-helper; N deterministic; TokenLimit not a param (§6).
  - REUSE stripPartPrefix (do NOT redeclare). Use regexp.MustCompile for the prefix test.
  - DO NOT modify the existing chunkPayload tests (SingleChunk/MultiChunkSplit/NewlineAnchoring/etc.).
  - VERIFY: go test -race -run TestChunkPayload ./internal/generate/ → all (old + new) PASS.

Task 3: MODIFY internal/generate/multiturn_test.go (truth table — d)
  - APPEND TestMultiTurnTriggerGate_TruthTable — a cases slice with t.Run subtests. Each row: build repo +
    manifest + cfg, call CommitStaged with Deps{Verbose: ui.NewVerbose(&buf, true)}, assert on buf + err.
    Rows: skip-cond-c / skip-cond-b / skip-cond-d / success-cond-a / control-all-true. See §"Test cases".
  - REUSE initRepo/commitRaw/writeFile/stageFile/headSHA (generate_test.go) + stubAppendManifest (this file).
  - DO NOT call Run directly; observe the gate via the verbose trigger.
  - VERIFY: go test -race -run TestMultiTurnTriggerGate ./internal/generate/ → all rows PASS.

Task 4: MODIFY internal/generate/multiturn_test.go (token_limit gate-level test — e-gate)
  - APPEND TestMultiTurnGate_TokenLimitNotATerm — drive CommitStaged with a non-truncating cfg.TokenLimit +
    small MultiTurnChunkTokens; assert the trigger IS present (gate fired) and a second run with a different
    non-truncating TokenLimit yields the same outcome. Comment the FR-T12/S3 caveat (G4).
  - VERIFY: go test -race -run TestMultiTurnGate ./internal/generate/ → PASS.

Task 5: MODIFY docs/how-it-works.md (Mode A subsection)
  - FIND the line "## Hook mode vs the snapshot-based flow" (≈line 262).
  - INSERT IMMEDIATELY BEFORE it the "## Multi-turn generation fallback" section (verbatim from §"Docs
    content" below). Keep a blank line before the new ## and after it (markdown norm).
  - DO NOT edit any other section. Cross-link PRD §9.24. Note the token_limit caveat honestly.
  - VERIFY: grep -n '## Multi-turn generation fallback' docs/how-it-works.md → exactly one match, before
    '## Hook mode'.

Task 6: VALIDATE — full gate set + scope discipline
  - RUN: gofmt -w internal/generate/multiturn_test.go docs/how-it-works.md ; gofmt -l .
  - RUN: go vet ./... ; go build ./...
  - RUN: go test -race ./internal/generate/... -v   # all S1/S2/S3 + new S4 tests green
  - RUN: go test -race ./...                         # whole repo green (additive: 2 files)
  - RUN (scope): git status --porcelain → expect EXACTLY:
        M internal/generate/multiturn_test.go
        M docs/how-it-works.md
  - RUN (no production edit): git diff --name-only → the two files above; NO generate.go/multiturn.go/
        generate_test.go.
```

### Test cases

#### `TestChunkPayload_CeilMath` (table — VERIFIED exact N)

```go
func TestChunkPayload_CeilMath(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		ct      int
		wantN   int
	}{
		{"one_chunk_exact_CT", strings.Repeat("abcd\n", 4), 5, 1},   // 20 runes, ET=5=CT
		{"two_chunks_2x_CT", strings.Repeat("abcd\n", 8), 5, 2},     // 40 runes, ET=10=2×CT
		{"three_chunks_3x_CT", strings.Repeat("abcd\n", 12), 5, 3},  // 60 runes, ET=15=3×CT
		{"two_half_x_ceil", strings.Repeat("ab\n", 33), 10, 3},      // 99 runes, ET=25=2.5×CT → N=3
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chunks := chunkPayload(tc.payload, tc.ct)
			if len(chunks) != tc.wantN {
				t.Errorf("len(chunks) = %d, want %d (payload %q, ct %d)", len(chunks), tc.wantN, tc.payload, tc.ct)
			}
		})
	}
	// Monotonicity: for the 'abcd\n'/CT=5 family, N is non-decreasing as the payload grows.
	prev := 0
	for _, n := range []int{4, 8, 12} {
		got := len(chunkPayload(strings.Repeat("abcd\n", n), 5))
		if got < prev {
			t.Errorf("monotonicity violated: N dropped from %d to %d at ×%d", prev, got, n)
		}
		prev = got
	}
}
```

#### `TestChunkPayload_NoFracturedBoundaries`

```go
func TestChunkPayload_NoFracturedBoundaries(t *testing.T) {
	// 6-rune lines; CT=1 ⇒ runesPerWindow=4 lands mid-line ("aaaa"), anchoring forward to the line's '\n'.
	payload := "aaaaaa\nbbbbbb\ncccccc\n"
	chunks := chunkPayload(payload, 1)
	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want ≥2 (payload exceeds the 1-token window)", len(chunks))
	}
	for i, c := range chunks {
		body := stripPartPrefix(t, c.text)
		if i < len(chunks)-1 && !strings.HasSuffix(body, "\n") {
			t.Errorf("chunk %d body does not end on '\\n' (fractured diff line): %q", i, body)
		}
	}
	// Lossless round-trip (FR-T2): concatenated bodies == original payload.
	var rebuilt strings.Builder
	for _, c := range chunks {
		rebuilt.WriteString(stripPartPrefix(t, c.text))
	}
	if rebuilt.String() != payload {
		t.Errorf("round-trip mismatch\ngot:  %q\nwant: %q", rebuilt.String(), payload)
	}
}
```

#### `TestChunkPayload_PartPrefixMonotonic`

```go
var partPrefixRe = regexp.MustCompile(`^PART (\d+)/(\d+):\n`)

func TestChunkPayload_PartPrefixMonotonic(t *testing.T) {
	payload := strings.Repeat("abcd\n", 12) // CT=5 ⇒ N=3
	chunks := chunkPayload(payload, 5)
	n := len(chunks)
	for i, c := range chunks {
		m := partPrefixRe.FindStringSubmatch(c.text)
		if m == nil {
			t.Errorf("chunk %d text does not match ^PART i/N:\\n: %q", i, c.text)
			continue
		}
		gotI, gotN := m[1], m[2]
		wantI := strconv.Itoa(c.index)
		wantN := strconv.Itoa(c.total)
		if gotI != wantI || gotN != wantN {
			t.Errorf("chunk %d prefix = PART %s/%s, want PART %s/%s (struct index/total)", i, gotI, gotN, wantI, wantN)
		}
		if c.index != i+1 {
			t.Errorf("chunk %d index = %d, want %d (not monotonic)", i, c.index, i+1)
		}
		if c.total != n {
			t.Errorf("chunk %d total = %d, want %d (N inconsistent)", i, c.total, n)
		}
	}
}
```
> NOTE: this test adds `regexp` and `strconv` to multiturn_test.go's imports (gofmt-sort them). If you
> prefer to avoid `strconv`, compare `m[1]` to `fmt.Sprintf("%d", c.index)` (fmt is not currently imported
> in multiturn_test.go either; strconv is the lighter choice). Pick one; keep gofmt-clean.

#### `TestMultiTurnTriggerGate_TruthTable` (drives CommitStaged)

| Row (t.Run name) | cond flipped | cfg / manifest setup | script | want trigger in buf? | want err |
|---|---|---|---|---|---|
| `skip_cond_c_multiturn_off` | (c) false | `cfg.MultiTurnFallback=false`; CT=4; SessionMode="append" | `[""]` | NO | `*RescueError{ErrRescue}` |
| `skip_cond_b_small_payload` | (b) false | defaults (CT=32000); SessionMode="append" | `[""]` | NO | `*RescueError{ErrRescue}` |
| `skip_cond_d_non_append` | (d) false | CT=4; SessionMode unset (`""`) | `[""]` | NO | `*RescueError{ErrRescue}` |
| `success_cond_a_not_exhausted` | (a) false | CT=4; SessionMode="append" | `["feat: win"]` | NO (gate not reached) | nil; commit lands |
| `control_all_true_gate_fires` | none | CT=4; SessionMode="append"; `MultiTurnFallback=true` | `["", "ok", "ok", "feat: mt win"]` | **YES** | nil; commit lands |

```go
func TestMultiTurnTriggerGate_TruthTable(t *testing.T) {
	bin := stubtest.Build(t)

	cases := []struct {
		name        string
		multiTurn   bool
		chunkTokens int
		append      bool // SessionMode: true⇒"append", false⇒unset("")
		script      []string
		wantTrigger bool
		wantRescue  bool
	}{
		{"skip_cond_c_multiturn_off", false, 4, true, []string{""}, false, true},
		{"skip_cond_b_small_payload", true, 32000, true, []string{""}, false, true},
		{"skip_cond_d_non_append", true, 4, false, []string{""}, false, true},
		{"success_cond_a_not_exhausted", true, 4, true, []string{"feat: win"}, false, false},
		{"control_all_true_gate_fires", true, 4, true, []string{"", "ok", "ok", "feat: mt win"}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			initRepo(t, repo)
			commitRaw(t, repo, "initial")
			writeFile(t, repo, "new.txt", strings.Repeat("change line\n", 8)) // enough body; cond b when CT small
			stageFile(t, repo, "new.txt")

			m := stubAppendManifest(t, bin, tc.script, !tc.append) // omitAppend = !tc.append
			cfg := config.Defaults()
			cfg.MaxDuplicateRetries = 0 // one-shot: exactly 1 attempt (the script's call 1)
			cfg.MultiTurnFallback = tc.multiTurn
			cfg.MultiTurnChunkTokens = tc.chunkTokens

			var buf bytes.Buffer
			_, err := CommitStaged(context.Background(), Deps{
				Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&buf, true),
			}, cfg)

			gotTrigger := strings.Contains(buf.String(), "multi-turn fallback")
			if gotTrigger != tc.wantTrigger {
				t.Errorf("trigger-in-buf = %v, want %v (buf tail: %q)", gotTrigger, tc.wantTrigger, tail(buf.String(), 200))
			}
			if tc.wantRescue {
				var re *RescueError
				if !errors.As(err, &re) || re.Kind != ErrRescue {
					t.Errorf("err = %v, want *RescueError{Kind:ErrRescue}", err)
				}
			} else if err != nil && tc.wantTrigger {
				// control row: multi-turn may still succeed (commit) — err should be nil on the happy script.
				t.Errorf("err = %v, want nil (control row: multi-turn should succeed)", err)
			}
		})
	}
}

// tail returns the last n bytes of s (for readable failure messages).
func tail(s string, n int) string { if len(s) > n { return s[len(s)-n:] }; return s }
```
> Row-specific notes:
> - **skip_cond_b**: the small `"change line\n"×8` diff is ~96 runes ⇒ ET=24 ≪ 32000 ⇒ cond (b) false.
> - **success_cond_a**: script `["feat: win"]` ⇒ one-shot call 1 returns "feat: win" ⇒ parses + not-dup ⇒
>   loop breaks (success) ⇒ `!success` is false ⇒ gate not reached ⇒ no trigger, no rescue, commit lands.
> - **control_all_true**: call 1 = "" ⇒ one-shot exhausts (cond a); CT=4 ⇒ ET(24)>4 (cond b);
>   MultiTurnFallback=true (c); SessionMode="append" (d) ⇒ gate fires ⇒ trigger PRESENT ⇒ multi-turn turns
>   get ok/ok/"feat: mt win" ⇒ final parses ⇒ commit. (If the diff is too small for cond b at CT=4, lower
>   the body or raise the repeat count; ET must exceed CT. Verified: ×8 ⇒ ET=24 > 4.)
> - `tail` is a NEW tiny helper — declare it once in the file (or inline `buf.String()` with a length cap).

#### `TestChunkPayload_TokenLimitNonInteraction` + `TestMultiTurnGate_TokenLimitNotATerm` (e)

```go
// FR-T12: multi-turn chunk sizing is architecturally independent of cfg.TokenLimit. chunkPayload's
// signature is (payload string, chunkTokens int) — TokenLimit is NOT a parameter, so no TokenLimit value
// can change N for a given (payload, chunkTokens). This is the strongest claim testable at the pure-helper
// layer. (NOTE: at the CommitStaged layer, StagedDiff DOES consult TokenLimit to BUILD the diff; see
// TestMultiTurnGate_TokenLimitNotATerm + the FR-T12 caveat in how-it-works.md.)
func TestChunkPayload_TokenLimitNonInteraction(t *testing.T) {
	payload := strings.Repeat("abcd\n", 8) // 40 runes
	n := len(chunkPayload(payload, 5))
	if n != 2 {
		t.Fatalf("baseline N = %d, want 2", n)
	}
	// Re-invocation is stable (deterministic); and there is NO TokenLimit argument to vary.
	if got := len(chunkPayload(payload, 5)); got != n {
		t.Errorf("non-deterministic: N flipped %d → %d", n, got)
	}
	// The claim is structural: chunkPayload has no TokenLimit parameter. A grep of multiturn.go confirms
	// the signature is chunkPayload(payload string, chunkTokens int) []chunk — TokenLimit is absent.
}

// FR-T12 gate-level: the FR-T1 gate (generate.go) reads only (MultiTurnFallback, EstimateTokens(payload),
// MultiTurnChunkTokens, SessionMode) — TokenLimit is NOT a term. With a NON-truncating TokenLimit (test
// diff < TokenLimit, so StagedDiff passes it through), multi-turn fires identically regardless of the
// TokenLimit value. (CAVEAT: when TokenLimit WOULD truncate the diff, S3's captured-payload decision hands
// multi-turn the truncated payload — a known divergence from FR-T12's literal "untruncated," documented in
// how-it-works.md and deferred to a future re-capture fix. We do NOT assert the full-diff case here.)
func TestMultiTurnGate_TokenLimitNotATerm(t *testing.T) {
	for _, tl := range []int{0, 1000, 100000} { // all NON-truncating for the small test diff
		bin := stubtest.Build(t)
		repo := t.TempDir()
		initRepo(t, repo)
		commitRaw(t, repo, "initial")
		writeFile(t, repo, "new.txt", strings.Repeat("change line\n", 8))
		stageFile(t, repo, "new.txt")

		m := stubAppendManifest(t, bin, []string{"", "ok", "ok", "feat: mt win"}, false)
		cfg := config.Defaults()
		cfg.MaxDuplicateRetries = 0
		cfg.MultiTurnChunkTokens = 4 // cond b true (ET≈24 > 4)
		cfg.TokenLimit = tl          // varied; none truncate the small diff

		var buf bytes.Buffer
		_, err := CommitStaged(context.Background(), Deps{
			Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&buf, true),
		}, cfg)
		if err != nil {
			t.Errorf("TokenLimit=%d: CommitStaged err = %v, want nil (multi-turn should succeed)", tl, err)
		}
		if !strings.Contains(buf.String(), "multi-turn fallback") {
			t.Errorf("TokenLimit=%d: trigger absent; want present (gate must fire regardless of TokenLimit)", tl)
		}
	}
}
```

### Docs content — `## Multi-turn generation fallback` (insert before `## Hook mode`)

```markdown
## Multi-turn generation fallback

For diffs too large for a single reliable request, stagecoach has an optional **multi-turn** generation
path (PRD §9.24). It exists because a provider's *per-request* reliability ceiling can lie well below its
advertised context window: a huge one-shot request may return empty or unparseable output even though the
model can handle the same content delivered in smaller pieces.

**When it triggers.** Multi-turn runs ONLY when all four hold: (1) the normal one-shot path exhausted its
retries on empty/unparseable output; (2) the captured payload exceeds one chunk (`multi_turn_chunk_tokens`,
default 32000); (3) `multi_turn_fallback` is enabled (default `true`); and (4) the resolved provider
declares `session_mode = "append"` (the **pi** provider does; others ship `""` until verified). If any
condition fails, the run proceeds to the normal rescue protocol unchanged — multi-turn is strictly an
extra attempt, never a worse outcome.

**Lossless, not summarized.** Multi-turn is deliberately *not* the lossy "chunk-summarize-combine" pattern.
The full captured diff is re-delivered across N+1 session turns in request-sized pieces — the model sees
the entire diff in its session history — then writes one message at the end:

- **Turn 1:** the normal system prompt + a priming preamble + the first chunk.
- **Turns 2..N:** each remaining chunk, prefixed `PART i/N:`. Boundaries anchor to newlines so no diff line
  is fractured.
- **Turn N+1:** "Now write the commit message for the diff above." This turn's output runs through the
  normal parse + duplicate-rejection pipeline, then commits like any other message.

Each turn is a separate provider invocation with its own timeout; total wall-clock ≈ `timeout × (N+1)`,
surfaced on the progress line at fallback time.

**Failure handling.** If any turn errors, times out, or the final output fails to parse/dedupe, the
multi-turn attempt aborts and control passes to the standard rescue protocol — the snapshot is safe and
the run is no worse off than a one-shot failure.

**`token_limit` does not apply (FR-T12).** `token_limit` governs only the one-shot path (it truncates the
payload to fit one request). Multi-turn deliberately ignores it: the whole point is lossless delivery of a
large payload. (Caveat: in this release the multi-turn path operates on the payload captured for the
one-shot attempt, so when `token_limit` is set and the diff exceeds it, multi-turn receives the already-
truncated payload rather than re-capturing the full diff. The chunking itself never consults `token_limit`.)
```

### Implementation Patterns & Key Details

```go
// === Why VERIFIED payloads (not naive ceil) — G1 ===
// chunkPayload's forward-newline anchor makes N depend on line structure. The 'abcd\n'/CT=5 family is
// clean because the 5-rune line divides runesPerWindow (20), bounding overshoot. The 2.5×→3 case uses
// 'ab\n'×33/CT=10 (verified). DO NOT invent "ET=CT+1 → N=2" — anchoring absorbs it. Use the table's rows.

// === Why drive CommitStaged (not a predicate fn) for the truth table — G2/G3 ===
// The FR-T1 gate is INLINE in CommitStaged (generate.go:297); there is no standalone predicate to isolate.
// The item explicitly allows "assert it was NOT called": observe the gate's VerboseWarn trigger
// ("one-shot exhausted → multi-turn fallback", generate.go:311) in the verbose buffer. ABSENT ⇒ gate did
// not pass ⇒ Run not entered. This is uniform across all skip rows.

// === Why verbose-buffer trigger absence (not call-counting) — G3 ===
// The trigger fires ONLY inside the gate (generate.go:311), immediately before Run. Its absence in the
// captured *ui.Verbose buffer is the cleanest proof the gate short-circuited. provider.Execute's per-turn
// VerboseCommand lines would be a secondary signal, but the trigger is the single authoritative one.

// === Why the FR-T12 test is honest about the S3 caveat — G4 ===
// FR-T12 says "untruncated payload." S3's D2 passes the captured payload (StagedDiff may have TokenLimit-
// truncated it). So in the diff-exceeds-TokenLimit case, multi-turn gets a truncated payload — diverging
// from FR-T12. S4 tests the TRUE non-interaction (chunkPayload/gate never read TokenLimit) and DOCUMENTS
// the caveat in how-it-works.md. Do NOT write a passing full-diff-when-TokenLimit-truncates test (would
// FAIL against S3; integration territory). Flag it for a future re-capture fix.

// === Why file separation avoids S3 conflict — G5 ===
// S3 edits generate.go + generate_test.go. S4 edits multiturn_test.go + how-it-works.md. Zero file overlap.
// multiturn_test.go is package generate ⇒ can call CommitStaged + reuse generate_test.go's helpers.

// === Why regexp for the prefix test — (c) ===
// regexp.MustCompile(`^PART (\d+)/(\d+):\n`) asserts the EXACT format (digits/slash/colon/newline) and
// extracts i/N for comparison to the struct fields. A regression to "Part 1 of 3" or "PART1/3" is caught.

// === Why t.Run subtests for the tables — (a)/(d) ===
// A failure names the exact row (essential for the 5-row truth table). Do not collapse into one function.
```

### Integration Points

```yaml
TESTS (internal/generate/multiturn_test.go — MODIFIED):
  - + TestChunkPayload_CeilMath / _NoFracturedBoundaries / _PartPrefixMonotonic (pure helper)
  - + TestChunkPayload_TokenLimitNonInteraction (pure helper, FR-T12)
  - + TestMultiTurnTriggerGate_TruthTable (drives CommitStaged; 5 rows)
  - + TestMultiTurnGate_TokenLimitNotATerm (drives CommitStaged; FR-T12 + caveat)
  - + imports: bytes, errors, ui (and regexp/strconv if the prefix test uses them)
  - + tiny helper tail(s, n) for readable buf failure messages (declare once)

DOCS (docs/how-it-works.md — MODIFIED):
  - + "## Multi-turn generation fallback" section (before "## Hook mode vs the snapshot-based flow")

CONSUMED (READ-ONLY — the seams; none modified):
  - internal/generate/multiturn.go: chunkPayload(payload, chunkTokens) []chunk ; chunk{index,total,text} ; Run(...)
  - internal/generate/generate.go: CommitStaged ; Deps{Git,Manifest,Verbose,Excludes} ; Result ; RescueError{Kind} ; ErrRescue ; the inline gate + the VerboseWarn trigger
  - internal/generate/multiturn_test.go (pre-existing): stripPartPrefix, stubAppendManifest
  - internal/generate/generate_test.go (pre-existing): initRepo, writeFile, stageFile, commitRaw, headSHA, gitOut
  - internal/ui/verbose.go: NewVerbose(w, on) ; VerboseWarn(msg)
  - internal/git/tokens.go: EstimateTokens(s) = ceil(runes/4)  (for ET reasoning; already imported as git)
  - internal/stubtest: Build, NewScript, Manifest  (already imported)
  - internal/config: Defaults() ; Config{MultiTurnFallback, MultiTurnChunkTokens, TokenLimit, MaxDuplicateRetries}
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/generate/multiturn_test.go docs/how-it-works.md
go vet ./...
go build ./...
gofmt -l .   # expect: empty
# Expected: Zero errors. The new imports (bytes/errors/ui[/regexp/strconv]) must all be USED (else
# unused-import). If you add regexp/strconv for the prefix test, USE them; otherwise omit.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new chunkPayload tests (pure helper)
go test -race ./internal/generate/ -v -run 'TestChunkPayload_(CeilMath|NoFracturedBoundaries|PartPrefixMonotonic|TokenLimitNonInteraction)'

# The truth table + gate-level token test (drive CommitStaged)
go test -race ./internal/generate/ -v -run 'TestMultiTurn(TriggerGate_TruthTable|Gate_TokenLimitNotATerm)'

# Full generate suite (S1/S2/S3 + S4 all green)
go test -race ./internal/generate/... -v

# Expected: All PASS. If CeilMath fails, the payload/CT is wrong — use the VERIFIED table (G1). If the
# truth table's control row fails the trigger assertion, cond (b) is false (raise MultiTurnChunkTokens
# gap: ET must exceed CT) — see G10.
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-repo green (S4 is additive: 2 files, no production change)
go test -race ./...

# Docs anchor verification
grep -n '## Multi-turn generation fallback' docs/how-it-works.md   # exactly one match
grep -n '## Hook mode vs the snapshot-based flow' docs/how-it-works.md   # the new section is BEFORE this line
awk '/## Multi-turn generation fallback/{mt=NR} /## Hook mode vs the snapshot-based flow/{if(mt && NR>mt){print "OK: multi-turn before hook mode"; exit}} END{print "FAIL: ordering"}' docs/how-it-works.md

# Expected: all tests pass; the docs anchor is present and ordered before "## Hook mode".
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Scope-discipline grep (enforce NO production-code edit + NO overlap with S3):
git status --porcelain   # expect EXACTLY: M internal/generate/multiturn_test.go ; M docs/how-it-works.md
git diff --name-only     # the two files above; NO generate.go / multiturn.go / generate_test.go

# Confirm the truth-table trigger string matches the gate's VerboseWarn arg (generate.go:311):
grep -n 'one-shot exhausted → multi-turn fallback' internal/generate/generate.go   # the assertion target
grep -n 'multi-turn fallback' internal/generate/multiturn_test.go                   # the test's Contains target

# Confirm the FR-T12 caveat is present in the docs (honesty gate):
grep -n 'token_limit' docs/how-it-works.md | grep -i 'multi-turn\|caveat\|truncat'

# Expected: exactly 2 modified files; the trigger string matches; the docs note the token_limit caveat.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go test -race ./internal/generate/...` green (S1/S2/S3 + new S4 tests).
- [ ] `go vet ./...` clean; `gofmt -l .` empty; `go build ./...` exit 0.
- [ ] `git status --porcelain` shows EXACTLY multiturn_test.go + how-it-works.md (no production edit).

### Feature Validation

- [ ] CeilMath asserts the 4 VERIFIED exact-N rows + monotonicity.
- [ ] NoFracturedBoundaries asserts every non-last body ends on `\n` + lossless round-trip.
- [ ] PartPrefixMonotonic asserts `^PART i/N:\n` (regexp) + strict 1..N + total==len.
- [ ] TriggerGate_TruthTable: 3 skip rows (trigger ABSENT + rescue) + success row (no trigger, commit) + control (trigger PRESENT).
- [ ] TokenLimitNonInteraction (pure) + TokenLimitNotATerm (gate) document FR-T12 + the S3 caveat.
- [ ] how-it-works.md has the new section before "## Hook mode", cross-linking PRD §9.24, with the token_limit caveat.

### Code Quality Validation

- [ ] Follows existing test style (plain if/t.Errorf, no testify; t.Run subtests for tables).
- [ ] Reuses stripPartPrefix/stubAppendManifest/initRepo/.../headSHA (no redeclaration).
- [ ] Anti-patterns avoided: no invented ceil-math payloads (G1); no predicate-fn extraction (G2); no
      passing full-diff-when-TokenLimit-truncates test (G4); no edits to S1/S2/S3 files (G5).
- [ ] New imports (bytes/errors/ui[/regexp/strconv]) all USED and gofmt-sorted.

### Documentation & Deployment

- [ ] Docs subsection is concise (~25–40 lines), matches existing markdown style, cross-links PRD §9.24.
- [ ] The token_limit caveat is stated honestly (S3's captured-payload limitation).
- [ ] No new env vars / config (tests + docs only).

---

## Anti-Patterns to Avoid

- ❌ Don't invent "chunkTokens+1 → N=2" with an arbitrary payload — the forward-newline anchor absorbs the
      overage. Use the VERIFIED table (research §2 / G1).
- ❌ Don't extract a gate predicate function to unit-test in isolation — the gate is inline in generate.go
      (S3's territory); drive CommitStaged instead (G2).
- ❌ Don't write a test asserting the FULL-diff chunk count when TokenLimit would truncate it — S3's
      captured-payload decision means multi-turn receives the truncated payload; that test FAILS against S3
      and is integration territory (G4). Test the TRUE non-interaction + document the caveat.
- ❌ Don't add S4's tests to generate_test.go — S3 is editing it in parallel; use multiturn_test.go (G5).
- ❌ Don't redeclare stripPartPrefix/stubAppendManifest/initRepo/... — they're same-package, in scope (G6).
- ❌ Don't skip t.Run subtests for the tables — a failure must name the exact row.
- ❌ Don't forget the docs caveat — hiding the FR-T12↔S3 gap misleads users into assuming multi-turn always
      sees the full diff.
- ❌ Don't edit any production file (multiturn.go/generate.go/...) — S4 is tests + docs only.
