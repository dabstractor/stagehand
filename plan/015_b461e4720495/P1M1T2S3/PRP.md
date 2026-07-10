name: "P1.M1.T2.S3 — Per-role git-config reading: stagecoach.role.<role>.timeout (FR-R7 git-config layer)"
description: >
  Add a per-role loop to internal/config/git.go `loadGitConfig(repoDir)` (immediately AFTER the global
  `stagecoach.timeout` block @148-156, BEFORE the booleans comment @158) that, for each role in
  `roleNames`, reads `stagecoach.role.<role>.timeout` via the existing `gitConfigGet` (exit-1 = missing
  = no-op, NOT error), parses it via `parseTimeout` ("600s" OR bare "600"), and stores it via
  `c.setRoleTimeout(role, d)` — mirroring the global `stagecoach.timeout` block EXACTLY. Because
  `loadGitConfig` HAS an error return, a malformed value is a HARD ERROR
  (`return nil, fmt.Errorf("git config %s: %w", key, perr)`) — the OPPOSITE of S2's loadFlags
  (silent-ignore). This is NEW infrastructure (Finding 2: git.go reads NO per-role keys today — not
  even provider/model/reasoning). This task adds per-role TIMEOUT ONLY (provider/model/reasoning stay
  file/env/flag — explicitly out of scope). Plus 3 unit tests in git_test.go + docs/configuration.md
  update (keys table row + rewrite the now-stale "no per-role keys" NOTE @240). CONSUMES
  `setRoleTimeout` from S1 (load.go:66-78, LANDED). The overlay (file.go:467-496) already merges
  per-role Timeout with a `!= 0` guard — NO overlay change needed. Does NOT touch loadFlags (S2),
  root.go (S2), the env branch (S1), docs/cli.md (S2 — parallel, the "—" → key flip is S2's downstream
  item), ResolveRoleTimeout/defaultRoleTimeouts (P1.M2.T1), the 480s→120s default (P1.M2.T2), the 13
  Execute call sites (P1.M3), or broader docs (P1.M4.T2.S1).

---

## Goal

**Feature Goal**: Wire the GIT-CONFIG layer (PRD §16.1 layer 4) of per-role generation timeouts (PRD §9.15
FR-R7, §9.8 FR36) so `git config stagecoach.role.planner.timeout 600s` (and stager/message/arbiter) is
read by `loadGitConfig`, parsed via `parseTimeout`, and stored into the partial `Config.Roles[role].Timeout`
via `setRoleTimeout` — using the EXACT `gitConfigGet → parseTimeout → wrapped-error-on-bad-value → set`
discipline the global `stagecoach.timeout` block already uses. This is NEW infrastructure: today git.go
reads ZERO per-role keys (Finding 2), so a `roleNames` loop must be added.

**Deliverable**:
1. **internal/config/git.go** — a new `for _, role := range roleNames { ... }` loop in `loadGitConfig`
   (after the global `stagecoach.timeout` block @148-156, before the booleans comment @158) that reads
   `stagecoach.role.<role>.timeout` and stores it via `c.setRoleTimeout(role, d)`. NO new imports
   (`roleNames`, `gitConfigGet`, `parseTimeout`, `setRoleTimeout`, `fmt` are all already in scope).
2. **internal/config/git_test.go** — 3 tests: (i) per-role reading for ≥2 roles incl. the bare-int form
   (clone `TestLoadGitConfig_TimeoutDurationForm` D2); (ii) bad-value error wrapping naming the
   per-role key (clone `TestLoadGitConfig_BadTimeout` D); (iii) field-merge via overlay — git sets ONLY
   Timeout (Provider/Model/Reasoning stay zero) and overlays cleanly onto a file-layer provider
   (clone `TestLoadGitConfig_OverlaysWithDefaults` H).
3. **docs/configuration.md** — (a) add a `stagecoach.role.<role>.timeout` row to the git-config keys
   table (after the `stagecoach.timeout` row @227); (b) REWRITE the now-stale NOTE @240 that claims
   "The git-config layer has **no** per-role keys" — scope the "no per-role" claim to
   provider/model/reasoning only and acknowledge the new per-role timeout key.

**Success Definition**:
- `git config stagecoach.role.planner.timeout 600s` (in a test repo) → after `loadGitConfig(repo)`,
  `cfg.Roles["planner"].Timeout == 600*time.Second`.
- `git config stagecoach.role.stager.timeout 300` (bare int) → `cfg.Roles["stager"].Timeout == 300*time.Second`
  (proves `parseTimeout`, not `time.ParseDuration`).
- `git config stagecoach.role.planner.timeout notanumber` → `loadGitConfig` returns an error whose
  message contains BOTH `stagecoach.role.planner.timeout` AND `invalid timeout` (hard error — mirrors
  the global `stagecoach.timeout` block).
- A role with NO `stagecoach.role.<role>.timeout` key is untouched (git exit 1 → `found=false` → skipped;
  `cfg.Roles[role]` is absent or `Timeout==0`). Other roles' values are independently read.
- Field-merge: a git-config per-role timeout does NOT clobber a file-layer per-role provider — after
  `overlay(fileCfg, gitConfig)`, `Roles["planner"]` has BOTH `Provider == <file value>` AND
  `Timeout == 600s` (FR-R3; verified by the overlay `!= 0` guard at file.go:491).
- `go build ./...`, `go vet ./internal/config/...`, `gofmt -l`, `make lint`, `make test` all pass.

## User Persona (if applicable)

**Target User**: A developer who keeps Stagecoach config with the repo (in `.git/config` via `git config
--local`) rather than a `.stagecoach.toml`, and wants to give one role — typically the (slower) planner
— its own generation budget for this repo, persistently and per-repo (not per-invocation like a flag).

**Use Case**: Multi-commit decomposition on a large monorepo: the planner reasons over the whole diff
and benefits from a larger timeout; the message/stager agents are quick. `git config --local
stagecoach.role.planner.timeout 600s` (committed to the team's repo-local convention or set once) gives
the planner its own budget for every run in this repo, while the global `stagecoach.timeout 120s`
covers the other roles. This is the layer-4 (per-repo, version-controllable) source.

**User Journey**: `git config --local stagecoach.role.planner.timeout 600s` → `stagecoach` →
`loadGitConfig` reads it into the partial `Config.Roles["planner"].Timeout` → `overlay` merges it onto
the final config → (after P1.M2.T1/P1.M3 land) the planner's `provider.Execute` call uses 600s while
other roles use the global. (This subtask delivers git-config read+store+overlay-merge; consumption is
downstream.)

**Pain Points Addressed**: Today the only per-role timeout sources are file `[role.<role>].timeout`
(S1/S2), env `STAGECOACH_<ROLE>_TIMEOUT` (S1), and flag `--<role>-timeout` (S2). There is NO
`stagecoach.role.<role>.timeout` git key (Finding 2 — git.go reads NO per-role keys at all). A
per-repo, version-controllable per-role timeout was impossible via git-config. This task adds it as
layer 4 (above the file layers, below env/flag).

## Why

- **FR-R7 / §9.15 / §16.1 layer 4 / §9.8 FR36**: Per-role timeouts resolve across the 7-layer precedence;
  the git-config layer (4) is one of them — higher than file (3) and global-file (2), lower than env (5)
  and flag (7). This task makes `stagecoach.role.<role>.timeout` actually populate
  `Config.Roles[role].Timeout` at load time.
- **Finding 2 (architecture/critical_findings.md)**: "The git-config layer reads NO per-role keys at
  all — not even for provider/model/reasoning. The `--planner-model` flag help text says 'git
  stagecoach.role.planner' but that git key is NOT actually read today. FR-R7 requires
  `stagecoach.role.<role>.timeout` — this is NEW infrastructure. A per-role loop over `roleNames` must
  be added to `loadGitConfig`."
- **Why a small, mechanical subtask**: the global `stagecoach.timeout` block (git.go:148-156) is the
  EXACT `gitConfigGet → parseTimeout → wrapped-error → set` template. Adding the same logic inside a
  `for _, role := range roleNames` loop — with the key namespaced `stagecoach.role.<role>.timeout` and
  the setter being `c.setRoleTimeout(role, d)` instead of `c.Timeout = d` — is a 1:1 extension of the
  proven pattern. `roleNames`, `gitConfigGet`, `parseTimeout`, `setRoleTimeout`, and `fmt` are ALL
  already in scope (same package; `fmt` already imported).
- **Complementary, non-overlapping**: S1 owns the env layer (LANDED); S2 owns the flag layer (parallel —
  edits root.go + loadFlags in load.go + docs/cli.md). THIS task owns the git-config layer (git.go).
  Resolution (P1.M2.T1), the default change (P1.M2.T2), the 13 call sites (P1.M3), and broader docs
  (P1.M4.T2.S1) are all fenced out.

## What

**User-visible behavior**: `git config --local stagecoach.role.<role>.timeout <dur>` (and
`--global`) now overrides only that role's generation timeout at layer 4 for every run in that repo.
Combined with the rest of FR-R7, it budgets that role independently. This subtask's observable effect is
at the unit-test level (`loadGitConfig` direct call → assert `cfg.Roles[role].Timeout`) plus the docs
table/NOTE; actual generation consumption lands with P1.M2.T1/P1.M3.

**Technical change (one loop + tests + docs):**
1. The per-role loop in `loadGitConfig` — `gitConfigGet(repoDir, "stagecoach.role."+role+".timeout")`
   with exit-1 = missing = no-op (the existing helper already maps exit 1 → `found=false`), `parseTimeout`
   (accepts `"600s"` and bare `"600"`), HARD ERROR on bad value (`return nil, fmt.Errorf("git config %s: %w", key, perr)`),
   store via `c.setRoleTimeout(role, d)`.
2. 3 tests in git_test.go.
3. docs/configuration.md — 1 table row + NOTE rewrite.

### Success Criteria
- [ ] `loadGitConfig` has a `for _, role := range roleNames` loop after the global `stagecoach.timeout` block.
- [ ] `git config stagecoach.role.planner.timeout 600s` → `cfg.Roles["planner"].Timeout == 600*time.Second`.
- [ ] `git config stagecoach.role.stager.timeout 300` (bare int) → `300*time.Second` (proves parseTimeout).
- [ ] `git config stagecoach.role.planner.timeout notanumber` → error contains `stagecoach.role.planner.timeout` + `invalid timeout`.
- [ ] A role with no `stagecoach.role.<role>.timeout` key is untouched; other roles read independently.
- [ ] Git-config per-role timeout does NOT clobber a file-layer per-role provider after `overlay` (FR-R3).
- [ ] docs/configuration.md keys table has a `stagecoach.role.<role>.timeout` row; the @240 NOTE no longer claims "no per-role keys" for timeout.
- [ ] `go build ./...`, `go vet ./internal/config/...`, `gofmt -l`, `make lint`, `make test` pass.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim loop to add (from the item contract), the exact insertion point (after the global
`stagecoach.timeout` block @148-156, before the booleans comment @158), the verbatim global-timeout
template to mirror, the 3 tests to clone with current line numbers + the overlay field-merge proof, the
exact docs edits (table row + the now-stale NOTE @240 to rewrite), the verified git-config multi-dot-key
behavior, the prerequisite (`setRoleTimeout` already landed), and the scope fences against 6 sibling
subtasks are all enumerated below.

### Documentation & References

```yaml
- file: internal/config/git.go
  why: "THE change site. loadGitConfig(repoDir string) (*Config, error) @108-245. The global
        stagecoach.timeout block @147-156 is the EXACT template to mirror (gitConfigGet → parseTimeout →
        wrapped error → set). INSERT the per-role loop immediately AFTER this block's closing brace
        (@156), BEFORE the booleans comment '// --- booleans (--bool canonicalizes; FINDING C) ---'
        (@158). gitConfigGet @71 (the helper to call). fmt IS imported (@5) — NO new imports."
  pattern: >
    // the global stagecoach.timeout block to mirror (@147-156):
    	// --- timeout: accepts both "90" (seconds) and "90s" (Go duration) forms. ---
    	if v, found, err := gitConfigGet(repoDir, "stagecoach.timeout"); err != nil {
    		return nil, err
    	} else if found {
    		d, perr := parseTimeout(v) // parseTimeout handles both "90" and "90s"
    		if perr != nil {
    			return nil, fmt.Errorf("git config stagecoach.timeout: %w", perr)
    		}
    		c.Timeout = d
    	}
    // the per-role loop to INSERT after it (from the item contract):
    	for _, role := range roleNames {
    		key := "stagecoach.role." + role + ".timeout"
    		if v, found, err := gitConfigGet(repoDir, key); err != nil {
    			return nil, err
    		} else if found {
    			d, perr := parseTimeout(v)
    			if perr != nil {
    				return nil, fmt.Errorf("git config %s: %w", key, perr)
    			}
    			c.setRoleTimeout(role, d)
    		}
    	}
  critical: "loadGitConfig HAS an error return (*Config, error). So a malformed per-role timeout is a
    HARD ERROR (return nil, fmt.Errorf(...)) — NOT a silent-ignore. This is the OPPOSITE of S2's loadFlags
    (which has no error return and silently ignores). It matches the global stagecoach.timeout block
    EXACTLY (which also errors on bad value). The error uses the variable `key` (so it reads
    'git config stagecoach.role.planner.timeout: ...' for planner, etc.) — do NOT hardcode a role name.
    The loop reads TIMEOUT ONLY — do NOT add provider/model/reasoning git reads (those are explicitly
    out of scope per the contract; the @240 docs NOTE keeps them as 'no git-config support')."

- file: internal/config/load.go
  why: "The dependencies. roleNames @17 (the loop source: planner/stager/message/arbiter — SAME package
        as git.go, directly accessible, NO import). setRoleTimeout @66-78 (S1 — LANDED; the setter the
        loop calls — consume, do NOT re-add). parseTimeout @640 (the parse helper — accepts '600s' AND
        bare '600')."
  pattern: "var roleNames = []string{\"planner\", \"stager\", \"message\", \"arbiter\"}  // load.go:17"
  critical: "roleNames is in package config (load.go) and git.go is in package config — it is DIRECTLY
    visible (no import, no qualifier). CONFIRM setRoleTimeout is landed: `grep -n 'func (c \\*Config)
    setRoleTimeout' internal/config/load.go` → load.go:66-78. parseTimeout is unexported, same package
    → directly callable."

- file: internal/config/file.go
  why: "PROOF the overlay needs NO change. overlay(dst, src) @362 — its Roles branch @467-496 does
        per-FIELD merge with a `!= 0` guard for Timeout (@488-492). The git-config partial Config has
        Roles[role]={Timeout:d} (other fields zero); overlay copies only the Timeout field onto the
        existing entry WITHOUT clobbering a file-layer Provider/Model/Reasoning. This is the FR-R3
        field-merge guarantee, already implemented by S2's overlay work — S3 CONSUMES it, does NOT modify it."
  pattern: >
    // file.go:477-495 (the Roles overlay branch — ALREADY EXISTS, read-only for S3):
    	for role, rc := range src.Roles {
    		existing := dst.Roles[role]
    		if rc.Provider != ""  { existing.Provider = rc.Provider }
    		if rc.Model != ""     { existing.Model = rc.Model }
    		if rc.Reasoning != "" { existing.Reasoning = rc.Reasoning }
    		if rc.Timeout != 0    { existing.Timeout = rc.Timeout }  // FR-R7 guard
    		dst.Roles[role] = existing
    	}
  critical: "Do NOT touch overlay. The `!= 0` guard is correct: timeout has no meaningful 'explicit 0'
    (0 ⇒ inherit global, resolved by P1.M2.T1's ResolveRoleTimeout). A git-config per-role timeout (always
    > 0 for a valid value) survives the guard and merges cleanly. Test C proves this by calling overlay()
    directly (overlay is unexported, same package — callable from git_test.go)."

- file: internal/config/git_test.go
  why: "The test patterns + helpers to reuse. initRepo(t,dir) @24 + setGitConfig(t,dir,key,val) @48
        (the helpers). t.Setenv(\"HOME\", t.TempDir()) isolates global git config (FINDING E — EVERY test
        does this). TestLoadGitConfig_BadTimeout @~196 (the bad-value-error test to clone — asserts
        strings.Contains both the key AND 'invalid timeout'). TestLoadGitConfig_TimeoutDurationForm @~214
        (the both-forms test to clone — int '90' + duration '2m30s'). TestLoadGitConfig_OverlaysWithDefaults
        @~256 (the overlay-composition test to clone — calls overlay(&cfg, gc) directly to prove partial
        merge)."
  pattern: >
    // test skeleton (clone of TestLoadGitConfig_BadTimeout for the per-role error case):
    	t.Setenv("HOME", t.TempDir())
    	repo := t.TempDir()
    	initRepo(t, repo)
    	setGitConfig(t, repo, "stagecoach.role.planner.timeout", "notanumber")
    	_, err := loadGitConfig(repo)
    	if err == nil { t.Fatal("loadGitConfig err=nil, want non-nil for bad per-role timeout") }
    	if !strings.Contains(err.Error(), "stagecoach.role.planner.timeout") { t.Errorf(...) }
    	if !strings.Contains(err.Error(), "invalid timeout") { t.Errorf(...) }
  critical: "git config multi-dot keys WORK (verified): `git config stagecoach.role.planner.timeout 600s`
    writes; `git config --get` returns it (exit 0); an UNSET key returns exit 1 (found=false, NOT error —
    gitConfigGet maps exit 1 → ('', false, nil) at git.go:80). So setGitConfig + loadGitConfig behave
    identically to the global-key tests. Tests read cfg.Roles[role].Timeout (the PER-ROLE field), NOT
    cfg.Timeout (the global). Test ≥2 roles (planner + stager) to prove the loop is general. For the
    overlay field-merge test, construct a base cfg with a file-layer provider, then overlay(&base, gc)."

- file: docs/configuration.md
  why: "THE docs change. ## Git-config keys @211; keys table @225-237 (INSERT a per-role timeout row
        after the stagecoach.timeout row @227). The NOTE @240 is now STALE: it says 'The git-config layer
        has **no** per-role keys (stagecoach.role.*)' — REWRITE it to scope the 'no per-role' claim to
        provider/model/reasoning ONLY and acknowledge the new per-role timeout key."
  pattern: >
    // TABLE row (INSERT after the `stagecoach.timeout` row @227):
    | `stagecoach.role.<role>.timeout` | string | `git config --get stagecoach.role.<role>.timeout` | Per-role generation timeout (§9.15 FR-R7); `<role>` ∈ `planner`\|`stager`\|`message`\|`arbiter`. Duration string (`"600s"` or bare `600`); unset ⇒ inherit global `stagecoach.timeout`. |
    // NOTE @240 rewrite (replace the stale 'no per-role keys' sentence):
    > The git-config layer has per-role **timeout** keys (`stagecoach.role.<role>.timeout`, §9.15 FR-R7),
      but **no** per-role provider/model/reasoning keys — those are CLI flags (`--planner-provider`,
      etc.), env vars (`STAGECOACH_PLANNER_*`), and config-file `[role.*]` blocks only. There is no
      `stagecoach.commits` or `stagecoach.max_commits` (decompose settings are flag/env only; `--max-commits`
      also reads `[generation]`). There is also no `stagecoach.exclude` git-config key and no
      `STAGECOACH_EXCLUDE` env var (deliberate — see [Exclusion globs](#exclusion-globs-generationexclude) below).
  critical: "The existing NOTE has THREE claims lumped together: (1) no per-role keys, (2) no
    stagecoach.commits/max_commits, (3) no stagecoach.exclude. Claim (1) is now PARTIALLY FALSE (timeout
    exists; provider/model/reasoning still don't). Rewrite to split them: timeout IS supported; the rest
    stays as-is. Keep the [!NOTE] admonition wrapper and the exclusion-globs cross-link. DO NOT touch
    docs/cli.md (S2 owns it, parallel — its per-role-timeout git-config column will read '—' until a
    sync flip; docs/configuration.md is the authoritative git-config reference)."

- docfile: plan/015_b461e4720495/architecture/critical_findings.md
  why: "Finding 2 is the load-bearing justification for this task: 'The git-config layer reads NO
        per-role keys at all — not even for provider/model/reasoning... FR-R7 requires
        stagecoach.role.<role>.timeout — this is NEW infrastructure. A per-role loop over roleNames must
        be added to loadGitConfig.' Finding 9 confirms parseTimeout handles both forms."
  section: "Finding 2 (No per-role git-config support exists today) + Finding 9 (parseTimeout)"

- docfile: plan/015_b461e4720495/P1M1T2S1/PRP.md
  why: "S1 is the CONTRACT for the dependency: it produces setRoleTimeout(role string, d time.Duration)
        at load.go:66-78 (ALREADY LANDED). This task CONSUMES setRoleTimeout in the git-config loop.
        Read it to confirm the helper signature + the map-value-copy idiom (which S3 does NOT re-add)."

- docfile: plan/015_b461e4720495/P1M1T2S2/PRP.md
  why: "S2 (parallel) is the CONTRACT for the flag-layer sibling. It edits root.go + loadFlags (load.go)
        + docs/cli.md — DIFFERENT files from S3 (git.go + git_test.go + docs/configuration.md) → NO
        merge conflict. S2's docs/cli.md per-role-timeout git-config column = '—' (stale after S3 lands);
        S2's PRP explicitly defers the '—' → key flip to S3 as a downstream item. S3's CONTRACTED docs
        scope is docs/configuration.md ONLY (avoids a parallel conflict on docs/cli.md)."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  git.go           # loadGitConfig @108-245 ← ADD per-role loop after the global timeout block @147-156;
                   # gitConfigGet @71 (reuse); fmt @5 (already imported)
  git_test.go      # initRepo @24, setGitConfig @48; TestLoadGitConfig_BadTimeout @~196;
                   # TestLoadGitConfig_TimeoutDurationForm @~214; TestLoadGitConfig_OverlaysWithDefaults @~256
                   # ← ADD 3 tests
  load.go          # roleNames @17 (the loop source); setRoleTimeout @66-78 (S1 — LANDED; consume);
                   # parseTimeout @640 (reuse)
  file.go          # overlay Roles branch @467-496 (FR-R7 != 0 guard — ALREADY EXISTS; consumed, not modified)
  config.go        # RoleConfig.Timeout time.Duration (S1 grandparent — LANDED; consumed, not modified)
docs/
  configuration.md # ## Git-config keys @211; keys table @225-237 ← ADD 1 row after @227;
                   # NOTE @240 ← REWRITE (split the stale 'no per-role keys' claim)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/config/git.go           # MODIFY: +1 per-role loop in loadGitConfig (after the global timeout block)
internal/config/git_test.go      # MODIFY: +3 tests (per-role reading, bad-value error, field-merge via overlay)
docs/configuration.md            # MODIFY: +1 table row + rewrite the @240 NOTE
# (no new files; no struct changes; no overlay change; no other package touched)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (loadGitConfig HAS an error return — HARD ERROR on bad value, NOT silent-ignore):
//   func loadGitConfig(repoDir string) (*Config, error). So a malformed stagecoach.role.<role>.timeout
//   is `return nil, fmt.Errorf("git config %s: %w", key, perr)` — an ERROR. This is the OPPOSITE of
//   S2's loadFlags (no error return → silent-ignore). It matches the global stagecoach.timeout block
//   EXACTLY. Do NOT silently skip a bad per-role value (that would diverge from the global-key discipline
//   and hide config typos). The error uses the variable `key` (role-specific) — do NOT hardcode a role.

// CRITICAL (use parseTimeout, NOT time.ParseDuration): parseTimeout (load.go:640) accepts "600s"/"2m"
//   (time.ParseDuration) AND bare "600" (strconv.Atoi seconds). time.ParseDuration rejects bare ints.
//   The global stagecoach.timeout, STAGECOACH_TIMEOUT env, --timeout flag, and the per-role env/flag
//   ALL use parseTimeout — the per-role git key must too (cross-layer consistency). A test with the
//   bare-int form (stagecoach.role.stager.timeout 300 → 300s) is what PROVES parseTimeout was used.

// CRITICAL (gitConfigGet exit-1 = missing = NO-OP, NOT error): gitConfigGet (git.go:71) maps git exit 1
//   to (value="", found=false, err=nil). So a role with NO stagecoach.role.<role>.timeout key is simply
//   skipped (found=false → the `else if found` branch is not taken). Only exit 0 (found) parses+sets;
//   any other exit is a wrapped error. This is identical to every existing global key. The per-role
//   loop therefore does NOT error when a role's key is absent — it leaves that role untouched.

// CRITICAL (DIRECT-set via setRoleTimeout, NOT c.Roles[role]=...): the loop calls c.setRoleTimeout(role,
//   d) (the S1 helper). Do NOT write c.Roles[role].Timeout = d directly — Go forbids &map[k]; setRoleTimeout
//   does the map-value-copy write-back. Setting Timeout does NOT clobber an existing Provider/Model/Reasoning
//   on that role entry (FR-R3 field-merge — setRoleTimeout copies out the whole RoleConfig, sets one field,
//   writes back). This matters because the git-config partial Config is later overlaid.

// CRITICAL (per-role field, NOT global): the loop sets c.Roles[role].Timeout (via setRoleTimeout), NOT
//   c.Timeout. They are DIFFERENT fields. The role→global fallback (Roles[role].Timeout==0 ⇒ use cfg.Timeout)
//   is P1.M2.T1's ResolveRoleTimeout — NOT this task. Tests must assert cfg.Roles[role].Timeout.

// CRITICAL (overlay needs NO change — but verify the merge in a test): overlay (file.go:467-496) already
//   merges per-role Timeout with a `!= 0` guard. The git-config partial Config has Roles[role]={Timeout:d}
//   (other fields zero); overlay copies only Timeout onto the final config's existing role entry, WITHOUT
//   clobbering a file-layer Provider/Model/Reasoning. Load calls overlay(&cfg, gc) at load.go:138. Test C
//   proves this by calling overlay(&base, gc) directly with a file-layer provider. Do NOT modify overlay.

// CRITICAL (depends on S1 — ALREADY MET): setRoleTimeout exists at load.go:66-78 (S1 landed). RoleConfig.Timeout
//   exists at config.go:42. c.setRoleTimeout(role, d) compiles. CONFIRM with grep before editing:
//   `grep -n 'func (c \*Config) setRoleTimeout' internal/config/load.go`.

// CRITICAL (git-config multi-dot keys WORK — verified): `git config stagecoach.role.planner.timeout 600s`
//   is valid git syntax (section=stagecoach, subsection=role.planner, key=timeout). WRITE succeeds; READ
//   via --get returns the value (exit 0); an UNSET key returns exit 1 (found=false). The git_test.go helper
//   setGitConfig(t, repo, "stagecoach.role.planner.timeout", "600s") works (it runs git -C <repo> config).

// COORDINATION (no conflict with S1/S2): S1 edits loadEnv in load.go (LANDED). S2 edits loadFlags in
//   load.go + root.go + docs/cli.md (parallel). THIS task edits loadGitConfig in git.go + git_test.go +
//   docs/configuration.md. DIFFERENT functions/files → clean merge. Anchor the git.go edit on the global
//   stagecoach.timeout block TEXT (robust to any line drift). S1/S2 do NOT touch git.go → git.go line
//   numbers are STABLE for this task.

// CRITICAL (docs @240 NOTE is STALE — must rewrite, not just append): the NOTE at docs/configuration.md:240
//   says "The git-config layer has **no** per-role keys (stagecoach.role.*)". After this task that claim
//   is PARTIALLY FALSE (timeout IS read). Rewrite the NOTE to split the claims: timeout IS supported;
//   provider/model/reasoning + commits/max_commits + exclude remain unsupported. Keep the [!NOTE] wrapper
//   and the exclusion-globs cross-link.

// CRITICAL (do NOT touch docs/cli.md — S2 owns it, parallel): S2's docs/cli.md per-role-timeout rows
//   have git-config column = '—'. Flipping them to the real key is S2's downstream item (S2's PRP says
//   "When [S3] lands, it can flip the docs/cli.md git-config column from — to stagecoach.role.<role>.timeout").
//   S3's CONTRACTED docs scope is docs/configuration.md ONLY — editing docs/cli.md in parallel with S2
//   risks a merge conflict. docs/configuration.md is the authoritative git-config reference; update it fully.

// SCOPE: do NOT modify setRoleTimeout / the env branch (S1), loadFlags (S2), root.go (S2), overlay
//   (S2/S1-grandparent), ResolveRoleTimeout/defaultRoleTimeouts (P1.M2.T1), the 480s→120s default
//   (P1.M2.T2), any of the 13 Execute call sites (P1.M3), docs/cli.md (S2), or
//   README/how-it-works docs (P1.M4.T2.S1).
```

## Implementation Blueprint

### Data models and structure
None. No new types, no struct changes, no overlay change. One new loop in `loadGitConfig` (reuses
`roleNames`, `gitConfigGet`, `parseTimeout`, `setRoleTimeout`, `fmt` — all in scope), three tests, and
two doc edits (one table row + one NOTE rewrite). The `0`-duration "inherit" sentinel is S1's concern;
this task stores whatever `parseTimeout` returns (always a positive duration for a valid git value; an
invalid value is a hard error).

### Implementation Tasks (ordered by dependencies)

> **Prerequisite**: S1 (P1.M1.T2.S1) merged — `setRoleTimeout(role string, d time.Duration)` must exist.
> CONFIRM (it does): `grep -n 'func (c \*Config) setRoleTimeout' internal/config/load.go` → load.go:66-78.
> Then proceed.

```yaml
Task 1: MODIFY internal/config/git.go — add the per-role timeout loop in loadGitConfig
  - LOCATE the global stagecoach.timeout block inside loadGitConfig (search "// --- timeout: accepts both"
    — currently @147; the block is @147-156, closing with `c.Timeout = d` then `}`).
  - CONFIRM the next line after the block's closing brace is the blank line + the booleans comment
    "// --- booleans (--bool canonicalizes; FINDING C) ---" (currently @158).
  - INSERT immediately AFTER the timeout block's closing brace, BEFORE the booleans comment:
    	// §9.15 FR-R7 / §9.8 FR36 / §16.1 layer 4 — per-role generation timeout via git config
    	// (NEW infrastructure: git.go read NO per-role keys before this). Mirrors the global
    	// stagecoach.timeout block above EXACTLY: gitConfigGet → parseTimeout → wrapped error →
    	// setRoleTimeout. gitConfigGet maps a missing key (git exit 1) to found=false (no-op), so a
    	// role with no stagecoach.role.<role>.timeout key is untouched. parseTimeout accepts "600s"
    	// and bare "600". A malformed value is a HARD ERROR (loadGitConfig has an error return) —
    	// the OPPOSITE of S2's loadFlags silent-ignore. Per-role provider/model/reasoning git keys
    	// are intentionally NOT read here (file/env/flag only); this loop is timeout-only.
    	for _, role := range roleNames {
    		key := "stagecoach.role." + role + ".timeout"
    		if v, found, err := gitConfigGet(repoDir, key); err != nil {
    			return nil, err
    		} else if found {
    			d, perr := parseTimeout(v)
    			if perr != nil {
    				return nil, fmt.Errorf("git config %s: %w", key, perr)
    			}
    			c.setRoleTimeout(role, d)
    		}
    	}
  - VERIFY it sets c.Roles[role].Timeout (via setRoleTimeout), NOT c.Timeout.
  - VERIFY the bad-value path is a HARD ERROR (return nil, fmt.Errorf(...)) — NOT silent.
  - VERIFY the error uses the variable `key` (so it reads 'git config stagecoach.role.planner.timeout: ...').
  - NO new imports: roleNames, gitConfigGet, parseTimeout, setRoleTimeout, fmt — all already in scope.
  - DEPENDENCIES: S1 (setRoleTimeout must exist — it does @66-78).

Task 2: MODIFY internal/config/git_test.go — add 3 tests
  - 2a. TEST A — TestLoadGitConfig_PerRoleTimeout (clone TestLoadGitConfig_TimeoutDurationForm @~214 —
        per-role reading for ≥2 roles, both forms):
        	t.Setenv("HOME", t.TempDir()) // isolate global git config (FINDING E)
        	repo := t.TempDir()
        	initRepo(t, repo)
        	setGitConfig(t, repo, "stagecoach.role.planner.timeout", "600s") // duration form
        	setGitConfig(t, repo, "stagecoach.role.stager.timeout", "300")   // bare-int form (proves parseTimeout)
        	cfg, err := loadGitConfig(repo)
        	if err != nil { t.Fatalf("loadGitConfig err=%v, want nil", err) }
        	if rc := cfg.Roles["planner"]; rc.Timeout != 600*time.Second {
        		t.Errorf("Roles[planner].Timeout=%v want 600s", rc.Timeout)
        	}
        	if rc := cfg.Roles["stager"]; rc.Timeout != 300*time.Second {
        		t.Errorf("Roles[stager].Timeout=%v want 300s (bare int via parseTimeout)", rc.Timeout)
        	}
        	// unset role: message has no key → absent or Timeout==0
        	if rc, ok := cfg.Roles["message"]; ok && rc.Timeout != 0 {
        		t.Errorf("Roles[message].Timeout=%v want 0 (unset role untouched)", rc.Timeout)
        	}
  - 2b. TEST B — TestLoadGitConfig_PerRoleTimeout_BadValue (clone TestLoadGitConfig_BadTimeout @~196 —
        hard error naming the per-role key):
        	t.Setenv("HOME", t.TempDir())
        	repo := t.TempDir()
        	initRepo(t, repo)
        	setGitConfig(t, repo, "stagecoach.role.planner.timeout", "notanumber")
        	_, err := loadGitConfig(repo)
        	if err == nil { t.Fatal("loadGitConfig err=nil, want non-nil for bad per-role timeout") }
        	if !strings.Contains(err.Error(), "stagecoach.role.planner.timeout") {
        		t.Errorf("err=%v, want it to contain 'stagecoach.role.planner.timeout'", err)
        	}
        	if !strings.Contains(err.Error(), "invalid timeout") {
        		t.Errorf("err=%v, want it to contain 'invalid timeout'", err)
        	}
  - 2c. TEST C — TestLoadGitConfig_PerRoleTimeout_FieldMergeViaOverlay (clone
        TestLoadGitConfig_OverlaysWithDefaults @~256 — proves git sets ONLY Timeout + overlay merges
        cleanly with a file-layer provider WITHOUT clobbering it):
        	t.Setenv("HOME", t.TempDir())
        	repo := t.TempDir()
        	initRepo(t, repo)
        	setGitConfig(t, repo, "stagecoach.role.planner.timeout", "600s")
        	gc, err := loadGitConfig(repo)
        	if err != nil { t.Fatalf("loadGitConfig err=%v", err) }
        	// git-config per-role entry has ONLY Timeout set (Provider/Model/Reasoning zero) — field hygiene
        	rc := gc.Roles["planner"]
        	if rc.Timeout != 600*time.Second { t.Errorf("Timeout=%v want 600s", rc.Timeout) }
        	if rc.Provider != "" || rc.Model != "" || rc.Reasoning != "" {
        		t.Errorf("Roles[planner]=%+v want only Timeout set (git layer sets one field)", rc)
        	}
        	// Simulate a lower (file) layer with a per-role provider, then overlay the git config:
        	// BOTH must survive (FR-R3 field-merge via the overlay != 0 guard at file.go:491).
        	base := Defaults()
        	base.Roles = map[string]RoleConfig{"planner": {Provider: "agy"}}
        	overlay(&base, gc)
        	merged := base.Roles["planner"]
        	if merged.Provider != "agy" {
        		t.Errorf("Provider=%q want agy (file layer preserved — git did not clobber)", merged.Provider)
        	}
        	if merged.Timeout != 600*time.Second {
        		t.Errorf("Timeout=%v want 600s (git layer merged in)", merged.Timeout)
        	}
  - NAMING: TestLoadGitConfig_PerRoleTimeout, TestLoadGitConfig_PerRoleTimeout_BadValue,
    TestLoadGitConfig_PerRoleTimeout_FieldMergeViaOverlay — matches the file's TestLoadGitConfig_<Detail>
    convention. PLACE next to the mirrored tests (D2/D/H).
  - USE t.Setenv("HOME", t.TempDir()) (FINDING E isolation — EVERY git test does this), time.Duration
    literals (600*time.Second), strings.Contains for error assertions. overlay is unexported but SAME
    package → callable from git_test.go.
  - DEPENDENCIES: Task 1 (the loop must exist).

Task 3: MODIFY docs/configuration.md — add the per-role timeout table row + rewrite the stale NOTE
  - 3a. KEYS TABLE: INSERT after the `stagecoach.timeout` row (currently @227), before the
        `stagecoach.autoStageAll` row (currently @228):
        | `stagecoach.role.<role>.timeout` | string | `git config --get stagecoach.role.<role>.timeout` | Per-role generation timeout (§9.15 FR-R7); `<role>` ∈ `planner`\|`stager`\|`message`\|`arbiter`. Duration string (`"600s"` or bare `600`); unset ⇒ inherit global `stagecoach.timeout`. |
  - 3b. NOTE @240 REWRITE: the current NOTE claims "The git-config layer has **no** per-role keys
        (`stagecoach.role.*`)". REPLACE the first sentence(s) so the claim is split — timeout IS now
        supported; provider/model/reasoning + commits + exclude remain unsupported. New NOTE body:
        > The git-config layer has per-role **timeout** keys (`stagecoach.role.<role>.timeout`, §9.15
          FR-R7), but **no** per-role provider/model/reasoning keys — those are CLI flags
          (`--planner-provider`, etc.), env vars (`STAGECOACH_PLANNER_*`), and config-file `[role.*]`
          blocks only. There is no `stagecoach.commits` and no `stagecoach.max_commits` (decompose
          settings `--commits`/`--single`/`--no-decompose` are flag/env only; `--max-commits` also reads
          from the `[generation]` config-file section). There is also no `stagecoach.exclude` git-config
          key and no `STAGECOACH_EXCLUDE` env var (deliberate — see [Exclusion globs](#exclusion-globs-generationexclude) below); exclusions are config-file + `--exclude`/`-x` only.
  - VERIFY the [!NOTE] admonition wrapper is preserved and the exclusion-globs cross-link survives.
  - VERIFY: provider/model/reasoning are STILL listed as "no git-config support" (this task is
    timeout-only). Do NOT claim per-role provider/model git keys exist.
  - DEPENDENCIES: none (docs ride with the work).

Task 4: VERIFY — build, vet, format, targeted tests, full suite, grep guards
  - go build ./...
  - go vet ./internal/config/...
  - gofmt -l internal/config/git.go internal/config/git_test.go
  - go test ./internal/config/... -run 'PerRoleTimeout' -v
  - make test && make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the per-role git-config loop (1:1 mirror of the global stagecoach.timeout block @147-156,
// placed inside a roleNames loop). Inside loadGitConfig, after the global timeout block:
for _, role := range roleNames {
	key := "stagecoach.role." + role + ".timeout"
	if v, found, err := gitConfigGet(repoDir, key); err != nil { // LookPath/start failure OR unexpected exit
		return nil, err
	} else if found {                                            // git exit 0 → key present
		d, perr := parseTimeout(v)                                // "600s" OR bare "600"
		if perr != nil {
			return nil, fmt.Errorf("git config %s: %w", key, perr) // HARD ERROR (loadGitConfig has error return)
		}
		c.setRoleTimeout(role, d)                                // DIRECT-set (git layer 4; overlay merges it later)
	}
	// (git exit 1 → found=false → this role is untouched; NO error)
}

// CONTRAST: S2's loadFlags per-role -timeout branch SILENTLY ignores a bad value (loadFlags has no
// error return). loadGitConfig HAS an error return → a bad per-role git value is a HARD ERROR. This
// env/flag-vs-git asymmetry is intentional: loadGitConfig already errors on a bad GLOBAL timeout
// (git.go:153), so the per-role key must too (consistency within the git layer).

// PATTERN: the per-role reading test (clone of TestLoadGitConfig_TimeoutDurationForm @~214)
func TestLoadGitConfig_PerRoleTimeout(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // FINDING E: isolate global ~/.gitconfig
	repo := t.TempDir()
	initRepo(t, repo)
	setGitConfig(t, repo, "stagecoach.role.planner.timeout", "600s")
	setGitConfig(t, repo, "stagecoach.role.stager.timeout", "300") // bare int → parseTimeout
	cfg, err := loadGitConfig(repo)
	if err != nil { t.Fatalf("loadGitConfig err=%v, want nil", err) }
	if rc := cfg.Roles["planner"]; rc.Timeout != 600*time.Second {
		t.Errorf("Roles[planner].Timeout=%v want 600s", rc.Timeout)
	}
	if rc := cfg.Roles["stager"]; rc.Timeout != 300*time.Second {
		t.Errorf("Roles[stager].Timeout=%v want 300s (bare int)", rc.Timeout)
	}
}

// PATTERN: the field-merge test (clone of TestLoadGitConfig_OverlaysWithDefaults @~256 — proves overlay
// needs NO change and the git value reaches the final config without clobbering a file-layer provider)
func TestLoadGitConfig_PerRoleTimeout_FieldMergeViaOverlay(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	initRepo(t, repo)
	setGitConfig(t, repo, "stagecoach.role.planner.timeout", "600s")
	gc, err := loadGitConfig(repo)
	if err != nil { t.Fatalf("loadGitConfig err=%v", err) }
	base := Defaults()
	base.Roles = map[string]RoleConfig{"planner": {Provider: "agy"}} // file-layer provider
	overlay(&base, gc)                                                // exactly what Load does at load.go:138
	merged := base.Roles["planner"]
	if merged.Provider != "agy" { t.Errorf("Provider=%q want agy (file preserved)", merged.Provider) }
	if merged.Timeout != 600*time.Second { t.Errorf("Timeout=%v want 600s (git merged)", merged.Timeout) }
}
```

### Integration Points

```yaml
NO database / routes / CLI / public-API / struct / overlay changes. One loop + three tests + two doc edits.

LOAD PATH (internal/config/git.go):
  - +1 loop in loadGitConfig: for role in roleNames → gitConfigGet(stagecoach.role.<role>.timeout)
    → parseTimeout → c.setRoleTimeout(role, d). Bad value = HARD ERROR.

CONSUMED (from S1, already landed):
  - setRoleTimeout(role string, d time.Duration) (load.go:66-78) — the loop calls it.
  - RoleConfig.Timeout time.Duration (config.go:42, from S1's grandparent) — the field it writes.

CONSUMED (pre-existing, no change):
  - overlay() Roles branch (file.go:467-496, FR-R7 `!= 0` guard) — merges the git per-role timeout onto
    the final config at Load's overlay(&cfg, gc) call (load.go:138) WITHOUT clobbering file-layer
    provider/model/reasoning. Load needs NO change.

DOCS (docs/configuration.md):
  - +1 row in the git-config keys table (stagecoach.role.<role>.timeout, after stagecoach.timeout @227)
  - REWRITE the @240 NOTE (split the stale "no per-role keys" claim: timeout IS supported;
    provider/model/reasoning + commits + exclude remain unsupported)

DOWNSTREAM (this subtask ENABLES but does NOT build — sibling subtasks):
  - docs/cli.md: S2's per-role-timeout rows have git-config column = '—' (stale after S3). S2's PRP
    defers the '—' → stagecoach.role.<role>.timeout flip as a downstream item. S3 does NOT touch
    docs/cli.md (S2 owns it, parallel) — the flip happens in S2 or a follow-up sync.
  - P1.M2.T1.S1: ResolveRoleTimeout(role, cfg) + defaultRoleTimeouts{planner:480s} (reads Roles[role].Timeout;
    0 ⇒ fall back to cfg.Timeout).
  - P1.M2.T2.S1: global default 480s→120s + pinning-test fixes.
  - P1.M3: 13 provider.Execute call sites pass the resolved per-role timeout instead of cfg.Timeout.

PRECEDENCE (this task = layer 4, the git-config source):
  CLI flag --<role>-timeout (S2, layer 7) > env STAGECOACH_<ROLE>_TIMEOUT (S1, layer 5) >
    stagecoach.role.<role>.timeout git (THIS, layer 4) > [role.<role>].timeout TOML (S1/S2 file, layer 3)
    > global timeout > built-in role default.

UNCHANGED (do NOT touch): config.go structs (S1 grandparent); load.go setRoleTimeout/env-branch/loadFlags
  (S1/S2); root.go (S2); file.go overlay (S2/S1-grandparent); git.go's other keys; Defaults().Timeout
  (stays 480s — P1.M2.T2); the 13 Execute call sites (P1.M3); docs/cli.md (S2);
  README/how-it-works docs (P1.M4.T2.S1).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build everything (the per-role loop must compile; setRoleTimeout/parseTimeout/gitConfigGet are consumed)
go build ./...
# Vet the changed package
go vet ./internal/config/...
# Format check
gofmt -l internal/config/git.go internal/config/git_test.go
# Expected: nothing listed. If listed: gofmt -w the file(s).
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new per-role git-config tests (targeted)
go test ./internal/config/... -run 'PerRoleTimeout' -v
# Expected: all pass — per-role reading (duration + bare-int + unset-role-untouched); bad-value hard error
#           naming the per-role key; field-merge via overlay (git timeout + file provider both survive).

# Full config package (regression — existing loadGitConfig / global-timeout / overlay tests stay green)
go test ./internal/config/... -v

# Whole suite (race) — loadGitConfig is on the load path of every config.Load
make test
# Expected: ALL pass. Global default still 480s (unchanged here) → 480s-pinning tests untouched.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary
make build

# Smoke: git config stagecoach.role.planner.timeout loads cleanly (parse happens; no consumer uses it yet,
# but load must not error). After this task the value is STORED in cfg.Roles["planner"].Timeout and merged
# via overlay; behavior observation (the planner actually using 600s) lands with P1.M2.T1/P1.M3. This smoke
# proves the git-config read path end-to-end:
BIN=/home/dustin/projects/stagecoach/bin/stagecoach
mkdir -p /tmp/sc_role_git && cd /tmp/sc_role_git && git init -q
git config --local stagecoach.role.planner.timeout 600s
# (a) a valid value loads WITHOUT a parse error (it may exit for other reasons — e.g. nothing staged —
#     but NOT a per-role timeout parse error):
$BIN --dry-run --no-color 2>&1 | head -5
# Expected: loads WITHOUT error about stagecoach.role.planner.timeout.
# (b) a malformed value FAILS at load with a clear, key-named error:
git config --local stagecoach.role.planner.timeout notanumber
$BIN --dry-run --no-color 2>&1 | head -5
# Expected: an error containing "git config stagecoach.role.planner.timeout" and "invalid timeout"; exit 1.
cd / && rm -rf /tmp/sc_role_git
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the per-role loop exists exactly once in loadGitConfig and uses setRoleTimeout (NOT c.Timeout=)
grep -n 'stagecoach.role." + role + ".timeout\|"stagecoach.role\.' internal/config/git.go
# Expected: one hit (the key := line inside the loop).
grep -nA10 'for _, role := range roleNames' internal/config/git.go
# Expected: ONE hit inside loadGitConfig (NOT the loadEnv/loadFlags loops in load.go); the body calls
#           c.setRoleTimeout(role, d) and returns a wrapped error on bad value.

# Grep guard: the loop returns a HARD ERROR on bad value (NOT silent like loadFlags)
grep -n 'git config %s: %w' internal/config/git.go
# Expected: one hit — fmt.Errorf("git config %s: %w", key, perr) inside the per-role loop. (The global
#           stagecoach.timeout block uses the literal "git config stagecoach.timeout: %w" form — different.)

# Grep guard: parseTimeout (not time.ParseDuration) is used in the per-role loop
grep -n 'parseTimeout' internal/config/git.go
# Expected: two hits — the global stagecoach.timeout block AND the new per-role loop. time.ParseDuration
#           must NOT appear in git.go (it lives only inside parseTimeout at load.go:641).

# Grep guard: the loop is TIMEOUT-ONLY — no per-role provider/model/reasoning git reads were added
grep -n 'stagecoach.role.*provider\|stagecoach.role.*model\|stagecoach.role.*reasoning' internal/config/git.go
# Expected: empty (those git keys are intentionally NOT read — file/env/flag only; this task is timeout-only).

# Grep guard: roleNames is referenced (loop source) — same package, no import added
grep -n 'roleNames' internal/config/git.go
# Expected: one hit (the for-range line). `import` block in git.go should be UNCHANGED (no new import).

# Grep guard: 3 new tests exist
grep -n 'func TestLoadGitConfig_PerRoleTimeout' internal/config/git_test.go
# Expected: three hits (PerRoleTimeout, PerRoleTimeout_BadValue, PerRoleTimeout_FieldMergeViaOverlay).

# Grep guard: docs/configuration.md has the new table row + the rewritten NOTE
grep -n 'stagecoach.role.<role>.timeout' docs/configuration.md
# Expected: ≥1 hit in the keys table.
grep -n 'per-role \*\*timeout\*\* keys' docs/configuration.md
# Expected: one hit (the rewritten NOTE — proves the stale "no per-role keys" claim was split).

# Scope-boundary guard: this subtask added NO loadFlags/env/resolution/default/overlay changes
grep -rn 'ResolveRoleTimeout\|defaultRoleTimeouts' internal/config/git.go internal/config/git.go
# Expected: empty (those are P1.M2.T1 — NOT this subtask).
grep -n '120 \* time.Second' internal/config/config.go
# Expected: empty (global default 480s→120s is P1.M2.T2; Defaults().Timeout must still be 480*time.Second).
# overlay (file.go) must be UNCHANGED:
git diff --stat -- internal/config/file.go
# Expected: empty (overlay is consumed, not modified).

# Scope-boundary guard: only git.go + git_test.go + docs/configuration.md changed
git diff --stat -- internal/config/ docs/
# Expected: internal/config/git.go + internal/config/git_test.go + docs/configuration.md.
#           NO load.go / root.go / config.go / file.go churn; NO docs/cli.md churn (S2 owns it).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/config/...` clean
- [ ] `gofmt -l` on the 2 changed .go files empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass, incl. the 3 new tests

### Feature Validation
- [ ] `loadGitConfig` has a `for _, role := range roleNames` loop after the global `stagecoach.timeout` block
- [ ] `git config stagecoach.role.planner.timeout 600s` → `cfg.Roles["planner"].Timeout == 600*time.Second`
- [ ] `git config stagecoach.role.stager.timeout 300` (bare int) → `300*time.Second` (proves parseTimeout)
- [ ] `git config stagecoach.role.planner.timeout notanumber` → error contains `stagecoach.role.planner.timeout` + `invalid timeout`
- [ ] A role with no `stagecoach.role.<role>.timeout` key is untouched; other roles read independently
- [ ] Git-config per-role timeout does NOT clobber a file-layer per-role provider after `overlay` (FR-R3)
- [ ] docs/configuration.md keys table has a `stagecoach.role.<role>.timeout` row
- [ ] docs/configuration.md @240 NOTE no longer claims "no per-role keys" for timeout (rewritten: timeout supported; provider/model/reasoning + commits + exclude remain unsupported)

### Scope-Boundary Validation
- [ ] NO `setRoleTimeout` modification / env-branch change (S1 — consumed only)
- [ ] NO loadFlags / root.go change (S2 — parallel)
- [ ] NO `overlay` change (consumed; file.go untouched)
- [ ] NO `ResolveRoleTimeout` / `defaultRoleTimeouts` added (P1.M2.T1)
- [ ] `Defaults().Timeout` STILL 480s; 480s-pinning tests UNCHANGED (P1.M2.T2)
- [ ] NO config.go struct / file.go / Execute call-site changes
- [ ] NO docs/cli.md change (S2 owns it — the "—" → key flip is S2's downstream item)
- [ ] NO README / how-it-works docs changes (P1.M4.T2.S1)
- [ ] NO per-role provider/model/reasoning git reads added (timeout-only, per contract)
- [ ] Only `internal/config/git.go` + `internal/config/git_test.go` + `docs/configuration.md` changed

### Code Quality & Docs
- [ ] The loop comment cites FR-R7 / §9.15 / FR36 / §16.1 layer 4 + the global-timeout-mirror rationale + the hard-error (not silent) rationale + the timeout-only scope note
- [ ] The error uses the variable `key` (role-specific: `git config stagecoach.role.planner.timeout: ...`)
- [ ] Tests use `t.Setenv("HOME", t.TempDir())` (FINDING E isolation), read `cfg.Roles[role].Timeout` (per-role field, not global), test ≥2 roles + bare-int form + the overlay field-merge
- [ ] docs table row matches the existing column convention (Key/Type/Reads with/Description); NOTE preserves the [!NOTE] wrapper + exclusion-globs cross-link

---

## Anti-Patterns to Avoid

- ❌ Don't silently skip a malformed per-role git value — `loadGitConfig` HAS an error return (`*Config, error`), so a bad `stagecoach.role.<role>.timeout` is a HARD ERROR (`return nil, fmt.Errorf("git config %s: %w", key, perr)`), exactly like the global `stagecoach.timeout` block at git.go:153. This is the OPPOSITE of S2's `loadFlags` (no error return → silent-ignore). Copying loadFlags' silent-ignore here would hide config typos and diverge from the git-layer discipline.
- ❌ Don't use `time.ParseDuration` in the loop — it rejects bare `"600"`. Use `parseTimeout` (load.go:640), consistent with the global `stagecoach.timeout` / `STAGECOACH_TIMEOUT` / `--timeout` / the per-role env (S1) / the per-role flag (S2). The bare-int test case (`stagecoach.role.stager.timeout 300` → 300s) is what PROVES parseTimeout was chosen.
- ❌ Don't set `c.Timeout` (the global) in the per-role loop — set `c.Roles[role].Timeout` via `c.setRoleTimeout(role, d)`. They are DIFFERENT fields; the role→global fallback is P1.M2.T1's `ResolveRoleTimeout`, not this task.
- ❌ Don't write `c.Roles[role].Timeout = d` directly — Go forbids `&map[k]`. Use `c.setRoleTimeout(role, d)` (the S1 helper, load.go:66-78), which does the map-value-copy write-back. It sets Timeout ONLY (FR-R3 field-merge: Provider/Model/Reasoning on that entry survive).
- ❌ Don't hardcode `"stagecoach.role.planner.timeout"` in the error or the key — use `key := "stagecoach.role." + role + ".timeout"` and `fmt.Errorf("git config %s: %w", key, perr)` so the loop produces the right key name for all 4 roles.
- ❌ Don't add per-role provider/model/reasoning git reads — the contract is **timeout-only**. Finding 2 and the @240 docs NOTE keep provider/model/reasoning as file/env/flag-only (no git-config). Adding them would expand scope and conflict with the docs rewrite.
- ❌ Don't modify `overlay` (file.go:467-496) — it already merges per-role Timeout with a `!= 0` guard (FR-R7). The git-config partial Config (`Roles[role]={Timeout:d}`) overlays cleanly onto a file-layer provider WITHOUT clobbering it. Test C proves this; no overlay change is needed or wanted.
- ❌ Don't touch `docs/cli.md` — S2 owns it and is editing it in parallel (its per-role-timeout rows have git-config column = `—`). Flipping those to the real key is S2's explicit downstream item. S3's contracted docs scope is `docs/configuration.md` ONLY (the authoritative git-config reference). Editing docs/cli.md in parallel risks a merge conflict.
- ❌ Don't forget to REWRITE (not just append to) the @240 NOTE — it currently says "The git-config layer has **no** per-role keys (`stagecoach.role.*`)", which becomes PARTIALLY false after this task (timeout IS read). Split the claim: timeout supported; provider/model/reasoning + commits + exclude remain unsupported. Keep the `[!NOTE]` wrapper and the exclusion-globs cross-link.
- ❌ Don't read `cfg.Timeout` in the tests to verify the per-role git key — read `cfg.Roles[role].Timeout`. (A test asserting on `cfg.Timeout` would pass even if the loop were missing, masking the bug.)
- ❌ Don't test only the planner role — test ≥2 (planner + stager) plus the bare-int form to prove the loop is general and parseTimeout (not ParseDuration) is used, and add the overlay field-merge test to prove no overlay change is needed.
- ❌ Don't touch `setRoleTimeout` / the env branch (S1), `loadFlags` (S2), `root.go` (S2), `ResolveRoleTimeout`/`defaultRoleTimeouts` (P1.M2.T1), the 480s→120s default change (P1.M2.T2), the 13 Execute call sites (P1.M3), `docs/cli.md` (S2), or README/how-it-works docs (P1.M4.T2.S1).

---

## Confidence Score: 10/10

One-pass success is essentially certain: the per-role loop is a 1:1 clone of the existing global
`stagecoach.timeout` block (git.go:147-156) — wrapped in a `for _, role := range roleNames` loop, with
the key namespaced `stagecoach.role.<role>.timeout`, the setter being `c.setRoleTimeout(role, d)`, and
the error using the variable `key`. The item contract gives the loop verbatim. The prerequisites
(`setRoleTimeout` from S1, `RoleConfig.Timeout` from S1's grandparent, `parseTimeout`, `gitConfigGet`,
`roleNames`, `fmt`) are ALL already in scope (same package; `fmt` already imported) — verified by reading
the files. The git-config multi-dot-key behavior (`stagecoach.role.planner.timeout` write/read/missing)
is verified in a temp repo (write OK; read found = exit 0; read missing = exit 1 = `found=false`, not
error — exactly what `gitConfigGet` handles). The overlay (file.go:467-496) already merges per-role
Timeout with a `!= 0` guard — NO overlay change needed (Test C proves it). The 3 tests are clones of
three existing tests (`TestLoadGitConfig_TimeoutDurationForm`, `TestLoadGitConfig_BadTimeout`,
`TestLoadGitConfig_OverlaysWithDefaults`). There is NO file-level conflict with the parallel siblings
(S1 edits loadEnv in load.go [LANDED]; S2 edits loadFlags in load.go + root.go + docs/cli.md — this task
edits loadGitConfig in git.go + git_test.go + docs/configuration.md). The only vigilance points — hard
error (not silent) on bad value, parseTimeout (not ParseDuration), per-role field (not global),
DIRECT-set via setRoleTimeout (not `c.Roles[role]=`), no overlay change, no per-role provider/model/
reasoning git reads, the @240 NOTE rewrite (not append), and not touching docs/cli.md — are all
enumerated as CRITICAL gotchas with grep guards.
