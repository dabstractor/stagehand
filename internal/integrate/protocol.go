// Package integrate provides the no-mangle write protocol engine (PRD §9.21 FR-I3 (a)–(g)).
// Every file-editing integration target (lazygit, git-alias) is driven through the Target interface
// by the Apply function, which enforces: parse-first refusal of unparseable files (a); marker-
// identified idempotent upsert + remove-no-op (b); user-confirmed unified-diff preview via
// git diff --no-index (c); timestamped backup before modification (d); atomic write (temp+rename)
// with post-write re-parse validation and auto-restore on failure (e); surgical scope where the
// Target owns the node edit and the protocol owns the no-mangle envelope (f); and create-if-missing
// through the same preview+confirm flow (g). The protocol, not any serializer, is the no-mangle
// guarantee: any incidental whole-document normalization is caught by re-parse validation or shown
// in the diff for the user to confirm.
package integrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/stagecoach/internal/ui"
)

// Action selects whether Apply installs or removes the marker-identified entry (PRD §9.21 FR-I3b).
type Action int

const (
	ActionUpsert Action = iota // insert-or-replace the marker entry (idempotent — replace, never duplicate)
	ActionRemove               // delete ONLY the marker-identified entry
)

// Outcome describes what Apply did, for the caller's (S2's) status report.
type Outcome int

const (
	OutcomeCreated  Outcome = iota // file did not exist; created with the entry (FR-I3g)
	OutcomeUpdated                 // file existed; marker entry inserted or replaced (FR-I3b)
	OutcomeRemoved                 // file existed; marker entry deleted
	OutcomeDeclined                // user answered N (or non-TTY auto-decline); NOTHING written (FR-I3c)
	OutcomeNoChange                // action is a no-op: remove a missing entry, or upsert identical bytes
)

// String renders Outcome for logs/verbose output. S2 maps these to user-facing verbs.
func (o Outcome) String() string {
	switch o {
	case OutcomeCreated:
		return "Created"
	case OutcomeUpdated:
		return "Updated"
	case OutcomeRemoved:
		return "Removed"
	case OutcomeDeclined:
		return "Declined"
	case OutcomeNoChange:
		return "NoChange"
	default:
		return fmt.Sprintf("Outcome(%d)", int(o))
	}
}

// Target is the format-specific adapter the protocol drives (PRD §9.21). Each file-editing
// integration target (lazygit, git-alias) implements this over its native parser. The protocol
// NEVER touches bytes except through these methods + the backup/atomic-write machinery: the Target
// owns the SURGICAL EDIT; the protocol owns the NO-MANGLE ENVELOPE (parse-first, diff, confirm,
// backup, validate).
//
// Stateful contract: Parse populates the target's in-memory state; HasEntry/Upsert/Remove read/mutate
// it. Validate must NOT depend on prior Parse state (use a local probe) so the post-write gate is
// clean and side-effect-free.
type Target interface {
	// Marker returns the identity string for stagehand's contribution — a comment or well-known key
	// whose presence means "stagehand owns this entry" (FR-I3b idempotency, FR-I3f surgical scope).
	Marker() string

	// Parse loads existing file content into the target's state. A non-nil error means the file is
	// unparseable; the protocol HARD-REFUSES to write (FR-I3a) and surfaces this error verbatim.
	// Parse is called only on content successfully read from disk (a missing file is the
	// create-if-missing path, not a parse error).
	Parse(data []byte) error

	// HasEntry reports whether the marker-identified entry is present in the parsed state. Drives
	// idempotency (Upsert replaces, never duplicates — FR-I3b) and the Remove no-op (removing a
	// missing entry yields OutcomeNoChange, nothing written).
	HasEntry() bool

	// Upsert returns new file bytes with the marker entry inserted (if absent) or replaced (if
	// present). It MUST be surgical (FR-I3f): only the marker entry changes semantically.
	// (For YAML, incidental whole-doc normalization is unavoidable — architecture §2; the protocol's
	// diff+confirm surfaces it.)
	Upsert() ([]byte, error)

	// Remove returns new file bytes with the marker entry deleted. Removing a non-present entry
	// returns the original bytes unchanged (the protocol treats new==old as OutcomeNoChange).
	Remove() ([]byte, error)

	// Validate re-parses data to confirm well-formedness WITHOUT relying on or mutating prior Parse
	// state (use a local probe). The protocol calls it on the freshly-written bytes (FR-I3e); failure
	// triggers a backup restore. Typically Validate(data) equals Parse(data) on a throwaway instance.
	Validate(data []byte) error
}

// ConfirmFunc asks the user whether to apply the change at path (with diff already rendered to out).
// Returns true to proceed, false to skip without writing (OutcomeDeclined). Apply uses DefaultConfirm
// when opts.Confirm is nil.
type ConfirmFunc func(out io.Writer, path, diff string) bool

// ApplyOptions configures a single file's no-mangle write (PRD §9.21 FR-I3).
type ApplyOptions struct {
	Path    string      // absolute path to the target config file
	Target  Target      // the format-specific adapter (Parse'd by Apply)
	Action  Action      // ActionUpsert or ActionRemove
	Yes     bool        // --yes: skip the confirm prompt (scripts) — FR-I3c
	Out     io.Writer   // preview diff + status; nil ⇒ os.Stderr
	Confirm ConfirmFunc // y/N prompt; nil ⇒ DefaultConfirm (os.Stdin, TTY-gated) — FR-I3c
}

// ApplyResult describes what Apply did, for the caller's (S2's) status report.
type ApplyResult struct {
	Outcome Outcome // Created/Updated/Removed/Declined/NoChange
	Path    string  // the target path (echoed back)
	Backup  string  // the backup file path written (empty if none was written)
}

// BackupPath returns the timestamped backup path for a file (FR-I3d):
// <file>.stagehand-backup.<unix-ts>. Exported so callers/tests can predict/locate the backup.
func BackupPath(path string, unixTs int64) string {
	return fmt.Sprintf("%s.stagehand-backup.%d", path, unixTs)
}

// Apply runs the FR-I3 (a)–(g) no-mangle write protocol over a single file. Returns an
// ApplyResult (always non-nil on success) and a plain error on any failure. NEVER calls os.Exit;
// the cmd layer (S2) routes exit codes. Writes NOTHING unless the change is confirmed and validated.
func Apply(ctx context.Context, opts ApplyOptions) (ApplyResult, error) {
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}
	res := ApplyResult{Path: opts.Path, Outcome: OutcomeNoChange}

	// ---- (a)/(g) read + parse-first ----
	orig, rerr := os.ReadFile(opts.Path)
	missing := errors.Is(rerr, os.ErrNotExist)
	if rerr != nil && !missing {
		return res, fmt.Errorf("read %s: %w", opts.Path, rerr)
	}
	var exists bool
	if !missing {
		if perr := opts.Target.Parse(orig); perr != nil { // (a) HARD refuse — never write an unparseable file
			return res, fmt.Errorf("refused to write %s: parse error (nothing was changed): %w", opts.Path, perr)
		}
		exists = opts.Target.HasEntry()
	}

	// ---- (b) idempotent upsert / Remove-no-op ----
	var newBytes []byte
	switch opts.Action {
	case ActionRemove:
		if missing || !exists {
			return res, nil // OutcomeNoChange — nothing to remove; NOTHING written, no backup
		}
		b, rerr := opts.Target.Remove()
		if rerr != nil {
			return res, fmt.Errorf("remove entry: %w", rerr)
		}
		newBytes = b
	case ActionUpsert:
		b, uerr := opts.Target.Upsert()
		if uerr != nil {
			return res, fmt.Errorf("upsert entry: %w", uerr)
		}
		newBytes = b
	default:
		return res, fmt.Errorf("unknown action %d", opts.Action)
	}

	// no-op: upsert produced identical bytes (already installed) ⇒ idempotent no-write (FR-I3b).
	if !missing && bytes.Equal(orig, newBytes) {
		return res, nil // OutcomeNoChange
	}

	// ---- (c) unified-diff preview + confirm ----
	diff, derr := previewDiff(ctx, opts.Path, orig, newBytes, missing)
	if derr != nil {
		return res, fmt.Errorf("build preview diff: %w", derr)
	}
	if !opts.Yes {
		confirm := opts.Confirm
		if confirm == nil {
			confirm = DefaultConfirm
		}
		if !confirm(out, opts.Path, diff) {
			res.Outcome = OutcomeDeclined
			return res, nil // NOTHING written, no backup
		}
	}

	// ---- (d) backup (only for a real modification of an existing file) ----
	if !missing {
		res.Backup = BackupPath(opts.Path, time.Now().Unix())
		if berr := os.WriteFile(res.Backup, orig, 0o644); berr != nil {
			return res, fmt.Errorf("write backup %s: %w", res.Backup, berr)
		}
	}

	// ---- (g) create-if-missing parent dirs (AFTER confirm, so a Decline litters nothing) ----
	if err := os.MkdirAll(filepath.Dir(opts.Path), 0o755); err != nil {
		return res, fmt.Errorf("create parent dirs: %w", err)
	}

	// ---- (e) atomic write (temp + rename) ----
	if err := atomicWrite(opts.Path, newBytes); err != nil {
		return res, fmt.Errorf("atomic write %s: %w", opts.Path, err)
	}

	// ---- (e) re-parse validate; on failure restore the backup ----
	if verr := opts.Target.Validate(newBytes); verr != nil {
		if rerr := restore(opts.Path, orig, missing); rerr != nil { // restore orig over the target
			return res, fmt.Errorf("validate failed (%v) AND restore failed: %w (backup at %s)", verr, rerr, res.Backup)
		}
		return res, fmt.Errorf("refused to keep %s: post-write validation failed — restored original (backup at %s): %w",
			opts.Path, res.Backup, verr)
	}

	// ---- success ----
	if missing {
		res.Outcome = OutcomeCreated
	} else if opts.Action == ActionRemove {
		res.Outcome = OutcomeRemoved
	} else {
		res.Outcome = OutcomeUpdated
	}
	return res, nil
}

// DefaultConfirm is the FR-I3c y/N prompt used when ApplyOptions.Confirm is nil. It writes the
// unified diff to out, then "Apply changes to <path>? [y/N] ", reads one line from os.Stdin, and
// accepts ONLY a line whose first non-space byte is 'y' or 'Y'. When stdin is NOT a terminal (a
// piped/scripted invocation without --yes) it AUTO-DECLINES without blocking — the safe default
// that never hangs a non-interactive run (--yes is the explicit script bypass).
func DefaultConfirm(out io.Writer, path, diff string) bool {
	if diff != "" {
		fmt.Fprint(out, diff)
		if !strings.HasSuffix(diff, "\n") {
			fmt.Fprintln(out)
		}
	}
	if !ui.IsTerminal(os.Stdin) { // non-interactive ⇒ do not block; use --yes to force
		fmt.Fprintf(out, "stagehand: non-interactive stdin — declining to modify %s (use --yes to apply)\n", path)
		return false
	}
	fmt.Fprintf(out, "Apply changes to %s? [y/N] ", path)
	var line string
	_, _ = fmt.Fscanln(os.Stdin, &line) // best-effort; EOF/empty ⇒ decline
	line = strings.TrimSpace(line)
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

// previewDiff returns the unified diff between the original and new content via
// git diff --no-index --no-color (PRD §19: []string args, NO shell). Both sides are materialized
// as temp files under <tmpdir>/{a,b}/<base> and git is run with -C <tmpdir> so the ---/+++
// labels read a/<basename> / b/<basename>. When oldMissing is true the a/<base> side is written
// empty (never an absent path — git would error "Could not access"); the diff then shows all
// lines added. Exit codes: 0 = identical (returns ""), 1 = differences (returns stdout), >1 = error.
func previewDiff(ctx context.Context, path string, oldBytes, newBytes []byte, oldMissing bool) (string, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git binary not found in PATH: %w", err)
	}
	base := filepath.Base(path)
	tmp, err := os.MkdirTemp("", "stagehand-diff-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	for _, sub := range []string{"a", "b"} {
		if err := os.MkdirAll(filepath.Join(tmp, sub), 0o755); err != nil {
			return "", err
		}
	}
	aPath := filepath.Join(tmp, "a", base)
	bPath := filepath.Join(tmp, "b", base)
	oldSide := oldBytes
	if oldMissing {
		oldSide = nil // empty file on the a/ side
	}
	if err := os.WriteFile(aPath, oldSide, 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(bPath, newBytes, 0o644); err != nil {
		return "", err
	}
	// run: git -C <tmp> diff --no-index --no-color -- a/<base> b/<base>
	stdout, _, code, runErr := runGit(ctx, gitPath, tmp, "diff", "--no-index", "--no-color", "--",
		filepath.ToSlash(filepath.Join("a", base)), filepath.ToSlash(filepath.Join("b", base)))
	if runErr != nil {
		return "", runErr // context cancel / start failure
	}
	if code == 0 || code == 1 {
		return stdout, nil // "" when identical; the diff text when different
	}
	return "", fmt.Errorf("git diff --no-index: exit %d", code)
}

// runGit runs a git command with -C dir and the given args. It mirrors internal/git/git.go run()
// but is repo-independent (git diff --no-index needs no repo). []string args, NO shell (PRD §19).
// Separate stdout/stderr buffers. Returns (stdout, stderr, exitCode, error) where error is nil
// for non-zero exits (git uses exit codes as semantic signals); only context cancel, LookPath
// failure, or start/I/O errors return non-nil error with code=-1.
func runGit(ctx context.Context, gitPath, dir string, args ...string) (stdout, stderr string, code int, err error) {
	full := make([]string, 0, len(args)+2)
	full = append(full, "-C", dir)
	full = append(full, args...)
	cmd := exec.CommandContext(ctx, gitPath, full...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	runErr := cmd.Run()
	stdout, stderr = outb.String(), errb.String()
	if runErr == nil {
		return stdout, stderr, 0, nil
	}
	if cerr := ctx.Err(); cerr != nil {
		return stdout, stderr, -1, cerr
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return stdout, stderr, exitErr.ExitCode(), nil
	}
	return stdout, stderr, -1, runErr
}

// atomicWrite writes data to a temp file in the SAME directory as path (same filesystem ⇒ atomic
// rename), then renames it over path with mode 0o644 (FR-I3e). The temp file lives in
// filepath.Dir(path), NOT os.TempDir(), to avoid a cross-filesystem rename (non-atomic).
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".stagehand-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// restore writes orig back over path after a validate failure (FR-I3e). If the file was missing
// before the Apply (create path), restore removes it; otherwise rewrites orig atomically. The
// backup file is RETAINED (FR-I3d — it is the user's safety record).
func restore(path string, orig []byte, wasMissing bool) error {
	if wasMissing {
		return os.Remove(path)
	}
	return atomicWrite(path, orig)
}
