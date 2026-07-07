# P1.M3.T2.S1 Research Findings — rereadFinalCommits after arbiter

## Prerequisite confirmed in place (T1.S1 = Complete)

`git.LogRange` is implemented (`internal/git/git.go`):
- `LogEntry{SHA, Subject}` type (L24).
- `Git.LogRange(ctx, baseSHA) ([]LogEntry, error)` interface method (L209) + `gitRunner` impl (L783).
- All-zeros sentinel (`strings.Repeat("0",40)`) → **no-range `HEAD` form** (`<zeros>..HEAD` is invalid — exit 128).
- Returns oldest-first (`--reverse`); empty/truly-unborn → `(nil, nil)` (128-as-non-error).
- `%H%x1f%s` delimiter, parsed per-line via `strings.Cut(line, "\x1f")`.

`git.DiffTree(ctx, sha, isRoot) ([]FileChange, error)` exists (L434).

## The edit site (decompose.go)

- **Step (2)** captures `preRunHEAD, isUnborn, err := deps.Git.RevParseHEAD(ctx)` (~L114).
- **Step (7)+(8)** arbiter block (~L178-189):
  ```go
  if status != "" && len(commits) > 0 {
      arbiterCommits := buildArbiterCommits(ctx, deps, commits, isUnborn)
      amended, err = runArbiterPhase(ctx, deps, arbiterCommits, chainData)
      if err != nil { return DecomposeResult{}, err }
      // <-- INSERT rereadFinalCommits call HERE (after success, before step 9)
  }
  // (9) Return: DecomposeResult{Commits: commits, Amended: amended}
  ```
- `CommitResult` struct (L43) + doc (L40-42) and `DecomposeResult` (L65) + §G-RESULT doc (L55-67) both
  describe the "post-arbiter gap" / "PRE-amend" — BOTH must be rewritten (gap is now closed).

## CRITICAL gotchas (will break a one-pass impl if missed)

1. **ADD `"strings"` import** to decompose.go (current imports: context/errors/fmt + 4 internal pkgs;
   NO `strings`). Needed for `strings.Repeat("0", 40)`.
2. **UPDATE existing `TestDecompose_ArbiterWiring`** (decompose_test.go ~L744): asserts
   `len(result.Commits) != 1` → MUST become `!= 2` (null-path commit is now re-read & included).
   `result.Amended == 0` stays correct (null target → 0). Two other full-Decompose arbiter tests
   (`TestDecompose_RoleResolvesSubProvider` ~L1500, the stager-retry test ~L1105) leave a CLEAN tree
   after the loop → arbiter does NOT run (`status == ""`) → UNAFFECTED.
3. **baseSHA**: `strings.Repeat("0", 40)` when `isUnborn`, else `preRunHEAD`.
4. **isRoot for DiffTree per entry**: `isUnborn && i == 0` (first entry on an unborn repo is concept 0 root).
5. **Message = ""** in rebuilt CommitResult (not printed — only SHA/Subject/Files via printDecomposeCommit).
6. **Best-effort on reread error**: `deps.Verbose.VerboseRawOutput(...)` (the only free-form Verbose
   method; nil-safe — `*ui.Verbose` methods no-op when nil/off; dcmDeps sets Verbose: nil). Keep loop
   `commits`, do NOT fail (commits already published).

## Verbose API (internal/ui/verbose.go)

- `*ui.Verbose` methods: `VerboseCommand`, `VerboseRawOutput`, `VerboseRetry` — ALL no-op when
  `v==nil || v.w==nil || !v.on`. Deps.Verbose is `*ui.Verbose` (nullable).
- No dedicated "warn" method → use `VerboseRawOutput` for the best-effort diagnostic.

## Testing approach for tip amend / mid-chain (the hard part)

The arbiter must return a DYNAMIC SHA (the tip / a mid-chain SHA) but the loop creates SHAs at runtime.
The **stub binary** (`cmd/stubagent`) only emits fixed responses (`STAGECOACH_STUB_OUT`/`SCRIPT`) — it
cannot read git. KEY FACTS:
- **Agent subprocess cwd is the USER's CWD, NOT the repo** (executor.go:25: "cmd.Dir is NOT set").
- BUT the **arbiter prompt is on STDIN** (PromptDelivery "stdin"), and the payload lists each commit's
  SHA on its own 40-hex line (format `SHA\nSubject\nfiles...\n\n`, per prompt/arbiter.go BuildArbiterUserPayload).

→ Solution: a **shell-script arbiter** that reads its stdin prompt, extracts the Nth 40-hex SHA, and
emits `{"target": "<sha>"}`. Override `stubtest.Manifest(...).Command` to point at the script.
- Tip (last SHA): `sed -n 's/^\([0-9a-f]\{40\}\)$/\1/p' | tail -n 1`
- Mid-chain concept[1] of 3 (2nd SHA): `sed -n 's/^\([0-9a-f]\{40\}\)$/\1/p' | sed -n '2p'`
- Bare 40-hex lines appear ONLY in the commit-list section (diff lines are +/-/space-prefixed) → robust.
- `runArbiter` validates `targetInRun` — the extracted SHA IS in the list → passes. `resolveTipAmend` /
  `resolveMidChain` REUSE original messages (no extra message-agent call needed).

## Helpers available / needed in tests (decompose_test.go, dcm*-prefixed)

- Existing: `dcmInitRepo`, `dcmWriteFile`, `dcmRunGit`/`dcmGitOut`, `dcmHeadSHA`, `dcmLogOneline`,
  `dcmLogCount`, `dcmStatusPorcelain`, `dcmPlannerManifest`, `dcmArbiterManifest`,
  `dcmMessageScriptManifest`, `dcmDeps`, `dcmStagerSeam`, `stubtest.Build`, `stubtest.Manifest`,
  `tooledStubManifest`, `piShape`.
- Need to ADD: `dcmShaResolves(t, repo, sha)` — `git -C <repo> rev-parse --verify <sha>^{commit}`
  exit 0 ⇒ resolvable (not dangling); and a `dcmScriptArbiter(t, mode)` helper that writes the
  shell script, chmod 0755, and returns a `provider.Manifest` with `.Command` overridden.

## Chain resolution facts (chain.go — UNCHANGED by this task)

- `resolveTipAmend`: CommitTree(newTipTree, [tipParent], tipMsg) → NEW tip SHA (reuses msg, no message-agent call).
- `resolveMidChain(i)`: rebuilds [i..N-1] with new SHAs; [0..i-1] unchanged.
- `resolveNewCommit` (null): CommitTree(leftoverTree, [tipSHA], newMsg) → (N+1)-th commit; CALLS
  generateMessage (needs a message-script entry).
- `computeAmended(target, chainData)`: nil→0; tip(idx N-1)→1; mid(idx i)→N-i.

## default_action.go — NO change

`runDecompose` (L270) iterates `res.Commits` via `printDecomposeCommit` (L308) which prints
`[<short-sha>] <subject>` + per-file lines. Accurate post-arbiter data flows through unchanged.
