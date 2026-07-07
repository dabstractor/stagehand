# Research Findings — P1.M1.T2.S1 (E2E provider-rendering integration test)

## The fix being guarded (already complete: P1.M1.T1.S1 + S2)
All five Render call sites now pass `""` for the sub-provider param so Render falls
back to the manifest's merged `DefaultProvider`:
- `internal/generate/generate.go` ~L192 → `deps.Manifest.Render(cfg.Model, "", sysPrompt, payload)`
- `internal/decompose/{planner,stager,message,arbiter}.go` → `Render(mdl, "", ...)`

This test is the regression guard proving it end-to-end through the real CLI.

## CRITICAL gotchas discovered (must be reflected in the test design)

### G1 — Verbose "DEBUG: command:" goes to STDERR, not stdout
`internal/cmd/default_action.go` wires `Verbose: stderr` (= `cmd.ErrOrStderr()`) into
`ui.NewVerbose(stderr, cfg.Verbose)`. `provider.Execute` calls `vb.VerboseCommand(...)`
which writes `"DEBUG: command: <argv>\n"` to that **stderr** writer
(`internal/ui/verbose.go`, `internal/provider/executor.go`).
The work-item contract step (e) says "captures stdout" — that is WRONG for verbose.
Existing `TestRunDefault_VerboseFlag` proves it: it asserts `"DEBUG: command:"` on the
**errBuf** (stderr buffer). The new test MUST assert on the stderr buffer (`rootCmd.SetErr`).

### G2 — Manifest env OVERRIDES t.Setenv (last-wins in exec.Cmd.Env)
`internal/provider/render.go` builds `spec.Env = os.Environ() + manifest.Env entries`
(manifest appended LAST → exec last-wins → manifest OVERRIDES parent env). So if
`[provider.pi.env] STAGECOACH_STUB_OUT = "feat: test"` is in the config, a `t.Setenv(
"STAGECOACH_STUB_OUT", ...)` CANNOT change it. Because the two subtests need DIFFERENT
stub outputs (single path = a commit message; decompose path = planner JSON), the test
must set `STAGECOACH_STUB_OUT` via `t.Setenv` and OMIT it from `[provider.pi.env]` —
exactly like the existing `setupStubRepo` helper does. (Putting a fixed value in
`[provider.pi.env]` is fine only for a single-output test.)

### G3 — The decompose path REQUIRES `tooled_flags` on the provider (else ResolveRoles fails BEFORE the planner runs)
`runDecompose` calls `decompose.ResolveRoles(*cfg, reg)` BEFORE `decompose.Decompose`.
`ResolveRoles` (`internal/decompose/roles.go`) resolves ALL FOUR roles, including the
stager, and applies the FR-D4 fallback: if the stager provider's manifest has empty
`TooledFlags`, it looks for another installed stager-capable provider; if none exists it
returns an error (`role "stager": provider "pi" cannot stage ...`). That error fires
BEFORE the planner is ever Rendered → the planner's `DEBUG: command:` line is never
emitted → the assertion would falsely fail. THEREFORE `[provider.pi]` MUST include a
non-empty `tooled_flags` (any value, e.g. `["--yes"]`) so pi is its own stager and
ResolveRoles succeeds. (This field is NOT in the contract's literal config — it must be
added.)

### G4 — Use the planner single-SHORTCUT for the decompose subtest (one agent round-trip)
Driving the full stager→message→arbiter loop through one shared stub binary is impractical
(the stubagent cannot run git; all roles share one binary/script). Instead, make the
planner return `single:true` + a `message`:
`{"count":1,"single":true,"commits":[{"title":"x","description":"dirty.txt"}],"message":"feat: ..."}`
`decompose.Decompose` then takes `runSingleShortcut` (`internal/decompose/decompose.go`):
AddAll → WriteTree → dup-check the planner message → publish. The message/stager/arbiter
roles are NOT invoked (verified by `TestDecompose_SingleShortcut_CleanMessage`). So only
the planner stub runs once, decompose SUCCEEDS (one commit created), and the planner's
rendered command (with `--provider openrouter`) is in stderr. Clean + robust.

### G5 — `shouldDecompose` returns false for --dry-run
`internal/cmd/default_action.go shouldDecompose` → decompose is skipped when `dryRun` is
true. So the decompose subtest must run WITHOUT `--dry-run` (it commits for real). The
single-commit subtest CAN use `--dry-run` (and should — it mirrors the PRD reproduction
and avoids mutating HEAD).

### G6 — Isolate HOME / XDG_CONFIG_HOME (avoid the test machine's global config leaking)
`config.Load` with `STAGECOACH_CONFIG` set uses that path as the global file, but the
registry still merges built-in defaults + any global config. Set HOME/XDG to a temp dir
(mirror `TestRunDefault_ConfigFlagHonored_Issue1`). Also ensure the repo has NO
`.stagecoach.toml` so repo-local discovery doesn't double-merge — the provider comes ONLY
from the `STAGECOACH_CONFIG` file.

## Assertion precision
The progress label prints `"Generating with pi…"` / `"Decomposing with pi…"` to stderr
(contains bare `pi`, but NOT `--provider pi`). So assert on the exact token
`"--provider openrouter"` (present) and `"--provider pi"` (absent) — both with the flag
prefix. The model `gpt-5.4-nano` and the stub temp path contain no `--provider pi`.

## Reusable helpers already in the cmd test package (internal/cmd/root_test.go)
`initRepo(t, dir)`, `writeConfigFile(t, dir, relPath, body)`, `chdir(t, dir)`,
`saveRootState(t)` / `restoreRootState(...)`, `writeFile`, `stageFile`, `commitRaw`,
`headSHA`, `gitOut`, `runGit`. And `stubtest.Build(t)` compiles the stub binary once.

## Build/test commands (verified)
`go build ./...` ✓ (clean). `go test ./internal/cmd/... -run <Name> -v`.
