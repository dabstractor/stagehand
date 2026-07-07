# Research Note — P1.M5.T1.S2 (bugfix-002): how-it-works.md overview reconciliation

## Scope
This task owns `docs/how-it-works.md` overview reconciliation for **bugfix-002 Issues 3 & 4 only**:
- **Issue 3** (P1.M3.T1.S1, commit `a9055fa`): merge conflicts now produce a clean single-line message.
- **Issue 4** (P1.M4.T1.S1, commit `04508d3`): `--dry-run` generation failures now exit 1 with a short
  message instead of 3/124 + the full recovery recipe.

Out of scope: Issues 1 & 2 (no how-it-works.md surface — README/cli/config/providers already Mode-A
synced by S1 + implementing subtasks). README is owned by S1 (done, commit `78e6bdd`).

## Mode-A coverage (what the implementing subtasks ALREADY touched — do NOT duplicate)

| Issue | Commit | Docs touched by Mode-A | how-it-works.md? |
|-------|--------|------------------------|------------------|
| 3 | a9055fa | **NONE** (message-only code fix in internal/git/git.go) | ❌ not touched |
| 4 | 04508d3 | `docs/cli.md` only (`--dry-run` flag row line 26 + exit-code note line 86) | ❌ not touched |

**Conclusion:** docs/how-it-works.md is **completely untouched by bugfix-002**. BOTH Issue 3 and
Issue 4 reconciliations are owned by THIS task. (cf. the issue_analysis.md Mode-A notes: Issue 3 →
"if it quotes the merge-conflict wording, align it; else note 'none — message-only change'";
Issue 4 → cli.md only. Both deferred how-it-works.md here.)

## Exact implemented wording (verified from source — quote it accurately in the doc)

**Issue 3 — `internal/git/git.go:230`** (the clean merge-conflict error):
```
"unresolved merge conflicts in the index — resolve them first, then re-run stagecoach"
```
Exit 1, **pre-generation** (WriteTree is step 3, before the model is invoked), HEAD/index untouched,
no snapshot, no rescue. (Probe: `git ls-files -u` non-empty ⇒ return the clean single line.)

**Issue 4 — `internal/cmd/default_action.go:178-185`** (the dry-run RescueError branch):
```
rescue (parse/dup exhaustion): "could not generate a commit message; run without --dry-run to see retries and the recovery recipe"
timeout:                      "generation timed out; run without --dry-run to see the recovery recipe"
→ exitcode.New(exitcode.Error, nil)  // exit 1, no FormatRescue recovery recipe printed
```
The library (`pkg/stagecoach`) is UNCHANGED — it still returns `*RescueError` (3/124); only the CLI
rendering special-cases dry-run.

## Current state of docs/how-it-works.md (verified line numbers)

§"Failure modes and exit codes" table (lines 55-62) — 6 rows, NO merge-conflict row, NO dry-run note:
```
| Agent missing on `$PATH`           | 1 (Error)        | Check the `[provider.<name>] command` path; install the agent |
| Generation failed (parse/retry...) | 3 (Rescue)       | Rescue message with tree SHA |
| Generation timed out               | 124 (Timeout)    | Rescue message with tree SHA |
| CAS failure (HEAD moved meanwhile) | 1 (Error)        | HEAD-moved message |
| Nothing to commit (clean tree)     | 2 (NothingTo...) | Stage files and retry |
| General error                      | 1 (Error)        | Inspect error message |
```
Line 64: `See [cli.md](cli.md#exit-codes) for the full exit-code table.`

§"Rescue protocol" (lines 66-83):
- Line 68 (THE OVER-CLAIM): `When generation fails after the snapshot is taken (exit 3 or 124),
  Stagecoach prints a recovery block to stderr with the frozen tree SHA and the exact git commit-tree
  command to commit manually:` — implies ANY post-snapshot generation failure → 3/124 + recovery block.
  Under `--dry-run` the snapshot IS taken (bugfix-001 Issue 6) but failure → exit 1 + short message.
- Lines 70-82: the rescue block example.
- Line 83: rejected-candidate append note.

## Drift → edits

| # | Issue | Location | Drift | Edit |
|---|-------|----------|-------|------|
| A | 3 | failure-modes table (~57-62) | table OMITS merge conflicts (PRD §18.2 has them); contract OUTPUT = "note merge conflicts → clean message, exit 1" | **REQUIRED** — add a merge-conflict row (exit 1, pre-generation, no snapshot) |
| B | 4 | §"Rescue protocol" intro (line 68) | over-claims recipe applies to all post-snapshot failures; under dry-run failure → exit 1 + short message, no recipe | **REQUIRED** — scope recipe to a real commit; add dry-run exception sentence |
| C | 4 | note under the failure-modes table (~64) | table's 3/124 rows read as universal; contract names this section | **RECOMMENDED** — one-line cross-ref to Rescue protocol |

## Validation harness
- `npx markdownlint-cli2 docs/how-it-works.md` → 0 errors (current baseline confirmed). markdownlint-cli2
  v0.22.1 cached via npx. NOT in CI (ci.yml) — run manually. .markdownlint.json: default=true,
  MD013/MD033/MD060 off (MD060 non-standard → ignored).
- Coherence greps vs docs/cli.md (source of truth, Mode-A synced): dry-run-failure-exit-1 wording.
- `go build ./... && go vet ./... && go test ./...` sanity (no .go touched).
