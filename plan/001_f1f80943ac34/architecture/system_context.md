# System Context — Stagecoach

> **Status:** Researched & validated against the live environment (2026-06-29).
> All agent CLIs verified against real `--help` output. Git plumbing verified by execution.

## 1. Project State

**Greenfield.** The repository contains only `PRD.md` and `plan/`. No Go code exists yet.
The Go module will be created from scratch following the package layout in PRD §14.

- **Runtime:** Go 1.26.4 installed (`go version go1.26.4-X:nodwarf5 linux/amd64`). PRD targets 1.22+; toolchain supports it.
- **Git:** 2.54.0 installed (PRD floor is 2.20; all plumbing primitives are ancient).
- **All 6 target agents are installed and on `$PATH`:** `pi`, `claude`, `gemini`, `opencode`, `codex`, `agent` (Cursor).

## 2. Originating Tool

`commit-pi` (zsh) lives at `~/projects/git-scripts/commit-pi` (9.5 KB). A near-identical
`commit-claude` fork exists alongside. The PRD is a faithful generalization of this script:

- It captures the staged diff (markdown ≤100 lines/file, other ≤300 KB, with lock/snap/map/vendor exclusions).
- It snapshots with `git write-tree`, captures `PARENT_SHA` before generation.
- It builds a system prompt with the last 20 commit messages + anti-reuse rule + JSON contract.
- It invokes `pi --provider zai --model glm-5-turbo --system-prompt ... --no-* -p` reading diff+instruction from stdin.
- It parses JSON via `sed`, retries on malformed output, runs a duplicate-rejection loop (last 50 subjects, 3 retries).
- It commits with `git commit-tree` + `git update-ref HEAD <new> <parent>` (CAS form).
- It installs a `trap 'handle_error' INT TERM` rescue that prints `TREE_SHA` + manual recovery command.

**Stagecoach changes vs commit-pi:**
- Agent-agnostic via provider manifests (not welded to `pi`).
- **Raw output default** (not JSON) — removes the double-quote constraint and fragile `sed` parse. JSON remains available per-provider.
- Go binary (not zsh) — distributable via Homebrew/go install/releases.
- Auto-stage-all, config precedence, `--dry-run`, `providers`/`config` subcommands.

## 3. High-Level Data Flow (PRD §11.1)

```
git index (staged) ─▶ diff capture ── diff payload ──┐
                                                     ▼
snapshot           ┌──────────┐          ┌────────────────┐
write-tree ───────▶│ freezes  │          │ prompt builder │
                   │ TREE_SHA │          │ (style learn)  │
                   └────┬─────┘          └───────┬────────┘
                        │ TREE,PARENT            │ system+user prompt
                        │                        ▼
                        │              ┌──────────────────┐
                        │              │ provider executor│──▶ external CLI agent (stdin→stdout)
                        │              └────────┬─────────┘
                        │                       │ raw/json output
                        │                       ▼
                        │              ┌──────────────────┐
                        │              │  parse + dedupe  │── retry loop ──┐
                        │              └────────┬─────────┘               │
                        │                       │ commit message          │
                        ▼                       ▼
                 ┌──────────────────────────────────────┐
                 │ commit-tree -p PARENT -m MSG TREE     │──▶ NEW_SHA
                 │ update-ref HEAD NEW PARENT (atomic)   │
                 └──────────────────────────────────────┘
                        │ on failure ──▶ rescue protocol (print TREE_SHA + recovery cmd)
```

The flow is **linear and synchronous in v1**. Stage-while-generating is NOT backgrounding Stagecoach;
it's the user running `git add` in another pane during the blocking generation call — safe because
the commit is built from the frozen `TREE_SHA`, not the live index.

## 4. Package Layout (PRD §14)

```
stagecoach/
├── cmd/stagecoach/main.go              # entrypoint: arg parsing, wiring, exit codes
├── internal/
│   ├── config/      config.go, defaults.go, file.go, config_test.go
│   ├── provider/    manifest.go, builtin.go, registry.go, executor.go, parse.go, *_test.go
│   ├── prompt/      system.go, examples.go, payload.go, *_test.go
│   ├── git/         git.go, plumbing.go, diff.go, log.go, stage.go, *_test.go
│   ├── generate/    generate.go, dedupe.go, rescue.go, *_test.go
│   └── ui/          output.go, exitcode.go
├── pkg/stagecoach/stagecoach.go         # PUBLIC API: GenerateCommit(ctx, opts) (Result, error)
├── providers/        pi.toml, claude.toml, gemini.toml, opencode.toml, codex.toml, cursor.toml
├── docs/PRD.md
├── .goreleaser.yaml, go.mod, go.sum, Makefile, README.md
```

### Design constraint protecting v2 (§11.3)

The core is `commitStaged(ctx, cfg) error` that assumes the index is already in the desired state.
- **v1 main:** `maybeAutoStage(); commitStaged()`
- **v2 main:** `for each partition { reset+stage partition; commitStaged() }`

**v1 must NOT entangle staging policy with commit logic.** The package boundary enforces this:
`internal/generate` never calls `git add`; `internal/git/stage.go` is only called from the CLI layer.

## 5. Process Model (§11.2)

Single process. Shells out to `git` (multiple times) and to the agent CLI (once per attempt).
All subprocesses inherit Stagecoach's working directory (repo root) and environment, with a
controlled minimal set of extra env vars passed to the agent only if the manifest's `[env]` requests.

**Signal handling (§18.4):** SIGINT/SIGTERM propagates to the running child's process group
(`SysProcAttr.Setpgid=true` so we can kill the whole tree), then triggers the rescue path if
the snapshot was taken. The default signal handler is restored before the final `update-ref`
(matching commit-pi's `trap - INT TERM`).

## 6. Dependencies (PRD §22.3)

| Dependency | Purpose | Notes |
|---|---|---|
| **Go 1.22+** stdlib | `os/exec`, `os/signal`, `encoding/json`, `context` | All ergonomic for subprocess + git plumbing. |
| **spf13/cobra** | CLI framework, subcommands (`providers`, `config`) | Recommended over bare `flag` / `urfave/cli`. |
| **pelletier/go-toml/v2** | Config file + manifest TOML parsing | **Note:** does NOT support `omitempty` in struct tags. Use pointers for nullable fields. |
| **No git library** | Shells out to real `git` binary | Matches commit-pi reference; identical semantics; smaller dep surface. go-git rejected. |

## 7. The Atomic Commit Invariant (§18.1)

> The repository's refs and index are modified **only** at the final `update-ref` step,
> and only if HEAD is unchanged since the snapshot. Every code path that does not reach a
> successful `update-ref` leaves the repository byte-for-byte unchanged (modulo harmless
> dangling objects in the object store, which `git gc` reaps).

This is enforced by: (1) `write-tree` is read-only w.r.t. refs, (2) `commit-tree` is read-only
w.r.t. refs (creates a dangling object), (3) `update-ref` CAS is the only ref mutation,
(4) the index is NEVER reset by Stagecoach.
