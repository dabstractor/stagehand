// Package cmd implements the lock command group for Stagecoach (PRD §9.27 FR-K4).
// It provides a `lock` cobra command group with one read-only leaf subcommand:
// `status` (print the current repo's run-lock holder — path, pid/host/repo/timestamp/
// snapshot, liveness, and (Unix) orphan status).
//
// The group defines a no-op PersistentPreRunE that OVERRIDES root's config.Load —
// cobra runs only the nearest ancestor's, so `lock status` works OUTSIDE a git repo
// (a diagnostic must run anywhere CWD is) and never triggers config.Load's first-run
// bootstrap write (FR-B3). Same rationale as hook.go/integrate.go.
//
// Registered via init() — ZERO edits to root.go (the providers.go/hook.go/integrate.go
// pattern). `stagecoach lock status` is READ-ONLY: it never acquires the flock and never
// breaks/removes a lock (FR52 preserved). The user decides whether to kill/rm.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/lock"
)

// lockCmd is the PRD §9.27 lock command group. No RunE → bare `stagecoach lock` prints
// help. Its no-op PersistentPreRunE OVERRIDES root's (cobra runs only the nearest):
// `lock status` needs no config and must run outside a git repo — and must not trigger
// config.Load's first-run bootstrap write (FR-B3). Same rationale as hook.go.
var lockCmd = &cobra.Command{
	Use:               "lock",
	Short:             "Inspect the per-repo run lock (FR52/§9.27)",
	Long:              `Read-only diagnostics for stagecoach's per-repo run lock (PRD §9.27 FR-K4).`,
	SilenceErrors:     true,
	SilenceUsage:      true,
	PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, // OVERRIDES root's config.Load
}

var lockStatusCmd = &cobra.Command{
	Use:           "status",
	Short:         "Print the run lock holder's pid/host/repo/liveness/orphan-status",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runLockStatus,
}

func init() {
	lockCmd.AddCommand(lockStatusCmd)
	rootCmd.AddCommand(lockCmd) // register on root — NO edit to root.go (hook/integrate/providers pattern)
}

// runLockStatus implements `stagecoach lock status` (PRD §9.27 FR-K4): a READ-ONLY
// diagnostic that prints the current repo's run-lock state — the lock path, the
// holder's parsed pid/hostname/repo/timestamp/snapshot, whether the holder is alive,
// and (Unix) whether it appears orphaned (reparented). It never acquires the flock and
// never breaks/removes a lock (FR52 preserved); the user decides whether to kill/rm.
// With no lock held it prints "no run lock for <repoDir>" and exits 0 (a read that found
// nothing is a success, not an error). It works outside a git repo (the lock group's
// no-op PersistentPreRunE skips config.Load). Consumes lock.Status (P1.M3.T1.S1).
func runLockStatus(cmd *cobra.Command, _ []string) error {
	repoDir, err := os.Getwd()
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
	}
	path, contents, alive, orphan, err := lock.Status(repoDir)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach lock status: %w", err))
	}
	out := cmd.OutOrStdout()
	if path == "" {
		fmt.Fprintf(out, "no run lock for %s\n", repoDir) // a read that found nothing → exit 0
		return nil
	}
	fmt.Fprintf(out, "Lock: %s\n", path)
	fmt.Fprintf(out, "  pid:       %s\n", contents.Pid)
	fmt.Fprintf(out, "  hostname:  %s\n", contents.Hostname)
	fmt.Fprintf(out, "  repo:      %s\n", contents.Repo)
	fmt.Fprintf(out, "  timestamp: %s\n", contents.Timestamp)
	if contents.Snapshot != "" {
		fmt.Fprintf(out, "  snapshot:  %s\n", contents.Snapshot)
	}
	fmt.Fprintf(out, "  alive:     %v\n", alive)
	switch {
	case orphan:
		fmt.Fprintln(out, "  orphaned:  true (holder reparented — launcher has exited)")
	case alive:
		fmt.Fprintln(out, "  orphaned:  false") // Windows always lands here (processAlive is always true)
	default:
		fmt.Fprintln(out, "  orphaned:  unknown (holder is dead)")
	}
	return nil // exit 0 — even when the holder is dead/orphaned; the USER decides whether to kill/rm
}
