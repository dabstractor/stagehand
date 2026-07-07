# Critical Implementation Findings & Gotchas

> **MUST-READ for every implementing agent.** These are issues discovered during research that
> deviate from naive assumptions and require explicit handling.

## FINDING 1: Rootless repo — `git rev-parse HEAD` returns literal `"HEAD"` (NOT empty)

**The trap:** On a repository with zero commits, `git rev-parse HEAD`:
- Prints the literal string `HEAD\n` to **stdout** (not empty!)
- Writes a fatal error to stderr: `fatal: ambiguous argument 'HEAD': unknown revision...`
- Exits with code **128**

**Verified by execution:**
```
$ git rev-parse HEAD; echo "EXIT=$?"
fatal: ambiguous argument 'HEAD': unknown revision or path not in working tree.
HEAD
EXIT=128
```

**Why this matters:** The zsh `commit-pi` script does `PARENT_SHA=$(git rev-parse HEAD 2>/dev/null)`
and then checks `[[ -n "$PARENT_SHA" ]]`. On a rootless repo, `PARENT_SHA` would be the string
`"HEAD"` (non-empty), so it would proceed to `git commit-tree -p HEAD ...` which would FAIL.
This is a **latent bug in commit-pi** that never triggers because the author always works in
repos with existing commits.

**Correct Go implementation:** Check the **exit code**, NOT string emptiness:
```go
func RevParseHEAD(ctx) (sha string, isUnborn bool, err error) {
    cmd := exec.CommandContext(ctx, "git", "-C", repo, "rev-parse", "HEAD")
    var out, errb bytes.Buffer
    cmd.Stdout, cmd.Stderr = &out, &errb
    runErr := cmd.Run()
    if runErr != nil {
        if exitErr, ok := runErr.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
            return "", true, nil  // unborn repo — NOT an error
        }
        return "", false, fmt.Errorf("git rev-parse HEAD: %w; stderr: %s", runErr, errb.String())
    }
    return strings.TrimSpace(out.String()), false, nil
}
```
When `isUnborn`: `PARENT_SHA = ""`, `commit-tree` called without `-p` (root commit),
`update-ref HEAD <new>` called without the expected-old arg (or with the all-zeros hash).

---

## FINDING 2: Codex `--ask-for-approval` is NOT a `codex exec` flag

The PRD manifest (§12.7) lists `bare_flags = ["--sandbox", "read-only", "--ask-for-approval", "never"]`.
But `codex exec --help` does NOT list `--ask-for-approval` — it exists only on the interactive
`codex` command. `codex exec` is already non-interactive.

**Resolution:** Drop `--ask-for-approval never`. Use `--sandbox read-only --ephemeral`.
The implementing agent should verify `codex exec` doesn't block on approval (it shouldn't — it's
non-interactive by design). See `external_deps.md` §codex for full details and alternatives.

---

## FINDING 3: `git update-ref` CAS failure messages vary by version + scenario

The exact stderr on CAS failure depends on the scenario:
- **HEAD moved to a different real SHA:** `fatal: cannot lock ref 'HEAD': is at <actual> but expected <expected-old>`
- **Expected-old = all-zeros but HEAD exists:** `fatal: update_ref failed for ref 'HEAD': cannot lock ref 'HEAD': reference already exists`

**Implication:** Do NOT substring-match the error message. Treat **exit code ≠ 0** as the CAS-failure
signal. The implementing agent must surface a clear message: "HEAD moved from <PARENT> to <actual>
while generating; aborting to avoid a non-fast-forward." (Re-read current HEAD for the `<actual>`.)

---

## FINDING 4: `git commit-tree -m` vs `-F -` for message delivery

- `-m "<msg>"` works but each `-m` is a separate paragraph. For a single message with embedded
  newlines (subject + body), pass the WHOLE message as ONE `-m` arg, OR use `-F -` (stdin).
- **Recommendation:** Use `-F -` (read message from stdin) to avoid ALL quoting issues with
  special characters, leading dashes, quotes, and newlines. This is the safest cross-platform choice.
  Go passes args as `[]string` (no shell), so `-m` with the full message also works, but `-F -`
  via `cmd.Stdin = strings.NewReader(msg)` is bulletproof.

---

## FINDING 5: pelletier/go-toml/v2 does NOT support `omitempty` in struct tags

Unlike `encoding/json`, go-toml/v2 has **no `omitempty`**. This affects two things:
1. **Manifest marshaling** (`providers show`): all struct fields are emitted, including zero values.
   Use pointer fields (`*string`, `*bool`) or a custom `MarshalTOML()` to suppress empty fields.
2. **Config overlay/merge:** cannot distinguish "field not present in TOML" from "field = zero value"
   for plain types. Use pointer types (`*bool` for `auto_stage_all`, etc.) or decode to
   `map[string]any` first to detect key presence.

**Provider manifest merge:** When a user override sets only `default_model`, ALL other fields must
remain from the built-in manifest. The merge must be **field-by-field** (only override non-zero/non-nil
fields from the user config), NOT a wholesale struct replacement. See `go_ecosystem_patterns.md` §2.4
and §5.4 for the overlay pattern.

---

## FINDING 6: `git diff --cached --quiet` exit codes are inverted from usual convention

- Exit **0** = NO staged differences (index == HEAD) — "nothing staged"
- Exit **1** = staged differences EXIST — "something staged"
- Exit **>1** = real error

A naive `if err != nil { /* assume error */ }` would treat exit 1 (something staged) as an error.
Must check `*exec.ExitError.ExitCode() == 1` explicitly as "has staged changes."

---

## FINDING 7: Diff byte-capping has no built-in git flag

`git diff` has no `--max-bytes`. The commit-pi script pipes through `head -c 300000`. In Go:
- Capture stdout to a `bytes.Buffer` or `io.LimitedReader`.
- For markdown files, the per-file line cap (`head -n 100`) is also a post-capture truncation.
- When truncation occurs, append a `\n... [diff truncated at N bytes/lines]` sentinel so the
  model knows the diff is incomplete.

**Diff capture structure (matching commit-pi):**
1. Markdown files (`.md`, `.markdown`): `git diff --cached -- '<file>'` per file, capped at `max_md_lines` (100) lines each.
2. Non-markdown files: `git diff --cached -- ':!*.lock' ':!package-lock.json' ':!pnpm-lock.yaml' ':!yarn.lock' ':!*.snap' ':!*.map' ':!vendor/*' ':!*.md' ':!*.markdown'`, capped at `max_diff_bytes` (300000) bytes total.
3. Concatenate markdown + other into one payload.

---

## FINDING 8: Signal handling requires process-group kill (`Setpgid`)

Because the agent subprocess is placed in its own process group (`SysProcAttr.Setpgid=true`), it
does NOT receive the terminal's Ctrl-C directly. Stagecoach MUST explicitly forward SIGINT/SIGTERM
to the child's process group via `syscall.Kill(-pid, sig)`.

Go 1.20+ provides `cmd.Cancel` and `cmd.WaitDelay` for clean context-based killing:
- `cmd.Cancel`: called when the context is cancelled; should `syscall.Kill(-pid, SIGTERM)`.
- `cmd.WaitDelay`: after Cancel, wait this long (e.g. 3s) before Go forcibly SIGKILLs.

**The signal handler** (installed via `signal.Notify` for SIGINT/SIGTERM):
1. If a child is running, forward the signal to its process group.
2. If the snapshot was taken, run the rescue path.
3. Cancel the context.
4. Restore the default signal handler before the final `update-ref` (so a Ctrl-C at the very last
   instant isn't mistaken for a failure — matching commit-pi's `trap - INT TERM`).

See `go_ecosystem_patterns.md` §3 and §4 for the canonical code patterns.

---

## FINDING 9: `git log --format='---%n%B'` delimiter can collide with markdown content

The commit-pi script uses `git log --format="---%n%B" -20` and splits on `---`. But `---` is also
a valid markdown horizontal rule that could appear inside a commit body. **More robust:** use a NUL
byte delimiter: `git log --format='%x00%B' -20` and split on `\x00`. This cannot occur in commit
message text. Alternatively, the Go implementation can parse the output by counting records.

---

## FINDING 10: Windows `SysProcAttr.Setpgid` is Unix-only

`Setpgid` is Linux/macOS only. On Windows, process-group killing requires Job Objects. Since the
PRD targets Linux/macOS/Windows × amd64/arm64 (§21.2), the signal/process-kill code needs a
build-tag-segregated abstraction:
- `internal/executil/exec_unix.go` (build tag `!windows`): `Setpgid` + `syscall.Kill(-pid, sig)`.
- `internal/executil/exec_windows.go` (build tag `windows`): `CREATE_NEW_PROCESS_GROUP` + `GenerateConsoleCtrlEvent` or Job Objects.

This is a **cross-platform portability task** that must be handled. The git binary itself works
fine on Windows; it's the Go process management that differs.

---

## FINDING 11: Auto-stage-all default and the nothing-to-commit path

When `auto_stage_all = true` (default) and nothing is staged:
1. Run `git add -A`.
2. Re-check `git diff --cached --quiet`.
3. If still nothing (clean tree): exit 2, "nothing to commit."
4. If now staged: proceed, print a notice `Nothing staged — staging all changes (N files).`

`--no-auto-stage` disables this (exit with "nothing staged" instead).
`--all` / `-a` forces `git add -A` even when something IS already staged (stages additional files).

This must be in the CLI layer, NOT in `commitStaged`, to preserve the v2 decomposition boundary (§11.3).
