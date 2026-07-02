// This file holds the built-in default table (PRD §16.1; decisions.md §6) as
// exported constants plus the [Default] constructor that materializes a Config
// from them. It is intentionally a sibling of config.go — config.go OWNS the
// "// Package config" doc; this file uses a plain "package config" line,
// mirroring how internal/git/log.go defers the package doc to git.go.
package config

import (
	"time"

	"github.com/dustin/stagehand/internal/provider"
)

// Built-in default values centralized here as exported constants (PRD §16.1
// bullet 1; decisions.md §6). They are referenced by [Default] and by the CLI
// flag defaults (P1.M7.T2); reference them by symbol, never re-hardcode the
// literals. There is intentionally NO DefaultProvider/DefaultModel constant —
// those resolve from each manifest's DefaultProvider/DefaultModel via the
// registry, so their config zero value "" IS the default.
const (
	// DefaultTimeout is the built-in per-agent-invocation timeout, 120 seconds
	// (PRD §16.1 timeout 120s; decisions.md §6). Expressed as a time.Duration,
	// NOT the "120s" TOML string (string→Duration conversion is the loader's
	// concern, P1.M5.T2.S1).
	DefaultTimeout = 120 * time.Second

	// DefaultAutoStageAll is the built-in default for whether the CLI auto-runs
	// "git add -A" before generating (PRD §16.1 auto_stage_all true; decisions
	// §6).
	DefaultAutoStageAll = true

	// DefaultMaxDiffBytes is the built-in total staged-diff byte cap (PRD
	// §16.1 max_diff_bytes 300000; decisions §6).
	DefaultMaxDiffBytes = 300000

	// DefaultMaxMdLines is the built-in per-markdown-file line cap within the
	// diff (PRD §16.1 max_md_lines 100; decisions §6).
	DefaultMaxMdLines = 100

	// DefaultMaxDuplicateRetries is the built-in outer duplicate-rejection
	// loop budget (PRD §16.1 max_duplicate_retries 3; decisions §6; §3: the
	// loop runs 0..N inclusive).
	DefaultMaxDuplicateRetries = 3

	// DefaultSubjectTargetChars is the built-in target subject-line character
	// count (PRD §16.2 subject_target_chars = 50; FR13/FR14; decisions §6).
	// §16.1 bullet 1 omits this field, but §16.2 and the work-item contract
	// mandate 50 in Default.
	DefaultSubjectTargetChars = 50

	// DefaultOutput is the built-in agent-output mode: the provider.OutputRaw
	// enum constant (== "raw"), NOT a magic string (PRD §16.1 output raw;
	// decisions §6). Tying the config default to the provider enum keeps a
	// single source of truth for "raw".
	DefaultOutput = provider.OutputRaw

	// DefaultStripCodeFence is the built-in default for stripping one ```/~~~
	// fence layer from agent stdout (PRD §16.1 strip_code_fence true; decisions
	// §6).
	DefaultStripCodeFence = true
)

// Default returns the built-in default Config — the lowest-precedence floor of
// the FR34 resolution chain, populated from the Default* constants above (PRD
// §16.1 bullet 1; decisions.md §6). The remaining fields (Provider, Model,
// Verbose, NoColor, ConfigPath) take their Go zero values, and ProviderOverrides
// is left nil (intentionally NOT an empty map): len(nil)==0 is the correct "no
// overrides" state, and the six built-in manifests are applied separately by
// provider.NewRegistry at Load() time (P1.M5.T3.S1), so they are NOT baked
// into Config here.
func Default() Config {
	return Config{
		Timeout:             DefaultTimeout,
		AutoStageAll:        DefaultAutoStageAll,
		MaxDiffBytes:        DefaultMaxDiffBytes,
		MaxMdLines:          DefaultMaxMdLines,
		MaxDuplicateRetries: DefaultMaxDuplicateRetries,
		SubjectTargetChars:  DefaultSubjectTargetChars,
		Output:              DefaultOutput,
		StripCodeFence:      DefaultStripCodeFence,
	}
}
