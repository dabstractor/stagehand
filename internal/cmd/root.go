// Package cmd implements the cobra CLI scaffold for Stagecoach (PRD §15.1/§15.2/§15.4/§21.1).
// It provides the root command with all eleven §15.2 global flags (persistent, inherited by every
// future subcommand), a PersistentPreRunE that resolves config once via config.Load(), an Execute()
// function that returns the command error (for exit-code mapping in main), and a Config() accessor
// for RunE consumers. The default action body, subcommands, signal handling, UI, and dry-run logic
// are added by sibling subtasks (S2/S3/S4/P1.M4.T2/T3/T4).
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/ui"
)

// Help output is wrapped to the LIVE terminal width so long flag descriptions stay readable and
// justified (continuation lines align at the description column). Three knobs govern the width:
const (
	// maxHelpWidth caps the wrap width even on very wide terminals. Past ~160 columns extra width
	// buys little readability and lets each flag's description sprawl across long, hard-to-scan lines.
	maxHelpWidth = 160
	// helpRightMargin is blank space reserved at the right edge of wrapped help so a line never
	// butts against the terminal boundary — easier scanning, room for the cursor / IDE chrome.
	helpRightMargin = 2
	// defaultHelpWidth is the fallback wrap width when the terminal width can't be detected (stdout
	// is piped/redirected/not a TTY). 80 is the universal safe assumption.
	defaultHelpWidth = 80
)

// detectHelpWidth is the terminal-width seam: in production it reads the live stdout terminal width
// via ui.TerminalWidth; tests override it (os.Stdout isn't a TTY under `go test`, so the production
// impl would always return 0 and exercise only the fallback). Mirrors the injectable-IO discipline
// used by config init --interactive (interactiveStdinIsTTY) and ResolveColor (isTTY param).
var detectHelpWidth = func() int { return ui.TerminalWidth(os.Stdout) }

// Version is set by main.go from the ldflags-injected `var version string` before Execute.
// cobra's Version field auto-registers --version (no -v shorthand) and prints+exits BEFORE
// PersistentPreRunE, so config does NOT load for --version.
var Version string

// Config-backed flags (resolved by config.Load via fs.Changed; registered at ZERO default so Changed
// reflects "user passed it"). See design §2 (timeout is a STRING) and §3 (zero defaults).
var (
	flagProvider string
	flagModel    string
	flagConfig   string // --config → LoadOpts.ConfigPathOverride (NOT a Config field)
	flagTimeout  string // STRING — config.Load reads via fs.GetString("timeout") (FINDING 7)
	flagVerbose  bool
	flagNoColor  bool
)

// Decompose/per-role flags (resolved by config.Load via fs.Changed; P4.M1.T1.S1). loadFlags reads
// them via fs.Changed — the &flagVar address is their use (satisfies the `unused` linter),
// exactly as flagProvider/flagModel do. Do NOT read these vars directly — cfg.Commits/Single/...
// is the source of truth after PersistentPreRunE.
var (
	flagCommits          int
	flagSingle           bool
	flagNoDecompose      bool
	flagMaxCommits       int
	flagReasoning        string
	flagPlannerProvider  string
	flagPlannerModel     string
	flagPlannerReasoning string
	flagStagerProvider   string
	flagStagerModel      string
	flagStagerReasoning  string
	flagMessageProvider  string
	flagMessageModel     string
	flagMessageReasoning string
	flagArbiterProvider  string
	flagArbiterModel     string
	flagArbiterReasoning string
)

// Behavioral flags (NOT Config fields; read directly by the default-action RunE in S2 / dry-run in S4).
var (
	flagAll         bool
	flagNoAutoStage bool
	flagDryRun      bool
)

// flagExclude holds repeatable --exclude/-x occurrences (§9.18 FR-X1). Read only via
// fs.Changed/fs.GetStringArray in config.Load's loadFlags — never read directly (same
// discipline as flagProvider/flagModel); the &flagExclude address here is its use.
var flagExclude []string

// §9.22 FR-E1 — --edit flag (flag-only; resolved by config.Load via fs.Changed). No env/git/config-file
// counterpart — read only via fs.Changed/fs.GetBool in loadFlags (same discipline as flagContext).
var flagEdit bool

// §9.22 FR-P1 — --push flag (full 5-layer precedence; resolved by config.Load via fs.Changed).
var flagPush bool

// §9.25 FR-V5 — --no-verify flag (full 5-layer precedence; resolved by config.Load via fs.Changed).
var flagNoVerify bool

// §9.19 FR-F1/FR-F6/FR-F8 — format/locale/template flags (resolved by config.Load via fs.Changed).
var (
	flagFormat   string
	flagLocale   string
	flagTemplate string
)

// §9.19 FR-F7 — context flag (flag-only; resolved by config.Load via fs.Changed). No env/git/config-file
// counterpart — read only via fs.Changed/fs.GetString in loadFlags (same discipline as flagExclude).
var flagContext string

// loadedCfg holds the config resolved in PersistentPreRunE; nil until then. Read by Config().
var loadedCfg *config.Config

// rootCmd is the cobra root. SilenceErrors+SilenceUsage → the CLI (main) controls all output.
var rootCmd = &cobra.Command{
	Use:           "stagecoach",
	Short:         "AI-assisted commit message generator",
	SilenceErrors: true,
	SilenceUsage:  true,
	Version:       Version,
	// PersistentPreRunE runs before any RunE (root or subcommand) EXCEPT --help/--version (cobra
	// short-circuits those first). It resolves config once and stores it for RunE access.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if shouldSkipConfigLoad(cmd) {
			return nil
		}
		repoDir, err := os.Getwd()
		if err != nil {
			return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: getwd: %w", err))
		}
		cfg, err := config.Load(cmd.Context(), config.LoadOpts{
			ConfigPathOverride: flagConfig,
			RepoDir:            repoDir,
			Flags:              cmd.Flags(),
		})
		if err != nil {
			return exitcode.New(exitcode.Error, fmt.Errorf("config: %w", err))
		}
		loadedCfg = cfg
		return nil
	},
	RunE: runDefault,
}

func init() {
	pf := rootCmd.PersistentFlags()
	// §15.2 config-backed flags (zero defaults; config.Load owns Layer-7 precedence via fs.Changed).
	pf.StringVar(&flagProvider, "provider", "", "Provider/agent to use (env STAGECOACH_PROVIDER, git stagecoach.provider; default auto-detected)")
	pf.StringVar(&flagModel, "model", "", "Model override (env STAGECOACH_MODEL, git stagecoach.model; default per-manifest default_model)")
	pf.StringVar(&flagConfig, "config", "", "Path to a config file, overrides discovery (env STAGECOACH_CONFIG)")
	pf.StringVar(&flagTimeout, "timeout", "", "Generation timeout, e.g. \"120s\" or 120 (env STAGECOACH_TIMEOUT, git stagecoach.timeout; default 120s)")
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "Print resolved command, raw output, retries (env STAGECOACH_VERBOSE)")
	pf.BoolVar(&flagNoColor, "no-color", false, "Disable color (env STAGECOACH_NO_COLOR, NO_COLOR; default TTY-aware)")
	// §15.2 behavioral flags (read by S2/S4 RunE; not Config fields).
	pf.BoolVarP(&flagAll, "all", "a", false, "Run git add -A before snapshotting, even if something is staged")
	pf.BoolVar(&flagNoAutoStage, "no-auto-stage", false, "If nothing is staged, exit instead of auto-staging")
	pf.BoolVar(&flagDryRun, "dry-run", false, "Generate and print the message; do not commit")
	pf.StringArrayVarP(&flagExclude, "exclude", "x", nil,
		"Exclude matching files from the agent payload (unions with .stagecoachignore and "+
			"[generation].exclude; never excluded from the commit)")
	// §15.2 decompose/per-role flags (P4.M1.T1.S1) — bound to package vars; loadFlags reads via fs.Changed.
	pf.IntVar(&flagCommits, "commits", 0,
		"Force exactly N commits when nothing is staged (skips the planner's count decision; 0 = auto-decompose). 1 ≡ --single (env/git stagecoach.commits)")
	pf.BoolVar(&flagSingle, "single", false,
		"Bypass decomposition; force the single-commit auto-stage-all behavior (alias: --no-decompose)")
	pf.BoolVar(&flagNoDecompose, "no-decompose", false,
		"Bypass decomposition; force the single-commit auto-stage-all behavior (alias: --single)")
	pf.IntVar(&flagMaxCommits, "max-commits", 12,
		"Safety cap on auto-decompose commit count (env/git stagecoach.max_commits)")
	pf.StringVar(&flagPlannerProvider, "planner-provider", "",
		"Per-role provider override for the decomposition planner (env STAGECOACH_PLANNER_PROVIDER; git stagecoach.role.planner)")
	pf.StringVar(&flagPlannerModel, "planner-model", "",
		"Per-role model override for the decomposition planner (env STAGECOACH_PLANNER_MODEL; git stagecoach.role.planner)")
	pf.StringVar(&flagStagerProvider, "stager-provider", "",
		"Per-role provider override for the (tooled) staging agent (env STAGECOACH_STAGER_PROVIDER; git stagecoach.role.stager)")
	pf.StringVar(&flagStagerModel, "stager-model", "",
		"Per-role model override for the (tooled) staging agent (env STAGECOACH_STAGER_MODEL; git stagecoach.role.stager)")
	pf.StringVar(&flagArbiterProvider, "arbiter-provider", "",
		"Per-role provider override for the leftover arbiter (env STAGECOACH_ARBITER_PROVIDER; git stagecoach.role.arbiter)")
	pf.StringVar(&flagArbiterModel, "arbiter-model", "",
		"Per-role model override for the leftover arbiter (env STAGECOACH_ARBITER_MODEL; git stagecoach.role.arbiter)")
	// §9.19 FR-F1/FR-F6 — message format + locale flags (zero default; loadFlags reads via fs.Changed).
	pf.StringVar(&flagFormat, "format", "",
		"Message format: auto|conventional|gitmoji|plain (env STAGECOACH_FORMAT; git stagecoach.format; "+
			"[generation].format; default auto). Unknown mode is a hard error.")
	pf.StringVar(&flagLocale, "locale", "",
		"Write the message in this language (free-form name or BCP-47 tag; env STAGECOACH_LOCALE; "+
			"git stagecoach.locale; [generation].locale; default empty)")
	// §9.19 FR-F8 — message template (distinct from the LOCAL `config init --template` bool: pflag's
	// AddFlagSet skips this inherited persistent flag on `config init` since a local name already exists).
	pf.StringVar(&flagTemplate, "template", "",
		"Wrap every commit message: the literal $msg is replaced with the generated message, e.g. "+
			"\"$msg (#205)\" (env STAGECOACH_TEMPLATE; git stagecoach.template; [generation].template; "+
			"default empty). Must contain $msg. (Distinct from 'config init --template'.)")
	pf.StringVar(&flagContext, "context", "",
		"Extra context appended to the message+planner payload, e.g. \"hotfix for #812\" "+
			"(flag only; per-invocation — no env/git/config key)")
	pf.BoolVar(&flagEdit, "edit", false,
		"If set, open your editor on the generated message before committing (uses $GIT_EDITOR). "+
			"The message file includes the tree SHA + a diff-tree name-status summary; comment lines ('#') "+
			"are stripped on close. An empty message aborts (exit 1, not a rescue). The edited message "+
			"bypasses the duplicate check (git parity). In decompose mode each commit is gated. Ignored "+
			"with --dry-run; not valid with hook exec. (§9.22 FR-E1)")
	pf.BoolVar(&flagPush, "push", false,
		"If set, push to the remote (runs git push, streaming) after a fully-successful "+
			"run. Never prompts; never auto-sets upstream. On push failure the commits stand — "+
			"git's stderr is shown verbatim (including the no-upstream hint), \"commits created; "+
			"push failed\" prints, and stagecoach exits 1. Skipped on --dry-run, the nothing-to-commit "+
			"exit, and any rescue/CAS abort. (env STAGECOACH_PUSH, git stagecoach.push, config "+
			"[generation].push; default false.) (§9.22 FR-P1)")
	pf.BoolVar(&flagNoVerify, "no-verify", false,
		"Bypass pre-commit and commit-msg hooks for this commit (mirrors git commit --no-verify; "+
			"prepare-commit-msg and post-commit still run). (env STAGECOACH_NO_VERIFY, git "+
			"stagecoach.noVerify; default false.) (§9.25 FR-V5)")
	// §15.2 reasoning flags (FR-R6) — global + per-role; zero default; loadFlags reads via fs.Changed.
	pf.StringVar(&flagReasoning, "reasoning", "",
		"Global reasoning effort: off|low|medium|high (env STAGECOACH_REASONING; git stagecoach.reasoning; default off for every role)")
	pf.StringVar(&flagPlannerReasoning, "planner-reasoning", "",
		"Per-role reasoning override for the decomposition planner (env STAGECOACH_PLANNER_REASONING; git stagecoach.role.planner)")
	pf.StringVar(&flagStagerReasoning, "stager-reasoning", "",
		"Per-role reasoning override for the (tooled) staging agent (env STAGECOACH_STAGER_REASONING; git stagecoach.role.stager)")
	pf.StringVar(&flagMessageProvider, "message-provider", "",
		"Per-role provider override for the message composer (env STAGECOACH_MESSAGE_PROVIDER; git stagecoach.role.message)")
	pf.StringVar(&flagMessageModel, "message-model", "",
		"Per-role model override for the message composer (env STAGECOACH_MESSAGE_MODEL; git stagecoach.role.message)")
	pf.StringVar(&flagMessageReasoning, "message-reasoning", "",
		"Per-role reasoning override for the message composer (env STAGECOACH_MESSAGE_REASONING; git stagecoach.role.message)")
	pf.StringVar(&flagArbiterReasoning, "arbiter-reasoning", "",
		"Per-role reasoning override for the leftover arbiter (env STAGECOACH_ARBITER_REASONING; git stagecoach.role.arbiter)")
	// --version is auto-added by cobra (Version field above); --help/-h is cobra's built-in.

	// Wrap flag-usage text to the live terminal width. Cobra's default usage template calls
	// pflag.FlagUsages() (== FlagUsagesWrapped(0)); pflag v1.0.x treats cols=0 as "do not wrap",
	// so long flag descriptions render as single un-justified lines that a terminal then
	// soft-wraps raggedly. We swap that call for the stagecoachFlagUsages template func (registered
	// just below), which wraps each FlagSet's usage to helpWrapWidth() (live screen width, capped
	// at maxHelpWidth, minus a right margin) and keeps continuation lines left-aligned at the
	// description column. We derive the template from Cobra's own default via a targeted string
	// swap (not a full copy), so we stay in lock-step with Cobra's layout. Applies to every
	// subcommand: SetUsageTemplate on root is inherited by children via (*Command).UsageTemplate().
	cobra.AddTemplateFunc("stagecoachFlagUsages", flagUsagesWrapped)
	rootCmd.SetUsageTemplate(strings.NewReplacer(
		".LocalFlags.FlagUsages ", "stagecoachFlagUsages .LocalFlags ",
		".InheritedFlags.FlagUsages ", "stagecoachFlagUsages .InheritedFlags ",
	).Replace(rootCmd.UsageTemplate()))
}

// flagUsagesWrapped is the stagecoachFlagUsages template func: it renders fs's usage block wrapped
// to helpWrapWidth() columns. pflag.FlagUsagesWrapped(cols) wraps the WHOLE line (flag column +
// description) at cols, indenting continuation lines to the description column — so feeding it the
// screen-derived total lets descriptions fill whatever space is left after the flag column and stay
// justified there. Used in the swapped usage template; see init().
func flagUsagesWrapped(fs *pflag.FlagSet) string {
	return fs.FlagUsagesWrapped(helpWrapWidth())
}

// helpWrapWidth returns the total column width Cobra/pflag should wrap help output to: the live
// terminal width (detectHelpWidth), capped at maxHelpWidth, with a helpRightMargin gutter reserved
// on the right; defaultHelpWidth when the width is unknown (piped/non-TTY). Always returns a
// positive width. (pflag cols=0 would disable wrapping entirely — one long line per flag — so this
// guarantees a deterministic, screen-fit, justified block instead.)
func helpWrapWidth() int {
	w := detectHelpWidth()
	if w <= 0 {
		w = defaultHelpWidth
	}
	if w > maxHelpWidth {
		w = maxHelpWidth
	}
	if w > helpRightMargin {
		w -= helpRightMargin
	}
	return w
}

// shouldSkipConfigLoad returns true for commands that operate on the config PATH or FILE
// itself, not the resolved config — so they work outside a git repo and never need the
// git-config layer. Matches config init/path/upgrade (help/version are already short-circuited
// by cobra). Upgrade operates on the config FILE (in-place rewrite), not the resolved config.
func shouldSkipConfigLoad(cmd *cobra.Command) bool {
	name := cmd.Name()
	return name == "init" || name == "path" || name == "upgrade"
}

// Config returns the config resolved by PersistentPreRunE, or nil if it was skipped/hasn't run.
// Used by the default action (S2) and subcommands (S3/S4). Safe to call from any RunE.
func Config() *config.Config { return loadedCfg }

// Execute runs the root command with the given context (set on rootCmd so PersistentPreRunE can read
// it via cmd.Context() for config.Load's cancellation seam). Returns the command error (main maps it
// to an exit code via exitcode.For). Does NOT call os.Exit.
func Execute(ctx context.Context) error {
	rootCmd.Version = Version // sync from package var (set by main before Execute)
	if ctx != nil {
		rootCmd.SetContext(ctx)
	}
	return rootCmd.Execute()
}
