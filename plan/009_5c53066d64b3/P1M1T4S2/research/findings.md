# Research Note — P1.M1.T4.S2: Multi-turn failure/skip integration tests

> Companion to `../PRP.md`. Captures the three findings that determined S2's scope and mechanism.
> Read this if you wonder why S2 has only 3 tests (not a re-test of everything) or how a "mid-turn
> failure" is triggered without modifying the stub.

## 1. T3.S3 (COMPLETE) already covers the skip paths WEAKLY — S2's value is the invariant depth

`internal/generate/generate_test.go` lines 868-941 (added by P1.M1.T3.S3, COMPLETE) already contain:

| Test (line) | Scenario | What it asserts | Gap S2 fills |
|---|---|---|---|
| `TestCommitStaged_MultiTurnFallbackSuccess` (868) | happy path | commit lands | none (T4.S1 deepens render) |
| `TestCommitStaged_MultiTurnSkipped_NonAppend` (895) | cond (d) false | `Kind==ErrRescue` only | idempotent-index + counter==1 |
| `TestCommitStaged_MultiTurnSkipped_SmallPayload` (918) | cond (b) false | `Kind==ErrRescue` only | idempotent-index + counter==1 |
| `TestCommitStaged_MultiTurnDuplicateRescue` (941) | final-turn dedupe fail | `Kind==ErrRescue` + Candidate | (different path — final turn, not mid-turn) |

**Implication:** S2 must NOT re-add Kind-only skip tests. S2's UNIQUE contributions:
- **(a) Mid-turn Execute-error → rescue** — entirely UNCOVERED. The duplicate test (941) completes all
  N+1 turns and fails on the FINAL turn's dedupe; NOTHING exercises `multiturn.Run`'s per-turn abort
  (`return "", false, execErr`) at turn 1..N. Only scenario (a) reaches it.
- **(b)/(c) invariant strengthenings** — TreeSHA==frozen, ParentSHA, atomic-HEAD (§18.1), idempotent-index
  (§20.2), and the stub-counter proof that Run was/wasn't entered. T3.S3 asserts none of these.

The canonical idempotent template is `TestCommitStaged_IdempotentIndexOnFailure` (generate_test.go:491) —
it snapshots `beforeHEAD`/`beforeIndex`/`beforeIndexFull` and compares after. S2's `assertMultiTurnRescue`
helper mirrors it and adds TreeSHA==frozen (`git write-tree` before the run) + the counter check.

## 2. Mid-turn failure mechanism: GLOBAL `STAGEHAND_STUB_EXIT=1` (no stub change)

`cmd/stubagent/main.go` has exactly ONE exit knob: `os.Exit(envInt("STAGEHAND_STUB_EXIT", 0))` — a single
value applied to EVERY call. `STAGEHAND_STUB_SCRIPT` varies OUTPUT only (via `selectScripted`), never the
exit code. There is NO per-call exit without modifying the stub, which:
- T4.S1's PRP explicitly forbids ("do NOT modify cmd/stubagent"), and
- this item's "MOCKING: stub agent + temp git repo (existing harness)" constraint discourages.

**So set `m.Env["STAGEHAND_STUB_EXIT"] = "1"` globally.** Traced through `CommitStaged` (generate.go):

1. **One-shot (call 0):** `Execute` returns a wrapped `*exec.ExitError` (executor.go:77). The loop's
   non-zero-exit branch (~line 258): NOT `DeadlineExceeded`, NOT `Canceled` → `lastCause = execErr`,
   **falls through** to `ParseOutput("")` (script[0] is `""`) → `ok=false` → retry. With
   `MaxDuplicateRetries=0`, the loop exhausts after 1 iteration → `!success` → **the FR-T1 gate fires**
   (conds a/b/c/d all hold).
2. **Run turn 1 (call 1):** `Execute` → wrapped `*exec.ExitError` → `multiturn.Run` returns
   `("", false, execErr)` (the turn-1 abort branch) → the gate sets `lastCause = cause` and falls through
   to the byte-identical rescue (`Kind: ErrRescue`, generate.go ~327).

**The failure lands on turn 1** (Run aborts at the FIRST error — `multiturn.go`'s turn-1/2..N/N+1 branches
all share `return "", false, execErr`). The contract's "exit 1 on turn 2" was illustrative; turn-1 failure
is equivalent FR-T7 coverage. The stub counter reads `2` (1 one-shot + 1 turn-1) — proving the gate FIRED
and Run aborted at turn 1 (did not continue to turns 2..N+1).

`re.Cause != nil` holds because the gate does `lastCause = cause` (the multi-turn error supersedes the
one-shot's exit-1 `lastCause`).

## 3. The stub counter is the "was Run entered?" discriminator

`stubtest.NewScript`/`appendScriptManifest` write a counter file (`m.Env["STAGEHAND_STUB_COUNTER"]`);
`cmd/stubagent`'s `selectScripted` does `readCounter` → `writeCounter(N+1)` per call. So after a run:

| Counter | Meaning | Scenario |
|---|---|---|
| `"1"` | ONLY the one-shot ran → gate SKIPPED Run (cond b or d false) | (b) small-payload, (c) non-append |
| `"2"` | one-shot + exactly 1 multi-turn turn → gate FIRED, Run aborted at turn 1 | (a) mid-turn failure |

This is the assertion T3.S3's skip tests LACK — they assert `Kind==ErrRescue` but cannot prove Run was
never entered. Reading `m.Env["STAGEHAND_STUB_COUNTER"]` (mutable Env map; persistent through
`CmdSpec.Env → cmd.Env`) closes that gap with zero stub change.

## 4. ErrRescue (exit 3), NOT ErrTimeout (exit 124)

`research-generate-config.md` §5 + generate.go ~327-330: a multi-turn turn error maps to
`&RescueError{Kind: ErrRescue}` byte-identically to one-shot-exhaustion. `ErrTimeout` is reserved for the
ONE-shot `DeadlineExceeded` kill (generate.go ~244). So asserting `re.Kind == ErrRescue` IS the FR-T7
"never worse than one-shot-exhausted" proof — same Kind + same TreeSHA + same ParentSHA ⇒ same
`FormatRescue` message + exit 3.

## 5. Evidence / verification

- `internal/generate/generate_test.go` read (lines 162-965): confirmed T3.S3's 4 multi-turn tests
  (868-941) are Kind-only; confirmed `TestCommitStaged_IdempotentIndexOnFailure` (491) idempotent template;
  confirmed `appendScriptManifest` (857) sets `SessionMode=&"append"`.
- `internal/generate/generate.go` read (gate ~290-330): confirmed the FR-T1 conditions (a-d), the
  `cause != nil || ok2==false ⇒ lastCause=cause/candidate=msg2 ⇒ byte-identical rescue` fall-through, and
  the one-shot non-zero-exit fall-through (~258) that lets global EXIT=1 still reach the gate.
- `internal/generate/multiturn.go` read: confirmed Run's per-turn `return "", false, execErr` abort branch.
- `internal/provider/executor.go` grep (line 77): confirmed `return out, errb, fmt.Errorf("provider %q: %w",
  spec.Command, werr)` on non-zero exit ⇒ `re.Cause != nil`.
- `cmd/stubagent/main.go` read: confirmed single global EXIT knob; `selectScripted` counter read+write.
- `internal/stubtest/stubtest.go` read: confirmed `NewScript` counter setup + mutable `Env` map.

**One-pass-success confidence: 9/10.** Residual risk: an implementer who didn't read T3.S3's existing tests
might re-add Kind-only skip tests (duplication) or try to modify the stub for a per-call exit (forbidden).
The PRP's "NON-NEGOTIABLE: do NOT duplicate T3.S3" callout and the global-EXIT mechanism section exist to
prevent both.
