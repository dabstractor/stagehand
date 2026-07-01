# Research Notes — P1.M3.T1.S1 (LogEntry type + LogRange method on Git interface)

Scope: add a `LogEntry{SHA, Subject}` type and a `LogRange(ctx, baseSHA) ([]LogEntry, error)` method
to the `Git` interface + `gitRunner`, returning commits in `baseSHA..HEAD` oldest-first. This is the
foundational capability S2 (`rereadFinalCommits`) consumes to fix Issue 3 (post-arbiter stale SHAs).

## ⚠️ CRITICAL CORRECTION to the contract / architecture doc — the all-zeros range is INVALID

The contract (item d) and `architecture/issue3_post_arbiter_output.md` claim `git log
<all-zeros>..HEAD` "lists ALL commits reachable from HEAD (git treats the zeros ref as
non-existent)." **Empirically FALSE.** Verified against the real git binary (git 2.x) in a 3-commit
repo (A→B→C):

| form | result |
|------|--------|
| `git log --reverse … 0000…0000..HEAD` (range) | **exit 128** `fatal: Invalid revision range 0000…0000..HEAD` |
| `git rev-list 0000…0000..HEAD` | **exit 128** `fatal: Invalid revision range …` |
| `git log --reverse … ^0000…0000 HEAD` (exclude) | **exit 128** `fatal: bad object 0000…0000` |
| `git log --reverse … 0000…0000` (plain arg) | **exit 128** `fatal: bad object 0000…0000` |
| **`git log --reverse … HEAD`** (NO range) | **exit 0** → A, B, C oldest-first ✅ |

→ The all-zeros SHA is **not a valid `git log` range/exclude/revision arg in any form.** The ONLY way
to get "all commits reachable from HEAD" is the **no-range form** `git log --reverse … HEAD`. So
`LogRange` MUST detect the all-zeros sentinel and branch to the no-range form. (On a TRULY-unborn repo
— no commits — even the no-range `HEAD` exits 128 `ambiguous argument 'HEAD'` → return (nil, nil),
matching the 128-as-non-error convention.)

## 1. Verified normal-range behavior

3-commit repo A→B→C (A = root):
- `git log --reverse --format=%H%x1f%s <A>..HEAD` → **B, C** (oldest-first). ✅ (excludes the base A —
  `A..HEAD` = reachable from HEAD but not from A.)
- `baseSHA == HEAD` (`HEAD..HEAD`) → exit 0, empty stdout → (nil, nil). ✅
- `%x1f` (ASCII Unit Separator) delimiter survives subjects with spaces/quotes/special chars; `awk
  -F'\x1f'` cleanly splits SHA from subject. Subjects are single-line (`%s`). ✅

## 2. Patterns to follow (existing git.go log/rev methods)

- `run()` seam (git.go:204): `stdout, stderr, code, err := g.run(ctx, g.workDir, args...)`. On a
  non-zero git exit, `err` is nil and `code` carries the real exit (gotcha G2).
- **128-as-non-error convention**: `RevParseHEAD` (L283), `RecentMessages` (L646), `RecentSubjects`
  (L698), `CommitCount` (L735) ALL do `if code == 128 { return <zero>, nil }` for the unborn signal.
  LogRange follows this for the truly-unborn case.
- **NUL-delimited precedent**: `RecentMessages` uses `--format=%x00%B` + split on `\x00`. LogRange's
  `%H%x1f%s` + split on `\x1f` is the direct analogue (SHA:subject instead of message records).
- **Defensive guards**: `RecentMessages`/`RecentSubjects` guard bad `n`. LogRange guards an empty
  stdout and unparseable lines.
- Read-only doc footer: "read-only with respect to refs/index (PRD §18.1)".
- Go 1.22 → `strings.Cut(s, "\x1f")` is available (cleanest SHA/subject split; ok=false if no delim).

## 3. The implementation (verified-correct)

```go
// LogEntry is one commit in a log range (oldest-first when produced by LogRange).
type LogEntry struct {
	SHA     string // full 40/64-hex commit SHA
	Subject string // first line of the commit message (%s — single-line by construction)
}

func (g *gitRunner) LogRange(ctx context.Context, baseSHA string) ([]LogEntry, error) {
	args := []string{"log", "--reverse", "--format=%H%x1f%s"}
	if baseSHA == strings.Repeat("0", 40) {
		// Originally-unborn repo: the all-zeros ref is NOT a valid `git log` range base (git rejects
		// `<zeros>..HEAD` as "Invalid revision range"). List ALL commits reachable from HEAD instead —
		// on an originally-unborn repo these are exactly the commits created this run.
		args = append(args, "HEAD")
	} else {
		args = append(args, baseSHA+"..HEAD")
	}

	stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return nil, err
	}
	if code == 128 {
		return nil, nil // truly-unborn repo (HEAD has no commits) — 128-as-non-error (RevParseHEAD/RecentSubjects/CommitCount)
	}
	if code != 0 {
		return nil, fmt.Errorf("git log: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}

	var entries []LogEntry
	for _, line := range strings.Split(stdout, "\n") {
		if line == "" {
			continue
		}
		sha, subject, ok := strings.Cut(line, "\x1f")
		if !ok {
			continue // defensive: skip a line lacking the delimiter
		}
		entries = append(entries, LogEntry{SHA: sha, Subject: subject})
	}
	return entries, nil
}
```
- `err`, `fmt`, `strings` already imported (git.go:3-12). No new imports.
- The all-zeros sentinel (`strings.Repeat("0", 40)`) matches the codebase's existing "unborn/root"
  convention (UpdateRefCAS callers pass all-zeros as expectedOld for a root commit).

## 4. Placement

- **LogEntry type**: near `FileChange` (git.go:11-19) — the analogous "one entry in a listing" struct.
- **LogRange interface method**: append as the LAST method in the `Git` interface, after
  `WorkingTreeDiff` (whose doc ends ~L174; the interface closes at L178/180). Mode-A JSDoc rides here.
- **gitRunner.LogRange impl**: place with the log-family methods, right after `CommitCount` (~L751-756).

## 5. Interface extension is backward-compatible

`Git` is implemented ONLY by `gitRunner` (no mocks — all tests use real git). Adding a method forces
gitRunner to implement it (which S1 does); there are no other implementors to break. Verified: `grep
'Git{'` / the only `New() Git` returns `*gitRunner` (git.go:190). All existing `git.Git` consumers
continue to compile unchanged.

## 6. Tests — internal/git/logrange_test.go (new file; per-method test-file convention)

Helpers (existing): `initRepo(t, repo)` (git_test.go:12), `makeEmptyCommit(t, repo, msg)` (used across
the git test files, e.g. recentsubjects_test.go:30). Cases:
1. **3-commit range**: make A→B→C; `LogRange(A)` → `[B, C]`, oldest-first, correct SHAs + subjects.
2. **Empty range**: `LogRange(HEAD-SHA)` → nil (base==HEAD).
3. **All-zeros (unborn) base**: `LogRange(strings.Repeat("0",40))` → `[A, B, C]` (all, oldest-first).
4. **Truly-unborn repo**: `initRepo` only (no commits) → `LogRange(all-zeros)` → (nil, nil).
5. (optional) special-chars subject round-trips verbatim.
Get SHAs in-test via `git -C <repo> rev-parse HEAD~2` (or a `gitOut`/`run` helper) — mirror the
recentsubjects_test.go assertion style.

## 7. Docs (Mode A) — rides with the work

JSDoc on `LogEntry` + the `LogRange` interface method (range semantics, oldest-first, %x1f delimiter,
all-zeros special case, 128-as-non-error). No other doc files reference LogRange (it's brand new), so
the interface doc IS the documentation. P1.M6.T1.S2 (final docs sweep) may reference it later.
