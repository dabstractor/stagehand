# System Context — Bugfix Delta (v2.1 Validation Fixes)

## Project Identity
- **Module**: `github.com/dustin/stagecoach`
- **Language**: Go 1.22 (toolchain go1.26.4)
- **Binary**: `cmd/stagecoach` (CLI tool for AI-assisted git commit messages)
- **Dependencies**: DELIBERATELY MINIMAL — `cobra`, `pflag`, `go-toml/v2`, `yaml.v3`. **NO** `golang.org/x/term`, `golang.org/x/sys`, or `mattn/go-isatty`. The project is intentionally dep-free for TTY detection ("project stays dep-free; see procgroup_windows.go").
- **Supported platforms**: linux, darwin, windows (all × amd64/arm64) per `.goreleaser.yaml`.

## Architecture Overview (relevant to the four fixes)

### Command Layer (`internal/cmd/`)
- Cobra-based CLI. Commands registered via `init()`.
- Each command file pairs with a `_test.go` file using table-driven + functional tests.
- `rootCmd` is the global cobra root; subcommands attach via `rootCmd.AddCommand()`.

### Integrate Feature (`internal/cmd/integrate*.go` + `internal/integrate/`)
- **Two targets**: `git-alias` and `lazygit`, each implementing `integrate.Entry` interface.
- **`integrate.Entry`** interface: `Name()`, `Detect()`, `ConfigPath()`, `Status()`, `Install()`, `Remove()`.
- **`integrate.Target`** interface: `Marker()`, `Parse()`, `HasEntry()`, `Upsert()`, `Remove()`, `Validate()`.
- **`integrate.Apply()`** (protocol.go): owns the no-mangle write envelope — parse-first refusal, unified-diff preview, confirm, backup, atomic write, validate+restore. File-editing targets (lazygit) go through this.
- **`gitAliasEntry`** does NOT use `Apply` — it manages its own preview+confirm via `ConfirmFunc` and delegates writes to `git config --global`.
- **`lazygitEntry.Install()`** delegates entirely to `integrate.Apply(ActionUpsert)`.
- **`DefaultConfirm`** (protocol.go): the FR-I3c y/N prompt; auto-declines on non-TTY stdin via `ui.IsTerminal(os.Stdin)`.

### Hook Exec Feature (`internal/cmd/hookexec.go` + `internal/hook/`)
- `runHookExec` is the RunE for `hook exec <msg-file> [<source> [<sha>]]`.
- `hook.Run()` (internal/hook/exec.go) is the source-gated, never-block runtime.
- `hook.ErrNoOp` sentinel: returned for source-gated no-ops (message/template/merge/squash/commit) or empty staged diffs.
- `hook.NoOpSource(source)` is an exported pure function checking the source gate.
- `hook.Run()` takes `generate.Deps` (Git, Manifest, Verbose, Excludes).

### Config Template (`internal/cmd/config.go`)
- `exampleConfigTemplate` is a `const` string (~lines 497-640) written by `config init --template`.
- EVERY line is commented out (`#`); the file is INERT.
- Its `[generation]` section lists 8 keys but is MISSING the 5 v2.1 keys: `exclude`, `format`, `locale`, `template`, `push`.

### UI Layer (`internal/ui/output.go`)
- `IsTerminal(f *os.File) bool`: tests `stat.Mode() & os.ModeCharDevice != 0`.
- **BUG**: `/dev/null` is a char device, so `IsTerminal(/dev/null)` returns true.
- Callers: `default_action.go` (color), `hookexec.go` (color), `config_init_interactive.go` (TTY gate via `interactiveStdinIsTTY` var), `integrate/protocol.go` (DefaultConfirm auto-decline).
- The code comment explicitly notes: "NOT a true isatty ioctl" and mentions `golang.org/x/term` as a future swap but defers it as "out of v1 scope."

### Platform-Specific Code Pattern
The project uses build-tag files for platform-specific code:
- `internal/provider/procgroup_unix.go` (`//go:build !windows`)
- `internal/provider/procgroup_windows.go` (`//go:build windows`)
- `internal/signal/signal_unix.go` (`//go:build !windows`)
- `internal/signal/signal_windows.go` (`//go:build windows`)
These use **stdlib-only** syscall (no `golang.org/x/sys` dependency).

## Key Design Constraints
1. **Dep-free**: No new dependencies. Platform-specific code uses stdlib `syscall`.
2. **TDD**: Every code change pairs with tests. Tests use `*bytes.Buffer` for output capture, temp dirs, stub binaries.
3. **No-mangle protocol**: The integrate feature's core guarantee — it must be impossible for stagecoach to mangle a config file.
4. **Idempotency**: Install operations must be idempotent (marker-based identity).
