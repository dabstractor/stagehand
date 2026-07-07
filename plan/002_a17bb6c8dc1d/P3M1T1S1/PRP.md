---
name: "P3.M1.T1.S1 — Implement planner system prompt + JSON contract + assembly in internal/prompt/planner.go (PRD §17.5, §13.6.2, FR-M3)"
description: |

  CREATE ONE NEW FILE `internal/prompt/planner.go` in the existing `prompt` package: the **planner**
  prompt half of stagecoach's v2 multi-commit decomposition (PRD §17.5). The planner is a **bare** agent
  that receives the full working-tree diff (P2.M2.T2.S2) + the §17.1 style examples, decides whether the
  changeset is ONE commit or SEVERAL, partitions it into logical units, and — only if one — emits the
  message. Unlike v1's free-form commit messages (§17.4 raw output), the planner emits STRUCTURED JSON
  (a list of concepts), so a JSON output contract + a robust parse are justified here (§17.5).

  CONTRACT (P3.M1.T1.S1, verbatim from the work item):
    1. RESEARCH NOTE: §17.5 specifies the planner prompt verbatim. The planner is bare and receives the
       full working-tree diff + style examples. Its JSON output contract:
       `{"count": N, "single": bool, "commits": [{"title": "...", "description": "..."}], "message": "..."}`.
       message present iff single==true. Forced-count mode prepends 'Produce EXACTLY N commits...'.
       Retry instruction: 'Respond with ONLY the JSON object described, no other text.' The style
       examples are the same RecentMessages-based examples from prompt/system.go.
    2. INPUT: The existing prompt package patterns (prompt/system.go, prompt/payload.go — string
       constants WITHOUT trailing newlines, assembly functions own inter-block newlines).
    3. LOGIC: Create internal/prompt/planner.go. Define plannerSystemPrompt constant (verbatim from
       §17.5 — role, rules, JSON contract). Define plannerUserPayload(diff, forcedCount) string —
       assembles the user instruction + diff (prepends forced-count directive if forcedCount>0). Define
       PlannerOutput struct with json tags: Count int, Single bool, Commits []PlannerCommit (Title,
       Description), Message string. Define ParsePlannerOutput(raw) (PlannerOutput, error) —
       json.Unmarshal with brace-balanced fallback (reuse provider.parseJSON pattern or
       provider.extractJSONObject).
    4. OUTPUT: prompt/planner.go exports BuildPlannerSystemPrompt(), BuildPlannerUserPayload(), and
       ParsePlannerOutput(). Consumed by decompose/planner.go (P3.M2.T2.S1).
    5. DOCS: none — prompt constants are self-documenting via PRD §17.5 reference.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `internal/prompt/system.go`, `payload.go` — CONSUMED as the pattern template; UNCHANGED.
    - `internal/provider/parse.go` — CONSUMED as the JSON-extraction algorithm template; UNCHANGED.
    - `internal/git/*` — UNCHANGED (WorkingTreeDiff is P2.M2.T2.S2, parallel; RecentMessages is v1).
    - `internal/decompose/*` — does NOT exist yet; THIS task does NOT create it (P3.M2.*).
    - go.mod / go.sum — UNCHANGED (stdlib only: encoding/json + fmt + strings).
    - Sibling prompt files: P3.M1.T1.S2 (stager.go) + P3.M1.T1.S3 (arbiter.go) ALSO add new files to
      internal/prompt/. planner.go is a NEW standalone file → zero merge friction.

  DELIVERABLES (2 new files, 0 modifications):
    CREATE internal/prompt/planner.go — `plannerSystemPrompt` const (verbatim §17.5, ends before
      `<style examples>`), `plannerUserInstruction` + `PlannerRetryInstruction` consts (verbatim §17.5),
      `PlannerCommit` + `PlannerOutput` structs (json tags), `BuildPlannerSystemPrompt(examples)`,
      `BuildPlannerUserPayload(diff, forcedCount)`, `ParsePlannerOutput(raw)`, private
      `extractJSONObject` (reimplemented brace-balanced JSON extractor).
    CREATE internal/prompt/planner_test.go — canonical-exact + properties + parse + edge-case tests.

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing tests still pass (purely
  additive — no existing file changes); the planner prompt is byte-faithful to §17.5; ParsePlannerOutput
  parses clean JSON, JSON-in-prose (brace-balanced fallback), and JSON-in-code-fence, and returns a
  non-nil error on garbage.

---

## Goal

**Feature Goal**: Implement the planner-agent prompt-construction + JSON-contract-parse layer for
multi-commit decomposition (PRD §17.5 / §13.6.2 / FR-M3) as a self-contained, dependency-free module in
the existing `internal/prompt` package. This is the prompt analogue of v1's `system.go`/`payload.go` —
the verbatim §17.5 planner system prompt (role + rules + JSON contract) with the §17.1 style examples
appended, the §17.5 user payload (with forced-count prepend), the §17.5 retry instruction, the
`PlannerOutput`/`PlannerCommit` JSON-contract types, and a robust `ParsePlannerOutput` that does
whole-string `json.Unmarshal` with a brace-balanced fallback (mirroring provider/parse.go's algorithm).

**Deliverable** (2 new files, 0 modifications):
1. `internal/prompt/planner.go` — the planner prompt module: `plannerSystemPrompt` const (verbatim
   §17.5 system prompt, ending at "...find the exact changes." with NO trailing newline — `<style
   examples>` is the runtime placeholder, NOT part of the const); `plannerUserInstruction` const
   ("Decompose these un-staged changes into commits:"); `PlannerRetryInstruction` exported const
   ("Respond with ONLY the JSON object described, no other text."); `PlannerCommit` + `PlannerOutput`
   structs (json tags: title/description/count/single/commits/message); `BuildPlannerSystemPrompt(
   examples []string) string` (const + blank + style examples in the `---` format); `BuildPlannerUser
   Payload(diff string, forcedCount int) string` (forced-count directive prepended iff forcedCount>0);
   `ParsePlannerOutput(raw string) (PlannerOutput, error)` (whole-Unmarshal → brace-balanced fallback →
   error); private `extractJSONObject(s) (string, bool)` (reimplemented from provider/parse.go).
2. `internal/prompt/planner_test.go` — canonical-exact test (independently-derived `want` for both the
   system prompt and the user payload, normal + forced-count), properties table (anti-copy-paste guards
   pinning that §17.1 mature elements are ABSENT, `---` count == len(examples), forced-count presence),
   ParsePlannerOutput table (clean JSON, JSON-in-prose, JSON-in-code-fence, malformed→error, empty→error,
   `commits:null`→nil slice, missing message + single:false→"", extra fields ignored), and edge cases.

**Success Definition**:
- `BuildPlannerSystemPrompt([]string{"feat: a"})` returns a string that is byte-faithful to §17.5:
  starts with "You are a commit-planning assistant.", contains the 4 rules, the JSON-contract line
  (with its `<int>`/`<bool>` placeholders VERBATIM), the single/message clause, and ends with the style
  examples in `---\n<msg>\n` format. NO §17.1 mature elements leak ("You are a commit message
  generator", anti-reuse block, "Target ~N characters").
- `BuildPlannerUserPayload("DIFF", 0)` == `"Decompose these un-staged changes into commits:\n\nDIFF"`.
- `BuildPlannerUserPayload("DIFF", 3)` == `"Produce EXACTLY 3 commits from these changes (do not
  reconsider the count):\nDecompose these un-staged changes into commits:\n\nDIFF"`.
- `ParsePlannerOutput(`{"count":2,"single":false,"commits":[...]}`)` → `(PlannerOutput{Count:2,
  Single:false, Commits:[...]}, nil)`.
- `ParsePlannerOutput(`Here is the plan: {"count":1,"single":true,"commits":[...],"message":"x"} hope
  this helps`)` → succeeds via the brace-balanced fallback (Single:true, Message:"x").
- `ParsePlannerOutput("not json at all")` → `(PlannerOutput{}, non-nil error)`.
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 2 new untracked files only.

## User Persona

**Target User**: the decompose planner agent invocation (internal code, P3.M2.T2.S1) and, by extension,
the end user running `stagecoach` on an un-staged working tree to get multiple logically-coherent commits.
The planner prompt is NOT user-facing CLI text; it is the system prompt + user payload piped to the bare
planner agent (PRD §13.6.2 / §17.5).

**Use Case**: when decomposition activates (nothing staged, working tree dirty — §13.6.1), the
orchestrator (P3.M2.T2.S1) calls `WorkingTreeDiff` (P2.M2.T2.S2) + `RecentMessages` (v1), assembles the
planner prompt via `BuildPlannerSystemPrompt` + `BuildPlannerUserPayload`, invokes the planner agent
(bare mode via `provider.Render`), and parses its JSON via `ParsePlannerOutput`. If parse fails, one
retry with `PlannerRetryInstruction` (§13.6.6). On success: `single==true` → single-shortcut (§13.6.4)
using `out.Message`; else loop over `out.Commits` into the stager/message/commit pipeline.

**Pain Points Addressed**: the planner needs (a) a verbatim-faithful system prompt whose JSON contract
is unambiguous so the model emits parseable output, and (b) a robust parser tolerant of the two real
model failure modes (JSON wrapped in prose; JSON wrapped in a code fence despite "no code fences"). v1's
raw-output parse (`provider.ParseOutput`) is the wrong tool here — it extracts a single string field,
not a structured object; `ParsePlannerOutput` is purpose-built for the `PlannerOutput` contract.

## Why

- **Closes the planner half of PRD §17.5 / §13.6.2 / FR-M3 at the prompt layer.** FR-M3: the planner
  "Receives the full working-tree diff snapshot (with binary placeholders per FR3c) plus the style
  examples from §9.3" and returns `JSON {count, single, commits:[{title,description}], message?}`. This
  task is the literal prompt-construction + JSON-parse implementation of that role — the first of the
  three decomposition prompts (planner §17.5 ← THIS; stager §17.6 = P3.M1.T1.S2; arbiter §17.7 =
  P3.M1.T1.S3).
- **Mechanical extension of the established prompt-package pattern.** system.go (S1/S2) already proved
  the convention: verbatim PRD constants without trailing newlines; a Build* function owns the
  blank-line topology; rich doc comments cite the PRD section + diagram the assembly. planner.go is the
  same pattern applied to §17.5 — no new architectural concept, just a new prompt + a typed JSON parse.
  The JSON-extraction algorithm already exists in provider/parse.go (`extractJSONObject`); planner.go
  reimplements that ~30-line pure function privately (see findings §5) — no new dependency, no scope
  bleed into provider.
- **Unblocks the decompose core (P3.M2).** P3.M2.T2.S1 (the planner agent call) consumes EXACTLY this
  task's exported surface (`BuildPlannerSystemPrompt`, `BuildPlannerUserPayload`, `ParsePlannerOutput`,
  `PlannerRetryInstruction`, `PlannerOutput`, `PlannerCommit`). Nothing in P3.M2 can run until this
  prompt+parse layer exists. It is the first dependency of the entire decompose pipeline.
- **Lowest-risk, maximal-reuse, backward-compatible.** Two NEW files; ZERO modifications to any existing
  file. The prompt package GAINS exports (additive — no existing caller or test breaks). go.mod/go.sum
  untouched (stdlib only). No import cycle (provider does not import prompt; planner.go imports only
  encoding/json + fmt + strings).

## What

One new file `internal/prompt/planner.go` (package `prompt`) exporting three functions, one const, and
two types, plus a private const + a private brace-balanced JSON extractor; and one new test file
`internal/prompt/planner_test.go`. No new types in any other package. No new dependencies. No caller
wiring (that is P3.M2.T2.S1). Specifically:

- **`plannerSystemPrompt`** (private const, no trailing newline): the verbatim §17.5 system prompt from
  "You are a commit-planning assistant." through "The "description" must be specific enough that a
  staging agent can find the exact changes." — INCLUDING the literal JSON-contract line (with its
  `<int>`/`<bool>`/`<short concept>` tokens, which are part of the format example shown to the model, NOT
  runtime placeholders). `<style examples>` is NOT in the constant (runtime placeholder).
- **`plannerUserInstruction`** (private const): `"Decompose these un-staged changes into commits:"`.
- **`PlannerRetryInstruction`** (exported const): `"Respond with ONLY the JSON object described, no
  other text."` — owned by the prompt package so the verbatim §17.5 retry text lives in one auditable
  place; consumed by decompose/planner.go's one-retry path (P3.M2.T2.S1, §13.6.6).
- **`PlannerCommit`** + **`PlannerOutput`** (exported structs, json tags): the §17.5 JSON contract.
- **`BuildPlannerSystemPrompt(examples []string) string`**: `plannerSystemPrompt + "\n\n" +` the style
  examples in the `---\n<msg>\n` format (same as system.go).
- **`BuildPlannerUserPayload(diff string, forcedCount int) string`**: normal path =
  `plannerUserInstruction + "\n\n" + diff`; forced path (forcedCount>0) prepends
  `fmt.Sprintf("Produce EXACTLY %d commits from these changes (do not reconsider the count):",
  forcedCount) + "\n"`.
- **`ParsePlannerOutput(raw string) (PlannerOutput, error)`**: TrimSpace → whole `json.Unmarshal` into
  `PlannerOutput` → on failure, `extractJSONObject` brace-balanced fallback + retry Unmarshal → else error.
- **`extractJSONObject(s) (string, bool)`** (private): reimplemented from provider/parse.go — first `{`
  to matching depth-0 `}`, string/escape-aware.

### Success Criteria

- [ ] `internal/prompt/planner.go` defines `plannerSystemPrompt` byte-faithful to §17.5 (verified by an
      independently-derived canonical-exact test), with NO trailing newline and NO `<style examples>`
      token inside the constant.
- [ ] `BuildPlannerSystemPrompt(examples)` == `plannerSystemPrompt + "\n\n" +` (for each ex: `"---\n" +
      ex + "\n"`); nil/empty examples ⇒ no `---` lines, no panic.
- [ ] `BuildPlannerUserPayload(diff, 0)` == `plannerUserInstruction + "\n\n" + diff`;
      `BuildPlannerUserPayload(diff, N>0)` == `forcedDirective + "\n" + plannerUserInstruction + "\n\n"
      + diff`; `forcedCount < 0` treated as normal (== 0).
- [ ] `PlannerOutput`/`PlannerCommit` json tags map 1:1 to the §17.5 contract; `ParsePlannerOutput`
      parses clean JSON, JSON-in-prose, and JSON-in-code-fence; returns `(PlannerOutput{}, non-nil
      error)` on garbage/empty; tolerates `commits:null` (nil slice) and extra unknown fields.
- [ ] `PlannerRetryInstruction == "Respond with ONLY the JSON object described, no other text."`.
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 2 new untracked files.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the verbatim §17.5 text
(findings §1 — captured character-for-character, with the CRITICAL literal-vs-placeholder distinction
spelled out); the exact prompt-package conventions to mirror (findings §3 — no-trailing-newline consts,
Build-owns-newlines, `---`-example format, rich doc comments, minimal decouled params); the struct +
parse algorithm (findings §4/§5 — json tags, two-attempt Unmarshal with brace-balanced fallback, the
reimplement-don't-import decision); the consumer contract (findings §6 — exactly what P3.M2.T2.S1 needs,
so the exported surface is right); the test conventions (findings §7 — canonical-exact, properties
table, anti-copy-paste guards, the shared `near`/`suffix` helpers already in system_test.go); and the
scope boundaries (findings §9 — zero modifications, stdlib only, no provider import). No decompose/git
knowledge required — the contract is fully self-contained at the prompt layer.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (verbatim §17.5 text + conventions + the JSON decision)
- docfile: plan/002_a17bb6c8dc1d/P3M1T1S1/research/findings.md
  why: §1 the VERBATIM §17.5 system prompt (the plannerSystemPrompt constant) + the CRITICAL literal-vs-
       placeholder distinction (the JSON-contract line's <int>/<bool> tokens are LITERAL; only <style
       examples> is a runtime placeholder); §2 the user-payload assembly decision (forced-count prepend
       topology — the one ambiguous point, pinned by a test) + the retry const; §3 the prompt-package
       conventions to mirror (no-trailing-newline, Build-owns-newlines, "---"-example format, NO
       hasMultiline/subjectTarget for the planner); §4 the struct + json tags; §5 the ParsePlannerOutput
       algorithm + the REIMPLEMENT-DON'T-IMPORT decision for extractJSONObject; §6 the consumer contract
       (P3.M2.T2.S1 — exact exported surface); §7 test conventions; §8 imports/lint; §9 scope boundaries.
  critical: §1 (the verbatim text + the literal-vs-placeholder distinction — the #1 implementation trap);
            §5 (reimplement extractJSONObject privately in planner.go — do NOT import provider, do NOT
            export provider's copy); §3.5 (anti-copy-paste: §17.1 mature elements MUST be ABSENT).

# MUST READ — the FILES TO MIRROR (the pattern templates; UNCHANGED — read only, do not edit)
- file: internal/prompt/system.go
  section: the package doc + the const-declaration style (maturePromptHeader/antiReuseProhibition:
           backtick raw strings, NO trailing newline, a comment citing §17.1 + noting non-ASCII);
           BuildSystemPrompt (the assembly pattern: strings.Builder, WriteString/WriteByte, the
           "---\n"+ex+"\n" example loop, the explicit '\n' placement between blocks);
           BuildFallbackPrompt (the S2 precedent: a SECOND self-contained prompt in the SAME package —
           planner.go is the THIRD, same pattern).
  why: this IS the template. planner.go is BuildSystemPrompt's §17.5 sibling: a verbatim const +
       a Build function that owns the blank-line topology + the "---" example format. Copy the STYLE
       (doc comments, no-trailing-newline, Builder assembly), NOT the §17.1 TEXT (§17.5 is separate).
  pattern: define `plannerSystemPrompt` as a backtick const ending at "...find the exact changes." with
           NO trailing newline; BuildPlannerSystemPrompt uses strings.Builder: WriteString(plannerSystem
           Prompt), WriteByte('\n'), WriteByte('\n'), then the `for _, ex := range examples { WriteString
           ("---\n"); WriteString(ex); WriteByte('\n') }` loop (identical to system.go's example loop).
  gotcha: the planner does NOT use §17.1's anti-reuse block or multi-line rule or subject-target line —
          §17.5 has its own inline "but NEVER reuse wording" rule and NO target line. Do NOT copy those
          §17.1 elements. (A test pins their absence.)

- file: internal/prompt/payload.go
  section: the const style (userInstruction/userInstructionReject: plain "..." string literals, NO
           trailing newline); BuildUserPayload (the assembly: fast-path return for the common case,
           strings.Builder for the multi-block case; the diff appended VERBATIM as the tail).
  why: the user-payload template. BuildPlannerUserPayload mirrors BuildUserPayload's structure: a
       private instruction const + a Build function that owns the `instruction + "\n\n" + diff` topology
       and appends diff verbatim. The forced-count prepend is the new wrinkle.
  pattern: `plannerUserInstruction = "Decompose these un-staged changes into commits:"` (trailing COLON,
           matching payload.go's §17.3 normal-instruction-ends-with-colon precedent); normal path =
           `return plannerUserInstruction + "\n\n" + diff` (fast path, mirrors payload.go's normal
           return); forced path = `return forcedDirective + "\n" + plannerUserInstruction + "\n\n" + diff`.

# MUST READ — the JSON-extraction algorithm template (UNCHANGED — reimplement privately, do NOT import)
- file: internal/provider/parse.go
  section: extractJSONObject (lines ~99–135 — the brace-balanced state machine: first '{', depth counter,
           inString flag, escaped flag for `\"` inside strings; byte-scanning is UTF-8-safe); parseJSON
           (lines ~76–97 — whole-string Unmarshal → extractJSONObject fallback → field extraction).
  why: ParsePlannerOutput needs EXACTLY this algorithm (the planner output may be JSON wrapped in prose
           or a code fence). Reimplement extractJSONObject VERBATIM (copy the ~30-line function) as a
           PRIVATE function in planner.go. Do NOT import provider (layering smell; prompt is a leaf pkg).
  pattern: copy provider's extractJSONObject body byte-for-byte into a private `extractJSONObject` in
           planner.go; ParsePlannerOutput does `s := strings.TrimSpace(raw)`; Attempt 1:
           `json.Unmarshal([]byte(s), &out)`; on err → Attempt 2: `sub, found := extractJSONObject(s)`
           then `json.Unmarshal([]byte(sub), &out)`; both fail → return error wrapping the last err.
  gotcha: provider's extractJSONObject is UNEXPORTED (lowercase) — you CANNOT call it from the prompt
          package; you MUST reimplement it. Reimplementing ~30 lines of a pure function is explicitly
          sanctioned by the work item ("reuse provider.parseJSON PATTERN"). Do NOT export provider's copy.

# MUST READ — the test exemplars (the test patterns to mirror in planner_test.go)
- file: internal/prompt/system_test.go
  section: TestBuildSystemPrompt_CanonicalExact (independently-derived `want` string via concatenated
           "...\n" + ... literals, asserted with %q diff on failure); TestBuildSystemPrompt_Properties
           (a `cases` slice of {name, inputs, check func(t, p)}); TestBuildFallbackPrompt_Properties (the
           ANTI-COPY-PASTE guards: a table of {name, needle, mustExist} asserting §17.1 elements ABSENT);
           the package-level helpers `near(s, needle)` + `suffix(s, n)` at the BOTTOM of the file.
  why: the EXACT test shapes planner_test.go mirrors. The canonical-exact test is the strongest guard
       against §17.5 text drift; the anti-copy-paste table pins that §17.1 mature elements are ABSENT
       (the #1 risk); the parse table mirrors a properties/cases table.
  pattern: write TestBuildPlannerSystemPrompt_CanonicalExact with an independently-derived `want` (build
           it from §17.5, NOT from the implementation); a TestBuildPlannerSystemPrompt_Properties table
           with anti-copy-paste cases ("You are a commit message generator" ABSENT, antiReuseProhibition
           ABSENT, "Target ~" ABSENT, "---" count == len(examples), JSON-contract line PRESENT); REUSE
           near/suffix (do NOT redeclare — they're in system_test.go, same package).

- file: internal/prompt/payload_test.go
  section: TestBuildUserPayload_NormalCanonicalExact + TestBuildUserPayload_RejectionCanonicalExact
           (two canonical-exact tests, one per payload path); TestBuildUserPayload_Properties (the
           "diff is the exact tail" + colon/period invariants).
  why: the user-payload test shape. planner_test.go mirrors this: a canonical-exact test for the NORMAL
       payload (forcedCount==0) AND one for the FORCED payload (forcedCount>0), plus a properties table.
  pattern: TestBuildPlannerUserPayload_NormalCanonicalExact asserts `got == "Decompose these un-staged
           changes into commits:\n\n" + diff`; TestBuildPlannerUserPayload_ForcedCanonicalExact asserts
           `got == "Produce EXACTLY 3 commits from these changes (do not reconsider the count):\nDecompose
           these un-staged changes into commits:\n\n" + diff`.

# MUST READ — the design reference (role + contract + the consumer)
- docfile: plan/002_a17bb6c8dc1d/architecture/decompose_architecture.md
  section: "The Four Agent Roles" table (planner: bare, JSON {count,single,commits,message?}, reuses
           prompt/planner.go); "Single-Commit Shortcut (§13.6.4)" (message present iff single==true);
           "Failure Handling" (planner unparseable/fails ⇒ surface error, exit non-rescue — the retry).
  why: confirms the planner's output contract + the single-shortcut semantics + that THIS task's exports
       are consumed by decompose/planner.go (P3.M2.T2.S1). NOTE the doc lists 4 new prompt/planner.go-
       style files; THIS task implements ONLY planner.go (§17.5).

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §17.5 (the planner prompt — system prompt sketch, user payload, forced-count, retry)
  why: §17.5 is the verbatim source for plannerSystemPrompt, the user payload, the forced-count prepend,
       and the retry instruction. The "(sketch)" label does NOT weaken this — the work item says "verbatim
       from §17.5", and §17.1/§17.2 set the precedent that PRD §17.x code blocks are the authoritative text.
- url: PRD.md §13.6.2 (the four agent roles — planner output contract) + §13.6.4 (single-shortcut: message
       present iff single==true) + §13.6.6 (planner-fails handling ⇒ the one retry)
  why: the runtime semantics the prompt + parse layer must serve.
```

### Current Codebase tree (relevant slice)

```bash
internal/prompt/
  system.go                 # READ: the §17.1 system-prompt template (const style + Build assembly +
                            #   "---"-example loop + rich doc comments). UNCHANGED.
  payload.go                # READ: the §17.3 user-payload template (instruction const + Build fast-path
                            #   + diff-as-verbatim-tail). UNCHANGED.
  system_test.go            # READ: the canonical-exact + properties + anti-copy-paste test shapes; ALSO
                            #   defines the package-level helpers near() + suffix() (REUSE — do NOT redeclare).
  payload_test.go           # READ: the two-path canonical-exact + properties test shape (normal/rejection
                            #   ⇒ normal/forced for the planner).
internal/provider/
  parse.go                  # READ: extractJSONObject + parseJSON (the JSON-extraction algorithm template;
                            #   REIMPLEMENT privately in planner.go — do NOT import). UNCHANGED.
go.mod / go.sum             # UNCHANGED (stdlib only: encoding/json + fmt + strings).
.golangci.yml               # READ: linter config (errcheck/gosimple/govet/ineffassign/staticcheck/unused).
```

### Desired Codebase tree with files to be added

```bash
internal/prompt/planner.go          # NEW — the planner prompt module (package prompt):
                                     #   const plannerSystemPrompt        (verbatim §17.5, no trailing \n)
                                     #   const plannerUserInstruction     ("Decompose these un-staged changes into commits:")
                                     #   const PlannerRetryInstruction    ("Respond with ONLY the JSON object described, no other text.")
                                     #   type PlannerCommit               (Title, Description; json tags)
                                     #   type PlannerOutput               (Count, Single, Commits, Message; json tags)
                                     #   func BuildPlannerSystemPrompt(examples []string) string
                                     #   func BuildPlannerUserPayload(diff string, forcedCount int) string
                                     #   func ParsePlannerOutput(raw string) (PlannerOutput, error)
                                     #   func extractJSONObject(s string) (string, bool)   [private, reimplemented]
internal/prompt/planner_test.go      # NEW — canonical-exact (system prompt + normal/forced user payload)
                                     #   + properties (anti-copy-paste guards, "---" count, JSON-contract present)
                                     #   + ParsePlannerOutput table (clean/prose/fence/malformed/empty/null/extra)
                                     #   + edge cases (nil examples, negative forcedCount, empty diff).
# go.mod/go.sum UNCHANGED. system.go/payload.go/provider/parse.go UNCHANGED. 0 modifications.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (literal-vs-placeholder — findings §1): inside plannerSystemPrompt, the JSON-contract line
//   {"count": <int>, "single": <bool>, "commits": [{"title": "<short concept>", ...}, ...]}
// is LITERAL text (shown to the model as the output-format example). The <int>/<bool>/<short concept>/
// <precisely which files/hunks belong here, by path>/<the full commit message> tokens are PART of the
// instructive example — they stay in the constant VERBATIM. ONLY the trailing "<style examples>" line
// is a runtime placeholder; it is NOT in the constant (BuildPlannerSystemPrompt appends the examples).
// The constant ENDS at: `The "description" must be specific enough that a staging agent can find the exact changes.`
// with NO trailing newline. This mirrors system.go's rule that the "<...>" annotation for examples is
// structural (system.go EXCLUDES "(up to 20, ≤100 lines total)" as structural); here the JSON tokens are
// INSTRUCTIVE (kept), only "<style examples>" is structural (excluded).

// CRITICAL (anti-copy-paste — findings §3.5, the #1 risk per S2's test): §17.5 is a SEPARATE, self-
// contained prompt. It MUST NOT contain §17.1's mature-prompt elements: NOT "You are a commit message
// generator" (§17.5 says "You are a commit-planning assistant"), NOT the anti-reuse block (§17.5 has its
// own inline "but NEVER reuse wording" in the rules), NOT the multi-line rule, NOT "Target ~N characters".
// A properties-table test pins each absence (mirror TestBuildFallbackPrompt_Properties's ABSENT cases).

// CRITICAL (no-trailing-newline + Build-owns-newlines — findings §3.1): ALL string constants
// (plannerSystemPrompt, plannerUserInstruction, PlannerRetryInstruction) are defined WITHOUT trailing
// newlines. The Build* functions own ALL inter-block newline placement. This is the load-bearing
// convention from system.go/payload.go — the §17.5 blank-line topology must live in exactly one auditable
// place (the Build function), not scattered across constants.

// CRITICAL (reimplement extractJSONObject, do NOT import provider — findings §5): provider/parse.go's
// extractJSONObject is UNEXPORTED (lowercase) — you CANNOT call it from the prompt package. Reimplement
// the ~30-line function VERBATIM as a private `extractJSONObject` in planner.go. Do NOT export
// provider's copy (scope creep into provider). Do NOT import provider into prompt (layering smell; the
// prompt package is a zero-internal-dep leaf — keep it that way). The work item sanctions this:
// "reuse provider.parseJSON PATTERN or provider.extractJSONObject" — "pattern" = same algorithm.

// GOTCHA (ParsePlannerOutput returns a NON-NIL error on parse failure — findings §5): do NOT swallow
// parse failures (returning a zero PlannerOutput with nil error would hide the problem). Return
// (PlannerOutput{}, fmt.Errorf("planner output: not valid JSON: %w", err)) so decompose/planner.go can
// trigger the ONE retry (§13.6.6: "Planner fails / returns unparseable output: surface the error").
// The retry is the CALLER's job (P3.M2.T2.S1 re-Executes with PlannerRetryInstruction); ParsePlannerOutput
// only parses.

// GOTCHA (the retry instruction is OWNED by planner.go, not the user payload — findings §2): §17.5's
// "Respond with ONLY the JSON object described, no other text." is the user-payload the caller sends on
// the SECOND planner agent call (not part of the FIRST call's user payload). Define it as the exported
// const `PlannerRetryInstruction` so the verbatim §17.5 text lives in one place and P3.M2.T2.S1 can use
// it without hardcoding. BuildPlannerUserPayload does NOT reference it.

// GOTCHA (forced-count prepend topology — findings §2, the one ambiguous point): §17.5 says forced-count
// "prepends" the directive but does not specify the separator. DECISION: forcedCount>0 ⇒
//   forcedDirective + "\n" + plannerUserInstruction + "\n\n" + diff
// (the two colon-ending instructions on consecutive lines, then ONE blank line, then the diff). This
// preserves the normal payload's `instruction\n\n<diff>` tail topology verbatim. Pin it with a canonical-
// exact test. forcedCount <= 0 ⇒ normal (negative treated as 0, defensive).

// GOTCHA (shared test helpers near()/suffix() — findings §7.6): they are ALREADY defined at the BOTTOM of
// system_test.go (same package `prompt`). Do NOT redeclare them in planner_test.go (duplicate-symbol
// compile error). Use them directly. The planner_test.go file is `package prompt` (internal test) so the
// unexported plannerSystemPrompt/plannerUserInstruction/extractJSONObject ARE visible to the tests.

// GOTCHA (Message is ALWAYS present on the struct — findings §4): json.Unmarshal of output omitting
// "message" leaves out.Message == "" (zero value). The "message present iff single==true" rule is a
// CALLER contract (decompose checks `out.Single && out.Message != ""`), NOT a struct-level constraint —
// a Go struct field cannot be "conditionally present". Do NOT add validation inside ParsePlannerOutput
// that errors when single==false && message!="" — the model's output is trusted at this layer; the
// orchestrator owns the single-shortcut decision (§13.6.4).

// GOTCHA (backtick raw string for plannerSystemPrompt): the §17.5 text contains double quotes (the JSON
// contract line: `"title"`, `"description"`, `"message"`) and backslashes are absent — so a backtick
// raw string literal is correct and clean (no escaping needed), exactly like system.go's
// maturePromptHeader/antiReuseProhibition. Do NOT use a double-quoted "..." literal (would need to
// escape every "). §17.5 is all-ASCII (no em-dash) so no encoding concern.

// GOTCHA (extractJSONObject handles code-fenced JSON for free): the planner prompt says "no code fences"
// but models sometimes emit ```json\n{...}\n``` anyway. Because extractJSONObject finds the FIRST '{' and
// scans to the balanced '}', it naturally extracts the JSON object out of a fenced block (the fence
// markers are prose outside the braces). No separate fence-stripping is needed — but a TEST must pin
// this (TestParsePlannerOutput_JSONInCodeFence), because it is a real model failure mode (§13.6.6 retry).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/prompt/planner.go — package prompt

// PlannerCommit is one partitioned concept from the planner (§17.5 JSON contract).
type PlannerCommit struct {
	Title       string `json:"title"`       // "<short concept>" — a short label for the concept.
	Description string `json:"description"` // "<precisely which files/hunks belong here, by path>" — staging instructions.
}

// PlannerOutput is the planner's full JSON response (§17.5). Message is present iff Single==true
// (the single-commit shortcut, §13.6.4); when Single==false it is the zero value "". The caller
// (decompose/planner.go) enforces the single⇔message contract, NOT this struct.
type PlannerOutput struct {
	Count   int             `json:"count"`   // N (== len(Commits); ==1 iff Single)
	Single  bool            `json:"single"`  // true ⇒ single-commit shortcut (§13.6.4)
	Commits []PlannerCommit `json:"commits"` // the partition (1..N); nil if the model emitted "commits":null
	Message string          `json:"message"` // the full commit message; present iff Single==true
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/prompt/planner.go — constants (verbatim §17.5)
  - ADD a package-level doc comment (mirror system.go's: cite PRD §17.5, explain the planner role, note
    that constants carry no trailing newline + Build* own inter-block newlines, note §17.5 is all-ASCII).
  - DEFINE `plannerSystemPrompt` (private const, backtick raw string): the verbatim §17.5 system prompt
    from "You are a commit-planning assistant." through `The "description" must be specific enough that
    a staging agent can find the exact changes.` — INCLUDING the literal JSON-contract line (with its
    <int>/<bool>/<short concept> tokens). NO trailing newline. NO "<style examples>" token (runtime
    placeholder). See findings §1 for the exact text.
  - DEFINE `plannerUserInstruction` (private const): `"Decompose these un-staged changes into commits:"`
    (trailing COLON — see findings §2 + payload.go precedent). NO trailing newline.
  - DEFINE `PlannerRetryInstruction` (exported const): `"Respond with ONLY the JSON object described, no
    other text."` (verbatim §17.5). NO trailing newline. Add a doc comment: this is the user payload for
    the caller's ONE retry after a parse failure (§13.6.6); owned here so the verbatim text is auditable.
  - NAMING: lowercase for private (plannerSystemPrompt, plannerUserInstruction); PascalCase for exported
    (PlannerRetryInstruction). Mirror system.go's const-naming.
  - GOTCHA: backtick raw string for plannerSystemPrompt (the JSON line has many double quotes; backticks
    avoid escaping). Do NOT include "<style examples>".

Task 2: CREATE internal/prompt/planner.go — PlannerCommit + PlannerOutput structs
  - DEFINE the two structs exactly as in "Data models and structure" above (json tags: title/description/
    count/single/commits/message). Add doc comments citing §17.5 + the single⇔message contract note.
  - GOTCHA: Message is always present on the struct (zero "" when absent); the caller enforces
    single⇔message. Do NOT add in-struct validation.

Task 3: CREATE internal/prompt/planner.go — BuildPlannerSystemPrompt(examples []string) string
  - SIGNATURE: `func BuildPlannerSystemPrompt(examples []string) string` (NO hasMultiline, NO
    subjectTarget — §17.5 has neither; see findings §3.4).
  - BODY (mirror system.go's BuildSystemPrompt assembly + example loop):
      var b strings.Builder
      b.WriteString(plannerSystemPrompt)
      b.WriteByte('\n'); b.WriteByte('\n')        // one blank line before the style examples
      for _, ex := range examples {
          b.WriteString("---\n"); b.WriteString(ex); b.WriteByte('\n')   // one "---" BEFORE each message
      }
      return b.String()
  - GOTCHA: nil/empty examples ⇒ the loop body never runs ⇒ no "---" lines, no panic (mirror
    system.go's TestBuildSystemPrompt_EmptyExamples defensive behavior). The result is
    plannerSystemPrompt + "\n\n" (a trailing blank line) — acceptable and matches the §17.5 topology
    (the "<style examples>" section is simply empty). Document this in the doc comment.
  - DOC COMMENT: cite §17.5, diagram the assembly topology (const + blank + "---"-examples), note the
    examples are the SAME RecentMessages-based format as system.go's BuildSystemPrompt, note no
    hasMultiline/subjectTarget (§17.5 has neither).

Task 4: CREATE internal/prompt/planner.go — BuildPlannerUserPayload(diff string, forcedCount int) string
  - SIGNATURE: `func BuildPlannerUserPayload(diff string, forcedCount int) string`.
  - BODY:
      if forcedCount <= 0 {
          return plannerUserInstruction + "\n\n" + diff        // §17.5 normal: fast path (mirror payload.go)
      }
      forced := fmt.Sprintf("Produce EXACTLY %d commits from these changes (do not reconsider the count):", forcedCount)
      return forced + "\n" + plannerUserInstruction + "\n\n" + diff
  - GOTCHA: forcedCount <= 0 (incl. negative) ⇒ normal. The diff is appended VERBATIM (no normalization;
    mirror payload.go's "diff is the exact tail"). NO trailing newline added beyond what diff carries.
  - DOC COMMENT: cite §17.5, explain the normal vs forced assembly, note the prepend-topology decision
    (findings §2), note diff is verbatim.

Task 5: CREATE internal/prompt/planner.go — extractJSONObject(s) (private, reimplemented)
  - COPY provider/parse.go's extractJSONObject body VERBATIM into a private `func extractJSONObject(s
    string) (string, bool)` in planner.go (the ~30-line brace-balanced state machine: first '{', depth,
    inString, escaped; byte-scanning UTF-8-safe). Add a doc comment noting it is a verbatim copy of
    provider's algorithm (kept private + reimplemented to avoid coupling the prompt leaf package to
    provider; sanctioned by the work item's "reuse the pattern").
  - GOTCHA: do NOT import provider. do NOT export provider's copy. The function is byte-identical to
    provider's so behavior matches exactly.

Task 6: CREATE internal/prompt/planner.go — ParsePlannerOutput(raw string) (PlannerOutput, error)
  - SIGNATURE: `func ParsePlannerOutput(raw string) (PlannerOutput, error)`.
  - BODY:
      s := strings.TrimSpace(raw)
      var out PlannerOutput
      if err := json.Unmarshal([]byte(s), &out); err == nil {
          return out, nil                                   // Attempt 1: whole-string Unmarshal
      }
      sub, found := extractJSONObject(s)
      if found {
          if err := json.Unmarshal([]byte(sub), &out); err == nil {
              return out, nil                               // Attempt 2: brace-balanced fallback
          } else {
              return PlannerOutput{}, fmt.Errorf("planner output: not valid JSON: %w", err)
          }
      }
      return PlannerOutput{}, fmt.Errorf("planner output: not valid JSON: %w", json.Unmarshal([]byte(s), &out))
      // (the final error re-runs whole-string Unmarshal purely to surface its error message; or capture
      //  the first err in a variable and reuse it — prefer capturing err1 once to avoid the double call.)
  - PREFER: capture `err1` from Attempt 1 and reuse it in the final return (cleaner than re-Unmarshaling).
      err1 := json.Unmarshal([]byte(s), &out)
      if err1 == nil { return out, nil }
      if sub, found := extractJSONObject(s); found {
          if err2 := json.Unmarshal([]byte(sub), &out); err2 == nil { return out, nil } else {
              return PlannerOutput{}, fmt.Errorf("planner output: not valid JSON: %w", err2)
          }
      }
      return PlannerOutput{}, fmt.Errorf("planner output: not valid JSON: %w", err1)
  - GOTCHA: returns NON-NIL error on failure (do NOT swallow). Tolerates "commits":null (→ nil slice,
    no panic), extra unknown fields (ignored by Unmarshal), missing message + single:false (→ "").
  - DOC COMMENT: cite §17.5 + §13.6.6, explain the two-attempt parse + brace-balanced fallback, note it
    returns a non-nil error so the caller can do the one retry, note it does NOT validate the
    single⇔message contract (the caller owns that).

Task 7: CREATE internal/prompt/planner_test.go — tests (mirror system_test.go/payload_test.go shapes)
  - IMPORTS: encoding/json, errors, strings, testing. Package: `prompt` (internal — near/suffix reused,
    unexported consts visible).
  - ADD TestBuildPlannerSystemPrompt_CanonicalExact: independently-derived `want` (build from §17.5, NOT
    from the impl) for examples=[]string{"feat: a", "fix: b\n\nBody."}; assert got==want with %q diff.
    The want MUST contain: the verbatim §17.5 header through the JSON-contract line + the single/message
    clause + "find the exact changes." + "\n\n" + "---\nfeat: a\n---\nfix: b\n\nBody.\n".
  - ADD TestBuildPlannerSystemPrompt_Properties: a cases table with anti-copy-paste guards:
      "role is commit-PLANNING assistant (not message generator)" — Contains "You are a commit-planning
        assistant"; NOT Contains "You are a commit message generator".
      "§17.1 anti-reuse block ABSENT" — NOT Contains "CRITICAL: You MUST NOT copy".
      "§17.1 subject-target line ABSENT" — NOT Contains "Target ~".
      "§17.1 multi-line rule ABSENT" — NOT Contains "multi-line commits AND".
      "JSON contract line PRESENT verbatim" — Contains `{"count": <int>, "single": <bool>`.
      "single/message clause PRESENT" — Contains "If single is true, set count=1".
      "--- count == len(examples)" — Count("---")==3 for 3 examples.
      "examples in order" — ALPHA<BETA<GAMMA index check.
  - ADD TestBuildPlannerSystemPrompt_EmptyExamples: nil/{} examples ⇒ no "---" lines, no panic, header
    still present.
  - ADD TestBuildPlannerUserPayload_NormalCanonicalExact: forcedCount==0 ⇒ got == "Decompose these
    un-staged changes into commits:\n\n" + diff (independently derived).
  - ADD TestBuildPlannerUserPayload_ForcedCanonicalExact: forcedCount==3 ⇒ got == "Produce EXACTLY 3
    commits from these changes (do not reconsider the count):\nDecompose these un-staged changes into
    commits:\n\n" + diff.
  - ADD TestBuildPlannerUserPayload_Properties: table — "normal: no Produce EXACTLY"; "forced: Produce
    EXACTLY N present with N interpolated"; "forced: N interpolated (5)"; "diff is the verbatim tail
    (normal)"; "diff is the verbatim tail (forced)"; "negative forcedCount == normal".
  - ADD TestPlannerRetryInstruction: assert PlannerRetryInstruction == "Respond with ONLY the JSON object
    described, no other text.".
  - ADD TestParsePlannerOutput: a cases table of {name, raw, wantOut, wantErr}:
      "clean multi-commit JSON" — `{"count":2,"single":false,"commits":[{"title":"A","description":"d1"},
        {"title":"B","description":"d2"}]}` → Count:2, Single:false, len(Commits):2, Message:"".
      "single-commit with message" — `{"count":1,"single":true,"commits":[{"title":"X","description":"d"}],
        "message":"feat: add thing"}` → Single:true, Message:"feat: add thing".
      "JSON in prose (brace-balanced fallback)" — "Here is the plan:\n{...valid...}\nThanks!" → parses.
      "JSON in code fence" — "```json\n{...valid...}\n```" → parses (extractJSONObject finds first '{').
      "leading/trailing whitespace trimmed" — "  \n{...valid...}\n  " → parses.
      "commits:null → nil slice, no panic" — `{"count":0,"single":false,"commits":null}` → Commits==nil.
      "extra unknown fields ignored" — `{...valid...,"extra":"ignored"}` → parses.
      "missing message + single:false → Message==\"\"" — covered by the multi-commit case.
      "malformed → error" — "not json at all" → wantErr != nil, out == zero PlannerOutput.
      "empty → error" — "" → wantErr != nil.
      "unbalanced braces → error" — `{"count":1` (no closer) → wantErr != nil.
    For success cases assert wantErr==nil AND the relevant fields; for error cases assert wantErr!=nil.
  - ADD TestExtractJSONObject (optional, private helper): a few cases mirroring provider's test — clean,
    prose-wrapped, code-fenced, no-brace→(false), unbalanced→(false), braces-in-string-ignored.
  - GOTCHA: do NOT redeclare near()/suffix() (in system_test.go). Use json.Marshal or hand-written raw
    strings for test inputs (raw strings with the JSON are clearest). For the canonical-exact `want`,
    build it from concatenated "...\n"+... literals (independently of the impl).
  - PLACEMENT: NEW file internal/prompt/planner_test.go.

Task 8: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/prompt/planner.go internal/prompt/planner_test.go`
  - `go build ./...`   (whole module compiles — the new file + test file)
  - `go vet ./...`
  - `golangci-lint run ./internal/prompt/...`   (errcheck/gosimple/govet/ineffassign/staticcheck/unused)
  - `go test -race ./internal/prompt/ -run "Planner" -v`   (all new planner tests)
  - `go test -race ./internal/prompt/`   (the WHOLE prompt package — system/payload tests still pass)
  - `go test ./...`   (FULL regression — no other package breaks; purely additive)
  - `git diff --exit-code go.mod go.sum` ⇒ empty.
  - `git status --short` ⇒ EXACTLY: `?? internal/prompt/planner.go` + `?? internal/prompt/planner_test.go`
    (2 entries); system.go/payload.go/provider/parse.go UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// === planner.go: the verbatim §17.5 system prompt constant (backtick raw string, NO trailing newline) ===
// The constant ENDS at "...find the exact changes." — the JSON-contract line (with <int>/<bool>/... tokens)
// is LITERAL (part of the format example); ONLY "<style examples>" is excluded (runtime placeholder).
const plannerSystemPrompt = `You are a commit-planning assistant. Given a diff of un-staged changes, decide whether they
form ONE coherent commit or SEVERAL, and partition them into logical units.

Rules:
- Prefer FEWER commits. A single commit is correct unless the changes clearly span
  unrelated concerns. Do not manufacture tiny commits.
- Each commit must be independently meaningful and reviewable. Group tightly-coupled
  changes (a function + its test, a refactor + its callers) together.
- Respect dependencies: if change B depends on change A, A comes first.
- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.

Respond with ONLY JSON, no prose, no code fences:
{"count": <int>, "single": <bool>, "commits": [{"title": "<short concept>", "description": "<precisely which files/hunks belong here, by path>"}, ...]}
- If single is true, set count=1 and ALSO include "message": "<the full commit message>".
- The "description" must be specific enough that a staging agent can find the exact changes.`

const plannerUserInstruction = "Decompose these un-staged changes into commits:"

// PlannerRetryInstruction is the §17.5 retry user payload for the caller's ONE retry after a parse
// failure (§13.6.6). Owned here so the verbatim §17.5 text is auditable in one place.
const PlannerRetryInstruction = "Respond with ONLY the JSON object described, no other text."


// === BuildPlannerSystemPrompt: const + blank + "---"-examples (mirrors system.go's BuildSystemPrompt) ===
func BuildPlannerSystemPrompt(examples []string) string {
	var b strings.Builder
	b.WriteString(plannerSystemPrompt)
	b.WriteByte('\n')
	b.WriteByte('\n') // one blank line between the JSON contract and the style examples
	for _, ex := range examples {
		b.WriteString("---\n") // one "---" BEFORE each message (same format as system.go)
		b.WriteString(ex)      // examples are pre-trimmed by RecentMessages
		b.WriteByte('\n')
	}
	return b.String()
}


// === BuildPlannerUserPayload: normal fast-path + forced-count prepend (mirrors payload.go's BuildUserPayload) ===
func BuildPlannerUserPayload(diff string, forcedCount int) string {
	if forcedCount <= 0 {
		return plannerUserInstruction + "\n\n" + diff // §17.5 normal (fast path; diff verbatim as tail)
	}
	forced := fmt.Sprintf("Produce EXACTLY %d commits from these changes (do not reconsider the count):", forcedCount)
	return forced + "\n" + plannerUserInstruction + "\n\n" + diff // §17.5 forced-count prepend
}


// === ParsePlannerOutput: whole-Unmarshal → brace-balanced fallback → error (mirrors provider.parseJSON) ===
func ParsePlannerOutput(raw string) (PlannerOutput, error) {
	s := strings.TrimSpace(raw)
	var out PlannerOutput
	if err := json.Unmarshal([]byte(s), &out); err == nil {
		return out, nil // Attempt 1: whole-string Unmarshal
	}
	if sub, found := extractJSONObject(s); found {
		if err := json.Unmarshal([]byte(sub), &out); err == nil {
			return out, nil // Attempt 2: brace-balanced fallback (JSON in prose / code fence)
		} else {
			return PlannerOutput{}, fmt.Errorf("planner output: not valid JSON: %w", err)
		}
	}
	err1 := json.Unmarshal([]byte(s), &out) // re-derive the most informative error for the message
	return PlannerOutput{}, fmt.Errorf("planner output: not valid JSON: %w", err1)
}
// (Tidier: capture err1 from Attempt 1 in a variable and reuse it, avoiding the re-Unmarshal above.)


// === extractJSONObject: VERBATIM copy of provider/parse.go's algorithm (private; reimplemented to avoid
//     coupling the prompt leaf package to provider — sanctioned by the work item's "reuse the pattern") ===
func extractJSONObject(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}
```

### Integration Points

```yaml
DATABASE:
  - none (pure prompt-construction + JSON parse; no git, no IO).

CONFIG:
  - none directly. forcedCount is passed in by the caller (decompose resolves it from `--commits N`;
    P4.M1.T1.S1 wires the flag). examples come from git.RecentMessages (v1). This layer takes them as
    plain params — it is decoupled from config (mirrors BuildSystemPrompt taking subjectTarget as an int).

ROUTES:
  - none (internal prompt module; no CLI flag, no public API surface in this task). The decompose
    orchestrator (P3.M2.T2.S1) wires the calls:
      diff, _   := deps.Git.WorkingTreeDiff(ctx, opts)                 # P2.M2.T2.S2 (parallel)
      examples  := <git.RecentMessages(...)>                           # v1
      sys       := prompt.BuildPlannerSystemPrompt(examples)           # THIS task
      payload   := prompt.BuildPlannerUserPayload(diff, forcedCount)   # THIS task
      raw, _    := <render(bare) + execute the planner agent>          # P3.M2.T2.S1
      out, err  := prompt.ParsePlannerOutput(raw)                      # THIS task
      if err != nil {
          raw, _ = <re-execute with payload = prompt.PlannerRetryInstruction>  # THIS task (the const)
          out, err = prompt.ParsePlannerOutput(raw)                    # one retry (§13.6.6)
      }
      if out.Single { <single-shortcut using out.Message, §13.6.4> } else { <loop out.Commits> }
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the new files (run after creation; fix anything that changes).
gofmt -w internal/prompt/planner.go internal/prompt/planner_test.go
gofmt -l internal/ pkg/          # Expected: empty (no files need formatting).

# Lint the new files specifically, then the whole module.
golangci-lint run ./internal/prompt/...
golangci-lint run ./...          # Expected: clean (errcheck/gosimple/govet/ineffassign/staticcheck/unused).

# Vet.
go vet ./...                     # Expected: no findings.

# Expected: zero errors. If any exist, READ the output and fix before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Run the new planner tests in isolation (verbose — confirm every case).
go test -race ./internal/prompt/ -run "Planner" -v
go test -race ./internal/prompt/ -run "ParsePlannerOutput" -v
go test -race ./internal/prompt/ -run "ExtractJSONObject" -v   # if the optional helper test is added

# Whole prompt package (system.go + payload.go tests must still pass — the new file is additive).
go test -race ./internal/prompt/

# Expected: all pass. The canonical-exact tests are the strongest guard — a mismatch means §17.5 text or
# assembly-topology drift; fix the implementation (or, if the test's independently-derived `want` is wrong,
# re-derive it from §17.5). Do NOT weaken the canonical-exact assertion to make it pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the whole module (the new file compiles + links; no existing importer breaks).
go build ./...

# Full regression (purely additive — no other package should change behavior).
go test ./...

# Confirm the module is unchanged apart from the two new files.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged"
git status --short                # Expected: ?? internal/prompt/planner.go  ?? internal/prompt/planner_test.go

# (No live agent / service to start — planner.go is a pure library module. The "integration" is the
#  decompose orchestrator in P3.M2.T2.S1, which is a SEPARATE work item and does NOT exist yet.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# §17.5 faithfulness self-check (run interactively; eyeball against prd_snapshot.md lines 1533–1549):
go test -run TestBuildPlannerSystemPrompt_CanonicalExact -v ./internal/prompt/ && \
  echo "PASS: planner system prompt is byte-faithful to §17.5"

# Robustness spot-checks (the real model failure modes — each must parse or error cleanly, never panic):
go test -run "TestParsePlannerOutput" -v ./internal/prompt/

# Anti-copy-paste guard (the #1 risk — §17.1 mature elements must NOT leak into the planner prompt):
go test -run "TestBuildPlannerSystemPrompt_Properties" -v ./internal/prompt/

# Expected: all pass. The ParsePlannerOutput cases (JSON-in-prose, JSON-in-code-fence, commits:null,
# malformed→error) are the domain-specific validations — they prove the parse is robust to the two real
# model failure modes + degrades to a non-nil error (triggering the caller's one retry) on garbage.
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

- [ ] `plannerSystemPrompt` is byte-faithful to §17.5 (pinned by the canonical-exact test).
- [ ] `BuildPlannerSystemPrompt` appends the style examples in the `---\n<msg>\n` format; nil/empty
      examples ⇒ no `---` lines, no panic.
- [ ] `BuildPlannerUserPayload` normal path == `instruction\n\n<diff>`; forced path prepends the
      `Produce EXACTLY N...` directive (N interpolated); negative forcedCount == normal.
- [ ] `ParsePlannerOutput` parses clean JSON, JSON-in-prose, and JSON-in-code-fence; returns a non-nil
      error on garbage/empty/unbalanced; tolerates `commits:null` and extra fields.
- [ ] `PlannerRetryInstruction` == the verbatim §17.5 retry text.
- [ ] `PlannerOutput`/`PlannerCommit` json tags map 1:1 to the §17.5 contract.
- [ ] No §17.1 mature-prompt element leaks into the planner prompt (anti-copy-paste test passes).

### Code Quality Validation

- [ ] Follows existing prompt-package conventions (no-trailing-newline consts; Build-owns-newlines; rich
      doc comments citing §17.5; `---`-example format; minimal decouled params).
- [ ] File placement matches the desired tree (2 new files in internal/prompt/).
- [ ] Anti-patterns avoided (no provider import; no trailing-newline constants; no swallowed parse errors;
      no in-struct single⇔message validation; no redeclared near/suffix helpers).
- [ ] Dependencies: stdlib only (encoding/json + fmt + strings); no new internal dep; no import cycle.

### Documentation & Deployment

- [ ] Rich doc comments on every exported symbol + the package (cite §17.5; diagram assembly; note the
      literal-vs-placeholder distinction; note the JSON-extraction reuse decision).
- [ ] No new environment variables or config (this layer is decoupled from config).
- [ ] Self-documenting via the §17.5 reference (no separate docs file needed, per the work item's DOCS: none).

---

## Anti-Patterns to Avoid

- ❌ Don't import `internal/provider` into `internal/prompt` — reimplement `extractJSONObject` privately
  (the work item sanctions "reuse the pattern"; importing couples a leaf package to a higher layer).
- ❌ Don't put a trailing newline on any string constant — the Build* functions own inter-block newlines.
- ❌ Don't include `<style examples>` in the `plannerSystemPrompt` constant — it is a runtime placeholder.
- ❌ Don't strip the `<int>`/`<bool>`/`<short concept>` tokens from the JSON-contract line — they are
  LITERAL instructive text (part of the format example shown to the model).
- ❌ Don't leak §17.1 mature-prompt elements ("You are a commit message generator", anti-reuse block,
  multi-line rule, "Target ~N characters") into the planner prompt — §17.5 is self-contained.
- ❌ Don't swallow `ParsePlannerOutput` failures (returning zero + nil error) — the caller needs the
  non-nil error to trigger the one retry (§13.6.6).
- ❌ Don't add in-struct / in-parse validation that errors when `single==false && message!=""` — the
  single⇔message contract is the orchestrator's concern (§13.6.4), not the parse layer's.
- ❌ Don't redeclare `near()`/`suffix()` in planner_test.go — they already live in system_test.go.
- ❌ Don't modify system.go, payload.go, provider/parse.go, or any existing file — this task is purely
  additive (2 new files). Don't add `hasMultiline`/`subjectTarget` params to BuildPlannerSystemPrompt —
  §17.5 has neither.
- ❌ Don't weaken the canonical-exact test to make it pass — re-derive `want` from §17.5 if it mismatches.

---

**Confidence Score: 9/10** — This is a mechanical extension of the established, well-tested prompt-
package pattern (system.go/payload.go) to a new §17.x section, plus a typed JSON parse that reuses a
proven algorithm already in the repo (provider/parse.go's extractJSONObject). Zero modifications to
existing files (purely additive), stdlib-only deps, no import cycle. The only residual uncertainty is
the forced-count prepend separator topology (§17.5 says "prepends" without specifying it) — resolved by
a defensible decision + a canonical-exact test that pins it, and trivially adjustable later without
affecting any other package. The exported surface exactly matches the consumer contract (findings §6),
so P3.M2.T2.S1 can proceed the moment this lands.
