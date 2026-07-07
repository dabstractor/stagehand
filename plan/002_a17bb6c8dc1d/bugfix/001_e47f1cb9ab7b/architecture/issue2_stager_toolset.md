# Issue 2: Stager Toolset Not Actually Scoped (Major)

## Root Cause

PRD §19 claims the stager agent is "structurally constrained" — it "cannot commit, amend, or push,
because stagecoach owns every ref mutation." The shipped `tooled_flags` profiles do NOT deliver this:

### pi (`internal/provider/builtin.go`, `builtinPi()`)
```go
TooledFlags: []string{
    "--no-extensions",
    "--no-skills",
    "--no-prompt-templates",
    "--no-context-files",
    "--no-session",
},
```
This is pi's bare flags **MINUS `--no-tools`** — pi's entire native tool system is enabled with NO
allowlist. The inline comment admits: *"pi has no git-scoped allowlist … stager safety is via the
stager task prompt + stagecoach's ref-mutation monopoly."* A pi stager can run arbitrary Bash,
including `git commit`, `git push`, `git update-ref`, `rm -rf`.

### claude (`internal/provider/builtin.go`, `builtinClaude()`)
```go
TooledFlags: []string{
    "--allowed-tools", "Bash(git:*),Read,Edit",
    "--setting-sources", "",
    "--no-session-persistence",
},
```
`Bash(git:*)` permits **every** git subcommand — `git commit`, `git push --force`,
`git update-ref HEAD`, `git reset --hard HEAD~5` — so it does NOT prevent commit/amend/push.

## The Fix — Three-Pronged Approach

### (a) Tighten claude's tooled_flags allowlist
Change `Bash(git:*)` to an explicit staging-only allowlist:
```
Bash(git add:*,git apply:*,git status:*,git diff:*)
```
This restricts the claude stager to only staging-relevant git subcommands. `git commit`, `git push`,
`git update-ref`, `git reset`, `git rebase` etc. are all unreachable.

**Files**: `internal/provider/builtin.go` (`builtinClaude()`), `providers/claude.toml`.

### (b) Honestly document pi's unsoped toolset
pi has no git-scoped allowlist flag (`--help` shows only all-or-nothing `--no-tools`). We cannot
tighten pi's profile without disabling tools entirely (which would prevent the stager from running
git at all). The honest fix:
- Update the inline comment in `builtinPi()` and the prose in `providers/pi.toml` to clearly state
  the stager is **instructionally** constrained (via the §17.6 task prompt), not **structurally**
  constrained.
- The §19 "structurally constrained" claim should be qualified for pi.

**Files**: `internal/provider/builtin.go` (comment), `providers/pi.toml` (comment).

### (c) Add a defensive HEAD-movement guard
As defense-in-depth (covers pi's unsoped profile + any future provider), snapshot HEAD before each
stager invocation and abort if HEAD moved after the stager returns:

1. Before `stageConcept` (or `invokeStagerRetry`), capture `preStagerHEAD = RevParseHEAD()`.
2. After the stager returns successfully, re-read HEAD: `postStagerHEAD = RevParseHEAD()`.
3. If `preStagerHEAD != postStagerHEAD`, abort with an error (the stager moved a ref — safety
   violation). The arbiter/loop should treat this as a hard error (non-rescue — no snapshot to
   restore; the stager corrupted the repo state).

**Where to add the guard**: `internal/decompose/decompose.go`, in the `invokeStagerRetry` function
(or wrapping `invokeStager`), since that's where stager calls originate. The guard wraps the real
`stageConcept` call (not the test-seam `deps.stager`, which already stages via real git and would
trivially pass). Alternatively, add it inside `stageConcept` itself in `stager.go`.

**Consideration**: The stager mutates the INDEX (git add), NOT HEAD. A correctly-behaving stager
never moves HEAD. So this guard is a true safety invariant — any HEAD movement means a
misbehaving/corrupted stager.

## Test Strategy

1. **Unit test for claude allowlist**: Assert `builtinClaude().TooledFlags` contains the tightened
   `Bash(git add:*,...)` pattern (not `Bash(git:*)`). The existing `TestBuiltinClaude` /
   `TestRender_Tooled` tests will need their expected values updated.

2. **Unit test for HEAD guard**: Set up a decompose test where the stager test-seam moves HEAD
   (e.g. via `git update-ref`), and assert the loop aborts with the safety-violation error.

3. **Comment/documentation verification**: The `providers show pi` and `providers show claude`
   output should reflect the updated comments.

## Files to Touch

| File | Change | Doc Mode |
|------|--------|----------|
| `internal/provider/builtin.go` | Tighten claude `TooledFlags`; update pi comment | none (internal) |
| `providers/claude.toml` | Update `tooled_flags` + comment | Mode A (reference doc) |
| `providers/pi.toml` | Update `tooled_flags` comment honestly | Mode A (reference doc) |
| `internal/decompose/decompose.go` | Add HEAD-movement guard in stager path | JSDoc on guard fn |
| `internal/decompose/stager.go` | (Alternative guard location) | — |
| `internal/provider/builtin_test.go` | Update expected TooledFlags | — |
| `internal/decompose/decompose_test.go` | Add HEAD-guard violation test | — |
