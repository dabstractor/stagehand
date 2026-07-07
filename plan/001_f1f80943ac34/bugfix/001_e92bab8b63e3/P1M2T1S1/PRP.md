# PRP — P1.M2.T1.S1: Add `reg.IsInstalled(m)` pre-flight check + plain exit-1 error in `buildDeps`

> **Scope discipline.** This subtask is the **single-line code fix** for PRD Issue 3 (provider
> command missing on `$PATH` triggers the rescue path / exit 3 instead of a pre-generation exit 1),
> plus its **Mode-A documentation**. The dedicated regression/missing-command **tests are S2's scope**
> (`P1.M2.T1.S2`) — do NOT write them here. Downstream subtasks P1.M3 (dry-run fidelity) and P1.M4
> (config-field application) both also touch `buildDeps`; make this edit minimal and surgical so it
> does not collide with them.

---

## Goal

**Feature Goal**: When the resolved provider's command is not on `$PATH`, `GenerateCommit` fails fast
**before** the `write-tree` snapshot with a plain error naming the missing command, so the process exits
**1** (`Error`) instead of being misclassified as a post-snapshot `*RescueError` (exit **3**).

**Deliverable**:
1. A `reg.IsInstalled(m)` pre-flight check added to `buildDeps` in `pkg/stagecoach/stagecoach.go`,
   immediately after `m.Validate()` and before the `return generate.Deps{...}`.
2. Two doc updates (Mode A): `docs/cli.md` exit-code table and `docs/how-it-works.md` failure-modes
   table, recording the new pre-generation exit-1 behavior.

**Success Definition**:
- `stagecoach --provider <missing-command>` (with `[provider.<name>] command = "/nonexistent/..."`)
  fails **pre-snapshot**, exit **1**, printing `provider "<name>": command "<cmd>" not found. Is the
  agent installed?` — with **no rescue block** and **no dangling tree object**.
- `go build ./...`, `go vet ./...`, and the **entire existing** `go test -race ./...` suite remain
  green (the change is behavior-preserving for every installed provider).

---

## Why

- **PRD §18.2 failure table** + **§13.5**: "on direct use, fail fast with 'provider X not found: is
  <command> installed?' … exit non-zero" — **pre-generation**, exit **1**.
- **Root cause** (full trace: `architecture/seam_provider_preflight.md`): the missing binary is only
  detected inside `provider.Execute`'s `cmd.Start` (`internal/provider/executor.go:64`), which runs
  ~40 lines / 2 pipeline steps **after** `WriteTree` (`generate.go:156` / `stagecoach.go:228`). The
  start-failure error is neither `DeadlineExceeded` nor `Canceled`, so the loop treats it as a
  non-zero exit, retries identically, exhausts attempts, and returns a `*RescueError{Kind:ErrRescue,
  TreeSHA:<non-empty>}` → `exitcode.For` maps to exit **3** → the §18.3 rescue block + a dangling tree.
- **Impact**: a trivial misconfiguration (wrong `command` path) produces a scary "commit generation
  failed" with a manual `git commit-tree … | xargs git update-ref` recipe and leaves an orphan tree.
- **Fix value**: the check belongs at manifest resolution, not at execution — `Registry.IsInstalled`
> already exists and is tested; this wires it into the one place every code path flows through.

---

## What

After `m.Validate()` succeeds and before `buildDeps` returns, call `reg.IsInstalled(m)`. On `false`,
return a **plain** (non-`*RescueError`, non-sentinel) `fmt.Errorf`:

```
provider %q: command %q not found. Is the agent installed?
```

Because it is a plain error, `exitcode.For` (`internal/exitcode/exitcode.go`) falls through every
`errors.Is` branch to `return Error` → **exit 1**, and `handleGenError`'s generic branch prints
`stagecoach: <msg>`. Because `buildDeps` returns before either pipeline runs, the check is strictly
before any `WriteTree` → no snapshot, no rescue, no dangling tree.

### Success Criteria

- [ ] `buildDeps` returns the plain missing-command error (exit 1) for an uninstalled provider,
      **before** any tree object is written.
- [ ] The check covers **both** the `CommitStaged` (common) and `runPipeline` (dry-run/SystemExtra)
      paths (single chokepoint — `buildDeps` feeds both).
- [ ] Every currently-green test stays green (no regression for installed providers).
- [ ] `docs/cli.md` and `docs/how-it-works.md` document the pre-generation exit-1 row.

---

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the exact function, line numbers, the primitive to reuse, the exact error
string, the exit-code mapping rationale, the docs files/sections, and the regression-safety proof are
all below.

### Documentation & References

```yaml
# MUST READ — binding architecture (root-cause trace + decision)
- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/seam_provider_preflight.md
  why: Proves WriteTree runs BEFORE the missing-command is detected; names buildDeps as the ideal
       insertion point (section 7) with the exact suggested shape; enumerates rejected alternatives.
  section: "7. Ideal insertion point for the pre-flight check"
  critical: The error MUST be a plain fmt.Errorf (NOT *RescueError) so exitcode.For maps it to 1.

- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/decisions.md
  why: D2 is the binding decision for this subtask — reuses Registry.IsInstalled, plain error, buildDeps.
  section: "D2 — Fix Issue 3 with a pre-snapshot IsInstalled check in buildDeps"

# The edit site
- file: pkg/stagecoach/stagecoach.go
  why: buildDeps (line 154) is the single chokepoint; insertion is between m.Validate() (line 182)
       and `return generate.Deps{...}` (line 186). reg (line 150) and m (line 180) already in scope.
  pattern: buildDeps already returns plain fmt.Errorf for "unknown provider %q" and Validate() failures
           → exitcode.For maps each to exit 1. Add the missing-command error in the SAME style.
  gotcha: fmt is already imported (line 12); no new imports. Do NOT move/reorder other return sites.

# The primitive being reused
- file: internal/provider/registry.go
  why: Registry.IsInstalled (line 76) does exec.LookPath(m.DetectCommand()); already tested
       (registry_test.go TestIsInstalled) and already used by providers list/show + buildDeps auto-detect.
  gotcha: IsInstalled prefers Detect over Command via DetectCommand(); for an explicit provider that
          sets only Command, DetectCommand() returns that command — correct probe. Validate() guarantees
          Command != nil && *Command != "" so DetectCommand() never returns "" here (no short-circuit).

# Exit-code mapping (why plain error => exit 1)
- file: internal/exitcode/exitcode.go
  why: For() falls through errors.Is(NothingToCommit/Timeout/Rescue/CAS) to `return Error` for a plain
       error. The Error=1 constant comment literally lists "agent missing".

# Docs to update (Mode A)
- file: docs/cli.md
  why: "Exit codes" table — exit-1 row already lists "agent missing"; affirm the PRE-GENERATION qualifier.
  section: "## Exit codes"
- file: docs/how-it-works.md
  why: "Failure modes and exit codes" table — ADD a pre-generation exit-1 row AHEAD of rescue rows.
  section: "### Failure modes and exit codes"
```

### Current Codebase tree (relevant slice)

```bash
pkg/stagecoach/stagecoach.go          # buildDeps (L154) — THE EDIT SITE
internal/provider/registry.go       # Registry.IsInstalled (L76) — reused primitive
internal/provider/manifest.go       # Validate() (L80), DetectCommand() (L103) — guarantees + probe
internal/exitcode/exitcode.go       # For() => Error(1) for a plain error
docs/cli.md                         # Exit codes table (Mode A)
docs/how-it-works.md                # Failure modes and exit codes table (Mode A)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: The error MUST be a plain fmt.Errorf, NOT a *generate.RescueError and NOT a sentinel.
//   exitcode.For maps *RescueError → 3 and ErrTimeout → 124. A plain error hits the final
//   `return Error` (1). Wrapping with fmt.Errorf("%w", someSentinel) would CHANGE the exit code.
//   Do NOT %w-wrap any generate.* sentinel here.

// CRITICAL: Place the check AFTER m.Validate() and BEFORE `return generate.Deps{...}`.
//   Validate() guarantees Command is non-nil/non-empty, so DetectCommand() returns a real string
//   and IsInstalled performs a genuine exec.LookPath (not the "" short-circuit).

// CRITICAL: Keep it in buildDeps (NOT CommitStaged, NOT Execute, NOT default_action.go).
//   buildDeps is called once by GenerateCommit and feeds BOTH CommitStaged and runPipeline, and it
//   returns before either pipeline runs → strictly before WriteTree → no dangling tree. (decisions.md D2)

// REGRESSION: IsInstalled uses exec.LookPath on m.DetectCommand(). For the test stub the Command is an
//   ABSOLUTE path to a built binary. LookPath on an absolute executable path returns "found" — this is
//   already proven by the green auto-detect branch (buildDeps calls IsInstalled over List() including
//   the stub) and providers list tests. So installed providers (incl. the stub) stay installed → no
//   behavior change. Only genuinely-missing commands take the new early-exit-1 path.
```

---

## Implementation Blueprint

### The exact edit (Task 1)

In `pkg/stagecoach/stagecoach.go`, function `buildDeps` (line 154). The current tail is:

```go
	m, ok := reg.Get(name)
	if !ok {
		return generate.Deps{}, fmt.Errorf("unknown provider %q", name)
	}
	if err := m.Validate(); err != nil {
		return generate.Deps{}, fmt.Errorf("provider %q: %w", name, err)
	}

	return generate.Deps{Git: git.New(repoDir), Manifest: m}, nil
}
```

Insert, between the `Validate()` error block and the `return generate.Deps{...}`:

```go
	// Pre-flight (PRD §18.2): fail fast with exit 1 BEFORE the snapshot if the provider command is
	// not on $PATH. Without this, a missing binary is only detected inside Execute's cmd.Start
	// (well after WriteTree), surfacing as a misleading exit-3 rescue with a dangling tree object.
	// reg.IsInstalled reuses the tested exec.LookPath(m.DetectCommand()) seam (registry.go:76).
	if !reg.IsInstalled(m) {
		return generate.Deps{}, fmt.Errorf(
			"provider %q: command %q not found. Is the agent installed?",
			name, m.DetectCommand())
	}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY pkg/stagecoach/stagecoach.go :: buildDeps
  - INSERT: the reg.IsInstalled(m) pre-flight block (code above) between m.Validate() (L182) and
            `return generate.Deps{...}` (L186).
  - REUSE: reg.IsInstalled(m) — the existing tested seam (internal/provider/registry.go:76). Do NOT
           add a new exec.LookPath call or import "os/exec".
  - ERROR SHAPE: plain fmt.Errorf("provider %q: command %q not found. Is the agent installed?", name,
            m.DetectCommand()). Do NOT %w-wrap any generate.* sentinel (would change the exit code).
  - NAMING/PLACEMENT: match the adjacent "unknown provider %q" / Validate() plain-error style.
  - DEPENDENCIES: none (reg + m already in scope; fmt already imported).
  - GUARDRAIL: this is the ONLY source edit. Do NOT touch CommitStaged, runPipeline, executor.go,
            registry.go, manifest.go, or exitcode.go.

Task 2: MODIFY docs/cli.md :: "## Exit codes" table
  - AFFIRM the exit-1 row's "agent missing" entry is a PRE-GENERATION fail-fast. The row currently
    reads: "General error (generation failed, parse failed after retries, agent missing, CAS, usage)."
  - MINIMAL CHANGE: extend to make the pre-snapshot nature explicit, e.g.
    "General error (generation failed, parse failed after retries, **provider command missing on
    $PATH (checked before the snapshot)**, CAS, usage)." Keep exit 1; do not alter other rows.
  - NAMING: stay consistent with the table's existing wording/voice.
  - PLACEMENT: the change stays inside the exit-1 row of the "## Exit codes" section.

Task 3: MODIFY docs/how-it-works.md :: "### Failure modes and exit codes" table
  - ADD a row, placed AHEAD of (above) the snapshot-based rescue rows, because it fires before WriteTree:
    "| Agent missing on `$PATH` | 1 (Error) | Check the `[provider.<name>] command` path; install the agent |"
  - PRESERVE the existing five rows and their order below it (Rescue/Timeout/CAS/NothingToCommit/General).
  - PLACEMENT: first row of the failure-modes table (it is the earliest failure in the pipeline).
```

### Implementation Patterns & Key Details

```go
// PATTERN (buildDeps already follows this for its other exit-1 returns): return a PLAIN fmt.Errorf.
// A plain error → exitcode.For falls through to `return Error` (1) → handleGenError generic branch
// prints `stagecoach: <msg>`. Compare the adjacent "unknown provider %q" return: identical discipline.
//
// GOTCHA: do NOT wrap a generate.* sentinel here. fmt.Errorf("...: %w", generate.ErrRescue) would make
// errors.Is(err, generate.ErrRescue) TRUE → exitcode.For returns 3. Keep it sentinel-free.
//
// why buildDeps (not elsewhere): single chokepoint feeding BOTH pipelines; returns before WriteTree.
```

### Integration Points

```yaml
CODE: none beyond the single buildDeps insertion (no new imports, no new exports, no API change).
DATABASE: none.
CONFIG: none.
ROUTES: none.
SIGNALS: none — the check runs before signal.SetSnapshot arms the rescue, so no disarm needed.
```

---

## Validation Loop

### Level 1: Syntax & Style (run after Task 1)

```bash
# Build + vet the affected packages. Expected: zero output / clean exit.
go build ./...
go vet ./pkg/stagecoach/... ./internal/provider/... ./internal/exitcode/...

# Format check (match repo convention; run formatter if it reports a diff).
gofmt -l pkg/stagecoach/stagecoach.go docs   # docs are .md; gofmt only touches the .go file
# Expected: gofmt lists nothing for stagecoach.go. If it does, run: gofmt -w pkg/stagecoach/stagecoach.go
```

### Level 2: Existing tests (regression guard — S1 writes NO new tests)

```bash
# The change must be behavior-preserving for every INSTALLED provider. Run the whole suite with -race.
go test -race ./...

# Targeted re-run of the most relevant packages (fast feedback):
go test -race ./pkg/stagecoach/... ./internal/provider/... ./internal/exitcode/... ./internal/generate/...
# Expected: all PASS. The stub binary is an absolute path → IsInstalled stays true → happy paths,
# dry-run, timeout, CAS, nothing-staged tests are unaffected.
```

> **If a previously-green test now fails**: the check almost certainly fired for a provider whose
> `command`/`detect` resolves to something not on `$PATH` in the test environment. Root-cause that
> specific case — do NOT weaken the check. (The auto-detect branch already proved installed stubs pass.)

### Level 3: Manual / end-to-end (proves the fix; mirrors the bug repro)

```bash
# Build the binary.
go build -o /tmp/stagecoach ./cmd/stagecoach

# Scratch repo + a provider whose command does not exist.
TMP=$(mktemp -d) && cd "$TMP"
git init -q && git config user.email t@e.com && git config user.name t
git commit -q --allow-empty -m init
printf '[provider.missing]\ncommand = "/nonexistent/path/agent"\n' > .stagecoach.toml
echo hi > a.txt && git add a.txt

# Run with the missing provider.
/tmp/stagecoach --provider missing; echo "EXIT=$?"
# EXPECT: prints a single line like
#   stagecoach: provider "missing": command "/nonexistent/path/agent" not found. Is the agent installed?
#   EXIT=1
# AND: NO "Commit generation failed" rescue block, NO "Tree ID:" line.

# PROVE no dangling tree was written: object count must be UNCHANGED by the run.
before=$(git count-objects -v | awk '/^count:/{print $2}')
/tmp/stagecoach --provider missing >/dev/null 2>&1
after=$(git count-objects -v | awk '/^count:/{print $2}')
[ "$before" = "$after" ] && echo "OK: no new objects ($before == $after)" || echo "FAIL: $before != $after"

# Contrast with a known-good installed provider (e.g. one of the built-ins if present) to confirm
# the happy path still works end-to-end after the edit.
```

### Level 4: Doc consistency

```bash
# Re-read the two tables and confirm:
#  - docs/cli.md exit-1 row names the pre-generation provider-missing case.
#  - docs/how-it-works.md failure-modes table has the new pre-generation row ABOVE the rescue rows.
grep -n "pre-generation\|missing on" docs/cli.md docs/how-it-works.md
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean.
- [ ] `go vet ./...` clean (or at least `./pkg/stagecoach/... ./internal/provider/... ./internal/exitcode/...`).
- [ ] `gofmt -l pkg/stagecoach/stagecoach.go` reports nothing.
- [ ] `go test -race ./...` — all previously-green tests still PASS (no new tests added in S1).
- [ ] Manual repro (Level 3): missing provider → exit 1, single-line message, **no** rescue block.
- [ ] Manual repro (Level 3): `git count-objects` unchanged across the run (no dangling tree).

### Feature Validation
- [ ] The error is a plain `fmt.Errorf` (a `grep`/read confirms no `*RescueError` and no `%w` of a
      `generate.*` sentinel at the insertion site).
- [ ] The check sits AFTER `m.Validate()` and BEFORE the `return generate.Deps{...}`.
- [ ] Both `CommitStaged` and `runPipeline` are protected (one `buildDeps` edit feeds both).
- [ ] `docs/cli.md` exit-1 row affirms pre-generation; `docs/how-it-works.md` has the new row above
      the rescue rows.

### Code Quality Validation
- [ ] Reuses the existing `Registry.IsInstalled`/`DetectCommand` seam — no new `exec.LookPath`, no
      `os/exec` import added to `pkg/stagecoach`.
- [ ] Matches `buildDeps`'s existing plain-error style (`unknown provider %q`, Validate failures).
- [ ] No edits outside `buildDeps` in source (CommitStaged/runPipeline/executor/registry/manifest/exitcode untouched).
- [ ] Docs wording consistent with existing table voice; no other doc rows disturbed.

### Documentation
- [ ] Error message is self-explanatory and names the provider + command.
- [ ] Doc tables reflect the actual implemented exit-1/pre-generation behavior.

---

## Anti-Patterns to Avoid

- ❌ **Don't wrap a `generate.*` sentinel** (`%w` of `ErrRescue`/`ErrTimeout`) — that flips the exit
  code to 3/124. Keep the error sentinel-free.
- ❌ **Don't return a `*RescueError`** — it carries a `TreeSHA` and maps to exit 3 with a rescue block.
- ❌ **Don't put the check in `CommitStaged`/`Execute`/`default_action.go`** — those either run after
  `WriteTree` (leaving a dangling tree) or don't cover both pipelines (decisions.md D2).
- ❌ **Don't add a second `exec.LookPath`** — reuse the tested `Registry.IsInstalled`.
- ❌ **Don't write the regression/missing-command tests here** — that is S2 (`P1.M2.T1.S2`). S1 only
  adds the code + docs and must keep the existing suite green.
- ❌ **Don't reorder/rewrite `buildDeps`** — surgical insertion only; P1.M4.T1.S1 will also edit
  `buildDeps` (apply `[generation]` output/strip_code_fence) and must not conflict with this change.

---

## Confidence Score

**9/10** — The fix is a single, surgical insertion at a precisely-identified chokepoint, reusing an
already-tested primitive, with the exact error string, exit-code rationale, regression-safety proof,
and manual repro all specified. The -1 reserves for the doc-wording judgment call (exact phrasing of
the pre-generation qualifier), which is non-blocking. One-pass implementation success is highly likely.
