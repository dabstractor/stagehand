---
name: "P1.M3.T3.S1 — Rescue message formatting (FormatRescue): the pure string-assembler for the §18.3 rescue message — PRD §9.10 FR43–FR45 / §18.3"
description: |

  Land the ONLY subtask of Rescue Protocol (P1.M3.T3): CREATE `internal/generate/rescue.go`
  (`package generate`) exporting ONE pure function — `FormatRescue(treeSHA, parentSHA, candidateMsg
  string) string` — that returns the PRD §18.3 rescue message (FR43–FR45), byte-for-byte verbatim:
  the `❌ Commit generation failed.` notice (❌ = U+274C, NON-ASCII), the 60-dash separator (×2), the
  "safely snapshotted" reassurance, the `Tree ID: <treeSHA>` line, and the manual-recovery command
  `git commit-tree [-p <parentSHA>] -m "Your message" <treeSHA> | xargs git update-ref HEAD` (the
  `-p <parentSHA>` segment OMITTED when parentSHA == "" — root commit; mirrors git.CommitTree's
  root semantics, FR39). PLUS the `(omit "-p <PARENT_SHA>" if this is the repository's first commit)`
  hint line (kept in BOTH cases — §18.3 verbatim). When `candidateMsg != ""`, APPEND the §18.3
  candidate note `A candidate message was produced but rejected: "<candidateMsg>". You can use it
  manually in the command above.` (separated from the closing separator by ONE blank line). PLUS
  `internal/generate/rescue_test.go` — canonical-exact + structural-invariant tests in the style of
  `internal/prompt/payload_test.go`.

  SCOPE BOUNDARY (load-bearing): this subtask provides the FORMATTER ONLY. It does NOT print anything
  (CLI P1.M4.T1), does NOT exit 3 (exit-code system P1.M4.T3.S3), does NOT detect the rescue condition
  TREE_SHA-set + NEW_SHA-unset (orchestrator P1.M3.T4, FR43), does NOT install the SIGINT/SIGTERM
  handler or produce the "(interrupted)" variant (signal handler P1.M4.T2), and does NOT fetch
  TREE_SHA/PARENT_SHA from git (P1.M1.T2.S3/S2). FormatRescue is a PURE string assembler: 3 strings in,
  1 string out, no I/O, no error. See research design-decisions.md §0.

  INPUT (upstream — already built, read-only): `treeSHA` = `git.WriteTree(ctx)` (P1.M1.T2.S3, the
  snapshot tree SHA — non-empty when rescue is reachable). `parentSHA` = the `sha` of
  `git.RevParseHEAD(ctx)` (P1.M1.T2.S2); `""` when `isUnborn == true` (FINDING 1: unborn detected by
  git exit 128, NOT string emptiness) ⇒ root commit ⇒ command omits -p. `candidateMsg` = the `msg` of
  `provider.ParseOutput(raw, manifest)` (P1.M2.T6.S1, Step 4/5 trim+normalize) IF a message was produced
  before the duplicate/parse rejection; `""` otherwise (timeout/interrupt/no-output). The orchestrator
  decides which failure mode (§18.2) and passes the appropriate candidateMsg.

  OUTPUT (downstream consumer): the orchestrator P1.M3.T4 `CommitStaged` calls
  `generate.FormatRescue(treeSHA, parentSHA, candidateMsg)` on the rescue path (FR43–FR45), hands the
  string to the CLI layer P1.M4.T1 which `fmt.Fprintln`s it (to stderr) and the exit-code system
  P1.M4.T3.S3 sets exit 3. The signature is FROZEN after this subtask.

  ⚠️ **THE signature — implement the WORK-ITEM form EXACTLY.** `func FormatRescue(treeSHA, parentSHA,
  candidateMsg string) string`. EXPORTED (capitalized F — per the work-item; the public API P1.M3.T5
  may re-export). THREE string params IN THIS ORDER, ONE string return, NO error (formatting has no
  failure mode). NOT `(string, error)`, NOT `*strings.Builder`. See design-decisions.md §1.

  ⚠️ **§18.3 VERBATIM — copy it character-for-character.** The message is 10 lines (prd_snapshot.md
  1177–1186, byte-verified): line 1 `❌ Commit generation failed.` (❌ = U+274C, NON-ASCII — 3 UTF-8
  bytes E2 9D 8C; write the literal ❌ in the Go string, do NOT substitute ASCII); lines 2 + 10 are
  EXACTLY 60 hyphens (verified: `tr -cd '-' | wc -c` == 60); line 7 (the command) has EXACTLY 2 leading
  spaces (verified via `cat -A`). The `<TREE_SHA>`/`<PARENT_SHA>` tokens are STRUCTURAL annotations —
  substitute the runtime args (like payload.go substitutes `diff` for `<diff payload>`), do NOT emit
  the literal `<…>` tokens. See design-decisions.md §2.

  ⚠️ **THE key decision — keep the `(omit "-p <PARENT_SHA>" if this is the repository's first commit)`
  hint line in BOTH cases (§18.3 line 9).** The CONTRACT says "returning the §18.3 formatted message"
  + specifies exactly ONE dynamic modification ("command omits -p for root"). Literal reading =
  EVERYTHING ELSE stays §18.3-verbatim, including line 9. The command line is the ONLY fully-dynamic
  line. Do NOT drop line 9 in the root case (would be an undocumented §18.3 deviation). See
  design-decisions.md §3.

  ⚠️ **ROOT commit — omit the ` -p <parentSHA>` SEGMENT, not just blank it.** When parentSHA == "",
  the command is `  git commit-tree -m "Your message" <treeSHA> | xargs git update-ref HEAD` (no `-p`
  substring at all). Mirrors git.CommitTree (P1.M1.T2.S4): `parents == nil/empty ⇒ no -p`. Gate on
  `parentSHA != ""`. See design-decisions.md §4.

  ⚠️ **NO "(interrupted)" variant — OUT OF SCOPE.** FormatRescue produces the §18.3 BASE form
  (`❌ Commit generation failed.`) ALWAYS. Appendix B.5's `❌ Commit generation failed (interrupted).`
  is the SIGNAL HANDLER's (P1.M4.T2) render — FormatRescue's signature has no `interrupted` param.
  Do NOT add a param, a reason, or a second function. See design-decisions.md §5.

  ⚠️ **Candidate note — appended AFTER the closing separator, ONE blank line, iff candidateMsg != "".**
  Exact text: `A candidate message was produced but rejected: "<candidateMsg>". You can use it
  manually in the command above.` (literal ASCII double-quotes around candidateMsg). Return value tail
  = closing-sep + `\n\n` + note. No trailing newline (CLI adds it). See design-decisions.md §6/§7.

  ⚠️ **NO new dependency — stdlib `strings` ONLY.** rescue.go imports `"strings"` (strings.Builder +
  WriteString); build the command with a conditional splice (NOT fmt.Sprintf — avoids the `fmt` import).
  rescue_test.go imports `"strings"` + `"testing"`. NO `"fmt"`, NO `internal/*`, NO third-party. `go mod
  tidy` MUST be a no-op; `git diff --exit-code go.mod go.sum` MUST be empty. internal/generate stays a
  stdlib-only LEAF. See design-decisions.md §8.

  ⚠️ **DO NOT add a second `// Package generate` doc comment.** dedupe.go (P1.M3.T2.S1,
  parallel-implementing) already carries the package doc. A second one triggers a revive/golint
  duplicate-package-comment warning. rescue.go uses FUNCTION-level doc comments ONLY (on FormatRescue
  + constants). See design-decisions.md §8.

  Deliverable: CREATE `internal/generate/rescue.go` — exported `FormatRescue(treeSHA, parentSHA,
  candidateMsg string) string` (§18.3 message, root -p omission, conditional candidate note). PLUS
  `internal/generate/rescue_test.go` — 4 canonical-exact tests (rooted/rootless × with/without
  candidate) + a structural-invariant table. Touches ONLY these two NEW files — NO go.mod/go.sum change,
  NO edit to internal/generate/dedupe.go (P1.M3.T2.S1's), internal/prompt/*, internal/git/*,
  internal/provider/*, internal/config/*, cmd/*, pkg/*, or the Makefile.

---

## Goal

**Feature Goal**: Implement the rescue-message formatter (PRD §9.10 FR43–FR45 / §18.3) that the
generation orchestrator (P1.M3.T4) and CLI layer (P1.M4.T1) use to print the manual-recovery
instructions when commit generation fails after the snapshot was taken. It is a single pure function
that assembles the §18.3 message byte-for-byte — the failure notice, the snapshot reassurance, the
`Tree ID`, the exact copy-pasteable `git commit-tree … | xargs git update-ref HEAD` recovery command
(with `-p <parentSHA>` omitted for a root/unborn repo), the first-commit hint, and (when a candidate
message was produced but rejected) the §18.3 candidate note so the user's wait wasn't wasted. This is
the entirety of P1.M3.T3; the rescue CONDITION detection, exit code 3, and SIGINT/SIGTERM "(interrupted)"
variant belong to later subtasks.

**Deliverable**:
1. **CREATE** `internal/generate/rescue.go` (`package generate`, imports `"strings"` ONLY) —
   - exported `func FormatRescue(treeSHA, parentSHA, candidateMsg string) string` — returns the §18.3
     rescue message: the 10-line boxed block (notice + 60-dash separators + reassurance + Tree ID +
     blank + manual-recovery header + command (2 leading spaces; `-p <parentSHA>` omitted when
     parentSHA == "") + blank + first-commit hint + closing 60-dash separator), with NO trailing
     newline; and, when `candidateMsg != ""`, a `\n\n` separator + the §18.3 candidate note.
2. **CREATE** `internal/generate/rescue_test.go` (`package generate`, imports `"strings"` + `"testing"`) —
   4 canonical-exact tests (rooted/rootless × candidate/no-candidate) + `TestFormatRescue_Properties`
   (structural-invariant table), mirroring `internal/prompt/payload_test.go`.

No other files touched. **No new file besides the two above. No go.mod/go.sum change** (stdlib
`strings` only). NO edit to `internal/generate/dedupe.go` or `dedupe_test.go` (P1.M3.T2.S1 owns them),
`internal/prompt/*`, `internal/git/*`, `internal/provider/*`, `internal/config/*`, `cmd/*`, `pkg/*`,
the `Makefile`.

**Success Definition**: `go build ./...` succeeds; `go test -race ./internal/generate/` is green with
the new suite passing (alongside P1.M3.T2.S1's dedupe tests); `gofmt -l internal/generate/` clean;
`go vet ./internal/generate/` clean; `golangci-lint run` (if available) clean (incl. NO duplicate
package-comment warning); go.mod/go.sum byte-unchanged; `FormatRescue` returns the §18.3 message
byte-for-byte for all four (rooted/rootless × candidate/no-candidate) cases; the command line omits
`-p` iff parentSHA == ""; the candidate note is appended iff candidateMsg != ""; the output never
ends with `\n` and never contains "interrupted".

## User Persona

**Target User**: The generate orchestrator (P1.M3.T4 `CommitStaged`) — on the rescue path (FR43:
TREE_SHA set and NEW_SHA not set) it calls `generate.FormatRescue(treeSHA, parentSHA, candidateMsg)`
and hands the result to the CLI layer. Transitively: `git.WriteTree` (P1.M1.T2.S3 — supplies
treeSHA), `git.RevParseHEAD` (P1.M1.T2.S2 — supplies parentSHA; "" for unborn), `provider.ParseOutput`
(P1.M2.T6.S1 — supplies candidateMsg when a message was produced). The CLI layer (P1.M4.T1) prints the
string and the exit-code system (P1.M4.T3.S3) exits 3. End-user persona is "the plan-holder" / "the
API-key refusenik" / "the multi-agent tinkerer" (PRD §7) whose generation failed but whose staged
files were safely snapshotted — they need a copy-pasteable recovery command (PRD §18.3 / Appendix B.5).

**Use Case**: When generation fails after the snapshot (timeout, SIGINT/SIGTERM, parse failure, or
duplicate-exhaustion — §18.2), the orchestrator has a TREE_SHA (the snapshot) but no NEW_SHA (no
commit was made). Rather than silently fail, Stagecoach prints the §18.3 rescue message: it reassures
the user their staged files are safe, shows the Tree ID, and gives the EXACT `git commit-tree | xargs
git update-ref HEAD` plumbing command to recover manually (with `-p <PARENT_SHA>` for a normal commit,
omitted for a repo's first commit). If a candidate message was produced but rejected, it is printed
too so the user can paste it into the `-m` slot.

**User Journey**: (internal API, no new end-user surface)
snapshot (`WriteTree`→treeSHA, `RevParseHEAD`→parentSHA) → generate → fail (timeout/parse/dedupe/
SIGINT) → orchestrator builds `candidateMsg` (the parsed msg, or "") → `FormatRescue(treeSHA,
parentSHA, candidateMsg)` → CLI `Fprintln(msg)` → exit 3. The user reads the message, copies the
command, runs it, and their originally-staged files are committed.

**Pain Points Addressed**: (1) Silent failure losing the user's staged snapshot — solved by the
recovery command (the snapshot tree is a real git object; `commit-tree` publishes it). (2) The user
not knowing HOW to recover — solved by an exact copy-pasteable plumbing command. (3) A wasted candidate
message (model produced a good message that was rejected for duplication) — solved by the candidate
note (§18.3 last paragraph). (4) Root-commit confusion (omit -p) — solved by the command adapting +
the hint line.

## Why

- **Unblocks the rescue path end-to-end.** Until `FormatRescue` exists, the orchestrator (P1.M3.T4)
  has no way to render the §18.3 message. P1.M3.T4 depends on this subtask; the CLI (P1.M4.T1), the
  exit-code system (P1.M4.T3.S3), and the signal handler's non-interrupted path (P1.M4.T2) depend
  transitively.
- **Satisfies PRD §9.10 FR43–FR45 + §18.3.** FR43 = the rescue condition (orchestrator); FR44 = print
  the failure notice + TREE_SHA + the exact recovery command; FR45 = the SIGINT/SIGTERM handler
  (P1.M4.T2). This subtask IS FR44's message-assembly core (the orchestrator/CLI do the printing +
  exit). §18.3 is the authoritative message spec — implemented byte-for-byte.
- **Faithful to the proven commit-pi safety model.** commit-pi's rescue prints the same
  `commit-tree | update-ref` plumbing so a failure never loses staged work (PRD Appendix C porting map).
  The Go port renders it deterministically from the captured SHAs.
- **No new user-facing surface** (PRD "DOCS: none — internal"). No new dependency (stdlib `strings` only).

## What

A new file `internal/generate/rescue.go` with one exported pure function, plus a new test file
`rescue_test.go`. No new types, no I/O, no config, no git, no subprocess, no signal handling. The
function is a deterministic string assembler: it returns the §18.3 message, with the command line
adapted to root vs. rooted, and the candidate note conditionally appended.

### Success Criteria

- [ ] `internal/generate/rescue.go` exists, `package generate`, imports EXACTLY `"strings"` (NO `"fmt"`,
      NO `internal/*`, NO third-party). Defines exported `FormatRescue(treeSHA, parentSHA, candidateMsg
      string) string`. Carries NO `// Package generate` doc comment (dedupe.go owns the package doc).
- [ ] `FormatRescue("9f3a1c", "abc1234", "")` returns EXACTLY the §18.3 rooted message (10 lines): the
      `❌` notice (U+274C), the 60-dash separators (×2), the "safely snapshotted" line, `Tree ID: 9f3a1c`,
      a blank line, the manual-recovery header, the command `  git commit-tree -p abc1234 -m "Your
      message" 9f3a1c | xargs git update-ref HEAD` (2 leading spaces, real SHAs), a blank line, the
      `(omit "-p <PARENT_SHA>" if this is the repository's first commit)` hint, and the closing
      separator. No trailing newline. No "(interrupted)".
- [ ] `FormatRescue("9f3a1c", "", "")` (ROOT/unborn) returns the SAME 10-line structure but the command
      line is `  git commit-tree -m "Your message" 9f3a1c | xargs git update-ref HEAD` (NO `-p`
      substring anywhere in the command line). The hint line is STILL present.
- [ ] `FormatRescue("9f3a1c", "abc1234", "feat: add bar")` returns the rooted message + `\n\n` + the
      §18.3 candidate note `A candidate message was produced but rejected: "feat: add bar". You can use
      it manually in the command above.` (literal `"` quotes around the message). Still no trailing newline.
- [ ] `FormatRescue("9f3a1c", "", "fix: x")` (root + candidate) returns the root message + candidate note.
- [ ] The candidate note is appended IFF `candidateMsg != ""`; for `candidateMsg == ""` there is no
      candidate note (the boxed message alone).
- [ ] The output NEVER ends with `\n` (`!strings.HasSuffix(got, "\n")` for all four cases).
- [ ] The output NEVER contains the substring "interrupted".
- [ ] The separator lines are EXACTLY 60 hyphens (`strings.Count`/length check on both separators).
- [ ] The command line has EXACTLY 2 leading spaces and contains treeSHA (in both the `Tree ID:` line
      and the command) and contains parentSHA iff parentSHA != "".
- [ ] `go build ./...` succeeds; `go test -race ./internal/generate/` green (rescue + dedupe suites);
      `gofmt -l internal/generate/` clean; `go vet ./internal/generate/` clean.
- [ ] go.mod/go.sum byte-unchanged (`git diff --exit-code go.mod go.sum` empty).
- [ ] Every other file byte-unchanged (`internal/generate/dedupe.go`, `internal/prompt/*`,
      `internal/git/*`, `internal/provider/*`, `internal/config/*`, `cmd/*`, `pkg/*`, `Makefile`).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: PRD §18.3 +
§9.10 FR43–FR45 (already in context as selected_prd_content; also prd_snapshot.md 1172–1189), the
design decisions (research design-decisions.md — the single most important read, esp. §2 the exact
message bytes, §3 the keep-the-hint-line decision, §4 the root -p omission, §6 the candidate note
position/text, §8 the no-second-package-doc gotcha), the upstream contracts (`git.WriteTree`/`RevParseHEAD`
supply the SHAs; `provider.ParseOutput` supplies candidateMsg), the in-package pure-function +
multi-line-string-test conventions (`prompt.BuildUserPayload` + `payload_test.go`), and the
copy-ready Go code in the Implementation Blueprint. No CLI/exit-code/signal/git-plumbing knowledge
required — FormatRescue is one pure string assembler + its tests.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M3T3S1/research/design-decisions.md
  why: the SINGLE most important read — the 10 decisions specific to this subtask: the scope boundary
       (FORMAT ONLY — §0), the frozen signature (§1), §18.3 verbatim + the byte-verification table
       (60 dashes, ❌=U+274C, 2 leading spaces — §2), THE key decision to keep the "(omit -p)" hint
       line in BOTH cases (§3), the root -p SEGMENT omission (§4), the "(interrupted)" variant is OUT
       OF SCOPE (§5), the candidate note position/text (§6), no trailing newline (§7), package +
       imports + the no-second-package-doc gotcha (§8), the test strategy (§9), frozen files +
       contracts (§10).
  critical: §2 (be byte-exact: 60 dashes, ❌ not ASCII, 2 leading spaces), §3 (keep the hint line
       always — do NOT drop it for root), §4 (omit the whole ` -p <parentSHA>` segment when root),
       §6 (candidate note AFTER the closing sep with one blank line; literal quotes), §8 (NO second
       `// Package generate` comment) are the things most likely to be implemented wrong.

- file: internal/prompt/payload.go   (P1.M3.T1.S3 — READ for the multi-line-string ASSEMBLY pattern; do NOT edit)
  section: `func BuildUserPayload(diff string, rejected []string) string` — builds a §17.x message with
           `var b strings.Builder` + `WriteString`/`WriteByte`, owns ALL inter-block newline placement,
           returns WITHOUT a trailing newline, defines verbatim string CONSTANTS for the load-bearing
           fragments. The NORMAL/REJECTION branches show the conditional-assembly style.
  why: the CLOSEST architectural precedent — FormatRescue is the rescue-layer analogue of
       BuildUserPayload (pure string assembler, multi-line §1x.x output, no trailing newline, Builder
       + conditional branches). Mirror its structure + its "verbatim constants, no trailing newline,
       independently-derived test want" conventions.
  pattern: `var b strings.Builder` → write each line with `WriteString`/`WriteByte('\n')` → return
           `b.String()` with NO trailing `\n`. Define the 60-dash separator (used twice) as a named
           `const` (DRY + the count is pinned once); inline the one-off lines or split around the
           dynamic SHAs (mirror payload.go's `rejectionPreamble`/`rejectionEpilogue` style).
  gotcha: payload.go returns WITHOUT a trailing newline — FormatRescue does too (§7; the CLI's Fprintln
          adds it). Do NOT copy payload.go's CONSTANTS (rescue has its own §18.3 strings).

- file: internal/prompt/payload_test.go   (P1.M3.T1.S3 — READ for the TEST PATTERN to mirror; do NOT edit)
  section: `TestBuildUserPayload_NormalCanonicalExact` + `TestBuildUserPayload_RejectionCanonicalExact` —
           build the `want` string INDEPENDENTLY from the PRD (string concatenation, NOT calling the
           impl), compare `got != want`, error with `--- got ---\n%q\n--- want ---\n%q`.
  why: the TEST CONVENTION to follow verbatim in rescue_test.go. FormatRescue has a multi-line output
       (like BuildUserPayload), so it uses the canonical-exact style — independently-derived `want`,
       `got != want`, `%q` diff. The 4 canonical cases (rooted/rootless × candidate/no-candidate) each
       get a `want` built from §18.3 + `strings.Repeat("-", 60)`.
  pattern: copy the `want := "…" + "…" + …` independent derivation + the `if got != want { t.Errorf(…%q…%q…) }`
           form. Add a `TestFormatRescue_Properties` table (mirror `parse_test.go`/`dedupe_test.go`) for
           structural invariants that the canonical tests don't isolate (❌ present, 2× 60-dash sep,
           -p iff rooted, no trailing newline, no "interrupted", candidate note iff candidateMsg != "").

- file: internal/git/git.go   (P1.M1.T2.S4 — READ for the CommitTree root-commit semantics; do NOT edit)
  section: `func (g *gitRunner) CommitTree(ctx, tree, parents []string, msg) (sha, err)` (lines ~230–255)
           — `parents == nil/empty ⇒ root commit (no -p appended); each element appends a -p <parent>`.
  why: the PROVEN root-commit semantics FormatRescue mirrors. The rescue command's `-p <parentSHA>`
       omission (parentSHA == "" ⇒ no -p) is the SAME gate CommitTree uses (`len(parents) == 0`). FR39
       confirms ("if PARENT_SHA is non-empty, git commit-tree -p …; else git commit-tree -m …"). Do not
       reinvent the rule — match CommitTree.
  critical: gate on `parentSHA != ""` (a STRING emptiness check — parentSHA is already resolved by the
            caller; RevParseHEAD returned isUnborn, and the orchestrator passed "" for unborn). Do NOT
            call RevParseHEAD inside FormatRescue (it's a pure function; §0).

- file: internal/generate/dedupe.go   (P1.M3.T2.S1 — READ for the sibling pure-function + package conventions; do NOT edit)
  section: the `// Package generate …` doc comment (dedupe.go OWNS it — rescue.go must NOT add a second
           one, §8) + `ExtractSubject`/`IsDuplicate` (exported, single-value, no-error, stdlib-only,
           PRD-cited doc comments, FROZEN-signature notes).
  why: the architectural PRECEDENT in the SAME package — Stagecoach's generation pipeline is built from
       small exported pure functions. FormatRescue is exactly this shape (exported, string-only,
       no-error, stdlib-only, PRD-cited doc comment, FROZEN-signature note). Mirror the doc-comment
       density (cite PRD §18.3 / FR44 + the commit-pi provenance + the FROZEN-signature + downstream
       consumer).
  gotcha: dedupe.go already has the package doc — rescue.go's first line is `package generate` with NO
          preceding `// Package generate …` comment (function-level doc comments only); else revive/
          golint flags a duplicate package comment.

- url: (PRD §9.10 FR43–FR45 + §18.3 + Appendix B.5 — already in context as selected_prd_content
       `h3.26`/`h2.18`/`h3.66`/`h3.84`; ALSO in plan/001_f1f80943ac34/prd_snapshot.md lines 1172–1189
       and 1383–1396)
  why: §18.3 (lines 1177–1186) is the AUTHORITATIVE message spec — the 10-line boxed block. §18.3 last
       paragraph (line 1189) is the candidate-note spec. FR44 defines the recovery command shape.
       B.5 (lines 1389–1396) is the signal-handler EXAMPLE render — note it differs from §18.3 in TWO
       ways ("(interrupted)" + no hint line) and is therefore OUT OF SCOPE for FormatRescue (§5).
  critical: §18.3 line 1177 is `❌ Commit generation failed.` (NOT "(interrupted)"); the separator is
            60 dashes; the command has 2 leading spaces; `-p <PARENT_SHA>` is omitted for the first
            commit. The candidate note uses literal `"` quotes. Copy §18.3 verbatim; treat B.5 as the
            signal handler's (P1.M4.T2) separate concern.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; go-toml/v2 + pflag  (UNCHANGED — rescue adds NO dep: stdlib strings only)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 — untouched
  generate/                     # P1.M3 — this subtask ADDS rescue.go + rescue_test.go to the package P1.M3.T2.S1 opened
    dedupe.go                   # EXISTS (P1.M3.T2.S1, parallel) — UNCHANGED (owns the package doc)
    dedupe_test.go              # EXISTS (P1.M3.T2.S1, parallel) — UNCHANGED
    rescue.go                   # NEW (this subtask) ← FormatRescue
    rescue_test.go              # NEW (this subtask) ← 4 canonical-exact + TestFormatRescue_Properties
  git/                          # P1.M1.T2/T3 — untouched (WriteTree/RevParseHEAD read-only refs; CommitTree root semantics)
  prompt/                       # P1.M3.T1 — DONE — untouched (payload.go/payload_test.go read-only refs)
  provider/                     # P1.M2 (T1–T6) — untouched (parse.go ParseOutput read-only ref)
  ui/                           # P1.M4 (empty stub) — untouched
cmd/stagecoach/main.go           # stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  generate/
    rescue.go        # NEW — exported FormatRescue(treeSHA, parentSHA, candidateMsg string) string (§18.3 message)
    rescue_test.go   # NEW — 4 canonical-exact (rooted/rootless × candidate/no-candidate) + TestFormatRescue_Properties
# All other files UNCHANGED. go.mod/go.sum UNCHANGED. After this subtask: the rescue formatter exists for
# the orchestrator (P1.M3.T4) to call on the rescue path; P1.M3.T4 calls FormatRescue(treeSHA, parentSHA,
# candidateMsg), hands the string to the CLI (P1.M4.T1) which prints it + exits 3 (P1.M4.T3.S3).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (SCOPE — FORMAT ONLY): this subtask exports FormatRescue ONLY. It does NOT print (CLI
// P1.M4.T1), does NOT exit 3 (exit-code system P1.M4.T3.S3), does NOT detect the rescue condition
// (orchestrator P1.M3.T4 FR43), does NOT install the SIGINT/SIGTERM handler or produce "(interrupted)"
// (signal handler P1.M4.T2), does NOT fetch SHAs from git (P1.M1.T2.S3/S2). Pure assembler: 3 strings
// in, 1 string out, no I/O, no error. (design-decisions.md §0)

// CRITICAL (signature — EXPORTED, 3 string params, 1 string return, NO error): implement EXACTLY
//   func FormatRescue(treeSHA, parentSHA, candidateMsg string) string
// Param order is treeSHA, parentSHA, candidateMsg. NOT (string, error), NOT *strings.Builder. (§1)

// CRITICAL (§18.3 VERBATIM — byte-exact): the message is 10 lines (prd_snapshot.md 1177–1186).
//   - line 1: "❌ Commit generation failed." — the ❌ is U+274C (NON-ASCII, bytes E2 9D 8C). Write the
//     literal ❌ in the Go double-quoted string. Do NOT replace with [X]/x/!.
//   - lines 2 + 10: EXACTLY 60 '-' (verified: tr -cd '-' | wc -c == 60). Pure ASCII, no spaces.
//   - line 7 (command): EXACTLY 2 leading spaces (verified: cat -A). The "Your message" has literal
//     ASCII double-quotes (part of the shell template the user copy-pastes).
//   - <TREE_SHA>/<PARENT_SHA> are STRUCTURAL annotations — substitute the runtime args (do NOT emit
//     the literal <…> tokens). (§2)

// CRITICAL (THE key decision — keep the hint line ALWAYS): §18.3 line 9
//   `(omit "-p <PARENT_SHA>" if this is the repository's first commit)`
// is emitted in BOTH rooted and rootless cases, byte-for-byte. The CONTRACT specifies exactly ONE
// dynamic modification ("command omits -p for root"); everything else stays §18.3-verbatim. Do NOT
// drop line 9 for root (undocumented deviation). The command line is the ONLY fully-dynamic line. (§3)

// CRITICAL (ROOT — omit the ` -p <parentSHA>` SEGMENT): when parentSHA == "", the command is
//   "  git commit-tree -m \"Your message\" <treeSHA> | xargs git update-ref HEAD"
// (NO "-p" substring). Mirrors git.CommitTree (parents==nil/empty ⇒ no -p). Gate on parentSHA != "".
// (§4)

// CRITICAL (NO "(interrupted)"): FormatRescue ALWAYS produces "❌ Commit generation failed." (the
// §18.3 BASE form). B.5's "(interrupted)" is the signal handler's (P1.M4.T2) render. Do NOT add a
// param/reason/second function. (§5)

// CRITICAL (candidate note — AFTER closing sep, ONE blank line, iff candidateMsg != ""): append
//   "\n\n" + `A candidate message was produced but rejected: "` + candidateMsg + `". You can use it
//   manually in the command above.`
// (literal " quotes around candidateMsg). Empty/"" candidateMsg ⇒ boxed message only. (§6)

// CRITICAL (NO trailing newline): return WITHOUT a final '\n' (the CLI's Fprintln adds it, §7). The
// internal blank lines (line 5, line 8, and the candidate-note "\n\n") ARE in the return value.

// GOTCHA (NO second package doc): dedupe.go (P1.M3.T2.S1) owns the `// Package generate` comment.
// rescue.go's first line is `package generate` with NO preceding package doc — function-level doc
// comments only. Else revive/golint flags a duplicate package comment. (§8)

// GOTCHA (imports: "strings" ONLY): rescue.go needs strings.Builder + WriteString/WriteByte. Build the
// command with a conditional splice (if parentSHA != "" { WriteString(" -p "); WriteString(parentSHA) })
// — NOT fmt.Sprintf (would need the "fmt" import). rescue_test.go: "strings" + "testing". NO "fmt", NO
// internal/*, NO third-party. go mod tidy MUST be a no-op. internal/generate stays a stdlib-only LEAF. (§8)

// GOTCHA (in-package tests, NEW file): rescue_test.go is `package generate` (white-box). Mirror
// payload_test.go's canonical-exact style (independently-derived want, got != want, %q diff) for the 4
// cases + parse_test.go/dedupe_test.go's table style for TestFormatRescue_Properties. NO subprocess,
// NO temp repo. (§9)

// GOTCHA (do NOT pre-empt the orchestrator/CLI): this subtask owns ONLY FormatRescue. The rescue
// condition (FR43), the §18.2 failure-mode→rescue mapping, exit 3, printing, and the SIGINT/SIGTERM
// handler are P1.M3.T4 / P1.M4.T1 / P1.M4.T3.S3 / P1.M4.T2. Do not implement them here. (§0/§10)
```

## Implementation Blueprint

### Data models and structure

```go
// internal/generate/rescue.go — NO data models. One pure function + a couple of verbatim string
// constants. No structs, no interfaces. (The orchestrator P1.M3.T4 may define a result/error type
// later; rescue needs none.)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/generate/rescue.go — FormatRescue
  - FILE: NEW internal/generate/rescue.go. PACKAGE: `package generate` (first line — NO preceding
      `// Package generate` comment; dedupe.go owns the package doc). IMPORT: EXACTLY `"strings"`
      (strings.Builder). NO "fmt", NO internal/*, NO third-party.
  - DEFINE the 60-dash separator as a named `const` (used twice — lines 2 + 10; DRY + the count is
      pinned once): `const rescueSep = "<60 hyphens>"`. Add a one-line comment citing §18.3 +
      prd_snapshot.md 1178/1186 + the tr-count verification. (A test asserts len(rescueSep)==60.)
  - IMPLEMENT exported `func FormatRescue(treeSHA, parentSHA, candidateMsg string) string`:
      - `var b strings.Builder`.
      - Write the 10-line §18.3 boxed block via WriteString/WriteByte:
          1. `"❌ Commit generation failed.\n"`  (literal ❌ = U+274C)
          2. rescueSep + "\n"
          3. `"Your staged files were safely snapshotted before generation.\n"`
          4. `"Tree ID: "` + treeSHA + "\n"
          5. `"\n"`  (blank line)
          6. `"To commit the originally staged files manually:\n"`
          7. the COMMAND (see splice below) + "\n"
          8. `"\n"`  (blank line)
          9. `"(omit \"-p <PARENT_SHA>\" if this is the repository's first commit)\n"`
          10. rescueSep  (NO trailing "\n" — §7)
      - COMMAND splice (line 7): WriteString(`  git commit-tree`); if parentSHA != "" { WriteString("
        -p "); WriteString(parentSHA) }; WriteString(` -m "Your message" `); WriteString(treeSHA);
        WriteString(" | xargs git update-ref HEAD"). (2 leading spaces in the prefix; literal " quotes
        around Your message; root omits the whole " -p <parentSHA>" segment — §4.)
      - CANDIDATE note (§6): if candidateMsg != "" { WriteString("\n\n"); WriteString(`A candidate
        message was produced but rejected: "`); WriteString(candidateMsg); WriteString(`". You can use
        it manually in the command above.`) }. (Appended AFTER the closing separator; one blank line;
        literal " quotes around candidateMsg; no trailing newline.)
      - return b.String().
      - GOTCHA: the command's `-m "Your message"` uses literal ASCII double-quotes — split the segment
        around the dynamic SHAs (WriteString the quoted parts as separate literals) OR escape them as
        \" inside a double-quoted Go literal. Either is correct; the split form is clearest.
  - DOC COMMENT on FormatRescue: cite PRD §18.3 / §9.10 FR44 + the commit-pi rescue provenance; note
      the root-commit -p omission (mirrors git.CommitTree, FR39); note the candidate-note gate; note
      NO trailing newline (CLI adds it); note the FROZEN-signature + downstream consumers (orchestrator
      P1.M3.T4 → CLI P1.M4.T1 → exit 3 P1.M4.T3.S3). Mirror dedupe.go's doc-comment density.

Task 2: CREATE internal/generate/rescue_test.go — 4 canonical-exact + properties table
  - FILE: NEW internal/generate/rescue_test.go. PACKAGE: `package generate` (white-box). IMPORT:
      "strings" (Repeat for the independent want + Count/HasSuffix/Contains asserts), "testing".
  - IMPLEMENT TestFormatRescue_RootedNoCandidate — independently-derived `want` (string concat +
      strings.Repeat("-", 60); the literal ❌; real SHAs 9f3a1c/abc1234; command with `-p abc1234`;
      the hint line; NO candidate note; NO trailing newline). `got != want` → t.Errorf with the
      `--- got ---\n%q\n--- want ---\n%q` form (mirror payload_test.go).
  - IMPLEMENT TestFormatRescue_RootlessNoCandidate — parentSHA=""; command WITHOUT -p; hint line STILL
      present; otherwise identical structure. Independently-derived `want`.
  - IMPLEMENT TestFormatRescue_RootedWithCandidate — rooted message + "\n\n" + the candidate note with
      "feat: add bar" in literal quotes. Independently-derived `want`.
  - IMPLEMENT TestFormatRescue_RootlessWithCandidate — root message + candidate note.
  - IMPLEMENT TestFormatRescue_Properties — a `cases := []struct{...}` table (name, treeSHA, parentSHA,
      candidateMsg) + `for _, tc := range cases { t.Run(tc.name, …) }` loop asserting the structural
      invariants: contains ❌; exactly 2 separator lines each 60 dashes (split on "\n", count lines
      == rescueSep); command line present with 2 leading spaces; command contains treeSHA; command
      contains parentSHA AND "-p " iff parentSHA != ""; "Tree ID: <treeSHA>" present; hint line
      present ALWAYS; candidate note present iff candidateMsg != "" (and contains "<candidateMsg>"
      with literal quotes when present); output does NOT contain "interrupted"; !HasSuffix(got, "\n").
  - ADD defensive cases to the properties table: empty treeSHA (no panic — produces a message with
      empty SHAs), candidateMsg containing a quote/newline (appended VERBATIM — quotes NOT escaped).
  - PATTERN: mirror payload_test.go (canonical-exact) + parse_test.go/dedupe_test.go (table). NO
      subprocess, NO temp repo.
  - GOTCHA: the `want` MUST be independently derived (concat + strings.Repeat("-", 60), the literal ❌)
      — do NOT reference rescue.go's rescueSep constant or call FormatRescue to build want, else the
      test is circular. The ❌ in the test literal must match the ❌ in rescue.go (both U+274C).

Task 3: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. Every other file
      (internal/generate/dedupe.go + dedupe_test.go [P1.M3.T2.S1's], internal/prompt/*, internal/git/*,
      internal/provider/*, internal/config/*, cmd/*, pkg/*, Makefile) MUST be byte-unchanged.
      `go test -race ./internal/generate/` MUST be green (rescue + dedupe suites together). `go vet`
      + `golangci-lint` (if available) MUST be clean — esp. NO duplicate-package-comment warning.
```

### Implementation Patterns & Key Details

```go
// FormatRescue — PRD §18.3 / §9.10 FR44: assemble the rescue message byte-for-byte. Pure: 3 strings
// in, 1 string out, no I/O, no error. Mirrors prompt.BuildUserPayload's Builder + no-trailing-newline
// style. The command line is the ONLY fully-dynamic line; the "(omit -p)" hint (§18.3 line 9) stays
// verbatim in both cases (design-decisions §3).
func FormatRescue(treeSHA, parentSHA, candidateMsg string) string {
	var b strings.Builder

	// Line 1 — failure notice. ❌ = U+274C (NON-ASCII, §18.3 line 1177). NOT "(interrupted)" (§5).
	b.WriteString("❌ Commit generation failed.\n")
	// Line 2 — separator (exactly 60 '-', verified prd_snapshot.md 1178).
	b.WriteString(rescueSep)
	b.WriteByte('\n')
	// Line 3 — reassurance.
	b.WriteString("Your staged files were safely snapshotted before generation.\n")
	// Line 4 — Tree ID (substitute treeSHA).
	b.WriteString("Tree ID: ")
	b.WriteString(treeSHA)
	b.WriteByte('\n')
	// Line 5 — blank.
	b.WriteByte('\n')
	// Line 6 — manual-recovery header.
	b.WriteString("To commit the originally staged files manually:\n")
	// Line 7 — the command (2 leading spaces; -p <parentSHA> omitted when root — mirrors git.CommitTree).
	b.WriteString("  git commit-tree")
	if parentSHA != "" {
		b.WriteString(" -p ")
		b.WriteString(parentSHA)
	}
	b.WriteString(` -m "Your message" `) // literal ASCII " around Your message (shell template)
	b.WriteString(treeSHA)
	b.WriteString(" | xargs git update-ref HEAD")
	b.WriteByte('\n')
	// Line 8 — blank.
	b.WriteByte('\n')
	// Line 9 — first-commit hint (kept ALWAYS — §18.3 verbatim, design-decisions §3).
	b.WriteString(`(omit "-p <PARENT_SHA>" if this is the repository's first commit)`)
	b.WriteByte('\n')
	// Line 10 — closing separator (NO trailing newline — the CLI's Fprintln adds it, §7).
	b.WriteString(rescueSep)

	// Candidate note (§18.3 last paragraph / line 1189): appended AFTER the closing separator with ONE
	// blank line, iff a candidate message was produced but rejected (duplicate-exhaustion / parse).
	if candidateMsg != "" {
		b.WriteString("\n\nA candidate message was produced but rejected: \"")
		b.WriteString(candidateMsg)
		b.WriteString("\". You can use it manually in the command above.")
	}

	return b.String()
}

// rescueSep is the §18.3 separator line: exactly 60 hyphens (verified against prd_snapshot.md lines
// 1178 + 1186: `sed -n '<n>p' … | tr -cd '-' | wc -c` == 60 on both). Used for both the top (line 2)
// and bottom (line 10) separators. A literal (not strings.Repeat) for byte-for-byte §18.3 fidelity —
// mirrors prompt/payload.go's verbatim-constant philosophy; a test pins len(rescueSep)==60.
const rescueSep = "------------------------------------------------------------" // 60 × '-'
```

```go
// rescue_test.go — independently-derived `want` (do NOT reference rescueSep or call FormatRescue).
func TestFormatRescue_RootedNoCandidate(t *testing.T) {
	treeSHA, parentSHA := "9f3a1c", "abc1234"
	sep := strings.Repeat("-", 60)
	want := "❌ Commit generation failed.\n" +
		sep + "\n" +
		"Your staged files were safely snapshotted before generation.\n" +
		"Tree ID: " + treeSHA + "\n" +
		"\n" +
		"To commit the originally staged files manually:\n" +
		"  git commit-tree -p " + parentSHA + ` -m "Your message" ` + treeSHA + " | xargs git update-ref HEAD\n" +
		"\n" +
		`(omit "-p <PARENT_SHA>" if this is the repository's first commit)` + "\n" +
		sep // NO trailing newline
	got := FormatRescue(treeSHA, parentSHA, "")
	if got != want {
		t.Errorf("FormatRescue rooted/no-candidate mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}
// (TestFormatRescue_RootlessNoCandidate: same but command = `  git commit-tree -m "Your message" ` +
//  treeSHA + ` | xargs git update-ref HEAD` — NO "-p"; hint line STILL present.)
// (TestFormatRescue_RootedWithCandidate / RootlessWithCandidate: append "\n\n" + the candidate note.)
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. rescue.go uses stdlib strings ONLY. `go mod tidy` MUST be a no-op.
        `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - rescue.go → (stdlib: strings) ONLY. It does NOT import fmt, internal/git, internal/provider,
        internal/prompt, internal/config, os, or any third-party. internal/generate remains a
        stdlib-only LEAF (alongside dedupe.go).
  - rescue_test.go → (stdlib: strings, testing) ONLY. In-package (package generate).

UPSTREAM CONTRACT (the inputs — do NOT implement, just consume):
  - P1.M1.T2.S3 git.WriteTree(ctx) (sha, error): the snapshot tree SHA. This IS the `treeSHA` param.
        Non-empty when rescue is reachable (FR43: TREE_SHA set). A pure SHA string (git's %H).
  - P1.M1.T2.S2 git.RevParseHEAD(ctx) (sha string, isUnborn bool, err error): `sha` if isUnborn==false,
        `""` if isUnborn==true (FINDING 1: unborn by exit 128, NOT string emptiness). This IS the
        `parentSHA` param — "" ⇒ root commit ⇒ command omits -p (§4).
  - P1.M2.T6.S1 provider.ParseOutput(raw, m Manifest) (msg string, ok bool, fellback bool): the
        parsed message IF one was produced before the duplicate/parse rejection. This IS the
        `candidateMsg` param (or "" if none). Arrives trimmed + newline-normalized (Step 4/5).

DOWNSTREAM CONTRACTS (the consumers — do NOT implement here, just honor the signature):
  - P1.M3.T4 (orchestrator CommitStaged): on the rescue path (FR43: treeSHA != "" && newSHA == "") —
        `msg := generate.FormatRescue(treeSHA, parentSHA, candidateMsg)`; hands `msg` to the CLI layer.
        The candidateMsg is the last-parsed message for duplicate/parse rescues; "" for timeout/interrupt.
  - P1.M4.T1 (CLI default action): `fmt.Fprintln(os.Stderr, msg)` (Println adds the trailing newline
        FormatRescue deliberately omits, §7).
  - P1.M4.T3.S3 (exit codes): exit 3 for the rescue path.
  - P1.M4.T2 (signal handler): produces the "(interrupted)" variant (§5) by its own means (post-process
        or compose) — NOT FormatRescue's job.
  - P1.M3.T5 (public API pkg/stagecoach): may re-export FormatRescue.
  => The FormatRescue(treeSHA, parentSHA, candidateMsg string) string signature is FROZEN after this subtask.

FROZEN FILES (do NOT edit):
  - internal/generate/dedupe.go + dedupe_test.go (P1.M3.T2.S1's, parallel — owns the package doc),
        internal/prompt/* (payload.go/payload_test.go are the read-only refs), internal/git/*,
        internal/provider/*, internal/config/*, cmd/stagecoach/main.go, pkg/*, Makefile, go.mod, go.sum.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the two new files
gofmt -w internal/generate/rescue.go internal/generate/rescue_test.go

# Vet the generate package (rescue.go + rescue_test.go + dedupe.go + dedupe_test.go)
go vet ./internal/generate/

# Lint (if available) — MUST be clean, esp. NO duplicate-package-comment warning
golangci-lint run ./internal/generate/ 2>/dev/null || echo "(golangci-lint not available — skip)"

# Confirm rescue.go imports EXACTLY "strings" (no fmt, no internal/*, no third-party)
sed -n '/^import/,/^)/p' internal/generate/rescue.go   # → a single "strings" import

# Confirm rescue.go has NO "// Package generate" comment (dedupe.go owns the package doc)
grep -n '^// Package generate' internal/generate/rescue.go && echo "FAIL: second package doc" || echo "OK: no dup package doc"

# Confirm the ❌ (U+274C) is present and ASCII stand-ins are NOT
grep -n '❌' internal/generate/rescue.go   # → the failure-notice line
! grep -n '\[X\]\|failed (interrupted)' internal/generate/rescue.go || echo "FAIL: ASCII stand-in or interrupted variant"

# Confirm go.mod/go.sum are unchanged (stdlib only)
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged ✓"

# Expected: Zero errors. `go vet` clean. The ❌ present as a literal UTF-8 rune.
```

### Level 2: Unit Tests (THE KEYSTONE — 4 canonical-exact + properties table)

```bash
# Run the NEW suite verbosely
go test -race -v ./internal/generate/ -run 'TestFormatRescue'

# Full generate package (rescue + dedupe together — confirms no collision with P1.M3.T2.S1)
go test -race ./internal/generate/

# Coverage of the new file specifically
go test -coverprofile=coverage.out ./internal/generate/ && go tool cover -func=coverage.out | grep -E 'rescue.go|FormatRescue'

# Expected: All pass. The rooted-vs-rootless command difference (-p present iff parentSHA != "") and the
# candidate-note gate (present iff candidateMsg != "") are the load-bearing assertions — if either fails,
# FormatRescue is mis-assembling the command or the note. The "no trailing newline" + "no interrupted"
# invariants are the anti-regression guards.
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-module build (rescue is internal; confirms nothing else broke, incl. P1.M3.T2.S1's dedupe)
go build ./...

# Optional sanity: print a sample rescue to eyeball the §18.3 layout (NOT a test — visual check)
# (The implementing agent may run a one-off `go run` snippet; no permanent file needed.)

# Expected: `go build ./...` succeeds; the suite prints PASS. FormatRescue has no runtime/endpoint/DB/git
# surface — it is one pure function — so there is no service to start, no curl, no repo to check.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Not applicable for this subtask (pure function, no I/O, no network, no DB, no UI, no git, no signal).
# The strongest creative check is human review:
#   1. Diff the FormatRescue output against PRD §18.3 (prd_snapshot.md 1177–1186) line-by-line — every
#      line, the separators (60 dashes), the 2 leading spaces on the command, the ❌.
#   2. Confirm the rooted command == B.5's `git commit-tree -p abc1234 -m "Your message" 9f3a1c... |
#      xargs git update-ref HEAD` (modulo the SHA values + the "(interrupted)" first line, which is
#      P1.M4.T2's concern — §5).
#   3. Confirm the rootless command omits -p entirely (FR39: "else git commit-tree -m …").
#   4. Confirm the candidate note matches §18.3 line 1189 verbatim (with the message in literal quotes).
#   5. Trace the downstream wiring (Integration Points) matches what P1.M3.T4 / P1.M4.T1 will call:
#        msg := generate.FormatRescue(treeSHA, parentSHA, candidateMsg)
#        fmt.Fprintln(os.Stderr, msg)   // CLI P1.M4.T1; Println adds the trailing newline
#        os.Exit(3)                      // exit-code system P1.M4.T3.S3
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed successfully.
- [ ] All tests pass: `go test -race ./internal/generate/` (rescue + dedupe suites).
- [ ] `go build ./...` succeeds.
- [ ] No vet errors: `go vet ./internal/generate/`.
- [ ] No formatting issues: `gofmt -l internal/generate/` (empty output).
- [ ] No lint warnings (incl. duplicate-package-comment): `golangci-lint run ./internal/generate/` (if available).
- [ ] go.mod/go.sum byte-unchanged: `git diff --exit-code go.mod go.sum`.

### Feature Validation

- [ ] `FormatRescue` returns the §18.3 message byte-for-byte for the rooted/no-candidate case (10 lines; ❌; 2× 60-dash sep; 2 leading spaces on the command; `-p <parentSHA>` present; hint line present; no trailing newline).
- [ ] Root case (`parentSHA == ""`): command omits the `-p <parentSHA>` segment ENTIRELY (no `-p` substring); hint line STILL present; otherwise identical structure.
- [ ] Candidate note appended iff `candidateMsg != ""`, AFTER the closing separator with ONE blank line, with the message in literal `"` quotes, exact §18.3 text.
- [ ] Output NEVER ends with `\n` and NEVER contains "interrupted" (the §18.3 BASE form only — §5).
- [ ] Scope respected: NO printing, NO exit 3, NO rescue-condition detection, NO signal handler, NO "(interrupted)" variant (those are P1.M4.T1 / P1.M4.T3.S3 / P1.M3.T4 / P1.M4.T2).
- [ ] Manual diff against PRD §18.3 (prd_snapshot.md 1177–1186) + the candidate note (line 1189) succeeds.

### Code Quality Validation

- [ ] Follows existing codebase patterns: exported pure function + Builder assembly (mirror `prompt.BuildUserPayload`), canonical-exact + table tests (mirror `payload_test.go` + `parse_test.go`/`dedupe_test.go`).
- [ ] File placement matches the desired codebase tree (`internal/generate/rescue.go` + `rescue_test.go`).
- [ ] Anti-patterns avoided (see Anti-Patterns section).
- [ ] Imports properly managed: `"strings"` only in rescue.go; no new dependency.
- [ ] NO second `// Package generate` doc comment (function-level doc comments only).
- [ ] Doc comments cite PRD §18.3 / §9.10 FR44 + the commit-pi rescue provenance + the FROZEN-signature.

### Documentation & Deployment

- [ ] Code is self-documenting with clear function/variable names + PRD-cited doc comments.
- [ ] No new environment variables (none needed — pure function).
- [ ] No new config keys (none needed — the formatter takes its inputs as params).

---

## Anti-Patterns to Avoid

- ❌ Don't print, exit, detect the rescue condition, or install a signal handler here — FormatRescue is a PURE string assembler; those are P1.M4.T1 / P1.M4.T3.S3 / P1.M3.T4 / P1.M4.T2.
- ❌ Don't drop the `(omit "-p <PARENT_SHA>" …)` hint line for the root case — §18.3 keeps it; the CONTRACT's only dynamic modification is the command's -p omission (§3). Keep it in BOTH cases.
- ❌ Don't produce the "(interrupted)" variant — §18.3's base form (`❌ Commit generation failed.`) only; the signal handler (P1.M4.T2) owns "(interrupted)" (§5). Don't add a param/reason/second function.
- ❌ Don't emit the literal `<TREE_SHA>`/`<PARENT_SHA>` tokens — they are §18.3 structural annotations; substitute the runtime args (like payload.go substitutes `diff`).
- ❌ Don't replace ❌ (U+274C) with an ASCII stand-in (`[X]`, `x`, `!`) — it's the literal §18.3 emoji; a test pins the UTF-8 bytes.
- ❌ Don't get the separator dash count wrong — EXACTLY 60 (verified). Don't add spaces inside it.
- ❌ Don't forget the command's 2 leading spaces, and don't blank the `-p <parentSHA>` for root — OMIT the whole segment (no `-p` substring).
- ❌ Don't append the candidate note INSIDE the box or without the blank-line separator — it goes AFTER the closing separator with ONE blank line, iff candidateMsg != "" (§6).
- ❌ Don't add a trailing `\n` to the return value — the CLI's Fprintln adds it (§7).
- ❌ Don't import `fmt`/`internal/*`/third-party — `"strings"` only; build the command with a conditional Builder splice (not Sprintf). `go mod tidy` must be a no-op.
- ❌ Don't add a second `// Package generate` doc comment — dedupe.go (P1.M3.T2.S1) owns it; rescue.go uses function-level doc comments only.
- ❌ Don't edit `internal/generate/dedupe.go`/`dedupe_test.go` (P1.M3.T2.S1's, parallel) or any other frozen file.
- ❌ Don't make the test circular — derive `want` independently (concat + `strings.Repeat("-", 60)` + the literal ❌), NOT by referencing `rescueSep` or calling `FormatRescue`.
