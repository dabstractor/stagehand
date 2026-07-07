# P1.M4.T4.S1 Research Findings — `--dry-run` stderr notice

## TL;DR
`stagecoach --dry-run` is **~95% already shipped**. The flag, the `DryRun` pass-through to the public
API, the dry-run success branch, message→stdout, exit 0, HEAD-unchanged, and the `↳ Generating…`
progress line are ALL present (P1.M4.T1.S2 default action + P1.M3.T5.S1 public API). The default-action
author **explicitly deferred exactly one decoration** to P1.M4.T4 — visible in the source comment at
`internal/cmd/default_action.go:128`:

> `// / "(no commit created)" decorations are P1.M4.T3/T4.`

**The single, unambiguous deliverable of P1.M4.T4.S1: print `(no commit created)` to STDERR in the
dry-run success path, and assert it in the existing CLI test.** Everything else is verification-only.

## §1 Contract vs. reality (line-by-line)

| Contract clause (item_description) | Status | Evidence |
|---|---|---|
| RESEARCH: "public API already supports DryRun in Options (P1.M3.T5.S1)" | ✅ DONE | `pkg/stagecoach/stagecoach.go:36` `DryRun bool`; `runPipeline` dry-run branch L254-276 |
| INPUT: "GenerateCommit with DryRun=true" | ✅ DONE | `internal/cmd/default_action.go:118` `DryRun: flagDryRun` |
| LOGIC: "Print the message to stdout (clean, for piping)" | ✅ DONE | `printDryRunMessage(stdout, res.Message)` L129 + helper L194-196 |
| LOGIC: "Print '(no commit created)' to stderr" | ❌ **GAP (THE deliverable)** | no such line anywhere; deferred in comment L128 |
| LOGIC: "Exit 0" | ✅ DONE | `return nil` L130; `exitcode.Success=0` doc notes "dry-run message printed" (`exitcode.go:23`) |
| LOGIC: "commit-tree/update-ref are skipped" (HEAD unchanged) | ✅ DONE | `runPipeline` dry-run returns before CommitTree/UpdateRefCAS; proven by `TestRunDefault_DryRun` (HEAD unchanged) |
| Mock: "integration test — dry-run produces a message, HEAD unchanged" | ✅ EXISTS (needs +1 assertion) | `TestRunDefault_DryRun` (`default_action_test.go:252`) asserts stdout=msg + HEAD unchanged + err==nil |
| OUTPUT: "Working stagecoach --dry-run" | ✅ (after the 1-line fix) | works end-to-end once the stderr notice is added |
| DOCS: "none" | ✅ N/A | CLI help already shows `--dry-run` (`root.go:89`); README is P1.M5.T4 |

## §2 The one gap — `(no commit created)` to stderr

Current code (`internal/cmd/default_action.go:126-130`):
```go
if flagDryRun || res.CommitSHA == "" {
    // Dry-run (Appendix B.3): stdout = the message ONLY (§15.5 pipe use case). The "↳ Generating…"
    // / "(no commit created)" decorations are P1.M4.T3/T4. No commit was created.
    printDryRunMessage(stdout, res.Message)
    return nil // exit 0
}
```
**Fix:** add `fmt.Fprintln(stderr, "(no commit created)")` between the two lines. Plain (no `↳ ` prefix,
no color) — Appendix B.3 shows it verbatim/plain, and the contract says "Print '(no commit created)'"
verbatim. On STDERR so stdout stays clean for piping (`stagecoach --dry-run --no-color | tee`).

The `↳ Generating…` progress (also Appendix B.3) is **already present** — emitted to stderr by
`u.Progress(label)` earlier in `runDefault` (default_action.go ~L110). P1.M4.T3.S1 owns it; it is done.

## §3 Why NOT to touch the public API (snapshot discrepancy — scoped OUT)

The contract prose says *"The snapshot is still taken (write-tree runs) but commit-tree/update-ref are
skipped."* The public API (`pkg/stagecoach.runPipeline` L221-228) **skips `WriteTree` in dry-run** (and
also skips duplicate-check — the dry-run branch does a single generate→parse pass):

```go
// Step 3 (commit path only): snapshot. DryRun skips it (no commit → no object-store write).
var treeSHA string
if !dryRun {
    treeSHA, err = deps.Git.WriteTree(ctx)
    ...
}
```

**Decision: do NOT modify the public API in P1.M4.T4.S1.** Rationale:
1. **Owned by a COMPLETE task.** `runPipeline` is the deliverable of P1.M3.T5.S1 (public API), marked
   *Complete*. Editing it is scope creep into a finished, tested unit.
2. **The contract's OWN mock doesn't verify it.** The item says: *"Mock: integration test — dry-run
   produces a message, HEAD unchanged."* Write-tree is not in the mock. Skipping it is invisible to the
   user (an orphan tree object is never created; nothing observable changes).
3. **It's a defensible optimization.** No commit → no object-store write needed. The author commented it
   deliberately ("no commit → no object-store write").
4. **All OBSERVABLE success criteria are already satisfied:** message produced ✓, HEAD unchanged ✓,
   exit 0 ✓. FR49's load-bearing clause is *"do not create the commit or move HEAD"* — both honored.
5. **Risk of destabilization.** `TestGenerateCommit_DryRun` + `TestGenerateCommit_Timeout` "dryrun"
   (`pkg/stagecoach/stagecoach_test.go`) pin the current dry-run error shapes (bare `ErrTimeout`, no
   `*RescueError`, no `TreeSHA`). Re-running write-tree would require re-arming the rescue path and
   reshaping these errors — a P1.M3.T5.S1 regression, not a P1.M4.T4.S1 concern.

**Scope boundary:** P1.M4.T4.S1 owns the CLI default-action output ONLY (`internal/cmd/default_action.go`
+ its test). The public API's internal dry-run mechanics are frozen.

## §4 Test plan (extend the EXISTING test, do not fork it)

`TestRunDefault_DryRun` (`internal/cmd/default_action_test.go:252`) already asserts stdout==message,
HEAD unchanged, err==nil. **Add two assertions** to it (mirrors FR49 + Appendix B.3):
```go
// stderr MUST contain "(no commit created)" (Appendix B.3) …
if !strings.Contains(errBuf.String(), "(no commit created)") { t.Errorf(...) }
// … and stdout MUST NOT (pipeable — §15.5).
if strings.Contains(stdout, "(no commit created)") { t.Errorf(...) }
```
`strings` is already imported in the test file (it uses `strings.TrimSpace`). No new imports.

## §5 Parallel coordination (P1.M4.T3.S3 in flight)

- **P1.M4.T3.S3** (exit-code verify/harden) edits **ONLY `internal/exitcode/`**. **Zero overlap** with
  P1.M4.T4.S1 (which edits `internal/cmd/default_action.go` + `default_action_test.go`).
- **P1.M4.T3.S2** (verbose) — the S3 PRP flagged it as "editing default_action.go one line (`Verbose:
  stderr`)". Per plan_status it is now **Complete**, and the current source ALREADY contains
  `Verbose: stderr` at default_action.go:119. No conflict.
- Safe to edit `internal/cmd/default_action.go` now.

## §6 Confidence: 9.5/10

Tiny, surgical change (1 `fmt.Fprintln` line + 2 test assertions) in well-trodden, already-tested code.
All surrounding wiring is present and green. Only residual risk: the snapshot prose discrepancy (§3),
which is explicitly scoped out as a non-observable, Complete-task-owned concern.
