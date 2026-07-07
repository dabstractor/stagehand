# Codebase Reality — exact patterns, line numbers, and what exists vs. what's missing

Verified by reading source + scout subagent. All line numbers are current as of this planning session.

## 1. The exec boundary (the single hardest integration point)

`internal/git/git.go` has exactly **two** exec seams, and **neither sets `cmd.Env`**:

- **`run()`** (`git.go:389`): `exec.CommandContext(ctx, gitPath, "-C", repo, args...)`. argv as
  `[]string`, NO shell (PRD §19). `cmd.Env` is NEVER set → child inherits the parent environment.
  Captures stdout + stderr to separate buffers.
- **`runWithInput()`** (`git.go:430`): identical + `cmd.Stdin = stdin`. Used only by `CommitTree`.
  Explicit comment (`git.go:426-427`): "cmd.Env is NOT set here, so the child inherits the parent
  environment."

**`GIT_INDEX_FILE` appears NOWHERE in `internal/`** (grep across all `*.go` = zero matches). Every
index-mutating primitive (ReadTree/WriteTree/Add/AddAll/OverlayTreePaths/FreezeWorkingTree) operates
on the repo's **single default `.git/index`** as resolved by `git -C workDir`.

**Implication**: the scoped-index mechanism (§7 of external_deps.md) requires a **NEW env-passing
exec seam** — e.g. `runWithEnv(ctx, repo, env []string, args...)` that sets
`cmd.Env = append(os.Environ(), env...)`. Then thread `GIT_INDEX_FILE=<tmp>` through read-tree /
write-tree / update-index against the throwaway index. Nothing of this kind exists today.

## 2. Config patterns — the exact templates to mirror

### `NoVerify` → mirror `Push` (5-layer bool)

`Push` is the established 5-layer boolean. The exact plumbing (all verified):
- **`config.go`**: `Push bool` field with `toml:"push"`; comment documents it as §9.22 FR-P1. Lives
  in the `[generation]`-adjacent block but is `[generation]`-file-decoded (see file.go).
- **`Defaults()`** (`config.go`): `Push: false`.
- **`file.go`**: `fileGeneration.Push bool` (`toml:"push"`); `materialize()` copies via
  `if g.Push { c.Push = true }` (only-true-propagates — the v1 limitation: cannot set false via file);
  `overlay()` copies via `if src.Push { dst.Push = true }`.
- **`load.go`**: `loadEnv()` reads `STAGECOACH_PUSH` via `strconv.ParseBool` (DIRECT set — can be
  false); `loadFlags()` reads `--push` via `fs.Changed("push")` + `fs.GetBool` (DIRECT set).
- **`root.go`**: `var flagPush bool`; `pf.BoolVar(&flagPush, "push", false, "…")` at line 206.

`NoVerify` follows this EXACTLY: `toml:"no_verify"`, `Defaults() NoVerify: false`, the
only-true-propagates materialize/overlay, `STAGECOACH_NO_VERIFY` env (DIRECT set), `--no-verify` flag
(`fs.Changed` + `GetBool`, DIRECT set). The only-true-propagates limitation is accepted (document in
`docs/configuration.md` like Push/MultiTurnFallback).

### `HookTimeout` → mirror `Timeout` (5-layer duration)

`Timeout` is the established 5-layer duration:
- **`config.go`**: `Timeout time.Duration` (`toml:"timeout"`); `Defaults()`: `120 * time.Second`.
- **`file.go`**: `fileDefaults.Timeout string` (duration string "120s"); `loadTOML()` parses via
  `time.ParseDuration` (with validation up front); `materialize()` sets `c.Timeout = timeout`
  (zero-value sentinel: a zero Duration means "unset" → overlay skips it).
- **`overlay()`**: `if src.Timeout != 0 { dst.Timeout = src.Timeout }` (duration-zero sentinel).
- **`load.go`**: `STAGECOACH_TIMEOUT` via `parseTimeout` (accepts "120s" OR bare int 120); `--timeout`
  is a STRING flag (pflag) parsed via `parseTimeout`.
- **git-config**: `stagecoach.timeout` — but `HookTimeout` has NO git-config key per FR-V6 (config
  default 10m, file `[generation].hook_timeout`, flag... actually the PRD only lists it as config).
  **Decision**: `HookTimeout` is file + default only (no env/flag/git-config) — simplest; mirrors
  how `multi_turn_chunk_tokens` is `[generation]`+default only. If a per-run override is wanted, the
  user sets it in the config file. (The PRD §15.2 flag table does NOT list a `--hook-timeout` flag.)

So `HookTimeout`: `time.Duration` field, `toml:"hook_timeout"`, `Defaults()` `10 * time.Minute`,
fileGeneration struct + materialize (`!= 0` guard) + overlay (`!= 0` guard). NO env, NO flag, NO
git-config (config-file + default only). This matches FR-V6's "config knob" framing.

## 3. The git primitives — signatures + scoped-index gap

All on `*gitRunner` (single field `workDir string`, `git.go:369`); `Git` interface at `git.go:87`;
`New(workDir) Git` at `git.go:375`.

| Primitive | Line | Signature | Index discipline | Scoped? |
|---|---|---|---|---|
| `run` | 389 | `(ctx, repo, args...) (stdout, stderr, code, err)` | — | NO env |
| `runWithInput` | 430 | `(ctx, repo, stdin, args...) (...)` | — | NO env |
| `WriteTree` | 492 | `(ctx) (sha, err)` | READS `.git/index` | No |
| `CommitTree` | 520 | `(ctx, tree, parents []string, msg) (sha, err)` | none (object only) | n/a |
| `UpdateRefCAS` | 562 | `(ctx, ref, newSHA, expectedOld) error` | none | n/a (sole ref mutation) |
| `ReadTree` | 1222 | `(ctx, tree) error` | REPLACES `.git/index` | No |
| `AddAll` | 1118 | `(ctx) error` | mutates `.git/index` | No |
| `Add` | 1136 | `(ctx, paths []string) error` | mutates `.git/index` | No |
| `OverlayTreePaths` | 1643 | `(ctx, baseTree, sourceTree, paths) (treeSHA, err)` | read-tree base → per-path update-index → write-tree | No |
| `FreezeWorkingTree` | 1562 | `(ctx, baseTree) (tStart, err)` | AddAll → WriteTree → ReadTree(base) | No |
| `DiffTreeNames` | 1582 | `(ctx, treeA, treeB) ([]string, err)` | read-only | n/a |
| `DiffTreeNameStatus` | 1813 | `(ctx, treeA, treeB) (nameStatus, err)` | read-only | n/a |
| `HooksPath` | 1752 | `(ctx) (string, err)` | read-only | n/a |
| `DiffTree` | 589 | `(ctx, sha, isRoot) ([]FileChange, err)` | read-only | n/a |

**`EmptyTreeSHA`** = `4b825dc642cb6eb9a060e54bf8d69288fbee4904` (`git.go:697`).

**The scoped-index gap**: NONE of these accept a custom index path. To run `pre-commit` against a
frozen tree without touching the live `.git/index`, the runner needs either:
(a) a new env-passing exec seam + scoped `ReadTreeInto`/`WriteTreeFrom`/`UpdateIndexIn` variants
    that take a `GIT_INDEX_FILE`, OR
(b) a single higher-level helper `RunHookAgainstTree(ctx, tree, hookName, env, ...) (postTree, err)`
    that manages the throwaway index internally (mktemp → `GIT_INDEX_FILE=<abs tmp>` →
    `read-tree <tree>` → run hook → `write-tree` → cleanup), plus a `DiffTreeNames`-based subset
    check.

**Recommendation**: add (a) the env seam + minimal scoped variants (they're thin), then build (b)
the higher-level helper on top. This keeps the git package's primitives composable and testable, and
the runner (in `internal/hooks/`) composes them. See `open_questions.md` §1.

## 4. The hook-MODE module (`internal/hook/`) — the existing §9.20 surface

These are the EXISTING prepare-commit-msg install/detect primitives (NOT what we're building). The
new runner imports them for recursion prevention only.

- **`Marker`** (`script.go:15`): `const Marker = "# stagecoach prepare-commit-msg hook v1"` — the
  identity line stagecoach writes as line 2 of its hook. Detection is `strings.Contains(data, Marker)`.
- **`HookFilename`** (`hook.go:17`): `const HookFilename = "prepare-commit-msg"`.
- **`Status`** (`hook.go:21`): `StatusNone` / `StatusStagecoach` / `StatusForeign`.
- **`Detect(hooksDir)`** (`hook.go:62`): `(Status, error)`. `os.ErrNotExist` → `StatusNone`; contains
  `Marker` → `StatusStagecoach`; present without Marker → `StatusForeign`.
- **`Install(hooksDir, strict, configPath)`** (`hook.go:80`): refuses on `StatusForeign`
  (`ErrForeignHook`); idempotent rewrite on `StatusStagecoach`.
- **`Script(strict, configPath)`** (`hook.go:124`): the `#!/bin/sh\n<Marker>\n…exec stagecoach hook
  exec "$@"\n` template.

**For FR-V4 (recursion prevention)**: the runner, before invoking `prepare-commit-msg`, calls
`hook.Detect(hooksDir)`; if `StatusStagecoach`, **skip** it (the message is already generated —
invoking it would recurse into `stagecoach hook exec`, which would regenerate). A foreign
`prepare-commit-msg` (StatusForeign) runs and may annotate; stagecoach reads the message file back.

## 5. The commit chokepoints — verified pipeline order

### `generate.CommitStaged` (the single-commit COMMIT path)
```
generate.go:
  RevParseHEAD (parent + isUnborn)                     [step 1]
  buildSystemPrompt                                     [step 2]
  StagedDiff (empty → ErrNothingToCommit)              [step 3]
  WriteTree → treeSHA *** SNAPSHOT ***                 [step 4]  signal.SetSnapshot + lock.SetSnapshot
  recentSubjects                                        [step 5]
  generate→parse→dedupe LOOP (incl. FR-T1 multi-turn)  [step 6]
  EditMessage (the --edit gate)                        [line 389]
  >>> [HOOKS INSERT HERE: pre→prepare→commit-msg] <<<  [between 389 and 399]
  CommitTree(treeSHA, parents, msg) → newSHA           [line 399, step 7]
  signal.RestoreDefault                                 [before UpdateRefCAS]
  UpdateRefCAS(HEAD, newSHA, expectedOld)               [line 410, step 8]
  >>> [post-commit INSERT HERE: best-effort] <<<       [after 410, before signal.ClearSnapshot@428]
  DiffTree (FR42 report)                                [step 9]
  return Result                                         [step 10]
```

### `pkg/stagecoach.runPipeline` (the single-commit DRY-RUN / SystemExtra path)
`stagecoach.go:411`. Self-contained mirror of CommitStaged for `opts.DryRun || opts.SystemExtra != ""`
(the common `!DryRun && SystemExtra==""` path delegates to `generate.CommitStaged` at line 148).
`runPipeline` runs the full pipeline but **skips CommitTree/UpdateRefCAS under DryRun** (the dangling
tree is intentional/harmless). **For FR-V8a (dry-run: skip pre/post-commit, run commit-msg on the
would-be message)**, this path needs commit-msg only. This is the SECOND single-commit chokepoint the
delta_prd's R3 didn't explicitly name — **the implementing agent must wire commit-msg into runPipeline's
dry-run branch** (or confirm the cmd-layer dry-run handling covers it).

### `decompose.publishCommit` (the multi-commit path)
`message.go:219`: `publishCommit(ctx, deps, tree, parentSHA, msg) (newSHA, error)` — the single
chokepoint for ALL decompose commits:
- main loop publish closure (`decompose.go:484`)
- `runSingleShortcut` (`:390`)
- `runOneFileShortcut` (`:336`)
- arbiter `resolveNewCommit` (chain.go:81) and `resolveTipAmend` (chain.go:137) — these produce
  user-facing commits and SHOULD run hooks.
- `runSingleEscape` delegates to `generate.CommitStaged` → covered by the CommitStaged wiring.

**`resolveMidChain` (chain.go:186)** is the silent deterministic rebuild: walks `j = i..N-1`, builds
`treePrime = OverlayTreePaths(tree[j], T_start, leftoverPaths)` then
`CommitTree(treePrime, parents, chainData[j].Message)` — **`msg[j]` reused verbatim**. This MUST SKIP
hooks (open Q#2). It does NOT go through `publishCommit` (it does its own CommitTree+UpdateRefCAS at
chain.go:203/215), so skipping hooks here is natural — just don't wire the runner into it.

`generateMessage` (message.go:91) is the message-generation loop (NOT a commit chokepoint); it ends
with `EditMessage` (the --edit gate) and returns the message. Hooks run at `publishCommit`, after
`generateMessage` returns.

## 6. The rescue path (reused UNCHANGED)

`generate.RescueError` (`generate.go`): `{Kind, TreeSHA, ParentSHA, Candidate, Cause}`. `Kind` is
`ErrTimeout` or `ErrRescue`. The CLI does `errors.As(err, &re)` → `FormatRescue(re.TreeSHA,
re.ParentSHA, re.Candidate)` → exit 124 (timeout) or 3 (rescue).

**FR-V7**: a hook failure (pre/prepare/commit-msg non-zero or timeout) returns
`*generate.RescueError{Kind: ErrRescue, TreeSHA: <frozen tree>, ParentSHA, Candidate: <the message>,
Cause: <hook error>}` — byte-identical to a generation failure. `FormatRescue` prints the frozen-tree
recovery recipe unchanged. No new exit code, no new rescue variant. The hook's stderr is surfaced
(the user's hook's own diagnostic).

## 7. Signal handling (reused UNCHANGED)

`internal/signal` arms rescue on `signal.SetSnapshot(treeSHA, parentSHA, "")` (generate.go step 4).
A SIGINT/SIGTERM after the snapshot triggers rescue. `signal.RestoreDefault()` runs before
`UpdateRefCAS` (so a Ctrl-C at the last instant isn't mistaken for failure); `signal.ClearSnapshot()`
on success. Hooks thread between EditMessage and CommitTree — **inside the armed-snapshot window**,
so a Ctrl-C during a hook still triggers the existing rescue. No signal change needed.

## 8. The docs that contradict the feature (Mode B headline)

**`docs/how-it-works.md:312`** (the "Hook mode vs the snapshot-based flow" section): explicitly states
the snapshot flow *"Bypasses pre-commit hooks"* and frames hook mode as the way to get hooks —
**both now FALSE**. This is the headline Mode B rewrite. The "When to use which" section (lines
327-329) also needs reframing: the two modes now COMPOSE (hook mode covers `git commit`; the snapshot
flow covers `stagecoach` and now honors hooks too).

Other doc surfaces (Mode A, ride with work):
- `docs/cli.md` global-flags table — add `--no-verify` row (after `--push` at line 43).
- `docs/configuration.md` `[generation]` table (line ~145, near `push`) — add `hook_timeout`;
  add `no_verify` to the bool-flag surface if that section lists bools.
