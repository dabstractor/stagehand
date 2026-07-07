// planner.go implements the planner agent invocation for multi-commit decomposition
// (PRD §13.6.2 / §13.6.4 / §13.6.6, §9.14 FR-M3/M4/M11).
//
// callPlanner is the decompose analogue of generate.CommitStaged's generation loop,
// specialized to the planner's structured-JSON output contract: it captures the full
// working-tree diff, assembles the §17.5 planner prompt (system + forced/normal user
// payload), Renders the resolved planner manifest in BARE mode, Executes the agent,
// parses prompt.PlannerOutput with one retry on parse/contract failure, enforces the
// single⇔message contract (FR-M11 single-shortcut: single==true ⇒ Message present) and
// the auto-decompose safety cap (FR-M4: Count ≤ MaxCommits), and returns the parsed
// partition. It generalizes the proven v1 generate loop to the planner role's JSON
// output + the single-shortcut + the safety cap — no new architectural concept.
//
// The planner performs ZERO git mutations (planning precedes all staging, so any
// failure is non-rescue — no snapshot/tree to rescue, §13.6.6).
//
// Consumed by the orchestrator (P3.M4.T1.S1); no caller wiring in this file.
package decompose

import (
	"context"
	"errors"
	"fmt"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/prompt"
	"github.com/dustin/stagecoach/internal/provider"
)

// ErrPlannerFailed is the sentinel for planner-agent failures (unparseable output
// after the one retry, single⇔message contract violation after the retry, agent
// non-zero exit after the retry, timeout, cancellation, or a render error). Wrapped
// (%w) around the underlying cause so errors.Is works.
//
// The orchestrator (P3.M4.T1.S1) treats ANY callPlanner error as NON-RESCUE:
// planning precedes all staging, so no snapshot/tree exists when the planner fails
// (PRD §13.6.6 — "no commits have been made yet; surface the error and exit
// non-rescue"). The auto-mode safety-cap error is NOT wrapped in this sentinel
// (it is a distinct, actionable remediation) but is likewise non-rescue.
var ErrPlannerFailed = errors.New("decompose: planner failed")

// callPlanner invokes the planner agent once (with one corrective retry) over the
// frozen T_start diff (baseTree → tStart) + style examples, parses the structured-JSON
// partition, and returns the PlannerOutput (concepts list or single-shortcut message).
//
// FR-M1b: the planner diffs the FROZEN T_start (TreeDiff(baseTree, tStart)), NOT a live
// WorkingTreeDiff. A concurrent working-tree change after the freeze is invisible to the planner.
//
// Pipeline: derive planner model → TreeDiff(baseTree, tStart) → system prompt + payload →
// Render(bare) → Execute → ParsePlannerOutput + validate → retry once on
// parse/contract failure → safety cap (auto mode only) → return.
//
// The model is derived via config.ResolveRoleModel("planner", deps.Config) because
// Deps carries RoleManifests (merged-but-unresolved manifests) but no pre-resolved
// (provider, model) pairs — the orchestrator retains those separately.
//
// Retries: at most 2 Execute calls (1 initial + 1 retry). The retry is triggered
// by ParsePlannerOutput error OR validatePlannerOutput error (shared budget).
// Timeout/cancel return ErrPlannerFailed immediately (no retry — the agent was
// killed). On the retry, PlannerRetryInstruction is prepended to the payload.
func callPlanner(ctx context.Context, deps Deps, forcedCount int, isUnborn bool, baseTree, tStart string) (prompt.PlannerOutput, error) {
	// 1. Derive the <role> model — Deps has no Models field. (Provider is the manifest name; it is NOT
	// passed to Render — v3 FR-R5b folds the inference backend into the model slash-prefix.)
	_, mdl, rsn := config.ResolveRoleModel("planner", deps.Config)

	// 2. Build the system prompt (style examples) BEFORE the diff so the FR3i prompt-reserve seam
	// (P1.M4.T1.S2) can measure its worst-case token count and thread it into opts.PromptReserveTokens.
	examples, err := plannerExamples(ctx, deps.Git, isUnborn)
	if err != nil {
		return prompt.PlannerOutput{}, fmt.Errorf("%w: recent messages: %w", ErrPlannerFailed, err)
	}
	sysPrompt := prompt.BuildPlannerSystemPrompt(examples, deps.Config.Format, deps.Config.Locale, forcedCount, deps.Config.MaxCommits)
	reserve := prompt.PlannerReserveTokens(sysPrompt, forcedCount, deps.Config.Context, git.EstimateTokens)

	// FR3j closed-loop: when token_limit is set, the gate re-measures the ACTUAL assembled prompt
	// (sysPrompt + BuildPlannerUserPayload(gatedDiff)) after water-fill truncation and re-trims until
	// it fits token_limit. nil when TokenLimit==0 (the gate branch doesn't run; byte-identical legacy path).
	var measureAssembled func(string) int
	if deps.Config.TokenLimit != 0 {
		measureAssembled = func(gatedDiff string) int {
			return git.EstimateTokens(sysPrompt + prompt.BuildPlannerUserPayload(gatedDiff, deps.Config.Context, forcedCount))
		}
	}

	// 3. FR-M1b: the FROZEN concept diff — TreeDiff(baseTree, tStart), with binary placeholders per FR3c.
	// NOT a live WorkingTreeDiff (a concurrent change after the freeze must be invisible to the planner).
	// PromptReserveTokens carries the worst-case prompt token count for M4.T2's water-fill / M4.T3's gate.
	diff, err := deps.Git.TreeDiff(ctx, baseTree, tStart, git.StagedDiffOptions{
		MaxDiffBytes:        deps.Config.MaxDiffBytes,
		MaxMDLines:          deps.Config.MaxMdLines,
		BinaryExtensions:    deps.Config.BinaryExtensions,
		Excludes:            deps.Excludes,
		TokenLimit:          deps.Config.TokenLimit,
		DiffContext:         deps.Config.DiffContextValue(),
		PromptReserveTokens: reserve,
		MeasureAssembled:    measureAssembled,
	})
	if err != nil {
		return prompt.PlannerOutput{}, fmt.Errorf("%w: tree diff: %w", ErrPlannerFailed, err)
	}

	// 4. The base user payload (style examples + system prompt are already built above).
	basePayload := prompt.BuildPlannerUserPayload(diff, deps.Config.Context, forcedCount)

	// 5. The retry loop (≤2 attempts: 1 initial + 1 retry on parse/contract failure).
	const maxAttempts = 2
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		payload := basePayload
		if attempt > 0 {
			payload = prompt.PlannerRetryInstruction + "\n\n" + payload
			deps.Verbose.VerboseRetry(attempt, "planner output unparseable or contract-invalid")
		}

		// v3 FR-R5b: the inference provider is the model slash-prefix ("inference/model"),
		// which Render splits into --provider <inference>. P1.M2 wires real per-role reasoning
		// via ResolveRoleModel's 3rd return (rsn).
		spec, rerr := deps.Roles.Planner.Render(mdl, sysPrompt, payload, rsn, provider.RenderBare)
		if rerr != nil {
			return prompt.PlannerOutput{}, fmt.Errorf("%w: render: %w", ErrPlannerFailed, rerr)
		}

		out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) || errors.Is(execErr, context.Canceled) {
				return prompt.PlannerOutput{}, fmt.Errorf("%w: %w", ErrPlannerFailed, execErr) // non-rescue; no retry
			}
			// Non-zero exit — stdout may be partial; fall through to parse.
		}

		parsed, perr := prompt.ParsePlannerOutput(out)
		if perr != nil {
			lastErr = perr
			continue // parse failure → retry
		}
		if verr := validatePlannerOutput(parsed); verr != nil {
			lastErr = verr
			continue // single⇔message / count contract → retry (same budget)
		}

		// 5. Accepted output — enforce the safety cap (auto mode only, FR-M4).
		if forcedCount == 0 && parsed.Count > deps.Config.MaxCommits {
			return prompt.PlannerOutput{}, fmt.Errorf(
				"planner proposed %d commits; exceeds max_commits (%d); use --commits or --max-commits",
				parsed.Count, deps.Config.MaxCommits)
		}
		return parsed, nil // SUCCESS
	}
	return prompt.PlannerOutput{}, fmt.Errorf("%w: %w", ErrPlannerFailed, lastErr)
}

// validatePlannerOutput enforces the single⇔message contract (FR-M11) and basic
// structural invariants on a parsed PlannerOutput. Returns nil on valid; a non-nil
// error triggers the one retry (same budget as a parse failure).
//
// Checks:
//   - Count ≥ 1
//   - Single==true ⇒ Message is non-empty (the shortcut is unusable without a message)
//   - Single==false ⇒ at least one Commit entry
//
// Lenient on Single==false + non-empty Message (harmless; orchestrator ignores it).
func validatePlannerOutput(out prompt.PlannerOutput) error {
	if out.Count < 1 {
		return errors.New("planner output: count < 1")
	}
	if out.Single {
		if out.Message == "" {
			return errors.New("planner output: single==true but message is empty") // FR-M11 load-bearing
		}
	} else if len(out.Commits) == 0 {
		return errors.New("planner output: single==false but no commits")
	}
	return nil
}

// plannerExamples returns the §17.5 style examples: RecentMessages(20) on a mature
// repo, or nil on an unborn repo (short-circuit — mirrors generate.buildSystemPrompt /
// recentSubjects; RecentMessages would exit 128 on unborn). The §17.5 planner prompt
// uses RecentMessages ONLY (no DetectMultiline/SubjectTargetChars — §17.5 omits both).
func plannerExamples(ctx context.Context, g git.Git, isUnborn bool) ([]string, error) {
	if isUnborn {
		return nil, nil
	}
	return g.RecentMessages(ctx, 20)
}
