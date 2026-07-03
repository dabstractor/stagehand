# Research: `hook exec` runtime (P1.M3.T2.S1)

Date: 2026-07-02. Condensed findings underpinning the PRP. All file:line refs are to the
current tree; verify before citing in code.

## 0. The defining design call — NO commit plumbing

`generate.CommitStaged` (internal/generate/generate.go) bundles **snapshot + generation + commit**
into one 10-step pipeline: RevParseHEAD → StagedDiff → **WriteTree (snapshot)** → system prompt →
generate/parse/dedupe LOOP → **CommitTree** → **UpdateRefCAS** → DiffTree. It arms
`signal.SetSnapshot` for the §18.3 rescue and returns `*RescueError`/`*CASError`.

**Hook mode must NOT do steps 3/7/8/9** (FR-H4: "No snapshot, no `commit-tree`, no `update-ref`:
git owns this commit"). So `hook exec` CANNOT call `pkg/stagehand.GenerateCommit` (it always
snapshots, even on DryRun) nor `generate.CommitStaged`. It reuses the **generation steps only**
(1, 2, 4, 5) by calling the same EXPORTED primitives, and writes the result to `<msg-file>`.

## 1. The generation loop is in-package-unexported → MIRROR it (precedent: pkg/stagehand)

CommitStaged's loop (generate.go ~step 5) is unexported and tangled with the snapshot (it
references `treeSHA`/`parentSHA` to build `*RescueError`). Extracting a shared helper would force a
behavior-sensitive refactor of the CORE pipeline and the error semantics differ (CommitStaged needs a
tree SHA for the rescue recipe; hook needs never-block). **Mirroring is the established pattern:**
`pkg/stagehand.buildSysPrompt` + `pkg/stagehand.runPipeline` BOTH re-implement generate's internals
with the comment *"This mirrors generate.X (unexported — can't import). It reuses the prompt
builders; NOT IP duplication."* — see pkg/stagehand/stagehand.go:393 and :415. `internal/hook.Run`
does the same. The loop is ~35 lines; the reused primitives are all exported.

### Reusable exported primitives (all already shipped)
- capture: `git.Git.StagedDiff(ctx, git.StagedDiffOptions{MaxDiffBytes,MaxMdLines,BinaryExtensions,Excludes})` (returns "" for empty — that IS the empty-diff no-op gate, FR-H4).
- prompt: `prompt.BuildSystemPrompt(examples, hasMultiline, subjectTarget, format, locale)` + `prompt.BuildFallbackPrompt(subjectTarget, format, locale)` (system.go:190/176) and `prompt.BuildUserPayload(diff, context, rejected)` (payload.go:97).
- role/model: `config.ResolveRoleModel("message", cfg)` → (provider, model, reasoning) (roles.go).
- render/exec/parse: `manifest.Render(model, sysPrompt, payload, reasoning)` (render.go:89), `provider.Execute(ctx, spec, timeout, vb)` (executor.go:44), `provider.ParseOutput(out, manifest)` → (msg, ok, fellback) (parse.go).
- finalize: `generate.FinalizeMessage(msg, cfg)` (template seam, finalize.go), `generate.ExtractSubject(msg)`, `generate.IsDuplicate(subject, recent)` (dedupe.go).
- history (mature-vs-fallback + dedupe set): `git.RevParseHEAD` (isUnborn short-circuits CommitCount/RecentMessages/RecentSubjects), `git.CommitCount`, `git.RecentMessages(20)`, `git.RecentSubjects(50)`. Mirror `generate.buildSystemPrompt`/`recentSubjects` (unexported — copy the ~15-line wrappers).

### buildSystemPrompt mirror (copy from generate.go:unexported, also in pkg/stagehand:393)
```go
func buildSystemPrompt(ctx, g, cfg, isUnborn) (string, error) {
    if isUnborn { return prompt.BuildFallbackPrompt(cfg.SubjectTargetChars, cfg.Format, cfg.Locale), nil }
    n, err := g.CommitCount(ctx); if err != nil { return "", err }
    if n <= 1   { return prompt.BuildFallbackPrompt(cfg.SubjectTargetChars, cfg.Format, cfg.Locale), nil }
    msgs, err := g.RecentMessages(ctx, 20); if err != nil { return "", err }
    return prompt.BuildSystemPrompt(msgs, prompt.DetectMultiline(msgs), cfg.SubjectTargetChars, cfg.Format, cfg.Locale), nil
}
```

## 2. Config-load contract from S2 (CRITICAL)

S2's PRP gives `hookCmd` a **no-op `PersistentPreRunE`** so install/uninstall/status don't trigger
`config.Load`'s first-run bootstrap WRITE (FR-B3, load.go:103-109). Cobra runs only the NEAREST
PersistentPreRunE (nearest-wins; no EnableTraverseRunHooks), so **`hook exec` inherits that no-op** and
`Config()` returns nil for it. Therefore `hook exec` must load config ITSELF inside RunE:
```go
cfg, err := config.Load(cmd.Context(), config.LoadOpts{
    ConfigPathOverride: flagConfig, RepoDir: repoDir, Flags: cmd.Flags(),
})
```
(mirror root.go's PersistentPreRunE exactly). This fits FR-H5: a config-load error is a FAILURE →
never-block (stderr line + exit 0, or non-zero under --strict). The persistent --provider/--model/
--timeout/--message-*/--format/... flags ARE inherited (root persistent flags), so `cmd.Flags()` has
them; the hook script passes only `--strict "$@"`, so they're unset and config.Load falls back to
env/file/git-config — exactly FR-H6 ("resolves provider/model/reasoning exactly like the
single-commit path").

## 3. Manifest resolution = mirror runDefault/buildDeps (~15 lines, duplicated 3rd time)

runDefault (default_action.go) and buildDeps (pkg/stagehand:325) both inline the same resolution:
`provider.DecodeUserOverrides(cfg.Providers)` → `provider.NewRegistry` → resolve message provider
(`config.ResolveRoleModel("message", cfg)`; if "" → `reg.DefaultProvider(installed)`) → `reg.Get` →
`m.Validate()` → `reg.IsInstalled(m)` (pre-flight) → apply `cfg.Output`/`cfg.StripCodeFence` overrides
→ `generate.Deps{Git, Manifest, Verbose, Excludes}`. hookexec.go copies this inline. **Do NOT import
pkg/stagehand from internal** (internal→pkg is an anti-pattern and would be a needless dependency);
the duplication is accepted (already present in runDefault vs buildDeps). Excludes via
`exclude.ResolveExcludePathspecs(cfg, repoDir, verbose)` (exclude.go:98) — mirror runDecompose.

## 4. Source gate (FR-H4) + architecture §3 (VERIFIED)

From architecture/external_deps.md §3 (git 2.54.0): args are `(msg-file, source, sha)`; **plain
`git commit` invokes the hook with ONLY arg 1 — source is ABSENT.** Named sources:
`message` (`-m`/`-F`) | `template` (`-t`/`commit.template`) | `merge` | `squash` | `commit`
(`-c`/`-C`/`--amend`). SHA present only when source=`commit`. Gate:
`source := args[1] if len(args)>=2 else ""`; no-op (exit 0, no generation) iff source ∈ the 5 named
sources. Empty/absent source → proceed. **Non-zero exit aborts the commit and `--no-verify` does NOT
skip prepare-commit-msg** — this is WHY FR-H5's exit-0-on-any-failure is load-bearing.

## 5. Message-file write (FR-H4) — prepend, preserve comment block

git hands `<msg-file>` containing (empty case) the comment block: lines starting with `#` (the
"Please enter the commit message…" template). Write the generated message at the TOP and keep git's
comment block verbatim beneath it; git itself strips `#` lines on commit. Ensure exactly one blank
separator line. Robust form:
```go
b := msg + "\n"
if len(orig) > 0 { b += "\n" + string(orig) }   // blank line then original comment block verbatim
os.WriteFile(path, []byte(b), 0o644)
```
(If orig already begins with `\n` you get one cosmetic extra blank line — git collapses it; do NOT
mutate orig.)

## 6. Never-block contract (FR-H5) — error → exit 0, msg-file UNTOUCHED

`hook.Run` returns: **nil** = generated+written; **ErrNoOp** = intended no-op (source gated OR empty
staged diff) → exit 0 silently; **any other error** = generation failure (timeout, parse/dup
exhaustion, agent missing, config error) → `<msg-file>` UNCHANGED + one stderr warning line + exit 0.
`--strict` (baked into the script by S1/S2: `exec stagehand hook exec --strict "$@"`) inverts the
last case to **non-zero** (exitcode.Error=1) → aborts the commit. The cmd layer owns exit codes; Run
just returns the error (does NOT call os.Exit). NEVER write the file on a failure path (write only
after a fully-accepted message). NO snapshot/signal/commit-tree/update-ref/rescue anywhere.

## 7. The cobra leaf (internal/cmd/hookexec.go — NEW file, do NOT edit S2's hook.go)

S2 owns internal/cmd/hook.go and defines the package-level `hookCmd` (with the no-op
PersistentPreRunE). hook exec attaches as a SIBLING leaf from a NEW file to avoid colliding with S2's
parallel work:
```go
var hookExecCmd = &cobra.Command{Use:"exec", Args: cobra.RangeArgs(1,3), RunE: runHookExec, ...}
func init() {
    hookExecCmd.Flags().BoolVar(&flagHookExecStrict, "strict", false, "Abort the commit on generation failure (default: never block)")
    hookCmd.AddCommand(hookExecCmd)   // hookCmd is S2's var; AddCommand mutates the live *Command
}
```
`hookCmd` is a package var initialized at package-init time (before any init() func), so
`hookCmd.AddCommand` in hookexec.go's init() is safe regardless of init() ordering. Args:
`cobra.RangeArgs(1,3)` = `<msg-file> [<source> [<sha>]]`. Documented as called-by-git (not users).

## 8. e2e scenarios (internal/e2e, `//go:build e2e`) — extend the harness

The hook script (`exec stagehand hook exec "$@"`) resolves `stagehand` from **$PATH** (S1 script is
not absolute-path). So the e2e must (a) prepend the built stagehand binary's dir to PATH, (b) set
`STAGEHAND_CONFIG` to a stub config (writeStubConfig) + `STAGEHAND_STUB_*` knobs (stubEnv), (c) set
`GIT_EDITOR=true` (or `:`) so git does NOT open a real editor on the hook-filled message, (d) run
real `git -C repo commit` (NOT runStagehand — git fires the hook). Helper additions to the e2e
package. Three scenarios the item names:
- **happy path**: stub returns "feat: …"; `GIT_EDITOR=true git commit` → HEAD message == stub output (hook filled the file).
- **failure-injection**: stub exit 1; `GIT_EDITOR='sh -c "echo fallback >$1"' git commit` → HEAD == "fallback" (hook exit-0'd, git continued; never-block proven).
- **`-m` no-op**: stub returns "X"; `git commit -m "explicit"` → HEAD == "explicit" (source=message → hook no-op'd; stub output never landed).

## 9. Out of scope (explicit fences)
- FR-E4 `--edit` usage-error rejection on hook exec → **P1.M5.T1.S1** (later). Do NOT add it here.
- `hook install/uninstall/status` (S2) and HooksPath/hookScript/Marker/ScriptMode (S1) → CONSUMED, not re-implemented.
- README surfacing → P1.M7.T1.S1. Docs here = [Mode A] docs/cli.md (`hook exec` reference) + docs/how-it-works.md (FR-H7 FAQ entry).
- Decomposition: hook mode NEVER decomposes (FR-H6 — fills one message for one in-flight commit).
