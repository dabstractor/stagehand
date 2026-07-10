name: "P1.M2.T1.S1 — Add NoParentWatchdog config field with full 7-layer precedence (FR-K6)"
description: >
  Add a new `NoParentWatchdog bool toml:"no_parent_watchdog"` config field by EXACTLY MIRRORING the
  existing `NoVerify` field at all 7 precedence touch points (Config struct, Defaults, fileGeneration
  struct, materialize, overlay, loadEnv, loadGitConfig). This is the FR-K6 escape hatch for the
  parent-death watchdog (the watchdog itself + its consumer land in P1.M2.T2, NOT here). Default
  false (the watchdog runs by default). FR-K6 lists ONLY env + git-config + file — so there is NO CLI
  flag (do NOT add a root.go flag or a loadFlags entry). Naming follows the CODEBASE, not the PRD's
  loose notation: TOML key `no_parent_watchdog` (snake_case), env `STAGECOACH_NO_PARENT_WATCHDOG`
  (all-caps), git key `stagecoach.noParentWatchdog` (camelCase — git rejects underscores). Plus a
  commented doc line + env reference in the bootstrap generated-config template (snake_case — go-toml
  drops camelCase keys silently), and 4 unit tests mirroring the NoVerify tests. No consumer is wired
  here (grep must show zero production readers); P1.M2.T2.S2 adds `if !cfg.NoParentWatchdog`.

---

## Goal

**Feature Goal**: Add the `NoParentWatchdog` configuration field and resolve it through all 7
precedence layers (default → file → git-config → env → [flag omitted by design]), so that a workflow
which intentionally detaches stagecoach from a short-lived launcher (`nohup`/`setsid`/`systemd-run`) can
disable the parent-death watchdog via `STAGECOACH_NO_PARENT_WATCHDOG=1`, `stagecoach.noParentWatchdog
true`, or `[generation] no_parent_watchdog = true`. The field is a pure opt-out (default false) and is
mechanically identical to the existing `NoVerify` field.

**Deliverable** (the 7 structural touch points + 2 doc lines + 4 tests; all by mirroring `NoVerify`):
1. **config.go** — `NoParentWatchdog bool \`toml:"no_parent_watchdog"\`` struct field (+ doc comment citing §9.27 FR-K6) and `NoParentWatchdog: false` in `Defaults()`.
2. **file.go** — `NoParentWatchdog bool \`toml:"no_parent_watchdog"\`` in the `fileGeneration` struct; `if g.NoParentWatchdog { c.NoParentWatchdog = true }` in `materialize()`; `if src.NoParentWatchdog { dst.NoParentWatchdog = true }` in `overlay()`.
3. **load.go** — `STAGECOACH_NO_PARENT_WATCHDOG` handler in `loadEnv()` (DIRECT set, can be false).
4. **git.go** — `stagecoach.noParentWatchdog` (camelCase) handler in `loadGitConfig()` via `gitConfigBool`.
5. **bootstrap.go** — a commented `# no_parent_watchdog = false  # …` line in the `generationCommented` block + a `#   STAGECOACH_NO_PARENT_WATCHDOG=1   # …` line in the env-var comment block.
6. **load_test.go / git_test.go** — 4 tests mirroring the NoVerify tests (env true, env false-escape, git-config true, env-beats-git precedence).
7. **NO CLI flag** in `internal/cmd/root.go` and **NO loadFlags entry** (FR-K6 has no flag). **NO consumer wiring** (P1.M2.T2.S2 owns the `if !cfg.NoParentWatchdog` arming).

**Success Definition**:
- `cfg.NoParentWatchdog` resolves correctly from each layer: default `false`; file `[generation]
  no_parent_watchdog = true` → `true`; `git config stagecoach.noParentWatchdog true` → `true`;
  `STAGECOACH_NO_PARENT_WATCHDOG=1` → `true`; `STAGECOACH_NO_PARENT_WATCHDOG=false` → `false` even when
  git/file set it true (DIRECT-set escape hatch beats layers 1-4).
- `Config.NoParentWatchdog` is a plain `bool`, default `false`, `toml:"no_parent_watchdog"`.
- `go build ./...` clean; `GOOS=windows go build ./...` clean (plain bool, no platform tag).
- `make test` green (incl. the 4 new tests); `make lint` clean; `make coverage-gate` ≥85%.
- `gofmt -l` empty on all edited files; the bootstrap template still decodes as valid TOML.
- Grep guards: exactly one `NoParentWatchdog` field decl; ZERO `--no-parent-watchdog` flag; ZERO
  production `cfg.NoParentWatchdog` readers (consumer is P1.M2.T2.S2).

## User Persona (if applicable)

**Target User**: A developer/operator who launches stagecoach detached from a short-lived parent
(`nohup stagecoach …`, `setsid`, a systemd unit, a service manager) — the exact workflows FR-K6 names.

**Use Case**: Those launchers exit immediately by design, which would trip the parent-death watchdog
(`getppid()` changes → rescue). `NoParentWatchdog` is the opt-out that says "I know; don't self-exit."

**User Journey**: `STAGECOACH_NO_PARENT_WATCHDOG=1 stagecoach …` (or `git config
stagecoach.noParentWatchdog true`, or `[generation] no_parent_watchdog = true` in the config file) →
the run does NOT arm the watchdog → a legitimately-detached run completes instead of being torn down
on launcher exit. (The arming logic itself is P1.M2.T2.S2; this task only makes the flag resolvable.)

**Pain Points Addressed**: FR-K6 — without an opt-out, the watchdog (FR-K1) would false-positive on
every intentional detach, killing legitimate backgrounded runs.

## Why

- **FR-K6 / §9.27**: the parent-death watchdog (FR-K1, P1.M2.T2) must have an escape hatch for
  intentional detachment. This task supplies the *configuration surface* the watchdog will gate on.
  It is a prerequisite for P1.M2.T2.S2 (`if !cfg.NoParentWatchdog { watchdog.Arm(...) }`).
- **Consistency**: the field mirrors `NoVerify` (a proven, tested plain-`bool` opt-out) at every
  precedence layer — same only-true-propagates file semantics, same DIRECT-set env escape hatch, same
  camelCase git key. No new pattern, no new type.
- **Bounded scope**: 7 mechanical copy-edits + 2 doc lines + 4 tests. No flag, no consumer, no
  migration, no version bump. It lands independently of the watchdog (P1.M2.T2), the lock diagnostics
  (P1.M3), and the docs sync (P1.M4.T2).

## What

**User-visible behavior**: None directly yet (the consumer is a later task). Internally,
`cfg.NoParentWatchdog` becomes resolvable from all precedence layers and defaults to `false`.

**Technical change**: an exact 7-point copy of `NoVerify` (renamed), plus 2 commented doc lines in the
bootstrap template, plus 4 tests. See the Implementation Blueprint for verbatim before/after.

### Success Criteria
- [ ] `Config.NoParentWatchdog bool \`toml:"no_parent_watchdog"\`` exists (config.go) with a §9.27 FR-K6 doc comment.
- [ ] `Defaults()` sets `NoParentWatchdog: false`.
- [ ] `fileGeneration.NoParentWatchdog bool \`toml:"no_parent_watchdog"\`` exists (file.go).
- [ ] `materialize()` propagates `if g.NoParentWatchdog { c.NoParentWatchdog = true }` (only-true).
- [ ] `overlay()` propagates `if src.NoParentWatchdog { dst.NoParentWatchdog = true }` (only-true).
- [ ] `loadEnv()` handles `STAGECOACH_NO_PARENT_WATCHDOG` (DIRECT set, can be false; bad bool → wrapped error).
- [ ] `loadGitConfig()` handles `stagecoach.noParentWatchdog` (camelCase) via `gitConfigBool`.
- [ ] `bootstrap.go` `generationCommented` block has `# no_parent_watchdog = false  # …` (SNAKE_CASE).
- [ ] `bootstrap.go` env-var block has `#   STAGECOACH_NO_PARENT_WATCHDOG=1   # …`.
- [ ] 4 new tests pass (env true / env false-escape / git-config true / env-beats-git precedence).
- [ ] NO `--no-parent-watchdog` flag in root.go; NO loadFlags entry.
- [ ] ZERO production `cfg.NoParentWatchdog` readers (consumer is P1.M2.T2.S2).
- [ ] `make test` + `make lint` + `make coverage-gate` pass; `gofmt -l` empty; cross-build clean.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact 7-point copy template (verbatim before/after for each touch point), the three naming
decisions (TOML snake_case / env all-caps / git camelCase) with the rationale that the PRD prose is
non-literal, the empirically-verified go-toml gotcha that forces snake_case in the bootstrap template,
the test idioms to clone (with line numbers), the explicit "no flag / no consumer / no migration"
fences, and the placement strategy (anchor each edit next to its `NoVerify` sibling).

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim code + gotchas for every touch point)
- docfile: plan/014_37208f58ffa2/P1M2T1S1/research/findings.md
  why: "§0 is the 7-point copy table; §1 has verbatim before/after for each touch point with exact
        line numbers; §2 the naming decisions; §3 the go-toml key-spelling gotcha; §4 the bootstrap
        doc additions; §5 the tests to clone; §6 the scope fences."
  critical: "§3: go-toml/v2 (v2.4.2) matches case-insensitively but WORD-SEPARATION-SENSITIVELY — a
             camelCase key is SILENTLY DROPPED. The bootstrap commented key MUST be snake_case
             no_parent_watchdog (NOT the item's literal noParentWatchdog, NOT PRD.md:1709)."

# MUST READ — the FR-K6 design + the exact mirror-table
- docfile: plan/014_37208f58ffa2/architecture/watchdog_config.md
  why: "The 'FR-K6: no_parent_watchdog config field' section gives the 7-row NoVerify→NoParentWatchdog
        copy table and the naming decisions (env all-caps, git camelCase, no flag)."
  critical: "It confirms NO CLI flag (FR-K6 = env + git + file only) and that the consumer
             (watchdog arming) is a LATER task — this subtask adds only the resolvable field."

# MUST READ — the 7 source touch points (each is anchored next to its NoVerify sibling)
- file: internal/config/config.go
  why: "Touch points 1 (struct field, ~136-142) and 2 (Defaults, ~214). NoVerify is the copy template
        for both. Add NoParentWatchdog IMMEDIATELY ADJACENT to NoVerify in each."
  pattern: "Struct field: `NoVerify bool \`toml:\"no_verify\"\`` with a multi-line doc comment citing
            the FR + the 5-layer precedence + the only-true-propagates file-layer limitation. Defaults:
            `NoVerify: false, // §… default (…)` aligned in the Defaults() literal."
  gotcha: "Defaults() is a struct literal — adding a field is mandatory (else the zero value false
           happens to be correct, but EVERY peer field is listed explicitly; match the convention)."

- file: internal/config/file.go
  why: "Touch points 3 (fileGeneration struct, ~68), 4 (materialize, ~298-300), 5 (overlay, ~362-364).
        fileGeneration is the [generation] TOML table struct (file.go:46) — so no_parent_watchdog is a
        [generation] KEY in a config file (NOT [defaults])."
  pattern: "materialize/overlay use only-true-propagates: `if g.NoVerify { c.NoVerify = true }` and
            `if src.NoVerify { dst.NoVerify = true }`. NoParentWatchdog is identical (plain bool, default
            false ⇒ only-true is harmless, per the file.go:393 note)."
  gotcha: "Do NOT reorder fields. Add NoParentWatchdog as a new line adjacent to NoVerify; let gofmt fix
           column alignment. materialize takes the fileGeneration receiver `g`; overlay takes src/dst."

- file: internal/config/load.go
  why: "Touch point 6 (loadEnv STAGECOACH_NO_VERIFY handler, ~317-326). The DIRECT-set escape hatch
        that can set false (unlike file/git only-true). os/strconv/fmt already imported — NO new import."
  pattern: "presence-semantic: `if v, ok := os.LookupEnv(\"STAGECOACH_NO_VERIFY\"); ok && v != \"\" { …
            strconv.ParseBool … cfg.NoVerify = b // DIRECT set }`. Bad bool → `fmt.Errorf(\"STAGECOACH_NO_VERIFY: %w\", err)`."
  gotcha: "Do NOT add a flag handler. The --no-verify flag block is at load.go:475-486 — FR-K6 has NO
           flag, so there is NO loadFlags entry and NO `fs.Changed(\"no-parent-watchdog\")`."

- file: internal/config/git.go
  why: "Touch point 7 (loadGitConfig stagecoach.noVerify handler, ~180-186). camelCase git key."
  pattern: "`if v, found, err := gitConfigBool(repoDir, \"stagecoach.noVerify\"); err != nil { return nil, err
            } else if found { c.NoVerify = v }`. The comment at git.go:180 explains git rejects underscores
            in the final key segment (the codebase's own rationale)."
  gotcha: "Use stagecoach.noParentWatchdog (camelCase) — NOT stagecoach.no_parent_watchdog. The latter is
           rejected by git as `invalid key` (mirrors the noVerify/autoStageAll convention)."

# MUST READ — the bootstrap generated-config template (PRD Mode A docs)
- file: internal/config/bootstrap.go
  why: "Two doc additions: (a) env-var block ~250-262 → add `#   STAGECOACH_NO_PARENT_WATCHDOG=1  # …`;
        (b) generationCommented block ~293-305 → add `# no_parent_watchdog = false  # …` (the [generation]
        commented keys)."
  pattern: "Both blocks are raw-string `const` templates of comment lines. Match the surrounding
            indentation/column style. generationCommented keys are ALL snake_case (max_diff_bytes,
            multi_turn_fallback, …)."
  gotcha: "SNAKE_CASE no_parent_watchdog only — go-toml silently drops camelCase (see findings §3).
           bootstrap_test.go is substring/valid-TOML based (NOT byte-exact) → adding a commented line is safe."

# MUST READ — the tests to clone (the 4 idioms)
- file: internal/config/load_test.go
  why: "Clone TestLoadEnv (the big env test @151, incl. the NoVerify assertion @171-172) for the env=true
        case; clone TestLoadEnv_BoolFalseEscape (@189-204) for the =false escape; clone
        TestLoad_NoVerifyPrecedence (@1686-1717) for the env-beats-git precedence test."
  pattern: "loadEnv-level: cfg := Defaults() → t.Setenv → err := loadEnv(&cfg) → assert cfg.<Field>.
            Load-level: loadEnvSetup(t) + chdir(t,repo) + Load(ctx, LoadOpts{RepoDir:repo, DisableBootstrap:true})."
  critical: "Do NOT clone TestLoadFlags_NoVerify (@1566) — there is no flag. The new tests cover env + git
             + precedence only."

- file: internal/config/git_test.go
  why: "The loadGitConfig test (~80-136) sets every git key and asserts; clone the noVerify lines
        (setGitConfig @88-90 + assertion @134-136) for noParentWatchdog."
  pattern: "setGitConfig(t, repo, \"stagecoach.noParentWatchdog\", \"true\") inside the existing
            loadGitConfig test, then assert cfg.NoParentWatchdog."

# CONTEXT — the consumer (LANDS LATER, not here)
- file: internal/cmd/default_action.go
  why: "P1.M2.T2.S2 will add `if !cfg.NoParentWatchdog { watchdog.Arm(ctx, 1*time.Second) }` after
        lock.Acquire. NOT this task. Cited so the PRP can state 'consumer lands later'."
  critical: "Do NOT add this arming call now. After this subtask, grep must show ZERO production
             cfg.NoParentWatchdog readers (only the new tests + the field wiring)."

# CONTEXT — the parallel sibling (no overlap)
- docfile: plan/014_37208f58ffa2/P1M1T2S1/PRP.md
  why: "PARALLEL sibling that edits internal/signal/* only (signal.Trigger export). It does NOT touch
        internal/config/*. No file overlap → no conflict. Read to confirm the non-overlap."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go       # EDIT — struct field (~142) + Defaults (~214)
  file.go         # EDIT — fileGeneration struct (~68) + materialize (~299) + overlay (~363)
  load.go         # EDIT — loadEnv STAGECOACH_NO_PARENT_WATCHDOG handler (~326, after NO_VERIFY)
  git.go          # EDIT — loadGitConfig stagecoach.noParentWatchdog handler (~186, after noVerify)
  bootstrap.go    # EDIT — generationCommented [generation] doc line + env-var block line
  load_test.go    # EDIT — +env-true, +env-false-escape, +precedence tests
  git_test.go     # EDIT — +stagecoach.noParentWatchdog set+assert in loadGitConfig test
  migrate.go      # READ-ONLY — field-specific migration only; NO change needed
internal/cmd/
  root.go         # READ-ONLY — NO flag added (FR-K6 has none)
  default_action.go  # READ-ONLY — consumer (if !cfg.NoParentWatchdog) is P1.M2.T2.S2, NOT here
  config.go       # READ-ONLY — exampleConfigTemplate (separate, byte-exact test) OUT OF SCOPE
go.mod            # READ-ONLY — go-toml/v2 v2.4.2 (case-insensitive, word-separation-sensitive)
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
internal/config/config.go      # +1 struct field (+doc comment) +1 Defaults entry
internal/config/file.go        # +1 fileGeneration field +1 materialize block +1 overlay block
internal/config/load.go        # +1 loadEnv handler (STAGECOACH_NO_PARENT_WATCHDOG)
internal/config/git.go         # +1 loadGitConfig handler (stagecoach.noParentWatchdog)
internal/config/bootstrap.go   # +1 [generation] commented key +1 env-var comment line
internal/config/load_test.go   # +2-3 tests (env true/false-escape + precedence)
internal/config/git_test.go    # +1 set+assert (extend the existing loadGitConfig test)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (go-toml key spelling — empirically verified): go-toml/v2 (v2.4.2) matches keys
// case-INSENSITIVELY but WORD-SEPARATION-SENSITIVELY (lowercases key + tag, compares; underscores are
// significant). A camelCase `noParentWatchdog` key in a config file is SILENTLY DROPPED (its lowercase
// `noparentwatchdog` ≠ `no_parent_watchdog`). So EVERY commented doc key in bootstrap.go MUST be
// snake_case `no_parent_watchdog` (matching the toml tag). The item description's literal
// `# noParentWatchdog = false` and PRD.md:1709 are WRONG — use snake_case or the opt-out silently fails.

// CRITICAL (naming — follow the codebase, not PRD prose): TOML key = no_parent_watchdog (snake_case,
// every toml tag is snake_case). Env var = STAGECOACH_NO_PARENT_WATCHDOG (all-caps, every env var is
// STAGECOACH_*). Git key = stagecoach.noParentWatchdog (camelCase — git REJECTS underscores in the
// final segment, per git.go:180's own comment; matches noVerify/autoStageAll). The PRD writes
// stagecoach_NO_PARENT_WATCHDOG / noParentWatchdog as CONCEPTUAL notation (it also writes
// stagecoach_NO_VERIFY for a var whose real code is STAGECOACH_NO_VERIFY). Follow the code.

// CRITICAL (NO CLI flag): FR-K6 lists env + git-config + file ONLY. Do NOT add a BoolVarP in root.go
// and do NOT add a fs.Changed("no-parent-watchdog") block in loadFlags. (NoVerify HAS a flag at
// load.go:475-486 + root.go; NoParentWatchdog does NOT — that is the one deliberate divergence.)

// CRITICAL (NO consumer wiring): do NOT add `if !cfg.NoParentWatchdog { watchdog.Arm(...) }` anywhere.
// That arming call (default_action.go, post lock.Acquire) is P1.M2.T2.S2. After this subtask the field
// has ZERO production readers (grep guard confirms exactly 0 outside config/* + tests).

// GOTCHA (only-true-propagates is harmless here): NoParentWatchdog is a plain bool default false, so
// the file/git only-true-propagates layers cannot turn a true back to false — but that is the SAME as
// NoVerify/Push and is correct (the env layer is the escape hatch that CAN set false). Do not "fix"
// this into a *bool (that would be a cross-cutting change; the task says mirror NoVerify = plain bool).

// GOTCHA (anchor next to NoVerify, not by line number): place every edit IMMEDIATELY ADJACENT to its
// NoVerify sibling (same struct/function/block). Parallel tasks don't touch config/*, but incidental
// drift is absorbed by the adjacency anchor. Do NOT reorder existing fields.

// GOTCHA (Defaults is a struct literal): every peer field is listed explicitly in Defaults() — you MUST
// add `NoParentWatchdog: false,` even though the zero value is already false (consistency + it makes the
// default documented/intentional, matching NoVerify: false).
```

## Implementation Blueprint

### Data models and structure

No new types. One new plain-`bool` field `NoParentWatchdog` added to two structs (`Config`,
`fileGeneration`) with the same `toml:"no_parent_watchdog"` tag, resolved through the existing
7-layer precedence (default → file → git-config → env → [no flag]). Mechanically identical to `NoVerify`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/config.go — struct field (touch point 1)
  - LOCATE the NoVerify field (grep -n 'NoVerify bool' config.go → ~142). Read its 6-line doc comment.
  - ADD immediately after the NoVerify field (same struct, next field):
        // NoParentWatchdog is the §9.27 FR-K6 opt-out for the parent-death watchdog (FR-K1, P1.M2.T2).
        // When true, stagecoach does NOT arm the watchdog that self-exits the run when its launcher dies
        // (the lazygit/IDE/detaching-terminal case). Default false — the watchdog runs by default; set this
        // only for intentional detachment (nohup/setsid/systemd-run). Resolution (FR-K6 = env + git + file,
        // NO flag): STAGECOACH_NO_PARENT_WATCHDOG / stagecoach.noParentWatchdog / [generation].no_parent_watchdog.
        // FILE LAYER LIMITATION (same as NoVerify/Push): only-true-propagates — a file setting
        // no_parent_watchdog = false is a no-op; the env layer can set it false (escape hatch).
        NoParentWatchdog bool `toml:"no_parent_watchdog"`
  - NAMING: NoParentWatchdog (PascalCase field), toml:"no_parent_watchdog" (snake_case tag).
  - PRESERVE: the NoVerify field and all surrounding fields/sections.

Task 2: EDIT internal/config/config.go — Defaults() (touch point 2)
  - LOCATE: grep -n 'NoVerify:             false' config.go → ~214.
  - ADD immediately after it (aligned to the same column):
        NoParentWatchdog:    false,            // §9.27 FR-K6 default (watchdog runs by default)
  - (Let gofmt adjust column alignment across the literal if needed: `gofmt -w config.go`.)

Task 3: EDIT internal/config/file.go — fileGeneration struct (touch point 3)
  - LOCATE: grep -n 'NoVerify             bool' file.go → ~68 (inside type fileGeneration struct).
  - ADD a new field adjacent to NoVerify (e.g. after HookTimeout, or right after NoVerify):
        NoParentWatchdog    bool     `toml:"no_parent_watchdog"` // §9.27 FR-K6 — only-true-propagates (mirrors NoVerify/Push)
  - NOTE: fileGeneration is the [generation] TOML table (file.go:46) → no_parent_watchdog is a [generation] key.
  - Run gofmt -w to fix column alignment.

Task 4: EDIT internal/config/file.go — materialize() (touch point 4)
  - LOCATE: grep -n 'if g.NoVerify' file.go → ~299 (inside func materialize).
  - ADD immediately after the `if g.NoVerify { c.NoVerify = true }` block:
        // §9.27 FR-K6 — no_parent_watchdog from file (only-true-propagates, mirrors NoVerify/Push).
        if g.NoParentWatchdog {
            c.NoParentWatchdog = true
        }

Task 5: EDIT internal/config/file.go — overlay() (touch point 5)
  - LOCATE: grep -n 'if src.NoVerify' file.go → ~363 (inside func overlay).
  - ADD immediately after the `if src.NoVerify { dst.NoVerify = true }` block:
        // §9.27 FR-K6 — no_parent_watchdog (only-true-propagates, same as NoVerify/Push)
        if src.NoParentWatchdog {
            dst.NoParentWatchdog = true
        }

Task 6: EDIT internal/config/load.go — loadEnv() STAGECOACH_NO_PARENT_WATCHDOG (touch point 6)
  - LOCATE: grep -n 'STAGECOACH_NO_VERIFY' load.go → ~317-326 (the loadEnv handler).
  - ADD immediately after the STAGECOACH_NO_VERIFY block:
        // §9.27 FR-K6 — no_parent_watchdog via env (presence-semantic, DIRECT set — can be false, the escape hatch).
        if v, ok := os.LookupEnv("STAGECOACH_NO_PARENT_WATCHDOG"); ok && v != "" {
            b, err := strconv.ParseBool(v)
            if err != nil {
                return fmt.Errorf("STAGECOACH_NO_PARENT_WATCHDOG: %w", err)
            }
            cfg.NoParentWatchdog = b // DIRECT set — can be false (escape hatch, mirrors STAGECOACH_NO_VERIFY)
        }
  - NO IMPORT CHANGES (os/strconv/fmt already imported).
  - DO NOT touch loadFlags (load.go:475-486) — FR-K6 has no flag.

Task 7: EDIT internal/config/git.go — loadGitConfig() stagecoach.noParentWatchdog (touch point 7)
  - LOCATE: grep -n 'stagecoach.noVerify' git.go → ~180-186 (the loadGitConfig handler).
  - ADD immediately after the stagecoach.noVerify block:
        // §9.27 FR-K6 — noParentWatchdog via git config (camelCase key, same convention as noVerify).
        if v, found, err := gitConfigBool(repoDir, "stagecoach.noParentWatchdog"); err != nil {
            return nil, err
        } else if found {
            c.NoParentWatchdog = v
        }
  - camelCase key (NOT snake_case — git rejects underscores in the final segment).

Task 8: EDIT internal/config/bootstrap.go — env-var block + generationCommented (PRD Mode A docs)
  - (a) ENV-VAR BLOCK: grep -n 'STAGECOACH_NO_COLOR' bootstrap.go → ~255. Add after it (or at the list end ~262):
        #   STAGECOACH_NO_PARENT_WATCHDOG=1   # opt out of the parent-death lock watchdog (§9.27 FR-K6)
  - (b) generationCommented BLOCK: grep -n 'multi_turn_chunk_tokens' bootstrap.go → ~304. Add a new commented
        [generation] key line (SNAKE_CASE — go-toml drops camelCase):
        # no_parent_watchdog    = false  # opt out of the parent-death lock watchdog — set true if you launch via nohup/setsid/systemd-run (§9.27 FR-K6)
  - MATCH the surrounding indentation/column style of each block.

Task 9: ADD tests (mirror NoVerify tests)
  - (a) internal/config/load_test.go — extend TestLoadEnv (the big env test @151): add
        t.Setenv("STAGECOACH_NO_PARENT_WATCHDOG", "true") and assert cfg.NoParentWatchdog (mirror the
        NoVerify assertion @171-172). OR add a focused TestLoadEnv_NoParentWatchdog func.
  - (b) internal/config/load_test.go — extend TestLoadEnv_BoolFalseEscape (@189): add
        t.Setenv("STAGECOACH_NO_PARENT_WATCHDOG", "false") on a Config{NoParentWatchdog: true} start,
        assert cfg.NoParentWatchdog==false (DIRECT-set escape hatch). OR a focused func.
  - (c) internal/config/git_test.go — in the existing loadGitConfig test (~80-136): add
        setGitConfig(t, repo, "stagecoach.noParentWatchdog", "true") near line 90, and an assertion
        `if !cfg.NoParentWatchdog { t.Errorf("NoParentWatchdog=false want true (stagecoach.noParentWatchdog set)") }` near line 135.
  - (d) internal/config/load_test.go — add TestLoad_NoParentWatchdogPrecedence (clone TestLoad_NoVerifyPrecedence @1686):
        loadEnvSetup(t)+chdir(t,repo); setGitConfig(... "stagecoach.noParentWatchdog","true"); Load → assert true;
        then t.Setenv("STAGECOACH_NO_PARENT_WATCHDOG","false"); Load → assert false (env beats git, escape).
  - NAMING: NoParentWatchdog in assertions (matches the field). NO flag test.
  - COVERAGE: env true / env false-escape / git true / env-beats-git precedence.

Task 10: VERIFY — build (native+cross), vet, format, focused + full tests, lint, coverage, grep guards
  - go build ./... ; GOOS=windows go build ./... ; GOOS=linux go build ./...
  - go vet ./internal/config/...
  - gofmt -l internal/config/*.go   # must be empty
  - go test ./internal/config/ -run 'NoParentWatchdog' -v
  - make test ; make lint ; make coverage-gate
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the env handler (loadEnv) — presence-semantic DIRECT set, mirrors STAGECOACH_NO_VERIFY
if v, ok := os.LookupEnv("STAGECOACH_NO_PARENT_WATCHDOG"); ok && v != "" {
    b, err := strconv.ParseBool(v)
    if err != nil {
        return fmt.Errorf("STAGECOACH_NO_PARENT_WATCHDOG: %w", err)
    }
    cfg.NoParentWatchdog = b // DIRECT set — can be false (the escape hatch that beats file/git only-true)
}

// PATTERN: the git-config handler (loadGitConfig) — camelCase key, mirrors stagecoach.noVerify
if v, found, err := gitConfigBool(repoDir, "stagecoach.noParentWatchdog"); err != nil {
    return nil, err
} else if found {
    c.NoParentWatchdog = v
}

// PATTERN: materialize/overlay — only-true-propagates (a file/git layer cannot set false; plain bool)
if g.NoParentWatchdog { c.NoParentWatchdog = true }   // materialize (receiver g)
if src.NoParentWatchdog { dst.NoParentWatchdog = true } // overlay (src→dst)

// PATTERN: the precedence test (clone TestLoad_NoVerifyPrecedence)
setGitConfig(t, repo, "stagecoach.noParentWatchdog", "true")     // layer 4 sets true
cfg, _ := Load(ctx, LoadOpts{RepoDir: repo, DisableBootstrap: true})
// assert cfg.NoParentWatchdog == true
t.Setenv("STAGECOACH_NO_PARENT_WATCHDOG", "false")               // layer 5 DIRECT-set beats layer 4
cfg, _ = Load(ctx, LoadOpts{RepoDir: repo, DisableBootstrap: true})
// assert cfg.NoParentWatchdog == false (env escape hatch)
```

### Integration Points

```yaml
NO database / migration / routes / new types / import changes / new flag. One plain-bool field mirrored
across the existing 7-layer precedence + 2 template doc lines + 4 tests.

CONFIG (internal/config):
  - config.go:   +field NoParentWatchdog (struct) +NoParentWatchdog: false (Defaults)
  - file.go:     +field (fileGeneration) +materialize block +overlay block   ([generation] table)
  - load.go:     +STAGECOACH_NO_PARENT_WATCHDOG handler (loadEnv)            (NO loadFlags entry)
  - git.go:      +stagecoach.noParentWatchdog handler (loadGitConfig)        (camelCase key)
  - bootstrap.go:+[generation] commented key (snake_case) +env-var comment line

PRECEDENCE (resolved by Load, unchanged model): default(false) < file[geneation] < git-config < env < [no flag]
  - file & git are only-true-propagates (plain bool default false ⇒ harmless, like NoVerify/Push).
  - env is DIRECT-set (can be false — the FR-K6 escape hatch).

DOWNSTREAM (this subtask ENABLES but does NOT build):
  - P1.M2.T2.S2 (watchdog arming) — will add `if !cfg.NoParentWatchdog { watchdog.Arm(ctx, 1*time.Second) }`
    in default_action.go after lock.Acquire. NO production caller exists after this subtask (expected).

SCOPE FENCES: NO root.go flag; NO loadFlags entry; NO default_action.go consumer; NO internal/cmd/config.go
  exampleConfigTemplate edit (byte-exact golden test config_test.go:438 — defer to P1.M4.T2); NO migration;
  NO config-version bump; NO README/docs (P1.M4.T2).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross build (the field is a plain bool with no platform tag — must build everywhere).
go build ./...
GOOS=windows go build ./...
GOOS=linux   go build ./...
# Expected: all clean. If GOOS=windows fails you added a platform-tagged symbol by mistake.

# Vet.
go vet ./internal/config/...

# Format — CRITICAL: every edited file must be gofmt-clean (struct/Defaults column alignment).
gofmt -l internal/config/*.go
# Expected: empty. If a file is listed: gofmt -w internal/config/<file>.

# Lint.
make lint      # golangci-lint v1.61 (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. unused would fire if a field were never read — but it IS read by the new tests,
#           so it stays clean. (The production reader lands in P1.M2.T2.S2; tests are sufficient here.)

# Scope guard: only internal/config files changed.
git diff --name-only
# Expected: internal/config/{config,file,load,git,bootstrap,load_test,git_test}.go (only).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new tests (focused).
go test ./internal/config/ -run 'NoParentWatchdog' -v
# Expected: PASS — env=true, env=false-escape, git-config=true, env-beats-git precedence.

# Regression: NoVerify tests still green (the copy source must be untouched behaviorally).
go test ./internal/config/ -run 'NoVerify|Verbose|Precedence|Push' -v

# Bootstrap template still valid TOML + substring checks (the new commented line must not break parsing).
go test ./internal/config/ -run 'Bootstrap' -v

# Full race suite.
make test
# Expected: green (race detector).

# Coverage gate (PRD §20.3: ≥85% on internal/{git,provider,generate,config}).
make coverage-gate
# Expected: passes (the new field + handlers + tests ADD coverage).
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary (sanity — the package must still build into the binary).
make build

# Manual proof that the field resolves end-to-end through Load (the real precedence path).
# Scratch repo + a config file that sets the [generation] key, then Load via --dry-run.
d=$(mktemp -d) && cd "$d" && git init -q
git config user.email t@t.com && git config user.name t
printf 'a\n' > f.txt && git add f.txt && git commit -qm init
printf 'b\n' >> f.txt && git add f.txt

# Layer 3 (git-config): camelCase key resolves.
git config stagecoach.noParentWatchdog true
SC=/home/dustin/projects/stagecoach/bin/stagecoach
"$SC" --dry-run 2>&1 | head -1   # runs without error (the field resolves; no consumer yet so no behavior change)
git config --unset stagecoach.noParentWatchdog

# Layer 2 (file): snake_case [generation] key resolves (proves go-toml parses it).
printf '[generation]\nno_parent_watchdog = true\n' > .stagecoach.toml
"$SC" --dry-run 2>&1 | head -1   # runs without error
rm .stagecoach.toml

# Layer 4 (env): all-caps var resolves.
STAGECOACH_NO_PARENT_WATCHDOG=1 "$SC" --dry-run 2>&1 | head -1   # runs without error
cd - && rm -rf "$d"

# (There is no observable BEHAVIOR yet — the watchdog arming is P1.M2.T2.S2. Level 2 unit tests are the
#  within-scope proof that the field resolves from every layer; the e2e "opt-out suppresses the watchdog"
#  scenario is P1.M4.T1.S1.)
```

> **Note**: a `--dry-run` run reaches config `Load` (and thus every layer incl. loadEnv/loadGitConfig),
> so it exercises the wiring end-to-end. There is no visible behavior change because the consumer is a
> later task — the unit tests + a clean `--dry-run` are the within-scope proof.

### Level 4: Creative & Domain-Specific Validation

```bash
# Scope guard 1: exactly one NoParentWatchdog field declaration (in Config) + one in fileGeneration.
grep -rn 'NoParentWatchdog\s*bool' internal/config/
# Expected: 2 hits — config.go (Config) and file.go (fileGeneration).

# Scope guard 2: NO CLI flag registered.
grep -rn 'no-parent-watchdog\|NoParentWatchdog' internal/cmd/
# Expected: ZERO hits (no BoolVarP in root.go, no fs.Changed in loadFlags). The consumer is P1.M2.T2.S2.

# Scope guard 3: ZERO production READERS of cfg.NoParentWatchdog (consumer lands later).
grep -rn 'cfg\.NoParentWatchdog\|c\.NoParentWatchdog' --include='*.go' internal/ cmd/ pkg/ | grep -v '_test.go'
# Expected: only config-package WRITES (Defaults/materialize/overlay/loadEnv/git). ZERO reads in
#           internal/cmd, internal/hooks, pkg/. (The arming `if !cfg.NoParentWatchdog` is P1.M2.T2.S2.)

# Scope guard 4: the bootstrap key is SNAKE_CASE (go-toml drops camelCase — the critical gotcha).
grep -n 'no_parent_watchdog' internal/config/bootstrap.go
# Expected: the commented [generation] line uses snake_case `no_parent_watchdog` (NOT noParentWatchdog).
grep -n 'noParentWatchdog' internal/config/bootstrap.go
# Expected: ZERO hits (camelCase must NOT appear in the generated template).

# Scope guard 5: git key is camelCase (git rejects underscores).
grep -n 'stagecoach.noParentWatchdog' internal/config/git.go
# Expected: 1 hit (the loadGitConfig handler). Confirm NO 'stagecoach.no_parent_watchdog' snake_case git key.

# Scope guard 6: env var is all-caps.
grep -n 'STAGECOACH_NO_PARENT_WATCHDOG' internal/config/load.go internal/config/bootstrap.go
# Expected: the loadEnv handler (load.go) + the env-var comment (bootstrap.go).

# Scope guard 7: Defaults() lists the field (struct literal completeness).
grep -n 'NoParentWatchdog:' internal/config/config.go
# Expected: 1 hit in Defaults() (`NoParentWatchdog: false,`).

# Scope guard 8: bootstrap template still valid TOML (the commented line didn't break the [generation] table).
go test ./internal/config/ -run 'Bootstrap' -v
# Expected: PASS (parses + substring checks).

# Scope guard 9: NoVerify (the copy source) is behaviorally untouched.
go test ./internal/config/ -run 'NoVerify' -v
# Expected: all pre-existing NoVerify tests still PASS.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=windows/linux go build ./...` clean
- [ ] `go vet ./internal/config/...` clean
- [ ] `gofmt -l internal/config/*.go` empty
- [ ] `make lint` zero errors (no `unused` — field is read by tests)
- [ ] `make test` (race) green, incl. the 4 new tests
- [ ] `make coverage-gate` ≥85% on `internal/config`

### Feature Validation
- [ ] `cfg.NoParentWatchdog` resolves: default false; file `[generation] no_parent_watchdog=true` → true;
      `stagecoach.noParentWatchdog true` → true; `STAGECOACH_NO_PARENT_WATCHDOG=1` → true; `=false` → false
      even when git/file set true (env DIRECT-set escape)
- [ ] 4 new tests pass (env true / env false-escape / git true / env-beats-git precedence)
- [ ] bootstrap template still decodes as valid TOML; commented key is snake_case

### Scope-Boundary Validation
- [ ] `git diff --name-only` == only `internal/config/*.go` (7 files)
- [ ] NO `--no-parent-watchdog` flag in `internal/cmd/root.go`; NO loadFlags entry in load.go
- [ ] ZERO production `cfg.NoParentWatchdog` readers (consumer is P1.M2.T2.S2)
- [ ] NO `internal/cmd/config.go` exampleConfigTemplate edit (byte-exact test; defer to P1.M4.T2)
- [ ] NO migration / NO config-version bump (backward-compatible default-false field)
- [ ] NoVerify (copy source) tests still green (no behavioral change to it)

### Code Quality & Docs
- [ ] Each edit anchored next to its NoVerify sibling (reviewable, drift-safe)
- [ ] TOML key snake_case; env all-caps; git key camelCase (codebase convention, not PRD prose)
- [ ] bootstrap commented key is snake_case `no_parent_watchdog` (go-toml drops camelCase)
- [ ] Doc comments cite §9.27 FR-K6 and note "no flag / env+git+file only"
- [ ] Tests clone the NoVerify idioms (no new helpers, no flag test)

---

## Anti-Patterns to Avoid

- ❌ Don't use camelCase `noParentWatchdog` for the bootstrap/template commented key. go-toml/v2
  (v2.4.2) matches case-insensitively but WORD-SEPARATION-SENSITIVELY — camelCase is silently dropped,
  so a user uncommenting it would NOT disable the watchdog. Use snake_case `no_parent_watchdog` (the
  item description's literal and PRD.md:1709 are both wrong on this). [empirically verified]
- ❌ Don't use a snake_case git key `stagecoach.no_parent_watchdog`. Git REJECTS underscores in the
  final config-key segment (`error: invalid key`); the codebase convention (and git.go:180's own
  comment) is camelCase — `stagecoach.noParentWatchdog`.
- ❌ Don't use the PRD's lowercase-prefixed `stagecoach_NO_PARENT_WATCHDOG` env var. Every env var in
  the codebase is all-caps `STAGECOACH_*` (e.g. `STAGECOACH_NO_VERIFY`). Use `STAGECOACH_NO_PARENT_WATCHDOG`.
- ❌ Don't add a CLI flag (`--no-parent-watchdog`) or a `loadFlags` entry. FR-K6 lists env + git-config +
  file ONLY — no flag. (NoVerify has a flag; NoParentWatchdog deliberately does NOT. This is the ONE
  divergence from the copy template.)
- ❌ Don't wire the consumer (`if !cfg.NoParentWatchdog { watchdog.Arm(...) }`). That arming call is
  P1.M2.T2.S2 (default_action.go, post lock.Acquire). This subtask adds only the resolvable field;
  grep must show zero production readers afterward.
- ❌ Don't promote the field to `*bool`. The task says mirror `NoVerify` (plain bool, default false).
  The only-true-propagates file/git layers cannot turn true→false, but that's identical to NoVerify/Push
  and is correct (the env layer is the escape hatch). A `*bool` would be a cross-cutting change.
- ❌ Don't edit `internal/cmd/config.go`'s `exampleConfigTemplate`. It has a byte-exact golden test
  (`config_test.go:438`) and is out of this item's scope — defer any template-doc parity to P1.M4.T2.
- ❌ Don't add a migration or bump `CurrentConfigVersion`. A default-false optional field is backward-
  compatible (old files decode fine, field stays false; go-toml silently drops unknown keys). Verified:
  migrate.go is field-specific, no enumeration exists.
- ❌ Don't reorder existing fields or anchor edits by line number. Place each edit IMMEDIATELY ADJACENT
  to its `NoVerify` sibling (same struct/function/block) and run `gofmt -w` to fix alignment.
- ❌ Don't clone `TestLoadFlags_NoVerify` into a flag test — there is no flag. Mirror the env/git/
  precedence NoVerify tests only.

---

## Confidence Score: 10/10

This is an exact 7-point mechanical copy of a proven, fully-tested field (`NoVerify`), with verbatim
before/after for every touch point, the three naming decisions resolved against the codebase (with the
PRD prose flagged as non-literal), the empirically-verified go-toml gotcha forcing snake_case in the
template, the test idioms to clone enumerated with line numbers, the explicit scope fences (no flag /
no consumer / no migration / no exampleConfigTemplate), and grep guards for each. No new pattern, no
new type, no new import. The only consumer lands in a later task, so there is no integration risk —
the unit tests + a clean `--dry-run` are the within-scope contract. One-pass success is essentially
guaranteed.
