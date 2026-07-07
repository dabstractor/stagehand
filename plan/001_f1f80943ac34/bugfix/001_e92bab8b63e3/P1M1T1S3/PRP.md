---
name: "P1.M1.T1.S3 — Regression tests: --config honored by default action + §19 notice once"
description: |
  Bugfix (Issue 1 + Issue 5 — the regression-test half). Test-only subtask that locks in the S1+S2
  config-handoff fix with two GREEN regression tests in `internal/cmd/default_action_test.go` driving
  the FULL in-process CLI seam (`PersistentPreRunE → runDefault → stagecoach.GenerateCommit →
  resolveConfig`) via `rootCmd.SetArgs` + `Execute`:

    (a) `TestRunDefault_ConfigFlagHonored_Issue1` — a user-defined provider (`[provider.stub]`)
        declared ONLY in a `--config` TOML resolves on the default action with `--dry-run`
        (exit 0 + the generated message). Before S1/S2 this was `unknown provider "stub"` (exit 1).
    (b) `TestRunDefault_RepoLocalNoticeOnce_Issue5` — a repo-local `.stagecoach.toml` that sets
        `provider` prints the §19 notice `repo-local config (.stagecoach.toml) sets provider to`
        EXACTLY ONCE (strings.Count == 1; was 2 before S1/S2's single-Load fix).

  Both share the root cause fixed in S1/S2 (no second `config.Load`), so they live in one subtask.

  Tiny enabler: add an exported test-observability seam `SetNoticeOut(io.Writer)` / `NoticeOut() io.Writer`
  to `internal/config/file.go` (5 lines) so the §19 notice — written to the unexported package-level
  `noticeOut` (`os.Stderr`, NOT the cobra err sink) — can be captured/counted from package `cmd`.
  This realizes the contract's stated fact that `noticeOut` is "swappable" (system_context §6) and
  production behavior is unchanged (it stays `os.Stderr`). No doc surface change.

  S1 (`pkg/stagecoach.Options.Config` + `resolveConfig` opts.Config!=nil branch) and S2
  (`runDefault`'s `Config: cfg`) are CONFIRMED landed in source — so both tests run green on first run.
---

## Goal

**Feature Goal**: Lock in the Issue 1 + Issue 5 fix (shipped in S1/S2) with two deterministic,
in-process regression tests that drive the real CLI↔`pkg/stagecoach` config-handoff seam end-to-end —
proving (a) `--config` is honored by the default commit action for a user-defined provider, and
(b) the §19 repo-local provider-redirect notice prints exactly once.

**Deliverable** (one small source enabler + two tests, all in-package Go):
1. `internal/config/file.go` — add `SetNoticeOut(io.Writer)` and `NoticeOut() io.Writer` (a test-only
   observability seam around the existing `noticeOut` var) — ~5 lines with a doc comment.
2. `internal/cmd/default_action_test.go` — add `TestRunDefault_ConfigFlagHonored_Issue1`,
   `TestRunDefault_RepoLocalNoticeOnce_Issue5`, and a `swapNoticeOut` test helper.

**Success Definition**:
- `stagecoach --config <file> --provider <user-defined-in-file> --dry-run` resolves the user-defined
  provider on the default action and prints the generated message (Issue 1 — no more
  `unknown provider`).
- The §19 notice `repo-local config (.stagecoach.toml) sets provider to` appears **exactly once**
  (`strings.Count == 1`) for a default-action run whose repo-local `.stagecoach.toml` sets `provider`
  (Issue 5 — was 2).
- `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green (the two new tests
  pass; no existing test regresses).
- No change to `GenerateCommit`/`Options`/`resolveConfig`/`runDefault` behavior, no doc edits.

## User Persona

**Target User**: The Stagecoach maintainer (and CI) who must be confident the S1/S2 single-Load fix is
never silently regressed (e.g. by someone re-introducing a `config.Load` inside `resolveConfig`, or
dropping the `Config: cfg` wiring in `runDefault`).

**Use Case**: `go test -race ./internal/cmd/` is the regression gate that fails loudly if `--config`
is ever dropped on the default action again, or if the §19 notice ever double-prints again.

## Why

- **Closes the M1 test gap.** `system_context.md §6` flags exactly two missing assertions: *"No test
  passes `--config /custom.toml` through the CLI default action"* and *"No test asserts the §19 notice
  count in the full CLI seam."* This subtask is that assertion. `system_context.md §5`: *"Issue 5 …
  gets a regression assertion in M1's test subtask (notice count == 1)."*
- **Both regressions share one root cause** (the now-removed second `config.Load` in `resolveConfig`),
  so testing them together maximizes coverage per line of test code and documents the shared etiology.
- **The full seam is required.** A `pkg/stagecoach`- or `internal/config`-level test cannot reproduce
  either bug: Issue 1's failure is *inside* `GenerateCommit` (the pre-check in `runDefault` always
  passed because it read the correct first-Load `cfg`), and Issue 5's `2` count is a property of the
  CLI calling `Load` twice across packages. Only the in-process `rootCmd.SetArgs` seam exercises both.

## What

### (a) `TestRunDefault_ConfigFlagHonored_Issue1`
A fresh, isolated git repo (temp HOME/XDG so no global config; **no `.stagecoach.toml`**) with one
staged file. A `--config` TOML (outside the repo) declares `[provider.stub]` pointing at the
`stubtest.Build(t)` binary. `STAGECOACH_STUB_OUT="feat: config honored"`. Drive
`Execute(ctx)` with `SetArgs(["--config", cfgPath, "--provider", "stub", "--dry-run"])`. Assert
exit 0 and the dry-run message on stdout == `"feat: config honored"`.

### (b) `TestRunDefault_RepoLocalNoticeOnce_Issue5`
A fresh git repo whose `.stagecoach.toml` defines `[provider.stub]` AND sets top-level
`provider = "stub"` (this top-level setting is what triggers the §19 notice). One staged file;
`STAGECOACH_STUB_OUT` set. Swap `config.noticeOut` → a `bytes.Buffer` (via the new seam). Drive
`Execute(ctx)` with `SetArgs(["--provider", "stub"])`. Assert `strings.Count(buf, needle) == 1`
where `needle = "repo-local config (.stagecoach.toml) sets provider to"`, the commit landed, and the
notice names `"stub"`.

### Success Criteria
- [ ] `SetNoticeOut`/`NoticeOut` added to `internal/config/file.go` (test-only seam; prod unchanged).
- [ ] `TestRunDefault_ConfigFlagHonored_Issue1` passes: `--config`-declared provider resolves on the
      default action (exit 0, dry-run message on stdout).
- [ ] `TestRunDefault_RepoLocalNoticeOnce_Issue5` passes: §19 notice count == 1.
- [ ] `go test -race ./...` green; `go build ./...`, `go vet ./...`, `gofmt -l .` clean.
- [ ] No edits to `pkg/stagecoach/*`, `runDefault`, `GenerateCommit`/`Options`/`Result`, any doc file,
      `Makefile`, `go.mod`, `PRD.md`, `tasks.json`, `prd_snapshot.md`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact harness helpers (`saveRootState`, `initRepo`,
`chdir`, `stubtest.Build`), the exact existing test to mirror (`TestRunDefault_DryRun`), the exact
notice text + file:line, the exact seam var + the 5-line addition, full copy-pasteable test bodies,
and executable validation commands. The architecture decisions (D1) and the predecessor PRPs (S1/S2)
are referenced by section with the relevant conclusions distilled.

### Documentation & References

```yaml
# MUST READ — binding architecture (do not re-litigate the fix; S1/S2 shipped it)
- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/system_context.md
  why: "§5 states Issue 5 'gets a regression assertion in M1's test subtask (notice count == 1)'. §6
        enumerates the two exact test gaps this subtask closes and the harness primitives to reuse
        (stubtest; setupStubRepo full seam via rootCmd.SetArgs; config.noticeOut swappable sink)."
  section: "§5 (test ownership), §6 (test gaps + harness primitives)"
  critical: "§6 lists config.noticeOut as a reusable 'swappable notice sink for tests' — that is the
             contract basis for the SetNoticeOut seam. The full seam (rootCmd.SetArgs) is required."

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/seam_config_handoff.md
  why: "§6 quotes loadRepoLocalConfig/repoProviderNotice + noticeOut (the seam under test). §8 proves
        no existing test passes --config or asserts the notice count (the gap). §9 is the data-flow
        diagram showing notice#1=PersistentPreRunE, notice#2=resolveConfig (now gone post-S1/S2)."
  section: "§6 (notice locus), §8 (test gap proof), §9 (data-flow)"

- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/decisions.md
  why: "D1 = the chosen fix (resolved-config injection); confirms Issue 5 needs no separate impl task.
        Establishes the tests assert the RESULT of D1, not a new behavior."
  section: "D1"

# Predecessor PRPs — the fix these tests lock in (already shipped & source-confirmed)
- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/P1M1T1S1/PRP.md
  why: "Documents the Options.Config field + the resolveConfig opts.Config!=nil skip-Load branch (the
        thing test (b) indirectly proves by asserting notice==1, and test (a) proves by asserting the
        --config provider resolves)."
- docfile: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/P1M1T1S2/PRP.md
  why: "Documents the runDefault `Config: cfg` wiring (the single line that makes both tests pass). Its
        Level-3 manual repro is the human equivalent of these two automated tests."

# Source under edit / cross-reference
- file: internal/config/file.go
  why: "EDIT TARGET (test seam). `var noticeOut io.Writer = os.Stderr` (line 50) + loadRepoLocalConfig
        (226-235) + repoProviderNotice (237-245). Add SetNoticeOut/NoticeOut right after the var."
  pattern: "Add two one-line exported funcs + a doc comment immediately after `var noticeOut ...` (line 50)."
  gotcha: "Do NOT change the default (os.Stderr) or the notice text/format. The seam is for TESTS to
           swap the sink; production behavior is byte-identical. noticeOut is captured by value at
           init, so reassigning os.Stderr elsewhere has no effect — the swap MUST go through SetNoticeOut."

- file: internal/cmd/default_action_test.go
  why: "EDIT TARGET (the two tests + helper). Append at EOF. Mirror TestRunDefault_DryRun for the seam
        shape (saveRootState/restoreRootState, SetOut/SetErr, SetArgs, Execute, assert)."
  pattern: "Reuse the existing package-private helpers verbatim: saveRootState/restoreRootState
            (root_test.go:105), initRepo (root_test.go:25), chdir (root_test.go:74), writeConfigFile
            (root_test.go:61), writeFile/stageFile/runGit/gitOut/headSHA (default_action_test.go:33-93),
            stubtest.Build (internal/stubtest)."

# READ-ONLY references (do NOT edit)
- file: internal/cmd/default_action.go
  why: "Confirms S2 landed (`Config: cfg` at line 131) and runDefault's stderr = cmd.ErrOrStderr() (33),
        which is why the §19 notice is NOT captured by rootCmd.SetErr (it bypasses the cobra sink). Also
        confirms the provider pre-validation (117-126) reads cfg — i.e. it always passed; the regression
        is strictly inside GenerateCommit, so the FULL seam must be driven."
- file: pkg/stagecoach/stagecoach.go
  why: "Confirms S1 landed (`if opts.Config != nil` at 126). When Config!=nil, resolveConfig does NOT
        call config.Load → exactly one Layer-3 loadRepoLocalConfig → one §19 notice."
- file: internal/provider/executor.go
  why: "Lines 56-57: `cmd.Stdout = &out; cmd.Stderr = &errb` — the stub subprocess does NOT inherit fd 2.
        So during Execute(), the ONLY writer to the noticeOut sink is the §19 notice. (Justifies that a
        swapped sink contains exactly the notice, no subprocess noise.)"
- file: internal/stubtest/stubtest.go
  why: "Build(t) compiles cmd/stubagent once per process and returns its path. Driven by env
        STAGECOACH_STUB_OUT (single response) — exactly what the existing default_action tests use."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── internal/config/
│   └── file.go                       # EDIT: +SetNoticeOut +NoticeOut (test seam)
├── internal/cmd/
│   ├── default_action_test.go        # EDIT: +2 regression tests + swapNoticeOut helper
│   ├── root_test.go                  # reuse: saveRootState/initRepo/chdir/writeConfigFile
│   └── ...
├── internal/stubtest/stubtest.go     # reuse: Build(t) → stubagent binary path
└── pkg/stagecoach/stagecoach.go        # READ-ONLY (S1 done): resolveConfig opts.Config!=nil branch
```

### Desired Codebase Tree After S3

```bash
stagecoach/
├── internal/config/file.go           # MODIFIED: +SetNoticeOut(io.Writer) +NoticeOut() (~5 lines)
└── internal/cmd/default_action_test.go  # MODIFIED: +TestRunDefault_ConfigFlagHonored_Issue1
                                         #        +TestRunDefault_RepoLocalNoticeOnce_Issue5
                                         #        +swapNoticeOut helper; + import internal/config
# (no other files touched)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/config/file.go` | MODIFY | Add `SetNoticeOut`/`NoticeOut` test seam around `noticeOut`. |
| `internal/cmd/default_action_test.go` | MODIFY | Add the two regression tests + `swapNoticeOut`; add `internal/config` import. |

**Explicitly NOT touched in S3**: `pkg/stagecoach/*` (S1 done), `internal/cmd/default_action.go`
(S2 done) & `root.go`, `GenerateCommit`/`Options`/`Result`/`resolveConfig`, `internal/provider/*`,
any `docs/*.md` / `README.md`, `Makefile`, `go.mod`, `PRD.md`, `tasks.json`, `prd_snapshot.md`.

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL — the §19 notice is NOT captured by rootCmd.SetErr. runDefault's stderr is cmd.ErrOrStderr()
// (default_action.go:33) and cobra output goes there too; but the notice is written by
// loadRepoLocalConfig to the package-level `noticeOut` (file.go:50, default os.Stderr), bypassing the
// cobra sink entirely. So test (b) MUST swap noticeOut (via the new SetNoticeOut seam) to count it.

// CRITICAL — noticeOut is captured by value at config-package init. Reassigning os.Stderr from a test
// does NOT redirect the notice. The swap MUST go through config.SetNoticeOut(buf) / restore via
// config.NoticeOut(). (see seam_config_handoff.md §6.)

// CRITICAL — drive the FULL seam (Execute → PersistentPreRunE → runDefault → GenerateCommit →
// resolveConfig), not just PersistentPreRunE. runDefault's provider PRE-CHECK (default_action.go:117)
// always honored --config (it reads the correct first-Load cfg); the Issue-1 failure was strictly INSIDE
// GenerateCommit's now-removed second Load. A test that stops at the pre-check would pass even with the
// bug present. rootCmd.SetArgs + Execute is the only harness that exercises the regression.

// CRITICAL — test (a) provider source must be ONLY the --config file. The repo must have NO
// .stagecoach.toml (Layer-3 would otherwise also define stub and mask the bug), and isolate HOME /
// XDG_CONFIG_HOME to temp dirs (cheap insurance against a stray real global config). "stub" is NOT a
// built-in (registry.go:15 built-ins = pi/claude/gemini/opencode/codex/cursor), so [provider.stub] is
// genuinely user-defined and resolves ONLY via cfg.Providers.

// CRITICAL — test (b) must set a TOP-LEVEL `provider = "stub"` in .stagecoach.toml to trigger the
// notice (repoProviderNotice returns "" when cfg.Provider == "" — file.go:240). setupStubRepo's toml
// sets ONLY [provider.stub] (no provider=), so it does NOT fire the notice; write a custom toml body.

// GOTCHA — these tests run GREEN on first run (S1/S2 are landed). If test (a) FAILS with
// "unknown provider \"stub\"", S1/S2 wiring is incomplete — re-check `Config: cfg` (default_action.go)
// and the opts.Config!=nil branch (stagecoach.go). If test (b) shows count==2, resolveConfig is still
// calling config.Load — re-check the same branch. Do NOT "fix" by editing resolveConfig here (S1 owns it).

// GOTCHA — reset flag state between runs. Every test MUST saveRootState/restoreRootState (resets
// --config/--provider/--dry-run Changed flags + loadedCfg). Omitting it leaks --config across tests.

// GOTCHA — notice text is `stagecoach: repo-local config (.stagecoach.toml) sets provider to %q\n`
// (file.go:244). The stable needle "repo-local config (.stagecoach.toml) sets provider to" appears
// exactly once per Load. Use strings.Count (not Contains) so a regression to 2 is caught as 2≠1.
```

## Implementation Blueprint

### Data models and structure

No new data models. The only type touch is exposing the existing `noticeOut` sink:

```go
// internal/config/file.go — the seam (add immediately after `var noticeOut io.Writer = os.Stderr`, line 50)
// SetNoticeOut sets the destination for the §19 repo-local provider-redirect notice (default os.Stderr).
// Intended for tests that need to observe/capture the notice (e.g. asserting it prints exactly once
// across the CLI↔pkg config-handoff seam). Pair with NoticeOut to restore. Non-test code should leave
// it at os.Stderr. (PRD §19; system_context §6 lists noticeOut as a swappable test sink.)
func SetNoticeOut(w io.Writer) { noticeOut = w }

// NoticeOut returns the current §19 notice destination (default os.Stderr). Pair with SetNoticeOut.
func NoticeOut() io.Writer { return noticeOut }
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD the notice-observability seam (internal/config/file.go)
  - LOCATE: `var noticeOut io.Writer = os.Stderr` (~line 50).
  - APPEND immediately after it the two exported funcs quoted above (SetNoticeOut / NoticeOut) + doc comment.
  - NAMING: SetNoticeOut(io.Writer) / NoticeOut() io.Writer (Go convention: SetX/X pair).
  - PRESERVE: the default value (os.Stderr), the var, loadRepoLocalConfig, repoProviderNotice — UNCHANGED.
  - DO NOT: change the notice text/format, the default sink, or any loader behavior. This is test infra only.
  - VERIFY: `go build ./internal/config/...` + `go vet ./internal/config/...` clean.

Task 2: ADD the swapNoticeOut helper (internal/cmd/default_action_test.go)
  - IMPORT: add "github.com/dustin/stagecoach/internal/config" to the test file's import block (if absent).
  - APPEND helper:
        func swapNoticeOut(t *testing.T, w io.Writer) {
            t.Helper()
            prev := config.NoticeOut()
            config.SetNoticeOut(w)
            t.Cleanup(func() { config.SetNoticeOut(prev) })
        }
  - WHY: swap noticeOut → w for the test, restore on cleanup. Used by test (b).

Task 3: ADD TestRunDefault_ConfigFlagHonored_Issue1 (Issue 1) — internal/cmd/default_action_test.go
  - PATTERN: mirror TestRunDefault_DryRun (saveRootState/restoreRootState, SetOut/SetErr, SetArgs, Execute).
  - SETUP:
      * bin := stubtest.Build(t)
      * Isolate global layer: home := t.TempDir(); t.Setenv("HOME", home); t.Setenv("XDG_CONFIG_HOME", home)
      * repo := t.TempDir(); initRepo(t, repo); chdir(t, repo)            // NO .stagecoach.toml
      * Write the --config file OUTSIDE the repo (temp dir) declaring ONLY [provider.stub]:
            cfgPath := filepath.Join(t.TempDir(), "custom.toml")
            body := fmt.Sprintf("[provider.stub]\ncommand = %q\nprompt_delivery = \"stdin\"\noutput = \"raw\"\nstrip_code_fence = true\n", bin)
            os.WriteFile(cfgPath, []byte(body), 0o644)
      * writeFile(t, repo, "new.txt", "hello"); stageFile(t, repo, "new.txt")
      * t.Setenv("STAGECOACH_STUB_OUT", "feat: config honored")
  - RUN: rootCmd.SetOut(&outBuf); rootCmd.SetErr(&errBuf);
         rootCmd.SetArgs([]string{"--config", cfgPath, "--provider", "stub", "--dry-run"})
         err := Execute(context.Background())
  - ASSERT: err == nil (t.Fatalf on non-nil — that IS the regression: "unknown provider \"stub\"");
            strings.TrimSpace(outBuf.String()) == "feat: config honored" (the dry-run message on stdout).
  - NAMING: TestRunDefault_ConfigFlagHonored_Issue1 (matches existing TestRunDefault_* convention).

Task 4: ADD TestRunDefault_RepoLocalNoticeOnce_Issue5 (Issue 5) — internal/cmd/default_action_test.go
  - PATTERN: mirror TestRunDefault_Commit (full commit path) + the swapNoticeOut helper.
  - SETUP:
      * bin := stubtest.Build(t)
      * repo := t.TempDir(); initRepo(t, repo); chdir(t, repo)
      * Write .stagecoach.toml with BOTH a top-level provider AND [provider.stub] (the provider= triggers
        the §19 notice):
            toml := fmt.Sprintf("provider = \"stub\"\n\n[provider.stub]\ncommand = %q\nprompt_delivery = \"stdin\"\noutput = \"raw\"\nstrip_code_fence = true\n", bin)
            writeConfigFile(t, repo, ".stagecoach.toml", toml)
      * runGit(t, repo, "add", ".stagecoach.toml"); runGit(t, repo, "commit", "-m", "init: config")
      * writeFile(t, repo, "new.txt", "hello"); stageFile(t, repo, "new.txt")
      * t.Setenv("STAGECOACH_STUB_OUT", "feat: add file")
      * var notice bytes.Buffer; swapNoticeOut(t, &notice)          // capture the §19 sink
  - RUN: rootCmd.SetOut(&outBuf); rootCmd.SetErr(&errBuf);
         rootCmd.SetArgs([]string{"--provider", "stub"}); err := Execute(context.Background())
  - ASSERT (Issue 5):
      * const needle = "repo-local config (.stagecoach.toml) sets provider to"
      * got := strings.Count(notice.String(), needle); require got == 1 (t.Errorf with the captured
        notice body on mismatch so 2-vs-1 is obvious).
      * strings.Contains(notice.String(), `"stub"`)  // notice names the configured provider
      * err == nil AND gitOut(t, repo, "log", "--format=%s", "-n1") == "feat: add file"  // full seam OK
  - NAMING: TestRunDefault_RepoLocalNoticeOnce_Issue5.

Task 5: VALIDATE (all gates must pass before declaring done — see Validation Loop)
```

### Implementation Patterns & Key Details

```go
// === Test (a): --config honored on the default action (Issue 1) ===
func TestRunDefault_ConfigFlagHonored_Issue1(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	bin := stubtest.Build(t)

	// Isolate the global layer; fresh repo with NO .stagecoach.toml (provider source = --config ONLY).
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	repo := t.TempDir()
	initRepo(t, repo)
	chdir(t, repo)

	// A --config file (outside the repo) declaring a USER-DEFINED provider ONLY here.
	cfgPath := filepath.Join(t.TempDir(), "custom.toml")
	body := fmt.Sprintf("[provider.stub]\ncommand = %q\nprompt_delivery = \"stdin\"\noutput = \"raw\"\nstrip_code_fence = true\n", bin)
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	writeFile(t, repo, "new.txt", "hello")
	stageFile(t, repo, "new.txt")
	t.Setenv("STAGECOACH_STUB_OUT", "feat: config honored")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"--config", cfgPath, "--provider", "stub", "--dry-run"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (--config must honor [provider.stub] on the default action)", err)
	}
	if got := strings.TrimSpace(outBuf.String()); got != "feat: config honored" {
		t.Errorf("dry-run stdout = %q, want %q", got, "feat: config honored")
	}
}

// === Test (b): §19 notice printed EXACTLY ONCE (Issue 5) ===
func TestRunDefault_RepoLocalNoticeOnce_Issue5(t *testing.T) {
	origArgs, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, origArgs, origOut, origErr, origRunE)

	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	chdir(t, repo)

	// Repo-local config: top-level provider= (fires the §19 notice) + [provider.stub] (resolves it).
	toml := fmt.Sprintf("provider = \"stub\"\n\n[provider.stub]\ncommand = %q\nprompt_delivery = \"stdin\"\noutput = \"raw\"\nstrip_code_fence = true\n", bin)
	writeConfigFile(t, repo, ".stagecoach.toml", toml)
	runGit(t, repo, "add", ".stagecoach.toml")
	runGit(t, repo, "commit", "-m", "init: config")

	writeFile(t, repo, "new.txt", "hello")
	stageFile(t, repo, "new.txt")
	t.Setenv("STAGECOACH_STUB_OUT", "feat: add file")

	var outBuf, errBuf, notice bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	swapNoticeOut(t, &notice) // §19 notice → buffer (it bypasses the cobra err sink)
	rootCmd.SetArgs([]string{"--provider", "stub"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	const needle = "repo-local config (.stagecoach.toml) sets provider to"
	if got := strings.Count(notice.String(), needle); got != 1 {
		t.Errorf("§19 notice count = %d, want 1\n--- captured notice ---\n%s", got, notice.String())
	}
	if !strings.Contains(notice.String(), `"stub"`) {
		t.Errorf("notice = %q, want it to name provider \"stub\"", notice.String())
	}
	if logMsg := gitOut(t, repo, "log", "--format=%s", "-n1"); logMsg != "feat: add file" {
		t.Errorf("git log subject = %q, want %q (full seam)", logMsg, "feat: add file")
	}
}
```

### Integration Points

```yaml
TEST SEAM (internal/config → internal/cmd):
  - new exported: SetNoticeOut(io.Writer) / NoticeOut() io.Writer in internal/config/file.go
  - consumer: internal/cmd/default_action_test.go swapNoticeOut helper (test (b) only)

NO-TOUCH (explicitly):
  - pkg/stagecoach/stagecoach.go        # S1 (done): Options.Config + resolveConfig branch
  - internal/cmd/default_action.go    # S2 (done): Config: cfg wiring
  - internal/cmd/root.go              # flagConfig, PersistentPreRunE, Config()
  - internal/provider/*, internal/config/load.go, file.go's loaders/notice text
  - any docs/*.md, README.md, Makefile, go.mod
  - PRD.md, tasks.json, prd_snapshot.md, anything under plan/ (except this PRP + research/)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l .                  # Expected: empty (run gofmt -w on any file it lists)
go vet ./internal/config/... ./internal/cmd/...   # Expected: exit 0
go build ./...              # Expected: exit 0

# Confirm the seam landed (exported, prod default unchanged)
grep -n "func SetNoticeOut\|func NoticeOut\|^var noticeOut" internal/config/file.go   # 3 matches
# Expected: Zero errors. Fix before proceeding.
```

### Level 2: The two new regression tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./internal/cmd/ -run 'TestRunDefault_ConfigFlagHonored_Issue1|TestRunDefault_RepoLocalNoticeOnce_Issue5' -v
# Expected: PASS — both green. (S1/S2 are landed, so these lock in the fix.)
#   If test (a) fails "unknown provider \"stub\"": S1/S2 wiring is incomplete — re-check
#     `Config: cfg` in internal/cmd/default_action.go and the `opts.Config != nil` branch in
#     pkg/stagecoach/stagecoach.go. Do NOT edit resolveConfig here.
#   If test (b) reports count=2: resolveConfig is still calling config.Load — same re-check.
```

### Level 3: Full suite + regression neighbors (System Validation)

```bash
cd /home/dustin/projects/stagecoach

# Whole cmd package (existing default_action tests must stay green — the seam addition is additive)
go test -race ./internal/cmd/ -v
# Whole config package (the seam is additive; file/load tests unaffected — they swap noticeOut in-package)
go test -race ./internal/config/ -v
# Public API (S1's injected-config branch + nil path)
go test -race ./pkg/stagecoach/ -v
# Everything, with the race detector
go test -race ./...
# Expected: ALL packages green.
```

### Level 4: Manual cross-check (the human equivalent of the two tests)

```bash
cd /home/dustin/projects/stagecoach
go build -o bin/stagecoach ./cmd/stagecoach
go build -o bin/stubagent ./cmd/stubagent

# Issue 1: --config honored on the default action (was: unknown provider)
REPO=$(mktemp -d) && cd "$REPO" && git init -q && git config user.email t@t && git config user.name t
cat > a.sh <<'EOF'
#!/usr/bin/env sh
cat >/dev/null; printf 'feat: config honored\n'
EOF
chmod +x a.sh
cat > custom.toml <<EOF
[provider.stub]
command = "$REPO/a.sh"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
EOF
echo hi > f.txt && git add f.txt
"$PWD/../../bin/stagecoach" --config custom.toml --provider stub --dry-run   # → "feat: config honored", exit 0

# Issue 5: §19 notice exactly once (was: 2)
printf 'provider = "stub"\n\n[provider.stub]\ncommand = "%s/a.sh"\nprompt_delivery = "stdin"\noutput = "raw"\nstrip_code_fence = true\n' "$REPO" > .stagecoach.toml
echo more >> f.txt && git add f.txt
"$(pwd)/bin/stagecoach" --provider stub 2>err.log
grep -c "repo-local config (.stagecoach.toml) sets provider to" err.log   # → 1

cd /home/dustin/projects/stagecoach && rm -rf "$REPO"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages pass (the two new tests green; none regress).

### Feature Validation
- [ ] `SetNoticeOut`/`NoticeOut` added to `internal/config/file.go`; default `noticeOut` unchanged (`os.Stderr`).
- [ ] `TestRunDefault_ConfigFlagHonored_Issue1` green: a `--config`-declared user-defined provider
      resolves on the default action (`--dry-run`, exit 0, message on stdout).
- [ ] `TestRunDefault_RepoLocalNoticeOnce_Issue5` green: §19 notice count == 1; commit landed.
- [ ] Manual Issue-1 + Issue-5 repros (Level 4) confirm the binary behavior.

### Scope Discipline Validation
- [ ] Did NOT edit `pkg/stagecoach/*` (S1's surface), `internal/cmd/default_action.go`/`root.go` (S2's),
      `GenerateCommit`/`Options`/`Result`/`resolveConfig`, or any loader/notice text.
- [ ] Did NOT edit any `docs/*.md`, `README.md`, `Makefile`, `go.mod`.
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this
      PRP + `research/`).
- [ ] The ONLY non-test source change is the 5-line `SetNoticeOut`/`NoticeOut` seam in `file.go`
      (justified: test observability; realizes the contract's "noticeOut is swappable" premise).

### Code Quality Validation
- [ ] Tests reuse existing harness helpers (no duplicated fixture machinery).
- [ ] Both tests call `saveRootState`/`restoreRootState` (no flag/loadedCfg leakage).
- [ ] Test (a) isolates HOME/XDG and uses a repo with NO `.stagecoach.toml` (provider source = `--config` only).
- [ ] Test (b) uses `strings.Count` (catches 2≠1, not just presence) and asserts the full seam (commit landed).
- [ ] `git diff --name-only` shows ONLY `internal/config/file.go` and `internal/cmd/default_action_test.go`.

---

## Anti-Patterns to Avoid

- ❌ Don't capture the §19 notice via `rootCmd.SetErr` — the notice writes to `noticeOut` (os.Stderr),
  bypassing the cobra sink. Use `swapNoticeOut` (the `SetNoticeOut` seam).
- ❌ Don't try to redirect the notice by reassigning `os.Stderr` — `noticeOut` captured it by value at
  init; only `SetNoticeOut` changes the actual sink.
- ❌ Don't stop the test at `PersistentPreRunE` / runDefault's provider pre-check — that path always
  honored `--config`; the regression lives inside `GenerateCommit`/`resolveConfig`. Drive the FULL seam.
- ❌ Don't put a `.stagecoach.toml` defining `[provider.stub]` in test (a)'s repo — that masks the bug
  (Layer-3 discovery would define stub too). The provider must come ONLY from the `--config` file.
- ❌ Don't omit the top-level `provider = "stub"` in test (b)'s `.stagecoach.toml` — without it
  `repoProviderNotice` returns "" and no notice fires (count 0, not the 1 you want).
- ❌ Don't use `strings.Contains` for the notice — use `strings.Count == 1` so a regression to 2 is
  caught (Contains would pass on 2 as well as 1).
- ❌ Don't edit `resolveConfig`/`Options`/`runDefault` to "make the test pass" — S1/S2 own the fix; if a
  test fails it means S1/S2 wiring is missing, not that S3 should re-fix it.
- ❌ Don't change the notice text/format or the `noticeOut` default — the seam is observation-only;
  production behavior must be byte-identical.
- ❌ Don't add CLI/docs changes or build a new harness pattern (subprocess) — the contract prescribes
  the in-process `rootCmd.SetArgs` seam; mirror the existing `TestRunDefault_*` tests.
- ❌ Don't skip `saveRootState`/`restoreRootState` — `--config`/`--provider`/`--dry-run` Changed flags
  and `loadedCfg` leak across tests without it.

---

## Alternative capture mechanisms (if `SetNoticeOut` is rejected as too invasive)

These are documented fallbacks; the primary is the `SetNoticeOut` seam above. Use only if a reviewer
insists on zero non-test source changes.

- **Alt B — fd-level stderr capture (pure test-only, Unix-only).** Because the provider executor
  captures subprocess stdout/stderr into Go buffers (executor.go:56-57) and runDefault/cobra write to
  `cmd.ErrOrStderr()`, the §19 notice is the ONLY writer to fd 2 during `Execute()`. A helper using
  `os.Pipe()` + `syscall.Dup`/`syscall.Dup2` to redirect fd 2 around `Execute()` captures exactly the
  notice. Caveats: Unix-only (needs `//go:build !windows` on the helper/test file), no existing
  `syscall` or build-tag usage in the repo to mirror, and the dup/restore/close ordering is easy to get
  subtly wrong (deadlock if the pipe fills — won't happen here, the notice is ~70 B). One-pass risk is
  higher than the seam.
- **Alt C — subprocess.** Build `bin/stagecoach`, run via `exec.Command` with `cmd.Stderr = &buf`,
  `strings.Count(buf, needle) == 1`. Clean and cross-platform, but a NEW harness pattern that diverges
  from the contract-prescribed in-process `rootCmd.SetArgs` seam; adds a binary build step to the test.
  Prefer this only if the in-process capture is infeasible.

---

## Confidence Score

**10/10** for one-pass implementation success.

Rationale: The fix under test (S1/S2) is already shipped and source-confirmed (`opts.Config != nil` at
`stagecoach.go:126`; `Config: cfg` at `default_action.go:131`). The two tests mirror an existing,
passing test (`TestRunDefault_DryRun`/`TestRunDefault_Commit`) using the codebase's own harness helpers
(`saveRootState`, `stubtest.Build`, `initRepo`, `chdir`, `writeConfigFile`), with full copy-pasteable
bodies provided. The only non-obvious element — capturing a notice that bypasses the cobra sink — is
resolved by a 5-line additive test seam that the contract itself authorizes (system_context §6:
"noticeOut … swappable notice sink for tests"; §5: "regression assertion in M1's test subtask (notice
count == 1)"), with two documented fallbacks. The diagnostic guidance (what a failure implies about
S1/S2 wiring) prevents the implementer from "fixing" the wrong file. No external research is required —
this is pure in-codebase test authoring against already-quarried architecture docs.
