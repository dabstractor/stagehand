---
name: "P1.M4.T3.S1 — Implement `config upgrade` command (PRD §9.17 FR-B5, §15.3) + FR-B6 help de-duplication"
description: |

  Add `stagecoach config upgrade` (internal/cmd/config.go): a cobra subcommand that rewrites an EXISTING
  global config IN PLACE to `config_version = CurrentConfigVersion` via a minimal TEXTUAL transform that
  preserves every other line (comments, ordering, user values — byte-for-byte). v2.0 has no removed/renamed
  keys, so the only edit is ensuring the top-level `config_version = 2` line is present/current; running
  twice is a no-op ("already up to date"). Also add `"upgrade"` to shouldSkipConfigLoad (root.go) so it
  works outside a repo, and apply FR-B6 help de-duplication: remove the manual "Subcommands:" block from
  BOTH `configCmd.Long` and `providersCmd.Long` (cobra's auto "Available Commands" is the single source).

  CONTRACT (PRD §9.17 FR-B5 / FR-B6, §15.3; work-item spec):
    - FR-B5: "config upgrade rewrites an existing config to CurrentConfigVersion in place: preserving user
      values for keys that still exist, comments out removed/renamed keys with a note. Simple, idempotent,
      future-extensible. There are no existing users to migrate (v1 had no config_version), so the first
      upgrade simply adds config_version=N and is a no-op otherwise."
    - Work item LOGIC: "read the existing config file (error if missing), parse it, set/update
      config_version to CurrentConfigVersion, re-serialize preserving structure, write back. For v2.0 this
      is primarily: ensure config_version=CurrentConfigVersion is present; leave all other content
      unchanged. Print a confirmation. Idempotent: running twice is safe."
    - Work item DOCS (Mode A): "Update the config command group Long help to list the upgrade subcommand.
      Remove the manual 'Subcommands:' block from config and providers parent commands (FR-B6 help
      de-duplication — cobra's 'Available Commands' is the single source)."

  INPUT (upstream — all EXIST, READ/CONSUME only, do NOT modify their behavior):
    - `config.CurrentConfigVersion = 2` (const) — internal/config/config.go:18. The value written.
    - `config.GlobalConfigPath() string` — internal/config/file.go:83. The read+write target.
    - The config-file shape: top-level `config_version` (root metadata, toml:"config_version" on fileConfig),
      then `[defaults]`/`[generation]`/`[role.*]`/`[provider.*]` tables.

  OUTPUT: `stagecoach config upgrade` brings a config file up to CurrentConfigVersion; existing user values
  are preserved. `config`/`providers` help no longer duplicates their leaf list.

  DELIVERABLES (2 source files EDITED, 1 test file EDITED):
    EDIT internal/cmd/config.go     — +configUpgradeCmd (cobra); +runConfigUpgrade (I/O + gate + message);
                                      +upgradeConfigVersion(content, version) (PURE textual transform);
                                      +AddCommand(configUpgradeCmd) in init(); FR-B6 remove configCmd
                                      "Subcommands:" block; Mode-A configCmd/configUpgradeCmd Long; pkg doc.
    EDIT internal/cmd/root.go        — shouldSkipConfigLoad: +`|| name == "upgrade"`.
    EDIT internal/cmd/providers.go   — FR-B6: remove providersCmd "Subcommands:" block (cobra auto-lists).
    EDIT internal/cmd/config_test.go — +upgradeConfigVersion pure unit tests + runConfigUpgrade Execute tests.

  SCOPE BOUNDARY (owned by siblings — do NOT implement/edit):
    - `config init` rewrite / --provider/--force/--template / buildBootstrapConfig — PARALLEL P1.M4.T2.S1
      (it edits config.go too; see COORDINATION below). Do NOT touch runConfigInit / configInitCmd /
      buildBootstrapConfig / exampleConfigTemplate.
    - First-run auto-bootstrap (P1.M4.T4); default_action.go / the decompose CLI flags (P4.M1).
    - config pkg internals: config.go (const/Defaults), file.go (fileConfig/load), load.go (advisory),
      role_defaults.go — all READ-ONLY.
    - P4.M2.T2.S1 ("Register config upgrade + help de-dup + verify init wiring") is a LATER verify step;
      this task IMPLEMENTS the command + dedup per its contract; P4.M2.T2.S1 will then just verify/wire.

  PARALLEL-EXECUTION COORDINATION (CRITICAL — sibling edits config.go too):
    P1.M4.T2.S1 (config init) runs in PARALLEL and edits `internal/cmd/config.go`. It (a) keeps
    `configCmd`'s manual "Subcommands:" block ("only update the init line"), (b) does NOT implement
    `config upgrade`, (c) does NOT edit root.go or providers.go. ⇒
    - CONFLICT POINT: `configCmd.Long`. The sibling updates the `init` line inside the "Subcommands:"
      block; THIS task REMOVES the whole block (FR-B6). SEQUENCING: the sibling lands FIRST; this task's
      edit removes whatever "Subcommands:" block then exists. Describe the edit as "remove the manual
      'Subcommands:' block from configCmd.Long (whatever its current form)".
    - NON-CONFLICTING: my additions (configUpgradeCmd/runConfigUpgrade/upgradeConfigVersion + the
      `configCmd.AddCommand(configUpgradeCmd)` line in init()) touch lines the sibling does NOT touch.
      Both append an AddCommand line in the SAME init() — add mine after the sibling's init/path lines.
    - root.go (shouldSkipConfigLoad) and providers.go (FR-B6 block) are NOT edited by the sibling → safe.

  Deliverable: `stagecoach config upgrade` rewrites the global config in place to config_version=2
  (preserving all other lines), errors clearly if the file is missing or malformed, is idempotent, and
  works outside a git repo; `config`/`providers --help` no longer duplicate their leaf list.
  `go build ./... && go test ./...` green; `go vet ./...` clean; go.mod/go.sum unchanged.

---

## Goal

**Feature Goal**: Ship the `config upgrade` remediation command (PRD §9.17 FR-B5) that the load-time
config_version advisory (P1.M4.T1.S1) already points users at: rewrite an existing global config in place
so its top-level `config_version` equals `CurrentConfigVersion`, via a minimal textual edit that preserves
every other line (comments, ordering, user values). It must be idempotent (a 2nd run is a no-op), error
clearly when the file is missing or malformed, and work outside a git repo. Also remove the redundant
manual "Subcommands:" blocks from the `config` and `providers` parent commands (FR-B6) so cobra's auto
"Available Commands" is the single source — which makes the new `upgrade` leaf appear with zero extra prose.

**Deliverable** (2 source edits + 1 source edit + 1 test edit):
1. EDIT `internal/cmd/config.go` — add `configUpgradeCmd` (cobra: `Use:"upgrade"`, NoArgs, Mode-A Long),
   `runConfigUpgrade` (read → validate-TOML → `upgradeConfigVersion` → write → message), the PURE
   `upgradeConfigVersion(content string, version int) (string, bool)` textual transform, register via
   `configCmd.AddCommand(configUpgradeCmd)` in `init()`; FR-B6 remove configCmd's "Subcommands:" block;
   Mode-A Long for configUpgradeCmd + configCmd intro; package-doc touch.
2. EDIT `internal/cmd/root.go` — `shouldSkipConfigLoad`: add `|| name == "upgrade"`.
3. EDIT `internal/cmd/providers.go` — FR-B6: remove providersCmd's "Subcommands:" block.
4. EDIT `internal/cmd/config_test.go` — pure `upgradeConfigVersion` unit tests + Execute-driven
   `runConfigUpgrade` tests (missing file / add / already-current / older-update / idempotent / malformed /
   extra-args / outside-repo).

**Success Definition**: `make build && ./bin/stagecoach config upgrade` on a config lacking
`config_version` rewrites it in place adding `config_version = 2` while leaving every other line
byte-identical (verified by diff), prints "Upgraded config at <path> to version 2.", exits 0; on a config
already at version 2 it prints "already at version 2 (no changes)" and does NOT rewrite (byte-identical);
on a missing file it exits 1 with "no config file at <path> (run 'stagecoach config init' first)"; on
malformed TOML it exits 1 with "not valid TOML" and leaves the file unchanged; `config upgrade x` exits 1
(NoArgs); it works from a non-git directory; running it twice is a no-op. `stagecoach config --help` and
`stagecoach providers --help` show their leaves ONCE (under "Available Commands"), with no manual
"Subcommands:" prose. `go test -race ./internal/cmd/` green; `go test ./...` no regression;
`go vet ./...` clean; `gofmt -l` empty; go.mod/go.sum unchanged.

## User Persona

**Target User**: A Stagecoach user whose config predates the schema-version feature, OR who edited an older
config by hand — they hit the load-time advisory ("config file has no config_version / is older … run
'stagecoach config upgrade'") and want a one-command remediation that does NOT discard their hand-tuned
values.

**Use Case**: `stagecoach` prints the config_version advisory → user runs `stagecoach config upgrade` →
their config is rewritten in place with `config_version = 2` added and everything else intact → the next
`stagecoach` run is advisory-free.

**User Journey**: run stagecoach → see advisory naming `config upgrade` → run `stagecoach config upgrade`
→ see "Upgraded config at ~/.config/stagecoach/config.toml to version 2." → `git diff` the config (if
version-controlled) shows ONLY the new `config_version` line → re-run stagecoach → advisory gone. Re-running
`config upgrade` later says "already at version 2 (no changes)".

**Pain Points Addressed**: (1) "the advisory told me to upgrade but I don't want to lose my config" —
solved by in-place textual preservation (no round-trip); (2) "did it change anything I care about?" —
solved by touching ONLY the `config_version` line + a clear confirmation; (3) "what if I run it twice?" —
idempotent (no-op the 2nd time).

## Why

- **Closes the loop on PRD §9.17 FR-B5 (P0).** The P1.M4.T1.S1 load-time advisory (load.go:263
  `configVersionNotice`) already prints *"Run 'stagecoach config upgrade' …"* — the command it names MUST
  exist and behave as implied (in-place rewrite to CurrentConfigVersion, safe when already current).
- **Preservation is the whole point.** FR-B5 explicitly requires preserving user values and (future)
  commenting-out removed keys — a TOML round-trip would strip every comment and reorder sections
  (external-research.md §1). A minimal textual transform is the only faithful implementation.
- **Idempotent + future-extensible.** v2.0 has no removed/renamed keys, so the transform is just the
  version line; the pure `upgradeConfigVersion` is the single extension point for a future v3 migration.
- **FR-B6 de-dup is bundled** (the work item's DOCS line) — removes a real v1 wart (subcommands listed
  twice) and makes `upgrade` appear with zero extra prose.

## What

A new cobra subcommand `configUpgradeCmd` (`Use: "upgrade"`) registered under `configCmd`, with
`runConfigUpgrade`:
1. `path := config.GlobalConfigPath()`; `os.ReadFile(path)` → IsNotExist → exit 1 "no config file … (run
   'stagecoach config init' first)"; other read error → exit 1.
2. **Validity gate**: `toml.Unmarshal(data, &map[string]any{})` → on error, exit 1 "config %s is not valid
   TOML: %w" (refuse to mangle an unparseable file). Never marshal back.
3. `newContent, changed := upgradeConfigVersion(string(data), config.CurrentConfigVersion)`.
4. If `!changed` → print "Config at %s is already at version %d (no changes)." → return nil (NO rewrite).
5. Else `os.WriteFile(path, []byte(newContent), 0o644)` → print "Upgraded config at %s to version %d.".

`upgradeConfigVersion(content string, version int) (string, bool)` — PURE textual transform:
- Scan lines in the **top-level region** (before the first `[table]` header) for an uncommented
  `config_version = N` (regex `^config_version\s*=\s*([0-9]+)`).
  - Found, value == version → return content unchanged, `changed=false`.
  - Found, value != version → rewrite THAT line to `config_version = <version>`, `changed=true`.
  - Not found → insert `config_version = <version>` after the leading comment/blank header block (before
    the first table/key), `changed=true`.
- Every other line is byte-identical.

`shouldSkipConfigLoad` (root.go) gains `|| name == "upgrade"`. FR-B6 removes the manual "Subcommands:"
block from `configCmd.Long` and `providersCmd.Long`.

### Success Criteria

- [ ] `configUpgradeCmd` exists (`Use:"upgrade"`, `Args: cobra.NoArgs`, Mode-A Long) and is registered via
      `configCmd.AddCommand(configUpgradeCmd)` in `config.go`'s `init()`; cobra auto-lists it.
- [ ] `shouldSkipConfigLoad` returns true for `cmd.Name()=="upgrade"` (works outside a repo; config.Load
      does NOT run for it).
- [ ] `upgradeConfigVersion(content, version)` is PURE (no I/O) and implements D4's three outcomes; for the
      "already current" case it returns the content BYTE-IDENTICAL with `changed=false`.
- [ ] `runConfigUpgrade`: missing file → exit 1 mentioning `config init`; malformed TOML → exit 1 "not
      valid TOML" WITHOUT rewriting; valid → writes the minimal edit + prints a confirmation; never
      `os.Exit`; routes errors via `exitcode.New(exitcode.Error, …)`.
- [ ] Idempotent: `upgradeConfigVersion` applied twice → 2nd call `changed=false`, byte-identical; the
      Execute path on an already-current file does NOT rewrite.
- [ ] FR-B6: `configCmd.Long` and `providersCmd.Long` contain NO manual "Subcommands:" block;
      `stagecoach config --help` / `stagecoach providers --help` list leaves once under "Available Commands".
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/cmd/` empty;
      go.mod/go.sum unchanged; only the 4 listed files differ (config.go, root.go, providers.go,
      config_test.go). runConfigInit/buildBootstrapConfig/exampleConfigTemplate UNCHANGED.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact cobra subcommand +
RunE skeleton + the PURE transform skeleton (below), the textual-vs-roundtrip rationale (external-research.md
§1/§2), the validity-gate idiom, the upstream signatures (all quoted + in design-decisions.md F1–F9), the
test conventions to mirror (config_test.go's setupNoRepo/saveRootState), and the parallel-sibling
coordination map (F4). No git/prompt/provider/decompose knowledge required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions
- docfile: plan/002_a17bb6c8dc1d/P1M4T3S1/research/design-decisions.md
  why: the 10 decisions + 9 findings. D1 (TEXTUAL not round-trip — the core correctness call), D2 (pure
       upgradeConfigVersion for testability), D3 (scan top-level only; break at first [table]), D4 (the 3
       idempotent outcomes), D5 (v2.0 = version line only; no key migrations), D6 (missing-file → config
       init), D7 (--config NOT honored), D8 (register + shouldSkipConfigLoad), D9/D10 (FR-B6 dedup + Mode-A).
  critical: D1, D3, D4 (the transform), F4 (the config.go SHARED EDIT with the parallel sibling), F6 (the
       advisory already names this command — it must behave as implied).

- docfile: plan/002_a17bb6c8dc1d/P1M4T3S1/research/external-research.md
  why: WHY textual editing (§1 go-toml marshal drops comments/reorders), the TOML top-level-before-tables
       rule (§2 — drives D3), go-toml non-strict unmarshal as a safe validity gate (§3), idempotency by
       construction (§4).
  critical: §1 (round-trip is forbidden by the contract), §2 (scan/break-at-table correctness), §3 (use
       map[string]any for the gate, not fileConfig — never reject a merely-incomplete config).

- docfile: plan/002_a17bb6c8dc1d/P1M4T2S1/PRP.md   (the PARALLEL sibling — consume its outputs)
  why: it rewrites config.go (config init) and is the ONLY other editor of config.go this cycle. Its
       outputs to consume: configCmd/configInitCmd/configPathCmd/exampleConfigTemplate/runConfigPath (kept),
       a rewritten runConfigInit + buildBootstrapConfig + --provider/--force/--template flags + imports
       (provider, strings). It KEEPS configCmd's "Subcommands:" block and does NOT implement upgrade.
  critical: F4 — configCmd.Long is the SHARED EDIT POINT. Sequence: sibling first; this task removes the
       whole "Subcommands:" block afterward. Do NOT touch runConfigInit/buildBootstrapConfig/exampleConfigTemplate.

# MUST READ — the file being extended (EDIT)
- file: internal/cmd/config.go   (EDIT — add upgrade cmd + transform; FR-B6 remove configCmd block; Mode-A)
  section: `configCmd` (the parent — REMOVE its manual "Subcommands:" block, update intro), the `init()`
       (ADD `configCmd.AddCommand(configUpgradeCmd)` next to the init/path AddCommand lines), `runConfigInit`/
       `configPathCmd`/`exampleConfigTemplate` (KEEP — do NOT touch).
  why: this is THE file. The upgrade command + pure transform + registration all land here. The sibling
       rewrites runConfigInit + adds helpers in the SAME file — your additions are independent lines.
  pattern: mirror configPathCmd/runConfigPath (the simplest existing leaf: NoArgs, SilenceErrors/Usage,
       RunE, routes via exitcode.New, prints to cmd.OutOrStdout, never os.Exit) for configUpgradeCmd.
  gotcha: COORDINATE configCmd.Long with the sibling (F4). Do NOT touch the sibling's runConfigInit/
       buildBootstrapConfig/flags. Add `import "strconv"` and/or `"regexp"` only if needed (strings is already
       imported after the sibling). Add `import "github.com/pelletier/go-toml/v2"` AS `toml` for the gate.

# MUST READ — shouldSkipConfigLoad (EDIT)
- file: internal/cmd/root.go   (EDIT — shouldSkipConfigLoad gains "upgrade")
  section: `func shouldSkipConfigLoad(cmd *cobra.Command) bool { name := cmd.Name(); return name ==
       "init" || name == "path" }` (root.go:97). Add `|| name == "upgrade"`.
  why: upgrade operates on the config FILE PATH, not the resolved config — it must work outside a git repo
       and must NOT run config.Load (no git-config layer, no advisory double-fire). Matches init/path.
  gotcha: the sibling does NOT edit root.go → conflict-free. Keep the function's doc comment current (it
       currently says "skip for config init/path"; add upgrade).

# MUST READ — providers parent (EDIT — FR-B6 dedup, no sibling conflict)
- file: internal/cmd/providers.go   (EDIT — remove providersCmd "Subcommands:" block)
  section: `providersCmd.Long` (providers.go:27) contains a `Subcommands:\n  list ...\n  show <name> ...`
       block. Remove that block; keep the intro prose. cobra auto-lists list/show under "Available Commands".
  why: FR-B6 (the work item DOCS line) requires removing the block from BOTH config AND providers parents.
       providers.go is NOT edited by the sibling → safe.
  gotcha: do NOT touch providersListCmd/providersShowCmd/runProvidersList/runProvidersShow — only the Long.

# MUST READ — the schema-version const + path (consume read-only; do NOT edit)
- file: internal/config/config.go   (READ CurrentConfigVersion)
  section: `const CurrentConfigVersion = 2` (config.go:18). The value the upgrade writes. Use the CONST,
       not Defaults().ConfigVersion (which is the 0 "unset" sentinel — meaningless here).
- file: internal/config/file.go   (READ GlobalConfigPath)
  section: `func GlobalConfigPath() string` (file.go:83). The read+write target. Tests: setupNoRepo sets
       HOME=XDG=t.TempDir() → path is <tmp>/stagecoach/config.toml.

# MUST READ — the advisory that names this command (READ — proves the contract)
- file: internal/config/load.go   (READ configVersionNotice)
  section: `configVersionNotice(fileLoaded, version)` (load.go:263) emits, for missing/older version:
       *"Run 'stagecoach config upgrade' or 'stagecoach config init --force'."* (load.go:275-282).
  why: the advisory is the user-facing trigger for THIS command. The command's behavior (rewrite in place
       to CurrentConfigVersion; safe if already current) must match what the advisory implies.

# MUST READ — the config tests + helpers (EDIT; reuse helpers)
- file: internal/cmd/config_test.go   (EDIT — add upgrade tests)
  section: `setupNoRepo(t)` (isolates HOME/XDG, chdir plain dir, returns globalDir), `saveRootState`/
       `restoreRootState` (rootCmd singleton hygiene), the existing `TestConfigInit_*`/`TestConfigPath_*`
       patterns (drive rootCmd via SetArgs + Execute; assert exitcode.For(err) + os.ReadFile(GlobalConfigPath())).
       The file already imports `regexp` (upgradeConfigVersion may use it).
  why: mirror these EXACTLY for the upgrade tests. upgradeConfigVersion is tested DIRECTLY (same package,
       pure) for determinism; runConfigUpgrade via Execute for I/O/error/missing/malformed.
  gotcha: each Execute test wraps in saveRootState/restoreRootState (rootCmd is a package global; flags/
       args persist across parses). setupNoRepo makes GlobalConfigPath() deterministic (<tmp>/stagecoach/…).

- url: https://toml.io/en/v1.0.0#keys
  why: root (top-level) keys must precede [table] headers — drives the scan/break-at-first-table logic (D3).
  critical: a `config_version` AFTER a `[table]` is NOT the schema key; break the scan at the first table.
- url: https://pkg.go.dev/github.com/pelletier/go-toml/v2#Unmarshal
  why: non-strict by default (ignores unknown keys) — safe validity gate; use `map[string]any` not fileConfig.
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  config.go            # EDIT: +configUpgradeCmd/runConfigUpgrade/upgradeConfigVersion; +AddCommand; FR-B6 block removal; Mode-A. (sibling also edits runConfigInit/helpers — independent lines.)
  config_test.go       # EDIT: +upgrade unit tests + Execute tests.
  root.go              # EDIT: shouldSkipConfigLoad += "upgrade". (sibling does NOT touch root.go.)
  providers.go         # EDIT: FR-B6 remove providersCmd "Subcommands:" block. (sibling does NOT touch providers.go.)
  root_test.go         # READ: reuse saveRootState/restoreRootState/chdir. DO NOT EDIT.
internal/config/
  config.go            # READ CurrentConfigVersion(=2). DO NOT EDIT.
  file.go              # READ GlobalConfigPath + fileConfig (top-level config_version). DO NOT EDIT.
  load.go              # READ configVersionNotice (the advisory naming this cmd). DO NOT EDIT.
go.mod / go.sum        # UNCHANGED (go-toml/v2 + cobra + exitcode already deps).
```

### Desired Codebase tree with files to be added/changed

```bash
internal/cmd/config.go            # EDIT — +configUpgradeCmd; +runConfigUpgrade; +upgradeConfigVersion (pure);
                                   #        +configCmd.AddCommand(configUpgradeCmd); FR-B6 configCmd block removal; Mode-A Long; pkg doc.
internal/cmd/root.go              # EDIT — shouldSkipConfigLoad: +`|| name == "upgrade"` + doc comment.
internal/cmd/providers.go         # EDIT — FR-B6: remove providersCmd "Subcommands:" block.
internal/cmd/config_test.go       # EDIT — +upgradeConfigVersion pure tests + runConfigUpgrade Execute tests.
# NO new files. go.mod/go.sum UNCHANGED. runConfigInit/buildBootstrapConfig/exampleConfigTemplate UNCHANGED
# (parallel sibling owns them). default_action.go/role_defaults.go/registry.go UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (TEXTUAL editing, NOT a TOML round-trip — D1/external-research §1): go-toml/v2 marshal DROPS all
// comments and reorders/reformats tables. FR-B5 requires "preserving user values … leave all other content
// unchanged." So upgradeConfigVersion does minimal LINE edits; toml.Unmarshal is used ONLY as a validity
// gate (refuse to mangle an unparseable file) — NEVER marshalled back.

// CRITICAL (scan the TOP-LEVEL region only — D3/external-research §2): config_version is ROOT metadata
// (toml:"config_version" on the root fileConfig struct). TOML root keys MUST precede [table] headers. So the
// scan for `config_version = N` walks lines UNTIL the first `[...]` header, then STOPS. A config_version
// after a table is a different key — never matched (no false hit, no duplicate root key).

// CRITICAL (the 3 idempotent outcomes — D4): found-and-same-value → return content BYTE-IDENTICAL +
// changed=false (the "already up to date" path; runConfigUpgrade must NOT rewrite). found-and-different →
// rewrite that ONE line. not-found → insert one line after the leading comment/blank header block. A 2nd run
// always hits the "same-value" branch → no-op. Do NOT special-case idempotency; it falls out of the transform.

// CRITICAL (validity gate uses map[string]any, NOT fileConfig — external-research §3): go-toml is non-strict;
// unmarshal into map[string]any rejects ONLY syntax errors. A v1 config with just [defaults] (or unknown
// future keys) parses fine. Never reject a merely-incomplete config. Reading the existing version is done
// textually (regex), not from this map — the map is purely a syntax check.

// GOTCHA (missing file → config init, NOT a fresh bootstrap — D6): upgrade targets an EXISTING file. IsNotExist
// → exit 1 "no config file at %s (run 'stagecoach config init' first)". Do NOT create a file (that's init's job).

// GOTCHA (--config / STAGECOACH_CONFIG is intentionally NOT honored — D7): the work item INPUT names
// GlobalConfigPath(). Upgrade is in shouldSkipConfigLoad (config.Load does NOT run), so the Layer-7 discovery
// override is not resolved. Upgrade rewrites the GLOBAL file. Document this in the command's Long help.

// GOTCHA (regex anchored at column 0 ignores COMMENTED lines): `^config_version\s*=\s*([0-9]+)` does NOT
// match `# config_version = 2` (starts with '#'). So a config whose only config_version is commented (e.g.
// the inert exampleConfigTemplate output) is treated as "no top-level config_version" → an uncommented line
// is inserted. The original comment line is preserved byte-for-byte.

// GOTCHA (config.go is shared with the PARALLEL sibling P1.M4.T2.S1 — F4): the sibling rewrites runConfigInit
// + adds buildBootstrapConfig/--provider/--force/--template + imports. Do NOT touch those. The ONE shared
// line-region is configCmd.Long (sibling updates the init line in the "Subcommands:" block; this task REMOVES
// the block). Sequence: sibling first, then this task. root.go + providers.go are NOT touched by the sibling.

// GOTCHA (FR-B6 dedup touches BOTH parents — D9): remove the manual "Subcommands:" block from configCmd.Long
// (config.go) AND providersCmd.Long (providers.go). cobra auto-lists leaves under "Available Commands" — so
// the new upgrade leaf appears with zero extra prose. Do NOT re-add a manual list (Mode-A "list the upgrade
// subcommand" is satisfied by REGISTRATION).

// GOTCHA (when rewriting an existing version line, a trailing inline comment is dropped): e.g.
// `config_version = 1  # v1` → rewritten to `config_version = 2` (the `# v1` is lost). Acceptable: this only
// happens on an actual version bump (value differs); the same-value path returns byte-identical (comment kept).
// Note it; do not over-engineer to preserve inline comments on a bump.

// GOTCHA (cobra rootCmd is a package global — test hygiene): each Execute-driven test MUST saveRootState +
// restoreRootState (resets args/out/err + resetFlags). setupNoRepo makes GlobalConfigPath() deterministic.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/cmd/config.go — ADD these. KEEP everything the sibling owns (runConfigInit, buildBootstrapConfig,
// exampleConfigTemplate, configInitCmd flags, etc.). Add imports: "strconv", "regexp" (if used), and
// `toml "github.com/pelletier/go-toml/v2"` for the validity gate. (fmt/os/path/filepath/cobra/config/exitcode already imported.)

// configUpgradeCmd implements `stagecoach config upgrade` (PRD §9.17 FR-B5). Rewrites an EXISTING global
// config in place so its top-level config_version equals CurrentConfigVersion, via a minimal TEXTUAL edit
// that preserves every other line. Idempotent. Works outside a git repo (shouldSkipConfigLoad).
var configUpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade an existing config to the current schema version",
	Long: `Rewrite an existing Stagecoach config file in place so its config_version matches this binary's
current schema version (` + fmt.Sprintf("`config_version = %d`", config.CurrentConfigVersion) + `).

Only the top-level config_version line is added or updated — every other line (your values, comments,
ordering) is preserved byte-for-byte. Running it twice is safe: a file already at the current version is
left unchanged ("already up to date").

This is the remediation the load-time advisory points at when a config has no config_version or an older
one. It targets the GLOBAL config (the path printed by ` + "`stagecoach config path`" + `).

If no config file exists, run ` + "`stagecoach config init`" + ` first. If the file is not valid TOML, it is
left untouched and an error is printed.`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runConfigUpgrade,
}

// runConfigUpgrade reads the global config, validates it is parseable TOML, ensures the top-level
// config_version equals CurrentConfigVersion (minimal textual edit), writes it back, and prints a
// confirmation. Never calls os.Exit; routes errors via exitcode.New. (PRD §9.17 FR-B5.)
func runConfigUpgrade(cmd *cobra.Command, args []string) error {
	path := config.GlobalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return exitcode.New(exitcode.Error, fmt.Errorf("no config file at %s (run 'stagecoach config init' first)", path))
		}
		return exitcode.New(exitcode.Error, fmt.Errorf("read config %s: %w", path, err))
	}
	// Validity gate: refuse to mangle an unparseable file. Non-strict (map[string]any) — a merely-
	// incomplete config (e.g. only [defaults]) is fine; only genuine syntax errors fail.
	var probe map[string]any
	if err := toml.Unmarshal(data, &probe); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("config %s is not valid TOML: %w", path, err))
	}
	newContent, changed := upgradeConfigVersion(string(data), config.CurrentConfigVersion)
	if !changed {
		fmt.Fprintf(cmd.OutOrStdout(), "Config at %s is already at version %d (no changes).\n", path, config.CurrentConfigVersion)
		return nil
	}
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("write config %s: %w", path, err))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Upgraded config at %s to version %d.\n", path, config.CurrentConfigVersion)
	return nil
}

// configVersionLineRe matches an UNCOMMENTED top-level config_version assignment, capturing the integer
// value. Anchored at column 0 (a leading '#' is not matched) — commented `# config_version = 2` is ignored.
var configVersionLineRe = regexp.MustCompile(`^config_version\s*=\s*([0-9]+)`)

// upgradeConfigVersion returns content with the TOP-LEVEL config_version set to version, via a minimal
// TEXTUAL edit that preserves every other line byte-for-byte (PRD §9.17 FR-B5: "preserving user values …
// leave all other content unchanged"). It scans only the top-level region (before the first [table] header —
// config_version is root metadata). Outcomes (D4):
//   - found with value == version  → content unchanged, changed=false (the "already up to date" path)
//   - found with value != version  → that ONE line rewritten, changed=true
//   - not found                    → one `config_version = <version>` line inserted after the leading
//                                    comment/blank header block, changed=true
// PURE (no I/O, no error) → fully unit-testable. v2.0 has no removed/renamed keys, so no other line is
// touched; this function is the single future extension point (add a version-keyed migration for v3+).
func upgradeConfigVersion(content string, version int) (string, bool) {
	lines := strings.Split(content, "\n")
	want := strconv.Itoa(version)

	// 1. Scan the top-level region for an existing config_version (stop at the first [table] header).
	for i, line := range lines {
		if isTableHeader(line) {
			break // config_version must precede tables; nothing top-level after this is the schema key
		}
		if m := configVersionLineRe.FindStringSubmatch(line); m != nil {
			if strings.TrimSpace(m[1]) == want {
				return content, false // already current — byte-identical
			}
			lines[i] = "config_version = " + want
			return strings.Join(lines, "\n"), true
		}
	}

	// 2. No top-level config_version — insert one after the leading comment/blank header block.
	insertAt := leadingHeaderEnd(lines)
	ins := append([]string{}, lines[:insertAt]...)
	ins = append(ins, "config_version = "+want)
	ins = append(ins, lines[insertAt:]...)
	return strings.Join(ins, "\n"), true
}

// isTableHeader reports whether line is a TOML [table] / [[array-of-tables]] header (non-comment, col 0).
func isTableHeader(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") {
		return false
	}
	return strings.HasPrefix(t, "[")
}

// leadingHeaderEnd returns the index of the first line that is NOT a comment and NOT blank — i.e. the end
// of the leading comment/blank header block. Used as the insertion point for a new top-level config_version
// (so it sits with the other root keys, before the first table). Returns len(lines) if the whole file is
// comments/blanks.
func leadingHeaderEnd(lines []string) int {
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		return i
	}
	return len(lines)
}
```

```go
// internal/cmd/root.go — EDIT shouldSkipConfigLoad (add "upgrade"; update the doc comment).
func shouldSkipConfigLoad(cmd *cobra.Command) bool {
	name := cmd.Name()
	return name == "init" || name == "path" || name == "upgrade" // upgrade operates on the FILE, not resolved config
}
```

```go
// internal/cmd/config.go init() — ADD the registration next to the sibling's init/path lines.
func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configUpgradeCmd) // NEW (P1.M4.T3.S1)
	rootCmd.AddCommand(configCmd)
}
```

```go
// internal/cmd/config.go configCmd.Long — FR-B6: REMOVE the manual "Subcommands:" block; update intro.
// (Sequence AFTER the sibling's init-line update. cobra auto-lists init/path/upgrade under "Available Commands".)
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the Stagecoach config file",
	Long: `Inspect, bootstrap, or upgrade the Stagecoach global config file.`,
	SilenceErrors: true,
	SilenceUsage:  true,
}
```

```go
// internal/cmd/providers.go providersCmd.Long — FR-B6: REMOVE the "Subcommands:" block; keep intro.
var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "Manage AI provider manifests",
	Long: `Inspect the built-in and user-defined provider manifests Stagecoach uses to generate commits.

User-defined providers (from the global or repo-local config file) override built-ins of the same
name; new names add new providers (PRD §12.8).`,
	SilenceErrors: true,
	SilenceUsage:  true,
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/cmd/root.go — shouldSkipConfigLoad += "upgrade"
  - FILE: internal/cmd/root.go. CHANGE `return name == "init" || name == "path"` →
      `return name == "init" || name == "path" || name == "upgrade"`.
  - UPDATE the function's doc comment ("skip for config init/path/upgrade — they operate on the config
      FILE/PATH, not the resolved config").
  - GOTCHA: the sibling does NOT edit root.go → conflict-free. This makes `config upgrade` work outside a
      git repo and skips config.Load (no advisory double-fire).

Task 2: EDIT internal/cmd/config.go — add imports for the upgrade feature
  - ADD `toml "github.com/pelletier/go-toml/v2"` (validity gate), `"strconv"` (int→string). `"regexp"` if
      used; `"strings"` (already imported after the sibling — confirm). KEEP existing imports.
  - GOTCHA: go.mod/go.sum UNCHANGED (go-toml/v2 already a dep). Do NOT duplicate the sibling's imports.

Task 3: EDIT internal/cmd/config.go — add configUpgradeCmd + runConfigUpgrade + upgradeConfigVersion
        (+ isTableHeader + leadingHeaderEnd + configVersionLineRe)
  - ADD the symbols in the "Data models" skeleton VERBATIM. upgradeConfigVersion is PURE (D2). The validity
      gate uses `toml.Unmarshal(data, &map[string]any{})` (D1/external-research §3). Missing-file → config
      init (D6). --config NOT honored (D7).
  - GOTCHA: scan top-level only; break at first [table] (D3). The 3 outcomes (D4). Never marshal back.
  - GOTCHA: do NOT touch runConfigInit / buildBootstrapConfig / exampleConfigTemplate / configInitCmd flags.

Task 4: EDIT internal/cmd/config.go — register + FR-B6 dedup + Mode-A
  - ADD `configCmd.AddCommand(configUpgradeCmd)` in init() (after the init/path lines).
  - FR-B6: REMOVE configCmd's manual "Subcommands:" block (whatever its form AFTER the sibling's init-line
      update — coordinate via F4); keep a one-line intro. Mode-A: configUpgradeCmd already has a full Long
      (Task 3); configCmd.Short/Long updated to mention upgrade conceptually (NOT a re-listed subcommand set).
  - UPDATE the package doc comment (currently "two leaf subcommands: init … path …") to mention the third
      leaf (upgrade) + the populated init (sibling) — Mode-A. (Keep it accurate vs the sibling's state.)
  - GOTCHA: cobra auto-lists init/path/upgrade under "Available Commands" — do NOT re-add a manual list.

Task 5: EDIT internal/cmd/providers.go — FR-B6 dedup
  - REMOVE providersCmd's manual "Subcommands:\n  list …\n  show <name> …" block; keep the intro prose.
      cobra auto-lists list/show. Do NOT touch the leaf commands or their RunE.

Task 6: EDIT internal/cmd/config_test.go — pure unit tests + Execute tests
  - ADD the tests in design-decisions.md "Test plan". upgradeConfigVersion tested DIRECTLY (deterministic):
      NoVersion_Inserts (assert the inserted line + byte-preservation of other lines); OlderVersion_Updates;
      CurrentVersion_NoChange (byte-identical, changed=false); CommentedVersionIgnored; VersionInTableNotMatched
      (parse result: root config_version==2, no duplicate); Idempotent (apply twice → 2nd changed=false).
  - runConfigUpgrade via Execute (setupNoRepo + saveRootState/restoreRootState): NoFile_Errors (exit 1,
      "config init"); AddsVersion (file gains config_version=2, provider preserved, stdout "Upgraded");
      AlreadyCurrent (byte-identical, stdout "no changes"/"already", NO rewrite); OlderUpdated (version→2,
      other key preserved); Idempotent (run twice); MalformedTOML (exit 1 "not valid TOML", file unchanged);
      ExtraArgsExits1 (NoArgs); WorksOutsideRepo (implicit via setupNoRepo).
  - PATTERN: pre-write the config via os.WriteFile(config.GlobalConfigPath(), []byte(...), 0o644); drive
      rootCmd.SetArgs(["config","upgrade"]) + Execute(context.Background()); assert exitcode.For(err) +
      os.ReadFile the path. For byte-identical asserts, compare the full file content before/after.
  - GOTCHA: each Execute test wraps in saveRootState/restoreRootState. setupNoRepo isolates the path.

Task 7: VERIFY (run all gates; fix before declaring done)
  - `go build ./... && go vet ./...` clean.
  - `go test -race ./internal/cmd/ -v` → all PASS (new upgrade tests + existing init/path/providers tests).
  - `go test ./...` → GREEN (no regression).
  - `gofmt -l internal/cmd/` empty.
  - `git diff --exit-code go.mod go.sum` → empty.
  - `go run ./cmd/stagecoach config --help` → leaves listed ONCE (Available Commands); no "Subcommands:" prose.
  - `go run ./cmd/stagecoach providers --help` → same.
  - `git status` shows EXACTLY 4 files: config.go, root.go, providers.go, config_test.go. runConfigInit/
      buildBootstrapConfig/exampleConfigTemplate/default_action.go/role_defaults.go/registry.go UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// PATTERN: pure transform + I/O split (D2). upgradeConfigVersion(content, version) is the ONLY thing that
// knows the textual edit; runConfigUpgrade is the ONLY thing that touches the filesystem. Tests exercise the
// transform directly for determinism; Execute tests cover the I/O/error paths.

// PATTERN: mirror the simplest existing leaf (configPathCmd/runConfigPath) for configUpgradeCmd — NoArgs,
// SilenceErrors/Usage, RunE, exitcode.New routing, cmd.OutOrStdout printing, never os.Exit.

// CRITICAL (textual, not round-trip — D1): toml.Unmarshal is a VALIDITY GATE only. NEVER toml.Marshal back —
// it would strip comments + reorder. The write is the minimally-edited original text.

// CRITICAL (scan top-level only — D3): break the scan at the first [table] header. config_version after a
// table is a different key.

// CRITICAL (idempotency by construction — D4): the same-value branch returns content byte-identical +
// changed=false; runConfigUpgrade then prints "already at version" and does NOT rewrite. No special-case.

// GOTCHA (validity gate = map[string]any, non-strict): never reject a merely-incomplete config; only syntax
// errors fail. Reading the existing version is textual (regex), not from this map.

// GOTCHA (config.go shared with the parallel sibling): the ONE shared region is configCmd.Long. Sequence the
// sibling's edit first; this task removes the whole "Subcommands:" block afterward. Your other additions are
// independent lines. root.go + providers.go are sibling-free.
```

### Integration Points

```yaml
COMMAND REGISTRATION (config.go init()):
  - add: "configCmd.AddCommand(configUpgradeCmd) — cobra auto-lists it (FR-B6: no manual subcommand list)."

CONFIG-LOAD SKIP (root.go shouldSkipConfigLoad):
  - add: "|| name == \"upgrade\" — works outside a repo; config.Load does NOT run."

SCHEMA.VERSION (config.CurrentConfigVersion — read-only):
  - write: "upgradeConfigVersion sets/adds `config_version = CurrentConfigVersion` (the const =2)."

PATH (config.GlobalConfigPath):
  - read+write: "runConfigUpgrade reads + writes GlobalConfigPath(). --config/STAGECOACH_CONFIG NOT honored (D7)."

VALIDITY (go-toml/v2):
  - gate: "toml.Unmarshal(data, &map[string]any{}) — reject ONLY syntax errors; never marshal back."

HELP DEDUP (FR-B6):
  - config.go: "remove configCmd.Long manual 'Subcommands:' block; Mode-A intro."
  - providers.go: "remove providersCmd.Long manual 'Subcommands:' block."

GO.MODULE: change NONE. go-toml/v2 + cobra + exitcode already deps.

FROZEN/LEAVE (do NOT edit):
  - runConfigInit / buildBootstrapConfig / exampleConfigTemplate / configInitCmd flags (parallel P1.M4.T2.S1).
  - default_action.go (P4.M1); role_defaults.go (P1.M3); registry.go/builtin.go (P1.M2).
  - config/config.go (const/Defaults), config/file.go (fileConfig/load), config/load.go (advisory) — read-only.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/cmd/config.go internal/cmd/root.go internal/cmd/providers.go internal/cmd/config_test.go
go vet ./internal/cmd/
# go.mod/go.sum must be unchanged:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; gofmt -l empty.
```

### Level 2: Unit + CLI tests (the cmd suite)

```bash
# The pure transform — deterministic, all branches:
go test -race ./internal/cmd/ -run TestUpgradeConfigVersion -v
# The Execute-driven upgrade paths:
go test -race ./internal/cmd/ -run TestConfigUpgrade -v
# Full cmd suite (upgrade + existing init/path/providers; no regression):
go test -race ./internal/cmd/ -v
# Whole repo:
go test ./...
# Expected: all green. Existing TestConfigInit_*/TestConfigPath_*/TestProviders_* still pass.
```

### Level 3: Integration Testing (the real command)

```bash
make build

# --- missing file ---
( cd "$(mktemp -d)" && HOME="$PWD" XDG_CONFIG_HOME="$PWD" \
  ../../bin/stagecoach config upgrade ; echo "exit=$?" )
# Expected: exit 1, message "no config file at ... (run 'stagecoach config init' first)".

# --- adds config_version, preserves the rest ---
T=$(mktemp -d); export HOME="$T" XDG_CONFIG_HOME="$T"
mkdir -p "$T/stagecoach"
printf '[defaults]\nprovider = "pi"\nmodel = ""\n' > "$T/stagecoach/config.toml"
./bin/stagecoach config upgrade                                 # → "Upgraded ... to version 2."
grep -q '^config_version = 2$' "$T/stagecoach/config.toml" && echo "PASS: version present"
grep -q 'provider = "pi"' "$T/stagecoach/config.toml" && echo "PASS: user value preserved"
# Expected: config_version = 2 added; provider = "pi" intact; ONLY the one line differs from the input.

# --- idempotent (already current) ---
cp "$T/stagecoach/config.toml" /tmp/before
./bin/stagecoach config upgrade                                # → "... already at version 2 (no changes)."
diff /tmp/before "$T/stagecoach/config.toml" && echo "PASS: byte-identical (no rewrite)"

# --- malformed TOML is left untouched ---
printf 'bad {toml\n' > "$T/stagecoach/config.toml"
./bin/stagecoach config upgrade ; echo "exit=$?"
# Expected: exit 1, "not valid TOML"; file still "bad {toml" (NOT rewritten).

# --- works outside a git repo (run from a plain dir) ---
( cd "$(mktemp -d)" && HOME="$T" XDG_CONFIG_HOME="$T" /path/to/bin/stagecoach config upgrade )
# Expected: succeeds (shouldSkipConfigLoad("upgrade") — no git repo needed).

# Expected: all the above behave as noted.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# FR-B6 help de-dup — leaves listed ONCE, no manual "Subcommands:" prose:
go run ./cmd/stagecoach config --help    | grep -c "Subcommands:"     # expect 0
go run ./cmd/stagecoach providers --help | grep -c "Subcommands:"     # expect 0
go run ./cmd/stagecoach config --help    | grep -E "^  (init|path|upgrade) "   # expect 3 (Available Commands)
go run ./cmd/stagecoach providers --help | grep -E "^  (list|show) "           # expect 2

# Advisory round-trip: a no-version config warns; after upgrade it does not.
T=$(mktemp -d); export HOME="$T" XDG_CONFIG_HOME="$T"; mkdir -p "$T/stagecoach"
printf '[defaults]\nprovider = "pi"\n' > "$T/stagecoach/config.toml"
( cd "$(mktemp -d)" && ../../bin/stagecoach config path 2>&1 >/dev/null )   # advisory is on load — trigger any load
# After: ./bin/stagecoach config upgrade  →  the advisory's suggested remediation ran cleanly.

# Race + full regression (the gate):
go test -race ./...
go vet ./...
gofmt -l internal/ pkg/ cmd/
# Expected: all green; exactly 4 files changed (config.go, root.go, providers.go, config_test.go).
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go test -race ./internal/cmd/` green (new upgrade tests + no regression in init/path/providers).
- [ ] `go test ./...` green; `go vet ./...` clean; `gofmt -l internal/cmd/` empty.
- [ ] go.mod/go.sum UNCHANGED (go-toml/v2 + cobra + exitcode already deps).

### Feature Validation

- [ ] `config upgrade` adds/updates the top-level `config_version = 2` via a minimal textual edit; every
      other line byte-identical.
- [ ] Idempotent: an already-current file is NOT rewritten ("already at version 2 (no changes)").
- [ ] Missing file → exit 1 mentioning `config init`; malformed TOML → exit 1 "not valid TOML" (file untouched);
      extra args → exit 1 (NoArgs); works outside a git repo.
- [ ] shouldSkipConfigLoad("upgrade") true; config.Load does NOT run for it.
- [ ] FR-B6: `config --help` and `providers --help` show leaves ONCE (no manual "Subcommands:" block).

### Code Quality Validation

- [ ] `upgradeConfigVersion` is PURE (no I/O) — unit-testable without a filesystem.
- [ ] Mirrors existing patterns (configPathCmd leaf shape; config_test.go helpers).
- [ ] File placement matches the desired tree (only config.go/root.go/providers.go/config_test.go touched).
- [ ] Anti-patterns avoided (see below): no TOML round-trip, no key migrations invented, no sibling overwrite.

### Documentation & Deployment

- [ ] configUpgradeCmd.Long (Mode-A) documents in-place rewrite, preservation, idempotency, missing-file
      remediation, and that it targets the global file.
- [ ] shouldSkipConfigLoad doc comment + config.go package doc updated to mention `upgrade`.

---

## Anti-Patterns to Avoid

- ❌ Don't round-trip through TOML (unmarshal→mutate→marshal) — go-toml/v2 marshal strips ALL comments and
  reorders/reformats, violating FR-B5 "leave all other content unchanged." Do a minimal TEXTUAL edit; use
  unmarshal ONLY as a validity gate.
- ❌ Don't invent key migrations for v2.0 — there are NO removed/renamed keys ("no existing users to
  migrate"). Only the `config_version` line is touched. `upgradeConfigVersion` is the future extension point.
- ❌ Don't scan past the first `[table]` header for `config_version` — it's root metadata; a match after a
  table is a different key. Break the scan at the first `[...]` header.
- ❌ Don't reject a merely-incomplete config at the validity gate — use `map[string]any` (non-strict) so a v1
  config with only `[defaults]` passes; reject ONLY syntax errors.
- ❌ Don't overwrite runConfigInit/buildBootstrapConfig/exampleConfigTemplate or the sibling's flags — those
  belong to the parallel P1.M4.T2.S1. Coordinate ONLY on configCmd.Long (remove its "Subcommands:" block).
- ❌ Don't honor `--config`/`STAGECOACH_CONFIG` — the contract names `GlobalConfigPath()`; upgrade is in
  shouldSkipConfigLoad (config.Load doesn't run), and the global file is the documented target.
- ❌ Don't re-add a manual "Subcommands:" list after FR-B6 dedup — cobra's auto "Available Commands" is the
  single source; registering `configUpgradeCmd` makes it appear.
- ❌ Don't `os.Exit` from runConfigUpgrade or skip `exitcode.New` routing — match configPathCmd (return the
  error; main maps it to an exit code).
