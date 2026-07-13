name: "P1.M1.T1.S1 — CHROME-DISABLE notes for all 7 built-in providers in builtin.go (FR-C5)"
description: >
  Add a CHROME-DISABLE note paragraph (PRD §9.28 FR-C5, §12.7.1) to each of the 7 built-in provider
  doc-comment blocks in internal/provider/builtin.go, recording per chrome surface {skills,
  extensions/prompt-templates, context files, MCP} what is disabled (by which exact flag token), what is
  not (no switch available), and the verification date/source — the same discipline as the existing
  FR-D5/FR-T9 verification notes. Documentation-only: the Manifest struct gains no new field and the
  FR-C2 verification confirms ZERO bare_flags additions are needed. The notes must name exact flag tokens
  so the sibling test subtask (P1.M1.T2.S1) can assert their presence in BareFlags.

---

## Goal

**Feature Goal**: Make every built-in provider's chrome-disable story complete, honest, and
machine-checkable — per PRD §9.28 FR-C5. Each of the 7 providers (pi, claude, agy, qwen-code, opencode,
codex, cursor) gets a `CHROME-DISABLE` note in its doc-comment recording, per chrome surface, what is
disabled (by which flag), what is not (no CLI switch exists → documented limitation, not an assumption),
and the verification source/date.

**Deliverable**: 7 new `CHROME-DISABLE (FR-C5, §9.28):` paragraphs appended to the doc-comment blocks
in `internal/provider/builtin.go` (one per builtinXxx function). No Manifest struct change, no
bare_flags change (the FR-C2 verification confirms every chrome-disable flag the CLIs expose is already
set). Each note names exact flag tokens for the surfaces it claims disabled.

**Success Definition**:
- All 7 provider doc-comments in builtin.go contain a grep-able `CHROME-DISABLE` paragraph.
- pi's note records the 4 chrome surfaces disabled (--no-extensions/--no-skills/--no-prompt-templates/
  --no-context-files) AND the MCP gap (no --no-mcp; FR-C3 documented limitation).
- claude's note records chrome covered via --tools "" + --setting-sources "".
- The 5 read-only providers (agy, qwen-code, opencode, codex, cursor) each note "no per-surface chrome
  switch exists; the read-only constraint (named flag) is mutation-safety, NOT chrome; chrome may load —
  documented limitation (FR-C4)" with the verification date/source.
- `go build ./...`, `go vet ./...`, `go test ./internal/provider/...`, `make test`, `make lint` all
  pass (the change is comment-only; existing tests are untouched and must still pass).
- Each note is structured so P1.M1.T2.S1 can assert the named flag tokens are present in BareFlags.

## Why

- **FR-C1–C5 / §9.28**: A provider may still discover/load agent chrome (skills, extensions/prompt-
  templates, AGENTS.md/CLAUDE.md context files, MCP servers) around a one-shot commit-message call even
  when it cannot mutate the repo. That chrome is pure overhead (MCP subprocess startup latency, token/
  quota cost from skill/context injection, nondeterminism). FR-C5 requires each provider's manifest
  header to record, per surface, what is disabled vs. what is a documented limitation — never a hidden
  assumption. This subtask produces that record in code (builtin.go), the single source of truth for the
  manifests.
- **Why documentation, not code**: the prior research (`plan/016_*/architecture/external_deps.md`,
  cross-checked against `plan/001_*/architecture/external_deps.md`) already confirmed pi and claude set
  every chrome-disable flag their CLIs expose, and the 5 read-only-constrained providers expose no
  per-surface chrome switches at all. So the work is making the existing-but-implicit story explicit and
  tracked — exactly what FR-C5 asks for. Inventing flags that do not exist is explicitly forbidden (FR-C4).

## What

**User-visible behavior**: None (internal doc-comment change). The manifests' rendered commands are
byte-unchanged; no flag is added or removed.

**Technical change (comment-only; verify, don't add flags):**
1. For each of the 7 `builtinXxx()` functions, append a `CHROME-DISABLE (FR-C5, §9.28):` paragraph to
   its doc-comment block, recording per-surface disable status + verification source.
2. Re-verify FR-C2: confirm no provider exposes a chrome-disable switch currently missing from
   bare_flags. Expected outcome (confirmed by research): zero additions. If a missed flag IS found, add
   it; otherwise document only.

### Success Criteria
- [ ] All 7 provider doc-comments contain a `CHROME-DISABLE` paragraph
- [ ] pi's note names --no-extensions/--no-skills/--no-prompt-templates/--no-context-files AND the MCP gap (FR-C3)
- [ ] claude's note names --tools "" + --setting-sources "" (chrome covered)
- [ ] agy/qwen-code/opencode/codex/cursor each state "no per-surface chrome switch; read-only constraint only; documented limitation (FR-C4)" + the read-only flag + verification date
- [ ] NO bare_flags added (unless FR-C2 verification finds one — expected: none)
- [ ] `go build ./...`, `make test`, `make lint` pass; existing provider tests unchanged

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact per-provider note content (surfaces, flag tokens, gaps, verification dates) is enumerated below, the doc-comment placement lines are given, the test pattern that will consume the notes is identified, and the FR-C2 verification outcome (zero additions) is pre-confirmed by two catalogs.

### Documentation & References

```yaml
- file: internal/provider/builtin.go
  why: "THE change site. 7 functions: builtinPi(42), builtinClaude(111), builtinAgy(198), builtinQwenCode(246), builtinOpenCode(285), builtinCodex(332), builtinCursor(370). Each has a doc-comment block before the func; append the CHROME-DISABLE paragraph there."
  pattern: "Existing doc-comments already carry verification notes ('VERIFIED vs `pi --help`', 'VERIFIED 2026-07-08, agy v1.1.0', '# TO CONFIRM per FR-D5'). The CHROME-DISABLE note uses the SAME discipline (date/source). Use // line-comments consistent with the block."
  gotcha: "The first line of each doc block starts with the function name (godoc convention) — appending a LATER paragraph does not break that. Use a recognizable marker '// CHROME-DISABLE (FR-C5, §9.28):' so it is grep-able and P1.M1.T1.S2 can mirror it into providers/*.toml."

- file: internal/provider/builtin.go (the verified bare_flags per provider)
  why: "The exact flag tokens the notes must reference. pi: --no-tools/--no-extensions/--no-skills/--no-prompt-templates/--no-context-files/--no-session. claude: --tools ''/--setting-sources ''/--no-session-persistence. agy: --mode plan. qwen-code: --approval-mode default. opencode: [] (empty). codex: --sandbox read-only/--ephemeral. cursor: --mode ask/--trust."
  gotcha: "Name the EXACT token for each surface the note claims disabled (e.g. 'extensions: disabled by --no-extensions') so T2.S1 can assert slices.Contains(BareFlags, token). For read-only providers, the note asserts NOTHING is chrome-disabled — only the read-only constraint flag (already tested) applies."

- file: internal/provider/manifest.go
  why: "Confirms there is NO ChromeDisable field — chrome-disable is expressed via bare_flags + the doc-comment note (this subtask). BareFlags is []string at line 70."
  gotcha: "Do NOT add a ChromeDisable struct field. FR-C5 is satisfied by the note + existing bare_flags, not a schema change."

- file: internal/provider/builtin_test.go
  why: "The test pattern T2.S1 will extend. Existing tests use reflect.DeepEqual(m.BareFlags, wantBare) (exact) and assert specific tokens. T2.S1 will add chrome-contract assertions checking the flag tokens the notes claim are PRESENT in BareFlags."
  pattern: "TestBuiltinManifests_PiFields (238) / ClaudeFields (300) assert exact BareFlags. The chrome-contract test (T2.S1) will check presence, e.g. that pi's BareFlags contains --no-extensions/--no-skills/--no-prompt-templates/--no-context-files."

- docfile: plan/016_e6bf7715e06e/architecture/external_deps.md
  why: "The complete per-provider chrome surface inventory (the source of truth for each note's content) + the FR-C2 verification (zero flag additions)."
  section: "Per-provider chrome surface inventory; Summary table"

- docfile: plan/001_f1f80943ac34/architecture/external_deps.md
  why: "The original --help catalog the task says to cross-check for FR-C2 (any chrome switch unset in bare_flags?). Confirms pi's full flag set + absence of --no-mcp; surfaces no chrome switches on the 5 read-only providers."
  section: "pi; claude rows"

- docfile: plan/016_e6bf7715e06e/P1M1T1S1/research/verification_deltas.md
  why: "The verified bare_flags table, the doc-comment line numbers, the FR-C2 zero-additions conclusion, and the note-content spec per provider. READ THIS before editing."
```

### Current Codebase tree (relevant slice)

```bash
internal/provider/
  builtin.go        # 7 builtinXxx() funcs — append CHROME-DISABLE note to each doc block (THE change)
  manifest.go       # Manifest struct; BareFlags []string (:70); NO ChromeDisable field (do NOT add one)
  builtin_test.go   # existing tests (PiFields/ClaudeFields etc.) — UNCHANGED in S1; T2.S1 adds chrome assertions
providers/*.toml    # reference files — S2 mirrors the notes here (NOT S1)
docs/providers.md   # P1.M2.T1 adds the Chrome-disable column (NOT S1)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (documentation-only): this subtask adds Go doc-comments ONLY. Do NOT add a Manifest struct
//   field, do NOT change any bare_flags value, do NOT alter rendered commands. The FR-C2 verification
//   (two catalogs) confirms every chrome-disable flag the CLIs expose is already set. If you "helpfully"
//   add a flag (e.g. a guessed --no-mcp), it does not exist and will break the rendered command — FR-C4
//   explicitly forbids inventing flags.

// GOTCHA (note must be machine-checkable): each "surface X disabled by flag Y" claim must name the
//   EXACT flag token in bare_flags, so T2.S1 can assert presence. For pi: extensions→--no-extensions,
//   skills→--no-skills, prompt-templates→--no-prompt-templates, context-files→--no-context-files.
//   For the 5 read-only providers: assert NOTHING chrome is disabled — the note states the limitation.

// GOTCHA (godoc): the first line of each doc-comment already starts with the function name. APPEND the
//   CHROME-DISABLE paragraph as a later block (leave the opening line intact). Use '// ' line comments.

// GOTCHA (marker line): start each note with '// CHROME-DISABLE (FR-C5, §9.28):' so it is grep-able,
//   mirrors cleanly into providers/*.toml (S2), and is locatable by the docs subtask (P1.M2.T1).

// GOTCHA (MCP is the subtle one — pi): pi has NO --no-mcp flag. --no-tools suppresses MCP tool USE, but
//   servers configured in user settings may still be discovered/connected at startup. This is a
//   documented, tracked LIMITATION (FR-C3), NOT an assumption that MCP is off. State it plainly.
```

## Implementation Blueprint

### Data models and structure

No struct/type changes. Pure doc-comment additions. Chrome-disable remains expressed via the existing
`BareFlags` field + the new notes.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY FR-C2 (re-check the catalogs for any missed chrome-disable flag)
  - RE-READ plan/016_*/architecture/external_deps.md (Summary table) + plan/001_*/architecture/external_deps.md
    (pi/claude rows + the read-only providers).
  - CONFIRM: every chrome-disable switch the CLIs expose is already in bare_flags. Expected outcome:
    ZERO additions (pi sets all 4 chrome flags + --no-tools; claude sets --tools ""/--setting-sources "";
    the 5 read-only providers expose no per-surface chrome switches).
  - IF (unexpected) a missed flag is found: add it to the provider's BareFlags. (Not expected; do not invent.)
  - DEPENDENCIES: none.

Task 2: ADD CHROME-DISABLE note to builtinPi (doc block lines 30-41, before func at 42)
  - APPEND a paragraph to the doc-comment block. Content (use the marker line):
        //
        // CHROME-DISABLE (FR-C5, §9.28): verified vs `pi --help` (external_deps.md §pi, 2026-06-29).
        //   extensions       — disabled by --no-extensions (bare_flags)
        //   skills           — disabled by --no-skills (bare_flags)
        //   prompt-templates — disabled by --no-prompt-templates (bare_flags)
        //   context files    — disabled by --no-context-files (AGENTS.md/CLAUDE.md; bare_flags)
        //   MCP servers      — NOT disabled: pi has NO --no-mcp flag (only --mcp-config <path>).
        //                       --no-tools suppresses MCP tool USE, but configured servers may still be
        //                       discovered/connected at startup. Documented, tracked LIMITATION (FR-C3),
        //                       never an assumption that MCP is off.
  - DEPENDENCIES: Task 1 (confirm no --no-mcp exists; do NOT add one).

Task 3: ADD CHROME-DISABLE note to builtinClaude (doc block 102-110, before func at 111)
  - APPEND:
        //
        // CHROME-DISABLE (FR-C5, §9.28): verified vs `claude --help` (external_deps.md §claude). Chrome
        //   is COVERED via two mechanisms: --tools "" disables ALL built-in tools (MCP surfaces as tools),
        //   and --setting-sources "" blocks the settings files where MCP servers, skills, and extensions
        //   are configured. Both are in bare_flags. No per-surface gap.
  - DEPENDENCIES: Task 1.

Task 4: ADD CHROME-DISABLE note to builtinAgy (doc block 154-197, before func at 198)
  - APPEND:
        //
        // CHROME-DISABLE (FR-C5, §9.28): verified vs `agy --help` (agy v1.1.0, 2026-07-08). agy exposes
        //   NO per-surface chrome-disable switch for skills/extensions/context-files/MCP. --mode plan
        //   (bare_flags) is the read-only, never-ask CONSTRAINT (mutation safety, §12.7.1) — it is NOT a
        //   chrome substitute. Chrome MAY load; the call stays read-only and never-mutate. Documented
        //   LIMITATION (FR-C4), not an assumption. Re-check at the next agy --help re-verification.
  - DEPENDENCIES: Task 1.

Task 5: ADD CHROME-DISABLE note to builtinQwenCode (doc block 221-245, before func at 246)
  - APPEND:
        //
        // CHROME-DISABLE (FR-C5, §9.28): flag surface assembled from docs (NOT yet --help-verified; # TO
        //   CONFIRM per FR-D5). qwen-code exposes NO known per-surface chrome-disable switch.
        //   --approval-mode default (bare_flags) is the read-only CONSTRAINT, not chrome. Chrome surface
        //   is unverified — documented LIMITATION (FR-C4). Re-verify at the FR-D5 token refresh (S2).
  - DEPENDENCIES: Task 1.

Task 6: ADD CHROME-DISABLE note to builtinOpenCode (doc block 268-284, before func at 285)
  - APPEND:
        //
        // CHROME-DISABLE (FR-C5, §9.28): verified vs `opencode run --help` (external_deps.md §opencode,
        //   opencode 1.1.23, 2026-07-08). The `run` subcommand is inherently read-only by design and
        //   exposes NO per-surface chrome-disable switch. bare_flags is empty because `run` is already a
        //   read-only one-shot — that is mutation safety, NOT chrome. Chrome MAY load; the call stays
        //   read-only. Documented LIMITATION (FR-C4).
  - DEPENDENCIES: Task 1.

Task 7: ADD CHROME-DISABLE note to builtinCodex (doc block 305-331, before func at 332)
  - APPEND:
        //
        // CHROME-DISABLE (FR-C5, §9.28): verified vs `codex exec --help` (external_deps.md §codex,
        //   codex-cli 0.143.0, 2026-07-08). codex exec exposes NO per-surface chrome-disable switch for
        //   MCP/AGENTS.md/skills. --sandbox read-only + --ephemeral (bare_flags) are the read-only,
        //   session-clean CONSTRAINT (mutation safety, §12.7.1), NOT chrome. Chrome MAY load; the call
        //   stays read-only and never-mutate. Documented LIMITATION (FR-C4).
  - DEPENDENCIES: Task 1.

Task 8: ADD CHROME-DISABLE note to builtinCursor (doc block 354-369, before func at 370)
  - APPEND:
        //
        // CHROME-DISABLE (FR-C5, §9.28): verified vs `agent --help` (external_deps.md §cursor). cursor
        //   exposes NO per-surface chrome-disable switch. --mode ask + --trust (bare_flags) are the
        //   read-only Q&A CONSTRAINT (mutation safety, §12.7.1), NOT chrome. Chrome MAY load; the call
        //   stays read-only. Documented LIMITATION (FR-C4).
  - DEPENDENCIES: Task 1.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the CHROME-DISABLE note (append to each provider's doc-comment block, before the func)
//   - Starts with the grep-able marker '// CHROME-DISABLE (FR-C5, §9.28):'
//   - Names the verification source + date (same discipline as the existing FR-D5/FR-T9 notes)
//   - For pi/claude: lists each disabled surface → its EXACT flag token (machine-checkable by T2.S1)
//   - For the MCP gap (pi) / read-only providers: states the LIMITATION plainly (FR-C3/FR-C4), never an assumption
//   - Uses '// ' line comments consistent with the surrounding block

// PATTERN: machine-checkability (what T2.S1 will assert)
//   pi note claims "--no-extensions disables extensions" → T2.S1 asserts BareFlags contains "--no-extensions"
//   claude note claims "--tools \"\" disables all tools" → T2.S1 asserts BareFlags contains "--tools" (and the "" value token)
//   read-only providers assert NOTHING chrome-disabled — only the read-only constraint flag (already tested)
```

### Integration Points

```yaml
NO struct / flag / rendered-command / public-API changes. Comment-only.

CODE:
  - internal/provider/builtin.go — +7 CHROME-DISABLE paragraphs (one per builtinXxx doc block)

DOWNSTREAM (consumes these notes — do NOT implement in S1):
  - P1.M1.T1.S2: mirror the notes into providers/*.toml reference files (TOML comments)
  - P1.M1.T2.S1: add chrome-disable contract assertions to builtin_test.go (asserts the named flag tokens)
  - P1.M2.T1.* : docs/providers.md Chrome-disable column + docs/how-it-works.md + README.md

UNCHANGED: Manifest struct; all 7 BareFlags values; rendered commands; existing tests.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (comment changes must not break compilation — they won't, but verify)
go build ./...
go vet ./...

# Format (doc-comments are Go comments; gofmt won't reformat // blocks, but check nothing else drifted)
gofmt -l internal/provider/
# Expected: empty.

# Lint (golangci-lint may have godoc/comment linters — verify the note style passes)
make lint
# Expected: zero errors. If a godoc linter complains about the appended paragraph, adjust spacing to match
#           the existing multi-paragraph doc blocks (which already pass lint).
```

### Level 2: Unit Tests (Component Validation)

```bash
# Provider package — existing tests must pass UNCHANGED (no code change in S1)
go test ./internal/provider/... -v
# Expected: all pass (PiFields/ClaudeFields/etc. assert exact BareFlags; S1 changes no BareFlags).

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Rendered commands are byte-unchanged — prove it with the existing render tests:
go test ./internal/provider/... -run 'RenderedCommand' -v
# Expected: pass (no flag added/removed).

# (No binary behavior change to smoke-test; the change is internal documentation.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: prove all 7 providers got a CHROME-DISABLE note
grep -c "CHROME-DISABLE (FR-C5" internal/provider/builtin.go
# Expected: 7

# Grep guard: prove each provider's note is present
for p in builtinPi builtinClaude builtinAgy builtinQwenCode builtinOpenCode builtinCodex builtinCursor; do
  # the note sits in the doc block above each func — verify by counting markers (7 total) and that
  # each func still exists
  :
done
grep -n "func builtin" internal/provider/builtin.go | wc -l   # Expected: 7

# Grep guard: pi's note records the MCP gap (FR-C3) — the one real limitation
grep -n "no-mcp\|NO --no-mcp\|FR-C3" internal/provider/builtin.go
# Expected: pi's CHROME-DISABLE note mentions the MCP gap.

# Scope-boundary guard: NO bare_flags values changed (comment-only diff)
git diff internal/provider/builtin.go | grep -E '^\+' | grep -E '"--' | grep -vE 'CHROME-DISABLE|FR-C|MCP|extensions|skills|plan|sandbox|mode|tools|approval|trust|context|session|prompt|ephemeral'
# Expected: empty (the only added lines referencing flags are inside the note comments, prefixed with //).
# Simpler check: the diff should be ALL comment lines (// ...) — no non-comment line should change.
git diff internal/provider/builtin.go | grep -E '^\+[^/+]' | grep -v '^\+\+\+'
# Expected: empty (every added line starts with // or is blank).

# Scope-boundary guard: no ChromeDisable struct field added
grep -n "ChromeDisable" internal/provider/manifest.go
# Expected: empty.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/provider/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass — existing provider tests unchanged

### Feature Validation
- [ ] All 7 provider doc-comments contain a `CHROME-DISABLE (FR-C5, §9.28):` paragraph
- [ ] pi's note names the 4 chrome-disable flags + the MCP gap (FR-C3, no --no-mcp)
- [ ] claude's note names --tools "" + --setting-sources "" (chrome covered)
- [ ] agy/qwen-code/opencode/codex/cursor each state "no per-surface chrome switch; read-only constraint only; documented limitation (FR-C4)" + the read-only flag + verification date
- [ ] Each "disabled by" claim names the EXACT flag token (consumable by T2.S1)

### Scope-Boundary Validation
- [ ] NO Manifest struct field added (no ChromeDisable)
- [ ] NO bare_flags value changed (comment-only diff — every added line is `//` or blank)
- [ ] NO providers/*.toml change (that's S2)
- [ ] NO builtin_test.go change (that's T2.S1)
- [ ] NO docs/providers.md / README.md change (that's P1.M2.T1)

### Code Quality
- [ ] Notes use the marker line `// CHROME-DISABLE (FR-C5, §9.28):` (grep-able, mirror-able)
- [ ] Verification source + date on each note (FR-D5/FR-T9 discipline)
- [ ] Limitations stated plainly as limitations (FR-C3/FR-C4), never as assumptions

---

## Anti-Patterns to Avoid

- ❌ Don't add a `ChromeDisable` field to the Manifest struct — FR-C5 is satisfied by the note + existing bare_flags, not a schema change.
- ❌ Don't add or invent flags (e.g. a guessed `--no-mcp`) — the FR-C2 verification confirms zero additions; FR-C4 explicitly forbids inventing flags. pi has NO --no-mcp (that's the documented limitation, FR-C3).
- ❌ Don't change any `bare_flags` value — the diff must be comment-only. Existing render tests pin exact BareFlags; any change breaks them and is out of scope.
- ❌ Don't touch providers/*.toml (S2), builtin_test.go (T2.S1), or docs/*.md (P1.M2.T1) — S1 is builtin.go doc-comments only.
- ❌ Don't write the note as a single run-on line — use the per-surface `//   surface — status` shape so T2.S1 can parse the flag tokens and S2 can mirror it cleanly.
- ❌ Don't omit the verification source/date — FR-C5 requires "the same discipline as FR-D5/FR-T9" (each existing note carries a date/source).
- ❌ Don't phrase the read-only constraint as if it disables chrome — FR-C4 is explicit that mutation safety is NOT a chrome substitute. State "chrome may load; the call stays read-only" plainly.
- ❌ Don't reword the pi MCP gap vaguely — it is the ONE real gap; name it (no --no-mcp; --no-tools suppresses use not discovery) and tag it FR-C3.

---

## Confidence Score: 9/10

One-pass success is very high: the task is comment-only with fully-specified per-provider content (the
external_deps.md inventory + the task description give every note's exact wording), the FR-C2 verification
is pre-confirmed (zero flag additions), and there are no compile/test risks (comments don't affect
behavior). The -1 is for the small risk that a golangci-lint godoc/comment linter objects to the appended
paragraph style — mitigated by matching the existing multi-paragraph doc blocks (which already pass lint)
and the Level-1 `make lint` gate. The other minor risk is an implementer "helpfully" adding a flag —
explicitly forbidden and guarded by the Level-4 comment-only-diff grep check.
