# Research: MeasureAssembled Wiring at Message-Role Sites (P1.M1.T2.S1)

> **Purpose:** Pin the exact edits for wiring the FR3j `MeasureAssembled` closure at the 3 message-role
> consumer call sites, checked against the live codebase on 2026-07-07. **`MeasureAssembled` field is
> LANDED (git.go:81, S1 Complete). The 3 sites are confirmed (generate.go:208, stagecoach.go:437,
> exec.go:123). Prior PRP (S2) wires `closedLoopGate` into `internal/git/git.go` — NOT the consumer
> sites → no conflict.**

---

## 1. Environment & Baseline

| Item | Value / Evidence |
|---|---|
| Module | `github.com/dustin/stagecoach`, `go 1.22` |
| Edit targets | `internal/generate/generate.go`, `pkg/stagecoach/stagecoach.go`, `internal/hook/exec.go` |
| S1 (MeasureAssembled field) | **LANDED** — git.go:81 `MeasureAssembled func(gatedDiff string) int`. |
| S2 (closedLoopGate wiring) | Parallel — wires `closedLoopGate` into the 3 diff functions in `internal/git/git.go`. Does NOT touch the consumer call sites → **no conflict**. |
| The 3 sites | All identical shape: `sysPrompt` built → `reserve := prompt.MessageReserveTokens(sysPrompt, ...)` → `deps.Git.StagedDiff(ctx, git.StagedDiffOptions{... PromptReserveTokens: reserve})`. |
| `sysPrompt` var | All 3 sites name it `sysPrompt` (a local string). |
| Imports | All 3 sites already import `git` (for git.StagedDiffOptions/EstimateTokens) and `prompt` (for MessageReserveTokens/BuildUserPayload). |

---

## 2. The Fix (per the contract)

At each of the 3 sites, BEFORE the `StagedDiffOptions{...}` literal, add a conditional closure:

```go
var measureAssembled func(string) int
if cfg.TokenLimit != 0 {
    measureAssembled = func(gatedDiff string) int {
        return git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, cfg.Context, nil))
    }
}
```

Then add `MeasureAssembled: measureAssembled,` to the `StagedDiffOptions{...}` literal (between
`PromptReserveTokens: reserve,` and the closing `}`).

**Why `nil` for `rejected`:** the diff is captured ONCE before the dedupe loop; at capture time
`rejected` is empty. The `PromptReserveTokens` first-cut estimate already accounts for worst-case
rejection-block growth. The `MeasureAssembled` closure measures the ACTUAL assembled prompt for the
gated diff (the closed-loop re-measurement) — the rejection block is not present at capture time.

**Why conditional on `cfg.TokenLimit != 0`:** when TokenLimit is 0, the gate branch (`> 0`) doesn't
run, so `MeasureAssembled` is never called. Omitting it keeps the legacy path byte-identical (nil
closure → `closedLoopGate` delegates to `applyWaterFillGate` which is also never called when
TokenLimit==0).

**Single-estimator rule:** uses the SAME `git.EstimateTokens` (chars/4) that the water-fill sizing
uses (FR3d/FR3j mandate the same estimator for sizing and measuring).

---

## 3. The 3 Sites (verified against live source)

### 3.1 generate.go:208 (CommitStaged)
```go
sysPrompt, err := buildSystemPrompt(ctx, deps.Git, cfg, isUnborn)   // line ~202
...
reserve := prompt.MessageReserveTokens(sysPrompt, ...)               // line ~205
...
diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{         // line ~208
    MaxDiffBytes:        cfg.MaxDiffBytes,
    ...
    PromptReserveTokens: reserve,
})
```
Insert the closure var BETWEEN the `reserve` line and the `StagedDiff` call. Add `MeasureAssembled:
measureAssembled,` to the literal.

### 3.2 stagecoach.go:437 (runPipeline)
```go
sysPrompt += "\n\n" + systemExtra                                    // line ~432
...
reserve := prompt.MessageReserveTokens(sysPrompt, ...)               // line ~435
...
diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{         // line ~437
    ...
    PromptReserveTokens: reserve,
})
```
Same insertion pattern.

### 3.3 exec.go:123 (hook.Run)
```go
reserve := prompt.MessageReserveTokens(sysPrompt, ...)               // line ~118
...
diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{         // line ~123
    ...
    PromptReserveTokens: reserve,
})
```
Same insertion pattern.

---

## 4. Multi-Turn Re-Capture Calls (NOT touched)

Each site also has a multi-turn re-capture StagedDiff call with `TokenLimit: 0` (generate.go:349,
stagecoach.go:573, exec.go:223). Those do NOT get a `MeasureAssembled` — `TokenLimit: 0` means the gate
doesn't run. Only the one-shot StagedDiff calls (the ones with `TokenLimit: cfg.TokenLimit`) get the
closure.

---

## 5. Decisions Log

| # | Question | Decision | Rationale |
|---|---|---|---|
| D1 | Closure shape? | `func(gatedDiff string) int { return git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, cfg.Context, nil)) }` | Per the contract. Measures the actual assembled prompt (sysPrompt + the payload wrapping the gated diff). |
| D2 | `nil` for rejected? | YES. | Capture-time state: the diff is captured ONCE before the dedupe loop; at capture time rejected is empty. PromptReserveTokens covers worst-case rejection growth. |
| D3 | Conditional on TokenLimit? | YES — only when `cfg.TokenLimit != 0`. | When TokenLimit==0 the gate branch doesn't run; omitting the closure keeps the legacy path byte-identical. |
| D4 | Where to insert? | Between `reserve :=` and the `StagedDiff` call (as a local `var`), then `MeasureAssembled: measureAssembled,` in the literal. | The Go idiom for conditional struct fields: declare a local, conditionally set it, reference it in the literal. |
| D5 | Multi-turn re-capture? | NOT touched. | TokenLimit: 0 → the gate doesn't run → MeasureAssembled is never called. |
| D6 | Scope? | ONLY the 3 one-shot StagedDiff calls. NOT the 3 decompose-role sites (those are P1.M1.T2.S2). NOT tokengate.go (S1/S2's territory). NOT docs/tests (P1.M1.T3). |
