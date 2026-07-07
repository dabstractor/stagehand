# P1.M3.T5.S1 — Design Decisions

Public library API (`pkg/stagecoach`). The PRD §14.1 surface is intentionally tiny:
`Options{Provider, Model, SystemExtra, DryRun, Timeout}` → `Result{CommitSHA, Subject, Message,
Provider, Model}`. This file records the load-bearing decisions. All signatures are quoted
VERBATIM from the on-disk code (the frozen contracts of P1.M1.T4 / P1.M2 / P1.M3.T1–T4).

---

## §0 — The central tension: the frozen `CommitStaged` cannot honor `DryRun` or `SystemExtra`

The item description says GenerateCommit "calls CommitStaged" and "If opts.DryRun: run the pipeline
but skip the commit-tree/update-ref steps." PRD §14.1 adds `SystemExtra string // appended to the
built system prompt`.

But the **frozen `generate.CommitStaged`** (P1.M3.T4.S2, READ-ONLY contract — on disk at
`internal/generate/generate.go`) is:
- `func CommitStaged(ctx context.Context, deps Deps, cfg config.Config) (Result, error)`
- It **always commits** (WriteTree → CommitTree → UpdateRefCAS; no dry-run branch).
- It **builds its own system prompt** via the unexported `buildSystemPrompt` helper → it has NO seam
  to accept `SystemExtra` (and `config.Config` has no `SystemExtra` field — confirmed in config.go).
- `generate.Deps` is `{Git git.Git; Manifest provider.Manifest}` — no place for a dry-run/extra flag.

P1.M3.T4.S2's PRP is explicit: *"CommitStaged always commits"* and *"dry-run is the CLI's job."*

**Therefore: the public API CANNOT honor `Options.DryRun` or `Options.SystemExtra` by a plain call to
`generate.CommitStaged`.** Calling it would (a) commit even when `DryRun` (defeating DryRun) and
(b) silently drop `SystemExtra` (a v1-stable-API defect). This is NOT a contradiction with P1.M3.T4.S2
— its scope was the orchestrator + its integration tests only; it correctly offloaded DryRun/SystemExtra
to the public layer. The parallel-context rule (do not modify the previous PRP's deliverable) FORBIDS
adding a dry-run/extra seam to `CommitStaged`/`Deps`/`Config`. So the resolution lives in `pkg/stagecoach`.

### Resolution (the design this PRP specifies)

**GenerateCommit delegates to `generate.CommitStaged` for the common case (`!DryRun && SystemExtra ==
""`)** — the CLI default action and most library calls. This honors the item description's "calls
CommitStaged" for the primary path AND uses the well-tested atomic commit flow with zero duplication.

**When `DryRun` OR `SystemExtra != ""`, GenerateCommit drives a self-contained path** (`runPipeline`,
unexported, in `pkg/stagecoach/stagecoach.go`) that reuses the SAME exported primitives `CommitStaged`
uses — so behavior matches and the only "duplication" is the loop skeleton + the commit plumbing:

- `git.Git` (RevParseHEAD / StagedDiff / WriteTree / CommitTree / UpdateRefCAS) — real `git.New(repo)`.
- `prompt.{BuildSystemPrompt, BuildFallbackPrompt, DetectMultiline, BuildUserPayload}`.
- `provider.{Manifest.Render, Execute, ParseOutput}`.
- `generate.{ExtractSubject, IsDuplicate}` (exported — reused, NOT re-implemented).
- `generate.{RescueError, CASError, ErrTimeout, ErrRescue, ErrCASFailed}` (exported — reused).

`runPipeline` is a faithful mirror of `generate.CommitStaged`'s 10-step flow (read `generate.go` as the
reference implementation) with two differences: (1) `SystemExtra` is appended to the system prompt;
(2) `DryRun` runs a SINGLE generation pass and returns `CommitSHA=""` without touching the object store.

This is the ONLY way to honor the full `Options` contract while respecting the frozen `CommitStaged`. It
is a documented consequence of P1.M3.T4.S2's (correct) scoping, not a defect.

---

## §1 — `Result` mapping drops `Changes` (public surface is PRD §14.1's shape)

`generate.Result` = `{CommitSHA, Subject, Message, Provider, Model, Changes []git.FileChange}`.
`stagecoach.Result` (PRD §14.1) = `{CommitSHA, Subject, Message, Provider, Model}` — **NO `Changes`**.
The public API is "intentionally tiny." So the common-path delegation maps `generate.Result` →
`stagecoach.Result` by dropping `Changes` (the file listing is a CLI/report concern, not a library
concern). `runPipeline` constructs `stagecoach.Result` directly (it never needs `Changes`).

## §2 — Error re-export (one import for library consumers)

`pkg/stagecoach` re-exports the typed errors so consumers import ONLY `pkg/stagecoach`:
```go
var (
    ErrNothingToCommit = generate.ErrNothingToCommit
    ErrTimeout         = generate.ErrTimeout
    ErrRescue          = generate.ErrRescue
    ErrCASFailed       = generate.ErrCASFailed
)
type RescueError = generate.RescueError  // type alias — interchangeable, errors.As works across both
type CASError   = generate.CASError
```
`errors.Is(err, stagecoach.ErrCASFailed)` and `errors.As(err, &stagecoach.RescueError{})` both work because
the public symbols ARE the generate symbols (alias / same sentinel).

## §3 — Config resolution + opts overrides

`GenerateCommit` has no CLI flags, so: `cfg, err := config.Load(ctx, config.LoadOpts{RepoDir: repoDir,
Flags: nil})` where `repoDir` = `os.Getwd()` (the public API operates on the CWD repo — Options has no
RepoDir, by PRD §14.1 design). `loadGitConfig(repoDir)` runs `git -C repoDir config`, which works from
ANY subdir (git walks up to the repo root), so CWD is correct for both config and git ops.

Then apply the Options that ARE config fields (additive override, HIGHEST precedence — opts is the
caller's explicit intent): `opts.Provider`/`opts.Model`/`opts.Timeout` overwrite `cfg` when non-zero.
`SystemExtra` and `DryRun` are NOT config fields — they flow into `runPipeline` directly.

## §4 — Manifest resolution (registry + auto-detect + Validate)

```go
overrides, _ := provider.DecodeUserOverrides(cfg.Providers)      // nil-safe → empty map
reg := provider.NewRegistry(overrides)
name := cfg.Provider
if name == "" {
    installed := /* names in reg.List() where reg.IsInstalled(m) */
    name = reg.DefaultProvider(installed)                          // "" if none installed
}
m, ok := reg.Get(name)
if !ok { return Result{}, fmt.Errorf("stagecoach: unknown or unavailable provider %q", name) }
if err := m.Validate(); err != nil { return Result{}, fmt.Errorf("stagecoach: provider %q: %w", name, err) }
deps := generate.Deps{Git: git.New(repoDir), Manifest: m}
```
`Manifest.Resolve()` is NOT called here — `CommitStaged`/`Render` resolve internally (mirrors P1.M3.T4.S2).
`Validate` is called explicitly to fail fast on a malformed manifest BEFORE any git/agent work.

## §5 — DryRun is single-pass (no dedupe loop)

`DryRun` previews the message that WOULD be committed. Dedupe (FR32) exists to avoid committing a
duplicate subject — moot when nothing is committed. So DryRun does ONE Render→Execute→ParseOutput pass
and returns `Result{CommitSHA:"", Subject, Message, Provider, Model}`. This is the faithful reading of
"run the pipeline but skip the commit-tree/update-ref steps": the pipeline's commit tail is skipped, and
the dedupe tail (which only matters for the commit) is naturally skipped too. DryRun does NOT call
`WriteTree` (no commit → no snapshot needed → no object-store write).

DryRun error contract (no snapshot ⇒ no `RescueError`, which requires a `TreeSHA`):
- nothing staged → `ErrNothingToCommit` (same as commit path).
- generation timeout → `ErrTimeout` (bare sentinel; no `RescueError`).
- model produced no valid message → a descriptive `fmt.Errorf` (NOT a typed sentinel; nothing to recover).

## §6 — SystemExtra reaches the prompt only in `runPipeline`

`SystemExtra` is appended to the system prompt as `sysPrompt + "\n\n" + SystemExtra` when non-empty. This
happens in `runPipeline` (which builds the prompt). In the common delegation path (`SystemExtra==""`) it
is a no-op by definition. So `SystemExtra != ""` forces the `runPipeline` path even when `!DryRun` —
there it runs the FULL generate→dedupe→commit flow (mirroring `CommitStaged`) with the extended prompt.

## §7 — `runPipeline` reuses `generate`'s exported error types for the commit path

When `runPipeline` commits (SystemExtra set, !DryRun), it returns the SAME typed errors as
`CommitStaged`: `*generate.RescueError{Kind:ErrTimeout}` (timeout, immediate), `*generate.RescueError
{Kind:ErrRescue}` (loop exhausted / ctx cancel), `*generate.CASError` (CAS fail, re-reads HEAD). This
keeps the error contract UNIFORM across the delegation path and the self-contained path — a library
consumer handles them identically regardless of whether SystemExtra/DryRun was set.

## §8 — Tests mirror `generate_test.go` at the public boundary; own git fixtures

`pkg/stagecoach/stagecoach_test.go` (`package stagecoach`) drives `GenerateCommit` end-to-end with the
**real** `provider.Execute` + a **stub** agent (`internal/stubtest`) against **real temp git repos** —
exactly the integration-test pattern of `internal/generate/generate_test.go` (P1.M3.T4.S2), but at the
PUBLIC boundary. Scenarios: commit-success; DryRun (CommitSHA=="", HEAD unchanged); nothing-staged →
ErrNothingToCommit; provider override; timeout → ErrTimeout.

**The git fixture helpers (`initRepo`/`writeFile`/`stageFile`/`headSHA`/`commitRaw`/`gitOut`/`runGit`)
are package-private in `generate_test.go` AND in `internal/git/*_test.go` → UNIMPORTABLE.** Copy the
~25-line set into `stagecoach_test.go` (same approach P1.M3.T4.S2 took). Set identity via repo-local
`git config user.name/email` (cleanest — no env pollution).

## §9 — `go mod tidy` is a no-op; only stdlib + same-module internals

`pkg/stagecoach/stagecoach.go` imports: `context`, `errors`, `fmt`, `os`, `strings` (stdlib) +
`github.com/dustin/stagecoach/internal/{config,generate,git,prompt,provider}` (same module — `internal/`
is importable by any package in `github.com/dustin/stagecoach`). NO new third-party dep. go.mod/go.sum
byte-unchanged.
