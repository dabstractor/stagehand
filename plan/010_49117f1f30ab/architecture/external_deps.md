# External Dependencies — git commit-hook semantics

Source: researcher subagent brief (git docs + git source `builtin/commit.c`/`run-command.c`).
Authoritative references: `githooks(5)`, `git-commit(1)`, `git-config(1)`.

## 1. Hook ordering + invocation arguments (plain `git commit`, no amend/merge/squash)

```
pre-commit  →  prepare-commit-msg  →  [editor]  →  commit-msg  →  commit created  →  post-commit
```

`pre-commit`, `prepare-commit-msg`, `commit-msg` run **before** the commit object exists (a non-zero
exit aborts). `post-commit` runs **after** the commit is final.

| Hook | Args | Notes |
|---|---|---|
| `pre-commit` | **0 args** | |
| `prepare-commit-msg` | `<msgfile> [<source> [<sha>]]` | source ∈ {message, template, merge, squash, commit} |
| `commit-msg` | **1 arg: `<msgfile>`** | validates the final message |
| `post-commit` | **0 args** | |

## 2. ⚠️ Plain-commit `prepare-commit-msg` source — ABSENT, not `""` (researcher finding)

The PRD §9.25 FR-V2 specifies `prepare-commit-msg <msg-file> ""` (empty source). **The researcher
flags this as likely a shell-`$2` fallacy.** `githooks(5)` says the hook takes "one to three
parameters," and git's NULL-terminated argv mechanics indicate that for a PLAIN commit (no
`-m`/`-t`/merge/squash/`-c`/`-C`/`--amend`), git passes a **single argument (the message file only)**
— the `source` parameter is absent, not the empty string `""`.

`$2` evaluates to empty whether the 2nd arg is `""` OR never passed, so observing "empty `$2`" does
NOT prove a 2-arg call.

**Resolution for the implementing agent:** run the 5-second test (create a `prepare-commit-msg` that
logs `$#`, `git commit --allow-empty`, check `argc=1` vs `argc=2`). Emit the matching argv. If
`argc=1`, pass the file only; if `argc=2`, pass `""`. Either is correct git parity; the difference
only matters to `$#`-checking hooks (rare). **Record the verified form in the runner source.**

## 3. Environment git sets for commit hooks

- **`GIT_INDEX_FILE` — YES, set** for `pre-commit`, `prepare-commit-msg`, `commit-msg`. This is the
  **canonical** mechanism git itself uses (`builtin/commit.c` `run_commit_hook` pushes
  `GIT_INDEX_FILE=<index_file>`). It is exactly what stagecoach wants to mirror for the scoped
  pre-commit (§5/§7). Source: git source `builtin/commit.c`.
- **`GIT_EDITOR`** — git sets `GIT_EDITOR=:` (no-op) for the commit hooks so a hook doesn't
  spuriously launch an editor. A faithful emulation should set this too.
- **`GIT_DIR`** — the hooks run in the repo's git-dir context; reliable guarantee is repo discovery
  works from CWD. Setting it explicitly is harmless.
- **CWD = worktree root** (non-bare repo). `githooks(5)`: "Before Git invokes a hook, it changes its
  working directory to … the root of the working tree in a non-bare repository."
- **stdin** — passed through (the TTY for interactive commits). Most pre-commit hooks don't read
  stdin; attaching `/dev/null` is safe and avoids hangs (FR-V6 specifies stdin `/dev/null`).

## 4. Hook discovery + executability

- **Discovery**: `core.hooksPath` overrides; otherwise `$GIT_DIR/hooks/<name>`.
- **⚠️ `git rev-parse --git-path hooks` gotcha**: the researcher cites that it returns `$GIT_DIR/hooks`
  and does NOT reflect `core.hooksPath`. **BUT** the scout verified on git 2.54.0 that
  `HooksPath()` (which uses `git rev-parse --git-path hooks`) DOES honor `core.hooksPath` on modern
  git (2.31+ changed this). **Decision: trust the scout's git-2.54.0-verified finding** — the
  existing `HooksPath()` is correct for modern git. Note the historical gotcha in a comment. If a
  user reports a `core.hooksPath` misresolution on old git, that's an upstream git-version issue.
- **Non-executable hook → silently skipped** (git checks `access(path, X_OK)`). FR-V1 specifies this.

## 5. `--no-verify` exact behavior (CONFIRMED)

`git commit --no-verify` bypasses **only** `pre-commit` and `commit-msg`. It does **NOT** skip
`prepare-commit-msg` or `post-commit`. Exact `git-commit(1)` wording: "This option bypasses the
pre-commit and commit-msg hooks."

**Stagecoach `--no-verify` MUST mirror this exactly** (FR-V5): skip pre-commit + commit-msg;
prepare-commit-msg and post-commit still run. Assert this in tests.

## 6. `prepare-commit-msg` + message file lifecycle

The hook **edits `<msgfile>` in place**; git reads it back as the final message and applies the
cleanup mode (default `strip`: removes leading/trailing blank lines, trailing whitespace, collapses
consecutive blanks, and **strips `#`-comment lines**). The comment char honors `core.commentChar`.

**For stagecoach**: write the generated+deduped+`--edit`-finalized message to a temp file, run
`prepare-commit-msg`, read it back, strip `#`-comment lines (honor `core.commentChar`), use the
result as the commit message for `commit-tree -m` (actually `-F -` via stdin). Then `commit-msg`
runs over the same file; read back again as the final.

## 7. `post-commit` exit code (CONFIRMED disregarded)

`post-commit` runs **after** the commit is final (object written, ref advanced). Its exit code does
**NOT** affect commit success — the commit already landed. `githooks(5)`: "it cannot affect the
outcome of `git commit`." A non-zero exit is logged as a warning (FR-V7: best-effort, via
`deps.Verbose`), never undoes a landed commit.

## 8. ⚠️ The central design tension: pre-commit hooks are working-tree-coupled

**This is the most important finding and is inherent to the design.** `GIT_INDEX_FILE=<tmp>` is the
correct mechanism (it is what `git commit` itself does), but real-world pre-commit hooks are
fundamentally **working-tree-coupled**:

- **husky** — a dispatcher; downstream commands do/don't respect `GIT_INDEX_FILE`.
- **lint-staged** — discovers staged files via `git diff --cached --name-only` (respects
  `GIT_INDEX_FILE`), re-stages via `git add` (respects it), BUT runs formatters on **working-tree
  files**, not index content.
- **pre-commit.com** — runs hooks on a copy of staged content; `git stash`-style save/restore can
  mis-target with a non-default `GIT_INDEX_FILE`.
- **Formatters (prettier, eslint --fix, gofmt)** — operate **purely on working-tree files**; they
  ignore `GIT_INDEX_FILE` entirely. Only the subsequent `git add` honors it.

**Implication for stagecoach**: on the single-commit snapshot path, the committed tree is
`write-tree` of the live index. If the working tree equals what's staged (the normal case), running
pre-commit with `GIT_INDEX_FILE=<throwaway-mirror>` is faithful. But stagecoach's value prop is
**stage-while-generating** — if the working tree has *unstaged* edits relative to the frozen tree,
formatters run on working-tree content, not frozen content → the pre-commit result may diverge.

**The PRD accepts this** (FR-V3): "A `pre-commit` hook may modify the content of paths already in
`T_start` (the common case: a formatter re-stages reformatted files); stagecoach accepts those
mutations and re-trees the snapshot." The working-tree mutation is a side effect that `git commit`
also keeps. This is acceptable and matches git-commit parity. **Do not try to snapshot the working
tree too** — that would forfeit the simplicity and the stage-while-generating property. The subset
check (only `T_start` paths may be staged by the hook) is the backstop.

**Recommended faithful sequence** (the scoped-index mechanism):
```
tmp := mktemp(); defer os.Remove(tmp)
GIT_INDEX_FILE=<abs tmp>
git read-tree <frozenTree>          # populate throwaway with the tree being committed
run hook "pre-commit"  (cwd=worktree root, env GIT_INDEX_FILE + GIT_EDITOR=:, stdin /dev/null)
  └─ non-zero → rescue (FR-V7); hook staging a non-T_start path → hard error (FR-V3 subset)
newTree := git write-tree            # capture hook-staged fixes (GIT_INDEX_FILE-scoped)
# subset check: DiffTreeNames(snapshotTree, newTree) must only add/modify snapshot paths
# prepare-commit-msg(<msgfile>) → read back, strip '#'
# commit-msg(<msgfile>) → read back as final
# commit-tree(newTree, parents, finalMsg) → update-ref CAS
# post-commit (exit code ignored, logged at --verbose)
```

**Gotchas to handle**:
1. `GIT_INDEX_FILE` must be **absolute** (a relative value resolves against the hook's CWD).
2. Hooks that re-stage write into the throwaway index → capture via write-tree (the plan does this).
3. A hook that hardcodes `.git/index` bypasses `GIT_INDEX_FILE` → the subset check (FR-V3) catches
   any non-`T_start` path it stages (hard error). This is the defense-in-depth backstop.
4. `git diff` (unstaged) semantics shift under `GIT_INDEX_FILE` — a hook computing "unstaged changes"
   diffs working tree against the throwaway, not the real index. Acceptable (rare).
