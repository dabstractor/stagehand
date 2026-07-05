package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/dustin/stagehand/internal/cmd"
	"github.com/dustin/stagehand/internal/exitcode"
	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/signal"
)

// version is injected at build time via -ldflags "-X main.version=…" (Makefile VERSION, default "dev").
// Bare `go install` and a default `make install` both leave it "dev" (no VERSION override), which is
// useless for telling builds apart. resolveVersion() enriches the bare "dev" with the VCS info Go
// 1.18+ embeds automatically (vcs.revision + vcs.modified) via debug.ReadBuildInfo, so EVERY build —
// ldflags or not, Makefile or bare `go install` — self-identifies as e.g. "dev (19f4df7-dirty)".
// A tagged release (goreleaser / VERSION=vX.Y.Z) keeps its real version string verbatim.
// P1.M4.T2 will replace context.Background() with a signal-aware context; S1 uses the baseline.
var version = "dev"

// resolveVersion turns the ldflags-injected version into a display string. A real release value is
// returned as-is; the bare "dev" default is enriched with the embedded VCS revision (+ "-dirty" when
// the source tree was modified at build time). Returns "dev" unchanged if no VCS info is available
// (e.g. a build with -buildvcs=false, or built from a non-VCS tarball).
func resolveVersion(v string) string {
	if v != "dev" {
		return v // tagged release (goreleaser / make VERSION=vX.Y.Z)
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	var rev, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if rev == "" {
		return "dev" // no embedded VCS (built from a non-git source or -buildvcs=false)
	}
	if len(rev) > 7 {
		rev = rev[:7] // short SHA, matching `git describe --always` / GitHub UX
	}
	return "dev (" + rev + dirty + ")"
}

func main() {
	cmd.Version = resolveVersion(version) // cobra's --version prints this (short-circuits before config load)
	ctx, _ := signal.Install(context.Background(), signal.Options{
		RescueFormat: generate.FormatRescue,
		Out:          os.Stderr,
	})
	err := cmd.Execute(ctx)
	code := exitcode.For(err)
	if err != nil && err.Error() != "" {
		fmt.Fprintf(os.Stderr, "stagehand: %v\n", err)
	}
	os.Exit(code)
}
