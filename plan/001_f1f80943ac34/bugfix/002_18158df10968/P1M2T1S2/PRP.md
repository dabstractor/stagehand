# PRP — P1.M2.T1.S2: Fix the `buildDeps` bridge so `[generation]` output/strip_code_fence override the manifest only when explicitly set

> **Scope discipline.** This subtask delivers the **user-facing behavior** for PRD bugfix-002 Issue 2
> (manifest-level `output`/`strip_code_fence` silently clobbered by the `[generation]` defaults). The
> **internal config-package plumbing** (making `Config.Output` a tri-state `*string` and stopping
> `Defaults()` from seeding `Output`/`StripCodeFence`) was **S1** (`P1.M2.T1.S1`, already merged — see
> `internal/config/config.go:37` `Output *string` and `Defaults()` returning `nil` for both). S2 owns:
> the **two-line `buildDeps` edit** that turns the S1 types into the opt-in-override behavior, the
> **end-to-end regression tests**, one **test-comment/compile fix**, and the **Mode-A docs**. Do NOT
> edit `internal/config/*` (S1's scope) or `internal/provider/*`.

---

## Goal

**Feature Goal**: When `[generation]` (or git-config) does **not** set `output` / `strip_code_fence`,
`pkg/stagecoach.buildDeps` must leave the resolved provider manifest's own `output` / `strip_code_fence`
**intact** — so a `[provider.X] output = "json"` (+ `json_field`) is honored by `provider.ParseOutput`
without the user repeating it under `[generation]`, a `[provider.X] strip_code_fence = false` is honored,
and `stagecoach providers show <name>` stops lying about what parsing will do. Only an **explicit**
`[generation]` / git-config value overrides the manifest (the genuine opt-in override).

**Deliverable**:
1. `pkg/stagecoach/stagecoach.go` `buildDeps`: change the `Output` guard from `if cfg.Output != "" { o := cfg.Output; m.Output = &o }` to `if cfg.Output != nil { m.Output = cfg.Output }` (the `StripCodeFence` guard is already `!= nil` and stays); refresh the now-stale "broader layer wins / decisions.md D4" comment to the opt-in-override wording.
2. `pkg/stagecoach/stagecoach_test.go`: compile-fix + refresh `TestGenerateCommit_ManifestOutputWins_WhenCfgOutputEmpty_Issue4` (`cfg.Output = ""` → `cfg.Output = nil`; "empty"/`!= ""` wording → "nil"/`!= nil`).
3. `pkg/stagecoach/stagecoach_test.go`: add end-to-end tests exercising the **real `config.Load`+registry path** via a repo-local `.stagecoach.toml` (the contract's clauses a/b/d): manifest-level `output=json` honored with **no** `[generation]`; manifest-level `strip_code_fence=false` honored; regression that default `raw` still works. (Clause c — `[generation] output=json` overrides manifest `raw` — is already covered by the existing `TestGenerateCommit_GenerationConfigFile_OutputJSON_Issue4`; verify it stays green.)
4. `docs/configuration.md` + `docs/providers.md`: change the "broader layer wins" wording to "opt-in override; when `[generation]` omits them, the per-provider `[provider.X]` value is honored", and align the defaults-table source column + `[generation]` block comment.

**Success Definition**: `go build ./... && go vet ./... && go test -race ./...` is GREEN; a
`[provider.stub] output="json"` + `json_field` with **no** `[generation]` block produces a parsed
JSON-field commit message (not the raw JSON blob); `stagecoach providers show stub` reports
`output = 'json'` and parsing now matches.

---

## Why

- **PRD bugfix-002 Issue 2 / §12.1 / §12.9 / §12.4 / §17.4**: `output` / `strip_code_fence` are
  **per-manifest** settings; `parseOutput` reads the **manifest's** values. A user who sets
  `output = "json"` (+ `json_field`) on a `[provider.X]` block must get JSON parsing for that provider
  without also repeating it under `[generation]`.
- **Root cause** (full trace: `architecture/issue_analysis.md` ISSUE 2): the bugfix-001 "Issue 4"
  bridge copied `cfg.Output`/`cfg.StripCodeFence` onto the resolved manifest unconditionally. Its guards
  (`cfg.Output != ""` / `cfg.StripCodeFence != nil`) ALWAYS passed because `Defaults()` seeded non-empty
  `"raw"` / non-nil `boolPtr(true)`. So a manifest's own `output="json"` / `strip_code_fence=false` was
  **always overwritten** → manifest-level JSON was dead, `strip_code_fence=false` was ignored, and
  `providers show` printed `output = 'json'` while parsing used `raw`.
- **This subtask's role**: S1 already removed the always-on defaults (so `cfg.Output`/`cfg.StripCodeFence`
  are `nil` unless a file/git-config layer sets them). S2 changes the bridge's `Output` guard to the
  symmetric `!= nil` form so the manifest wins by default. After S2 the manifest's `Resolve()`
  (`internal/provider/manifest.go:121-160`) supplies the §12.1 `raw`/`true` fallback when neither the
  manifest nor `[generation]` sets a value — losing nothing.

---

## What

### The bridge edit

In `pkg/stagecoach/stagecoach.go` `buildDeps` (the block immediately after the `reg.IsInstalled`
pre-flight and before `return generate.Deps{...}`):

- `Output`: `if cfg.Output != "" { o := cfg.Output; m.Output = &o }`  →  `if cfg.Output != nil { m.Output = cfg.Output }`
- `StripCodeFence`: `if cfg.StripCodeFence != nil { m.StripCodeFence = cfg.StripCodeFence }`  →  **unchanged** (already the `!= nil` opt-in form).

Both branches are now symmetric `if cfg.X != nil { m.X = cfg.X }`: **nil ⇒ honor the manifest; non-nil ⇒ override**. The local-copy (`o := cfg.Output`) is dropped — `cfg.Output` is already a `*string`, so the pointer is assigned directly (no aliasing concern: `cfg` is a value-param and the assignment copies the pointer into the manifest struct).

### Success Criteria

- [ ] `buildDeps` uses `if cfg.Output != nil` (not `!= ""`); the local copy is removed; `StripCodeFence` guard unchanged.
- [ ] The stale "broader setting wins / decisions.md D4" comment is replaced with the opt-in-override wording.
- [ ] `TestGenerateCommit_ManifestOutputWins_WhenCfgOutputEmpty_Issue4` compiles (`cfg.Output = nil`) and its comments reflect "nil"/`!= nil".
- [ ] New test (a): `[provider.stub] output="json"` + `json_field`, **no** `[generation]`, stub emits JSON → `res.Message` == the extracted field (NOT the raw blob).
- [ ] New test (b): `[provider.stub] strip_code_fence=false`, stub emits a fenced block → fences preserved in `res.Message`.
- [ ] New test (d): no `[generation]`, manifest default `raw` → plain raw message still round-trips.
- [ ] Existing `TestGenerateCommit_GenerationConfigFile_OutputJSON_Issue4` (clause c) + git-config + injected-SCF tests stay GREEN.
- [ ] `docs/configuration.md` + `docs/providers.md` describe the opt-in override; the defaults-table source column no longer claims `config.Defaults()` for `output`/`strip_code_fence`.
- [ ] `go build ./... && go vet ./... && go test -race ./...` GREEN.

---

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the exact before/after for the bridge edit, the exact existing-test compile
fix, copy-paste-ready new-test skeletons, the doc anchors with current→replacement wording, and the
runnable validation commands are all below.

### Documentation & References

```yaml
# MUST READ — binding architecture (root cause + blast radius, verified SAFE + mechanical)
- file: plan/001_f1f80943ac34/bugfix/002_18158df10968/architecture/issue_analysis.md
  why: ISSUE 2 — proves the unconditional bridge is the root cause; Part 2 is THIS subtask's edit;
       lists the test patterns (setupTestRepo/setupScriptedRepo + the existing Issue-4 tests) and the
       doc anchors. Confirms cfg.Output/cfg.StripCodeFence are consumed ONLY in buildDeps.
  section: "ISSUE 2 ... Fix — Part 2 (the bridge — delivers the behavior)" and "Test patterns (behavior validation)"

# THE S1 prerequisite that makes this safe (READ-ONLY — do not edit)
- file: plan/001_f1f80943ac34/bugfix/002_18158df10968/P1M2T1S1/PRP.md
  why: Documents S1's contract (Config.Output is now *string; Defaults() leaves Output/StripCodeFence nil)
       and explicitly defers the buildDeps bridge to S2 (this task). Confirms the transient whole-repo
       compile break at pkg/stagecoach/stagecoach.go that S2 resolves.
  critical: S1 changed the config-package LOADERS (file.go/git.go) to set *string/*bool only when the
            source provides a value. So after S1, cfg.Output/cfg.StripCodeFence are genuinely nil when
            [generation]/git-config omit them — the precondition for the `!= nil` guard to mean "manifest wins".

# THE EDIT SITE (Task 1)
- file: pkg/stagecoach/stagecoach.go
  why: buildDeps (~line 206-211, the block after reg.IsInstalled + before `return generate.Deps{...}`).
       Currently `if cfg.Output != ""` — this is the TRANSIENT S1 compile-break; S2 changes it to `!= nil`.
  pattern: The StripCodeFence guard IMMEDIATELY BELOW it (`if cfg.StripCodeFence != nil { m.StripCodeFence = cfg.StripCodeFence }`)
           is the EXACT target shape — make Output its twin.
  gotcha: Drop the `o := cfg.Output` local copy. cfg.Output is *string; assign the pointer directly
          (m.Output = cfg.Output). No aliasing hazard (cfg is a value-param; the pointer is copied into m).
  gotcha: The block's doc comment currently says "broader setting wins (decisions.md D4)" and "defensive;
          always non-empty post-Defaults" — both are now FALSE (Defaults no longer sets it). REPLACE the
          comment (see Implementation Tasks).

# THE CONSUMER that proves the behavior (READ-ONLY)
- file: internal/provider/parse.go
  why: ParseOutput (~line 48-58) does `r := m.Resolve(); switch *r.Output { case "json": ...; case "raw": ... }`.
       Whatever m.Output the bridge leaves in place is what ParseOutput uses. Resolve() is nil-pointer-safe.
  critical: This is why NOT touching m.Output (the nil-cfg path) correctly yields manifest-level json/raw.
            json mode REQUIRES m.JsonField to be set (ParseOutput extracts obj[*r.JsonField]); the new
            test (a) MUST include `json_field = "..."` in the manifest TOML or extraction falls back to raw.

# THE FALLBACK that makes removing the config default safe (READ-ONLY)
- file: internal/provider/manifest.go
  why: Resolve() (121-160) fills nil Output → strPtr(DefaultOutput="raw") (line 152) and nil StripCodeFence
       → boolPtr(DefaultStripCodeFence=true) (line 158). So a manifest that sets NEITHER output nor
       strip_code_fence still resolves to raw/true. This is why S1+S2 lose nothing.
  gotcha: Validate() enforces Name/Command only; it does NOT touch Output/StripCodeFence, so a manifest
          with strip_code_fence=false or output=json passes Validate and reaches ParseOutput unchanged.

# PRIMARY TEST PATTERNS — the existing Issue-4 tests to MIRROR (real config.Load+registry path)
- file: pkg/stagecoach/stagecoach_test.go
  why: TestGenerateCommit_GenerationConfigFile_OutputJSON_Issue4 (line 675) is the template for the new
       tests: it writes a CUSTOM .stagecoach.toml string (NOT setupTestRepo, which hardcodes output="raw"
       /strip_code_fence=true), runs GenerateCommit(DryRun:true), asserts res.Message. It exercises the
       REAL config.Load+registry path (resolveConfig→config.Load reads the toml→registry merges it).
  pattern: TempDir + os.WriteFile(.stagecoach.toml, customTOML) + initRepo + commitRaw("initial") +
           writeFile+stageFile(new.txt) + chdir(+t.Cleanup) + GenerateCommit(ctx, Options{...}).
  gotcha: setupTestRepo/setupScriptedRepo (lines 112-180) HARDCODE `output = "raw"` and `strip_code_fence = true`
          in the toml, so they CANNOT produce a json/false manifest. Use the custom-TOML approach (mirror
          test #1) for the new (a)/(b) tests. (d) can reuse setupTestRepo as-is (raw manifest default).

# THE TEST TO COMPILE-FIX (Task 2)
- file: pkg/stagecoach/stagecoach_test.go
  why: TestGenerateCommit_ManifestOutputWins_WhenCfgOutputEmpty_Issue4 (line 839) has `cfg.Output = ""`
       (line 846) which no longer compiles (Output is *string). Change to `cfg.Output = nil` and refresh
       the "empty"/`!= ""`/"Defaults() ALWAYS sets Output=\"raw\"" comments to "nil"/`!= nil`/"nil by default".
  gotcha: This test STAYS a valid injected-config guard; the contract calls this update "cosmetic". The
          new test (a) additionally covers the REAL-loader nil path ("the natural nil path").

# DOCS (Mode A) — exact anchors
- file: docs/configuration.md
  why: line 84 "These [generation] values override any per-provider defaults — the broader layer wins" →
       opt-in-override wording. Lines 79-80 (Built-in defaults table) list `output`/`strip_code_fence`
       Source as `config.Defaults()` — now FALSE (S1 removed them); the effective raw/true default comes
       from the manifest's Resolve(). Lines 55-56 ([generation] block comment) `# output = "raw"` /
       `# strip_code_fence = true` — clarify these are opt-in (uncomment to OVERRIDE; leave unset to honor
       the per-provider manifest value).
- file: docs/providers.md
  why: line 124 "A [generation] output / strip_code_fence value ... overrides these per-provider manifest
       defaults — the broader layer wins (see configuration.md)." → opt-in-override wording. Lines 30-32
       (schema rows) are CORRECT (manifest defaults raw/true) but should carry the same opt-in note so
       providers show and parsing semantics agree.
```

### Current Codebase tree (relevant slice)

```bash
pkg/stagecoach/stagecoach.go        # buildDeps (~L206-211) — THE EDIT (Output guard) + comment refresh
pkg/stagecoach/stagecoach_test.go   # compile-fix test #4 (L839) + ADD new tests (a)/(b)/(d)
internal/config/config.go         # S1 DONE: Output *string, Defaults() nil — DO NOT TOUCH
internal/config/file.go           # S1 DONE: materialize/overlay pointer-ified — DO NOT TOUCH
internal/config/git.go            # S1 DONE: c.Output = &v — DO NOT TOUCH
internal/provider/parse.go        # ParseOutput (READ-ONLY consumer of m.Output/m.StripCodeFence)
internal/provider/manifest.go     # Resolve() raw/true fallback (READ-ONLY)
docs/configuration.md             # Mode A: lines 55-56, 79-80, 84
docs/providers.md                 # Mode A: lines 30-32, 124
```

### Desired Codebase tree (files MODIFIED; no new files)

```bash
pkg/stagecoach/stagecoach.go        # buildDeps Output guard -> != nil (drop local copy) + comment
pkg/stagecoach/stagecoach_test.go   # +3 test funcs (a/b/d), 1 compile-fix (test #4)
docs/configuration.md             # opt-in-override wording + defaults-table source column
docs/providers.md                 # opt-in-override wording + schema-row note
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the whole fix): the guard must be `if cfg.Output != nil` (NOT `!= ""`). After S1, cfg.Output
//   is *string and nil unless [generation]/git-config set it. A leftover `!= ""` either fails to compile
//   (comparing *string to string) or — if "fixed" wrongly — reintroduces the clobber. Assign the pointer
//   directly: m.Output = cfg.Output (drop the `o := cfg.Output` local).

// CRITICAL (test correctness): the new json test (a) MUST set `json_field` in the manifest TOML.
//   ParseOutput's json branch extracts obj[*r.JsonField]; with JsonField unset it resolves to "" →
//   obj[""] is absent → fallback to RAW (test would observe the raw blob and wrongly "pass" if you
//   asserted on the blob). Assert on the EXTRACTED field value to prove json mode actually ran.

// CRITICAL (test fixture): setupTestRepo/setupScriptedRepo HARDCODE output="raw" / strip_code_fence=true.
//   They CANNOT build a json/false manifest. For (a)/(b) write a custom .stagecoach.toml string (mirror
//   the existing TestGenerateCommit_GenerationConfigFile_OutputJSON_Issue4). (d) may reuse setupTestRepo.

// GOTCHA (stub JSON quoting): TOML literal strings ('...') preserve embedded double-quotes, so emit
//   JSON via a literal string: `STAGECOACH_STUB_OUT = '{"subject":"feat: ..."}'` (exactly as test #1 does).
//   Double-quoted TOML strings would need escaping.

// GOTCHA (chdir): GenerateCommit resolves the repo via os.Getwd(). Every test chdir's into a t.TempDir()
//   and registers t.Cleanup(os.Chdir(wd)) — mirror that exactly (the Issue-4 tests already do).

// GOTCHA (no behavior change for installed/default providers): when neither the manifest nor [generation]
//   sets output/strip_code_fence, the manifest's Resolve() yields raw/true — identical to the pre-fix
//   behavior. So every existing happy-path/raw test stays green; only the json/false manifest cases change.
```

---

## Implementation Blueprint

### The bridge edit (Task 1)

In `pkg/stagecoach/stagecoach.go` `buildDeps`, the current block (after the `reg.IsInstalled` pre-flight,
before `return generate.Deps{Git: git.New(repoDir), Manifest: m}, nil`) is:

```go
	// Apply [generation] output/strip_code_fence onto the resolved manifest (PRD Issue 4 / §16.2 / §12.9).
	// cfg.Output / cfg.StripCodeFence are populated by every loader (file, git-config) and Defaults, but
	// were previously dropped here — ParseOutput reads ONLY the manifest's pointer fields. Copying them
	// onto the manifest makes the [generation] / git-config values override the per-provider per-manifest
	// values (broader setting wins), which ParseOutput then honors. (decisions.md D4.)
	//
	// Copy into locals (not &cfg.*) to avoid aliasing the cfg value-param's address. Output is guarded
	// (defensive; it is always non-empty post-Defaults); StripCodeFence is applied unconditionally so the
	// broader [generation] layer consistently overrides any per-manifest default. No re-Validate():
	// ParseOutput's switch-default degrades an unknown Output to raw.
	if cfg.Output != "" {
		o := cfg.Output
		m.Output = &o
	}
	if cfg.StripCodeFence != nil {
		m.StripCodeFence = cfg.StripCodeFence
	}
```

Replace the WHOLE block (comment + both guards) with:

```go
	// Apply [generation] / git-config output/strip_code_fence onto the resolved manifest ONLY when the
	// user explicitly set them (PRD bugfix-002 Issue 2 / §16.2 / §12.9). After S1, cfg.Output/cfg.StripCodeFence
	// are *string/*bool and nil unless a file or git-config layer provided them. nil ⇒ leave the manifest's
	// own value intact (the per-provider [provider.X] setting wins, or Resolve() supplies the §12.1 raw/true
	// fallback); non-nil ⇒ override the manifest. This makes [generation] a true OPT-IN override and keeps
	// `providers show` truthful (it displays the registry manifest, which parsing now matches). ParseOutput's
	// switch-default degrades an unknown Output to raw, so no re-Validate() is needed.
	if cfg.Output != nil {
		m.Output = cfg.Output
	}
	if cfg.StripCodeFence != nil {
		m.StripCodeFence = cfg.StripCodeFence
	}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY pkg/stagecoach/stagecoach.go :: buildDeps
  - REPLACE: the entire comment + Output/StripCodeFence guard block (code above). The Output guard
    becomes `if cfg.Output != nil { m.Output = cfg.Output }` (drop the `o := cfg.Output` local copy);
    the StripCodeFence guard is UNCHANGED (`!= nil`) — keep it verbatim.
  - REFRESH COMMENT: from "broader setting wins / decisions.md D4 / always non-empty post-Defaults" to
    the opt-in-override wording (provided). Reference PRD bugfix-002 Issue 2 (not bugfix-001 Issue 4).
  - GUARDRAIL: this is the ONLY source edit. Do NOT touch CommitStaged, runPipeline, parse.go,
    manifest.go, config/*, or exitcode.go.
  - DEPENDENCIES: requires S1 (already merged) — cfg.Output is *string. After this edit `go build ./...`
    compiles (the transient S1 break is resolved).

Task 2: MODIFY pkg/stagecoach/stagecoach_test.go :: TestGenerateCommit_ManifestOutputWins_WhenCfgOutputEmpty_Issue4 (~L839)
  - COMPILE FIX: `cfg.Output = ""` (L846) → `cfg.Output = nil`.
  - COMMENT REFRESH: the header (L826-838) and inline comments say "empty"/`!= ""`/"Defaults() ALWAYS
    sets Output=\"raw\"". Update to "nil"/`!= nil`/"nil by default". Adjust the test's stated premise:
    nil is now ALSO the real-loader default (when [generation] is absent), not solely an injected edge
    case — but keep the test as a unit-level guard on the injected-Options.Config path (the new test (a)
    covers the real-loader path). Update the final assertion message wording ("when cfg.Output is empty"
    → "when cfg.Output is nil").
  - OPTIONAL: rename to TestGenerateCommit_ManifestOutputWins_WhenCfgOutputNil_Issue4 for accuracy
    (not required; the contract calls this cosmetic).
  - DO NOT change the assertion logic (it still expects the manifest's json to win) — only the value +
    comments.

Task 3: ADD pkg/stagecoach/stagecoach_test.go :: TestGenerateCommit_ManifestOutputJSON_Honored_NoGeneration (clause a)
  - THE KEYSTONE: proves manifest-level output="json" is honored with NO [generation] block (dead before
    the fix). Follow the TestGenerateCommit_GenerationConfigFile_OutputJSON_Issue4 pattern (custom TOML).
  - TOML: [provider.stub] command=<bin>, prompt_delivery="stdin", output="json", json_field="subject",
    strip_code_fence=true, [provider.stub.env] STAGECOACH_STUB_OUT='{"subject":"feat: manifest json wins"}'.
    NO [generation] block.
  - ASSERT: res.Message == "feat: manifest json wins" (the EXTRACTED field, NOT the raw JSON blob).
  - TDD: without Task 1's fix (or if someone reintroduces an unconditional clobber), this FAILS (raw
    blob '{"subject":...}' observed). See skeleton below.

Task 4: ADD pkg/stagecoach/stagecoach_test.go :: TestGenerateCommit_ManifestStripCodeFenceFalse_Honored_NoGeneration (clause b)
  - Proves manifest-level strip_code_fence=false is honored with NO [generation] block.
  - TOML: [provider.stub] ... output="raw", strip_code_fence=false, [provider.stub.env]
    STAGECOACH_STUB_OUT = a fenced block ("```\nfeat: keep fence\n```").
  - ASSERT: strings.Contains(res.Message, "```") (fence RETAINED). (Mirror the injected-SCF test's assertion.)
  - TDD: without the fix (old default boolPtr(true) clobber), fence is stripped → FAILS.

Task 5: ADD pkg/stagecoach/stagecoach_test.go :: TestGenerateCommit_ManifestDefaultRaw_StillWorks (clause d — regression)
  - Proves the default raw path is UNCHANGED: no [generation], manifest default raw → plain message round-trips.
  - SIMPLEST: reuse setupTestRepo(t, stubtest.Options{Out: "feat: default raw ok"}) (its toml has output="raw",
    strip_code_fence=true, no [generation]) — stage a file, GenerateCommit(DryRun:true), assert
    res.Message == "feat: default raw ok". (This reuses the existing helper, so it's ~10 lines.)
  - GUARD: this must stay green both before and after the fix (raw/true is the unchanged default).

Task 6: MODIFY docs/configuration.md (Mode A)
  - LINE 84: replace "These `[generation]` values override any per-provider `[provider.<name>]` defaults
    — the broader layer wins." with opt-in-override wording, e.g.:
    "These `[generation]` values are an **opt-in override**: when `[generation]` (and git-config) omit
    them, the per-provider `[provider.<name>]` value is honored, falling back to the §12.1 manifest
    defaults (`output = "raw"`, `strip_code_fence = true`). Set `output = "json"` here only to force JSON
    parsing across ALL providers."
  - LINES 79-80 (Built-in defaults table): the `output`/`strip_code_fence` rows list Source as
    `config.Defaults()` — now FALSE. Change the Source column to `provider manifest (§12.1)` for those
    TWO rows (the effective default raw/true comes from the manifest's Resolve(), not config.Defaults()).
    Keep the Default column (`"raw"` / `true`) — those are still the effective values.
  - LINES 55-56 ([generation] block comment): `# output = "raw"` / `# strip_code_fence = true` — add a
    one-line note that these are OPT-IN (uncomment to override the per-provider manifest value; leave them
    commented/unset to honor the manifest). Keep them as illustrative examples.

Task 7: MODIFY docs/providers.md (Mode A)
  - LINE 124: replace "A `[generation] output` / `strip_code_fence` value in the config file or git-config
    overrides these per-provider manifest defaults — the broader layer wins (see configuration.md)." with
    opt-in-override wording, e.g.:
    "A `[generation] output` / `strip_code_fence` value in the config file or git-config is an **opt-in
    override**: when unset, the per-provider manifest value above is what `parseOutput` uses (so
    `providers show` and parsing agree). Set it only to force a value across all providers (see
    configuration.md)."
  - LINES 30-32 (schema rows): the manifest defaults (`"raw"`/`""`/`true`) are CORRECT — keep them. Add a
    short note under the schema (or extend the line-124 paragraph's reach) that these per-provider values
    are the parse-time source of truth unless `[generation]` overrides them. (One sentence; do not bloat.)
```

### Test skeletons (copy-paste-ready)

**Task 3 — clause (a), the keystone:**

```go
// TestGenerateCommit_ManifestOutputJSON_Honored_NoGeneration proves PRD bugfix-002 Issue 2: a
// [provider.stub] output="json" (+ json_field) is honored by ParseOutput with NO [generation] block.
// Before the S2 bridge fix, config.Defaults() seeded Output="raw" and buildDeps's `if cfg.Output != ""`
// guard ALWAYS passed, clobbering the manifest's "json" — so the raw JSON blob was returned verbatim.
// After S1 (Output is *string, nil default) + S2 (`if cfg.Output != nil`), the manifest's "json" wins.
//
// TDD check (manual, do not commit): revert buildDeps to `if cfg.Output != "" { o := "raw"; m.Output = &o }`
// (or any unconditional clobber) and re-run — this test FAILS (raw blob observed).
func TestGenerateCommit_ManifestOutputJSON_Honored_NoGeneration(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	jsonOut := `{"subject":"feat: manifest json wins"}`

	// Manifest sets output="json" + json_field. NO [generation] block — the manifest value must win.
	toml := "[provider.stub]\n" +
		"command = \"" + bin + "\"\n" +
		"prompt_delivery = \"stdin\"\n" +
		"output = \"json\"\n" + // manifest-level — must be honored with no [generation]
		"json_field = \"subject\"\n" + // REQUIRED: parseJSON extracts obj["subject"]
		"strip_code_fence = true\n" +
		"\n[provider.stub.env]\n" +
		"STAGECOACH_STUB_OUT = '" + jsonOut + "'\n" // literal string preserves the JSON quotes
	if err := os.WriteFile(repo+"/.stagecoach.toml", []byte(toml), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "data")
	stageFile(t, repo, "new.txt")

	wd, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty (DryRun)", res.CommitSHA)
	}
	// The manifest's output="json" was honored (no [generation] override) ⇒ the JSON field extracted.
	if res.Message != "feat: manifest json wins" {
		t.Errorf("Message = %q, want %q (manifest output=json must be honored with no [generation] block)",
			res.Message, "feat: manifest json wins")
	}
}
```

**Task 4 — clause (b):**

```go
// TestGenerateCommit_ManifestStripCodeFenceFalse_Honored_NoGeneration proves PRD bugfix-002 Issue 2:
// a [provider.stub] strip_code_fence=false is honored by ParseOutput with NO [generation] block (the
// ``` fences are RETAINED). Before the fix, config.Defaults() seeded StripCodeFence=boolPtr(true) and
// the bridge's `!= nil` guard always passed, clobbering the manifest's false.
func TestGenerateCommit_ManifestStripCodeFenceFalse_Honored_NoGeneration(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	stubOut := "```\n" + "feat: keep the fence" + "\n```" // fenced block

	toml := "[provider.stub]\n" +
		"command = \"" + bin + "\"\n" +
		"prompt_delivery = \"stdin\"\n" +
		"output = \"raw\"\n" +
		"strip_code_fence = false\n" + // manifest-level false — must be honored with no [generation]
		"\n[provider.stub.env]\n" +
		"STAGECOACH_STUB_OUT = '" + stubOut + "'\n"
	if err := os.WriteFile(repo+"/.stagecoach.toml", []byte(toml), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "data")
	stageFile(t, repo, "new.txt")

	wd, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}
	// strip_code_fence=false honored (no [generation] override) ⇒ the ``` fence is RETAINED.
	if !strings.Contains(res.Message, "```") {
		t.Errorf("Message = %q; want to contain \"```\" (fence retained because manifest strip_code_fence=false)",
			res.Message)
	}
}
```

**Task 5 — clause (d), regression (reuses setupTestRepo):**

```go
// TestGenerateCommit_ManifestDefaultRaw_StillWorks is a regression guard (PRD bugfix-002 Issue 2 clause d):
// with no [generation] block and the manifest default output="raw"/strip_code_fence=true (setupTestRepo's
// .stagecoach.toml), a plain raw message still round-trips unchanged. This must pass BOTH before and after
// the S2 fix (raw/true is the unchanged default).
func TestGenerateCommit_ManifestDefaultRaw_StillWorks(t *testing.T) {
	setupTestRepo(t, stubtest.Options{Out: "feat: default raw ok"}) // output="raw", strip=true, no [generation]
	repoDir, _ := os.Getwd()

	writeFile(t, repoDir, "new.txt", "data")
	stageFile(t, repoDir, "new.txt")

	res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
	if err != nil {
		t.Fatalf("GenerateCommit: %v", err)
	}
	if res.Message != "feat: default raw ok" {
		t.Errorf("Message = %q, want %q (default raw path must be unchanged)", res.Message, "feat: default raw ok")
	}
}
```

> **Clause (c)** (`[generation] output="json"` overrides a `[provider.stub] output="raw"` manifest → JSON parsed)
> is **already covered** by the existing `TestGenerateCommit_GenerationConfigFile_OutputJSON_Issue4` (line 675)
> and the git-config variant (line 727). Do NOT duplicate them — just verify they stay GREEN after Task 1
> (the `!= nil` guard preserves the non-nil override path).

### Implementation Patterns & Key Details

```go
// PATTERN (tri-state opt-in override): `if cfg.X != nil { m.X = cfg.X }`. nil ⇒ defer to the manifest;
//   non-nil ⇒ override. This is the SAME shape StripCodeFence already had; Output becomes its twin now
//   that S1 made it *string. The manifest's Resolve() supplies the final raw/true fallback.

// WHY assign the pointer directly (m.Output = cfg.Output, no local copy): cfg.Output is *string. The old
//   `o := cfg.Output; m.Output = &o` existed because cfg.Output was a plain string and &cfg.Output would
//   alias the value-param's field. With a pointer, the assignment copies the pointer into m — safe and
//   one line. (StripCodeFence already does exactly this.)

// WHY no re-Validate(): Validate() checks Name/Command only. An Output like "yaml" hits ParseOutput's
//   switch-default → treated as raw (graceful). The bridge does not need to enforce Output validity.

// WHY custom TOML (not setupTestRepo) for (a)/(b): the shared helpers hardcode output="raw"/strip=true.
//   Writing the toml inline (as test #1 does) still runs the REAL config.Load+registry path — GenerateCommit
//   → resolveConfig → config.Load reads CWD/.stagecoach.toml → DecodeUserOverrides → registry merges.
```

### Integration Points

```yaml
CODE: pkg/stagecoach/stagecoach.go buildDeps (the one source edit) + pkg/stagecoach/stagecoach_test.go.
DATABASE: none.
CONFIG: none (no new config keys; the [generation] output/strip_code_fence TOML keys and git-config keys
        are unchanged — only their override SEMANTICS change from always-on to opt-in).
ROUTES: none.
SIGNALS: none.
DOWNSTREAM: none — S1 (config package) is already merged; no further subtask depends on this bridge.
```

---

## Validation Loop

### Level 1: Syntax & Type (run after Task 1)

```bash
# The whole repo must now compile (S1's transient pkg/stagecoach break is resolved by Task 1).
go build ./...
go vet ./...
# Expected: clean. If pkg/stagecoach/stagecoach.go still errors at the cfg.Output site, the edit is incomplete.
gofmt -l pkg/stagecoach/stagecoach.go
# Expected: lists nothing. If it does: gofmt -w pkg/stagecoach/stagecoach.go
```

### Level 2: The new + fixed tests (run after Tasks 1-5)

```bash
# Run the pkg/stagecoach suite verbosely, with -race.
go test -race ./pkg/stagecoach/... -v
# Expected: ALL PASS. Specifically watch:
#   TestGenerateCommit_ManifestOutputJSON_Honored_NoGeneration .... PASS (clause a — keystone)
#   TestGenerateCommit_ManifestStripCodeFenceFalse_Honored_NoGeneration .. PASS (clause b)
#   TestGenerateCommit_ManifestDefaultRaw_StillWorks ............. PASS (clause d)
#   TestGenerateCommit_ManifestOutputWins_WhenCfgOutput[Empty|Nil]_Issue4 .. PASS (compile-fixed)
#   TestGenerateCommit_GenerationConfigFile_OutputJSON_Issue4 ... PASS (clause c — still green)
#   TestGenerateCommit_GitConfig_OutputJSON_Issue4 .............. PASS
#   TestGenerateCommit_InjectedConfig_StripCodeFenceFalse_Issue4 .. PASS
```

### Level 3: Regression guard (prove the new tests catch the bug — optional, throwaway)

```bash
# Sanity check the keystone (clause a) fails under the OLD clobber. On a throwaway branch:
#   in buildDeps, temporarily replace `if cfg.Output != nil { m.Output = cfg.Output }` with an
#   unconditional clobber, e.g. `o := "raw"; m.Output = &o` (mimics bugfix-001), and re-run:
#     go test -race -run TestGenerateCommit_ManifestOutputJSON_Honored_NoGeneration ./pkg/stagecoach/ -v
#   It MUST now FAIL (Message == the raw JSON blob). Then `git checkout pkg/stagecoach/stagecoach.go`.
# This is OPTIONAL verification of test quality, not a CI gate.
```

### Level 4: Full suite + docs consistency (run after Tasks 6-7)

```bash
# Entire suite, race-enabled — no regressions (raw/true default is unchanged for installed providers).
go test -race ./...
# Expected: all PASS.

# Docs consistency: the opt-in-override wording is present and the defaults-table no longer lies.
grep -n "opt-in\|broader layer wins\|config.Defaults()" docs/configuration.md docs/providers.md
# Expected: "broader layer wins" is GONE; "opt-in" appears; the output/strip_code_fence rows' Source
# column no longer says config.Defaults() (it says provider manifest / §12.1).
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (S1 transient break resolved).
- [ ] `go vet ./...` clean.
- [ ] `gofmt -l pkg/stagecoach/stagecoach.go pkg/stagecoach/stagecoach_test.go` reports nothing.
- [ ] `go test -race ./...` — entire suite green.

### Feature Validation (Issue 2 contract)
- [ ] `buildDeps` Output guard is `if cfg.Output != nil { m.Output = cfg.Output }` (no local copy).
- [ ] `buildDeps` StripCodeFence guard unchanged (`!= nil`); both branches symmetric.
- [ ] The stale "broader setting wins / decisions.md D4" comment is replaced.
- [ ] (a) manifest `output=json` honored with no `[generation]` → extracted JSON field.
- [ ] (b) manifest `strip_code_fence=false` honored with no `[generation]` → fence retained.
- [ ] (c) existing `[generation] output=json` override test stays green.
- [ ] (d) default raw path unchanged.
- [ ] test #4 compiles (`cfg.Output = nil`) and comments reflect nil/`!= nil`.

### Code Quality Validation
- [ ] One source edit only (buildDeps); no edits to config/*, provider/*, generate/*, cmd/*.
- [ ] New tests mirror the existing Issue-4 test pattern (custom TOML, real config.Load+registry path).
- [ ] No duplicated test for clause (c) (existing test covers it).
- [ ] Test #4 update preserves its intent (injected-config manifest-wins guard) — only value + comments change.

### Documentation (Mode A)
- [ ] docs/configuration.md line 84 → opt-in override.
- [ ] docs/configuration.md lines 79-80 → Source column = provider manifest (§12.1), not config.Defaults().
- [ ] docs/configuration.md lines 55-56 → opt-in note on the commented examples.
- [ ] docs/providers.md line 124 → opt-in override.
- [ ] docs/providers.md lines 30-32 → manifest defaults kept; opt-in note added.
- [ ] "broader layer wins" wording eliminated from both files.

---

## Anti-Patterns to Avoid

- ❌ **Don't keep `if cfg.Output != ""`** (or "fix" it to `!= ""` on a string) — after S1, `cfg.Output` is
  `*string`; the guard MUST be `!= nil` or it won't compile / reintroduces the clobber.
- ❌ **Don't keep the `o := cfg.Output` local copy** — assign the pointer directly (`m.Output = cfg.Output`),
  exactly as the StripCodeFence branch already does.
- ❌ **Don't omit `json_field` from the clause-(a) test TOML** — without it, ParseOutput's json extraction
  falls back to raw and the test would observe the raw blob (a false pass if you asserted on the blob).
  Assert on the EXTRACTED field value.
- ❌ **Don't use setupTestRepo/setupScriptedRepo for the json/false tests** — they hardcode `output="raw"`
  /`strip_code_fence=true`. Write a custom `.stagecoach.toml` (mirror existing Issue-4 test #1).
- ❌ **Don't duplicate clause (c)** — `TestGenerateCommit_GenerationConfigFile_OutputJSON_Issue4` already
  covers `[generation] output=json` overriding a `raw` manifest. Verify it stays green; don't add a twin.
- ❌ **Don't edit `internal/config/*` or `internal/provider/*`** — S1 owns the config package; the provider
  package is a read-only consumer. The fix is entirely in `pkg/stagecoach/stagecoach.go`.
- ❌ **Don't change `buildDeps`'s other guards** (unknown-provider, Validate, IsInstalled pre-flight) —
  surgical replacement of the Output/StripCodeFence block only.
- ❌ **Don't leave the stale comment** referencing "decisions.md D4" / "broader setting wins" / "always
  non-empty post-Defaults" — all three claims are now false and would mislead future readers.
- ❌ **Don't claim `config.Defaults()` supplies output/strip_code_fence in the docs** — S1 removed those;
  the effective raw/true default comes from the manifest's `Resolve()`.

---

## Confidence Score

**9/10** — A two-line bridge edit (`!= ""`+local-copy → `!= nil`+pointer-assign, mirroring the
already-correct StripCodeFence twin) that turns S1's already-merged types into the documented
opt-in-override behavior, plus copy-paste-ready tests (each with a TDD revert-check), an exact
compile-fix for the one stale test, and precise doc anchors with current→replacement wording. The
-1 reserves for the doc-table source-column judgment call (whether to say "provider manifest (§12.1)"
or keep a note inline) and the optional test-rename, both non-blocking and spelled out.
