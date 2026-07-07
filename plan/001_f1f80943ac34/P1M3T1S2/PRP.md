---
name: "P1.M3.T1.S2 — New-repo conventional-commit fallback prompt (§17.2): the SECOND stage of the prompt layer — the ≤1-commit branch of PRD §9.3 (FR14) / §17.2 / Appendix A"
description: |

  Land the SECOND subtask of Prompt Construction (P1.M3.T1): ADD to the EXISTING
  `internal/prompt/system.go` (`package prompt`, created by sibling S1) one EXPORTED pure function —
  `BuildFallbackPrompt(subjectTarget int) string` (PRD §17.2, FR14) — plus its one unexported canonical
  string constant `fallbackPromptBody`. It is the new-repo (≤1 commit) counterpart to S1's mature-repo
  `BuildSystemPrompt`: when the orchestrator (P1.M3.T4) finds `git.CommitCount(...) <= 1` it calls THIS
  function instead of S1's. It is the PRODUCER of the system-prompt `string` for the provider executor
  (P1.M2.T5) on the new-repo branch.

  ⚠️ **S2 EDITS two existing files (S1 created them); it does NOT create new files.** APPEND one constant
  + one function to `internal/prompt/system.go`; APPEND test functions to `internal/prompt/system_test.go`.
  S1's mature-repo content must remain byte-for-byte intact (purely additive diff at end of each file).

  THE NEW-REPO FALLBACK PROMPT (PRD §17.2, AUTHORITATIVE — committed VERBATIM, all ASCII, no em-dash):

    You are a commit message generator.

    Output ONLY the commit message. No preamble, no markdown, no code fences.

    Focus on the ESSENCE of the change (the intent/purpose), not implementation
    details like filenames or function names.

    Target ~50 characters (~7 words). Format: type(scope): description

  Four blocks, one blank line between each: (1) role, (2) the SHORT §17.2 output contract, (3) the
  essence instruction (2 lines), (4) the conventional-commit target+format line.

  INPUT: `subjectTarget int` — the orchestrator passes `cfg.SubjectTargetChars` (P1.M1.T4.S1, default 50).
  OUTPUT: a system-prompt `string` for the provider executor (P1.M2.T5 `Execute`) via the orchestrator
  (P1.M3.T4 `CommitStaged`) on the new-repo branch. DOCS: none — internal prompt string (PRD §17 /
  Appendix A are reference).

  ⚠️ **THE signature — implement the WORK-ITEM signature EXACTLY.** `func BuildFallbackPrompt(subjectTarget
  int) string` — EXPORTED (caller is `internal/generate`, cross-package), returns `string` ONLY (no error
  — no failure mode), takes `subjectTarget` as a PARAMETER (decoupled from `config`, exactly like S1's
  `BuildSystemPrompt`). See research design-decisions.md §1.

  ⚠️ **THE key decision — interpolate the CHAR count, keep "(~7 words)" FIXED.** §17.2's last line is
  `Target ~50 characters (~7 words). Format: type(scope): description`. Render it as
  `fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)` —
  ONLY the `50` becomes `%d` (exact analogy with S1's `subjectTargetLine`, where §17.1's `Target ~50
  characters for the subject line.` became `fmt.Sprintf("Target ~%d characters for the subject line.",
  subjectTarget)`). "(~7 words)" stays VERBATIM. With the default `subjectTarget=50` the output is
  BYTE-IDENTICAL to PRD §17.2. See design-decisions.md §2 (this is the load-bearing call).

  ⚠️ **THE verbatim source is PRD §17.2 (the `selected_prd_content` `h3.61` block), NOT the work-item
  paraphrase** (which elides with "…"). Copy the full §17.2 text character-for-character. Unlike §17.1,
  §17.2 has NO em-dash and NO non-ASCII bytes — the `~` is an ASCII tilde (U+007E). design-decisions §4.

  ⚠️ **THE #1 risk is copy-pasting S1's mature constants.** §17.2 is a SMALLER, single-lane prompt (no
  history ⇒ no examples ⇒ no anti-reuse ⇒ single-line subject). It OMITS: the "no quoting" clause, the
  "If a body is warranted" body clause, the "Match the tone and style" examples intro, the `---` example
  markers, the `antiReuseProhibition` block, the multi-line rule, and the "for the subject line."
  wording. It ADDS `Format: type(scope): description` and `(~7 words)`. Tests assert every §17.1 element
  is ABSENT. See design-decisions.md §5.

  ⚠️ **NO new file, NO new import, NO go.mod change.** `system.go` already imports `"fmt"` + `"strings"`
  (S1); `BuildFallbackPrompt` uses `fmt` only. `go mod tidy` is a no-op; `git diff --exit-code go.mod
  go.sum` is empty. `internal/prompt` remains a stdlib-only leaf in the import graph. design-decisions §6/§7.

  Deliverable: EDIT `internal/prompt/system.go` — APPEND `const fallbackPromptBody` (verbatim §17.2 body,
  no trailing newline) + `func BuildFallbackPrompt(subjectTarget int) string` (body + "\n\n" + interpolated
  target line). EDIT `internal/prompt/system_test.go` — APPEND `TestBuildFallbackPrompt_CanonicalExact`
  (exact-match, `subjectTarget=50`) + `TestBuildFallbackPrompt_Properties` (structural table incl. §17.1
  ABSENCE guards) + `TestBuildFallbackPrompt_SubjectTargetInterpolated` (`subjectTarget=72`). Touches
  ONLY these two existing files — NO go.mod/go.sum change, NO edit to any provider/config/git/cmd file,
  NO edit to S1's existing symbols/comments.

---

## Goal

**Feature Goal**: Implement the generation pipeline's prompt-construction stage for NEW repos (PRD §9.3
FR14 / §17.2): one pure function that assembles the canonical conventional-commit fallback system prompt —
role, the short raw-output contract, the essence instruction, and the `type(scope): description`
target/format line — with the user's configured subject-length target interpolated into the character
count. This is the new-repo (≤1 commit) branch of the prompt layer: when the orchestrator finds no
commits to learn style from, it falls back to teaching the Conventional Commits scaffold directly rather
than imitating history.

**Deliverable**:
1. **EDIT** `internal/prompt/system.go` (`package prompt`, imports already `fmt` + `strings`) — APPEND:
   - unexported `const fallbackPromptBody` — the verbatim PRD §17.2 body (blocks 1–3), NO trailing newline,
   - exported `func BuildFallbackPrompt(subjectTarget int) string` — returns `fallbackPromptBody + "\n\n"
     + fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)`.
2. **EDIT** `internal/prompt/system_test.go` (`package prompt`, imports already `strings` + `testing`) —
   APPEND `TestBuildFallbackPrompt_CanonicalExact`, `TestBuildFallbackPrompt_Properties`,
   `TestBuildFallbackPrompt_SubjectTargetInterpolated`.

No other files touched. **No new file, no go.mod/go.sum change** (stdlib `fmt` only, already imported).
NO edit to any file in `internal/provider/`, `internal/config/`, `internal/git/`, `cmd/`, `pkg/`, the
`Makefile`, or S1's existing symbols within the two prompt files.

**Success Definition**: `go build ./...` succeeds; `go test -race ./internal/prompt/` is green with the
new suite passing (and S1's suite still passing — untouched); `gofmt -l internal/prompt/` clean; `go vet
./internal/prompt/` clean; `golangci-lint run` (if available) clean; go.mod/go.sum byte-unchanged; for
`subjectTarget=50` the assembled prompt matches PRD §17.2 byte-for-byte (verified by the exact-match
canonical test); for `subjectTarget=72` only the char count changes while "(~7 words)" survives (verified
by the interpolation test); every §17.1 mature-prompt element is ABSENT from the fallback (verified by
the properties table).

## User Persona

**Target User**: The generate orchestrator (P1.M3.T4 `CommitStaged`) on its NEW-REPO branch — when
`git.CommitCount(ctx) <= 1` it calls `prompt.BuildFallbackPrompt(cfg.SubjectTargetChars)` instead of
S1's `BuildSystemPrompt`. Transitively the provider executor (P1.M2.T5 — receives the assembled string as
`sysPrompt`) and every verified agent CLI (pi, claude, gemini, opencode, codex, cursor). End-user persona
is "the plan-holder" / "the API-key refusenik" / "the multi-agent tinkerer" (PRD §7) whose FIRST commits
on a fresh repo still get a clean Conventional-Commits-style message even though there is no history to
imitate.

**Use Case**: On a repo with ≤1 commit (a brand-new project, or an unborn repo with zero commits), there
are no recent messages to learn style from, so the orchestrator cannot run S1's style-learning path. It
instead assembles the §17.2 fallback: the model is told it is a commit-message generator, given the raw-
output contract, pointed at the essence of the change, and handed the Conventional Commits scaffold
(`type(scope): description`, ~50 chars). The user payload (diff) arrives separately via stdin
(P1.M3.T1.S3). The result: a usable first commit message even with zero prior history.

**User Journey**: (internal API, no new end-user surface) `CommitCount <= 1` →
`BuildFallbackPrompt(cfg.SubjectTargetChars)` → `provider.Render(model, provider, sys, payload)` →
`Execute` → `ParseOutput` → dedupe → `CommitTree` + `UpdateRefCAS`.

**Pain Points Addressed**: Generic or rambling first-commit messages on fresh repos (e.g. "update
files", "initial commit wip"), and the cold-start problem where there is no style to imitate. The
fallback gives the model a concrete, conventional format to target from commit #1.

## Why

- **Completes the prompt layer's two branches (P1.M3.T1).** S1 shipped the mature-repo path (the common
  case, `CommitCount > 1`); S2 ships the new-repo path (the cold-start case, `CommitCount ≤ 1`). Until
  S2 lands, the orchestrator (P1.M3.T4) has no system-prompt string to hand the executor on a fresh
  repo. P1.M3.T4 (orchestrator), P1.M3.T2 (dedupe), P1.M3.T3 (rescue), and P1.M3.T5 (public API) all
  depend on BOTH branches existing.
- **Satisfies PRD §9.3 FR14.** "For repos with ≤1 commit: use a conventional-commit fallback prompt
  (`type(scope): description`, ~50 chars)." S2 IS FR14.
- **Resolves the cold-start gap honestly.** Imitating history (S1) is impossible with zero history; the
  PRD's answer is to teach the Conventional Commits scaffold explicitly. S2 implements that answer.
- **Honors Appendix A's verbatim mandate while staying config-driven.** The body is committed verbatim;
  only the user's configured char target is interpolated (consistent with S1), so the default path is
  byte-canonical and the config is respected.
- **No new user-facing surface** (PRD "DOCS: none — internal"). No new dependency.

## What

Two additive edits to existing files. `internal/prompt/system.go` gains one unexported string constant
(`fallbackPromptBody`) and one exported pure function (`BuildFallbackPrompt`). `internal/prompt/system_test.go`
gains three test functions. No new types, no I/O, no config, no git, no subprocess. The function is a
deterministic string transformation: it takes `( subjectTarget int )` and returns `string`.

### Success Criteria

- [ ] `internal/prompt/system.go` STILL imports EXACTLY `fmt` and `strings` (NO new import, NO third-
      party, NO `internal/*`). It now ALSO defines exported `BuildFallbackPrompt(subjectTarget int) string`
      and unexported `const fallbackPromptBody`.
- [ ] `BuildFallbackPrompt(50)` reproduces PRD §17.2 BYTE-FOR-BYTE — role + blank + short output contract
      + blank + essence (2 lines) + blank + `Target ~50 characters (~7 words). Format: type(scope):
      description`. The exact-match canonical test pins this.
- [ ] The char count is interpolated via `fmt.Sprintf("Target ~%d characters (~7 words). Format:
      type(scope): description", subjectTarget)` (literal `~` preserved); "(~7 words)" is NOT scaled and
      NOT parameterized — it stays verbatim. A test with `subjectTarget=72` asserts `Target ~72
      characters (~7 words)` AND that `~50 characters` is ABSENT AND that `(~7 words)` is STILL present.
- [ ] The §17.1 MATURE-prompt elements are ABSENT from the fallback (anti-copy-paste guards, each a test):
      "no quoting", "If a body is warranted", "Match the tone and style", "---", "CRITICAL: You MUST NOT
      copy", "for the subject line", "multi-line".
- [ ] The §17.2 ADDITIONS are present: `Format: type(scope): description` and `(~7 words)`.
- [ ] `fallbackPromptBody` has NO trailing newline; `BuildFallbackPrompt` owns the single `"\n\n"`
      (exactly one blank line) between the body and the target line. No trailing newline at end of output.
- [ ] S1's symbols are byte-for-byte intact: `BuildSystemPrompt`, `DetectMultiline`,
      `countNonEmptyLines`, `subjectTargetLine`, `maturePromptHeader`, `antiReuseProhibition`,
      `multilineRuleAllow`, `multilineRuleSingle` — unchanged. S1's tests still pass untouched.
- [ ] `go build ./...` succeeds; `go test -race ./internal/prompt/` green; `gofmt -l internal/prompt/`
      clean; `go vet ./internal/prompt/` clean.
- [ ] go.mod/go.sum byte-unchanged (`git diff --exit-code go.mod go.sum` empty).
- [ ] Every other file byte-unchanged (`internal/provider/*`, `internal/config/*`, `internal/git/*`,
      `cmd/*`, `pkg/*`, `Makefile`).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: PRD §17.2 (the
canonical prompt text, quoted verbatim in the description + Goal), the S1 sibling implementation in
`internal/prompt/system.go` (the pattern to mirror: constant-with-no-trailing-newline + builder-owned
`\n` + `fmt.Sprintf` subject-target line), the design decisions (research design-decisions.md — the
single most important read, esp. §2 the interpolation call and §5 the §17.1-vs-§17.2 diff), the config
wiring (`Config.SubjectTargetChars` default 50), the in-package table-driven test convention
(`internal/prompt/system_test.go` from S1), and the copy-ready Go code in the Implementation Blueprint.
No executor/parse/CLI/git-plumbing knowledge required — S2 is one pure function + its constant + its tests.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M3T1S2/research/design-decisions.md
  why: the SINGLE most important read — the 10 decisions specific to S2: MODIFY-not-create scope (§0),
       the EXACT signature (§1), THE interpolation call — char count %d, "(~7 words)" FIXED, default 50
       byte-canonical (§2), the body-constant + inline-Sprintf decomposition (§3), verbatim §17.2 source
       all-ASCII no em-dash (§4), the §17.1-vs-§17.2 diff / anti-copy-paste guards (§5), placement in
       system.go with fmt already imported (§6), leaf-package no-config-import (§7), orchestrator wiring
       downstream (§8), the test strategy (§9), frozen files (§10).
  critical: §2 (THE load-bearing call — interpolate char count ONLY, keep "(~7 words)" verbatim; default
       50 == byte-exact §17.2), §5 (the §17.1 elements that must be ABSENT — the tests pin these), §0
       (EDIT two existing files, append-only, S1 intact) are the things most likely to be implemented wrong.

- file: internal/prompt/system.go   (S1 — READ for the pattern to mirror; you EDIT this file)
  section: the canonical-constant block + `subjectTargetLine` + `BuildSystemPrompt`.
  why: the PATTERN to follow verbatim in style — (a) constants are defined WITHOUT trailing newlines and
       carry an explanatory comment citing the PRD section; (b) the builder function owns ALL inter-block
       `\n` placement via `strings.Builder` or concatenation; (c) the subject-target line is rendered with
       `fmt.Sprintf("Target ~%d characters …", subjectTarget)` preserving the literal `~`; (d) the
       function is EXPORTED, returns `string` only, and is decoupled from `config` (takes a plain int).
  pattern: copy S1's doc-comment density (cite PRD §17.2 / FR14), its constant-naming style
       (`maturePromptHeader` ⇒ `fallbackPromptBody`), and its `fmt`+`strings`-only import discipline.
  gotcha: do NOT reuse S1's `subjectTargetLine` — it emits the §17.1 wording ("…for the subject line."),
          which is WRONG for §17.2. Inline the §17.2-specific `fmt.Sprintf` in `BuildFallbackPrompt`.

- file: internal/prompt/system_test.go   (S1 — READ for the test style; you EDIT this file)
  section: `TestBuildSystemPrompt_CanonicalExact` (exact-match) + `TestBuildSystemPrompt_Properties`
           (structural table) + the `near` failure helper.
  why: the TEST STYLE to mirror — one exact-match canonical case (independently derived from the PRD,
       pinned byte-for-byte, diff-friendly `%q` failure output) PLUS a table of structural property
       checks. APPEND your three new test functions; do NOT touch S1's tests or the `near` helper.
  pattern: in-package `package prompt`; `strings` + `testing`; table-driven; descriptive `t.Run` names;
           assertions via `strings.Contains` / exact `!=`.
  gotcha: this file already declares `near` — do NOT redeclare it (compile error). Reuse it if useful.

- file: internal/config/config.go   (P1.M1.T4.S1 — read for the SubjectTargetChars field; do NOT edit)
  section: `Config.SubjectTargetChars int` (`toml:"subject_target_chars"`, default 50 in `Defaults()`).
  why: confirms the `subjectTarget int` parameter wiring — the orchestrator (P1.M3.T4) passes
       `cfg.SubjectTargetChars`. `BuildFallbackPrompt` is decoupled (takes a plain int), but this proves
       the parameter is not a guess: it is a real config field with a real default, and S1's
       `BuildSystemPrompt` takes the SAME parameter for the SAME reason.
  critical: do NOT import config into system.go — the parameter decouples them. The default 50 matches
            PRD §17.2's "Target ~50 characters", which is why the default path is byte-canonical.

- file: internal/git/git.go   (P1.M1.T3.S3 — read for the CommitCount contract; do NOT edit)
  section: `func (g *gitRunner) CommitCount(ctx) (int, error)` — returns `0` for an unborn repo (git
           exit 128 ⇒ `0, nil`) and the parsed count otherwise.
  why: the UPSTREAM contract. The orchestrator branches `count <= 1` ⇒ `BuildFallbackPrompt` (FR14);
       `count > 1` ⇒ S1's `BuildSystemPrompt`. S2 does NOT implement this branch — it only provides the
       function the branch calls. Knowing `CommitCount` returns `0` (unborn) AND `1` (single root
       commit) for the two ≤1 cases confirms the fallback covers both cold-start situations.
  critical: the `<= 1` branch is the ORCHESTRATOR's (P1.M3.T4), not S2's. Do not implement it here.

- url: (PRD §17.2 — already in your context as selected_prd_content `h3.61`; the authoritative prompt text)
  why: the verbatim canonical string. Copy blocks 1–3 into `fallbackPromptBody` character-for-character,
       and use the last line's `50` as the `%d` interpolation point.
  critical: copy from §17.2 (the full block), NOT the work-item paraphrase (which elides with "…").
            §17.2 has NO em-dash and NO non-ASCII bytes (unlike §17.1) — the `~` is ASCII (U+007E).
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; go-toml/v2 + pflag  (UNCHANGED — S2 adds NO dep: stdlib fmt, already imported)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 — untouched (Config.SubjectTargetChars read-only ref)
  generate/                     # P1.M3 (empty stub) — the FUTURE consumer; untouched
  git/                          # P1.M1.T2/T3 — untouched (CommitCount read-only ref)
  prompt/                       # S1 created system.go + system_test.go; S2 EDITS both (append-only)
    system.go                   # EXISTS (S1) ← S2 APPENDS: const fallbackPromptBody + BuildFallbackPrompt
    system_test.go              # EXISTS (S1) ← S2 APPENDS: 3 TestBuildFallbackPrompt_* functions
  provider/                     # P1.M2 (T1–T6) — untouched
  ui/                           # P1.M4 (empty stub) — untouched
cmd/stagecoach/main.go           # stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  prompt/
    system.go         # EDITED (append) — + const fallbackPromptBody + func BuildFallbackPrompt
    system_test.go    # EDITED (append) — + TestBuildFallbackPrompt_CanonicalExact/Properties/SubjectTargetInterpolated
# All other files UNCHANGED. go.mod/go.sum UNCHANGED. NO new file. After S2: the prompt layer's BOTH
# branches exist (mature §17.1 via S1, new-repo §17.2 via S2); S3 adds the user payload (§17.3).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (THE interpolation call — §2): render the §17.2 target line as
//   fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)
// ONLY the `50` becomes `%d` (exact analogy with S1's subjectTargetLine, where §17.1's
// "Target ~50 characters for the subject line." → "Target ~%d characters for the subject line.").
// "(~7 words)" stays VERBATIM — do NOT scale it (no `subjectTarget/7`), do NOT parameterize it. With
// subjectTarget=50 the output is BYTE-IDENTICAL to PRD §17.2. Scaling would add an undocumented magic
// constant (÷7) + rounding ambiguity + a deviation from the canonical string. (design-decisions §2)

// CRITICAL (signature — EXPORTED, string-only, subjectTarget is a PARAMETER): implement EXACTLY
//   func BuildFallbackPrompt(subjectTarget int) string
// Exported (caller is internal/generate, cross-package). Returns string ONLY — no failure mode. Takes
// subjectTarget as a plain int (decoupled from config; the orchestrator passes cfg.SubjectTargetChars).
// Identical decoupling to S1's BuildSystemPrompt(examples, hasMultiline, subjectTarget). (design-decisions §1)

// CRITICAL (EDIT two EXISTING files — append-only; S1 byte-intact): S1 created system.go +
// system_test.go. S2 APPENDS to both. Do NOT create a new file, do NOT reformat/reorder/rename/touch
// S1's constants/functions/comments. The diff is purely additive at the end of each file. (design-decisions §0/§10)

// CRITICAL (verbatim source = PRD §17.2, NOT the work-item paraphrase): the work item elides the prompt
// with "…". Implement the FULL §17.2 block (selected_prd_content h3.61). Unlike §17.1, §17.2 is ALL
// ASCII — no em-dash, no non-ASCII bytes; the "~" is ASCII tilde U+007E. (design-decisions §4)

// CRITICAL (do NOT reuse S1's subjectTargetLine): it emits §17.1's wording ("…for the subject line."),
// which is WRONG for §17.2. Inline the §17.2-specific fmt.Sprintf inside BuildFallbackPrompt.

// GOTCHA (constant carries NO trailing newline; builder owns the "\n\n"): define fallbackPromptBody as
// §17.2 blocks 1–3 with NO trailing newline; BuildFallbackPrompt joins body + "\n\n" + target line. The
// "\n\n" yields EXACTLY ONE blank line between the essence block and the target line (matches §17.2).
// NO trailing newline at the end of the returned string. (design-decisions §3; mirrors S1's rule)

// GOTCHA (the #1 risk is copy-pasting S1's mature constants): §17.2 is SMALLER and single-lane. It
// OMITS: "no quoting" clause, "If a body is warranted" body clause, "Match the tone and style" examples
// intro, "---" markers, antiReuseProhibition block, multi-line rule, "for the subject line." wording.
// It ADDS: "Format: type(scope): description" and "(~7 words)". Tests assert every omitted §17.1 element
// is ABSENT. (design-decisions §5)

// GOTCHA (no new import, no new dep): system.go already imports "fmt" + "strings" (S1). BuildFallbackPrompt
// uses fmt ONLY. `go mod tidy` MUST be a no-op. `git diff --exit-code go.mod go.sum` MUST be empty.
// internal/prompt stays a stdlib-only LEAF (no internal/config, no internal/git, no third-party). (design-decisions §6/§7)

// GOTCHA (in-package tests, append-only): system_test.go is `package prompt` (white-box). APPEND three
// new test functions; do NOT touch S1's tests. This file already declares `near` — do NOT redeclare it
// (would be a compile error). Reuse `near` if you want diff-friendly failure output. (design-decisions §9)

// GOTCHA (do NOT pre-empt S3 or the orchestrator): S2 owns ONLY BuildFallbackPrompt + its body constant.
// The user payload §17.3 (S3) and the `CommitCount <= 1` branch (P1.M3.T4 orchestrator) are SEPARATE
// subtasks. Do not implement them here. (design-decisions §0/§8)
```

## Implementation Blueprint

### Data models and structure

```go
// internal/prompt/system.go  — APPEND to the existing file (after S1's BuildSystemPrompt). Do NOT touch
// S1's constants/functions. `package prompt` and the `import ("fmt" "strings")` block are unchanged.

// fallbackPromptBody is PRD §17.2 MINUS the final target/format line: role, the SHORT §17.2 raw-output
// contract (no "no quoting", no body clause — new repo ⇒ single-line subject), and the essence
// instruction. Committed VERBATIM from PRD §17.2 (Appendix A). NO trailing newline — BuildFallbackPrompt
// owns the single "\n\n" (one blank line) before the target line, mirroring S1's rule that constants
// carry no trailing newline so inter-block newline placement lives in exactly one auditable place.
//
// Unlike §17.1 (S1), §17.2 is ALL ASCII — no em-dash, no non-ASCII bytes; the "~" in the target line is
// an ASCII tilde. See research design-decisions.md §4.
const fallbackPromptBody = `You are a commit message generator.

Output ONLY the commit message. No preamble, no markdown, no code fences.

Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.`

// BuildFallbackPrompt implements PRD §9.3 FR14 / §17.2: the new-repo (≤1 commit) conventional-commit
// fallback system prompt. When the orchestrator (P1.M3.T4) finds git.CommitCount(...) <= 1 it calls
// THIS instead of S1's BuildSystemPrompt — there is no history to learn style from, so the model is
// taught the Conventional Commits scaffold (type(scope): description) directly.
//
// ASSEMBLY (PRD §17.2, exact):
//
//	fallbackPromptBody                       // role + short output contract + essence (no trailing \n)
//	'\n' '\n'                                // exactly ONE blank line
//	fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)
//
// THE interpolation call (research design-decisions.md §2 — the load-bearing decision): ONLY the `50`
// in §17.2's "Target ~50 characters (~7 words). Format: type(scope): description" becomes `%d` — exact
// analogy with S1's subjectTargetLine (where §17.1's "Target ~50 characters for the subject line."
// became "Target ~%d characters for the subject line."). "(~7 words)" stays VERBATIM (a fixed gloss,
// not a spec; scaling it would add an undocumented ÷7 magic constant). With the default
// cfg.SubjectTargetChars=50 (P1.M1.T4.S1) the output is BYTE-IDENTICAL to PRD §17.2.
//
// Do NOT reuse S1's subjectTargetLine — it emits the §17.1 wording, wrong for §17.2.
//
// Defensive: subjectTarget is a plain int with no failure mode; fmt.Sprintf cannot fail. Returns string
// only (no error). See design-decisions.md §1/§2/§3.
func BuildFallbackPrompt(subjectTarget int) string {
	return fallbackPromptBody + "\n\n" +
		fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)
}
```

```go
// internal/prompt/system_test.go  — APPEND to the existing file (after S1's tests). Do NOT touch S1's
// tests or the `near` helper. `package prompt`; `import ("strings" "testing")` unchanged.

// TestBuildFallbackPrompt_CanonicalExact asserts the FULL assembled string for subjectTarget=50, pinning
// PRD §17.2 byte-for-byte (role + blank + short output contract + blank + 2-line essence + blank + the
// type(scope) target/format line). Independently derived from PRD §17.2 (not from the implementation).
func TestBuildFallbackPrompt_CanonicalExact(t *testing.T) {
	got := BuildFallbackPrompt(50)

	const want = "You are a commit message generator.\n" +
		"\n" +
		"Output ONLY the commit message. No preamble, no markdown, no code fences.\n" +
		"\n" +
		"Focus on the ESSENCE of the change (the intent/purpose), not implementation\n" +
		"details like filenames or function names.\n" +
		"\n" +
		"Target ~50 characters (~7 words). Format: type(scope): description"

	if got != want {
		t.Errorf("BuildFallbackPrompt(50) mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestBuildFallbackPrompt_Properties is a table of structural invariants. It guards (a) every §17.2
// block is present, (b) the §17.2 ADDITIONS are present, and (c) — the anti-copy-paste guards — every
// §17.1 MATURE-prompt element is ABSENT (the #1 implementation risk is copy-pasting S1's constants).
func TestBuildFallbackPrompt_Properties(t *testing.T) {
	p := BuildFallbackPrompt(50)
	cases := []struct {
		name      string
		needle    string
		mustExist bool
	}{
		// §17.2 blocks present.
		{"role present", "You are a commit message generator.", true},
		{"short output contract present", "Output ONLY the commit message. No preamble, no markdown, no code fences.", true},
		{"essence line 1 present", "Focus on the ESSENCE of the change (the intent/purpose), not implementation", true},
		{"essence line 2 present", "details like filenames or function names.", true},
		// §17.2 ADDITIONS present (vs §17.1).
		{"conventional-commit format present", "Format: type(scope): description", true},
		{"~7 words gloss present", "(~7 words)", true},
		// §17.1 MATURE elements ABSENT (anti-copy-paste guards).
		{"§17.1 'no quoting' clause ABSENT", "no quoting", false},
		{"§17.1 body clause ABSENT", "If a body is warranted", false},
		{"§17.1 examples intro ABSENT", "Match the tone and style", false},
		{"§17.1 '---' markers ABSENT", "---", false},
		{"§17.1 anti-reuse block ABSENT", "CRITICAL: You MUST NOT copy", false},
		{"§17.1 'for the subject line' wording ABSENT", "for the subject line", false},
		{"§17.1 multi-line rule ABSENT", "multi-line", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			has := strings.Contains(p, tc.needle)
			if tc.mustExist && !has {
				t.Errorf("expected %q in BuildFallbackPrompt(50); not found", tc.needle)
			}
			if !tc.mustExist && has {
				t.Errorf("BuildFallbackPrompt(50) must NOT contain §17.1 element %q (copy-paste leak)", tc.needle)
			}
		})
	}

	// Blank-line topology: body + exactly ONE blank line + target line; no trailing newline.
	if !strings.HasSuffix(p, "Format: type(scope): description") {
		t.Errorf("prompt must end with the format line (no trailing newline); got suffix %q", suffix(p, 40))
	}
	if strings.HasSuffix(p, "\n") {
		t.Error("prompt must NOT end with a trailing newline")
	}
	if n := strings.Count(p, "\n\n"); n != 3 {
		t.Errorf("expected exactly 3 blank-line separators (\\n\\n) in §17.2; got %d", n)
	}
}

// TestBuildFallbackPrompt_SubjectTargetInterpolated pins §2: a non-default subjectTarget changes ONLY
// the char count; "(~7 words)" survives verbatim; no hardcoded 50 leaks.
func TestBuildFallbackPrompt_SubjectTargetInterpolated(t *testing.T) {
	p := BuildFallbackPrompt(72)
	if !strings.Contains(p, "Target ~72 characters (~7 words). Format: type(scope): description") {
		t.Errorf("subjectTarget=72 not interpolated as expected; got %q", suffix(p, 80))
	}
	if strings.Contains(p, "~50 characters") {
		t.Error("subjectTarget=72 must NOT leak a hardcoded '~50 characters'")
	}
	if !strings.Contains(p, "(~7 words)") {
		t.Error("the fixed '(~7 words)' gloss must survive a non-default subjectTarget (§2)")
	}
}

// suffix returns the last n bytes of s (for readable failure output). (Kept local to the new tests so
// the S1 `near` helper is untouched; if you prefer, reuse `near` — but do NOT redeclare `near`.)
func suffix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/prompt/system.go — APPEND const fallbackPromptBody + func BuildFallbackPrompt
  - FILE: the EXISTING internal/prompt/system.go (created by S1). APPEND at end of file. Do NOT touch the
      `package prompt` line, the `import ("fmt" "strings")` block, or ANY of S1's symbols
      (maturePromptHeader, antiReuseProhibition, multilineRuleAllow, multilineRuleSingle,
      subjectTargetLine, DetectMultiline, countNonEmptyLines, BuildSystemPrompt) or their comments.
  - DEFINE unexported `const fallbackPromptBody` — PRD §17.2 blocks 1–3 VERBATIM, NO trailing newline
      (the raw string literal ends immediately after "...function names.").
  - IMPLEMENT exported `func BuildFallbackPrompt(subjectTarget int) string` returning
      `fallbackPromptBody + "\n\n" + fmt.Sprintf("Target ~%d characters (~7 words). Format:
      type(scope): description", subjectTarget)`.
  - GOTCHA: signature is EXACTLY the work-item form (exported, string-only, subjectTarget is a param).
  - GOTCHA: interpolate ONLY the char count (`50`→`%d`); keep "(~7 words)" verbatim; literal "~" preserved.
  - GOTCHA: do NOT reuse S1's subjectTargetLine (wrong wording). Inline the §17.2 Sprintf.
  - GOTCHA: NO new import (fmt already imported by S1). NO go.mod change.

Task 2: EDIT internal/prompt/system_test.go — APPEND three test functions
  - FILE: the EXISTING internal/prompt/system_test.go (created by S1). APPEND at end of file. Do NOT
      touch S1's tests or the existing `near` helper.
  - IMPLEMENT TestBuildFallbackPrompt_CanonicalExact (subjectTarget=50, exact-match, the full §17.2 string).
  - IMPLEMENT TestBuildFallbackPrompt_Properties (table: §17.2 blocks present, §17.2 additions present,
      §17.1 elements ABSENT, blank-line topology: exactly 3 "\n\n", no trailing newline).
  - IMPLEMENT TestBuildFallbackPrompt_SubjectTargetInterpolated (subjectTarget=72: "~72 characters (~7
      words)" present, "~50 characters" absent, "(~7 words)" still present).
  - ADD a small `suffix(s string, n int) string` helper (OR reuse the existing `near`) — do NOT
      redeclare `near` (compile error).
  - GOTCHA: no subprocess, no temp repo, no git — pure-function tests.

Task 3: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. Every other file
      (provider/*, config/*, git/*, cmd/*, pkg/*, Makefile) MUST be byte-unchanged. S1's symbols within
      the two prompt files MUST be byte-unchanged. `go test -race ./internal/prompt/` MUST be green
      (both S1's and S2's tests).
```

### Implementation Patterns & Key Details

```go
// THE assembly — constant carries NO trailing newline; BuildFallbackPrompt owns the single "\n\n".
// EXACT §17.2 (default subjectTarget=50 is byte-canonical).
func BuildFallbackPrompt(subjectTarget int) string {
	return fallbackPromptBody + "\n\n" +
		fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)
}

// THE interpolation (§2, the load-bearing call): ONLY `50`→`%d`; "(~7 words)" FIXED; literal "~" kept.
// Default 50 ⇒ "Target ~50 characters (~7 words). Format: type(scope): description"  (== PRD §17.2).
// Non-default 72 ⇒ "Target ~72 characters (~7 words). Format: type(scope): description".

// THE constant — verbatim §17.2 blocks 1–3, NO trailing newline.
const fallbackPromptBody = `You are a commit message generator.

Output ONLY the commit message. No preamble, no markdown, no code fences.

Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.`
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. BuildFallbackPrompt uses stdlib fmt ONLY (already imported by S1). `go mod tidy` MUST
        be a no-op. `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - system.go → (stdlib: fmt, strings) ONLY (unchanged by S2). internal/prompt stays a LEAF — no
        internal/config, no internal/git, no internal/provider, no third-party.
  - system_test.go → (stdlib: strings, testing) ONLY (unchanged by S2). In-package (package prompt).

UPSTREAM CONTRACT (the input — do NOT implement, just consume):
  - P1.M1.T4.S1 config.Config.SubjectTargetChars (int, default 50, toml:"subject_target_chars"): the
        orchestrator passes this as `subjectTarget`. Same field S1's BuildSystemPrompt consumes.
  - P1.M1.T3.S3 git.CommitCount: returns `(int, error)` — `0` for unborn repo (git exit 128 ⇒ 0,nil),
        else the parsed count. The orchestrator takes the fallback branch when count <= 1.

DOWNSTREAM CONTRACTS (the output — do NOT implement here, just honor the string's role):
  - P1.M3.T4 (orchestrator CommitStaged): owns the branch —
        if count <= 1 { sys = prompt.BuildFallbackPrompt(cfg.SubjectTargetChars) }
        else          { sys = prompt.BuildSystemPrompt(recent, prompt.DetectMultiline(recent), cfg.SubjectTargetChars) }
        then provider.Render(model, provider, sys, payload) (P1.M2.T4 — sys is the system_prompt arg).
  - P1.M2.T5 (Execute): runs the agent with `sys` as the system prompt; returns captured stdout.
  - P1.M2.T6 (ParseOutput): turns stdout into the message — independent of the prompt.
  => The `BuildFallbackPrompt(subjectTarget int) string` signature is FROZEN after S2. Do not change it.

SIBLING SUBTASKS (same package — do NOT implement, just leave room):
  - P1.M3.T1.S1 (DONE): the mature-repo (§17.1) prompt — BuildSystemPrompt + DetectMultiline. UNCHANGED.
  - P1.M3.T1.S3: the user payload §17.3 ("Generate a commit message for these changes:" + diff + optional
        rejection block). Will add to this package later. Do NOT create it here.

FROZEN FILES (do NOT edit):
  - internal/provider/*, internal/config/*, internal/git/*, cmd/stagecoach/main.go, pkg/*, Makefile,
        go.mod, go.sum, AND S1's existing symbols/comments within internal/prompt/system.go and
        internal/prompt/system_test.go.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the two edited files
gofmt -w internal/prompt/system.go internal/prompt/system_test.go

# Vet the prompt package (compiles system.go + system_test.go, incl. S1 + S2)
go vet ./internal/prompt/

# Confirm NO new import leaked into system.go (still exactly fmt + strings)
sed -n '/^import (/,/^)/p' internal/prompt/system.go   # → "fmt" "strings" only

# Confirm go.mod/go.sum are unchanged (stdlib only)
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged ✓"

# Expected: Zero errors. `go vet` clean. If it flags an unused import, remove it. No editor should
# "normalize" the §17.2 text (it is all ASCII — no em-dash risk here, unlike S1).
```

### Level 2: Unit Tests (THE KEYSTONE — table-driven suite)

```bash
# Run the NEW suite verbosely (every subtest listed)
go test -race -v ./internal/prompt/ -run TestBuildFallbackPrompt

# Confirm S1's suite is STILL green (untouched)
go test -race -v ./internal/prompt/ -run 'TestBuildSystemPrompt|TestDetectMultiline|TestCountNonEmptyLines'

# Full prompt package (S1 + S2 together)
go test -race ./internal/prompt/

# Coverage of the new code specifically
go test -coverprofile=coverage.out ./internal/prompt/ && go tool cover -func=coverage.out | grep -E 'system.go|BuildFallbackPrompt'

# Expected: All pass. The canonical exact-match test (subjectTarget=50) is the load-bearing one — if it
# fails, diff `got` vs `want` (%q output) and fix the §17.2 text byte-for-byte. The Properties table's
# ABSENT checks catch any §17.1 copy-paste leak.
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-module build (S2 is internal; this confirms nothing else broke)
go build ./...

# Optional: confirm the assembled string reads correctly (sanity print, not a test)
go test -run TestBuildFallbackPrompt_CanonicalExact -v ./internal/prompt/

# Expected: `go build ./...` succeeds; the canonical test prints PASS. S2 has no runtime/endpoint/DB
# surface — it is a pure function — so there is no service to start, no curl, no DB to check.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Not applicable for S2 (pure string function, no I/O, no network, no DB, no UI).
# The strongest creative check is human review: open the canonical test's `want` string side-by-side
# with PRD §17.2 (selected_prd_content h3.61) and confirm byte-for-byte equality, then confirm the
# orchestrator wiring contract (Integration Points) matches what P1.M3.T4 will call:
#   prompt.BuildFallbackPrompt(cfg.SubjectTargetChars)  on the count <= 1 branch.
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed successfully.
- [ ] All tests pass: `go test -race ./internal/prompt/` (S1's AND S2's, together).
- [ ] `go build ./...` succeeds.
- [ ] No vet errors: `go vet ./internal/prompt/`.
- [ ] No formatting issues: `gofmt -l internal/prompt/` (empty output).
- [ ] go.mod/go.sum byte-unchanged: `git diff --exit-code go.mod go.sum`.

### Feature Validation

- [ ] `BuildFallbackPrompt(50)` matches PRD §17.2 byte-for-byte (canonical exact-match test).
- [ ] `BuildFallbackPrompt(72)` interpolates the char count ONLY; "(~7 words)" survives (interpolation test).
- [ ] Every §17.1 mature-prompt element is ABSENT from the fallback (properties table).
- [ ] The §17.2 additions (`Format: type(scope): description`, `(~7 words)`) are present.
- [ ] The orchestrator's planned call `prompt.BuildFallbackPrompt(cfg.SubjectTargetChars)` matches the
      frozen signature exactly.
- [ ] No trailing newline; exactly 3 blank-line separators (`\n\n`) in the §17.2 output.

### Code Quality Validation

- [ ] Follows S1's patterns (constant-with-no-trailing-newline, builder-owned `\n`, `fmt.Sprintf`
      subject-target line, EXPORTED string-only function, parameter-decoupled-from-config).
- [ ] Append-only edit to `system.go` + `system_test.go`; S1's symbols byte-for-byte intact.
- [ ] Anti-patterns avoided (check against Anti-Patterns section): no §17.1 copy-paste, no `(~7 words)`
      scaling, no `subjectTargetLine` reuse, no new import, no new file.
- [ ] `internal/prompt` remains a stdlib-only leaf (no config/git/provider import).

### Documentation & Deployment

- [ ] Doc-comments cite PRD §9.3 FR14 / §17.2 and research design-decisions.md (§1/§2/§3).
- [ ] The interpolation decision (§2) is explained in the `BuildFallbackPrompt` doc-comment so a future
      reviewer understands why "(~7 words)" is fixed.
- [ ] No new env vars, no new config fields (the function consumes the existing `SubjectTargetChars`).

---

## Anti-Patterns to Avoid

- ❌ Don't copy-paste S1's mature constants into the fallback — §17.2 is a smaller, single-lane prompt.
  The properties table's ABSENT checks exist to catch exactly this.
- ❌ Don't scale "(~7 words)" with `subjectTarget/7` — it introduces an undocumented magic constant and
  rounding ambiguity, and deviates from the verbatim canonical string. Keep it fixed (§2).
- ❌ Don't reuse S1's `subjectTargetLine` — it emits the §17.1 wording ("…for the subject line."), which
  is wrong for §17.2. Inline the §17.2-specific `fmt.Sprintf`.
- ❌ Don't hardcode `50` and ignore the parameter — the work-item signature deliberately takes
  `subjectTarget int`, and the new-repo path must honor `cfg.SubjectTargetChars` like the mature path.
- ❌ Don't add a trailing newline to `fallbackPromptBody` or to the returned string — S1's rule (constant
  has no trailing newline; builder owns `\n`) applies; §17.2 has no trailing newline.
- ❌ Don't create a new file, add a new import, or change go.mod — `fmt` is already imported (S1).
- ❌ Don't redeclare the `near` helper in `system_test.go` (compile error) — reuse it or add a differently
  named `suffix` helper.
- ❌ Don't implement the `CommitCount <= 1` branch or the user payload (§17.3) — those are the orchestrator
  (P1.M3.T4) and S3 respectively.
- ❌ Don't touch S1's symbols, tests, or comments — the diff is purely append-at-end-of-file.
