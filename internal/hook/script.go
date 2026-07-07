// Package hook holds the primitives for stagehand's git hook mode (PRD §9.20): the
// prepare-commit-msg script template stagehand installs, plus the constants that identify and
// permission it. P1.M3.T1.S2 builds the `hook install|uninstall|status` commands on top of these.
package hook

import (
	"os"
	"strings"
)

// Marker is the identity line stagehand writes as the SECOND line of its prepare-commit-msg hook (after the
// shebang). Its presence is how `hook status`/`hook uninstall` (P1.M3.T1.S2) recognize a stagehand-owned
// hook (marker present → ours, rewrite/remove; absent → foreign, refuse — PRD §9.20 FR-H2/FR-H3).
const Marker = "# stagecoach prepare-commit-msg hook v1"

// ScriptMode is the file mode stagehand writes the hook with (executable — PRD §9.20 FR-H1).
const ScriptMode os.FileMode = 0o755

// hookScript returns the exact bytes of the prepare-commit-msg hook stagehand installs (PRD §9.20 FR-H1).
// It is strict POSIX sh (no bashisms) so it runs under git-for-windows' sh (Appendix E #15). When strict is
// true the runtime call gets `--strict` (PRD §9.20 FR-H5: failures then abort the commit).
//
// When configPath is non-empty, an `export STAGEHAND_CONFIG=<path>` line is baked in BEFORE the exec
// line so that `hook exec` at commit time resolves the SAME config the user explicitly selected at
// `hook install --config <path>` time (report Finding 4). Without this, `--config` passed to
// `hook install` was silently ignored — the installed script invoked `stagehand hook exec` with no
// config hint, so config.Load fell back to env/discovery and could resolve a DIFFERENT config (or a
// different provider) than the one active at install time. config.Load honors STAGEHAND_CONFIG as the
// layer between --config and discovery (internal/config/file.go ResolveConfigPath), so the env export
// is the faithful bridge. The trailing newline keeps the file POSIX-clean.
//
// configPath is single-shell-word-quoted so paths with spaces/special chars survive. Empty configPath
// omits the export line entirely (the default, no-op case).
func hookScript(strict bool, configPath string) string {
	run := `exec stagehand hook exec "$@"`
	if strict {
		run = `exec stagehand hook exec --strict "$@"`
	}
	var export string
	if configPath != "" {
		export = "export STAGEHAND_CONFIG=" + shellSingleQuote(configPath) + "\n"
	}
	return "#!/bin/sh\n" + Marker + "\n" + export + run + "\n"
}

// shellSingleQuote renders s as a single shell word that survives POSIX sh, including paths with
// spaces, single quotes, or other shell metacharacters. Uses the canonical POSIX idiom: wrap the
// string in single quotes and replace each embedded single quote with the sequence '\'' (close
// quote, an escaped quote, reopen quote). Pure; no I/O.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
