// Package hook holds the runtime for stagehand's prepare-commit-msg hook (PRD §9.20 FR-H4/H5/H6).
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

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/prompt"
	"github.com/dustin/stagehand/internal/provider"
)

// ErrNoOp indicates Run declined to generate (FR-H4): a named message source was present, or the
// staged diff was empty. The caller exits 0 silently — this is the intended no-op, NOT a failure.
var ErrNoOp = errors.New("stagehand: hook no-op (message source present or nothing staged)")

// noOpSources are the prepare-commit-msg sources where a message already exists (architecture §3).
// A plain `git commit` passes NO source (absent) — that is the empty case stagehand fills.
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

	// Step B: capture staged diff.
	diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
		MaxDiffBytes:     cfg.MaxDiffBytes,
		MaxMDLines:       cfg.MaxMdLines,
		BinaryExtensions: cfg.BinaryExtensions,
		Excludes:         deps.Excludes,
	})
	if err != nil {
		return err
	}
	if diff == "" { // FR-H4: empty staged diff → no-op.
		return ErrNoOp
	}

	// Step C: parent / unborn.
	_, isUnborn, err := deps.Git.RevParseHEAD(ctx)
	if err != nil {
		return err
	}

	// Step D: system prompt + recent subjects (stable across attempts).
	sysPrompt, err := hookSystemPrompt(ctx, deps.Git, cfg, isUnborn)
	if err != nil {
		return err
	}
	recent, err := hookRecentSubjects(ctx, deps.Git, isUnborn)
	if err != nil {
		return err
	}

	// Step E: resolve the message role (FR-H6 — exactly like the single-commit path).
	_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
	retryInstr := *deps.Manifest.Resolve().RetryInstruction

	// Step F: generate→parse→dedupe loop (mirrors CommitStaged step 5).
	var rejected []string
	var parseFail bool

	for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
		payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
		if parseFail {
			payload = retryInstr + "\n\n" + payload // FR29 corrective preamble
		}

		spec, rerr := deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)
		if rerr != nil {
			return fmt.Errorf("hook render: %w", rerr)
		}

		out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) {
				return errors.New("stagehand: hook generation timed out") // no retry; never-block
			}
			if errors.Is(execErr, context.Canceled) {
				return errors.New("stagehand: hook generation cancelled")
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

	// Step G: exhaustion after bounded retries.
	return fmt.Errorf("stagehand: hook generation failed after %d retries", cfg.MaxDuplicateRetries)
}
