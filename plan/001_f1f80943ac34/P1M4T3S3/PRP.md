---
name: "P1.M4.T3.S3 — Exit-code constants and ExitError type (VERIFY + HARDEN the already-shipped §15.4 system) — PRD §15.4, arch/go_ecosystem_patterns.md §1.2"
description: |

  ⚠️ THIS IS NOT A CREATION TASK. The §15.4 exit-code system (constants 0/1/2/3/124, the `ExitError`
  type with `Error()`+`Unwrap()`, and `For(err) int` using `errors.As`) **already exists, is fully
  wired into `main()` and every `RunE`, and is green-tested** — it shipped in **P1.M4.T1.S1** ("custom
  exit-code wiring (cobra)"). The P1.M4.T3.S1 PRP confirms the scope: *"Exit codes / ExitError —
  already shipped P1.M4.T1.S1 (`internal/exitcode`); S3 refine-only."* The plan status is "Researching".

  S3 is a **VERIFICATION + HARDENING + COMPLETENESS-AUDIT** pass against the P1.M4.T3.S3 contract.
  The implementing agent must NOT recreate, rename, or relocate anything — doing so would regress
  ~40 call sites (`internal/cmd/{root,default_action,providers,config}.go`), `cmd/stagecoach/main.go`,
  and every CLI test.

  CONTRACT (P1.M4.T3.S3, verbatim):
    1. RESEARCH: "PRD §15.4 exit codes: 0=success, 1=general error, 2=nothing to commit, 3=rescue,
       124=timeout. See go_ecosystem_patterns.md §1.2 for the ExitError pattern with errors.As."
    3. LOGIC: "Create internal/ui/exitcode.go (or internal/exitcode/exitcode.go). Define constants:
       ExitSuccess=0, ExitError=1, ExitNothingToCommit=2, ExitRescue=3, ExitTimeout=124. Define
       `ExitError struct{Code int; Err error}` with Error()+Unwrap(). Define `For(err error) int`
       using errors.As to extract the code, defaulting to 1. This is consumed by main() (P1.M4.T1.S1).
       Mock: unit tests for error→code mapping."
    4. OUTPUT: "Exit-code system for main() and all command RunE functions."

  CURRENT STATE (already satisfies every load-bearing requirement — READ `research/findings.md`):
    - `internal/exitcode/exitcode.go`: consts `Success=0,Error=1,NothingToCommit=2,Rescue=3,Timeout=124`
      (values IDENTICAL to contract; names omit the `Exit` prefix by design — D1); `ExitError{Code,Err}`
      with `Error()`+`Unwrap()`; `New(code,err)`; `For(err)` using `errors.As` (default 1) PLUS a
      correct generate-domain mapping (ErrNothingToCommit→2, ErrTimeout→124 BEFORE rescue, ErrRescue→3,
      context.DeadlineExceeded→124, ErrCASFailed→1) that the contract did not mention but that is
      load-bearing for the CLI's RunE handlers.
    - `internal/exitcode/exitcode_test.go`: 12-case table + a nil-Err test — **1 genuine gap**: no test
      for a `*ExitError` recovered via `errors.As` through a `fmt.Errorf("…: %w", …)` chain (the exact
      guarantee the contract names).
    - Wired: `cmd/stagecoach/main.go:25` (`exitcode.For` → `os.Exit`); ~40 `exitcode.New(...)` sites
      across the CLI RunE handlers; tests assert all 5 codes end-to-end.

  DELIVERABLE (bounded to `internal/exitcode/` ONLY — 0-1 optional doc edit + 1 mandatory test delta):
    EDIT internal/exitcode/exitcode_test.go  — ADD table rows proving the `errors.As` contract on a
                                               WRAPPED `*ExitError` (the one coverage gap), plus an
                                               optional ExitError-beats-sentinel precedence row.
    EDIT internal/exitcode/exitcode.go       — OPTIONAL ONLY: enrich the package/type doc comments to
                                               name P1.M4.T3.S3 + §15.4 + the intentional naming
                                               decision (D1). NO logic change. NO rename. NO relocate.
    (audit)                                  — a read-only grep audit confirming every RunE maps to the
                                               correct §15.4 code (already PASS — see findings.md §5).

  SCOPE BOUNDARY (owned by siblings / frozen — do NOT edit):
    - `cmd/stagecoach/main.go` — trivial `exitcode.For`→`os.Exit`; correct; leave it.
    - `internal/cmd/*` — all `exitcode.New(...)` call sites; the `handleGenError` mapping is CORRECT and
      its rescue branch is byte-FROZEN (§18.3). The parallel **P1.M4.T3.S2** (verbose) edits
      `default_action.go` (one line) — zero overlap with S3, but do NOT touch default_action.go.
    - `internal/generate/*`, `pkg/stagecoach/*`, `internal/ui/*` — exitcode CONSUMES the generate error
      taxonomy (read-only); do not change those packages.
    - Constant NAMES (`Success` not `ExitSuccess`, etc.) — intentional (D1); deployed at ~40 sites.

  SUCCESS: `go test -race ./internal/exitcode/ -v` green INCLUDING the new wrapped-ExitError case;
  `go vet ./internal/exitcode/` clean; `gofmt -l internal/exitcode/` empty; `grep` audit confirms no
  RunE returns a raw error that would mis-default to 1; `go test -race ./internal/cmd/` green (NO
  behavioral regression — S3 changes nothing the CLI depends on); the only files changed are inside
  `internal/exitcode/`.

---

## Goal

**Feature Goal**: Certify and harden Stagecoach's §15.4 exit-code system so it provably satisfies the
P1.M4.T3.S3 contract. The system is already built (P1.M4.T1.S1) and wired into `main()` + every
command `RunE`; this task is the **refinement gate**: verify the constants (0/1/2/3/124), the
`ExitError{Code,Err}` contract (`Error()`+`Unwrap()`), and the `For(err)` `errors.As`-based extraction
(defaulting to 1); close the single test-coverage gap (a `*ExitError` recovered through a
`fmt.Errorf("%w")` wrap chain); and lock the naming/intent in the doc comments — all with **zero
behavioral change** to the CLI.

**Deliverable** (0-1 optional doc edit + 1 mandatory test delta, confined to `internal/exitcode/`):
1. EDIT `internal/exitcode/exitcode_test.go` — ADD table rows:
   - `For(fmt.Errorf("wrap: %w", New(7, errors.New("x")))) == 7` — proves `errors.As` traverses the
     wrap chain to recover an `ExitError.Code` (the contract's core guarantee, currently untested).
   - (optional) `For(fmt.Errorf("%w", New(NothingToCommit, errors.New("y")))) == NothingToCommit` —
     proves the explicit-`ExitError` branch takes precedence over the generate-sentinel branch.
2. EDIT `internal/exitcode/exitcode.go` — OPTIONAL: enrich doc comments to reference P1.M4.T3.S3 + PRD
   §15.4 + the intentional naming decision (`Success`/`Error`/… within `package exitcode`, not the
   contract's `Exit`-prefixed names — D1). **NO logic change.**
3. AUDIT (read-only, no file change) — `grep` confirmation that every `RunE` error maps to the correct
   §15.4 code (already PASS — documented in `research/findings.md §5`).

**Success Definition**:
- `go test -race ./internal/exitcode/ -v` is green AND the new wrapped-`ExitError` case passes.
- `go vet ./internal/exitcode/` clean; `gofmt -l internal/exitcode/` empty.
- `go test -race ./internal/cmd/` green — **zero** behavioral regression (S3 changes nothing the CLI
  depends on; the only edits are inside `internal/exitcode/`).
- `grep` audit: no `RunE` returns a raw error that would mis-default to exit 1 where §15.4 intends
  another code (2/3/124).
- `git status` shows changes ONLY under `internal/exitcode/`.

## User Persona

**Target User**: the Stagecoach CLI *integrator/scripter* (PRD §7 personas) who pipelines
`stagecoach` in shell (`stagecoach --dry-run | git commit -F -`, lazygit keybinds, CI gates) and relies
on **deterministic exit codes** to branch: `2` = "nothing to commit, skip", `3` = "rescue, alert a
human", `124` = "timeout, retry", `0` = "done". Also the *contributor* adding a new `RunE` who needs
a single, documented, well-tested helper (`exitcode.New(code, err)`) + a `For()` that "just works" on
both explicit and domain errors.

**Use Case**: `stagecoach && echo OK || echo "rc=$?"; case $? in 2) ;; 3) notify ;; esac` — the exit
code is the machine contract. S3 guarantees the mapping is correct AND tested for the wrap-chain case
(so a future `RunE` that does `return fmt.Errorf("ctx: %w", exitcode.New(exitcode.Timeout, err))`
still exits 124).

**User Journey**: script runs `stagecoach` → main calls `exitcode.For(err)` → `errors.As` unwraps any
nesting to find the `*ExitError` (or falls through to the domain mapping) → `os.Exit(code)` → the
shell sees 0/1/2/3/124 and branches correctly.

**Pain Points Addressed**: silent regressions where a wrapped `ExitError` would wrongly default to 1
(untested path) — the new test locks the `errors.As` traversal guarantee the contract names.

## Why

- **Closes the P1.M4.T3.S3 contract as written** (constants + `ExitError` + `For` via `errors.As` +
  unit tests) — by *verifying* the already-shipped implementation rather than redundantly recreating it.
- **Locks the one untested guarantee.** The contract explicitly says `For` "uses `errors.As` to extract
  the code". `errors.As` traverses `fmt.Errorf("%w")` chains — but no existing test exercises an
  `ExitError` reached *through* a wrap. Adding that case converts an implicit stdlib guarantee into a
  checked regression test.
- **Prevents a catastrophic mis-implementation.** A naive reading of "Create internal/exitcode/
  exitcode.go" would overwrite a file referenced at ~40 sites + `main.go`, breaking the entire CLI.
  This PRP makes the "already exists — refine only" reality unambiguous (per the S1 PRP's "S3
  refine-only" and `research/findings.md`).
- **Documents intent** (D1 naming, §15.4 authority over arch/go_ecosystem_patterns.md §1.2's generic
  table, the timeout-before-rescue ordering) so future contributors don't "tidy" it into a regression.

## What

A bounded verification + hardening of `internal/exitcode/`:

```go
// internal/exitcode/exitcode.go — ALREADY CORRECT; S3 may ONLY enrich doc comments (optional).
const ( Success = 0; Error = 1; NothingToCommit = 2; Rescue = 3; Timeout = 124 )
type ExitError struct { Code int; Err error }   // Error()+Unwrap() already implemented
func New(code int, err error) *ExitError         // already implemented
func For(err error) int                           // errors.As + generate-domain mapping; default 1

// internal/exitcode/exitcode_test.go — ADD (mirror the existing `tests := []struct{name,err,want}{}` table):
{"wrapped ExitError → its Code (errors.As traverses %w)",
    fmt.Errorf("wrap: %w", New(7, errors.New("x"))), 7},                        // MANDATORY new row
// optional:
{"wrapped ExitError NothingToCommit beats sentinel branch",
    fmt.Errorf("%w", New(NothingToCommit, errors.New("y"))), NothingToCommit},
```

NO changes to `main.go`, `internal/cmd/*`, `internal/generate/*`, `pkg/stagecoach/*`, `internal/ui/*`,
or any constant identifier.

### Success Criteria

- [ ] `internal/exitcode/exitcode.go` constants are EXACTLY `Success=0, Error=1, NothingToCommit=2,
      Rescue=3, Timeout=124` (verified, not changed — D1 keeps the no-`Exit`-prefix names).
- [ ] `ExitError` implements `Error() string` (returns `Err.Error()` or `""` when `Err==nil`) and
      `Unwrap() error` (returns `Err`); `New(code,err)` constructs it. (Verified present.)
- [ ] `For(nil)==0`; `For(*ExitError)==Code`; `For(generic)==1`; `For` uses `errors.As`. (Verified.)
- [ ] **NEW**: `For(fmt.Errorf("…: %w", exitcode.New(7, e))) == 7` is asserted by an added test row
      (proves `errors.As` wrap-chain traversal — the contract's core guarantee).
- [ ] (optional) `For(fmt.Errorf("%w", New(NothingToCommit,e))) == NothingToCommit` (ExitError branch
      precedence over the generate-sentinel branch).
- [ ] `go test -race ./internal/exitcode/ -v` green; `go vet ./internal/exitcode/` clean;
      `gofmt -l internal/exitcode/` empty.
- [ ] `go test -race ./internal/cmd/` green — ZERO behavioral regression (S3 edits only `internal/exitcode/`).
- [ ] `grep` audit documents (in the implementation summary) that no `RunE` mis-defaults to 1.
- [ ] `git status` shows changes ONLY under `internal/exitcode/`.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can execute this verification+hardening
task from: the exact existing implementation (quoted below + in `research/findings.md`), the §15.4
table (the authoritative spec), the `errors.As` semantics (the one new test), the existing
table-driven test style to mirror, the call-site audit (already done — findings §5), and the explicit
"do NOT recreate/rename/relocate" guardrails. No signal/verbose/generate/CLI internals required (all
explicitly out of scope).

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M4T3S3/research/findings.md
  why: THE decisive doc. §1 line-by-line contract-vs-reality table (every load-bearing req ✅), §2 the
       naming decision (D1 — keep `Success`/`Error`/…, NOT `ExitSuccess`), §3 why `For()` is a correct
       SUPERSET (do not weaken the generate-domain mapping), §4 the ONE test gap (wrapped ExitError),
       §5 the call-site audit (PASS), §6 scope/parallel-coordination (S2 in-flight; do NOT touch
       default_action.go), §7 confidence 9/10.
  critical: §1 (don't recreate), §2 (don't rename), §4 (the one test to add), §6 (parallel boundary).

- file: internal/exitcode/exitcode.go   (P1.M4.T1.S1 — the ALREADY-CORRECT impl; S3 may only enrich docs)
  section: the `const` block (Success/Error/NothingToCommit/Rescue/Timeout — values are the §15.4 spec);
       `type ExitError struct { Code int; Err error }` + `Error()` + `Unwrap()` + `New(code, err)`;
       `func For(err error) int` (errors.As → Code; then generate-domain Is-chain; default Error).
  why: this IS the deliverable the contract describes — already built. S3 verifies it. The package doc
       already notes "§15.4 overrides arch/go_ecosystem_patterns.md §1.2's generic table (2=nothing-to-
       commit, not usage; 3=rescue, not config)" — that insight is load-bearing, do not lose it.
  pattern: read it, run its tests, confirm green; optionally enrich comments to name P1.M4.T3.S3.
  gotcha: do NOT remove the generate-domain mapping (ErrNothingToCommit/ErrTimeout/ErrRescue/
       context.DeadlineExceeded/ErrCASFailed) — it is load-bearing for the CLI RunE handlers. Do NOT
       reorder the ErrTimeout-before-ErrRescue check (a timeout is a RescueError{Kind:ErrTimeout}→124).

- file: internal/exitcode/exitcode_test.go   (P1.M4.T1.S1 — S3 EDITS: add rows to the `tests` table)
  section: `func TestFor` — `tests := []struct{ name string; err error; want int }{ ... }` then
       `for _, tc := range tests { t.Run(tc.name, ...) }`. MIRROR this style exactly for the new rows.
  why: the one mandatory change lives here. Add the wrapped-ExitError row; the existing runner picks it up.
  pattern: new row → `{"wrapped ExitError → Code (errors.As traverses %w)", fmt.Errorf("wrap: %w", New(7, errors.New("x"))), 7}`.
  gotcha: `errors`, `fmt` are already imported in the test. The runner asserts `got := For(tc.err); got != want → Errorf`.

- file: cmd/stagecoach/main.go   (P1.M4.T1.S1 — READ only; the consumer of For())
  section: `err := cmd.Execute(ctx); code := exitcode.For(err); if err != nil && err.Error() != "" { ... };
       os.Exit(code)`. The `err.Error() != ""` guard is why SILENT exitcodes (New(code,nil)) suppress
       main's "stagecoach: %v" print while still honoring the code.
  why: confirms For() is the single source of truth for the process exit code. No change needed.

- file: internal/cmd/default_action.go   (P1.M4.T1.S2 — READ only; the exit-mapping exemplar + FROZEN)
  section: `handleGenError` (L143-164) — `errors.As(err, &re)` → RescueError (timeout→124 else 3, silent);
       `errors.As(err, &ce)` → CASError (exit 1, silent); `errors.Is(ErrNothingToCommit)` → 2 (main prints);
       generic → exit 1. Plus ~12 explicit `exitcode.New(...)` sites in runDefault.
  why: PROVES the domain mapping in For() + the explicit New() calls cover every §15.4 outcome correctly.
  gotcha: DO NOT EDIT — (a) the rescue branch is byte-frozen (§18.3), (b) the parallel P1.M4.T3.S2 is
       editing this file (one line: `Verbose: stderr`). Zero overlap is required.

- docfile: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md
  section: §1.2 "Custom Exit Codes" — the `ExitError` + `errors.As` + `For()` pattern (illustrative).
  why: the contract points here as the pattern source. NOTE: its exit-code TABLE (2=usage, 3=config) is
       GENERIC/illustrative and is OVERRIDDEN by PRD §15.4 (2=nothing-to-commit, 3=rescue). The existing
       exitcode.go package doc already states this override; preserve that note.
  critical: take the PATTERN (ExitError+errors.As+For), NOT the illustrative code-table semantics.

- url: (PRD internal) PRD.md §15.4 — the AUTHORITATIVE exit-code table (0/1/2/3/124).
  why: the single source of truth for what each code means. The existing constants match it exactly.
  critical: 2=nothing-to-commit (NOT usage); 3=rescue (NOT config); 124=timeout (mirrors GNU `timeout`).

- url: https://pkg.go.dev/errors#As
  section: `errors.As` — "finds the first error in err's chain that matches target … unwraps … repeatedly".
  why: the guarantee the new test locks: a `*ExitError` nested under `fmt.Errorf("…: %w", …)` is still
       recovered by `errors.As(err, &ee)` inside `For()`.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                              # module github.com/dustin/stagecoach ; go 1.22 ; UNCHANGED (no new deps)
internal/
  exitcode/exitcode.go              # P1.M4.T1.S1 — consts + ExitError + For() (S3: optional doc enrich ONLY)
  exitcode/exitcode_test.go         # P1.M4.T1.S1 — 12-case table (S3 EDITS: +1 mandatory, +1 optional row)
  cmd/default_action.go             # P1.M4.T1.S2 — handleGenError + ~12 exitcode.New sites (READ; FROZEN; S2 edits 1 line)
  cmd/root.go                       # P1.M4.T1.S1 — 2 exitcode.New(Error,…) sites (READ)
  cmd/providers.go / cmd/config.go  # P1.M4.T1.S3/S4 — exitcode.New(Error,…) sites (READ)
cmd/stagecoach/main.go               # P1.M4.T1.S1 — exitcode.For → os.Exit (READ)
internal/generate/generate.go       # P1.M3.T4.S2 — ErrNothingToCommit/ErrTimeout/ErrRescue/ErrCASFailed + RescueError/CASError (READ — the taxonomy For() maps)
Makefile                            # build/test(-race)/vet/coverage/lint/clean (UNCHANGED)
```

### Desired Codebase tree with files to be added/changed

```bash
internal/exitcode/exitcode_test.go  # EDIT — ADD the wrapped-ExitError table row (mandatory) + optional precedence row.
internal/exitcode/exitcode.go       # EDIT (OPTIONAL) — enrich doc comments to name P1.M4.T3.S3 + §15.4 + D1 naming intent. NO logic/name/relocate change.
# ALL other files UNCHANGED. main.go, internal/cmd/*, internal/generate/*, pkg/stagecoach/*, internal/ui/* untouched.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (DO NOT RECREATE): internal/exitcode/exitcode.go already implements the ENTIRE contract and
// is referenced at ~40 sites + main.go. Overwriting/relocating it regresses the whole CLI. S3 = verify +
// harden, confined to internal/exitcode/. If you feel an urge to "create internal/ui/exitcode.go", STOP —
// read research/findings.md §1; the contract's "Create … (or internal/exitcode/exitcode.go)" was satisfied
// by the 2nd option in P1.M4.T1.S1.

// CRITICAL (DO NOT RENAME — D1): keep Success/Error/NothingToCommit/Rescue/Timeout (NO "Exit" prefix).
// The contract's "ExitSuccess=0, …" names are illustrative of the VALUES; the package name already
// supplies "Exit" context, so exitcode.Success is idiomatic. Renaming = ~40-site churn + merge-conflict
// risk with the parallel S2 (which edits default_action.go, saturated with exitcode.X references).

// CRITICAL (DO NOT WEAKEN For()'s domain mapping): For() maps generate.ErrNothingToCommit→2,
// ErrTimeout→124, ErrRescue→3, context.DeadlineExceeded→124, ErrCASFailed→1 IN ADDITION to the
// errors.As(ExitError) path. This is load-bearing — without it, every RunE would have to wrap generate
// errors manually. The contract didn't mention it because it predates P1.M3.T4.S2's error taxonomy.
// Leave the mapping intact; the new test ADDS coverage, it does not replace it.

// GOTCHA (timeout-before-rescue ordering is correct, do not "fix"): ErrTimeout is checked BEFORE
// ErrRescue because a timeout is a *RescueError{Kind: ErrTimeout} — it must map to 124, not 3. There is
// already an explicit test ("RescueError(ErrTimeout) → 124 (timeout before rescue)"). Keep the order.

// GOTCHA (the new test must use fmt.Errorf %w, NOT a direct *ExitError): the existing table already
// covers direct New(7,e) and wrapped SENTINELS. The GAP is a *ExitError reached THROUGH a wrap chain.
// Use fmt.Errorf("wrap: %w", New(7, errors.New("x"))) — errors.As traverses %w, so For() must yield 7.

// GOTCHA (ExitError branch precedes the sentinel branches): For() checks errors.As(err,&ee) FIRST, so
// an explicit New(NothingToCommit, e) (even wrapped) yields 2 via the ExitError branch, NOT via the
// ErrNothingToCommit Is-check. Both give 2, but the optional precedence test documents which branch fired.

// GOTCHA (parallel S2 may leave pkg/stagecoach mid-build): while P1.M4.T3.S2 is in flight, `go build ./...`
// can report `pkg/stagecoach/stagecoach.go: undefined: io` (S2 adding Options.Verbose io.Writer). That is
// NOT an S3 bug. Validate S3 with `go test ./internal/exitcode/` + `go vet ./internal/exitcode/` (self-
// contained); run the full `go test -race ./...` gate only after S2 has merged (or note the transient).

// GOTCHA (silent exitcodes): New(code, nil) → ExitError.Error()=="" → main.go's `err.Error() != ""`
// guard skips the "stagecoach: %v" print but os.Exit(code) STILL honors the code. This is how rescue/CAS
// print their own detailed message and suppress main's generic line. Do not "fix" the empty-string case.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/exitcode/exitcode.go — ALREADY IMPLEMENTED (P1.M4.T1.S1). S3 may ONLY enrich doc comments.
// (Reproduced for reference; DO NOT overwrite — confirm your working copy matches, then leave the logic
//  byte-identical. The only permitted change is OPTIONAL comment text.)
package exitcode

import (
	"context"
	"errors"
	"github.com/dustin/stagecoach/internal/generate"
)

const (
	Success         = 0
	Error           = 1
	NothingToCommit = 2
	Rescue          = 3
	Timeout         = 124
)

type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return ""
}
func (e *ExitError) Unwrap() error { return e.Err }

func New(code int, err error) *ExitError { return &ExitError{Code: code, Err: err} }

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
	if errors.Is(err, generate.ErrTimeout) {
		return Timeout
	}
	if errors.Is(err, generate.ErrRescue) {
		return Rescue
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}
	if errors.Is(err, generate.ErrCASFailed) {
		return Error
	}
	return Error
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY the existing implementation against the contract (READ + RUN, no edit)
  - RUN: `go test -race ./internal/exitcode/ -v` → must be green (12 cases + nil-Err). Capture output.
  - RUN: `go vet ./internal/exitcode/` → clean. `gofmt -l internal/exitcode/` → empty.
  - READ: internal/exitcode/exitcode.go — confirm const VALUES are 0/1/2/3/124; ExitError has
      Error()+Unwrap(); New(code,err); For() uses errors.As first, default Error(1).
  - READ: research/findings.md §1 table — cross-check each contract row is ✅.
  - GOTCHA: if any check FAILS, STOP and report — the "already shipped" premise would be wrong. (It is
      not; this is a guardrail.) Do NOT "fix" by rewriting — diagnose first.

Task 2: EDIT internal/exitcode/exitcode_test.go — ADD the wrapped-ExitError case (MANDATORY)
  - FILE: EDIT internal/exitcode/exitcode_test.go. Locate `func TestFor`'s `tests := []struct{...}{...}`.
  - ADD row (mirror the existing `{name, err, want}` shape EXACTLY):
        {"wrapped ExitError → Code (errors.As traverses %w)", fmt.Errorf("wrap: %w", New(7, errors.New("x"))), 7},
    Place it near the other ExitError rows (after "explicit ExitError custom code").
  - WHY: this is the ONE coverage gap (findings §4). It locks the contract's "uses errors.As to extract
      the code" guarantee for a *ExitError nested under fmt.Errorf("%w").
  - GOTCHA: `errors` and `fmt` are already imported — no new import. The shared runner asserts
      `got := For(tc.err); if got != tc.want { t.Errorf(...) }`, so the new row is exercised automatically.
  - GOTCHA: use `errors.New("x")` (not a generate sentinel) as the inner Err so ONLY the ExitError branch
      can produce the result — a clean test of errors.As traversal.

Task 3 (OPTIONAL): EDIT internal/exitcode/exitcode_test.go — ADD the ExitError-precedence case
  - ADD row:
        {"wrapped ExitError NothingToCommit beats sentinel branch", fmt.Errorf("%w", New(NothingToCommit, errors.New("y"))), NothingToCommit},
  - WHY: documents that the errors.As(ExitError) branch is checked BEFORE the generate-sentinel Is-checks
      (both yield 2 here, but the row pins which branch fired). Skip if you prefer a minimal diff.
  - GOTCHA: harmless if omitted — Task 2 is the mandatory one.

Task 4 (OPTIONAL): EDIT internal/exitcode/exitcode.go — enrich doc comments (NO logic change)
  - FILE: EDIT internal/exitcode/exitcode.go. You may extend the package doc comment and/or the
      ExitError/For doc comments to name: "P1.M4.T3.S3 (refine-verify of the system shipped in
      P1.M4.T1.S1)", "PRD §15.4 is authoritative (overrides arch/go_ecosystem_patterns.md §1.2's generic
      table)", and the D1 naming intent ("names omit the `Exit` prefix by design — the package name
      supplies the context").
  - PRESERVE: every const value, the ExitError methods, New(), and the ENTIRE For() body (incl. the
      generate-domain mapping + the ErrTimeout-before-ErrRescue order). Byte-identical logic.
  - GOTCHA: if any doubt, SKIP this task — a no-op doc edit is strictly better than an accidental logic
      change. The package doc is already excellent.

Task 5: AUDIT (read-only grep) — confirm no RunE mis-defaults to 1
  - RUN:
        grep -rn "return " internal/cmd/*.go | grep -v "_test"   # every RunE return path
        grep -rn "exitcode\.\(New\|Success\|Error\|NothingToCommit\|Rescue\|Timeout\)" internal/cmd/
    Confirm every error return is either an exitcode.New(...) with the correct §15.4 code OR a
    generate-domain error that For() maps correctly (nothing-to-commit/rescue/timeout/CAS).
  - EXPECTED: already PASS (findings §5). Document the result in the implementation summary; change NO file.
  - GOTCHA: this is a CONFIDENCE check, not a fix-it task. If you spot a genuine mis-mapping, STOP and
    report it (it would belong to the owning task, not S3's internal/exitcode-only scope).

Task 6: FINAL VALIDATION (the gate)
  - RUN: `gofmt -w internal/exitcode/`; `go vet ./internal/exitcode/`; `go test -race ./internal/exitcode/ -v`.
  - RUN: `go test -race ./internal/cmd/` → green (proves zero behavioral regression — S3 changed nothing
      the CLI depends on). NOTE: if the parallel S2 has NOT merged, `go test -race ./...` may fail to
      BUILD pkg/stagecoach (`undefined: io`) — that is S2's transient, not S3's; scope the regression
      check to ./internal/cmd/ + ./internal/exitcode/ and note the S2 dependency in the summary.
  - RUN: `git status` → changes ONLY under internal/exitcode/.
```

### Implementation Patterns & Key Details

```go
// The ONE mandatory test addition — mirror the existing table-driven runner VERBATIM:
tests := []struct {
	name string
	err  error
	want int
}{
	// ...existing rows...
	{"explicit ExitError custom code", New(7, errors.New("custom")), 7},
	// ↓ ADD (the gap): a *ExitError recovered THROUGH a fmt.Errorf("%w") wrap chain ↓
	{"wrapped ExitError → Code (errors.As traverses %w)", fmt.Errorf("wrap: %w", New(7, errors.New("x"))), 7},
	// ...existing rows...
}

// The contract's core guarantee, now under test:
//   For(fmt.Errorf("outer: %w", exitcode.New(exitcode.Timeout, err))) == 124   // errors.As traverses %w

// What NOT to do (the guardrails):
//   ✗ rewrite internal/exitcode/exitcode.go            (it's correct + deployed)
//   ✗ rename Success→ExitSuccess etc.                  (~40-site churn; D1)
//   ✗ trim For()'s generate-domain mapping             (load-bearing for RunE handlers)
//   ✗ touch main.go / internal/cmd/* / internal/ui/*   (frozen / parallel S2)
```

### Integration Points

```yaml
PACKAGE LAYOUT (PRD §14):
  - verify: internal/exitcode/exitcode.go is the §14 "exit-code system" (the §14 "internal/ui/exitcode.go"
    slot was implemented as internal/exitcode/ per P1.M4.T1.S1 — S1's PRP notes this). NO relocation.

CONSUMER (unchanged):
  - cmd/stagecoach/main.go:25 — `code := exitcode.For(err)` then `os.Exit(code)`. The single exit-code
    source of truth. S3 changes nothing here.

CLI RUNE HANDLERS (unchanged, frozen):
  - internal/cmd/{root,default_action,providers,config}.go — ~40 exitcode.New(...) sites. The
    handleGenError mapping (rescue→3, timeout→124, CAS→1, nothing-to-commit→2, generic→1) is correct and
    the rescue branch is byte-frozen (§18.3). S3 does not edit these.

ERROR TAXONOMY (consumed read-only by For()):
  - internal/generate/generate.go — ErrNothingToCommit/ErrTimeout/ErrRescue/ErrCASFailed +
    RescueError{Kind}/CASError. For()'s domain mapping depends on these; do not change them.

PARALLEL COORDINATION (P1.M4.T3.S2 — in flight):
  - S2 edits internal/cmd/default_action.go (1 line: Verbose: stderr) + executor.go/generate.go/
    stagecoach.go. ZERO overlap with S3 (internal/exitcode/ only). While S2 is mid-edit, `go build ./...`
    may transiently fail on pkg/stagecoach (`undefined: io`) — NOT an S3 bug; scope validation to
    ./internal/exitcode/ + ./internal/cmd/.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After any edit to internal/exitcode/* — fix before proceeding.
gofmt -w internal/exitcode/
go vet ./internal/exitcode/
gofmt -l internal/exitcode/   # must be empty

# Expected: zero errors. gofmt -l empty for internal/exitcode/.
```

### Level 2: Unit Tests (the core deliverable)

```bash
# The exit-code package — must be green INCLUDING the new wrapped-ExitError case.
go test -race ./internal/exitcode/ -v
# Expected: all green. Confirm the new "wrapped ExitError → Code (errors.As traverses %w)" row runs and
# passes (proves For() recovers an ExitError through a fmt.Errorf("%w") chain). Confirm the existing
# 12 cases + nil-Err test are unchanged and green.

# (optional) verbose subtest listing:
go test -race ./internal/exitcode/ -v -run TestFor
```

### Level 3: Regression (no behavioral change to the CLI)

```bash
# The CLI suite — proves S3 changed nothing the CLI depends on.
go test -race ./internal/cmd/ -v
# Expected: all green. root_test.go / default_action_test.go (asserts For()==Success/NothingToCommit/
# Rescue/Timeout/Error across 5 codes) / providers_test.go / config_test.go all unchanged and passing.

# NOTE on the full gate: while the parallel P1.M4.T3.S2 is in flight, `go test -race ./...` may fail to
# BUILD pkg/stagecoach (`undefined: io`) — that is S2's transient, not S3's. If you see it, scope to
# ./internal/exitcode/ + ./internal/cmd/ and note the S2 dependency. After S2 merges, `go test -race ./...`
# must be fully green.

# Whole-tree build sanity (post-S2-merge):
go build ./...
```

### Level 4: Audit & End-to-End (confidence, no file change)

```bash
# Audit: confirm every RunE maps to the correct §15.4 code (expected PASS — findings §5).
grep -rn "exitcode\.\(New\|Success\|Error\|NothingToCommit\|Rescue\|Timeout\)" internal/cmd/ | grep -v "_test"

# End-to-end exit-code proof (build + run in a scratch repo; observe $?):
make build
cd /tmp && rm -rf ec-smoke && git init ec-smoke && cd ec-smoke &&
  git config user.email t@t.co && git config user.name t
# nothing staged, --no-auto-stage → exit 2:
./path/to/bin/stagecoach --no-auto-stage; echo "rc=$?"   # expect rc=2
# a successful dry-run → exit 0 (needs a configured/stub provider); a timeout → 124; a rescue → 3.

# Expected: the shell observes 0/1/2/3/124 per §15.4 (the contract OUTPUT: "Exit-code system for main()
# and all command RunE functions"). These are integration confirmations, not S3 code changes.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed (Level 3 full-gate deferred until S2 merges if transient).
- [ ] `go test -race ./internal/exitcode/ -v` green, INCLUDING the new wrapped-ExitError row.
- [ ] `go test -race ./internal/cmd/` green — ZERO behavioral regression.
- [ ] `go vet ./internal/exitcode/` clean; `gofmt -l internal/exitcode/` empty.
- [ ] No new `go.mod` dependencies (stdlib + existing internal/generate only).

### Feature Validation

- [ ] Constants verified: `Success=0, Error=1, NothingToCommit=2, Rescue=3, Timeout=124` (PRD §15.4).
- [ ] `ExitError{Code,Err}` implements `Error()` (nil-Err→"") + `Unwrap()`; `New(code,err)` constructs it.
- [ ] `For(nil)→0`; `For(*ExitError)→Code`; `For(generic)→1`; `For` uses `errors.As` (default 1).
- [ ] **NEW**: wrapped `*ExitError` (`fmt.Errorf("…: %w", New(7,e))`) → 7, asserted by test.
- [ ] Audit confirms no `RunE` mis-defaults to 1 where §15.4 intends 2/3/124.
- [ ] End-to-end smoke: shell observes 0/1/2/3/124 for the corresponding conditions.

### Code Quality Validation

- [ ] NO recreation / rename / relocation of `internal/exitcode/exitcode.go` logic (D1, guardrails).
- [ ] `For()`'s generate-domain mapping + ErrTimeout-before-ErrRescue order preserved.
- [ ] File placement unchanged (`internal/exitcode/`); `git status` shows changes ONLY there.
- [ ] New test row mirrors the existing table-driven `{name,err,want}` style exactly.

### Documentation & Deployment

- [ ] (optional) Doc comments name P1.M4.T3.S3 + §15.4 authority + D1 naming intent.
- [ ] Implementation summary records: verification result, the added test, the audit result, and the
      S2-parallel note (transient pkg/stagecoach build state, if observed).

---

## Anti-Patterns to Avoid

- ❌ **Don't recreate `internal/exitcode/exitcode.go`.** It already exists (P1.M4.T1.S1), is correct, and
  is referenced at ~40 sites + `main.go`. Overwriting it regresses the entire CLI. S3 = verify + harden.
- ❌ **Don't rename the constants** (`Success`→`ExitSuccess`, …). The no-prefix names are idiomatic within
  `package exitcode` and deployed everywhere; renaming is pure churn + merge-conflict risk with S2.
- ❌ **Don't trim `For()`'s generate-domain mapping** or reorder ErrTimeout before ErrRescue. The mapping
  is load-bearing (RunE handlers rely on it); the order is correct (a timeout is a RescueError→124, not 3).
- ❌ **Don't edit `main.go` / `internal/cmd/*` / `internal/ui/*` / `internal/generate/*`.** The exit
  mapping in `handleGenError` is correct and its rescue branch is byte-frozen (§18.3); `default_action.go`
  is being edited by the parallel S2. S3's only files are under `internal/exitcode/`.
- ❌ **Don't add the new test as a direct `*ExitError`** — that case already exists. The gap is a
  `*ExitError` reached THROUGH `fmt.Errorf("%w")`; use the wrap form to actually exercise `errors.As`.
- ❌ **Don't "fix" the transient `pkg/stagecoach: undefined: io` build error** if you see it — that's S2's
  in-flight work, not an S3 bug. Scope validation to `./internal/exitcode/` + `./internal/cmd/`.
