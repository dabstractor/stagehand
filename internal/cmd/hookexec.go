// Package cmd implements the hook exec cobra leaf for Stagecoach (PRD §9.20 FR-H4/H5/H6).
// It loads config ITSELF (hookCmd's no-op PersistentPreRunE skips root's load), resolves the
// message manifest (mirroring runDefault/buildDeps), and applies the never-block exit-code mapping.
//
// Registered via init() — ZERO edits to hook.go (S2) or root.go (providers.go pattern).
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/exclude"
	"github.com/dabstractor/stagecoach/internal/exitcode"
	"github.com/dabstractor/stagecoach/internal/generate"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/hook"
	"github.com/dabstractor/stagecoach/internal/provider"
	"github.com/dabstractor/stagecoach/internal/ui"
)

var flagHookExecStrict bool

var hookExecCmd = &cobra.Command{
	Use:   "exec <msg-file> [<source> [<sha>]]",
	Short: "Generate a commit message into git's prepare-commit-msg file (called by the installed hook)",
	Long: `Called by stagecoach's prepare-commit-msg hook — not by users. Generates a message for the
staged diff and writes it at the top of <msg-file>, preserving git's comment block. No-op (exit 0)
when a message source is present (message/template/merge/squash/commit) or nothing is staged. Any
generation failure leaves the file untouched and exits 0 (never block) unless --strict aborts. (PRD §9.20)`,
	Args:          cobra.RangeArgs(1, 3),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runHookExec,
}

func init() {
	hookExecCmd.Flags().BoolVar(&flagHookExecStrict, "strict", false,
		"Abort the commit on generation failure (default: never block — exit 0 and leave the message empty)")
	hookCmd.AddCommand(hookExecCmd) // S2's hookCmd; NO edit to hook.go
}

func runHookExec(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	stderr := cmd.ErrOrStderr()

	repoDir, err := os.Getwd()
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: getwd: %w", err))
	}

	// §9.22 FR-E4: --edit on hook exec is a usage error (git already opens the editor).
	if cmd.Flags().Changed("edit") {
		fmt.Fprintln(stderr, "stagecoach: --edit is not valid with hook exec (git already opens the editor)")
		return exitcode.New(exitcode.Error, nil)
	}

	g := git.New(repoDir)
	msgFile := args[0]
	source := ""
	if len(args) >= 2 {
		source = args[1]
	}

	// neverBlock is the FR-H5 contract: ONE stderr line; exit 0 unless --strict (then exit 1, silent).
	neverBlock := func(err error) error {
		fmt.Fprintf(stderr, "stagecoach: %s\n", err)
		if flagHookExecStrict {
			return exitcode.New(exitcode.Error, nil) // silent non-zero → aborts the commit
		}
		return nil // exit 0 → commit proceeds to an empty editor
	}

	// hookCmd's no-op PersistentPreRunE (S2) skipped root's config.Load → load it ourselves.
	cfg, err := config.Load(ctx, config.LoadOpts{
		ConfigPathOverride: flagConfig,
		RepoDir:            repoDir,
		Flags:              cmd.Flags(),
	})
	if err != nil {
		return neverBlock(fmt.Errorf("config: %w", err))
	}

	// Resolve the message-role manifest (mirror runDefault / buildDeps — the accepted 3rd duplication).
	overrides, oerr := provider.DecodeUserOverrides(cfg.Providers)
	if oerr != nil {
		return neverBlock(fmt.Errorf("provider overrides: %w", oerr))
	}
	reg := provider.NewRegistry(overrides)

	msgProvider, msgModel, _ := config.ResolveRoleModel("message", *cfg)
	name := msgProvider
	if name == "" {
		var installed []string
		for _, m := range reg.List() {
			if reg.IsInstalled(m) {
				installed = append(installed, m.Name)
			}
		}
		name = reg.DefaultProvider(installed)
	}
	m, ok := reg.Get(name)
	if !ok {
		return neverBlock(fmt.Errorf("unknown provider %q", name))
	}
	if verr := m.ValidateModel(msgModel); verr != nil {
		return neverBlock(fmt.Errorf("provider %q: %w", name, verr))
	}
	if !reg.IsInstalled(m) {
		return neverBlock(fmt.Errorf("provider %q: command %q not found", name, m.DetectCommand()))
	}
	if cfg.Output != nil {
		m.Output = cfg.Output
	}
	if cfg.StripCodeFence != nil {
		m.StripCodeFence = cfg.StripCodeFence
	}

	verbose := ui.NewVerbose(stderr, cfg.Verbose)
	excludes, eerr := exclude.ResolveExcludePathspecs(*cfg, repoDir, verbose)
	if eerr != nil {
		return neverBlock(fmt.Errorf("resolve excludes: %w", eerr))
	}

	// Best-effort progress line (stderr; the message itself goes to msgFile, never stdout).
	labelProvider := name
	u := ui.New(cmd.OutOrStdout(), stderr, ui.ResolveColor(cfg.NoColor, ui.IsTerminal(os.Stdout)))
	if rerr := hook.Run(ctx, generate.Deps{
		Git:      g,
		Manifest: m,
		Verbose:  verbose,
		Excludes: excludes,
		Progress: func() { u.Progress(ui.ProgressLabel("Generating", msgModel, labelProvider)) },
	}, *cfg, msgFile, source); rerr != nil && !errors.Is(rerr, hook.ErrNoOp) {
		return neverBlock(rerr) // generation failure → never-block (or strict abort)
	}
	return nil // success OR intended no-op → exit 0
}
