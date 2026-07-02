# Research — P1.M3.T1.S1: internal/git/git.go — Git wrapper + run(args) exec helper

## 1. Contract (verbatim from work item)
- `Git struct{dir string; git string (default "git")}` — both fields unexported; `git` holds the resolved binary path.
- `run(args ...string) (stdout string, err error)`: `exec.Command(git, args...)` with `Dir=dir`; capture stdout+stderr; on non-zero exit return typed `*ExitError{Args, Code, Stderr}`.
- Constructor `New(dir)` resolves git via `exec.LookPath`.
- OUTPUT: `git.Git` + `run()` consumed by every plumbing/diff/log/stage subtask (M3.T2/T3/T4) and the test util (M3.T1.S2).

## 2. Governing docs (verified read)
- **PRD §22.3** (PRD.md:1307-1312): "No git library dependency. Stagehand shells out to the real `git` binary (matching `commit-pi`). go-git is tempting but adds a large dependency... Shelling out is simpler, matches the reference implementation, and guarantees identical semantics." → NO go-git; stdlib `os/exec` only.
- **PRD §19** (PRD.md:1200-1202): "No shell interpolation. Commands are built as `[]string` and run via `exec.Command` directly, never via `sh -c`/`zsh -c`." → `exec.Command(g.git, args...)`, NEVER `sh -c`.
- **PRD §20.1 layer 2** (PRD.md:1212): "Unit — git wrapper, with a real git binary. Each `internal/git/*` test creates a temp directory, `git init`, stages known content..." → tests use a REAL git binary + temp repos (no mocks of the binary itself).
- **decisions.md §2**: `internal/git.Git` interface = RevParseHEAD/WriteTree/CommitTree/UpdateRefCAS/HasStagedChanges/AddAll/StagedDiff/CommitCount/RecentMessages/RecentSubjects, "Backed by `exec.Command(\"git\", ...)`. Tested with a temp repo + real git binary."
- **system_context §2**: git **2.54.0** present on host; go **1.26.4** (≥1.22 required).

## 3. The in-repo exec pattern to mirror — `internal/provider/executor.go`
The sibling executor is the canonical in-repo reference for an `os/exec` wrapper:
- `exec.Command(cmd, args...)`, `cmd.Dir`, `bytes.Buffer` for stdout+stderr, typed error struct (`*AgentError`), `exitCodeOf` via `errors.As(err, &ee)` + `ee.ExitCode()`, `excerpt()` tail-trim helper. `git.go`'s `run` is a SIMPLER sibling (no timeout, no process-group, no stdin feed) of `Executor.Run`.
- Doc-comment density: every exported symbol has a multi-line godoc citing the governing PRD section. Package doc lives on the FIRST file of the package.
- Tests (`executor_test.go`) are white-box (`package provider`), stdlib `testing` only, drive REAL host binaries (`/bin/echo`, `/bin/sh`), one behavior per `Test*` function, `errors.As` assertions.

## 4. os/exec API facts the implementation depends on
- `exec.LookPath("git") (string, error)` → returns resolved absolute path or `*exec.Error`/`*fs.PathError`-ish `LookPathError` if absent. Used in `New` for fail-fast resolution. https://pkg.go.dev/os/exec#LookPath
- `exec.Command(name, arg...)` → when `name` has NO path separator it calls LookPath internally; when `name` is an ABSOLUTE resolved path it skips re-resolution. Storing the LookPath result avoids per-call PATH search.
- `cmd.Dir` = working dir; empty string = inherit caller cwd (matches executor.go Dir semantics).
- `cmd.Stdout`/`cmd.Stderr` set to `*bytes.Buffer` → captured. **GOTCHA**: `(*exec.ExitError).Stderr` is ONLY populated by os/exec when `cmd.Stderr == nil`; when we attach our own buffer we MUST read `stderr.String()` (our buffer), NOT `ee.Stderr` (which will be empty). https://pkg.go.dev/os/exec#ExitError
- `cmd.Run()` → `nil` on exit 0; `*exec.ExitError` on non-zero exit; other error if it failed to START (binary gone, permission). Distinguish via `errors.As(err, &ee)` where `ee` is `*exec.ExitError` (stdlib, qualified).
- `(*exec.ExitError).ExitCode() int` → the child's exit code. https://pkg.go.dev/os/exec#ExitError

## 5. Naming-collision gotcha (CRITICAL)
The contract names the typed error `ExitError`. Go stdlib ALSO has `exec.ExitError`. Inside `package git`:
- `ExitError` (unqualified) = OUR type.
- `exec.ExitError` (qualified) = stdlib type.
They coexist fine; the implementer MUST use the qualified `*exec.ExitError` for the stdlib one in the `errors.As` target and the unqualified `ExitError`/`&ExitError{}` for ours. This is exactly the executor.go precedent (its `AgentError` is unqualified; it `errors.As` into `*exec.ExitError`).

## 6. Verified git behaviors for the MOCKING tests (host, git 2.54.0)
| command | context | result |
|---|---|---|
| `git version` | any dir | stdout "git version 2.54.0\n" → `strings.Contains(out,"git version")` true |
| `git rev-parse --git-dir` | `git init`'d temp dir | stdout ".git\n", exit 0 |
| `git rev-parse --git-dir` | plain temp dir (no repo) | exit **128**, stderr starts "fatal: not a git repository..." → our `*ExitError{Code:128, Stderr contains "not a git repository"}` |

## 7. Dependency boundary (DO NOT cross)
- S1 (`git.go`) has NO dependencies on other subtasks. It is the root of the `internal/git` package.
- S2 (`gittestutil_test.go`, the temp-repo harness) **depends on** S1 (it builds repos and drives them via `git.Git`/`run`). Therefore S1's own `git_test.go` MUST create its minimal `git init` temp dirs INLINE with `os/exec` (NOT import the not-yet-existing S2 harness).
- Downstream plumbing/diff/log/stage methods (T2/T3/T4) call `g.run(...)` from the same `package git`; `run` stays UNEXPORTED (lowercase) — only same-package + white-box tests can call it, which is the contract.
- `run` is a LOW-LEVEL helper. It does NOT implement RevParseHEAD/WriteTree/etc. (those are T2). It returns RAW captured stdout (callers trim). It does NOT own timeout/process-group/stdin (those are the provider Executor's concerns, §18.4 — git plumbing calls are synchronous and trusted).

## 8. Env / syscall posture for `run`
- Leave `cmd.Env = nil` → child inherits the stagehand process environment (git needs PATH, HOME, GIT_*; inheritance is correct). Do NOT call `os.Environ()` (no need; nil inherits). Matches "thin wrapper" intent.
- No `SysProcAttr` / `Setpgid` (that's the provider Executor's process-group kill for agent subprocesses; git plumbing is synchronous, no signal-handler seam here).
- No stdin → leave `cmd.Stdin` nil (os/exec connects to /dev/null automatically).

## 9. Validation gates (verified working on host)
- `go build ./internal/git/` (new package compiles)
- `go vet ./internal/git/` (clean)
- `test -z "$(gofmt -l internal/git/)"` (gofmt -l always exits 0 → wrap in test -z, proven idiom from P1.M1/M2 PRPs)
- `go test ./internal/git/` (PASS)
- `go test ./...` (whole-module integrity; Makefile `test` target green; provider+ui tests unaffected)
Baseline: `go vet ./...` clean, `go test ./...` green (provider, ui) before this task.

## 10. Scope discipline (anti-regression)
- ONLY create `internal/git/git.go` + `internal/git/git_test.go`. New package `git`.
- Do NOT touch main.go, Makefile, go.mod, go.sum, internal/ui, internal/provider.
- Do NOT run `go mod tidy` (stdlib-only file; tidy can strip unused-but-needed deps — P1.M1.T1.S1 precedent).
- git.go OWNS the `// Package git` doc comment (mirrors exitcode.go / manifest.go); git_test.go uses plain `package git`.
- No README/docs created (DOCS = Mode A inline godoc only).
