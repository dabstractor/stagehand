# P1.M3.T1.S2 — Design Decisions (recursion prevention + message-file lifecycle)

Ground truth read before writing this note:
- **Bug-Fix PRD §9.25 FR-V4** (prepare-commit-msg composes with the installed stagecoach hook: skip stagecoach's
  OWN prepare-commit-msg on the plumbing path — recursion prevention; a foreign one runs + annotates; read back).
  §9.25 FR-V2 (prepare-commit-msg `<msgfile> ""`; commit-msg `<msgfile>`; read back the result).
- **The S1 CONTRACT** (P1.M3.T1S1/PRP.md): creates `internal/hooks/runner.go` with `RunCommitHooks` (pre-commit →
  prepare-commit-msg → commit-msg) + helpers, INCLUDING: the **seam** `shouldSkipStagecoachPrepareCommitMsg(
  hooksDir) bool` (S1 STUBS it `false` + a TODO comment); `runPrepareCommitMsg`/`runCommitMsg` which each call
  `runMsgHook` (a per-call temp-file create/write/run/read-back); `stripCommentLines(s)` (hardcoded '#' prefix);
  `runHook` (the os/exec seam: `exec.CommandContext`, env, timeout, stdin=/dev/null, stdout/stderr pass-through).
  S2 CONSUMES runner.go and EDITS these pieces.
- **codebase_reality.md §4**: `hook.Marker` (script.go:15) = `"# stagecoach prepare-commit-msg hook v1"`;
  `hook.HookFilename` = `"prepare-commit-msg"`; `hook.Status` = StatusNone/StatusStagecoach/StatusForeign;
  `hook.Detect(hooksDir) (Status, error)` — `os.ErrNotExist`→StatusNone; contains Marker→StatusStagecoach;
  present-without-Marker→StatusForeign. "For FR-V4: the runner calls hook.Detect; if StatusStagecoach, SKIP it."
- **internal/git/git.go** (the Git interface): `HooksPath`/`GitDir`/`TopLevel` (L368, EXISTS — resolves S1 §10);
  `ReadTreeInto`/`WriteTreeFrom`; `ConfigGlobalGet` (global-only; NO local `git config --get`). The only mock Git
  is `contentionFakeGit` (cmd/lock_contention_test.go:18) which EMBEDS `git.Git` ⇒ safe to extend the interface.
- **internal/generate/finalize.go:125** `stripCommentsAndTrim` (unexported, package generate, hardcoded '#') —
  the existing pattern; S2's runner needs a commentChar-AWARE version (parameterized).
- Verified at research time: `go build ./... && go test ./...` GREEN.

---

## §0 — Scope: edit S1's runner.go (the seam + message lifecycle + strip) + add CommentChar to Git + tests

**S2 owns:** (a) fill `shouldSkipStagecoachPrepareCommitMsg` via `hook.Detect` (+ verbose log on skip); (b) refactor
the message-file lifecycle to ONE shared temp file (prepare + commit-msg both operate on it; strip ONLY at the
final read-back); (c) honor `core.commentChar` (add `CommentChar(ctx)` to the Git interface; parameterize
`stripCommentLines`); (d) add the `internal/hook` import to runner.go; (e) 3+ tests.

**Consumed UNCHANGED (do NOT edit):** S1's `runPreCommitScoped` (the scoped pre-commit + enforceSubset + re-tree),
`runHook` (the os/exec seam), `hookExecutable`, `tmpIndexPath`, `RunPostCommit`, `rescueErr`. internal/hook/*
(Detect/Marker/Status — consumed). internal/hooks/subset.go (enforceSubset — consumed). The wiring (M3.T2) +
docs (M4.T1) are NOT this task.

**No conflict with S1 (sequential):** S1 CREATES runner.go; S2 EDITS it. S2's edits are to the message-hook
portion + the seam + strip — S1's pre-commit/runHook/RunPostCommit are untouched. S2 adds CommentChar to
internal/git/git.go (S1 does NOT touch git.go — TopLevel already exists), so no cross-file parallel conflict.

---

## §1 — Fill the seam: `shouldSkipStagecoachPrepareCommitMsg` via `hook.Detect` (FR-V4 recursion prevention)

**Decision:** replace S1's stub body with:
```go
func shouldSkipStagecoachPrepareCommitMsg(hooksDir string) bool {
	status, _ := hook.Detect(hooksDir)      // err is non-fatal: on read error, treat as NOT stagecoach (run it)
	return status == hook.StatusStagecoach
}
```
The function stays PURE (returns bool, no I/O — the verbose log is in the CALLER, `runPrepareCommitMsg`). The
`hook.Detect` error is IGNORED (a read error ⇒ StatusNone from Detect's own handling, OR we treat unknown as
"don't skip" — running a hook that might be stagecoach's is the conservative failure mode: the worst case is a
recursion, but Detect only errors on non-ErrNotExist read failures, which are rare; the default-safe choice is
to NOT skip on error, matching Detect's StatusNone-on-ErrNotExist). Add `internal/hook` to runner.go's imports.

**The verbose log** lives in `runPrepareCommitMsg` (the caller), after the skip is detected:
```go
if shouldSkipStagecoachPrepareCommitMsg(hooksDir) {
	if opts.Verbose != nil {
		opts.Verbose.VerboseWarn("skipping stagecoach's own prepare-commit-msg hook on the plumbing path (FR-V4 recursion prevention)")
	}
	return nil
}
```
This is nil-guarded (opts.Verbose may be nil — S1's convention). The message matches the contract verbatim.

---

## §2 — Message-file lifecycle: ONE shared temp file for prepare-commit-msg + commit-msg (strip ONLY at the end)

**Decision:** refactor S1's per-call `runMsgHook` (which created a SEPARATE temp file per hook) into a SINGLE
shared message file owned by `RunCommitHooks`. This faithfully emulates git (ONE message file for the whole
commit; prepare-commit-msg writes/annotates it, commit-msg sees the SAME file). The lifecycle:
1. `RunCommitHooks` creates ONE temp file (`os.CreateTemp("", "stagecoach-hookmsg-*.txt")`), writes the incoming
   `msg` (the generated+deduped+--edit-finalized message), `defer os.Remove(path)`.
2. **prepare-commit-msg** runs on that file (if present + not stagecoach's own): `runHook(... hookPath,
   []string{msgPath, ""}, ...)`. May annotate (append a ticket ref). Non-zero/timeout → abort (§4).
3. **commit-msg** runs on the SAME file (if present + not NoVerify): `runHook(... hookPath, []string{msgPath},
   ...)`. Sees prepare's annotations. May mutate (lint fixes). Non-zero/timeout → abort.
4. **Final read-back ONCE** (after commit-msg): `os.ReadFile(msgPath)` → `stripCommentLines(content,
   commentChar)` → `finalMsg`. (No stripping BETWEEN prepare and commit-msg — commit-msg sees the full file.)

This REPLACES S1's `runMsgHook`/`runPrepareCommitMsg`/`runCommitMsg` (per-call temp) with thin "run hook on the
shared file" helpers. The read-back + strip is centralized in `RunCommitHooks` (single place). The temp file is
cleaned up by the `defer os.Remove`.

**Why ONE file (not S1's per-call):** (a) git parity — git uses ONE message file; commit-msg sees prepare's
output. (b) the contract: "prepare-commit-msg and commit-msg both operate on this same file; the final read-back
   (after commit-msg) is the finalMsg." (c) a prepare hook that leaves a non-message artifact (e.g. a trailing
   block) is visible to commit-msg — faithful. (d) one create + one read + one remove (vs 2× in S1's skeleton).

---

## §3 — Honor `core.commentChar` (FR: git's cleanup=strip honors it)

**Decision:** add `CommentChar(ctx) (string, error)` to the **Git interface** (+ gitRunner impl in git.go). It
runs `git config --get core.commentChar`: exit 0 → the trimmed value (may be multi-char); exit 1 (unset) → `"#"`;
  empty value OR `"auto"` → `"#"` (git's default; "auto" is too complex to resolve and rare); other exit → error.
`RunCommitHooks` calls `g.CommentChar(ctx)` ONCE (one cheap git call, after commit-msg) and threads the char into
`stripCommentLines(content, commentChar)`. On CommentChar error, default `"#"` (best-effort — never block the
commit on a commentChar read failure).

**Why an interface method (not os/exec in the runner):** core.commentChar is a git-config READ (a git operation,
per S1's philosophy "git ops via the Git interface; os/exec only for user hook scripts"). It is SAFE to add to the
interface: the only non-real implementor is `contentionFakeGit` which EMBEDS `git.Git` (embedding promotes all
interface methods ⇒ compiles; it never calls CommentChar ⇒ no runtime issue). It mirrors the TopLevel one-method
addition (P1.M2). Narrowly scoped (CommentChar, not a generic ConfigGet). The tests use real repos (`git.New`),
which implement it directly.

**Parameterize `stripCommentLines`:** `stripCommentLines(s, commentChar string) string` — drop lines starting with
`commentChar` (default '#'); `strings.HasPrefix(line, commentChar)`. (S1's version hardcoded '#' → S2 parameterizes.)

---

## §4 — Error mapping for the msg hooks (RescueError with full context)

**Decision:** `runPrepareCommitMsg`/`runCommitMsg` return the CAUSE error (or nil) — NOT a pre-built RescueError.
`RunCommitHooks` wraps a non-nil cause into `*generate.RescueError{Kind:ErrRescue, TreeSHA:snapshotTree,
ParentSHA, Candidate:msg, Cause: err}` — centralizing the full-context rescue construction so the rescue state is
byte-identical to a generation failure (FR-V7). (S1's pre-commit helper keeps its own rescueErr — S2 does NOT
touch pre-commit; only the msg-hook portion is refactored.) The Candidate is the PRE-hook `msg` (the best
message the user could manually commit). Non-zero exit AND timeout both flow through this (runHook returns
*exec.ExitError or context.DeadlineExceeded).

---

## §5 — The refactored RunCommitHooks message-hook section (copy-ready)

```go
// (c)+(d) MESSAGE-FILE LIFECYCLE: ONE shared temp file for prepare-commit-msg + commit-msg (git parity).
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

// (c) PREPARE-COMMIT-MSG — ALWAYS runs (NoVerify + DryRun do NOT gate it). Skipped if absent/non-exec OR
// stagecoach's OWN hook (FR-V4 recursion prevention).
if cerr := runPrepareCommitMsg(ctx, cfg, opts, hooksDir, gitDir, workTree, msgPath); cerr != nil {
	return "", "", &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: snapshotTree,
		ParentSHA: parentSHA, Candidate: finalMsg, Cause: fmt.Errorf("prepare-commit-msg: %w", cerr)}
}

// (d) COMMIT-MSG — skip if NoVerify (FR-V5); RUNS under DryRun (FR-V8a: lint the would-be message).
if !cfg.NoVerify {
	if cerr := runCommitMsg(ctx, cfg, opts, hooksDir, gitDir, workTree, msgPath); cerr != nil {
		return "", "", &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: snapshotTree,
			ParentSHA: parentSHA, Candidate: finalMsg, Cause: fmt.Errorf("commit-msg: %w", cerr)}
	}
}

// Final read-back (after commit-msg) + strip comment lines (git cleanup=strip; honor core.commentChar).
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

Where the thin helpers (replacing S1's runMsgHook-based ones):
```go
// runPrepareCommitMsg runs prepare-commit-msg <msgPath> "" (PRD FR-V2; verify argc via open_questions §3).
// ALWAYS runs (caller gates NoVerify/DryRun for the OTHER hooks). Returns the cause error on non-zero/timeout.
func runPrepareCommitMsg(ctx context.Context, cfg config.Config, opts HookOpts, hooksDir, gitDir, workTree, msgPath string) error {
	hookPath := filepath.Join(hooksDir, "prepare-commit-msg")
	if !hookExecutable(hookPath) {
		return nil // absent/non-exec → silent skip
	}
	if shouldSkipStagecoachPrepareCommitMsg(hooksDir) { // FR-V4: stagecoach's OWN hook → skip (recursion)
		if opts.Verbose != nil {
			opts.Verbose.VerboseWarn("skipping stagecoach's own prepare-commit-msg hook on the plumbing path (FR-V4 recursion prevention)")
		}
		return nil
	}
	return runHook(ctx, cfg.HookTimeout, hookPath, []string{msgPath, ""}, gitDir, workTree, nil, opts)
}

// runCommitMsg runs commit-msg <msgPath>. Returns the cause error on non-zero/timeout.
func runCommitMsg(ctx context.Context, cfg config.Config, opts HookOpts, hooksDir, gitDir, workTree, msgPath string) error {
	hookPath := filepath.Join(hooksDir, "commit-msg")
	if !hookExecutable(hookPath) {
		return nil
	}
	return runHook(ctx, cfg.HookTimeout, hookPath, []string{msgPath}, gitDir, workTree, nil, opts)
}

// shouldSkipStagecoachPrepareCommitMsg — FR-V4 recursion prevention. S1 stubbed false; S2 fills via hook.Detect.
func shouldSkipStagecoachPrepareCommitMsg(hooksDir string) bool {
	status, _ := hook.Detect(hooksDir) // read error ⇒ StatusNone ⇒ don't skip (conservative)
	return status == hook.StatusStagecoach
}

// stripCommentLines drops git message-file comment lines (lines beginning with commentChar, default '#').
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

---

## §6 — CommentChar on the Git interface (internal/git/git.go)

**Decision:** add to the `Git` interface + `gitRunner`:
```go
// CommentChar returns the commit-message comment character from git config (core.commentChar), defaulting to
// "#" when unset/empty/"auto". Used by the commit-hooks runner (§9.25) to strip comment lines from the message
// file read-back (git's cleanup=strip honors core.commentChar). Runs `git config --get core.commentChar`:
// exit 0 = found (trimmed); exit 1 = unset → "#"; empty or "auto" → "#"; else wrapped error. Read-only.
CommentChar(ctx context.Context) (char string, err error)
```
gitRunner impl: `g.run(ctx, g.workDir, "config", "--get", "core.commentChar")`; exit 0 → TrimSpace(stdout), ""or
"auto" → "#"; exit 1 → "#"; else error. (Mirrors ConfigGlobalGet's exit-code handling but LOCAL scope, default '#'.)

---

## §7 — Tests (3 contract scenarios + commentChar + the recursion-skip-is-pure)

Add to `internal/hooks/runner_test.go` (white-box package hooks; temp repo + real shell-script hooks, mirroring
S1's idiom):
1. **TestRunCommitHooks_PrepareCommitMsg_StagecoachMarker_Skipped** — write a prepare-commit-msg containing the
   Marker line (`# stagecoach prepare-commit-msg hook v1`) + a body that would mutate the file; NoVerify=true
   (isolate prepare); run → finalMsg == original msg (the hook was SKIPPED, no mutation, no recursion). Assert
   finalMsg unchanged.
2. **TestRunCommitHooks_PrepareCommitMsg_Foreign_AnnotationReadBack** — write a FOREIGN prepare-commit-msg that
   appends a non-comment line (`echo "Refs: #123" >> "$1"`); NoVerify=true; run → finalMsg CONTAINS "Refs: #123"
   (the annotation was read back from the shared file). (commit-msg absent.)
3. **TestRunCommitHooks_PrepareCommitMsg_Absent_NoOp** — no prepare-commit-msg installed; NoVerify=true; run →
   finalMsg == original msg (no-op).
4. **TestStripCommentLines_HonorsCommentChar** — pure table: '#' strips "# note" but keeps "real"; ';' strips
   "; note" but keeps "real"; empty commentChar → defaults '#'.
5. **TestCommentChar** (in internal/git, or via the runner) — `git config core.commentChar ';'` → CommentChar
   returns ";"; unset → "#".

(Tests 1–3 use the temp-repo + chmod-0755 shell-script-hook pattern from S1's runner_test.go. snapshotTree = the
seed commit's tree; msg = "feat: test"; NoVerify=true isolates prepare-commit-msg.)

---

## §8 — No new external deps; go.mod UNCHANGED

**Decision:** runner.go adds the `internal/hook` import (already in the module). git.go's CommentChar uses
already-imported stdlib. No new external dep. `go mod tidy` is a no-op.

---

## Summary table (the 8 calls at a glance)

| § | Decision | Source |
|---|----------|--------|
| 0 | Edit S1's runner.go (seam + msg lifecycle + strip) + add CommentChar to Git + tests; pre-commit/runHook/RunPostCommit frozen | contract |
| 1 | Fill shouldSkipStagecoachPrepareCommitMsg via hook.Detect==StatusStagecoach; verbose log in the caller | FR-V4, reality §4 |
| 2 | ONE shared temp msg file for prepare + commit-msg; strip ONLY at the final read-back | contract, FR-V2 |
| 3 | Honor core.commentChar via a new Git.CommentChar method; parameterize stripCommentLines | contract |
| 4 | msg hooks return the cause; RunCommitHooks wraps *RescueError{snapshotTree,parentSHA,msg,Cause} | FR-V7 |
| 5 | Copy-ready RunCommitHooks message-hook section + the thin helpers | §1–§4 |
| 6 | CommentChar on the Git interface (safe: contentionFakeGit embeds git.Git) | §3 |
| 7 | 3 contract scenarios (marker-skipped / foreign-append / absent) + commentChar tests | contract |
| 8 | No new deps; go.mod UNCHANGED | internal/hook already in module |
