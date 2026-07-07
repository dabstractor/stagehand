---
name: "P1.M4.T1.S1 — config_version advisory warning on load (PRD §9.17 FR-B4)"
description: |

  Add a load-time ADVISORY (no auto-migration) that warns on stderr when a loaded config FILE's
  `config_version` is missing, older, or newer than the compiled-in `CurrentConfigVersion` (PRD §9.17
  FR-B4). The warning is routed through the existing `noticeOut` sink (file.go) for testability and is
  checked AFTER all overlay layers so the highest-layer version wins.

  CONTRACT (PRD §9.17 FR-B4): "Every config file carries config_version = <int>; the binary knows
  CurrentConfigVersion. On load: if the file's version is missing or older, print a clear warning naming
  the mismatch and the remediation (stagecoach config upgrade, or config init --force); if newer, warn that
  the file is ahead of the binary. Advisory only — no automatic migration."

  DELIVERABLES (one source edit, two test updates, one pure helper + Load wiring + new tests + a docs header):
    1. MODIFY internal/config/config.go  — `Defaults()` leaves `ConfigVersion: 0` (NOT CurrentConfigVersion)
       so `0` is a genuine "missing/unset" sentinel (the task's "cfg.ConfigVersion == 0" logic is unreachable
       otherwise). Update the field + func doc comments.
    2. MODIFY internal/config/load.go    — add a pure `configVersionNotice(fileLoaded bool, version int)
       string` helper (mirrors `repoProviderNotice`); add a `var fileLoaded bool` set true when the global
       or repo-local file loads; emit `fmt.Fprint(noticeOut, configVersionNotice(fileLoaded, cfg.ConfigVersion))`
       at the end of `Load()` (happy path only).
    3. MODIFY internal/config/config_test.go  — update the `Defaults().ConfigVersion` assertion (expect 0).
    4. MODIFY internal/config/file_test.go    — update TestOverlay_V2Scalars part (b) (expect 0, not
       CurrentConfigVersion).
    5. MODIFY internal/config/load_test.go    — ADD tests: `TestConfigVersionNotice` (pure, all branches) +
       `TestLoad_ConfigVersionAdvisory_{Older,Missing,Current,Newer,NoFile}` (capture noticeOut, assert text).
    6. MODIFY internal/cmd/config.go          — update `exampleConfigTemplate` header to DOCUMENT
       config_version + the upgrade mechanism (Mode A).

  SCOPE NOTE (load-bearing, design §0): `ConfigVersion` + `CurrentConfigVersion` were ADDED to Config by
  P1.M3.T1.S1 and decoded in P1.M3.T1.S2 (both Complete). That work PINNED `Defaults().ConfigVersion` to
  `CurrentConfigVersion` (2). But with non-zero overlay semantics, that makes `cfg.ConfigVersion == 0`
  UNREACHABLE after Load — so the task's "missing" branch is dead code. THIS subtask (the advisory task,
  the one that makes ConfigVersion meaningful) OWNS changing `Defaults()` to leave it 0. That single edit
  makes `0` a real "no source declared a version" sentinel. Verified it breaks EXACTLY 2 shipped
  assertions (config_test.go:68, file_test.go:594) — both bounded, one-line updates enumerated in the PRP.
  No consumer besides the new advisory reads ConfigVersion; no test outside internal/config/ references it.

  SCOPE BOUNDARY (what this does NOT do — owned by siblings): NO auto-migration (FR-B4: "advisory only —
  no existing users to migrate"); NO `config upgrade` command (P1.M4.T3); NO populated `config init`
  (P1.M4.T2 rewrites init to a bootstrap — this subtask only adds a header doc to the EXISTING inert
  template); NO first-run auto-bootstrap (P1.M4.T4). The advisory WARNS only.

  INPUT (upstream — already built, read-only): `Config.ConfigVersion int` (config.go, P1.M3.T1.S1);
  `CurrentConfigVersion = 2` const (config.go); `fileConfig.ConfigVersion` + `materialize()` non-zero copy
  + `overlay()` non-zero merge (file.go, P1.M3.T1.S2); `noticeOut`/`SetNoticeOut`/`NoticeOut` +
  `repoProviderNotice` pure-function pattern (file.go); `Load()` (load.go).

  OUTPUT (downstream): P1.M4.T2 (populated config init) writes `config_version = CurrentConfigVersion`
  into its bootstrap output and should carry this subtask's template header note; P1.M4.T3 (`config
  upgrade`) is the remediation the warning points users to; P1.M4.T4 (first-run fallback) ensures a file
  always exists so the no-file path is transient (the fileLoaded guard keeps it quiet until then).

  ⚠️ `Defaults()` MUST change to 0 (design §0) — without it the "missing" branch is unreachable and FR-B4's
     "missing" case silently passes as current. Update the 2 breaking assertions (§3).
  ⚠️ The advisory is gated on `fileLoaded` (design §1) — do NOT warn on the no-file path (pure Defaults,
     cfg.ConfigVersion==0 but there is no file to be stale).
  ⚠️ Route through the EXISTING `noticeOut` var (design §2) — the task's "use the existing noticeOut pattern".
     Reusing it is verified-safe (no existing test asserts noticeOut content; the 3 capture-and-ignore
     tests still pass).
  ⚠️ Advisory text for the OLDER case is the task's verbatim wording; adapt missing/ahead per design §6.

  Deliverable: a clear stderr advisory when a config file's schema version mismatches the binary; no
  automatic migration. `go build ./... && go test ./...` green; go.mod/go.sum unchanged.

---

## Goal

**Feature Goal**: Emit a clear, testable, stderr-routed ADVISORY warning whenever a loaded Stagecoach config
FILE declares a `config_version` that is missing, older than, or newer than the compiled-in
`CurrentConfigVersion` (PRD §9.17 FR-B4), pointing the user at `stagecoach config upgrade` / `config init
--force`. The advisory is checked after all precedence overlays (highest-layer version wins), is purely
informational (no auto-migration), and is suppressed entirely when no config file is loaded. This requires
making `0` a genuine "no source declared a version" sentinel, which means `Defaults()` must stop pinning
`ConfigVersion` to `CurrentConfigVersion`.

**Deliverable** (1 source edit + 1 helper/wiring + 2 test updates + new tests + 1 docs header; go.mod
unchanged):
1. `internal/config/config.go` — `Defaults()` sets `ConfigVersion: 0`; updated doc comments.
2. `internal/config/load.go` — `configVersionNotice(fileLoaded bool, version int) string` (pure) +
   `fileLoaded` tracking in `Load()` + the `fmt.Fprint(noticeOut, …)` emit at the end of `Load()`.
3. `internal/config/config_test.go` — `Defaults().ConfigVersion` assertion updated (expect 0).
4. `internal/config/file_test.go` — `TestOverlay_V2Scalars` part (b) updated (expect 0).
5. `internal/config/load_test.go` — `TestConfigVersionNotice` + `TestLoad_ConfigVersionAdvisory_*`.
6. `internal/cmd/config.go` — `exampleConfigTemplate` header documents `config_version` + upgrade.

**Success Definition**: a config file with `config_version = 1` → `Load()` prints
`stagecoach: config file uses schema version 1; current is 2. Run 'stagecoach config upgrade' or 'stagecoach
config init --force'.` to `noticeOut`; a file with NO `config_version` → prints the "missing" advisory; a
file with `config_version = 3` → prints the "ahead" advisory; a file with `config_version = 2` → silent;
NO config file at all → silent (fileLoaded guard). `configVersionNotice` unit-tests all 5 branches.
`go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean; go.mod/go.sum byte-unchanged;
only the 6 listed files differ.

## User Persona

**Target User**: The Stagecoach user upgrading across schema versions (PRD §7.1 "the plan-holder"). When
Stagecoach bumps `CurrentConfigVersion` (a breaking config change), a user's existing config file is stale.
The advisory tells them their config is old and exactly how to refresh it — without silently misbehaving
or clobbering their hand-edits. Transitively: the future `config upgrade` (P1.M4.T3) and `config init
--force` commands the advisory points to.

**Use Case**: User upgrades stagecoach; runs `stagecoach`; sees `stagecoach: config file uses schema version
1; current is 2. Run 'stagecoach config upgrade' or 'stagecoach config init --force'.` on stderr; the commit
still proceeds (advisory only). Or: a hand-written config omits `config_version` → "missing" advisory
nudges them to add it.

**User Journey**: user runs `stagecoach` → `Load()` resolves config (defaults→file→git→env→flags) → after
overlay, the advisory compares the highest-layer `config_version` to `CurrentConfigVersion` → if mismatched
(and a file was loaded), the one-line warning prints to stderr → the pipeline continues normally.

**Pain Points Addressed**: (1) Silent staleness — an old config silently behaving wrong; solved by the
explicit warning. (2) Not knowing the remediation — solved by naming `config upgrade` / `config init
--force` in the message. (3) Auto-migration fear — the advisory is read-only; it never rewrites the user's
file (FR-B4: "advisory only — no existing users to migrate").

## Why

- **It IS the FR-B4 advisory.** PRD §9.17 FR-B4 mandates the missing/older/newer warning as the v2
  config-versioning safety net. This subtask implements exactly that.
- **Unblocks the config-versioning milestone (P1.M4).** `config upgrade` (P1.M4.T3) is the remediation
  the advisory points to; `config init` rewrite (P1.M4.T2) will emit `config_version =
  CurrentConfigVersion`; the advisory is the load-side half that DETECTS the need.
- **Makes `ConfigVersion` meaningful.** The field + const were added in P1.M3.T1.S1 but inert (Defaults
  pinned it to current, so it was never observably stale). This subtask is the first thing that READS it
  for a decision — and corrects the sentinel so "missing" is detectable.
- **No behavior change for current configs.** A file with `config_version = 2` (what the bootstrap will
  write) is silent. Only stale/missing/ahead files warn.

## What

A pure helper `configVersionNotice(fileLoaded, version)` returning the advisory text (or ""), a
`fileLoaded` boolean tracked across the global+repo-local file loads in `Load()`, and a single
`fmt.Fprint(noticeOut, …)` emit at the end of `Load()`. Plus the `Defaults()` edit (ConfigVersion→0) that
makes the "missing" branch reachable, its 2 test-assertion updates, the new advisory unit + integration
tests, and a config_version doc block in the `config init` template header. No new types, no new
dependencies, no new files, no auto-migration, no command registration.

### Success Criteria

- [ ] `internal/config/config.go`: `Defaults()` returns `ConfigVersion: 0` (the `ConfigVersion:
      CurrentConfigVersion` line removed). The `ConfigVersion` field doc + `Defaults()` doc say 0 = "unset;
      the load-time advisory compares the resolved value to CurrentConfigVersion".
- [ ] `internal/config/load.go`: a pure `func configVersionNotice(fileLoaded bool, version int) string`
      exists (mirrors `repoProviderNotice` — no I/O); `Load()` tracks `var fileLoaded bool` (set true in the
      global `g != nil` and repo `r != nil` branches) and emits `fmt.Fprint(noticeOut, configVersionNotice(
      fileLoaded, cfg.ConfigVersion))` immediately before `return &cfg, nil`.
- [ ] `configVersionNotice(false, _) == ""` (no file → silent); `(true, CurrentConfigVersion) == ""`
      (current → silent); `(true, 0)` is the "missing" message; `(true, 1)` (1<2) is the "older" message
      (task's verbatim text); `(true, 3)` (3>2) is the "ahead" message. All messages are `\n`-terminated and
      name `config upgrade` / `config init --force`.
- [ ] A config FILE with `config_version = 2` → `Load()` writes nothing to `noticeOut`; a file WITHOUT
      `config_version` → the "missing" advisory; `config_version = 1` → "older"; `config_version = 3` →
      "ahead". NO config file at all → nothing (fileLoaded guard).
- [ ] `internal/config/config_test.go`: the `Defaults().ConfigVersion` assertion expects `0`.
- [ ] `internal/config/file_test.go`: `TestOverlay_V2Scalars` part (b) expects `dst.ConfigVersion == 0`
      (Defaults pins 0; nil src must not clobber); its `(a)` still expects 3 (src non-zero wins).
- [ ] `internal/config/load_test.go`: `TestConfigVersionNotice` (5 branches) + `TestLoad_ConfigVersionAdvisory_{
      Older,Missing,Current,Newer,NoFile}` all PASS.
- [ ] `internal/cmd/config.go`: `exampleConfigTemplate` has a commented header block documenting
      `config_version` (top-level metadata, not a precedence layer), `CurrentConfigVersion`, the
      missing/older/ahead advisory, and the `config upgrade` / `config init --force` remediation.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/` clean; go.mod/go.sum
      byte-unchanged; ONLY the 6 listed files differ.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact `Defaults()` edit
+ its 2 test updates (§3, verbatim), the pure `configVersionNotice` signature + the 5 message strings
(§6, verbatim), the `fileLoaded` tracking + emit placement in `Load()` (§5, verbatim), the
`noticeOut`/`repoProviderNotice` pattern to mirror (quoted), the 6 new test cases (each spelled out), and
the template-header doc content (given). No git/provider/prompt/decompose knowledge required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions
- docfile: plan/002_a17bb6c8dc1d/P1M4T1S1/research/design-decisions.md
  why: the 8 decisions. §0 (Defaults() MUST change to 0 — makes "missing" reachable; the load-bearing call),
       §1 (fileLoaded guard — no spurious no-file warning), §2 (noticeOut reuse is SAFE — no test asserts its
       content), §3 (EXACTLY 2 assertions break — config_test.go:68 + file_test.go:594, both one-line fixes),
       §4 (configVersionNotice is pure, mirrors repoProviderNotice), §5 (placement at end of Load happy path),
       §6 (the 3 message strings), §7 (template header doc), §8 (no conflicts with parallel/future siblings).
  critical: §0 (the Defaults() edit + WHY — read FIRST), §3 (the 2 breaking assertions — update them or the
       suite goes red), §2 (proves noticeOut reuse won't break the 3 capture-and-ignore tests), §1 (the guard).

# MUST READ — the file the advisory lives in (edit Load + add the helper)
- file: internal/config/load.go   (P1.M3.T2 — READ Load(); EDIT to add configVersionNotice + fileLoaded + emit)
  section: `func Load(ctx, opts) (*Config, error)` — the precedence pipeline. Layer 2 (`if g, err :=
       loadTOML(globalPath); … else if g != nil { overlay(&cfg, g) } else if explicit { … }`) and Layer 3
       (`if r, err := loadRepoLocalConfig(); … else if r != nil { overlay(&cfg, r) }`) are where fileLoaded
       is set. The emit goes right before the final `return &cfg, nil` (after the `cfg.Commits == 1 → Single`
       normalize). load.go already imports "fmt" and reaches the package-level `noticeOut`.
  why: this is THE file to edit. The advisory is a post-overlay side-effect of Load.
  pattern: mirror `repoProviderNotice`'s "pure string-returner + caller does fmt.Fprint(noticeOut, …)" split.
  gotcha: place the emit AFTER every `return nil, err` (only the happy path advises). loadGitConfig (Layer 4)
       does NOT set ConfigVersion (git config has no config_version key) — so fileLoaded is file-only.

# MUST READ — the notice machinery + the pure-function pattern to mirror (edit Defaults via config.go)
- file: internal/config/file.go   (READ noticeOut + repoProviderNotice + materialize/overlay; do NOT edit)
  section: `var noticeOut io.Writer = os.Stderr` + `SetNoticeOut`/`NoticeOut` accessors + `repoProviderNotice(
       cfg *Config) string` (PURE — returns "" or the §19 text; the CALLER loadRepoLocalConfig does
       `fmt.Fprint(noticeOut, msg)`). materialize: `if fc.ConfigVersion != 0 { c.ConfigVersion =
       fc.ConfigVersion }`. overlay: `if src.ConfigVersion != 0 { dst.ConfigVersion = src.ConfigVersion }`.
  why: configVersionNotice MIRRORS repoProviderNotice (pure string, no I/O; caller prints). The non-zero
       guards in materialize/overlay are WHY Defaults must be 0 (else 0 is never observed). noticeOut is the
       sink the task names.
  gotcha: do NOT edit file.go. The non-zero overlay semantics are correct and stay; the fix is Defaults()=0
       (config.go), not changing overlay/materialize.

# MUST READ — the file with Defaults() + the const (EDIT Defaults only)
- file: internal/config/config.go   (P1.M3.T1.S1 — EDIT Defaults(); READ CurrentConfigVersion + the field)
  section: `const CurrentConfigVersion = 2` (KEEP at 2 — the const is the comparison target, unchanged).
       `ConfigVersion int toml:"config_version"` field (KEEP — update its doc comment). `func Defaults() … {
       … ConfigVersion: CurrentConfigVersion … }` (EDIT → remove that line so it stays 0).
  why: Defaults() currently pins ConfigVersion=2, making cfg.ConfigVersion always ≥2 after Load (non-zero
       overlay never lowers it) → the task's "cfg.ConfigVersion == 0 (missing)" is UNREACHABLE. Removing the
       pin makes 0 the genuine "no source set a version" sentinel.
  pattern: every other non-zero-overlay field in Defaults already uses its zero value as "unset" (Timeout is
       the exception — it's 120s; ConfigVersion joins the 0=unset convention like MaxCommits-default-via-12...
       actually MaxCommits is 12; pick the cleaner: ConfigVersion=0=unset is the natural int zero-value).
  gotcha: KEEP CurrentConfigVersion=2. ONLY change the Defaults() return. Update the field doc ("Defaults()
       leaves it 0") + the Defaults() doc ("ConfigVersion is 0; the advisory compares it").

# MUST READ — the 2 assertions that break + the test conventions
- file: internal/config/config_test.go   (EDIT line ~68; the Defaults test)
  section: `TestDefaults` asserts `c.ConfigVersion != CurrentConfigVersion` (line 68-69, expects 2). With
       Defaults()=0 this becomes `0 != 2` → FAIL. UPDATE to expect 0.
  why: the #1 breaking assertion. One-line fix.
  gotcha: KEEP the `CurrentConfigVersion != 2` check (line 71-72) — the const is unchanged at 2.
- file: internal/config/file_test.go   (EDIT line ~594; TestOverlay_V2Scalars part b)
  section: part (b): `dst := Defaults(); src := &Config{Provider: "pi"}; overlay(&dst, src); if
       dst.ConfigVersion != CurrentConfigVersion {…}` (line 594-595, expects 2). With Defaults()=0,
       dst.ConfigVersion stays 0 → FAIL. UPDATE to expect 0. KEEP part (a) (line 577-581: src sets 3 →
       expect 3; unaffected). KEEP line 509-510 (loads a file WITH config_version=2 → expect 2; unaffected).
  why: the #2 breaking assertion. One-line fix.
  pattern: the test's INTENT ("nil src must not clobber dst's value") still holds — dst's value is now 0.
- file: internal/config/load_test.go   (EDIT: ADD the advisory tests; READ the noticeOut capture pattern)
  section: the `noticeOut = &strings.Builder{}; defer restore` idiom (lines 494/515/920) — the EXACT pattern
       the new advisory tests use to capture output. Those existing tests CAPTURE-AND-IGNORE (assert on
       cfg.Provider, not the builder) → they stay green even though the new advisory now also writes to their
       builder. Use the same idiom in the new tests but ASSERT the builder content.
  why: the test pattern to mirror + proof that noticeOut reuse is safe.

# MUST READ — the DOCS target (the config init template)
- file: internal/cmd/config.go   (EDIT exampleConfigTemplate header)
  section: `const exampleConfigTemplate = \`# Stagecoach configuration file …\`` — the inert, fully-commented
       config (the Mode-A user-facing config documentation). Add a commented (`#`) header block documenting
       config_version + the upgrade mechanism. PLACE near the top (after the precedence block).
  why: the task's DOCS (Mode A) requirement: "Update the config init template header to document
       config_version and the upgrade mechanism."
  gotcha: keep EVERY added line commented (`#`) so the file stays INERT — the cmd config_test checks no line
       is an uncommented TOML header (line 172) and uses Contains (additive-safe). P1.M4.T2 (populated init)
       runs later and should carry this note into its output.

- url: (PRD §9.17 FR-B4 + §16.1 — already in context as selected_prd_content `h3.33`; ALSO
       plan/002_a17bb6c8dc1d/prd_snapshot.md §9.17 / §16.1)
  why: FR-B4 is the AUTHORITATIVE advisory contract (missing/older warn + remediation; newer warns; advisory
       only). §16.1 confirms config_version is METADATA, not a precedence layer (it never participates in
       value resolution — only the advisory reads it).
  critical: FR-B4's "advisory only — no automatic migration" (do NOT migrate); FR-B4's remediation wording
       ("stagecoach config upgrade, or config init --force") must appear in the message.
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go            # P1.M3.T1.S1. EDIT: Defaults() ConfigVersion 2→0 (+ doc comments). READ CurrentConfigVersion(=2), field.
  config_test.go       # P1.M3.T1.S1. EDIT: line 68 Defaults().ConfigVersion assertion (expect 0).
  file.go              # P1.M3.T1.S2. READ: noticeOut/SetNoticeOut/NoticeOut, repoProviderNotice (pattern), materialize/overlay (non-zero). DO NOT EDIT.
  file_test.go         # P1.M3.T1.S2. EDIT: line 594 TestOverlay_V2Scalars(b) (expect 0). KEEP (a) + line 509.
  git.go / git_test.go # git-config reader (no config_version key). UNCHANGED.
  load.go              # P1.M3.T2. EDIT: add configVersionNotice + fileLoaded + emit in Load(). READ Load().
  load_test.go         # P1.M3.T2. EDIT: ADD TestConfigVersionNotice + TestLoad_ConfigVersionAdvisory_*. READ noticeOut idiom.
  roles.go / role_defaults.go / *_test.go  # per-role (P1.M3.T2/T3). UNCHANGED.
internal/cmd/
  config.go            # the config init template. EDIT: exampleConfigTemplate header (+ config_version doc).
  config_test.go       # template tests (Contains/len — additive-safe). UNCHANGED (verify still green).
go.mod / go.sum        # UNCHANGED (no new imports: load.go already has "fmt"; tests use testing+strings).
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. 6 MODIFIED files only:
internal/config/config.go        # Defaults() ConfigVersion 2→0; doc comments updated.
internal/config/load.go          # + configVersionNotice (pure) + fileLoaded tracking + emit in Load().
internal/config/config_test.go   # Defaults().ConfigVersion assertion → expect 0.
internal/config/file_test.go     # TestOverlay_V2Scalars (b) → expect 0.
internal/config/load_test.go     # + TestConfigVersionNotice + TestLoad_ConfigVersionAdvisory_{Older,Missing,Current,Newer,NoFile}.
internal/cmd/config.go           # exampleConfigTemplate header + config_version/upgrade doc block.
# go.mod/go.sum UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (Defaults() MUST be 0, design §0): the SHIPPED Defaults() pins ConfigVersion=CurrentConfigVersion
// (2). With non-zero overlay (materialize `if fc.ConfigVersion != 0`; overlay `if src.ConfigVersion != 0`),
// a resolved cfg.ConfigVersion is ALWAYS ≥ 2 → the task's "cfg.ConfigVersion == 0 (missing)" is UNREACHABLE
// and the "missing" branch is dead code. FIX: Defaults() leaves ConfigVersion at 0. Then 0 = "no source
// declared a version". This breaks EXACTLY 2 assertions (config_test.go:68, file_test.go:594) — update both.

// CRITICAL (fileLoaded guard, design §1): with Defaults()=0, the NO-FILE path also yields cfg.ConfigVersion==0.
// Do NOT warn there ("config file has no version" is wrong when there's no file). Gate the advisory on
// fileLoaded (set true when global g!=nil OR repo r!=nil). loadGitConfig sets NO config_version (git has no
// such key) → fileLoaded is file-only. Two lines in Load().

// GOTCHA (noticeOut reuse is SAFE, design §2): the 3 noticeOut-capturing tests (load_test.go:494/515/920)
// redirect to a strings.Builder to SUPPRESS output and then assert cfg.Provider — they NEVER read the
// builder. So the new advisory now ALSO writing to their builder (their repo files lack config_version →
// "missing" fires) does NOT break them. Reuse noticeOut as the task requires. No test asserts noticeOut is
// empty or exact-matches it.

// GOTCHA (place emit on the HAPPY path only): the fmt.Fprint(noticeOut, …) goes right BEFORE `return &cfg,
// nil`, AFTER every `return nil, err`. A hard load error (bad file, bad env) returns before the advisory —
// correct (we're failing, not advising). The Commits==1→Single normalize just above it doesn't touch
// ConfigVersion (ordering immaterial).

// GOTCHA (config_version is METADATA, not a precedence layer — §16.1): env (loadEnv) and flags (loadFlags)
// do NOT set ConfigVersion (verified — no STAGECOACH_CONFIG_VERSION, no --config-version). So after Layer 4
// (git, also no config_version), cfg.ConfigVersion is final; the end-of-Load placement is correct.

// GOTCHA (keep CurrentConfigVersion=2): edit ONLY the Defaults() return value, not the const. The const is
// the comparison target. config_test.go:71-72 asserts it's 2 — keep it.

// GOTCHA (template edits stay COMMENTED): every line added to exampleConfigTemplate must start with `#` so
// the file stays INERT. cmd/config_test.go:172 fails on any uncommented TOML header; the Contains checks
// (line 178/184/187) are additive-safe. P1.M4.T2 (populated init) later carries this note into its output.

// GOTCHA (no parallel/future conflict, design §8): the running P1.M3.T3.S1 creates role_defaults.go +
// edits docs/providers.md — no overlap with these 6 files. Future P1.M4.T2/T3/T4 run AFTER (sequential).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/load.go — ADD a pure helper (mirrors repoProviderNotice in file.go). NO new types.

// configVersionNotice returns the PRD §9.17 FR-B4 advisory text when a loaded config file's schema version
// is missing (0), older, or newer than CurrentConfigVersion; "" when no file was loaded (fileLoaded=false)
// or the version is current. PURE (no I/O) so it is unit-testable; the caller (Load) writes the result to
// noticeOut. config_version is metadata, not a precedence layer (PRD §16.1) — this is its only consumer.
//
// fileLoaded: whether ANY config file (global or repo-local) was read this Load. version: the RESOLVED
// cfg.ConfigVersion (highest non-zero layer; 0 if no layer declared one — Defaults leaves it 0).
func configVersionNotice(fileLoaded bool, version int) string {
	if !fileLoaded {
		return "" // no file → nothing to be stale
	}
	switch {
	case version == CurrentConfigVersion:
		return ""
	case version == 0:
		return fmt.Sprintf("stagecoach: config file has no config_version; current is %d. "+
			"Run 'stagecoach config upgrade' or 'stagecoach config init --force'.\n", CurrentConfigVersion)
	case version < CurrentConfigVersion:
		return fmt.Sprintf("stagecoach: config file uses schema version %d; current is %d. "+
			"Run 'stagecoach config upgrade' or 'stagecoach config init --force'.\n", version, CurrentConfigVersion)
	default: // version > CurrentConfigVersion
		return fmt.Sprintf("stagecoach: config file uses schema version %d; this binary supports up to %d. "+
			"Upgrade stagecoach, or run 'stagecoach config init --force' to regenerate.\n", version, CurrentConfigVersion)
	}
}
```

```go
// internal/config/config.go — EDIT Defaults() (the single source edit). CurrentConfigVersion + field UNCHANGED.

func Defaults() Config {
	return Config{
		// …(all existing fields unchanged)…
		Roles:               nil,
		ConfigVersion:       0, // UNSET sentinel — the load-time advisory (P1.M4.T1.S1) compares the resolved
		//                              value to CurrentConfigVersion; 0 ⇒ no source declared a schema version.
	}
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/config.go — Defaults() ConfigVersion → 0 (do this FIRST; it's the foundation)
  - FILE: internal/config/config.go. CHANGE the `ConfigVersion: CurrentConfigVersion,` line in Defaults() to
      `ConfigVersion:       0,` (or just remove the line so it stays at the zero value; explicit `0` is clearer).
  - UPDATE the `Config.ConfigVersion` field doc comment: replace "Defaults() pins it to CurrentConfigVersion."
      with "Defaults() leaves it 0 (unset); the load-time advisory (P1.M4.T1.S1, load.go) compares the
      resolved value to CurrentConfigVersion. 0 ⇒ no source declared a schema version."
  - UPDATE the Defaults() func doc comment: replace "ConfigVersion is pinned to CurrentConfigVersion (a
      Defaults() Config is always current-schema)." with "ConfigVersion is 0 (unset; a Defaults() Config has
      no schema version until a file declares one)."
  - KEEP `const CurrentConfigVersion = 2` UNCHANGED (it's the comparison target). KEEP its doc comment
      (it already describes the advisory correctly).
  - GOTCHA: edit ONLY Defaults() + the 2 doc comments. Do NOT touch the field declaration, the const, or
      anything else in config.go.

Task 2: EDIT internal/config/config_test.go — update the Defaults().ConfigVersion assertion
  - FILE: internal/config/config_test.go, TestDefaults (~line 68).
  - CHANGE:
        if c.ConfigVersion != CurrentConfigVersion {
            t.Errorf("ConfigVersion = %d, want CurrentConfigVersion (%d)", c.ConfigVersion, CurrentConfigVersion)
        }
    TO:
        if c.ConfigVersion != 0 {
            t.Errorf("ConfigVersion = %d, want 0 (Defaults leaves it unset; the advisory compares it)", c.ConfigVersion)
        }
  - KEEP the `if CurrentConfigVersion != 2` check (line 71-72) — the const is still 2.
  - GOTCHA: this is one of exactly 2 breaking assertions (design §3).

Task 3: EDIT internal/config/file_test.go — update TestOverlay_V2Scalars part (b)
  - FILE: internal/config/file_test.go, TestOverlay_V2Scalars part (b) (~line 592-595).
  - CHANGE:
        if dst.ConfigVersion != CurrentConfigVersion {
            t.Errorf("ConfigVersion=%d want CurrentConfigVersion (nil src must not clobber)", dst.ConfigVersion)
        }
    TO:
        if dst.ConfigVersion != 0 {
            t.Errorf("ConfigVersion=%d want 0 (Defaults pins 0; nil src must not clobber)", dst.ConfigVersion)
        }
  - KEEP part (a) (line 577-581: src sets 3 → expect 3 — UNAFFECTED). KEEP the comment "ConfigVersion=2" on
      line 577 → update to "ConfigVersion=0" for accuracy. KEEP line 509-510 (loads file WITH version=2 →
      expect 2 — UNAFFECTED).
  - GOTCHA: this is the 2nd of exactly 2 breaking assertions.

Task 4: EDIT internal/config/load.go — add configVersionNotice + fileLoaded + emit
  - FILE: internal/config/load.go. PACKAGE unchanged (`package config`).
  - ADD the pure `func configVersionNotice(fileLoaded bool, version int) string` (see "Data models" — paste
      verbatim). Place it near Load() (e.g. after Load, before loadEnv). load.go already imports "fmt".
  - EDIT Load(): add `var fileLoaded bool` right before the Layer 2 block. In the Layer 2 `else if g != nil`
      branch add `fileLoaded = true`; in the Layer 3 `else if r != nil` branch add `fileLoaded = true`.
  - EDIT Load(): immediately before the final `return &cfg, nil` (after the `if cfg.Commits == 1` normalize),
      add:
        if msg := configVersionNotice(fileLoaded, cfg.ConfigVersion); msg != "" {
            fmt.Fprint(noticeOut, msg)
        }
  - GOTCHA: emit AFTER every `return nil, err` (happy path only). loadGitConfig sets no config_version →
      fileLoaded is correctly file-only. Do NOT add config_version handling to loadEnv/loadFlags (it's
      metadata, not env/flag-resolvable).

Task 5: EDIT internal/config/load_test.go — add the advisory tests
  - FILE: internal/config/load_test.go. PACKAGE `config`. ADD:
  - TestConfigVersionNotice (PURE — no Load, no I/O; table over the 5 branches):
      * configVersionNotice(false, 0) == ""            (no file)
      * configVersionNotice(false, 2) == ""            (no file, even if version looks current)
      * configVersionNotice(true, CurrentConfigVersion) == ""   (current → silent)
      * configVersionNotice(true, 0) → CONTAINS "has no config_version" AND "config upgrade" AND "config init --force"
      * configVersionNotice(true, 1) → CONTAINS "schema version 1" AND "current is 2" AND "config upgrade"
      * configVersionNotice(true, 3) → CONTAINS "schema version 3" AND "supports up to 2"
  - TestLoad_ConfigVersionAdvisory_Older: loadEnvSetup + chdir(repo); writeConfigFile(globalDir,
      "config.toml", "config_version = 1\n"); capture noticeOut (orig:=noticeOut; noticeOut=&buf; defer
      restore); Load(ctx, LoadOpts{RepoDir: repo}). Assert buf.String() CONTAINS the older advisory
      ("schema version 1"; "current is 2"; "config upgrade"). Assert cfg.ConfigVersion == 1.
  - TestLoad_ConfigVersionAdvisory_Missing: same, but the file has NO config_version (e.g.
      "[defaults]\nprovider = \"pi\"\n"). Assert buf CONTAINS "has no config_version". Assert
      cfg.ConfigVersion == 0.
  - TestLoad_ConfigVersionAdvisory_Current: file with "config_version = 2\n". Assert buf.String() == ""
      (silent). Assert cfg.ConfigVersion == 2.
  - TestLoad_ConfigVersionAdvisory_Newer: file with "config_version = 3\n". Assert buf CONTAINS
      "schema version 3" AND "supports up to 2".
  - TestLoad_ConfigVersionAdvisory_NoFile: loadEnvSetup + chdir(repo); NO config file written (global dir
      empty, no repo file). Capture noticeOut. Load. Assert buf.String() == "" (fileLoaded guard — no
      spurious warning). Assert cfg.ConfigVersion == 0.
  - GOTCHA: each test MUST capture+restore noticeOut (the idiom at load_test.go:494). The "NoFile" case is
      the fileLoaded-guard regression test (load-bearing). Use `strings.Contains` (not exact-equal) so the
      message wording has latitude, but assert the key phrases + remediation are present.

Task 6: EDIT internal/cmd/config.go — document config_version in the exampleConfigTemplate header
  - FILE: internal/cmd/config.go, `const exampleConfigTemplate`. ADD a commented (`#`) header block (PLACE
      after the existing "Resolution precedence" block, near the top) documenting:
      * config_version is a TOP-LEVEL metadata key (NOT under [defaults], NOT a precedence layer — §16.1).
      * The binary's CurrentConfigVersion; the file should match it.
      * On load, a missing/older/newer config_version prints an advisory pointing at `stagecoach config
        upgrade` / `config init --force` (advisory only — no auto-migration).
      * Example: `# config_version = 2   # schema version; matches ` + "`stagecoach config upgrade`" + ``.
  - CONTENT (paste, keep every line commented `#`):
        # ---------------------------------------------------------------------------
        # config_version — schema version (PRD §9.17 FR-B4). Top-level metadata, NOT a [defaults] key and
        # NOT a precedence layer (§16.1): it never overrides another field; it only tells stagecoach which
        # schema the file was written for. This binary supports config_version = 2.
        # ---------------------------------------------------------------------------
        # config_version = 2
        #
        # On load, if this is missing/older than the binary's version, stagecoach prints an advisory and
        # points you at the remediation; it NEVER auto-migrates your file (no behavior change, just a
        # warning on stderr):
        #   stagecoach config upgrade      # rewrite this file in place to the current schema (P1.M4.T3)
        #   stagecoach config init --force # regenerate the bootstrap config, overwriting this file
  - GOTCHA: keep every added line commented `#` (the file stays INERT). Do NOT uncomment any existing line.
      cmd/config_test.go (Contains + the uncommented-header check) stays green. P1.M4.T2 (populated init)
      should carry this note into its output — flag in a code comment if natural.

Task 7: VERIFY (run all gates; fix before declaring done)
  - `go build ./... && go vet ./...` clean.
  - `go test ./internal/config/ -v` → all PASS, incl. the new advisory tests + the 2 updated assertions.
  - `go test ./...` → GREEN (no regression; cmd/config_test still green — template additive).
  - `gofmt -l internal/` empty.
  - `git diff --exit-code go.mod go.sum` → empty (no new deps).
  - `git status` shows ONLY the 6 modified files (config.go, load.go, config_test.go, file_test.go,
      load_test.go, internal/cmd/config.go); NOTHING else.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the pure-notice-function + caller-prints split (mirror repoProviderNotice in file.go).
//   repoProviderNotice(cfg *Config) string  → returns "" or the §19 text; loadRepoLocalConfig does
//                                              fmt.Fprint(noticeOut, repoProviderNotice(cfg)).
//   configVersionNotice(fileLoaded bool, version int) string  → returns "" or the FR-B4 text; Load does
//                                              fmt.Fprint(noticeOut, configVersionNotice(fileLoaded, cfg.ConfigVersion)).
//   Pure ⇒ unit-testable without I/O or a Config.

// PATTERN: the noticeOut capture idiom in tests (load_test.go:494):
//   origNoticeOut := noticeOut
//   var buf strings.Builder
//   noticeOut = &buf
//   defer func() { noticeOut = origNoticeOut }()
//   …Load…
//   check buf.String()

// CRITICAL: the fileLoaded guard prevents the no-file false positive.
//   With Defaults()=0, "no config file" ALSO yields cfg.ConfigVersion==0. Without the guard, Load would
//   print "config file has no config_version" when there is NO file — wrong. The guard (fileLoaded) is
//   what distinguishes "no file" from "file without a version". loadGitConfig never sets config_version,
//   so fileLoaded is correctly tracked from the FILE layers only (global g!=nil || repo r!=nil).

// CRITICAL: Defaults()=0 is what makes version==0 ("missing") REACHABLE.
//   materialize: `if fc.ConfigVersion != 0 { c.ConfigVersion = fc.ConfigVersion }` (non-zero copy)
//   overlay:     `if src.ConfigVersion != 0 { dst.ConfigVersion = src.ConfigVersion }` (non-zero merge)
//   With Defaults()=2, these never lower the value below 2 → version==0 unreachable. Defaults()=0 + these
//   guards ⇒ version==0 exactly when no source set it. Do NOT change materialize/overlay (they're correct).

// GOTCHA: emit on the happy path ONLY (before `return &cfg, nil`, after every `return nil, err`).
//   A hard load error discards cfg — don't advise about a config we're failing to return.
```

### Integration Points

```yaml
CONFIG.SENTINEL:
  - change: "internal/config/config.go Defaults() — ConfigVersion: CurrentConfigVersion → 0"
  - rationale: "makes 0 a genuine 'no source declared a version' sentinel; required for FR-B4 'missing' detection"
  - breaking-tests: "config_test.go:68 (Defaults assertion) + file_test.go:594 (overlay nil-src) — both updated"

NOTICE.SINK (reuse, no new var):
  - sink: "internal/config/file.go var noticeOut io.Writer = os.Stderr (with SetNoticeOut/NoticeOut)"
  - pattern: "configVersionNotice (pure) → Load does fmt.Fprint(noticeOut, …)"
  - safe-because: "no existing test asserts noticeOut content (the 3 capture-and-ignore tests stay green)"

LOAD.WIRING:
  - add: "var fileLoaded bool (set true in global g!=nil + repo r!=nil branches); emit before return &cfg,nil"
  - gotcha: "loadGitConfig + loadEnv + loadFlags never set ConfigVersion (metadata, not precedence)"

DOCS (Mode A):
  - change: "internal/cmd/config.go exampleConfigTemplate header — + config_version/upgrade doc block (commented)"
  - forward: "P1.M4.T2 (populated config init) should carry this config_version note into its bootstrap output"

GO.MODULE: change NONE. load.go already imports "fmt"; tests use "testing"+"strings". `go mod tidy` is a no-op.

FROZEN/LEAVE (do NOT edit):
  - internal/config/file.go (noticeOut/materialize/overlay/repoProviderNotice — READ-only).
  - internal/config/git.go, roles.go, role_defaults.go (+tests) — UNCHANGED.
  - internal/provider/*, internal/git/*, internal/prompt/*, internal/generate/*, internal/ui/* — UNCHANGED.
  - go.mod, go.sum, Makefile, PRD.md, providers/*.toml, pkg/* — UNCHANGED.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/config/config.go internal/config/load.go internal/config/config_test.go \
  internal/config/file_test.go internal/config/load_test.go internal/cmd/config.go
go vet ./internal/config/ ./internal/cmd/
# Confirm the Defaults() edit (ConfigVersion no longer pinned to the const):
grep -n "ConfigVersion:" internal/config/config.go   # expect "ConfigVersion:       0," in Defaults(), NOT CurrentConfigVersion
# Confirm the helper + emit exist:
grep -n "func configVersionNotice\|fileLoaded\|configVersionNotice(fileLoaded" internal/config/load.go
# Confirm go.mod/go.sum unchanged:
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; Defaults() shows ConfigVersion: 0; configVersionNotice + fileLoaded + emit present.
```

### Level 2: Config-package unit tests (the new suite + the 2 updated assertions)

```bash
# The pure helper — all 5 branches:
go test ./internal/config/ -run TestConfigVersionNotice -v
# Expected PASS: no-file→"", current→"", missing→contains "has no config_version"+"config upgrade",
#                older→contains "schema version 1"+"current is 2", newer→contains "schema version 3"+"supports up to 2".

# The Load integration — all 5 scenarios (capture noticeOut):
go test ./internal/config/ -run TestLoad_ConfigVersionAdvisory -v
# Expected PASS: Older/Missing/Current/Newer/NoFile. NoFile MUST be "" (the fileLoaded guard).

# The 2 UPDATED assertions + full config suite (no regression):
go test ./internal/config/ -v
# Expected: all PASS. If TestDefaults or TestOverlay_V2Scalars fails on ConfigVersion, the Defaults()=0
# edit's 2 assertion updates were missed (Task 2/3) — apply them. If a noticeOut-capture test
# (TestLoad_RepoFileOverridesGlobal / _GitOverridesRepoFile / _FullPrecedenceMatrix) fails, it's NOT this
# subtask (they ignore the builder) — re-check; they should be unaffected (design §2).
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...     # Expect clean.
go test ./...      # Expect all PASS — incl. internal/cmd/config_test.go (template additive; Contains-safe).
# Confirm ONLY the 6 target files differ (nothing else):
git status --porcelain
# Expected: exactly 6 modified files:
#   internal/config/config.go, internal/config/load.go, internal/config/config_test.go,
#   internal/config/file_test.go, internal/config/load_test.go, internal/cmd/config.go
# Confirm the LEAVE files are byte-unchanged:
git diff --exit-code internal/config/file.go internal/config/git.go internal/config/roles.go \
  internal/config/role_defaults.go internal/provider internal/git internal/prompt internal/generate \
  internal/ui pkg cmd/stagecoach Makefile go.mod go.sum PRD.md providers \
  && echo "frozen files UNCHANGED (expected)"
```

### Level 4: Correctness reasoning (advisory-only, no migration; the version matrix)

```bash
# No server/DB/subprocess. Verify by reasoning + the Level-2 tests:
#   1. Advisory ONLY: grep that Load() never WRITES a config file (no os.WriteFile in load.go) —
#      FR-B4 "advisory only — no automatic migration".
grep -n "os.WriteFile\|WriteFile" internal/config/load.go   # expect no matches (advisory only)
#   2. The version matrix (design §0 table) is fully covered by TestConfigVersionNotice + the 5 Load tests.
#   3. config_version is metadata: grep that loadEnv/loadFlags do NOT read a config_version env/flag
#      (it's file-only metadata, §16.1).
grep -n "CONFIG_VERSION\|config-version\|ConfigVersion" internal/config/load.go   # expect ONLY the emit + helper, NOT in loadEnv/loadFlags
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l internal/` clean.
- [ ] `go test ./...` PASS (config suite incl. the new advisory tests + the 2 updated assertions; cmd
      config_test still green; no repo-wide regression).
- [ ] go.mod/go.sum byte-unchanged (`git diff --exit-code go.mod go.sum` empty).
- [ ] `git status` shows EXACTLY the 6 modified files; every LEAVE file byte-unchanged.

### Feature Validation
- [ ] `Defaults().ConfigVersion == 0` (the sentinel change); `CurrentConfigVersion == 2` (const unchanged).
- [ ] `configVersionNotice(false, _) == ""`; `(true, CurrentConfigVersion) == ""`; `(true, 0)` = "missing";
      `(true, 1)` = "older" (task wording); `(true, 3)` = "ahead".
- [ ] A loaded file with `config_version = 2` → silent; `= 1` → older advisory; `= 3` → ahead advisory; NO
      config_version key → missing advisory; NO file at all → silent (fileLoaded guard).
- [ ] Every advisory names `config upgrade` and/or `config init --force` (the FR-B4 remediation).
- [ ] Advisory is read-only — Load writes no config file (FR-B4 "advisory only — no automatic migration").
- [ ] `exampleConfigTemplate` header documents config_version (top-level metadata) + CurrentConfigVersion +
      the missing/older/ahead advisory + the upgrade remediation; all added lines commented `#`.

### Code Quality Validation
- [ ] `configVersionNotice` mirrors `repoProviderNotice` (pure string-returner; caller prints) — no new
      pattern invented.
- [ ] The 2 assertion updates preserve the original tests' INTENT (Defaults/nil-src-no-clobber) under the
      new sentinel.
- [ ] No edits to file.go's materialize/overlay (the non-zero guards are correct; the fix is Defaults()=0).
- [ ] Anti-patterns avoided (see below); no out-of-scope churn; no new dependency.

### Documentation
- [ ] config.go: the ConfigVersion field + Defaults() doc comments reflect the 0=unset sentinel + the
      advisory as the consumer.
- [ ] exampleConfigTemplate header documents config_version + the upgrade mechanism (Mode A).
- [ ] No new env vars (config_version is file-only metadata, not env/flag-resolvable).

---

## Anti-Patterns to Avoid

- ❌ **Don't leave `Defaults()` pinning ConfigVersion=2.** That makes `cfg.ConfigVersion == 0` unreachable
  (non-zero overlay never lowers it) → the task's "missing" branch is dead code and FR-B4's "missing" case
  silently passes as current. Change Defaults() to 0 (design §0).
- ❌ **Don't forget the `fileLoaded` guard.** With Defaults()=0, the no-file path ALSO yields version==0;
  without the guard it would print a spurious "config file has no config_version" when there's no file.
  Gate on fileLoaded (design §1).
- ❌ **Don't skip the 2 assertion updates.** config_test.go:68 and file_test.go:594 both break under
  Defaults()=0; update both or the suite goes red (design §3).
- ❌ **Don't invent a new notice sink.** Reuse the existing `noticeOut` (the task's "use the existing
  noticeOut pattern"). It's verified-safe (design §2). Don't add a parallel writer var.
- ❌ **Don't change `materialize()`/`overlay()` to fix "missing".** The non-zero guards are correct; the fix
  is purely Defaults()=0. Editing the overlay mechanics risks the field-merge invariants other tests pin.
- ❌ **Don't emit the advisory on an error path.** Place the `fmt.Fprint` right before `return &cfg, nil`
  (happy path only). A failing load returns before it — don't advise about a discarded config.
- ❌ **Don't auto-migrate.** FR-B4 is explicit: "advisory only — no automatic migration." Load must NOT
  rewrite the user's config file. `config upgrade` (P1.M4.T3) owns migration.
- ❌ **Don't add config_version to loadEnv/loadFlags.** It's metadata, not a precedence layer (§16.1); env
  and flags never set it. Only files declare it.
- ❌ **Don't uncomment template lines or add uncommented ones.** Every `exampleConfigTemplate` addition must
  start with `#` (keep the file inert; the cmd config_test checks for uncommented TOML headers).
- ❌ **Don't touch the parallel P1.M3.T3.S1's files** (role_defaults.go, docs/providers.md) or future
  P1.M4.T2/T3/T4 work — this subtask WARNS only (design §8).
