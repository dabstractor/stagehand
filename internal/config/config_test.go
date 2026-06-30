package config

import (
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

func TestDefaults(t *testing.T) {
	c := Defaults()
	// [defaults] (PRD §16.1 does not pin provider/model => "")
	if c.Provider != "" {
		t.Errorf("Provider = %q, want %q", c.Provider, "")
	}
	if c.Model != "" {
		t.Errorf("Model = %q, want %q", c.Model, "")
	}
	if c.Timeout != 120*time.Second {
		t.Errorf("Timeout = %v, want 120s", c.Timeout)
	}
	if !c.AutoStageAll {
		t.Errorf("AutoStageAll = false, want true")
	}
	if c.Verbose {
		t.Errorf("Verbose = true, want false")
	}
	// CLI/UI-only
	if c.NoColor {
		t.Errorf("NoColor = true, want false")
	}
	// [generation] (PRD §16.1 + subject_target_chars=50 from §16.2)
	if c.MaxDiffBytes != 300000 {
		t.Errorf("MaxDiffBytes = %d, want 300000", c.MaxDiffBytes)
	}
	if c.MaxMdLines != 100 {
		t.Errorf("MaxMdLines = %d, want 100", c.MaxMdLines)
	}
	if c.MaxDuplicateRetries != 3 {
		t.Errorf("MaxDuplicateRetries = %d, want 3", c.MaxDuplicateRetries)
	}
	if c.SubjectTargetChars != 50 {
		t.Errorf("SubjectTargetChars = %d, want 50", c.SubjectTargetChars)
	}
	if c.Output != nil {
		t.Errorf("Output = %v, want nil", c.Output)
	}
	if c.StripCodeFence != nil {
		t.Errorf("StripCodeFence = %v, want nil", c.StripCodeFence)
	}
}

func TestTOMLMarshalKeysAndNoColorExclusion(t *testing.T) {
	c := Defaults()
	c.Output = strPtr("raw")
	c.StripCodeFence = boolPtr(true)
	data, err := toml.Marshal(c)
	if err != nil {
		t.Fatalf("toml.Marshal(explicit values) err = %v", err)
	}
	s := string(data)
	for _, key := range []string{
		"provider", "model", "timeout", "auto_stage_all", "verbose",
		"max_diff_bytes", "max_md_lines", "max_duplicate_retries",
		"subject_target_chars", "output", "strip_code_fence",
	} {
		if !strings.Contains(s, key+" =") {
			t.Errorf("marshaled TOML missing key %q:\n%s", key, s)
		}
	}
	// NoColor is toml:"-" and must NEVER appear in a config file (PRD §15.2: flag/env only).
	nc := Defaults()
	nc.NoColor = true
	data2, err := toml.Marshal(nc)
	if err != nil {
		t.Fatalf("toml.Marshal(NoColor=true) err = %v", err)
	}
	if strings.Contains(string(data2), "no_color") || strings.Contains(string(data2), "NoColor") {
		t.Errorf("NoColor leaked into TOML (toml:\"-\" not honored):\n%s", data2)
	}
}
