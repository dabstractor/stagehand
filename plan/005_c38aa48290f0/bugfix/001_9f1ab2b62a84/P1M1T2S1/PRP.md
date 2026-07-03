name: "P1.M1.T2.S1 — Defer the hook exec 'Generating…' progress line past the no-op gates via a nil-safe Progress callback on generate.Deps"
description: |
  Fixes Issue 2 (Minor): `hook exec` prints `↳ Generating with <provider>…` to stderr UNCONDITIONALLY before
  `hook.Run`, so the misleading line appears for every no-op (source-gated `message`/`template`/`merge`/`squash`/
  `commit`, and empty staged diffs). Because git invokes `prepare-commit-msg` on every commit, a user with the
  hook installed sees this noise on every `git commit -m "…"`. The fix adds a nil-safe `Progress func()` callback
  to `generate.Deps`, invokes it inside `hook.Run` ONLY after BOTH the source gate and the empty-diff gate pass,
  and removes the unconditional `u.Progress(…)` call from `runHookExec` (setting the callback instead). No
  signature changes; backward-compatible (every other `generate.Deps` constructor leaves Progress nil —
  CommitStaged/decompose never call it). TDD: extend the source-gate test + add an empty-diff test asserting the
  progress line is absent on stderr for no-ops.

---

## Goal

**Feature Goal**: `hook exec` is silent on stderr for no-op cases (source-gated message sources and empty staged
diffs). The `↳ Generating…` progress line appears ONLY when generation is actually about to run (a non-empty
staged diff AND a non-no-op source).

**Deliverable**: Three coordinated code edits + two test changes:
1. `internal/generate/generate.go` — add `Progress func()` field to `generate.Deps` (after `Excludes`).
2. `internal/hook/exec.go` — invoke `deps.Progress()` inside `Run`, after the `diff == ""` gate and before Step C
   (parent/unborn), guarded by `if deps.Progress != nil`.
3. `internal/cmd/hookexec.go` — remove the unconditional `u.Progress(…)` call (L131); set the `Progress` callback
   on the `generate.Deps` literal passed to `hook.Run` (L133-138) instead.
4. `internal/cmd/hookexec_test.go` — extend `TestHookExec_SourceGateExit0` (assert progress absent) + add
   `TestHookExec_EmptyDiffNoProgress`.

**Success Definition**:
- `hook exec <msgfile> message` (source gate) → exit 0, msg-file untouched, stderr contains NO "Generating".
- `hook exec <msgfile>` with an empty staged diff → exit 0, msg-file untouched, stderr contains NO "Generating".
- `hook exec <msgfile>` with a real staged diff → the `↳ Generating…` line STILL appears on stderr (regression
  guard: optionally pinned by extending `TestHookExec_StrictFailureNonZero`).
- `go build ./...`, `go vet`, `go test ./...`, `golangci-lint` all pass. No signature changes; the default
  action / single-commit / decompose paths are unaffected (their `generate.Deps` leave `Progress` nil).

## Why

- **FR-H4** (PRD §9.20): `hook exec` must "Exit 0 immediately (no generation)" for no-op sources and empty diffs.
  Printing "Generating…" while doing nothing violates the spirit of that contract and is actively misleading.
- **User impact**: git invokes `prepare-commit-msg` on *every* commit, so a user with the hook installed sees the
  noise on every `git commit -m "manual"` (source=`message`) — even though no generation occurs. This is the most
  common hook path, making the cosmetic bug highly visible.
- **Why a callback (not a duplicated pre-check in runHookExec)**: `hook.Run` already owns both gates (source +
  empty-diff). Duplicating them in `runHookExec` would create a second source of truth that can drift. A nil-safe
  callback invoked once, after both gates pass, is the single source of truth: the line fires iff generation is
  truly about to run.

## What

No user-facing, config, API, or help-text surface change — this is an internal output-hygiene fix. The `hook exec
--help` text already says *"No-op (exit 0) when a message source is present … or nothing is staged"*; after this
fix, actual behavior matches it (stderr is silent on those paths). The exit-code mapping is unchanged
(`ErrNoOp` → exit 0).

### Success Criteria

- [ ] `generate.Deps` has a `Progress func()` field (nil-safe; comment cites hook.Run + CommitStaged never-calls).
- [ ] `hook.Run` calls `deps.Progress()` only after the source gate AND the empty-diff gate pass.
- [ ] `runHookExec` no longer calls `u.Progress(…)` unconditionally; it sets the `Progress` callback.
- [ ] Source-gate test asserts stderr has no "Generating".
- [ ] New empty-diff test asserts exit 0 + msg-file untouched + stderr has no "Generating".
- [ ] A real-diff path still prints "Generating" (regression guard).

## All Needed Context

### Context Completeness Check

_This PRP names every edit by file + line, the exact replacement text, the callback placement (after which gate,
before which step), the nil-safety argument with a per-callsite table, and the test patterns (helpers, stderr
capture, the chdir gotcha for the empty-diff test). An implementer with no codebase knowledge can complete it._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/docs/issue_analysis.md
  why: §Issue 2 is the root-cause analysis + fix design (the callback approach is the chosen design).
  section: "## Issue 2 (Minor): hook exec prints 'Generating…' progress line for no-op sources and empty diffs"
  critical: "The callback fires ONLY after BOTH gates pass (source-gate AND diff==''). CommitStaged never calls
             Progress — the default action owns its own progress printing."

- docfile: plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/P1M1T2S1/research/findings.md
  why: Verified line numbers, the ui.Progress output format ("↳ "+msg to stderr), the backward-compat callsite
       table, and the test-helper + chdir details.
  critical: "Empty-diff test MUST os.Chdir(repoDir) — StagedDiff runs against os.Getwd(); without chdir the diff
             target is the nondeterministic test process CWD."

- file: internal/cmd/hookexec.go
  why: runHookExec — the unconditional progress call (L131) to REMOVE + the generate.Deps literal (L133-138) to
       extend with the Progress callback. msgModel/labelProvider/u are already computed above (L129-130).
  pattern: |
    # REMOVE (L131):
    u.Progress(ui.ProgressLabel("Generating", msgModel, labelProvider))
    # ADD a field to the Deps literal (L134-137):
    Progress: func() { u.Progress(ui.ProgressLabel("Generating", msgModel, labelProvider)) },
  gotcha: "Keep `u` (the *ui.UI) and the labelProvider/msgModel vars exactly as computed — only move the CALL
           into the callback. Do not change the ui.New(...) line or color resolution."

- file: internal/hook/exec.go
  why: hook.Run — the two gates (source L99, empty-diff L113) and the insertion point (after L114, before L117).
  pattern: |
    if diff == "" { // FR-H4: empty staged diff → no-op.   (L113-114, unchanged)
        return ErrNoOp
    }
    // --- INSERT (after the empty-diff gate, before Step C) ---
    if deps.Progress != nil {
        deps.Progress() // emit "Generating…" now that generation is truly about to run
    }
    // Step C: parent / unborn.                              (L117, unchanged)
  gotcha: "Placement is load-bearing: AFTER diff=='' (so empty diffs are silent) and BEFORE the RevParseHEAD /
          prompt / generate work. If placed before the empty-diff gate the bug persists; the source gate (which
          returns earlier, L99) is already safely above."

- file: internal/generate/generate.go
  why: generate.Deps (L25-30) — add the Progress field. This is the only struct change; additive, zero-value nil.
  pattern: |
    type Deps struct {
        Git      git.Git
        Manifest provider.Manifest
        Verbose  *ui.Verbose
        Excludes []string
        Progress func() // optional; hook.Run calls it after no-op gates pass (nil-safe — never called by CommitStaged)
    }
  gotcha: "CommitStaged (the function in this same file) must NOT be modified — it has no progress concept. The
           default action prints its own progress (ui.Progress in default_action.go). Every other Deps constructor
           leaves Progress nil (see research table)."

- file: internal/cmd/hookexec_test.go
  why: The test PATTERN (stubtest.Build / hookexecNewTestRepo / writeTestStubConfig / resetRootCmd / errBuf capture)
       AND TestHookExec_SourceGateExit0 (L58) to extend + TestHookExec_StrictFailureNonZero (L97, chdir block) as
       the template for the new empty-diff test.
  pattern: |
    var outBuf, errBuf bytes.Buffer
    rootCmd.SetArgs([]string{"--config", cfg, "hook", "exec", msgFile /*, "message" */})
    rootCmd.SetOut(&outBuf); rootCmd.SetErr(&errBuf)
    err := rootCmd.ExecuteContext(context.TODO())
    # assertion to add: !bytes.Contains(errBuf.Bytes(), []byte("Generating"))
  gotcha: "The empty-diff test needs os.Chdir(repoDir)+defer restore (copy L109-115 from StrictFailureNonZero) and
           NO source arg (so it passes the source gate and reaches StagedDiff → empty → ErrNoOp). The source-gate
           test does NOT chdir (the gate fires before any git op) — leave it as-is."

- file: internal/ui/output.go
  why: Confirms ui.Progress writes "↳ "+msg+"\n" to u.stderr (L106) and ProgressLabel builds the body (L135). The
       assertion `!Contains(errBuf,"Generating")` covers both the resolved ("Generating with …") and bare forms.
  pattern: "func (u *UI) Progress(msg string) { fmt.Fprintln(u.stderr, progressPrefix+msg) }  // progressPrefix = \"↳ \""
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/generate.go   # generate.Deps (L25-30); CommitStaged (no Progress refs — grep-confirmed)
internal/hook/exec.go           # Run: source gate (L99), StagedDiff (L101), empty-diff gate (L113), Step C (L117)
internal/cmd/hookexec.go        # runHookExec: progress call (L131, REMOVE), generate.Deps literal (L133-138)
internal/cmd/hookexec_test.go   # TestHookExec_SourceGateExit0 (L58), TestHookExec_StrictFailureNonZero (L97)
internal/ui/output.go           # UI.Progress / ProgressLabel (no change — reference only)
```

### Desired Codebase tree with files to be added/changed

```bash
internal/generate/generate.go   # + Progress func() field on Deps
internal/hook/exec.go           # + deps.Progress() call after the empty-diff gate
internal/cmd/hookexec.go        # ~ remove L131 unconditional call; + Progress callback in the Deps literal
internal/cmd/hookexec_test.go   # ~ extend SourceGateExit0; + TestHookExec_EmptyDiffNoProgress; (opt) extend StrictFailureNonZero
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: place the callback AFTER the empty-diff gate (after `if diff == "" { return ErrNoOp }`, before Step C).
// Before the gate → the bug persists for empty diffs. The source gate (L99) already returns above, so it's safe.

// CRITICAL: the empty-diff test MUST os.Chdir(repoDir). runHookExec builds git.New(os.Getwd()); without chdir,
// StagedDiff runs against the test process CWD (nondeterministic staged state). Copy the chdir+defer block from
// TestHookExec_StrictFailureNonZero (hookexec_test.go L109-115).

// CRITICAL: do NOT modify CommitStaged. It has no progress concept; the default action prints progress itself
// (ui.Progress in default_action.go). generate.Deps.Progress is nil on the CommitStaged/decompose paths BY DESIGN.

// GOTCHA: keep the closure capturing u/msgModel/labelProvider exactly as computed today — only MOVE the call from
// a direct invocation into the `Progress: func() { … }` field. Do not recompute or reorder the ui.New(...) line.

// GOTCHA: assertion granularity. errBuf may legitimately contain OTHER stderr (e.g. config-load errors). Assert on
// the specific substring "Generating" (covers "↳ Generating with …" and "↳ Generating…"), NOT on errBuf emptiness.

// GOTCHA: zero-value-nil safety. Adding a func field is backward-compatible ONLY because every existing Deps
// constructor either sets it (hookexec, after this task) or leaves it nil (decompose/stagehand → CommitStaged,
// which never reads it). Do not add a non-nil default anywhere except hookexec.go.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/generate/generate.go — the one struct change (additive, nil-safe):
type Deps struct {
	Git      git.Git
	Manifest provider.Manifest
	Verbose  *ui.Verbose
	Excludes []string
	// Progress is an optional callback invoked by hook.Run after no-op gates pass (nil-safe — never called by
	// CommitStaged). runHookExec sets it to emit the "Generating…" line only when generation is about to run.
	Progress func()
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/generate/generate.go — add the Deps.Progress field
  - ADD `Progress func()` after the Excludes field (L29) with the doc comment above.
  - DO NOT touch CommitStaged or any other code in this file.
  - NAMING: `Progress` (matches ui.Progress method semantics). Type `func()` (no args — the closure captures all
    label values; keeps the field generic and the hook/exec.go side self-contained).

Task 2: MODIFY internal/hook/exec.go — invoke the callback after the gates
  - INSERT, immediately after `if diff == "" { return ErrNoOp }` (L113-114) and before `// Step C` (L117):
      if deps.Progress != nil {
          deps.Progress()
      }
  - Add a one-line comment: "Progress: generation is about to run (both no-op gates passed)."
  - FOLLOW pattern: the existing nil-guards on deps.Verbose (e.g. `if deps.Verbose != nil`) throughout Run.
  - GOTCHA: the callback runs once per Run invocation, AFTER StagedDiff succeeded AND diff is non-empty — never
    on the source-gate or empty-diff ErrNoOp paths.

Task 3: MODIFY internal/cmd/hookexec.go — remove the unconditional call, set the callback
  - DELETE L131: `u.Progress(ui.ProgressLabel("Generating", msgModel, labelProvider))`.
  - In the generate.Deps literal (L133-138), ADD the field (preserving Git/Manifest/Verbose/Excludes order):
      Progress: func() { u.Progress(ui.ProgressLabel("Generating", msgModel, labelProvider)) },
  - PRESERVE: the `u := ui.New(...)` line (L130) and labelProvider/msgModel computation (L129 + ResolveRoleModel).
    The closure captures them by reference; they are stable for the lifetime of runHookExec.
  - GOTCHA: `u` must still be constructed even though the call moves — color/UI may be used elsewhere or for
    future output; do not delete the ui.New line.

Task 4: MODIFY internal/cmd/hookexec_test.go — extend + add (TDD)
  - EXTEND TestHookExec_SourceGateExit0 (L58): after the existing assertions, add:
      if bytes.Contains(errBuf.Bytes(), []byte("Generating")) {
          t.Errorf("progress line must NOT appear for a source-gated no-op; stderr:\n%s", errBuf.String())
      }
  - ADD TestHookExec_EmptyDiffNoProgress:
      * stubBin := stubtest.Build(t); repoDir := hookexecNewTestRepo(t)  // seed commit, NO git add
      * cfg := writeTestStubConfig(t, stubBin)
      * msgFile := filepath.Join(t.TempDir(), "msg"); write "# comments\n"
      * os.Chdir(repoDir) + defer os.Chdir(origDir)  // copy from StrictFailureNonZero L109-115
      * rootCmd.SetArgs([]string{"--config", cfg, "hook", "exec", msgFile})  // NO source → reaches StagedDiff
      * SetOut/SetErr(&errBuf); ExecuteContext
      * assert err == nil (exit 0); msgFile == "# comments\n"; !bytes.Contains(errBuf, "Generating")
  - (OPTIONAL, recommended) EXTEND TestHookExec_StrictFailureNonZero: add
      `if !bytes.Contains(errBuf.Bytes(), []byte("Generating")) { t.Errorf("…progress SHOULD fire for a real diff…") }`
      to pin the positive case (real staged diff → progress DOES appear) as a regression guard.
  - FOLLOW pattern: defer resetRootCmd(); the existing SetArgs/SetOut/SetErr/ExecuteContext idiom.

Task 5: VERIFY — full build + vet + lint + tests
  - go build ./... ; go vet ./internal/generate/... ./internal/hook/... ./internal/cmd/...
  - go test ./internal/cmd/... -run 'HookExec' -v ; go test ./...
  - golangci-lint run ./internal/generate/... ./internal/hook/... ./internal/cmd/...
```

### Implementation Patterns & Key Details

```go
// hookexec.go — the move (direct call → closure on Deps). BEFORE:
u.Progress(ui.ProgressLabel("Generating", msgModel, labelProvider))   // ← delete this line
if rerr := hook.Run(ctx, generate.Deps{
    Git: g, Manifest: m, Verbose: verbose, Excludes: excludes,
}, *cfg, msgFile, source); rerr != nil && !errors.Is(rerr, hook.ErrNoOp) {

// AFTER:
if rerr := hook.Run(ctx, generate.Deps{
    Git:      g,
    Manifest: m,
    Verbose:  verbose,
    Excludes: excludes,
    Progress: func() { u.Progress(ui.ProgressLabel("Generating", msgModel, labelProvider)) },
}, *cfg, msgFile, source); rerr != nil && !errors.Is(rerr, hook.ErrNoOp) {

// hook/exec.go — the single insertion point (after the empty-diff gate, before Step C):
	if diff == "" { // FR-H4: empty staged diff → no-op.
		return ErrNoOp
	}

	// Progress: generation is about to run (both no-op gates passed). Nil-safe; CommitStaged leaves it nil.
	if deps.Progress != nil {
		deps.Progress()
	}

	// Step C: parent / unborn.
	_, isUnborn, err := deps.Git.RevParseHEAD(ctx)
```

### Integration Points

```yaml
UPSTREAM/DOWNSTREAM: none — fully self-contained. No config, no API, no help-text, no docs surface changes.
CALLSITES AFFECTED:
  - internal/cmd/hookexec.go (runHookExec): the ONLY producer of a non-nil Progress.
  - internal/hook/exec.go (Run): the ONLY consumer.
  - internal/decompose/decompose.go:247 + pkg/stagehand/stagehand.go:386: generate.Deps constructors for
    CommitStaged — UNCHANGED (Progress stays nil; CommitStaged never reads it). Verified by grep.
EXIT-CODE MAPPING: unchanged (ErrNoOp → exit 0). The fix only removes a stray stderr line; behavior is otherwise
  byte-identical.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagehand-competitor-feature-parity
gofmt -w internal/generate/generate.go internal/hook/exec.go internal/cmd/hookexec.go internal/cmd/hookexec_test.go
go build ./...
go vet ./internal/generate/... ./internal/hook/... ./internal/cmd/...
golangci-lint run ./internal/generate/... ./internal/hook/... ./internal/cmd/...
# Expected: zero errors.
```

### Level 2: Unit Tests (the contract)

```bash
go test ./internal/cmd/... -run 'HookExec' -v
# REQUIRED outcomes:
#  TestHookExec_SourceGateExit0: exit 0, msg-file unmodified, errBuf has NO "Generating" (the extended assertion).
#  TestHookExec_EmptyDiffNoProgress: exit 0, msg-file unmodified, errBuf has NO "Generating" (NEW).
#  TestHookExec_StrictFailureNonZero: exit 1 (strict), msg-file unmodified — still passes; (optional) progress
#    DID fire (errBuf contains "Generating") because there is a real staged diff.
go test ./...   # nothing else changed behavior
# Expected: full suite green. The only behavioral change is the removal of a stray stderr line on no-op paths.
```

### Level 3: Integration (manual repro from the issue)

```bash
# Build, set up a repo, install the hook (or call hook exec directly).
go build -o /tmp/stagehand ./cmd/stagehand
REPO=$(mktemp -d); cd "$REPO"; git init -q; git config user.email t@t; git config user.name t
echo hi > f; git add f; git commit -qm seed
printf 'config_version=3\n[provider.stub]\ncommand="/tmp/stub"\nprompt_delivery="stdin"\noutput="raw"\nstrip_code_fence=true\ndefault_model="x"\n' > /tmp/cfg.toml

# (a) Source-gated no-op: must be SILENT on stderr now.
printf '# c\n' > /tmp/m; /tmp/stagehand --config /tmp/cfg.toml hook exec /tmp/m message
echo "exit=$?"; grep -c Generating /dev/stderr 2>/dev/null || true   # stderr should have NO "Generating"

# (b) Empty-diff no-op: must be SILENT on stderr now.
printf '# c\n' > /tmp/m; /tmp/stagehand --config /tmp/cfg.toml hook exec /tmp/m
echo "exit=$?"   # 0; stderr should have NO "Generating"

# (c) Real diff (regression): progress SHOULD appear.
echo more >> f; git add f; printf '# c\n' > /tmp/m
/tmp/stagehand --config /tmp/cfg.toml --provider stub hook exec /tmp/m 2>/tmp/err
grep "Generating" /tmp/err   # expected: the "↳ Generating…" line IS present
# Expected: (a)/(b) silent; (c) prints "Generating". (c) may fail generation if /tmp/stub is absent — that's fine;
#   the point is the progress line appears BEFORE generation, proving the happy-path wiring survived.
```

### Level 4: Cross-cutting / Regression

```bash
go test ./internal/generate/... -v   # CommitStaged untouched; Deps additive field does not break generate tests
go test ./internal/hook/... -v       # hook.Run core logic untouched apart from the one guarded callback
go test -race ./internal/cmd/...     # the closure captures only read-only locals; no new shared state
# Expected: green. No new concurrency surface (the callback runs synchronously inside Run, single goroutine).
```

## Final Validation Checklist

### Technical
- [ ] `go build ./...`, `go vet`, `golangci-lint` clean; `gofmt` no diff.
- [ ] `go test ./...` green (incl. the new + extended hookexec tests).

### Feature
- [ ] Source-gated no-op → exit 0, msg-file untouched, stderr has NO "Generating".
- [ ] Empty-diff no-op → exit 0, msg-file untouched, stderr has NO "Generating".
- [ ] Real staged diff → "Generating" STILL appears (regression guard).
- [ ] Exit-code mapping unchanged (ErrNoOp → 0).

### Code Quality
- [ ] The callback is the single source of truth (no duplicated gate logic in runHookExec).
- [ ] `generate.Deps.Progress` is nil-safe (guarded `if deps.Progress != nil`); CommitStaged/decompose paths unaffected.
- [ ] No signature changes; no docs/help/config surface change.

### Scope Boundaries (do NOT cross)
- [ ] CommitStaged is NOT modified (no progress concept there).
- [ ] default_action.go / decompose paths are NOT touched (Progress stays nil there).
- [ ] No hook install/uninstall/status changes; no help-text edits.

---

## Anti-Patterns to Avoid
- ❌ Don't duplicate the NoOpSource/empty-diff checks in runHookExec — use the callback so hook.Run owns the gates.
- ❌ Don't place the callback BEFORE the `diff == ""` gate — the empty-diff path would still print "Generating".
- ❌ Don't modify CommitStaged or add a non-nil Progress default anywhere except hookexec.go.
- ❌ Don't assert on errBuf *emptiness* — assert on the specific "Generating" substring (other stderr may be valid).
- ❌ Don't forget `os.Chdir(repoDir)` in the empty-diff test — StagedDiff targets os.Getwd().
- ❌ Don't delete the `u := ui.New(...)` line — the closure still needs `u`.

---

## Confidence Score

**9.5/10** for one-pass success. This is a small, surgical, three-edit fix with a fully-specified design (the
callback approach is already the chosen design in issue_analysis.md), exact verified line numbers, a complete
backward-compatibility audit (every Deps constructor listed), and copy-ready test code (helpers + chdir block
exist to mirror). The only residual risk is the empty-diff test's `os.Chdir` detail (clearly flagged), and the
optional positive-progress assertion on StrictFailureNonZero (recommended but not blocking). No external
dependencies, no platform-specific code, no new APIs — the change is contained to three files in two packages.
