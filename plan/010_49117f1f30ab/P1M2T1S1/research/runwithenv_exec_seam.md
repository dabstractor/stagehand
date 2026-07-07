# Research: runWithEnv exec seam in internal/git (P1.M2.T1.S1)

Verified against the live codebase. Source of truth for the env-passing exec primitive.

## Why this seam exists (codebase_reality.md §1)

`internal/git/git.go` has exactly TWO exec seams — `run()` (git.go:389) and `runWithInput()` (git.go:430)
— and NEITHER sets `cmd.Env` (the child inherits the parent env). `GIT_INDEX_FILE` appears NOWHERE in
internal/; every index-mutating primitive (ReadTree/WriteTree/Add/AddAll/OverlayTreePaths/FreezeWorkingTree)
operates on the repo's single default `.git/index`. The v2.4 hook-execution feature (FR-V3: `pre-commit`
scoped to the snapshot tree `T_start`) needs a THROWAWAY index — `GIT_INDEX_FILE=<abs tmp>` threaded
through read-tree/write-tree/update-index. That requires a NEW env-passing seam. This task adds it.

## The two existing seams (the structure to mirror)

`run()` (389-414): `exec.CommandContext(ctx, gitPath, "-C", repo, args...)`; argv as []string, NO shell
(PRD §19); separate stdout/stderr buffers; exit-code semantics (non-zero git exit → (stdout, stderr,
code, nil); err != nil only for infra failures). cmd.Env NEVER set.

`runWithInput()` (430-460): the closest analog — "run() plus ONE difference" (`cmd.Stdin = stdin`). Its
doc (424-428) states it "shares its structure exactly (LookPath → -C repo → separate buffers →
errors.As(ExitError) with err==nil for non-zero exits). run() itself is intentionally left unmodified
(see research §1)." runWithEnv is the SAME pattern with `cmd.Env` as the one difference.

## runWithEnv — mirror runWithInput, swap the one line

```go
func (g *gitRunner) runWithEnv(ctx context.Context, repo string, extraEnv []string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	gitPath, lerr := exec.LookPath("git")
	if lerr != nil {
		return "", "", -1, fmt.Errorf("git binary not found in PATH: %w", lerr)
	}
	full := make([]string, 0, len(args)+2)
	full = append(full, "-C", repo)
	full = append(full, args...)
	cmd := exec.CommandContext(ctx, gitPath, full...) // []string args, NO shell (PRD §19)
	cmd.Env = append(os.Environ(), extraEnv...)        // ← the ONE difference from run(): scope the child env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	runErr := cmd.Run()
	stdout, stderr = out.String(), errb.String()
	if runErr == nil {
		return stdout, stderr, 0, nil
	}
	if cerr := ctx.Err(); cerr != nil {
		return stdout, stderr, -1, cerr
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return stdout, stderr, exitErr.ExitCode(), nil
	}
	return stdout, stderr, -1, runErr
}
```

## ⚠️ GOTCHA #1 — `"os"` is NOT imported in git.go

git.go's import block has `"os/exec"` but NOT `"os"`. `os.Environ()` requires `import "os"`. Add it to
the import block (alphabetical: between "io" and "os/exec"). WITHOUT this, `go build` fails
("undefined: os"). This is the easy-to-miss build-breaker.

## ⚠️ GOTCHA #2 — `unused` lint is ENABLED; an uncalled runWithEnv trips U1000

`.golangci.yml`: `disable-all: true` + enable: errcheck, gosimple, govet, ineffassign, staticcheck,
**unused**. `make lint` runs `golangci-lint run`. An unexported method with NO caller anywhere in the
package trips U1000. The contract says "unused until S2" — but if the orchestrator gates S1 on lint
before S2 lands, S1 FAILS. Therefore S1 MUST include a focused unit test that calls runWithEnv (keeps it
"used" → lint green) AND validates the seam. S2 (P1.M2.T1.S2) will add the production callers (the
scoped ReadTreeInto/WriteTreeFrom variants).

## The test (deterministic, no commit needed, no parent-env risk)

Use git's env-based config injection (`GIT_CONFIG_COUNT` / `GIT_CONFIG_KEY_<n>` / `GIT_CONFIG_VALUE_<n>`)
+ `git config --get`. These env vars are NEVER in the parent env (git-specific protocol), so there is no
duplicate-key/override ambiguity, and the assertion is deterministic. `initRepo` does NOT commit, but
`git config --get` works in a fresh/unborn repo (it reads config, independent of HEAD). If cmd.Env is NOT
set (the bug), the child never sees GIT_CONFIG_COUNT → `git config --get stagecoach.envtest` exits 1 with
empty output → the test fails loudly.

```go
func TestGitRunner_RunWithEnv_PassesEnv(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	g := &gitRunner{workDir: repo} // white-box; mirror TestRun_HappyPath (git_test.go:57)
	// Inject a config key via git's env-var protocol (deterministic; never in parent env).
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
Place in `internal/git/git_test.go` next to TestRun_HappyPath (the existing unexported-run test).

## Additive semantics — preserves the "inherits parent env" guarantee

`cmd.Env = append(os.Environ(), extraEnv...)` is a SUPERSET (parent env + extras), not a replacement.
The documented guarantee at git.go:426-427 ("the child inherits the parent environment") is preserved —
runWithEnv merely ADDS scoped vars (GIT_INDEX_FILE, GIT_EDITOR). It does NOT clobber PATH, HOME, or the
configured user.identity env. This is the FR-V6 model (the hook sees git's env + the scoped index).

## Placement + interface decision

- PLACE runWithEnv immediately AFTER runWithInput (ends line 460), co-located with run/runWithInput
  (the "co-located with run()" discipline, before the `// ---- Stubs:` section at 462).
- KEEP IT UNEXPORTED (not on the Git interface) — mirrors run/runWithInput (both unexported gitRunner
  methods, NOT on the Git interface at git.go:87). The scoped variants (S2) are the public surface.
  Add it to the Git interface ONLY if internal/hooks (M3) needs to call it directly; the contract PREFERS
  the scoped-variants-as-public-surface shape, so leave it unexported.

## Scope boundary (no conflict)

- **P1.M1.T2.S1 (parallel)** is `--no-verify` in internal/cmd/root.go + docs/cli.md. DIFFERENT file →
  ZERO overlap with internal/git/git.go.
- **P1.M2.T1.S2 (next)** adds the scoped ReadTreeInto/WriteTreeFrom variants that CALL runWithEnv (the
  production consumers). This task (S1) adds the seam + the test; S2 adds the callers.
- **M3 (internal/hooks runner)** eventually invokes hooks; it uses the scoped variants (S2), not
  runWithEnv directly (per the unexported preference).
- This task touches ONLY `internal/git/git.go` (import + runWithEnv) + `internal/git/git_test.go` (test).
  No docs, no interface change, no scoped variants, no hook runner.

## DOCS: none — internal exec seam, no user-facing/config/API surface change.
