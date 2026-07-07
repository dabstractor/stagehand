# S5 Context Synthesis — RenderMultiTurn golden/table tests + Mode A provider docs

Scout/research synthesis for **P1.M1.T1.S5**. Read-only. No files were modified.
Records the EXACT landed state of the tree (S1–S4) and the precise remaining work, so the
implementer does not duplicate S3's tests or re-derive the docs insertion points.

---

## 1. The S3-landed reality (the load-bearing finding)

S3 is **COMPLETE**. `internal/provider/render.go:203` already defines `RenderMultiTurn`, and
`internal/provider/render_test.go` already contains **all 4** of the tests the S5 work item's logic
bullets (a)/(b)/(d)/(e) describe:

| S5 item case | S3 test (already landed) | File:line | Asserts |
|---|---|---|---|
| (a) pi turn 1 | `TestRenderMultiTurn_PiTurn1_Golden` | render_test.go:503 | byte-exact `wantArgs` (DeepEqual) + `containsToken(--session-id)` + `!containsToken(--no-session)` + `--system-prompt <sys>` + `-p` + Stdin=payload |
| (b) pi turn 2 | `TestRenderMultiTurn_PiTurn2_NoSysPromptFlag_NoPrepend` | render_test.go:537 | NO `--system-prompt`, Stdin=payload (no prepend) even though `sysPrompt` arg was `<sys>` |
| (d) non-append errors | `TestRenderMultiTurn_NonAppendProviderErrors` | render_test.go:562 | `SessionMode: strPtr("")` → `err != nil`, `err` contains `"session_mode"`, `spec == nil` |
| (e) immutability | `TestRenderMultiTurn_DoesNotMutateManifest` | render_test.go:589 | `m.BareFlags` snapshot unchanged after call; still contains `--no-session` |

A shared helper `mtPiManifest()` (render_test.go:487) returns the pi-shape Manifest literal with
`SessionMode: strPtr("append")` + the full `--no-session`-bearing BareFlags slice — **reuse it** in the
new S5 tests (do NOT redefine a parallel literal).

Helpers `containsPair` (606) and `containsToken` (616) are the established presence-assertion idiom
(research-tests-ui.md §5: "assert a flag-value pair or single token").

## 2. The genuine remaining TEST work for S5

Two things the S5 contract asks for that S3 did NOT deliver:

1. **(c) cross-turn sessionID stability** — S3 uses the SAME literal `"stagecoach-test"` for turn 1 and
   turn 2 but never EXPLICITLY asserts the sessionID string renders identically across turns. This is the
   one real coverage gap. Add `TestRenderMultiTurn_SessionIDStableAcrossTurns`: call `RenderMultiTurn`
   for turn 1 AND turn 2 (and optionally turn 3) with one sessionID; assert
   `containsPair(spec.Args, "--session-id", sessionID)` is true for EVERY turn's spec (proving the same
   id appears after `--session-id` on each turn → stable across turns).

2. **The "golden/table" cases-slice form** — the item TITLE and description both say "golden/table" and
   reference `TestRender_GoldenPerProvider` (render_test.go:30 — the §5 table-driven template) as the
   idiom. S3 used INDIVIDUAL functions, not a `cases` slice. S5 adds the consolidated table-driven
   representation: `TestRenderMultiTurn_GoldenTable`, a `cases` slice of `{name, turn, wantSessionID,
   wantSysPromptPresent}` run under `t.Run`, asserting **presence** via `containsPair`/`containsToken`
   (NOT byte-exact DeepEqual — that would duplicate S3's individual tests; the item explicitly says
   "assert presence + the system-prompt turn-1-only distinction" and "Do NOT assert exact arg position").

   This makes the table test **complementary, not duplicative**: S3's individual tests pin byte-exact
   args; S5's table test pins the contract surface (session-id present, no-session absent, system-prompt
   turn-1-only, -p present) in the codebase's documented table idiom.

## 3. The DOCS reality (the larger, entirely-undone deliverable)

S4's contract point 5 is explicit: "DOCS: none — the providers.md session_mode doc rides with P1.M1.T1.S5."
So BOTH doc edits are S5's and only S5's.

### 3a. `docs/providers.md` — the manifest-schema section is STALE

The schema table (providers.md:15–35) lists **21 fields with NO `session_mode` row**, and the prose says
"the 21-field schema" (line 3) and "Each manifest has 21 fields" (line 13). But `internal/provider/
manifest.go` ALREADY has the `SessionMode *string toml:"session_mode"` field (line 66, between
ProviderFlag:59 and BareFlags:69) — so the struct now has **22 fields**. The doc is stale by exactly one
field. S5 edits:

- **Line 3**: "the 21-field schema" → "the 22-field schema".
- **Line 13**: "Each manifest has 21 fields (matching the TOML tags in `internal/provider/manifest.go`):"
  → "...**22** fields...".
- **After line 28** (`| provider_flag | ...`): insert a NEW row `| session_mode | ... | "" | ... |`
  between `provider_flag` and `bare_flags` (PRD §12.1 ordering — matches the struct's slot).
- **Prose note** (near the schema or in a short new subsection): ONLY pi declares `"append"` today
  (**VERIFIED 2026-07-05 per FR-T9** — see architecture/fr-t9-verification.md); every other built-in
  ships `""`; adding another provider requires the FR-T9 empirical append-turn verification bar; cross-link
  §9.24 (multi-turn fallback).

### 3b. `docs/configuration.md` — the `[provider.<name>]` override surface

configuration.md does **NOT** have a per-field row table for `[provider.<name>]` (that lives in
providers.md's schema table). configuration.md surfaces the **merge semantics** at line 22:
> "When a `[provider.<name>]` section appears in a config file, its fields are **merged onto** the
> built-in manifest of the same name (field-by-field: present values override, absent values inherit)."

The item's "if provider-level override is surfaced there" resolves to: **yes — as merge semantics, not a
row table**. So the S5 edit is a short note near line 22 documenting that `session_mode` is
config-overridable with S2's semantics — an **explicit `session_mode = ""` on pi disables multi-turn
fallback** (overrides the built-in `"append"`); omitting the key inherits the built-in; setting `"append"`
on a non-append provider is a user override at their own FR-T9 risk. Cross-link providers.md#the-schema.
This mirrors how `output`/`strip_code_fence` override semantics are discussed (configuration.md:143).

## 4. The FR-T9 authority (the docs wording source)

`architecture/fr-t9-verification.md` records the VERIFIED pi flag set (2026-07-05 live run): BareFlags
MINUS `--no-session`, PLUS `--session-id <id>`; `--system-prompt` turn-1-only; `-p` kept;
`--continue`/`-c` NOT used. The verdict: re-invoking the same `--session-id` appends a recallable turn
(turn 1 "remember BANANA" → turn 2 "BANANA"). This file is the authority for the "VERIFIED 2026-07-05;
FR-T9" wording in the providers.md note and for the table-test's expected args.

## 5. Sibling-task boundaries (what S5 does NOT touch)

- `internal/provider/render.go` (S3, LANDED) — the renderer; S5 adds NO render code.
- `internal/provider/manifest.go` (S1, LANDED) — the field + Resolve + Validate; S5 adds NO schema code.
- `internal/provider/merge.go` (S2, LANDED) — the override clause; S5 only DOCUMENTS its semantics.
- `internal/provider/builtin.go` + `providers/pi.toml` (S4, parallel) — the shipped `"append"` value +
  VERIFIED comment; S5 references it in docs but does NOT edit these.
- `internal/provider/builtin_test.go` — the decode-parity fixtures; S5 does NOT touch.
- `multiturn.go` / `generate.go` (P1.M1.T3) — the N+1 turn protocol; S5 is unit + docs only.
- Integration/stub tests (P1.M1.T4) — S5's tests are PURE unit (Manifest literals, no spawning).
