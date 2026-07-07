---
name: "P1.M1.T3.S1 — Extend progress line format with chunk token budget in CommitStaged gate (Issue 3 / FR-T11)"
description: |
  Surgical 0.5-point fix (Issue 3 — Minor, FR-T11): the multi-turn fallback progress line at
  internal/generate/generate.go (the `fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns,
  ~%dm total\n", turns, totalMin)` inside the FR-T1 gate, ~line 338) omits the per-chunk token budget
  that FR-T11 requires in verbose output. Change the format string from 2 args to 3 args, inserting
  `(chunks of ~%d tokens), ` after `%d turns` and passing `cfg.MultiTurnChunkTokens` (already in scope —
  used two lines above in the `turns := len(chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1` line)
  as the new 2nd argument. No new computation, no new variable, no new import. The two named tests
  (TestMultiTurnTriggerGate_TruthTable, TestCommitStaged_MultiTurnRenderContract) do NOT capture this
  line (they assert on the verbose buffer; the progress line writes DIRECTLY to os.Stderr) → add ONE
  focused unit test (TestCommitStaged_MultiTurnProgressLine_ChunkTokens) + a small captureStderr helper
  that swaps os.Stderr to a temp file (race-safe: no t.Parallel anywhere in internal/generate tests) and
  asserts the captured stderr contains the new substring (e.g. "chunks of ~4 tokens"). Reference the
  Fprintf by its UNIQUE CONTENT, not a line number (the parallel P1.M1.T2.S1 edits the comment at ~L307
  and may shift the line). DOCS: none (verbose diagnostic; how-it-works.md is P1.M4.T1). The corrected
  line is the copy source for P1.M2.T1.S2 (runPipeline) and P1.M3.T1.S2 (hook.Run). Baseline GREEN.
---

## Goal

**Feature Goal**: Make the multi-turn fallback progress line (PRD §9.24 FR-T5, emitted on `os.Stderr` at
fallback time) carry the per-chunk token budget (`cfg.MultiTurnChunkTokens`) that FR-T11 requires in the
verbose surface — so a user running `stagecoach --verbose` (or reading the fallback progress line) sees
the chunk size the large diff is being split into, not just the turn count and total wall-clock budget.

**Deliverable** (1 production format-string change + 1 focused unit test + 1 small test helper):
1. `internal/generate/generate.go` — change the Fprintf format string from 2 args to 3 args (insert the
   per-chunk token budget as the new 2nd `%d`).
2. `internal/generate/multiturn_test.go` — add `captureStderr(t, fn) string` helper + the focused test
   `TestCommitStaged_MultiTurnProgressLine_ChunkTokens` that triggers the gate, captures `os.Stderr`, and
   asserts the new substring.

No other files touched. No docs (P1.M4.T1 owns the cross-cutting how-it-works.md sync).

**Success Definition**: `go build/vet/gofmt` clean; `go test -race ./...` green; the progress line now
reads `↳ falling back to multi-turn: <N> turns (chunks of ~<chunkTokens> tokens), ~<M>m total`; the new
test proves the `chunks of ~<N> tokens` substring appears on stderr when the gate fires; the two named
tests are UNCHANGED (they assert on the verbose buffer, which does not carry this line); `git diff --stat`
shows ONLY generate.go + multiturn_test.go.

## User Persona

**Target User**: The user whose large diff triggers the multi-turn fallback and who watches the terminal
(or reads `--verbose` logs) to understand what stagecoach is doing during the (potentially many-minute)
N+1-turn run. Also the contributor copying this corrected gate into runPipeline/hook (P1.M2/P1.M3).

**Use Case**: A user sees `↳ falling back to multi-turn: 9 turns, ~18m total` and wonders "how big is
each chunk?" After the fix, the line reads `↳ falling back to multi-turn: 9 turns (chunks of ~32000
tokens), ~18m total` — the chunk budget is visible, so the user understands the per-request sizing
(FR-T3) and can tune `multi_turn_chunk_tokens` if they want fewer/shorter turns.

**User Journey**: large diff → one-shot exhausts → gate fires → progress line (os.Stderr) now shows the
chunk budget → multi-turn runs → commit lands. The user sees the chunk budget at fallback time.

**Pain Points Addressed**: FR-T11's verbose surface was missing the per-chunk token estimate (Issue 3).
Without it, the user can't tell what chunk size the run is using without digging into config — making the
`tunable knob` (`multi_turn_chunk_tokens`) opaque at the moment it matters most (fallback time).

## Why

- **PRD §9.24 FR-T11 is the mandate:** the verbose multi-turn surface must print *"the per-chunk token
  estimate."* The progress line (FR-T5: "the CLI prints the turn count and total budget on the progress
  line at fallback time") is the natural home for it — it already carries the turn count and total
  budget; the chunk budget is the missing third datum.
- **The value is ALREADY in scope.** `cfg.MultiTurnChunkTokens` is used two lines above the Fprintf
  (`turns := len(chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1`). The fix is a format-string
  change — zero new computation, zero risk to the gate logic.
- **The corrected line is the copy source for P1.M2/P1.M3.** runPipeline (dry-run) and hook.Run will
  copy this gate; fixing the format here ensures the copies inherit the FR-T11 surface without a second
  edit. (The `resolution_strategy.md` ISSUE 1/2 fixes propagate the whole gate, including this line.)
- **Aligns code with the already-correct docs.** `docs/how-it-works.md` (and FR-T11) describe the
  per-chunk estimate as part of the verbose surface; the code was the lone omission.

## What

A 2-arg → 3-arg `fmt.Fprintf` format-string change at the progress line inside the FR-T1 multi-turn gate
in `CommitStaged`:

```go
// CURRENT
fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", turns, totalMin)
// TARGET
fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n", turns, cfg.MultiTurnChunkTokens, totalMin)
```

Plus a focused unit test that captures `os.Stderr` (the line is a direct stderr write, NOT routed through
`deps.Verbose`) and asserts the new `chunks of ~<N> tokens` substring. The two named tests
(`TestMultiTurnTriggerGate_TruthTable`, `TestCommitStaged_MultiTurnRenderContract`) are UNCHANGED — they
assert on the verbose buffer (`buf`), which does not carry this `os.Stderr` line.

### Success Criteria

- [ ] generate.go's progress Fprintf reads `"↳ falling back to multi-turn: %d turns (chunks of ~%d
      tokens), ~%dm total\n"` with args `turns, cfg.MultiTurnChunkTokens, totalMin`.
- [ ] `cfg.MultiTurnChunkTokens` is the new 2nd arg (in scope; no new computation).
- [ ] `internal/generate/multiturn_test.go` has `captureStderr(t, fn) string` + the new
      `TestCommitStaged_MultiTurnProgressLine_ChunkTokens` test, both passing.
- [ ] The test asserts the captured stderr contains `chunks of ~<chunkTokens> tokens` (built dynamically).
- [ ] The two named tests are UNCHANGED (no edits to TruthTable or RenderContract).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] ONLY `internal/generate/generate.go` + `internal/generate/multiturn_test.go` modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim current Fprintf (the unique content to grep for —
not a brittle line number), the verbatim target, the in-scope variable (`cfg.MultiTurnChunkTokens`), the
reason the named tests don't capture the line (verbose buffer vs os.Stderr), the complete `captureStderr`
helper, the complete test body (mirroring the TruthTable's `control_all_true_gate_fires` case), the
race-safety proof (no `t.Parallel` in the package), and the parallel-T2.S1 line-drift caveat. No
inference required.

### Documentation & References

```yaml
# MUST READ — the bug analysis + this task's research
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/docs/architecture/resolution_strategy.md
  why: "ISSUE 3 (the --verbose per-chunk token estimate gap; FR-T11). Names the progress line, its
        current 2-arg format, and the prescribed 3-arg fix."
  critical: "ISSUE 3 prescribes the exact format-string change. This PRP's verbatim current→target IS
             that fix."

- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/docs/architecture/research_config_provider_docs.md
  why: "§4 (Verbose) documents the FR-T11 verbose surface expectation (per-chunk token estimate)."
  critical: "Confirms FR-T11 wants the per-chunk estimate in the verbose/progress surface — the property
             this fix delivers."

- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M1T3S1/research/progress_line_chunk_tokens.md
  why: "THIS subtask's research: §2 the verbatim fix; §3 why reference by CONTENT not line number
        (T2.S1 drift); §4 why the named tests don't capture the line (verbose buffer vs os.Stderr);
        §5 the captureStderr mechanic + race-safety; §6 the test design; §8 decisions D1–D6. READ FIRST."
  critical: "§4 (named tests do NOT capture the line → add a focused test) and §5 (the os.Stderr swap is
             race-safe — no t.Parallel) are the two non-obvious insights an implementer would otherwise
             fumble. §3 prevents a brittle line-number edit."

- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M1T2S1/PRP.md
  why: "The parallel sibling (Issue 4 — mtPayload). Confirms it edits generate.go:311 + the comment at
        ~L307, which is ABOVE the Fprintf and may shift its line number. T3.S1 references the Fprintf by
        content (not number) so the two don't conflict. Both edit multiturn_test.go with DISTINCT
        additions (T2.S1: a TokenLimit==0 test; T3.S1: the progress-line test + captureStderr)."
  critical: "Do NOT touch L311 or the ~L307 comment (T2.S1's territory). The Fprintf is a distinct region.
             Reference it by its format-string content so T2.S1's comment growth doesn't break the grep."

# The edit target + the test file
- file: internal/generate/generate.go
  why: "EDIT (1 line). The Fprintf inside the FR-T1 multi-turn gate. Reference by UNIQUE CONTENT:
        `fmt.Fprintf(os.Stderr, \"↳ falling back to multi-turn: %d turns, ~%dm total\\n\", turns, totalMin)`.
        (Live grep reports it at ~line 338 today; T2.S1's comment edit at ~L307 may shift it — grep, do
        not hard-code the number.)"
  pattern: "The 3 variables — turns, cfg.MultiTurnChunkTokens, totalMin — are all in scope at the Fprintf
            (turns/totalMin are computed on the preceding lines; cfg.MultiTurnChunkTokens is used two
            lines above in the `turns := len(chunkPayload(...))` line)."
  gotcha: "Do NOT touch the preceding `turns`/`totalMin` computation, the FR-T5 comment, the
           deps.Verbose.VerboseWarn line, or anything else in the gate. ONE format-string change only."

- file: internal/generate/multiturn_test.go
  why: "EDIT (1 helper + 1 test + imports). Add `captureStderr(t, fn) string`, add
        TestCommitStaged_MultiTurnProgressLine_ChunkTokens, and ensure `os` + `fmt` are imported (the
        file currently imports bytes/strings/config/git/provider/stubtest/ui — `os` is NEW; `fmt` may
        need adding if not present)."
  pattern: "Mirror TestMultiTurnTriggerGate_TruthTable's control_all_true_gate_fires case (initRepo +
            commitRaw + writeFile(large) + stageFile + stubAppendManifest with [\"\", \"ok\", \"ok\",
            \"feat: mt win\"] + cfg.MaxDuplicateRetries=0 + cfg.MultiTurnFallback=true +
            cfg.MultiTurnChunkTokens=4). Wrap the CommitStaged call in captureStderr; assert the captured
            stderr Contains the dynamic substring `fmt.Sprintf(\"chunks of ~%d tokens\", cfg.MultiTurnChunkTokens)`."
  gotcha: "The progress line writes to os.Stderr, NOT deps.Verbose. The existing `buf.Contains(...)`
           assertions in TruthTable/RenderContract match the VERBOSE trigger line (the NEXT statement,
           deps.Verbose.VerboseWarn) — NOT this Fprintf. Do NOT 'fix' those tests to assert the chunk
           substring (they don't capture it). Add the focused test instead."

# Read-only refs
- file: internal/generate/generate.go   # the gate region (READ-ONLY context — the Fprintf + neighbors)
  why: "Confirms the Fprintf's exact current content + that cfg.MultiTurnChunkTokens is in scope (used
        at the `turns := len(chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1` line two lines above)."
- file: internal/generate/multiturn_test.go   # the test pattern source
  why: "TestMultiTurnTriggerGate_TruthTable (~:512) is the gate-trigger pattern to mirror; stubAppendManifest
        (:195), initRepo/commitRaw/writeFile/stageFile, tail (:652), and the config/git/ui/stubtest imports
        are all reusable. No helper redeclaration."

# External references
- url: https://pkg.go.dev/os#CreateTemp
  why: "os.CreateTemp(dir, pattern) creates a temp file — the sink for captureStderr's os.Stderr swap.
        t.TempDir() scopes cleanup; the pattern 'stderr-*.txt' keeps it identifiable."
  critical: "os.Stderr is a *os.File; swapping it to the temp file's *os.File makes the in-process
             fmt.Fprintf write to the file. Restore the original immediately after fn() (before t.Errorf)."
- url: https://pkg.go.dev/fmt#Fprintf
  why: "fmt.Fprintf(os.Stderr, format, args…) — the function under edit. The format string's %d count
        must match the arg count (2 → 3) or Fprintf silently drops/zero-fills."
  critical: "The arg count MUST match the verb count. The fix adds one %d AND one arg (cfg.MultiTurnChunkTokens);
             a mismatch is a latent bug (no compile error — fmt.Fprintf is variadic)."
```

### Current Codebase Tree (this task's scope)

```bash
stagecoach/
└── internal/generate/
    ├── generate.go         # EDIT: the progress Fprintf format string (2 args → 3 args)
    └── multiturn_test.go   # EDIT: +captureStderr helper +TestCommitStaged_MultiTurnProgressLine_ChunkTokens (+os/fmt imports)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/generate/generate.go         # the Fprintf: + "(chunks of ~%d tokens), " + cfg.MultiTurnChunkTokens arg
    internal/generate/multiturn_test.go   # +captureStderr +TestCommitStaged_MultiTurnProgressLine_ChunkTokens
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/generate/generate.go` | MODIFY (1 line) | The progress Fprintf: 2-arg → 3-arg format string (insert the per-chunk token budget). **Only production change.** |
| `internal/generate/multiturn_test.go` | MODIFY (append helper + test + imports) | `captureStderr` helper + the focused progress-line test; add `os`/`fmt` imports if absent. |

**Explicitly NOT touched**: `internal/generate/multiturn.go` (S1 ChunkCount), `internal/generate/generate_multiturn_test.go` (the RenderContract test — it does not capture the line), `internal/generate/{rescue,dedupe,finalize}.go` and their tests, `pkg/stagecoach/stagecoach.go` (P1.M2 — runPipeline), `internal/hook/exec.go` (P1.M3 — hook), `docs/*` (P1.M4), any other package, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — reference the Fprintf by UNIQUE CONTENT, not a line number): the contract cited :337;
// the live grep reports :338; the parallel T2.S1 (Issue 4) rewrites the comment at ~L307 and may shift the
// Fprintf's line number when it lands. GREP for the exact current string:
//   fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", turns, totalMin)
// and replace it. Do NOT hard-code a line number — it is brittle across the parallel edit.

// CRITICAL (G2 — the named tests do NOT capture this line; do NOT edit them): TestMultiTurnTriggerGate_TruthTable
// and TestCommitStaged_MultiTurnRenderContract build ui.NewVerbose(&buf, true) and assert on buf.String().
// The progress Fprintf writes DIRECTLY to os.Stderr (NOT &buf). The existing buf.Contains("multi-turn
// fallback") assertions match the VERBOSE trigger line (deps.Verbose.VerboseWarn — the NEXT statement),
// NOT the Fprintf. So those tests are blind to this format change. Leave them UNCHANGED; add a focused
// test that captures os.Stderr. (Research §4 / D2.)

// CRITICAL (G3 — capture os.Stderr via a temp-file swap; it is race-safe): the package has ZERO t.Parallel()
// calls (verified: grep -rn 't\.Parallel()' internal/generate/*_test.go → none), so tests run SERIALLY
// within the package. Swapping the package-global os.Stderr is sequential → the -race detector will not
// flag it. Restore the original os.Stderr immediately after fn() (before any t.Errorf) so test output
// stays clean. (Research §5 / D3.) If a FUTURE test adds t.Parallel(), captureStderr would need a mutex
// — out of scope here.

// CRITICAL (G4 — the arg count MUST match the verb count): fmt.Fprintf is variadic — a %d/arg mismatch
// does NOT compile-error; it silently drops or zero-fills. The fix adds ONE %d AND ONE arg
// (cfg.MultiTurnChunkTokens). Verify the final call has exactly 3 verbs (%d %d %d) and 3 args
// (turns, cfg.MultiTurnChunkTokens, totalMin).

// GOTCHA (G5 — cfg.MultiTurnChunkTokens is ALREADY in scope; no new computation): it is used two lines
// above the Fprintf in `turns := len(chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1`. Just pass
// it as the new 2nd arg. Do NOT recompute it, re-derive it, or introduce a new variable.

// GOTCHA (G6 — use a SMALL chunkTokens in the test, not 50000): the contract's example used 50000, but
// that needs a >200000-rune payload (slow). Use cfg.MultiTurnChunkTokens=4 with the existing ~96-rune
// fixture (strings.Repeat("change line\n", 8) ⇒ EstimateTokens ≈ 24 > 4 ⇒ cond b fires). The substring
// "chunks of ~4 tokens" is unambiguous (4 does not appear as the turn-count or total-min). Build the
// expected substring dynamically: fmt.Sprintf("chunks of ~%d tokens", cfg.MultiTurnChunkTokens). (D4.)

// GOTCHA (G7 — assert the NEW field + a sanity line; do NOT over-assert): assert Contains(captured,
// "chunks of ~<N> tokens") (the FR-T11 deliverable) + Contains(captured, "falling back to multi-turn")
// (the line is intact). Do NOT assert the EXACT full line — the turns/totalMin numbers depend on the
// chunk count (a function of the payload size) and the timeout, and are brittle. (D5.)

// GOTCHA (G8 — multiturn_test.go needs `os` (and maybe `fmt`) imported): the file imports
// bytes/strings/config/git/provider/stubtest/ui today. captureStderr uses os.CreateTemp/os.ReadFile/os.Stderr
// (needs "os"); the dynamic-substring assertion uses fmt.Sprintf (needs "fmt" — check if already present;
// if not, add it). Add only what's missing; Go rejects unused imports AND missing imports.

// GOTCHA (G9 — restore os.Stderr even if fn panics/Fatalf's): captureStderr must restore os.Stderr via
// BOTH an immediate post-fn assignment AND a defer (so a t.Fatalf inside fn — e.g. CommitStaged errors —
// still restores via the defer before the test framework prints its failure to the real stderr). os.Stderr
// is unbuffered — no flush needed.

// GOTCHA (G10 — the corrected line is the COPY SOURCE for P1.M2/P1.M3): keep the format canonical:
// "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n". runPipeline (P1.M2.T1.S2)
// and hook.Run (P1.M3.T1.S2) copy this gate verbatim — they inherit the FR-T11 surface via this exact
// string. Do not introduce a different format here.
```

## Implementation Blueprint

### Data models and structure

None. The fix is a format-string change. `turns` (int), `cfg.MultiTurnChunkTokens` (int), and `totalMin`
(int) are all in scope at the Fprintf.

### The production edit (exact — current → target)

**`internal/generate/generate.go`** — the Fprintf inside the FR-T1 multi-turn gate (grep for the unique
content; do not trust a line number):

```go
// CURRENT (2 args — omits the per-chunk token budget FR-T11 requires)
			fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", turns, totalMin)

// TARGET (3 args — the per-chunk token budget is the new 2nd %d, right after "turns")
			fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n", turns, cfg.MultiTurnChunkTokens, totalMin)
```

### The test additions (exact — append to `internal/generate/multiturn_test.go`)

```go
// captureStderr swaps os.Stderr to a temp file for the duration of fn, then restores it and returns the
// captured content. Race-safe ONLY because no test in this package calls t.Parallel() (tests run serially,
// so the package-global os.Stderr swap is sequential — the -race detector does not flag it). Used to
// assert on direct os.Stderr writes (the multi-turn progress line) that bypass deps.Verbose.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stderr-*.txt")
	if err != nil {
		t.Fatalf("create temp stderr: %v", err)
	}
	orig := os.Stderr
	os.Stderr = f
	defer func() { os.Stderr = orig }() // restore even if fn Fatalf's/panics
	fn()
	os.Stderr = orig // restore now (before any t.Errorf below) so test output is clean
	if err := f.Close(); err != nil {
		t.Fatalf("close temp stderr: %v", err)
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("read temp stderr: %v", err)
	}
	return string(b)
}

// TestCommitStaged_MultiTurnProgressLine_ChunkTokens (FR-T11): the multi-turn fallback progress line
// (os.Stderr) carries the per-chunk token budget. Mirrors TestMultiTurnTriggerGate_TruthTable's
// control_all_true_gate_fires case (gate fires), but captures os.Stderr (not the verbose buffer) and
// asserts the new "chunks of ~<N> tokens" substring.
func TestCommitStaged_MultiTurnProgressLine_ChunkTokens(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	// ~96 runes ⇒ EstimateTokens ≈ 24 > 4 (FR-T1 cond b: payload exceeds one chunk).
	writeFile(t, repo, "new.txt", strings.Repeat("change line\n", 8))
	stageFile(t, repo, "new.txt")

	// Script: call 1 = "" (one-shot parse-fail ⇒ exhaust ⇒ gate fires); "ok","ok" priming; final = message.
	m := stubAppendManifest(t, bin, []string{"", "ok", "ok", "feat: mt win"}, false) // append (cond d)
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0   // exactly 1 one-shot attempt ⇒ exhaust ⇒ gate fires
	cfg.MultiTurnFallback = true  // cond c
	cfg.MultiTurnChunkTokens = 4  // cond b (24 > 4); small ⇒ a distinctive, easy-to-assert substring

	var vbuf bytes.Buffer
	captured := captureStderr(t, func() {
		_, err := CommitStaged(context.Background(), Deps{
			Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&vbuf, true),
		}, cfg)
		if err != nil {
			t.Fatalf("CommitStaged: %v (expected multi-turn success)", err)
		}
	})

	// FR-T11: the progress line carries the per-chunk token budget.
	wantChunk := fmt.Sprintf("chunks of ~%d tokens", cfg.MultiTurnChunkTokens) // "chunks of ~4 tokens"
	if !strings.Contains(captured, wantChunk) {
		t.Errorf("progress line missing %q (FR-T11 per-chunk token estimate);\ngot stderr: %q", wantChunk, captured)
	}
	// Sanity: the line is still the multi-turn progress line.
	if !strings.Contains(captured, "falling back to multi-turn") {
		t.Errorf("progress line missing 'falling back to multi-turn';\ngot stderr: %q", captured)
	}
}
```

> **Imports:** ensure `internal/generate/multiturn_test.go` imports `"os"` (new — for CreateTemp/ReadFile/
> Stderr) and `"fmt"` (for Sprintf — check if already present). Add only what's missing.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: FIX generate.go — the progress Fprintf format string (the production change)
  - FILE: internal/generate/generate.go
  - GREP for the unique current content:
        fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", turns, totalMin)
    (Do NOT trust a line number — the parallel T2.S1 comment edit may have shifted it. G1.)
  - REPLACE with the TARGET (3 args: turns, cfg.MultiTurnChunkTokens, totalMin) from §"The production edit".
  - VERIFY cfg.MultiTurnChunkTokens is the new 2nd arg and the format has exactly 3 %d verbs. (G4/G5.)
  - DO NOT: touch the preceding turns/totalMin computation, the FR-T5 comment, the
    deps.Verbose.VerboseWarn line, the gate conditions, or L311/the ~L307 comment (T2.S1's territory).
  - VERIFY: go build ./internal/generate/ → exit 0.

Task 2: ADD captureStderr + the test to multiturn_test.go
  - FILE: internal/generate/multiturn_test.go
  - IMPORTS: add "os" (CreateTemp/ReadFile/Stderr); add "fmt" if not already imported (Sprintf). Remove
    nothing. (G8.)
  - APPEND captureStderr (from §"The test additions") — the temp-file os.Stderr swap helper. (G3/G9.)
  - APPEND TestCommitStaged_MultiTurnProgressLine_ChunkTokens (from §"The test additions") — mirrors the
    TruthTable control_all_true_gate_fires case; wraps CommitStaged in captureStderr; asserts the dynamic
    substring + a sanity line. (G6/G7.)
  - DO NOT: edit TestMultiTurnTriggerGate_TruthTable or TestCommitStaged_MultiTurnRenderContract (G2 —
    they don't capture os.Stderr); use t.Parallel() (G3); change the stub binary or stubAppendManifest.
  - VERIFY: go test -race ./internal/generate/ -run TestCommitStaged_MultiTurnProgressLine_ChunkTokens -v
    → PASS, and the captured stderr visibly contains "chunks of ~4 tokens".

Task 3: VALIDATE
  - RUN: gofmt -w internal/generate/generate.go internal/generate/multiturn_test.go ; gofmt -l .
  - RUN: go vet ./... ; go build ./...
  - RUN: go test -race ./internal/generate/    # new test + existing multi-turn tests green
  - RUN: go test -race ./...                    # whole repo green
  - RUN (the fix landed — grep the new format):
        grep -n 'chunks of ~%d tokens' internal/generate/generate.go   # EXPECT: exactly 1 match.
  - RUN (scope): git diff --stat -- internal/ pkg/ cmd/ docs/ → EXPECT ONLY:
        internal/generate/generate.go + internal/generate/multiturn_test.go.
```

### Implementation Patterns & Key Details

```go
// === Why reference the Fprintf by content, not line number (G1) ===
// The parallel T2.S1 (Issue 4) rewrites the comment at ~L307. If its replacement is longer, every line
// below shifts down. A line-number anchor (:337/:338) breaks; a content grep (the exact format string)
// is stable. Always grep for:
//   fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", turns, totalMin)

// === Why the named tests don't capture this line (G2) ===
// ui.NewVerbose(&buf, true) routes verbose output to &buf. The progress Fprintf writes to os.Stderr
// (a *os.File), bypassing &buf. The existing buf.Contains("multi-turn fallback") matches the VERBOSE
// trigger (deps.Verbose.VerboseWarn, the NEXT statement), NOT the Fprintf. So TruthTable/RenderContract
// are blind to this format change — leave them alone; add a dedicated os.Stderr-capturing test.

// === Why the os.Stderr swap is race-safe (G3) ===
// grep -rn 't\.Parallel()' internal/generate/*_test.go → ZERO matches. Within a package, Go runs tests
// SERIALLY unless t.Parallel() is called. A sequential swap of the package-global os.Stderr is not a
// data race (no concurrent access). The -race detector stays silent. Restore via defer + immediate
// assignment so a Fatalf inside fn still restores.

// === Why chunkTokens=4 in the test, not 50000 (G6) ===
// 50000 needs a >200000-rune payload (slow, memory-heavy). chunkTokens=4 + the existing ~96-rune fixture
// (24 tokens > 4) fires the gate cheaply. "chunks of ~4 tokens" is unambiguous (4 ≠ turns ≠ totalMin).
// The expected substring is built dynamically so it's correct for any value.
```

### Integration Points

```yaml
PRODUCTION (internal/generate/generate.go):
  - the progress Fprintf: + "(chunks of ~%d tokens), " in the format + cfg.MultiTurnChunkTokens as the 2nd arg

TESTS (internal/generate/multiturn_test.go):
  - +captureStderr(t, fn) helper (temp-file os.Stderr swap; race-safe, no t.Parallel)
  - +TestCommitStaged_MultiTurnProgressLine_ChunkTokens (gate fires ⇒ stderr contains "chunks of ~4 tokens")

CONSUMED (READ-ONLY):
  - cfg.MultiTurnChunkTokens (in scope at the Fprintf — used 2 lines above)
  - multiturn_test.go helpers: stubAppendManifest, initRepo, commitRaw, writeFile, stageFile, stubtest.Build, ui.NewVerbose

GATE: go test -race ./... → GREEN ; grep 'chunks of ~%d tokens' generate.go → 1 match

NO-TOUCH (explicitly — owned by siblings):
  - internal/generate/multiturn.go (S1 ChunkCount), generate_multiturn_test.go (RenderContract — doesn't capture the line)
  - generate.go L311 + ~L307 comment (T2.S1 — Issue 4, parallel)
  - pkg/stagecoach/stagecoach.go (P1.M2 — runPipeline), internal/hook/exec.go (P1.M3 — hook)
  - docs/* (P1.M4); PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by LATER subtasks):
  - P1.M2.T1.S2 (runPipeline) and P1.M3.T1.S2 (hook.Run) copy this corrected gate verbatim — they inherit
    the FR-T11 "chunks of ~%d tokens" surface via this exact format string. (G10.)
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/generate/generate.go internal/generate/multiturn_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/generate/...   # Expected: exit 0
go build ./...                   # Expected: exit 0 (format-string change; no signature change)

# Expected: zero errors. If `undefined: os` (or fmt) in multiturn_test.go, you forgot to add the import (G8).
# If gofmt rewrites the Fprintf line, re-run gofmt -w — the long format string is fine wrapped or unwrapped.
```

### Level 2: The New Unit Test (the deliverable)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./internal/generate/ -v -run TestCommitStaged_MultiTurnProgressLine_ChunkTokens
# Expected: PASS. The captured stderr visibly contains "chunks of ~4 tokens" + "falling back to multi-turn".

# Existing multi-turn tests still green (unchanged):
go test -race ./internal/generate/ -v -run 'TestMultiTurnTriggerGate_TruthTable|TestCommitStaged_MultiTurnRenderContract'
# Expected: both PASS (they assert on the verbose buffer, not os.Stderr — unaffected by the format change).
```

### Level 3: Whole-Repository Regression + Scope Discipline

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...    # Expected: ALL packages green
go vet ./...           # Expected: exit 0

# The fix landed (exactly 1 match — the new format string):
grep -n 'chunks of ~%d tokens' internal/generate/generate.go
# Expected: exactly one match (the progress Fprintf).

# The arg count matches the verb count (3 verbs, 3 args):
grep -n 'turns, cfg.MultiTurnChunkTokens, totalMin' internal/generate/generate.go
# Expected: exactly one match.

# Scope: ONLY the 2 intended files.
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/generate/generate.go + internal/generate/multiturn_test.go only.

# Sibling/parallel territory UNTOUCHED:
git diff --stat -- internal/generate/multiturn.go internal/generate/generate_multiturn_test.go pkg/stagecoach/ internal/hook/ docs/
# Expected: EMPTY (multiturn.go = S1; generate_multiturn_test.go = RenderContract, unchanged; P1.M2/P1.M3/docs untouched).
```

### Level 4: Behavioral Cross-Check (manual repro of the new progress line)

```bash
cd /home/dustin/projects/stagecoach

# The new test IS the behavioral proof (it captures os.Stderr and asserts the substring). For a manual
# cross-check, build a tiny repo that triggers multi-turn and eyeball the stderr line. (The authoritative
# proof is the unit test in Level 2; this is optional.)
go test -race ./internal/generate/ -v -run TestCommitStaged_MultiTurnProgressLine_ChunkTokens 2>&1 | head
# Expected: PASS; if you add a t.Logf(captured) you'd see: "↳ falling back to multi-turn: 3 turns (chunks of ~4 tokens), ~1m total"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green.
- [ ] `go test -race ./internal/generate/ -v -run TestCommitStaged_MultiTurnProgressLine_ChunkTokens` PASS.

### Feature Validation
- [ ] The progress Fprintf reads `... %d turns (chunks of ~%d tokens), ~%dm total\n` with args
      `turns, cfg.MultiTurnChunkTokens, totalMin`.
- [ ] The new test proves the captured stderr contains `chunks of ~<chunkTokens> tokens`.
- [ ] The two named tests (TruthTable, RenderContract) are UNCHANGED and still pass.

### Scope Discipline Validation
- [ ] ONLY `internal/generate/generate.go` + `internal/generate/multiturn_test.go` modified (`git diff --stat`).
- [ ] Did NOT edit `multiturn.go` (S1), `generate_multiturn_test.go` (RenderContract), generate.go L311/~L307
      comment (T2.S1), `pkg/stagecoach/` (P1.M2), `internal/hook/` (P1.M3), docs (P1.M4).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation
- [ ] The Fprintf is referenced by content (grep), not a brittle line number (G1).
- [ ] The format string has exactly 3 `%d` verbs and 3 args (G4).
- [ ] `captureStderr` restores os.Stderr via defer + immediate assignment (G9); no `t.Parallel` introduced (G3).
- [ ] The test uses a small chunkTokens (4) + the existing fixture (not a 50000-token payload) (G6).

---

## Anti-Patterns to Avoid

- ❌ Don't reference the Fprintf by line number (:337/:338). The parallel T2.S1's comment edit at ~L307
  may shift it. Grep for the unique format-string content (gotcha G1).
- ❌ Don't edit TestMultiTurnTriggerGate_TruthTable or TestCommitStaged_MultiTurnRenderContract to assert
  the chunk substring. They capture the VERBOSE buffer (`&buf`), not `os.Stderr`; the progress Fprintf
  bypasses `&buf`. Leave them unchanged; add a dedicated os.Stderr-capturing test (G2).
- ❌ Don't mismatch the verb/arg count. `fmt.Fprintf` is variadic — a `%d`/arg mismatch does NOT
  compile-error; it silently drops/zero-fills. The fix adds ONE `%d` AND ONE arg (`cfg.MultiTurnChunkTokens`)
  (G4).
- ❌ Don't introduce a new variable or computation for the chunk budget. `cfg.MultiTurnChunkTokens` is
  already in scope (used 2 lines above). Just pass it (G5).
- ❌ Don't use `t.Parallel()` in the new test (or anywhere in the package). The `captureStderr` helper
  swaps the package-global `os.Stderr`; it is race-safe ONLY because the package's tests are serial (G3).
- ❌ Don't forget to restore `os.Stderr` via BOTH a defer AND an immediate post-fn assignment — a
  `t.Fatalf` inside `fn` must still restore it so test output reaches the real stderr (G9).
- ❌ Don't use `chunkTokens=50000` in the test (the contract's example) — it needs a >200000-rune payload.
  Use `chunkTokens=4` + the existing ~96-rune fixture; build the expected substring dynamically (G6).
- ❌ Don't over-assert the exact full progress line. The `turns`/`totalMin` numbers depend on the chunk
  count (payload-size-dependent) and the timeout — brittle. Assert the new field (`chunks of ~N tokens`)
  + a sanity `Contains("falling back to multi-turn")` (G7).
- ❌ Don't touch `multiturn.go` (S1), `generate_multiturn_test.go` (RenderContract), generate.go L311/~L307
  (T2.S1), `pkg/stagecoach/` (P1.M2), `internal/hook/` (P1.M3), or docs (P1.M4).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a one-line production format-string change (2 args → 3 args) with the verbatim current
and target strings quoted from the live source, the new arg (`cfg.MultiTurnChunkTokens`) confirmed in
scope (used two lines above), and zero new computation/import in production. The test reuses an existing,
passing gate-trigger pattern (TruthTable's `control_all_true_gate_fires`) and adds a small, well-understood
`captureStderr` helper (temp-file os.Stderr swap) whose race-safety is proven (no `t.Parallel` anywhere in
the package). The three plausible mistakes — (G1) trusting a brittle line number instead of grepping the
content, (G2) editing the named tests that don't capture the line, and (G4) a verb/arg-count mismatch
(`fmt.Fprintf`'s silent variadic drop) — are front-loaded as CRITICAL gotchas. The parallel T2.S1 (Issue 4)
touches a distinct region (L311 + ~L307 comment); the only interaction is potential line drift, which the
content-grep approach neutralizes. The residual 0.5 uncertainty is the `captureStderr` temp-file mechanic
(it is robust but not previously used in this repo — if the box's tmpdir is unwritable the helper Fatalf's
clearly; and if a future sibling adds `t.Parallel()` the helper would need a mutex, out of scope here),
mitigated by the defer+restore discipline and the no-`t.Parallel` verification. No production-code risk
outside the one Fprintf; no parallel-edit risk (T2.S1's region is distinct; both edit multiturn_test.go
with disjoint additions).
