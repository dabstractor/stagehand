// Package generate — work-description mode read/answer loop (PRD §9.26 FR-W3–FR-W7).
//
// This is the read-on-demand transport layer for work-description mode, the description-first analogue
// of multiturn.go's Run. Where multi-turn DELIVERS a captured payload across N+1 turns (push), work-
// description mode PULLS file diffs the model asks for (READ <path>) across a bounded number of rounds
// (FR-W4/FR-W6). Both reuse the SAME provider session machinery (session_mode="append", FR-T8/T9):
//
//   - Turn 1:   the description-first payload (work description + skeleton + context), rendered with the
//     system prompt (turn-1-only flag, like multi-turn).
//   - Rounds 2..K: each model response is scanned for `READ <path>` lines (FR-W3); the requested staged
//     diffs are appended as the next turn's payload (chunked per FR-W5). A response with no valid READ
//     line is the commit-message candidate (FR-W7) → the loop returns.
//   - Forced conclusion: after cfg.WorkDescReadRounds rounds, the next turn refuses further reads and
//     demands the commit message (FR-W6); any READ lines thereafter are ignored.
//
// Failure handling mirrors multiturn.Run's FR-T7: ANY turn's Execute error or RenderMultiTurn error
// aborts the loop and returns the raw cause (the caller maps it to &RescueError{Cause: cause}). The
// final message is parsed via the EXISTING ParseOutput pipeline (FR-W7); the caller runs dedupe. This
// loop does NOT cascade into multi-turn fallback (§9.24) — FR-W7: that would re-deliver the whole diff.
package generate

import (
	"context"
	"fmt"
	"strings"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/provider"
)

// readChunkTokenCap is the per-call diff chunk cap (PRD §9.26 FR-W5: "default ~16K tokens, internal").
// A staged file diff exceeding this is returned in chunks; re-requesting the same path returns the
// next chunk (the implicit cursor is the per-file byte offset tracked in readState.offsets).
const readChunkTokenCap = 16000

// refuseReadsFmt is the forced-conclusion turn payload (FR-W6, verbatim with N interpolated). Emitted
// when the round cap is reached: no further reads are answered and the model must output the message.
const refuseReadsFmt = "Read budget exhausted (%d/%d) — output the commit message now. Any further READ lines will be ignored."

// readState tracks the per-session read loop state: the round count (against cfg.WorkDescReadRounds)
// and the per-file byte offset (the implicit cursor, FR-W5 — re-requesting a path returns the next chunk).
type readState struct {
	rounds  int            // number of model responses processed that contained ≥1 READ request
	N       int            // the round cap (cfg.WorkDescReadRounds)
	offsets map[string]int // path → byte offset into its staged diff (the implicit cursor)
}

// RunWorkDescription drives the description-first read/answer loop (PRD §9.26 FR-W4–FR-W7). It is the
// transport layer invoked by CommitStaged when cfg.WorkDescription is non-empty (the sole caller).
// The model sees the work description + skeleton on turn 1 and pulls file diffs via `READ <path>`;
// a response with no valid READ line is the commit-message candidate (FR-W7).
//
// sysPrompt is the ALREADY-EXTENDED work-description system prompt (prompt.BuildWorkDescSystemPrompt);
// payload is the description-first user payload (prompt.BuildWorkDescPayload); skeleton is the numstat
// skeleton (the READ-able path set, git.StagedNumstatSkeleton). msgModel/msgReasoning resolve the
// provider/model like the default path. Returns (msg, ok, cause): ok==false (no valid message after the
// round cap) is NOT a cause — the caller decides rescue per FR-W7 (the existing §9.10 rescue).
//
// Failure handling (FR-T7 parity): ANY turn's Execute error OR RenderMultiTurn error ⇒ RunWorkDescription
// aborts and returns the raw cause as `cause`. A non-"append" provider yields a turn-1 RenderMultiTurn
// error (its session_mode gate), surfaced as cause — provider support is identical to §9.24 (FR-W4).
func RunWorkDescription(ctx context.Context, deps Deps, cfg config.Config, manifest provider.Manifest,
	sysPrompt, payload, skeleton, msgModel, msgReasoning string) (msg string, ok bool, cause error) {

	// FR-R7: resolve the message role's per-turn timeout so [role.message].timeout / --message-timeout
	// bound each work-description read-loop turn instead of the flat cfg.Timeout. With no per-role
	// override ResolveRoleTimeout returns cfg.Timeout (the message role has no built-in) — behavior-
	// preserving by default.
	msgTimeout := config.ResolveRoleTimeout("message", cfg)

	// Mint a fresh, one-run-scope session id (FR-T6 parity — never resumed on a later run).
	sessionID := newSessionID()

	// Turn 1: system prompt + the description-first payload. RenderMultiTurn emits the
	// system_prompt_flag only on turn 1 (its own turn-1-only gate).
	spec, rerr := manifest.RenderMultiTurn(msgModel, sysPrompt, payload, msgReasoning, sessionID, 1)
	if rerr != nil {
		return "", false, rerr // non-append provider ⇒ RenderMultiTurn's session_mode gate; surface as cause
	}
	out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
	if execErr != nil {
		return "", false, execErr // FR-T7 parity: any turn error/timeout/cancel/non-zero-exit aborts
	}

	st := readState{
		N:       cfg.WorkDescReadRounds,
		offsets: make(map[string]int),
	}
	if st.N < 1 {
		st.N = 1 // defensive (the prompt already stated ≥1; guarantee termination, FR-W6)
	}

	// Drives turns 2.. until the model emits a message (no READ) or the round cap forces conclusion.
	// Bounded by an absolute safety ceiling (round cap + a handful of forced-conclusion turns) so a
	// misbehaving model that keeps emitting READ lines after the budget cannot loop forever.
	for turn := 2; ; turn++ {
		paths := parseReadLines(out, skeleton)
		if len(paths) == 0 {
			// FR-W7: a response with no valid READ line is the commit-message candidate.
			m, parseOK, _ := provider.ParseOutput(out, manifest)
			return m, parseOK, nil
		}

		// Budget check (FR-W6): once the round cap is reached, refuse further reads and demand the message.
		if st.rounds >= st.N {
			refuse := fmt.Sprintf(refuseReadsFmt, st.rounds, st.N)
			spec, rerr := manifest.RenderMultiTurn(msgModel, "", refuse, msgReasoning, sessionID, turn)
			if rerr != nil {
				return "", false, rerr
			}
			out2, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
			if execErr != nil {
				return "", false, execErr
			}
			// The forced-conclusion turn's response: strip READ lines, parse the message (FR-W6).
			m, parseOK, _ := provider.ParseOutput(stripReadLines(out2), manifest)
			return m, parseOK, nil
		}

		st.rounds++
		// Build the answer turn: each requested staged diff (chunked per FR-W5), non-staged noted.
		answer := buildReadAnswer(ctx, deps.Git, cfg, deps.Excludes, paths, &st)
		spec, rerr = manifest.RenderMultiTurn(msgModel, "", answer, msgReasoning, sessionID, turn)
		if rerr != nil {
			return "", false, rerr
		}
		out, _, execErr = provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
		if execErr != nil {
			return "", false, execErr // FR-T7 parity
		}
	}
}

// parseReadLines extracts the READ <path> requests from a model response (PRD §9.26 FR-W3). It is the
// loose, forgiving parser: case-insensitive verb, whitespace- and punctuation-forgiving, one path per
// line or comma-separated, several per response. skeleton is the rendered numstat block — its paths are
// the staged set; only paths IN that set are returned (non-staged/unrecognized are ignored by the caller
// with a note, FR-W3). Returns the de-duplicated, order-preserved list of staged paths to fulfill.
//
// The parser scans line-by-line; a line whose first token (uppercased, trimmed of surrounding
// punctuation) is "READ" treats the rest of the line as one or more comma/space-separated paths. Lines
// with no READ verb are part of the candidate message (FR-W7: a response with no valid READ line is the
// message). An empty skeleton ⇒ no paths are valid (nothing staged to read).
func parseReadLines(response, skeleton string) []string {
	staged := skeletonPaths(skeleton)
	if len(staged) == 0 {
		return nil // nothing staged ⇒ no READ is fulfillable
	}
	seen := make(map[string]bool)
	var out []string
	for _, line := range strings.Split(response, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Tokenize: verb = first whitespace-delimited token, stripped of leading punctuation.
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		verb := strings.ToUpper(strings.TrimLeft(fields[0], " \t,.:;!?-*/`\"'"))
		if verb != "READ" {
			continue // not a READ line — part of the candidate message (FR-W7)
		}
		// The remainder of the line (after the verb) carries one or more comma/space-separated paths.
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0]))
		if rest == "" {
			continue
		}
		for _, raw := range splitPaths(rest) {
			p := normalizePath(raw)
			if p == "" || seen[p] {
				continue
			}
			if _, ok := staged[p]; ok {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}

// stripReadLines removes every `READ <path>` line from a model response, returning the remainder
// (the candidate commit message). It is the FR-W7 corollary to parseReadLines: a response that is
// ONLY READ lines has no message after stripping → ParseOutput returns ok=false. Used by the forced-
// conclusion turn (FR-W6: "any READ lines thereafter are ignored and only the message is parsed").
func stripReadLines(response string) string {
	var b strings.Builder
	for _, line := range strings.Split(response, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			b.WriteByte('\n')
			continue
		}
		fields := strings.Fields(trimmed)
		verb := strings.ToUpper(strings.TrimLeft(fields[0], " \t,.:;!?-*/`\"'"))
		if verb == "READ" {
			continue // drop READ lines — only the message remains
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

// splitPaths splits a READ line's remainder into individual path tokens on commas, semicolons, and
// whitespace (FR-W3: "one path per line or comma-separated"). Backticks/quotes around a path are
// stripped by normalizePath.
func splitPaths(s string) []string {
	s = strings.ReplaceAll(s, ",", " ")
	s = strings.ReplaceAll(s, ";", " ")
	return strings.Fields(s)
}

// normalizePath canonicalizes a READ target: trims surrounding whitespace/punctuation/quotes so
// "`foo.go`" and "foo.go," both match "foo.go" (FR-W3: whitespace- and punctuation-forgiving). Returns
// "" for an empty/whitespace-only token.
func normalizePath(raw string) string {
	return strings.Trim(raw, " \t,.:;!?-*/`\"'[]()<>")
}

// skeletonPaths parses the rendered numstat skeleton into a set of staged paths (the READ-able menu).
// The skeleton is the block git.StagedNumstatSkeleton renders:
//
//	Change summary (numstat: added\tdeleted\tpath):
//	<added>\t<deleted>\t<path>
//	...
//
// Each data line is "<added>\t<deleted>\t<path>" (binary rows render "-\t-\t<path>"). The path is the
// 3rd tab-field. The header line and any blank/non-conforming line are skipped. Returns nil for an
// empty/non-conforming skeleton.
func skeletonPaths(skeleton string) map[string]bool {
	if skeleton == "" {
		return nil
	}
	set := make(map[string]bool)
	for _, line := range strings.Split(skeleton, "\n") {
		if line == "" || strings.HasPrefix(line, "Change summary") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		path := strings.TrimSpace(fields[len(fields)-1])
		if path != "" {
			set[path] = true
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

// buildReadAnswer constructs the answer turn payload for a set of READ requests (PRD §9.26 FR-W3/FR-W5).
// For each requested path: if it is staged and has a (remaining) diff chunk, that chunk is appended
// (labeled "part i/N" when chunked, FR-W5); if it is not staged or fully read, a short note is appended
// (FR-W3: "<path> is not in the staged changes" / FR-W5: "end of diff"). The per-file byte offset
// (st.offsets) is the implicit cursor: re-requesting a path returns the NEXT chunk (FR-W5).
func buildReadAnswer(ctx context.Context, g git.Git, cfg config.Config, excludes, paths []string, st *readState) string {
	opts := git.StagedDiffOptions{
		MaxDiffBytes:     cfg.MaxDiffBytes,
		MaxMDLines:       cfg.MaxMdLines,
		BinaryExtensions: cfg.BinaryExtensions,
		Excludes:         excludes,
		DiffContext:      cfg.DiffContextValue(),
	}
	var b strings.Builder
	for _, p := range paths {
		diff, err := g.StagedFileDiff(ctx, p, opts)
		if err != nil || diff == "" {
			// Either not staged, fully read (cursor exhausted), or a read error. Note it (FR-W3/FR-W5).
			// On a read error, treat the file as unreadable and note it (best-effort; the loop continues).
			fmt.Fprintf(&b, "%s is not in the staged changes (or has no further diff).\n\n", p)
			continue
		}
		chunk, total, advance := nextChunk(diff, st.offsets[p])
		if total <= 1 {
			fmt.Fprintf(&b, "%s:\n%s\n\n", p, chunk)
		} else {
			part := (st.offsets[p] / chunkRuneBudget()) + 1
			fmt.Fprintf(&b, "%s — part %d of %d; READ %s again for the next part:\n%s\n\n",
				p, part, total, p, chunk)
		}
		st.offsets[p] += advance
	}
	return strings.TrimRight(b.String(), "\n")
}

// nextChunk returns the chunk of diff starting at offset (the implicit cursor), the total number of
// chunks the full diff would span at the per-call cap, and the byte advance to the next chunk boundary
// (FR-W5). When offset >= len(diff) the chunk is "" and advance is 0 (the caller notes "end of diff").
// Boundaries hug newline edges so a hunk is never split mid-line; the cap is readChunkTokenCap tokens
// realized as readChunkTokenCap*4 runes (the rune-equivalent of git.EstimateTokens's ceil(runes/4),
// matching multiturn.go's chunk sizing discipline).
func nextChunk(diff string, offset int) (chunk string, total int, advance int) {
	if offset >= len(diff) {
		return "", 1, 0 // cursor exhausted (FR-W5 end-of-diff); the caller notes it
	}
	budget := chunkRuneBudget()
	total = chunkCount(diff, budget)
	end := advanceRunes(diff, offset, budget)
	// Anchor FORWARD to the next newline so a line is never split mid-line.
	if i := strings.IndexByte(diff[end:], '\n'); i >= 0 {
		end += i + 1
	} else {
		end = len(diff)
	}
	if end > len(diff) {
		end = len(diff)
	}
	return diff[offset:end], total, end - offset
}

// chunkRuneBudget is the per-chunk rune budget (readChunkTokenCap tokens realized as runes).
func chunkRuneBudget() int { return readChunkTokenCap * 4 }

// chunkCount returns the number of chunks diff would span at the rune budget (mirrors multiturn's
// window+forward-anchor discipline, approximated here by rune-windowing without forward-anchoring —
// the exact boundary is computed in nextChunk; this is the label count, FR-W5 "part i of N").
func chunkCount(diff string, runeBudget int) int {
	if runeBudget < 1 {
		runeBudget = 1
	}
	if len(diff) == 0 {
		return 1
	}
	n := 0
	for offset := 0; offset < len(diff); {
		end := advanceRunes(diff, offset, runeBudget)
		if i := strings.IndexByte(diff[end:], '\n'); i >= 0 {
			end += i + 1
		} else {
			end = len(diff)
		}
		n++
		offset = end
	}
	if n == 0 {
		n = 1
	}
	return n
}
