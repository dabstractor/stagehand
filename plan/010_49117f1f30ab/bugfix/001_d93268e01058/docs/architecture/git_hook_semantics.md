# Git Hook Semantics for the Commit Path — External Research

## 1. prepare-commit-msg argc for a plain commit

### Answer: argc=1 (the PRD's argc=2 is wrong)

**githooks(5) documentation:**
> It takes one to three parameters. The first is the name of the file that holds the commit log
> message. The second is the source of the commit message, and can be: `message`, `template`,
> `merge`, `squash`, `commit`. **If the mode is of neither of these, no second parameter is passed.**

For a **plain commit** (no `-m`, no `-F`, not merge, not squash, not `--amend`), git passes
**only the message-file path**. `$2` is **unset** and `$#` is **1**.

### Source-code evidence (builtin/commit.c)
The `prepare_to_commit()` function tracks the commit source via a `whence` enum. For `FROM_COMMIT`
with no `commit` pointer (plain `git commit`), nothing is pushed as argument 2 → argc=1.

### argc table

| Commit mode           | argc | `$2`      | `$3` |
|-----------------------|------|-----------|------|
| Plain `git commit`    | **1** | unset     | —    |
| `-m` / `-F`           | 2    | `message` | —    |
| `-t` / template       | 2    | `template`| —    |
| Merge commit          | 2    | `merge`   | —    |
| Squash commit         | 2    | `squash`  | —    |
| `-c`/`-C`/`--amend`   | 3    | `commit`  | SHA  |

### Practical impact
- `$#` differs (2 vs 1). Hooks that branch on `$#` break.
- `[ -z "$2" ]` / `[ -n "$2" ]` are indistinguishable in both cases (empty string vs unset).
- Most common hooks (husky, commitlint) use `[ -z "$2" ]` which works either way → narrow blast radius.

---

## 2. COMMIT_EDITMSG / message-file trailing newline

### Answer: git writes a trailing newline (`feat: change\n`)

When git writes the commit message to `.git/COMMIT_EDITMSG` (before prepare-commit-msg runs), the
file **always ends with `0x0a`** (a single trailing newline). This is true for every commit source.

In git's `prepare_to_commit()`, `strbuf_complete_line()` is called, which appends `\n` if the buffer
doesn't already end with one.

### Why this matters
Without trailing newline (stagehand's bug):
```
# File: "feat: change" (no \n)
# After `echo 'Signed-off-by: Dev <dev@example.com>' >> "$1"`:
feat: changeSigned-off-by: Dev <dev@example.com>   ← CORRUPTED
```

With trailing newline (git's behavior):
```
# File: "feat: change\n"
# After `echo 'Signed-off-by: Dev <dev@example.com>' >> "$1"`:
feat: change
Signed-off-by: Dev <dev@example.com>   ← CORRECT
```

### Impact
The `Signed-off-by` trailer pattern (Linux kernel, corporate contribution agreements, `git commit -s`
parity), branch-name hooks, and ticket-ref-injecting hooks ALL corrupt the subject when the generated
message is single-line (the most common shape).

---

## 3. Empty commit-message handling

### Answer: git aborts with exit 1 after both hooks

After both `prepare-commit-msg` and `commit-msg` have run, git reads the message file back, applies
cleanup (strip comment lines, strip trailing whitespace), and checks whether the result is empty. If
it is:

```
Aborting commit due to empty commit message.
```
Exit code **1**. No commit object is created; HEAD and index unchanged.

### Key points
- Applies to **both** hooks — the check runs once at the end, after both have had a chance to modify the file.
- The hook itself exited 0 — it just emptied the file. This is NOT a hook failure.
- `git commit --allow-empty-message` skips this check. Stagehand has no analog.

---

## 4. Git config key naming rules

### Answer: final segment allows only alphanumeric + `-`; underscores rejected

**git-config(5):**
> The variable names are case-insensitive, allow only alphanumeric characters and `-`, and must
> start with an alphabetic character.

The variable name (last segment after final `.`) **does not allow underscores**.

| Key                       | Valid? | Why                              |
|---------------------------|--------|----------------------------------|
| `stagehand.no_verify`     | **INVALID** | `_` rejected              |
| `stagehand.noVerify`      | **VALID**   | All alphanumeric          |
| `stagehand.no-verify`     | **VALID**   | Alphanumeric + `-`        |
| `stagehand.noverify`      | **VALID**   | All alphanumeric          |

Case-insensitive: `noVerify` = `noverify` = `NOVERIFY`. camelCase is for readability.

### Convention in this codebase
ALL multi-word git-config keys use camelCase: `autoStageAll`, `maxDiffBytes`, `tokenLimit`,
`diffContext`, `maxDuplicateRetries`, `subjectTargetChars`, `stripCodeFence`. The fix should use
`stagehand.noVerify` to match.

---

## Summary table

| # | Question                     | Correct answer                          | PRD/code claim                  | Divergence |
|---|------------------------------|-----------------------------------------|---------------------------------|------------|
| 1 | prepare-commit-msg argc      | **argc=1** (`$2` unset)                 | PRD/code: argc=2 (`$2=""`)      | **BUG**    |
| 2 | Message-file trailing newline| **Trailing `\n` always present**        | Code: `WriteString(finalMsg)`   | **BUG**    |
| 3 | Empty-message abort          | **Exit 1, "Aborting commit..."**        | Code: no check at all           | **GAP**    |
| 4 | Config key naming            | **`noVerify`; `_` rejected**            | PRD: `no_verify` (invalid)      | **BUG**    |

## Sources
- **githooks(5)** — prepare-commit-msg invocation contract: "no second parameter is passed" for plain commits.
- **git-config(5)** — "allow only alphanumeric characters and `-`" for variable names.
- **git source: builtin/commit.c** — `prepare_to_commit()` with `strbuf_complete_line()` and `whence` enum.
- **git source: config.c** — `iskeychar()` enforces alphanumeric + `-` only.
- Empirical verification against git 2.54.0.
