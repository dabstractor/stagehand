name: "P1.M4.T1.S1 — No-mangle write protocol engine (internal/integrate/protocol.go)"
description: |
  Implement the FORMAT-AGNOSTIC engine that backs every file-editing integration target (PRD §9.21
  FR-I3 a–g): a `Target` interface (Parse/HasEntry/Upsert/Remove/Validate/Marker) and a `protocol.Apply`
  orchestrator that runs, in order — (a) parse-first hard refusal of unparseable files; (b) marker-
  identified idempotent upsert (replace, never duplicate) + Remove-of-missing no-op; (c) unified-diff
  preview via `git diff --no-index` + per-file `y/N` confirm (`--yes` bypass, non-TTY auto-decline);
  (d) timestamped `<file>.stagecoach-backup.<unix-ts>` before modifying; (e) atomic write (temp+rename) +
  re-Parse validate, auto-restore backup on failure; (f) surgical scope (the Target owns the node edit;
  the protocol owns the no-mangle envelope); (g) create-if-missing (file + parent dirs) through the same
  preview+confirm flow. The engine consumes `internal/ui` (preview writer + `IsTerminal` for the TTY-gated
  confirm). Tested with an in-package text-block `Target` (no yaml/toml dep): golden round-trips,
  corrupt-input refusal, backup/restore on injected validation failure, idempotent double-install,
  create-if-missing, decline. **FOUNDATIONAL** — consumed by P1.M4.T1.S2 (command surface) and
  P1.M4.T2.S1/S2 (git-alias, lazygit). Ships NO cobra command, NO concrete target, NO new third-party dep.

---

## Goal

**Feature Goal**: Ship the reusable, format-agnostic engine that makes it **structurally impossible for
stagecoach to mangle a user-owned config file** (PRD §9.20 FR-I3, §9.21). Every file-editing integration
target (lazygit, git-alias, future) is driven through one `Target` interface by one `protocol.Apply`
function that enforces parse-first refusal, marker idempotency, a user-confirmed unified diff, a
timestamped backup, an atomic write, a post-write re-parse with auto-restore, and create-if-missing —
the full FR-I3 (a)–(g) sequence, non-negotiable and in order. Because yaml.v3 (the lazygit parser) cannot
guarantee byte-identity outside the edited node (architecture/external_deps.md §2, VERIFIED), the protocol
itself — not any serializer — is the no-mangle guarantee: any incidental whole-document normalization is
either caught by the re-parse (→ backup restored) or shown in the diff (→ user confirms).

**Deliverable**:
1. `internal/integrate/protocol.go` — `Target` interface; `Action`/`Outcome` enums; `ApplyOptions`;
   `ApplyResult`; `ConfirmFunc`; `Apply(opts ApplyOptions) (ApplyResult, error)`; `DefaultConfirm`;
   `BackupPath(path string) string`; plus the unexported `previewDiff` (git `--no-index`) and
   `atomicWrite`/`restore` helpers. Pure library code (plain errors; the cmd layer routes exit codes).
2. `internal/integrate/protocol_test.go` — an in-package `blockTarget` (marker-delimited text) test
   vehicle + the full test matrix (golden round-trips, corrupt-input refusal, backup/restore, idempotent
   double-install, create-if-missing, Remove-no-op, Decline, preview non-empty iff changed).
3. (Optional, only if it keeps the diff small) `internal/integrate/doc.go` — the package doc comment
   restating FR-I3 in one paragraph; otherwise the package doc lives atop `protocol.go`.

**Success Definition**: A consumer (S2/T2) implements `Target` and calls `Apply`; the engine guarantees
(a) an unparseable file is never written and its parse error is surfaced; (b) installing twice yields the
same file (no duplicate entry) and a no-op result the second time; (c) every modification is preceded by a
readable unified diff and a `y/N` prompt that `--yes` bypasses and a non-TTY stdin auto-declines; (d) a
`<file>.stagecoach-backup.<unix-ts>` is written before every real modification; (e) the write is atomic
(temp+rename) and a post-write parse failure restores the original content; (g) a missing file (and its
parent dirs) is created through the same flow. `go build ./...`, `go test ./...`, `go vet ./...`,
`golangci-lint run`, `gofmt` all green; no new third-party dependency in `go.mod`.

## User Persona

**Target User**: the "plan-holder" (PRD §7.1) running `stagecoach integrate install lazygit` — stagecoach is
about to edit a dotfile the user has hand-tuned for years. Their overriding fear is "will this trash my
config?" The no-mangle protocol is the guarantee it will not.

**Use Case**: `integrate install <target>` resolves a config path, builds a `Target`, and calls
`protocol.Apply`. The user sees a unified diff of exactly what will change, confirms `y`, and gets a
timestamped backup as a safety net. On any anomaly the file is left untouched.

**User Journey**: run install → see diff → type `y` → file updated, backup written, success line printed.
On a parse failure: "refused to write — your config has a syntax error (shown), nothing was changed."
On `N`: "declined — nothing was changed."

**Pain Points Addressed**: incumbents (and naive ports) round-trip config files through a serializer that
silently drops comments, reorders keys, and normalizes spacing — the user discovers the damage later. The
protocol makes the edit surgical-and-visible by construction: parse-first refusal + diff + confirm +
backup + validate-restore.

## Why

- **PRD §9.21 lead-in**: "it edits user-owned dotfiles, so its write protocol is the point: **it must be
  impossible for stagecoach to mangle a config file.**"
- **PRD §9.21 FR-I3 (a)–(g)**: the non-negotiable, in-order protocol every file-editing target follows.
- **architecture/external_deps.md §2 (VERIFIED)**: yaml.v3 re-encodes the whole document (drops blank
  lines, normalizes comment spacing) — so "preserved outside the edited node" (FR-I3f) is delivered by the
  protocol envelope (parse→diff→confirm→backup→validate), NOT by the library. The protocol IS the guarantee.
- **RESEARCH NOTE (work item)**: the unified-diff preview shells out to `git diff --no-index -- <old> <new>`
  (zero new dependencies; matches the repo's shell-out-to-git ethos, PRD §19) rather than vendoring a diff lib.
- **Scope fences**: foundational — no input dependencies; provides `Apply`/`Target`/`DefaultConfirm` consumed
  by S2 (command surface + detection gating, FR-I1/I2) and T2.S1/S2 (git-alias, lazygit). Does NOT add the
  cobra commands, the target registry, yaml.v3, or any concrete Target.

## What

A format-agnostic engine: one `Target` interface, one `Apply` function, one `DefaultConfirm`, and the
git-`--no-index` preview + atomic-write + backup helpers. The engine is exercised by an in-package
text-block test Target (no real format dependency) so it is provably correct before any concrete target exists.

### Success Criteria

- [ ] `internal/integrate/protocol.go`: `Target` interface (Marker/Parse/HasEntry/Upsert/Remove/Validate);
      `Action` (Upsert/Remove); `Outcome` (Created/Updated/Removed/Declined/NoChange);
      `ApplyOptions{Path, Target, Action, Yes, Out, Confirm}`; `ApplyResult{Outcome, Path, Backup}`;
      `ConfirmFunc`; `Apply`; `DefaultConfirm`; `BackupPath`.
- [ ] `Apply` runs FR-I3 (a)–(g) in order: parse-first hard refusal; idempotent upsert / Remove-no-op;
      no-op when new==old; git-`--no-index` diff; `y/N` confirm (`--yes` bypass; non-TTY auto-decline when
      `Confirm==nil`); timestamped backup; `MkdirAll` parents; atomic temp+rename write; post-write
      `Validate` with auto-restore-on-failure.
- [ ] `Apply` writes NOTHING on: parse error, Decline, Remove-of-missing, upsert-identical, or validate
      failure (the last restores orig and retains the backup).
- [ ] `previewDiff` runs `git diff --no-index --no-color` via `exec.CommandContext` (`[]string` args, NO
      shell — PRD §19); maps exit `0|1`→stdout (empty if identical), `>1`/LookPath-miss→error.
- [ ] `internal/integrate/protocol_test.go`: an in-package `blockTarget` (marker-delimited text) + golden
      round-trips, corrupt-input refusal, backup/restore (injected Validate failure), idempotent
      double-install, create-if-missing (file + parent dirs), Remove-no-op, Decline, diff-non-empty-iff-changed.
- [ ] `go build ./...` + `go test ./...` + `go vet ./...` + `golangci-lint run` + `gofmt -l` clean; `go.mod`
      gains NO new `require` (yaml.v3 is T2.S2's addition, not this subtask's).

## All Needed Context

### Context Completeness Check

_This PRP names the exact `Target` method set + semantics, the exact `Apply` step sequence (a–g, in order,
with the no-op/decline/restore branch points), the verified `git diff --no-index` behavior (exit codes,
`a/`/`b/` labeling, create-if-missing as an empty `a/` side), the atomic-write + backup file naming, the
`ConfirmFunc` injection + `DefaultConfirm` TTY rule, the test `blockTarget` design with its corrupt-input
and bad-Validate variants, and the scope fence (no cobra, no concrete target, no yaml). An implementer with
no prior codebase knowledge can build it from this document + codebase access._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/P1M4T1S1/research/codebase-patterns.md
  why: Condensed research — the yaml.v3 §2 consequence (protocol IS the no-mangle guarantee), the verified
       git-diff --no-index behavior + labeling + exit codes, the Target interface design, the Apply
       algorithm a–g, the blockTarget test vehicle, the existing patterns (hook/config/ui/git.run), and the
       scope fence (S2/T2 own commands + concrete targets).
  section: all
  critical: |
    yaml.v3 is NOT a current dependency and this subtask does NOT add it — the engine is format-agnostic and
    is tested with an in-package text-block Target. The diff preview is `git diff --no-index` (shell-out, []string
    args, PRD §19), NOT a vendored diff library. The engine is PURE LIBRARY code: plain errors, no cobra, no os.Exit.

- docfile: plan/005_c38aa48290f0/architecture/external_deps.md
  why: §2 (VERIFIED) — yaml.v3 whole-document re-encode; the protocol (not the serializer) must guarantee
       no-mangle. §1 — lazygit customCommands schema (context for T2.S2, NOT this subtask).
  section: "## 2. Comment-preserving YAML in Go"

- docfile: plan/005_c38aa48290f0/prd_snapshot.md
  why: §9.21 FR-I3 (a)–(g) verbatim (the contract), §9.21 lead-in ("must be impossible to mangle"), §19
       (no shell — []string args), §15.4 exit codes (S2 routes them; the engine returns plain errors).
  section: "§9.21 (FR-I3), §19, §15.4"

- file: internal/hook/hook.go
  why: THE "manage a file the no-mangle way" analog — Detect (marker-identity: ours vs foreign vs none),
       Install (idempotent rewrite / foreign refusal / creates parent dirs via MkdirAll), Uninstall
       (remove-only-ours). FR-I3b/f/g mirror these invariants. Note hook writes via plain os.WriteFile
       (single owned file, no parse concern) — the protocol is STRICTER (parse+backup+atomic+validate).
  pattern: "marker-identity string + status enum + sentinel errors (ErrForeignHook/ErrNoHook) + byte-for-byte
            unchanged assertion on refusal. Status enum + String() table-test style."
  gotcha: "hook.Install uses os.WriteFile (NOT atomic). Do NOT copy that for the protocol — FR-I3e mandates
           temp+rename. Copy the marker/foreign-refusal/MkdirAll IDEAS, not the write mechanism."

- file: internal/hook/script.go
  why: `Marker` const + `ScriptMode` — the precedent for a package-level identity string + a file-mode const.
       integrate.Target.Marker() is the generalization (a method, since each target has its own marker).
  pattern: "const Marker = \"# stagecoach-…\" identity line; const Mode os.FileMode."

- file: internal/hook/hook_test.go
  why: THE test style to mirror — t.TempDir() + real files; byte-for-byte 'unchanged' assertions on the
       refusal paths; table-driven enum.String(); idempotent-reinstall test (Install twice ⇒ same file).
       protocol_test.go follows this exactly (Apply twice ⇒ same file; corrupt input ⇒ unchanged).
  pattern: "dir := t.TempDir(); os.WriteFile(path, content, 0o644); ...; after, _ := os.ReadFile(path);
            if string(before) != string(after) { t.Error(\"file modified\") }"

- file: internal/cmd/config.go   (runConfigUpgrade, ~L155)
  why: THE parse→gate→write analog — reads file, `toml.Unmarshal(&probe)` (non-strict), on parse error
       `exitcode.New(exitcode.Error, fmt.Errorf(\"…is not valid TOML: %w\", err))` = REFUSE TO MANGLE
       (exactly FR-I3a "unparseable file is never written to"). It does NOT do backup/atomic/diff/confirm;
       those are what THIS subtask generalizes into the engine.
  pattern: "parse-error ⇒ return wrapped error WITHOUT writing. The protocol's step (a)."

- file: internal/ui/output.go
  why: `IsTerminal(f *os.File) bool` (L36) — the stdlib TTY heuristic DefaultConfirm reuses (non-TTY stdin
       + no --yes ⇒ auto-decline, don't block a piped run). `New(stdout, stderr, color)` — injectable
       writers (the engine takes a plain io.Writer for the preview; S2 passes a *ui.UI-derived writer).
  pattern: "ui.IsTerminal(os.Stdin) gates interactive prompts. The engine imports internal/ui ONLY for this."

- file: internal/git/git.go   (run(), ~L283)
  why: THE exec []string-args pattern to mirror for previewDiff's git-diff helper — exec.LookPath(\"git\")
       → exec.CommandContext(ctx, gitPath, full...) ([]string, NO shell, PRD §19) → separate bytes.Buffer
       stdout/stderr → errors.As(runErr, &exitErr) → ExitCode() with err==nil for non-zero exits.
  pattern: "LookPath → CommandContext(args...) → separate buffers → errors.As(ExitError). previewDiff maps
            exit 0|1 → success(stdout), >1 → error."
  gotcha: "Do NOT route previewDiff through internal/git.Git — it is repo-bound (-C <repo>) and semantically
           a repo wrapper; `git diff --no-index` is repo-independent. Write a small self-contained helper."

- file: internal/exitcode/exitcode.go
  why: Success=0/Error=1/New(code,err)/*ExitError/errors.As. The engine is LIBRARY code — it returns PLAIN
       errors; S2 wraps them via exitcode.New(exitcode.Error, err) at the cobra RunE boundary. Do NOT import
       exitcode into the engine for control flow (it may be referenced for nothing here; prefer plain errors).

- file: go.mod
  why: confirms current deps are cobra/pflag/go-toml/v2 ONLY. yaml.v3 is ABSENT — this subtask adds NOTHING
       to go.mod (T2.S2 adds yaml.v3). The blockTarget test Target needs no third-party parser.
  gotcha: "if you find yourself reaching for yaml/toml in protocol.go or its tests, STOP — you are
           implementing a concrete target, which is T2's job. The engine + its tests are parser-free."
```

### Current Codebase tree (relevant slice)

```bash
internal/
  integrate/                   # NEW PACKAGE (this subtask creates it)
    protocol.go                # NEW — Target interface, Action/Outcome, ApplyOptions/ApplyResult,
                               #        ConfirmFunc, Apply, DefaultConfirm, BackupPath, previewDiff,
                               #        atomicWrite/restore helpers. (doc comment atop this file.)
    protocol_test.go           # NEW — blockTarget test vehicle + full matrix (golden/refuse/restore/
                               #        idempotent/create-if-missing/remove-noop/decline/diff).
  hook/
    script.go                  # REF — Marker/ScriptMode precedent (do NOT edit)
    hook.go                    # REF — marker-identity/foreign-refusal/MkdirAll analog (do NOT edit)
    hook_test.go               # REF — test style (t.TempDir, byte-unchanged assertions)
  cmd/config.go                # REF — runConfigUpgrade parse-gate-write analog (do NOT edit)
  ui/output.go                 # DEP — IsTerminal (DefaultConfirm TTY gate) (do NOT edit)
  git/git.go                   # REF — run() exec []string-args pattern (do NOT edit)
  exitcode/exitcode.go         # REF — exit codes (engine returns plain errors; S2 routes) (do NOT edit)
go.mod                         # UNCHANGED (no new require)
```

### Desired Codebase tree

```bash
# ONE new package, internal/integrate, with protocol.go (+ its test). No edits to any existing file.
# No cobra wiring (S2), no Target impls (T2), no yaml.v3 (T2.S2), no docs (Mode A: none directly — S2).
internal/integrate/
  protocol.go        # Target, Action, Outcome, ApplyOptions, ApplyResult, ConfirmFunc, Apply,
                     # DefaultConfirm, BackupPath, previewDiff (git --no-index), atomicWrite, restore
  protocol_test.go   # blockTarget + TestApply_* matrix
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the protocol IS the no-mangle guarantee): yaml.v3 (T2.S2's parser) re-encodes the WHOLE doc —
// it can drop blank lines and normalize comment spacing OUTSIDE the edited node (architecture §2, VERIFIED).
// So FR-I3(f) "surgical scope" is NOT a serializer property; it is delivered by the envelope: the Target's
// Upsert/Remove do the node edit, and the protocol wraps it in parse→diff→confirm→backup→validate so any
// incidental normalization is either caught by re-parse-validate (→ backup restored) or shown in the diff
// (→ user confirms). Do NOT try to make the engine "byte-perfect" via a serializer — make it TRANSPARENT.

// CRITICAL (git diff --no-index exit codes, VERIFIED): 0 = identical (EMPTY stdout), 1 = differences (the
// NORMAL case — NOT an error), >1 = real error. previewDiff returns stdout for both 0 and 1; errors only
// on >1 / LookPath miss / context cancel. Pass --no-color (the preview writer is plain). Pass `a/<base>`
// and `b/<base>` inside a temp dir run via `git -C <tmp>` for clean `--- a/<base>` / `+++ b/<base>` labels.

// CRITICAL (create-if-missing preview): do NOT pass a literally-absent path to git diff --no-index (it
// errors `Could not access`). ALWAYS materialize BOTH sides: write an EMPTY `a/<base>` when the original
// file is missing — the diff then shows all lines added under `@@ -0,0 +N,M @@` (reads as "new file").

// CRITICAL (atomic write = same filesystem): the temp file MUST be in filepath.Dir(Path) (os.CreateTemp
// in the target's own dir) so os.Rename is atomic. A temp in os.TempDir() can cross a filesystem boundary
// → rename becomes copy+unlink (non-atomic). Write bytes → os.Chmod(tmp, 0o644) → os.Rename(tmp, Path).

// CRITICAL (backup is RETAINED): FR-I3d says "write backup before modifying" — it does NOT say delete.
// Keep the <file>.stagecoach-backup.<unix-ts> on BOTH success and validate-failure (it is the user's
// safety record). On validate-failure, restore the in-memory orig over Path (equivalent to the backup)
// and leave the backup file in place; surface "<path> restored from backup <backup>".

// CRITICAL (write NOTHING unless confirmed+validated): Apply writes bytes to disk ONLY after (c) confirm
// passes AND only the final validated content. On parse error / Decline / Remove-no-op / upsert-identical,
// the file is untouched and NO backup is written. The backup is written only for a real modification.

// GOTCHA (engine is pure library): return PLAIN errors (fmt.Errorf %w). Do NOT call os.Exit, do NOT import
// cobra, do NOT import exitcode for control flow. S2 (the cmd layer) wraps results via exitcode.New at the
// RunE boundary. Writers are injected (opts.Out io.Writer; DefaultConfirm reads os.Stdin).

// GOTCHA (Validate must not rely on prior Parse state): the protocol calls Target.Validate(newBytes) AFTER
// Upsert/Remove already consumed the parsed state. Validate must use a LOCAL probe (e.g. a fresh
// Unmarshal) — not the instance's post-Parse state — so the post-write gate is clean and side-effect-free.

// GOTCHA (no third-party parser here): protocol.go + protocol_test.go import ONLY stdlib + internal/ui
// (for IsTerminal). If you need yaml/toml, you are writing a concrete Target — that is T2's scope. The
// blockTarget test vehicle is hand-rolled text parsing (strings.Split/Contains).

// GOTCHA (MkdirAll AFTER confirm): create parent dirs (FR-I3g) only once the user has confirmed — do not
// litter the filesystem for a Decline or a no-op. os.MkdirAll(filepath.Dir(Path), 0o755) sits between the
// confirm step and the atomic write.

// GOTCHA (idempotency vs backup): a second Upsert that produces byte-identical content is OutcomeNoChange
// (new==old) → no write, no backup. The FIRST upsert on an existing file (content changes) writes exactly
// one backup. Double-install never accumulates backups or duplicate entries.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/integrate/protocol.go  (NEW — package integrate)
package integrate // import "github.com/dustin/stagecoach/internal/integrate"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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

// Outcome is what Apply did, for the caller's (S2's) status report.
type Outcome int

const (
	OutcomeCreated  Outcome = iota // file did not exist; created with the entry (FR-I3g)
	OutcomeUpdated                 // file existed; marker entry inserted or replaced (FR-I3b)
	OutcomeRemoved                 // file existed; marker entry deleted
	OutcomeDeclined                // user answered N (or non-TTY auto-decline); NOTHING written (FR-I3c)
	OutcomeNoChange                // action is a no-op: remove a missing entry, or upsert identical bytes
)

// String renders Outcome for logs/verbose. (S2 maps these to user-facing verbs.)
func (o Outcome) String() string { /* Created/Updated/Removed/Declined/NoChange */ }

// Target is the format-specific adapter the protocol drives (PRD §9.21). Each file-editing integration
// target (lazygit, git-alias) implements this over its native parser. The protocol NEVER touches bytes
// except through these methods + the backup/atomic-write machinery: the Target owns the SURGICAL EDIT;
// the protocol owns the NO-MANGLE ENVELOPE (parse-first, diff, confirm, backup, validate).
//
// Stateful contract: Parse populates the target's in-memory state; HasEntry/Upsert/Remove read/mutate
// it. Validate must NOT depend on prior Parse state (use a local probe) so the post-write gate is clean.
type Target interface {
	// Marker is the identity string for stagecoach's contribution — a comment or well-known key whose
	// presence means "stagecoach owns this entry" (FR-I3b idempotency, FR-I3f surgical scope).
	Marker() string

	// Parse loads existing file content into the target's state. A non-nil error ⇒ unparseable file ⇒
	// the protocol HARD-REFUSES to write (FR-I3a) and surfaces this error verbatim. Parse is called ONLY
	// on content successfully read from disk (a missing file is the create-if-missing path, not a parse error).
	Parse(data []byte) error

	// HasEntry reports whether the marker-identified entry is present in the parsed state. Drives
	// idempotency (Upsert replaces, never duplicates — FR-I3b) and the Remove no-op (removing a missing
	// entry ⇒ OutcomeNoChange, nothing written).
	HasEntry() bool

	// Upsert returns new file bytes with the marker entry inserted (if absent) or replaced (if present).
	// It MUST be surgical (FR-I3f): only the marker entry changes semantically. (For YAML, incidental
	// whole-doc normalization is unavoidable — architecture §2; the protocol's diff+confirm surfaces it.)
	Upsert() ([]byte, error)

	// Remove returns new file bytes with the marker entry deleted. Removing a non-present entry returns
	// the original bytes unchanged (the protocol treats new==old as OutcomeNoChange).
	Remove() ([]byte, error)

	// Validate re-parses data to confirm well-formedness WITHOUT relying on or mutating prior Parse
	// state (use a local probe). The protocol calls it on the freshly-written bytes (FR-I3e); failure ⇒
	// restore the backup. Typically Validate(data) == Parse(data) on a throwaway instance.
	Validate(data []byte) error
}

// ConfirmFunc asks the user whether to apply the change at path (with diff already rendered to out).
// Returns true to proceed, false to skip without writing (OutcomeDeclined). Apply uses DefaultConfirm
// when opts.Confirm == nil.
type ConfirmFunc func(out io.Writer, path, diff string) bool

// ApplyOptions configures a single file's no-mangle write (PRD §9.21 FR-I3).
type ApplyOptions struct {
	Path    string      // absolute path to the target config file
	Target  Target      // the format-specific adapter (Parse'd by Apply)
	Action  Action      // ActionUpsert or ActionRemove
	Yes     bool        // --yes: skip the confirm prompt (scripts) — FR-I3c
	Out     io.Writer   // preview diff + status (cmd's stderr in prod); nil ⇒ os.Stderr
	Confirm ConfirmFunc // y/N prompt; nil ⇒ DefaultConfirm (os.Stdin, TTY-gated) — FR-I3c
}

// ApplyResult describes what Apply did, for the caller's (S2's) status report.
type ApplyResult struct {
	Outcome Outcome // Created/Updated/Removed/Declined/NoChange
	Path    string  // the target path (echoed back)
	Backup  string  // the backup file path written (empty if none was written)
}

// BackupPath returns the timestamped backup path for a file (FR-I3d): <file>.stagecoach-backup.<unix-ts>.
// Exported so S2/tests can predict/locate the backup. unixTs is seconds since epoch.
func BackupPath(path string, unixTs int64) string {
	return fmt.Sprintf("%s.stagecoach-backup.%d", path, unixTs)
}

// Apply runs the FR-I3 (a)–(g) no-mangle write protocol over a single file. See the step comments for the
// exact in-order sequence. Returns an ApplyResult (always non-nil on success) and a plain error on any
// failure (parse refusal, read/write/validate failure, git-diff error). NEVER calls os.Exit; the cmd layer
// (S2) routes exit codes. Writes NOTHING unless the change is confirmed and validated.
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

// DefaultConfirm is the FR-I3c y/N prompt used when ApplyOptions.Confirm is nil. It writes the unified
// diff + "Apply changes to <path>? [y/N]" to out, reads one line from os.Stdin, and accepts ONLY a line
// whose first non-space byte is 'y' or 'Y' (mirrors the lowercase-`y`-default of `N`). When stdin is NOT a
// terminal (a piped/scripted invocation without --yes) it AUTO-DECLINES without blocking — the safe
// default that never hangs a non-interactive run (--yes is the explicit script bypass).
func DefaultConfirm(out io.Writer, path, diff string) bool {
	if diff != "" {
		fmt.Fprint(out, diff)
		if !strings.HasSuffix(diff, "\n") {
			fmt.Fprintln(out)
		}
	}
	if !ui.IsTerminal(os.Stdin) { // non-interactive ⇒ do not block; the user passes --yes to force
		fmt.Fprintf(out, "stagecoach: non-interactive stdin — declining to modify %s (use --yes to apply)\n", path)
		return false
	}
	fmt.Fprintf(out, "Apply changes to %s? [y/N] ", path)
	var line string
	fmt.Fscanln(os.Stdin, &line) // best-effort; EOF/empty ⇒ decline
	line = strings.TrimSpace(line)
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

// previewDiff returns the unified diff between the original and new content via
// `git diff --no-index --no-color -- a/<base> b/<base>` (PRD §19: []string args, NO shell). Both sides are
// materialized as temp files under <tmpdir>/{a,b}/<base> and git is run with -C <tmpdir> so the ---/+++
// labels read `a/<basename>` / `b/<basename>`. When oldMissing is true the `a/<base>` side is written EMPTY
// (never an absent path — git would error `Could not access`); the diff then shows all lines added.
// Exit codes (VERIFIED git 2.54.0): 0 = identical (returns ""), 1 = differences (returns stdout), >1 = error.
func previewDiff(ctx context.Context, path string, oldBytes, newBytes []byte, oldMissing bool) (string, error) {
	gitPath, lerr := execLookPath() // exec.LookPath("git")
	if lerr != nil {
		return "", fmt.Errorf("git binary not found in PATH: %w", lerr)
	}
	base := filepath.Base(path)
	tmp, err := os.MkdirTemp("", "stagecoach-diff-")
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

// atomicWrite writes data to a temp file in the SAME directory as path (same filesystem ⇒ atomic rename),
// then renames it over path and chmods to 0o644 (FR-I3e). The temp file lives in filepath.Dir(path), NOT
// os.TempDir(), to avoid a cross-filesystem rename.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".stagecoach-*")
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

// restore writes orig back over path after a validate failure (FR-I3e). If the file was missing (create
// path), restore removes it; otherwise rewrites orig atomically. The backup file is RETAINED (FR-I3d).
func restore(path string, orig []byte, wasMissing bool) error {
	if wasMissing {
		return os.Remove(path) // we created it; validating failed ⇒ remove it again
	}
	return atomicWrite(path, orig)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/integrate/protocol.go — types + Target interface
  - IMPLEMENT the data-models block: package doc (one paragraph restating FR-I3), Action/Outcome enums
    (+ Outcome.String), Target interface (Marker/Parse/HasEntry/Upsert/Remove/Validate), ConfirmFunc,
    ApplyOptions, ApplyResult, BackupPath. NO Apply body yet (Task 3).
  - NAMING: Action/Outcome enums iota; ApplyResult fields Outcome/Path/Backup; ConfirmFunc(out,path,diff).
  - PLACEMENT: internal/integrate/protocol.go.

Task 2: CREATE internal/integrate/protocol.go — the git-diff + atomic-write + restore helpers
  - IMPLEMENT execLookPath()/runGit(ctx,gitPath,dir,args...) mirroring internal/git/git.go run() shape
    (LookPath → exec.CommandContext([]string, NO shell) → separate bytes.Buffer stdout/stderr →
    errors.As(ExitError) → code, err==nil for non-zero). KEEP them unexported.
  - IMPLEMENT previewDiff (the a/<base>,b/<base> temp-dir + `git -C <tmp> diff --no-index --no-color`
    form; empty `a/` side when oldMissing; exit 0|1→stdout, >1→error) and atomicWrite (CreateTemp in
    filepath.Dir(path) → Write → Close → Chmod 0o644 → Rename; defer os.Remove(tmp)) and restore.
  - GOTCHA: do NOT route through internal/git.Git (repo-bound); self-contained exec here.

Task 3: CREATE internal/integrate/protocol.go — Apply (FR-I3 a–g, in order)
  - IMPLEMENT Apply per the data-models block's step comments, IN ORDER: read+parse-first (a) → idempotent
    upsert / Remove-no-op (b) → new==old no-op → previewDiff (c) → confirm (c, --yes bypass, nil⇒Default) →
    backup (d) → MkdirAll parents (g, AFTER confirm) → atomicWrite (e) → Validate + restore-on-fail (e) →
    Outcome. Return plain errors (fmt.Errorf %w); NEVER os.Exit.
  - GOTCHA: writes NOTHING on parse-error/Decline/Remove-no-op/upsert-identical; writes a backup ONLY for a
    real modification of an existing file; retains the backup on success AND validate-failure.

Task 4: CREATE internal/integrate/protocol.go — DefaultConfirm (FR-I3c)
  - IMPLEMENT DefaultConfirm: write diff to out (ensure trailing newline); if !ui.IsTerminal(os.Stdin) →
    print one "non-interactive — declining (use --yes)" line + return false (no block); else print
    "Apply changes to <path>? [y/N] " + fmt.Fscanln(os.Stdin) + accept only y/Y-first. Import internal/ui
    ONLY for IsTerminal.

Task 5: CREATE internal/integrate/protocol_test.go — the blockTarget test vehicle
  - DEFINE blockTarget (in-package, unexported): Marker="# stagecoach-test-marker"; Parse scans lines for a
    START marker + END marker (`# end-stagecoach-test-marker`) — a START with no END ⇒ parse error (corrupt
    input); HasEntry = block present; Upsert ensures a fixed block (`<marker>\nmanaged-line\n<endmarker>`)
    is present, replacing any existing block's managed line, preserving all other lines verbatim; Remove
    deletes the block + markers (absent ⇒ original bytes); Validate re-scans for balance. A `badValidate`
    field flips Validate to always-fail. A configurable `managedLine` lets golden round-trips vary content.
  - Helper applyToFile(t, target, action, yes, confirm) (ApplyResult, error) over a t.TempDir() path.

Task 6: CREATE internal/integrate/protocol_test.go — the matrix
  - TestApply_UpsertCreatesMissing: missing file, ActionUpsert, Yes=true → OutcomeCreated; file exists with
    the block; parent dir created; NO backup.
  - TestApply_UpsertIdempotentDoubleInstall: Upsert twice (Yes=true) on an existing file → first
    OutcomeUpdated (one backup, block present); second OutcomeNoChange (new==old, NO new backup, file
    byte-identical to after the first). The golden round-trip.
  - TestApply_UpsertReplacesNotDuplicates: existing block with stale content → Upsert → block content
    replaced (exactly ONE block, not two); surrounding lines preserved verbatim (surgical scope, FR-I3f).
  - TestApply_RemoveDeletesOnlyEntry: file with block + foreign lines → Remove → block gone, foreign lines
    intact; OutcomeRemoved; one backup.
  - TestApply_RemoveMissingIsNoOp: file WITHOUT the block → Remove → OutcomeNoChange; file UNCHANGED; no backup.
  - TestApply_CorruptInputRefusal: seed an unbalanced marker (START, no END) → Upsert → error wrapping the
    parse error; file byte-identical to before; NO backup written (parse refusal precedes backup).
  - TestApply_DeclineWritesNothing: Confirm stub returns false → OutcomeDeclined; file UNCHANGED; no backup;
    no parent dir created (MkdirAll runs AFTER confirm).
  - TestApply_BackupWritten: existing file, real Upsert change, Yes=true → res.Backup is
    <path>.stagecoach-backup.<ts>; backup file exists and equals orig bytes; target file equals newBytes.
  - TestApply_ValidateFailureRestoresBackup: badValidate=true → error wrapping "validation failed"; target
    file restored to orig bytes (byte-identical); backup file RETAINED; res.Backup non-empty.
  - TestApply_NonTTYAutoDecline: Confirm==nil + stdin not a terminal (test cannot easily fake a TTY, so
    assert the BEHAVIOR via a Confirm==nil path is non-blocking — prefer: assert DefaultConfirm returns
    false when os.Stdin is piped, tested directly, OR gate the Apply test on an injected Confirm that mimics
    the rule). Keep deterministic: test DefaultConfirm(false-TTY) logic via a focused unit test, and test
    Apply's Confirm==nil path with a Confirm that returns a fixed bool.
  - TestPreviewDiff_NonEmptyIffChanged: previewDiff(orig, new, false) is "" iff bytes.Equal; non-empty and
    contains the +/- managed line when changed; create-if-missing (oldMissing) ⇒ diff shows all-added.
  - TestBackupPath_Format: BackupPath("/x/y", 1700000000) == "/x/y.stagecoach-backup.1700000000".
  - TestOutcome_String: the five outcomes render their names.

Task 7: VERIFY build/test/lint (no go.mod change)
  - go build ./... ; go test ./internal/integrate/... -v ; go test ./... ; go vet ./... ;
    golangci-lint run ; gofmt -l internal/integrate/. Confirm go.mod is UNCHANGED (git diff go.mod empty).
```

### Implementation Patterns & Key Details

```go
// runGit mirrors internal/git/git.go run() but is repo-independent (git diff --no-index needs no repo).
// []string args, exec.CommandContext, NO shell (PRD §19). Separate stdout/stderr buffers. errors.As(ExitError)
// ⇒ code with err==nil for non-zero exits (git uses exit codes as semantic signals).
func runGit(ctx context.Context, gitPath, dir string, args ...string) (stdout, stderr string, code int, err error) {
	full := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, gitPath, full...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	runErr := cmd.Run()
	stdout, stderr = out.String(), errb.String()
	if runErr == nil {
		return stdout, stderr, 0, nil
	}
	if cerr := ctx.Err(); cerr != nil {
		return stdout, stderr, -1, cerr
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return stdout, stderr, exitErr.ExitCode(), nil // non-zero ⇒ code, err==nil
	}
	return stdout, stderr, -1, runErr
}

// The blockTarget test vehicle — parser-free (strings only), deterministic. This is the SHAPE every real
// Target (lazygit/git-alias) will take; it proves the engine before any parser exists.
type blockTarget struct {
	marker      string // "# stagecoach-test-marker"
	endMarker   string // "# end-stagecoach-test-marker"
	managedLine string // the line Upsert installs (default "managed-line")
	badValidate bool   // inject a Validate failure (the backup/restore test)
	lines       []string // post-Parse state (the file's lines)
	hasEntry    bool
}

func (t *blockTarget) Parse(data []byte) error {
	t.lines = strings.Split(string(data), "\n")
	// balanced-marker check: a START with no following END ⇒ corrupt input (parse error)
	start, end := -1, -1
	for i, l := range t.lines {
		if strings.TrimSpace(l) == t.marker {
			if start != -1 {
				return fmt.Errorf("duplicate start marker")
			}
			start = i
		}
		if strings.TrimSpace(l) == t.endMarker {
			end = i
		}
	}
	if start != -1 && end == -1 {
		return fmt.Errorf("unbalanced marker: %s without %s", t.marker, t.endMarker) // corrupt-input refusal
	}
	t.hasEntry = start != -1
	return nil
}
// ... HasEntry/Upsert (ensure the 3-line block present, replacing managed line, preserving others) /
//     Remove (drop the block) / Validate (re-Parse into a throwaway; honor badValidate).
```

### Integration Points

```yaml
PROVIDES (to P1.M4.T1.S2 and P1.M4.T2.S1/S2 — the consumers):
  - integrate.Target            # the interface every file-editing target implements
  - integrate.Apply(ctx, opts)  # the FR-I3 (a)-(g) engine; returns (ApplyResult, error)
  - integrate.ApplyOptions      # {Path, Target, Action, Yes, Out, Confirm}
  - integrate.ApplyResult       # {Outcome, Path, Backup} for S2's status report
  - integrate.Action            # ActionUpsert | ActionRemove
  - integrate.Outcome           # Created/Updated/Removed/Declined/NoChange (+ String)
  - integrate.ConfirmFunc       # func(out, path, diff) bool
  - integrate.DefaultConfirm    # the y/N prompt S2 gets for free when Confirm==nil
  - integrate.BackupPath        # <file>.stagecoach-backup.<unix-ts>

CONSUMES (do NOT re-implement):
  - internal/ui.IsTerminal (DefaultConfirm's non-TTY auto-decline gate).
  - the shell-out-to-git ethos (PRD §19) via the in-package runGit helper (NOT internal/git.Git).

OUT OF SCOPE (owned by sibling subtasks — do NOT implement):
  - S2: the `integrate list|install|remove` cobra commands, the target registry, detection gating (FR-I1/I2),
        the --yes flag wiring, *ui.UI/color, exit-code routing, user-facing docs (Mode A).
  - T2.S1: the git-alias Target (FR-I4 delegates the edit to `git config`).
  - T2.S2: the lazygit Target (comment-preserving YAML via yaml.v3 Node API, FR-I5) — adds yaml.v3 to go.mod.
  - Any concrete Target, any cobra command, any new third-party dependency, any user-facing doc edit.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/integrate/protocol.go internal/integrate/protocol_test.go
go build ./...            # the new package must compile against internal/ui + stdlib only
go vet ./...
golangci-lint run
# go.mod MUST be unchanged (no new require — yaml.v3 is T2.S2's addition):
git diff --name-only go.mod && echo "WARN: go.mod touched" || echo "go.mod clean OK"
# Expected: zero errors; go.mod clean.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/integrate/... -v
# Expected: all pass — UpsertCreatesMissing, UpsertIdempotentDoubleInstall (golden round-trip),
# UpsertReplacesNotDuplicates, RemoveDeletesOnlyEntry, RemoveMissingIsNoOp, CorruptInputRefusal,
# DeclineWritesNothing, BackupWritten, ValidateFailureRestoresBackup, PreviewDiff_NonEmptyIffChanged,
# BackupPath_Format, Outcome_String.
go test -race ./internal/integrate/...
# Expected: clean (atomicWrite uses os.Rename; backup/restore have no shared state, but -race confirms).
```

### Level 3: Integration Testing (System Validation)

```bash
# The engine is library code; "integration" = drive Apply through the real git-diff path (not a stub).
go build -o /tmp/stagecoach ./cmd/stagecoach   # confirms the package compiles into the binary (S2 wires it later)
# Manual smoke (a throwaway text file + the blockTarget shape via a tiny Go test, OR wait for S2's command):
#   - create a file with a balanced marker block → Upsert twice → diff shows the change once, then a no-op.
#   - create an unbalanced marker → Apply refuses, file untouched.
# git diff --no-index sanity (the preview backend):
printf 'a\nb\n' > /tmp/o.txt; printf 'a\nB\nb\n' > /tmp/n.txt
git -C /tmp diff --no-index --no-color -- o.txt n.txt | sed 's/^/  /'   # shows the +/- line, exit 1
rm -f /tmp/o.txt /tmp/n.txt
# Expected: the engine's previewDiff produces the same unified-diff hunk shape.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Backup/restore resilience: inject a Validate failure and confirm the file is byte-identical to the
# pre-Apply snapshot (the never-mangle guarantee under failure). Covered by TestApply_ValidateFailureRestoresBackup;
# re-run with -count=1 to defeat the test cache:
go test ./internal/integrate/... -run ValidateFailureRestoresBackup -v -count=1
# Idempotency under repeated install (the "run it 10 times" stress):
go test ./internal/integrate/... -run IdempotentDoubleInstall -v -count=1
# Expected: file stable across repeated Apply; exactly one backup for the first real change; no duplicates.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...`; `go test ./...`; `go vet ./...`; `golangci-lint run`; `gofmt -l` clean.
- [ ] `go.mod` UNCHANGED (no new `require` — yaml.v3 is T2.S2's, not this subtask's).
- [ ] No `os.Exit`, no `cobra`, no `exitcode` control-flow import in `internal/integrate` (pure library).
- [ ] `previewDiff` uses `exec.CommandContext` with `[]string` args, NO shell (PRD §19); exit 0|1→stdout, >1→error.

### Feature Validation
- [ ] (a) Unparseable file → hard refusal; parse error surfaced; file UNCHANGED; no backup written.
- [ ] (b) Upsert is idempotent: double-install ⇒ second is NoChange, one block (no duplicate), one backup max.
- [ ] (c) Every real modification is preceded by a non-empty unified diff + y/N; `--yes` bypasses; `Confirm==nil`
      non-TTY auto-declines; Decline ⇒ nothing written, no backup, no parent dir.
- [ ] (d) `<file>.stagecoach-backup.<unix-ts>` written before every real modification of an existing file; retained
      on success AND validate-failure.
- [ ] (e) Atomic write (temp+rename in the target's own dir); post-write Validate failure restores orig + retains backup.
- [ ] (f) Surgical scope: only the marker entry changes; surrounding lines preserved verbatim (golden round-trip).
- [ ] (g) Missing file + parent dirs created via the same preview+confirm flow (AFTER confirm); Outcome=Created.
- [ ] Remove-of-missing ⇒ NoChange, nothing written; Upsert-identical ⇒ NoChange, nothing written.

### Code Quality Validation
- [ ] The engine is format-agnostic and parser-free (only stdlib + internal/ui.IsTerminal).
- [ ] `Target` is the single extension point; S2/T2 implement it, they do NOT fork Apply.
- [ ] All writers are injected (`opts.Out`, `ConfirmFunc`); tests are deterministic (no real stdin/TTY).
- [ ] Follows existing conventions: t.TempDir tests, byte-unchanged assertions, sentinel-free plain errors,
      `[]string` exec args, marker-identity + foreign-refusal lineage from internal/hook.

### Documentation & Deployment
- [ ] Package doc comment atop `protocol.go` restates FR-I3 (a)–(g) in one paragraph (Mode A: no separate doc —
      user-facing protocol behavior is documented with S2).
- [ ] No README/docs edits in this subtask (S2 owns the user-facing surface).

---

## Anti-Patterns to Avoid

- ❌ Don't add yaml.v3 / go-toml / any diff library — the engine is parser-free; the diff is `git diff --no-index` (shell-out, PRD §19); concrete parsers are T2's job.
- ❌ Don't route `previewDiff` through `internal/git.Git` — it is repo-bound (`-C <repo>`); `--no-index` is repo-independent. Use a small self-contained `exec.CommandContext` helper.
- ❌ Don't pass an absent path to `git diff --no-index` (it errors `Could not access`) — always materialize both sides; empty `a/<base>` for create-if-missing.
- ❌ Don't write the temp file in `os.TempDir()` for the atomic write — it can cross a filesystem and make `os.Rename` non-atomic. Temp in `filepath.Dir(path)`.
- ❌ Don't write ANYTHING before confirm, or write a backup for a Decline/no-op/parse-refusal — the backup exists only for a real, confirmed modification.
- ❌ Don't delete the backup on success or on validate-failure — FR-I3d retains it as the user's safety record; validate-failure restores orig content but keeps the backup file.
- ❌ Don't create parent dirs before confirm — a Decline must not litter the filesystem (MkdirAll sits AFTER confirm).
- ❌ Don't make `Validate` depend on the instance's post-Parse state — use a local probe so the post-write gate is side-effect-free.
- ❌ Don't call `os.Exit`, import cobra, or import exitcode for control flow — the engine is pure library code returning plain errors; S2 routes exit codes.
- ❌ Don't implement the `integrate list|install|remove` commands, the target registry, detection gating, or any concrete Target (git-alias/lazygit) — those are S2 / T2.S1 / T2.S2.
- ❌ Don't block on stdin when it's not a TTY and `--yes` is unset — auto-decline (the safe default); `--yes` is the explicit script bypass.

---

## Confidence Score

**9/10** — one-pass success likelihood is high. The contract is precisely specified (exact `Target` method
set + semantics, exact `Apply` step sequence a–g with every branch point, verified `git diff --no-index`
behavior, atomic-write + backup file naming, `ConfirmFunc` injection + `DefaultConfirm` TTY rule, and a
complete parser-free test vehicle with its corrupt-input and bad-Validate variants). The single residual
uncertainty is cosmetic: the `git -C <tmp> diff --no-index a/<base> b/<base>` `diff --git` header reads
`a/a/<base>` (the `-C` prefix doubles the `a/`) — the `---`/`+++` lines are clean (`a/<base>`/`b/<base>`)
and that is what the user reads; if S2 wants the header relabeled it is a one-line post-process, not an
engine redesign. No new dependency, no cobra, no concrete target — the scope is tight and the consumers
(S2/T2) are named as contracts.
