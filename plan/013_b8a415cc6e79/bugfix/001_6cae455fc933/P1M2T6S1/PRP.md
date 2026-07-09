name: "P1.M2.T6.S1 — Detect --model/--provider shadowing by per-role config and emit a VerboseWarn hint in default_action.go (Issue 6)"
description: >
  Minor UX-diagnostics fix (no behavioral change). On the single-commit path, `--model` sets the
  GLOBAL default (`cfg.Model`), but a `[role.message] model = "Y"` in config takes precedence
  (FR-R3) for the message role — so the generated commit actually uses "Y", the user's bare `--model`
  value is never even validated (FR-R5b never fires), and stagecoach prints no error or warning.
  This is correct per spec but a real footgun: a user running `stagecoach --model glm-5.2` against a
  populated config silently gets the config's model. The fix adds a one-line `--verbose`-ONLY hint
  (`DEBUG: note: --model shadowed by [role.message].model; use --message-model to override`, and the
  `--provider` analog) emitted from `runDefault` right after the message-role resolution. When
  `--verbose` is off the hint is a complete no-op (zero stderr noise for normal users). FR-R3
  precedence is UNCHANGED. Also adds the precedence gotcha to the `--model` CLI help text and the
  role-config docs (Mode A).

---

## Goal

**Feature Goal**: When `--verbose` is on AND the user EXPLICITLY passed `--model` (or `--provider`)
that a per-role `[role.message]` config entry silently overrides, print a one-line `DEBUG: note: …`
hint so the shadowing is no longer invisible. Off by default; no exit-code or behavioral change.

**Deliverable** (Go, `internal/cmd` + Mode A docs — no new files):
1. In `internal/cmd/default_action.go` `runDefault`: create a `*ui.Verbose` sink
   (`ui.NewVerbose(stderr, cfg.Verbose)`, mirroring `runDecompose:399`) — **`runDefault` has none
   today** — and, immediately after the existing `config.ResolveRoleModel("message", *cfg)` call
   (~L176), emit `verbose.VerboseWarn(...)` when `cmd.Flags().Changed("model") && roleModel != cfg.Model`
   and when `cmd.Flags().Changed("provider") && roleProvider != cfg.Provider`.
2. Tests in `internal/cmd/default_action_test.go` (full `Execute` path, stub provider, temp repo)
   proving: (a) the hint fires for the model-shadow case; (b) it does NOT fire when verbose is off,
   when `--model` is absent, or when there is no per-role model; (c) the `--provider` analog fires.
3. Mode A docs: a SHORT precedence note in the `--model` flag description (`internal/cmd/root.go`
   ~L167), a precedence gotcha in `docs/cli.md` (~L25 + ~L120), and a cross-reference in
   `docs/configuration.md` (~L241 role-config section).

**Success Definition** (every command run from repo root):
- `go build ./...` succeeds; `go test -race ./...` is green; `make coverage-gate` passes
  (`internal/cmd` stays ≥85%, PRD §20.3); `make lint` is clean.
- With a `[role.message] model = "Y"` config, `stagecoach --model X --provider stub --verbose` prints
  (stderr) a line containing `DEBUG: note: --model shadowed by [role.message].model; use --message-model to override`.
- The SAME command WITHOUT `--verbose` prints NOTHING extra (the hint is a complete no-op when off).
- `grep -n "VerboseWarn" internal/cmd/default_action.go` shows the two hint call sites inside `runDefault`.

## User Persona (if applicable)

**Target User**: A developer with a populated stagecoach config (e.g. `[role.message] model =
"zai/glm-5-turbo"`, exactly what `config init` writes for non-pi providers) who runs
`stagecoach --model glm-5.2` expecting "just use this model for this run."

**Use Case**: "I passed `--model glm-5.2`; why did my commit use the config's model — and why no
error?" Today this is unanswerable without reading the source (the bare `glm-5.2` is silently never
validated). After the fix, a `--verbose` run prints the one-line note pointing at `--message-model`.

**User Journey**: `stagecoach --verbose --model glm-5.2` → scan stderr → see
`DEBUG: note: --model shadowed by [role.message].model; use --message-model to override` → re-run
with `--message-model glm-5.2` (or edit the config) → the intended model is used.

**Pain Points Addressed**: A silent precedence footgun where an explicit flag is ignored with zero
feedback. The hint turns an invisible "wrong model used" into a one-line, actionable diagnostic.

## Why

- **PRD §9.15 FR-R3 is correct, but UX-punishing**: per-role config beats the global `[defaults]`.
  `--model`/`--provider` set the GLOBAL default only; a `[role.<role>]` entry still wins for that
  role. The PRD's own Issue 6 flags this as a footgun and suggests exactly this hint.
- **Zero behavioral risk**: the hint is `--verbose`-gated via `VerboseWarn` (which no-ops when off).
  No precedence change, no exit-code change, no new config key/flag/env. Off → byte-identical to today.
- **Cheap, surgical, single function**: all logic lives in `runDefault` after the one role-resolution
  call it already makes. The decompose path is explicitly out of scope (see Anti-Patterns).

## What

`--verbose` prints a `DEBUG: note: …` hint when an EXPLICIT `--model` or `--provider` is overridden
by a `[role.message]` entry, on the single-commit (default) path only. Concretely, after
`runDefault` resolves the message role, it compares the RESOLVED `roleModel`/`roleProvider` against
the explicit flag values (`cfg.Model`/`cfg.Provider`, which equal the flag values because the flag is
the highest precedence layer) and emits the matching note when they differ. The hint uses the existing
`VerboseWarn` channel/format. Nothing else changes.

### Success Criteria

- [ ] `runDefault` constructs a `*ui.Verbose` via `ui.NewVerbose(stderr, cfg.Verbose)`.
- [ ] When `cmd.Flags().Changed("model")` AND `roleModel != cfg.Model`, `VerboseWarn` emits the
      model-shadow note (exact wording in Implementation Blueprint).
- [ ] When `cmd.Flags().Changed("provider")` AND `roleProvider != cfg.Provider`, `VerboseWarn` emits
      the provider-shadow note.
- [ ] When `--verbose` is OFF, NO hint is printed (VerboseWarn no-op; normal users see nothing new).
- [ ] When `--model` is absent, OR there is no per-role message model, OR the per-role model equals
      the `--model` value, NO model hint is printed (no false positives).
- [ ] No change to FR-R3 precedence, exit codes, or any other behavior.
- [ ] `go build ./...`, `go test -race ./...`, `make coverage-gate`, and `make lint` all pass.

## All Needed Context

### Context Completeness Check

_Yes._ The exact function (`runDefault`), the exact insertion line (after the `ResolveRoleModel`
call at `default_action.go:176`), the exact detection condition (with proof it is necessary &
sufficient), the EXISTING `VerboseWarn` contract (quoted), the corrected fact that `runDefault`
has no verbose sink today (the architecture research was wrong — see Gotchas), the verified reason
the stub provider makes clean tests possible (no `provider_flag` ⇒ `ValidateModel` accepts any
model), the existing test harness to copy, and the exact doc locations are all named below with line
numbers. An implementer who has never seen this repo can apply the edits and verify with the
copy-pasteable commands.

### Documentation & References

```yaml
# MUST READ — the contract & ground truth
- file: PRD.md
  why: "§9.15 FR-R3 (per-role beats global [defaults]); §9.15 FR-R5b (bare-model validation); the
        Issue 6 description (h3.5) which proposes this exact --verbose hint."
  critical: "FR-R3 precedence is CORRECT — this PR must NOT change behavior, only surface a --verbose
    hint. The hint is advisory; it does not alter which model/provider is used."

- file: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/prd_snapshot.md
  why: "Snapshot of the PRD section for this issue (h2.3 / h3.5 = Issue 6)."
  section: "h3.5 Issue 6"

- file: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/architecture/research_provider_verbose.md
  why: "Pre-existing architecture research that confirmed Issue 6 with the precedence path."
  critical: "It claims 'Verbose sink available at default_action.go:399' — that line is inside
    runDecompose, NOT runDefault. runDefault has NO verbose sink; this PR creates one. See
    P1M2T6S1/research/findings.md §3 for the correction."

- file: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M2T6S1/research/findings.md
  why: "THIS task's own research: the verbose-sink correction (§3), the detection-condition proof
        (§5, incl. edge cases), the scope boundary (§7), and the test-design rationale (§8). Read it first."

# The code to edit — current state captured at research time
- file: internal/cmd/default_action.go
  why: "Owns runDefault — the single-commit default action. The message-role resolution is at L176
        (`roleProvider, roleModel, _ := config.ResolveRoleModel(\"message\", *cfg)`). Insert the
        verbose sink + shadow checks immediately AFTER that line, BEFORE the labelProvider/labelModel
        computation (~L178). cmd.Flags().Changed(\"model\"/\"provider\") is available — `cmd` is the
        first parameter of runDefault(cmd *cobra.Command, args []string)."
  pattern: "runDecompose (same file, ~L399) already does `verbose := ui.NewVerbose(stderr, cfg.Verbose)`
    and calls verbose.VerboseRoles(...)/VerboseWarn-style methods. Copy that sink-creation line into
    runDefault just before the checks."
  gotcha: "runDefault does NOT currently create a *ui.Verbose. You MUST add `verbose :=
    ui.NewVerbose(stderr, cfg.Verbose)` in runDefault; reusing the (nonexistent) sink is a compile
    error. Do NOT move logic into runDecompose (wrong path — only fires on decompose)."

- file: internal/config/roles.go
  why: "ResolveRoleModel(role, cfg) — L34-57. per-role (cfg.Roles[role]) beats global for each field
        independently; absent role inherits global. The returned (provider, model, reasoning) IS what
        the generation pipeline uses (verified: pkg/stagecoach/stagecoach.go:348 + :502)."
  critical: "Do NOT modify roles.go. roleModel/roleProvider (already computed at L176) are the
    RESOLVED values — comparing them to cfg.Model/cfg.Provider detects effective shadowing."

- file: internal/ui/verbose.go
  why: "VerboseWarn(msg string) — L101-108. Format `DEBUG: <msg>\\n`; no-op when v==nil, v.w==nil, or
        !v.on. NewVerbose(w, on) — L33. This is the exact channel/format to use; do NOT change it."
  critical: "Because VerboseWarn guards on !v.on, emitting it unconditionally is SAFE — when
    --verbose is off it writes zero bytes. That is the whole UX guarantee (no noise for normal users)."

- file: internal/provider/manifest.go
  why: "ValidateModel (L136-154) — proves the stub provider accepts ANY model in tests: it only
        enforces the FR-R5b slash rule when *r.ProviderFlag != \"\"; the test stub has no
        provider_flag, so ValidateModel returns nil for any model string."
  critical: "Read-only context — do NOT edit. Explains why the model-shadow test can set an arbitrary
    [role.message] model without a validation error aborting before the hint."

- file: pkg/stagecoach/stagecoach.go
  why: "buildDeps (L348) and the message-model resolve (L502) both call ResolveRoleModel(\"message\",
        cfg) — PROOF the per-role config drives the ACTUAL generation, not just the label. The
        shadowing this PR hints about is real."
  critical: "Read-only context — do NOT edit. Confirms roleModel != cfg.Model genuinely means 'your
    --model will not be used'."

# Flag-change detection idiom (already used in this package)
- file: internal/cmd/hookexec.go
  why: "L56 `if cmd.Flags().Changed(\"edit\")` — the exact idiom for 'did the user pass this flag'.
        Mirror it: `cmd.Flags().Changed(\"model\")`, `cmd.Flags().Changed(\"provider\")`."

# Test patterns to mirror
- file: internal/cmd/default_action_test.go
  why: "Full-Execute test harness. setupStubRepo (L88) writes a .stagecoach.toml with [provider.stub]
        and seeds a commit; setupStubRepoRaw (L156) writes a RAW toml (use this to inject a
        [role.message] block). TestRunDefault_DryRun (L288) is the template for a --verbose-stderr
        assertion via errBuf + rootCmd.SetArgs/SetOut/SetErr + Execute(context.Background())."
  pattern: "saveRootState/restoreRootState around each test; rootCmd.SetOut(&outBuf); rootCmd.SetErr(&errBuf);
    rootCmd.SetArgs([]string{...}); err := Execute(context.Background()); then strings.Contains(errBuf.String(), ...)."

# Mode A docs (ride with the work)
- file: internal/cmd/root.go
  why: "The --model StringVar description (~L167). Add a SHORT precedence clause (help is
        terminal-width-wrapped via flagUsagesWrapped; keep it to one appended sentence)."
- file: docs/cli.md
  why: "The --model row (~L25) and the 'Message-role resolution' note (~L120). Add the precedence
        gotcha + the new --verbose hint."
- file: docs/configuration.md
  why: "The role-config / per-role section (~L241). Cross-reference the precedence gotcha."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  default_action.go          # runDefault: ResolveRoleModel("message") at L176; NO verbose sink today  ← EDIT (add sink + checks)
  default_action_test.go     # setupStubRepo(L88), setupStubRepoRaw(L156), TestRunDefault_DryRun(L288)  ← ADD tests
  root.go                    # --model flag StringVar description (~L167)                               ← EDIT (Mode A doc)
  hookexec.go                # cmd.Flags().Changed(...) idiom (L56) — REFERENCE ONLY
internal/config/
  roles.go                   # ResolveRoleModel (L34-57) — REFERENCE ONLY (do not edit)
internal/ui/
  verbose.go                 # VerboseWarn (L101-108), NewVerbose (L33) — REFERENCE ONLY (do not edit)
pkg/stagecoach/
  stagecoach.go              # buildDeps(L348)/L502 use ResolveRoleModel("message") — PROOF shadowing is real
docs/
  cli.md                     # --model row (~L25), Message-role resolution (~L120)  ← EDIT (Mode A doc)
  configuration.md           # role-config section (~L241)                          ← EDIT (Mode A doc)
```

### Desired Codebase tree with files to be edited and responsibility

```bash
internal/cmd/default_action.go          # EDIT: runDefault — add `verbose := ui.NewVerbose(stderr, cfg.Verbose)` +
                                        #       two VerboseWarn shadow checks after the L176 ResolveRoleModel call
internal/cmd/default_action_test.go     # ADD: 4-5 tests (model shadow fires; no-op when off/absent/no-per-role;
                                        #      provider shadow fires) via the Execute + stub-repo harness
internal/cmd/root.go                    # EDIT: append a SHORT precedence clause to the --model flag help (~L167)
docs/cli.md                             # EDIT: --model precedence gotcha + the new --verbose hint (~L25, ~L120)
docs/configuration.md                   # EDIT: cross-reference the precedence gotcha in the role-config section (~L241)
# (NO new files; NO changes to roles.go, verbose.go, manifest.go, stagecoach.go, PRD.md, tasks.json, prd_snapshot.md)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL — runDefault has NO *ui.Verbose sink today. The architecture research
// (research_provider_verbose.md, Issue 6) said "Verbose sink available at default_action.go:399" —
// that line is inside runDecompose, a DIFFERENT function. You MUST create the sink in runDefault:
//     verbose := ui.NewVerbose(stderr, cfg.Verbose)
// mirroring runDecompose:399. Without this, `verbose.VerboseWarn(...)` will not compile.

// CRITICAL — emit the hint in runDefault (the single-commit path), NOT runDecompose. The subtask
// title scopes this to default_action.go. runDecompose resolves all four roles and has the same
// latent issue, but it is OUT OF SCOPE (see Anti-Patterns). Only the "message" role is active on
// the single-commit path, so only the message role is checked here.

// CRITICAL — detect an EXPLICIT flag via cmd.Flags().Changed("model"/"provider"), NOT via
// cfg.Model != "". cfg.Model can come from [defaults]/env (intentional config — must NOT warn).
// Only the flag-changed bit means "the user typed --model this run". Idiom: hookexec.go:56.
// When Changed("model") is true, cfg.Model == the flag value (flag is the highest layer), so
// comparing roleModel != cfg.Model compares the resolved value against the explicit flag value.

// CRITICAL — use the value comparison `roleModel != cfg.Model` (and `roleProvider != cfg.Provider`),
// NOT merely `cfg.Roles["message"].Model != ""`. The value compare fires ONLY when the explicit
// --model will actually NOT be used (it implies the per-role model is set AND differs), and avoids
// a false positive when --model X equals [role.message].model X (no real surprise). cfg.Roles may
// be nil/absent — ResolveRoleModel handles that (falls back to global ⇒ roleModel==cfg.Model ⇒ no warn).

// GOTCHA — VerboseWarn already no-ops when --verbose is off (verbose.go:101-108 guards !v.on).
// So you call verbose.VerboseWarn(...) UNCONDITIONALLY at the check site; there is no need for an
// extra `if cfg.Verbose` wrapper. That unconditional call is what guarantees zero noise off-path.

// GOTCHA — the hint must be emitted to `stderr` (runDefault's `stderr := cmd.ErrOrStderr()`), via
// the verbose sink, to match every other DEBUG: line's stream. Do NOT print to stdout (stdout is
// the pipeable result stream, §15.5) and do NOT use fmt.Fprintln(stderr, ...) unguarded (that would
// leak the hint when --verbose is off). The verbose sink is the correct, guarded channel.

// GOTCHA (tests) — the stub provider ([provider.stub] with prompt_delivery="stdin", no
// provider_flag) accepts ANY model in ValidateModel (manifest.go:136-154 only enforces the FR-R5b
// slash rule when provider_flag is set). So a test can set [role.message] model = "<anything>" and
// pass --model <other> without a validation error aborting before the hint. Use --dry-run so no
// commit is created (the hint is emitted in runDefault BEFORE GenerateCommit, so --dry-run reaches it).
```

## Implementation Blueprint

### Data models and structure

None. No struct/type changes. The fix reuses `*ui.Verbose` (already defined in `internal/ui/verbose.go`)
and the existing `roleProvider`/`roleModel` locals already computed at `default_action.go:176`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE the verbose sink in runDefault + ADD the two shadow checks (internal/cmd/default_action.go)
  - CONTEXT: runDefault's message-role resolution is at L176:
        roleProvider, roleModel, _ := config.ResolveRoleModel("message", *cfg)
    Immediately AFTER that line (before the labelProvider/labelModel computation at ~L178), add:
        verbose := ui.NewVerbose(stderr, cfg.Verbose) // mirrors runDecompose; runDefault had no sink
        if cmd.Flags().Changed("model") && roleModel != cfg.Model {
            verbose.VerboseWarn("note: --model shadowed by [role.message].model; use --message-model to override")
        }
        if cmd.Flags().Changed("provider") && roleProvider != cfg.Provider {
            verbose.VerboseWarn("note: --provider shadowed by [role.message].provider; use --message-provider to override")
        }
  - WHY these conditions: see Known Gotchas + research/findings.md §5. Under Changed("model"),
    cfg.Model == the flag value; roleModel is the RESOLVED value; they differ IFF a per-role message
    model overrides the explicit flag. Identical-value + absent-per-role + absent-flag all correctly
    skip the warning.
  - NAMING/PLACEMENT: local `verbose` (matches runDecompose:399). Place the block right after the
    ResolveRoleModel call, before label resolution, so the hint appears in the verbose stream ahead
    of the "↳ Generating…" progress + the GenerateCommit DEBUG lines.
  - PRESERVE: do NOT touch the labelProvider/labelModel logic, the ValidateModel calls, or anything
    below. FR-R3 precedence is unchanged.
  - VERIFY: go build ./internal/cmd/...   # compiles; verbose.VerboseWarn resolves

Task 2: ADD tests (internal/cmd/default_action_test.go)
  - ADD a helper or inline a raw toml that injects a [role.message] block. Simplest: mirror
    setupStubRepo (L88) but append a [role.message] section to the toml, OR use setupStubRepoRaw
    (L156) with a hand-written toml containing [provider.stub] + [role.message]. Set
    t.Setenv("STAGECOACH_STUB_OUT", "feat: x"). Stage one file (writeFile+stageFile) so HasStagedChanges
    is true and the run reaches the message-role resolution.
  - TEST A — model shadow fires (the PRD scenario):
        config: [provider.stub] + [role.message] model = "stub-msg"
        args:   ["--provider", "stub", "--model", "other", "--verbose", "--dry-run"]
        assert: strings.Contains(errBuf.String(), "DEBUG: note: --model shadowed by [role.message].model")
  - TEST B — no hint when --verbose is OFF (the no-noise guarantee):
        same config + args as TEST A but WITHOUT "--verbose"
        assert: !strings.Contains(errBuf.String(), "shadowed")
  - TEST C — no hint when --model is absent (intentional per-role config):
        config: [provider.stub] + [role.message] model = "stub-msg"
        args:   ["--provider", "stub", "--verbose", "--dry-run"]   (no --model)
        assert: !strings.Contains(errBuf.String(), "shadowed")
  - TEST D — no hint when there is no per-role model (global --model is used):
        config: [provider.stub] only
        args:   ["--provider", "stub", "--model", "foo", "--verbose", "--dry-run"]
        assert: !strings.Contains(errBuf.String(), "shadowed")
  - TEST E — provider shadow fires:
        config: [provider.stub] + [provider.alt] (BOTH = same stub binary) + [role.message] provider = "alt"
        args:   ["--provider", "stub", "--verbose", "--dry-run"]
        assert: strings.Contains(errBuf.String(), "DEBUG: note: --provider shadowed by [role.message].provider")
    (Two provider aliases pointing at the stub binary keep the run valid: cfg.Provider="stub" is
     registered, roleProvider="alt" is registered (ValidateModel skipped/nil for empty model), and
     they differ ⇒ the provider note fires.)
  - FOLLOW pattern: saveRootState/restoreRootState + rootCmd.SetOut/SetErr/SetArgs + Execute(ctx)
    exactly as TestRunDefault_DryRun (L288). Assert on errBuf.String() (stderr is where DEBUG: lands).
  - MOCK/STUB: the compiled stubagent binary (stubtest.Build) — same as every other default_action test.
  - COVERAGE: fires (model + provider), and three negative cases (off / no-flag / no-per-role).
  - NAMING: TestRunDefault_VerboseModelShadowHint, TestRunDefault_VerboseShadowHint_OffIsNoop,
    TestRunDefault_VerboseShadowHint_NoFlag, TestRunDefault_VerboseShadowHint_NoPerRoleModel,
    TestRunDefault_VerboseProviderShadowHint.
  - VERIFY: go test ./internal/cmd/ -run 'VerboseShadowHint|VerboseProviderShadowHint' -v

Task 3: Mode A docs — --model precedence note (internal/cmd/root.go ~L167 + docs/cli.md + docs/configuration.md)
  - root.go ~L167: append ONE concise sentence to the --model StringVar description, e.g.
        "Model override (env STAGECOACH_MODEL, git stagecoach.model; default per-manifest default_model). " +
        "Sets the GLOBAL default — a [role.<role>] model in config takes precedence for that role; use --<role>-model to override (a --verbose run notes the shadowing)."
    Keep it short (help is terminal-width-wrapped; do not bloat). If the appended text makes the
    wrapped help unwieldy, prefer a shorter clause and put the full note in docs/cli.md instead.
  - docs/cli.md ~L25 (--model row) and ~L120 (Message-role resolution): add a precedence gotcha
    paragraph: "--model and --provider set the GLOBAL default only. A [role.<role>] model/provider in
    config (or a --<role>-model/--<role>-provider flag) takes precedence for that role (FR-R3) — so a
    populated config can silently shadow --model. Use --<role>-model (e.g. --message-model) to override
    a specific role, or run with --verbose to see a note when --model/--provider is shadowed."
  - docs/configuration.md ~L241 (role-config / per-role section): cross-reference the same gotcha in
    one sentence (point at the --message-model flag and the --verbose hint).
  - DO NOT edit PRD.md, tasks.json, prd_snapshot.md, or .gitignore.
  - VERIFY: go build ./... (root.go change compiles); make lint (markdownlint on the .md if configured).
```

### Implementation Patterns & Key Details

```go
// The single insertion point in runDefault — immediately after the message-role resolve (L176):
	roleProvider, roleModel, _ := config.ResolveRoleModel("message", *cfg)

	// P1.M2.T6.S1 (Issue 6): --model/--provider set the GLOBAL default; a [role.message] entry takes
	// precedence (FR-R3) and silently shadows an explicit flag. Emit a --verbose-only hint so the
	// footgun is visible. runDefault had no *ui.Verbose sink (runDecompose did, at L399) — create one.
	// VerboseWarn no-ops when --verbose is off, so normal users see nothing. No behavioral change.
	verbose := ui.NewVerbose(stderr, cfg.Verbose)
	if cmd.Flags().Changed("model") && roleModel != cfg.Model {
		verbose.VerboseWarn("note: --model shadowed by [role.message].model; use --message-model to override")
	}
	if cmd.Flags().Changed("provider") && roleProvider != cfg.Provider {
		verbose.VerboseWarn("note: --provider shadowed by [role.message].provider; use --message-provider to override")
	}

	labelProvider := roleProvider // ← existing code resumes here (unchanged)
	// …

// Regression-test shape (default_action_test.go) — the model-shadow hint must appear on stderr:
func TestRunDefault_VerboseModelShadowHint(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	chdir(t, repo)
	isolateHome(t)
	toml := fmt.Sprintf(`[provider.stub]
command = %q
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true

[role.message]
model = "stub-msg"
`, bin)
	writeConfigFile(t, repo, ".stagecoach.toml", toml)
	t.Setenv("STAGECOACH_STUB_OUT", "feat: x")
	writeFile(t, repo, "a.txt", "hi")
	stageFile(t, repo, "a.txt")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--provider", "stub", "--model", "other", "--verbose", "--dry-run"})
	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(errBuf.String(), "DEBUG: note: --model shadowed by [role.message].model") {
		t.Errorf("stderr = %q, want a --model shadow hint", errBuf.String())
	}
}
// (Mirror this for the off/no-flag/no-per-role negatives and the provider-shadow positive.)
```

### Integration Points

```yaml
NO BUILD/BINARY/CONFIG/PRECEDENCE CHANGES:
  - No config-file schema, no new CLI flag, no env var, no migration, no exit-code change.
  - FR-R3 precedence is UNCHANGED (per-role still beats global). The hint is advisory only.
  - internal/ui/verbose.go is UNCHANGED (VerboseWarn/NewVerbose signature/format intact).
  - internal/config/roles.go is UNCHANGED.
  - pkg/stagecoach/stagecoach.go is UNCHANGED.

VERBOSE-PIPELINE CONSISTENCY (the only "integration" affected):
  - runDefault now emits one or two extra `DEBUG: note: …` lines into the SAME stderr verbose
    stream the rest of the run uses (the GenerateCommit DEBUG lines follow, since the hint is emitted
    before the GenerateCommit call). Stream (stderr) and format (`DEBUG: …`) match existing lines.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After the code edit — build + vet the package
go build ./internal/cmd/...
go vet ./internal/cmd/...

# After all edits — whole repo build + lint
go build ./...          # expect: success
make lint               # golangci-lint run (.golangci.yml) — expect: clean

# Confirm the structural change is present (grep gate)
grep -n "ui.NewVerbose" internal/cmd/default_action.go    # runDefault now has a sink (was: only runDecompose)
grep -n "VerboseWarn" internal/cmd/default_action.go      # two hint call sites inside runDefault
grep -n "Changed(\"model\")\|Changed(\"provider\")" internal/cmd/default_action.go   # the two explicit-flag checks

# Expected: zero build/vet/lint errors. If errors exist, READ output and fix before proceeding.
```

### Level 2: Unit/Integration Tests (Component Validation)

```bash
# Targeted: the new shadow-hint tests
go test ./internal/cmd/ -run 'VerboseShadowHint|VerboseProviderShadowHint' -v

# Full cmd package (race)
go test -race ./internal/cmd/...

# Whole repo (race) — the Makefile `test` target
go test -race ./...

# Coverage gate (PRD §20.3, ≥85% on internal/cmd — one of the gated packages)
make coverage-gate

# Expected: all pass. internal/cmd coverage stays ≥85%. If the gate fails, the new tests
# (Task 2) are designed to cover the added branch lines.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the real binary
go build -o /tmp/stagecoach ./cmd/stagecoach

# Throwaway repo + a populated config that shadows --model (the exact PRD footgun)
tmp=$(mktemp -d) && git init -q "$tmp" && cd "$tmp"
git config user.email t@t && git config user.name t
echo a > a.txt && git add . && git commit -qm "init"

# A stub-like provider is easiest via the stubagent binary; for a real binary smoke test use any
# configured stdin provider. The key assertion is the --verbose HINT, which is emitted regardless
# of whether generation actually runs (it fires before GenerateCommit). Minimal repro using a config
# with a [role.message] model and an explicit --model:
cat > .stagecoach.toml <<'EOF'
[provider.stub]
command = "/tmp/stubagent"   # or any stdin provider you have built: go build -o /tmp/stubagent ./cmd/stubagent
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true

[role.message]
model = "config-model"
EOF
echo b > b.txt && git add b.txt   # staged change so the run reaches message-role resolution

# (a) --verbose + --model → the shadow hint MUST appear on stderr
/tmp/stagecoach --provider stub --model flag-model --verbose --dry-run 2>&1 \
  | grep "DEBUG: note: --model shadowed by \[role.message\].model" && echo "OK: model-shadow hint present"

# (b) WITHOUT --verbose → NO hint (the no-noise guarantee)
/tmp/stagecoach --provider stub --model flag-model --dry-run 2>&1 \
  | grep "shadowed" && echo "FAIL: hint leaked without --verbose" || echo "OK: silent without --verbose"

cd - >/dev/null && rm -rf "$tmp"

# Expected: (a) prints the hint; (b) prints the OK (silent) branch.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Stream guard (PRD §15.5): the hint must go to STDERR, never STDOUT (stdout is the pipeable result).
# Re-run (a) above capturing stdout and stderr separately and assert the hint is ONLY on stderr:
#   /tmp/stagecoach ... --verbose --dry-run >/tmp/out 2>/tmp/err
#   grep -q "shadowed" /tmp/err && ! grep -q "shadowed" /tmp/out && echo "OK: stderr-only"

# No-behavior-change guard: the COMMIT actually produced must use the per-role model (FR-R3 unchanged).
# (Run without --dry-run against a real provider and confirm the rendered command in --verbose names
#  the per-role model, NOT the --model value — i.e. the hint is accurate, behavior is unaltered.)

# Race detector is mandatory in this repo; ensure the new Execute-path tests are exercised under -race:
go test -race ./internal/cmd/ -run 'VerboseShadowHint|VerboseProviderShadowHint' -count=3
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `go build ./...` succeeds; `go vet ./internal/cmd/...` clean; `make lint` clean.
- [ ] Level 1 grep gates: `ui.NewVerbose` now in runDefault; two `VerboseWarn` call sites; two `Changed("model"/"provider")` checks.
- [ ] Level 2: `go test -race ./...` green.
- [ ] Level 2: `make coverage-gate` passes (`internal/cmd` ≥85%, PRD §20.3).
- [ ] Level 3: `--verbose --model` against a `[role.message] model` config prints the shadow hint.
- [ ] Level 3: the same command WITHOUT `--verbose` prints NOTHING extra (no-noise guarantee).

### Feature Validation

- [ ] Model hint fires iff `cmd.Flags().Changed("model") && roleModel != cfg.Model`.
- [ ] Provider hint fires iff `cmd.Flags().Changed("provider") && roleProvider != cfg.Provider`.
- [ ] No hint when `--verbose` is off (VerboseWarn no-op).
- [ ] No hint when `--model` absent, OR no per-role message model, OR per-role model == `--model` value.
- [ ] Hint goes to STDERR only (stdout stays the clean result stream).
- [ ] FR-R3 precedence unchanged; no exit-code change; behavior identical when `--verbose` is off.

### Code Quality Validation

- [ ] Follows existing patterns (runDecompose's `ui.NewVerbose(stderr, cfg.Verbose)`; `cmd.Flags().Changed` idiom from hookexec.go; Execute+stub-repo test harness).
- [ ] Sink created in `runDefault` (NOT moved into / duplicated from `runDecompose`).
- [ ] Anti-patterns avoided (see Anti-Patterns): no behavioral change, no decompose-scope creep, no unguarded stderr print, no `cfg.Model != ""` heuristic.
- [ ] No changes to `internal/config/roles.go`, `internal/ui/verbose.go`, `internal/provider/manifest.go`, `pkg/stagecoach/stagecoach.go`, `PRD.md`, `tasks.json`, `prd_snapshot.md`.

### Documentation & Deployment

- [ ] Mode A: `--model` flag help (root.go) has a concise precedence clause.
- [ ] Mode A: `docs/cli.md` (--model row + Message-role resolution) documents the precedence gotcha + the new `--verbose` hint.
- [ ] Mode A: `docs/configuration.md` role-config section cross-references the gotcha.
- [ ] Code is self-documenting: the comment block above the checks explains why runDefault creates a sink and that the hint is verbose-only / advisory.

---

## Anti-Patterns to Avoid

- ❌ Don't emit the hint in `runDecompose` or for the planner/stager/arbiter roles — this PR is scoped to
  `runDefault` + the message role (the single-commit path). Decompose is a separate, four-role surface
  and is out of scope; doing it here is scope creep and more test surface than a 1-point bugfix warrants.
- ❌ Don't create a verbose sink elsewhere and pass it in, or duplicate the shadow logic — create the sink
  inline in `runDefault` (mirroring `runDecompose:399`) right where the checks run. One localized change.
- ❌ Don't detect shadowing via `cfg.Model != ""` or `cfg.Roles["message"].Model != ""` alone. `cfg.Model`
  can come from `[defaults]`/env (intentional — must NOT warn), and a bare per-role-presence check warns
  even when `--model X` equals `[role.message].model X` (no real surprise). Use
  `cmd.Flags().Changed("model") && roleModel != cfg.Model` — it means exactly "the explicit flag will not be used."
- ❌ Don't print the hint with `fmt.Fprintln(stderr, ...)` unguarded by verbose — that leaks it when
  `--verbose` is off. Route through `verbose.VerboseWarn(...)` (it guards `!v.on`).
- ❌ Don't print the hint to stdout — stdout is the pipeable result stream (§15.5). Use the verbose sink
  (which writes to `stderr`).
- ❌ Don't change FR-R3 precedence, add a flag/env/config key, or alter any exit code. This is a
  diagnostics-only, off-by-default hint.
- ❌ Don't change `VerboseWarn`'s signature/format or `roles.go`. Reuse them as-is.
- ❌ Don't edit `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `.gitignore`.
- ❌ Don't skip the "off ⇒ no hint" test (Task 2 TEST B) — that test IS the no-noise guarantee; without it
  a future refactor could regress the off-path silence that is the whole UX point of gating on `--verbose`.
