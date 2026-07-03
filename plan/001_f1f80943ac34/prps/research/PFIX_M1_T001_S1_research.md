# Research Notes — BUG-001 / PFIX_M1_T001_S1 (FR50 --verbose non-functional)

## Root cause (two independent defects)
1. `pkg/stagehand/stagehand.go` (~line 209, deps literal): constructs
   `Output: ui.NewOutput(os.Stdout, os.Stderr, false, false)` — `verbose` is
   hardcoded `false`. `cfg.Verbose` (resolved by config.Load from
   --verbose/-v/STAGEHAND_VERBOSE) is never threaded in. So even if Verbosef
   were called, the generate step could never emit verbose output.
2. `Verbosef` is defined in `internal/ui/output.go` (gates on `o.verbose`,
   writes to stderr) but has ZERO production call sites. A grep for
   `.Verbosef(` across internal/generate, internal/provider, cmd/stagehand,
   pkg/stagehand (excluding _test.go) returns nothing.

## FR50 spec (PRD §15.2 / prd_snapshot.md L318 / CONFIGURATION.md L278)
`--verbose` / `-v` / `STAGEHAND_VERBOSE=1` — print to STDERR:
  (a) the resolved provider command,
  (b) the raw agent stdout,
  (c) each retry attempt.
Stream discipline (FR51): verbose MUST go to stderr only; stdout must stay
byte-clean (only the FR42 success block + FR49 dry-run message hit stdout).

## Why the resolved command must be emitted by the EXECUTOR (not generate.go)
`provider.(*Executor).Run` does DefaultModel/DefaultProvider resolution
INSIDE Run:
    if model == ""    { model = m.DefaultModel }
    if provider == "" { provider = m.DefaultProvider }
then `m.Render(model, provider, sys, payload)`. Only the executor therefore
knows the truly-resolved command. generate.go passes `cfg.Model`/`cfg.Provider`
(which may be "" → auto-resolved) to Run, so it cannot reconstruct the exact
command without duplicating the resolution logic. => executor emits
resolved command + raw stdout; generate.go emits retry markers. (Matches the
bug-hunt suggestedFix verbatim.)

## Threading design (minimal, pattern-faithful)
- Add `Verbose bool` to `stagehand.Options` (additive — Options is documented
  additive-only; append after `Timeout`; existing callers use named fields, so
  no breakage).
- `cmd/stagehand/run.go buildOptions`: add `Verbose: cfg.Verbose` (the resolved
  value; mirrors how Provider/Model/Timeout are threaded). Remove/rewrite the
  "Known v1 gap" doc comment on `runDefault`.
- `pkg/stagehand/stagehand.go GenerateCommit`: build `out` ONCE with
  `opts.Verbose`, reuse for BOTH the executor's verbose sink AND Deps.Output.
- `provider.Executor`: add an OPTIONAL exported field `Output *ui.Output`
  (nil => silent). Do NOT change `NewExecutor(dir)` signature — it is called in
  ~12 test sites (executor_test.go uses `&Executor{}` struct literals;
  stubprovider_test.go / integration_test.go / invariants_test.go /
  integration_real_test.go use `provider.NewExecutor(dir|".")`). A nil field
  leaves all of them byte-identical (verbose no-ops). Set the field at the
  stagehand.go wiring point: `exec := provider.NewExecutor("."); exec.Output = out`.
- `provider -> ui` is a CLEAN new edge (ui is a leaf: imports only fmt/io/os;
  no cycle). No architecture rule forbids it (the only documented one-way edge
  is config -> provider).

## Call sites to add Verbosef
### internal/provider/executor.go `Run`
- After default-resolution + `m.Render` success, before `cmd.Start`:
  emit `m.Command` + `r.Args` (+ "(payload via stdin)" when r.DeliverViaStdin).
  GUARD every call: `if e.Output != nil { e.Output.Verbosef(...) }`.
- On the success return path (`case err := <-done: if err == nil`), before
  `return stdout.String(), nil`: emit raw `stdout.String()`.
  (Keep error paths unchanged to stay minimal; success path is the reproduced
  bug — empty stderr on success.)
### internal/generate/generate.go `CommitStaged` (two nested loops)
- Top of inner loop (before `deps.Runner.Run`): emit attempt markers using
  loop vars `dupAttempt` (outer) and `parseAttempt` (inner) — e.g.
  `out.Verbosef("stagehand: generation attempt (duplicate=%d parse=%d)\n", dupAttempt, parseAttempt)`.
- After Parse miss / duplicate rejection: emit the reason. `out` is already
  in scope (`cfg, out := deps.Config, deps.Output`).
- DO NOT re-emit raw stdout here (executor already does) — avoids duplicates.

## Docs impact
`docs/CONFIGURATION.md` ~L288 has a `> **Verbose v1 gap.**` note that becomes
false after the fix — remove/replace it so the doc matches behavior.
(Also the run.go "Known v1 gap" comment + the stagehand.go "ui.NewOutput uses
verbose=false" comment must be updated.)

## Validation commands (verified working in this repo)
- `go build ./...`                  (build/typecheck)  EXIT 0 today
- `go vet ./...`                    (vet)              EXIT 0 today
- `test -z "$(gofmt -s -l internal/ cmd/ pkg/)"`  (fmt gate; prints nothing today)
- `go test ./...`                   (full suite; currently green)
- golangci-lint is NOT installed in this env — use go vet + gofmt instead.

## Reproduction to confirm the fix (after implementation)
Build, then in a scratch repo with a staged change + a stub provider:
  `./bin/stagehand --provider <name> --verbose 2>/tmp/err.txt`
`/tmp/err.txt` must now contain the resolved command, raw stdout, and attempt
markers (was empty before). Without --verbose it must remain empty.
