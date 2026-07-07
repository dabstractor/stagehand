name: "P1.M5.T1.S1 — --edit: EDITMSG round-trip inserted at the message-finalization seam"
description: |
  Add the `--edit` workflow convenience (PRD §9.22 FR-E1/E2/E3/E4, → G19): a FLAG-ONLY (default false)
  editor gate that opens the user's editor on `.git/STAGECOACH_EDITMSG` (the message + a commented summary
  of tree SHA + diff-tree --name-status), and on close strips comment lines + trailing whitespace and
  publishes the edited message via the normal plumbing path. An empty result ABORTS (exit 1 "empty commit
  message — aborted") — an INTENTIONAL abort, NOT a rescue (HEAD and index untouched; the orphan tree is
  gc'd eventually). The edited message is USER-AUTHORED: it BYPASSES the duplicate re-check (FR-E3, git
  parity) and the template (FR-F8) is applied BEFORE the editor opens (so the user edits final text).
  The snapshot is frozen before the editor opens, so editing-while-staging stays safe (FR-E2 — same §13.4
  property, extended through the editor; docs/how-it-works.md MUST call this out explicitly).
  In decompose mode `--edit` gates EACH commit's message before its serialized publication (including the
  FR-M11 shortcut and the arbiter N+1, FR-E4); with `--dry-run` it is IGNORED with a warning; on
  `hook exec` it is a USAGE ERROR (FR-E4).

  IMPLEMENTATION STRATEGY (post-dedupe editor stage — see research/editor-gate-design.md §6 for the
  dedupe-vs-editor ordering proof): the editor is a NEW stage `generate.EditMessage(msg, cfg, editCtx)`
  inserted at 4 explicit sites AFTER the dedupe loop accepts a message and BEFORE publication (CommitTree
  / publishCommit). It does NOT live inside P1.M2.T2.S2's `FinalizeMessage` (which is pre-dedupe so the
  template is visible to dedupe — FR-F8); the editor is post-dedupe so the user's hand-written message
  bypasses the re-check (FR-E3). The 4 sites + transitive coverage of the arbiter N+1 (chain.go's
  resolveNewCommit reuses generateMessage → site #3) = every commit path. Editor resolution shells to
  `git var GIT_EDITOR` (external_deps.md §6, VERIFIED — faithful superset of FR-E1's chain; shell-interpreted
  → invoke via `sh -c '<editor> "$@"' -- <file>`, never bare exec). The `.git/` dir + the name-status
  summary come from TWO NEW `git.Git` interface methods: `GitDir(ctx)` (`git rev-parse --absolute-git-dir`)
  and `DiffTreeNameStatus(ctx, treeA, treeB)` (`git diff-tree --no-commit-id --name-status -r`). A THIRD
  new method, `Editor(ctx)` (`git var GIT_EDITOR`), is the editor-resolution seam (stub-able in tests).

  CONSUMES: P1.M2.T2.S2's `FinalizeMessage` seam + `cfg.Template` (template applies BEFORE the editor,
  FR-E3 ordering — the seam already publishes this contract); the existing `git.Git` boundary; the
  generate/decompose loops' accept-then-publish structure. PROVIDES: `--edit` flag, `EditMessage`,
  `EditContext`, `ErrEmptyMessage`, 3 new git.Git methods, the abort exit mapping, docs. NO edits to
  the prompt package, chain.go, or hook install. The non-interactive default path is UNTOUCHED (cfg.Edit
  == false ⇒ EditMessage is a no-op short-circuit).

---

## Goal

**Feature Goal**: `stagecoach --edit` opens the user's editor (resolved via `git var GIT_EDITOR` →
`core.editor` → `$VISUAL` → `$EDITOR` → `vi`) on `.git/STAGECOACH_EDITMSG` containing the generated message
plus a commented summary (tree SHA + the diff-tree --name-status of the snapshot), and commits the
stripped result via the normal plumbing path. The user edits the FINAL text (template already applied,
FR-E3). An empty result aborts (exit 1, not a rescue). The edited message bypasses the duplicate
re-check (FR-E3). The snapshot is frozen before the editor opens, so the user can stage the next batch
during the edit (FR-E2). In decompose mode every commit's message is gated (FR-E4).

**Deliverable**:
1. `internal/generate/finalize.go` (EXTEND) — `EditMessage(msg string, cfg config.Config, editCtx EditContext) (string, error)`,
   `EditContext{Git git.Git; TreeSHA, NameStatus string}`, `ErrEmptyMessage` sentinel. No-op when `cfg.Edit == false`.
2. `internal/git/git.go` (EXTEND the `Git` interface + impl) — `GitDir(ctx) (string, error)` (`git rev-parse --absolute-git-dir`),
   `Editor(ctx) (string, error)` (`git var GIT_EDITOR`), `DiffTreeNameStatus(ctx, treeA, treeB) (string, error)`
   (`git diff-tree --no-commit-id --name-status -r <treeA> <treeB>`).
3. `internal/config/config.go` (EDIT) — `Config.Edit bool \`toml:"-"\`` (flag-only, like `Context`) + `Defaults()` `Edit: false`.
4. `internal/config/load.go` (EDIT) — `loadFlags` reads `--edit` into `cfg.Edit`.
5. `internal/cmd/root.go` (EDIT) — `flagEdit bool` + `pf.BoolVar(&flagEdit, "edit", false, "...")` (global persistent).
6. `internal/cmd/default_action.go` (EDIT) — `--dry-run` + `--edit` → warn + skip the editor (FR-E4).
7. `internal/cmd/hookexec.go` (EDIT) — `--edit` on `hook exec` → usage error (FR-E4).
8. `internal/exitcode/exitcode.go` (EDIT) — `For()` maps `generate.ErrEmptyMessage` → `exitcode.Error` (exit 1).
9. 4 EditMessage call sites (EDIT) — generate.go, pkg/stagecoach/stagecoach.go, decompose/message.go,
   decompose/decompose.go (`runSingleShortcut`); arbiter N+1 covered transitively via #3 (chain.go UNTOUCHED).
10. docs/cli.md (EDIT) — `--edit` global-flags row incl. abort semantics; docs/how-it-works.md (EDIT) —
    the FR-E2 edit-while-staging property (REQUIRED docs call-out).
11. Tests — fake-editor scripts (rewrite/empty/leave) across single + decompose paths; the 3 new git
    methods; EditMessage strip semantics; the abort exit code.

**Success Definition**:
- `--edit` unset → every commit path byte-identical to today (EditMessage is a `cfg.Edit`-gated no-op;
  all existing tests pass unchanged — the regression guard).
- `stagecoach --edit` (single commit) → the editor opens on `.git/STAGECOACH_EDITMSG` containing the message
  + `# Please edit...` + `# Tree: <sha>` + `# <name-status lines>`; on save the stripped message lands.
- A message emptied in the editor → `stagecoach` exits 1 with "empty commit message — aborted"; HEAD and
  index untouched; no commit object created.
- The edited message is NOT re-checked for duplicates (a hand-edited message matching a recent subject
  still commits — FR-E3 git parity).
- The user can `git add` in another pane during the editor session; the in-flight commit is unaffected (FR-E2).
- In decompose mode, EVERY commit's message is gated (per-concept, the FR-M11 shortcut, the arbiter N+1).
- `stagecoach --dry-run --edit` → warns "--edit ignored in --dry-run mode" and does NOT open the editor.
- `stagecoach hook exec ... ` with `--edit` inherited → usage error (FR-E4).
- `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l` all green.

## User Persona

**Target User**: the "plan-holder" (PRD §7.1) who trusts the generated message ~90% but wants a final
eyeball + tweak before it lands — the same reason `git commit -e` exists, but on top of generation and
without losing stagecoach's snapshot/stage-while-generating property. Their fear: "the AI wrote something
slightly wrong and I can't fix it without re-running." `--edit` lets them fix it in-place, once.

**Use Case**: `stagecoach --edit` (or `git stagecoach --edit` via the alias, or the lazygit keybind with
`--edit` baked in) → generation runs → the editor pops with the message + a summary of what's landing →
the user tweaks a word, saves, closes → the commit lands with their edit.

**User Journey**: `stagecoach --edit` → generation → editor opens → review/tweak the message (the
name-status summary shows exactly what's in the commit) → save+close → `[<sha>] <subject>` prints.
If they clear the message to abort: `empty commit message — aborted` (exit 1, nothing changed).

**Pain Points Addressed**: incumbents (opencommit) lack an edit gate entirely; aicommits has `--emoji`
but no `-e`. Git's own `commit -e` is the gold standard but cannot ride on top of generation. `--edit`
delivers git-parity editing WITHOUT sacrificing stagecoach's snapshot atomicity or the stage-while-generating
overlap (FR-E2 — unique to the snapshot architecture, impossible with `git commit -e`).

## Why

- **FR-E1 (PRD §9.22)**: the EDITMSG round-trip — write message + commented summary (tree SHA, diff-tree
  --name-status) to `.git/STAGECOACH_EDITMSG`; open the resolved editor; strip comments + trailing
  whitespace; empty → exit 1 abort (intentional, NOT a rescue).
- **FR-E2**: editing-while-staging stays safe — the snapshot is frozen before the editor opens (the docs
  MUST call this out; it's the §13.4 property extended through the editor).
- **FR-E3**: the edited message is user-authored — bypasses the duplicate re-check; the template (FR-F8)
  is applied BEFORE the editor opens so the user edits final text.
- **FR-E4**: composition — decompose gates EACH commit; `--dry-run` → warn + ignore; `hook exec` → usage error.
- **architecture/external_deps.md §6 (VERIFIED)**: editor resolution via `git var GIT_EDITOR` (the exact
  GIT_EDITOR → core.editor → VISUAL → EDITOR → vi chain, a faithful superset of FR-E1); the value is
  shell-interpreted → invoke via `sh -c '<editor> "$@"' -- <file>`.
- **Scope fences**: CONSUMES P1.M2.T2.S2's `FinalizeMessage` seam + `cfg.Template` (template-before-editor
  ordering, FR-E3 — the seam's godoc already publishes this contract). Does NOT touch the prompt package,
  chain.go (arbiter reuses generateMessage → transitively covered), hook install, or any v1 generation
  logic. The non-interactive default path is untouched (cfg.Edit gates everything).

## What

A flag-only `--edit` (default false) that inserts an editor round-trip at the message-finalization seam.
The editor is a POST-dedupe, PRE-publish stage (FR-E3: the user's message bypasses the re-check; FR-F8:
the template was already applied pre-dedupe by FinalizeMessage, so the editor shows final text).

### Success Criteria

- [ ] `internal/generate/finalize.go`: `EditMessage(msg, cfg, editCtx) (string, error)` (no-op when
      `cfg.Edit == false`); `EditContext{Git git.Git; TreeSHA, NameStatus string}`; `ErrEmptyMessage`
      sentinel; the write→editor→strip→validate round-trip.
- [ ] `internal/git/git.go`: `GitDir` / `Editor` / `DiffTreeNameStatus` added to the `Git` interface
      + implemented on `*gitRunner` (read-only w.r.t. refs/index; `--absolute-git-dir`, `git var GIT_EDITOR`,
      `git diff-tree --no-commit-id --name-status -r`).
- [ ] `Config.Edit bool \`toml:"-"\`` + `Defaults()` `Edit: false`; `loadFlags` reads `--edit`;
      `root.go` registers the global persistent `--edit` bool flag.
- [ ] 4 `EditMessage` call sites: generate.go (post-loop, pre-CommitTree), stagecoach.go runPipeline,
      decompose/message.go (post-loop, pre-`return msg`), decompose runSingleShortcut (post-dup-check, pre-publishCommit).
- [ ] `--dry-run` + `--edit` → stderr warning, editor NOT opened (FR-E4).
- [ ] `hook exec` + `--edit` (inherited persistent flag) → usage error (FR-E4).
- [ ] `ErrEmptyMessage` → exit 1 "empty commit message — aborted" (NOT rescue; HEAD+index untouched).
- [ ] docs/cli.md `--edit` row; docs/how-it-works.md FR-E2 call-out.
- [ ] Fake-editor tests (rewrite/empty/leave) across single + decompose; the 3 git methods; strip semantics.
- [ ] Full build/test/vet/lint/fmt green; `--edit` unset → byte-identical to today.

## All Needed Context

### Context Completeness Check

_This PRP names the EXACT post-dedupe/pre-publish insertion points (with the surrounding anchor lines),
the 3 new `git.Git` methods (with the exact git subcommands + the read-only convention), the
`EditMessage` signature + the `EditContext` fields + the `ErrEmptyMessage` sentinel, the EDITMSG file
format + the strip semantics, the editor invocation (`sh -c '<editor> "$@"' -- <file>`, stdin/out/err =
os.Std*), the abort-is-not-rescue proof, the FR-E3 dedupe-bypass ordering proof (why the editor is
post-dedupe while the template is pre-dedupe), the FR-E2 freeze proof (no code needed — invariant holds),
the dry-run + hook-exec guards, the flag-only config plumbing (mirrors `Context`), the exit-code mapping
spot, the docs rows, and the fake-editor test pattern. An implementer with no prior codebase knowledge
can build it from this document + codebase access._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/P1M5T1S1/research/editor-gate-design.md
  why: THE design playbook — the dedupe-vs-editor ordering proof (§6: why EditMessage is a SEPARATE
       post-dedupe stage, NOT inside FinalizeMessage — FR-E3 requires the user message bypass dedupe,
       while FR-F8 requires the template be visible to dedupe; these are CONTRADICTORY if both are in
       one seam), the 5-site coverage table + the transitive arbiter proof, the EDITMSG round-trip,
       the abort-is-not-rescue invariant, the strip semantics (git parity), the fake-editor test pattern,
       and the FR-E4 composition (dry-run warn, hook-exec usage error).
  section: all
  critical: |
    The editor is POST-dedupe (FR-E3: user message bypasses re-check); the template is PRE-dedupe
    (FR-F8: dedupe sees the templated subject). These MUST stay on opposite sides of the dedupe check.
    Do NOT fold the editor into FinalizeMessage (it's pre-dedupe). EditMessage is a sibling stage, called
    AFTER the loop's accept+break, BEFORE publication.

- docfile: plan/005_c38aa48290f0/architecture/external_deps.md
  why: §6 (Editor resolution, VERIFIED) — `git var GIT_EDITOR` performs the exact resolution
       (GIT_EDITOR → core.editor → VISUAL → EDITOR → vi); the value is shell-interpreted (may contain
       args/quotes) → invoke via `sh -c '<editor> "$@"' -- <file>`, NEVER a bare exec. This SUPERSETS
       FR-E1's chain faithfully (it adds `core.editor`, which FR-E1's prose omits).
  section: "## 6. Editor resolution (gates FR-E1) — VERIFIED (git-var docs)"
  critical: |
    `git var GIT_EDITOR` is the recommended implementation (the PRP item's RESEARCH NOTE mandates it).
    The returned string is shell-interpreted — always wrap in `sh -c '<editor> "$@"' -- <file>`. A bare
    `exec.Command(editor, file)` BREAKS on editors with args (e.g. `code --wait`, `vim -f`).

- docfile: plan/005_c38aa48290f0/P1M2T2S2/PRP.md   (and the COMPLETED source internal/generate/finalize.go)
  why: THE upstream seam contract. FinalizeMessage(msg, cfg) is the PRE-dedupe template stage (FR-F8);
       its godoc publishes the ORDERING CONTRACT ("template → (future) editor → publish; keep template
       first") — this subtask IS the "(future) editor" stage, slotted AFTER FinalizeMessage and AFTER
       dedupe (see editor-gate-design.md §6 for why post-dedupe). cfg.Template is applied by FinalizeMessage
       before EditMessage runs, so the editor sees the templated (final) message (FR-E3).
  section: "Data models; Implementation Tasks (the seam + the 4 call sites)"
  critical: |
    FinalizeMessage is UNCHANGED (keep it pre-dedupe). EditMessage is a NEW sibling. The 4 EditMessage
    call sites are at the SAME 4 files as the FinalizeMessage sites, but a few lines LATER (after the
    dedupe break, before publish). Do NOT modify FinalizeMessage or its 4 call sites.

- docfile: plan/005_c38aa48290f0/prd_snapshot.md   (and PRD.md §9.22 / §13.3-§13.4)
  why: §9.22 FR-E1 (the EDITMSG round-trip + abort), FR-E2 (edit-while-staging safe — docs duty),
       FR-E3 (bypass dedupe + template-before-editor), FR-E4 (decompose-each-commit + dry-run + hook-exec);
       §13.3 (the snapshot invariants — why the abort leaves HEAD+index untouched); §13.4 (the
       stage-while-generating property FR-E2 extends).
  section: "§9.22 (FR-E1/E2/E3/E4), §13.3, §13.4"

- file: internal/generate/finalize.go   (COMPLETED — P1.M2.T2.S2)
  why: THE file to EXTEND with EditMessage + EditContext + ErrEmptyMessage (co-located with ApplyTemplate/
       FinalizeMessage — same package, same pure-function + FROZEN-signature doc style). FinalizeMessage
       stays pre-dedupe (unchanged); EditMessage is the new post-dedupe sibling. The ordering contract is
       published here.
  pattern: |
    // EditMessage is the §9.22 FR-E1 editor gate. cfg.Edit==false ⇒ identity (the default; byte-identical
    // to the pre-feature path). When true: write msg + a commented summary to <gitDir>/STAGECOACH_EDITMSG,
    // open the editor (git var GIT_EDITOR via sh -c), strip comment lines + trailing whitespace, return
    // the edited message. Empty result ⇒ ErrEmptyMessage (caller aborts: exit 1, NOT a rescue).
    // POST-dedupe (FR-E3: user message bypasses the re-check) and PRE-publish (CommitTree/publishCommit).
    func EditMessage(ctx context.Context, msg string, cfg config.Config, editCtx EditContext) (string, error) {
        if !cfg.Edit { return msg, nil }   // THE no-op guard — the regression invariant
        ...
    }

- file: internal/generate/generate.go   (EDIT — CALL SITE #1)
  why: THE single-commit loop. The accept is L~248 `msg = m; success = true; break`; publication is step 7
       `CommitTree` L~255. Insert EditMessage BETWEEN them, after the loop, before CommitTree. treeSHA is
       in scope (step 3, L~WritTree); parentSHA + isUnborn too. The name-status needs the parent's TREE
       (not SHA) — capture via `deps.Git.RevParseTree(ctx, "HEAD")` (or EmptyTreeSHA if isUnborn) before
       the loop, then `deps.Git.DiffTreeNameStatus(ctx, parentTree, treeSHA)`.
  pattern: |
    // AFTER the loop (success==true), BEFORE step 7 CommitTree:
    parentTree := git.EmptyTreeSHA
    if !isUnborn {
        if pt, perr := deps.Git.RevParseTree(ctx, "HEAD"); perr == nil { parentTree = pt }
    }
    nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, parentTree, treeSHA) // best-effort; "" on err
    msg, err = EditMessage(ctx, msg, cfg, EditContext{Git: deps.Git, TreeSHA: treeSHA, NameStatus: nameStatus})
    if err != nil { return Result{}, err }  // ErrEmptyMessage propagates → CLI exit 1
    // Step 7: commit-tree ...
  gotcha: |
    ErrEmptyMessage must propagate as a PLAIN error (NOT wrapped in *RescueError — it's an abort, not a
    rescue). Returning it bare from EditMessage → `return Result{}, err` surfaces it to exitcode.For(),
    which maps it to exit 1. Do NOT call signal.SetSnapshot rescue disposition for it (no TREE_SHA recipe
    — the user INTENDED the abort). The snapshot already exists (treeSHA) but is harmless (gc'd).

- file: pkg/stagecoach/stagecoach.go   (EDIT — CALL SITE #2, runPipeline)
  why: THE public-API copy of the loop. Same accept→publish structure as generate.go. Mirror the
       EditMessage insertion exactly (it's the public surface — must match the CLI behavior). cfg is `cfg`
       (config.Config); the Git boundary is the runPipeline's deps.Git. Insert after the loop break,
       before CommitTree.
  pattern: "identical to generate.go #1 — EditMessage(ctx, msg, cfg, EditContext{...}) before CommitTree."
  gotcha: "Qualify as generate.EditMessage (different package). The public API must expose --edit parity."

- file: internal/decompose/message.go   (EDIT — CALL SITE #3, generateMessage)
  why: THE decompose message-role loop. The accept is L~172 `msg = m; success = true; break`; the func
       returns `msg, nil` at L~181. Insert EditMessage BETWEEN break and return. treeB (the concept tree)
       + treeA are in scope; DiffTreeNameStatus(treeA, treeB) is the summary. This site ALSO covers the
       arbiter N+1 (chain.go resolveNewCommit reuses generateMessage) — transitively.
  pattern: |
    // AFTER the loop, BEFORE `return msg, nil`:
    nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, treeA, treeB)
    msg, err = EditMessage(ctx, msg, deps.Config, EditContext{Git: deps.Git, TreeSHA: treeB, NameStatus: nameStatus})
    if err != nil { return "", err }  // ErrEmptyMessage → propagates to runLoop's FR-M12 handling
  gotcha: |
    ErrEmptyMessage in decompose: it's a per-concept ABORT (FR-E4). runLoop (decompose.go) treats a
    returned error from generateMessage as a concept-i failure → FR-M12 isolation (already-landed commits
    stand; the run aborts). Do NOT special-case it — the existing error propagation is correct. The CLI
    exit mapping (exitcode.For) maps ErrEmptyMessage → exit 1 for the whole run.

- file: internal/decompose/decompose.go   (EDIT — CALL SITE #4, runSingleShortcut)
  why: THE FR-M11 shortcut. The dup-check block is L~323-329; publishCommit is L~335. Insert EditMessage
       BETWEEN them (after the message is finalized + dup-checked, before publishCommit). treePrime ==
       tStart (the frozen tree) is in scope; baseTree is the name-status base.
  pattern: |
    // AFTER the dup-check block, BEFORE publishCommit:
    nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, baseTree, treePrime)
    msg, err = EditMessage(ctx, msg, deps.Config, EditContext{Git: deps.Git, TreeSHA: treePrime, NameStatus: nameStatus})
    if err != nil { return DecomposeResult{}, err }
  gotcha: "treePrime == tStart (FR-M1b frozen). The editor runs on the frozen snapshot — staging the next
           batch during the edit is safe (FR-E2)."

- file: internal/decompose/chain.go   (READ ONLY — arbiter N+1, NO EDIT)
  why: PROOF of transitive coverage. resolveNewCommit (L78) calls generateMessage (L109) → call site #3
       covers it. Do NOT add EditMessage here (it would run TWICE — once in generateMessage, once here).
  gotcha: "Adding EditMessage to resolveNewCommit double-edits. The arbiter N+1 is covered by #3."

- file: internal/git/git.go   (EDIT — the Git interface + *gitRunner impl)
  why: ADD 3 methods to the interface + impl. (1) GitDir: `git rev-parse --absolute-git-dir` → the .git
       dir (for STAGECOACH_EDITMSG path). Exits 128 on non-repo (real error — mirror HooksPath's convention,
       NOT RevParseHEAD's 128-as-unborn). (2) Editor: `git var GIT_EDITOR` → the resolved editor string
       (shell-interpreted). (3) DiffTreeNameStatus: `git diff-tree --no-commit-id --name-status -r <treeA>
       <treeB>` → the raw A/M/D lines for the EDITMSG summary. All read-only w.r.t. refs/index.
  pattern: |
    // GitDir:
    func (g *gitRunner) GitDir(ctx context.Context) (string, error) {
        stdout, stderr, code, err := g.run(ctx, g.workDir, "rev-parse", "--absolute-git-dir")
        if err != nil { return "", err }
        if code != 0 { return "", fmt.Errorf("git rev-parse --absolute-git-dir: failed (exit %d): %s", code, strings.TrimSpace(stderr)) }
        return strings.TrimSpace(stdout), nil
    }
    // Editor:
    func (g *gitRunner) Editor(ctx context.Context) (string, error) {
        stdout, stderr, code, err := g.run(ctx, g.workDir, "var", "GIT_EDITOR")
        if err != nil { return "", err }
        if code != 0 { return "", fmt.Errorf("git var GIT_EDITOR: failed (exit %d): %s", code, strings.TrimSpace(stderr)) }
        return strings.TrimSpace(stdout), nil
    }
    // DiffTreeNameStatus:
    func (g *gitRunner) DiffTreeNameStatus(ctx context.Context, treeA, treeB string) (string, error) {
        stdout, stderr, code, err := g.run(ctx, g.workDir, "diff-tree", "--no-commit-id", "--name-status", "-r", treeA, treeB)
        if err != nil { return "", err }
        if code != 0 { return "", fmt.Errorf("git diff-tree --name-status: failed (exit %d): %s", code, strings.TrimSpace(stderr)) }
        return stdout, nil  // raw A/M/D lines; caller prefixes with "# " for the EDITMSG
    }
  gotcha: |
    Add to the Git INTERFACE (the type, L~58) AND to *gitRunner (impl). EVERY existing test double of Git
    (search `git.Git` in *_test.go — stubtest, fakeGit, etc.) must gain no-op/stub implementations of the
    3 methods or the package won't compile. Use `go build ./...` after the interface edit to find them all.
    `--absolute-git-dir` is git 2.13+ (2017) — universally available. `git var GIT_EDITOR` honors the
    dumb-terminal edge case + repo-local core.editor (external_deps.md §6).

- file: internal/config/config.go   (EDIT — Config.Edit field)
  why: ADD `Edit bool \`toml:"-"\`` (flag-only, like Context L~108). toml:"-" = NOT a config-file key
       (FR-E1: "flag only"). Add to Defaults(): `Edit: false`.
  pattern: |
    // Edit is the §9.22 FR-E1 --edit flag (flag-only: no env, no git key, no config-file key). When true,
    // an editor round-trip gates each commit message before publication (post-dedupe, pre-CommitTree).
    // Default false (non-interactive). See generate.EditMessage.
    Edit bool `toml:"-"`
    // ...in Defaults(): Edit: false,
  gotcha: "Mirror Context (flag-only, toml:\"-\") — NOT Template (full precedence). FR-E1 says 'flag only'."

- file: internal/config/load.go   (EDIT — loadFlags reads --edit)
  why: loadFlags (L~264) reads Config-backed flags. Add an `--edit` block (BoolVar, like --verbose/--no-color).
  pattern: |
    if fs.Changed("edit") {
        if v, err := fs.GetBool("edit"); err == nil { cfg.Edit = v }
    }
  gotcha: "Bool DIRECT set (can be false as escape hatch). Place near the --verbose/--no-color block."

- file: internal/cmd/root.go   (EDIT — register --edit global persistent flag)
  why: flagFormat/flagTemplate/flagContext are at L~74-81; their registrations follow. Add `flagEdit bool`
       + a BoolVar. flagDryRun is at L~64/129 (precedent for a global bool flag).
  pattern: |
    // near L74-81: add `flagEdit bool`
    // after the --context StringVar:
    pf.BoolVar(&flagEdit, "edit", false,
        "Open your editor on the generated message before committing (git var GIT_EDITOR). "+
            "An empty message aborts (exit 1). The edited message bypasses the duplicate check. "+
            "In decompose mode each commit is gated. Ignored with --dry-run; not valid with hook exec. "+
            "(§9.22 FR-E1)")
  gotcha: "GLOBAL persistent flag — inherited by subcommands. hook exec must REJECT it (FR-E4) — see hookexec.go edit."

- file: internal/cmd/hookexec.go   (EDIT — reject --edit on hook exec, FR-E4)
  why: `--edit` is meaningless on hook exec (git already opens the editor for the commit). FR-E4: rejected
       as a usage error. Check at the top of runHookExec (after arg parsing, before config load).
  pattern: |
    if cmd.Flags().Changed("edit") {
        fmt.Fprintf(stderr, "stagecoach: --edit is not valid with hook exec (git already opens the editor)\n")
        return exitcode.New(exitcode.Error, nil) // silent non-zero (already printed) — or exitcode.Usage if defined
    }
  gotcha: "There is no exitcode.Usage constant (see exitcode.go) — use exitcode.Error (exit 1). Print the
           message ourselves (SilenceUsage is true on hookExecCmd). Match the neverBlock pattern's stderr style."

- file: internal/cmd/default_action.go   (EDIT — dry-run + --edit warn-and-skip, FR-E4)
  why: `--dry-run` + `--edit` → warn + ignore (FR-E4). default_action.go owns the dry-run path (flagDryRun).
       Simplest: if flagDryRun && cfg.Edit, print a warning to stderr and set cfg.Edit=false for the run
       (so EditMessage is a no-op). Do this right after config load, before the generation dispatch.
  pattern: |
    if flagDryRun && cfg.Edit {
        fmt.Fprintln(cmd.ErrOrStderr(), "stagecoach: --edit ignored in --dry-run mode (nothing to commit)")
        cfg.Edit = false
    }
  gotcha: "Mutate the LOCAL cfg copy (Load returns *cfg; either mutate *cfg or copy). The warning is stderr
           (FR51 — progress to stderr). The dry-run path returns before publication, so EditMessage is
           naturally unreached — but the explicit guard is belt-and-suspenders + the user-facing warning is required."

- file: internal/exitcode/exitcode.go   (EDIT — map ErrEmptyMessage → exit 1)
  why: For() (L~50) is the single exit-code source. Add `errors.Is(err, generate.ErrEmptyMessage) → Error`
       (exit 1) BEFORE the generic default. It's NOT Rescue (exit 3) — the user intended the abort; no
       manual-recovery recipe is needed (HEAD+index untouched).
  pattern: |
    if errors.Is(err, generate.ErrNothingToCommit) { return NothingToCommit }
    if errors.Is(err, generate.ErrEmptyMessage) { return Error }   // §9.22 FR-E1 abort — exit 1, not rescue
    // ...existing CAS/Rescue/Timeout mapping...
  gotcha: |
    The CLI's printRescueOrCAS (default_action.go ~L190) handles *RescueError/*CASError with SILENT exitcode
    (already-printed). ErrEmptyMessage is NEITHER — it should print "empty commit message — aborted" to
    stderr. Simplest: in For() return exitcode.Error; the message is the err.Error() ("stagecoach: empty
    commit message — aborted"). The default_action.go generic path (`return exitcode.New(exitcode.Error, err)`)
    already prints err.Error() via main — verify ErrEmptyMessage is NOT caught by errors.As(*RescueError)
    first (it isn't — it's a plain sentinel). Add a small guard: the `printRescueOrCAS` switch should NOT
    match ErrEmptyMessage (it won't — it only matches *RescueError/*CASError). The generic tail prints it.

- file: internal/git/git_test.go   (REF — git test double convention)
  why: The git test harness (initTempRepo pattern). Mirror for the 3 new methods' tests. ALSO: search for
       every fake/stub Git in the repo (stubtest, fakeGit, etc.) — they must gain the 3 methods or fail to
       compile after the interface edit. `go build ./...` after editing the interface surfaces them all.
  pattern: "initTempRepo(t) → (repo, git.Git); call the method; assert."

- file: internal/generate/generate_test.go   (REF — stub manifest + git test harness)
  why: The fake-editor test pattern. stubtest.Manifest is the stub provider; the Git is real (git.New on a
       t.TempDir() repo). To inject a fake editor, the test Git must return the fake-editor script path from
       Editor() — but generate.go uses deps.Git (real). OPTION: add a `generate.EditDeps` override (a func
       field) OR set `GIT_EDITOR=<script>` env in the test (git var GIT_EDITOR reads it). The ENV route is
       simplest + most faithful: `t.Setenv("GIT_EDITOR", scriptPath)` — git var resolves it, no Git stub needed.
  pattern: |
    // fake editor script:
    script := filepath.Join(t.TempDir(), "fakeeditor.sh")
    os.WriteFile(script, []byte("#!/bin/sh\necho 'edited subject' > \"$1\"\n"), 0755)
    t.Setenv("GIT_EDITOR", script)  // git var GIT_EDITOR resolves this — no Git stub needed
    // ...run CommitStaged with cfg.Edit=true; assert Result.Message == "edited subject"
  gotcha: "t.Setenv auto-restores. The fake script MUST be executable (0755) + have a shebang. For the
           'empty' case: `#!/bin/sh\n: > \"$1\"` (truncate). For 'leave': `#!/bin/sh\nexit 0` (untouched)."

- file: internal/generate/finalize_test.go   (REF — pure-function test style for finalize.go)
  why: EditMessage's strip-semantics unit test (no Git/editor — test the pure strip function directly):
       comment-line removal, trailing-whitespace trim, empty-after-strip → ErrEmptyMessage. Mirror the
       ApplyTemplate table style.
  pattern: "split on '\\n'; drop lines with HasPrefix('#'); TrimSpace each; drop empty; join."

- file: docs/cli.md   (EDIT — Mode A, --edit row)
  why: Global-flags table. --format(L38)/--locale(L39)/--template row → add --edit after it. Note abort
       semantics, dedupe-bypass, decompose-each-commit, dry-run-ignore, hook-exec-reject.
  critical: |
    | `--edit` | bool | false | (flag-only) | (none) | Open your editor ($GIT_EDITOR via `git var`) on the
      generated message before committing. The EDITMSG includes the tree SHA + a diff-tree name-status
      summary; comment lines (`#`) are stripped on close. An empty result aborts (exit 1, not a rescue).
      The edited message bypasses the duplicate check (git parity). In decompose mode each commit is gated.
      Ignored with `--dry-run`; not valid with `hook exec`. (§9.22 FR-E1) |

- file: docs/how-it-works.md   (EDIT — Mode A, FR-E2 call-out, REQUIRED)
  why: FR-E2 EXPLICITLY requires the docs call-out: "the snapshot is frozen before the editor opens, so the
       user can stage the next batch during the edit session." Add a short paragraph in the stage-while-
       generating section (§13.4 equivalent) noting `--edit` extends the property through the editor.
  critical: "This is the ONE thing `git commit -e`-style flows cannot offer on top of generation — call it out."

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  finalize.go      # EXTEND — EditMessage + EditContext + ErrEmptyMessage (beside ApplyTemplate/FinalizeMessage)
  generate.go      # EDIT — CALL SITE #1 (post-loop, pre-CommitTree step 7)
  finalize_test.go # EXTEND — strip-semantics table + EditMessage no-op-when-unset
  generate_test.go # EXTEND — fake-editor integration (rewrite/empty/leave) via t.Setenv("GIT_EDITOR",...)
internal/git/
  git.go           # EDIT — Git interface + *gitRunner: +GitDir +Editor +DiffTreeNameStatus
  git_test.go      # EXTEND — the 3 new methods (initTempRepo harness)
internal/decompose/
  message.go       # EDIT — CALL SITE #3 (post-loop, pre-`return msg`)
  decompose.go     # EDIT — CALL SITE #4 (runSingleShortcut, post-dup-check, pre-publishCommit)
  chain.go         # UNCHANGED (arbiter N+1 reuses generateMessage — transitively covered)
internal/config/
  config.go        # EDIT — Config.Edit field + Defaults()
  load.go          # EDIT — loadFlags reads --edit
internal/cmd/
  root.go          # EDIT — flagEdit + --edit BoolVar (global persistent)
  default_action.go# EDIT — dry-run+--edit warn-and-skip
  hookexec.go      # EDIT — reject --edit on hook exec (FR-E4)
internal/exitcode/
  exitcode.go      # EDIT — For() maps ErrEmptyMessage → Error (exit 1)
pkg/stagecoach/stagecoach.go # EDIT — CALL SITE #2 (runPipeline, post-loop, pre-CommitTree)
docs/
  cli.md           # EDIT — --edit global-flags row
  how-it-works.md  # EDIT — FR-E2 edit-while-staging call-out (REQUIRED)
```

### Desired Codebase tree with files to be added and responsibility of file

No NEW files. All work is EXTENSIONS to existing files. `internal/generate/finalize.go` gains
`EditMessage`/`EditContext`/`ErrEmptyMessage` (co-located with the template seam). The 3 `git.Git`
methods are added to the existing interface + impl. Every change is additive + guarded by `cfg.Edit`.

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the editor is POST-dedupe, the template is PRE-dedupe — do NOT merge them): FR-E3 requires
// the user's edited message BYPASS the duplicate re-check (git parity); FR-F8 requires the template be
// VISIBLE to the dedupe check. These are CONTRADICTORY if both stages are in one seam. FinalizeMessage
// (P1.M2.T2.S2) is pre-dedupe (template); EditMessage (this subtask) is post-dedupe (editor). They are
// SIBLINGS, not nested. Do NOT call EditMessage inside FinalizeMessage. The 4 EditMessage sites are a few
// lines AFTER the 4 FinalizeMessage sites (after the dedupe break, before publish).

// CRITICAL (byte-identity): cfg.Edit==false (the default) MUST reproduce today's bytes in EVERY path.
// EditMessage short-circuits `if !cfg.Edit { return msg, nil }` as its FIRST line. All existing
// generate/decompose/stagecoach tests pass UNCHANGED (they never set cfg.Edit) — that is the regression guard.

// CRITICAL (abort is NOT a rescue): ErrEmptyMessage is a PLAIN sentinel (not *RescueError). On the editor
// returning an empty message, EditMessage returns ErrEmptyMessage; the loops `return Result{}, err` /
// `return "", err` propagate it BARE; exitcode.For() maps it to exit 1 (NOT 3/124). No rescue recipe is
// printed (HEAD+index untouched — the user intended the abort). The orphan treeSHA is gc'd eventually
// (harmless, same as any failed generation). Do NOT arm the signal rescue disposition for it.

// CRITICAL (editor invocation via sh -c, NEVER bare exec): the value from `git var GIT_EDITOR` is
// shell-interpreted (may be `code --wait`, `vim -f`, `nano`, an alias). Invoke via
// `exec.CommandContext(ctx, "sh", "-c", editor+" \"$@\"", "--", editMsgPath)` with cmd.Stdin/Out/Err =
// os.Stdin/Out/Err (interactive). A bare `exec.Command(editor, editMsgPath)` BREAKS on editors with args.
// external_deps.md §6 (VERIFIED).

// CRITICAL (editor resolution is best-effort, never fatal): if `git var GIT_EDITOR` fails (rare), fall
// back to os.Getenv("VISUAL") → os.Getenv("EDITOR") → "vi". If the editor subprocess exits non-zero,
// return a wrapped error (NOT ErrEmptyMessage — that's only for an empty RESULT). The CLI maps a
// non-zero editor exit to exit 1 (generic error). Do NOT silently commit an unedited message on editor
// failure (the user may have aborted via `:cq` in vim — treat non-zero as abort).

// CRITICAL (the Git interface edit ripples to ALL test doubles): adding GitDir/Editor/DiffTreeNameStatus
// to git.Git means EVERY fake/stub Git in the repo (search `git.Git` in *_test.go: stubtest, internal/git
// fakes, decompose/stagecoach test doubles) must gain the 3 methods or the package won't compile. Run
// `go build ./...` immediately after the interface edit to enumerate them; add no-op/stub impls.

// CRITICAL (vim's `:cq` abort): vim exits non-zero on `:cq` (quit-with-error). Treat ANY non-zero editor
// exit as an abort (return an error, do NOT commit). This is the standard git behavior (`git commit -e`
// aborts on `:cq`). Match it. Do NOT treat non-zero editor exit as ErrEmptyMessage (different cause).

// CRITICAL (the EDITMSG path is <gitDir>/STAGECOACH_EDITMSG, NOT a temp file): use `git rev-parse
// --absolute-git-dir` (honors worktrees + commondir). Write 0644. Do NOT use t.TempDir() for the EDITMSG
// in PRODUCTION (only tests may redirect). The file is in .git/ so it's repo-local + cleaned by git gc
// eventually (git's own COMMIT_EDITMSG lives there too).

// GOTCHA (name-status summary is best-effort): if DiffTreeNameStatus errors (rare — bad tree), pass "" —
// the EDITMSG shows just the message + the Tree: comment. Do NOT fail the run on a summary error (the
// editor gate's purpose is the MESSAGE edit; the summary is a nicety). Capture the error at the call site
// with `nameStatus, _ := ...`.

// GOTCHA (strip semantics — git parity): git strips lines beginning with the comment char (`#` default)
// + trailing whitespace per line. Replicate: split on "\n"; for each line, if strings.HasPrefix(line, "#")
// drop it; else strings.TrimSpace(line) and keep if non-empty; join with "\n". A message that is ALL
// comments/whitespace → "" → ErrEmptyMessage. Do NOT strip inline comments within a kept line (git doesn't).

// GOTCHA (--edit is flag-only, NOT full precedence): mirror Context (P1.M2.T2.S1), NOT Template
// (P1.M2.T2.S2). `toml:"-"` = not a config-file key. No env var, no git key. FR-E1: "flag only".

// GOTCHA (dry-run guard mutates the LOCAL cfg): default_action.go loads *cfg; if flagDryRun && cfg.Edit,
// print the warning + set cfg.Edit=false on the local copy. The dry-run path returns before publication,
// so EditMessage is unreached regardless — but the explicit guard + warning are required (FR-E4).

// GOTCHA (hook exec persistent-flag inheritance): --edit is a GLOBAL persistent flag (root.go), so it's
// INHERITED by hook exec. hookexec.go MUST explicitly reject it (FR-E4) — check cmd.Flags().Changed("edit")
// at the top of runHookExec. cobra does NOT auto-reject inherited persistent flags on a subcommand.

// GOTCHA (the arbiter N+1 is transitively covered — do NOT edit chain.go): resolveNewCommit (chain.go L78)
// calls generateMessage (L109) which has call site #3. Adding EditMessage to resolveNewCommit would run it
// TWICE. The transitive-coverage argument from P1.M2.T2.S2 applies identically here.
```

## Implementation Blueprint

### Data models and structure

```go
// === internal/generate/finalize.go (EXTEND — add after FinalizeMessage) ===
package generate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
)

// ErrEmptyMessage is the §9.22 FR-E1 abort signal: the editor returned an empty message (after stripping
// comments + whitespace). It is an INTENTIONAL abort, NOT a rescue — HEAD and the index are untouched
// (the editor runs after WriteTree but before CommitTree; the orphan tree is gc'd eventually). The CLI
// maps it to exit 1 with "empty commit message — aborted" (NOT exit 3/124 — no manual-recovery recipe).
var ErrEmptyMessage = errors.New("stagecoach: empty commit message — aborted")

// EditContext carries the snapshot + git boundary the editor gate needs to build the EDITMSG summary
// (§9.22 FR-E1: "the message plus a commented summary (tree SHA, diff-tree --name-status of the snapshot)").
// TreeSHA is always the frozen snapshot tree; NameStatus is the raw `git diff-tree --name-status -r` output
// (best-effort — "" if unavailable). Git resolves the .git dir + the editor command.
type EditContext struct {
	Git        git.Git // the git boundary (for GitDir + Editor)
	TreeSHA    string  // the frozen snapshot tree (treeB in decompose; treeSHA in single)
	NameStatus string  // raw A/M/D lines for the summary (best-effort; "" if unavailable)
}

// EditMessage is the §9.22 FR-E1 editor gate — a POST-dedupe, PRE-publish stage. cfg.Edit==false ⇒
// identity (the default; byte-identical to the pre-feature path). When true: write msg + a commented
// summary to <gitDir>/STAGECOACH_EDITMSG, open the resolved editor (`git var GIT_EDITOR` via sh -c), strip
// comment lines + trailing whitespace on close, return the edited message.
//
// An empty result (after strip) ⇒ ErrEmptyMessage (caller aborts: exit 1, NOT a rescue). A non-zero editor
// exit (e.g. vim's `:cq`) ⇒ a wrapped error (treated as an abort, NOT committed).
//
// ORDERING (FR-E3 + FR-F8): EditMessage runs AFTER generate.FinalizeMessage (which applies the template,
// pre-dedupe) and AFTER the dedupe check (so the user's hand-written message bypasses the re-check — git
// parity). The template was applied before, so the user edits the FINAL text.
func EditMessage(ctx context.Context, msg string, cfg config.Config, editCtx EditContext) (string, error) {
	if !cfg.Edit {
		return msg, nil // THE no-op guard — the byte-identity regression invariant
	}

	// 1. Resolve the .git dir + the EDITMSG path.
	gitDir, err := editCtx.Git.GitDir(ctx)
	if err != nil {
		return "", fmt.Errorf("--edit: resolve git dir: %w", err)
	}
	editMsgPath := filepath.Join(gitDir, "STAGECOACH_EDITMSG")

	// 2. Build the EDITMSG content: message + commented summary.
	var buf strings.Builder
	buf.WriteString(msg)
	buf.WriteString("\n\n# Please edit this commit message. Lines starting with '#' will be removed,\n")
	buf.WriteString("# and trailing whitespace will be stripped. An empty message aborts the commit.\n")
	fmt.Fprintf(&buf, "#\n# Tree: %s\n", editCtx.TreeSHA)
	if editCtx.NameStatus != "" {
		buf.WriteString("# Changes:\n")
		for _, line := range strings.Split(strings.TrimRight(editCtx.NameStatus, "\n"), "\n") {
			fmt.Fprintf(&buf, "# %s\n", line)
		}
	}
	if err := os.WriteFile(editMsgPath, []byte(buf.String()), 0o644); err != nil {
		return "", fmt.Errorf("--edit: write %s: %w", editMsgPath, err)
	}

	// 3. Resolve the editor (git var GIT_EDITOR → VISUAL → EDITOR → vi). Best-effort fallback; never fatal.
	editor, err := editCtx.Git.Editor(ctx)
	if err != nil || editor == "" {
		editor = firstNonEmpty(os.Getenv("VISUAL"), os.Getenv("EDITOR"), "vi")
	}

	// 4. Invoke via sh -c (the value is shell-interpreted — may contain args). Interactive stdio.
	cmd := exec.CommandContext(ctx, "sh", "-c", editor+" \"$@\"", "--", editMsgPath)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("--edit: editor %q exited with error: %w", editor, err) // abort (e.g. vim :cq)
	}

	// 5. Read back + strip comment lines + trailing whitespace (git parity).
	raw, err := os.ReadFile(editMsgPath)
	if err != nil {
		return "", fmt.Errorf("--edit: read %s: %w", editMsgPath, err)
	}
	edited := stripCommentsAndTrim(string(raw))
	if edited == "" {
		return "", ErrEmptyMessage // §9.22 FR-E1 abort — exit 1, NOT a rescue
	}
	return edited, nil
}

// stripCommentsAndTrim removes lines beginning with '#', trims trailing whitespace per line, drops empty
// lines, and joins the survivors with '\n'. Mirrors git's prepare-commit-msg cleanup (comment-char '#').
func stripCommentsAndTrim(s string) string {
	var kept []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if t := strings.TrimRight(line, " \t\r"); t != "" {
			kept = append(kept, t)
		}
	}
	return strings.Join(kept, "\n")
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}
```

```go
// === internal/git/git.go (EXTEND the Git interface, ~L58, + *gitRunner impl, near HooksPath ~L1425) ===
// Add to the Git interface:
	// GitDir returns the absolute path to the repository's .git directory via `git rev-parse
	// --absolute-git-dir` (honors worktrees + commondir; git 2.13+, universally available). Used by the
	// --edit editor gate (§9.22 FR-E1) to locate .git/STAGECOACH_EDITMSG. `--absolute-git-dir` succeeds on
	// an UNBORN repo, so exit 128 here is a REAL error (non-repo/corrupt) — mirror HooksPath's convention
	// (branch on code != 0, NOT on code == 128). Read-only w.r.t. refs and the index (PRD §18.1).
	GitDir(ctx context.Context) (dir string, err error)

	// Editor returns the user's resolved editor command via `git var GIT_EDITOR` (the exact chain
	// GIT_EDITOR → core.editor → $VISUAL → $EDITOR → vi — external_deps.md §6, VERIFIED). The returned
	// string is SHELL-INTERPRETED (may contain args/quotes); callers invoke it via `sh -c '<editor> "$@"'
	// -- <file>`, NEVER a bare exec. Read-only w.r.t. refs and the index (PRD §18.1).
	Editor(ctx context.Context) (editor string, err error)

	// DiffTreeNameStatus returns the raw `git diff-tree --no-commit-id --name-status -r <treeA> <treeB>`
	// output — the A/M/D/<status>\t<path> lines for the --edit EDITMSG summary (§9.22 FR-E1). It is the
	// tree-to-tree name-status analogue of DiffTree (which diffs a commit vs its parent). Empty output
	// when treeA == treeB. Read-only w.r.t. refs and the index.
	DiffTreeNameStatus(ctx context.Context, treeA, treeB string) (nameStatus string, err error)

// Add to *gitRunner (impls — see the Documentation & References file: pattern block for the bodies).
```

```go
// === internal/config/config.go (EDIT — Config.Edit field, near Context L~108) ===
	// Edit is the §9.22 FR-E1 --edit flag (FLAG-ONLY: no env, no git key, no config-file key — mirrors
	// Context). When true, an editor round-trip gates each commit message before publication (post-dedupe,
	// pre-CommitTree). Default false (non-interactive). See generate.EditMessage.
	Edit bool `toml:"-"`
// ...in Defaults(): Edit: false,

// === internal/config/load.go (EDIT — loadFlags, near --verbose/--no-color block) ===
	if fs.Changed("edit") {
		if v, err := fs.GetBool("edit"); err == nil { cfg.Edit = v }
	}

// === internal/cmd/root.go (EDIT — flagEdit + BoolVar, near --context) ===
	var flagEdit bool   // (add to the flagFormat/flagContext var block)
	// after the --context StringVar:
	pf.BoolVar(&flagEdit, "edit", false,
		"Open your editor on the generated message before committing (resolved via `git var GIT_EDITOR`). "+
			"The message file includes the tree SHA + a diff-tree name-status summary; comment lines ('#') "+
			"are stripped on close. An empty message aborts (exit 1, not a rescue). The edited message "+
			"bypasses the duplicate check (git parity). In decompose mode each commit is gated. Ignored "+
			"with --dry-run; not valid with hook exec. (§9.22 FR-E1)")

// === internal/cmd/default_action.go (EDIT — dry-run + --edit warn-and-skip, after config load) ===
	if flagDryRun && cfg.Edit {
		fmt.Fprintln(cmd.ErrOrStderr(), "stagecoach: --edit ignored in --dry-run mode (nothing to commit)")
		cfg.Edit = false
	}

// === internal/cmd/hookexec.go (EDIT — reject --edit, top of runHookExec) ===
	if cmd.Flags().Changed("edit") {
		fmt.Fprintln(stderr, "stagecoach: --edit is not valid with hook exec (git already opens the editor)")
		return exitcode.New(exitcode.Error, nil)
	}

// === internal/exitcode/exitcode.go (EDIT — For(), after ErrNothingToCommit) ===
	if errors.Is(err, generate.ErrEmptyMessage) {
		return Error // §9.22 FR-E1 abort — exit 1, NOT rescue (no recipe; HEAD+index untouched)
	}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD the 3 git.Git methods (interface + *gitRunner impl)
  - EDIT internal/git/git.go: add GitDir/Editor/DiffTreeNameStatus to the Git interface (~L58, after
    HooksPath's doc) + implement on *gitRunner (near HooksPath ~L1425). USE the exact subcommands:
    `rev-parse --absolute-git-dir`, `var GIT_EDITOR`, `diff-tree --no-commit-id --name-status -r`.
    Read-only convention; branch on code != 0 (NOT 128) for GitDir (works on unborn — no 128-as-unborn).
  - RUN `go build ./...` immediately to enumerate EVERY fake/stub git.Git in the repo; add no-op/stub
    impls of the 3 methods to each (search: `git.Git` in *_test.go, stubtest, internal/git fakes).

Task 2: ADD Config.Edit + loadFlags + root.go --edit flag
  - EDIT internal/config/config.go: `Edit bool \`toml:"-"\`` after Context (L~108) + Defaults() Edit: false.
  - EDIT internal/config/load.go: loadFlags block (fs.Changed("edit") → cfg.Edit).
  - EDIT internal/cmd/root.go: flagEdit var + pf.BoolVar (after --context).

Task 3: ADD EditMessage + EditContext + ErrEmptyMessage to internal/generate/finalize.go
  - IMPLEMENT EditMessage (ctx, msg, cfg, editCtx) (the write→editor→strip→validate round-trip),
    EditContext{Git, TreeSHA, NameStatus}, ErrEmptyMessage, stripCommentsAndTrim, firstNonEmpty.
  - IMPORTS: add context, fmt, os, os/exec, path/filepath + errors (if not present) + git (internal/git).
    NOTE: finalize.go currently imports only strings + config — this adds the editor/exec deps.

Task 4: EDIT the 4 EditMessage call sites (post-dedupe, pre-publish)
  - internal/generate/generate.go: AFTER the loop (success==true), BEFORE step-7 CommitTree (~L248→L255).
    Capture parentTree (RevParseTree("HEAD") or EmptyTreeSHA if isUnborn), nameStatus
    (DiffTreeNameStatus(parentTree, treeSHA)), then `msg, err = EditMessage(ctx, msg, cfg, EditContext{...})`;
    `if err != nil { return Result{}, err }` (ErrEmptyMessage propagates BARE).
  - pkg/stagecoach/stagecoach.go: mirror generate.go #1 (runPipeline's post-loop, pre-CommitTree).
  - internal/decompose/message.go: AFTER the loop (success==true), BEFORE `return msg, nil` (~L172→L181).
    nameStatus = DiffTreeNameStatus(treeA, treeB); `msg, err = EditMessage(ctx, msg, deps.Config,
    EditContext{Git: deps.Git, TreeSHA: treeB, NameStatus: nameStatus})`; `if err != nil { return "", err }`.
  - internal/decompose/decompose.go (runSingleShortcut): AFTER the dup-check block (~L329), BEFORE
    publishCommit (~L335). nameStatus = DiffTreeNameStatus(baseTree, treePrime); `msg, err = EditMessage(...)`;
    `if err != nil { return DecomposeResult{}, err }`.
  - DO NOT edit chain.go (arbiter N+1 transitive via #3).

Task 5: EDIT exitcode.For() + default_action.go dry-run guard + hookexec.go reject
  - internal/exitcode/exitcode.go: add `errors.Is(err, generate.ErrEmptyMessage) → Error` after
    ErrNothingToCommit. IMPORTS: add generate (if not present).
  - internal/cmd/default_action.go: after config load, `if flagDryRun && cfg.Edit { warn + cfg.Edit=false }`.
  - internal/cmd/hookexec.go: top of runHookExec, `if cmd.Flags().Changed("edit") { warn + return
    exitcode.New(exitcode.Error, nil) }`.

Task 6: TESTS — strip semantics + fake-editor integration + the 3 git methods
  - internal/generate/finalize_test.go: stripCommentsAndTrim table (comments removed, trailing ws trimmed,
    empty-after-strip → ""); EditMessage no-op when cfg.Edit==false (byte-identity).
  - internal/generate/generate_test.go: fake-editor integration via `t.Setenv("GIT_EDITOR", script)` —
    rewrite (edited subject lands), empty (ErrEmptyMessage → exit 1, no commit), leave (original lands).
    Single-commit path (CommitStaged with cfg.Edit=true). Assert HEAD+index untouched on abort.
  - internal/decompose (message_test.go or decompose_test.go): same 3 fakes across generateMessage +
    runSingleShortcut. Assert per-concept gating + arbiter N+1 transitive coverage.
  - internal/git/git_test.go: GitDir (--absolute-git-dir, absolute path), Editor (git var GIT_EDITOR,
    honors GIT_EDITOR env), DiffTreeNameStatus (A/M/D lines, empty when trees equal).
  - internal/exitcode/exitcode_test.go: For() maps ErrEmptyMessage → Error (1), NOT Rescue (3).

Task 7: EDIT docs/cli.md + docs/how-it-works.md
  - docs/cli.md: --edit global-flags row (the Documentation & References critical block).
  - docs/how-it-works.md: FR-E2 paragraph — "With --edit, the snapshot is frozen before the editor opens,
    so you can stage the next batch in another pane during the edit — the same stage-while-generating
    property, extended through the editor. This is the one thing git commit -e-style flows cannot offer
    on top of generation." (REQUIRED — FR-E2 mandates the call-out.)
```

### Implementation Patterns & Key Details

```go
// generate.go call site #1 — the post-dedupe editor gate (unqualified; same package):
// AFTER `msg = m; success = true; break` and `if !success { return ...RescueError }`, BEFORE step-7 CommitTree:
parentTree := git.EmptyTreeSHA
if !isUnborn {
	if pt, perr := deps.Git.RevParseTree(ctx, "HEAD"); perr == nil {
		parentTree = pt
	}
}
nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, parentTree, treeSHA) // best-effort; "" on err
msg, err = EditMessage(ctx, msg, cfg, EditContext{Git: deps.Git, TreeSHA: treeSHA, NameStatus: nameStatus})
if err != nil {
	return Result{}, err // ErrEmptyMessage → exitcode.For() → exit 1 (NOT rescue)
}
// Step 7: commit-tree ...

// decompose/message.go call site #3 — post-loop, pre-return:
nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, treeA, treeB)
msg, err = EditMessage(ctx, msg, deps.Config, EditContext{Git: deps.Git, TreeSHA: treeB, NameStatus: nameStatus})
if err != nil {
	return "", err // ErrEmptyMessage → runLoop's FR-M12 abort (already-landed commits stand)
}

// The editor invocation (external_deps.md §6 — shell-interpreted, NEVER bare exec):
cmd := exec.CommandContext(ctx, "sh", "-c", editor+" \"$@\"", "--", editMsgPath)
cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
if err := cmd.Run(); err != nil {
	return "", fmt.Errorf("--edit: editor %q exited with error: %w", editor, err) // vim :cq → abort
}

// exitcode.For() — the abort mapping (NOT rescue):
if errors.Is(err, generate.ErrNothingToCommit) { return NothingToCommit }
if errors.Is(err, generate.ErrEmptyMessage) { return Error } // §9.22 FR-E1 — exit 1, no recipe
// ...existing CAS/Rescue/Timeout...
```

### Integration Points

```yaml
GIT INTERFACE (3 new read-only methods):
  - GitDir: `git rev-parse --absolute-git-dir` (the .git dir; honors worktrees/commondir).
  - Editor: `git var GIT_EDITOR` (the resolved editor; shell-interpreted → sh -c invocation).
  - DiffTreeNameStatus: `git diff-tree --no-commit-id --name-status -r <treeA> <treeB>` (the summary).
  - ALL existing fake/stub git.Git impls MUST gain the 3 methods (go build ./... enumerates them).

GENERATE PACKAGE (owns the editor gate):
  - internal/generate/finalize.go: EditMessage + EditContext + ErrEmptyMessage (beside FinalizeMessage).
  - EditMessage is a SIBLING of FinalizeMessage (post-dedupe; FinalizeMessage is pre-dedupe). NOT nested.

CALL SITES (4 — post-dedupe, pre-publish):
  - generate.go, pkg/stagecoach/stagecoach.go, decompose/message.go, decompose/decompose.go (runSingleShortcut).
  - transitive: arbiter N+1 (chain.go), one-file short-circuit, --single escape — covered via #3/#1 (NO edits).

CONFIG (flag-only, mirrors Context):
  - Config.Edit `toml:"-"` + Defaults() Edit: false + loadFlags --edit + root.go --edit BoolVar.

EXIT MAPPING:
  - exitcode.For(): ErrEmptyMessage → Error (exit 1, NOT rescue). No recipe printed.

GUARDS (FR-E4):
  - default_action.go: --dry-run + --edit → warn + cfg.Edit=false.
  - hookexec.go: --edit on hook exec → usage error (exit 1).

DOCS (Mode A):
  - docs/cli.md (--edit row), docs/how-it-works.md (FR-E2 edit-while-staging — REQUIRED call-out).

OUT OF SCOPE:
  - FinalizeMessage / its 4 call sites (P1.M2.T2.S2 — unchanged).
  - chain.go (arbiter N+1 transitive).
  - Prompt package, hook install, --push (P1.M5.T2.S1).
  - Any env/config/git precedence for --edit (flag-only per FR-E1).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/generate/finalize.go internal/git/git.go internal/config/config.go \
        internal/config/load.go internal/cmd/root.go internal/cmd/default_action.go \
        internal/cmd/hookexec.go internal/exitcode/exitcode.go internal/generate/generate.go \
        pkg/stagecoach/stagecoach.go internal/decompose/message.go internal/decompose/decompose.go
go build ./...   # the interface edit must compile across ALL fake/stub git.Git impls
go vet ./...
golangci-lint run
# Expected: zero errors. The FIRST `go build ./...` after the Git interface edit surfaces every test
# double needing the 3 new methods — fix them all before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/generate/... -v   # stripCommentsAndTrim + EditMessage no-op + fake-editor integration
go test ./internal/git/... -v        # GitDir + Editor + DiffTreeNameStatus
go test ./internal/config/... -v     # --edit flag plumbing (cfg.Edit set)
go test ./internal/exitcode/... -v   # ErrEmptyMessage → Error (1)
go test ./internal/decompose/... -v  # fake-editor across generateMessage + runSingleShortcut
go test ./pkg/stagecoach/... -v       # runPipeline EditMessage call site
# Expected: all pass. The byte-identity guard: every pre-existing test (cfg.Edit never set) passes UNCHANGED.
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach
# Flag exists + help:
/tmp/stagecoach --help 2>&1 | grep -A2 -- '--edit'
# Hook exec rejects --edit (FR-E4):
/tmp/stagecoach hook exec /tmp/msg 2>&1 | grep 'not valid with hook exec' || true   # (only if --edit passed)
# Dry-run + --edit warns (FR-E4):
/tmp/stagecoach --dry-run --edit 2>&1 | grep 'ignored in --dry-run mode' || true
# Manual smoke in a scratch repo (interactive — run by hand, not CI):
#   cd /tmp/scratch-repo && git add foo.txt && /tmp/stagecoach --edit
#   → editor opens → save → commit lands; empty the message → "empty commit message — aborted" exit 1.
```

### Level 4: Regression & Cross-cutting

```bash
go test -race ./...     # full suite
golangci-lint run ./...
# Byte-identity guard: cfg.Edit==false (default) → every commit path byte-identical to today — all
# pre-existing generate/decompose/stagecoach tests MUST pass UNCHANGED (they never set cfg.Edit).
# Exit-code guard: ErrEmptyMessage → exit 1 (NOT 3/124); HEAD+index untouched on abort (verify in a test
# repo: capture HEAD + `git diff --cached` before, run with a fake empty editor, assert unchanged after).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` compiles (all fake/stub git.Git impls updated for the 3 new methods).
- [ ] `go test ./...` green; pre-existing tests pass UNCHANGED (cfg.Edit never set → byte-identity).
- [ ] `go vet ./...` + `golangci-lint run` clean; `gofmt` no diff.

### Feature Validation
- [ ] `--edit` unset → every commit path byte-identical to today (EditMessage no-op).
- [ ] `--edit` (single) → editor opens on `.git/STAGECOACH_EDITMSG` (message + `# Please edit...` + `# Tree:` + `# <name-status>`); stripped message lands.
- [ ] Empty editor result → exit 1 "empty commit message — aborted"; HEAD+index untouched; no commit object.
- [ ] Edited message bypasses the duplicate check (a hand-edit matching a recent subject still commits — FR-E3).
- [ ] Editing-while-staging safe (stage in another pane during the edit; in-flight commit unaffected — FR-E2).
- [ ] Decompose: every commit gated (per-concept, FR-M11 shortcut, arbiter N+1).
- [ ] `--dry-run --edit` → warns, editor NOT opened (FR-E4).
- [ ] `hook exec --edit` → usage error (FR-E4).
- [ ] vim `:cq` (non-zero editor exit) → abort, no commit.

### Code Quality Validation
- [ ] Editor is POST-dedupe (FR-E3); template stays PRE-dedupe (FR-F8) — the two stages are SIBLINGS, not nested.
- [ ] ErrEmptyMessage is a PLAIN sentinel (NOT *RescueError) → exit 1, no recipe (abort, not rescue).
- [ ] Editor invoked via `sh -c '<editor> "$@"' -- <file>` (shell-interpreted; external_deps.md §6).
- [ ] 3 git methods are read-only w.r.t. refs/index; GitDir branches on code != 0 (no 128-as-unborn).
- [ ] cfg.Edit is flag-only (toml:"-"); no env/config/git precedence (FR-E1).
- [ ] chain.go UNCHANGED (arbiter N+1 transitive via generateMessage).

### Documentation & Deployment
- [ ] docs/cli.md `--edit` row (abort semantics, dedupe-bypass, decompose-each-commit, dry-run, hook-exec).
- [ ] docs/how-it-works.md FR-E2 call-out (REQUIRED — the edit-while-staging property).
- [ ] The FinalizeMessage godoc's ORDERING CONTRACT is honored (editor slotted after template, after dedupe).

---

## Anti-Patterns to Avoid

- ❌ Don't fold EditMessage into FinalizeMessage — FinalizeMessage is PRE-dedupe (FR-F8); the editor is
  POST-dedupe (FR-E3). They're siblings, not nested. Merging breaks the dedupe-bypass.
- ❌ Don't treat ErrEmptyMessage as a rescue (exit 3/124) — it's an intentional abort (exit 1); no recipe.
- ❌ Don't invoke the editor with a bare `exec.Command(editor, file)` — the value is shell-interpreted
  (may have args); use `sh -c '<editor> "$@"' -- <file>` (external_deps.md §6).
- ❌ Don't silently commit on a non-zero editor exit (vim `:cq`) — treat it as an abort.
- ❌ Don't edit chain.go — the arbiter N+1 is covered transitively via generateMessage (call site #3);
  a second EditMessage there double-edits.
- ❌ Don't give --edit full precedence (env/config/git) — FR-E1 says "flag only"; mirror Context, not Template.
- ❌ Don't fail the run if DiffTreeNameStatus errors — the summary is best-effort; pass "" and continue.
- ❌ Don't add `--edit` to hookExecCmd's local flags — it's a GLOBAL persistent flag; hookexec.go REJECTS
  the inherited flag instead (FR-E4).
- ❌ Don't strip inline comments within a kept message line — git doesn't (only full `#`-prefixed lines).
- ❌ Don't forget the fake-editor tests across BOTH single + decompose paths (FR-E1 contract).

---

## Confidence Score

**8/10** for one-pass implementation success. The design is well-pinned: the post-dedupe/pre-publish
insertion points are explicit (4 sites + the transitive arbiter proof), the 3 git methods use exact
subcommands with a known read-only convention, the EDITMSG round-trip + strip semantics mirror git
parity, the abort-is-not-rescue invariant is proven (WriteTree done, CommitTree not reached), and the
`cfg.Edit==false` no-op is the byte-identity regression guard. The two genuine risks: (1) the `git.Git`
interface edit ripples to EVERY test double in the repo (mitigated by `go build ./...` enumerating them
immediately after the interface edit); (2) the dedupe-vs-editor ordering subtlety (the editor MUST be
post-dedupe while the template is pre-dedupe — resolved explicitly in research/editor-gate-design.md §6
and locked by the "siblings not nested" anti-pattern). The −2 is residual risk in the fake-editor test
harness wiring (the `t.Setenv("GIT_EDITOR", script)` route is clean but the editor subprocess must be
executable + the stdio must be the test's, which needs care in a non-TTY CI environment — the
strip-semantics unit test is CI-safe regardless). The non-interactive default path is untouched, so the
feature is transparent when off.
