// Package decompose implements the multi-commit decomposition pipeline (PRD §13.6.2): given an
// un-staged working tree, it produces N logically-coherent commits by running a four-agent pipeline
// (planner → stager → message → arbiter) with per-role provider/model resolution.
//
// This file (arbiter.go) implements the arbiter agent invocation (PRD §13.6.5 / §9.14 FR-M9,
// §17.7): runArbiter is the BARE arbiter-role call — the decompose analogue of callPlanner's
// Render→Execute→parse pattern, specialized to the arbiter's {"target": "<sha>" | null} JSON output
// contract. It converts the run's commits ([]CommitInfo, each carrying SHA + Subject + a
// []git.FileChange diff-tree file-list) into []prompt.ArbiterCommit (the FileChange→path-string
// seam), builds the §17.7 prompt (BuildArbiterSystemPrompt zero-arg + BuildArbiterUserPayload),
// Renders the resolved arbiter manifest in BARE mode, Executes ONCE (no retry — §17.7 defines no
// retry instruction), parses prompt.ParseArbiterOutput, and validates the returned target is one
// of THIS run's commits (§13.6.5 "may only target a commit from this run").
//
// It returns a confident in-list target, or degrades to null (prompt.ArbiterOutput{Target: nil})
// on ANY indecision (parse failure, timeout, cancel, empty target, target-not-in-list) — the arbiter
// OWNS the §13.6.5 "when in doubt, prefer a NEW commit (return null)" decision rather than
// punting it to the resolution logic. Only a render error returns a wrapped ErrArbiterFailed.
//
// runArbiter performs ZERO git reads (the orchestrator pre-computes the []CommitInfo via DiffTree
// and the leftoverDiff via WorkingTreeDiff, and gates the call on StatusPorcelain != "" per FR-M9).
// It ONLY DECIDES — resolution (new commit / tip amend / mid-chain chain rebuild) is P3.M3.T2.S1.
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

// ErrArbiterFailed is the sentinel for the arbiter's ONE true infra failure: a render error (the
// arbiter manifest could not be rendered). Wrapped (%w) so errors.Is works. Near-impossible
// post-ResolveRoles (the manifest was Validated + install-checked), but wrapped for consistency
// with the sibling sentinels (ErrPlannerFailed/ErrStagerFailed/ErrMessageFailed) + verbose
// logging.
//
// IMPORTANT — the arbiter OWNS the §13.6.5 "when in doubt, null" decision: agent failures
// (timeout, cancel, parse-fail) and semantic ambiguity (empty target, target-not-in-list) do NOT
// return errors — they degrade to prompt.ArbiterOutput{Target: nil} (null ⇒ new commit; no work
// lost). The resolution logic (S2) reads out.Target: nil ⇒ new commit, &sha ⇒ amend. (S2 should
// treat ANY runArbiter error as null too, defensively.)
var ErrArbiterFailed = errors.New("decompose: arbiter failed")

// CommitInfo is one commit made this run, as the orchestrator builds it for the arbiter
// (PRD §13.6.5: "SHAs, messages, and file-lists (diff-tree) of every commit made this run").
// The orchestrator populates Files from git.DiffTree(sha, isRoot) verbatim (hence
// []git.FileChange, not []string). runArbiter converts these to []prompt.ArbiterCommit (the
// prompt layer's []string-path form) via convertArbiterCommits.
type CommitInfo struct {
	SHA     string           // the commit's full SHA (40/64 hex) — the value the arbiter may return as "target".
	Subject string           // the commit's subject line (§13.6.5's "messages").
	Files   []git.FileChange // the diff-tree file-list (DiffTree's return verbatim); may be empty.
}

// runArbiter invokes the arbiter agent ONCE (no retry) over the run's commits + leftover diff,
// parses the JSON {"target": "<sha>" | null} output, validates the target is one of the run's
// commits, and returns the decision (PRD §13.6.5 / §9.14 FR-M9 / §17.7).
//
// Pipeline: derive arbiter model → convert commits (FileChange→path seam) + valid-SHA set →
// BuildArbiterSystemPrompt (zero-arg) + BuildArbiterUserPayload → Render BARE → Execute ONCE →
// timeout/cancel ⇒ null / non-zero exit ⇒ fall-through to parse → ParseArbiterOutput →
// parse-fail ⇒ null → in-list validate → confident target OR null.
//
// The arbiter is SINGLE-SHOT: §17.7 defines NO retry instruction (contrast the planner which has
// PlannerRetryInstruction). "When in doubt, null" — ANY indecision degrades to
// prompt.ArbiterOutput{Target: nil} with a NIL error (the arbiter OWNS the null decision, §13.6.5).
// Only a render error returns a wrapped ErrArbiterFailed.
//
// runArbiter performs ZERO git reads: commits and leftoverDiff are PARAMETERS (the orchestrator
// pre-computes them via DiffTree and WorkingTreeDiff). The StatusPorcelain trigger is the
// orchestrator's gate (FR-M9), not runArbiter's.
func runArbiter(ctx context.Context, deps Deps, commits []CommitInfo, leftoverDiff string) (prompt.ArbiterOutput, error) {
	// 1. Derive the <role> model — Deps has no Models field. (Provider is the manifest name; it is NOT
	// passed to Render — v3 FR-R5b folds the inference backend into the model slash-prefix.)
	_, mdl, rsn := config.ResolveRoleModel("arbiter", deps.Config)
	arbiterTimeout := config.ResolveRoleTimeout("arbiter", deps.Config) // FR-R7 (§9.15/§16.1): per-role timeout (no built-in → cfg.Timeout by default)

	// 2. Convert []CommitInfo → []prompt.ArbiterCommit (FileChange→path seam) + build the valid-SHA set.
	arbiterCommits, validSHAs := convertArbiterCommits(commits)

	// 3. Build the §17.7 system prompt (zero-arg) + user payload.
	sysPrompt := prompt.BuildArbiterSystemPrompt()
	payload := prompt.BuildArbiterUserPayload(arbiterCommits, leftoverDiff)

	// v3 FR-R5b: the inference provider is the model slash-prefix ("inference/model"),
	// which Render splits into --provider <inference>. P1.M2 wires real per-role reasoning
	// via ResolveRoleModel's 3rd return (rsn).
	spec, rerr := deps.Roles.Arbiter.Render(mdl, sysPrompt, payload, rsn, provider.RenderBare)
	if rerr != nil {
		return prompt.ArbiterOutput{}, fmt.Errorf("%w: render: %w", ErrArbiterFailed, rerr)
	}

	// 5. Execute ONCE (NO retry — §17.7 defines no retry instruction; the arbiter is "when in doubt, null").
	out, _, execErr := provider.Execute(ctx, *spec, arbiterTimeout, deps.Verbose)
	if execErr != nil {
		if errors.Is(execErr, context.DeadlineExceeded) || errors.Is(execErr, context.Canceled) {
			// Timeout / cancel → graceful null (the arbiter OWNS the null decision, §13.6.5).
			return prompt.ArbiterOutput{Target: nil}, nil
		}
		// Non-zero exit — stdout may be partial/valid; fall through to parse (mirrors callPlanner/generate).
	}

	// 6. Parse the arbiter's JSON output. Parse failure → graceful null (NOT an error — "when in doubt, null").
	parsed, perr := prompt.ParseArbiterOutput(out)
	if perr != nil {
		return prompt.ArbiterOutput{Target: nil}, nil
	}

	// 7. Validate target in-list (§13.6.5 "may only target a commit from this run"; empty ⇒ null).
	if parsed.Target != nil && targetInRun(*parsed.Target, validSHAs) {
		return parsed, nil // confident in-list target — S2 amends/rebuilds
	}
	return prompt.ArbiterOutput{Target: nil}, nil // empty / not-in-list → null (ambiguous → new commit)
}

// convertArbiterCommits converts []CommitInfo → []prompt.ArbiterCommit and builds the valid-SHA set
// (the §13.6.5 "may only target a commit from this run" membership check). Each git.FileChange is
// mapped to its .Path (Path is ALWAYS set; Status/SrcPath are NOT part of the arbiter payload —
// ArbiterCommit.Files is "diff-tree --name-only" paths). The valid set is built once so targetInRun
// can check membership in O(1).
func convertArbiterCommits(commits []CommitInfo) ([]prompt.ArbiterCommit, map[string]struct{}) {
	out := make([]prompt.ArbiterCommit, len(commits))
	valid := make(map[string]struct{}, len(commits))
	for i, c := range commits {
		files := make([]string, len(c.Files))
		for j, f := range c.Files {
			files[j] = f.Path // Path ALWAYS set; Status/SrcPath NOT part of the arbiter payload
		}
		out[i] = prompt.ArbiterCommit{SHA: c.SHA, Subject: c.Subject, Files: files}
		valid[c.SHA] = struct{}{}
	}
	return out, valid
}

// targetInRun checks whether target is in the set of valid run-commit SHAs (§13.6.5 "may only
// target a commit from this run"). Exact full-SHA membership (the arbiter is instructed to copy a
// SHA "from the list" verbatim per §17.7). Empty target ⇒ false ⇒ null (safe).
func targetInRun(target string, validSHAs map[string]struct{}) bool {
	if target == "" {
		return false
	}
	_, ok := validSHAs[target]
	return ok
}
