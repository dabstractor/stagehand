# Design Decisions — P1.M3.T4.S1 Stub Provider (fake agent)

> The single most important read for this subtask. Each decision is load-bearing; cite it in the PRP.

## §0 — Scope: this is TEST INFRASTRUCTURE, not a shipped feature

PRD §20.1 layer 3 defines the stub as the enabler for the integration-test layer: *"A fake agent:
a tiny Go binary (or shell script) that reads stdin and writes a canned message to stdout. Drives
`generate.CommitStaged` end-to-end."* This subtask builds the stub itself + a reusable helper so that:

- **P1.M3.T4.S2** (`CommitStaged` orchestrator integration tests) — drives the full pipeline
  (snapshot → generate → parse → dedupe → commit) and asserts the resulting commit exists with the
  right tree/parent/message. S2 CONSUMES this stub.
- **P1.M5.T1.S1** (property/invariant tests) — idempotent-index / atomic-HEAD / snapshot-immutability.
  Also CONSUMES this stub.

OUT OF SCOPE (do NOT implement here): `CommitStaged` itself (S2), the property tests' bodies (M5.T1),
the real-agent `//go:build integration_real` suite (M5.T1.S2). This subtask delivers the stub + helper
ONLY, plus the stub's OWN self-tests.

## §1 — THE form decision: Go binary (`cmd/stubagent`), NOT a shell script

PRD §20.1 allows "a tiny Go binary (or shell script)". **Choose the Go binary.** Three reasons:

1. **Cross-platform CI (PRD §20.4).** The CI matrix is `{linux, macos, windows} × {amd64, arm64}`.
   A `#!/bin/sh` stub does not run on Windows (no `sh`). A Go binary compiles and runs on all three
   OSes. A shell script would force a `t.Skip` on Windows CI, defeating the purpose.
2. **No shell-quoting / injection surface.** The executor already passes args as `[]string` (no `sh -c`,
   PRD §19 / critical_findings.md FINDING 8). A Go binary reads its behavior from **env vars** (§2),
   which are also `[]string` ("KEY=VAL") — zero shell surface.
3. **Reproducible + cheap.** `go build -o <tmp> ./cmd/stubagent` is deterministic; the binary is a few
   KB of stdlib-only Go. No interpreter dependency.

Location: **`cmd/stubagent/main.go`** (a `main` package; Go requires `main` packages under `cmd/`).
It imports **stdlib only** (`bufio`, `fmt`, `os`, `strconv`, `strings`, `time`). NO `internal/*`,
NO third-party. `go mod tidy` MUST be a no-op (matches the stdlib-only-leaf convention of `generate`).

## §2 — THE config mechanism: behavior via ENVIRONMENT VARIABLES (not flags, not stdin)

The stub is configurable per-invocation to simulate success / failure / timeout / duplicate outputs.
**Configuration comes from env vars**, set via the manifest's `Env` map. This is the elegant fit because
the plumbing ALREADY exists end-to-end:

```
Manifest.Env map[string]string
   └─► Render(): env := os.Environ(); for k,v := range r.Env { env = append(env, k+"="+v) }   (render.go)
        └─► CmdSpec.Env []string
             └─► Execute(): if len(spec.Env) > 0 { cmd.Env = spec.Env }                       (executor.go)
                  └─► stub process reads os.Getenv(...)
```

Verified against `internal/provider/render.go` (Env construction) + `internal/provider/executor.go`
(`cmd.Env = spec.Env` when non-empty). **A test-only Manifest with `Env: {STAGECOACH_STUB_OUT: "..."}`
configures the stub with zero new plumbing.** This is exactly how real providers carry secrets/config
(e.g. the pi manifest sets env entries), so the stub exercises the real env-propagation path for free.

Why NOT flags: the manifest rendering owns the argv (Subcommand/flags/payload). Adding flags would
collide with the rendering algorithm (§12.2) and the payload routing. Env vars are orthogonal and
already merged last-wins by the executor. Why NOT stdin: stdin IS the prompt payload (the agent's
input); the stub must read it as a real agent does, not parse it for behavior config.

**Env var contract (the stub's public API to tests):**

| Env var | Default | Meaning |
|---|---|---|
| `STAGECOACH_STUB_OUT` | `""` | Stdout text to emit (single-response mode). Multi-line OK. |
| `STAGECOACH_STUB_EXIT` | `"0"` | Process exit code. Non-zero ⇒ simulates a failed agent (§18.2 retry/rescue). |
| `STAGECOACH_STUB_SLEEP_MS` | `"0"` | Sleep duration BEFORE emitting output (timeout simulation). |
| `STAGECOACH_STUB_STDERR` | `""` | Stderr text to emit (for stderr-capture / verbose-mode tests). |
| `STAGECOACH_STUB_SCRIPT` | `""` | Path to a call-script file (enables call-varying mode; see §3). |
| `STAGECOACH_STUB_COUNTER` | `""` | Path to a call-counter file (used with SCRIPT; see §3). |

Prefix `STAGECOACH_STUB_` avoids collisions with real `STAGECOACH_*` config env vars (config §16.1 layer 5)
and signals "test only". All values are strings (env vars are strings); the stub parses numerics via
`strconv.Atoi` with a safe default on parse error (a malformed value never panics — robustness).

## §3 — Call-varying mode: file-backed script + counter (for the dedupe loop)

PRD §20.1 layer 3 requires the stub to support **"duplicate-retry-then-success"** (and
"parse-failure-then-rescue"): the orchestrator's retry loop calls the agent AGAIN after a duplicate
subject / parse failure, and the SECOND call must return a DIFFERENT output. Env vars don't persist
across separate processes (each `Execute` is a fresh stub process), so cross-call state needs a FILE.

**Script mode** (`STAGECOACH_STUB_SCRIPT=<path>` set):

- The script file's contents are split on `\n` into an ordered list of responses. **Blank lines are
  SIGNIFICANT** (an empty response ⇒ `ParseOutput` returns `ok=false` ⇒ orchestrator retries — this is
  exactly the "parse-failure-then-rescue" trigger). Do NOT skip blanks.
- The call index is read from `STAGECOACH_STUB_COUNTER=<path>`: read the integer (0 if absent/empty),
  USE it to select `responses[index]`, then increment and write back. Out-of-range index ⇒ the LAST
  response (a test that wants a stable tail sets one trailing response).
- Selection logic: `if index >= len(responses) { index = len(responses)-1 }`; emit `responses[index]`
  as stdout. Exit code / sleep / stderr still come from `STAGECOACH_STUB_EXIT` / `_SLEEP_MS` / `_STDERR`
  (uniform across scripted calls — the §20.1 scenarios only need call-varying OUTPUT, not exit code).

**Why this covers every §20.1 layer-3 scenario:**

| Scenario | Stub config | Result |
|---|---|---|
| success | `OUT="feat: add x"`, EXIT=0 | one call, canned message |
| duplicate-retry-then-success | SCRIPT lines `[dup, fresh]`, EXIT=0 | call 1→dup (rejected), call 2→fresh (accepted) |
| parse-failure-then-rescue | SCRIPT lines `["", good]`, EXIT=0 | call 1→empty (ok=false), call 2→good; OR only `[""]` → exhaust→rescue |
| timeout | `SLEEP_MS=10000`, small orchestrator timeout | `Execute` hits DeadlineExceeded |
| non-zero-exit retry | `EXIT=1` | orchestrator retries, then rescues (§18.2) |

**Atomicity:** the orchestrator calls the agent SERIALLY (one in-flight agent per `CommitStaged`; the
dedupe loop is sequential). So the counter read-increment-write needs NO file lock. Each TEST gets its
own script/counter files in its own `t.TempDir()`, so concurrent tests don't collide. Document the
serial assumption; do NOT add `flock` (Unix-only, §10 critical_findings — would break Windows).

## §4 — THE ordering gotcha: drain stdin FIRST, then sleep, then output (avoid deadlock)

The executor pipes the payload via `cmd.Stdin = strings.NewReader(spec.Stdin)` (executor.go). The OS
stdin pipe has a bounded buffer (~64 KiB on Linux). **If the stub sleeps BEFORE draining stdin and the
payload exceeds the pipe buffer, the parent blocks writing stdin while the child sleeps ⇒ deadlock**
(neither progresses; the test hangs until timeout). The correct order, ALWAYS:

1. **Drain stdin fully** (`io.Copy(io.Discard, os.Stdin)`) — consume the prompt payload, as a real
   agent does. Returns immediately on empty/nil stdin (the executor gives /dev/null when `Stdin=="`).
2. **Sleep** `STAGECOACH_STUB_SLEEP_MS` (if > 0) — AFTER stdin is drained, so the parent isn't blocked.
3. **Write stderr** (`STAGECOACH_STUB_STDERR`) then **stdout** (`STAGECOACH_STUB_OUT` or scripted line).
4. **Exit** with `STAGECOACH_STUB_EXIT`.

A self-test asserts a large-payload + sleep combination completes (not hangs) — this pins the ordering.
Note: stdout/stderr are captured to `bytes.Buffer` in the parent (not OS pipes), so there is NO deadlock
risk on the OUTPUT side — only on stdin. Document this asymmetry.

## §5 — The reusable helper: `internal/stubtest` (cross-package, importable)

The stub must be reusable by S2 (`internal/generate` integration tests) and M5.T1 (property tests).
A Go `_test.go` file is NOT importable by other packages' tests (test files are compiled only into
their own package's test binary). Therefore the helper MUST live in a **regular `.go` file** in a real,
importable package: **`internal/stubtest/stubtest.go`** (`package stubtest`), plus its own tests
`internal/stubtest/stubtest_test.go`.

Precedent: stdlib `net/http/httptest`, `testing/iotest`, `testing/fstest` — all regular importable
packages that exist solely to support tests, and whose functions take `*testing.T`/`testing.TB`.

Why NOT `internal/generate/stub_provider_test.go` (the work item's first suggested name): that name is a
`_test.go` file IN `package generate` — not importable by M5.T1's tests (which may live in `internal/`
or a dedicated package), and it conflates "the stub" with "the CommitStaged integration tests" (which
is S2's deliverable, not S1's). The work item's parenthetical "(or a small cmd/stubagent/main.go
compiled in tests)" explicitly sanctions the binary+helper form. **S1 owns the stub + helper; S2 owns
the generate-package integration tests that USE it.**

Helper API (the FROZEN surface S2/M5 import):

```go
package stubtest

// Options configures a stub agent invocation (translates to STAGECOACH_STUB_* env vars).
type Options struct {
    Out     string  // STAGECOACH_STUB_OUT   (single-response; "" if Script set)
    Exit    int     // STAGECOACH_STUB_EXIT  (default 0)
    SleepMS int     // STAGECOACH_STUB_SLEEP_MS (default 0)
    Stderr  string  // STAGECOACH_STUB_STDERR (default "")
    Script  string  // STAGECOACH_STUB_SCRIPT path (call-varying mode; "" disables)
    Counter string  // STAGECOACH_STUB_COUNTER path (used with Script; "" disables)
    // Manifest output knobs (defaults match the §12.1 built-ins):
    Output         string  // "" → "raw"
    StripCodeFence *bool   // nil → true (so a test can emit fenced output to exercise the parser)
}

// Build compiles ./cmd/stubagent ONCE per process (sync.Once-cached) to a temp path and returns it.
// Skips t if `go` is not resolvable on PATH. The path is reused across tests in the same binary.
func Build(t testing.TB) string

// Manifest returns a test-only provider.Manifest whose Command is bin and whose Env encodes opts.
// PromptDelivery="stdin", Output/StripCodeFence per opts. Ready to pass to Manifest.Render(...).
func Manifest(bin string, o Options) provider.Manifest

// Env returns the []string "K=V" env slice for opts (os.Environ() + STAGECOACH_STUB_*).
// Exposed so a test can construct a CmdSpec directly (bypassing Render) if it prefers.
func Env(o Options) []string

// NewScript is the high-level "duplicate-retry-then-success" helper: it writes responses (one per
// line, in order) to a file in t.TempDir(), creates a counter file path, and returns a Manifest
// wired for call-varying mode. responses[0] is the first call's output, responses[1] the second, etc.
// Blank entries are SIGNIFICANT (empty output ⇒ parse ok=false).
func NewScript(t testing.TB, bin string, responses []string) provider.Manifest
```

`Build` uses `exec.Command("go", "build", "-o", path, "github.com/dustin/stagecoach/cmd/stubagent")`
— import-path form resolves from ANY cwd (no cmd.Dir needed). Cached via `sync.Once` + a path in
`os.MkdirTemp`, so N tests in one `go test` run compile the stub exactly ONCE.

`stubtest` imports `internal/provider` (for `provider.Manifest`) — a clean one-way edge (stubtest →
provider), no cycle. The stub binary itself imports NO internal packages.

## §6 — The build helper: why `go build` at test time is fine (and the skip contract)

- During `go test`, the `go` toolchain is ALWAYS present (it's what's running). `exec.LookPath("go")`
  succeeds. In exotic sandboxes where it might not, the helper `t.Skip`s with a clear message — the
  same pattern `internal/provider/executor_test.go` uses (`mustBin(t, "cat")` skips when `cat` absent).
- Building `./cmd/stubagent` is the main module's OWN package ⇒ NO network, NO module download.
  Deterministic + fast (~tens of ms).
- Under `-race` / `-coverprofile`: the stub binary is a SEPARATE `go build` artifact, NOT the
  race/coverage-instrumented test binary. It runs uninstrumented — which is correct (it's a fixture,
  not code under coverage). The PRD §20.3 85% gate covers `internal/{git,provider,generate,config}`
  — `cmd/stubagent` and `internal/stubtest` are NOT gated packages (they're test infra).
- No interaction with goreleaser / `dist/`: the binary goes to `os.MkdirTemp`, not `./bin` or `./dist`.

## §7 — Self-tests: drive the stub through the REAL Execute seam

The stub's OWN tests (`cmd/stubagent/main_test.go` + `internal/stubtest/stubtest_test.go`) build the
binary via `stubtest.Build(t)` and invoke it through `provider.Execute(ctx, spec, timeout)` — the
EXACT seam the orchestrator (S2) and real providers use. This is the strongest validation: if the stub
behaves through `Execute`, it behaves in `CommitStaged`. Mirror `internal/provider/executor_test.go`'s
style (construct `CmdSpec`, call `Execute`, assert stdout/stderr/err). Cases:

1. **echo success**: `OUT="feat: x"` → stdout contains "feat: x", err nil.
2. **multi-line OUT**: `OUT="subject\n\nbody"` → stdout has both lines (round-trips).
3. **exit non-zero**: `EXIT=1` → `Execute` returns a non-nil wrapped `*exec.ExitError`, stdout empty.
4. **timeout**: `SLEEP_MS=2000`, `Execute(..., 200*time.Millisecond)` → err IS `context.DeadlineExceeded`,
   returns within seconds (the process-group kill fired — proves the stub is killable, FINDING 8).
5. **stderr capture**: `STDERR="boom"` → stderr == "boom" (separate from stdout).
6. **stdin drain ordering (no deadlock)**: `OUT="ok"`, large Stdin (1 MiB) + `SLEEP_MS=500` → completes
   without hanging (pins §4).
7. **script call-varying**: `NewScript(t, bin, []string{"dup", "fresh"})` → call 1 stdout=="dup",
   call 2 stdout=="fresh", call 3 stdout=="fresh" (clamps to last).
8. **script blank = parse failure**: `NewScript(t, bin, []string{"", "good"})` → call 1 stdout=="".
9. **robustness**: a malformed `EXIT="not-a-number"` does NOT panic (defaults to 0).

## §8 — Package-doc, imports, and frozen files

- `cmd/stubagent/main.go`: `package main`, stdlib only (`bufio`/`fmt`/`os`/`strconv`/`strings`/`time`).
  Has a `// Command stubagent is ...` doc comment (the `// Package`-vs-`// Command` convention for
  main packages — Go doc shows "Command stubagent"). NO `// Package` comment (it's `main`).
- `internal/stubtest/stubtest.go`: `package stubtest`, imports `os`, `os/exec`, `strconv`, `sync`,
  `testing`, `github.com/dustin/stagecoach/internal/provider`. Has a `// Package stubtest ...` doc.
- `internal/stubtest/stubtest_test.go`: `package stubtest` (white-box), imports `context`, `errors`,
  `os`, `strings`, `testing`, `time`, `github.com/dustin/stagecoach/internal/provider` + the stubtest pkg.
- `cmd/stubagent/main_test.go`: `package main` (white-box), imports the same stdlib set + provider +
  stubtest (to reuse `Build`). Actually simpler: put ALL stub self-tests in `internal/stubtest/stubtest_test.go`
  and have `cmd/stubagent/main_test.go` be minimal or omitted — see the PRP's final placement.

FROZEN (do NOT edit): everything in `internal/{git,provider,prompt,config,generate}`, `cmd/stagecoach`,
`pkg/*`, `Makefile`, `go.mod`, `go.sum`. Only NEW files are created. `go mod tidy` is a no-op (no new
deps — the stub is stdlib-only; stubtest imports only the already-present `internal/provider`).

## §9 — Downstream contracts (S2 / M5 consume the stub via `stubtest`)

```go
// In S2 (internal/generate commitstaged_test.go):
bin := stubtest.Build(t)                                   // build once
m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add x"})
spec, _ := m.Render("", "", "", "fake user payload")       // full render path
out, _, err := provider.Execute(ctx, *spec, 5*time.Second)  // real executor
msg, ok, _ := provider.ParseOutput(out, m)                  // real parser
// ... then assert the commit exists via git.DiffTree etc.

// In S2 (duplicate-retry-then-success):
repo := initRepoWithHistory(t, "feat: dup")                // recent subject "feat: dup"
bin := stubtest.Build(t)
m := stubtest.NewScript(t, bin, []string{"feat: dup", "feat: fresh"})  // call1=dup, call2=fresh
// CommitStaged(repo, m, ...) → rejects "feat: dup", retries, accepts "feat: fresh"
```

The `stubtest.Options` / `Manifest` / `Build` / `NewScript` signatures are FROZEN after this subtask.
S2 and M5 depend on them; do not rename without coordinating.

## §10 — Validation summary (project-specific, verified working)

- `go build ./...` — builds the stub binary's package + everything (no breakage).
- `go test -race ./cmd/stubagent/ ./internal/stubtest/` — the stub + helper tests green.
- `go test -race ./...` — nothing else regresses (only NEW files added).
- `go vet ./cmd/stubagent/ ./internal/stubtest/` clean.
- `gofmt -l cmd/stubagent/ internal/stubtest/` empty.
- `git diff --exit-code go.mod go.sum` empty (no new deps).
- Manual: `go run ./cmd/stubagent` with `STAGECOACH_STUB_OUT="hi"` env → prints "hi", exit 0.
