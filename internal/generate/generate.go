// Package generate implements the snapshot-based atomic-commit orchestrator —
// stagehand's defensibility core (PRD §13; decisions.md §3, the ★ two-nested-loop
// contract ★). CommitStaged drives the full pipeline:
//
//	diff → snapshot (write-tree) → install signal handler → build prompt →
//	generate (OUTER duplicate-rejection loop wrapping an INNER parse-correction
//	loop) → dedupe → commit-tree → restore signal handler → update-ref CAS,
//	with rescue-render on failure, and the FR42 success print on success.
//
// The pipeline integrates every earlier milestone: M2 (provider Run/Parse), M3
// (git plumbing/diff/log), M4 (prompt builders), M5 (config), and this
// milestone's S2 (firstLine/isDuplicate), S3 (Rescue), and T2.S1
// (installSignalHandler/restoreSignalHandler).
//
// Safety invariants (PRD §18.1): the repo's refs change ONLY at the final
// UpdateRefCAS compare-and-swap; the index is snapshotted by write-tree BEFORE
// generation so it can be re-staged concurrently; on any post-snapshot failure
// the dangling tree is recoverable via Rescue's printed manual command; a
// concurrent HEAD movement aborts the CAS (NEVER --force) and prints the §13.5
// head-moved message.
//
// CommitStaged NEVER calls os.Exit (it returns typed sentinel errors; the CLI
// in P1.M7.T2 maps them via errors.Is to exit codes) and NEVER stages — staging
// (git add) is a CLI-only concern (the v2 seam).
//
// This file is the primary/first file of package generate and OWNS the
// // Package generate doc; the sibling files (dedupe.go/rescue.go/signal.go)
// already defer to it via a plain "package generate" line, mirroring how
// internal/git/git.go owns "// Package git" while internal/git/log.go defers.
package generate

import (
	"context"
	"errors"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/prompt"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/ui"
)

// gitClient is the consumer-side contract over the git operations
// CommitStaged needs. It is defined HERE (in the generate package, the leaf
// integrator) rather than in internal/git so *git.Git satisfies it
// STRUCTURALLY with ZERO edits to the shipped git type beyond the one additive
// DiffTreeNameStatus seam (P1.M6.T1.S1 Task 2) — Go duck-typing makes the
// dependency one-way (generate → git). Because gitClient embeds CommitCount +
// RecentMessages, a gitClient value also satisfies prompt.HistoryReader, so it
// can be passed straight to prompt.FetchExamples.
//
// Defining the interface in the consuming package (not the producer) follows
// the Go proverb "Accept interfaces, return structs" and keeps the test seam
// hermetic: generate_test.go supplies a stubGit that implements gitClient with
// canned returns + a call log, so the two-nested-loop logic is unit-testable
// WITHOUT a real git binary or a real agent.
type gitClient interface {
	// StagedDiff captures the staged change set (FR1-FR5); ("", nil) when
	// nothing is staged → CommitStaged turns that into ErrNothingToCommit.
	StagedDiff(git.DiffSettings) (string, error)
	// RevParseHEAD returns the parent SHA CommitStaged builds on; hasParent
	// is false (and the SHA "") on an unborn repo → root-commit path.
	RevParseHEAD() (sha string, hasParent bool, err error)
	// WriteTree freezes the index into an immutable tree (the snapshot); the
	// post-snapshot window begins once it succeeds.
	WriteTree() (string, error)
	// CommitTree builds a (initially dangling) commit object from the tree
	// (-p omitted when parent==""); touches no ref.
	CommitTree(parent, msg, tree string) (string, error)
	// UpdateRefCAS atomically advances HEAD — the ONLY ref mutation in the
	// pipeline; never --force; the 1-arg no-expected form only for root.
	UpdateRefCAS(ref, newSHA, expected string) error
	// CommitCount is the history gate (newRepo = count<=1) AND a prompt
	// example source; routes unborn → (0, nil).
	CommitCount() (int, error)
	// RecentMessages is the raw example stream for prompt.FetchExamples
	// (makes gitClient satisfy prompt.HistoryReader structurally).
	RecentMessages(n int) (string, error)
	// RecentSubjects is the duplicate-subject set for the OUTER loop.
	RecentSubjects(n int) ([]string, error)
	// DiffTreeNameStatus is the FR42 success-print query.
	DiffTreeNameStatus(sha string) (string, error)
}

// runner is the consumer-side contract over the agent run + parse two-step
// (FR24/§12.9) that the INNER loop drives. It carries BOTH Run and Parse so a
// stub runner fakes both in unit tests while the REAL *provider.Executor
// satisfies it structurally (Run is shipped; the additive Parse seam is added
// by P1.M6.T1.S1 Task 1 because provider.parseOutput is package-private).
type runner interface {
	// Run execs the agent (the provider.Executor owns model/provider default
	// resolution when model/provider==""); returns *TimeoutError on a
	// deadline, *AgentError on a non-zero exit, BARE context.Canceled on a
	// context cancel (the signal path).
	Run(ctx context.Context, m provider.Manifest, model, providerName, sys, payload string) (string, error)
	// Parse runs the §12.9 output-parsing pipeline over the agent stdout and
	// returns (cleaned message, ok); ok is false on an empty/unparseable msg.
	Parse(raw string, m provider.Manifest) (string, bool)
}

// Deps is the single value CommitStaged takes, holding EVERY collaborator so
// the whole pipeline is injectable for hermetic unit tests. *git.Git satisfies
// Git (structurally) and *provider.Executor satisfies Runner (structurally),
// so the production caller wires the real types while tests wire stubs. Output
// is the *ui.Output used for BOTH the failure rescue render AND the FR42
// success print.
type Deps struct {
	// Git is the snapshot/plumbing/diff/log client (gitClient).
	Git gitClient
	// Runner is the agent run+parse two-step (runner).
	Runner runner
	// Manifest is the resolved provider manifest (Command, Env,
	// RetryInstruction, Output, ...).
	Manifest provider.Manifest
	// Config is the resolved stagehand config (Timeout, MaxDuplicateRetries,
	// MaxDiffBytes, MaxMdLines, SubjectTargetChars, Model, Provider, ...).
	Config config.Config
	// Output is the *ui.Output: Resultf→stdout (FR42 success), Progressf→stderr
	// always (rescue + head-moved), Verbosef→stderr gated.
	Output *ui.Output

	// DryRun (PRD FR49) runs the full pipeline (diff→snapshot→generate→
	// parse→dedupe) but short-circuits BEFORE commit-tree/update-ref,
	// returning Result{CommitSHA:""} with the generated message and leaving
	// HEAD unchanged. Additive seam added by P1.M7.T1.S1 for the public
	// stagehand.GenerateCommit wrapper; the zero value (false) is the M6.T1.S1
	// behavior (commit is created) — byte-identical when unset.
	DryRun bool

	// SystemExtra is appended to the built system prompt (after
	// prompt.BuildSystemPrompt) so a caller can inject extra guidance (a
	// project convention, a style nudge) without re-implementing the prompt
	// builders. Additive seam added by P1.M7.T1.S1; the zero value ("") is a
	// no-op — byte-identical to M6.T1.S1 when unset.
	SystemExtra string
}

// Result is the successful-commit return value: the new commit's full SHA, its
// subject (first line of the message), and the full message. CommitStaged
// returns it ONLY on success (after the FR42 success print); failure paths
// return a zero Result and a sentinel error.
type Result struct {
	CommitSHA string // the full 40-hex SHA of the created commit
	Subject   string // firstLine(msg) — the commit subject
	Message   string // the full generated commit message
}

// Sentinel errors CommitStaged RETURNS (it NEVER os.Exit). The CLI (P1.M7.T2)
// maps them via errors.Is to the ui exit codes:
//
//   - ErrNothingToCommit: nothing staged (diff=="") → exit 2
//     (ui.ExitNothingToCommit). NO rescue (no snapshot was taken).
//   - ErrRescue: a snapshot was taken but no commit was created (timeout /
//     agent-error / parse-fail-after-inner / dup-exhaustion / post-snapshot git
//     error / CommitTree error / a signal-driven cancel) → exit 3
//     (ui.ExitRescue). Rescue is rendered BEFORE returning EXCEPT on the
//     signal-cancel path (the signal handler already rendered it).
//   - ErrHeadMoved: UpdateRefCAS failed (HEAD moved concurrently) → exit 1
//     (ui.ExitError). Prints the §13.5 head-moved message + manual recovery;
//     NEVER force-updates and NEVER rescues.
var (
	ErrNothingToCommit = errors.New("nothing staged to commit")
	ErrRescue          = errors.New("commit generation failed; rescue instructions printed")
	ErrHeadMoved       = errors.New("HEAD moved while generating; aborted to avoid non-fast-forward")
)

// instruction is the base user-payload instruction the OUTER loop resets to at
// the top of every duplicate-retry iteration (decisions.md §3). It is the
// "instruction slot" the inner parse-correction retry swaps to the manifest's
// RetryInstruction when parse fails (Ambiguity #2: the diff STAYS and the
// payload is rebuilt via prompt.AssemblePayload).
const instruction = "Generate a commit message for these changes:"

// CommitStaged is the snapshot-based atomic-commit orchestrator (PRD §13;
// decisions.md §3, the ★ two-nested-loop contract ★). It captures the staged
// diff, snapshots the index, installs the post-snapshot signal handler, builds
// the system prompt, runs the two-nested-loop generation (OUTER
// duplicate-rejection wrapping an INNER parse-correction loop), and on a unique
// subject commits via commit-tree + update-ref CAS. It RETURNS typed sentinel
// errors (NEVER os.Exit), NEVER stages (v2 seam), and prints the FR42 success
// block on success (decisions.md §3 governs the success print; this supersedes
// Appendix D's "main.go prints" assignment).
//
// The ordered control flow is the binding spec (decisions.md §3 +
// reference_impl.md §2); see the inline comments for the line-by-line
// rationale. Every collaborator arrives via the single deps value so the whole
// pipeline is stubbable for hermetic unit tests.
func CommitStaged(ctx context.Context, deps Deps) (Result, error) {
	cfg, out := deps.Config, deps.Output

	// (1) Capture the staged diff. Empty ⇒ ErrNothingToCommit (exit 2); there
	// is no snapshot yet, so NO rescue and NO WriteTree.
	diff, err := deps.Git.StagedDiff(git.DiffSettings{
		MaxMdLines:   cfg.MaxMdLines,
		MaxDiffBytes: cfg.MaxDiffBytes,
	})
	if err != nil {
		return Result{}, err
	}
	if diff == "" {
		return Result{}, ErrNothingToCommit
	}

	// (2) Capture the parent SHA (fixed at snapshot time so a later HEAD
	// movement is caught by UpdateRefCAS) and snapshot the index. A root
	// commit (hasParent=false) yields parentSHA="" → CommitTree/UpdateRefCAS
	// omit -p / the expected-old (FR39). WriteTree aborts on an unresolved
	// merge conflict BEFORE generation (FR8); its error is returned DIRECTLY
	// (NOT ErrRescue — no tree was created, so there is nothing to rescue).
	parentSHA, _, err := deps.Git.RevParseHEAD()
	if err != nil {
		return Result{}, err
	}
	treeSHA, err := deps.Git.WriteTree()
	if err != nil {
		return Result{}, err
	}

	// (3) Post-snapshot window begins: a dangling tree exists, so failures
	// from here Rescue. Build a cancellable run context, optionally layered
	// with a deadline (guard >0 so a zero/negative Timeout does not fire
	// immediately), and install the SIGINT/SIGTERM handler AFTER WriteTree
	// (P1.M6.T2.S1). The SAME cancel func is threaded into Run AND the signal
	// handler, so signal-cancel and timeout-deadline collapse onto one
	// ctx.Done() observed by the executor's process-group kill.
	runCtx, cancel := context.WithCancel(ctx)
	if cfg.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(runCtx, cfg.Timeout)
	}
	defer cancel()
	installSignalHandler(cancel, func() {
		if treeSHA != "" { // gate: only rescue once a snapshot exists
			Rescue(out, treeSHA, parentSHA, "") // candidate is "" on the signal path
		}
	})

	// (4) Build the prompt inputs. count<=1 is the newRepo gate (FR39) AND the
	// FetchExamples early-return condition; hasMultiline picks the §17.1
	// multi-line rule. Any error here is post-snapshot → Rescue("") + ErrRescue.
	count, err := deps.Git.CommitCount()
	if err != nil {
		Rescue(out, treeSHA, parentSHA, "")
		return Result{}, ErrRescue
	}
	examples, hasMultiline, err := prompt.FetchExamples(deps.Git, prompt.DefaultExampleCount)
	if err != nil {
		Rescue(out, treeSHA, parentSHA, "")
		return Result{}, ErrRescue
	}
	sys := prompt.BuildSystemPrompt(examples, hasMultiline, count <= 1, cfg.SubjectTargetChars)
	// SystemExtra seam (P1.M7.T1.S1): append caller-supplied guidance after
	// the built system prompt. Empty ⇒ no-op (byte-identical M6.T1.S1).
	if deps.SystemExtra != "" {
		sys += "\n\n" + deps.SystemExtra
	}
	subjects, err := deps.Git.RecentSubjects(50)
	if err != nil {
		Rescue(out, treeSHA, parentSHA, "")
		return Result{}, ErrRescue
	}

	// (5) The two nested loops (decisions.md §3; reference_impl.md §2).
	//
	// OUTER (duplicate-rejection): dupAttempt runs 0..MaxDuplicateRetries
	// INCLUSIVE ⇒ 1 initial + N retries (default 4 total per PRD FR32). It
	// gets a FRESH inner budget every iteration (the inner budget does NOT
	// accumulate across dup-retries). The instruction slot is RESET to the
	// original at the top of each outer iteration.
	//
	// INNER (parse-correction): parseAttempt runs 1..2 ⇒ a FRESH 2-try budget
	// (1 corrective retry). On a parse miss the payload is REBUILT via
	// AssemblePayload — the diff and the growing rejected list STAY; ONLY the
	// instruction slot swaps to the manifest RetryInstruction (Ambiguity #2).
	rejected := []string{}
	var msg string
	var ok, committed bool
outer:
	for dupAttempt := 0; dupAttempt <= cfg.MaxDuplicateRetries; dupAttempt++ {
		instr := instruction // reset the instruction slot fresh each outer iter
		ok = false
		for parseAttempt := 1; parseAttempt <= 2; parseAttempt++ {
			// FR50 verbose: mark every generation attempt so a verbose run can
			// follow the two-nested-loop control flow. Self-gated by Verbosef.
			out.Verbosef("stagehand: generation attempt (duplicate=%d parse=%d)\n", dupAttempt, parseAttempt)
			payload := prompt.AssemblePayload(diff, instr, rejected)
			stdout, runErr := deps.Runner.Run(runCtx, deps.Manifest, cfg.Model, cfg.Provider, sys, payload)
			if runErr != nil {
				// Signal-cancel double-rescue guard (Ambiguity #5): a bare
				// context.Canceled means the signal handler already rendered
				// Rescue + exited(3), so this goroutine returns ErrRescue
				// WITHOUT rendering Rescue a second time. Everything else
				// (timeout/agent-error) renders Rescue("") then returns ErrRescue.
				if errors.Is(runErr, context.Canceled) {
					return Result{}, ErrRescue
				}
				Rescue(out, treeSHA, parentSHA, "")
				return Result{}, ErrRescue
			}
			msg, ok = deps.Runner.Parse(stdout, deps.Manifest)
			if ok {
				break // a parseable message: hand it to the dedupe check
			}
			// FR50 verbose: the inner parse-correction loop is about to rebuild
			// the payload with the RetryInstruction. Self-gated by Verbosef.
			out.Verbosef("stagehand: parse produced no usable message; retrying with correction\n")
			// Parse miss: swap ONLY the instruction slot (the diff STAYS).
			if deps.Manifest.RetryInstruction != "" {
				instr = deps.Manifest.RetryInstruction
			}
		}
		// Inner budget exhausted with no parseable message: post-snapshot
		// failure (candidate="" — no valid msg was produced).
		if !ok {
			Rescue(out, treeSHA, parentSHA, "")
			return Result{}, ErrRescue
		}
		subject := firstLine(msg)
		if !isDuplicate(subject, subjects) {
			committed = true // a unique subject → COMMIT
			break outer
		}
		// FR50 verbose: the OUTER duplicate-rejection loop is about to reject
		// this subject and retry. Self-gated by Verbosef.
		out.Verbosef("stagehand: duplicate subject %q; retrying (duplicate attempt %d)\n", subject, dupAttempt)
		rejected = append(rejected, subject) // feeds the next AssemblePayload
	}

	// Outer budget exhausted with every subject a duplicate: dup-exhaustion.
	// A VALID message (msg) was produced but rejected, so Rescue is rendered
	// with candidate=msg (the only path that passes a non-empty candidate).
	if !committed {
		Rescue(out, treeSHA, parentSHA, msg)
		return Result{}, ErrRescue
	}

	// DryRun short-circuit (PRD FR49, P1.M7.T1.S1): the full pipeline ran
	// (diff→snapshot→generate→parse→dedupe) and produced a UNIQUE message, but
	// we stop BEFORE commit-tree/update-ref. restoreSignalHandler disarms the
	// post-snapshot handler (a late Ctrl-C must NOT be mistaken for a
	// generation failure), the message is printed to stdout (byte-clean for
	// `| tee`), and Result.CommitSHA is "" (no commit/ref mutation). The zero
	// value (DryRun=false) skips this block entirely — byte-identical M6.T1.S1.
	if deps.DryRun {
		restoreSignalHandler()
		out.Resultf("%s\n", msg)
		return Result{CommitSHA: "", Subject: firstLine(msg), Message: msg}, nil
	}

	// (6) COMMIT block. CommitTree builds a (dangling) commit object; its
	// error is post-snapshot → Rescue("") + ErrRescue.
	newSHA, err := deps.Git.CommitTree(parentSHA, msg, treeSHA) // omits -p when parentSHA==""
	if err != nil {
		Rescue(out, treeSHA, parentSHA, "")
		return Result{}, ErrRescue
	}

	// Restore the signal handler IMMEDIATELY BEFORE UpdateRefCAS (Gotcha #1,
	// PRD §18.4 step 3) so a last-instant Ctrl-C is NOT mistaken for a
	// generation failure. restoring AFTER would leave the rescue-armed window
	// open across the ref mutation.
	restoreSignalHandler()

	// (7) The ONLY ref mutation: a compare-and-swap on HEAD. A CAS conflict
	// (HEAD moved concurrently) prints the §13.5 head-moved message + manual
	// recovery and returns ErrHeadMoved — NEVER --force, NEVER Rescue (the
	// commit object exists and is recoverable via the printed command).
	if err := deps.Git.UpdateRefCAS("HEAD", newSHA, parentSHA); err != nil {
		out.Progressf("HEAD moved while generating; aborting to avoid a non-fast-forward.\n")
		out.Progressf("Your generated message was: %s\n", msg)
		parentFlag := ""
		if parentSHA != "" {
			parentFlag = "-p " + parentSHA + " " // omit -p for the root commit
		}
		out.Progressf("To commit the snapshot manually:\n  git commit-tree %s-m %q %s | xargs git update-ref HEAD\n",
			parentFlag, msg, treeSHA)
		return Result{}, ErrHeadMoved
	}

	// (8) FR42 success print (stdout) + Result. CommitStaged OWNS the success
	// print (decisions.md §3; Appendix D's "main.go prints" is superseded).
	// The short SHA is the first 7 hex chars (matches `git rev-parse --short`
	// minimum); the diff-tree name-status lines follow verbatim.
	short := newSHA
	if len(short) > 7 {
		short = short[:7]
	}
	subj := firstLine(msg)
	out.Resultf("[%s] %s\n", short, subj)
	if dt, derr := deps.Git.DiffTreeNameStatus(newSHA); derr == nil {
		out.Resultf("%s", dt) // best-effort: a failed query never fails the commit
	}
	return Result{CommitSHA: newSHA, Subject: subj, Message: msg}, nil
}
