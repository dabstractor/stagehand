# Stale-reference sweep findings — P1.M7.T1.S2

Date: 2026-07-03. Grep-verified across `README.md`, `docs/`, `*.go` (help text + templates), `*.md`.
Method: `grep -rn '<pattern>' --include='*.go' --include='*.md' --include='*.toml' . | grep -v '/plan/' | grep -v '.git/'`.

## The ONE confirmed stale string (MUST FIX)

`internal/cmd/config.go`, the `exampleConfigTemplate` string constant (the inert commented config
written by `config init --template`; this IS user-facing — it lands on a user's disk and is the
Mode-A config documentation surface). The `config_version` header block (lines ~524–529):

```
# config_version — schema version (PRD §9.17 FR-B4). Top-level metadata, NOT a [defaults] key and
# NOT a precedence layer (§16.1): it never overrides another field; it only tells stagecoach which
# schema the file was written for. This binary supports config_version = 2.   <-- STALE: 2 -> 3
# ---------------------------------------------------------------------------
# config_version = 2   <-- STALE example line: 2 -> 3
```

- `internal/config/config.go:11` — `const CurrentConfigVersion = 3` (authoritative; the binary is v3).
- `internal/cmd/config.go:526` header prose: "This binary supports config_version = 2." → must read `3`.
- `internal/cmd/config.go:528` commented example line: `# config_version = 2` → must read `# config_version = 3`.
- Both are in the SAME string constant → ONE `edit` call with two replacements (or one block replace).

### Test-safety (verified — the fix will NOT break tests)
- `internal/cmd/config_test.go:438` asserts the `--template` output equals `exampleConfigTemplate` by
  FULL-STRING equality (`if got != exampleConfigTemplate`). Editing the constant keeps both sides
  identical → test stays green.
- No test pins the literal substring `"config_version = 2"` against the template (grep of config_test.go
  + file_test.go shows only `config_version = 3` assertions, all on the POPULATED bootstrap path, not
  the `--template` inert path). SAFE.
- The populated-bootstrap tests (`config init` without `--template`) already assert `config_version = 3`
  and are unrelated to this edit.

## Patterns verified CLEAN (record these clean results in the commit message)

### Pattern 1 — pre-v3 terminology (`--planner-agent` flag, `[agent.*]` config blocks as CURRENT)
- `--planner-agent`: ZERO hits as a flag. `internal/decompose/planner.go:31` says "sentinel for
  planner-agent failures" — that describes the planner ROLE (which exists), NOT the abandoned flag.
  CLEAN.
- `[agent.*]` config blocks: present ONLY in (a) the v2→v3 migration code (`internal/cmd/config.go`
  `rewriteV2ToV3`, lines ~125–345) and its upgrade help text — CORRECT, describes legacy handling;
  (b) test inputs (`config_test.go:1250 "[agent.pi]"`) that verify migration — CORRECT.
  NOT presented as a current v3 config shape anywhere user-facing. CLEAN.
- `default_provider`: in `docs/cli.md:185` + `docs/configuration.md:54` ONLY as the legacy field the
  `config upgrade` command folds into the model prefix — CORRECT migration docs. CLEAN.

### Pattern 2 — "config_version = 2" (after the fix)
- `internal/cmd/config.go:122` — code comment explaining the version-regex anchoring uses `# config_version = 2`
  as the example of a COMMENTED line the regex ignores. This is CORRECT (it's teaching the regex behavior,
  not asserting the binary is v2). Do NOT touch.
- Test inputs (`default_action_test.go:1204`, `config_test.go:1167+`, `file_test.go:548/669`) contain
  `config_version = 2` as v2 FIXTURES fed into the migration tests — CORRECT, these represent legacy
  files being upgraded. Do NOT touch.
- docs/ + README.md: ZERO `config_version = 2` hits. CLEAN.

### Pattern 3 — superseded non-goal phrasings ("no hook installer", "--edit is deferred/v1.1")
- "hook installer": ZERO current-claim hits in docs/README. The only occurrences are in `PRD.md`
  (read-only) where it is correctly marked superseded with strikethrough
  (`~~Hook installer~~ — superseded in v2.1`). CLEAN.
- "--edit is deferred / v1.1": ZERO current-claim hits in docs/README. The only occurrences are
  `PRD.md` (read-only, correctly `~~Interactive commit-message editing~~ — superseded`) and
  `internal/cmd/hookexec.go:57` — a CORRECT runtime error: "--edit is not valid with hook exec
  (git already opens the editor)" (FR-E4). CLEAN.

### Pattern 4 — COMPETITOR-ANALYSIS.md presented as an existing repo file
- `COMPETITOR-ANALYSIS.md` does NOT exist (confirmed: `ls` → "No such file or directory").
- Referenced ONLY in `PRD.md` (lines 13, 499, 2189) and `FUTURE_SPEC.md:8` — BOTH read-only,
  human-owned, and frame it as the historical planning "evidence base"/"source-level review", not as
  a file a repo visitor should open. CLEAN for our scope (docs/ + README + user-facing strings = zero hits).
- NOTE: per the FORBIDDEN-OPERATIONS + S1 scope fence, PRD.md and FUTURE_SPEC.md are READ-ONLY; do not
  modify them even to soften the COMPETITOR-ANALYSIS.md mentions. The task scope is docs/README/strings.

## Out of scope (do NOT touch — belongs to other owners/tasks)
- `README.md` + `docs/README.md` — P1.M7.T1.S1 owns these (running in parallel).
- `docs/{cli,configuration,how-it-works,providers}.md` Mode-A docs — owned by M1–M6 / P1.M6.T2.S1.
- `PRD.md`, `FUTURE_SPEC.md`, `tasks.json`, `prd_snapshot.md` — read-only / orchestrator-owned.
- Model-name currency in the template's `[role.*]` examples (`gemini-2.5-pro` etc.) — illustrative in
  the inert `--template` reference; live currency is the manifests' job (M2 / FR-D5), not this sweep.

## Optional (non-blocking) refinement — flag for implementer discretion, NOT required
- The same `exampleConfigTemplate` block says "it NEVER auto-migrates your file (no behavior change)".
  Per FR-B7 the binary DOES auto-migrate in memory (with a deprecation notice) — it just never rewrites
  the file on disk. "NEVER auto-migrates your FILE" is defensible (file = on-disk), but the implementer
  MAY tighten the wording to "never rewrites your file on disk (older files are migrated in memory)"
  for precision. This is NOT the confirmed stale string; the version-number fix is the required change.
