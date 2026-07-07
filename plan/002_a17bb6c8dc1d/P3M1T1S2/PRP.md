---
name: "P3.M1.T1.S2 — Implement stager task prompt in internal/prompt/stager.go (PRD §17.6, §13.6.2, FR-M5)"
description: |

  CREATE ONE NEW FILE `internal/prompt/stager.go` in the existing `prompt` package: the **stager**
  task-prompt half of stagecoach's v2 multi-commit decomposition (PRD §17.6). The stager is a **tooled**
  agent (git access, repo-scoped). Unlike the planner (S1) and arbiter (S3), the stager's prompt is
  delivered **as the user payload** with a minimal/empty system prompt, it has **NO JSON contract** (it
  returns a text confirmation of paths staged, not structured output), and there is **NO parse
  function** (the orchestrator reads the exit code; the index is the truth source). So this is the
  SIMPLEST of the three decomposition prompts: one exported function, two private constants, zero
  imports.

  CONTRACT (P3.M1.T1.S2, verbatim from the work item):
    1. RESEARCH NOTE: §17.6 specifies the stager task prompt verbatim. The stager is TOOLED (git access,
       repo-scoped). It receives a concept's title + description as a TASK (not a system-prompt-and-diff).
       It must stage exactly that concept's changes and stop. Hard guardrails: no commit/amend/push/ref-
       mutation. The prompt is delivered as the user payload with minimal/empty system prompt.
    2. INPUT: The existing prompt package patterns.
    3. LOGIC: Create internal/prompt/stager.go. Define BuildStagerTask(title, description string) string
       — assembles the §17.6 stager task prompt: 'Stage, but do NOT commit, all changes in this
       repository that match this concept:' + title + description + the git instructions and guardrails
       (verbatim from §17.6). No JSON contract (stager returns a text confirmation of paths staged, not
       structured output).
    4. OUTPUT: prompt/stager.go exports BuildStagerTask(). Consumed by decompose/stager.go (P3.M2.T3.S1).
    5. DOCS: none — self-documenting via PRD §17.6.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `internal/prompt/system.go`, `payload.go` — CONSUMED as the pattern template; UNCHANGED.
    - `internal/prompt/planner.go` (P3.M1.T1.S1, parallel) + `internal/prompt/arbiter.go` (S3, planned)
      — separate standalone new files in the SAME package; ZERO merge friction.
    - `internal/decompose/stager.go` (P3.M2.T3.S1) — does NOT exist yet; THIS task does NOT create it.
    - go.mod / go.sum — UNCHANGED (ZERO imports — not even stdlib).
    - Sibling prompt files: stager.go is a NEW standalone file → zero merge friction with S1/S3.

  DELIVERABLES (2 new files, 0 modifications):
    CREATE internal/prompt/stager.go — `stagerInstruction` const (verbatim §17.6 instruction line),
      `stagerGuardrails` const (verbatim §17.6 5-line git-instructions + guardrails block), and
      `BuildStagerTask(title, description string) string`. ZERO imports.
    CREATE internal/prompt/stager_test.go — canonical-exact + properties (incl. anti-copy-paste) +
      edge-case tests.

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing tests still pass (purely
  additive — no existing file changes); the stager task prompt is byte-faithful to §17.6; title +
  description are interpolated verbatim in the §17.6 topology; no §17.1/§17.5 element leaks in.

---

## Goal

**Feature Goal**: Implement the stager-agent task-prompt-construction layer for multi-commit decomposition
(PRD §17.6 / §13.6.2 / FR-M5) as a self-contained, zero-import module in the existing `internal/prompt`
package. This is the prompt analogue of v1's `payload.go` — the verbatim §17.6 stager task prompt (the
instruction line + the 5-line git-instructions/guardrails block) with one concept's `title` + `description`
(from the planner, S1) interpolated between them. The stager is the ONLY tooled decompose role; its
behavioral guardrails (no commit/amend/push/ref-mutation) live in this task prompt AND are enforced
structurally (tooled_flags §12.1; stagecoach owns all ref ops). Unlike the planner (S1), there is NO
system-prompt constant, NO JSON contract, and NO parse function — the stager returns free-form text and
the orchestrator reads its exit code.

**Deliverable** (2 new files, 0 modifications):
1. `internal/prompt/stager.go` — the stager prompt module: `stagerInstruction` private const (verbatim
   §17.6 single-line instruction, trailing colon, NO trailing newline); `stagerGuardrails` private const
   (verbatim §17.6 5-line git-instructions + guardrails block — as a double-quoted `"..."` literal with
   `\n` joins because it contains backticks; NO trailing newline); `BuildStagerTask(title, description
   string) string` (`stagerInstruction + "\n\n" + title + "\n" + description + "\n\n" + stagerGuardrails`).
   ZERO imports (pure `+`-concatenation).
2. `internal/prompt/stager_test.go` — canonical-exact test (independently-derived `want` for a known
   `(title, description)`), properties table (instruction/guardrails/em-dash present, title+description
   interpolated in order + verbatim, anti-copy-paste guards pinning §17.1/§17.5 elements ABSENT), and
   edge-case tests (empty title / empty description / both empty — no panic).

**Success Definition**:
- `BuildStagerTask("Refactor auth middleware", "Stage internal/auth/middleware.go and its callers")`
  returns a string byte-faithful to §17.6: starts with the verbatim instruction line (trailing colon),
  one blank line, the title, the description (consecutive, no blank between), one blank line, then the
  verbatim 5-line guardrails block (with both backtick git commands, the literal `<path>` token, the
  em-dash, and the hard-guardrails clause). NO §17.1 mature elements leak ("You are a commit message
  generator", anti-reuse block, "Target ~"); NO §17.5 planner elements leak ("You are a commit-planning
  assistant", JSON contract).
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 2 new untracked files only.

## User Persona

**Target User**: the decompose stager agent invocation (internal code, P3.M2.T3.S1) and, by extension,
the end user running `stagecoach` on an un-staged working tree to get multiple logically-coherent commits.
The stager task prompt is NOT user-facing CLI text; it is the user payload piped to the tooled stager
agent (PRD §13.6.2 / §17.6). The system prompt for the stager is minimal/empty (§17.6) — the orchestrator
passes it; this task does NOT define a system-prompt constant.

**Use Case**: in the decompose loop (§13.6.3 step 2a), for each concept[i] the orchestrator
(P3.M2.T3.S1) calls `BuildStagerTask(concept.Title, concept.Description)`, renders the stager agent in
TOOLED mode (`Render(RenderTooled)`) with a minimal/empty system prompt + this task payload, executes it
(the stager runs `git add` / `git apply --cached` in the repo), and on exit 0 freezes a tree snapshot
(`write-tree`) BEFORE launching stager[i+1]. The stager's free-form text output ("list of paths staged")
is informational; the truth source for what was staged is the index (`git diff --cached --name-only`).

**Pain Points Addressed**: the stager needs a verbatim-faithful, unambiguous task prompt that (a) names
exactly the one concept (title + description) to stage, (b) instructs the git mechanics (`git add`,
hunk-stage via `git apply --cached`), and (c) restates the hard guardrails (no commit/amend/push/ref-
mutation; only update the index; then reply with staged paths and stop). The prompt + the structural
tooled_flags scoping together make a misbehaving stager unable to corrupt history (§17.6's safety proof).

## Why

- **Closes the stager half of PRD §17.6 / §13.6.2 / FR-M5 at the prompt layer.** FR-M5: the stager
  "for one concept, find all related changes and stage them (`git add`, hunk-stage via `git apply
  --cached`)" and "never commits, moves refs, or pushes — stagecoach owns all ref mutations." This task is
  the literal task-prompt-construction implementation of that role — the second of the three decomposition
  prompts (planner §17.5 = S1, parallel; stager §17.6 ← THIS; arbiter §17.7 = S3).
- **Mechanical extension of the established prompt-package pattern.** payload.go already proved the
  convention for interleaving dynamic content between named static constants: it splits
  `userInstructionReject` / `rejectionPreamble` / `rejectionEpilogue` around the rejected-subject list.
  stager.go splits `stagerInstruction` / `stagerGuardrails` around the title+description — the same
  pattern. No new architectural concept; just a new §17.x section + a single `+`-concatenation Build
  function.
- **Unblocks the decompose stager loop (P3.M2.T3.S1).** P3.M2.T3.S1 (the tooled stager agent call +
  snapshot freeze) consumes EXACTLY this task's exported surface (`BuildStagerTask(title, description
  string) string`). Nothing in the stager loop can run until this prompt-construction layer exists.
- **Lowest-risk, maximal-reuse, backward-compatible.** Two NEW files; ZERO modifications to any existing
  file. The prompt package GAINS one export (additive — no existing caller or test breaks). go.mod/go.sum
  untouched (ZERO imports — not even stdlib). No import cycle, no internal dependency.

## What

One new file `internal/prompt/stager.go` (package `prompt`) exporting ONE function, plus two private
constants; and one new test file `internal/prompt/stager_test.go`. No new types. No new dependencies. No
JSON. No parse. No system-prompt constant (§17.6 deliberately leaves it minimal/empty). No caller wiring
(that is P3.M2.T3.S1). Specifically:

- **`stagerInstruction`** (private const): `"Stage, but do NOT commit, all changes in this repository
  that match this concept:"` — the verbatim §17.6 instruction line (trailing COLON, mirroring payload.go's
  `userInstruction` precedent). NO trailing newline.
- **`stagerGuardrails`** (private const): the verbatim §17.6 5-line git-instructions + guardrails block,
  from "Use git to stage the relevant files and hunks..." through "...staged and stop." As a
  **double-quoted `"..."` string literal with `\n` joins** (it contains two backtick characters → CANNOT
  be a backtick raw string). NO trailing newline. Preserves the literal tokens `` `git add <path>` `` and
  `` `git apply --cached` `` (with `<path>` as a LITERAL instructive token, NOT a placeholder) and the
  em-dash in "file contents — only update the index".
- **`BuildStagerTask(title, description string) string`**: `stagerInstruction + "\n\n" + title + "\n" +
  description + "\n\n" + stagerGuardrails`. Pure `+`-concatenation. ZERO imports.

### Success Criteria

- [ ] `internal/prompt/stager.go` defines `stagerInstruction` + `stagerGuardrails` byte-faithful to §17.6
      (verified by an independently-derived canonical-exact test), each with NO trailing newline.
- [ ] `BuildStagerTask(title, description)` == `stagerInstruction + "\n\n" + title + "\n" + description +
      "\n\n" + stagerGuardrails`; title and description interpolated VERBATIM (title single-line;
      description may be multi-line — internal newlines survive).
- [ ] The verbatim §17.6 tokens survive: the two backtick git commands (`` `git add <path>` ``,
      `` `git apply --cached` ``), the literal `<path>` token, the hard-guardrails clause ("Do not
      commit, do not amend, do not push, do not modify file contents — only update the index"), and the
      em-dash (NOT an ASCII hyphen).
- [ ] NO §17.1 mature element leaks ("You are a commit message generator", "Output ONLY the commit
      message", "CRITICAL: You MUST NOT copy", "Target ~") AND no §17.5 planner element leaks ("You are a
      commit-planning assistant", "Respond with ONLY JSON", "Decompose these un-staged changes") — the
      stager prompt is self-contained (anti-copy-paste properties test passes).
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 2 new untracked files.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the verbatim §17.6 text
(findings §1 — captured character-for-character, with the CRITICAL literal-vs-placeholder distinction:
only `<title>`/`<description>` are placeholders; `<path>` inside `` `git add <path>` `` is LITERAL); the
exact assembly topology (findings §2 — `instruction\n\n<title>\n<description>\n\n<guardrails>`, the one
ambiguous title↔description point pinned by a canonical-exact test); the prompt-package conventions
(findings §3 — no-trailing-newline consts, split-constants-around-interleaved-dynamic-content (payload.go
precedent), Build-owns-newlines, rich doc comments, zero imports); the backtick gotcha (findings §4 — use
a double-quoted literal for stagerGuardrails); the em-dash gotcha (findings §5); the "why no system
prompt / no JSON / no parse" rationale (findings §6 — the simplest of the three prompts); the consumer
contract (findings §7 — exactly `BuildStagerTask(title, description) string`, consumed by P3.M2.T3.S1);
the test conventions (findings §8 — mirror payload_test.go/system_test.go, REUSE near/suffix); and the
scope boundaries (findings §9 — zero modifications, zero imports, no import cycle).

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (verbatim §17.6 text + conventions + the decisions)
- docfile: plan/002_a17bb6c8dc1d/P3M1T1S2/research/findings.md
  why: §1 the VERBATIM §17.6 task prompt + the CRITICAL literal-vs-placeholder distinction (only
       <title>/<description> are placeholders; <path> inside `git add <path>` is LITERAL); §2 the assembly
       topology decision (the ONE ambiguous point — title↔description separator — pinned by a canonical
       test); §3 the prompt-package conventions (no-trailing-newline, split-constants-around-interleaved-
       content (payload.go precedent), Build-owns-newlines, zero imports); §4 the BACKTICK GOTCHA
       (stagerGuardrails contains backticks → MUST use a double-quoted literal, NOT a raw string); §5 the
       EM-DASH gotcha (the one non-ASCII byte); §6 why no system prompt / no JSON / no parse (the simplest
       of the three decomposition prompts); §7 the consumer contract (P3.M2.T3.S1); §8 test conventions;
       §9 imports/lint/scope.
  critical: §1 (the verbatim text + the literal-vs-placeholder distinction); §4 (the backtick gotcha — the
            #1 implementation trap — a raw string literal will NOT compile because the block contains
            backticks); §2 (the assembly topology — the load-bearing decision); §6 (do NOT invent a
            StagerSystemPrompt constant or a ParseStagerOutput function — §17.6 deliberately omits both).

# MUST READ — the FILES TO MIRROR (the pattern templates; UNCHANGED — read only, do not edit)
- file: internal/prompt/payload.go
  section: the const style (userInstruction/userInstructionReject/rejectionPreamble/rejectionEpilogue:
           plain "..." double-quoted string literals, NO trailing newline); the SPLIT-CONSTANTS-AROUND-
           INTERLEAVED-DYNAMIC-CONTENT pattern (BuildUserPayload interleaves the rejected-subject list
           between rejectionPreamble and rejectionEpilogue); the fast-path `return a + "\n\n" + b` style
           for the common case.
  why: THIS is the closest template. stager.go mirrors payload.go's split-constants pattern: it splits
       stagerInstruction/stagerGuardrails because title+description are interleaved between them (exactly
       as the rejected list is interleaved in payload.go). Copy the STYLE (named consts, no-trailing-
       newline, Build-owns-newlines, `+`-concatenation fast path), NOT the §17.3 text.
  pattern: define `stagerInstruction = "Stage, but do NOT commit, all changes in this repository that
           match this concept:"` (trailing COLON, matching payload.go's userInstruction); define
           stagerGuardrails as a double-quoted literal with `\n` joins; BuildStagerTask is a single
           `return stagerInstruction + "\n\n" + title + "\n" + description + "\n\n" + stagerGuardrails`.
  gotcha: payload.go's consts have NO backticks so some use raw strings; stagerGuardrails HAS backticks →
          MUST use a double-quoted literal (findings §4).

- file: internal/prompt/system.go
  section: the package doc + const-declaration style (maturePromptHeader/antiReuseProhibition: rich doc
           comments citing the PRD §; the em-dash annotation in antiReuseProhibition's doc comment — DO
           THE SAME for stagerGuardrails); BuildSystemPrompt (the rich-doc-comment-citing-§17.1 pattern);
           the "no trailing newline on constants" convention explicitly documented.
  why: the const-doc-comment style + the em-dash documentation precedent. stagerGuardrails carries an
       em-dash (the ONE non-ASCII byte) — mirror system.go's explicit "NOTE the EM-DASH" doc comment.
  pattern: each const gets a doc comment citing §17.6 + noting any non-ASCII bytes (the em-dash); the Build
           function's doc comment cites §17.6 + diagrams the assembly topology.
  gotcha: do NOT copy §17.1's TEXT (role/output-contract/anti-reuse) — §17.6 is a SEPARATE self-contained
          prompt. An anti-copy-paste test pins §17.1 elements ABSENT.

# MUST READ — the test exemplars (the test patterns to mirror in stager_test.go)
- file: internal/prompt/payload_test.go
  section: TestBuildUserPayload_NormalCanonicalExact / TestBuildUserPayload_RejectionCanonicalExact (two
           canonical-exact tests, independently-derived `want`, asserted with %q diff on failure);
           TestBuildUserPayload_Properties (the cases table of structural invariants, including
           "diff is the verbatim tail"); TestBuildUserPayload_EdgeCases (empty diff, no panic).
  why: the EXACT test shapes stager_test.go mirrors. The canonical-exact test is the strongest guard
       against §17.6 text/topology drift; the properties table pins interpolation-in-order + verbatim +
       anti-copy-paste; the edge tests cover empty-string interpolation.
  pattern: write TestBuildStagerTask_CanonicalExact with an independently-derived `want` (build from
           §17.6, NOT from the impl — see findings §8 for the exact `want`); a TestBuildStagerTask_Properties
           cases table (instruction/guardrails/em-dash present, title+description in order + verbatim, §17.1
           + §17.5 elements ABSENT); a TestBuildStagerTask_EdgeCases (empty title/description/both).

- file: internal/prompt/system_test.go
  section: the package-level helpers `near(s, needle)` + `suffix(s, n)` at the BOTTOM of the file.
  why: these helpers are ALREADY defined (same package `prompt`). stager_test.go MUST reuse them — do NOT
       redeclare (duplicate-symbol compile error). The `near` helper is used for readable failure output
       around a substring (e.g. the em-dash check).
  pattern: import "strings" + "testing" in stager_test.go; call near()/suffix() directly (they're in
           system_test.go, same package).

# MUST READ — the design reference (role + the consumer)
- docfile: plan/002_a17bb6c8dc1d/architecture/decompose_architecture.md
  section: "The Four Agent Roles" table (stager: tooled, "stage one concept's changes (`git add`,
           hunk-stage) | exits 0; mutates index | reuses prompt/stager.go, Render(RenderTooled)");
           "Pipeline Flow" step 2a (stager[i] → index now holds concepts[0..i]); "Failure Handling"
           (stager exit non-zero → retry once → treat as empty; stager stages nothing → skip).
  why: confirms the stager role is TOOLED, that its output is "exits 0; mutates index" (NOT structured
       output — hence no JSON/parse), and that THIS task's export is consumed by decompose/stager.go
       (P3.M2.T3.S1). The doc lists prompt/stager.go as the reuse; THIS task implements it.

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §17.6 (the stager task prompt — verbatim instruction + title/description + guardrails block)
  why: §17.6 is the verbatim source for stagerInstruction + stagerGuardrails + the assembly topology. The
       fenced code block is authoritative (same precedent as §17.1/§17.2 code blocks per S1).
- url: PRD.md §13.6.2 (the four agent roles — stager: tooled, "exits 0; mutates the index; returns a short
       confirmation") + §13.6.6 (stager failure handling: stages nothing → skip; exits non-zero → retry once)
  why: the runtime semantics the task prompt must serve — confirms NO JSON output contract (the index is
       the truth source) and that failure is exit-code-driven (hence no parse/retry-instruction constant).
```

### Current Codebase tree (relevant slice)

```bash
internal/prompt/
  payload.go              # READ: the CLOSEST template — split-constants-around-interleaved-content +
                          #   fast-path `+`-concatenation + named consts w/ no trailing newline. UNCHANGED.
  payload_test.go         # READ: the canonical-exact + properties + edge test shapes to mirror. UNCHANGED.
  system.go               # READ: the const-doc-comment style + the em-dash documentation precedent +
                          #   BuildSystemPrompt's rich doc comment. UNCHANGED.
  system_test.go          # READ: the canonical-exact + anti-copy-paste-properties shapes; ALSO defines the
                          #   package-level helpers near() + suffix() (REUSE — do NOT redeclare). UNCHANGED.
  planner.go              # PARALLEL (S1) — separate standalone new file; ZERO merge friction.
go.mod / go.sum           # UNCHANGED (ZERO imports in stager.go).
.golangci.yml             # READ: linter config (errcheck/gosimple/govet/ineffassign/staticcheck/unused).
```

### Desired Codebase tree with files to be added

```bash
internal/prompt/stager.go          # NEW — the stager prompt module (package prompt):
                                    #   const stagerInstruction   (verbatim §17.6 instruction line, trailing colon, no trailing \n)
                                    #   const stagerGuardrails    (verbatim §17.6 5-line git-instructions+guardrails block;
                                    #                              double-quoted literal w/ \n joins — contains backticks; no trailing \n)
                                    #   func BuildStagerTask(title, description string) string   (pure + concatenation; ZERO imports)
internal/prompt/stager_test.go      # NEW — canonical-exact + properties (incl. anti-copy-paste §17.1/§17.5 ABSENT) + edge tests.
                                    #   REUSES near()/suffix() from system_test.go (same package — do NOT redeclare).
# go.mod/go.sum UNCHANGED. payload.go/system.go/planner.go all UNCHANGED. 0 modifications. 0 imports.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the BACKTICK GOTCHA — findings §4, the #1 implementation trap): stagerGuardrails contains
//   TWO backtick characters: `git add <path>` and `git apply --cached`. A Go BACKTICK RAW STRING LITERAL
//   (`...`) CANNOT contain a backtick — the file will NOT COMPILE. stagerGuardrails MUST be a
//   DOUBLE-QUOTED "..." string literal (in which backticks are ordinary chars needing no escaping), with
//   the 5-line wrapping preserved via explicit "\n" joins (concatenated "...\n" + literals). This is the
//   FIRST constant in the prompt package that contains backticks → first use of the double-quoted-with-\n
//   form here (maturePromptHeader/antiReuseProhibition/rejectionPreamble use raw strings only because they
//   have NO backticks). stagerInstruction has no backticks but use a plain "..." literal for consistency.

// CRITICAL (literal-vs-placeholder — findings §1): in §17.6 ONLY <title> and <description> are runtime
//   placeholders (interpolated from the planner's PlannerCommit). The <path> inside `git add <path>` is a
//   LITERAL instructive token (part of the command example shown to the model) — it STAYS in the constant
//   VERBATIM. Do NOT try to parameterize <path>. (Same principle as S1's <int>/<bool> JSON tokens: format
//   examples stay literal; only the data slots are placeholders.)

// CRITICAL (the EM-DASH — findings §5): stagerGuardrails line 4 has an EM-DASH "—" (U+2014) in
//   "file contents — only update the index". It is the ONE non-ASCII byte (same as §17.1's
//   antiReuseProhibition). In a double-quoted Go string literal the UTF-8 em-dash bytes are literal (no
//   escaping). It MUST NOT be replaced with an ASCII hyphen "-" (verbatim §17.6 fidelity). Document it in
//   the const's doc comment (mirror system.go's "NOTE the EM-DASH" annotation).

// CRITICAL (assembly topology — findings §2, the load-bearing decision): §17.6 shows <title> and
//   <description> on CONSECUTIVE lines (a single \n between, NOT a blank line). The assembly is:
//     stagerInstruction + "\n\n" + title + "\n" + description + "\n\n" + stagerGuardrails
//   i.e. instruction + blank + title + description + blank + guardrails. Pin it with a canonical-exact
//   test. (This is the one ambiguous point — §17.6 doesn't spell out the title↔description separator.)

// CRITICAL (no system prompt / no JSON / no parse — findings §6): the stager is the SIMPLEST of the three
//   decomposition prompts. DO NOT invent a StagerSystemPrompt constant (§17.6: system prompt "minimal/
//   empty" — the orchestrator's concern). DO NOT add JSON-contract types (the stager returns free-form
//   text; the index is the truth source). DO NOT add a ParseStagerOutput (failure is exit-code-driven per
//   §13.6.6). BuildStagerTask is the SOLE export.

// GOTCHA (no-trailing-newline + Build-owns-newlines — findings §3): BOTH stagerInstruction and
//   stagerGuardrails are defined WITHOUT trailing newlines. BuildStagerTask owns ALL inter-block newline
//   placement (the two "\n\n" separators + the "\n" between title and description). This is the load-
//   bearing convention from system.go/payload.go — the §17.6 blank-line topology lives in exactly one
//   auditable place.

// GOTCHA (ZERO imports — findings §9): BuildStagerTask is pure `+`-concatenation of two consts + two
//   params. No `strings`, no `fmt`, no `encoding/json`. The .go file OMITS the import block entirely. Do
//   NOT add a `strings.Builder` (overkill for a 5-part concat; payload.go's fast path uses `+`).

// GOTCHA (shared test helpers near()/suffix() — findings §8): they are ALREADY defined at the BOTTOM of
//   system_test.go (same package `prompt`). Do NOT redeclare them in stager_test.go (duplicate-symbol
//   compile error). stager_test.go is `package prompt` (internal test) so the unexported
//   stagerInstruction/stagerGuardrails ARE visible to the tests.

// GOTCHA (anti-copy-paste — findings §8, the #1 risk): §17.6 is a SEPARATE, self-contained prompt. It
//   MUST NOT contain §17.1's mature elements ("You are a commit message generator", "Output ONLY the
//   commit message", the anti-reuse block "CRITICAL: You MUST NOT copy", "Target ~") NOR §17.5's planner
//   elements ("You are a commit-planning assistant", "Respond with ONLY JSON", "Decompose these un-staged
//   changes"). A properties-table test pins each absence.

// GOTCHA (PARALLEL execution with S1): planner.go (S1) is being implemented in parallel. stager.go is a
//   standalone new file — it does NOT import planner.go or reference PlannerCommit (it takes plain
//   (title, description string)). Zero merge friction. Both land as independent files in package prompt.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/prompt/stager.go — package prompt
// (NO data models / types. The stager has NO JSON contract — its output is free-form text and the index
//  is the truth source. BuildStagerTask returns a plain string. See findings §6.)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/prompt/stager.go — stagerInstruction const (verbatim §17.6)
  - ADD a package-level doc comment (mirror system.go's: cite PRD §17.6, explain the stager role (tooled,
    repo-scoped; stages exactly one concept's changes and stops), note that constants carry no trailing
    newline + BuildStagerTask owns inter-block newlines, note §17.6 is all-ASCII EXCEPT one em-dash).
  - DEFINE `stagerInstruction` (private const, plain "..." double-quoted literal):
      `Stage, but do NOT commit, all changes in this repository that match this concept:`
    (trailing COLON — mirrors payload.go's userInstruction). NO trailing newline.
  - NAMING: lowercase for private (stagerInstruction). Mirror payload.go's const-naming.
  - GOTCHA: do NOT use a backtick raw string (stagerInstruction has no backticks so it COULD compile as
    one, but use a "..." literal for consistency with stagerGuardrails which MUST be "...").

Task 2: CREATE internal/prompt/stager.go — stagerGuardrails const (verbatim §17.6, DOUBLE-QUOTED)
  - DEFINE `stagerGuardrails` (private const) as a DOUBLE-QUOTED "..." string literal with explicit "\n"
    joins (concatenated "...\n" + literals) — the verbatim §17.6 5-line git-instructions + guardrails
    block. EXACTLY:
      "Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply\n" +
      "only the relevant hunks via `git apply --cached`). Stage ONLY changes belonging to this\n" +
      "concept; leave unrelated changes unstaged. Do not commit, do not amend, do not push, do not\n" +
      "modify file contents — only update the index. When done, reply with the list of paths you\n" +
      "staged and stop."
    NO trailing newline.
  - DOC COMMENT: cite §17.6; note it is the prompt-level restatement of §13.6.2/§17.6's structural
    guardrails (no commit/amend/push/ref-mutation; only update the index), enforced STRUCTURALLY too via
    tooled_flags (§12.1) — stagecoach owns all ref ops; NOTE the TWO BACKTICK chars (`git add <path>` and
    `git apply --cached`) → hence a double-quoted literal (NOT a raw string); NOTE the EM-DASH "—" (U+2014)
    in "file contents — only update the index" is the ONE non-ASCII byte (must NOT become an ASCII hyphen);
    NOTE the literal `<path>` token inside `git add <path>` is instructive (NOT a runtime placeholder).
  - GOTCHA: a backtick raw string literal WILL NOT COMPILE (the block contains backticks). MUST be a
    double-quoted literal. (Findings §4 — the #1 trap.)
  - GOTCHA: preserve the EXACT 5-line wrapping from §17.6's fenced code block (the line breaks are
    verbatim, like §17.1's maturePromptHeader line wrapping).

Task 3: CREATE internal/prompt/stager.go — BuildStagerTask(title, description string) string
  - SIGNATURE: `func BuildStagerTask(title, description string) string` (NO config, NO git — minimal
    decoupled params; the concept comes from the planner as plain strings).
  - BODY (single expression, pure + concatenation — NO Builder, NO branches):
      return stagerInstruction + "\n\n" + title + "\n" + description + "\n\n" + stagerGuardrails
  - DOC COMMENT: cite §17.6 + §13.6.2; explain the stager is TOOLED (git access, repo-scoped) and this is
    its TASK prompt (delivered as the user payload; system prompt minimal/empty — NOT a prompt constant);
    diagram the assembly topology:
        stagerInstruction       // "...match this concept:" (no trailing \n)
        '\n' '\n'               // ONE blank line before the concept
        title                   // the concept's short label (verbatim; single-line per planner §17.5)
        '\n'                    // title + description on CONSECUTIVE lines (§17.6 — NO blank between)
        description             // the staging instructions (verbatim; may be multi-line)
        '\n' '\n'               // ONE blank line before the guardrails
        stagerGuardrails        // the 5-line git-instructions + hard-guardrails block (no trailing \n)
    note title/description are interpolated VERBATIM (the planner's PlannerCommit{Title,Description});
    note the stager returns free-form text ("list of paths staged") and the index is the truth source
    (hence NO JSON contract / NO parse — the caller P3.M2.T3.S1 reads exit code); note the guardrails are
    ALSO enforced structurally (tooled_flags §12.1; stagecoach owns all ref ops — §17.6 safety proof);
    defensive: empty title/description do not panic (the planner always supplies both per §17.5).
  - GOTCHA: ZERO imports (no strings, no fmt). The file's import block is OMITTED entirely.
  - PLACEMENT: function after the two consts.

Task 4: CREATE internal/prompt/stager_test.go — tests (mirror payload_test.go/system_test.go shapes)
  - IMPORTS: "strings", "testing". Package: `prompt` (internal — near/suffix REUSED from system_test.go;
    unexported stagerInstruction/stagerGuardrails visible).
  - GOTCHA: do NOT redeclare near()/suffix() (in system_test.go). Do NOT import planner.go types.
  - ADD TestBuildStagerTask_CanonicalExact: independently-derived `want` (build from §17.6, NOT from the
    impl) for e.g. title="Refactor auth middleware", description="Stage internal/auth/middleware.go and
    its callers in internal/api/.". Assert got==want with a %q diff on failure. The want is EXACTLY:
        "Stage, but do NOT commit, all changes in this repository that match this concept:\n" +
        "\n" +
        "Refactor auth middleware\n" +
        "Stage internal/auth/middleware.go and its callers in internal/api/.\n" +
        "\n" +
        "Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply\n" +
        "only the relevant hunks via `git apply --cached`). Stage ONLY changes belonging to this\n" +
        "concept; leave unrelated changes unstaged. Do not commit, do not amend, do not push, do not\n" +
        "modify file contents — only update the index. When done, reply with the list of paths you\n" +
        "staged and stop."
  - ADD TestBuildStagerTask_Properties: a cases table of structural invariants:
      "instruction line present and starts output" — HasPrefix("Stage, but do NOT commit...") .
      "guardrails: first line present" — Contains "Use git to stage the relevant files and hunks".
      "guardrails: `git add <path>` backtick command present (literal)" — Contains "`git add <path>`".
      "guardrails: `git apply --cached` backtick command present" — Contains "`git apply --cached`".
      "guardrails: literal <path> token present (NOT a placeholder)" — Contains "<path>".
      "guardrails: Stage ONLY clause present" — Contains "Stage ONLY changes belonging to this concept".
      "guardrails: hard-guardrails clause present" — Contains "Do not commit, do not amend, do not push".
      "guardrails: 'only update the index' present" — Contains "only update the index".
      "guardrails: reply-with-paths instruction present" — Contains "reply with the list of paths you staged and stop".
      "em-dash present (NOT ascii hyphen)" — Contains "contents — only" AND NOT Contains "contents - only".
      "title interpolated, in order before description" — for title="TTT" desc="DDD": Index(title) < Index(desc).
      "description interpolated, in order after title" — Index(desc) > Index(title) AND Index(desc) < Index("Use git").
      "title is verbatim (symbols/spaces survive)" — for title="feat(api): add [x] & y": Contains it verbatim.
      "multi-line description: internal newlines survive" — for desc="line1\nline2": Contains "line1\nline2".
      "anti-copy-paste: §17.1 'commit message generator' ABSENT" — NOT Contains "You are a commit message generator".
      "anti-copy-paste: §17.1 'Output ONLY the commit message' ABSENT" — NOT Contains "Output ONLY the commit message".
      "anti-copy-paste: §17.1 anti-reuse block ABSENT" — NOT Contains "CRITICAL: You MUST NOT copy".
      "anti-copy-paste: §17.1 'Target ~' ABSENT" — NOT Contains "Target ~".
      "anti-copy-paste: §17.5 'commit-planning assistant' ABSENT" — NOT Contains "You are a commit-planning assistant".
      "anti-copy-paste: §17.5 JSON contract ABSENT" — NOT Contains "Respond with ONLY JSON".
      "anti-copy-paste: §17.5 planner user-instruction ABSENT" — NOT Contains "Decompose these un-staged changes".
      "blank-line topology: exactly one blank line before title" — HasPrefix(stagerInstruction + "\n\n" + title).
      "blank-line topology: exactly one blank line after description (before guardrails)" — Contains description + "\n\n" + stagerGuardrails OR check strings.Count of "\n\n".
  - ADD TestBuildStagerTask_EdgeCases:
      "empty title does not panic" — BuildStagerTask("", "desc") under defer recover().
      "empty description does not panic" — BuildStagerTask("title", "").
      "both empty" — BuildStagerTask("", ""); assert it starts with the instruction + still contains the guardrails.
  - PLACEMENT: NEW file internal/prompt/stager_test.go.

Task 5: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/prompt/stager.go internal/prompt/stager_test.go`
  - `go build ./...`   (whole module compiles — the new file + test file. NOTE: if planner.go (S1) is
    mid-implementation and has a build error, that is S1's concern, NOT this task's — stager.go must
    compile independently; if `go build ./...` fails ONLY on planner.go, confirm stager.go itself is clean
    via `go build ./internal/prompt/` is not possible in isolation — instead `go vet`/`gofmt` on the file
    + confirm the stager_test tests pass once planner.go lands. Coordinate: this task's stager.go/stager_test.go
    must be individually gofmt-clean + lint-clean.)
  - `go vet ./...`
  - `golangci-lint run ./internal/prompt/...`   (errcheck/gosimple/govet/ineffassign/staticcheck/unused)
  - `go test -race ./internal/prompt/ -run "Stager" -v`   (all new stager tests)
  - `go test -race ./internal/prompt/`   (the WHOLE prompt package — system/payload/planner tests still pass)
  - `go test ./...`   (FULL regression — no other package breaks; purely additive)
  - `git diff --exit-code go.mod go.sum` ⇒ empty.
  - `git status --short` ⇒ includes `?? internal/prompt/stager.go` + `?? internal/prompt/stager_test.go`;
    payload.go/system.go UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// === stager.go: the verbatim §17.6 instruction line (private const, plain "..." literal, NO trailing \n) ===
const stagerInstruction = "Stage, but do NOT commit, all changes in this repository that match this concept:"


// === stager.go: the verbatim §17.6 guardrails block (private const, DOUBLE-QUOTED literal w/ \n joins) ===
// NOTE the TWO backtick chars (`git add <path>` and `git apply --cached`) → CANNOT be a backtick raw
// string literal (will not compile); use a double-quoted "..." literal (backticks are ordinary chars).
// NOTE the EM-DASH "—" (U+2014) in "file contents — only update the index" — the ONE non-ASCII byte;
// do NOT replace with an ASCII hyphen (verbatim §17.6 fidelity). NOTE the literal `<path>` token is
// instructive (part of the command example), NOT a runtime placeholder.
const stagerGuardrails = "Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply\n" +
	"only the relevant hunks via `git apply --cached`). Stage ONLY changes belonging to this\n" +
	"concept; leave unrelated changes unstaged. Do not commit, do not amend, do not push, do not\n" +
	"modify file contents — only update the index. When done, reply with the list of paths you\n" +
	"staged and stop."


// === BuildStagerTask: the §17.6 task prompt (delivered as the user payload; system prompt minimal/empty) ===
// Assembly: instruction + blank + title + description(consecutive) + blank + guardrails. Pure +
// concatenation; ZERO imports. title/description interpolated verbatim from the planner's PlannerCommit.
func BuildStagerTask(title, description string) string {
	return stagerInstruction + "\n\n" + title + "\n" + description + "\n\n" + stagerGuardrails
}
```

### Integration Points

```yaml
DATABASE:
  - none (pure prompt-construction; no git, no IO).

CONFIG:
  - none directly. title/description come from the planner's PlannerCommit (S1); this layer takes them as
    plain strings — it is decoupled from config (mirrors BuildUserPayload taking diff as a string).

ROUTES:
  - none (internal prompt module; no CLI flag, no public API surface in this task). The decompose
    orchestrator's stager loop (P3.M2.T3.S1) wires the call:
      for i, concept := range plannerOutput.Commits {                              # S1
          task := prompt.BuildStagerTask(concept.Title, concept.Description)       # THIS task — the SOLE export
          raw, exitErr := <render(RenderTooled) + execute the stager agent>        # P3.M2.T3.S1
          if exitErr != nil { <retry once; §13.6.6> }
          # success = exitErr == nil (NOT a parse of `raw`); the truth source is the index
      }
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the new files (run after creation; fix anything that changes).
gofmt -w internal/prompt/stager.go internal/prompt/stager_test.go
gofmt -l internal/ pkg/          # Expected: empty (no files need formatting).

# Lint the new files specifically, then the whole module.
golangci-lint run ./internal/prompt/...
golangci-lint run ./...          # Expected: clean (errcheck/gosimple/govet/ineffassign/staticcheck/unused).

# Vet.
go vet ./...                     # Expected: no findings.

# Expected: zero errors. If any exist, READ the output and fix before proceeding. NOTE: if planner.go
# (S1, parallel) is mid-implementation with a build error, stager.go must STILL be individually gofmt-clean
# and lint-clean (run `gofmt -l internal/prompt/stager.go` and `golangci-lint run` on just this file).
```

### Level 2: Unit Tests (Component Validation)

```bash
# Run the new stager tests in isolation (verbose — confirm every case).
go test -race ./internal/prompt/ -run "Stager" -v

# Whole prompt package (system.go + payload.go + planner.go tests must still pass — the new file is
# additive).
go test -race ./internal/prompt/

# Expected: all pass. The canonical-exact test is the strongest guard — a mismatch means §17.6 text or
# assembly-topology drift; fix the implementation (or, if the test's independently-derived `want` is wrong,
# re-derive it from §17.6). Do NOT weaken the canonical-exact assertion to make it pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the whole module (the new file compiles + links; no existing importer breaks).
go build ./...

# Full regression (purely additive — no other package should change behavior).
go test ./...

# Confirm the module is unchanged apart from the two new files.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged"
git status --short                # Expected (at least): ?? internal/prompt/stager.go  ?? internal/prompt/stager_test.go

# (No live agent / service to start — stager.go is a pure library module. The "integration" is the decompose
#  stager loop in P3.M2.T3.S1, which is a SEPARATE work item and does NOT exist yet.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# §17.6 faithfulness self-check (run interactively; eyeball against PRD §17.6):
go test -run TestBuildStagerTask_CanonicalExact -v ./internal/prompt/ && \
  echo "PASS: stager task prompt is byte-faithful to §17.6"

# Anti-copy-paste guard (the #1 risk — §17.1/§17.5 elements must NOT leak into the stager prompt):
go test -run "TestBuildStagerTask_Properties" -v ./internal/prompt/

# Defensive spot-check (empty title/description never panic):
go test -run "TestBuildStagerTask_EdgeCases" -v ./internal/prompt/

# Expected: all pass. The Properties cases (backtick commands present, em-dash present, §17.1/§17.5 ABSENT)
# are the domain-specific validations — they prove the prompt is byte-faithful to §17.6 AND self-contained.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] All tests pass: `go test ./...` (and specifically `go test -race ./internal/prompt/`).
- [ ] No lint errors: `golangci-lint run ./internal/prompt/...` (and `./...`).
- [ ] No vet errors: `go vet ./...`.
- [ ] No formatting issues: `gofmt -l internal/ pkg/` empty.
- [ ] go.mod / go.sum UNCHANGED (`git diff --exit-code go.mod go.sum` ⇒ empty).

### Feature Validation

- [ ] `stagerInstruction` + `stagerGuardrails` are byte-faithful to §17.6 (pinned by the canonical-exact test).
- [ ] `BuildStagerTask(title, description)` interpolates title + description verbatim in the §17.6 topology
      (`instruction\n\n<title>\n<description>\n\n<guardrails>`); title before description (consecutive, no
      blank between); multi-line description's internal newlines survive.
- [ ] The verbatim §17.6 tokens survive: the two backtick git commands, the literal `<path>` token, the
      hard-guardrails clause, and the em-dash (NOT an ASCII hyphen).
- [ ] NO §17.1 mature-prompt element AND no §17.5 planner-prompt element leaks into the stager prompt
      (anti-copy-paste properties test passes).
- [ ] NO system-prompt constant, NO JSON-contract types, NO parse function invented (§17.6 omits all three;
      BuildStagerTask is the sole export).

### Code Quality Validation

- [ ] Follows existing prompt-package conventions (no-trailing-newline consts; Build-owns-newlines; split-
      constants-around-interleaved-content per payload.go; rich doc comments citing §17.6; minimal decoupled
      params; ZERO imports).
- [ ] File placement matches the desired tree (2 new files in internal/prompt/).
- [ ] Anti-patterns avoided (no backtick raw string for stagerGuardrails; no trailing-newline constants; no
      invented system prompt / JSON / parse; no redeclared near/suffix helpers; no strings.Builder for a
      trivial concat).
- [ ] Dependencies: ZERO (not even stdlib); no new internal dep; no import cycle.

### Documentation & Deployment

- [ ] Rich doc comments on the package + both consts + BuildStagerTask (cite §17.6; diagram assembly; note
      the backtick/em-dash/literal-token gotchas; note why no system prompt / no JSON / no parse).
- [ ] No new environment variables or config (this layer is decoupled from config).
- [ ] Self-documenting via the §17.6 reference (no separate docs file needed, per the work item's DOCS: none).

---

## Anti-Patterns to Avoid

- ❌ Don't use a BACKTICK RAW STRING literal for `stagerGuardrails` — it contains two backtick characters
  and WILL NOT COMPILE. Use a double-quoted `"..."` literal with `\n` joins (findings §4 — the #1 trap).
- ❌ Don't put a trailing newline on any string constant — `BuildStagerTask` owns inter-block newlines.
- ❌ Don't try to parameterize the literal `<path>` token inside `` `git add <path>` `` — it is instructive
  text (part of the command example), NOT a runtime placeholder. Only `<title>`/`<description>` are
  placeholders (findings §1).
- ❌ Don't replace the em-dash "—" with an ASCII hyphen "-" — verbatim §17.6 fidelity (same rule system.go
  applies to antiReuseProhibition).
- ❌ Don't invent a `StagerSystemPrompt` constant — §17.6 says the system prompt is "minimal/empty" (the
  orchestrator's concern). Don't add JSON-contract types or a `ParseStagerOutput` — the stager returns
  free-form text and the index is the truth source (findings §6).
- ❌ Don't leak §17.1 mature-prompt elements ("You are a commit message generator", anti-reuse block,
  "Target ~") OR §17.5 planner-prompt elements ("You are a commit-planning assistant", JSON contract,
  "Decompose these un-staged changes") into the stager prompt — §17.6 is self-contained.
- ❌ Don't redeclare `near()`/`suffix()` in stager_test.go — they already live in system_test.go.
- ❌ Don't add a `strings.Builder` or any import — `BuildStagerTask` is pure `+`-concatenation; the file has
  ZERO imports.
- ❌ Don't modify payload.go, system.go, planner.go (S1), or any existing file — this task is purely
  additive (2 new files). Don't add config/git params to `BuildStagerTask` — §17.6 takes only title+description.
- ❌ Don't weaken the canonical-exact test to make it pass — re-derive `want` from §17.6 if it mismatches.

---

**Confidence Score: 9/10** — This is the SIMPLEST task in the decompose prompts epic: one ~1-line Build
function (a single `+`-concatenation expression) + two private string constants, a mechanical extension of
payload.go's split-constants-around-interleaved-content pattern. Zero modifications to existing files
(purely additive), ZERO imports (not even stdlib), no new types, no JSON, no parse, no import cycle. The
only residual uncertainties are (a) the title↔description separator topology (§17.6 shows them on
consecutive lines without spelling out the newline) — resolved by a defensible §17.6-faithful decision + a
canonical-exact test, trivially adjustable later; and (b) the backtick/em-dash gotchas — both fully spelled
out in findings §4/§5 with the exact constant source provided. The exported surface
(`BuildStagerTask(title, description) string`) exactly matches the consumer (P3.M2.T3.S1, confirmed by
decompose_architecture.md), so the stager loop can proceed the moment this lands. Parallel-safe with S1
(planner.go) and S3 (arbiter.go) — three independent standalone files in package prompt.
