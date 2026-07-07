// Package integrate provides the target registry and Entry interface for the
// `stagecoach integrate` command surface (PRD §9.21 FR-I1/I2). The Registry
// holds pluggable integration targets (git-alias, lazygit, future) and the
// Entry interface is the uniform contract every target implements.
//
// This file is pure library — no cobra, no os.Exit, no exitcode import.
// The cmd layer (internal/cmd/integrate.go) wraps errors via exitcode.New.
package integrate

import (
	"context"
	"errors"
	"io"
	"sort"
)

// Status is the integration state of one target (PRD §9.21 FR-I1).
// DISTINCT from hook.Status — integrate owns its own enum and exact report tokens.
type Status int

const (
	StatusNotInstalled Status = iota // no stagecoach-managed entry in the target's config
	StatusInstalled                  // stagecoach entry present (marker/key ours)
	StatusForeign                    // a conflicting entry exists at the target's key/alias
)

// String renders the FR-I1 tokens EXACTLY: "not installed" / "installed" / "foreign".
func (s Status) String() string {
	switch s {
	case StatusInstalled:
		return "installed"
	case StatusForeign:
		return "foreign"
	default:
		return "not installed"
	}
}

// Entry is one integration target (git-alias, lazygit, future). The cmd layer
// (list/install/remove) drives every target uniformly through this interface;
// each target owns its install/remove MECHANICS — lazygit calls protocol.Apply
// (FR-I5), git-alias delegates to `git config` (FR-I4, which does NOT use Apply).
// The four registry-facing methods (Name/Detect/ConfigPath/Status) back `list`
// + detection gating; Install/Remove back the commands.
// T2.S1/T2.S2 implement this; S2 tests with fakes.
type Entry interface {
	// Name is the target's CLI token (e.g. "git-alias", "lazygit") — the <target>
	// argument. Stable, unique.
	Name() string

	// Detect reports whether the target's TOOL is on $PATH (FR-I2 detection gating).
	// nil ⇒ present (install may proceed); non-nil ⇒ absent — the command prints
	// the error's message as the note and skips install for this target (exit 1).
	// git-alias detects git (always present for stagecoach); lazygit detects lazygit.
	Detect(ctx context.Context) error

	// ConfigPath resolves the config file/path the target edits (FR-I1 "resolved
	// config path" column + the note/error context). May be empty/"—" if the target
	// cannot resolve it (e.g. tool absent) — never fatal for `list`; it just shows "—".
	ConfigPath(ctx context.Context) (string, error)

	// Status reports the integration's current state (FR-I1). Reads the target's config
	// for the stagecoach entry: NotInstalled (absent) / Installed (ours) / Foreign
	// (a conflicting entry). Independent of Detect (a target can be StatusInstalled with
	// the tool since uninstalled).
	Status(ctx context.Context) (Status, error)

	// Install applies the integration. The target decides HOW (lazygit: protocol.Apply;
	// git-alias: git config). opts carries the shared controls every target honors:
	// Yes (skip confirm, --yes), Out (preview/status writer), Confirm (nil ⇒
	// DefaultConfirm). Returns an InstallResult (Outcome is S1's unified enum) and a
	// plain error on failure (the cmd layer maps to exit 1). Decline/NoChange are
	// reported via Outcome, NOT as errors.
	Install(ctx context.Context, opts InstallOptions) (InstallResult, error)

	// Remove deletes the stagecoach entry (uninstall symmetry). Same controls/contract
	// as Install.
	Remove(ctx context.Context, opts RemoveOptions) (RemoveResult, error)
}

// InstallOptions are the shared controls passed to Entry.Install (PRD §9.21 FR-I3c).
// Target-specific values (--alias-name, --key) are NOT here — T2's defaultEntries()
// constructs each Entry with its resolved flag values, so the interface stays narrow.
// Confirm==nil ⇒ DefaultConfirm (S1).
type InstallOptions struct {
	Yes     bool        // --yes: skip the y/N confirm (scripts)
	Out     io.Writer   // preview diff + status (cmd's stderr in prod); nil ⇒ os.Stderr (target decides)
	Confirm ConfirmFunc // y/N prompt; nil ⇒ DefaultConfirm (TTY-gated, S1). Reused from protocol.go.
}

// RemoveOptions mirror InstallOptions for Entry.Remove.
type RemoveOptions struct {
	Yes     bool
	Out     io.Writer
	Confirm ConfirmFunc
}

// InstallResult is what Entry.Install did, for the cmd layer's per-target status line.
// Outcome is S1's integrate.Outcome — the unified vocabulary across target kinds.
type InstallResult struct {
	Outcome Outcome // S1's enum — lazygit copies ApplyResult.Outcome; git-alias maps its own result
	Target  string  // the target name (echoed for the status line)
	Path    string  // the config path touched
	Backup  string  // the backup path written (empty if none)
}

// RemoveResult mirrors InstallResult for Entry.Remove.
type RemoveResult struct {
	Outcome Outcome
	Target  string
	Path    string
	Backup  string
}

// Sentinels for the dispatch refusal paths. Callers use errors.Is.
var (
	// ErrUnknownTarget is returned/wrapped when a named target is not in the registry.
	ErrUnknownTarget = errors.New("unknown integration target")
	// ErrToolNotDetected wraps a target-specific Detect failure for detection-gating
	// context (FR-I2). The target's Detect error is the primary signal; this is the
	// category the cmd layer recognizes.
	ErrToolNotDetected = errors.New("target tool not detected on $PATH")
)

// Registry holds the compiled-in integration targets (PRD §9.21). Mirrors
// provider.Registry: a name-keyed map, List() sorted ascending by Name
// (deterministic for `list`), Get for unknown-target refusal. Seeds EXACTLY from
// the passed slice (no built-in targets in S2 — T2 supplies git-alias/lazygit).
// Pure data structure; no exec inside (Entry methods do the probing).
// Construct fresh per command (no global state).
type Registry struct {
	entries map[string]Entry
}

// NewRegistry builds a Registry from entries. Duplicate Names ⇒ the last wins
// (defensive; T2's list has none). nil/empty ⇒ an empty registry.
func NewRegistry(entries []Entry) *Registry {
	m := make(map[string]Entry, len(entries))
	for _, e := range entries {
		if e != nil {
			m[e.Name()] = e
		}
	}
	return &Registry{entries: m}
}

// Get returns the Entry for name and whether it exists.
func (r *Registry) Get(name string) (Entry, bool) {
	e, ok := r.entries[name]
	return e, ok
}

// List returns every Entry, sorted ascending by Name (deterministic for
// `integrate list`). Fresh slice.
func (r *Registry) List() []Entry {
	out := make([]Entry, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}
