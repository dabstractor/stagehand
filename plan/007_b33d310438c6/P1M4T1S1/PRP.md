---
name: "P1.M4.T1.S1 — estimateTokens (chars/4) shared utility (PRD §9.1 FR3d/FR3i): the SINGLE model-agnostic token estimator consumed by S2 (prompt-reserve) and M4.T2 (water-fill sizing)"
description: |

  Add the single model-agnostic token estimator that the FR3d token-budget overlay and the FR3i water-fill
  truncation both measure with. Stagecoach never loads a tokenizer (it shells out to an arbitrary agent CLI),
  so it uses the standard `~4 chars ≈ 1 token` heuristic (PRD §9.1 FR3d; git_diff_semantics.md §5). This
  subtask delivers the ONE shared estimator as a pure, allocation-free Go function; S2 (P1.M4.T1.S2) and
  M4.T2 (P1.M4.T2.S1/S2) both call it, so the budget arithmetic is internally consistent (no second formula).

  CONTRACT (item_description §3, verbatim): `func estimateTokens(s string) int` returning
  `len([]rune(s)) / 4` ROUNDED UP (ceiling division; rune count, NOT byte count, so multi-byte UTF-8 does
  not over-count). Add `func estimateTokensBytes(b []byte) int` if helpful. Table tests: empty⇒0, 4 ASCII
  chars⇒1, 8⇒2, a 4-rune CJK string⇒1 (rune-based). This is the SINGLE estimator used by S2 and M4.T2 — do
  NOT introduce a second formula.

  DELIVERABLES (2 NEW files; nothing edited; go.mod unchanged):
    1. CREATE `internal/git/tokens.go`      — `package git`. EXPORTED `func EstimateTokens(s string) int`
       + `func EstimateTokensBytes(b []byte) int`, both `ceilDiv(utf8.RuneCount…, 4)`. Import: `unicode/utf8` only.
    2. CREATE `internal/git/tokens_test.go` — `package git` (white-box). Pure table tests mirroring
       `binary_test.go`'s `TestIsBinaryByExtension`.

  SCOPE NOTE (placement + export, design §0): the item says "internal/git (or a small internal/util)".
  There is NO `internal/util`; every consumer of S2/M4.T2 (generate, decompose, cmd, pkg/stagecoach) ALREADY
  imports `internal/git`, and `internal/prompt` does NOT (so no neutral-package need). The new file is
  `internal/git/tokens.go`. The item writes the name lowercase as pseudocode, but S2 sets
  `StagedDiffOptions.PromptReserveTokens` from CROSS-PACKAGE call sites (the field exists at git.go:72; "the
  git layer RECEIVES this, it does not compute it"), so the function MUST be EXPORTED: `EstimateTokens`.

  SCOPE NOTE (the formula, design §2 — do NOT change it): the contract + FR3d specify `chars/4`.
  `architecture/git_diff_semantics.md §5` recommends `chars/3` for budget ceilings (code is ~3 chars/token).
  These are RECONCILED by the SEPARATE `margin` in FR3d/FR3i (`body_budget = token_limit − skeleton −
  prompt − margin`) — the margin is the actual safety buffer; the estimator is just the consistent measure.
  This subtask delivers ONLY the estimator (chars/4). Do NOT "improve" it to chars/3 — that would diverge
  from the contract and desynchronize S2/M4.T2. Implement chars/4, full stop.

  SCOPE BOUNDARY (what this does NOT do): NO prompt-reserve measurement (S2 — P1.M4.T1.S2); NO water-fill
  solver/truncation (M4.T2.S1/S2); NO token-limit gate (M4.T3); NO config/CLI/API/doc surface; NO edits to
  any existing file. This is a pure leaf utility + its tests — the dependency-free foundation for the rest
  of M4.T1/M4.T2.

  INPUT: none (pure utility — takes a string or []byte, returns an int). OUTPUT: `EstimateTokens(s) int`,
  consumed by S2 (P1.M4.T1.S2, prompt reserve) and M4.T2.S1/S2 (water-fill sizing + truncation arithmetic).

  ⚠️ Use CEILING division `(n+3)/4` (not truncating `n/4`) — a 1-rune string is 1 token, a 5-char string is 2.
  ⚠️ Count RUNES (`utf8.RuneCountInString`/`utf8.RuneCount`), NOT bytes — a 4-rune CJK string is 1 token, not 3.
  ⚠️ EXPORT the function (`EstimateTokens`) — S2 measures PromptReserveTokens from cross-package call sites.
  ⚠️ ONE formula only — no chars/3 variant, no line-based variant, no per-provider hook. (§4)

  Deliverable: 2 NEW files; `go build ./... && go test ./...` green; go.mod/go.sum unchanged.

---

## Goal

**Feature Goal**: Land the single, shared, model-agnostic token estimator (`ceil(runes/4)`) that the FR3d
token-budget overlay and the FR3i water-fill truncation both measure with — a pure, allocation-free Go
function so that S2's prompt-reserve measurement and M4.T2's per-file sizing/level-solver use ONE consistent
formula (the budget arithmetic is internally coherent: a "token" measured upstream is a "token" measured
downstream). PRD §9.1 FR3d ("~4 chars ≈ 1 token estimate"); git_diff_semantics.md §5.

**Deliverable** (2 NEW files; go.mod unchanged):
1. `internal/git/tokens.go` — `package git`, import `unicode/utf8` only. EXPORTED
   `func EstimateTokens(s string) int` + `func EstimateTokensBytes(b []byte) int`, both
   `ceilDiv(utf8.RuneCount…(s), 4)` where `ceilDiv(n, d int) int { return (n+d-1)/d }`.
2. `internal/git/tokens_test.go` — `package git` (white-box). Pure table tests (no git, no I/O) covering
   the contract's table + the UTF-8/ceiling edge cases.

**Success Definition**: `EstimateTokens("")==0`; `EstimateTokens("abcd")==1`; `EstimateTokens(8 chars)==2`;
`EstimateTokens("你好世界")==1` (4 runes, 12 bytes — rune-based, NOT 3); `EstimateTokens("a")==1` (ceiling);
`EstimateTokensBytes([]byte("abcd"))==EstimateTokens("abcd")`. `go build ./... && go vet ./... &&
go test ./...` GREEN; `gofmt -l` clean; go.mod/go.sum byte-unchanged; only the 2 new files differ.

## User Persona

**Target User**: The downstream subtasks (S2 = P1.M4.T1.S2 prompt-reserve; M4.T2 = P1.M4.T2.S1/S2 water-fill
sizing + truncation). Transitively: every user who sets `token_limit` (PRD §9.1 FR3d) so a large diff fits
their model's context window without stagecoach maintaining a per-model tokenizer registry.

**Use Case**: S2 calls `EstimateTokens(systemPrompt + styleExamples)` to set
`StagedDiffOptions.PromptReserveTokens` at each of the 6 diff call sites; M4.T2 calls `EstimateTokens` to
size each file's diff body and to solve the water-fill level `L`. Both use the SAME formula, so
`body_budget = token_limit − skeleton − prompt − margin` is measured in consistent units.

**User Journey**: (internal) a call site measures a string → `EstimateTokens(s)` → an int token count →
threaded into the budget arithmetic. No git, no I/O, no tokenizer — pure arithmetic.

**Pain Points Addressed**: Two subtasks measuring "tokens" with two different formulas would make the budget
incoherent (the prompt reserve subtracted in one unit wouldn't match the body sizes summed in another). ONE
shared estimator prevents that drift.

## Why

- **It IS the foundation for the FR3d/FR3i token budget.** S2 and M4.T2 both need to measure strings in
  tokens; this is the single function they call. Landing it first (M4.T1.S1, before S2 and M4.T2) means
  both consumers import a stable, tested API.
- **Honors the model-agnostic design (PRD N2).** Stagecoach never loads a tokenizer; `chars/4` is the
  standard, documented heuristic when you don't own the tokenizer (git_diff_semantics.md §5). One function,
  one formula — no per-model registry, ever.
- **Minimal blast radius.** 2 NEW files, pure stdlib, edits nothing. No risk to any existing test or
  behavior. go.mod unchanged.

## What

A new `tokens.go` with two exported one-liners over a ceiling-division helper (all rune-based via
`unicode/utf8`), and a new `tokens_test.go` with a table test. No new deps, no config, no API beyond the two
functions, no CLI, no docs, no edits to existing files.

### Success Criteria

- [ ] `internal/git/tokens.go` exists, `package git`, imports EXACTLY `unicode/utf8` (nothing else needed;
      no `fmt`, no `strings`).
- [ ] EXPORTED `func EstimateTokens(s string) int` returns `ceilDiv(utf8.RuneCountInString(s), 4)`.
- [ ] EXPORTED `func EstimateTokensBytes(b []byte) int` returns `ceilDiv(utf8.RuneCount(b), 4)`.
- [ ] `ceilDiv(n, d int) int` is `(n+d-1)/d` (UNEXPORTED helper) — yields 0 for n=0, ceiling for n>0.
- [ ] `EstimateTokens("")==0`; `("abcd")==1`; `(8 chars)==2`; `("a")==1`; `("abcde")==2` (ceilDiv(5,4)=2);
      `("你好世界")==1` (4 runes, 12 bytes — rune-based); `EstimateTokensBytes([]byte("abcd"))==1`.
- [ ] `internal/git/tokens_test.go` exists, `package git`, imports `testing` (+ stdlib), and is a pure table
      test (no `initRepo`, no I/O).
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/git/tokens*.go` clean;
      go.mod/go.sum byte-unchanged; ONLY the 2 new files differ (`git status` clean except them).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact formula
(`ceilDiv(utf8.RuneCountInString(s),4)`), the export decision (§0), the test table (§5, each case spelled
out), the "do not change to chars/3" note (§2), and the file paths. No git/diff/decompose/render knowledge
required — this is a pure arithmetic utility.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions
- docfile: plan/007_b33d310438c6/P1M4T1S1/research/design-decisions.md
  why: the 6 decisions. §0 (placement internal/git/tokens.go; EXPORTED — cross-package S2 consumers),
       §1 (the formula + ceiling/rune math + the table verification), §2 (chars/4 contract vs chars/3 arch
       doc — the margin reconciles; do NOT change the formula), §3 (rune-based not byte-based), §4 (SINGLE
       estimator — no second formula), §5 (test file + table mirroring TestIsBinaryByExtension), §6 (no
       conflict with parallel skeleton.go).
  critical: §2 (do NOT switch to chars/3 — would diverge from the contract + desync S2/M4.T2), §0 (EXPORT
       the function — S2 is cross-package), §1/§3 (ceiling + rune-count — the test table pins both).

# MUST READ — the heuristic source + the chars/3 ceiling caveat (the §2 tension)
- docfile: plan/007_b33d310438c6/architecture/git_diff_semantics.md
  section: "## 5. Token estimation (`~4 chars ≈ 1 token`)".
  why: documents that chars/4 is the standard model-agnostic estimate (Stagecoach never loads a tokenizer),
       AND that code is ~3 chars/token (the arch doc's chars/3 ceiling recommendation). The contract
       (FR3d) picks chars/4; the FR3d/FR3i `margin` is the safety buffer (M4.T2), not the estimator ratio.
  critical: implement chars/4 (the contract); the arch doc's chars/3 is reconciled by the margin, NOT by
       changing this formula. (research §2.)

# MUST READ — the contract formula in the PRD (the authoritative spec)
- file: PRD.md (or plan/007_b33d310438c6/prd_snapshot.md)
  section: "9.1 Diff capture" — FR3d ("truncates the diff to fit using the ~4 chars ≈ 1 token estimate with
           a safety margin") + FR3i ("body_budget = token_limit − skeleton − prompt − margin"; per-file body
           size as a token estimate).
  why: FR3d PINS the "~4 chars ≈ 1 token estimate" (chars/4, the contract) and shows the `margin` is the
       separate safety term. FR3i confirms the estimator is used for per-file sizing + the level solver.
  critical: chars/4 is the spec; the margin (M4.T2) is the safety. Do not conflate them.

# MUST READ — the consumer seam (S2 sets this field; confirms EstimateTokens is cross-package ⇒ EXPORTED)
- file: internal/git/git.go   (READ ONLY — do NOT edit)
  section: `StagedDiffOptions` — fields `TokenLimit int`, `DiffContext int`, `PromptReserveTokens int`
           (around line 48-72). The PromptReserveTokens doc (line ~70-72): "when TokenLimit > 0. The git
           layer RECEIVES this (it does not compute it — keeps internal/git …)". Added by P1.M1.T2 (Complete).
  why: confirms (a) the field S2 will SET exists, (b) it is measured UPSTREAM (at the cross-package call
       sites in generate/decompose), so EstimateTokens MUST be exported, and (c) internal/git itself does
       not compute the reserve — S2 does, calling this estimator.
  critical: do NOT edit git.go. EstimateTokens is consumed OUTSIDE internal/git ⇒ capital E.

# MUST READ — the pure-helper-file + table-test pattern to mirror
- file: internal/git/binary.go + internal/git/binary_test.go   (READ ONLY — do NOT edit)
  section: binary.go's pure helpers (`isBinaryByExtension`, `binaryPlaceholderLine`) + binary_test.go's
           `TestIsBinaryByExtension` (a `cases := []struct{…}{…}` table with a loop + one `t.Errorf`/case).
  why: the SHAPE to mirror — a pure-helper file in `package git` + a white-box table test in the matching
       `*_test.go`. `tokens.go` is to `tokens_test.go` as `binary.go` is to `binary_test.go`.
  critical: mirror the table-test idiom (named cases, one t.Errorf/case). Do NOT edit binary.go/_test.go.

# The import graph (proves placement + no cycle — from the research grep)
- note: "every S2 call-site package already imports internal/git (generate/generate.go, decompose/*,
         pkg/stagecoach, cmd/*); internal/prompt does NOT import internal/git. ⇒ an EXPORTED EstimateTokens
         in internal/git is reachable by all consumers with no new edge and no cycle."

- url: (PRD §9.1 FR3d/FR3i — already in context as selected_prd_content `h3.17`; the "~4 chars ≈ 1 token
       estimate with a safety margin" + "body_budget = token_limit − skeleton − prompt − margin".)
  why: the AUTHORITATIVE contract for both the ratio (chars/4) and the safety mechanism (margin). This
       subtask implements the ratio; M4.T2 implements the margin.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go              # P1.M1.T2 — StagedDiffOptions{TokenLimit, DiffContext, PromptReserveTokens}. READ ONLY.
  binary.go           # pure-helper file precedent (isBinaryByExtension, …). READ ONLY (style mirror).
  binary_test.go      # TestIsBinaryByExtension table-test precedent. READ ONLY (style mirror).
  numstat.go          # P1.M3.T1.S1 (numstatRow/numstatRows). READ ONLY. (skeleton.go is the parallel S2's.)
  tokens.go           # *** CREATE *** — EstimateTokens + EstimateTokensBytes + ceilDiv.
  tokens_test.go      # *** CREATE *** — pure table tests.
go.mod / go.sum       # UNCHANGED (stdlib unicode/utf8 only — no new module deps).
```

### Desired Codebase tree with files to be added

```bash
internal/git/tokens.go        # NEW — EstimateTokens(s string) int + EstimateTokensBytes(b []byte) int + ceilDiv.
internal/git/tokens_test.go   # NEW — TestEstimateTokens + TestEstimateTokensBytes table tests (pure, no git).
# go.mod/go.sum UNCHANGED. NO other file changes.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (chars/4 is the contract; do NOT switch to chars/3, design §2): architecture/git_diff_semantics.md
//   §5 recommends chars/3 for budget ceilings (code ~3 chars/token), but PRD FR3d pins "~4 chars ≈ 1 token
//   estimate WITH A SAFETY MARGIN". The margin (FR3d/FR3i, M4.T2) is the safety buffer; the estimator is the
//   consistent measure. Switching to chars/3 here would diverge from the contract AND desynchronize S2/M4.T2.
//   Implement ceilDiv(runes,4).

// CRITICAL (CEILING division, not truncating, design §1): use (n+d-1)/d, NOT n/d. A 1-rune string is 1 token
//   (not 0); a 5-char string is 2 (not 1). The contract's "rounded up" is literal. (n+3)/4 yields 0 for n=0
//   naturally — no special-case.

// CRITICAL (RUNE count, not byte count, design §3): utf8.RuneCountInString(s) / utf8.RuneCount(b), NOT
//   len(s). A 4-rune CJK string (12 bytes) is 1 token, not 3. The contract's "4-rune CJK ⇒ 1" test PINS this.
//   len([]rune(s)) (the item's literal) also works but allocates a rune slice; utf8.RuneCountInString is the
//   allocation-free equivalent (same count).

// CRITICAL (EXPORTED, design §0): the item writes "estimateTokens" (lowercase) as pseudocode, but S2 sets
//   StagedDiffOptions.PromptReserveTokens from CROSS-PACKAGE call sites (git.go:72 "git layer RECEIVES this;
//   does not compute it"). So the function is `EstimateTokens` (capital E). Unexported = S2 can't reach it.

// GOTCHA (placement internal/git, not internal/util, design §0): no internal/util exists; creating one for a
//   2-line function is over-engineering. Every S2/M4.T2 consumer already imports internal/git; internal/prompt
//   does not (no neutral-package need). NEW internal/git/tokens.go.

// GOTCHA (ONE formula — no second estimator, design §4): S2 and M4.T2 MUST use the same ceilDiv(runes,4).
//   Do not add a chars/3 variant, a line-based variant, or a per-provider tokenizer hook. The model-agnostic
//   design (N2) depends on ONE formula.

// GOTCHA (no conflict with the parallel P1.M3.T1.S2): that creates internal/git/skeleton.go + edits git.go's
//   3 diff functions + golden tests. It does NOT touch tokens.go/tokens_test.go. This subtask edits NOTHING.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/git/tokens.go — NO new types. Two exported one-liners + one unexported ceiling-div helper.
// The "data model" is the int token count; the structure is the single formula.

// tokens.go
package git

import "unicode/utf8"

// [file-level comment: FR3d/FR3i model-agnostic token estimator; ~4 chars ≈ 1 token; rune-based ceiling.]

// EstimateTokens returns a model-agnostic token-count estimate for s using the standard "~4 chars ≈ 1 token"
// heuristic (PRD §9.1 FR3d; git_diff_semantics.md §5): ceil(runeCount / 4), rounded UP. Rune-based (not
// byte-based) so multi-byte UTF-8 (CJK, emoji) does not over-count. The SINGLE estimator used by the
// prompt-reserve measurement (P1.M4.T1.S2) and the FR3i water-fill sizing/truncation (P1.M4.T2) — both call
// this so the budget arithmetic is consistent. The FR3d/FR3i safety margin (applied in M4.T2) absorbs the
// code-vs-prose density gap; this function is the consistent measure, not the safety mechanism.
func EstimateTokens(s string) int {
	return ceilDiv(utf8.RuneCountInString(s), 4)
}

// EstimateTokensBytes is the []byte form of EstimateTokens (same ceil(runes/4) formula). Convenience for
// callers that hold a []byte (e.g. a diff body buffer); allocation-free via utf8.RuneCount.
func EstimateTokensBytes(b []byte) int {
	return ceilDiv(utf8.RuneCount(b), 4)
}

// ceilDiv returns the ceiling of n/d for n≥0, d>0: (n+d-1)/d. Yields 0 for n=0 (no special-case).
func ceilDiv(n, d int) int { return (n + d - 1) / d }
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/git/tokens.go — EstimateTokens + EstimateTokensBytes + ceilDiv
  - FILE: NEW internal/git/tokens.go. PACKAGE: `package git`. IMPORT: `unicode/utf8` ONLY (no fmt/strings).
  - ADD a file-level comment (NOT a `// Package git` doc — that lives in git.go; avoid a duplicate package
      doc): explain this is the FR3d/FR3i model-agnostic estimator, ~4 chars ≈ 1 token, rune-based ceiling,
      the SINGLE formula shared by S2 + M4.T2, and that the FR3d/FR3i margin (M4.T2) is the safety buffer.
  - IMPLEMENT `func ceilDiv(n, d int) int { return (n + d - 1) / d }` (UNEXPORTED helper; one line).
  - IMPLEMENT `func EstimateTokens(s string) int { return ceilDiv(utf8.RuneCountInString(s), 4) }`.
  - IMPLEMENT `func EstimateTokensBytes(b []byte) int { return ceilDiv(utf8.RuneCount(b), 4) }`.
  - DOC COMMENTS on both exported funcs cite PRD §9.1 FR3d, the rune-based ceiling semantics, and the
      "single estimator / margin-is-separate" rationale (so future readers don't second-guess chars/4).
  - GOTCHA: ceiling (not truncating); runes (not bytes); exported (cross-package S2). Do NOT add a chars/3
      variant. Do NOT edit any other file.

Task 2: CREATE internal/git/tokens_test.go — pure table tests (mirror TestIsBinaryByExtension)
  - FILE: NEW internal/git/tokens_test.go. PACKAGE: `package git` (white-box — can call the unexported
      ceilDiv too if desired, but the table should target the EXPORTED funcs). IMPORT: `testing` (+ stdlib).
  - NO git repo, NO I/O — pure arithmetic. Fast.
  - TestEstimateTokens (table; one t.Errorf/case):
      * "" → 0
      * "a" (1 rune) → 1            (ceiling: any non-empty string ≥ 1 token)
      * "abcd" (4 ASCII) → 1
      * "abcde" (5 ASCII) → 2       (ceilDiv(5,4)=2 — the ceiling pin)
      * "abcdefgh" (8 ASCII) → 2
      * "你好世界" (4 runes, 12 bytes) → 1   (THE rune-based UTF-8 pin — byte-based would wrongly say 3)
      * a 4000-rune string → 1000   (sanity; no int overflow)
  - TestEstimateTokensBytes (parity + a couple of cases):
      * EstimateTokensBytes([]byte("abcd")) == 1
      * EstimateTokensBytes([]byte("你好世界")) == 1   (rune-based parity with the string form)
      * EstimateTokensBytes([]byte("abcdefgh")) == 2
      * parity: EstimateTokens(s) == EstimateTokensBytes([]byte(s)) for each table input (loop).
  - (Optional) TestCeilDiv: a few (n,d) → expected pairs to pin the helper directly (0/4→0, 1/4→1, 4/4→1,
      5/4→2, 8/4→2).
  - GOTCHA: hardcode the expected ints (do NOT derive them from the function — circular). Mirror
      binary_test.go's table idiom (named cases, descriptive t.Errorf).

Task 3: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/git/tokens.go internal/git/tokens_test.go`
  - `go vet ./internal/git/`
  - `go test ./internal/git/ -run 'TestEstimateTokens|TestCeilDiv' -v` → PASS.
  - `go build ./... && go test ./...` → GREEN (no regression; the new funcs aren't called anywhere yet).
  - `git diff --exit-code go.mod go.sum` → empty (stdlib unicode/utf8 only — no new module deps).
  - `git status` → ONLY internal/git/tokens.go + internal/git/tokens_test.go (2 new files); nothing else.
```

### Implementation Patterns & Key Details

```go
// THE formula (ceiling, rune-based, the contract):
//   tokens = ceil(runeCount / 4) = (runeCount + 3) / 4
// Rune count via utf8.RuneCountInString(s) (allocation-free; == len([]rune(s)) but no slice alloc).

// THE helper:
func ceilDiv(n, d int) int { return (n + d - 1) / d }   // 0 for n=0; ceiling for n>0

// THE functions (EXPORTED — S2 is cross-package):
func EstimateTokens(s string) int      { return ceilDiv(utf8.RuneCountInString(s), 4) }
func EstimateTokensBytes(b []byte) int { return ceilDiv(utf8.RuneCount(b), 4) }

// CEILING vs TRUNCATING (the easy bug): truncating n/4 gives ("a")==0 (WRONG — a non-empty string is ≥1
// token). Ceiling (n+3)/4 gives ("a")==1, ("abcde")==2. The contract's "rounded up" is literal.

// RUNE vs BYTE (the UTF-8 bug): len("你好世界")==12 → byte-based 12/4=3 (WRONG); utf8.RuneCountInString==4
// → rune-based 4/4=1 (correct). Diffs are ASCII-dominated but commit messages/paths/docs carry UTF-8.

// WHY EXPORTED: S2 (P1.M4.T1.S2) sets StagedDiffOptions.PromptReserveTokens at the 6 cross-package call
// sites (generate/decompose/pkg) — git.go:72 says the git layer RECEIVES the reserve, doesn't compute it.
// So the measurement is outside internal/git ⇒ EstimateTokens must be reachable ⇒ capital E.

// WHY chars/4 NOT chars/3: the contract (FR3d "~4 chars ≈ 1 token estimate with a safety margin") + the
// SINGLE-estimator rule (S2 + M4.T2 must agree). The arch doc's chars/3 ceiling-recommendation is absorbed
// by the FR3d/FR3i `margin` (M4.T2), not by changing this formula. (research §2.)
```

### Integration Points

```yaml
GO.MODULE (go.mod/go.sum): change NONE. tokens.go imports only stdlib `unicode/utf8`; the test imports
      `testing`. `go mod tidy` is a no-op.

PACKAGE EDGES: NONE added. tokens.go is `package git` (a leaf helper file, like binary.go/numstat.go).
      internal/git is imported BY generate/decompose/cmd/pkg/stagecoach (all S2/M4.T2 consumers); nothing
      internal/git imports is affected. No cycle.

UPSTREAM: NONE. Pure utility — takes a string/[]byte, returns an int. No inputs from other packages.

DOWNSTREAM (NOT this task — just honor the API):
  - P1.M4.T1.S2 (prompt reserve): `opts.PromptReserveTokens = git.EstimateTokens(sysPrompt + examples)` at
        the 6 StagedDiff/TreeDiff/WorkingTreeDiff call sites (sets the field at git.go:72).
  - P1.M4.T2.S1/S2 (FR3i water-fill): `git.EstimateTokens(fileBody)` for per-file sizing + the level
        solver; `git.EstimateTokens(skeleton)` for the skeleton budget term.
  => The `EstimateTokens(s string) int` + `EstimateTokensBytes(b []byte) int` signatures are FROZEN after
     this subtask. Both consumers use ceilDiv(runes,4) — the SINGLE formula (no second estimator).

FROZEN/LEAVE (do NOT edit):
  - internal/git/git.go (StagedDiffOptions is READ-ONLY; the diff functions are S2's/M4.T2's to edit).
  - internal/git/binary.go, numstat.go, skeleton.go (if it exists by the time this runs — the parallel S2).
  - everything else (internal/{prompt,generate,decompose,provider,config,cmd}, pkg/*, go.mod, Makefile, PRD.md).
  - NO config/CLI/API/docs surface in this subtask (the estimator is internal; the user-facing token_limit
    description belongs in M5's docs sweep).
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/git/tokens.go internal/git/tokens_test.go
go vet ./internal/git/
# Confirm the file is package git, imports only unicode/utf8, and exports the two funcs:
head -8 internal/git/tokens.go   # → package git ; import "unicode/utf8"
grep -n 'func EstimateTokens\|func EstimateTokensBytes\|func ceilDiv' internal/git/tokens.go
# Confirm go.mod/go.sum unchanged:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; tokens.go imports only unicode/utf8; the 3 funcs present; go.mod byte-unchanged.
```

### Level 2: Unit tests (the new pure table tests)

```bash
go test ./internal/git/ -run 'TestEstimateTokens|TestEstimateTokensBytes|TestCeilDiv' -v
# Expected PASS — verify each contract case:
#   "" → 0 ; "a" → 1 (ceiling) ; "abcd" → 1 ; "abcde" → 2 (ceiling) ; "abcdefgh" → 2 ;
#   "你好世界" → 1 (4 runes, 12 bytes — RUNE-based, not 3) ; 4000 runes → 1000 ;
#   EstimateTokensBytes parity with EstimateTokens on every input.
# If "你好世界" → 3, the impl used len(s) (bytes) instead of utf8.RuneCountInString (fix to rune count).
# If "a" → 0 or "abcde" → 1, the impl used truncating n/4 instead of ceiling (n+3)/4 (fix the helper).

go test ./internal/git/    # full git suite — the new tests + NO regression (nothing calls the funcs yet).
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...   # Expect clean (a pure leaf helper; nothing references it yet).
go test ./...    # Expect all PASS — no regression (the funcs are unreferenced outside the new test).
# Confirm ONLY the 2 new files differ (edits NOTHING):
git status --porcelain
# Expected: exactly 2 untracked files (internal/git/tokens.go, internal/git/tokens_test.go); NO modifications.
git diff --exit-code internal/git/git.go internal/git/binary.go internal/git/numstat.go \
  go.mod go.sum PRD.md && echo "frozen files UNCHANGED (expected)"
```

### Level 4: Correctness reasoning (the formula contract)

```bash
# No git/server/DB. The correctness IS the table test. Verify by reasoning + Level 2:
#   1. CEILING: ceilDiv(n,4)=(n+3)/4 → 0 only at n=0; ≥1 for any n≥1. (TestEstimateTokens: "a"→1, "abcde"→2.)
#   2. RUNE-BASED: a 4-rune/12-byte CJK string → 1 token (NOT 3). Pins UTF-8 correctness.
#   3. SINGLE FORMULA: EstimateTokens and EstimateTokensBytes are both ceilDiv(runes,4) — verify parity
#      (the parity loop in TestEstimateTokensBytes). No chars/3 / line-based / per-provider variant exists:
! grep -q 'chars.*3\|/ 3\|RuneCountInString.*3' internal/git/tokens.go && echo "OK: no second formula"
#   4. The chars/4-vs-chars/3 safety question is NOT this estimator's concern — it is the FR3d/FR3i margin
#      (M4.T2). Confirm this subtask adds no margin/safety logic:
! grep -q 'margin\|safety' internal/git/tokens.go && echo "OK: no margin logic (M4.T2's job)"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l internal/git/tokens*.go` clean.
- [ ] `go test ./...` PASS (the new table tests + no repo-wide regression).
- [ ] go.mod/go.sum byte-unchanged; tokens.go imports ONLY `unicode/utf8`.
- [ ] `git status` shows EXACTLY 2 new files (tokens.go, tokens_test.go); NO file modified.

### Feature Validation
- [ ] `EstimateTokens` + `EstimateTokensBytes` exist, exported, `package git`.
- [ ] `ceilDiv(n,d)=(n+d-1)/d` (ceiling; 0 for n=0). Both funcs are `ceilDiv(utf8.RuneCount…, 4)`.
- [ ] Table passes: ""→0; "a"→1; "abcd"→1; "abcde"→2; "abcdefgh"→2; "你好世界"→1 (rune-based); 4000 runes→1000.
- [ ] `EstimateTokensBytes` parity with `EstimateTokens` (same count for a string and its []byte form).
- [ ] ONE formula only — no chars/3 variant, no line-based variant, no per-provider hook.

### Code Quality Validation
- [ ] NEW file mirrors the pure-helper-file precedent (binary.go/numstat.go); the test mirrors
      `TestIsBinaryByExtension`'s table idiom (named cases, one t.Errorf/case).
- [ ] Doc comments cite PRD §9.1 FR3d, the rune-based ceiling semantics, and the "single estimator /
      margin-is-separate" rationale.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn; no new dependency.

### Documentation
- [ ] Doc comments on `EstimateTokens`/`EstimateTokensBytes` record: the ~4-chars-≈-1-token heuristic, the
      rune-based ceiling, the SINGLE-estimator rule (S2 + M4.T2), and that the FR3d/FR3i margin (M4.T2) is
      the safety buffer (so chars/4 isn't second-guessed).
- [ ] NO docs/*.md edits (the user-facing `token_limit` description belongs in M5's docs sweep).

---

## Anti-Patterns to Avoid

- ❌ **Don't switch the formula to chars/3.** The contract (FR3d "~4 chars ≈ 1 token estimate") + the
  SINGLE-estimator rule pin chars/4. The architecture doc's chars/3 ceiling-recommendation is absorbed by
  the separate FR3d/FR3i `margin` (M4.T2), not by changing this estimator. Switching desyncs S2/M4.T2.
  (§2)
- ❌ **Don't use truncating division `n/4`.** A 1-rune string would yield 0 tokens (wrong — any non-empty
  string is ≥1 token). Use ceiling `(n+3)/4`. The contract says "rounded up". (§1)
- ❌ **Don't count bytes (`len(s)`).** Multi-byte UTF-8 over-counts: a 4-rune CJK string is 1 token, not 3.
  Use `utf8.RuneCountInString`/`utf8.RuneCount`. The contract's "4-rune CJK ⇒ 1" test pins this. (§3)
- ❌ **Don't leave the function unexported.** The item writes `estimateTokens` as pseudocode, but S2 measures
  `PromptReserveTokens` from CROSS-PACKAGE call sites (git.go:72: "git layer RECEIVES this; doesn't compute
  it"). It MUST be `EstimateTokens` (capital E). (§0)
- ❌ **Don't create internal/util.** No such package exists; making one for a 2-line function is
  over-engineering. Every consumer already imports internal/git; internal/prompt doesn't need it. Use
  internal/git/tokens.go. (§0)
- ❌ **Don't add a second estimator.** No chars/3 variant, no line-based variant, no per-provider tokenizer
  hook. S2 and M4.T2 MUST share ONE formula or the budget arithmetic is incoherent. The model-agnostic
  design (N2) depends on it. (§4)
- ❌ **Don't add margin/safety logic here.** This subtask is the ESTIMATOR only. The FR3d/FR3i `margin` (the
  safety buffer) is M4.T2's concern. (§2)
- ❌ **Don't edit any existing file.** This is 2 NEW files (tokens.go + tokens_test.go). git.go
  (StagedDiffOptions) is READ-ONLY; the diff functions are S2's/M4.T2's to edit. (scope boundary)
- ❌ **Don't derive test expectations from the function.** Hardcode the expected ints (else the test is
  circular and can't catch a wrong formula). (§5)
