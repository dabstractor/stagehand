// Package exclude implements the .stagecoachignore loader and the gitignore-glob →
// :(exclude,glob) pathspec translator (PRD §9.18 FR-X1, FR-X2, FR-X5).
//
// A repo can carry a .stagecoachignore file of gitignore-style globs. Every user exclusion
// glob (from .stagecoachignore and from cfg.Exclude) is faithfully translated into a git
// :(exclude,glob) pathspec that behaves like gitignore (* stops at /, ** spans components,
// anchoring and dir/ honored). The single entry point is ResolveExcludePathspecs, which
// returns the ready-to-use pathspec union consumed by P1.M1.T2.S1 as
// StagedDiffOptions.Excludes.
//
// This package produces user sources (b) .stagecoachignore and (c)+(d) cfg.Exclude ONLY.
// The built-in denylist (FR-X1 source (a)) lives in internal/git/git.go and is applied
// separately. It is NOT folded into the output of ResolveExcludePathspecs.
package exclude

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/ui"
)

// StagecoachIgnoreFile is the fixed repo-root filename (PRD §9.18 FR-X1b/FR-X2).
const StagecoachIgnoreFile = ".stagecoachignore"

// TranslatePattern converts one gitignore-style glob (relative to the repo root) into a
// single git :(exclude,glob)<core> pathspec (PRD §9.18 FR-X2; architecture/external_deps.md §4).
//
// Translation rules (research/gitignore-to-pathspec.md):
//  1. Strip a leading "/" (record anchored=true).
//  2. Strip a trailing "/" (record dirOnly=true).
//  3. hasInternalSlash = remaining contains "/" (middle separator anchors).
//  4. core = dirOnly ? p+"/**" : p ; if !anchored && !hasInternalSlash: core = "**/"+core.
//  5. Return ":(exclude,glob)"+core.
func TranslatePattern(glob string) string {
	p := glob

	anchored := strings.HasPrefix(p, "/")
	if anchored {
		p = p[1:]
	}

	dirOnly := strings.HasSuffix(p, "/")
	if dirOnly {
		p = strings.TrimSuffix(p, "/")
	}

	hasInternalSlash := strings.Contains(p, "/")

	core := p
	if dirOnly {
		core += "/**"
	}
	if !anchored && !hasInternalSlash {
		core = "**/" + core
	}

	return ":(exclude,glob)" + core
}

// LoadStagecoachIgnore reads <repoRoot>/.stagecoachignore and returns the raw (untranslated) globs.
// Blank and #-comment lines are ignored; ! (negation) lines are skipped with a VerboseWarn
// (FR-X2: pathspecs cannot un-exclude). A missing file returns (nil, nil). Only a non-ENOENT read
// error is returned as err.
func LoadStagecoachIgnore(repoRoot string, v *ui.Verbose) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, StagecoachIgnoreFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil // FR-X2: missing file = no-op
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", StagecoachIgnoreFile, err)
	}

	var globs []string
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "!") {
			v.VerboseWarn("ignoring unsupported negation pattern in .stagecoachignore: " + line)
			continue // FR-X2: pathspecs have no un-exclude
		}
		globs = append(globs, line)
	}
	return globs, nil
}

// ResolveExcludePathspecs returns the translated UNION of the .stagecoachignore globs and cfg.Exclude
// (the S1-resolved raw union of [generation].exclude and --exclude/-x), each run through
// TranslatePattern. Order: ignore-file globs first, then cfg.Exclude. An empty union yields an
// empty (nil) slice with nil error — a valid no-exclusions result. Consumed by P1.M1.T2.S1 as
// StagedDiffOptions.Excludes.
func ResolveExcludePathspecs(cfg config.Config, repoRoot string, v *ui.Verbose) ([]string, error) {
	globs, err := LoadStagecoachIgnore(repoRoot, v)
	if err != nil {
		return nil, err
	}

	var out []string
	for _, g := range globs {
		out = append(out, TranslatePattern(g))
	}
	for _, g := range cfg.Exclude {
		out = append(out, TranslatePattern(g))
	}
	return out, nil
}
