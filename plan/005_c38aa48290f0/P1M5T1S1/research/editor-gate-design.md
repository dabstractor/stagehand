# Editor gate (`--edit`) design — research notes

Date: 2026-07-03. Gates FR-E1/E2/E3/E4 (PRD §9.22). Built on the **completed** P1.M2.T2.S2
`generate.FinalizeMessage` seam (ordering contract: template → editor → publish).

## 1. The seam contract (from P1.M2.T2.S2 — the load-bearing integration point)

`internal/generate/finalize.go` already publishes the ORDERING CONTRACT in its godoc:

> ORDERING CONTRACT (P1.M5.T1.S1): the --edit editor gate slots AFTER this seam (FR-E3: the template
> is applied before the editor opens). Extend the pipeline as template → (future) editor → publish;
> keep template first.

So the editor gate is **stage 2 of the FinalizeMessage pipeline**. Today `FinalizeMessage(msg, cfg) =
ApplyTemplate(msg, cfg.Template)`. P1.M5.T1.S1 extends it to `ApplyTemplate → EditMessage`.

**Decision: extend `FinalizeMessage` itself** (not a separate seam call). The 4 existing call sites
(generate.go L237, stagecoach.go runPipeline, decompose/message.go, decompose/runSingleShortcut) already
invoke `FinalizeMessage`. By adding the editor stage INSIDE FinalizeMessage, all 4 sites gain `--edit`
transitively — the same transitive-coverage argument P1.M2.T2.S2 used. This is the cleanest integration:
the seam is the single funnel; the editor is a new stage of it.

## 2. The 5 commit sites and where the editor must fire

| # | Site | File | Current seam call | Editor fires? | Why |
|---|------|------|-------------------|---------------|-----|
| 1 | single-commit loop | `internal/generate/generate.go` L237 `m = FinalizeMessage(m, cfg)` | ✓ via seam | YES | FR-E1 single path |
| 2 | public-API runPipeline | `pkg/stagecoach/stagecoach.go` `m = generate.FinalizeMessage(m, cfg)` | ✓ via seam | YES | parity with #1 |
| 3 | decompose message role | `internal/decompose/message.go` `m = generate.FinalizeMessage(m, deps.Config)` | ✓ via seam | YES | FR-E4 each commit |
| 4 | decompose FR-M11 shortcut | `internal/decompose/decompose.go` `msg := generate.FinalizeMessage(plannerMsg, deps.Config)` | ✓ via seam | YES | FR-E4 + FR-M11 |
| 5 | decompose arbiter N+1 | `internal/decompose/chain.go` `resolveNewCommit` → calls `generateMessage` (#3) | transitively | YES | FR-E4 (covered by #3) |

**All 5 are covered by extending FinalizeMessage.** Zero new call sites. P1.M2.T2.S2's seam placement
proves itself here: the editor slot is FREE because the seam is already at the right point (after
template, before dedupe — which is exactly where FR-E3 wants the editor: "the template is applied
before the editor opens").

### CRITICAL: the editor needs the SUMMARY (tree SHA + diff-tree name-status). The seam doesn't have it.

`FinalizeMessage(msg, cfg)` takes only `msg` + `cfg`. The EDITMSG file needs:
- the message (have it — `msg`)
- a commented summary: tree SHA + `diff-tree --name-status` (NOT available in FinalizeMessage)

The tree SHA is captured at step 3 of CommitStaged (`treeSHA`), and at the freeze point in decompose.
The DiffTree/TreeDiff name-status is NOT currently captured for the EDITMSG — it must be computed.

**Decision: pass the summary data THROUGH the seam.** Extend `FinalizeMessage`'s signature to accept an
`EditContext` carrying the tree SHA + the name-status lines. Two options:

- **Option A (chosen):** add an `EditContext` struct + change `FinalizeMessage` to
  `FinalizeMessage(msg string, cfg config.Config, editCtx EditContext) string`. The 4 call sites already
  hold the tree SHA (generate.go `treeSHA`; decompose has `treeB`/`tStart`). The name-status is computed
  via a NEW `git.Git` method `DiffTreeNamesStatus(treeA, treeB)` (or `DiffTree` on the commit). **This
  changes a public-ish function signature** — but FinalizeMessage has exactly 4 callers, all in this
  repo, and the change is additive (EditContext can be zero-value = no editor).
- **Option B:** keep FinalizeMessage as-is (template only) and add a SEPARATE `EditMessage(msg, cfg, editCtx)`
  call site at each of the 4 points. More edits, but no signature change.

**Recommendation: Option A** — the seam is meant to be the single funnel, and P1.M2.T2.S2 explicitly
designed it to be EXTENDED. A zero-value `EditContext{}` (empty TreeSHA) means "no editor summary
available" → the editor still opens but with just the message + a generic comment. This keeps the
feature coherent (one seam, one pipeline) and matches the published contract. The signature change is
internal (all 4 callers are in-repo).

### The tree SHA + name-status availability per site:

| Site | tree SHA available? | name-status source |
|------|---------------------|--------------------|
| generate.go (#1) | `treeSHA` (step 3, WriteTree) ✓ | `DiffTree` against parent — but commit not yet made. Use `git diff-tree --name-status -r --root <parent-tree> <treeSHA>` OR `git diff --name-status <parentTree> <treeSHA>`. Simplest: a NEW git helper `DiffTreeNameStatus(treeA, treeB)` returning the raw name-status string. parentTree = `git rev-parse HEAD^{tree}` (or EmptyTreeSHA if unborn). |
| stagecoach.go (#2) | `treeSHA` ✓ | same as #1 |
| decompose/message.go (#3) | `treeB` ✓ (the concept tree) | `treeA..treeB` name-status (the concept diff) — `DiffTreeNameStatus(treeA, treeB)`. Already have treeA+treeB in generateMessage's signature. |
| decompose/runSingleShortcut (#4) | `tStart` ✓ | `baseTree..tStart` name-status. baseTree available in signature. |
| chain.go arbiter N+1 (#5) | `treePrime` ✓ (in resolveNewCommit) | `treeA..treePrime` name-status. Covered transitively via #3. |

**So every site has the tree SHA + the two trees needed for name-status already in scope.** The only
new git primitive needed is `DiffTreeNameStatus(treeA, treeB) (string, error)` returning the raw
`git diff-tree --name-status -r <treeA> <treeB>` output (A/M/D lines). For generate.go #1/#2, treeA =
the parent's tree (HEAD^{tree} or EmptyTreeSHA).

## 3. Editor resolution — `git var GIT_EDITOR` (external_deps.md §6, VERIFIED)

`internal/git` has NO `GitDir()` or `GitEditor()` method yet. Both are needed:
- `GitDir()` → resolve `.git/STAGECOACH_EDITMSG` path (use `git rev-parse --git-path .` or `--absolute-git-dir`).
- `GitEditor()` → resolve the editor command (`git var GIT_EDITOR`).

**Decision: add TWO methods to the `git.Git` interface:**
1. `GitDir(ctx) (string, error)` — `git rev-parse --absolute-git-dir` (git 2.13+, universally available;
   returns the absolute `.git` dir, honors worktrees + commondir). The EDITMSG file goes in
   `<gitDir>/STAGECOACH_EDITMSG` (FR-E1). `--absolute-git-dir` exits 128 on non-repo (real error, like
   HooksPath's `--git-path hooks`).
2. `Editor(ctx) (string, error)` — `git var GIT_EDITOR`. Returns the resolved editor string
   (GIT_EDITOR → core.editor → VISUAL → EDITOR → vi — a faithful superset of FR-E1's chain). The value
   is **shell-interpreted** (may contain args/quotes), so the caller invokes it via
   `sh -c '<editor> "$@"' -- <file>`, NEVER a bare `exec.Command(editor, file)`.

Both follow the existing `run()` exec pattern (separate buffers, exit-code-as-signal). They are
read-only w.r.t. refs/index (safe).

## 4. The EDITMSG round-trip (FR-E1)

```
1. resolve gitDir = Git.GitDir(ctx)
2. editMsgPath = filepath.Join(gitDir, "STAGECOACH_EDITMSG")
3. build the EDITMSG content:
   <message>
   # Please edit this commit message. Lines starting with '#' will be stripped.
   # Tree: <treeSHA>
   # <name-status lines, each prefixed with '# '>
4. write editMsgPath (0644)
5. resolve editor = Git.Editor(ctx); if err → fall back to os.Getenv("EDITOR")/"vi" (best-effort, never fatal)
6. invoke: sh -c '<editor> "$@"' -- <editMsgPath>  (cmd.Stdin/out/err = os.Stdin/out/err — interactive)
7. read editMsgPath back
8. strip: remove lines starting with '#', strip trailing whitespace per line, trim overall
9. if result is empty (after strip) → return ErrEmptyMessage (caller aborts: exit 1, HEAD+index untouched)
10. return the stripped message
```

**Strip semantics (git parity):** git's `prepare-commit-msg` strips lines beginning with `#` (comment
char) and trailing whitespace. We replicate: split on `\n`, drop lines where `strings.HasPrefix(line, "#")`,
then `strings.TrimSpace` each surviving line, drop now-empty lines, join with `\n`. A message that is
only comments/whitespace → empty → abort.

## 5. The abort is NOT a rescue (FR-E1 critical)

FR-E1: *"An empty result aborts with exit 1 ("empty commit message — aborted"; intentional abort, not a
rescue: HEAD and the index are untouched, the orphan tree object is garbage-collected by git eventually)."*

The editor runs AFTER WriteTree (the snapshot exists) but BEFORE CommitTree (no commit object made).
So on abort: HEAD untouched (CAS never attempted), index untouched (WriteTree is read-only), and the
only artifact is the orphan `treeSHA` (gc'd eventually — harmless, same as any failed generation).

**The new sentinel:** `var ErrEmptyMessage = errors.New("stagecoach: empty commit message — aborted")`.
Returned from the editor stage when the stripped result is empty. The CLI maps it to **exit 1** with the
message "empty commit message — aborted" (NOT exit 3/124 — not a rescue). The FinalizeMessage seam
propagates it; the generation loops must NOT treat it as a retryable parse-failure (it's a hard abort —
the user explicitly emptied the message).

**CRITICAL loop interaction:** the editor stage is INSIDE the generation loop (via FinalizeMessage). If
the editor returns ErrEmptyMessage, the loop must NOT `continue` (retry) — it must return immediately
(abort the whole run). This means FinalizeMessage returns `(string, error)` now, OR the editor returns
a sentinel the loop checks. **Decision: change FinalizeMessage to return `(string, error)`.** On
`errors.Is(err, ErrEmptyMessage)` the loops return a NEW typed error wrapping it that the CLI maps to
exit 1. All 4 call sites already `return` on error from their loop body — adding the error return is
mechanical.

**Signature change (Option A confirmed):**
`FinalizeMessage(msg string, cfg config.Config, editCtx EditContext) (string, error)`

## 6. FR-E3: bypass duplicate re-check

FR-E3: *"The edited message is user-authored. It bypasses the duplicate re-check (git parity: git never
rejects a hand-written message)."*

This is ALREADY satisfied by seam placement IF the editor runs AFTER dedupe. But P1.M2.T2.S2 placed
FinalizeMessage BEFORE dedupe (so the template is visible to dedupe). The editor must NOT be visible to
dedupe (the user wrote it; we don't second-guess).

**Resolution: the FinalizeMessage pipeline must be SPLIT in its relationship to dedupe:**
- **template stage:** BEFORE dedupe (FR-F8 — dedupe sees templated subject) ✓ already done
- **editor stage:** AFTER dedupe (FR-E3 — user message bypasses re-check)

This means the editor CANNOT live inside the current FinalizeMessage (which is pre-dedupe). **Revised
design:** the editor is a SEPARATE stage called AFTER the dedupe check accepts the message, BEFORE
CommitTree. The seam ordering becomes:

```
parse → template (FinalizeMessage, pre-dedupe) → dedupe → EDITOR (post-dedupe) → CommitTree
```

**This requires Option B after all:** a separate `EditMessage(msg, cfg, editCtx) (string, error)` call,
inserted at each of the 4 sites AFTER the dedupe `break`/accept and BEFORE `publishCommit`/`CommitTree`.
This is a SMALL number of well-defined insertion points (one per commit path, right before publication).

### Revised insertion points (after dedupe accept, before publish):

| Site | After | Before |
|------|-------|--------|
| generate.go (#1) | loop `break` (msg accepted) L~248 | `CommitTree` L~255 (step 7) |
| stagecoach.go (#2) | loop break | CommitTree equivalent |
| decompose/message.go (#3) | loop `break` (msg accepted) | `return msg, nil` (the publish happens in runLoop via publishCommit) |
| decompose/runSingleShortcut (#4) | the dup-check block L~325 | `publishCommit` L~335 |

For decompose, the editor gate happens in `generateMessage`/`runSingleShortcut` BEFORE returning the
message — the `runLoop`'s overlapped generation + serialized publication is preserved (the editor runs
during the message-generation phase, before publishCommit's CAS).

**CRITICAL for FR-E4 "decompose gates EACH commit":** the editor runs per-commit (inside
generateMessage for #3/#5, inside runSingleShortcut for #4). The user edits each message in sequence.
The snapshot is frozen (treeB/tStart) so staging the next batch during the edit is safe (FR-E2).

## 7. FR-E4 composition

- `--dry-run` + `--edit`: `--edit` IGNORED with a warning ("--edit ignored in --dry-run mode"). The
  dry-run path returns after generation (no CommitTree, no editor). Detected in the CLI: if both flags
  set, print warning to stderr and skip the editor. (Simplest: the EditMessage function checks a
  `cfg.DryRun`-equivalent — but dry-run is a CLI flag, not cfg. So gate in the CLI: don't call EditMessage
  when dry-run. OR: pass a `skipEdit bool` through. **Decision: EditMessage takes `cfg`, and we add
  `cfg.Edit bool` (the resolved flag); the dry-run gate is at the call site — the CLI sets up the run
  such that dry-run short-circuits before publication, so EditMessage is naturally not reached on
  dry-run. Add an explicit guard anyway: if the caller is in a dry-run context, skip.**)
- `hook exec` + `--edit`: **usage error** (FR-E4). `--edit` is rejected on the `hook exec` subcommand.
  Register `--edit` as a GLOBAL persistent flag on root (NOT on hookExecCmd). Then in hookexec.go's
  runHookExec, if `flagEdit` is changed, return a usage error (exit 2 / cobra error). Actually cleaner:
  do NOT register `--edit` on hookExecCmd at all; but persistent flags inherit... The cobra precedent
  (from P1.M2.T2.S2's `config init --template` collision) is that a LOCAL flag shadows a persistent one.
  But here we want a HARD error, not a shadow. **Decision: in runHookExec, check `cmd.Flags().Changed("edit")`
  (or the persistent flag); if set, return exitcode.New(exitcode.Usage, errors.New("--edit is not valid
  with hook exec (git already opens the editor)")). cobra's `SilenceUsage` + a returned error → exit 1
  with the message. Match the exit code convention used elsewhere for usage errors.**

## 8. FR-E2: edit-while-staging stays safe

The snapshot (treeSHA/treeB/tStart) is frozen at WriteTree/freeze time — BEFORE the editor opens. The
editor only reads/writes the EDITMSG file; it never touches the index or HEAD. So the user can `git add`
the next batch in another pane during the editor session; the in-flight commit is unaffected (identical
to the existing stage-while-generating property, §13.4). No code needed — the invariant holds by
construction (the editor is a pure file round-trip between WriteTree and CommitTree). **Docs must call
this out explicitly (FR-E2 requires the docs call-out).**

## 9. Testing strategy (fake-editor scripts)

FR-E1 contract: "Tests: fake-editor scripts (EDITOR set to a script that rewrites / empties / leaves the
file) across single + decompose paths."

**Pattern (Go):** in tests, set the editor to a small script via a temp file:
```go
// fake editor that rewrites the message
script := t.TempDir() + "/fakeeditor.sh"
os.WriteFile(script, []byte("#!/bin/sh\necho 'rewritten message' > \"$1\"\n"), 0755)
// override Git.Editor via a stub Git that returns the script path
```
Since `Git.Editor(ctx)` is the resolution point, tests inject a stub `git.Git` whose `Editor()` returns
the fake script path. Three fakes:
1. **rewrite** — `echo 'rewritten subject' > "$1"` → edited message lands.
2. **empty** — `: > "$1"` (truncate) → stripped result empty → ErrEmptyMessage → abort.
3. **leave** — `exit 0` without touching → original message + comments stripped → original lands.

Run each across: single-commit (generate.CommitStaged via generate_test stub harness), decompose
(generateMessage + runSingleShortcut). The existing test doubles (stubtest.Manifest, git test repo in
t.TempDir()) are reused; only the Git stub gains an `Editor()` method + `GitDir()` method.

## 10. Config plumbing (mirrors --template / --context)

`--edit` is a FLAG (FR-E1: "flag only, default false"). Unlike --template it has NO env/config/git
precedence (the item says "flag only"). **Decision: mirror `--context` (P1.M2.T2.S1's flag-only
precedent), NOT --template's full precedence.** Add `Config.Edit bool \`toml:"-"\`` (toml:"-" = not a
file key, like Context). Wire: `--edit` flag on root → loadFlags reads it → cfg.Edit. Default false.

## 11. Summary of NEW code

| Artifact | Location | Purpose |
|----------|----------|---------|
| `git.GitDir(ctx)` | internal/git/git.go (interface + impl) | resolve `.git` dir for EDITMSG path |
| `git.Editor(ctx)` | internal/git/git.go (interface + impl) | resolve editor via `git var GIT_EDITOR` |
| `git.DiffTreeNameStatus(ctx, treeA, treeB)` | internal/git/git.go (interface + impl) | raw name-status for the EDITMSG summary |
| `generate.EditMessage(msg, cfg, editCtx)` | internal/generate/finalize.go (NEW func) | the EDITMSG round-trip (write→editor→strip→validate) |
| `generate.EditContext{TreeSHA, NameStatus, Git}` | internal/generate/finalize.go (NEW struct) | carries the summary data + the Git boundary |
| `generate.ErrEmptyMessage` | internal/generate/finalize.go (NEW sentinel) | abort signal (exit 1, not rescue) |
| `Config.Edit bool \`toml:"-"\`` | internal/config/config.go (NEW field) | flag-only config |
| `--edit` flag | internal/cmd/root.go (NEW StringVar→BoolVar) | global persistent flag |
| hook-exec usage guard | internal/cmd/hookexec.go (EDIT) | reject --edit on hook exec (FR-E4) |
| 4 EditMessage call sites | generate.go, stagecoach.go, decompose/message.go, decompose/decompose.go | post-dedupe, pre-publish |
| CLI exit mapping | internal/cmd/default_action.go (EDIT) | ErrEmptyMessage → exit 1 "empty commit message — aborted" |
| docs/cli.md | docs/cli.md (EDIT) | --edit row incl. abort semantics |
| docs/how-it-works.md | docs/how-it-works.md (EDIT) | FR-E2 edit-while-staging call-out |

**Transitive coverage proof (revised):** the 4 explicit EditMessage calls cover all 5 sites (arbiter N+1
reuses generateMessage → #3 covers it). No edit to chain.go.
