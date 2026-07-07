# Bug Fix Requirements

## Overview

End-to-end validation of the **Hook execution on the commit path** feature (v2.4, FR-V1–V8, → G22).
The feature was tested against PRD §9.25 and git's actual hook semantics, using a real compiled
`stagehand` binary with a stub agent, exercising the single-commit path (`generate.CommitStaged`),
the dry-run path (`pkg/stagehand.runPipeline`), the decompose path (`decompose.publishCommit` /
arbiter resolution), and direct git-parity comparisons against `git commit`.

**Overall assessment:** the core mechanism works well — hooks fire in git's documented order,
`--no-verify` skips the right subset, the FR-V3 freeze backstop catches swept paths, the FR-V7
rescue path fires on hook failure (exit 3), post-commit is best-effort, and FR-V4 recursion
prevention correctly skips stagehand's own `prepare-commit-msg`. **However, four real bugs were
found**, all divergences from documented git parity. Three corrupt commit content or state; one
makes a documented configuration layer non-functional. All four reproduce deterministically.

Testing method: throwaway `git init` repos + real shell-script hooks + the stub agent, with
side-by-side comparison against `git commit` for the same hook. All findings include exact
reproduction steps and were verified against git 2.54.0.

## Critical Issues (Must Fix)

None. The core feature is functional; no finding prevents hook execution from working at all.

## Major Issues (Should Fix)

### Issue 1: prepare-commit-msg invoked with 2 args (`<msgfile> ""`) but git passes 1 arg for a plain commit

**Severity**: Major
**PRD Reference**: §9.25 FR-V2 ("`prepare-commit-msg <msg-file> ""` with an empty source —
identical to a plain `git commit`"); architecture/open_questions.md §2 (explicitly directed the
implementer to verify and match git's argv)
**Expected Behavior**: For a plain commit (no `-m`/merge/squash/amend), git invokes
`prepare-commit-msg <msgfile>` with **one** argument — the `source` parameter is ABSENT (not the
empty string `""`). `$#` is 1 and `$2` is unset.
**Actual Behavior**: stagehand invokes `prepare-commit-msg <msgfile> ""` with **two** arguments
(`$#`=2, `$2` set to the empty string `""`). The runner source comment at
`internal/hooks/runner.go:52` and `:178` claims **"VERIFIED argc=2 for a plain commit"** — this
verification never actually happened (or was done incorrectly); direct testing of git 2.54.0 shows
`argc=1`.
**Steps to Reproduce**:
1. `git init` a repo, seed a commit, stage a change.
2. Install a `prepare-commit-msg` that logs `$#`:
   ```sh
   #!/bin/sh
   echo "ARGC=$#" > /tmp/args.txt
   ```
3. `git commit` (plain, via `GIT_EDITOR=true git commit`) → result file shows `ARGC=1`.
4. `stagehand --provider stub` (stub outputs any message) → result file shows `ARGC=2`.
**Impact**: Hooks that branch on `$#` (e.g. `[ "$#" -eq 1 ] && …`) misbehave under stagehand.
Most common hooks (husky, commitlint) use `[ -z "$2" ]` which works either way, so the practical
blast radius is narrow — but it is a documented git-parity guarantee that is violated, and the
in-source verification claim is false (a signal that the divergence is unintentional).
**Suggested Fix**: In `internal/hooks/runner.go` `runPrepareCommitMsg`, change the argv from
`[]string{msgPath, ""}` to `[]string{msgPath}` (single arg). The architecture research
(external_deps.md §2) already directed this: "If `argc=1`, pass the file only." Correct the
"VERIFIED argc=2" comments at lines 52 and 178.

### Issue 2: Hook message file written WITHOUT a trailing newline — prepare-commit-msg appends corrupt the subject line

**Severity**: Major
**PRD Reference**: §9.25 FR-V2 ("faithfully emulating `git commit`'s hook ordering"); FR-V4
("stagehand writes its generated … message to the message file, then runs `prepare-commit-msg`")
**Expected Behavior**: git writes the commit message to the message file FOLLOWED BY A TRAILING
NEWLINE (verified: `git commit -m "feat: change"` writes `feat: change\n` to COMMIT_EDITMSG). A
`prepare-commit-msg` hook that appends a line (e.g. a `Signed-off-by` trailer, a branch-name
annotation, a ticket ref) produces a clean multi-line message.
**Actual Behavior**: stagehand writes the message with `msgFile.WriteString(finalMsg)` and NO
trailing newline (`internal/hooks/runner.go:103`). When a hook appends via `echo "…" >> "$1"`, the
appended content is concatenated directly onto the subject line, corrupting it. The
comment-stripping (which splits on `\n`) cannot recover the line boundary, so the corruption lands
in the commit.
**Steps to Reproduce** (side-by-side with git):
```sh
# Identical hook in both cases:
cat > .git/hooks/prepare-commit-msg << 'EOF'
#!/bin/sh
echo "Signed-off-by: Dev <dev@example.com>" >> "$1"
EOF
chmod +x .git/hooks/prepare-commit-msg

# git commit (single-line -m message):
git commit -m "feat: change"
git log -1 --format=%B   # → "feat: change\nSigned-off-by: Dev <dev@example.com>\n"  (CORRECT)

# stagehand (stub outputs "feat: change"):
stagehand --provider stub
git log -1 --format=%B   # → "feat: changeSigned-off-by: Dev <dev@example.com>"  (CORRUPTED)
```
Hex dump confirms: git's file is `feat: change\n` (`0a` trailer); stagehand's is `feat: change`
(no trailer), so the `>>` append merges into one line.
**Impact**: The `Signed-off-by` trailer pattern (Linux kernel, corporate contribution agreements,
`git commit -s` parity), branch-name hooks, and ticket-ref-injecting hooks ALL corrupt the subject
when the generated message is single-line — which is the most common commit-message shape. This is
the single highest-impact finding: it silently produces wrong commit messages for a mainstream
workflow. The same bug affects the decompose path (same `RunCommitHooks`) and is masked by the
existing decompose/arbiter hook tests, which assert only `strings.Contains(msg, "[HOOK-RAN]")`
rather than checking line boundaries.
**Suggested Fix**: In `internal/hooks/runner.go`, after `msgFile.WriteString(finalMsg)`, ensure the
file ends with a newline if `finalMsg` does not (e.g. `if !strings.HasSuffix(finalMsg, "\n") {
finalMsg += "\n" }` before writing, mirroring git's COMMIT_EDITMSG format). This is the minimal
fix that restores git parity for append-style hooks.

### Issue 3: The `no_verify` git-config layer is entirely missing (no reader) AND the documented key name is invalid for git

**Severity**: Major
**PRD Reference**: §9.25 FR-V5 ("Surfaced as CLI `--no-verify`, env `STAGEHAND_NO_VERIFY`, git
config `stagehand.no_verify`"); §15.2 flag table (`git config` column = `stagehand.no_verify`);
§9.8 FR34 (precedence: … per-repo git config …)
**Expected Behavior**: `no_verify` resolves through the full 5-layer precedence documented in
`config.go:130` ("`--no-verify` / `STAGEHAND_NO_VERIFY` / `stagehand.no_verify` /
`[generation].no_verify`"), mirroring the established `push` key (which IS read at
`internal/config/git.go:174` via `gitConfigBool(repoDir, "stagehand.push")` and IS settable via
`git config stagehand.push true`).
**Actual Behavior**: Two distinct defects in the same layer:
1. **The git-config reader is missing.** `internal/config/git.go` `loadGitConfig` never queries
   `stagehand.no_verify` (grep for `no_verify`/`noVerify` in git.go returns 0 matches). The
   field is read from flag (`load.go:447`), env (`load.go:315`), and file (`file.go:291`), but
   NOT from git config. So the precedence is only 4 layers, contradicting the doc comment and
   the PRD.
2. **The documented key name is invalid for git.** `git config stagehand.no_verify true` fails
   with `error: invalid key: stagehand.no_verify` — git config variable names (the final segment
   after the last dot) cannot contain underscores. So even if the reader existed, the standard
   way to set the key does not work. (All other stagehand git-config keys avoid underscores by
   using camelCase — e.g. `stagehand.autoStageAll`, `stagehand.maxDiffBytes` — so this is a
   pattern break introduced by this feature.)
**Steps to Reproduce**:
```sh
git config stagehand.push true      # → works (no underscore)
git config stagehand.no_verify true # → "error: invalid key: stagehand.no_verify"
```
And in code: `grep -c "no_verify" internal/config/git.go` → `0` (the reader is absent).
**Impact**: A user who follows the docs (`docs/cli.md:44`, `docs/configuration.md:155`) and runs
`git config stagehand.no_verify true` to persistently bypass hooks for a repo gets an error. Even
manually editing `.git/config` to add the key has no effect — stagehand never reads it. The
`--no-verify` flag and `STAGEHAND_NO_VERIFY` env DO work (verified), so this is a
documented-surface gap, not a total failure. The feature was explicitly scoped to "mirror Push"
(see `codebase_reality.md §2` and the task context) but the git-config reader was never added.
**Suggested Fix**: (a) Add a git-config reader for `no_verify` in `loadGitConfig` mirroring the
`push` reader at `git.go:174` (`gitConfigBool(repoDir, "stagehand.<valid-name>")`). (b) Use a
git-valid key name — either match the existing camelCase convention (`stagehand.noVerify`) or a
dash form (`stagehand.no-verify`). (c) Update `config.go:130`, `docs/cli.md:44`, and
`docs/configuration.md:155` to the corrected, settable key name. If the TOML file key
(`[generation].no_verify`) must stay snake_case, that is fine (TOML allows underscores) — only
the git-config key needs the fix.

### Issue 4: A hook that empties the message file produces a commit with an EMPTY message (git aborts)

**Severity**: Major
**PRD Reference**: §9.25 FR-V2 ("faithfully emulating `git commit`'s hook ordering"); §9.8/§13.2
(the atomic-commit core must not land a bad commit)
**Expected Behavior**: If `prepare-commit-msg` or `commit-msg` empties the message file (a common
rejection / force-re-edit pattern), git aborts: `Aborting commit due to empty commit message.`
(exit 1, no commit created). Stagehand should do the same — an empty message after hooks must not
become a commit.
**Actual Behavior**: After `RunCommitHooks` returns, `generate.CommitStaged` reassigns
`msg = fm` (the hook-adjusted message) and passes it directly to `CommitTree` with NO empty-message
check (`internal/generate/generate.go:431-439`). An emptied message flows straight through and
creates a commit whose message is empty (or a lone `\n`). The decompose path has the same gap
(`internal/decompose/message.go` `publishCommit`: `RunCommitHooks` → `CommitTree(finalTree,
parents, finalMsg)` with no guard). Notably, the `--edit` path (`EditMessage` in `finalize.go`)
DOES guard against this (`if edited == "" { return ErrEmptyMessage }`), so the omission is
specific to the hooks path.
**Steps to Reproduce** (side-by-side with git):
```sh
cat > .git/hooks/commit-msg << 'EOF'
#!/bin/sh
> "$1"     # empty the message
exit 0
EOF
chmod +x .git/hooks/commit-msg

git commit -m "feat: change"        # → "Aborting commit due to empty commit message." exit 1, NO commit
stagehand --provider stub           # → "[abc1234] " (empty subject), exit 0, COMMIT CREATED
git log -1 --format=%B | xxd        # → 0a  (a lone newline = empty message)
```
The same outcome occurs when `prepare-commit-msg` (instead of `commit-msg`) empties the file.
**Impact**: A hook that empties the message — whether to reject it, or a buggy hook that truncates
it — silently creates a commit with no message. This is an invalid git state (`git commit` refuses
it unless `--allow-empty-message`), diverges from the git-parity contract, and the empty subject
also breaks downstream concerns (the anti-duplicate subject check, `git log` readability).
**Suggested Fix**: After `RunCommitHooks` returns (in `CommitStaged` at line 431, in
`runPipeline` after the hooks block, and in decompose `publishCommit`), check whether the
finalized message is empty (after trimming) and abort the way `git commit` does — return a
non-rescue error ("empty commit message — aborted", exit 1, mirroring `EditMessage`'s
`ErrEmptyMessage` path). HEAD and the index are already untouched at that point (no `update-ref`
ran), so the abort is clean.

## Minor Issues (Nice to Fix)

### Issue 5: Decompose/arbiter hook tests assert only `strings.Contains`, masking the newline corruption (Issue 2)

**Severity**: Minor (test-quality gap; not a runtime bug)
**PRD Reference**: §20.2 (invariant tests), §20.5 (e2e scenario harness)
**Expected Behavior**: The decompose hook tests (`internal/decompose/message_test.go`
`TestPublishCommit_PrepareCommitMsgAnnotates`, `chain_test.go`
`TestResolveArbiter_NullNewCommit_RunsHooks` / `_TipAmend_RunsHooks`) should verify the hook's
appended marker lands on a SEPARATE line (the git-parity outcome), not merely that the marker
substring is present.
**Actual Behavior**: All three use `strings.Contains(logMsg, "[HOOK-RAN]")`, which passes even when
the message is corrupted to `feat: add new[HOOK-RAN]` (the Issue 2 corruption). This is why Issue 2
shipped undetected through the decompose test suite.
**Suggested Fix**: Once Issue 2 is fixed, tighten these assertions to check line structure (e.g.
`strings.Contains("\n"+logMsg+"\n", "\n[HOOK-RAN]\n")`) so a regression of the trailing-newline
behavior fails the test. Not blocking on its own — it matters only as a guard for Issue 2's fix.

## Testing Summary

- **Total tests performed**: ~30 distinct scenarios across single-commit, dry-run, and decompose
  paths, plus 4 direct side-by-side comparisons against `git commit`.
- **Passing**: The core hook mechanism (ordering, `--no-verify` scope, FR-V3 freeze backstop, FR-V7
  rescue, post-commit best-effort, FR-V4 recursion prevention, dry-run commit-msg-only composition,
  hook timeout, foreign-hook annotation pickup, non-executable/absent-hook skipping, verbose
  logging, per-concept decompose scoping).
- **Failing / divergent from git parity**: 4 Major issues (Issues 1–4 above) + 1 minor test gap.
- **Areas with good coverage**: hook ordering and `--no-verify` semantics; FR-V3 subset enforcement;
  FR-V7 rescue mapping; recursion prevention; dry-run composition; the headline freeze-safety
  invariant (already covered by `internal/generate/hooks_freeze_test.go`).
- **Areas needing more attention**: git-parity of the message-file FORMAT (newline, argc) — these
  are the root cause of Issues 1 and 2 and were asserted too loosely; post-hook message VALIDATION
  (empty-message guard — Issue 4); and the git-config precedence layer for `no_verify` (Issue 3),
  which appears to have been specified to "mirror Push" but never received the `loadGitConfig`
  reader that `push` has.
