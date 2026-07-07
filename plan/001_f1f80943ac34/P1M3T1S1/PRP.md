---
name: "P1.M3.T1.S1 — Mature-repo system prompt (style examples + anti-reuse + multi-line rule): the FIRST stage of the generation pipeline's prompt layer — PRD §9.3 (FR10–FR13) / §17.1 / Appendix A"
description: |

  Land the FIRST subtask of Prompt Construction (P1.M3.T1): `internal/prompt/system.go` (`package
  prompt`) exporting TWO pure functions — `BuildSystemPrompt(examples []string, hasMultiline bool,
  subjectTarget int) string` (PRD §17.1, FR13) and `DetectMultiline(examples []string) bool` (FR12) —
  plus the canonical prompt string constants committed VERBATIM from PRD §17.1 (Appendix A). It is the
  CONSUMER of the mature-repo inputs `RecentMessages ([]string)` + `CommitCount` (P1.M1.T3.S3) and the
  PRODUCER of the system-prompt `string` handed to the provider executor (P1.M2.T5) via the orchestrator
  (P1.M3.T4). Sibling subtasks: P1.M3.T1.S2 (new-repo §17.2 prompt) and P1.M3.T1.S3 (user payload §17.3)
  add to the SAME `internal/prompt` package later.

  THE MATURE-REPO PROMPT (PRD §17.1, AUTHORITATIVE — "ported and refined from commit-pi", §17.4 raw-output
  design call). Structure, top to bottom:
    1. Role: "You are a commit message generator."
    2. RAW-output contract: "Output ONLY the commit message. No preamble, no markdown, no code fences,
       no quoting. If a body is warranted, use a blank line between subject and body." (NOT commit-pi's
       JSON contract — §17.4 dropped it.)
    3. Essence-not-filenames instruction (PRD wording).
    4. Examples intro: "Match the tone and style of these recent commits from this repository:" then ONE
       "---" line BEFORE EACH example message (`---\n<msg>`), matching commit-pi's `git log --format=
       "---%n%B"`.
    5. Anti-reuse prohibition block VERBATIM (PRD §17.1), including the EM-DASH (U+2014) in "the STYLE
       to match — format" (commit-pi used an ASCII hyphen; the PRD refined it — use the em-dash).
    6. Multi-line rule conditioned on FR12's detection (two verbatim constants, if/else).
    7. Subject target: fmt.Sprintf("Target ~%d characters for the subject line.", subjectTarget).

  INPUT: `examples []string` from `git.RecentMessages(ctx, 20)` (P1.M1.T3.S3 — newest-first, each element
  a TRIMMED full commit message, NUL-delimited, capped at 100 lines keeping complete messages); the
  orchestrator computes `hasMultiline := DetectMultiline(examples)` (FR12) and passes it in.

  OUTPUT: a system-prompt `string` for the provider executor (P1.M2.T5 `Execute`) via the orchestrator
  (P1.M3.T4 `CommitStaged`). DOCS: none — internal prompt strings (PRD §17 / Appendix A are reference).

  ⚠️ **THE signature — implement the WORK-ITEM signature EXACTLY.** `func BuildSystemPrompt(examples
  []string, hasMultiline bool, subjectTarget int) string` — EXPORTED (caller is `internal/generate`,
  cross-package), returns `string` ONLY (no error — there is no failure mode), takes `hasMultiline` as a
  PARAMETER (decoupling FR12-detect from FR13-construct). See research design-decisions.md §1.

  ⚠️ **THE detection is a SEPARATE exported helper.** Add `func DetectMultiline(examples []string)
  bool` — a faithful port of commit-pi's awk heuristic (`has_multiline=$(… | awk
  '/^---$/{if(lines>1)found=1; lines=0; next} {lines++} END{print found+0}')`): true ⇔ ANY example has
  >1 NON-EMPTY line. Implement via `countNonEmptyLines(msg) > 1` (NOT `strings.Contains(msg, "\n")` — the
  awk strips empty lines then counts; the exact port removes all doubt). See commit-pi-origin.md §2.

  ⚠️ **THE canonical strings come from PRD §17.1, NOT commit-pi.** commit-pi shipped the JSON contract
  ("Return valid JSON only…", "no double quotes"); PRD §17.4 replaced it with raw output. Port the PRD.
  See commit-pi-origin.md §3.

  ⚠️ **THE em-dash.** PRD §17.1 anti-reuse uses `—` (U+2014, UTF-8 0xE2 0x80 0x94) in "the STYLE to
  match — format". commit-pi used `-`. Use the em-dash VERBATIM — it is the ONLY non-ASCII byte in the
  whole prompt and a test asserts its presence. See design-decisions.md §5.

  ⚠️ **THE examples annotation is EXCLUDED.** PRD §17.1's code block contains the line "(up to 20, ≤100
  lines total)" — that is a STRUCTURAL annotation (like `<commit 1 full message>`), NOT literal text.
  commit-pi never emitted it; the caps are enforced upstream by `RecentMessages` (n=20, 100-line cap).
  Do NOT emit it. See commit-pi-origin.md §4.

  ⚠️ **THE subjectTarget wiring.** `fmt.Sprintf("Target ~%d characters for the subject line.",
  subjectTarget)` (preserve the literal `~`). The orchestrator passes `cfg.SubjectTargetChars`
  (P1.M1.T4.S1, default 50). BuildSystemPrompt is decoupled from `config` — pure `(fmt, strings)`. See
  design-decisions.md §6/§7/§8.

  Deliverable: `internal/prompt/system.go` (`package prompt`, imports `fmt`+`strings`) — `BuildSystemPrompt`
  + `DetectMultiline` + unexported `countNonEmptyLines` + the verbatim canonical string constants.
  PLUS `internal/prompt/system_test.go` (`package prompt`, imports `strings`+`testing`) — table-driven
  `TestBuildSystemPrompt` + `TestDetectMultiline` + `TestCountNonEmptyLines`. Touches ONLY these two NEW
  files — NO go.mod/go.sum change (stdlib only), NO edit to any provider/config/git/cmd file.

---

## Goal

**Feature Goal**: Implement the generation pipeline's prompt-construction stage for MATURE repos
(PRD §9.3 FR10–FR13 / §17.1): two pure functions that (a) detect whether the repo's recent history
contains multi-line (subject+body) commits (FR12), and (b) assemble the canonical system prompt — role,
raw-output contract, essence instruction, `---`-separated style examples, the verbatim anti-reuse
prohibition, the multi-line-rule selected by the detection, and the subject-length target (FR13). This is
the "style learning" half of stagecoach's core IP (PRD §13): the model is shown up to 20 real recent
commits so its output matches the repo's conventions, while an explicit prohibition forbids copying the
example text verbatim.

**Deliverable**:
1. **CREATE** `internal/prompt/system.go` (`package prompt`, imports `fmt`, `strings`) —
   - exported `func BuildSystemPrompt(examples []string, hasMultiline bool, subjectTarget int) string`
     assembling the §17.1 prompt (constant header + `---`-separated examples + verbatim anti-reuse +
     if/else multi-line rule + `Target ~%d characters…`),
   - exported `func DetectMultiline(examples []string) bool` (faithful awk port),
   - unexported `func countNonEmptyLines(s string) int`,
   - unexported canonical string constants: `maturePromptHeader`, `antiReuseProhibition`,
     `multilineRuleAllow`, `multilineRuleSingle`.
2. **CREATE** `internal/prompt/system_test.go` (`package prompt`, imports `strings`, `testing`) —
   `TestBuildSystemPrompt` (one exact-match canonical case + a structural-properties table),
   `TestDetectMultiline` (table), `TestCountNonEmptyLines` (table).

No other files touched. **No go.mod/go.sum change** (stdlib `fmt`+`strings` only). NO edit to any file in
`internal/provider/`, `internal/config/`, `internal/git/`, `cmd/`, `pkg/`, the `Makefile`, or any sibling
prompt file (S2/S3 do not exist yet).

**Success Definition**: `go build ./...` succeeds; `go test -race ./internal/prompt/` is green with the
new suite passing; `gofmt -l internal/prompt/` clean; `go vet ./internal/prompt/` clean; `golangci-lint
run` (if available) clean; go.mod/go.sum byte-unchanged; the assembled prompt matches PRD §17.1's blank-
line topology exactly (verified by the exact-match canonical test) INCLUDING the em-dash, the raw-output
contract (NOT JSON), the `---`-before-each-example format, and the EXCLUDED `(up to 20…)` annotation.

## User Persona

**Target User**: The generate orchestrator (P1.M3.T4 `CommitStaged` — calls `DetectMultiline` then
`BuildSystemPrompt` on the mature-repo branch where `CommitCount > 1`), transitively the provider
executor (P1.M2.T5 — receives the assembled string as `sysPrompt`) and every verified agent CLI (pi,
claude, gemini, opencode, codex, cursor). End-user persona is "the plan-holder" / "the API-key
refusenik" / "the multi-agent tinkerer" (PRD §7) whose commit messages must match their repo's existing
conventions without parroting them.

**Use Case**: On a repo with >1 commit, the orchestrator fetches the last 20 full messages
(`git.RecentMessages(ctx, 20)`), detects whether the history uses multi-line bodies
(`prompt.DetectMultiline(recent)`), and assembles the system prompt
(`prompt.BuildSystemPrompt(recent, hasMulti, cfg.SubjectTargetChars)`). That string is passed to the
provider executor as the agent's system prompt; the user payload (diff) arrives separately via stdin
(P1.M3.T1.S3). The result: a commit message in the repo's voice — same format, tone, length, conventions
— but with entirely original wording.

**User Journey**: (internal API, no new end-user surface) `CommitCount > 1` → `RecentMessages(ctx, 20)`
→ `DetectMultiline(msgs)` → `BuildSystemPrompt(msgs, hasMulti, 50)` → `provider.Render(...,
sys, payload)` → `Execute` → `ParseOutput` → dedupe → `CommitTree` + `UpdateRefCAS`.

**Pain Points Addressed**: Generic AI commit messages that ignore a repo's conventions (e.g. emitting
`feat: ...` in a repo that uses `module-123: ...`), AND models that lazily copy an example verbatim. The
style-examples block teaches the convention; the verbatim anti-reuse prohibition forbids copying.

## Why

- **Opens the generation pipeline (P1.M3).** Prompt construction is the FIRST stage of the generation
  flow (PRD §11.1: diff capture → snapshot → **prompt builder** → executor → parse → dedupe → commit).
  P1.M3.T2 (dedupe), P1.M3.T3 (rescue), P1.M3.T4 (orchestrator), and P1.M3.T5 (public API) are all
  blocked until a system-prompt string is producible. S1 is the mature-repo path (the common case).
- **Satisfies PRD §9.3 (FR12 + FR13).** FR12 = detect multi-line history; FR13 = construct the prompt
  with role + raw-output contract + essence + anti-reuse examples + conditioned multi-line rule +
  subject target. This subtask is BOTH FRs (detection as `DetectMultiline`, construction as
  `BuildSystemPrompt`).
- **Implements the v1 design call (§17.4).** Raw output (not JSON) removes the double-quote constraint
  and the fragile `sed` parse. The prompt's output-contract line is the raw contract; JSON mode is a
  per-provider executor/parse concern (P1.M2.T6), NOT a prompt concern.
- **Ports commit-pi faithfully where it matters, refines where the PRD says so.** The `---`-separated
  examples and the awk multi-line heuristic are faithful ports; the output contract, the essence
  wording, and the em-dash are PRD refinements. The PRD wins on conflict (research commit-pi-origin.md
  §3 is the diff table).
- **No new user-facing surface** (PRD "DOCS: none — internal prompt strings"). No new dependency.

## What

A compiled `internal/prompt` package (currently an empty directory) with two new exported pure
functions, one unexported helper, and four unexported canonical string constants, all in a single new
file `system.go`, with a single new test file `system_test.go`. No new types, no I/O, no config, no git,
no subprocess. Both functions are deterministic string transformations; `BuildSystemPrompt` takes
`( []string, bool, int )` and returns `string`, `DetectMultiline` takes `[]string` and returns `bool`.

### Success Criteria

- [ ] `internal/prompt/system.go` exists, `package prompt`, imports EXACTLY `fmt` and `strings` (NO
      third-party, NO `internal/*`). Defines exported `BuildSystemPrompt(examples []string, hasMultiline
      bool, subjectTarget int) string` and `DetectMultiline(examples []string) bool`, plus unexported
      `countNonEmptyLines(s string) int` and the four canonical string constants.
- [ ] `BuildSystemPrompt` reproduces PRD §17.1's blank-line topology EXACTLY: header (role + raw-output
      contract + essence + examples-intro) → `---\n<msg>` per example → blank line → verbatim anti-reuse
      block → blank line → selected multi-line rule → `Target ~%d characters…` (no blank line between
      rule and target). The exact-match canonical test pins this.
- [ ] The output contract is RAW (`Output ONLY the commit message…`); the JSON contract (`Return valid
      JSON`) is ABSENT (we ported the PRD, not commit-pi).
- [ ] The anti-reuse block contains the EM-DASH `—` (U+2014), NOT an ASCII hyphen. A test asserts
      `strings.Contains(prompt, "match — format")`.
- [ ] Each example is preceded by exactly one `---` line; `strings.Count(prompt, "\n---\n")` semantics
      hold (`---` count == `len(examples)` for the non-empty case).
- [ ] The `(up to 20, ≤100 lines total)` annotation is ABSENT from the output.
- [ ] `DetectMultiline` returns `true` iff ANY example has >1 NON-EMPTY line (awk-faithful); `nil`/empty
      → `false`; never panics.
- [ ] `subjectTarget` is interpolated via `fmt.Sprintf("Target ~%d characters for the subject line.",
      subjectTarget)` (literal `~` preserved); a test with `subjectTarget=72` asserts `Target ~72…`.
- [ ] `BuildSystemPrompt(nil, false, 50)` does NOT panic and emits no `---` lines (defensive — §9).
- [ ] `go build ./...` succeeds; `go test -race ./internal/prompt/` green; `gofmt -l internal/prompt/`
      clean; `go vet ./internal/prompt/` clean.
- [ ] go.mod/go.sum byte-unchanged (`git diff --exit-code go.mod go.sum` empty).
- [ ] Every other file byte-unchanged (`internal/provider/*`, `internal/config/*`, `internal/git/*`,
      `cmd/*`, `pkg/*`, `Makefile`).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: PRD §17.1 (the
canonical prompt text, quoted verbatim in the description + Goal), the commit-pi origin diff (research
commit-pi-origin.md — what to port faithfully vs. refine), the 12 design calls (research
design-decisions.md — the single most important read), the input contract (`git.RecentMessages` returns
trimmed newest-first `[]string`), the config wiring (`Config.SubjectTargetChars` default 50), the
in-package table-driven test convention (`git/recentmessages_test.go`), and the copy-ready Go code in
the Implementation Blueprint. No executor/parse/CLI/git-plumbing knowledge required — S1 is two pure
functions + their constants + their tests.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M3T1S1/research/design-decisions.md
  why: the SINGLE most important read — the 12 non-obvious calls: file placement/scope (§0), the
       BuildSystemPrompt signature EXPORTED + string-only + no-error (§1), DetectMultiline as a separate
       helper + the orchestrator wiring (§2), canonical constants from PRD §17.1 NOT commit-pi (§3), the
       examples `---` format + EXCLUDED annotation (§4), the EM-DASH (§5), subjectTarget wiring (§6),
       fmt+strings-only imports + go.mod unchanged (§7), leaf-package no-cycle (§8), empty/nil defensive
       (§9), rule if/else (§10), the exact assembly topology (§11), the test strategy (§12).
  critical: §1 (EXACT signature — exported, string-only, hasMultiline is a PARAM), §2 (DetectMultiline is
       a SEPARATE exported function, faithful awk port via countNonEmptyLines>1), §5 (em-dash VERBATIM),
       §4 (EXCLUDE the "(up to 20…)" annotation), §11 (the exact newline topology) are the things most
       likely to be implemented wrong.

- docfile: plan/001_f1f80943ac34/P1M3T1S1/research/commit-pi-origin.md
  why: the GROUND-TRUTH origin — the verbatim commit-pi prompt (§1), the EXACT awk heuristic + its
       faithful Go port (§2), the diff table commit-pi→PRD §17.1 (§3 — what "refined" changed, so you port
       the RIGHT text), the proof the annotation is excluded (§4), and the exact blank-line topology (§5).
  critical: §3 (the diff table — output contract JSON→raw, essence "specific" dropped, "from this
       repository" added, hyphen→em-dash, "completely"→"entirely", "+length"), §2 (the awk → countNonEmptyLines
       port), §4 (annotation EXCLUDED — commit-pi never emitted it).

- file: internal/git/git.go   (P1.M1.T3.S3 — read for the RecentMessages contract; do NOT edit)
  section: `RecentMessages` (lines ~497–540) — returns `[]string` newest-first, each element a TRIMMED
           full commit message (NUL-delimited split, blanks dropped, capped at 100 lines keeping complete
           messages). `CommitCount` (~588) returns `(int, error)` (decides mature>1 vs new≤1).
  why: the INPUT contract. `BuildSystemPrompt`'s `examples []string` IS the `RecentMessages` return
       value. Knowing messages are TRIMMED (no trailing newline) is why the assembly appends `\n` after
       each. Knowing single-line messages have NO `\n` and multi-line messages DO is why
       countNonEmptyLines works.
  critical: messages are ALREADY trimmed and COMPLETE — do not re-trim or re-cap in BuildSystemPrompt.
       The 100-line cap and the n=20 bound are enforced HERE (RecentMessages), so the prompt must NOT
       re-state them (that is why the `(up to 20…)` annotation is excluded).

- file: internal/config/config.go   (P1.M1.T4.S1 — read for the SubjectTargetChars field; do NOT edit)
  section: `Config.SubjectTargetChars int` (`toml:"subject_target_chars"`, default 50 in `Defaults()`).
  why: confirms the `subjectTarget int` parameter wiring — the orchestrator (P1.M3.T4) passes
       `cfg.SubjectTargetChars`. BuildSystemPrompt is decoupled (takes a plain int), but this proves the
       parameter is not a guess: it is a real config field with a real default.
  critical: do NOT import config into system.go — the parameter decouples them. The default 50 matches
       PRD §17.1's "~50".

- file: internal/git/recentmessages_test.go   (read for the in-package test convention; do NOT edit)
  section: the grouped, documented test-function style + the `makeEmptyCommit`/`initRepo` helpers.
  why: the test STYLE to follow — grouped functions each with a clear name, `// Test…` intent. Note this
       file is git-integration (temp repos); system_test.go is PURE (no git) — mirror the STYLE, not the
       git fixtures.
  critical: tests are IN-PACKAGE (`package git` there → `package prompt` here), white-box. Pure-function
       tests use `strings` assertions, no subprocess.

- url: (PRD §17.1 — already in your context as selected_prd_content; the authoritative prompt text)
  why: the verbatim canonical strings. The header, the anti-reuse block (with em-dash), the two multi-
       line rules, and "Target ~50 characters for the subject line." are ALL copied verbatim from here.
  critical: copy the em-dash `—` verbatim; copy "multi-line" (hyphenated) verbatim; do NOT copy the
       `(up to 20, ≤100 lines total)` line (it is an annotation — see commit-pi-origin.md §4).
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; go-toml/v2 + pflag  (UNCHANGED — S1 adds NO dep: stdlib fmt+strings)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 — untouched (Config.SubjectTargetChars read-only ref)
  generate/                     # P1.M3 (empty stub) — the FUTURE consumer; untouched
  git/                          # P1.M1.T2/T3 — untouched (RecentMessages/CommitCount read-only ref)
  prompt/                       # EMPTY — this subtask creates system.go + system_test.go
    system.go                   # NEW (this subtask) ← BuildSystemPrompt + DetectMultiline + constants
    system_test.go              # NEW (this subtask) ← TestBuildSystemPrompt + TestDetectMultiline + ...
  provider/                     # P1.M2 (T1–T6) — untouched
  ui/                           # P1.M4 (empty stub) — untouched
cmd/stagecoach/main.go           # stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  prompt/
    system.go         # NEW — BuildSystemPrompt + DetectMultiline + countNonEmptyLines + 4 canonical consts
    system_test.go    # NEW — TestBuildSystemPrompt (exact-match + structural) + TestDetectMultiline + TestCountNonEmptyLines
# All other files UNCHANGED. go.mod/go.sum UNCHANGED. After S1: the prompt layer's mature-repo path
# exists; S2 adds the new-repo prompt (§17.2), S3 adds the user payload (§17.3) to the same package.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (signature — EXPORTED, string-only, hasMultiline is a PARAMETER): implement EXACTLY
//   func BuildSystemPrompt(examples []string, hasMultiline bool, subjectTarget int) string
// Exported because the caller is internal/generate (cross-package). Returns string ONLY — there is no
// failure mode (empty examples → empty examples block, defensive; hasMultiline selects a constant;
// subjectTarget formats one line). The orchestrator gates on CommitCount>1 before calling. (design-decisions §1)

// CRITICAL (DetectMultiline is a SEPARATE exported function, faithful awk port): add
//   func DetectMultiline(examples []string) bool
// Returns true iff ANY example has >1 NON-EMPTY line — a faithful port of commit-pi's awk
//   awk '/^---$/{if(lines>1)found=1; lines=0; next} {lines++} END{print found+0}'
// Implement via countNonEmptyLines(msg) > 1 (which uses `line != ""`), NOT strings.Contains(msg, "\n")
// and NOT a TrimSpace-based count. The awk runs over `sed '/^$/d'` output, which strips only
// truly-empty lines — a whitespace-only line SURVIVES and is counted. `line != ""` mirrors that
// EXACTLY; the boolean agrees with the alternatives on RecentMessages' trimmed output, but `line
// != ""` is the literally-faithful port. (commit-pi-origin §2, design-decisions §2)

// CRITICAL (canonical strings from PRD §17.1, NOT commit-pi): commit-pi shipped the JSON contract
// ("Return valid JSON only…", "no double quotes inside the message"); PRD §17.4 replaced it with raw
// output ("Output ONLY the commit message. No preamble, no markdown, no code fences, no quoting. If a
// body is warranted, use a blank line between subject and body."). Port the PRD. (commit-pi-origin §3)

// CRITICAL (the EM-DASH): PRD §17.1 anti-reuse uses "—" (U+2014, UTF-8 0xE2 0x80 0x94) in "the STYLE
// to match — format". commit-pi used "-" (ASCII). Use the em-dash VERBATIM. It is the ONLY non-ASCII
// byte in the whole prompt. A test asserts strings.Contains(p, "match — format"). (design-decisions §5)

// CRITICAL (EXCLUDE the "(up to 20, ≤100 lines total)" annotation): that line in PRD §17.1's code block
// is a STRUCTURAL annotation (like <commit 1 full message>), NOT literal text — commit-pi never emitted
// it, and the caps are enforced upstream by RecentMessages (n=20, 100-line cap). Do NOT emit it.
// (commit-pi-origin §4)

// GOTCHA (examples are TRIMMED): RecentMessages returns each message with NO trailing newline. So the
// assembly must append '\n' after each example so the next "---" starts on its own line: "---\n" + ex +
// "\n". Do NOT re-trim ex (it is already trimmed).

// GOTCHA (blank-line topology is load-bearing): match PRD §17.1 EXACTLY — blank line between the last
// example and CRITICAL; blank line between anti-reuse and the rule; NO blank line between the rule and
// the Target line. Constants are defined WITHOUT trailing newlines; BuildSystemPrompt owns ALL inter-
// block '\n' placement so it lives in exactly one auditable place. (design-decisions §11)

// GOTCHA (subjectTarget ~ is literal): fmt.Sprintf("Target ~%d characters for the subject line.",
// subjectTarget) — the "~" is part of the PRD string, preserved verbatim. Do NOT drop it. The caller
// passes cfg.SubjectTargetChars (default 50); do NOT hardcode 50 in BuildSystemPrompt. (design-decisions §6)

// GOTCHA (imports are fmt + strings ONLY): no internal/config, no internal/git, no internal/provider,
// no third-party. internal/prompt is a LEAF in the import graph. `go mod tidy` MUST be a no-op.
// `git diff --exit-code go.mod go.sum` MUST be empty. (design-decisions §7/§8)

// GOTCHA (in-package tests): system_test.go is `package prompt` (white-box), NOT `package prompt_test`.
// Matches every _test.go in this repo (git/, provider/). Lets the table reference the unexported
// constants + countNonEmptyLines directly. No subprocess, no temp repo — pure-function tests. (design-decisions §12)

// GOTCHA (do NOT pre-empt S2/S3): this file owns the MATURE-repo prompt + DetectMultiline ONLY. The
// new-repo §17.2 prompt (S2) and the user payload §17.3 (S3) are sibling subtasks that add to the SAME
// package later. Do not create payload.go or a new-repo builder here. Keep system.go self-contained.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/prompt/system.go
package prompt

import (
	"fmt"
	"strings"
)

// Canonical prompt string constants — committed VERBATIM from PRD §17.1 (Appendix A). These are the
// "refined from commit-pi" versions: commit-pi shipped the JSON contract; PRD §17.4 replaced it with
// the raw-output contract (no double-quote constraint, no fragile sed parse). See research
// commit-pi-origin.md §3 for the full commit-pi→PRD diff.
//
// Constants are defined WITHOUT trailing newlines; BuildSystemPrompt owns ALL inter-block newline
// placement so the §17.1 blank-line topology lives in exactly one auditable place (design-decisions §11).

// maturePromptHeader is the prompt preamble through the examples-intro line: role, RAW-output contract
// (§17.4 design call — NOT commit-pi's JSON contract), essence instruction, and "Match the tone…" intro.
// Note "from this repository" (PRD refinement; commit-pi had "from these recent commits:").
const maturePromptHeader = `You are a commit message generator.

Output ONLY the commit message. No preamble, no markdown, no code fences,
no quoting. If a body is warranted, use a blank line between subject and body.

Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.

Match the tone and style of these recent commits from this repository:`

// antiReuseProhibition is the verbatim anti-reuse block (PRD §17.1). NOTE the EM-DASH "—" (U+2014) in
// "the STYLE to match — format" — commit-pi used an ASCII hyphen "-"; the PRD refined it to an em-dash.
// It is the ONLY non-ASCII byte in the entire prompt.
const antiReuseProhibition = `CRITICAL: You MUST NOT copy or reuse ANY phrasing from the examples above.
They show the STYLE to match — format, tone, length, conventions. Producing
the same text you have seen is STRICTLY FORBIDDEN. Your output must be
entirely original wording describing THIS specific change. Reusing example
text is a critical failure.`

// The two multi-line rules (PRD §17.1), selected by hasMultiline. Verbatim, including the "multi-line"
// hyphenation. commit-pi's wording is identical here (only cosmetic hyphenation differs).
const (
	// multilineRuleAllow is used when FR12 detected multi-line commits in history.
	multilineRuleAllow = "Only add a body (blank line + description) if the history shows multi-line commits AND these changes truly warrant detailed explanation. Otherwise, use a single-line subject only."
	// multilineRuleSingle is used when the history is single-line only (or examples is empty).
	multilineRuleSingle = "Only output a single-line subject (no body)."
)

// subjectTargetLine renders the PRD §17.1 subject-length target. The "~" is literal (PRD: "Target ~50
// characters for the subject line."). subjectTarget is wired from config.Config.SubjectTargetChars
// (P1.M1.T4.S1, default 50) by the orchestrator (P1.M3.T4); BuildSystemPrompt is decoupled from config.
func subjectTargetLine(subjectTarget int) string {
	return fmt.Sprintf("Target ~%d characters for the subject line.", subjectTarget)
}

// DetectMultiline implements PRD §9.3 FR12: detect whether the recent history contains multi-line
// (subject + body) commits by scanning the examples. It is a faithful port of commit-pi's awk heuristic:
//
//	examples=$(git log --format="---%n%B" -20 | sed '/^$/d' | head -100)
//	has_multiline=$(echo "$examples" | awk '/^---$/{if(lines>1)found=1; lines=0; next} {lines++} END{print found+0}')
//
// The awk counts, per commit (delimited by "---"), the number of NON-EMPTY lines (sed stripped blanks
// first); found=1 if ANY commit had >1 non-empty line. git.RecentMessages (P1.M1.T3.S3) has already
// split on the NUL delimiter, trimmed each message, and capped at 100 lines keeping complete messages,
// so DetectMultiline only needs the per-message ">1 non-empty line" test. It returns true iff ANY
// example has more than one non-empty line. nil/empty → false; never panics.
//
// Why countNonEmptyLines and NOT strings.Contains(msg, "\n"): they agree for every realistic git
// message, but the awk strips empty lines then counts, and countNonEmptyLines mirrors that exactly —
// removing all doubt about whitespace-only body lines (which sed '/^$/d' does NOT strip, so the awk
// counts them, and so does countNonEmptyLines). See research commit-pi-origin.md §2.
func DetectMultiline(examples []string) bool {
	for _, msg := range examples {
		if countNonEmptyLines(msg) > 1 {
			return true
		}
	}
	return false
}

// countNonEmptyLines mirrors commit-pi's `sed '/^$/d'` EXACTLY: a truly-empty line is dropped, a
// whitespace-only line SURVIVES and is counted (sed's /^$/ matches only the empty string — do NOT use
// strings.TrimSpace, which would also drop "   "). It is the per-message embodiment of the awk's
// `lines` counter.
func countNonEmptyLines(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		if line != "" { // exact sed '/^$/d' mirror — NOT strings.TrimSpace
			n++
		}
	}
	return n
}

// BuildSystemPrompt implements PRD §9.3 FR13 / §17.1: assemble the mature-repo system prompt from the
// canonical constants + the repo's recent-commit examples + the detected multi-line flag + the subject
// target. It is the style-learning half of stagecoach's core IP (PRD §13): the model is shown up to 20
// real recent commits so its output matches the repo's conventions, while the verbatim anti-reuse
// prohibition forbids copying the example text.
//
// ASSEMBLY TOPOLOGY (PRD §17.1, exact — see research commit-pi-origin.md §5):
//
//	maturePromptHeader          // "…from this repository:" (no trailing \n)
//	'\n'
//	for each ex: "---\n" + ex + '\n'   (one "---" BEFORE each message; examples are pre-trimmed)
//	'\n'                        // blank line between last example and the anti-reuse block
//	antiReuseProhibition        // "…is a critical failure." (no trailing \n)
//	'\n' '\n'                   // blank line between anti-reuse and the multi-line rule
//	<multi-line rule>           // selected by hasMultiline (no trailing \n)
//	'\n'
//	subjectTargetLine(subjectTarget)   // "Target ~N characters for the subject line."
//
// The "(up to 20, ≤100 lines total)" line from PRD §17.1's code block is INTENTIONALLY EXCLUDED — it is
// a structural annotation, not literal text (commit-pi never emitted it; caps are enforced upstream by
// RecentMessages). See commit-pi-origin.md §4.
//
// WHY hasMultiline is a PARAMETER (not computed inside): §9.3 splits this into FR12 (detect) and FR13
// (construct); BuildSystemPrompt is FR13 and takes the flag as input so detection (DetectMultiline) is
// independently testable and the caller controls it. The orchestrator wires them:
//
//	hasMulti := prompt.DetectMultiline(recent)                                  // FR12
//	sys := prompt.BuildSystemPrompt(recent, hasMulti, cfg.SubjectTargetChars)   // FR13
//
// Defensive: nil/empty examples emit NO "---" lines and no panic (the orchestrator gates on
// CommitCount>1, so examples are non-empty in practice). See design-decisions.md §9.
func BuildSystemPrompt(examples []string, hasMultiline bool, subjectTarget int) string {
	var b strings.Builder

	// Header (role + raw-output contract + essence + examples intro).
	b.WriteString(maturePromptHeader)
	b.WriteByte('\n')

	// Style examples: one "---" line BEFORE each message. RecentMessages returns trimmed messages
	// (no trailing newline), so append '\n' so the next "---" starts on its own line.
	for _, ex := range examples {
		b.WriteString("---\n")
		b.WriteString(ex)
		b.WriteByte('\n')
	}

	// Blank line, then the verbatim anti-reuse prohibition.
	b.WriteByte('\n')
	b.WriteString(antiReuseProhibition)

	// Blank line, then the multi-line rule selected by the detection (FR12 → FR13).
	b.WriteByte('\n')
	b.WriteByte('\n')
	if hasMultiline {
		b.WriteString(multilineRuleAllow)
	} else {
		b.WriteString(multilineRuleSingle)
	}

	// Subject target on its own line (no blank line between rule and target per §17.1).
	b.WriteByte('\n')
	b.WriteString(subjectTargetLine(subjectTarget))

	return b.String()
}
```

```go
// internal/prompt/system_test.go
package prompt

import (
	"strings"
	"testing"
)

// TestBuildSystemPrompt_CanonicalExact asserts the FULL assembled string for a known input, pinning the
// PRD §17.1 blank-line topology byte-for-byte (including the em-dash, the raw-output contract, the
// "---"-before-each-example format, the excluded annotation, and the rule/target placement). This is the
// strongest guard against accidental newline/dash drift. Independently derived from PRD §17.1 (not from
// the implementation) so a match is meaningful.
func TestBuildSystemPrompt_CanonicalExact(t *testing.T) {
	examples := []string{
		"feat: add foo",
		"fix: handle nil deref\n\nThe parser panicked on an unresolved manifest.",
	}
	const subjectTarget = 50
	got := BuildSystemPrompt(examples, true, subjectTarget)

	const want = "You are a commit message generator.\n" +
		"\n" +
		"Output ONLY the commit message. No preamble, no markdown, no code fences,\n" +
		"no quoting. If a body is warranted, use a blank line between subject and body.\n" +
		"\n" +
		"Focus on the ESSENCE of the change (the intent/purpose), not implementation\n" +
		"details like filenames or function names.\n" +
		"\n" +
		"Match the tone and style of these recent commits from this repository:\n" +
		"---\n" +
		"feat: add foo\n" +
		"---\n" +
		"fix: handle nil deref\n" +
		"\n" +
		"The parser panicked on an unresolved manifest.\n" +
		"\n" +
		"CRITICAL: You MUST NOT copy or reuse ANY phrasing from the examples above.\n" +
		"They show the STYLE to match — format, tone, length, conventions. Producing\n" +
		"the same text you have seen is STRICTLY FORBIDDEN. Your output must be\n" +
		"entirely original wording describing THIS specific change. Reusing example\n" +
		"text is a critical failure.\n" +
		"\n" +
		"Only add a body (blank line + description) if the history shows multi-line commits AND these changes truly warrant detailed explanation. Otherwise, use a single-line subject only.\n" +
		"Target ~50 characters for the subject line."

	if got != want {
		// Diff-friendly failure: show where the strings diverge.
		t.Errorf("BuildSystemPrompt mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestBuildSystemPrompt_Properties is a table of structural invariants on the assembled prompt, each
// guarding a specific design decision. These complement the exact-match test by pinning the properties
// that matter most (em-dash, raw-not-JSON contract, "---" count, excluded annotation, rule selection,
// subjectTarget formatting, example ordering).
func TestBuildSystemPrompt_Properties(t *testing.T) {
	singleLine := []string{"feat: one", "chore: two"}
	multiLine := []string{"feat: one\n\nBody one.", "chore: two"}
	cases := []struct {
		name          string
		examples      []string
		hasMultiline  bool
		subjectTarget int
		check         func(t *testing.T, p string)
	}{
		{
			name: "em-dash present (NOT ascii hyphen)", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, "match — format") {
					t.Errorf("anti-reuse block missing em-dash (U+2014); got substring near 'match': %q", near(p, "match"))
				}
				if strings.Contains(p, "match - format") { // ASCII hyphen variant
					t.Errorf("anti-reuse block uses ASCII hyphen '-', expected em-dash '—'")
				}
			},
		},
		{
			name: "raw-output contract present", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, "Output ONLY the commit message. No preamble, no markdown, no code fences") {
					t.Error("raw-output contract missing")
				}
			},
		},
		{
			name: "JSON contract ABSENT (ported PRD not commit-pi)", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if strings.Contains(p, "Return valid JSON") {
					t.Error("commit-pi JSON contract leaked into the PRD prompt")
				}
				if strings.Contains(p, "no double quotes") {
					t.Error("commit-pi 'no double quotes' constraint leaked in")
				}
			},
		},
		{
			name: "(up to 20) annotation ABSENT", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if strings.Contains(p, "(up to 20") || strings.Contains(p, "≤100 lines total") {
					t.Error("structural annotation '(up to 20, ≤100 lines total)' must NOT be in the runtime prompt")
				}
			},
		},
		{
			name: "--- count == len(examples)", examples: []string{"a", "b", "c"}, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if got := strings.Count(p, "---"); got != 3 {
					t.Errorf("--- count = %d, want 3 (one before each example)", got)
				}
			},
		},
		{
			name: "examples appear in order", examples: []string{"ALPHA", "BETA", "GAMMA"}, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				i := strings.Index(p, "ALPHA")
				j := strings.Index(p, "BETA")
				k := strings.Index(p, "GAMMA")
				if i < 0 || j < 0 || k < 0 || !(i < j && j < k) {
					t.Errorf("examples out of order: ALPHA@%d BETA@%d GAMMA@%d", i, j, k)
				}
			},
		},
		{
			name: "hasMultiline=false → single-line rule, allow rule ABSENT", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, multilineRuleSingle) {
					t.Error("expected the single-line rule")
				}
				if strings.Contains(p, multilineRuleAllow) {
					t.Error("the allow-body rule must be ABSENT when hasMultiline=false")
				}
			},
		},
		{
			name: "hasMultiline=true → allow rule, single-line rule ABSENT", examples: multiLine, hasMultiline: true, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, multilineRuleAllow) {
					t.Error("expected the allow-body rule")
				}
				if strings.Contains(p, multilineRuleSingle) {
					t.Error("the single-line rule must be ABSENT when hasMultiline=true")
				}
			},
		},
		{
			name: "subjectTarget interpolated (72)", examples: singleLine, hasMultiline: false, subjectTarget: 72,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, "Target ~72 characters for the subject line.") {
					t.Error("subjectTarget=72 not interpolated")
				}
				if strings.Contains(p, "~50 characters") {
					t.Error("subjectTarget leaked a hardcoded 50")
				}
			},
		},
		{
			name: "no blank line between rule and target", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				want := multilineRuleSingle + "\n" + "Target ~50 characters for the subject line."
				if !strings.Contains(p, want) {
					t.Error("expected the rule immediately followed by the target line (no blank line between)")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, BuildSystemPrompt(tc.examples, tc.hasMultiline, tc.subjectTarget))
		})
	}
}

// TestBuildSystemPrompt_EmptyExamples verifies the defensive path: nil/empty examples must not panic
// and must omit all "---" lines while keeping the header, anti-reuse, rule, and target.
func TestBuildSystemPrompt_EmptyExamples(t *testing.T) {
	for _, ex := range [][]string{nil, {}} {
		p := BuildSystemPrompt(ex, false, 50) // must not panic
		if strings.Contains(p, "---") {
			t.Errorf("empty examples must emit no '---' lines; got %q", p)
		}
		for _, must := range []string{
			"You are a commit message generator.",
			antiReuseProhibition,
			multilineRuleSingle,
			"Target ~50 characters for the subject line.",
		} {
			if !strings.Contains(p, must) {
				t.Errorf("empty-examples prompt missing required block %q", must)
			}
		}
	}
}

// TestDetectMultiline is the table for the FR12 detection (faithful awk port: >1 non-empty line ⇒ true).
func TestDetectMultiline(t *testing.T) {
	cases := []struct {
		name     string
		examples []string
		want     bool
	}{
		{"nil → false", nil, false},
		{"empty → false", []string{}, false},
		{"all single-line → false", []string{"feat: a", "fix: b"}, false},
		{"one single-line → false", []string{"feat: a"}, false},
		{"one multi-line (body) → true", []string{"feat: a\n\nBody text."}, true},
		{"mixed, one multi-line → true", []string{"feat: a", "fix: b\n\nBody."}, true},
		{"whitespace-only body line counts (awk-faithful) → true", []string{"feat: a\n   \nbody"}, true},
		{"subject + trailing blanks trimmed upstream ⇒ single-line here → false", []string{"subject"}, false},
		{"only blanks → 0 non-empty lines → false", []string{"\n\n"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectMultiline(tc.examples); got != tc.want {
				t.Errorf("DetectMultiline(%v) = %v, want %v", tc.examples, got, tc.want)
			}
		})
	}
}

// TestCountNonEmptyLines targets the helper directly (the awk's per-message `lines` counter).
func TestCountNonEmptyLines(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"one", 1},
		{"a\nb", 2},
		{"a\n\nb", 2},    // internal blank not counted
		{"a\n   \nb", 3}, // whitespace-only line SURVIVES (sed '/^$/d' keeps it) → 3 non-empty lines
		{"\n\n", 0},
		{"\n\nfoo\n\n", 1},
	}
	for _, c := range cases {
		if got := countNonEmptyLines(c.in); got != c.want {
			t.Errorf("countNonEmptyLines(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// near returns a short window around the first occurrence of needle in s (for readable failure output).
func near(s, needle string) string {
	i := strings.Index(s, needle)
	if i < 0 {
		return "(needle not found)"
	}
	start := i - 20
	if start < 0 {
		start = 0
	}
	end := i + len(needle) + 20
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/prompt/system.go — constants + BuildSystemPrompt + DetectMultiline
  - PACKAGE: `package prompt`; IMPORTS: EXACTLY "fmt", "strings" (NO internal/*, NO third-party).
  - DEFINE the four unexported canonical string constants VERBATIM from PRD §17.1: maturePromptHeader
      (role + RAW-output contract + essence + "…from this repository:"), antiReuseProhibition (WITH the
      em-dash U+2014), multilineRuleAllow, multilineRuleSingle. NO trailing newlines on constants.
  - IMPLEMENT unexported `countNonEmptyLines(s string) int` (Split on "\n", count lines that are
      non-empty via `line != ""` — exact sed '/^$/d' mirror, NOT TrimSpace).
  - IMPLEMENT exported `DetectMultiline(examples []string) bool` — loop, return true on first message
      with countNonEmptyLines > 1; nil/empty → false.
  - IMPLEMENT exported `subjectTargetLine(subjectTarget int) string` — fmt.Sprintf("Target ~%d characters
      for the subject line.", subjectTarget) (literal "~").
  - IMPLEMENT exported `BuildSystemPrompt(examples []string, hasMultiline bool, subjectTarget int) string`
      per the Data Models assembly (strings.Builder; EXACT topology). Do NOT emit the "(up to 20…)" line.
  - GOTCHA: signature is EXACTLY the work-item form (exported, string-only, hasMultiline is a param).
  - GOTCHA: em-dash in antiReuseProhibition — copy the PRD's "—", not "-".

Task 2: CREATE internal/prompt/system_test.go — table-driven suite
  - PACKAGE: `package prompt` (in-package white-box — matches git/provider test files); IMPORTS:
      "strings", "testing".
  - IMPLEMENT TestBuildSystemPrompt_CanonicalExact (one exact-match case pinning the full §17.1 string).
  - IMPLEMENT TestBuildSystemPrompt_Properties (table of structural checks: em-dash, raw-not-JSON,
      annotation absent, "---" count, example order, rule selection, subjectTarget interp, no-blank-line
      rule→target). Use the unexported constants directly (same-package).
  - IMPLEMENT TestBuildSystemPrompt_EmptyExamples (nil + empty slice: no panic, no "---", all blocks
      present).
  - IMPLEMENT TestDetectMultiline (table: nil/empty/single/mixed/whitespace-body/blanks-only).
  - IMPLEMENT TestCountNonEmptyLines (table) + the `near` failure helper.
  - GOTCHA: no subprocess, no temp repo, no git — pure-function tests.

Task 3: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. Every other file
      (provider/*, config/*, git/*, cmd/*, pkg/*, Makefile) MUST be byte-unchanged. `go test -race
      ./internal/prompt/` MUST be green.
```

### Implementation Patterns & Key Details

```go
// THE assembly — constants carry NO trailing newline; BuildSystemPrompt owns every '\n'. EXACT §17.1.
var b strings.Builder
b.WriteString(maturePromptHeader)             // "…from this repository:"
b.WriteByte('\n')
for _, ex := range examples {                 // "---" BEFORE each trimmed message
	b.WriteString("---\n")
	b.WriteString(ex)
	b.WriteByte('\n')
}
b.WriteByte('\n')                             // blank line → anti-reuse
b.WriteString(antiReuseProhibition)           // "…is a critical failure." (em-dash inside)
b.WriteByte('\n')
b.WriteByte('\n')                             // blank line → rule
if hasMultiline {
	b.WriteString(multilineRuleAllow)
} else {
	b.WriteString(multilineRuleSingle)
}
b.WriteByte('\n')                             // NO blank line → target
b.WriteString(subjectTargetLine(subjectTarget))

// THE detection — faithful awk port (per-message ">1 non-empty line"), NOT strings.Contains.
func DetectMultiline(examples []string) bool {
	for _, msg := range examples {
		if countNonEmptyLines(msg) > 1 {
			return true
		}
	}
	return false
}
func countNonEmptyLines(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		if line != "" { // exact sed '/^$/d' mirror (NOT TrimSpace)
			n++
		}
	}
	return n
}

// THE subjectTarget — literal "~", parameterized int (caller passes cfg.SubjectTargetChars).
fmt.Sprintf("Target ~%d characters for the subject line.", subjectTarget)
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. system.go uses stdlib fmt + strings ONLY. `go mod tidy` MUST be a no-op.
        `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - system.go → (stdlib: fmt, strings) ONLY. It does NOT import internal/config, internal/git,
        internal/provider, os/exec, or any third-party. internal/prompt is a LEAF.
  - system_test.go → (stdlib: strings, testing) ONLY. In-package (package prompt).

UPSTREAM CONTRACT (the inputs — do NOT implement, just consume):
  - P1.M1.T3.S3 git.RecentMessages: returns `[]string` newest-first, each element a TRIMMED full commit
        message (NUL-delimited, ≤100 lines, complete messages only). This IS the `examples` parameter.
  - P1.M1.T3.S3 git.CommitCount: returns `(int, error)`; the orchestrator takes the mature branch when
        count > 1 (it will NOT call BuildSystemPrompt on the new-repo branch — that is S2's §17.2 prompt).
  - P1.M1.T4.S1 config.Config.SubjectTargetChars (int, default 50, toml:"subject_target_chars"): the
        orchestrator passes this as `subjectTarget`.

DOWNSTREAM CONTRACTS (the output — do NOT implement here, just honor the string's role):
  - P1.M3.T4 (orchestrator CommitStaged): `hasMulti := prompt.DetectMultiline(recent)`;
        `sys := prompt.BuildSystemPrompt(recent, hasMulti, cfg.SubjectTargetChars)`;
        then `provider.Render(model, provider, sys, payload)` (P1.M2.T4 — sys is the system_prompt arg).
  - P1.M2.T5 (Execute): runs the agent with `sys` as the system prompt; returns captured stdout.
  - P1.M2.T6 (ParseOutput): turns stdout into the message — independent of the prompt.
  => The `BuildSystemPrompt(...) string` and `DetectMultiline(...) bool` signatures are FROZEN after
     S1. Do not change them. S2 (new-repo prompt) and S3 (user payload) ADD to the package, they do not
     modify these functions.

SIBLING SUBTASKS (same package — do NOT implement, just leave room):
  - P1.M3.T1.S2: the new-repo (≤1 commit) §17.2 conventional-commit prompt. Will add its own function
        (e.g. NewRepoSystemPrompt) to this package. Do NOT create it here.
  - P1.M3.T1.S3: the user payload §17.3 ("Generate a commit message for these changes:" + diff +
        optional rejection block). Will add payload.go to this package. Do NOT create it here.

FROZEN FILES (do NOT edit):
  - internal/provider/*, internal/config/*, internal/git/*, cmd/stagecoach/main.go, pkg/*, Makefile,
        go.mod, go.sum.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the new files
gofmt -w internal/prompt/system.go internal/prompt/system_test.go

# Vet the prompt package (compiles system.go + system_test.go)
go vet ./internal/prompt/

# Confirm the import set is exactly stdlib (no internal/*, no third-party)
head -8 internal/prompt/system.go   # → import ( "fmt" "strings" )

# Confirm go.mod/go.sum are unchanged (stdlib only)
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged ✓"

# Expected: Zero errors. If `go vet` flags an unused import, remove it. The em-dash is fine in a Go
# source file (UTF-8); do not let an editor "fix" it to a hyphen.
```

### Level 2: Unit Tests (THE KEYSTONE — table-driven suite)

```bash
# Run the new suite verbosely (every subtest listed)
go test -race -v ./internal/prompt/ -run TestBuildSystemPrompt
go test -race -v ./internal/prompt/ -run 'TestDetectMultiline|TestCountNonEmptyLines'

# Full prompt package
go test -race ./internal/prompt/

# Coverage of the new file specifically
go test -coverprofile=coverage.out ./internal/prompt/ && go tool cover -func=coverage.out | grep system.go

# Expected: All pass. The canonical exact-match test is the load-bearing one — if it fails, diff the
# (got vs want) %q output the test prints: the usual cause is a misplaced blank line, a hyphen instead
# of the em-dash, or the JSON contract leaking in. Coverage target (PRD §20.3): ≥85% on system.go.
```

### Level 3: Whole-Module Integration (No Regressions)

```bash
# Build everything (system.go compiles into the prompt package)
go build ./...

# Full test suite with the race detector (existing suites MUST stay green)
go test -race ./...

# Lint (if golangci-lint is installed; the Makefile `lint` target)
golangci-lint run ./internal/prompt/ 2>/dev/null || echo "(golangci-lint not installed; skipped)"

# Confirm frozen files are byte-unchanged
git diff --exit-code internal/provider internal/config internal/git cmd pkg Makefile go.mod go.sum \
  && echo "frozen files unchanged ✓"

# Expected: build succeeds; all tests green; go.mod/go.sum + every frozen file byte-unchanged.
```

### Level 4: Correctness Reasoning (No Subprocess Needed)

```bash
# BuildSystemPrompt and DetectMultiline are pure functions — no server, no DB, no subprocess. The
# "integration" is the §17.1 contract. Verify by reasoning + the tests:
#
#   1. The exact-match canonical test pins the FULL §17.1 string (blank-line topology, em-dash,
#      raw-output contract, "---"-before-each-example, rule/target placement).
#   2. The properties table pins each design decision independently (em-dash not hyphen; raw not JSON;
#      annotation absent; "---" count == len; rule selection; subjectTarget interp; no-blank-line
#      rule→target).
#   3. DetectMultiline is the faithful awk port: >1 NON-EMPTY line ⇒ true (countNonEmptyLines, not
#      strings.Contains — the whitespace-only-body row documents the exact-port choice).
#   4. Empty/nil examples never panic (defensive — the orchestrator gates on CommitCount>1).
#
# (No Level-4 commands to run beyond Levels 1–3 — there is no runtime to start. The tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed; `go build ./...` succeeds; `go test -race ./internal/prompt/` green.
- [ ] `go vet ./internal/prompt/` clean; `gofmt -l internal/prompt/` empty.
- [ ] `go mod tidy` is a no-op; `git diff --exit-code go.mod go.sum` empty (stdlib fmt+strings only).
- [ ] `go test -race ./...` green (existing git/provider/config suites — no regressions).
- [ ] Every frozen file byte-unchanged (`git diff --exit-code` on internal/provider, internal/config,
      internal/git, cmd, pkg, Makefile).

### Feature Validation

- [ ] `func BuildSystemPrompt(examples []string, hasMultiline bool, subjectTarget int) string` — EXPORTED,
      string-only (no error), `hasMultiline` is a PARAMETER (not computed inside).
- [ ] `func DetectMultiline(examples []string) bool` — EXPORTED, faithful awk port (>1 non-empty line).
- [ ] The assembled prompt matches PRD §17.1 exactly: raw-output contract (NOT JSON), em-dash (NOT
      hyphen), "---" before each example, blank-line topology, "(up to 20…)" annotation EXCLUDED.
- [ ] `subjectTarget` interpolated as `Target ~%d characters…` (literal "~"); not hardcoded.
- [ ] Empty/nil examples do not panic and emit no "---" lines.

### Code Quality Validation

- [ ] Follows the package's established patterns: in-package white-box tests, table-driven, grouped
      doc-comments, stdlib-only imports, constants defined without trailing newlines.
- [ ] File placement matches the desired codebase tree (system.go + system_test.go in internal/prompt/).
- [ ] Anti-patterns avoided: no JSON contract; no ASCII hyphen for the em-dash; no "(up to 20…)"
      annotation; no `strings.Contains`-based detection; no config/git import; no hardcoded subjectTarget;
      no edit to any frozen file; no pre-emption of S2/S3.
- [ ] Doc comments cite PRD §17.1, §9.3 (FR12/FR13), §17.4, Appendix A, and the commit-pi awk provenance.

### Documentation & Deployment

- [ ] Doc comments on `BuildSystemPrompt`/`DetectMultiline` cite PRD §17.1 (the prompt), §9.3 (FR12/FR13),
      the commit-pi awk heuristic, and the raw-output design call (§17.4).
- [ ] No new environment variables, CLI surface, or user-facing docs (PRD "DOCS: none — internal prompt
      strings").

---

## Anti-Patterns to Avoid

- ❌ Don't port commit-pi's JSON contract ("Return valid JSON only…", "no double quotes") — PRD §17.4
      replaced it with the raw-output contract. Port the PRD §17.1 text. (research commit-pi-origin §3)
- ❌ Don't use an ASCII hyphen "-" for the em-dash — PRD §17.1 uses "—" (U+2014). A test asserts it.
      (research design-decisions §5)
- ❌ Don't emit the "(up to 20, ≤100 lines total)" line — it is a structural annotation, not literal
      text; commit-pi never emitted it; caps are enforced upstream by RecentMessages. (commit-pi-origin §4)
- ❌ Don't compute `hasMultiline` inside `BuildSystemPrompt` — it is a PARAMETER (the work-item signature
      is binding). Detection is the separate `DetectMultiline` helper. (design-decisions §1/§2)
- ❌ Don't implement detection as `strings.Contains(msg, "\n")` — port the awk faithfully via
      `countNonEmptyLines(msg) > 1` (the awk strips empty lines then counts). (commit-pi-origin §2)
- ❌ Don't hardcode `50` for the subject target — interpolate `subjectTarget` via `fmt.Sprintf("Target
      ~%d characters…", n)`. (design-decisions §6)
- ❌ Don't define constants WITH trailing newlines — keep them clean and let BuildSystemPrompt own all
      inter-block `\n` placement (one auditable place). (design-decisions §11)
- ❌ Don't import `internal/config`, `internal/git`, or `internal/provider` — system.go is a leaf
      (fmt+strings only); `go mod tidy` must be a no-op. (design-decisions §7/§8)
- ❌ Don't re-trim or re-cap the examples in BuildSystemPrompt — `RecentMessages` already trimmed them
      and capped at 100 lines keeping complete messages. Just append "\n" after each.
- ❌ Don't write `package prompt_test` (external test) — every _test.go in this repo is in-package
      white-box; mirror that to reach the unexported constants + countNonEmptyLines.
- ❌ Don't pre-empt P1.M3.T1.S2 (new-repo §17.2 prompt) or S3 (user payload §17.3) — this file owns the
      mature-repo prompt + DetectMultiline only. Leave room; don't create payload.go here.

---

## Confidence Score

**9/10** for one-pass implementation success. The deliverable is two pure functions + four string
constants — no I/O, no concurrency, no subprocess, no third-party deps. The canonical prompt text is
pinned verbatim by PRD §17.1 (in-context) and cross-checked against the ACTUAL origin script
commit-pi (read in full — research commit-pi-origin.md is the verbatim diff table, so there is zero
ambiguity about what to port faithfully vs. refine). The 12 non-obvious calls (signature, the separate
DetectMultiline + faithful awk port, em-dash, EXCLUDED annotation, raw-not-JSON, subjectTarget wiring,
blank-line topology, leaf-package imports, empty-defensive) are each backed by an authoritative source
(PRD §17.1/§17.4, the commit-pi script, config.Config.SubjectTargetChars) and pinned by dedicated test
rows (the exact-match canonical test + the 10-row properties table + the DetectMultiline/countNonEmpty
tables). The copy-ready Go in the Implementation Blueprint is independently derived from PRD §17.1 (not
from any existing code), so implementing it produces a test-passing file directly. The one residual
risk — an editor silently converting the em-dash to a hyphen, or a copy-paste dropping a blank line — is
caught by the exact-match test and the `strings.Contains(p, "match — format")` assertion.
