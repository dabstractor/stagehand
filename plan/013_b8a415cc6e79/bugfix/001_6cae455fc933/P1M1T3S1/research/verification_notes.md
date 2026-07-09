# P1.M1.T3.S1 — Verification Notes (docs snake_case → camelCase git-config keys)

This note records the direct verifications performed beyond the pre-existing
`architecture/research_gitconfig_keys.md`. The architecture research already
CONFIRMS PRD Issue 2; this file adds implementation-time guardrails.

## 1. Confirmed exact current text of the two target lines

`docs/configuration.md`, INI example block (lines 207–211):
```ini
[stagecoach]
    provider = pi
    model = glm-5.2
    timeout = 120s
    auto_stage_all = true          ← line 210 (BUG: snake_case)
```

Table row (line 218), shown with exact column layout:
```
| `stagecoach.auto_stage_all` | bool | `git config --get --bool stagecoach.auto_stage_all` | Auto-stage all when nothing staged |
```
Two textual occurrences of `auto_stage_all` → both become `autoStageAll`.
The Description column ("Auto-stage all when nothing staged") is human prose
(no underscore) and is UNCHANGED.

## 2. git.go source confirms camelCase is the only spelling read

- `internal/config/git.go:158` → `gitConfigBool(repoDir, "stagecoach.autoStageAll")`
- git.go:102–105 comment: "KEY NAMES ARE CAMELCASE (FINDING A): git config rejects
  underscores ('invalid key')."
- All 8 multi-word git keys are camelCase (autoStageAll, stripCodeFence, noVerify,
  maxDiffBytes, maxMdLines, tokenLimit, diffContext, maxDuplicateRetries,
  subjectTargetChars). Single-word keys (provider, model, output, format, locale,
  template, timeout, verbose, push) have no snake/camel distinction.

## 3. Test coverage already proves the behavior (no test change needed)

- `internal/config/git_test.go:304` `TestLoadGitConfig_CamelCaseKeysOnly`
  sets `stagecoach.autoStageAll` (camelCase, succeeds) and at lines 326–328
  asserts that attempting a snake_case key produces "invalid key" in stderr.
- `git_test.go:78` uses `setGitConfig(t, repo, "stagecoach.autoStageAll", "on")`.
- `git_test.go:177` sets `stagecoach.autoStageAll` to `"off"` (bool normalization).
These tests are green and pin the camelCase contract. This task changes NO Go code
and NO test — docs only.

## 4. grep verification design (must distinguish two layers)

**Rule:** `stagecoach.`-prefixed keys, or keys inside a `[stagecoach]` INI block,
are GIT-CONFIG keys (must be camelCase). Bare option names or keys under
`[defaults]` / `[generation]` are TOML config-file keys (legitimately snake_case —
DO NOT change).

Post-fix expected results:
- `grep -nE 'stagecoach\.[a-z]+_[a-z]' docs/configuration.md`
  → after the fix the ONLY remaining hit is **line 229**:
  `> ... and no `stagecoach.max_commits`.` — this is INTENTIONAL prose
  documenting a key that does NOT exist (`max_commits` is a config-file
  `[generation]` key, deliberately not exposed as a git key). It is NOT a
  discrepancy. Do NOT change it. Line 218 must be gone from this grep.
- The `[stagecoach]` INI block: after the fix the only multi-word key
  (`autoStageAll`) is camelCase; no underscored keys remain.

## 5. Lines that must NOT be touched (scope fence)

| Line | Content | Why leave it |
|------|---------|--------------|
| 87 | `# auto_stage_all = true` (TOML `[defaults]`) | TOML key — snake_case correct |
| 106–118 | `[generation]` block (`max_diff_bytes`, etc.) | TOML keys — snake_case correct |
| 133–151 | "Built-in defaults" table (option names) | Option names, not git keys |
| 166 | prose "via `git config stagecoach.autoStageAll`-style *bool behavior" | Uses camelCase already; not a snake_case error. (Prose-accuracy nit re: which key governs multi_turn_fallback is a SEPARATE, out-of-scope docs issue — not this task.) |
| 229 | prose "no `stagecoach.max_commits`" | Documents a non-existent key on purpose |

## 6. markdownlint baseline (validation-gate accuracy)

`.markdownlint.json` = `{default:true, MD013:false, MD033:false, MD060:false}`.
`npx --no-install markdownlint-cli2 docs/configuration.md` currently reports
**5 pre-existing errors, NONE at lines 210/218**: 156, 162, 164, 166
(MD032/MD028 blockquote-list spacing) and 280 (MD040 fenced-code-language).
Tool: markdownlint-cli2 v0.23.0 (markdownlint v0.41.0) is available via `npx`.
The 2-line fix changes identifier spelling inside a code fence and a table cell
→ introduces NO new lint errors. Validation gate = "error count does not increase
and no error references lines 210/218"; do NOT attempt to fix the unrelated 5.

## 7. Parallel-execution coordination (IMPORTANT)

Sibling **P1.M1.T2.S1** is being implemented in parallel and will INSERT two
rows into the SAME `docs/configuration.md` env-var table (~line 199–201,
between the `STAGECOACH_PUSH` row and the `## Git-config keys` header). This
shifts everything below by +2 lines:
- target line 210 → ~212
- target line 218 → ~220

⇒ Implementation MUST locate edits by CONTENT, not by hardcoded line number.
`grep -n 'auto_stage_all' docs/configuration.md` returns EXACTLY the two lines
to fix (regardless of upstream drift). Both edits are content-anchored.
