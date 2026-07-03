// Package git provides shell-free access to the git binary.
//
// Binary detection and placeholder generation (PRD §9.1 FR3a/b, FR3c):
//
// FR3a — Two-signal binary detection. Git's numstat output uses a content-sniffing heuristic
// (added=="-" && deleted=="-") as the PRIMARY binary signal. However, git sniffs by CONTENT,
// not extension: a file named "report.png" whose content is plain text is NOT flagged by numstat
// (it emits "1\t0\treport.png"). The SUPPLEMENTAL signal is a built-in extension denylist
// (defaultBinaryExtensions, 36 entries). A path is binary iff numstat says so OR the extension
// matches — the UNION is strictly more robust than either signal alone. The denylist is
// overridable/extendable via the binary_extensions config (wired by StagedDiff in S2).
//
// FR3b — One-line placeholder. Instead of the useless "Binary files a/… and b/… differ" hunk,
// binary files emit a single placeholder line: "<status>\t[binary] <path>", where <status> comes
// from git name-status (A/M/D/T/R100).
//
// FR3c — Applies in every diff path. The variadic diffArgs parameter serves all three consumers:
//   - StagedDiff (S2):     diffArgs = ["--cached"]
//   - WorkingTreeDiff:      diffArgs = []
//   - TreeDiff:             diffArgs = [treeA, treeB]
//
// FR-X4 — User-exclude placeholder. Files matched by the user's exclude pathspecs (from
// .stagehandignore, cfg.Exclude, or --exclude) emit a "<status>\t[excluded] <path>" placeholder
// instead of their diff body. The placeholder signals the file changed while hiding its contents.
// Detection uses a set-difference probe: git diff --name-only with the exclude pathspecs yields the
// surviving paths; the complement against all changed paths is the excluded set. Empty excludes ⇒
// no probe, no placeholders (zero overhead). Binary+excluded files get [binary] only (binary wins).
//
// This file contains the shared primitives; wiring into StagedDiff is S2's scope.
package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// defaultBinaryExtensions is the FR3a built-in binary extension denylist, as a set for O(1) lookup.
// Entries are lowercase, WITHOUT a leading dot. Git's numstat content-sniffing is the PRIMARY binary
// signal; this list SUPPLEMENTS it for files git may misclassify. Overridable/extendable via the
// binary_extensions config (wired by StagedDiff/S2 + config); isBinaryByExtension accepts extraExts.
var defaultBinaryExtensions = map[string]bool{
	// images
	"png": true, "jpg": true, "jpeg": true, "gif": true, "webp": true, "bmp": true, "ico": true, "svgz": true,
	// documents / archives
	"pdf": true, "zip": true, "tar": true, "gz": true, "tgz": true, "bz2": true, "7z": true, "rar": true,
	// compiled / native
	"exe": true, "dll": true, "so": true, "dylib": true, "o": true, "class": true, "jar": true, "war": true, "wasm": true, "a": true,
	// media
	"mp3": true, "mp4": true, "mov": true, "avi": true, "mkv": true, "flac": true, "ogg": true, "wav": true,
	// fonts
	"ttf": true, "otf": true, "woff": true, "woff2": true,
}

// isBinaryByExtension reports whether path's terminal extension is in defaultBinaryExtensions or in the
// caller-supplied extraExts (the binary_extensions config override; entries are dot-tolerant and
// case-insensitive). Extension lookup is case-insensitive and ignores a missing/empty extension.
// Pure function: no I/O. (PRD §9.1 FR3a.)
func isBinaryByExtension(path string, extraExts []string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if ext == "" {
		return false
	}
	if defaultBinaryExtensions[ext] {
		return true
	}
	for _, e := range extraExts {
		if strings.ToLower(strings.TrimSpace(strings.TrimPrefix(e, "."))) == ext {
			return true
		}
	}
	return false
}

// binaryPlaceholderLine returns the FR3b one-line placeholder for a binary file:
// "<status>\t[binary] <path>". status is the raw git name-status code (A/M/D/T, or "R100" for a
// rename). Pure function: no I/O. (PRD §9.1 FR3b.)
func binaryPlaceholderLine(status, path string) string {
	return status + "\t[binary] " + path
}

// excludedPlaceholderLine returns the FR-X4 one-line placeholder for a user-excluded file:
// "<status>\t[excluded] <path>". Mirrors binaryPlaceholderLine; distinguishable by tag
// ([excluded] vs [binary]). status is the raw git name-status code. Pure function: no I/O.
// (PRD §9.18 FR-X4.)
func excludedPlaceholderLine(status, path string) string {
	return status + "\t[excluded] " + path
}

// detectBinaryFiles returns the set of binary paths among the files matched by
// `git diff <diffArgs>`. Detection is the FR3a two-signal UNION: (a) git numstat emits
// "-\t-\t<path>" for files it content-sniffs as binary (PRIMARY), (b) the extension denylist
// (isBinaryByExtension) catches files git may misclassify (SUPPLEMENTAL). diffArgs selects the
// diff domain and is forwarded verbatim: ["--cached"] (staged, S2), [] (working tree,
// P2.M2.T2.S2), or [treeA, treeB] (tree-to-tree, P2.M2.T1.S2) — serving all FR3c paths.
// Read-only w.r.t. refs/index. (PRD §9.1 FR3a.)
func (g *gitRunner) detectBinaryFiles(ctx context.Context, diffArgs ...string) (map[string]bool, error) {
	args := make([]string, 0, 1+len(diffArgs)+1)
	args = append(args, "diff")
	args = append(args, diffArgs...)
	args = append(args, "--numstat")
	stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return nil, err // infrastructural (git missing / ctx cancel / start failure) — propagate unwrapped
	}
	if code != 0 {
		return nil, fmt.Errorf("git diff --numstat: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	binary := make(map[string]bool)
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue // empty line (trailing "\n") or malformed — skip (do NOT TrimSpace the line; preserve paths)
		}
		added, deleted, path := fields[0], fields[1], fields[2]
		if (added == "-" && deleted == "-") || isBinaryByExtension(path, nil) {
			binary[path] = true // FR3a union: numstat content-sniff OR extension denylist
		}
	}
	return binary, nil
}

// fileStatuses returns a map of path → git change-status (A/M/D/T, or "R100"/"C80" for
// rename/copy) for the files matched by `git diff <diffArgs> --name-status`. The status is the
// source of the <status> in the FR3b placeholder (PRD §9.1 FR3b). For a rename/copy the map is
// keyed by the DESTINATION path (fields[len-1]); the source is dropped (the placeholder carries
// the new name). diffArgs is forwarded verbatim, same contract as detectBinaryFiles.
// Read-only w.r.t. refs/index. (PRD §9.1 FR3b.)
func (g *gitRunner) fileStatuses(ctx context.Context, diffArgs ...string) (map[string]string, error) {

	args := make([]string, 0, 1+len(diffArgs)+1)
	args = append(args, "diff")
	args = append(args, diffArgs...)
	args = append(args, "--name-status")
	stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("git diff --name-status: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	statuses := make(map[string]string)
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 2 {
			continue
		}
		statuses[fields[len(fields)-1]] = fields[0] // destination path → status (R100 line: fields[2]→"R100")
	}
	return statuses, nil
}

// detectExcludedStatuses returns the subset of allStatuses (path→status) whose paths the USER exclude
// pathspecs remove from `git diff <diffArgs>`. It runs `git diff <diffArgs> --name-only -- <excludes>`
// for the SURVIVING paths, then returns allStatuses minus those (the excluded set, statuses preserved).
// Empty excludes ⇒ (nil, nil) with NO git call — zero overhead in the common case. diffArgs selects
// the diff domain, variadic, identical to detectBinaryFiles: "--cached" (staged), nothing (working
// tree), treeA treeB (tree-to-tree). Read-only w.r.t. refs/index. (PRD §9.18 FR-X4 placeholder source.)
func (g *gitRunner) detectExcludedStatuses(ctx context.Context, allStatuses map[string]string,
	excludes []string, diffArgs ...string) (map[string]string, error) {
	if len(excludes) == 0 {
		return nil, nil // no user exclusions ⇒ no placeholders, no git call (zero overhead)
	}
	args := []string{"diff"}
	args = append(args, diffArgs...)
	args = append(args, "--name-only", "--")
	args = append(args, excludes...)
	stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return nil, err // infrastructural (git missing / ctx cancel / start failure) — propagate unwrapped
	}
	if code != 0 {
		return nil, fmt.Errorf("git diff (exclude probe): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	surviving := make(map[string]bool, len(allStatuses))
	for _, line := range strings.Split(stdout, "\n") {
		if p := strings.TrimSpace(line); p != "" {
			surviving[p] = true
		}
	}
	excluded := make(map[string]string)
	for path, st := range allStatuses {
		if !surviving[path] { // present in all-changed but removed by the exclude pathspecs
			excluded[path] = st
		}
	}
	return excluded, nil
}
