---
name: "P1.M2.T2.S2 — Add tooled_flags to pi + claude (the stager-capable providers)"
description: |
  Populate the `TooledFlags []string` field (added+plumbed in P1.M1 — Manifest field, Merge regime-2,
  Render `RenderTooled` mode — all COMPLETE) on the TWO "explicit tool-disable switch" providers so they
  can serve the v2 **stager** role (§11.5: tools ON, git-scoped, non-interactive — the only role that
  mutates the index). The other five providers (gemini, agy, opencode, codex, cursor) keep
  `TooledFlags == nil` → they cannot stager (FR-D4 falls back to a provider that can).

  This subtask MODIFIES 5 existing files (no new files, no new types, no logic change, no dep). It builds
  on P1.M1's TooledFlags plumbing and runs AFTER the sibling P1.M2.T2.S1 (reorder preferredBuiltins +
  pi `default_model=""`) — its edits are ADDITIVE on top of S1's.

  ⚠️ **THE #1 trap — the 4-WAY PARITY CHAIN on `tooled_flags`.** `TooledFlags` is a plain `[]string`.
  go-toml/v2 decodes a PRESENT array → non-nil slice; an ABSENT key → `nil` (S1 FINDING C/D). Setting a
  NON-EMPTY `TooledFlags` in `builtinPi()`/`builtinClaude()` REQUIRES writing the IDENTICAL `tooled_flags
  = [...]` array in ALL FOUR parity artifacts or BOTH `reflect.DeepEqual` oracles fail (non-nil ≠ nil):
    builtinPi().TooledFlags  ⇄  piTOML (builtin_test.go)  ⇄  providers/pi.toml
    builtinClaude().TooledFlags ⇄ claudeTOML (builtin_test.go) ⇄ providers/claude.toml
  Oracles: `TestBuiltinManifests_DecodeParity/{pi,claude}` + `TestProviderReferenceFiles_DecodeParity/{pi,claude}`.
  Array elements must match BYTE-FOR-BYTE — esp. the ONE string `"Bash(git:*),Read,Edit"` (commas are
  inside the TOML string, NOT array separators; keep it a single element). See research §2.

  ⚠️ **THE #2 design call — pi tooled = bare MINUS `--no-tools` (NO git allowlist — pi has none).** pi's
  tool-disable is a literal switch (`--no-tools`/`-nt`); tooled mode removes it → pi's native tool system ON.
  `TooledFlags = ["--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session"]`
  (the 5 remaining bare flags — still chrome-less + ephemeral). pi has NO `--allowed-tools`/`--tools`
  equivalent (external_deps.md §pi shows only all-or-nothing `--no-tools`), so pi's stager safety is enforced
  by the STAGER TASK PROMPT (§17.6) + stagecoach's ref-mutation monopoly (§13.6.2/§19), NOT by flag-scoping.
  Do NOT invent a non-existent allowlist flag. See research §1.

  ⚠️ **THE #3 design call — claude tooled = tools ENABLED + a git/read/edit allowlist.** claude's bare mode
  disables ALL tools (`--tools ""`). Tooled INVERTS it: enable tools restricted via an allowlist.
  `TooledFlags = ["--allowed-tools","Bash(git:*),Read,Edit","--setting-sources","","--no-session-persistence"]`
  (clean-slate + ephemeral carry over). See research §1.

  ⚠️ **THE #4 gotcha — `--allowed-tools` vs `--tools` discrepancy (TO CONFIRM at integration).**
  `external_deps.md §claude` (verified `claude --help` 2026-06-29) records the flag as `--tools <tools...>`;
  the item-spec CONTRACT says `--allowed-tools`; the codebase's synthetic tooled fixture
  (`render_test.go dualModeManifest` + `TestRender_TooledModeAppendsTooledFlags`) ALREADY uses
  `--allowed-tools`. **Decision: honor the item CONTRACT (`--allowed-tools`)** — authoritative for this work
  item, matches the codebase model, modern Claude Code exposes `--allowed-tools`. Carry a `# TO CONFIRM`
  note in the claude doc comment + providers/claude.toml so the first real stager run (P3.M2.T3) catches a
  wrong flag — the §12.7.2 progressive-verification discipline. See research §1.

  ⚠️ **THE #5 gotcha — target POST-S1 state; this task's edits are ADDITIVE on top of S1.** S1
  (reorder preferredBuiltins + pi `default_model=""`) edits the SAME files (builtin.go builtinPi,
  builtin_test.go piTOML/PiFields/render tests, providers/pi.toml). The repo is currently MID-S1
  (builtin.go has FR-D2; providers/pi.toml + builtin_test.go still show glm-5-turbo). **This task runs AFTER
  S1 completes** — assume S1's edits are in place and ADD `tooled_flags` on top. Do NOT revert
  `default_model=""`, do NOT un-split the pi render tests, do NOT touch preferredBuiltins/registry.go (that
  is S1's entire scope). If S1 has not fully landed (piTOML still shows glm-5-turbo), STOP and flag it.
  See research §3.

  ⚠️ **THE #6 gotcha — LEAVE the five non-stager providers + the synthetic tooled tests UNTOUCHED.**
  gemini/agy/opencode/codex/cursor keep `TooledFlags == nil` (their TOML literals + providers/*.toml have no
  tooled_flags key → nil ⇄ nil parity holds). `render_test.go`'s tooled tests use the SYNTHETIC
  `dualModeManifest` (they test Render's mode LOGIC, not the built-ins). `merge_test.go`/`manifest_test.go`
  test TooledFlags with synthetic fixtures. PRD.md's Appendix D is human-owned/read-only. None are touched.
  See research §4/§5/§6.

  Deliverable: MODIFIED `internal/provider/builtin.go` (`builtinPi()`+`builtinClaude()` gain non-empty
  `TooledFlags` + doc comments), `internal/provider/builtin_test.go` (`piTOML`/`claudeTOML` +tooled_flags;
  `PiFields`/`ClaudeFields` +TooledFlags assertions; +2 tooled render tests via real `Render`), `providers/pi.toml`
  + `providers/claude.toml` (+tooled_flags block + comment), `docs/providers.md` (schema row + mode ternary +
  stager subsection + capability marker). OUTPUT: pi + claude are stager-capable; the other five are not;
  all parity + render tests green. INPUT = the frozen `TooledFlags` field + `Render(...RenderTooled)` plumbing.
---

## Goal

**Feature Goal**: Populate `TooledFlags` on the two explicit-tool-disable providers (pi, claude) so they
can serve the v2 stager role (§11.5) — `Render(model, provider, sys, user, RenderTooled)` succeeds for pi
and claude (non-empty `tooled_flags`) and emits the tooled argv (tools on, git-scoped for claude); the other
five providers keep `TooledFlags == nil` (tooled render errors → FR-D4 fallback). Every parity oracle stays
green via the 4-way `tooled_flags` chain; the docs show which providers are stager-capable.

**Deliverable** (all EDITS — no new files):
1. `internal/provider/builtin.go`: `builtinPi()` + `builtinClaude()` each gain a non-empty `TooledFlags`
   `[]string` literal (exact values below) + a doc-comment note (claude's carries the `--allowed-tools`
   `# TO CONFIRM`). S1's edits preserved.
2. `internal/provider/builtin_test.go`: `piTOML`/`claudeTOML` gain a matching `tooled_flags = [...]` block;
   `PiFields`/`ClaudeFields` gain a `TooledFlags` `reflect.DeepEqual` assertion; +2 new tests
   (`RenderedCommand_Pi_Tooled`, `RenderedCommand_Claude_Tooled`) calling the real `Render(...,RenderTooled)`.
3. `providers/pi.toml` + `providers/claude.toml`: each gains a `tooled_flags = [...]` block + a
   tooled-render comment. S1's pi.toml `default_model=""` preserved.
4. `docs/providers.md`: schema table gains a `tooled_flags` row; "Command rendering" gains the bare/tooled
   ternary; NEW "Tooled mode and the stager role" subsection lists stager-capable (pi, claude) vs not;
   provider table gains a "Stager?" marker.

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `builtinPi().TooledFlags`
and `builtinClaude().TooledFlags` are non-empty with the exact values; the 4-way parity holds
(builtin ⇄ piTOML/claudeTOML ⇄ providers/{pi,claude}.toml — both `DecodeParity` oracles green);
`Render(...,RenderTooled)` succeeds for pi/claude and emits the asserted tooled argv; the five non-stager
providers' `TooledFlags` stay nil (their parity still green); gemini/agy/opencode/codex/cursor tests,
`render_test.go`, `merge_test.go`, `manifest_test.go`, `registry.go` byte-unchanged; go.mod unchanged.

## User Persona

**Target User**: The stager role (P3.M2.T3) — the per-concept staging agent that must run `git add` and
apply hunks. It calls `manifest.Render(model, provider, sys, user, RenderTooled)`. Only a provider with
non-empty `TooledFlags` can fill it; FR-D4 falls back to the next stager-capable provider when the chosen
default can't. Transitively, every user who runs multi-commit decomposition (`stagecoach --commits`).

**Use Case**: A user runs `stagecoach --commits` on a dirty tree. The planner proposes N concepts. For each,
the stager (a pi or claude manifest rendered in `RenderTooled`) stages exactly concept[i]'s subset. pi's
tooled profile runs with its native tools on (no chrome); claude's runs with a git/read/edit allowlist.

**User Journey**: (internal, v2) config picks a stager-capable provider → `Render(...,RenderTooled)` → argv
uses `tooled_flags` (not `bare_flags`) → executor runs the tooled agent → it mutates the index only →
stagecoach snapshots the frozen tree and commits. This subtask is what makes pi/claude ELIGIBLE for that role.

**Pain Points Addressed**: Without non-empty `TooledFlags`, pi/claude render-tooled ERRORS ("tooled mode
requires non-empty tooled_flags") → no provider can stager → multi-commit decomposition can't run. This
task lands the two providers that can (the explicit-switch pair — the only ones with a clean "tools on"
story per §12.7.1).

## Why

- **Unblocks the stager role (P3.M2.T3).** The decompose pipeline's stager is the one role that needs tools
  on. Only pi and claude (§12.7.1 explicit-switch providers) can express a clean tooled profile; landing
  their `TooledFlags` now makes them stager-eligible so P3.M2.T3 has real targets.
- **The §12.7.1 inversion, realized.** Bare mode disables tools (pi `--no-tools`, claude `--tools ""`);
  tooled mode INVERTS — tools on, scoped (claude allowlist) or chrome-stripped (pi). `tooled_flags` is the
  field that expresses each provider's "tooled but safe" idiom. This task populates it for the two that have
  a verified story; the other five honestly stay nil (cannot stager in v2).
- **No logic change.** The `TooledFlags` field, the Merge regime-2 (wholesale-replace), and the
  `Render(RenderTooled)` branch all EXIST (P1.M1). This task is pure DATA (two `[]string` literals) + parity
  sync + docs — the cheapest possible way to make two providers stager-capable.
- **Honest scoping.** Only pi + claude get `TooledFlags` now. gemini/agy/opencode/codex/cursor stay nil
  (no verified tooled combo; agy is experimental). FR-D4 handles the fallback. This matches §12.7.1's
  "empty tooled_flags ⇒ cannot serve as a stager; it can still serve the bare roles."

## What

Two `[]string` literals (pi, claude) + their 4-way parity sync + 2 tooled render tests + docs. No new
files, no new types, no behavioral logic change. Render/Merge/Validate/Resolve/registry are frozen.

### Success Criteria

- [ ] `builtinPi().TooledFlags == []string{"--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session"}`
      (5 tokens — bare MINUS `--no-tools`).
- [ ] `builtinClaude().TooledFlags == []string{"--allowed-tools","Bash(git:*),Read,Edit","--setting-sources","","--no-session-persistence"}`
      (5 tokens, incl. the one `Bash(git:*),Read,Edit` string + the `""` value arg to `--setting-sources`).
- [ ] `piTOML` + `claudeTOML` (builtin_test.go) each carry a matching `tooled_flags = [...]` block (after
      `bare_flags`, before `output`). Elements byte-for-byte equal to the Go literals.
- [ ] `providers/pi.toml` + `providers/claude.toml` each carry a matching `tooled_flags = [...]` block.
- [ ] `PiFields` + `ClaudeFields` each gain a `reflect.DeepEqual` assertion on `TooledFlags`.
- [ ] `TestBuiltinManifests_DecodeParity/{pi,claude}` GREEN (builtin ⇄ TOML literal, now incl. tooled_flags).
- [ ] `TestProviderReferenceFiles_DecodeParity/{pi,claude}` GREEN (builtin ⇄ providers/*.toml).
- [ ] NEW `TestBuiltinManifests_RenderedCommand_Pi_Tooled`: `builtinPi().Render("glm-5-turbo","zai","<sys>","<user>",RenderTooled)`
      → `spec.Args` = `["--provider","zai","--model","glm-5-turbo","--system-prompt","<sys>","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session","-p"]`
      (NO `--no-tools`); `spec.Stdin == "<user>"`; NO error.
- [ ] NEW `TestBuiltinManifests_RenderedCommand_Claude_Tooled`: `builtinClaude().Render("sonnet","","<sys>","<user>",RenderTooled)`
      → `spec.Args` = `["--model","sonnet","--system-prompt","<sys>","--allowed-tools","Bash(git:*),Read,Edit","--setting-sources","","--no-session-persistence","-p"]`
      (uses `--allowed-tools`, NOT `--tools ""`); `spec.Stdin == "<user>"`; NO error.
- [ ] gemini/agy/opencode/codex/cursor `TooledFlags` stay nil (their TOML + providers/*.toml unchanged);
      `AgyFields`'s `TooledFlags == nil` assertion still passes.
- [ ] S1's edits preserved: `builtinPi().DefaultModel == strPtr("")`; piTOML `default_model = ""`;
      `providers/pi.toml` `default_model = ""`; the pi render-test split; `preferredBuiltins` FR-D1 order.
- [ ] `docs/providers.md`: schema table has a `tooled_flags` row; "Command rendering" notes the bare/tooled
      ternary; a "Tooled mode and the stager role" subsection lists stager-capable (pi, claude) vs not;
      provider table has a "Stager?" marker.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean on edited files;
      `git diff --exit-code go.mod go.sum` empty; `render.go`/`merge.go`/`manifest.go`/`registry.go`/
      `render_test.go`/`merge_test.go`/`manifest_test.go`/`referencefiles_test.go` byte-unchanged; the five
      non-stager providers' constructors + TOML + tests byte-unchanged; PRD.md byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the two exact `TooledFlags`
`[]string` values, the 4-way parity rule (which 4 artifacts + which 2 oracles), the exact tooled-render
argvs (computed below), the S1 post-state to preserve, the LEAVE list, and the docs additions. No
git/generate/prompt knowledge required — this is two literals + parity sync + docs.

### Documentation & References

```yaml
# MUST READ — the authoritative research (every edit + parity + gotchas)
- docfile: plan/002_a17bb6c8dc1d/P1M2T2S2/research/tooled-flags-pi-claude.md
  why: the exact TooledFlags values (§1), the 4-way parity chain (§2 — THE trap), the S1 post-state to
       preserve (§3), the test strategy (§4 — what to update/add/leave), the docs scope (§5), files
       touched/frozen (§6). The single most important read.
  critical: §2 (the 4-way tooled_flags parity — one stale/omitted entry fails both DecodeParity oracles)
       and §1 (the exact values incl. the single `"Bash(git:*),Read,Edit"` string element + the
       `--allowed-tools` TO CONFIRM).

# The PRD basis
- file: plan/002_a17bb6c8dc1d/prd_snapshot.md   (or PRD.md)
  section: "11.5 Two invocation modes: bare and tooled" (h3.42) — defines the two modes + that the stager
       is the ONLY tooled role. The "why" of TooledFlags.
  section: "12.1 The manifest schema" (h3.43) — the `tooled_flags` field definition + the §12.2 mode ternary
       ("tooled with no tooled_flags defined => error"). The authoritative field semantics.
  section: "12.7.1 The tools-disable asymmetry" (h4.1) — the conceptual basis: pi+claude are explicit-switch
       providers (the only ones with a clean "tools on" story); the stager INVERTS bare; empty tooled_flags
       ⇒ not stager-capable. The safety model (tooled_flags + ref-mutation monopoly + stager prompt).
  critical: §12.1 says `tooled_flags` expresses "git-scoped allowlist + non-interactive approval mode" per
       provider idiom; nil/empty ⇒ cannot serve as a stager. §12.7.1: the stager's safety is "tools scoped
       to staging, never commit/update-ref/push."

# The CONTRACT (item spec) — the exact TooledFlags values are authoritative
  - pi:     ["--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session"]
  - claude: ["--allowed-tools","Bash(git:*),Read,Edit","--setting-sources","","--no-session-persistence"]
  - gemini/agy/opencode/codex/cursor: nil/empty (unchanged).

# The frozen plumbing (assume COMPLETE — do NOT edit)
- file: internal/provider/manifest.go
  why: confirms `TooledFlags []string `toml:"tooled_flags"`` EXISTS (P1.M1.T1.S1) and Resolve leaves it
       as-is (slice regime). This task only SETS the field on pi/claude.
  critical: do NOT edit manifest.go. The field + Resolve behavior are the contract.

- file: internal/provider/render.go
  why: confirms `RenderMode`/`RenderTooled` + the mode ternary (`case RenderTooled: if len(r.TooledFlags)==0
       {return error}; args=append(args, r.TooledFlags...)`) EXIST (P1.M1.T2.S1). The tooled render tests
       call this. NO logic change.
  critical: the tooled branch ERRORS on empty TooledFlags — that's why pi/claude MUST be non-empty to stager,
       and why the other five correctly error (FR-D4 fallback).

- file: internal/provider/merge.go
  why: confirms TooledFlags merge is regime-2 (len>0 → wholesale replace; empty/nil → inherit). A user
       override of tooled_flags REPLACES the built-in's. NO change.

# The prerequisite (assume COMPLETE) — the S1 contract
- file: plan/002_a17bb6c8dc1d/P1M2T2S1/PRP.md
  why: S1 (reorder preferredBuiltins + pi default_model="") edits builtin.go builtinPi, builtin_test.go
       piTOML/PiFields/render tests, providers/pi.toml, registry.go. This task runs AFTER S1 and ADDS
       tooled_flags on top. Read it to know the post-S1 state you must preserve.
  critical: do NOT duplicate/undo S1. If piTOML still shows glm-5-turbo (S1 not landed), STOP and flag.

# The files to edit (read each before editing)
- file: internal/provider/builtin.go
  section: builtinPi() (~L40) + builtinClaude() (~L70). Add `TooledFlags: []string{...}` to each + a
       doc-comment note. Leave the other five constructors untouched.
  pattern: mirror the existing field-literal style (one `[]string{}` literal, inline comments). Keep
       builtin.go ZERO-import.

- file: internal/provider/builtin_test.go
  section: piTOML + claudeTOML consts (add `tooled_flags = [...]` after `bare_flags`); PiFields (Test 3) +
       ClaudeFields (Test 4) (add a `TooledFlags` reflect.DeepEqual assertion). ADD two tooled render tests
       (call the REAL Render, not the renderArgs helper).
  critical: the DecodeParity table (Test 6) auto-covers — do NOT edit the table, only the TOML literals.
       The `renderArgs` helper is BARE-ONLY (no mode param) — use the real `Render(...,RenderTooled)` for
       the tooled tests (S1's render tests depend on renderArgs's signature; don't change it).

- file: providers/pi.toml + providers/claude.toml
  section: add a `# --- tooled mode (v2; §11.5) ---` block with `tooled_flags = [...]` (after the
       `bare_flags` block, before `# --- output ---`). Add a tooled-render comment. Preserve S1's pi.toml
       `default_model = ""` + rendered-command placeholders.

- file: docs/providers.md
  section: "The schema" table (+tooled_flags row); "Command rendering" (+bare/tooled ternary); NEW
       "Tooled mode and the stager role" subsection; "The 7 built-in providers" table (+Stager? marker).

# The LEAVE files (DO NOT EDIT)
- file: internal/provider/render_test.go   — tooled tests use the SYNTHETIC dualModeManifest (Render mode
       logic, not the built-ins). Unaffected.
- file: internal/provider/merge_test.go + manifest_test.go   — TooledFlags merge/Resolve tested with
       synthetic fixtures. Unaffected.
- file: internal/provider/referencefiles_test.go   — the parity oracle itself (it enforces the sync; it is
       NOT edited — it already loops providerFiles). Unaffected.
- file: internal/provider/registry.go + registry_test.go   — S1's scope (preferredBuiltins). Unaffected.
- the five non-stager providers' constructors + geminiTOML/opencodeTOML/codexTOML/cursorTOML/agyTOML +
       providers/{gemini,opencode,codex,cursor,agy}.toml + their *Fields tests   — TooledFlags stays nil.
- PRD.md (Appendix D is human-owned/read-only)   — do NOT modify.
```

### Current Codebase tree (relevant slice — POST-S1 assumed)

```bash
internal/provider/
  builtin.go               # builtinPi + builtinClaude — EDIT (+TooledFlags +doc comment). Others untouched.
  builtin_test.go          # piTOML/claudeTOML + PiFields/ClaudeFields + 2 new tooled render tests — EDIT
  manifest.go              # Manifest type (TooledFlags field EXISTS) — UNCHANGED
  merge.go / render.go     # TooledFlags merge + RenderTooled mode (EXIST) — UNCHANGED
  registry.go              # preferredBuiltins (S1 FR-D1) — UNCHANGED (S1's scope)
  render_test.go / merge_test.go / manifest_test.go / referencefiles_test.go — UNCHANGED
providers/
  pi.toml                  # +tooled_flags block + comment (preserve S1 default_model="") — EDIT
  claude.toml              # +tooled_flags block + comment — EDIT
  {gemini,opencode,codex,cursor,agy}.toml — UNCHANGED (TooledFlags stays nil)
docs/
  providers.md             # schema row + mode ternary + stager subsection + Stager? marker — EDIT
go.mod / go.sum            # UNCHANGED (pure data + docs; no new dep)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. All edits are in-place modifications to the 5 files listed above.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — 4-WAY PARITY on tooled_flags): TooledFlags is a plain []string. go-toml decodes a present
// array → non-nil; absent key → nil (S1 FINDING C/D). Setting NON-EMPTY TooledFlags in builtinPi/builtinClaude
// REQUIRES the IDENTICAL tooled_flags = [...] in piTOML/claudeTOML (builtin_test.go) AND providers/pi.toml/
// claude.toml. Two reflect.DeepEqual oracles enforce it: TestBuiltinManifests_DecodeParity + TestProviderReferenceFiles_DecodeParity.
// One stale/omitted entry (non-nil ≠ nil) fails BOTH. Elements byte-for-byte (esp. the single "Bash(git:*),Read,Edit" string).

// CRITICAL (#2 — pi has NO git allowlist): pi's TooledFlags = bare MINUS --no-tools (5 flags). pi's --help
// (external_deps.md §pi) shows only all-or-nothing --no-tools; there is NO --allowed-tools/--tools. Do NOT
// invent one. pi's stager safety = stager task prompt (§17.6) + stagecoach's ref-mutation monopoly, not flag-scoping.

// CRITICAL (#3 — claude --allowed-tools TO CONFIRM): external_deps.md §claude records --tools; the item
// CONTRACT + codebase render fixtures use --allowed-tools. HONOR --allowed-tools (verbatim per item) but
// carry a # TO CONFIRM note in claude's doc comment + providers/claude.toml. First real stager run (P3.M2.T3)
// verifies. The single string "Bash(git:*),Read,Edit" is ONE array element (commas inside the TOML string).

// CRITICAL (#4 — target POST-S1 state; ADDITIVE only): this task runs AFTER S1. builtin.go builtinPi already
// has DefaultModel=strPtr("") (FR-D2); S1 also sets piTOML default_model="", providers/pi.toml default_model="",
// splits the pi render tests, reorders preferredBuiltins. Do NOT revert any of those. ADD tooled_flags on top.
// If piTOML still shows glm-5-turbo → S1 not landed → STOP and flag.

// CRITICAL (#5 — use the REAL Render for tooled tests, NOT renderArgs): the builtin_test.go renderArgs helper
// is BARE-ONLY (appends BareFlags, no mode param) and S1's render tests depend on its signature. For the
// tooled render tests call builtinPi().Render(..., RenderTooled) / builtinClaude().Render(..., RenderTooled)
// directly and assert spec.Args + spec.Stdin. Do NOT add a mode param to renderArgs.

// GOTCHA (TooledFlags is a SLICE, not a pointer): it uses the slice merge regime (regime-2: len>0 → wholesale
// replace). A non-nil EMPTY slice would also pass len()==0 (treated as "not overridden" on merge) — but pi/claude
// here are NON-empty, so it's a clean replace target. No nil-vs-empty subtlety for pi/claude (they're populated).
// (The opencode/cursor NON-NIL-EMPTY BareFlags/Subcommand subtlety does NOT apply to TooledFlags here.)

// GOTCHA (LEAVE the five non-stager providers + synthetic tests): gemini/agy/opencode/codex/cursor stay nil;
// their TOML literals + providers/*.toml have no tooled_flags key (nil ⇄ nil parity holds). render_test.go's
// tooled tests use the SYNTHETIC dualModeManifest. merge_test.go/manifest_test.go use synthetic TooledFlags
// fixtures. PRD.md Appendix D is read-only. NONE are touched.

// GOTCHA (DecodeParity table auto-covers): TestBuiltinManifests_DecodeParity is a table-driven loop over
// {pi,claude,gemini,...}. Do NOT edit the table — just add tooled_flags to the piTOML/claudeTOML LITERALS.
// The loop will then reflect.DeepEqual the populated TooledFlags automatically.

// GOTCHA (sys goes via flag for pi/claude): both pi+claude have system_prompt_flag="--system-prompt" (NON-empty),
// so in the tooled render tests spec.Stdin is just "<user>" (sys emitted via the flag, NOT prepended). Assert
// spec.Stdin == "<user>", NOT "<sys>\n\n<user>".
```

## Implementation Blueprint

### Data models and structure

No new types. Two `[]string` literals added to existing constructors:

```go
// internal/provider/builtin.go — ADD TooledFlags to builtinPi() (after BareFlags, before Output).
// (builtinPi's other fields — incl. S1's DefaultModel=strPtr("") — UNCHANGED.)
func builtinPi() Manifest {
	return Manifest{
		// ... existing fields UNCHANGED ...
		BareFlags: []string{
			"--no-tools", "--no-extensions", "--no-skills",
			"--no-prompt-templates", "--no-context-files", "--no-session",
		},
		// TOOLED MODE (v2 §11.5 — the stager role). pi has no git-scoped allowlist (--help shows only the
		// all-or-nothing --no-tools), so pi's tooled profile = the bare invocation MINUS --no-tools: pi's
		// native tool system ON, everything else still off (chrome-less + ephemeral). The stager's safety
		// (git-only, never commit/update-ref/push) is enforced by the stager task prompt (§17.6) + stagecoach's
		// monopoly on ref mutations (§13.6.2/§19), not by flag-scoping.
		TooledFlags: []string{
			"--no-extensions",
			"--no-skills",
			"--no-prompt-templates",
			"--no-context-files",
			"--no-session",
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// ...
	}
}

// ADD TooledFlags to builtinClaude() (after BareFlags, before Output).
func builtinClaude() Manifest {
	return Manifest{
		// ... existing fields UNCHANGED ...
		BareFlags: []string{
			"--tools", "", "--setting-sources", "", "--no-session-persistence",
		},
		// TOOLED MODE (v2 §11.5 — the stager role). INVERTS claude's bare mode: instead of --tools "" (disable
		// ALL tools), ENABLE tools RESTRICTED via an allowlist to Bash(git:*) (git only) + Read + Edit — the
		// staging-relevant toolset. --setting-sources "" (clean slate) + --no-session-persistence (ephemeral)
		// carry over from bare. # TO CONFIRM (integration, P3.M2.T3): external_deps.md §claude records --tools;
		// the item contract + codebase use --allowed-tools (the explicit-enable allow-list flag). Verify against
		// a real claude --help at the first stager run; if wrong, swap the flag token (the value is the allowlist).
		TooledFlags: []string{
			"--allowed-tools", "Bash(git:*),Read,Edit",
			"--setting-sources", "",
			"--no-session-persistence",
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// ...
	}
}
```

### The decode-parity TOML fixtures (ADD to piTOML + claudeTOML in builtin_test.go)

Insert AFTER the `bare_flags = [...]` array and BEFORE `output = "raw"` (matching manifest.go field order):

```toml
# piTOML — add this block (the 5 tooled flags, identical to builtinPi().TooledFlags):
tooled_flags = [
  "--no-extensions",
  "--no-skills",
  "--no-prompt-templates",
  "--no-context-files",
  "--no-session",
]

# claudeTOML — add this block (identical to builtinClaude().TooledFlags; "Bash(git:*),Read,Edit" is ONE string):
tooled_flags = [
  "--allowed-tools", "Bash(git:*),Read,Edit",
  "--setting-sources", "",
  "--no-session-persistence",
]
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD TooledFlags to builtinPi() + builtinClaude() (builtin.go)
  - EDIT builtinPi(): add the TooledFlags []string literal (5 flags: bare minus --no-tools) AFTER BareFlags.
  - EDIT builtinClaude(): add the TooledFlags []string literal (5 tokens incl. --allowed-tools + the "" value
      arg) AFTER BareFlags.
  - ADD a doc-comment note on each constructor (pi: tools-on, no allowlist, safety via prompt+ref-monopoly;
      claude: allowlist inversion + the --allowed-tools # TO CONFIRM). Keep builtin.go ZERO-import.
  - DO NOT touch the other five constructors, DefaultModel/DefaultProvider, or any other field.
  - PRESERVE S1's DefaultModel=strPtr("") on pi.

Task 2: SYNC the parity TOML literals + field assertions (builtin_test.go)
  - EDIT piTOML: add the `tooled_flags = [ 5 flags ]` block (after bare_flags, before output).
  - EDIT claudeTOML: add the `tooled_flags = [ 5 tokens ]` block.
  - EDIT PiFields (Test 3): ADD `wantTooled := []string{"--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session"}; if !reflect.DeepEqual(m.TooledFlags, wantTooled) { t.Errorf(...) }`.
  - EDIT ClaudeFields (Test 4): ADD the equivalent TooledFlags DeepEqual assertion with claude's 5 tokens.
  - DO NOT edit the DecodeParity table (Test 6) — it auto-covers via the loop. DO NOT change renderArgs.
  - PRESERVE S1's piTOML default_model="" + PiFields DefaultModel=="" assertion + the pi render-test split.

Task 3: ADD the two tooled render tests (builtin_test.go) — call the REAL Render
  - ADD TestBuiltinManifests_RenderedCommand_Pi_Tooled:
      spec, err := builtinPi().Render("glm-5-turbo", "zai", "<sys>", "<user>", RenderTooled)
      assert err == nil; assert spec.Args == ["--provider","zai","--model","glm-5-turbo","--system-prompt",
        "<sys>","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session","-p"]
        (NO --no-tools); assert spec.Stdin == "<user>" (sys via flag).
  - ADD TestBuiltinManifests_RenderedCommand_Claude_Tooled:
      spec, err := builtinClaude().Render("sonnet", "", "<sys>", "<user>", RenderTooled)
      assert err == nil; assert spec.Args == ["--model","sonnet","--system-prompt","<sys>","--allowed-tools",
        "Bash(git:*),Read,Edit","--setting-sources","","--no-session-persistence","-p"] (uses --allowed-tools,
        NOT --tools ""); assert spec.Stdin == "<user>".
  - Use `reflect.DeepEqual(spec.Args, want)`; check `err != nil` with t.Fatalf. RenderMode/RenderTooled are
      same-package symbols (no import). spec is *CmdSpec.

Task 4: SYNC the provider reference files (providers/pi.toml + providers/claude.toml)
  - EDIT providers/pi.toml: add a `# --- tooled mode (v2; §11.5) ---` block with `tooled_flags = [ 5 flags ]`
      (after the bare_flags block, before `# --- output ---`). Add a tooled-render comment line.
  - EDIT providers/claude.toml: add the same block with claude's 5 tokens + the # TO CONFIRM note in the comment.
  - PRESERVE S1's pi.toml `default_model = ""` + the rendered-command placeholder comment. Do NOT remove any field.

Task 5: UPDATE docs/providers.md (Mode A)
  - SCHEMA table: ADD a `tooled_flags` row (list of string; default `nil`; "flags for tooled/stager mode —
      tools ON, git-scoped, non-interactive. nil/empty ⇒ not stager-capable."). (Optionally also add the
      missing `experimental` row for completeness — low priority, note it.)
  - COMMAND RENDERING section: after the `bare_flags...` line, add: "In tooled mode (the stager role),
      `tooled_flags` replaces `bare_flags`; tooled mode with empty `tooled_flags` errors (that provider
      cannot serve as a stager)."
  - NEW SUBSECTION "## Tooled mode and the stager role" (after "Tools-disable asymmetry"): the two modes
      (§11.5) + the stager-capable list (pi, claude: yes; gemini/agy/opencode/codex/cursor: no) + the safety
      model (§12.7.1: tools scoped to staging, never commit/update-ref/push; FR-D4 fallback).
  - PROVIDER TABLE: ADD a "Stager?" column (pi=yes, claude=yes, gemini/agy/opencode/codex/cursor=no).
  - DO NOT fix the pre-existing stale pi `default_model` cell (S1/P4.M3 docs debt) — note it, leave it.

Task 6: VERIFY (no further edits)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. render.go/merge.go/manifest.go/
      registry.go + render_test.go/merge_test.go/manifest_test.go/referencefiles_test.go byte-unchanged.
      The five non-stager providers' constructors + TOML + tests byte-unchanged. PRD.md byte-unchanged.
      S1's edits intact. All DecodeParity + reference-file + tooled-render tests green.
```

### Implementation Patterns & Key Details

```go
// THE parity invariant (the entire "logic" of this task — 4 artifacts, identical array):
//   builtinPi().TooledFlags  ==  piTOML "tooled_flags = [...]"  ==  providers/pi.toml "tooled_flags = [...]"
//   builtinClaude().TooledFlags == claudeTOML "tooled_flags" == providers/claude.toml "tooled_flags"
// Both reflect.DeepEqual oracles (TestBuiltinManifests_DecodeParity + TestProviderReferenceFiles_DecodeParity)
// enforce it. The single "Bash(git:*),Read,Edit" element must be byte-identical across all four.

// THE tooled render tests — use the REAL Render (RenderTooled), NOT the bare-only renderArgs helper:
func TestBuiltinManifests_RenderedCommand_Pi_Tooled(t *testing.T) {
	spec, err := builtinPi().Render("glm-5-turbo", "zai", "<sys>", "<user>", RenderTooled)
	if err != nil {
		t.Fatalf("pi tooled render error: %v", err)
	}
	want := []string{
		"--provider", "zai", "--model", "glm-5-turbo", "--system-prompt", "<sys>",
		"--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session",
		"-p", // print_flag LAST (§12.2); NO --no-tools (tools on)
	}
	if !reflect.DeepEqual(spec.Args, want) {
		t.Errorf("pi tooled Args:\n got %v\nwant %v", spec.Args, want)
	}
	if spec.Stdin != "<user>" { // sys via --system-prompt flag → only user payload on stdin
		t.Errorf("pi tooled Stdin = %q, want %q", spec.Stdin, "<user>")
	}
}

func TestBuiltinManifests_RenderedCommand_Claude_Tooled(t *testing.T) {
	spec, err := builtinClaude().Render("sonnet", "", "<sys>", "<user>", RenderTooled)
	if err != nil {
		t.Fatalf("claude tooled render error: %v", err)
	}
	want := []string{
		"--model", "sonnet", "--system-prompt", "<sys>",
		"--allowed-tools", "Bash(git:*),Read,Edit", // tools ENABLED + git/read/edit allowlist (NOT --tools "")
		"--setting-sources", "",
		"--no-session-persistence",
		"-p",
	}
	if !reflect.DeepEqual(spec.Args, want) {
		t.Errorf("claude tooled Args:\n got %v\nwant %v", spec.Args, want)
	}
	if spec.Stdin != "<user>" {
		t.Errorf("claude tooled Stdin = %q, want %q", spec.Stdin, "<user>")
	}
}

// THE field assertions (add to PiFields / ClaudeFields):
wantTooled := []string{"--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session"}
if !reflect.DeepEqual(m.TooledFlags, wantTooled) {
	t.Errorf("TooledFlags = %v, want %v", m.TooledFlags, wantTooled)
}
// (claude: wantTooled = []string{"--allowed-tools","Bash(git:*),Read,Edit","--setting-sources","","--no-session-persistence"})
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): change NONE. Pure data + parity sync + docs. `go mod tidy` MUST be a no-op.
      `git diff --exit-code go.mod go.sum` MUST be empty. builtin.go stays ZERO-import.

PACKAGE EDGES: NONE added/removed. No import changes anywhere (RenderMode/RenderTooled/CmdSpec are
      same-package symbols; the tooled render tests use the existing Render method).

FROZEN/LEAVE (do NOT edit):
  - manifest.go (TooledFlags field EXISTS), merge.go (regime-2 EXISTS), render.go (RenderTooled EXISTS),
    registry.go (S1's preferredBuiltins).
  - render_test.go / merge_test.go / manifest_test.go / referencefiles_test.go (synthetic fixtures / the
    parity oracle itself).
  - the five non-stager providers' constructors + TOML literals + providers/*.toml + *Fields tests.
  - PRD.md (Appendix D — human-owned/read-only).

DOWNSTREAM (NOT this task):
  - P3.M2.T3 (stager): calls manifest.Render(..., RenderTooled). pi/claude now succeed; the other five error
    ("tooled mode requires non-empty tooled_flags") → FR-D4 falls back to a stager-capable provider.
  - The --allowed-tools flag is verified at the first real stager run (P3.M2.T3 / real-agent scaffold) — the
    # TO CONFIRM note surfaces it. If wrong, swap the flag token (the allowlist value is what matters).

NO NEW DATABASE / ROUTES / CLI COMMANDS / FILES.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
go build ./...          # Expect clean.
go vet ./...            # Expect clean.
gofmt -l internal/provider/ providers/ docs/ 2>/dev/null; gofmt -w internal/provider/builtin.go internal/provider/builtin_test.go
test -z "$(gofmt -l internal/provider/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"   # MUST be empty.
# builtin.go STILL zero-import:
grep -n '^import\|^	"' internal/provider/builtin.go && echo "note: builtin.go has imports (should be NONE)" || echo "builtin.go zero-imports (good)"
# Confirm FROZEN files untouched:
git diff --exit-code internal/provider/manifest.go internal/provider/merge.go internal/provider/render.go internal/provider/registry.go internal/provider/render_test.go internal/provider/merge_test.go internal/provider/manifest_test.go internal/provider/referencefiles_test.go PRD.md && echo "FROZEN files UNCHANGED (expected)"
# Confirm the five non-stager providers untouched:
git diff --exit-code -- providers/gemini.toml providers/opencode.toml providers/codex.toml providers/cursor.toml providers/agy.toml && echo "non-stager TOML UNCHANGED (expected)"
```

### Level 2: Provider-package unit tests (the parity + render oracles)

```bash
go test ./internal/provider/... -v
# Expected PASS — verify explicitly:
#   TestBuiltinManifests_DecodeParity/pi ......... piTOML tooled_flags matches builtinPi (4-way parity)
#   TestBuiltinManifests_DecodeParity/claude ..... claudeTOML tooled_flags matches builtinClaude
#   TestProviderReferenceFiles_DecodeParity/pi ... providers/pi.toml tooled_flags matches builtinPi
#   TestProviderReferenceFiles_DecodeParity/claude  ... providers/claude.toml tooled_flags matches builtinClaude
#   TestBuiltinManifests_PiFields ................. TooledFlags DeepEqual (5 flags)
#   TestBuiltinManifests_ClaudeFields ............. TooledFlags DeepEqual (5 tokens)
#   TestBuiltinManifests_RenderedCommand_Pi_Tooled ..... RenderTooled succeeds; no --no-tools
#   TestBuiltinManifests_RenderedCommand_Claude_Tooled . RenderTooled succeeds; --allowed-tools present
#   TestBuiltinManifests_AgyFields ................. TooledFlags == nil (still — agy untouched)
#   TestRender_TooledModeAppendsTooledFlags ........ STILL green (synthetic — untouched)
#   TestRender_TooledModeEmptyFlagsErrors .......... STILL green (synthetic — untouched)
#   ALL gemini/opencode/codex/cursor tests ......... STILL green (TooledFlags stays nil)
```

### Level 3: Whole-repo build/test + CLI smoke

```bash
go build ./...   # Expect clean.
go test ./...    # Expect all PASS. If a non-provider package breaks, it is unrelated — investigate, do NOT
                 # revert the TooledFlags change (it is pure additive data + docs).
# Smoke the CLI show output (tooled_flags now appears for pi/claude):
go build -o /tmp/stagecoach ./cmd/stagecoach
/tmp/stagecoach providers show pi 2>/dev/null | grep "tooled_flags"     # → tooled_flags = [...] (non-empty)
/tmp/stagecoach providers show claude 2>/dev/null | grep "tooled_flags" # → tooled_flags = [...]
# Confirm the five non-stager providers show NO tooled_flags (nil → omitted on marshal):
/tmp/stagecoach providers show gemini 2>/dev/null | grep -c "tooled_flags"   # → 0 (absent)

# Straggler grep — confirm ONLY pi+claude have non-empty TooledFlags in the source:
grep -n 'TooledFlags:' internal/provider/builtin.go   # MUST show exactly 2 hits (pi + claude)
```

### Level 4: Behavioral spot-check (proves stager eligibility)

```bash
# pi + claude now render in tooled mode WITHOUT error (the stager precondition); the others error.
# (Covered by TestBuiltinManifests_RenderedCommand_{Pi,Claude}_Tooled; this is a CLI-level confirmation.)
go test ./internal/provider/ -run 'TestBuiltinManifests_RenderedCommand_(Pi|Claude)_Tooled' -v
# Expected: both PASS (err == nil). This is the proof pi+claude are stager-capable.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on edited files.
- [ ] `go test ./...` PASS (provider suite + no non-provider regression).
- [ ] go.mod/go.sum byte-unchanged; FROZEN files (manifest/merge/render/registry + their tests + referencefiles_test) byte-unchanged;
      the five non-stager providers' constructors + TOML + tests byte-unchanged; PRD.md byte-unchanged.
- [ ] builtin.go STILL zero-import.

### Feature Validation
- [ ] `builtinPi().TooledFlags` == the 5 bare-minus-`--no-tools` flags; `builtinClaude().TooledFlags` == the
      5 allowlist tokens (incl. `Bash(git:*),Read,Edit` + the `""` value arg).
- [ ] 4-way parity green: builtin ⇄ piTOML/claudeTOML ⇄ providers/{pi,claude}.toml (both DecodeParity oracles).
- [ ] `PiFields` + `ClaudeFields` assert `TooledFlags` (DeepEqual).
- [ ] `RenderedCommand_Pi_Tooled` + `RenderedCommand_Claude_Tooled` PASS (RenderTooled succeeds; tooled argv; spec.Stdin == "<user>").
- [ ] gemini/agy/opencode/codex/cursor `TooledFlags` stay nil (parity + AgyFields nil-assertion still green).
- [ ] S1's edits preserved (pi DefaultModel="", piTOML default_model="", providers/pi.toml default_model="", pi render-test split, preferredBuiltins FR-D1).

### Code Quality Validation
- [ ] Render/Merge/Validate/Resolve/registry LOGIC unchanged (pure additive data).
- [ ] Edits follow existing conventions ([]string literal style; assertStr/reflect.DeepEqual; comment style).
- [ ] No out-of-scope churn (LEAVE files/fixtures untouched; PRD.md untouched).
- [ ] Anti-patterns avoided (see below).

### Documentation
- [ ] `docs/providers.md`: `tooled_flags` schema row; bare/tooled mode ternary; "Tooled mode and the stager
      role" subsection (stager-capable list); provider-table "Stager?" marker.
- [ ] `builtin.go` doc comments note pi's no-allowlist story + claude's `--allowed-tools` # TO CONFIRM.
- [ ] `providers/pi.toml` + `providers/claude.toml` carry the `tooled_flags` block + a tooled-render comment.

---

## Anti-Patterns to Avoid

- ❌ **Don't break the 4-way `tooled_flags` parity.** builtin.go (Go literal), builtin_test.go (piTOML/claudeTOML),
  and providers/{pi,claude}.toml must ALL carry the IDENTICAL array. One stale/omitted entry (non-nil ≠ nil)
  fails BOTH DecodeParity oracles. The single `"Bash(git:*),Read,Edit"` element must be byte-identical everywhere.
- ❌ **Don't invent a pi git-allowlist flag.** pi's `--help` shows only all-or-nothing `--no-tools`. pi's tooled
  profile = bare MINUS `--no-tools` (tools on, no chrome). Safety is via the stager prompt + stagecoach's
  ref-mutation monopoly, not flag-scoping. Do NOT add a `--tools`/`--allowed-tools` to pi.
- ❌ **Don't use `--tools` for claude.** The item CONTRACT says `--allowed-tools`; the codebase render fixtures
  agree. Honor it (with a # TO CONFIRM). The first real stager run verifies; if wrong, swap the token later.
- ❌ **Don't add a mode param to `renderArgs`.** It's bare-only and S1's render tests depend on its signature.
  For the tooled render tests, call the REAL `Render(..., RenderTooled)` and assert `spec.Args`/`spec.Stdin`.
- ❌ **Don't touch the LEAVE files.** render_test.go (synthetic dualModeManifest), merge_test.go/manifest_test.go
  (synthetic TooledFlags fixtures), referencefiles_test.go (the oracle), registry.go (S1's scope), the five
  non-stager providers, and PRD.md (Appendix D — human-owned). Editing them is churn / out-of-scope.
- ❌ **Don't populate the other five providers' TooledFlags.** gemini/agy/opencode/codex/cursor stay nil — they
  cannot stager in v2 (no verified tooled combo; agy is experimental). FR-D4 handles fallback. Only pi+claude.
- ❌ **Don't revert S1's edits.** This task runs AFTER S1. Preserve pi `default_model=""`, the pi render-test
  split, and `preferredBuiltins` FR-D1. If piTOML still shows `glm-5-turbo`, S1 hasn't landed — STOP and flag.
- ❓ **Don't fix the stale pi `default_model` cell in docs/providers.md.** It's pre-existing doc debt (S1 didn't
  touch docs; P4.M3.T1.S1 owns the v2 docs sync). Note it; focus this task's doc edits on `tooled_flags`.
- ❌ **Don't edit the DecodeParity table.** It's a loop over all providers; adding `tooled_flags` to the piTOML/
  claudeTOML LITERALS is enough — the loop auto-covers. Editing the table is unnecessary churn.
- ❌ **Don't forget `spec.Stdin`.** pi+claude have a NON-empty `system_prompt_flag`, so in tooled mode the sys
  prompt goes via the FLAG and `spec.Stdin == "<user>"` (NOT the prepended `"<sys>\n\n<user>"`). Assert accordingly.

---

## Confidence Score

**9/10** — A well-scoped "two `[]string` literals + 4-way parity sync + 2 render tests + docs" task. Every edit
is enumerated, the parity chain (the one genuine trap) is flagged four ways, the tooled argvs are pre-computed
with exact `spec.Args`/`spec.Stdin`, the S1 post-state to preserve is spelled out, and the LEAVE list prevents
churn. The -1 reserves for the `--allowed-tools`/`--tools` flag uncertainty (carried as a # TO CONFIRM — the
manifest is correct DATA per the contract; the real flag is verified at the first stager run in P3.M2.T3, which
is well downstream and does not block this task's parity/render tests).
