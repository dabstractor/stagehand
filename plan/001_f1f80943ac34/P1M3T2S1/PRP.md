---
name: "P1.M3.T2.S1 — Subject extraction + exact-match check (dedupe primitives): the two pure functions the duplicate-rejection retry loop is built from — PRD §9.7 FR30/FR32"
description: |

  Land the FIRST (and only) subtask of Duplicate Rejection Loop (P1.M3.T2): CREATE `internal/generate/dedupe.go`
  (`package generate`) exporting TWO pure functions — `IsDuplicate(subject string, recent []string) bool`
  (PRD §9.7 FR32: exact match of the generated subject against the last 50 commit subjects, via a Go
  `map[string]struct{}` set for O(1) lookup — the faithful port of commit-pi's `grep -Fxq`) and
  `ExtractSubject(message string) string` (PRD §9.7 FR30: the generated subject = the first line of the
  parsed commit message). PLUS `internal/generate/dedupe_test.go` — table-driven pure-function tests in
  the style of `internal/provider/parse_test.go`.

  SCOPE BOUNDARY (load-bearing): this subtask provides the two PRIMITIVES ONLY. The retry LOOP, the
  `max_duplicate_retries` counter (default 3, FR32), the rescue-path entry (FR33), the rejection-list
  `append`, and the `prompt.BuildUserPayload(diff, rejected)` call are the ORCHESTRATOR (P1.M3.T4) —
  explicitly NOT this subtask. The work-item is unambiguous: "This subtask provides the check + extract
  functions; the orchestrator wires the loop." Implementing the loop here would duplicate T4, couple a
  pure-logic file to git/config/prompt, and violate scope. See research design-decisions.md §0.

  INPUT (upstream — already built, read-only): `recent []string` = `git.RecentSubjects(ctx, 50)` from
  P1.M1.T3.S4 (single-line, TrimSpace'd, newest-first subjects; nil on unborn/non-repo). `message string`
  = the `msg` return of `provider.ParseOutput(raw, manifest)` from P1.M2.T6.S1 (Step 5 TrimSpaces it;
  Step 4 normalizes `\r\n`→`\n`). Config `MaxDuplicateRetries` (default 3) belongs to the ORCHESTRATOR's
  loop, not dedupe.

  OUTPUT (downstream consumer): the orchestrator P1.M3.T4 `CommitStaged` calls, per generation attempt:
  `subject := generate.ExtractSubject(msg)`; if `generate.IsDuplicate(subject, recent)` it `append`s
  `subject` to `rejected` and calls `prompt.BuildUserPayload(diff, rejected)` (P1.M3.T1.S3, implementing
  in parallel) for the retry. The two signatures are FROZEN after this subtask.

  ⚠️ **THE signatures — implement the WORK-ITEM forms EXACTLY.** `func IsDuplicate(subject string, recent
  []string) bool` and `func ExtractSubject(message string) string`. Both EXPORTED (capitalized — per the
  work-item contract; the public API P1.M3.T5 may re-export them). Both return a single non-error value
  (bool / string) — there is NO failure mode (pure transformations). `recent` is a plain `[]string`
  (NOT a pre-built set) — IsDuplicate builds the set INTERNALLY. See design-decisions.md §1.

  ⚠️ **THE match semantics — EXACT, case-SENSITIVE, whole-subject (FR32).** commit-pi's `grep -Fxq` is an
  exact whole-LINE match, case-sensitive (no `-i`). So `IsDuplicate` must NOT lowercase, must NOT do
  substring/prefix matching: `"Fix: Foo"` != `"fix: foo"` (case differs → NOT a dup); `"fix: foobar"` !=
  `"fix: foo"` (prefix → NOT a dup). A test pins EACH of these. See design-decisions.md §3.

  ⚠️ **ExtractSubject = "first line (split on \n, trim)" — be LITERAL.** Take the prefix up to the first
  `\n`, then `strings.TrimSpace` it. Do NOT trim the WHOLE message first (ParseOutput already did — Step 5;
  trimming-first would deviate from the work-item's literal "split on \n, trim" and change the leading-blank
  edge case). Use `strings.IndexByte` (O(pos), zero-alloc) — semantically identical to
  `strings.Split(message, "\n")[0]` but without allocating the full line slice (mirrors git.go's style).
  See design-decisions.md §2.

  ⚠️ **NO trimming inside IsDuplicate.** Both inputs are pre-trimmed by their producers (`ExtractSubject`
  trims the subject; `git.RecentSubjects` trims each element and skips empties). A plain `set[subject]`
  lookup is correct; defensive trimming would mask an upstream bug. Trust the contract (same philosophy
  as `prompt.BuildUserPayload` trusting FR30 single-line subjects). See design-decisions.md §3.

  ⚠️ **NO new dependency — stdlib `strings` ONLY.** dedupe.go imports `"strings"` (IndexByte + TrimSpace
  for ExtractSubject); IsDuplicate needs NO import (builtin map/range). dedupe_test.go imports `"strings"`
  + `"testing"`. NO `"fmt"`, NO `internal/*`, NO third-party. `go mod tidy` MUST be a no-op;
  `git diff --exit-code go.mod go.sum` MUST be empty. internal/generate stays a stdlib-only LEAF.
  See design-decisions.md §4.

  ⚠️ **NEW package — internal/generate is currently EMPTY.** dedupe.go + dedupe_test.go are the FIRST files
  in `package generate`. No merge-collision risk (unlike the prompt-layer S1/S2/S3 coordination). The
  orchestrator T4 lands in this same package later. Touches ONLY these two NEW files.

  Deliverable: CREATE `internal/generate/dedupe.go` — exported `IsDuplicate(subject string, recent
  []string) bool` (map-set build + O(1) lookup) + exported `ExtractSubject(message string) string`
  (IndexByte first-line + TrimSpace). PLUS `internal/generate/dedupe_test.go` — `TestExtractSubject` +
  `TestIsDuplicate` (table-driven, PRD-cited cases). Touches ONLY these two NEW files — NO go.mod/go.sum
  change, NO edit to internal/prompt/* (S1/S2/S3 own it; S3 implementing in parallel), internal/git/*,
  internal/provider/*, internal/config/*, cmd/*, pkg/*, or the Makefile.

---

## Goal

**Feature Goal**: Implement the duplicate-rejection primitives (PRD §9.7 FR30 / FR32) that the generation
orchestrator's retry loop (P1.M3.T4) is built from: one function that extracts the generated commit
subject (= the first line of the parsed message, FR30), and one function that checks whether that subject
exactly matches any of the last 50 commit subjects (FR32) via an exact, case-sensitive, whole-subject set
lookup (the Go `map` port of commit-pi's `grep -Fxq`). This is the "dedupe half" of P1.M3.T2; the loop
itself (counter, rescue, rejection-list append) is the orchestrator.

**Deliverable**:
1. **CREATE** `internal/generate/dedupe.go` (`package generate`, imports `"strings"` ONLY) —
   - exported `func IsDuplicate(subject string, recent []string) bool` — builds `map[string]struct{}` from
     `recent`, returns whether `subject` is a key (exact, case-sensitive, whole-subject match),
   - exported `func ExtractSubject(message string) string` — returns `TrimSpace` of the prefix up to the
     first `\n` (the first line; FR30).
2. **CREATE** `internal/generate/dedupe_test.go` (`package generate`, imports `"strings"` + `"testing"`) —
   `TestExtractSubject` + `TestIsDuplicate` (table-driven pure-function tests, PRD §9.7-cited cases).

No other files touched. **No new file besides the two above. No go.mod/go.sum change** (stdlib `strings`
only). NO edit to `internal/prompt/*`, `internal/git/*`, `internal/provider/*`, `internal/config/*`,
`cmd/*`, `pkg/*`, the `Makefile`.

**Success Definition**: `go build ./...` succeeds; `go test -race ./internal/generate/` is green with the
new suite passing; `gofmt -l internal/generate/` clean; `go vet ./internal/generate/` clean; `golangci-lint
run` (if available) clean; go.mod/go.sum byte-unchanged; `ExtractSubject` returns the trimmed first line
for every test case (multi-line body excluded, trailing spaces trimmed, empty→""); `IsDuplicate` is
case-sensitive, whole-subject, and returns false for nil/empty `recent` and empty `subject`.

## User Persona

**Target User**: The generate orchestrator (P1.M3.T4 `CommitStaged`) — on EVERY generation attempt it calls
`generate.ExtractSubject(msg)` (FR30) then `generate.IsDuplicate(subject, recent)` (FR32). Transitively
`git.RecentSubjects` (P1.M1.T3.S4 — supplies `recent`), `provider.ParseOutput` (P1.M2.T6.S1 — supplies
`msg`), and `prompt.BuildUserPayload` (P1.M3.T1.S3 — receives the appended `rejected` list on a retry).
End-user persona is "the plan-holder" / "the API-key refusenik" / "the multi-agent tinkerer" (PRD §7)
whose generated commit subject must not silently duplicate an existing commit (PRD §9.7 / Appendix B.4:
"Attempt 1: subject 'fix: handle null user' matches an existing commit — retrying. Attempt 2: …accepted.").

**Use Case**: After the orchestrator parses the agent's stdout into a commit message, it extracts the
subject (the first line — FR30), then checks it against the last 50 commit subjects (FR31, fetched via
`git log --format=%s -50`). If the subject EXACTLY matches one (FR32), the orchestrator retries generation
with a rejection list naming the matched subject(s) (via `prompt.BuildUserPayload`'s §17.3 rejection
block), up to `max_duplicate_retries` (default 3); on exhaustion it enters the rescue path (FR33). This
subtask provides the extract + check primitives that gate each iteration of that loop.

**User Journey**: (internal API, no new end-user surface)
`ParseOutput` → `ExtractSubject(msg)` → `IsDuplicate(subject, recent)` → (if true) `append(rejected,
subject)` → `BuildUserPayload(diff, rejected)` → retry → accept → `CommitTree` + `UpdateRefCAS`. The
`ExtractSubject` + `IsDuplicate` calls are the ONLY dedupe logic executed per attempt.

**Pain Points Addressed**: (1) The model regenerating an existing commit subject verbatim (a real failure
mode with style-learning prompts) — solved by exact-match detection (FR32). (2) Inconsistent / fuzzy
duplicate detection (e.g. case-insensitive or substring) producing false rejections — solved by exact,
case-sensitive, whole-subject matching (faithful to `grep -Fxq`). (3) A subject accidentally carrying the
message body — solved by extracting only the first line (FR30).

## Why

- **Unblocks the orchestrator's retry loop (P1.M3.T4).** Until these two primitives exist, the orchestrator
  has no way to extract a subject from a parsed message or check it against history. P1.M3.T4 depends on
  this subtask; P1.M3.T3 (rescue) and P1.M3.T5 (public API) depend transitively.
- **Satisfies PRD §9.7 FR30 + FR32.** FR30 = extract the generated subject (first line); FR32 = exact-match
  check against the last 50 subjects, retry on match. This subtask IS both FRs' core logic (the loop wiring
  + counter + rescue is T4/T3).
- **Faithful port of commit-pi's proven semantics.** commit-pi uses `grep -Fxq` (exact line match) and
  `head -1`-style subject extraction (PRD Appendix C porting map). The Go port uses a `map` set for O(1)
  lookup and `IndexByte` for the first line — identical RESULT, idiomatic Go.
- **No new user-facing surface** (PRD "DOCS: none — internal"). No new dependency (stdlib `strings` only).

## What

A new file `internal/generate/dedupe.go` with two exported pure functions, plus a new test file
`dedupe_test.go`. No new types, no I/O, no config, no git, no subprocess. Both functions are deterministic
in-memory transformations: `ExtractSubject(message string) string` returns the trimmed first line;
`IsDuplicate(subject string, recent []string) bool` builds a set from `recent` and reports exact membership.

### Success Criteria

- [ ] `internal/generate/dedupe.go` exists, `package generate`, imports EXACTLY `"strings"` (NO `"fmt"`,
      NO `internal/*`, NO third-party). Defines exported `IsDuplicate(subject string, recent []string) bool`
      and exported `ExtractSubject(message string) string`.
- [ ] `ExtractSubject("fix: foo\n\nbody")` == `"fix: foo"` (FR30: first line; body EXCLUDED).
- [ ] `ExtractSubject("fix: foo")` == `"fix: foo"` (single line).
- [ ] `ExtractSubject("")` == `""` (empty message, no panic).
- [ ] `ExtractSubject("fix: foo  \nbody")` == `"fix: foo"` (trailing spaces on the subject line trimmed).
- [ ] `IsDuplicate("fix: foo", []string{"a", "fix: foo", "b"})` == `true` (match present, in the middle).
- [ ] `IsDuplicate("fix: foo", []string{"a", "b"})` == `false` (no match).
- [ ] `IsDuplicate("fix: foo", nil)` == `false` AND `IsDuplicate("fix: foo", []string{})` == `false`
      (empty recent).
- [ ] `IsDuplicate("Fix: Foo", []string{"fix: foo"})` == `false` (case-SENSITIVE — `grep -Fxq` has no -i).
- [ ] `IsDuplicate("fix: foobar", []string{"fix: foo"})` == `false` (EXACT whole-subject — prefix is NOT a
      match; `grep -x`).
- [ ] `IsDuplicate("", []string{"", "x"})` == `true` if "" is literally in recent (defensive correctness:
      set membership is literal; but note `git.RecentSubjects` never stores "" so this won't occur in
      practice — the test pins the pure-function behavior, not the pipeline contract).
- [ ] `go build ./...` succeeds; `go test -race ./internal/generate/` green; `gofmt -l internal/generate/`
      clean; `go vet ./internal/generate/` clean.
- [ ] go.mod/go.sum byte-unchanged (`git diff --exit-code go.mod go.sum` empty).
- [ ] Every other file byte-unchanged (`internal/prompt/*`, `internal/git/*`, `internal/provider/*`,
      `internal/config/*`, `cmd/*`, `pkg/*`, `Makefile`).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: PRD §9.7 FR30/FR32
(already in context as selected_prd_content), the design decisions (research design-decisions.md — the
single most important read, esp. §2 ExtractSubject's IndexByte-vs-Split + the trim-only-first-line call,
§3 IsDuplicate's case-sensitivity + no-trimming + map-set, §0 the scope boundary), the upstream contracts
(`git.RecentSubjects` returns trimmed single-line subjects; `provider.ParseOutput` returns a trimmed +
newline-normalized message), the in-package table-driven pure-function test convention
(`internal/provider/parse_test.go`), the sibling pure-function pattern (`prompt.BuildUserPayload`,
`prompt.DetectMultiline` — exported, single-value, no-error, stdlib-only), and the copy-ready Go code in
the Implementation Blueprint. No git-plumbing/provider-render/CLI knowledge required — dedupe is two pure
functions + their tests.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M3T2S1/research/design-decisions.md
  why: the SINGLE most important read — the 7 decisions specific to this subtask: the scope boundary
       (TWO primitives, NOT a loop — §0), the frozen signatures (§1), ExtractSubject's IndexByte-vs-Split +
       trim-only-first-line call (§2), IsDuplicate's case-sensitivity + no-trimming + map-set (§3), package
       + imports (§4), test strategy (§5), frozen files (§6), upstream/downstream contracts (§7).
  critical: §0 (do NOT implement the loop/counter/rescue — that's T4), §2 (be LITERAL: trim the FIRST LINE
       only, not the whole message; use IndexByte), §3 (case-SENSITIVE exact whole-subject match; NO
       trimming inside IsDuplicate) are the things most likely to be implemented wrong.

- file: internal/provider/parse.go   (P1.M2.T6.S1 — READ for the ParseOutput contract; do NOT edit)
  section: `func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)` — Step 4
           (`normalizeNewlines`: `\r\n`→`\n`, collapse 3+`\n`→2) + Step 5 (`msg = strings.TrimSpace(msg)`).
  why: the INPUT contract for ExtractSubject — confirms the `message` arg arrives ALREADY trimmed at the
       boundaries and newline-normalized. This is WHY ExtractSubject only needs the per-line TrimSpace
       (trailing spaces on the subject line), NOT a whole-message trim (design-decisions §2).
  critical: ExtractSubject must assume msg is pre-trimmed; do NOT re-trim the whole message (would deviate
            from the work-item's literal "split on \n, trim" and alter the leading-blank edge case).

- file: internal/git/git.go   (P1.M1.T3.S4 — READ for the RecentSubjects contract; do NOT edit)
  section: `func (g *gitRunner) RecentSubjects(ctx context.Context, n int) ([]string, error)` — runs
           `git log --format=%s -<n>`; each subject is single-line by git's %s definition (NO embedded
           newline possible); each is `strings.TrimSpace`'d and empties are skipped; nil on unborn/non-repo.
  why: the INPUT contract for IsDuplicate — confirms every element of `recent` is single-line, trimmed,
       and non-empty. This is WHY IsDuplicate does a plain `set[subject]` lookup with NO defensive
       trimming (design-decisions §3).
  critical: subjects are single-line + trimmed by the producer; trust it (do not re-trim, which would
            mask an upstream bug). n defaults to 50 (FR31); the caller (orchestrator) passes it.

- file: internal/provider/parse_test.go   (P1.M2.T6.S1 — READ for the test PATTERN to mirror; do NOT edit)
  section: `TestParseOutput` — a table-driven pure-function suite: `cases := []struct{...}` with a
           `name`/inputs/`want` per row, a `for _, tc := range cases { t.Run(tc.name, ...) }` loop, and an
           inline PRD-citation comment per case ("// --- raw mode (FR26) ---"). NO subprocess, NO temp repo.
  why: the TEST CONVENTION to follow verbatim in dedupe_test.go. dedupe is pure-function (no git/I/O), so
       it uses THIS style — NOT the temp-repo style of internal/git/*_test.go (which needs a real git repo).
  pattern: copy the table struct + t.Run loop + inline `// FR30` / `// FR32` citation comments. Define
           table-local helpers only if needed (none needed here — inputs are plain strings/slices).

- file: internal/prompt/system.go   (P1.M3.T1.S1/S2 — READ for the sibling pure-function PATTERN; do NOT edit)
  section: `DetectMultiline(examples []string) bool` (exported, bool-only, no-error, stdlib-only) and
           `BuildUserPayload` (exported, string-only, no-error) — both pure transformations, both exported
           per the work-item contract, both decoupled from config/git/provider.
  why: the architectural PRECEDENT — Stagecoach's generation pipeline is built from small exported pure
       functions that the orchestrator composes. dedupe.go's IsDuplicate/ExtractSubject are exactly this
       shape (exported, single-value, no-error, stdlib-only). Mirror the doc-comment density (cite PRD §/FR).
  gotcha: do NOT reuse prompt's constants/helpers — dedupe has NO string constants (just two functions).

- url: (PRD §9.7 FR30–FR33 + Appendix B.4 — already in context as selected_prd_content `h3.23`/`h3.83`;
       ALSO in plan/001_f1f80943ac34/prd_snapshot.md lines 280–283 and 1374–1379)
  why: FR30 ("subject = first line of the message") defines ExtractSubject; FR32 ("exactly matches one of
       the 50, retry") defines IsDuplicate's exact-match semantics + the retry (which is T4); FR33 (rescue
       on exhaustion — T3/T4); B.4 shows the end-to-end retry in action ("Attempt 1 … matches … retrying.
       Attempt 2: …accepted.").
  critical: FR32's "exactly matches" = case-sensitive whole-subject equality (commit-pi `grep -Fxq`).
            Do NOT interpret "exactly" loosely (no case-folding, no substring). A test pins each.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; go-toml/v2 + pflag  (UNCHANGED — dedupe adds NO dep: stdlib strings only)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 — untouched (read-only ref: MaxDuplicateRetries default 3 — ORCHESTRATOR's, not dedupe's)
  generate/                     # P1.M3 — CURRENTLY EMPTY; this subtask CREATES dedupe.go + dedupe_test.go (the package's FIRST files)
    dedupe.go                   # NEW (this subtask) ← IsDuplicate + ExtractSubject
    dedupe_test.go              # NEW (this subtask) ← TestIsDuplicate + TestExtractSubject (table-driven)
  git/                          # P1.M1.T2/T3 — untouched (RecentSubjects read-only ref)
  prompt/                       # P1.M3.T1 — S1/S2 DONE; S3 (payload.go) IMPLEMENTING IN PARALLEL — UNTOUCHED by this subtask
    system.go                   # EXISTS (S1+S2) — UNCHANGED
    payload.go                  # EXISTS (S3, parallel) — UNCHANGED by this subtask
  provider/                     # P1.M2 (T1–T6) — untouched (parse.go ParseOutput read-only ref)
  ui/                           # P1.M4 (empty stub) — untouched
cmd/stagecoach/main.go           # stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  generate/
    dedupe.go        # NEW — exported IsDuplicate(subject string, recent []string) bool + ExtractSubject(message string) string
    dedupe_test.go   # NEW — TestIsDuplicate + TestExtractSubject (table-driven pure-function tests)
# All other files UNCHANGED. go.mod/go.sum UNCHANGED. After this subtask: the dedupe primitives exist for
# the orchestrator (P1.M3.T4) to wire its retry loop; P1.M3.T4 calls ExtractSubject(msg) + IsDuplicate(subject,
# recent), and on a match appends to `rejected` and calls prompt.BuildUserPayload(diff, rejected) (P1.M3.T1.S3).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (SCOPE — do NOT implement the loop): this subtask exports IsDuplicate + ExtractSubject ONLY.
// The retry loop, the max_duplicate_retries counter (default 3), the rescue path (FR33), the rejected-list
// append, and the BuildUserPayload call are the ORCHESTRATOR (P1.M3.T4). Implementing them here duplicates
// T4 and couples a pure-logic file to git/config/prompt. (design-decisions.md §0)

// CRITICAL (signatures — EXPORTED, single-value, no-error): implement EXACTLY
//   func IsDuplicate(subject string, recent []string) bool
//   func ExtractSubject(message string) string
// Both exported (capitalized — per work-item; the public API P1.M3.T5 may re-export). Neither returns an
// error (no failure mode). `recent` is a plain []string — IsDuplicate builds the set INTERNALLY. (§1)

// CRITICAL (ExtractSubject — be LITERAL: first line, trim): take the prefix up to the first '\n', then
// strings.TrimSpace. Use strings.IndexByte (O(pos), zero-alloc) — identical to strings.Split(message,
// "\n")[0] but without allocating the full line slice (mirrors git.go's IndexByte style). Do NOT trim the
// WHOLE message first (ParseOutput already did — Step 5; trimming-first deviates from the work-item's
// literal "split on \n, trim"). Empty message → "". (design-decisions.md §2)

// CRITICAL (IsDuplicate — EXACT, case-SENSITIVE, whole-subject): commit-pi's `grep -Fxq` is exact
// whole-LINE, case-sensitive (no -i). Build map[string]struct{} from recent; return `_, ok := set[subject]`.
// Do NOT lowercase (FR32 "exactly matches" = byte equality). Do NOT substring/prefix match ("fix: foobar"
// is NOT a dup of "fix: foo"). A test pins EACH. (design-decisions.md §3)

// CRITICAL (NO trimming inside IsDuplicate): both inputs are pre-trimmed by producers — ExtractSubject
// trims the subject; git.RecentSubjects trims each element + skips empties. A plain set[subject] lookup is
// correct. Defensive trimming would mask an upstream bug. Trust the contract. (design-decisions.md §3)

// GOTCHA (NEW package — internal/generate is EMPTY): dedupe.go + dedupe_test.go are the FIRST files in
// package generate. No merge-collision risk. The orchestrator T4 lands in this same package later. (§4)

// GOTCHA (imports: "strings" ONLY): dedupe.go needs strings (IndexByte + TrimSpace) for ExtractSubject;
// IsDuplicate needs NO import (builtin map/range). dedupe_test.go: "strings" + "testing". NO "fmt", NO
// internal/*, NO third-party. `go mod tidy` MUST be a no-op. internal/generate stays a stdlib-only LEAF. (§4)

// GOTCHA (in-package white-box tests, NEW file): dedupe_test.go is `package generate`. Table-driven
// pure-function tests (mirror parse_test.go) — NO subprocess, NO temp repo, NO initRepo/makeEmptyCommit
// (those are the git-package style for repo-dependent tests; dedupe has no git dependency). (§5)

// GOTCHA (do NOT pre-empt the orchestrator): this subtask owns ONLY IsDuplicate + ExtractSubject. The
// max_duplicate_retries counter, the 50-subject fetch (FR31 — git.RecentSubjects), the rescue path (FR33),
// and the BuildUserPayload rejection-block call (P1.M3.T1.S3) are T4/T3/M1.T3.S4/M3.T1.S3 respectively.
// Do not implement them here. (§0/§6)
```

## Implementation Blueprint

### Data models and structure

```go
// internal/generate/dedupe.go — NO data models. Two pure functions, no structs, no interfaces.
// (The orchestrator P1.M3.T4 may define a result struct later; dedupe needs none.)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/generate/dedupe.go — IsDuplicate + ExtractSubject
  - FILE: NEW internal/generate/dedupe.go. PACKAGE: `package generate`. IMPORT: EXACTLY "strings" (NO "fmt",
      NO internal/*, NO third-party). IsDuplicate needs NO import (builtin map/range).
  - IMPLEMENT exported `func ExtractSubject(message string) string`:
      - first := message; if nl := strings.IndexByte(message, '\n'); nl >= 0 { first = message[:nl] };
        return strings.TrimSpace(first).
      - GOTCHA: trim the FIRST LINE only (not the whole message). Use IndexByte (zero-alloc, identical to
        Split[0]). Empty message → "".
  - IMPLEMENT exported `func IsDuplicate(subject string, recent []string) bool`:
      - set := make(map[string]struct{}, len(recent)); for _, s := range recent { set[s] = struct{}{} };
        _, dup := set[subject]; return dup.
      - GOTCHA: EXACT, case-SENSITIVE, whole-subject (grep -Fxq). NO trimming inside (inputs pre-trimmed).
  - DOC COMMENTS: cite PRD §9.7 FR30 (ExtractSubject) / FR32 (IsDuplicate); note the commit-pi grep -Fxq
      provenance + the map-set O(1) rationale; note the upstream contracts (ParseOutput trims msg;
      RecentSubjects trims subjects); note the FROZEN-signature + downstream consumer (orchestrator T4).
      Mirror the doc-comment density of git.go / parse.go / system.go.

Task 2: CREATE internal/generate/dedupe_test.go — TestIsDuplicate + TestExtractSubject
  - FILE: NEW internal/generate/dedupe_test.go. PACKAGE: `package generate` (white-box). IMPORT: "strings",
      "testing".
  - IMPLEMENT TestExtractSubject — table with cases: multi-line (body excluded), single-line, empty (""),
      trailing spaces trimmed, leading-whitespace line, \r\n defensive, whitespace-only message. Inline
      "// FR30: subject = first line" citation per case.
  - IMPLEMENT TestIsDuplicate — table with cases: match present (incl. match-in-middle), no match, nil
      recent, empty recent ([]string{}), empty subject, case-SENSITIVE (different case → false),
      exact-not-substring (prefix → false), duplicate-entries-in-recent. Inline "// FR32: exactly matches"
      citation per case.
  - PATTERN: mirror parse_test.go — `cases := []struct{...}{...}` + `for _, tc := range cases { t.Run(tc.name,
      func(t *testing.T){...}) }`. NO subprocess, NO temp repo.
  - GOTCHA: assert case-sensitivity and exactness EXPLICITLY (the two most error-prone semantics).

Task 3: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. Every other file
      (internal/prompt/* — incl. S3's parallel payload.go, internal/git/*, internal/provider/*,
      internal/config/*, cmd/*, pkg/*, Makefile) MUST be byte-unchanged. `go test -race ./internal/generate/`
      MUST be green.
```

### Implementation Patterns & Key Details

```go
// ExtractSubject — FR30: "the generated subject (first line of the message)". LITERAL: first line, trimmed.
func ExtractSubject(message string) string {
	// First line = everything up to the first '\n'. IndexByte is O(pos) and allocates nothing —
	// semantically identical to strings.Split(message, "\n")[0] but avoids building the full line slice.
	// The message is already trimmed + newline-normalized by provider.ParseOutput (P1.M2.T6.S1 Step 4/5),
	// so we trim only the first line (clears trailing spaces on the subject line). Do NOT trim the whole
	// message first (deviates from the work-item's literal "split on \n, trim").
	first := message
	if nl := strings.IndexByte(message, '\n'); nl >= 0 {
		first = message[:nl]
	}
	return strings.TrimSpace(first)
}

// IsDuplicate — FR32: "if the subject exactly matches one of the 50". Exact, case-SENSITIVE, whole-subject
// (the Go map-set port of commit-pi's `grep -Fxq`: -x = whole line, no -i = case-sensitive). O(1) lookup.
func IsDuplicate(subject string, recent []string) bool {
	// Build a set from recent. Both `subject` (from ExtractSubject, trimmed) and each `recent` element
	// (from git.RecentSubjects, trimmed + empties skipped) are pre-trimmed by their producers, so a plain
	// map lookup is correct — NO defensive trimming (would mask an upstream bug).
	set := make(map[string]struct{}, len(recent))
	for _, s := range recent {
		set[s] = struct{}{}
	}
	_, dup := set[subject]
	return dup
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. dedupe.go uses stdlib strings ONLY. `go mod tidy` MUST be a no-op.
        `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - dedupe.go → (stdlib: strings) ONLY. It does NOT import fmt, internal/git, internal/provider,
        internal/prompt, internal/config, os/exec, or any third-party. internal/generate is a stdlib-only
        LEAF for this subtask.
  - dedupe_test.go → (stdlib: strings, testing) ONLY. In-package (package generate).

UPSTREAM CONTRACT (the inputs — do NOT implement, just consume):
  - P1.M1.T3.S4 git.RecentSubjects(ctx, n int) ([]string, error): returns up to n single-line, TrimSpace'd,
        newest-first subjects; nil on unborn/non-repo (exit 128). This IS the `recent` parameter. Default
        n=50 (FR31). Each subject is single-line by git's %s definition and trimmed by the producer.
  - P1.M2.T6.S1 provider.ParseOutput(raw, m Manifest) (msg string, ok bool, fellback bool): Step 4
        normalizes newlines (\r\n→\n, collapse 3+\n→2); Step 5 TrimSpaces msg. This IS the `message`
        parameter — arrives trimmed + newline-normalized.

DOWNSTREAM CONTRACTS (the consumers — do NOT implement here, just honor the signatures):
  - P1.M3.T4 (orchestrator CommitStaged): per attempt — `subject := generate.ExtractSubject(msg)`;
        `if generate.IsDuplicate(subject, recent) { rejected = append(rejected, subject); ...retry... }`.
        The retry counter (cfg.MaxDuplicateRetries, default 3) and the FR33 rescue path are T4/T3.
  - P1.M3.T1.S3 (parallel, payload.go): prompt.BuildUserPayload(diff, rejected) — the orchestrator passes
        the appended `rejected` slice; BuildUserPayload emits the §17.3 rejection block listing them.
  - P1.M3.T5 (public API pkg/stagecoach): may re-export IsDuplicate/ExtractSubject.
  => The `IsDuplicate(subject string, recent []string) bool` and `ExtractSubject(message string) string`
     signatures are FROZEN after this subtask.

FROZEN FILES (do NOT edit):
  - internal/prompt/* (S1/S2/S3 own the prompt layer; S3's payload.go is implementing in parallel),
        internal/git/* (RecentSubjects built), internal/provider/* (ParseOutput built), internal/config/*,
        cmd/stagecoach/main.go, pkg/*, Makefile, go.mod, go.sum.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the two new files
gofmt -w internal/generate/dedupe.go internal/generate/dedupe_test.go

# Vet the generate package (dedupe.go + dedupe_test.go)
go vet ./internal/generate/

# Confirm dedupe.go imports EXACTLY "strings" (no fmt, no internal/*, no third-party)
sed -n '/^import/,/)/p' internal/generate/dedupe.go   # → a single "strings" import

# Confirm go.mod/go.sum are unchanged (stdlib only)
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged ✓"

# Expected: Zero errors. `go vet` clean. No non-ASCII bytes (no em-dash risk here — pure logic).
```

### Level 2: Unit Tests (THE KEYSTONE — table-driven suite)

```bash
# Run the NEW suite verbosely (every subtest listed)
go test -race -v ./internal/generate/ -run 'TestIsDuplicate|TestExtractSubject'

# Full generate package
go test -race ./internal/generate/

# Coverage of the new file specifically
go test -coverprofile=coverage.out ./internal/generate/ && go tool cover -func=coverage.out | grep -E 'dedupe.go|IsDuplicate|ExtractSubject'

# Expected: All pass. The case-sensitivity + exact-not-substring assertions in TestIsDuplicate are the
# load-bearing ones — if either fails, IsDuplicate is doing fuzzy matching (wrong). The body-excluded +
# trailing-spaces-trimmed assertions in TestExtractSubject pin FR30.
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-module build (dedupe is internal; confirms nothing else broke, incl. S3's parallel payload.go)
go build ./...

# Optional: confirm the primitives behave as the orchestrator will call them (sanity print, not a test)
go test -run 'TestIsDuplicate|TestExtractSubject' -v ./internal/generate/

# Expected: `go build ./...` succeeds; the suite prints PASS. dedupe has no runtime/endpoint/DB/git surface
# — it is two pure functions — so there is no service to start, no curl, no repo to check.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Not applicable for this subtask (pure functions, no I/O, no network, no DB, no UI, no git).
# The strongest creative check is human review: confirm the match semantics against PRD §9.7 FR32
# ("exactly matches") + commit-pi's `grep -Fxq` (case-sensitive, whole-line). Then confirm the orchestrator
# wiring contract (Integration Points) matches what P1.M3.T4 will call:
#   subject := generate.ExtractSubject(msg)
#   if generate.IsDuplicate(subject, recent) { rejected = append(rejected, subject); ... }
# and that prompt.BuildUserPayload(diff, rejected) (P1.M3.T1.S3) consumes the appended `rejected` slice.
#
# Optional end-to-end mental trace against Appendix B.4:
#   Attempt 1: ExtractSubject → "fix: handle null user"; IsDuplicate(..., last50) → true → retry.
#   Attempt 2: ExtractSubject → "fix: guard against missing user record"; IsDuplicate → false → accept.
# (The loop + retry counter is T4; dedupe supplies only the two calls per attempt.)
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed successfully.
- [ ] All tests pass: `go test -race ./internal/generate/`.
- [ ] `go build ./...` succeeds.
- [ ] No vet errors: `go vet ./internal/generate/`.
- [ ] No formatting issues: `gofmt -l internal/generate/` (empty output).
- [ ] go.mod/go.sum byte-unchanged: `git diff --exit-code go.mod go.sum`.

### Feature Validation

- [ ] `ExtractSubject` returns the trimmed first line (FR30): multi-line body excluded, single-line passthrough, empty→"", trailing spaces trimmed.
- [ ] `IsDuplicate` is exact (FR32): match present→true, no match→false, nil/empty recent→false.
- [ ] `IsDuplicate` is case-SENSITIVE: different-case subject → false (pinned by a test).
- [ ] `IsDuplicate` is whole-subject (not substring/prefix): prefix subject → false (pinned by a test).
- [ ] Scope respected: NO retry loop, NO counter, NO rescue, NO rejection-list builder, NO BuildUserPayload call (those are T4/T3/M3.T1.S3).
- [ ] Manual trace against Appendix B.4 succeeds (ExtractSubject + IsDuplicate per attempt).

### Code Quality Validation

- [ ] Follows existing codebase patterns: exported pure functions (mirror `prompt.DetectMultiline`/`BuildUserPayload`), table-driven pure-function tests (mirror `provider/parse_test.go`).
- [ ] File placement matches the desired codebase tree (`internal/generate/dedupe.go` + `dedupe_test.go`).
- [ ] Anti-patterns avoided (see Anti-Patterns section).
- [ ] Imports properly managed: `"strings"` only in dedupe.go; no new dependency.
- [ ] Doc comments cite PRD §9.7 FR30/FR32 and the commit-pi `grep -Fxq` provenance.

### Documentation & Deployment

- [ ] Code is self-documenting with clear function/variable names + PRD-cited doc comments.
- [ ] No new environment variables (none needed — pure functions).
- [ ] No new config keys (MaxDuplicateRetries is the orchestrator's, not dedupe's).

---

## Anti-Patterns to Avoid

- ❌ Don't implement the retry loop / counter / rescue here — that's the orchestrator (P1.M3.T4). This subtask is the two PRIMITIVES.
- ❌ Don't make IsDuplicate case-insensitive or substring-based — FR32 "exactly matches" = `grep -Fxq` = exact, case-sensitive, whole-subject.
- ❌ Don't trim the whole message inside ExtractSubject — trim only the first line (ParseOutput already trimmed the whole; the work-item says "split on \n, trim").
- ❌ Don't add defensive trimming inside IsDuplicate — both inputs are pre-trimmed by producers; trimming would mask upstream bugs.
- ❌ Don't import `fmt`/`internal/*`/third-party — `"strings"` only; `go mod tidy` must be a no-op.
- ❌ Don't edit `internal/prompt/*` (S3's payload.go is implementing in parallel) or any other frozen file.
- ❌ Don't use the temp-repo test style (`initRepo`/`makeEmptyCommit`) — dedupe is pure-function; use the `parse_test.go` table-driven style.
- ❌ Don't lowercase, `strings.EqualFold`, or `strings.Contains` for the duplicate check — exact map membership only.
