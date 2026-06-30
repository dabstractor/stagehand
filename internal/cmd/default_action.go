package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

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

	// §3: re-apply the CLI-resolved provider/model/timeout (Layer-7 flags already applied by
	// PersistentPreRunE) as Options — GenerateCommit re-loads config with Flags:nil, so opts is how the
	// CLI flags take effect (opts override is highest precedence in resolveConfig).

	// Validate the provider before printing the progress label (Issue 7: avoid optimistically
	// announcing an invalid provider). This mirrors the same resolution logic as
	// pkg/stagehand.buildDeps but is intentionally lightweight — just the existence check.
	if cfg.Provider != "" {
		overrides, _ := provider.DecodeUserOverrides(cfg.Providers)
		reg := provider.NewRegistry(overrides)
		if _, ok := reg.Get(cfg.Provider); !ok {
			return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q", cfg.Provider))
		}
	}

	// Build the progress label now that the provider is validated.
	label := "Generating"
	if cfg.Provider != "" {
		label += " with " + cfg.Provider
		if cfg.Model != "" {
			label += " (" + cfg.Model + ")"
		}
	}
	label += "…"
	u.Progress(label)

	res, err := stagehand.GenerateCommit(ctx, stagehand.Options{
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

// printDryRunMessage writes the generated message to w (stdout) for --dry-run. stdout = message ONLY
// (§15.5: `stagehand --dry-run --no-color | tee /tmp/msg.txt`). Decorations are P1.M4.T3/T4 (design §9).
func printDryRunMessage(w io.Writer, msg string) {
	fmt.Fprintln(w, msg)
}
