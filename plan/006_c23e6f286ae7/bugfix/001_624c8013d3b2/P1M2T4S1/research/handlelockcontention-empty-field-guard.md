# P1.M2.T4.S1 (Issue 4b) — Research: guard handleLockContention empty fields + partial-read test

> Defense-in-depth counterpart to Issue 4a (P1.M2.T3.S1, the holder-side never-empty rewrite in
> `internal/lock/lock.go`). This subtask (4b) is the CONTENDER-SIDE guard in `internal/cmd/default_action.go`.
> The two subtasks touch DIFFERENT files → no merge conflict. 4a narrows the empty-file window to ~zero; 4b
> guarantees that even if a contender still reads a partial/empty lock file (the residual race window), the
> Busy message renders sensibly instead of `"on  (pid  on )"`.

## 1. The exact current code (internal/cmd/default_action.go, handleLockContention)

```go
func handleLockContention(stderr io.Writer, heldErr *lock.HeldError, g git.Git, ctx context.Context) error {
	// No-op fast path (§18.5): holder published snapshot= and the contender's index matches it.
	if snap := heldErr.Contents.Snapshot; snap != "" {
		contenderTree, werr := g.WriteTree(ctx) // index-read-only + one harmless dangling tree (G4)
		if werr == nil && contenderTree == snap {
			fmt.Fprintln(stderr, "nothing to do — an in-progress run already covers your staged changes.")
			return exitcode.New(exitcode.Success, nil) // exit 0, SILENT
		}
		// werr != nil (e.g. merge conflicts) or SHAs differ → fall through to Busy (G5).
	}
	fmt.Fprintf(stderr,
		"stagecoach: another stagecoach run is already in progress on %s (pid %s on %s). "+
			"Your newly-staged changes will remain staged — re-run stagecoach after it finishes. Lock: %s.\n",
		heldErr.Contents.Repo, heldErr.Contents.Pid, heldErr.Contents.Hostname, heldErr.Path)
	return exitcode.New(exitcode.Busy, nil) // exit 5, SILENT
}
```

The 4 substitutions in the Busy `fmt.Fprintf`:
1. `%s` ← `heldErr.Contents.Repo` (the "on %s")
2. `%s` ← `heldErr.Contents.Pid` (the "pid %s")
3. `%s` ← `heldErr.Contents.Hostname` (the second "on %s")
4. `%s` ← `heldErr.Path` (the "Lock: %s") — ALWAYS non-empty (it is the lock file path from `lockPath`)

## 2. The bug (why empty fields render gibberish)

When a contender loses the flock race and its `os.ReadFile(path)` (in `lock.Acquire`'s EWOULDBLOCK branch)
lands in the empty/partial-file window (Issue 4a's target), `parseContents` yields ALL-EMPTY fields. With
empty Repo/Pid/Hostname the Busy line becomes:

```
stagecoach: another stagecoach run is already in progress on  (pid  on ). Your newly-staged ...
```

Note the DOUBLE SPACES: `on  (` (space+empty+space+paren) and `pid  on` (space+empty+space). Ugly and
uninformative. Functionally conservative (empty `snapshot=` → no-op fast path skipped → Busy is the safe
outcome), but the diagnostic is broken.

## 3. The guard (the exact edit — compute fallbacks BEFORE the fmt.Fprintf)

Insert a fallback block between the fast-path `if` and the Busy `fmt.Fprintf`. Substitute:

| field        | empty fallback    |
|--------------|-------------------|
| `Contents.Repo`     | `"an unknown repo"` |
| `Contents.Pid`      | `"<unknown>"`       |
| `Contents.Hostname` | `"<unknown>"`       |
| `Contents.Path`     | (unchanged — always non-empty) |

```go
	// Issue 4b guard: a contender may read a partial/empty lock file (the residual race window from
	// Issue 4a's SetSnapshot rewrite) yielding empty Repo/Pid/Hostname diagnostics. Substitute sensible
	// fallbacks so the Busy message never renders as "on  (pid  on )". Path is always non-empty (it is
	// the lock file path from lockPath), so it is passed through unchanged.
	repo := heldErr.Contents.Repo
	if repo == "" {
		repo = "an unknown repo"
	}
	pid := heldErr.Contents.Pid
	if pid == "" {
		pid = "<unknown>"
	}
	hostname := heldErr.Contents.Hostname
	if hostname == "" {
		hostname = "<unknown>"
	}
	fmt.Fprintf(stderr,
		"stagecoach: another stagecoach run is already in progress on %s (pid %s on %s). "+
			"Your newly-staged changes will remain staged — re-run stagecoach after it finishes. Lock: %s.\n",
		repo, pid, hostname, heldErr.Path)
	return exitcode.New(exitcode.Busy, nil) // exit 5, SILENT
```

The guarded message with all-empty input reads:
`"stagecoach: another stagecoach run is already in progress on an unknown repo (pid <unknown> on <unknown>). Your newly-staged changes will remain staged — re-run stagecoach after it finishes. Lock: /x.lock."` — sensible, no double spaces.

## 4. What MUST NOT change (the scope fences)

- **Exit code**: still `exitcode.New(exitcode.Busy, nil)` → exit 5, SILENT (`err.Error() == ""`). The guard
  is message-only.
- **No-op fast path**: the `if snap := heldErr.Contents.Snapshot; snap != "" { … }` block is UNTOUCHED.
- **Message tone**: same wording; only the 3 diagnostic substitutions gain fallback values. Path unchanged.
- **`internal/lock/lock.go`**: NOT touched (4a / P1.M2.T3.S1 owns it, running in parallel). This subtask
  edits ONLY `internal/cmd/default_action.go` + `internal/cmd/lock_contention_test.go`.
- **Public API / signatures**: `handleLockContention`'s signature is unchanged.

## 5. The test (add to internal/cmd/lock_contention_test.go)

Name follows the existing `TestHandleLockContention_Busy_<Scenario>` convention →
`TestHandleLockContention_Busy_EmptyDiagnostics`. Table-driven with TWO rows (both reach the Busy branch
with empty diagnostics, covering the two entry paths):

| row                   | Snapshot     | contender WriteTree | how it reaches Busy                          |
|-----------------------|--------------|---------------------|----------------------------------------------|
| `empty_snapshot`      | `""`         | (any)               | fast path SKIPPED (snapshot empty)           |
| `nonmatching_snapshot`| `"abc123"`   | `"zzz999"`          | fast path FAILS (trees differ) → fall through|

Both rows set Repo/Pid/Hostname = "" (the partial-read reproduction). Assertions per row:
1. `exitcode.For(err) == exitcode.Busy` (5).
2. `err.Error() == ""` (SILENT — same as every other Busy test).
3. `strings.Contains(msg, "an unknown repo")` (the repo fallback present).
4. `strings.Contains(msg, "<unknown>")` (the pid/hostname fallback present).
5. `!strings.Contains(msg, "on  (")` (the contract's exact broken-pattern check — no double-space `on  (`).
6. `!strings.Contains(msg, "  ")` (robust: NO double space anywhere — no legit double space exists in the
   message, so this is a stronger guard against the broken rendering).

The test uses the EXISTING `contentionFakeGit` test double (same package) — for the `empty_snapshot` row
the fake's WriteTree is never called (fast path skipped), so a zero-value `&contentionFakeGit{}` suffices;
for the `nonmatching_snapshot` row set `writeTreeSHA: "zzz999"`. NO new imports needed (`bytes`, `context`,
`strings`, `exitcode`, `lock` are all already imported in lock_contention_test.go).

## 6. Why both 4a AND 4b (defense-in-depth, not redundant)

4a (P1.M2.T3.S1) makes the holder's rewrite never produce an empty file (Seek→Write→Truncate→Sync on the
held fd) → narrows the empty-window to ~zero. 4b (this subtask) guards the contender's MESSAGE rendering so
that even the residual/nondeterministic window (or a genuinely malformed/partial file from any cause) cannot
produce gibberish. Both ship: 4a fixes the root cause; 4b is the belt-and-suspenders that makes the
diagnostic robust regardless of file state. The issue_analysis.md and the work-item contract both call for
both. Neither subtask depends on the other at the code level (different files) — they land independently and
compose.
