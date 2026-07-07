# P1.M1.T2.S4 — CommitTree Validation & Design Notes

> Empirically verified on **git 2.54.0** + **go1.26.4** (linux/amd64), this box.
> This file is the source of truth for every line of the `CommitTree` implementation and its tests.

---

## 1. The stdin gap: why `run()` (S1) is insufficient — and the `runWithInput` decision

### The problem

S1's `run()` helper is the only shell-out point, but it was designed for stdout/stderr-only git
calls (rev-parse, write-tree, diff, log). Its body **never sets `cmd.Stdin`**:

```go
cmd := exec.CommandContext(ctx, gitPath, full...)
var out, errb bytes.Buffer
cmd.Stdout = &out
cmd.Stderr = &errb
// ← cmd.Stdin is nil → child's stdin is /dev/null
runErr := cmd.Run()
```

`CommitTree` MUST deliver the commit message via stdin (`-F -`, FINDING 4) so the message is
bulletproof against special characters, leading dashes, quotes, and newlines. With `-F -`, git
reads the message from the child's stdin pipe until EOF. A `nil` `cmd.Stdin` means git reads an
empty message → either an empty-commit failure or a misleading success with a blank message.

### Why we do NOT modify `run()`

- S2 and S3 (both landed) explicitly forbid modifying `run()`: their PRPs state
  "run() is NOT re-implemented or modified (delegated to, unchanged)" and rely on its exact
  signature `run(ctx, repo, args...) (stdout, stderr, exitCode, err)`.
- Changing `run()`'s signature (e.g. adding an `io.Reader` param) would force edits to
  `RevParseHEAD` and `WriteTree` callers — out of scope and a merge hazard.
- S3 (WriteTree) is being implemented in parallel and also edits `git.go`; keeping `run()`
  byte-identical eliminates any textual overlap with S3's WriteTree-body edit.

### The decision: add a NEW co-located helper `runWithInput`

Add an unexported method on `*gitRunner`:

```go
func (g *gitRunner) runWithInput(ctx context.Context, repo string, stdin io.Reader, args ...string) (stdout string, stderr string, exitCode int, err error)
```

It is **structurally identical to `run()`** plus one line: `cmd.Stdin = stdin`. It preserves:

- `exec.LookPath("git")` resolution (→ "git binary not found" on miss; `code == -1`).
- `-C repo` arg prefixing (NOT `cmd.Dir` / `os.Chdir` — §19, goroutine-safe).
- Separate stdout/stderr `bytes.Buffer`s.
- `ctx.Err()`-before-`errors.As(*exec.ExitError)` ordering → `err == nil` for non-zero exits.
- `[]string` args, **no shell** (§19).

This keeps both shell-out points in the SAME file with the SAME structure. The ~20 lines of
near-duplication is a deliberate, documented trade-off for not touching `run()` during parallel
landing. (A future refactor could extract a shared `runCore(ctx, repo, stdin, args...)` that both
`run()` and `runWithInput()` delegate to, but that would rewrite `run()`'s body — S2/S3's
territory — and is explicitly OUT OF SCOPE for S4.)

**Import delta:** `runWithInput`'s signature references `io.Reader` → add `"io"` to git.go's
imports. `CommitTree` itself uses only `strings.NewReader`, `strings.TrimSpace`, `fmt.Errorf`
(all already imported). So the single import change is `+ "io"`.

---

## 2. The `parents []string` ↔ `parentSHA` reconciliation (CONTRACT vs INTERFACE)

### The discrepancy

- **Work-item contract** (item description): "Implement `CommitTree(ctx, treeSHA, parentSHA,
  message) (newSHA, err)`. ... If parentSHA != "" (not unborn): append `-p`, parentSHA."
- **The `Git` interface** (S1, ALREADY LANDED and consumed by the orchestrator):
  ```go
  CommitTree(ctx context.Context, tree string, parents []string, msg string) (sha string, err error)
  ```

The interface is **authoritative** — it is already compiled into the package and stubbed on
`*gitRunner`. The contract's singular `parentSHA` is a simplified description of the v1 use case
(the orchestrator passes either `nil` for a root commit or `[]string{parentSHA}` for a single
parent). The interface's `[]string` is deliberately future-proof for v2 multi-commit decomposition
(octopus/merge commits: `-p A -p B`).

### The mapping (what the implementation does)

```go
args := []string{"commit-tree", tree}
for _, p := range parents {
    args = append(args, "-p", p)   // repeatable; git commit-tree supports multiple -p
}
args = append(args, "-F", "-")      // message via stdin ALWAYS (root or child)
```

- **Root commit** (`parents == nil` OR `len(parents) == 0`): the loop adds nothing → no `-p` →
  git creates a root commit. This is the "parentSHA == ''" / "isUnborn" case from the contract.
- **Child commit** (`len(parents) >= 1`): each parent gets a `-p` flag.
- **No special "is this the first commit?" branching in CommitTree** — `parents` being empty IS
  the root signal. (The orchestrator, P1.M3.T4, decides `parents` from `RevParseHEAD`'s
  `isUnborn`: `isUnborn ⇒ parents=nil`; else `parents=[]string{sha}`. CommitTree does not call
  RevParseHEAD itself — it trusts its caller.)

> **Implementing agent:** do NOT add a separate `isUnborn`/`parentSHA` parameter or change the
> interface. Implement exactly the `parents []string` signature that is already on `*gitRunner`.

---

## 3. Empirical verification (git 2.54.0, this box)

Script run in a throwaway temp repo (`git init -q`, no global config overrides). Full transcript
captured below; key results:

```
$ echo "hello" > a.txt && git add a.txt && TREE=$(git write-tree); echo "TREE=$TREE"
TREE=2e81171448eb9f2ee3821e3d447aa6b2fe3ddba1   # 40-hex, exit 0

# ROOT commit via commit-tree -F - (NO -p), multi-line message with leading-dash body
$ MSG=$'feat: root commit\n\nbody line 1\n--weird--leading dashes'
$ printf '%s' "$MSG" | git commit-tree "$TREE" -F -; echo "EXIT=$?"
0c4c498dc8e250d13ce33f9588c8445be75a0dc2         # 40-hex, EXIT=0
$ git cat-file -p <ROOT>
tree 2e81171448eb9f2ee3821e3d447aa6b2fe3ddba1
author ... <...> 1782760783 -0400
committer ... <...> 1782760783 -0400
                                                  # ← NO "parent" line ⇒ root commit confirmed

feat: root commit
                                                  # ← blank line (paragraph separator)
body line 1
--weird--leading dashes                           # ← leading-dash body preserved verbatim

# CHILD commit via commit-tree -p ROOT -F -
$ CHILD=$(printf '%s' "$MSG" | git commit-tree "$TREE" -p "$ROOT" -F -)
$ git cat-file -p "$CHILD" | head -5
tree 2e81171448eb9f2ee3821e3d447aa6b2fe3ddba1
parent 0c4c498dc8e250d13ce33f9588c8445be75a0dc2   # ← parent present ⇒ child confirmed

# Message that STARTS with flag-like text — catastrophic with -m, safe with -F -
$ DASHMSG=$'-n -p --foo'
$ printf '%s' "$DASHMSG" | git commit-tree "$TREE" -F -   # exit 0, no misparse
$ git log --format=%B --no-walk <that-sha>
-n -p --foo                                        # ← exact bytes preserved

# Bad tree → non-zero (error branch)
$ printf 'x' | git commit-tree 0000...0 -F - 2>&1; echo "EXIT=$?"
fatal: 0000000000000000000000000000000000000000 is not a valid object
EXIT=128
```

### Findings pinned

| Scenario | git exit | stdout | stderr contains | What it proves |
|---|---|---|---|---|
| Root commit (no `-p`) | 0 | 40-hex NEW_SHA | (empty) | `parents==nil` → root commit; `cat-file -p` has no `parent` line |
| Child commit (`-p ROOT`) | 0 | 40-hex NEW_SHA | (empty) | parent linked; `cat-file -p` shows `parent <ROOT>` |
| `-F -` stdin, special chars | 0 | 40-hex | (empty) | newlines, `--`, leading-dash-first messages all preserved byte-for-byte |
| Bad/nonexistent tree | 128 | (empty) | `fatal: <sha> is not a valid object` | error branch: `code != 0` |
| Message roundtrip via `git log --format=%B` | — | `msg + "\n"` | — | `-F -` trims a single trailing newline from input; `%B` re-appends one → compare with `TrimSpace` |

**Critical `-F -` vs `-m` demonstration:** a message beginning with `-n -p --foo` is stored
verbatim with `-F -`. With `-m`, git would try to parse `-n`, `-p`, `--foo` as flags and fail or
silently produce a garbage commit. This is exactly why FINDING 4 mandates `-F -`.

---

## 4. Identity handling: production inherits env; tests set repo-local config

### Production (the `CommitTree` method)

`runWithInput` sets `cmd.Env = nil` (inherits the parent process environment), exactly like `run()`.
Stagecoach commits **AS the configured user** — git resolves `user.name`/`user.email` from the
user's global/repo git config and any `GIT_AUTHOR_*`/`GIT_COMMITTER_*` env vars they exported.
CommitTree does NOT inject identity. (The architecture reference §2's `cmd.Env = append(...,
"GIT_AUTHOR_NAME="+...)` is an illustrative standalone example, NOT Stagecoach's behavior.)

### Tests (the fixtures)

Temp repos created by `initRepo` (S1) have **no** `user.name`/`user.email` configured, and the
test process env does not carry identity globally. `git commit-tree` would fail with
`fatal: empty ident name/address not allowed`. Resolution: a fixture `setIdentityConfig(t, dir)`
runs `git -C dir config user.name Test` and `git -C dir config user.email test@example.com`,
writing to `.git/config`. `commit-tree` then resolves identity from repo config (it uses the same
identity resolution as `git commit`). This:

- mirrors production reality (the user has identity configured),
- avoids polluting the test process env (`t.Setenv` is process-global within a package run),
- is robust in CI where global git config is absent.

(`S2`'s `makeEmptyCommit` instead sets identity via the `git commit` command's own `cmd.Env` —
that works for that one `commit` invocation but is NOT visible to the later `commit-tree` call
CommitTree makes, because the env was scoped to makeEmptyCommit's command. Hence the repo-config
approach here.)

### Identity NOT needed for the failure-path tests

`TestCommitTree_GitBinaryMissing` (LookPath fails first) and `TestCommitTree_ContextCancelled`
(pre-cancelled ctx) never reach the point where git needs identity, so they skip `setIdentityConfig`.

---

## 5. Test design — assertion matrix

All in `package git` (white-box, to call `New()` and reach `*gitRunner`; mirrors S1/S2/S3).
Reuse `initRepo` (git_test.go) and `makeEmptyCommit` (revparse_test.go) — both in scope (same
package). **Distinct** new helper names (no collision with S3's `makeMergeConflict` closure-locals
`writeFile`/`runGit`, which are not package-level):

| Helper | Purpose |
|---|---|
| `setIdentityConfig(t, dir)` | `git config user.name/user.email` in repo (§4) |
| `writeFile(t, dir, name, body)` | `os.WriteFile(filepath.Join(dir,name), body, 0o644)` |
| `stageFile(t, dir, name)` | `git -C dir add <name>` (default env) |
| `writeTreeOf(t, dir)` | `git -C dir write-tree` → trimmed 40-hex TREE_SHA |
| `headSHA(t, dir)` | `git -C dir rev-parse HEAD` → trimmed parent SHA (child test) |
| `commitMessage(t, dir, sha)` | `git -C dir log --format=%B -n 1 <sha>` → trimmed message (roundtrip) |

| Test | Fixture | Key assertions | What it proves |
|---|---|---|---|
| `TestCommitTree_RootCommit` | initRepo + setIdentityConfig + stage+writeTree | `err==nil`; SHA matches `^[0-9a-f]{40,64}$`; `cat-file -p`/log shows **no parent** | Root commit (parents==nil ⇒ no `-p`) |
| `TestCommitTree_ChildCommit` | initRepo + setIdentityConfig + makeEmptyCommit + stage+writeTree + headSHA | `err==nil`; SHA matches hex; commit's parent == headSHA | Child commit (`parents=[p]` ⇒ `-p p`) |
| `TestCommitTree_MessageViaStdin` | initRepo + setIdentityConfig + stage+writeTree; msg = `feat: x\n\nbody\n--dash\n"quotes"` | `err==nil`; `commitMessage(sha)` == `strings.TrimSpace(msg)` | **The core `-F -` guarantee**: special chars/leading-dash preserved byte-for-byte |
| `TestCommitTree_BadTree` | initRepo + setIdentityConfig; tree=`000...0` | `err != nil`; err contains `"git commit-tree: failed"` (code != 0); SHA == `""` | Error branch: invalid tree → exit 128 |
| `TestCommitTree_GitBinaryMissing` | `t.Setenv("PATH","")` | `err != nil` contains "git binary not found"; SHA == `""` | `runWithInput`'s err path propagated (not misread as commit success) |
| `TestCommitTree_ContextCancelled` | pre-cancel ctx | `errors.Is(err, context.Canceled)`; SHA == `""` | ctx.Err() surfaced (not exit 0) |

The message-roundtrip assertion compares `strings.TrimSpace(retrieved)` to the input message,
because `git log --format=%B` appends one trailing `\n` as a record separator (verified §3). The
test message is crafted WITHOUT a trailing newline so `-F -`'s "trim single trailing newline" rule
is a no-op and the stored message equals the input exactly.

---

## 6. The `TestStubsPanic` edit (required consequence — mirrors S2/S3)

`git_test.go`'s `TestStubsPanic` (after S2+S3 landed) has 9 `assertPanics` lines, the FIRST being:

```go
assertPanics(t, "CommitTree", func() { _, _ = g.CommitTree(ctx, "tree", nil, "msg") })
```

Once `CommitTree` is real (no panic), `assertPanics` fails with "expected panic, but did not
panic". Resolution (identical pattern to S2's RevParseHEAD removal and S3's WriteTree removal):
**DELETE that single line.** After removal, `TestStubsPanic` covers the remaining 8 stubs
(UpdateRefCAS, DiffTree, StagedDiff, HasStagedChanges, RecentMessages, RecentSubjects,
CommitCount, AddAll). This is the ONLY edit to `git_test.go`. Document it in the commit message.

**Parallel-execution note:** S3 (WriteTree) removed the `WriteTree` line; S4 removes the
`CommitTree` line. These are DISTINCT lines in the static list — no textual overlap. If both land
near-simultaneously, each edit targets a different `assertPanics(...)` line. If `git_test.go` does
not contain the `CommitTree` line at edit time, S4's edit has already been applied (or the line
was removed by an earlier pass) — re-read the file and proceed.

---

## 7. Decisions log

- **D1 — `runWithInput`, not modifying `run()`:** Add a new stdin-capable helper rather than
  altering `run()`'s signature/body. Rationale: S2/S3 forbid touching `run()`; S3 is landing
  concurrently; modifying `run()` would cascade edits to RevParseHEAD/WriteTree callers.
  Trade-off: ~20 lines of near-duplicate exec boilerplate, isolated to git.go, documented.
- **D2 — `-F -` always, never `-m`:** FINDING 4 mandates stdin delivery. Verified (§3) that
  `-F -` preserves leading-dash messages that `-m` would misparse as flags. No `-m` path exists.
- **D3 — `parents []string`, not a singular `parentSHA`:** The interface is authoritative (landed).
  The contract's singular `parentSHA` maps to `[]string{parentSHA}`; root = empty slice/nil. No
  interface change, no extra `isUnborn` parameter.
- **D4 — branch on `code != 0` for the error path:** Mirrors WriteTree (S3). commit-tree's only
  non-zero exit is a bad tree/parent (128 on git 2.x). Treat any non-zero as a commit-object
  failure; name "git commit-tree: failed" and include the trimmed stderr. Do NOT pin on 128
  specifically (future-proof; the contract says "on success return trimmed stdout").
- **D5 — err checked BEFORE code:** Inherited invariant. `runWithInput` returns `err != nil` with
  `code == -1` for LookPath/context/start failures; `err == nil` for real git exits (0, 128).
  So `if err != nil { return "", err }` is the authoritative infrastructural guard. A missing git
  binary is NEVER misread as a commit failure (guarded by `TestCommitTree_GitBinaryMissing`).
- **D6 — identity via repo config in tests, NOT production env injection:** Production inherits the
  parent env (commits AS the user). Tests write `user.name`/`user.email` to `.git/config` so
  `commit-tree` resolves identity without env pollution or global-config assumptions.
- **D7 — no SHA validation in production:** On exit 0, return `strings.TrimSpace(stdout)`. Do not
  add hex/length checks (downstream `update-ref` will reject a bad SHA). The `^[0-9a-f]{40,64}$`
  regex is TEST-ONLY.
- **D8 — `io` is the only new import:** `runWithInput` needs `io.Reader`. CommitTree uses only
  already-imported symbols (`strings`, `fmt`). Adding `io` to git.go's import block (sorted) is
  the single import delta.
