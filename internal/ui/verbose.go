package ui

import (
	"fmt"
	"io"
	"strings"
)

// Verbose is Stagecoach's --verbose diagnostics sink (PRD §9.13 FR50, §15.2, §19). When ON, it prints
// the resolved provider command, the raw agent stdout, and each retry attempt to a writer (the CLI's
// stderr) with a "DEBUG: " prefix (the commit-pi convention named in the work-item contract). When OFF
// (the default), or when the receiver is nil, or when the writer is nil, EVERY method is a no-op
// (zero bytes, zero allocations) — so callers thread a *Verbose (possibly nil) and call methods
// unconditionally with no nil guards.
//
// SECURITY (PRD §19): VerboseCommand logs ARGV ONLY (Command+Args). It NEVER logs spec.Env (which
// carries *_API_KEY credentials). Stdin contents are NOT logged at VERBOSE=1 (deferred to a future
// VERBOSE=2 — see D9; Config.Verbose is a bool, so VERBOSE=2 is currently un-parseable and out of scope).
//
// The writer is INJECTABLE: the CLI passes cmd.ErrOrStderr() (stderr); a library consumer of
// pkg/stagecoach passes its own writer or nil. This keeps the library side-effect-free by default
// (it never writes to os.Stderr directly). Sibling to output.go (P1.M4.T3.S1's ↳/color layer); this
// file owns ONLY verbose diagnostics.
type Verbose struct {
	w  io.Writer // destination (stderr in prod, *bytes.Buffer in tests); nil ⇒ no-op
	on bool      // cfg.Verbose — resolved by config.Load across all 7 layers
}

// NewVerbose constructs a Verbose sink. on=false (the common case) ⇒ every method is a no-op. w may be
// nil ⇒ every method is a no-op (the library default: a caller that supplies no writer gets silence
// even if cfg.Verbose is true). From the CLI: ui.NewVerbose(stderr, cfg.Verbose). From tests:
// ui.NewVerbose(&buf, true|false).
func NewVerbose(w io.Writer, on bool) *Verbose {
	return &Verbose{w: w, on: on}
}

// VerboseCommand prints the resolved provider command (PRD §9.13 FR50). cmd is the rendered argv
// (Command + Args, space-joined) — the CALLER builds it from CmdSpec; this method never touches Env.
// Format: "DEBUG: command: <cmd>\n". No-op when v==nil, v.w==nil, or !v.on.
func (v *Verbose) VerboseCommand(cmd string) {
	if v == nil || v.w == nil || !v.on {
		return
	}
	fmt.Fprintln(v.w, "DEBUG: command: "+cmd)
}

// VerboseRawOutput prints the raw agent stdout (PRD §9.13 FR50 — "the raw agent stdout"), pre-parse
// and pre-fence-strip, so a user can see exactly what the model returned. Format: "DEBUG: raw output:\n"
// followed by the output verbatim, with a trailing newline ensured (so the next DEBUG line is clean).
// No-op when v==nil, v.w==nil, or !v.on.
//
// NOTE: stdin contents are NOT logged at VERBOSE=1 (deferred to a future VERBOSE=2 — would require
// Config.Verbose to become an int; currently it is a bool and ParseBool("2") errors).
func (v *Verbose) VerboseRawOutput(output string) {
	if v == nil || v.w == nil || !v.on {
		return
	}
	fmt.Fprint(v.w, "DEBUG: raw output:\n")
	fmt.Fprint(v.w, output)
	if !strings.HasSuffix(output, "\n") {
		fmt.Fprint(v.w, "\n")
	}
}

// VerboseStderr prints the provider's captured stderr — the stderr twin of VerboseRawOutput.
// Providers (pi, opencode, …) emit their real failure diagnostics to STDERR, not stdout: upstream
// errors, rate-limit notices, context-length rejections, auth failures. Without this, a run whose
// model rejected the request returns EMPTY stdout and surfaces only as an unexplained "parse failed
// (no valid commit message)", with the actual reason sitting in a captured-then-discarded stderr
// buffer. Format: "DEBUG: stderr:\n" + stderr verbatim (trailing newline ensured). No-op when v==nil,
// v.w==nil, !v.on, OR stderr is empty (a clean run stays clean). Like VerboseRawOutput this is the
// provider's OWN output — surfacing it falls squarely within --verbose's "raw output" purpose
// (PRD §9.13 FR50); it is not a new diagnostic scope.
func (v *Verbose) VerboseStderr(stderr string) {
	if v == nil || v.w == nil || !v.on || stderr == "" {
		return
	}
	fmt.Fprint(v.w, "DEBUG: stderr:\n")
	fmt.Fprint(v.w, stderr)
	if !strings.HasSuffix(stderr, "\n") {
		fmt.Fprint(v.w, "\n")
	}
}

// VerbosePayload prints the size of the payload being delivered to the provider (PRD §9.13 FR50 —
// diagnostics). bytes is the stdin/payload length the executor is about to pipe (spec.Stdin for
// stdin-delivery providers; the trailing positional/flag arg length otherwise). Stagecoach's token
// budgeting (FR3d/FR3i) is the difference between a payload that fits and one the model rejects, so
// surfacing the shipped size lets a user see at a glance whether the gate ran and whether the chars/4
// estimate is in the right neighborhood — without this, a silently-ignored token_limit (e.g. a key in
// the wrong TOML section) looks identical to a working one. Only the SIZE is logged, never the
// contents (PRD §19). Format: "DEBUG: payload: <bytes> bytes (~<tokens> tokens est)\n". No-op when
// v==nil, v.w==nil, !v.on, or bytes<=0 (positional/flag delivery with no measured payload → skip).
func (v *Verbose) VerbosePayload(bytes int) {
	if v == nil || v.w == nil || !v.on || bytes <= 0 {
		return
	}
	fmt.Fprintf(v.w, "DEBUG: payload: %d bytes (~%d tokens est)\n", bytes, (bytes+3)/4)
}

// VerboseWarn prints a general warning for diagnostics such as unsupported .stagecoachignore
// negation patterns (PRD §9.18 FR-X2). Format: "DEBUG: <msg>\n". No-op when v==nil, v.w==nil, or !v.on.
func (v *Verbose) VerboseWarn(msg string) {
	if v == nil || v.w == nil || !v.on {
		return
	}
	fmt.Fprintln(v.w, "DEBUG: "+msg)
}

// VerboseRetry prints a retry attempt and its reason (PRD §9.13 FR50 — "each retry attempt"). attempt
// is 1-based (matches Appendix B.4 "Attempt 1"). Format: "DEBUG: attempt <n>: <reason>\n". No-op when
// v==nil, v.w==nil, or !v.on.
func (v *Verbose) VerboseRetry(attempt int, reason string) {
	if v == nil || v.w == nil || !v.on {
		return
	}
	fmt.Fprintf(v.w, "DEBUG: attempt %d: %s\n", attempt, reason)
}

// RoleLine is one role's resolved (provider, model, reasoning) for the --verbose four-role roster
// (PRD §9.13 FR51b). The caller maps config.RoleConfig → RoleLine at the decompose call site.
type RoleLine struct {
	Name      string // "planner" | "stager" | "message" | "arbiter"
	Model     string
	Provider  string
	Reasoning string // off|low|medium|high; "" ⇒ off (no suffix)
}

// reasoningSuffix returns " (reasoning: <level>)" for low/medium/high; empty for ""/"off"/unknown.
func reasoningSuffix(level string) string {
	switch level {
	case "low", "medium", "high":
		return " (reasoning: " + level + ")"
	}
	return ""
}

// VerboseRoles prints the four-role roster (one "DEBUG:" line each) when verbose is on. No-op when
// v==nil, v.w==nil, or !v.on (same guard idiom as VerboseCommand/VerboseRawOutput/VerboseRetry).
func (v *Verbose) VerboseRoles(roles []RoleLine) {
	if v == nil || v.w == nil || !v.on {
		return
	}
	for _, r := range roles {
		fmt.Fprintf(v.w, "DEBUG: %-8s %s%s\n", r.Name, invocation(r.Model, r.Provider), reasoningSuffix(r.Reasoning))
	}
}
