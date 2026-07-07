package generate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/stubtest"
)

// assembledPromptSeparatorTokens is the Render stdin-separator allowance: provider.Render prepends
// `sysPrompt + "\n\n" + userPayload` to the stub's stdin when the manifest has no system_prompt_flag
// (render.go:158). The FR3j MeasureAssembled closure measures `EstimateTokens(sysPrompt + payload)`
// (Go `+`, NO separator). So capturedStdin = closure_measurement + "\n\n", and EstimateTokens rises by
// <=1 (ceil(runes/4) on +2 runes). FR3j guarantees closure_measurement <= tokenLimit, therefore
// capturedStdin <= tokenLimit + 1. The +1 is the bounded separator artifact, NOT a violation of FR3j
// (whose invariant is on the separator-free assembled prompt the closure measures). Equal to
// git.EstimateTokens("\n\n") = ceil(2/4) = 1.
const assembledPromptSeparatorTokens = 1

// TestCommitStaged_TokenLimitInvariant_AssembledPromptFits (PRD §9.1 FR3j / §20.5) is the INTEGRATION
// proof that the closed-loop token-budget guarantee holds end-to-end on the message-role path: real git
// diff capture -> the REAL MeasureAssembled closure -> closedLoopGate -> provider.Execute (the stub) ->
// the captured stdin fits token_limit. Distinct from S1's PURE unit test (synthetic measures): this
// drives CommitStaged against a real temp repo with a stub provider and asserts on the stub's RECEIVED
// stdin (the closest observable to the assembled prompt).
//
// The stub manifest has NO system_prompt_flag, so Render prepends sysPrompt + "\n\n" + payload to stdin
// (render.go:158); the captured stdin IS the full assembled prompt (+ the 1-token separator allowance).
// The gate is forced to run by making the untruncated prompt far exceed tokenLimit (~345 tokens gated
// to 200), and the "[truncated]" sentinel proves the closed-loop ACTIVELY truncated (not a no-op).
func TestCommitStaged_TokenLimitInvariant_AssembledPromptFits(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// A large staged diff: ~1600 runes of changes => EstimateTokens(diff) ~= 390 >> tokenLimit=200.
	// The gate MUST truncate to fit (forcing the closed loop to run).
	writeFile(t, repo, "feature.go", "package main\n")
	stageFile(t, repo, "feature.go")
	body := strings.Repeat("change line content here\n", 60) // 23 runes/line x 60 ~= 1380 runes ~= 345 tokens
	writeFile(t, repo, "big.go", body)                       // the large body the gate truncates
	stageFile(t, repo, "big.go")

	// Capture the stub's received stdin (the assembled prompt). Mirrors
	// TestCommitStaged_ExcludedPayloadCapture (generate_test.go:660).
	stdinFile := filepath.Join(t.TempDir(), "stdin.txt")
	t.Setenv("STAGEHAND_STUB_STDINFILE", stdinFile)
	stub := stubtest.Build(t)
	m := stubtest.Manifest(stub, stubtest.Options{Out: "feat: add big feature"})

	cfg := config.Config{
		Provider: "stub",
		Model:    "stub",
		Timeout:  30 * time.Second,
	}
	cfg.TokenLimit = 200 // << 345 => the water-fill + closed-loop gate MUST truncate to fit

	deps := Deps{Git: git.New(repo), Manifest: m}
	if _, err := CommitStaged(context.Background(), deps, cfg); err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}

	// Read the captured assembled prompt (sysPrompt + "\n\n" + payload wrapping the gated diff).
	data, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("read captured stdin: %v (did the stub run? STAGEHAND_STUB_STDINFILE=%s)", err, stdinFile)
	}
	captured := string(data)

	// (FR3j invariant) EstimateTokens(assembled prompt) <= tokenLimit + separator allowance. The +1 is
	// the Render "\n\n" separator (render.go:158); the closure measures the separator-free prompt.
	measured := git.EstimateTokens(captured)
	if measured > cfg.TokenLimit+assembledPromptSeparatorTokens {
		t.Errorf("FR3j invariant violated: EstimateTokens(captured stdin) = %d, want <= %d (tokenLimit %d + %d separator allowance)\n"+
			"captured stdin (first 400 chars): %q", measured, cfg.TokenLimit+assembledPromptSeparatorTokens,
			cfg.TokenLimit, assembledPromptSeparatorTokens, truncForLog(captured, 400))
	}

	// (Gate-ran proof) the closed loop ACTIVELY truncated — the water-fill sentinel is present. A no-op
	// (payload fit without truncation) would lack it; with tokenLimit=200 << 345, truncation is mandatory.
	if !strings.Contains(captured, "[truncated]") {
		t.Errorf("expected the water-fill '[truncated]' sentinel in the captured stdin (tokenLimit=%d << untruncated~%d), "+
			"got none — the closed-loop gate did not truncate (was it wired?)\ncaptured (first 400 chars): %q",
			cfg.TokenLimit, git.EstimateTokens(body), truncForLog(captured, 400))
	}
}

// truncForLog returns s truncated to the first n runes for a readable test failure message.
func truncForLog(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
