# Progress-line chunk-token budget (Issue 3 / FR-T11) — P1.M1.T3.S1 Research

> Verified against the live repo (module `github.com/dustin/stagecoach`, 2026-07-05). No files modified —
> research only. This is a 0.5-point surgical fix: extend ONE `fmt.Fprintf` format string from 2 args to
> 3 args, and add ONE focused unit test that captures `os.Stderr`.

## 1. The bug (Issue 3 / FR-T11)

PRD §9.24 FR-T11 requires the `--verbose` multi-turn surface to include the per-chunk token estimate.
The progress line emitted at fallback time (FR-T5) currently prints the turn count + total wall-clock
budget but NOT the per-chunk token budget:

```go
// internal/generate/generate.go (the Fprintf inside the FR-T1 multi-turn gate, ~line 338)
fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", turns, totalMin)
```

`cfg.MultiTurnChunkTokens` (int, default 32000 — the per-chunk token budget, the target size each chunk
aims for) is ALREADY in scope at this line (it is used two lines above: `turns := len(chunkPayload(mtPayload,
cfg.MultiTurnChunkTokens)) + 1`). It is just not printed. The fix adds it as a third `%d`.

## 2. The exact fix (verbatim)

```go
// CURRENT (2 args)
fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns, ~%dm total\n", turns, totalMin)

// TARGET (3 args — the per-chunk token budget is inserted after "turns")
fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n", turns, cfg.MultiTurnChunkTokens, totalMin)
```

The second `%d` is `cfg.MultiTurnChunkTokens` (the per-chunk token estimate FR-T11 asks for). No new
computation, no new variable, no new import — `cfg.MultiTurnChunkTokens` is already in scope.

## 3. Line-number drift (reference by CONTENT, not number)

- The contract cited generate.go:**337**. The live grep (`grep -n 'falling back to multi-turn'`) reports
  the Fprintf at **338** today.
- The parallel P1.M1.T2.S1 (Issue 4, the mtPayload fix) edits generate.go:**311** + rewrites the comment
  at **~L307**. If T2.S1's replacement comment is LONGER than the original, every line below (including
  the Fprintf) shifts DOWN by the delta. So when T3.S1 runs (after T2.S1 lands), the Fprintf could be at
  338, 339, 340, or wherever.
- **Resolution: reference the line by its UNIQUE CONTENT** (`fmt.Fprintf(os.Stderr, "↳ falling back to
  multi-turn: %d turns, ~%dm total\n", turns, totalMin)`), not by a brittle line number. The implementer
  greps for that exact string and replaces it. This is robust to T2.S1's comment-growth shift.

## 4. The two named tests do NOT capture the progress line → add a focused unit test

The contract says: "Update TestMultiTurnTriggerGate_TruthTable (multiturn_test.go) and
TestCommitStaged_MultiTurnRenderContract (generate_multiturn_test.go) to also assert the chunk-tokens
substring IF they capture the progress line."

**They do NOT capture it.** Verified:
- Both tests build `ui.NewVerbose(&buf, true)` and assert on `buf.String()`. The verbose sink writes to
  `&buf` (a `*bytes.Buffer`), NOT `os.Stderr`.
- The progress Fprintf writes DIRECTLY to `os.Stderr` (the code comment explains why: "Deps.Progress is a
  no-arg callback (can't carry the message) → direct stderr write").
- The existing `buf.Contains("multi-turn fallback")` assertions match the VERBOSE trigger line
  (`deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")` — the NEXT statement after the
  Fprintf), NOT the progress Fprintf.

So neither test sees the progress line. Per the contract's "if they capture" clause (FALSE here), the
deliverable is a NEW focused unit test that captures `os.Stderr` and asserts the chunk-tokens substring.

## 5. Capturing `os.Stderr` in-process (the one non-obvious mechanic)

The progress line is a DIRECT `os.Stderr` write. No existing in-process stderr-capture helper exists in
the repo (the only stderr captures are subprocess-level: `cmd.Stderr = &errb` in `internal/e2e/*` — a
different mechanism). The clean approach for a unit test: **swap `os.Stderr` to a temp file** around the
`CommitStaged` call, then read it back.

**Race-safety:** `grep -rn 't\.Parallel()' internal/generate/*_test.go` returns ZERO matches. Go runs
tests SERIALLY within a package unless `t.Parallel()` is called, so swapping the package-global
`os.Stderr` is sequential — the `-race` detector will not flag it. (If a future test in this package adds
`t.Parallel()`, this capture helper would need a mutex — out of scope here; documented as a gotcha.)

**Cleanliness:** the only direct `os.Stderr` write on this path is the progress Fprintf. `deps.Verbose`
writes to `&buf` (not os.Stderr). The stub subprocess's stderr is captured by the provider executor (not
inherited). So the captured temp file should contain the progress line (+ any other direct os.Stderr
writes, which the substring `Contains` check tolerates).

The `captureStderr` helper (new, in multiturn_test.go):
```go
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stderr-*.txt")
	if err != nil { t.Fatalf("create temp stderr: %v", err) }
	orig := os.Stderr
	os.Stderr = f
	os.Stderr = orig // restore immediately after fn (before any t.Errorf, to keep test output clean)
	if err := f.Close(); err != nil { t.Fatalf("close temp stderr: %v", err) }
	b, err := os.ReadFile(f.Name())
	if err != nil { t.Fatalf("read temp stderr: %v", err) }
	return string(b)
}
```
(Restore via BOTH a `defer` and an immediate post-fn assignment so a `t.Fatalf` inside `fn` still
restores via the defer. `os.Stderr` is unbuffered — no flush needed.)

## 6. The test design (mirrors TestMultiTurnTriggerGate_TruthTable's control_all_true_gate_fires case)

```go
func TestCommitStaged_MultiTurnProgressLine_ChunkTokens(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	// ~96 runes ⇒ EstimateTokens ≈ 24 > 4 (cond b satisfied with a small chunk budget)
	writeFile(t, repo, "new.txt", strings.Repeat("change line\n", 8))
	stageFile(t, repo, "new.txt")

	m := stubAppendManifest(t, bin, []string{"", "ok", "ok", "feat: mt win"}, false) // append; call1=""⇒exhaust
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0   // exactly 1 one-shot attempt ⇒ exhaust ⇒ gate fires
	cfg.MultiTurnFallback = true  // cond c
	cfg.MultiTurnChunkTokens = 4  // cond b (24 > 4); small ⇒ a distinctive, easy-to-assert value

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
		t.Errorf("progress line missing %q;\ngot stderr: %q", wantChunk, captured)
	}
	// Sanity: the line is still the multi-turn progress line.
	if !strings.Contains(captured, "falling back to multi-turn") {
		t.Errorf("progress line missing 'falling back to multi-turn';\ngot stderr: %q", captured)
	}
}
```

**Why chunkTokens=4 (not 50000):** the contract's example used 50000, but that needs a >200000-rune
payload (slow, memory-heavy). chunkTokens=4 with the existing ~96-rune fixture (from the TruthTable test)
triggers the gate cheaply, and the substring `"chunks of ~4 tokens"` is unambiguous (4 doesn't appear as
the turn-count or total-min — those are N+1 and ≥1 respectively, distinct values). The assertion builds
the expected substring dynamically (`fmt.Sprintf("chunks of ~%d tokens", cfg.MultiTurnChunkTokens)`) so it
is correct for ANY chunk value.

## 7. Scope + parallel-safety

- **Touched:** `internal/generate/generate.go` (1 Fprintf format string) + `internal/generate/multiturn_test.go`
  (1 new helper `captureStderr` + 1 new test + add `"os"`/`"fmt"` to imports if not present).
- **NOT touched:** `internal/generate/multiturn.go` (S1 ChunkCount), `pkg/stagecoach/stagecoach.go` (P1.M2),
  `internal/hook/exec.go` (P1.M3), docs (P1.M4). The corrected progress line is the COPY SOURCE for
  P1.M2.T1.S2 / P1.M3.T1.S2 (they copy this gate).
- **Parallel with T2.S1 (Issue 4, mtPayload):** T2.S1 edits generate.go:**311** + comment ~L307.
  T3.S1 edits the Fprintf at ~L338. **Distinct regions** (T2.S1 is ABOVE the Fprintf; the only interaction
  is T2.S1's comment growth shifting the Fprintf's line number — which T3.S1 handles by content-based
  grep, §3). Both edit multiturn_test.go (T2.S1 adds a TokenLimit==0 test; T3.S1 adds the progress-line
  test + captureStderr helper) — distinct additions, no overlap. No conflict.
- **DOCS:** none (verbose diagnostic format; the cross-cutting how-it-works.md is P1.M4.T1).

## 8. Decisions log

- **D1** reference the Fprintf by its UNIQUE CONTENT (the format string), not a line number — T2.S1's
  comment edit may shift the line. (§3.)
- **D2** the named tests (TruthTable, RenderContract) do NOT capture the progress line (verbose buffer vs
  os.Stderr) → add a focused unit test; do NOT edit those two tests. (§4.)
- **D3** capture `os.Stderr` via a temp-file swap (race-safe: no `t.Parallel` in the package). (§5.)
- **D4** use `chunkTokens=4` (cheap payload via the existing ~96-rune fixture; substring
  `"chunks of ~4 tokens"` is unambiguous); build the expected substring dynamically. (§6.)
- **D5** assert the NEW field (`chunks of ~N tokens`) + a sanity `Contains("falling back to multi-turn")`;
  do NOT over-assert the exact full line (turns/totalMin numbers are timing/config-dependent and brittle).
- **D6** the corrected line is the copy source for P1.M2/P1.M3 — keep the format canonical
  (`%d turns (chunks of ~%d tokens), ~%dm total`).
