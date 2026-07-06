# Design Decisions — P1.M3.T2.S2 (runPipeline dry-run hook wiring, FR-V8a)

The authoritative chokepoint map is `architecture/codebase_reality.md` §5 (runPipeline is the dry-run/
SystemExtra path; the runner is the frozen S1/S2 API). This file records the NON-OBVIOUS design calls —
the things a mechanical "call RunCommitHooks" reading would get wrong.

---

## §0 — Scope: TWO inserts in `runPipeline` (pkg/stagehand/stagehand.go) + tests. Nothing else.

The runner (`internal/hooks/runner.go`) is COMPLETE (S1) + the message lifecycle (S2) — FROZEN; this task
CONSUMES its API, does not touch it. `buildDeps` wiring (`Hooks: hooks.DefaultRunner{}` + the
`internal/hooks` import) is S1's job (DONE — S1's Task 5). So this task touches ONLY the body of
`runPipeline`: two nil-guarded insert points (INSERT A = RunCommitHooks; INSERT B = RunPostCommit) and the
new tests. No imports, no buildDeps, no runner.go, no generate.go, no cmd. (Disjoint from S1's regions:
S1 edits buildDeps ~L325-386 + generate.go; this task edits runPipeline ~L650-700. Zero merge overlap.)

---

## §1 — INSERT A goes AFTER EditMessage, BEFORE the `if dryRun` block — ONE shared insert, NOT dry-run-only.

The item contract says "at the point where the message is finalized (post-EditMessage equivalent)." In
runPipeline that point is right after the `generate.EditMessage(...)` call (and its `if err != nil` block)
and right before the `// ---- Dry-run success: skip commit-tree/update-ref. ----` / `if dryRun {…}`
block. Put INSERT A THERE, OUTSIDE the `if dryRun` branch (shared by both the dry-run path and the
SystemExtra-commit path). Pass `dryRun` (the runPipeline param) through to RunCommitHooks.

**Why a single shared insert (not a dry-run-only branch):** `runPipeline` is the commit path for
`opts.DryRun || opts.SystemExtra != ""`. The `!opts.DryRun && opts.SystemExtra != ""` case (a SystemExtra
REAL commit) ALSO flows through runPipeline's commit tail — and FR-V1 says EVERY plumbing-path commit runs
hooks. There is no other task that wires runPipeline's commit tail (S1 wired `CommitStaged`, not
runPipeline; T3 is decompose). So if INSERT A were placed INSIDE `if dryRun`, the SystemExtra-commit path
would be silently hook-less — a FR-V1 violation. The shared insert (dryRun threaded) covers both correctly:
- **dryRun=true:** RunCommitHooks(dryRun=true) skips pre-commit, runs prepare-commit-msg + commit-msg; the
  dry-run block prints the (hook-accepted) `msg`. (The commit-msg-REJECT case is §2.)
- **dryRun=false (SystemExtra commit):** RunCommitHooks(dryRun=false) runs the full pre→prepare→commit-msg
  sequence, reassigns `treeSHA,msg`; the commit tail then CommitTree+UpdateRefCAS on the hook-adjusted
  values; INSERT B fires post-commit. (Mirror of S1's CommitStaged.)

---

## §2 — THE headline design call: commit-msg rejection under --dry-run → WARN-AND-PRINT, NOT a rescue.

The runner (`runner.go` RunCommitHooks) does NOT special-case dry-run for the commit-msg *exit code*: it
gates pre-commit on `cfg.NoVerify || opts.DryRun` (skip), but commit-msg runs under DryRun and a non-zero
exit returns `("", "", &generate.RescueError{Cause: "commit-msg: …"})` — exactly as on the commit path.
On the COMMIT path that *RescueError is the correct FR-V7 rescue (exit 3). On the DRY-RUN path it is WRONG:
FR-V8a's intent is "the user still sees lint results" — a dry-run is a PREVIEW; a commit-msg rejection is
INFORMATION, not a failure. So runPipeline's dry-run branch must CATCH that *RescueError and warn-and-print
(the would-be message + a notice) instead of propagating it as a rescue.

**The discriminator — `errors.As(herr, &*generate.RescueError)`:** under dry-run, a RunCommitHooks error is
warn-and-print ONLY if it is a `*generate.RescueError` (a hook non-zero/timeout: pre-commit is skipped so
this is a prepare-commit-msg or commit-msg rejection). A NON-RescueError (e.g. hooks-dir resolution,
message-file create/write, final read-back — all wrapped `fmt.Errorf`) is an INFRASTRUCTURE failure, not a
lint result, and MUST propagate (something is genuinely broken). So:

```go
if herr != nil {
    if dryRun {
        var re *generate.RescueError
        if errors.As(herr, &re) {           // hook rejection ⇒ warn-and-print (FR-V8a)
            fmt.Fprintf(os.Stderr, "⚠ commit hook rejected the would-be message under --dry-run: %v\n", re.Cause)
            // keep the would-be message so the dry-run Result carries it; runner returned "" on error
            wouldBe := re.Candidate
            if wouldBe == "" { wouldBe = msg }
            msg = wouldBe
            // fall through to the existing `if dryRun` Result (which prints msg)
        } else {
            return Result{}, herr            // infrastructure error ⇒ propagate (not a lint result)
        }
    } else {
        return Result{}, herr                // !dryRun ⇒ rescue (FR-V7, exit 3) — mirrors S1's CommitStaged
    }
} else {
    treeSHA, msg = ft, fm                    // hook accepted (annotated msg) — reassign for downstream
}
```

This leans warn-and-print per the item contract ("dry-run is a preview; a commit-msg rejection is
information, not a failure") and exits 0 (FR49 — dry-run exit 0) even on a lint rejection. The `else`
(reassign `treeSHA,msg = ft,fm`) is the dry-run-SUCCESS path: `fm` is the prepare-annotated, commit-msg-
accepted, comment-stripped message, which the dry-run block then prints.

**Why both prepare-commit-msg AND commit-msg rejections are warn-and-print (not just commit-msg):** under
dry-run, pre-commit is skipped, so a RunCommitHooks *RescueError can only come from prepare-commit-msg or
commit-msg — both are MESSAGE hooks whose rejection is lint-like information. Treating the whole
*RescueError category as warn-and-print is simpler and consistent with "dry-run is a preview" than
string-matching the Cause for "commit-msg". (A real infrastructure error is non-RescueError and propagates.)

---

## §3 — INSERT B (post-commit) goes in the COMMIT TAIL (after UpdateRefCAS, before ClearSnapshot).

The commit tail (CommitTree → RestoreDefault → UpdateRefCAS → ClearSnapshot → Result) is reached ONLY when
`!dryRun` (the `if dryRun` block returns early above it). Put INSERT B there, nil-guarded, after the
UpdateRefCAS `if err != nil {…}` block and before `signal.ClearSnapshot()`:

```go
if deps.Hooks != nil {
    _ = deps.Hooks.RunPostCommit(ctx, deps.Git, cfg, dryRun, deps.Verbose) // exit disregarded (FR-V7)
}
```

`dryRun` is false here (the commit tail is !dryRun-only), so passing `dryRun` is equivalent to `false`;
RunPostCommit also self-guards (`if opts.DryRun { return nil }`). The return is discarded — post-commit's
exit code is DISREGARDED (the commit already landed; FR-V7). This is byte-for-byte the same INSERT B S1 put
in CommitStaged. (Under dry-run this code is unreachable, so post-commit is correctly skipped — FR-V8a.)

---

## §4 — The runner already handles DryRun INTERNALLY; just pass `dryRun` through.

`HookOpts{DryRun: …}` is honored inside `runner.go` (S1/S2), NOT by the caller:
- **pre-commit:** `if !(cfg.NoVerify || opts.DryRun)` → skipped under DryRun (FR-V8a). ✓
- **prepare-commit-msg:** ALWAYS runs (NoVerify/DryRun don't gate it — git parity). ✓
- **commit-msg:** `if !cfg.NoVerify` → RUNS under DryRun (FR-V8a: lint the would-be message). ✓
- **RunPostCommit:** `if opts.DryRun { return nil }` → self-skips under DryRun (FR-V8a). ✓

So the caller (runPipeline) does NOT re-implement dry-run gating — it passes `dryRun` and the runner does
the rest. The ONLY caller-side dry-run logic is the §2 warn-and-print for a commit-msg rejection (because
the runner intentionally returns the same *RescueError shape under dry-run; the dry-run SEMANTICS of that
error — "information, not failure" — belong to the caller, since the caller knows it is a dry-run preview).

The adapter (`hooks.DefaultRunner`, S1's NEW adapter.go) translates the inlined `(dryRun, verbose)` to
`HookOpts{DryRun: dryRun, Verbose: verbose}` — so runPipeline calls `deps.Hooks.RunCommitHooks(ctx,
deps.Git, cfg, treeSHA, parentSHA, msg, dryRun, deps.Verbose)` and the adapter does the HookOpts mapping.

---

## §5 — The would-be message printed under rejection is `RescueError.Candidate` (≈ the post-EditMessage msg).

On a commit-msg rejection the runner returns `("", "", *RescueError{Candidate: finalMsg, …})`. Tracing
`finalMsg` under dry-run: it starts as the passed-in `msg`; pre-commit is skipped (no mutation); the message
file is written with `finalMsg`; prepare-commit-msg may annotate the FILE but `finalMsg` is NOT reassigned
until the post-commit-msg read-back (which never runs on rejection). So `Candidate` == the post-EditMessage
`msg` (the prepare annotation, if any, is lost on the rejection path — an edge case of an edge case:
dry-run + prepare annotates + commit-msg rejects). For the common dry-run case (no prepare hook, or prepare
absent) Candidate == the generated message == exactly what the user wants to see.

So `wouldBe := re.Candidate; if wouldBe == "" { wouldBe = msg }` is correct and robust. (The success path
uses `fm` — the read-back, comment-stripped, prepare-annotated message — which DOES include the annotation.)

---

## §6 — stderr/stdout separation (FR51): message → stdout; warning + hook stderr → stderr.

`printDryRunMessage(stdout, res.Message)` (cmd/default_action.go:201,438) writes the would-be message to
**stdout** — stdout must stay CLEAN (FR51: stdout is the message for piping). My `fmt.Fprintf(os.Stderr,
"⚠ …")` notice and the runner's verbatim hook-stderr passthrough (`cmd.Stderr = os.Stderr` in runHook) both
go to **stderr**. So the lint result + the notice are on stderr (visible, but not polluting the piped
message). This matches runPipeline's existing non-verbose stderr writes (e.g. the `↳ falling back to
multi-turn` line) and FR51. The warning is ALWAYS printed (not Verbose-gated) — FR-V8a's whole point is the
user SEES the lint result, and the runner already passes the hook's own stderr through verbatim regardless
of Verbose.

**Test implication:** to assert "the lint result surfaced," capture `os.Stderr` during the GenerateCommit
call (os.Pipe) and assert it contains the hook's distinctive stderr message. SAFE because the stagehand_test
suite has ZERO `t.Parallel()` calls (verified) — a global os.Stderr swap does not race. Restore os.Stderr in
a t.Cleanup. (Do NOT add t.Parallel() to these tests.)

---

## §7 — Coordination with S1 (parallel): DISJOINT regions of stagehand.go; no merge conflict.

S1 (P1.M3.T2.S1) edits stagehand.go's `buildDeps` (~L325-386: adds the `internal/hooks` import + `Hooks:
hooks.DefaultRunner{}`) AND generate.go. THIS task edits stagehand.go's `runPipeline` (~L411-700: the two
inserts). Different line ranges, different functions — no textual overlap. `deps.Hooks` is ALREADY wired
into the Deps that runPipeline receives (S1's buildDeps), so runPipeline finds `deps.Hooks != nil` at
runtime with no buildDeps change here. If S1 has not landed yet, `deps.Hooks` is nil → both inserts are
no-ops → runPipeline behaves byte-identically to today (hooks skipped) — so this task is SAFE to land
before or after S1, and the dry-run tests that assert hook behavior require S1's buildDeps wiring to be
present (they go through GenerateCommit → buildDeps, which S1 wires).

---

## §8 — No new imports; go.mod/go.sum unchanged.

stagehand.go ALREADY imports `errors`, `fmt`, `os`, and `generate` (verified). INSERT A uses `errors.As`,
`fmt.Fprintf`, `os.Stderr`, and `*generate.RescueError` — all already imported. INSERT B uses nothing new.
`internal/hooks` is NOT imported by stagehand.go (it's imported by S1's buildDeps edit, and the adapter is
referenced as `hooks.DefaultRunner{}` only in buildDeps — runPipeline accesses hooks only via the
`deps.Hooks CommitHookRunner` interface, which is in `generate`). So runPipeline adds NO import.
`go mod tidy` is a no-op.

---

## §9 — Test strategy (package stagehand, white-box via the exported GenerateCommit; mirror TestGenerateCommit_DryRun).

Route through the EXPORTED `GenerateCommit(ctx, Options{Provider: "stub", DryRun: true})` (the existing
dry-run test pattern) — it calls buildDeps (S1 wires DefaultRunner) → runPipeline. Install the hook in
`repo/.git/hooks/<name>` with mode 0o755 (the runner's `hookExecutable` checks the owner-exec bit 0o100).
Required cases:

1. **TestGenerateCommit_DryRun_CommitMsgReject_PrintsMessage** (THE headline FR-V8a test): commit-msg hook
   exits 1 AND echoes a distinctive line to stderr ("reject: bad format"); DryRun:true; assert
   `err == nil` (warn-and-print, exit 0), `res.Message` non-empty (the would-be message printed),
   `res.CommitSHA == ""` + HEAD unchanged (nothing committed), AND captured stderr contains the hook's lint
   message (the result surfaced). Captures os.Stderr (no t.Parallel).
2. **TestGenerateCommit_DryRun_SkipsPreCommit** (FR-V8a pre-commit-skip): a pre-commit hook that exits 1
   (would ABORT if run); DryRun:true; commit-msg absent; assert `err == nil` + `res.Message` non-empty →
   proves pre-commit was SKIPPED under dry-run (had it run, exit 1 → rescue → error).
3. **TestGenerateCommit_DryRun_CommitMsgAccept** (commit-msg RUNS under dry-run): a commit-msg hook that
   exits 0 (annotates or no-ops); DryRun:true; assert `err == nil` + `res.Message` non-empty → proves
   commit-msg ran and accepted under dry-run.
4. (Recommended) **TestGenerateCommit_SystemExtra_PreCommitAbort_Rescue**: pre-commit exits 1;
   `SystemExtra:"extra"` (forces runPipeline's !dryRun commit tail), DryRun:false; assert the run returns a
   `*generate.RescueError` (errors.As) + HEAD unchanged → proves INSERT A's !dryRun branch (the SystemExtra
   commit path) maps a hook abort to the rescue (FR-V7), mirroring S1's CommitStaged. (Covers the shared
   insert's non-dry-run side, which no other task tests.)

Mirror setupTestRepo (stub provider via repo-local .stagehand.toml) + initRepo + chdir + a staged file.
The stub message must be UNIQUE (not duplicating the seed subject) so the dedupe loop accepts it on attempt 0.
