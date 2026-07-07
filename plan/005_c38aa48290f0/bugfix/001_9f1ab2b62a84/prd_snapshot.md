# Bug Fix Requirements

## Overview

Creative end-to-end validation of the v2.1 "competitor feature parity + tool integrations" delta (§9.18–§9.23) against PRD.md. Testing was performed by building the binary, setting up real/temp git repositories, exercising every shipped feature through its user-facing surface, and inspecting payloads, prompts, config files, and git state directly.

The implementation is largely solid: the project builds clean, `go vet`/`go test ./...` pass, and the core happy paths for all six feature areas work (exclusions emit `[excluded]` placeholders and stay payload-only; format/locale/context/template ordering is correct per §17.8; `--template`/`--format` unknown-value hard errors fire; hook install/uninstall/status + foreign-refusal work; `--edit` editor gate respects empty-message abort semantics; `--push` streams and fails-correctly with no upstream; git-alias install/remove handles foreign conflicts; `models` lists/falls back per FR-L1).

One Major issue and three Minor issues were found. The Major issue (silent duplicate key binding on lazygit install) directly contradicts the integrate feature's central "never mangle a config file" guarantee.

## Critical Issues (Must Fix)

None.

## Major Issues (Should Fix)

### Issue 1: `integrate install lazygit` silently creates a duplicate `<c-a>` key binding when a foreign one already exists
**Severity**: Major
**PRD Reference**: §9.21 ("its write protocol is the point: it must be impossible for stagecoach to mangle a config file"), FR-I1 (`foreign` status), FR-I3 (no-mangle protocol / surgical scope), FR-I5 (lazygit target). Contradicts the stated guarantee and is inconsistent with FR-I4's git-alias foreign-conflict handling.
**Expected Behavior**: When lazygit's config already contains a `customCommands` entry bound to the target key (e.g. `<c-a>`) that is NOT stagecoach-marked, installing stagecoach should at minimum surface the conflict to the user before writing (e.g. a warning that a conflicting `<c-a>` exists and will result in a duplicate), analogous to the git-alias target which prints `WARNING: alias.<name> is currently set to "<value>" (not stagecoach) — it will be overwritten.` before proceeding. Because `customCommands` is a YAML *sequence*, two entries can legally share a key — unlike git config, where a single key has one value — so the install produces a real conflicting binding rather than an overwrite.
**Actual Behavior**: `lazygitEntry.Install()` (internal/cmd/integrate_lazygit.go) calls `integrate.Apply(ActionUpsert)` with no foreign-key check. `lazygitTarget.Upsert()` only looks for the **marker** (`# stagecoach-integration`); an unmarked entry sharing the same key is invisible to it, so it APPENDS a new marked entry. The result is two `customCommands` entries bound to `<c-a>` — a duplicate that conflicts in lazygit. No warning is printed, and the preview diff shows only the new addition (no conflict call-out). With `--yes` (common in scripts/automation) the duplication is entirely silent. (`integrate list` correctly reports `foreign`, but `install` does not act on it.)
**Steps to Reproduce**:
1. Build: `go build -o /tmp/stagecoach ./cmd/stagecoach`.
2. Create a lazygit config containing a foreign `<c-a>` binding (unmarked):
   ```
   mkdir -p /tmp/lg/.config/lazygit
   cat > /tmp/lg/.config/lazygit/config.yml <<'EOF'
   customCommands:
     - key: '<c-a>'
       description: 'My existing AI commit'
       command: 'some-other-tool'
       output: none
   EOF
   ```
3. Install stagecoach's entry: `HOME=/tmp/lg /tmp/stagecoach integrate install lazygit --yes` (lazygit must be on PATH, or use a shim that answers `--print-config-dir`).
4. Inspect the config: `grep -c "key: '<c-a>'" /tmp/lg/.config/lazygit/config.yml` → returns `2` (the foreign entry plus the stagecoach entry). No warning was printed.
**Suggested Fix**: In `lazygitEntry.Install()` (internal/cmd/integrate_lazygit.go), mirror the git-alias target: after `Parse`, probe for an *unmarked* item whose key equals the target binding (`lazygitTarget.findKeyItem(key)` already exists and is used by `Status`). If one is found, surface a `WARNING: a <key> binding already exists (not managed by stagecoach); installing will create a duplicate customCommands entry — use --key to choose a different binding.` line to `opts.Out`, and (ideally) still require the normal confirm so the user is forced to acknowledge it. This restores parity with FR-I4 and the §9.21 no-mangle promise.

## Minor Issues (Nice to Fix)

### Issue 2: `hook exec` prints the "Generating…" progress line for no-op sources and empty diffs
**Severity**: Minor
**PRD Reference**: §9.20 FR-H4 ("Exit 0 immediately (no generation) when `<source>` is any of `message`, `template`, `merge`, `squash`, `commit` … Also no-op when the staged diff is empty.").
**Expected Behavior**: For a source-gated no-op (`message`/`template`/`merge`/`squash`/`commit`) or an empty staged diff, `hook exec` should be silent (no generation is attempted).
**Actual Behavior**: `runHookExec` (internal/cmd/hookexec.go) prints `u.Progress(ui.ProgressLabel("Generating", …))` *unconditionally before* calling `hook.Run`. `hook.Run` then returns `hook.ErrNoOp` immediately (source gate / empty diff) without invoking the agent — but the misleading "Generating with <provider>…" line has already been emitted to stderr. Because git invokes `prepare-commit-msg` on every commit, a user with the hook installed sees this noise on *every* `git commit -m "…"` even though no generation occurs.
**Steps to Reproduce**:
1. Install the hook in a repo, ensure a stub/global provider is configured.
2. `echo x >> file; git add file; git commit -m "manual message"` — observe `↳ Generating with <provider>…` printed to stderr despite the message being supplied via `-m` (source=`message`, a no-op).
   - Direct form: `printf '# c\n' > /tmp/m; /tmp/stagecoach hook exec /tmp/m message` → prints `↳ Generating with <provider>…` then no-ops (exit 0, file untouched).
**Suggested Fix**: Move the `u.Progress(...)` call to *after* the source/empty-diff gates have passed — i.e. emit it only when generation is actually about to run (inside `hook.Run` after the `NoOpSource`/empty-diff checks, or have `runHookExec` call a lightweight pre-check before printing).

### Issue 3: `config init --template` reference config omits the v2.1 `[generation]` keys
**Severity**: Minor
**PRD Reference**: §9.19 FR-F1/F6/F8, §9.22 FR-P1, §9.18 FR-X1 (the new `[generation]` keys: `exclude`, `format`, `locale`, `template`, `push`); also docs/configuration.md line 112 ("documents every available option") and the example in PRD §16.2.
**Expected Behavior**: The inert reference config written by `config init --template` (`exampleConfigTemplate`) should document every available `[generation]` option, including the five v2.1 keys, as commented lines — matching its stated purpose and the shipped docs.
**Actual Behavior**: `exampleConfigTemplate`'s `[generation]` section (internal/cmd/config.go, ~lines 565–578) lists `max_diff_bytes`, `max_md_lines`, `max_duplicate_retries`, `subject_target_chars`, `output`, `strip_code_fence`, `max_commits`, `binary_extensions` — but is missing `exclude`, `format`, `locale`, `template`, and `push`. docs/configuration.md *does* document these keys (lines 105–108, 131–134, 167–170, 192–193), so the generated template is inconsistent with both the docs and its own header comment.
**Steps to Reproduce**:
1. `/tmp/stagecoach config init --template --config /tmp/ref.toml`.
2. `grep -E "exclude|format|locale|template|push" /tmp/ref.toml` → no matches in the `[generation]` section (only `config_version` and unrelated `generation`-section headings appear).
**Suggested Fix**: Add the five commented lines to `exampleConfigTemplate`'s `[generation]` block, mirroring the wording already present in docs/configuration.md (e.g. `# exclude = []`, `# format = "auto"`, `# locale = ""`, `# template = ""`, `# push = false` with their one-line descriptions and the `$msg` / unknown-mode-hard-error notes).

### Issue 4: `ui.IsTerminal` treats `/dev/null` (a character device) as a terminal
**Severity**: Minor
**PRD Reference**: §9.23 FR-L3 (`config init --interactive` TTY gate: non-TTY → exit 1 pointing at plain `config init`); §9.21 FR-I3c (the integrate `DefaultConfirm` non-interactive auto-decline: "When stdin is NOT a terminal … it AUTO-DECLINES without blocking").
**Expected Behavior**: A stdin redirected from `/dev/null` (a common non-interactive pattern) should be detected as non-TTY so the interactive gate exits with the intended "requires a terminal" hint, and so `DefaultConfirm` auto-declines without attempting to read.
**Actual Behavior**: `ui.IsTerminal` (internal/ui/output.go, ~lines 25–30) tests `stat.Mode() & os.ModeCharDevice != 0`. `/dev/null` *is* a character device, so `IsTerminal(os.Stdin)` returns `true` when stdin is `/dev/null`. Consequences: (a) `config init --interactive < /dev/null` bypasses the FR-L3 TTY gate, prints "Detected providers" + the prompt, then fails with `stagecoach: unexpected end of input` (exit 1) instead of the clean `--interactive requires a terminal on stdin` message; (b) `DefaultConfirm` with `/dev/null` stdin does not take the documented non-interactive auto-decline path (it attempts to read, gets EOF, and declines — safe by accident, but the explicit "non-interactive stdin — declining" notice is skipped). Real pipes/files are detected correctly (they are not char devices), so the impact is limited to the `/dev/null`-redirect case.
**Steps to Reproduce**:
1. `/tmp/stagecoach config init --interactive --force < /dev/null` → prints "Detected providers" + "Pick a provider [pi]: " then "stagecoach: unexpected end of input" (exit 1), instead of the FR-L3 "requires a terminal on stdin" message.
**Suggested Fix**: Distinguish `/dev/null`/char-device from a real terminal — e.g. check that stdin is a char device AND that an `ioctl(TIOCGETA`/`TCGETS`-equivalent) succeeds (a true isatty), or explicitly exclude `/dev/null` by path. golang.org/x/term's `IsTerminal` (or an equivalent ioctl probe) is the conventional fix; the code comment already notes "NOT a true isatty ioctl".

## Testing Summary
- Total tests performed: ~40 (manual end-to-end across all six feature areas + targeted code/payload inspection).
- Passing: the vast majority of functional scenarios (happy paths, edge cases, error handling, idempotency for hook/git-alias/lazygit, format/template/context ordering, payload-only exclusion guarantee, edit abort, push failure semantics).
- Failing: 1 Major (lazygit duplicate key binding) + 3 Minor (hook exec progress noise, reference template missing v2.1 keys, IsTerminal /dev/null misfire).
- Areas with good coverage: exclusions (union, negation, placeholders, payload-only guarantee across all three diff paths); message shaping (format modes, locale, context ordering with/without rejection, template hard-error + ordering vs dedupe); hook mode (install/uninstall/status, foreign refusal, never-block, source gate, message-file write preserving comments); `--edit` (append/empty-abort); `--push` (success + no-upstream failure, commits-stand); git-alias (install/remove/foreign-conflict); `models` (live list + curated fallback + error cases); docs coherence (README surfaces all six features + FR-H7 FAQ).
- Areas needing more attention: lazygit foreign-key conflict handling on install (Issue 1); the four Minor items above.
