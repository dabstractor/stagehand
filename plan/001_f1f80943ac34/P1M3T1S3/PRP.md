---
name: "P1.M3.T1.S3 — User payload assembly (instruction + diff + rejection block): the THIRD and FINAL stage of the prompt layer — PRD §9.3 (FR15) / §9.7 (FR32) / §17.3"
description: |

  Land the THIRD and final subtask of Prompt Construction (P1.M3.T1): CREATE `internal/prompt/payload.go`
  (`package prompt`) exporting ONE pure function — `BuildUserPayload(diff string, rejected []string) string`
  (PRD §17.3, FR15/FR32) — plus its unexported canonical string constants. It is the PRODUCER of the
  user-payload `string` that becomes the stdin content for the provider executor (P1.M2.T5) via the
  orchestrator (P1.M3.T4). Sibling S1 (mature §17.1 system prompt) and S2 (new-repo §17.2 system prompt)
  created `system.go`; S3 creates a SEPARATE new file `payload.go` (zero merge conflict with S2's parallel
  append to `system.go` — see research design-decisions.md §0).

  THE USER PAYLOAD (PRD §17.3, AUTHORITATIVE — re-verified byte-for-byte against prd_snapshot.md
  lines 1117–1142). TWO renderings, one chosen by `len(rejected) == 0`:

  NORMAL (no duplicate rejection):
    Generate a commit message for these changes:        ← COLON
    <blank>
    <diff>

  REJECTION (a duplicate-rejection retry — rejected non-empty):
    Generate a commit message for these changes.        ← PERIOD (NOT colon — see §2 below)
    <blank>
    IMPORTANT: The following messages were REJECTED because they already exist
    in git history. You MUST generate something COMPLETELY DIFFERENT:
    - <rejected subject 1>
    - <rejected subject 2>
    <blank>
    Create an entirely new message with different wording.
    <blank>
    <diff>

  INPUT: `diff string` — the raw `git.StagedDiff(ctx, opts)` return value from P1.M1.T3.S1 (the
  concatenated markdown + non-markdown staged diff, possibly line/byte-capped with a `... [diff truncated]`
  sentinel). `rejected []string` — the duplicate subjects from the dedupe loop (P1.M3.T2, FR30: each is
  "the first line of the message", guaranteed single-line). `len(rejected) == 0` selects the normal path.

  OUTPUT: a user-payload `string` — the 4th argument to `manifest.Render(model, provider, sys, payload)`
  (P1.M2.T4), which becomes `spec.Stdin` for stdin-delivery providers (P1.M2.T5 — render.go line 100:
  `case "stdin": spec.Stdin = payload`). DOCS: none — internal prompt string (PRD §17.3 is reference).

  ⚠️ **THE signature — implement the WORK-ITEM signature EXACTLY.** `func BuildUserPayload(diff string,
  rejected []string) string` — EXPORTED (caller is `internal/generate`, cross-package), returns `string`
  ONLY (no error — there is no failure mode), takes `rejected` as a PARAMETER (decoupled from the dedupe
  loop; the caller controls retry). See research design-decisions.md §1.

  ⚠️ **THE load-bearing call — COLON (normal) vs PERIOD (rejection).** §17.3 renders the instruction line
  DIFFERENTLY in the two cases: `Generate a commit message for these changes:` (colon) in the normal
  block (prd_snapshot.md line 1123) vs `Generate a commit message for these changes.` (period) in the
  rejection block (line 1130). The work-item paraphrase elides this (shows the colon uniformly). The PRD
  §17.3 is AUTHORITATIVE, NOT the paraphrase (exact S1/S2 precedent: "copy §17.x character-for-character").
  Implement TWO instruction constants. The canonical tests pin BOTH. See design-decisions.md §2.

  ⚠️ **THE diff is appended VERBATIM — no trailing-newline normalization.** `BuildUserPayload` is a pure
  assembler. The diff's shape (incl. whether it ends with `\n`) is `git.StagedDiff`'s contract; the
  payload's trailing bytes == the diff's trailing bytes. Do NOT TrimSpace, re-cap, or alter the diff.
  (design-decisions.md §6)

  ⚠️ **THE rejection-block `<...>` tokens are annotation placeholders, NOT literal text.** `<diff payload>`,
  `<rejected subject 1>`, `<rejected subject 2>` are structural annotations (like §17.1's excluded
  `(up to 20…)` line). The diff is the runtime `diff` arg; the subjects are the runtime `rejected` slice
  elements. §17.3 is ENTIRELY ASCII — no em-dash (unlike §17.1), no non-ASCII bytes. (design-decisions.md §5)

  ⚠️ **NO edit to system.go / system_test.go (S1/S2 own those); NEW file payload.go + payload_test.go.**
  This sidesteps the parallel S2 append-to-system.go merge collision. NO new import beyond `"strings"`
  (NOT even `"fmt"` — there is no `%d` interpolation here, unlike S1/S2). `internal/prompt` stays a
  stdlib-only leaf. `go mod tidy` MUST be a no-op; `git diff --exit-code go.mod go.sum` MUST be empty.
  (design-decisions.md §0/§9)

  Deliverable: CREATE `internal/prompt/payload.go` — 4 unexported canonical string constants (verbatim
  §17.3, NO trailing newlines) + exported `func BuildUserPayload(diff string, rejected []string) string`
  (normal fast-path `userInstruction + "\n\n" + diff`; rejection path via `strings.Builder`). PLUS
  `internal/prompt/payload_test.go` — `TestBuildUserPayload_NormalCanonicalExact` +
  `TestBuildUserPayload_RejectionCanonicalExact` + `TestBuildUserPayload_Properties` +
  `TestBuildUserPayload_EdgeCases`. Touches ONLY these two NEW files — NO go.mod/go.sum change, NO edit to
  system.go, system_test.go, any provider/config/git/cmd file, or the Makefile.

---

## Goal

**Feature Goal**: Implement the generation pipeline's user-payload stage (PRD §9.3 FR15 / §9.7 FR32 /
§17.3): one pure function that assembles the canonical user prompt delivered to the agent via stdin — a
short stable instruction ("Generate a commit message for these changes") followed by the staged diff,
with an optional duplicate-rejection block inserted between the instruction and the diff on a retry. This
completes the prompt layer (P1.M3.T1): S1/S2 produce the SYSTEM prompt; S3 produces the USER payload.
Together they are the two strings the orchestrator hands `manifest.Render(model, provider, sys, payload)`.

**Deliverable**:
1. **CREATE** `internal/prompt/payload.go` (`package prompt`, imports `"strings"` ONLY) —
   - unexported `const userInstruction` — `Generate a commit message for these changes:` (§17.3 normal, colon),
   - unexported `const userInstructionReject` — `Generate a commit message for these changes.` (§17.3 rejection, period),
   - unexported `const rejectionPreamble` — the two-line `IMPORTANT: …\n…DIFFERENT:` block (raw string literal),
   - unexported `const rejectionEpilogue` — `Create an entirely new message with different wording.`,
   - exported `func BuildUserPayload(diff string, rejected []string) string`.
2. **CREATE** `internal/prompt/payload_test.go` (`package prompt`, imports `strings` + `testing`) —
   `TestBuildUserPayload_NormalCanonicalExact`, `TestBuildUserPayload_RejectionCanonicalExact`,
   `TestBuildUserPayload_Properties`, `TestBuildUserPayload_EdgeCases`.

No other files touched. **No new file besides the two above. No go.mod/go.sum change** (stdlib `strings`
only). NO edit to `internal/prompt/system.go`, `internal/prompt/system_test.go` (S1/S2 own those), or any
file in `internal/provider/`, `internal/config/`, `internal/git/`, `cmd/`, `pkg/`, the `Makefile`.

**Success Definition**: `go build ./...` succeeds; `go test -race ./internal/prompt/` is green with the
new suite passing (and S1/S2's suites still passing — untouched); `gofmt -l internal/prompt/` clean; `go
vet ./internal/prompt/` clean; `golangci-lint run` (if available) clean; go.mod/go.sum byte-unchanged; the
normal payload matches PRD §17.3's normal block byte-for-byte (colon instruction + blank + diff); the
rejection payload matches PRD §17.3's rejection block byte-for-byte (period instruction + blank + IMPORTANT
preamble + per-subject `- ` list + blank + epilogue + blank + diff); the diff is always the exact tail
of the output in both paths.

## User Persona

**Target User**: The generate orchestrator (P1.M3.T4 `CommitStaged`) — on EVERY attempt it calls
`prompt.BuildUserPayload(diff, rejected)`; `rejected` is `[]string{}` on the first attempt and non-empty
(the matched duplicate subjects) on a duplicate-rejection retry (P1.M3.T2 / FR32, up to
`max_duplicate_retries` default 3). Transitively the provider manifest's `Render` (P1.M2.T4 — receives the
string as `userPayload`, the 4th arg), the executor (P1.M2.T5 — pipes it as `spec.Stdin` for stdin
delivery), and every verified agent CLI (pi, claude, gemini, opencode, codex, cursor). End-user persona is
"the plan-holder" / "the API-key refusenik" / "the multi-agent tinkerer" (PRD §7) whose staged changes are
summarized into a commit message, and whose duplicate-subject retries get a clearly-worded "don't reuse
this" nudge.

**Use Case**: After confirming staged changes (`git.HasStagedChanges`) and capturing the diff
(`git.StagedDiff`), the orchestrator assembles the user payload: a short instruction + the diff, piped to
the agent via stdin (never as a CLI arg — FR15: avoids arg-length limits and shell injection). If the
generated subject duplicates one of the last 50 commits (FR31/FR32), the dedupe loop retries generation
with `rejected` non-empty, and `BuildUserPayload` inserts the §17.3 rejection block — listing the rejected
subjects with an explicit "you MUST generate something COMPLETELY DIFFERENT" directive — between the
instruction and the diff.

**User Journey**: (internal API, no new end-user surface)
`HasStagedChanges` → `StagedDiff` → `BuildUserPayload(diff, [])` (attempt 1) → `Render(..., sys, payload)`
→ `Execute` → `ParseOutput` → dedupe check → (if duplicate) `BuildUserPayload(diff, [dupSubjects])` → retry
→ accept → `CommitTree` + `UpdateRefCAS`. The retry's `BuildUserPayload` call is the ONLY place the
rejection block is emitted.

**Pain Points Addressed**: (1) Shell-injection / arg-length failures from passing a large diff as a CLI
arg — solved by assembling a stdin payload string (FR15). (2) The model regenerating the SAME duplicate
subject on retry — solved by the explicit §17.3 rejection block naming the rejected subjects and demanding
entirely new wording (FR32). (3) Inconsistent payload formatting across providers — solved by one
canonical assembler all providers consume identically.

## Why

- **Completes the prompt layer (P1.M3.T1) — the LAST of three subtasks.** S1 shipped the mature-repo
  system prompt (§17.1); S2 ships the new-repo system prompt (§17.2); S3 ships the user payload (§17.3).
  Until S3 lands, the orchestrator (P1.M3.T4) has the system prompt but NO user payload to feed
  `Render`'s 4th argument. P1.M3.T4 (orchestrator), P1.M3.T2 (dedupe), P1.M3.T3 (rescue), and P1.M3.T5
  (public API) all depend on the prompt layer being complete.
- **Satisfies PRD §9.3 FR15 + §9.7 FR32.** FR15 = the user instruction + diff via stdin; FR32 = on a
  duplicate-rejection retry, append an explicit rejection list to the user prompt. S3 IS both FRs.
- **Delivers stdin payload as the design intends (§17.4 raw-output call).** The user payload is raw text
  (instruction + diff), not JSON; the executor (P1.M2.T5) pipes it to stdin and parses the agent's stdout.
  S3 assembles exactly that raw stdin string.
- **No new user-facing surface** (PRD "DOCS: none — internal"). No new dependency (stdlib `strings` only).

## What

A new file `internal/prompt/payload.go` with one exported pure function and four unexported canonical
string constants, plus a new test file `payload_test.go`. No new types, no I/O, no config, no git, no
subprocess. The function is a deterministic string transformation: it takes `( diff string, rejected
[]string )` and returns `string`. `len(rejected) == 0` selects the normal (colon) fast-path; non-empty
selects the rejection (period) path with the §17.3 rejection block.

### Success Criteria

- [ ] `internal/prompt/payload.go` exists, `package prompt`, imports EXACTLY `"strings"` (NO `"fmt"`, NO
      third-party, NO `internal/*`). Defines exported `BuildUserPayload(diff string, rejected []string)
      string` and four unexported string constants (`userInstruction`, `userInstructionReject`,
      `rejectionPreamble`, `rejectionEpilogue`).
- [ ] `BuildUserPayload("DIFF", nil)` and `BuildUserPayload("DIFF", []string{})` BOTH return
      `"Generate a commit message for these changes:\n\nDIFF"` — the §17.3 NORMAL rendering, COLON
      instruction, blank line, then the diff verbatim. The normal canonical test pins this.
- [ ] `BuildUserPayload("DIFF", []string{"a", "b"})` returns the §17.3 REJECTION rendering byte-for-byte:
      PERIOD instruction + blank + the two-line IMPORTANT preamble + `- a\n- b\n` + blank + epilogue +
      blank + `DIFF`. The rejection canonical test pins this.
- [ ] The instruction's terminal punctuation is CONTEXT-CORRECT: COLON in the normal path, PERIOD in the
      rejection path (NOT colon in both). A properties check asserts `strings.HasSuffix`/`Contains`
      discriminates the two.
- [ ] Each rejected subject appears on its own line prefixed with exactly `"- "`; for N subjects there are
      exactly N `- `-prefixed list lines, in input order.
- [ ] The diff is ALWAYS the exact tail of the output in BOTH paths (`strings.HasSuffix(out, diff)`),
      appended VERBATIM (no trim, no newline normalization).
- [ ] `BuildUserPayload("", nil)` does not panic (defensive) and returns
      `"Generate a commit message for these changes:\n\n"` (empty diff tail).
- [ ] `go build ./...` succeeds; `go test -race ./internal/prompt/` green (S1's AND S2's AND S3's, together);
      `gofmt -l internal/prompt/` clean; `go vet ./internal/prompt/` clean.
- [ ] go.mod/go.sum byte-unchanged (`git diff --exit-code go.mod go.sum` empty).
- [ ] Every other file byte-unchanged (`system.go`, `system_test.go`, `internal/provider/*`,
      `internal/config/*`, `internal/git/*`, `cmd/*`, `pkg/*`, `Makefile`).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: PRD §17.3 (the two
canonical payload renderings, quoted verbatim in the description + Goal + this section), the design
decisions (research design-decisions.md — the single most important read, esp. §2 the colon/period call
and §3 the rejection-block topology), the S1/S2 sibling implementation in `internal/prompt/system.go` (the
pattern to mirror: constant-with-no-trailing-newline + builder-owned `\n` + exported string-only function),
the diff input contract (`git.StagedDiff` returns a possibly-sentinel-terminated `string`), the downstream
consumer (`render.go` `Render(..., userPayload)` → `spec.Stdin`), the in-package table-driven test
convention (`internal/prompt/system_test.go` from S1/S2), and the copy-ready Go code in the Implementation
Blueprint. No executor/parse/CLI/git-plumbing knowledge required — S3 is one pure function + its constants
+ its tests.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M3T1S3/research/design-decisions.md
  why: the SINGLE most important read — the 11 decisions specific to S3: NEW file payload.go not append-to-system.go (§0),
       the EXACT signature (§1), THE colon-vs-period call (§2 — load-bearing), the rejection-block topology (§3), constants
       carry no trailing newline (§4), all-ASCII + <...> are annotation placeholders (§5), diff appended verbatim (§6),
       len(rejected)==0 branch (§7), subjects single-line per FR30 / no sanitization (§8), imports "strings" ONLY not even fmt (§9),
       test strategy (§10), downstream Render/Execute wiring (§11).
  critical: §2 (THE load-bearing call — colon normal, period rejection, §17.3 authoritative not the paraphrase), §3 (the exact
       rejection-block newline topology), §0 (NEW file, zero merge conflict with S2) are the things most likely to be implemented wrong.

- file: internal/prompt/system.go   (S1+S2 — READ for the pattern to mirror; do NOT edit)
  section: the canonical-constant block (maturePromptHeader/antiReuseProhibition/fallbackPromptBody) + BuildSystemPrompt
           (strings.Builder assembly) + BuildFallbackPrompt (constant + "\n\n" + Sprintf).
  why: the PATTERN to follow verbatim in style — (a) canonical string constants are defined WITHOUT trailing newlines and carry an
       explanatory comment citing the PRD section; (b) the builder function owns ALL inter-block `\n` placement; (c) the function is
       EXPORTED, returns `string` only, and is decoupled from `config`/`git`; (d) a short fast-path (BuildFallbackPrompt's single-line
       `body + "\n\n" + Sprintf`) is acceptable for the no-loop case.
  pattern: copy S1/S2's doc-comment density (cite PRD §17.3 / FR15 / FR32), its constant-naming style, and its single-stdlib-import
           discipline. Use `strings.Builder` for the rejection path (the subject loop), exactly as BuildSystemPrompt uses it for the
           examples loop.
  gotcha: do NOT reuse S1/S2 constants — §17.3's strings are different text. Define payload.go's OWN constants. And note: payload.go
          needs `"strings"` ONLY (no `"fmt"` — there is no `%d` here), so its import block is SMALLER than system.go's.

- file: internal/git/git.go   (P1.M1.T3.S1 — read for the StagedDiff contract; do NOT edit)
  section: `func (g *gitRunner) StagedDiff(ctx, opts StagedDiffOptions) (string, error)` (line ~398) — returns the concatenated
           markdown + non-markdown staged diff. Markdown part appends `\n` per file for a clean boundary; non-markdown part is written
           as-is; a byte/line cap appends a `... [diff truncated at N bytes/lines]` sentinel WITHOUT a trailing newline.
  why: the INPUT contract — the `diff string` argument. Confirms the diff MAY OR MAY NOT end with `\n`, which is WHY BuildUserPayload
       appends it verbatim (no normalization): the diff's trailing bytes are StagedDiff's responsibility, not the payload assembler's.
  critical: do NOT TrimSpace/re-cap/alter the diff in BuildUserPayload. The orchestrator gates on HasStagedChanges first, so diff is
            non-empty in practice; empty-diff is a defensive no-panic case.

- file: internal/provider/render.go   (P1.M2.T4 — read for the downstream consumer; do NOT edit)
  section: `func (m Manifest) Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error)` (line ~55) — userPayload is the
           4th arg. `case "stdin": spec.Stdin = payload` (line ~100). When `system_prompt_flag == ""`, Render PREPENDS sysPrompt
           (`payload = sysPrompt + "\n\n" + userPayload`) — that prepend is Render's concern, not BuildUserPayload's.
  why: confirms BuildUserPayload's return value is LITERALLY the stdin content for stdin-delivery providers, and that the system-prompt
       prepend happens downstream (so BuildUserPayload must NOT prepend the system prompt itself — §17.3 owns only the user payload).
  critical: the `BuildUserPayload(diff string, rejected []string) string` signature is FROZEN after S3 — Render's 4th param depends on it.

- url: (PRD §17.3 — already in your context as selected_prd_content `h3.62`; ALSO re-verified in
       plan/001_f1f80943ac34/prd_snapshot.md lines 1117–1142)
  why: the verbatim canonical strings. Copy the instruction, the IMPORTANT preamble (two lines), and the epilogue character-for-character.
       Use the normal block's colon and the rejection block's period — BOTH are intentional §17.3 renderings (design-decisions §2).
  critical: copy from §17.3 (the full block in prd_snapshot.md), NOT the work-item paraphrase (which elides the period). §17.3 is ALL
            ASCII (no em-dash, unlike §17.1). The `<diff payload>` / `<rejected subject N>` tokens are annotation placeholders, NOT
            literal output — replace them with the runtime `diff` arg and `rejected` slice elements.
- url: (PRD §9.7 FR30–FR32 + Appendix B.4 — already in context; the dedupe contract + the end-to-end retry example)
  why: FR30 ("subject = first line of the message") guarantees each `rejected` element is single-line (no embedded newline) — this is
       WHY `"- " + s + "\n"` always yields exactly one list line and no sanitization is needed (design-decisions §8). FR32 confirms the
       rejection-list retry semantics. B.4 shows the retry in action ("Attempt 1 … matches … retrying").
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagehand ; go 1.22 ; go-toml/v2 + pflag  (UNCHANGED — S3 adds NO dep: stdlib strings only)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 — untouched (read-only ref: MaxDuplicateRetries default 3, SubjectTargetChars default 50)
  generate/                     # P1.M3 (empty stub) — the FUTURE consumer; untouched
  git/                          # P1.M1.T2/T3 — untouched (StagedDiff read-only ref)
  prompt/                       # S1 created system.go; S2 appends to it; S3 CREATES payload.go + payload_test.go (NEW files)
    system.go                   # EXISTS (S1+S2) — UNCHANGED by S3 (BuildSystemPrompt / BuildFallbackPrompt / DetectMultiline / constants)
    system_test.go              # EXISTS (S1+S2) — UNCHANGED by S3
    payload.go                  # NEW (this subtask) ← 4 canonical consts + BuildUserPayload
    payload_test.go             # NEW (this subtask) ← TestBuildUserPayload_* (4 functions)
  provider/                     # P1.M2 (T1–T6) — untouched (render.go Render is read-only ref)
  ui/                           # P1.M4 (empty stub) — untouched
cmd/stagehand/main.go           # stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  prompt/
    payload.go        # NEW — const userInstruction / userInstructionReject / rejectionPreamble / rejectionEpilogue + BuildUserPayload
    payload_test.go   # NEW — TestBuildUserPayload_NormalCanonicalExact / RejectionCanonicalExact / Properties / EdgeCases
# All other files UNCHANGED. go.mod/go.sum UNCHANGED. After S3: the prompt layer is COMPLETE (mature §17.1 via S1,
# new-repo §17.2 via S2, user payload §17.3 via S3); P1.M3.T4 (orchestrator) can now call BuildSystemPrompt OR
# BuildFallbackPrompt for the system prompt AND BuildUserPayload for the user payload.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (THE colon-vs-period call — §2): §17.3 renders the instruction line DIFFERENTLY in the two cases:
//   NORMAL:     "Generate a commit message for these changes:"   (COLON — prd_snapshot.md line 1123)
//   REJECTION:  "Generate a commit message for these changes."   (PERIOD — prd_snapshot.md line 1130)
// The work-item paraphrase elides this (shows the colon uniformly and says "insert the rejection block between
// the instruction and the diff"). The PRD §17.3 is AUTHORITATIVE, NOT the paraphrase (exact S1/S2 precedent).
// Implement TWO instruction constants (userInstruction colon, userInstructionReject period). Canonical tests pin BOTH.
// (design-decisions.md §2 — the single most error-prone decision)

// CRITICAL (signature — EXPORTED, string-only, rejected is a PARAMETER): implement EXACTLY
//   func BuildUserPayload(diff string, rejected []string) string
// Exported (caller is internal/generate, cross-package). Returns string ONLY — no failure mode (empty diff ⇒
// instruction + optional block with empty tail; empty/nil rejected ⇒ normal colon path). Takes rejected as a
// plain []string (decoupled from the dedupe loop; the caller controls retry count). (design-decisions.md §1)

// CRITICAL (NEW file payload.go — do NOT edit system.go): S2 (implementing in parallel) APPENDS to system.go;
// if S3 also appends to system.go the two appends collide. A new file sidesteps the merge. Same `package prompt`.
// Touches ONLY payload.go + payload_test.go. (design-decisions.md §0)

// CRITICAL (the diff is appended VERBATIM): do NOT TrimSpace, re-cap, or normalize the diff's trailing newline.
// The payload's trailing bytes == the diff's trailing bytes (StagedDiff owns the diff's shape, incl. whether it
// ends with \n and the ... [diff truncated] sentinel). BuildUserPayload is a PURE assembler. (design-decisions.md §6)

// CRITICAL (the <...> tokens are annotation placeholders, NOT literal): "<diff payload>", "<rejected subject 1>",
// "<rejected subject 2>" in the PRD code block are structural annotations (like §17.1's excluded "(up to 20…)" line).
// Replace them with the runtime `diff` arg and `rejected` slice elements. Do NOT emit literal angle brackets.
// §17.3 is ENTIRELY ASCII — no em-dash (unlike §17.1's anti-reuse block), no non-ASCII bytes. (design-decisions.md §5)

// GOTCHA (len(rejected) == 0 is the branch — nil and []string{} are EQUIVALENT): both have length 0 ⇒ both take the
// normal (colon) path. The dedupe loop (P1.M3.T2) passes []string{} on attempt 1 and non-empty on retries. (§7)

// GOTCHA (subjects are single-line per FR30 — NO sanitization): each rejected element is "the first line of the message"
// (FR30), guaranteed no embedded newline, so "- " + s + "\n" always yields exactly one list line. Do NOT TrimSpace or
// re-split — BuildUserPayload trusts the upstream contract (as S1 trusts RecentMessages' trimming). (§8)

// GOTCHA (constants carry NO trailing newline; the builder owns every '\n'): same rule as S1/S2. Each canonical constant
// is defined WITHOUT a trailing newline; BuildUserPayload owns all inter-block newline placement so the §17.3 blank-line
// topology lives in exactly one auditable place (the builder body). (design-decisions.md §4)

// GOTCHA (imports: "strings" ONLY — NOT even "fmt"): there is NO %d/%v interpolation here (unlike S1/S2's
// subject-target line). The normal path is plain `+` concatenation; the rejection path uses strings.Builder (the subject
// loop). `go mod tidy` MUST be a no-op. internal/prompt stays a stdlib-only LEAF. (design-decisions.md §9)

// GOTCHA (in-package tests, NEW file): payload_test.go is `package prompt` (white-box — same as system_test.go).
// Reuse the `near`/`suffix` helpers from system_test.go if useful (same package — already defined; do NOT redeclare).
// No subprocess, no temp repo, no git — pure-function tests. (design-decisions.md §10)

// GOTCHA (do NOT pre-empt the orchestrator or the dedupe loop): S3 owns ONLY BuildUserPayload + its constants. The
// retry counter (max_duplicate_retries, default 3), the subject extraction (FR30), the 50-subject fetch (FR31), and the
// Render/Execute call are P1.M3.T2 / P1.M3.T4 / P1.M2.T4 / P1.M2.T5 respectively. Do not implement them here. (§11)
```

## Implementation Blueprint

### Data models and structure

```go
// internal/prompt/payload.go
package prompt

import "strings"

// Canonical user-payload string constants — committed VERBATIM from PRD §17.3 (re-verified against
// prd_snapshot.md lines 1117–1142). The user payload is the stdin content for the provider executor
// (P1.M2.T5 — render.go line 100 `case "stdin": spec.Stdin = payload`); the diff is NEVER a CLI arg
// (PRD §9.3 FR15: avoids arg-length limits and shell injection).
//
// Constants are defined WITHOUT trailing newlines; BuildUserPayload owns ALL inter-block newline
// placement so the §17.3 blank-line topology lives in exactly one auditable place (mirrors S1/S2's rule).
//
// §17.3 is ENTIRELY ASCII — no em-dash (unlike §17.1's anti-reuse block), no non-ASCII bytes. The
// `<diff payload>` / `<rejected subject N>` tokens in the PRD code block are STRUCTURAL annotations
// (placeholders for dynamic content), NOT literal output — BuildUserPayload substitutes the runtime
// `diff` arg and `rejected` slice elements. See research design-decisions.md §5.

// userInstruction is the §17.3 NORMAL instruction (FR15): the stable user-facing string that introduces
// the diff directly. NOTE the trailing COLON — §17.3's normal block ends the line with ":" (prd_snapshot.md
// line 1123). Used when len(rejected) == 0.
const userInstruction = "Generate a commit message for these changes:"

// userInstructionReject is the §17.3 REJECTION instruction: the SAME stable string but ending with a PERIOD
// (prd_snapshot.md line 1130), because the rejection block's "IMPORTANT: …" directive follows on its own.
// NOTE: this is INTENTIONAL §17.3 fidelity — the work-item paraphrase elides the period (shows the colon
// uniformly), but the PRD §17.3 is authoritative (exact S1/S2 precedent: copy §17.x character-for-character).
// See research design-decisions.md §2 — the single most error-prone decision; a canonical test pins it.
const userInstructionReject = "Generate a commit message for these changes."

// rejectionPreamble is the two-line §17.3 IMPORTANT block header that opens the rejection list. Committed
// VERBATIM (the line break falls after "already exist", exactly as in §17.3). NO trailing newline —
// BuildUserPayload appends '\n' then the per-subject list. Used only when len(rejected) > 0.
const rejectionPreamble = `IMPORTANT: The following messages were REJECTED because they already exist
in git history. You MUST generate something COMPLETELY DIFFERENT:`

// rejectionEpilogue is the single-line §17.3 closing directive, emitted AFTER the rejected-subject list.
// Committed VERBATIM. NO trailing newline — BuildUserPayload appends "\n\n" (one blank line) then the diff.
const rejectionEpilogue = "Create an entirely new message with different wording."

// BuildUserPayload implements PRD §9.3 FR15 / §9.7 FR32 / §17.3: assemble the user payload delivered to
// the agent via stdin (never as a CLI arg — FR15). It is the user-payload half of the prompt layer (the
// system-prompt half is S1/S2's BuildSystemPrompt / BuildFallbackPrompt). The orchestrator (P1.M3.T4)
// calls it on EVERY generation attempt:
//
//	payload := prompt.BuildUserPayload(diff, rejected)
//	spec, _ := manifest.Render(model, provider, sys, payload)   // payload is the 4th arg (P1.M2.T4)
//
// `rejected` is []string{} on the first attempt and non-empty (the matched duplicate subjects, FR30:
// each "the first line of the message") on a duplicate-rejection retry (FR32, up to
// max_duplicate_retries default 3). len(rejected) == 0 selects the normal path.
//
// ASSEMBLY (PRD §17.3, exact — see research design-decisions.md §3):
//
//	NORMAL (len(rejected) == 0):
//	  userInstruction + "\n\n" + diff
//	    → "Generate a commit message for these changes:\n\n<diff>"   (COLON instruction + blank + diff)
//
//	REJECTION (len(rejected) > 0):
//	  userInstructionReject + "\n\n"          // PERIOD instruction + blank line
//	  + rejectionPreamble + "\n"              // the two-line IMPORTANT header, then end its 2nd line
//	  + for each s in rejected: "- " + s + "\n"   // one list line per rejected subject (single-line per FR30)
//	  + "\n"                                  // blank line after the list
//	  + rejectionEpilogue + "\n\n"            // "Create an entirely new message…" + blank line
//	  + diff                                  // appended VERBATIM (no normalization)
//
// THE colon-vs-period call (design-decisions.md §2): the normal instruction ends ":" (introduces the diff),
// the rejection instruction ends "." (the IMPORTANT directive follows). BOTH are §17.3 renderings; the
// work-item paraphrase elides the period but the PRD is authoritative.
//
// The diff is appended VERBATIM — its trailing bytes (including whether it ends with '\n' and any
// StagedDiff truncation sentinel) are preserved. The diff's shape is git.StagedDiff's contract (P1.M1.T3.S1),
// not this assembler's. See design-decisions.md §6.
//
// Defensive: nil/empty rejected ⇒ normal path (len(nil)==0); empty diff ⇒ instruction (+ optional block)
// with an empty tail — no panic (the orchestrator gates on HasStagedChanges, so diff is non-empty in
// practice). Subjects are assumed single-line (FR30) — no sanitization (design-decisions.md §8).
func BuildUserPayload(diff string, rejected []string) string {
	if len(rejected) == 0 {
		// §17.3 NORMAL: colon instruction + blank line + diff (verbatim). Fast path — no loop, no Builder.
		return userInstruction + "\n\n" + diff
	}

	// §17.3 REJECTION: period instruction + blank + IMPORTANT preamble + per-subject list + blank + epilogue + blank + diff.
	var b strings.Builder
	b.WriteString(userInstructionReject)
	b.WriteString("\n\n")
	b.WriteString(rejectionPreamble)
	b.WriteByte('\n') // end the preamble's second line, then the list
	for _, s := range rejected {
		b.WriteString("- ")
		b.WriteString(s)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')                // blank line after the list
	b.WriteString(rejectionEpilogue) // "Create an entirely new message with different wording."
	b.WriteString("\n\n")            // blank line before the diff
	b.WriteString(diff)              // verbatim
	return b.String()
}
```

```go
// internal/prompt/payload_test.go
package prompt

import (
	"strings"
	"testing"
)

// TestBuildUserPayload_NormalCanonicalExact asserts the FULL assembled NORMAL payload (empty/nil rejected)
// is byte-for-byte the §17.3 normal rendering: COLON instruction + blank line + diff verbatim. Independently
// derived from PRD §17.3 (not from the implementation) so a match is meaningful. Pins the colon.
func TestBuildUserPayload_NormalCanonicalExact(t *testing.T) {
	const diff = "diff --git a/foo.go b/foo.go\n@@ -1,3 +1,4 @@"
	want := "Generate a commit message for these changes:\n" +
		"\n" +
		diff

	for _, rej := range [][]string{nil, {}} { // nil and empty are equivalent → normal path
		if got := BuildUserPayload(diff, rej); got != want {
			t.Errorf("BuildUserPayload(diff, %v) mismatch:\n--- got ---\n%q\n--- want ---\n%q", rej, got, want)
		}
	}
}

// TestBuildUserPayload_RejectionCanonicalExact asserts the FULL assembled REJECTION payload (non-empty
// rejected) is byte-for-byte the §17.3 rejection rendering: PERIOD instruction + blank + two-line IMPORTANT
// preamble + per-subject "- " list + blank + epilogue + blank + diff. Pins the period (NOT colon) and the
// exact blank-line topology. Independently derived from PRD §17.3.
func TestBuildUserPayload_RejectionCanonicalExact(t *testing.T) {
	const diff = "diff --git a/foo.go b/foo.go\n@@ -1,3 +1,4 @@"
	rejected := []string{"fix: handle null user", "feat: add bar"}
	got := BuildUserPayload(diff, rejected)

	want := "Generate a commit message for these changes.\n" + // PERIOD
		"\n" +
		"IMPORTANT: The following messages were REJECTED because they already exist\n" +
		"in git history. You MUST generate something COMPLETELY DIFFERENT:\n" +
		"- fix: handle null user\n" +
		"- feat: add bar\n" +
		"\n" +
		"Create an entirely new message with different wording.\n" +
		"\n" +
		diff

	if got != want {
		t.Errorf("BuildUserPayload rejection mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestBuildUserPayload_Properties is a table of structural invariants guarding the load-bearing decisions:
// the colon-vs-period distinction, the per-subject "- " list, the diff-always-the-tail rule, and the
// presence/absence of the rejection block in each path.
func TestBuildUserPayload_Properties(t *testing.T) {
	const diff = "DIFFCONTENT"
	cases := []struct {
		name     string
		diff     string
		rejected []string
		check    func(t *testing.T, p string)
	}{
		{
			name: "normal: instruction ends with COLON", diff: diff, rejected: nil,
			check: func(t *testing.T, p string) {
				if !strings.HasPrefix(p, "Generate a commit message for these changes:\n\n") {
					t.Errorf("normal payload must start with colon instruction + blank line; got %q", near(p, "Generate"))
				}
			},
		},
		{
			name: "normal: rejection block ABSENT", diff: diff, rejected: nil,
			check: func(t *testing.T, p string) {
				for _, absent := range []string{"IMPORTANT:", "REJECTED", "Create an entirely new message"} {
					if strings.Contains(p, absent) {
						t.Errorf("normal payload must NOT contain rejection element %q", absent)
					}
				}
			},
		},
		{
			name: "rejection: instruction ends with PERIOD (not colon)", diff: diff, rejected: []string{"a"},
			check: func(t *testing.T, p string) {
				if !strings.HasPrefix(p, "Generate a commit message for these changes.\n\n") {
					t.Errorf("rejection payload must start with PERIOD instruction + blank line; got %q", near(p, "Generate"))
				}
				if strings.HasPrefix(p, "Generate a commit message for these changes:") { // colon variant
					t.Error("rejection instruction must end with PERIOD, not COLON (design-decisions §2)")
				}
			},
		},
		{
			name: "rejection: IMPORTANT preamble + epilogue present", diff: diff, rejected: []string{"a"},
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, "IMPORTANT: The following messages were REJECTED because they already exist") {
					t.Error("rejection preamble line 1 missing")
				}
				if !strings.Contains(p, "in git history. You MUST generate something COMPLETELY DIFFERENT:") {
					t.Error("rejection preamble line 2 missing")
				}
				if !strings.Contains(p, "Create an entirely new message with different wording.") {
					t.Error("rejection epilogue missing")
				}
			},
		},
		{
			name: "rejection: each subject on its own '- ' line, in order", diff: diff, rejected: []string{"ONE", "TWO", "THREE"},
			check: func(t *testing.T, p string) {
				for _, want := range []string{"- ONE\n", "- TWO\n", "- THREE\n"} {
					if !strings.Contains(p, want) {
						t.Errorf("rejection list missing line %q", want)
					}
				}
				if got := strings.Count(p, "\n- "); got != 3 { // 3 subjects ⇒ 3 "- "-prefixed lines
					t.Errorf("expected 3 '- '-prefixed list lines; got %d", got)
				}
				// order
				i, j, k := strings.Index(p, "- ONE"), strings.Index(p, "- TWO"), strings.Index(p, "- THREE")
				if !(i < j && j < k) {
					t.Errorf("subjects out of order: ONE@%d TWO@%d THREE@%d", i, j, k)
				}
			},
		},
		{
			name: "rejection: single subject yields exactly one list line", diff: diff, rejected: []string{"solo"},
			check: func(t *testing.T, p string) {
				if got := strings.Count(p, "\n- "); got != 1 {
					t.Errorf("single subject ⇒ 1 list line; got %d", got)
				}
			},
		},
		{
			name: "diff is the exact tail — normal", diff: "TAIL_NORMAL\nnope", rejected: nil,
			check: func(t *testing.T, p string) {
				if !strings.HasSuffix(p, "TAIL_NORMAL\nnope") {
					t.Errorf("diff must be the verbatim tail; got suffix %q", suffix(p, 40))
				}
			},
		},
		{
			name: "diff is the exact tail — rejection", diff: "TAIL_REJECT", rejected: []string{"a"},
			check: func(t *testing.T, p string) {
				if !strings.HasSuffix(p, "TAIL_REJECT") {
					t.Errorf("diff must be the verbatim tail; got suffix %q", suffix(p, 40))
				}
			},
		},
		{
			name: "diff with trailing newline preserved verbatim", diff: "diff\n", rejected: nil,
			check: func(t *testing.T, p string) {
				if !strings.HasSuffix(p, "diff\n") {
					t.Error("trailing newline of diff must be preserved (no normalization)")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, BuildUserPayload(tc.diff, tc.rejected))
		})
	}
}

// TestBuildUserPayload_EdgeCases covers the defensive paths: empty diff (no panic), the nil==empty
// equivalence, and the blank-line topology count.
func TestBuildUserPayload_EdgeCases(t *testing.T) {
	t.Run("empty diff does not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("BuildUserPayload(\"\", nil) panicked: %v", r)
			}
		}()
		got := BuildUserPayload("", nil)
		const want = "Generate a commit message for these changes:\n\n"
		if got != want {
			t.Errorf("empty-diff normal payload = %q, want %q", got, want)
		}
	})

	t.Run("nil and empty rejected produce identical output", func(t *testing.T) {
		const diff = "D"
		if BuildUserPayload(diff, nil) != BuildUserPayload(diff, []string{}) {
			t.Error("nil and []string{} rejected must produce identical normal payloads")
		}
	})

	t.Run("rejection: exactly two blank lines separate epilogue from diff and list from epilogue", func(t *testing.T) {
		p := BuildUserPayload("DIFF", []string{"a"})
		// list item '- a\n' then '\n' (blank) then epilogue then '\n\n' (blank) then diff
		if !strings.Contains(p, "- a\n\nCreate an entirely new message with different wording.\n\nDIFF") {
			t.Errorf("rejection blank-line topology wrong around list/epilogue/diff; got %q", near(p, "Create an"))
		}
	})
}

// NOTE: `near` and `suffix` are already defined in system_test.go (same package). Do NOT redeclare them
// (compile error). Reuse them directly as shown above.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/prompt/payload.go — 4 constants + BuildUserPayload
  - FILE: NEW internal/prompt/payload.go. PACKAGE: `package prompt`. IMPORT: EXACTLY "strings" (NOT "fmt",
      NO internal/*, NO third-party). Do NOT edit system.go / system_test.go.
  - DEFINE four unexported canonical string constants VERBATIM from PRD §17.3, each with NO trailing newline:
      userInstruction ("…changes:" COLON), userInstructionReject ("…changes." PERIOD), rejectionPreamble
      (two-line IMPORTANT header as a raw string literal), rejectionEpilogue ("Create an entirely new
      message with different wording.").
  - IMPLEMENT exported `func BuildUserPayload(diff string, rejected []string) string`:
      - if len(rejected) == 0 → return userInstruction + "\n\n" + diff  (NORMAL, fast path).
      - else → strings.Builder: userInstructionReject + "\n\n" + rejectionPreamble + "\n" + per-subject
        ("- " + s + "\n") + "\n" + rejectionEpilogue + "\n\n" + diff.
  - GOTCHA: signature is EXACTLY the work-item form (exported, string-only, rejected is a param).
  - GOTCHA: COLON (normal) vs PERIOD (rejection) — two constants, per §17.3 (design-decisions §2).
  - GOTCHA: append the diff VERBATIM (no TrimSpace/normalization).
  - GOTCHA: NO "fmt" import (no %d here); "strings" only.

Task 2: CREATE internal/prompt/payload_test.go — 4 test functions
  - FILE: NEW internal/prompt/payload_test.go. PACKAGE: `package prompt` (white-box). IMPORT: "strings",
      "testing".
  - IMPLEMENT TestBuildUserPayload_NormalCanonicalExact (nil + []string{} → exact §17.3 normal string).
  - IMPLEMENT TestBuildUserPayload_RejectionCanonicalExact (2 subjects → exact §17.3 rejection string).
  - IMPLEMENT TestBuildUserPayload_Properties (table: colon/period, rejection block presence/absence,
      per-subject "- " lines + order + count, diff-always-the-tail in both paths, trailing-newline preserved).
  - IMPLEMENT TestBuildUserPayload_EdgeCases (empty diff no-panic, nil==empty equivalence, blank-line
      topology around list/epilogue/diff).
  - REUSE the `near`/`suffix` helpers from system_test.go (same package) — do NOT redeclare them.
  - GOTCHA: no subprocess, no temp repo, no git — pure-function tests.

Task 3: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. Every other file
      (system.go, system_test.go, provider/*, config/*, git/*, cmd/*, pkg/*, Makefile) MUST be
      byte-unchanged. `go test -race ./internal/prompt/` MUST be green (S1's + S2's + S3's, together).
```

### Implementation Patterns & Key Details

```go
// THE branch — len(rejected)==0 selects the normal (colon) fast-path; non-empty selects the rejection path.
func BuildUserPayload(diff string, rejected []string) string {
	if len(rejected) == 0 {
		return userInstruction + "\n\n" + diff          // §17.3 NORMAL (colon)
	}
	var b strings.Builder
	b.WriteString(userInstructionReject)               // §17.3 rejection (PERIOD)
	b.WriteString("\n\n")
	b.WriteString(rejectionPreamble)                   // "IMPORTANT: …\n…DIFFERENT:"
	b.WriteByte('\n')
	for _, s := range rejected {                        // one "- " line per single-line subject (FR30)
		b.WriteString("- ")
		b.WriteString(s)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')                                  // blank line after the list
	b.WriteString(rejectionEpilogue)                   // "Create an entirely new message…"
	b.WriteString("\n\n")                              // blank line before the diff
	b.WriteString(diff)                                // VERBATIM (no normalization)
	return b.String()
}

// THE constants — verbatim §17.3, NO trailing newlines (builder owns every '\n').
const userInstruction       = "Generate a commit message for these changes:"   // normal  (COLON)
const userInstructionReject = "Generate a commit message for these changes."   // rejection (PERIOD)
const rejectionPreamble = `IMPORTANT: The following messages were REJECTED because they already exist
in git history. You MUST generate something COMPLETELY DIFFERENT:`
const rejectionEpilogue = "Create an entirely new message with different wording."
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. payload.go uses stdlib strings ONLY (no fmt, no third-party). `go mod tidy` MUST be a
        no-op. `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - payload.go → (stdlib: strings) ONLY. It does NOT import fmt, internal/config, internal/git,
        internal/provider, os/exec, or any third-party. internal/prompt stays a LEAF.
  - payload_test.go → (stdlib: strings, testing) ONLY. In-package (package prompt).

UPSTREAM CONTRACT (the inputs — do NOT implement, just consume):
  - P1.M1.T3.S1 git.StagedDiff(ctx, opts StagedDiffOptions) (string, error): returns the concatenated
        markdown + non-markdown staged diff (possibly line/byte-capped with a `... [diff truncated]`
        sentinel, possibly NOT ending in '\n'). This IS the `diff` parameter — appended VERBATIM.
  - P1.M3.T2 dedupe loop: supplies `rejected []string` — []string{} on attempt 1; non-empty (the matched
        duplicate subjects) on retries up to max_duplicate_retries (default 3, FR32). Each subject is the
        first line of a generated message (FR30) — guaranteed single-line.

DOWNSTREAM CONTRACTS (the output — do NOT implement here, just honor the string's role):
  - P1.M3.T4 (orchestrator CommitStaged): `payload := prompt.BuildUserPayload(diff, rejected)`;
        `spec, _ := manifest.Render(model, provider, sys, payload)` (P1.M2.T4 — payload is the 4th arg).
  - P1.M2.T4 (render.go Render): `case "stdin": spec.Stdin = payload` (line ~100). When
        system_prompt_flag == "", Render PREPENDS the system prompt (`sys + "\n\n" + payload`) — that
        prepend is Render's concern, NOT BuildUserPayload's (§17.3 owns only the user payload).
  - P1.M2.T5 (Execute): pipes spec.Stdin to the agent subprocess; captures stdout.
  - P1.M2.T6 (ParseOutput): turns stdout into the message — independent of the payload.
  => The `BuildUserPayload(diff string, rejected []string) string` signature is FROZEN after S3.

SIBLING SUBTASKS (same package — already exist; do NOT edit):
  - P1.M3.T1.S1 (DONE): the mature-repo (§17.1) system prompt — BuildSystemPrompt + DetectMultiline. UNCHANGED.
  - P1.M3.T1.S2 (IMPLEMENTING in parallel): the new-repo (§17.2) system prompt — BuildFallbackPrompt. APPENDS
        to system.go; S3's separate payload.go file avoids any merge collision with it.

FROZEN FILES (do NOT edit):
  - internal/prompt/system.go, internal/prompt/system_test.go (S1/S2 own those), internal/provider/*,
        internal/config/*, internal/git/*, cmd/stagehand/main.go, pkg/*, Makefile, go.mod, go.sum.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the two new files
gofmt -w internal/prompt/payload.go internal/prompt/payload_test.go

# Vet the prompt package (compiles system.go + system_test.go + payload.go + payload_test.go, i.e. S1+S2+S3)
go vet ./internal/prompt/

# Confirm payload.go imports EXACTLY "strings" (no fmt, no internal/*, no third-party)
sed -n '/^import/,/)/p' internal/prompt/payload.go   # → a single "strings" import

# Confirm go.mod/go.sum are unchanged (stdlib only)
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged ✓"

# Expected: Zero errors. `go vet` clean. §17.3 is all ASCII — no em-dash risk (unlike S1).
```

### Level 2: Unit Tests (THE KEYSTONE — table-driven suite)

```bash
# Run the NEW suite verbosely (every subtest listed)
go test -race -v ./internal/prompt/ -run TestBuildUserPayload

# Confirm S1's + S2's suites are STILL green (untouched)
go test -race -v ./internal/prompt/ -run 'TestBuildSystemPrompt|TestDetectMultiline|TestBuildFallbackPrompt'

# Full prompt package (S1 + S2 + S3 together)
go test -race ./internal/prompt/

# Coverage of the new file specifically
go test -coverprofile=coverage.out ./internal/prompt/ && go tool cover -func=coverage.out | grep -E 'payload.go|BuildUserPayload'

# Expected: All pass. The two canonical exact-match tests are the load-bearing ones — if either fails, diff
# `got` vs `want` (%q output) and fix the §17.3 text byte-for-byte. The Properties table's colon/period and
# diff-always-the-tail checks catch the two most likely implementation errors.
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-module build (S3 is internal; confirms nothing else broke, incl. S2's parallel append)
go build ./...

# Optional: confirm the assembled payloads read correctly (sanity print, not a test)
go test -run 'TestBuildUserPayload_(Normal|Rejection)CanonicalExact' -v ./internal/prompt/

# Expected: `go build ./...` succeeds; both canonical tests print PASS. S3 has no runtime/endpoint/DB
# surface — it is a pure function — so there is no service to start, no curl, no DB to check.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Not applicable for S3 (pure string function, no I/O, no network, no DB, no UI).
# The strongest creative check is human review: open each canonical test's `want` string side-by-side with
# PRD §17.3 (plan/001_f1f80943ac34/prd_snapshot.md lines 1117–1142) and confirm byte-for-byte equality for
# BOTH the normal (colon) and rejection (period) renderings. Then confirm the orchestrator wiring contract
# (Integration Points) matches what P1.M3.T4 will call:
#   payload := prompt.BuildUserPayload(diff, rejected)
# and that render.go consumes it as the 4th arg to Render → spec.Stdin.
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed successfully.
- [ ] All tests pass: `go test -race ./internal/prompt/` (S1's, S2's, AND S3's, together).
- [ ] `go build ./...` succeeds.
- [ ] No vet errors: `go vet ./internal/prompt/`.
- [ ] No formatting issues: `gofmt -l internal/prompt/` (empty output).
- [ ] go.mod/go.sum byte-unchanged: `git diff --exit-code go.mod go.sum`.

### Feature Validation

- [ ] `BuildUserPayload("DIFF", nil)` == `"Generate a commit message for these changes:\n\nDIFF"` (normal,
      COLON — canonical test).
- [ ] `BuildUserPayload("DIFF", []string{"a","b"})` == the full §17.3 rejection rendering (PERIOD
      instruction + IMPORTANT preamble + `- a`/`- b` list + epilogue + blank + diff — canonical test).
- [ ] The instruction's terminal punctuation is COLON (normal) / PERIOD (rejection) — properties check.
- [ ] Each rejected subject is on its own `- `-prefixed line, in input order; count == len(rejected).
- [ ] The diff is ALWAYS the exact tail (`strings.HasSuffix`) in BOTH paths; trailing newline preserved.
- [ ] The orchestrator's planned call `prompt.BuildUserPayload(diff, rejected)` matches the frozen signature.
- [ ] Empty diff does not panic (defensive).

### Code Quality Validation

- [ ] Follows S1/S2 patterns (constant-with-no-trailing-newline, builder-owned `\n`, EXPORTED string-only
      function, parameter-decoupled-from-config/git/provider, stdlib-only leaf package).
- [ ] NEW file `payload.go` + `payload_test.go` (no edit to system.go/system_test.go — zero S2 merge risk).
- [ ] Anti-patterns avoided (check against Anti-Patterns section): no colon-in-both, no diff normalization,
      no `<...>` literal emission, no fmt import, no sanitization of subjects.
- [ ] `internal/prompt` remains a stdlib-only leaf (imports `"strings"` only in payload.go).

### Documentation & Deployment

- [ ] Doc-comments cite PRD §9.3 FR15 / §9.7 FR32 / §17.3 and research design-decisions.md (§2/§3/§6).
- [ ] The colon-vs-period decision (§2) is explained in the `userInstructionReject` comment so a future
      reviewer understands why the period is intentional (not a typo).
- [ ] No new env vars, no new config fields (the function consumes runtime `diff` + `rejected` args only).

---

## Anti-Patterns to Avoid

- ❌ Don't use the COLON in the rejection path — §17.3 renders the rejection instruction with a PERIOD.
  The work-item paraphrase elides this; the PRD §17.3 (prd_snapshot.md line 1130) is authoritative. Two
  instruction constants; the canonical test pins the period. (design-decisions §2)
- ❌ Don't normalize the diff — no `TrimSpace`, no re-cap, no trailing-newline fixup. The diff is appended
  VERBATIM; its shape is `git.StagedDiff`'s contract. (design-decisions §6)
- ❌ Don't emit the `<diff payload>` / `<rejected subject N>` tokens literally — they are §17.3 annotation
  placeholders (like §17.1's excluded `(up to 20…)` line), replaced by the runtime `diff` arg and `rejected`
  slice elements. (design-decisions §5)
- ❌ Don't append to `system.go` — S2 is appending to it in parallel. Create a NEW file `payload.go`.
  (design-decisions §0)
- ❌ Don't import `"fmt"` — there is no `%d` interpolation here (unlike S1/S2). `"strings"` only.
- ❌ Don't sanitize/TrimSpace the rejected subjects — they are single-line by FR30 contract; trust the
  upstream. (design-decisions §8)
- ❌ Don't add a trailing newline to any constant — S1/S2's rule (constant has no trailing newline; builder
  owns `\n`) applies. (design-decisions §4)
- ❌ Don't redeclare `near`/`suffix` in `payload_test.go` (compile error) — reuse them from system_test.go.
- ❌ Don't implement the dedupe loop, the retry counter, the subject extraction, or the Render/Execute call —
  those are P1.M3.T2 / P1.M3.T4 / P1.M2.T4 / P1.M2.T5 respectively. S3 owns ONLY BuildUserPayload + its
  constants.
- ❌ Don't edit `system.go`, `system_test.go`, or any provider/config/git/cmd file — only `payload.go` +
  `payload_test.go` are new.
