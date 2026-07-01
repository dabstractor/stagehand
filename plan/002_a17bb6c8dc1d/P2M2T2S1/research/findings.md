# P2.M2.T2.S1 — StatusPorcelain: Research Findings

Empirical ground truth + codebase pattern analysis for the `StatusPorcelain` method.
All git behaviors below were VERIFIED by running real git commands against temp repos.

---

## §1. The contract (verbatim from the work item)

> Add `StatusPorcelain(ctx context.Context) (string, error)` to the `Git` interface and `gitRunner`.
> Implementation: run `git status --porcelain`, return stdout trimmed. Non-zero exit → error.
> The caller (decompose orchestrator) checks `strings.TrimSpace(output) != ""` to decide whether
> to run the arbiter.
> OUTPUT: StatusPorcelain returns the porcelain output string. Empty string means clean tree.
> Read-only w.r.t. refs/index.

**Critical simplification:** the caller ONLY checks emptiness. StatusPorcelain does NOT parse paths,
does NOT split lines, does NOT handle rename notation, does NOT use `-z`. It is a pure
emptiness/clean-tree signal for the arbiter trigger (PRD §13.6.5: "if `git status --porcelain` is
non-empty (some changes were not claimed by any stager), the arbiter runs"). This makes it the
SIMPLEST method in the Git interface.

---

## §2. Verified `git status --porcelain` behaviors (the empirical ground truth)

Run against `/tmp/sp_probe` (a real temp repo), all commands invoked exactly as `git status --porcelain`:

| State                              | stdout                  | exit code |
| ---------------------------------- | ----------------------- | --------- |
| Clean UNBORN repo (no commits)     | `""`                    | **0**     |
| Unborn repo + untracked file       | `?? a.txt\n`            | **0**     |
| Unborn repo + staged file          | `A  a.txt\n`            | **0**     |
| Born repo, committed, clean tree   | `""`                    | **0**     |
| Born repo, modified (not staged)   | ` M a.txt\n`            | **0**     |
| Born repo, staged new file         | `A  b.txt\n`            | **0**     |
| NON-repo directory                 | (stderr: fatal msg)     | **128**   |

### KEY conclusions (these drive the implementation):

1. **Exit code 0 on EVERY success path — clean AND dirty, born AND unborn.** There is NO "128 =
   unborn" concept for `git status --porcelain`. It works perfectly on unborn repos (lists untracked
   files as `??`). This is UNLIKE RevParseHEAD/RevParseTree/RecentMessages/CommitCount, where
   exit-128 IS the unborn signal. StatusPorcelain must NOT copy their `if code == 128 { return "", nil }`
   branch — that would silently swallow a non-repo/corrupt error as a false "clean".

2. **Exit code 128 ⟺ real error (non-repo, corrupt repo, bad config).** 128 is the ONLY non-zero
   exit in practice. Therefore the implementation uses the SIMPLE branch (`if code != 0 → error`),
   identical to `StagedFileCount` and `ReadTree`. A 128 here is a real caller error, NOT a benign
   unborn signal.

3. **Porcelain format = `XY <path>`** — a 2-char status code (X = index, Y = working tree), a
   single space, then the path. Untracked = `??`, staged-add = `A `, modified-not-staged = ` M`.
   One line per changed path, with a trailing `\n`. `strings.TrimSpace(stdout)` removes the trailing
   newline AND collapses to the canonical form. The caller compares the result to `""`.

4. **Read-only w.r.t. refs AND the index.** `git status --porcelain` inspects state; it mutates
   nothing (no `.git/index` write, no ref update). Safe under the PRD §18.1 invariant.

---

## §3. The exact method to COPY (StagedFileCount / ReadTree — the simple-branch read-only pattern)

StatusPorcelain is a near-mechanical copy of `StagedFileCount` (read-only, stdout → parse) — even
simpler, because there is NO parsing (return the trimmed string as-is).

### `(*gitRunner).StagedFileCount` (internal/git/git.go:756) — the template:
```go
func (g *gitRunner) StagedFileCount(ctx context.Context) (int, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "diff", "--cached", "--name-only")
	if err != nil {
		return 0, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		return 0, fmt.Errorf("git diff --cached --name-only: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	count := 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, nil
}
```

### StatusPorcelain (the port — swap command + drop the count loop):
```go
func (g *gitRunner) StatusPorcelain(ctx context.Context) (string, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "status", "--porcelain")
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (run sets code=-1) — UNWRAPPED
	}
	if code != 0 {
		return "", fmt.Errorf("git status --porcelain: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}
```

### The diff vs the template (the WHOLE implementation):
- Command: `"diff", "--cached", "--name-only"` → `"status", "--porcelain"`.
- Return type: `(int, error)` → `(string, error)`.
- Body: DROP the `count` loop (no parsing) → `return strings.TrimSpace(stdout), nil`.
- Error prefix: `"git diff --cached --name-only: failed"` → `"git status --porcelain: failed"`.
- Error branch: IDENTICAL (`if err != nil { return "", err }` then `if code != 0 { ... fmt.Errorf }`).

That is the entire implementation. Three statements after the `run` call.

---

## §4. run()'s INVARIANT (consumed — do NOT modify)

`func (g *gitRunner) run(ctx, repo, args ...string) (stdout, stderr string, exitCode int, err error)`
(internal/git/git.go:140):
- A **non-zero git exit** is returned as `(stdout, stderr, exitCode, nil)` — `err` is nil. Git uses
  exit codes as semantic signals; callers inspect `exitCode`.
- `err != nil` ⟺ **infrastructural failure ONLY**: LookPath miss (`"git binary not found"`),
  context cancel (`ctx.Err()`), or start/I/O failure. In all three `exitCode == -1`.
- Therefore StatusPorcelain's two branches are EXACTLY right:
  1. `if err != nil { return "", err }` — catches context.Canceled + git-missing, propagated
     **UNWRAPPED** so `errors.Is(err, context.Canceled)` works at the call site.
  2. `if code != 0 { return "", fmt.Errorf(...) }` — catches the 128 non-repo/corrupt error.
- `ctx.Err()` check inside run() means a cancelled context returns the bare `context.Canceled`
  (UNWRAPPED) — verified by `TestRun_CapturesExitCodeAndSeparateBuffers` and every sibling's
  `_ContextCancelled` test.

---

## §5. Reusable test helpers (all already package-level in `internal/git/*_test.go`)

DO NOT redefine any of these (duplicate-symbol compile error):

| Helper            | Defined in               | Purpose                                            |
| ----------------- | ------------------------ | -------------------------------------------------- |
| `initRepo(t,dir)` | git_test.go              | `git init` + repo-local user.name/user.email       |
| `writeFile(t,d,n,body)` | committree_test.go | create a file (0644)                               |
| `stageFile(t,dir,name)` | committree_test.go | `git add <name>`                                   |
| `makeEmptyCommit(t,dir,msg)` | revparse_test.go | `git commit --allow-empty -m` with test identity |
| `execGit(t,dir,args...)` | revparsetree_test.go | independent-oracle git command (trimmed stdout) |

StatusPorcelain tests use these EXACTLY as the sibling tests do (`hasstaged_test.go`,
`stagedcount_test.go`, `readtree_test.go`). No new helpers needed.

### Test idiom (mirror `readtree_test.go` / `stagedcount_test.go`):
```go
func TestStatusPorcelain_<Scenario>(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	// ... set up repo state via writeFile/stageFile/makeEmptyCommit ...
	g := New(repo)
	out, err := g.StatusPorcelain(context.Background())
	// assert err + out via strings.Contains / errors.Is
}
```

### Independent-oracle assertions:
StatusPorcelain's OUTPUT is verified by `strings.Contains(out, "<expected porcelain line>")`, NOT by
re-calling StatusPorcelain. The repo STATE is built via real git (writeFile/stageFile/makeEmptyCommit),
so the test is not circular.

---

## §6. The unborn-repo special case (the KEY test)

`TestStatusPorcelain_CleanUnbornRepo` is the single most important test: on a repo with NO commits
and NO files, `StatusPorcelain` must return `("", nil)` — NOT an error. This proves StatusPorcelain
does NOT carry RevParseHEAD's "128 = unborn" convention (because `git status --porcelain` exits 0 on
an unborn repo). If a future implementer wrongly copies RevParseTree's `if code == 128 { return "", nil }`,
this test still passes (clean unborn is exit 0, not 128) — but `TestStatusPorcelain_NotARepo` (exit
128 → real error) is the test that CATCHES that mistake. The pair together pins the convention.

---

## §7. Scope boundaries (frozen / owned elsewhere — do NOT edit)

- **`run()` / `runWithInput()`** — CONSUMED, never modified.
- **`TreeDiff` (P2.M2.T1.S2, parallel)** — appends to the SAME interface block + SAME file. StatusPorcelain
  appends AFTER TreeDiff (currently the last interface method at git.go:133 + last body at git.go:822).
  Appending at the END of both minimizes merge friction (both are independent additive lines). Keep BOTH
  additions on any 3-way merge conflict at the closing interface brace.
- **`WorkingTreeDiff` (P2.M2.T2.S2, sibling)** — do NOT implement. It is a separate, larger work item
  (unstaged diff with binary filtering). StatusPorcelain is the small emptiness-signal sibling.
- **Decompose wiring (P3.x)** — no caller references StatusPorcelain yet. This task only adds + tests the method.
- **go.mod / go.sum** — UNCHANGED (stdlib only: `context`/`fmt`/`strings` already imported in git.go).
- **`// Method ownership` comment block** (git.go ~line 30) — a v1 provenance map; do NOT edit.
- **`StagedDiffOptions`, `EmptyTreeSHA`, `defaultExcludes`** — UNCHANGED (StatusPorcelain takes no opts).

---

## §8. Architecture-doc spec (authoritative)

From `plan/002_a17bb6c8dc1d/architecture/binary_git_v2.md` §"StatusPorcelain":
```go
// StatusPorcelain returns `git status --porcelain` output.
// Used to detect leftovers after the decompose loop (arbiter trigger).
// Non-empty output → arbiter runs. Empty → perfect run.
StatusPorcelain(ctx context.Context) (string, error)
```
From `plan/002_a17bb6c8dc1d/architecture/decompose_architecture.md`:
- line 27: "arbiter (only if status --porcelain non-empty)"
- line 84-85: "StatusPorcelain returns `git status --porcelain` output (arbiter trigger)."

Both match the work-item contract and §2's empirical behaviors exactly. The signature is
`(string, error)` — NO options struct (it is a pure emptiness signal, no caps/excludes/binary filtering).

---

## §9. Validation gates (verified-present in this repo)

- `go build ./...` (module: `github.com/dustin/stagehand`, go 1.22)
- `go vet ./...`
- `golangci-lint run ./...` (config: `.golangci.yml`; enabled: errcheck, gosimple, govet, ineffassign, staticcheck, unused)
- `go test -race ./internal/git/ -run "TestStatusPorcelain" -v` (new tests)
- `go test -race ./internal/git/` (whole package regression)
- `go test ./...` (full module regression — the new interface method is additive, no caller breaks)
- `gofmt -l internal/ pkg/` (empty output = formatted)
- `git diff --exit-code go.mod go.sum` (empty = deps unchanged)

All commands are project-specific and executable as written.
