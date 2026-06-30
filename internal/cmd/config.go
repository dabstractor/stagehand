// Package cmd implements the config command group for Stagehand (PRD §9.8 FR38, §15.3, §16.2).
// It provides a `config` cobra command with two leaf subcommands: `init` (write a fully-commented
// example config to the global config path, creating parent dirs, refusing to overwrite) and `path`
// (print the resolved global config path to stdout). Both are thin views over the P1.M1.T4.S2
// globalConfigPath resolver (newly exported as config.GlobalConfigPath()).
//
// Both leaves are in shouldSkipConfigLoad (cmd.Name()=="init"/"path"), so root's PersistentPreRunE
// returns nil immediately — they work OUTSIDE a git repo and never need config.Load.
//
// Registered via init() in this file — ZERO edits to root.go (parallel-safe with S2/S3, design D2).
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/exitcode"
)

// configCmd is the PRD §15.3 "config" command group. It has NO RunE → bare `stagehand config` prints
// help (cobra default). init/path are its leaves (registered in init()). Both leaves are in
// shouldSkipConfigLoad (cmd.Name()=="init"/"path") so root's PersistentPreRunE returns nil immediately
// — they work OUTSIDE a git repo and never need config.Load.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the Stagehand config file",
	Long: `Inspect or bootstrap the Stagehand global config file.

Subcommands:
  init   Write a commented example config to the global config path.
  path   Print the resolved global config path.`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a commented example config to the global path",
	Long: `Write a fully-commented example config to Stagehand's global config path.

The written file documents every available option (defaults, generation tuning, provider overrides)
with all lines commented out, so it changes no behavior until you uncomment the lines you want. Parent
directories are created as needed.

If a config file already exists at the global path, it is NOT overwritten (exit code 1) to protect
your edits. Delete the file first to regenerate it.

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

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configPathCmd)
	rootCmd.AddCommand(configCmd) // register on S1's root — NO edit to root.go (design D2)
}

// runConfigPath implements `stagehand config path` (FR38). Prints the resolved global config path to
// stdout (one line). Returns nil. Never calls os.Exit. Works outside a git repo (config load skipped).
func runConfigPath(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(cmd.OutOrStdout(), config.GlobalConfigPath())
	return nil
}

// runConfigInit implements `stagehand config init` (FR38). Writes the commented exampleConfigTemplate
// to the global config path (creating parent dirs). REFUSES to overwrite an existing file (exit 1,
// non-destructive). Prints a confirmation to stdout on success. Never calls os.Exit.
func runConfigInit(cmd *cobra.Command, args []string) error {
	path := config.GlobalConfigPath()
	if _, err := os.Stat(path); err == nil {
		// File exists — do NOT clobber the user's config.
		return exitcode.New(exitcode.Error, fmt.Errorf("config file already exists at %s (not overwritten)", path))
	} else if !os.IsNotExist(err) {
		// Unable to stat (permissions, etc.) — surface it rather than guessing.
		return exitcode.New(exitcode.Error, fmt.Errorf("check config path %s: %w", path, err))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("create config dir %s: %w", filepath.Dir(path), err))
	}
	if err := os.WriteFile(path, []byte(exampleConfigTemplate), 0o644); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("write config %s: %w", path, err))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Wrote example config to %s\n", path)
	return nil
}

// exampleConfigTemplate is the commented example config written by `config init` (PRD §16.2 / FR38).
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
#
# Git config keys (PRD §9.8 FR36 / §16.3) — alternative to this file, scoped to one repo:
#   git config stagehand.provider pi
#   git config stagehand.model ""
#   git config stagehand.timeout 120s
#   git config stagehand.auto_stage_all true
#   (read via ` + "`git config --get stagehand.<key>`" + `)

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
# output                = "raw"   # agent output mode: "raw" | "json"
# strip_code_fence      = true    # remove ` + "`" + ` fences from agent output

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
`
