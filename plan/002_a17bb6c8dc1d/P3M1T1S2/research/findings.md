# Research Findings — P3.M1.T1.S2 (stager task prompt, `internal/prompt/stager.go`)

Scope: one new file `internal/prompt/stager.go` exporting exactly ONE function
`BuildStagerTask(title, description string) string`, plus `internal/prompt/stager_test.go`. Zero
modifications to any existing file. This is the stager half of PRD §17.6 / §13.6.2 / FR-M5.

## §1 — The VERBATIM §17.6 task prompt (the authoritative source)

From PRD §17.6 (the fenced code block is authoritative, exactly as §17.1/§17.2 code blocks are). The
stager prompt is delivered **as the user payload** (system prompt "minimal/empty" — NOT a prompt-package
constant; see §6). The full task prompt, with `<title>`/`<description>` as RUNTIME placeholders:

```
Stage, but do NOT commit, all changes in this repository that match this concept:

<title>
<description>

Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply
only the relevant hunks via `git apply --cached`). Stage ONLY changes belonging to this
concept; leave unrelated changes unstaged. Do not commit, do not amend, do not push, do not
modify file contents — only update the index. When done, reply with the list of paths you
staged and stop.
```

Decomposition into two constants (mirrors payload.go's split of static text around dynamic content):

- **`stagerInstruction`** (private const, ONE line, NO trailing newline):
  `Stage, but do NOT commit, all changes in this repository that match this concept:`
  (trailing COLON, like payload.go's `userInstruction`.)

- **`stagerGuardrails`** (private const, FIVE lines, NO trailing newline):
  ```
  Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply
  only the relevant hunks via `git apply --cached`). Stage ONLY changes belonging to this
  concept; leave unrelated changes unstaged. Do not commit, do not amend, do not push, do not
  modify file contents — only update the index. When done, reply with the list of paths you
  staged and stop.
  ```
  NOTE the literal tokens that MUST survive verbatim: the two backtick-quoted git commands
  `` `git add <path>` `` and `` `git apply --cached` ``, and `<path>` inside the first (a literal
  instructive token, NOT a runtime placeholder — it is part of the command example shown to the
  model). The hard guardrails clause "Do not commit, do not amend, do not push, do not modify file
  contents — only update the index" is the prompt-level restatement of §13.6.2/§17.6's structural
  guardrails (no commit/amend/push/ref-mutation); it is enforced STRUCTURALLY too (tooled_flags,
  §12.1; stagecoach owns all ref ops).

CRITICAL literal-vs-placeholder distinction: in §17.6, ONLY `<title>` and `<description>` are runtime
placeholders (interpolated from the planner's `PlannerCommit`). The `<path>` inside
`` `git add <path>` `` is a LITERAL instructive token (part of the command example) — it stays in the
constant VERBATIM. This is exactly analogous to the planner's `<int>`/`<bool>` tokens (S1 findings §1):
the JSON-format tokens stayed literal; only `<style examples>` was a placeholder. Here, only
`<title>`/`<description>` are placeholders.

## §2 — The assembly topology (the ONE ambiguous point, pinned by a canonical-exact test)

§17.6 shows the layout:
```
Stage, but do NOT commit, all changes in this repository that match this concept:
<blank>
<title>
<description>
<blank>
Use git to stage...
```

So: instruction colon → ONE blank line → title → description (CONSECUTIVE, no blank between) → ONE
blank line → guardrails. The ambiguous part is the title↔description separator: §17.6 shows them on
consecutive lines (a single `\n` between, NOT a blank line). DECISION (the load-bearing call, pinned
by a canonical-exact test):

```
BuildStagerTask(title, description) =
    stagerInstruction + "\n\n" + title + "\n" + description + "\n\n" + stagerGuardrails
```

i.e. `instruction\n\n<title>\n<description>\n\n<guardrails>`. title and description are interpolated
VERBATIM (title is single-line per planner §17.5; description may be multi-line — its internal newlines
survive). This mirrors payload.go's "diff is the verbatim tail" principle applied to two middle fields.

Defensive edge: empty description ⇒ `title\n\n\n<guardrails>` (an extra blank line). The planner
always supplies both (§17.5), so this is defensive only — no panic, covered by an edge test.

## §3 — Prompt-package conventions to mirror (read from system.go + payload.go)

1. **Constants carry NO trailing newline; the Build* function owns ALL inter-block newline placement.**
   (load-bearing convention from system.go/payload.go — the §17.6 blank-line topology lives in exactly
   one auditable place: BuildStagerTask.) Both `stagerInstruction` and `stagerGuardrails` end WITHOUT
   a trailing `\n`.
2. **Split static text into named constants when dynamic content is interleaved between them.**
   payload.go is the direct precedent: it splits `userInstructionReject` / `rejectionPreamble` /
   `rejectionEpilogue` because the rejected-subject LIST is interleaved between preamble and epilogue.
   stager.go splits `stagerInstruction` / `stagerGuardrails` because title+description are interleaved
   between them. Same pattern.
3. **Rich doc comments citing the PRD section + diagramming the assembly.** Every const and the Build
   function get a doc comment (see system.go's maturePromptHeader / BuildSystemPrompt comments).
4. **Fast-path return for the common case.** payload.go's normal path is `return userInstruction +
   "\n\n" + diff` (no Builder). BuildStagerTask is ALWAYS a single expression (no branches, no Builder)
   — there's only one path. Pure `+` concatenation; ZERO imports needed (no `strings`, no `fmt`).
5. **Minimal decoupled params.** BuildStagerTask takes ONLY `(title, description string)` — no config,
   no git, no examples (unlike BuildSystemPrompt's examples/hasMultiline/subjectTarget). The stager
   concept comes from the planner; the prompt layer takes it as plain strings.

## §4 — The backtick gotcha (the ONE real implementation trap for stager.go)

`stagerGuardrails` contains TWO backtick characters: `` `git add <path>` `` and `` `git apply --cached` ``.
A Go **backtick raw string literal** (`` `...` ``) CANNOT contain a backtick. So stagerGuardrails CANNOT
be a raw string. Use a **double-quoted `"..."` string literal** instead — in a double-quoted literal,
backticks are ordinary characters needing NO escaping. The 5-line wrapping is preserved with explicit
`\n` joins (concatenated `"...\n" +` literals), exactly preserving §17.6's verbatim line layout.

`stagerInstruction` has no backticks → could be either form, but a plain `"..."` double-quoted literal
is cleanest (single line). Do NOT use a raw string for it either (consistency; it has no special chars).

NOTE: the prompt package's existing constants that use backtick raw strings (maturePromptHeader,
antiReuseProhibition, fallbackPromptBody, rejectionPreamble) do so because they contain NO backticks.
stagerGuardrails is the FIRST constant in the package that contains backticks → first use of the
double-quoted-with-`\n` form here. This is correct and idiomatic Go.

## §5 — The em-dash gotcha (the ONE non-ASCII byte)

`stagerGuardrails` line 4 contains an EM-DASH "—" (U+2014) in "file contents — only update the index".
This is the same non-ASCII byte §17.1's `antiReuseProhibition` carries ("the STYLE to match — format").
system.go documents it explicitly in the const's doc comment. Do the same for stagerGuardrails: note
the em-dash in the doc comment (the ONLY non-ASCII byte in the stager prompt). In a double-quoted Go
string literal the UTF-8 em-dash bytes are literal (no escaping) — identical to how a raw string
would carry them. It MUST NOT be replaced with an ASCII hyphen "-" (verbatim §17.6 fidelity — same
rule system.go applies to antiReuseProhibition).

## §6 — No system prompt, no JSON contract, no parse (the simplest of the three decomposition prompts)

| Aspect | planner (S1) | stager (THIS, S2) | arbiter (S3) |
|---|---|---|---|
| system prompt const | YES (`plannerSystemPrompt`) | **NO** ("minimal/empty", orchestrator's concern) | YES |
| user payload / task | YES (`BuildPlannerUserPayload`) | YES (`BuildStagerTask`) — this is THE prompt | YES |
| JSON contract types | YES (`PlannerOutput`/`PlannerCommit`) | **NO** | YES |
| Parse function | YES (`ParsePlannerOutput`) | **NO** | YES |
| retry const | YES (`PlannerRetryInstruction`) | **NO** | (TBD S3) |

Why the stager is so much simpler:
- **No system prompt:** §17.6 says the prompt is "delivered as the user payload; system prompt
  minimal/empty." The stager is TOOLED (git access, repo-scoped); its behavioral guardrails live in the
  TASK prompt + are enforced STRUCTURALLY (tooled_flags §12.1; stagecoach owns all ref ops, §17.6's
  "enforced structurally"). So there is no defined system-prompt CONSTANT to export. The orchestrator
  (P3.M2.T3.S1) renders the tooled agent with an empty/minimal system prompt arg + this task payload.
  DO NOT invent a `StagerSystemPrompt` constant — §17.6 deliberately omits it.
- **No JSON contract:** the stager's output is "a text confirmation of paths staged" (free-form prose),
  NOT structured output. Per the work item: "No JSON contract (stager returns a text confirmation of
  paths staged, not structured output)." So there is NOTHING to parse — the orchestrator reads the
  stager's exit code (0 = success) + the staged paths via `git diff --cached --name-only` (the truth
  source is the INDEX, not the stager's prose). DO NOT add a `ParseStagerOutput`.
- **No retry const:** the stager's failure handling (§13.6.6: exit non-zero → retry once → treat as
  empty) is exit-code-driven, not parse-driven. No retry-instruction constant is needed.

So `BuildStagerTask` is the SOLE export. Total file surface: 2 private consts + 1 exported function +
a package doc comment. ZERO imports.

## §7 — The consumer contract (P3.M2.T3.S1, `internal/decompose/stager.go`)

Per `plan/002_a17bb6c8dc1d/architecture/decompose_architecture.md` (the "Four Agent Roles" table +
"Pipeline Flow" step 2a): the stager role is **tooled** ("stager | tooled | stage one concept's changes
(`git add`, hunk-stage) | exits 0; mutates index | reuses `prompt/stager.go`, `Render(RenderTooled)`").

The orchestrator's stager loop (P3.M2.T3.S1) will do, per concept[i]:
```go
task := prompt.BuildStagerTask(concept.Title, concept.Description)   // THIS task — the SOLE export
raw, exitErr := <render(RenderTooled) + execute the stager agent>     // P3.M2.T3.S1
// success = exitErr == nil (NOT a parse of `raw`); the index is the truth source
```
So the exported surface this task MUST provide is EXACTLY: `func BuildStagerTask(title, description
string) string`. Nothing else. `concept` is a `PlannerCommit` (Title, Description) from S1.

NOTE on parallel execution: S1 (`planner.go`) is being implemented IN PARALLEL with this task. S1
defines `PlannerCommit{Title, Description string}`. This task does NOT import planner.go or
decompose — it takes plain `(title, description string)`, so it is fully decoupled from S1's exact
struct layout (only the FIELD NAMES matter conceptually, and they're plain strings passed positionally).
Zero merge friction: stager.go is a standalone new file; planner.go (S1) and arbiter.go (S3) are
separate standalone new files in the same `prompt` package.

## §8 — Test conventions (mirror payload_test.go + system_test.go shapes)

`internal/prompt/stager_test.go` — `package prompt` (internal test, so the private consts
`stagerInstruction`/`stagerGuardrails` are visible; REUSE the package-level `near`/`suffix` helpers
already defined at the bottom of system_test.go — do NOT redeclare them, that's a duplicate-symbol
compile error).

Mirror these EXACT test shapes:
1. **`TestBuildStagerTask_CanonicalExact`** — an independently-derived `want` string for a known
   `(title, description)`, built from §17.6 (NOT from the implementation). Assert `got == want` with a
   `%q` diff on failure. This pins the entire blank-line topology byte-for-byte — the strongest guard.
   The `want` is:
   ```
   "Stage, but do NOT commit, all changes in this repository that match this concept:\n" +
   "\n" +
   title + "\n" +
   description + "\n" +
   "\n" +
   "Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply\n" +
   "only the relevant hunks via `git apply --cached`). Stage ONLY changes belonging to this\n" +
   "concept; leave unrelated changes unstaged. Do not commit, do not amend, do not push, do not\n" +
   "modify file contents — only update the index. When done, reply with the list of paths you\n" +
   "staged and stop."
   ```
2. **`TestBuildStagerTask_Properties`** — a `cases` table of structural invariants:
   - instruction line present + starts the output.
   - guardrails block present verbatim (the two backtick git commands; `<path>` literal token;
     "Stage ONLY changes belonging to this concept"; the hard-guardrails clause; "reply with the list
     of paths you staged and stop").
   - em-dash present (NOT ascii hyphen) in "file contents — only".
   - title interpolated, in order before description.
   - description interpolated, in order after title.
   - title is the verbatim value (a weird title with spaces/symbols survives).
   - description verbatim (multi-line description's internal newlines survive).
   - **anti-copy-paste guards** (the #1 risk): §17.1 mature elements ABSENT ("You are a commit
     message generator", "Output ONLY the commit message", anti-reuse block "CRITICAL: You MUST NOT
     copy", "Target ~"); §17.5 planner elements ABSENT ("You are a commit-planning assistant", the
     JSON contract "Respond with ONLY JSON", "Decompose these un-staged changes"). The stager prompt
     is self-contained (§17.6).
3. **`TestBuildStagerTask_EdgeCases`** — empty title, empty description, both empty (no panic; the
   assembly still produces a well-formed string); the nil=="" equivalence isn't applicable (strings
   aren't nilable) but empty-string interpolation is the defensive path.
4. (No parse tests — there is no parse function.)

## §9 — Imports, lint, scope boundaries

- **Imports:** ZERO. `BuildStagerTask` is pure `+`-concatenation of two consts + two params. No
  `strings`, no `fmt`, no `encoding/json` (unlike S1's planner.go). A .go file with no imports simply
  omits the `import` block. (This is the cleanest possible leaf module.)
- **go.mod / go.sum:** UNCHANGED (stdlib only — and in fact not even that; no imports at all).
- **Lint (.golangci.yml: errcheck/gosimple/govet/ineffassign/staticcheck/unused):** a pure
  string-returning function with no error returns, no goroutines, no assignments — nothing for any of
  the six enabled linters to flag. Clean by construction.
- **Scope boundaries (frozen / owned elsewhere — do NOT edit):**
  - `internal/prompt/system.go`, `payload.go`, `system_test.go`, `payload_test.go` — CONSUMED as the
    pattern templates + the `near`/`suffix` helper source; UNCHANGED.
  - `internal/prompt/planner.go` (S1, parallel) + `internal/prompt/arbiter.go` (S3, planned) —
    separate standalone new files in the same package; ZERO merge friction (stager.go is standalone).
  - `internal/decompose/stager.go` (P3.M2.T3.S1) — does NOT exist yet; THIS task does NOT create it.
  - `internal/provider/*`, `internal/git/*`, `internal/config/*` — UNCHANGED (this layer is decoupled
    from all of them; it takes plain strings).
  - No new types in any package, no new deps, no import cycle (prompt is a zero-dep leaf).

## §10 — Why this is the lowest-risk task in the entire decompose epic

A single ~6-line function (2 consts + 1 Build expression) that is a mechanical extension of payload.go's
interleave-constants-around-dynamic-content pattern. Zero imports, zero modifications, zero new types,
zero parse logic. The only residual uncertainty is the title↔description separator (§2) — resolved by a
defensible §17.6-faithful decision + a canonical-exact test, trivially adjustable later without
affecting any other package. The exported surface (`BuildStagerTask(title, description) string`) exactly
matches the consumer (P3.M2.T3.S1). High one-pass confidence.
