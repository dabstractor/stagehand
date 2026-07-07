---
name: "P1.M1.T2.S1 — Busy exit code in internal/exitcode"
description: |
  Add the `Busy = 5` exit-code constant to `internal/exitcode/exitcode.go` (after `Rescue=3`, before
  `Timeout=124` — keep 124 last as it mirrors GNU `timeout`), with an inline doc comment. NO change to
  `For()` (its existing `*ExitError` short-circuit returns 5 for `New(Busy,nil)`) or to `ExitError`/`New`
  (`New(code,nil)` already supports the silent non-zero exit the contention path needs). Add one `TestFor`
  row proving `New(Busy,nil)→Busy` + a `TestBusyCodeValue` pin (==5). Add the `5`/Busy row to the
  `docs/cli.md` `## Exit codes` table + a note explaining the two contention behaviors (no-op exit 0 and
  busy exit 5) distinct from commit-failure codes. Value 5 is free (4 reserved). This is the exitcode half
  of FR52; the lock primitive is S1, the wiring/E2E is the sibling S2.
---

## Goal

**Feature Goal**: Add a distinct, script-distinguishable `Busy` exit code (`5`) to the exit-code registry
so the FR52 per-repo run-lock contention path can signal "another stagecoach run holds the lock; retry"
separately from the commit-failure codes (1/3/124) — letting wrappers/CI tell "busy, retry" from "failed."

**Deliverable** (one production file + its test + one doc, all surgical):
1. `internal/exitcode/exitcode.go` — `Busy = 5` added to the const block (after `Rescue=3`, before
   `Timeout=124`) with an inline comment; `For()`, `ExitError`, `New` UNCHANGED.
2. `internal/exitcode/exitcode_test.go` — one new `TestFor` table row (`New(Busy, nil) → Busy`) + a new
   `TestBusyCodeValue` test pinning `Busy == 5`.
3. `docs/cli.md` — a `5`/Busy row in the `## Exit codes` table (between `3` and `124`) + a note below the
   table explaining the two contention behaviors and the "busy, retry" vs "failed" distinction.

**Success Definition**: `exitcode.Busy` is a valid exported constant equal to 5; `exitcode.For(exitcode.New(exitcode.Busy, nil))` returns `5`; `go build/vet/gofmt` clean; `go test -race ./...` green; `docs/cli.md` lists code 5 and the contention note. No change to `For()`'s arms, `ExitError`, `New`, or any other package.

## User Persona

**Target User**: (1) The S2 contributor wiring the contention path (`runDefault` will `return exitcode.New(exitcode.Busy, nil)` after printing the holder message); (2) end users / CI scripts that wrap stagecoach and need to distinguish "busy, retry" from a real failure.

**Use Case**: A user accidentally runs stagecoach twice in two terminals on the same repo. The second run finds the lock held, sees genuinely new staged work, prints "another stagecoach run is already in progress (pid N on host)… re-run after it finishes," and exits **5**. The user's wrapper script sees 5 (not 1/3/124), knows it was not a failure, and can retry or surface a "busy" status.

**User Journey**: second stagecoach invocation → `lock.Acquire` returns `*HeldError` → `runDefault` prints the contention message → `return exitcode.New(exitcode.Busy, nil)` → `main.go`'s `exitcode.For(err)` short-circuit returns 5 → process exits 5 (no double-print, since `ExitError{Err:nil}.Error()==""`).

**Pain Points Addressed**: Removes the ambiguity where a contention refusal would otherwise reuse code 1 (indistinguishable from a generation failure) — scripts can now branch on 5 = "retry later."

## Why

- **PRD §18.5 (FR52) mandates a distinct contention exit code.** The contention-behavior section ends: "The non-zero exit code is distinct from the commit-failure codes so scripts can tell 'busy, retry' from 'failed' (§15.4; add exit `Busy`)." This subtask is that `Busy` code.
- **PRD §15.4 is the exit-code table of record.** It currently lists 0/1/2/3/124; FR52 calls for adding `Busy`. The `docs/cli.md` table mirrors §15.4 and must carry the new row + the rationale.
- **The silent-exit pattern is already supported.** `exitcode.New(code, nil)` + `ExitError{Err:nil}.Error()==""` exists precisely so a caller can print a message once and exit a forced code without `main.go` re-printing (proven by `TestExitError_NilErr`). `Busy` reuses this; no new machinery.
- **No behavioral risk.** Adding an unused constant + comment changes no control flow. `For()` is untouched (its `*ExitError` short-circuit returns `ee.Code` for any `New(code,…)`); the only new runtime path is S2's future `New(Busy, nil)`, which the new test row proves resolves to 5.
- **Unblocks S2 (the wiring).** S2's `runDefault` contention branch returns `exitcode.New(exitcode.Busy, nil)`; that symbol must exist first. This task is the constant; S2 is the call site + E2E.

## What

A single exported constant `Busy = 5` with an inline comment, placed in the existing const block after
`Rescue` and before `Timeout`. Plus one test row, one value-pin test, and one docs table row + note. No
change to `For()`, `ExitError`, `New`, or any other package. `Busy` is only ever returned via explicit
`exitcode.New(exitcode.Busy, nil)` (the S2 contention path); it never flows through a sentinel arm.

### Success Criteria

- [ ] `internal/exitcode/exitcode.go` const block contains `Busy = 5` with an inline comment, positioned
      after `Rescue = 3` and before `Timeout = 124`.
- [ ] `Timeout = 124` remains the LAST constant in the block (mirrors GNU `timeout`).
- [ ] `For()` is UNCHANGED (no new arm; the `*ExitError` short-circuit covers `Busy`).
- [ ] `ExitError`, `New`, `Error()`, `Unwrap()` are UNCHANGED.
- [ ] `exitcode_test.go` adds a `TestFor` row proving `For(New(Busy, nil)) == Busy` AND a `TestBusyCodeValue`
      proving `Busy == 5`.
- [ ] `docs/cli.md` `## Exit codes` table has a `5`/Busy row (between `3` and `124`), plus a note below the
      table on the two contention behaviors and the "busy, retry" vs "failed" distinction.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] No file outside `internal/exitcode/` and `docs/cli.md` is modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the current const block verbatim, gives the exact one-line insert
(with the resolved comment text and placement), explains why `For()` needs no change (with the
short-circuit line quoted), provides the exact test row + value-pin test, the exact docs table row + note
text, and the verification gates. `Busy` is confirmed absent (a genuine add, not a verify task), and the
baseline is green. No inference required.

### Documentation & References

```yaml
# MUST READ — the binding exit-code contract + the contention rationale
- file: PRD.md
  why: "§15.4 (Exit codes) is the table of record (0/1/2/3/124) that gains the Busy row. §18.5 (FR52, the per-repo run lock) — its 'Contention behavior' paragraph ends: 'The non-zero exit code is distinct from the commit-failure codes so scripts can tell \"busy, retry\" from \"failed\" (§15.4; add exit Busy).' That sentence is the authorization for this constant."
  critical: "§15.4 + §18.5 together mandate: a NEW code (not reusing 1), distinct from commit-failure codes, for the contention refusal. This subtask owns ONLY the exitcode constant + the cli.md row; the lock primitive is S1, the wiring is S2."

- docfile: plan/006_c23e6f286ae7/architecture/integration_seams.md
  why: "§2 is the authoritative spec for THIS edit: the exact const-block target (Busy = 5 after Rescue, before Timeout), the value rationale (5 is free; 4 reserved; no sysexits.h collision), and the explicit 'For() mapping — no change needed' with the short-circuit line reference."
  critical: "§2 confirms: value 5 (4 reserved); placement after Rescue/before Timeout; NO For() change (the *ExitError short-circuit handles it); NO new errors.Is arm. §8 confirms docs/cli.md ## Exit codes (line ~366) is THIS task's doc edit (Mode A, rides with the contention/wiring subtask)."

- docfile: plan/006_c23e6f286ae7/P1M1T2S1/research/busy_exitcode_notes.md
  why: "EMPIRICALLY VERIFIED: the live const block (verbatim), the For() short-circuit line that makes a new arm unnecessary, the New(code,nil) silent-exit proof (TestExitError_NilErr), the comment-style decision (drop the redundant 'Busy — ' prefix to match siblings), the exact test row + value-pin test, the exact docs row + note, and the decisions D1–D8. READ THIS FIRST."
  critical: "§3 (why For() is unchanged) and §4 (why New/ExitError are unchanged) prevent the two most likely over-edits. §5 gives the copy-paste test additions; §6 gives the copy-paste docs additions."

- file: internal/exitcode/exitcode.go
  why: "THE edit target. The const block (lines ~22-30) is where Busy is inserted. For() (line ~52) has the `var ee *ExitError; if errors.As(err, &ee) { return ee.Code }` short-circuit — this is WHY no new arm is needed. ExitError/New (lines ~36-49) already support New(code, nil) silent exits."
  pattern: "Inline `// comment` per constant, no name prefix (e.g. `Rescue = 3 // snapshot taken…`). gofmt-aligned `=` and `//` columns. Constants omit the 'Exit' prefix (exitcode.Success, not exitcode.ExitSuccess) — naming decision D1, deployed at ~40 call sites."
  gotcha: "Insert Busy BETWEEN Rescue and Timeout (keep Timeout=124 LAST). gofmt re-aligns the block automatically — run gofmt -w after the edit."

- file: internal/exitcode/exitcode_test.go
  why: "EDIT TARGET (tests). Same-package (package exitcode). TestFor is table-driven (tests []struct{name;err;want} + t.Run) — ADD one row. TestExitError_NilErr proves New(Error,nil) → For returns Error; the same machinery covers New(Busy,nil)."
  pattern: "Table row shape: {\"<name>\", New(<code>, nil), <code>}. See the existing {\"explicit ExitError custom code\", New(7, errors.New(\"custom\")), 7} row — mirror it with Busy."
  gotcha: "Add the value-pin test TestBusyCodeValue (Busy == 5) — the integer is a public contract for shell scripts; pinning it catches a future renumbering that would silently break consumers."

- file: docs/cli.md
  why: "EDIT TARGET (docs). The ## Exit codes table is at lines ~368-374 (rows 0/1/2/3/124); the explanatory paragraph is at line ~376. Add the 5/Busy row BETWEEN the 3 and 124 rows; add the contention note AFTER the line-376 paragraph."
  pattern: "Row format: `| \`<code>\` | <Meaning> |`. The existing paragraph at :376 starts 'Exit codes mirror the constants in internal/exitcode/exitcode.go.' — append the contention note as a new paragraph after it."
  gotcha: "Place the 5 row between 3 and 124 (ascending order, matching the const block where Busy precedes Timeout). Do NOT reorder the existing rows."

# Cross-references (read-only — do NOT edit)
- docfile: plan/006_c23e6f286ae7/P1M1T1S1/PRP.md
  why: "S1 (the lock primitive) — establishes the S1/S2 scope boundary. S1 owns internal/lock/* + docs/how-it-works.md + docs/configuration.md and EXPLICITLY does NOT touch internal/exitcode or docs/cli.md (those are THIS task). S1's HeldError type is what S2's wiring will translate into exitcode.New(Busy,nil)."
  critical: "No conflict: S1 never edits exitcode.go or docs/cli.md. This task's symbol (exitcode.Busy) is what S2's runDefault will use. The three subtasks are sequential complements: S1=primitive, S1(this)=code, S2(sibling)=wiring+E2E."

# External references (exact, anchor-level)
- url: https://go.dev/ref/spec#Iota
  why: "Confirms Go const blocks without iota use explicit values (as this block does: 0/1/2/3/124). Busy=5 is an explicit value, consistent with the existing style — no iota introduction."
- url: https://man.openbsd.org/sysexits
  why: "sysexits.h reference — confirms 5 is not a load-bearing sysexits code (sysexits uses 64-78 for its named errors), so 5 is safe and unambiguous for a custom 'busy' meaning. integration_seams §2 cites this as the no-collision rationale."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/exitcode/
    ├── exitcode.go        # EDIT TARGET: const block + (unchanged) For/ExitError/New
    └── exitcode_test.go   # EDIT TARGET: +1 TestFor row + TestBusyCodeValue
└── docs/
    └── cli.md             # EDIT TARGET: ## Exit codes table + note (line ~366-376)
# (internal/lock/ is S1's territory — do NOT touch)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/exitcode/exitcode.go        # +Busy = 5 constant (const block); For/ExitError/New unchanged
    internal/exitcode/exitcode_test.go   # +1 TestFor row + TestBusyCodeValue
    docs/cli.md                          # +5/Busy table row + contention note
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/exitcode/exitcode.go` | MODIFY | Add `Busy = 5` to the const block (after Rescue, before Timeout) + inline comment. **Only production file touched.** |
| `internal/exitcode/exitcode_test.go` | MODIFY | Add one `TestFor` row (`New(Busy,nil)→Busy`) + `TestBusyCodeValue` (==5). |
| `docs/cli.md` | MODIFY | Add the `5`/Busy row to `## Exit codes` + the contention note. |

**Explicitly NOT touched**: `internal/lock/*` (S1 — the lock primitive), `internal/cmd/*`
(S2 — runDefault wiring), `internal/generate/*` + `internal/decompose/*` (S2 — SetSnapshot call sites),
`docs/how-it-works.md` + `docs/configuration.md` (S1 — lock docs), `README.md` (S3 — safety pitch),
any other package, `PRD.md`, `tasks.json`, `prd_snapshot.md`, anything under `plan/`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — do NOT add a For() arm for Busy): For()'s `var ee *ExitError; if errors.As(err, &ee)
// { return ee.Code }` short-circuit (exitcode.go ~line 57) ALREADY returns 5 for New(Busy,nil). Busy is
// ONLY constructed via explicit New (the S2 contention path) — it never flows through a sentinel arm.
// Adding `if errors.Is(err, somethingBusy) { return Busy }` would be dead code (no Busy sentinel exists
// or should exist). The contract is explicit: "No change to For()."

// CRITICAL (G2 — Busy=5, NOT 4): value 5 is the chosen code. 4 is deliberately reserved for future use
// (integration_seams §2). 5 is distinct from 0/1/2/3/124 and avoids sysexits.h collisions. The
// TestBusyCodeValue pin (==5) guards this exact value for shell-script consumers.

// CRITICAL (G3 — keep Timeout=124 LAST): insert Busy AFTER Rescue=3 and BEFORE Timeout=124. The contract
// mandates Timeout remain last (it mirrors GNU `timeout`). Ascending order otherwise (0,1,2,3,5,124).

// GOTCHA (G4 — comment style): the const block uses inline comments that do NOT repeat the constant name
// (Rescue = 3 // snapshot taken…, NOT // Rescue — snapshot taken…). Drop the contract's redundant
// "Busy — " prefix; use: Busy = 5 // another stagecoach run holds the per-repo lock; retry after it finishes (FR52 §18.5)

// GOTCHA (G5 — gofmt re-aligns the block): adding Busy changes the column alignment (the `=` and `//`
// columns are gofmt-aligned across the block). Run `gofmt -w internal/exitcode/exitcode.go` after the
// insert; do NOT hand-align (gofmt is authoritative).

// GOTCHA (G6 — the silent-exit pattern is intentional, not a bug): New(Busy, nil) produces an ExitError
// whose Error() returns "" (because Err==nil). This is BY DESIGN for the contention path: runDefault
// prints the holder message once, then returns New(Busy,nil) so main.go does NOT double-print. Do NOT
// "fix" this by requiring a non-nil Err — it would cause double-printing. (Proven by TestExitError_NilErr.)

// GOTCHA (G7 — naming: Busy, not ExitBusy): the package omits the "Exit" prefix everywhere
// (exitcode.Success, exitcode.Rescue, etc. — decision D1, ~40 call sites). The new constant is
// exitcode.Busy, NOT exitcode.ExitBusy.

// GOTCHA (G8 — scope vs siblings): S1 owns internal/lock + docs/how-it-works + docs/configuration.
// S2 owns runDefault wiring + generate/decompose SetSnapshot + docs E2E. S3 owns README. This task owns
// ONLY exitcode.go + exitcode_test.go + docs/cli.md. Editing a sibling file is scope creep.
```

## Implementation Blueprint

### Data models and structure

No new types. `Busy` is an untyped (int) constant in the existing const block, exactly like its siblings.
`ExitError`, `New`, `For` are unchanged. The "model" fact is the value→semantics mapping: `5 → Busy
(contention refusal, distinct from commit-failure 1/3/124)`.

### The single const-block edit (exact — current → target)

```go
// CURRENT (exitcode.go ~lines 22-30)
const (
	Success         = 0   // commit created, or dry-run message printed
	Error           = 1   // general error (generation failed, parse failed, agent missing, CAS, usage/flag)
	NothingToCommit = 2   // clean tree after auto-stage, or nothing staged with --no-auto-stage
	Rescue          = 3   // snapshot taken, commit not created — manual recovery printed
	Timeout         = 124 // generation exceeded --timeout (mirrors GNU `timeout`)
)

// TARGET — insert Busy AFTER Rescue, BEFORE Timeout (gofmt re-aligns the columns)
const (
	Success         = 0   // commit created, or dry-run message printed
	Error           = 1   // general error (generation failed, parse failed, agent missing, CAS, usage/flag)
	NothingToCommit = 2   // clean tree after auto-stage, or nothing staged with --no-auto-stage
	Rescue          = 3   // snapshot taken, commit not created — manual recovery printed
	Busy            = 5   // another stagecoach run holds the per-repo lock; retry after it finishes (FR52 §18.5)
	Timeout         = 124 // generation exceeded --timeout (mirrors GNU `timeout`)
)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/exitcode/exitcode.go — add the Busy constant
  - FILE: internal/exitcode/exitcode.go
  - LOCATE the const block (the `const (` … `)` containing Success/Error/NothingToCommit/Rescue/Timeout).
  - INSERT one line AFTER the `Rescue = 3 …` line and BEFORE the `Timeout = 124 …` line:
        Busy            = 5   // another stagecoach run holds the per-repo lock; retry after it finishes (FR52 §18.5)
  - COMMENT: inline, NO redundant "Busy — " prefix (match sibling style). Text exactly as above.
  - DO NOT: edit For(), ExitError, New, Error(), Unwrap(), or the package/const doc comments.
  - RUN: gofmt -w internal/exitcode/exitcode.go   (re-aligns the `=` and `//` columns)
  - VERIFY: go build ./internal/exitcode/  → exit 0.

Task 2: EDIT internal/exitcode/exitcode_test.go — add the test row + value pin
  - FILE: internal/exitcode/exitcode_test.go
  - EDIT 1: in TestFor's `tests` slice, ADD one row (anywhere in the slice; grouped with the other
    explicit-ExitError rows is cleanest):
        {"explicit ExitError Busy (silent, nil err) → 5", New(Busy, nil), Busy},
  - EDIT 2: ADD a new top-level test (after TestExitError_NilErr or at file end):
        func TestBusyCodeValue(t *testing.T) {
            // Busy is 5 — distinct from 0/1/2/3/124; 4 is reserved (integration_seams §2).
            if Busy != 5 {
                t.Errorf("Busy = %d, want 5", Busy)
            }
        }
  - WHY the row: proves For(New(Busy,nil))==Busy via the *ExitError short-circuit (the exact path S2 uses).
  - WHY the pin: the integer 5 is a public contract for shell scripts; pin it against accidental renumbering.
  - RUN: gofmt -w internal/exitcode/exitcode_test.go
  - VERIFY: go test -race ./internal/exitcode/ -v  → TestFor + TestBusyCodeValue + TestExitError_NilErr PASS.

Task 3: EDIT docs/cli.md — the table row + the contention note
  - FILE: docs/cli.md
  - EDIT 1 (table row): in the `## Exit codes` table, INSERT a new row BETWEEN the `3` (Rescue) row and
    the `124` (Timeout) row:
        | `5` | Busy — another stagecoach run holds the per-repo lock; retry after it finishes. |
  - EDIT 2 (note): AFTER the existing explanatory paragraph that begins "Exit codes mirror the constants
    in `internal/exitcode/exitcode.go`." (line ~376), ADD a new paragraph:
        Code `5` (Busy) is distinct from the commit-failure codes so scripts can tell "busy, retry" from
        "failed." Contention on the per-repo run lock (FR52) has two behaviors: if a contending run's
        staged changes are already covered by the in-progress run's published snapshot, it exits **0**
        ("nothing to do — an in-progress run already covers your staged changes"); if genuinely new work
        is staged, it exits **5** with the holder's pid/host and leaves the new changes staged for a
        re-run. Stagecoach never force-breaks the lock.
  - DO NOT: reorder the existing table rows; do NOT edit other sections of docs/cli.md.

Task 4: VALIDATE (run all gates; all must pass before declaring done)
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: gofmt -l .            # must be empty
  - RUN: go test -race ./...   # whole repo green
  - RUN targeted: go test -race ./internal/exitcode/ -v -run 'TestFor|TestBusyCodeValue|TestExitError_NilErr'
  - FIX-FORWARD: read failures, fix, re-run. Do NOT skip.
```

### Implementation Patterns & Key Details

```go
// === Why For() is unchanged — the short-circuit does the work ===
// For(err) order: nil→0; *ExitError→ee.Code (errors.As); then sentinel arms; else 1.
// New(Busy, nil) → *ExitError{Code:5, Err:nil} → errors.As matches → returns ee.Code (5).
// Busy has NO sentinel error and never will (it is only ever explicit-New'd by the S2 contention path).
// Therefore no errors.Is arm, no Busy-specific branch — adding one is dead code.

// === Why New(code, nil) is the silent-exit pattern (do not "fix" it) ===
// The contention path: runDefault prints the holder message ONCE, then returns exitcode.New(exitcode.Busy, nil).
// main.go calls exitcode.For(err)→5, then os.Exit(5). Because ExitError{Err:nil}.Error()=="", main does
// NOT re-print anything. A non-nil Err would cause double-printing. This is proven by TestExitError_NilErr
// (which does New(Error,nil) and asserts Error()==""). The same machinery serves Busy unchanged.

// === Why the comment drops the "Busy — " prefix ===
// The 5 sibling constants all use inline comments that describe the constant WITHOUT restating its name:
//   Rescue = 3 // snapshot taken, commit not created — manual recovery printed
// Mirror that: Busy = 5 // another stagecoach run holds the per-repo lock; retry after it finishes (FR52 §18.5)
// (The contract's "Busy — …" wording was prose narration, not the literal comment text.)

// === Why a value-pin test (Busy == 5) ===
// Callers write `exitcode.Busy` (the symbol), but shell scripts/CI match on the INTEGER. Pinning the
// integer in a test catches a future renumbering (e.g. someone inserting a code at 5 and bumping Busy to
// 6) that would silently break external consumers. 4 is intentionally reserved (integration_seams §2).
```

### Integration Points

```yaml
EXITCODE (internal/exitcode/exitcode.go):
  - const block: +Busy = 5 (after Rescue, before Timeout)
  - For() / ExitError / New / Error() / Unwrap(): UNCHANGED

TESTS (internal/exitcode/exitcode_test.go):
  - TestFor: +1 row (New(Busy,nil) → Busy)
  - +TestBusyCodeValue (Busy == 5)

DOCS (docs/cli.md):
  - ## Exit codes table: +| `5` | Busy — … | row (between 3 and 124)
  - +contention note paragraph (two behaviors: no-op exit 0, busy exit 5; distinct from commit-failure codes)

CONSUMED BY (informational — NOT this task; S2 wires it):
  - internal/cmd/default_action.go (S2): the contention branch returns exitcode.New(exitcode.Busy, nil)
    after printing the holder's pid/host/repo. The lock primitive (S1) provides *HeldError; S2 translates.

NO-TOUCH (explicitly — owned by sibling subtasks):
  - internal/lock/*                       # S1 — the lock primitive
  - internal/cmd/* , generate/* , decompose/*   # S2 — the wiring + SetSnapshot call sites + E2E
  - docs/how-it-works.md , configuration.md     # S1 — lock docs
  - README.md                             # S3 — race-free safety pitch
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by the SIBLING S2, NOT this task):
  - S2 wires runDefault: lock.Acquire → on *HeldError, print message + return exitcode.New(exitcode.Busy, nil)
  - S2's E2E asserts the stagecoach subprocess exits 5 on a held lock (distinct from 1/3/124)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/exitcode/         # Expected: empty (run gofmt -w if listed — it re-aligns the const block)
go vet ./internal/exitcode/...      # Expected: exit 0
go build ./...                      # Expected: exit 0 (adding an unused constant breaks nothing)

# Expected: Zero output/errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

# Targeted: the new + existing exitcode tests
go test -race ./internal/exitcode/ -v -run 'TestFor|TestBusyCodeValue|TestExitError_NilErr'

# Full exitcode suite
go test -race ./internal/exitcode/ -v

# Expected: ALL PASS. TestFor's new row proves For(New(Busy,nil))==Busy (5). TestBusyCodeValue pins
# Busy==5. TestExitError_NilErr still green (unchanged).
```

### Level 3: Whole-Repository Regression (no collateral)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...                  # Expected: ALL packages green (only exitcode + docs/cli.md changed)
go vet ./...                         # Expected: exit 0

# Confirm ONLY the 3 in-scope files changed
git diff --stat -- internal/exitcode/ docs/cli.md
# Expected: internal/exitcode/exitcode.go + exitcode_test.go + docs/cli.md (nothing else).

# Confirm S1's lock territory + sibling territories are UNTOUCHED
git diff --stat -- internal/lock/ internal/cmd/ internal/generate/ internal/decompose/ docs/how-it-works.md docs/configuration.md README.md
# Expected: EMPTY (this task does not touch them).
```

### Level 4: Behavioral Cross-Check (prove the short-circuit + the value)

```bash
cd /home/dustin/projects/stagecoach

# Throwaway main: proves For(New(Busy,nil))==5 (the exact S2 contention path) end-to-end.
cat > /tmp/sh_busy_check.go <<'EOF'
package main
import ("fmt";"github.com/dustin/stagecoach/internal/exitcode")
func main() {
  code := exitcode.For(exitcode.New(exitcode.Busy, nil))
  fmt.Printf("For(New(Busy,nil)) = %d (Busy=%d)\n", code, exitcode.Busy)
  if code != 5 || exitcode.Busy != 5 {
    fmt.Println("FAIL"); return
  }
  // silent-exit: ExitError{Err:nil}.Error() == "" (no double-print in main)
  ee := exitcode.New(exitcode.Busy, nil)
  fmt.Printf("silent-exit Error() = %q (want empty)\n", ee.Error())
  fmt.Println("PASS")
}
EOF
go run /tmp/sh_busy_check.go && rm -f /tmp/sh_busy_check.go
# Expected: For(New(Busy,nil)) = 5 (Busy=5) ; silent-exit Error() = "" (want empty) ; PASS

# Docs cross-check: the 5/Busy row + the contention note are present
grep -n '| `5` | Busy' docs/cli.md                  # → the new table row
grep -n 'busy, retry' docs/cli.md                   # → the contention note paragraph
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.

### Feature Validation

- [ ] `exitcode.go` const block has `Busy = 5` (after `Rescue = 3`, before `Timeout = 124`).
- [ ] `Timeout = 124` is the LAST constant (mirrors GNU `timeout`).
- [ ] `For()` is UNCHANGED (no new arm).
- [ ] `ExitError` / `New` / `Error()` / `Unwrap()` UNCHANGED.
- [ ] `For(New(Busy, nil)) == 5` (proven by the new TestFor row + the Level-4 cross-check).
- [ ] `TestBusyCodeValue` asserts `Busy == 5`.
- [ ] `docs/cli.md` `## Exit codes` has the `5`/Busy row (between `3` and `124`) + the contention note.

### Scope Discipline Validation

- [ ] ONLY `internal/exitcode/exitcode.go` + `exitcode_test.go` + `docs/cli.md` modified (`git diff --stat`).
- [ ] Did NOT edit `For()` (no new arm), `ExitError`, or `New`.
- [ ] Did NOT touch `internal/lock/*` (S1), `internal/cmd/*` / `generate/*` / `decompose/*` (S2 wiring),
      `docs/how-it-works.md` / `configuration.md` (S1 docs), `README.md` (S3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research).

### Code Quality Validation

- [ ] The inline comment drops the redundant "Busy — " prefix (matches the 5 sibling constants' style).
- [ ] The constant is `Busy` (no `Exit` prefix — package naming decision D1).
- [ ] gofmt re-aligns the const block; no hand-alignment.
- [ ] The value-pin test guards the public integer contract for shell scripts.

---

## Anti-Patterns to Avoid

- ❌ Don't add a `For()` arm for `Busy` — the `*ExitError` short-circuit (`errors.As → ee.Code`) already
  returns 5 for `New(Busy, nil)`. Busy has no sentinel and never will; an arm would be dead code (gotcha G1).
- ❌ Don't use value `4` — `4` is reserved; the code is `5` (gotcha G2, integration_seams §2).
- ❌ Don't place `Busy` after `Timeout` — keep `Timeout = 124` LAST (mirrors GNU `timeout`); insert Busy
  between `Rescue` and `Timeout` (gotcha G3).
- ❌ Don't prefix the comment with "Busy — " — the sibling constants don't restate their names; use the
  inline descriptive form (gotcha G4).
- ❌ Don't hand-align the const block — run `gofmt -w`; it re-aligns the `=` and `//` columns (gotcha G5).
- ❌ Don't "fix" the `New(code, nil)` silent-exit pattern — `ExitError{Err:nil}.Error()==""` is intentional
  (the contention path prints the message once, then silent-exits 5 to avoid double-printing). Requiring a
  non-nil Err would break that (gotcha G6).
- ❌ Don't name it `ExitBusy` — the package omits the `Exit` prefix everywhere (exitcode.Success, etc.);
  the name is `exitcode.Busy` (gotcha G7).
- ❌ Don't touch `internal/lock/*` (S1), the wiring in `cmd/generate/decompose` (S2), or the lock docs /
  README (S1/S3) — this task is the constant + the cli.md row only (gotcha G8).
- ❌ Don't reorder the existing `docs/cli.md` exit-code rows — insert the `5` row between `3` and `124`.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a minimal, fully-prescribed change — one constant + inline comment, one test row + one
value-pin test, one docs table row + one note paragraph — with every detail pinned to verified live state.
The const block is quoted verbatim (current → target); the value (5, with 4 reserved), the placement (after
Rescue, before Timeout), and the comment text (no redundant name prefix) are all decided with rationale. The
two most likely over-edits — adding a `For()` arm (unnecessary; the `*ExitError` short-circuit handles it,
proven by reading `For()`) and "fixing" the `New(code,nil)` silent-exit (intentional; proven by
`TestExitError_NilErr`) — are front-loaded as CRITICAL gotchas (G1, G6). The new `TestFor` row is the exact
assertion of the S2 call path (`For(New(Busy,nil))==5`), and the value-pin test guards the public integer
contract. The baseline is GREEN, `Busy` is confirmed absent (a genuine add), and S1's lock-package territory
is cleanly fenced (no file overlap). The residual 0.5 uncertainty is purely cosmetic (gofmt column
re-alignment and the exact wording of the docs note), both caught by the deterministic `gofmt -l .` and
`go test -race ./...` gates. The sibling S2 (wiring/E2E) consumes this symbol and cannot be broken by it
(the constant is unused until S2 wires `runDefault`).
