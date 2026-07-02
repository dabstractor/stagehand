// file.go is the source-loader layer of the FR34 precedence chain (PRD §16;
// decisions.md §6). It provides three config-source readers — readGlobalFile,
// readRepoFile, readGitConfig — each of which reads exactly ONE source into a
// fresh, self-contained [overlay] holding ONLY the fields that source actually
// set (nil pointer = "not set by this source"; a non-nil pointer, even to the
// zero value, = "set by this source"). Load() (P1.M5.T3.S1) layers these
// overlays lowest→highest; no reader mutates a shared/base Config.
//
// This file is a sibling of config.go (which OWNS the "// Package config" doc)
// and therefore carries a plain "package config" line, mirroring how
// internal/git/log.go defers the package doc to internal/git/git.go. It is the
// ONLY file in package config that imports go-toml/v2 and os/exec: config.go
// keeps the resolved Config type decoupled from both.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/dustin/stagehand/internal/provider"
)

// overlay is a PARTIAL configuration layer produced by one config source
// (global TOML, repo TOML, or repo git-config). Each scalar pointer is nil
// when the source did NOT set it, and a non-nil pointer (even to the zero
// value) when the source DID set it. This present-but-zero distinction —
// model == "" or verbose == false as a NON-nil pointer — is exactly what lets
// Load() (P1.M5.T3.S1) layer overlays lowest→highest without a second
// sentinel: a higher overlay never needs to know whether a lower value was a
// default or an explicit zero. ProviderOverrides holds the source's OWN
// [provider.<name>] tables (nil when the source has none); it is NOT
// field-merged over the built-ins here — that merge happens in the provider
// registry at Load() time (decisions.md §6).
type overlay struct {
	Provider            *string
	Model               *string
	Timeout             *time.Duration
	AutoStageAll        *bool
	Verbose             *bool
	NoColor             *bool
	MaxDiffBytes        *int
	MaxMdLines          *int
	MaxDuplicateRetries *int
	SubjectTargetChars  *int
	Output              *string
	StripCodeFence      *bool
	ProviderOverrides   map[string]provider.Manifest
}

// fileDTO is the top-level TOML shape of a config file (PRD §16.2): scalars
// are split across the [defaults] and [generation] tables, and the
// [provider.<name>] tables map DIRECTLY to provider.Manifest values (which
// already carry their own toml tags — file.go does NOT redefine a manifest
// DTO). Each table is a POINTER so go-toml/v2 leaves it nil when the whole
// table is ABSENT; likewise each scalar inside defaultsDTO/generationDTO is a
// pointer so a present zero value (model "", verbose false) is distinguishable
// from an absent key. go-toml/v2 leaves a pointer field nil when the TOML
// key/table is absent and populates it (even with the zero value) when present
// — this is the presence-detection trick the overlay relies on.
type fileDTO struct {
	Defaults   *defaultsDTO                 `toml:"defaults"`
	Generation *generationDTO               `toml:"generation"`
	Provider   map[string]provider.Manifest `toml:"provider"`
}

// defaultsDTO mirrors the [defaults] table (PRD §16.2): every field is a
// pointer for presence detection. Timeout is the §16.2 TOML STRING ("120s"),
// NOT a time.Duration — the string→Duration conversion happens in parseFile.
type defaultsDTO struct {
	Provider     *string `toml:"provider"`
	Model        *string `toml:"model"`
	Timeout      *string `toml:"timeout"`
	AutoStageAll *bool   `toml:"auto_stage_all"`
	Verbose      *bool   `toml:"verbose"`
	NoColor      *bool   `toml:"no_color"`
}

// generationDTO mirrors the [generation] table (PRD §16.2): every field is a
// pointer for presence detection.
type generationDTO struct {
	MaxDiffBytes        *int    `toml:"max_diff_bytes"`
	MaxMdLines          *int    `toml:"max_md_lines"`
	MaxDuplicateRetries *int    `toml:"max_duplicate_retries"`
	Output              *string `toml:"output"`
	StripCodeFence      *bool   `toml:"strip_code_fence"`
	SubjectTargetChars  *int    `toml:"subject_target_chars"`
}

// GlobalConfigPath returns the absolute path to the global stagehand config
// file, honoring XDG_CONFIG_HOME then falling back to $HOME/.config and always
// appending "stagehand/config.toml" (PRD §16.1/§16.2; FR34). It is the single
// source of truth for both readGlobalFile and the CLI `config path` / `config
// init` subcommands (P1.M7.T3.S2), so the two cannot diverge. It deliberately
// does NOT use os.UserConfigDir: that helper diverges to
// ~/Library/Application Support on macOS, whereas the PRD mandates a plain
// ~/.config fallback. Both XDG_CONFIG_HOME and HOME being empty is a hard
// error.
func GlobalConfigPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return "", errors.New("config: cannot resolve global config directory: both XDG_CONFIG_HOME and HOME are unset")
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "stagehand", "config.toml"), nil
}

// parseDuration parses the TOML/git-config timeout value, accepting BOTH a
// time.ParseDuration unit form ("120s", "2m") and a bare integer number of
// seconds ("90"). The §16.2 file uses "120s" while the §16.3 git-config
// example uses the bare "90"; both must work. A bare integer (with or without
// a trailing "s") is interpreted as seconds. The "120s" → time.Duration
// conversion is explicitly a loader concern, not Config's (see the package
// doc).
func parseDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	n, err := strconv.Atoi(strings.TrimSuffix(s, "s"))
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return time.Duration(n) * time.Second, nil
}

// parseFile reads and parses ONE config file into a fresh overlay (PRD §16.2).
// A MISSING file is NOT an error — it yields an empty overlay (all-nil
// pointers, nil ProviderOverrides) and a nil error, matching the FR34
// precedence contract (every layer is optional). A real read error (e.g.
// permission denied) or malformed TOML IS an error, wrapped with the file path
// for diagnostics. Each NON-NIL DTO pointer is copied straight into the
// overlay (the DTO is discarded after this call, so aliasing its pointer is
// fine); the [provider.*] map is copied verbatim (nil when absent). parseFile
// is the shared core reused by readGlobalFile, readRepoFile, AND by Load()
// (T3.S1) for the --config / STAGEHAND_CONFIG override path.
func parseFile(path string) (overlay, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return overlay{}, nil // missing file is NOT an error
		}
		return overlay{}, fmt.Errorf("config %s: %w", path, err)
	}

	var d fileDTO
	if err := toml.Unmarshal(data, &d); err != nil {
		return overlay{}, fmt.Errorf("config %s: %w", path, err)
	}

	var out overlay
	if d.Defaults != nil {
		out.Provider = d.Defaults.Provider
		out.Model = d.Defaults.Model
		if d.Defaults.Timeout != nil {
			dur, err := parseDuration(*d.Defaults.Timeout)
			if err != nil {
				return overlay{}, fmt.Errorf("config %s: %w", path, err)
			}
			out.Timeout = &dur
		}
		out.AutoStageAll = d.Defaults.AutoStageAll
		out.Verbose = d.Defaults.Verbose
		out.NoColor = d.Defaults.NoColor
	}
	if d.Generation != nil {
		out.MaxDiffBytes = d.Generation.MaxDiffBytes
		out.MaxMdLines = d.Generation.MaxMdLines
		out.MaxDuplicateRetries = d.Generation.MaxDuplicateRetries
		out.Output = d.Generation.Output
		out.StripCodeFence = d.Generation.StripCodeFence
		out.SubjectTargetChars = d.Generation.SubjectTargetChars
	}
	out.ProviderOverrides = d.Provider
	return out, nil
}

// readGlobalFile reads the global TOML config (PRD §16.1 layer 3) via
// GlobalConfigPath into a fresh overlay. It takes NO repoDir: the global path
// is environment-derived only. A missing global file yields an empty overlay
// and a nil error (parseFile's contract).
func readGlobalFile() (overlay, error) {
	p, err := GlobalConfigPath()
	if err != nil {
		return overlay{}, err
	}
	return parseFile(p)
}

// readRepoFile reads the per-repo TOML config (PRD §16.1 layer 4): the
// .stagehand.toml file inside repoDir. An empty repoDir means "inherit the
// stagehand process's current working directory" — filepath.Join("", ...) then
// yields ".stagehand.toml", the correct relative path. A missing file yields
// an empty overlay and a nil error (parseFile's contract).
func readRepoFile(repoDir string) (overlay, error) {
	return parseFile(filepath.Join(repoDir, ".stagehand.toml"))
}

// gitKey describes one known stagehand.* scalar git-config key (PRD §16.3).
// git-config keys are CAMELCASE (autoStageAll, maxDiffBytes, ...) — NOT the
// snake_case used in the TOML file. isBool marks the keys read via
// `git config --bool` (autoStageAll, verbose, noColor, stripCodeFence).
type gitKey struct {
	name   string // camelCase suffix after "stagehand."
	isBool bool   // read via --bool
}

// gitConfigKeys is the complete set of stagehand.* scalar keys recognized by
// readGitConfig (PRD §16.3). Git-config CANNOT express [provider.<name>]
// tables, so there is intentionally no key for provider overrides here. Order
// is irrelevant — each key is queried independently — but the slice gives a
// stable iteration order.
var gitConfigKeys = []gitKey{
	{name: "provider"},
	{name: "model"},
	{name: "timeout"},
	{name: "autoStageAll", isBool: true},
	{name: "verbose", isBool: true},
	{name: "noColor", isBool: true},
	{name: "maxDiffBytes"},
	{name: "maxMdLines"},
	{name: "maxDuplicateRetries"},
	{name: "subjectTargetChars"},
	{name: "output"},
	{name: "stripCodeFence", isBool: true},
}

// readGitConfig reads the per-repo git-config layer (PRD §16.1 layer 5; §16.3)
// by running `git config [--bool] --get stagehand.<key>` once per known
// stagehand.* scalar key, with cmd.Dir = repoDir. It builds each command as a
// []string (NEVER sh -c, PRD §19) and resolves the git binary once via
// exec.LookPath (fail fast if absent). A key that git reports as UNSET (exit
// code 1) leaves the corresponding overlay pointer nil and is NOT an error;
// any other non-zero exit or failure is a wrapped error. Bool keys are read
// with --bool and parsed via strconv.ParseBool. Timeout is parsed via
// parseDuration; the int keys via strconv.Atoi. ProviderOverrides stays nil:
// git-config cannot express [provider.<name>] tables.
//
// config does NOT import internal/git: readGitConfig uses os/exec directly to
// keep the config→provider one-way edge the only internal import (decisions.md
// key decision 1). Whether a value came from repo-LOCAL vs GLOBAL git config
// (relevant to the §19 trust notice) is determined by Load() (T3.S1); this
// reader just reads `git config --get` per the literal contract.
func readGitConfig(repoDir string) (overlay, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return overlay{}, fmt.Errorf("git config: git binary not found in PATH: %w", err)
	}

	var out overlay
	for _, k := range gitConfigKeys {
		args := []string{"config", "--get"}
		if k.isBool {
			args = append(args, "--bool")
		}
		args = append(args, "stagehand."+k.name)

		cmd := exec.Command(gitPath, args...)
		cmd.Dir = repoDir
		stdout, err := cmd.Output()
		if err != nil {
			// exit 1 = "key unset" per `git config --get`; treat as "not set
			// by this source" (leave the pointer nil) and continue — NOT an
			// error. Any other failure is a real error.
			var ee *exec.ExitError
			if errors.As(err, &ee) && ee.ExitCode() == 1 {
				continue
			}
			return overlay{}, fmt.Errorf("git config --get stagehand.%s: %w", k.name, err)
		}

		val := strings.TrimSpace(string(stdout))
		if err := applyGitKey(&out, k, val); err != nil {
			return overlay{}, fmt.Errorf("git config stagehand.%s: %w", k.name, err)
		}
	}
	return out, nil
}

// applyGitKey writes a single parsed git-config value into the matching
// overlay field, keeping the per-key dispatch out of the readGitConfig loop.
// val is the trimmed stdout of `git config --get`. Bool keys parse the
// "true"/"false" shape that --bool emits (strconv.ParseBool also accepts
// "1"/"0"/"t"/"f" defensively); int keys parse a base-10 integer; timeout
// parses via parseDuration. String keys take a pointer to a fresh copy of val.
func applyGitKey(out *overlay, k gitKey, val string) error {
	switch k.name {
	case "provider":
		v := val
		out.Provider = &v
	case "model":
		v := val
		out.Model = &v
	case "timeout":
		dur, err := parseDuration(val)
		if err != nil {
			return err
		}
		out.Timeout = &dur
	case "autoStageAll":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		out.AutoStageAll = &b
	case "verbose":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		out.Verbose = &b
	case "noColor":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		out.NoColor = &b
	case "maxDiffBytes":
		n, err := strconv.Atoi(val)
		if err != nil {
			return err
		}
		out.MaxDiffBytes = &n
	case "maxMdLines":
		n, err := strconv.Atoi(val)
		if err != nil {
			return err
		}
		out.MaxMdLines = &n
	case "maxDuplicateRetries":
		n, err := strconv.Atoi(val)
		if err != nil {
			return err
		}
		out.MaxDuplicateRetries = &n
	case "subjectTargetChars":
		n, err := strconv.Atoi(val)
		if err != nil {
			return err
		}
		out.SubjectTargetChars = &n
	case "output":
		v := val
		out.Output = &v
	case "stripCodeFence":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		out.StripCodeFence = &b
	}
	return nil
}
