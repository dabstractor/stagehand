// Package config holds stagehand's resolved, in-memory settings type — the
// single Config value threaded through the whole binary. Every package reads
// its resolved settings off a Config: the generate step (P1.M6.T1.S1) reads
// cfg.Timeout, cfg.MaxDuplicateRetries, cfg.MaxDiffBytes, cfg.MaxMdLines,
// cfg.Output, cfg.StripCodeFence; the prompt layer (P1.M4/M6 callers) reads
// cfg.SubjectTargetChars; the CLI (P1.M7.T2) reads cfg.Provider, cfg.Model,
// cfg.AutoStageAll, cfg.Verbose, cfg.NoColor, cfg.ConfigPath. The precedence
// chain (FR34) is resolved by Load() (P1.M5.T3.S1), which starts from
// [Default] — the lowest-precedence floor — and layers config-file
// (.stagehand.toml), git-config, env, and flag values on top.
//
// Config is deliberately the RESOLVED, in-memory type: it carries no "toml:"
// struct tags. PRD §16.2 splits the scalars across TWO TOML tables ([defaults]
// and [generation]), so a flat Config CANNOT be unmarshaled directly from a
// .stagehand.toml file. The tagged DTOs (defaultsDTO/generationDTO) and the
// DTO→Config assembly belong to the loader task (P1.M5.T2.S1); this package
// keeps the type definition clean and decoupled from go-toml. Likewise the
// "120s" string in §16.2 is a TOML string; Config.Timeout is a time.Duration
// and the string→Duration conversion is a loader concern, not Config's.
//
// The ProviderOverrides map carries the user's per-provider manifest
// overrides (PRD §16.1 bullet 7; decisions.md §6). The six built-in manifests
// are NOT baked into Config: they are injected by provider.NewRegistry at
// Load() time (P1.M5.T3.S1) as provider.NewRegistry(provider.Builtins(),
// cfg.ProviderOverrides), which merges overrides field-by-field onto the
// built-ins. Default() therefore leaves ProviderOverrides nil. This keeps the
// one-way import edge config → provider with no cycle (plan_overview.md key
// decision 1: the Manifest type lives in provider M2; config M5 imports it;
// provider/registry does NOT import config).
//
// This file is the first/primary file of package config and OWNS the package
// doc; sibling files (defaults.go, and the white-box tests) use a plain
// "package config" line, mirroring how internal/git/git.go owns the
// "// Package git" doc while internal/git/log.go uses a plain "package git".
package config

import (
	"time"

	"github.com/dustin/stagehand/internal/provider"
)

// Config is the resolved, in-memory representation of every stagehand
// setting (PRD §16; decisions.md §2 + §6). It is produced by Load()
// (P1.M5.T3.S1) and consumed read-only by every downstream package. It has
// NO toml struct tags: it is the resolved type, not a directly-unmarshaled
// one (PRD §16.2 splits scalars across [defaults]/[generation] tables; the
// tagged DTOs live in the loader, P1.M5.T2.S1). Config never deep-copies its
// slice/map fields — the registry deep-copies ProviderOverrides at
// construction, and no other Config field holds reference state.
type Config struct {
	// Provider is the default agent name (e.g. "pi"). The empty string means
	// "not set by any higher-precedence source"; resolution of the actual
	// default provider is the registry/executor's job, not Config's (PRD
	// §16.2 [defaults] provider). Zero value "" is the correct config default.
	Provider string

	// Model is the model override; the empty string means "use the resolved
	// manifest's DefaultModel" (PRD §16.2 [defaults] model). Zero value "" is
	// the correct config default.
	Model string

	// Timeout is the per-agent-invocation timeout (PRD §16.1/§16.2 timeout
	// "120s"). It is a time.Duration in memory; the "120s" TOML
	// string→Duration conversion is the loader's concern (P1.M5.T2.S1).
	Timeout time.Duration

	// AutoStageAll controls whether the CLI auto-runs "git add -A" before
	// generating (PRD §16.1/§16.2 auto_stage_all; v1 main wires maybeAutoStage
	// + CommitStaged, decisions.md §1).
	AutoStageAll bool

	// Verbose enables verbose logging (PRD §16.2 [defaults] verbose).
	Verbose bool

	// NoColor disables color output (PRD §16.2 [defaults] no_color). Consumed
	// by internal/ui.
	NoColor bool

	// MaxDiffBytes is the total staged-diff byte cap (PRD §16.1/§16.2
	// max_diff_bytes = 300000). Consumed by internal/git/diff.go.
	MaxDiffBytes int

	// MaxMdLines is the per-markdown-file line cap applied within the diff
	// (PRD §16.1/§16.2 max_md_lines = 100). Consumed by internal/git/diff.go.
	MaxMdLines int

	// MaxDuplicateRetries is the outer duplicate-rejection loop budget (PRD
	// §16.1 max_duplicate_retries = 3; decisions.md §3: the outer loop runs
	// 0..N inclusive). Consumed by the generate step (P1.M6.T1.S1).
	MaxDuplicateRetries int

	// SubjectTargetChars is the target subject-line character count (PRD
	// §16.2 subject_target_chars = 50; FR13/FR14 ~50 chars). Consumed by
	// prompt.BuildSystemPrompt. (§16.1 bullet 1 omits this field but §16.2
	// and the contract mandate 50 in Default.)
	SubjectTargetChars int

	// Output selects how agent stdout is interpreted: "raw" (default) or
	// "json" — the provider.OutputRaw/provider.OutputJSON enum values (PRD
	// §16.1/§16.2 output). Consumed by the provider parse pipeline.
	Output string

	// StripCodeFence removes one ```/~~~ fence layer from agent stdout when
	// true (PRD §16.1/§16.2 strip_code_fence; provider parse pipeline).
	StripCodeFence bool

	// ConfigPath is the resolved config-file path actually loaded, set by
	// Load() (P1.M5.T3.S1) for diagnostics. It is "" when only defaults were
	// applied (PRD §16.2). Zero value "" is the correct default.
	ConfigPath string

	// ProviderOverrides holds the user's per-provider manifest overrides keyed
	// by provider name (PRD §16.1 bullet 7; decisions.md §6). nil means "no
	// overrides" (the Default). The six built-in manifests are NOT baked into
	// Config; Load() merges them field-by-field via
	// provider.NewRegistry(provider.Builtins(), cfg.ProviderOverrides). Config
	// never clones this map — the registry deep-copies at construction.
	ProviderOverrides map[string]provider.Manifest
}
