package generate

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/stubtest"
	"github.com/dabstractor/stagecoach/internal/ui"
)

// TestMultiTurnFallback_FalseFromFile proves Issue 1's *bool fix (P1.M1.T1.S1) propagates
// multi_turn_fallback END-TO-END from a TOML FILE (via the public config.Load API) to the generate
// CONSUMER (the FR-T1 multi-turn trigger gate inside CommitStaged). This is the file→consumer proof,
// COMPLEMENTARY to (and NOT duplicating) S1's white-box materialize/overlay unit test in
// internal/config/file_test.go (different package, different layer).
//
// Why in-process, NOT e2e (the gotcha): a one-shot-exhaust + large-diff + session_mode=append setup
// yields exit 3 (Rescue) for BOTH multi_turn_fallback=true (if chunked calls also fail) AND =false.
// Exit code cannot distinguish them. The deterministic distinguisher is the verbose trigger line
// "multi-turn fallback" in the captured *bytes.Buffer — observable cleanly in-process (mirrors
// multiturn_test.go's TestMultiTurnTriggerGate_TruthTable), fragile via subprocess stderr.
//
// The CONTROL row (true) proves the test setup is CAPABLE of firing multi-turn, so the false-case
// absence of the trigger is meaningful (not a setup defect like cond b false). This control is what
// makes (c) a trustworthy regression guard.
//
// Distinct from TestMultiTurnTriggerGate_TruthTable: that test sets cfg.MultiTurnFallback DIRECTLY
// (boolPtr). This test SOURCES cfg from a TOML FILE via config.Load to prove file→consumer end-to-end.
func TestMultiTurnFallback_FalseFromFile(t *testing.T) {
	bin := stubtest.Build(t)

	cases := []struct {
		name        string
		tomlBody    string
		wantTrigger bool
	}{
		{
			// (c) the headline: TOML false ⇒ MultiTurnFallbackValue()==false ⇒ FR-T1 gate short-circuits
			// (cond c false) ⇒ trigger ABSENT ⇒ falls through to rescue (*RescueError{ErrRescue}).
			name:        "false_no_trigger_falls_to_rescue",
			tomlBody:    "config_version = 3\n[generation]\nmulti_turn_fallback = false\n",
			wantTrigger: false,
		},
		{
			// CONTROL: identical setup but TOML true ⇒ cond c true ⇒ gate FIRES ⇒ trigger PRESENT.
			// Proves the setup can fire multi-turn, so the false-case absence is meaningful (not a
			// setup defect). Without this row, a "no trigger" false result could hide cond b false.
			name:        "control_true_fires_trigger",
			tomlBody:    "config_version = 3\n[generation]\nmulti_turn_fallback = true\n",
			wantTrigger: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			initRepo(t, repo)
			commitRaw(t, repo, "initial")
			// ~96 runes ⇒ EstimateTokens ≈ 24 > chunkTokens(4) ⇒ FR-T1 cond (b) true (payload exceeds
			// one chunk). ONLY cond (c) varies across the rows; the control proves the gate can fire.
			writeFile(t, repo, "new.txt", strings.Repeat("change line\n", 8))
			stageFile(t, repo, "new.txt")

			// SOURCE cfg from a TOML file (end-to-end file→consumer), NOT a direct struct assignment.
			// writeTempTOML is package config, so write the file inline here. DisableBootstrap:true
			// skips the first-run auto-write; RepoDir feeds loadGitConfig (Layer 4) — empty/absent
			// keys ⇒ nil pointer ⇒ overlay inherits lower layer (the TOML file is Layer 2 here).
			tomlPath := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(tomlPath, []byte(tc.tomlBody), 0o644); err != nil {
				t.Fatalf("write TOML: %v", err)
			}
			cfgPtr, err := config.Load(context.Background(), config.LoadOpts{
				ConfigPathOverride: tomlPath,
				RepoDir:            repo,
				DisableBootstrap:   true,
			})
			if err != nil {
				t.Fatalf("config.Load: %v", err)
			}
			cfg := *cfgPtr

			// (c) headline accessor assertion: TOML false reached the resolved Config.
			if got := cfg.MultiTurnFallbackValue(); got != tc.wantTrigger {
				t.Fatalf("MultiTurnFallbackValue() = %v, want %v (TOML did not reach the resolved Config)",
					got, tc.wantTrigger)
			}

			// Make cond (b) true so ONLY cond (c) varies (mirrors multiturn_test.go which sets these
			// directly). With the default 32000 the small diff would NOT exceed it ⇒ cond b false for
			// BOTH rows, hiding the gate. The loaded TOML does not set these (it only sets the bool).
			cfg.MultiTurnChunkTokens = 4 // ~24-token diff > 4 ⇒ cond (b) true
			cfg.MaxDuplicateRetries = 0  // exactly one one-shot attempt ⇒ exhaust ⇒ reach the FR-T1 gate

			// SessionMode="append" (cond d true) + unparseable one-shot (call 1 = "" ⇒ cond a true).
			// omitAppend=false ⇒ SessionMode set to "append".
			m := stubAppendManifest(t, bin, []string{""}, false)

			var buf bytes.Buffer
			_, err = CommitStaged(context.Background(), Deps{
				Git:      git.New(repo),
				Manifest: m,
				Verbose:  ui.NewVerbose(&buf, true),
			}, cfg)

			gotTrigger := strings.Contains(buf.String(), "multi-turn fallback")
			if gotTrigger != tc.wantTrigger {
				t.Errorf("trigger-in-buf = %v, want %v; buf tail: %q",
					gotTrigger, tc.wantTrigger, tail(buf.String(), 200))
			}

			// Both rows exhaust the one-shot (call 1 = "") and reach the FR-T1 gate decision. The false
			// row short-circuits the gate (no multi-turn) ⇒ rescue. The true row FIRES multi-turn, but
			// the single-call script ("") yields only call 1 (empty) — the multi-turn turn-1 priming
			// expects "ok"; an exhausted script returns "" which fails to parse ⇒ the run also exhausts
			// and returns rescue. So BOTH rows assert *RescueError{Kind:ErrRescue} — the trigger line is
			// the distinguishing observable between "gate fired then failed" (true) and "gate skipped"
			// (false). (Mirror the truth-table's skip rows which all assert rescue.)
			var re *RescueError
			if !errors.As(err, &re) || re.Kind != ErrRescue {
				t.Errorf("err = %v, want *RescueError{Kind:ErrRescue} (both rows rescue; the trigger is "+
					"the distinguisher)", err)
			}
		})
	}
}
