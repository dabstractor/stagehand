// Package config — multi-turn config-knob tests (PRD §9.24 FR-T1c/FR-T3).
//
// TestMaterializeOverlay_MultiTurn proves the two multi-turn [generation] keys thread through the
// file-decode layer (fileGeneration → materialize → overlay) into the resolved Config the generate core
// (P1.M1.T3.S2 reads cfg.MultiTurnChunkTokens; P1.M1.T3.S3 reads cfg.MultiTurnFallback) consumes.
// Mirrors the gold-standard TestMaterializeOverlay_DiffContext_TokenLimit.
//
// Contract cases (P1.M1.T2.S3):
//
//	(b) multi_turn_chunk_tokens = 16000 in a file → resolved 16000 (int override honored)
//	(c) multi_turn_fallback = true in a file → resolved true (bool override honored)
//	(d) multi_turn_fallback = false in a file → resolved STILL true (ACCEPTED v1 limitation:
//	    only-true-propagates, mirrors AutoStageAll; cannot disable via file)
//	(e) overlay precedence: repo-file chunk-tokens value overrides global-file value
//
// Case (a) — Defaults() returns true/32000 — is already pinned by TestDefaults (S1, LANDED); not duplicated.
package config

import "testing"

// TestMaterializeOverlay_MultiTurn is the load-bearing proof for the multi-turn config knobs across the
// materialize (file→Config) and overlay (Defaults → file) layers.
func TestMaterializeOverlay_MultiTurn(t *testing.T) {
	// ---- materialize-only: file → Config (the file→Config copy BEFORE any Defaults overlay) ----
	// materialize does NOT seed Defaults; an omitted key yields the Go zero-value (false/0).
	materializeCases := []struct {
		name         string
		fileFallback bool
		fileChunk    int
		wantFallback bool // omitted ⇒ false (Go zero-value); materialize does NOT seed the true default
		wantChunk    int  // omitted ⇒ 0
	}{
		{"both_omitted_zero_value", false, 0, false, 0},
		{"chunk_set_16000", false, 16000, false, 16000}, // (b) int honored at the file→Config copy
		{"fallback_set_true", true, 0, true, 0},         // (c) bool=true honored
		// The root cause of the limitation: false is INDISTINGUISHABLE from omitted at materialize time:
		{"fallback_false_same_as_omitted", false, 0, false, 0},
	}
	for _, tc := range materializeCases {
		tc := tc
		t.Run("materialize/"+tc.name, func(t *testing.T) {
			fc := &fileConfig{Generation: fileGeneration{
				MultiTurnFallback:    tc.fileFallback,
				MultiTurnChunkTokens: tc.fileChunk,
			}}
			c := materialize(fc, 0)
			if c.MultiTurnFallback != tc.wantFallback {
				t.Errorf("MultiTurnFallback = %v, want %v (materialize: omitted/false ⇒ Go zero-value false; it does NOT seed the default)", c.MultiTurnFallback, tc.wantFallback)
			}
			if c.MultiTurnChunkTokens != tc.wantChunk {
				t.Errorf("MultiTurnChunkTokens = %d, want %d (materialize: omitted ⇒ 0; set ⇒ propagated)", c.MultiTurnChunkTokens, tc.wantChunk)
			}
		})
	}

	// ---- overlay chain: Defaults() → overlay(file) — the RESOLVED value the generate core reads ----
	overlayCases := []struct {
		name         string
		fileFallback bool
		fileChunk    int
		wantFallback bool
		wantChunk    int
	}{
		{"omitted_keeps_defaults", false, 0, true, 32000},            // both omitted ⇒ Defaults win
		{"chunk_override_16000", false, 16000, true, 16000},          // (b) int override honored end-to-end
		{"chunk_override_48000", false, 48000, true, 48000},          // a larger value
		{"fallback_true_reasserts_true", true, 0, true, 32000},       // (c) bool=true honored (redundant w/ default)
		{"fallback_false_ignored_STAYS_true", false, 0, true, 32000}, // (d) THE limitation: false CANNOT disable
	}
	for _, tc := range overlayCases {
		tc := tc
		t.Run("overlay/"+tc.name, func(t *testing.T) {
			cfg := Defaults() // MultiTurnFallback=true, MultiTurnChunkTokens=32000
			g := materialize(&fileConfig{Generation: fileGeneration{
				MultiTurnFallback:    tc.fileFallback,
				MultiTurnChunkTokens: tc.fileChunk,
			}}, 0)
			overlay(&cfg, g)
			if cfg.MultiTurnFallback != tc.wantFallback {
				if tc.name == "fallback_false_ignored_STAYS_true" {
					t.Errorf("MultiTurnFallback = %v, want true — ACCEPTED v1 limitation: "+
						"multi_turn_fallback=false in a file is silently ignored (only-true-propagates, "+
						"mirrors auto_stage_all). This assertion deliberately PINS the limitation as "+
						"known/tested behavior, not a silent bug. To disable multi-turn for a provider, "+
						"set session_mode = \"\" on that provider (see docs/configuration.md). [got %v]", cfg.MultiTurnFallback, cfg.MultiTurnFallback)
				} else {
					t.Errorf("MultiTurnFallback = %v, want %v (resolved value the generate core reads)", cfg.MultiTurnFallback, tc.wantFallback)
				}
			}
			if cfg.MultiTurnChunkTokens != tc.wantChunk {
				t.Errorf("MultiTurnChunkTokens = %d, want %d (resolved value the generate core reads)", cfg.MultiTurnChunkTokens, tc.wantChunk)
			}
		})
	}

	// ---- (e) overlay precedence: repo-file overrides global-file for chunk tokens ----
	t.Run("overlay/repo_overrides_global_chunk_tokens", func(t *testing.T) {
		cfg := Defaults() // 32000
		global := materialize(&fileConfig{Generation: fileGeneration{MultiTurnChunkTokens: 48000}}, 0)
		overlay(&cfg, global)
		if cfg.MultiTurnChunkTokens != 48000 {
			t.Fatalf("after global overlay: MultiTurnChunkTokens = %d, want 48000", cfg.MultiTurnChunkTokens)
		}
		repo := materialize(&fileConfig{Generation: fileGeneration{MultiTurnChunkTokens: 16000}}, 0)
		overlay(&cfg, repo) // higher layer (repo) wins
		if cfg.MultiTurnChunkTokens != 16000 {
			t.Errorf("after repo overlay: MultiTurnChunkTokens = %d, want 16000 (repo overrides global; higher layer wins)", cfg.MultiTurnChunkTokens)
		}
	})

	// ---- end-to-end via loadTOML (proves the TOML decode → resolved value path) ----
	t.Run("loadTOML/chunk_override_end_to_end", func(t *testing.T) {
		body := `
[generation]
multi_turn_chunk_tokens = 16000
`
		path := writeTempTOML(t, body)
		cfg, err := loadTOML(path)
		if err != nil || cfg == nil {
			t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
		}
		if cfg.MultiTurnChunkTokens != 16000 {
			t.Errorf("loadTOML MultiTurnChunkTokens = %d, want 16000 (materialized file value)", cfg.MultiTurnChunkTokens)
		}
		dst := Defaults()
		overlay(&dst, cfg)
		if dst.MultiTurnChunkTokens != 16000 {
			t.Errorf("after overlay: MultiTurnChunkTokens = %d, want 16000 (resolved)", dst.MultiTurnChunkTokens)
		}
		if !dst.MultiTurnFallback {
			t.Errorf("after overlay: MultiTurnFallback = false, want true (key omitted ⇒ default wins)")
		}
	})

	// ---- end-to-end: multi_turn_fallback = false is silently ignored (the ACCEPTED limitation) ----
	t.Run("loadTOML/fallback_false_ignored_end_to_end", func(t *testing.T) {
		body := `
[generation]
multi_turn_fallback = false
`
		path := writeTempTOML(t, body)
		cfg, err := loadTOML(path)
		if err != nil || cfg == nil {
			t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
		}
		// At materialize time the false decodes — INDISTINGUISHABLE from omitted (both false):
		if cfg.MultiTurnFallback {
			t.Errorf("loadTOML MultiTurnFallback = true, want false (materialized zero-value; false == omitted)")
		}
		// But after overlay on Defaults, the false CANNOT disable the default-true:
		dst := Defaults()
		overlay(&dst, cfg)
		if !dst.MultiTurnFallback {
			t.Errorf("after overlay: MultiTurnFallback = %v, want true — ACCEPTED v1 limitation: "+
				"multi_turn_fallback=false in a config file is silently ignored (only-true-propagates, "+
				"mirrors auto_stage_all). Pinned here as known/tested behavior.", dst.MultiTurnFallback)
		}
	})
}
