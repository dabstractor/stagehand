// adapter.go is the CommitHookRunner injection adapter (P1.M3.T2.S1 — the wiring seam between
// generate.CommitStaged and this package's runner). DefaultRunner is the production
// generate.CommitHookRunner: it delegates to the package-level RunCommitHooks/RunPostCommit,
// translating the inlined (dryRun, verbose) params to HookOpts.
//
// This is a SEPARATE file from runner.go (the S1/S2 core) so the M3.T2 wiring layer never collides
// with parallel runner.go edits. DefaultRunner satisfies generate.CommitHookRunner STRUCTURALLY
// (Go duck typing) — it does NOT import internal/generate, so it adds no edge to the
// generate↔hooks import graph (hooks already imports generate in runner.go for RescueError; this
// adapter reuses no generate symbol). Wired into generate.Deps by pkg/stagehand.buildDeps; also
// used by the external hooks_freeze_test.
package hooks

import (
	"context"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/ui"
)

// DefaultRunner is the production CommitHookRunner. Its zero value is ready to use. It satisfies
// generate.CommitHookRunner structurally (no generate import): RunCommitHooks delegates to the
// package-level RunCommitHooks, and RunPostCommit to RunPostCommit, translating the inlined
// (dryRun, verbose) to HookOpts{DryRun, Verbose}.
type DefaultRunner struct{}

// RunCommitHooks delegates to the package-level RunCommitHooks (S1/S2), translating the inlined
// (dryRun, verbose) to HookOpts. Returns the (possibly re-treed) finalTree and the (possibly
// hook-annotated) finalMsg for commit-tree.
func (DefaultRunner) RunCommitHooks(ctx context.Context, g git.Git, cfg config.Config,
	snapshotTree, parentSHA, msg string, dryRun bool, verbose *ui.Verbose) (string, string, error) {
	return RunCommitHooks(ctx, g, cfg, snapshotTree, parentSHA, msg, HookOpts{DryRun: dryRun, Verbose: verbose})
}

// RunPostCommit delegates to the package-level RunPostCommit (S1/S2), translating the inlined
// (dryRun, verbose) to HookOpts. Always returns nil (post-commit exit code is disregarded — FR-V7).
func (DefaultRunner) RunPostCommit(ctx context.Context, g git.Git, cfg config.Config,
	dryRun bool, verbose *ui.Verbose) error {
	return RunPostCommit(ctx, g, cfg, HookOpts{DryRun: dryRun, Verbose: verbose})
}

// ReconcileIndex delegates to the package-level ReconcileIndex (report Finding F1), translating the
// inlined (dryRun, verbose) to HookOpts. Best-effort: the caller logs a non-nil error at --verbose
// and NEVER undoes the commit (the commit already landed).
func (DefaultRunner) ReconcileIndex(ctx context.Context, g git.Git, snapshotTree, finalTree string,
	dryRun bool, verbose *ui.Verbose) error {
	return ReconcileIndex(ctx, g, snapshotTree, finalTree, HookOpts{DryRun: dryRun, Verbose: verbose})
}
