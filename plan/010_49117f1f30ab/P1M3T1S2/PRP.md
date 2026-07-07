---
name: "P1.M3.T1.S2 — Recursion prevention (hook.Detect skip) + message-file lifecycle: fill S1's shouldSkipStagecoachPrepareCommitMsg seam, unify prepare/commit-msg on ONE shared temp file (strip at final read-back), honor core.commentChar — PRD §9.25 FR-V4 / FR-V2"
description: |

  Land the SECOND subtask of the commit-hooks runner (P1.M3.T1): refine S1's `internal/hooks/runner.go` with
  (a) FR-V4 recursion prevention — skip stagecoach's OWN prepare-commit-msg on the plumbing path; (b) the
  faithful message-file lifecycle — ONE shared temp file for prepare-commit-msg + commit-msg, read back once
  and stripped of comment lines at the end; (c) core.commentChar honored. S1 (P1.M3.T1.S1, being implemented
  in parallel) creates runner.go with the sequence + the scoped pre-commit + runHook + RunPostCommit, and
  STUBS two things S2 owns: the `shouldSkipStagecoachPrepareCommitMsg(hooksDir)` seam (stub `false`) and the
  per-call message-file handling. S2 fills the seam, unifies the message file, and adds core.commentChar.

  S2 CONSUMES S1's runner.go and EDITS the message-hook portion + the seam + strip — it does NOT touch S1's
  `runPreCommitScoped`, `runHook`, `hookExecutable`, `RunPostCommit`, or `enforceSubset` (frozen).

  THE THREE CHANGES:
    (1) SEAM — `shouldSkipStagecoachPrepareCommitMsg(hooksDir)`:
          `status, _ := hook.Detect(hooksDir); return status == hook.StatusStagecoach`
        The caller (`runPrepareCommitMsg`) logs at --verbose when it skips (nil-guarded):
          "skipping stagecoach's own prepare-commit-msg hook on the plumbing path (FR-V4 recursion prevention)"
        Add `internal/hook` to runner.go's imports.
    (2) MESSAGE-FILE LIFECYCLE — ONE shared temp file in RunCommitHooks (`os.CreateTemp("",
        "stagecoach-hookmsg-*.txt")`, `defer os.Remove`): write the incoming msg; prepare-commit-msg runs on it
        (may annotate); commit-msg runs on the SAME file (sees prepare's output); read back ONCE after
        commit-msg → `stripCommentLines(content, commentChar)` → finalMsg. REPLACES S1's per-call `runMsgHook`
        (which used a separate temp per hook) with thin "run hook on the shared file" helpers. NO stripping
        between prepare and commit-msg — strip ONLY at the final read-back (git parity: commit-msg sees the
        full file).
    (3) core.commentChar — add `CommentChar(ctx) (string, error)` to the Git interface (+ gitRunner impl =
        `git config --get core.commentChar`, default "#"); parameterize `stripCommentLines(s, commentChar)`.

  ⚠️ **#1 — FR-V4 recursion prevention is a SKIP, not a recursion.** If `hook.Detect(hooksDir) ==
      StatusStagecoach` (the installed prepare-commit-msg is stagecoach's own, identified by its Marker line),
      SKIP it on the plumbing path — the message is already generated; invoking it would exec `stagecoach hook
      exec` and recurse. A FOREIGN prepare-commit-msg (StatusForeign) RUNS and may annotate (append a ticket
      ref) — stagecoach reads the file back. StatusNone → no hook, skip. (research §1; reality §4.)

  ⚠️ **#2 — ONE shared message file for prepare + commit-msg (git parity).** S1's skeleton used a per-call temp
      (`runMsgHook` created a new file per hook). S2 UNIFIES: RunCommitHooks creates ONE temp file, threads it
      through both hooks, reads back once. commit-msg MUST see prepare-commit-msg's output (the same file),
      NOT a stripped copy. Strip ONLY at the final read-back. (research §2.)

  ⚠️ **#3 — core.commentChar via a new Git.CommentChar method (NOT os/exec in the runner).** core.commentChar is
      a git-config READ (a git op → the Git interface, per S1's philosophy). Add `CommentChar(ctx)` to the Git
      interface + gitRunner. SAFE: the only mock Git (`contentionFakeGit`) EMBEDS `git.Git` ⇒ adding a method
      compiles (promoted) and is never called on the mock ⇒ no runtime issue. Mirrors the TopLevel one-method
      addition. Default "#"; unset/empty/"auto" → "#". (research §3/§6.)

  ⚠️ **#4 — msg hooks return the CAUSE; RunCommitHooks wraps the *RescueError (full context).** `runPrepare
      CommitMsg`/`runCommitMsg` return the cause error (or nil); RunCommitHooks wraps a non-nil cause into
      `*generate.RescueError{Kind:ErrRescue, TreeSHA:snapshotTree, ParentSHA, Candidate:msg, Cause}` — byte-
      identical to a generation failure (FR-V7). S1's pre-commit keeps its own rescueErr (S2 does NOT touch
      pre-commit). (research §4.)

  ⚠️ **#5 — Do NOT touch S1's frozen pieces.** `runPreCommitScoped` (scoped pre-commit + enforceSubset +
      re-tree), `runHook` (the os/exec seam), `hookExecutable`, `tmpIndexPath`, `RunPostCommit`, `rescueErr`,
      `enforceSubset` (subset.go). S2 edits ONLY: the seam body, the message-hook portion of RunCommitHooks,
      `runPrepareCommitMsg`/`runCommitMsg` (→ thin shared-file helpers), `stripCommentLines` (→ parameterized),
      the imports (+`internal/hook`). Plus CommentChar in internal/git/git.go. (research §0.)

  ⚠️ **#6 — No conflict with S1 (sequential) or parallel work.** S1 CREATES runner.go; S2 EDITS it (sequential
      handoff). S2 adds CommentChar to internal/git/git.go — S1 does NOT touch git.go (TopLevel already exists
      at L368), so no cross-file conflict. The wiring (M3.T2) + docs (M4.T1) are separate tasks.

  ⚠️ **#7 — No new external deps; go.mod UNCHANGED.** runner.go adds the `internal/hook` import (in-module).
      git.go's CommentChar uses already-imported stdlib.

  Deliverable: MODIFIED `internal/hooks/runner.go` (fill the seam; refactor the message-hook portion to ONE
  shared file; parameterize stripCommentLines; +`internal/hook` import) + MODIFIED `internal/git/git.go`
  (CommentChar interface method + gitRunner impl) + MODIFIED/extended `internal/hooks/runner_test.go` (3
  contract scenarios + commentChar tests). NO wiring (M3.T2). NO docs (M4.T1). `go build/vet/test ./...` green.

---

## Goal

**Feature Goal**: Complete S1's commit-hooks runner with (a) FR-V4 recursion prevention — detect and skip
stagecoach's OWN prepare-commit-msg on the plumbing path (invoking it would recurse into `stagecoach hook exec`);
(b) the faithful message-file lifecycle — ONE shared temp file for prepare-commit-msg + commit-msg (commit-msg
sees prepare's annotations), read back once and stripped of comment lines; (c) `core.commentChar` honored in
the strip. A foreign prepare-commit-msg's annotation (ticket ref, branch name) is read back and committed; a
stagecoach-owned prepare-commit-msg is skipped (no recursion); an absent one is a no-op.

**Deliverable** (MODIFIED files):
1. **MODIFIED `internal/hooks/runner.go`** — fill `shouldSkipStagecoachPrepareCommitMsg` (via `hook.Detect`);
   refactor RunCommitHooks' message-hook section to ONE shared temp file (write → prepare → commit-msg → read
   back → strip); replace the per-call `runMsgHook`-based `runPrepareCommitMsg`/`runCommitMsg` with thin
   shared-file helpers; parameterize `stripCommentLines(s, commentChar)`; add the `internal/hook` import; add
   the nil-guarded verbose log on recursion skip.
2. **MODIFIED `internal/git/git.go`** — add `CommentChar(ctx) (string, error)` to the `Git` interface +
   `gitRunner` impl (`git config --get core.commentChar`, default "#").
3. **MODIFIED `internal/hooks/runner_test.go`** — 3 contract scenarios (stagecoach-marker skipped / foreign
   annotation read back / absent no-op) + a `stripCommentLines` commentChar table + a CommentChar test.

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean; a
stagecoach-marker prepare-commit-msg is SKIPPED (no recursion, msg unchanged); a foreign prepare-commit-msg's
appended annotation is read back into finalMsg; an absent prepare-commit-msg is a no-op; `core.commentChar`
(non-'#') is honored (only lines starting with that char are stripped); prepare + commit-msg operate on the
SAME file; S1's frozen pieces (pre-commit/runHook/RunPostCommit/enforceSubset) byte-unchanged; go.mod/go.sum
byte-unchanged.

## User Persona

**Target User**: A user who (a) has stagecoach's hook-mode `prepare-commit-msg` installed AND commits via
`stagecoach` (the plumbing path) — without S2, the plumbing path would invoke that hook and recurse into
`stagecoach hook exec`; (b) has a FOREIGN `prepare-commit-msg` that annotates the message (e.g. appends a ticket
ref) — expects the annotation to land in the commit; (c) uses a non-`#` `core.commentChar` — expects comment
lines stripped correctly. Transitively: the wiring subtasks P1.M3.T2/T3 that call RunCommitHooks.

**Use Case**: A user with a foreign prepare-commit-msg that appends "Refs: #123" runs `stagecoach`. The runner
writes the generated message to the shared file, runs prepare-commit-msg (appends "Refs: #123"), runs
commit-msg (sees the annotation), reads back, strips '#' comment lines → the commit message includes "Refs:
#123". If instead the installed prepare-commit-msg is stagecoach's own, the runner skips it (no recursion).

**User Journey**: (internal) RunCommitHooks → [pre-commit] → create shared msg file (write msg) →
runPrepareCommitMsg (skip if stagecoach-own/absent; else annotate) → runCommitMsg (lint; sees prepare's output)
→ read back → stripCommentLines(commentChar) → finalMsg → caller does commit-tree.

**Pain Points Addressed**: Without S2, (a) the plumbing path would recurse through stagecoach's own
prepare-commit-msg hook (infinite/regenerate loop); (b) a foreign prepare-commit-msg's annotation would be lost
(or commit-msg wouldn't see it); (c) non-`#` comment chars would leave comment lines in the commit.

## Why

- **Closes S1's two stubs (the recursion seam + the message lifecycle).** S1 deliberately deferred these to S2
  (a clean seam + a lifecycle refinement). S2 is the focused completion.
- **Satisfies PRD §9.25 FR-V4 (recursion prevention + foreign annotation) and FR-V2 (the message-file
  round-trip).** Git parity: ONE message file; commit-msg sees prepare's output; read back + cleanup=strip.
- **Faithful + minimal.** S2 refactors only the message-hook portion of S1's runner (the pre-commit/runHook/
  RunPostCommit core is untouched) + adds one narrow Git method. No new deps, no wiring, no docs.

## What

Modified `internal/hooks/runner.go` (the seam + the message-hook lifecycle + parameterized strip + the
`internal/hook` import), modified `internal/git/git.go` (the `CommentChar` method), and extended
`internal/hooks/runner_test.go` (3 scenarios + commentChar). No new files, no new deps, no wiring, no docs.

### Success Criteria

- [ ] `shouldSkipStagecoachPrepareCommitMsg(hooksDir)` returns `hook.Detect(hooksDir) == hook.StatusStagecoach`.
- [ ] `runPrepareCommitMsg` skips (nil-guarded verbose log) when stagecoach's own hook is detected; runs a
      foreign hook; no-op when absent. `runCommitMsg` runs commit-msg (skip only via NoVerify).
- [ ] RunCommitHooks creates ONE shared temp msg file; prepare + commit-msg both operate on it; the final
      read-back (after commit-msg) → `stripCommentLines` → finalMsg. No stripping between the two hooks.
- [ ] `stripCommentLines(s, commentChar)` drops lines starting with `commentChar` (default '#').
- [ ] `CommentChar(ctx)` on the Git interface (+ gitRunner): `git config --get core.commentChar`; default '#'
      on unset/empty/"auto"/error-best-effort.
- [ ] msg-hook non-zero/timeout → `*generate.RescueError{Kind:ErrRescue, TreeSHA:snapshotTree, ParentSHA,
      Candidate:msg, Cause}` (RunCommitHooks wraps the cause the thin helpers return).
- [ ] Tests: stagecoach-marker prepare-commit-msg → skipped (msg unchanged); foreign → annotation read back;
      absent → no-op; stripCommentLines honors commentChar; CommentChar reads the config.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean; S1's frozen pieces +
      go.mod/go.sum byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the S1 runner.go structure (the
seam + runMsgHook + stripCommentLines to refactor), the copy-ready code (Blueprint §1–§3), the `hook.Detect`
API (confirmed), the CommentChar interface addition (Blueprint §4), and the test matrix (Blueprint §5). No
decompose/generate-internals knowledge required.

### Documentation & References

```yaml
# MUST READ — the design calls (the seam, the shared file, commentChar, the error mapping, the tests)
- docfile: plan/010_49117f1f30ab/P1M3T1S2/research/design-decisions.md
  why: §0 (scope: edit S1's runner.go + CommentChar + tests; pre-commit/runHook/RunPostCommit frozen), §1 (the
       seam via hook.Detect + the verbose log), §2 (ONE shared msg file; strip ONLY at the end), §3 (core.
       commentChar via Git.CommentChar; parameterize strip), §4 (msg hooks return cause; RunCommitHooks wraps
       *RescueError), §5 (copy-ready RunCommitHooks section + thin helpers), §6 (CommentChar on Git), §7 (tests).
  critical: §1 (skip on StatusStagecoach; run StatusForeign; no-op StatusNone), §2 (ONE shared file; commit-msg
       sees prepare's output; strip only at the end), §3 (Git.CommentChar, not os/exec) are the things most
       likely to go wrong.

# MUST READ — the S1 CONTRACT (the runner.go S2 edits)
- docfile: plan/010_49117f1f30ab/P1M3T1S1/PRP.md
  why: S1 creates runner.go with: the `shouldSkipStagecoachPrepareCommitMsg` seam (STUB false + TODO); the
       per-call `runMsgHook` (S2 replaces with shared-file helpers); `stripCommentLines` (hardcoded '#', S2
       parameterizes); `runHook` (the os/exec seam — S2 REUSES, not rewrites); `runPreCommitScoped`/`RunPostCommit`
       (frozen); the RescueError mapping.
  critical: S2 EDITS runner.go — do NOT rewrite S1's pre-commit/runHook/RunPostCommit. Only the seam + the
       message-hook portion + strip change. Plus CommentChar in git.go.

# MUST READ — the hook.Detect API (reality §4)
- docfile: plan/010_49117f1f30ab/architecture/codebase_reality.md
  section: "## 4. The hook-MODE module" — Marker, HookFilename, Status, Detect(hooksDir)→(Status,error); "For
           FR-V4: hook.Detect; StatusStagecoach → SKIP; StatusForeign → runs + annotates; read back."
  critical: Detect's exact semantics (ErrNotExist→StatusNone; Marker→StatusStagecoach; else StatusForeign).

# THE FILES BEING MODIFIED — READ before editing
- file: internal/hooks/runner.go   (S1 — the file S2 edits)
  section: the `shouldSkipStagecoachPrepareCommitMsg` stub (fill it); the RunCommitHooks message-hook section
           (refactor to ONE shared file); `runPrepareCommitMsg`/`runCommitMsg`/`runMsgHook` (→ thin shared-file
           helpers); `stripCommentLines` (parameterize); the imports (add `internal/hook`).
  why: the EXACT structure S2 refactors. S1's `runHook`/`hookExecutable`/`runPreCommitScoped`/`RunPostCommit`
       are REUSED UNCHANGED.
  critical: do NOT touch runPreCommitScoped/runHook/hookExecutable/tmpIndexPath/RunPostCommit/rescueErr/enforceSubset.

- file: internal/git/git.go   (add CommentChar to the Git interface + gitRunner)
  section: the `Git interface` block (add `CommentChar(ctx) (string, error)` near ConfigGlobalGet); the
           gitRunner impl (mirror ConfigGlobalGet's exit-code handling but LOCAL scope, default '#').
  why: the CommentChar addition (core.commentChar read).
  critical: the only non-real Git implementor is `contentionFakeGit` (embeds git.Git) ⇒ adding a method is SAFE.

# The hook.Detect API (consumed — read-only)
- file: internal/hook/hook.go   (S1 references; S2 imports + calls)
  section: `Detect(hooksDir string) (Status, error)` (L47); `StatusNone`/`StatusStagecoach`/`StatusForeign` (L21-23);
           `Marker` (script.go:15 = "# stagecoach prepare-commit-msg hook v1").
  why: the recursion-prevention detection. `status == hook.StatusStagecoach` ⇒ skip.
  critical: Detect is in package `hook` (singular); the runner is package `hooks` (plural) — add the import.

# The existing strip pattern (reference — S2 parameterizes it)
- file: internal/generate/finalize.go
  section: `stripCommentsAndTrim` (L125) — drops lines beginning with '#'. The pattern S2's parameterized
           `stripCommentLines(s, commentChar)` mirrors (but takes the char, and lives in package hooks).
  why: confirms the strip idiom; S2 does NOT import it (different package, hardcoded '#').

# The RescueError type (the failure→rescue mapping)
- file: internal/generate/generate.go
  section: `type RescueError{Kind error; TreeSHA, ParentSHA, Candidate string; Cause error}`; `ErrRescue`.
  why: RunCommitHooks wraps a msg-hook cause into *RescueError{...,Candidate:msg}.

# The PRD basis (in your context as selected_prd_content)
- file: PRD.md (or plan/010_…/prd_snapshot.md)
  section: "9.25 Hook execution on the commit path" FR-V4 (skip stagecoach's own prepare-commit-msg; foreign
           annotates; read back) + FR-V2 (prepare/commit-msg on the message file; read back the result).
  critical: FR-V4 recursion prevention is a SKIP for StatusStagecoach; StatusForeign RUNS.
```

### Current Codebase tree (relevant slice)

```bash
internal/hooks/
  runner.go          # S1 — EDIT (seam + message-hook lifecycle + parameterized strip + internal/hook import)
  runner_test.go     # S1 — EDIT (3 contract scenarios + commentChar tests)
  subset.go          # P1.M2.T2.S1 — UNCHANGED (enforceSubset; consumed)
internal/git/git.go  # EDIT (add CommentChar to Git interface + gitRunner impl)
internal/hook/       # the EXISTING hook-mode package (Detect/Marker/Status — consumed by S2; UNCHANGED)
internal/generate/   # RescueError — UNCHANGED (consumed)
go.mod / go.sum      # UNCHANGED (internal/hook already in module; stdlib only for CommentChar)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. THREE in-place edits: internal/hooks/runner.go + internal/hooks/runner_test.go + internal/git/git.go.
```

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (#1 — StatusStagecoach SKIPS; StatusForeign RUNS): hook.Detect(hooksDir)==StatusStagecoach ⇒ skip
//   prepare-commit-msg (recursion prevention). StatusForeign ⇒ run it (annotation). StatusNone ⇒ no-op.
//   A Detect read error ⇒ treat as NOT stagecoach (don't skip) — conservative. (research §1)

// CRITICAL (#2 — ONE shared msg file; strip ONLY at the end): RunCommitHooks creates ONE temp file; prepare +
//   commit-msg BOTH operate on it; read back ONCE after commit-msg → strip. Do NOT strip between the hooks
//   (commit-msg must see prepare's full output). REPLACES S1's per-call runMsgHook. (research §2)

// CRITICAL (#3 — CommentChar on the Git interface, NOT os/exec): core.commentChar is a git-config read → the
//   Git interface (S1's philosophy). Adding CommentChar is SAFE (contentionFakeGit embeds git.Git). Default "#".
//   (research §3/§6)

// CRITICAL (#4 — msg hooks return the CAUSE; RunCommitHooks wraps *RescueError): the thin runPrepareCommitMsg/
//   runCommitMsg return the cause error (or nil); RunCommitHooks wraps a non-nil cause into *RescueError{
//   Kind:ErrRescue, TreeSHA:snapshotTree, ParentSHA, Candidate:msg, Cause}. (research §4)

// GOTCHA (verbose log in the CALLER, nil-guarded): shouldSkipStagecoachPrepareCommitMsg is PURE (returns bool);
//   the "skipping stagecoach's own prepare-commit-msg hook…" log is in runPrepareCommitMsg, guarded
//   `if opts.Verbose != nil`.
// GOTCHA (add internal/hook import): runner.go is package hooks (plural); hook.Detect is package hook (singular).
//   Add "github.com/dustin/stagecoach/internal/hook" to runner.go's imports.
// GOTCHA (core.commentChar "auto"): git's core.commentChar can be "auto" (derive from template) — too complex
//   to resolve; treat "auto"/empty/unset as "#" (the common-case default). Documented.
// GOTCHA (do NOT touch S1's frozen pieces): runPreCommitScoped, runHook, hookExecutable, tmpIndexPath,
//   RunPostCommit, rescueErr, enforceSubset — UNCHANGED. S2 edits ONLY the seam + the message-hook portion +
//   stripCommentLines + imports.
// GOTCHA (CommentChar best-effort in RunCommitHooks): on CommentChar error, default "#" — NEVER block the commit
//   on a commentChar read failure.
// GOTCHA (no new deps): internal/hook is in-module; CommentChar uses stdlib. `go mod tidy` is a no-op.
```

## Implementation Blueprint

### §1. EDIT `internal/hooks/runner.go` — fill the seam + the shared message-file lifecycle

**Edit A — imports.** Add `"github.com/dustin/stagecoach/internal/hook"` to the import block.

**Edit B — fill the seam** (replace S1's stub):
```go
// shouldSkipStagecoachPrepareCommitMsg implements FR-V4 recursion prevention: if the installed prepare-commit-msg
// is stagecoach's OWN (detected by its Marker line), skip it on the plumbing path — the message is already
// generated and invoking it would exec `stagecoach hook exec` and recurse. A foreign hook (StatusForeign) runs
// and may annotate; absent (StatusNone) is a no-op. Pure (returns bool; the verbose log is in the caller).
func shouldSkipStagecoachPrepareCommitMsg(hooksDir string) bool {
	status, _ := hook.Detect(hooksDir) // a read error ⇒ StatusNone ⇒ don't skip (conservative: run rather than recurse-stall)
	return status == hook.StatusStagecoach
}
```

**Edit C — parameterize stripCommentLines** (replace S1's hardcoded '#' version):
```go
// stripCommentLines drops git message-file comment lines (lines beginning with commentChar) — git's default
// cleanup=strip, honoring core.commentChar. Used on the final read-back of the shared message file.
func stripCommentLines(s, commentChar string) string {
	if commentChar == "" {
		commentChar = "#"
	}
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, commentChar) {
			continue
		}
		b.WriteString(line + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
```

**Edit D — replace the per-call `runMsgHook`-based `runPrepareCommitMsg`/`runCommitMsg` with thin shared-file
helpers** (they run the hook on the EXISTING shared file; return the cause error or nil):
```go
// runPrepareCommitMsg runs prepare-commit-msg <msgPath> "" (PRD FR-V2; argc per open_questions §3) on the shared
// message file. ALWAYS runs (NoVerify/DryRun don't gate it — the caller gates the OTHER hooks). Skipped if
// absent/non-exec OR stagecoach's OWN hook (FR-V4). Returns the cause error on non-zero/timeout (caller wraps *RescueError).
func runPrepareCommitMsg(ctx context.Context, cfg config.Config, opts HookOpts, hooksDir, gitDir, workTree, msgPath string) error {
	hookPath := filepath.Join(hooksDir, "prepare-commit-msg")
	if !hookExecutable(hookPath) {
		return nil // absent/non-exec → silent skip
	}
	if shouldSkipStagecoachPrepareCommitMsg(hooksDir) { // FR-V4: stagecoach's OWN → skip (recursion)
		if opts.Verbose != nil {
			opts.Verbose.VerboseWarn("skipping stagecoach's own prepare-commit-msg hook on the plumbing path (FR-V4 recursion prevention)")
		}
		return nil
	}
	return runHook(ctx, cfg.HookTimeout, hookPath, []string{msgPath, ""}, gitDir, workTree, nil, opts)
}

// runCommitMsg runs commit-msg <msgPath> on the shared message file. Returns the cause error on non-zero/timeout.
func runCommitMsg(ctx context.Context, cfg config.Config, opts HookOpts, hooksDir, gitDir, workTree, msgPath string) error {
	hookPath := filepath.Join(hooksDir, "commit-msg")
	if !hookExecutable(hookPath) {
		return nil
	}
	return runHook(ctx, cfg.HookTimeout, hookPath, []string{msgPath}, gitDir, workTree, nil, opts)
}
```
(DELETE S1's `runMsgHook` — its create/write/read-back per call is replaced by the shared file in RunCommitHooks.)

**Edit E — RunCommitHooks' message-hook section** (replace S1's prepare/commit-msg calls with the shared-file
lifecycle; the pre-commit section ABOVE it is UNCHANGED):
```go
	// (c)+(d) MESSAGE-FILE LIFECYCLE — ONE shared temp file for prepare-commit-msg + commit-msg (git parity:
	// commit-msg sees prepare's output). Strip ONLY at the final read-back (git cleanup=strip).
	msgFile, err := os.CreateTemp("", "stagecoach-hookmsg-*.txt")
	if err != nil {
		return "", "", fmt.Errorf("hooks: create message file: %w", err)
	}
	msgPath := msgFile.Name()
	defer os.Remove(msgPath)
	if _, werr := msgFile.WriteString(finalMsg); werr != nil {
		msgFile.Close()
		return "", "", fmt.Errorf("hooks: write message file: %w", werr)
	}
	msgFile.Close()

	// (c) PREPARE-COMMIT-MSG — ALWAYS runs (NoVerify + DryRun don't gate it). Skipped if absent OR stagecoach's OWN.
	if cerr := runPrepareCommitMsg(ctx, cfg, opts, hooksDir, gitDir, workTree, msgPath); cerr != nil {
		return "", "", &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: snapshotTree,
			ParentSHA: parentSHA, Candidate: finalMsg, Cause: fmt.Errorf("prepare-commit-msg: %w", cerr)}
	}

	// (d) COMMIT-MSG — skip if NoVerify (FR-V5); RUNS under DryRun (FR-V8a).
	if !cfg.NoVerify {
		if cerr := runCommitMsg(ctx, cfg, opts, hooksDir, gitDir, workTree, msgPath); cerr != nil {
			return "", "", &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: snapshotTree,
				ParentSHA: parentSHA, Candidate: finalMsg, Cause: fmt.Errorf("commit-msg: %w", cerr)}
		}
	}

	// Final read-back (after commit-msg) + strip comment lines (honor core.commentChar).
	commentChar, ccErr := g.CommentChar(ctx)
	if ccErr != nil || commentChar == "" {
		commentChar = "#" // best-effort default — never block the commit on a commentChar read
	}
	data, rErr := os.ReadFile(msgPath)
	if rErr != nil {
		return "", "", fmt.Errorf("hooks: read back message file: %w", rErr)
	}
	finalMsg = stripCommentLines(string(data), commentChar)

	return finalTree, finalMsg, nil
```
> NOTE: the `finalTree`/`finalMsg` defaults (`= snapshotTree, msg`) and the pre-commit section above are S1's
> (UNCHANGED). S2 replaces only the prepare/commit-msg portion + adds the shared-file lifecycle + the final
> read-back/strip. Adapt the surrounding code to S1's exact variable names.

### §2. EDIT `internal/git/git.go` — add `CommentChar` to the Git interface + gitRunner

**Interface** (near `ConfigGlobalGet`):
```go
	// CommentChar returns the commit-message comment character from git config (core.commentChar), defaulting
	// to "#" when unset/empty/"auto". Used by the commit-hooks runner (§9.25) to strip comment lines from the
	// message-file read-back (git's cleanup=strip honors core.commentChar). Runs `git config --get core.commentChar`
	// (LOCAL then global): exit 0 = found (trimmed; may be multi-char); exit 1 = unset → "#"; empty/"auto" → "#";
	// else wrapped error. Read-only w.r.t. refs and the index (PRD §18.1).
	CommentChar(ctx context.Context) (char string, err error)
```
**gitRunner impl:**
```go
// CommentChar returns core.commentChar (default "#") via `git config --get core.commentChar`.
func (g *gitRunner) CommentChar(ctx context.Context) (string, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "config", "--get", "core.commentChar")
	if err != nil {
		return "", err
	}
	switch code {
	case 0:
		c := strings.TrimSpace(stdout)
		if c == "" || c == "auto" {
			return "#", nil // empty or "auto" → git's default
		}
		return c, nil
	case 1:
		return "#", nil // unset → git's default
	default:
		return "", fmt.Errorf("git config --get core.commentChar: exit %d: %s", code, strings.TrimSpace(stderr))
	}
}
```

### §3. EDIT `internal/hooks/runner_test.go` — 3 contract scenarios + commentChar

```go
// (1) RECURSION PREVENTION — a stagecoach-marker prepare-commit-msg is SKIPPED (msg unchanged, no recursion).
func TestRunCommitHooks_PrepareCommitMsg_StagecoachMarker_Skipped(t *testing.T) {
	repoDir, g := initTempRepoForHooks(t) // helper: temp repo + seed commit; returns (dir, git.Git, snapshotTree)
	// Install a prepare-commit-msg that IS stagecoach's own (Marker present) + would mutate the file if run.
	writeHook(t, repoDir, "prepare-commit-msg", "#!/bin/sh\n# stagecoach prepare-commit-msg hook v1\necho 'RECURRED' >> \"$1\"\n")
	cfg := config.Config{NoVerify: true} // isolate prepare (skip pre-commit + commit-msg)
	finalTree, finalMsg, err := RunCommitHooks(context.Background(), g, cfg, snapshotTree, parentSHA, "feat: test", HookOpts{})
	if err != nil {
		t.Fatalf("expected nil err (skip), got: %v", err)
	}
	_ = finalTree
	if strings.Contains(finalMsg, "RECURRED") {
		t.Errorf("stagecoach's own prepare-commit-msg was NOT skipped (recursion): %q", finalMsg)
	}
	if finalMsg != "feat: test" {
		t.Errorf("msg changed despite skip: %q", finalMsg)
	}
}

// (2) FOREIGN ANNOTATION — a foreign prepare-commit-msg appends a line; stagecoach reads it back.
func TestRunCommitHooks_PrepareCommitMsg_Foreign_AnnotationReadBack(t *testing.T) {
	repoDir, g, snapshotTree, parentSHA := initTempRepoForHooks(t)
	writeHook(t, repoDir, "prepare-commit-msg", "#!/bin/sh\necho 'Refs: #123' >> \"$1\"\n")
	cfg := config.Config{NoVerify: true} // isolate prepare (no commit-msg)
	_, finalMsg, err := RunCommitHooks(context.Background(), g, cfg, snapshotTree, parentSHA, "feat: test", HookOpts{})
	if err != nil {
		t.Fatalf("expected nil err, got: %v", err)
	}
	if !strings.Contains(finalMsg, "Refs: #123") {
		t.Errorf("foreign prepare-commit-msg annotation not read back: %q", finalMsg)
	}
}

// (3) ABSENT — no prepare-commit-msg → no-op (msg unchanged).
func TestRunCommitHooks_PrepareCommitMsg_Absent_NoOp(t *testing.T) {
	repoDir, g, snapshotTree, parentSHA := initTempRepoForHooks(t) // no hooks installed
	cfg := config.Config{NoVerify: true}
	_, finalMsg, err := RunCommitHooks(context.Background(), g, cfg, snapshotTree, parentSHA, "feat: test", HookOpts{})
	if err != nil {
		t.Fatalf("expected nil err, got: %v", err)
	}
	if finalMsg != "feat: test" {
		t.Errorf("absent prepare-commit-msg should be a no-op: %q", finalMsg)
	}
}

// (4) stripCommentLines honors the comment char (pure table).
func TestStripCommentLines_HonorsCommentChar(t *testing.T) {
	cases := []struct{ name, in, char, want string }{
		{"hash strips # lines", "feat: x\n# comment\nbody", "#", "feat: x\nbody"},
		{"semicolon strips ; lines", "feat: x\n; comment\nbody", ";", "feat: x\nbody"},
		{"empty char defaults to hash", "feat: x\n# c\nbody", "", "feat: x\nbody"},
		{"no comment lines unchanged", "feat: x\nbody", "#", "feat: x\nbody"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripCommentLines(tc.in, tc.char); got != tc.want {
				t.Errorf("stripCommentLines(%q, %q) = %q, want %q", tc.in, tc.char, got, tc.want)
			}
		})
	}
}
```
> **Helpers:** `initTempRepoForHooks(t)` = S1's temp-repo + seed-commit helper, ALSO returning the seed commit's
> tree (snapshotTree) + SHA (parentSHA) — mirror S1's `initTempRepo` + add `g.WriteTree(ctx)`/`rev-parse HEAD`.
> `writeHook(t, repoDir, name, body)` = resolve `<repoDir>/.git/hooks/` (or `g.HooksPath`), `os.WriteFile` +
> `os.Chmod(path, 0o755)`. (Both mirror S1's runner_test.go helpers.) A CommentChar test (`git config
> core.commentChar ';'` → `g.CommentChar(ctx) == ";"`) may live in internal/git or hook into scenario 2.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/git/git.go — add CommentChar (interface + gitRunner)
  - ADD CommentChar(ctx) to the Git interface (near ConfigGlobalGet) + the gitRunner impl (Blueprint §2).
  - GOTCHA: LOCAL scope (`git config --get`, NOT --global); default "#"; "auto"/empty → "#".
  - CONFIRM: contentionFakeGit embeds git.Git ⇒ compiles (no mock breakage).

Task 2: EDIT internal/hooks/runner.go — fill the seam (Edit A+B)
  - ADD "internal/hook" import. FILL shouldSkipStagecoachPrepareCommitMsg via hook.Detect==StatusStagecoach.
  - PARAMETERIZE stripCommentLines(s, commentChar).

Task 3: EDIT internal/hooks/runner.go — the shared message-file lifecycle (Edit C+D+E)
  - REPLACE S1's runMsgHook-based runPrepareCommitMsg/runCommitMsg with the thin shared-file helpers (Blueprint §1 Edit D).
  - DELETE S1's runMsgHook (its per-call temp is superseded).
  - REPLACE RunCommitHooks' prepare/commit-msg section with the shared-file lifecycle (Edit E): ONE temp file →
      prepare → commit-msg → read back → stripCommentLines(commentChar). Wrap non-nil causes in *RescueError.
  - DO NOT touch runPreCommitScoped/runHook/hookExecutable/RunPostCommit/rescueErr/enforceSubset.

Task 4: EDIT internal/hooks/runner_test.go — the 3 scenarios + commentChar (Blueprint §3)
  - ADD the initTempRepoForHooks + writeHook helpers (mirror S1's).
  - ADD TestRunCommitHooks_PrepareCommitMsg_StagecoachMarker_Skipped / _Foreign_AnnotationReadBack / _Absent_NoOp.
  - ADD TestStripCommentLines_HonorsCommentChar (+ optionally a CommentChar config test).

Task 5: VERIFY
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. S1's frozen pieces byte-unchanged.
      `go build/vet/test ./...` green. The 3 scenarios + commentChar tests pass.
```

### Implementation Patterns & Key Details

```go
// THE seam (FR-V4 recursion prevention — skip stagecoach's OWN prepare-commit-msg):
func shouldSkipStagecoachPrepareCommitMsg(hooksDir string) bool {
	status, _ := hook.Detect(hooksDir)
	return status == hook.StatusStagecoach
}

// THE shared message file (ONE file; prepare + commit-msg; strip at the end):
msgFile, _ := os.CreateTemp("", "stagecoach-hookmsg-*.txt"); msgPath := msgFile.Name(); defer os.Remove(msgPath)
msgFile.WriteString(finalMsg); msgFile.Close()
runPrepareCommitMsg(..., msgPath)   // skip if stagecoach-own/absent; else annotate the file
if !cfg.NoVerify { runCommitMsg(..., msgPath) }  // sees prepare's output
commentChar, _ := g.CommentChar(ctx); if commentChar == "" { commentChar = "#" }
finalMsg = stripCommentLines(readBack(msgPath), commentChar)

// THE error mapping (msg hooks return cause; RunCommitHooks wraps full-context *RescueError):
if cerr := runPrepareCommitMsg(...); cerr != nil {
	return "", "", &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: snapshotTree,
		ParentSHA: parentSHA, Candidate: finalMsg, Cause: fmt.Errorf("prepare-commit-msg: %w", cerr)}
}
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. runner.go adds the in-module internal/hook import; git.go's CommentChar
      uses stdlib. `go mod tidy` is a no-op.

PACKAGE EDGES: internal/hooks → internal/hook (NEW edge — Detect/Status/Marker; in-module). internal/git gains
      CommentChar (interface + gitRunner). No new external dep. contentionFakeGit (embeds git.Git) is unaffected.

UPSTREAM (consume, do NOT edit):
  - S1's runner.go: runHook/hookExecutable/runPreCommitScoped/RunPostCommit/rescueErr/tmpIndexPath + the HookOpts/
    RunCommitHooks signatures.
  - internal/hook: Detect/Status/Marker/HookFilename.
  - internal/hooks/subset.go: enforceSubset (S1 consumes it in pre-commit; S2 doesn't touch).
  - generate.RescueError/ErrRescue.

DOWNSTREAM (NOT this task):
  - P1.M3.T2/T3 (wiring): call RunCommitHooks (now with recursion prevention + the faithful message lifecycle).
  - P1.M4.T1 (docs): the feature docs.

FROZEN/LEAVE (do NOT edit):
  - S1's runPreCommitScoped, runHook, hookExecutable, tmpIndexPath, RunPostCommit, rescueErr (runner.go).
  - internal/hooks/subset.go (+ test).
  - internal/hook/* (Detect/Marker/Status — consumed, not modified).
  - internal/generate/* (RescueError), internal/config/*, internal/cmd/*, pkg/stagecoach/*.
  - PRD.md, go.mod, Makefile. NO wiring (M3.T2). NO docs (M4.T1).

NO NEW DATABASE / ROUTES / CLI / FILES / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/hooks/runner.go internal/hooks/runner_test.go internal/git/git.go
go vet ./internal/hooks/ ./internal/git/
go build ./...
grep -n 'hook.Detect\|"github.com/dustin/stagecoach/internal/hook"' internal/hooks/runner.go && echo "(seam + import present)"
grep -n 'CommentChar' internal/git/git.go | head && echo "(CommentChar present)"
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; build clean; seam+import present; CommentChar present; go.mod/go.sum byte-unchanged.
```

### Level 2: The 3 scenarios + commentChar + S1's tests still green

```bash
go test ./internal/hooks/... -v -run 'PrepareCommitMsg|StripCommentLines|CommentChar'
go test ./internal/hooks/...   # S1's runner tests (pre-commit/timeout/no-verify/dry-run/post-commit) MUST stay green
go test ./internal/git/        # CommentChar + no regression
# Expected PASS — verify:
#   StagecoachMarker_Skipped .... stagecoach's own prepare-commit-msg SKIPPED (msg unchanged, no "RECURRED")
#   Foreign_AnnotationReadBack . appended "Refs: #123" IS in finalMsg (read back from the shared file)
#   Absent_NoOp ................ no prepare-commit-msg ⇒ msg unchanged
#   StripCommentLines_HonorsCommentChar ... '#' and ';' both strip their lines; empty defaults '#'
#   S1's tests .................. still PASS (S2 didn't touch pre-commit/runHook/RunPostCommit)
# If StagecoachMarker fails (msg changed), the seam didn't skip — check hook.Detect==StatusStagecoach.
# If Foreign fails (no annotation), the shared file isn't read back — check the read-back path.
```

### Level 3: Whole-repo + frozen-file check

```bash
go build ./...   # Expect clean.
go test ./...    # Expect all PASS (hooks + git suites; contentionFakeGit still compiles — embeds git.Git).
git diff --name-only | grep -E 'internal/hooks/(runner|runner_test)\.go|internal/git/git\.go' && echo "(expected files)"
git diff --exit-code internal/hooks/subset.go internal/hook internal/generate internal/config internal/cmd pkg && echo "frozen pkgs UNCHANGED (expected)"
# Confirm S1's frozen helpers in runner.go are byte-unchanged:
git diff internal/hooks/runner.go | grep '^-' | grep -v '^---' | grep -E 'runPreCommitScoped|func runHook|func RunPostCommit|hookExecutable|tmpIndexPath' || echo "S1 frozen helpers UNCHANGED (good)"
# Confirm NO wiring into callers (M3.T2 owns it):
! grep -rq "hooks.RunCommitHooks\|hooks.RunPostCommit" internal/generate internal/cmd pkg && echo "no caller wiring (good)"
```

### Level 4: FR-V4 + lifecycle correctness reasoning

```bash
# Verify by reasoning + the tests:
#   1. StatusStagecoach → SKIP prepare-commit-msg (no recursion); StatusForeign → RUN + read back; StatusNone → no-op. (Test 1/2/3)
#   2. ONE shared file: prepare + commit-msg operate on it; commit-msg sees prepare's output; read back ONCE. (Test 2)
#   3. core.commentChar honored (non-'#' strips correctly); default '#'; "auto"→"#". (StripCommentLines + CommentChar)
#   4. msg-hook non-zero/timeout → *RescueError{snapshotTree,parentSHA,msg,Cause} (byte-identical to gen failure).
#   5. S1's frozen pieces (pre-commit/runHook/RunPostCommit/enforceSubset) byte-unchanged. (Level 3 grep)
# (No Level-4 commands beyond Levels 1–3 — the tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on the edited files.
- [ ] `go test ./...` GREEN (the 3 scenarios + commentChar + S1's runner tests + no regression).
- [ ] go.mod/go.sum byte-unchanged; `internal/hook` import added to runner.go; CommentChar added to Git.

### Feature Validation
- [ ] `shouldSkipStagecoachPrepareCommitMsg` = `hook.Detect(hooksDir)==StatusStagecoach`; verbose log on skip.
- [ ] ONE shared temp msg file; prepare + commit-msg both operate on it; final read-back → stripCommentLines.
- [ ] `stripCommentLines(s, commentChar)` honors the char (default '#'); `CommentChar(ctx)` reads core.commentChar.
- [ ] msg-hook non-zero/timeout → `*generate.RescueError{...,Candidate:msg}`.
- [ ] Tests: stagecoach-marker skipped; foreign annotation read back; absent no-op; commentChar honored.

### Code Quality Validation
- [ ] S1's frozen pieces (runPreCommitScoped/runHook/hookExecutable/RunPostCommit/rescueErr/enforceSubset) UNCHANGED.
- [ ] CommentChar added safely (contentionFakeGit embeds git.Git; no mock breakage).
- [ ] Anti-patterns avoided (see below); no wiring (M3.T2); no docs (M4.T1).

### Documentation
- [ ] Doc comments cite PRD §9.25 FR-V4 (recursion) + FR-V2 (message round-trip) + core.commentChar. No docs/*.md
      here (M4.T1 owns the feature docs).

---

## Anti-Patterns to Avoid

- ❌ **Don't RUN stagecoach's own prepare-commit-msg (it would recurse).** `hook.Detect==StatusStagecoach` ⇒ SKIP.
      StatusForeign RUNS (annotation); StatusNone ⇒ no-op. (research §1)
- ❌ **Don't use a per-hook temp file (S1's runMsgHook).** ONE shared file for prepare + commit-msg; commit-msg
      sees prepare's output; strip ONLY at the final read-back. (research §2)
- ❌ **Don't read core.commentChar via os/exec in the runner.** It's a git-config read → the Git interface
      (`CommentChar(ctx)`). Safe to add (contentionFakeGit embeds git.Git). (research §3)
- ❌ **Don't strip comment lines BETWEEN prepare and commit-msg.** Strip ONLY at the final read-back. Commit-msg
      must see prepare's full output (including any non-comment artifacts). (research §2)
- ❌ **Don't touch S1's frozen pieces.** runPreCommitScoped/runHook/hookExecutable/tmpIndexPath/RunPostCommit/
      rescueErr/enforceSubset — UNCHANGED. S2 edits ONLY the seam + the message-hook portion + strip + imports.
- ❌ **Don't build the *RescueError inside the thin msg-hook helpers.** They return the CAUSE; RunCommitHooks
      wraps the full-context *RescueError (TreeSHA/ParentSHA/Candidate). (research §4)
- ❌ **Don't block the commit on a CommentChar read failure.** Default "#" best-effort. (research §3)
- ❌ **Don't treat core.commentChar="auto" as a literal prefix.** "auto"/empty/unset → "#" (the common-case
      default; "auto" resolution is out of scope). (research §3)
- ❌ **Don't forget the `internal/hook` import.** runner.go is package `hooks` (plural); Detect is package
      `hook` (singular). Add the import.
- ❌ **Don't add new external deps / edit PRD.md / wire callers / edit subset.go.** Scope: runner.go + runner_test.go + git.go (CommentChar).

---

## Confidence Score

**9/10** — S2 is a focused refinement of S1's runner.go: the seam is a one-line `hook.Detect==StatusStagecoach`
(Detect's API confirmed in hook.go:47 + reality §4); the message-file lifecycle is a clean refactor (ONE shared
temp file replacing S1's per-call runMsgHook — copy-ready); core.commentChar is a safe one-method Git interface
addition (contentionFakeGit embeds git.Git ⇒ no mock breakage; the only non-real implementor). S1's frozen pieces
(pre-commit/runHook/RunPostCommit/enforceSubset) are explicitly untouched, so S2 can't regress S1's tests. The
3 contract scenarios map directly to the 3 Detect statuses (Stagecoach→skip, Foreign→annotate, None→no-op), and
the stripCommentLines table pins the commentChar behavior. The one residual risk — adapting to S1's exact
variable names / the precise structure S1 lands (S2 runs after S1) — is covered by the "adapt the surrounding
code to S1's exact variable names" note + the run-until-green validation (S1's runner tests must stay green,
proving S2 didn't break the frozen pieces). The -1 reserves for the CommentChar interface addition needing the
gitRunner impl + the one mock that embeds git.Git (verified safe, but an interface touch is inherently slightly
broader than a same-file edit).
