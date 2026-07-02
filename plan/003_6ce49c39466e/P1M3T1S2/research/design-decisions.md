# P1.M3.T1.S2 — Design Decisions (`config upgrade` on-disk →v3 rewrite + test fixture updates)

Ground truth read before writing this note:
- **PRD §9.17 FR-B5/B7** (h3.33 — AUTHORITATIVE): `config upgrade` rewrites in place, preserving user
  values, commenting out removed/renamed keys with a note, idempotent; the →v3 rewrite folds
  `default_provider` into the model slash-prefix for multi-backend providers (global + per-role + the
  provider's own `default_model`), removes `default_provider`, maps `agent/[agent.*]`→`provider/[provider.*]`
  first, single-backend untouched, NO value invented.
- **PRD §16.2** (h3.67 — the v3 file shape: `model = "zai/glm-5.2"`, no `default_provider`).
- **plan/003_6ce49c39466e/P1M3T1S1/PRP.md** — the S1 CONTRACT (in-memory migration). S1 owns: `CurrentConfigVersion`→3,
  `migrate.go` (in-memory `migrateV2ToV3` on `*Config`), `Load` wiring, `migrate_test.go`, and the version-
  literal test-fixture sweep (its §6 breakage map). S2 is the ON-DISK half.
- **internal/cmd/config.go** (read in FULL) — `runConfigUpgrade` (L140: read→validate TOML→
  `upgradeConfigVersion(string(data), config.CurrentConfigVersion)`→write), `upgradeConfigVersion` (L178-204:
  PURE TEXTUAL version-bump — scans top-level for `config_version`, rewrites/inserts ONE line; touches
  nothing else), `configVersionLineRe` (L117), `isTableHeader`/`leadingHeaderEnd` helpers. The cmd package
  ALREADY imports regexp/strconv/strings/toml/cobra/config/provider/exitcode.
- **internal/cmd/config_test.go** (grep'd all `config_version` + upgrade test funcs) — `TestUpgradeConfigVersion_*`
  (L808-916, PURE unit tests calling `upgradeConfigVersion(input, config.CurrentConfigVersion)` — verified
  during this research; S1's parallel sweep converted any literal `2`);
  `TestConfigUpgrade_*` (L951-1076, COMMAND tests using the real `CurrentConfigVersion`); many `config_version = 2`
  OUTPUT assertions (bootstrap/init/upgrade — S1's §6 bump-breakage) + INPUT v2 fixtures.
- **internal/cmd/default_action_test.go:1203** — a v2 INPUT fixture (`config_version = 2` + `[provider.pi]
  default_provider = "openrouter"`) fed to the DEFAULT ACTION (not `config upgrade`). S1's IN-MEMORY migration
  affects it; S2's on-disk upgrade does NOT.
- **architecture/scout_config_model.md §(d)/(§f)** — the authoritative touchpoint map: `upgradeConfigVersion`
  (L178-204) "is the future extension point — v3 migration needs more than a version bump"; `default_provider`
  flows as RAW `any` in `Config.Providers`; the manifest field is `default_model` (NOT `model`) in provider blocks.

---

## §0 — Scope & the S1/S2 split (CONFLICT-FREE)

**S2 owns (this subtask):**
1. `internal/cmd/config.go` — extend `upgradeConfigVersion` (L178) with the on-disk →v3 TEXTUAL rewrite; rewrite
   the `config upgrade` `Long` help text (Mode A).
2. `internal/cmd/config_test.go` — ADD new rewrite-behavior tests (pure unit + command round-trip). ADDITIVE.

**S2 does NOT touch (S1 / other domains — FROZEN):**
- `internal/config/*` — S1 owns the bump (`CurrentConfigVersion`→3), the in-memory `migrate.go`, `Load` wiring.
- `internal/cmd/default_action_test.go` — the v2 fixture at L1203 is exercised by the DEFAULT ACTION, whose
  behavior changes via S1's IN-MEMORY migration (not S2's on-disk upgrade). S1's §6 owns it.
- The version-LITERAL test-fixture sweep (`config_version = 2` → `3` OUTPUT assertions across config_test.go) —
  S1's §6 breakage map owns it (it's the bump's mechanical breakage). S2 does NOT redo it.

**Why this split is conflict-free:** S2's code change is GATED (§1) and `rewriteV2ToV3` is a NO-OP on clean
inputs (no `default_provider`/`[agent.*]`), so every existing `TestUpgradeConfigVersion_*` test (which passes
`config.CurrentConfigVersion` = 3 after S1 — verified) keeps its exact old behavior → they pass UNCHANGED. The `TestConfigUpgrade_*`
command tests use the real `CurrentConfigVersion` (3 after S1); their v2 inputs have NO `default_provider`, so
S2's rewrite is version-only on them → their assertions hold once S1 has fixed the version literals (2→3). S2
therefore only ADDS tests; it edits no existing test's assertions. (§5 details the one premise to verify.)

---

## §1 — THE GATE: v3 rewrite only when `version >= 3 && cur < 3`; `cur >= version` → no-op

**Decision:** restructure `upgradeConfigVersion(content, version)`:
```
cur := parseTopLevelConfigVersion(content)          // 0 if missing/commented/in-table
if cur >= version { return content, false }          // idempotent (cur==version) OR ahead (cur>version): no-op
if version >= 3 && cur < 3 {
    out := rewriteV2ToV3(content)                    // agent rename + fold + comment-out (NO version change)
    return setConfigVersionLine(out, version)        // set/insert the config_version line
}
return setConfigVersionLine(content, version)        // OLD behavior: just the version line (target < 3)
```

**VERIFIED CURRENT STATE (re-checked at PRP-writing time):** the existing `TestUpgradeConfigVersion_*`
(L808-916) ALL call `upgradeConfigVersion(input, config.CurrentConfigVersion)` — i.e. they pass
`config.CurrentConfigVersion`, NOT a literal. After S1 lands that is **3**. (S1's parallel §6 sweep converted
any literal `2` to `config.CurrentConfigVersion`; `git status` showed config_test.go in flux during this
research.) So in S2's context the existing unit tests exercise **version=3**.

**Why the gate still preserves every existing unit test (verified for version=3):** two properties combine —
(a) `cur >= version → no-op` handles the "current" / idempotent cases; (b) `rewriteV2ToV3` is a **NO-OP on
inputs that have no `default_provider` and no `[agent.*]`** (Pass 1 finds no agent headers; Pass 2a builds an
empty `providerPrefix`; Pass 2b folds/comments nothing). The existing test inputs are CLEAN (`[defaults]
provider="pi"`, `[generation] …`, a commented/in-table config_version) — they contain NO `default_provider` —
so `rewriteV2ToV3` leaves them byte-identical and only `setConfigVersionLine` sets `config_version = 3`.
Verified case-by-case for version=3:
- `_NoVersion_Inserts(input, 3)`: cur=0<3 → rewriteV2ToV3 (no-op, clean input) + insert "config_version = 3". ✓
- `_OlderVersion_Updates("config_version=1", 3)`: cur=1<3 → rewriteV2ToV3 (no-op) + set line to 3. ✓
- `_CurrentVersion_NoChange("config_version=3", 3)`: S1 set the input to current(3) → cur=3>=3 → no-op, false. ✓
- `_CommentedVersionIgnored("# config_version=1", 3)`: cur=0 (commented) → rewriteV2ToV3 (no-op) + insert 3; commented line preserved. ✓
- `_VersionInTableNotMatched("[defaults]\nconfig_version=1", 3)`: cur=0 (in-table) → rewriteV2ToV3 (no-op) + insert top-level 3; in-table preserved. ✓
- `_Idempotent(input, 3)`: 1st → rewrite+set 3 (true); 2nd → cur=3>=3 → no-op (false). ✓

So S2 EDITS NONE of these. The load-bearing property is **rewriteV2ToV3 is a no-op on clean inputs** + the
**cur>=version no-op**. The `version >= 3 && cur < 3` condition ensures the v3 fold ONLY fires when upgrading
TO v3+ (never when a future/hypothetical caller targets <3); the trailing `else` (target<3) is forward-compat
defense, not test-exercised today.

`cur >= version → no-op` ALSO gives idempotency for the command path: after a v2→v3 upgrade the file is
`config_version = 3`; re-running `config upgrade` (target=3) → cur=3>=3 → no-op → "already up to date". (§4)

---

## §2 — On-disk = TEXTUAL line rewrite (NOT struct; preserves comments per FR-B5)

**Decision:** `rewriteV2ToV3` operates on RAW TOML TEXT (lines), NOT the decoded `*Config`. go-toml re-marshaling
is REJECTED because it drops comments/reorders keys — FR-B5 requires "commenting out removed/renamed keys with a
note" and "preserving user values … leave all other content unchanged". A line-based surgical edit preserves
every other line byte-for-byte. (S1's `migrateV2ToV3` is the in-memory STRUCT counterpart; S2 cannot reuse it —
different input domain. Both implement the SAME FR-B7 mapping.)

**The line-based algorithm** (3 passes over `strings.Split(content, "\n")`):
- **Pass 1 — agent→provider rename:** for each line that is a table header `[agent.<name>]` → `[provider.<name>]`
  (FR-B7 "map agent/[agent.*] → provider/[provider.*] first"). Header-only; values untouched.
- **Pass 2a — collect state:** scan, tracking the current table path (e.g. `provider.pi`, `defaults`,
  `role.planner`). Record:
  - `rawDP[name]` = the `[provider.<name>] default_provider` value X (if non-empty).
  - `providerFlag[name]` = the `[provider.<name>] provider_flag` value (if non-empty) — for multi-backend classify.
  - `globalProvider` = the `[defaults] provider` value.
  - `roleProvider[r]` = the `[role.<r>] provider` value.
  Then `providerPrefix[name] = X` ONLY where `isMultiBackendText(name, providerFlag[name])` (§3) — single-backend
  default_provider is NOT a prefix (FR-B7 "single-backend untouched").
- **Pass 2b — emit (fold + comment-out):** scan again, tracking the table path. For each `key = "val"` line:
  - `[provider.<name>] default_provider` → COMMENT OUT with a note (§6). (Removed in v3 regardless of backend.)
  - `[provider.<name>] default_model` (the raw model key — §4) → if `providerPrefix[name]` set and val bare → `X/val`.
  - `[defaults] model` → if `providerPrefix[globalProvider]` set and val bare → `X/val`.
  - `[role.<r>] model` → effective provider = `roleProvider[r]` or `globalProvider`; if `providerPrefix[ep]` set and val bare → `X/val`.
  - every other line → unchanged.

**Bare = `!strings.Contains(val, "/")`** (idempotent + no-invent: an already-prefixed `zai/x` is skipped; a bare
model with no resolvable prefix STAYS bare → FR-R5b error the user resolves). Matches S1's fold condition.

---

## §3 — Multi-backend classification ON DISK (mirror S1, from the text)

**Decision:** a provider is multi-backend (its `default_provider` is a meaningful inference backend to fold) iff
`name == "pi"` OR its `[provider.<name>]` block sets a non-empty `provider_flag`. This mirrors S1's
`isMultiBackend`/`v2MultiBackendBuiltins={"pi"}` + raw `provider_flag` — but sourced from the TEXT (Pass 2a
captures `provider_flag`) rather than the decoded map. The cmd package DOES import `internal/provider`, but
mirroring S1's local classify keeps the two migrations provably consistent and avoids depending on a manifest
lookup during a textual rewrite. opencode/agy use the model prefix WITHOUT provider_flag and never carried a v2
default_provider → they are single-backend here (no fold), matching S1.

---

## §4 — The raw provider model key is `default_model`, NOT `model`

**Decision:** inside `[provider.<name>]` blocks the model field is `default_model` (manifest tag
`DefaultModel toml:"default_model"`, scout §(f)). Inside `[defaults]` and `[role.<r>]` it is `model`. The fold
targets the RIGHT key per section. (S1's #2 gotcha is the same finding.) Do NOT look for `model` inside a
provider block, nor `default_model` inside `[defaults]`.

---

## §5 — Existing command tests pass under the gate (one premise to verify)

The `TestConfigUpgrade_*` command tests call `runConfigUpgrade` → `upgradeConfigVersion(data, CurrentConfigVersion=3)`.
Their v2 INPUT fixtures have NO `default_provider`, so `rewriteV2ToV3` is a no-op-except-version on them (no fold,
no comment-out) → only the `config_version` line changes (2→3). Thus:
- Their version-LITERAL OUTPUT assertions (e.g. L973, L1030 `config_version = 2`) break from S1's BUMP → **S1's
  §6 fixes them to 3** (S2 does not touch them).
- Their BEHAVIORAL assertions (file upgraded, valid TOML, idempotent) still hold under S2's code.

**The one premise to verify (not assume):** `TestConfigUpgrade_AlreadyCurrent` (L981). Its INPUT is
`config_version = 2`; under S1's bump "current" is 3, so a v2 file is NOT current. S1's §6 must have made it
genuinely "current" (input `config_version = 3`, or vs `CurrentConfigVersion`) to keep green. Under S2's gate,
`cur(3) >= version(3)` → no-op → "already up to date" → the test passes. **S2's job is only to VERIFY this test
passes** (run it); if S1 left the input at 2, S2 updates the INPUT to `config.CurrentConfigVersion` (the genuine
no-op case) — a one-line INPUT fix, not a version-literal OUTPUT fix. Either way S2 does not fight S1.

---

## §6 — COMMENT OUT `default_provider` (not hard-delete) — FR-B5

**Decision:** the rewrite comments out each `default_provider` line with a note rather than deleting it, per
FR-B5 ("commenting out removed/renamed keys with a note") — auditable, reversible, and matches "preserving user
values". Form: prefix the line with `# ` and append a brief note, e.g.
`# default_provider = "zai"  # v3 (FR-B7): removed — inference backend is now a slash-prefix on model`.
(The item contract's "(a) DELETE the default_provider line" is reconciled to comment-out-with-note per the
authoritative FR-B5; the key is no longer ACTIVE, which is what "removed" means operationally.) This applies to
EVERY `default_provider` in a provider block (multi- and single-backend) — the field is gone in v3.

---

## §7 — Idempotency

**Decision:** two layers. (1) `cur >= version → no-op` in `upgradeConfigVersion` — a v3 file (config_version=3)
re-run with target=3 returns unchanged ("already up to date"). (2) the fold itself is idempotent (bare-check
`!strings.Contains(val,"/")` skips already-prefixed models; a commented-out `default_provider` is a `#` line,
not re-processed by `parseKVString`). So even if `rewriteV2ToV3` were somehow re-run, it would not double-prefix.
Combined: `config upgrade` is safe to run any number of times. (FR-B5 "idempotent".)

---

## §8 — `config upgrade` Long help text (Mode A DOCS) — S2 owns the FINAL text

**Decision:** S2 rewrites the `configUpgradeCmd.Long` (cmd/config.go:95-106) to describe the on-disk →v3 rewrite:
it now folds `default_provider` into the model slash-prefix (multi-backend), renames `[agent.*]`→`[provider.*]`,
comments out removed keys with a note, and bumps `config_version` — in addition to noting that loading an older
config auto-migrates IN MEMORY (S1). S1's provisional Long touch (its §7) is superseded by S2's comprehensive
text (S2 implements the on-disk behavior, so S2 is authoritative on the command's help). Provide the exact final
text in the Blueprint.

---

## §9 — No new imports; go.mod UNCHANGED

**Decision:** cmd/config.go ALREADY imports regexp/strconv/strings/toml (+ cobra/config/provider/exitcode). The
rewrite uses regexp (new patterns for agent header / kv string / value replace) + strings + strconv (already
there). No new import; `go mod tidy` is a no-op; `git diff --exit-code go.mod go.sum` empty.

---

## §10 — Test strategy (ADDITIVE: new pure-unit + new command round-trip)

**Decision:** add to `internal/cmd/config_test.go` (`package cmd` white-box):
1. **`TestUpgradeConfigVersion_V3Rewrite_*`** (PURE unit, `upgradeConfigVersion(input, 3)`):
   - Folds `[provider.pi] default_provider="zai"` + `default_model="glm-5.2"` → `default_model="zai/glm-5.2"`;
     `default_provider` commented out with note; `config_version` set to 3.
   - Folds `[defaults] model` (global provider=pi) and `[role.planner] model` (role provider=pi) → prefixed.
   - Role inheriting global pi (`[role.message]` no provider) → prefixed.
   - Single-backend claude: `default_provider` commented out, model NOT prefixed.
   - Idempotent: `upgradeConfigVersion(v3output, 3)` → no-op (cur=3>=3).
   - Bare pi model with NO default_provider → stays bare (no-invent).
   - `[agent.foo]` → `[provider.foo]` rename (then fold applies).
   - v2 input with config_version=2 → cur=2<3 → rewrite fires; output config_version=3.
2. **`TestConfigUpgrade_V2ToV3Rewrite`** (COMMAND round-trip): write a v2 file with `[provider.pi]
   default_provider="zai"` + `[defaults] model="glm-5.2"`, run `config upgrade`, assert the on-disk file now has
   `model = "zai/glm-5.2"`, the `default_provider` line is commented, and `config_version = 3`; re-run → no change.
3. **VERIFY** (not new tests): the existing `TestUpgradeConfigVersion_*` (which pass `config.CurrentConfigVersion`
   = 3) and `TestConfigUpgrade_*` still pass under S2's gated code (they do — §1/§5: the gate's `cur>=version`
   no-op + rewriteV2ToV3's no-op-on-clean-inputs preserve them). Run them; fix ONLY `TestConfigUpgrade_AlreadyCurrent`'s
   INPUT if S1 left it at 2 (§5).

Pure unit tests build input STRINGS directly (no filesystem); the command round-trip uses the existing
`writeConfigFile`/temp-home harness pattern (see TestConfigUpgrade_AddsVersion L951).

---

## Summary table (the 10 calls at a glance)

| § | Decision | Source |
|---|----------|--------|
| 0 | S2 = upgradeConfigVersion rewrite + Long text + ADDITIVE tests; config/* + default_action_test.go + version-literal sweep = S1 | item contract, S1 §6 |
| 1 | GATE: rewrite only when target>=3 && cur<3; cur>=target→no-op | preserves existing unit tests (verified) |
| 2 | On-disk = TEXTUAL line rewrite (3 passes); reject go-toml re-marshal (drops comments) | FR-B5 |
| 3 | Multi-backend = pi OR raw provider_flag (mirror S1, from text) | FR-B7, S1 §1 |
| 4 | Provider-block model key = `default_model`; defaults/role = `model` | scout §(f), S1 §2 |
| 5 | Existing command tests pass under the gate; verify `AlreadyCurrent` (S1 may have set input→3) | §1 gate |
| 6 | COMMENT OUT default_provider with a note (FR-B5), not hard-delete | FR-B5 |
| 7 | Idempotent: cur>=version no-op + bare-check skip | FR-B5 |
| 8 | S2 owns the final `config upgrade` Long text (supersedes S1's provisional touch) | item DOCS, S1 §7 |
| 9 | No new imports; go.mod UNCHANGED | cmd/config.go imports |
| 10 | ADDITIVE tests: V3Rewrite pure-unit + command round-trip; verify existing pass | §1/§5 |
