name: "P1.M2.T5.S1 — Fix --verbose payload-size line for positional/flag-delivery providers (Issue 5)"
description: >
  Internal diagnostics-correctness fix in the provider execution pipeline. `executor.go` reports the
  payload size to `--verbose` via `vb.VerbosePayload(len(spec.Stdin))`, but `spec.Stdin` carries the
  payload ONLY for `stdin`-delivery providers. For `positional` (cursor) and `flag` (user manifests)
  delivery the payload is appended to `spec.Args` and `spec.Stdin == ""`, so the executor calls
  `VerbosePayload(0)`, which hits the `bytes <= 0` no-op guard and emits nothing. PRD §9.13 FR50's
  stated purpose for this line — "expose whether the token-limit gate actually ran" — is therefore
  defeated for those providers. The fix records the payload size as a delivery-agnostic field on
  `CmdSpec` at the one place that knows the delivery mode (the `Render`/`RenderMultiTurn` delivery
  switch), and has the executor report that field instead of `len(spec.Stdin)`. No change to verbose
  output FORMAT, no config/API/user-facing surface change, no docs change.

---

## Goal

**Feature Goal**: Make `--verbose` emit the `DEBUG: payload: <bytes> bytes (~<tokens> tokens est)` line
for EVERY provider delivery mode (stdin, positional, flag), not just stdin. The line must reflect the
actual delivered payload size regardless of how the payload reaches the agent (piped stdin vs trailing
positional arg vs `prompt_flag` arg).

**Deliverable** (Go, internal only — no new files):
1. Add a `PayloadBytes int` field to `CmdSpec` (`internal/provider/render.go`), documented as the
   delivery-agnostic payload size set by the renderers.
2. Set `spec.PayloadBytes = len(payload)` in BOTH delivery switches — `Manifest.Render`
   (`internal/provider/render.go` ~L163–178) and `Manifest.RenderMultiTurn` (~L272–282) — for all three
   cases (stdin/positional/flag).
3. Change the executor's call (`internal/provider/executor.go`) from `vb.VerbosePayload(len(spec.Stdin))`
   to `vb.VerbosePayload(spec.PayloadBytes)`.
4. Tests: a render-side test proving `PayloadBytes == len(payload)` for each delivery mode (incl.
   multi-turn), and an executor-side test proving the `DEBUG: payload:` line is emitted for a
   positional-delivery spec (the exact regression this fix exists for).

**Success Definition** (every command run from repo root):
- `go build ./...` succeeds; `go test -race ./...` is green; `make coverage-gate` passes
  (`internal/provider` stays ≥85%, PRD §20.3); `make lint` is clean.
- `grep -n "VerbosePayload" internal/provider/executor.go` shows exactly ONE call and it reads
  `spec.PayloadBytes` (NOT `len(spec.Stdin)`).
- `grep -n "spec.PayloadBytes = len(payload)" internal/provider/render.go` returns TWO hits (Render +
  RenderMultiTurn).
- `grep -n "PayloadBytes" internal/provider/render.go` shows the field declaration AND the doc comment.
- New tests `go test ./internal/provider/ -run 'PayloadBytes|Positional|FlagDelivery|Verbose' -v` pass
  and prove the line now appears for positional/flag.

## User Persona (if applicable)

**Target User**: A developer running `stagecoach --verbose` against a `positional`/`flag`-delivery
provider (e.g. a custom manifest, or cursor) who is trying to diagnose whether the **token-limit gate**
(§9.4 FR3d/FR3i) truncated/skipped the payload — e.g. a silently-misconfigured `token_limit` in the
wrong TOML section looks identical to a working one without this line.

**Use Case**: "My commit message came out wrong / the model saw a truncated diff — did the token-limit
gate fire? How big was the payload I actually shipped?" Today this is unanswerable for positional/flag
providers (no line printed). After the fix the `DEBUG: payload:` line appears for them too.

**User Journey**: `stagecoach --verbose --provider cursor ...` → scan stderr → see
`DEBUG: payload: 12345 bytes (~3086 tokens est)` → compare against configured `token_limit` → decide.

**Pain Points Addressed**: A diagnostics line that silently vanishes for a whole class of providers,
making FR50's contract non-uniform and the token-limit gate opaque for cursor / custom flag providers.

## Why

- **Correctness of FR50**: PRD §9.13 FR50 promises the payload-size line for *every* provider
  invocation. The implementation only honors it for `stdin` delivery. This makes the diagnostics
  surface non-uniform and directly undermines FR50's stated rationale ("expose whether the token-limit
  gate actually ran").
- **Cheap, surgical, zero behavioral risk**: the audit in `research/findings.md §6–7` proves every
  production `CmdSpec` construction site is correct under the change (all `Execute` callers route through
  `Render`/`RenderMultiTurn`; the one direct constructor, `internal/cmd/models.go:151`, uses `Stdin:""`
  for a `--list-models` probe and correctly wants no payload line — `PayloadBytes` stays 0 → no-op,
  unchanged). So the fix cannot regress any existing path.
- **No surface area churn**: verbose output FORMAT is unchanged (same `"DEBUG: payload: …"` line); no
  config key, flag, env var, or doc is added or altered. It is purely making an existing, documented
  diagnostic appear where it was missing.

## What

`--verbose` prints the payload-size line for all three provider delivery modes. Concretely, the
`CmdSpec` produced by `Manifest.Render`/`Manifest.RenderMultiTurn` will carry the delivered payload's
byte length in a new `PayloadBytes int` field (set for stdin, positional, AND flag), and the executor
will log that field. The line is emitted by the same `VerbosePayload` function, in the same format,
at the same point in the pipeline — only the set of providers that emit it grows from "stdin only" to
"all delivery modes".

### Success Criteria

- [ ] `CmdSpec` has a documented `PayloadBytes int` field.
- [ ] `Manifest.Render` sets `spec.PayloadBytes = len(payload)` for stdin, positional, AND flag cases.
- [ ] `Manifest.RenderMultiTurn` sets `spec.PayloadBytes = len(payload)` for stdin, positional, AND flag.
- [ ] `executor.go` calls `vb.VerbosePayload(spec.PayloadBytes)` (the ONLY `VerbosePayload` call site).
- [ ] For a `stdin`-delivery spec, `PayloadBytes == len(spec.Stdin)` (no behavior change for pi/claude/…).
- [ ] For a `positional`-delivery spec, `--verbose` now emits the `DEBUG: payload:` line (previously absent).
- [ ] For a `flag`-delivery spec, `--verbose` now emits the `DEBUG: payload:` line (previously absent).
- [ ] `internal/cmd/models.go` list-models path still emits NO payload line (no payload; `PayloadBytes==0`).
- [ ] `go build ./...`, `go test -race ./...`, `make coverage-gate`, and `make lint` all pass.

## All Needed Context

### Context Completeness Check

_Yes._ The exact bug location (single call site), the exact struct to extend, the exact two switches
to edit (with their current code quoted), the exact executor line to change, the verified non-regression
audit of every `CmdSpec` construction site, and the exact existing test functions to follow are all
named below with line numbers. An implementer who has never seen this repo can apply the edits and
verify them with the copy-pasteable commands.

### Documentation & References

```yaml
# MUST READ — the contract & ground truth
- file: PRD.md
  why: "§9.13 FR50 — promises the payload-size line (byte count + chars/4 token estimate) for every
        provider invocation; §19 — verbose logs SIZE only, NEVER stdin contents (security invariant)."
  critical: "FR50's rationale is 'expose whether the token-limit gate ran'. This fix makes the line
    actually appear for positional/flag delivery so that rationale holds. Do NOT log contents (§19)."

- file: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/prd_snapshot.md
  why: "Snapshot of the PRD section for this issue (h2.3 / h3.4 = Issue 5)."
  section: "h3.4 Issue 5"

- file: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/architecture/research_provider_verbose.md
  why: "Pre-existing architecture research that CONFIRMED this bug with exact line numbers and proposed
        the exact fix adopted here. Issue 5 section."
  critical: "It states CmdSpec 'intentionally does NOT carry the delivery mode' — adding PayloadBytes
    (SIZE, not mode) does not violate that; but the CmdSpec doc comment must be EXTENDED to say so."

- file: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M2T5S1/research/findings.md
  why: "THIS task's own research: the full non-regression audit of all CmdSpec construction sites
        (§6) and Execute callers (§7), plus the test patterns (§8). Read it first."

# The code to edit — current state captured at research time
- file: internal/provider/render.go
  why: "Owns CmdSpec (L22–29) and BOTH delivery switches: Manifest.Render (~L163–178) and
        Manifest.RenderMultiTurn (~L272–282). Extend CmdSpec + set PayloadBytes in both switches."
  pattern: "Delivery switch shape (identical in both methods):
        switch *r.PromptDelivery {
          case \"stdin\":      spec.Stdin = payload
          case \"positional\": spec.Args = append(spec.Args, payload)
          case \"flag\":       spec.Args = append(spec.Args, *r.PromptFlag, payload)
        }
    The `payload` local is in scope right at the switch — set spec.PayloadBytes = len(payload) there."
  gotcha: "CmdSpec's doc comment says it 'intentionally does NOT carry the delivery mode'. ADD a line
    explaining PayloadBytes is the SIZE (delivery-agnostic), set by the renderers, so a future reader
    doesn't delete it as redundant with Stdin."

- file: internal/provider/executor.go
  why: "Owns the single VerbosePayload call site: `vb.VerbosePayload(len(spec.Stdin))` (the line right
        after `vb.VerboseCommand(...)`). Change the arg to `spec.PayloadBytes`."
  pattern: "The comment on that line ('size only (never contents) — exposes whether the token-limit
        gate ran') stays valid and should be preserved/kept accurate."
  gotcha: "Do NOT add a `len(spec.Stdin)` fallback (e.g. `if spec.PayloadBytes>0 {...} else {len(spec.Stdin)}`)
    — research §6 proves every site is correct with the direct read, and a fallback would re-mask the
    exact bug being fixed."

- file: internal/provider/verbose.go   # actually internal/ui/verbose.go
  why: "Owns VerbosePayload(bytes int) with the `bytes <= 0` no-op guard and the format string
        'DEBUG: payload: %d bytes (~%d tokens est)\\n'."
  critical: "Do NOT change its signature, guard, or format. The fix is purely about WHO calls it with
    WHAT value (executor passes PayloadBytes instead of len(spec.Stdin)). §19: never log contents."

- file: internal/cmd/models.go
  why: "The ONLY production CmdSpec constructed WITHOUT going through Render (L151:
        CmdSpec{Command: argv[0], Args: argv[1:], Stdin: \"\"}). Verified NON-REGRESSION."
  critical: "It is a --list-models probe with no diff payload → Stdin==\"\" → PayloadBytes stays 0 →
    VerbosePayload(0) is a (correct) no-op. Do NOT change models.go. (If you 'fixed' it to set
    PayloadBytes you'd be inventing a payload that doesn't exist.)"

# Test patterns to mirror
- file: internal/provider/render_test.go
  why: "TestRender_FlagDelivery (L192–204) is the template for a delivery-mode Render test; helpers
        strPtr/containsPair/containsToken are in scope. TestRenderMultiTurn_PiTurn1_Golden (L499) is the
        multi-turn template (uses mtPiManifest())."
  pattern: "Build a minimal Manifest{Name,Command,PromptDelivery(,PromptFlag)}, call Render, assert on
        spec.Args/spec.Stdin. For THIS task: ALSO assert spec.PayloadBytes == len(payload) per mode."

- file: internal/provider/executor_test.go
  why: "TestExecute_Verbose (the verbose keystone) is the template: `var buf bytes.Buffer; vb :=
        ui.NewVerbose(&buf, true); spec := CmdSpec{...}; Execute(ctx, spec, timeout, vb); got := buf.String();
        strings.Contains(got, \"DEBUG: …\")`."
  gotcha: "TestExecute_Verbose builds CmdSpec DIRECTLY and sets Stdin but NOT PayloadBytes. After the
    fix the executor reads spec.PayloadBytes (==0 there), so its (unasserted) payload line would stop.
    Update that spec to set PayloadBytes and assert the line, AND add a dedicated positional-delivery
    test proving the line now appears (the regression). See Implementation Tasks Task 4."
```

### Current Codebase tree (relevant slice)

```bash
internal/provider/
  render.go          # CmdSpec (L22–29); Render delivery switch (~L163–178); RenderMultiTurn switch (~L272–282)  ← EDIT
  executor.go        # the single VerbosePayload(len(spec.Stdin)) call site  ← EDIT
  manifest.go        # delivery-mode constants (DefaultPromptDelivery="stdin"; valid stdin/positional/flag)
  builtin.go         # which builtins use which delivery mode (stdin=pi/claude/agy/qwen/opencode/codex; positional=cursor)
  render_test.go     # TestRender_FlagDelivery (L192), TestRenderMultiTurn_PiTurn1_Golden (L499)  ← ADD assertions + tests
  executor_test.go   # TestExecute_Verbose (verbose keystone)  ← UPDATE + ADD positional test
internal/ui/
  verbose.go         # VerbosePayload(bytes int) — DO NOT CHANGE (guard + format unchanged)
internal/cmd/
  models.go          # direct CmdSpec{...Stdin:""} at L151 — DO NOT CHANGE (verified non-regression)
```

### Desired Codebase tree with files to be edited and responsibility

```bash
internal/provider/render.go      # EDIT: add CmdSpec.PayloadBytes (+doc); set in Render switch; set in RenderMultiTurn switch
internal/provider/executor.go    # EDIT: VerbosePayload(len(spec.Stdin)) → VerbosePayload(spec.PayloadBytes)
internal/provider/render_test.go # ADD: assert PayloadBytes==len(payload) per mode (Render + MultiTurn)
internal/provider/executor_test.go # UPDATE TestExecute_Verbose (set PayloadBytes, assert line) + ADD positional test
# (NO new files; NO changes to internal/ui/verbose.go, internal/cmd/models.go, PRD.md, docs/, or any config)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL — there are TWO delivery switches and BOTH must set PayloadBytes.
// Render (single-turn) ~L163–178 AND RenderMultiTurn ~L272–282. Forgetting one means the
// multi-turn fallback path still emits no line for positional/flag. Set it in BOTH.

// CRITICAL — set PayloadBytes for ALL THREE cases (stdin/positional/flag), not just the two
// that were "broken". stdin must ALSO set it so that, for stdin delivery, PayloadBytes == len(spec.Stdin)
// (keeps the executor read uniform and the existing stdin behavior identical).

// CRITICAL — do NOT add a `len(spec.Stdin)` fallback in the executor. research/findings.md §6 proves
// every CmdSpec site is correct under the direct read:
//   - Render / RenderMultiTurn → set PayloadBytes explicitly
//   - models.go:151 → Stdin:"" (list-models probe, no payload) → PayloadBytes 0 → correct no-op
// A fallback would re-mask the bug for any future direct-construction caller.

// GOTCHA — CmdSpec's doc comment says it "intentionally does NOT carry the delivery mode — Stdin=\"\"
// disambiguates." ADD a sentence: PayloadBytes is the SIZE (delivery-agnostic), set by the renderers,
// so the executor can report it without knowing the mode. Otherwise a future "cleanup" may delete it.

// GOTCHA — TestExecute_Verbose (executor_test.go) builds CmdSpec directly with Stdin set but
// PayloadBytes unset. It does NOT currently assert the payload line, so it won't FAIL — but update it
// to set PayloadBytes and assert the line, so the verbose keystone stays meaningful under the new
// contract (executor reports PayloadBytes, not len(Stdin)).

// SECURITY (PRD §19) — VerbosePayload logs SIZE ONLY, never contents. Do not change that. The fix
// changes which size value is passed, not what is printed.
```

## Implementation Blueprint

### Data models and structure

Extend the existing `CmdSpec` struct (pure data; no new types, no pydantic/ORM — this is Go).

```go
// internal/provider/render.go
type CmdSpec struct {
    Command     string   // the executable (resolved manifest.Command), e.g. "pi", "agent"
    Args        []string // the flag portion AFTER command, in §12.2 token order (NOT including Command)
    Stdin       string   // payload to pipe (stdin delivery); "" → executor uses os.DevNull
    Env         []string // os.Environ() + manifest Env entries as "KEY=VAL" (manifest wins on collision)
    PayloadBytes int     // size of the delivered payload in bytes, regardless of delivery mode.
                         // Set by Render/RenderMultiTurn for ALL modes (stdin/positional/flag) so the
                         // executor can report it via VerbosePayload without knowing the delivery mode.
                         // 0 ⇒ no payload (e.g. models.go list-models probe) ⇒ no payload-size line.
                         // NOTE: CmdSpec stays delivery-mode-agnostic — this is the SIZE, not the mode.
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD PayloadBytes to CmdSpec (internal/provider/render.go, ~L22–29)
  - IMPLEMENT: add `PayloadBytes int` field to the CmdSpec struct (after Env).
  - DOCUMENT: per-field comment (see Data models above) AND extend the struct doc comment to note
              PayloadBytes is the delivery-agnostic SIZE set by the renderers (so it isn't mistaken
              for redundant with Stdin or deleted later).
  - NAMING: Go-idiomatic exported field, camelCase from snake intent → `PayloadBytes` (int).
  - PLACEMENT: internal/provider/render.go, in the CmdSpec struct block.
  - VERIFY: go build ./internal/provider/   # struct change compiles

Task 2: SET PayloadBytes in the Manifest.Render delivery switch (internal/provider/render.go ~L163–178)
  - IMPLEMENT: in the existing `switch *r.PromptDelivery` block, set
        spec.PayloadBytes = len(payload)
    for ALL THREE cases (stdin, positional, flag). `payload` is the local already in scope (userPayload,
    optionally prepended with sysPrompt when SystemPromptFlag==""). Place the assignment inside each case
    (or once after the switch, since `payload` is the same value in all three — AFTER the switch is
    cleanest and avoids triplication: `spec.PayloadBytes = len(payload)` once after the switch, before
    the Env block / return). NOTE: the `default` case returns an error before reaching that line, which
    is correct (invalid delivery mode → no spec returned).
  - FOLLOW pattern: the existing switch already constructs `spec` immediately above.
  - VERIFY: go test ./internal/provider/ -run 'TestRender_' -v   # existing render tests still green

Task 3: SET PayloadBytes in the Manifest.RenderMultiTurn delivery switch (internal/provider/render.go ~L272–282)
  - IMPLEMENT: identical one-line addition (`spec.PayloadBytes = len(payload)`) after the
    RenderMultiTurn delivery switch. Same reasoning as Task 2.
  - FOLLOW pattern: mirror Task 2 exactly; RenderMultiTurn is the sibling renderer (PRD §9.24 FR-T6).
  - VERIFY: go test ./internal/provider/ -run 'TestRenderMultiTurn' -v

Task 4: USE PayloadBytes in the executor (internal/provider/executor.go)
  - EDIT: change
        vb.VerbosePayload(len(spec.Stdin)) // size only (never contents) — exposes whether the token-limit gate ran
    to
        vb.VerbosePayload(spec.PayloadBytes) // size only (never contents) — exposes whether the token-limit gate ran
  - PRESERVE the comment (still accurate). This is the ONLY VerbosePayload call site.
  - DO NOT add a len(spec.Stdin) fallback (see Gotchas + research §6).
  - VERIFY: go build ./internal/provider/ ; grep -n VerbosePayload internal/provider/executor.go

Task 5: UPDATE + ADD render-side tests (internal/provider/render_test.go)
  - ADD to TestRender_FlagDelivery (L192): assert `spec.PayloadBytes == len("PAYLOAD")` for flag delivery.
  - ADD a sibling test `TestRender_PositionalDelivery_PayloadBytes` mirroring TestRender_FlagDelivery but
        with PromptDelivery="positional", asserting `spec.PayloadBytes == len(payload)`.
  - ADD a stdin assertion to an existing stdin-delivery render test (or a small new one) proving
        `spec.PayloadBytes == len(spec.Stdin)` for stdin delivery.
  - ADD to (or alongside) TestRenderMultiTurn_PiTurn1_Golden (L499): assert `spec.PayloadBytes ==
        len("<payload>")` for the multi-turn path (stdin; payload local). Optionally add a multi-turn
        positional/flag mini-test if a helper manifest is cheap.
  - FOLLOW pattern: TestRender_FlagDelivery's Manifest{...}/Render(...)/assert-on-spec shape + the
        strPtr/containsPair helpers already in the file.
  - NAMING: TestRender_PositionalDelivery_PayloadBytes, TestRenderMultiTurn_*_PayloadBytes.
  - COVERAGE: stdin, positional, flag (Render); at least stdin (MultiTurn); the three payload sizes
        include the system-prompt-prepend case where payload != userPayload.
  - VERIFY: go test ./internal/provider/ -run 'PayloadBytes|Positional|FlagDelivery' -v

Task 6: UPDATE + ADD executor-side tests (internal/provider/executor_test.go)
  - UPDATE TestExecute_Verbose: set `PayloadBytes: len("feat: hello\n")` on its directly-constructed
        CmdSpec, and ADD an assertion that `got` contains `"DEBUG: payload: 12 bytes"` (the verbose
        keystone should still prove the payload line emits). This keeps the test meaningful now that
        the executor reads PayloadBytes (not len(Stdin)).
  - ADD `TestExecute_VerbosePayload_PositionalDelivery`: construct a CmdSpec representing a positional
        delivery (Command:"cat" or "echo", Args with the payload as a trailing arg, Stdin:"", and
        PayloadBytes set to len(payload)); run Execute with a verbose-on sink; assert the captured
        buffer contains `DEBUG: payload: <N> bytes`. This is the REGRESSION test — before the fix this
        line was absent for positional delivery (VerbosePayload(0) no-op).
  - FOLLOW pattern: TestExecute_Verbose's `var buf bytes.Buffer; vb := ui.NewVerbose(&buf, true); …;
        got := buf.String(); strings.Contains(got, …)` idiom. Use mustBin(t,"cat") for a real subprocess
        (or "true"/"echo" if simpler), mirroring the file's existing mustBin usage.
  - MOCK/STUB: no external services — executor tests run real tiny binaries (cat/echo) guarded by
        mustBin, matching the existing style.
  - COVERAGE: positional delivery emits the line (regression); stdin still emits the line; size value
        matches len(payload).
  - VERIFY: go test ./internal/provider/ -run 'TestExecute_Verbose' -v
```

### Implementation Patterns & Key Details

```go
// The single cleanest insertion point in EACH renderer — AFTER the delivery switch, ONCE:
//   (the switch only mutates spec.Stdin / spec.Args; `payload` is identical in all three valid cases,
//    and the default case has already returned an error, so this line never runs for invalid modes)

	spec := &CmdSpec{Command: *r.Command, Args: args}
	switch *r.PromptDelivery {
	case "stdin":
		spec.Stdin = payload
	case "positional":
		spec.Args = append(spec.Args, payload)
	case "flag":
		spec.Args = append(spec.Args, *r.PromptFlag, payload)
	default:
		return nil, fmt.Errorf("provider render %q: unsupported prompt_delivery %q", m.Name, *r.PromptDelivery)
	}
	spec.PayloadBytes = len(payload) // ← ADD THIS ONE LINE (after the switch, once)

// (Do the same after the RenderMultiTurn switch.)

// Executor change — swap the size source, keep everything else:
	vb.VerboseCommand(strings.Join(append([]string{spec.Command}, spec.Args...), " "))
	vb.VerbosePayload(spec.PayloadBytes) // was: len(spec.Stdin)  ← size only; never contents (§19)

// Regression test shape (executor_test.go) — the line must now appear for positional:
func TestExecute_VerbosePayload_PositionalDelivery(t *testing.T) {
	mustBin(t, "echo") // or "cat"; any binary that exits 0
	var buf bytes.Buffer
	vb := ui.NewVerbose(&buf, true)
	payload := "some diff payload that is NOT on stdin"
	spec := CmdSpec{
		Command: "echo", Args: []string{payload}, // positional delivery: payload is a trailing arg
		Stdin: "",                                // positional → no stdin
		Env:  os.Environ(),
		PayloadBytes: len(payload),               // the renderer would set this; we set it directly here
	}
	if _, _, err := Execute(context.Background(), spec, 3*time.Second, vb); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := buf.String()
	want := fmt.Sprintf("DEBUG: payload: %d bytes", len(payload))
	if !strings.Contains(got, want) {
		t.Errorf("positional delivery: missing %q in verbose output:\n%s", want, got)
	}
}
```

### Integration Points

```yaml
NO BUILD/BINARY/CONFIG CHANGES:
  - No config-file schema, no CLI flag, no env var, no migration, no docs change (PRD FR50 already
    documents the line; this makes the implementation match). tasks.json §5: DOCS none.
  - internal/ui/verbose.go is UNCHANGED (VerbosePayload signature/guard/format intact; §19 honored).
  - internal/cmd/models.go is UNCHANGED (verified non-regression: Stdin:"" → PayloadBytes 0 → no-op).

VERBOSE-PIPELINE CONSISTENCY (the only "integration" affected):
  - executor.go VerbosePayload call now reads spec.PayloadBytes (set by Render/RenderMultiTurn).
  - All production Execute callers route through Render/RenderMultiTurn (research §7), so every path
    that ships a real diff payload now reports its size; the list-models probe path correctly reports none.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After each file edit — build + vet the package
go build ./internal/provider/...
go vet ./internal/provider/...

# After all edits — whole repo build + lint
go build ./...                 # expect: success
make lint                      # golangci-lint run (.golangci.yml) — expect: clean

# Confirm the structural changes are present (grep gates)
grep -n "PayloadBytes" internal/provider/render.go          # field decl + doc + 2 setter lines
grep -n "VerbosePayload" internal/provider/executor.go      # exactly ONE call, reading spec.PayloadBytes
grep -n "len(spec.Stdin)" internal/provider/executor.go     # expect: NO VerbosePayload match (stdin still used elsewhere is fine)

# Expected: zero build/vet/lint errors. If errors exist, READ output and fix before proceeding.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Targeted: render sets PayloadBytes per mode; executor reports it
go test ./internal/provider/ -run 'Render|MultiTurn|Execute|Verbose|PayloadBytes|Positional|FlagDelivery' -v

# Full provider package (race)
go test -race ./internal/provider/...

# Whole repo (race) — the Makefile `test` target
go test -race ./...

# Coverage gate (PRD §20.3, ≥85% on internal/provider — one of the 4 gated packages)
make coverage-gate

# Expected: all pass. internal/provider coverage stays ≥85%. If the gate fails, add/repair tests
# (Tasks 5–6 are designed to keep coverage healthy on the changed lines).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the real binary
go build -o /tmp/stagecoach ./cmd/stagecoach

# Set up a throwaway repo with a dirty tree
tmp=$(mktemp -d) && git init -q "$tmp" && cd "$tmp"
git config user.email t@t && git config user.name t
echo a > a.txt && git add . && git commit -qm "init"
echo b > b.txt && git add .   # staged change to generate a diff payload

# (a) stdin-delivery provider (pi) — payload-size line MUST appear (unchanged behavior)
#     Configure a stub/stdin provider per the repo's test harness, or use --dry-run + --verbose:
/tmp/stagecoach --verbose --dry-run 2>&1 | grep "DEBUG: payload:" && echo "OK: stdin payload line present"

# (b) positional-delivery provider (e.g. cursor, or a custom positional manifest) —
#     BEFORE the fix this line was ABSENT; AFTER it MUST appear:
#     (substitute a real positional-delivery provider manifest; the line content is what matters)
/tmp/stagecoach --verbose --provider cursor --dry-run 2>&1 | grep "DEBUG: payload:" && echo "OK: positional payload line now present"

cd - >/dev/null && rm -rf "$tmp"

# Prove models.go (list-models) is NOT regressed — it should emit NO payload line (no payload):
/tmp/stagecoach models --verbose 2>&1 | grep "DEBUG: payload:" && echo "FAIL: list-models should have no payload line" \
  || echo "OK: list-models emits no payload line (non-regression)"

# Expected: (a) and (b) print the payload line; the models check prints the OK (no-line) branch.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Security guard (PRD §19): verbose must NEVER log payload CONTENTS, only the size.
# After triggering a verbose run (Level 3), assert the diff body does not leak:
#   - the 'DEBUG: payload:' line must show a byte count + token estimate, NOT the diff text
#   - 'DEBUG: raw output:' shows the MODEL's output (allowed); the INPUT payload contents must not appear
# (This is unchanged by the fix, but worth re-confirming since this PR touches the verbose pipeline.)

# Race detector is mandatory in this repo (Makefile test target uses -race); ensure the new executor
# test path is exercised under -race:
go test -race ./internal/provider/ -run 'TestExecute_Verbose' -count=3
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `go build ./...` succeeds; `go vet ./internal/provider/...` clean; `make lint` clean.
- [ ] Level 1 grep gates: `PayloadBytes` field + 2 setters in render.go; one `VerbosePayload(spec.PayloadBytes)` in executor.go.
- [ ] Level 2: `go test -race ./...` green.
- [ ] Level 2: `make coverage-gate` passes (`internal/provider` ≥85%, PRD §20.3).
- [ ] Level 3: positional-delivery provider now prints `DEBUG: payload:` under `--verbose`.
- [ ] Level 3: `models --verbose` still prints NO payload line (non-regression).

### Feature Validation

- [ ] `Manifest.Render` sets `spec.PayloadBytes == len(payload)` for stdin, positional, AND flag.
- [ ] `Manifest.RenderMultiTurn` sets `spec.PayloadBytes == len(payload)` for all three modes.
- [ ] `executor.go` calls `vb.VerbosePayload(spec.PayloadBytes)` (the only `VerbosePayload` call).
- [ ] For stdin delivery, `PayloadBytes == len(spec.Stdin)` (existing pi/claude/agy/… behavior identical).
- [ ] For positional (cursor) and flag (user manifests), the `DEBUG: payload:` line now appears.
- [ ] No `len(spec.Stdin)` fallback added in the executor (research §6 proves it's unneeded + would re-mask the bug).
- [ ] §19 honored: only SIZE logged, never contents (VerbosePayload unchanged).

### Code Quality Validation

- [ ] Follows existing codebase patterns (delivery switch shape; executor verbose idiom; mustBin test style).
- [ ] Field placement in CmdSpec matches the existing struct block; comment extended (not deleted).
- [ ] Anti-patterns avoided (see Anti-Patterns): no new delivery-mode field, no fallback, no format change.
- [ ] No changes to `internal/ui/verbose.go`, `internal/cmd/models.go`, `PRD.md`, `docs/`, or any config.

### Documentation & Deployment

- [ ] No user-facing/config/API surface change (tasks.json §5: DOCS none) — none required, none added.
- [ ] Code is self-documenting: the `PayloadBytes` field comment explains WHY it exists despite the
      "CmdSpec carries no delivery mode" rule.
- [ ] The verbose-line comment in executor.go still reads accurately ("exposes whether the token-limit gate ran").

---

## Anti-Patterns to Avoid

- ❌ Don't set `PayloadBytes` in only ONE of the two renderers (Render vs RenderMultiTurn) — the
  multi-turn fallback path would still be broken for positional/flag. Edit BOTH switches.
- ❌ Don't set it for only stdin or only the "broken" positional/flag cases — set it for ALL THREE in
  each switch so the executor read is uniform and stdin stays `PayloadBytes == len(spec.Stdin)`.
- ❌ Don't add a `len(spec.Stdin)` fallback in the executor (`if spec.PayloadBytes > 0 … else len(spec.Stdin)`).
  research/findings.md §6 proves every site is correct with the direct read; a fallback re-masks the bug.
- ❌ Don't add a `DeliveryMode` field to CmdSpec — the struct is deliberately mode-agnostic; PayloadBytes
  is the SIZE, which is all the executor needs.
- ❌ Don't change `VerbosePayload`'s signature/guard/format or log contents — §19 security invariant.
- ❌ Don't modify `internal/cmd/models.go` to "set" a PayloadBytes — it's a list-models probe with no
  payload; PayloadBytes=0 (correct no-op) is the right state. Touching it is scope creep + wrong.
- ❌ Don't skip the regression test (Task 6 positional case) "because it's a one-line fix" — that test IS
  the proof the bug is fixed; without it a future refactor could silently reintroduce the no-op.
- ❌ Don't edit `PRD.md`, `tasks.json`, `prd_snapshot.md`, `.gitignore`, or any doc — this is an internal
  diagnostics correctness fix with no surface change.
