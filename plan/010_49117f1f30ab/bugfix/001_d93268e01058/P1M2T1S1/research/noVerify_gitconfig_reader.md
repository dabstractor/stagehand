# Research: stagehand.noVerify git-config reader + key-name fix (P1.M2.T1.S1, bugfix Issue 3)

Verified against the live codebase. Source of truth for the missing-layer + invalid-key fix.

## The two defects (Issue 3, Major)

1. **The git-config reader is MISSING.** `loadGitConfig()` (`internal/config/git.go:110`) reads
   `stagehand.push` (line 174) but NEVER queries any `stagehand.noVerify`/`no_verify` key
   (`grep -cn "no_verify\|noVerify\|NoVerify" internal/config/git.go` = 0). The other 4 layers are correct:
   env (load.go:315, DIRECT set), flag (load.go:447, DIRECT set), TOML file (overlay file.go:351-352,
   `if src.NoVerify { dst.NoVerify = true }`), default (config.go Defaults NoVerify=false). So no_verify
   resolves through only 4 layers, contradicting the doc comment (config.go:130) and PRD §9.25 FR-V5.
2. **The documented key is INVALID for git.** `git config stagehand.no_verify true` fails:
   `error: invalid key: stagehand.no_verify` — git rejects underscores in the final config segment. Every
   other multi-word stagehand git-config key uses camelCase: `stagehand.autoStageAll`, `stagehand.maxDiffBytes`,
   `stagehand.stripCodeFence` (git.go:158/168). `no_verify` is a pattern break.

## The fix — camelCase `stagehand.noVerify` (git-valid, matches convention)

Add a reader mirroring the `push` reader (git.go:174-179) EXACTLY, placed right after it (~line 179),
using `gitConfigBool(repoDir, "stagehand.noVerify")`:

```go
// §9.25 FR-V5 — noVerify via git config (camelCase key: git rejects underscores in the final segment,
// matching the autoStageAll/maxDiffBytes/stripCodeFence convention).
if v, found, err := gitConfigBool(repoDir, "stagehand.noVerify"); err != nil {
    return nil, err
} else if found {
    c.NoVerify = v
}
```

`gitConfigBool(repo, key) (value bool, found bool, err error)` (git.go:66) runs `git -C <repo> config
--bool --get <key>`. camelCase is git-valid (no underscore in the final segment). `stagehand.noVerify`
is settable: `git config stagehand.noVerify true` works.

## The 4 git-config-key references to fix (`stagehand.no_verify` → `stagehand.noVerify`)

The contract listed 3; the grep found a 4th (root.go flag help text — same invalid key, user-facing).
ALL of these are the GIT-CONFIG key (not the TOML key) and must be corrected:

1. `docs/cli.md:44` — the `--no-verify` row's git-config column cell: `stagehand.no_verify` → `stagehand.noVerify`.
2. `docs/configuration.md:155` — the precedence chain in the `**no_verify**` prose: `stagehand.no_verify` →
   `stagehand.noVerify`. SURGICAL: the SAME line also contains `[generation].no_verify` (the TOML file key,
   correct) — change ONLY the `stagehand.no_verify` token, leave `[generation].no_verify` intact.
3. `internal/config/config.go:130` — the NoVerify doc comment's precedence chain: `stagehand.no_verify` →
   `stagehand.noVerify`. Same surgical rule: leave `[generation].no_verify` (TOML) intact.
4. `internal/cmd/root.go:219` — the `--no-verify` flag help string: `git stagehand.no_verify` →
   `git stagehand.noVerify`. (Contract missed this; it is user-facing `--help` text printing the invalid key.)

## DO NOT CHANGE — these are correct (TOML key, separate namespace)

- The TOML struct tag `config.go:134` `toml:"no_verify"` — TOML allows underscores; the FILE key stays
  `no_verify`. (The git-config key and the TOML key are different namespaces.)
- `docs/configuration.md:117` (`# no_verify = false` comment), `:149` (defaults-table `no_verify` row),
  `:155`'s `[generation].no_verify` — these are the TOML file key, correct.
- `internal/config/file.go` overlay() (:351-352) and materialize() (:291) — already handle NoVerify correctly.
- `internal/config/load.go` loadEnv (:315, DIRECT set) and loadFlags (:447, DIRECT set) — already correct
  (the escape hatch that CAN set false, unlike the file/git only-true layers).

## Overlay semantics (load-bearing for the test)

overlay() uses ONLY-TRUE-PROPAGATES for NoVerify (`if src.NoVerify { dst.NoVerify = true }`, same as Push).
So: Defaults() NoVerify=false → git config noVerify=true ⇒ loadGitConfig returns partial{NoVerify:true} ⇒
overlay sets cfg.NoVerify=true. git config noVerify=false ⇒ no-op (can't set false via the git layer —
same v1 limitation as Push/AutoStageAll; the flag/env DIRECT-set layers are the escape hatch).

## TDD test plan (write failing tests FIRST, then the reader)

**Test 1 — `internal/config/git_test.go::TestLoadGitConfig_ReadsValues` (line 71):** after the existing
`setGitConfig(t, repo, "stagehand.push", "true")` (line 87), add `setGitConfig(t, repo, "stagehand.noVerify",
"true")`; after the Push assertion (line 128-130), add `if !cfg.NoVerify { t.Errorf("NoVerify=false want
true (stagehand.noVerify set)") }`. FAILS today (reader absent).

**Test 2 — `internal/config/load_test.go::TestLoad_NoVerifyPrecedence` (NEW, mirror `TestLoad_PushPrecedence`
load_test.go:1389-1429):** `_, repo, _ := loadEnvSetup(t); chdir(t, repo)`; (a) `setGitConfig(t, repo,
"stagehand.noVerify", "true")` → `Load(...)` → assert `cfg.NoVerify == true` (proves the new reader works);
(b) `t.Setenv("STAGEHAND_NO_VERIFY", "false")` → `Load(...)` → assert `cfg.NoVerify == false` (env DIRECT >
git — the escape hatch). Reuses `loadEnvSetup`/`chdir`/`setGitConfig`/`Load` exactly as PushPrecedence does.

## Scope boundary (no conflict)

- **P1.M1.T2.S1 (parallel)** is the trailing-newline fix in `internal/hooks/runner.go` (WriteString). My task
  touches `internal/config/git.go` (reader) + `internal/config/git_test.go` + `internal/config/load_test.go`
  (tests) + `docs/cli.md` + `docs/configuration.md` + `internal/config/config.go` (comment) + `internal/cmd/root.go`
  (help text). DIFFERENT files → ZERO overlap with runner.go.
- This task does NOT touch overlay/materialize/loadEnv/loadFlags (correct), the TOML tag (correct), or the
  hooks runner (M1's scope). It adds ONE reader + fixes the key name in 4 docs/comments + 2 tests.

## DOCS: Mode A — the 4 key-name fixes ride WITH this subtask (they reference the specific key being fixed).
