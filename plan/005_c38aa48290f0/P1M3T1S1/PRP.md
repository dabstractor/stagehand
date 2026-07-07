name: "P1.M3.T1.S1 — Git.HooksPath() helper + POSIX hook-script template with marker"
description: |
  Add `HooksPath() (string, error)` to the `internal/git` `Git` interface — wrapping
  `git rev-parse --git-path hooks` with cwd-relative → absolute resolution (honoring `core.hooksPath` and
  linked worktrees) — and define the strict-POSIX `prepare-commit-msg` hook-script template
  (`hookScript(strict bool) string` + `Marker` constant) in a NEW `internal/hook` package. Both are pure
  primitives consumed by P1.M3.T1.S2 (`hook install|uninstall|status`). No user-facing surface ships here
  (Mode A: no docs) — the CLI commands and their docs ride with S2.

---

## Goal

**Feature Goal**: Provide the two lowest-level building blocks for stagecoach's git hook mode (PRD §9.20):
(1) a reliable way to locate the repo's hooks directory as an ABSOLUTE path regardless of `core.hooksPath`,
subdirectory invocation, or linked worktrees; and (2) the exact bytes of the `prepare-commit-msg` script
stagecoach installs, carrying its identity marker and honoring the `--strict` opt-in.

**Deliverable**:
1. `HooksPath(ctx context.Context) (string, error)` added to the `Git` interface + implemented on
   `*gitRunner` in `internal/git/git.go`, with temp-repo tests in `internal/git/hookspath_test.go`.
2. A NEW package `internal/hook` with `internal/hook/script.go` containing the exported `Marker` constant,
   the exported `ScriptMode` file-mode constant, and the unexported `hookScript(strict bool) string`, plus
   `internal/hook/script_test.go`.

**Success Definition**:
- `HooksPath` returns an absolute, cleaned path to the hooks directory for all four layouts (default
  `.git/hooks`; `core.hooksPath` set; invoked from a subdirectory; from a linked worktree), and a real error
  on a non-repo (exit 128).
- `hookScript(false)` == the exact 3-line strict-POSIX script (`#!/bin/sh` + Marker + `exec stagecoach hook
  exec "$@"`); `hookScript(true)` appends `--strict` to the exec line; both parse under `sh -n` with no
  bashisms.
- `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run` all green.

## User Persona

**Target User**: (Indirect) the "plan-holder" (PRD §7.1) who runs `stagecoach hook install` so plain
`git commit` in their IDE/lazygit auto-fills the message. This subtask ships no command they touch — it is
the plumbing S2 wires to `hook install`.

**Use Case**: `stagecoach hook install` (S2) calls `Git.HooksPath()` to find where to write, and `hookScript`
to get the bytes to write. This subtask makes both calls possible and correct.

**User Journey**: none user-visible in S1. S2's journey: `hook install` → `HooksPath()` resolves the dir →
write `hookScript(strict)` at `<dir>/prepare-commit-msg` mode `ScriptMode` → mark with `Marker`.

**Pain Points Addressed**: incumbents install `prepare-commit-msg` hooks but naively assume `.git/hooks`,
breaking under `core.hooksPath` and worktrees. Resolving via `git rev-parse --git-path hooks` (verified,
architecture §3) is the correct, portable location; the strict-POSIX script runs under git-for-windows `sh`.

## Why

- **PRD §9.20 FR-H1**: *"`stagecoach hook install` resolves the hook directory via `git rev-parse --git-path
  hooks` (this honors `core.hooksPath` and worktrees) and writes an executable `prepare-commit-msg` POSIX-sh
  script containing a marker line (`# stagecoach prepare-commit-msg hook v1`) and, essentially, `exec
  stagecoach hook exec "$@"`."* — S1 provides the resolver + the script; S2 does the writing.
- **PRD §9.20 FR-H5**: *"`hook install --strict` bakes a `--strict` flag into the script."* — the `strict`
  parameter of `hookScript`.
- **Architecture §3 (VERIFIED, git 2.54.0)**: `git rev-parse --git-path hooks` honors `core.hooksPath`,
  returns the common-dir hooks path from a linked worktree, and returns a **cwd-RELATIVE path from a
  subdirectory** (must be resolved to absolute). The hook is NOT suppressed by `--no-verify`.
- **Appendix E #15**: confirm the script runs under git-for-windows `sh` → keep it strict POSIX (verify
  in-task). No bashisms.
- **Scope fence**: OUTPUT is exactly `HooksPath()` + `hookScript(strict bool) string`, consumed by S2. This
  subtask does NOT create the `hook` cobra commands, the runtime (`hook exec`), or any docs — those are S2 /
  P1.M3.T2.S1.

## What

Add one `Git` interface method (+ its `*gitRunner` impl) and one new package file with the hook-script
constants + builder. Everything is a pure/plumbing primitive with unit tests; no wiring into the CLI.

### Success Criteria

- [ ] `Git` interface gains `HooksPath(ctx context.Context) (string, error)`; `*gitRunner` implements it via
      `git rev-parse --git-path hooks` with absolute-path resolution against `g.workDir`.
- [ ] `HooksPath` returns an absolute path for: default layout, `core.hooksPath` (relative AND absolute
      values), subdirectory invocation, and linked worktree; returns a non-nil error on a non-repo (exit 128).
- [ ] `internal/hook/script.go`: `const Marker = "# stagecoach prepare-commit-msg hook v1"`,
      `const ScriptMode os.FileMode = 0o755`, `func hookScript(strict bool) string`.
- [ ] `hookScript(false)` / `hookScript(true)` produce the exact scripts (below); tests assert bytes, marker
      placement, shebang, and POSIX-ness (`sh -n`).
- [ ] Full build/test/vet/lint green; existing `internal/git` tests unchanged.

## All Needed Context

### Context Completeness Check

_This PRP names the exact interface insertion point (after `DiffTreeNames`, before the interface's closing
brace at git.go:259), the impl pattern copied from the existing runner methods, the one design call
(cwd-relative → absolute resolution against `g.workDir`, with the reasoning), the exact script bytes, the
package to create (`internal/hook`), and the test harness to mirror (`internal/git/revparse_test.go` +
`git_test.go`'s `initRepo`). An implementer with no prior codebase knowledge can complete it from this
document + codebase access._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/P1M3T1S1/research/hook-path-and-script.md
  why: The condensed research — §3 verified facts, the resolution decision (join relative output with
       g.workDir then filepath.Abs), the exit-code convention (128 is a REAL error here, NOT unborn), the
       exact script bytes, and why hookScript lives in internal/hook.
  section: all (short)
  critical: |
    run() execs `git -C g.workDir …`, so relative --git-path output is relative to g.workDir. Resolve with
    filepath.Abs(filepath.Join(g.workDir, raw)). One branch covers all four layouts. Exit 128 = real error.

- docfile: plan/005_c38aa48290f0/architecture/external_deps.md
  why: §3 "prepare-commit-msg hook semantics (gates FR-H1/FR-H4) — VERIFIED (git 2.54.0)". The authoritative
       verification that --git-path hooks honors core.hooksPath + worktrees and returns a relative path from
       a subdirectory; and Appendix E #15 (strict POSIX for git-for-windows sh).
  section: "## 3." (lines 37-48)
  critical: "From a subdirectory it returns a RELATIVE path → resolve to absolute. Linked worktree → common
             dir hooks (correct). Keep the script strict POSIX (no bashisms)."

- docfile: plan/005_c38aa48290f0/prd_snapshot.md
  why: §9.20 FR-H1 (install resolves via `git rev-parse --git-path hooks`; marker line;
       `exec stagecoach hook exec "$@"`), FR-H5 (`--strict` baked into the script). The contract this
       primitive serves.
  section: "§9.20 FR-H1 + FR-H5"
  critical: 'Marker EXACTLY `# stagecoach prepare-commit-msg hook v1`; body `exec stagecoach hook exec "$@"`;
             strict appends `--strict`.'

- file: internal/git/git.go
  why: The Git interface (L57-259) + the gitRunner method pattern. Add HooksPath to the interface right
       AFTER DiffTreeNames (its doc ends L258, interface closes L259). Implement it on *gitRunner near the
       end of the file, mirroring the run()→check err→check code→trim/parse shape of StatusPorcelain /
       RevParseHEAD. run() returns (stdout, stderr, exitCode, err); a non-zero git exit has err==nil and the
       code in exitCode (INVARIANT at L279-282).
  pattern: |
    // interface (after DiffTreeNames' doc block, before the closing } at L259):
    // HooksPath returns the ABSOLUTE path to this repo's hooks directory via
    // `git rev-parse --git-path hooks` (honors core.hooksPath and linked worktrees — architecture §3).
    // git may return a cwd-relative path (notably from a subdirectory); it is resolved to absolute against
    // the runner's workDir. Exit 128 (non-repo/corrupt) is a REAL error (this call works on unborn repos,
    // so there is NO 128-as-non-error convention here — like StatusPorcelain). Read-only (PRD §18.1).
    HooksPath(ctx context.Context) (string, error)
  gotcha: |
    ADD `"path/filepath"` to the import block (L3-13). Resolve ABSOLUTE: if filepath.IsAbs(raw) return
    filepath.Clean(raw); else filepath.Abs(filepath.Join(g.workDir, raw)). Do NOT special-case 128 as
    non-error. Branch on code != 0.

- file: internal/git/git.go  (StatusPorcelain impl, ~L154-167 doc + its method body)
  why: The closest existing convention — a read-only method where 128 is a REAL error (NOT unborn). Mirror
       its err/code handling and doc tone for HooksPath.
  pattern: "err != nil → return err; code != 0 → wrapped fmt.Errorf with exit + trimmed stderr; else parse."

- file: internal/git/revparse_test.go
  why: THE test harness to mirror. minGitEnv(), makeEmptyCommit(t, dir, msg), the table of scenarios
       (born/unborn/binary-missing/context-cancelled), t.TempDir() + initRepo(t, repo). Copy this structure
       into internal/git/hookspath_test.go.
  pattern: "repo := t.TempDir(); initRepo(t, repo); g := New(repo); got, err := g.HooksPath(ctx); assert."

- file: internal/git/git_test.go
  why: initRepo(t, dir) helper (L12-37) — `git init` + repo-local user.name/user.email. Reuse it (same
       package). For a linked-worktree test, add a commit (makeEmptyCommit) then `git -C repo worktree add
       <wt>` via exec.Command (worktree add REQUIRES ≥1 commit). For core.hooksPath, `git -C repo config
       core.hooksPath <val>`.
  gotcha: "New(path) binds workDir=path; there is no cmd.Dir/Chdir — subdirectory invocation is simulated by
           New(filepath.Join(repo, 'sub')) after os.MkdirAll(sub)."

- file: internal/git/stagediff_test.go
  why: Reference for a test that creates subdirectories / nested paths inside a temp repo (helper patterns
       for writing files and staging). Use it if you need file-creation helpers for the subdir test.

- url: https://git-scm.com/docs/git-rev-parse#Documentation/git-rev-parse.txt---git-pathpath
  why: `--git-path <path>` semantics — "resolve <path> relative to the .git directory … core.hooksPath is
       honored for the hooks path." Confirms the command choice.
  critical: "The hooks path specifically honors core.hooksPath; other --git-path targets do not special-case."
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go              # EDIT — add HooksPath to Git interface + *gitRunner impl; add "path/filepath" import
  git_test.go         # UNCHANGED (reuse initRepo helper — same package)
  revparse_test.go    # UNCHANGED (mirror its structure)
  hookspath_test.go   # NEW — temp-repo tests for HooksPath (4 layouts + non-repo)
internal/hook/        # NEW PACKAGE (S2 builds install|uninstall|status on top)
  script.go           # NEW — Marker, ScriptMode, hookScript(strict bool) string
  script_test.go      # NEW — script-bytes / marker / POSIX (sh -n) tests
```

### Desired Codebase tree

```bash
# One interface method + impl in the existing internal/git package, and one brand-new internal/hook package
# holding only the script template (S2 will add install.go/status.go/etc. to the same package). No CLI wiring,
# no docs (Mode A). No new third-party dependencies.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (resolution): run() execs `git -C g.workDir …`, so git's cwd IS g.workDir and any RELATIVE
// --git-path output is relative to g.workDir. Resolve: filepath.IsAbs(raw) ? filepath.Clean(raw)
// : filepath.Abs(filepath.Join(g.workDir, raw)). filepath.Abs also Cleans, so `../.git/hooks` from a
// subdirectory collapses to the correct absolute path. This single branch handles ALL four layouts.

// CRITICAL (exit codes): `git rev-parse --git-path hooks` succeeds on an UNBORN repo (needs no commits).
// So 128 is a REAL error here (non-repo/corrupt) — do NOT copy RevParseHEAD's `code == 128 → nil` unborn
// convention. Mirror StatusPorcelain: branch on `code != 0`, never on `code == 128`.

// CRITICAL (run() contract): a non-zero git exit returns (stdout, stderr, exitCode, nil) — err is nil, the
// code is in exitCode (git.go L279-282). Check `if err != nil` first (binary-missing / ctx-cancel; code=-1),
// THEN `if code != 0`. Never treat exit 1/128 as a Go error from run().

// CRITICAL (POSIX, Appendix E #15): the script is `#!/bin/sh` + a `#` comment + a single `exec … "$@"`.
// `"$@"` and `exec` are POSIX; no `[[`, arrays, `function`, `local`, or process substitution. It must run
// under git-for-windows sh. Verify with `sh -n` in a test (skip the test if `sh` is not on PATH).

// GOTCHA (script bytes): the marker is the SECOND line (after the shebang), EXACTLY
// `# stagecoach prepare-commit-msg hook v1`. Trailing newline on the file. strict==true changes ONLY the
// exec line (adds ` --strict` before `"$@"`). Build with explicit "\n" joins — do not rely on fmt quirks.

// GOTCHA (placement): hookScript is UNEXPORTED per the contract → it must live in the package S2 consumes it
// from (internal/hook). Marker + ScriptMode are EXPORTED so S2's status detection / file writer and any
// cross-package test can reference them. HooksPath belongs on the git runner (it wraps a git call).

// GOTCHA (imports): internal/hook/script.go needs `os` only for os.FileMode on ScriptMode. Keep it tiny.
```

## Implementation Blueprint

### Data models and structure

No data models. Two additions: one interface method + impl (git package) and three package-level symbols
(hook package).

```go
// internal/hook/script.go  (NEW)
package hook

import "os"

// Marker is the identity line stagecoach writes as the SECOND line of its prepare-commit-msg hook (after the
// shebang). Its presence is how `hook status`/`hook uninstall` (P1.M3.T1.S2) recognize a stagecoach-owned
// hook (marker present → ours, rewrite/remove; absent → foreign, refuse — PRD §9.20 FR-H2/FR-H3).
const Marker = "# stagecoach prepare-commit-msg hook v1"

// ScriptMode is the file mode stagecoach writes the hook with (executable — PRD §9.20 FR-H1).
const ScriptMode os.FileMode = 0o755

// hookScript returns the exact bytes of the prepare-commit-msg hook stagecoach installs (PRD §9.20 FR-H1).
// It is strict POSIX sh (no bashisms) so it runs under git-for-windows' sh (Appendix E #15). When strict is
// true the runtime call gets `--strict` (PRD §9.20 FR-H5: failures then abort the commit). The trailing
// newline keeps the file POSIX-clean.
func hookScript(strict bool) string {
	run := `exec stagecoach hook exec "$@"`
	if strict {
		run = `exec stagecoach hook exec --strict "$@"`
	}
	return "#!/bin/sh\n" + Marker + "\n" + run + "\n"
}
```

Exact expected output — `hookScript(false)`:

```sh
#!/bin/sh
# stagecoach prepare-commit-msg hook v1
exec stagecoach hook exec "$@"
```

`hookScript(true)` differs on the last line only: `exec stagecoach hook exec --strict "$@"`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/git/git.go — add "path/filepath" import
  - ADD `"path/filepath"` to the import block (L3-13), keeping gofmt import ordering.

Task 2: EDIT internal/git/git.go — add HooksPath to the Git interface
  - INSERT the HooksPath method + its doc comment AFTER DiffTreeNames' doc block, BEFORE the interface's
    closing `}` (L259). Doc: absolute path, honors core.hooksPath + worktrees, 128 is a real error, read-only.

Task 3: EDIT internal/git/git.go — implement (*gitRunner).HooksPath near the end of the file
  - IMPLEMENT with run(ctx, g.workDir, "rev-parse", "--git-path", "hooks"); err-then-code checks (mirror
    StatusPorcelain); trim stdout; resolve absolute via filepath.IsAbs / filepath.Abs(filepath.Join(...)).

Task 4: CREATE internal/hook/script.go
  - IMPLEMENT Marker, ScriptMode, hookScript(strict bool) string (snippet above), with the FR-H1/FR-H5 docs.

Task 5: CREATE internal/git/hookspath_test.go
  - Mirror revparse_test.go. Tests (all t.TempDir()+initRepo, ctx=context.Background()):
    * TestHooksPath_DefaultLayout: got == filepath.Join(repo, ".git", "hooks"); filepath.IsAbs(got).
    * TestHooksPath_CoreHooksPath_Relative: `git config core.hooksPath myhooks` → got == Join(repo,"myhooks").
    * TestHooksPath_CoreHooksPath_Absolute: set an absolute value (t.TempDir()) → got == that path.
    * TestHooksPath_FromSubdirectory: os.MkdirAll(repo/sub); New(repo/sub).HooksPath() ==
      Join(repo,".git","hooks") (relative `../.git/hooks` resolves correctly).
    * TestHooksPath_LinkedWorktree: makeEmptyCommit; `git -C repo worktree add <wt>`; New(wt).HooksPath()
      resolves to the MAIN repo's .git/hooks (absolute; common dir).
    * TestHooksPath_NonRepo: New(t.TempDir()) (no init) → err != nil (exit 128 → wrapped error).
    * (optional, mirror revparse_test) TestHooksPath_GitBinaryMissing: t.Setenv("PATH","") → "git binary
      not found" error.
  - NOTE: worktree/config helpers via exec.Command("git","-C",dir,...) with minGitEnv() (reuse the helpers).

Task 6: CREATE internal/hook/script_test.go
  - TestHookScript_NonStrict: equals the exact 3-line script; first line "#!/bin/sh"; second line == Marker;
    contains `exec stagecoach hook exec "$@"`; does NOT contain "--strict".
  - TestHookScript_Strict: last content line == `exec stagecoach hook exec --strict "$@"`; still starts with
    shebang + Marker.
  - TestHookScript_MarkerPresent: strings.Contains(hookScript(false), Marker) && strings.Contains(hookScript(true), Marker).
  - TestHookScript_POSIX: write hookScript(false)/(true) to t.TempDir() files, run `sh -n <file>`; expect
    exit 0. Skip via t.Skip if exec.LookPath("sh") fails (Windows CI). (Optional: `checkbashisms`/`dash -n`
    if present.)
  - TestScriptMode: ScriptMode == 0o755.
```

### Implementation Patterns & Key Details

```go
// internal/git/git.go — the impl (mirrors StatusPorcelain's err/code shape):
func (g *gitRunner) HooksPath(ctx context.Context) (string, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "rev-parse", "--git-path", "hooks")
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		// 128 = non-repo/corrupt — a REAL error (this call works on unborn repos; no 128-as-non-error here).
		return "", fmt.Errorf("git rev-parse --git-path hooks: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	raw := strings.TrimSpace(stdout)
	if raw == "" { // defensive: never observed on a valid repo
		return "", fmt.Errorf("git rev-parse --git-path hooks: empty output")
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw), nil // core.hooksPath=abs, or a worktree returning an absolute common path
	}
	// Relative output (default `.git/hooks`, or `../.git/hooks` from a subdirectory) is relative to the -C dir.
	abs, aerr := filepath.Abs(filepath.Join(g.workDir, raw))
	if aerr != nil {
		return "", fmt.Errorf("resolve hooks path %q against %q: %w", raw, g.workDir, aerr)
	}
	return abs, nil
}

// hookspath_test.go — worktree + config helpers (reuse minGitEnv/initRepo/makeEmptyCommit):
func gitDo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = minGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
// core.hooksPath:  gitDo(t, repo, "config", "core.hooksPath", "myhooks")
// linked worktree: makeEmptyCommit(t, repo, "init"); wt := t.TempDir()+"/wt"; gitDo(t, repo, "worktree", "add", wt)
```

### Integration Points

```yaml
GIT PACKAGE (owns HooksPath):
  - add: Git interface method + *gitRunner impl in internal/git/git.go; import "path/filepath".
  - consumed by: internal/hook install (P1.M3.T1.S2) — NOT wired here.

HOOK PACKAGE (NEW — owns the script template):
  - new: internal/hook/script.go (Marker, ScriptMode, hookScript). S2 adds install.go/status.go here.
  - Marker + ScriptMode exported for S2's detection + writer; hookScript unexported (same-package consumer).

OUT OF SCOPE (do NOT touch):
  - CLI: no `hook` cobra command, no root.go wiring (S2).
  - Runtime: no `hook exec` (P1.M3.T2.S1).
  - Docs: none (Mode A — FR-H docs ride with S2/S2's command surface).
  - Config: hook mode reuses the `message` role config (FR-H6) — not this subtask.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/git/git.go internal/git/hookspath_test.go internal/hook/script.go internal/hook/script_test.go
go build ./...        # HooksPath impl + new internal/hook package must compile
go vet ./...
golangci-lint run
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/git/... -run HooksPath -v   # 4 layouts + non-repo (+ optional binary-missing)
go test ./internal/hook/... -v                 # script bytes / marker / POSIX (sh -n) / ScriptMode
go test ./internal/git/... -v                  # confirm existing git tests still pass (interface unchanged behavior)
# Expected: all pass. HooksPath returns absolute paths; scripts match exactly.
```

### Level 3: Integration Testing (System Validation)

```bash
# Manual sanity: HooksPath in a real repo (build a tiny throwaway main, or exercise via the test above).
# Prove the script is POSIX under a real sh and (if available) dash:
cat > /tmp/sh_probe <<'EOF'
#!/bin/sh
# stagecoach prepare-commit-msg hook v1
exec stagecoach hook exec "$@"
EOF
sh -n /tmp/sh_probe && echo "POSIX OK"
command -v dash >/dev/null && dash -n /tmp/sh_probe && echo "dash OK" || true
command -v checkbashisms >/dev/null && checkbashisms /tmp/sh_probe || true
# core.hooksPath honored (manual):
tmp=$(mktemp -d); git -C "$tmp" init -q; git -C "$tmp" config core.hooksPath custom
git -C "$tmp" rev-parse --git-path hooks   # → "custom" (relative) — proves resolution is needed
```

### Level 4: Regression & Cross-cutting

```bash
go test -race ./...     # full suite (make test)
golangci-lint run ./...
# Coverage: the new internal/hook package and HooksPath are pure/deterministic — aim for full line coverage
# (make coverage-gate if the project enforces ≥85% on core packages).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` compiles (HooksPath + internal/hook).
- [ ] `go test ./...` green; existing internal/git tests unchanged.
- [ ] `go vet ./...` + `golangci-lint run` clean; `gofmt` no diff.

### Feature Validation
- [ ] `HooksPath` returns an ABSOLUTE path for default / core.hooksPath (rel+abs) / subdirectory / linked
      worktree; non-repo → error.
- [ ] `hookScript(false)` == the exact 3-line strict-POSIX script; `hookScript(true)` appends `--strict`.
- [ ] Marker is exactly `# stagecoach prepare-commit-msg hook v1` and is line 2 of the script.
- [ ] `sh -n` accepts both scripts (POSIX; no bashisms).

### Code Quality Validation
- [ ] HooksPath mirrors StatusPorcelain's err/code convention (128 is a real error; branch on code != 0).
- [ ] Relative→absolute resolution uses filepath.Abs(filepath.Join(g.workDir, raw)) — one branch, all layouts.
- [ ] hookScript unexported in internal/hook; Marker + ScriptMode exported for S2.
- [ ] No CLI/docs/runtime edits (Mode A; S2 owns those).

### Documentation & Deployment
- [ ] None (Mode A). Godoc comments on HooksPath / Marker / ScriptMode / hookScript are the only "docs".

---

## Anti-Patterns to Avoid

- ❌ Don't copy RevParseHEAD's `code == 128 → nil` unborn convention — `--git-path hooks` works on unborn
  repos; 128 here is a real error (mirror StatusPorcelain).
- ❌ Don't return the raw `git rev-parse --git-path hooks` output — it can be relative (subdirectory);
  resolve to absolute against g.workDir.
- ❌ Don't `filepath.Join` an already-absolute path with g.workDir (Join won't treat the 2nd arg as absolute
  — check filepath.IsAbs first).
- ❌ Don't put `hookScript` in internal/git — S2's consumer is internal/hook; unexported symbols don't cross
  packages.
- ❌ Don't introduce bashisms (`[[`, arrays, `function`, `local`) — the script must run under git-for-windows sh.
- ❌ Don't wire a `hook` cobra command, `hook exec`, or docs — those are S2 / P1.M3.T2.S1.
- ❌ Don't assume `.git/hooks` — that breaks under core.hooksPath and worktrees (the whole reason for FR-H1's
  `git rev-parse --git-path hooks`).

---

## Confidence Score

**9/10** for one-pass implementation success. The surface is tiny and fully pinned: one interface method
whose impl copies the established StatusPorcelain err/code shape, one resolution branch justified by the
`-C g.workDir` runner design, and one pure string builder with exact expected bytes. The test harness
(`initRepo` / `makeEmptyCommit` / `minGitEnv` in the same package, mirrored from `revparse_test.go`) is
already in place. The only residual risk is environment-specific test detail — the exact relative form git
returns from a subdirectory/worktree — but the resolution logic (`filepath.Abs(filepath.Join(...))`) is
form-agnostic, so the tests assert the resolved absolute path rather than the raw git output, neutralizing
that risk. The −1 is the small chance CI lacks `sh`/`git worktree` support, handled by `t.Skip` guards.
