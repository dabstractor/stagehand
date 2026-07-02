// defaults_test.go is a WHITE-BOX test (package config, matching the house
// convention used by internal/ui, internal/provider, internal/git, and
// internal/prompt). White-box lets the test reference the unexported-free
// Config type and the Default* symbols directly and assert the exact §16.1
// table. Imports are stdlib "testing" + "time" ONLY — no testify, no os/exec,
// no file I/O: these are pure value tests over [Default].
package config

import (
	"testing"
	"time"
)

// TestDefault_ExactTable asserts that Default() returns the EXACT built-in
// default table mandated by PRD §16.1 bullet 1 + §16.2 subject_target_chars
// (decisions.md §6): every defaulted scalar matches its Default* constant AND
// the literal table value, and every non-defaulted field is at its Go zero
// value. Referencing the Default* constants as well as the literals means a
// typo in either the constant or Default() itself fails this test.
func TestDefault_ExactTable(t *testing.T) {
	c := Default()

	// Defaulted scalars: must equal BOTH the Default* constant and the
	// literal §16.1/§16.2 table value.
	if c.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want DefaultTimeout %v", c.Timeout, DefaultTimeout)
	}
	if c.Timeout != 120*time.Second {
		t.Errorf("Timeout = %v, want 120*time.Second (PRD §16.1)", c.Timeout)
	}
	if c.AutoStageAll != DefaultAutoStageAll {
		t.Errorf("AutoStageAll = %v, want DefaultAutoStageAll %v", c.AutoStageAll, DefaultAutoStageAll)
	}
	if c.AutoStageAll != true {
		t.Errorf("AutoStageAll = %v, want true (PRD §16.1)", c.AutoStageAll)
	}
	if c.MaxDiffBytes != DefaultMaxDiffBytes {
		t.Errorf("MaxDiffBytes = %d, want DefaultMaxDiffBytes %d", c.MaxDiffBytes, DefaultMaxDiffBytes)
	}
	if c.MaxDiffBytes != 300000 {
		t.Errorf("MaxDiffBytes = %d, want 300000 (PRD §16.1)", c.MaxDiffBytes)
	}
	if c.MaxMdLines != DefaultMaxMdLines {
		t.Errorf("MaxMdLines = %d, want DefaultMaxMdLines %d", c.MaxMdLines, DefaultMaxMdLines)
	}
	if c.MaxMdLines != 100 {
		t.Errorf("MaxMdLines = %d, want 100 (PRD §16.1)", c.MaxMdLines)
	}
	if c.MaxDuplicateRetries != DefaultMaxDuplicateRetries {
		t.Errorf("MaxDuplicateRetries = %d, want DefaultMaxDuplicateRetries %d", c.MaxDuplicateRetries, DefaultMaxDuplicateRetries)
	}
	if c.MaxDuplicateRetries != 3 {
		t.Errorf("MaxDuplicateRetries = %d, want 3 (PRD §16.1)", c.MaxDuplicateRetries)
	}
	if c.SubjectTargetChars != DefaultSubjectTargetChars {
		t.Errorf("SubjectTargetChars = %d, want DefaultSubjectTargetChars %d", c.SubjectTargetChars, DefaultSubjectTargetChars)
	}
	if c.SubjectTargetChars != 50 {
		t.Errorf("SubjectTargetChars = %d, want 50 (PRD §16.2)", c.SubjectTargetChars)
	}
	if c.Output != DefaultOutput {
		t.Errorf("Output = %q, want DefaultOutput %q", c.Output, DefaultOutput)
	}
	// DefaultOutput == "raw" (the provider.OutputRaw enum value).
	if c.Output != "raw" {
		t.Errorf("Output = %q, want \"raw\" (provider.OutputRaw, PRD §16.1)", c.Output)
	}
	if c.StripCodeFence != DefaultStripCodeFence {
		t.Errorf("StripCodeFence = %v, want DefaultStripCodeFence %v", c.StripCodeFence, DefaultStripCodeFence)
	}
	if c.StripCodeFence != true {
		t.Errorf("StripCodeFence = %v, want true (PRD §16.1)", c.StripCodeFence)
	}

	// Non-defaulted fields: must be at their Go zero values (the §16.1
	// contract; Provider/Model/ConfigPath resolve later via higher precedence
	// or the registry).
	if c.Provider != "" {
		t.Errorf("Provider = %q, want \"\" (zero value)", c.Provider)
	}
	if c.Model != "" {
		t.Errorf("Model = %q, want \"\" (zero value)", c.Model)
	}
	if c.Verbose != false {
		t.Errorf("Verbose = %v, want false (zero value)", c.Verbose)
	}
	if c.NoColor != false {
		t.Errorf("NoColor = %v, want false (zero value)", c.NoColor)
	}
	if c.ConfigPath != "" {
		t.Errorf("ConfigPath = %q, want \"\" (zero value)", c.ConfigPath)
	}
}

// TestDefault_ProviderOverridesEmpty asserts that Default() does NOT bake the
// six built-in manifests into Config: ProviderOverrides is left nil (len==0).
// The built-ins are injected separately by provider.NewRegistry at Load() time
// (P1.M5.T3.S1); baking them here would double-layer them.
func TestDefault_ProviderOverridesEmpty(t *testing.T) {
	c := Default()
	if got := len(c.ProviderOverrides); got != 0 {
		t.Errorf("len(ProviderOverrides) = %d, want 0 (built-ins injected by the registry, not baked into Config)", got)
	}
}
