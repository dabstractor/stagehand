// Package hook holds the runtime for stagecoach's prepare-commit-msg hook (PRD §9.20 FR-H4/H5/H6).
// Run generates a commit message for the staged diff and writes it at the top of git's message file,
// WITHOUT any commit plumbing (no snapshot/WriteTree, no commit-tree, no update-ref, no rescue/signal:
// git owns this commit). Source-gated: no-op exit 0 when source ∈ {message,template,merge,squash,commit}
// or the staged diff is empty. Never-block: any failure → descriptive error, <msg-file> UNTOUCHED.
package hook

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/generate"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/prompt"
	"github.com/dabstractor/stagecoach/internal/provider"
)

// ErrNoOp indicates Run declined to generate (FR-H4): a named message source was present, or the
// staged diff was empty. The caller exits 0 silently — this is the intended no-op, NOT a failure.
var ErrNoOp = errors.New("stagecoach: hook no-op (message source present or nothing staged)")

// noOpSources are the prepare-commit-msg sources where a message already exists (architecture §3).
// A plain `git commit` passes NO source (absent) — that is the empty case stagecoach fills.
var noOpSources = map[string]struct{}{
	"message":  {},
	"template": {},
	"merge":    {},
	"squash":   {},
	"commit":   {},
}

// NoOpSource reports whether source names a prepare-commit-msg path that already has a message
// (FR-H4). Empty/absent source (plain `git commit`) ⇒ false (proceed).
func NoOpSource(source string) bool {
	_, ok := noOpSources[source]
	return ok
}

// WriteMessageFile prepends msg to git's prepare-commit-msg file at <path>, preserving git's comment
// block beneath it verbatim (FR-H4). The message goes first; a single blank line separates it from the
// original content. git strips `#` comment lines itself on commit. Called ONLY on a fully-accepted msg.
func WriteMessageFile(path, msg string) error {
	orig, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read message file: %w", err)
	}
	var b strings.Builder
	b.WriteString(msg)
	b.WriteByte('\n')
	if len(orig) > 0 {
		b.WriteByte('\n') // exactly one blank separator line
		b.Write(orig)     // git's comment block, VERBATIM
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// hookSystemPrompt constructs the system prompt for the hook generation loop.
// Mirrors generate.buildSystemPrompt (unexported — can't import). Reuses the prompt builders.
func hookSystemPrompt(ctx context.Context, g git.Git, cfg config.Config, isUnborn bool) (string, error) {
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

// hookRecentSubjects returns recent commit subjects for dedupe checking, or nil on
// an unborn repo (no dup check needed — vacuous). Mirrors generate.recentSubjects.
func hookRecentSubjects(ctx context.Context, g git.Git, isUnborn bool) ([]string, error) {
	if isUnborn {
		return nil, nil
	}
	return g.RecentSubjects(ctx, 50)
}

// Run is the source-gated, never-block hook runtime (PRD §9.20 FR-H4/H5/H6). It runs the
// message-generation pipeline (capture → prompt → generate → dedupe) and writes the accepted
// message at the top of <msgFile>, preserving git's comment block. It does NOT perform any commit
// plumbing (no WriteTree/CommitTree/UpdateRefCAS/DiffTree — git owns this commit).
//
// Returns ErrNoOp for intended no-ops (source gate / empty diff). Any generation failure returns a
// descriptive error WITHOUT writing the file (the caller applies the never-block exit-code mapping).
func Run(ctx context.Context, deps generate.Deps, cfg config.Config, msgFile, source string) error {
	// FR-H4: source gate — message/template/merge/squash/commit → a message already exists.
	if NoOpSource(source) {
		return ErrNoOp
	}

	// Step B: parent / unborn — derived BEFORE the diff so the FR3i prompt-reserve seam
	// (P1.M4.T1.S2) can build the system prompt and measure its worst-case token count upstream.
	_, isUnborn, err := deps.Git.RevParseHEAD(ctx)
	if err != nil {
		return err
	}

	// Step C: system prompt (stable across attempts). Built BEFORE StagedDiff so the FR3i reserve can
	// be measured and threaded into opts.PromptReserveTokens (unread until M4.T3 — behavior-free). On
	// the empty-diff path this fetches RecentMessages before the empty-check returns — rare, cheap,
	// accepted (gated upstream by HasStagedChanges).
	sysPrompt, err := hookSystemPrompt(ctx, deps.Git, cfg, isUnborn)
	if err != nil {
		return err
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

	// Step D: capture staged diff. PromptReserveTokens carries the worst-case prompt token count for
	// M4.T2's water-fill / M4.T3's gate (unread until M4.T3).
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
		return err
	}
	if diff == "" { // FR-H4: empty staged diff → no-op.
		return ErrNoOp
	}

	// Progress: generation is about to run (both no-op gates passed). Nil-safe; CommitStaged leaves it nil.
	if deps.Progress != nil {
		deps.Progress()
	}

	// Step E: recent subjects (for dedupe — NOT needed for the reserve).
	recent, err := hookRecentSubjects(ctx, deps.Git, isUnborn)
	if err != nil {
		return err
	}

	// Step F: resolve the message role (FR-H6 — exactly like the single-commit path).
	_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
	// FR-R7/FR-H6: resolve the message role's timeout so [role.message].timeout / --message-timeout bound
	// the hook one-shot generation (and the multi-turn total budget, FR-T5) instead of the flat cfg.Timeout.
	// With no per-role override ResolveRoleTimeout returns cfg.Timeout (no message built-in) — behavior-
	// preserving by default.
	msgTimeout := config.ResolveRoleTimeout("message", cfg)
	resolved := deps.Manifest.Resolve() // bound: the FR-T1 gate (P1.M3.T1.S2) reads resolved.SessionMode
	retryInstr := *resolved.RetryInstruction

	// Step G: generate→parse→dedupe loop (mirrors CommitStaged step 6).
	var rejected []string
	var parseFail bool
	var payload string // hoisted: survives the loop for the FR-T1 multi-turn gate (P1.M3.T1.S2)

	for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
		payload = prompt.BuildUserPayload(diff, cfg.Context, rejected) // `=` (declared above), not `:=`
		if parseFail {
			payload = retryInstr + "\n\n" + payload // FR29 corrective preamble
		}

		spec, rerr := deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)
		if rerr != nil {
			return fmt.Errorf("hook render: %w", rerr)
		}

		out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) {
				return errors.New("stagecoach: hook generation timed out") // no retry; never-block
			}
			if errors.Is(execErr, context.Canceled) {
				return errors.New("stagecoach: hook generation cancelled")
			}
			// Non-zero exit: fall through to ParseOutput (partial stdout may be valid).
		}

		m, ok, _ := provider.ParseOutput(out, deps.Manifest)
		if !ok {
			parseFail = true
			if deps.Verbose != nil {
				deps.Verbose.VerboseRetry(attempt+1, "parse failed (no valid commit message)")
			}
			continue // FR29 retry
		}
		parseFail = false

		m = generate.FinalizeMessage(m, cfg) // §9.19 FR-F8 template, BEFORE dedupe

		subject := generate.ExtractSubject(m)
		if generate.IsDuplicate(subject, recent) {
			rejected = append(rejected, subject)
			if deps.Verbose != nil {
				deps.Verbose.VerboseRetry(attempt+1, fmt.Sprintf("subject %q matches an existing commit", subject))
			}
			continue // FR32 retry
		}

		// SUCCESS — write atop the file, preserve comments.
		return WriteMessageFile(msgFile, m)
	}

	// FR-T1 multi-turn fallback (PRD §9.24). The one-shot loop above exhausted; if the provider is
	// multi-turn-capable (append session mode) and the untruncated payload exceeds one chunk, retry as a
	// lossless N+1-turn session. On success the message is written to the msg-file (the ONLY write site);
	// on ANY failure (turn error, empty final parse, or duplicate subject) fall through to the exhaustion
	// error below — the cmd layer's neverBlock maps that to exit 0 + an untouched msg-file (FR-H5 always).
	// Mirrors the canonical gate in internal/generate/generate.go (CommitStaged), with hook adaptations:
	// generate.ChunkCount (exported), generate.Run (exported), nil-guarded Verbose, WriteMessageFile-on-
	// success / fall-through-on-failure, NO signal/rescue.
	if cfg.MultiTurnFallbackValue() &&
		resolved.SessionMode != nil && *resolved.SessionMode == "append" {

		// FR-T2/FR-T12 (Issue 4): mtPayload is ALWAYS rebuilt from the untruncated `diff` (NOT reused from
		// the one-shot `payload`, which may carry the retryInstr corrective preamble from a failed parse).
		// When token_limit is set (non-zero) the one-shot `diff` was truncated → RE-CAPTURE with TokenLimit=0.
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
			// On re-capture error, fall back to the (possibly-truncated) one-shot diff's payload (best-effort).
		}

		// Condition (b): the (now-untruncated) payload must exceed one chunk for multi-turn to help.
		if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {
			turns := generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) + 1 // N chunks + 1 final turn
			totalMin := int((msgTimeout * time.Duration(turns)).Minutes())
			if totalMin < 1 {
				totalMin = 1
			}
			fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n",
				turns, cfg.MultiTurnChunkTokens, totalMin)

			// FR-T11 verbose trigger line (per-turn verbose is emitted inside generate.Run).
			if deps.Verbose != nil { // hook nil-guard (CommitStaged assumes non-nil; the hook does not)
				deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")
			}

			// FR-T2/FR-T4: lossless N+1-turn delivery of the UNTRUNCATED payload (FR-T12).
			msg2, ok2, cause := generate.Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)

			if cause == nil && ok2 {
				finalMsg := generate.FinalizeMessage(msg2, cfg)
				if !generate.IsDuplicate(generate.ExtractSubject(finalMsg), recent) {
					return WriteMessageFile(msgFile, finalMsg) // SUCCESS — the ONLY write site (FR-H4)
				}
				// Duplicate subject → fall through to exhaustion (FR-H5: exit 0, msg-file untouched).
			}
			// cause != nil (turn error/timeout) OR ok2==false (final parse empty) OR duplicate → fall through.
		}
	}

	// Step G: exhaustion after bounded retries (also the FR-T1 gate's fall-through on any failure).
	return fmt.Errorf("stagecoach: hook generation failed after %d retries", cfg.MaxDuplicateRetries)
}
