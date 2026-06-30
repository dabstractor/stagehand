// Package stagehand is Stagehand's public library surface (PRD §14.1).
// The entry point is GenerateCommit, which generates (and, unless Options.DryRun, creates) a commit
// from the currently-staged index. The surface is intentionally tiny: an integrator imports this package
// instead of reimplementing the pipeline or shelling out to the CLI.
//
// Stable as of v1.0.
package stagehand

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/prompt"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/signal"
	"github.com/dustin/stagehand/internal/ui"
)

// Options configures a GenerateCommit call. All fields are optional (zero value ⇒ inherit the resolved
// default). This struct is ADDITIVE-ONLY for future versions (Appendix E item 6): new fields may be
// added, existing fields will not be removed or repurposed.
//
// Stable as of v1.0.
type Options struct {
	Provider    string        // manifest name; "" → resolved default (auto-detect installed built-ins)
	Model       string        // "" → manifest default_model
	SystemExtra string        // appended to the built system prompt (extra integrator instructions)
	DryRun      bool          // if true, return the message WITHOUT committing (CommitSHA == "")
	Timeout     time.Duration // per-attempt generation timeout; 0 → config default (120s)
	Verbose     io.Writer     // optional; when set AND cfg.Verbose, diagnostics (resolved command, raw output, retries) are written here (the CLI passes stderr). nil ⇒ silent. Additive-only (PRD §14.1).
	VerboseOn   bool          // when true, forces cfg.Verbose=true (highest precedence — CLI --verbose / library consumer override). Overrides config/env/git-config layers.
	// Config optionally supplies an already-resolved configuration; when non-nil, config.Load is
	// skipped entirely (the caller — typically the in-module CLI — has already loaded config once).
	// Options overrides below still apply on top. nil ⇒ config.Load runs as before (standalone path).
	// Additive-only (PRD §14.1); external out-of-module callers leave this nil.
	Config *config.Config
}

// Result is the outcome of GenerateCommit. On DryRun (or any non-committing outcome) CommitSHA is "".
//
// Stable as of v1.0.
type Result struct {
	CommitSHA string // the published commit SHA; "" if DryRun or not committed
	Subject   string // the commit subject (first line)
	Message   string // the full commit message (subject [+ body])
	Provider  string // the resolved provider name
	Model     string // the resolved model
}

// ---- Typed-error re-exports (so library consumers import only pkg/stagehand) ----
// These ARE the generate-package symbols (alias / same sentinel), so errors.Is / errors.As work
// uniformly whether the error came from the delegation path (CommitStaged) or runPipeline.

var (
	// ErrNothingToCommit is returned when the staged diff is empty (caller should stage first).
	ErrNothingToCommit = generate.ErrNothingToCommit
	// ErrTimeout is returned when generation exceeded the timeout.
	ErrTimeout = generate.ErrTimeout
	// ErrRescue is returned when generation failed after exhausting retries.
	ErrRescue = generate.ErrRescue
	// ErrCASFailed is returned when HEAD moved since the snapshot (non-fast-forward).
	ErrCASFailed = generate.ErrCASFailed
)

// RescueError carries the post-snapshot context for a rescue (see generate.RescueError).
// Returned wrapped for BOTH ErrTimeout and ErrRescue. Type alias — interchangeable with
// generate.RescueError.
type RescueError = generate.RescueError

// CASError carries the "HEAD moved" context (see generate.CASError). Type alias.
type CASError = generate.CASError

// GenerateCommit generates and (unless Options.DryRun) creates a commit from the
// currently-staged index. It does NOT decide what to stage: the caller stages first (or the CLI
// layer uses its auto-stage-all). Repo = the current working directory. Returns a typed error
// (ErrNothingToCommit / ErrTimeout / ErrRescue / ErrCASFailed, or a *RescueError / *CASError) the
// caller maps to their own UX.
//
// Stable as of v1.0.
func GenerateCommit(ctx context.Context, opts Options) (Result, error) {
	cfg, repoDir, err := resolveConfig(ctx, opts)
	if err != nil {
		return Result{}, err
	}
	deps, err := buildDeps(cfg, repoDir)
	if err != nil {
		return Result{}, err
	}
	deps.Verbose = ui.NewVerbose(opts.Verbose, cfg.Verbose)

	// Common path: no DryRun, no SystemExtra → delegate to the frozen, tested orchestrator.
	if !opts.DryRun && opts.SystemExtra == "" {
		res, gerr := generate.CommitStaged(ctx, deps, cfg)
		if gerr != nil {
			return Result{}, gerr
		}
		return Result{
			CommitSHA: res.CommitSHA,
			Subject:   res.Subject,
			Message:   res.Message,
			Provider:  res.Provider,
			Model:     res.Model,
		}, nil // drop res.Changes (design §1)
	}

	// Advanced path: DryRun and/or SystemExtra → self-contained (CommitStaged can't honor these).
	return runPipeline(ctx, deps, cfg, opts.SystemExtra, opts.DryRun)
}

// resolveConfig loads the full 7-layer config and applies Options overrides (highest precedence).
func resolveConfig(ctx context.Context, opts Options) (config.Config, string, error) {
	repoDir, err := os.Getwd()
	if err != nil {
		return config.Config{}, "", fmt.Errorf("getwd: %w", err)
	}

	var cfg config.Config
	if opts.Config != nil {
		cfg = *opts.Config // shallow copy; caller already resolved config — skip config.Load (D1)
	} else {
		cfgPtr, err := config.Load(ctx, config.LoadOpts{RepoDir: repoDir, Flags: nil})
		if err != nil {
			return config.Config{}, "", fmt.Errorf("load config: %w", err)
		}
		cfg = *cfgPtr // copy to value
	}

	// Apply caller overrides (highest precedence — explicit intent wins over file/env/git-config).
	if opts.Provider != "" {
		cfg.Provider = opts.Provider
	}
	if opts.Model != "" {
		cfg.Model = opts.Model
	}
	if opts.Timeout != 0 {
		cfg.Timeout = opts.Timeout
	}
	if opts.VerboseOn {
		cfg.Verbose = true
	}

	return cfg, repoDir, nil
}

// buildDeps resolves the provider manifest from the registry and constructs generate.Deps.
func buildDeps(cfg config.Config, repoDir string) (generate.Deps, error) {
	overrides, err := provider.DecodeUserOverrides(cfg.Providers)
	if err != nil {
		return generate.Deps{}, fmt.Errorf("provider overrides: %w", err)
	}

	reg := provider.NewRegistry(overrides)

	name := cfg.Provider
	if name == "" {
		var installed []string
		for _, m := range reg.List() {
			if reg.IsInstalled(m) {
				installed = append(installed, m.Name)
			}
		}
		name = reg.DefaultProvider(installed)
	}
	if name == "" {
		return generate.Deps{}, fmt.Errorf(
			"no provider configured and none of the built-ins (%s) are installed",
			strings.Join([]string{"pi", "claude", "gemini", "opencode", "codex", "cursor"}, ", "))
	}

	m, ok := reg.Get(name)
	if !ok {
		return generate.Deps{}, fmt.Errorf("unknown provider %q", name)
	}
	if err := m.Validate(); err != nil {
		return generate.Deps{}, fmt.Errorf("provider %q: %w", name, err)
	}

	// Pre-flight (PRD §18.2): fail fast with exit 1 BEFORE the snapshot if the provider command is
	// not on $PATH. Without this, a missing binary is only detected inside Execute's cmd.Start
	// (well after WriteTree), surfacing as a misleading exit-3 rescue with a dangling tree object.
	// reg.IsInstalled reuses the tested exec.LookPath(m.DetectCommand()) seam (registry.go:76).
	if !reg.IsInstalled(m) {
		return generate.Deps{}, fmt.Errorf(
			"provider %q: command %q not found. Is the agent installed?",
			name, m.DetectCommand())
	}

	// Apply [generation] output/strip_code_fence onto the resolved manifest (PRD Issue 4 / §16.2 / §12.9).
	// cfg.Output / cfg.StripCodeFence are populated by every loader (file, git-config) and Defaults, but
	// were previously dropped here — ParseOutput reads ONLY the manifest's pointer fields. Copying them
	// onto the manifest makes the [generation] / git-config values override the per-provider per-manifest
	// values (broader setting wins), which ParseOutput then honors. (decisions.md D4.)
	//
	// Copy into locals (not &cfg.*) to avoid aliasing the cfg value-param's address. Output is guarded
	// (defensive; it is always non-empty post-Defaults); StripCodeFence is applied unconditionally so the
	// broader [generation] layer consistently overrides any per-manifest default. No re-Validate():
	// ParseOutput's switch-default degrades an unknown Output to raw.
	if cfg.Output != "" {
		o := cfg.Output
		m.Output = &o
	}
	if cfg.StripCodeFence != nil {
		m.StripCodeFence = cfg.StripCodeFence
	}

	return generate.Deps{Git: git.New(repoDir), Manifest: m}, nil
}

// buildSysPrompt constructs the system prompt. On unborn or CommitCount≤1 → fallback;
// else → mature with recent messages + multiline detection.
// This mirrors generate.buildSystemPrompt (unexported — can't import). It reuses the prompt
// builders; NOT IP duplication.
func buildSysPrompt(ctx context.Context, g git.Git, cfg config.Config, isUnborn bool) (string, error) {
	if isUnborn {
		return prompt.BuildFallbackPrompt(cfg.SubjectTargetChars), nil
	}
	n, err := g.CommitCount(ctx)
	if err != nil {
		return "", err
	}
	if n <= 1 {
		return prompt.BuildFallbackPrompt(cfg.SubjectTargetChars), nil
	}
	msgs, err := g.RecentMessages(ctx, 20)
	if err != nil {
		return "", err
	}
	return prompt.BuildSystemPrompt(msgs, prompt.DetectMultiline(msgs), cfg.SubjectTargetChars), nil
}

// runPipeline is the self-contained path for DryRun/SystemExtra. It mirrors generate.CommitStaged
// but (a) appends SystemExtra to the system prompt and (b) DryRun runs a single pass without
// committing. It reuses the SAME exported primitives CommitStaged uses (git.Git, prompt.*,
// provider.{Render,Execute,ParseOutput}, generate.{ExtractSubject,IsDuplicate,RescueError,CASError}).
func runPipeline(ctx context.Context, deps generate.Deps, cfg config.Config, systemExtra string, dryRun bool) (Result, error) {
	// Step 1: capture parent + isUnborn.
	parentSHA, isUnborn, err := deps.Git.RevParseHEAD(ctx)
	if err != nil {
		return Result{}, err
	}

	// Step 2: diff payload; empty → nothing to commit.
	diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
		MaxDiffBytes: cfg.MaxDiffBytes,
		MaxMDLines:   cfg.MaxMdLines,
	})
	if err != nil {
		return Result{}, err
	}
	if diff == "" {
		return Result{}, ErrNothingToCommit
	}

	// Step 3: snapshot (FR49 — dry-run runs the full diff→snapshot→… pipeline; the dangling tree in
	// dry-run is intentional and harmless — commit-tree/update-ref are skipped later for dry-run).
	treeSHA, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return Result{}, err
	}
	signal.SetSnapshot(treeSHA, parentSHA, "") // arm rescue (§18.4) — both the commit and dry-run paths

	// Step 4: system prompt (+ SystemExtra) + recent subjects (built ONCE).
	sysPrompt, err := buildSysPrompt(ctx, deps.Git, cfg, isUnborn)
	if err != nil {
		return Result{}, err
	}
	if systemExtra != "" {
		sysPrompt += "\n\n" + systemExtra
	}

	var recent []string
	if !isUnborn {
		recent, err = deps.Git.RecentSubjects(ctx, 50)
		if err != nil {
			return Result{}, err
		}
	}

	resolved := deps.Manifest.Resolve()
	model := cfg.Model
	if model == "" {
		model = *resolved.DefaultModel
	}

	// ---- Generation+dedupe loop (FR29/FR32) — runs for both dry-run and commit paths. ----
	// Mirror generate.CommitStaged step 5. Same loop body for dryRun and !dryRun;
	// on success dry-run returns early (before CommitTree), commit continues to the tail.
	retryInstr := *resolved.RetryInstruction
	var rejected []string
	var candidate, msg string
	var parseFail, success bool
	var lastCause error

	for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
		payload := prompt.BuildUserPayload(diff, rejected)
		if parseFail {
			payload = retryInstr + "\n\n" + payload
		}

		spec, rerr := deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
		if rerr != nil {
			return Result{}, fmt.Errorf("render: %w", rerr)
		}

		out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) {
				return Result{}, &generate.RescueError{
					Kind: generate.ErrTimeout, TreeSHA: treeSHA, ParentSHA: parentSHA,
					Candidate: candidate, Cause: execErr,
				}
			}
			if errors.Is(execErr, context.Canceled) {
				return Result{}, &generate.RescueError{
					Kind: generate.ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
					Candidate: candidate, Cause: execErr,
				}
			}
			// Non-zero exit: fall through to ParseOutput (stdout may be partial-valid).
			lastCause = execErr
		} else {
			lastCause = nil
		}

		m, ok, _ := provider.ParseOutput(out, deps.Manifest)
		if !ok {
			parseFail = true
			candidate = m
			deps.Verbose.VerboseRetry(attempt+1, "parse failed (no valid commit message)")
			continue
		}
		parseFail = false
		signal.SetCandidate(m) // keep the §18.3 candidate note current

		subject := generate.ExtractSubject(m)
		if generate.IsDuplicate(subject, recent) {
			rejected = append(rejected, subject)
			candidate = m
			deps.Verbose.VerboseRetry(attempt+1, fmt.Sprintf("subject %q matches an existing commit", subject))
			continue
		}

		msg = m
		success = true
		break
	}
	if !success {
		return Result{}, &generate.RescueError{
			Kind: generate.ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
			Candidate: candidate, Cause: lastCause,
		}
	}

	// ---- Dry-run success: skip commit-tree/update-ref. ----
	if dryRun {
		signal.ClearSnapshot() // disarm — no rescue on dry-run success
		return Result{
			CommitSHA: "",
			Subject:   generate.ExtractSubject(msg),
			Message:   msg,
			Provider:  deps.Manifest.Name,
			Model:     model,
		}, nil
	}

	// ---- Commit tail (mirror CommitStaged steps 7-8). ----
	var parents []string
	if !isUnborn {
		parents = []string{parentSHA}
	}
	newSHA, err := deps.Git.CommitTree(ctx, treeSHA, parents, msg)
	if err != nil {
		return Result{}, err
	}

	signal.RestoreDefault() // §18.4 step 3: default disposition for the update-ref window
	expectedOld := parentSHA
	if isUnborn {
		expectedOld = strings.Repeat("0", 40)
	}
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
		if errors.Is(err, git.ErrCASFailed) {
			actual, _, _ := deps.Git.RevParseHEAD(ctx)
			return Result{}, &generate.CASError{
				TreeSHA: treeSHA, Expected: parentSHA,
				Actual: actual, Message: msg,
			}
		}
		return Result{}, err
	}

	signal.ClearSnapshot() // belt-and-suspenders disarm on success

	return Result{
		CommitSHA: newSHA,
		Subject:   generate.ExtractSubject(msg),
		Message:   msg,
		Provider:  deps.Manifest.Name,
		Model:     model,
	}, nil
}
