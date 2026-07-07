---
name: "P2.M1.T3.S1 — BuildStagerTask files param + files block (omit when empty) + guardrails wording + stager.go call-site + tests"
description: |

  Implement PRD §17.6 / §9.14 FR-M5: extend `BuildStagerTask` to take the concept's `files []string`
  and render a `Files for this concept (where these changes live):` guidance block (omitted ENTIRELY
  when `files` is nil/empty — no blank-line artifact), and update the `stagerGuardrails` second sentence
  to the new §17.6 wording (preserve the em-dash U+2014 and the two backtick commands). Wire the single
  call-site in `decompose/stager.go`. The files list is GUIDANCE (where the concept's changes live), NOT a
  hard constraint — FR-M1c (content ⊆ T_start) remains the sole content guarantee.

  CONTRACT (P2.M1.T3.S1, verbatim):
    1. RESEARCH NOTE: See planner_prompt.md §2.4. BuildStagerTask(title, description string) at
       prompt/stager.go:60; stagerGuardrails const at stager.go:31-35 (second sentence must change wording;
       the em-dash U+2014 in 'file contents — only update the index' is preserved per §17.6 fidelity; the two
       backtick commands unchanged). Sole call-site: decompose/stager.go:99
       `task := prompt.BuildStagerTask(concept.Title, concept.Description)`. The deps.stager test seam
       (decompose/roles.go:73) takes the WHOLE PlannerCommit so it is transparent to the signature change.
    2. INPUT: concept.Files from P2.M1.T1.S1.
    3. LOGIC: Implement §17.6/FR-M5 files block. (a) BuildStagerTask signature: add `files []string`.
       (b) Topology: stagerInstruction + blank + title + '\n' + description + [if len(files)>0: blank +
       'Files for this concept (where these changes live):\n' + strings.Join(files, '\n')] + blank +
       stagerGuardrails. Omit the files block ENTIRELY when files is nil/empty (no blank-line artifact).
       (c) Update stagerGuardrails wording: change 'Stage ONLY changes belonging to this concept; leave
       unrelated changes unstaged.' to 'Stage ONLY the changes the description assigns to this concept
       (the files above are where they live); leave everything else unstaged.' (verbatim PRD §17.6 line
       1826); preserve the em-dash and the two backtick commands. (d) Update decompose/stager.go:99 to pass
       concept.Files. Mock: none (pure string tests). TDD: update TestBuildStagerTask_CanonicalExact (two
       cases: files=[a.go,b.go] block present; files=nil block absent — byte-identity minus block); add
       TestBuildStagerTask_FilesBlock_OmittedWhenEmpty (nil and [] must NOT contain 'Files for this concept');
       update TestBuildStagerTask_Properties (replace the 'Stage ONLY changes belonging to this' needle with
       the new wording; add 'Files for this concept (where these changes live):' PRESENT-with-files; keep
       em-dash/backtick/anti-copy assertions); update edge-case tests to pass the new files arg (nil).
    4. OUTPUT: A stager task that surfaces the concept's files as guidance for consumption by the tooled
       stager agent; empty files omit the block cleanly.
    5. DOCS: none — internal prompt construction (no user-facing surface; §17.6 narrative is in PRD already).

  ⚠️ §1 — EM-DASH IS PRESERVED (the OPPOSITE of the planner task). The stager file ALREADY uses the real
  em-dash "—" (U+2014) and the comment at stager.go:25-26 mandates: "Do NOT replace with an ASCII hyphen
  (verbatim §17.6 fidelity)." (Compare: planner.go mandates ASCII-only and substitutes " -- ".) The NEW
  guardrails wording has EXACTLY ONE em-dash — in "file contents — only update the index" — same site as
  before, so the file-top comment "§17.6 is ENTIRELY ASCII EXCEPT one em-dash" stays TRUE verbatim. Do NOT
  ASCII-substitute. Every `want` string below keeps the U+2014 byte.

  ⚠️ §2 — OMIT THE BLOCK CLEANLY (no blank-line artifact). When `len(files)==0` (nil OR empty slice), the
  entire files block — INCLUDING its leading "\n\n" — is omitted. The output for files==nil MUST be
  byte-identical to the PRE-files-era output: `...description + "\n\n" + stagerGuardrails`. The single most
  common bug here is emitting a stray "\n\n" before the guardrails when files is empty — guard against it with
  the dedicated TestBuildStagerTask_FilesBlock_OmittedWhenEmpty.

  ⚠️ §3 — THE GUARDRAILS LINE-WRAPPING CHANGES (not just the sentence). PRD §17.6 re-wraps the WHOLE
  5-line guardrails block. Do NOT surgically swap only the second sentence on its old line boundaries — the
  new wording is longer and the visual wrap (the `\n` placement inside the const) shifts. Replace the ENTIRE
  `stagerGuardrails` const with the verbatim 5 lines in §"The stagerGuardrails const" below.

  ⚠️ §4 — THE TWO-BACKTICK CONSTRAINT (why stagerGuardrails is double-quoted, not a backtick raw string).
  The const contains TWO backtick chars (`git add <path>` and `git apply --cached`); a Go backtick raw
  string literal cannot contain backticks, so the const MUST stay a series of double-quoted "..." lines
  joined by `+ "\n"` (the existing style at stager.go:31-35). Keep that literal style.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `internal/prompt/planner.go` + `planner_test.go` → P2.M1.T2.S1 (COMPLETE). UNCHANGED.
    - `internal/decompose/decompose.go` + `decompose_test.go` → P2.M1.T1.S2 (COMPLETE). UNCHANGED.
    - `internal/decompose/planner.go` + `planner_test.go` → P2.M1.T2.S1 (COMPLETE). UNCHANGED.
    - `internal/decompose/stager_test.go` → TRANSPARENT (constructs `PlannerCommit{Title,Description}`
      without Files — defaults to nil — and passes the WHOLE struct to stageConcept; it NEVER calls
      BuildStagerTask directly). ZERO edits; stays green. (The deps.stager test seam at roles.go:73 also
      takes the whole PlannerCommit.)
    - `PlannerCommit.Files` + `ParsePlannerOutput` → COMPLETE (P2.M1.T1.S1). This task only READS Files.
      Do NOT re-add the field or change parse code.
    - `docs/how-it-works.md` → P2.M1.T2.S2 (the Mode-A stager-narrative doc edit rides with T2). Item §5:
      NO DOCS here.
    - `internal/config/*`, `cmd/stagecoach/*`, `docs/cli.md`, `docs/configuration.md` → no new flags/keys.
    - `internal/prompt/reserve.go` → the stager has NO reserve token calc (stagerReserveTokens does not
      exist; only planner/message do). reserve.go UNCHANGED; reserve_test.go UNCHANGED.

  DELIVERABLES (0 NEW files, 3 EDITED files):
    EDIT internal/prompt/stager.go          — (a) add `files []string` to BuildStagerTask signature;
                                              (b) add the conditional files block to the topology;
                                              (c) replace the stagerGuardrails const (new §17.6 wording,
                                              em-dash preserved); (d) optionally add a stagerFilesHeader
                                              const for the block header (mirrors the named-const convention).
    EDIT internal/prompt/stager_test.go     — UPDATE CanonicalExact (TWO cases: files present / nil absent);
                                              ADD FilesBlock_OmittedWhenEmpty (nil + []); UPDATE Properties
                                              (new guardrails needle + files-header PRESENT-with-files;
                                              keep em-dash/backtick/anti-copy); UPDATE EdgeCases (pass nil).
    EDIT internal/decompose/stager.go       — line 99: pass `concept.Files` as the new third arg.

  SUCCESS: BuildStagerTask(title, desc, files) renders stagerInstruction + blank + title + "\n" + desc +
  [if len(files)>0: blank + "Files for this concept (where these changes live):\n" + Join(files,"\n")] +
  blank + stagerGuardrails; files==nil/[] ⇒ byte-identical to the pre-files output (block + its leading
  "\n\n" both omitted); the guardrails second sentence is the new §17.6 wording; the em-dash U+2014 and the
  two backtick commands are preserved; decompose/stager.go:99 passes concept.Files; the test seam and
  decompose/stager_test.go are untouched (transparent); `go build/vet/test ./...` green; go.mod/go.sum
  unchanged; EXACTLY 3 files change.

---

## Goal

**Feature Goal**: Implement PRD §17.6 (FR-M5 stager task prompt) — surface the planner's per-concept
`files` list to the tooled stager agent as a guidance block ("where these changes live"), and adopt the
updated §17.6 guardrails wording. `BuildStagerTask` gains a `files []string` parameter; a non-empty list
inserts a `Files for this concept (where these changes live):` block between the description and the
guardrails; an empty/nil list omits the block ENTIRELY (byte-identical to the legacy no-files output). The
files list is GUIDANCE only — FR-M1c (content ⊆ T_start) remains the sole content guarantee.

**Deliverable** (0 NEW + 3 EDITED):
1. **EDIT `internal/prompt/stager.go`** — `BuildStagerTask(title, description string, files []string) string`;
   new conditional files-block topology; replace `stagerGuardrails` const (new §17.6 wording; em-dash +
   backticks preserved); add `stagerFilesHeader` const (mirrors named-const convention).
2. **EDIT `internal/prompt/stager_test.go`** — `CanonicalExact` (two cases: `["a.go","b.go"]` present / nil
   absent); `+FilesBlock_OmittedWhenEmpty` (nil + `[]string{}`); `Properties` (new guardrails needle +
   files-header present-with-files; keep em-dash/backtick/anti-copy); `EdgeCases` (pass nil).
3. **EDIT `internal/decompose/stager.go`** — line 99: `prompt.BuildStagerTask(concept.Title, concept.Description, concept.Files)`.

**Success Definition**:
- `BuildStagerTask("T", "D", []string{"a.go","b.go"})` ⇒ `stagerInstruction + "\n\nT\nD\n\n" +
  stagerFilesHeader + "\n" + "a.go\nb.go" + "\n\n" + stagerGuardrails`.
- `BuildStagerTask("T", "D", nil)` AND `BuildStagerTask("T", "D", []string{})` ⇒ `stagerInstruction +
  "\n\nT\nD\n\n" + stagerGuardrails` (byte-identical to each other AND to the pre-files-era output — NO
  stray "\n\n" before the guardrails).
- The new `stagerGuardrails` const contains the verbatim §17.6 wording: `"Stage ONLY the changes the
  description\nassigns to this concept (the files above are where they live); leave everything else
  unstaged."` AND preserves the em-dash `"contents — only"` (U+2014, NOT ASCII hyphen) AND the two backtick
  commands `` `git add <path>` `` / `` `git apply --cached` `` AND the literal `<path>` token.
- `decompose/stager.go:99` passes `concept.Files`; the `deps.stager` test seam (roles.go:73) and
  `decompose/stager_test.go` are UNCHANGED (they thread the whole `PlannerCommit`, transparent to the
  signature change).
- `go build ./... && go vet ./... && go test ./...` GREEN; go.mod/go.sum unchanged; EXACTLY 3 files change.

## User Persona

**Target User**: a developer running `stagecoach` auto-decompose on a mixed working tree. The planner has
already partitioned the changes into concepts and declared, per concept, which files it touches (FR-M3,
P2.M1.T1.S1). The tooled stager agent (one invocation per concept) now receives that file list as guidance
so it knows WHERE to look — it still resolves the exact hunks mechanically and FR-M1c is the hard content
guarantee.

**Use Case**: a concept titled "Refactor auth middleware" with `files: ["internal/auth/middleware.go",
"internal/api/handlers.go"]`. The stager task now says, right after the description: "Files for this concept
(where these changes live):\ninternal/auth/middleware.go\ninternal/api/handlers.go" — guiding the agent to
the right paths without over-constraining it (a single file split across two concepts can still be named in
both; the description says which part belongs where).

**Pain Points Addressed**: today the stager receives only the concept's title + description with no file
hint, so it must re-derive where the changes live from the full working-tree diff. The planner ALREADY
computed the per-concept file partition (FR-M3) — surfacing it as guidance makes staging faster and more
precise (the planner's `files` real job is "telling each concept's stager where to look", FR-M3b). When the
planner omits files (empty/nil), the block vanishes cleanly so no stale/empty guidance leaks into the prompt.

## Why

- **Closes PRD §17.6 / FR-M5 at the prompt layer.** §17.6: "It receives one concept's title + description +
  files (from the planner, §17.5) as a task." Today `BuildStagerTask` takes only title + description.
- **Surfaces the planner's file partition as guidance.** FR-M3b: "`files`' real job is telling each
  concept's stager where to look (FR-M5)." The planner already computed it; this task delivers it.
- **Guidance, not a constraint (the FR-M1c invariant is untouched).** §17.6: "The `files` list is guidance
  (where the concept's changes live), not a hard constraint — FR-M1c (content ⊆ `T_start`) remains the sole
  content guarantee; an empty list simply omits the files block." This task adds ZERO freeze/validation
  logic — the freeze enforcement (ErrFreezeViolation, verifyFreezeSubset, P1) is unchanged and remains the
  sole content guarantee.
- **Updated guardrails wording.** §17.6 rewords the second guardrails sentence to reference the files block
  ("the files above are where they live") so the stager ties the "Stage ONLY … this concept" instruction to
  the surfaced file list. The hard guardrails (no commit/amend/push/ref-mutation) and the structural
  enforcement (tooled_flags; stagecoach owns all ref ops) are unchanged.
- **Clean omission.** §17.6: "an empty list simply omits the files block." No blank-line artifact, no
  empty header — the prompt stays well-formed whether or not files are present.

## What

A pure prompt-construction change: one builder gains a `[]string` param and a conditional block; one const
gets new wording; one call-site gains one arg; the prompt tests are rewritten/added/updated. No behavior
change to staging, the loop, the freeze, parsing, validation, the hard cap, or the test seam. No new types,
no interface change, no import change.

### Success Criteria

- [ ] `BuildStagerTask(title, description string, files []string) string` exists in `internal/prompt/stager.go`
      with the topology: `stagerInstruction + "\n\n" + title + "\n" + description + [len(files)>0 ? "\n\n" +
      stagerFilesHeader + "\n" + strings.Join(files, "\n") : ""] + "\n\n" + stagerGuardrails`.
- [ ] A new `stagerFilesHeader` const equals `"Files for this concept (where these changes live):"` (no
      trailing newline — mirrors the package convention; the builder owns the "\n" after it).
- [ ] `len(files)==0` (nil OR `[]string{}`) ⇒ the files block AND its leading `"\n\n"` are BOTH omitted; the
      output is byte-identical to `stagerInstruction + "\n\n" + title + "\n" + description + "\n\n" +
      stagerGuardrails` (the pre-files-era rendering).
- [ ] The `stagerGuardrails` const is REPLACED with the verbatim 5-line §17.6 wording (see §"The
      stagerGuardrails const"); the second sentence is `"Stage ONLY the changes the description\nassigns to
      this concept (the files above are where they live); leave everything else unstaged."`; the em-dash
      U+2014 in `"contents — only"` is PRESERVED (NOT an ASCII hyphen); the two backtick commands and the
      `<path>` token are unchanged.
- [ ] The file-top comment `// §17.6 is ENTIRELY ASCII EXCEPT one em-dash "—" (U+2014) in the guardrails
      block.` remains TRUE and is preserved (the new wording still has exactly one em-dash).
- [ ] `internal/decompose/stager.go:99` becomes `prompt.BuildStagerTask(concept.Title, concept.Description,
      concept.Files)`; the surrounding `stageConcept` body is byte-unchanged.
- [ ] Tests: `TestBuildStagerTask_CanonicalExact` has TWO cases (files-present block rendered; files=nil
      block absent — byte-identity-minus-block); `TestBuildStagerTask_FilesBlock_OmittedWhenEmpty` ADDED
      (nil + `[]string{}` both assert NOT `strings.Contains(p, "Files for this concept")` and assert the
      two are byte-equal to each other); `TestBuildStagerTask_Properties` needle updated to the new wording
      + files-header PRESENT (with files) + per-path rendering + em-dash/backtick/anti-copy kept;
      `TestBuildStagerTask_EdgeCases` passes nil as the new third arg.
- [ ] `internal/decompose/stager_test.go` UNCHANGED (transparent — constructs `PlannerCommit` without Files,
      defaults nil; threads the whole struct through `stageConcept`).
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; go.mod/go.sum unchanged; EXACTLY 3 files change.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the EXACT new const bytes
(§"The stagerGuardrails const" — copy/paste), the em-dash-preserved rule (⚠️§1 — opposite of the planner),
the clean-omission rule (⚠️§2), the line-wrap-shift gotcha (⚠️§3), the double-quoted-literal constraint
(⚠️§4 — backticks in the const), the exact builder topology (§Implementation Blueprint), the single call-site
(decompose/stager.go:99), the exact test specs (§Validation), and the scope fence (the stager test seam +
decompose/stager_test.go are transparent; planner/decompose.go/docs are out of scope). No git/provider/
freeze/orchestrator knowledge required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE research note (exact new topology + the guardrails wording diff + caller)
- docfile: plan/008_82253c999440/docs/architecture/planner_prompt.md
  section: §2.4 (Stager files block + guardrails wording) — the new topology sketch, the omit-when-empty rule,
       the EXACT old→new guardrails wording diff (line-wrapping shifts), and the caller-update one-liner.
       Also §3.2 (the stager_test.go assertion inventory: CanonicalExact two cases, +FilesBlock_OmittedWhenEmpty,
       Properties needle swap, EdgeCases nil).
  critical: §2.4 quotes BOTH the current guardrails block AND the target verbatim — the line `\n` placement
       DIFFERS (the new wording is longer); replace the WHOLE const, do not swap one sentence in place (⚠️§3).

# MUST READ — PRD §17.6 (the authoritative source for the new wording + the files-block contract)
- docfile: PRD.md
  section: §17.6 (Stager task prompt) — the task-prompt sketch with the `Files for this concept (where these
       changes live):` block, the verbatim guardrails (copy the em-dash verbatim — ⚠️§1), and the
       "empty list simply omits the files block" / "guidance, not a hard constraint" contract.
  critical: the em-dash in "file contents — only update the index" is a U+2014, preserved verbatim (do NOT
       ASCII-substitute — the stager file's top comment already documents this byte as the ONE non-ASCII byte).

# MUST READ — S1's PRP (PlannerCommit.Files — COMPLETE; this task only READS concept.Files)
- docfile: plan/008_82253c999440/P2M1T1S1/PRP.md
  section: the PlannerCommit.Files field (json:"files"; []string; guidance, not a constraint).
  critical: do NOT re-add or redeclare Files (S1 done). decompose/stager.go:99 reads concept.Files (already a
       []string field on PlannerCommit) and passes it straight through.

# MUST READ — the FILE TO EDIT: the builder + the guardrails const
- file: internal/prompt/stager.go   (EDIT)
  section: `stagerGuardrails` const (L31-35 — REPLACE the whole const); `BuildStagerTask` (L60 — add `files
       []string` param + conditional block). `stagerInstruction` (L18) UNCHANGED. Add `stagerFilesHeader`
       const near the others.
  why: the two load-bearing edits live here. The package convention (consts have NO trailing newline; the
       builder owns ALL inter-block `\n` placement — L9-12) is already documented; follow it verbatim. A new
       `stagerFilesHeader` const continues that convention (one auditable block-header token).
  pattern: mirror the EXISTING `stagerInstruction`/`stagerGuardrails` const style — a double-quoted "..."
       literal (NOT a backtick raw string) because the guardrails contain backtick chars (⚠️§4).
  gotcha: the em-dash U+2014 is ALREADY in the file and MUST stay (⚠️§1 — the opposite of planner.go's
       ASCII-only rule). The top comment "§17.6 is ENTIRELY ASCII EXCEPT one em-dash" stays true after the edit.

# MUST READ — the single call-site (transparent to the test seam)
- file: internal/decompose/stager.go   (EDIT — ONE line)
  section: L99 `task := prompt.BuildStagerTask(concept.Title, concept.Description)` → add `, concept.Files`.
  why: `concept` is a `prompt.PlannerCommit` (P2.M1.T1.S1 added the `Files []string` field); `concept.Files`
       is in scope. The `stageConcept` signature (L93) takes the WHOLE `concept prompt.PlannerCommit`, so the
       test seam (`deps.stager` at roles.go:73) and decompose/stager_test.go are transparent — ZERO edits.
  gotcha: do NOT change `stageConcept`'s signature or the `deps.stager` seam type — both take the whole
       PlannerCommit. Only L99 gains one arg.

# MUST READ — the test file to edit (current shape: CanonicalExact single-case; Properties; EdgeCases)
- file: internal/prompt/stager_test.go   (EDIT)
  section: `TestBuildStagerTask_CanonicalExact` (L12 — split into two cases: files=[a.go,b.go] present; files=nil
       absent); `TestBuildStagerTask_Properties` (L36 — swap the "Stage ONLY changes belonging to this\nconcept"
       needle to the new wording; ADD the files-header PRESENT-with-files + per-path assertions; keep em-dash,
       backtick, anti-copy-paste); `TestBuildStagerTask_EdgeCases` (L152 — pass nil as the 3rd arg to all three
       BuildStagerTask calls). ADD `TestBuildStagerTask_FilesBlock_OmittedWhenEmpty`.
  pattern: the existing `want` string in CanonicalExact is built with `+` concatenation and the em-dash byte
       verbatim — copy that style for the new `want` (do NOT ASCII-substitute the em-dash).
  gotcha: the existing Properties assertion `strings.Contains(p, desc+"\n\n"+stagerGuardrails)` (L144) STAYS
       valid ONLY when files==nil/empty (no block between desc and guardrails). Run Properties with files==nil
       for that assertion, and ADD a separate files-present sub-check for the block. The "blank-line topology:
       one blank line after description" sub-test (L143) must use the nil variant too.

# MUST READ — PlannerCommit (the source of concept.Files — READ-ONLY, COMPLETE)
- file: internal/prompt/planner.go   (READ-ONLY)
  section: `type PlannerCommit struct { ...; Files []string \`json:"files"\` ... }` (L82-88).
  why: confirms `concept.Files` is a `[]string` ready to pass through; this task does NOT touch planner.go.
```

### Current Codebase tree (relevant slice)

```bash
internal/prompt/
  stager.go             # EDIT: stagerGuardrails const (L31-35) + BuildStagerTask (L60) + new stagerFilesHeader const
  stager_test.go        # EDIT: CanonicalExact (2 cases) + FilesBlock_OmittedWhenEmpty + Properties + EdgeCases
  planner.go            # READ-ONLY: PlannerCommit.Files source (COMPLETE, P2.M1.T1.S1)
  reserve.go            # UNCHANGED (stager has NO reserve calc)
  reserve_test.go       # UNCHANGED
  format.go, payload.go, system.go  # UNCHANGED
internal/decompose/
  stager.go             # EDIT — L99 call-site (ONE line; stageConcept signature + deps.stager seam unchanged)
  stager_test.go        # READ-ONLY/TRANSPARENT (constructs PlannerCommit w/o Files → nil; threads whole struct)
  roles.go              # READ-ONLY: deps.stager seam takes whole PlannerCommit (L73) — transparent
  planner.go            # FENCE — P2.M1.T2.S1 (COMPLETE)
  decompose.go          # FENCE — P2.M1.T1.S2 (COMPLETE)
docs/how-it-works.md    # FENCE — P2.M1.T2.S2 (Mode-A doc)
```

### Desired Codebase tree with files to be added and responsibility of file

No NEW files. The 3 edited files and their new responsibilities are described in **Deliverable** above and
the Implementation Tasks below.

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (⚠️§1): the stager file PRESERVES the em-dash U+2014 (the OPPOSITE of planner.go). The comment at
// stager.go:25-26 mandates: "Do NOT replace with an ASCII hyphen (verbatim §17.6 fidelity)." The NEW guardrails
// wording has EXACTLY ONE em-dash — in "file contents — only update the index" — the same site as before, so the
// file-top comment "§17.6 is ENTIRELY ASCII EXCEPT one em-dash" stays TRUE verbatim. Every test `want` keeps U+2014.

// CRITICAL (⚠️§2): OMIT THE BLOCK CLEANLY. When len(files)==0, omit the block AND its leading "\n\n". The
// nil/empty output MUST be byte-identical to stagerInstruction + "\n\n" + title + "\n" + description + "\n\n" +
// stagerGuardrails. The classic bug: always emitting "\n\n" before the guardrails → a stray blank line. The
// dedicated TestBuildStagerTask_FilesBlock_OmittedWhenEmpty catches this (assert nil==[]string{}==legacy bytes).

// CRITICAL (⚠️§3): the guardrails LINE-WRAPPING SHIFTS. PRD §17.6 re-wraps the whole 5-line block (the new
// wording is longer). Replace the ENTIRE stagerGuardrails const with the 5 verbatim lines below — do NOT
// surgically swap one sentence on the old line boundaries (the internal "\n" positions move).

// CRITICAL (⚠️§4): stagerGuardrails is a DOUBLE-QUOTED "..." literal series (NOT a backtick raw string) because
// it contains backtick chars (`git add <path>`, `git apply --cached`). A backtick raw string cannot contain
// backticks (compile error). Keep the existing `+ "\n"` join style.

// GOTCHA: the Properties test's `strings.Contains(p, desc+"\n\n"+stagerGuardrails)` (L144) and the "one blank
// line after description" sub-test (L143) are ONLY valid in the nil/empty-files case (no block in between).
// Run Properties with files==nil for those, and add a SEPARATE files-present sub-check for the block topology.

// GOTCHA: the `stagerFilesHeader` const is "Files for this concept (where these changes live):" — note the
// trailing COLON (mirrors stagerInstruction's trailing colon convention). NO trailing newline (package rule);
// the builder emits "\n" then strings.Join(files, "\n") after it.

// GOTCHA: paths are rendered one-per-line via strings.Join(files, "\n") — NOT one-per-blank-line. Two files
// ⇒ "a.go\nb.go" (a single "\n" between them), then "\n\n" before the guardrails.

// NOTE: the deps.stager test seam (decompose/roles.go:73) and decompose/stager_test.go both thread the WHOLE
// prompt.PlannerCommit — they never call BuildStagerTask directly. Adding the `files` param is therefore
// transparent: ZERO edits to either. Do NOT "fix" them.
```

## Implementation Blueprint

### Data models and structure

No data-model change. `PlannerCommit.Files` (S1, COMPLETE) is only READ by this task (decompose/stager.go:99
passes `concept.Files` straight through). `PlannerOutput`, `BuildPlannerUserPayload`, `ParsePlannerOutput`,
`PlannerRetryInstruction`, `stagerInstruction` — all UNCHANGED.

### The stagerGuardrails const — EXACT new bytes (copy/paste into `internal/prompt/stager.go`)

> Transcribe VERBATIM from PRD §17.6. The em-dash "—" (U+2014) in "file contents — only update the index" is
> PRESERVED (⚠️§1 — do NOT ASCII-substitute). The two backtick commands and the `<path>` token are unchanged.
> The const stays a double-quoted `"..."` literal series joined by `+ "\n"` (⚠️§4 — backticks forbid a raw
> string). NO trailing newline (package convention — the builder owns `\n` placement). Source: PRD §17.6
> (selected_prd_content) / architecture §2.4.

```go
// stagerGuardrails is the verbatim §17.6 five-line git-instructions + hard-guardrails block. It is the
// prompt-level restatement of §13.6.2/§17.6's structural guardrails (no commit/amend/push/ref-mutation;
// only update the index), enforced STRUCTURALLY too via tooled_flags (§12.1; stagecoach owns all ref ops).
// The second sentence references the surfaced files block ("the files above are where they live").
//
// NOTE the TWO BACKTICK chars (`git add <path>` and `git apply --cached`) — hence a double-quoted "..."
// literal (a backtick raw string cannot contain backticks; will not compile).
// NOTE the EM-DASH "—" (U+2014) in "file contents — only update the index" — the ONE non-ASCII byte.
// Do NOT replace with an ASCII hyphen (verbatim §17.6 fidelity).
// NOTE the literal `<path>` token inside `git add <path>` is instructive (part of the command example),
// NOT a runtime placeholder. Only <title>/<description>/<files> are placeholders.
// NO trailing newline.
const stagerGuardrails = "Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply\n" +
	"only the relevant hunks via `git apply --cached`). Stage ONLY the changes the description\n" +
	"assigns to this concept (the files above are where they live); leave everything else unstaged.\n" +
	"Do not commit, do not amend, do not push, do not modify file contents — only update the index.\n" +
	"When done, reply with the list of paths you staged and stop."
```

### The stagerFilesHeader const + the new BuildStagerTask topology

```go
// stagerFilesHeader is the verbatim §17.6 files-block header (trailing COLON — mirrors stagerInstruction).
// Rendered ONLY when len(files) > 0 (PRD §17.6: "an empty list simply omits the files block"). NO trailing
// newline (package convention); BuildStagerTask emits "\n" then the joined paths after it.
const stagerFilesHeader = "Files for this concept (where these changes live):"

// BuildStagerTask implements PRD §17.6 / §13.6.2 / FR-M5: assemble the stager task prompt (delivered as
// the user payload; system prompt minimal/empty — not a prompt-package constant). The orchestrator
// (P3.M4.T1.S1) calls this for each concept[i] from the planner's output:
//
//	task := prompt.BuildStagerTask(concept.Title, concept.Description, concept.Files)
//
// The stager returns free-form text ("list of paths staged"); the truth source is the index
// (git diff --cached --name-only), hence NO JSON contract / NO parse — the caller reads the exit code.
// The guardrails are ALSO enforced structurally (tooled_flags §12.1; stagecoach owns all ref ops —
// §17.6's safety proof).
//
// files is GUIDANCE (where the concept's changes live), NOT a hard constraint — FR-M1c (content ⊆ T_start)
// remains the sole content guarantee. An empty/nil list OMITS the files block entirely (no blank-line
// artifact) — PRD §17.6 line 1818.
//
// ASSEMBLY TOPOLOGY (PRD §17.6):
//
//	stagerInstruction       // "...match this concept:" (no trailing \n)
//	'\n' '\n'               // ONE blank line before the concept
//	title                   // the concept's short label (verbatim; single-line per planner §17.5)
//	'\n'                    // title + description on CONSECUTIVE lines (§17.6 — no blank between)
//	description             // the staging instructions (verbatim; may be multi-line)
//	[if len(files) > 0:     // the files block, ONLY when non-empty (omitted cleanly otherwise)
//		'\n' '\n'           // ONE blank line before the files block
//		stagerFilesHeader   // "Files for this concept (where these changes live):" (no trailing \n)
//		'\n'                // header + first path on consecutive lines
//		strings.Join(files, '\n')]  // one path per line (single \n between paths)
//	'\n' '\n'               // ONE blank line before the guardrails
//	stagerGuardrails        // the 5-line git-instructions + hard-guardrails block (no trailing \n)
//
// title, description, and files are interpolated VERBATIM from the planner's PlannerCommit{Title,
// Description, Files}. Defensive: empty title/description/files do not panic (the planner always supplies
// title+description per §17.5; files may legitimately be nil/empty ⇒ block omitted).
func BuildStagerTask(title, description string, files []string) string {
	var b strings.Builder
	b.WriteString(stagerInstruction)
	b.WriteString("\n\n")
	b.WriteString(title)
	b.WriteByte('\n')
	b.WriteString(description)
	if len(files) > 0 {
		b.WriteString("\n\n")
		b.WriteString(stagerFilesHeader)
		b.WriteByte('\n')
		b.WriteString(strings.Join(files, "\n"))
	}
	b.WriteString("\n\n")
	b.WriteString(stagerGuardrails)
	return b.String()
}
```

> NOTE: the current implementation uses plain `+` concatenation (one line). The `strings.Builder` form above
> is clearer for the conditional block but EITHER is acceptable — the BYTES are authoritative. If keeping the
> one-liner style, build the optional block into a local `filesBlock := ""; if len(files) > 0 { filesBlock =
// "\n\n" + stagerFilesHeader + "\n" + strings.Join(files, "\n") }` and splice it before the trailing
> `"\n\n" + stagerGuardrails`. `import "strings"` is ALREADY present (used by nothing today in stager.go, but
> the package imports it elsewhere — confirm `goimports`/`gofmt` is happy; if stager.go does not yet import
> "strings", ADD it).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/prompt/stager.go — guardrails const + header const + builder
  - REPLACE: the stagerGuardrails const (L31-35) with the 5 verbatim §17.6 lines (§"The stagerGuardrails const"
          above). Em-dash U+2014 PRESERVED (⚠️§1); two backticks + <path> token unchanged; double-quoted literal
          style kept (⚠️§4); NO trailing newline.
  - ADD: const stagerFilesHeader = "Files for this concept (where these changes live):" (place it adjacent to
          stagerInstruction/stagerGuardrails; trailing colon; NO trailing newline).
  - REWRITE: BuildStagerTask — signature `func BuildStagerTask(title, description string, files []string)
          string`; topology in §"The stagerFilesHeader const + the new BuildStagerTask topology" above. Omit the
          files block AND its leading "\n\n" when len(files)==0 (⚠️§2). Paths via strings.Join(files, "\n").
  - UPDATE the doc comment: add `files []string` to the signature line in the example comment (L41) and to the
          @param-style prose; note the omit-when-empty + guidance-not-contract semantics. UPDATE the ASSEMBLY
          TOPOLOGY block to include the conditional files block.
  - PRESERVE: stagerInstruction (L18) byte-unchanged; the file-top comment block (L1-12) including the
          "§17.6 is ENTIRELY ASCII EXCEPT one em-dash" invariant (still true). Optionally note the new
          stagerFilesHeader is pure ASCII.
  - DEPENDENCIES: `import "strings"` — ADD to the import block if not present (stager.go currently has NO
          imports; add `import "strings"`). Confirm with gofmt.

Task 2: EDIT internal/decompose/stager.go — the ONE call-site (L99)
  - CHANGE L99 from:
      task := prompt.BuildStagerTask(concept.Title, concept.Description)
    to:
      task := prompt.BuildStagerTask(concept.Title, concept.Description, concept.Files)
  - PRESERVE: the stageConcept signature (L93 — `concept prompt.PlannerCommit`), the deps.stager seam type
          (roles.go:73 — takes the whole PlannerCommit), the Render call, the Execute call, and the freeze/HEAD
          guards — all byte-unchanged.
  - DEPENDENCIES: Task 1 (the new signature). `concept.Files` is already in scope (PlannerCommit.Files, S1).

Task 3: EDIT internal/prompt/stager_test.go — CanonicalExact (2 cases) + add OmittedWhenEmpty + Properties + EdgeCases
  - UPDATE TestBuildStagerTask_CanonicalExact: convert to a table of TWO cases sharing the same (title,description):
      (a) files=[]string{"a.go","b.go"} → want INCLUDES the files block:
          ...description + "\n\n" + "Files for this concept (where these changes live):\n" + "a.go\nb.go" +
          "\n\n" + stagerGuardrails(new).
      (b) files=nil → want EXCLUDES the block (byte-identity to case (a) MINUS the "\n\nFiles for this
          concept (where these changes live):\na.go\nb.go" segment). Assert both cases against independently-
          derived `want` strings (copy/paste the em-dash verbatim — ⚠️§1). The new stagerGuardrails wording is
          in BOTH `want` strings.
  - ADD TestBuildStagerTask_FilesBlock_OmittedWhenEmpty: assert BuildStagerTask(t,d,nil) and
          BuildStagerTask(t,d,[]string{}) BOTH do NOT contain "Files for this concept"; AND assert the two
          outputs are byte-equal to EACH OTHER (==) AND byte-equal to BuildStagerTask(t,d,nil) rendered
          without the block (the legacy shape). This is the ⚠️§2 regression net.
  - UPDATE TestBuildStagerTask_Properties:
      • Run the main `p := BuildStagerTask(title, desc)` with files==NIL (L39 → `BuildStagerTask(title, desc,
        nil)`) so the existing `desc+"\n\n"+stagerGuardrails` (L144) and "one blank line after description"
        (L143) assertions still hold (no block between desc and guardrails when files is nil).
      • REPLACE the needle `{"guardrails: Stage ONLY clause present", "Stage ONLY changes belonging to
        this\nconcept", true}` with `{"guardrails: Stage ONLY clause present (new §17.6 wording)", "Stage ONLY
        the changes the description\nassigns to this concept (the files above are where they live)", true}`.
      • ADD `{"files-header PRESENT (with files)", "Files for this concept (where these changes live):",
        true}` and `{"per-path rendering PRESENT (with files)", "a.go\nb.go", true}` — but these must run on a
        files-PRESENT variant: build `pFiles := BuildStagerTask(title, desc, []string{"a.go","b.go"})` and
        assert against pFiles (NOT the nil `p`).
      • KEEP: the em-dash sub-test ("em-dash present (NOT ascii hyphen)" — "contents — only" present, "contents
        - only" absent), the backtick-command assertions, the <path> token, the "only update the index"
        assertion (now against the new const), and ALL the anti-copy-paste assertions (§17.1 / §17.5 elements
        ABSENT — unchanged).
      • UPDATE the "guardrails: 'only update the index' present" needle — still "only update the index" (the
        phrase is unchanged in the new wording; the assertion stays valid as-is).
      • UPDATE the title-interpolation / description-order sub-tests: they call BuildStagerTask with 2 args
        today (L120, L129) → add `, nil` as the third arg.
  - UPDATE TestBuildStagerTask_EdgeCases: all three BuildStagerTask calls (L159 empty-title, L174 empty-desc,
        L189 both-empty) → add `, nil` as the third arg. The assertions (HasPrefix stagerInstruction; Contains
        stagerGuardrails) stay valid with nil files (block omitted ⇒ desc directly precedes guardrails; for
        empty-title/empty-desc the topology still holds).
  - LEAVE UNCHANGED: nothing else in stager_test.go (there are only these three test funcs).
  - DEPENDENCIES: Task 1.
```

### Implementation Patterns & Key Details

```go
// PATTERN: conditional block with NO blank-line artifact. Build the optional segment into a local and splice
// it, OR use strings.Builder with an `if len(files) > 0 { ... }` between description and the guardrails'
// "\n\n". The CRITICAL invariant: when files is empty, the bytes are EXACTLY
//   stagerInstruction + "\n\n" + title + "\n" + description + "\n\n" + stagerGuardrails
// (no stray "\n\n"). TestBuildStagerTask_FilesBlock_OmittedWhenEmpty is the gate.

// PATTERN: named const per block (stagerInstruction / stagerFilesHeader / stagerGuardrails), each WITHOUT a
// trailing newline; BuildStagerTask owns ALL inter-block "\n" placement (stager.go:9-12 convention). The new
// stagerFilesHeader follows this exactly.

// PATTERN: paths one-per-line via strings.Join(files, "\n") — a single "\n" between paths, then "\n\n" before
// the guardrails. Two files ⇒ "...live):\na.go\nb.go\n\nUse git to stage...".

// GOTCHA: the guardrails const is a DOUBLE-QUOTED literal series (backticks inside forbid a raw string — ⚠️§4).
// The em-dash inside a double-quoted "..." literal is fine (Go source is UTF-8); do NOT escape it.

// GOTCHA: the Properties test mixes a nil-files `p` (for the desc→guardrails adjacency assertions) and a
// files-present `pFiles` (for the block assertions). Do NOT run the block assertions against the nil `p`.
```

### Integration Points

```yaml
NO DATABASE / NO CONFIG / NO ROUTES / NO CLI surface change.
  - This is a pure prompt-construction change. concept.Files ALREADY exists (P2.M1.T1.S1); no new flags, no
    new config keys, no new types.
BUILD:
  - `go build ./...` must pass (the signature change + the call-site + the new `import "strings"` in stager.go).
  - go.mod/go.sum UNCHANGED (only the stdlib `strings` import is added — already a transitive dep).
TESTS:
  - `go test ./internal/prompt/...` (the in-scope prompt package).
  - `go test ./internal/decompose/...` (regression net — stageConcept tests stay green, untouched; the test
    seam threads the whole PlannerCommit so it is transparent to the signature change).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
gofmt -w internal/prompt/stager.go internal/prompt/stager_test.go internal/decompose/stager.go
go vet ./internal/prompt/... ./internal/decompose/...
# Expected: zero issues. If stager.go did not import "strings", gofmt/goimports adds it; confirm it is present.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The in-scope package — the stager builder + its tests.
go test ./internal/prompt/... -run 'BuildStagerTask' -v
# Expected: TestBuildStagerTask_CanonicalExact (2 cases), TestBuildStagerTask_FilesBlock_OmittedWhenEmpty,
#   TestBuildStagerTask_Properties, TestBuildStagerTask_EdgeCases — all PASS.

# Regression net: stageConcept's signature is unchanged ⇒ these stay green with ZERO edits.
go test ./internal/decompose/... -run 'TestStageConcept' -v
# Expected: all green. stageConcept threads the whole PlannerCommit (transparent to the BuildStagerTask
#   signature change). The deps.stager seam (roles.go:73) is likewise unchanged.
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-repo build + vet + test (catches any orphan 2-arg BuildStagerTask call).
go build ./... && go vet ./... && go test ./...
# Expected: GREEN. If any call-site still uses the 2-arg form, it fails to compile — grep:
grep -rn "BuildStagerTask(" --include="*.go" . | grep -v "concept.Files\|\[\]string{\"a.go\"\|files=\[\]string"
# The ONLY non-test call-site (decompose/stager.go:99) MUST show concept.Files as the 3rd arg.
```

### Level 4: Byte-Identity & Omission (Domain-Specific Validation)

The canonical-exact + omission tests ARE the byte-identity gate. The exact `want` strings (em-dash U+2014
preserved verbatim — ⚠️§1; new §17.6 guardrails wording):

```go
// TestBuildStagerTask_CanonicalExact — case (a): files = []string{"a.go", "b.go"} (block PRESENT).
const title = "Refactor auth middleware"
const description = "Stage internal/auth/middleware.go and its callers in internal/api/."
const wantFiles = "Stage, but do NOT commit, all changes in this repository that match this concept:\n" +
	"\n" +
	"Refactor auth middleware\n" +
	"Stage internal/auth/middleware.go and its callers in internal/api/.\n" +
	"\n" +
	"Files for this concept (where these changes live):\n" +
	"a.go\n" +
	"b.go\n" +
	"\n" +
	"Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply\n" +
	"only the relevant hunks via `git apply --cached`). Stage ONLY the changes the description\n" +
	"assigns to this concept (the files above are where they live); leave everything else unstaged.\n" +
	"Do not commit, do not amend, do not push, do not modify file contents — only update the index.\n" +
	"When done, reply with the list of paths you staged and stop."
// got := BuildStagerTask(title, description, []string{"a.go", "b.go"})

// TestBuildStagerTask_CanonicalExact — case (b): files = nil (block ABSENT — byte-identity minus the block).
const wantNoFiles = "Stage, but do NOT commit, all changes in this repository that match this concept:\n" +
	"\n" +
	"Refactor auth middleware\n" +
	"Stage internal/auth/middleware.go and its callers in internal/api/.\n" +
	"\n" +
	"Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply\n" +
	"only the relevant hunks via `git apply --cached`). Stage ONLY the changes the description\n" +
	"assigns to this concept (the files above are where they live); leave everything else unstaged.\n" +
	"Do not commit, do not amend, do not push, do not modify file contents — only update the index.\n" +
	"When done, reply with the list of paths you staged and stop."
// got := BuildStagerTask(title, description, nil)
// ASSERT: wantFiles == wantNoFiles with the segment "\n\nFiles for this concept (where these changes live):\na.go\nb.go" removed.
```

```go
// TestBuildStagerTask_FilesBlock_OmittedWhenEmpty — the ⚠️§2 regression net.
pNil := BuildStagerTask("T", "D", nil)
pEmpty := BuildStagerTask("T", "D", []string{})
if strings.Contains(pNil, "Files for this concept") {
	t.Errorf("nil files must NOT contain the files block")
}
if strings.Contains(pEmpty, "Files for this concept") {
	t.Errorf("empty-slice files must NOT contain the files block")
}
if pNil != pEmpty {
	t.Errorf("nil and []string{} must render byte-identically;\nnil=%q\nempty=%q", pNil, pEmpty)
}
// And the legacy shape (no block, no stray blank line):
legacy := stagerInstruction + "\n\n" + "T" + "\n" + "D" + "\n\n" + stagerGuardrails
if pNil != legacy {
	t.Errorf("nil-files output must equal the legacy (no-block) shape;\ngot=%q\nwant=%q", pNil, legacy)
}
```

## Final Validation Checklist

### Technical Validation

- [ ] `gofmt -w` clean on the 3 files; `go vet ./...` zero issues; `import "strings"` present in stager.go.
- [ ] `go build ./...` compiles (signature change + call-site).
- [ ] `go test ./...` GREEN (whole repo).
- [ ] Every `BuildStagerTask(` call-site passes a 3rd arg: `grep -rn "BuildStagerTask(" --include="*.go" .`
      shows `concept.Files` at decompose/stager.go:99 and explicit `nil`/`[]string{...}` in the tests.
- [ ] go.mod/go.sum UNCHANGED.

### Feature Validation

- [ ] `BuildStagerTask(t, d, ["a.go","b.go"])` renders the files block: `...description\n\nFiles for this
      concept (where these changes live):\na.go\nb.go\n\nUse git to stage...`.
- [ ] `BuildStagerTask(t, d, nil)` AND `BuildStagerTask(t, d, []string{})` OMIT the block AND its leading
      `\n\n` (byte-identical to each other and to the legacy no-block shape — TestBuildStagerTask_FilesBlock_OmittedWhenEmpty).
- [ ] The `stagerGuardrails` const second sentence is `"Stage ONLY the changes the description assigns to this
      concept (the files above are where they live); leave everything else unstaged."`.
- [ ] The em-dash U+2014 in `"contents — only"` is PRESERVED (Properties em-dash sub-test green); the two
      backtick commands and the `<path>` token are present and unchanged.
- [ ] `decompose/stager.go:99` passes `concept.Files`; `stageConcept`'s signature and the `deps.stager` seam
      are byte-unchanged.

### Code Quality Validation

- [ ] Const names EXACTLY `stagerInstruction` (unchanged), `stagerFilesHeader` (new), `stagerGuardrails` (rewritten).
- [ ] `stagerFilesHeader` is `"Files for this concept (where these changes live):"` (trailing colon, NO trailing newline).
- [ ] `stagerGuardrails` is the verbatim 5-line §17.6 block; double-quoted literal style (NOT a raw string);
      em-dash preserved; NO trailing newline.
- [ ] Package conventions followed (consts without trailing `\n`; BuildStagerTask owns `\n` placement; the
      conditional block introduces no blank-line artifact).
- [ ] The file-top comment "§17.6 is ENTIRELY ASCII EXCEPT one em-dash" is preserved and still TRUE.

### Scope Validation

- [ ] EXACTLY 3 files changed: `internal/prompt/stager.go`, `internal/prompt/stager_test.go`, `internal/decompose/stager.go`.
- [ ] `internal/decompose/stager_test.go` UNCHANGED (transparent — threads whole PlannerCommit; Files defaults nil).
- [ ] `internal/decompose/roles.go` UNCHANGED (the deps.stager seam takes the whole PlannerCommit).
- [ ] Planner (`internal/prompt/planner.go`, `planner_test.go`, `internal/decompose/planner*.go`) UNCHANGED
      (P2.M1.T2.S1 COMPLETE).
- [ ] `internal/decompose/decompose*.go` UNCHANGED (P2.M1.T1.S2 COMPLETE).
- [ ] `docs/how-it-works.md` UNCHANGED (P2.M1.T2.S2 — Mode-A doc; item §5 says NO DOCS here).
- [ ] `internal/config/*`, `cmd/stagecoach/*`, `docs/cli.md`, `docs/configuration.md` UNCHANGED.

---

## Anti-Patterns to Avoid

- ❌ Don't ASCII-substitute the em-dash (⚠️§1) — the stager file PRESERVES U+2014 (the opposite of planner.go).
  Keep "file contents — only update the index" with the real em-dash in both the const and every test `want`.
- ❌ Don't emit a stray `"\n\n"` before the guardrails when files is empty (⚠️§2) — the block AND its leading
  blank line are BOTH omitted; nil/empty must be byte-identical to the legacy no-block shape.
- ❌ Don't surgically swap one guardrails sentence on the old line boundaries (⚠️§3) — replace the WHOLE const;
  the line-wrapping shifts because the new wording is longer.
- ❌ Don't switch stagerGuardrails to a backtick raw string (⚠️§4) — it contains backticks; keep the
  double-quoted `"..."` + `"\n"` join style.
- ❌ Don't change `stageConcept`'s signature or the `deps.stager` seam type — both take the whole
  `PlannerCommit`; only the L99 call gains `concept.Files`.
- ❌ Don't edit `decompose/stager_test.go`, `roles.go`, the planner, `decompose.go`, the docs, config, or the
  CLI — out of scope (other work items / transparent / complete).
- ❌ Don't redeclare `PlannerCommit.Files` or change `ParsePlannerOutput` — S1 is COMPLETE; this task only
  READS `concept.Files`.
- ❌ Don't run the Properties files-block assertions against the nil-files `p` — use a separate files-present
  `pFiles` for the header/per-path needles; the nil `p` is for the desc→guardrails adjacency assertions.
- ❌ Don't add any freeze/validation/constraint logic for `files` — it is GUIDANCE; FR-M1c (unchanged) is the
  sole content guarantee.
