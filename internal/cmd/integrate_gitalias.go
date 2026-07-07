package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/integrate"
)

const (
	gitAliasTarget       = "git-alias"   // Entry.Name()
	defaultAliasName     = "stagecoach"  // default alias name → `git stagecoach`
	stagecoachAliasValue = "!stagecoach" // the stored value (incl. `!`); command part is "stagecoach"
)

var flagAliasName string // --alias-name (local on integrateInstallCmd AND integrateRemoveCmd)

func init() {
	// Register --alias-name on BOTH leaves (you remove the alias by name). Shared var.
	// Default "" → resolved to "stagecoach" inside the entry. hook.go's --strict is the
	// local-flag precedent.
	integrateInstallCmd.Flags().StringVar(&flagAliasName, "alias-name", "",
		"Override the git alias name (default: stagecoach → `git stagecoach`)")
	integrateRemoveCmd.Flags().StringVar(&flagAliasName, "alias-name", "",
		"Override the git alias name to remove (default: stagecoach)")
}

// gitAliasEntry implements integrate.Entry for the git-alias target (PRD §9.21 FR-I4/I6).
// It delegates the .gitconfig WRITE to `git config --global` (so it does NOT use protocol.Apply)
// but owns its preview+confirm via the shared ConfirmFunc. aliasName is the resolved name
// (default "stagecoach").
type gitAliasEntry struct {
	git       git.Git // repo-independent for --global; cwd from os.Getwd() (no-op for global scope)
	aliasName string  // resolved (never "" — defaultEntries resolves "" → "stagecoach")
}

// newGitAliasEntry builds the entry for the current invocation (reads the resolved --alias-name).
func newGitAliasEntry() *gitAliasEntry {
	name := flagAliasName
	if name == "" {
		name = defaultAliasName
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "." // global config ignores cwd; never fatal
	}
	return &gitAliasEntry{git: git.New(cwd), aliasName: name}
}

func (e *gitAliasEntry) Name() string { return gitAliasTarget }

// aliasKey returns "alias.<name>".
func (e *gitAliasEntry) aliasKey() string { return "alias." + e.aliasName }

// isOurs reports whether a stored alias value (incl. its leading `!`) is stagecoach's command.
func isOurs(storedValue string) bool { return strings.TrimPrefix(storedValue, "!") == defaultAliasName }

// Detect — FR-I2: git-alias needs only git. exec.LookPath("git"); nil if present.
func (e *gitAliasEntry) Detect(_ context.Context) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found on PATH: %w", err)
	}
	return nil
}

// ConfigPath — the global gitconfig path (list CONFIG column; display-only, best-effort).
func (e *gitAliasEntry) ConfigPath(_ context.Context) (string, error) {
	if g := os.Getenv("GIT_CONFIG_GLOBAL"); g != "" {
		if abs, err := filepath.Abs(g); err == nil {
			return abs, nil
		}
		return g, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve global gitconfig: %w", err)
	}
	return filepath.Join(home, ".gitconfig"), nil
}

// Status — read back alias.<name>: unset → NotInstalled; ours → Installed; foreign → Foreign.
func (e *gitAliasEntry) Status(ctx context.Context) (integrate.Status, error) {
	v, found, err := e.git.ConfigGlobalGet(ctx, e.aliasKey())
	if err != nil {
		return integrate.StatusNotInstalled, err
	}
	if !found {
		return integrate.StatusNotInstalled, nil
	}
	if isOurs(v) {
		return integrate.StatusInstalled, nil
	}
	return integrate.StatusForeign, nil
}

// Install — FR-I4: show command+usage (+ conflict if foreign), confirm, then
// `git config --global alias.<name> '!stagecoach'`.
func (e *gitAliasEntry) Install(ctx context.Context, opts integrate.InstallOptions) (integrate.InstallResult, error) {
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}
	res := integrate.InstallResult{Outcome: integrate.OutcomeNoChange, Target: e.Name(), Path: e.configPathOr(""), Backup: ""}

	cur, found, err := e.git.ConfigGlobalGet(ctx, e.aliasKey())
	if err != nil {
		return res, fmt.Errorf("read alias %s: %w", e.aliasName, err)
	}

	// Already ours ⇒ idempotent NoChange (do NOT rewrite).
	if found && isOurs(cur) {
		return res, nil
	}

	// FR-I4: surface the WARNING about foreign conflicts to stderr BEFORE the confirm step
	// so it fires in both interactive and --yes modes (mirrors lazygitEntry.Install pattern).
	if found { // foreign (not ours) — warn before overwriting
		fmt.Fprintf(out, "WARNING: %s is currently set to %q (not stagecoach) — it will be overwritten.\n",
			e.aliasKey(), cur)
	}

	// Build the preview (command + usage + conflict note if foreign).
	preview := fmt.Sprintf("Command:  git config --global %s '%s'\nResult:   git %s  →  stagecoach\n",
		e.aliasKey(), stagecoachAliasValue, e.aliasName)
	if found { // foreign (not ours) — include in preview for interactive confirmation
		preview += fmt.Sprintf("\nNOTE: %s is currently set to %q (not stagecoach) — it will be overwritten.\n",
			e.aliasKey(), cur)
	}

	if !opts.Yes {
		confirm := opts.Confirm
		if confirm == nil {
			confirm = integrate.DefaultConfirm // TTY-gated y/N; non-TTY auto-decline
		}
		if !confirm(out, res.Path, preview) {
			res.Outcome = integrate.OutcomeDeclined
			return res, nil // NOTHING written
		}
	}

	if err := e.git.ConfigGlobalSet(ctx, e.aliasKey(), stagecoachAliasValue); err != nil {
		return res, fmt.Errorf("set alias %s: %w", e.aliasName, err)
	}
	if found {
		res.Outcome = integrate.OutcomeUpdated // overwrote a foreign alias
	} else {
		res.Outcome = integrate.OutcomeCreated // newly installed
	}
	return res, nil
}

// Remove — FR-I6: `git config --global --unset alias.<name>` ONLY when the value is ours.
// A foreign alias is left untouched (NoChange + note). An unset alias is NoChange.
func (e *gitAliasEntry) Remove(ctx context.Context, opts integrate.RemoveOptions) (integrate.RemoveResult, error) {
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}
	res := integrate.RemoveResult{Outcome: integrate.OutcomeNoChange, Target: e.Name(), Path: e.configPathOr(""), Backup: ""}

	cur, found, err := e.git.ConfigGlobalGet(ctx, e.aliasKey())
	if err != nil {
		return res, fmt.Errorf("read alias %s: %w", e.aliasName, err)
	}
	if !found {
		return res, nil // nothing to remove — NoChange
	}
	if !isOurs(cur) {
		// FR-I6: NEVER remove a foreign alias. Inform + NoChange.
		fmt.Fprintf(out, "stagecoach: %s is set to %q (not stagecoach); leaving it unchanged.\n", e.aliasKey(), cur)
		return res, nil
	}

	// Ours — preview + confirm the unset (FR-I3c), then unset.
	preview := fmt.Sprintf("Command:  git config --global --unset %s\nResult:   removes `git %s`\n",
		e.aliasKey(), e.aliasName)
	if !opts.Yes {
		confirm := opts.Confirm
		if confirm == nil {
			confirm = integrate.DefaultConfirm
		}
		if !confirm(out, res.Path, preview) {
			res.Outcome = integrate.OutcomeDeclined
			return res, nil
		}
	}
	if _, err := e.git.ConfigGlobalUnset(ctx, e.aliasKey()); err != nil {
		return res, fmt.Errorf("unset alias %s: %w", e.aliasName, err)
	}
	res.Outcome = integrate.OutcomeRemoved
	return res, nil
}

// configPathOr returns ConfigPath or "" on error (for Result.Path; never fatal).
func (e *gitAliasEntry) configPathOr(fallback string) string {
	if p, err := e.ConfigPath(context.Background()); err == nil && p != "" {
		return p
	}
	return fallback
}
