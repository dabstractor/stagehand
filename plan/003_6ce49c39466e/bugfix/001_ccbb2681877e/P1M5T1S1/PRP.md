---
name: "P1.M5.T1.S1 (bugfix Issue 5) — Defense-in-depth agent→provider textual remap in loadTOML + migration test"
description: |

  Issue 5 (minor, defense-in-depth). A v2 config file using the abandoned intermediate `agent` /
  `[agent.*]` terminology SILENTLY LOSES its provider block on in-memory load: `fileConfig` (file.go) has
  NO `Agent` field and go-toml/v2 drops unknown `[agent.*]` tables, so `cfg.Provider == ""` and
  `cfg.Providers["pi"]` is absent — until the user runs the on-disk `config upgrade`. This task adds a
  TEXTUAL remap to `loadTOML` so such files load correctly in memory WITHOUT requiring `config upgrade`.

  CONTRACT (item_description §1–§5, verbatim):
    1. RESEARCH: "loadTOML reads data then toml.Unmarshal(data, &fc); fileConfig has NO Agent field;
       go-toml silently drops [agent.*]. migrateV2ToV3 documents this as a no-op. PRD fix: have loadTOML
       detect and remap [agent.*] textually before the typed decode."
    2. INPUT: "the raw TOML data []byte in loadTOML, before the toml.Unmarshal call. Add `strings` (and
       regexp) if not already imported."
    3. LOGIC: "Add helper remapAgentTerminology(data []byte) []byte: (a) [agent. → [provider.; (b)
       line-oriented — lines matching ^\s*agent\s*= have the key `agent` rewritten to `provider` (key name
       only, not occurrences inside values/comments). Call data = remapAgentTerminology(data) before
       unmarshal. Must be idempotent. The text approach is what the PRD requests."
    4. OUTPUT: "a v2 config using abandoned agent terminology loads with the provider block preserved in
       memory (cfg.Provider / cfg.Providers populated) without requiring config upgrade."
    5. DOCS: "Update the migrateV2ToV3 doc comment (migrate.go:35-45) to reflect the agent→provider remap
       is now handled textually in loadTOML (no longer a pure no-op). Rides WITH the work — no separate
       docs subtask."

  DELIVERABLES (3 files MODIFIED; go.mod UNCHANGED — regexp + strings are stdlib):
    1. MODIFY internal/config/file.go       — add `regexp` + `strings` imports; add package-level
       `agentKeyRe` + `remapAgentTerminology` helper; call `data = remapAgentTerminology(data)` in loadTOML
       before `toml.Unmarshal(data, &fc)`.
    2. MODIFY internal/config/migrate.go    — update the AGENT TERMINOLOGY doc-comment paragraph AND the
       `(0)` implementation comment to state the remap is now handled textually upstream in loadTOML.
    3. MODIFY internal/config/file_test.go  — add `TestLoadTOML_AgentTerminologyRemapped` (the integration
       test the contract specifies) + `TestRemapAgentTerminology` (a focused table-driven helper unit test
       for idempotency + key-name-only precision).

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/config/load.go — Load orchestrator; calls migrateV2ToV3 at :157 (post-decode). READ ONLY.
      The remap is in loadTOML (pre-decode); migrateV2ToV3's default_provider fold is UNCHANGED.
    - internal/cmd/config.go — the ON-DISK `config upgrade` rewrite (agentHeaderRe/rewriteV2ToV3).
      READ ONLY. This task's in-memory remap is the in-memory twin; do NOT merge/reuse the on-disk code.
    - internal/config/bootstrap.go, bootstrap_test.go — parallel P1.M4.T1.S1 (header env vars). ZERO
      overlap with file.go/migrate.go/file_test.go.
    - migrateV2ToV3's LOGIC is UNCHANGED — only its doc comment.

  SUCCESS: a v2 file with `[defaults]\nagent = "pi"\n[agent.pi]\ndefault_model = "glm-5.2"\n` loads with
  `cfg.Provider == "pi"` and `cfg.Providers["pi"]["default_model"] == "glm-5.2"` (today both are lost).
  The remap is idempotent (a normal `[provider.pi]` file is byte-unaffected). `go build ./... &&
  go test ./...` green; gofmt/vet clean; go.mod/go.sum byte-unchanged; exactly 3 files differ.

---

## Goal

**Feature Goal**: Close Issue 5 (defense-in-depth) — make `loadTOML` textually remap the abandoned
intermediate `agent`/`[agent.*]` terminology to `provider`/`[provider.*]` BEFORE the typed decode, so a
v2 config written with that terminology loads with its provider block preserved in memory. No user should
have to run `config upgrade` just to load such a file.

**Deliverable** (3 modified files; go.mod unchanged):
1. `internal/config/file.go` — add `regexp` + `strings` imports; add `agentKeyRe` + `remapAgentTerminology`;
   call it in `loadTOML` before `toml.Unmarshal`.
2. `internal/config/migrate.go` — update the two agent-terminology comments (doc-comment paragraph + `(0)`
   block) to reflect the upstream textual remap.
3. `internal/config/file_test.go` — `TestLoadTOML_AgentTerminologyRemapped` (integration) +
   `TestRemapAgentTerminology` (table-driven helper unit test).

**Success Definition**:
- A temp file with `config_version = 2`, `[defaults] agent = "pi"`, `[agent.pi] default_model = "glm-5.2"`
  loads via `loadTOML` with `cfg.Provider == "pi"` AND `cfg.Providers["pi"]["default_model"] == "glm-5.2"`.
- The remap is IDEMPOTENT: a normal `[provider.pi]` / `provider =` file is byte-identical after remap
  (asserted by the helper unit test's idempotency cases).
- The key remap touches ONLY the key token (`agent =` → `provider =`); `agent` inside comments/values and
  `[agent.pi]` headers are handled separately and never corrupted (asserted by the precision cases).
- `go build ./... && go test ./...` green; `gofmt -l internal/config/` empty; `go vet ./internal/config/`
  clean; `go.mod`/`go.sum` byte-unchanged; exactly 3 files modified.

## User Persona

**Target User**: a Stagehand user (PRD §7 personas) carrying a config file written during the brief
intermediate "agent" terminology window (or hand-edited from an old example). They run `stagehand` and it
silently uses no/empty provider because the `[agent.pi]` block was dropped on load — confusing, since the
file "looks" configured. Today they must discover and run `config upgrade` to fix it.

**Use Case**: user points Stagehand at their existing config → it loads with the provider block preserved
(provider/model/manifest resolve correctly) → generation proceeds. No `config upgrade` required for the
in-memory load.

**User Journey**: user runs `stagehand` with an `[agent.pi]` config → loadTOML textually remaps to
`[provider.pi]` before decode → `cfg.Provider == "pi"`, `cfg.Providers["pi"]` populated → the registry
resolves the pi manifest → generation works.

**Pain Points Addressed**: silent loss of the provider block on load (the file "looks" configured but
Stagehand runs unconfigured). Defense-in-depth: even though the `agent` terminology likely never shipped
in a release, a file using it now loads correctly instead of failing opaquely.

## Why

- **It IS Issue 5.** The bug list (§h3.4) names this exact gap (FR-B7): files using the abandoned
  `agent`/`[agent.*]` terminology should map back to `provider`/`[provider.*]` "first". Today only the
  on-disk `config upgrade` does it; in-memory load is a documented no-op that loses the block.
- **Completes FR-B7 in memory.** PRD §9.17 FR-B7 says the agent→provider remap happens "first" (before the
  default_provider fold). The on-disk path does it; this makes the in-memory load path do it too, so the
  guarantee holds whether or not the user has run `config upgrade`.
- **Mirrors an established house pattern.** `internal/cmd/config.go` ALREADY does this exact textual
  `[agent.*]`→`[provider.*]` remap for the on-disk path (`agentHeaderRe`/`rewriteV2ToV3`). The in-memory
  remap is the same FR-B7 step in a different domain — not a new pattern.
- **Cheap, isolated, safe.** A pure helper + one call site + a doc-comment sync + 2 tests. No schema
  change, no precedence change, no behavior change for provider-terminology files (idempotent). go.mod
  untouched (stdlib only).

## What

A textual pre-decode remap in `loadTOML`, its doc sync in `migrate.go`, and two tests:

1. **`remapAgentTerminology(data []byte) []byte`** (file.go) — pure, idempotent. Two transforms on the raw
   TOML text:
   - (a) `strings.ReplaceAll(s, "[agent.", "[provider.")` — table headers (`[agent.pi]` → `[provider.pi]`).
   - (b) `agentKeyRe.ReplaceAllString(s, "${1}provider${2}")` where
     `agentKeyRe = regexp.MustCompile(`(?m)^(\s*)agent(\s*=)`)` — the bare key, line-oriented, key-name
     only (preserves indent + `=`; ignores `agent` in comments/values/headers).
2. **One call site**: `data = remapAgentTerminology(data)` in `loadTOML`, between the `os.ReadFile` error
   block and `var fc fileConfig` (immediately before `toml.Unmarshal(data, &fc)`).
3. **Doc sync** (migrate.go): the AGENT TERMINOLOGY doc-comment paragraph + the `(0)` implementation
   comment — both currently claim the remap is a no-op; update to state it's handled textually upstream in
   `loadTOML`/`remapAgentTerminology`.
4. **Two tests** (file_test.go): `TestLoadTOML_AgentTerminologyRemapped` (integration, via `writeTempTOML`+
   `loadTOML`) and `TestRemapAgentTerminology` (table-driven helper unit test).

### Success Criteria

- [ ] `internal/config/file.go` imports `regexp` and `strings` (neither is imported today).
- [ ] `remapAgentTerminology(data []byte) []byte` exists in file.go; it is pure + idempotent.
- [ ] `loadTOML` calls `data = remapAgentTerminology(data)` before `toml.Unmarshal(data, &fc)`.
- [ ] `migrate.go`: both agent-terminology comments updated (no longer claim "no-op"/"no agent data reaches cfg").
- [ ] `TestLoadTOML_AgentTerminologyRemapped` asserts `cfg.Provider == "pi"` and
      `cfg.Providers["pi"]["default_model"] == "glm-5.2"` for the v2 agent-terminology fixture (FAILS
      without the fix; PASSES with it).
- [ ] `TestRemapAgentTerminology` covers: header remap, key remap (spaced/tight/indented), comment/value
      untouched, already-provider idempotency, header+key together, double-run idempotency.
- [ ] `go build ./... && go test ./...` GREEN; `gofmt -l internal/config/` empty; `go vet` clean;
      go.mod/go.sum byte-unchanged; exactly 3 files modified (file.go, migrate.go, file_test.go).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact loadTOML insertion
point (quoted), the exact two transforms + their regexp (given), the decode trace proving cfg is populated
(quoted), the two unique migrate.go comment strings to edit (quoted), the test fixture body (given), and
the existing `writeTempTOML`/`TestLoadTOML*` patterns to mirror. No registry/git/generate knowledge needed.

### Documentation & References

```yaml
# MUST READ — THE decisive doc
- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/P1M5T1S1/research/findings.md
  why: §1 the decode seam + insertion point + the cfg-population trace; §2 the proven on-disk precedent
       (cmd/config.go agentHeaderRe); §3 the two transforms with exact regexp + idempotency/safety proof;
       §4 the two migrate.go comment strings to update; §5 the test plan + writeTempTOML reuse; §6 validation;
       §7 confidence/risks (import addition, parallel coordination).
  critical: §1 (insertion point + trace), §3 (the (?m)^(\s*)agent(\s*=) regexp — key-name only), §4 (the
            two unique migrate.go strings).

# MUST READ — the file to EDIT (the decode seam + insertion point)
- file: internal/config/file.go   (EDIT: +imports, +helper, +1 call in loadTOML)
  section: loadTOML (file.go:124-150). The insertion point is BETWEEN the `os.ReadFile` error-handling
       block (`return nil, fmt.Errorf("read config %s: %w", path, err)`) and `var fc fileConfig`, i.e.
       immediately before `if err := toml.Unmarshal(data, &fc); err != nil {`.
  why: this is THE call site. The remap MUST run on `data` BEFORE `toml.Unmarshal` so the agent-keyed
       tables/keys survive the typed decode (fileConfig has no Agent field → otherwise dropped).
  pattern: loadTOML is the single decode entry point; materialize() then copies Defaults.Provider +
       the whole Providers map into *Config (the trace in research §1 proving cfg.Provider/cfg.Providers
       populate after remap).
  gotcha: file.go imports today are `fmt, io, os, path/filepath, time` + go-toml/v2 — `strings` AND
       `regexp` are BOTH absent and MUST be added. Forgetting one → compile error.

# MUST READ — the file to EDIT (doc-comment sync; LOGIC UNCHANGED)
- file: internal/config/migrate.go   (EDIT: 2 comments only — the doc-comment AGENT TERMINOLOGY paragraph
       + the `(0)` implementation comment. DO NOT touch migrateV2ToV3's logic.)
  section: the `migrateV2ToV3` doc comment's "AGENT TERMINOLOGY" paragraph and the `(0)` block at the top
       of the function body (both quoted verbatim in research §4). They currently say the agent remap is a
       "NO-OP in memory" / "no agent data reaches cfg" — which becomes FALSE after this fix.
  why: the contract §5 mandates updating the doc to reflect the upstream textual remap. Leaving the old
       "no-op" text creates a code/docs contradiction (a maintenance trap).
  pattern: keep the comments concise and factual; point at loadTOML/remapAgentTerminology as the new owner
       of the agent→provider step; note migrateV2ToV3 itself needs no agent logic (upstream already ran).
  gotcha: migrateV2ToV3's DEFAULT_PROVIDER FOLD LOGIC IS UNCHANGED. Only the two agent-terminology
       comments change. Load calls migrateV2ToV3 at load.go:157 (post-decode) — complementary order.

# MUST READ — the proven on-disk precedent (mirror its regexp idiom; DO NOT edit/reuse)
- file: internal/cmd/config.go   (READ ONLY)
  section: `agentHeaderRe = regexp.MustCompile("^\\[agent\\.(.+?)\\]\\s*$")` (config.go:125-127) +
       `rewriteV2ToV3` pass 1 (config.go:~259-263) which renames `[agent.<name>]` → `[provider.<name>]`.
       Also `kvStringRe = regexp.MustCompile("^\\s*([A-Za-z_]...)\\s*=\\s*\"([^\"]*)\"")` (config.go:138)
       — the house line-oriented key-match idiom.
  why: PROVES textual agent→provider remap is the established FR-B7 house pattern. The in-memory remap is
       the in-memory twin (raw bytes-before-decode). Mirror the regexp idiom; keep them INDEPENDENT (the
       on-disk rewrite also folds default_provider, which loadTOML must NOT — that's migrateV2ToV3's job).
  gotcha: do NOT import or call internal/cmd from internal/config (import cycle / layering). Re-implement
       the small regexp locally in file.go (it's 2 lines).

# MUST READ — the test file to extend (reuse writeTempTOML + the TestLoadTOML* pattern)
- file: internal/config/file_test.go   (EDIT: +2 tests; READ the patterns)
  section: `writeTempTOML(t, body) string` (top of file) + `TestLoadTOMLValid` / `TestLoadTOML_V2Fields`
       (the canonical loadTOML assertion style). file_test.go is `package config` (internal) → can call
       `loadTOML` directly.
  why: mirror `TestLoadTOMLValid` for the integration test (writeTempTOML → loadTOML → assert cfg fields).
       The helper unit test is table-driven (mirrors migrate_test.go's `TestMigrateV2ToV3` table style).
  pattern: assertions use `t.Errorf("X=%q want Y", got)`; `writeTempTOML` writes to t.TempDir() (auto-cleaned).
  gotcha: file_test.go already imports bytes, fmt, os, path/filepath, strings, testing, time — the new
       tests need only `strings` + `testing` (both present). NO new test-file imports.

# READ ONLY — proves the call order is complementary (no conflict)
- file: internal/config/load.go   (line 157: `migrateV2ToV3(&cfg)`)
  why: confirms migrateV2ToV3 runs AFTER loadTOML decodes. The remap (pre-decode, in loadTOML) and the
       default_provider fold (post-decode, in migrateV2ToV3) run in the correct order and do not overlap.
  gotcha: DO NOT edit load.go.

- url: (PRD internal) prd_snapshot.md §h3.4 (Issue 5) + PRD §9.17 FR-B7 ("first map agent/[agent.*] →
       provider/[provider.*]"). AUTHORITATIVE statement of the gap + the FR it satisfies.
  why: Issue 5 is the bug; FR-B7 is the requirement. The suggested fix ("have loadTOML/migrateV2ToV3 detect
       and remap [agent.*] textually before the typed decode") is EXACTLY what this task implements.
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  file.go            # EDIT: +regexp/+strings imports, +agentKeyRe, +remapAgentTerminology, +1 call in loadTOML.
  file_test.go       # EDIT: +TestLoadTOML_AgentTerminologyRemapped, +TestRemapAgentTerminology. READ writeTempTOML/TestLoadTOML*.
  migrate.go         # EDIT: 2 agent-terminology comments only (doc paragraph + (0) block). LOGIC UNCHANGED.
  load.go            # READ ONLY: migrateV2ToV3 call at :157 (post-decode; complementary order). DO NOT EDIT.
internal/cmd/config.go  # READ ONLY: on-disk agentHeaderRe/rewriteV2ToV3 precedent (mirror the regexp; don't reuse).
go.mod / go.sum      # UNCHANGED (regexp + strings are stdlib).
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. 3 MODIFIED files only:
internal/config/file.go         # +2 imports (regexp, strings), +agentKeyRe var, +remapAgentTerminology func, +1 call in loadTOML.
internal/config/migrate.go      # 2 comment edits (AGENT TERMINOLOGY doc paragraph + (0) block). NO logic change.
internal/config/file_test.go    # +TestLoadTOML_AgentTerminologyRemapped, +TestRemapAgentTerminology.
# go.mod/go.sum UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (ADD BOTH imports): file.go today imports fmt, io, os, path/filepath, time + go-toml/v2.
//   `strings` AND `regexp` are BOTH ABSENT. The helper needs strings.ReplaceAll AND regexp.MustCompile.
//   Add BOTH. Forgetting one → "undefined: strings/regexp" compile error.

// CRITICAL (key-name ONLY, not a blind replace): do NOT `strings.ReplaceAll(s, "agent", "provider")` —
//   that would corrupt `model = "agent"`, comments, and `[agent.pi]` mid-value. Use the line-oriented
//   regexp `(?m)^(\s*)agent(\s*=)` → `${1}provider${2}` which matches ONLY a bare `agent =` key at line
//   start (after optional indent). It does NOT match `# agent =`, `model = "agent"`, `my_agent =`, or
//   `[agent.pi]` (the header is handled by transform (a)).

// CRITICAL (IDEMPOTENT): a normal provider-terminology file must be byte-unaffected. Transform (a)
//   `[agent.`→`[provider.` can't match `[provider.`. Transform (b)'s regexp can't match `provider =`.
//   So remap(remap(x)) == remap(x). The helper unit test pins this (already-provider + double-run cases).

// GOTCHA (transform (a) is a literal prefix replace, NOT the on-disk regexp): the contract specifies
//   `[agent.` → `[provider.` (literal). The on-disk cmd/config.go uses `^\[agent\.(.+?)\]\s*$` (anchored,
//   whole-line) — that's the on-disk twin; for the in-memory pre-decode remap the simpler literal
//   `strings.ReplaceAll("[agent.", "[provider.")` is what the contract requests and is safe in practice
//   (a `[agent.` substring only occurs in table headers; a comment `# [agent.pi]` → `# [provider.pi]`
//   stays an inert comment).

// GOTCHA (no schema key is named `agent` in v3): defaults keys = provider/model/reasoning/timeout/
//   auto_stage_all/verbose; role keys = provider/model/reasoning; provider.* keys = manifest fields. So a
//   bare `agent =` key is ALWAYS the abandoned terminology → remapping it is always correct, never
//   destructive. (Safety backstop even if an unusual file has `agent =` somewhere unexpected.)

// GOTCHA (call order is complementary): loadTOML (remap, pre-decode) runs BEFORE migrateV2ToV3
//   (default_provider fold, post-decode, load.go:157). Do NOT move the fold into loadTOML and do NOT add
//   agent logic to migrateV2ToV3 — the remap belongs in loadTOML so the data survives decode; the fold
//   belongs in migrateV2ToV3 where it already is. The two are complementary, not duplicative.

// GOTCHA (parallel P1.M4.T1.S1 touches DIFFERENT files): that task edits bootstrap.go +
//   bootstrap_test.go ONLY. This task edits file.go + migrate.go + file_test.go. ZERO overlap. Safe to
//   run in parallel; the full `go test ./...` gate is safe after both merge.

// GOTCHA (do NOT edit internal/cmd/config.go): the on-disk rewrite already handles [agent.*] for
//   `config upgrade`. This task is the IN-MEMORY load path. internal/config must not import internal/cmd
//   (layering/cycle). Re-implement the 2-line regexp locally in file.go.
```

## Implementation Blueprint

### Data models and structure

```go
// NO new data models. fileConfig already has the fields the remap targets (Defaults.Provider, Provider map).
// The only new declarations are a package-level regexp var + a pure helper function:
var agentKeyRe = regexp.MustCompile(`(?m)^(\s*)agent(\s*=)`)

func remapAgentTerminology(data []byte) []byte { ... }
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/file.go — add imports, the helper, and the call site
  - IMPORTS: add `regexp` and `strings` to the import block (both absent today). Keep gofmt grouping
      (stdlib together; go-toml/v2 stays in its own group).
  - ADD a package-level regexp var near the top (after the decode-struct block, before loadTOML, or just
      above the helper — co-locate with the helper for readability):
        // agentKeyRe matches a bare `agent =` KEY at line start (after optional indent) — the abandoned
        // intermediate terminology's defaults key. Line-oriented (multiline); captures indent + the ws+'='
        // so the rewrite preserves them. Does NOT match comments, values, or [agent.*] headers.
        var agentKeyRe = regexp.MustCompile(`(?m)^(\s*)agent(\s*=)`)
  - ADD the helper (pure, idempotent):
        // remapAgentTerminology defense-in-depth-remaps the abandoned intermediate agent/[agent.*]
        // terminology to provider/[provider.*] in raw TOML text BEFORE the typed decode (PRD §9.17 FR-B7
        // "first"). Two transforms: (a) [agent. → [provider. table headers; (b) a bare `agent =` key →
        // `provider =` (line-oriented, key name only). Pure + idempotent (a no-op on provider-terminology
        // files). fileConfig has no Agent field, so without this go-toml silently drops [agent.*] tables.
        func remapAgentTerminology(data []byte) []byte {
            s := string(data)
            s = strings.ReplaceAll(s, "[agent.", "[provider.")        // (a) table headers
            s = agentKeyRe.ReplaceAllString(s, "${1}provider${2}")    // (b) bare key, key-name only
            return []byte(s)
        }
  - CALL SITE: in loadTOML, between the os.ReadFile error block and `var fc fileConfig`, insert:
        // Defense-in-depth (PRD §9.17 FR-B7): remap abandoned [agent.*]/agent terminology → provider
        // BEFORE the typed decode, so a v2 file using the old terminology loads with its provider block
        // preserved (otherwise go-toml silently drops [agent.*] — fileConfig has no Agent field).
        data = remapAgentTerminology(data)
  - WHY: this is THE fix. The remap runs on raw bytes before toml.Unmarshal, so `[agent.pi]`→`[provider.pi]`
      and `agent =`→`provider =` survive into fc.Defaults.Provider / fc.Provider["pi"], then materialize.
  - GOTCHA: do NOT also fold default_provider here (that's migrateV2ToV3's post-decode job). Do NOT touch
      any other loadTOML line (the timeout parse, the materialize call — all unchanged).

Task 2: EDIT internal/config/migrate.go — sync the two agent-terminology comments (LOGIC UNCHANGED)
  - EDIT the AGENT TERMINOLOGY doc-comment paragraph. Replace the text that says it's a "NO-OP in memory"
      / "no agent-keyed data ever reaches the typed Config" with text stating the remap is now handled
      textually upstream in loadTOML (remapAgentTerminology, before the typed decode), so agent-keyed data
      DOES reach cfg as provider; migrateV2ToV3 itself needs no agent logic (the upstream remap ran first).
      Example replacement (keep concise, factual):
        // AGENT TERMINOLOGY (FR-B7 "first map agent/[agent.*] → provider/[provider.*]"): handled
        // UPSTREAM in loadTOML, which calls remapAgentTerminology on the raw TOML text BEFORE the typed
        // decode — so agent-keyed data reaches the typed Config already remapped to provider. This
        // function therefore needs no agent logic; it only folds default_provider (below). The on-disk
        // `config upgrade` command (P1.M3.T1.S2) performs the same remap when persisting to the file.
  - EDIT the `(0)` implementation comment block. Replace the "documented no-op / no agent data reaches
      cfg / silently drops [agent.*]" text with a one-line note that the remap is done upstream in loadTOML:
        // (0) agent→provider: handled UPSTREAM by loadTOML's remapAgentTerminology (before decode), so
        // cfg already uses provider terminology here. No agent-specific work in this function.
  - WHY: contract §5 (the comments currently contradict the new behavior). Their old "no-op" wording is a
      maintenance trap once loadTOML remaps.
  - GOTCHA: migrateV2ToV3's DEFAULT_PROVIDER FOLD LOGIC (steps 1-3) IS UNCHANGED. Only the two comments
      change. Do not add/remove any executable line.

Task 3: EDIT internal/config/file_test.go — add the integration test (the contract's explicit ask)
  - ADD `TestLoadTOML_AgentTerminologyRemapped` (mirror TestLoadTOMLValid; reuse writeTempTOML):
        func TestLoadTOML_AgentTerminologyRemapped(t *testing.T) {
            body := `
config_version = 2

[defaults]
agent = "pi"

[agent.pi]
default_model = "glm-5.2"
`
            path := writeTempTOML(t, body)
            cfg, err := loadTOML(path)
            if err != nil || cfg == nil {
                t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
            }
            if cfg.Provider != "pi" {
                t.Errorf("Provider=%q want \"pi\" (agent→provider remap lost the default)", cfg.Provider)
            }
            m, ok := cfg.Providers["pi"]
            if !ok {
                t.Fatalf("Providers[\"pi\"] missing (agent→provider remap lost the [agent.pi] block)")
            }
            if m["default_model"] != "glm-5.2" {
                t.Errorf("pi.default_model=%v want glm-5.2", m["default_model"])
            }
        }
  - WHY: the contract's explicit TDD case. Today (pre-fix) it FAILS (Provider=="", no Providers["pi"]);
      after Task 1 it PASSES → real regression guard.
  - GOTCHA: this test must PASS only after Task 1. (If run before, it documents the bug.)

Task 4: EDIT internal/config/file_test.go — add the helper unit test (idempotency + precision)
  - ADD a table-driven `TestRemapAgentTerminology` (no file I/O; calls remapAgentTerminology directly):
        func TestRemapAgentTerminology(t *testing.T) {
            tests := []struct{ name, in, want string }{
                {"table header", "[agent.pi]", "[provider.pi]"},
                {"key spaced", `agent = "pi"`, `provider = "pi"`},
                {"key tight", `agent="pi"`, `provider="pi"`},
                {"indented key", "  agent = \"pi\"", "  provider = \"pi\""},
                {"comment untouched", "# agent = keep", "# agent = keep"},
                {"value untouched", `model = "agent"`, `model = "agent"`},
                {"prefixed key untouched", "my_agent = \"x\"", "my_agent = \"x\""},
                {"already-provider idempotent", "[provider.pi]\nprovider = \"pi\"", "[provider.pi]\nprovider = \"pi\""},
                {"header + key together", "[agent.pi]\nagent = \"pi\"", "[provider.pi]\nprovider = \"pi\""},
            }
            for _, tc := range tests {
                t.Run(tc.name, func(t *testing.T) {
                    got := string(remapAgentTerminology([]byte(tc.in)))
                    if got != tc.want {
                        t.Errorf("remapAgentTerminology(%q) = %q, want %q", tc.in, got, tc.want)
                    }
                })
            }
            // double-run idempotency: remap(remap(x)) == remap(x) on a mixed input
            mixed := "[agent.pi]\nagent = \"pi\"\nmodel = \"agent\"\n"
            once := string(remapAgentTerminology([]byte(mixed)))
            twice := string(remapAgentTerminology([]byte(once)))
            if twice != once {
                t.Errorf("remap not idempotent on mixed input:\n once=%q\n twice=%q", once, twice)
            }
        }
  - WHY: pins the precise behavior (key-name only; idempotency) the contract demands ("be careful to only
      remap the key name, not arbitrary occurrences"). Catches a naive blind-replace regression.
  - GOTCHA: assert EXACT equality (got == want), not Contains — the helper must be surgical. The
      "already-provider" + "double-run" cases are the idempotency guards.

Task 5: VERIFY (run all gates; fix before declaring done)
  - `gofmt -w internal/config/file.go internal/config/migrate.go internal/config/file_test.go`
  - `go vet ./internal/config/`
  - `go test ./internal/config/ -run 'TestLoadTOML_AgentTerminologyRemapped|TestRemapAgentTerminology' -v`
      → ALL PASS.
  - `go test ./internal/config/ -v` → ALL PASS (no regression in TestLoadTOMLValid/TestLoadTOML_V2Fields/
      TestMigrateV2ToV3 — the migrate.go comment edit must not change logic).
  - `go build ./... && go test ./...` → GREEN (whole tree; proves the migrate.go edit is comment-only).
  - `git diff --exit-code go.mod go.sum` → empty (no new deps; regexp+strings are stdlib).
  - `git status --porcelain` → EXACTLY 3 files: file.go, migrate.go, file_test.go.
```

### Implementation Patterns & Key Details

```go
// PATTERN (the two transforms — pure + idempotent):
var agentKeyRe = regexp.MustCompile(`(?m)^(\s*)agent(\s*=)`)

func remapAgentTerminology(data []byte) []byte {
	s := string(data)
	s = strings.ReplaceAll(s, "[agent.", "[provider.")     // (a) table headers
	s = agentKeyRe.ReplaceAllString(s, "${1}provider${2}") // (b) bare key, line-oriented, key-name only
	return []byte(s)
}

// PATTERN (the call site in loadTOML — BEFORE the typed decode):
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) { return nil, nil }
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	// Defense-in-depth (PRD §9.17 FR-B7): remap abandoned [agent.*]/agent → provider BEFORE the typed
	// decode so a v2 agent-terminology file loads with its provider block preserved.
	data = remapAgentTerminology(data)

	var fc fileConfig
	if err := toml.Unmarshal(data, &fc); err != nil { ... }

// PATTERN (the test — mirror TestLoadTOMLValid; reuse writeTempTOML):
//   path := writeTempTOML(t, body); cfg, err := loadTOML(path); if err != nil || cfg == nil { t.Fatalf(...) }
//   then assert cfg.Provider + cfg.Providers["pi"]["default_model"].

// CRITICAL: the key regexp is `(?m)^(\s*)agent(\s*=)` — MULTILINE so `^` is line-start; it matches ONLY a
//   bare `agent =` key. `${1}` = indent, `${2}` = ws+'='. It cannot match `# agent =`, `model = "agent"`,
//   `my_agent =`, or `[agent.pi]`. This is the surgical precision the contract demands.

// CRITICAL: do NOT fold default_provider in loadTOML — that's migrateV2ToV3's post-decode job (load.go:157).
//   The remap ONLY renames terminology so the data survives decode; the slash-prefix fold stays put.

// CRITICAL: migrate.go is a COMMENT-ONLY edit. Do not add/remove executable lines in migrateV2ToV3.
```

### Integration Points

```yaml
DECODE (loadTOML, file.go):
  - insert: "data = remapAgentTerminology(data) between the os.ReadFile error block and `var fc fileConfig`"
  - imports: "+regexp, +strings (both absent today)"

MIGRATION DOC (migrate.go):
  - edit: "the AGENT TERMINOLOGY doc-comment paragraph + the (0) implementation comment"
  - logic: "UNCHANGED (comment-only edit)"

TESTS (file_test.go):
  - add: "TestLoadTOML_AgentTerminologyRemapped (integration via writeTempTOML+loadTOML)"
  - add: "TestRemapAgentTerminology (table-driven helper unit test; idempotency + key-name-only precision)"

GO.MODULE: change NONE. regexp + strings are stdlib. `go mod tidy` is a no-op.

FROZEN/LEAVE (do NOT edit):
  - internal/config/load.go (migrateV2ToV3 call at :157; complementary order).
  - internal/cmd/config.go (on-disk agentHeaderRe/rewriteV2ToV3 — mirror the regexp, don't reuse/import).
  - internal/config/bootstrap.go, bootstrap_test.go (parallel P1.M4.T1.S1).
  - migrateV2ToV3's default_provider fold logic (steps 1-3 in migrate.go).
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/config/file.go internal/config/migrate.go internal/config/file_test.go
go vet ./internal/config/
gofmt -l internal/config/   # must be empty
# Confirm the imports + call landed:
grep -n 'regexp\|"strings"' internal/config/file.go          # expect BOTH imports present
grep -n 'remapAgentTerminology(data)' internal/config/file.go # expect the call in loadTOML
# Confirm go.mod/go.sum unchanged:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; both imports present; the call present in loadTOML; go.mod/go.sum byte-identical.
```

### Level 2: Unit tests (the new tests + no regression)

```bash
# The two new tests in isolation:
go test ./internal/config/ -run 'TestLoadTOML_AgentTerminologyRemapped|TestRemapAgentTerminology' -v
# Expected: ALL PASS. (TestLoadTOML_AgentTerminologyRemapped FAILS without Task 1 — verify by reverting if desired.)

# Full config suite (no regression — esp. the loadTOML + migrate tests):
go test ./internal/config/ -v
# Expected: ALL PASS. Critically: TestLoadTOMLValid + TestLoadTOML_V2Fields (provider-terminology files
# unaffected by the idempotent remap) + TestMigrateV2ToV3 (migrate.go was comment-only — logic unchanged).
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...     # Expect clean (the migrate.go edit is comment-only; file.go adds a pure helper).
go test ./...      # Expect ALL PASS — nothing else depends on raw agent-terminology text.
# Confirm EXACTLY 3 files differ:
git status --porcelain
# Expected: exactly 3 modified files (file.go, migrate.go, file_test.go).
# Confirm frozen/LEAVE files are untouched:
git diff --exit-code internal/config/load.go internal/cmd/config.go go.mod go.sum \
  && echo "frozen files UNCHANGED (expected)"
```

### Level 4: Correctness reasoning (idempotency + precision + the trace)

```bash
# No server/DB. Verify by reasoning + Level 2:
#   1. The remap populates cfg — the integration test proves cfg.Provider=="pi" + Providers["pi"] set.
#   2. Idempotency — TestRemapAgentTerminology's "already-provider" + "double-run" cases prove a normal
#      provider-terminology file is byte-unaffected.
#   3. Precision — the "comment untouched" / "value untouched" / "prefixed key untouched" cases prove the
#      key remap is key-name-only (no blind "agent"→"provider" replace).
#   4. Complementary order — load.go:157 still calls migrateV2ToV3 AFTER loadTOML decodes (unchanged):
grep -n 'migrateV2ToV3(&cfg)' internal/config/load.go   # expect the post-decode call still present
#   5. migrate.go is comment-only — confirm no executable line changed:
git diff internal/config/migrate.go | grep -E '^\+[^+]' | grep -vE '^\+\s*//' \
  && echo "WARNING: non-comment line added to migrate.go" || echo "OK: migrate.go is comment-only"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l internal/config/` clean.
- [ ] `go test ./...` PASS (config suite incl. the 2 new tests; no repo-wide regression).
- [ ] go.mod/go.sum byte-unchanged.
- [ ] `git status` shows EXACTLY 3 modified files (file.go, migrate.go, file_test.go); every LEAVE file
      (load.go, cmd/config.go, bootstrap*.go) unchanged.

### Feature Validation
- [ ] A v2 file with `[defaults] agent = "pi"` + `[agent.pi] default_model = "glm-5.2"` loads with
      `cfg.Provider == "pi"` AND `cfg.Providers["pi"]["default_model"] == "glm-5.2"`.
- [ ] `TestLoadTOML_AgentTerminologyRemapped` PASSES (and FAILS without the loadTOML call).
- [ ] The remap is idempotent: a normal `[provider.pi]` / `provider =` file is byte-unaffected
      (TestRemapAgentTerminology "already-provider" + "double-run" cases).
- [ ] The key remap is key-name-only: comments, values, prefixed keys, and headers are not corrupted
      (TestRemapAgentTerminology precision cases).
- [ ] migrate.go's two agent-terminology comments updated; its default_provider-fold LOGIC unchanged.

### Code Quality Validation
- [ ] `remapAgentTerminology` is pure + idempotent + has a doc comment citing FR-B7.
- [ ] `agentKeyRe` uses the multiline `(?m)^(\s*)agent(\s*=)` form (key-name only); the header transform
      uses the literal `strings.ReplaceAll("[agent.", "[provider.")` (per the contract).
- [ ] The new tests mirror existing patterns (writeTempTOML+loadTOML; table-driven helper test).
- [ ] No new deps (regexp + strings are stdlib); no out-of-scope churn.
- [ ] Anti-patterns avoided (see below).

### Documentation
- [ ] migrate.go's comments no longer claim the agent remap is a no-op; they point at loadTOML's
      remapAgentTerminology as the upstream owner of the agent→provider step.
- [ ] remapAgentTerminology + agentKeyRe have doc comments explaining the FR-B7 defense-in-depth purpose.
- [ ] No new env vars / config keys / CLI flags.

---

## Anti-Patterns to Avoid

- ❌ **Don't use a blind `strings.ReplaceAll(s, "agent", "provider")`.** It corrupts `model = "agent"`,
  comments, `[agent.pi]` mid-value, and `my_agent`. Use the line-oriented `(?m)^(\s*)agent(\s*=)` regexp
  for the KEY (key-name only) and the literal `[agent.`→`[provider.` for HEADERS. (gotcha)
- ❌ **Don't fold `default_provider` in loadTOML.** That's migrateV2ToV3's post-decode job (load.go:157).
  The remap ONLY renames terminology so data survives decode; the slash-prefix fold stays where it is.
- ❌ **Don't change migrateV2ToV3's logic.** The migrate.go edit is COMMENT-ONLY (the two agent-terminology
  comments). Adding/removing executable lines risks the 14-case TestMigrateV2ToV3 table. (Task 2 gotcha)
- ❌ **Don't forget BOTH imports.** file.go needs `regexp` AND `strings` (both absent today). One missing →
  compile error. (gotcha)
- ❌ **Don't import/reuse internal/cmd/config.go.** It has the on-disk twin (agentHeaderRe/rewriteV2ToV3);
  mirror the regexp idiom locally in file.go (2 lines). internal/config must not import internal/cmd
  (layering/cycle). (gotcha)
- ❌ **Don't edit load.go, cmd/config.go, or bootstrap*.go.** load.go's migrateV2ToV3 call (complementary
  order) is unchanged; cmd/config.go owns the on-disk path; bootstrap*.go is parallel P1.M4.T1.S1. (scope)
- ❌ **Don't assert `Contains` in the helper unit test.** Use exact equality (`got == want`) — the helper
  must be surgical; a `Contains` check would miss an over-eager remap corrupting the rest of the line.
- ❌ **Don't skip the idempotency cases.** The "already-provider" and "double-run" cases are the regression
  guard that a normal provider-terminology file loads byte-unaffected (the contract's idempotency ask).
