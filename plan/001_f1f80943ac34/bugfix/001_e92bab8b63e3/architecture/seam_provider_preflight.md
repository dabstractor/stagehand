# Provider Pre-flight Check — Root Cause & Architecture (PRD Issue 3)

A missing/uninstalled provider command triggers the rescue path (exit 3) instead of
failing fast before the snapshot with exit 1. This document traces the exact code path
proving the snapshot is taken before the missing command is detected, and identifies the
ideal insertion point for a pre-flight `exec.LookPath` check.

---

## 1. CommitStaged ordering — snapshot (WriteTree) BEFORE first Execute (proof)

`internal/generate/generate.go` — `CommitStaged` runs a 10-step pipeline. The relevant
ordering is **Step 3 (WriteTree) → Step 4 (prompt) → Step 5 (generate loop, first Execute)**.

**Step 3 — snapshot is taken (line 156-161):**
```go
// internal/generate/generate.go:150-161
	// Step 3: snapshot — freeze the index into an immutable tree object.
	// Fails on unresolved merge conflicts (exit 128). BEFORE generation — not a rescue.
	treeSHA, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return Result{}, err
	}
	// *** SNAPSHOT TAKEN — HEAD & committed content are frozen w.r.t. this run. ***
	signal.SetSnapshot(treeSHA, parentSHA, "") // arm rescue (§18.4)
```

**Step 4 — system prompt + recent subjects (lines 163-172):** built once.

**Step 5 — the FIRST `provider.Execute` (line 196)**, inside the generate→dedupe loop:
```go
// internal/generate/generate.go:184-221
	for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
		// Build user payload each attempt (rejection list / retry_instruction change).
		payload := prompt.BuildUserPayload(diff, rejected)
		if parseFail {
			payload = retryInstr + "\n\n" + payload // FR29 corrective preamble
		}

		spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
		if rerr != nil {
			return Result{}, fmt.Errorf("commit staged: render: %w", rerr)
		}

		out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)   // <-- LINE 196: first Execute
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) {
				return Result{}, &RescueError{Kind: ErrTimeout, ...}
			}
			if errors.Is(execErr, context.Canceled) {
				return Result{}, &RescueError{Kind: ErrRescue, ...}
			}
			// Non-zero exit (*exec.ExitError): fall through to ParseOutput.
			// stdout may be partial-valid. Record the cause for eventual rescue.
			lastCause = execErr
		} else {
			lastCause = nil
		}

		m, ok, _ := provider.ParseOutput(out, deps.Manifest)
		if !ok {
			parseFail = true
			candidate = m
			deps.Verbose.VerboseRetry(attempt+1, "parse failed (no valid commit message)")
			continue // FR29 retry (consumes an attempt)
		}
		...
	}
	if !success {
		return Result{}, &RescueError{
			Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
			Candidate: candidate, Cause: lastCause,
		}
	}
```

**Proof of ordering:** `WriteTree` is at **line 156**; the first `provider.Execute` is at
**line 196**. The missing-command failure surfaces only inside `Execute` (at `cmd.Start`),
i.e. **40 lines / 2 pipeline steps after the snapshot is frozen and rescue is armed
(line 161)**. There is no `exec.LookPath` between manifest resolution (which happens in
`buildDeps`, *before* `CommitStaged` is even called) and the snapshot.

**How the `cmd.Start` failure becomes exit 3:** the wrapped start error (executor.go:65,
`fmt.Errorf("provider %q: start: %w", ...)`) is not `DeadlineExceeded` and not `Canceled`,
so it hits the fall-through branch → `lastCause = execErr` → `ParseOutput` on empty stdout
fails → loop retries identically `MaxDuplicateRetries+1` times → `success == false` → returns
`*RescueError{Kind: ErrRescue, TreeSHA: treeSHA, ...}`. The `TreeSHA` is non-empty because the
snapshot was already written. `exitcode.For()` maps `ErrRescue` → **exit 3**, and
`handleGenError` prints the full §18.3 rescue block (tree SHA + manual `git commit-tree`
recipe). A dangling tree object is also left in the object store.

The **identical bug** exists in the DryRun/SystemExtra path (`pkg/stagecoach/stagecoach.go`
`runPipeline`): `WriteTree` at **line 228**, first `Execute` at **lines 265/303**.

---

## 2. Execute (executor.go) — full function, verbatim

`internal/provider/executor.go:44-82`. The doc comment enumerates the error contract; the
`cmd.Start` failure path is lines 64-65.

```go
// internal/provider/executor.go:44-82
func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout string, stderr string, err error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout) // SHADOW — see doc; do not rename
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	if spec.Stdin != "" {
		cmd.Stdin = strings.NewReader(spec.Stdin) // "" ⇒ nil ⇒ /dev/null (CmdSpec contract)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out // separate capture
	cmd.Stderr = &errb
	if len(spec.Env) > 0 {
		cmd.Env = spec.Env // Render populates; nil ⇒ inherit parent env
	}
	setupProcessGroup(cmd) // platform seam (procgroup_*.go): Setpgid + Cancel + WaitDelay

	vb.VerboseCommand(strings.Join(append([]string{spec.Command}, spec.Args...), " "))
	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("provider %q: start: %w", spec.Command, err)   // <-- LINE 65: command-not-found
	}
	signal.RegisterChild(cmd.Process.Pid) // arm signal forwarding (Setpgid ⇒ PGID==PID)
	defer signal.ClearChild()             // clear before return so a later signal can't kill a recycled PID

	if werr := cmd.Wait(); werr != nil {
		vb.VerboseRawOutput(out.String())
		if ctxErr := ctx.Err(); ctxErr != nil {
			return out.String(), errb.String(), ctxErr // timeout → DeadlineExceeded; cancel → Canceled
		}
		return out.String(), errb.String(), fmt.Errorf("provider %q: %w", spec.Command, werr) // exit failure
	}
	vb.VerboseRawOutput(out.String())
	return out.String(), errb.String(), nil
}
```

### Error-contract summary (from the doc comment, executor.go:34-42)

| Outcome | `err` is | Orchestrator intent |
|---|---|---|
| timeout | `context.DeadlineExceeded` | exit 124 + rescue |
| signal/parent cancel | `context.Canceled` | exit 3 + rescue |
| non-zero exit | wrapped `*exec.ExitError` | retry, then rescue |
| **start failure (command not found)** | **wrapped LookPath/start error** | **"command not found", exit 1** |
| success | `nil` | — |

**The gap:** the doc comment *intends* a start failure to map to exit 1, but the orchestrators
(`CommitStaged` line 196-200, `runPipeline` lines 265/303-312) only special-case
`DeadlineExceeded` and `Canceled`. A start-failure error hits the generic "non-zero exit:
fall through to ParseOutput" branch and is indistinguishable from a non-zero exit. Execute
itself does NOT call `exec.LookPath` ahead of `cmd.Start` — `exec.CommandContext` lazily
resolves the path inside `Start()`, so a missing binary fails exactly at line 64.

---

## 3. IsInstalled / LookPath (registry.go)

`internal/provider/registry.go:72-83`. This already implements the exact `exec.LookPath`
probe needed for the pre-flight — but it is only called from `providers list/show`
(`internal/cmd/providers.go:132,159`) and from `buildDeps`'s **auto-detect** branch
(stageshand.go:156-160, only when `cfg.Provider == ""`). It is **never** called on an
explicitly-selected provider.

```go
// internal/provider/registry.go:72-83
// IsInstalled reports whether the provider's discovery command is on $PATH (FR46). It probes
// m.DetectCommand() (Detect if set & non-empty, else Command) via exec.LookPath. A manifest with
// neither Detect nor Command set (DetectCommand()=="") reports false. NOTE cursor is the only built-in
// where Detect ≠ Name (Detect="agent" — the binary), so this correctly probes "agent", not "cursor".
func (r *Registry) IsInstalled(m Manifest) bool {
	cmd := m.DetectCommand()
	if cmd == "" {
		return false
	}
	_, err := exec.LookPath(cmd)
	return err == nil
}
```

`DetectCommand()` (manifest.go:103-112) prefers `Detect` over `Command`; for a user-defined
provider that sets only `Command` (the Issue 3 repro: `command = "/nonexistent/path/agent"`),
`DetectCommand()` returns that command — so `IsInstalled` correctly probes it.

---

## 4. Manifest struct — Command field, and how buildDeps builds it

`internal/provider/manifest.go:33-75` — the `Manifest` struct. The discovery-relevant fields:

```go
// internal/provider/manifest.go:36-42
	// --- discovery (§12.1) ---
	Name       string   `toml:"name"`       // REQUIRED. The identity; registry sets this from the table key.
	Detect     *string  `toml:"detect"`     // nil/"" => DetectCommand falls back to Command.
	Command    *string  `toml:"command"`    // REQUIRED (post-merge). nil in a partial override => inherit.
	Subcommand []string `toml:"subcommand"` // nil => none; inserted between command and flags.
```

`Command` is `*string` (the pointer-scalar design: nil ⇒ inherit on merge; non-nil ⇒ override).
`Validate()` (manifest.go:80-101) enforces `Command != nil && *Command != ""` post-merge, but
does **not** verify the binary exists on `$PATH`:

```go
// internal/provider/manifest.go:80-101
func (m Manifest) Validate() error {
	if m.Name == "" {
		return errors.New("provider manifest: name is required")
	}
	if m.Command == nil || *m.Command == "" {
		return errors.New("provider manifest: command is required")
	}
	...
}
```

`DetectCommand()` (manifest.go:103-112):
```go
// internal/provider/manifest.go:103-112
func (m Manifest) DetectCommand() string {
	if m.Detect != nil && *m.Detect != "" {
		return *m.Detect
	}
	if m.Command != nil {
		return *m.Command
	}
	return ""
}
```

### buildDeps — how the manifest is constructed (pkg/stagecoach/stagecoach.go:145-178)

`buildDeps` resolves the provider manifest from the registry and returns `generate.Deps`. It is
called once by `GenerateCommit` (stageshand.go:91) and feeds BOTH `CommitStaged` (common path)
and `runPipeline` (dry-run/SystemExtra path). This is the **single chokepoint** for manifest
resolution.

```go
// pkg/stagecoach/stagecoach.go:145-178
// buildDeps resolves the provider manifest from the registry and constructs generate.Deps.
func buildDeps(cfg config.Config, repoDir string) (generate.Deps, error) {
	overrides, err := provider.DecodeUserOverrides(cfg.Providers)
	if err != nil {
		return generate.Deps{}, fmt.Errorf("provider overrides: %w", err)
	}

	reg := provider.NewRegistry(overrides)

	name := cfg.Provider
	if name == "" {
		var installed []string
		for _, m := range reg.List() {
			if reg.IsInstalled(m) {            // <-- IsInstalled ONLY used for auto-detect
				installed = append(installed, m.Name)
			}
		}
		name = reg.DefaultProvider(installed)
	}
	if name == "" {
		return generate.Deps{}, fmt.Errorf(
			"no provider configured and none of the built-ins (%s) are installed",
			strings.Join([]string{"pi", "claude", "gemini", "opencode", "codex", "cursor"}, ", "))
	}

	m, ok := reg.Get(name)
	if !ok {
		return generate.Deps{}, fmt.Errorf("unknown provider %q", name)
	}
	if err := m.Validate(); err != nil {
		return generate.Deps{}, fmt.Errorf("provider %q: %w", name, err)
	}

	return generate.Deps{Git: git.New(repoDir), Manifest: m}, nil   // <-- LINE 177: insertion point is right before this
}
```

Note: when `cfg.Provider != ""` (explicit selection — the Issue 3 scenario), the auto-detect
loop is skipped entirely, so `IsInstalled` is never consulted for the chosen provider.

---

## 5. Exit codes & RescueError → exit 3 mapping

`internal/exitcode/exitcode.go:20-28` — the constants:

```go
// internal/exitcode/exitcode.go:20-28
const (
	Success         = 0   // commit created, or dry-run message printed
	Error           = 1   // general error (generation failed, parse failed, agent missing, CAS, usage/flag)
	NothingToCommit = 2   // clean tree after auto-stage, or nothing staged with --no-auto-stage
	Rescue          = 3   // snapshot taken, commit not created — manual recovery printed
	Timeout         = 124 // generation exceeded --timeout (mirrors GNU `timeout`)
)
```

Note `Error = 1` explicitly lists "agent missing" in its comment — confirming exit 1 is the
intended code for a missing provider command.

`exitcode.For()` (exitcode.go:62-90) maps a `*RescueError` via its `Unwrap()` (which returns
`Kind`):

```go
// internal/exitcode/exitcode.go:62-90
func For(err error) int {
	if err == nil {
		return Success
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	if errors.Is(err, generate.ErrNothingToCommit) {
		return NothingToCommit
	}
	// *RescueError.Unwrap()==Kind; check timeout BEFORE rescue (a timeout IS a rescue with Kind=ErrTimeout).
	if errors.Is(err, generate.ErrTimeout) {
		return Timeout
	}
	if errors.Is(err, generate.ErrRescue) {
		return Rescue
	}
	...
	return Error
}
```

`RescueError` (internal/generate/generate.go:66-94): `TreeSHA` is "always non-empty — rescue
fires only after WriteTree". The bug makes a missing-command produce a `*RescueError` with a
non-empty `TreeSHA` (because the snapshot was already taken), so `exitcode.For` correctly but
misleadingly returns **3** instead of **1**. `handleGenError` (internal/cmd/default_action.go)
then prints the full §18.3 rescue block including the `git commit-tree … | xargs git update-ref`
manual-recovery command — a recovery recipe that should never be shown for a simple
"command not found" misconfiguration.

**Key invariant:** if the pre-flight check in `buildDeps` returns a plain error (not a
`*RescueError`), `exitcode.For` falls through to the final `return Error` (exit 1), and
`handleGenError`'s generic branch returns `exitcode.New(exitcode.Error, err)` so main prints
`stagecoach: <msg>`. No snapshot, no rescue block, no dangling tree object.

---

## 6. Existing tests for the missing-provider-command scenario

There is **only one** test touching this, and it tests `Execute` in isolation — NOT the pipeline:

`internal/provider/executor_test.go:128-137`:
```go
// 8. Command not found ⇒ wrapped Start() error.
func TestExecute_CommandNotFound(t *testing.T) {
	spec := CmdSpec{Command: "definitely-not-a-real-binary-xyz-stagecoach", Env: os.Environ()}
	_, _, err := Execute(context.Background(), spec, 3*time.Second, nil)
	if err == nil {
		t.Fatal("err = nil, want non-nil (command not found)")
	}
	if !strings.Contains(err.Error(), "start") && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "executable") {
		t.Errorf("err = %v, want it to mention start/not found/executable", err)
	}
}
```

**There is NO test** at the `CommitStaged` level (`internal/generate/*_test.go`) or the
`GenerateCommit` level (`pkg/stagecoach/stagecoach_test.go`) that:
- constructs a manifest pointing at a missing command and asserts `CommitStaged`/`GenerateCommit` fails fast with exit 1 BEFORE taking a snapshot, or
- asserts no dangling tree object is written, or
- asserts the error is NOT a `*RescueError`.

A grep for `LookPath|not found|notFound|missing|uninstalled|preflight` across
`internal/generate/*_test.go` and `pkg/stagecoach/*_test.go` returns no matches (only unrelated
"object missing?" / "preamble missing" strings).

`internal/provider/registry_test.go:149-168` (`TestIsInstalled`) covers `IsInstalled` itself
(false for bogus, true for `go`, Detect-wins-over-Command) — so the primitive is tested; the
gap is that it is never wired into the explicit-provider resolution path.

---

## 7. Ideal insertion point for the pre-flight check

**Recommended: `buildDeps` in `pkg/stagecoach/stagecoach.go`, immediately after `m.Validate()`
(line 173-175) and before the `return generate.Deps{...}` (line 177).**

### Why buildDeps is the right seam

1. **Single chokepoint.** `buildDeps` is called once by `GenerateCommit` (stageshand.go:91)
   and its returned `Deps` feed BOTH code paths:
   - the common path → `generate.CommitStaged` (stageshand.go:96), and
   - the advanced path → `runPipeline` (stageshand.go:108, used by DryRun/SystemExtra).
   A check here protects both pipelines with one edit. (Note: `CommitStaged` is also a
   public-ish internal API called directly by tests with stub manifests; a check there would
   need its own registry/LookPath and would not be exercised by the public surface. The public
   contract is `GenerateCommit`, which always goes through `buildDeps`.)
2. **Runs BEFORE WriteTree.** `buildDeps` completes and returns before `GenerateCommit` calls
   either pipeline, so the check fails before any snapshot object is written — satisfying the
   PRD §18.2 "pre-generation" requirement and leaving no dangling tree.
3. **Registry already in scope.** `reg` (line 150) and the resolved `m` (line 169) are both
   in hand, so `reg.IsInstalled(m)` (or a direct `exec.LookPath`) is a one-liner with no new
   dependencies or imports.
4. **Matches the existing "unknown provider" / "Validate" pattern.** buildDeps already returns
   plain errors for `unknown provider %q` (line 171) and `Validate` failures (line 173-175),
   which `exitcode.For` maps to exit 1. A missing-command error fits this pattern exactly.

### Suggested shape (for the implementer — not committed here)

After line 175 (`if err := m.Validate(); err != nil { ... }`) and before line 177:

```go
	// Pre-flight (PRD §18.2): fail fast with exit 1 BEFORE the snapshot if the provider
	// command is not on $PATH. Without this, the missing binary is only detected inside
	// Execute's cmd.Start (well after WriteTree), surfacing as a misleading exit-3 rescue.
	if !reg.IsInstalled(m) {
		return generate.Deps{}, fmt.Errorf(
			"provider %q: command %q not found. Is the agent installed?",
			name, m.DetectCommand())
	}
```

This reuses the existing, tested `Registry.IsInstalled`/`DetectCommand` seam (registry.go:76,
manifest.go:103). Because `Validate()` already guaranteed `Command` is non-nil/non-empty,
`DetectCommand()` returns a non-empty string here, so `IsInstalled` performs a real
`exec.LookPath` (not the empty-string short-circuit).

### Alternatives considered (NOT recommended)

- **`CommitStaged` (generate.go, before line 156 WriteTree):** would protect direct callers
  of the internal API, but `CommitStaged` receives `Deps{Manifest}` not a `*Registry`, so it
  cannot call `IsInstalled` without an API change or importing `exec` directly. It also
  duplicates the check that belongs in resolution. The public path (`GenerateCommit`) is
  already covered by `buildDeps`.
- **`Resolve()` (manifest.go):** `Resolve` is a pure data transform (fills nil defaults) and
  is called repeatedly (e.g. for display); adding `exec.LookPath` side-effects there violates
  its purity and would slow every `providers show`.
- **`default_action.go` (CLI layer):** `runDefault` already does a lightweight
  `reg.Get(cfg.Provider)` existence check (default_action.go ~line 135) but deliberately NOT an
  `IsInstalled`/LookPath — putting the real pre-flight there would leave the library API
  (`pkg/stagecoach.GenerateCommit`) unprotected and would not fix the DryRun path uniformly.

---

## Files Retrieved

1. `internal/generate/generate.go` (lines 150-221) — `CommitStaged` snapshot (WriteTree, line 156) and the generate loop's first Execute (line 196) + the fall-through→rescue logic.
2. `internal/provider/executor.go` (lines 44-82) — full `Execute`; the `cmd.Start` failure path (line 64-65).
3. `internal/provider/registry.go` (lines 72-83) — `IsInstalled` (`exec.LookPath(m.DetectCommand())`).
4. `internal/provider/manifest.go` (lines 36-42, 80-101, 103-112) — `Manifest.Command` field, `Validate`, `DetectCommand`.
5. `pkg/stagecoach/stagecoach.go` (lines 83-108, 145-178, 206-312) — `GenerateCommit` delegation; `buildDeps` (the insertion target); `runPipeline` (dry-run path with the same WriteTree-before-Execute ordering).
6. `internal/exitcode/exitcode.go` (lines 20-28, 62-90) — exit-code constants and `For()` mapping `*RescueError`→3 / generic→1.
7. `internal/generate/generate.go` (lines 66-94) — `RescueError` type (TreeSHA "always non-empty").
8. `internal/generate/rescue.go` (lines 18-72) — `FormatRescue` (the §18.3 block printed on exit 3).
9. `internal/cmd/default_action.go` (lines ~130-200) — `runDefault` provider-validation + `handleGenError` rescue/CAS/generic matrix.
10. `internal/provider/executor_test.go` (lines 128-137) — the only existing missing-command test (Execute-level only).
11. `plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/prd_snapshot.md` (Issue 3, lines 105-125) — the bug report and suggested fix confirming the `buildDeps`/pre-`WriteTree` location.

## Start Here

`pkg/stagecoach/stagecoach.go` — open `buildDeps` (line 145). The pre-flight `reg.IsInstalled(m)`
check goes immediately after `m.Validate()` (line 173-175), before the `return` at line 177.
This single edit fixes both `CommitStaged` (common path) and `runPipeline` (dry-run/SystemExtra)
and runs before any `WriteTree`. Add a `pkg/stagecoach/stagecoach_test.go` test that registers a
`[provider.missing]` with `command = "/nonexistent/..."` via a repo-local `.stagecoach.toml`,
calls `GenerateCommit`, and asserts: (a) the error is NOT a `*RescueError` (`errors.As(err,
&re)` is false), (b) `exitcode.For(err) == 1`, and (c) no new tree object was written to the
repo (e.g. `git cat-file -t <tree>` of any pre-run snapshot fails, or `Count-objects` is
unchanged).
