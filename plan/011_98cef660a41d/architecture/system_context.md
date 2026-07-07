# System Context — v2.5 Reliability Hardening

## Project State

Stagecoach is a mature Go CLI tool (single static binary) that writes git commit messages
using AI coding agents the user already has installed. The codebase has implemented the
full PRD through v2.4 (hook execution on the commit path, multi-turn fallback, multi-commit
decomposition, per-role models, payload exclusions, message shaping, git hook mode, tool
integrations, and the entire v1.0 single-commit core).

## The v2.5 Delta (This Changeset)

Two reliability-hardening fixes surfaced by a large-diff failure investigation:

### 1. FR3j — Closed-loop token-budget guarantee (§9.1)

**Problem:** The `token_limit` water-fill (FR3i) sizes diff bodies by subtracting *estimates*
of the non-body parts from `token_limit`. This is an open-loop budget whose estimation drift
let the *assembled* prompt land slightly over the limit (observed ~152K tokens delivered
against a 150K `token_limit`).

**Fix:** After the water-fill, stagecoach assembles the *actual* full prompt (system prompt +
`BuildUserPayload(gatedDiff)`), measures it with the same `chars/4` estimator, and re-trims
until it fits. This makes "the prompt never exceeds `token_limit`" a **hard invariant**.
FR3i is reframed as the first-cut allocator under that guarantee.

**Scope:** Every role that routes through the gate (message, planner, arbiter). The stager
bypasses the gate (no diff). Multi-turn (FR-T12) deliberately ignores `token_limit` and is
out of scope.

### 2. §18.5 — Stale lock-file reaping

**Problem:** `flock` auto-releases on process death, so the *lock* never goes stale — but
the lock *file* survived any non-deferred exit (the signal-rescue path's `os.Exit`,
`SIGKILL`, a crash) and accumulated as unbounded disk litter.

**Fix:** (a) A lock file is stale iff its recorded `pid` is a dead process on its `hostname`;
stagecoach reaps such files on `Acquire` (self-healing, never a live pid). (b) The signal-rescue
path now releases the file before exiting (reaping stays as the backstop for `SIGKILL`/crash).

**Scope:** No commit/CAS/rescue behavior changes.

## Codebase Conventions (from research)

- **Leaf-purity invariant:** `internal/git` does NOT import `internal/prompt`. Cross-package
  seams use function injection (the `TokenEstimator` pattern in `reserve.go` is the precedent).
- **Single estimator:** `git.EstimateTokens(s string) int` = `ceil(runes/4)`. Used everywhere;
  no second formula.
- **Pure helpers:** Budget/gate logic in `tokengate.go` is PURE (no git, no ctx, no I/O) for
  exhaustive unit testing without a repo.
- **Build-tag split:** Cross-platform code splits via `//go:build` tags
  (`lock_unix.go` / `lock_windows.go`, `procgroup_unix.go` / `procgroup_windows.go`).
- **Injected seams in signal package:** `signal.Options` has `Kill`, `Exit`, `RescueFormat`,
  `Out` — all injectable for testing. The package is stdlib-only (cannot import stagecoach packages).
- **Singleton pattern:** `lock.current atomic.Pointer[Locker]` and `signal.active atomic.Pointer[Handler]`
  enable nil-safe package wrappers (`SetSnapshot`, `RegisterChild`, etc.).
