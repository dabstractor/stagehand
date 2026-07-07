# Seam Facts — P1.M3.T4.S1 (verified against the working tree)

> Exact signatures/behaviors the stub + helper must match. All line numbers from the current tree.

## The provider pipeline the stub plugs into (P1.M2 — all DONE, read-only)

```
Manifest (T1)  ──Render──►  CmdSpec (T4)  ──Execute──►  stdout (T5)  ──ParseOutput──►  msg (T6)
```

### `provider.CmdSpec` — internal/provider/render.go:23
```go
type CmdSpec struct {
    Command string   // the executable path (we set this to the built stub binary path)
    Args    []string // flags AFTER command (Render builds these; stub needs none beyond payload routing)
    Stdin   string   // payload to pipe; "" → executor uses /dev/null
    Env     []string // os.Environ() + manifest Env "K=V" (manifest appended last → last-wins override)
}
```

### `provider.Execute` — internal/provider/executor.go:44
```go
func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration) (stdout string, stderr string, err error)
```
- timeout > 0 ⇒ `context.WithTimeout` shadows ctx; `ctx.Err()` distinguishes timeout vs cancel.
- `cmd.Stdin = strings.NewReader(spec.Stdin)` when `spec.Stdin != ""`; else nil → /dev/null.
- stdout/stderr captured to SEPARATE `bytes.Buffer`s, returned even on error.
- `cmd.Env = spec.Env` when `len(spec.Env) > 0`; else inherits parent env.
- `setupProcessGroup(cmd)` (procgroup_unix.go) ⇒ child is its own PGID leader; on ctx cancel
  `cmd.Cancel` sends SIGTERM to `-pid` (whole group), `WaitDelay`=3s → SIGKILL escalation.
- **Error contract:** timeout ⇒ `context.DeadlineExceeded`; cancel ⇒ `context.Canceled`;
  non-zero exit ⇒ wrapped `*exec.ExitError`; start miss ⇒ wrapped LookPath error; success ⇒ nil.

### `Manifest.Render` — internal/provider/render.go (method on `Manifest`)
```go
func (m Manifest) Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error)
```
- Calls `m.Validate()` then `m.Resolve()` (nil-pointer-safe on a copy).
- Env construction (the seam the stub uses): `env := os.Environ(); for k,v := range r.Env { env = append(env, k+"="+v) }; spec.Env = env`.
- For the stub manifest (`PromptDelivery="stdin"`, no flags), Render yields a CmdSpec whose Args is
  empty and whose Stdin is the payload. The stub's Env knobs ride along in `spec.Env`. ✓

### `provider.Manifest` — internal/provider/manifest.go (pointer-scalar design)
- The fields the stub helper sets: `Name`, `Command` (*string), `PromptDelivery` (*string="stdin"),
  `Output` (*string="raw"), `StripCodeFence` (*bool), `Env` (map[string]string).
- Helpers `strPtr`/`boolPtr` are UNEXPORTED in package provider → the stubtest helper (a different
  package) CANNOT call them. **It must construct pointer fields itself** with local `&`-helpers:
  `s := "stdin"; m.PromptDelivery = &s`. (Verified: `strPtr` is unexported in manifest.go.)
- `Env` is a plain `map[string]string` (NOT a pointer) — assign directly: `m.Env = map{...}`.

### `provider.ParseOutput` — internal/provider/parse.go (what consumes the stub's stdout)
```go
func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)
```
- `ok = msg != ""` after trim+normalize. **An empty stub output ⇒ ok=false ⇒ orchestrator retries**
  (FR29) — this is the "parse-failure" lever the stub exposes via an empty `OUT` or blank script line.

## The git seam S2's tests will use (P1.M1 — DONE, read-only) — for context only

- `git.New(workDir string) Git` (git.go).
- Test fixture convention (git_test.go): `repo := t.TempDir(); initRepo(t, repo)`; identity set via
  `GIT_AUTHOR_*`/`GIT_COMMITTER_*` env (git_test.go) OR repo-local `git config user.name/email`
  (committree_test.go `setIdentityConfig`). The stub subtask itself needs NO git — only S2 does.

## Module facts
- `module github.com/dustin/stagecoach`, `go 1.22`, deps: go-toml/v2 + pflag. (go.mod)
- `go version go1.26.4` in this env (≥ 1.22 ✓).
- `go list ./...` currently lists: cmd/stagecoach, internal/{config,generate,git,prompt,provider}.
  After this subtask it ALSO lists `cmd/stubagent` and `internal/stubtest`.

## No existing "stub"/"stubtest"/"stubagent" references (grep clean) — names are free.
