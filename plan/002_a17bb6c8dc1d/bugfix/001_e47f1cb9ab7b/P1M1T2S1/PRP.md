# PRP — P1.M1.T2.S1: E2E integration test asserting `--provider <default_provider>` (not the manifest name)

## Goal

**Feature Goal**: Add an end-to-end (CLI-level) integration test that proves the
provider/sub-provider conflation fix (P1.M1.T1.S1 + S2) works through the **real cobra
CLI**: when a pi-shaped config sets `default_provider = "openrouter"`, the rendered agent
command emits `--provider openrouter` (the sub-provider) and does **NOT** emit
`--provider pi` (the manifest name). The test covers BOTH execution paths — single-commit
(`generate.CommitStaged`) and multi-commit decomposition (the planner role) — using the
bundled `cmd/stubagent` as a stand-in for pi so no real model calls are made.

**Deliverable**: One new test (two `t.Run` subtests) added to
`internal/cmd/default_action_test.go`, package `cmd`. It drives `rootCmd.SetArgs(...)` +
`Execute(ctx)` against a temp git repo + a pi-shaped `STAGECOACH_CONFIG` TOML whose
`[provider.pi]` points `command` at the compiled stub binary, then asserts on the captured
**stderr** `DEBUG: command:` line.

**Success Definition**: `go test ./internal/cmd/ -run <TestName> -v` passes; the rendered
command contains `--provider openrouter` and omits `--provider pi` on BOTH the
single-commit and decompose paths. This is the regression guard that prevents the
conflation (PRD Issue 1) from being reintroduced.

## Why

- **Business value**: PRD Issue 1 is Critical — every common pi configuration (the
  `config init` bootstrap, `--provider pi`, `git config stagecoach.provider pi`,
  `STAGECOACH_PROVIDER=pi`) emitted `--provider <manifest-name>`, which is not a valid pi
  sub-provider and silently overrode the user's `default_provider`. The unit tests missed
  it because they invoked `Render` *directly* with the sub-provider string, bypassing the
  caller conflation. This E2E test closes that detection gap at the CLI seam.
- **Integration with existing features**: It exercises the full `PersistentPreRunE →
  config.Load → runDefault/runDecompose → Render → provider.Execute → ui.Verbose` chain,
  proving the fix holds where users actually invoke it.
- **Scope guard**: This is a TEST-ONLY change (no source edits, no docs). It depends on
  the already-complete S1 (`generate.go`) and S2 (four `decompose/*` role files) fixes. It
  must not alter any production behavior.

## What

A Go test in `internal/cmd/default_action_test.go` that, for each of two paths:

1. Builds the stub binary via `stubtest.Build(t)`.
2. Creates a temp git repo (real git — no mocks), with an initial commit.
3. Writes a pi-shaped config TOML to a temp path **outside** the repo (no `.stagecoach.toml`
   in the repo) and points `STAGECOACH_CONFIG` at it.
4. Sets `STAGECOACH_PROVIDER=pi` and the path-appropriate `STAGECOACH_STUB_OUT` via
   `t.Setenv`; isolates `HOME`/`XDG_CONFIG_HOME`.
5. Drives `rootCmd.SetArgs(...)` + `Execute(ctx)`, capturing stdout into one buffer and
   **stderr into another**.
6. Asserts the stderr buffer contains `DEBUG: command:` with `--provider openrouter` and
   does **not** contain `--provider pi`.

### Success Criteria

- [ ] Single-commit path (staged change + `--dry-run --verbose --no-color`): stderr
      contains `--provider openrouter`, not `--provider pi`; stdout equals the canned
      dry-run message.
- [ ] Decompose path (dirty/un-staged tree + `--verbose --no-color`, no `--dry-run`):
      stderr contains `--provider openrouter`, not `--provider pi`; one real commit is
      created (HEAD advances).
- [ ] `go build ./...` is clean; `go test ./internal/cmd/ -run <TestName> -v` passes; the
      full `go test ./...` suite still passes.

## All Needed Context

### Context Completeness Check

If someone knew nothing about this codebase, the references + the inline test skeleton
below give them everything: which file to edit, which helpers to reuse, the exact config
TOML, the exact stub outputs, the stream to assert on, and the four non-obvious gotchas
(G1–G6) that will silently break the test if ignored.

### Documentation & References

```yaml
# MUST READ before writing the test.
- file: internal/cmd/default_action_test.go
  why: The file you are EDITING. Study the established CLI-test pattern: saveRootState/
       restoreRootState bracketing, rootCmd.SetOut/SetErr buffers, rootCmd.SetArgs, and
       setupStubRepo (the stub-via-config helper to mirror). Model the new test on
       TestRunDefault_VerboseFlag (asserts "DEBUG: command:" on the STDERR buffer) and
       TestRunDefault_ConfigFlagHonored_Issue1 (HOME/XDG isolation + --config path).
  pattern: every test calls saveRootState(t)+defer restoreRootState(...), sets out/err
           buffers, SetArgs, then Execute(ctx); cleanup via t.Setenv/t.TempDir.
  gotcha: (G1) verbose output goes to the STDERR buffer (errBuf), NOT stdout.

- file: internal/cmd/root_test.go
  why: Reuse these EXISTING package-internal helpers — do NOT reinvent:
       initRepo(t,dir), writeConfigFile(t,dir,relPath,body), chdir(t,dir),
       saveRootState(t) / restoreRootState(t,...), writeFile, stageFile, commitRaw,
       headSHA, gitOut, runGit.
  pattern: helpers live in root_test.go and are shared across all *_test.go in package cmd.

- file: internal/generate/generate.go   # ~L192
  why: The S1 fix site. CommitStaged calls `deps.Manifest.Render(cfg.Model, "", sysPrompt,
       payload)` — passes "" so Render falls back to DefaultProvider. This test proves it.
  critical: the rendered command for the single-commit path comes from THIS Render call.

- file: internal/decompose/planner.go   # L98
  why: The S2 fix site for the planner role: `deps.Roles.Planner.Render(mdl, "", sysPrompt,
       payload, provider.RenderBare)`. The decompose subtest proves the planner renders the
       sub-provider.
  critical: the planner is the FIRST (and only) role invoked in the single-shortcut path.

- file: internal/provider/render.go
  why: Render's fallback: `providerToUse := provider; if providerToUse == "" {
       providerToUse = *r.DefaultProvider }`. With "" passed + DefaultProvider="openrouter"
       → emits "--provider openrouter". Emits nothing when DefaultProvider=="" (pi default).
  pattern: token order is command, subcommand, (provider_flag, provider), (model_flag,
           model), (system_prompt_flag, sys), bare/tooled flags, print_flag, payload-by-delivery.

- file: internal/provider/executor.go   # VerboseCommand call
  why: `vb.VerboseCommand(strings.Join(append([]string{spec.Command}, spec.Args...), " "))`
       is what produces the `DEBUG: command: <argv>` line. It logs ARGV only (Command+Args),
       never Env (security: no API keys).
  critical: (G1) the writer is the ui.Verbose.w which the CLI sets to stderr.

- file: internal/ui/verbose.go
  why: VerboseCommand format is literally `fmt.Fprintln(v.w, "DEBUG: command: "+cmd)`.
       v.w is the stderr writer; nil/off ⇒ no-op. Confirms the assertion string + stream.
  gotcha: nil-safe + on-gated — verbose MUST be on (cfg.Verbose) or nothing is printed.

- file: internal/cmd/default_action.go
  why: runDefault (single path: GenerateCommit) and runDecompose (decompose path:
       ResolveRoles → decompose.Decompose). Shows shouldDecompose() returns FALSE for
       --dry-run (G5) and that Verbose sink = cmd.ErrOrStderr() (stderr).
  critical: (G3) runDecompose calls ResolveRoles BEFORE Decompose — tooled_flags required.

- file: internal/decompose/roles.go   # ResolveRoles
  why: (G3) iterates ALL FOUR roles incl. stager; FR-D4 fallback errors if the stager
       provider has empty TooledFlags and no other installed provider is stager-capable.
  critical: [provider.pi] MUST set non-empty tooled_flags or ResolveRoles errors before
            the planner renders → assertion falsely fails.

- file: internal/decompose/decompose.go   # Decompose + runSingleShortcut
  why: (G4) when the planner returns Single==true, Decompose takes runSingleShortcut:
       AddAll→WriteTree→dup-check planner msg→publish. Stager/message/arbiter are NOT
       invoked. One stub round-trip, one commit, clean test.
  critical: single:true + a "message" field is the planner contract for the shortcut.

- file: internal/prompt/planner.go   # PlannerOutput JSON
  why: PlannerOutput JSON shape: {"count":N,"single":bool,"commits":[{title,description}],
       "message":"..."}. message present iff single==true. The exact planner JSON the stub
       must print for the decompose subtest.

- file: internal/stubtest/stubtest.go   # Build
  why: stubtest.Build(t) compiles cmd/stubagent ONCE per test process (cached) → absolute
       path to the stub binary. Point [provider.pi].command at it.
  gotcha: returns an ABSOLUTE temp path; reg.IsInstalled treats it as installed.

- file: cmd/stubagent/main.go
  why: the stub reads STAGECOACH_STUB_OUT from its env and prints it verbatim to stdout
       (ParseOutput trims). This is the seam the test controls.
  gotcha: (G2) the stub's env = os.Environ() + manifest Env; manifest Env WINS. So control
          STAGECOACH_STUB_OUT via t.Setenv and OMIT it from [provider.pi.env].

- file: internal/config/load.go   # ConfigPathOverride / STAGECOACH_CONFIG
  why: STAGECOACH_CONFIG (or --config) → ConfigPathOverride → used as the global file path
       (explicit=true skips globalConfigPath() discovery). Still merge built-ins; isolate
       HOME/XDG (G6).
```

### Current Codebase tree (relevant subset)

```bash
cmd/stubagent/main.go                 # the fake agent binary (compiled in-test)
internal/cmd/
  root.go                             # rootCmd, flags (--verbose/--no-color/--dry-run/--config), Execute()
  default_action.go                   # runDefault (single) + runDecompose + shouldDecompose
  default_action_test.go              # ← EDIT: add the new test here
  root_test.go                        # shared helpers (initRepo, writeConfigFile, chdir, saveRootState...)
  config_test.go                      # more CLI-test patterns
internal/generate/generate.go         # S1 fix: Render(model, "", ...)  (~L192)
internal/decompose/{planner,stager,message,arbiter}.go   # S2 fix: Render(mdl, "", ...)
internal/decompose/roles.go           # ResolveRoles (tooled_flags / stager fallback)
internal/decompose/decompose.go       # Decompose + runSingleShortcut
internal/provider/{render,executor}.go
internal/ui/verbose.go                # "DEBUG: command: <argv>" → stderr sink
internal/stubtest/stubtest.go         # Build(t)
internal/prompt/planner.go            # PlannerOutput JSON contract
```

### Desired Codebase tree with files to be added/changed

```bash
internal/cmd/default_action_test.go   # MODIFY — append ONE new test func (two t.Run subtests)
plan/.../P1M1T2S1/research/findings.md # (already created — research notes)
```

No other files change. No new packages.

### Known Gotchas of our codebase & Library Quirks

```go
// G1 (CRITICAL): verbose "DEBUG: command:" is written to STDERR (cmd.ErrOrStderr()),
// NOT stdout. The work-item contract step (e) says "captures stdout" — that is wrong.
// Assert on the STDERR buffer (rootCmd.SetErr(&errBuf)). Proof: TestRunDefault_VerboseFlag.

// G2 (CRITICAL): provider.Render builds cmd.Env = os.Environ() + manifest.Env (manifest
// appended LAST → exec last-wins → manifest OVERRIDES). So a value in [provider.pi.env]
// beats t.Setenv. Because the two subtests need DIFFERENT STAGECOACH_STUB_OUT values, set it
// via t.Setenv and DO NOT put STAGECOACH_STUB_OUT in [provider.pi.env]. (Mirror setupStubRepo.)

// G3 (CRITICAL, decompose only): ResolveRoles resolves the stager role and ERRORS if the
// provider has empty tooled_flags (FR-D4 fallback finds no stager-capable provider). That
// error fires BEFORE the planner renders → no "DEBUG: command:" → false failure. FIX:
// [provider.pi] MUST include a non-empty tooled_flags (e.g. ["--yes"]).

// G4 (decompose only): drive the planner single-shortcut (single:true + message) so
// runSingleShortcut commits without invoking stager/message/arbiter (the stub can't run git
// and all roles share one binary). One round-trip, one commit, clean assertion.

// G5: shouldDecompose returns FALSE when --dry-run is set. Single subtest uses --dry-run;
// decompose subtest must NOT (it commits for real).

// G6: isolate HOME + XDG_CONFIG_HOME to a temp dir and keep the repo free of
// .stagecoach.toml so the provider resolves ONLY from the STAGECOACH_CONFIG file.
```

## Implementation Blueprint

### The config TOML (pi-shaped stub; shared by both subtests)

Write this to a temp path via `os.WriteFile`; point `t.Setenv("STAGECOACH_CONFIG", path)` at
it. `bin` = `stubtest.Build(t)`. Note `tooled_flags` (G3) and the ABSENCE of
`STAGECOACH_STUB_OUT` in the env block (G2 — set per-subtest via `t.Setenv`).

```toml
config_version = 2

[defaults]
provider = "pi"

[provider.pi]
command             = "<bin>"      # absolute path from stubtest.Build(t)
detect              = "<bin>"      # DetectCommand falls back to command; set for parity with the repro
provider_flag       = "--provider"
default_provider    = "openrouter" # <<< the sub-provider that MUST be rendered
model_flag          = "--model"
default_model       = "gpt-5.4-nano"
system_prompt_flag  = "--system"
prompt_delivery     = "stdin"
print_flag          = "-p"
output              = "raw"
tooled_flags        = ["--yes"]    # G3: REQUIRED so ResolveRoles can resolve the stager role

# NOTE: do NOT put STAGECOACH_STUB_OUT here (G2 — it would override t.Setenv and you need
# different values per subtest). Set it via t.Setenv in each subtest.
```

### Stub outputs per subtest

```go
// Single-commit path: any valid commit-message string (ParseOutput trims).
const singleStubOut = "feat: single path provider render"

// Decompose path: planner single-shortcut JSON (runSingleShortcut commits the message
// directly; stager/message/arbiter are NOT invoked). count==1 + single==true + message.
const decomposeStubOut = `{"count":1,"single":true,` +
    `"commits":[{"title":"add dirty","description":"dirty.txt"}],` +
    `"message":"feat: decompose path provider render"}`
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: APPEND test to internal/cmd/default_action_test.go
  - ADD: a table/factory that writes the pi-shaped config TOML (above) to a temp path and
          returns that path. Use os.WriteFile + fmt.Sprintf for the <bin> substitution.
  - REUSE: stubtest.Build(t) for <bin>; t.TempDir() for the config path; os.WriteFile.
  - NAMING: helper writePiStubConfig(t, bin) string (local to this test file).
  - PLACEMENT: internal/cmd/default_action_test.go, after TestRunDefault_VerboseFlag.

Task 2: ADD subtest "single_commit" (the v1 path via generate.CommitStaged)
  - SETUP: saveRootState(t) + defer restoreRootState(...).
  - REPO: t.TempDir(); initRepo(t, repo); chdir(t, repo); commitRaw(t, repo, "initial").
  - ISOLATE (G6): t.Setenv("HOME", t.TempDir()); t.Setenv("XDG_CONFIG_HOME", <same>).
  - CONFIG: cfgPath := writePiStubConfig(t, bin); t.Setenv("STAGECOACH_CONFIG", cfgPath).
  - ENV: t.Setenv("STAGECOACH_PROVIDER", "pi"); t.Setenv("STAGECOACH_STUB_OUT", singleStubOut).
  - STAGE A CHANGE: writeFile(t, repo, "new.txt", "content"); stageFile(t, repo, "new.txt").
  - DRIVE: var outBuf, errBuf bytes.Buffer; rootCmd.SetOut(&outBuf); rootCmd.SetErr(&errBuf);
           rootCmd.SetArgs([]string{"--dry-run", "--verbose", "--no-color"}); Execute(ctx).
  - ASSERT: err == nil (dry-run success, exit 0).
  - ASSERT (G1): stderr := errBuf.String(); strings.Contains(stderr, "DEBUG: command:");
           strings.Contains(stderr, "--provider openrouter"); !Contains("--provider pi").
  - ASSERT (bonus): strings.TrimSpace(outBuf.String()) == singleStubOut (dry-run msg on stdout).

Task 3: ADD subtest "decompose" (the planner role via decompose.Decompose)
  - SETUP: saveRootState(t) + defer restoreRootState(...)  (fresh rootCmd state — REQUIRED
           because subtests share the package-level rootCmd).
  - REPO: t.TempDir(); initRepo(t, repo); chdir(t, repo); commitRaw(t, repo, "initial").
  - ISOLATE (G6): t.Setenv("HOME", ...); t.Setenv("XDG_CONFIG_HOME", ...).
  - CONFIG: same writePiStubConfig(t, bin); t.Setenv("STAGECOACH_CONFIG", cfgPath).
  - ENV: t.Setenv("STAGECOACH_PROVIDER", "pi");
         t.Setenv("STAGECOACH_STUB_OUT", decomposeStubOut).
  - DIRTY TREE (G4): writeFile(t, repo, "dirty.txt", "content"); DO NOT stage it
           (un-staged → shouldDecompose routes to decompose; StatusPorcelain != "").
  - DRIVE: var outBuf, errBuf bytes.Buffer; rootCmd.SetOut(&outBuf); rootCmd.SetErr(&errBuf);
           rootCmd.SetArgs([]string{"--verbose", "--no-color"});   # NO --dry-run (G5)
           Execute(ctx).
  - ASSERT: err == nil (single-shortcut commits successfully).
  - ASSERT (G1): stderr := errBuf.String(); Contains("--provider openrouter");
           !Contains("--provider pi").
  - ASSERT (bonus): one new commit created — headSHA differs / gitOut log subject ==
           "feat: decompose path provider render".

Task 4: RUN validation gates (see Validation Loop). Confirm the full suite still passes.
```

### Implementation Patterns & Key Details

```go
// The canonical CLI-test skeleton to copy (from TestRunDefault_VerboseFlag / ...ConfigFlagHonored_Issue1):
func TestRunDefault_ProviderSubProviderRendering_Issue1(t *testing.T) {
    bin := stubtest.Build(t)

    // Shared pi-shaped config writer (Task 1). Returns the temp config file path.
    writePiStubConfig := func(t *testing.T) string {
        t.Helper()
        cfgPath := filepath.Join(t.TempDir(), "config.toml")
        body := fmt.Sprintf(`config_version = 2
[defaults]
provider = "pi"
[provider.pi]
command = %q
detect  = %q
provider_flag = "--provider"
default_provider = "openrouter"
model_flag = "--model"
default_model = "gpt-5.4-nano"
system_prompt_flag = "--system"
prompt_delivery = "stdin"
print_flag = "-p"
output = "raw"
tooled_flags = ["--yes"]
`, bin, bin)
        if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
            t.Fatalf("write cfg: %v", err)
        }
        return cfgPath
    }

    // (Task 2) single-commit path
    t.Run("single_commit", func(t *testing.T) {
        origArgs, origOut, origErr, origRunE := saveRootState(t)
        defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

        home := t.TempDir()
        t.Setenv("HOME", home)
        t.Setenv("XDG_CONFIG_HOME", home)
        cfgPath := writePiStubConfig(t)
        t.Setenv("STAGECOACH_CONFIG", cfgPath)
        t.Setenv("STAGECOACH_PROVIDER", "pi")

        repo := t.TempDir()
        initRepo(t, repo)
        chdir(t, repo)
        commitRaw(t, repo, "initial")
        writeFile(t, repo, "new.txt", "content")
        stageFile(t, repo, "new.txt")

        t.Setenv("STAGECOACH_STUB_OUT", "feat: single path provider render")

        var outBuf, errBuf bytes.Buffer
        rootCmd.SetOut(&outBuf)
        rootCmd.SetErr(&errBuf)
        rootCmd.SetArgs([]string{"--dry-run", "--verbose", "--no-color"})

        if err := Execute(context.Background()); err != nil {
            t.Fatalf("Execute err=%v, want nil", err)
        }
        stderr := errBuf.String()
        if !strings.Contains(stderr, "--provider openrouter") {
            t.Errorf("stderr = %q, want rendered command to contain --provider openrouter", stderr)
        }
        if strings.Contains(stderr, "--provider pi") {
            t.Errorf("stderr = %q, must NOT contain --provider pi (manifest name)", stderr)
        }
    })

    // (Task 3) decompose path — planner single-shortcut
    t.Run("decompose", func(t *testing.T) {
        origArgs, origOut, origErr, origRunE := saveRootState(t)
        defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

        home := t.TempDir()
        t.Setenv("HOME", home)
        t.Setenv("XDG_CONFIG_HOME", home)
        cfgPath := writePiStubConfig(t)
        t.Setenv("STAGECOACH_CONFIG", cfgPath)
        t.Setenv("STAGECOACH_PROVIDER", "pi")

        repo := t.TempDir()
        initRepo(t, repo)
        chdir(t, repo)
        commitRaw(t, repo, "initial")
        writeFile(t, repo, "dirty.txt", "content") // UN-staged → triggers decompose routing

        t.Setenv("STAGECOACH_STUB_OUT",
            `{"count":1,"single":true,"commits":[{"title":"add dirty","description":"dirty.txt"}],"message":"feat: decompose path provider render"}`)

        var outBuf, errBuf bytes.Buffer
        rootCmd.SetOut(&outBuf)
        rootCmd.SetErr(&errBuf)
        rootCmd.SetArgs([]string{"--verbose", "--no-color"}) // NO --dry-run (G5)

        if err := Execute(context.Background()); err != nil {
            t.Fatalf("Execute err=%v, want nil (single-shortcut should commit)", err)
        }
        stderr := errBuf.String()
        if !strings.Contains(stderr, "--provider openrouter") {
            t.Errorf("stderr = %q, want rendered planner command to contain --provider openrouter", stderr)
        }
        if strings.Contains(stderr, "--provider pi") {
            t.Errorf("stderr = %q, must NOT contain --provider pi (manifest name)", stderr)
        }
    })
}

// CRITICAL DETAIL: every subtest MUST call saveRootState/restoreRootState independently —
// rootCmd is a package-level singleton; SetArgs/SetOut/SetErr leak between subtests without it.
```

### Integration Points

```yaml
TEST FILE:
  - add to: internal/cmd/default_action_test.go
  - pattern: append after TestRunDefault_VerboseFlag; reuse root_test.go helpers.
  - imports already present: bytes, context, fmt, os, path/filepath, strings, testing,
    github.com/dustin/stagecoach/internal/stubtest (all already imported in this file).

NO SOURCE/CONFIG CHANGES:
  - This task edits ONLY the test file. generate.go and the four decompose role files are
    already fixed (S1/S2). Do not touch them, render.go, or any TOML manifest.
```

## Validation Loop

### Level 1: Build & Vet (Immediate Feedback)

```bash
# After writing the test — must compile + vet clean.
go build ./...
go vet ./internal/cmd/

# Expected: zero errors. If the test references an unknown helper/symbol, fix the import
# or reuse the existing root_test.go helper instead.
```

### Level 2: The New Test (Component Validation)

```bash
# Run ONLY the new test, verbose.
go test ./internal/cmd/ -run 'TestRunDefault_ProviderSubProviderRendering_Issue1' -v

# Expected: both subtests PASS. If "decompose" fails with a ResolveRoles/stager error → you
# forgot tooled_flags (G3). If a subtest fails finding "DEBUG: command:" / "--provider
# openrouter" → you asserted on stdout instead of stderr (G1), or verbose is off. If the
# stub got the wrong output → you left STAGECOACH_STUB_OUT in [provider.pi.env] (G2).

# Temporarily prove the test GUARDS the bug: revert ONE fix (e.g. generate.go L192 back to
# cfg.Provider), rerun, confirm the single_commit subtest FAILS with "--provider pi". Then
# restore the fix. (This is optional but strongly recommended to confirm the guard bites.)
```

### Level 3: Full Suite (System Validation)

```bash
# The whole module must still pass (no regressions from the new test, no shared-state leaks).
go test ./...

# Expected: all PASS, including the existing TestRunDefault_* and TestRouting_* tests.
# A leak (rootCmd state across tests) would surface as flaky/failing siblings — ensure every
# subtest brackets with saveRootState/restoreRootState.
```

### Level 4: Domain-Specific Validation

```bash
# Confirm the stub binary builds (stubtest.Build compiles cmd/stubagent in-test; this also
# runs implicitly in the test, but a direct build is a fast sanity check).
go build ./cmd/stubagent

# (Optional) Reproduce the original bug path manually to document the before/after:
#   - Build a stagecoach binary, set STAGECOACH_CONFIG to the pi config above on a repo with a
#     staged change, run `stagecoach --dry-run --verbose --no-color`, and observe the
#     `DEBUG: command:` line shows `--provider openrouter` (post-fix). Not required for CI.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` is clean.
- [ ] `go vet ./internal/cmd/` is clean.
- [ ] New test passes: `go test ./internal/cmd/ -run 'TestRunDefault_ProviderSubProviderRendering_Issue1' -v`.
- [ ] Full suite passes: `go test ./...`.

### Feature Validation

- [ ] Single-commit subtest: stderr contains `--provider openrouter`, not `--provider pi`;
      dry-run stdout equals the canned message.
- [ ] Decompose subtest: stderr contains `--provider openrouter`, not `--provider pi`; one
      commit is created.
- [ ] Both paths assert on the STDERR buffer (G1), not stdout.
- [ ] The decompose subtest config includes `tooled_flags` (G3) and omits `STAGECOACH_STUB_OUT`
      from `[provider.pi.env]` (G2).
- [ ] (Recommended) Temporarily reverting the S1/S2 fix makes at least one subtest FAIL —
      proving the guard actually catches the regression.

### Code Quality Validation

- [ ] Reuses existing root_test.go helpers (initRepo/chdir/saveRootState/writeFile/stageFile/
      commitRaw); no duplicated helper bodies.
- [ ] Both subtests bracket with saveRootState/restoreRootState (package-level rootCmd).
- [ ] Follows the file's conventions (helper naming, t.Helper(), assertion messages quoting
      the captured buffer).
- [ ] No production source, manifest, or doc files are modified.

## Anti-Patterns to Avoid

- ❌ Don't assert the rendered command on **stdout** — verbose goes to **stderr** (G1).
- ❌ Don't put `STAGECOACH_STUB_OUT` in `[provider.pi.env]` and also try to vary it with
  `t.Setenv` — manifest env wins (G2).
- ❌ Don't omit `tooled_flags` from the pi config for the decompose subtest — ResolveRoles
  errors before the planner renders (G3).
- ❌ Don't run the decompose subtest with `--dry-run` — decompose is skipped when dry-run
  is set (G5).
- ❌ Don't write a `.stagecoach.toml` inside the repo while also using `STAGECOACH_CONFIG` —
  double-merge; isolate HOME/XDG and keep the repo config-free (G6).
- ❌ Don't drive the full stager→message→arbiter loop through one stub binary; use the
  planner single-shortcut (G4).
- ❌ Don't skip `saveRootState`/`restoreRootState` in a subtest — rootCmd is shared state.
- ❌ Don't edit `generate.go`, the decompose role files, `render.go`, or any manifest — this
  is test-only.

---

## Confidence Score: 9/10

The fix is already complete (S1/S2); this is a pure test-addition task with a fully
specified skeleton, exact config, exact stub outputs, and six documented gotchas that are
the only realistic failure modes. The one residual uncertainty: the precise wording of the
`reg.IsInstalled` path check for an absolute stub path is assumed to behave as it does in
the existing `setupStubRepo` tests (which pass `command = "<bin>"` and resolve fine) — if
`detect` is somehow required to differ, the test will surface it at Level 2 and the fix is a
one-line config tweak.
