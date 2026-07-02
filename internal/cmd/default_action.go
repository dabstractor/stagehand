package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/decompose"
	"github.com/dustin/stagehand/internal/exitcode"
	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/ui"
	"github.com/dustin/stagehand/pkg/stagehand"
)

// runDefault is the root command's default action (PRD §15.1): commit staged changes, auto-staging all
// if nothing is staged and auto_stage_all is on (§9.4 FR16–FR20). It delegates generation+commit to the
// PUBLIC API pkg/stagehand.GenerateCommit (US12 dogfooding), renders the FR42 success report, and maps
// every §15.4 outcome by RETURNING an error that exitcode.For (S1) + main translate to an exit code.
// It never calls os.Exit (only main does). Auto-stage lives HERE (CLI layer), not in the orchestrator
// (PRD §11.3 — CommitStaged/GenerateCommit never call git add).
//
// Output streams (design §5): stdout = the result (FR42 commit report, or the dry-run message — pipeable
// per §15.5); stderr = notices + diagnostics (FR18 auto-stage notice, the §18.3 rescue block, the §13.5
// CAS message). To avoid main double-printing a detailed message, the rescue/CAS paths return a SILENT
// exitcode.New(code, nil) (ExitError.Error()=="" → main's `err.Error() != ""` guard skips printing).
func runDefault(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context() // S1's Execute set this; P1.M4.T2 swaps it for a signal-aware ctx later.
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	cfg := Config()
	if cfg == nil {
		// Defensive: PersistentPreRunE always loads config for the root action (cmd.Name()=="stagehand"),
		// so this is unreachable in practice. Still fail loudly rather than nil-deref.
		return exitcode.New(exitcode.Error, errors.New("stagehand: configuration not loaded"))
	}

	u := ui.New(stdout, stderr, ui.ResolveColor(cfg.NoColor, ui.IsTerminal(os.Stdout)))

	repoDir, err := os.Getwd()
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagehand: getwd: %w", err))
	}
	g := git.New(repoDir)

	// ---- §9.4 auto-stage-all state machine (FR16–FR20) ----
	// FR20: --all/-a forces `git add -A` BEFORE the staged check, even if something is already staged.
	if flagAll {
		if err := g.AddAll(ctx); err != nil {
			return exitcode.New(exitcode.Error, fmt.Errorf("git add -A: %w", err))
		}
	}

	hasStaged, err := g.HasStagedChanges(ctx)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("staged changes check: %w", err))
	}

	if !hasStaged {
		// FR-M1 (P4.M1.T1.S1): nothing staged + dirty tree + decompose enabled → decompose (NO AddAll).
		if shouldDecompose(cfg, flagDryRun, flagNoAutoStage) {
			status, err := g.StatusPorcelain(ctx)
			if err != nil {
				return exitcode.New(exitcode.Error, fmt.Errorf("git status --porcelain: %w", err))
			}
			if status == "" {
				return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit.")) // clean tree
			}
			return runDecompose(ctx, stdout, stderr, u, cfg, g) // planner gets the working-tree diff
		}
		switch {
		case flagNoAutoStage:
			// FR19: --no-auto-stage + nothing staged → exit 2 "Nothing staged." (--no-auto-stage wins
			// over cfg.AutoStageAll). main prints "stagehand: Nothing staged." (non-nil err).
			return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing staged."))
		case cfg.AutoStageAll:
			// FR16/FR18: auto-stage all, print the transparent notice, re-check.
			if err := g.AddAll(ctx); err != nil {
				return exitcode.New(exitcode.Error, fmt.Errorf("git add -A: %w", err))
			}
			n, err := g.StagedFileCount(ctx)
			if err != nil {
				return exitcode.New(exitcode.Error, fmt.Errorf("git diff --cached --name-only: %w", err))
			}
			if n == 0 {
				// Clean tree: AddAll staged nothing. Skip the FR18 "(0 files)" notice and go straight
				// to the FR17 exit-2 path (Issue 7 cosmetic fix; D5).
				return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
			}
			fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d files).", n))) // FR18 (text verbatim, em-dash; colorized)
			hasStaged, err = g.HasStagedChanges(ctx)
			if err != nil {
				return exitcode.New(exitcode.Error, fmt.Errorf("staged changes check: %w", err))
			}
			if !hasStaged {
				// FR17: clean tree even after auto-stage.
				return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
			}
		default:
			// cfg.AutoStageAll==false (config), no --no-auto-stage flag → don't auto-stage; exit 2.
			return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
		}
	}

	// ---- Staged (or staged via auto-stage): capture isUnborn, then generate+commit via the public API ----
	// isUnborn feeds DiffTree's isRoot for the FR42 report (design §1). Captured BEFORE GenerateCommit;
	// AddAll/HasStagedChanges don't change isUnborn (staging isn't a commit).
	_, isUnborn, err := g.RevParseHEAD(ctx)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("git rev-parse HEAD: %w", err))
	}

	// §3: hand the CLI-resolved config (cfg, loaded ONCE by PersistentPreRunE — which honors
	// --config via flagConfig→ConfigPathOverride) into GenerateCommit via Options.Config.
	// resolveConfig then SKIPS its own config.Load (S1's opts.Config != nil branch): --config is
	// honored on the default action (Issue 1) and the §19 repo-local notice prints once (Issue 5).
	// Provider/Model/Timeout/VerboseOn below re-assert the Options>everything precedence (redundant
	// for the CLI path, mandatory for the standalone-library Options.Config==nil contract).

	// FR51b: validate the provider + resolve the invocation label. Build the registry once;
	// surface DecodeUserOverrides errors (was ignored before — consistent with runDecompose).
	overrides, oerr := provider.DecodeUserOverrides(cfg.Providers)
	if oerr != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("provider overrides: %w", oerr))
	}
	reg := provider.NewRegistry(overrides)

	// FR51b: resolve the message role's provider (auto-detect mirrors pkg/stagehand.buildDeps) so
	// the label names the resolved invocation even when --provider is unset.
	labelProvider := cfg.Provider
	if labelProvider == "" {
		var installed []string
		for _, m := range reg.List() {
			if reg.IsInstalled(m) {
				installed = append(installed, m.Name)
			}
		}
		labelProvider = reg.DefaultProvider(installed)
	}
	// Validate an EXPLICIT provider (autodetect is validated inside GenerateCommit/buildDeps).
	if cfg.Provider != "" {
		if _, ok := reg.Get(cfg.Provider); !ok {
			return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q", cfg.Provider))
		}
	}
	u.Progress(ui.ProgressLabel("Generating", cfg.Model, labelProvider))

	res, err := stagehand.GenerateCommit(ctx, stagehand.Options{
		Config:    cfg,
		Provider:  cfg.Provider,
		Model:     cfg.Model,
		Timeout:   cfg.Timeout,
		DryRun:    flagDryRun,
		Verbose:   stderr,
		VerboseOn: cfg.Verbose,
	})
	if err != nil {
		return handleGenError(stderr, err) // §4: rescue/CAS/timeout/nothing/generic matrix
	}

	// ---- Success ----
	if flagDryRun || res.CommitSHA == "" {
		// Dry-run (Appendix B.3): stdout = the message ONLY (§15.5 pipe use case). The "↳ Generating…"
		// progress is already on stderr (u.Progress above). "(no commit created)" → STDERR so stdout stays
		// clean for piping (FR49 / P1.M4.T4.S1). No commit was created.
		printDryRunMessage(stdout, res.Message)
		fmt.Fprintln(stderr, "(no commit created)") // Appendix B.3; stderr keeps stdout clean for piping
		return nil                                  // exit 0
	}

	// Commit path: FR42 report. Compute the DiffTree file list ourselves — pkg/stagehand.Result drops
	// Changes (design §1). Best-effort: a DiffTree error post-commit is non-fatal (commit already landed).
	changes, derr := g.DiffTree(ctx, res.CommitSHA, isUnborn)
	if derr != nil {
		changes = nil // report without the file list; do NOT fail the success
	}
	printCommitReport(stdout, res, changes)
	return nil // exit 0
}

// handleGenError maps a GenerateCommit error to the §15.4 outcome WITH the right user-facing output. It
// prints the detailed message for rescue/CAS (to stderr) and returns a SILENT exitcode.New(code, nil) so
// main does not double-print; for friendly/generic errors it returns exitcode.New(code, err) so main
// prints "stagehand: <msg>". (design §4)
func handleGenError(stderr io.Writer, err error) error {
	// Dry-run generation failure (PRD §9.12 FR49 + bugfix-002 Issue 4): --dry-run runs the full
	// pipeline (incl. the snapshot), so a timeout or parse/dedupe-exhaustion surfaces a
	// *generate.RescueError from the library. For a "preview" that was never going to commit, the
	// §18.3 manual commit-tree recovery recipe is misleading — so print a short stderr line, map to
	// exit 1 (exitcode.Error), and omit the recipe. (flagDryRun is a package var, root.go:40.)
	if flagDryRun {
		var re *generate.RescueError
		if errors.As(err, &re) { // dry-run timeout OR rescue (both are *RescueError)
			msg := "could not generate a commit message; run without --dry-run to see retries and the recovery recipe"
			if errors.Is(err, generate.ErrTimeout) {
				msg = "generation timed out; run without --dry-run to see the recovery recipe"
			}
			fmt.Fprintln(stderr, msg)
			return exitcode.New(exitcode.Error, nil) // exit 1, silent (already printed)
		}
	}

	var re *generate.RescueError
	if errors.As(err, &re) { // covers BOTH ErrTimeout and ErrRescue (both are *RescueError)
		fmt.Fprintln(stderr, generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate))
		code := exitcode.Rescue
		if errors.Is(err, generate.ErrTimeout) { // timeout → 124; rescue → 3 (timeout checked first)
			code = exitcode.Timeout
		}
		return exitcode.New(code, nil) // silent → main prints nothing; exit code honored
	}
	var ce *generate.CASError
	if errors.As(err, &ce) {
		fmt.Fprintln(stderr, ce.Error())         // the §13.5 "HEAD moved…" message
		return exitcode.New(exitcode.Error, nil) // silent; exit 1
	}
	if errors.Is(err, generate.ErrNothingToCommit) {
		return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit.")) // exit 2; main prints
	}
	return exitcode.New(exitcode.Error, err) // generic (render/exec/write-tree/unknown-provider/…): exit 1
}

// shortSHA returns the 7-char short form of a full SHA (Appendix B uses 7-char SHAs). Returns sha
// unchanged if shorter than 7 (defensive; a real SHA is 40 hex chars).
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// printCommitReport renders the FR42 success report to w (stdout): `[<short-sha>] <subject>` followed by
// the DiffTree name-status file list, one entry per line. Isolated as a function so P1.M4.T3 can restyle
// it (the "↳ Created" decoration + color) without touching runDefault's flow (design §10). Format matches
// PRD Appendix B.1's data; the ↳ progress wrapper is P1.M4.T3.
func printCommitReport(w io.Writer, res stagehand.Result, changes []git.FileChange) {
	fmt.Fprintf(w, "[%s] %s\n", shortSHA(res.CommitSHA), res.Subject)
	for _, c := range changes {
		if c.SrcPath != "" { // R/C rename/copy: "<status>\t<src>\t<dst>" → show "R100  old → new"
			fmt.Fprintf(w, "%s  %s → %s\n", c.Status, c.SrcPath, c.Path)
			continue
		}
		fmt.Fprintf(w, "%s  %s\n", c.Status, c.Path)
	}
}

// shouldDecompose is the FR-M1/M2 routing predicate (PURE — no I/O, no package-flag reads). True iff
// the default action should route to multi-commit decomposition instead of the v1 AddAll→GenerateCommit
// path. Decompose activates iff NOTHING is staged (caller guarantees via hasStaged), auto-stage-all is
// on, the user did not opt out (--single/--no-decompose/--commits 1 ⇒ cfg.Single), and --dry-run is not
// set (decompose commits; --dry-run honors the preview).
func shouldDecompose(cfg *config.Config, dryRun, noAutoStage bool) bool {
	if cfg == nil {
		return false
	}
	if cfg.Single || cfg.Commits == 1 { // --single/--no-decompose/--commits 1 → v1
		return false
	}
	if dryRun { // decompose commits; --dry-run → single preview
		return false
	}
	return cfg.AutoStageAll && !noAutoStage // FR-M1 trigger context (auto-stage on)
}

// runDecompose builds decompose.Deps (ResolveRoles for the four roles) and runs the pipeline.
// Prints each landed commit's FR42 report to stdout (including partial landings on FR-M12), then maps
// the error. Calls internal/decompose.Decompose directly (pkg/stagehand.Decompose is P4.M2.T1.S1 —
// not yet shipped; that task swaps this one call site to the public wrapper).
func runDecompose(ctx context.Context, stdout, stderr io.Writer, u *ui.UI, cfg *config.Config, g git.Git) error {
	overrides, err := provider.DecodeUserOverrides(cfg.Providers)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("decompose: provider overrides: %w", err))
	}
	reg := provider.NewRegistry(overrides)
	roleManifests, roleModels, err := decompose.ResolveRoles(*cfg, reg)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("decompose: %w", err))
	}
	verbose := ui.NewVerbose(stderr, cfg.Verbose)
	// FR51b: --verbose enumerates all four roles (planner/stager/message/arbiter).
	verbose.VerboseRoles([]ui.RoleLine{
		{Name: "planner", Model: roleModels.Planner.Model, Provider: roleModels.Planner.Provider, Reasoning: roleModels.Planner.Reasoning},
		{Name: "stager", Model: roleModels.Stager.Model, Provider: roleModels.Stager.Provider, Reasoning: roleModels.Stager.Reasoning},
		{Name: "message", Model: roleModels.Message.Model, Provider: roleModels.Message.Provider, Reasoning: roleModels.Message.Reasoning},
		{Name: "arbiter", Model: roleModels.Arbiter.Model, Provider: roleModels.Arbiter.Provider, Reasoning: roleModels.Arbiter.Reasoning},
	})
	// FR51b: main line surfaces the PLANNER role's resolved invocation.
	u.Progress(ui.ProgressLabel("Decomposing", roleModels.Planner.Model, roleModels.Planner.Provider))
	deps := decompose.Deps{
		Git:      g,
		Registry: reg,
		Config:   *cfg,
		Roles:    roleManifests,
		Verbose:  verbose,
		Out:      stderr, // the loop prints §18.3 rescue + §13.5 CAS here (P3.M4.T1.S2)
	}

	res, derr := decompose.Decompose(ctx, deps)
	for _, c := range res.Commits { // print landed commits (success AND FR-M12 partial)
		printDecomposeCommit(stdout, c)
	}
	if derr != nil {
		return handleDecomposeError(derr) // suppress re-print; map exit code
	}
	return nil
}

// printDecomposeCommit renders the FR42 success line for one decompose.CommitResult to w (stdout):
// `[<short-sha>] <subject>` then one line per file (mirrors printCommitReport; input type differs).
func printDecomposeCommit(w io.Writer, c decompose.CommitResult) {
	fmt.Fprintf(w, "[%s] %s\n", shortSHA(c.SHA), c.Subject)
	for _, f := range c.Files {
		if f.SrcPath != "" { // R/C rename/copy
			fmt.Fprintf(w, "%s  %s → %s\n", f.Status, f.SrcPath, f.Path)
			continue
		}
		fmt.Fprintf(w, "%s  %s\n", f.Status, f.Path)
	}
}

// handleDecomposeError maps a decompose error to the §15.4 outcome WITHOUT double-printing:
// rescue/CAS are already printed by the loop → silent exitcode.New(exitcode.For(err), nil);
// planner/safety/infra → exitcode.New(exitcode.Error, err) so main prints 'stagehand: <msg>'.
func handleDecomposeError(err error) error {
	var re *generate.RescueError
	var ce *generate.CASError
	if errors.As(err, &re) || errors.As(err, &ce) { // *DecomposeRescueError unwraps to *RescueError
		return exitcode.New(exitcode.For(err), nil) // SILENT — loop already printed to stderr
	}
	return exitcode.New(exitcode.Error, err) // planner/safety/infra — main prints
}

// printDryRunMessage writes the generated message to w (stdout) for --dry-run. stdout = message ONLY
// (§15.5: `stagehand --dry-run --no-color | tee /tmp/msg.txt`). Decorations are P1.M4.T3/T4 (design §9).
func printDryRunMessage(w io.Writer, msg string) {
	fmt.Fprintln(w, msg)
}
