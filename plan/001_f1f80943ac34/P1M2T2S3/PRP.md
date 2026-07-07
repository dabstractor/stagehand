---
name: "P1.M2.T2.S3 — codex + cursor built-in manifests (research discrepancy fixes)"
description: |
  Land the THIRD and FINAL subtask of Built-in Provider Manifests (P1.M2.T2): EXTEND the compiled-in
  `internal/provider/builtin.go` to add the **codex** and **cursor** manifests — the last two of the
  §12.7.1 "read-only constraint" providers (no global tool-disable switch; constrained to read-only,
  never-ask profiles: codex `--sandbox read-only --ephemeral`, cursor `--mode ask --trust`). S1 landed
  pi+claude (explicit tool-disable switch); S2 landed gemini+opencode (read-only constraint); THIS
  subtask (S3) adds the final two → `BuiltinManifests()` returns all SIX providers.

  This subtask MODIFIES the two files S1 created and S2 extended (`builtin.go` + `builtin_test.go`). It
  does NOT create new files, does NOT touch `manifest.go`/`manifest_test.go` (S1 frozen contract) or
  `merge.go`/`merge_test.go` (S2 merge), and adds NO dependency (go.mod unchanged — `builtin.go` stays
  import-free).

  ⚠️ **THE central design call — THE codex discrepancy fix (TWO revisions).** The PRD §12.7 codex TOML
  carries a DISCREPANCY flagged in external_deps.md §codex: `--ask-for-approval` is NOT a `codex exec`
  flag (it lives on the interactive `codex` command; `codex exec` is already non-interactive). The
  work-item contract RESOLVES it with TWO revisions to the §12.7 codex TOML:
    (1) `prompt_delivery` "positional" → **"stdin"** (external_deps.md §codex BONUS FINDING: `codex exec`
        reads stdin with `-`; avoids arg-length limits on ~300 KB diffs — same rationale as gemini in S2).
    (2) `bare_flags` `["--sandbox","read-only","--ask-for-approval","never"]` →
        **`["--sandbox","read-only","--ephemeral"]`** (DROP the invalid `--ask-for-approval never`;
        ADD `--ephemeral` = "Run without persisting session files", confirmed in `codex exec --help`).
  Still read-only, still never-asks, now also session-clean. The codex decode-parity fixture is §12.7
  codex with BOTH lines changed + deviation comments. cursor has NO revision (§12.7 cursor == work-item
  spec verbatim), so its fixture is the verbatim §12.7 cursor TOML. See research §2–§5.

  ⚠️ **THE second design call — `Detect`/`Command` = "agent" for cursor (≠ Name), the ONLY provider where
  detect ≠ name.** §12.7 writes `detect = "agent"` / `command = "agent"` — the standalone Cursor Agent
  binary is `agent`; the map key + Name is "cursor". So `builtinCursor()` sets Detect/Command to "agent"
  (NOT "cursor"). `NameMatchesKey` still passes (checks `.Name == key`). The CursorFields test MUST assert
  `*Detect == "agent"` (a careless copy from codex's matching values would fail). See research §6A.

  ⚠️ **THE third design call — cursor `Subcommand = []string{}` is a NON-NIL EMPTY slice, NOT nil.**
  §12.7 writes `subcommand = []`. Per go-toml-pointer-behavior FINDING D, a present-but-empty array
  decodes to a NON-NIL empty slice (`len 0`, `!= nil`). So `builtinCursor().Subcommand` MUST be
  `[]string{}` — omitting the field (→ nil) FAILS decode-parity (nil ≠ non-nil-empty). This is the SAME
  gotcha as opencode's `BareFlags = []` in S2 (research §6B). codex's `Subcommand = ["exec"]` is a
  non-nil 1-element slice (no gotcha).

  ⚠️ **THE fourth design call — both PREPEND the system prompt (no sys-prompt flag).** Neither codex nor
  cursor exposes a `--system-prompt` flag (external_deps.md §codex/§cursor VERIFIED). Both set
  `SystemPromptFlag = strPtr("")` (explicit empty, NON-NIL — matching the TOML). The §12.2 renderer's
  `if sys_flag != "" and sys` is therefore FALSE → the sys prompt is PREPENDED to the payload
  (`"<sys>\n\n<user payload>"`). This is the "no sys-prompt flag → prepend" branch.

  ⚠️ **THE fifth design call — document the TWO `# TO CONFIRM` items inline (work item: "Add a comment
  documenting the two TO CONFIRM items").** From PRD Appendix E item 4 + §12.7 inline notes: (1) codex
  `exec` writes the final answer to stdout + exits 0; (2) cursor `--mode ask` wins over `-p`'s default
  full-tools profile (genuinely read-only). Neither blocks the manifest shape — they are integration-time
  runtime confirmations for P1.M2.T5/T6 + the real-agent scaffold (P1.M5.T1.S2). Encode them as
  `// TO CONFIRM (integration): …` comments in the two constructors (honest stubbing per §12.7.2). See
  research §7.

  ⚠️ **THE sixth design call — cursor's §12.2 render ORDER differs from the §12.7 illustrative "Rendered"
  block (NOT a bug; document it).** §12.2's argv algorithm orders: command, subcommand, [provider],
  [model], [sys], bare_flags, [print_flag], [positional payload]. For cursor this yields
  `agent --model gpt-5 --mode ask --trust -p "<…>"`; §12.7's hand-written block shows
  `agent -p --mode ask --trust --model gpt-5 "<…>"`. Same tokens, different order — cursor parses flags
  in any order, so semantically identical. §12.2 is AUTHORITATIVE (the real P1.M2.T4 renderer implements
  it); the RenderedCommand_Cursor test asserts the `renderArgs` (§12.2) output with a comment on the diff.
  See research §6C.

  ⚠️ **THE seventh design call — EXTEND the existing files (S1's, S2-extended), do NOT create new ones;
  update the one S2 test that would otherwise break.** S1 created `builtin.go` + `builtin_test.go`;
  S2 extended both (4 keys). S3 adds `builtinCodex()`/`builtinCursor()` constructors, extends
  `BuiltinManifests()`'s map (4→6 keys), and EXTENDS the test file: (a) UPDATE `TestBuiltinManifests_
  KeysAndCount` (4→6 keys — else it fails), (b) UPDATE `TestBuiltinManifests_DecodeParity` table
  (+codex/+cursor rows), (c) ADD `codexTOML`/`cursorTOML` constants + 4 new tests (CodexFields,
  CursorFields, RenderedCommand_Codex, RenderedCommand_Cursor). `NameMatchesKey` + `Validate` iterate
  the whole map → auto-cover the new providers (no edit). The `renderArgs`/`assertStr`/`assertNilStr`
  helpers are REUSED unchanged (S1/S2 render tests depend on `renderArgs(m, provider, model, sys)`).

  Deliverable: MODIFIED `internal/provider/builtin.go` (`package provider`, ZERO imports) —
  `BuiltinManifests()` now returning `{"pi","claude","gemini","opencode","codex","cursor"}` + the two
  new unexported constructors; MODIFIED `internal/provider/builtin_test.go` — the 2 updated tests + 4 new
  tests, all passing. INPUT = S1's `Manifest` + `strPtr`/`boolPtr` (frozen in `manifest.go`). OUTPUT =
  the complete 6-manifest built-in set the registry (P1.M2.T3) consumes; codex/cursor argv the
  renderer/executor/parser will run.
---

## Goal

**Feature Goal**: Add the codex and cursor provider manifests to the compiled-in defaults so
`BuiltinManifests() map[string]Manifest` returns all SIX providers (pi, claude, gemini, opencode, codex,
cursor), every field matching PRD §12.7 exactly — codex with the TWO mandated revisions
(`prompt_delivery`="stdin"; `bare_flags`=`["--sandbox","read-only","--ephemeral"]`), cursor verbatim —
nil/non-nil pattern included — and each manifest `Validate()`ing clean.

**Deliverable**:
1. **MODIFY** `internal/provider/builtin.go` (`package provider`, **ZERO imports**):
   (a) `func BuiltinManifests() map[string]Manifest` now returns
       `{"pi": builtinPi(), "claude": builtinClaude(), "gemini": builtinGemini(), "opencode": builtinOpenCode(),
         "codex": builtinCodex(), "cursor": builtinCursor()}`
       (fresh construction each call — S1 design call #4, unchanged).
   (b) **ADD** unexported `func builtinCodex() Manifest` — every field per the codex table below, built
       with S1's `strPtr`/`boolPtr`; `PromptDelivery=strPtr("stdin")` (**REVISED #1**); `Subcommand=
       []string{"exec"}`; `PrintFlag`/`DefaultModel`/`SystemPromptFlag`/`ProviderFlag`=strPtr("")
       (non-nil empty); `BareFlags=["--sandbox","read-only","--ephemeral"]` (**REVISED #2**);
       `DefaultProvider`/`PromptFlag`/`JsonField`/`RetryInstruction`/`Env` nil (absent in §12.7);
       a `// TO CONFIRM (integration): …` comment on codex stdout-on-success.
   (c) **ADD** unexported `func builtinCursor() Manifest` — every field per the cursor table;
       `Detect`/`Command`=strPtr("agent") (**≠ Name "cursor"**); `Subcommand=[]string{}` (NON-NIL empty);
       `PrintFlag`="-p"; `ModelFlag`="--model"; `DefaultModel`/`SystemPromptFlag`/`ProviderFlag`=strPtr("");
       `BareFlags=["--mode","ask","--trust"]`; `DefaultProvider`/`PromptFlag`/`JsonField`/
       `RetryInstruction`/`Env` nil (absent in §12.7); a `// TO CONFIRM (integration): …` comment on
       cursor ask-wins-over-`-p`.
   (d) UPDATE the `BuiltinManifests` doc comment: now cites all six providers + §12.7; "remaining two
       (codex, cursor) … S3" line REMOVED (all six now present).
2. **MODIFY** `internal/provider/builtin_test.go` (`package provider`, white-box; imports
   `testing`+`reflect`+`go-toml/v2` unchanged) — the 2 updated tests + 4 new tests (see Implementation
   Tasks), all passing.

No other files touched. **No go.mod/go.sum change** (`builtin.go` has zero imports). NO edit to
`manifest.go`/`manifest_test.go` (S1) or `merge.go`/`merge_test.go` (S2). No registry (P1.M2.T3), no
renderer/executor/parser (P1.M2.T4/T5/T6), no `providers/*.toml` files (P1.M5.T2).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/` clean; `go mod
tidy` is a no-op; `go test -race ./internal/provider/ -v` passes (S1/S2's pi/claude/gemini/opencode tests
STILL green + the 2 updated + 4 new codex/cursor tests green) and `go test -race ./...` stays green; the
codex + cursor manifests match the tables below exactly (incl. codex's two revisions, cursor's
detect="agent", cursor's non-nil-empty Subcommand, and the explicit-empty vs absent pattern);
`reflect.DeepEqual(builtinCodex(), decode(codexTOML))` and the cursor equivalent both hold (codexTOML =
§12.7 codex with the two documented revisions; cursorTOML = verbatim §12.7 cursor); both `Validate()` →
nil.

## User Persona

**Target User**: The registry (P1.M2.T3) — it calls `BuiltinManifests()` to fetch the compiled-in defaults,
then `MergeManifest(builtin, userOverride)` (S2 merge) to overlay any `[provider.codex]`/`[provider.cursor]`
config, then `Validate()` + `Resolve()` before handing the manifest to the renderer/executor/parser.
Transitively every user story routed through "call an agent" (US) and FR36/FR37 (provider management).

**Use Case**: A user runs `stagecoach` with zero config and has `codex` (or `agent`/cursor) installed. The
registry has no `[provider.*]` override, so `BuiltinManifests()["codex"]` IS the resolved codex manifest;
the renderer turns it into `codex exec -m <model> --sandbox read-only --ephemeral` (payload piped to stdin
via `-`); the executor runs it; the parser cleans stdout. This subtask is what makes "zero config" work
for the last two read-only-constrained agents.

**User Journey**: (internal API, no end-user surface yet) `BuiltinManifests()` (THIS subtask adds the
final 2 entries) → registry selects `codex`/`cursor` (or merges a user override via S2) → `Validate()` →
`Resolve()` → renderer builds argv per §12.2 → executor runs → parser cleans.

**Pain Points Addressed**: Removes "what are the exact default flags for codex/cursor / is the
`--ask-for-approval` discrepancy resolved / does codex use stdin or positional / does the built-in match
the PRD TOML" ambiguity by landing two literal, decode-parity-tested manifests now. The codex discrepancy
is resolved ONCE, here, with a traceable rationale (drop the invalid flag; switch to stdin; add ephemeral).

## Why

- **Zero config works because of this — and this completes the six.** PRD §12.1: "Built-in manifests are
  compiled into the binary (so the tool works with zero config)." codex and cursor are the last two of
  the §12.7.1 "read-only constraint" providers (no global tool-disable switch; constrained so they cannot
  mutate the repo or block on a prompt). Landing them now completes the full default provider set, so the
  registry + renderer can be built/tested against ALL THREE provider categories (explicit switch:
  pi/claude; read-only constraint: gemini/opencode/codex/cursor).
- **The codex discrepancy is resolved ONCE, here, traceably.** PRD §12.7 carried `--ask-for-approval never`
  in codex's bare_flags, but external_deps.md §codex (live `codex exec --help`) proves that flag is NOT on
  `codex exec`. The work-item contract + external_deps.md resolve it: DROP the flag (exec is already
  non-interactive), switch to stdin (avoids arg-length limits; reads stdin with `-`), ADD `--ephemeral`
  (no session files leak). Encoding it now + pinning it in a decode-parity test with documented deviations
  means no future agent re-litigates it or passes a rejected flag.
- **Unlocks the registry + renderer for ALL six targets + stdin delivery.** P1.M2.T3 imports
  `BuiltinManifests()`; P1.M2.T4 renders one. Adding codex (stdin) + cursor (positional) gives the
  renderer both delivery modes across the full set (stdin: pi/claude/gemini/codex; positional:
  opencode/cursor), and exercises the `subcommand=["exec"]` + `print_flag=-p` combinations.
- **Proves the detect≠name + non-nil-empty-slice edge cases.** cursor is the ONLY provider where
  `Detect`/`Command` ("agent") ≠ `Name` ("cursor"), and the SECOND with a non-nil-empty slice
  (cursor.Subcommand=[]string{}, after opencode.BareFlags in S2). Landing it validates the schema handles
  both faithfully — the decode-parity oracle catches any carelessness.
- **No user-facing surface change** (PRD "DOCS: none — compiled-in defaults"). `providers show`
  (P1.M4.T1.S3) and the reference `providers/*.toml` files (P1.M5.T2) are where users SEE these later.
- **No new dependency, no new import edge.** `builtin.go` stays import-free (literal construction); the
  package's production code stays stdlib-only (S1/S2/S3 discipline); go.mod is unchanged.

## What

A compiled `internal/provider` package exporting `BuiltinManifests() map[string]Manifest` now returning
all SIX literal manifests (pi, claude, gemini, opencode from S1/S2; codex, cursor from THIS subtask), each
constructed fresh per call, decode-parity-verified against its PRD TOML (codex with the two documented
revisions; cursor verbatim), both `Validate()`ing clean. No registry, no rendering, no execution, no parsing.

### Success Criteria

- [ ] `BuiltinManifests()` returns EXACTLY 6 keys: `pi`, `claude`, `gemini`, `opencode`, `codex`, `cursor`.
      Each returned manifest's `.Name` equals its map key (incl. cursor, where Detect/Command="agent").
- [ ] `builtinCodex()` sets every field per the codex table with `strPtr`/`boolPtr`: `Name="codex"`,
      `Detect`/`Command`="codex", `Subcommand=["exec"]` (**non-nil 1-element**), `PromptDelivery="stdin"`
      (**REVISED #1**), `PrintFlag`/`DefaultModel`/`SystemPromptFlag`/`ProviderFlag`=strPtr("")
      (**non-nil empty**), `ModelFlag="-m"`, `BareFlags=["--sandbox","read-only","--ephemeral"]`
      (**REVISED #2**, 3 tokens), `Output="raw"`, `StripCodeFence=true`; AND leaves
      `PromptFlag`/`DefaultProvider`/`JsonField`/`RetryInstruction`/`Env` **nil** (absent in §12.7).
- [ ] `builtinCursor()` sets every field per the cursor table: `Name="cursor"`, `Detect`/`Command`=
      **"agent"** (**≠ Name**), `Subcommand=[]string{}` (**NON-NIL empty** — write it explicitly),
      `PromptDelivery="positional"`, `PrintFlag="-p"`, `ModelFlag="--model"`, `DefaultModel`/
      `SystemPromptFlag`/`ProviderFlag`=strPtr("") (**non-nil empty**), `BareFlags=["--mode","ask",
      "--trust"]` (3 tokens), `Output="raw"`, `StripCodeFence=true`; AND leaves `PromptFlag`/
      `DefaultProvider`/`JsonField`/`RetryInstruction`/`Env` **nil** (absent in §12.7).
- [ ] Both `builtinCodex().Validate()` and `builtinCursor().Validate()` return nil.
- [ ] `reflect.DeepEqual(builtinCodex(), decode(codexTOML))` AND `reflect.DeepEqual(builtinCursor(),
      decode(cursorTOML))` both hold. `codexTOML` = §12.7 codex verbatim EXCEPT `prompt_delivery="stdin"`
      (revision #1) and `bare_flags=["--sandbox","read-only","--ephemeral"]` (revision #2); `cursorTOML`
      = verbatim §12.7 cursor (incl. `detect="agent"`, `subcommand=[]`).
- [ ] `TestBuiltinManifests_KeysAndCount` updated to expect 6 keys; `TestBuiltinManifests_DecodeParity`
      table extended with codex + cursor rows.
- [ ] New tests pass: `TestBuiltinManifests_CodexFields`, `TestBuiltinManifests_CursorFields`,
      `TestBuiltinManifests_RenderedCommand_Codex` (== `["codex","exec","-m","gpt-5","--sandbox",
      "read-only","--ephemeral"]` — stdin, payload not in argv), `TestBuiltinManifests_RenderedCommand_
      Cursor` (== `["agent","--model","gpt-5","--mode","ask","--trust","-p","<sys>\n\n<payload>"]` —
      positional per §12.2, with a comment that §12.7's illustrative block orders tokens differently).
- [ ] S1/S2's tests STILL pass unchanged: `PiFields`, `ClaudeFields`, `GeminiFields`, `OpenCodeFields`,
      `RenderedCommand_Pi_MatchesCommitPi`, `RenderedCommand_Gemini`, `RenderedCommand_OpenCode`,
      `FreshEachCall`, `NameMatchesKey`, `Validate` (the latter two now also cover codex/cursor via the
      larger map). The `renderArgs`/`assertStr`/`assertNilStr` helpers are UNCHANGED (signature-preserving).
- [ ] The two `# TO CONFIRM` items appear as `// TO CONFIRM (integration): …` comments in the two new
      constructors (codex stdout-on-success; cursor ask-wins-over-`-p`).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, `go mod tidy` (diff-empty) clean;
      `go test -race ./internal/provider/` AND `go test -race ./...` green.
- [ ] go.mod/go.sum byte-unchanged; `manifest.go`/`manifest_test.go` (S1) + `merge.go`/`merge_test.go`
      (S2) byte-unchanged; every file outside the two modified `builtin*.go` files byte-unchanged;
      `builtin.go` STILL has ZERO imports.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the two field tables
(codex with its two revisions; cursor with detect="agent" and non-nil-empty Subcommand), the `strPtr`/
`boolPtr` construction idiom (from S1's `manifest.go`), the decode-parity test approach + the verbatim TOML
strings (provided below), the §12.2 render algorithm (S1's `renderArgs` helper, reused) + the exact
expected argvs, and the test specs. The subtleties (codex's two revisions; cursor's detect≠name; cursor's
non-nil-empty Subcommand; cursor's §12.2-vs-§12.7 render order; the two TO CONFIRM comments) are documented
inline in both the literals and the tests.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- file: internal/provider/builtin.go   (S1 created; S2 EXTENDED to 4 keys; you MODIFY it to 6 keys)
  why: the file you are EXTENDING. Read S1's builtinPi()/builtinClaude() AND (once S2 lands)
       builtinGemini()/builtinOpenCode() to mirror their construction style (doc comment citing the PRD
       section + the explicit-empty notes; strPtr/boolPtr field assignment; absent fields omitted with a
       trailing comment). You ADD builtinCodex()/builtinCursor() and extend the BuiltinManifests() map
       (one line per new entry) + update its doc comment.
  pattern: copy S1/S2's constructor structure EXACTLY. The new constructors are FREE FUNCTIONS (no
       receiver): `func builtinCodex() Manifest` / `func builtinCursor() Manifest`.
  critical: do NOT add imports (the file stays import-free). do NOT call Validate/Resolve inside the
       constructors (they BUILD; the registry validates). do NOT add a package-level var (fresh per call).
       do NOT set cursor.Detect/Command to "cursor" — §12.7 says "agent".

- file: internal/provider/manifest.go   (S1 — COMPLETE; read it, do NOT edit it)
  why: the EXACT Manifest type + field names/tags the constructors build, AND the unexported helpers
       `strPtr(string) *string` / `boolPtr(bool) *bool` (same package — use them directly, no import).
       Also `Validate()` (both new built-ins must pass it) + `Resolve()` + `DetectCommand()` + the
       Default* constants. Confirm field names match exactly: Name, Detect, Command, Subcommand ([]string
       — PLAIN slice), PromptDelivery, PromptFlag, PrintFlag, ModelFlag, DefaultModel, SystemPromptFlag,
       ProviderFlag, DefaultProvider, BareFlags ([]string — PLAIN slice), Output, JsonField,
       StripCodeFence, RetryInstruction, Env.
  critical: Subcommand and BareFlags are PLAIN []string (not pointers) — a non-nil empty slice is a
       distinct value from nil (this is why cursor.Subcommand must be `[]string{}`, see research §6B).
       do NOT edit.

- file: internal/provider/builtin_test.go   (S1 created; S2 EXTENDED; you MODIFY it)
  why: the file you are EXTENDING. Reuse S1/S2's `assertStr`/`assertNilStr`/`renderArgs` helpers (do NOT
       change their signatures — S1/S2's render tests depend on `renderArgs(m, provider, model, sys)`).
       Mirror the existing test style (table-driven DecodeParity; per-provider Fields tests). You ADD the
       codexTOML/cursorTOML constants + 4 new tests, and UPDATE KeysAndCount (4→6) + DecodeParity (+2 rows).
  critical: codexTOML is §12.7 codex with BOTH revisions (prompt_delivery="stdin"; bare_flags revised) —
       see research §2–§4. If you paste the verbatim §12.7 codex TOML, the codex decode-parity test WILL
       FAIL. cursorTOML is verbatim §12.7 cursor (detect="agent", subcommand=[]).

- docfile: plan/001_f1f80943ac34/P1M2T2S3/research/codex-cursor-manifests.md
  why: the field-by-field value tables (§2 codex, §5 cursor), the codex discrepancy + its two revisions
       (§3 stdin, §4 bare_flags), the explicit-empty vs absent map, the THREE cursor gotchas (§6: detect≠
       name; subcommand=[] non-nil-empty; §12.2-vs-§12.7 render order), the two TO CONFIRM items (§7),
       the test strategy (§8). The single most important read.
  critical: §3–§4 (codex's two revisions) and §6A/§6B (cursor detect≠name + non-nil-empty Subcommand) are
       the things most likely to be implemented wrong.

- file: PRD.md
  section: "12.7 Verified providers: Codex, Cursor Agent" (h3.43) — the AUTHORITATIVE codex + cursor
       manifest TOMLs. The codex TOML block is the decode-parity base, BUT two lines are REVISED in the
       fixture (prompt_delivery "positional"→"stdin"; bare_flags drops --ask-for-approval, adds --ephemeral).
       The cursor TOML block IS the verbatim decode-parity fixture (no revision).
  why: every codex/cursor field value comes from here. codex: note the 4 explicit-empty scalars
       (print_flag/default_model/system_prompt_flag/provider_flag=""), subcommand=["exec"], and the
       bare_flags line you will REVISE. cursor: note detect="agent"/command="agent" (≠name), subcommand=[]
       (→ non-nil empty), the 3 explicit-empty scalars (default_model/system_prompt_flag/provider_flag=""),
       print_flag="-p", bare_flags=["--mode","ask","--trust"].
  critical: §12.7's "Rendered" prose blocks are ILLUSTRATIVE — for cursor the token order differs from the
       §12.2 algorithm; the render test asserts the §12.2 (renderArgs) order, with a comment. Do NOT try to
       hand-match the §12.7 cursor prose ordering.

- file: PRD.md
  section: "12.2 Command rendering algorithm" (h3.38) — the AUTHORITATIVE argv algorithm. S1 already ported
       it into the `renderArgs` test helper (REUSED unchanged here). The §12.2 positional branch
       (`args += [payload]` when delivery=="positional") applies to cursor; the stdin branch (payload
       piped, not in argv) applies to codex.
  critical: for stdin delivery the payload is NOT in argv (piped via `-` for codex); sys is prepended to
       the payload when sys_flag=="". For positional delivery the payload IS the trailing positional arg.

- file: PRD.md
  section: "12.7.1 The tools-disable asymmetry" (h4.0) — the conceptual framing: codex+cursor are
       "read-only constraint" providers (no global disable switch; constrained so they cannot mutate the
       repo or block on a prompt, but the model may still internally reason with tools). This is WHY codex
       uses --sandbox read-only + --ephemeral and cursor uses --mode ask + --trust.
  section: "12.7.2 On stubbing and progressive verification" (h4.1) — the CONTRACT for the TO CONFIRM
       notes: "the manifest schema and the six default manifests are fixed by this document; the exact
       behavior of each manifest is confirmed by a real end-to-end run during implementation … Any
       manifest field that cannot be confirmed is left at a safe default and marked with a # TO CONFIRM
       comment, never silently assumed." THIS subtask carries the two §12.7 TO CONFIRM comments.

- file: plan/001_f1f80943ac34/architecture/external_deps.md
  section: §codex (⚠️ DISCREPANCY) + §cursor (✅ VERIFIED) — live `--help` captures (2026-06-29).
  why: independent confirmation + the discrepancy resolution. §codex: `codex exec --help` shows
       [PROMPT]/-m/-s/--ephemeral/-o/--json; `--ask-for-approval` is ONLY on interactive `codex` (NOT
       `codex exec`); BONUS FINDING "codex exec reads stdin with `-`" → stdin viable; recommended-revision
       manifest (drop the flag, add --ephemeral). §cursor: `agent --help` shows -p/--print, --mode
       (plan|ask), --trust, --model; `--mode ask` = "Q&A style, read-only (no edits)"; TO CONFIRM that
       ask wins over -p's default full-tools.
  critical: §codex is the PRIMARY source for BOTH codex revisions (work item is consistent with it).

- file: PRD.md
  section: "Appendix E — Open questions" (h2.28) — item 4 (codex/cursor "mostly resolved") names the two
       residual TO CONFIRM items this subtask carries inline: (a) codex exec writes final answer to stdout
       + exits 0; (b) cursor --mode ask wins over -p's default full-tools. Both expected; quick to confirm
       during the first real run (P1.M5.T1.S2 real-agent scaffold).
  section: "Appendix D — Built-in manifest quick reference" (h2.27) — the cross-provider table. codex row
       (command `codex exec`, delivery stdin [revised], model flag -m, sys-prompt *(prepend)*, bare
       `--sandbox read-only --ephemeral` [revised], output raw); cursor row (command `agent`, delivery
       positional, model flag --model, sys-prompt *(prepend)*, bare `--mode ask --trust`, output raw).

- file: plan/001_f1f80943ac34/P1M2T1S1/research/go-toml-pointer-behavior.md
  why: FINDING C/D — absent key → nil; present key (even `""`/`[]`/`false`) → non-nil. This is WHY the
       literal must reproduce the TOML's nil/non-nil pattern exactly and why cursor's `subcommand = []` →
       NON-NIL empty slice (research §6B).
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; go-toml/v2 v2.4.2 + pflag  (UNCHANGED by this subtask)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 — FROZEN, do NOT touch; do NOT import from provider
  git/                          # P1.M1.T2/T3 — untouched
  provider/                     # S1 created the package; S2(merge) added merge.go; S1(builtin)+S2 extended builtin.go
    manifest.go                 # S1 — Manifest + Validate + DetectCommand + Resolve + strPtr/boolPtr  (CONTRACT — do NOT edit)
    manifest_test.go            # S1 — tests  (do NOT edit)
    merge.go                    # S2(merge) — MergeManifest  (do NOT edit)
    merge_test.go               # S2(merge) — tests  (do NOT edit)
    builtin.go                  # S1(builtin) created this; S2 extended to pi+claude+gemini+opencode (4). THIS subtask MODIFIES it (+codex +cursor → 6)
    builtin_test.go             # S1(builtin) created; S2 extended to 12 tests. THIS subtask MODIFIES it (+4 tests, update 2)
cmd/stagecoach/main.go           # `package main; func main(){}` stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be modified

```bash
internal/
  provider/
    builtin.go                  # MODIFIED — BuiltinManifests() now returns 6 keys + builtinCodex() + builtinCursor() (still ZERO imports)
    builtin_test.go             # MODIFIED — codexTOML/cursorTOML + 4 new tests + KeysAndCount(4→6) + DecodeParity(+2)
# manifest.go/manifest_test.go (S1) + merge.go/merge_test.go (S2 merge) UNCHANGED. go.mod/go.sum UNCHANGED.
# Every other file UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (design call #1 — THE codex discrepancy fix, TWO revisions): codex deviates from the verbatim
// §12.7 TOML in exactly TWO places, both mandated by the work-item contract + external_deps.md §codex:
//   (1) PromptDelivery MUST be strPtr("stdin"), NOT strPtr("positional"). codex exec reads stdin via "-"
//       (external_deps.md §codex BONUS FINDING); stdin avoids arg-length limits on ~300 KB diffs (same
//       rationale as gemini in S2). If you paste verbatim §12.7, the codex DecodeParity test FAILS.
//   (2) BareFlags MUST be ["--sandbox","read-only","--ephemeral"], NOT the §12.7
//       ["--sandbox","read-only","--ask-for-approval","never"]. --ask-for-approval is NOT a codex exec
//       flag (external_deps.md §codex DISCREPANCY); codex exec is already non-interactive. Drop it; ADD
//       --ephemeral (confirmed in codex exec --help; keeps the one-shot session-clean).
// The codex decode-parity fixture (codexTOML) = §12.7 codex with BOTH lines changed + deviation comments.

// CRITICAL (design call #2 — cursor Detect/Command = "agent", ≠ Name): cursor is the ONLY provider where
// Detect/Command differ from Name. §12.7 writes detect="agent"/command="agent"; the standalone Cursor
// Agent binary is `agent`; the map key + Name is "cursor". builtinCursor() MUST set Detect/Command to
// strPtr("agent"). NameMatchesKey still passes (checks .Name=="cursor"=="cursor" key). The CursorFields
// test asserts *Detect=="agent" — a careless copy from codex (where all match) would FAIL.

// CRITICAL (design call #3 — cursor Subcommand is a NON-NIL EMPTY slice): cursor.Subcommand MUST be
// []string{}, NOT nil. §12.7 writes subcommand = []; FINDING D says a present empty array decodes to a
// NON-NIL empty slice. Omitting the field (→ nil) FAILS decode-parity (nil ≠ non-nil-empty). This is the
// SAME gotcha as opencode.BareFlags=[] in S2. Write it explicitly: Subcommand: []string{}.
//   codex.Subcommand = []string{"exec"}   // §12.7 writes subcommand = ["exec"] → NON-NIL 1-element

// CRITICAL (design call #4 — both PREPEND the system prompt): both set SystemPromptFlag = strPtr("")
// (explicit empty, NON-NIL — matching the TOML). The §12.2 renderer's `if sys_flag != "" and sys` is
// therefore FALSE → the sys prompt is NOT a flag; per §12.2 it is PREPENDED to the payload. Neither CLI
// exposes a --system-prompt flag (external_deps.md VERIFIED). Do NOT invent a flag value.

// CRITICAL (design call #5 — document the TWO TO CONFIRM items inline): per the work item ("Add a comment
// documenting the two TO CONFIRM items") + PRD §12.7.2 + Appendix E #4, carry two comments:
//   codex:  // TO CONFIRM (integration): that `codex exec` writes the assistant's final answer to stdout
//           // and exits 0 on success (expected; -o <file>/--json are fallbacks). Verify P1.M5.T1.S2.
//   cursor: // TO CONFIRM (integration): that `--mode ask` wins over `-p`'s default full-tools profile
//           // (i.e. the combo is genuinely read-only). Expected (ask = read-only Q&A). Verify P1.M5.T1.S2.
// Neither blocks the manifest shape; both are runtime confirmations for executor/parser + real-agent tests.

// CRITICAL (design call #6 — cursor's §12.2 render ORDER ≠ §12.7 illustrative block; NOT a bug): the
// §12.2 algorithm (renderArgs) orders tokens command,subcommand,[provider],[model],[sys],bare,[print],
// [payload] → cursor renders `agent --model gpt-5 --mode ask --trust -p "<…>"`. §12.7's hand-written block
// shows `agent -p --mode ask --trust --model gpt-5 "<…>"`. Same tokens, different order; cursor parses
// flags in any order → identical semantics. §12.2 is authoritative (the real renderer implements it).
// The RenderedCommand_Cursor test asserts the renderArgs output + a comment on the diff. Do NOT hand-match
// the §12.7 prose ordering.

// CRITICAL (design call #7 — EXTEND, don't recreate; preserve S1/S2 helper signatures): this subtask
// MODIFIES builtin.go + builtin_test.go (S1 created, S2 extended to 4). The renderArgs helper signature
// is `renderArgs(m Manifest, provider, model, sys string) []string` — S1/S2 render tests depend on it;
// do NOT add a payload param. For the cursor POSITIONAL render test, append the payload manually:
//   flags := renderArgs(builtinCursor(), "", "gpt-5", "")
//   argv := append(flags, "<sys>\n\n<payload>")
// For the codex STDIN render test, do NOT append payload (piped to stdin):
//   argv := renderArgs(builtinCodex(), "", "gpt-5", "<sys>")   // model explicit (default is "")
// KeysAndCount MUST be updated 4→6 (else it fails once the map has 6 entries). DecodeParity table +2 rows.

// GOTCHA: do NOT call Validate/Resolve inside the constructors. The constructors BUILD; the registry
// (P1.M2.T3) runs Validate → Resolve on the (merged) result. Tests assert both new built-ins Validate(),
// but the constructors stay pure data.

// GOTCHA: the DecodeParity test uses reflect.DeepEqual on Manifest. nil pointers compare equal ONLY to
// nil; nil slices compare equal ONLY to nil; a non-nil empty slice compares UNEQUAL to nil. So the test is
// exactly the right oracle for "built-in matches the decoded TOML" — it catches any nil/non-nil mismatch
// (the codex stdin value + revised bare_flags, the cursor non-nil-empty Subcommand, the explicit-empty
// scalars, cursor's detect="agent").

// GOTCHA: codex/cursor DefaultModel = strPtr("") (NON-NIL empty). This is INTENTIONAL ("user must set
// model" — codex reads ~/.codex/config.toml; cursor has per-account model availability). In renderArgs,
// if model is "" AND default is "", modelToUse stays "" → no model flag emitted (proving the design). The
// render tests pass an EXPLICIT model ("gpt-5") so the flag appears and the argv matches the tables.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/provider/builtin.go — ADD these two constructors; EXTEND BuiltinManifests()'s map.
package provider
// (NO imports — literal construction via same-package strPtr/boolPtr only. UNCHANGED from S1/S2.)

// ... S1's builtinPi() / builtinClaude() and S2's builtinGemini() / builtinOpenCode() UNCHANGED above ...

// builtinCodex returns the codex manifest per PRD §12.7 (VERIFIED vs `codex exec --help`, external_deps.md
// §codex), with TWO revisions that resolve the §codex discrepancy flagged in external_deps.md:
//   (1) PromptDelivery="stdin" (§12.7 said "positional") — codex exec reads stdin via "-" (external_deps.md
//       §codex BONUS FINDING); stdin avoids arg-length limits on ~300 KB diffs.
//   (2) BareFlags=["--sandbox","read-only","--ephemeral"] (§12.7 said
//       ["--sandbox","read-only","--ask-for-approval","never"]) — --ask-for-approval is NOT a codex exec
//       flag (it lives on interactive `codex`; codex exec is already non-interactive); --ephemeral keeps
//       the one-shot session-clean.
// codex has no global tool-disable switch; --sandbox read-only constrains it to a read-only, never-ask
// profile (§12.7.1 "read-only constraint").
//
// NOTE: (1) Subcommand=["exec"] (§12.7 subcommand = ["exec"] → NON-NIL 1-element). (2) PrintFlag/
// DefaultModel/SystemPromptFlag/ProviderFlag are strPtr("") — §12.7 WRITES them "" (NON-NIL empty): exec
// is already non-interactive (no print flag), model comes from ~/.codex/config.toml (no default), no
// sys-prompt flag (sys PREPENDED to the payload per §12.2), no sub-provider. (3) DefaultProvider is NIL
// — §12.7 OMITS the key. (4) The sys prompt is prepended (no --system-prompt flag on codex exec).
//
// TO CONFIRM (integration): that `codex exec` writes the assistant's final answer to stdout and exits 0
// on success. Expected; -o <file> (write last message to file) and --json (JSONL events) are fallback
// output channels if stdout proves unreliable. Verify during the real-agent scaffold (P1.M5.T1.S2).
func builtinCodex() Manifest {
	return Manifest{
		Name:             "codex",
		Detect:           strPtr("codex"),
		Command:          strPtr("codex"),
		Subcommand:       []string{"exec"}, // §12.7 `subcommand = ["exec"]` → NON-NIL 1-element slice
		PromptDelivery:   strPtr("stdin"),   // REVISED #1 from §12.7 "positional" (codex exec reads stdin via "-")
		PrintFlag:        strPtr(""),        // §12.7 explicit empty (NON-NIL) — exec is already non-interactive
		ModelFlag:        strPtr("-m"),
		DefaultModel:     strPtr(""), // §12.7 explicit empty (NON-NIL) — model from ~/.codex/config.toml
		SystemPromptFlag: strPtr(""), // §12.7 explicit empty (NON-NIL) — no sys flag on exec; sys prepended (§12.2)
		ProviderFlag:     strPtr(""), // §12.7 explicit empty (NON-NIL) — codex has no sub-provider
		BareFlags: []string{
			"--sandbox", "read-only", // read-only, never-mutate profile
			"--ephemeral", // REVISED #2: run without persisting session files (replaces invalid --ask-for-approval)
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// PromptFlag, DefaultProvider, JsonField, RetryInstruction, Env: nil (absent in §12.7).
	}
}

// builtinCursor returns the cursor manifest per PRD §12.7 (VERIFIED vs `agent --help`, external_deps.md
// §cursor), VERBATIM (no revisions). The standalone Cursor Agent binary is `agent` (so Detect/Command =
// "agent", NOT "cursor" — cursor is the ONLY provider where detect ≠ name). cursor's -p print mode
// defaults to FULL tool access; we override with --mode ask ("Q&A style, read-only, no edits") + --trust
// (skip the workspace-trust prompt) so it cannot mutate the repo (§12.7.1 "read-only constraint").
//
// NOTE: (1) Detect/Command = "agent" (≠ Name "cursor") — §12.7 writes detect/command = "agent". (2)
// Subcommand = []string{} — §12.7 writes subcommand = []; a present empty array decodes NON-NIL empty
// (FINDING D); write it explicitly (do NOT omit → nil). (3) DefaultModel/SystemPromptFlag/ProviderFlag
// are strPtr("") — §12.7 WRITES them "" (NON-NIL empty): cursor has per-account model availability (no
// single default), no sys-prompt flag (sys prepended), no sub-provider. (4) DefaultProvider is NIL —
// §12.7 OMITS the key. (5) The sys prompt is prepended (no --system-prompt flag on agent).
//
// TO CONFIRM (integration): that `--mode ask` wins over `-p`'s default full-tools profile — i.e. the
// combo (-p --mode ask --trust) is genuinely read-only. Expected (ask is defined as read-only Q&A);
// verify against a real run during the real-agent scaffold (P1.M5.T1.S2).
func builtinCursor() Manifest {
	return Manifest{
		Name:             "cursor",
		Detect:           strPtr("agent"), // §12.7 detect = "agent" — the binary is `agent` (≠ Name "cursor")
		Command:          strPtr("agent"), // §12.7 command = "agent"
		Subcommand:       []string{},      // §12.7 `subcommand = []` → NON-NIL empty slice (FINDING D); do NOT omit
		PromptDelivery:   strPtr("positional"),
		PrintFlag:        strPtr("-p"), // §12.7 `-p` = non-interactive (writes answer to stdout)
		ModelFlag:        strPtr("--model"),
		DefaultModel:     strPtr(""), // §12.7 explicit empty (NON-NIL) — user must set (per-account availability)
		SystemPromptFlag: strPtr(""), // §12.7 explicit empty (NON-NIL) — no sys flag on agent; sys prepended (§12.2)
		ProviderFlag:     strPtr(""), // §12.7 explicit empty (NON-NIL) — cursor has no sub-provider
		BareFlags: []string{
			"--mode", "ask", // "Q&A style, read-only" — overrides -p's default full-tools profile
			"--trust", // skip the workspace-trust prompt (else -p would block)
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// PromptFlag, DefaultProvider, JsonField, RetryInstruction, Env: nil (absent in §12.7).
	}
}
```

**And update `BuiltinManifests()`** (extend the map + doc comment):

```go
// BuiltinManifests returns the compiled-in default provider manifests (PRD §12.3 pi, §12.4 claude,
// §12.5 gemini, §12.6 opencode, §12.7 codex + cursor), keyed by manifest name. These are the zero-config
// defaults a user override (config [provider.<name>]) merges onto via MergeManifest (S2) in the registry
// (P1.M2.T3).
//
// The manifests are constructed FRESH on every call (no package-level var): strPtr/boolPtr allocate new
// pointers and slice literals allocate new backing arrays each call, so no caller can corrupt a built-in
// by mutating a returned BareFlags/Env. MergeManifest (S2) already never mutates base, so the normal
// registry path is safe either way — fresh-per-call additionally guards against any direct mutation.
//
// The full §12.7 set: pi + claude (the "explicit tool-disable switch" pair, S1), gemini + opencode
// (read-only constraint, S2), and codex + cursor (read-only constraint, S3 — codex's two revisions
// resolve the external_deps.md §codex discrepancy). All six providers are now present.
func BuiltinManifests() map[string]Manifest {
	return map[string]Manifest{
		"pi":       builtinPi(),
		"claude":   builtinClaude(),
		"gemini":   builtinGemini(),
		"opencode": builtinOpenCode(),
		"codex":    builtinCodex(),
		"cursor":   builtinCursor(),
	}
}
```

> **gofmt note:** run `gofmt -w internal/provider/builtin.go internal/provider/builtin_test.go`. Do not
> hand-align (gofmt will align the map keys). One doc comment per function (citing the PRD section + the
> codex revisions + the explicit-empty notes + the TO CONFIRM items) is required — it seeds `providers
> show` / reference-file docs later.
>
> **Imports:** `builtin.go` has NONE (unchanged from S1/S2). `builtin_test.go` imports are UNCHANGED
> (`testing` + `reflect` + `github.com/pelletier/go-toml/v2` — already there from S1; S3 adds nothing).

### The decode-parity TOML fixtures (ADD to builtin_test.go)

```go
// codexTOML — PRD §12.7 codex VERBATIM EXCEPT two lines revised per the work-item contract +
// external_deps.md §codex (the discrepancy resolution). This is the ONLY intentional deviation set from
// the verbatim PRD codex TOML; decoding it must match builtinCodex().
//   (1) prompt_delivery = "stdin"      (§12.7 said "positional"; codex exec reads stdin via "-")
//   (2) bare_flags = ["--sandbox","read-only","--ephemeral"]  (§12.7 had "--ask-for-approval","never" —
//       NOT a codex exec flag; dropped; --ephemeral added).
const codexTOML = `name = "codex"
detect = "codex"
command = "codex"
subcommand = ["exec"]
prompt_delivery = "stdin"   # REVISED #1 from §12.7 "positional" (codex exec reads stdin via "-")
print_flag = ""
model_flag = "-m"
default_model = ""
system_prompt_flag = ""
provider_flag = ""
bare_flags = ["--sandbox", "read-only", "--ephemeral"]   # REVISED #2: dropped --ask-for-approval; added --ephemeral
output = "raw"
strip_code_fence = true
`

// cursorTOML — PRD §12.7 cursor VERBATIM (no revision). Decoding it must match builtinCursor().
// Note: detect/command = "agent" (≠ name "cursor"); subcommand = [] decodes to a NON-NIL empty slice
// (FINDING D) — builtinCursor sets Subcommand: []string{}.
const cursorTOML = `name = "cursor"
detect = "agent"
command = "agent"
subcommand = []
prompt_delivery = "positional"
print_flag = "-p"
model_flag = "--model"
default_model = ""
system_prompt_flag = ""
provider_flag = ""
bare_flags = ["--mode", "ask", "--trust"]
output = "raw"
strip_code_fence = true
`
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/provider/builtin.go — add builtinCodex + builtinCursor + extend the map
  - ADD the two constructors per the Data Models block. Use S1's strPtr/boolPtr (same package).
  - codex: per the codex table; Subcommand=[]string{"exec"}; PromptDelivery=strPtr("stdin") (REVISED #1);
      PrintFlag/DefaultModel/SystemPromptFlag/ProviderFlag=strPtr("") (non-nil empty);
      BareFlags=["--sandbox","read-only","--ephemeral"] (REVISED #2 — dropped --ask-for-approval, added
      --ephemeral); DefaultProvider/PromptFlag/JsonField/RetryInstruction/Env nil (omit). Include the
      `// TO CONFIRM (integration): …` comment on codex stdout-on-success.
  - cursor: per the cursor table; Detect/Command=strPtr("agent") (≠ Name "cursor"!);
      Subcommand=[]string{} (NON-NIL empty — write it explicitly); PromptDelivery=strPtr("positional");
      PrintFlag=strPtr("-p"); ModelFlag=strPtr("--model"); DefaultModel/SystemPromptFlag/ProviderFlag=
      strPtr("") (non-nil empty); BareFlags=["--mode","ask","--trust"]; DefaultProvider/PromptFlag/
      JsonField/RetryInstruction/Env nil (omit). Include the `// TO CONFIRM (integration): …` comment on
      cursor ask-wins-over-`-p`.
  - EXTEND BuiltinManifests() to return {"pi","claude","gemini","opencode","codex","cursor"} (fresh per
      call). UPDATE its doc comment (all six providers; REMOVE the "remaining two … S3" line).
  - IMPORTS: NONE (verify with grep — builtin.go must have zero import lines, same as S1/S2).
  - GOTCHA: do NOT add Validate/Resolve calls; do NOT add a package-level var; do NOT set absent fields;
      do NOT set cursor.Detect/Command to "cursor"; do NOT change S1/S2's constructors.

Task 2: MODIFY internal/provider/builtin_test.go — add fixtures + 4 tests, update 2 tests
  - ADD the codexTOML / cursorTOML constants (above). codexTOML has BOTH revisions; cursorTOML is verbatim
      §12.7 (detect="agent", subcommand=[]).
  - UPDATE TestBuiltinManifests_KeysAndCount: assert len==6 and all of pi/claude/gemini/opencode/codex/
      cursor present.
  - UPDATE TestBuiltinManifests_DecodeParity table: add {"codex", builtinCodex(), codexTOML} and
      {"cursor", builtinCursor(), cursorTOML} rows. (reflect.DeepEqual catches nil/non-nil + the codex
      revisions + the cursor detect/empty-slice.)
  - ADD TestBuiltinManifests_CodexFields: assert EVERY codex field (Detect/Command non-nil "codex";
      Subcommand reflect.DeepEqual ["exec"] NON-NIL; PromptDelivery=="stdin" [REVISED #1]; PrintFlag/
      DefaultModel/SystemPromptFlag/ProviderFlag NON-NIL *==""; ModelFlag "-m"; BareFlags reflect.DeepEqual
      ["--sandbox","read-only","--ephemeral"] [REVISED #2]; Output "raw"; StripCodeFence non-nil true) AND
      absent fields nil (PromptFlag/DefaultProvider/JsonField/RetryInstruction/Env). Reuse S1/S2's
      assertStr/assertNilStr helpers.
  - ADD TestBuiltinManifests_CursorFields: assert EVERY cursor field (Detect/Command NON-NIL *=="agent"
      [≠ Name "cursor" — assert the value explicitly]; Subcommand != nil && len==0 [NON-NIL EMPTY — assert
      `m.Subcommand != nil` explicitly, then len 0]; PromptDelivery "positional"; PrintFlag "-p";
      ModelFlag "--model"; DefaultModel/SystemPromptFlag/ProviderFlag NON-NIL *==""; BareFlags
      reflect.DeepEqual ["--mode","ask","--trust"]; Output "raw"; StripCodeFence true) AND absent fields
      nil (PromptFlag/DefaultProvider/JsonField/RetryInstruction/Env).
  - ADD TestBuiltinManifests_RenderedCommand_Codex: argv := renderArgs(builtinCodex(), "", "gpt-5",
      "<sys>") (model explicit — default is ""); assert argv == ["codex","exec","-m","gpt-5","--sandbox",
      "read-only","--ephemeral"] (stdin: payload NOT in argv, piped via "-"; sys prepended to stdin payload;
      no print/sys/provider flag).
  - ADD TestBuiltinManifests_RenderedCommand_Cursor: flags := renderArgs(builtinCursor(), "", "gpt-5", "")
      (model explicit — default is ""); argv := append(flags, "<sys>\n\n<payload>") (positional: payload
      appended per §12.2); assert argv == ["agent","--model","gpt-5","--mode","ask","--trust","-p",
      "<sys>\n\n<payload>"]. Add a comment: "§12.2 algorithm order; §12.7's illustrative 'Rendered' block
      orders tokens differently (-p first) — semantically identical (cursor parses flags in any order)."
  - DO NOT change renderArgs/assertStr/assertNilStr signatures (S1/S2 render tests depend on them).
  - NameMatchesKey + Validate auto-cover codex/cursor (they iterate the whole map) — no edit needed.

Task 3: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. S1's manifest.go/
      manifest_test.go AND S2(merge)'s merge.go/merge_test.go MUST be byte-unchanged. S1/S2's pi/claude/
      gemini/opencode tests MUST stay green (no field/type/import/signature change). The config + git
      suites MUST stay green.
```

### Implementation Patterns & Key Details

```go
// The codex Fields test — pins the two revisions + the explicit-empty/absent pattern (mirrors S1/S2's
// PiFields/ClaudeFields/GeminiFields/OpenCodeFields style using the existing assertStr/assertNilStr helpers).
func TestBuiltinManifests_CodexFields(t *testing.T) {
	m := builtinCodex()
	assertStr(t, "Detect", m.Detect, "codex")
	assertStr(t, "Command", m.Command, "codex")
	wantSub := []string{"exec"}
	if !reflect.DeepEqual(m.Subcommand, wantSub) {
		t.Errorf("Subcommand = %v, want %v", m.Subcommand, wantSub)
	}
	assertStr(t, "PromptDelivery", m.PromptDelivery, "stdin") // REVISED #1 from §12.7 "positional"
	assertStr(t, "PrintFlag", m.PrintFlag, "")                // NON-NIL explicit empty
	assertStr(t, "ModelFlag", m.ModelFlag, "-m")
	assertStr(t, "DefaultModel", m.DefaultModel, "") // NON-NIL explicit empty (model from ~/.codex/config.toml)
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "") // NON-NIL explicit empty (prepend)
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")         // NON-NIL explicit empty
	assertNilStr(t, "DefaultProvider", m.DefaultProvider)    // ABSENT in §12.7 → nil
	wantBare := []string{"--sandbox", "read-only", "--ephemeral"} // REVISED #2
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags = %v, want %v", m.BareFlags, wantBare)
	}
	assertStr(t, "Output", m.Output, "raw")
	if m.StripCodeFence == nil || *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want non-nil true", m.StripCodeFence)
	}
	// Absent → nil
	assertNilStr(t, "PromptFlag", m.PromptFlag)
	assertNilStr(t, "JsonField", m.JsonField)
	assertNilStr(t, "RetryInstruction", m.RetryInstruction)
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// The cursor Fields test — pins Detect/Command=="agent" (≠ Name), the NON-NIL-EMPTY Subcommand, and the
// 3 explicit-empty scalars. Note the Detect/Command assertion: "agent", NOT "cursor".
func TestBuiltinManifests_CursorFields(t *testing.T) {
	m := builtinCursor()
	assertStr(t, "Name", &m.Name, "cursor") // (Name is plain string; assert via a temp pointer or direct compare)
	assertStr(t, "Detect", m.Detect, "agent") // §12.7 detect = "agent" — the binary is `agent` (≠ Name "cursor")
	assertStr(t, "Command", m.Command, "agent")
	// Subcommand: §12.7 writes subcommand = [] → NON-NIL empty slice (FINDING D). Assert BOTH non-nil AND len 0.
	if m.Subcommand == nil {
		t.Fatal("Subcommand = nil, want NON-NIL empty []string{} (§12.7 subcommand = [] per FINDING D)")
	}
	if len(m.Subcommand) != 0 {
		t.Errorf("Subcommand = %v, want empty", m.Subcommand)
	}
	assertStr(t, "PromptDelivery", m.PromptDelivery, "positional")
	assertStr(t, "PrintFlag", m.PrintFlag, "-p")
	assertStr(t, "ModelFlag", m.ModelFlag, "--model")
	assertStr(t, "DefaultModel", m.DefaultModel, "")        // NON-NIL explicit empty (user must set)
	assertStr(t, "SystemPromptFlag", m.SystemPromptFlag, "") // NON-NIL explicit empty (prepend)
	assertStr(t, "ProviderFlag", m.ProviderFlag, "")         // NON-NIL explicit empty
	assertNilStr(t, "DefaultProvider", m.DefaultProvider)    // ABSENT → nil
	wantBare := []string{"--mode", "ask", "--trust"}
	if !reflect.DeepEqual(m.BareFlags, wantBare) {
		t.Errorf("BareFlags = %v, want %v", m.BareFlags, wantBare)
	}
	assertStr(t, "Output", m.Output, "raw")
	if m.StripCodeFence == nil || *m.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want non-nil true", m.StripCodeFence)
	}
	// Absent → nil
	assertNilStr(t, "PromptFlag", m.PromptFlag)
	assertNilStr(t, "JsonField", m.JsonField)
	assertNilStr(t, "RetryInstruction", m.RetryInstruction)
	if m.Env != nil {
		t.Errorf("Env = %v, want nil", m.Env)
	}
}

// The codex render test — stdin delivery: argv has NO payload (piped via "-"); sys prepended to stdin payload.
func TestBuiltinManifests_RenderedCommand_Codex(t *testing.T) {
	argv := renderArgs(builtinCodex(), "", "gpt-5", "<sys>") // model explicit (default is "")
	want := []string{
		"codex", "exec", // command + subcommand
		"-m", "gpt-5", // model_flag + user-set model
		"--sandbox", "read-only", "--ephemeral", // REVISED bare_flags (read-only + session-clean)
		// stdin delivery: "<sys>\n\n<user payload>" piped to stdin via "-" (NOT in argv). No print/sys/provider flag.
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("codex rendered argv:\n got %v\nwant %v", argv, want)
	}
}

// The cursor render test — positional delivery: payload IS the trailing positional arg (§12.2).
// NOTE: this is the §12.2 ALGORITHM order (renderArgs). §12.7's illustrative "Rendered" block shows
// `agent -p --mode ask --trust --model gpt-5 "<…>"` (different token order). Same tokens; cursor parses
// flags in any order → identical semantics. §12.2 is authoritative (the real P1.M2.T4 renderer).
func TestBuiltinManifests_RenderedCommand_Cursor(t *testing.T) {
	flags := renderArgs(builtinCursor(), "", "gpt-5", "") // model explicit (default is "")
	argv := append(flags, "<sys>\n\n<payload>")           // positional: payload appended per §12.2
	want := []string{
		"agent",                 // command (§12.7 command = "agent")
		"--model", "gpt-5",      // model_flag + user-set model
		"--mode", "ask", "--trust", // bare_flags (read-only + skip ws-trust)
		"-p",                    // print_flag LAST per §12.2
		"<sys>\n\n<payload>",    // positional payload (sys prepended — no sys flag on agent)
	}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("cursor rendered argv:\n got %v\nwant %v", argv, want)
	}
}

// The DecodeParity table update — add two rows. The existing loop + reflect.DeepEqual handle them.
// (codexTOML has BOTH revisions; cursorTOML is verbatim §12.7 with detect="agent", subcommand=[].)
func TestBuiltinManifests_DecodeParity(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  Manifest
		toml string
	}{
		{"pi", builtinPi(), piTOML},
		{"claude", builtinClaude(), claudeTOML},
		{"gemini", builtinGemini(), geminiTOML},       // S2 fixture
		{"opencode", builtinOpenCode(), opencodeTOML}, // S2 fixture
		{"codex", builtinCodex(), codexTOML},          // codexTOML = §12.7 codex with BOTH revisions
		{"cursor", builtinCursor(), cursorTOML},       // cursorTOML = verbatim §12.7 cursor
	} {
		var decoded Manifest
		if err := toml.Unmarshal([]byte(tc.toml), &decoded); err != nil {
			t.Fatalf("%s: decode failed: %v", tc.name, err)
		}
		if !reflect.DeepEqual(tc.got, decoded) {
			t.Errorf("%s: built-in != decoded TOML\n built-in: %+v\n decoded:  %+v", tc.name, tc.got, decoded)
		}
	}
}

// KeysAndCount update — 4 → 6 keys.
func TestBuiltinManifests_KeysAndCount(t *testing.T) {
	m := BuiltinManifests()
	if len(m) != 6 {
		t.Fatalf("BuiltinManifests() returned %d keys, want 6", len(m))
	}
	for _, k := range []string{"pi", "claude", "gemini", "opencode", "codex", "cursor"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
}
```

> **NOTE on `assertStr(t, "Name", &m.Name, "cursor")`**: S1/S2's `assertStr` helper takes a `*string`. For
> the plain `Name` field, either pass `&m.Name` (address of a plain string is a `*string`) or assert
> directly `if m.Name != "cursor" { t.Errorf(...) }`. The existing PiFields/ClaudeFields tests do NOT
> assert Name (NameMatchesKey covers it); adding a Name check in CursorFields is optional but clarifies
> the detect≠name point. Prefer the direct compare for Name.

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. builtin.go has zero imports (unchanged from S1/S2); builtin_test.go uses testing +
        reflect + go-toml/v2 (all already in go.mod from S1; S3 adds nothing). `go mod tidy` MUST be a
        no-op. `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - internal/provider → (stdlib: NONE in builtin.go; testing+reflect+toml in builtin_test.go) ONLY.
        UNCHANGED from S1/S2 — no new edge.
  - internal/provider → internal/config : FORBIDDEN (cycle; same as S1/S2). The REGISTRY (P1.M2.T3) is
        the sole importer of both config and provider.

FROZEN FILES (do NOT edit):
  - internal/provider/manifest.go + manifest_test.go (S1): the Manifest type + strPtr/boolPtr + Validate
        are a CONTRACT. This subtask MODIFIES builtin.go/builtin_test.go ONLY.
  - internal/provider/merge.go + merge_test.go (S2 merge): MergeManifest. This subtask does NOT depend on
        it; do NOT edit it.
  - internal/config/* (P1.M1.T4), internal/git/* (P1.M1.T2/T3), cmd/stagecoach/main.go, Makefile.

DOWNSTREAM CONTRACTS (do NOT implement here — just honor the shapes they will consume):
  - P1.M2.T3 (registry): `builtins := BuiltinManifests(); base := builtins["codex"]` (or "cursor");
        `merged := MergeManifest(base, decode(reencode(config.Providers["codex"])))`. This subtask
        provides codex + cursor as bases. The registry's $PATH detection uses DetectCommand() → "codex"
        for codex, "agent" for cursor (≠ key "cursor").
  - P1.M2.T4 (renderer): reads the RESOLVED manifest per §12.2 — the two render tests prove the DATA is
        sufficient to render the codex(stdin)/cursor(positional) argvs. For codex, the renderer will emit
        the literal "-" positional token that signals stdin to codex exec (NOT modeled in renderArgs —
        that is the renderer's job; this subtask only sets PromptDelivery="stdin"). The real renderer will
        have its OWN tests; renderArgs here is throwaway.
  - P1.M2.T5 (executor): reads *resolved.Command ("codex"/"agent") + resolved.Env (nil → none); for codex
        the payload goes to the stdin pipe (with "-" as the trailing positional); for cursor it is the
        positional arg.
  - P1.M2.T6 (parser): reads *resolved.Output ("raw"), *resolved.JsonField (""), *resolved.StripCodeFence (true).
  - P1.M5.T1.S2 (real-agent scaffold): confirms the two TO CONFIRM items against REAL codex/cursor runs.
  => BuiltinManifests() signature + the codex/cursor field values are now FROZEN for downstream. Do not
     change them after this subtask.

NO DATABASE / NO ROUTES / NO CLI / NO RENDERER/EXECUTOR/PARSER / NO REGISTRY / NO providers/*.toml FILES
(P1.M5.T2).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1 (builtin.go):
gofmt -w internal/provider/builtin.go internal/provider/builtin_test.go
test -z "$(gofmt -l internal/provider/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/provider/        # (and `go vet ./...`) Expect zero diagnostics.
go build ./...                     # Whole module compiles. Expect exit 0.
# Expected: clean. builtin.go STILL has ZERO imports (verify) — the only `import` lines are in builtin_test.go.
grep -n '^import\|^	"' internal/provider/builtin.go && echo "note: builtin.go has imports (should be NONE)" || echo "builtin.go zero-imports (good)"

# Confirm NO new dependency + NO edit to S1(manifest)/S2(merge) files + no config edge:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"   # MUST be empty.
git diff --exit-code internal/provider/manifest.go internal/provider/manifest_test.go internal/provider/merge.go internal/provider/merge_test.go && echo "S1(manifest)+S2(merge) files UNCHANGED (expected)"   # MUST be empty.
grep -n 'internal/config' internal/provider/builtin.go && echo "BAD: config import" || echo "no config import (good)"
```

### Level 2: Unit Tests (Component Validation)

```bash
# All tests (white-box; no git/exec/config needed — pure literal data + toml round-trip + §12.2 port):
go test -race ./internal/provider/ -v
# Expected: PASS — S1/S2's pi/claude/gemini/opencode tests (PiFields, ClaudeFields, GeminiFields,
#   OpenCodeFields, RenderedCommand_Pi_MatchesCommitPi, RenderedCommand_Gemini, RenderedCommand_OpenCode,
#   FreshEachCall, NameMatchesKey, Validate) STILL GREEN; the 2 updated (KeysAndCount now 6, DecodeParity
#   now 6 rows) + 4 new (CodexFields, CursorFields, RenderedCommand_Codex, RenderedCommand_Cursor) GREEN.
#   Validate + NameMatchesKey now also cover codex/cursor via the larger map.

# Full suite must stay green (no regression; confirms no stray import edge broke config/git):
go test -race ./...
# Expected: all packages PASS (config, git, provider).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build + scope checks:
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"   # main.go stub still links.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged"
# Confirm this subtask touched ONLY the two builtin*.go files:
git diff --exit-code -- internal/config internal/git cmd Makefile internal/provider/manifest.go internal/provider/manifest_test.go internal/provider/merge.go internal/provider/merge_test.go && echo "frozen + S1(manifest) + S2(merge) files UNCHANGED by this subtask"
grep -n 'func builtinCodex\|func builtinCursor' internal/provider/builtin.go   # MUST print both function lines.
grep -c '"codex"\|"cursor"' internal/provider/builtin.go                       # MUST be >= 2 (the map keys).
# Expected: binary builds; go.mod/go.sum unchanged; frozen+S1(manifest)+S2(merge) files unchanged; both
# constructors present; the map has 6 keys.

# Coverage of the new code (Makefile has a coverage target):
go test -race ./internal/provider/ -coverprofile=/tmp/cov.out && go tool cover -func=/tmp/cov.out | grep -E 'builtinCodex|builtinCursor|BuiltinManifests'
# Expected: builtinCodex + builtinCursor at/near 100% line coverage (exercised by Fields/DecodeParity/
# Render/Keys/Validate). (make coverage runs the project-wide gate; the ≥85% target is enforced at P1.M5.T3.S3.)

# Smoke the two render equivalences directly (the argv pins for the renderer):
go test -race ./internal/provider/ -run 'TestBuiltinManifests_RenderedCommand_Codex|TestBuiltinManifests_RenderedCommand_Cursor' -v
# Expected: PASS — codex renders the stdin argv (revised bare_flags); cursor renders the §12.2 positional argv.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Lint gate (project-wide):
golangci-lint run ./internal/provider/ 2>/dev/null || echo "golangci-lint not installed (Makefile lint gate is project-wide; run `make lint` in CI)."

# Cross-check against Appendix D (h2.27) quick-reference table by eye:
#   codex row: command `codex exec`, delivery stdin [revised], model flag -m, sys-prompt *(prepend)*,
#     bare `--sandbox read-only --ephemeral` [revised], output raw. ✔
#   cursor row: command `agent`, delivery positional, model flag --model, sys-prompt *(prepend)*,
#     bare `--mode ask --trust`, output raw. ✔
# The DecodeParity + Fields tests already assert these mechanically; this is a human sanity pass.

# Sanity: confirm codex's two revisions are the ONLY diffs from verbatim §12.7 codex (diff the TOMLs
# mentally): every §12.7 codex line is present in codexTOML except prompt_delivery (positional→stdin) and
# bare_flags (dropped --ask-for-approval; added --ephemeral). ✔  cursorTOML is byte-identical to §12.7.

# Confirm the two TO CONFIRM comments are present in the constructors:
grep -n 'TO CONFIRM' internal/provider/builtin.go   # MUST print 2 lines (one in builtinCodex, one in builtinCursor).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, `go mod tidy` is a
      no-op; `git diff --exit-code go.mod go.sum` empty; `builtin.go` STILL has ZERO imports.
- [ ] Level 2 green: `go test -race ./internal/provider/ -v` (S1/S2's tests + 2 updated + 4 new) AND
      `go test -race ./...`.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; `manifest.go`/`manifest_test.go` (S1 manifest) +
      `merge.go`/`merge_test.go` (S2 merge) unchanged; every file outside the two modified `builtin*.go`
      files unchanged; both new constructors present; map has 6 keys.
- [ ] Level 4: the two `// TO CONFIRM` comments present in builtin.go.

### Feature Validation

- [ ] `BuiltinManifests() map[string]Manifest` returns exactly `{"pi","claude","gemini","opencode",
      "codex","cursor"}`.
- [ ] codex manifest: every field per §12.7 (with BOTH revisions); `PromptDelivery`=="stdin" (rev #1);
      `Subcommand`==`["exec"]` (non-nil); `PrintFlag`/`DefaultModel`/`SystemPromptFlag`/`ProviderFlag` are
      NON-NIL `*""`; `BareFlags`==`["--sandbox","read-only","--ephemeral"]` (rev #2);
      `DefaultProvider`/`PromptFlag`/`JsonField`/`RetryInstruction`/`Env` nil.
- [ ] cursor manifest: every field per §12.7 (verbatim); `Detect`/`Command`=="agent" (≠ Name "cursor");
      `Subcommand`==`[]string{}` (NON-NIL empty); `DefaultModel`/`SystemPromptFlag`/`ProviderFlag` are
      NON-NIL `*""`; `BareFlags`==`["--mode","ask","--trust"]`; `DefaultProvider`/`PromptFlag`/`JsonField`/
      `RetryInstruction`/`Env` nil.
- [ ] Both `Validate()` → nil.
- [ ] `reflect.DeepEqual(built-in, decode(TOML))` holds for both (codexTOML = §12.7 codex with the two
      revisions; cursorTOML = verbatim §12.7 cursor).
- [ ] codex rendered via §12.2 == `["codex","exec","-m","gpt-5","--sandbox","read-only","--ephemeral"]`
      (stdin).
- [ ] cursor rendered via §12.2 == `["agent","--model","gpt-5","--mode","ask","--trust","-p",
      "<sys>\n\n<payload>"]` (positional; comment notes §12.7 illustrative block orders tokens differently).
- [ ] S1/S2's pi/claude/gemini/opencode tests + helpers UNCHANGED and still passing.

### Code Quality Validation

- [ ] Follows repo conventions: white-box `package provider`, stdlib `testing`, `t.Errorf`,
      `reflect.DeepEqual`, reuse of S1/S2's `assertStr`/`assertNilStr`/`renderArgs` helpers (mirrors
      `builtin_test.go`); free-function constructors; zero imports in `builtin.go`.
- [ ] File placement matches the desired tree (MODIFY `builtin.go` + `builtin_test.go` ONLY — S1(manifest)/
      S2(merge) untouched).
- [ ] The explicit-empty vs absent pattern is reproduced exactly for both providers (4 + 3 non-nil-empty
      scalars; absent fields nil) — NOT flattened.
- [ ] cursor's `Subcommand` is `[]string{}` (non-nil empty), matching §12.7's `subcommand = []`
      (FINDING D) — NOT omitted to nil.
- [ ] cursor's `Detect`/`Command` are "agent" (≠ Name "cursor"), matching §12.7 — NOT set to "cursor".
- [ ] The two `// TO CONFIRM` comments are present and cite the open runtime questions (§12.7.2 honesty).
- [ ] `internal/provider` production code still imports nothing outside stdlib (S1/S2/S3 discipline
      preserved); `builtin.go` imports NOTHING.
- [ ] No premature scope: no registry (P1.M2.T3), no Validate/Resolve call inside constructors, no
      renderer/exec/parse (T4/T5/T6), no `providers/*.toml` (P1.M5.T2), no codex `-` token (renderer job),
      no cursor `--output-format json` (downstream).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Doc comment on `BuiltinManifests` (updated to all six providers) + each new constructor citing the
      PRD section + the codex revisions + the explicit-empty notes + the TO CONFIRM items (seeds
      `providers show` / reference-file docs later).
- [ ] No new env vars / no user-facing docs (PRD "DOCS: none — compiled-in defaults"; public docs come
      with `providers show` P1.M4.T1.S3 and reference files P1.M5.T2).

---

## Anti-Patterns to Avoid

- ❌ Don't create new files — this subtask MODIFIES the two `builtin*.go` files S1 created and S2 extended
  (extends the map + adds constructors/tests). Creating a new file duplicates the `BuiltinManifests`
  surface and breaks the single-source-of-truth invariant the registry depends on.
- ❌ Don't paste the verbatim §12.7 codex TOML as the codex decode-parity fixture — it has
  `prompt_delivery = "positional"` AND `--ask-for-approval never` in bare_flags, both of which contradict
  the mandated revisions; the test WILL FAIL. Use the fixture with BOTH revisions (+ deviation comments).
- ❌ Don't keep `--ask-for-approval never` in codex's bare_flags — external_deps.md §codex proves it is NOT
  a `codex exec` flag (codex exec is already non-interactive). DROP it and ADD `--ephemeral`.
- ❌ Don't set cursor's `Detect`/`Command` to "cursor" — §12.7 writes "agent" (the standalone binary).
  cursor is the ONLY provider where detect ≠ name.
- ❌ Don't omit cursor's `Subcommand` (leaving it nil) — §12.7 writes `subcommand = []` which decodes
  NON-NIL empty; nil fails decode-parity. Write `Subcommand: []string{}` explicitly.
- ❌ Don't "helpfully" set a default model for codex/cursor — `default_model = ""` is INTENTIONAL (codex
  reads ~/.codex/config.toml; cursor has per-account model availability; require the user to set it).
- ❌ Don't invent a system-prompt flag value for codex/cursor — neither CLI has one; both set
  `system_prompt_flag = ""` (explicit empty) and the §12.2 renderer prepends the sys prompt to the payload.
- ❌ Don't change the `renderArgs`/`assertStr`/`assertNilStr` helper signatures — S1/S2 render tests depend
  on them. For the cursor positional render, append the payload manually; for codex stdin, don't.
- ❌ Don't try to hand-match the §12.7 cursor "Rendered" prose ordering in the render test — §12.2 is
  authoritative; the test asserts the `renderArgs` output (with a comment on the order difference).
- ❌ Don't forget to UPDATE `TestBuiltinManifests_KeysAndCount` (4→6) — it WILL fail once the map has 6
  entries. Same for the `DecodeParity` table (+2 rows).
- ❌ Don't forget the two `// TO CONFIRM (integration): …` comments — the work item explicitly requires
  documenting them (§12.7.2 honesty about stubbed runtime behavior).
- ❌ Don't call `Validate`/`Resolve` inside the constructors — they BUILD; the registry validates.
- ❌ Don't add imports to `builtin.go` — it stays import-free (literal `strPtr`/`boolPtr` + slice literals).
- ❌ Don't edit the frozen files (`manifest.go`, `merge.go`, config, git, main.go, Makefile, go.mod).
