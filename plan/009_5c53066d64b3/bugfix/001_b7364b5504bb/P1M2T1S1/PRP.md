---
name: "P1.M2.T1.S1 — Hoist payload variable out of runPipeline generation loop"
description: |
  Pure variable-scoping refactor (0.5 pt — the structural enabler for P1.M2.T1.S2). In
  `pkg/stagecoach/stagecoach.go`, the `runPipeline` generation+dedupe loop (the `--dry-run` / SystemExtra
  path) declares `payload := prompt.BuildUserPayload(...)` INSIDE the loop (line 490), making it
  loop-scoped — invisible after the loop. The FR-T1 multi-turn trigger gate (landing in P1.M2.T1.S2) must
  read the **last-built payload** at the rescue insertion point (between `if !success {` and the
  `return &RescueError`). This task hoists `payload` to function scope — mirroring the already-hoisted
  `CommitStaged` reference (`internal/generate/generate.go:226`) — so S2's gate can read it. NO behavioral
  change: payload is still rebuilt every iteration; only its scope moves (loop → function).

  ⚠️ **THE central design call — TWO edits, mirroring CommitStaged exactly.** (1) After line 487
  (`var lastCause error`), add `var payload string // hoisted: survives the loop for the FR-T1 multi-turn
  gate (mirrors CommitStaged)`. (2) Line 490, switch `payload := prompt.BuildUserPayload(...)` →
  `payload = prompt.BuildUserPayload(...)` (`:=` → `=`). That is the ENTIRE change. The loop body is
  otherwise byte-identical; payload is still assigned at the top of every iteration.

  ⚠️ **THE second design call — behavioral UNCHANGED is the keystone invariant.** payload is STILL rebuilt
  every iteration (the `payload = prompt.BuildUserPayload(...)` is the first statement in the loop body,
  unchanged). The ONLY difference is SCOPE: loop-scoped → function-scoped, so the last-built payload
  survives for the FR-T1 gate to read post-loop. Every existing `pkg/stagecoach` test (which exercises the
  loop, not post-loop payload reads) MUST stay green byte-for-byte. No new test is warranted — a pure
  scoping refactor has no new behavior to pin; the existing suite is the regression net.

  ⚠️ **THE third design call — this is the ENABLER, not the gate.** Do NOT insert the FR-T1 multi-turn
  trigger gate, do NOT touch `multiturn.Run`, do NOT add a verbose line. Those are P1.M2.T1.S2's 2-point
  scope. S1 ONLY hoists the variable so S2 has somewhere to read it. Inserting the gate here would steal
  S2's scope and couple two changes.

  SCOPE: edit `pkg/stagecoach/stagecoach.go` ONLY (the 2 edits above). No tests, no docs, no other files.
  INPUT = the runPipeline var block (483-487) + the loop-scoped payload (490). OUTPUT = `payload` hoisted
  to function scope, readable at the rescue insertion point; consumed by P1.M2.T1.S2. DOCS: none (pure
  refactor).
---

## Goal

**Feature Goal**: Hoist the `payload` variable from loop scope to function scope in `runPipeline`
(`pkg/stagecoach/stagecoach.go`), so the last-built payload survives the generation loop and is readable at
the rescue insertion point — the structural prerequisite for the FR-T1 multi-turn trigger gate
(P1.M2.T1.S2). Zero behavioral change to the loop.

**Deliverable** (edit to ONE file):
1. **`pkg/stagecoach/stagecoach.go`** — (a) add `var payload string // hoisted: survives the loop for the
   FR-T1 multi-turn gate (mirrors CommitStaged)` immediately after line 487 (`var lastCause error`);
   (b) line 490, change `payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)` →
   `payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)` (`:=` → `=`).

**Success Definition**: `gofmt -l pkg/stagecoach/` clean; `go vet ./pkg/stagecoach/` clean (no shadowing);
`go build ./...` succeeds; `go test ./pkg/stagecoach/...` green AND `go test -race ./...` green (no
behavioral change — every existing test passes byte-for-byte). go.mod/go.sum unchanged; only
`pkg/stagecoach/stagecoach.go` touched; the loop body is identical except the one `:=`→`=`.

## User Persona

**Target User**: The NEXT subtask (P1.M2.T1.S2 — wires the FR-T1 multi-turn gate into runPipeline) and,
transitively, the user who runs `stagecoach --dry-run` on a large diff (Issue 1: the dry-run path silently
lacks multi-turn). This task is the variable-scope prerequisite that makes S2's gate possible.

**Use Case**: (internal refactor, no user-visible change yet) `runPipeline` builds `payload` each loop
iteration; after this task the last-built payload is still in scope at the rescue point, where S2's gate
will read it to decide whether to fire multi-turn.

**User Journey**: `--dry-run` → `runPipeline` → generation loop exhausts → (S2, future) FR-T1 gate reads
the hoisted `payload` → multi-turn fires → dry-run succeeds instead of rescue-erroring.

**Pain Points Addressed**: removes the structural blocker (loop-scoped payload) that prevented wiring
multi-turn into the dry-run path. (The user-visible fix lands in S2; this task is the foundation.)

## Why

- **Structural enabler for S2.** The FR-T1 gate (S2) reads the last-built payload at the rescue insertion
  point. A loop-scoped `payload` is invisible there. Hoisting it (mirroring CommitStaged) is the minimal
  change that unblocks S2 without altering loop behavior.
- **Mirrors the proven CommitStaged pattern.** `internal/generate/generate.go:226` already hoists
  `var payload string` before its loop for the same reason (its FR-T1 gate already reads it). runPipeline
  is a pre-multi-turn copy that never got the hoist. This task brings it to parity.
- **Zero risk.** A pure scoping refactor: payload is still rebuilt every iteration; only its lifetime
  changes. The existing `pkg/stagecoach` suite is a strong regression net.
- **Tiny + isolated.** 2 edits in one file; no tests/docs/deps. Keeps S1 and S2 cleanly separable (S1 =
  hoist; S2 = gate) so each is independently reviewable.

## What

One variable hoisted to function scope in `runPipeline`; the loop's `:=` becomes `=`. No gate, no
multi-turn logic, no verbose line, no tests, no docs. The loop behaves identically; only the post-loop
readability of `payload` changes (for S2 to consume).

### Success Criteria

- [ ] `var payload string // hoisted: survives the loop for the FR-T1 multi-turn gate (mirrors CommitStaged)`
      added immediately after `var lastCause error` (line 487) in `runPipeline`.
- [ ] Line 490 changed from `payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)` to
      `payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)`.
- [ ] The rest of the loop body is byte-identical (the `if parseFail { payload = retryInstr + "\n\n" +
      payload }` and the `Render(..., payload, ...)` are unchanged).
- [ ] `gofmt -l pkg/stagecoach/`, `go vet ./pkg/stagecoach/`, `go build ./...` clean; `go test -race
      ./pkg/stagecoach/...` AND `go test -race ./...` green (no behavioral change).
- [ ] go.mod/go.sum unchanged; only `pkg/stagecoach/stagecoach.go` touched; no FR-T1 gate / multiturn.Run /
      verbose line added (those are S2).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the 2-edit recipe (quoted
verbatim below), the line numbers, and the "behavioral unchanged" invariant. No multi-turn/git/provider
knowledge required — this is a variable-scope hoist.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M2T1S1/research/hoist_payload_runpipeline.md
  why: the exact 2 edits (Edit 1 + Edit 2) with line numbers, the CommitStaged reference pattern, the
       "behavioral unchanged" rationale, the no-shadowing proof, and the scope boundary (S1 = hoist;
       S2 = gate).
  critical: this is the ENABLER, not the gate. Do NOT insert the FR-T1 gate / multiturn.Run / verbose line
       — those are P1.M2.T1.S2. S1 ONLY hoists `payload` (2 edits).

- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/docs/architecture/resolution_strategy.md
  section: "ISSUE 1: Dry-Run Path Multi-Turn Propagation" → "Edit 1 — Hoist payload (lines 483–487)" +
           "Edit 2 — Loop body: := → = (line 490)"
  why: the authoritative edit recipe this task implements (Edit 1 + Edit 2). Line 124 is the exact hoist
       line; line 136 is the exact `=` assignment.
  critical: the resolution_strategy scopes Edit 1 + Edit 2 as THIS task (S1); the gate insertion is a
       separate, later step in the same ISSUE 1 section (consumed by S2).

- file: pkg/stagecoach/stagecoach.go
  section: runPipeline var block (483-487) + loop (489) + payload line (490)
  why: the file you edit. var block: `retryInstr`/`rejected`/`candidate,msg`/`parseFail,success`/
       `lastCause`. Loop body: `payload :=` (490) → hoist + `=`.
  pattern: insert the hoist immediately after `var lastCause error` (487), keeping the var block together;
           then `:=` → `=` at 490. Mirror the exact comment text.
  gotcha: the `if parseFail { payload = retryInstr + "\n\n" + payload }` (492) ALREADY uses `=` — after the
           hoist it assigns to the function-scoped payload (same per-iteration effect). Do NOT touch it.

- file: internal/generate/generate.go
  section: CommitStaged var block (~226) — `var payload string // hoisted: ... (D1)` + loop `payload =`
  why: the REFERENCE pattern (the multi-turn-enabled sibling). runPipeline mirrors it. READ to confirm the
       shape; do NOT edit generate.go (P1.M1.T3.S1 owns the CommitStaged verbose line there).
  gotcha: the CommitStaged var block orders things slightly differently (it has `var msg string` and
           `success := false` inline); you do NOT need to reorder runPipeline's block — just ADD the one
           `var payload string` line after `var lastCause error`.

- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M1T3S1/PRP.md
  why: confirms the PARALLEL task touches ONLY `internal/generate/generate.go` (CommitStaged verbose
       progress line) + `internal/generate/multiturn_test.go`. ZERO overlap with pkg/stagecoach/stagecoach.go.
       Its PRP line 18 notes "the line is the copy source for P1.M2.T1.S2" — S2 copies the verbose format,
       not S1; the hoist is independent of it.
```

### Current Codebase tree (relevant slice)

```bash
pkg/stagecoach/stagecoach.go   # runPipeline var block (483-487) + loop payload (490) — EDIT (2 edits: hoist + := → =)
go.mod / go.sum              # unchanged
# NO tests added (pure refactor; existing pkg/stagecoach suite is the regression net).
# NO docs (pure refactor). NO other files.
```

### Desired Codebase tree with files to be added

```bash
# NO new files. ONE edit: pkg/stagecoach/stagecoach.go (hoist `var payload string` + `:=` → `=`).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: this is the ENABLER (S1), not the gate (S2). Do NOT insert the FR-T1 trigger gate, do NOT
// call multiturn.Run, do NOT add a verbose progress line, do NOT touch dedupe. Those are P1.M2.T1.S2.
// S1 is EXACTLY two edits: hoist `var payload string` + `:=` → `=`.

// CRITICAL: behavioral UNCHANGED. payload is STILL rebuilt every iteration — the `payload = prompt.
// BuildUserPayload(...)` assignment remains the first statement in the loop body. Only the SCOPE moves
// (loop → function) so the last-built payload survives for S2's gate. If any existing test changes
// behavior, you accidentally altered the loop body — re-check you only changed `:=` to `=`.

// CRITICAL: keep the hoist's comment text exact: "hoisted: survives the loop for the FR-T1 multi-turn
// gate (mirrors CommitStaged)". It documents WHY the variable is hoisted (otherwise a future reader sees
// a function-scoped var assigned-only-in-the-loop and "tidies" it back to `:=`, re-breaking S2).

// GOTCHA: insert the hoist IMMEDIATELY after `var lastCause error` (line 487), inside the existing var
// block — do NOT scatter declarations. The `if parseFail { payload = retryInstr + "\n\n" + payload }`
// (line 492) ALREADY uses `=`; after the hoist it assigns to the function-scoped payload (same effect).

// GOTCHA: no shadowing. grep confirms `payload` is declared only at line 490 today (→ becomes the hoist);
// the loop body only ASSIGNS. `go vet ./pkg/stagecoach/` is clean now and stays clean (the hoist removes
// the only in-loop declaration, it doesn't add one). If vet warns, you left a `:=` somewhere.

// GOTCHA: do NOT reorder runPipeline's var block to match CommitStaged's ordering. Just ADD the one
// `var payload string` line after `var lastCause error`. Minimal diff.
```

## Implementation Blueprint

### Data models and structure

N/A — no types, no data models. A variable-scope hoist. The "before/after":

```go
// BEFORE (runPipeline, loop-scoped payload — invisible after the loop):
// 487   var lastCause error
// 489   for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
// 490       payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)   // `:=` loop-scoped

// AFTER (function-scoped payload — survives the loop for the FR-T1 gate):
// 487   var lastCause error
// 488   var payload string // hoisted: survives the loop for the FR-T1 multi-turn gate (mirrors CommitStaged)
// 489   for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
// 490       payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)   // `=` (hoisted above)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: stagecoach.go — hoist `var payload string` (Edit 1)
  - ADD immediately after line 487 (`var lastCause error`):
    `var payload string // hoisted: survives the loop for the FR-T1 multi-turn gate (mirrors CommitStaged)`.
  - KEEP it inside the var block (do not scatter). Exact comment text (documents the WHY for future readers).

Task 2: stagecoach.go — loop body `:=` → `=` (Edit 2)
  - CHANGE line 490 from `payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)` to
    `payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)`.
  - DO NOT touch any other line in the loop body (the `if parseFail { payload = retryInstr + "\n\n" +
    payload }` and `Render(..., payload, ...)` are unchanged).

Task 3: VERIFY (no further file change)
  - RUN `gofmt -w pkg/stagecoach/stagecoach.go`; `go vet ./pkg/stagecoach/`; `go build ./...`;
    `go test -race ./pkg/stagecoach/...`; `go test -race ./...`.
  - go.mod/go.sum byte-unchanged. Only pkg/stagecoach/stagecoach.go touched. No gate / multiturn / verbose
    line added (S2's scope).
```

### Implementation Patterns & Key Details

```go
// The hoist — mirrors CommitStaged (internal/generate/generate.go:226) exactly:
var lastCause error
var payload string // hoisted: survives the loop for the FR-T1 multi-turn gate (mirrors CommitStaged)

for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
	payload = prompt.BuildUserPayload(diff, cfg.Context, rejected) // `=` (NOT `:=`) — payload hoisted above
	if parseFail {
		payload = retryInstr + "\n\n" + payload
	}
	// ... Render(..., payload, ...) unchanged ...
}

// After this task: at the rescue insertion point (between `if !success {` and `return &RescueError`),
// `payload` is now READABLE — S2 (P1.M2.T1.S2) inserts the FR-T1 gate that reads it. S1 does NOT insert
// the gate; it only makes the variable visible there.
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — pure refactor; no new dep. go mod tidy MUST be a no-op.

FROZEN / NOT-EDITED:
  - Everything except the 2 edits in pkg/stagecoach/stagecoach.go's runPipeline.
  - The runPipeline loop body (only the `:=`→`=` changes; the parseFail branch + Render call are unchanged).
  - internal/generate/generate.go (P1.M1.T3.S1 owns the CommitStaged verbose line there — parallel, no overlap).
  - internal/hook/exec.go (P1.M3.T1.S1/S2 own the hook-path multi-turn propagation — separate).
  - No tests, no docs.

DOWNSTREAM CONSUMER (do NOT implement here):
  - P1.M2.T1.S2 (next): inserts the FR-T1 multi-turn trigger gate into runPipeline at the rescue insertion
    point (between `if !success {` and `return &RescueError`), reading the hoisted `payload`; adds dedupe
    + the verbose progress line (copying the format P1.M1.T3.S1 finalizes in CommitStaged). This task
    (S1) is the structural prerequisite — it hoists the variable S2 reads.

NO DATABASE / NO ROUTES / NO CONFIG / NO CLI / NO NEW FILES / NO TESTS / NO DOCS.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w pkg/stagecoach/stagecoach.go
test -z "$(gofmt -l pkg/stagecoach/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./pkg/stagecoach/        # THE key gate: catches a shadowing warning or a stray `:=`.
go build ./...                 # Whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: clean. If `go vet` warns about `payload` (declared and not used / shadowing), you either left
# a `:=` in the loop or hoisted without switching to `=` — re-check Edit 2.
```

### Level 2: Unit Tests (Component Validation) — the no-behavioral-change gate

```bash
go test -race ./pkg/stagecoach/...   # the runPipeline generation loop + dry-run path
# Expected: ALL PASS UNCHANGED. payload is still rebuilt every iteration; only its scope changed. These
#   tests exercise the loop (generate→parse→dedupe) and the dry-run success/rescue paths, none of which
#   read post-loop payload — so they are byte-for-byte unaffected.
go test -race ./...                 # full module — no regression.
# Expected: green throughout. This refactor adds NO new behavior, so NO new test is warranted.
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only pkg/stagecoach/stagecoach.go changed, and only the 2 edits:
git diff --stat pkg/stagecoach/stagecoach.go
git diff pkg/stagecoach/stagecoach.go | grep -E '^\+' | grep -v '^\+\+\+'   # the added/changed lines
# Expected: exactly (1) the new `var payload string // hoisted: ...` line and (2) the `payload =` (was `:=`).
#   No other added/changed lines. No gate, no multiturn import, no verbose Fprintf.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Scope audit: confirm NO multi-turn machinery leaked into this refactor (S2's scope).
grep -nE 'multiturn|MultiTurn|FR-T1|trigger gate|falling back to multi-turn' pkg/stagecoach/stagecoach.go
# Expected: NO new matches (S1 is a pure hoist; the gate is S2). Pre-existing references, if any in
# unrelated parts of the file, are fine — this audit is about the DIFF, so check `git diff` instead:
git diff pkg/stagecoach/stagecoach.go | grep -iE 'multiturn|FR-T1|gate' || echo "no gate/multiturn added (correct — S2's scope)"
# golangci-lint: `make lint` (project-wide gate).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `gofmt -l pkg/stagecoach/`, `go vet ./pkg/stagecoach/`, `go build ./...`, `go mod tidy` no-op.
- [ ] Level 2 green: `go test -race ./pkg/stagecoach/...` AND `go test -race ./...` (no behavioral change).
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only `pkg/stagecoach/stagecoach.go` changed; diff is
      exactly the 2 edits (hoist + `:=`→`=`).

### Feature Validation

- [ ] `var payload string // hoisted: survives the loop for the FR-T1 multi-turn gate (mirrors CommitStaged)`
      added after `var lastCause error` (line 487).
- [ ] Line 490 is `payload = prompt.BuildUserPayload(...)` (was `:=`).
- [ ] The rest of the loop body is byte-identical (parseFail branch + Render call unchanged).
- [ ] `payload` is now function-scoped — readable at the rescue insertion point (where S2's gate will read it).

### Code Quality Validation

- [ ] Mirrors the CommitStaged hoist (`internal/generate/generate.go:226`) — same shape, same comment intent.
- [ ] Minimal diff (2 edits); no var-block reordering; no shadowing (go vet clean).
- [ ] No scope creep into the FR-T1 gate / multiturn.Run / verbose line (S2) or other paths (hook = P1.M3).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] No docs (pure refactor; no user-facing/config/API surface change).
- [ ] go.mod/go.sum byte-unchanged; no new files; no new tests.

---

## Anti-Patterns to Avoid

- ❌ Don't insert the FR-T1 multi-turn gate, call `multiturn.Run`, add a verbose line, or touch dedupe. Those
  are P1.M2.T1.S2's 2-point scope. S1 is EXACTLY the 2-edit hoist — the structural enabler, nothing more.
- ❌ Don't change the loop's behavior. payload MUST still be rebuilt every iteration (`payload = prompt.
  BuildUserPayload(...)` as the first statement in the loop body). Only the SCOPE moves (loop → function).
  If a test changes behavior, you accidentally edited the loop body beyond `:=`→`=`.
- ❌ Don't leave the `:=` at line 490 after hoisting `var payload string` — Go will error ("no new variables
  on left side of :=") OR shadow. The hoist (Edit 1) and the `:=`→`=` (Edit 2) are a PAIR; do both.
- ❌ Don't reorder runPipeline's var block to match CommitStaged's ordering. Just ADD the one
  `var payload string` line after `var lastCause error`. Minimal diff.
- ❌ Don't drop or reword the hoist's comment. "hoisted: survives the loop for the FR-T1 multi-turn gate
  (mirrors CommitStaged)" documents the WHY — without it a future reader "tidies" the function-scoped
  var back to `:=`, silently re-breaking the S2 gate that reads it post-loop.
- ❌ Don't touch the `if parseFail { payload = retryInstr + "\n\n" + payload }` line (492). It already uses
  `=`; after the hoist it assigns to the function-scoped payload (same per-iteration effect).
- ❌ Don't add a test. A pure scoping refactor has no new behavior to pin; the existing `pkg/stagecoach`
  suite (which exercises the loop and the dry-run success/rescue paths) is the regression net.
- ❌ Don't touch `internal/generate/generate.go` or `internal/hook/exec.go`. generate.go is P1.M1.T3.S1's
  (parallel, no overlap); hook is P1.M3's. This task is `pkg/stagecoach/stagecoach.go` only.
- ❌ Don't change go.mod/go.sum or add files. Two edits, one file.
- ❌ Don't skip `go vet ./pkg/stagecoach/` — it is THE gate that confirms no shadowing / no stray `:=`. And
  don't skip `go test -race ./pkg/stagecoach/...` — it confirms the loop behaves identically.
