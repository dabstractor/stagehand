# P1.M1.T2.S1 — Research findings (Issue 2: hook exec progress noise)

Surveyed 2026-07-03. The fix is small and surgical: 3 coordinated code edits + 2 test changes.

## Exact line numbers (verified)

**internal/cmd/hookexec.go** (`runHookExec`):
- L129: `labelProvider := name`
- L130: `u := ui.New(cmd.OutOrStdout(), stderr, ui.ResolveColor(cfg.NoColor, ui.IsTerminal(os.Stdout)))`
- L131: `u.Progress(ui.ProgressLabel("Generating", msgModel, labelProvider))` ← **REMOVE** (the unconditional call)
- L133-138: `if rerr := hook.Run(ctx, generate.Deps{Git:g, Manifest:m, Verbose:verbose, Excludes:excludes}, …)` ← **ADD `Progress:` field here**

**internal/hook/exec.go** (`Run`):
- L99-100: `if NoOpSource(source) { return ErrNoOp }` (source gate)
- L101-112: StagedDiff capture
- L113-114: `if diff == "" { return ErrNoOp }` (empty-diff gate)
- **INSERT after L114 / before L117**: `if deps.Progress != nil { deps.Progress() }`
- L117-118: `// Step C: parent / unborn.` + `_, isUnborn, err := deps.Git.RevParseHEAD(ctx)`

**internal/generate/generate.go** (`Deps`, L25-30): fields Git, Manifest, Verbose, Excludes → **ADD `Progress func()` after Excludes**.

## ui.Progress output format (for assertions)

`u.Progress(msg)` → `fmt.Fprintln(u.stderr, "↳ "+msg)`. So the line is `↳ Generating with <model> in <provider>…`
(or `↳ Generating…` when nothing resolved). Asserting `!bytes.Contains(errBuf, []byte("Generating"))` covers both
forms and the `↳` prefix. The progress line goes to STDERR (stdout stays clean) — `rootCmd.SetErr(&errBuf)` captures it.

## Backward-compatibility check (all generate.Deps construction sites)

| Site | Sets Progress? | Why safe |
|------|----------------|----------|
| `internal/cmd/hookexec.go:133` (hook.Run) | YES (this task) | the one we wire |
| `internal/decompose/decompose.go:247` (CommitStaged) | NO (nil) | CommitStaged never calls Progress (grep-confirmed: 0 refs in generate.go); default_action.go owns that path's progress |
| `pkg/stagecoach/stagecoach.go:386` (buildDeps) | NO (nil) | feeds CommitStaged — same as above |
| `pkg/stagecoach/stagecoach.go:328/349/356/359/367` | NO (empty `generate.Deps{}` error cases) | nil Progress is fine |

**Conclusion**: adding `Progress func()` is a zero-value-nil additive change. No other caller is touched.
CommitStaged is NOT modified (it has no progress concept — the default action prints progress itself at the CLI layer).

## Test patterns (hookexec_test.go)

Helpers: `hookexecNewTestRepo(t)` (temp repo + seed commit, returns dir), `writeTestStubConfig(t, stubBin)` (stub
provider config), `stubtest.Build(t)` (stub binary), `resetRootCmd()` (clears flags between tests), `runGitCmd`.
Capture: `var outBuf, errBuf bytes.Buffer`; `rootCmd.SetArgs(...)`; `rootCmd.SetOut(&outBuf)`; `rootCmd.SetErr(&errBuf)`;
`rootCmd.ExecuteContext(context.TODO())`.

**TestHookExec_SourceGateExit0** (L58) — EXTEND: it passes source=`"message"`, does NOT chdir (source gate fires
before any git op). Add assertion `!bytes.Contains(errBuf.Bytes(), []byte("Generating"))`.

**NEW TestHookExec_EmptyDiffNoProgress** — MUST `os.Chdir(repoDir)` (StagedDiff runs against os.Getwd(); the
seed-only repo has no staged changes → diff="" → ErrNoOp). Args: `{"--config", cfg, "hook", "exec", msgFile}`
(NO source → passes source gate → reaches StagedDiff). Assert: exit 0 (err nil), msg-file == "# comments\n",
`!bytes.Contains(errBuf.Bytes(), []byte("Generating"))`. Mirror the chdir+defer-restore block from
TestHookExec_StrictFailureNonZero (L109-115).

**TestHookExec_StrictFailureNonZero** (L97) — UNCHANGED, still valid: it has a real staged diff (`git add new.txt`)
→ Progress WOULD fire, but the test asserts only exit code + msg-file (not stderr content). It proves the happy-path
progress wiring does not break the error path. Optionally add a positive `bytes.Contains(errBuf, "Generating")`
assertion to pin "progress DOES fire when there's a real diff" — recommended for symmetry.

## Why the callback (not an inline pre-check)

The alternative — have `runHookExec` duplicate the NoOpSource check + an empty-diff probe before printing — would
re-run logic that `hook.Run` already owns, creating a second source of truth for the gates (drift risk). A
nil-safe `func()` callback on Deps, invoked once inside `hook.Run` right after BOTH gates pass, is the single
source of truth: the progress fires iff generation is truly about to run. CommitStaged ignores it (nil) — no effect
on the default/single-commit path.

## Scope fence

- IN: generate.go Deps field; hook/exec.go callback invocation; hookexec.go (remove unconditional call + set
  callback); hookexec_test.go (extend + add test).
- OUT: CommitStaged (no progress concept); default_action.go progress (owns the single-commit path separately);
  hook install/uninstall/status; any signature change; any docs/help change (the help text already describes the
  no-op behavior; this fix makes actual behavior match it).
