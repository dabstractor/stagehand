// Package generate implements Stagecoach's commit-generation pipeline (PRD §13).
// CommitStaged is the atomic, snapshot-based orchestrator: it captures the parent
// and a frozen write-tree snapshot, runs a bounded generate→parse→dedupe retry
// loop, builds the commit from the frozen tree via git plumbing, and advances HEAD
// via a single compare-and-swap update-ref (never git commit, never git add).
package generate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/lock"
	"github.com/dustin/stagecoach/internal/prompt"
	"github.com/dustin/stagecoach/internal/provider"
	"github.com/dustin/stagecoach/internal/signal"
	"github.com/dustin/stagecoach/internal/ui"
)

// CommitHookRunner runs the repo's commit hooks around the plumbing commit path (PRD §9.25
// FR-V1/V2/V3/V7). Injected into Deps (NOT called as hooks.RunCommitHooks) to break the
// generate↔hooks import cycle (internal/hooks imports internal/generate for RescueError). The
// CLI (pkg/stagecoach.buildDeps) wires hooks.DefaultRunner; tests inject a stub OR nil (nil ⇒
// hooks skipped — back-compatible with the legacy no-hooks CommitStaged tests). dryRun + verbose
// are INLINED (not a hooks.HookOpts) so generate need not import internal/hooks — zero
// information loss (HookOpts is exactly those two fields). git.Git/config.Config/*ui.Verbose are
// all already imported by this package.
type CommitHookRunner interface {
	RunCommitHooks(ctx context.Context, g git.Git, cfg config.Config, snapshotTree, parentSHA, msg string,
		dryRun bool, verbose *ui.Verbose) (finalTree, finalMsg string, err error)
	RunPostCommit(ctx context.Context, g git.Git, cfg config.Config, dryRun bool, verbose *ui.Verbose) error
	// ReconcileIndex syncs the live index's snapshot-path entries to the committed tree after a
	// permitted pre-commit mutation re-treed (PRD §9.25 FR-V3; report Finding F1). Called by the
	// commit path AFTER update-ref succeeds when finalTree != snapshotTree. Best-effort: a non-nil
	// error is logged at --verbose and NEVER undoes the commit. dryRun is inlined here (mirroring the
	// other two methods) so generate need not import internal/hooks.
	ReconcileIndex(ctx context.Context, g git.Git, snapshotTree, finalTree string, dryRun bool, verbose *ui.Verbose) error
}

// Deps carries the runtime collaborators that vary by environment/test. Injected
// (not resolved inside CommitStaged) so tests can pass a stub Manifest (stubtest.Manifest)
// and the real git.Git, while the CLI resolves the manifest via the registry.
type Deps struct {
	Git      git.Git           // the git boundary (real *gitRunner via git.New(repo) in prod+tests)
	Manifest provider.Manifest // the provider manifest to Render+Execute (stub in tests)
	Verbose  *ui.Verbose       // nil-safe --verbose diagnostics sink (P1.M4.T3.S2); logs retries here + passed to provider.Execute for command/raw-output logging
	Excludes []string          // resolved user exclude pathspecs (from exclude.ResolveExcludePathspecs); nil ⇒ none
	// Progress is an optional callback invoked by hook.Run after no-op gates pass (nil-safe —
	// never called by CommitStaged). runHookExec sets it to emit the "Generating…" line
	// only when generation is about to run.
	Progress func()
	// Hooks runs the repo's commit hooks around the commit (PRD §9.25 FR-V1/V2/V3/V7). Injected
	// (not called as hooks.RunCommitHooks) to break the generate↔hooks import cycle (hooks imports
	// generate for RescueError). nil ⇒ hooks skipped (no-op) — back-compatible with the legacy
	// no-hooks CommitStaged tests (which construct Deps without Hooks). Wired by
	// pkg/stagecoach.buildDeps (hooks.DefaultRunner{}); the dry-run path (runPipeline) and the
	// decompose path (publishCommit) thread it separately. CommitStaged is the !DryRun path, so
	// it always passes dryRun=false.
	Hooks CommitHookRunner
}

// Result is the outcome of a successful CommitStaged. Carries everything the CLI
// (FR42) and the public API wrapper (P1.M3.T5) need to render the success report
// WITHOUT re-querying git. Changes is step 9's DiffTree output (the "what landed"
// file listing); nil/empty for a no-op commit (not an error).
type Result struct {
	CommitSHA string           // NEW_SHA from commit-tree (the published commit; HEAD now points here)
	Subject   string           // ExtractSubject(Message) — for the "[<short-sha>] <subject>" line (FR42)
	Message   string           // the full commit message (subject [+ body]) committed verbatim
	Provider  string           // deps.Manifest.Name — the concrete provider used
	Model     string           // resolved model (cfg.Model or the manifest DefaultModel)
	Changes   []git.FileChange // DiffTree(newSHA, isUnborn) — FR42's file listing
}

// ---- Typed errors (sentinels + context wrappers) ----

// ErrNothingToCommit is returned when the staged diff is empty (nothing meaningful
// for the model). CLI → exit 2 (PRD §15.4). Reached BEFORE the snapshot (step 2)
// — no TREE_SHA. Returned as a bare sentinel.
var ErrNothingToCommit = errors.New("stagecoach: nothing staged to commit")

// ErrTimeout is returned when generation exceeded cfg.Timeout (the agent was
// killed). CLI → exit 124 + FormatRescue (PRD §15.4). Returned wrapped in
// *RescueError{Kind: ErrTimeout}. Reached AFTER the snapshot — TREE_SHA is set.
var ErrTimeout = errors.New("stagecoach: generation timed out")

// ErrRescue is returned when generation failed after exhausting retries (parse-fail
// / duplicate / non-zero exit / ctx cancel). CLI → exit 3 + FormatRescue (PRD
// §15.4). Returned wrapped in *RescueError{Kind: ErrRescue}.
var ErrRescue = errors.New("stagecoach: commit generation failed after retries")

// ErrCASFailed is git.ErrCASFailed re-exported so the CLI imports a single package.
// Returned wrapped in *CASError. Detected via errors.Is(err, generate.ErrCASFailed)
// (== errors.Is(err, git.ErrCASFailed)). CLI → exit 1 + the §13.5 HEAD-moved
// message (CASError.Error()).
var ErrCASFailed = git.ErrCASFailed

// RescueError carries the post-snapshot context for PRD §18.3's rescue message
// (FR43–FR44). Returned for BOTH ErrTimeout and ErrRescue (both render FormatRescue;
// the exit code differs). The CLI does:
//
//	var re *generate.RescueError
//	if errors.As(err, &re) {
//	    print(FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate))
//	    exit = errors.Is(err, generate.ErrTimeout) ? 124 : 3
//	}
type RescueError struct {
	Kind      error  // ErrTimeout or ErrRescue — Unwrap() returns this (enables errors.Is)
	TreeSHA   string // the frozen snapshot (always non-empty — rescue fires only after WriteTree)
	ParentSHA string // "" on a root commit (FormatRescue omits -p)
	Candidate string // the last generated message ("" if none) — FormatRescue appends the candidate note
	Cause     error  // underlying: context.DeadlineExceeded / *exec.ExitError / nil — for verbose/diag
}

func (e *RescueError) Error() string {
	switch e.Kind {
	case ErrTimeout:
		return "stagecoach: generation timed out after the snapshot was taken"
	default:
		return "stagecoach: commit generation failed after retries"
	}
}

func (e *RescueError) Unwrap() error { return e.Kind }

// CASError carries the §13.5 "HEAD moved" context. The orchestrator RE-READS HEAD
// via RevParseHEAD on CAS failure (git.ErrCASFailed docstring, decision D5) to
// obtain Actual, and reads Actual^{tree} into ActualTree. The CLI does:
//
//	var ce *generate.CASError
//	if errors.As(err, &ce) { print(ce.Error()); exit = 1 }
//
// Error() branches on ActualTree == TreeSHA: when the commit that beat us to HEAD carries the
// SAME tree as our frozen snapshot, our staged changes are already committed (the common case:
// a duplicate stagecoach run). It then says so plainly instead of printing a manual commit-tree
// recipe that would create a DUPLICATE commit.
type CASError struct {
	TreeSHA    string // the snapshot tree (for the manual commit-tree recovery command)
	Expected   string // the parentSHA captured at step 1 (the CAS expected-old)
	Actual     string // HEAD re-read after the CAS failed ("" if the re-read itself errored)
	ActualTree string // tree of Actual (Actual^{tree}); "" if unknown. == TreeSHA ⇒ already committed
	Message    string // the generated commit message (for the manual commit-tree -m)
}

func (e *CASError) Error() string {
	// Already-committed fast path: the commit that won the CAS race has the same tree as our
	// frozen snapshot, so our exact staged changes are already landed — a duplicate run. Printing
	// the manual commit-tree recipe here would invite the user to create a DUPLICATE commit.
	if e.TreeSHA != "" && e.ActualTree != "" && e.ActualTree == e.TreeSHA {
		return fmt.Sprintf("HEAD advanced to %s while generating — that commit's tree matches this "+
			"snapshot, so your staged changes are already committed (another stagecoach run landed "+
			"them). Nothing to do.", e.Actual)
	}
	return fmt.Sprintf("HEAD moved from %s to %s while generating; aborting to avoid a non-fast-forward. "+
		"Your generated message was: %s. To commit the snapshot manually: "+
		"git commit-tree -p %s -m %q %s | xargs git update-ref HEAD",
		e.Expected, e.Actual, e.Message, e.Expected, e.Message, e.TreeSHA)
}

func (e *CASError) Unwrap() error { return git.ErrCASFailed }

// CommitStaged is Stagecoach's core pipeline (PRD §13.3 / §9): the synchronous,
// atomic, snapshot-based commit orchestrator. It assumes the index is already in the
// desired state (PRD §11.3); it NEVER calls git add. The commit is built from a
// FROZEN tree (WriteTree) via plumbing (CommitTree → UpdateRefCAS); HEAD is advanced
// by a single compare-and-swap. Any failure before/including UpdateRefCAS leaves the
// repo byte-for-byte unchanged (PRD §18.1).
//
// Pipeline (10 steps, PRD §13.3):
//  1. RevParseHEAD  — capture parent + isUnborn
//  2. System prompt (built ONCE, stable across attempts) — built BEFORE the diff so its worst-case
//     token count can be measured and threaded into opts.PromptReserveTokens (FR3i seam, P1.M4.T1.S2)
//  3. StagedDiff    — diff payload; empty → ErrNothingToCommit (carries PromptReserveTokens)
//  4. WriteTree     — freeze the index into an immutable tree object (SNAPSHOT)
//  5. Recent subjects (fetched ONCE)
//  6. Generate→Parse→Dedupe LOOP (bounded by cfg.MaxDuplicateRetries)
//  8. CommitTree    — build dangling commit from the frozen tree
//  9. UpdateRefCAS  — sole ref mutation; CAS fail → CASError, never force
//
// 10. DiffTree      — "what landed" for the FR42 report
//
// 11. Return Result
func CommitStaged(ctx context.Context, deps Deps, cfg config.Config) (Result, error) {
	// Step 1: capture parent + isUnborn.
	parentSHA, isUnborn, err := deps.Git.RevParseHEAD(ctx)
	if err != nil {
		return Result{}, err
	}

	// Step 2: system prompt (built ONCE, stable across attempts). Built BEFORE StagedDiff so the FR3i
	// prompt-reserve seam (P1.M4.T1.S2) can measure its worst-case token count and thread it into
	// opts.PromptReserveTokens (the field is unread until M4.T3 — behavior-free). buildSystemPrompt
	// needs isUnborn (from RevParseHEAD above). ✓ On the empty-diff path this fetches RecentMessages
	// before the empty-check returns — rare, cheap, accepted (gated upstream by HasStagedChanges).
	sysPrompt, err := buildSystemPrompt(ctx, deps.Git, cfg, isUnborn)
	if err != nil {
		return Result{}, err
	}
	reserve := prompt.MessageReserveTokens(sysPrompt, cfg.MaxDuplicateRetries, cfg.SubjectTargetChars, cfg.Context, git.EstimateTokens)

	// FR3j closed-loop: when token_limit is set, the gate re-measures the ACTUAL assembled prompt
	// (sysPrompt + BuildUserPayload(gatedDiff)) after water-fill truncation and re-trims until it
	// fits. nil when TokenLimit==0 (the gate branch doesn't run; byte-identical legacy path).
	var measureAssembled func(string) int
	if cfg.TokenLimit != 0 {
		measureAssembled = func(gatedDiff string) int {
			return git.EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, cfg.Context, nil))
		}
	}

	// Step 3: diff payload; empty → nothing to commit (design §6). PromptReserveTokens carries the
	// worst-case prompt token count for M4.T2's water-fill / M4.T3's gate (unread until M4.T3).
	// FR-T12 / Issue 4 reconciliation: a sub-floor token_limit makes the one-shot StagedDiff return
	// ErrBelowTokenFloor (the one-shot payload cannot fit, so FR3j's closed loop cannot honor it). When
	// multi-turn is available (MultiTurnFallback + session_mode="append") multi-turn is the CORRECT path:
	// it deliberately ignores token_limit (FR-T12), re-capturing the diff with TokenLimit=0 (which bypasses
	// the floor) for lossless chunked delivery. So instead of failing, we re-capture the UNTRUNCATED diff
	// here (so the snapshot + generation flow can proceed) and arm skipOneShot to bypass the one-shot
	// generation loop and fall straight through to the multi-turn gate below. When multi-turn is NOT
	// available the config is genuinely impossible → propagate the floor error (FR3j's loud failure).
	skipOneShot := false
	diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
		MaxDiffBytes:        cfg.MaxDiffBytes,
		MaxMDLines:          cfg.MaxMdLines,
		BinaryExtensions:    cfg.BinaryExtensions,
		Excludes:            deps.Excludes,
		TokenLimit:          cfg.TokenLimit,
		DiffContext:         cfg.DiffContextValue(),
		PromptReserveTokens: reserve,
		MeasureAssembled:    measureAssembled,
	})
	if err != nil {
		if errors.Is(err, git.ErrBelowTokenFloor) && cfg.TokenLimit > 0 {
			resolved0 := deps.Manifest.Resolve()
			if cfg.MultiTurnFallbackValue() && cfg.WorkDescription == "" &&
				resolved0.SessionMode != nil && *resolved0.SessionMode == "append" {
				// FR-T12: re-capture the UNTRUNCATED diff (TokenLimit=0 bypasses the floor). Multi-turn will
				// chunk/deliver it losslessly; the one-shot loop is skipped (it cannot honor token_limit).
				fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
					MaxDiffBytes:     cfg.MaxDiffBytes,
					MaxMDLines:       cfg.MaxMdLines,
					BinaryExtensions: cfg.BinaryExtensions,
					Excludes:         deps.Excludes,
					TokenLimit:       0, // FR-T12: multi-turn ignores token_limit
					DiffContext:      cfg.DiffContextValue(),
					// No PromptReserveTokens/MeasureAssembled: TokenLimit=0 skips the gate branch entirely.
				})
				if derr != nil {
					return Result{}, derr
				}
				diff = fullDiff
				skipOneShot = true
			} else {
				return Result{}, err // no multi-turn rescue possible → FR3j loud failure
			}
		} else {
			return Result{}, err
		}
	}
	if diff == "" {
		return Result{}, ErrNothingToCommit
	}

	// Step 4: snapshot — freeze the index into an immutable tree object.
	// Fails on unresolved merge conflicts (exit 128). BEFORE generation — not a rescue.
	treeSHA, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return Result{}, err
	}
	// *** SNAPSHOT TAKEN — HEAD & committed content are frozen w.r.t. this run. ***
	signal.SetSnapshot(treeSHA, parentSHA, "") // arm rescue (§18.4)
	lock.SetSnapshot(treeSHA)                  // publish frozen index tree for the FR52 no-op fast path (nil-safe: no-op w/o lock)

	// (F1) snapshotTreeForReconcile is the PRE-hook frozen tree, captured so that AFTER a permitted
	// pre-commit mutation re-trees (committing treeSHA != snapshot), the live index's snapshot-path
	// entries can be reconciled to the committed tree. Defaults to treeSHA so ReconcileIndex is a no-op
	// when no hooks ran or no mutation occurred. See the post-commit reconciliation below.
	snapshotTreeForReconcile := treeSHA

	// Step 5: recent subjects (fetched ONCE; for dedupe — NOT needed for the reserve).
	recent, err := recentSubjects(ctx, deps.Git, isUnborn)
	if err != nil {
		return Result{}, err
	}

	// Step 6: GENERATION+DEDUPE LOOP (design §4 — FR29 + FR32 share one bounded counter).
	resolved := deps.Manifest.Resolve()
	retryInstr := *resolved.RetryInstruction // resolved default: "Output ONLY the commit message…"

	// FR-R3: resolve the message role so --message-model / [role.message] drive Render.
	// No message override ⇒ (cfg.Provider, cfg.Model, cfg.Reasoning) — back-compatible.
	// Provider is discarded (manifest is deps.Manifest, selected upstream by buildDeps; P1.M2.T2.S1).
	_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
	// FR-R7/FR25: resolve the message role's timeout so [role.message].timeout / --message-timeout
	// bound the message agent's one-shot generation (and the multi-turn total budget, FR-T5) instead
	// of the flat cfg.Timeout. With no per-role override ResolveRoleTimeout returns cfg.Timeout
	// (the message role has no built-in) — behavior-preserving by default.
	msgTimeout := config.ResolveRoleTimeout("message", cfg)

	var rejected []string
	var candidate string // last generated message (for RescueError.Candidate)
	var parseFail bool   // previous attempt failed parsing → prepend retryInstr next attempt
	var lastCause error  // last Execute error (for RescueError.Cause)
	var msg string       // the successful message (set on break)
	var payload string   // hoisted: the last-built payload survives the loop for the FR-T1 gate (D1)
	success := false

	// §9.26 FR-W1/FR-W8: work-description mode. When cfg.WorkDescription is non-empty, the message role
	// uses the description-first read-on-demand loop INSTEAD of the default diff-first one-shot/multi-turn
	// paths. FR-W7: it does NOT cascade into multi-turn fallback (§9.24) — a user who wants that runs
	// without --work-description. On a non-append provider, RunWorkDescription's turn-1 RenderMultiTurn
	// yields a cause (its session_mode gate) → rescue (FR-W4: provider support is identical to §9.24).
	// On success the message is deduped (FR-W7: the normal ParseOutput → FinalizeMessage → dedupe path);
	// a duplicate or no-valid-message falls through to the existing rescue (§9.10). When the mode is
	// active, the DEFAULT diff-first loop below is SKIPPED entirely (FR-W1: either/or, not both).
	workDescActive := cfg.WorkDescription != ""
	if workDescActive {
		wdSysPrompt := prompt.BuildWorkDescSystemPrompt(sysPrompt, cfg.WorkDescReadRounds)
		skeleton, serr := deps.Git.StagedNumstatSkeleton(ctx, git.StagedDiffOptions{
			MaxDiffBytes:     cfg.MaxDiffBytes,
			MaxMDLines:       cfg.MaxMdLines,
			BinaryExtensions: cfg.BinaryExtensions,
			Excludes:         deps.Excludes,
			DiffContext:      cfg.DiffContextValue(),
		})
		if serr != nil {
			return Result{}, fmt.Errorf("work-description skeleton: %w", serr)
		}
		wdPayload := prompt.BuildWorkDescPayload(cfg.WorkDescription, cfg.Context, skeleton)
		wdMsg, wdOK, wdCause := RunWorkDescription(ctx, deps, cfg, deps.Manifest,
			wdSysPrompt, wdPayload, skeleton, msgModel, msgReasoning)
		if wdCause != nil {
			// FR-T7 parity: a turn error/timeout/cancel/non-append-provider aborts → rescue.
			lastCause = wdCause
			if errors.Is(wdCause, context.DeadlineExceeded) {
				return Result{}, &RescueError{Kind: ErrTimeout, TreeSHA: treeSHA, ParentSHA: parentSHA, Candidate: wdMsg, Cause: wdCause}
			}
		} else if wdOK {
			// FR-W7: dedupe the result (FinalizeMessage BEFORE dedupe, one-shot parity — D3).
			finalMsg := FinalizeMessage(wdMsg, cfg)
			signal.SetCandidate(finalMsg)
			if !IsDuplicate(ExtractSubject(finalMsg), recent) {
				msg = finalMsg
				success = true
			} else {
				candidate = finalMsg // duplicate → rescue with the finalized candidate
			}
		} else {
			candidate = wdMsg // no valid message after the round cap → rescue (FR-W7)
		}
	}

	for attempt := 0; attempt <= cfg.MaxDuplicateRetries && !success && !workDescActive && !skipOneShot; attempt++ {
		// Build user payload each attempt (rejection list / retry_instruction change).
		payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)
		if parseFail {
			payload = retryInstr + "\n\n" + payload // FR29 corrective preamble
		}

		// v3 FR-R5b: the inference provider is the model slash-prefix ("inference/model"),
		// which Render splits into --provider <inference>. P1.M2 wires real per-role reasoning.
		// (Old: cfg.Provider was the manifest name, NOT the upstream backend — the provider param
		// has been folded into the model slash-prefix; DefaultProvider field removed.)
		spec, rerr := deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)
		if rerr != nil {
			return Result{}, fmt.Errorf("commit staged: render: %w", rerr)
		}

		out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) {
				// §5: immediate rescue, NO retry — agent was killed.
				return Result{}, &RescueError{
					Kind: ErrTimeout, TreeSHA: treeSHA, ParentSHA: parentSHA,
					Candidate: candidate, Cause: execErr,
				}
			}
			if errors.Is(execErr, context.Canceled) {
				return Result{}, &RescueError{
					Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
					Candidate: candidate, Cause: execErr,
				}
			}
			// Non-zero exit (*exec.ExitError): fall through to ParseOutput.
			// stdout may be partial-valid. Record the cause for eventual rescue.
			lastCause = execErr
		} else {
			lastCause = nil
		}

		m, ok, _ := provider.ParseOutput(out, deps.Manifest)
		if !ok {
			parseFail = true
			candidate = m
			deps.Verbose.VerboseRetry(attempt+1, "parse failed (no valid commit message)")
			continue // FR29 retry (consumes an attempt)
		}
		parseFail = false
		m = FinalizeMessage(m, cfg) // §9.19 FR-F8 seam — template BEFORE dedupe (§9.7 judges the final subject)
		signal.SetCandidate(m)      // keep the §18.3 candidate note current

		subject := ExtractSubject(m) // same package — no prefix
		if IsDuplicate(subject, recent) {
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
		// FR-T1 multi-turn fallback trigger gate (PRD §9.24). Multi-turn activates ONLY when one-shot
		// exhausted (already true here at !success — condition a) AND the payload exceeds one chunk (b)
		// AND multi_turn_fallback is enabled (c) AND the resolved manifest declares session_mode="append"
		// (d). If any condition fails, fall through to the existing rescue (byte-identical, FR-T7).
		// Finalize happens BEFORE dedupe (§9.7 judges the final subject; D3).
		//
		// FR-T12 (PRD §9.24): multi-turn deliberately IGNORES token_limit — the whole point is lossless
		// delivery of a payload that exceeded what one request could carry. When token_limit is set
		// (non-zero) it truncated the one-shot `diff`/`payload` above; for the multi-turn path we
		// RE-CAPTURE the diff with TokenLimit=0 and rebuild the payload from the UNTRUNCATED diff, so
		// the feature delivers its headline benefit even when token_limit truncates the one-shot
		// payload below the chunk threshold. When token_limit is unset (0) the re-capture is skipped
		// (the captured payload is already untruncated), keeping the fast path fast.
		if cfg.MultiTurnFallbackValue() && !workDescActive &&
			resolved.SessionMode != nil && *resolved.SessionMode == "append" {

			// FR-T2/FR-T12: mtPayload is ALWAYS rebuilt from the untruncated `diff` via BuildUserPayload
			// (NOT reused from the one-shot `payload`, which may carry the retryInstr corrective preamble
			// from a failed parse — multi-turn has its own priming preamble, FR-T4). When token_limit is
			// set (non-zero), the one-shot `diff` was truncated, so we RE-CAPTURE with TokenLimit=0 below
			// and rebuild from the untruncated fullDiff. When token_limit is unset (0), `diff` is already
			// untruncated, so we rebuild from it directly (avoids a redundant StagedDiff call).
			mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
			if cfg.TokenLimit != 0 {
				fullDiff, derr := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
					MaxDiffBytes:        cfg.MaxDiffBytes,
					MaxMDLines:          cfg.MaxMdLines,
					BinaryExtensions:    cfg.BinaryExtensions,
					Excludes:            deps.Excludes,
					TokenLimit:          0, // FR-T12: multi-turn ignores token_limit
					DiffContext:         cfg.DiffContextValue(),
					PromptReserveTokens: 0, // multi-turn chunking doesn't use the one-shot reserve
				})
				if derr == nil {
					mtPayload = prompt.BuildUserPayload(fullDiff, cfg.Context, rejected)
				}
				// On re-capture error, fall back to the one-shot payload (best-effort: it is still a
				// valid, if possibly-truncated, diff — multi-turn will still attempt delivery with it).
			}

			// Condition (b): the (now-untruncated) payload must exceed one chunk for multi-turn to help.
			if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {
				// FR-T5: surface the turn count + total wall-clock budget (timeout × turns) on the progress
				// line. Deps.Progress is a no-arg callback (can't carry the message) → direct stderr write.
				turns := len(chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1 // N chunks + 1 final turn
				totalMin := int((msgTimeout * time.Duration(turns)).Minutes())
				if totalMin < 1 {
					totalMin = 1
				}
				fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n", turns, cfg.MultiTurnChunkTokens, totalMin)

				// FR-T11: verbose trigger line (per-turn verbose is emitted by provider.Execute inside Run).
				deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")

				// FR-T2/FR-T4: lossless N+1-turn delivery of the UNTRUNCATED payload (FR-T12).
				msg2, ok2, cause := Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)

				if cause == nil && ok2 {
					// Dedupe the multi-turn result. §9.7 judges the FINAL subject → finalize BEFORE dedupe
					// (one-shot parity; avoids the template-duplicate-slip bug — D3).
					finalMsg := FinalizeMessage(msg2, cfg)
					signal.SetCandidate(finalMsg)
					if !IsDuplicate(ExtractSubject(finalMsg), recent) {
						msg = finalMsg
						success = true // multi-turn succeeded → skip the rescue return
					} else {
						// Duplicate → rescue with the finalized candidate (one-shot parity: candidate = m post-finalize).
						candidate = finalMsg
					}
				} else {
					// cause != nil (turn error/timeout) OR ok2==false (final parse empty) → rescue.
					if cause != nil {
						lastCause = cause // the multi-turn failure supersedes one-shot's lastCause
					}
					if msg2 != "" {
						candidate = msg2 // raw parse output (one-shot parse-fail parity: candidate = m raw)
					}
				}
			}
		}
		if !success {
			return Result{}, &RescueError{
				Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
				Candidate: candidate, Cause: lastCause,
			}
		}
	}

	// §9.22 FR-E1: post-dedupe editor gate (EditMessage). AFTER the dedupe loop accepts a message
	// and BEFORE CommitTree. The user's hand-edited message bypasses the re-check (FR-E3 git parity).
	// The template was already applied by FinalizeMessage (pre-dedupe, FR-F8).
	parentTree := git.EmptyTreeSHA
	if !isUnborn {
		if pt, perr := deps.Git.RevParseTree(ctx, "HEAD"); perr == nil {
			parentTree = pt
		}
	}
	nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, parentTree, treeSHA) // best-effort; "" on err
	msg, err = EditMessage(ctx, msg, cfg, EditContext{Git: deps.Git, TreeSHA: treeSHA, NameStatus: nameStatus})
	if err != nil {
		return Result{}, err // ErrEmptyMessage propagates BARE → exitcode.For() → exit 1 (NOT rescue)
	}

	// §9.25 FR-V1/V2/V3/V7: run the repo's pre-commit → prepare-commit-msg → commit-msg hooks
	// scoped to the frozen snapshot, between the finalized message and commit-tree. Injected via
	// Deps.Hooks to break the generate↔hooks import cycle (nil ⇒ no hooks — back-compatible).
	// CommitStaged is the !DryRun path, so dryRun=false. A hook abort returns BEFORE CommitTree
	// (no dangling commit; HEAD + live index untouched) as a *RescueError (FR-V7 → exit 3, byte-
	// identical to a generation failure) or ErrHookSweptConcurrentWork (FR-V3 freeze backstop).
	// signal is still armed here (RestoreDefault runs only before UpdateRefCAS), so a Ctrl-C during
	// a hook rescues with the ORIGINAL snapshot. On success, treeSHA/msg are REASSIGNED so ALL
	// downstream (CommitTree, CASError recovery, Result Subject/Message) use the hook-adjusted
	// values (a permitted pre-commit mutation re-trees; prepare/commit-msg may annotate).
	if deps.Hooks != nil {
		ft, fm, herr := deps.Hooks.RunCommitHooks(ctx, deps.Git, cfg, treeSHA, parentSHA, msg, false, deps.Verbose)
		if herr != nil {
			return Result{}, herr // *RescueError (FR-V7) or ErrHookSweptConcurrentWork (FR-V3)
		}
		// (F1) capture the PRE-hook frozen tree so the post-commit reconciliation (below) can detect a
		// permitted pre-commit re-tree and sync the live index's snapshot-path entries to the committed
		// (post-hook) tree. snapshotTreeForReconcile was initialized to treeSHA above; only reassign it
		// when a hook actually ran (so the no-hooks path stays a no-op reconcile).
		snapshotTreeForReconcile = treeSHA // the PRE-hook tree (treeSHA is reassigned on the next line)
		treeSHA, msg = ft, fm              // hook may have re-treed (permitted mutation) + annotated the msg
	}

	// §9.25 git parity (Issue 4): a prepare-commit-msg / commit-msg hook may have emptied the message
	// file (a rejection / force-re-edit pattern). git aborts "Aborting commit due to empty commit
	// message."; mirror it — return the BARE ErrEmptyMessage (exit 1, NOT a rescue), same as the --edit
	// path above. HEAD + live index are untouched (the abort returns before CommitTree → no update-ref ran).
	if strings.TrimSpace(msg) == "" {
		return Result{}, ErrEmptyMessage
	}

	// Step 7: commit-tree — build the DANGLING commit object from the FROZEN tree.
	var parents []string
	if !isUnborn {
		parents = []string{parentSHA}
	}
	newSHA, err := deps.Git.CommitTree(ctx, treeSHA, parents, msg)
	if err != nil {
		return Result{}, err
	}

	// Step 8: update-ref CAS — the SOLE ref mutation; never force (design §8).
	signal.RestoreDefault() // §18.4 step 3: default disposition for the update-ref window
	expectedOld := parentSHA
	if isUnborn {
		expectedOld = strings.Repeat("0", 40) // all-zeros for root commit CAS
	}
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
		if errors.Is(err, git.ErrCASFailed) {
			// Re-read HEAD for the §13.5 message (D5), and its tree to detect the already-committed
			// fast path (another stagecoach run landed the same snapshot → duplicate-run race).
			actual, _, _ := deps.Git.RevParseHEAD(ctx)
			actualTree := ""
			if actual != "" {
				actualTree, _ = deps.Git.RevParseTree(ctx, actual)
			}
			return Result{}, &CASError{
				TreeSHA: treeSHA, Expected: parentSHA,
				Actual: actual, ActualTree: actualTree, Message: msg,
			}
		}
		return Result{}, err // non-CAS infra error — propagate
	}

	// §9.25 FR-V1: post-commit runs AFTER update-ref succeeded (best-effort; its exit code is
	// DISREGARDED — FR-V7). The commit already landed; RunPostCommit logs a non-zero exit as a
	// --verbose warning and NEVER undoes (git itself disregards post-commit's exit). nil-guarded.
	if deps.Hooks != nil {
		// (F1) FIRST reconcile the live index's snapshot-path entries to the committed tree when a
		// permitted pre-commit mutation re-treed (the commit just landed; HEAD now points at the
		// hook's tree). git-commit parity: a formatter/lint-staged/prettier pre-commit that modifies and
		// re-stages a file must leave `git status` clean and the index holding the hook's blob, so a
		// subsequent plain `git commit` cannot silently re-commit the pre-hook version. Best-effort: a
		// non-nil error is logged at --verbose and NEVER undoes the commit (it already landed).
		if rerr := deps.Hooks.ReconcileIndex(ctx, deps.Git, snapshotTreeForReconcile, treeSHA, false, deps.Verbose); rerr != nil {
			if deps.Verbose != nil {
				deps.Verbose.VerboseWarn(fmt.Sprintf("post-mutation index reconcile failed (commit stands): %v", rerr))
			}
		}
		_ = deps.Hooks.RunPostCommit(ctx, deps.Git, cfg, false, deps.Verbose)
	}

	// Step 9: diff-tree — "what landed" for the FR42 report.
	signal.ClearSnapshot() // belt-and-suspenders disarm on success
	changes, err := deps.Git.DiffTree(ctx, newSHA, isUnborn)
	if err != nil {
		return Result{}, err
	}

	// Step 10: return Result.
	model := msgModel
	if model == "" {
		model = *resolved.DefaultModel
	}
	return Result{
		CommitSHA: newSHA,
		Subject:   ExtractSubject(msg),
		Message:   msg,
		Provider:  deps.Manifest.Name,
		Model:     model,
		Changes:   changes,
	}, nil
}

// buildSystemPrompt constructs the system prompt ONCE before the generation loop.
// On unborn or CommitCount≤1 → fallback (§17.2); else → mature (§17.1) with
// recent messages + multiline detection.
func buildSystemPrompt(ctx context.Context, g git.Git, cfg config.Config, isUnborn bool) (string, error) {
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

// recentSubjects returns recent commit subjects for dedupe checking, or nil on
// an unborn repo (no dup check needed — vacuous).
func recentSubjects(ctx context.Context, g git.Git, isUnborn bool) ([]string, error) {
	if isUnborn {
		return nil, nil
	}
	return g.RecentSubjects(ctx, 50)
}
