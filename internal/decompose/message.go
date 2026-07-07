// Package decompose implements the multi-commit decomposition pipeline (PRD §13.6.2): given an
// un-staged working tree, it produces N logically-coherent commits by running a four-agent pipeline
// (planner → stager → message → arbiter) with per-role provider/model resolution.
//
// This file (message.go) implements the message-role generation and the serialized publication
// primitives (PRD §13.6.2 / §13.6.3, §9.14 FR-M6/M7/M8/M12):
//
//   - generateMessage: the BARE message-role generate/dedupe/parse loop over a tree-to-tree concept
//     diff (§13.6.3 invariant 2: message[i] reasons over `git diff tree[i-1] tree[i]`, never
//     index-vs-HEAD). It is "a variant of generate.CommitStaged's loop that takes a diff string
//     instead of calling StagedDiff" — the diff source is TreeDiff(treeA, treeB), the model is
//     derived via ResolveRoleModel("message"), and the manifest is deps.Roles.Message in BARE mode.
//     It runs the SAME bounded generate→parse→dedupe retry loop as CommitStaged (reusing
//     generate.ExtractSubject / generate.IsDuplicate / prompt.BuildUserPayload). It derives the
//     rescue parent + isUnborn via RevParseHEAD internally (safe under overlap: the concurrent
//     stager mutates the INDEX, not HEAD). Returns the message, or a *generate.RescueError on
//     generation failure (timeout/parse-fail/duplicate-exhausted/non-zero-exit/cancel) carrying
//     TreeSHA=treeB + ParentSHA + Candidate for the §18.3 / FR-M12 rescue.
//   - publishCommit: the serialized publication primitive (§13.6.3 / FR-M7): CommitTree(tree,
//     parents, msg) → newSHA, then UpdateRefCAS(HEAD, newSHA, expectedOld). parentSHA is EXPLICIT
//     (the exact CAS expected-old = newSHA[i-1] the orchestrator holds; it does NOT re-read HEAD
//     for the CAS expected-old). Returns newSHA on success or a *generate.CASError (whose .Error()
//     IS the §13.5 "HEAD moved" message) on CAS failure. Root commit (parentSHA="") uses no -p +
//     all-zeros expectedOld.
//
// Both are SIGNAL-FREE synchronous primitives consumed by the orchestrator (P3.M4.T1.S1). No
// concept-iteration loop, no overlap goroutine scheduling, no signal arming (those are P3.M4).
package decompose

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/generate"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/hooks"
	"github.com/dustin/stagecoach/internal/prompt"
	"github.com/dustin/stagecoach/internal/provider"
)

// ErrMessageFailed is the sentinel for message-generation INFRA failures (TreeDiff error,
// RevParseHEAD error, RecentMessages/RecentSubjects error, render error, empty-diff guard).
// Generation failures (timeout/parse/duplicate-exhaustion/non-zero-exit/cancel) return
// *generate.RescueError DIRECTLY (not wrapped) so errors.As(err, &re) works.
var ErrMessageFailed = errors.New("decompose: message generation failed")

// ErrPublicationFailed is the sentinel for publication-step INFRA failures (CommitTree error).
// The CAS failure returns *generate.CASError DIRECTLY (not wrapped) so errors.As(err, &ce) works.
// Non-CAS UpdateRefCAS failures propagate verbatim (git infra; matches CommitStaged).
var ErrPublicationFailed = errors.New("decompose: publication failed")

// generateMessage is the BARE message-role generate/dedupe/parse loop over a tree-to-tree concept
// diff (PRD §13.6.2 / §13.6.3 invariant 2, §9.14 FR-M6/M7/M8/M12). It is "a variant of
// generate.CommitStaged's loop that takes a diff string instead of calling StagedDiff": the diff
// source is TreeDiff(treeA, treeB) (tree-to-tree, never index-vs-HEAD).
//
// Pipeline: TreeDiff(treeA, treeB) → RevParseHEAD (parent+isUnborn) → system prompt + recent
// subjects → ResolveRoleModel("message") → the CommitStaged step-5 loop (Render BARE → Execute
// → ParseOutput → ExtractSubject → IsDuplicate, bounded by MaxDuplicateRetries, FR29
// retry-instruction prepend on parse-fail, FR32 rejection-list append on duplicate) → return msg
// or *generate.RescueError.
//
// The rescue parent + isUnborn are derived INTERNALLY via RevParseHEAD (safe under overlap: the
// concurrent stager mutates the INDEX, not HEAD). Recent subjects are fetched FRESH each call
// (includes just-committed concepts for cross-concept dedupe). It does NOT import or call the
// signal package (SIGNAL-FREE — signal.RestoreDefault is one-shot; loop signal is P3.M4.T1.S2).
func generateMessage(ctx context.Context, deps Deps, treeA, treeB string) (string, error) {
	// 1. Current HEAD (parent for rescue + isUnborn for prompt). Derived BEFORE the diff so the FR3i
	//    prompt-reserve seam (P1.M4.T1.S2) can build the system prompt and measure its worst-case token
	//    count upstream. Safe under overlap: the concurrent stager mutates the INDEX, not HEAD.
	parentSHA, isUnborn, err := deps.Git.RevParseHEAD(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: rev-parse head: %w", ErrMessageFailed, err)
	}

	// 2. System prompt (v1, unchanged — the message role IS the §13.1–§13.5 agent). Built BEFORE the diff
	//    so the FR3i reserve can be measured and threaded into opts.PromptReserveTokens (unread until
	//    M4.T3 — behavior-free). Site 5 is the MESSAGE role (its dedupe loop grows `rejected`) ⇒ use
	//    MessageReserveTokens (with the worst-case rejection block), NOT the planner formula (design §1).
	sysPrompt, err := messageSystemPrompt(ctx, deps.Git, deps.Config, isUnborn)
	if err != nil {
		return "", fmt.Errorf("%w: system prompt: %w", ErrMessageFailed, err)
	}
	reserve := prompt.MessageReserveTokens(sysPrompt, deps.Config.MaxDuplicateRetries, deps.Config.SubjectTargetChars, deps.Config.Context, git.EstimateTokens)

	// FR3j closed-loop: when token_limit is set, the gate re-measures the ACTUAL assembled prompt
	// (sysPrompt + BuildUserPayload(gatedDiff)) after water-fill truncation and re-trims until it
	// fits token_limit. nil when TokenLimit==0 (the gate branch doesn't run; byte-identical legacy path).
	var measureAssembled func(string) int
	if deps.Config.TokenLimit != 0 {
		measureAssembled = func(gatedDiff string) int {
			return git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, deps.Config.Context, nil))
		}
	}

	// 3. Concept diff (§13.6.3 invariant 2 — tree-to-tree, never index-vs-HEAD). PromptReserveTokens
	//    carries the worst-case prompt token count for M4.T2's water-fill / M4.T3's gate.
	diff, err := deps.Git.TreeDiff(ctx, treeA, treeB, git.StagedDiffOptions{
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
		return "", fmt.Errorf("%w: tree diff: %w", ErrMessageFailed, err)
	}
	if diff == "" {
		return "", fmt.Errorf("%w: empty concept diff %s..%s", ErrMessageFailed, treeA, treeB)
	}

	// 4. Recent subjects (FRESH — includes just-committed concepts for cross-concept dedupe; NOT needed
	//    for the reserve).
	recent, err := messageRecentSubjects(ctx, deps.Git, isUnborn)
	if err != nil {
		return "", fmt.Errorf("%w: recent subjects: %w", ErrMessageFailed, err)
	}

	// 5. Derive the <role> model — Deps has no Models field. (Provider is the manifest name; it is NOT
	// passed to Render — v3 FR-R5b folds the inference backend into the model slash-prefix.)
	_, mdl, rsn := config.ResolveRoleModel("message", deps.Config)
	resolved := deps.Roles.Message.Resolve()
	retryInstr := *resolved.RetryInstruction

	// 6. GENERATION+DEDUPE LOOP — a variant of CommitStaged's step-6 loop (diff = concept diff,
	//    not StagedDiff; manifest = deps.Roles.Message; Render BARE; ResolveRoleModel for
	//    provider/model).
	var rejected []string
	var candidate string
	var parseFail bool
	var lastCause error
	var msg string
	success := false

	for attempt := 0; attempt <= deps.Config.MaxDuplicateRetries; attempt++ {
		payload := prompt.BuildUserPayload(diff, deps.Config.Context, rejected)
		if parseFail {
			payload = retryInstr + "\n\n" + payload // FR29 corrective preamble
		}

		// v3 FR-R5b: the inference provider is the model slash-prefix ("inference/model"),
		// which Render splits into --provider <inference>. P1.M2 wires real per-role reasoning
		// via ResolveRoleModel's 3rd return (rsn).
		spec, rerr := deps.Roles.Message.Render(mdl, sysPrompt, payload, rsn, provider.RenderBare)
		if rerr != nil {
			return "", fmt.Errorf("%w: render: %w", ErrMessageFailed, rerr)
		}

		out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) {
				// §5: immediate rescue, NO retry — agent was killed.
				return "", &generate.RescueError{
					Kind: generate.ErrTimeout, TreeSHA: treeB,
					ParentSHA: parentSHA, Candidate: candidate, Cause: execErr,
				}
			}
			if errors.Is(execErr, context.Canceled) {
				return "", &generate.RescueError{
					Kind: generate.ErrRescue, TreeSHA: treeB,
					ParentSHA: parentSHA, Candidate: candidate, Cause: execErr,
				}
			}
			// Non-zero exit (*exec.ExitError): fall through to ParseOutput.
			// stdout may be partial-valid. Record the cause for eventual rescue.
			lastCause = execErr
		} else {
			lastCause = nil
		}

		m, ok, _ := provider.ParseOutput(out, deps.Roles.Message)
		if !ok {
			parseFail = true
			candidate = m
			deps.Verbose.VerboseRetry(attempt+1, "parse failed (no valid commit message)")
			continue // FR29 retry (consumes an attempt)
		}
		parseFail = false
		m = generate.FinalizeMessage(m, deps.Config) // §9.19 FR-F8 seam — before dedupe

		subject := generate.ExtractSubject(m)
		if generate.IsDuplicate(subject, recent) {
			rejected = append(rejected, subject)
			candidate = m
			deps.Verbose.VerboseRetry(attempt+1, fmt.Sprintf("subject %q matches an existing commit", subject))
			continue // FR32 retry (consumes an attempt)
		}

		msg = m
		success = true
		break // SUCCESS — accept the message
	}
	if !success {
		return "", &generate.RescueError{
			Kind: generate.ErrRescue, TreeSHA: treeB,
			ParentSHA: parentSHA, Candidate: candidate, Cause: lastCause,
		}
	}

	// §9.22 FR-E1: post-dedupe editor gate. AFTER the dedupe loop accepts a message and BEFORE
	// the caller publishes. The user's hand-edited message bypasses the re-check (FR-E3 git parity).
	// This site ALSO covers the arbiter N+1 (chain.go resolveNewCommit reuses generateMessage — transitively).
	nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, treeA, treeB) // best-effort; "" on err
	msg, err = generate.EditMessage(ctx, msg, deps.Config, generate.EditContext{Git: deps.Git, TreeSHA: treeB, NameStatus: nameStatus})
	if err != nil {
		return "", err // ErrEmptyMessage → propagates to runLoop's FR-M12 handling
	}

	return msg, nil
}

// publishCommit is the serialized publication primitive (PRD §13.6.3 invariant 3 / §9.14 FR-M7):
// CommitTree(tree, parents, msg) → newSHA, then UpdateRefCAS(HEAD, newSHA, expectedOld).
//
// parentSHA is EXPLICIT (the exact CAS expected-old = newSHA[i-1] the orchestrator holds — it
// does NOT re-read HEAD for the CAS expected-old; the re-read is ONLY for the §13.5 Actual
// after the CAS fails). Root commit (parentSHA="") uses no -p (parents=nil) + all-zeros
// expectedOld (mirrors CommitStaged's isUnborn path).
//
// Returns newSHA on success. On CAS failure (errors.Is(err, git.ErrCASFailed)) it re-reads HEAD
// for the §13.5 message's Actual and returns *generate.CASError (NOT wrapped in
// ErrPublicationFailed — errors.As-able). On CommitTree failure it returns ErrPublicationFailed-
// wrapped error. Non-CAS UpdateRefCAS failures propagate verbatim (git infra; matches
// CommitStaged). SIGNAL-FREE (no signal import).
func publishCommit(ctx context.Context, deps Deps, tree, parentSHA, msg string) (string, error) {
	var parents []string
	if parentSHA != "" {
		parents = []string{parentSHA}
	}
	// Run the repo's commit hooks scoped to THIS concept's tree (PRD §9.25 FR-V1/V3/V7/V8c). The
	// caller passes tree[i]/tStart, so pre-commit sees only that concept's staged subset via the
	// runner's throwaway index (FR-V3/V8c — no caller edit needed). A hook abort returns
	// *generate.RescueError (FR-V7) and propagates to the runLoop's FR-M12 handling. DryRun:false —
	// decompose always commits (the dry-run path is single-commit runPipeline, P1.M3.T2.S2).
	finalTree, finalMsg, herr := hooks.RunCommitHooks(ctx, deps.Git, deps.Config, tree, parentSHA, msg,
		hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
	if herr != nil {
		return "", herr // *generate.RescueError (FR-V7) — propagate DIRECTLY (not wrapped)
	}
	// §9.25 git parity (Issue 4): a prepare-commit-msg / commit-msg hook may have emptied the message file
	// (a rejection / force-re-edit pattern). git aborts "Aborting commit due to empty commit message.";
	// mirror it — return the BARE generate.ErrEmptyMessage (exit 1, NOT a rescue), same as the --edit path
	// (generate.EditMessage), generate.CommitStaged's guard (S1), and runPipeline's guard (S2). The bare
	// error propagates via runLoop's existing hard-error path (it is not *CASError, so it skips FR-M12b).
	// HEAD + live index are untouched (the abort returns before CommitTree → no update-ref ran).
	if strings.TrimSpace(finalMsg) == "" {
		return "", generate.ErrEmptyMessage
	}
	newSHA, err := deps.Git.CommitTree(ctx, finalTree, parents, finalMsg)
	if err != nil {
		return "", fmt.Errorf("%w: commit-tree: %w", ErrPublicationFailed, err)
	}
	expectedOld := parentSHA
	if parentSHA == "" {
		expectedOld = strings.Repeat("0", 40)
	}
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
		if errors.Is(err, git.ErrCASFailed) {
			actual, _, _ := deps.Git.RevParseHEAD(ctx) // re-read for the §13.5 message's Actual (D5)
			actualTree := ""                           // + Actual^{tree} for the already-committed fast path
			if actual != "" {
				actualTree, _ = deps.Git.RevParseTree(ctx, actual)
			}
			return "", &generate.CASError{
				TreeSHA: tree, Expected: expectedOld, Actual: actual, ActualTree: actualTree, Message: msg,
			}
		}
		return "", err // non-CAS infra — propagate verbatim (matches CommitStaged)
	}
	// Best-effort post-commit AFTER update-ref succeeded (FR-V7 — exit disregarded; commit stands).
	// (F1) FIRST reconcile the live index's snapshot-path entries to the committed tree when a permitted
	// pre-commit mutation re-treed (tree != finalTree). git-commit parity for formatter/lint-staged/
	// prettier hooks in a decompose concept commit. Best-effort: a non-nil error is logged at --verbose
	// and NEVER undoes the commit.
	if rerr := hooks.ReconcileIndex(ctx, deps.Git, tree, finalTree, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose}); rerr != nil {
		if deps.Verbose != nil {
			deps.Verbose.VerboseWarn(fmt.Sprintf("post-mutation index reconcile failed (commit stands): %v", rerr))
		}
	}
	_ = hooks.RunPostCommit(ctx, deps.Git, deps.Config, hooks.HookOpts{DryRun: false, Verbose: deps.Verbose})
	return newSHA, nil
}

// messageSystemPrompt constructs the system prompt ONCE before the generation loop.
// Verbatim re-port of generate.buildSystemPrompt (UNEXPORTED in generate — re-ported privately
// to keep decompose self-contained without editing generate.go). On unborn or CommitCount≤1 →
// fallback (§17.2); else → mature (§17.1) with recent messages + multiline detection.
func messageSystemPrompt(ctx context.Context, g git.Git, cfg config.Config, isUnborn bool) (string, error) {
	if isUnborn {
		return prompt.BuildFallbackPrompt(cfg.SubjectTargetChars, cfg.Format, cfg.Locale), nil
	}
	n, err := g.CommitCount(ctx)
	if err != nil {
		return "", err
	}
	if n <= 1 {
		return prompt.BuildFallbackPrompt(cfg.SubjectTargetChars, cfg.Format, cfg.Locale), nil
	}
	msgs, err := g.RecentMessages(ctx, 20)
	if err != nil {
		return "", err
	}
	return prompt.BuildSystemPrompt(msgs, prompt.DetectMultiline(msgs), cfg.SubjectTargetChars, cfg.Format, cfg.Locale), nil
}

// messageRecentSubjects returns recent commit subjects for dedupe checking, or nil on an unborn
// repo (no dup check needed — vacuous). Verbatim re-port of generate.recentSubjects (UNEXPORTED
// in generate — re-ported privately to keep decompose self-contained). Called FRESH each
// generateMessage call (recent subjects grow as concepts publish — intentional; prevents
// cross-concept duplicate subjects; matches CommitStaged).
func messageRecentSubjects(ctx context.Context, g git.Git, isUnborn bool) ([]string, error) {
	if isUnborn {
		return nil, nil
	}
	return g.RecentSubjects(ctx, 50)
}
