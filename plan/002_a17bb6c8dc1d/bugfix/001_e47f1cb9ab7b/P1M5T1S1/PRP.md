---
name: "Blank pi per-role models in bootstrap + add sub-provider annotation (Issue 5)"
work_item: P1.M5.T1.S1
changeset: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b (Stagecoach v2.0 QA bug pass)
issue: 5 (Minor — latent; surfaces once Issue 1 is fixed, which is Complete)
kind: code + test (Go), Mode-A doc (the bootstrap output header comment IS the user-facing doc)
depends_on:
  - P1.M1.T1 (Issue 1, Critical: provider/sub-provider conflation — callers now pass "" so Render omits --provider when pi DefaultProvider is "")  ✅ Complete
  # Issue 5 is LATENT on Issue 1: pi only reaches model-routing once Issue 1 stops emitting the bogus
  # --provider pi. With Issue 1 fixed, pi now receives `pi --model gpt-5.4-nano …` with NO --provider,
  # routes gpt-5.4-nano to its default backend ("google") where it does NOT exist → model-not-found.
---

## Goal

**Feature Goal**: Make `config init` / auto-bootstrap produce a **functional out-of-the-box** config
for the default provider **pi** (PRD §9.17 FR-B1 "writes a populated, **working** config"). Today the
bootstrap writes pi's per-role `gpt-5.4*` models (planner/stager/message/arbiter) but **no**
`default_provider` (sub-provider). Those models cannot route without a sub-provider, so once Issue 1
stopped emitting a bogus `--provider pi`, pi routes them to its default backend ("google") where they
don't exist → **model-not-found**. The fix: when the bootstrap target is **pi** (which has no
`default_provider`), blank all four per-role models (`model = ""`) so pi picks its own backend
default, and add a clear annotation telling the user how to pin a sub-provider + compatible models.

**Deliverable**:

1. `internal/config/bootstrap.go` `buildBootstrapConfig`: when `target == "pi"`, write `model = ""`
   for **all four** `[role.*]` blocks (planner, stager, message, arbiter) and emit one sub-provider
   annotation comment in the per-role header block.
2. Updated tests in `internal/config/bootstrap_test.go` AND `internal/cmd/config_test.go` reflecting
   the blank pi models (claude/gemini unchanged).
3. The **`roleDefaults` table** (`role_defaults.go`) is **NOT modified** — only the bootstrap output
   is blanked for pi.

**Success Definition**:

- `GenerateBootstrapConfig("pi")` / `buildBootstrapConfig("pi", …)` produces `model = ""` for all
  four role blocks and contains **no** `gpt-5.4` string in a pi-only config.
- `GenerateBootstrapConfig("claude")` still writes opus/sonnet/haiku (claude has no sub-provider
  concept; ProviderFlag is empty `""`) — **unchanged**.
- The sub-provider annotation comment appears (once) in the pi bootstrap.
- The gemini stager-fallback path (stager routed to pi with `gpt-5.4-mini`) is **unchanged** (target
  != pi is not blanked).
- `go build ./...`, `go vet ./...`, `go test ./...` all green.

## Why

- **FR-B1 contract**: "the tool works immediately" after `config init`. The default provider is pi,
  so the shipped bootstrap must produce a working pi config. Writing un-routable models violates the
  "works immediately" promise the moment Issue 1 lands.
- **Root cause** (architecture doc + verified in code): `DefaultModelsForProvider("pi")` returns the
  FR-D4 table column `{gpt-5.4, gpt-5.4-mini, gpt-5.4-nano, gpt-5.4-mini}`. These are OpenAI models
  that require a pi sub-provider (`default_provider`) to route. The bootstrap writes none (Appendix E
  #12 is an open question), so the bootstrap ships a config that cannot route. Blanking the models is
  the conservative Option-A fix until a verified routing sub-provider exists.
- **Scope discipline**: this is the **only** Issue-5 task. Do NOT touch the `roleDefaults` table, do
  NOT attempt Option B (write a `default_provider` — needs external verification, Appendix E #12),
  do NOT change the gemini/other-provider bootstrap paths.

## What

### Behavioral change (pi target only)

`config init` (or the first-run auto-bootstrap) for **pi** now emits:

```toml
[defaults]
provider = "pi"
# ...

# --- per-role models for the default provider "pi" (PRD §16.4, §9.15) ---
# NOTE: pi requires a default_provider (sub-provider) to route models. The shipped per-role
# models are empty so pi picks its own backend default; set [provider.pi] default_provider
# and compatible per-role models to pin a specific backend.

[role.planner]
model = ""

[role.stager]
provider = "pi"
model = ""

[role.message]
model = ""

[role.arbiter]
model = ""
```

Non-pi targets (claude, gemini, …) are **byte-identical** to today.

### Success Criteria

- [ ] `buildBootstrapConfig("pi", …)` writes `model = ""` for planner, stager, message, arbiter.
- [ ] A pi-only bootstrap contains **no** `gpt-5.4` substring.
- [ ] The sub-provider NOTE annotation appears exactly once.
- [ ] `buildBootstrapConfig("claude", …)` still writes opus/sonnet/haiku (unchanged).
- [ ] The gemini stager-fallback test still passes (stager → pi with `gpt-5.4-mini`, target != pi).
- [ ] `go test ./internal/config/ ./internal/cmd/` green.

## All Needed Context

### Context Completeness Check

**Pass** — this PRP quotes the exact code to change (with line numbers), the exact trap to avoid
(the stager re-pull), the exact existing assertions to update, and the validation commands. An agent
who has never seen this repo can implement it from this file + the three named `.go` files.

### Documentation & References

```yaml
# MUST READ — the authoritative recon for this issue
- file: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue5_bootstrap_config.md
  why: Root cause + Option A (blank models) vs Option B (write default_provider). Recommends Option A.
  critical: "the shipped bootstrap should not write models that cannot route" — blank pi models.

# MUST READ — the file being edited
- file: internal/config/bootstrap.go
  why: buildBootstrapConfig (the edit target), stagerFallback (the re-pull trap), writeRoleBlock.
  pattern: models come from DefaultModelsForProvider(target) which returns a MUTABLE COPY.
  gotcha: |
    stagerFallback(target, models) RE-PULLS DefaultModelsForProvider(name) for each preferredBuiltin
    when models["stager"]=="" — that returns a FRESH copy from the package table, so blanking the
    `models` map does NOT blank the stager output. The stager MUST be blanked explicitly. (See
    Implementation Tasks Task 1, step 3 — the whole PRP hinges on this.)
  key lines:
    - 117: `models := DefaultModelsForProvider(target)`
    - 118: `stagerName, stagerModel := stagerFallback(target, models)`
    - 120: `fmt.Fprintf(&b, "\n# --- per-role models for the default provider %q (…) ---\n", target)`
    - 122-126: planner/stager/message/arbiter writeRoleBlock calls
    - 87-95: stagerFallback (the re-pull loop at line 90-93)

# MUST READ — the table that feeds the bootstrap (DO NOT MODIFY)
- file: internal/config/role_defaults.go
  why: roleDefaults["pi"] = {planner:gpt-5.4, stager:gpt-5.4-mini, message:gpt-5.4-nano, arbiter:gpt-5.4-mini}.
        DefaultModelsForProvider returns a COPY (line 96-103) — callers may mutate freely.
  pattern: DO NOT change this table. The blanking happens in buildBootstrapConfig for pi ONLY.
  gotcha: role_defaults_test.go asserts the table still has gpt-5.4* for pi — that test must STAY GREEN
          (the table is unchanged).

# MUST EDIT — unit tests (buildBootstrapConfig level)
- file: internal/config/bootstrap_test.go
  why: TestBuildBootstrapConfig_Pi asserts pi writes gpt-5.4* — MUST change to model = "".
        TestBuildBootstrapConfig_OtherInstalledCommented (target pi) asserts gpt-5.4 / gpt-5.4-nano — MUST change.
  pattern: uses the loose `assertContains(t, content, "[role.planner]", 'model = "gpt-5.4"')` helper (substrings,
           not same-line). Keep using it but ALSO add a precise negative assertion (`!strings.Contains(content, "gpt-5.4")`).
  gotcha: the loose helper would NOT catch a stager that still has gpt-5.4-mini (other roles' `model = ""`
          satisfy the substring). The negative `gpt-5.4` assertion is what catches the stager re-pull bug.

# MUST EDIT — CLI-level tests (config init --provider pi)
- file: internal/cmd/config_test.go
  why: the `config init --provider pi` test (function near line 320; assertions at 350-353) asserts pi
        writes gpt-5.4* via the real CLI path (GenerateBootstrapConfig). MUST change to model = "".
  pattern: same loose `assertContains` helper (imported / local in this package).
  key lines: 350-353 (pi model assertions) → change to `model = ""`.
  do NOT touch: line ~391 (TestConfigInit_ProviderStagerFallback, GEMINI target) — gemini's stager
                fallback to pi with gpt-5.4-mini stays correct (target != pi, not blanked).

# SUPPORTING — confirms WHY pi models can't route (Issue 1 surface)
- file: internal/provider/builtin.go
  why: builtinPi() has DefaultProvider = strPtr("") (line 50, §12.3 explicit-empty NON-NIL) and
        ProviderFlag = strPtr("--provider") (line 49). Per Issue 1's fix, Render omits --provider when
        DefaultProvider is "" → pi routes the model on its own default backend. This is WHY gpt-5.4*
        (OpenAI models) hit pi's "google" default backend and fail.
  critical: read-only context — do NOT modify builtin.go.
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
├── bootstrap.go          # ← EDIT: buildBootstrapConfig (blank pi models + annotation)
├── bootstrap_test.go     # ← EDIT: TestBuildBootstrapConfig_Pi + _OtherInstalledCommented
├── role_defaults.go      # READ-ONLY: roleDefaults table + DefaultModelsForProvider (NOT modified)
├── role_defaults_test.go # UNCHANGED (asserts table still has gpt-5.4* for pi — stays green)
└── roles.go, config.go, load.go …
internal/cmd/
├── config.go             # calls config.GenerateBootstrapConfig (line ~262) — no change needed
└── config_test.go        # ← EDIT: config init --provider pi assertions (lines ~350-353)
internal/provider/
└── builtin.go            # READ-ONLY context (pi DefaultProvider="" / ProviderFlag="--provider")
```

### Desired Codebase tree

```bash
# No files added or deleted. Only 3 files edited:
internal/config/bootstrap.go          # buildBootstrapConfig: blank pi models + annotation
internal/config/bootstrap_test.go     # update pi assertions
internal/cmd/config_test.go           # update pi assertions (config init path)
```

### Known Gotchas of our codebase & Library quirks

```go
// CRITICAL — the stager re-pull trap (THE thing that makes this non-trivial):
// stagerFallback (bootstrap.go:87-95) does, when models["stager"] == "":
//   for _, name := range preferredBuiltins {
//       if col := DefaultModelsForProvider(name); col != nil && col["stager"] != "" {
//           return name, col["stager"]
//       }
//   }
// DefaultModelsForProvider returns a FRESH copy from the package-level roleDefaults table — NOT the
// `models` map you just blanked. So for pi (blanked stager) it immediately re-finds pi's stager =
// "gpt-5.4-mini" and returns ("pi", "gpt-5.4-mini"). If you only blank the `models` map, the stager
// block will STILL print `model = "gpt-5.4-mini"` → 3 roles blank, stager not → test fails.
// FIX: after stagerFallback, when target=="pi" also set stagerModel = "" (keep stagerName = "pi").

// GOTCHA — keep the stager routed to pi (do NOT blank stagerName):
// For pi target, stagerName == "pi" (== target), so writeRoleBlock writes `provider = "pi"` for the
// stager. That is the EXISTING behavior (pre-fix the stager block already wrote provider="pi"). Keep
// it — only the model changes to "". Do NOT set stagerName="" (that would drop the provider line and
// is an unrelated behavior change).

// CRITICAL — only blank when target == "pi":
// claude's models (opus/sonnet/haiku) need NO sub-provider (claude ProviderFlag is "" — no
// --provider concept). gemini/agy/etc. likewise. Blanking is pi-specific. The gemini stager-fallback-
// to-pi path (stager="gpt-5.4-mini") is INTENTIONALLY left as-is (out of scope; separate concern).
// Tests for claude/gemini MUST stay green and unchanged.

// GOTCHA — the loose assertContains helper:
// `assertContains(t, content, "[role.stager]", 'model = ""')` passes as long as BOTH substrings exist
// ANYWHERE — so with 4× `model = ""` it cannot prove the STAGER specifically is blank. The robust
// negative check is `!strings.Contains(content, "gpt-5.4")` for a pi-only config (no commented
// provider blocks ⇒ gpt-5.4 must be entirely absent). This single assertion catches the re-pull bug.

// GOTCHA — do NOT modify roleDefaults (role_defaults.go):
// The table is the FR-D4 source of truth; role_defaults_test.go asserts pi still maps to gpt-5.4*.
// The blanking is a bootstrap-OUTPUT concern only. Editing the table would break that test and
// change DefaultModelsForProvider for ALL callers (not just bootstrap).

// GOTCHA — TOML validity: `model = ""` is valid TOML (empty string). The TestBuildBootstrapConfig_
// ValidTOML table test (bootstrap_test.go) already covers {"pi", …} cases — it must still pass.
```

## Implementation Blueprint

### Data models and structure

No data-model changes. The `models map[string]string` returned by `DefaultModelsForProvider(target)`
is already a mutable copy (`role_defaults.go:96-103`); the fix mutates that copy in place for pi.
The `roleDefaults` table and `RoleModelDefaults` type are **unchanged**.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/config/bootstrap.go — blank pi models + annotation (THE core edit)
  - LOCATE: buildBootstrapConfig (line ~110). The three relevant lines:
        117   models := DefaultModelsForProvider(target) // non-nil (target is a validated built-in)
        118   stagerName, stagerModel := stagerFallback(target, models)
        120   fmt.Fprintf(&b, "\n# --- per-role models for the default provider %q (PRD §16.4, §9.15) ---\n", target)
  - STEP 1 (blank the copy): right after line 117, add a pi branch that blanks the mutable copy:
        models := DefaultModelsForProvider(target)
        piBlanked := target == "pi"
        if piBlanked {
            // pi's gpt-5.4* models require a sub-provider (default_provider) to route; the bootstrap
            // writes none (Appendix E #12 open). Blank them so pi picks its own backend default.
            for role := range models {
                models[role] = ""
            }
        }
  - STEP 2 (stager re-pull fix — CRITICAL): after the existing `stagerName, stagerModel := stagerFallback(...)`
    call (line 118), blank the stager model explicitly when piBlanked (stagerFallback re-pulls the
    table value; see Gotchas):
        stagerName, stagerModel := stagerFallback(target, models)
        if piBlanked {
            // stagerFallback re-pulls pi's stager model from the FR-D4 table (a fresh copy); force
            // it blank so all four roles stay empty. pi remains the stager (stager-capable).
            stagerModel = ""
        }
    NOTE: keep stagerName as returned ("pi"); do NOT blank stagerName (see Gotchas).
  - STEP 3 (annotation): right after the section-header Fprintf (line 120), emit the NOTE once:
        fmt.Fprintf(&b, "\n# --- per-role models for the default provider %q (PRD §16.4, §9.15) ---\n", target)
        if piBlanked {
            b.WriteString("# NOTE: pi requires a default_provider (sub-provider) to route models. The shipped per-role\n")
            b.WriteString("# models are empty so pi picks its own backend default; set [provider.pi] default_provider\n")
            b.WriteString("# and compatible per-role models to pin a specific backend.\n")
        }
  - PRESERVE: the planner/stager/message/arbiter writeRoleBlock calls below (lines ~122-126) are
    UNCHANGED — they already read from `models[...]` / `stagerName,stagerModel`, so blanking flows
    through automatically. Do not touch them.
  - RESULT: pi bootstrap writes `model = ""` for all four roles + the NOTE. Non-pi targets unchanged.

Task 2: MODIFY internal/config/bootstrap_test.go — update pi assertions + add robustness
  - TestBuildBootstrapConfig_Pi: REPLACE the four gpt-5.4* assertions with empty-model + negative:
        // OLD (delete):
        //   assertContains(t, content, "[role.planner]", `model = "gpt-5.4"`)
        //   assertContains(t, content, "[role.stager]",  `model = "gpt-5.4-mini"`)
        //   assertContains(t, content, "[role.message]", `model = "gpt-5.4-nano"`)
        //   assertContains(t, content, "[role.arbiter]", `model = "gpt-5.4-mini"`)
        // NEW:
        assertContains(t, content, "[role.planner]", `model = ""`)
        assertContains(t, content, "[role.stager]",  `model = ""`)
        assertContains(t, content, "[role.message]", `model = ""`)
        assertContains(t, content, "[role.arbiter]", `model = ""`)
        // Negative: a pi-only config must contain NO gpt-5.4 anywhere (catches the stager re-pull):
        if strings.Contains(content, "gpt-5.4") {
            t.Errorf("pi bootstrap must not ship un-routable gpt-5.4* models; got:\n%s", content)
        }
        // Annotation present (once):
        if !strings.Contains(content, "requires a default_provider (sub-provider)") {
            t.Error("pi bootstrap missing the sub-provider annotation")
        }
    KEEP: the existing "pi IS stager-capable — no fallback annotation" check (line ~no 'cannot serve').
  - TestBuildBootstrapConfig_OtherInstalledCommented (target pi, also claude installed): the two pi
    model assertions must flip to empty (claude's commented opus/sonnet/haiku assertions STAY):
        // OLD: assertContains(t, content, "[role.planner]", `model = "gpt-5.4"`)
        //      assertContains(t, content, "[role.message]", `model = "gpt-5.4-nano"`)
        // NEW:
        assertContains(t, content, "[role.planner]", `model = ""`)
        assertContains(t, content, "[role.message]", `model = ""`)
    KEEP: the claude commented-block assertions (`# model = "haiku"`, `=== claude (installed)`) and
          the `[role.message]` count == 1 check — all still hold. NOTE: with claude installed, the
          config legitimately contains NO gpt-5.4 (pi blanked, claude is opus/sonnet/haiku), so an
          optional `!strings.Contains(content, "gpt-5.4")` is also valid here.
  - TestBuildBootstrapConfig_GeminiStagerFallback and _ValidTOML: NO CHANGES (target != pi; TOML valid).
  - TestGenerateBootstrapConfig_NamedProvider (claude): NO CHANGES (asserts opus/sonnet/haiku).

Task 3: MODIFY internal/cmd/config_test.go — update the config init --provider pi assertions
  - LOCATE: the `config init --provider pi` test (rootCmd.SetArgs `{"config","init","--provider","pi"}`),
    assertions at lines ~350-353:
        // OLD (delete):
        //   assertContains(t, content, "[role.planner]", `model = "gpt-5.4"`)
        //   assertContains(t, content, "[role.message]", `model = "gpt-5.4-nano"`)
        //   assertContains(t, content, "[role.stager]",  `model = "gpt-5.4-mini"`)
        //   assertContains(t, content, "[role.arbiter]", `model = "gpt-5.4-mini"`)
        // NEW:
        assertContains(t, content, "[role.planner]", `model = ""`)
        assertContains(t, content, "[role.message]", `model = ""`)
        assertContains(t, content, "[role.stager]",  `model = ""`)
        assertContains(t, content, "[role.arbiter]", `model = ""`)
        if strings.Contains(content, "gpt-5.4") {
            t.Errorf("config init --provider pi must not ship gpt-5.4* models; got:\n%s", content)
        }
    KEEP: the config_version=2, provider="pi", and "no fallback annotation" checks.
  - DO NOT TOUCH: TestConfigInit_ProviderStagerFallback (GEMINI, line ~391) — gemini's stager routed
    to pi with `model = "gpt-5.4-mini"` stays correct (target != pi is not blanked). This is the
    regression guard that proves you only blanked pi.

Task 4: VERIFY (no edit) — table + other providers unchanged
  - role_defaults_test.go: must pass unchanged (asserts roleDefaults["pi"] still = gpt-5.4*). If it
    fails, you accidentally edited role_defaults.go — revert that.
  - All non-pi bootstrap behavior (claude opus/sonnet/haiku, gemini stager-fallback) unchanged.
```

### Implementation Patterns & Key Details

```go
// PATTERN: mutate the COPY returned by DefaultModelsForProvider, never the package table.
// role_defaults.go documents DefaultModelsForProvider returns "a COPY ... callers may mutate it
// freely without affecting the package-level table" — rely on exactly that contract.

// PATTERN: the existing writeRoleBlock(b, role, prov, model, annotation) already handles model=""
// correctly (it does `fmt.Fprintf(b, "model = %q\n", model)` ⇒ `model = ""`). No helper change needed.

// CRITICAL ordering: blank the models map (Task 1 step 1) is NOT sufficient for the stager because
// stagerFallback re-pulls from the table. The explicit `if piBlanked { stagerModel = "" }` (step 2)
// is mandatory. Without it the test's `!strings.Contains(content, "gpt-5.4")` will FAIL on the stager.
```

### Integration Points

```yaml
NONE beyond the three files. buildBootstrapConfig is pure (no I/O, no detection); it is called by:
  - GenerateBootstrapConfig(prov)  (bootstrap.go:31) — used by `config init` (internal/cmd/config.go:262)
  - bootstrapWriteConfig(path)     (bootstrap.go:37) — the Load() first-run fallback (FR-B3)
Both callers pass the resolved target straight through, so the blanking reaches every bootstrap path
automatically. No CLI wiring, no config-schema change, no manifest change.
```

## Validation Loop

### Level 1: Syntax & Style (immediate)

```bash
# Build + vet the touched packages (and whole module).
go build ./...
go vet ./internal/config/ ./internal/cmd/
# Expected: clean. If vet complains, read + fix.
```

### Level 2: Unit / package tests (the primary gate)

```bash
# The two packages with edited tests:
go test ./internal/config/ -run 'TestBuildBootstrapConfig|TestGenerateBootstrapConfig' -v
go test ./internal/cmd/   -run 'TestConfigInit' -v

# Full affected packages:
go test ./internal/config/ ./internal/cmd/
# Expected: PASS. Specifically these MUST pass:
#   TestBuildBootstrapConfig_Pi                      (pi now model="")
#   TestBuildBootstrapConfig_OtherInstalledCommented (pi model="", claude commented intact)
#   TestBuildBootstrapConfig_GeminiStagerFallback    (UNCHANGED — gemini stager→pi gpt-5.4-mini)
#   TestBuildBootstrapConfig_ValidTOML               (all targets incl. pi still valid TOML)
#   TestGenerateBootstrapConfig_NamedProvider        (claude opus/sonnet/haiku UNCHANGED)
#   role_defaults_test.go (whole file)               (table UNCHANGED — pi still gpt-5.4*)
#   TestConfigInit (pi case, ~line 350-353)          (pi now model="")
#   TestConfigInit_ProviderStagerFallback            (gemini UNCHANGED — regression guard)
```

### Level 3: Whole-module regression + behavior spot-check

```bash
# Whole suite with the race detector (the project's standard gate):
go test ./...
# Expected: all green. The blanking is pi-bootstrap-output-only; no other package should regress.

# (Optional) eyeball the actual generated bootstrap for pi:
go test ./internal/config/ -run TestBuildBootstrapConfig_Pi -v 2>&1 | head   # then print content if useful
# Or run: bin/stagecoach config init --provider pi  (in a throwaway HOME) and confirm model="" x4 + NOTE.
```

### Level 4: Manual smoke (optional, confirms FR-B1 "works immediately")

```bash
# In a temp HOME so you don't clobber your real config:
export TMPHOME=$(mktemp -d) && export HOME=$TMPHOME && export XDG_CONFIG_HOME=$TMPHOME/.config
SH=$(pwd)/bin/stagecoach   # build first: make build
$SH config init --provider pi
cat "$XDG_CONFIG_HOME/stagecoach/config.toml" | grep -E '^\[role|^model|default_provider|NOTE'
# Expected: four [role.*] blocks each with `model = ""`, and the NOTE line. NO gpt-5.4 lines.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` clean
- [ ] `go vet ./internal/config/ ./internal/cmd/` clean
- [ ] `go test ./...` green (whole module, incl. `-race` if you run it: `go test -race ./...`)

### Feature Validation

- [ ] `buildBootstrapConfig("pi", …)` writes `model = ""` for all four roles
- [ ] A pi-only bootstrap contains no `gpt-5.4` substring (negative assertion passes)
- [ ] The sub-provider NOTE annotation appears (once) in the pi bootstrap
- [ ] `buildBootstrapConfig("claude", …)` still writes opus/sonnet/haiku (unchanged)
- [ ] The gemini stager-fallback path is unchanged (TestConfigInit_ProviderStagerFallback + TestBuildBootstrapConfig_GeminiStagerFallback green)
- [ ] `roleDefaults` table unchanged (role_defaults_test.go green — pi still gpt-5.4*)

### Code Quality Validation

- [ ] Only `bootstrap.go`, `bootstrap_test.go`, `cmd/config_test.go` modified (`git status --short`)
- [ ] `role_defaults.go` NOT modified
- [ ] `internal/provider/builtin.go` NOT modified (read-only context)
- [ ] The stager re-pull trap is handled (explicit `stagerModel = ""` for pi)
- [ ] Follows existing patterns: mutate the DefaultModelsForProvider copy; reuse writeRoleBlock; reuse the loose `assertContains` helper + add a precise negative assertion

### Documentation (Mode A — bootstrap output IS the doc)

- [ ] The sub-provider annotation in the generated TOML is clear and accurate (mentions `default_provider`, that empty models let pi pick its backend, and how to pin a backend)
- [ ] No separate docs/* file needs editing for this subtask (the header comment is the user-facing doc)

---

## Anti-Patterns to Avoid

- ❌ Don't blank the stager by only editing the `models` map — `stagerFallback` re-pulls from the table and the stager will keep `gpt-5.4-mini`. You MUST explicitly set `stagerModel = ""` for pi.
- ❌ Don't modify `roleDefaults` / `role_defaults.go` — the blanking is a bootstrap-OUTPUT concern; the table is the FR-D4 source of truth and other tests assert it.
- ❌ Don't blank models for non-pi targets — claude/gemini/etc. need no sub-provider and their tests must stay green. The blanking is `target == "pi"` ONLY.
- ❌ Don't change `stagerName` for pi — keep it `"pi"` (only the model blanks); dropping the provider line is an unrelated behavior change.
- ❌ Don't write a `default_provider` into the bootstrap (Option B) — that needs external verification (Appendix E #12) and is explicitly out of scope.
- ❌ Don't rely on the loose `assertContains(..., "model = \"\"")` alone to prove the stager is blank — add the `!strings.Contains(content, "gpt-5.4")` negative assertion; that's what catches the re-pull bug.
- ❌ Don't edit `internal/cmd/config_test.go`'s gemini stager-fallback test (~line 391) — it's the regression guard proving you only touched pi.
