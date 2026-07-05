// Package prompt: pure table tests for the FR3d/FR3i prompt-reserve helpers (P1.M4.T1.S2).
//
// These tests are PURE: no git, no I/O, no internal imports. A LOCAL estimator mirrors S1's
// git.EstimateTokens (ceil(runes/4) ceiling) so the tests can assert exact ints without depending on
// internal/git. The reserve helpers take an injected TokenEstimator, so this local estimator exercises
// the same code path that production (git.EstimateTokens) drives.
package prompt

import (
	"strings"
	"testing"
)

// est mirrors S1's git.EstimateTokens: ceil(runeCount/4), rounded UP. Rune-based (not byte-based) so
// multi-byte UTF-8 does not over-count. Kept local to keep the test pure (no internal/git import).
func est(s string) int {
	r := len([]rune(s))
	return (r + 3) / 4
}

// TestMessageReserveTokens_StableWorstCase pins the contract's central assertion: the message reserve is
// a WORST-CASE ceiling, stable across retry attempts (there is no per-attempt parameter — it is inherent),
// and it grows monotonically with maxDuplicateRetries. The reserve must equal est(sysPrompt) +
// est(BuildUserPayload with the worst-case rejected slice) + reserveSafetyMargin (the empty-diff trick).
func TestMessageReserveTokens_StableWorstCase(t *testing.T) {
	const sysPrompt = "S"
	r0 := MessageReserveTokens(sysPrompt, 0, 50, "", est)
	r3 := MessageReserveTokens(sysPrompt, 3, 50, "", est)

	// The reserve is a worst-case ceiling — more retries ⇒ more slots ⇒ larger overhead ⇒ larger reserve.
	if r3 <= r0 {
		t.Fatalf("reserve must grow with maxDuplicateRetries: r0=%d r3=%d (want r3 > r0)", r0, r3)
	}

	// r0 (no rejection slots) == normal-instruction overhead only.
	wantR0 := est(sysPrompt) + est(BuildUserPayload("", "", nil)) + reserveSafetyMargin
	if r0 != wantR0 {
		t.Fatalf("r0 = %d, want %d (normal instruction overhead + margin)", r0, wantR0)
	}

	// r3 == worst-case rejection path: 3 synthetic subjects at ~subjectTargetChars each.
	worstRejected := []string{
		strings.Repeat("x", 50),
		strings.Repeat("x", 50),
		strings.Repeat("x", 50),
	}
	wantR3 := est(sysPrompt) + est(BuildUserPayload("", "", worstRejected)) + reserveSafetyMargin
	if r3 != wantR3 {
		t.Fatalf("r3 = %d, want %d (worst-case rejection path + margin)", r3, wantR3)
	}

	// Stability: the helper takes maxDuplicateRetries (NOT a per-attempt count) — calling it twice yields
	// the same value. (No per-attempt parameter exists, so "stability across attempts" is inherent; this
	// asserts the function is a pure ceiling function of its inputs.)
	if a, b := MessageReserveTokens(sysPrompt, 3, 50, "", est), MessageReserveTokens(sysPrompt, 3, 50, "", est); a != b {
		t.Fatalf("reserve not stable: two calls yielded %d and %d", a, b)
	}
}

// TestMessageReserveTokens_GrowsWithContext asserts the reserve includes the §17.8 user-context block:
// a non-empty context adds tokens (the contextIntro header + the context text).
func TestMessageReserveTokens_GrowsWithContext(t *testing.T) {
	const sysPrompt = "S"
	rEmpty := MessageReserveTokens(sysPrompt, 3, 50, "", est)
	rCtx := MessageReserveTokens(sysPrompt, 3, 50, "some context provided by the user", est)
	if rCtx <= rEmpty {
		t.Fatalf("reserve must grow with non-empty context: empty=%d ctx=%d (want ctx > empty)", rEmpty, rCtx)
	}
}

// TestMessageReserveTokens_NegativeClamp guards against a misconfigured negative maxDuplicateRetries: it
// must behave like 0 (no rejection slots, no negative-length slice, no panic).
func TestMessageReserveTokens_NegativeClamp(t *testing.T) {
	const sysPrompt = "S"
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("negative maxDuplicateRetries panicked: %v", r)
		}
	}()
	rNeg := MessageReserveTokens(sysPrompt, -1, 50, "", est)
	r0 := MessageReserveTokens(sysPrompt, 0, 50, "", est)
	if rNeg != r0 {
		t.Fatalf("negative maxDuplicateRetries should clamp to 0: rNeg=%d r0=%d", rNeg, r0)
	}
}

// TestMessageReserveTokens_NegativeSubjectClamp guards against a misconfigured negative subjectTargetChars:
// strings.Repeat would panic; the helper clamps to ≥1.
func TestMessageReserveTokens_NegativeSubjectClamp(t *testing.T) {
	const sysPrompt = "S"
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("negative subjectTargetChars panicked: %v", r)
		}
	}()
	_ = MessageReserveTokens(sysPrompt, 3, -5, "", est) // must not panic
}

// TestPlannerReserveTokens pins the planner reserve: it includes PlannerRetryInstruction in the overhead
// (the worst case is the 1 retry) and grows with forcedCount (the forced-count directive adds a line).
func TestPlannerReserveTokens(t *testing.T) {
	const sysPrompt = "P"
	r0 := PlannerReserveTokens(sysPrompt, 0, "", est)
	r3 := PlannerReserveTokens(sysPrompt, 3, "", est)
	if r3 <= r0 {
		t.Fatalf("planner reserve must grow with forcedCount: r0=%d r3=%d (want r3 > r0)", r0, r3)
	}

	// Exact parity: the overhead is PlannerRetryInstruction + "\n\n" + BuildPlannerUserPayload("", ctx, forced).
	want := est(sysPrompt) + est(PlannerRetryInstruction+"\n\n"+BuildPlannerUserPayload("", "", 3)) + reserveSafetyMargin
	if r3 != want {
		t.Fatalf("r3 = %d, want %d (retry instr + forced payload + margin)", r3, want)
	}
}

// TestPlannerReserveTokens_ModeConditionalBuilderOutput threads the new mode-conditional builder
// output through PlannerReserveTokens to confirm it does not panic and the formula holds for both the
// auto (soft-target-interpolated) and forced (no soft target) sysPrompts.
func TestPlannerReserveTokens_ModeConditionalBuilderOutput(t *testing.T) {
	autoSys := BuildPlannerSystemPrompt(nil, "auto", "", 0, 12)
	forcedSys := BuildPlannerSystemPrompt(nil, "auto", "", 3, 12)

	rAuto := PlannerReserveTokens(autoSys, 0, "", est)
	rForced := PlannerReserveTokens(forcedSys, 3, "", est)
	if rAuto <= 0 {
		t.Fatalf("auto reserve must be positive; got %d", rAuto)
	}
	if rForced <= 0 {
		t.Fatalf("forced reserve must be positive; got %d", rForced)
	}

	// Exact parity for auto: overhead = PlannerRetryInstruction + "\n\n" + BuildPlannerUserPayload("", ctx, 0).
	wantAuto := est(autoSys) + est(PlannerRetryInstruction+"\n\n"+BuildPlannerUserPayload("", "", 0)) + reserveSafetyMargin
	if rAuto != wantAuto {
		t.Fatalf("rAuto = %d, want %d (retry instr + normal payload + margin)", rAuto, wantAuto)
	}

	// Exact parity for forced: overhead = PlannerRetryInstruction + "\n\n" + BuildPlannerUserPayload("", ctx, 3).
	wantForced := est(forcedSys) + est(PlannerRetryInstruction+"\n\n"+BuildPlannerUserPayload("", "", 3)) + reserveSafetyMargin
	if rForced != wantForced {
		t.Fatalf("rForced = %d, want %d (retry instr + forced payload + margin)", rForced, wantForced)
	}
}

// TestArbiterReserveTokens pins the arbiter reserve: it includes the commits + headers overhead and grows
// with the number of commits (each commit adds SHA + subject + files + a blank separator).
func TestArbiterReserveTokens(t *testing.T) {
	const sysPrompt = "A"
	rNil := ArbiterReserveTokens(sysPrompt, nil, est)
	rTwo := ArbiterReserveTokens(sysPrompt, []ArbiterCommit{
		{SHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", Subject: "feat: add foo", Files: []string{"a.go", "b.go"}},
		{SHA: "cafecafecafecafecafecafecafecafecafecafe", Subject: "fix: bar baz", Files: []string{"c.go"}},
	}, est)
	if rTwo <= rNil {
		t.Fatalf("arbiter reserve must grow with commits: nil=%d two=%d (want two > nil)", rNil, rTwo)
	}

	// Exact parity for the nil case: overhead = BuildArbiterUserPayload(nil, "").
	want := est(sysPrompt) + est(BuildArbiterUserPayload(nil, "")) + reserveSafetyMargin
	if rNil != want {
		t.Fatalf("rNil = %d, want %d (headers overhead + margin)", rNil, want)
	}
}

// TestMeasureReserve_MarginIncluded pins the shared core: est(sys) + est(overhead) + reserveSafetyMargin.
func TestMeasureReserve_MarginIncluded(t *testing.T) {
	got := measureReserve("A", "B", est)
	want := est("A") + est("B") + reserveSafetyMargin
	if got != want {
		t.Fatalf("measureReserve = %d, want %d (est(A)+est(B)+256=%d+%d+256)", got, want, est("A"), est("B"))
	}
	if reserveSafetyMargin != 256 {
		t.Fatalf("reserveSafetyMargin = %d, want 256 (distinct from FR3i's body_budget margin, M4.T2)", reserveSafetyMargin)
	}
}
