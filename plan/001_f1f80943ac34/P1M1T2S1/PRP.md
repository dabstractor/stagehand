---
name: "P1.M1.T2.S1 — Git wrapper interface and command runner helper"
description: |
  Create `internal/git/git.go`: the `Git` interface (11 methods) + a `gitRunner` struct + the
  fully-working low-level `run(ctx, repo, args...) (stdout, stderr string, exitCode int, err error)`
  helper that wraps `exec.CommandContext` for the real `git` binary. Plus the `New(workDir) Git`
  constructor and two auxiliary types (`FileChange`, `StagedDiffOptions`). The 11 interface methods
  ship as panic-stubs now (S2–T3.S5 implement them); `run()` is the real, tested runtime deliverable.
  NO shell execution anywhere (PRD §19). First Go file in `internal/git/`.
---

## Goal

**Feature Goal**: Establish the type-safe, shell-free boundary to the real `git` binary — an
unexported `run()` helper that resolves git via `exec.LookPath`, targets a repo via the `-C` flag,
captures stdout/stderr to **separate** buffers, and returns git's exit code through the tuple
(`err` stays `nil` for non-zero exits so callers can read git's *inverted* exit-code semantics) —
together with the `Git` interface contract (all 11 plumbing methods) and a constructable
`gitRunner`/`New()` so every subsequent git subtask (S2–S6, T3.S1–S5) has a single, agreed
signature to implement against.

**Deliverable**: One new file — `internal/git/git.go` — containing:
1. The `Git` interface with 11 methods (`RevParseHEAD`, `WriteTree`, `CommitTree`, `UpdateRefCAS`,
   `DiffTree`, `StagedDiff`, `HasStagedChanges`, `RecentMessages`, `RecentSubjects`, `CommitCount`,
   `AddAll`).
2. Two auxiliary value types: `FileChange`, `StagedDiffOptions`.
3. The `gitRunner` struct (holds `workDir`) with the **fully implemented** `run()` helper.
4. All 11 methods as `panic("… not yet implemented — see P1.M1.T2.S<N>")` stubs (so `*gitRunner`
   satisfies `Git` and `New()` returns `Git`).
5. The `New(workDir string) Git` constructor.

Plus one test file — `internal/git/git_test.go` (package `git`) — proving `run()` works against a
real temp git repo (happy path, exit-code capture, separate stdout/stderr, LookPath-failure branch)
and that stubs panic. No other files touched.

**Success Definition**: `go build ./...` and `go vet ./...` exit 0; `go test -race ./internal/git/`
exits 0 with the `run()` cases passing against git 2.54.0; `New(".")` returns a non-nil `Git`
backed by `*gitRunner`; calling any of the 11 methods panics with a "not yet implemented" message
pointing at its owning subtask; `git grep -nE 'sh -c|zsh -c|cmd /c' internal/git/` finds nothing
(no shell execution — PRD §19).

## User Persona

**Target User**: The Stagecoach contributors implementing the rest of milestone P1.M1.T2/T3
(plumbing methods S2–S6 and diff/log/stage methods T3.S1–S5) and the P1.M3.T4 orchestrator that
will call them.

**Use Case**: Every git operation in Stagecoach flows through one `Git` instance obtained from
`New(workDir)`. Each subsequent subtask implements exactly one interface method by delegating to
the private `run()` helper — no subprocess plumbing is ever re-written, and every git invocation
is guaranteed shell-free and `-C repo`-scoped.

**User Journey**: `g := git.New(repoPath)` → (later) `sha, unborn, err := g.RevParseHEAD(ctx)` →
(later) `tree, err := g.WriteTree(ctx)` → … Each call is one typed line; the subprocess mechanics,
buffer capture, and exit-code extraction are centralized and tested once, here.

**Pain Points Addressed**: Eliminates per-method re-implementation of `exec.CommandContext`
boilerplate; eliminates the entire class of shell-injection bugs by forcing args-as-`[]string`;
encodes the unborn-repo (exit 128) and has-staged (exit 1) *traps* into the interface contract
rather than rediscovering them per method.

## Why

- **PRD §19 (Security):** *"Commands are built as `[]string` and run via `exec.Command` directly,
  never via `sh -c`."* This subtask is the structural enforcement of that rule for ALL git access —
  every method delegates to `run()`, which uses `exec.CommandContext` + `[]string` args only.
- **PRD §22.3 (Dependencies):** *"No git library dependency. Stagecoach shells out to the real `git`
  binary… go-git is tempting but adds a large dependency."* `run()` is the single shell-out point.
- **PRD §13 / §11.1 (Core IP):** The atomic snapshot flow (`rev-parse` → `write-tree` →
  `commit-tree` → `update-ref` CAS → `diff-tree`) is a sequence of plumbing calls. `run()` is the
  engine; the interface is the typed contract for each step. Steps 3–6 must never touch the index
  or HEAD between them — `run()` makes those calls composable and individually testable.
- **Foundation for P1.M1.T2.S2–S6 and P1.M1.T3.S1–S5:** every one of those subtasks' PRPs will say
  "implement `Git.<Method>` in `internal/git/git.go` delegating to `run()`". This subtask defines
  that method's exact signature and the `run()` it delegates to. Defining the full interface now
  (rather than growing it method-by-method) means the orchestrator (P1.M3.T4) can be written
  against a stable `Git` type from day one.
- **Guards against the two latent `commit-pi` bugs** documented in `critical_findings.md`: the
  unborn-`HEAD`-returns-`"HEAD"`-not-empty trap (FINDING 1) and the `diff --cached --quiet`
  inverted-exit-code trap (FINDING 6). Both are encoded in the interface contract (RevParseHEAD
  returns `isUnborn`; HasStagedChanges returns `bool`) and in `run()`'s "exit code ≠ Go error"
  invariant.

## What

A single Go file defining the typed git boundary, plus its internal test. No git *operations* are
implemented here beyond the generic `run()`; no CLI, no config, no provider code. The 11 methods
exist only to satisfy the interface and to lock their signatures for downstream subtasks.

### Success Criteria

- [ ] `internal/git/git.go` exists and declares `package git`.
- [ ] `Git` interface defined with exactly these 11 methods (signatures in §Implementation Blueprint):
      `RevParseHEAD`, `WriteTree`, `CommitTree`, `UpdateRefCAS`, `DiffTree`, `StagedDiff`,
      `HasStagedChanges`, `RecentMessages`, `RecentSubjects`, `CommitCount`, `AddAll`.
- [ ] `FileChange` and `StagedDiffOptions` value types defined (fields in §Blueprint).
- [ ] `gitRunner` struct defined with a `workDir string` field.
- [ ] `run()` is an **unexported** method on `*gitRunner` with the exact signature
      `run(ctx context.Context, repo string, args ...string) (stdout string, stderr string, exitCode int, err error)`
      and is fully implemented per §Blueprint (LookPath → `-C repo` args → separate buffers →
      `errors.As(*exec.ExitError)` → `err=nil` for non-zero exits).
- [ ] `New(workDir string) Git` returns a non-nil `Git` backed by `*gitRunner`.
- [ ] All 11 interface methods exist on `*gitRunner` as panic-stubs whose message names the owning
      subtask (e.g. `RevParseHEAD` → "see P1.M1.T2.S2").
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go test -race ./internal/git/` exits 0 (all `run()` cases pass against the real `git`).
- [ ] NO shell execution: `git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go`
      prints nothing.
- [ ] NO use of Go's `cmd.Dir` field or `os.Chdir`: `internal/git/git.go` never sets `cmd.Dir` and
      never calls `os.Chdir`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP gives the exact module path, the exact file to create, the exact
`run()` body (verified-equivalent to a throwaway program that was run against git 2.54.0), the
exact 11 interface signatures, the two auxiliary types, the exact stub pattern, the exact test
cases (each tied to an empirically-confirmed git behavior), and the exact validation commands with
expected exit codes. No inference required.

### Documentation & References

```yaml
# MUST READ — the work-item contract and the authoritative git-exec patterns
- file: PRD.md
  why: "§19 Security (no sh -c; args as []string), §22.3 Deps (no go-git; shell out to real git), §13/§11.1 (the atomic plumbing sequence these methods implement)."
  critical: "§19 is the NON-NEGOTIABLE rule this file enforces structurally. §22.3 forbids go-git — you MUST exec the real binary. This subtask owns ONLY git.go + its test; do NOT implement method bodies (S2–T3.S5) or touch any other package."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_summary.md
  why: "The implementation-facing cheat sheet: the atomic-commit sequence, the exit-code table, and the Cross-Platform Notes that pin the three invariants — (1) always `git -C <repo>` not os.Chdir, (2) capture stdout+stderr to SEPARATE buffers, (3) pass args as []string NEVER sh -c."
  critical: "The exit-code table (128=unborn, 1=staged, etc.) is the justification for run() returning exitCode separately with err=nil. The 'Always use git -C <repo>' note resolves the contract's ambiguous 'sets cmd.Dir via -C <repo>' phrasing (see gotcha G1)."

- docfile: plan/001_f1f80943ac34/architecture/git_plumbing_reference.md
  why: "Per-command Go os/exec patterns (§§1–7) — every example is exec.CommandContext(ctx,\"git\",\"-C\",repo,…). Use these as the reference each stubbed method will delegate to run() for."
  critical: "§4 (rev-parse unborn trap) and §5 (diff --cached --quiet inverted exit codes) are the two behaviors encoded directly into the interface contract (isUnborn bool; HasStagedChanges bool)."

- docfile: plan/001_f1f80943ac34/P1M1T2S1/research/run_helper_validation.md
  why: "EMPIRICALLY VERIFIED on git 2.54.0 + go1.26.4: the run() helper body, the four-tuple semantics (exit 128 → exitCode=128, err=nil), the unborn-repo stdout='HEAD\\n' trap, the separate-buffer proof, the LookPath-failure test via t.Setenv, and the full interface-signature table. READ THIS FIRST — it is the source of truth for every line of run()."
  critical: "Section 4.1 is the verified run() body. Section 5 is the verified method-signature table. Section 8 (Decisions Log) resolves every ambiguous contract point — D1 through D6 — that an implementing agent would otherwise have to guess at."

- docfile: plan/001_f1f80943ac34/architecture/critical_findings.md
  why: "FINDING 1 (unborn HEAD returns literal 'HEAD', detect via exit 128 not emptiness), FINDING 6 (diff --quiet exit 1 = staged, NOT an error). These two traps are why run() must NOT turn non-zero exits into Go errors."
  critical: "FINDING 8 (signal handling needs process-group kill) is NOT this subtask's concern (that is P1.M2.T5/P1.M4.T2) — run() here does NOT set SysProcAttr/Setpgid; the agent subprocess is a different package. Do not pre-empt that work."

- docfile: plan/001_f1f80943ac34/P1M1T1S1/PRP.md
  why: "The CONTRACT for the inputs: module path github.com/dustin/stagecoach, go 1.22, the §14 directory tree (internal/git/ already exists as an EMPTY dir), main.go stub. Confirms the module compiles today and internal/git/ is waiting for this file."
  critical: "internal/git/ already exists (empty) — DO NOT mkdir it, just create git.go inside. The module path is github.com/dustin/stagecoach, so the package's import path is github.com/dustin/stagecoach/internal/git."

- docfile: plan/001_f1f80943ac34/P1M1T1S2/PRP.md
  why: "The CONTRACT for the validation surface: `make test` = `go test -race ./...`, `make build`, `make lint` (golangci-lint, absent on this box — use `go vet`/`gofmt` as the local gate). This subtask's tests are exercised via `make test`."
  critical: "golangci-lint is NOT installed locally (S2 verified). For lint-equivalent local validation use `go vet ./...` and `gofmt -l .`; the CI matrix (P1.M5.T3.S1) runs golangci-lint later. Do NOT add a .golangci.yml."

# External references (exact, anchor-level)
- url: https://pkg.go.dev/os/exec#CommandContext
  why: "CommandContext creates a Cmd cancelled by ctx; Cmd.Stdout/Cmd.Stderr accept io.Writer (a *bytes.Buffer); a non-zero exit surfaces as *exec.ExitError. This is the entire surface run() uses."
  critical: "exec.CommandContext(ctx, name, args...) does NOT spawn a shell — args are passed to execve directly. This is HOW §19's no-shell rule is satisfied."
- url: https://pkg.go.dev/os/exec#ExitError
  why: "ExitError.ExitCode() returns the child's exit status; it is returned by Cmd.Run() ONLY for non-zero exits (exit 0 → Run returns nil error)."
  critical: "This is why run() does errors.As(runErr, &exitErr) to extract the code, and why a non-zero exit is recoverable as a (code, nil) tuple instead of an opaque error."
- url: https://pkg.go.dev/os/exec#LookPath
  why: "LookPath resolves a binary name to an absolute path using PATH; returns exec.ErrNotFound (Go 1.19+) when absent."
  critical: "run() calls LookPath(\"git\") and returns a clear error if git is missing — testable via t.Setenv(\"PATH\",\"\")."
- url: https://git-scm.com/docs/git-commit-tree#_options
  why: "Documents -F - (read message from stdin), -p (repeatable parent; omit for root commit). Confirms the CommitTree(msg-via-stdin, parents []string) contract S4 will implement."
- url: https://git-scm.com/docs/git-update-ref
  why: "Documents the 3-arg compare-and-swap form and the all-zeros expected-old value for unborn refs. Confirms the UpdateRefCAS(ref, new, expectedOld) contract S5 will implement (never the 2-arg force form)."
```

### Current Codebase Tree (after P1.M1.T1.S1 has landed)

```bash
stagecoach/
├── .gitignore            # from S1 — has /bin/ *.test coverage.out /dist/
├── PRD.md
├── go.mod                # from S1 — module github.com/dustin/stagecoach, go 1.22 (no deps)
├── Makefile              # from S2 — build/test/lint/coverage/install/clean targets
├── cmd/stagecoach/main.go # from S1 — stub: package main; func main(){}
├── internal/             # from S1 — empty package dirs; internal/git/ EXISTS and is EMPTY
│   ├── config/           #   empty ← P1.M1.T4
│   ├── provider/         #   empty ← P1.M2
│   ├── prompt/           #   empty ← P1.M3.T1
│   ├── git/              #   empty ← THIS subtask writes git.go here
│   ├── generate/         #   empty ← P1.M3.T4
│   └── ui/               #   empty ← P1.M4.T3
├── pkg/stagecoach/        # from S1 — empty
├── providers/            # from S1 — empty
├── docs/                 # from S1 — empty
└── plan/                 # unchanged (planning artifacts)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
├── ... (everything above, unchanged)
└── internal/
    └── git/
        ├── git.go        # NEW — Git interface + FileChange + StagedDiffOptions + gitRunner + run() + New() + 11 stubs
        └── git_test.go   # NEW — package git; tests run() against a real temp git repo + stub-panic tests
```

**File responsibilities:**
| Path | Action | Responsibility |
|---|---|---|
| `internal/git/git.go` | NEW | The `Git` interface, two value types, `gitRunner` struct, fully-working `run()` helper, `New()` constructor, and 11 panic-stub methods. |
| `internal/git/git_test.go` | NEW | Internal-package tests for `run()` (happy path, exit-code capture, separate buffers, LookPath failure) + a stub-panic assertion. |

**Explicitly NOT created/modified now:** any method body for the 11 methods (those are S2–S6,
T3.S1–S5); `SysProcAttr`/`Setpgid`/process-group code (P1.M2.T5 / P1.M4.T2 signal handling); the
agent executor (P1.M2.T5); `go.mod`/`go.sum` (no new deps — stdlib only); `.golangci.yml` or CI
files (P1.M5.T3); any file under `cmd/`, `pkg/`, or other `internal/*` packages.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — contract phrasing): the contract says run() "sets cmd.Dir via -C <repo>".
// This is LOOSE phrasing. It means "sets the working directory via the -C FLAG", NOT "sets Go's
// cmd.Dir field". Resolution (research D1): pass "-C", repo as the first two args and DO NOT set
// cmd.Dir or call os.Chdir. Every architecture example (git_plumbing_reference.md §§1–7) does
// exec.CommandContext(ctx,"git","-C",repo,…) and NEVER sets cmd.Dir. Setting cmd.Dir would be the
// exec equivalent of os.Chdir, which the summary explicitly warns against for goroutine safety.

// CRITICAL (G2 — the core invariant): run() must return err == nil for NON-ZERO git exits.
// Git encodes meaning in exit codes: 1 = has-staged-changes; 128 = unborn-repo / not-a-SHA.
// If run() turned those into Go errors, every caller would re-errors.As() to recover the code —
// defeating the (stdout,stderr,exitCode,err) tuple. VERIFIED: on an unborn repo,
// run(ctx,repo,"rev-parse","HEAD") returns exitCode=128, err=nil, stdout="HEAD\n". A naive
// `if err := cmd.Run(); err != nil { return err }` is WRONG here.

// CRITICAL (G3 — unborn-repo trap, FINDING 1): on a repo with zero commits, `git rev-parse HEAD`
// prints the LITERAL STRING "HEAD\n" to stdout (NOT empty!) and exits 128. A naive
// `if stdout == "" { unborn }` check is a latent bug. The interface encodes the fix:
// RevParseHEAD returns (sha string, isUnborn bool, err error); the S2 impl detects unborn via
// exitCode==128, NOT stdout emptiness. run() must surface that 128 in exitCode (see G2).

// CRITICAL (G4 — has-staged trap, FINDING 6): `git diff --cached --quiet` exits 1 when changes
// ARE staged (NOT an error) and 0 when nothing is staged. The interface encodes the fix:
// HasStagedChanges returns (bool, error); the T3.S2 impl reads exitCode==1 → true. Again, run()
// surfacing exit 1 as err=nil (G2) is what makes this clean.

// GOTCHA (G5 — LookPath location): New(workDir string) Git has NO error return, so you cannot
// resolve the git binary in the constructor without panicking (bad). Resolution (research D2):
// call exec.LookPath("git") INSIDE run(); on failure return ("","",-1, err). This keeps New()
// clean and surfaces a missing git as a runtime error from run(). (LookPath is cheap; calling it
// per invocation is fine. Caching it on the struct is an optional micro-optimization, not required.)

// GOTCHA (G6 — exit code sentinel): use exitCode = -1 to mean "could not determine a real exit
// code" (LookPath miss, context cancelled, start/I/O failure). 0 = success; ≥1 = git's real code.
// Callers that switch on exitCode must treat -1 as "err is authoritative" (err != nil in that case).

// GOTCHA (G7 — why stubs panic, not return errors): the 11 methods must exist on *gitRunner for
// New() to return Git (Go requires full interface satisfaction at compile time). They ship as
// panic-stubs naming the owning subtask. Panic (not error) fails FAST and unambiguously — a silent
// error invites the orchestrator to proceed with zero values. THIS subtask's tests exercise run()
// (fully implemented) + the panic behavior, so stubs do not block a green test suite.

// GOTCHA (G8 — run() is NOT in the interface): run() is an unexported "helper" on *gitRunner.
// It cannot be in the exported Git interface (an unexported method would make Git unsatisfiable
// outside package git). It is exercised ONLY by the internal test `package git`. The 11 exported
// methods are the interface surface; run() is the private engine they delegate to (in S2–T3.S5).

// GOTCHA (G9 — test package must be `git`, not `git_test`): because run() and gitRunner.workDir
// are unexported, the test file MUST be `package git` (white-box) to call them. A `package git_test`
// (black-box) test could only see the exported Git interface, which is insufficient to test run().

// GOTCHA (G10 — testing LookPath failure without uninstalling git): Go 1.17+ provides
// t.Setenv("PATH","") which makes exec.LookPath("git") fail for that test. Use it; restore is
// automatic. (Do NOT mutate os.Environ globally or call os.Setenv directly — t.Setenv is
// race-detector-safe and auto-restores.)

// GOTCHA (G11 — no deps): this file uses ONLY the Go stdlib (context, os/exec, bytes, errors,
// fmt, strings). Do NOT run `go get`; do NOT edit go.mod; no go.sum is generated. (PRD §22.3.)

// GOTCHA (G12 — context cancellation): when ctx is cancelled, cmd.Run() returns a non-ExitError.
// run() checks ctx.Err() BEFORE errors.As(ExitError) and returns (-1, ctx.Err()). This subtask
// does NOT install signal handlers or set SysProcAttr/Setpgid (that is P1.M4.T2); exec's default
// context cancellation (kill the direct child) is sufficient for git plumbing calls.
```

## Implementation Blueprint

### Data models and structure

```go
// FileChange is one entry in a diff-tree "what landed" listing.
// diff-tree --name-status -r emits "<status>\t<path>" or "<status>\t<src>\t<dst>" (rename/copy).
// The S6 (DiffTree) implementation parses these lines into FileChange values.
type FileChange struct {
    Status  string // "A","M","D","R","C","T","U"; R/C carry a similarity score e.g. "R100"
    SrcPath string // non-empty only for R/C (the rename/copy source); "" otherwise
    Path    string // the destination path — always set
}

// StagedDiffOptions configures staged-diff capture (commit-pi parity, PRD §9.1 / FINDING 7).
// The T3.S1 (StagedDiff) implementation consumes these.
type StagedDiffOptions struct {
    MaxDiffBytes int      // byte cap on the non-markdown section (commit-pi default 300000); 0 = unlimited
    MaxMDLines   int      // per-file line cap for markdown files (commit-pi default 100); 0 = unlimited
    Excludes     []string // pathspec magic-prefix excludes, e.g. []string{":!*.lock", ":!vendor/*"}
}
```

> These are value types now (no methods); they exist so the interface signatures compile. Their
> fields are forward-looking but match commit-pi's documented behavior so T3.S1 / S6 do not need
> to amend the interface later.

### The `Git` interface (exact — copy verbatim)

```go
package git

import "context"

// Git is the shell-free boundary to the real git binary. Every method delegates to the private
// run() helper on *gitRunner, which execs git with args as []string (NEVER sh -c — PRD §19) and
// targets the repo via the -C flag (NEVER os.Chdir — goroutine-safe).
//
// Method ownership (each implemented in its own later subtask):
//   RevParseHEAD      — P1.M1.T2.S2   WriteTree        — P1.M1.T2.S3
//   CommitTree        — P1.M1.T2.S4   UpdateRefCAS     — P1.M1.T2.S5
//   DiffTree          — P1.M1.T2.S6
//   StagedDiff        — P1.M1.T3.S1   HasStagedChanges — P1.M1.T3.S2
//   RecentMessages    — P1.M1.T3.S3   CommitCount      — P1.M1.T3.S3
//   RecentSubjects    — P1.M1.T3.S4   AddAll           — P1.M1.T3.S5
type Git interface {
    // RevParseHEAD returns the SHA HEAD points at. On a repo with zero commits it returns
    // sha="" and isUnborn=true (detected via git exit 128, NOT stdout emptiness — FINDING 1).
    RevParseHEAD(ctx context.Context) (sha string, isUnborn bool, err error)

    // WriteTree materializes the index into a tree object and returns its SHA. Fails (non-nil err)
    // when the index has unresolved merge conflicts (git exit 128).
    WriteTree(ctx context.Context) (sha string, err error)

    // CommitTree creates a commit object for tree with the given parents and message (delivered
    // via stdin with -F -). parents==nil/empty ⇒ root commit (no -p). Returns the new commit SHA.
    CommitTree(ctx context.Context, tree string, parents []string, msg string) (sha string, err error)

    // UpdateRefCAS atomically moves ref to newSHA only if it currently equals expectedOld
    // (3-arg compare-and-swap; NEVER the 2-arg force form). For a root commit pass expectedOld =
    // the all-zeros hash. Returns a non-nil err on CAS mismatch (HEAD moved).
    UpdateRefCAS(ctx context.Context, ref, newSHA, expectedOld string) error

    // DiffTree returns the file-level change set of sha vs its first parent ("what landed").
    // isRoot must be true for a root commit so git diffs against the empty tree (--root flag).
    DiffTree(ctx context.Context, sha string, isRoot bool) ([]FileChange, error)

    // StagedDiff returns the staged diff payload (markdown per-file + non-markdown aggregate),
    // applying byte/line caps and pathspec excludes per opts (commit-pi parity, PRD §9.1).
    StagedDiff(ctx context.Context, opts StagedDiffOptions) (diff string, err error)

    // HasStagedChanges reports whether the index differs from HEAD (git diff --cached --quiet:
    // exit 1 ⇒ true, exit 0 ⇒ false). NOT an error when changes exist (FINDING 6).
    HasStagedChanges(ctx context.Context) (bool, error)

    // RecentMessages returns up to n most-recent full commit messages (NUL-delimited query,
    // FINDING 9). Callers must short-circuit when RevParseHEAD reports isUnborn.
    RecentMessages(ctx context.Context, n int) (messages []string, err error)

    // RecentSubjects returns up to n most-recent commit subjects (first line) for duplicate
    // detection. Callers must short-circuit when isUnborn.
    RecentSubjects(ctx context.Context, n int) (subjects []string, err error)

    // CommitCount returns the number of commits reachable from HEAD (decides mature vs new-repo
    // prompt). Callers must short-circuit when isUnborn.
    CommitCount(ctx context.Context) (count int, err error)

    // AddAll stages all changes (git add -A). Used by the auto-stage-all path (PRD §9.4 / FINDING 11).
    AddAll(ctx context.Context) error
}
```

### The struct, `run()`, `New()`, and the stubs (exact — copy verbatim)

```go
import (
    "bytes"
    "context"
    "errors"
    "fmt"
    "os/exec"
)

// gitRunner is the production Git implementation. It wraps exec.CommandContext for the real git
// binary. Construct with New.
type gitRunner struct {
    workDir string // the repo path passed as -C <repo> by every bound method
}

// New returns a Git bound to workDir. The git binary is resolved lazily inside run() (New has no
// error return); a missing git surfaces as a runtime error from the first run() call.
func New(workDir string) Git {
    return &gitRunner{workDir: workDir}
}

// run is the low-level git exec helper. It is the ONLY place Stagecoach shells out to git.
//   - resolves the git binary via exec.LookPath (PRD §19: real binary, never go-git per §22.3)
//   - targets repo via the -C flag (NOT os.Chdir / cmd.Dir — goroutine-safe)
//   - captures stdout and stderr to SEPARATE buffers
//   - returns the exit code extracted from *exec.ExitError
//
// INVARIANT: a NON-ZERO git exit is returned as (stdout, stderr, exitCode, nil) — err is nil.
// Git uses exit codes as semantic signals (1 = has-staged; 128 = unborn/not-a-SHA), and callers
// inspect exitCode. Only infrastructural failures (LookPath miss, context cancel, start/I/O)
// return err != nil, with exitCode = -1.
func (g *gitRunner) run(ctx context.Context, repo string, args ...string) (stdout string, stderr string, exitCode int, err error) {
    gitPath, lerr := exec.LookPath("git")
    if lerr != nil {
        return "", "", -1, fmt.Errorf("git binary not found in PATH: %w", lerr)
    }

    full := make([]string, 0, len(args)+2)
    full = append(full, "-C", repo) // repo via flag, not cmd.Dir (gotcha G1)
    full = append(full, args...)

    cmd := exec.CommandContext(ctx, gitPath, full...) // []string args, NO shell (PRD §19)
    var out, errb bytes.Buffer
    cmd.Stdout = &out  // separate buffer
    cmd.Stderr = &errb // separate buffer

    runErr := cmd.Run()
    stdout, stderr = out.String(), errb.String()

    if runErr == nil {
        return stdout, stderr, 0, nil
    }
    if cerr := ctx.Err(); cerr != nil { // context cancelled (timeout/signal) — not a git exit
        return stdout, stderr, -1, cerr
    }
    var exitErr *exec.ExitError
    if errors.As(runErr, &exitErr) { // non-zero git exit → capture code, err stays nil (gotcha G2)
        return stdout, stderr, exitErr.ExitCode(), nil
    }
    return stdout, stderr, -1, runErr // start / I/O failure
}

// ---- Stubs: each method is implemented in its own later subtask. They panic to fail fast. ----

func (g *gitRunner) RevParseHEAD(ctx context.Context) (string, bool, error) {
    panic("gitRunner.RevParseHEAD: not yet implemented — see P1.M1.T2.S2")
}
func (g *gitRunner) WriteTree(ctx context.Context) (string, error) {
    panic("gitRunner.WriteTree: not yet implemented — see P1.M1.T2.S3")
}
func (g *gitRunner) CommitTree(ctx context.Context, tree string, parents []string, msg string) (string, error) {
    panic("gitRunner.CommitTree: not yet implemented — see P1.M1.T2.S4")
}
func (g *gitRunner) UpdateRefCAS(ctx context.Context, ref, newSHA, expectedOld string) error {
    panic("gitRunner.UpdateRefCAS: not yet implemented — see P1.M1.T2.S5")
}
func (g *gitRunner) DiffTree(ctx context.Context, sha string, isRoot bool) ([]FileChange, error) {
    panic("gitRunner.DiffTree: not yet implemented — see P1.M1.T2.S6")
}
func (g *gitRunner) StagedDiff(ctx context.Context, opts StagedDiffOptions) (string, error) {
    panic("gitRunner.StagedDiff: not yet implemented — see P1.M1.T3.S1")
}
func (g *gitRunner) HasStagedChanges(ctx context.Context) (bool, error) {
    panic("gitRunner.HasStagedChanges: not yet implemented — see P1.M1.T3.S2")
}
func (g *gitRunner) RecentMessages(ctx context.Context, n int) ([]string, error) {
    panic("gitRunner.RecentMessages: not yet implemented — see P1.M1.T3.S3")
}
func (g *gitRunner) RecentSubjects(ctx context.Context, n int) ([]string, error) {
    panic("gitRunner.RecentSubjects: not yet implemented — see P1.M1.T3.S4")
}
func (g *gitRunner) CommitCount(ctx context.Context) (int, error) {
    panic("gitRunner.CommitCount: not yet implemented — see P1.M1.T3.S3")
}
func (g *gitRunner) AddAll(ctx context.Context) error {
    panic("gitRunner.AddAll: not yet implemented — see P1.M1.T3.S5")
}
```

> **Every code block above is verified** — the `run()` body is byte-equivalent to a throwaway Go
> program executed against git 2.54.0 (research §4.1), and the stub/interface shapes follow the
> verified signature table (research §5).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/git/git.go (single file, package git)
  - FILE: internal/git/git.go
  - WRITE the file with EXACTLY these top-level declarations, in this order:
      1. package git + imports (bytes, context, errors, fmt, os/exec)
      2. type FileChange struct { Status, SrcPath, Path string }
      3. type StagedDiffOptions struct { MaxDiffBytes, MaxMDLines int; Excludes []string }
      4. type Git interface { …11 methods… }  (verbatim from "The Git interface" above)
      5. type gitRunner struct { workDir string }
      6. func New(workDir string) Git { return &gitRunner{workDir: workDir} }
      7. func (g *gitRunner) run(ctx, repo, args…) (string,string,int,error)  (verbatim body above)
      8. the 11 stub methods (verbatim from "struct, run(), New(), stubs" above)
  - NAMING: package `git` (matches the dir); file `git.go`. Struct `gitRunner` (unexported);
    constructor `New` (exported); helper `run` (unexported); interface `Git` (exported).
  - PLACEMENT: internal/git/git.go (the dir already exists from S1 — do NOT mkdir).
  - DEPENDENCIES: stdlib only. Do NOT edit go.mod / run go get.
  - VERIFY compile: `go build ./internal/git/` → exit 0.

Task 2: CREATE internal/git/git_test.go (package git — white-box, to reach run() and gitRunner)
  - FILE: internal/git/git_test.go
  - PACKAGE line: `package git`  (NOT git_test — gotcha G9)
  - IMPORTS: bytes, context, os, os/exec, strings, testing
  - WRITE these test functions (each tied to a verified behavior):
      TestNew:
        - g := New("/tmp"); assert g != nil
        - assert _, ok := g.(*gitRunner); ok == true
        - assert g.(*gitRunner).workDir == "/tmp"
      initRepo(t, dir): helper — run `git -C dir init` via exec.Command directly with a minimal
        env (PATH, HOME, GIT_*_NAME/EMAIL) so identity is set; t.Helper(). Used by the run() tests.
      TestRun_HappyPath:
        - repo := t.TempDir(); initRepo(t, repo)
        - stdout, stderr, code, err := g.run(ctx, repo, "rev-parse", "--git-dir")
        - assert err == nil, code == 0, strings.TrimSpace(stdout) == ".git", stderr == ""
      TestRun_CapturesExitCodeAndSeparateBuffers:
        - repo := t.TempDir(); initRepo(t, repo)   // UNBORN (zero commits)
        - stdout, stderr, code, err := g.run(ctx, repo, "rev-parse", "HEAD")
        - assert err == nil            (gotcha G2: exit 128 is NOT a Go error)
        - assert code == 128
        - assert strings.TrimSpace(stdout) == "HEAD"   (gotcha G3: the literal trap string)
        - assert stderr != "" && strings.Contains(stderr, "ambiguous argument 'HEAD'")
          (proves stderr captured separately — if combined into stdout this fails)
      TestRun_LookPathFailure:
        - t.Setenv("PATH", "")   // gotcha G10: makes LookPath("git") fail for this test only
        - _, _, code, err := g.run(ctx, ".", "version")
        - assert err != nil && strings.Contains(err.Error(), "git binary not found")
        - assert code == -1
      TestStubsPanic:
        - for each of the 11 methods, call it inside a func that recovers; assert it panicked
          and the message contains "not yet implemented". (Use a small helper that takes a func().)
          Example for one: assertPanics(t, "RevParseHEAD", func() { _,_,_ = g.RevParseHEAD(ctx) })
  - NAMING: test funcs TestXxx; helper initRepo; assertion helper assertPanics(t, name, func()).
  - COVERAGE: run() happy path + exit-code/separate-buffer + LookPath branch + stub panic.
  - VERIFY: `go test -race ./internal/git/` → exit 0, all pass.

Task 3: VALIDATE — run the full gate set (see Validation Loop) and confirm scope discipline
  - RUN: go build ./... ; go vet ./... ; gofmt -l internal/git/ ; go test -race ./internal/git/
  - RUN: git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go   (expect: no matches)
  - RUN: git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go                    (expect: no matches)
  - RUN: git status --porcelain → expect ONLY internal/git/git.go and internal/git/git_test.go.
  - DO NOT: edit go.mod, add deps, touch other packages, or implement any method body.
```

### Implementation Patterns & Key Details

```go
// === run() invariant in action (the thing every stubbed method will rely on) ===
// When S2 implements RevParseHEAD, it will do:
//   stdout, _, code, err := g.run(ctx, g.workDir, "rev-parse", "HEAD")
//   if err != nil { return "", false, err }          // LookPath/start failure only
//   if code == 128 { return "", true, nil }          // unborn — exit-code signal, NOT an error
//   if code != 0  { return "", false, fmt.Errorf("git rev-parse HEAD: exit %d", code) }
//   return strings.TrimSpace(stdout), false, nil
// Note err is nil at code==128 — that is the whole point of the four-tuple return.

// === Why append into a freshly-made slice (not the variadic args directly) ===
// `full := make([]string, 0, len(args)+2); full = append(full, "-C", repo); full = append(full, args...)`
// Prepending "-C", repo to a variadic MUST use a new slice: append(args, ...) would mutate the
// caller's backing array if it had capacity. Allocating a fresh slice is safe and clear.

// === Why errors.As, not a type assertion ===
// `var exitErr *exec.ExitError; if errors.As(runErr, &exitErr)` unwraps wrapped errors and is the
// forward-compatible form (vs `runErr.(*exec.ExitError)` which breaks if exec ever wraps). Use As.

// === Why context cancellation is checked BEFORE ExitError ===
// On ctx.Done(), exec kills the child; cmd.Run() may return either an *exec.ExitError (killed →
// signal exit) or a plain error. We want ctx cancellation to surface as err=ctx.Err() (exitCode=-1)
// so callers distinguish "you cancelled" from "git returned 128". Check ctx.Err() first.
```

### Integration Points

```yaml
MODULE (consumed, not modified):
  - module path: "github.com/dustin/stagecoach" → package import path "github.com/dustin/stagecoach/internal/git"
  - go directive: 1.22 → context, os/exec, errors.As, t.Setenv (1.17+) all available
  - deps: NONE added (stdlib only)

INTERNAL/GIT PACKAGE (created here):
  - file: internal/git/git.go      # interface + struct + run() + New() + stubs
  - file: internal/git/git_test.go # package git white-box tests
  - the dir already exists (empty) from P1.M1.T1.S1 — do NOT mkdir

LATER-SUBTASK HOOKS (informational — do NOT implement now; each REPLACES one stub):
  - P1.M1.T2.S2: implements RevParseHEAD  (exit 128 → isUnborn)
  - P1.M1.T2.S3: implements WriteTree      (exit 128 → conflict detection)
  - P1.M1.T2.S4: implements CommitTree     (-F - stdin; parents []string controls -p)
  - P1.M1.T2.S5: implements UpdateRefCAS   (3-arg CAS, all-zeros for root)
  - P1.M1.T2.S6: implements DiffTree       (--root for root; parses into []FileChange)
  - P1.M1.T3.S1: implements StagedDiff     (consumes StagedDiffOptions; caps + excludes)
  - P1.M1.T3.S2: implements HasStagedChanges (exit 1 → true)
  - P1.M1.T3.S3: implements RecentMessages + CommitCount (NUL-delimited; rev-list --count)
  - P1.M1.T3.S4: implements RecentSubjects  (%s one-per-line)
  - P1.M1.T3.S5: implements AddAll         (git add -A)
  - P1.M3.T4:    orchestrator calls these via the Git interface (stable from day one because the
                 full interface is defined here)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l internal/git/          # Expected: no output (run `gofmt -w internal/git/` if it lists files)
go vet ./internal/git/...        # Expected: exit 0, no warnings
go build ./internal/git/         # Expected: exit 0 (package compiles standalone)
go build ./...                   # Expected: exit 0 (whole module compiles)

# Expected: Zero output/errors. gofmt is the formatting gate (golangci-lint is absent locally —
# see gotcha / S2's PRP); `go vet` catches shadowed vars, unreachable code, etc.
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

go test -race -v ./internal/git/   # Expected: all tests PASS, exit 0
# Must see: TestNew, TestRun_HappyPath, TestRun_CapturesExitCodeAndSeparateBuffers,
#           TestRun_LookPathFailure, TestStubsPanic — all ok.

# Or via the Makefile target from S2:
make test                          # Expected: exit 0 (runs go test -race ./...; internal/git passes)

# Expected: all pass. If TestRun_CapturesExitCodeAndSeparateBuffers fails on code!=128, the box's
# git differs — re-run `git rev-parse HEAD; echo $?` in an empty `git init` dir to re-pin the code
# and update the assertion. (Verified 128 on git 2.54.0.)
```

### Level 3: Security & Structural Invariants (the §19 enforcement)

```bash
cd /home/dustin/projects/stagecoach

# PRD §19: NO shell execution anywhere in the git wrapper.
git grep -nE '\b(sh|zsh|bash)\s+-c\b|cmd\s*/c\b' internal/git/git.go
# Expected: NO output (no matches). A match is a §19 violation.

# No os.Chdir / cmd.Dir — repo is targeted via the -C flag only (goroutine-safe).
git grep -nE 'cmd\.Dir|os\.Chdir' internal/git/git.go
# Expected: NO output (no matches).

# LookPath is present (contract: resolve git binary).
git grep -n 'exec.LookPath' internal/git/git.go
# Expected: exactly one match, inside run().

# exec.CommandContext is the only exec entry point.
git grep -n 'exec.CommandContext' internal/git/git.go
# Expected: exactly one match, inside run().

# All 11 interface methods exist on gitRunner (so New() returns Git and the package compiles).
go doc -all ./internal/git 2>/dev/null | grep -E 'func \(\*gitRunner\)' | wc -l
# Expected: 12 (run + 11 stubs).
```

### Level 4: Runtime Smoke Test (prove run() works against a real repo)

```bash
cd /home/dustin/projects/stagecoach

# Build a tiny throwaway program that exercises run() exactly as the test does, to confirm the
# real binary path end-to-end (this mirrors the research verification):
cat > /tmp/smoke_git.go <<'EOF'
package main
import ("bytes";"context";"errors";"fmt";"os";"os/exec")
func run(ctx context.Context, repo string, args ...string)(string,string,int,error){
  p,e:=exec.LookPath("git"); if e!=nil{return "","",-1,fmt.Errorf("lookpath: %w",e)}
  f:=append([]string{"-C",repo},args...)
  c:=exec.CommandContext(ctx,p,f...); var o,eb bytes.Buffer; c.Stdout=&o; c.Stderr=&eb
  re:=c.Run()
  if re==nil{return o.String(),eb.String(),0,nil}
  if ce:=ctx.Err();ce!=nil{return o.String(),eb.String(),-1,ce}
  var ee*exec.ExitError; if errors.As(re,&ee){return o.String(),eb.String(),ee.ExitCode(),nil}
  return o.String(),eb.String(),-1,re
}
func main(){
  dir,_:=os.MkdirTemp("","smoke"); defer os.RemoveAll(dir)
  exec.Command("git","-C",dir,"init").Run()
  o,eb,c,e:=run(context.Background(),dir,"rev-parse","--git-dir")
  fmt.Printf("git-dir: code=%d err=%v stdout=%q stderr=%q\n",c,e,o,eb)
  o2,eb2,c2,e2:=run(context.Background(),dir,"rev-parse","HEAD")
  fmt.Printf("HEAD(unborn): code=%d err=%v stdout=%q stderr_has_ambiguous=%v\n",
    c2,e2,o2,bytes.Contains([]byte(eb2),[]byte("ambiguous")))
}
EOF
go run /tmp/smoke_git.go
# Expected output (on git 2.54.0):
#   git-dir: code=0 err=<nil> stdout=".git\n" stderr=""
#   HEAD(unborn): code=128 err=<nil> stdout="HEAD\n" stderr_has_ambiguous=true
rm -f /tmp/smoke_git.go
# This proves run()'s real-binary path and the (code=128, err=nil) invariant end-to-end.
```

## Final Validation Checklist

### Technical Validation

- [ ] `gofmt -l internal/git/` reports nothing.
- [ ] `go vet ./...` exits 0.
- [ ] `go build ./...` exits 0.
- [ ] `go test -race ./internal/git/` exits 0 (all 5 test funcs pass).
- [ ] `make test` exits 0 (the S2 target runs the same `-race` suite).

### Feature Validation

- [ ] `Git` interface declared with exactly 11 methods and the signatures in §Blueprint.
- [ ] `FileChange` and `StagedDiffOptions` value types declared with the fields in §Blueprint.
- [ ] `gitRunner` struct declared with a `workDir string` field.
- [ ] `run()` is unexported, on `*gitRunner`, with the exact 4-tuple signature and the verified body.
- [ ] `New(workDir string) Git` returns a non-nil `*gitRunner`.
- [ ] All 11 interface methods exist on `*gitRunner` as panic-stubs naming their owning subtask.
- [ ] `run()` returns `err==nil` for non-zero git exits (verified by `TestRun_CapturesExitCodeAndSeparateBuffers`).
- [ ] `run()` returns `exitCode==128, stdout=="HEAD\n"` on an unborn repo's `rev-parse HEAD`.

### Security & Scope Discipline Validation

- [ ] NO `sh -c` / `zsh -c` / `bash -c` / `cmd /c` anywhere (`git grep` Level 3 → no matches).
- [ ] NO `cmd.Dir` field set, NO `os.Chdir` call (`git grep` Level 3 → no matches).
- [ ] `exec.LookPath("git")` called inside `run()`.
- [ ] Only `exec.CommandContext` used (no `exec.Command`, no `CombinedOutput`).
- [ ] No new dependencies; `go.mod` unchanged; no `go.sum` generated.
- [ ] Created ONLY `internal/git/git.go` and `internal/git/git_test.go` (`git status --porcelain`).
- [ ] Did NOT implement any of the 11 method bodies (those are S2–S6, T3.S1–S5).
- [ ] Did NOT add `SysProcAttr`/`Setpgid`/signal handling (that is P1.M2.T5 / P1.M4.T2).
- [ ] Did NOT touch `go.mod`, `Makefile`, `cmd/`, other `internal/*`, `pkg/`, `providers/`, or `plan/`.

---

## Anti-Patterns to Avoid

- ❌ Don't turn a non-zero git exit into a Go `err` — git uses exit codes as signals (1=staged, 128=unborn). Return them via `exitCode` with `err=nil`; only LookPath/context/start failures set `err` (gotcha G2).
- ❌ Don't detect an unborn repo by checking `stdout == ""` — git prints the literal string `"HEAD\n"` and exits 128. The interface encodes `isUnborn bool`; S2 sets it from `exitCode==128` (gotcha G3).
- ❌ Don't set Go's `cmd.Dir` field or call `os.Chdir` to scope git to the repo — pass `-C repo` as args. The contract's "sets cmd.Dir via -C <repo>" means "via the -C flag", confirmed by every architecture example (gotcha G1).
- ❌ Don't resolve `git` in `New()` — it has no error return. Resolve in `run()` via `exec.LookPath` so a missing binary surfaces as a clean runtime error (gotcha G5).
- ❌ Don't omit the 11 stub methods — `*gitRunner` must satisfy the full `Git` interface for `New() Git` to compile. Ship them as panic-stubs naming the owning subtask (gotcha G7).
- ❌ Don't put `run()` in the `Git` interface — it's an unexported helper; the interface holds only the 11 exported methods (gotcha G8).
- ❌ Don't write the test as `package git_test` (black-box) — `run()` and `gitRunner.workDir` are unexported; the test MUST be `package git` (gotcha G9).
- ❌ Don't test LookPath failure by uninstalling git or mutating global env — use `t.Setenv("PATH","")` (gotcha G10).
- ❌ Don't append `-C repo` onto the caller's variadic slice — allocate a fresh slice to avoid mutating the caller's backing array.
- ❌ Don't use a type assertion `runErr.(*exec.ExitError)` — use `errors.As(runErr, &exitErr)` (forward-compatible).
- ❌ Don't add `go-git`, any git library, or any other dependency — PRD §22.3 mandates the real binary via `exec` (gotcha G11).
- ❌ Don't implement method bodies, signal handling, or process-group logic here — those belong to S2–S6, T3.S1–S5, and P1.M2.T5/P1.M4.T2 respectively.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: The entire `run()` body is byte-equivalent to a throwaway Go program that was executed
against the exact installed git (2.54.0) and Go (1.26.4) toolchain — the four-tuple semantics
(`exit 128 → exitCode=128, err=nil`), the separate-buffer behavior, and the `LookPath`-failure
branch are all empirically confirmed (research §4). The interface signatures, the two auxiliary
types, the stub pattern, and the constructor are all copy-paste-specified verbatim. The 11 method
signatures are pinned and each maps 1:1 to a later subtask's contract, so the orchestrator gets a
stable type. The only residual uncertainty (not 10/10) is environmental: the unborn-repo exit code
is version-sensitive in principle (128 on 2.54.0, but git has occasionally shifted messaging); the
test pins `128` and the PRP tells the agent how to re-pin if a different git is present. The
"no-shell / no-cmd.Dir / no-Chdir" invariants are enforced by greppable Level-3 gates. The
parallel-execution note (S2 Makefile also landing) is a non-conflict: this subtask writes only
`internal/git/`, which S2 never touches.
