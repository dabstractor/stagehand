---
name: "P1.M3.T1.S2 — `config upgrade` on-disk →v3 rewrite + test fixture updates: the ON-DISK half of FR-B7 (fold default_provider into the model slash-prefix, rename [agent.*]→[provider.*], bump config_version to 3), as a TEXTUAL line rewrite in upgradeConfigVersion — PRD §9.17 FR-B5/FR-B7 / §16.2"
description: |

  Land the SECOND subtask of Config v3 migration (P1.M3.T1): the ON-DISK `config upgrade` rewrite. S1 (the
  parallel subtask) owns the IN-MEMORY migration (`migrateV2ToV3` on the resolved `*Config` inside `Load`,
  `CurrentConfigVersion`→3, the version-literal test-fixture sweep). S2 owns the ON-DISK half: extend
  `internal/cmd/config.go`'s `upgradeConfigVersion` (L178-204, today a PURE textual version-bump) so that
  upgrading a `<v3` file performs the FR-B7 rewrite IN THE FILE TEXT — then add rewrite-behavior tests and
  the Mode-A help text.

  WHY ON-DISK IS TEXTUAL (not struct): FR-B5 mandates "preserving user values … commenting out removed/renamed
  keys with a note … leave all other content unchanged." go-toml re-marshaling DROPS comments and reorders
  keys — incompatible with FR-B5. So the rewrite is a SURGICAL LINE-BASED edit over `strings.Split(content,
  "\n")` that preserves every other line byte-for-byte. (S1's `migrateV2ToV3` is the in-memory STRUCT
  counterpart on `*Config`; the two implement the SAME FR-B7 mapping in different domains. S2 does NOT reuse
  S1's function — different input.)

  THE →v3 REWRITE (PRD §9.17 FR-B7, AUTHORITATIVE — `rewriteV2ToV3(content) string`):
    Pass 1: rename every `[agent.<name>]` table header → `[provider.<name>]` (FR-B7 "map agent/[agent.*] →
            provider/[provider.*] first").
    Pass 2a (collect): scan, tracking the current table path. Record each `[provider.<name>] default_provider`
            value X, each `[provider.<name>] provider_flag` value, the `[defaults] provider` (global), and each
            `[role.<r>] provider`. Build `providerPrefix[name]=X` ONLY for MULTI-BACKEND providers (name=="pi"
            OR a non-empty provider_flag — mirrors S1's `isMultiBackend`/`v2MultiBackendBuiltins`, but sourced
            from the text). Single-backend default_provider is NOT a prefix (FR-B7 "single-backend untouched").
    Pass 2b (emit): scan again. For each uncommented `key = "val"` line:
            - `[provider.<name>] default_provider`  → COMMENT OUT with a note (FR-B5; removed in v3).
            - `[provider.<name>] default_model`     → if providerPrefix[name] set & val bare → "X/val".
            - `[defaults] model`                    → if providerPrefix[globalProvider] set & val bare → "X/val".
            - `[role.<r>] model`                    → effective provider = roleProvider[r] or globalProvider;
                                                      if providerPrefix[ep] set & val bare → "X/val".
            Bare = !strings.Contains(val,"/") (idempotent + no-invent). Every other line unchanged.
    (config_version itself is set by the caller, NOT here.)

  ⚠️ **#1 — THE GATE (load-bearing for conflict-free parallel execution).** `upgradeConfigVersion(content,
  version)` is restructured: `cur := parseTopLevelConfigVersion(content); if cur >= version → no-op (idempotent
  / ahead); if version >= 3 && cur < 3 → setConfigVersionLine(rewriteV2ToV3(content), version); else (target
  < 3) → setConfigVersionLine(content, version)` (forward-compat old behavior). VERIFIED at PRP-writing time:
  the existing `TestUpgradeConfigVersion_*` call `upgradeConfigVersion(input, config.CurrentConfigVersion)`
  (= 3 after S1 — S1's parallel §6 sweep converted any literal `2`). The gate preserves them because (a)
  `cur >= version → no-op` handles the "current"/idempotent cases, AND (b) `rewriteV2ToV3` is a **NO-OP on
  clean inputs** (no `default_provider`, no `[agent.*]`) — the existing test inputs are clean, so only the
  `config_version` line is set to 3. S2 therefore EDITS NO existing pure-unit test; its test changes are
  ADDITIVE. See design-decisions §1/§5.

  ⚠️ **#2 — The raw provider model key is `default_model`, NOT `model`.** Inside `[provider.<name>]` blocks
  the field is `default_model` (manifest tag `DefaultModel toml:"default_model"`); inside `[defaults]` and
  `[role.<r>]` it is `model`. The fold targets the RIGHT key per section. (scout §(f); S1's #2 gotcha.)

  ⚠️ **#3 — Multi-backend classify ON DISK mirrors S1: pi OR a non-empty provider_flag (from the text).** Do
  NOT import/consult the Manifest during a textual rewrite. opencode/agy use the model prefix WITHOUT
  provider_flag and never carried a v2 default_provider → single-backend here (no fold). (FR-B7; S1 §1.)

  ⚠️ **#4 — COMMENT OUT default_provider (FR-B5), do not hard-delete.** Prefix `# ` + append a note. The item
  contract's "(a) DELETE the default_provider line" is reconciled to comment-out-with-note per the AUTHORITATIVE
  FR-B5 ("commenting out removed/renamed keys with a note") — auditable, reversible, "preserving user values".
  Applies to EVERY default_provider in a provider block (the field is gone in v3).

  ⚠️ **#5 — IDEMPOTENT.** `cur >= version → no-op` (a v3 file re-upgraded with target=3 is unchanged → "already
  up to date"); the fold's bare-check skips already-prefixed models; a commented `# default_provider` is not
  re-processed. `config upgrade` is safe to run any number of times. (FR-B5.)

  ⚠️ **#6 — S2's test changes are ADDITIVE; the version-literal sweep is S1's.** S2 ADDS
  `TestUpgradeConfigVersion_V3Rewrite_*` (pure unit, `upgradeConfigVersion(input, 3)`) + a command round-trip
  `TestConfigUpgrade_V2ToV3Rewrite`. S2 does NOT touch `internal/config/*` (S1) NOR `default_action_test.go`
  (L1203 — exercised by the default action's IN-MEMORY migration, S1's domain) NOR the `config_version = 2`→`3`
  literal OUTPUT fixes (S1's §6 breakage map). S2 only VERIFIES the existing `TestConfigUpgrade_*` still pass
  under the gate (they do — their v2 inputs have no default_provider, so the rewrite is version-only; §5).

  ⚠️ **#7 — `config upgrade` Long help (Mode A DOCS): S2 owns the FINAL text.** S1's provisional Long touch
  (its §7) is superseded by S2's comprehensive text describing the on-disk →v3 rewrite (fold + agent rename +
  comment-out + bump) AND the in-memory auto-migration (S1). S2 implements the on-disk behavior → S2 is
  authoritative on the command's help.

  ⚠️ **#8 — NO new imports; go.mod UNCHANGED.** cmd/config.go already imports regexp/strconv/strings/toml
  (+cobra/config/provider/exitcode). The rewrite adds new regexes (pkg-level vars) + helpers; no new import.
  `go mod tidy` is a no-op.

  Deliverable: MODIFIED `internal/cmd/config.go` (gated `upgradeConfigVersion` + `rewriteV2ToV3` + helpers +
  refactored `parseTopLevelConfigVersion`/`setConfigVersionLine` + the `config upgrade` Long text) + MODIFIED
  `internal/cmd/config_test.go` (ADDITIVE rewrite-behavior tests). NO other file touched. OUTPUT:
  `stagecoach config upgrade` rewrites an existing `<v3` file to v3 in place (models prefixed, default_provider
  commented out, [agent.*] renamed, config_version=3), preserving all other lines; idempotent; `go build/vet/
  test ./...` green.

---

## Goal

**Feature Goal**: Implement the on-disk half of the v3 config migration (PRD §9.17 FR-B5/FR-B7): extend
`config upgrade` so that upgrading a `<v3` config file performs the FR-B7 rewrite IN THE FILE TEXT — folding
each multi-backend provider's `default_provider` into a slash-prefix on its model (`default_model`, the global
`[defaults] model`, and each `[role.<r>] model` that targets it), commenting out the removed `default_provider`
key with a note, renaming any `[agent.<name>]` tables to `[provider.<name>]`, and bumping `config_version` to
3 — while preserving every other line byte-for-byte (FR-B5). Idempotent (a v3 file is a no-op). This is the
persist-to-disk complement to S1's in-memory migration: a user runs it once to stop the load-time deprecation
notice and make the file v3-native.

**Deliverable** (all EDITS — no new files):
1. **MODIFIED `internal/cmd/config.go`**:
   - REFACTOR `upgradeConfigVersion` (L178) into the GATED form: `cur >= version`→no-op; `version>=3 && cur<3`→
     `setConfigVersionLine(rewriteV2ToV3(content), version)`; else→`setConfigVersionLine(content, version)`.
   - EXTRACT `parseTopLevelConfigVersion(content) int` + `setConfigVersionLine(content, version) (string,bool)`
     helpers (from the existing scan/insert body) — pure, reusable.
   - ADD `rewriteV2ToV3(content) string` (the 3-pass textual rewrite: agent→provider rename, collect state,
     fold + comment-out) + its helpers (`isMultiBackendText`, `replaceQuotedValue`, `commentOutWithNote`) +
     pkg-level regexes (`agentHeaderRe`, `tableHeaderRe`, `kvStringRe`).
   - REWRITE `configUpgradeCmd.Long` (L95) — Mode-A help describing the →v3 rewrite + in-memory auto-migration.
2. **MODIFIED `internal/cmd/config_test.go`**:
   - ADD `TestUpgradeConfigVersion_V3Rewrite_*` (pure unit — fold global/per-role/provider `default_model`;
     single-backend untouched; comment-out; agent rename; idempotent; no-invent; config_version set to 3).
   - ADD `TestConfigUpgrade_V2ToV3Rewrite` (command round-trip — write v2 file, upgrade, assert on-disk
     prefixed model + commented default_provider + config_version=3; re-run → no change).

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean; a v2 file
with `[provider.pi] default_provider = "zai"` + `[defaults] model = "glm-5.2"` upgraded in place yields
`model = "zai/glm-5.2"` (global + per-role + provider `default_model`), a commented-out `default_provider`
with a note, `[agent.*]`→`[provider.*]`, and `config_version = 3`; a single-backend provider (claude) has its
`default_provider` commented but its model NOT prefixed; re-running `config upgrade` is a no-op; the existing
`TestUpgradeConfigVersion_*` (version=2) and `TestConfigUpgrade_*` tests still pass unchanged; go.mod/go.sum
byte-unchanged; `internal/config/*` + `default_action_test.go` byte-unchanged.

## User Persona

**Target User**: A user who upgraded the stagecoach binary (v3) and keeps a v2 (or unversioned) config file
with `[provider.pi] default_provider = "zai"` + bare models. S1 makes it LOAD correctly (in-memory fold +
deprecation notice). S2 lets the user PERSIST the migration: `stagecoach config upgrade` rewrites the file to
v3-native form so the notice stops and the file is self-consistent. Transitively: anyone reading the file
after upgrade (editors, version control, future loads).

**Use Case**: After seeing S1's one-time "auto-migrated in memory … run 'stagecoach config upgrade' to persist"
notice, the user runs `stagecoach config upgrade`. The file is rewritten in place: `model = "glm-5.2"` becomes
`model = "zai/glm-5.2"`, the `default_provider = "zai"` line is commented out with a note, `config_version`
becomes 3. Next load: no notice, no in-memory migration needed.

**User Journey**: (CLI) `stagecoach config upgrade` → `runConfigUpgrade` reads file → validates TOML →
`upgradeConfigVersion(data, 3)`: `cur=2 < 3` → `rewriteV2ToV3` (fold + comment-out + agent rename) →
`setConfigVersionLine(…, 3)` → write back → "Upgraded config … to version 3." Re-run → `cur=3 >= 3` → no-op →
"already at version 3".

**Pain Points Addressed**: Without S2, the v3 migration is invisible on disk — the file stays v2-shaped, the
deprecation notice fires every load, and the `default_provider` dead key lingers confusingly. S2 makes the
migration real and permanent, auditable (commented-out key with a note), and safe (idempotent, preserves
everything else).

## Why

- **Completes FR-B7 (the on-disk half).** S1 migrates in memory (transparent load); FR-B7 also says
  "`config upgrade` performs the same rewrite on disk." S2 is that command behavior. Together S1+S2 satisfy
  the full FR-B7.
- **Satisfies PRD §9.17 FR-B5.** "`config upgrade` rewrites an existing config to CurrentConfigVersion in
  place: preserving user values for keys that still exist, commenting out removed/renamed keys with a note.
  Simple, idempotent, future-extensible." S2 implements exactly this for the v2→v3 step.
- **Textual (not struct) by necessity.** FR-B5's "commenting out removed/renamed keys with a note" + "preserve
  user values … leave all other content unchanged" REQUIRES a surgical line edit; re-marshaling would discard
  comments/ordering. S2 is the only faithful implementation.
- **Idempotent + additive to the existing command.** The gate makes the v3 rewrite a strict superset of the
  existing version-bump behavior (target<3 path unchanged); no regression for any existing caller/test.

## What

A modified `internal/cmd/config.go` (gated `upgradeConfigVersion` + `rewriteV2ToV3` + helpers + refactored
scan/insert + the `config upgrade` Long text) and a modified `internal/cmd/config_test.go` (additive rewrite-
behavior tests). No new files, no new imports, no dependency change, no `internal/config/*` change.

### Success Criteria

- [ ] `upgradeConfigVersion` is GATED: `cur >= version`→`(content,false)`; `version>=3 && cur<3`→
      `setConfigVersionLine(rewriteV2ToV3(content), version)`; else→`setConfigVersionLine(content, version)`.
- [ ] `parseTopLevelConfigVersion` + `setConfigVersionLine` extracted (pure; the target<3 path is byte-
      identical to today's behavior → existing `TestUpgradeConfigVersion_*` pass UNCHANGED).
- [ ] `rewriteV2ToV3` does the 3-pass textual rewrite: `[agent.<name>]`→`[provider.<name>]`; folds
      `default_provider`→model prefix for MULTI-BACKEND providers only (pi OR non-empty provider_flag) across
      `[provider.<name>] default_model`, `[defaults] model`, `[role.<r>] model`; comments out EVERY
      `default_provider` with a note; bare-check (no-invent); preserves all other lines.
- [ ] `configUpgradeCmd.Long` describes the →v3 rewrite (fold + agent rename + comment-out + bump) + the
      in-memory auto-migration (Mode A).
- [ ] `TestUpgradeConfigVersion_V3Rewrite_*` + `TestConfigUpgrade_V2ToV3Rewrite` added and passing.
- [ ] The existing `TestUpgradeConfigVersion_*` (version=2) + `TestConfigUpgrade_*` tests pass UNCHANGED.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/cmd/` clean.
- [ ] go.mod/go.sum byte-unchanged; `internal/config/*` + `internal/cmd/default_action_test.go` + every other
      file byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the gated `upgradeConfigVersion`
+ `rewriteV2ToV3` + helpers (copy-ready in the Blueprint), the gate rationale (preserves existing tests —
verified case-by-case), the `default_model`-vs-`model` key distinction, the multi-backend classify rule, the
comment-out-with-note decision, the existing `upgradeConfigVersion`/`isTableHeader`/`leadingHeaderEnd`/
`configVersionLineRe` (the code being refactored), the S1 contract (in-memory half + version-literal sweep),
and the additive test plan. No decompose/render/git knowledge required — S2 is a textual line transform.

### Documentation & References

```yaml
# MUST READ — the design calls + the gate + the conflict-free test split
- docfile: plan/003_6ce49c39466e/P1M3T1S2/research/design-decisions.md
  why: the 10 calls — scope/S1-split (§0), THE GATE preserving existing unit tests (§1, verified case-by-case),
       on-disk=TEXTUAL not struct (§2), multi-backend classify from text mirroring S1 (§3), default_model key
       (§4), existing command tests pass under the gate + the AlreadyCurrent premise (§5), comment-out not
       delete (§6), idempotency (§7), Long text ownership (§8), no new imports (§9), additive tests (§10).
  critical: §1 (THE GATE — without it existing tests break and S1/S2 collide on config_test.go), §4
       (default_model not model), §0 (S2 is additive; the version-literal sweep + default_action_test.go are
       S1's) are the things most likely to derail one-pass success.

# MUST READ — the S1 CONTRACT (the in-memory half + the version-literal sweep S1 owns)
- docfile: plan/003_6ce49c39466e/P1M3T1S1/PRP.md
  why: S1 ships `CurrentConfigVersion=3`, the in-memory `migrateV2ToV3` (on `*Config`), `Load` wiring, and the
       §6 version-literal test-fixture sweep (config_test.go OUTPUT `config_version = 2`→3; keep INPUT v2
       fixtures). S2 CONSUMES `CurrentConfigVersion=3` (the command's target) and must NOT redo S1's sweep.
  critical: §6 — S1 owns the `config_version = 2`→`3` literal fixes and `default_action_test.go`; S2 is
       ADDITIVE only. S1's `isMultiBackend`/`v2MultiBackendBuiltins={"pi"}` is the in-memory twin of S2's
       `isMultiBackendText` — mirror it so the two migrations are provably consistent.

# MUST READ — the authoritative touchpoint map
- docfile: plan/003_6ce49c39466e/architecture/scout_config_model.md
  section: "§(d) CurrentConfigVersion + config_version read/compare/write" — flags `upgradeConfigVersion`
           (L178-204) as "the future extension point — v3 migration needs more than a version bump"; lists the
           test fixtures that hardcode `config_version = 2`.
  section: "§(f) [provider.<name>] default_provider decode/use path (RAW map)" — confirms the provider-block
           model field is `default_model` (manifest tag), the value flows as raw `any`, the manifest field was
           removed in v3.
  critical: §(d) confirms S2's extension point is `upgradeConfigVersion` (not runConfigUpgrade). §(f) confirms
       the key is `default_model` in provider blocks (NOT `model`).

# The PRD basis (in your context as selected_prd_content)
- file: PRD.md (or plan/003_6ce49c39466e/prd_snapshot.md)
  section: "9.17 Config bootstrap & versioning" FR-B5 (idempotent, preserve user values, comment-out removed
           keys) + FR-B7 (the →v3 rewrite: fold default_provider into model prefix for multi-backend, global +
           per-role + the provider's own; remove default_provider; map agent/[agent.*]→provider/[provider.*]
           first; single-backend untouched; NO value invented; `config upgrade` does the same on disk) (h3.33).
  section: "16.2 Full config file example" (h3.67) — the v3 file shape: `model = "zai/glm-5.2"`, no
           `default_provider` field, `config_version 3`.
  critical: FR-B7 "No value is invented" + "Single-backend providers are untouched" are hard constraints the
       rewrite + tests must honor. FR-B5 "commenting out removed/renamed keys with a note" overrides the
       contract's loose "DELETE" wording.

# THE FILE BEING MODIFIED — READ FULLY before editing
- file: internal/cmd/config.go
  section: `runConfigUpgrade` (L140 — read→validate TOML→`upgradeConfigVersion(string(data),
           config.CurrentConfigVersion)`→write→print), `upgradeConfigVersion` (L178-204 — the PURE textual
           version-bump being extended), `configVersionLineRe` (L117), `isTableHeader` (L~206),
           `leadingHeaderEnd` (L~213), `configUpgradeCmd.Long` (L95-106).
  why: the EXACT current state S2 refactors. Note `upgradeConfigVersion`'s 3 outcomes (found==version→unchanged;
       found!=version→rewrite line; not found→insert after header) — S2 preserves these for target<3 via
       `setConfigVersionLine`. Note the imports already include regexp/strconv/strings/toml.
  critical: REUSE `configVersionLineRe`, `isTableHeader`, `leadingHeaderEnd` (do not redefine). The cmd package
       imports `internal/provider` already — but DO NOT use it in `rewriteV2ToV3` (classify locally, §3).

# THE TEST FILE BEING MODIFIED — mirror its idioms; existing tests stay GREEN
- file: internal/cmd/config_test.go
  section: `TestUpgradeConfigVersion_*` (L808-916 — PURE unit, `upgradeConfigVersion(input, 2)`; S2 leaves
           these UNCHANGED — the gate preserves them), `TestConfigUpgrade_*` (L951-1076 — COMMAND tests via
           the real CurrentConfigVersion; S2 leaves these, only VERIFIES), the `writeConfigFile`/temp-home
           harness pattern (L951 TestConfigUpgrade_AddsVersion).
  why: the test STYLE (white-box `package cmd`; table/sub-tests; temp-home + writeConfigFile for command
           tests) and the harness pattern for the new round-trip test. Confirms the gate's claim (existing
           tests pass unchanged) is checkable.
  critical: do NOT edit the existing upgrade tests' assertions (S1 owns their version literals; the gate
       preserves their behavior). S2 ADDS new test functions only.

# The default_action fixture S2 does NOT touch (S1's in-memory domain)
- file: internal/cmd/default_action_test.go   (read-only — do NOT edit)
  section: L1203 `TestRunDefault_ProviderSubProviderRendering_Issue1` — a v2 INPUT fixture (`config_version = 2`
           + `[provider.pi] default_provider = "openrouter"`) fed to the DEFAULT ACTION (not `config upgrade`).
  why: confirms this fixture is exercised by the default action's IN-MEMORY migration (S1), NOT by S2's
       on-disk upgrade. S2 does not touch it.
  critical: if a test here references `config upgrade` behavior, that would be S2 — but L1203 is a default-
       action render test (S1's migration). Leave it.

# The frozen in-memory twin (S1 — read-only)
- file: internal/config/migrate.go   (S1 — read-only; exists after S1 lands)
  section: `migrateV2ToV3`, `isMultiBackend`, `v2MultiBackendBuiltins`.
  why: S2's `isMultiBackendText`/`rewriteV2ToV3` are the on-disk TEXTUAL twins of S1's struct functions —
       mirror the multi-backend rule (pi OR provider_flag) so both migrations agree.
  critical: do NOT import `internal/config`'s migrate functions into cmd for the rewrite (different domain:
       struct vs text). Classify locally.
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  config.go        # runConfigUpgrade + upgradeConfigVersion + configVersionLineRe + isTableHeader + leadingHeaderEnd
                   #   + configUpgradeCmd.Long — EDIT (gated upgradeConfigVersion + rewriteV2ToV3 + helpers + Long)
  config_test.go   # TestUpgradeConfigVersion_* + TestConfigUpgrade_* — EDIT (ADDITIVE rewrite tests only)
  default_action_test.go  # L1203 v2 default-action fixture — UNCHANGED (S1's in-memory domain)
  {root,default_action,providers}.go — UNCHANGED
internal/config/   # S1: config.go (CurrentConfigVersion=3) + migrate.go + load.go — UNCHANGED (S1's domain)
go.mod / go.sum     # UNCHANGED (no new import — regexp/strconv/strings/toml already present)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. All edits are in-place: internal/cmd/config.go (code + help) + internal/cmd/config_test.go (additive tests).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — THE GATE): `upgradeConfigVersion` MUST branch on `cur` vs `version`. `cur>=version`→no-op;
//   `version>=3 && cur<3`→ rewriteV2ToV3 + setConfigVersionLine; else (target<3)→setConfigVersionLine (forward-
//   compat). The existing TestUpgradeConfigVersion_* call it with config.CurrentConfigVersion (=3 after S1) and
//   their inputs are CLEAN (no default_provider/[agent.*]) — so rewriteV2ToV3 is a no-op on them and only the
//   version line is set. That (not the target<3 branch) is what keeps them passing UNCHANGED. Verify
//   rewriteV2ToV3 is genuinely a no-op on a default_provider-free input. (design-decisions §1)

// CRITICAL (#2 — provider-block model key is default_model, NOT model): inside [provider.<name>] fold
//   `default_model`; inside [defaults]/[role.<r>] fold `model`. Do NOT cross them. (scout §(f); §4)

// CRITICAL (#3 — multi-backend classify from TEXT, mirror S1): fold only when name=="pi" OR the provider
//   block's provider_flag is non-empty (captured in Pass 2a). Single-backend (claude/gemini/…/opencode/agy)
//   default_provider is commented out but NOT folded. Do NOT import/consult the Manifest in the textual
//   rewrite. (FR-B7; §3)

// CRITICAL (#4 — COMMENT OUT default_provider, do not hard-delete): prefix `# ` + append a note. FR-B5 is
//   authoritative ("commenting out removed/renamed keys with a note"). Applies to EVERY default_provider.
//   (§6)

// CRITICAL (#5 — IDEMPOTENT): cur>=version→no-op; the fold's bare-check (!strings.Contains(val,"/")) skips
//   already-prefixed models; a commented `# default_provider` is not matched by kvStringRe. (§7)

// CRITICAL (#6 — S2 is ADDITIVE for tests; the version-literal sweep is S1's): do NOT edit existing
//   TestUpgradeConfigVersion_* / TestConfigUpgrade_* assertions (S1 owns the config_version=2→3 literals;
//   the gate preserves behavior). Do NOT touch internal/config/* or default_action_test.go (S1). S2 ADDS new
//   test functions + VERIFIES the existing ones pass. (§0/§5)

// GOTCHA (kvStringRe must skip comments): a `#`-prefixed line must NOT match `^\s*(\w+)…` — ensure the regex
//   starts with `^\s*([A-Za-z_]…` so a leading `#` fails the word-char anchor. Verified in the Blueprint.
// GOTCHA (table tracking): use tableHeaderRe to BOTH detect a header and extract its dotted path; for a
//   header it doesn't match (e.g. [[array]]), set section to a non-matching sentinel so keys aren't
//   mis-attributed. Config files don't use array-of-tables for our sections.
// GOTCHA (map iteration order): Pass 2a builds providerPrefix from rawDP; Pass 2b reads it — order-independent.
// GOTCHA (re-use existing helpers): configVersionLineRe (L117), isTableHeader (L~206), leadingHeaderEnd
//   (L~213) already exist — REUSE them; do not redefine.
// GOTCHA (no new imports): regexp/strconv/strings/toml already imported; the rewrite adds pkg-level regexes
//   + unexported helpers only.
// GOTCHA (verify TestConfigUpgrade_AlreadyCurrent): if S1 left its INPUT at `config_version = 2`, S2 updates
//   the INPUT to config.CurrentConfigVersion (the genuine no-op case) — a one-line INPUT fix, not an OUTPUT
//   literal fix. Run it to confirm. (§5)
```

## Implementation Blueprint

### Data models and structure

```go
// === internal/cmd/config.go — pkg-level regexes (add near configVersionLineRe at L117) ===

// agentHeaderRe captures the name in an `[agent.<name>]` table header (the abandoned intermediate terminology
// mapped back to `[provider.<name>]` first, per FR-B7).
var agentHeaderRe = regexp.MustCompile(`^\[agent\.(.+?)\]\s*$`)

// tableHeaderRe captures the dotted path inside a simple `[table]` header (non-comment). Used to track the
// current section during the rewrite. Does NOT match array-of-tables `[[…]]` (config files don't use those for
// our sections; a non-match sets section to a non-matching sentinel).
var tableHeaderRe = regexp.MustCompile(`^\[([a-zA-Z0-9._-]+)\]\s*$`)

// kvStringRe captures an UNCOMMENTED `key = "value"` assignment (key, value). Leading whitespace allowed; a
// leading `#` fails the `[A-Za-z_]` anchor so comment lines (including the rewrite's own commented-out
// default_provider) are NOT matched. The value is the first double-quoted string; trailing inline comments
// are ignored. Used to read model/default_model/default_provider/provider/provider_flag values during the
// rewrite.
var kvStringRe = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_-]*)\s*=\s*"([^"]*)"`)
```

```go
// === internal/cmd/config.go — REFACTOR upgradeConfigVersion (L178) into the GATED form ===

// upgradeConfigVersion returns content upgraded to `version`. GATED (PRD §9.17 FR-B5/FR-B7):
//   - cur >= version → (content, false): already current (cur==version) or ahead (cur>version) — no-op.
//     This is the idempotency / "already up to date" path.
//   - version >= 3 && cur < 3 → setConfigVersionLine(rewriteV2ToV3(content), version): the on-disk →v3
//     rewrite (fold default_provider into the model slash-prefix, comment it out, rename [agent.*], then set
//     config_version=3).
//   - else (target < 3) → setConfigVersionLine(content, version): the ORIGINAL version-line-only behavior
//     (forward-compat; not test-exercised today). NOTE: the existing TestUpgradeConfigVersion_* call this with
//     config.CurrentConfigVersion (=3 after S1) and CLEAN inputs (no default_provider/[agent.*]); they pass
//     UNCHANGED via the `version>=3 && cur<3` branch because rewriteV2ToV3 is a no-op on clean inputs + the
//     `cur>=version`→no-op. Keep rewriteV2ToV3 a genuine no-op on default_provider-free input.
//
// PURE (no I/O, no error) → fully unit-testable.
func upgradeConfigVersion(content string, version int) (string, bool) {
	cur := parseTopLevelConfigVersion(content)
	if cur >= version {
		return content, false // idempotent (cur==version) or ahead (cur>version)
	}
	if version >= 3 && cur < 3 {
		return setConfigVersionLine(rewriteV2ToV3(content), version)
	}
	return setConfigVersionLine(content, version)
}

// parseTopLevelConfigVersion returns the top-level config_version integer (0 if missing, commented, or only
// present inside a [table]). Scans only the top-level region (before the first [table] header). Extracted from
// the pre-gate upgradeConfigVersion body.
func parseTopLevelConfigVersion(content string) int {
	for _, line := range strings.Split(content, "\n") {
		if isTableHeader(line) {
			break // config_version must precede tables
		}
		if m := configVersionLineRe.FindStringSubmatch(line); m != nil {
			if n, err := strconv.Atoi(strings.TrimSpace(m[1])); err == nil {
				return n
			}
		}
	}
	return 0
}

// setConfigVersionLine returns content with the TOP-LEVEL config_version set to `version`, via a minimal
// textual edit that preserves every other line. Found → that ONE line rewritten; not found → one line
// inserted after the leading comment/blank header block. Always returns changed=true (the caller has already
// gated on cur<version). Extracted from the pre-gate upgradeConfigVersion body; behavior for target<3 is
// byte-identical to v2.0.
func setConfigVersionLine(content string, version int) (string, bool) {
	lines := strings.Split(content, "\n")
	want := strconv.Itoa(version)
	for i, line := range lines {
		if isTableHeader(line) {
			break
		}
		if configVersionLineRe.FindStringSubmatch(line) != nil {
			lines[i] = "config_version = " + want
			return strings.Join(lines, "\n"), true
		}
	}
	insertAt := leadingHeaderEnd(lines)
	ins := append([]string{}, lines[:insertAt]...)
	ins = append(ins, "config_version = "+want)
	ins = append(ins, lines[insertAt:]...)
	return strings.Join(ins, "\n"), true
}
```

```go
// === internal/cmd/config.go — NEW rewriteV2ToV3 + helpers (the on-disk FR-B7 rewrite) ===

// rewriteV2ToV3 performs the PRD §9.17 FR-B7 on-disk rewrite on raw TOML TEXT (lines), preserving every line
// that is not transformed (FR-B5: "preserving user values … leave all other content unchanged"). It does NOT
// touch config_version (the caller sets that via setConfigVersionLine). IDEMPOTENT + INVENTS NOTHING.
//
// Three passes over the lines:
//  1. agent→provider: rename every `[agent.<name>]` table header → `[provider.<name>]` (FR-B7 "first").
//  2a. collect: track the current table; record each provider's default_provider (X) + provider_flag, the
//      global [defaults] provider, and each [role.<r>] provider. Build providerPrefix[name]=X ONLY for
//      MULTI-BACKEND providers (name=="pi" OR a non-empty provider_flag — mirrors internal/config's
//      isMultiBackend). Single-backend default_provider is NOT a prefix (FR-B7 "single-backend untouched").
//  2b. emit: comment out EVERY default_provider (removed in v3); fold the prefix onto default_model
//      ([provider.<name>]), model ([defaults]), and model ([role.<r>]) when the target provider has a prefix
//      and the value is bare (!strings.Contains(val,"/")).
//
// go-toml re-marshaling is REJECTED here: it drops comments and reorders keys, violating FR-B5's
// comment-out-with-note + preserve-user-values requirements. A surgical line edit is the only faithful
// implementation. (internal/config.migrateV2ToV3 is the in-memory STRUCT twin; same FR-B7 mapping, different
// domain — not reused.)
func rewriteV2ToV3(content string) string {
	lines := strings.Split(content, "\n")

	// Pass 1: agent→provider table-header rename.
	for i, line := range lines {
		if m := agentHeaderRe.FindStringSubmatch(line); m != nil {
			lines[i] = "[provider." + m[1] + "]"
		}
	}

	// Pass 2a: collect state.
	rawDP := map[string]string{}       // provider name → default_provider value
	providerFlag := map[string]string{} // provider name → provider_flag value
	globalProvider := ""
	roleProvider := map[string]string{}
	section := ""
	for _, line := range lines {
		if isTableHeader(line) {
			section = tableSection(line) // dotted path, or "" (a non-matching sentinel)
			continue
		}
		km := kvStringRe.FindStringSubmatch(line)
		if km == nil {
			continue
		}
		key, val := km[1], km[2]
		switch {
		case strings.HasPrefix(section, "provider."):
			name := strings.TrimPrefix(section, "provider.")
			if key == "default_provider" {
				rawDP[name] = val
			}
			if key == "provider_flag" {
				providerFlag[name] = val
			}
		case section == "defaults":
			if key == "provider" {
				globalProvider = val
			}
		case strings.HasPrefix(section, "role."):
			if key == "provider" {
				roleProvider[strings.TrimPrefix(section, "role.")] = val
			}
		}
	}
	// providerPrefix = X only for multi-backend providers with a non-empty default_provider.
	providerPrefix := map[string]string{}
	for name, x := range rawDP {
		if x != "" && isMultiBackendText(name, providerFlag[name]) {
			providerPrefix[name] = x
		}
	}

	// Pass 2b: emit (fold + comment-out).
	section = ""
	for i, line := range lines {
		if isTableHeader(line) {
			section = tableSection(line)
			continue
		}
		km := kvStringRe.FindStringSubmatch(line)
		if km == nil {
			continue
		}
		key, val := km[1], km[2]
		switch {
		case strings.HasPrefix(section, "provider."):
			name := strings.TrimPrefix(section, "provider.")
			if key == "default_provider" {
				lines[i] = commentOutWithNote(line, "v3 (FR-B7): removed — inference backend is now a slash-prefix on model")
			}
			if key == "default_model" { // raw provider model key is default_model (manifest tag), NOT model
				if x, ok := providerPrefix[name]; ok && val != "" && !strings.Contains(val, "/") {
					lines[i] = replaceQuotedValue(line, x+"/"+val)
				}
			}
		case section == "defaults":
			if key == "model" {
				if x, ok := providerPrefix[globalProvider]; ok && val != "" && !strings.Contains(val, "/") {
					lines[i] = replaceQuotedValue(line, x+"/"+val)
				}
			}
		case strings.HasPrefix(section, "role."):
			if key == "model" {
				ep := roleProvider[strings.TrimPrefix(section, "role.")]
				if ep == "" {
					ep = globalProvider // role inherits the global provider
				}
				if x, ok := providerPrefix[ep]; ok && val != "" && !strings.Contains(val, "/") {
					lines[i] = replaceQuotedValue(line, x+"/"+val)
				}
			}
		}
	}
	return strings.Join(lines, "\n")
}

// tableSection returns the dotted path inside a [...] table header (e.g. "provider.pi"), or "" for a header
// the strict regex doesn't match (so keys under it aren't mis-attributed to a real section).
func tableSection(line string) string {
	if m := tableHeaderRe.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

// isMultiBackendText mirrors internal/config.isMultiBackend for the on-disk rewrite: a provider is
// multi-backend iff it is the known built-in "pi" OR its block sets a non-empty provider_flag. opencode/agy
// route via the model slash-prefix WITHOUT provider_flag and never carried a v2 default_provider → not
// multi-backend here. (FR-B7 "single-backend untouched".)
func isMultiBackendText(name, providerFlag string) bool {
	return name == "pi" || providerFlag != ""
}

// replaceQuotedValue returns line with its FIRST double-quoted string replaced by newVal (preserves the key,
// spacing, and any trailing inline comment). For our constrained keys (model/default_model) the first quoted
// string IS the value.
func replaceQuotedValue(line, newVal string) string {
	loc := regexp.MustCompile(`"[^"]*"`).FindStringIndex(line)
	if loc == nil {
		return line
	}
	return line[:loc[0]] + `"` + newVal + `"` + line[loc[1]:]
}

// commentOutWithNote returns line prefixed with "# " and a trailing note (FR-B5 "commenting out removed/renamed
// keys with a note"). The line is no longer ACTIVE TOML but remains auditable/reversible.
func commentOutWithNote(line, note string) string {
	return "# " + line + "  # " + note
}
```

```go
// === internal/cmd/config.go — REWRITE configUpgradeCmd.Long (Mode A) ===
//   Replace the existing Long (L95-106) with comprehensive text covering the on-disk →v3 rewrite AND the
//   in-memory auto-migration (S1). S2 owns this final text (it implements the on-disk behavior).

	Long: `Rewrite an existing Stagecoach config file in place so its config_version matches this binary's
current schema version (` + fmt.Sprintf("`config_version = %d`", config.CurrentConfigVersion) + `).

For files older than v3 this is more than a version bump: the removed ` + "`default_provider`" + ` field is
folded into a slash-PREFIX on the affected ` + "`model`" + ` values for multi-backend providers (e.g.
` + "`model = \"glm-5.2\"`" + ` + ` + "`default_provider = \"zai\"`" + ` becomes ` + "`model = \"zai/glm-5.2\"`" + `),
the ` + "`default_provider`" + ` line is commented out with a note, and any abandoned ` + "`[agent.*]`" + `
tables are renamed to ` + "`[provider.*]`" + `. Single-backend providers are left alone (their
default_provider, if any, is just commented out). Every other line (your values, comments, ordering) is
preserved. No value is invented: a bare model with no resolvable prefix stays bare.

Loading an OLDER config also auto-migrates it IN MEMORY with a one-time deprecation notice, so the tool works
immediately — ` + "`config upgrade`" + ` persists that migration to the file so the notice stops.

Running it twice is safe: a file already at the current version is left unchanged ("already up to date").

This targets the file reported by ` + "`stagecoach config path`" + ` — by default the GLOBAL config, but the
--config flag and STAGECOACH_CONFIG env var ARE honored. If no config file exists, run
` + "`stagecoach config init`" + ` first. If the file is not valid TOML, it is left untouched and an error is
printed.`,
```

```go
// === internal/cmd/config_test.go — ADDITIVE tests (package cmd white-box) ===

// TestUpgradeConfigVersion_V3Rewrite exercises the on-disk →v3 rewrite via the pure function (target=3).
// Sub-tests pin each FR-B7 guarantee: fold (provider default_model / global model / per-role model / role
// inheriting global), single-backend untouched, comment-out, agent rename, idempotency, no-invent, version bump.
func TestUpgradeConfigVersion_V3Rewrite(t *testing.T) {
	t.Run("folds provider default_model + comments out default_provider + bumps version", func(t *testing.T) {
		input := "config_version = 2\n" +
			"\n" +
			"[provider.pi]\n" +
			"default_provider = \"zai\"\n" +
			"default_model = \"glm-5.2\"\n" +
			"provider_flag = \"--provider\"\n"
		got, changed := upgradeConfigVersion(input, 3)
		if !changed {
			t.Fatal("changed=false, want true (v2 → v3 rewrite)")
		}
		if !strings.Contains(got, "default_model = \"zai/glm-5.2\"") {
			t.Errorf("default_model not folded:\n%s", got)
		}
		if strings.Contains(got, "\ndefault_provider = \"zai\"\n") {
			t.Errorf("default_provider still active (should be commented out):\n%s", got)
		}
		if !strings.Contains(got, "# default_provider = \"zai\"") {
			t.Errorf("default_provider not commented out with note:\n%s", got)
		}
		if !strings.HasPrefix(got, "config_version = 3\n") {
			t.Errorf("config_version not bumped to 3:\n%s", got)
		}
		// The result must be valid TOML.
		var m map[string]any
		if err := toml.Unmarshal([]byte(got), &m); err != nil {
			t.Fatalf("upgraded output is not valid TOML: %v\n%s", err, got)
		}
	})

	t.Run("folds global [defaults] model when global provider is multi-backend", func(t *testing.T) {
		input := "config_version = 2\n" +
			"[defaults]\n" +
			"provider = \"pi\"\n" +
			"model = \"glm-5.2\"\n" +
			"\n" +
			"[provider.pi]\n" +
			"default_provider = \"zai\"\n"
		got, _ := upgradeConfigVersion(input, 3)
		if !strings.Contains(got, "model = \"zai/glm-5.2\"") {
			t.Errorf("global model not folded:\n%s", got)
		}
	})

	t.Run("folds per-role model (explicit role provider) and role inheriting global", func(t *testing.T) {
		input := "config_version = 2\n" +
			"[defaults]\n" +
			"provider = \"pi\"\n" +
			"\n" +
			"[role.planner]\n" +
			"provider = \"pi\"\n" +
			"model = \"glm-5.2\"\n" +
			"\n" +
			"[role.message]\n" + // no provider → inherits global pi
			"model = \"glm-5.2\"\n" +
			"\n" +
			"[provider.pi]\n" +
			"default_provider = \"zai\"\n"
		got, _ := upgradeConfigVersion(input, 3)
		// Both role models must be prefixed (one explicit provider, one inherited).
		if c := strings.Count(got, "model = \"zai/glm-5.2\""); c != 2 {
			t.Errorf("expected 2 folded role models, got %d:\n%s", c, got)
		}
	})

	t.Run("single-backend provider: default_provider commented out, model NOT prefixed", func(t *testing.T) {
		input := "config_version = 2\n" +
			"[provider.claude]\n" +
			"default_provider = \"anthropic\"\n" +
			"default_model = \"opus\"\n"
		got, _ := upgradeConfigVersion(input, 3)
		if !strings.Contains(got, "# default_provider = \"anthropic\"") {
			t.Errorf("single-backend default_provider not commented out:\n%s", got)
		}
		if strings.Contains(got, "\"anthropic/opus\"") {
			t.Errorf("single-backend model must NOT be prefixed:\n%s", got)
		}
		if !strings.Contains(got, "default_model = \"opus\"") {
			t.Errorf("single-backend default_model must be unchanged:\n%s", got)
		}
	})

	t.Run("agent table header renamed to provider", func(t *testing.T) {
		input := "config_version = 2\n" +
			"[agent.pi]\n" +
			"default_provider = \"zai\"\n" +
			"default_model = \"glm-5.2\"\n"
		got, _ := upgradeConfigVersion(input, 3)
		if strings.Contains(got, "[agent.pi]") {
			t.Errorf("[agent.pi] not renamed:\n%s", got)
		}
		if !strings.Contains(got, "[provider.pi]") {
			t.Errorf("missing [provider.pi]:\n%s", got)
		}
		if !strings.Contains(got, "default_model = \"zai/glm-5.2\"") {
			t.Errorf("default_model not folded after rename:\n%s", got)
		}
	})

	t.Run("idempotent: a v3 file is a no-op", func(t *testing.T) {
		v3 := "config_version = 3\n[provider.pi]\ndefault_model = \"zai/glm-5.2\"\n"
		got, changed := upgradeConfigVersion(v3, 3)
		if changed {
			t.Errorf("a v3 file must be a no-op; got changed=true:\n%s", got)
		}
		if got != v3 {
			t.Errorf("a v3 file must be byte-unchanged; got:\n%s", got)
		}
	})

	t.Run("bare pi model with NO default_provider stays bare (no-invent)", func(t *testing.T) {
		input := "config_version = 2\n[provider.pi]\ndefault_model = \"glm-5.2\"\n"
		got, _ := upgradeConfigVersion(input, 3)
		if !strings.Contains(got, "default_model = \"glm-5.2\"") {
			t.Errorf("a bare model with no default_provider must stay bare:\n%s", got)
		}
		if strings.Contains(got, "/glm-5.2\"") {
			t.Errorf("a prefix was invented (no default_provider to fold):\n%s", got)
		}
	})
}

// TestConfigUpgrade_V2ToV3Rewrite is the COMMAND round-trip: write a v2 file, run `config upgrade`, assert the
// on-disk result (prefixed model + commented default_provider + config_version=3); re-run → no change.
// Mirrors the TestConfigUpgrade_AddsVersion harness (temp home + writeConfigFile + Execute).
func TestConfigUpgrade_V2ToV3Rewrite(t *testing.T) {
	home, _, globalDir := chdirTempHome(t) // the existing helper used by TestConfigUpgrade_AddsVersion
	_ = home
	globalPath := filepath.Join(globalDir, "config.toml")
	v2 := "config_version = 2\n" +
		"\n" +
		"[defaults]\n" +
		"provider = \"pi\"\n" +
		"model = \"glm-5.2\"\n" +
		"\n" +
		"[provider.pi]\n" +
		"default_provider = \"zai\"\n" +
		"provider_flag = \"--provider\"\n"
	writeConfigFile(t, globalDir, "config.toml", v2)

	bin := stubtest.Build(t)
	out, err := stubtest.Run(bin, []string{"config", "upgrade"})
	if err != nil {
		t.Fatalf("config upgrade failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Upgraded config") {
		t.Errorf("expected upgrade confirmation; got:\n%s", out)
	}

	data, rerr := os.ReadFile(globalPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	upgraded := string(data)
	if !strings.Contains(upgraded, "model = \"zai/glm-5.2\"") {
		t.Errorf("on-disk global model not folded:\n%s", upgraded)
	}
	if !strings.Contains(upgraded, "# default_provider = \"zai\"") {
		t.Errorf("on-disk default_provider not commented out:\n%s", upgraded)
	}
	if !strings.Contains(upgraded, "config_version = 3") {
		t.Errorf("on-disk config_version not 3:\n%s", upgraded)
	}

	// Re-run → no change (idempotent).
	out2, err2 := stubtest.Run(bin, []string{"config", "upgrade"})
	if err2 != nil {
		t.Fatalf("second config upgrade failed: %v\n%s", err2, out2)
	}
	if !strings.Contains(out2, "already at version 3") && !strings.Contains(out2, "no changes") {
		t.Errorf("second run should be a no-op; got:\n%s", out2)
	}
	data2, _ := os.ReadFile(globalPath)
	if string(data2) != upgraded {
		t.Errorf("second run changed the file (not idempotent)")
	}
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: internal/cmd/config.go — ADD pkg-level regexes (agentHeaderRe, tableHeaderRe, kvStringRe)
  - ADD near configVersionLineRe (L117). kvStringRe MUST anchor on [A-Za-z_] so comment lines don't match.
  - GOTCHA: reuse configVersionLineRe/isTableHeader/leadingHeaderEnd (already present).

Task 2: internal/cmd/config.go — REFACTOR upgradeConfigVersion into the GATED form + extract helpers
  - REPLACE upgradeConfigVersion (L178-204) with the gated version (cur>=version→no-op; version>=3&&cur<3→
      rewriteV2ToV3+setConfigVersionLine; else→setConfigVersionLine).
  - EXTRACT parseTopLevelConfigVersion + setConfigVersionLine (pure) from the old body.
  - GOTCHA: the existing `TestUpgradeConfigVersion_*` call this with `config.CurrentConfigVersion` (=3 after
      S1) and CLEAN inputs (no default_provider). They pass UNCHANGED because `rewriteV2ToV3` is a no-op on
      clean inputs + `cur>=version`→no-op. Verify both by running them.

Task 3: internal/cmd/config.go — ADD rewriteV2ToV3 + helpers
  - IMPLEMENT the 3-pass textual rewrite (agent rename → collect → emit) + tableSection + isMultiBackendText +
      replaceQuotedValue + commentOutWithNote, per the Blueprint.
  - GOTCHA: provider-block model key = default_model; defaults/role = model. Multi-backend = pi OR provider_flag.
      Comment-out (not delete) default_provider. Bare-check on every fold.

Task 4: internal/cmd/config.go — REWRITE configUpgradeCmd.Long (Mode A)
  - REPLACE the Long (L95-106) with the comprehensive text (on-disk →v3 rewrite + in-memory auto-migration +
      idempotent). S2 owns the final text.

Task 5: internal/cmd/config_test.go — ADD rewrite-behavior tests (ADDITIVE)
  - ADD TestUpgradeConfigVersion_V3Rewrite (sub-tests: fold provider/global/role/inherited; single-backend;
      agent rename; idempotent; no-invent; version bump; valid-TOML output).
  - ADD TestConfigUpgrade_V2ToV3Rewrite (command round-trip + idempotent re-run). Mirror the
      TestConfigUpgrade_AddsVersion harness (chdirTempHome + writeConfigFile + stubtest.Build/Run).
  - GOTCHA: do NOT edit existing upgrade tests (S1 owns their version literals; the gate preserves behavior).

Task 6: VERIFY (no further edits)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. internal/config/* +
      default_action_test.go + every non-target file byte-unchanged. Existing TestUpgradeConfigVersion_* (v=2)
      + TestConfigUpgrade_* pass UNCHANGED. If TestConfigUpgrade_AlreadyCurrent fails because S1 left its input
      at "config_version = 2", update that ONE input to config.CurrentConfigVersion (§5). `go test ./...` green.
```

### Implementation Patterns & Key Details

```go
// THE GATE — preserves every existing unit test (version=2 → old behavior; cur>=version → no-op):
func upgradeConfigVersion(content string, version int) (string, bool) {
	cur := parseTopLevelConfigVersion(content)
	if cur >= version {
		return content, false
	}
	if version >= 3 && cur < 3 {
		return setConfigVersionLine(rewriteV2ToV3(content), version)
	}
	return setConfigVersionLine(content, version) // target < 3 → OLD behavior (byte-identical)
}

// THE fold (bare-check + providerPrefix + correct key per section):
if key == "default_model" { // inside [provider.<name>]
	if x, ok := providerPrefix[name]; ok && val != "" && !strings.Contains(val, "/") {
		lines[i] = replaceQuotedValue(line, x+"/"+val)
	}
}
// inside [defaults] / [role.<r>] the key is "model"; effective provider for a role = roleProvider[r] or global.

// THE comment-out (FR-B5 — not hard-delete):
if key == "default_provider" {
	lines[i] = commentOutWithNote(line, "v3 (FR-B7): removed — inference backend is now a slash-prefix on model")
}

// THE multi-backend classify (from text, mirrors internal/config.isMultiBackend):
func isMultiBackendText(name, providerFlag string) bool { return name == "pi" || providerFlag != "" }
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. cmd/config.go already imports regexp/strconv/strings/toml. The rewrite
      adds pkg-level regexes + unexported helpers; no new import. `go mod tidy` is a no-op.

PACKAGE EDGES: NONE added. cmd already imports config + provider (unchanged). rewriteV2ToV3 classifies
      multi-backend LOCALLY (does NOT consult the Manifest) so it stays a pure textual transform.

UPSTREAM (consume, do NOT edit):
  - S1: config.CurrentConfigVersion == 3 (the command's target version) + the in-memory migrate.go (the struct
        twin — same FR-B7 mapping). S2's isMultiBackendText mirrors S1's isMultiBackend so the two agree.

DOWNSTREAM (consumers — not this task):
  - `runConfigUpgrade` calls upgradeConfigVersion(data, config.CurrentConfigVersion) — UNCHANGED (the gate is
        inside upgradeConfigVersion). Its print messages ("Upgraded … to version N" / "already at version N")
        stay accurate.
  - config.Load (S1) auto-migrates in memory; after `config upgrade` the file is v3-native → no notice.

FROZEN/LEAVE (do NOT edit):
  - internal/config/* (S1: config.go CurrentConfigVersion, migrate.go, load.go, migrate_test.go, the version-
        literal sweep in config_test.go/default_action_test.go).
  - internal/cmd/{root,default_action,providers}.go, internal/cmd/default_action_test.go.
  - internal/provider/*, internal/decompose/*, internal/generate/*, internal/git/*, internal/prompt/*, pkg/*.
  - PRD.md, go.mod, Makefile, providers/*.toml, docs/*.
  - configVersionLineRe, isTableHeader, leadingHeaderEnd (REUSE — do not redefine).

NO NEW DATABASE / ROUTES / CLI COMMANDS / FILES.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/cmd/config.go internal/cmd/config_test.go
go vet ./internal/cmd/
# Confirm no new import (regexp/strconv/strings/toml already present):
grep -A8 '^import (' internal/cmd/config.go | head -12
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; go.mod/go.sum byte-unchanged; the new regexes/helpers present.
```

### Level 2: cmd-package unit tests (the new rewrite tests + NO regression)

```bash
go test ./internal/cmd/ -v -run 'TestUpgradeConfigVersion_V3Rewrite|TestConfigUpgrade_V2ToV3Rewrite'
# Expected PASS — every sub-test (fold provider/global/role/inherited; single-backend; agent rename;
# idempotent; no-invent; version bump; valid TOML; command round-trip + idempotent re-run).
go test ./internal/cmd/ -v -run 'TestUpgradeConfigVersion|TestConfigUpgrade'
# Expected: the EXISTING TestUpgradeConfigVersion_* (which pass config.CurrentConfigVersion = 3) + TestConfigUpgrade_*
# pass UNCHANGED (the gate's cur>=version no-op + rewriteV2ToV3's no-op-on-clean-inputs preserve them). If TestConfigUpgrade_AlreadyCurrent fails, S1 left its input at "config_version = 2" — update
# that INPUT to config.CurrentConfigVersion (§5).
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...     # Expect clean.
go test ./...      # Expect all PASS. (S1's in-memory migration + version-literal sweep must already be green;
                   #  S2 is additive.)
# Confirm frozen files byte-unchanged:
git diff --exit-code internal/config internal/cmd/default_action.go internal/cmd/default_action_test.go \
  internal/cmd/root.go internal/cmd/providers.go internal/provider internal/decompose internal/generate \
  internal/git internal/prompt pkg go.mod go.sum PRD.md && echo "frozen files UNCHANGED (expected)"
# Confirm ONLY the two target files changed:
git diff --name-only | grep -E 'internal/cmd/config\.go|internal/cmd/config_test\.go' \
  && echo "target files modified (expected)"
```

### Level 4: Rewrite-correctness reasoning (no runtime beyond the command)

```bash
# The rewrite is a pure text transform; the command round-trip is the integration proof. Verify by reasoning +
# the tests:
#   1. A v2 file ([provider.pi] default_provider="zai" + [defaults] model="glm-5.2", provider="pi") upgraded →
#      model="zai/glm-5.2", default_provider commented out, config_version=3, valid TOML. (V3Rewrite sub-tests)
#   2. Single-backend (claude) default_provider → commented out, model NOT prefixed. (FR-B7 "single-backend
#      untouched".)
#   3. Bare pi model + NO default_provider → stays bare (FR-B7 "no value invented").
#   4. [agent.*] → [provider.*], then the fold applies. (FR-B7 "map first".)
#   5. Idempotent: a v3 file re-upgraded → no-op (cur>=version); the fold's bare-check skips prefixed models.
#   6. The gate: target=2 → old behavior (existing unit tests unchanged); target=3 && cur<3 → rewrite.
# (No Level-4 commands beyond Levels 1–3 — the tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l internal/cmd/` clean.
- [ ] `go test ./...` GREEN (the new rewrite tests + the existing upgrade tests UNCHANGED + S1's suite green).
- [ ] go.mod/go.sum byte-unchanged; no new import (regexp/strconv/strings/toml already present).
- [ ] internal/config/* + default_action_test.go + every non-target file byte-unchanged.

### Feature Validation
- [ ] `upgradeConfigVersion` is GATED (cur>=version→no-op; version>=3&&cur<3→rewriteV2ToV3+setConfigVersionLine;
      else→old setConfigVersionLine).
- [ ] `rewriteV2ToV3` folds default_provider into default_model/model (multi-backend only; pi OR provider_flag);
      comments out EVERY default_provider with a note; renames [agent.*]→[provider.*]; bare-check; preserves all
      other lines; produces valid TOML.
- [ ] `configUpgradeCmd.Long` describes the →v3 rewrite + in-memory auto-migration (Mode A).
- [ ] `TestUpgradeConfigVersion_V3Rewrite` + `TestConfigUpgrade_V2ToV3Rewrite` pass; existing upgrade tests pass
      UNCHANGED.

### Code Quality Validation
- [ ] Follows conventions: PURE textual helpers (mirrors the existing upgradeConfigVersion purity); white-box
      `package cmd` tests; reuses configVersionLineRe/isTableHeader/leadingHeaderEnd; doc comments cite FR-B5/B7.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (LEAVE files untouched).

### Documentation
- [ ] `upgradeConfigVersion`/`rewriteV2ToV3` doc comments document the gate, the on-disk FR-B7 rewrite, the
      default_model key, the multi-backend classify, and the comment-out-with-note rationale. `configUpgradeCmd.
      Long` is the Mode-A user-facing description.
- [ ] No docs/*.md edits (changeset doc sync is P4.M2.T1).

---

## Anti-Patterns to Avoid

- ❌ **Don't drop the GATE / don't let `rewriteV2ToV3` touch clean inputs.** The existing `TestUpgradeConfigVersion_*`
      call `upgradeConfigVersion(input, config.CurrentConfigVersion)` (=3) with CLEAN inputs (no default_provider).
      They pass UNCHANGED only because (a) `cur >= version → no-op` and (b) `rewriteV2ToV3` is a no-op on clean
      inputs. If `rewriteV2ToV3` accidentally transforms a default_provider-free input, those tests break AND S1/S2
      collide on config_test.go. Keep `version >= 3 && cur < 3` AND ensure rewriteV2ToV3 folds/comments ONLY when
      a default_provider/[agent.*] is actually present. (design-decisions §1)
- ❌ **Don't re-marshal via go-toml.** It drops comments and reorders keys — FR-B5 requires comment-out-with-note
      + preserve-user-values. Use the surgical line edit (rewriteV2ToV3). (§2)
- ❌ **Don't fold `model` inside a provider block / `default_model` inside [defaults].** Provider-block key =
      `default_model`; defaults/role key = `model`. (scout §(f); §4)
- ❌ **Don't fold single-backend providers.** Only pi OR a provider with a non-empty provider_flag folds. A
      single-backend default_provider is commented out, its model NOT prefixed. (FR-B7; §3)
- ❌ **Don't hard-delete `default_provider`.** Comment it out with a note (FR-B5 authoritative). (§6)
- ❌ **Don't invent a prefix.** Fold only when default_provider is non-empty AND the model is non-empty and bare.
      A bare model with no default_provider STAYS bare. (FR-B7 "no value invented"; §7)
- ❌ **Don't edit the existing upgrade tests' assertions.** S1 owns the `config_version = 2`→`3` literals; the
      gate preserves the behavior. S2 ADDS new test functions. (§0/§5)
- ❌ **Don't touch internal/config/* or default_action_test.go.** Those are S1's in-memory-migration domain
      (CurrentConfigVersion bump, migrate.go, the version-literal sweep, the default-action fixture). (§0)
- ❌ **Don't consult/import the Manifest in the textual rewrite.** Classify multi-backend locally
      (`isMultiBackendText`: pi OR provider_flag) so the rewrite stays a pure text transform and matches S1. (§3)
- ❌ **Don't redefine configVersionLineRe/isTableHeader/leadingHeaderEnd.** Reuse them. (Blueprint)
- ❌ **Don't make the rewrite non-idempotent.** `cur >= version → no-op` + the bare-check skip + the commented-
      out-key non-match together guarantee `config upgrade` is safe to run repeatedly. (§7)

---

## Confidence Score

**9/10** — A well-bounded textual transform + a gate, with the FR-B7 mapping pinned verbatim (fold default_provider
into the model slash-prefix for multi-backend, global + per-role + the provider's own default_model; comment out
the removed key; rename [agent.*]; single-backend untouched; no-invent; idempotent). The two subtle traps are
defused by research: (1) the GATE + `rewriteV2ToV3`'s no-op-on-clean-inputs (§1, verified for version=3 — the
existing tests call `upgradeConfigVersion(input, config.CurrentConfigVersion)`) make every existing
`TestUpgradeConfigVersion_*` pass UNCHANGED, so S2 is purely additive for tests and cannot collide with S1's
parallel version-literal sweep; (2) on-disk is TEXTUAL (line-based), not struct re-marshal, because FR-B5
mandates comment-out-with-note + preserve-user-values (go-toml re-marshal would violate both). The `rewriteV2ToV3`
code is copy-ready (3 passes + helpers), the multi-backend classify mirrors S1's `isMultiBackend` (pi OR
provider_flag) so the two migrations agree, and the `default_model`-vs-`model` key distinction is explicit. The
one residual risk is the `TestConfigUpgrade_AlreadyCurrent` premise (§5) — if S1 left its input at
`config_version = 2`, S2 updates that one INPUT to `config.CurrentConfigVersion`; this is a one-line,
run-to-confirm fix. The -1 reserves for a regex edge case (e.g. single-quoted values or unusual inline comments
in a user file) the tests don't cover; the double-quoted-value assumption is documented and matches the
bootstrap/examples.
