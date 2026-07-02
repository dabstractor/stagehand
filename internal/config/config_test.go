// config_test.go is a WHITE-BOX test (package config, matching the house
// convention). It imports internal/provider so it can construct
// provider.Manifest values to populate ProviderOverrides. No testify, no
// os/exec, no file I/O — pure value round-trip tests.
package config

import (
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/provider"
)

// TestConfig_RoundTrip_Populated builds a Config with EVERY scalar set to a
// distinct non-default value and a ProviderOverrides map with two entries,
// then asserts each field reads back exactly — including the override
// contents. This exercises the "round-trip a populated Config" scenario that
// the downstream consumers (Load P1.M5.T3.S1, generate P1.M6.T1.S1, CLI
// P1.M7.T2) depend on.
//
// Go map/slice reference semantics note: a value copy `c2 := cfg` preserves
// ALL fields, including the aliased ProviderOverrides map header — Config does
// NOT deep-copy, and it does not need to: the registry deep-copies
// ProviderOverrides at construction (NewRegistry), and no other Config field
// holds reference state.
func TestConfig_RoundTrip_Populated(t *testing.T) {
	pi := provider.Manifest{Name: "pi", DefaultModel: "glm-5.2"}
	myagent := provider.Manifest{Name: "myagent", Command: "/opt/a"}

	cfg := Config{
		Provider:            "claude",
		Model:               "glm-5.2",
		Timeout:             90 * time.Second,
		AutoStageAll:        false,
		Verbose:             true,
		NoColor:             true,
		MaxDiffBytes:        123456,
		MaxMdLines:          42,
		MaxDuplicateRetries: 7,
		SubjectTargetChars:  60,
		Output:              provider.OutputJSON,
		StripCodeFence:      false,
		ConfigPath:          "/tmp/x.toml",
		ProviderOverrides: map[string]provider.Manifest{
			"pi":      pi,
			"myagent": myagent,
		},
	}

	// A value copy preserves every field (Config has no methods that mutate).
	c2 := cfg

	// Scalars: each reads back the exact distinct value set above.
	if c2.Provider != "claude" {
		t.Errorf("Provider = %q, want \"claude\"", c2.Provider)
	}
	if c2.Model != "glm-5.2" {
		t.Errorf("Model = %q, want \"glm-5.2\"", c2.Model)
	}
	if c2.Timeout != 90*time.Second {
		t.Errorf("Timeout = %v, want 90s", c2.Timeout)
	}
	if c2.AutoStageAll != false {
		t.Errorf("AutoStageAll = %v, want false", c2.AutoStageAll)
	}
	if c2.Verbose != true {
		t.Errorf("Verbose = %v, want true", c2.Verbose)
	}
	if c2.NoColor != true {
		t.Errorf("NoColor = %v, want true", c2.NoColor)
	}
	if c2.MaxDiffBytes != 123456 {
		t.Errorf("MaxDiffBytes = %d, want 123456", c2.MaxDiffBytes)
	}
	if c2.MaxMdLines != 42 {
		t.Errorf("MaxMdLines = %d, want 42", c2.MaxMdLines)
	}
	if c2.MaxDuplicateRetries != 7 {
		t.Errorf("MaxDuplicateRetries = %d, want 7", c2.MaxDuplicateRetries)
	}
	if c2.SubjectTargetChars != 60 {
		t.Errorf("SubjectTargetChars = %d, want 60", c2.SubjectTargetChars)
	}
	if c2.Output != provider.OutputJSON {
		t.Errorf("Output = %q, want provider.OutputJSON %q", c2.Output, provider.OutputJSON)
	}
	if c2.StripCodeFence != false {
		t.Errorf("StripCodeFence = %v, want false", c2.StripCodeFence)
	}
	if c2.ConfigPath != "/tmp/x.toml" {
		t.Errorf("ConfigPath = %q, want \"/tmp/x.toml\"", c2.ConfigPath)
	}

	// ProviderOverrides: two entries, each reading back its set fields.
	if got := len(c2.ProviderOverrides); got != 2 {
		t.Fatalf("len(ProviderOverrides) = %d, want 2", got)
	}
	gotPi, ok := c2.ProviderOverrides["pi"]
	if !ok {
		t.Fatal("ProviderOverrides missing key \"pi\"")
	}
	if gotPi.Name != "pi" {
		t.Errorf("ProviderOverrides[\"pi\"].Name = %q, want \"pi\"", gotPi.Name)
	}
	if gotPi.DefaultModel != "glm-5.2" {
		t.Errorf("ProviderOverrides[\"pi\"].DefaultModel = %q, want \"glm-5.2\"", gotPi.DefaultModel)
	}
	gotAgent, ok := c2.ProviderOverrides["myagent"]
	if !ok {
		t.Fatal("ProviderOverrides missing key \"myagent\"")
	}
	if gotAgent.Name != "myagent" {
		t.Errorf("ProviderOverrides[\"myagent\"].Name = %q, want \"myagent\"", gotAgent.Name)
	}
	if gotAgent.Command != "/opt/a" {
		t.Errorf("ProviderOverrides[\"myagent\"].Command = %q, want \"/opt/a\"", gotAgent.Command)
	}
}
