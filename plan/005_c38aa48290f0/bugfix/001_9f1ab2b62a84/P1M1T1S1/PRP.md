# PRP — Add foreign-key conflict WARNING to `lazygitEntry.Install()`

**Work item**: P1.M1.T1.S1 · **PRD ref**: §h2.2/Issue 1 (Major) — `integrate install lazygit` silently creates a duplicate `<c-a>` key binding. **PRD clauses**: §9.21 (no-mangle guarantee), FR-I1 (`foreign` status), FR-I3 (no-mangle protocol), FR-I5 (lazygit target), FR-I4 parity (git-alias foreign-conflict handling).

---

## Goal

**Feature Goal**: Before `lazygitEntry.Install()` delegates to `integrate.Apply(ActionUpsert)`, run a best-effort probe for an **unmarked** `customCommands` entry already bound to the target key (e.g. `<c-a>`). If one exists, print a `WARNING` line to the install output stream explaining that installing will create a **duplicate** binding (because `customCommands` is a YAML *sequence*), and advising `--key` to choose a different binding. The install then proceeds through Apply's normal no-mangle preview/confirm/write flow unchanged. This restores parity with `gitAliasEntry.Install()`'s foreign-conflict surfacing (FR-I4) and the §9.21 "never silently mangle a config" promise.

**Deliverable**:
1. Modified `lazygitEntry.Install()` in `internal/cmd/integrate_lazygit.go` — a ~10-line additive probe before the existing `integrate.Apply(...)` call. **No signature change**; returns `integrate.InstallResult` as before.
2. Three new TDD tests in `internal/cmd/integrate_lazygit_test.go`.
3. A "Conflicting key behavior" note added to the `lazygit` target section of `docs/cli.md` (Mode A doc update).

**Success Definition**: When a lazygit `config.yml` already contains an unmarked entry bound to `<c-a>`, `stagecoach integrate install lazygit --yes` (and the interactive path) prints the exact WARNING line to stderr/out and still writes the marked stagecoach entry (resulting in two `<c-a>` entries — a documented duplicate, no longer *silent*). When no conflicting key exists, no WARNING is printed and behavior is identical to today. All existing lazygit tests still pass; `go test -race ./...` is green.

---

## User Persona

**Target User**: A developer who integrates stagecoach into lazygit (`stagecoach integrate install lazygit`), often non-interactively in scripts/CI (`--yes`).

**Use Case**: The user already has a personal `<c-a>` binding in lazygit's `config.yml` (unmarked). They run `stagecoach integrate install lazygit --yes`.

**Pain Points Addressed**: Today this **silently** appends a second `<c-a>` entry, producing a real conflicting binding in lazygit with no warning, no diff call-out, and (with `--yes`) zero indication anything collided. The user is left debugging a broken key binding.

---

## Why

- **No-mangle guarantee (§9.21)**: The integrate feature's central promise is that stagecoach must be impossible to mangle a config file silently. A silent duplicate is a violation of that promise's *spirit* even though the bytes round-trip cleanly.
- **FR-I4 parity**: `git-alias` already surfaces foreign conflicts (`WARNING: alias.<name> is currently set to "<value>" (not stagecoach) — it will be overwritten.`). lazygit is inconsistent.
- **`integrate list` already detects this**: it reports `foreign`. `install` must act on the same signal, not ignore it.

---

## What

A single best-effort probe in `lazygitEntry.Install()`, placed BEFORE the `integrate.Apply()` call:

1. Read the config file (`os.ReadFile(e.resolvedPath())`). On read error (missing/unreadable) → **skip** the probe and proceed to Apply (Apply owns the create path). Never return an error from the probe.
2. Parse on a **throwaway** `lazygitTarget{key: e.key}`. On parse error → **skip** (Apply will refuse with its own parse error). Never return early.
3. If `probe.findKeyItem(e.key) != nil` (an unmarked item binds the key), print to the resolved output stream:
   `WARNING: a <key> binding already exists (not managed by stagecoach); installing will create a duplicate customCommands entry — use --key to choose a different binding.`
4. Fall through to the existing `integrate.Apply(ActionUpsert)` call, **unchanged**.

The WARNING appears in BOTH interactive mode (before Apply's diff+confirm prompt) and `--yes` mode (before Apply's immediate write).

### Success Criteria

- [ ] Installing over an unmarked foreign `<c-a>` entry prints the exact WARNING (contains `WARNING` and `duplicate`) to the output stream.
- [ ] Installing over a foreign entry still appends the marked stagecoach entry → config ends with exactly **two** `<c-a>` entries (the foreign + stagecoach's marked one).
- [ ] Installing with no conflicting key prints **no** WARNING (clean golden / empty config).
- [ ] Interactive install surfaces BOTH the WARNING (on `opts.Out`) AND the diff (in the Confirm func's `diff` arg).
- [ ] `Install()` signature and return type are unchanged; the actual write still goes through Apply's no-mangle protocol (parse-first, diff, confirm, backup, atomic-write, validate).
- [ ] Read errors and parse errors in the probe never abort Install early (they fall through to Apply).
- [ ] Existing tests pass; `go test -race ./...` is green.

---

## All Needed Context

### Context Completeness Check

✅ Passes the "No Prior Knowledge" test: every file path, line number, exact method to reuse (`findKeyItem`, `Parse`, `resolvedPath`), the pattern to mirror (`gitAliasEntry.Install`), the test helpers (`newIsolatedLazygitEntry`), the exact WARNING string, the test fixtures, and a flagged **contract correction** (see Gotchas) are all specified below with verified line numbers.

### Documentation & References

```yaml
# MUST READ — architecture / issue analysis (the authoritative fix design for this exact work item)
- docfile: plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/architecture/issue_analysis.md
  section: "Issue 1 (Major)"  # Root cause + fix design + pattern reference
  why: "Names findKeyItem, Status()'s existing foreign detection, and the git-alias WARNING pattern to mirror."
- docfile: plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/architecture/system_context.md
  why: "Integrate feature architecture; confirms lazygitEntry delegates to Apply, git-alias does NOT."

# PRIMARY EDIT TARGET
- file: internal/cmd/integrate_lazygit.go
  why: "lazygitEntry.Install() is the method to modify."
  pattern: "Install() at line 302 currently delegates entirely to integrate.Apply(ActionUpsert)."
  gotcha: "Do NOT change the Apply call itself — add the probe BEFORE it only."

# THE PROBE SEAM (reuse, do not reinvent)
- file: internal/cmd/integrate_lazygit.go
  why: "findKeyItem(key) at line 193 returns an UNMARKED item whose key value == key (or nil). Parse(data) at line 67. resolvedPath() at line 336."
  pattern: "Status() at line 283 already uses exactly this probe: tgt.Parse(data) then tgt.findKeyItem(e.key) -> StatusForeign. Mirror it."

# THE PATTERN TO MIRROR (semantic + nil-Out resolution)
- file: internal/cmd/integrate_gitalias.go
  why: "gitAliasEntry.Install() is the reference for foreign-conflict surfacing + nil-Out handling."
  pattern: "Top of Install(): `out := opts.Out; if out == nil { out = os.Stderr }`. Foreign warning ~lines 130-137."
  gotcha: "git-alias embeds its warning in the `preview` string (it owns its own confirm). lazygit CANNOT — it delegates diff+confirm to Apply — so write the WARNING directly to `out` BEFORE the Apply call. This is the contract's chosen mechanism (item §3c)."

# APPLY SEMANTICS — needed for the correct test assertion
- file: internal/integrate/protocol.go
  why: "Apply()'s final outcome assignment (lines 228-235) determines OutcomeCreated vs OutcomeUpdated."
  pattern: "`if missing { OutcomeCreated } else if ActionRemove { OutcomeRemoved } else { OutcomeUpdated }`."
  gotcha: "Outcome is FILE-centric, not entry-centric. A pre-existing file with a foreign entry yields OutcomeUpdated, NOT OutcomeCreated. See Anti-Patterns."

# TEST PATTERNS — mirror these exactly
- file: internal/cmd/integrate_lazygit_test.go
  why: "newIsolatedLazygitEntry(t,key) helper; foreignYAML literal (unmarked <c-a>) at line 701; Confirm-captures-diff pattern at line 577 (TestLazygitEntry_Install_ConfirmReceivesDiff)."
- file: internal/cmd/integrate_gitalias_test.go
  why: "TestGitAlias_Install_ForeignConflictInPreview at line 208 — the precedent for asserting 'WARNING' appears in output."
- file: internal/cmd/testdata/lazygit/golden_input.yml
  why: "A clean config whose only customCommands key is 'b' (NOT '<c-a>') — use as the NoForeignNoWarning fixture."

# DOCS (Mode A)
- file: docs/cli.md
  why: "lazygit target section ~lines 292-340 has 'No-mangle behavior' + 'Idempotency' but NO conflict note."
  pattern: "Mirror the git-alias target's 'Conflicting alias behavior' subsection (~lines 269-277)."
  gotcha: "lazygit does NOT overwrite (customCommands is a sequence) — it APPENDS, creating a duplicate. The note wording must reflect this."
```

### Current Codebase tree (relevant slice)

```bash
internal/
  cmd/
    integrate_lazygit.go          # EDIT: lazygitEntry.Install() (line 302)
    integrate_lazygit_test.go     # EDIT: +3 tests
    integrate_gitalias.go         # READ: pattern to mirror
    integrate_gitalias_test.go    # READ: test precedent
    testdata/lazygit/
      golden_input.yml            # READ: NoForeign fixture (key 'b')
      golden_corrupt.yml          # (existing parse-refusal fixture)
  integrate/
    protocol.go                   # READ: Apply() + Outcome semantics
docs/
  cli.md                          # EDIT: + 'Conflicting key behavior' note (lazygit section)
plan/005_c38aa48290f0/bugfix/001_9f1ab2b62a84/
  architecture/issue_analysis.md # READ: authoritative fix design (§Issue 1)
  architecture/system_context.md # READ: integrate architecture
  P1M1T1S1/
    PRP.md                        # THIS FILE
    research/findings.md          # supporting findings + contract-correction detail
```

### Desired Codebase tree with files to be added/modified

```bash
internal/cmd/integrate_lazygit.go        # MODIFY — additive probe in Install()
internal/cmd/integrate_lazygit_test.go   # MODIFY — +3 test functions
docs/cli.md                              # MODIFY — + 'Conflicting key behavior' note
# (no new files; no new packages; no new imports — fmt & os already imported)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (CONTRACT CORRECTION): the item description (§6) asserts res.Outcome == OutcomeCreated
// for the foreign-key test. This is WRONG and the test will FAIL. Verified in protocol.go:228-235 —
// OutcomeCreated is reserved EXCLUSIVELY for the missing-file path (FR-I3g). A pre-existing file
// (foreign entry written, no marker) modified by Upsert yields OutcomeUpdated. Assert OutcomeUpdated.
// Rationale: Outcome is FILE-centric, not entry-centric.

// GOTCHA: opts.Out may be nil (e.g. existing TestLazygitEntry_Install_Creates passes Out omitted).
// fmt.Fprintf(opts.Out, ...) would PANIC on nil. Resolve first, mirroring gitAliasEntry.Install:
//   out := opts.Out
//   if out == nil { out = os.Stderr }
//   ... fmt.Fprintf(out, "WARNING: ...", e.key)
// (Apply resolves nil Out itself, but our probe runs BEFORE Apply.)

// GOTCHA: the probe is BEST-EFFORT. On os.ReadFile error -> SKIP (Apply handles create path).
// On probe.Parse error -> SKIP (Apply will refuse with its own parse error). NEVER return early
// and NEVER return an error from the probe. Use a THROWAWAY target separate from Apply's tgt.

// GOTCHA: yaml.v3 Node-API encode preserves mapping key order and emits <c-a> single-quoted
// (existing TestIntegrateLazygitKeyFlag asserts strings.Contains(data, "key: '<c-s>'")).
// So both the foreign (`- key: '<c-a>'`) and stagecoach (`- key: '<c-a>' # stagecoach-integration`)
// lines contain the substring `key: '<c-a>'` -> strings.Count == 2 is a valid assertion.

// CRITICAL: lazygit's customCommands is a SEQUENCE — two entries CAN share a key (unlike git config).
// That is WHY this is a duplicate, not an overwrite. The WARNING text must say "duplicate".
```

---

## Implementation Blueprint

### Implementation Tasks (ordered by dependencies — TDD: tests FIRST, then implement)

```yaml
Task 1: ADD three failing tests to internal/cmd/integrate_lazygit_test.go
  - PLACE: directly after TestLazygitEntry_Install_ConfirmReceivesDiff (~line 600), before TestLazygitEntry_Install_CorruptRefuses.
  - IMPLEMENT: (a) TestLazygitEntry_Install_ForeignKeyWarning, (b) TestLazygitEntry_Install_NoForeignNoWarning,
    (c) TestLazygitEntry_Install_ForeignKeyWarningInteractiveConfirm.
  - FOLLOW pattern: TestLazygitEntry_Install_ConfirmReceivesDiff (line 577) for Out=&buf + Confirm-func-diff-capture;
    newIsolatedLazygitEntry(t,"") for the isolated tmp configPath.
  - REUSE: the foreignYAML literal from TestLazygitEntry_Status_States (line 701):
        customCommands:
          - key: '<c-a>'
            command: 'other-tool'
            context: 'files'
  - NAMING: test funcs exactly as named above (matches the contract so the orchestrator can find them).
  - ASSERTIONS (see full bodies in "Implementation Patterns" below):
      (a) foreign: buf contains "WARNING" and "duplicate"; config has exactly TWO <c-a> entries;
          res.Outcome == integrate.OutcomeUpdated  <-- CONTRACT CORRECTION (NOT OutcomeCreated);
          config contains "stagecoach-integration" marker.
      (b) no-foreign: pre-write golden_input.yml (key 'b'); buf does NOT contain "WARNING".
      (c) interactive: foreign config; Yes:false; Confirm captures diff and returns true;
          assert buf (opts.Out) contains "WARNING" AND the captured diff contains "stagecoach-integration"
          (Apply's diff of the appended entry).
  - CONFIRM: after writing these, `go test ./internal/cmd/ -run TestLazygitEntry_Install_ForeignKey -v`
    FAILS (no WARNING printed yet). This is the red phase — expected.

Task 2: MODIFY lazygitEntry.Install() in internal/cmd/integrate_lazygit.go (line 302)
  - INSERT: a best-effort foreign-key probe as the FIRST statements of Install(), before the existing
    `tgt := &lazygitTarget{key: e.key}` / `integrate.Apply(...)` block.
  - EXACT CODE: see "Implementation Patterns" below (the probe block).
  - PRESERVE: the existing integrate.Apply(...) call UNCHANGED (same ApplyOptions, same return mapping).
  - NO IMPORTS: `fmt` and `os` are already imported.
  - VERIFY red→green: after this task, the Task 1 tests PASS.

Task 3: UPDATE docs/cli.md — add a "Conflicting key behavior" note to the lazygit target section
  - PLACE: in the `lazygit` target subsection, immediately AFTER the "Idempotency" paragraph and BEFORE
    "`integrate list` shows:" (~line 326).
  - CONTENT: a short paragraph mirroring the git-alias target's "Conflicting alias behavior" (lines 269-277),
    but accurate for lazygit's APPEND (not overwrite) semantics. See "Implementation Patterns" below.
  - WHY: Mode A doc update per contract §7 — the WARNING is self-documenting, but a docs note restores
    parity with the documented git-alias conflict behavior.
```

### Implementation Patterns & Key Details

**Task 2 — the exact probe to insert** at the top of `lazygitEntry.Install()` (line 302), before `tgt := &lazygitTarget{key: e.key}`:

```go
func (e *lazygitEntry) Install(ctx context.Context, opts integrate.InstallOptions) (integrate.InstallResult, error) {
	// FR-I4 / §9.21 parity: best-effort foreign-key probe BEFORE Apply. lazygitTarget.Upsert keys on the
	// MARKER; an UNMARKED entry already bound to our key is invisible to it and would be DUPLICATED
	// (customCommands is a YAML sequence — two entries can legally share a key). Surface it as a WARNING so
	// the user can pick --key. Mirrors gitAliasEntry.Install's foreign-conflict surfacing. Best-effort:
	// any read/parse failure simply skips the probe and falls through to Apply (Apply owns those paths).
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}
	if data, rerr := os.ReadFile(e.resolvedPath()); rerr == nil {
		probe := &lazygitTarget{key: e.key} // throwaway — separate state from Apply's tgt
		if perr := probe.Parse(data); perr == nil {
			if probe.findKeyItem(e.key) != nil {
				fmt.Fprintf(out, "WARNING: a %s binding already exists (not managed by stagecoach); installing will create a duplicate customCommands entry — use --key to choose a different binding.\n", e.key)
			}
		}
	}

	tgt := &lazygitTarget{key: e.key}
	res, err := integrate.Apply(ctx, integrate.ApplyOptions{
		Path:    e.resolvedPath(),
		Target:  tgt,
		Action:  integrate.ActionUpsert,
		Yes:     opts.Yes,
		Out:     opts.Out,
		Confirm: opts.Confirm,
	})
	if err != nil {
		return integrate.InstallResult{}, err
	}
	return integrate.InstallResult{Outcome: res.Outcome, Target: e.Name(), Path: res.Path, Backup: res.Backup}, nil
}
```

Notes on the probe:
- `out` is the **resolved** stream (nil ⇒ os.Stderr) — used for the WARNING so a nil `opts.Out` never panics.
- The probe is fully self-contained; `Apply` runs its own independent `Parse` on its own `tgt`. No shared mutable state.
- WARNING text is **EXACT** (item §3c) and contains both substrings the tests assert (`WARNING`, `duplicate`).

**Task 1 — test bodies** (place after `TestLazygitEntry_Install_ConfirmReceivesDiff`):

```go
func TestLazygitEntry_Install_ForeignKeyWarning(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "") // key defaults to <c-a>
	foreignYAML := `customCommands:
  - key: '<c-a>'
    command: 'other-tool'
    context: 'files'
`
	if err := os.WriteFile(e.configPath, []byte(foreignYAML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var buf bytes.Buffer
	res, err := e.Install(context.Background(), integrate.InstallOptions{Yes: true, Out: &buf})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !strings.Contains(buf.String(), "WARNING") {
		t.Errorf("output missing WARNING; got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "duplicate") {
		t.Errorf("output missing 'duplicate'; got %q", buf.String())
	}
	// CONTRACT CORRECTION: Apply reports an existing-file modification as Updated, NOT Created
	// (OutcomeCreated is reserved for the missing-file path — protocol.go:228-235).
	if res.Outcome != integrate.OutcomeUpdated {
		t.Errorf("Outcome = %v, want Updated", res.Outcome)
	}

	// The config must contain exactly TWO <c-a> entries: the foreign (unmarked) + stagecoach's marked one.
	data, err := os.ReadFile(e.configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if n := strings.Count(string(data), "key: '<c-a>'"); n != 2 {
		t.Errorf("want exactly 2 customCommands entries with key '<c-a>', got %d.\nconfig:\n%s", n, data)
	}
	if !strings.Contains(string(data), "stagecoach-integration") {
		t.Error("config missing stagecoach-integration marker")
	}
}

func TestLazygitEntry_Install_NoForeignNoWarning(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	// golden_input.yml's only customCommands key is 'b' — no <c-a> conflict.
	if err := os.WriteFile(e.configPath, readFixture(t, "golden_input.yml"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var buf bytes.Buffer
	_, err := e.Install(context.Background(), integrate.InstallOptions{Yes: true, Out: &buf})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if strings.Contains(buf.String(), "WARNING") {
		t.Errorf("output should NOT contain WARNING for a clean config; got %q", buf.String())
	}
}

func TestLazygitEntry_Install_ForeignKeyWarningInteractiveConfirm(t *testing.T) {
	e := newIsolatedLazygitEntry(t, "")
	foreignYAML := `customCommands:
  - key: '<c-a>'
    command: 'other-tool'
    context: 'files'
`
	if err := os.WriteFile(e.configPath, []byte(foreignYAML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var buf bytes.Buffer
	var gotDiff string
	_, err := e.Install(context.Background(), integrate.InstallOptions{
		Yes: false,
		Out: &buf,
		Confirm: func(_ io.Writer, _ string, diff string) bool {
			gotDiff = diff
			return true
		},
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	// WARNING surfaces on opts.Out (our buf); Apply's diff surfaces in the Confirm func's diff arg.
	if !strings.Contains(buf.String(), "WARNING") {
		t.Errorf("opts.Out missing WARNING; got %q", buf.String())
	}
	if !strings.Contains(gotDiff, "stagecoach-integration") {
		t.Errorf("confirm diff missing stagecoach entry; got %q", gotDiff)
	}
}
```

**Task 3 — docs/cli.md note** (insert after the lazygit "Idempotency" paragraph, ~line 326, before "`integrate list` shows:"):

```markdown
**Conflicting key behavior:** Because `customCommands` is a YAML *sequence*, lazygit permits two entries to share a key binding. If an **unmarked** entry already binds your target key (e.g. `<c-a>`), `install` prints a `WARNING` to stderr noting that a duplicate `customCommands` entry will be created, then proceeds through the normal no-mangle preview/confirm flow (outcome: *Updated*). Use `--key '<other>'` to install under a different binding instead. (`integrate list` reports this pre-existing state as `foreign`.) Unlike the `git-alias` target — where a foreign alias is **overwritten** — the lazygit target cannot overwrite (a sequence key is not unique), so it **appends** and surfaces the resulting duplicate for you to resolve.
```

### Integration Points

```yaml
SOURCE FILES:
  - modify: internal/cmd/integrate_lazygit.go  (lazygitEntry.Install, line 302)
  - modify: internal/cmd/integrate_lazygit_test.go  (+3 tests)
  - modify: docs/cli.md  (lazygit target subsection)
DATABASE: none
CONFIG: none (no new env vars, no new flags)
ROUTES/REGISTRY: none (lazygit is already registered; Install signature unchanged)
DEPENDENCIES: none (fmt & os already imported; no new go.mod entries)
```

---

## Validation Loop

### Level 1: Syntax & Style (run after Task 2; before proceeding to docs)

```bash
go build ./...                                  # compiles; expected: no errors
go vet ./internal/cmd/...                       # expected: no issues
gofmt -l internal/cmd/integrate_lazygit.go internal/cmd/integrate_lazygit_test.go  # expected: empty (no diffs)
# Expected: zero errors. Read any output and fix before proceeding.
```

### Level 2: Unit Tests (the TDD gates)

```bash
# Red phase (after Task 1, before Task 2) — these FAIL (no WARNING yet). Expected.
go test -race ./internal/cmd/ -run 'TestLazygitEntry_Install_ForeignKey' -v
go test -race ./internal/cmd/ -run 'TestLazygitEntry_Install_NoForeignNoWarning' -v

# Green phase (after Task 2) — all three PASS.
go test -race ./internal/cmd/ -run 'TestLazygitEntry_Install' -v

# Regression: the whole lazygit + gitalias + cmd package must stay green.
go test -race ./internal/cmd/ -v
# Expected: all pass, including pre-existing TestLazygitEntry_Install_* and TestLazygitEntry_Status_States.
```

### Level 3: Integration / End-to-End (mirrors the PRD reproduction steps)

```bash
# Build the binary.
go build -o /tmp/stagecoach ./cmd/stagecoach

# Reproduce the conflict scenario (PRD §h2.2 Steps to Reproduce) — now with a WARNING.
mkdir -p /tmp/lg/.config/lazygit
cat > /tmp/lg/.config/lazygit/config.yml <<'EOF'
customCommands:
  - key: '<c-a>'
    description: 'My existing AI commit'
    command: 'some-other-tool'
    output: none
EOF

# Install with --yes. lazygit need not be installed IF you pre-seed the config path;
# to force the resolved path without lazygit on PATH, point HOME at /tmp/lg and ensure the
# file is found via the platform-default fallback ($XDG_CONFIG_HOME/lazygit/config.yml):
HOME=/tmp/lg XDG_CONFIG_HOME=/tmp/lg/.config /tmp/stagecoach integrate install lazygit --yes
# EXPECTED: a line starting "WARNING: a <c-a> binding already exists ..." is printed.

# Inspect the config: exactly two <c-a> entries now exist (foreign + stagecoach), with a WARNING shown.
grep -c "key: '<c-a>'" /tmp/lg/.config/lazygit/config.yml   # -> 2
grep -c "stagecoach-integration" /tmp/lg/.config/lazygit/config.yml  # -> 1

# Clean config: no WARNING.
rm /tmp/lg/.config/lazygit/config.yml
cat > /tmp/lg/.config/lazygit/config.yml <<'EOF'
gui:
  showRandomTip: false
EOF
HOME=/tmp/lg XDG_CONFIG_HOME=/tmp/lg/.config /tmp/stagecoach integrate install lazygit --yes
# EXPECTED: NO "WARNING" line; Outcome Created (new file content appended cleanly).
```

### Level 4: Domain-Specific Validation

```bash
# Docs sanity: the new note is present and renders.
grep -n "Conflicting key behavior" docs/cli.md          # -> one match in the lazygit section
grep -n "duplicate" docs/cli.md                         # -> present in the lazygit conflict note

# Full CI-equivalent gate (what .github/workflows/ci.yml runs).
go test -race ./...                                     # expected: all pass
# Expected: green. This is the project's mandatory gate.
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` succeeds.
- [ ] `go vet ./internal/cmd/...` clean.
- [ ] `gofmt -l` reports no diffs on the two changed `.go` files.
- [ ] `go test -race ./...` is green (the project's CI gate).

### Feature Validation
- [ ] Foreign `<c-a>` entry → install prints WARNING (contains `WARNING` + `duplicate`), config ends with exactly two `<c-a>` entries, `Outcome == OutcomeUpdated`.
- [ ] Clean config (golden `key: 'b'` or empty) → no WARNING.
- [ ] Interactive install → WARNING on `opts.Out` AND Apply's diff in the Confirm func's `diff` arg.
- [ ] Read error / parse error in probe never aborts Install early (falls through to Apply).
- [ ] `Install()` signature and return type unchanged; write still flows through Apply's no-mangle protocol.

### Code Quality Validation
- [ ] Follows existing conventions: `newIsolatedLazygitEntry` helper, `readFixture`, `*bytes.Buffer` capture, Confirm-func-diff pattern.
- [ ] No new dependencies, no new imports, no signature changes.
- [ ] Probe is best-effort and side-effect-free (throwaway target, no shared state with Apply's `tgt`).

### Documentation & Deployment
- [ ] `docs/cli.md` lazygit section has a "Conflicting key behavior" note accurate to APPEND (not overwrite) semantics.
- [ ] The WARNING line text is self-documenting (exact wording from contract §3c).

---

## Anti-Patterns to Avoid

- ❌ **Do NOT assert `OutcomeCreated` for the foreign-key test.** The item description (§6) says so, but it is wrong: `integrate.Apply` (protocol.go:228–235) returns `OutcomeUpdated` for any upsert on an *existing* file. `OutcomeCreated` is reserved exclusively for the missing-file/create path. Assert `OutcomeUpdated`. (Documented in `research/findings.md`.)
- ❌ **Do NOT write `fmt.Fprintf(opts.Out, ...)` without resolving nil.** `opts.Out` can be nil (existing tests pass it omitted); a raw `Fprintf` panics. Resolve `out := opts.Out; if out == nil { out = os.Stderr }` first — exactly as `gitAliasEntry.Install` does.
- ❌ **Do NOT return an error or return early from the probe.** It is best-effort: read/parse failures skip the probe and fall through to Apply (Apply owns the create path and the parse-refusal error).
- ❌ **Do NOT change the `integrate.Apply(...)` call, its options, or `Install`'s signature.** The fix is purely an additive probe before it; the no-mangle protocol must remain the one that writes.
- ❌ **Do NOT try to make Upsert detect the foreign entry / prevent the duplicate.** The scope is to *surface* the conflict (WARNING), not to block it — `--key` is the user's resolution lever. Blocking would change install semantics and is out of scope.
- ❌ **Do NOT describe the lazygit conflict as an "overwrite" in docs.** Unlike git-alias (single-valued key → overwrite), lazygit's `customCommands` is a sequence → it **appends** a duplicate. The docs note and WARNING must say "duplicate".

---

## Confidence Score

**9/10** — One-pass success likelihood is very high. The change is a small, additive, dependency-free probe reusing an *existing* helper (`findKeyItem`) already proven by `Status()`, mirroring a *sibling* target's well-established pattern (`gitAliasEntry.Install`). Exact code, exact WARNING string, exact test bodies, verified line numbers, and verified validation commands are all provided. The one deliberate divergence from the literal item description (asserting `OutcomeUpdated` instead of the incorrect `OutcomeCreated`) is documented with the precise `protocol.go` lines that prove it, so the implementer will not be misled by the contract's error.
