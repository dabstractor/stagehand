# Research: Busy Exit Code (P1.M1.T2.S1)

> **Purpose:** Pin the exact, verified edit for adding `exitcode.Busy = 5`, checked against the live
> codebase on 2026-07-03 (baseline `go test ./internal/exitcode/` GREEN; `Busy` absent repo-wide) and
> the architecture `integration_seams.md §2`. This is a 3-site surgical change: one constant + its
> comment, one test row (+ a value-pin test), and one docs table row + note.

---

## 1. Environment & Baseline

| Item | Value / Evidence |
|---|---|
| Module | `github.com/dustin/stagecoach`, `go 1.22` |
| Edit targets | `internal/exitcode/exitcode.go`, `internal/exitcode/exitcode_test.go`, `docs/cli.md` |
| Baseline | `go test ./internal/exitcode/` → **ok (cached)** |
| `Busy` present? | **ABSENT** repo-wide — `grep -rn Busy internal/exitcode/ docs/cli.md` → none (confirmed). Genuine addition, not a verify-and-confirm. |
| S1 boundary | `internal/lock/*` + docs/how-it-works.md + docs/configuration.md. S1 EXPLICITLY does NOT touch `internal/exitcode` or `docs/cli.md` (those are THIS task). No conflict. |

---

## 2. The exitcode.go const block — exact current → target

### Current (exitcode.go:22-30)
```go
const (
	Success         = 0   // commit created, or dry-run message printed
	Error           = 1   // general error (generation failed, parse failed, agent missing, CAS, usage/flag)
	NothingToCommit = 2   // clean tree after auto-stage, or nothing staged with --no-auto-stage
	Rescue          = 3   // snapshot taken, commit not created — manual recovery printed
	Timeout         = 124 // generation exceeded --timeout (mirrors GNU `timeout`)
)
```

### Target (add `Busy = 5` AFTER Rescue, BEFORE Timeout)
```go
const (
	Success         = 0   // commit created, or dry-run message printed
	Error           = 1   // general error (generation failed, parse failed, agent missing, CAS, usage/flag)
	NothingToCommit = 2   // clean tree after auto-stage, or nothing staged with --no-auto-stage
	Rescue          = 3   // snapshot taken, commit not created — manual recovery printed
	Busy            = 5   // another stagecoach run holds the per-repo lock; retry after it finishes (FR52 §18.5)
	Timeout         = 124 // generation exceeded --timeout (mirrors GNU `timeout`)
)
```

**Value rationale (integration_seams §2):** `5` is in the 1–7 range, distinct from all existing codes
(0/1/2/3/124), does not collide with sysexits.h conventions that might confuse scripts. `4` is deliberately
reserved for future use → `5` is the chosen value. Keeping `Timeout = 124` LAST mirrors GNU `timeout`
(contract requirement).

**Comment-style decision:** the existing block uses INLINE comments that do NOT repeat the constant name
(`Rescue = 3 // snapshot taken…`, not `Rescue = 3 // Rescue — snapshot taken…`). The contract's prose
("Busy — another stagecoach run…") narrates the name; the actual comment should DROP the redundant
`Busy — ` prefix to match the 5 sibling constants. Final comment text:
`// another stagecoach run holds the per-repo lock; retry after it finishes (FR52 §18.5)`.
(gofmt re-aligns the `=` and `//` columns automatically after the insert — run `gofmt -w`.)

---

## 3. For() — NO change (the short-circuit already covers it)

The contract is explicit: "No change to For() — Busy is only returned via explicit
`exitcode.New(exitcode.Busy, nil)`." Reading the live `For()` (exitcode.go:52) confirms why:

```go
func For(err error) int {
	if err == nil {
		return Success
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code   // ← an ExitError{Code: Busy} returns 5 HERE; no new arm needed
	}
	... generate-domain sentinel arms (NothingToCommit/EmptyMessage/Timeout/Rescue/CAS) ...
}
```

`exitcode.New(exitcode.Busy, nil)` builds `*ExitError{Code: 5, Err: nil}`; `For()`'s `errors.As` arm
returns `ee.Code` (= 5). **No `errors.Is` arm, no Busy-specific branch is required.** Adding one would be
dead code (Busy never flows through any sentinel path — it is ONLY constructed via explicit `New`).

---

## 4. New(code, nil) — the silent non-zero exit (already supported)

`ExitError.Err` may be `nil` for a SILENT non-zero exit. This is the contention path: `runDefault` prints
the contention message ONCE (holder pid/host/repo), then returns `exitcode.New(exitcode.Busy, nil)` so
`main.go` does NOT double-print (an `ExitError{Err:nil}.Error()` returns `""`). The existing
`TestExitError_NilErr` proves `New(Error, nil)` works and `For()` returns the code; the same machinery
covers `New(Busy, nil)`. **No change to `ExitError`/`New`/`Error()` is needed.**

---

## 5. Test additions — mirror the existing taxonomy

`exitcode_test.go` is `package exitcode` (white-box). It has two tests:
- `TestFor` — table-driven (`tests []struct{name; err; want}` + `t.Run`). **ADD one row.**
- `TestExitError_NilErr` — proves `New(Error, nil)`. (Unchanged; the TestFor row covers Busy's silent path.)

### Row to add to `TestFor`'s table
```go
{"explicit ExitError Busy (silent, nil err) → 5", New(Busy, nil), Busy},
```
This is the single most important assertion: it proves `New(Busy, nil)` → `For` returns `Busy` (5) via the
`*ExitError` short-circuit, which is exactly how `runDefault` (S2) will produce the Busy exit. It also
exercises the `Err:nil` silent path for the new code.

### New value-pin test (guard against accidental renumbering)
```go
func TestBusyCodeValue(t *testing.T) {
	// Busy is 5 — distinct from 0/1/2/3/124; 4 is reserved (integration_seams §2).
	// Scripts and CI match on this exact value; pin it.
	if Busy != 5 {
		t.Errorf("Busy = %d, want 5", Busy)
	}
}
```
This pins the exact value that external scripts rely on (the PRD §18.5 / §15.4 "busy, retry" contract).
A constant exists precisely so callers write `exitcode.Busy`, but the integer value is a public contract
for shell scripts — pinning it in a test catches a future renumbering that would silently break consumers.

---

## 6. docs/cli.md — the Exit codes table + note

### Current (docs/cli.md:366-376)
```
## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success (commit created, or dry-run message printed). |
| `1` | General error (...). |
| `2` | Nothing to commit (...). |
| `3` | Rescue condition (...). |
| `124` | Timeout (generation exceeded `--timeout`). |

Exit codes mirror the constants in `internal/exitcode/exitcode.go`. A timeout is reported as `124` ...
```

### Target — (a) one table row (insert BETWEEN `3` and `124`, ascending numeric order matching the const block)
```
| `5` | Busy — another stagecoach run holds the per-repo lock; retry after it finishes. |
```

### Target — (b) a note paragraph AFTER the existing "Exit codes mirror…" paragraph (line 376)
```
Code `5` (Busy) is distinct from the commit-failure codes so scripts can tell "busy, retry" from "failed."
Contention on the per-repo run lock (FR52) has two behaviors: if a contending run's staged changes are
already covered by the in-progress run's published snapshot, it exits **0** ("nothing to do — an in-progress
run already covers your staged changes"); if genuinely new work is staged, it exits **5** with the holder's
pid/host and leaves the new changes staged for a re-run. Stagecoach never force-breaks the lock.
```
This is the contract's "note below the table explaining the two contention behaviors (no-op exit 0 and busy
exit 5) and that this is distinct from commit-failure codes."

---

## 7. Decisions Log

| # | Question | Decision | Rationale |
|---|---|---|---|
| D1 | Value for Busy? | **5** | Free (no collision with 0/1/2/3/124); `4` reserved (integration_seams §2); in the low 1–7 range scripts expect. |
| D2 | Placement in const block? | After `Rescue=3`, before `Timeout=124`. | Contract mandate: keep `Timeout=124` LAST (mirrors GNU timeout). Ascending order otherwise. |
| D3 | Change For()? | **NO.** | The `*ExitError` short-circuit (`errors.As → ee.Code`) already returns 5 for `New(Busy,nil)`. Busy never flows through a sentinel arm. Adding an arm = dead code. |
| D4 | Change ExitError/New? | **NO.** | `New(code, nil)` + `Err:nil → Error()==""` already support the silent non-zero exit (proven by TestExitError_NilErr). |
| D5 | Comment text? | Inline, no redundant "Busy — " prefix: `// another stagecoach run holds the per-repo lock; retry after it finishes (FR52 §18.5)`. | Matches the 5 sibling constants' inline-comment style (none repeat the name). Contract's "Busy — " was prose narration. |
| D6 | Test approach? | One `TestFor` row (`New(Busy,nil)→Busy`) + one `TestBusyCodeValue` (==5). | The row proves the short-circuit path S2 relies on; the pin guards the public integer contract for scripts. |
| D7 | Docs row position? | Between `3` and `124`. | Ascending numeric order, matching the const block (Busy before Timeout). |
| D8 | Scope vs S1/S2/S3? | ONLY exitcode.go + exitcode_test.go + docs/cli.md. | S1 = lock pkg + 2 docs; S2 (sibling) = wiring/contention-message/E2E; S3 = README. This task is the constant + the cli.md row only. |
