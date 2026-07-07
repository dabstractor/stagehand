---
name: "P1.M4.T4.S1 — --dry-run flag: COMPLETE the single deferred decoration '(no commit created)' → stderr (PRD FR49 / §9.12, Appendix B.3)"
description: |

  ⚠️ THIS IS NOT A CREATION TASK. `stagecoach --dry-run` is **~95% already shipped** across two COMPLETE
  tasks: the public API `Options.DryRun` + `runPipeline` dry-run branch (P1.M3.T5.S1) and the CLI
  default action's flag registration, `DryRun` pass-through, dry-run success branch, message→stdout,
  exit 0, and the `↳ Generating…` progress line (P1.M4.T1.S2). The default-action author **explicitly
  deferred exactly ONE line** to P1.M4.T4 — proven by the in-source comment at
  `internal/cmd/default_action.go:128`:  `// / "(no commit created)" decorations are P1.M4.T3/T4.`

  P1.M4.T4.S1 is the **COMPLETION gate**: add the `(no commit created)` notice to STDERR in the dry-run
  success path (so stdout stays clean for piping — `stagecoach --dry-run --no-color | tee`), and extend
  the existing `TestRunDefault_DryRun` to assert it is on stderr and NOT on stdout. That is the entire
  deliverable. The implementing agent must NOT recreate the flag, the pass-through, the branch, the
  public-API dry-run mechanics, or the progress line — they all exist.

  CONTRACT (P1.M4.T4.S1, verbatim):
    1. RESEARCH: "PRD FR49 / §9.12. --dry-run runs the full pipeline and prints the resulting message
       but does NOT create the commit or move HEAD. Exit 0. Appendix B.3 shows the dry-run output
       format: message on stdout + '(no commit created)'. The public API already supports DryRun in
       Options (P1.M3.T5.S1)."  ← CONFIRMED present.
    2. INPUT: "GenerateCommit with DryRun=true (P1.M3.T5.S1)."  ← CONFIRMED: default_action.go:118.
    3. LOGIC: "… Print the message to stdout (clean, for piping). Print '(no commit created)' to stderr.
       Exit 0. … commit-tree/update-ref are skipped. Mock: integration test — dry-run produces a message,
       HEAD unchanged."  ← message/stdout/exit-0/HEAD-unchanged ALL done; the ONE gap is the stderr line.
    4. OUTPUT: "Working `stagecoach --dry-run` that previews the message without committing."  ← true after the fix.
    5. DOCS: "none — documented in CLI help (P1.M4.T1.S1) and README (P1.M5.T4)."

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `pkg/stagecoach/stagecoach.go` (`runPipeline` dry-run branch) — P1.M3.T5.S1 (Complete). Its choice
      to skip `WriteTree` + duplicate-check in dry-run is a deliberate, defensible optimization; the
      contract's "snapshot is still taken" prose is NOT observable (the contract's own mock checks only
      "message produced, HEAD unchanged"). Do NOT modify the public API. See Context §"Known Gotchas".
    - `internal/exitcode/` — P1.M4.T3.S3 (in flight, parallel); edits ONLY that dir. Zero overlap.
    - The flag registration (`root.go:89`), the `DryRun: flagDryRun` pass-through, the `↳ Generating…`
      progress (`u.Progress`), `printDryRunMessage`, and exit-0 mapping — all present; leave them.

  DELIVERABLE (bounded to `internal/cmd/default_action.go` + its test):
    EDIT internal/cmd/default_action.go          — add ONE line: `fmt.Fprintln(stderr, "(no commit created)")`
                                                   in the dry-run success branch (between printDryRunMessage
                                                   and `return nil`). Update the branch comment to reflect it
                                                   is now implemented (no longer deferred).
    EDIT internal/cmd/default_action_test.go     — extend `TestRunDefault_DryRun` with 2 assertions: stderr
                                                   CONTAINS "(no commit created)"; stdout does NOT (pipeable).

  SUCCESS: `go test -race ./internal/cmd/ -v` green with the new assertions; `go vet ./internal/cmd/`
  clean; `gofmt -l internal/cmd/` empty; `git status` shows changes ONLY under `internal/cmd/`;
  `make build` succeeds; a manual `./bin/stagecoach --provider stub --dry-run` (in a stub repo) prints
  the message on stdout and "(no commit created)" on stderr with exit 0.

---

## Goal

**Feature Goal**: Complete Stagecoach's `--dry-run` UX (PRD FR49 / §9.12) by adding the one decoration
the default-action author deferred to P1.M4.T4 — the `(no commit created)` notice on **stderr** — so the
Appendix B.3 output format is fully realized while stdout remains a clean, pipeable commit-message stream.

**Deliverable** (2 edits, confined to `internal/cmd/`):
1. EDIT `internal/cmd/default_action.go` — in the `if flagDryRun || res.CommitSHA == ""` success branch,
   add `fmt.Fprintln(stderr, "(no commit created)")` after `printDryRunMessage(stdout, res.Message)` and
   before `return nil`. Plain text (no `↳ ` prefix, no color) — matching Appendix B.3 verbatim. Update the
   branch comment so it no longer says this decoration is deferred.
2. EDIT `internal/cmd/default_action_test.go` — in `TestRunDefault_DryRun`, add two assertions:
   `errBuf.String()` CONTAINS `"(no commit created)"`; `outBuf.String()` does NOT.

**Success Definition**:
- Appendix B.3 dry-run output is fully produced: progress `↳ Generating…` (stderr, already present) →
  message (stdout, already present) → `(no commit created)` (stderr, **the new line**).
- stdout is **message-only** — `stagecoach --dry-run --no-color | tee /tmp/msg.txt` yields a clean message
  (the `(no commit created)` notice never contaminates stdout). Asserted by the (already-present)
  exact-match `stdout == "feat: dry run"` check PLUS the new "stdout must NOT contain" assertion.
- Exit 0 (`return nil`); HEAD unchanged (public API skips commit-tree/update-ref; already proven).
- `go test -race ./internal/cmd/ -v` green; `go vet ./internal/cmd/` clean; `gofmt -l internal/cmd/` empty.
- `git status` shows changes ONLY under `internal/cmd/`.

## User Persona

**Target User**: the scripter/integrator (PRD §7 personas) who previews a commit message before
committing or pipes it onward — `stagecoach --dry-run | git commit -F -`, a lazygit keybind, a CI "show
me what you'd write" step. Also the cautious developer who wants to eyeball the message without moving HEAD.

**Use Case**: `stagecoach --dry-run` → see the generated message → decide. stdout = the message (pipeable);
stderr = human-readable scaffolding (`↳ Generating…`, `(no commit created)`). Exit 0 = "I have a message
for you; nothing was committed."

**User Journey**: user runs `stagecoach --dry-run` → progress line on stderr → message printed to stdout
→ `(no commit created)` confirmation on stderr → shell sees exit 0. Piping `| tee msg.txt` captures ONLY
the message because the notice is on stderr.

**Pain Points Addressed**: ambiguity about whether a commit was actually created (the explicit
`(no commit created)` notice removes all doubt); stdout pollution that would break
`stagecoach --dry-run | git commit -F -` (the notice is correctly routed to stderr).

## Why

- **Closes the P1.M4.T4.S1 contract as written** by adding the single line the contract names
  ("Print '(no commit created)' to stderr") — the only piece of the dry-run UX not yet present.
- **Realizes the Appendix B.3 output format** end-to-end (progress → message → notice), matching the
  PRD's documented terminal session exactly.
- **Protects the pipe use case (§15.5).** `stagecoach --dry-run --no-color | tee` MUST yield a clean
  message. Routing the notice to stderr (not stdout) is what makes that work; the new "stdout must NOT
  contain" assertion locks it as a regression test.
- **Avoids scope creep.** A naive reading of "--dry-run flag" could lead to recreating the flag, the
  pass-through, the public-API dry-run path, or the progress line — all of which already exist and are
  tested. This PRP makes the "one deferred line" reality unambiguous (mirrors the P1.M4.T3.S3
  "verify/don't-recreate" framing).

## What

A two-edit completion of the dry-run output stream separation:

```go
// internal/cmd/default_action.go — the dry-run success branch (ALREADY EXISTS; ADD one line + fix comment):
if flagDryRun || res.CommitSHA == "" {
    // Dry-run (Appendix B.3): stdout = the message ONLY (§15.5 pipe use case). The "↳ Generating…"
    // progress is already on stderr (u.Progress above). "(no commit created)" → STDERR so stdout stays
    // clean for piping (FR49 / P1.M4.T4.S1).
    printDryRunMessage(stdout, res.Message)
    fmt.Fprintln(stderr, "(no commit created)") // Appendix B.3; stderr keeps stdout clean for piping
    return nil                                    // exit 0
}
```

```go
// internal/cmd/default_action_test.go — extend TestRunDefault_DryRun (strings already imported):
// stderr MUST contain "(no commit created)" (Appendix B.3) …
if !strings.Contains(errBuf.String(), "(no commit created)") {
    t.Errorf("stderr = %q, want to contain '(no commit created)'", errBuf.String())
}
// … and stdout MUST NOT (pipeable — §15.5). (The existing exact-match check already implies this;
// this assertion makes the stream-separation contract explicit.)
if strings.Contains(stdout, "(no commit created)") {
    t.Errorf("stdout = %q, must NOT contain '(no commit created)' (pipeable)", stdout)
}
```

NO changes to: `pkg/stagecoach/*` (public API — Complete, frozen), `internal/exitcode/*` (parallel S3),
`internal/ui/*` (Progress/Success/Error/Verbose — Complete), `internal/cmd/root.go` (flag already
registered), `internal/generate/*`, or any other file.

### Success Criteria

- [ ] `stagecoach --dry-run` prints the generated message to **stdout** AND `(no commit created)` to
      **stderr**; exit 0; HEAD unchanged (Appendix B.3).
- [ ] stdout is **message-only** — `stagecoach --dry-run | tee` captures a clean message (the notice is
      never on stdout). Asserted by the existing exact-match check + the new "must NOT contain" check.
- [ ] `TestRunDefault_DryRun` asserts both: stderr CONTAINS `(no commit created)`, stdout does NOT.
- [ ] `go test -race ./internal/cmd/ -v` green; `go vet ./internal/cmd/` clean; `gofmt -l internal/cmd/` empty.
- [ ] `git status` shows changes ONLY under `internal/cmd/`.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can execute this completion task from: the
exact current code of the dry-run branch (quoted below), the Appendix B.3 format (quoted), the existing
test to extend (quoted), the stream-separation rationale (stderr keeps stdout pipeable), and the explicit
"do NOT recreate the flag/pass-through/public-API/progress" guardrails. No signal/generate/provider/UI
internals are required (all frozen / out of scope).

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M4T4S1/research/findings.md
  why: THE decisive doc. §1 contract-vs-reality table (every clause ✅ except the stderr line), §2 the
       exact one-line fix, §3 why NOT to touch the public API (snapshot prose is non-observable + owned by
       Complete P1.M3.T5.S1), §4 the test plan, §5 parallel coordination (S3 = internal/exitcode/ only,
       zero overlap), §6 confidence 9.5/10.
  critical: §1 (don't recreate — it's ~95% done), §2 (the one line), §3 (don't touch pkg/stagecoach).

- file: internal/cmd/default_action.go   (P1.M4.T1.S2 — the file you EDIT; 2-line region only)
  section: `runDefault`'s success branch, ~L124-130:
       `if flagDryRun || res.CommitSHA == "" { ... printDryRunMessage(stdout, res.Message); return nil }`.
       The branch comment at L127-128 currently says `(no commit created)` is "P1.M4.T3/T4" — you are T4;
       implement it and update the comment. Also note L118 already passes `DryRun: flagDryRun` (DONE),
       and `u.Progress(label)` (~L110) already emits `↳ Generating…` to stderr (DONE — Appendix B.3 line 1).
  why: this is where the deliverable lives. ADD `fmt.Fprintln(stderr, "(no commit created)")` between
       `printDryRunMessage(...)` and `return nil`.
  pattern: stream discipline already encoded in the file (stdout = result data; stderr = notices). The
       `fmt.Fprintln(stderr, ...)` idiom is already used for the FR18 auto-stage notice (search `Fprintln(stderr`).
  gotcha: `stdout`/`stderr` are the `cmd.OutOrStdout()`/`cmd.ErrOrStderr()` writers captured at the top of
       runDefault — both are already in scope in the branch; no new variables. `fmt` is already imported.

- file: internal/cmd/default_action_test.go   (P1.M4.T1.S2 — the file you EDIT; extend ONE test)
  section: `func TestRunDefault_DryRun` (~L252-275). It already: sets `rootCmd.SetOut(&outBuf)`,
       `SetErr(&errBuf)`, `SetArgs([]string{"--provider","stub","--dry-run"})`, asserts
       `strings.TrimSpace(outBuf.String()) == "feat: dry run"`, asserts HEAD unchanged, asserts `err==nil`.
  why: ADD the two assertions (stderr contains "(no commit created)"; stdout does not) right after the
       existing stdout check. `strings` is already imported.
  pattern: mirror the existing `t.Errorf("... = %q, want ...", got)` style. The `outBuf`/`errBuf` are
       `bytes.Buffer` already declared in the test.
  gotcha: do NOT fork into a new test — extend the canonical dry-run test. The stub provider emits
       "feat: dry run" (setupStubRepo), so the message is deterministic.

- file: internal/cmd/root.go   (P1.M4.T1.S1 — READ only; the flag is ALREADY registered)
  section: `pf.BoolVar(&flagDryRun, "dry-run", false, "Generate and print the message; do not commit")` (L89);
       `flagDryRun` package var (L40). `runDefault` reads it (default_action.go:118).
  why: confirms the flag + behavioral-var wiring is DONE — do not re-register or rename it.
  gotcha: NO change here. The contract's "DOCS: CLI help (P1.M4.T1.S1)" is satisfied by this very line.

- file: pkg/stagecoach/stagecoach.go   (P1.M3.T5.S1 — READ only; the public API is FROZEN)
  section: `Options.DryRun bool` (L36); `GenerateCommit` dispatch (`!opts.DryRun ...` → CommitStaged, else
       runPipeline) L93-109; `runPipeline` dry-run branch L254-276 (single generate→parse pass, returns
       `Result{CommitSHA:""}`, no WriteTree/CommitTree/UpdateRef).
  why: confirms DryRun=true returns a Result with CommitSHA=="" and HEAD unmoved — which is why the CLI's
       `if flagDryRun || res.CommitSHA == ""` branch fires and why exit 0/HEAD-unchanged already work.
  gotcha: DO NOT EDIT. The contract's "snapshot is still taken (write-tree runs)" prose is NOT observable
       (the contract's own mock checks only "message produced, HEAD unchanged") and runPipeline's choice
       to skip WriteTree is a deliberate, defensible optimization by the owning Complete task (P1.M3.T5.S1).
       Modifying it would destabilize TestGenerateCommit_DryRun / TestGenerateCommit_Timeout.

- file: internal/ui/output.go   (P1.M4.T3.S1 — READ only; the progress line is ALREADY emitted)
  section: `UI.Progress(msg)` → `fmt.Fprintln(u.stderr, "↳ "+msg)`. Called in runDefault as `u.Progress(label)`.
  why: confirms Appendix B.3 line 1 (`↳ Generating…`) is DONE and on stderr. You do NOT need a UI method
       for `(no commit created)` — Appendix B.3 shows it PLAIN (no `↳ ` prefix, no color), so a direct
       `fmt.Fprintln(stderr, "(no commit created)")` is the faithful rendering.
  gotcha: do NOT route the notice through `u.Success`/`u.Progress`/`u.Yellow` — those add a `↳ ` prefix
       and/or color, which Appendix B.3 does NOT show for `(no commit created)`. Plain Fprintln to stderr.

- url: (PRD internal) PRD.md §9.12 / FR49 + Appendix B.3 — the AUTHORITATIVE dry-run spec & output format.
  why: FR49 = "run the full pipeline, print the message, do NOT create the commit or move HEAD, exit 0";
       Appendix B.3 = the exact terminal session (progress → message → blank → `(no commit created)`).
  critical: the contract routes `(no commit created)` to STDERR (quoted in item_description: "Print '(no
       commit created)' to stderr") specifically so stdout stays clean for piping.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                                  # module github.com/dustin/stagecoach ; go 1.22 ; UNCHANGED (no new deps)
internal/cmd/
  root.go                               # P1.M4.T1.S1 — --dry-run flag + flagDryRun var (READ; ALREADY REGISTERED)
  default_action.go                     # P1.M4.T1.S2 — runDefault: DryRun pass-through + dry-run branch (EDIT: +1 line)
  default_action_test.go                # P1.M4.T1.S2 — TestRunDefault_DryRun (EDIT: +2 assertions)
  providers.go / config.go              # P1.M4.T1.S3/S4 — subcommands (READ; unrelated)
pkg/stagecoach/stagecoach.go              # P1.M3.T5.S1 — Options.DryRun + runPipeline dry-run branch (READ; FROZEN)
internal/ui/output.go                   # P1.M4.T3.S1 — UI.Progress (↳ prefix, stderr) (READ; progress already wired)
internal/exitcode/exitcode.go           # P1.M4.T1.S1 — Success=0 doc notes "dry-run message printed" (READ; S3 in flight)
Makefile                                # build / test(-race) / vet / lint / coverage / clean (UNCHANGED)
```

### Desired Codebase tree with files to be added/changed

```bash
internal/cmd/default_action.go          # EDIT — +1 line: fmt.Fprintln(stderr, "(no commit created)"); + comment refresh.
internal/cmd/default_action_test.go     # EDIT — TestRunDefault_DryRun: +2 assertions (stderr has it; stdout doesn't).
# ALL other files UNCHANGED. pkg/stagecoach/*, internal/exitcode/*, internal/ui/*, internal/cmd/root.go untouched.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (DON'T RECREATE): `--dry-run` is ~95% shipped. The flag (root.go:89), the DryRun pass-through
// (default_action.go:118), the dry-run success branch (L126), message→stdout (printDryRunMessage L129/194),
// exit 0 (return nil L130), HEAD-unchanged (runPipeline skips commit-tree/update-ref), and the `↳ Generating…`
// progress (u.Progress ~L110) ALL EXIST and are green-tested (TestRunDefault_DryRun). The ONLY missing piece
// is the `(no commit created)` stderr line — explicitly deferred in the comment at default_action.go:128.
// Overwriting/recreating any of the above regresses ~6 call sites + 2 test files. S1 = add one line + assert it.

// CRITICAL (DON'T TOUCH THE PUBLIC API): pkg/stagecoach.runPipeline SKIPS WriteTree (and duplicate-check) in
// dry-run. The contract prose says "the snapshot is still taken (write-tree runs)" — but (a) it's owned by the
// COMPLETE task P1.M3.T5.S1, (b) it is NOT observable (an orphan tree object is never created; the contract's
// OWN mock checks only "message produced, HEAD unchanged"), (c) it's a deliberate optimization, and (d) modifying
// it would regress TestGenerateCommit_DryRun/TestGenerateCommit_Timeout's pinned error shapes (bare ErrTimeout,
// no *RescueError). Leave it. See research/findings.md §3.

// GOTCHA (plain, not decorated): Appendix B.3 renders `(no commit created)` PLAIN — no `↳ ` prefix, no color.
// Do NOT route it through u.Success/u.Progress/u.Yellow (those add a prefix and/or ANSI). A direct
// `fmt.Fprintln(stderr, "(no commit created)")` is the faithful rendering. (Color would also be fine on stderr
// for piping, but the appendix shows it plain, so match the appendix.)

// GOTCHA (stream separation is the POINT): the contract deliberately puts `(no commit created)` on STDERR so
// `stagecoach --dry-run --no-color | tee /tmp/msg.txt` captures a clean message. The existing exact-match check
// (`strings.TrimSpace(outBuf.String()) == "feat: dry run"`) already guards stdout; the new "stdout must NOT
// contain" assertion makes the contract explicit as a regression test.

// GOTCHA (extend, don't fork): add the assertions to the EXISTING TestRunDefault_DryRun — it is the canonical
// dry-run CLI test (sets outBuf/errBuf, --provider stub --dry-run, asserts stdout + HEAD + err). `strings` is
// already imported. Do not create a parallel test.

// GOTCHA (parallel S3): P1.M4.T3.S3 (exit-code verify/harden) edits ONLY internal/exitcode/. It does NOT touch
// internal/cmd/. Zero overlap. (P1.M4.T3.S2 — the "Verbose: stderr" line in default_action.go — is Complete and
// already in the source at L119; no conflict.) Safe to edit default_action.go now.

// GOTCHA (stdout/stderr writers are already in scope): the dry-run branch already has `stdout` and `stderr`
// (cmd.OutOrStdout()/cmd.ErrOrStderr()) captured at the top of runDefault. `fmt` is already imported. No new
// vars/imports needed.
```

## Implementation Blueprint

### Data models and structure

No new data models. `flagDryRun` (bool, `internal/cmd/root.go:40`) and `stagecoach.Options.DryRun`
(bool, `pkg/stagecoach/stagecoach.go:36`) already exist and flow through to the dry-run branch. This task
adds a single output line + test assertions; no struct/type changes.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY the existing dry-run wiring is present and green (READ + RUN, no edit)
  - RUN: `go test -race ./internal/cmd/ -run TestRunDefault_DryRun -v` → must be green (stdout==message,
      HEAD unchanged, err==nil). Capture output.
  - READ: internal/cmd/default_action.go — confirm (a) L118 `DryRun: flagDryRun`, (b) the success branch
      `if flagDryRun || res.CommitSHA == ""` (~L126), (c) `printDryRunMessage(stdout, res.Message)` (~L129),
      (d) `return nil` (~L130), (e) `u.Progress(label)` (~L110), (f) the comment at ~L128 that DEFERS
      "(no commit created)" to P1.M4.T3/T4 (you are T4).
  - READ: internal/cmd/root.go:89 — confirm `--dry-run` is registered (DONE).
  - GOTCHA: if any of these are MISSING, STOP and report — the "~95% shipped" premise would be wrong. (It is
      not; this is a guardrail.) Do NOT "fix" by recreating — diagnose first.

Task 2: EDIT internal/cmd/default_action.go — add the `(no commit created)` stderr line (THE deliverable)
  - FILE: EDIT internal/cmd/default_action.go. In the success branch:
        if flagDryRun || res.CommitSHA == "" {
            printDryRunMessage(stdout, res.Message)
            return nil // exit 0
        }
    INSERT between the two lines:
        fmt.Fprintln(stderr, "(no commit created)") // Appendix B.3; stderr keeps stdout clean for piping
  - ALSO refresh the branch comment: it currently says the "(no commit created)" decoration is deferred to
    "P1.M4.T3/T4" — update it to state the notice is now implemented (P1.M4.T4.S1) and routed to stderr so
    stdout stays clean (§15.5 pipe use case). Keep the note that `↳ Generating…` progress is on stderr.
  - WHY: this is the ONE gap (research/findings.md §2). Appendix B.3 line 3. Contract clause 3 ("Print '(no
      commit created)' to stderr").
  - GOTCHA: PLAIN text (no `↳ ` prefix, no color) — Appendix B.3 shows it verbatim/plain. Do NOT use
      u.Success/u.Progress/u.Yellow (they add prefix/color). `stdout`, `stderr`, `fmt` all already in scope.

Task 3: EDIT internal/cmd/default_action_test.go — extend TestRunDefault_DryRun with stream assertions
  - FILE: EDIT internal/cmd/default_action_test.go, func TestRunDefault_DryRun. After the existing
      `stdout := strings.TrimSpace(outBuf.String()); if stdout != "feat: dry run" { ... }` block, ADD:
        // Appendix B.3: "(no commit created)" on stderr; stdout stays clean for piping (§15.5).
        if !strings.Contains(errBuf.String(), "(no commit created)") {
            t.Errorf("stderr = %q, want to contain '(no commit created)'", errBuf.String())
        }
        if strings.Contains(stdout, "(no commit created)") {
            t.Errorf("stdout = %q, must NOT contain '(no commit created)' (pipeable)", stdout)
        }
  - WHY: locks the stream-separation contract (the whole point of routing the notice to stderr). `strings`
      is already imported; `errBuf`/`stdout` already declared in the test.
  - GOTCHA: extend the EXISTING test — do not fork. Keep the existing stdout-exact-match + HEAD-unchanged +
      err==nil assertions intact.

Task 4: FINAL VALIDATION (the gate)
  - RUN: `gofmt -w internal/cmd/`; `go vet ./internal/cmd/`; `gofmt -l internal/cmd/` (must be empty).
  - RUN: `go test -race ./internal/cmd/ -v` → green INCLUDING the new assertions in TestRunDefault_DryRun.
  - RUN: `go test -race ./internal/cmd/ -run TestRunDefault_DryRun -v` → confirm the new assertions ran.
  - RUN: `make build` → succeeds (proves the one-line edit compiles).
  - RUN: `git status` → changes ONLY under internal/cmd/.
  - (optional smoke) In a scratch stub repo: `./bin/stagecoach --provider stub --dry-run` → stdout = message,
      stderr contains "(no commit created)", exit 0, HEAD unchanged. (Requires the stub provider; the unit
      test already proves this deterministically.)
```

### Implementation Patterns & Key Details

```go
// The ONE deliverable line — mirror the file's existing stream discipline (stdout = data; stderr = notices):
// (The FR18 auto-stage notice in this same file already uses fmt.Fprintln(stderr, ...) — same idiom.)
printDryRunMessage(stdout, res.Message)
fmt.Fprintln(stderr, "(no commit created)") // Appendix B.3; stderr keeps stdout clean for piping
return nil                                   // exit 0

// The test assertions — mirror the existing t.Errorf("... = %q, want ...", got) style:
if !strings.Contains(errBuf.String(), "(no commit created)") {
    t.Errorf("stderr = %q, want to contain '(no commit created)'", errBuf.String())
}
if strings.Contains(stdout, "(no commit created)") {
    t.Errorf("stdout = %q, must NOT contain '(no commit created)' (pipeable)", stdout)
}

// What NOT to do (the guardrails):
//   ✗ recreate the flag / pass-through / branch / public-API dry-run path / progress line (all exist)
//   ✗ modify pkg/stagecoach/runPipeline (Complete task P1.M3.T5.S1; non-observable snapshot prose)
//   ✗ route the notice through u.Success/u.Progress/u.Yellow (adds ↳ prefix / color; appendix shows it plain)
//   ✗ put "(no commit created)" on stdout (breaks `stagecoach --dry-run | tee`)
//   ✗ fork a new test instead of extending TestRunDefault_DryRun
```

### Integration Points

```yaml
CLI DEFAULT ACTION (PRD §15.1):
  - the dry-run success branch in runDefault (internal/cmd/default_action.go). The notice is appended to the
    existing stderr stream used by u.Progress (Appendix B.3 line 1) and the FR18 auto-stage notice. No new
    writer, no new stream.

PUBLIC API (frozen — read-only dependency):
  - pkg/stagecoach.GenerateCommit(opts with DryRun:true) returns Result{CommitSHA:""} on success → the CLI's
    `if flagDryRun || res.CommitSHA == ""` branch fires → the message is printed + (now) the stderr notice.
    P1.M4.T4.S1 does NOT change the public API; it only consumes Result.Message.

EXIT CODE (frozen — read-only dependency):
  - the dry-run path returns nil → main's exitcode.For(nil) == exitcode.Success(0). The Success doc already
    says "commit created, or dry-run message printed" (internal/exitcode/exitcode.go:23). No change needed.

PARALLEL COORDINATION (P1.M4.T3.S3 — in flight):
  - S3 edits ONLY internal/exitcode/. P1.M4.T4.S1 edits ONLY internal/cmd/. Zero overlap. The full
    `go test -race ./...` gate is safe to run after both merge.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After the edit to internal/cmd/default_action.go — fix before proceeding.
gofmt -w internal/cmd/
go vet ./internal/cmd/
gofmt -l internal/cmd/   # must be empty

# Expected: zero errors. gofmt -l empty for internal/cmd/.
```

### Level 2: Unit Tests (the core deliverable)

```bash
# The CLI suite — must be green INCLUDING the new dry-run assertions.
go test -race ./internal/cmd/ -v
# Expected: all green. Confirm TestRunDefault_DryRun runs and passes BOTH new assertions:
#   stderr contains "(no commit created)" AND stdout does not.

# Targeted run (fast feedback loop):
go test -race ./internal/cmd/ -run TestRunDefault_DryRun -v
# Expected: PASS. stdout == "feat: dry run" (message only); stderr contains "(no commit created)";
# HEAD unchanged; err == nil.
```

### Level 3: Integration / Smoke (System Validation)

```bash
# Build sanity (the one-line edit compiles into the binary).
make build
# Expected: ./bin/stagecoach produced; exit 0.

# (optional, requires a stub-provider repo) End-to-end dry-run in a scratch repo:
cd /tmp && rm -rf dryrun-smoke && git init dryrun-smoke && cd dryrun-smoke &&
  git config user.email t@t.co && git config user.name t &&
  # configure the stub provider per internal/stubtest, stage a file, then:
  /home/dustin/projects/stagecoach/bin/stagecoach --provider stub --dry-run --no-color >msg.txt 2>notice.txt
echo "rc=$?"                                         # expect rc=0
cat msg.txt                                          # expect: the generated message ONLY (no notice)
grep -q "(no commit created)" notice.txt && echo OK  # expect: OK (notice on stderr)
git log --format=%s -n1                              # expect: the ORIGINAL HEAD subject (unchanged)

# Expected: stdout (msg.txt) = message only; stderr (notice.txt) contains "(no commit created)";
# exit 0; HEAD unchanged. (The unit test already proves this deterministically — this is a confidence check.)
```

### Level 4: Regression & Audit (confidence, no file change)

```bash
# Whole-tree build + test (safe after the parallel S3 merges; scoped to ./internal/cmd/ if S3 is mid-edit).
go build ./...
go test -race ./internal/cmd/ -v   # zero behavioral regression (only +1 output line + +2 assertions)

# Audit: confirm the notice is on stderr, never stdout (grep).
grep -n "no commit created" internal/cmd/default_action.go   # exactly ONE occurrence, in an Fprintln(stderr, ...)
grep -n "no commit created" internal/cmd/default_action_test.go  # the two new assertions

# Expected: no RunE mis-routes the notice to stdout; the only production occurrence writes to stderr.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l internal/cmd/` empty; `go vet ./internal/cmd/` clean.
- [ ] Level 2: `go test -race ./internal/cmd/ -v` green, INCLUDING the new TestRunDefault_DryRun assertions.
- [ ] Level 3: `make build` succeeds.
- [ ] Level 4: `go build ./...` succeeds; `git status` shows changes ONLY under `internal/cmd/`.
- [ ] No new `go.mod` dependencies (stdlib `fmt`/`strings` only).

### Feature Validation

- [ ] `stagecoach --dry-run` prints the message to **stdout** AND `(no commit created)` to **stderr** (Appendix B.3).
- [ ] stdout is **message-only** (pipeable): `--dry-run | tee` captures a clean message; asserted by exact-match
      + the new "stdout must NOT contain" check.
- [ ] Exit 0; HEAD unchanged (both already true; the edit doesn't touch them).
- [ ] `TestRunDefault_DryRun` asserts: stderr CONTAINS `(no commit created)`, stdout does NOT.

### Code Quality Validation

- [ ] NO recreation of the flag / pass-through / branch / public-API dry-run path / progress line (all exist).
- [ ] NO modification of `pkg/stagecoach/*` (frozen Complete task) or `internal/exitcode/*` (parallel S3).
- [ ] The notice is PLAIN (no `↳ ` prefix, no color), matching Appendix B.3 verbatim.
- [ ] File placement unchanged; `git status` shows changes ONLY under `internal/cmd/`.
- [ ] New assertions mirror the existing `t.Errorf("... = %q, want ...", got)` style.

### Documentation & Deployment

- [ ] Branch comment in default_action.go updated: "(no commit created)" is no longer "deferred" — it is
      implemented (P1.M4.T4.S1) and routed to stderr so stdout stays clean (§15.5).
- [ ] No new env vars / config keys / CLI flags (the `--dry-run` flag predates this task).
- [ ] Implementation summary records: the one-line addition, the two assertions, and the §3 decision to leave
      the public-API dry-run mechanics untouched.

---

## Anti-Patterns to Avoid

- ❌ **Don't recreate the dry-run feature.** It is ~95% shipped (flag, pass-through, branch, message→stdout,
  exit 0, HEAD-unchanged, progress line — all present and green-tested). P1.M4.T4.S1 adds ONE deferred line.
- ❌ **Don't modify `pkg/stagecoach/runPipeline`** to run WriteTree/duplicate-check in dry-run. It's owned by
  the COMPLETE task P1.M3.T5.S1, the contract's "write-tree runs" prose is non-observable, the contract's own
  mock checks only "message produced, HEAD unchanged", and changing it would regress the pinned dry-run error
  shapes in TestGenerateCommit_DryRun/TestGenerateCommit_Timeout. Leave it (research/findings.md §3).
- ❌ **Don't route `(no commit created)` through `u.Success`/`u.Progress`/`u.Yellow`.** Those add a `↳ ` prefix
  and/or ANSI color; Appendix B.3 shows the notice PLAIN. Use a direct `fmt.Fprintln(stderr, "(no commit created)")`.
- ❌ **Don't put the notice on stdout.** That breaks `stagecoach --dry-run --no-color | tee /tmp/msg.txt` (§15.5).
  The whole point of the task is stream separation: message → stdout, notice → stderr.
- ❌ **Don't fork a new test.** Extend the canonical `TestRunDefault_DryRun` (it already sets up outBuf/errBuf,
  `--provider stub --dry-run`, and the message/HEAD/err assertions). `strings` is already imported.
- ❌ **Don't touch `internal/exitcode/*`.** The parallel P1.M4.T3.S3 owns that dir; Success=0 already documents
  "dry-run message printed". Zero overlap is required.
