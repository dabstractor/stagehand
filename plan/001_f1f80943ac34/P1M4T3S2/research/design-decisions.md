# Design Decisions & Findings — P1.M4.T3.S2 Verbose Mode

## Findings (F1–F8) — facts established by reading the codebase

### F1 — `cfg.Verbose` is ALREADY fully resolved across all 7 layers.
`config.Config.Verbose bool` (config.go:21) is populated by every loader: `Defaults()` (false),
TOML `[defaults] verbose` (file.go:32/136), git `stagecoach.verbose` (git.go:148),
`STAGECOACH_VERBOSE` env (load.go:108–111), and `--verbose/-v` flag (load.go:146–149). All use the
DIRECT-set idiom so `--verbose=false` / `STAGECOACH_VERBOSE=false` work (the escape hatch).
→ **This task does NOT touch config.** It CONSUMES `cfg.Verbose`. Verified by grep: no loader is
missing verbose. So the verbose *intent* is solved; only the verbose *output* is missing.

### F2 — The verbose output does not exist anywhere yet.
`grep -rn "VerboseCommand\|VerboseRawOutput\|VerboseRetry\|DEBUG:" internal/ pkg/ cmd/` → zero
matches. The executor returns `(stdout, stderr, err)` but never logs them; the orchestrator's
retry loop has no diagnostics. So S2 is GREENFIELD output plumbing — no existing verbose code to
refactor, only seams to wire.

### F3 — `provider.Execute` is called in exactly 3 production sites + 9 test calls.
- `internal/generate/generate.go:194` (CommitStaged loop)
- `pkg/stagecoach/stagecoach.go:257` (runPipeline dry-run single pass)
- `pkg/stagecoach/stagecoach.go:295` (runPipeline commit loop)
- `internal/provider/executor_test.go` — 9 `Execute(ctx, spec, timeout)` calls.

Adding a nil-safe `vb *ui.Verbose` param to `Execute` is **compiler-driven**: `go build`/`go test`
will list EVERY call site that needs updating. Production sites pass `deps.Verbose`; tests append
`nil`. No call site can be silently missed.

### F4 — There are TWO generation code paths that both need verbose wiring.
1. `generate.CommitStaged` (generate.go) — the common path (no DryRun, no SystemExtra).
2. `pkg/stagecoach.runPipeline` (stagecoach.go) — the DryRun and/or SystemExtra path.
Both contain the Render→Execute→Parse→dedupe loop. Because `generate.Deps` carries the `Verbose`
sink and BOTH paths receive `deps`, threading `deps.Verbose` wires BOTH paths uniformly (Execute
gets it as a param; each loop logs retries). **Do not wire only one path** — that would leave
`--dry-run -v` silent, contradicting Appendix B.3+B.4.

### F5 — `generate.Deps` is the natural injection point (already a DI struct).
`generate.Deps{Git, Manifest}` is explicitly an injected-dependencies struct (generate.go:24
docstring: "carries the runtime collaborators that vary by environment/test"). Adding a
`Verbose *ui.Verbose` field is consistent with its purpose. `buildDeps` (stagecoach.go:184)
constructs it — the one place to set `deps.Verbose = ui.NewVerbose(opts.Verbose, cfg.Verbose)`.
Deps zero-value (`Verbose: nil`) is **nil-safe** → all existing Deps constructors (generate_test,
stagecoach_test) keep working unchanged.

### F6 — The public `Options` must stay additive-only (PRD §14.1, Appendix E item 6).
`pkg/stagecoach.Options` docstring: "ADDITIVE-ONLY for future versions: new fields may be added,
existing fields will not be removed or repurposed." Adding `Verbose io.Writer` (a **stdlib** type
— no `internal/ui` leak into the public surface) is compliant. Zero value `nil` ⇒ silent, so all
existing `Options{Provider: "stub"}` test literals are unaffected. The construction of the
internal `*ui.Verbose` happens INSIDE `GenerateCommit`/`buildDeps`, not at the public seam.

### F7 — `TestRunDefault_DryRun` locks stdout to be message-only (the stream invariant).
default_action_test.go:272 asserts `strings.TrimSpace(stdout) == "feat: dry run"` EXACTLY. Verbose
MUST write to the **stderr** writer. In that test `cfg.Verbose == false` so verbose is a no-op
anyway; but the design must guarantee DEBUG lines can never reach stdout. The `*ui.Verbose` writes
only to its own `w` field (set to `cmd.ErrOrStderr()` by the CLI), so this is structurally
guaranteed.

### F8 — P1.M4.T2.S1 (signal handler) is COMPLETE; the "do not touch executor/generate" restriction
from the S1 PRP era NO LONGER APPLIES.
`internal/signal/` exists; `executor.go` already calls `signal.RegisterChild/ClearChild`;
`generate.go` calls `signal.SetSnapshot/SetCandidate/RestoreDefault/ClearSnapshot`. So S2 may freely
edit `executor.go`, `generate.go`, `stagecoach.go`, `default_action.go`. (S1's PRP scoped those as
do-not-touch because T2.S1 was then in-flight; the plan_status now shows T2.S1 = Complete.)

## Decisions (D1–D9)

### D1 — Verbose lines use the `DEBUG:` prefix (contract-driven), NOT `↳`.
Per the work-item contract ("commit-pi uses VERBOSE=1 with DEBUG: prefix lines") and
external-research.md §1. Keeps verbose visually distinct from S1's always-on `↳` progress and
avoids editing S1's `output.go`. Formats are fixed/deterministic (see external-research.md §1 table).

### D2 — The verbose sink is a NEW `*ui.Verbose` type in a NEW file `internal/ui/verbose.go`.
Why a new type (not extending S1's `*ui.UI`): adding a `verbose bool` field to `UI` requires
editing S1's `output.go` (the struct literal + `New` signature live there) → merge-conflict risk
with the parallel S1 implementation. A sibling file `verbose.go` with its own `Verbose` type is
**zero-conflict** and keeps single-responsibility (UI=color/progress, Verbose=diagnostics). The CLI
constructs both objects from the same inputs (`stderr` + flags). (If S1 is already complete at
implementation time, the implementer MAY instead fold verbose into `*ui.UI` — but the separate-type
approach is the safe default and is what this PRP specifies.)

### D3 — `*ui.Verbose` methods are NIL-SAFE (nil receiver, nil writer, off → all no-op).
Signature: `func (v *Verbose) VerboseCommand(cmd string)` etc. with a leading
`if v == nil || v.w == nil || !v.on { return }`. This makes threading trivial: callers pass
`deps.Verbose` (which may be nil) and call methods unconditionally — no `if verbose != nil` guards
scattered through the pipeline. Tests pass `nil` for non-verbose cases.

### D4 — Core packages (`provider`, `generate`) import `internal/ui` directly (no interface layer).
`internal/ui` is stdlib-only → no import cycle (F6/external §6). Defining a `VerboseSink`
interface in core + implementing it in ui would preserve strict layering but adds a type for 3
methods with a single implementation — over-engineering for v1. The project already accepts
cross-cutting internal imports (`signal` is imported by both provider and generate). Direct
concrete-type threading it is. (Revisit if a second implementation ever appears.)

### D5 — `provider.Execute` gains a nil-safe `vb *ui.Verbose` parameter (honors the contract literally).
The contract: "Wire these into the executor (log the rendered command)… OUTPUT: Verbose logging
wired into the executor (P1.M2.T5)." So Execute logs `VerboseCommand` (argv, before Start) and
`VerboseRawOutput` (captured stdout, after Wait — on BOTH success and error paths, since partial
output aids diagnosis). The orchestrator logs `VerboseRetry`. New signature:
`Execute(ctx, spec, timeout, vb *ui.Verbose)`. Existing 9 test calls append `nil` (F3, compiler-driven).

### D6 — `VerboseCommand` logs ARGV ONLY, never Env (PRD §19 security — external §2).
The display string is `strings.Join(append([]string{spec.Command}, spec.Args...), " ")`. `spec.Env`
(`os.Environ()` + manifest env, possibly carrying `*_API_KEY`) is NEVER logged. This is the #1
security constraint and is explicitly required by §19 line 1203.

### D7 — Retries are logged at the FAILURE site, 1-based, in BOTH loops.
In each retry loop (CommitStaged + runPipeline), the two `continue` points (parse-fail, duplicate)
call `deps.Verbose.VerboseRetry(attempt+1, <reason>)` BEFORE the `continue`. `attempt+1` ⇒ 1-based
(matches Appendix B.4 "Attempt 1"). Reasons: parse-fail → `"parse failed (no valid commit
message)"`; duplicate → `fmt.Sprintf("subject %q matches an existing commit", subject)`. The
successful (final) attempt emits NO retry line (its acceptance is the normal success flow).

### D8 — `Options.Verbose io.Writer` is the public seam; `*ui.Verbose` is constructed internally.
`GenerateCommit` does `deps.Verbose = ui.NewVerbose(opts.Verbose, cfg.Verbose)` after `buildDeps`.
So `on = cfg.Verbose` (the single source of truth for "is verbose on"), `w = opts.Verbose` (the
sink; nil ⇒ silent even if cfg.Verbose, because a library has no business writing to os.Stderr).
The CLI passes `Options.Verbose: cmd.ErrOrStderr()`. A library consumer passes its own writer or
nil. This cleanly separates intent (cfg.Verbose) from channel (opts.Verbose) and keeps the library
side-effect-free by default.

### D9 — VERBOSE=2 / stdin logging is OUT OF SCOPE (documented, deferred).
Per external-research.md §3: `Config.Verbose` is a `bool`; `ParseBool("2")` fails; supporting
VERBOSE=2 needs a cross-cutting config change to `int` (owned by P1.M1.T4). S2 implements VERBOSE=1
only. The `VerboseRawOutput` logs stdout (FR50 "raw agent stdout"); stdin is protected (§19) and
deferred. A code comment + this doc record the future hook.

## Out of scope (explicitly — do NOT implement)
- VERBOSE=2 stdin logging (D9).
- Editing S1's `internal/ui/output.go` (D2 — use a new `verbose.go`).
- Touching `config/*` (F1 — cfg.Verbose is already resolved).
- Colorizing verbose lines (verbose is plain `DEBUG:` text; color is S1's separate concern).
- Changing the §18.3 rescue block or any frozen output.
- The mid-pipeline "↳ Snapshotting…" / "↳ Attempt N … accepted" progress lines (those are
  progress/S1-domain; verbose only adds `DEBUG:` diagnostics).
