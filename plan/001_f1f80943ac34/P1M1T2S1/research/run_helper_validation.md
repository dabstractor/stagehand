# Research: `run()` Helper & Git Interface Contract Validation

> **Purpose:** Empirically verified semantics for the `internal/git/gitRunner.run()` low-level helper
> (P1.M1.T2.S1) and the exact method signatures the `Git` interface must expose for subsequent
> subtasks S2–S6 and T3.S1–S5. Every exit code, every stdout/stderr shape, and every Go
> `os/exec` behavior below was confirmed by execution on the exact installed toolchain
> (git 2.54.0, go1.26.4-X:nodwarf5 linux/amd64) on 2026-06-29.

---

## 1. Environment

| Component | Version / Path | Verified via |
|---|---|---|
| git binary | `git version 2.54.0` at `/usr/bin/git` | `git --version`, `which git` |
| Go toolchain | `go1.26.4-X:nodwarf5 linux/amd64` | `go version` |
| Module | `github.com/dustin/stagecoach`, `go 1.22` | existing `go.mod` (landed by S1) |
| `internal/git/` dir | EXISTS, EMPTY (created by S1's §14 tree) | `ls -la internal/git/` |
| Only existing `*.go` | `./cmd/stagecoach/main.go` (stub) | `find . -name '*.go'` |

**Implication:** This subtask creates `internal/git/git.go` (and its `_test.go`) as the FIRST Go
files in `internal/git/`. No merge conflicts with other packages. The module compiles today.

---

## 2. The `run()` Helper — Core Design (CONTRACT-FAITHFUL)

The work-item contract specifies this exact helper signature:

```
run(ctx, repo, args...) (stdout string, stderr string, exitCode int, err error)
```

### 2.1 The four contract responsibilities and their resolution

| Contract phrase | Resolution | Source of truth |
|---|---|---|
| "resolves the git binary via `exec.LookPath("git")`" | Call `exec.LookPath("git")` **inside `run()`** (NOT in the constructor — `New(workDir) Git` has no error return). On failure return `("", "", -1, err)`. | contract §3 |
| "sets cmd.Dir via `-C <repo>`" | Pass `"-C", repo` as the FIRST two args to git. This is git's own working-dir mechanism. Do **NOT** set Go's `cmd.Dir` field and do **NOT** `os.Chdir`. (See §2.3 for why the phrase means "set the working dir via the -C flag", not "set Go's cmd.Dir field".) | architecture `git_plumbing_summary.md` "Always use `git -C <repo>` (not `os.Chdir`)" — every code example in `git_plumbing_reference.md` uses `exec.CommandContext(ctx, "git", "-C", repo, …)`. |
| "captures stdout+stderr to separate `bytes.Buffers`" | `cmd.Stdout = &out`, `cmd.Stderr = &errb` (two distinct `bytes.Buffer`). | contract §3 |
| "returns the exit code from `*exec.ExitError`" | On non-zero exit, `errors.As(runErr, &exitErr)` → `exitCode = exitErr.ExitCode()`, and **`err = nil`**. Only infrastructural failures (LookPath miss, context cancel, start I/O) return a non-nil `err` with `exitCode = -1`. | contract §3 + empirical §3 below |

### 2.2 The non-obvious invariant: non-zero exit is NOT a Go error

This is the **central design decision** and the thing a naive implementation gets wrong. Git uses
exit codes as **semantic signals**, not just success/failure:

| Git exit code | Meaning | Correct handling |
|---|---|---|
| 0 | success | `exitCode=0, err=nil` |
| 1 | semantic signal (e.g. `diff --cached --quiet` = "has staged changes"; `update-ref` CAS mismatch) | `exitCode=1, err=nil` ← caller inspects `exitCode` |
| 128 | semantic signal (unborn repo for `rev-parse HEAD`; not-a-valid-SHA) | `exitCode=128, err=nil` ← caller inspects `exitCode` |
| −1 (sentinel) | infrastructural failure: binary not found, context cancelled, start failure | `exitCode=-1, err=<the error>` |

**If `run()` returned `err != nil` for exit 1/128, every caller would have to re-`errors.As` to
recover the exit code — defeating the purpose of returning `exitCode` separately.** The
four-tuple return exists precisely so semantic exit codes flow through `exitCode` and only "real"
problems flow through `err`.

### 2.3 "`-C repo` args" vs "Go's `cmd.Dir` field" — why we use args

The contract phrase *"sets cmd.Dir via `-C <repo>`"* is loose phrasing for *"sets the working
directory via the `-C` flag"*. Evidence this means the flag, not Go's field:

- `git_plumbing_summary.md` (Cross-Platform Notes): *"Always use `git -C <repo>` (not `os.Chdir`)
  for goroutine safety."* Setting Go's `cmd.Dir` is the exec equivalent of `os.Chdir` for the
  child and is **not** what the patterns show.
- **Every** code example in `git_plumbing_reference.md` §§ 1–7 is
  `exec.CommandContext(ctx, "git", "-C", repo, …)` — zero examples set `cmd.Dir`.

**Decision:** `fullArgs := append([]string{"-C", repo}, args…)` then `exec.CommandContext(ctx, gitPath, fullArgs…)`.
Do NOT set `cmd.Dir`. This is goroutine-safe (the parent process CWD is never touched) and matches
all architecture examples.

---

## 3. Empirically Verified Git Behaviors (the method contract)

Run in throwaway repos (`/tmp/shtest`, `/tmp/shtest2`). These pin the exact exit codes and
stdout/stderr shapes each interface method must implement in its owning subtask.

### 3.1 `rev-parse HEAD` — the unborn-repo trap (→ S2 `RevParseHEAD`)

```
$ git rev-parse HEAD; echo "EXIT=$?"    # on a repo with ZERO commits
fatal: ambiguous argument 'HEAD': unknown revision or path not in working tree.
HEAD                                      ← stdout is the LITERAL STRING "HEAD\n" (NOT empty!)
EXIT=128
```

- **stdout = `"HEAD\n"`** (non-empty) — a naive `if stdout == "" { unborn }` check is WRONG and is
  a documented latent bug in the original `commit-pi`. **Detect unborn via exit code 128, not
  stdout emptiness.** (See `critical_findings.md` FINDING 1.)
- Correct `run()` behavior on the unborn case: `stdout="HEAD\n", stderr=<fatal msg>,
  exitCode=128, err=nil`. The S2 method inspects `exitCode == 128` → returns `isUnborn=true`.

### 3.2 `diff --cached --quiet` — inverted exit codes (→ T3.S2 `HasStagedChanges`)

```
nothing staged → EXIT=0      (index == HEAD)
something staged → EXIT=1
real error → EXIT > 1
```

- Exit **1 = "has staged changes"** — it is NOT an error. `run()` returns `exitCode=1, err=nil`;
  T3.S2 reads `exitCode == 1` → `true`. (`critical_findings.md` FINDING 6.)

### 3.3 `write-tree` (→ S3 `WriteTree`)

```
clean index          → prints 40-char tree SHA, EXIT=0
unmerged index       → EXIT=128, stderr "error: cannot write a tree with unresolved merge conflicts"
```

- On a conflict, `run()` returns `exitCode=128, err=nil`; S3 surfaces a "resolve conflicts" error.
- Empty-tree SHA `4b825dc642cb6eb9a060e54bf8d69288fbee4904` is a valid `write-tree` output
  (confirmed on an empty unborn repo) — do NOT treat it as an error.

### 3.4 `commit-tree <tree> -F -` — root vs non-root (→ S4 `CommitTree`)

```
root commit    : printf 'msg' | git commit-tree <tree> -F -           # NO -p, EXIT=0, prints SHA
non-root commit: printf 'msg' | git commit-tree <tree> -p <parent> -F -  # EXIT=0, prints SHA
```

- **`-F -` reads the message from stdin** — bulletproof against quotes, leading dashes, newlines.
  S4 sets `cmd.Stdin = strings.NewReader(msg)`. (`critical_findings.md` FINDING 4.)
- Root commit = omit `-p`. `parents` arg controls this: `len(parents)==0` → no `-p`.
- `commit-tree` does **NOT** move any ref (confirmed: after `commit-tree`, `rev-parse HEAD` still
  returns 128 until `update-ref` runs). The commit is dangling until CAS publish.

### 3.5 `update-ref <ref> <new> <expected-old>` — CAS (→ S5 `UpdateRefCAS`)

```
correct CAS  : git update-ref HEAD <new> <actual-current>     → EXIT=0
wrong old    : git update-ref HEAD <new> <different-value>    → EXIT≠0 (stderr "cannot lock ref")
```

- 3-arg form = compare-and-swap. **Never use the 2-arg (force) form.** (`critical_findings.md`
  FINDING 3 — CAS-failure messages vary by version; signal = exit code ≠ 0, not a substring.)
- Root commit CAS: `expectedOld = all-zeros hash` (`0000…0`, 40 zeros for sha-1).

### 3.6 `diff-tree --no-commit-id --name-status -r [--root] <sha>` (→ S6 `DiffTree`)

```
git diff-tree --no-commit-id --name-status -r --root <root-commit-sha>
A	path/to/file          ← tab-separated: <status>\t<path>
R100\told\tnew            ← renames: <status>\t<src>\t<dst> (3 fields)
```

- Root commit requires `--root` (diff against empty tree); without it, a root commit prints nothing.
- Statuses: A/M/D/R/C/T/U. R/C carry a similarity score (`R90`, `C75`).
- Verified output on a root commit with one added file: `A\tf.txt\n`, EXIT=0.

### 3.7 Log queries (→ T3.S3 `CommitCount`, `RecentMessages`; T3.S4 `RecentSubjects`)

```
git rev-list --count HEAD        → "0"/"1"/integer, EXIT=0   (fails 128 on unborn)
git log --format=%x00%B -50      → NUL-delimited full messages (split on \x00)  [VERIFIED hexdump: starts 0x00 'initial']
git log --format=%s -50          → one subject per line, EXIT=0
```

- **NUL-delimited** (`%x00%B`) is the robust choice — avoids the `---` markdown-collision latent
  bug in `commit-pi`. (`critical_findings.md` FINDING 9.)
- All three fail with exit 128 + "your current branch … does not have any commits yet" on an unborn
  repo; callers must short-circuit on the `isUnborn` flag from `RevParseHEAD` (S2) before calling these.

---

## 4. Go `os/exec` Semantics — Verified by Execution

A throwaway Go program modeled the exact `run()` logic and confirmed:

| Case | Observed `run()` output |
|---|---|
| `run(ctx, repo, "rev-parse", "HEAD")` on a repo WITH a commit | `stdout="<sha>\n", stderr="", exitCode=0, err=nil` |
| `run(ctx, repo, "rev-parse", "HEAD-nonexistent-x")` | `exitCode=128, err=nil` (non-zero exit is captured in `exitCode`, **NOT** a Go error), stdout non-empty, stderr non-empty |

**This is the proof that `errors.As(runErr, &exitErr)` + returning `exitErr.ExitCode()` with
`err=nil` is correct.** A naive `if err := cmd.Run(); err != nil { return err }` would have
returned a Go error for exit 128, forcing every caller to re-parse — exactly what the four-tuple
return is designed to avoid.

### 4.1 The minimal correct `run()` body (verified-equivalent to the throwaway program)

```go
func (g *gitRunner) run(ctx context.Context, repo string, args ...string) (string, string, int, error) {
    gitPath, err := exec.LookPath("git")
    if err != nil {
        return "", "", -1, fmt.Errorf("git binary not found in PATH: %w", err)
    }
    full := append([]string{"-C", repo}, args...)
    cmd := exec.CommandContext(ctx, gitPath, full...)
    var out, errb bytes.Buffer
    cmd.Stdout, cmd.Stderr = &out, &errb
    runErr := cmd.Run()
    if runErr == nil {
        return out.String(), errb.String(), 0, nil
    }
    if ctx.Err() != nil { // context cancelled (timeout/signal) — NOT a git exit code
        return out.String(), errb.String(), -1, ctx.Err()
    }
    var exitErr *exec.ExitError
    if errors.As(runErr, &exitErr) {
        return out.String(), errb.String(), exitErr.ExitCode(), nil // semantic exit code, err=nil
    }
    return out.String(), errb.String(), -1, runErr // start/I/O failure
}
```

---

## 5. Interface Method Signatures (the contract for S2–T3.S5)

Derived from the architecture docs + the exit-code table above. Signatures chosen to be
context-aware (all take `context.Context`), to encode the trap-detection directly in the return
tuple where it matters, and to use the option-struct pattern for the one method with many knobs.

| Method | Signature | Owning subtask |
|---|---|---|
| `RevParseHEAD` | `(ctx) (sha string, isUnborn bool, err error)` | S2 |
| `WriteTree` | `(ctx) (sha string, err error)` | S3 |
| `CommitTree` | `(ctx, tree string, parents []string, msg string) (sha string, err error)` | S4 |
| `UpdateRefCAS` | `(ctx, ref, newSHA, expectedOld string) error` | S5 |
| `DiffTree` | `(ctx, sha string, isRoot bool) ([]FileChange, error)` | S6 |
| `StagedDiff` | `(ctx, opts StagedDiffOptions) (string, error)` | T3.S1 |
| `HasStagedChanges` | `(ctx) (bool, error)` | T3.S2 |
| `RecentMessages` | `(ctx, n int) ([]string, error)` | T3.S3 |
| `RecentSubjects` | `(ctx, n int) ([]string, error)` | T3.S4 |
| `CommitCount` | `(ctx) (int, error)` | T3.S3 |
| `AddAll` | `(ctx) error` | T3.S5 |

### 5.1 Auxiliary types defined in THIS subtask (required by the signatures above)

```go
// FileChange is one entry in a diff-tree "what landed" listing (S6 parses it).
type FileChange struct {
    Status  string // A, M, D, R, C, T, U (R/C carry a similarity score, e.g. "R100")
    SrcPath string // non-empty only for R/C (the rename/copy source)
    Path    string // the destination path (always set)
}

// StagedDiffOptions configures staged-diff capture (T3.S1 consumes it).
type StagedDiffOptions struct {
    MaxDiffBytes int      // byte cap on the non-markdown section (commit-pi: 300000); 0 = unlimited
    MaxMDLines   int      // per-file line cap for markdown (commit-pi: 100); 0 = unlimited
    Excludes     []string // pathspec excludes for the non-markdown section, e.g. []string{":!*.lock", ":!vendor/*"}
}
```

These types are defined now because the interface cannot be written without them; their FIELDS are
forward-looking but match commit-pi's documented behavior so T3.S1 does not need to amend them.

---

## 6. Why the struct must stub all 11 methods in THIS subtask

The contract requires `New(workDir string) Git` to return the `Git` interface. For `*gitRunner`
to satisfy `Git`, **Go requires every interface method to exist on the struct at compile time**.
Therefore `git.go` must include all 11 methods. Since S2–T3.S5 implement the bodies, this subtask
ships them as **stubs that panic with a message naming the implementing subtask**:

```go
func (g *gitRunner) RevParseHEAD(ctx context.Context) (string, bool, error) {
    panic("gitRunner.RevParseHEAD: not yet implemented — see P1.M1.T2.S2")
}
```

**Why panic (not `return errors.New`):** a silent error invites the orchestrator (P1.M3.T4) to
proceed with a zero-value result and produce confusing downstream failures. A panic fails
immediately at the first use of an unimplemented method — the strongest possible "not ready" signal.
The validation for THIS subtask only exercises `run()` (which IS implemented) and the panic
behavior, so stubs do not block the green test.

**The one method that is NOT stubbed is `run()`** — it is fully implemented now and is the entire
runtime deliverable of this subtask. `run()` is unexported (a "helper"), so it is NOT part of the
`Git` interface; it is exercised by the internal test `package git`.

---

## 7. Test Strategy (what the implementing agent must write)

`internal/git/git_test.go` in **`package git`** (internal test — needs access to `gitRunner` and
`run`). Cases (all verified against real git 2.54.0 above):

1. `TestNew` — `New(dir)` returns non-nil `Git`; type-asserts to `*gitRunner`; `workDir` field set.
2. `TestRun_HappyPath` — `init` a temp repo; `run(ctx, repo, "rev-parse", "--git-dir")` →
   `exitCode==0, err==nil, stdout=="%!(EXTRA ...)"` trimmed non-empty, `stderr==""`.
3. `TestRun_CapturesExitCodeAndStderrSeparately` — on an UNBORN temp repo,
   `run(ctx, repo, "rev-parse", "HEAD")` → `exitCode==128, err==nil` (proves exit code is NOT a Go
   error), `stdout=="HEAD\n"` (the trap string), `stderr` non-empty (proves stdout/stderr are
   separate buffers — if combined, stdout would contain the fatal message).
4. `TestRun_LookPathError` — `t.Setenv("PATH", "")` (Go 1.17+) makes `git` unfindable →
   `exitCode==-1, err!=nil` and err mentions "git binary not found". (Robust way to test the
   LookPath branch without uninstalling git.)
5. `TestStubsPanic` — for each stubbed method, `assertPanics(t, func(){ _ , _ , _ = g.RevParseHEAD(ctx) })`
   and check the panic message contains "not yet implemented".

`t.TempDir()` provides a per-test scratch dir; a small `initRepo(t, dir)` helper runs `git init`
via `exec.Command` directly (NOT via the not-yet-implemented methods). Tests run under `make test`
(`go test -race ./...`); the `-race` flag is compatible with `exec`-based tests.

---

## 8. Decisions Log (resolutions of ambiguous contract points)

| # | Ambiguity | Resolution | Rationale |
|---|---|---|---|
| D1 | "sets cmd.Dir via `-C <repo>`" — field or flag? | Use the `-C repo` **flag** (args); do NOT set Go's `cmd.Dir`. | All architecture examples use the flag; `cmd.Dir`/`os.Chdir` is explicitly discouraged for goroutine safety. |
| D2 | Where does `LookPath` run — `New()` or `run()`? | In `run()`. | `New(workDir) Git` has no error return; resolving in `run()` lets a missing binary surface as `err` cleanly. |
| D3 | Does non-zero exit return a Go `err`? | **No** — exit code flows through `exitCode`, `err=nil`. Only LookPath/context/start failures set `err` (with `exitCode=-1`). | Git uses exit codes as semantic signals (1=staged, 128=unborn); callers must read `exitCode`. Empirically verified. |
| D4 | Is `run()` exported or in the interface? | Unexported `run`; NOT in the `Git` interface. | Contract calls it a "helper"; it's the private engine the bound methods delegate to. |
| D5 | How do stubs satisfy the interface? | All 11 methods exist as panic-stubs now; S2–T3.S5 replace each. | Go requires full interface satisfaction for `New() Git` to compile. |
| D6 | Are `FileChange`/`StagedDiffOptions` defined now? | Yes — the interface signatures require them. | T3/S6 consume them; defining now avoids an interface-breaking amendment later. |
