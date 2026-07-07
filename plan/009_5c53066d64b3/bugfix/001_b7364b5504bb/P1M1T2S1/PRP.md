---
name: "P1.M1.T2.S1 — Rebuild mtPayload from diff (not payload) when TokenLimit==0 in CommitStaged gate"
description: |
  One-line bugfix (Issue 4 — Minor): the CommitStaged multi-turn gate at generate.go:311 sets
  `mtPayload := payload`, but `payload` may carry the retryInstr preamble (from the last failed one-shot
  parse attempt at L233: `payload = retryInstr + "\n\n" + payload`). The TokenLimit≠0 path (L323) rebuilds
  mtPayload from the untruncated diff WITHOUT retryInstr — so the two paths are inconsistent. Fix: replace
  L311 with `mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)` — rebuild from the existing
  untruncated `diff` variable (in scope at L193) without retryInstr, matching the TokenLimit≠0 path. Update
  the comment at L307. Add a test mirroring TestMultiTurnGate_TokenLimitTruncated_Recaptures for the
  TokenLimit==0 + parseFail case, asserting the mtPayload lacks the retryInstr preamble + multi-turn succeeds.
  No docs (internal logic fix aligning code with the already-correct FR-T2 docs). Baseline GREEN.
---

## Goal

**Feature Goal**: Eliminate the mtPayload inconsistency (Issue 4) where the TokenLimit==0 multi-turn path
includes the one-shot retryInstr preamble in the multi-turn payload (while the TokenLimit≠0 path does not),
by always rebuilding mtPayload from the untruncated `diff` variable via `prompt.BuildUserPayload`.

**Deliverable** (1 one-line production fix + 1 comment update + 1 test):
1. `internal/generate/generate.go:311` — replace `mtPayload := payload` with
   `mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)`.
2. `internal/generate/generate.go` ~L307 — update the comment to reflect mtPayload is ALWAYS rebuilt from
   diff (not just in the TokenLimit≠0 branch).
3. `internal/generate/multiturn_test.go` — add a test mirroring `TestMultiTurnGate_TokenLimitTruncated_Recaptures`
   for the TokenLimit==0 + parseFail case.

**Success Definition**: `go test -race ./internal/generate/` green; the mtPayload passed to the multi-turn
protocol never contains the retryInstr preamble, regardless of TokenLimit; multi-turn still fires + succeeds
on a TokenLimit==0 large diff with a forced one-shot parse failure. No file outside generate.go +
multiturn_test.go touched.

## User Persona

**Target User**: The user whose large diff triggers the multi-turn fallback (§9.24). When the one-shot path
exhausts on a parse failure and multi-turn activates, the model should receive a CLEAN diff payload (no
stale retryInstr preamble from the failed one-shot attempt) so the multi-turn priming protocol works as
designed (FR-T4). Also the contributor copying this corrected gate into runPipeline/hook (P1.M2/P1.M3).

**Use Case**: A user with a large diff (TokenLimit unset) runs `stagecoach`; the one-shot attempt fails to
parse (empty output); multi-turn fires. Before the fix, the multi-turn payload carried the retryInstr
preamble ("Output ONLY the commit message. No preamble, no markdown, no quotes.") prepended to the diff,
which conflicts with the multi-turn priming preamble ("reply with exactly: ok"). After the fix, the
multi-turn payload is a clean rebuild from the untruncated diff.

**Pain Points Addressed**: Eliminates a confusing double-instruction in the multi-turn payload (the
retryInstr is one-shot-only; multi-turn has its own preamble per FR-T4). Ensures both TokenLimit paths
(==0 and ≠0) produce identical, retryInstr-free mtPayload.

## Why

- **Aligns code with the already-correct FR-T2 documentation.** FR-T2 ("the multi-turn payload is the SAME captured payload the one-shot path would send") and docs/how-it-works.md:262-298 describe a clean diff rebuild. The code's TokenLimit==0 path violates this by reusing the `payload` variable (which carries retryInstr after a parse failure).
- **One-line fix, zero behavioral risk on the happy path.** Rebuilding from `diff` (which is what `payload` was originally built from at L231, minus the retryInstr) produces the same content as `payload` when parseFail is false. The fix only changes behavior when parseFail is true (stripping the retryInstr) — which is the exact case multi-turn fires on.
- **The corrected gate is the copy source for P1.M2/P1.M3.** The runPipeline (dry-run) and hook.Run gates will be copied from this corrected CommitStaged gate. Fixing it here ensures the copies inherit the correct logic.
- **The architecture docs prescribe the exact fix.** `resolution_strategy.md` ISSUE 4 (lines 39-60) gives the verbatim before/after; `research_commitstaged_reference.md` §4 (lines 117-142) analyzes the mtPayload variable.

## What

A one-line replacement at generate.go:311 + a comment update at ~L307 + a mirroring test. No signature
changes, no caller changes, no docs, no other package.

### Success Criteria

- [ ] generate.go:311 is `mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)` (NOT `mtPayload := payload`).
- [ ] The comment at ~L307 reflects that mtPayload is ALWAYS rebuilt from `diff` (not "we use [payload] directly").
- [ ] A new test in multiturn_test.go mirrors `TestMultiTurnGate_TokenLimitTruncated_Recaptures` for TokenLimit==0 + parseFail.
- [ ] The test asserts multi-turn fires + succeeds (commit lands).
- [ ] The test asserts the mtPayload does NOT contain the retryInstr preamble.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] No file outside `internal/generate/generate.go` + `internal/generate/multiturn_test.go` modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim buggy line (generate.go:311), the verbatim fix, the
payload-hoist/retryInstr logic (L227-233), the diff capture (L193), the BuildUserPayload signature, the
existing test to mirror (multiturn_test.go:618, full body quoted), the retryInstr-specific assertion
substring, and the architecture doc references. The one-line fix is copy-paste-ready.

### Documentation & References

```yaml
# MUST READ — the bug analysis + the exact fix
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/docs/architecture/resolution_strategy.md
  why: "ISSUE 4 (lines 30-64): the exact bug location (line 311), the problem (payload may carry retryInstr from the last failed one-shot attempt), the verbatim before/after fix, and the test prescription ('mirror TestMultiTurnGate_TokenLimitTruncated_Recaptures but for TokenLimit==0 + parseFail')."
  critical: "Lines 42-53 give the verbatim before (`mtPayload := payload`) and after (`mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)`) + the corrected comment text. Lines 62-64 give the test spec."

- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/docs/architecture/research_commitstaged_reference.md
  why: "§4 (lines 117-142): the mtPayload variable analysis — L311 `mtPayload := payload` (DEFAULT: reuse the one-shot payload), L312 `if cfg.TokenLimit != 0` (FR-T12 re-capture branch), L323 `mtPayload = BuildUserPayload(fullDiff, ...)` (rebuild without retryInstr). Confirms both the bug and the fix path."

- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M1T2S1/research/mtpayload_fix_notes.md
  why: "EMPIRICALLY VERIFIED (live code, 2026-07-05): the bug at L311 confirmed; baseline GREEN; the fix + comment update; the test design (mirror the existing pattern at :618 — NOT :418 as the contract's stale line said); the retryInstr-specific substring for the assertion; decisions D1–D5. READ THIS FIRST."

# The edit target
- file: internal/generate/generate.go
  why: "EDIT (2 spots). (a) Line 311: `mtPayload := payload` → `mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)`. (b) Comment at ~L307: update to reflect mtPayload is ALWAYS rebuilt from diff. The `diff` variable (L193), `cfg.Context`, and `rejected` are all in scope at L311."
  pattern: "The TokenLimit≠0 branch (L312-326) already rebuilds mtPayload via `prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)` at L323 — the fix makes the TokenLimit==0 path use the same rebuild (from `diff` instead of `fullDiff`, since when TokenLimit==0 `diff` IS already untruncated)."
  gotcha: "`diff` (L193) is the untruncated staged diff captured BEFORE the one-shot loop. When TokenLimit==0, StagedDiff used TokenLimit=cfg.TokenLimit=0, so `diff` is untruncated. When TokenLimit≠0, the re-capture branch (L312-326) overwrites mtPayload with the re-captured fullDiff version. So the fix is safe for both paths."

- file: internal/generate/multiturn_test.go
  why: "EDIT (1 new test). Mirror `TestMultiTurnGate_TokenLimitTruncated_Recaptures` (at :618 — NOT :418; S1's ChunkCount tests shifted the line). The existing test exercises the TokenLimit≠0 re-capture path; the new test exercises the TokenLimit==0 + parseFail path."
  pattern: "Existing test shape: stubtest.Build(t) → initRepo + commitRaw → writeFile + stageFile (large diff) → stubAppendManifest(t, bin, []string{...}, false) → cfg := config.Defaults() + cfg.MaxDuplicateRetries=0 + cfg.MultiTurnChunkTokens=small + cfg.TokenLimit=value → CommitStaged(ctx, Deps{...}, cfg) → assert buf.Contains('multi-turn fallback') + err==nil."
  gotcha: "The new test uses cfg.TokenLimit=0 (the bug's trigger). The stub script's call 1 returns \"\" (empty → parseFail → retryInstr prepended to `payload`). Calls 2..N return \"ok\"; the final call returns a valid message. Assert multi-turn fires + succeeds AND the mtPayload lacks the retryInstr preamble."

# Read-only refs
- file: internal/prompt/payload.go
  why: "READ-ONLY. `func BuildUserPayload(diff, context string, rejected []string) string` (line 97). Confirms the signature: takes (diff, cfg.Context, rejected) and returns the user payload string WITHOUT retryInstr (retryInstr is prepended by the CALLER at L233, not by BuildUserPayload)."
- file: internal/generate/multiturn_test.go
  why: "READ-ONLY ref. `TestMultiTurnGate_TokenLimitTruncated_Recaptures` (:618) is the template. `stubAppendManifest` and `stubtest.Build` are the test helpers. The `initRepo`/`commitRaw`/`writeFile`/`stageFile` helpers are the same-package test fixtures."

# External reference
- url: https://pkg.go.dev/strings#Builder
  why: "Confirm strings.Builder usage in the test's large-diff construction (strings.Repeat). The existing test uses `strings.Repeat(\"change line\\n\", 8)` for a ~96-rune diff."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/generate/
    ├── generate.go         # EDIT: line 311 (mtPayload rebuild) + comment ~L307
    └── multiturn_test.go   # EDIT: +1 test (TokenLimit==0 + parseFail mirror)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/generate/generate.go         # L311: mtPayload := prompt.BuildUserPayload(diff, ...) + comment
    internal/generate/multiturn_test.go   # +TestMultiTurnGate_TokenLimitZero_ParseFail_NoRetryInstr
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/generate/generate.go` | MODIFY (1 line + comment) | Replace `mtPayload := payload` (L311) with `mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)`; update comment at ~L307. **Only production file.** |
| `internal/generate/multiturn_test.go` | MODIFY (append 1 test) | Add a test mirroring `TestMultiTurnGate_TokenLimitTruncated_Recaptures` for TokenLimit==0 + parseFail, asserting mtPayload lacks retryInstr + multi-turn succeeds. |

**Explicitly NOT touched**: `internal/generate/multiturn.go` (S1's ChunkCount — parallel),
`internal/generate/rescue.go`/`dedupe.go` (unaffected), `pkg/stagecoach/stagecoach.go` (P1.M2 — runPipeline),
`internal/hook/exec.go` (P1.M3 — hook), `docs/*` (P1.M4), any other package, `PRD.md`, `tasks.json`,
`prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — the fix rebuilds from `diff`, NOT from `payload`): `diff` (L193) is the untruncated
// staged diff captured BEFORE the one-shot loop. It does NOT carry retryInstr (retryInstr is prepended to
// `payload` at L233, not to `diff`). So `BuildUserPayload(diff, cfg.Context, rejected)` produces a clean
// payload without retryInstr. Do NOT use `payload` — that's the bug.

// CRITICAL (G2 — update the comment at L307): the current comment says "When token_limit is unset,
// `payload` is already untruncated (derived from the untruncated diff) so we use it directly and avoid
// the second StagedDiff call." This is now WRONG — we rebuild from `diff` (not reuse `payload`), precisely
// to strip any retryInstr. Update the comment to reflect this.

// CRITICAL (G3 — the retryInstr-specific assertion substring): the test must check for a substring UNIQUE
// to the retryInstr, NOT the ambiguous "Output ONLY the commit message" (which also appears in the
// multi-turn final-turn prompt "Output ONLY the message"). Use "No preamble, no markdown, no quotes."
// (the retryInstr-specific tail) for an unambiguous assertion.

// GOTCHA (G4 — the test-to-mirror is at :618, NOT :418): the contract cited multiturn_test.go:418-463,
// but S1 (P1.M1.T1.S1, parallel "Implementing") added ChunkCount tests at :415+ which shifted the line
// numbers. `TestMultiTurnGate_TokenLimitTruncated_Recaptures` is now at :618. Grep for it; do not trust
// the contract's stale line number.

// GOTCHA (G5 — observing mtPayload content in the integration test): mtPayload is a local variable inside
// CommitStaged — not directly inspectable from tests. The test observes the multi-turn payload's content
// via `STAGECOACH_STUB_STDINFILE` (the stub writes received stdin to that file) or via the verbose turn-
// count (the retryInstr would inflate the payload → more chunks → more turns). The implementing agent
// should verify the observation path works; the key property is: with the fix, no multi-turn chunk
// contains "No preamble, no markdown, no quotes."

// GOTCHA (G6 — TokenLimit=0 means the re-capture branch is SKIPPED): the test MUST use cfg.TokenLimit=0
// so the `if cfg.TokenLimit != 0` branch (L312) is NOT entered — that's the only way to exercise the
// L311 path (the bug). With TokenLimit≠0, L323 overwrites mtPayload and the bug is invisible.

// GOTCHA (G7 — force parseFail): the one-shot must FAIL parsing so multi-turn fires. The stub script's
// call 1 returns "" (empty → ParseOutput ok=false → parseFail=true → retryInstr prepended to `payload`
// at L233). MaxDuplicateRetries=0 means exactly 1 one-shot attempt, then exhausted → multi-turn fires.
```

## Implementation Blueprint

### Data models and structure

No data-model change. The fix is a single assignment replacement. The `diff` (string), `cfg.Context`
(string), and `rejected` ([]string) variables are all in scope at L311.

### The edit (exact — current → target)

**generate.go:311** (inside the FR-T1 multi-turn gate):
```go
// CURRENT (the bug — payload may carry retryInstr from the last failed one-shot parse attempt)
			mtPayload := payload

// TARGET (rebuild from the untruncated diff WITHOUT retryInstr)
			mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
```

**generate.go ~L307** (comment update):
```go
// CURRENT (excerpt)
			// FR-T12 re-capture: if token_limit is set, the one-shot `payload` is truncated and unsuitable.
			// Re-capture the diff honoring FR-T12 (TokenLimit=0) and rebuild the payload from it. When
			// token_limit is unset, `payload` is already untruncated (derived from the untruncated diff)
			// so we use it directly and avoid the second StagedDiff call.

// TARGET (excerpt — reflect that mtPayload is ALWAYS rebuilt from diff)
			// FR-T2/FR-T12: mtPayload is ALWAYS rebuilt from the untruncated `diff` via BuildUserPayload
			// (NOT reused from the one-shot `payload`, which may carry the retryInstr corrective preamble
			// from a failed parse — multi-turn has its own priming preamble, FR-T4). When token_limit is
			// set (non-zero), the one-shot `diff` was truncated, so we RE-CAPTURE with TokenLimit=0 below
			// and rebuild from the untruncated fullDiff. When token_limit is unset (0), `diff` is already
			// untruncated, so we rebuild from it directly (avoids a redundant StagedDiff call).
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: FIX generate.go:311 — rebuild mtPayload from diff
  - FILE: internal/generate/generate.go
  - LOCATE line 311 inside the FR-T1 multi-turn gate (the `mtPayload := payload` line).
  - REPLACE with: `mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)`
  - VERIFY: `diff`, `cfg.Context`, `rejected` are in scope (they are — diff at L193, cfg/rejected are
    function params/locals). `prompt` is already imported (used at L231).
  - DO NOT: change the TokenLimit≠0 branch (L312-326); change any other line; touch the condition checks.

Task 2: UPDATE the comment at generate.go ~L307
  - LOCATE the comment block above L311 (starts ~L307 with "FR-T12 re-capture: if token_limit is set...").
  - REPLACE with the target comment (above) that reflects mtPayload is ALWAYS rebuilt from diff.
  - DO NOT: change the FR-T1/FR-T12 comments at L291-303 (the condition explanations); those are correct.

Task 3: ADD the test to multiturn_test.go
  - FILE: internal/generate/multiturn_test.go
  - PLACE: after `TestMultiTurnGate_TokenLimitTruncated_Recaptures` (at :618 — grep for it; do NOT trust
    the contract's stale :418 line number).
  - WRITE a test `TestMultiTurnGate_TokenLimitZero_ParseFail_NoRetryInstr` mirroring the existing test:
      - cfg.TokenLimit = 0 (G6 — exercises the L311 path, NOT the re-capture branch).
      - Large diff: writeFile(t, repo, "new.txt", strings.Repeat("change line\n", 8)) + stageFile.
      - cfg.MaxDuplicateRetries = 0 (one-shot: 1 attempt, then exhausted → multi-turn fires).
      - cfg.MultiTurnChunkTokens = 4 (small so the untruncated diff exceeds one chunk → condition (b) true).
      - stubAppendManifest(t, bin, []string{"", "ok", ..., "feat: mt win"}, false) — call 1 = "" (parseFail;
        retryInstr prepended to `payload`); calls 2..N = "ok"; final = a valid message.
      - Call CommitStaged(ctx, Deps{Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&buf, true)}, cfg).
      - ASSERT: buf.Contains("multi-turn fallback") (multi-turn fired).
      - ASSERT: err == nil (commit landed — multi-turn succeeded).
      - ASSERT: the mtPayload delivered to the multi-turn protocol does NOT contain the retryInstr preamble.
        Use the retryInstr-specific substring "No preamble, no markdown, no quotes." (G3 — NOT the ambiguous
        "Output ONLY"). Observe via STAGECOACH_STUB_STDINFILE (the env var on the stub's Env map that captures
        received stdin) or the verbose turn-count. (G5 — verify the observation path works.)
  - DO NOT: change the existing TestMultiTurnGate_* tests; use testify; change the stub binary.

Task 4: VALIDATE
  - RUN: gofmt -w internal/generate/generate.go internal/generate/multiturn_test.go
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: go test -race ./internal/generate/ -v -run TestMultiTurnGate   # all multi-turn gate tests pass
  - RUN: go test -race ./...                                             # full suite green
  - FIX-FORWARD: read failures, fix, re-run.
```

### Implementation Patterns & Key Details

```go
// === The one-line fix (L311) ===
// BEFORE (bug): mtPayload := payload
//   `payload` was rebuilt at L231 (BuildUserPayload) then prepended with retryInstr at L233 when parseFail.
//   So if the last one-shot attempt failed parsing, payload = retryInstr + "\n\n" + BuildUserPayload(diff,...).
//   The retryInstr preamble ("Output ONLY the commit message. No preamble, no markdown, no quotes.")
//   is one-shot-only; multi-turn has its own priming preamble (FR-T4).
// AFTER (fix): mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
//   Rebuilds from the untruncated `diff` (L193) WITHOUT retryInstr. Matches the TokenLimit≠0 path (L323).

// === Why both TokenLimit paths now produce the same mtPayload ===
// TokenLimit==0: mtPayload = BuildUserPayload(diff, ...) — diff is untruncated (StagedDiff used TL=0).
// TokenLimit≠0: the re-capture branch (L312-326) overwrites mtPayload = BuildUserPayload(fullDiff, ...).
//   fullDiff is the re-captured untruncated diff (StagedDiff with TokenLimit=0).
// Both produce a retryInstr-free payload derived from the untruncated diff. Consistent.

// === The retryInstr-specific assertion substring ===
// The default RetryInstruction: "Output ONLY the commit message. No preamble, no markdown, no quotes."
// The multi-turn final-turn prompt: "Now write the commit message for the diff above. Output ONLY the message."
// Both contain "Output ONLY" → ambiguous. Use "No preamble, no markdown, no quotes." (retryInstr-only).
```

### Integration Points

```yaml
PRODUCTION (internal/generate/generate.go):
  - L311: mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)  (was: := payload)
  - ~L307: comment updated (mtPayload ALWAYS rebuilt from diff)
  - the TokenLimit≠0 re-capture branch (L312-326) is UNCHANGED

TESTS (internal/generate/multiturn_test.go):
  - +TestMultiTurnGate_TokenLimitZero_ParseFail_NoRetryInstr (mirror of :618 for TL==0 + parseFail)

CONSUMED (READ-ONLY):
  - internal/prompt/payload.go: BuildUserPayload(diff, context, rejected) (L97)
  - internal/generate/multiturn_test.go: stubAppendManifest, stubtest.Build, initRepo, commitRaw, writeFile, stageFile

GATE: go test -race ./internal/generate/ → GREEN

NO-TOUCH (explicitly):
  - internal/generate/multiturn.go (S1 ChunkCount — parallel)
  - internal/generate/{rescue,dedupe}.go (unaffected)
  - pkg/stagecoach/stagecoach.go (P1.M2 — runPipeline), internal/hook/exec.go (P1.M3 — hook)
  - docs/* (P1.M4); PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational):
  - P1.M2.T1.S2 (runPipeline): copies this corrected gate — inherits the mtPayload rebuild from diff.
  - P1.M3.T1.S2 (hook.Run): copies this corrected gate — inherits the mtPayload rebuild from diff.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/generate/   # Expected: empty
go vet ./internal/generate/... # Expected: exit 0
go build ./...                 # Expected: exit 0 (one-line replacement; no signature change)

# Expected: Zero errors.
```

### Level 2: Unit Tests (the new + existing multi-turn gate tests)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./internal/generate/ -v -run TestMultiTurnGate

# Expected: ALL pass — including the new TestMultiTurnGate_TokenLimitZero_ParseFail_NoRetryInstr
# (multi-turn fires + succeeds; mtPayload lacks retryInstr) AND the existing
# TestMultiTurnGate_TokenLimitTruncated_Recaptures (TokenLimit≠0 path, unchanged).
```

### Level 3: Whole-Repository Regression

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...    # Expected: ALL packages green
go vet ./...           # Expected: exit 0

# Confirm ONLY the 2 intended files changed
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/generate/generate.go + internal/generate/multiturn_test.go only.
```

### Level 4: The Bug-Is-Gone Check

```bash
cd /home/dustin/projects/stagecoach

# L311 is the fix (NOT `mtPayload := payload`):
sed -n '311p' internal/generate/generate.go
# Expected: mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)

# The new test directly asserts the mtPayload lacks retryInstr. Cross-check by running it explicitly:
go test -race ./internal/generate/ -v -run 'TestMultiTurnGate_TokenLimitZero_ParseFail'

# Expected: PASS. Before the fix, this test would FAIL (the mtPayload included retryInstr).
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.

### Feature Validation

- [ ] generate.go:311 is `mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)`.
- [ ] The comment at ~L307 reflects mtPayload is ALWAYS rebuilt from diff (not "we use [payload] directly").
- [ ] The new test passes: multi-turn fires + succeeds; mtPayload lacks the retryInstr preamble.
- [ ] The existing `TestMultiTurnGate_TokenLimitTruncated_Recaptures` still passes (TokenLimit≠0 path unchanged).

### Scope Discipline Validation

- [ ] ONLY `internal/generate/generate.go` + `internal/generate/multiturn_test.go` modified.
- [ ] Did NOT edit `multiturn.go` (S1), `pkg/stagecoach/` (P1.M2), `internal/hook/` (P1.M3), docs (P1.M4).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Anti-Patterns to Avoid

- ❌ Don't reuse `payload` for mtPayload — that's the bug. `payload` carries retryInstr when parseFail (L233).
  Rebuild from `diff` via `BuildUserPayload`. (gotcha G1)
- ❌ Don't forget to update the comment at L307. The current comment says "we use [payload] directly" —
  that's now wrong. Update it to reflect the rebuild from `diff`. (G2)
- ❌ Don't assert on "Output ONLY the commit message" — that substring also appears in the multi-turn
  final-turn prompt. Use "No preamble, no markdown, no quotes." (retryInstr-specific). (G3)
- ❌ Don't trust the contract's stale line number (:418) for the test-to-mirror. S1's ChunkCount tests
  shifted it. Grep for `TestMultiTurnGate_TokenLimitTruncated_Recaptures` (now at :618). (G4)
- ❌ Don't use cfg.TokenLimit≠0 in the new test — that enters the re-capture branch (L312) which overwrites
  mtPayload, making the bug invisible. Use cfg.TokenLimit=0 to exercise the L311 path. (G6)
- ❌ Don't forget to force parseFail (stub call 1 returns "") — without it, `payload` has no retryInstr and
  the test doesn't exercise the bug. (G7)
- ❌ Don't edit `multiturn.go` (S1), `pkg/stagecoach/` (P1.M2), `internal/hook/` (P1.M3), or docs (P1.M4).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a one-line production fix (`mtPayload := payload` → `mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)`) with the verbatim buggy line and fix quoted from the live source (generate.go:311), the variables confirmed in scope (`diff` L193, `cfg.Context`, `rejected`), the BuildUserPayload signature verified (payload.go:97), and the architecture docs (`resolution_strategy.md` ISSUE 4 + `research_commitstaged_reference.md` §4) prescribing the exact before/after + comment text. The fix matches the TokenLimit≠0 path's existing rebuild (L323), so it's a proven pattern. The test mirrors an existing, passing test (`TestMultiTurnGate_TokenLimitTruncated_Recaptures` at :618) with clear setup (TokenLimit=0, parseFail, large diff, small chunk budget) and a well-defined assertion (retryInstr-specific substring "No preamble, no markdown, no quotes."). The prior parallel PRP (S1) touches only `multiturn.go` (ChunkCount) — no conflict. The one residual uncertainty (not 10/10) is the test's observation mechanism for mtPayload content (it's a local in CommitStaged; the STUB's STDINFILE captures the last turn's stdin, not the first chunk's — the implementing agent may need to verify the observation path or use the verbose turn-count as a proxy), mitigated by the two complementary assertions (multi-turn succeeds + the retryInstr-specific substring check via whatever observation path works).
