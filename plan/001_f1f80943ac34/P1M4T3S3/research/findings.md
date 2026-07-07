# P1.M4.T3.S3 — Research Findings

## TL;DR (the decisive finding)

**The §15.4 exit-code system described in the S3 work-item contract ALREADY EXISTS, is fully wired,
and its tests are green.** It shipped in **P1.M4.T1.S1** ("Root command, global flags, and custom
exit-code wiring (cobra)"). The P1.M4.T3.S1 PRP confirms the scope explicitly: *"Exit codes /
ExitError — already shipped P1.M4.T1.S1 (`internal/exitcode`); S3 refine-only."*

➡ **Therefore S3 is NOT a creation task. It is a VERIFICATION + HARDENING + COMPLETENESS-AUDIT task.**
A PRP that directs the agent to "create `internal/exitcode/exitcode.go`" would overwrite a deployed,
tested file and regress ~40 call sites + `cmd/stagecoach/main.go` + every CLI test (root/providers/
config/default_action). The honest PRP must confine changes to small, additive refinements inside
`internal/exitcode/` and treat the rest as a read-only audit.

---

## 1. Existing implementation vs. the S3 contract (line-by-line)

File: `internal/exitcode/exitcode.go` (READ — already correct).

| Contract requirement (P1.M4.T3.S3) | Existing reality | Status |
|---|---|---|
| Location `internal/ui/exitcode.go` **OR** `internal/exitcode/exitcode.go` | `internal/exitcode/exitcode.go` (the 2nd option) | ✅ matches |
| Constants `ExitSuccess=0, ExitError=1, ExitNothingToCommit=2, ExitRescue=3, ExitTimeout=124` | `Success=0, Error=1, NothingToCommit=2, Rescue=3, Timeout=124` — **values identical; names omit the `Exit` prefix** | ⚠ see §2 |
| `ExitError struct{Code int; Err error}` with `Error()+Unwrap()` | Exactly that, plus `New(code, err)` constructor | ✅ matches |
| `For(err error) int` using `errors.As`, defaulting to 1 | `For()` uses `errors.As(err, &ee)` → `ee.Code`, default `Error`(1) | ✅ matches (+ richer, see §3) |
| Unit tests for error→code mapping | `internal/exitcode/exitcode_test.go` — 12-case table + a nil-Err test | ✅ mostly; **1 gap — see §4** |
| OUTPUT: consumed by `main()` (P1.M4.T1.S1) | `cmd/stagecoach/main.go:25` calls `exitcode.For(err)`; `os.Exit(code)` | ✅ wired |

**Conclusion:** every LOAD-BEARING contract requirement is satisfied. The only deltas are (a) the
constant-name prefix (intentional, see §2) and (b) one missing test case (a wrapped `*ExitError`,
see §4).

## 2. Naming: `Exit` prefix (contract) vs no prefix (existing)

The contract text reads `ExitSuccess=0, ExitError=1, …`. The existing package uses `Success`,
`Error`, `NothingToCommit`, `Rescue`, `Timeout` (no `Exit` prefix). Reasoning to KEEP existing names:
- They are referenced at **~40 sites** (`exitcode.Success`, `exitcode.New(exitcode.Error, …)`,
  `exitcode.NothingToCommit`, `exitcode.Rescue`, `exitcode.Timeout`) across `internal/cmd/{root,
  default_action,providers,config}.go` + 5 test files.
- Within the `exitcode` package, `exitcode.Success` reads better than `exitcode.ExitSuccess`
  (the package name already supplies the "Exit" context). This is idiomatic Go.
- Renaming = pure churn + merge-conflict risk with the parallel S2 work (S2 edits default_action.go,
  which is saturated with `exitcode.X` references).

**Decision (D1):** KEEP the existing names. Do NOT rename. Do NOT add aliases. Treat the contract's
`Exit`-prefixed names as illustrative of the *values*, not a mandate on the *identifiers*.

## 3. `For()` is a SUPERSET of the contract (and that's correct)

The contract says `For(err)` uses `errors.As` to extract the code, defaulting to 1. The existing
`For()` does that AND additionally maps the **generate-domain error taxonomy** (shipped with
P1.M3.T4.S2 / P1.M3.T3.S1) to §15.4 codes:

```go
func For(err error) int {
    if err == nil { return Success }
    var ee *ExitError
    if errors.As(err, &ee) { return ee.Code }      // ← the contract requirement
    if errors.Is(err, generate.ErrNothingToCommit) { return NothingToCommit } // 2
    if errors.Is(err, generate.ErrTimeout)         { return Timeout }          // 124 (before rescue!)
    if errors.Is(err, generate.ErrRescue)          { return Rescue }           // 3
    if errors.Is(err, context.DeadlineExceeded)    { return Timeout }          // 124
    if errors.Is(err, generate.ErrCASFailed)       { return Error }            // 1
    return Error                                                                              // default 1
}
```

This is **correct and load-bearing** — without it, the CLI's `RunE` functions would have to wrap
every generate error in `exitcode.New(...)` manually. The domain mapping keeps `handleGenError`'s
explicit `exitcode.New` calls minimal. **Do NOT remove or weaken the domain mapping.** (All mapped
sentinels/types confirmed to exist in `internal/generate/generate.go`: `ErrNothingToCommit` L49,
`ErrTimeout` L54, `ErrRescue` L59, `ErrCASFailed`=git re-export L65, `RescueError` L76, `CASError` L101.)

**Ordering subtlety (already correct, do not touch):** `ErrTimeout` is checked BEFORE `ErrRescue`
because a timeout IS a `*RescueError{Kind: ErrTimeout}` — it must map to 124, not 3. There is an
explicit test for this (`RescueError(ErrTimeout) → 124 (timeout before rescue)`).

## 4. The ONE genuine coverage gap (the concrete test-hardening delta)

The existing `exitcode_test.go` table covers direct `*ExitError` and wrapped *sentinels*, but it has
**no case for a `*ExitError` discovered via `errors.As` through a `fmt.Errorf("…: %w", …)` chain.**
The contract's guarantee is precisely "extract the code via `errors.As`" — `errors.As` traverses the
wrap chain, so `For(fmt.Errorf("outer: %w", exitcode.New(7, errors.New("x"))))` MUST yield `7`.
This works today (Go stdlib semantics) but is **untested**. That is the single, well-scoped test to ADD.

Secondary (optional) coverage to consider:
- A wrapped-and-then-ExitError precedence test: `fmt.Errorf("%w", New(NothingToCommit, e))` → 2 (proves
  the explicit-ExitError branch wins over the sentinel branch when both could apply).
- Confirm `errors.Is(New(Error, errors.New("x")), someUnrelatedSentinel)` is false (ExitError Unwrap
  chains to Err only) — defensive, optional.

## 5. Call-site audit (read-only; confirms correctness, no changes needed)

`exitcode.For` is the single consumer in `main.go:25` → `os.Exit(code)`. Every CLI `RunE` returns
errors via `exitcode.New(code, err)` (the explicit path) or lets `generate.*` errors flow through
`handleGenError` → `exitcode.New(...)` / domain-mapped by `For`. Verified sites:

- `internal/cmd/root.go:61,69` → `exitcode.New(exitcode.Error, …)` (getwd, config load)
- `internal/cmd/default_action.go` → 12 `exitcode.New(...)` sites incl. `NothingToCommit`(L67,84,88),
  `handleGenError` rescue→`Rescue`(L150)/timeout→`Timeout`(L152)/CAS→`Error`(L157)/generic→`Error`(L164)
- `internal/cmd/providers.go:85,100,104`, `internal/cmd/config.go:94,97,100,103` → `exitcode.New(exitcode.Error, …)`
- Tests assert `exitcode.For(err) == exitcode.{Success,NothingToCommit,Rescue,Timeout,Error}` in
  `root_test.go`, `default_action_test.go` (5 codes), `providers_test.go`, `config_test.go`.

**No call site returns a raw error that would wrongly default to 1 where another §15.4 code is
intended.** No missing `exitcode.New` wrapping found. Audit result: PASS.

## 6. Scope boundary & parallel coordination

- **S3 edits ONLY `internal/exitcode/exitcode.go` (doc-comment enrichment, optional) and
  `internal/exitcode/exitcode_test.go` (add the wrapped-ExitError case + any optional cases).**
- S3 does NOT touch: `main.go`, `internal/cmd/*`, `internal/generate/*`, `pkg/stagecoach/*`,
  `internal/ui/*`. The exit mapping in `handleGenError` is **frozen** (rescue §18.3 is byte-frozen;
  the CAS/nothing-to-commit/generic mapping is correct and tested). Editing it risks conflict with the
  **parallel P1.M4.T3.S2** (verbose), which edits `default_action.go` (one line) + executor/generate/
  stagecoach. Zero file overlap with S3.
- **Known transient build state (NOT an S3 bug):** while S2 is in-flight, `go build ./...` may report
  `pkg/stagecoach/stagecoach.go: undefined: io` (S2 adding `Options.Verbose io.Writer` before the import
  lands, or mid-edit). S3's validation uses `go test ./internal/exitcode/` (self-contained) and
  `go vet ./internal/exitcode/`; the full-suite gate is run only after S2 merges. Do not "fix" pkg/stagecoach.

## 7. Confidence

The system is complete and correct. S3 is a low-risk, 1-point polish: add ~1-2 test cases, optionally
enrich doc comments, and produce the audit/verification. **One-pass success likelihood: 9/10** (the
only risk is an agent misreading the contract as "create" and rewriting the file — the PRP forbids that
explicitly).
