---
name: "P3.M1.T1.S3 — Implement arbiter system prompt + JSON contract in internal/prompt/arbiter.go (PRD §17.7, §13.6.5, FR-M9)"
description: |

  CREATE ONE NEW FILE `internal/prompt/arbiter.go` in the existing `prompt` package: the **arbiter**
  prompt half of stagehand's v2 multi-commit decomposition (PRD §17.7). The arbiter is a **bare** agent
  that runs ONLY if the working tree is non-empty after the loop (§13.6.5). It receives the commits made
  this run (SHA + subject + file-list each) + a diff of the remaining changes, and returns a target SHA
  or null. Like the planner (S1), the arbiter emits STRUCTURED JSON (a single `target` field), so a JSON
  output contract + a robust parse are justified. Unlike the planner, the §17.7 system prompt has NO
  `<style examples>` placeholder, so `BuildArbiterSystemPrompt()` takes NO arguments.

  CONTRACT (P3.M1.T1.S3, verbatim from the work item):
    1. RESEARCH NOTE: §17.7 specifies the arbiter prompt verbatim. The arbiter is bare and runs only if
       the working tree is non-empty after the loop. It receives: commits made this run (SHA+subject+
       file-list each) + a diff of remaining changes. Returns JSON: `{"target": "<sha>"}` or
       `{"target": null}`. 'When in doubt, prefer a NEW commit (return null).' May only target a commit
       from the provided list.
    2. INPUT: The existing prompt package patterns + provider.extractJSONObject for JSON parsing.
    3. LOGIC: Create internal/prompt/arbiter.go. Define arbiterSystemPrompt constant (verbatim from
       §17.7). Define BuildArbiterUserPayload(commits []ArbiterCommit, leftoverDiff string) string —
       assembles the commit list (SHA+subject+files each) + the leftover diff. Define ArbiterOutput
       struct: Target *string (nil=null=new commit). Define ParseArbiterOutput(raw string)
       (ArbiterOutput, error).
    4. OUTPUT: prompt/arbiter.go exports BuildArbiterSystemPrompt(), BuildArbiterUserPayload(), and
       ParseArbiterOutput(). Consumed by decompose/arbiter.go (P3.M3.T1.S1).
    5. DOCS: none — self-documenting via PRD §17.7.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `internal/prompt/planner.go` (P3.M1.T1.S1, COMPLETE) — REUSED: arbiter.go CALLS its package-level
      `extractJSONObject` (same package prompt). arbiter.go MUST NOT redefine it (duplicate-symbol error).
    - `internal/prompt/stager.go` (P3.M1.T1.S2, parallel) — separate standalone new file; ZERO friction.
    - `internal/prompt/system.go`, `payload.go`, `*_test.go` — CONSUMED as pattern + helper source; UNCHANGED.
    - `internal/provider/parse.go` — UNCHANGED (algorithm already mirrored in planner.go).
    - `internal/decompose/*` — does NOT exist yet; THIS task does NOT create it (P3.M3.*).
    - go.mod / go.sum — UNCHANGED (stdlib only: encoding/json + fmt + strings).

  DELIVERABLES (2 new files, 0 modifications):
    CREATE internal/prompt/arbiter.go — `arbiterSystemPrompt` const (verbatim §17.7, NO style examples,
      NO trailing newline, backtick raw string), `arbiterCommitsHeader` + `arbiterLeftoverHeader` consts
      (designed §17.7-faithful section headers), `ArbiterCommit` + `ArbiterOutput` structs (Target *string),
      `BuildArbiterSystemPrompt()`, `BuildArbiterUserPayload(commits, leftoverDiff)`,
      `ParseArbiterOutput(raw)`. REUSES planner.go's `extractJSONObject` (no redeclaration).
    CREATE internal/prompt/arbiter_test.go — canonical-exact (system prompt + user payload) + properties
      (anti-copy-paste §17.1/§17.5/§17.6 ABSENT, §17.7 + em-dash PRESENT) + ParseArbiterOutput table
      (null→nil, sha→&sha, prose/fence fallback, non-string→error, malformed/empty/unbalanced→error,
      round-trip) + ArbiterOutput null-semantics test.

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing tests still pass (purely
  additive — no existing file changes); the arbiter prompt is byte-faithful to §17.7 (incl. the em-dash);
  ParseArbiterOutput parses clean JSON, JSON-in-prose (brace-balanced fallback), JSON-in-code-fence, and
  returns a non-nil error on garbage; the *string Target tri-state is correct (nil ⇔ null ⇔ new commit).

---

## Goal

**Feature Goal**: Implement the arbiter-agent prompt-construction + JSON-contract-parse layer for
multi-commit decomposition (PRD §17.7 / §13.6.5 / FR-M9) as a self-contained, stdlib-only module in the
existing `internal/prompt` package. This is the prompt analogue of v1's `system.go`/`payload.go` — the
verbatim §17.7 arbiter system prompt (role + decision rules + JSON contract; NO style examples, unlike the
planner), the user payload (the commit list SHA+subject+files each + the leftover diff), the
`ArbiterOutput` JSON-contract type (Target `*string` — nil ⇔ null ⇔ new commit), the `ArbiterCommit` input
type, and a robust `ParseArbiterOutput` that does whole-string `json.Unmarshal` with a brace-balanced
fallback (REUSING planner.go's in-package `extractJSONObject`).

**Deliverable** (2 new files, 0 modifications):
1. `internal/prompt/arbiter.go` — the arbiter prompt module: `arbiterSystemPrompt` const (verbatim §17.7
   system prompt, the ENTIRE fenced block from "You reconcile leftover changes..." through `Respond with
   ONLY JSON: {"target": "<sha from the list>"} or {"target": null}.`, with NO trailing newline — there is
   NO `<style examples>` placeholder in §17.7); `arbiterCommitsHeader` + `arbiterLeftoverHeader` private
   consts (designed §17.7-faithful section headers, NO trailing newline); `ArbiterCommit` struct (SHA,
   Subject, Files; the input); `ArbiterOutput` struct (Target `*string`, json tag `"target"`); `BuildAr
   biterSystemPrompt() string` (zero-arg; returns the constant); `BuildArbiterUserPayload(commits
   []ArbiterCommit, leftoverDiff string) string` (headers + commit blocks + leftover diff verbatim);
   `ParseArbiterOutput(raw string) (ArbiterOutput, error)` (whole-Unmarshal → brace-balanced fallback
   → error; REUSES planner.go's `extractJSONObject` — does NOT redefine it).
2. `internal/prompt/arbiter_test.go` — canonical-exact tests (independently-derived `want` for the system
   prompt and the multi-commit user payload), properties table (anti-copy-paste guards pinning §17.1/
   §17.5/§17.6 elements ABSENT and §17.7 + em-dash PRESENT), ParseArbiterOutput table (null→nil Target,
   sha→non-nil Target, JSON-in-prose, JSON-in-code-fence, non-string target→error, malformed/empty/
   unbalanced→error, extra fields ignored, round-trip), and a dedicated ArbiterOutput null-semantics test.

**Success Definition**:
- `BuildArbiterSystemPrompt()` returns a string byte-faithful to §17.7: starts with "You reconcile
  leftover changes into commits that were just made.", contains the 3 decision rules, preserves the em-dash
  in "return null) — never force a fit.", and ends with the JSON-contract line `Respond with ONLY JSON:
  {"target": "<sha from the list>"} or {"target": null}.` (the `<sha from the list>` token LITERAL). NO
  §17.1/§17.5/§17.6 element leaks; NO `<style examples>` token.
- `BuildArbiterUserPayload([]ArbiterCommit{{SHA:"a1b2", Subject:"feat: x", Files:[]string{"f.go","g.go"}},
  {SHA:"c3d4", Subject:"fix: y", Files:[]string{"h.go"}}}, "LEFTOVER")` ==
  `"The commits made this run (message and changed files for each):\n\na1b2\nfeat: x\nf.go\ng.go\n\nc3d4
  \nfix: y\nh.go\n\nA diff of changes that were not included in any commit:\n\nLEFTOVER"`.
- `ParseArbiterOutput(`{"target": null}`)` → `(ArbiterOutput{Target: nil}, nil)`.
- `ParseArbiterOutput(`{"target": "a1b2c3d4"}`)` → `(ArbiterOutput{Target: &"a1b2c3d4"}, nil)`.
- `ParseArbiterOutput(`The answer is {"target": "a1b2"} — done`)` → succeeds via the brace-balanced
  fallback (Target non-nil, *Target == "a1b2").
- `ParseArbiterOutput("not json at all")` → `(ArbiterOutput{}, non-nil error)`.
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 2 new untracked files only.

## User Persona

**Target User**: the decompose arbiter agent invocation (internal code, P3.M3.T1.S1) and, by extension,
the end user running `stagehand` on an un-staged working tree to get multiple logically-coherent commits.
The arbiter prompt is NOT user-facing CLI text; it is the system prompt + user payload piped to the bare
arbiter agent (PRD §13.6.2 / §17.7). The arbiter never performs git itself — stagehand owns all ref
mutations (FR-M10); the arbiter ONLY decides whether leftover changes belong with an existing commit or
warrant a new one.

**Use Case**: after the per-concept loop, IF `git status --porcelain` is non-empty (some changes were not
claimed by any stager), the orchestrator (P3.M3.T1.S1) collects the commits made this run (SHA + subject +
file-list via `diff-tree`), the leftover diff (working-tree diff with binary placeholders), assembles the
arbiter prompt via `BuildArbiterSystemPrompt` + `BuildArbiterUserPayload`, invokes the arbiter agent (bare
mode via `provider.Render`), and parses its JSON via `ParseArbiterOutput`. On `Target == nil` → new
(N+1)-th commit; on `Target` pointing at a SHA in the list → amend/rebuild that commit (tip amend or
mid-chain chain rebuild). If the working tree is clean after the loop, the arbiter does NOT run (§13.6.5).

**Pain Points Addressed**: the arbiter needs (a) a verbatim-faithful system prompt whose JSON contract is
unambiguous (return a SHA from the list OR null — "when in doubt, prefer a NEW commit"), and (b) a robust
parser tolerant of the two real model failure modes (JSON wrapped in prose; JSON wrapped in a code fence).
The tri-state need (SHA / null / ambiguous→null) is cleanly expressed as `Target *string` (nil ⇔ null ⇔
new commit), the safe default.

## Why

- **Closes the arbiter half of PRD §17.7 / §13.6.5 / FR-M9 at the prompt layer.** FR-M9: the arbiter
  "decide which just-made commit (by SHA) the leftovers belong to, or 'new'" and returns `JSON {target:
  "<sha>" | null}`. This task is the literal prompt-construction + JSON-parse implementation of that role —
  the THIRD and final decomposition prompt (planner §17.5 = S1 ✅; stager §17.6 = S2 parallel; arbiter
  §17.7 ← THIS). With it, the entire P3.M1 prompt epic is complete and P3.M3 can proceed.
- **Mechanical extension of the established prompt-package pattern (a planner.go sibling).** planner.go (S1)
  already proved the convention: verbatim PRD constant without trailing newline; a Build* function owns the
  blank-line topology; typed JSON-contract structs; a two-attempt `Parse*Output` (whole-Unmarshal →
  brace-balanced fallback). arbiter.go is the SAME pattern applied to §17.7 — no new architectural concept.
  The `extractJSONObject` algorithm ALREADY lives in package prompt (planner.go); arbiter.go REUSES it
  (same package) rather than redefining it (the #1 trap — see findings §4.1).
- **Unblocks the arbiter + leftover reconciliation (P3.M3).** P3.M3.T1.S1 (the arbiter agent call) consumes
  EXACTLY this task's exported surface (`BuildArbiterSystemPrompt`, `BuildArbiterUserPayload`,
  `ParseArbiterOutput`, `ArbiterOutput`, `ArbiterCommit`). The leftover-reconciliation pipeline cannot run
  until this prompt+parse layer exists.
- **Lowest-risk, maximal-reuse, backward-compatible.** Two NEW files; ZERO modifications to any existing
  file. The prompt package GAINS exports (additive — no existing caller or test breaks). go.mod/go.sum
  untouched (stdlib only). No import cycle, no new internal dependency (extractJSONObject is reused in-
  package from planner.go).

## What

One new file `internal/prompt/arbiter.go` (package `prompt`) exporting three functions and two types, plus
two private const headers and a private const system prompt; and one new test file
`internal/prompt/arbiter_test.go`. No new dependencies. No caller wiring (that is P3.M3.T1.S1). No system
prompt is "built" at runtime beyond returning the verbatim constant (§17.7 has no style examples).
Specifically:

- **`arbiterSystemPrompt`** (private const, backtick raw string, no trailing newline): the verbatim §17.7
  system prompt — the ENTIRE fenced block from "You reconcile leftover changes into commits that were just
  made." through `Respond with ONLY JSON: {"target": "<sha from the list>"} or {"target": null}.`. The
  `<sha from the list>` token is LITERAL instructive text (NOT a runtime placeholder). Preserves the em-dash
  in "return null) — never force a fit." (U+2014, the ONE non-ASCII byte). NO `<style examples>` token
  (§17.7 has none — unlike §17.5).
- **`arbiterCommitsHeader`** (private const): `"The commits made this run (message and changed files for
  each):"` (designed §17.7-faithful section header, trailing COLON, NO trailing newline).
- **`arbiterLeftoverHeader`** (private const): `"A diff of changes that were not included in any commit:"`
  (designed §17.7-faithful section header, trailing COLON, NO trailing newline).
- **`ArbiterCommit`** + **`ArbiterOutput`** (exported structs): the arbiter's input commit type + the
  §17.7 JSON contract (`Target *string`, nil ⇔ null ⇔ new commit).
- **`BuildArbiterSystemPrompt() string`**: zero-arg; returns `arbiterSystemPrompt`.
- **`BuildArbiterUserPayload(commits []ArbiterCommit, leftoverDiff string) string`**: `arbiterCommitsHeader
  + "\n\n"` + per-commit blocks (each: `SHA + "\n" + Subject + "\n"` + each file `file + "\n"`, separated
  by ONE blank line) + `arbiterLeftoverHeader + "\n\n" + leftoverDiff` (verbatim tail).
- **`ParseArbiterOutput(raw string) (ArbiterOutput, error)`**: TrimSpace → whole `json.Unmarshal` into
  `ArbiterOutput` → on failure, REUSE planner.go's `extractJSONObject` for the brace-balanced fallback +
  retry Unmarshal → else non-nil error.
- **`extractJSONObject`** (private, OWNED BY planner.go): arbiter.go does NOT define it. It CALLS the
  existing package-prompt copy (planner.go:161). Redefining it = "redeclared in this package" compile error.

### Success Criteria

- [ ] `internal/prompt/arbiter.go` defines `arbiterSystemPrompt` byte-faithful to §17.7 (verified by an
      independently-derived canonical-exact test), with NO trailing newline, the em-dash preserved, the
      `<sha from the list>` token LITERAL, and NO `<style examples>` token inside the constant.
- [ ] `BuildArbiterSystemPrompt()` == `arbiterSystemPrompt` (zero-arg; returns the constant verbatim).
- [ ] `BuildArbiterUserPayload(commits, leftoverDiff)` == the §2 assembly; commits rendered
      SHA+subject+files each (files one per line), one blank line between commit blocks, leftoverDiff
      appended VERBATIM as the tail; empty/nil commits and empty Files do not panic.
- [ ] `ArbiterOutput.Target` is `*string` with json tag `"target"`; `{"target": null}` → nil,
      `{"target":"<sha>"}` → &"<sha>", `{}` → nil, `{"target":123}` → parse error.
- [ ] `ParseArbiterOutput` parses clean JSON, JSON-in-prose, and JSON-in-code-fence (via the REUSED
      `extractJSONObject`); returns `(ArbiterOutput{}, non-nil error)` on garbage/empty/unbalanced; does
      NOT validate target-in-list (caller's job).
- [ ] arbiter.go does NOT define `extractJSONObject` (reuses planner.go's copy) — no redeclaration.
- [ ] NO `ArbiterRetryInstruction` is invented (§17.7 does not define one; the work item does not list it).
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 2 new untracked files.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the verbatim §17.7 text
(findings §1 — captured character-for-character, with the em-dash, the no-backticks ⇒ raw-string note, and
the no-style-examples ⇒ zero-arg Build difference); the user-payload format design (findings §2 — the ONE
ambiguous point, the exact assembly pinned by a canonical-exact test); the types (findings §3 —
ArbiterCommit fields + the *string null semantics with the full json.Unmarshals-truth table); the parse
algorithm + the CRITICAL "extractJSONObject ALREADY EXISTS — reuse, do not redeclare" rule (findings §4.1
— the #1 trap); the exports + consumer contract (findings §5 — exactly what P3.M3.T1.S1 needs); the
"no retry instruction" asymmetry (findings §6); the prompt-package conventions (findings §7); the test
conventions including the REUSE-near/suffix and REUSE-extractJSONObject rules (findings §8); and the scope
boundaries (findings §9 — zero modifications, stdlib only, reuse planner.go). No decompose/git knowledge
required — the contract is fully self-contained at the prompt layer.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (verbatim §17.7 text + conventions + the JSON decision)
- docfile: plan/002_a17bb6c8dc1d/P3M1T1S3/research/findings.md
  why: §1 the VERBATIM §17.7 system prompt (the arbiterSystemPrompt constant) + the em-dash (U+2014) +
       the no-backticks⇒raw-string note + the NO-style-examples⇒zero-arg-BuildArbiterSystemPrompt
       difference (the key contrast with S1); §2 the user-payload format DECISION (the one ambiguous point
       — §17.7 gives only "the commit list + the leftover diff"; the exact assembly is pinned by a test);
       §3 ArbiterCommit + ArbiterOutput (Target *string) with the full json.Unmarshal truth table (nil ⇔
       null ⇔ new commit); §4 ParseArbiterOutput's two-attempt algorithm; §4.1 the CRITICAL
       extractJSONObject-ALREADY-EXISTS-REUSE-DON'T-REDECLARE rule (the #1 trap); §5 the exports + the
       consumer (P3.M3.T1.S1); §6 no ArbiterRetryInstruction; §7 conventions; §8 tests; §9 scope.
  critical: §4.1 (extractJSONObject ALREADY exists in package prompt at planner.go:161 — arbiter.go must
            CALL it, NOT redefine it, or the package will not compile: "redeclared in this package");
            §3.2 (Target *string: nil ⇔ null ⇔ new commit — the load-bearing design; a non-string target is
            a parse error); §1.1 (the em-dash must be preserved, never an ASCII hyphen); §2 (the user-
            payload format is a DESIGN decision, not verbatim §17.7 — pin it with a canonical-exact test).

# MUST READ — the CLOSEST TEMPLATE: planner.go (S1, COMPLETE — read it, mirror its style; UNCHANGED)
- file: internal/prompt/planner.go
  section: the package-level doc comment (cite PRD §; explain the role; note no-trailing-newline + Build-
           owns-newlines); the `plannerSystemPrompt` backtick raw-string const (verbatim PRD, no trailing
           \n, rich doc comment); PlannerCommit + PlannerOutput structs (json tags, doc comments); the
           BuildPlannerSystemPrompt assembly (strings.Builder, WriteString/WriteByte, explicit '\n' placement);
           BuildPlannerUserPayload (fast-path `+` concat / multi-block); ParsePlannerOutput (TrimSpace →
           whole-Unmarshal → extractJSONObject fallback → error); the private extractJSONObject (~30-line
           brace-balanced state machine at line 161 — arbiter.go REUSES this, it does NOT copy it).
  why: arbiter.go is planner.go's §17.7 sibling: same package, same const style, same struct+json-tag
       style, same Build* assembly, same Parse*Output two-attempt algorithm. Copy the STYLE (doc comments,
       no-trailing-newline, Builder assembly, two-attempt parse), NOT the §17.5 TEXT. The arbiter differs
       in THREE ways: (a) BuildArbiterSystemPrompt is ZERO-arg (no style examples); (b) BuildArbiterUser
       Payload takes []ArbiterCommit (a richer input than a bare diff); (c) ParseArbiterOutput REUSES the
       existing extractJSONObject instead of providing its own.
  pattern: define `arbiterSystemPrompt` as a backtick const ending at the JSON-contract line with NO
           trailing newline; BuildArbiterSystemPrompt() simply returns arbiterSystemPrompt;
           BuildArbiterUserPayload uses strings.Builder (the commit loop + nested file loop need a Builder,
           like BuildPlannerSystemPrompt's example loop); ParseArbiterOutput calls extractJSONObject (the
           planner.go copy) directly — DO NOT paste the function body into arbiter.go.
  gotcha: §17.7 has an em-dash (planner's §17.5 does not) — preserve it. §17.7 has NO style-examples
          placeholder (planner's §17.5 does) — BuildArbiterSystemPrompt is zero-arg. arbiter.go does NOT
          define extractJSONObject (planner.go already does, same package).

# MUST READ — the S1 PRP (the full reference PRP arbiter.go's PRP mirrors in structure)
- docfile: plan/002_a17bb6c8dc1d/P3M1T1S1/PRP.md
  why: S1 is the closest analog (system prompt + JSON contract + parse + structs). This PRP's structure,
       gotchas, and validation gates are modeled on S1's. Read it to understand the exact conventions
       (no-trailing-newline, Build-owns-newlines, anti-copy-paste tests, the parse-fallback rationale).
  section: "Known Gotchas" (literal-vs-placeholder; reimplement-vs-reuse); "Implementation Tasks"; the
           planner_test.go canonical-exact + properties + parse-table shapes (mirrored here).

# MUST READ — the user-payload split-constants pattern (UNCHANGED — read only)
- file: internal/prompt/payload.go
  section: the const style (userInstruction/userInstructionReject/rejectionPreamble/rejectionEpilogue:
           named consts, NO trailing newline, Build-owns-newlines); BuildUserPayload (split-constants-
           around-interleaved-dynamic-content; the diff appended VERBATIM as the tail).
  why: BuildArbiterUserPayload splits arbiterCommitsHeader / arbiterLeftoverHeader around the commit
       blocks (like payload.go splits its consts around the rejected-subject list) and appends leftoverDiff
       verbatim as the tail (like payload.go appends the diff). Copy the STYLE.
  pattern: `arbiterCommitsHeader` + `arbiterLeftoverHeader` are named private consts (trailing COLON, no
           trailing \n); BuildArbiterUserPayload uses strings.Builder to assemble header + commit blocks +
           header + leftoverDiff; leftoverDiff is the verbatim tail.

# MUST READ — the test exemplars (the test patterns to mirror in arbiter_test.go)
- file: internal/prompt/planner_test.go
  section: TestBuildPlannerSystemPrompt_CanonicalExact (independently-derived `want` via concatenated
           "...\n" + ... literals, %q diff on failure); TestBuildPlannerSystemPrompt_Properties (cases
           table incl. anti-copy-paste ABSENT guards + "---" count); the ParsePlannerOutput cases table
           (clean/prose/fence/whitespace/null/extra/malformed/empty/unbalanced); TestParsePlannerOutput_
           RoundTrip (Marshal → parse back); TestExtractJSONObject (the shared extractor's coverage —
           already applies to arbiter.go's reused copy).
  why: the EXACT test shapes arbiter_test.go mirrors. The canonical-exact test is the strongest guard
       against §17.7 text + §2-assembly drift; the parse table covers the *string Target semantics (a
       richer assertion than the planner's struct compare — add a nil-vs-non-nil check).
  pattern: write TestBuildArbiterSystemPrompt_CanonicalExact with an independently-derived `want` (build
           from §17.7, NOT from the impl); a TestBuildArbiterSystemPrompt_Properties table with anti-copy-
           paste cases (§17.1/§17.5/§17.6 ABSENT, §17.7 + em-dash PRESENT); a TestBuildArbiterUserPayload_
           CanonicalExact (multi-commit with files — the §2 assembly); a ParseArbiterOutput cases table
           (null→nil, sha→&sha, prose/fence/whitespace/extra/malformed/empty/unbalanced/non-string→error,
           round-trip); a TestArbiterOutput_NullSemantics (nil ⇔ null ⇔ new commit).
  gotcha: REUSE near()/suffix() (system_test.go bottom — do NOT redeclare). REUSE the in-package
          extractJSONObject in TestExtractJSONObject-style coverage IF desired, but do NOT re-test it
          exhaustively (planner_test.go already does — it is the same function). Focus arbiter_test.go's
          coverage on the arbiter-specific surface.

# MUST READ — the const-doc-comment + em-dash precedent
- file: internal/prompt/system.go
  section: the package doc + const-declaration style (maturePromptHeader/antiReuseProhibition: rich doc
           comments citing the PRD § + the "NOTE the EM-DASH" annotation; BuildSystemPrompt's rich doc).
  why: the const-doc-comment style + the em-dash documentation precedent. arbiterSystemPrompt carries an
       em-dash (§1.1) — mirror system.go's explicit "NOTE the EM-DASH" doc comment. The §17.7 const is
       ASCII-except-one-em-dash (no backticks ⇒ backtick raw string compiles, like maturePromptHeader).

# MUST READ — the design reference (role + contract + the consumer)
- docfile: plan/002_a17bb6c8dc1d/architecture/decompose_architecture.md
  section: "The Four Agent Roles" table (arbiter: bare, JSON `{target: "<sha>"|null}`, reuses
           prompt/arbiter.go); "Arbiter Resolution (§13.6.5)" (null→new; target==HEAD→tip amend;
           target==mid-chain→chain rebuild; ambiguous→null — all are the CONSUMER's job, P3.M3.T1/T2).
  why: confirms the arbiter role is BARE, its output contract is the `target` SHA-or-null JSON, and that
       THIS task's exports are consumed by decompose/arbiter.go (P3.M3.T1.S1). The doc lists prompt/arbiter.go
       as the reuse; THIS task implements it. Confirms ParseArbiterOutput does NOT do git or chain-rebuild.

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §17.7 (the arbiter prompt — verbatim system prompt + "user payload: the commit list + the
       leftover diff")
  why: §17.7 is the verbatim source for arbiterSystemPrompt. The "(sketch)" label does NOT weaken this —
       the work item says "verbatim from §17.7", and §17.1/§17.2/§17.5 set the precedent that PRD §17.x
       code blocks are authoritative text.
- url: PRD.md §13.6.5 (the arbiter runtime semantics — null/target-HEAD/target-mid-chain/ambiguous→null;
       "Stagehand performs ALL git; the arbiter only decides")
  why: the runtime semantics the prompt + parse layer must serve. Confirms ParseArbiterOutput only parses;
       the resolution mechanics (new/tip-amend/mid-chain/ambiguous-default) are the consumer's (P3.M3).
```

### Current Codebase tree (relevant slice)

```bash
internal/prompt/
  planner.go              # READ (CLOSEST TEMPLATE): §17.5 system-prompt const + PlannerOutput struct +
                          #   Build* assembly + ParsePlannerOutput two-attempt parse + extractJSONObject
                          #   (line 161 — arbiter.go REUSES this; do NOT redefine). S1 COMPLETE. UNCHANGED.
  planner_test.go         # READ: the canonical-exact + properties + parse-table + round-trip test shapes.
  payload.go              # READ: split-constants-around-interleaved-content + diff-as-verbatim-tail. UNCHANGED.
  payload_test.go         # READ: two-path canonical-exact + properties test shape. UNCHANGED.
  system.go               # READ: const-doc-comment style + the em-dash "NOTE" precedent. UNCHANGED.
  system_test.go          # READ: canonical-exact + anti-copy-paste shapes; ALSO defines near()+suffix()
                          #   at the bottom (REUSE — do NOT redeclare). UNCHANGED.
  stager.go               # PARALLEL (S2) — separate standalone new file; ZERO merge friction.
internal/provider/
  parse.go                # READ (for understanding only): extractJSONObject algorithm (already mirrored in
                          #   planner.go; arbiter.go reuses planner.go's copy). UNCHANGED.
go.mod / go.sum           # UNCHANGED (stdlib only: encoding/json + fmt + strings).
.golangci.yml             # READ: linter config (errcheck/gosimple/govet/ineffassign/staticcheck/unused).
```

### Desired Codebase tree with files to be added

```bash
internal/prompt/arbiter.go          # NEW — the arbiter prompt module (package prompt):
                                     #   const arbiterSystemPrompt      (verbatim §17.7, backtick raw string,
                                     #                                  em-dash preserved, no trailing \n, no <style examples>)
                                     #   const arbiterCommitsHeader     ("The commits made this run (message and changed files for each):")
                                     #   const arbiterLeftoverHeader    ("A diff of changes that were not included in any commit:")
                                     #   type ArbiterCommit             (SHA, Subject, Files []string)
                                     #   type ArbiterOutput             (Target *string; json:"target")
                                     #   func BuildArbiterSystemPrompt() string                                    (zero-arg; returns the const)
                                     #   func BuildArbiterUserPayload(commits []ArbiterCommit, leftoverDiff string) string
                                     #   func ParseArbiterOutput(raw string) (ArbiterOutput, error)                 (reuses extractJSONObject)
                                     #   [extractJSONObject NOT defined here — reused from planner.go]
internal/prompt/arbiter_test.go      # NEW — canonical-exact (system prompt + multi-commit user payload)
                                     #   + properties (anti-copy-paste §17.1/§17.5/§17.6 ABSENT; §17.7 + em-dash PRESENT)
                                     #   + ParseArbiterOutput table (null/sha/prose/fence/non-string/malformed/empty/unbalanced/extra/round-trip)
                                     #   + ArbiterOutput null-semantics (nil ⇔ null ⇔ new commit).
                                     #   REUSES near()/suffix() from system_test.go + extractJSONObject from planner.go.
# go.mod/go.sum UNCHANGED. planner.go/stager.go/system.go/payload.go/provider/parse.go all UNCHANGED. 0 modifications.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (extractJSONObject ALREADY EXISTS — findings §4.1, the #1 trap): planner.go (S1, COMPLETE) ALREADY
//   defines a package-level unexported `func extractJSONObject(s string) (string, bool)` at line 161. Because
//   arbiter.go is in the SAME package (`prompt`), it SHARES that function. If arbiter.go defines its OWN
//   extractJSONObject, the Go compiler ERRORS: "extractJSONObject redeclared in this package" → the package
//   will NOT build. THE RULE: arbiter.go CALLS the existing planner.go copy directly (extractJSONObject(s));
//   it does NOT paste the body, it does NOT redefine it. (This differs from S1, which created the FIRST copy
//   because none existed yet. S3 reuses it.) ParseArbiterOutput's fallback is therefore a one-liner call.

// CRITICAL (Target *string — findings §3.2, the load-bearing design): the §17.7 contract is `{"target":
//   "<sha>"}` OR `{"target": null}`. Express this as `Target *string`:
//     {"target": null}     → Target == nil          (new commit — the "when in doubt" default)
//     {"target": "<sha>"}  → Target != nil, *Target == "<sha>"
//     {}                   → Target == nil          (absent ⇒ nil ⇒ new commit; the safe default)
//     {"target": ""}       → Target == &"" (non-nil) (degenerate — CALLER rejects empty ⇒ new commit)
//     {"target": 123}      → json.Unmarshal ERROR   (non-string ⇒ type error ⇒ ParseArbiterOutput errors)
//   nil Target ⇔ "new commit" is the safe default for BOTH null and missing-field. ParseArbiterOutput does
//   NOT validate target-in-list (it has no list) nor reject empty — those are the CALLER's job (P3.M3.T1.S1,
//   §13.6.5 "Ambiguous → default to null"). Do NOT add in-struct validation.

// CRITICAL (the EM-DASH — findings §1.1): §17.7 line 7 — "When in doubt, prefer a NEW commit (return null)
//   — never force a fit." — has an EM-DASH "—" (U+2014), the ONE non-ASCII byte (same as §17.1's
//   antiReuseProhibition / §17.6's stagerGuardrails). In a backtick raw string literal the UTF-8 em-dash bytes
//   are literal (no escaping). It MUST NOT become an ASCII hyphen "-" (verbatim §17.7 fidelity). Document it
//   in the const's doc comment (mirror system.go's "NOTE the EM-DASH" annotation).

// CRITICAL (no-trailing-newline + Build-owns-newlines — findings §7): ALL string constants
//   (arbiterSystemPrompt, arbiterCommitsHeader, arbiterLeftoverHeader) are defined WITHOUT trailing newlines.
//   The Build* functions own ALL inter-block newline placement. This is the load-bearing convention from
//   system.go/payload.go/planner.go — the §17.7 blank-line topology lives in exactly one auditable place.

// GOTCHA (BuildArbiterSystemPrompt is ZERO-arg — findings §1.3): §17.7 has NO `<style examples>` placeholder
//   (unlike §17.5). So BuildArbiterSystemPrompt() takes NO arguments and just returns arbiterSystemPrompt. Do
//   NOT add an `examples []string` parameter (there is nothing to append). The thin wrapper exists for API
//   symmetry with the Build* family + to keep the const private (no-trailing-newline enforced in one place).

// GOTCHA (the user-payload format is a DESIGN decision — findings §2, the one ambiguous point): §17.7 says
//   only "the commit list + the leftover diff" — it does NOT specify the assembly format. DECISION (pin with
//   a canonical-exact test):
//     arbiterCommitsHeader + "\n\n"
//     for each commit: SHA + "\n" + Subject + "\n" + (each file: file + "\n") + "\n"   // blank line after each
//     arbiterLeftoverHeader + "\n\n" + leftoverDiff   // verbatim tail
//   arbiterCommitsHeader / arbiterLeftoverHeader are designed §17.7-faithful section headers (NOT verbatim PRD
//   text). leftoverDiff is appended VERBATIM (no normalization; payload.go's "diff is the exact tail"). Empty/
//   nil commits and empty Files do NOT panic (defensive — the loop body simply doesn't run).

// GOTCHA (NO ArbiterRetryInstruction — findings §6): §17.7 defines NO retry instruction (unlike §17.5). The
//   work item's export list does NOT include one. Do NOT copy-paste S1's PlannerRetryInstruction pattern into
//   arbiter.go. If the caller (P3.M3.T1.S1) wants a retry, it passes a plain string; this layer is not its home.

// GOTCHA (ParseArbiterOutput returns a NON-NIL error on failure — findings §4.2): do NOT swallow parse failures
//   (a zero ArbiterOutput with nil error hides the problem). Return (ArbiterOutput{}, fmt.Errorf("arbiter
//   output: not valid JSON: %w", err)) so decompose/arbiter.go can retry / default-to-null (§13.6.5).

// GOTCHA (shared test helpers near()/suffix() — findings §8): they are ALREADY defined at the BOTTOM of
//   system_test.go (same package `prompt`). Do NOT redeclare them in arbiter_test.go (duplicate-symbol compile
//   error — the SAME trap as the extractJSONObject redeclaration, but for test helpers). arbiter_test.go is
//   `package prompt` (internal test) so the unexported arbiterSystemPrompt/arbiterCommitsHeader/
//   arbiterLeftoverHeader ARE visible, and extractJSONObject (planner.go) is callable.

// GOTCHA (backtick raw string for arbiterSystemPrompt): §17.7 contains double quotes (the JSON contract line:
//   `"target"`) and angle brackets (`<sha from the list>`) but NO backticks → a backtick raw string literal
//   compiles cleanly (no escaping of " needed), exactly like planner.go's plannerSystemPrompt. The em-dash is a
//   UTF-8 byte literal in the raw string. Do NOT use a double-quoted "..." literal (would need to escape every ").

// GOTCHA (PARALLEL execution with S2 + S1 already complete): planner.go (S1) is COMPLETE — its extractJSONObject
//   IS available for arbiter.go to reuse. stager.go (S2) is being implemented in parallel — it is a standalone
//   new file with zero overlap. arbiter.go is also a standalone new file → zero merge friction with both.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/prompt/arbiter.go — package prompt

// ArbiterCommit is one commit made this run, as shown to the arbiter (§13.6.5: "SHAs, messages, and
// file-lists"). Built by the consumer (decompose/arbiter.go) from diff-tree output; the SHA is the value
// the arbiter may return as "target".
type ArbiterCommit struct {
	SHA     string   // the commit's full SHA (40/64 hex) — the value the arbiter may return as "target".
	Subject string   // the commit's subject line (one line; §13.6.5's "messages").
	Files   []string // the file-list (diff-tree --name-only) for this commit; may be empty.
}

// ArbiterOutput is the arbiter's JSON response (§17.7). A nil Target means {"target": null} → a NEW commit
// (the §17.7 "when in doubt" default); a non-nil Target points at the SHA to amend. The caller
// (decompose/arbiter.go) validates Target is in the provided commit list and non-empty, and resolves
// new/tip-amend/mid-chain/ambiguous→null (§13.6.5). ParseArbiterOutput does NOT validate — it only parses.
type ArbiterOutput struct {
	Target *string `json:"target"` // nil ⇔ null ⇔ new commit; &"<sha>" ⇔ amend that commit.
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/prompt/arbiter.go — arbiterSystemPrompt const (verbatim §17.7)
  - ADD a package-level doc comment (mirror planner.go's: cite PRD §17.7, explain the arbiter role (bare;
    runs only if the working tree is non-empty after the loop; receives commits-made-this-run + a leftover
    diff; returns a target SHA or null; performs NO git itself — stagehand owns all ref ops), note that
    constants carry no trailing newline + Build* own inter-block newlines, note §17.7 is ASCII EXCEPT one
    em-dash + has NO style-examples placeholder + has NO backticks).
  - DEFINE `arbiterSystemPrompt` (private const, backtick raw string): the verbatim §17.7 system prompt —
    the ENTIRE fenced block from "You reconcile leftover changes into commits that were just made." through
    `Respond with ONLY JSON: {"target": "<sha from the list>"} or {"target": null}.` INCLUDING the literal
    `<sha from the list>` token (instructive, NOT a placeholder). NO trailing newline. NO `<style examples>`
    token (§17.7 has none). See findings §1 for the exact text.
  - NAMING: lowercase for private (arbiterSystemPrompt). Mirror planner.go's const-naming.
  - GOTCHA: backtick raw string (the JSON line has double quotes; backticks avoid escaping; §17.7 has NO
    backticks so the raw string compiles). Preserve the em-dash. Add a "NOTE the EM-DASH" doc annotation.

Task 2: CREATE internal/prompt/arbiter.go — arbiterCommitsHeader + arbiterLeftoverHeader consts
  - DEFINE `arbiterCommitsHeader` (private const, plain "..." literal): `"The commits made this run
    (message and changed files for each):"` (trailing COLON, NO trailing newline). Add a doc comment noting
    it is a DESIGNED §17.7-faithful section header (NOT verbatim PRD text) introducing the commit list.
  - DEFINE `arbiterLeftoverHeader` (private const, plain "..." literal): `"A diff of changes that were not
    included in any commit:"` (trailing COLON, NO trailing newline). Same doc note (designed header).
  - GOTCHA: these are NOT verbatim §17.7 text (§17.7 gives no headers) — they are a clean assembly choice
    (findings §2). Named consts so the topology is auditable.

Task 3: CREATE internal/prompt/arbiter.go — ArbiterCommit + ArbiterOutput structs
  - DEFINE the two structs exactly as in "Data models and structure" above (ArbiterCommit: SHA, Subject,
    Files []string; ArbiterOutput: Target *string, json tag "target"). Add doc comments citing §17.7 +
    §13.6.5 + the *string null-semantics note (nil ⇔ null ⇔ new commit).
  - GOTCHA: ArbiterOutput.Target is *string (NOT string) so null vs sha vs absent are distinguishable as
    nil-vs-non-nil. Do NOT add in-struct validation (target-in-list / non-empty is the caller's job).

Task 4: CREATE internal/prompt/arbiter.go — BuildArbiterSystemPrompt() string
  - SIGNATURE: `func BuildArbiterSystemPrompt() string` (ZERO args — §17.7 has no style examples; see
    findings §1.3).
  - BODY: `return arbiterSystemPrompt` (the thin wrapper keeps the const private + API symmetry).
  - DOC COMMENT: cite §17.7; note the arbiter system prompt is the verbatim constant with NO appended style
    examples (unlike the planner §17.5); note this is why the function is zero-arg; note stagehand performs
    all git (FR-M10) and the arbiter only decides.

Task 5: CREATE internal/prompt/arbiter.go — BuildArbiterUserPayload(commits, leftoverDiff) string
  - SIGNATURE: `func BuildArbiterUserPayload(commits []ArbiterCommit, leftoverDiff string) string`.
  - BODY (strings.Builder — the commit loop + nested file loop need a Builder, like BuildPlannerSystemPrompt):
      var b strings.Builder
      b.WriteString(arbiterCommitsHeader)
      b.WriteByte('\n'); b.WriteByte('\n')                         // one blank line before the commit list
      for _, c := range commits {
          b.WriteString(c.SHA); b.WriteByte('\n')
          b.WriteString(c.Subject); b.WriteByte('\n')
          for _, f := range c.Files {
              b.WriteString(f); b.WriteByte('\n')
          }
          b.WriteByte('\n')                                        // one blank line separating commit blocks
      }
      b.WriteString(arbiterLeftoverHeader)
      b.WriteByte('\n'); b.WriteByte('\n')                         // one blank line before the diff
      b.WriteString(leftoverDiff)                                  // verbatim tail (no normalization)
      return b.String()
  - GOTCHA: empty/nil commits ⇒ the loop body never runs (no panic; output is headers + diff — acceptable,
    though impossible in practice since the arbiter runs after ≥1 commit). A commit with empty Files ⇒
    block is SHA+Subject only (no file lines). leftoverDiff is appended VERBATIM (payload.go precedent).
  - DOC COMMENT: cite §17.7 + §13.6.5; diagram the assembly topology (header + blank + per-commit blocks
    [SHA\nSubject\nfiles one-per-line\n] separated by blanks + header + blank + verbatim diff); note the
    header consts are designed (not verbatim §17.7); note the consumer (P3.M3.T1.S1) builds []ArbiterCommit
    from diff-tree; defensive behavior on empty inputs.

Task 6: CREATE internal/prompt/arbiter.go — ParseArbiterOutput(raw string) (ArbiterOutput, error)
  - SIGNATURE: `func ParseArbiterOutput(raw string) (ArbiterOutput, error)`.
  - BODY (mirrors ParsePlannerOutput EXACTLY, reusing the planner.go extractJSONObject):
      s := strings.TrimSpace(raw)
      var out ArbiterOutput
      err1 := json.Unmarshal([]byte(s), &out)
      if err1 == nil {
          return out, nil                                           // Attempt 1: whole-string Unmarshal
      }
      if sub, found := extractJSONObject(s); found {               // Attempt 2: brace-balanced fallback
          if err2 := json.Unmarshal([]byte(sub), &out); err2 == nil {
              return out, nil
          } else {
              return ArbiterOutput{}, fmt.Errorf("arbiter output: not valid JSON: %w", err2)
          }
      }
      return ArbiterOutput{}, fmt.Errorf("arbiter output: not valid JSON: %w", err1)
  - GOTCHA: DO NOT define extractJSONObject in arbiter.go — CALL the planner.go copy (same package). DO NOT
    import provider. Returns NON-NIL error on failure. Tolerates extra unknown fields (ignored), missing
    "target" (→ nil Target), "target":null (→ nil Target), "target":"<sha>" (→ &"<sha>"). A non-string
    target ("target":123) is a json type error → non-nil error (correct — violates §17.7 contract).
  - DOC COMMENT: cite §17.7 + §13.6.5; explain the two-attempt parse + brace-balanced fallback (reuses
    planner.go's extractJSONObject); note it returns a non-nil error so the caller can retry / default-to-
    null; note it does NOT validate target-in-list / non-empty (the caller's job — §13.6.5 ambiguous→null).

Task 7: CREATE internal/prompt/arbiter_test.go — tests (mirror planner_test.go shapes)
  - IMPORTS: encoding/json, strings, testing. Package: `prompt` (internal — near/suffix REUSED from
    system_test.go; unexported arbiterSystemPrompt/arbiterCommitsHeader/arbiterLeftoverHeader visible;
    extractJSONObject callable).
  - GOTCHA: do NOT redeclare near()/suffix() (system_test.go). Do NOT redeclare extractJSONObject (planner.go).
  - ADD TestBuildArbiterSystemPrompt_CanonicalExact: independently-derived `want` (build from §17.7, NOT
    from the impl) for the zero-arg call; assert got==want with a %q diff. The want MUST contain the verbatim
    §17.7 block: the role line, the "You are given the commits..." line, the 3 decision rules (with the
    em-dash in "return null) — never force a fit."), and the JSON-contract line with the LITERAL `<sha from
    the list>` token. It MUST be a single contiguous constant (no appended examples).
  - ADD TestBuildArbiterSystemPrompt_Properties: a cases table with anti-copy-paste guards:
      "role is reconcile arbiter PRESENT" — Contains "You reconcile leftover changes into commits".
      "JSON contract line PRESENT verbatim" — Contains `Respond with ONLY JSON: {"target": "<sha from the list>"} or {"target": null}.`.
      "'prefer a NEW commit (return null)' PRESENT" — Contains "prefer a NEW commit (return null)".
      "'You may only target a commit from the provided list' PRESENT" — Contains it.
      "em-dash PRESENT (NOT ascii hyphen)" — Contains "return null) — never force a fit" AND NOT Contains "return null) - never force a fit".
      "§17.1 'commit message generator' ABSENT" — NOT Contains "You are a commit message generator".
      "§17.1 anti-reuse block ABSENT" — NOT Contains "CRITICAL: You MUST NOT copy".
      "§17.1 subject-target line ABSENT" — NOT Contains "Target ~".
      "§17.5 'commit-planning assistant' ABSENT" — NOT Contains "You are a commit-planning assistant".
      "§17.5 JSON contract ABSENT" — NOT Contains `{"count": <int>` (the planner's contract).
      "§17.5 planner user-instruction ABSENT" — NOT Contains "Decompose these un-staged changes".
      "§17.6 stager instruction ABSENT" — NOT Contains "Stage, but do NOT commit".
      "§17.6 stager guardrails ABSENT" — NOT Contains "git apply --cached".
      "no <style examples> token" — NOT Contains "<style examples>".
  - ADD TestBuildArbiterUserPayload_CanonicalExact: independently-derived `want` for two commits with files:
      commits := []ArbiterCommit{
          {SHA: "a1b2", Subject: "feat: add login", Files: []string{"a/login.go", "a/login_test.go"}},
          {SHA: "c3d4", Subject: "fix: nil deref", Files: []string{"a/x.go"}},
      }
      leftoverDiff := "diff --git a/left.go b/left.go\n@@ ...s..."
      The want is EXACTLY:
          "The commits made this run (message and changed files for each):\n" +
          "\n" +
          "a1b2\n" +
          "feat: add login\n" +
          "a/login.go\n" +
          "a/login_test.go\n" +
          "\n" +
          "c3d4\n" +
          "fix: nil deref\n" +
          "a/x.go\n" +
          "\n" +
          "A diff of changes that were not included in any commit:\n" +
          "\n" +
          leftoverDiff
    assert got==want with a %q diff.
  - ADD TestBuildArbiterUserPayload_Properties: a cases table:
      "commits header present and starts output" — HasPrefix(arbiterCommitsHeader).
      "leftover header present" — Contains arbiterLeftoverHeader.
      "SHAs present, in order" — for SHAs "AAA","BBB": Index(AAA) < Index(BBB).
      "subjects present" — Contains "feat: add login" AND "fix: nil deref".
      "files present, one per line" — Contains "a/login.go\na/login_test.go".
      "leftover diff is the verbatim tail" — HasSuffix(leftoverDiff).
      "one blank line between commit blocks" — Contains "a/login_test.go\n\nc3d4".
      "one blank line after commits header" — HasPrefix(arbiterCommitsHeader + "\n\n").
      "one blank line after leftover header" — Contains(arbiterLeftoverHeader + "\n\n").
  - ADD TestBuildArbiterUserPayload_EdgeCases:
      "nil commits does not panic" — BuildArbiterUserPayload(nil, "DIFF"); no panic; contains leftover header.
      "empty commits slice" — BuildArbiterUserPayload([]ArbiterCommit{}, "DIFF"); no panic.
      "commit with empty Files" — {SHA:"x", Subject:"s", Files:nil}; output has "x\ns\n\n" (no file lines).
      "empty leftoverDiff" — BuildArbiterUserPayload(commits, ""); no panic; leftover header present.
  - ADD TestParseArbiterOutput: a cases table of {name, raw, wantTarget (string pointer-or-nil sentinel),
    wantErr}. Because *string is awkward in a table, model it as {name, raw, wantNil bool, wantSHA string,
    wantErr bool}:
      "null target → nil" — `{"target": null}` → wantNil=true, wantErr=false.
      "sha target → non-nil" — `{"target": "a1b2c3d4"}` → wantNil=false, wantSHA="a1b2c3d4", wantErr=false.
      "literal placeholder sha (model returns the contract token)" — `{"target": "<sha from the list>"}` → wantSHA="<sha from the list>".
      "JSON in prose (brace-balanced fallback)" — "The answer is {\"target\":\"a1b2\"} — done" → wantSHA="a1b2".
      "JSON in code fence" — "```json\n{\"target\":\"a1b2\"}\n```" → wantSHA="a1b2".
      "leading/trailing whitespace trimmed" — "  \n{\"target\":null}\n  " → wantNil=true.
      "field absent → nil (new-commit default)" — `{}` → wantNil=true.
      "extra unknown fields ignored" — `{"target":"a1b2","extra":"ignored","note":"x"}` → wantSHA="a1b2".
      "empty string target → non-nil empty (caller rejects)" — `{"target": ""}` → wantNil=false, wantSHA="".
      "non-string target (number) → error" — `{"target": 123}` → wantErr=true.
      "non-string target (bool) → error" — `{"target": true}` → wantErr=true.
      "malformed → error" — "not json at all" → wantErr=true, out == zero ArbiterOutput.
      "empty → error" — "" → wantErr=true.
      "unbalanced braces → error" — `{"target":"a1b2"` (no closer) → wantErr=true.
    For success cases: assert err==nil; if wantNil assert out.Target==nil; else assert out.Target != nil &&
    *out.Target == wantSHA. For error cases: assert err != nil; assert out.Target == nil (zero value).
  - ADD TestArbiterOutput_NullSemantics: the dedicated *string tri-state guard:
      parse `{"target": null}` → Target == nil.
      parse `{"target": "a1b2"}` → Target != nil && *Target == "a1b2".
      assert that a nil Target and a pointer-to-empty are DISTINCT: &"" != nil (a Go sanity check pinning
        the design — empty-string-target is non-nil so the caller can distinguish it from null).
  - ADD TestParseArbiterOutput_RoundTrip: Marshal an ArbiterOutput{Target: nil} → parse back → Target==nil;
    Marshal ArbiterOutput{Target: &"a1b2"} → parse back → Target != nil && *Target=="a1b2". (Confirms the
    *string survives marshal/parse.)
  - PLACEMENT: NEW file internal/prompt/arbiter_test.go.

Task 8: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/prompt/arbiter.go internal/prompt/arbiter_test.go`
  - `go build ./...`   (whole module compiles — the new file + test file. CRITICAL: if arbiter.go
    redeclares extractJSONObject, this FAILS with "extractJSONObject redeclared in this package" — remove
    the redefinition and CALL planner.go's copy instead.)
  - `go vet ./...`
  - `golangci-lint run ./internal/prompt/...`   (errcheck/gosimple/govet/ineffassign/staticcheck/unused)
  - `go test -race ./internal/prompt/ -run "Arbiter" -v`   (all new arbiter tests)
  - `go test -race ./internal/prompt/`   (the WHOLE prompt package — system/payload/planner/stager tests still pass)
  - `go test ./...`   (FULL regression — no other package breaks; purely additive)
  - `git diff --exit-code go.mod go.sum` ⇒ empty.
  - `git status --short` ⇒ EXACTLY: `?? internal/prompt/arbiter.go` + `?? internal/prompt/arbiter_test.go`
    (2 entries); planner.go/stager.go/system.go/payload.go/provider/parse.go UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// === arbiter.go: the verbatim §17.7 system prompt constant (backtick raw string, NO trailing newline) ===
// The constant is the ENTIRE §17.7 fenced block. The `<sha from the list>` token is LITERAL instructive text
// (part of the format example), NOT a runtime placeholder. There is NO `<style examples>` token (§17.7 has
// none). NOTE the EM-DASH "—" (U+2014) in "return null) — never force a fit." — the ONE non-ASCII byte; do
// NOT replace it with an ASCII hyphen. §17.7 has NO backticks ⇒ a backtick raw string compiles cleanly.
const arbiterSystemPrompt = `You reconcile leftover changes into commits that were just made. You are given the commits
created this run (with their messages and changed files) and a diff of changes that were not
included in any of them.

Decide: do these leftovers logically belong WITH one of those commits, or do they warrant a
NEW commit?
- Choose an existing commit only if the leftovers are part of the SAME logical change.
- When in doubt, prefer a NEW commit (return null) — never force a fit.
- You may only target a commit from the provided list.

Respond with ONLY JSON: {"target": "<sha from the list>"} or {"target": null}.`

const arbiterCommitsHeader = "The commits made this run (message and changed files for each):"

const arbiterLeftoverHeader = "A diff of changes that were not included in any commit:"


// === BuildArbiterSystemPrompt: returns the verbatim constant (ZERO-arg — §17.7 has no style examples) ===
func BuildArbiterSystemPrompt() string {
	return arbiterSystemPrompt
}


// === BuildArbiterUserPayload: headers + commit blocks + verbatim leftover diff (strings.Builder) ===
func BuildArbiterUserPayload(commits []ArbiterCommit, leftoverDiff string) string {
	var b strings.Builder
	b.WriteString(arbiterCommitsHeader)
	b.WriteByte('\n')
	b.WriteByte('\n') // one blank line before the commit list
	for _, c := range commits {
		b.WriteString(c.SHA)
		b.WriteByte('\n')
		b.WriteString(c.Subject)
		b.WriteByte('\n')
		for _, f := range c.Files {
			b.WriteString(f)
			b.WriteByte('\n') // files one per line
		}
		b.WriteByte('\n') // one blank line separating commit blocks
	}
	b.WriteString(arbiterLeftoverHeader)
	b.WriteByte('\n')
	b.WriteByte('\n') // one blank line before the diff
	b.WriteString(leftoverDiff) // verbatim tail (no normalization)
	return b.String()
}


// === ParseArbiterOutput: whole-Unmarshal → brace-balanced fallback (REUSES planner.go's extractJSONObject) → error ===
// NOTE: extractJSONObject is NOT defined here — it is the planner.go (S1) package-level copy. arbiter.go is in
// the same package (`prompt`), so it calls extractJSONObject directly. Redefining it would be a compile error.
func ParseArbiterOutput(raw string) (ArbiterOutput, error) {
	s := strings.TrimSpace(raw)
	var out ArbiterOutput
	err1 := json.Unmarshal([]byte(s), &out)
	if err1 == nil {
		return out, nil // Attempt 1: whole-string Unmarshal
	}
	if sub, found := extractJSONObject(s); found { // Attempt 2: brace-balanced fallback (planner.go copy)
		if err2 := json.Unmarshal([]byte(sub), &out); err2 == nil {
			return out, nil
		} else {
			return ArbiterOutput{}, fmt.Errorf("arbiter output: not valid JSON: %w", err2)
		}
	}
	return ArbiterOutput{}, fmt.Errorf("arbiter output: not valid JSON: %w", err1)
}
```

### Integration Points

```yaml
DATABASE:
  - none (pure prompt-construction + JSON parse; no git, no IO).

CONFIG:
  - none directly. The []ArbiterCommit + leftoverDiff come from the decompose orchestrator (the commits via
    diff-tree, the diff via WorkingTreeDiff); this layer takes them as plain values — it is decoupled from
    config (mirrors BuildPlannerUserPayload taking diff/concept as strings).

ROUTES:
  - none (internal prompt module; no CLI flag, no public API surface in this task). The decompose arbiter
    (P3.M3.T1.S1) wires the call:
      if dirty {                                                                        # §13.6.5 trigger
          commits := <collect SHA+subject+files via diff-tree for each commit made this run>
          leftoverDiff := <WorkingTreeDiff>                                             # P2.M2.T2.S2
          sys := prompt.BuildArbiterSystemPrompt()                                      # THIS task
          user := prompt.BuildArbiterUserPayload(commits, leftoverDiff)                 # THIS task
          raw, _ := <render(bare) + execute the arbiter agent>                          # P3.M3.T1.S1
          out, err := prompt.ParseArbiterOutput(raw)                                    # THIS task
          if err != nil || out.Target == nil || !inList(out.Target, commits) { <new commit> }   # §13.6.5 ambiguous→null
          else { <resolve target: tip-amend OR mid-chain chain-rebuild> }                        # P3.M3.T2.S1
      }
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the new files (run after creation; fix anything that changes).
gofmt -w internal/prompt/arbiter.go internal/prompt/arbiter_test.go
gofmt -l internal/ pkg/          # Expected: empty (no files need formatting).

# Lint the new files specifically, then the whole module.
golangci-lint run ./internal/prompt/...
golangci-lint run ./...          # Expected: clean (errcheck/gosimple/govet/ineffassign/staticcheck/unused).

# Vet.
go vet ./...                     # Expected: no findings.

# Expected: zero errors. If any exist, READ the output and fix before proceeding. CRITICAL: if
# `go build ./...` fails with "extractJSONObject redeclared in this package", arbiter.go duplicated the
# planner.go function — remove the redefinition and CALL planner.go's copy in ParseArbiterOutput instead.
# NOTE: if stager.go (S2, parallel) is mid-implementation with a build error, arbiter.go must STILL be
# individually gofmt-clean and lint-clean (`gofmt -l internal/prompt/arbiter.go`).
```

### Level 2: Unit Tests (Component Validation)

```bash
# Run the new arbiter tests in isolation (verbose — confirm every case).
go test -race ./internal/prompt/ -run "Arbiter" -v

# Whole prompt package (system.go + payload.go + planner.go + stager.go tests must still pass — the new
# file is additive).
go test -race ./internal/prompt/

# Expected: all pass. The canonical-exact tests are the strongest guards — a mismatch means §17.7 text or
# §2-assembly-topology drift; fix the implementation (or, if the test's independently-derived `want` is wrong,
# re-derive it from §17.7 / findings §2). Do NOT weaken a canonical-exact assertion to make it pass. The
# ArbiterOutput null-semantics + round-trip tests are the strongest guards on the *string Target design.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the whole module (the new file compiles + links; no existing importer breaks).
go build ./...

# Full regression (purely additive — no other package should change behavior).
go test ./...

# Confirm the module is unchanged apart from the two new files.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged"
git status --short                # Expected (at least): ?? internal/prompt/arbiter.go  ?? internal/prompt/arbiter_test.go

# (No live agent / service to start — arbiter.go is a pure library module. The "integration" is the decompose
#  arbiter in P3.M3.T1.S1, which is a SEPARATE work item and does NOT exist yet.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# §17.7 faithfulness self-check (run interactively; eyeball against PRD §17.7):
go test -run TestBuildArbiterSystemPrompt_CanonicalExact -v ./internal/prompt/ && \
  echo "PASS: arbiter system prompt is byte-faithful to §17.7"

# Anti-copy-paste guard (the #1 risk — §17.1/§17.5/§17.6 elements must NOT leak into the arbiter prompt):
go test -run "TestBuildArbiterSystemPrompt_Properties" -v ./internal/prompt/

# *string null-semantics + parse-robustness guards:
go test -run "TestArbiterOutput_NullSemantics|TestParseArbiterOutput" -v ./internal/prompt/

# Defensive spot-check (empty commits / empty Files / empty diff never panic):
go test -run "TestBuildArbiterUserPayload_EdgeCases" -v ./internal/prompt/

# Expected: all pass. The Properties cases (em-dash present, JSON-contract verbatim, §17.1/§17.5/§17.6 ABSENT)
# are the domain-specific validations — they prove the prompt is byte-faithful to §17.7 AND self-contained.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] All tests pass: `go test ./...` (and specifically `go test -race ./internal/prompt/`).
- [ ] No lint errors: `golangci-lint run ./internal/prompt/...` (and `./...`).
- [ ] No vet errors: `go vet ./...`.
- [ ] No formatting issues: `gofmt -l internal/ pkg/` empty.
- [ ] go.mod / go.sum UNCHANGED (`git diff --exit-code go.mod go.sum` ⇒ empty).
- [ ] arbiter.go does NOT redefine `extractJSONObject` (it reuses planner.go's copy) — `go build ./...` has
      no "redeclared in this package" error.

### Feature Validation

- [ ] `arbiterSystemPrompt` is byte-faithful to §17.7 (pinned by the canonical-exact test), incl. the em-dash
      and the LITERAL `<sha from the list>` token; NO trailing newline; NO `<style examples>` token.
- [ ] `BuildArbiterSystemPrompt()` is zero-arg and returns the constant verbatim.
- [ ] `BuildArbiterUserPayload(commits, leftoverDiff)` renders the §2 assembly (SHA+subject+files each; one
      blank line between blocks; leftoverDiff verbatim as the tail); empty/nil commits and empty Files do not
      panic.
- [ ] `ArbiterOutput.Target` is `*string` (json `"target"`); `{"target":null}`→nil, `{"target":"<sha>"}`→
      &"<sha>", `{}`→nil, `{"target":123}`→error.
- [ ] `ParseArbiterOutput` parses clean JSON, JSON-in-prose, JSON-in-code-fence; returns a non-nil error on
      garbage/empty/unbalanced; does NOT validate target-in-list (the caller's job).
- [ ] NO `ArbiterRetryInstruction` is invented (§17.7 does not define one).

### Code Quality Validation

- [ ] Follows existing prompt-package conventions (no-trailing-newline consts; Build-owns-newlines; named
      header consts; rich doc comments citing §17.7; minimal decoupled params; stdlib only).
- [ ] File placement matches the desired tree (2 new files in internal/prompt/).
- [ ] Anti-patterns avoided (no extractJSONObject redeclaration; no trailing-newline constants; no invented
      retry instruction; no redeclared near/suffix helpers; no in-struct validation; no provider import).
- [ ] Dependencies: stdlib only (encoding/json + fmt + strings); no new internal dep; no import cycle.

### Documentation & Deployment

- [ ] Rich doc comments on the package + all consts + both structs + all three functions (cite §17.7; diagram
      the user-payload assembly; note the em-dash / *string null-semantics / no-style-examples / reuse-
      extractJSONObject gotchas; note why no retry instruction).
- [ ] No new environment variables or config (this layer is decoupled from config).
- [ ] Self-documenting via the §17.7 reference (no separate docs file needed, per the work item's DOCS: none).

---

## Anti-Patterns to Avoid

- ❌ Don't define `extractJSONObject` in arbiter.go — planner.go (S1, complete) ALREADY defines it in package
  `prompt`; redefining it is a "redeclared in this package" compile error. CALL planner.go's copy in
  ParseArbiterOutput (findings §4.1 — the #1 trap).
- ❌ Don't put a trailing newline on any string constant — the Build* functions own inter-block newlines.
- ❌ Don't replace the em-dash "—" with an ASCII hyphen "-" — verbatim §17.7 fidelity (findings §1.1).
- ❌ Don't treat `<sha from the list>` as a runtime placeholder — it is LITERAL instructive text in the JSON
  contract line; it stays in `arbiterSystemPrompt` verbatim (findings §1).
- ❌ Don't add an `examples []string` parameter to `BuildArbiterSystemPrompt` — §17.7 has NO style examples
  (unlike §17.5); the function is zero-arg (findings §1.3).
- ❌ Don't invent an `ArbiterRetryInstruction` — §17.7 defines no retry instruction and the work item does not
  list one (findings §6). Don't copy-paste S1's retry-const pattern.
- ❌ Don't make `ArbiterOutput.Target` a plain `string` — the null/sha/absent tri-state needs `*string` so
  nil ⇔ null ⇔ new commit (findings §3.2).
- ❌ Don't add in-struct validation or target-in-list checks inside `ParseArbiterOutput` — it has no list and
  the resolution (new/tip-amend/mid-chain/ambiguous→null) is the consumer's job (P3.M3, §13.6.5).
- ❌ Don't swallow parse failures — `ParseArbiterOutput` returns a non-nil error so the caller can retry /
  default-to-null (findings §4.2).
- ❌ Don't redeclare `near()`/`suffix()` in arbiter_test.go — they already live in system_test.go. Don't import
  provider into arbiter.go — the prompt package is a zero-internal-dep leaf.
- ❌ Don't modify planner.go, stager.go, system.go, payload.go, or any existing file — this task is purely
  additive (2 new files). Don't add config/git params to the Build* functions — they take plain values.
- ❌ Don't weaken a canonical-exact test to make it pass — re-derive `want` from §17.7 / findings §2 if it
  mismatches.

---

**Confidence Score: 9/10** — This is a planner.go (S1) sibling: the same proven pattern (verbatim PRD
constant without trailing newline, Build-owns-newlines, typed JSON-contract structs, a two-attempt Parse*
Output with brace-balanced fallback) applied to §17.7. Zero modifications to existing files (purely
additive), stdlib-only imports, no new internal dependency. The arbiter differs from the planner in exactly
three well-specified ways: (a) zero-arg `BuildArbiterSystemPrompt` (§17.7 has no style examples); (b) a
richer `BuildArbiterUserPayload([]ArbiterCommit, leftoverDiff)` input (the one design-decision point,
fully specified in findings §2 + pinned by a canonical-exact test); (c) `ParseArbiterOutput` REUSES
planner.go's `extractJSONObject` (the #1 trap — redefining it is a compile error; the PRP spells this out
in three places). The `Target *string` null-semantics (nil ⇔ null ⇔ new commit) is the load-bearing design,
fully specified with a json.Unmarshal truth table + dedicated tests. The exported surface exactly matches
the consumer (P3.M3.T1.S1, confirmed by decompose_architecture.md). Parallel-safe with S2 (stager.go) —
two independent standalone files in package prompt. The only residual uncertainty is the user-payload
format (a defensible §17.7-faithful design choice, not verbatim PRD text) — trivially adjustable later
since only the canonical-exact test pins it.
