# Research: hoist payload out of runPipeline generation loop (P1.M2.T1.S1)

Verified against the live codebase. Source of truth for the 2-line structural enabler.

## The structural blocker (why this task exists)

`runPipeline` (`pkg/stagecoach/stagecoach.go`) has its own generation+dedupe loop (the `--dry-run` /
SystemExtra path) that is a pre-multi-turn copy of `CommitStaged`. The FR-T1 multi-turn trigger gate
(landing in P1.M2.T1.S2) must read the **last-built payload** at the rescue insertion point (between
`if !success {` and `return &RescueError`). But `payload` is declared INSIDE the loop with `:=`
(stagecoach.go:490), making it loop-scoped — invisible after the loop. This task hoists it to function
scope so S2's gate can read it. It is the structural prerequisite for S2; it changes NO behavior.

## Current code (runPipeline var block + loop)

`pkg/stagecoach/stagecoach.go`:
```go
// 483   retryInstr := *resolved.RetryInstruction
// 484   var rejected []string
// 485   var candidate, msg string
// 486   var parseFail, success bool
// 487   var lastCause error
// 488   (blank)
// 489   for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
// 490       payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)   // ← loop-scoped `:=`
// 491       if parseFail {
// 492           payload = retryInstr + "\n\n" + payload
// 493       }
// 495       spec, rerr := deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)
```

## The reference (CommitStaged — already hoisted)

`internal/generate/generate.go` (the multi-turn-enabled sibling):
```go
// 226   var payload string   // hoisted: the last-built payload survives the loop for the FR-T1 gate (D1)
// 227   success := false
// 229   for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
// 231       payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)   // `=` (NOT `:=`)
```
This is the exact pattern to mirror in runPipeline.

## The 2 edits (resolution_strategy.md §ISSUE 1, Edit 1 + Edit 2)

**Edit 1** — after line 487 (`var lastCause error`), add:
```go
var payload string // hoisted: survives the loop for the FR-T1 multi-turn gate (mirrors CommitStaged)
```

**Edit 2** — line 490, switch `:=` → `=`:
```go
payload = prompt.BuildUserPayload(diff, cfg.Context, rejected) // `=` (payload hoisted above)
```

## Behavioral unchanged (the keystone invariant)

payload is STILL rebuilt every iteration (the `payload = prompt.BuildUserPayload(...)` assignment is the
first statement in the loop body, unchanged). The ONLY difference is SCOPE: loop-scoped → function-scoped.
After the loop, the last-built payload is now readable at the rescue insertion point. No loop-iteration
behavior changes; existing pkg/stagecoach tests (which exercise the loop, not post-loop payload reads) stay
green byte-for-byte.

## No shadowing risk (verified)

grep of `payload` in stagecoach.go: line 434 (a comment), 490 (`:=` → becomes `=`), 492 (`=`), 495 (read).
After hoisting `var payload string` before the loop, there is exactly ONE `payload` declaration in
runPipeline (the hoist) — the loop body only ASSIGNS to it. `go vet ./pkg/stagecoach/` is clean today
(baseline confirmed); the hoist introduces no new declaration inside the loop, so no shadow warning.
(The `if parseFail { payload = retryInstr + "\n\n" + payload }` at 492 already uses `=` — it assigns to
the function-scoped payload after the hoist, same per-iteration effect.)

## Scope boundary (no conflict)

- **P1.M2.T1.S2 (next)** inserts the FR-T1 multi-turn trigger gate into runPipeline at the rescue insertion
  point and READS the hoisted `payload`. This task (S1) is the structural enabler; S2 is the consumer.
  S1 does NOT insert the gate — that's S2's 2-point scope.
- **P1.M1.T3.S1 (parallel, Implementing)** touches ONLY `internal/generate/generate.go` (the CommitStaged
  verbose progress line format) + `internal/generate/multiturn_test.go`. ZERO overlap with
  `pkg/stagecoach/stagecoach.go`. The hoist is independent of the verbose-line format P1.M1.T3.S1 finalizes
  (line 18 of its PRP: "the line is the copy source for P1.M2.T1.S2" — S2 copies it, not S1).
- This task touches ONLY `pkg/stagecoach/stagecoach.go` (2 edits). No tests added (pure refactor; existing
  tests are the regression net), no docs, no other files.

## DOCS: none — pure variable-scoping refactor; no user-facing/config/API surface change.
