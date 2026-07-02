// load_test.go is a WHITE-BOX test (package config, matching the house
// convention used by config_test.go, defaults_test.go, and file_test.go). It
// exercises Load() (P1.M5.T3.S1): the FR34 precedence chain, the
// provider-override per-key shallow merge + built-in field-merge, the §19
// repo-local trust notice, the --config/STAGEHAND_CONFIG discovery override,
// and the present-but-zero pointer semantics. It REUSES the helpers already
// defined in file_test.go (initGitRepo, gitSet, writeFile, golden162, the
// pointer helpers sp/ip/bp/dp, ptrStrEq/..., and assertEmptyOverlay) — no
// testify, no new infra. Imports are stdlib (testing, reflect, path/filepath)
// plus internal/provider for the field-merge assertion against provider.Builtins.
package config

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dustin/stagehand/internal/provider"
)

// sixBuiltins is the expected sorted provider name set produced by
// NewRegistry(provider.Builtins(), nil) — the registry surface Load returns
// when no overrides are present.
var sixBuiltins = []string{"claude", "codex", "cursor", "gemini", "opencode", "pi"}

// noGlobalFile points XDG at an empty temp dir so readGlobalFile yields an
// empty overlay and never records a ConfigPath. Used to make tests
// deterministic regardless of the host's real ~/.config.
func noGlobalFile(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

// assertConfigScalarsEq compares the 12 resolved scalars of got against want
// (ConfigPath and ProviderOverrides are checked separately where relevant).
func assertConfigScalarsEq(t *testing.T, got, want Config) {
	t.Helper()
	got.ConfigPath = want.ConfigPath // compared separately by callers
	if !reflect.DeepEqual(got.Provider, want.Provider) ||
		got.Model != want.Model ||
		got.Timeout != want.Timeout ||
		got.AutoStageAll != want.AutoStageAll ||
		got.Verbose != want.Verbose ||
		got.NoColor != want.NoColor ||
		got.MaxDiffBytes != want.MaxDiffBytes ||
		got.MaxMdLines != want.MaxMdLines ||
		got.MaxDuplicateRetries != want.MaxDuplicateRetries ||
		got.SubjectTargetChars != want.SubjectTargetChars ||
		got.Output != want.Output ||
		got.StripCodeFence != want.StripCodeFence {
		t.Errorf("resolved scalars do not match expected:\n got=%+v\nwant=%+v", got, want)
	}
}

// TestLoad_DefaultsOnly asserts the floor case: empty Flags + a clean repo
// resolves to Default() scalars, a registry of the six built-ins, ConfigPath
// == "", and no trust notice (no repo-local source).
func TestLoad_DefaultsOnly(t *testing.T) {
	noGlobalFile(t)
	repo := initGitRepo(t) // empty repo: no .stagehand.toml, no stagehand.* keys

	cfg, reg, notice, err := Load(Flags{}, repo)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}

	// Every scalar equals Default(); ProviderOverrides stays nil.
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("cfg = %+v\nwant Default() %+v (ProviderOverrides should be nil)", cfg, Default())
	}
	if cfg.ConfigPath != "" {
		t.Errorf("ConfigPath = %q, want \"\" (no file loaded)", cfg.ConfigPath)
	}

	if got := reg.List(); !reflect.DeepEqual(got, sixBuiltins) {
		t.Errorf("reg.List() = %v, want %v", got, sixBuiltins)
	}
	if notice != "" {
		t.Errorf("trustNotice = %q, want \"\" (no repo-local source)", notice)
	}
}

// TestLoad_Precedence_EachLevelBeatsBelow proves each FR34 layer beats the one
// below it via DISTINCT provider values: global "g" < repo-file "r" <
// git-config "t" < env "e" < flag "f". The top-three layers reuse one repo
// fixture (g/r/t set); the lower-two use separate repos so the absent layers
// are truly absent.
func TestLoad_Precedence_EachLevelBeatsBelow(t *testing.T) {
	// Fixture: global file "g" (via XDG), repo file "r", git-config "t".
	xdg := t.TempDir()
	writeFile(t, xdg, filepath.Join("stagehand", "config.toml"),
		"[defaults]\nprovider = \"g\"\n")
	t.Setenv("XDG_CONFIG_HOME", xdg)

	repo := initGitRepo(t)
	writeFile(t, repo, ".stagehand.toml", "[defaults]\nprovider = \"r\"\n")
	gitSet(t, repo, "config", "stagehand.provider", "t")

	cases := []struct {
		name  string
		envP  *string
		flagP *string
		want  string
	}{
		{"flag beats env beats git beats repo beats global", sp("e"), sp("f"), "f"},
		{"env beats git beats repo beats global", sp("e"), nil, "e"},
		{"git beats repo beats global (no env/flag)", nil, nil, "t"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags := Flags{
				Env:  FlagsLayer{Provider: tc.envP},
				Flag: FlagsLayer{Provider: tc.flagP},
			}
			cfg, _, _, err := Load(flags, repo)
			if err != nil {
				t.Fatalf("Load: unexpected error: %v", err)
			}
			if cfg.Provider != tc.want {
				t.Errorf("Provider = %q, want %q", cfg.Provider, tc.want)
			}
		})
	}

	// repo-file "r" beats global "g" when git-config is absent.
	t.Run("repo file beats global", func(t *testing.T) {
		repoFileOnly := initGitRepo(t)
		writeFile(t, repoFileOnly, ".stagehand.toml", "[defaults]\nprovider = \"r\"\n")
		cfg, _, _, err := Load(Flags{}, repoFileOnly)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Provider != "r" {
			t.Errorf("Provider = %q, want \"r\"", cfg.Provider)
		}
	})

	// global "g" beats the Default() "" when neither repo file nor git-config.
	t.Run("global beats default", func(t *testing.T) {
		cleanRepo := initGitRepo(t)
		cfg, _, _, err := Load(Flags{}, cleanRepo)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Provider != "g" {
			t.Errorf("Provider = %q, want \"g\"", cfg.Provider)
		}
	})
}

// TestLoad_FlagBeatsEnv asserts the FR34 flag>env rule directly.
func TestLoad_FlagBeatsEnv(t *testing.T) {
	noGlobalFile(t)
	repo := initGitRepo(t)
	flags := Flags{
		Env:  FlagsLayer{Provider: sp("a")},
		Flag: FlagsLayer{Provider: sp("b")},
	}
	cfg, _, _, err := Load(flags, repo)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Provider != "b" {
		t.Errorf("Provider = %q, want \"b\" (flag beats env)", cfg.Provider)
	}
}

// TestLoad_ProviderFieldMerge asserts the field-merge-over-built-ins rule: a
// [provider.pi] override setting ONLY default_model yields a registry pi whose
// DefaultModel is overridden while the built-in BareFlags/PrintFlag/ModelFlag/
// PromptDelivery/Command survive intact. The merge happens inside NewRegistry,
// asserted via the returned reg.
func TestLoad_ProviderFieldMerge(t *testing.T) {
	xdg := t.TempDir()
	writeFile(t, xdg, filepath.Join("stagehand", "config.toml"),
		"[provider.pi]\ndefault_model = \"glm-override\"\n")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	repo := initGitRepo(t)

	cfg, reg, _, err := Load(Flags{}, repo)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// The resolved override map carries the single pi override.
	if cfg.ProviderOverrides == nil {
		t.Fatal("ProviderOverrides = nil, want pi override")
	}
	ov, ok := cfg.ProviderOverrides["pi"]
	if !ok {
		t.Fatal("ProviderOverrides missing key \"pi\"")
	}
	if ov.DefaultModel != "glm-override" {
		t.Errorf("override pi.DefaultModel = %q, want \"glm-override\"", ov.DefaultModel)
	}

	// Registry pi: DefaultModel overridden, built-in fields intact.
	pi, ok := reg.Get("pi")
	if !ok {
		t.Fatal("reg.Get(\"pi\"): provider missing")
	}
	if pi.DefaultModel != "glm-override" {
		t.Errorf("reg pi.DefaultModel = %q, want \"glm-override\" (override won)", pi.DefaultModel)
	}
	builtin := provider.Builtins()["pi"]
	if !reflect.DeepEqual(pi.BareFlags, builtin.BareFlags) {
		t.Errorf("reg pi.BareFlags = %#v, want built-in %#v (field-merge keeps it)", pi.BareFlags, builtin.BareFlags)
	}
	if pi.PrintFlag != builtin.PrintFlag {
		t.Errorf("reg pi.PrintFlag = %q, want built-in %q", pi.PrintFlag, builtin.PrintFlag)
	}
	if pi.ModelFlag != builtin.ModelFlag {
		t.Errorf("reg pi.ModelFlag = %q, want built-in %q", pi.ModelFlag, builtin.ModelFlag)
	}
	if pi.PromptDelivery != builtin.PromptDelivery {
		t.Errorf("reg pi.PromptDelivery = %q, want built-in %q", pi.PromptDelivery, builtin.PromptDelivery)
	}
	if pi.Command != builtin.Command {
		t.Errorf("reg pi.Command = %q, want built-in %q", pi.Command, builtin.Command)
	}
	if pi.SystemPromptFlag != builtin.SystemPromptFlag {
		t.Errorf("reg pi.SystemPromptFlag = %q, want built-in %q", pi.SystemPromptFlag, builtin.SystemPromptFlag)
	}
}

// TestLoad_ProviderOverrides_PerKeyShallowMerge asserts the per-key shallow
// merge across file sources: a global [provider.alpha] and a repo
// [provider.beta] BOTH survive into the registry (different keys), while a
// repo [provider.pi] REPLACES a global [provider.pi] (same key — higher
// source's whole entry wins, no field-merge of two user manifests).
func TestLoad_ProviderOverrides_PerKeyShallowMerge(t *testing.T) {
	xdg := t.TempDir()
	writeFile(t, xdg, filepath.Join("stagehand", "config.toml"),
		"[provider.alpha]\ncommand = \"/alpha\"\n"+
			"[provider.pi]\ndefault_model = \"global-pi\"\n")
	t.Setenv("XDG_CONFIG_HOME", xdg)

	repo := initGitRepo(t)
	writeFile(t, repo, ".stagehand.toml",
		"[provider.beta]\ncommand = \"/beta\"\n"+
			"[provider.pi]\ndefault_model = \"repo-pi\"\n")

	cfg, reg, _, err := Load(Flags{}, repo)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Different-named providers from BOTH file layers survive into the map.
	if _, ok := cfg.ProviderOverrides["alpha"]; !ok {
		t.Error("ProviderOverrides missing \"alpha\" (global key must survive)")
	}
	if _, ok := cfg.ProviderOverrides["beta"]; !ok {
		t.Error("ProviderOverrides missing \"beta\" (repo key must survive)")
	}
	// Same-named key: repo entry replaced the global entry (no field-merge of
	// two user manifests — the value is the repo one verbatim).
	pi, ok := cfg.ProviderOverrides["pi"]
	if !ok {
		t.Fatal("ProviderOverrides missing \"pi\"")
	}
	if pi.DefaultModel != "repo-pi" {
		t.Errorf("override pi.DefaultModel = %q, want \"repo-pi\" (repo replaced global)", pi.DefaultModel)
	}

	// All three user providers are present in the registry alongside the
	// six built-ins.
	for _, name := range []string{"alpha", "beta"} {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("reg missing user provider %q", name)
		}
	}
	regPi, ok := reg.Get("pi")
	if !ok {
		t.Fatal("reg missing \"pi\"")
	}
	if regPi.DefaultModel != "repo-pi" {
		t.Errorf("reg pi.DefaultModel = %q, want \"repo-pi\"", regPi.DefaultModel)
	}
	if got := reg.List(); len(got) != len(sixBuiltins)+2 {
		t.Errorf("reg.List() = %v (len %d), want six built-ins + alpha + beta (len %d)", got, len(got), len(sixBuiltins)+2)
	}
}

// TestLoad_TrustNotice_RepoFile asserts a repo .stagehand.toml that sets the
// provider triggers the §19 notice naming the resolved provider.
func TestLoad_TrustNotice_RepoFile(t *testing.T) {
	noGlobalFile(t)
	repo := initGitRepo(t)
	writeFile(t, repo, ".stagehand.toml", "[defaults]\nprovider = \"claude\"\n")

	cfg, _, notice, err := Load(Flags{}, repo)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Provider != "claude" {
		t.Errorf("Provider = %q, want \"claude\"", cfg.Provider)
	}
	want := "stagehand: repo-local config changed provider to claude"
	if notice != want {
		t.Errorf("trustNotice = %q, want %q", notice, want)
	}
}

// TestLoad_TrustNotice_GitConfig asserts stagehand.provider git-config triggers
// the §19 notice, and its absence does not.
func TestLoad_TrustNotice_GitConfig(t *testing.T) {
	noGlobalFile(t)

	t.Run("set fires notice", func(t *testing.T) {
		repo := initGitRepo(t)
		gitSet(t, repo, "config", "stagehand.provider", "pi")
		cfg, _, notice, err := Load(Flags{}, repo)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Provider != "pi" {
			t.Errorf("Provider = %q, want \"pi\"", cfg.Provider)
		}
		want := "stagehand: repo-local config changed provider to pi"
		if notice != want {
			t.Errorf("trustNotice = %q, want %q", notice, want)
		}
	})

	t.Run("unset no notice", func(t *testing.T) {
		repo := initGitRepo(t)
		_, _, notice, err := Load(Flags{}, repo)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if notice != "" {
			t.Errorf("trustNotice = %q, want \"\" (no repo-local provider)", notice)
		}
	})
}

// TestLoad_TrustNotice_NotGlobalNorEnvNorFlag asserts global file, env, and CLI
// flag each set the provider WITHOUT firing the §19 notice (only repo-local
// sources are attacker-committable).
func TestLoad_TrustNotice_NotGlobalNorEnvNorFlag(t *testing.T) {
	t.Run("global file", func(t *testing.T) {
		xdg := t.TempDir()
		writeFile(t, xdg, filepath.Join("stagehand", "config.toml"),
			"[defaults]\nprovider = \"pi\"\n")
		t.Setenv("XDG_CONFIG_HOME", xdg)
		repo := initGitRepo(t)
		cfg, _, notice, err := Load(Flags{}, repo)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Provider != "pi" {
			t.Errorf("Provider = %q, want \"pi\"", cfg.Provider)
		}
		if notice != "" {
			t.Errorf("trustNotice = %q, want \"\" (global file is not repo-local)", notice)
		}
	})

	t.Run("env", func(t *testing.T) {
		noGlobalFile(t)
		repo := initGitRepo(t)
		flags := Flags{Env: FlagsLayer{Provider: sp("pi")}}
		cfg, _, notice, err := Load(flags, repo)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Provider != "pi" {
			t.Errorf("Provider = %q, want \"pi\"", cfg.Provider)
		}
		if notice != "" {
			t.Errorf("trustNotice = %q, want \"\" (env is not repo-local)", notice)
		}
	})

	t.Run("flag", func(t *testing.T) {
		noGlobalFile(t)
		repo := initGitRepo(t)
		flags := Flags{Flag: FlagsLayer{Provider: sp("pi")}}
		cfg, _, notice, err := Load(flags, repo)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Provider != "pi" {
			t.Errorf("Provider = %q, want \"pi\"", cfg.Provider)
		}
		if notice != "" {
			t.Errorf("trustNotice = %q, want \"\" (flag is not repo-local)", notice)
		}
	})
}

// TestLoad_TrustNotice_HigherLayerKeepsNotice asserts that when a repo-local
// source sets the provider but a higher (env/flag) layer overrides the FINAL
// value, the §19 notice still fires and names the final resolved provider
// (the redirection was visible at repo-local time).
func TestLoad_TrustNotice_HigherLayerKeepsNotice(t *testing.T) {
	noGlobalFile(t)
	repo := initGitRepo(t)
	writeFile(t, repo, ".stagehand.toml", "[defaults]\nprovider = \"claude\"\n")
	flags := Flags{Flag: FlagsLayer{Provider: sp("gemini")}}

	cfg, _, notice, err := Load(flags, repo)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Provider != "gemini" {
		t.Errorf("Provider = %q, want \"gemini\" (flag overrode repo-local)", cfg.Provider)
	}
	want := "stagehand: repo-local config changed provider to gemini"
	if notice != want {
		t.Errorf("trustNotice = %q, want %q", notice, want)
	}
}

// TestLoad_ConfigFlagOverridesDiscovery asserts --config (flag wins over env
// via resolvedConfigPath) REPLACES the global+repo file layers: only the
// explicit file is the file layer, and no trust notice fires for it (it is
// user-chosen). It also confirms the git-config layer still applies under
// --config (and, being repo-local, CAN still fire the notice).
func TestLoad_ConfigFlagOverridesDiscovery(t *testing.T) {
	// Global and repo files both set provider, but they must be IGNORED.
	xdg := t.TempDir()
	writeFile(t, xdg, filepath.Join("stagehand", "config.toml"),
		"[defaults]\nprovider = \"g\"\n")
	t.Setenv("XDG_CONFIG_HOME", xdg)

	repo := initGitRepo(t)
	writeFile(t, repo, ".stagehand.toml", "[defaults]\nprovider = \"r\"\n")

	// Explicit --config file is the sole file layer.
	cfgPath := writeFile(t, t.TempDir(), "explicit.toml",
		"[defaults]\nprovider = \"c\"\n")

	t.Run("explicit file replaces global+repo; no notice", func(t *testing.T) {
		flags := Flags{Flag: FlagsLayer{ConfigPath: sp(cfgPath)}}
		cfg, _, notice, err := Load(flags, repo)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Provider != "c" {
			t.Errorf("Provider = %q, want \"c\" (explicit --config is the file layer)", cfg.Provider)
		}
		if cfg.ConfigPath != cfgPath {
			t.Errorf("ConfigPath = %q, want %q (the explicit path)", cfg.ConfigPath, cfgPath)
		}
		if notice != "" {
			t.Errorf("trustNotice = %q, want \"\" (explicit --config is not repo-local)", notice)
		}
	})

	t.Run("flag ConfigPath wins over env ConfigPath", func(t *testing.T) {
		envPath := writeFile(t, t.TempDir(), "env.toml",
			"[defaults]\nprovider = \"envc\"\n")
		flags := Flags{
			Env:  FlagsLayer{ConfigPath: sp(envPath)},
			Flag: FlagsLayer{ConfigPath: sp(cfgPath)},
		}
		cfg, _, _, err := Load(flags, repo)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Provider != "c" {
			t.Errorf("Provider = %q, want \"c\" (flag ConfigPath beats env)", cfg.Provider)
		}
		if cfg.ConfigPath != cfgPath {
			t.Errorf("ConfigPath = %q, want %q (flag path wins)", cfg.ConfigPath, cfgPath)
		}
	})

	// git-config still applies under --config, and being repo-local it CAN
	// fire the §19 notice (beating the explicit file at layer 5 > layer 3/4).
	t.Run("git-config still applies and can notify", func(t *testing.T) {
		repoWithGit := initGitRepo(t)
		writeFile(t, repoWithGit, ".stagehand.toml", "[defaults]\nprovider = \"r\"\n")
		gitSet(t, repoWithGit, "config", "stagehand.provider", "t")
		flags := Flags{Flag: FlagsLayer{ConfigPath: sp(cfgPath)}}
		cfg, _, notice, err := Load(flags, repoWithGit)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Provider != "t" {
			t.Errorf("Provider = %q, want \"t\" (git-config beats explicit file)", cfg.Provider)
		}
		want := "stagehand: repo-local config changed provider to t"
		if notice != want {
			t.Errorf("trustNotice = %q, want %q", notice, want)
		}
	})
}

// TestLoad_PresentButZero asserts the present-but-zero pointer semantics: a
// higher source setting model="" (a non-nil pointer to the zero value)
// OVERWRITES a lower source's non-empty model.
func TestLoad_PresentButZero(t *testing.T) {
	xdg := t.TempDir()
	writeFile(t, xdg, filepath.Join("stagehand", "config.toml"),
		"[defaults]\nmodel = \"lower-model\"\n")
	t.Setenv("XDG_CONFIG_HOME", xdg)

	repo := initGitRepo(t)
	// repo file sets model = "" explicitly (present-but-zero) -> must win.
	writeFile(t, repo, ".stagehand.toml", "[defaults]\nmodel = \"\"\n")

	cfg, _, _, err := Load(Flags{}, repo)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "" {
		t.Errorf("Model = %q, want \"\" (present-but-zero overrides lower non-empty)", cfg.Model)
	}
}

// TestLoad_ConfigPathRecordedForGlobal asserts cfg.ConfigPath is the global
// path when the global file parsed non-empty, and "" when it is absent.
func TestLoad_ConfigPathRecordedForGlobal(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		xdg := t.TempDir()
		writeFile(t, xdg, filepath.Join("stagehand", "config.toml"),
			"[defaults]\nverbose = true\n")
		t.Setenv("XDG_CONFIG_HOME", xdg)
		repo := initGitRepo(t)
		cfg, _, _, err := Load(Flags{}, repo)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		want := filepath.Join(xdg, "stagehand", "config.toml")
		if cfg.ConfigPath != want {
			t.Errorf("ConfigPath = %q, want %q", cfg.ConfigPath, want)
		}
	})
	t.Run("absent", func(t *testing.T) {
		noGlobalFile(t)
		repo := initGitRepo(t)
		cfg, _, _, err := Load(Flags{}, repo)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.ConfigPath != "" {
			t.Errorf("ConfigPath = %q, want \"\" (global file absent)", cfg.ConfigPath)
		}
	})
}
