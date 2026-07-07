---
name: "P1.M1.T2.S1 — Wire MeasureAssembled at message-role consumer sites (generate, pkg, hook)"
description: |
  Wire the FR3j closed-loop `MeasureAssembled` closure at the 3 message-role StagedDiff call sites:
  generate.go:208 (CommitStaged), stagecoach.go:437 (runPipeline), exec.go:123 (hook.Run). At each site,
  when `cfg.TokenLimit != 0`, add a closure: `func(gatedDiff string) int { return
  git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, cfg.Context, nil)) }` to the
  StagedDiffOptions literal. The closure measures the ACTUAL assembled prompt (FR3j closed-loop) so the
  gate can re-trim until it fits token_limit. When TokenLimit==0, omit the closure (the gate doesn't run).
  Uses `nil` for rejected (capture-time state; PromptReserveTokens covers worst-case rejection growth).
  Single-estimator rule: same `git.EstimateTokens` used for sizing. S1 landed the field; S2 wires the
  gate function; THIS task wires the closures at the consumers. No docs. Baseline GREEN.
---

## Goal

**Feature Goal**: Wire the FR3j `MeasureAssembled` closure at the 3 message-role consumer sites so the
closed-loop gate (`closedLoopGate`, wired by S2) can measure the ACTUAL assembled prompt
(sysPrompt + BuildUserPayload(gatedDiff)) and re-trim until it fits `token_limit` — closing the loop
that FR3i's open-loop water-fill cannot guarantee alone.

**Deliverable** (3 identical edits in 3 files):
1. `internal/generate/generate.go` (~line 208): add a conditional `MeasureAssembled` closure to the
   CommitStaged `StagedDiffOptions` literal.
2. `pkg/stagecoach/stagecoach.go` (~line 437): same for runPipeline.
3. `internal/hook/exec.go` (~line 123): same for hook.Run.

**Success Definition**: When `cfg.TokenLimit != 0`, each of the 3 one-shot StagedDiff calls passes a
non-nil `MeasureAssembled` closure that measures `EstimateTokens(sysPrompt + BuildUserPayload(gatedDiff,
cfg.Context, nil))`. When `cfg.TokenLimit == 0`, the closure is nil (the gate doesn't run). `go build/vet/
gofmt` clean; `go test ./...` green. The multi-turn re-capture calls (TokenLimit: 0) are untouched.

## User Persona

**Target User**: The user who sets `token_limit` and relies on the FR3j guarantee that the assembled
prompt NEVER exceeds `token_limit`. Also the contributor implementing P1.M1.T3 (the closed-loop invariant
tests that verify the guarantee end-to-end).

**Use Case**: A user sets `token_limit = 120000`. The message-role diff is captured, gated by the
water-fill (FR3i), then the `MeasureAssembled` closure measures the real assembled prompt and re-trims
if it landed over 120000 (FR3j). This task provides the closure that enables that re-measurement.

**Pain Points Addressed**: After S1 (field) + S2 (gate function), the closed-loop is structurally wired
but the consumers pass `nil` for `MeasureAssembled` — the gate delegates to the open-loop path (no
re-measurement). This task injects the actual measurement closure, activating the closed-loop guarantee.

## Why

- **FR3j mandates the closed-loop guarantee.** "After the water-fill produces the gated diff, stagecoach assembles the actual full prompt — the system prompt plus BuildUserPayload(gatedDiff) — measures it with the same EstimateTokens used for sizing, and if it exceeds token_limit, reduces the body budget by the overshoot plus a small slack and re-applies the per-file truncation. Invariant: EstimateTokens(assembledFullPrompt) ≤ token_limit, always."
- **The seam exists (S1) and the gate calls it (S2).** S1 landed `MeasureAssembled func(gatedDiff string) int` on StagedDiffOptions; S2 wired `closedLoopGate` to pass it through. The only missing piece is the consumers INJECTING the closure. Without this task, `MeasureAssembled` is nil at every call site → the closed-loop is dead code.
- **3 identical edits.** Each site already has `sysPrompt` in scope, `cfg.Context` available, and both `git.EstimateTokens` and `prompt.BuildUserPayload` imported. The edit is a mechanical closure + conditional + field addition.
- **No behavioral change when TokenLimit==0.** The closure is nil → `closedLoopGate`'s nil-safe path delegates to `applyWaterFillGate` → the legacy path is byte-identical.

## What

A conditional `MeasureAssembled` closure added to 3 `StagedDiffOptions` literals. When `TokenLimit != 0`,
the closure measures the actual assembled prompt; when `TokenLimit == 0`, the closure is nil (omitted).
No signature changes, no new imports, no docs.

### Success Criteria

- [ ] Each of the 3 one-shot StagedDiff calls (generate.go:208, stagecoach.go:437, exec.go:123) passes a
      non-nil `MeasureAssembled` closure when `cfg.TokenLimit != 0`.
- [ ] The closure body is `func(gatedDiff string) int { return git.EstimateTokens(sysPrompt +
      prompt.BuildUserPayload(gatedDiff, cfg.Context, nil)) }`.
- [ ] When `cfg.TokenLimit == 0`, the closure is nil (the field is not set).
- [ ] The multi-turn re-capture StagedDiff calls (TokenLimit: 0) are UNTOUCHED.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test ./...` green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim StagedDiffOptions literal at each of the 3 sites
(with line numbers), gives the exact closure body, the conditional pattern, the insertion point (between
`reserve :=` and the StagedDiff call), and confirms all variables/imports are in scope. S1's field is
confirmed landed; S2's gate wiring is confirmed parallel (different file).

### Documentation & References

```yaml
# MUST READ — the FR3j spec + the wiring architecture
- file: PRD.md
  why: "§9.1 FR3j (closed-loop budget guarantee): 'after the water-fill produces the gated diff, stagecoach assembles the actual full prompt — the system prompt plus BuildUserPayload(gatedDiff) — measures it with the same EstimateTokens used for sizing, and if it exceeds token_limit, reduces the body budget by the overshoot and re-applies the per-file truncation. Invariant: EstimateTokens(assembledFullPrompt) ≤ token_limit, always.'"
  critical: "FR3j's 'Measure it with the SAME EstimateTokens' is the single-estimator rule (D2). FR3j's 'assembles the actual full prompt' is what the MeasureAssembled closure does."

- docfile: plan/011_98cef660a41d/architecture/fr3j_closed_loop.md
  why: "§'The 6 Consumer Wiring Sites': lists the 3 message-role sites (generate.go:~208, stagecoach.go:~437, exec.go:~123) and the 3 decompose-role sites (planner.go, message.go, arbiter.go — P1.M1.T2.S2's territory). Confirms the closure shape and the nil-rejected rationale."
  critical: "The 6-site table confirms the message-role vs decompose-role split (S1 vs S2). The closure shape + the nil-rejected rationale are the implementation spec."

- docfile: plan/011_98cef660a41d/P1M1T1S2/PRP.md
  why: "The parallel sibling (wires closedLoopGate into the 3 diff functions in git.go). Confirms it does NOT touch the consumer call sites (generate.go, stagecoach.go, exec.go) → no conflict. Confirms closedLoopGate is nil-safe (MeasureAssembled==nil → delegates to applyWaterFillGate)."
  critical: "S2 makes the diff functions CALL closedLoopGate. S1/T2.S1 (THIS task) makes the consumers INJECT the closure. Both are needed for the closed-loop to work. S2 is parallel; the consumer wiring is correct regardless of whether S2 has landed."

- docfile: plan/011_98cef660a41d/P1M1T2S1/research/measure_assembled_wiring_notes.md
  why: "EMPIRICALLY VERIFIED (live code, 2026-07-07): MeasureAssembled field LANDED (git.go:81); the 3 sites confirmed with their exact StagedDiffOptions literals; sysPrompt named at all 3; imports confirmed (git.EstimateTokens + prompt.BuildUserPayload already imported at all 3); multi-turn re-capture calls identified (TokenLimit:0 → NOT touched); decisions D1–D6. READ THIS FIRST."

# The edit targets (verbatim current state at each site)
- file: internal/generate/generate.go
  why: "EDIT (CommitStaged, ~line 208). The StagedDiffOptions literal is at lines 208-216. `sysPrompt` is at line ~202. `reserve` is at line ~205. Insert the closure var between reserve and StagedDiff; add `MeasureAssembled: measureAssembled,` to the literal."
  pattern: "The literal currently ends with `PromptReserveTokens: reserve,` then `})`. Add `MeasureAssembled: measureAssembled,` before the `})`."
  gotcha: "Do NOT touch the multi-turn re-capture StagedDiff call at line ~349 (TokenLimit: 0 — the gate doesn't run there)."

- file: pkg/stagecoach/stagecoach.go
  why: "EDIT (runPipeline, ~line 437). The StagedDiffOptions literal is at lines 437-445. `sysPrompt` is at line ~432 (with systemExtra appended). `reserve` is at line ~435. Same insertion pattern."
  gotcha: "Do NOT touch the multi-turn re-capture StagedDiff call at line ~573 (TokenLimit: 0)."

- file: internal/hook/exec.go
  why: "EDIT (hook.Run, ~line 123). The StagedDiffOptions literal is at lines 123-131. `sysPrompt` is built above (around line ~112). `reserve` is at line ~118. Same insertion pattern."
  gotcha: "Do NOT touch the multi-turn re-capture StagedDiff call at line ~223 (TokenLimit: 0)."

# Read-only refs
- file: internal/git/git.go
  why: "READ-ONLY. StagedDiffOptions.MeasureAssembled field at :81 (S1 LANDED). The field type is `func(gatedDiff string) int`. closedLoopGate (S2 wires it) passes opts.MeasureAssembled through; nil → delegates to applyWaterFillGate (nil-safe)."
- file: internal/prompt/payload.go
  why: "READ-ONLY. `func BuildUserPayload(diff, context string, rejected []string) string` (line 97). The closure calls it with (gatedDiff, cfg.Context, nil)."
- file: internal/git/tokens.go
  why: "READ-ONLY. `func EstimateTokens(s string) int` — the chars/4 estimator. The closure uses the SAME estimator the water-fill sizing uses (single-estimator rule, FR3j)."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── internal/generate/generate.go     # EDIT: CommitStaged StagedDiffOptions (~line 208)
├── pkg/stagecoach/stagecoach.go        # EDIT: runPipeline StagedDiffOptions (~line 437)
└── internal/hook/exec.go             # EDIT: hook.Run StagedDiffOptions (~line 123)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/generate/generate.go     # +MeasureAssembled closure (conditional on TokenLimit!=0)
    pkg/stagecoach/stagecoach.go        # +MeasureAssembled closure
    internal/hook/exec.go             # +MeasureAssembled closure
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/generate/generate.go` | MODIFY | Add the conditional MeasureAssembled closure to CommitStaged's StagedDiffOptions. |
| `pkg/stagecoach/stagecoach.go` | MODIFY | Add the closure to runPipeline's StagedDiffOptions. |
| `internal/hook/exec.go` | MODIFY | Add the closure to hook.Run's StagedDiffOptions. |

**Explicitly NOT touched**: `internal/git/git.go` (S1 field + S2 gate wiring), the 3 multi-turn
re-capture calls (TokenLimit: 0), the 3 decompose-role sites (P1.M1.T2.S2), `internal/git/tokengate.go`
(S1/S2), any tests (P1.M1.T3), any docs, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — the closure captures sysPrompt, cfg.Context — both must be in scope at the StagedDiff
// call site). All 3 sites have `sysPrompt` (a local string) and `cfg` (a config.Config) in scope.
// Verify by reading the lines between the sysPrompt assignment and the StagedDiff call.

// CRITICAL (G2 — nil for rejected, NOT the `rejected` variable). The diff is captured ONCE before the
// dedupe loop; at capture time `rejected` is empty (or not yet populated). The MeasureAssembled closure
// measures the baseline assembled prompt (sysPrompt + payload wrapping the gated diff) WITHOUT a
// rejection block. The PromptReserveTokens first-cut estimate already accounts for worst-case rejection
// growth. Using `nil` matches the capture-time state. Do NOT pass the `rejected` variable — it may not
// be populated yet at the StagedDiff call site.

// CRITICAL (G3 — conditional on cfg.TokenLimit != 0). When TokenLimit is 0, the gate branch (>0)
// doesn't run, so MeasureAssembled is never called. Omitting the closure (nil) keeps the legacy path
// byte-identical. Use a local `var measureAssembled func(string) int` + conditional assignment.

// GOTCHA (G4 — the closure uses the SAME git.EstimateTokens as the sizing). FR3j mandates the
// single-estimator rule: the same chars/4 estimator used for water-fill sizing is used for the
// closed-loop re-measurement. Do NOT use a different estimator (e.g. a real tokenizer).

// GOTCHA (G5 — do NOT touch the multi-turn re-capture StagedDiff calls). Each site has a SECOND
// StagedDiff call (for multi-turn fallback) with `TokenLimit: 0`. Those do NOT get a MeasureAssembled
// (TokenLimit: 0 → the gate doesn't run). Only the ONE-SHOT StagedDiff calls (with TokenLimit:
// cfg.TokenLimit) get the closure.

// GOTCHA (G6 — the 3 edits are byte-identical in shape). Each site has the same StagedDiffOptions
// literal + the same sysPrompt variable name + the same cfg.Context reference. The closure body is
// identical at all 3 sites. Copy-paste the pattern; verify each compiles.
```

## Implementation Blueprint

### Data models and structure

No new types. The `MeasureAssembled func(gatedDiff string) int` field already exists on
`StagedDiffOptions` (S1). This task injects a closure at each consumer site.

### The edit pattern (identical at all 3 sites)

**Before** (current, at each site):
```go
	reserve := prompt.MessageReserveTokens(sysPrompt, cfg.MaxDuplicateRetries, cfg.SubjectTargetChars, cfg.Context, git.EstimateTokens)

	diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
		MaxDiffBytes:        cfg.MaxDiffBytes,
		MaxMDLines:          cfg.MaxMdLines,
		BinaryExtensions:    cfg.BinaryExtensions,
		Excludes:            deps.Excludes,
		TokenLimit:          cfg.TokenLimit,
		DiffContext:         cfg.DiffContextValue(),
		PromptReserveTokens: reserve,
	})
```

**After** (target — insert the closure var + add the field to the literal):
```go
	reserve := prompt.MessageReserveTokens(sysPrompt, cfg.MaxDuplicateRetries, cfg.SubjectTargetChars, cfg.Context, git.EstimateTokens)

	// FR3j closed-loop: when token_limit is set, the gate re-measures the ACTUAL assembled prompt
	// (sysPrompt + BuildUserPayload(gatedDiff)) after water-fill truncation and re-trims until it
	// fits. nil when TokenLimit==0 (the gate branch doesn't run; byte-identical legacy path).
	var measureAssembled func(string) int
	if cfg.TokenLimit != 0 {
		measureAssembled = func(gatedDiff string) int {
			return git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, cfg.Context, nil))
		}
	}

	diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
		MaxDiffBytes:        cfg.MaxDiffBytes,
		MaxMDLines:          cfg.MaxMdLines,
		BinaryExtensions:    cfg.BinaryExtensions,
		Excludes:            deps.Excludes,
		TokenLimit:          cfg.TokenLimit,
		DiffContext:         cfg.DiffContextValue(),
		PromptReserveTokens: reserve,
		MeasureAssembled:    measureAssembled,
	})
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/generate/generate.go — CommitStaged StagedDiffOptions (~line 208)
  - FILE: internal/generate/generate.go
  - LOCATE the `reserve := prompt.MessageReserveTokens(...)` line (~205) followed by the StagedDiff call
    (~208). (If S1/S2 shifted lines, grep for `PromptReserveTokens: reserve` to locate the literal.)
  - INSERT (between reserve and StagedDiff): the `var measureAssembled` + conditional closure (verbatim
    from "The edit pattern" above).
  - ADD `MeasureAssembled: measureAssembled,` to the StagedDiffOptions literal (after
    `PromptReserveTokens: reserve,`).
  - DO NOT: touch the multi-turn re-capture StagedDiff call (~line 349).
  - RUN: gofmt -w internal/generate/generate.go; go build ./internal/generate/

Task 2: EDIT pkg/stagecoach/stagecoach.go — runPipeline StagedDiffOptions (~line 437)
  - FILE: pkg/stagecoach/stagecoach.go
  - Same insertion pattern as Task 1. `sysPrompt` is at ~432; `reserve` at ~435; StagedDiff at ~437.
  - DO NOT: touch the multi-turn re-capture StagedDiff call (~line 573).
  - RUN: gofmt -w pkg/stagecoach/stagecoach.go; go build ./pkg/stagecoach/

Task 3: EDIT internal/hook/exec.go — hook.Run StagedDiffOptions (~line 123)
  - FILE: internal/hook/exec.go
  - Same insertion pattern. `sysPrompt` is built above (~112); `reserve` at ~118; StagedDiff at ~123.
  - DO NOT: touch the multi-turn re-capture StagedDiff call (~line 223).
  - RUN: gofmt -w internal/hook/exec.go; go build ./internal/hook/

Task 4: VALIDATE
  - RUN: gofmt -l .
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test ./...     # full suite green (the closure is nil when TokenLimit==0; no behavior change)
  - FIX-FORWARD: a compile failure = a missing import or wrong variable name; read the error, fix.
```

### Implementation Patterns & Key Details

```go
// === The MeasureAssembled closure (FR3j closed-loop re-measurement) ===
// The gate (closedLoopGate, wired by S2) calls this closure AFTER water-fill truncation to measure
// the ACTUAL assembled prompt size. If it exceeds token_limit, the gate reduces the body budget and
// re-applies truncation (a bounded loop converging in 1-2 passes).
//
// The closure measures: EstimateTokens(sysPrompt + BuildUserPayload(gatedDiff, cfg.Context, nil))
//   sysPrompt:   the system prompt (style examples + instructions) — the non-diff prefix.
//   gatedDiff:   the diff AFTER water-fill truncation (what the gate produced).
//   cfg.Context: the --context free-text hint (part of the payload framing).
//   nil:         rejected is nil (capture-time state — the diff is captured ONCE before the dedupe loop;
//                PromptReserveTokens covers worst-case rejection growth).
//   EstimateTokens: the SAME chars/4 estimator the water-fill sizing uses (single-estimator rule, FR3j).

// === Why conditional on cfg.TokenLimit != 0 ===
// When TokenLimit is 0, the gate branch (>0) in the diff function doesn't run. closedLoopGate is never
// called. The closure is nil → harmless. The conditional avoids constructing the closure (which captures
// sysPrompt) when it will never be called — keeping the legacy path clean.

// === Why the 3 edits are byte-identical ===
// All 3 message-role sites (CommitStaged, runPipeline, hook.Run) have:
//   - `sysPrompt` in scope (built via buildSystemPrompt or equivalent)
//   - `cfg` in scope (config.Config with .TokenLimit, .Context)
//   - `git.EstimateTokens` + `prompt.BuildUserPayload` imported
//   - the same StagedDiffOptions literal shape
// The closure body + the insertion pattern are the same at all 3 sites.
```

### Integration Points

```yaml
PRODUCTION (3 files, 1 edit each):
  - internal/generate/generate.go: +MeasureAssembled closure to CommitStaged's StagedDiffOptions
  - pkg/stagecoach/stagecoach.go:   +MeasureAssembled closure to runPipeline's StagedDiffOptions
  - internal/hook/exec.go:        +MeasureAssembled closure to hook.Run's StagedDiffOptions

CONSUMED (READ-ONLY):
  - internal/git/git.go:81: StagedDiffOptions.MeasureAssembled field (S1 LANDED)
  - internal/git/tokengate.go: closedLoopGate (S2 wires it into the diff functions; nil-safe)
  - internal/prompt/payload.go:97: BuildUserPayload(diff, context, rejected)
  - internal/git/tokens.go: EstimateTokens(s) — the chars/4 estimator

GATE: go build ./... → OK; go test ./... green (closure is nil when TokenLimit==0)

NO-TOUCH (explicitly):
  - internal/git/git.go (S1 field + S2 gate wiring)
  - the 3 multi-turn re-capture StagedDiff calls (TokenLimit: 0)
  - the 3 decompose-role sites (P1.M1.T2.S2 — planner.go, message.go, arbiter.go)
  - internal/git/tokengate.go (S1/S2)
  - tests (P1.M1.T3), docs, PRD.md, tasks.json, prd_snapshot.md, plan/*
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach

gofmt -l .               # Expected: empty
go vet ./...              # Expected: exit 0
go build ./...            # Expected: exit 0 (closure field; no signature change)

# Expected: Zero errors.
```

### Level 2: Whole-Repository Regression

```bash
cd /home/dustin/projects/stagecoach

go test ./...    # Expected: ALL packages green (the closure is nil when TokenLimit==0; no behavior change)

# Confirm ONLY the 3 intended files changed.
git diff --stat -- internal/ pkg/
# Expected: internal/generate/generate.go + pkg/stagecoach/stagecoach.go + internal/hook/exec.go only.
```

### Level 3: Field-Presence Cross-Check

```bash
cd /home/dustin/projects/stagecoach

# The MeasureAssembled closure is present at all 3 one-shot StagedDiff sites.
grep -n "measureAssembled" internal/generate/generate.go pkg/stagecoach/stagecoach.go internal/hook/exec.go
# Expected: 3 matches for the var declaration + 3 for the conditional + 3 for the literal field
#           (9 total — 3 per site).

# The multi-turn re-capture calls do NOT have MeasureAssembled.
grep -B5 "PromptReserveTokens: 0" internal/generate/generate.go pkg/stagecoach/stagecoach.go internal/hook/exec.go | grep "MeasureAssembled" || echo "OK: multi-turn re-capture calls have NO MeasureAssembled"
```

### Level 4: Closed-Loop Activation (when TokenLimit != 0)

```bash
cd /home/dustin/projects/stagecoach

# When TokenLimit != 0, the closure is non-nil → closedLoopGate (S2) re-measures.
# When TokenLimit == 0, the closure is nil → closedLoopGate delegates to applyWaterFillGate (nil-safe).
# Verify the conditional:
grep -A2 "var measureAssembled" internal/generate/generate.go pkg/stagecoach/stagecoach.go internal/hook/exec.go
# Expected: 3 `if cfg.TokenLimit != 0 {` blocks.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test ./...` — all packages green.

### Feature Validation

- [ ] All 3 one-shot StagedDiff calls pass a non-nil `MeasureAssembled` when `cfg.TokenLimit != 0`.
- [ ] The closure body is `func(gatedDiff string) int { return git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, cfg.Context, nil)) }`.
- [ ] When `cfg.TokenLimit == 0`, the closure is nil.
- [ ] The multi-turn re-capture StagedDiff calls (TokenLimit: 0) are UNTOUCHED.

### Scope Discipline Validation

- [ ] ONLY `internal/generate/generate.go` + `pkg/stagecoach/stagecoach.go` + `internal/hook/exec.go` modified.
- [ ] Did NOT touch `internal/git/git.go` (S1/S2), tokengate.go (S1/S2), the decompose-role sites (P1.M1.T2.S2).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Anti-Patterns to Avoid

- ❌ Don't pass the `rejected` variable to `BuildUserPayload` in the closure — use `nil`. The diff is
  captured ONCE before the dedupe loop; at capture time rejected is empty. PromptReserveTokens covers
  worst-case rejection growth. (gotcha G2)
- ❌ Don't omit the `cfg.TokenLimit != 0` conditional — when TokenLimit is 0, the gate branch doesn't
  run, and constructing the closure (which captures sysPrompt) is wasteful. Keep the legacy path clean
  with a nil closure. (G3)
- ❌ Don't use a different estimator than `git.EstimateTokens` (e.g. a real tokenizer). FR3j mandates
  the single-estimator rule — the same chars/4 estimator used for water-fill sizing. (G4)
- ❌ Don't touch the multi-turn re-capture StagedDiff calls — those have TokenLimit: 0 and don't need
  MeasureAssembled. (G5)
- ❌ Don't edit `internal/git/git.go` (S1/S2), the decompose-role sites (P1.M1.T2.S2), tokengate.go, or
  any tests/docs.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is 3 byte-identical edits — each a conditional closure + a struct field addition —
with the exact closure body supplied by the contract, the exact StagedDiffOptions literals quoted from
the live source (generate.go:208, stagecoach.go:437, exec.go:123), all variables confirmed in scope
(`sysPrompt`, `cfg.Context`, `git.EstimateTokens`, `prompt.BuildUserPayload`), and the MeasureAssembled
field confirmed LANDED (git.go:81, S1). The prior parallel PRP (S2) wires the gate function in git.go
(different file → no conflict). When TokenLimit==0, the closure is nil → byte-identical legacy path
(proven by closedLoopGate's nil-safe delegation). Adding a func-typed field to a struct literal is
provably backward-compatible (zero-value). The residual 0.5 uncertainty is purely line-number drift from
the parallel S1/S2 work (if they shifted the StagedDiff call lines) — mitigated by the "grep for
`PromptReserveTokens: reserve` to locate the literal" guidance. The P1.M1.T2.S2 (decompose sites) and
P1.M1.T3 (invariant tests) boundaries are cleanly fenced.
