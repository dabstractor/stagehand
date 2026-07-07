# P1.M4.T1.S1 — Codebase research notes (no-mangle write protocol engine)

Research date: 2026-07-02. Scope: the **engine** only (`internal/integrate/protocol.go`) — the
`Target` interface + `protocol.Apply` orchestrating PRD §9.21 **FR-I3 (a)–(g)**. Does NOT implement
the `integrate list|install|remove` command surface (that is **S2**) nor the `git-alias`/`lazygit`
targets (that is **T2.S1/S2**). This subtask is **foundational** (no input dependencies) and is
consumed by S2 + T2.

## 1. The load-bearing research: yaml.v3 CANNOT guarantee byte-identity (architecture §2, VERIFIED)

From `plan/005_c38aa48290f0/architecture/external_deps.md` §2 (empirically verified, yaml.v3 v3.0.1):

- `yaml.Node` carries `HeadComment`/`LineComment`/`FootComment` + `Style`.
- **Byte-identity OUTSIDE the edited node is NOT guaranteed** — yaml.v3 re-encodes the whole document.
  A zero-edit round-trip still (a) dropped a blank line between sections, (b) normalized inline-comment
  spacing. Quote style + comment text/placement WERE preserved.
- Default encoder indent is **4**; call `enc.SetIndent(2)` to match conventional configs.
- go-yaml is **archived (2025)** — no upstream fixes coming; design around it.

**Consequence (the work item's RESEARCH NOTE, echoed here):** the PRD's FR-I3(f) "preserved outside the
edited node" must be satisfied via **the PROTOCOL, not the serializer**. The protocol IS the no-mangle
guarantee: parse-first refusal → node edit → re-parse validate → timestamped backup → atomic write
(temp+rename) → **unified-diff preview so any incidental normalization is VISIBLE and CONFIRMED by the
user before writing**. The Target's `Upsert`/`Remove` do the surgical node edit; the protocol wraps it in
the parse→diff→confirm→backup→validate→atomic-write envelope so a normalization surprise is either (e)
caught by re-parse-validate (and the backup restored) or (c) shown in the diff and confirmed.

> yaml.v3 is NOT currently a dependency (`go.mod` has only cobra/pflag/go-toml/v2). **This subtask does
> NOT add it** — the engine is format-agnostic and is tested with an in-package text-block Target.
> T2.S2 (lazygit) adds yaml.v3 and the concrete YAML Target. The engine must not assume any parser.

## 2. The unified-diff preview: shell out to `git diff --no-index` (VERIFIED, git 2.54.0)

The work item RESEARCH NOTE: prefer shelling to `git diff --no-index -- <old> <new>` (zero new deps,
matches the repo's shell-out-to-git ethos, PRD §19) over vendoring a diff library. **Verified:**

- `git diff --no-index --no-color -- a/<base> b/<base>` (with the two content versions written into
  `tmpdir/a/<base>` and `tmpdir/b/<base>`, run via `git -C <tmpdir>`) produces a clean unified diff whose
  `---`/`+++` lines label `a/<basename>` / `b/<basename>` — the real config filename (the `diff --git`
  header line reads `a/a/<base>` due to the `-C` prefix; cosmetic, ignored).
- **Exit codes**: `0` = identical (empty stdout); `1` = differences (the NORMAL case here, NOT an error);
  `>1` = real error. Treat 0 and 1 as success (return stdout, empty if 0).
- **Create-if-missing (FR-I3g)**: write an EMPTY `tmpdir/a/<base>` + the new content to
  `tmpdir/b/<base>` → diff shows all lines added under `@@ -0,0 +N,M @@` (clean, reads as "new file").
  Do NOT pass a literally-absent path (git errors `Could not access`); always materialize both sides.
- `--no-index` does NOT require a repo and works outside one. It DOES require `git` on `$PATH`
  (`exec.LookPath`). Pass `--no-color` (the preview writer is plain; color is the cmd layer's concern).
- Run via `exec.CommandContext(ctx, gitPath, args...)` with `[]string` args, **NO shell** (PRD §19),
  separate stdout/stderr buffers (mirror `internal/git/git.go` `run()` structure). This is a NEW
  self-contained helper in `protocol.go` — do NOT route through `internal/git.Git` (it is repo-bound via
  `-C <repo>` and is semantically a repo wrapper; `--no-index` is repo-independent).

## 3. Existing patterns to follow (CONTRACT)

### internal/ui (the preview/confirm/TTY precedent named in the work item)
- `ui.IsTerminal(f *os.File) bool` (`output.go`) — stdlib-only TTY heuristic (char-device). The DefaultConfirm
  uses it to AUTO-DECLINE when stdin is not a TTY and `--yes` is not set (don't block a piped invocation).
- `ui.New(stdout, stderr io.Writer, color bool)` — injectable writers (buffers in tests). The engine takes a
  plain `io.Writer` for the preview diff (the cmd layer passes its stderr / a `*ui.UI`-derived writer).
- There is NO existing y/N prompt in the codebase — this subtask ADDS `DefaultConfirm` (reads os.Stdin, y/N,
  TTY-gated) inside `internal/integrate`. It is the precedent S2 will reuse.

### internal/hook (the closest "manage a file the no-mangle way" analog)
- `hook.Install/Uninstall/Detect` (`hook.go`) + `Marker`/`ScriptMode` (`script.go`): the **marker-identity +
  foreign-refusal + idempotent-rewrite + never-clobber + creates-parent-dirs** pattern. FR-I3b/f/g mirror it.
- `hook_test.go`: the **test style** — `t.TempDir()` + real files, byte-for-byte "unchanged" assertions on
  the foreign-refusal path, table-driven `Status.String()`. The protocol tests follow this exactly.
- **GOTCHA**: hook writes via plain `os.WriteFile` (NOT atomic temp+rename) — it is a single owned file with
  no parse concern. The protocol is STRICTER: it adds parse-first refusal + backup + atomic write +
  re-validate because it edits ARBITRARY user config files (FR-I3 the whole point).

### internal/cmd/config.go `runConfigUpgrade` (the closest parse→gate→write analog)
- Reads file → `toml.Unmarshal` probe (non-strict) → on parse error `exitcode.New(Error, ...)` (**refuse to
  mangle**, exactly FR-I3a's "unparseable file is never written to") → surgical line edit → `os.WriteFile`.
- It does NOT do backup/atomic-write/diff-preview/confirm — those are what THIS subtask ADDS as the general
  engine. `runConfigUpgrade` is the precedent for the parse-error-hard-refusal + `exitcode.New` routing only.

### internal/git/git.go `run()` (the exec []string-args pattern to mirror for the diff helper)
- `exec.LookPath("git")` → `exec.CommandContext(ctx, gitPath, full...)` (`full` is `[]string`, NO shell) →
  separate `bytes.Buffer` for stdout/stderr → `errors.As(runErr, &exitErr)` to recover `ExitCode()` with
  `err==nil` for non-zero exits (git uses exit codes as semantic signals). The diff helper reuses this shape
  but maps exit `0|1`→success(stdout), `>1`→error.

### internal/exitcode/exitcode.go
- `Success=0`, `Error=1`, `New(code, err)`, `*ExitError` with `errors.As`. The engine returns PLAIN errors
  (it is library code, not a cobra RunE); the cmd layer (S2) wraps them via `exitcode.New(exitcode.Error, err)`.

## 4. The Target interface design (the contract S2/T2 implement)

Format-agnostic; the protocol NEVER touches bytes except through these methods + the backup/atomic-write
machinery. The Target owns the **surgical edit**; the protocol owns the **no-mangle envelope**.

```go
type Target interface {
    Marker() string                  // identity string for stagecoach's contribution (FR-I3b/f)
    Parse(data []byte) error         // load existing content into state; err ⇒ HARD refuse (FR-I3a)
    HasEntry() bool                  // marker entry present in parsed state (idempotency / Remove no-op)
    Upsert() ([]byte, error)         // insert-or-replace the marker entry (surgical, FR-I3b/f)
    Remove() ([]byte, error)         // delete the marker entry; missing entry ⇒ bytes unchanged
    Validate(data []byte) error      // re-parse to confirm well-formedness, NO state reliance (FR-I3e)
}
```

Stateful: `Parse` populates the target's in-memory state; `HasEntry`/`Upsert`/`Remove` read/mutate it.
`Validate` must NOT depend on prior `Parse` state (use a local probe) so the post-write gate is clean — the
protocol calls it on the freshly-written bytes; on failure it restores the backup (FR-I3e). For most real
targets `Validate(data)` is `return t.Parse(data)` on a throwaway, or a separate `Unmarshal(&probe)`.

## 5. The protocol.Apply algorithm (FR-I3 a–g, in order)

Inputs: `ApplyOptions{Path, Target, Action(Upsert|Remove), Yes, Out io.Writer, Confirm ConfirmFunc}`.

```
1. out := opts.Out (or os.Stderr)
2. orig, rerr := os.ReadFile(Path); missing := errors.Is(rerr, os.ErrNotExist)
     rerr && !missing → return err (read failure)
3. if !missing:                                    # (a) parse first
        if perr := Target.Parse(orig); perr != nil → HARD REFUSE: return err wrapping perr (NEVER write)
        exists := Target.HasEntry()
     else: exists = false
4. switch Action:                                  # (b) idempotent upsert / Remove no-op
        Remove: if missing || !exists → OutcomeNoChange, return (nothing to remove)
                newBytes = Target.Remove()
        Upsert: newBytes = Target.Upsert()
     err → return err
5. if !missing && bytes.Equal(orig, newBytes) → OutcomeNoChange, return   # upsert identical / idempotent
6. diff := previewDiff(orig|empty, newBytes, missing)   # (c) git diff --no-index (temp a/<base>, b/<base>)
7. if !Yes:                                        # (c) confirm
        confirm := opts.Confirm; if nil → DefaultConfirm
        if !confirm(out, Path, diff) → OutcomeDeclined, return (NOTHING written)
8. if !missing: backup := Path + ".stagecoach-backup." + unixTs; WriteFile(backup, orig, 0o644)   # (d)
9. MkdirAll(filepath.Dir(Path), 0o755)             # (g) create-if-missing parent dirs (AFTER confirm)
10. atomicWrite(Path, newBytes):                   # (e) temp file in SAME dir + os.Rename + Chmod 0o644
11. if verr := Target.Validate(newBytes); verr != nil:   # (e) re-parse validate
        restore orig over Path (from in-memory orig; backup file RETAINED)
        return err wrapping verr + "restored backup <backup>"
12. return Outcome{Created if missing else (Updated|Removed)}, Backup
```

- **Backup retention**: the timestamped backup is RETAINED on both success and validate-failure (FR-I3d
  says "write before modifying"; it does not say delete). On validate-failure the in-memory `orig` is
  restored over the target (equivalent to the backup) and the backup file is kept as the user's record.
- **Atomic write**: temp file via `os.CreateTemp(dir, ".stagecoach-*")` in `filepath.Dir(Path)` (same
  filesystem ⇒ rename is atomic), write bytes, `os.Chmod(tmp, 0o644)`, `os.Rename(tmp, Path)`.
- **ConfirmFunc injection** (`func(out io.Writer, path, diff string) bool`): tests pass a deterministic
  bool-returning stub; `opts.Confirm == nil` ⇒ `DefaultConfirm` (prints the diff + `Apply changes to <path>? [y/N]`
  to `out`, reads one line from `os.Stdin`, accepts only a `y`/`Y`-first answer; if `!ui.IsTerminal(os.Stdin)`
  and `!Yes` ⇒ auto-decline without blocking).

## 6. The test Target (the protocol is format-agnostic ⇒ test it with an in-package fake)

`protocol_test.go` defines a `blockTarget` over a marker-delimited text block (NOT yaml/toml — keeps the
engine dep-free and deterministic):

```
<arbitrary lines>
# stagecoach-test-marker
managed-line
# end-stagecoach-test-marker
<arbitrary lines>
```

- `Parse`: scan lines; a START marker with no matching END marker ⇒ parse error (the **corrupt-input
  refusal** case). A balanced block (or no block) parses clean.
- `HasEntry`: block present.
- `Upsert`: ensure the block is present with a fixed `managed-line` (insert if absent; replace if present
  with different content). Preserves all other lines verbatim (surgical).
- `Remove`: delete the block + its markers; absent ⇒ bytes unchanged.
- `Validate`: re-scan; a `badValidate` variant flips `Validate` to always-fail → the **backup/restore**
  case.

Test matrix (from the work item): golden-file round-trips (upsert twice ⇒ identical file;
insert+remove ⇒ original bytes), corrupt-input refusal (unbalanced marker → Parse err → file UNCHANGED,
no backup), backup/restore on injected validation failure (badValidate → file restored to orig, backup
retained, error surfaced), idempotent double-install (Upsert twice ⇒ second is OutcomeNoChange, one
backup at most / file stable), create-if-missing (missing file ⇒ Created, parent dirs made), Remove of a
missing entry ⇒ OutcomeNoChange, Decline (Confirm returns false ⇒ nothing written, no backup), and the
git-diff preview is non-empty exactly when newBytes != orig.

## 7. Out of scope (owned by sibling subtasks — do NOT implement here)

- **S2** (`integrate list|install|remove` command surface + detection gating, FR-I1/I2): the cobra
  commands, the target registry, `--yes` flag, wiring `*ui.UI`/color, exit-code routing. This subtask
  provides `Apply`/`Target`/`DefaultConfirm` for S2 to call.
- **T2.S1** (`git-alias`): a Target impl + the FR-I4 "git does the edit" delegation.
- **T2.S2** (`lazygit`): the comment-preserving YAML Target (adds yaml.v3, FR-I5).
- User-facing docs (Mode A says "none directly" — protocol behavior is documented with S2).
