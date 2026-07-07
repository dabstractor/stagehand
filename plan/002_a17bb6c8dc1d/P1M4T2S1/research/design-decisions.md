# P1.M4.T2.S1 — Populated `config init`: Design Decisions & Findings

> Companion to `../PRP.md`. The non-obvious design calls an implementer must internalize. Every
> decision is justified against the codebase as it exists (post P1.M3; with P1.M4.T1.S1 assumed
> implemented first per the parallel-execution contract).

## The contract (PRD §9.17 FR-B1/B2, §9.16 FR-D1/D4, §16.4)

`stagecoach config init` must now write a **POPULATED, working** config (not the inert template):
- Cascading detection (FR-D1) finds the highest-priority INSTALLED built-in (order: pi, opencode,
  cursor, agy, gemini, codex, claude).
- Writes `[defaults] provider = <detected>` UNCOMMENTED.
- Writes that provider's `[role.*]` per-role default models (FR-D4 table) UNCOMMENTED so the tool
  works immediately.
- Writes other INSTALLED providers as COMMENTED-OUT `[role.*]` blocks (switching = one-line uncomment).
- Parent dirs created; existing file NOT overwritten unless `--force`; the written path is always printed.

Flags: `--provider <name>` (target a specific provider), `--force` (overwrite), `--template` (retain the
old inert behavior). Stager fallback (FR-D4): a provider whose manifest has nil `TooledFlags` cannot
serve as the stager → the stager role falls back to the next stager-capable provider, annotated.

---

## F1 — `config init` is in `shouldSkipConfigLoad`; Config() is nil there

`internal/cmd/root.go`'s `shouldSkipConfigLoad` returns true for `cmd.Name()=="init"`/`"path"`, so root's
`PersistentPreRunE` returns nil → **no `config.Load`**, no git shell-out, no user overrides decoded. So
inside `runConfigInit`, `Config()` is **nil**. The registry must be built from **built-ins only**:
`provider.NewRegistry(nil)`. Do NOT use providers.go's `newRegistry()` (it reads `Config().Providers` —
nil here → also yields built-ins, but calling it is misleading; build directly with `nil` overrides).
**Works outside a git repo** (the whole point of the skip).

## F2 — The pure builder vs detection+I/O split (the testability crux)

Cascading detection (`reg.IsInstalled` → `exec.LookPath`) is **non-deterministic** — what's on `$PATH`
varies per machine. So `runConfigInit`'s auto-detect output can't be asserted exactly in tests.

**Resolution:** factor the TOML generation into a PURE function
`buildBootstrapConfig(target string, installed []string) string` that takes an already-resolved target
provider name + the installed list and returns the exact TOML string. It does NO detection, NO I/O.
`runConfigInit` does detection (registry + IsInstalled) and I/O (write file) and DELEGATES the string to
the builder. Then:
- **Deterministic unit tests** call `buildBootstrapConfig("pi", []string{"pi","claude"})` and assert the
  EXACT output (provider line, role models from DefaultModelsForProvider, commented claude block, etc.).
- **End-to-end tests** use `--provider pi` (pins the target, deterministic regardless of `$PATH`) and
  assert the written file. The auto-detect path is tested only for VALIDITY (config_version=2, non-empty
  provider ∈ built-ins, `[role.*]` present) — never the exact provider.

## F3 — `DefaultModelsForProvider` + the stager `""` sentinel (FR-D4 fallback)

`config.DefaultModelsForProvider(name)` returns a COPY of the role→model column, or nil for an unknown
name. A **stager value of `""`** means the provider CANNOT serve as the stager (its built-in manifest
has nil/empty `TooledFlags` — today only **pi** and **claude** are stager-capable). Per FR-D4, when the
target provider's stager cell is `""`, the bootstrap must fall back to the next stager-capable provider
and **annotate the fallback** in a comment.

**Stager-fallback resolution:** scan `preferredBuiltins` order (pi, opencode, cursor, agy, gemini, codex,
claude); the fallback is the FIRST whose `DefaultModelsForProvider(...)["stager"] != ""`. That is always
**pi** (`stager: "gpt-5.4-mini"`) — pi and claude are the only stager-capable providers and pi is first.
Compute it dynamically (don't hardcode "pi") so it stays correct if a provider gains `TooledFlags`. The
`[role.stager]` block then carries `provider = "<fallback>"` + the fallback's stager model + a comment
explaining why (target lacks `tooled_flags`).

## F4 — Target resolution: `--provider` → detect → fallback

1. `--provider <name>` (if set): validate it's a known built-in (`reg.Get(name)`); error (exit 1) if not.
   Use it as the target. (config init can't see user-defined providers — Config() is nil, F1.)
2. Else: `target = reg.DefaultProvider(installed)` (cascading FR-D1 over installed built-ins).
3. Else (nothing installed → `""`): **fallback to `"pi"`** (`preferredBuiltins[0]`) with a comment noting
   "no built-in agent detected on $PATH; defaulted to pi — edit if you use a different agent." This keeps
   the config VALID and "works immediately" once pi is installed. (Writing an empty provider would mean
   runtime auto-detect — but a populated config should name a concrete default; pi is the top preference.)

## F5 — KEEP `exampleConfigTemplate`; it becomes the `--template` output (and carries T1.S1's header)

The parallel item **P1.M4.T1.S1** (assumed implemented first) adds a commented `config_version`/upgrade
header block to `exampleConfigTemplate` and changes `Defaults().ConfigVersion → 0`. This subtask does
NOT remove or rewrite that const — it becomes the exact output of `config init --template` (the inert
reference, FR-B2 "retained behind `--template`"). The populated path does NOT use it. The existing
`TestConfigInit_TemplateIsInert` / `TestConfigInit_WritesTemplate` assertions move to the `--template`
path (they still pass — the template is unchanged by this subtask).

## F6 — The populated output's `config_version` is UNCOMMENTED (= CurrentConfigVersion)

Unlike the inert template (everything commented), the populated config is LIVE, so it declares its
schema: write `config_version = 2` (uncommented, top-level) using `config.CurrentConfigVersion` (the
const; read-only). This is what makes the P1.M4.T1.S1 advisory SILENT for a freshly-bootstrapped config
(the version matches the binary). Do NOT read `Defaults().ConfigVersion` — use the const directly.

## F7 — `cmd` → `provider` is already a dependency (no new cycle)

`internal/cmd/default_action.go` and `providers.go` already `import "github.com/dustin/stagecoach/
internal/provider"`. So `config.go` importing `provider` (for `NewRegistry`/`Registry`/`IsInstalled`/
`DefaultProvider`) adds NO new edge. `provider` does not import `cmd`. No cycle. Mirror providers.go's
`installedNames(reg)` helper verbatim (it's unexported there — re-implement the 5-line loop locally, or
factor a shared helper; simplest: local loop in config.go).

## F8 — `--force` applies to BOTH `--template` and populated; refuse-overwrite message unchanged

The current refuse-overwrite error is `exitcode.New(exitcode.Error, fmt.Errorf("config file already
exists at %s (not overwritten)", path))`. KEEP that exact message (existing test asserts `Contains
"already exists"`). `--force` bypasses the existence check and overwrites. `--template --force` writes
the inert template over an existing file. The path is ALWAYS printed on success (FR-B1) — update the
success message slightly: populated → "Wrote config to %s" (drop "example"); `--template` → keep
"Wrote example config to %s".

## F9 — Update the Mode-A help + docs/configuration.md

`configInitCmd.Long` must describe the populated bootstrap, the three flags, and cascading detection
(FR-B1/B2). `configCmd.Long` currently has a manual "Subcommands:" block — **FR-B6 (help de-dup) is
owned by P4.M2.T2.S1**, so do NOT remove it here (avoid conflict); only update the init line. Update
`docs/configuration.md` to document the populated format + the 3 flags (the "File format" section
currently says "Every line ... is commented out" — that's now only true for `--template`).

---

## D1–D9 — Decision summary (maps to PRP contract clauses)

- **D1 (pure builder):** `buildBootstrapConfig(target string, installed []string) string` — deterministic,
  unit-testable; no detection, no I/O. `runConfigInit` = detect + I/O + delegate.
- **D2 (built-ins registry):** `provider.NewRegistry(nil)` (Config() is nil; F1). Local `installedNames`
  loop (mirror providers.go).
- **D3 (target):** `--provider` (validated) → `reg.DefaultProvider(installed)` → fallback `"pi"` (F4).
- **D4 (stager fallback):** target stager `""` ⇒ `[role.stager]` routes to first stager-capable in
  `preferredBuiltins`, annotated (F3).
- **D5 (flags):** `--provider <string>`, `--force <bool>`, `--template <bool>` as LOCAL flags on
  `configInitCmd` (`configInitCmd.Flags()`). Read in `runConfigInit`.
- **D6 (overwrite):** refuse unless `--force` (both paths); keep the "already exists" message (F8).
- **D7 (keep template):** `exampleConfigTemplate` = the `--template` output (carries T1.S1's header; F5).
- **D8 (deterministic tests):** pin with `--provider` for exact output; auto-detect for validity only (F2).
- **D9 (TOML validity):** generated string parses via `go-toml` into `map[string]any`; substring asserts.

## SCOPE BOUNDARY (owned by siblings — do NOT implement)
- `config upgrade` (P1.M4.T3), first-run fallback (P1.M4.T4), FR-B6 help de-dup (P4.M2.T2.S1).
- `Defaults()`/`CurrentConfigVersion`/`load.go` advisory (P1.M4.T1.S1 — consume read-only).
- root.go, providers.go, default_action.go (P1.M4.T1/P4.M1 — do NOT edit).
