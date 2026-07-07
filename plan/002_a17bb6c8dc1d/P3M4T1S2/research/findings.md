# P3.M4.T1.S2 Research Findings — Per-Concept Failure Isolation (FR-M12) + Multi-Commit Rescue

## 1. What S1 (P3.M4.T1.S1) leaves behind — the CONTRACT (consume verbatim)

S1 creates `internal/decompose/decompose.go` + `decompose_test.go` and edits `roles.go` (adds the
unexported `stager` test-seam field to `Deps`). S2 EDITS those same files. Key S1 surfaces S2 consumes:

- `Decompose(ctx, deps) (DecomposeResult, error)` — entry point; routes by mode; calls `runLoop`;
  gates arbiter on `StatusPorcelain != ""`.
- `runLoop(ctx, deps, concepts, baseTree, preRunHEAD, isUnborn) ([]CommitResult, []ChainEntry, error)`
  — the 1-deep-overlap loop. STRUCTURE (verbatim from S1 PRP):
  ```
  for i, concept := range concepts {
      if err := invokeStager(ctx, deps, concept); err != nil { drainMsg(inflight); return nil,nil,err } // S1 PROPAGATES
      treeI, err := freezeSnapshot(ctx, deps); if err != nil { drainMsg(inflight); return nil,nil,... }
      skipped := treeI == prevTree              // FR-M8 part (a): empty-skip
      if err := publish(inflight); err != nil { return nil,nil,err }   // S1 PROPAGATES msg/CAS errors
      inflight = nil
      if !skipped { inflight = launch(i, prevTree, treeI); prevTree = treeI }
  }
  if err := publish(inflight); err != nil { return nil,nil,err }
  return commits, chainData, nil
  ```
  - `publish` is a CLOSURE: `res := <-ch; if res.err != nil { return res.err }; publishCommit(...)`.
  - `launch(i, treeA, treeB) chan msgOut` — spawns generateMessage in a goroutine, buffered(1) chan.
  - `msgOut{conceptIdx, treeA, treeB, msg, err}` — UNEXPORTED, local to runLoop.
- `DecomposeResult{Commits []CommitResult; Amended int}` + `CommitResult{SHA, Subject, Message, Files}`.
- `ErrDecomposeFailed` sentinel. `invokeStager` test-seam (deps.stager if non-nil else stageConcept).
- `runArbiterPhase`, `computeAmended`, `buildCommitResult`, `dupCheckMessage` helpers.
- `Deps{Git, Registry, Config, Roles RoleManifests, Verbose *ui.Verbose, stager ...}` — S2 ADDS `Out io.Writer`.

S1 is SIGNAL-FREE for the loop/shortcut (`internal/signal` NOT imported in decompose.go). S1's PRP
explicitly defers to S2: "signal arming is S2 (P3.M4.T1.S2)" + "S2 will add loop signal arming + the
multi-commit rescue variant" + "S1 propagates structurally; S2 wraps these in FR-M12 isolation."

## 2. The consumed primitives' error contracts (verified in source)

- `generateMessage(ctx, deps, treeA, treeB) (string, error)` (message.go):
  - SUCCESS → message string.
  - GEN FAILURE (timeout/parse/dup-exhausted/non-zero-exit/cancel) → `*generate.RescueError{Kind,
    TreeSHA: treeB, ParentSHA: <RevParseHEAD at call time>, Candidate, Cause}` DIRECTLY (not wrapped).
    **CRITICAL: ParentSHA = RevParseHEAD() inside generateMessage. By the time msg[i] runs, the loop
    has ALREADY published commit[i-1] (publish(msg[i-1]) precedes launch(msg[i])), so HEAD ==
    newSHA[i-1]. So RescueError.ParentSHA == newSHA[i-1] == the correct parent for concept i's
    rescue. The §18.3 multi-commit variant's "parent (newSHA[i-1])" is ALREADY in the error.**
  - INFRA FAILURE (TreeDiff err / RevParseHEAD err / render err / empty-diff) → `ErrMessageFailed`-wrapped.
- `publishCommit(ctx, deps, tree, parentSHA, msg) (string, error)` (message.go):
  - SUCCESS → newSHA.
  - CAS FAILURE → `*generate.CASError{TreeSHA, Expected, Actual, Message}` DIRECTLY (errors.As-able;
    `ce.Error()` IS the §13.5 "HEAD moved…" message WITH the tree[i] recovery command baked in).
  - CommitTree FAILURE → `ErrPublicationFailed`-wrapped. Non-CAS UpdateRefCAS → propagated verbatim.
- `stageConcept(ctx, deps, concept) error` (stager.go): `ErrStagerFailed`-wrapped on any failure;
  nil on success. NO retry (the orchestrator owns FR-M8/M12 retry). `freezeSnapshot(ctx, deps) (string, error)`.

## 3. Error types + exit-code mapping (verified)

`internal/generate/generate.go`:
```go
type RescueError struct { Kind error; TreeSHA, ParentSHA, Candidate string; Cause error }
func (e *RescueError) Error() string; func (e *RescueError) Unwrap() error { return e.Kind }
// Kind ∈ {generate.ErrTimeout, generate.ErrRescue}

type CASError struct { TreeSHA, Expected, Actual, Message string }
func (e *CASError) Error() string  // IS the §13.5 "HEAD moved from <E> to <A>… git commit-tree -p <E> -m %q <TreeSHA> | xargs git update-ref HEAD"
func (e *CASError) Unwrap() error { return git.ErrCASFailed }
```
`internal/exitcode` `For(err)` mapping (S2 relies on this via errors.Is traversal):
- `errors.Is(err, generate.ErrRescue)` → 3 (Rescue); `errors.Is(err, generate.ErrTimeout)` → 124 (Timeout).
- `errors.Is(err, generate.ErrCASFailed)` → 1 (Error).
So a wrapper that `Unwrap()`s to `*RescueError` (→ Kind) maps correctly; the raw `*CASError` maps to 1.

## 4. FormatRescue + rescueSep (internal/generate/rescue.go) — the pattern to extend

`FormatRescue(treeSHA, parentSHA, candidateMsg) string` is the §18.3 base rescue (FROZEN signature —
do NOT change it; main.go wires `signal.Options.RescueFormat = generate.FormatRescue`). It uses the
package-private `rescueSep = "----…----"` (exactly 60 '-'). The §18.3 **multi-commit variant** (last
¶ of §18.3): "print tree[i], its parent (newSHA[i-1]), and the same commit-tree|update-ref recipe.
Already-published commits 0..i-1 are final and untouched; any concepts whose staging completed remain
staged for the user to finish. The arbiter is not run when the loop aborts via rescue."

**DECISION: add `FormatRescueMulti(treeSHA, parentSHA, candidateMsg, conceptTitle string, index, count int) string`
to `internal/generate/rescue.go` (SAME package — reuses `rescueSep`; mirrors FormatRescue's recipe
lines exactly; adds a concept-naming header + a multi-commit reassurance line).** This is "extend
generate" per the work item. decompose calls `generate.FormatRescueMulti(...)`.

## 5. signal package (internal/signal/signal.go) — the one-shot RestoreDefault GOTCHA

- `signal.SetSnapshot(treeSHA, parentSHA, candidate)` — arms the snapshot (mutex-protected; nil-safe
  no-op if Install wasn't called). The handler on Ctrl-C calls `opts.RescueFormat(tree, parent, cand)`
  (= generate.FormatRescue, the BASE form) → correct for multi-commit too (parent is right).
- `signal.ClearSnapshot()` — disarms (sets snap fields to "").
- `signal.RestoreDefault()` — **ONE-SHOT + PERMANENT**: `stopped.CompareAndSwap(false,true)` then
  `signal.Stop(h.ch); close(h.ch)`. After it, the handler is DEAD for the rest of the process.
  → **The loop CANNOT call RestoreDefault per-concept (it would kill the handler after concept 0).
  S2 uses SetSnapshot/ClearSnapshot toggling per-concept instead; RestoreDefault stays single-commit-
  only (CommitStaged's internal §18.4 step 3).**
- signal imports NO stagecoach packages (stdlib-only) → decompose → signal is NOT an import cycle.
- In tests without Install, SetSnapshot/ClearSnapshot are nil-safe no-ops.

## 6. Where the loop prints the rescue (Deps.Out — S2 adds it)

`ui.Verbose.w` is UNEXPORTED (no public writer accessor). The loop needs an `io.Writer` to print the
multi-commit rescue. **DECISION: S2 adds `Out io.Writer` to `Deps` (roles.go).** In prod the CLI
passes `cmd.ErrOrStderr()` (stderr, matching the single-commit rescue destination); tests pass a
`*bytes.Buffer` to assert the printed message. Mirrors how `generate.Deps` is injectable.

## 7. §18.2 failure table — the authoritative exit-code semantics (verified)

| Failure (v2) | Response | Exit |
|---|---|---|
| Stager stages nothing / exits non-zero twice | skip concept (no empty commit); log; continue | **0** |
| `message[i]` fails mid-loop | rescue **for concept i only** (§13.6.6); prior commits stand | **3** |
| Arbiter invalid/unknown target | default to a NEW commit (null) | 0 |
| `update-ref` CAS failure (HEAD moved) | print message + manual recovery (do NOT force) | **1** |
| Planner unparseable/fails | surface error; nothing snapshotted yet | 1 |
| Decompose exceeds max_commits | error: raise --max-commits/--commits | 1 |

So: message[i] rescue → exit 3 (Rescue); CAS → exit 1 (Error); stager-fail-twice → continue (exit 0
eventually, or whatever the run's final outcome is).

## 8. The overlapped stager[i+1] is ALREADY complete when msg[i] fails (key insight)

S1's loop is `iter i+1: stage[i+1] (msg[i] in flight) → freeze tree[i+1] → publish(msg[i])`. stage[i+1]
runs SYNCHRONOUSLY before publish(msg[i]) drains msg[i]. So when publish(msg[i]) receives msg[i]'s
RescueError, **stage[i+1] has ALREADY completed** — its staging is in the live index (frozen into
tree[i+1]). §13.6.6's "The overlapped stager[i+1], if already running, is allowed to complete so its
staging is not lost" is AUTOMATICALLY satisfied by S1's synchronous-then-drain ordering. S2 does NOT
need a wait/drain for stager[i+1]; it only must NOT reset the index (it never does). The partial
result returns with concepts[0..i+1] staged for the user.

## 9. CLI double-print coordination (flagged for P4, NOT S2)

`internal/cmd/default_action.go` `handleGenError` prints `FormatRescue(...)` for `*RescueError` and
`ce.Error()` for `*CASError`. S2's loop ALREADY prints the multi-commit rescue / §13.5 message to
`deps.Out`. To avoid double-print, P4's decompose CLI handler (P4.M1.T1.S1) must NOT re-print when
Decompose returns a partial result + typed error (the loop already printed). `exitcode.For` still maps
the exit code via `errors.Is` (works through S2's DecomposeRescueError Unwrap chain). S2 is INTERNAL
error handling; the CLI routing is P4. (handleGenError is single-commit-specific; P4 adds a decompose
branch.) S2's loop prints + returns the typed error; the exit code is honored.
