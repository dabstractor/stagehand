// Package generate implements Stagehand's commit-generation pipeline (PRD §13).
// CommitStaged is the atomic, snapshot-based orchestrator: it captures the parent
// and a frozen write-tree snapshot, runs a bounded generate→parse→dedupe retry
// loop, builds the commit from the frozen tree via git plumbing, and advances HEAD
// via a single compare-and-swap update-ref (never git commit, never git add).
package generate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/prompt"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/signal"
	"github.com/dustin/stagehand/internal/ui"
)

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
var ErrNothingToCommit = errors.New("stagehand: nothing staged to commit")

// ErrTimeout is returned when generation exceeded cfg.Timeout (the agent was
// killed). CLI → exit 124 + FormatRescue (PRD §15.4). Returned wrapped in
// *RescueError{Kind: ErrTimeout}. Reached AFTER the snapshot — TREE_SHA is set.
var ErrTimeout = errors.New("stagehand: generation timed out")

// ErrRescue is returned when generation failed after exhausting retries (parse-fail
// / duplicate / non-zero exit / ctx cancel). CLI → exit 3 + FormatRescue (PRD
// §15.4). Returned wrapped in *RescueError{Kind: ErrRescue}.
var ErrRescue = errors.New("stagehand: commit generation failed after retries")

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
		return "stagehand: generation timed out after the snapshot was taken"
	default:
		return "stagehand: commit generation failed after retries"
	}
}

func (e *RescueError) Unwrap() error { return e.Kind }

// CASError carries the §13.5 "HEAD moved" context. The orchestrator RE-READS HEAD
// via RevParseHEAD on CAS failure (git.ErrCASFailed docstring, decision D5) to
// obtain Actual. The CLI does:
//
//	var ce *generate.CASError
//	if errors.As(err, &ce) { print(ce.Error()); exit = 1 }
type CASError struct {
	TreeSHA  string // the snapshot tree (for the manual commit-tree recovery command)
	Expected string // the parentSHA captured at step 1 (the CAS expected-old)
	Actual   string // HEAD re-read after the CAS failed ("" if the re-read itself errored)
	Message  string // the generated commit message (for the manual commit-tree -m)
}

func (e *CASError) Error() string {
	return fmt.Sprintf("HEAD moved from %s to %s while generating; aborting to avoid a non-fast-forward. "+
		"Your generated message was: %s. To commit the snapshot manually: "+
		"git commit-tree -p %s -m %q %s | xargs git update-ref HEAD",
		e.Expected, e.Actual, e.Message, e.Expected, e.Message, e.TreeSHA)
}

func (e *CASError) Unwrap() error { return git.ErrCASFailed }

// CommitStaged is Stagehand's core pipeline (PRD §13.3 / §9): the synchronous,
// atomic, snapshot-based commit orchestrator. It assumes the index is already in the
// desired state (PRD §11.3); it NEVER calls git add. The commit is built from a
// FROZEN tree (WriteTree) via plumbing (CommitTree → UpdateRefCAS); HEAD is advanced
// by a single compare-and-swap. Any failure before/including UpdateRefCAS leaves the
// repo byte-for-byte unchanged (PRD §18.1).
//
// Pipeline (10 steps, PRD §13.3):
//  1. RevParseHEAD  — capture parent + isUnborn
//  2. StagedDiff    — diff payload; empty → ErrNothingToCommit
//  3. WriteTree     — freeze the index into an immutable tree object (SNAPSHOT)
//  4. System prompt + recent subjects (built ONCE, stable across attempts)
//  5. Generate→Parse→Dedupe LOOP (bounded by cfg.MaxDuplicateRetries)
//  7. CommitTree    — build dangling commit from the frozen tree
//  8. UpdateRefCAS  — sole ref mutation; CAS fail → CASError, never force
//  9. DiffTree      — "what landed" for the FR42 report
//
// 10. Return Result
func CommitStaged(ctx context.Context, deps Deps, cfg config.Config) (Result, error) {
	// Step 1: capture parent + isUnborn.
	parentSHA, isUnborn, err := deps.Git.RevParseHEAD(ctx)
	if err != nil {
		return Result{}, err
	}

	// Step 2: diff payload; empty → nothing to commit (design §6).
	diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
		MaxDiffBytes:     cfg.MaxDiffBytes,
		MaxMDLines:       cfg.MaxMdLines,
		BinaryExtensions: cfg.BinaryExtensions,
		Excludes:         deps.Excludes,
	})
	if err != nil {
		return Result{}, err
	}
	if diff == "" {
		return Result{}, ErrNothingToCommit
	}

	// Step 3: snapshot — freeze the index into an immutable tree object.
	// Fails on unresolved merge conflicts (exit 128). BEFORE generation — not a rescue.
	treeSHA, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return Result{}, err
	}
	// *** SNAPSHOT TAKEN — HEAD & committed content are frozen w.r.t. this run. ***
	signal.SetSnapshot(treeSHA, parentSHA, "") // arm rescue (§18.4)

	// Step 4: system prompt (built ONCE) + recent subjects (fetched ONCE).
	sysPrompt, err := buildSystemPrompt(ctx, deps.Git, cfg, isUnborn)
	if err != nil {
		return Result{}, err
	}
	recent, err := recentSubjects(ctx, deps.Git, isUnborn)
	if err != nil {
		return Result{}, err
	}

	// Step 5: GENERATION+DEDUPE LOOP (design §4 — FR29 + FR32 share one bounded counter).
	resolved := deps.Manifest.Resolve()
	retryInstr := *resolved.RetryInstruction // resolved default: "Output ONLY the commit message…"

	// FR-R3: resolve the message role so --message-model / [role.message] drive Render.
	// No message override ⇒ (cfg.Provider, cfg.Model, cfg.Reasoning) — back-compatible.
	// Provider is discarded (manifest is deps.Manifest, selected upstream by buildDeps; P1.M2.T2.S1).
	_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)

	var rejected []string
	var candidate string // last generated message (for RescueError.Candidate)
	var parseFail bool   // previous attempt failed parsing → prepend retryInstr next attempt
	var lastCause error  // last Execute error (for RescueError.Cause)
	var msg string       // the successful message (set on break)
	success := false

	for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
		// Build user payload each attempt (rejection list / retry_instruction change).
		payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
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

		out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
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
		return Result{}, &RescueError{
			Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
			Candidate: candidate, Cause: lastCause,
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
			// Re-read HEAD for the §13.5 message (D5).
			actual, _, _ := deps.Git.RevParseHEAD(ctx)
			return Result{}, &CASError{
				TreeSHA: treeSHA, Expected: parentSHA,
				Actual: actual, Message: msg,
			}
		}
		return Result{}, err // non-CAS infra error — propagate
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
