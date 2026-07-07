---
name: "P1.M3.T4.S1 — Stub provider (fake agent) for integration testing — PRD §20.1 layer 3"
description: |

  Build the reusable fake-agent test infrastructure that enables the integration-test layer of the
  whole generation pipeline. Per PRD §20.1 layer 3: *"A fake agent: a tiny Go binary (or shell script)
  that reads stdin and writes a canned message to stdout. Drives `generate.CommitStaged` end-to-end."*
  This subtask delivers the stub ITSELF (not the CommitStaged integration tests — those are P1.M3.T4.S2).

  Three deliverables, all NEW files, no edits to existing code:
    1. **CREATE `cmd/stubagent/main.go`** (`package main`, **STDLIB ONLY**) — the fake-agent binary. It
       reads stdin (drains the prompt payload, as a real agent does), then behaves per **environment
       variables** (the manifest `Env` seam, already wired end-to-end by Render+Execute): emit a canned
       message to stdout (`STAGECOACH_STUB_OUT`), exit non-zero (`STAGECOACH_STUB_EXIT`), sleep to simulate
       timeout (`STAGECOACH_STUB_SLEEP_MS`), write stderr (`STAGECOACH_STUB_STDERR`), and — for the dedupe
       loop — return a DIFFERENT output per successive call via a file-backed script + counter
       (`STAGECOACH_STUB_SCRIPT` / `STAGECOACH_STUB_COUNTER`). The stub IS the mock: no external services.
    2. **CREATE `internal/stubtest/stubtest.go`** (`package stubtest`, imports `internal/provider` + stdlib)
       — the reusable, importable helper. Exports `Build(t) string` (compiles `./cmd/stubagent` ONCE per
       process, cached), `Options`, `Manifest(bin, opts)` (a test-only provider.Manifest pointing Command
       at the stub with Env knobs set), `Env(opts)` (the `K=V` slice for a raw CmdSpec), and
       `NewScript(t, bin, responses)` (the high-level "duplicate-retry-then-success" constructor). This
       is the FROZEN surface that P1.M3.T4.S2 (CommitStaged integration tests) and P1.M5.T1 (property
       tests) import.
    3. **CREATE `internal/stubtest/stubtest_test.go`** (`package stubtest`, white-box) — drives the
       compiled binary through the REAL `provider.Execute` seam (the exact path the orchestrator uses)
       and asserts every behavior: echo success, multi-line, non-zero exit, timeout (DeadlineExceeded +
       process-group kill), stderr capture, stdin-drain-no-deadlock ordering, script call-varying,
       blank=parse-failure, malformed-env robustness. Mirror `internal/provider/executor_test.go`.

  SCOPE BOUNDARY (load-bearing): this subtask is TEST INFRASTRUCTURE only. It does NOT implement
  `generate.CommitStaged` (S2), the property-test bodies (M5.T1), or the `//go:build integration_real`
  suite (M5.T1.S2). It does NOT touch the provider pipeline (Manifest/Render/Execute/ParseOutput are
  DONE in P1.M2 — read-only refs). It adds NO dependency (stub binary is stdlib-only; stubtest imports
  only the already-present `internal/provider`). See research design-decisions.md §0/§8.

  INPUT (upstream — already built, read-only): the provider pipeline from P1.M2. `provider.CmdSpec`
  (render.go, P1.M2.T4) is what the stub is invoked as; `provider.Execute(ctx, spec, timeout)` (executor.go,
  P1.M2.T5) is the seam; `provider.Manifest` (manifest.go, P1.M2.T1) with its `Env map[string]string` is
  how a test-only manifest configures the stub; `provider.ParseOutput` (parse.go, P1.M2.T6) consumes the
  stub's stdout (empty ⇒ `ok=false` ⇒ orchestrator retries — the "parse-failure" lever).

  OUTPUT (downstream consumers): P1.M3.T4.S2 imports `internal/stubtest` to drive `CommitStaged`
  end-to-end across all §20.1 layer-3 scenarios (success, duplicate-retry-then-success, parse-failure-
  then-rescue, timeout, CAS-failure, root-commit, auto-stage-all). P1.M5.T1.S1 imports it for
  property/invariant tests. The `stubtest.{Build,Options,Manifest,Env,NewScript}` API is FROZEN after
  this subtask.

  ⚠️ **Go binary, NOT a shell script.** PRD §20.4's CI matrix is `{linux, macos, windows} × {amd64,arm64}`.
  A `#!/bin/sh` stub does not run on Windows. A Go binary runs everywhere and adds zero shell surface.
  (design-decisions §1)

  ⚠️ **Behavior via ENV VARS, not flags/stdin.** The plumbing already exists end-to-end:
  `Manifest.Env → Render (os.Environ()+env) → CmdSpec.Env → Execute (cmd.Env=spec.Env) → stub os.Getenv`.
  A test-only Manifest's `Env` map configures the stub with NO new plumbing. stdin IS the prompt payload
  (the agent's input) — the stub drains it, never parses it for config. (design-decisions §2)

  ⚠️ **Drain stdin BEFORE sleeping/output (deadlock guard).** The executor pipes the payload via a bounded
  OS pipe. If the stub sleeps before draining stdin and the payload exceeds the pipe buffer, parent+child
  deadlock. Order is ALWAYS: drain stdin → sleep → write stderr → write stdout → exit. A self-test pins
  this with a 1 MiB payload + sleep. (design-decisions §4)

  ⚠️ **Call-varying output via file-backed script+counter (for the dedupe loop).** Env doesn't persist
  across separate stub processes, so cross-call state needs a FILE. `STAGECOACH_STUB_SCRIPT` = a file whose
  `\n`-split lines are ordered responses (BLANK lines are significant — empty output ⇒ parse ok=false ⇒
  retry); `STAGECOACH_STUB_COUNTER` = a file holding the call index (read→use→increment; out-of-range ⇒
  last response). No file lock — the orchestrator calls the agent SERIALLY (one in-flight per CommitStaged).
  (design-decisions §3)

  ⚠️ **Helper must be IMPORTABLE → regular .go in a real package, NOT a `_test.go` file.** `_test.go`
  files compile only into their own package's test binary and cannot be imported by other packages' tests
  (S2 lives in `internal/generate`, M5.T1 may live elsewhere). Hence `internal/stubtest/stubtest.go` (a
  normal package, like stdlib `net/http/httptest`). The work item's `internal/generate/stub_provider_test.go`
  name is superseded by this — S2 owns the generate-package integration tests; S1 owns the stub + helper.
  (design-decisions §5)

  ⚠️ **Pointer-scalar manifest gotcha: `provider.strPtr`/`boolPtr` are UNEXPORTED.** The stubtest helper
  (package `stubtest`, ≠ `provider`) CANNOT call them. Construct pointer fields with local `&`-helpers
  (e.g. `s := "stdin"; m.PromptDelivery = &s`). `Env` is a plain `map[string]string` — assign directly.
  (seam-facts.md / manifest.go)

  Deliverable: CREATE `cmd/stubagent/main.go` + `internal/stubtest/stubtest.go` +
  `internal/stubtest/stubtest_test.go`. STDLIB-ONLY stub binary; helper imports only `internal/provider`.
  `go mod tidy` MUST be a no-op. Touches ONLY these three NEW files — NO go.mod/go.sum change, NO edit to
  any `internal/*`, `cmd/stagecoach`, `pkg/*`, or the Makefile.

---

## Goal

**Feature Goal**: Implement the reusable fake-agent binary + helper that PRD §20.1 layer 3 calls for — the
test infrastructure that lets every integration/property test in the rest of v1 drive the generation
pipeline WITHOUT a real AI agent. The binary is a tiny, stdlib-only Go program that behaves exactly like
a provider on the `provider.Execute` seam: it reads the prompt from stdin and writes a commit message to
stdout, with its behavior (canned output, exit code, simulated timeout, stderr, and per-call output
variation for the dedupe loop) controlled entirely by environment variables that ride the existing
manifest-Env→CmdSpec.Env→cmd.Env plumbing. The helper compiles the binary once per test process and hands
back a ready-to-use test-only `provider.Manifest`. Together they make `CommitStaged` (S2) and the
property tests (M5.T1) writable as plain table-driven tests with zero external dependencies.

**Deliverable** (three NEW files, nothing else touched):
1. **`cmd/stubagent/main.go`** — `package main`, imports `bufio`/`fmt`/`io`/`os`/`strconv`/`strings`/`time`
   ONLY. Reads behavior from the `STAGECOACH_STUB_*` env vars; drains stdin; (optionally) sleeps; writes
   stderr; writes stdout (single-response OR scripted); exits with the configured code. Has a
   `// Command stubagent is the test fake-agent …` doc comment.
2. **`internal/stubtest/stubtest.go`** — `package stubtest`, imports `os`/`os/exec`/`sync`/`testing` +
   `github.com/dustin/stagecoach/internal/provider`. Exports `Build(t testing.TB) string`,
   `Options` struct, `Manifest(bin string, o Options) provider.Manifest`, `Env(o Options) []string`,
   `NewScript(t testing.TB, bin string, responses []string) provider.Manifest`. Has a
   `// Package stubtest …` doc comment.
3. **`internal/stubtest/stubtest_test.go`** — `package stubtest` (white-box), imports `context`/`errors`/
   `os`/`strings`/`testing`/`time` + `internal/provider`. Drives the built binary through `provider.Execute`
   and asserts all behaviors (echo, multi-line, non-zero exit, timeout, stderr, stdin-drain ordering,
   script call-varying, blank=parse-failure, malformed-env robustness). Mirror `executor_test.go`.

**Success Definition**: `go build ./...` succeeds (the stub package now compiles); `go test -race
./cmd/stubagent/ ./internal/stubtest/` is green; `go test -race ./...` shows NO regression elsewhere;
`go vet ./cmd/stubagent/ ./internal/stubtest/` clean; `gofmt -l cmd/stubagent/ internal/stubtest/` empty;
`golangci-lint run` (if available) clean; go.mod/go.sum byte-unchanged; the stub behaves correctly through
the REAL `provider.Execute` seam for every §20.1 layer-3 capability (success/exit-nonzero/timeout/stderr/
call-varying/blank-output); a manual `STAGECOACH_STUB_OUT="hi" go run ./cmd/stubagent` prints `hi` and exits 0.

## User Persona

**Target User**: The integration/property-test author (P1.M3.T4.S2 and P1.M5.T1 implementers). They need a
deterministic, dependency-free stand-in for `pi`/`claude`/etc. so they can drive `generate.CommitStaged`
end-to-end and assert on the resulting git commit. Transitively: the `provider.Execute` seam is the
contract — the stub must be indistinguishable from a real provider at that boundary. (End-user personas
are PRD §7 "plan-holder"/"refusenik"/"tinkerer"; they never see the stub — it's test-only.)

**Use Case**: Write a test like: build the stub (`bin := stubtest.Build(t)`), make a test-only manifest
(`m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add x"})`), render+execute it through the real
pipeline, parse the output, and (in S2) assert `CommitStaged` produced a commit with subject "feat: add x".
For the dedupe case, `stubtest.NewScript(t, bin, []string{"feat: dup","feat: fresh"})` makes call 1 return
a duplicate subject and call 2 a fresh one. No API keys, no network, no real agent install — runs in CI on
all three OSes.

**User Journey**: (internal test infra, no end-user surface) test calls `stubtest.Build` (compiles stub
once) → `stubtest.Manifest`/`NewScript` (test-only manifest with Env knobs) → `m.Render(...)` (real
renderer) → `provider.Execute(ctx, *spec, timeout)` (real executor, real process-group kill) →
`provider.ParseOutput(out, m)` (real parser) → assert. The stub is invoked as a fresh subprocess each call,
reading its behavior from env; the script+counter files give it memory across calls.

**Pain Points Addressed**: (1) Integration tests can't depend on a real LLM (cost, latency, nondeterminism,
no API key in CI) — solved by a deterministic fake. (2) Windows CI (§20.4) — solved by a Go binary (a
shell script would force `t.Skip` on Windows). (3) Testing the dedupe/rescue failure paths (duplicate
subject, parse failure, timeout, non-zero exit) — solved by env-driven failure simulation + call-varying
script. (4) Boilerplate to build/locate a helper binary per test — solved by the once-cached `Build`.

## Why

- **Unblocks the integration-test layer (PRD §20.1 layer 3) for the entire rest of v1.** S2 (CommitStaged)
  and M5.T1 (property tests) cannot be written until a stub exists. This is the keystone test dependency.
- **Faithful to commit-pi's proven model.** commit-pi's test suite used a fake agent that reads stdin and
  writes a canned message (PRD Appendix C porting map). The Go port makes it a cross-platform binary.
- **Exercises the REAL provider pipeline.** Because the stub is driven through `Manifest.Render` →
  `provider.Execute` → `provider.ParseOutput`, every integration test also regression-tests the renderer
  (env propagation!), the executor (process-group kill, timeout, stdin piping), and the parser — for free.
- **No new dependency, no new user-facing surface** (PRD "DOCS: none — test infrastructure"). stdlib-only
  binary; helper imports only the already-present `internal/provider`.

## What

A new `main` package (`cmd/stubagent`) implementing the fake agent, plus a new importable helper package
(`internal/stubtest`) and its tests. The binary reads `STAGECOACH_STUB_*` env vars, drains stdin, optionally
sleeps, writes stderr/stdout, and exits with a configured code; in script mode it varies output per call
via a file-backed counter. The helper builds the binary once and constructs test-only manifests. No new
types in `internal/provider`, no changes to the pipeline, no config, no git, no signals.

### Success Criteria

- [ ] `cmd/stubagent/main.go` exists, `package main`, imports EXACTLY stdlib (`bufio`/`fmt`/`io`/`os`/
      `strconv`/`strings`/`time`) — NO `internal/*`, NO third-party. Has a `// Command stubagent …` doc.
      `func main()` drains stdin (`io.Copy(io.Discard, os.Stdin)`), then sleeps `STAGECOACH_STUB_SLEEP_MS`
      (parsed via `strconv.Atoi`, default 0, never panics on malformed), then writes `STAGECOACH_STUB_STDERR`
      to `os.Stderr`, then writes the selected output to `os.Stdout` (single-response = `STAGECOACH_STUB_OUT`;
      script-mode = the counter-indexed line of `STAGECOACH_STUB_SCRIPT`), then `os.Exit(code)` where code =
      `STAGECOACH_STUB_EXIT` (parsed, default 0, never panics).
- [ ] Script mode: when `STAGECOACH_STUB_SCRIPT` is set, the stub reads `STAGECOACH_STUB_COUNTER` (integer,
      0 if absent/unparseable), selects `lines[index]` (out-of-range ⇒ last line; BLANK lines are kept
      verbatim — empty output ⇒ `ParseOutput` ok=false), increments+writes back the counter, emits that
      line. When `STAGECOACH_STUB_SCRIPT` is unset, emit `STAGECOACH_STUB_OUT` (single-response, every call).
- [ ] `internal/stubtest/stubtest.go` exists, `package stubtest`, imports `os`/`os/exec`/`sync`/`testing` +
      `internal/provider` ONLY. Exports `Build(t testing.TB) string` (compiles `./cmd/stubagent` to a
      temp path ONCE via `sync.Once`, `t.Skip`s if `go` not on PATH), `Options` (Out/Exit/SleepMS/Stderr/
      Script/Counter/Output/StripCodeFence), `Manifest(bin, o)` (test-only `provider.Manifest`: Command=bin,
      PromptDelivery="stdin", Output/StripCodeFence per o, Env=the STAGECOACH_STUB_* knobs), `Env(o)` (the
      `K=V` slice incl. `os.Environ()`), `NewScript(t, bin, responses)` (writes responses one-per-line to a
      file in `t.TempDir()`, wires Script+Counter, returns a Manifest). Has a `// Package stubtest …` doc.
- [ ] `internal/stubtest/stubtest_test.go` exists, `package stubtest` (white-box), drives the built binary
      through `provider.Execute` and asserts: echo success; multi-line OUT round-trips; `EXIT=1` ⇒ non-nil
      wrapped `*exec.ExitError`; `SLEEP_MS=2000`+200ms timeout ⇒ `context.DeadlineExceeded` within seconds;
      `STDERR="boom"` ⇒ stderr=="boom" (separate from stdout); 1 MiB stdin + `SLEEP_MS=500` completes (no
      deadlock — pins drain-before-sleep); `NewScript([dup,fresh])` ⇒ call1=dup, call2=fresh, call3=fresh
      (clamp); `NewScript(["",good])` ⇒ call1="" (parse failure); malformed `EXIT="x"` ⇒ exit 0 (no panic).
- [ ] `go build ./...` succeeds; `go test -race ./...` green (new tests pass, nothing regresses);
      `go vet ./cmd/stubagent/ ./internal/stubtest/` clean; `gofmt -l cmd/stubagent/ internal/stubtest/` empty.
- [ ] go.mod/go.sum byte-unchanged (`git diff --exit-code go.mod go.sum` empty).
- [ ] Every other file byte-unchanged (all of `internal/{git,provider,prompt,config,generate}`,
      `cmd/stagecoach`, `pkg/*`, `Makefile`).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the seam facts
(`Execute`/`Render`/`CmdSpec`/`Manifest`/`ParseOutput` signatures — research seam-facts.md), the design
decisions (the binary-vs-script call §1, env-driven config §2, script+counter §3, stdin-drain ordering §4,
helper-package location §5, build helper §6, self-test plan §7), the upstream pipeline contracts (P1.M2 is
DONE — read-only), the executor-test convention to mirror (`internal/provider/executor_test.go`), and the
copy-ready Go code in the Implementation Blueprint. No git/CLI/config/signal knowledge required — the stub
is a standalone stdlib binary + a thin helper.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M3T4S1/research/design-decisions.md
  why: the SINGLE most important read — the 10 decisions specific to this subtask: scope = TEST INFRA
       (§0), Go-binary-not-shell-script for Windows CI (§1), env-var config via the existing Manifest.Env
       seam (§2), file-backed script+counter for call-varying output / the dedupe loop (§3), drain-stdin-
       BEFORE-sleep deadlock guard (§4), the helper must be an IMPORTABLE package not a _test.go file (§5),
       why go-build-at-test-time is fine + the skip contract (§6), the self-test plan driving the stub
       through the real Execute seam (§7), package-doc/import/frozen-file rules (§8), the FROZEN helper
       API S2/M5 consume (§9), validation summary (§10).
  critical: §1 (Go binary — Windows), §2 (env vars — the existing seam), §3 (script+counter for dedupe),
       §4 (stdin drain order — deadlock), §5 (importable package) are the things most likely to be done
       wrong. §9's API is FROZEN for S2/M5.

- docfile: plan/001_f1f80943ac34/P1M3T4S1/research/seam-facts.md
  why: the EXACT verified signatures the stub/helper must match — CmdSpec fields, Execute's error contract
       (timeout→DeadlineExceeded, exit→*exec.ExitError), Render's Env construction (os.Environ()+manifest
       Env last-wins), Manifest's pointer-scalar design (strPtr/boolPtr UNEXPORTED → helper uses local
       &-helpers; Env is a plain map), ParseOutput's ok=false-on-empty (the parse-failure lever). Line
       numbers included so the implementer doesn't re-derive them.
  critical: the Env seam (Manifest.Env → Render → CmdSpec.Env → Execute cmd.Env) is THE integration point
       — the whole stub design hinges on it already existing. ParseOutput ok=false on empty output is how
       the stub triggers "parse-failure-then-rescue".

- file: internal/provider/executor.go   (P1.M2.T5 — READ for the Execute seam + error contract; do NOT edit)
  section: `func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration) (stdout, stderr string, err error)`
  why: THIS is the seam the stub is invoked through. The stub's self-tests call Execute exactly as the
       orchestrator will. Note: stdin is `strings.NewReader(spec.Stdin)` (bounded OS pipe ⇒ drain-before-
       sleep matters); stdout/stderr captured to SEPARATE buffers (no output-side deadlock); cmd.Env =
       spec.Env when non-empty (the env-var seam); setupProcessGroup ⇒ child is killable via ctx cancel.
  pattern: construct a `provider.CmdSpec{Command: bin, Env: stubtest.Env(opts)}` (or render a Manifest),
       call `provider.Execute(ctx, spec, timeout)`, assert stdout/stderr/err.
  gotcha: Execute returns `context.DeadlineExceeded` (not a wrapped error) on timeout and a wrapped
          `*exec.ExitError` on non-zero exit — use `errors.Is(err, context.DeadlineExceeded)` and
          `errors.As(err, &exitErr)`.

- file: internal/provider/executor_test.go   (P1.M2.T5 — READ for the TEST PATTERN to mirror; do NOT edit)
  section: `TestExecute_TimeoutKillsProcess` + `TestExecute_StderrCaptureAndNonZeroExit` + `mustBin`.
  why: the TEST CONVENTION — construct a CmdSpec, call Execute, assert. `mustBin(t, "sleep")` skips when
       a binary is absent; the stub tests use `stubtest.Build(t)` analogously (skip if `go` absent). The
       timeout test's "returns within seconds" assertion (proving the process-group kill fired) is mirrored
       for the stub's timeout self-test.
  pattern: `spec := CmdSpec{Command: <bin>, Args: ..., Stdin: ..., Env: ...}`; `out, errb, err := Execute(...)`;
           `errors.Is`/`errors.As` for err; `time.Since(start)` bounds for timeout-kill responsiveness.

- file: internal/provider/render.go   (P1.M2.T4 — READ for the Env-construction seam; do NOT edit)
  section: the Env block in `Render`: `env := os.Environ(); for k,v := range r.Env { env = append(env, k+"="+v) }; spec.Env = env`.
  why: PROVES the stub's env-var design needs no new plumbing. A test-only Manifest's `Env` map is copied
       verbatim into `CmdSpec.Env` and applied by Execute as `cmd.Env`. The stub reads `os.Getenv`. End-to-
       end verified. This is why behavior-via-env-vars is the elegant choice (design-decisions §2).
  gotcha: manifest Env is appended AFTER os.Environ() ⇒ last-wins ⇒ overrides any real STAGECOACH_* var.
          Prefix stub knobs with STAGECOACH_STUB_ to avoid colliding with real config env (config §16.1).

- file: internal/provider/manifest.go   (P1.M2.T1 — READ for the Manifest pointer-scalar design; do NOT edit)
  section: the `Manifest` struct + `strPtr`/`boolPtr` (UNEXPORTED) + `Env map[string]string` (plain map).
  why: the stubtest helper (package stubtest) constructs a provider.Manifest but CANNOT call provider's
       unexported strPtr/boolPtr. It must build pointer fields with local &-helpers (e.g. `s := "stdin";
       m.PromptDelivery = &s`). Env is a plain map → assign directly `m.Env = map[string]string{...}`.
  gotcha: do NOT try to call provider.strPtr from package stubtest (won't compile). Do NOT leave
          PromptDelivery nil — Render's Resolve defaults it to "stdin" anyway, but setting it explicitly
          documents intent and makes the test manifest self-describing.

- file: internal/provider/parse.go   (P1.M2.T6 — READ for the ok=false lever; do NOT edit)
  section: `func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)` — Step 5:
           `msg = strings.TrimSpace(msg); ok = msg != ""`.
  why: an EMPTY stub output ⇒ ok=false ⇒ the orchestrator retries (FR29) then rescues. This is how the
       stub simulates "parse-failure-then-rescue" (single-response OUT="" OR a blank script line). The
       stub doesn't need to emit garbage — empty is the cleanest parse-failure trigger.

- url: (PRD §20.1 layer 3 + §20.4 CI matrix — already in context as selected_prd_content `h3.68`/`h2.20`;
       ALSO plan/001_f1f80943ac34/prd_snapshot.md — §20.1 the stub definition, §20.4 the {linux,macos,
       windows}×{amd64,arm64} matrix that mandates a Go binary over a shell script)
  why: §20.1 layer 3 is the AUTHORITATIVE stub spec (reads stdin, writes canned stdout, drives CommitStaged
       end-to-end, covers success/duplicate-retry/parse-failure/timeout/CAS/root/auto-stage-all). §20.4 is
       WHY the stub is a Go binary (Windows). §20.3 (≥85% on internal/{git,provider,generate,config})
       confirms cmd/stubagent + internal/stubtest are NOT gated (test infra).
  critical: the §20.1 layer-3 scenario list IS the stub's capability checklist — every scenario must be
            achievable with the stub's env knobs (success=OUT; dup-retry=NewScript; parse-failure=blank;
            timeout=SLEEP_MS+small orchestrator timeout; CAS/root/auto-stage=S2 repo setup, stub just
            succeeds normally).
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; go-toml/v2 + pflag  (UNCHANGED — stub adds NO dep)
go.sum                          # unchanged
cmd/
  stagecoach/main.go             # stub (P1.M1.T1) — UNCHANGED
  stubagent/
    main.go                     # NEW (this subtask) ← the fake-agent binary (stdlib only)
internal/
  config/                       # P1.M1.T4 — untouched
  generate/                     # P1.M3 — untouched (rescue.go/dedupe.go exist from T3.S1/T2.S1; CommitStaged is S2)
  git/                          # P1.M1.T2/T3 — untouched (read-only ref; S2 uses it, this subtask doesn't)
  prompt/                       # P1.M3.T1 — untouched
  provider/                     # P1.M2 (T1–T6) — untouched (Execute/Render/Manifest/ParseOutput read-only refs)
  stubtest/
    stubtest.go                 # NEW (this subtask) ← reusable helper (Build/Options/Manifest/Env/NewScript)
    stubtest_test.go            # NEW (this subtask) ← drives the stub through provider.Execute
  ui/                           # P1.M4 (empty stub) — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — UNCHANGED
```

### Desired Codebase tree with files to be added

```bash
cmd/stubagent/main.go              # NEW — fake-agent binary: drain stdin → sleep → stderr → stdout → exit(code),
                                   #        behavior from STAGECOACH_STUB_* env (OUT/EXIT/SLEEP_MS/STDERR/SCRIPT/COUNTER).
                                   #        Script mode = file-backed counter for call-varying output (dedupe loop).
internal/stubtest/stubtest.go      # NEW — Build(t) compiles ./cmd/stubagent once; Options; Manifest(bin,o);
                                   #        Env(o); NewScript(t,bin,responses). FROZEN API for S2 + M5.T1.
internal/stubtest/stubtest_test.go # NEW — drives the built binary through provider.Execute; asserts every behavior.
# All other files UNCHANGED. go.mod/go.sum UNCHANGED. After this subtask: S2 imports internal/stubtest to drive
# CommitStaged end-to-end; M5.T1 imports it for property tests.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (SCOPE — TEST INFRA ONLY): this subtask delivers the stub + helper. It does NOT implement
// generate.CommitStaged (S2), property-test bodies (M5.T1), or //go:build integration_real (M5.T1.S2).
// It does NOT touch the provider pipeline (P1.M2 DONE — read-only). Pure additive: 3 NEW files.
// (design-decisions §0)

// CRITICAL (Go binary, NOT shell script): PRD §20.4 CI = {linux,macos,windows}. A #!/bin/sh stub does
// not run on Windows. cmd/stubagent/main.go is a real Go program → compiles+runs on all 3 OSes, zero
// shell surface. (§1)

// CRITICAL (behavior via ENV VARS): the seam already exists — Manifest.Env → Render(os.Environ()+env) →
// CmdSpec.Env → Execute(cmd.Env=spec.Env) → stub os.Getenv. A test-only Manifest's Env map configures the
// stub with NO new plumbing. stdin is the PROMPT (drain it, don't parse it for config). Prefix knobs
// STAGECOACH_STUB_ to dodge real STAGECOACH_* config env. (§2)

// CRITICAL (DRAIN STDIN BEFORE SLEEP/OUTPUT — deadlock guard): the executor pipes the payload via a
// bounded OS pipe (~64 KiB). If the stub sleeps before draining and the payload exceeds the buffer,
// parent blocks writing stdin while child sleeps ⇒ deadlock (test hangs till timeout). Order is ALWAYS:
// io.Copy(io.Discard, os.Stdin) → sleep → write stderr → write stdout → exit. A self-test pins this with
// a 1 MiB payload + sleep. (§4) NOTE: stdout/stderr are captured to bytes.Buffer in the parent (NOT OS
// pipes) → NO deadlock risk on the output side, only on stdin.

// CRITICAL (call-varying output via FILE — for the dedupe loop): env doesn't persist across separate
// stub processes (each Execute is a fresh process), so cross-call state needs a FILE. STAGECOACH_STUB_SCRIPT
// = a file whose \n-split lines are ordered responses (BLANK lines SIGNIFICANT — empty ⇒ ParseOutput
// ok=false ⇒ orchestrator retries = parse-failure-then-rescue). STAGECOACH_STUB_COUNTER = integer index
// (read→use→increment; out-of-range ⇒ last line). No file lock — orchestrator calls the agent SERIALLY
// (one in-flight per CommitStaged); each test owns its own script/counter files in t.TempDir(). (§3)

// CRITICAL (helper must be IMPORTABLE): a _test.go file compiles only into its OWN package's test binary
// and CANNOT be imported by other packages' tests (S2 is internal/generate; M5.T1 may be elsewhere).
// Therefore internal/stubtest/stubtest.go is a REGULAR .go file in a real package (like net/http/httptest),
// NOT a _test.go file. (§5)

// GOTCHA (pointer-scalar manifest): provider.strPtr/boolPtr are UNEXPORTED (manifest.go). The stubtest
// helper (package stubtest ≠ provider) CANNOT call them. Build pointer fields with local &-helpers:
//   s := "stdin"; m.PromptDelivery = &s
// Env is a plain map[string]string → assign directly: m.Env = map[string]string{...}. (seam-facts.md)

// GOTCHA (numeric env parsing never panics): strconv.Atoi("garbage") returns an error — the stub MUST
// ignore it and fall back to the default (EXIT→0, SLEEP_MS→0, COUNTER→0). A malformed env value must
// never crash the stub (robustness; a test might set a bogus value). (§2)

// GOTCHA (build once, cache): N tests in one `go test` run would rebuild the stub N times if Build
// weren't cached. Use sync.Once + a package-level path (os.MkdirTemp) so the compile happens EXACTLY
// once per test process. Build via import path `github.com/dustin/stagecoach/cmd/stubagent` (resolves
// from any cwd — no cmd.Dir needed). Skip (t.Skip) if exec.LookPath("go") fails. (§6)

// GOTCHA (// Command, not // Package, for main): cmd/stubagent/main.go is `package main`. Its doc
// comment is `// Command stubagent is …` (Go doc renders main packages as "Command"). Do NOT write
// `// Package main` or `// Package stubagent`. internal/stubtest/stubtest.go IS a real package → its
// doc is `// Package stubtest …`. (§8)

// GOTCHA (do NOT pre-empt S2/M5): this subtask owns ONLY the stub + helper + the stub's self-tests.
// CommitStaged, the property-test bodies, and the integration_real suite are S2 / M5.T1 / M5.T1.S2.
// Do not implement them here. (§0/§9)
```

## Implementation Blueprint

### Data models and structure

```go
// cmd/stubagent/main.go — NO exported types (it's `package main`). Behavior is a flat sequence reading
// env vars; the only "model" is the (scriptLines, counterIndex) pair in script mode. Keep main() small:
// parse env → drain stdin → sleep → write stderr → select+write stdout → exit.

// internal/stubtest/stubtest.go — the ONE exported data model:
type Options struct {
	Out           string  // STAGECOACH_STUB_OUT (single-response). Used when Script=="".
	Exit          int     // STAGECOACH_STUB_EXIT  (default 0; non-zero simulates a failed agent)
	SleepMS       int     // STAGECOACH_STUB_SLEEP_MS (default 0; >0 simulates a slow/timing-out agent)
	Stderr        string  // STAGECOACH_STUB_STDERR (default ""; written to the child's stderr)
	Script        string  // STAGECOACH_STUB_SCRIPT path (call-varying mode; "" disables)
	Counter       string  // STAGECOACH_STUB_COUNTER path (used with Script; "" → Script ignores the counter)
	Output         string // manifest Output; "" → "raw"
	StripCodeFence *bool  // manifest StripCodeFence; nil → true (so a test can emit fences to exercise the parser)
}
// The helper translates Options → provider.Manifest{Command:bin, PromptDelivery:"stdin", Output:…,
// StripCodeFence:…, Env: map[string]string{...}}. No other state.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE cmd/stubagent/main.go — the fake-agent binary (STDLIB ONLY)
  - FILE: NEW cmd/stubagent/main.go. PACKAGE: `package main`. DOC: `// Command stubagent is a tiny
      fake-agent binary for Stagecoach's integration/property tests (PRD §20.1 layer 3). It reads the
      prompt from stdin and writes a canned commit message to stdout, with behavior (output, exit code,
      simulated timeout, stderr, and per-call output variation for the dedupe loop) controlled entirely by
      STAGECOACH_STUB_* environment variables — set via a test-only provider.Manifest's Env map (the existing
      Manifest.Env→CmdSpec.Env→cmd.Env seam). It is invoked through provider.Execute exactly like a real
      agent. STDLIB ONLY; no internal/*, no third-party.`
  - IMPORT: EXACTLY `bufio`, `fmt`, `io`, `os`, `strconv`, `strings`, `time`. NO internal/*, NO third-party.
  - IMPLEMENT numeric-env helpers that NEVER panic:
      func envInt(key string, def int) int { v:=os.Getenv(key); n,err:=strconv.Atoi(v); if err!=nil||n<0 {return def}; return n }
      (n<0 → def: a negative sleep/exit/counter is nonsensical; clamp, don't panic.)
  - IMPLEMENT main():
      1. DRAIN STDIN FIRST (deadlock guard, §4): `io.Copy(io.Discard, os.Stdin)` — ignore the returned
         error (EOF/nil both fine; the executor gives /dev/null when Stdin==""). This consumes the prompt
         payload so a large payload + later sleep can't deadlock the bounded OS pipe.
      2. SLEEP (AFTER drain): `if ms := envInt("STAGECOACH_STUB_SLEEP_MS", 0); ms > 0 { time.Sleep(time.Duration(ms)*time.Millisecond) }`.
      3. WRITE STDERR: `if s := os.Getenv("STAGECOACH_STUB_STDERR"); s != "" { fmt.Fprint(os.Stderr, s) }`.
      4. SELECT + WRITE STDOUT:
         - scriptFile := os.Getenv("STAGECOACH_STUB_SCRIPT")
         - if scriptFile == "" : out := os.Getenv("STAGECOACH_STUB_OUT")  // single-response mode
         - else: out = selectScripted(scriptFile)   // call-varying mode (see helper below)
         - `fmt.Fprint(os.Stdout, out)`  // Fprint (NOT Println) — emit EXACTLY `out`, no extra newline
           (the test's STAGECOACH_STUB_OUT is the verbatim desired stdout; ParseOutput trims anyway, but
           emitting exactly what was configured makes assertions byte-exact).
      5. EXIT: `os.Exit(envInt("STAGECOACH_STUB_EXIT", 0))`.
  - IMPLEMENT selectScripted(scriptFile string) string (script mode, §3):
      - data, err := os.ReadFile(scriptFile); if err != nil { return "" }  (missing/unreadable ⇒ empty output)
      - lines := strings.Split(string(data), "\n")   // BLANK lines kept verbatim (significant: empty ⇒ parse ok=false)
      - if len(lines) == 0 { return "" }
      - index := 0; counterFile := os.Getenv("STAGECOACH_STUB_COUNTER")
        if counterFile != "" { index = readCounter(counterFile) ; writeCounter(counterFile, index+1) }
      - if index < 0 || index >= len(lines) { index = len(lines)-1 }   // clamp to last (stable tail)
      - return lines[index]
  - IMPLEMENT readCounter(path) int / writeCounter(path, int):
      - readCounter: data,err:=os.ReadFile(path); n,err:=strconv.Atoi(strings.TrimSpace(string(data)));
        if err!=nil {return 0}; if n<0 {return 0}; return n   (absent/unparseable/negative ⇒ 0)
      - writeCounter: os.WriteFile(path, []byte(strconv.Itoa(n)), 0o644)  (ignore error — best-effort;
        a missing counter file just means the next call sees 0; serial callers make this safe, §3)
  - GOTCHA: do NOT print a trailing newline beyond `out`. do NOT log to stdout. do NOT use `log` (would
      need the import + pollute stdout). Keep main() linear (no goroutines — the stub is single-threaded).

Task 2: CREATE internal/stubtest/stubtest.go — the reusable helper
  - FILE: NEW internal/stubtest/stubtest.go. PACKAGE: `package stubtest`. DOC: `// Package stubtest
      provides a reusable fake-agent (cmd/stubagent) and helpers for Stagecoach's integration and property
      tests (PRD §20.1 layer 3). Build compiles the stub once per test process; Manifest/NewScript return
      test-only provider.Manifests whose Env knobs drive the stub's behavior through the real
      provider.Execute seam. Used by generate.CommitStaged integration tests (P1.M3.T4.S2) and the
      property/invariant tests (P1.M5.T1).`
  - IMPORT: `os`, `os/exec`, `strconv`, `sync`, `testing`, `github.com/dustin/stagecoach/internal/provider`.
  - DEFINE `type Options struct { … }` (see Data models).
  - IMPLEMENT local pointer helpers (provider.strPtr/boolPtr are UNEXPORTED):
      func strPtr(s string) *string { return &s }
      func boolPtr(b bool) *bool    { return &b }
      (These are package-local; they shadow provider's unexported ones — that's fine, different package.)
  - IMPLEMENT `func Build(t testing.TB) string` using a **flag-based Once** (NOT a Skip inside Do — see
    GOTCHA below):
      - package-level: `var ( stubOnce sync.Once; stubPath string; stubNoGo bool; stubBuildErr string )`.
      - `stubOnce.Do(func(){ ... })` body:
          goPath, err := exec.LookPath("go")
          if err != nil { stubNoGo = true; return }   // set a FLAG; do NOT t.Skip inside Do (GOTCHA)
          dir, err := os.MkdirTemp("", "stagecoach-stubagent-*"); if err != nil { panic/ Fatal? — no, can't
            call t.Fatal inside Do either; capture via stubPath + a stubBuildErr string }
          name := "stubagent"; if runtime.GOOS == "windows" { name = "stubagent.exe" }
          stubPath = filepath.Join(dir, name)
          build := exec.Command(goPath, "build", "-o", stubPath, "github.com/dustin/stagecoach/cmd/stubagent")
          if out, err := build.CombinedOutput(); err != nil { stubBuildErr = fmt.Sprintf("%v\n%s", err, out); return }
      - AFTER `stubOnce.Do(...)`: `if stubNoGo { t.Skipf("go toolchain not on PATH; cannot build stubagent") }`;
        `if stubBuildErr != "" { t.Fatalf("go build stubagent: %s", stubBuildErr) }`; `return stubPath`.
      - (testing.TB covers *testing.T and *testing.B.)
      - GOTCHA (flag-based Once — IMPORTANT): do NOT call t.Skip/t.Fatal INSIDE sync.Once.Do. `sync.Once.Do`
        marks itself done even when f calls runtime.Goexit (t.Skip→Goexit) or panics; a later caller would
        then see the once as done with stubPath still "" and fail confusingly instead of skipping. Set a
        FLAG (stubNoGo / stubBuildErr) inside Do, then Skip/Fatal AFTER Do returns — every caller skips
        cleanly. (Add a `stubBuildErr string` package var + import `fmt` for it.)
      - GOTCHA: use `filepath.Join` → needs `path/filepath`; `runtime.GOOS` → needs `runtime`; the buildErr
        message → needs `fmt`. Add them to the import list. The build is via IMPORT PATH (no cmd.Dir) so
        it resolves from any cwd.
  - IMPLEMENT `func Env(o Options) []string`:
      - env := os.Environ()
      - add non-default knobs only (keeps env small + makes single-response-vs-script obvious):
          if o.Script != "" { env = appendStr(env, "STAGECOACH_STUB_SCRIPT", o.Script)
                              if o.Counter != "" { env = appendStr(env, "STAGECOACH_STUB_COUNTER", o.Counter) } }
          else              { env = appendStr(env, "STAGECOACH_STUB_OUT", o.Out) }   // single-response
      - always-set knobs (even defaults, so a test can rely on them): EXIT, SLEEP_MS, STDERR:
          env = appendStr(env, "STAGECOACH_STUB_EXIT", strconv.Itoa(o.Exit))   // default "0"
          if o.SleepMS > 0 { env = appendStr(env, "STAGECOACH_STUB_SLEEP_MS", strconv.Itoa(o.SleepMS)) }
          if o.Stderr != "" { env = appendStr(env, "STAGECOACH_STUB_STDERR", o.Stderr) }
      - helper: `func appendStr(env []string, k, v string) []string { return append(env, k+"="+v) }`.
      - return env.
  - IMPLEMENT `func Manifest(bin string, o Options) provider.Manifest`:
      - m := provider.Manifest{ Name: "stub", Command: strPtr(bin), PromptDelivery: strPtr("stdin") }
      - out := o.Output; if out == "" { out = "raw" }; m.Output = strPtr(out)
      - if o.StripCodeFence != nil { m.StripCodeFence = boolPtr(*o.StripCodeFence) } else { m.StripCodeFence = boolPtr(true) }
      - m.Env = map[string]string{}; for _, kv := range Env(o) { split on first "=" → m.Env[k]=v }
        (OR build the map directly from o — cleaner: set map keys STAGECOACH_STUB_* mirroring Env's logic.
        Pick ONE source of truth — recommend a private optsToEnvMap(o) map[string]string that both Env()
        and Manifest() use, so the slice and map never drift. Env() = os.Environ()+map→slice.)
      - return m. (A Validate() call isn't needed — Name+Command are set; the test renders it.)
  - IMPLEMENT `func NewScript(t testing.TB, bin string, responses []string) provider.Manifest`:
      - dir := t.TempDir()
      - scriptPath := filepath.Join(dir, "script.txt"); os.WriteFile(scriptPath, []byte(strings.Join(responses, "\n")), 0o644)
        (Join on "\n" → each response is its own line; a "" response becomes a blank line = parse failure.)
      - counterPath := filepath.Join(dir, "counter")   // absent ⇒ stub reads 0
      - return Manifest(bin, Options{Script: scriptPath, Counter: counterPath, Exit: 0})
      - t.TempDir() is auto-cleaned → no manual cleanup. t is testing.TB → works for B too.
  - GOTCHA: do NOT import internal/provider types beyond Manifest (keep the edge minimal). do NOT add a
    second `// Package stubtest` doc (only stubtest.go carries it; stubtest_test.go has none).

Task 3: CREATE internal/stubtest/stubtest_test.go — drive the stub through provider.Execute
  - FILE: NEW internal/stubtest/stubtest_test.go. PACKAGE: `package stubtest` (white-box — can call
      unexported optsToEnvMap if you made one; else black-box is fine too). IMPORT: `context`, `errors`,
      `os/exec`, `strings`, `testing`, `time`, `github.com/dustin/stagecoach/internal/provider`.
  - SHARED setup: `bin := Build(t)` at the top of each test (Build is cached → cheap). Each test builds a
      CmdSpec via `provider.CmdSpec{Command: bin, Env: Env(o)}` (bypasses Render — fastest) OR via
      `m := Manifest(bin, o); spec,_ := m.Render("","","","PAYLOAD"); Execute(ctx, *spec, timeout)` (full
      render path — exercises env propagation too; PREFER this for at least the echo + script tests so the
      manifest→render→execute→env chain is covered). Add a Stdin to exercise draining.
  - IMPLEMENT tests (mirror executor_test.go's style — construct spec, Execute, assert):
      1. TestStub_EchoSuccess: Options{Out:"feat: add x"} → render+execute → stdout=="feat: add x", err nil.
      2. TestStub_MultilineOut: Options{Out:"subject\n\nbody line"} → stdout round-trips both lines.
      3. TestStub_NonZeroExit: Options{Exit:1} → Execute err != nil; errors.As(&*exec.ExitError) true; stdout=="".
      4. TestStub_TimeoutKilled: Options{SleepMS:2000}, Execute(ctx, spec, 200*time.Millisecond) →
         errors.Is(err, context.DeadlineExceeded); assert elapsed < 5s (process-group kill fired — mirrors
         executor_test.go TestExecute_TimeoutKillsProcess).
      5. TestStub_StderrCapture: Options{Stderr:"boom"} → stderr=="boom"; stdout=="".
      6. TestStub_DrainStdinNoDeadlock: Options{Out:"ok", SleepMS:500}; CmdSpec.Stdin = strings.Repeat("x", 1<<20)
         (1 MiB); Execute with 10s timeout → completes, err nil, stdout=="ok" (pins drain-before-sleep, §4).
      7. TestStub_ScriptCallVarying: NewScript(t, bin, []string{"feat: dup","feat: fresh"}) → render+execute
         3×: call1 stdout=="feat: dup", call2=="feat: fresh", call3=="feat: fresh" (clamp to last).
      8. TestStub_ScriptBlankIsParseFailure: NewScript(t, bin, []string{"", "feat: good"}) → call1
         stdout=="" (→ ParseOutput ok=false); call2 stdout=="feat: good".
      9. TestStub_MalformedEnvNoPanic: CmdSpec.Env with STAGECOACH_STUB_EXIT="not-a-number" (+ OUT="x") →
         stub exits 0, stdout=="x" (strconv.Atoi fallback, §2 robustness).
  - PATTERN: mirror executor_test.go (CmdSpec + Execute + errors.Is/As + time.Since bounds). Each test is
      independent (own t.TempDir via NewScript where needed). NO real git, NO real agent.
  - GOTCHA: for the render-path tests, Render needs a non-empty userPayload to populate Stdin (so the drain
    is exercised) — pass "fake prompt payload". model/provider/sysPrompt can be "" (the stub manifest has
    no model/provider/sys flags, so Render yields an empty-Args CmdSpec with Stdin=payload — perfect).
  - GOTCHA: TestStub_TimeoutKilled and the deadlock test use real wall-clock sleeps — keep them tight
    (200ms / 500ms) so the suite stays fast.

Task 4: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. Every other file
      (all of internal/{git,provider,prompt,config,generate}, cmd/stagecoach, pkg/*, Makefile) MUST be
      byte-unchanged. `go build ./...` MUST succeed (cmd/stubagent now compiles). `go test -race ./...`
      MUST be green (new stubtest tests pass, nothing regresses). `go vet` + `gofmt` clean.
```

### Implementation Patterns & Key Details

```go
// cmd/stubagent/main.go — the fake agent. Behavior is ENTIRELY env-driven (the manifest.Env seam).
// STDLIB ONLY. Drain stdin BEFORE sleeping (deadlock guard). Script mode gives call-varying output.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// envInt reads key as a non-negative int; any parse error / negative → def. Never panics (robustness).
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}

func main() {
	// 1. Drain stdin FIRST (deadlock guard): the executor pipes the payload via a bounded OS pipe; if we
	//    slept before draining and the payload exceeded the buffer, parent+child would deadlock. io.Discard
	//    consumes the prompt as a real agent would. /dev/null (Stdin=="") → io.Copy returns immediately.
	io.Copy(io.Discard, os.Stdin)

	// 2. Sleep AFTER draining (timeout simulation). The parent isn't blocked on stdin anymore.
	if ms := envInt("STAGECOACH_STUB_SLEEP_MS", 0); ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}

	// 3. Stderr (captured separately by Execute; useful for verbose-mode / stderr tests).
	if s := os.Getenv("STAGECOACH_STUB_STDERR"); s != "" {
		fmt.Fprint(os.Stderr, s)
	}

	// 4. Select + write stdout. Script mode ⇒ call-varying (dedupe loop); else single-response OUT.
	out := os.Getenv("STAGECOACH_STUB_OUT")
	if scriptFile := os.Getenv("STAGECOACH_STUB_SCRIPT"); scriptFile != "" {
		out = selectScripted(scriptFile)
	}
	fmt.Fprint(os.Stdout, out) // EXACTLY `out` — no extra newline (ParseOutput trims; assertions stay byte-exact)

	// 5. Exit with the configured code (non-zero simulates a failed agent → orchestrator retry/rescue).
	os.Exit(envInt("STAGECOACH_STUB_EXIT", 0))
}

// selectScripted returns the call-indexed line of the script file, advancing a file-backed counter so
// successive invocations of the stub (each a fresh process) get successive responses. Blank lines are
// significant (empty output ⇒ ParseOutput ok=false ⇒ orchestrator retries = parse-failure-then-rescue).
func selectScripted(scriptFile string) string {
	data, err := os.ReadFile(scriptFile)
	if err != nil {
		return "" // missing/unreadable script ⇒ empty output
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return ""
	}
	index := 0
	if counterFile := os.Getenv("STAGECOACH_STUB_COUNTER"); counterFile != "" {
		index = readCounter(counterFile)
		writeCounter(counterFile, index+1) // best-effort; serial callers make races impossible (§3)
	}
	if index < 0 || index >= len(lines) {
		index = len(lines) - 1 // clamp to last → stable tail after the script is exhausted
	}
	return lines[index]
}

func readCounter(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func writeCounter(path string, n int) {
	_ = os.WriteFile(path, []byte(strconv.Itoa(n)), 0o644)
}

// (bufio is imported for completeness if a streaming read is later desired; if unused, drop it to keep
// goimports/vet happy — prefer to OMIT unused imports. Final import set: fmt, io, os, strconv, strings, time.)
```

```go
// internal/stubtest/stubtest.go — the reusable helper. IMPORTABLE (regular package, not _test.go).
package stubtest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/dustin/stagecoach/internal/provider"
)

// Options configures a stub invocation; Manifest/Env translate it to STAGECOACH_STUB_* env vars.
type Options struct {
	Out            string // STAGECOACH_STUB_OUT (single-response; used when Script=="")
	Exit           int    // STAGECOACH_STUB_EXIT (default 0; non-zero ⇒ failed-agent simulation)
	SleepMS        int    // STAGECOACH_STUB_SLEEP_MS (default 0; >0 ⇒ slow/timing-out agent)
	Stderr         string // STAGECOACH_STUB_STDERR (default "")
	Script         string // STAGECOACH_STUB_SCRIPT path (call-varying mode; "" disables)
	Counter        string // STAGECOACH_STUB_COUNTER path (used with Script)
	Output         string // manifest Output; "" → "raw"
	StripCodeFence *bool  // manifest StripCodeFence; nil → true
}

// Build compiles ./cmd/stubagent ONCE per test process (cached) and returns its path. Skips t if the go
// toolchain isn't on PATH. The path is reused across all tests in the binary.
//
// GOTCHA: t.Skip/t.Fatal are called AFTER sync.Once.Do — never inside it. Do() marks itself done even when
// f calls runtime.Goexit (t.Skip) or panics, so a Skip-in-Do would leave stubPath empty and make later
// callers fail instead of skip. Flags (stubNoGo/stubBuildErr) are set in Do; Skip/Fatal happen after.
var (
	stubOnce     sync.Once
	stubPath     string
	stubNoGo     bool
	stubBuildErr string
)

func Build(t testing.TB) string {
	t.Helper()
	stubOnce.Do(func() {
		goPath, err := exec.LookPath("go")
		if err != nil {
			stubNoGo = true
			return
		}
		dir, err := os.MkdirTemp("", "stagecoach-stubagent-*")
		if err != nil {
			stubBuildErr = fmt.Sprintf("mkdtemp: %v", err)
			return
		}
		name := "stubagent"
		if runtime.GOOS == "windows" {
			name = "stubagent.exe"
		}
		stubPath = filepath.Join(dir, name)
		// Import-path form resolves from any cwd (no cmd.Dir needed).
		build := exec.Command(goPath, "build", "-o", stubPath, "github.com/dustin/stagecoach/cmd/stubagent")
		if out, err := build.CombinedOutput(); err != nil {
			stubBuildErr = fmt.Sprintf("%v\n%s", err, out)
			stubPath = ""
		}
	})
	if stubNoGo {
		t.Skipf("go toolchain not on PATH; cannot build stubagent")
	}
	if stubBuildErr != "" {
		t.Fatalf("go build stubagent: %s", stubBuildErr)
	}
	return stubPath
}

// optsEnvMap is the single source of truth for the STAGECOACH_STUB_* knobs (Env and Manifest both use it).
func optsEnvMap(o Options) map[string]string {
	m := map[string]string{
		"STAGECOACH_STUB_EXIT": strconv.Itoa(o.Exit),
	}
	if o.SleepMS > 0 {
		m["STAGECOACH_STUB_SLEEP_MS"] = strconv.Itoa(o.SleepMS)
	}
	if o.Stderr != "" {
		m["STAGECOACH_STUB_STDERR"] = o.Stderr
	}
	if o.Script != "" {
		m["STAGECOACH_STUB_SCRIPT"] = o.Script
		if o.Counter != "" {
			m["STAGECOACH_STUB_COUNTER"] = o.Counter
		}
	} else {
		m["STAGECOACH_STUB_OUT"] = o.Out // single-response mode
	}
	return m
}

// Env returns the "K=V" env slice for o (os.Environ() + STAGECOACH_STUB_*). Use to build a raw CmdSpec.
func Env(o Options) []string {
	env := os.Environ()
	for k, v := range optsEnvMap(o) {
		env = append(env, k+"="+v)
	}
	return env
}

// local pointer helpers (provider.strPtr/boolPtr are unexported — different package can't call them).
func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

// Manifest returns a test-only provider.Manifest pointing Command at the stub, ready to Render+Execute.
func Manifest(bin string, o Options) provider.Manifest {
	out := o.Output
	if out == "" {
		out = "raw"
	}
	scf := true
	if o.StripCodeFence != nil {
		scf = *o.StripCodeFence
	}
	return provider.Manifest{
		Name:             "stub",
		Command:          strPtr(bin),
		PromptDelivery:   strPtr("stdin"),
		Output:           strPtr(out),
		StripCodeFence:   boolPtr(scf),
		Env:              optsEnvMap(o),
	}
}

// NewScript wires call-varying mode: responses[0] is call 1's stdout, responses[1] call 2's, etc.; blank
// entries are significant (empty output ⇒ ParseOutput ok=false). After the list is exhausted the last
// response repeats. Files live in t.TempDir() (auto-cleaned).
func NewScript(t testing.TB, bin string, responses []string) provider.Manifest {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "script.txt")
	if err := os.WriteFile(script, []byte(strings.Join(responses, "\n")), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	counter := filepath.Join(dir, "counter") // absent ⇒ stub reads 0
	return Manifest(bin, Options{Script: script, Counter: counter})
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. cmd/stubagent is stdlib-only; internal/stubtest imports only the already-present
        internal/provider. `go mod tidy` MUST be a no-op. `git diff --exit-code go.mod go.sum` empty.

PACKAGE EDGES (import graph):
  - cmd/stubagent/main.go → (stdlib: bufio/fmt/io/os/strconv/strings/time) ONLY. NO internal/*, NO 3rd-party.
        (Drop `bufio` if unused — final set: fmt/io/os/strconv/strings/time.)
  - internal/stubtest/stubtest.go → (stdlib: os/os/exec/path/filepath/runtime/strconv/strings/sync/testing)
        + github.com/dustin/stagecoach/internal/provider. ONE internal edge (stubtest → provider); no cycle.
  - internal/stubtest/stubtest_test.go → stdlib (context/errors/strings/testing/time) + internal/provider
        + the stubtest package itself (white-box).

UPSTREAM CONTRACT (the seam — already built by P1.M2, read-only):
  - provider.CmdSpec{Command, Args, Stdin, Env} (render.go) — the stub is invoked as Command=<bin path>,
        Args=<none beyond payload routing>, Stdin=<prompt payload>, Env=<STAGECOACH_STUB_* + os.Environ()>.
  - provider.Execute(ctx, spec, timeout) (executor.go) — runs the stub, returns (stdout, stderr, err).
        timeout→DeadlineExceeded; non-zero exit→wrapped *exec.ExitError; setupProcessGroup makes it killable.
  - provider.Manifest{Env map[string]string} (manifest.go) — the test-only manifest's Env is the stub's
        config channel; Render copies it (os.Environ()+env, last-wins) into CmdSpec.Env.
  - provider.ParseOutput(raw, m) (parse.go) — consumes the stub's stdout; empty ⇒ ok=false ⇒ orchestrator
        retries (the parse-failure lever the stub exposes via OUT="" or a blank script line).

DOWNSTREAM CONTRACTS (the consumers — do NOT implement here, just honor the API):
  - P1.M3.T4.S2 (CommitStaged integration tests, package generate): `bin := stubtest.Build(t)`;
        `m := stubtest.Manifest(bin, stubtest.Options{Out:"feat: x"})` OR `stubtest.NewScript(t, bin, []string{"dup","fresh"})`;
        then `m.Render(...)` → `provider.Execute(...)` → `provider.ParseOutput(...)` → assert the commit via git.
  - P1.M5.T1.S1 (property/invariant tests): same Build + Manifest/NewScript to drive the pipeline while
        asserting idempotent-index / atomic-HEAD / snapshot-immutability.
  => The stubtest.{Build,Options,Manifest,Env,NewScript} signatures are FROZEN after this subtask.

FROZEN FILES (do NOT edit):
  - All of internal/{git,provider,prompt,config,generate}, cmd/stagecoach/main.go, pkg/*, Makefile,
        go.mod, go.sum. Only the 3 NEW files are created.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the three new files
gofmt -w cmd/stubagent/main.go internal/stubtest/stubtest.go internal/stubtest/stubtest_test.go

# Vet the new packages
go vet ./cmd/stubagent/ ./internal/stubtest/

# Lint (if available) — MUST be clean
golangci-lint run ./cmd/stubagent/ ./internal/stubtest/ 2>/dev/null || echo "(golangci-lint not available — skip)"

# Confirm cmd/stubagent/main.go imports ONLY stdlib (no internal/*, no third-party)
go list -deps ./cmd/stubagent | grep -E 'dustin/stagecoach|github\.com/' || echo "OK: stubagent has no internal/3rd-party deps"

# Confirm internal/stubtest imports ONLY internal/provider (+ stdlib)
go list -deps ./internal/stubtest | grep 'dustin/stagecoach'   # → only .../internal/provider

# Confirm cmd/stubagent uses "// Command stubagent" (not "// Package") doc comment
grep -n '^// Command stubagent' cmd/stubagent/main.go   # → the doc line
grep -n '^// Package' cmd/stubagent/main.go && echo "FAIL: main pkg has Package doc" || echo "OK"

# Confirm go.mod/go.sum unchanged
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged ✓"

# Expected: Zero errors. `go vet` clean. No stray deps. main package has a Command doc.
```

### Level 2: Unit Tests (THE KEYSTONE — drive the stub through provider.Execute)

```bash
# Run the new stubtest suite verbosely (Build compiles the stub once on first use)
go test -race -v ./internal/stubtest/

# Also run the cmd/stubagent package (it has no _test.go of its own if all tests live in stubtest;
# `go test ./cmd/stubagent/` just confirms it compiles as a test target — expected: "no test files" OK)
go test ./cmd/stubagent/

# Full module — confirm NOTHING else regressed (all P1.M1/M2/M3 suites still green)
go test -race ./...

# Expected: All stubtest tests pass. The load-bearing assertions are:
#   - echo success (stdout == STAGECOACH_STUB_OUT verbatim)
#   - non-zero exit ⇒ wrapped *exec.ExitError (errors.As)
#   - timeout ⇒ context.DeadlineExceeded AND returns within ~3s (process-group kill fired)
#   - 1 MiB stdin + sleep completes (drain-before-sleep — no deadlock)
#   - NewScript call-varying (call1=responses[0], call2=responses[1], call3=clamp-to-last)
#   - blank script line ⇒ empty stdout (parse ok=false)
#   - malformed EXIT ⇒ exit 0 (no panic)
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-module build — cmd/stubagent now compiles as a real binary target
go build ./...

# Manual sanity: run the stub directly and observe behavior (NOT a test — eyeball check)
BIN=$(mktemp); go build -o "$BIN" ./cmd/stubagent
STAGECOACH_STUB_OUT="feat: manual check" "$BIN"; echo " (exit=$?)"
# Expected: prints "feat: manual check", exit 0.

STAGECOACH_STUB_EXIT=42 "$BIN" 2>/dev/null; echo "exit=$?"
# Expected: exit=42.

echo "prompt payload" | STAGECOACH_STUB_OUT="drained ok" "$BIN"
# Expected: prints "drained ok" (stdin was drained without deadlock).

# Drive it through the REAL Execute seam (one-liner via go run of a tiny check, OR rely on the stubtest
# suite which does exactly this). The stubtest suite IS the integration validation — Level 2 covers it.

# Expected: `go build ./...` succeeds; the manual runs print/exit as above; the stubtest suite passes.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Not applicable for this subtask beyond the seam coverage already in Level 2 (the stub has no DB/UI/git/
# network/config/signal surface). The strongest creative checks:
#   1. Cross-platform: `GOOS=windows go build -o /tmp/stub.exe ./cmd/stubagent` succeeds (Windows CI §20.4).
#      `GOOS=darwin GOARCH=arm64 go build -o /tmp/stub ./cmd/stubagent` succeeds (macOS arm64).
#   2. Trace the env seam end-to-end: Manifest.Env → Render (os.Environ()+env) → CmdSpec.Env → Execute
#      (cmd.Env=spec.Env) → stub os.Getenv. Confirm a knob set in Options reaches the stub (the echo test
#      in Level 2 proves OUT does; the stderr test proves STDERR does).
#   3. Confirm the §20.1 layer-3 scenario list is fully achievable with the stub's knobs:
#        success              → Options{Out:"..."}
#        duplicate-retry      → NewScript(t, bin, []string{"<dup subject>","<fresh subject>"})
#        parse-failure-rescue → NewScript(t, bin, []string{"","<good>"})  (blank ⇒ ok=false)
#        timeout              → Options{SleepMS:2000} + a small orchestrator timeout
#        non-zero-exit        → Options{Exit:1}
#        CAS/root/auto-stage  → S2 repo setup; the stub just succeeds (Options{Out:"..."})
GOOS=windows go build -o /tmp/stub-check.exe ./cmd/stubagent && echo "windows build ✓"
GOOS=darwin  GOARCH=arm64 go build -o /tmp/stub-check     ./cmd/stubagent && echo "darwin/arm64 build ✓"
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed successfully.
- [ ] `go build ./...` succeeds (cmd/stubagent compiles as a binary target).
- [ ] All tests pass: `go test -race ./...` (new stubtest suite green; nothing else regressed).
- [ ] No vet errors: `go vet ./cmd/stubagent/ ./internal/stubtest/`.
- [ ] No formatting issues: `gofmt -l cmd/stubagent/ internal/stubtest/` (empty output).
- [ ] No lint warnings: `golangci-lint run ./cmd/stubagent/ ./internal/stubtest/` (if available).
- [ ] Cross-platform build: `GOOS=windows go build ./cmd/stubagent` succeeds (PRD §20.4).
- [ ] go.mod/go.sum byte-unchanged: `git diff --exit-code go.mod go.sum`.

### Feature Validation

- [ ] The stub echoes `STAGECOACH_STUB_OUT` to stdout verbatim (no extra newline) and exits 0.
- [ ] Multi-line `STAGECOACH_STUB_OUT` round-trips through stdout.
- [ ] `STAGECOACH_STUB_EXIT=1` (or any non-zero) ⇒ `provider.Execute` returns a wrapped `*exec.ExitError`.
- [ ] `STAGECOACH_STUB_SLEEP_MS` + a small Execute timeout ⇒ `context.DeadlineExceeded` within seconds
      (the process-group kill fired — the stub is killable like a real agent).
- [ ] `STAGECOACH_STUB_STDERR` is captured to Execute's stderr return, separate from stdout.
- [ ] A 1 MiB stdin payload + `SLEEP_MS` completes without hanging (stdin drained BEFORE sleep — §4).
- [ ] Script mode (`STAGECOACH_STUB_SCRIPT`+`_COUNTER`) returns successive responses per call (call-varying),
      clamps to the last response after exhaustion, and treats BLANK lines as empty output (parse ok=false).
- [ ] Malformed numeric env (`EXIT="x"`) never panics — falls back to the default (0).
- [ ] Scope respected: NO CommitStaged (S2), NO property-test bodies (M5.T1), NO integration_real suite
      (M5.T1.S2), NO edits to the provider pipeline or any other existing file.

### Code Quality Validation

- [ ] Follows existing patterns: CmdSpec+Execute+assert (mirror `internal/provider/executor_test.go`);
      `t.Skip` on missing binary (mirror `mustBin`); `t.TempDir()` fixtures (mirror git tests).
- [ ] File placement matches the desired codebase tree (`cmd/stubagent/main.go`, `internal/stubtest/{stubtest,stubtest_test}.go`).
- [ ] Anti-patterns avoided (see Anti-Patterns section).
- [ ] Imports properly managed: stub binary stdlib-only; helper imports only `internal/provider` + stdlib.
- [ ] `cmd/stubagent/main.go` has a `// Command stubagent` doc (NOT `// Package`); `internal/stubtest/stubtest.go`
      has the `// Package stubtest` doc; `stubtest_test.go` has none.
- [ ] `stubtest` API (`Build`/`Options`/`Manifest`/`Env`/`NewScript`) matches the FROZEN contract for S2/M5.

### Documentation & Deployment

- [ ] Code is self-documenting with PRD-§20.1-cited doc comments (the stub IS §20.1 layer 3's fake agent).
- [ ] No new environment variables in the SHIPPED product (STAGECOACH_STUB_* are test-only — they configure
      a test binary, never read by production code).
- [ ] No new config keys (none needed — the stub is configured via a test-only Manifest's Env map).

---

## Anti-Patterns to Avoid

- ❌ Don't make the stub a shell script — PRD §20.4 requires Windows CI; a `#!/bin/sh` stub won't run there. Use a Go binary.
- ❌ Don't configure the stub via flags or stdin — flags collide with the renderer's argv; stdin IS the prompt payload. Use env vars (the existing Manifest.Env→CmdSpec.Env→cmd.Env seam).
- ❌ Don't sleep before draining stdin — the executor's stdin pipe is bounded; a large payload + sleep deadlocks. ALWAYS `io.Copy(io.Discard, os.Stdin)` first.
- ❌ Don't skip blank lines in the script file — an empty output IS the parse-failure trigger (ParseOutput ok=false). Blank lines are significant.
- ❌ Don't put the helper in a `_test.go` file — test files aren't importable by other packages' tests (S2, M5). Use a regular `internal/stubtest/stubtest.go` (like `net/http/httptest`).
- ❌ Don't call `provider.strPtr`/`boolPtr` from package stubtest — they're unexported. Use local `&`-helpers.
- ❌ Don't add a file lock to the counter — the orchestrator calls the agent serially (one in-flight per CommitStaged); each test owns its own counter file. A lock adds Unix-only syscalls (breaks Windows) for no benefit.
- ❌ Don't let a malformed env value panic — `strconv.Atoi` errors must fall back to the default (robustness).
- ❌ Don't rebuild the stub per test — cache via `sync.Once` so one `go test` run compiles it exactly once.
- ❌ Don't implement CommitStaged, the property tests, or the integration_real suite here — those are S2 / M5.T1 / M5.T1.S2. This subtask is the stub + helper ONLY.
- ❌ Don't add a trailing newline beyond the configured output — emit EXACTLY `STAGECOACH_STUB_OUT`/the script line so assertions stay byte-exact (ParseOutput trims anyway, but exactness makes tests crisp).
