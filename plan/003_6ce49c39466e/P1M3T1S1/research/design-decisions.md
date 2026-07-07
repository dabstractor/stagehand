# Design Decisions — P1.M3.T1.S1 (CurrentConfigVersion → 3 + in-memory v3 migration, FR-B7)

> Companion to `architecture/scout_config_model.md` §(d)/(f). Each § is a non-obvious call; each cites
> evidence gathered from the live v3 tree (2026-07-01). Numbered for cross-reference from the PRP.

## §0 — Scope: S1 = version bump + IN-MEMORY migration + notice; S2 = on-disk upgrade rewrite

- **S1** bumps `CurrentConfigVersion` 2→3 (`config.go:18`), adds the in-memory `migrateV2ToV3` (+ notice),
  wires it into `Load`, and updates every test broken by the bump+migration so `go test ./...` stays green.
- **S2** (P1.M3.T1.S2) owns the on-disk `config upgrade` →v3 REWRITE: extending `upgradeConfigVersion`
  (`cmd/config.go:178`) / `runConfigUpgrade` to fold `default_provider` into `model` in the FILE and drop
  the key (today upgrade only bumps the version LINE), plus the NEW rewrite-behavior tests.
- The `[agent.*]`→`[provider.*]` TEXTUAL rewrite is S2 (it operates on raw TOML text; see §3).

## §1 — Multi-backend is classified WITHOUT importing internal/provider (decoupling invariant)

The migration must "prepend default_provider to model for each MULTI-BACKEND provider (manifest has
provider_flag set)". But `config` deliberately does NOT import `internal/provider` (the raw-map
decoupling invariant — `Config.Providers` is `map[string]map[string]any` precisely so config need not
know the Manifest type; `file_test.go` etc. enforce this). Verified: `grep stagecoach/internal
internal/provider/` is empty ⇒ no cycle today, but importing provider would regress the deliberate
layering. So multi-backend is determined LOCALLY:

- **`v2MultiBackendBuiltins = {"pi"}`** — the v2 built-ins whose manifests carried a `default_provider`
  (non-empty `provider_flag`). In the v3 tree ONLY `builtinPi()` has `ProviderFlag: strPtr("--provider")`
  (non-empty); claude/gemini/agy/opencode/codex/cursor are all `strPtr("")` (`builtin.go`). opencode/agy
  route their inference backend via the model slash-prefix WITHOUT a provider_flag and never carried a
  default_provider in v2 (`builtin.go` opencode comment: "provider is part of the model string"). So the
  set of v2 providers that could have a meaningful `default_provider` is exactly `{pi}`.
- **`isMultiBackend(name, raw)`** also accepts a USER-DEFINED provider whose raw map sets a non-empty
  `"provider_flag"` (a custom multi-backend provider). This covers `[provider.myagent] provider_flag =
  "--backend"` + `default_provider = "X"`.
- This is a MIGRATION SHIM (transitional). If a future built-in gains a provider_flag, add its name to
  `v2MultiBackendBuiltins`. The set is tiny, stable, and local.

## §2 — The fold keys off `default_provider` PRESENCE; idempotent; invents nothing (FR-B7)

The migration reads `cfg.Providers[name]["default_provider"]` (a raw `any`, decoded by go-toml to a Go
`string`). For each multi-backend provider with a NON-EMPTY `default_provider = "X"`:
- prepend `X/` to its model wherever it appears: global `Config.Model` (if `cfg.Provider == name`),
  each `Config.Roles[r].Model` (effective provider = role override or global), and the raw
  `cfg.Providers[name]["default_model"]` (the provider-manifest field — **`default_model`, NOT `model`**;
  `manifest.go:52` `DefaultModel toml:"default_model"`; the contract's "model" is generic shorthand).
- delete the `default_provider` key from the raw map.

Guard (idempotent + no-invent, FR-B7): only fold when `X != ""` AND the target model is non-empty AND
bare (`!strings.Contains(model, "/")`). A bare model with no `default_provider` STAYS bare → becomes the
FR-R5b error the user resolves by writing `"<backend>/<model>"`. **Single-backend providers are untouched**
(a `default_provider` on a single-backend provider, meaningless in v2, just has its dead key dropped, no
fold). A present-but-EMPTY `default_provider` just drops the dead key.

## §3 — The "agent → provider" step is a DOCUMENTED IN-MEMORY NO-OP

FR-B7 says "first map abandoned `agent`/`[agent.*]` terminology → `provider`/`[provider.*]`". Verified
there is **NO in-memory path for it**: `fileConfig` (`file.go:26-35`) has fields `ConfigVersion/Defaults/
Generation/Role/Provider` and **no `Agent` field**; `loadTOML` uses `toml.Unmarshal` (`file.go:134`) which
SILENTLY DROPS unknown `[agent.*]` tables (go-toml/v2 ignores unknown keys unless
`DisallowUnknownFields`). So no `agent`-keyed data ever reaches the typed `Config`. (`grep toml:"agent"
\| [agent \| Agent map` internal/ → empty; "agent" appears only in `bootstrap.go` human-readable COMMENTS.)

⇒ The in-memory `migrateV2ToV3` agent step is a **no-op with a doc comment** explaining why (so the
implementer does NOT chase non-existent data). The REAL `[agent.*]`→`[provider.*]` rewrite is textual and
belongs to S2's on-disk `config upgrade` (which reads raw file text where `[agent.*]` tables survive).
This is the honest, accurate finding that prevents a failed implementation.

## §4 — Migration runs INSIDE Load, BEFORE the caller's DecodeUserOverrides; sets ConfigVersion=3

- `DecodeUserOverrides(cfg.Providers)` (`registry.go:154`) re-encodes each raw map to TOML and unmarshals
  into the v3 `Manifest` — which no longer has a `DefaultProvider` field (removed in P1.M1.T1.S1; only
  `ProviderFlag` remains, `manifest.go:58`). go-toml SILENTLY DROPS the `default_provider` key ⇒ the value
  is lost. So the fold MUST happen while `default_provider` is still in the raw map ⇒ inside `Load`,
  before `return &cfg` (callers run DecodeUserOverrides AFTER Load). ✓
- Trigger: `fileLoaded && cfg.ConfigVersion < CurrentConfigVersion` (covers 0/1/2; `fileLoaded` guards the
  no-file bootstrap path). Runs AFTER all overlays + the Commits==1 normalize, REPLACING the
  `configVersionNotice` older/missing branches. The ahead (`version > current`) case still uses
  `configVersionNotice` (its only remaining live branch in Load; the older/missing branches stay as pure
  tested utilities).
- After migrating, set `cfg.ConfigVersion = CurrentConfigVersion` (the in-memory Config is now v3-SHAPED:
  models prefixed, default_provider gone). The notice captures the ORIGINAL version first. This prevents
  any downstream re-trigger and is the accurate in-memory representation. (TestLoad_ConfigVersion must
  then expect 3 for a v2-file load.)

## §5 — Notice: ONE deprecation notice (no double-notice with the generic advisory)

Today `Load` calls `configVersionNotice` (load.go:152) which prints "schema version N; current is 3.
Run config upgrade…" for older files. If the migration ALSO printed, the user sees TWO notices. So Load
is restructured: the migration branch prints `migrationNotice(origVersion)` (FR-B7-specific: "auto-
migrated in memory; default_provider folded into model; run config upgrade to persist") and the
`configVersionNotice` call moves to an `else if` that now only fires for the AHEAD case. `migrationNotice`
is PURE (no I/O), mirroring `configVersionNotice`'s testability; Load writes it to `noticeOut`.

## §6 — Test-green scope: S1 fixes ALL breakage from the bump+migration (enumerated)

Bumping 2→3 + the migration changes Load behavior, breaking tests that hardcode the version. S1's success
criterion is `go test ./...` GREEN, so S1 updates them all. Breakage map (`grep -rc "config_version = 2
\| current is 2 \| supports up to 2"`):

| File | # | Nature | S1 fix |
|---|---|---|---|
| `internal/config/load_test.go` | 10 | `TestConfigVersionNotice` ("current is 2"/"supports up to 2"), `TestLoad_ConfigVersionAdvisory_Older` (now migration notice), `TestLoad_ConfigVersion` (cfg.ConfigVersion 2→3) | behavioral: update to migration-notice expectations + ConfigVersion==3 |
| `internal/config/bootstrap_test.go` | 4 | asserts `config init`/bootstrap output contains "config_version = 2" | mechanical: 2→3 (or assert vs `config.CurrentConfigVersion`) |
| `internal/config/file_test.go` | 1 | round-trip config_version==2 (int64) | mechanical: →3 |
| `internal/cmd/config_test.go` | 28 | `config init`/`GenerateBootstrapConfig`/`upgradeConfigVersion` OUTPUT assertions "config_version = 2"; some INPUT fixtures (e.g. `:860 input := "config_version = 2\n"`) | mechanical: OUTPUT assertions 2→3; KEEP INPUT v2 fixtures as "2" (they represent v2 files being migrated/upgraded). S2 adds the rewrite-behavior assertions (model prefix, no default_provider). |
| `internal/cmd/default_action_test.go` | 1 | `:1203 fmt.Sprintf("config_version = 2\n…")` — a v2 INPUT fixture | KEEP as "2" (v2 input) IF the test exercises migration; else update. Inspect context. |

**The implementer's rule:** `go test ./...` after the bump+migration; every failure is either a literal
"2" that must become "3" (OUTPUT assertions) or a Load-notice test whose expectation changed (behavioral).
INPUT fixtures representing v2 files STAY "2". Do NOT touch the `config upgrade` REWRITE behavior (S2).

## §7 — config upgrade --help text (DOCS, Mode A): accurate + forward-compatible

`runConfigUpgrade`'s `Long` (`cmd/config.go:95-104`) today says "Only the top-level config_version line
is added or updated". S1's --help touch: add a line noting v3 folds `default_provider` into the `model`
slash-prefix and that loading an older config auto-migrates IN MEMORY (so `config upgrade` is recommended
to persist). Word it to stay accurate AFTER S2 lands the on-disk rewrite. S1 does NOT implement the
on-disk rewrite (S2). Keep the edit minimal and truthful for S1's state (in-memory migration works; on-disk
rewrite is coming).
