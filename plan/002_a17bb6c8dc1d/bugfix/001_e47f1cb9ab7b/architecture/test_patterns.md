# Test Patterns & Conventions

## Testing Stack

### Real Git (NO mocks)
All tests use **real git** operations against temporary repositories. There are NO mock
implementations of the `git.Git` interface. Tests create temp repos via:
```go
repo := t.TempDir()
run := func(args ...string) { exec.Command("git", append([]string{"-C", repo}, args...)...).Run() }
run("init")
// ... create files, stage, commit as needed
g := git.New(repo)
```

**Implication for Issue 3**: Adding `LogRange` to the Git interface requires implementing it ONLY in
`gitRunner`. No mocks need updating (none exist). All tests will automatically use the real
implementation.

### Stub Agent (`internal/stubtest`)
The `cmd/stubagent` binary (`cmd/stubagent/main.go`) is a fake agent that reads config from env
vars and prints a canned response. It's compiled in-test via:
```go
bin := stubtest.Build(t)  // compiles cmd/stubagent to a temp path
```

Factory functions:
```go
// Single canned response:
m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add login"})

// Multi-response script (sequential responses):
m := stubtest.NewScript(t, bin, []string{"feat: first", "feat: second"})

// Timeout simulation:
m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: x", SleepMS: 400})
```

The stub reads `STAGEHAND_STUB_OUT` from its env to determine the response. The stubtest.Manifest
factory sets this up in the manifest's `Env` map.

### Integration Tests (CLI-level)
`internal/cmd/*_test.go` tests drive the real cobra CLI via `rootCmd.SetArgs(...)` +
`rootCmd.Execute()`. They set up `HOME`/`XDG_CONFIG_HOME` via `t.Setenv` to control config paths.

### Test File Conventions
- Co-located: `foo_test.go` next to `foo.go`.
- Table-driven where applicable.
- `t.Setenv` for env var isolation (Go 1.17+ auto-cleanup).
- `t.TempDir()` for temp directories (auto-cleanup).
- Tests assert on `generate.Result` / `decompose.CommitResult` fields, NOT on rendered command
  strings (this is WHY Issue 1 escaped detection — the gap is at the caller level, not the Render
  level).

## Key Test Files by Issue

| Issue | Primary Test Files | What to Add |
|-------|--------------------|-------------|
| 1 | `generate_test.go`, `decompose/{planner,stager,message,arbiter}_test.go`, `default_action_test.go` | Assert rendered CmdSpec has correct `--provider` token |
| 2 | `provider/builtin_test.go`, `decompose/decompose_test.go` | Assert tightened TooledFlags; assert HEAD-guard violation |
| 3 | `git/git_test.go` (or new `logrange_test.go`), `decompose/decompose_test.go` | Assert post-arbiter SHAs are accurate/resolvable |
| 4 | `config/file_test.go`, `cmd/config_test.go` | Assert `--config`/`STAGEHAND_CONFIG` honored |
| 5 | `config/bootstrap_test.go` | Assert pi bootstrap has empty models |

## Build & Test Commands
```bash
go build ./...          # must compile
go test ./...           # must pass
go test -v ./internal/generate/...   # specific package
go test -run TestName ./internal/... # specific test
```
