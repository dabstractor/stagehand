// Package cmd implements the config command group for Stagehand (PRD §9.8 FR38, §15.3, §16.2).
// It provides a `config` cobra command with three leaf subcommands: `init` (bootstrap a populated
// working config to the global config path, creating parent dirs, refusing to overwrite unless
// --force), `path` (print the resolved global config path to stdout), and `upgrade` (rewrite an
// existing config in place to the current schema version via a minimal textual transform).
//
// All three leaves are in shouldSkipConfigLoad (cmd.Name()=="init"/"path"/"upgrade"), so root's
// PersistentPreRunE returns nil immediately — they work OUTSIDE a git repo and never need config.Load.
//
// Registered via init() in this file — ZERO edits to root.go (parallel-safe with S2/S3, design D2).
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/exitcode"
	"github.com/dustin/stagehand/internal/provider"
)

// preferredBuiltins is the FR-D1 cascading provider priority order (local copy — mirrors
// internal/provider/registry.go's unexported preferredBuiltins). Used by runConfigInit
// for the --provider validation error message. (config/bootstrap.go has its own copy for
// stagerFallback + commented-block ordering — pre-existing mirror pattern.)
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "codex", "claude"}

// configCmd is the PRD §15.3 "config" command group. It has NO RunE → bare `stagehand config` prints
// help (cobra default). init/path are its leaves (registered in init()). Both leaves are in
// shouldSkipConfigLoad (cmd.Name()=="init"/"path") so root's PersistentPreRunE returns nil immediately
// — they work OUTSIDE a git repo and never need config.Load.
var configCmd = &cobra.Command{
	Use:           "config",
	Short:         "Manage the Stagehand config file",
	Long:          `Inspect, bootstrap, or upgrade the Stagehand global config file.`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap a working config (auto-detects your agent)",
	Long: `Bootstrap a populated, working config to Stagehand's global config path.

By DEFAULT, detects the highest-priority installed built-in agent (order: pi, opencode,
cursor, agy, gemini, codex, claude) and writes a config with that provider's per-role
default models UNCOMMENTED so the tool works immediately. If no agent is detected, defaults
to "pi". Other installed providers appear as commented-out [role.*] blocks (one-line
uncomment to route a role to a different agent).

Flags:
  --provider <name>  Target a specific built-in provider instead of auto-detecting.
  --force            Overwrite an existing config file.
  --template         Write the inert all-commented reference config (v1 behavior).

Parent directories are created as needed. If a config file already exists, it is NOT
overwritten unless --force is passed (exit code 1).

See ` + "`stagehand config path`" + ` for the target location.`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runConfigInit,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the resolved global config path",
	Long: `Print the resolved global config path (the file ` + "`config init`" + ` writes and Stagehand
reads as its global config layer).

This is the DISCOVERED global location ($XDG_CONFIG_HOME/stagehand/config.toml, or
~/.config/stagehand/config.toml by default) — not a --config/STAGEHAND_CONFIG override, which selects
a separate read path.`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runConfigPath,
}

// configUpgradeCmd implements `stagehand config upgrade` (PRD §9.17 FR-B5). Rewrites an EXISTING
// global config in place so its top-level config_version equals CurrentConfigVersion, via a minimal
// TEXTUAL edit that preserves every other line. Idempotent. Works outside a git repo (shouldSkipConfigLoad).
var configUpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade an existing config to the current schema version",
	Long: `Rewrite an existing Stagehand config file in place so its config_version matches this binary's
current schema version (` + fmt.Sprintf("`config_version = %d`", config.CurrentConfigVersion) + `).

Only the top-level config_version line is added or updated — every other line (your values, comments,
ordering) is preserved byte-for-byte. Running it twice is safe: a file already at the current version is
left unchanged ("already up to date").

This is the remediation the load-time advisory points at when a config has no config_version or an older
one. It targets the GLOBAL config (the path printed by ` + "`stagehand config path`" + `).

If no config file exists, run ` + "`stagehand config init`" + ` first. If the file is not valid TOML, it is
left untouched and an error is printed.`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runConfigUpgrade,
}

// configVersionLineRe matches an UNCOMMENTED top-level config_version assignment, capturing the integer
// value. Anchored at column 0 (a leading '#' is not matched) — commented `# config_version = 2` is ignored.
var configVersionLineRe = regexp.MustCompile(`^config_version\s*=\s*([0-9]+)`)

func init() {
	configInitCmd.Flags().String("provider", "", "Target a specific provider instead of auto-detecting")
	configInitCmd.Flags().Bool("force", false, "Overwrite an existing config file")
	configInitCmd.Flags().Bool("template", false, "Write the inert all-commented reference config (v1 behavior)")

	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configUpgradeCmd)
	rootCmd.AddCommand(configCmd) // register on S1's root — NO edit to root.go (design D2)
}

// runConfigPath implements `stagehand config path` (FR38). Prints the resolved global config path to
// stdout (one line). Returns nil. Never calls os.Exit. Works outside a git repo (config load skipped).
func runConfigPath(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(cmd.OutOrStdout(), config.GlobalConfigPath())
	return nil
}

// runConfigUpgrade reads the global config, validates it is parseable TOML, ensures the top-level
// config_version equals CurrentConfigVersion (minimal textual edit), writes it back, and prints a
// confirmation. Never calls os.Exit; routes errors via exitcode.New. (PRD §9.17 FR-B5.)
func runConfigUpgrade(cmd *cobra.Command, args []string) error {
	path := config.GlobalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return exitcode.New(exitcode.Error, fmt.Errorf("no config file at %s (run 'stagehand config init' first)", path))
		}
		return exitcode.New(exitcode.Error, fmt.Errorf("read config %s: %w", path, err))
	}
	// Validity gate: refuse to mangle an unparseable file. Non-strict (map[string]any) — a merely-
	// incomplete config (e.g. only [defaults]) is fine; only genuine syntax errors fail.
	var probe map[string]any
	if err := toml.Unmarshal(data, &probe); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("config %s is not valid TOML: %w", path, err))
	}
	newContent, changed := upgradeConfigVersion(string(data), config.CurrentConfigVersion)
	if !changed {
		fmt.Fprintf(cmd.OutOrStdout(), "Config at %s is already at version %d (no changes).\n", path, config.CurrentConfigVersion)
		return nil
	}
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("write config %s: %w", path, err))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Upgraded config at %s to version %d.\n", path, config.CurrentConfigVersion)
	return nil
}

// upgradeConfigVersion returns content with the TOP-LEVEL config_version set to version, via a minimal
// TEXTUAL edit that preserves every other line byte-for-byte (PRD §9.17 FR-B5: "preserving user values …
// leave all other content unchanged"). It scans only the top-level region (before the first [table] header —
// config_version is root metadata). Outcomes (D4):
//   - found with value == version  → content unchanged, changed=false (the "already up to date" path)
//   - found with value != version  → that ONE line rewritten, changed=true
//   - not found                    → one `config_version = <version>` line inserted after the leading
//     comment/blank header block, changed=true
//
// PURE (no I/O, no error) → fully unit-testable. v2.0 has no removed/renamed keys, so no other line is
// touched; this function is the single future extension point (add a version-keyed migration for v3+).
func upgradeConfigVersion(content string, version int) (string, bool) {
	lines := strings.Split(content, "\n")
	want := strconv.Itoa(version)

	// 1. Scan the top-level region for an existing config_version (stop at the first [table] header).
	for i, line := range lines {
		if isTableHeader(line) {
			break // config_version must precede tables; nothing top-level after this is the schema key
		}
		if m := configVersionLineRe.FindStringSubmatch(line); m != nil {
			if strings.TrimSpace(m[1]) == want {
				return content, false // already current — byte-identical
			}
			lines[i] = "config_version = " + want
			return strings.Join(lines, "\n"), true
		}
	}

	// 2. No top-level config_version — insert one after the leading comment/blank header block.
	insertAt := leadingHeaderEnd(lines)
	ins := append([]string{}, lines[:insertAt]...)
	ins = append(ins, "config_version = "+want)
	ins = append(ins, lines[insertAt:]...)
	return strings.Join(ins, "\n"), true
}

// isTableHeader reports whether line is a TOML [table] / [[array-of-tables]] header (non-comment, col 0).
func isTableHeader(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") {
		return false
	}
	return strings.HasPrefix(t, "[")
}

// leadingHeaderEnd returns the index of the first line that is NOT a comment and NOT blank — i.e. the end
// of the leading comment/blank header block. Used as the insertion point for a new top-level config_version
// (so it sits with the other root keys, before the first table). Returns len(lines) if the whole file is
// comments/blanks.
func leadingHeaderEnd(lines []string) int {
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		return i
	}
	return len(lines)
}

// runConfigInit implements `stagehand config init` (PRD §9.17 FR-B1/B2). Bootstraps a populated
// working config by default (auto-detects provider + per-role models from the FR-D4 table), or writes
// the inert exampleConfigTemplate when --template is passed. Refuses to overwrite unless --force.
// Parent dirs are created; the written path is always printed. Never calls os.Exit.
// The populated-config generation is delegated to config.GenerateBootstrapConfig (P1.M4.T4.S1).
func runConfigInit(cmd *cobra.Command, args []string) error {
	path := config.GlobalConfigPath()

	force, _ := cmd.Flags().GetBool("force")
	if !force {
		if _, err := os.Stat(path); err == nil {
			return exitcode.New(exitcode.Error, fmt.Errorf("config file already exists at %s (not overwritten)", path))
		} else if !os.IsNotExist(err) {
			return exitcode.New(exitcode.Error, fmt.Errorf("check config path %s: %w", path, err))
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("create config dir %s: %w", filepath.Dir(path), err))
	}

	tmpl, _ := cmd.Flags().GetBool("template")
	var content string
	if tmpl {
		content = exampleConfigTemplate
	} else {
		providerName, _ := cmd.Flags().GetString("provider")
		if providerName != "" {
			reg := provider.NewRegistry(nil)
			if _, ok := reg.Get(providerName); !ok {
				return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q (use a built-in: %s)",
					providerName, strings.Join(preferredBuiltins, ", ")))
			}
		}
		content = config.GenerateBootstrapConfig(providerName)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("write config %s: %w", path, err))
	}

	if tmpl {
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote example config to %s\n", path)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote config to %s\n", path)
	}
	return nil
}

// exampleConfigTemplate is the commented example config written by `config init --template` (PRD §16.2 / FR38).
// EVERY option line is commented out (#), so the file is INERT until the user uncomments it. This
// template IS the Mode-A user-facing config documentation: the header explains the §9.8 precedence
// order, STAGEHAND_* env vars, and `stagehand.*` git-config keys; the [defaults]/[generation]/
// [provider.X] sections mirror §16.2 with documented default values and (for providers) field names
// that match internal/provider/manifest.go toml tags.
const exampleConfigTemplate = `# Stagehand configuration file (PRD §16.2).
#
# Generated by ` + "`stagehand config init`" + `. Every option below is COMMENTED OUT (#), so this file
# is inert — it documents the available options without changing any defaults. To use an option,
# copy its line to a new (uncommented) line and adjust the value.
#
# Resolution precedence (highest -> lowest), PRD §9.8 FR34 / §16.1:
#   CLI flags  >  STAGEHAND_* env vars  >  repo git config (stagehand.*)  >
#   repo-local .stagehand.toml  >  THIS global file  >  provider defaults  >  built-in defaults
#
# This is the GLOBAL file. A repo-local file (./.stagehand.toml) and repo git config (stagehand.*)
# both override it; CLI flags and env vars override those.
#
# Environment variables (PRD §9.8 FR35) — override this file, are overridden by CLI flags:
#   STAGEHAND_PROVIDER   default provider/agent (e.g. "pi", "claude", "gemini")
#   STAGEHAND_MODEL      model override ("" -> provider manifest default_model)
#   STAGEHAND_TIMEOUT    generation timeout, e.g. "120s" or 120 (seconds)
#   STAGEHAND_CONFIG     path to a config file, overrides discovery
#   STAGEHAND_VERBOSE    "true"/"false" — print resolved command, raw output, retries
#   STAGEHAND_NO_COLOR   "true"/"false" — disable color (also honors NO_COLOR)
#   STAGEHAND_PLANNER_PROVIDER / _MODEL   per-role override: decomposition planner (PRD §16.4, §9.15)
#   STAGEHAND_STAGER_PROVIDER  / _MODEL   per-role override: (tooled) staging agent
#   STAGEHAND_MESSAGE_PROVIDER / _MODEL   per-role override: bare commit-message agent
#   STAGEHAND_ARBITER_PROVIDER / _MODEL   per-role override: leftover arbiter
#   STAGEHAND_COMMITS                    force exactly N commits when nothing is staged (PRD §9.14); 1 == --single
#
# ---------------------------------------------------------------------------
# config_version — schema version (PRD §9.17 FR-B4). Top-level metadata, NOT a [defaults] key and
# NOT a precedence layer (§16.1): it never overrides another field; it only tells stagehand which
# schema the file was written for. This binary supports config_version = 2.
# ---------------------------------------------------------------------------
# config_version = 2
#
# On load, if this is missing/older than the binary's version, stagehand prints an advisory and
# points you at the remediation; it NEVER auto-migrates your file (no behavior change, just a
# warning on stderr):
#   stagehand config upgrade      # rewrite this file in place to the current schema (P1.M4.T3)
#   stagehand config init --force # regenerate the bootstrap config, overwriting this file

# Git config keys (PRD §9.8 FR36 / §16.3) — alternative to this file, scoped to one repo:
#   git config stagehand.provider pi
#   git config stagehand.model ""
#   git config stagehand.timeout 120s
#   git config stagehand.auto_stage_all true
#   (read via ` + "`git config --get stagehand.<key>`" + `)

# ---------------------------------------------------------------------------
# CLI flags (PRD §15.2) — highest precedence; only an EXPLICITLY-passed flag overrides lower layers
# ---------------------------------------------------------------------------
# --provider / --model                       global default for ALL roles (§16.4)
# --<role>-provider / --<role>-model         per-role override (role = planner|stager|message|arbiter)
# --commits <N>                              force exactly N commits (N>=2); --commits 1 == --single (§9.14)
# --single / --no-decompose                  bypass decomposition; force the single-commit path (§9.14)
# --max-commits <N>                          safety cap on auto-decompose (default 12; §9.14 FR-M4)

# ---------------------------------------------------------------------------
# [defaults] — top-level Stagehand behavior (PRD §16.2)
# ---------------------------------------------------------------------------
# [defaults]
# provider       = "pi"     # default agent; "" -> auto-detect (first installed built-in)
# model          = ""       # "" -> use the provider manifest's default_model
# timeout        = "120s"   # generation timeout (Go duration string, e.g. "2m", or bare seconds)
# auto_stage_all = true     # run ` + "`git add -A`" + ` when nothing is staged
# verbose        = false    # print the resolved command, raw agent output, and retries

# ---------------------------------------------------------------------------
# [generation] — diff capture & output tuning (PRD §16.2)
# ---------------------------------------------------------------------------
# [generation]
# max_diff_bytes        = 300000  # byte cap on the non-markdown diff section
# max_md_lines          = 100     # per-file line cap for markdown diffs
# max_duplicate_retries = 3       # re-generation attempts when the subject duplicates a recent commit
# subject_target_chars  = 50      # target subject-line length for truncation
# output                = "raw"   # agent output mode: "raw" | "json" — applies to parsing across ALL providers
# strip_code_fence      = true    # strip ` + "`" + ` fences from agent output (all providers)
# max_commits           = 12      # safety cap on auto-decompose (PRD §9.14 FR-M4); default 12
# binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
# NOTE: [generation] output/strip_code_fence override any per-provider [provider.<name>] values.

# ---------------------------------------------------------------------------
# [provider.<name>] — override a built-in or define a new provider (PRD §16.2, §12.8)
# ---------------------------------------------------------------------------
# A [provider.<name>] section FIELD-MERGES onto a built-in of the same name. A brand-new <name>
# adds a new provider. Use ` + "`stagehand providers show <name>`" + ` to inspect the merged result.
#
# Override a built-in (e.g. pin pi to a different model/provider):
# [provider.pi]
# default_model    = "glm-5.2"
# default_provider = "zai"
#
# Define a brand-new provider (PRD §12.8):
# [provider.myagent]
# command            = "/opt/myagent/bin/agent"
# prompt_delivery    = "stdin"          # stdin | positional | flag
# print_flag         = "--once"
# model_flag         = "--model"
# default_model      = "my-model-7b"
# system_prompt_flag = "--system"
# default_provider   = "zai"
# bare_flags         = ["--no-mcp", "--ephemeral"]
# output             = "raw"            # raw | json

# ---------------------------------------------------------------------------
# [role.<role>] — per-role provider/model overrides (PRD §16.4, §9.15 FR-R1–R5)
# ---------------------------------------------------------------------------
# The four agent roles — planner, stager, message, arbiter — each resolve their provider/model
# independently. A single [defaults] (above) covers ALL roles; a [role.*] table overrides it for the
# roles you care about. Both fields "" -> inherit [defaults]. Precedence (highest wins):
#   flag > STAGEHAND_<ROLE>_* env > [role.*] config > [defaults] > provider manifest default.
#
# [role.planner]
# provider = "agy"
# model    = "gemini-2.5-pro"
#
# [role.stager]            # tooled agent that runs git; needs tooled_flags in its provider manifest
# provider = "agy"
# model    = "gemini-2.5-flash"
#
# [role.message]           # bare commit-message agent — inherits [defaults] (omit to inherit)
# provider = ""            # "" -> inherit [defaults].provider
# model    = ""            # "" -> inherit [defaults].model
#
# [role.arbiter]           # bare leftover arbiter — inherits [defaults]
# provider = ""
# model    = ""
`
