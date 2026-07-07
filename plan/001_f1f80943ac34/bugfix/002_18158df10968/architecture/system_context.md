# System Context — Bugfix-002

## Scope
Second-pass QA bugfix for Stagecoach v1.0 (post bugfix-001). Four **silent wrong-behavior**
bugs (no crashes, no data-integrity regressions). All center on the **config↔manifest handoff
seam** and **user-facing error wording**. The snapshot/atomic-commit core (PRD §13/§18.1),
rescue/timeout/CAS/signal mapping (§18.2-18.4), and root-commit path are verified correct and
**must not regress**.

## Project shape (verified)
- **Language/Runtime:** Go (modules). `go build ./...`, `go vet ./...` clean; `go test -race ./...` green.
- **Entry points:** `cmd/stagecoach/main.go` (CLI) → `internal/cmd/root.go` (cobra root) →
  `internal/cmd/default_action.go` (`runDefault`) → `pkg/stagecoach/stagecoach.go`
  (`GenerateCommit`, the public library API) → `internal/generate/generate.go`
  (`CommitStaged`, the frozen orchestrator) + `pkg/stagecoach.runPipeline` (dry-run/SystemExtra path).
- **Config:** `internal/config/` — 7-layer precedence resolver (`Load`), flat plain-typed `Config`.
- **Provider pipeline:** `internal/provider/` — `Manifest` (pointer-scalar tri-state), `Render`→`Execute`→`ParseOutput`.
- **Git boundary:** `internal/git/git.go` — shell-free `run()`/`runWithInput()` to the real git binary.
- **No new external dependencies.** No new tech. This is a pure Go bugfix — `external_deps.md` is N/A.

## The two seams this bugfix touches

### Seam A — config-file PATH resolution (Issue 1)
`config.Load` (internal/config/load.go:40-65) resolves the global-file path as:
`--config` (ConfigPathOverride) > `STAGECOACH_CONFIG` (env) > `globalConfigPath()` (discovery).
It then calls `loadTOML(globalPath)`, which returns `(nil, nil)` on `os.IsNotExist`
(internal/config/file.go:100). **Bug:** there is no distinction between "the user EXPLICITLY
named a file that's missing" and "the discovery default isn't there yet" — both become "layer
absent", so Stagecoach falls back to auto-detecting an installed built-in and silently invokes a
REAL agent. The fix must make an explicit (override/env) missing path a hard error while
preserving `loadTOML`'s `(nil,nil)` contract for discovery.

### Seam B — the `[generation]` ↔ manifest override bridge (Issue 2)
`pkg/stagecoach.buildDeps` (pkg/stagecoach/stagecoach.go:197-211) copies `cfg.Output` /
`cfg.StripCodeFence` onto the resolved `provider.Manifest` UNCONDITIONALLY:
```go
if cfg.Output != "" { ... m.Output = &o }        // cfg.Output is ALWAYS "raw" — Defaults() sets it
if cfg.StripCodeFence != nil { m.StripCodeFence = ... } // ALWAYS non-nil — Defaults() sets boolPtr(true)
```
Both guards always pass because `Defaults()` (internal/config/config.go:68-69) seeds non-empty/
non-nil values. Result: a **manifest-level** `output="json"` or `strip_code_fence=false` is
silently clobbered, and `providers show` *lies* (it shows the registry's pre-override manifest).
The fix makes `Config.Output` a `*string` (tri-state, like `StripCodeFence` already is) and stops
defaulting BOTH fields in `Defaults()` so `[generation]` becomes a true opt-in override and the
manifest's own values win by default. The manifest's `Resolve()` already supplies the §12.1
`raw`/`true` fallbacks (internal/provider/manifest.go: DefaultOutput/DefaultStripCodeFence).

### UX wording (Issues 3 & 4)
- **Issue 3:** `git.WriteTree` (internal/git/git.go:218-224) returns a noisy multi-line error
  (`unmerged (...)` ×N + `fatal: git-write-tree`) instead of PRD §13.5's clean single line.
  Fix is localized to the `WriteTree` error message (affects both CommitStaged step 3 and
  runPipeline step 3 automatically — still pre-generation).
- **Issue 4:** `--dry-run` returns a full `*RescueError` (exit 3/124 + manual `commit-tree`
  recovery recipe) even though no commit was ever intended. Fix is localized to the CLI layer
  (`internal/cmd/default_action.go` `handleGenError`): on dry-run + RescueError, print a short
  message and exit 1, omitting the recovery recipe. The library API (`pkg/stagecoach`) is
  unchanged — it still returns `*RescueError`; only the CLI rendering special-cases dry-run.

## Non-goals / constraints
- Do NOT touch the snapshot/atomic-commit core, CAS, signal handling, or root-commit path.
- Do NOT change `loadTOML`'s `(nil, nil)`-on-missing contract (discovery still tolerates absence).
- Do NOT change the public `pkg/stagecoach` library error contract (RescueError/CASError) — Issue 4
  is a CLI-layer UX change only.
- `PRD.md` is READ-ONLY. Source changes are made by downstream worker agents, NOT by this planner.

## Key files (authoritative, verified file:line in issue_analysis.md)
| Concern | File | Key symbols |
|---|---|---|
| Issue 1 | internal/config/load.go:48-65 | `Load`, `globalPath`, `opts.ConfigPathOverride` |
| Issue 1 (contract) | internal/config/file.go:100 | `loadTOML` IsNotExist→(nil,nil) |
| Issue 2 (defaults) | internal/config/config.go:35-36,68-69 | `Config.Output`, `Config.StripCodeFence`, `Defaults()` |
| Issue 2 (loaders) | internal/config/file.go:159-163,210-214; git.go:124-127,152-155 | `materialize`, `overlay`, `loadGitConfig` |
| Issue 2 (bridge) | pkg/stagecoach/stagecoach.go:197-211 | `buildDeps` |
| Issue 2 (manifest) | internal/provider/manifest.go:26,92-94,138-148 | `Manifest.Output/StripCodeFence`, `Resolve` |
| Issue 3 | internal/git/git.go:206-225 | `WriteTree` |
| Issue 4 | internal/cmd/default_action.go:169-188 | `handleGenError`, `flagDryRun` |
| Exit codes | internal/exitcode/exitcode.go | `For()`, constants |
