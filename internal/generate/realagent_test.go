//go:build integration_real

// Package generate test: the PRD §20.1 layer-4 "Integration — real agents (opt-in, not in CI)" suite.
// Built ONLY under -tags integration_real; runs ONLY when STAGEHAND_RUN_REAL=1. NOT in CI
// (make test / make coverage pass no -tags). Drives generate.CommitStaged against each of the 6 real
// builtin provider manifests (pi/claude/gemini/opencode/codex/cursor). Resolves the two
// `// TO CONFIRM (integration)` notes in internal/provider/builtin.go (codex exec→stdout; cursor --mode ask).
//
// Manual run command:
//
//	STAGEHAND_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestRealAgents -v -timeout 30m
package generate

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/provider"
)

// realDefault is the best-effort model + inference provider for one AGENT's real run (env-overridable).
// NOTE the terminology (PRD §12 / FR-R5b): the loop's `name` is the AGENT (pi, claude, …); `inference`
// here is the upstream MODEL-PROVIDER (zai, openrouter, …) — the two must NEVER be conflated. The
// inference provider is what a user sets via [provider.<name>] default_provider and what Render
// forwards as the agent's --provider; it is NOT the agent identity and does NOT go into cfg.Provider.
// model=="" ⇒ let Render fall back to the manifest DefaultModel (or emit no model flag).
// inference=="" ⇒ no --provider flag (single-backend agents, or the combined-form agents).
type realDefault struct {
	model, inference string
}

// realDefaults — best-effort per-AGENT model + inference provider (env-overridable). "" ⇒ fall back
// to the manifest DefaultModel / emit no flag. Sourced from architecture/external_deps.md.
// Override per-run via STAGEHAND_REAL_MODEL_<NAME> / STAGEHAND_REAL_INFERENCE_<NAME>.
var realDefaults = map[string]realDefault{
	"pi":       {"glm-5-turbo", "zai"},            // explicit personal override (commit-pi); manifest default empty (FR-D2)
	"claude":   {"", ""},                          // sonnet from manifest default
	"gemini":   {"", ""},                          // gemini-2.5-pro from manifest default
	"opencode": {"anthropic/claude-sonnet-4", ""}, // manifest default is "" → MUST supply a model
	"codex":    {"", ""},                          // model from ~/.codex/config.toml
	"cursor":   {"", ""},                          // per-account default model
}

// providerNames — registry preference order (registry.go preferredBuiltins); deterministic subtest order.
var providerNames = []string{"pi", "opencode", "cursor", "gemini", "codex", "claude"} // FR-D1 preference order (registry.go preferredBuiltins) minus agy (experimental — non-TTY stdout drop, issue #76; not real-tested). Subtest display order only.

// envOr returns the value of the environment variable key, or def if unset or empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// realConfig builds a config for one AGENT's real run. cfg.Provider is the AGENT name (pi, claude, …)
// and cfg.Model is the model. The inference provider is intentionally NOT a config field here — it is
// applied to the manifest's default_provider in TestRealAgents (mirroring a user's
// [provider.<name>] default_provider), the only field Render consults for the --provider flag (FR-R5b).
// Timeout/MaxDuplicateRetries inherit config.Defaults() (120s/3).
func realConfig(name string) config.Config {
	cfg := config.Defaults()
	d := realDefaults[name]
	cfg.Provider = name // the AGENT — NOT the inference provider (prior code conflated the two)
	cfg.Model = envOr("STAGEHAND_REAL_MODEL_"+strings.ToUpper(name), d.model)
	return cfg
}

// logResolvedCommand renders the manifest to its concrete argv and logs it (payload truncated) so the
// operator can audit the EXACT real invocation — this is what makes the external_deps.md TO CONFIRM
// items (codex exec flags, cursor --mode ask) visually verifiable. Mirrors CommitStaged: model from
// cfg, provider param "" (the inference provider comes from m.DefaultProvider — never cfg.Provider).
func logResolvedCommand(t *testing.T, name string, m provider.Manifest, cfg config.Config) {
	t.Helper()
	spec, err := m.Render(cfg.Model, "<system prompt>", "<staged diff>", "off")
	if err != nil {
		t.Logf("[%s] render error (manifest may be invalid): %v", name, err)
		return
	}
	args := strings.Join(spec.Args, " ")
	if len(args) > 200 {
		args = args[:200] + " …(truncated)"
	}
	t.Logf("[%s] resolved command: %s %s   (stdin=%t)", name, spec.Command, args, spec.Stdin != "")
}

// TestRealAgents drives each real builtin provider manifest through CommitStaged end-to-end. Opt-in:
// build tag (integration_real) + STAGEHAND_RUN_REAL=1 + binary on $PATH. NOT in CI.
func TestRealAgents(t *testing.T) {
	if os.Getenv("STAGEHAND_RUN_REAL") != "1" {
		t.Skip("skipping real-agent suite; set STAGEHAND_RUN_REAL=1 and build with -tags integration_real")
	}

	reg := provider.NewRegistry(nil) // pure built-ins — no user-config noise

	for _, name := range providerNames {
		name := name
		t.Run(name, func(t *testing.T) {
			m, ok := reg.Get(name)
			if !ok {
				t.Fatalf("registry has no builtin %q (keep providerNames in sync with BuiltinManifests)", name)
			}

			// Gate 2: per-subtest install check.
			if !reg.IsInstalled(m) {
				t.Skipf("%s (%s) not on $PATH", name, m.DetectCommand())
			}

			// Fixture (mirror TestCommitStaged_Success): born repo + initial commit + staged file.
			repo := t.TempDir()
			initRepo(t, repo)
			commitRaw(t, repo, "initial")
			writeFile(t, repo, "main.go", "package main\n\nfunc main() { println(\"hello\") }\n")
			stageFile(t, repo, "main.go")

			cfg := realConfig(name)

			// Apply the inference provider to the manifest — mirrors a user's [provider.<name>]
			// default_provider. This is the ONLY source Render consults for the --provider flag; stuffing
			// it into cfg.Provider (as prior code did) is the agent/model-provider conflation FR-R5b now
			// rejects at Render. DefaultProvider is nil on the pure built-in; set it only when non-empty.
			if inf := envOr("STAGEHAND_REAL_INFERENCE_"+strings.ToUpper(name), realDefaults[name].inference); inf != "" {
				// v3 FR-R5b: fold the inference provider into the model slash-prefix.
				// cfg.Model may already contain a prefix from env override; only prepend if absent.
				if cfg.Model == "" {
					cfg.Model = inf + "/" + realDefaults[name].model
				} else if !strings.Contains(cfg.Model, "/") {
					cfg.Model = inf + "/" + cfg.Model
				}
			}

			logResolvedCommand(t, name, m, cfg)

			// RUN: real agent via CommitStaged. Timeout per attempt = cfg.Timeout (120s).
			res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
			if err != nil {
				t.Fatalf("real agent %s failed end-to-end: %v\n(resolved command logged above — distinguish manifest bug vs unavailable model)", name, err)
			}

			// Assert the COMMIT, not the message words (the agent's text is nondeterministic).
			if res.Message == "" {
				t.Errorf("res.Message is empty; agent produced no parseable commit message")
			}
			if !shaRe.MatchString(res.CommitSHA) {
				t.Errorf("CommitSHA = %q, want a hex SHA", res.CommitSHA)
			}
			if got := headSHA(t, repo); got != res.CommitSHA {
				t.Errorf("HEAD = %q, want %q (commit did not land on HEAD)", got, res.CommitSHA)
			}
			if got := gitOut(t, repo, "log", "--format=%B", "-n1", res.CommitSHA); got != res.Message {
				t.Errorf("git log message = %q, want %q (message did not round-trip into the commit)", got, res.Message)
			}
			if len(res.Changes) == 0 {
				t.Errorf("res.Changes is empty; DiffTree reported no landed file")
			}

			short := res.CommitSHA
			if len(short) > 7 {
				short = short[:7]
			}
			t.Logf("[%s] OK — committed %s: %q", name, short, res.Message)
		})
	}
}
