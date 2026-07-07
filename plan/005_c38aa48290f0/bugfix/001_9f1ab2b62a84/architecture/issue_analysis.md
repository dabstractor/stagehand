# Issue Analysis — Root Causes + Fix Designs

## Issue 1 (Major): `integrate install lazygit` silently creates duplicate `<c-a>` key binding

### Root Cause
`lazygitEntry.Install()` (`internal/cmd/integrate_lazygit.go`, ~line 163-178) delegates entirely to `integrate.Apply(ActionUpsert)` with **no foreign-key check**. `lazygitTarget.Upsert()` (~line 80-110) only looks for the **marker** (`isStagecoachItem` checks `item.Content[1].LineComment` contains `lazygitMarker`). An unmarked entry sharing the same key is invisible to Upsert, so it **APPENDS** a new marked entry → two `customCommands` entries bound to `<c-a>`.

Because `customCommands` is a YAML *sequence*, two entries can legally share a key (unlike git config where a single key maps to one value). So the install produces a real conflicting duplicate.

### Fix Design
Mirror the git-alias target's foreign-conflict handling. The helper `lazygitTarget.findKeyItem(key)` already exists (~line 206) and is used by `Status()` to detect `StatusForeign`. It finds an **unmarked** item whose `key` value == key.

**In `lazygitEntry.Install()`**, before calling `integrate.Apply`:
1. Read the config file (best-effort — `os.ReadFile(e.resolvedPath())`; missing file → no foreign key possible, skip).
2. Parse with a throwaway `lazygitTarget{key: e.key}` (`tgt.Parse(data)`).
3. Call `tgt.findKeyItem(e.key)` to check for an unmarked conflicting entry.
4. If found, print a WARNING to `opts.Out`:
   `WARNING: a %s binding already exists (not managed by stagecoach); installing will create a duplicate customCommands entry — use --key to choose a different binding.`
5. Proceed with `integrate.Apply(ActionUpsert)` as before (the diff+confirm flow lets the user decline).

This surfaces the conflict in ALL cases: interactive (WARNING + diff + confirm prompt), and `--yes` (WARNING + immediate write). It restores parity with git-alias's `WARNING: alias.<name> is currently set to "<value>" (not stagecoach) — it will be overwritten.`

**Pattern reference** (`integrate_gitalias.go` ~line 130-137):
```go
if found { // foreign (not ours) — surface before overwriting (FR-I4)
    preview += fmt.Sprintf("\nWARNING: %s is currently set to %q (not stagecoach) — it will be overwritten.\n",
        e.aliasKey(), cur)
}
```

**Test pattern** (`integrate_gitalias_test.go` `TestGitAlias_Install_ForeignConflictInPreview`):
- Pre-write a config with a foreign entry.
- Call Install with a `Confirm` func that captures the diff.
- Assert the diff/warning contains "WARNING".
- For lazygit: the WARNING goes to `opts.Out` (stderr), so capture stderr and assert it contains "WARNING" + "duplicate".

**Existing test to extend**: `TestLazygitEntry_Status_States` already tests `StatusForeign` detection (~line 696). A new `TestLazygitEntry_Install_ForeignKeyWarning` should be added.

---

## Issue 2 (Minor): `hook exec` prints "Generating…" progress line for no-op sources and empty diffs

### Root Cause
`runHookExec` (`internal/cmd/hookexec.go`, ~line 128-131) prints `u.Progress(ui.ProgressLabel("Generating", msgModel, labelProvider))` **unconditionally** before calling `hook.Run()`. `hook.Run()` returns `hook.ErrNoOp` immediately for source-gated no-ops (`NoOpSource(source)`) or empty diffs (`diff == ""`) — but the misleading progress line is already on stderr.

Because git invokes `prepare-commit-msg` on every commit, a user with the hook installed sees this noise on *every* `git commit -m "…"`.

### Fix Design
Move the progress emission to AFTER the no-op gates pass. The cleanest approach: add an optional `Progress func()` callback to `generate.Deps`, and have `hook.Run` call it after the source-gate AND empty-diff checks pass. `runHookExec` sets the callback instead of printing directly.

**`generate.Deps`** (internal/generate/generate.go, line 25-30):
```go
type Deps struct {
    Git      git.Git
    Manifest provider.Manifest
    Verbose  *ui.Verbose
    Excludes []string
    Progress func() // optional; hook.Run calls it after no-op gates pass (nil-safe)
}
```

**`hook.Run()`** (internal/hook/exec.go): after the `NoOpSource` check AND the `diff == ""` check, add:
```go
if deps.Progress != nil {
    deps.Progress()
}
```

**`runHookExec`** (internal/cmd/hookexec.go): replace the unconditional `u.Progress(...)` with:
```go
deps := generate.Deps{
    Git:      g,
    Manifest: m,
    Verbose:  verbose,
    Excludes: excludes,
    Progress: func() { u.Progress(ui.ProgressLabel("Generating", msgModel, labelProvider)) },
}
```

This ensures the progress line fires ONLY when generation is actually about to run. `generate.CommitStaged` (the default action path) does not set `Progress` — it has its own progress printing in `default_action.go` — so this change is nil-safe and does not affect the default path.

**Test pattern**: `TestHookExec_SourceGateExit0` already tests the source-gate path. Extend it to assert `errBuf` does NOT contain "Generating". Add a test for empty-diff silence.

---

## Issue 3 (Minor): `config init --template` reference config omits v2.1 `[generation]` keys

### Root Cause
`exampleConfigTemplate`'s `[generation]` section (`internal/cmd/config.go`, ~lines 565-578) lists 8 keys but is MISSING: `exclude`, `format`, `locale`, `template`, `push`. The docs (`docs/configuration.md` lines 105-108, 131-134, 167-170, 192-193) DO document these keys.

### Fix Design
Add 5 commented lines to the `[generation]` block in `exampleConfigTemplate`, mirroring the wording from `docs/configuration.md`. Insert them in logical order (matching docs/configuration.md's commented example):

```toml
# [generation]
# max_diff_bytes        = 300000  # byte cap on the non-markdown diff section
# max_md_lines          = 100     # per-file line cap for markdown diffs
# max_duplicate_retries = 3       # re-generation attempts when the subject duplicates a recent commit
# subject_target_chars  = 50      # target subject-line length for truncation
# output                = "raw"   # agent output mode: "raw" | "json" — applies to parsing across ALL providers
# strip_code_fence      = true    # strip ``` fences from agent output (all providers)
# max_commits           = 12      # safety cap on auto-decompose (PRD §9.14 FR-M4); default 12
# binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
# exclude               = []      # gitignore-style globs; UNION across global+repo+flag (§9.18 FR-X1)
# format                = "auto"  # auto|conventional|gitmoji|plain; unknown = hard error (exit 1) (§9.19 FR-F1)
# locale                = ""      # free-form language name or BCP-47 tag; never validated (§9.19 FR-F6)
# template              = ""      # wrap every message; must contain literal $msg, e.g. "$msg (#205)" (§9.19 FR-F8)
# push                  = false   # run `git push` after a fully-successful run; on failure commits stand (§9.22 FR-P1)
# NOTE: [generation] output/strip_code_fence override any per-provider [provider.<name>] values.
```

**Exact insertion point**: After the `binary_extensions` line and before the `NOTE:` line in the `[generation]` block (~line 578).

**Config struct reference** (`internal/config/config.go`):
- `Exclude []string` (line 103) — `toml:"exclude"` — default nil
- `Format string` (line 85) — `toml:"format"` — default "auto"
- `Locale string` (line 89) — `toml:"locale"` — default ""
- `Template string` (line 93) — `toml:"template"` — default ""
- `Push bool` (line 120) — `toml:"push"` — default false

**Test pattern**: grep the template output for the 5 keys.

---

## Issue 4 (Minor): `ui.IsTerminal` treats `/dev/null` as a terminal

### Root Cause
`IsTerminal` (`internal/ui/output.go`, ~line 25-30) tests `stat.Mode() & os.ModeCharDevice != 0`. `/dev/null` IS a character device, so `IsTerminal(/dev/null)` returns `true`. The code comment explicitly notes "NOT a true isatty ioctl."

Consequences:
- `config init --interactive < /dev/null` bypasses the FR-L3 TTY gate → crashes with "unexpected end of input" instead of the clean "requires a terminal" message.
- `DefaultConfirm` with `/dev/null` stdin doesn't take the documented auto-decline path (reads EOF → declines by accident, but skips the "non-interactive stdin — declining" notice).

### Fix Design
Replace the char-device heuristic with a **true isatty ioctl probe** using stdlib `syscall` (no new dependency — consistent with the project's dep-free principle and the procgroup pattern).

**Platform-specific files** (following procgroup_unix.go / procgroup_windows.go convention):

1. **`internal/ui/isatty_unix.go`** (`//go:build !windows`):
   - The ioctl constant differs: Linux uses `TCGETS`, Darwin/BSD uses `TIOCGETA`. Split further:
     - `isatty_linux.go` (`//go:build linux`): `const ioctlReadTermios = syscall.TCGETS`
     - `isatty_darwin.go` (`//go:build darwin`): `const ioctlReadTermios = syscall.TIOCGETA`
     - Shared `isatty_ioctl.go` (`//go:build !windows`): `func isTerminalFd(fd uintptr) bool` using `syscall.Syscall(SYS_IOCTL, fd, ioctlReadTermios, ...)`.
   - NOTE: `syscall.TCGETS` exists in Go stdlib on Linux; `syscall.TIOCGETA` on Darwin. Both use `syscall.SYS_IOCTL`.
   - Implementation:
     ```go
     func isTerminalFd(fd uintptr) bool {
         var t syscall.Termios  // or [128]byte for portability
         _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, uintptr(ioctlReadTermios),
             uintptr(unsafe.Pointer(&t)), 0, 0, 0)
         return errno == 0
     }
     ```
   - Use a `[128]byte` raw buffer instead of `syscall.Termios` to avoid struct-layout differences across BSD variants.

2. **`internal/ui/isatty_windows.go`** (`//go:build windows`):
   - Use `kernel32.dll!GetConsoleMode` (resolving via `syscall.NewLazyDLL` — same pattern as `procgroup_windows.go`).
   - `/dev/null` doesn't exist on Windows (it's `NUL`), so the char-device issue doesn't apply, but a proper console mode check is still more correct.

3. **Update `IsTerminal`** in `output.go`:
   ```go
   func IsTerminal(f *os.File) bool {
       return isTerminalFd(f.Fd())
   }
   ```
   This replaces the char-device heuristic entirely.

**Consideration**: The `fd` is `uintptr` from `f.Fd()`. On Unix, stdin=0, stdout=1, stderr=2. The ioctl will succeed (errno==0) only on a real terminal/pty; `/dev/null` (a char device but NOT a terminal) will return errno!=0.

**Test**: Add `TestIsTerminal_DevNull` that opens `/dev/null` and asserts `IsTerminal` returns false. (Platform-gated or skip on systems without `/dev/null`.)

**Callers impacted** (all benefit, no signature change):
- `config_init_interactive.go` line 20 (`interactiveStdinIsTTY`)
- `integrate/protocol.go` line 251 (`DefaultConfirm`)
- `hookexec.go` line 130 (color resolution)
- `default_action.go` line 46 (color resolution)
