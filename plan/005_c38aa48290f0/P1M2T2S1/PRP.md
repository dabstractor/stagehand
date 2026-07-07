name: "P1.M2.T2.S1 — --context user-payload injection (message + planner roles)"
description: |
  Add a flag-only `--context <text>` (FR-F7) that appends the block
  `Additional context from the user (treat as authoritative):\n<text>` to the USER PAYLOAD for the
  message and planner roles ONLY. The block is inserted after the instruction line and before the diff,
  and BEFORE the duplicate-rejection block when both are present (ordering contract:
  instruction → context → rejection → diff). Two builders change signature — `BuildUserPayload` (payload.go,
  §17.3) and `BuildPlannerUserPayload` (planner.go, §17.5) — and the 3 message call sites + 1 planner call
  site thread `cfg.Context` through. Stager and arbiter payloads are UNCHANGED. Hook `exec` takes no
  `--context` (N/A). `context == ""` output stays BYTE-IDENTICAL to today. This subtask does NOT touch the
  SYSTEM-prompt builders (that is S3, implementing in parallel) or `--template` (S2.T2.S2).

---

## Goal

**Feature Goal**: A user can pass `--context "this is a hotfix for #812"` and that authoritative context is
threaded into the user payload the agent sees for BOTH the message role and the planner role, positioned
after the instruction line, before the diff, and before the duplicate-rejection block when both are present.
Absent the flag, every payload is byte-for-byte what it is today.

**Deliverable**: A flag-only `--context` (config field `Config.Context`, wired only via `fs.Changed` in
`loadFlags` — no env, no git key, no config-file key), a shared `contextBlock(text)` helper in
`internal/prompt/payload.go`, extended `BuildUserPayload(diff, context string, rejected []string)` and
`BuildPlannerUserPayload(diff, context string, forcedCount int)` with an explicit ordering contract, the four
call sites updated, canonical-exact + table-driven unit tests locking the ordering and the byte-identity of
the no-context path, and the `docs/cli.md` `--context` row (Mode A).

**Success Definition**:
- `context == ""` → both builders emit BYTE-IDENTICAL output to today (existing `payload_test.go`
  canonical-exact tests pass with only the new `""` arg added — the load-bearing regression guard).
- `--context "X"` (no rejection) → the message payload is
  `userInstruction(":") + "\n\n" + "Additional context from the user (treat as authoritative):\nX" + "\n\n" + diff`.
- `--context "X"` + a rejection list → the block appears AFTER the instruction line and BEFORE the
  `IMPORTANT: …` rejection preamble (ordering: instruction → context → rejection → diff); the instruction
  still ends with a PERIOD (rejection path punctuation unchanged by context).
- The planner payload with `--context "X"` inserts the same block after `plannerUserInstruction`, before the
  diff (and after the forced-count directive in forced mode).
- Stager (`stager.go`) and arbiter (`arbiter.go`) payloads are untouched; hook exec has no `--context`.
- `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run` all green.

## User Persona

**Target User**: The "plan-holder" (PRD §7.1) who knows *why* a change exists in a way the diff cannot show
("this is a hotfix for #812", "revert of yesterday's perf experiment") and wants the agent to treat that as
authoritative when writing the message or partitioning commits.

**Use Case**: `stagecoach --context "hotfix for the auth regression in #812"` — the message role receives the
context so it can name the intent; on the decompose path the planner receives it so it groups the changes
sensibly. Composes with `--format`/`--locale` (S3) and duplicate-rejection retries (§9.7).

**User Journey**: user runs `stagecoach --context "…"` → `config.Load` records `cfg.Context` from the flag
(flag-only; per-invocation) → the message/planner call sites pass `cfg.Context` into the payload builders →
the builders insert the `Additional context…` block in the correct slot → the agent's stdin carries it.

**Pain Points Addressed**: the diff alone can't convey intent/motivation; §9.19 FR-F7 gives the user a
per-invocation channel to state it authoritatively without editing config or history.

## Why

- **FR-F7 (PRD §9.19)**: *"`--context "<text>"` (flag only; per-invocation information by nature) appends a
  block to the **user payload** for the message and planner roles:
  `Additional context from the user (treat as authoritative):\n<text>`."* This subtask IS that feature.
- **§17.8 (Context)**: *"inserted into the user payload (message and planner roles), after the instruction
  line and before the diff — the same slot the duplicate-rejection block occupies (§17.3), and before it when
  both are present."* → the exact ordering contract this PRP implements.
- **Scope fences**: flag-only (no env/git/config key per FR-F7); message + planner ONLY (stager/arbiter
  unchanged); hook exec has no flag surface (N/A). System-prompt format/locale is S3 (parallel, disjoint
  functions). `--template` is S2.T2.S2. See `research/design-decisions.md`.

## What

Add `--context`, thread `cfg.Context` into the two user-payload builders, and insert the context block in the
correct slot with an explicit, test-locked ordering contract.

### Success Criteria

- [ ] `--context` flag registered (root.go); `Config.Context` field added (`toml:"-"`, flag-only); wired ONLY
      via `fs.Changed("context")` in `loadFlags` — no env, no git, no file key.
- [ ] `BuildUserPayload(diff, context string, rejected []string)`: inserts `Additional context…` block after
      the instruction line, before the rejection block (if any), before the diff. `context==""` byte-identical.
- [ ] `BuildPlannerUserPayload(diff, context string, forcedCount int)`: inserts the same block after
      `plannerUserInstruction`, before the diff (after the forced-count line in forced mode). `context==""`
      byte-identical.
- [ ] All 4 call sites (generate.go, decompose/message.go, pkg/stagecoach/stagecoach.go, decompose/planner.go)
      pass `cfg.Context`; stager/arbiter builders untouched.
- [ ] Canonical-exact + property tests: ordering (instruction→context→rejection→diff), byte-identity of the
      empty-context path, planner normal + forced.
- [ ] `docs/cli.md` `--context` row added (env + git-config columns = `—`).
- [ ] Full build/test/vet/lint green.

## All Needed Context

### Context Completeness Check

_This PRP names the two exact builder signatures to change with line numbers, the verbatim §17.8 block text
and its exact newline placement, the four call sites (with the `cfg` variable name at each), the flag-only
config wiring pattern (mirroring `flagExclude`/`Exclude`), the exact test file & canonical-exact style to
mirror (`payload_test.go`), the docs row format, and the scope fences vs S3/S2.T2.S2. An implementer with no
prior codebase knowledge can complete it from this document + codebase access._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/prd_snapshot.md
  why: Authoritative spec. §9.19 FR-F7 (the feature) + §17.8 "Context" (exact block text + slot ordering) +
       §17.3 (the user-payload structure the context block inserts into) + §17.5 (planner payload).
  section: "§9.19 FR-F7" + "§17.8 Format modes, locale, and context" + "§17.3 The user payload" + "§17.5 Planner prompt"
  critical: |
    §17.8 Context block, VERBATIM (exactly one '\n' between the header line and the text):
        Additional context from the user (treat as authoritative):
        <text>
    Slot: "after the instruction line and before the diff — the same slot the duplicate-rejection block
    occupies (§17.3), and BEFORE it when both are present." → ordering: instruction → context → rejection → diff.
    FR-F7: flag ONLY (no env, no config key, no git key) — per-invocation by nature.

- docfile: plan/005_c38aa48290f0/P1M2T2S1/research/design-decisions.md
  why: The load-bearing calls — colon/period is NOT changed by context (governed only by len(rejected));
       flag-only resolution precedent; planner forced-count interaction; the 4 call sites; the S3 disjointness proof.
  section: "all (short)"
  critical: "Decision §2: context does NOT change the instruction punctuation — keeps context=='' byte-identical."

- file: internal/prompt/payload.go
  why: THE primary file to edit. `BuildUserPayload(diff string, rejected []string)` (line 77): NORMAL path
       (len(rejected)==0) returns `userInstruction + "\n\n" + diff` (line 80); REJECTION path (line 84-98)
       builds with strings.Builder. Constants `userInstruction`(21, colon), `userInstructionReject`(28, period),
       `rejectionPreamble`(33), `rejectionEpilogue`(38) carry NO trailing newline; the func owns all inter-block
       newlines. Add a `contextBlock(text)` helper + a `contextConst` and insert the block in BOTH paths.
  pattern: |
    - Add: const contextIntro = "Additional context from the user (treat as authoritative):"
    - Add helper: contextBlock(text) returns "" if text=="" else contextIntro + "\n" + text (NO trailing newline).
    - NORMAL path: if context=="" keep the fast path (byte-identical); else
        userInstruction + "\n\n" + contextBlock(context) + "\n\n" + diff.
    - REJECTION path: after `b.WriteString(userInstructionReject); b.WriteString("\n\n")`, if context!="" write
        contextBlock(context) + "\n\n" BEFORE `b.WriteString(rejectionPreamble)`. Everything else unchanged.
  gotcha: "context=='' MUST reproduce today's bytes EXACTLY in BOTH paths — the existing canonical-exact tests
           (payload_test.go:11,28) are the proof; only their call args gain a \"\" (context)."

- file: internal/prompt/planner.go
  why: SECOND file to edit — the USER-payload func only. `BuildPlannerUserPayload(diff string, forcedCount int)`
       (line 109): normal returns `plannerUserInstruction + "\n\n" + diff` (111); forced prepends
       `forced + "\n"` (113-114). Add `context string` param; insert `contextBlock(context) + "\n\n"` after the
       instruction line, before the diff. `plannerUserInstruction`(44) has a trailing COLON.
  pattern: |
    normal: plannerUserInstruction + "\n\n" + [contextBlock+"\n\n" if context!=""] + diff
    forced: forced + "\n" + plannerUserInstruction + "\n\n" + [contextBlock+"\n\n" if context!=""] + diff
    Reuse the SAME contextBlock helper from payload.go (same package `prompt` — no import).
  gotcha: "Do NOT touch BuildPlannerSystemPrompt (that is S3's function, implementing in parallel — same file,
           DIFFERENT function). Do NOT touch plannerSystemPrompt/plannerUserInstruction consts or ParsePlannerOutput."

- file: internal/config/config.go
  why: Add the flag-only Config field. Format(85)/Locale(89) show the field+doc style; but Context is FLAG-ONLY.
       Defaults() (136) sets every field — add `Context: ""`. Config is NEVER decoded from a TOML file (a
       separate fileConfig is — see the Providers `toml:"-"` note at line 105-107), so `toml:"-"` on Context
       guarantees no file source.
  pattern: |
    // Context is the §9.19 FR-F7 per-invocation context text. FLAG-ONLY: no env, no git key, no config-file
    // key (per-invocation by nature). Injected into the message + planner USER payloads (§17.8). Empty = no block.
    Context string `toml:"-"`
    ...and in Defaults(): Context: "",   // §9.19 FR-F7 default (empty = no context block)
  gotcha: "toml:\"-\" — Context must NEVER come from a file. Do NOT add a [generation].context key to fileConfig."

- file: internal/config/load.go
  why: The flag-only wiring seam. loadFlags (the func containing the format/locale block at 335-345) is where
       flags reach cfg via fs.Changed. Add a context block there. Do NOT add anything to loadEnv (env) or the
       git/file loaders. No validation (like Locale, FR-F6 — but context isn't even validated by spec).
  pattern: |
    // §9.19 FR-F7 — context via CLI flag ONLY (no env/git/file source; per-invocation). Mirrors --exclude's
    // flag-only discipline (there is no STAGECOACH_CONTEXT / stagecoach.context / [generation].context).
    if fs.Changed("context") {
        if v, err := fs.GetString("context"); err == nil {
            cfg.Context = v
        }
    }
  gotcha: "Flag-only: NOTHING in loadEnv, NOTHING in the git-config reader, NOTHING in fileConfig. Only loadFlags."

- file: internal/cmd/root.go
  why: Register the --context flag. flagExclude (line 69) is the flag-only precedent (a var read ONLY via
       fs.Changed, never directly). The format/locale flag vars are at 72-76; StringVar registrations at 149-155.
  pattern: |
    - Add near the format/locale vars (72-76):  var flagContext string
    - Register alongside --format/--locale (after line 155):
        pf.StringVar(&flagContext, "context", "",
            "Extra context appended to the message+planner payload, e.g. \"hotfix for #812\" "+
                "(flag only; per-invocation — no env/git/config key)")
  gotcha: "Flag-only — do NOT add an env/git mention in the help string beyond noting there is none."

- file: internal/generate/generate.go
  why: CALL SITE #1 (message role). Line 194: `payload := prompt.BuildUserPayload(diff, rejected)` inside the
       retry loop (cfg is `cfg`, a config.Config). cfg.Context is loop-invariant → pass it every attempt.
  pattern: "payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)"

- file: internal/decompose/message.go
  why: CALL SITE #2 (decompose message role; also produces the arbiter N+1 message). Line 119:
       `payload := prompt.BuildUserPayload(diff, rejected)`; here cfg is `deps.Config`.
  pattern: "payload := prompt.BuildUserPayload(diff, deps.Config.Context, rejected)"

- file: pkg/stagecoach/stagecoach.go
  why: CALL SITE #3 (public API wrapper — a third copy of the generation loop). Line 481:
       `payload := prompt.BuildUserPayload(diff, rejected)`; cfg is `cfg`.
  pattern: "payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)"
  gotcha: "Easy to miss — a missed caller is a build break. grep-verify (see gotchas)."

- file: internal/decompose/planner.go
  why: CALL SITE #4 (planner role). Line 85: `basePayload := prompt.BuildPlannerUserPayload(diff, forcedCount)`;
       cfg is `deps.Config`.
  pattern: "basePayload := prompt.BuildPlannerUserPayload(diff, deps.Config.Context, forcedCount)"

- file: internal/prompt/payload_test.go
  why: THE test-pattern model. TestBuildUserPayload_NormalCanonicalExact(11) & _RejectionCanonicalExact(28)
       pin FULL bytes; _Properties(52) is table-driven with `check func(t, p string)` using
       strings.HasPrefix/Contains/Count/Index; _EdgeCases(163). Helpers `near`/`suffix` live in system_test.go
       (same package) — do NOT redeclare (payload_test.go:193-194). Stdlib testing only.
  pattern: |
    - Update the 3 existing BuildUserPayload calls to pass "" for context (proves empty-context byte-identity).
    - Add cases for context!="": normal (block after colon-instruction, before diff), rejection (block AFTER
      the period-instruction and BEFORE "IMPORTANT:"), and a canonical-exact for each.
    - Add a BuildPlannerUserPayload context test (normal + forced) — mirror planner_test.go if present.
  gotcha: "Assert the block header is exactly 'Additional context from the user (treat as authoritative):' and
           the text follows on the NEXT line (single '\\n'). Assert ORDERING with strings.Index (context before
           IMPORTANT before diff)."

- docfile: docs/cli.md
  why: Mode-A doc. 6-col global-flags table `| Flag | Type | Default | Env | Git config | Description |`.
       --format(38)/--locale(39) rows are the shape. Add the --context row after --locale.
  section: "Global flags table (lines 24-54)"
  critical: |
    Add: | `--context <text>` | string | "" | — | — | Extra authoritative context appended to the message and
    planner payloads (e.g. `"hotfix for #812"`). Flag only — per-invocation; no env var, git-config, or
    config-file key. |
```

### Current Codebase tree (relevant slice)

```bash
internal/prompt/
  payload.go       # BuildUserPayload (§17.3) — EDIT (add context param + contextBlock helper; insert block)
  planner.go       # BuildPlannerUserPayload (§17.5) — EDIT (add context param; insert block). Do NOT touch
                   #   BuildPlannerSystemPrompt (S3) / plannerSystemPrompt / ParsePlannerOutput.
  stager.go        # UNCHANGED (no --context for the stager)
  arbiter.go       # UNCHANGED (no --context for the arbiter)
  payload_test.go  # EDIT — thread "" through existing calls; add context cases (message + planner)
internal/config/config.go   # ADD Config.Context (toml:"-", flag-only) + Defaults() Context: ""
internal/config/load.go     # ADD fs.Changed("context") block in loadFlags (NO env/git/file wiring)
internal/cmd/root.go        # ADD flagContext var + --context StringVar registration
internal/generate/generate.go        # CALL SITE #1 (line 194)
internal/decompose/message.go        # CALL SITE #2 (line 119)
pkg/stagecoach/stagecoach.go           # CALL SITE #3 (line 481)
internal/decompose/planner.go        # CALL SITE #4 (line 85)
docs/cli.md                          # ADD --context row (Mode A)
```

### Desired Codebase tree

No new files. Edits only (payload.go, planner.go, payload_test.go, config.go, load.go, root.go, the 4 call
sites, docs/cli.md). The `contextBlock` helper + `contextIntro` const live in payload.go and are reused by
planner.go (same package).

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (byte-identity): context=="" MUST reproduce today's bytes EXACTLY in EVERY path of BOTH builders.
// Gate the block on `context != ""`. The existing payload_test.go canonical-exact tests are the proof — keep
// their `want` strings unchanged; only add a "" arg to the calls. If a canonical test needs its want edited,
// the empty-context path was broken.

// CRITICAL: The context block header ends with a COLON and the text follows on the NEXT line (single '\n'):
//   "Additional context from the user (treat as authoritative):\n<text>"
// Then the ASSEMBLER appends "\n\n" before the next element (rejection block or diff) — matching payload.go's
// "constants carry no trailing newline; the Build func owns inter-block newlines" rule.

// CRITICAL (ordering): instruction → context → rejection → diff. In the rejection path the context block goes
// AFTER `userInstructionReject + "\n\n"` and BEFORE `rejectionPreamble`. Do NOT change the colon/period choice:
// it is governed ONLY by len(rejected) (§17.3). context!="" with no rejection still uses the COLON instruction.

// CRITICAL: FOUR callers. grep to confirm none missed after the signature change:
//   grep -rn 'BuildUserPayload\|BuildPlannerUserPayload' --include='*.go' | grep -v '_test.go' | grep -v 'func Build'
// (expected: generate.go:194, decompose/message.go:119, pkg/stagecoach/stagecoach.go:481, decompose/planner.go:85)

// CRITICAL (parallel S3): S3 (P1.M2.T1.S3) edits the SYSTEM-prompt builders (BuildSystemPrompt/
// BuildFallbackPrompt/BuildPlannerSystemPrompt) and explicitly does NOT touch payload.go or the USER-payload
// funcs. planner.go is shared but the functions are disjoint (BuildPlannerSystemPrompt=S3 vs
// BuildPlannerUserPayload=here). Do NOT edit BuildPlannerSystemPrompt or add format/locale here.

// FLAG-ONLY (FR-F7): Context comes ONLY from the flag. NOTHING in loadEnv (no STAGECOACH_CONTEXT), NOTHING in
// the git-config reader (no stagecoach.context), NOTHING in fileConfig (no [generation].context). toml:"-" on
// the Config field. Context is passed VERBATIM — no trimming, no validation (an empty flag value == unset).

// GOTCHA: Do NOT modify the stager (stager.go) or arbiter (arbiter.go) payload builders — the work item scopes
// context to the message and planner roles ONLY. Hook exec (P1.M3) takes no --context (git owns the commit).
```

## Implementation Blueprint

### Data models and structure

No new types. One new unexported const + one helper in `internal/prompt/payload.go`:

```go
// §17.8 context block header (FR-F7). Committed VERBATIM (trailing COLON; the text follows on the next line).
// NO trailing newline — BuildUserPayload / BuildPlannerUserPayload own inter-block newline placement.
const contextIntro = "Additional context from the user (treat as authoritative):"

// contextBlock returns the §17.8 context block ("<intro>\n<text>") or "" when text is empty. Empty ⇒ the
// caller inserts nothing, preserving byte-identity with the pre-feature payload. Passed verbatim (no trim).
func contextBlock(text string) string {
	if text == "" {
		return ""
	}
	return contextIntro + "\n" + text
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/prompt/payload.go — contextIntro const + contextBlock helper + inject into BuildUserPayload
  - ADD contextIntro const and contextBlock(text) helper (snippets above), with doc comments citing §9.19
    FR-F7 / §17.8.
  - CHANGE signature: func BuildUserPayload(diff, context string, rejected []string) string
  - NORMAL path (len(rejected)==0):
      if context == "" { return userInstruction + "\n\n" + diff }      // byte-identical fast path
      return userInstruction + "\n\n" + contextBlock(context) + "\n\n" + diff
  - REJECTION path: after b.WriteString(userInstructionReject); b.WriteString("\n\n"):
      if context != "" { b.WriteString(contextBlock(context)); b.WriteString("\n\n") }   // BEFORE rejectionPreamble
      ...(rest unchanged)
  - UPDATE the func doc comment's ASSEMBLY block to include the context slot (ordering: instruction→context→rejection→diff).

Task 2: EDIT internal/prompt/planner.go — inject context into BuildPlannerUserPayload ONLY
  - CHANGE signature: func BuildPlannerUserPayload(diff, context string, forcedCount int) string
  - normal (forcedCount<=0):
      if context == "" { return plannerUserInstruction + "\n\n" + diff }
      return plannerUserInstruction + "\n\n" + contextBlock(context) + "\n\n" + diff
  - forced (forcedCount>0):
      base := forced + "\n" + plannerUserInstruction + "\n\n"
      if context != "" { base += contextBlock(context) + "\n\n" }
      return base + diff
  - PRESERVE: BuildPlannerSystemPrompt (S3), plannerSystemPrompt/plannerUserInstruction consts, ParsePlannerOutput.
  - UPDATE the func doc comment's ASSEMBLY block for the context slot.

Task 3: EDIT internal/config/config.go — add the flag-only Context field
  - ADD field `Context string `toml:"-"`` (doc: §9.19 FR-F7, flag-only, message+planner user payload, empty=no block).
  - ADD to Defaults(): Context: "",   // §9.19 FR-F7 default (empty = no context block)

Task 4: EDIT internal/config/load.go — wire --context in loadFlags ONLY
  - ADD (near the format/locale flag block ~335-345):
      if fs.Changed("context") { if v, err := fs.GetString("context"); err == nil { cfg.Context = v } }
  - Do NOT add anything to loadEnv / the git reader / fileConfig. No validation.

Task 5: EDIT internal/cmd/root.go — register --context
  - ADD var flagContext string (near flagFormat/flagLocale).
  - ADD pf.StringVar(&flagContext, "context", "", "<help — flag only; per-invocation>").

Task 6: EDIT the four call sites — thread cfg.Context
  - internal/generate/generate.go:194 → prompt.BuildUserPayload(diff, cfg.Context, rejected)
  - internal/decompose/message.go:119 → prompt.BuildUserPayload(diff, deps.Config.Context, rejected)
  - pkg/stagecoach/stagecoach.go:481 → prompt.BuildUserPayload(diff, cfg.Context, rejected)
  - internal/decompose/planner.go:85 → prompt.BuildPlannerUserPayload(diff, deps.Config.Context, forcedCount)
  - grep-verify zero remaining old-arity calls.

Task 7: EDIT internal/prompt/payload_test.go — thread "" + add context cases
  - Update the 3 existing BuildUserPayload(diff, ...) calls to BuildUserPayload(diff, "", ...) so the
    canonical-exact tests PROVE empty-context byte-identity (do NOT edit their want strings).
  - ADD canonical-exact cases:
      * normal + context "hotfix for #812": want = userInstruction + "\n\n" +
        "Additional context from the user (treat as authoritative):\nhotfix for #812" + "\n\n" + diff.
      * rejection + context: want has the context block AFTER the period-instruction+blank and BEFORE
        "IMPORTANT:" (assert with strings.Index ordering: context < IMPORTANT < diff).
  - ADD property cases: block ABSENT when context=="" (both paths); block PRESENT and header exact when set;
    ordering instruction→context→rejection→diff.
  - ADD BuildPlannerUserPayload context cases: normal (block after plannerUserInstruction, before diff) and
    forced (forced line, then instruction, then context block, then diff); context=="" byte-identical to today.
  - Reuse near/suffix from system_test.go (do NOT redeclare).

Task 8: EDIT docs/cli.md — add the --context row after --locale (line 39)
  - | `--context <text>` | string | "" | — | — | Extra authoritative context appended to the message and
    planner payloads (e.g. `"hotfix for #812"`). Flag only — per-invocation; no env var, git-config, or
    config-file key. |
```

### Implementation Patterns & Key Details

```go
// BuildUserPayload — context block inserted after the instruction, before rejection/diff. Empty ⇒ today's bytes.
func BuildUserPayload(diff, context string, rejected []string) string {
	if len(rejected) == 0 {
		if context == "" {
			return userInstruction + "\n\n" + diff // §17.3 NORMAL — byte-identical fast path
		}
		return userInstruction + "\n\n" + contextBlock(context) + "\n\n" + diff // instruction → context → diff
	}

	var b strings.Builder
	b.WriteString(userInstructionReject)
	b.WriteString("\n\n")
	if context != "" { // §17.8: context BEFORE the rejection block when both present
		b.WriteString(contextBlock(context))
		b.WriteString("\n\n")
	}
	b.WriteString(rejectionPreamble)
	b.WriteByte('\n')
	for _, s := range rejected {
		b.WriteString("- ")
		b.WriteString(s)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(rejectionEpilogue)
	b.WriteString("\n\n")
	b.WriteString(diff)
	return b.String()
}

// BuildPlannerUserPayload — context after the instruction line, before the diff (after forced-count directive).
func BuildPlannerUserPayload(diff, context string, forcedCount int) string {
	block := ""
	if context != "" {
		block = contextBlock(context) + "\n\n"
	}
	if forcedCount <= 0 {
		return plannerUserInstruction + "\n\n" + block + diff
	}
	forced := fmt.Sprintf("Produce EXACTLY %d commits from these changes (do not reconsider the count):", forcedCount)
	return forced + "\n" + plannerUserInstruction + "\n\n" + block + diff
}
// NOTE: when context=="" && forcedCount<=0 this equals plannerUserInstruction + "\n\n" + diff (today's bytes).
```

### Integration Points

```yaml
CONFIG (this subtask OWNS the field + flag; flag-only):
  - add: Config.Context string `toml:"-"` (config.go) + Defaults() Context: "".
  - wire: fs.Changed("context") in loadFlags (load.go) ONLY. No env, no git, no fileConfig.
  - flag: --context (root.go).

PROMPT PACKAGE:
  - edit: BuildUserPayload (payload.go) + BuildPlannerUserPayload (planner.go); shared contextBlock helper.
  - UNCHANGED: stager.go, arbiter.go, BuildPlannerSystemPrompt (planner.go), BuildSystemPrompt/BuildFallbackPrompt (S3).

CALL SITES (thread cfg.Context):
  - internal/generate/generate.go, internal/decompose/message.go, pkg/stagecoach/stagecoach.go (message role),
    internal/decompose/planner.go (planner role).

DOCS (Mode A):
  - docs/cli.md → --context global-flags row.

OUT OF SCOPE:
  - System-prompt format/locale (S3 P1.M2.T1.S3). --template/message finalization (S2.T2.S2).
  - Stager/arbiter payloads. Hook exec --context (N/A — git owns the commit; note only).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/prompt/payload.go internal/prompt/planner.go internal/prompt/payload_test.go \
        internal/config/config.go internal/config/load.go internal/cmd/root.go
go build ./...      # all four call sites MUST compile — proves no caller missed the arity change
go vet ./...
golangci-lint run

# Expected: zero errors. A build failure on a Build*UserPayload arity mismatch ⇒ a caller was missed — re-grep.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/prompt/... -v
# Must pass:
#  - TestBuildUserPayload_NormalCanonicalExact / _RejectionCanonicalExact with the new "" context arg,
#    want strings UNCHANGED (empty-context byte-identity — FR regression guard).
#  - New: context normal (block after colon-instruction, before diff); context+rejection (block after
#    period-instruction, BEFORE "IMPORTANT:"); ordering instruction→context→rejection→diff.
#  - New: BuildPlannerUserPayload context normal + forced; empty-context byte-identical.

go test ./internal/config/... -v      # Config.Context default "", flag wiring (if a loadFlags test exists)
go test ./internal/generate/... ./internal/decompose/... ./pkg/stagecoach/... -v   # call-site threading compiles + passes
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach
# Verify the flag exists and is documented as flag-only:
/tmp/stagecoach --help 2>&1 | grep -A1 -- '--context'
# Optional stub-agent smoke (mirror internal/generate/generate_test.go's STAGECOACH_STUB_STDINFILE pattern to
# assert the captured stdin CONTAINS "Additional context from the user (treat as authoritative):" when
# --context is set) — the user-payload half is delivered via stdin (render.go stdin delivery).
```

### Level 4: Regression & Cross-cutting

```bash
go test -race ./...          # full suite (make test)
make coverage-gate           # ≥85% on the core packages (unaffected packages stay green)
golangci-lint run ./...

# Byte-identity guard: with context=="" every payload equals the pre-change output. The retained
# payload_test.go canonical-exact tests enforce this — they MUST pass with only the "" arg added.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` compiles (all four Build*UserPayload call sites updated — no arity mismatch).
- [ ] `go test ./...` green; payload_test.go canonical-exact tests pass with only the `""` context arg added.
- [ ] `go vet ./...` + `golangci-lint run` clean; `gofmt` no diff.

### Feature Validation
- [ ] `--context ""`/unset → both builders byte-identical to today (all paths, incl. rejection + forced).
- [ ] `--context "X"` normal → block after colon-instruction, before diff; header text exact; text on next line.
- [ ] `--context "X"` + rejection → block AFTER period-instruction, BEFORE "IMPORTANT:" (ordering enforced).
- [ ] Planner payload gets the block in normal + forced modes.
- [ ] Stager + arbiter payloads unchanged; hook exec has no --context.
- [ ] Flag-only: no STAGECOACH_CONTEXT, no stagecoach.context, no [generation].context.

### Code Quality Validation
- [ ] Matches internal/prompt conventions: consts no trailing newline; assembler owns "\n" placement; doc
      comments cite §9.19 FR-F7 / §17.8; pure functions.
- [ ] contextBlock helper is shared (defined once in payload.go, reused by planner.go — same package).
- [ ] No system-prompt/format/locale/template changes; no stager/arbiter changes.

### Documentation & Deployment
- [ ] docs/cli.md has the `--context` row (env + git-config columns = `—`, flag-only).
- [ ] No new env vars / git keys / config-file keys introduced.

---

## Anti-Patterns to Avoid

- ❌ Don't change the colon/period instruction choice — it is governed ONLY by len(rejected) (§17.3); context
  keeps the same instruction. This preserves empty-context byte-identity.
- ❌ Don't add env/git/config-file resolution for context — FR-F7 is flag-only (mirror flagExclude discipline).
- ❌ Don't edit BuildPlannerSystemPrompt / BuildSystemPrompt / BuildFallbackPrompt — that is S3 (parallel).
- ❌ Don't touch stager.go or arbiter.go — context is message + planner ONLY.
- ❌ Don't miss pkg/stagecoach/stagecoach.go:481 — a third copy of the generation loop; a missed caller = build break.
- ❌ Don't trim/validate the context text — pass it verbatim; only "" gates the block.
- ❌ Don't give the context block a trailing newline in the helper — the assembler owns inter-block "\n\n".
- ❌ Don't build --template here — that is S2.T2.S2.

---

## Confidence Score

**9/10** for one-pass implementation success. The seam is narrow and fully pinned: two builders with exact
line numbers, the verbatim §17.8 block text with precise newline placement, the four call sites with their
`cfg` variable names, the flag-only config precedent (`flagExclude`/`Exclude`) copied directly, and the
existing `payload_test.go` canonical-exact tests that mechanically prove empty-context byte-identity. The one
genuine judgment call — that context does NOT alter the colon/period instruction — is resolved in favor of
the most spec-literal reading (§17.8 says "insert into the existing slot", not "change the instruction") and
locked by a canonical test; if review prefers otherwise it is a one-line change. Disjointness from the
parallel S3 work is verified (different functions, S3's own PRP fences out payload.go).
