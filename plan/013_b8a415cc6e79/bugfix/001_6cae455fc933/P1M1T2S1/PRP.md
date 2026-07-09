name: "P1.M1.T2.S1 — Add DIRECT-set env cases for STAGECOACH_AUTO_STAGE_ALL and STAGECOACH_MULTI_TURN_FALLBACK in loadEnv"
description: >
  Add two presence-semantic DIRECT-set env-var cases to `loadEnv` (internal/config/load.go) so a user
  can persistently enable/disable the two default-`true` booleans `auto_stage_all` and
  `multi_turn_fallback` via `STAGECOACH_AUTO_STAGE_ALL` / `STAGECOACH_MULTI_TURN_FALLBACK` (PRD Issue 3,
  §9.8 FR35, §15.2 layer 5). Because S1 (P1.M1.T1.S1, already landed) made these fields `*bool`, the
  DIRECT set uses `boolPtr(b)` — a non-nil pointer (incl. `*false`) is the explicit override that beats
  the default-`true` lower layers. Plus unit tests (true/false/invalid/unset + env>file precedence) and
  two rows in the docs/configuration.md env-var table.

---

## Goal

**Feature Goal**: Restore the full FR34 precedence ladder for the two default-`true` booleans by adding
the missing Layer 5 (environment) source. After this change, `STAGECOACH_AUTO_STAGE_ALL=false` and
`STAGECOACH_MULTI_TURN_FALLBACK=false` (or `=true`) are honored as DIRECT `*bool` overrides that win
over the Defaults/TOML/git-config layers — the last remaining gap after S1 fixed the `*bool` overlay
(Issue 1) and before T3 reconciles the git-key docs (Issue 2).

**Deliverable**:
1. **internal/config/load.go** — two new `os.LookupEnv` + `strconv.ParseBool` DIRECT-set blocks in
   `loadEnv`, mirroring `STAGECOACH_PUSH`/`STAGECOACH_NO_VERIFY` but writing `boolPtr(b)` (because the
   fields are `*bool`).
2. **internal/config/load_test.go** — unit tests covering true / false (DIRECT escape) / invalid-value
   (error) / unset (no-op) for BOTH vars, plus a full-`Load` precedence test proving env `false` beats
   a file/Defaults `true`.
3. **docs/configuration.md** — two new rows in the environment-variables table (after the
   `STAGECOACH_PUSH` row, line 199, before `## Git-config keys`, line 201).

**Success Definition**:
- `STAGECOACH_AUTO_STAGE_ALL=true` → `cfg.AutoStageAllValue()==true`; `=false` → `==false`; `=bogus` →
  `loadEnv` returns an error whose message contains `STAGECOACH_AUTO_STAGE_ALL`; unset → no change
  (`AutoStageAllValue()==true`, the Defaults seed). Same four cases for `STAGECOACH_MULTI_TURN_FALLBACK`.
- A full `Load` with `[defaults] auto_stage_all = true` in the file + `STAGECOACH_AUTO_STAGE_ALL=false`
  in the env yields `cfg.AutoStageAllValue()==false` (layer 5 > layers 1-4).
- `go build ./...`, `go vet ./internal/config/...`, `make test` all green; `gofmt -l` clean.
- The two new docs rows are present and correctly formatted.

## User Persona (if applicable)

**Target User**: A developer/CI author who wants to **persistently** disable auto-stage-all (or
multi-turn fallback) for a shell/session/repo without editing a TOML file and without using the
per-invocation `--no-auto-stage` flag.

**Use Case**: `STAGECOACH_AUTO_STAGE_ALL=false stagecoach` in a CI script or shell so stagecoach never
auto-stages leftover files — the persistent analog of `--no-auto-stage`.

**User Journey**: Before (Issue 3): the only working persistent disable was the undocumented camelCase
git-config key; the TOML path was silently broken (Issue 1) and the documented git key was invalid
(Issue 2). After S1+T2: the user can `export STAGECOACH_AUTO_STAGE_ALL=false` and it Just Works, with
the full precedence ladder (flag > env > git-config > file > default) honored.

**Pain Points Addressed**: PRD Issue 3 — "No working, correctly-documented persistent way to disable
`auto_stage_all`." This task adds the env layer; combined with S1's `*bool` overlay it closes the gap.

## Why

- **Issue 3 (Major)**: `loadEnv` had no `STAGECOACH_AUTO_STAGE_ALL` case (confirmed by grep in
  research_config_precedence.md §2). With the TOML path silently broken (Issue 1, fixed by S1) and the
  documented git key invalid (Issue 2, fixed by T3), there was no correctly-documented persistent
  disable. PRD §9.8 FR35 (env vars use the `stagecoach_` prefix), §15.2 (env is precedence layer 5).
- **Complementary to S1**: S1 made the `*bool` overlay propagate `false` end-to-end from the FILE/git
  layers. This task adds the ENV layer so the override has a persistent, easily-scripted source. The
  two tasks compose: env DIRECT-set `*bool` (boolPtr) is the highest non-flag layer, so it wins.
- **Bounded scope**: this is the ENV half of the milestone's "restore persistent disable" arc. It does
  NOT touch the `*bool` overlay mechanics (S1, done), the git-config key docs (T3), or any consumer
  (they already read the accessors after S1). It adds 2 production blocks + tests + 2 docs rows.

## What

**User-visible behavior**: `STAGECOACH_AUTO_STAGE_ALL` and `STAGECOACH_MULTI_TURN_FALLBACK` env vars are
now honored as persistent boolean overrides. Presence-semantic (a present, non-empty value overrides;
unset/empty is a no-op), parse via `strconv.ParseBool` (accepts `1/0/t/f/T/F/true/false/TRUE/FALSE`),
DIRECT-set as `*bool` so `false` is meaningful. An unparseable value is a hard load error (exit 1),
consistent with `STAGECOACH_PUSH`/`STAGECOACH_VERBOSE`/`STAGECOACH_NO_VERIFY`.

**Technical change**: two `os.LookupEnv` + `strconv.ParseBool` + `boolPtr(b)` blocks in `loadEnv`,
placed in the bool DIRECT-set group (after `STAGECOACH_NO_VERIFY`, before the string
`STAGECOACH_WORK_DESCRIPTION` var). The ONLY difference from the `STAGECOACH_PUSH` block is `boolPtr(b)`
instead of a plain `b` assignment (because these fields are `*bool`, not `bool`).

### Success Criteria
- [ ] `STAGECOACH_AUTO_STAGE_ALL=false` → `cfg.AutoStageAllValue()==false` (DIRECT escape; `*false` non-nil wins over default-true).
- [ ] `STAGECOACH_AUTO_STAGE_ALL=true` → `cfg.AutoStageAllValue()==true`.
- [ ] `STAGECOACH_AUTO_STAGE_ALL=bogus` → `loadEnv` error containing `STAGECOACH_AUTO_STAGE_ALL`.
- [ ] unset `STAGECOACH_AUTO_STAGE_ALL` → `AutoStageAllValue()==true` (Defaults seed, no-op).
- [ ] The four analogous cases pass for `STAGECOACH_MULTI_TURN_FALLBACK`.
- [ ] Full `Load`: file `[defaults] auto_stage_all = true` + env `STAGECOACH_AUTO_STAGE_ALL=false` ⇒ `AutoStageAllValue()==false` (layer 5 > layer 2).
- [ ] `go build ./...`, `go vet ./internal/config/...`, `make test` green; `gofmt -l` clean.
- [ ] Two docs rows present after `STAGECOACH_PUSH` (line 199), before `## Git-config keys`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — exact line numbers, the verbatim block to mirror, the one CRITICAL distinction (`boolPtr(b)` vs
plain `b`), the exact test functions to clone with current line numbers, the test-helpers available, the
exact docs insertion point, and explicit scope fences vs S1/T3.

### Documentation & References

```yaml
# MUST READ — the production pattern to mirror 1:1 (the ONLY difference is boolPtr(b))
- file: internal/config/load.go
  why: "loadEnv (229-324) is where the env cases live. STAGECOACH_PUSH (301-307) and STAGECOACH_NO_VERIFY
        (310-316) are the bool DIRECT-set precedent. Insert the two new blocks between line 315 (end of
        NO_VERIFY) and line 320 (start of the string STAGECOACH_WORK_DESCRIPTION var)."
  pattern: >
    if v, ok := os.LookupEnv("STAGECOACH_PUSH"); ok && v != "" {
        b, err := strconv.ParseBool(v)
        if err != nil { return fmt.Errorf("STAGECOACH_PUSH: %w", err) }
        cfg.Push = b // DIRECT set — can be false (escape hatch)
    }
  critical: "PUSH/NO_VERIFY write cfg.Push=b / cfg.NoVerify=b because those are PLAIN bool. AutoStageAll
             and MultiTurnFallback are *bool (S1), so you MUST write cfg.AutoStageAll=boolPtr(b) /
             cfg.MultiTurnFallback=boolPtr(b). A plain assignment will not compile. The non-nil pointer
             (incl. *false) is the explicit-override signal."

- file: internal/config/config.go
  why: "boolPtr helper at line 7 (UNEXPORTED — fine, load.go is package config). AutoStageAll *bool (69),
        MultiTurnFallback *bool (84). Defaults() seeds boolPtr(true) at 189/199. Accessors
        AutoStageAllValue() (241-246) and MultiTurnFallbackValue() (253-258): nil⇒true, non-nil⇒deref."
  pattern: "Tests read the accessors, NEVER the raw *bool field. boolPtr is already defined — DO NOT re-add it."

- file: internal/config/load_test.go
  why: "The test patterns to clone. TestLoadEnv_Push (1322) = true/false DIRECT-escape shape;
        TestLoadEnv_BadBoolErrors (229) = invalid-value error shape; TestLoad_EnvBoolFalseEscape (716)
        and TestLoad_EnvOverridesGit (601) = full-Load precedence shape."
  pattern: >
    cfg := Defaults(); t.Setenv("STAGECOACH_PUSH","true"); loadEnv(&cfg); assert cfg.Push==true.
    Then cfg2:=Config{Push:true}; t.Setenv("STAGECOACH_PUSH","false"); loadEnv(&cfg2); assert false.
  critical: "Use t.Setenv (Go 1.17+) — it auto-cleans env; NO manual os.Unsetenv needed. For *bool
             fields, start from Defaults() (seed boolPtr(true)) and assert the ACCESSOR flips to false
             on '=false'. For the full-Load precedence test, mirror TestLoad_EnvBoolFalseEscape (716):
             use loadEnvSetup(t)+chdir(t,repo)+writeConfigFile(t,globalDir,'config.toml',body) and
             Load(ctx, LoadOpts{RepoDir: repo})."

- file: docs/configuration.md
  why: "The environment-variables table. The LAST row is STAGECOACH_PUSH at line 199; line 201 begins
        '## Git-config keys'. Insert the two new rows BETWEEN 199 and 201."
  pattern: "Row format: '| `VAR` | `--flag` (or (no flag)) | Description | `VAR=value stagecoach` |'"
  critical: "DO NOT touch any other docs line. Line 166 has a known inaccuracy (claims multi_turn_fallback
             is settable via a stagecoach.multiTurnFallback git key that does NOT exist) — that is Issue 2
             / sibling P1.M1.T3.S1. T2.S1 adds ONLY the two env-var table rows."

- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/architecture/research_config_precedence.md
  why: "§2 confirms load.go was greenfield for these vars and lists every existing STAGECOACH_* case.
        §1/§4 document the *bool overlay model + the boolPtr/intPtr precedent."
  section: "§2 CONFIRMED: No STAGECOACH_AUTO_STAGE_ALL / STAGECOACH_MULTI_TURN_FALLBACK in loadEnv"

- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M1T1S1/PRP.md
  why: "S1 is the CONTRACT this task builds on. Read it to confirm the *bool fields, the boolPtr helper,
        the accessor signatures, and the explicit scope fence ('adding env vars is sibling task T2')."

- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M1T1S2/PRP.md
  why: "S2 (e2e integration tests) is landing in PARALLEL and explicitly does NOT set STAGECOACH_AUTO_STAGE_ALL
        (it uses the git-config layer for its precedence test, fenced as T2's job). T2.S1 now delivers that
        env var. No conflict: S2 tests behavior via the binary/git-config; T2.S1 adds the env source."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go        # boolPtr @7; AutoStageAll *bool @69; MultiTurnFallback *bool @84; Defaults boolPtr(true) @189/199; accessors @241/@253
  load.go          # loadEnv @229-324; STAGECOACH_PUSH @301-307; STAGECOACH_NO_VERIFY @310-316; STAGECOACH_WORK_DESCRIPTION @320; return nil @324  ← EDIT HERE
  load_test.go     # TestLoadEnv_Push @1322; TestLoadEnv_BadBoolErrors @229; TestLoad_EnvBoolFalseEscape @716; TestLoad_EnvOverridesGit @601  ← ADD TESTS
  git.go           # git.go:162 already does c.AutoStageAll = boolPtr(v) (S1); NO multiTurnFallback git key
  file.go          # materialize/overlay *bool guards (S1, DONE) — not touched by T2
docs/
  configuration.md # env-var table ends @199 (STAGECOACH_PUSH); '## Git-config keys' @201  ← ADD 2 ROWS
internal/cmd/root.go  # --no-auto-stage flag @170 (CONFIRMED; the docs 'Mirrors flag' value, inverse)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/config/load.go        # MODIFY: +2 env DIRECT-set blocks in loadEnv (boolPtr(b))
internal/config/load_test.go   # MODIFY: +4-5 test funcs (true/false/invalid/unset × both vars + precedence)
docs/configuration.md          # MODIFY: +2 rows in env-var table (after line 199)
# (no new files)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (*bool, not bool): AutoStageAll and MultiTurnFallback are *bool (S1). The PUSH/NO_VERIFY
//   blocks write cfg.Push = b (plain bool). Your two blocks MUST write cfg.AutoStageAll = boolPtr(b)
//   and cfg.MultiTurnFallback = boolPtr(b). boolPtr is already defined at config.go:7 — do NOT re-add.
//   A plain cfg.AutoStageAll = b will NOT COMPILE (cannot use bool as *bool).

// CRITICAL (DIRECT set, not overlay): the env layer writes DIRECTLY (cfg.AutoStageAll = boolPtr(b)),
//   bypassing the overlay() function. This is intentional — it is the escape hatch the overlay's
//   nil-inherit model cannot provide. A non-nil *false here is the final word at layer 5 (only flags,
//   layer 6/7, can beat it — and there is no --auto-stage flag, only the inverse --no-auto-stage).
//   So STAGECOACH_AUTO_STAGE_ALL=false stagecoach (--no-auto-stage absent) → false honored.

// CRITICAL (error discipline): strconv.ParseBool accepts 1/0/t/f/T/F/true/false/TRUE/FALSE/etc.
//   Anything else (e.g. "2", "yes", "bogus") returns an error. Wrap it EXACTLY as the other bool
//   blocks do: return fmt.Errorf("STAGECOACH_AUTO_STAGE_ALL: %w", err). A bad bool is a HARD load
//   failure (exit 1) — do not silently default. Mirror STAGECOACH_VERBOSE/TestLoadEnv_BadBoolErrors.

// CRITICAL (presence-semantic, not truthiness): guard with os.LookupEnv(name); ok && v != "". An
//   EMPTY string (STAGECOACH_AUTO_STAGE_ALL=) is a NO-OP (consistent with every other STAGECOACH_* var).
//   Do NOT use os.Getenv + truthiness. An unset var is also a no-op (Defaults seed boolPtr(true) stays).

// CRITICAL (test env cleanup): use t.Setenv (Go 1.17+) — it registers a cleanup that unsets the var.
//   Do NOT use os.Setenv/os.Unsetenv (they leak across tests and t.Setenv enforces no-parallel-unsafe).

// SCOPE FENCE: do NOT add a stagecoach.multiTurnFallback git key (none exists; git.go has only
//   autoStageAll). Do NOT fix docs line 166's git-key inaccuracy (that's T3 / Issue 2). Do NOT touch
//   the *bool overlay in file.go or consumers (S1, done). Only load.go + load_test.go + docs table.
```

## Implementation Blueprint

### Data models and structure
None. No new types. Two existing `*bool` fields (`AutoStageAll`, `MultiTurnFallback`) gain an env source.
`boolPtr` (config.go:7) and the accessors (`AutoStageAllValue()`, `MultiTurnFallbackValue()`) already exist.

### Implementation Tasks (ordered by dependencies)

> **Prerequisite**: S1 (P1.M1.T1.S1) must be merged — it is ALREADY APPLIED in the working tree (config.go
> shows `AutoStageAll *bool` @69, accessors @241/@253). Confirm before starting: the two fields are `*bool`
> and `boolPtr` exists. If S1 were somehow reverted, this task would not compile.

```yaml
Task 1: MODIFY internal/config/load.go — add two DIRECT-set env blocks in loadEnv
  - LOCATE the STAGECOACH_NO_VERIFY block (load.go:310-316, ending `cfg.NoVerify = b` @315) and the
    STAGECOACH_WORK_DESCRIPTION block (@320). Insert the two new blocks BETWEEN them (after line 315,
    before line 320) — grouping the bool DIRECT-set vars together.
  - BLOCK A (STAGECOACH_AUTO_STAGE_ALL):
        // §9.4 FR16 / §9.8 FR35 / §15.2 layer 5 — auto_stage_all via env (presence-semantic, DIRECT *bool
        // set; mirrors STAGECOACH_PUSH). boolPtr(b) makes a non-nil incl. *false the explicit override a
        // default-true field needs (env DIRECT-set beats default/file/git layers 1-4).
        if v, ok := os.LookupEnv("STAGECOACH_AUTO_STAGE_ALL"); ok && v != "" {
            b, err := strconv.ParseBool(v)
            if err != nil {
                return fmt.Errorf("STAGECOACH_AUTO_STAGE_ALL: %w", err)
            }
            cfg.AutoStageAll = boolPtr(b) // DIRECT *bool set — non-nil so false overrides the default-true lower layers
        }
  - BLOCK B (STAGECOACH_MULTI_TURN_FALLBACK): identical shape, sets cfg.MultiTurnFallback = boolPtr(b),
    error wrapper fmt.Errorf("STAGECOACH_MULTI_TURN_FALLBACK: %w", err). Reference §9.24 FR-T1c in the comment.
  - NAMING: env var names are ALL-CAPS SNAKE (STAGECOACH_AUTO_STAGE_ALL, STAGECOACH_MULTI_TURN_FALLBACK)
    matching the STAGECOACH_<SETTING> convention (FR35); error-message prefix matches the var name verbatim.
  - DEPENDENCIES: S1 (boolPtr + *bool fields). strconv and os are already imported in load.go (used by
    STAGECOACH_PUSH/NO_VERIFY) — no new imports.

Task 2: MODIFY internal/config/load_test.go — add unit + precedence tests (mirror existing funcs)
  - ADD TestLoadEnv_AutoStageAll (clone TestLoadEnv_Push @1322 shape; read via AutoStageAllValue()):
        // true
        cfg := Defaults(); t.Setenv("STAGECOACH_AUTO_STAGE_ALL","true")
        loadEnv(&cfg); assert cfg.AutoStageAllValue()==true
        // false DIRECT escape (start from Defaults boolPtr(true))
        cfg2 := Defaults(); t.Setenv("STAGECOACH_AUTO_STAGE_ALL","false")
        loadEnv(&cfg2); assert cfg2.AutoStageAllValue()==false  // *false non-nil wins over default-true
        // unset no-op (separate assertion or its own tiny func)
        cfg3 := Defaults() // no env set
        loadEnv(&cfg3); assert cfg3.AutoStageAllValue()==true   // Defaults seed unchanged
  - ADD TestLoadEnv_MultiTurnFallback — identical shape, MultiTurnFallbackValue(), STAGECOACH_MULTI_TURN_FALLBACK.
  - ADD TestLoadEnv_AutoStageAll_BadBoolErrors (clone TestLoadEnv_BadBoolErrors @229):
        cfg := Defaults(); t.Setenv("STAGECOACH_AUTO_STAGE_ALL","bogus")
        err := loadEnv(&cfg); assert err!=nil AND strings.Contains(err.Error(),"STAGECOACH_AUTO_STAGE_ALL")
  - ADD TestLoadEnv_MultiTurnFallback_BadBoolErrors — identical shape.
    (OPTIONAL: collapse the two bad-bool funcs into one table-driven test with both var names — either
     style is fine; the existing file uses discrete funcs, so prefer that for consistency.)
  - ADD TestLoad_AutoStageAll_EnvOverridesFile (clone TestLoad_EnvBoolFalseEscape @716):
        _, repo, globalDir := loadEnvSetup(t); chdir(t, repo)
        writeConfigFile(t, globalDir, "config.toml", "[defaults]\nauto_stage_all = true\n")  // layer 2 = true
        t.Setenv("STAGECOACH_AUTO_STAGE_ALL", "false")                                        // layer 5 = false (DIRECT *bool)
        cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo})
        assert err==nil AND cfg.AutoStageAllValue()==false    // env *false beats file true (layer 5 > 2)
    - ALSO assert a sibling true-control inside the same test OR a tiny t.Run: env "true" over a file
      "false" ⇒ Value()==true (proves the test setup is bidirectional, not accidentally always-false).
      e.g. writeConfigFile(..., "[defaults]\nauto_stage_all = false\n") + env "true" ⇒ AutoStageAllValue()==true.
  - ADD TestLoad_MultiTurnFallback_EnvOverridesFile — mirror, using `[generation]\nmulti_turn_fallback = false\n`
    file body + STAGECOACH_MULTI_TURN_FALLBACK=true ⇒ MultiTurnFallbackValue()==true (and the inverse).
  - HELPERS available (no new helpers needed): loadEnvSetup, chdir, writeConfigFile, setGitConfig, newFlagSet.
  - NAMING: test funcs prefixed TestLoadEnv_ (unit) / TestLoad_ (full Load), matching the file's convention.
  - COVERAGE: every success criterion in the "What" section has a test. Use t.Setenv (auto-cleanup).
  - DEPENDENCIES: Task 1.

Task 3: MODIFY docs/configuration.md — add two rows to the env-var table
  - LOCATE the STAGECOACH_PUSH row (line 199, the LAST row of the environment-variables table) and
    `## Git-config keys` (line 201). Insert two rows BETWEEN them (i.e., as new lines 200-201, pushing
    the Git-config header down).
  - ROWS (exact text, matching the table's 4-column '| Variable | Mirrors flag | Description | Example |' format):
        | `STAGECOACH_AUTO_STAGE_ALL` | `--no-auto-stage` (inverse) | Auto-stage all when nothing staged (true = enable, false = disable) | `STAGECOACH_AUTO_STAGE_ALL=false stagecoach` |
        | `STAGECOACH_MULTI_TURN_FALLBACK` | (no flag) | Enable lossless multi-turn fallback on large diffs (true = enable, false = disable) | `STAGECOACH_MULTI_TURN_FALLBACK=false stagecoach` |
  - ACCURACY: `--no-auto-stage` flag CONFIRMED (internal/cmd/root.go:170); it is the per-invocation
    INVERSE, the env var is the persistent form. There is NO `--no-multi-turn` flag ⇒ "(no flag)" is correct.
  - DO NOT touch any other docs line (esp. line 166's multiTurnFallback-git-key inaccuracy — T3's job).

Task 4: VERIFY — build, vet, format, targeted tests, full suite, grep guards
  - go build ./...
  - go vet ./internal/config/...
  - gofmt -l internal/config/load.go internal/config/load_test.go   # must list nothing
  - go test ./internal/config/... -run 'LoadEnv_AutoStageAll|LoadEnv_MultiTurnFallback|Load_AutoStageAll_Env|Load_MultiTurnFallback_Env' -v
  - make test                                                       # full race suite green
  - grep guard (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the DIRECT-set *bool env block (the deliverable — mirrors STAGECOACH_PUSH @301, ONE difference)
if v, ok := os.LookupEnv("STAGECOACH_AUTO_STAGE_ALL"); ok && v != "" { // presence-semantic
    b, err := strconv.ParseBool(v)                                    // accepts 1/0/t/f/true/false/...
    if err != nil {
        return fmt.Errorf("STAGECOACH_AUTO_STAGE_ALL: %w", err)       // HARD load error, mirroring PUSH
    }
    cfg.AutoStageAll = boolPtr(b) // ← boolPtr(b), NOT cfg.AutoStageAll = b. *bool field; non-nil ⇒ override
}
// (MultiTurnFallback block is identical with its own var name + cfg.MultiTurnFallback = boolPtr(b).)

// PATTERN: the false-DIRECT-escape unit test (clone of TestLoadEnv_Push @1322, accessor-flavored)
func TestLoadEnv_AutoStageAll(t *testing.T) {
    cfg := Defaults()
    t.Setenv("STAGECOACH_AUTO_STAGE_ALL", "true")
    if err := loadEnv(&cfg); err != nil { t.Fatalf("loadEnv err=%v", err) }
    if !cfg.AutoStageAllValue() { t.Errorf("AutoStageAll=false want true (STAGECOACH_AUTO_STAGE_ALL=true)") }

    // DIRECT-set escape hatch: =false ⇒ *false non-nil wins over the default-true seed
    cfg2 := Defaults()
    t.Setenv("STAGECOACH_AUTO_STAGE_ALL", "false")
    if err := loadEnv(&cfg2); err != nil { t.Fatalf("loadEnv escape err=%v", err) }
    if cfg2.AutoStageAllValue() { t.Errorf("AutoStageAll=true want false (STAGECOACH_AUTO_STAGE_ALL=false DIRECT set escape)") }

    // unset ⇒ no-op (Defaults boolPtr(true) unchanged)
    cfg3 := Defaults()
    if err := loadEnv(&cfg3); err != nil { t.Fatalf("loadEnv err=%v", err) }
    if !cfg3.AutoStageAllValue() { t.Errorf("AutoStageAll=false want true (no env set)") }
}

// PATTERN: the full-Load precedence test (clone of TestLoad_EnvBoolFalseEscape @716)
func TestLoad_AutoStageAll_EnvOverridesFile(t *testing.T) {
    _, repo, globalDir := loadEnvSetup(t)
    chdir(t, repo)
    writeConfigFile(t, globalDir, "config.toml", "[defaults]\nauto_stage_all = true\n")
    t.Setenv("STAGECOACH_AUTO_STAGE_ALL", "false") // env DIRECT *false beats file true (layer 5 > 2)
    cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo})
    if err != nil { t.Fatalf("Load err=%v", err) }
    if cfg.AutoStageAllValue() { t.Errorf("AutoStageAll=true want false (env DIRECT set must override file's true)") }
}
```

### Integration Points

```yaml
NO database / migration / routes. One production file + tests + docs table.

ENV LAYER (internal/config/load.go loadEnv): +2 os.LookupEnv DIRECT-set blocks, *bool via boolPtr(b).
TEST LAYER (internal/config/load_test.go): +4-5 funcs mirroring TestLoadEnv_Push / BadBoolErrors / EnvBoolFalseEscape.
DOCS (docs/configuration.md): +2 rows in the env-var table (after line 199 STAGECOACH_PUSH, before 201 '## Git-config keys').

PRECEDENCE (unchanged model): Defaults(1) → global TOML(2) → repo TOML(3) → git-config(4) → ENV(5, this task) → flags(6/7).
  Env DIRECT-set *bool is layer 5; only flags beat it. There is no --auto-stage flag (only --no-auto-stage
  at layer 6/7, which is an exit-2 fast path in default_action.go, not a Config field), so
  STAGECOACH_AUTO_STAGE_ALL is effectively the highest Config-source for AutoStageAll.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Compile (test files included)
go build ./...
# Vet the changed package
go vet ./internal/config/...
# Format check on the two .go files (docs/configuration.md is not gofmt'd)
gofmt -l internal/config/load.go internal/config/load_test.go
# Expected: nothing listed. If listed, `gofmt -w` the file(s).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new loadEnv + full-Load tests (targeted)
go test ./internal/config/... -run 'LoadEnv_AutoStageAll|LoadEnv_MultiTurnFallback|Load_AutoStageAll_Env|Load_MultiTurnFallback_Env' -v

# Full config package (regression — existing TestLoadEnv_Push/BadBoolErrors/EnvBoolFalseEscape must stay green)
go test ./internal/config/... -v
# Expected: ALL pass. The true/false/invalid/unset cases for BOTH vars + the two env>file precedence tests.
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole default-tag suite with race detector (project standard)
make test
# Expected: green (no behavior change for the default-true path; only the new env source is added).

# Manual end-to-end smoke (independent of the unit tests) — proves env false disables auto-stage via the binary:
make build
d=$(mktemp -d) && cd "$d" && git init -q && git config user.name T && git config user.email t@e && \
  printf 'init\n' > readme.md && git add readme.md && git commit -q -m seed && printf 'b\n' > b.txt && \
  printf 'config_version = 3\n[provider.stub]\ncommand = "/bin/true"\nprompt_delivery = "stdin"\noutput = "raw"\n' > cfg.toml && \
  STAGECOACH_AUTO_STAGE_ALL=false /home/dustin/projects/stagecoach/bin/stagecoach --config cfg.toml --provider stub; echo "exit=$?"
# Expected: exit=2 "Nothing to commit." (the env false won); b.txt still un-staged (git status shows ?? b.txt); no new commit.
# Positive control (drop the env var): exit=0, b.txt committed.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the two new env cases exist in load.go (exactly two occurrences each of the var name)
grep -n 'STAGECOACH_AUTO_STAGE_ALL\|STAGECOACH_MULTI_TURN_FALLBACK' internal/config/load.go
# Expected: 2 lines for each var (the LookupEnv line + the error-wrapper line). boolPtr(b) on the assignment line.

# Grep guard: tests cover all four cases per var + precedence
grep -n 'STAGECOACH_AUTO_STAGE_ALL\|STAGECOACH_MULTI_TURN_FALLBACK' internal/config/load_test.go
# Expected: multiple hits (true/false/unset/invalid + full-Load precedence).

# Grep guard: docs rows present exactly once each
grep -n 'STAGECOACH_AUTO_STAGE_ALL\|STAGECOACH_MULTI_TURN_FALLBACK' docs/configuration.md
# Expected: exactly one table row each (2 hits total).

# Scope-fence guard: NO production change outside load.go (consumers/overlay untouched — S1's job)
git diff --stat -- internal/config/file.go internal/config/git.go internal/config/config.go internal/cmd internal/generate internal/hook pkg/stagecoach
# Expected: empty (only load.go + load_test.go + docs/configuration.md changed).

# Scope-fence guard: docs line 166 (the multiTurnFallback-git-key inaccuracy) NOT touched — that's T3
git diff docs/configuration.md | grep -c '^@@'   # expect a single hunk around the env-var table (~line 199-201)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/config/...` clean
- [ ] `gofmt -l internal/config/load.go internal/config/load_test.go` lists nothing
- [ ] `go test ./internal/config/...` green (targeted run + full package)
- [ ] `make test` (race) green

### Feature Validation
- [ ] `STAGECOACH_AUTO_STAGE_ALL=true` → `AutoStageAllValue()==true`
- [ ] `STAGECOACH_AUTO_STAGE_ALL=false` → `AutoStageAllValue()==false` (DIRECT *bool escape)
- [ ] `STAGECOACH_AUTO_STAGE_ALL=bogus` → load error containing `STAGECOACH_AUTO_STAGE_ALL`
- [ ] unset `STAGECOACH_AUTO_STAGE_ALL` → `AutoStageAllValue()==true` (no-op, Defaults seed)
- [ ] The four analogous cases pass for `STAGECOACH_MULTI_TURN_FALLBACK`
- [ ] Full `Load`: file `auto_stage_all = true` + env `STAGECOACH_AUTO_STAGE_ALL=false` ⇒ `AutoStageAllValue()==false` (layer 5 > 2); true-control also asserted
- [ ] Manual binary smoke (Level 3): env `false` ⇒ exit 2 "Nothing to commit.", no commit (the headline Issue 3 fix)

### Scope-Boundary Validation
- [ ] Only load.go + load_test.go + docs/configuration.md changed (consumers/overlay/git.go/file.go untouched)
- [ ] No `stagecoach.multiTurnFallback` git key added (none exists; git layer is T3/Issue 2 territory)
- [ ] docs/configuration.md line 166 inaccuracy NOT "fixed" here (T3 / Issue 2); only the two env-var table rows added
- [ ] No new exported helpers added to the config package (boolPtr stays unexported; already exists)

### Code Quality & Docs
- [ ] The two new blocks use `boolPtr(b)` (not plain `b`) — compiles against the `*bool` fields
- [ ] Error wrappers match the existing bool-block discipline (`fmt.Errorf("VAR: %w", err)`)
- [ ] Comments cite the PRD sections (§9.4 FR16 / §9.8 FR35 / §15.2 layer 5 for AutoStageAll; §9.24 FR-T1c for MultiTurnFallback)
- [ ] Tests use `t.Setenv` (auto-cleanup), read via accessors, and include a true/false bidirectional control for the precedence test
- [ ] Docs rows match the table's 4-column format and the `--no-auto-stage` (inverse) / (no flag) values are accurate

---

## Anti-Patterns to Avoid

- ❌ Don't write `cfg.AutoStageAll = b` (plain bool) — the field is `*bool` (S1); it won't compile. Use `cfg.AutoStageAll = boolPtr(b)`. This is the ONE difference from the STAGECOACH_PUSH block you're mirroring.
- ❌ Don't re-add `boolPtr` — it already exists at config.go:7 (unexported; load.go is package config so it's in scope).
- ❌ Don't use `os.Getenv` + truthiness — use `os.LookupEnv(name); ok && v != ""` (presence-semantic, matching every other STAGECOACH_* var). An empty value is a no-op.
- ❌ Don't swallow a bad bool — wrap and return the error (`return fmt.Errorf("STAGECOACH_AUTO_STAGE_ALL: %w", err)`). A present-but-unparseable value is a HARD load failure, consistent with STAGECOACH_VERBOSE/PUSH/NO_VERIFY.
- ❌ Don't use `os.Setenv`/`os.Unsetenv` in tests — use `t.Setenv` (Go 1.17+, auto-cleanup, parallel-safe).
- ❌ Don't read the raw `*bool` field in tests (`cfg.AutoStageAll`) — read the accessor `cfg.AutoStageAllValue()` (nil⇒true, non-nil⇒deref). Asserting on the pointer directly is fragile.
- ❌ Don't touch the `*bool` overlay (file.go materialize/overlay) or consumers (default_action.go, generate.go, hook/exec.go, stagecoach.go) — that was S1's job and is DONE. Only load.go gets production changes.
- ❌ Don't fix docs line 166's "git config stagecoach.autoStageAll-style *bool behavior" wording for multi_turn_fallback — there is no multiTurnFallback git key, but reconciling that is Issue 2 / sibling P1.M1.T3.S1. T2.S1 adds ONLY the two env-var table rows.
- ❌ Don't add a `--no-multi-turn` flag or a `stagecoach.multiTurnFallback` git key — both are out of scope. Only the ENV source is this task.
- ❌ Don't forget the unset/no-op case and a bidirectional (true-over-false AND false-over-true) control in the precedence test — without them a false-positive "passes" could hide a setup defect.

---

## Confidence Score: 9/10

One-pass success is very high: the change is a near-verbatim clone of two existing, field-tested blocks
(STAGECOACH_PUSH @301, STAGECOACH_NO_VERIFY @310) with exactly ONE mechanical difference (`boolPtr(b)`
because the fields are `*bool`), and the test functions to clone (TestLoadEnv_Push @1322,
TestLoadEnv_BadBoolErrors @229, TestLoad_EnvBoolFalseEscape @716) are enumerated with current line
numbers and verbatim shapes. boolPtr, the accessors, and the test helpers all pre-exist. The -1 is for
the docs table insertion (markdown table formatting must be exact, and the implementer must insert
between lines 199 and 201 without disturbing the surrounding rows) and the bidirectional precedence
control (a careless single-direction test could mask a setup defect — flagged, but it's the one
judgment call). The manual binary smoke in Level 3 independently proves the headline Issue 3 fix.
