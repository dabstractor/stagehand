// Package stagehand is the thin, v1-stable public library surface (PRD §14.1)
// that an external integrator — a GUI, a CI runner, a pre-commit hook (US12) —
// links against to generate a commit message from the currently-staged index.
// It owns exactly three exported types (Options, Result) and one function
// (GenerateCommit), plus re-exported aliases for the three sentinel errors so
// an integrator can branch on them via errors.Is WITHOUT importing the
// internal package.
//
// GenerateCommit is a STRICTLY THIN wrapper: it resolves the Config+Registry
// (config.Load), layers the caller's Options on top (Options is the HIGHEST
// precedence source), resolves the provider+model (with first-detected
// fallback), wires the real collaborators (git.New, provider.NewExecutor,
// ui.NewOutput), and DELEGATES the entire diff→snapshot→generate→dedupe→commit
// pipeline to internal/generate.CommitStaged (the single two-nested-loop
// orchestrator, decisions.md §3). It NEVER re-implements the pipeline, NEVER
// stages (git add is a CLI-only concern — the v2 seam, decisions.md §1), and
// NEVER os.Exit (it returns the sentinel errors so the caller maps exit codes).
//
// Stability (PRD Appendix E.6): every exported symbol in this package carries
// a `// Stable as of v1.0` godoc marker. The Options struct is FIXED and
// additive-only — a future v2 may add fields but must not remove or rename
// these, so an integrator compiling against v1 stays source-compatible.
package stagehand

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/ui"
)

// Options is the single argument GenerateCommit takes (PRD §14.1). Every field
// is optional; the zero value of each means "do not override the resolved
// config" so a bare Options{} generates a commit with the auto-resolved
// provider+model. Options is the HIGHEST-precedence source: when a field is
// set it overrides whatever config.Load resolved from defaults/files (env/flag
// parsing is the CLI layer's job, P1.M7.T2.S1 — GenerateCommit does NOT read
// STAGEHAND_* env vars).
//
// Field order is FIXED per PRD §14.1 (Appendix E.6: additive-only).
//
// Stable as of v1.0
type Options struct {
	// Provider names the agent provider to use (e.g. "pi"); empty means
	// "use cfg.Provider, else the first detected provider on $PATH".
	Provider string
	// Model overrides the resolved provider's DefaultModel; empty means "use
	// cfg.Model, else the manifest DefaultModel".
	Model string
	// SystemExtra is appended to the built system prompt (after the
	// conventional-commit + history examples), letting a caller inject project
	// conventions or style nudges without re-implementing the prompt builders.
	// Empty is a no-op.
	SystemExtra string
	// DryRun (PRD FR49) runs the full pipeline but skips commit-tree/update-ref:
	// the generated message is printed to stdout and returned with
	// CommitSHA="" and HEAD left unchanged. It NEVER stages.
	DryRun bool
	// Timeout is the per-agent-invocation deadline. A zero/negative value
	// means "no deadline" (the resolved config.Timeout is used only when this
	// is zero, matching the config-layer semantics).
	Timeout time.Duration
	// Verbose enables FR50 stderr tracing of the resolved provider command,
	// the raw agent stdout, and each generation/retry attempt. false is a
	// no-op (stderr stays empty, stdout byte-clean) — the existing behavior
	// when unset. Resolved by the CLI layer from --verbose/-v/STAGEHAND_VERBOSE
	// and threaded through GenerateCommit into the shared *ui.Output that drives
	// the executor and the generate orchestrator.
	Verbose bool
}

// Result is the successful-commit return value (PRD §14.1): the new commit's
// full SHA, its subject (first line) and full message, plus the RESOLVED
// provider+model (so an integrator that left them empty for auto-resolution
// can observe which provider/model actually ran). On error GenerateCommit
// returns the zero Result and a sentinel.
//
// Field order is FIXED per PRD §14.1 (Appendix E.6: additive-only).
//
// Stable as of v1.0
type Result struct {
	// CommitSHA is the full 40-hex SHA of the created commit, or "" on a
	// DryRun (no commit was made).
	CommitSHA string
	// Subject is the first line of the generated message.
	Subject string
	// Message is the full generated commit message.
	Message string
	// Provider is the resolved provider name (the value the executor used).
	Provider string
	// Model is the resolved model name (the value the executor used).
	Model string
}

// Sentinel errors GenerateCommit RETURNS (it NEVER os.Exit). These are aliases
// of the internal/generate values so an integrator can branch via
// errors.Is(stagehand.ErrNothingToCommit, err) WITHOUT importing internal/*.
// The CLI (P1.M7.T2.S1) maps them to exit codes.
var (
	// ErrNothingToCommit is returned when nothing is staged (diff==""). No
	// snapshot was taken, so there is nothing to rescue. Alias of
	// generate.ErrNothingToCommit.
	//
	// Stable as of v1.0
	ErrNothingToCommit = generate.ErrNothingToCommit
	// ErrRescue is returned when a snapshot was taken but no commit was
	// created (timeout / agent-error / parse-fail / dup-exhaustion /
	// post-snapshot git error / signal-cancel); the rescue instructions were
	// already printed to stderr. Alias of generate.ErrRescue.
	//
	// Stable as of v1.0
	ErrRescue = generate.ErrRescue
	// ErrHeadMoved is returned when HEAD moved concurrently with generation
	// (the update-ref CAS aborted); the generated message + manual recovery
	// command were printed to stderr. Alias of generate.ErrHeadMoved.
	//
	// Stable as of v1.0
	ErrHeadMoved = generate.ErrHeadMoved
)

// GenerateCommit generates a commit message from the currently-staged index and
// (unless Options.DryRun is set) creates the commit (PRD §14.1, FR49). It is a
// thin wrapper that resolves Config+Registry via config.Load (repoDir = the
// current working directory), layers opts on top (opts is the HIGHEST
// precedence source), resolves the provider (opts.Provider → cfg.Provider →
// first detected) and model (opts.Model → cfg.Model → manifest.DefaultModel),
// wires the real collaborators, and delegates the full pipeline to
// internal/generate.CommitStaged. It maps the internal Result to the public
// Result, adding the resolved Provider+Model.
//
// On error it returns the zero Result and the sentinel (propagated as-is so
// the caller can branch via errors.Is). It NEVER calls git add/AddAll and
// NEVER os.Exit (staging policy and exit-code mapping are the CLI layer's job).
//
// The success block (and, in DryRun, the message) is printed to stdout by
// CommitStaged — callers must NOT re-print them.
//
// Stable as of v1.0
func GenerateCommit(ctx context.Context, opts Options) (Result, error) {
	// Resolve the config + registry from the current working directory. An
	// empty Flags{} means "no env/flag layer" — env/flag parsing is the CLI
	// layer's job (P1.M7.T2.S1 folds STAGEHAND_* env into opts there).
	cfg, reg, _, err := config.Load(config.Flags{}, ".")
	if err != nil {
		return Result{}, err
	}

	// Layer opts onto cfg AFTER Load (avoids the FlagsLayer non-nil-pointer-
	// to-zero-value trap: applying opts via config.Flags would let an empty
	// opts.Provider OVERWRITE a file-set provider). opts is the HIGHEST
	// precedence source, so only a non-empty/non-zero value overrides.
	if opts.Provider != "" {
		cfg.Provider = opts.Provider
	}
	if opts.Model != "" {
		cfg.Model = opts.Model
	}
	if opts.Timeout > 0 {
		cfg.Timeout = opts.Timeout
	}

	// Resolve the provider: opts.Provider → cfg.Provider → first detected in
	// reg.List() order (reg.List is sorted, so this is deterministic). Mirrors
	// cmd/stagehand/providers.go resolveDefault (inlined here to avoid
	// importing the cmd package — a library must not depend on cmd/*).
	resolvedProvider := cfg.Provider
	if resolvedProvider == "" {
		detected := reg.Detect()
		for _, n := range reg.List() {
			if detected[n] {
				resolvedProvider = n
				break
			}
		}
	}
	if resolvedProvider == "" {
		return Result{}, fmt.Errorf("stagehand: no provider configured and no built-in agent detected on PATH")
	}

	// Look up the resolved provider's manifest (fails fast on an unknown name
	// rather than letting the executor produce an opaque LookPath error).
	manifest, ok := reg.Get(resolvedProvider)
	if !ok {
		return Result{}, fmt.Errorf("stagehand: provider %q not found", resolvedProvider)
	}

	// Resolve the model: opts.Model → cfg.Model → manifest.DefaultModel.
	resolvedModel := cfg.Model
	if resolvedModel == "" {
		resolvedModel = manifest.DefaultModel
	}

	// Write the resolved values back into cfg so CommitStaged (and the
	// executor it drives) see them — cfg.Provider/cfg.Model are what the
	// two-nested-loop reads off deps.Config.
	cfg.Provider, cfg.Model = resolvedProvider, resolvedModel

	// Wire the real collaborators. git.New(".") and provider.NewExecutor(".")
	// both bind to the current working directory (repoDir = cwd, matching the
	// config.Load repoDir above). A SINGLE *ui.Output honoring opts.Verbose is
	// shared by the executor (FR50 resolved-command/raw-stdout traces) and the
	// generate orchestrator (retry markers), so the whole agent trace flows
	// through one FR50 stderr sink. noColor=false means color auto-disables
	// when stdout is not a TTY (a pipe), so `GenerateCommit | tee` stays
	// byte-clean regardless of verbose.
	g, err := git.New(".")
	if err != nil {
		return Result{}, err
	}

	out := ui.NewOutput(os.Stdout, os.Stderr, opts.Verbose, false)
	exec := provider.NewExecutor(".") // exec.Output nil-safe for direct callers
	exec.Output = out                 // set the FR50 verbose sink
	deps := generate.Deps{
		Git:         g,
		Runner:      exec,
		Manifest:    manifest,
		Config:      cfg,
		Output:      out, // same sink: executor + generate share one verbose Output
		DryRun:      opts.DryRun,
		SystemExtra: opts.SystemExtra,
	}

	res, err := generate.CommitStaged(ctx, deps)
	if err != nil {
		return Result{}, err
	}

	// Map the internal Result to the public Result, adding the resolved
	// Provider+Model so an integrator that left them empty can observe which
	// provider/model actually ran.
	return Result{
		CommitSHA: res.CommitSHA,
		Subject:   res.Subject,
		Message:   res.Message,
		Provider:  resolvedProvider,
		Model:     resolvedModel,
	}, nil
}
