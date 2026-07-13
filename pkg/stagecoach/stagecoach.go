// Package stagecoach is Stagecoach's public library surface (PRD §14.1).
// The entry point is GenerateCommit, which generates (and, unless Options.DryRun, creates) a commit
// from the currently-staged index. The surface is intentionally tiny: an integrator imports this package
// instead of reimplementing the pipeline or shelling out to the CLI.
//
// Stable as of v1.0.
package stagecoach

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/decompose"
	"github.com/dabstractor/stagecoach/internal/exclude"
	"github.com/dabstractor/stagecoach/internal/generate"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/hooks"
	"github.com/dabstractor/stagecoach/internal/prompt"
	"github.com/dabstractor/stagecoach/internal/provider"
	"github.com/dabstractor/stagecoach/internal/signal"
	"github.com/dabstractor/stagecoach/internal/ui"
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
	// WorkDescription activates work-description mode for the message role (PRD §9.26 FR-W1): a non-empty
	// value leads the prompt with this description + the numstat skeleton and lets the model pull staged
	// file diffs on demand via a `READ <path>` text protocol (description-first, read-on-demand). Empty ⇒
	// the default diff-first path (§9.5) runs unchanged. Message role only; never the default. Library
	// callers set this instead of --work-description/--work-description-file. Additive-only (PRD §14.1).
	WorkDescription string
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

// RoleModel is a per-role provider/model/reasoning override for DecomposeOptions (PRD §14.1, §16.4, FR-R1–R5,
// FR-R6). A zero value ⇒ the role inherits the global default (FR-R2); a non-empty field overrides just that
// field (FR-R3 field-merge). Models are provider-specific (FR-R5). Reasoning is off|low|medium|high (FR-R6);
// "" ⇒ inherit the global [defaults].reasoning (off by default for every role).
//
// Stable as of v2.0 (additive-only per §14.1).
type RoleModel struct {
	Provider  string
	Model     string
	Reasoning string // off|low|medium|high (FR-R6); "" ⇒ inherit the global [defaults].reasoning (off by default)
}

// DecomposeOptions configures the multi-commit pipeline (PRD §14.1, §13.6). The embedded Options
// (Provider/Model/DryRun/Timeout/Verbose/VerboseOn/Config) apply to the MESSAGE role. Count 0 ⇒
// auto-decompose (planner decides); >0 ⇒ force exactly Count commits. Single true ⇒ bypass the planner
// (delegate to GenerateCommit, the v1 single-commit path). MaxCommits 0 ⇒ the config default (12);
// >0 ⇒ override the safety cap. Planner/Stager/Arbiter are per-role overrides (zero ⇒ global default);
// each RoleModel carries Provider, Model, and Reasoning (FR-R6).
//
// Stable as of v2.0.
type DecomposeOptions struct {
	Options              // embedded: Provider/Model/DryRun/Timeout/… apply to the MESSAGE role
	Count      int       // 0 ⇒ auto-decompose (planner decides); >0 ⇒ force exactly Count commits
	Single     bool      // true ⇒ bypass planner, force one GenerateCommit (--single)
	MaxCommits int       // 0 ⇒ config default (12); >0 ⇒ override the auto-decompose safety cap
	Planner    RoleModel // planner role provider/model (zero ⇒ global default)
	Stager     RoleModel // stager role provider/model (zero ⇒ global default)
	Arbiter    RoleModel // arbiter role provider/model (zero ⇒ global default)
}

// DecomposeResult is the outcome of Decompose: the ordered commits created this run (PRD §14.1).
// Commits is one Result per concept that produced a commit (empty concepts skipped). Amended is the
// number of commits the arbiter folded leftovers into (0 if the arbiter did not run or made a new commit).
// Provider is the resolved MESSAGE-role provider (for display).
//
// Stable as of v2.0.
type DecomposeResult struct {
	Commits  []Result // one per concept that produced a commit (empty concepts skipped)
	Amended  int      // number of commits the arbiter folded leftovers into
	Provider string   // resolved MESSAGE provider (for display)
}

// ---- Typed-error re-exports (so library consumers import only pkg/stagecoach) ----
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
	deps.Excludes, err = exclude.ResolveExcludePathspecs(cfg, repoDir, deps.Verbose)
	if err != nil {
		return Result{}, fmt.Errorf("resolve excludes: %w", err)
	}

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

// Decompose turns a dirty, un-staged working tree into N logically-coherent commits (PRD §14.1, §13.6).
// It is a NO-OP that delegates to GenerateCommit when opts.Single is true or opts.Count == 1; otherwise it
// activates the planner→stager→message→arbiter pipeline (via internal/decompose.Decompose).
// PRECONDITION (FR-M1): the caller must ensure NOTHING is staged — Decompose does NOT re-check
// HasStagedChanges (the CLI gates on it). Options.DryRun is honored ONLY on the single-delegation path
// (the multi-commit pipeline always commits). On a per-concept failure (FR-M12) the already-landed commits
// are returned alongside a *RescueError / *CASError (the same typed errors GenerateCommit returns).
//
// Stable as of v2.0.
func Decompose(ctx context.Context, opts DecomposeOptions) (DecomposeResult, error) {
	// (1) NO-OP delegation (PRD §14.1): Single or Count==1 → GenerateCommit.
	// Honors opts.DryRun (the single path is dry-run aware).
	if opts.Single || opts.Count == 1 {
		r, err := GenerateCommit(ctx, opts.Options)
		if err != nil {
			return DecomposeResult{}, err
		}
		return DecomposeResult{Commits: []Result{r}, Amended: 0, Provider: r.Provider}, nil
	}

	// (2) Multi-commit path: resolve config → ResolveRoles → build Deps → internal Decompose → map.
	cfg, repoDir, err := resolveDecomposeConfig(ctx, opts)
	if err != nil {
		return DecomposeResult{}, err
	}
	overrides, err := provider.DecodeUserOverrides(cfg.Providers)
	if err != nil {
		return DecomposeResult{}, fmt.Errorf("decompose: provider overrides: %w", err)
	}
	reg := provider.NewRegistry(overrides)
	roleManifests, roleModels, err := decompose.ResolveRoles(cfg, reg)
	if err != nil {
		return DecomposeResult{}, fmt.Errorf("decompose: %w", err)
	}
	deps := decompose.Deps{
		Git:      git.New(repoDir),
		Registry: reg,
		Config:   cfg,
		Roles:    roleManifests,
		Verbose:  ui.NewVerbose(opts.Verbose, cfg.Verbose),
		Out:      opts.Verbose, // nil-safe rescue/CAS sink (G-DEPS-OUT-SINK)
	}
	deps.Excludes, err = exclude.ResolveExcludePathspecs(cfg, repoDir, deps.Verbose)
	if err != nil {
		return DecomposeResult{}, fmt.Errorf("resolve excludes: %w", err)
	}
	ires, derr := decompose.Decompose(ctx, deps)
	return mapDecomposeResult(ires, roleModels), derr // partial + error on FR-M12
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
	// §9.26 FR-W1 — work-description mode (library caller override; mirrors the CLI flag).
	if opts.WorkDescription != "" {
		cfg.WorkDescription = opts.WorkDescription
	}

	return cfg, repoDir, nil
}

// resolveDecomposeConfig loads the full 7-layer config (reusing resolveConfig) and layers the
// decompose-specific overrides (Count/Single/MaxCommits/per-role) on top.
func resolveDecomposeConfig(ctx context.Context, opts DecomposeOptions) (config.Config, string, error) {
	cfg, repoDir, err := resolveConfig(ctx, opts.Options)
	if err != nil {
		return config.Config{}, "", err
	}

	// Decompose-specific overrides (highest precedence — explicit intent wins over file/env/git-config).
	// G-COUNT-MAXCOMMITS-ZERO-IS-INHERIT: 0 means "inherit the config value" (don't clobber).
	if opts.Count != 0 {
		cfg.Commits = opts.Count
	}
	if opts.Single {
		cfg.Single = true // already short-circuited above, but set for consistency
	}
	if opts.MaxCommits != 0 {
		cfg.MaxCommits = opts.MaxCommits
	}

	// Per-role field-merge (G-ROLE-FIELD-MERGE): zero RoleModel ⇒ inherit global (FR-R2).
	if opts.Planner.Provider != "" || opts.Planner.Model != "" || opts.Planner.Reasoning != "" ||
		opts.Stager.Provider != "" || opts.Stager.Model != "" || opts.Stager.Reasoning != "" ||
		opts.Arbiter.Provider != "" || opts.Arbiter.Model != "" || opts.Arbiter.Reasoning != "" {
		if cfg.Roles == nil {
			cfg.Roles = map[string]config.RoleConfig{}
		}
		applyRoleOverride(cfg.Roles, "planner", opts.Planner)
		applyRoleOverride(cfg.Roles, "stager", opts.Stager)
		applyRoleOverride(cfg.Roles, "arbiter", opts.Arbiter)
	}

	return cfg, repoDir, nil
}

// applyRoleOverride applies a non-zero RoleModel onto cfg.Roles[role] (FR-R3 field-merge).
func applyRoleOverride(roles map[string]config.RoleConfig, role string, rm RoleModel) {
	if rm.Provider == "" && rm.Model == "" && rm.Reasoning == "" {
		return
	}
	rc := roles[role] // copy (zero value if absent)
	if rm.Provider != "" {
		rc.Provider = rm.Provider
	}
	if rm.Model != "" {
		rc.Model = rm.Model
	}
	if rm.Reasoning != "" {
		rc.Reasoning = rm.Reasoning
	}
	roles[role] = rc
}

// mapDecomposeResult converts the internal decompose.DecomposeResult to the public DecomposeResult,
// injecting Provider from roleModels.Message (the internal result has no Provider field).
func mapDecomposeResult(ires decompose.DecomposeResult, roleModels decompose.RoleModels) DecomposeResult {
	commits := make([]Result, len(ires.Commits))
	for i, c := range ires.Commits {
		commits[i] = Result{
			CommitSHA: c.SHA,
			Subject:   c.Subject,
			Message:   c.Message,
			Provider:  roleModels.Message.Provider,
			Model:     roleModels.Message.Model,
		}
	}
	return DecomposeResult{
		Commits:  commits,
		Amended:  ires.Amended,
		Provider: roleModels.Message.Provider,
	}
}

// buildDeps resolves the provider manifest from the registry and constructs generate.Deps.
func buildDeps(cfg config.Config, repoDir string) (generate.Deps, error) {
	overrides, err := provider.DecodeUserOverrides(cfg.Providers)
	if err != nil {
		return generate.Deps{}, fmt.Errorf("provider overrides: %w", err)
	}

	reg := provider.NewRegistry(overrides)

	// FR-R3: resolve the message role's provider so --message-provider / [role.message]
	// selects the manifest. No message override ⇒ cfg.Provider (back-compatible).
	// Model/reasoning are resolved at the Render call sites (CommitStaged / runPipeline);
	// buildDeps selects only the provider/manifest.
	msgProvider, _, _ := config.ResolveRoleModel("message", cfg)
	name := msgProvider
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
			strings.Join([]string{"pi", "claude", "opencode", "codex", "cursor"}, ", "))
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

	// Apply [generation] / git-config output/strip_code_fence onto the resolved manifest ONLY when the
	// user explicitly set them (PRD bugfix-002 Issue 2 / §16.2 / §12.9). After S1, cfg.Output/cfg.StripCodeFence
	// are *string/*bool and nil unless a file or git-config layer provided them. nil ⇒ leave the manifest's
	// own value intact (the per-provider [provider.X] setting wins, or Resolve() supplies the §12.1 raw/true
	// fallback); non-nil ⇒ override the manifest. This makes [generation] a true OPT-IN override and keeps
	// `providers show` truthful (it displays the registry manifest, which parsing now matches). ParseOutput's
	// switch-default degrades an unknown Output to raw, so no re-Validate() is needed.
	if cfg.Output != nil {
		m.Output = cfg.Output
	}
	if cfg.StripCodeFence != nil {
		m.StripCodeFence = cfg.StripCodeFence
	}

	return generate.Deps{Git: git.New(repoDir), Manifest: m, Hooks: hooks.DefaultRunner{}}, nil
}

// buildSysPrompt constructs the system prompt. On unborn or CommitCount≤1 → fallback;
// else → mature with recent messages + multiline detection.
// This mirrors generate.buildSystemPrompt (unexported — can't import). It reuses the prompt
// builders; NOT IP duplication.
func buildSysPrompt(ctx context.Context, g git.Git, cfg config.Config, isUnborn bool) (string, error) {
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

	// Step 2: system prompt (+ SystemExtra) — built BEFORE the diff so the FR3i prompt-reserve seam
	// (P1.M4.T1.S2) can measure its worst-case token count and thread it into opts.PromptReserveTokens.
	// The reserve is measured AFTER the SystemExtra append (design §9) so SystemExtra is included.
	sysPrompt, err := buildSysPrompt(ctx, deps.Git, cfg, isUnborn)
	if err != nil {
		return Result{}, err
	}
	if systemExtra != "" {
		sysPrompt += "\n\n" + systemExtra
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

	// Step 3: diff payload; empty → nothing to commit. PromptReserveTokens carries the worst-case prompt
	// token count for M4.T2's water-fill / M4.T3's gate (unread until M4.T3).
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
		return Result{}, err
	}
	if diff == "" {
		return Result{}, ErrNothingToCommit
	}

	// Step 4: snapshot (FR49 — dry-run runs the full diff→snapshot→… pipeline; the dangling tree in
	// dry-run is intentional and harmless — commit-tree/update-ref are skipped later for dry-run).
	treeSHA, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return Result{}, err
	}
	signal.SetSnapshot(treeSHA, parentSHA, "") // arm rescue (§18.4) — both the commit and dry-run paths

	// (F1) snapshotTreeForReconcile is the PRE-hook frozen tree, captured so that AFTER a permitted
	// pre-commit mutation re-trees (committing treeSHA != snapshot), the live index's snapshot-path
	// entries can be reconciled to the committed tree. Defaults to treeSHA so ReconcileIndex is a no-op
	// when no hooks ran or no mutation occurred. See the post-commit reconciliation below.
	snapshotTreeForReconcile := treeSHA

	// Step 5: recent subjects (built ONCE; for dedupe — NOT needed for the reserve).

	var recent []string
	if !isUnborn {
		recent, err = deps.Git.RecentSubjects(ctx, 50)
		if err != nil {
			return Result{}, err
		}
	}

	resolved := deps.Manifest.Resolve()
	// FR-R3: resolve the message role so --message-model / [role.message] drive Render.
	// No message override ⇒ (cfg.Provider, cfg.Model, cfg.Reasoning) — back-compatible.
	// Provider is discarded (manifest is deps.Manifest, selected upstream by buildDeps; P1.M2.T2.S1).
	_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
	model := msgModel
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
	var payload string // hoisted: survives the loop for the FR-T1 multi-turn gate (mirrors CommitStaged)

	// §9.26 FR-W1/FR-W8: work-description mode (dry-run path). When cfg.WorkDescription is non-empty,
	// the message role uses the description-first read-on-demand loop instead of the diff-first loop.
	// On success the message is deduped (FR-W7); duplicate/no-valid-message → rescue (byte-identical to
	// the commit path). FR-W7: does NOT cascade into multi-turn fallback. When active, the DEFAULT loop
	// below is SKIPPED entirely (FR-W1: either/or).
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
		wdMsg, wdOK, wdCause := generate.RunWorkDescription(ctx, deps, cfg, deps.Manifest,
			wdSysPrompt, wdPayload, skeleton, msgModel, msgReasoning)
		if wdCause != nil {
			if errors.Is(wdCause, context.DeadlineExceeded) {
				return Result{}, &generate.RescueError{Kind: generate.ErrTimeout, TreeSHA: treeSHA, ParentSHA: parentSHA, Candidate: wdMsg, Cause: wdCause}
			}
			lastCause = wdCause
		} else if wdOK {
			finalMsg := generate.FinalizeMessage(wdMsg, cfg)
			signal.SetCandidate(finalMsg)
			if !generate.IsDuplicate(generate.ExtractSubject(finalMsg), recent) {
				msg = finalMsg
				success = true
			} else {
				candidate = finalMsg
			}
		} else {
			candidate = wdMsg
		}
	}

	for attempt := 0; attempt <= cfg.MaxDuplicateRetries && !success && !workDescActive; attempt++ {
		payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)
		if parseFail {
			payload = retryInstr + "\n\n" + payload
		}

		spec, rerr := deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)
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
		m = generate.FinalizeMessage(m, cfg) // §9.19 FR-F8 seam — template BEFORE dedupe (§9.7 judges the final subject)
		signal.SetCandidate(m)               // keep the §18.3 candidate note current

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
		// ---- FR-T1 multi-turn fallback trigger gate (PRD §9.24) — ported from CommitStaged (generate.go:290-374). ----
		// Multi-turn activates ONLY when one-shot exhausted (already true here at !success — condition a)
		// AND multi_turn_fallback is enabled (c) AND the resolved manifest declares session_mode="append"
		// (d). Condition (b) (payload exceeds one chunk) is evaluated below. If any condition fails, fall
		// through to the existing byte-identical rescue (FR-T7).
		//
		// FR-T12 (PRD §9.24): multi-turn deliberately IGNORES token_limit — the whole point is lossless
		// delivery of a payload that exceeded what one request could carry. When token_limit is set
		// (non-zero) it truncated the one-shot `diff`/`payload` above; for the multi-turn path we
		// RE-CAPTURE the diff with TokenLimit=0 and rebuild the payload from the UNTRUNCATED diff.
		if cfg.MultiTurnFallbackValue() && !workDescActive &&
			resolved.SessionMode != nil && *resolved.SessionMode == "append" {

			// FR-T2/Issue4: mtPayload is ALWAYS rebuilt from the untruncated `diff` via BuildUserPayload
			// (NOT reused from the one-shot `payload`, which may carry the retryInstr corrective preamble
			// from a failed parse — multi-turn has its own priming preamble, FR-T4). When token_limit is
			// set (non-zero), the one-shot `diff` was truncated, so we RE-CAPTURE with TokenLimit=0 below
			// and rebuild from the untruncated fullDiff. When token_limit is unset (0), `diff` is already
			// untruncated, so we rebuild from it directly (avoids a redundant StagedDiff call).
			mtPayload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
			if cfg.TokenLimit != 0 { // FR-T12: re-capture with TokenLimit=0
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
				// On re-capture error, fall back to the one-shot payload (best-effort).
			}

			// Condition (b): the (now-untruncated) payload must exceed one chunk for multi-turn to help.
			if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {
				// FR-T5: surface the turn count + total wall-clock budget (timeout × turns) on the progress
				// line. Deps.Progress is a no-arg callback (can't carry the message) → direct stderr write.
				turns := generate.ChunkCount(mtPayload, cfg.MultiTurnChunkTokens) + 1 // N chunks + 1 final turn
				totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
				if totalMin < 1 {
					totalMin = 1
				}
				fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n",
					turns, cfg.MultiTurnChunkTokens, totalMin) // Issue 3 format (per-chunk token estimate)

				// FR-T11: verbose trigger line (per-turn verbose is emitted by provider.Execute inside Run).
				deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")

				// FR-T2/FR-T4: lossless N+1-turn delivery of the UNTRUNCATED payload (FR-T12).
				msg2, ok2, cause := generate.Run(ctx, deps, cfg, deps.Manifest, sysPrompt, mtPayload, msgModel, msgReasoning)

				if cause == nil && ok2 {
					// Dedupe the multi-turn result. §9.7 judges the FINAL subject → finalize BEFORE dedupe
					// (one-shot parity; avoids the template-duplicate-slip bug — D3).
					finalMsg := generate.FinalizeMessage(msg2, cfg)
					signal.SetCandidate(finalMsg)
					if !generate.IsDuplicate(generate.ExtractSubject(finalMsg), recent) {
						msg = finalMsg
						success = true // multi-turn won → skip the rescue return
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
			return Result{}, &generate.RescueError{
				Kind: generate.ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
				Candidate: candidate, Cause: lastCause,
			}
		}
	}

	// §9.22 FR-E1: post-dedupe editor gate (EditMessage). AFTER the dedupe loop accepts a message
	// and BEFORE CommitTree. The user's hand-edited message bypasses the re-check (FR-E3 git parity).
	parentTree := git.EmptyTreeSHA
	if !isUnborn {
		if pt, perr := deps.Git.RevParseTree(ctx, "HEAD"); perr == nil {
			parentTree = pt
		}
	}
	nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, parentTree, treeSHA) // best-effort; "" on err
	msg, err = generate.EditMessage(ctx, msg, cfg, generate.EditContext{Git: deps.Git, TreeSHA: treeSHA, NameStatus: nameStatus})
	if err != nil {
		return Result{}, err // ErrEmptyMessage propagates BARE → exitcode.For() → exit 1
	}

	// ---- Commit hooks (PRD §9.25 FR-V1/V2/V8a). Threaded between EditMessage and the commit/dry-run split
	// so BOTH the dry-run and the SystemExtra-commit paths run hooks. Under --dry-run the runner skips
	// pre-commit + self-skips post-commit, but runs prepare-commit-msg + commit-msg on the would-be message
	// (FR-V8a: the user still sees lint results). A commit-msg REJECTION under --dry-run is warn-and-print
	// (the would-be message + a stderr notice, exit 0) — a dry-run is a preview; a lint rejection is
	// information, not a failure. On the commit path (!dryRun) a hook abort is the FR-V7 rescue (exit 3,
	// mirroring S1's CommitStaged). ----
	if deps.Hooks != nil {
		ft, fm, herr := deps.Hooks.RunCommitHooks(ctx, deps.Git, cfg, treeSHA, parentSHA, msg, dryRun, deps.Verbose)
		if herr != nil {
			if dryRun {
				var re *generate.RescueError
				if errors.As(herr, &re) {
					// FR-V8a: hook rejection under --dry-run → warn-and-print (exit 0). Keep the would-be message so the
					// dry-run Result (below) carries it; the runner returned "" on error.
					fmt.Fprintf(os.Stderr, "⚠ commit hook rejected the would-be message under --dry-run: %v\n", re.Cause)
					wouldBe := re.Candidate
					if wouldBe == "" {
						wouldBe = msg
					}
					msg = wouldBe // fall through to the `if dryRun` Result (prints msg; exit 0)
				} else {
					return Result{}, herr // infrastructure error (hooks dir / msg file / read-back) → propagate
				}
			} else {
				return Result{}, herr // !dryRun → rescue (FR-V7, exit 3) — mirrors S1's CommitStaged
			}
		} else {
			// (F1) capture the PRE-hook frozen tree (before reassignment) so the post-commit reconciliation
			// can detect a permitted pre-commit re-tree and sync the live index's snapshot-path entries.
			snapshotTreeForReconcile = treeSHA // the PRE-hook tree (treeSHA is reassigned on the next line)
			treeSHA, msg = ft, fm              // hook accepted (possibly re-treed + prepare-annotated) → downstream uses these
		}
	}

	// §9.25 git parity (Issue 4): a prepare-commit-msg / commit-msg hook may have emptied the message file
	// (a rejection / force-re-edit pattern). git aborts "Aborting commit due to empty commit message.";
	// mirror it — return the BARE generate.ErrEmptyMessage (exit 1, NOT a rescue), same as the --edit path
	// and S1's CommitStaged guard. This is NOT the dryRun warn-and-print path (ErrEmptyMessage is not a
	// *RescueError, and this guard sits AFTER the hooks block) — an empty message aborts even under --dry-run.
	// HEAD + live index are untouched (the abort returns before CommitTree → no update-ref ran).
	if strings.TrimSpace(msg) == "" {
		return Result{}, generate.ErrEmptyMessage
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

	// ---- Post-commit hook (PRD §9.25 FR-V7). Reached ONLY when !dryRun (the `if dryRun` block returns above).
	// Best-effort; the return is DISREGARDED (the commit already landed). Mirrors S1's CommitStaged. ----
	if deps.Hooks != nil {
		// (F1) FIRST reconcile the live index's snapshot-path entries to the committed tree when a
		// permitted pre-commit mutation re-treed (the commit just landed; HEAD now points at the hook's
		// tree). git-commit parity: a formatter/lint-staged/prettier pre-commit must leave `git status`
		// clean and the index holding the hook's blob. Best-effort: a non-nil error is logged at --verbose
		// and NEVER undoes the commit.
		if rerr := deps.Hooks.ReconcileIndex(ctx, deps.Git, snapshotTreeForReconcile, treeSHA, dryRun, deps.Verbose); rerr != nil {
			if deps.Verbose != nil {
				deps.Verbose.VerboseWarn(fmt.Sprintf("post-mutation index reconcile failed (commit stands): %v", rerr))
			}
		}
		_ = deps.Hooks.RunPostCommit(ctx, deps.Git, cfg, dryRun, deps.Verbose)
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
