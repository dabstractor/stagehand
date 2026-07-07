---
name: "P1.M1.T1.S1 — Add ChunkCount exported wrapper in multiturn.go"
description: |
  Trivial one-line exported wrapper. `chunkPayload` (multiturn.go:52) is unexported and consumed only
  internally by the N+1 turn protocol. Cross-package callers (`pkg/stagecoach.runPipeline`, `internal/hook`)
  need the chunk count for their progress-message turn-count computation but can't call the unexported
  function. Add `ChunkCount(payload string, chunkTokens int) int` immediately after `chunkPayload` (before
  `advanceRunes`); body is `return len(chunkPayload(payload, chunkTokens))`. Do NOT export the `chunk`
  type. Add pure unit tests asserting `ChunkCount == len(chunkPayload(...))`. Consumed by P1.M2.T1.S2
  (runPipeline progress) and P1.M3.T1.S2 (hook progress). No docs.
---

## Goal

**Feature Goal**: Export a thin cross-package helper (`generate.ChunkCount`) that gives `pkg/stagecoach` and
`internal/hook` the multi-turn chunk count for progress-message turn-count computation, without exporting
the internal `chunk` type or the `chunkPayload` function itself.

**Deliverable** (1 exported function + 5 pure unit tests):
1. `internal/generate/multiturn.go`: add `func ChunkCount(payload string, chunkTokens int) int` immediately
   after `chunkPayload` (~line 93, before `advanceRunes`); body: `return len(chunkPayload(payload, chunkTokens))`.
2. `internal/generate/multiturn_test.go`: add `TestChunkCount_*` cases verifying `ChunkCount == len(chunkPayload(...))`
   for empty payload (→1), single-chunk under budget (→1), multi-chunk (→N), sub-1 budget (→1), CJK-heavy.

**Success Definition**: `generate.ChunkCount` is callable from `pkg/stagecoach` and `internal/hook`; it
returns the same count as `len(chunkPayload(...))` for every input; the `chunk` type and `chunkPayload`
remain unexported; `go build/vet/gofmt` clean and `go test -race ./...` green.

## Why

- **Unblocks the dry-run + hook multi-turn propagation (Issues 1 & 2).** Both `runPipeline`
  (`pkg/stagecoach`) and `hook.Run` (`internal/hook`) need the turn count for the progress line
  (`"↳ falling back to multi-turn: %d turns, ~%dm total"`) — which S2 (P1.M2.T1.S2) and S2 (P1.M3.T1.S2)
  will wire. Without an exported helper they'd have to duplicate the chunk-count logic or reach into
  unexported symbols.
- **Minimal, zero-risk wrapper.** It's a one-line delegation to an already well-tested function
  (`chunkPayload`, 9 `TestChunkPayload_*` tests). No new logic; no new code paths; `chunkPayload` itself
  is untouched.
- **Doesn't leak internals.** The `chunk` struct (with `index`/`total`/`text` fields) stays unexported;
  only the integer count crosses the package boundary — exactly what a progress line needs.

## What

A one-line exported function + 5 pure unit tests. No caller wiring (S2's job), no docs.

### Success Criteria

- [ ] `ChunkCount(payload string, chunkTokens int) int` exists immediately after `chunkPayload`.
- [ ] Body is `return len(chunkPayload(payload, chunkTokens))` — delegates, no duplication.
- [ ] Godoc comment explains it's for cross-package progress-message turn-count computation.
- [ ] The `chunk` type and `chunkPayload` remain unexported.
- [ ] `TestChunkCount_*` passes for: empty→1, single-chunk→1, multi-chunk→N matching chunkPayload, sub-1 budget→1, CJK→matching.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.

## All Needed Context

### Context Completeness Check

**Yes.** This PRP quotes the exact `chunkPayload` signature + the insertion point (after it, before
`advanceRunes`). The function body is one line. The tests are pure table cases matching the existing
`TestChunkPayload_*` style.

### Documentation & References

```yaml
- file: internal/generate/multiturn.go
  why: "EDIT. chunkPayload at :52 (unexported, well-tested). chunk type at :31 (unexported — keep it so). advanceRunes at :95. Insert ChunkCount between chunkPayload's close and advanceRunes (~line 93)."
  pattern: "One-line wrapper: `func ChunkCount(payload string, chunkTokens int) int { return len(chunkPayload(payload, chunkTokens)) }`. Godoc explaining it's for cross-package progress turn-count computation."
  gotcha: "Do NOT export chunkPayload itself or the chunk type — only the integer count crosses the boundary. Do NOT modify chunkPayload."

- file: internal/generate/multiturn_test.go
  why: "EDIT. Same package (package generate). Reuse the existing string-literal payloads + the `chunkPayload` function (in-package call) as the oracle. Mirror TestChunkPayload_EmptyPayload/SingleChunk/MultiChunk/RuneBasedCJK for the test inputs."
  pattern: "Pure table tests: `got := ChunkCount(payload, budget); want := len(chunkPayload(payload, budget)); if got != want { t.Errorf(...) }`. No I/O, no t.TempDir, no testify."

- file: internal/generate/generate.go # :332
  why: "READ-ONLY ref — the EXISTING in-package caller. Line 332: `turns := len(chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1`. This is the exact pattern the cross-package callers (runPipeline/hook) will use via ChunkCount — S1 provides the exported symbol they need."

- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/architecture/system_context.md
  why: "§4 documents the chunkPayload export constraint (the function is unexported; cross-package callers can't reach it). Confirms ChunkCount is the recommended export shape."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/generate/
    ├── multiturn.go        # EDIT: + ChunkCount (after chunkPayload, before advanceRunes)
    └── multiturn_test.go   # EDIT: + TestChunkCount_* (5 cases)
```

### Desired Codebase Tree After S1

```bash
stagecoach/
└── (only existing files modified)
    internal/generate/multiturn.go        # +func ChunkCount
    internal/generate/multiturn_test.go   # +TestChunkCount_*
```

### Known Gotchas

```go
// CRITICAL: ChunkCount delegates to chunkPayload — it MUST NOT re-implement the chunking logic.
// The whole point is that cross-package callers get the EXACT SAME count chunkPayload produces,
// so the progress line's turn count is always consistent with the actual N+1 turn protocol.

// GOTCHA: the chunk type stays unexported. Only the int count crosses the package boundary.
// Exporting chunk would leak the internal "PART i/N:" prefix detail to pkg/stagecoach/hook, which
// neither caller needs (they only want the count for the progress line).

// GOTCHA: placement — immediately AFTER chunkPayload (line ~93), BEFORE advanceRunes (line 95).
// Keeping ChunkCount next to the function it wraps is the natural reading order.

// GOTCHA: test style — same package (package generate), so chunkPayload is callable in the test
// as the oracle: `want := len(chunkPayload(payload, budget))`. Pure string-arithmetic table cases.
```

## Implementation Blueprint

### Implementation Tasks

```yaml
Task 1: ADD ChunkCount in internal/generate/multiturn.go (after chunkPayload, before advanceRunes)
  - LOCATE: the closing brace of chunkPayload (~line 93), immediately before `func advanceRunes` (line 95).
  - INSERT:
        // ChunkCount returns the number of chunks chunkPayload would split payload into at the given
        // chunkTokens budget. It is the exported cross-package helper for progress-message turn-count
        // computation (the progress line prints N+1 turns where N = ChunkCount). It delegates to the
        // unexported chunkPayload — the single source of truth for chunk sizing — so the count is always
        // consistent with the actual N+1 turn protocol. Pure; no I/O.
        func ChunkCount(payload string, chunkTokens int) int {
            return len(chunkPayload(payload, chunkTokens))
        }
  - DO NOT: modify chunkPayload; export the chunk type; add any logic beyond the delegation.

Task 2: ADD TestChunkCount_* in internal/generate/multiturn_test.go (5 pure cases)
  - PLACEMENT: alongside the existing TestChunkPayload_* tests.
  - PATTERN: each case computes `got := ChunkCount(payload, budget)` and `want := len(chunkPayload(payload, budget))`
    and asserts equality. Mirror the existing TestChunkPayload_* inputs:
  - (a) TestChunkCount_EmptyPayload: payload="" → ChunkCount==1 (chunkPayload yields 1 empty chunk).
  - (b) TestChunkCount_SingleChunk: small payload, large budget → ChunkCount==1.
  - (c) TestChunkCount_MultiChunk: large payload, small budget → ChunkCount == len(chunkPayload(...)) (>1).
  - (d) TestChunkCount_SubOneBudget: budget=0 or -1 → ChunkCount==1 (defensive collapse; chunkPayload clamps).
  - (e) TestChunkCount_CJK: CJK-heavy payload → ChunkCount == len(chunkPayload(...)).
  - DO NOT: use I/O, t.TempDir, or testify. These are pure string-arithmetic assertions.

Task 3: VALIDATE
  - RUN: gofmt -w internal/generate/multiturn.go internal/generate/multiturn_test.go
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test -race ./internal/generate/ -v -run TestChunkCount
  - RUN: go test -race ./...   # full suite green
```

### Implementation Patterns & Key Details

```go
// === the complete production change (one function) ===
// ChunkCount returns the number of chunks chunkPayload would split payload into at the given
// chunkTokens budget. It is the exported cross-package helper for progress-message turn-count
// computation (the progress line prints N+1 turns where N = ChunkCount). It delegates to the
// unexported chunkPayload — the single source of truth for chunk sizing — so the count is always
// consistent with the actual N+1 turn protocol. Pure; no I/O.
func ChunkCount(payload string, chunkTokens int) int {
	return len(chunkPayload(payload, chunkTokens))
}
```

```go
// === test pattern (each case) ===
	got := ChunkCount(payload, budget)
	want := len(chunkPayload(payload, budget))
	if got != want {
		t.Errorf("ChunkCount(%q, %d) = %d, want %d (len(chunkPayload))", payload, budget, got, want)
	}
```

### Integration Points

```yaml
EXPORTED SYMBOL (internal/generate/multiturn.go):
  - func ChunkCount(payload string, chunkTokens int) int  # delegates to len(chunkPayload(...))

UNEXPORTED (unchanged):
  - type chunk struct { ... }           # stays internal
  - func chunkPayload(...) []chunk      # stays internal; ChunkCount delegates to it

CONSUMERS (downstream — NOT in S1):
  - P1.M2.T1.S2: pkg/stagecoach.runPipeline progress line: `turns := generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) + 1`
  - P1.M3.T1.S2: internal/hook progress line (same pattern)

NO-TOUCH: generate.go (the existing in-package caller at :332 keeps using chunkPayload directly — it's
same-package); any other file; docs; PRD.md, tasks.json, prd_snapshot.md, plan/*
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach
gofmt -w internal/generate/multiturn.go internal/generate/multiturn_test.go
gofmt -l .            # Expected: empty
go vet ./internal/generate/...  # Expected: exit 0
go build ./...        # Expected: exit 0
```

### Level 2: Unit Tests

```bash
cd /home/dustin/projects/stagecoach
go test -race ./internal/generate/ -v -run TestChunkCount   # 5 cases pass
go test -race ./internal/generate/ -v                       # full generate suite green
```

### Level 3: Whole-Repository Regression

```bash
cd /home/dustin/projects/stagecoach
go test -race ./...   # Expected: ALL packages green (S1 adds a wrapper; no behavior change)
git diff --stat       # Expected: internal/generate/multiturn.go + multiturn_test.go only
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.

### Feature Validation
- [ ] `ChunkCount(payload string, chunkTokens int) int` exists after `chunkPayload`.
- [ ] Body delegates to `len(chunkPayload(payload, chunkTokens))`.
- [ ] `chunk` type and `chunkPayload` remain unexported.
- [ ] 5 `TestChunkCount_*` cases pass (empty→1, single→1, multi→N, sub-1→1, CJK→matching).

### Scope Discipline
- [ ] ONLY `internal/generate/{multiturn,multiturn_test}.go` modified.
- [ ] Did NOT wire into `runPipeline` (P1.M2.T1.S2) or `hook.Run` (P1.M3.T1.S2).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Anti-Patterns to Avoid

- ❌ Don't re-implement the chunking logic in ChunkCount — it MUST delegate to `chunkPayload` so the count
  is always consistent with the actual N+1 turn protocol. A re-implementation would risk drift.
- ❌ Don't export `chunkPayload` itself or the `chunk` type — only the int count is needed cross-package.
  Exporting them would leak internal details (the "PART i/N:" prefix shape) to callers that don't need it.
- ❌ Don't modify `chunkPayload` — it's tested and correct. S1 adds a wrapper, not a change.
- ❌ Don't wire ChunkCount into runPipeline or hook here — those are P1.M2.T1.S2 / P1.M3.T1.S2.

---

## Confidence Score

**10/10** — a one-line exported wrapper delegating to a well-tested function, with 5 pure table-test
cases that use the function itself as the oracle. There is literally nothing that can go wrong beyond a
typo (caught by `go build`).
