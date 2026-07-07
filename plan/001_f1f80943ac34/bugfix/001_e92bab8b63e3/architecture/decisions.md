# Architectural Decisions — Stagecoach v1.0 Bugfix Pass

Each decision is the binding choice for downstream PRP agents. Rationale cites the recon reports
and PRD. Do not re-litigate without a blocking technical reason.

---

## D1 — Fix Issues 1 & 5 by passing the *resolved* config into `GenerateCommit` (PRD "Option a")

**Choice:** Add an additive field to `pkg/stagecoach.Options` that carries a pre-resolved
`*config.Config`. When non-nil, `GenerateCommit`/`resolveConfig` **skips `config.Load` entirely**
(still applies the existing `Options` Provider/Model/Timeout/Verbose overrides on top, to preserve
the standalone-library contract). `runDefault` passes the CLI's already-loaded `cfg` (`Config()`).

**Rejected alternatives:**
- *Add `ConfigPathOverride` to `Options` only (PRD "Option b").* Fixes Issue 1 but **not** Issue 5
  (the second `Load` still runs → notice still prints twice). Would also need a `MuteRepoNotice`
  `LoadOpts` field as a band-aid. The PRD itself says Option (a) "also fixes Issues 5 and the
  duplicated side-effects of the double load."
- *Threading `generate.Deps` from the CLI.* Over-scoped; the manifest resolution belongs in
  `buildDeps`.

**API-stability note:** `Options` is documented "Stable as of v1.0 / ADDITIVE-ONLY for future
versions" (`pkg/stagecoach/stagecoach.go`). Adding a field is explicitly permitted. `pkg/stagecoach`
**already imports `internal/config`**, so accepting `*config.Config` introduces no new import edge.
External (out-of-module) callers cannot reference the unexported `internal/config.Config` type, but
that is fine: the field is nil-optional and exists for the in-module CLI; standalone callers leave it
nil and get the existing single-`Load` behavior unchanged.

**Precedence contract preserved:** `Options` overrides > Layer-7 flags > env > git-config >
repo-local > global > defaults. The CLI's first `Load` already folded Layer-7 in, so re-applying
`opts.Provider/Model/Timeout/Verbose` on the passed config is redundant-but-correct and keeps the
library's standalone path identical.

---

## D2 — Fix Issue 3 with a pre-snapshot `IsInstalled` check in `buildDeps`

**Choice:** In `buildDeps`, immediately after `m.Validate()` and before the pre-flight/return, call
the existing `reg.IsInstalled(m)` (`registry.go:72-83`, which does `exec.LookPath(m.DetectCommand())`).
On `false`, return a **plain** (non-`*RescueError`) error:

> `provider %q: command %q not found. Is the agent installed?`

`exitcode.For` maps a plain error to **exit 1**, and `handleGenError`'s generic branch prints
`stagecoach: <msg>` — exactly the §18.2 / §13.5 contract.

**Why `buildDeps` (not `CommitStaged`, not `Execute`, not the CLI):**
- Single chokepoint → covers **both** the `CommitStaged` and `runPipeline` paths with one edit.
- Runs **before** `WriteTree` → no dangling tree object, no armed rescue.
- `reg` and resolved `m` are already in scope → one-liner, no new imports.
- Matches the existing `unknown provider %q` / `Validate` plain-error pattern (already exit 1).
- `Validate()` guarantees `Command != nil && *Command != ""`, so `DetectCommand()` returns a real
  string and `IsInstalled` performs a genuine `LookPath`.

**Rejected:** distinguishing `cmd.Start` failure from non-zero exit inside `Execute` (would leave
the snapshot already written on the `CommitStaged`/`runPipeline` paths and still produces a dangling
tree). The pre-flight is strictly better.

---

## D3 — Fix Issues 2 & 6 by routing dry-run through the full loop (snapshot + dedupe + retry)

**Choice:** In `runPipeline`:
1. **Take the `WriteTree` snapshot unconditionally** (remove the `if !dryRun` gate) and call
   `signal.SetSnapshot` for both paths. (Issue 6.)
2. **Replace the dry-run single-pass block** with the same bounded generate→parse→dedupe loop already
   present for the SystemExtra commit path (rejected-subject list, `parseFail` + `retryInstr`
   preamble, `IsDuplicate` check, `MaxDuplicateRetries+1` attempts). (Issue 2.)
3. On dry-run, after `msg = m; success = true`, **return `Result{CommitSHA: "", Message: msg, ...}`**
   — i.e. skip ONLY `CommitTree` + `UpdateRefCAS`. FR49 satisfied: full pipeline, no commit/move-HEAD.
4. Dry-run timeout/exhaustion now returns a `*RescueError` (with the real `TreeSHA`) instead of the
   bare `ErrTimeout` sentinel. **This changes one locked-in test**
   (`stagecoach_test.go:224-250`, `TestGenerateCommit_Timeout/dryrun` asserts `errors.As(err, &re)` is
   **false**) — that assertion must be updated to expect a `*RescueError` with a non-empty `TreeSHA`,
   mirroring the commit-path subtest.

**Rejected:** adding a "no-commit" flag to `generate.CommitStaged`. `CommitStaged` is the frozen,
heavily-tested orchestrator; the cleaner, lower-risk path is to deduplicate the loop *within*
`runPipeline` (which already holds a second copy) rather than perturb `CommitStaged`'s contract.

**FR mapping now satisfied on dry-run:** FR49 (full pipeline), FR29 (parse-retry),
FR30-FR33 (duplicate rejection + bounded retries).

---

## D4 — Fix Issue 4 by APPLYING `[generation] output/strip_code_fence` onto the manifest (not removing)

**Choice:** In `buildDeps`, after `m.Validate()`, apply the resolved config's values onto the manifest
before returning `Deps`:

```go
if cfg.Output != "" {
    o := cfg.Output
    m.Output = &o
}
scf := cfg.StripCodeFence
m.StripCodeFence = &scf
```

(copy into locals to avoid aliasing the cfg value's address; mirror the `o := cfg.Output` pattern).

**Rationale:** This is the user-least-surprising choice and the **smaller** change (~5 lines in one
function). Users who set `output = "json"` / `git config stagecoach.output json` reasonably expect it
honored, and `docs/configuration.md` + the `config init` template advertise these knobs. The
per-manifest `[provider.X] output` override still works (it is merged before `buildDeps` applies the
`[generation]` layer, so the `[generation]` value wins as the broader setting — consistent with
"generation config tunes all providers"). `provider.ParseOutput` already reads these manifest
pointer fields, so no change to the parser.

**Rejected:** removing the fields entirely (~6 files: `config.go`, `file.go`, `git.go`, the `config
init` template, and ~6 loader tests). Larger blast radius, removes a documented capability, and
contradicts the `config init` template that ships as the "canonical config reference".

**Note on the `materialize` quirk:** `file.go:153` has a "v1 limitation: cannot set false via file"
comment for `StripCodeFence`. That quirk is out of scope for this fix; the git-config loader
(`stagecoach.stripCodeFence`) *can* set false, and applying `cfg.StripCodeFence` (always resolved via
`Defaults()`→true) works regardless. Do not attempt to fix the file-loader false-set quirk here.

**Depends on M1:** only meaningful once `buildDeps` receives the correctly-resolved `cfg` (so
`--config`'d `[generation]` knobs are honored). Sequence after the config-handoff fix.

---

## D5 — Fix Issue 7 with an early-return when the post-AddAll staged count is 0

**Choice:** In `internal/cmd/default_action.go`, in the `cfg.AutoStageAll` branch, between the
`StagedFileCount` error guard and the `Fprintln` notice, add:

```go
if n == 0 {
    return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
}
```

On a clean tree `AddAll` is a no-op and `StagedFileCount` returns 0, so this skips the misleading
"staging all changes (0 files)." line and goes straight to the exit-2 "Nothing to commit." path —
identical exit code/message (§15.4), better UX. The downstream `if !hasStaged` FR17 re-check stays as
a belt-and-suspenders guard.

**Rejected:** gating only the notice print on `n > 0` (keeps the redundant `HasStagedChanges`
re-check as the single source of truth). Either is acceptable; the early-return is cleaner since
`n == 0` is a strict subset of `!hasStaged` here. Use the early-return form.

---

## D6 — Documentation (SOW §5) plan

**Mode A (doc rides with the implementing subtask):**
- Issue 1 → `docs/cli.md` `--config` row is already correct, but the "default action honors it"
  contract should be affirmed; no template change needed beyond a prose touch if warranted.
- Issue 3 → `docs/cli.md` exit-code/failure table + `docs/how-it-works.md` failure-modes table get
  the "agent missing on $PATH → pre-generation → exit 1" row.
- Issue 4 → `docs/configuration.md` `[generation]` table affirms these knobs now apply.
- Issues 2/6 → `docs/cli.md`/`docs/how-it-works.md` dry-run description affirms full pipeline.

**Mode B (final changeset-level task):**
- `README.md` config-precedence / dry-run / agent-configure blurbs.
- `internal/cmd/config.go` `exampleConfigTemplate` — confirm the `[generation] output`/`strip_code_fence`
  comment lines are accurate post-fix.

The final "Sync changeset-level documentation" task declares dependencies on **every** implementing
subtask and runs last.
