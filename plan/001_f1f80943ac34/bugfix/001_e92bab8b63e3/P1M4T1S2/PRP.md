# PRP — P1.M4.T1.S2: Tests proving `cfg.Output`/`cfg.StripCodeFence` reach `ParseOutput` end-to-end (Issue 4)

**Issue**: PRD Issue 4 (Minor) — the `[generation]` `output` / `strip_code_fence` fields (file **and**
git-config) were loaded but never applied. The bridge that applies them landed in the sibling subtask
**P1.M4.T1.S1** (`buildDeps`). This subtask is **TEST-ONLY**: it proves, end-to-end through
`GenerateCommit`, that those config knobs now flow through `buildDeps` into the manifest and are honored
by `provider.ParseOutput`.
**PRD refs**: §16.2 (`[generation] output`, `[generation] strip_code_fence`), §16.1 (layer-1 defaults),
§16.3 (git-config keys `stagecoach.output` / `stagecoach.stripCodeFence`), §12.9 (parse uses the manifest's
`output`/`strip_code_fence`).
**Binding decisions**: `plan/.../bugfix/001_e92bab8b63e3/architecture/decisions.md` **D4**;
`.../architecture/seam_config_and_autostage.md` **Part A** ("Tests covering these paths").
**Dependency**: **P1.M4.T1.S1** (Complete) — the `buildDeps` cfg→manifest bridge already ships in
`pkg/stagecoach/stagecoach.go` (lines ~196-207). This subtask adds NO production code; it adds the missing
end-to-end tests the contract calls out.

---

## Goal

**Feature Goal**: Close the **test-coverage gap** identified in the Issue-4 contract & Part A: today
`internal/provider/parse_test.go` exercises `ParseOutput` only via hand-built `Manifest` values, and the
`internal/config/*_test.go` loader tests assert only that `cfg.Output`/`cfg.StripCodeFence` are
**populated** — **no test** asserts those values actually reach `ParseOutput` through
`buildDeps`/`GenerateCommit`. Add a focused battery in `pkg/stagecoach/stagecoach_test.go` that drives the
**real** `GenerateCommit` (DryRun) through the real `config.Load` (file + git-config layers) **and**
through the injected-config path, asserting the parsed commit message reflects JSON parsing and/or
fence-retention per the `[generation]`/git-config setting **overriding** the resolved manifest — and that
the manifest's own value wins when the cfg field is unset.

**Deliverable**: One **test-only** change to `pkg/stagecoach/stagecoach_test.go` — ~4 new `Test…`
functions (plus optional small helpers) that are green under `go test -race`. No production-code edits,
no new packages, no doc edits (DOCS: none — test-only per contract item 5).

**Success Definition**:
- The new tests **fail** if the S1 `buildDeps` bridge is reverted (i.e. they genuinely exercise the seam,
  not a tautology) — see the "TDD / failing-test-first" note in contract item 3.
- The new tests **pass** with the shipped bridge.
- `go build ./...`, `go vet ./...`, `go test -race ./...`, `make lint`, `make coverage-gate` are all green.
- No existing test regresses (the suite is additive — new `Test…` functions only).

## Why

- **Regression value**: Issue 4 was a *silent no-op* — config knobs were populated everywhere and asserted
  by loader tests, yet never consumed. The S1 bridge is ~6 lines; without an end-to-end test, a future
  refactor could silently re-drop the bridge and every loader test would still pass. This battery pins
  the cfg→manifest→ParseOutput contract at the only seam that proves it.
- **Scope respect**: This is strictly the **test** half of Issue 4. It does **not** touch the production
  bridge (S1 owns it), does **not** alter `ParseOutput`/manifest/registry/loaders, and does **not** fix
  the `file.go` "cannot set `strip_code_fence=false` via file" quirk (out of scope — that's why the
  false-case is exercised via **git-config**, which *can* set false).

## What

### What "fixed" looks like (the behavior the tests must prove)

These are the four scenarios the battery covers. All run `GenerateCommit(ctx, Options{Provider:"stub",
DryRun:true})` with a stub agent whose stdout is a controlled blob, and assert on `Result.Message`.

1. **File `[generation] output="json"` overrides the manifest's `output="raw"`.**
   A `.stagecoach.toml` declares `[provider.stub] output="raw"` + `json_field="subject"` and a
   `[generation] output="json"` block. The stub emits `{"subject":"feat: from json config"}`. The
   parsed message is `feat: from json config` (the extracted JSON field) — **not** the raw JSON blob.
   *(Proves the file-loader path.)*
2. **Git-config `stagecoach.output json` overrides the manifest's `output="raw"`.**
   Same manifest, no `[generation]` block; instead `git config --local stagecoach.output json`. Stub emits
   the JSON blob → parsed message is the extracted field. *(Proves the git-config loader path for Output.)*
3. **Git-config `stagecoach.stripCodeFence false` overrides the manifest's `strip_code_fence=true`.**
   Manifest `strip_code_fence=true`; `git config --local stagecoach.stripCodeFence false`. Stub emits a
   fenced block → the parsed message **retains** the ``` fences. *(Proves the git-config loader path for
   StripCodeFence — the false-case the file loader cannot express.)*
4. **The manifest's `output="json"` wins when the cfg `Output` field is unset (empty).**
   Inject `Options.Config` with `Output:""` (bypassing `config.Load`/`Defaults`) and a `[provider.stub]
   output="json"` manifest. Stub emits JSON → parsed message is the extracted field. *(Proves the
   `buildDeps` guard's inherit branch — see the CRITICAL note below; this is the only faithful way to
   satisfy the contract's "per-manifest value still wins when `[generation]` is unset".)*

### Success Criteria

- [ ] Four new `Test…Issue4` functions in `pkg/stagecoach/stagecoach_test.go`, all green under
      `go test -race ./pkg/stagecoach/ -run Issue4 -v`.
- [ ] Tests exercise the **real** `GenerateCommit` DryRun path (which calls `buildDeps` → bridge →
      `ParseOutput`), not a direct `ParseOutput` call.
- [ ] Tests 1–2 assert the parsed message is the **extracted JSON field**, distinct from the raw blob
      (so they fail if JSON parsing did not engage).
- [ ] Test 3 asserts the parsed message **contains** ``` (fence retained), which fails if stripping were
      still on.
- [ ] Test 4 injects `cfg.Output=""` and asserts the manifest's `output="json"` wins.
- [ ] At least one of the override tests is shown to **fail** when the S1 bridge is temporarily removed
      (manual TDD check; document the verification, do not commit a broken bridge).
- [ ] No production code touched; no other test files modified.

## All Needed Context

### Context Completeness Check

✅ Passes the "No Prior Knowledge" test: the exact seam (the S1 `buildDeps` bridge, with line cites), the
exact consumer (`ParseOutput`), the exact loader wiring (file Layer 3 / git Layer 4), the exact stub
harness (`stubtest.Build` + `STAGECOACH_STUB_OUT`), the test-file patterns to mirror, the TOML-escaping
gotcha, the DryRun plumbing, and the precise validation commands are all specified below with code.

### Documentation & References

```yaml
# MUST READ — the bridge UNDER TEST (shipped by S1; do NOT modify, only exercise it)
- file: pkg/stagecoach/stagecoach.go
  lines: 154-209 (buildDeps); the cfg→manifest bridge is the block ~196-207 between the IsInstalled
         pre-flight and the `return generate.Deps{...}`.
  why: This is THE seam. cfg.Output (guarded on != "") and cfg.StripCodeFence (unconditional) are copied
       onto the manifest's pointer fields here; ParseOutput reads those pointers. The tests prove the
       copy actually happens end-to-end.
  critical: Two asymmetries the tests MUST reflect — (a) Output is guarded `if cfg.Output != ""` so an
            EMPTY cfg.Output lets the manifest win (test 4); (b) StripCodeFence is applied UNCONDITIONALLY
            so the cfg value ALWAYS wins (there is no "manifest wins" case for StripCodeFence).

# MUST READ — the consumer (do NOT modify; confirm it reads these pointer fields)
- file: internal/provider/parse.go
  lines: 44-67 (ParseOutput: Resolve(); Step 2 `if *r.StripCodeFence { stripCodeFence(s) }`;
         Step 3 `switch *r.Output { case "json": parseJSON(...); case "raw": msg=s }`)
  why: Confirms ParseOutput reads ONLY the manifest's resolved Output/StripCodeFence. No parser change is
       needed — once buildDeps writes the pointers, ParseOutput honors them.
  gotcha: `parseJSON(s, *r.JsonField)` needs a NON-EMPTY json_field to extract anything — so every JSON
          test's manifest MUST set `json_field` (e.g. "subject"), else extraction fails → fallback to raw
          and the test sees the raw blob instead of the field. (parse.go:71-93.)

# MUST READ — how the manifest gets its fields (pointer semantics + registry merge)
- file: internal/provider/manifest.go
  lines: 78-80 (Output *string, JsonField *string, StripCodeFence *bool); 137-167 (Resolve: fills nils,
         PRESERVES present values); 95-127 (Validate rejects invalid Output enum but is nil-tolerant).
  why: Explains the *string/*bool override-signal design (non-nil ⇒ override). Resolve() preserves the
       cfg-written pointers, so ParseOutput sees them.
- file: internal/provider/registry.go
  lines: 41-63 (NewRegistry: a user-defined name with NO built-in base is added VERBATIM —
         `override.Name = name; manifests[name] = override`); 65-69 (Get).
  why: Confirms an injected `Providers["stub"]` (no built-in "stub" exists) survives into buildDeps and
       Validate passes if Command is set. Required for test 4 (injected manifest).
- file: internal/provider/merge.go
  lines: 28-92 (MergeManifest regime 1: a NON-NIL override pointer WINS — explicit ""/false included).
  why: Confirms a `[provider.stub] output="raw"` + `json_field="subject"` in .stagecoach.toml produces a
       manifest whose Output="raw" / JsonField="subject" survive the merge — so the bridge can override
       Output to "json" while JsonField="subject" is preserved for parseJSON.

# MUST READ — the config fields + how loaders populate them
- file: internal/config/config.go
  lines: 31-32 (Output string; StripCodeFence bool); 62-75 (Defaults: Output "raw", StripCodeFence true).
  why: cfg.Output is ALWAYS non-empty ("raw") in production (config.Load calls Defaults() first). This is
       WHY test 4 must inject Output="" directly — see the CRITICAL note.
- file: internal/config/load.go
  lines: 56-101 (Load: Layer1 Defaults → Layer2 global TOML → Layer3 repo-local .stagecoach.toml →
         Layer4 loadGitConfig(repoDir) → Layer5 env → Layer7 flags). Output/StripCodeFence are set by
         Layers 1/2/3/4 only (no env/flag).
  why: Tests 1–3 rely on config.Load running (Options.Config == nil) so Layers 3+4 are honored. Test 4
       sets Options.Config non-nil so resolveConfig SKIPS config.Load (no Defaults → Output can be "").
- file: internal/config/git.go
  lines: 113-129 (gitConfigGet(repoDir,"stagecoach.output") via `git -C <repo> config --get`);
         147-156 (gitConfigBool(repoDir,"stagecoach.stripCodeFence") via `git config --get --bool`).
  why: Confirms the git-config layer reads REPO-LOCAL config — so a test's
       `git -C <repo> config stagecoach.output json` / `stagecoach.stripCodeFence false` is read back. The
       bool key is CAMELCASE (stagecoach.stripCodeFence), matching git_test.go:154.

# MUST READ — the test harness to reuse
- file: pkg/stagecoach/stagecoach_test.go
  lines: 28-118 (fixture helpers: initRepo, writeFile, stageFile, runGit, headSHA, setupTestRepo,
         setupScriptedRepo) and 130-150 (objectCountLine/looseObjectTypes).
  why: The new tests REUSE initRepo/writeFile/stageFile/runGit verbatim (they are package-private helpers
         already in this file). Match setupTestRepo's chdir-into-temp-repo + t.Cleanup(restore) pattern.
  pattern: TestGenerateCommit_MissingProviderCommand_Issue3 (485-589) is the CLOSEST analogue — it inlines
           a hand-written .stagecoach.toml, chdirs, stages a file, and asserts on the GenerateCommit
           result/error. Mirror its structure for tests 1–3.
  pattern: TestResolveConfig_InjectedConfig (592-690) shows how to inject a config.Config via Options.Config
           (the Providers map shape) — mirror it for test 4.
- file: internal/stubtest/stubtest.go
  lines: 18-32 (Options: Out/Script/Output/StripCodeFence); 62-80 (optsEnvMap → STAGECOACH_STUB_OUT for
         single-response mode); 100-120 (Build compiles ./cmd/stubagent once per process).
  why: stubtest.Build(t) returns the stub binary path. The stub reads STAGECOACH_STUB_OUT from its env and
       writes EXACTLY that to stdout (no added newline). In .stagecoach.toml, set it via the
       [provider.stub.env] table.
- file: cmd/stubagent/main.go
  lines: 40-60 (drains stdin; reads STAGECOACH_STUB_OUT or STAGECOACH_STUB_SCRIPT; writes stdout; exits).
  why: Confirms the stub is a faithful stand-in: it is invoked through provider.Execute exactly like a
       real agent, so ParseOutput receives its stdout through the real pipeline.
- file: internal/config/git_test.go
  lines: 44-52 (setGitConfig helper: `git -C <dir> config <key> <value>`); 71 (t.Setenv("HOME",
         t.TempDir()) to ISOLATE the global git config).
  why: Reuse the `git -C <repo> config <key> <value>` form for tests 2–3. The HOME-isolation trick is
       RECOMMENDED for the pkg/stagecoach git-config tests too (avoids a stray global ~/.gitconfig
       `stagecoach.*` value leaking into the merged `config --get` read).

# Binding architecture (context only — already implemented)
- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/decisions.md
  section: D4 — Apply [generation] output/strip_code_fence onto the manifest (not remove).
  why: States the exact bridge, the "broader setting wins" precedence, the Output-guard /
       StripCodeFence-unconditional asymmetry, and the explicit "file-loader false-set quirk is OUT OF
       SCOPE" note. The tests must reflect all three.
- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/seam_config_and_autostage.md
  section: PART A — "Tests covering these paths".
  why: The scout report explicitly says: "No test asserts that cfg.Output/cfg.StripCodeFence ever reach
       ParseOutput … add a test in pkg/stagecoach/stagecoach_test.go that sets [generation] output="json" +
       strip_code_fence=false in config and asserts the resulting deps.Manifest carries those values (and
       that ParseOutput then honors them)." This PRP is that test.
```

### Current Codebase tree (relevant slice)

```bash
pkg/stagecoach/
  stagecoach.go          # buildDeps bridge (S1, ~196-207) — UNDER TEST, do not edit
  stagecoach_test.go     # ADD the 4 Issue-4 tests here (reuses initRepo/writeFile/stageFile/runGit)
internal/provider/
  parse.go              # ParseOutput — reads m.Output/.m.StripCodeFence (NO edit)
  manifest.go           # pointer fields + Resolve (NO edit)
  registry.go, merge.go # user-defined provider + field-merge (NO edit)
internal/config/
  config.go, load.go, git.go, file.go  # loaders that populate cfg (NO edit)
internal/stubtest/stubtest.go          # stubtest.Build + Options (reuse, NO edit)
cmd/stubagent/main.go                  # the fake agent (NO edit)
```

### Desired Codebase tree (files MODIFIED — no new files)

```bash
pkg/stagecoach/stagecoach_test.go   # +~4 Test…Issue4 functions (+ optional small helpers). TEST-ONLY.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL — the cfg.Output="" subtlety (test 4). config.Defaults() ALWAYS sets Output="raw"
// (config.go:73), and config.Load() applies Defaults() FIRST. So in PRODUCTION cfg.Output is NEVER "".
// The buildDeps bridge guards Output on `cfg.Output != ""` (stagecoach.go:204). The ONLY way to exercise
// the guard's "manifest wins" branch is to inject Options.Config directly (resolveConfig then does
// `cfg = *opts.Config` and SKIPS config.Load/Defaults), with Output explicitly set to "". That is what
// test 4 does. NOTE for reviewers: cfg.Output="" never occurs via the real loaders — this test pins the
// bridge's guard contract (a deliberate regression guard), it is NOT claiming a production code path.
//
// PRODUCTION REALITY (document so it is not misread): when `[generation] output` is ABSENT in a real
// config file, config.Load yields cfg.Output="raw" (the Layer-1 default), which the bridge then copies
// onto the manifest — i.e. the [generation] DEFAULT ("raw") overrides a per-provider output="json"
// (decision D4: "[generation] value wins as the broader setting"). So a per-provider output="json" only
// takes effect if the user ALSO sets [generation] output="json". Test 4 isolates the empty-field case
// precisely because that is the only state in which the manifest's own Output survives.

// CRITICAL — StripCodeFence is applied UNCONDITIONALLY (stagecoach.go:208-209; no guard). There is NO
// "manifest wins" case for StripCodeFence: cfg.StripCodeFence (always a concrete bool post-Defaults)
// ALWAYS overrides the manifest. Do NOT write a test claiming "manifest strip_code_fence wins when
// [generation] unset" — it cannot (by D4 design). The contract's "per-manifest wins" clause is satisfied
// by test 4 for the OUTPUT field only.

// CRITICAL — file.go CANNOT set strip_code_fence=false (file.go:153 v1 quirk:
//   if g.StripCodeFence { c.StripCodeFence = true }   // cannot set false via file
// ). That is why the fence-retention test (test 3) uses the GIT-CONFIG loader
// (`git config stagecoach.stripCodeFence false`), which CAN set false. Do NOT try to test fence-retention
// via a `[generation] strip_code_fence = false` line in .stagecoach.toml — it silently stays true.

// CRITICAL — TOML escaping for the stub output. The stub's stdout is delivered via the
// [provider.stub.env] STAGECOACH_STUB_OUT entry in .stagecoach.toml. A JSON blob contains double-quotes
// and a fenced blob contains backticks; placing these in a TOML *basic* string ("...") breaks parsing
// (the inner quote closes the string). Use a TOML *literal* string (single-quotes '...') for these
// values — literal strings allow " ` { } unescaped. Example file content:
//   STAGECOACH_STUB_OUT = '{"subject":"feat: from json config"}'
// In the Go test, build that line so the single-quotes are literal TOML delimiters and the JSON quotes
// are Go-escaped:  "\"STAGECOACH_STUB_OUT = '{\\\"subject\\\":\\\"feat: from json config\\\"}'\\n\""
// (or, cleaner, assemble the TOML with fmt.Sprintf and pass the JSON via a Go raw string `` ` ``.)

// CRITICAL — every JSON test's manifest MUST set json_field (e.g. json_field = "subject"). parseJSON
// extracts obj[json_field]; an empty json_field ⇒ extraction fails ⇒ ParseOutput FELLBACK to raw and the
// test would see the whole JSON blob, falsely appearing to fail. Set json_field in [provider.stub].

// GOTCHA — isolate the global git config for the git-config tests (tests 2–3). `git config --get`
// (git.go:46) reads the MERGED view (local + global + system). A stray global ~/.gitconfig
// `stagecoach.output` would leak in. Mirror internal/config/git_test.go:71: call
// `t.Setenv("HOME", t.TempDir())` before running git config, so the global scope is empty and only the
// repo-local value (written via `git -C <repo> config ...`) is seen. (Existing stagecoach tests skip this
// and rely on a clean dev env; add it here for determinism.)

// GOTCHA — the DryRun path STILL requires a non-empty staged diff. runPipeline returns ErrNothingToCommit
// if `deps.Git.StagedDiff` is "" (stagecoach.go runPipeline step 2). So every test MUST stage a NEW file
// (writeFile + stageFile) before calling GenerateCommit, exactly like TestGenerateCommit_DryRun.

// GOTCHA — keep the stub's emitted subject UNIQUE vs. the repo's existing commits, or the DryRun
// dedupe loop (FR30–FR33, now active on dry-run per Issue 2) will reject it and retry. The repo's only
// prior commit is "initial", so any "feat: ..." subject is safe; just don't reuse "initial".

// GOTCHA — fence-retention assertion choice. Use a MULTI-LINE fenced block for test 3 so the pass/fail
// signal is unambiguous: with strip ON, "```\nfeat: x\n```" → "feat: x" (fence removed, non-empty,
// succeeds); with strip OFF → "```\nfeat: x\n```" (fence kept). Asserting the message CONTAINS "```"
// cleanly distinguishes the two without relying on error behavior. (A single-line ```x``` with strip ON
// strips to "" → ok=false → rescue error → a noisier failure; multi-line is the precise signal.)
```

## Implementation Blueprint

### Test data shapes (no new production types — these are test-local)

The battery varies three knobs. Use the existing `stubtest.Build(t)` stub and the existing
`initRepo`/`writeFile`/`stageFile`/`runGit` helpers (already in `stagecoach_test.go`). For tests 1–3 the
manifest + stub output are encoded into a repo-local `.stagecoach.toml`; for test 4 the manifest is an
injected `config.Config`.

```go
// The stub emits, per test, one of:
jsonBlob := `{"subject":"feat: from json config"}`          // tests 1, 2, 4 (json_field="subject")
fencedBlock := "```\nfeat: keep the fence\n```"             // test 3 (strip_code_fence=false keeps it)

// The manifest under test always starts at the "raw / strip-on" baseline that the cfg layer overrides:
//   [provider.stub]
//   command = "<stub bin>"
//   prompt_delivery = "stdin"
//   output = "raw"            # the value the cfg layer must OVERRIDE
//   json_field = "subject"    # REQUIRED for json extraction (else parseJSON falls back to raw)
//   strip_code_fence = true   # the value the cfg layer must OVERRIDE (test 3)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: READ-ONLY orientation (no edits)
  - READ pkg/stagecoach/stagecoach.go buildDeps (154-209): confirm the S1 bridge is present (the
    `if cfg.Output != "" { o := cfg.Output; m.Output = &o }` + `scf := cfg.StripCodeFence;
    m.StripCodeFence = &scf` block). If it is MISSING, STOP — S1 is not actually complete and these
    tests cannot be the "green" deliverable; surface it instead.
  - READ pkg/stagecoach/stagecoach_test.go: confirm initRepo/writeFile/stageFile/runGit/headSHA exist
    (package-private) and that TestGenerateCommit_MissingProviderCommand_Issue3 + TestResolveConfig_InjectedConfig
    are the patterns to mirror.

Task 1: ADD TestGenerateCommit_GenerationConfigFile_OutputJSON_Issue4 (file-loader override)
  - IMPLEMENT: a repo with .stagecoach.toml carrying [provider.stub] (output="raw", json_field="subject",
    strip_code_fence=true, command=stub bin, [provider.stub.env] STAGECOACH_STUB_OUT = '<jsonBlob>') AND a
    [generation] output = "json" block. Stage a new file. Call GenerateCommit(DryRun:true, Provider:"stub")
    (do NOT set Options.Config — let config.Load read the file). Assert res.Message == "feat: from json config"
    (the extracted field, NOT the raw blob) and res.CommitSHA == "".
  - FOLLOW pattern: TestGenerateCommit_MissingProviderCommand_Issue3 (inline TOML + chdir + stage + assert).
  - NAMING/PLACEMENT: top-level func in pkg/stagecoach/stagecoach_test.go.
  - TOML: use a literal string for STAGECOACH_STUB_OUT (single-quotes) so the JSON quotes survive.

Task 2: ADD TestGenerateCommit_GitConfig_OutputJSON_Issue4 (git-config override of Output)
  - IMPLEMENT: .stagecoach.toml with [provider.stub] (output="raw", json_field="subject",
    strip_code_fence=true, STAGECOACH_STUB_OUT='<jsonBlob>') and NO [generation] block. Then
    `git -C <repo> config stagecoach.output json`. t.Setenv("HOME", t.TempDir()) first (isolate global).
    Stage a file. GenerateCommit(DryRun:true, Provider:"stub"). Assert res.Message == extracted field.
  - FOLLOW pattern: setGitConfig form from internal/config/git_test.go:44 (`git -C <repo> config <k> <v>`),
    via the existing runGit helper or exec.Command.
  - KEY: proves Layer-4 git-config feeds cfg.Output through buildDeps to ParseOutput.

Task 3: ADD TestGenerateCommit_GitConfig_StripCodeFenceFalse_Issue4 (git-config override of StripCodeFence)
  - IMPLEMENT: .stagecoach.toml with [provider.stub] (output="raw", strip_code_fence=true,
    STAGECOACH_STUB_OUT = '''<fencedBlock>''' as a TOML MULTI-LINE literal string so the newlines survive).
    Then `git -C <repo> config stagecoach.stripCodeFence false` (camelCase). t.Setenv("HOME", t.TempDir()).
    Stage a file. GenerateCommit(DryRun:true, Provider:"stub"). Assert strings.Contains(res.Message, "```")
    is TRUE (the fence was RETAINED because cfg.StripCodeFence=false overrode the manifest's true).
  - FOLLOW pattern: same as Task 2; the multi-line literal string is:
        STAGECOACH_STUB_OUT = '''
        ```
        feat: keep the fence
        ```
        '''
    (go-toml strips the single newline immediately after the opening '''; trailing whitespace is trimmed
    by ParseOutput step 1.)
  - KEY: proves the false-case the file loader cannot express; uses the camelCase git key.

Task 4: ADD TestGenerateCommit_ManifestOutputWins_WhenCfgOutputEmpty_Issue4 (guard inherit branch)
  - IMPLEMENT: NO .stagecoach.toml. Inject Options.Config built from config.Defaults() with Output set to ""
    and a Providers["stub"] map carrying {command, prompt_delivery:"stdin", output:"json",
    json_field:"subject", strip_code_fence:true, env:{STAGECOACH_STUB_OUT: jsonBlob}}. Build a real repo
    (initRepo + one commit + a staged new file) — runPipeline needs a real git repo. Call
    GenerateCommit(ctx, Options{Config:&cfg, Provider:"stub", DryRun:true}). Assert res.Message == extracted
    field (the manifest's output="json" won because cfg.Output=="" ⇒ the `!= ""` guard fell through).
  - FOLLOW pattern: TestResolveConfig_InjectedConfig (592-690) for the injected-config + Providers-map shape.
  - GOTCHA: the env sub-table is injected as Providers["stub"]["env"] = map[string]any{"STAGECOACH_STUB_OUT": ...};
    DecodeUserOverrides round-trips it into Manifest.Env (same path the file loader uses — verified by
    setupTestRepo passing). Use a Go raw string `` `{"subject":...}` `` for the JSON value (no escaping).
  - COMMENT: add the CRITICAL note (from the gotchas block) explaining cfg.Output="" never occurs via the
    real loaders and this pins the bridge's guard contract.

Task 5: VERIFY the TDD property (failing-test-first) — manual, do NOT commit a broken bridge
  - TEMPORARILY comment out the S1 bridge block in buildDeps; re-run the 4 new tests; confirm tests 1–3
    FAIL (raw blob / stripped fence observed instead of the override) and test 4 still passes (it tests
    the guard's ABSENCE-of-override path). Restore the bridge. Record the result in the test's doc comment
    or the task notes. (This is the contract's "failing test first" proof that the tests genuinely cover
    the seam — they are not tautologies.)

Task 6: VALIDATE — run the gates (see Validation Loop). All green, no regressions.
```

### Implementation Patterns & Key Details

```go
// PATTERN — the inline-TOML repo setup (tests 1–3), mirroring TestGenerateCommit_MissingProviderCommand_Issue3.
// Use TOML LITERAL strings (single-quote) for the stub output so JSON quotes / backticks survive parsing.
func TestGenerateCommit_GenerationConfigFile_OutputJSON_Issue4(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	jsonOut := `{"subject":"feat: from json config"}` // Go raw string — no escaping
	// NB: the STAGECOACH_STUB_OUT line uses TOML literal quotes '...' so the JSON "..." inside is literal.
	toml := "[provider.stub]\n" +
		"command = \"" + bin + "\"\n" +
		"prompt_delivery = \"stdin\"\n" +
		"output = \"raw\"\n" +            // manifest baseline — [generation] must override this
		"json_field = \"subject\"\n" +    // REQUIRED: parseJSON extracts obj["subject"]
		"strip_code_fence = true\n" +
		"\n[provider.stub.env]\n" +
		"STAGECOACH_STUB_OUT = '" + jsonOut + "'\n" + // literal string: '{"subject":"..."}'
		"\n[generation]\n" +
		"output = \"json\"\n" // the [generation] override
	if err := os.WriteFile(repo+"/.stagecoach.toml", []byte(toml), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "data")
	stageFile(t, repo, "new.txt")

	wd, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun)", res.CommitSHA)
	}
	// The JSON field was extracted ⇒ the [generation] output="json" overrode the manifest's "raw".
	// (If the bridge were absent, res.Message would equal the raw jsonOut blob.)
	if res.Message != "feat: from json config" {
		t.Errorf("Message = %q, want %q ([generation] output=json must make ParseOutput extract the JSON field)",
			res.Message, "feat: from json config")
	}
}

// PATTERN — the git-config override (test 2/3). Isolate HOME so a global stagecoach.* can't leak in.
func TestGenerateCommit_GitConfig_OutputJSON_Issue4(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate global git config (FINDING E; mirrors git_test.go:71)
	bin := stubtest.Build(t)
	repo := t.TempDir()
	jsonOut := `{"subject":"feat: from git-config json"}`
	toml := "[provider.stub]\n" +
		"command = \"" + bin + "\"\n" +
		"prompt_delivery = \"stdin\"\n" +
		"output = \"raw\"\n" +
		"json_field = \"subject\"\n" +
		"strip_code_fence = true\n" +
		"\n[provider.stub.env]\n" +
		"STAGECOACH_STUB_OUT = '" + jsonOut + "'\n"
	if err := os.WriteFile(repo+"/.stagecoach.toml", []byte(toml), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	runGit(t, repo, "config", "stagecoach.output", "json") // Layer-4 override (camelCase not needed for this key)
	writeFile(t, repo, "new.txt", "data")
	stageFile(t, repo, "new.txt")

	wd, _ := os.Getwd()
	os.Chdir(repo)
	t.Cleanup(func() { os.Chdir(wd) })

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}
	if res.Message != "feat: from git-config json" {
		t.Errorf("Message = %q, want %q (git config stagecoach.output=json must reach ParseOutput)",
			res.Message, "feat: from git-config json")
	}
}

// PATTERN — the injected-config inherit test (test 4). Bypass config.Load so Output can be "".
func TestGenerateCommit_ManifestOutputWins_WhenCfgOutputEmpty_Issue4(t *testing.T) {
	bin := stubtest.Build(t)
	// Start from Defaults (so Timeout/MaxDuplicateRetries/etc. are sane), then ZERO Output to exercise
	// the bridge's `if cfg.Output != ""` guard. In production cfg.Output is ALWAYS non-empty ("raw"),
	// because config.Load applies Defaults() first; this injection is the only way to reach the guard's
	// "manifest wins" branch and pins that contract.
	cfg := config.Defaults()
	cfg.Provider = "stub"
	cfg.Output = "" // "[generation] output unset" at the field level
	cfg.Providers = map[string]map[string]any{
		"stub": {
			"command":          bin,
			"prompt_delivery":  "stdin",
			"output":           "json", // the manifest's own value — must win because cfg.Output==""
			"json_field":       "subject",
			"strip_code_fence": true,
			"env": map[string]any{
				"STAGECOACH_STUB_OUT": `{"subject":"feat: manifest wins when cfg unset"}`,
			},
		},
	}
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "data")
	stageFile(t, repo, "new.txt")

	wd, _ := os.Getwd()
	os.Chdir(repo)
	t.Cleanup(func() { os.Chdir(wd) })

	res, err := GenerateCommit(context.Background(), Options{Config: &cfg, Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}
	// manifest output="json" won (cfg.Output="" ⇒ guard fell through) ⇒ JSON extracted.
	if res.Message != "feat: manifest wins when cfg unset" {
		t.Errorf("Message = %q, want %q (manifest output=json must win when cfg.Output is empty)",
			res.Message, "feat: manifest wins when cfg unset")
	}
}
```

### Integration Points

```yaml
TEST FILE:
  - file: pkg/stagecoach/stagecoach_test.go
    change: "+4 Test…Issue4 functions (optionally +1 small TOML-builder helper). TEST-ONLY."
    risk: ADDITIVE only — new top-level test functions; no edits to existing tests or production code.
          Reuses package-private helpers (initRepo/writeFile/stageFile/runGit/commitRaw) already present.

NO PRODUCTION CODE. NO DOC EDITS (DOCS: none — test-only, per contract item 5). NO NEW PACKAGES.
NO NEW DEPENDENCIES (uses only testing, os, os/exec, strings, context, and the existing internal/* +
cmd/stubagent already imported/built by this test file).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After adding the tests — fix before proceeding.
go build ./...          # compiles (the new tests are in a _test.go file; `go build` still type-checks deps)
go vet ./...            # vet clean
make lint               # golangci-lint — zero findings (watch for: unused vars, shadowing, error-checks)
# Expected: zero errors. Common first-pass issues: a TOML literal-string quote mistake (parse error at
# config.Load time surfaces as a GenerateCommit error inside the test, not a compile error — see Level 2).
```

### Level 2: The new tests (Component Validation)

```bash
# Run ONLY the four new tests, verbosely.
go test -race ./pkg/stagecoach/ -run 'Issue4' -v
# Expected: 4 PASS. If a JSON test reports Message == the raw JSON blob, the override did NOT engage —
# re-check (a) the [generation]/git-config line is actually present, (b) json_field="subject" is set,
# (c) the S1 bridge block is present in buildDeps. If the fence test reports a stripped message, the
# git-config bool key is wrong (must be stagecoach.stripCodeFence, camelCase) or HOME wasn't isolated.

# Sanity: confirm the tests genuinely exercise the seam (TDD failing-test-first, contract item 3).
# Temporarily comment out the S1 bridge in buildDeps (~196-207) and re-run:
#   go test -race ./pkg/stagecoach/ -run 'Issue4' -v
# Expected now: tests 1, 2 FAIL (raw blob observed), test 3 FAIL (fence stripped), test 4 STILL PASSES
# (it tests the guard's no-override path). Restore the bridge afterward. Do NOT commit the removal.
```

### Level 3: Full suite + regression (System Validation)

```bash
# Full race suite — nothing else regresses (the change is additive).
go test -race ./...
# Expected: all packages pass. A failure elsewhere means a test accidentally perturbed shared state
# (most likely: a chdir not restored, or a leaked STAGECOACH_/HOME env var) — re-check t.Cleanup/t.Setenv.

# The pre-existing buildDeps-tail test must stay green (proves the bridge placement is intact):
go test -race ./pkg/stagecoach/ -run TestGenerateCommit_MissingProviderCommand_Issue3 -v
```

### Level 4: Coverage gate (PRD §20.3)

```bash
# pkg/stagecoach is NOT in the gate set (internal/{git,provider,generate,config} are), so these tests do
# not move the gate number — but confirm no regression and that pkg/stagecoach coverage is non-decreasing:
make coverage-gate     # all 4 core packages still >= 85%
go test -cover ./pkg/stagecoach/   # advisory: confirm the new tests lifted pkg/stagecoach coverage
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean.
- [ ] `go vet ./...` clean.
- [ ] `go test -race ./...` — all packages pass.
- [ ] `make lint` — zero findings.
- [ ] `make coverage-gate` — all 4 core packages >= 85% (no regression).

### Feature Validation
- [ ] 4 new `Test…Issue4` functions, all green under `go test -race ./pkg/stagecoach/ -run Issue4 -v`.
- [ ] Test 1: `[generation] output="json"` (file) → JSON field extracted (not the raw blob).
- [ ] Test 2: `git config stagecoach.output json` → JSON field extracted.
- [ ] Test 3: `git config stagecoach.stripCodeFence false` → ``` fences RETAINED in the message.
- [ ] Test 4: injected `cfg.Output=""` → manifest `output="json"` wins (guard inherit branch).
- [ ] TDD check performed: tests 1–3 fail when the S1 bridge is removed (genuine seam coverage).
- [ ] No production code, no other test files, no doc files modified.

### Code Quality Validation
- [ ] Reuses existing `initRepo`/`writeFile`/`stageFile`/`runGit`/`commitRaw` helpers (no duplication).
- [ ] Mirrors `TestGenerateCommit_MissingProviderCommand_Issue3` (inline TOML) and
      `TestResolveConfig_InjectedConfig` (injection) patterns.
- [ ] Every `os.Chdir` is paired with a `t.Cleanup(func(){ os.Chdir(wd) })`.
- [ ] Git-config tests call `t.Setenv("HOME", t.TempDir())` to isolate the global git config.
- [ ] JSON test manifests all set `json_field`; stub outputs use TOML literal strings for special chars.

### Documentation & Boundaries
- [ ] Test 4 carries the CRITICAL comment explaining cfg.Output="" never occurs via real loaders and that
      this pins the bridge's guard contract (avoids a reviewer misreading it as a production path).
- [ ] Scope respected: test-only; the `file.go` false-set quirk is NOT touched (fence-false tested via
      git-config only); `ParseOutput`/manifest/registry/loaders are NOT modified.

---

## Anti-Patterns to Avoid

- ❌ Don't test `ParseOutput` directly with a hand-built manifest — that's what `parse_test.go` already
  does and it does NOT prove the cfg→manifest bridge. Drive `GenerateCommit` end-to-end.
- ❌ Don't set `Options.Config` for the file/git-config tests (1–3) — that SKIPS `config.Load`, so the
  `.stagecoach.toml` / `git config` values would never be read. Only test 4 injects config.
- ❌ Don't forget `json_field` on JSON-test manifests — without it parseJSON falls back to raw and the test
  sees the raw blob (a false "bridge broken" signal).
- ❌ Don't put JSON/fence content in a TOML *basic* string (`"..."`) — the inner quotes break parsing. Use a
  TOML *literal* string (`'...'`), or a multi-line literal (`'''...'''`) for newlines.
- ❌ Don't test fence-retention (`strip_code_fence=false`) via the `[generation]` file block — `file.go`
  can't set false (v1 quirk). Use `git config stagecoach.stripCodeFence false`.
- ❌ Don't write a "manifest StripCodeFence wins" test — impossible by design (StripCodeFence is applied
  unconditionally; D4). The contract's "per-manifest wins" clause is satisfied by test 4 (Output) only.
- ❌ Don't skip staging a new file before `GenerateCommit` — DryRun still returns `ErrNothingToCommit` on an
  empty diff, which would mask the real assertion.
- ❌ Don't reuse a subject that duplicates the repo's existing commit ("initial") — the DryRun dedupe loop
  (active per Issue 2) would reject and retry it.
- ❌ Don't modify the S1 bridge, `ParseOutput`, the manifest, the registry, the loaders, or any doc — this
  subtask is test-only. If a test reveals a real bridge bug, surface it; do not "fix" it here.
- ❌ Don't commit the temporary bridge-removal used for the TDD check.

---

## Confidence Score

**9 / 10** — This is a precisely-scoped, test-only addition at a single file, with the seam under test
already shipped (S1) and fully cited. The consumer (`ParseOutput`), the loader wiring (file Layer 3 /
git Layer 4), the registry's handling of user-defined providers, the stub harness, and the exact test
patterns to mirror are all verified against the working tree. The one residual subtlety — that
`cfg.Output=""` (test 4) never occurs via the real loaders and is the only way to exercise the bridge's
guard/inherit branch — is explicitly documented (with the production reality noted) so it is not
misread. The battery is provably non-tautological (Task 5: tests 1–3 fail when the bridge is removed).
No production-code risk; the only implementation hazards are mechanical (TOML escaping, HOME isolation,
staging a file), all called out above.
