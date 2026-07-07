---
name: "P1.M2.T1.S1 — runWithEnv exec seam in internal/git"
description: |
  Foundational env-passing exec primitive for v2.4 hook execution (FR-V1–V8 → G22). `internal/git/git.go`
  has two exec seams — `run()` (389) and `runWithInput()` (430) — and NEITHER sets `cmd.Env` (the child
  inherits the parent env); `GIT_INDEX_FILE` appears NOWHERE in internal/. The hook feature (FR-V3:
  `pre-commit` scoped to the snapshot tree `T_start`) needs a THROWAWAY index — `GIT_INDEX_FILE=<abs tmp>`
  threaded through read-tree/write-tree/update-index — which requires a NEW env-passing seam. This task
  adds `runWithEnv(ctx, repo, extraEnv, args...)`: identical to `runWithInput` EXCEPT
  `cmd.Env = append(os.Environ(), extraEnv...)` instead of `cmd.Stdin`. Additive (a SUPERSET) — preserves
  the documented "child inherits parent env" guarantee (git.go:426-427). No behavior change; the seam is
  consumed by P1.M2.T1.S2 (the scoped ReadTreeInto/WriteTreeFrom variants).

  ⚠️ **THE central design call — mirror runWithInput EXACTLY; the ONE difference is `cmd.Env`.** runWithEnv
  is `run()` + `cmd.Env = append(os.Environ(), extraEnv...)`, just as runWithInput is `run()` + `cmd.Stdin`.
  Same LookPath → `-C repo` → `[]string` args (NO shell, PRD §19) → separate stdout/stderr buffers →
  exit-code semantics (non-zero git exit ⇒ (stdout, stderr, code, nil); err != nil only for LookPath miss /
  ctx cancel / start-I/O failure). Place it immediately after runWithInput (line 460), co-located with
  run/runWithInput (the established "co-located" discipline).

  ⚠️ **THE second design call — `"os"` is NOT imported in git.go; ADD it.** git.go imports `"os/exec"` but
  NOT `"os"`. `os.Environ()` requires `import "os"` (add it to the import block, alphabetical: between
  "io" and "os/exec"). WITHOUT this, `go build` FAILS ("undefined: os"). This is the easy-to-miss
  build-breaker — verify the import lands.

  ⚠️ **THE third design call — `unused` lint is ENABLED; S1 MUST be self-contained (add a test).**
  `.golangci.yml` enables `unused`; an unexported `runWithEnv` with no caller trips U1000. The contract's
  "unused until S2" would leave S1 lint-broken if gated before S2 lands. Therefore S1 includes a focused
  unit test that BOTH validates the seam (cmd.Env is set) AND keeps runWithEnv "used" (lint green). Use
  git's env-based config injection (`GIT_CONFIG_COUNT`/`GIT_CONFIG_KEY_0`/`GIT_CONFIG_VALUE_0`) +
  `git config --get` — deterministic (never in parent env ⇒ no duplicate-key risk), needs NO commit
  (`initRepo` doesn't commit; `git config --get` works in an unborn repo).

  ⚠️ **THE fourth design call — keep runWithEnv UNEXPORTED (NOT on the Git interface).** Mirrors run/
  runWithInput (both unexported `gitRunner` methods; the Git interface at git.go:87 exposes only
  RevParseHEAD/WriteTree/etc.). The scoped variants (S2) are the public surface. Add runWithEnv to the Git
  interface ONLY if internal/hooks (M3) needs to call it directly; the contract PREFERS the
  scoped-variants-as-public-surface shape, so leave it unexported.

  SCOPE: edit `internal/git/git.go` (add `"os"` import + the `runWithEnv` method) + `internal/git/git_test.go`
  (the focused test). No docs, no Git-interface change, no scoped variants (S2), no hook runner (M3), no
  config. INPUT = run()/runWithInput() structure. OUTPUT = an env-passing exec seam; `cmd.Env` is a
  superset of the parent env (additive). DOCS: none — internal exec seam.
---

## Goal

**Feature Goal**: Add an env-passing exec primitive `runWithEnv(ctx, repo, extraEnv, args...)` to
`internal/git/git.go` — identical to `runWithInput` except `cmd.Env = append(os.Environ(), extraEnv...)` —
so a git subprocess can run with scoped env vars (`GIT_INDEX_FILE=<abs tmp>`, `GIT_EDITOR=:`) while still
inheriting the full parent environment. This is the foundation for the scoped-index git primitives
(P1.M2.T1.S2) that the v2.4 commit-hooks runner (FR-V3) depends on.

**Deliverable** (edits to existing files):
1. **`internal/git/git.go`** — (a) add `"os"` to the import block (currently only `"os/exec"`); (b) add
   `func (g *gitRunner) runWithEnv(ctx context.Context, repo string, extraEnv []string, args ...string)
   (stdout, stderr string, exitCode int, err error)` immediately after `runWithInput` (line 460), mirroring
   its structure exactly with `cmd.Env = append(os.Environ(), extraEnv...)` as the one difference.
2. **`internal/git/git_test.go`** — add `TestGitRunner_RunWithEnv_PassesEnv` (GIT_CONFIG_COUNT env
   injection + `git config --get`) proving extraEnv reaches the child; placed next to `TestRun_HappyPath`.

**Success Definition**: `gofmt -l internal/git/` clean; `go vet ./internal/git/` clean; `go build ./...`
succeeds (the `"os"` import lands); `go test -race ./internal/git/` green (new test passes + existing
tests unchanged); `make lint` green (runWithEnv is "used" by the test ⇒ no U1000). go.mod/go.sum
unchanged; only `internal/git/git.go` + `internal/git/git_test.go` touched; runWithEnv is unexported and
NOT on the Git interface; no scoped variants / hook runner / docs added.

## User Persona

**Target User**: The NEXT subtask (P1.M2.T1.S2 — scoped ReadTreeInto/WriteTreeFrom variants that call
runWithEnv with `GIT_INDEX_FILE=<tmp>`), and transitively the commit-hooks runner (M3, FR-V3) which scopes
`pre-commit` to the snapshot tree via a throwaway index.

**Use Case**: (internal seam, no user-visible behavior yet) a git primitive runs with an extra env var —
e.g. `runWithEnv(ctx, repo, []string{"GIT_INDEX_FILE=" + tmpIndex}, "read-tree", tree)` populates the
throwaway index, not the repo's default `.git/index`.

**User Journey**: (future) plumbing path → `pre-commit` hook → scoped to `T_start` via a throwaway index
→ runWithEnv threads `GIT_INDEX_FILE` through read-tree/write-tree → the freeze invariant (FR-V3) holds.
This task is the exec primitive that makes the threading possible.

**Pain Points Addressed**: removes the structural blocker (no env-passing exec seam) for scoped-index git
operations, which the hook feature requires.

## Why

- **Structural enabler for S2 + M3.** The scoped-index mechanism (codebase_reality.md §1) requires a new
  env-passing seam; today every index primitive operates on the single default `.git/index`. runWithEnv is
  the minimal primitive S2's scoped variants build on.
- **Mirrors the proven run/runWithInput pattern.** runWithInput already established "run() + one
  difference, co-located, unexported, shares exit-code semantics." runWithEnv follows the same discipline
  with `cmd.Env` as the one difference — low-risk, idiomatic for this file.
- **Additive = safe.** `append(os.Environ(), extraEnv...)` is a SUPERSET; the documented "child inherits
  parent env" guarantee (git.go:426-427) is preserved. PATH/HOME/identity env are not clobbered; only
  scoped vars are added.
- **Self-contained + lint-green.** The included test validates the seam and satisfies the `unused` lint
  gate, so S1 is independently green (not dependent on S2 landing first).

## What

One unexported env-passing exec method (mirroring runWithInput) + one import + one focused test. No Git-
interface change, no scoped variants, no hook runner, no docs. The seam is unused in production until S2.

### Success Criteria

- [ ] `runWithEnv(ctx, repo, extraEnv []string, args ...string) (stdout, stderr string, exitCode int, err error)`
      exists on `*gitRunner` in `internal/git/git.go`, placed immediately after `runWithInput` (line 460).
- [ ] Its body is byte-identical to `runWithInput` EXCEPT `cmd.Env = append(os.Environ(), extraEnv...)`
      (no `cmd.Stdin`); it shares the LookPath → `-C repo` → separate buffers → exit-code-semantics shape.
- [ ] `"os"` is added to git.go's import block (between "io" and "os/exec"); `go build ./...` succeeds.
- [ ] runWithEnv is UNEXPORTED and NOT added to the `Git` interface (mirrors run/runWithInput).
- [ ] `TestGitRunner_RunWithEnv_PassesEnv` exists in `internal/git/git_test.go` and passes: extraEnv
      (`GIT_CONFIG_COUNT`/`KEY`/`VALUE`) reaches the child (`git config --get` returns the injected value).
- [ ] `gofmt -l internal/git/`, `go vet ./internal/git/`, `go build ./...`, `go test -race ./internal/git/`,
      `make lint` all clean/green; go.mod/go.sum unchanged; only git.go + git_test.go touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the runWithEnv body (quoted
verbatim below), the `"os"`-import gotcha, the placement (after runWithInput, line 460), and the test
(GIT_CONFIG_COUNT). No PRD/hook/scoped-index knowledge required — this is one exec method mirroring an
existing sibling.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/010_49117f1f30ab/P1M2T1S1/research/runwithenv_exec_seam.md
  why: the runWithEnv body (verbatim), the two gotchas ("os" import + unused-lint), the additive-semantics
       proof, the GIT_CONFIG_COUNT test choice, the placement, and the S1/S2/M3 boundary.
  critical: (1) ADD "os" to git.go imports (only "os/exec" is there today) or build fails; (2) the `unused`
       lint is enabled — the test is REQUIRED to keep S1 lint-green, not optional.

- docfile: plan/010_49117f1f30ab/docs/architecture/codebase_reality.md
  section: "1. The exec boundary (the single hardest integration point)"
  why: confirms run()/runWithInput() NEVER set cmd.Env; GIT_INDEX_FILE appears nowhere in internal/; the
       scoped-index mechanism REQUIRES this new env-passing seam (`cmd.Env = append(os.Environ(), env...)`).
  critical: the seam is additive (superset of parent env) — preserves the documented inheritance guarantee.

- file: internal/git/git.go
  section: run() (389-414), runWithInput() (420-460), import block (3-16), Git interface (87)
  why: the structure to mirror. runWithInput is the closest analog ("run() + one difference"); its doc
       (424-428) states the co-located/shared-structure discipline. Place runWithEnv right after it (460).
  pattern: LookPath → full=`["-C", repo, args...]` → exec.CommandContext (NO shell) → the ONE difference
           line → separate buffers → cmd.Run → exit-code semantics (nil err for non-zero exits).
  gotcha: git.go imports "os/exec" but NOT "os" — ADD "os" (for os.Environ) or build fails. run/runWithInput/
          runWithEnv are all UNEXPORTED gitRunner methods, NOT on the Git interface — keep runWithEnv the same.

- file: internal/git/git_test.go
  section: TestRun_HappyPath (~57) — `g := &gitRunner{workDir: repo}` then `g.run(...)`
  why: the white-box test access pattern (construct *gitRunner directly; run is unexported). Mirror it for
       runWithEnv. Place the new test next to TestRun_HappyPath.
  pattern: `repo := t.TempDir(); initRepo(t, repo); g := &gitRunner{workDir: repo}; … g.runWithEnv(…)`.
  gotcha: initRepo does NOT commit (git init + config user only). The test must NOT need a HEAD tree — use
           `git config --get` (works in an unborn repo) + GIT_CONFIG_COUNT env injection.

- file: .golangci.yml
  why: `disable-all: true` + enable: errcheck, gosimple, govet, ineffassign, staticcheck, **unused**.
       `make lint` runs `golangci-lint run`. An uncalled unexported method trips U1000 — the test is the
       caller that keeps runWithEnv "used" so S1 is lint-green before S2 adds production callers.
  critical: do NOT silence U1000 with //nolint — add the real test (it validates the seam too).

- docfile: plan/010_49117f1f30ab/P1M1T2S1/PRP.md
  why: confirms the PARALLEL task is `--no-verify` in internal/cmd/root.go + docs/cli.md. DIFFERENT file →
       ZERO overlap with internal/git/git.go.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go        # run() (389), runWithInput() (430), import block (3-16), Git interface (87) — EDIT (add "os" import + runWithEnv after line 460)
  git_test.go   # TestRun_HappyPath (~57), initRepo (13) — EDIT (add TestGitRunner_RunWithEnv_PassesEnv)
go.mod / go.sum # unchanged (stdlib "os" only; no new module dep)
# NO docs, NO Git-interface change, NO scoped variants (S2), NO hook runner (M3).
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Edits: internal/git/git.go (import + runWithEnv) + internal/git/git_test.go (one test).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: git.go imports "os/exec" but NOT "os". os.Environ() needs `import "os"`. ADD it to the
// import block (alphabetical, between "io" and "os/exec"). Without it `go build` fails ("undefined: os").
// This is the #1 build-breaker for this task — verify the import lands before anything else.

// CRITICAL: `unused` lint (golangci-lint, enabled in .golangci.yml) flags an unexported method with no
// caller. The contract says "unused until S2", but S1 MUST be lint-green on its own. The included test
// (TestGitRunner_RunWithEnv_PassesEnv) is the caller that keeps runWithEnv "used". Do NOT //nolint it.

// CRITICAL: runWithEnv mirrors runWithInput EXACTLY except ONE line: `cmd.Env = append(os.Environ(),
// extraEnv...)` (NOT cmd.Stdin). Same LookPath → -C repo → []string args (NO shell) → separate buffers →
// exit-code semantics (non-zero git exit ⇒ (stdout, stderr, code, nil); err!=nil only for LookPath/ctx/I/O).
// Do NOT add stdin handling, do NOT change the exit-code contract, do NOT reorder the error branches.

// CRITICAL: append(os.Environ(), extraEnv...) is ADDITIVE (a superset). It preserves the documented
// "child inherits parent env" guarantee (git.go:426-427). Do NOT replace os.Environ() with extraEnv
// (that would clobber PATH/HOME/identity). The scoped vars are ADDED, never substituted.

// GOTCHA: keep runWithEnv UNEXPORTED and OFF the Git interface. run/runWithInput are both unexported
// gitRunner methods (the Git interface at git.go:87 exposes only RevParseHEAD/WriteTree/etc.). The scoped
// variants (S2) are the public surface. Add runWithEnv to the interface ONLY if M3 needs it directly
// (it doesn't — M3 uses the S2 variants).

// GOTCHA: place runWithEnv immediately AFTER runWithInput (line 460), BEFORE the `// ---- Stubs:`
// section (462). Co-located with its siblings — the established discipline (runWithInput doc, 423).

// GOTCHA (test): initRepo does NOT commit. The test must NOT need a HEAD tree. Use `git config --get`
// + GIT_CONFIG_COUNT env injection (works in an unborn repo; deterministic; GIT_CONFIG_* is never in the
// parent env so no duplicate-key risk). Access the runner via `g := &gitRunner{workDir: repo}` (white-box,
// mirror TestRun_HappyPath) — NOT via New(repo) (which returns the Git interface, can't call unexported).
```

## Implementation Blueprint

### Data models and structure

No new types. The method + the one-line difference from runWithInput:

```go
// internal/git/git.go — import block: ADD "os" (between "io" and "os/exec"):
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"           // ← ADD (os.Environ for runWithEnv)
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// internal/git/git.go — immediately AFTER runWithInput (line 460), before the `// ---- Stubs:` section:
// runWithEnv is run() plus a scoped child environment. It exists because run()/runWithInput() leave
// cmd.Env unset (the child inherits the parent env), and the v2.4 hook feature (FR-V3) needs git
// primitives that operate on a THROWAWAY index via GIT_INDEX_FILE=<abs tmp>. It is co-located with
// run()/runWithInput() and shares their structure exactly (LookPath → -C repo → []string args, NO shell →
// separate buffers → errors.As(ExitError) with err==nil for non-zero exits). run() and runWithInput()
// are intentionally left unmodified (see research §1).
//
// Additive: cmd.Env = append(os.Environ(), extraEnv...) — a SUPERSET of the parent env. The documented
// "child inherits the parent environment" guarantee (runWithInput doc) is preserved; PATH/HOME/identity
// env are NOT clobbered. Only the scoped vars (e.g. GIT_INDEX_FILE, GIT_EDITOR) are added.
func (g *gitRunner) runWithEnv(ctx context.Context, repo string, extraEnv []string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	gitPath, lerr := exec.LookPath("git")
	if lerr != nil {
		return "", "", -1, fmt.Errorf("git binary not found in PATH: %w", lerr)
	}
	full := make([]string, 0, len(args)+2)
	full = append(full, "-C", repo) // repo via flag, not cmd.Dir (gotcha G1)
	full = append(full, args...)
	cmd := exec.CommandContext(ctx, gitPath, full...) // []string args, NO shell (PRD §19)
	cmd.Env = append(os.Environ(), extraEnv...)        // ← the ONE difference from run(): scoped child env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
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
```

```go
// internal/git/git_test.go — next to TestRun_HappyPath:
func TestGitRunner_RunWithEnv_PassesEnv(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	g := &gitRunner{workDir: repo} // white-box; mirror TestRun_HappyPath (unexported method access)
	// Inject a config key via git's env-var protocol (GIT_CONFIG_COUNT/KEY/VALUE): deterministic, never
	// in the parent env (no duplicate-key risk), and needs no commit (git config --get works unborn).
	// If cmd.Env is NOT set, the child never sees GIT_CONFIG_COUNT → config --get exits 1, output empty.
	out, _, code, err := g.runWithEnv(context.Background(), repo, []string{
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=stagecoach.envtest",
		"GIT_CONFIG_VALUE_0=passed-via-env",
	}, "config", "--get", "stagecoach.envtest")
	if err != nil || code != 0 {
		t.Fatalf("runWithEnv config --get: code=%d err=%v (cmd.Env likely not set)", code, err)
	}
	if got := strings.TrimSpace(out); got != "passed-via-env" {
		t.Errorf("extraEnv did not reach the child: got %q, want %q (cmd.Env not set?)", got, "passed-via-env")
	}
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: git.go — ADD the "os" import
  - ADD `"os"` to the import block (alphabetical, between "io" and "os/exec").
  - VERIFY: `go build ./internal/git/` succeeds (was failing on undefined: os before the import).
  - GOTCHA: this is the #1 build-breaker; do it FIRST.

Task 2: git.go — ADD runWithEnv (after runWithInput, line 460)
  - ADD the method per the Data Models block, immediately after runWithInput's closing brace (line 460)
    and before the `// ---- Stubs:` section (462).
  - BODY: mirror runWithInput EXACTLY; the ONE difference is `cmd.Env = append(os.Environ(), extraEnv...)`
    (no cmd.Stdin). Keep the LookPath/-C/[]string/separate-buffer/exit-code semantics identical.
  - DOC: state it is run()+scoped-env, co-located, additive (superset — preserves the inheritance guarantee).
  - GOTCHA: keep it UNEXPORTED; do NOT add it to the Git interface.

Task 3: git_test.go — ADD TestGitRunner_RunWithEnv_PassesEnv
  - ADD the test per the Data Models block, next to TestRun_HappyPath. White-box: `g := &gitRunner{workDir: repo}`.
  - Use GIT_CONFIG_COUNT/KEY/VALUE + `git config --get` (deterministic; no commit needed; no parent-env risk).
  - WHY REQUIRED: validates the seam AND keeps `unused` lint green (runWithEnv has a caller).

Task 4: VERIFY (no further file change)
  - RUN `gofmt -w internal/git/git.go internal/git/git_test.go`; `go vet ./internal/git/`; `go build ./...`;
    `go test -race ./internal/git/`; `make lint`.
  - go.mod/go.sum byte-unchanged. Only git.go + git_test.go touched. No scoped variants / hook runner / docs.
```

### Implementation Patterns & Key Details

```go
// The one-line-difference discipline (runWithInput is the template):
//   runWithInput: cmd.Stdin = stdin                  // run() + stdin
//   runWithEnv:    cmd.Env = append(os.Environ(), extraEnv...) // run() + scoped env
// Everything else (LookPath, -C repo, []string args, separate buffers, exit-code semantics) is IDENTICAL.

// Additive env — the safety property:
cmd.Env = append(os.Environ(), extraEnv...) // SUPERSET: parent env + scoped vars (GIT_INDEX_FILE, GIT_EDITOR)
// NOT: cmd.Env = extraEnv                   // would clobber PATH/HOME/identity — WRONG

// White-box test access (runWithEnv is unexported, like run/runWithInput):
g := &gitRunner{workDir: repo}              // NOT New(repo) (returns the Git interface; can't call unexported)
out, _, code, err := g.runWithEnv(ctx, repo, []string{"GIT_INDEX_FILE=" + tmp}, "read-tree", tree)
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — stdlib "os" only; no new module dep. go mod tidy MUST be a no-op.

FROZEN / NOT-EDITED:
  - run() (389) and runWithInput() (430) — intentionally unmodified (the runWithInput doc, 425, states this;
    research §1 confirms). runWithEnv is a NEW sibling, not a refactor of either.
  - The Git interface (87) — runWithEnv stays OFF it (mirrors run/runWithInput). The scoped variants (S2)
    are the public surface.
  - internal/cmd/root.go + docs/cli.md (P1.M1.T2.S1, parallel — --no-verify; different file, no overlap).
  - No docs (internal seam).

DOWNSTREAM CONSUMERS (do NOT implement here):
  - P1.M2.T1.S2 (next): the scoped ReadTreeInto/WriteTreeFrom variants that CALL runWithEnv with
    GIT_INDEX_FILE=<abs tmp> — the production consumers (this task's test is the only caller until S2).
  - M3 (internal/hooks runner): uses the S2 scoped variants (not runWithEnv directly) to scope pre-commit
    to T_start (FR-V3).

NO DATABASE / NO ROUTES / NO CONFIG / NO CLI / NO NEW FILES (besides the test edit).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w internal/git/git.go internal/git/git_test.go
test -z "$(gofmt -l internal/git/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/git/        # Catches a malformed method / wrong exit-branch order.
go build ./...                # THE "os"-import gate: fails with "undefined: os" if you forgot the import.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: clean + build succeeds. If `go build` errors "undefined: os", add "os" to git.go's imports.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test -race ./internal/git/ -v -run 'TestGitRunner_RunWithEnv|TestRun_HappyPath'
# Expected: TestGitRunner_RunWithEnv_PassesEnv PASS (extraEnv reaches the child: config --get ⇒ "passed-via-env")
#   AND TestRun_HappyPath PASS UNCHANGED. If the new test fails with code=1/empty output, cmd.Env was not set.
go test -race ./internal/git/   # the full git suite — no regression (run/runWithInput unchanged).
go test -race ./...             # full module — no regression.
# Expected: green throughout.
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only git.go + git_test.go changed:
git diff --name-only | grep -Ev '^internal/git/git\.go$|^internal/git/git_test\.go$' \
  && echo "UNEXPECTED file changed" || echo "only internal/git/git.go + git_test.go changed (good)"
# Confirm runWithEnv is unexported + OFF the Git interface:
grep -n 'runWithEnv' internal/git/git.go            # the method def + (in the test only) the call
grep -n 'runWithEnv' internal/git/git.go | grep -i 'interface' && echo "BAD: on the interface" || echo "not on Git interface (good)"
```

### Level 4: Creative & Domain-Specific Validation

```bash
# THE lint gate — `unused` is enabled; the test keeps runWithEnv "used":
make lint 2>&1 | grep -i 'runWithEnv\|unused\|U1000' && echo "BAD: runWithEnv flagged" || echo "runWithEnv not flagged by unused (good)"
# Expected: no U1000/unused finding for runWithEnv (the test is its caller). If flagged, the test isn't
#   calling it (re-check Task 3) — do NOT silence with //nolint.
# Additive-env audit (optional): confirm cmd.Env is append(os.Environ(), extraEnv...) — a SUPERSET, not a
# replacement (grep the one line):
grep -n 'cmd.Env = append(os.Environ()' internal/git/git.go
# Expected: exactly one match (the runWithEnv difference line). NOT `cmd.Env = extraEnv`.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `gofmt -l internal/git/`, `go vet ./internal/git/`, `go build ./...` (the "os"-import
      gate), `go mod tidy` no-op.
- [ ] Level 2 green: `TestGitRunner_RunWithEnv_PassesEnv` passes; `go test -race ./internal/git/` +
      `go test -race ./...` green.
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only git.go + git_test.go changed; runWithEnv
      unexported + off the Git interface.
- [ ] Level 4: `make lint` green — no `unused`/U1000 finding for runWithEnv; `cmd.Env` is the additive
      `append(os.Environ(), extraEnv...)`.

### Feature Validation

- [ ] `runWithEnv` exists, mirrors runWithInput except `cmd.Env = append(os.Environ(), extraEnv...)`.
- [ ] `"os"` imported (build succeeds).
- [ ] The test proves extraEnv reaches the child (GIT_CONFIG_COUNT ⇒ `git config --get` returns the value).
- [ ] run()/runWithInput() byte-unchanged.

### Code Quality Validation

- [ ] Mirrors runWithInput's structure + doc style (co-located, shared exit-code semantics, "intentionally
      unmodified" note for run/runWithInput).
- [ ] Additive env (superset — preserves the inheritance guarantee); unexported; off the Git interface.
- [ ] No scope creep into scoped variants (S2), the hook runner (M3), the Git interface, config, or docs.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] No docs (internal exec seam; no user-facing/config/API surface change).
- [ ] go.mod/go.sum byte-unchanged; no new files (only git.go + git_test.go edits).

---

## Anti-Patterns to Avoid

- ❌ Don't forget the `"os"` import. git.go has `"os/exec"` but NOT `"os"`; `os.Environ()` needs `import "os"`.
  Without it `go build` fails ("undefined: os"). Add it FIRST and verify the build.
- ❌ Don't leave runWithEnv without a caller. `unused` lint (enabled) flags uncalled unexported methods
  (U1000). The contract's "unused until S2" makes S1 lint-broken if gated first. The included test is the
  caller — do NOT //nolint around it; add the real test.
- ❌ Don't deviate from the runWithInput template. runWithEnv is `run()` + ONE line (`cmd.Env`), exactly as
  runWithInput is `run()` + `cmd.Stdin`. Same LookPath/-C/[]string/separate-buffers/exit-code-semantics.
  Don't add stdin handling, don't change the error-branch order, don't alter the exit-code contract.
- ❌ Don't REPLACE os.Environ() with extraEnv. `cmd.Env = append(os.Environ(), extraEnv...)` is ADDITIVE
  (a superset) — it preserves PATH/HOME/identity. `cmd.Env = extraEnv` would clobber the parent env and
  break the documented "child inherits parent env" guarantee (git.go:426-427).
- ❌ Don't add runWithEnv to the Git interface. run/runWithInput are unexported gitRunner methods; the Git
  interface (87) exposes only RevParseHEAD/WriteTree/etc. The scoped variants (S2) are the public surface.
  Add it to the interface ONLY if M3 needs it directly (it doesn't).
- ❌ Don't modify run() or runWithInput(). The runWithInput doc (425) explicitly says "run() itself is
  intentionally left unmodified"; research §1 confirms. runWithEnv is a NEW sibling, not a refactor.
- ❌ Don't place runWithEnv away from its siblings. Put it immediately after runWithInput (line 460), before
  the `// ---- Stubs:` section — co-located (the established discipline).
- ❌ Don't write a test that needs a commit/HEAD tree. initRepo does NOT commit. Use `git config --get` +
  GIT_CONFIG_COUNT env injection (works unborn; deterministic; no parent-env/duplicate-key risk). Don't use
  GIT_AUTHOR_NAME (parent-env duplicate-key ambiguity) unless you also clear it carefully.
- ❌ Don't access the runner via `New(repo)` in the test — it returns the Git interface, which can't call the
  unexported runWithEnv. Use `g := &gitRunner{workDir: repo}` (white-box; mirror TestRun_HappyPath).
- ❌ Don't touch internal/cmd/root.go, docs/cli.md (P1.M1.T2.S1, parallel), or add scoped variants (S2) /
  hook runner (M3). This task is git.go + git_test.go only.
- ❌ Don't change go.mod/go.sum or add files. `os` is stdlib; one method + one import + one test.
- ❌ Don't skip `make lint`. It is the gate that confirms the `unused` concern is resolved (the test is the
  caller) AND that no other linter regressed.
