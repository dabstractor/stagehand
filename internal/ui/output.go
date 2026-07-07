package ui

import (
	"fmt"
	"io"
	"os"
)

// ANSI SGR codes (Select Graphic Rendition). Emitted ONLY when u.color is true.
const (
	ansiReset  = "\x1b[0m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
)

// progressPrefix is the Appendix-B "↳" glyph (U+21B3) + one space, prefixing every progress line.
const progressPrefix = "↳ "

// IsTerminal reports whether f is a real terminal/pty (true isatty probe). Returns false for
// /dev/null, pipes, files, and redirects — delegating to the platform-specific isTerminalFd:
//
//   - linux:   ioctl(TCGETS) succeeds iff terminal
//   - darwin:  ioctl(TIOCGETA) succeeds iff terminal
//   - windows: GetConsoleMode returns nonzero iff console handle
//   - other:   legacy char-device heuristic (safe fallback; see isatty_other.go)
//
// stat/ioctl errors → false (treat as non-TTY → the safe default). --no-color / NO_COLOR remain
// the authoritative overrides (see ResolveColor). Signature is stable; all callers (config init
// --interactive, integrate DefaultConfirm, hook exec / default-action color resolution) benefit
// automatically.
func IsTerminal(f *os.File) bool {
	return isTerminalFd(f.Fd())
}

// TerminalWidth reports the column width of the terminal attached to f, or 0 when it can't be
// determined (f is not a TTY — pipe/file/redirect — or the platform can't report it). Mirrors
// IsTerminal's platform delegation (linux/darwin: ioctl(TIOCGWINSZ); windows:
// GetConsoleScreenBufferInfo; other: 0). The CLI uses it to wrap --help output to the live screen
// width; a 0 return lets callers fall back to a fixed default. Always returns >= 0.
func TerminalWidth(f *os.File) int {
	return terminalWidthFd(f.Fd())
}

// noColorEnvSet reports whether the NO_COLOR convention (https://no-color.org) disables color: the var
// is present AND not an empty string. Byte-identical idiom to config/load.go's STAGECOACH_NO_COLOR
// handling (line 112) — kept consistent so a user's mental model transfers between the two vars.
func noColorEnvSet() bool {
	v, ok := os.LookupEnv("NO_COLOR")
	return ok && v != ""
}

// ResolveColor decides whether ANSI color should be emitted (PRD §9.13 FR51, §15.2). Color is ON iff:
// cfg.NoColor is false (--no-color / STAGECOACH_NO_COLOR, already resolved by config.Load) AND the bare
// NO_COLOR env var is unset/empty (https://no-color.org) AND stdout is a terminal. The TTY check is
// PASSED IN (isTTY) so the untestable IO is decoupled from the testable flag/env logic (work item:
// "can't easily test TTY in unit tests; test the flag/env logic"). Callers pass isTTY = IsTerminal(os.Stdout).
func ResolveColor(noColor bool, isTTY bool) bool {
	if noColor {
		return false
	}
	if noColorEnvSet() {
		return false
	}
	return isTTY
}

// UI renders Stagecoach's CLI output with optional ANSI color (PRD §9.13, Appendix B). Progress/Success/
// Error go to STDERR (FR51: stdout stays clean for piping); the actual RESULT data (commit report,
// dry-run message) stays PLAIN on stdout via the caller's own print path — never thread it through here.
// Writers are injectable (cobra's cmd.OutOrStdout/ErrOrStderr in prod, *bytes.Buffer in tests); color is
// a resolved bool passed to New (tests pass true/false directly — no real TTY needed).
type UI struct {
	stdout io.Writer
	stderr io.Writer
	color  bool
}

// New constructs a UI writing to the given streams with the given color resolution. From the CLI:
//
//	ui.New(stdout, stderr, ui.ResolveColor(cfg.NoColor, ui.IsTerminal(os.Stdout)))
//
// From tests: ui.New(&outBuf, &errBuf, true|false). nil writers default to os.Stdout/os.Stderr.
func New(stdout, stderr io.Writer, color bool) *UI {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return &UI{stdout: stdout, stderr: stderr, color: color}
}

// Color reports whether color is enabled (for conditional callers / tests).
func (u *UI) Color() bool { return u.color }

// Green wraps s in ANSI green iff color is enabled; otherwise returns s unchanged (zero ANSI bytes).
func (u *UI) Green(s string) string { return u.colorize(ansiGreen, s) }

// Red wraps s in ANSI red iff color is enabled; otherwise returns s unchanged (zero ANSI bytes).
func (u *UI) Red(s string) string { return u.colorize(ansiRed, s) }

// Yellow wraps s in ANSI yellow iff color is enabled; otherwise returns s unchanged (zero ANSI bytes).
func (u *UI) Yellow(s string) string { return u.colorize(ansiYellow, s) }

func (u *UI) colorize(code, s string) string {
	if !u.color {
		return s
	}
	return code + s + ansiReset
}

// Progress writes a progress line to STDERR with the Appendix-B "↳ " prefix (FR51: stdout stays clean
// for piping). Callers build the body via ProgressLabel (FR51b):
//
//	u.Progress(ui.ProgressLabel("Generating", "sonnet", "claude"))
//	// → stderr: "↳ Generating with sonnet in claude…\n"
func (u *UI) Progress(msg string) {
	fmt.Fprintln(u.stderr, progressPrefix+msg)
}

// Success writes a success notice to STDERR in green (when color): the Appendix-B "↳ " prefix + msg.
// Example: Success("Created abc1234") -> green "↳ Created abc1234". (The data report itself stays plain
// on stdout via the caller's print path.)
func (u *UI) Success(msg string) {
	fmt.Fprintln(u.stderr, u.Green(progressPrefix+msg))
}

// invocation renders the FR51b "<model> in <provider>" core (shared by ProgressLabel and VerboseRoles).
// provider=="" ⇒ "" (nothing resolved); model=="" ⇒ "<provider>" alone (the provider's own default);
// else "<model> in <provider>". The model string already carries the inference backend (FR-R5b), so
// it is printed VERBATIM — no special formatting or splitting.
func invocation(model, provider string) string {
	if provider == "" {
		return ""
	}
	if model != "" {
		return model + " in " + provider
	}
	return provider
}

// ProgressLabel builds the FR51b progress-line body (without the "↳ " prefix — Progress adds that):
// "<verb>…" when nothing is resolved (provider==""), else "<verb> with <invocation>…". Pure (no I/O),
// trivially unit-testable. The main label omits reasoning (FR51b); reasoning appears only in the
// --verbose four-role enumeration via VerboseRoles.
func ProgressLabel(verb, model, provider string) string {
	if inv := invocation(model, provider); inv != "" {
		return verb + " with " + inv + "…"
	}
	return verb + "…"
}

// Error writes an error notice to STDERR in red (when color). Example: Error("generation failed").
// (The frozen §18.3 rescue block is NOT routed through here — it stays plain; see handleGenError.)
func (u *UI) Error(msg string) {
	fmt.Fprintln(u.stderr, u.Red(msg))
}
