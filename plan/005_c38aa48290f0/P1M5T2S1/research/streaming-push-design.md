# Streaming `git push` — design notes (P1.M5.T2.S1)

## 1. The push point lives in the CLI layer, NOT the orchestrator

`git push` is **not** a plumbing primitive (it moves a remote ref over the network, mutates remote
state, and is interactive by nature). It is therefore a **post-run CLI convenience**, exactly like
the FR42 success report. Both commit paths in `internal/cmd/default_action.go` end their happy path
by printing the report then `return nil` (success → exit 0):

- **single path** — `runDefault`: after `printCommitReport(stdout, res, changes)` (the `// Commit
  path: FR42 report` block), `return nil`.
- **decompose path** — `runDecompose`: the loop prints each landed commit via
  `printDecomposeCommit`, then `if derr != nil { return handleDecomposeError(derr) }; return nil`.

The `--push` step is inserted at BOTH success returns: after the report prints, before `return nil`.
A helper `runPush(ctx, stderr, g, cfg)` returns an error that maps to exit 1 on failure (FR-P2) — the
commits already stand; only the exit code + the closing note change.

## 2. Skip conditions are ALREADY satisfied by the existing control flow (FR-P3)

FR-P3 lists three skip cases. ALL THREE already leave `runDefault`/`runDecompose` BEFORE the success
return, so a push call placed at the success return is naturally unreachable:

| FR-P3 skip case | Where `default_action.go` returns | Reaches push? |
| --- | --- | --- |
| `--dry-run` | `runDefault` early-returns `nil` after `printDryRunMessage` (before the commit path) | NO |
| zero commits (exit 2) | `exitcode.New(NothingToCommit, ...)` in the auto-stage state machine / clean-tree check | NO (err) |
| rescue / CAS abort | `handleGenError` / `handleDecomposeError` → `return exitcode.New(...)` (err) | NO (err) |

So the **only** explicit guard the push needs is `if cfg.Push` (default false). Placing it at the two
success returns makes the FR-P3 skip conditions structurally un-reachable. Belt-and-suspenders: also
short-circuit on `flagDryRun` (the dry-run path returns before the commit path, but the explicit
guard is the documented contract).

## 3. The streaming `Push` method is NET-NEW on the git runner

`internal/git/git.go` has exactly two exec helpers, both **capturing** (`run`, `runWithInput` — they
buffer stdout/stderr into `bytes.Buffer` and return the strings). Neither can stream verbatim to the
user's terminal. A streaming push needs stdout/stderr wired directly to `os.Stdout`/`os.Stderr`:

```go
func (g *gitRunner) Push(ctx context.Context, stdout, stderr io.Writer) error
```

It runs `git push` with **no arguments** (plain `git push`, per FR-P1) targeting the repo via `-C`
(the same goroutine-safe convention as every other method), wires `cmd.Stdout`/`cmd.Stderr` to the
passed writers (the CLI passes `os.Stdout`/`os.Stderr` → verbatim streaming), and on non-zero exit
returns a wrapped error carrying git's exit code. **No `--set-upstream` is ever added** (FR-P2).
Context cancellation (timeout/signal) propagates via `ctx.Err()` (mirrors `run`'s invariant).

## 4. The no-upstream test contract (external_deps.md §8, VERIFIED)

`git push` on a branch with no upstream exits **128** with stderr containing `has no upstream
branch` and `--set-upstream`. **Caveat (critical):** the developer's real global git config sets
`push.autoSetupRemote=true`, which makes the push **silently succeed** — masking the FR-P2 failure
path entirely. Tests MUST run git with:

```
GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
```

and assert on **stable substrings** (`has no upstream branch`, `--set-upstream`), never full stderr
text (git wording varies across versions). The `Push` method itself is config-agnostic (it just runs
`git push` and streams output); the env isolation belongs in the **test** that sets up the bare
remote + the no-upstream branch.

## 5. Exit-code mapping (FR-P2: push failure = exit 1, commits stand)

`exitcode.For()` already maps a generic non-nil error → `exitcode.Error` (1) as the default tail.
Push failure needs NO new sentinel and NO new mapping: a plain `fmt.Errorf("git push failed: %w",
err)` returned from `runDefault`/`runDecompose` flows to `main` → `exitcode.For` → exit 1. The
**closing note** ("commits created; push failed") is printed to **stderr** by `runPush` BEFORE it
returns the error (so the note always lands even though main's generic path would otherwise print
only the wrapped error). The commits are already published — push failure does not roll them back.

## 6. The clean-push E2E scenario (a temp bare remote)

The happy-path E2E (`stagecoach --push` against a configured upstream) needs a throwaway remote:

```
bare=$(mktemp -d); git init --bare "$bare"
# in the working repo: set the upstream once (test setup, NOT stagecoach's job)
git remote add origin "$bare"; git push -u origin HEAD
# ... stagecoach creates commits, then `--push` runs plain `git push` → succeeds, exit 0
```

The no-upstream scenario deliberately does NOT run the `-u` setup → `git push` fails 128 → stagecoach
prints "commits created; push failed" + the verbatim git stderr → exit 1. The skip-on-dry-run
scenario runs `stagecoach --dry-run --push` → the dry-run early-return fires before the push site → no
push, exit 0.

## 7. Interaction with P1.M5.T1.S1 (`--edit`)

`--edit` and `--push` are independent post-run conveniences on the same success path. `--edit` gates
EACH commit's message (pre-publish, inside the orchestrator); `--push` runs ONCE after the ENTIRE
run publishes (post-publish, in the CLI). They compose: `stagecoach --edit --push` edits each message,
publishes, then pushes. Neither touches the other's code. The P1.M5.T1.S1 PRP does NOT add a push
site; this PRP adds it. Both are `cfg.<Flag>`-gated no-ops when off → byte-identity when both unset.

## 8. Config surface (full 5-layer precedence, NOT flag-only)

Unlike `--edit` (flag-only, FR-E1), `--push` gets the **full precedence stack** (FR-P1: `--push` /
`STAGECOACH_PUSH` / `stagecoach.push` / `[generation].push`, default false). This mirrors `Template`
(P1.M2.T2.S2), NOT `Context`. So:

- `Config.Push bool \`toml:"push"\`` with `toml:"push"` (a config-file key, under `[generation]`).
- `Defaults()` → `Push: false`.
- `loadEnv`: `STAGECOACH_PUSH` (presence-semantic, bool — mirrors `STAGECOACH_VERBOSE`).
- `loadGitConfig`: `stagecoach.push` (the existing `stagecoach.*` reader).
- `loadFlags`: `--push` (`fs.Changed("push")` → DIRECT set).
- `root.go`: `pf.BoolVar(&flagPush, "push", false, "...")`.
