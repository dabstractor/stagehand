---
name: "P1.M2.T2.S1 (bugfix Issue 2) — Use the message role's provider in buildDeps for manifest selection"
description: |
  Bugfix for Issue 2 (PRD §9.15 FR-R3), PROVIDER half. Per-role `message` overrides are silently ignored
  on the single-commit path: `--message-provider X` / `[role.message] provider = "X"` / the message role's
  resolved provider never reaches manifest selection because `buildDeps` (pkg/stagecoach/stagecoach.go:316)
  reads the GLOBAL `cfg.Provider` (L324) instead of resolving the `message` role. Fix: replace
  `name := cfg.Provider` with `msgProvider, _, _ := config.ResolveRoleModel("message", cfg); name := msgProvider`.
  The existing `if name == ""` auto-detect block (L325) stays — it still fires when both the message
  provider and the global are empty (the common no-config case). With no message override,
  `ResolveRoleModel("message", cfg)` returns exactly `cfg.Provider` — **byte-identical / back-compatible**.

  ⚠️ **THE central design call — buildDeps selects ONLY the provider/manifest; model+reasoning are the
  sibling task's concern.** `config.ResolveRoleModel` returns `(provider, model, reasoning)`. buildDeps
  consumes ONLY the provider (the manifest's identity); the model+reasoning are DISCARDED with `_`. They
  are resolved at the Render call sites (`generate.CommitStaged` + `runPipeline`) by the parallel task
  P1.M2.T1.S1. Splitting the concerns this way (a) avoids a "declared and not used" compile error, (b)
  mirrors how buildDeps already works (it selects the manifest; Render fills model/reasoning), and (c)
  does NOT overlap with the sibling task (which edits runPipeline/generate.go, NOT buildDeps).

  ⚠️ **THE second design call — the `if name == ""` auto-detect block is UNCHANGED.** `ResolveRoleModel`
  returns `""` for the provider when both the message role and the global are empty; the existing
  registry auto-detect (reg.List → IsInstalled → DefaultProvider) then runs exactly as today. Do NOT
  remove or restructure it — only the single `name :=` initializer line changes.

  ⚠️ **THE third design call — the test must use STUB-BACKED providers, NOT the literal "pi"/"claude"
  built-ins.** buildDeps has an `IsInstalled` pre-flight (exec.LookPath) that rejects any provider whose
  command is not on $PATH; "pi"/"claude" binaries are absent in CI. Register two providers ("alpha"/
  "beta") via `cfg.Providers` (the raw map DecodeUserOverrides reads), both with `command = <stub binary
  abs path>` (stubtest.Build) so IsInstalled passes. The test is WHITE-BOX (`package stagecoach`) —
  stagecoach_test.go already is — so it can call the unexported `buildDeps` directly and assert
  `deps.Manifest.Name`.

  ⚠️ **NON-overlap with the parallel task P1.M2.T1.S1.** That task edits `runPipeline` (stagecoach.go:401)
  + `generate.go CommitStaged` for model/reasoning and EXPLICITLY leaves buildDeps to this task ("provider
  →manifest selection stays in buildDeps (P1.M2.T2.S1)"). The two share only the file stagecoach.go, in
  DIFFERENT functions (buildDeps L316 vs runPipeline L401) → no textual merge conflict. Together they
  deliver full FR-R3 on the single path: provider (this task) + model/reasoning (sibling).

  Deliverable: ONE code edit (buildDeps L324) + TWO white-box tests in pkg/stagecoach/stagecoach_test.go
  (message-provider override selects the override manifest; no-override regression selects the global).
  NO new files, NO new types, NO new imports (`config` already imported), NO go.mod change, NO docs.
  OUTPUT: `--message-provider` / `[role.message] provider` now selects the correct manifest on BOTH the
  common (CommitStaged) and advanced (runPipeline/DryRun) single-commit paths (both consume the deps
  buildDeps builds). Combined with P1.M2.T1.S1, all three message-role fields (provider/model/reasoning)
  are honored on the single-commit path.
---

## Goal

**Feature Goal**: Make `buildDeps` honor the `message` role's resolved provider when selecting the
provider manifest, so `--message-provider X` / `[role.message] provider = "X"` selects manifest X on the
single-commit path — with zero behavior change when no message override is set (the common case resolves
to `cfg.Provider`, byte-identical to today).

**Deliverable** (edits to existing files; no new files):
1. **`pkg/stagecoach/stagecoach.go`** `buildDeps` (L324): replace `name := cfg.Provider` with
   `msgProvider, _, _ := config.ResolveRoleModel("message", cfg); name := msgProvider` (+ an FR-R3/back-
   compat comment). The `if name == ""` auto-detect block, `reg.Get`, `Validate`, the `IsInstalled`
   pre-flight, and the Output/StripCodeFence bridge are UNCHANGED (they now operate on the message-
   resolved name).
2. **`pkg/stagecoach/stagecoach_test.go`** (white-box `package stagecoach`): two tests —
   `TestBuildDeps_MessageProviderOverride` (global "alpha" + `Roles["message"]={Provider:"beta"}` →
   `deps.Manifest.Name == "beta"`) and `TestBuildDeps_NoMessageOverride_InheritsGlobal` (`Roles` nil →
   `deps.Manifest.Name == "alpha"`). Both use `stubtest.Build(t)`-backed providers via `cfg.Providers` so
   the `IsInstalled` pre-flight passes.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l` clean; `go test -race ./...` green
with the two new tests passing and ALL existing tests unchanged (the no-override path is byte-identical).
Concretely: with `cfg.Provider="alpha"` and `cfg.Roles["message"]={Provider:"beta"}`, `buildDeps` returns
a Deps whose `Manifest.Name == "beta"`; with `cfg.Roles` empty, `Manifest.Name == "alpha"` (the global).
go.mod/go.sum unchanged; `config` is already imported (no new import).

## User Persona

**Target User**: Any user who sets the message role's provider on the single-commit (default) path —
`stagecoach --message-provider claude`, `[role.message] provider = "claude"` in config, or the
`STAGECOACH_MESSAGE_PROVIDER` env var. Transitively PRD §9.15 FR-R3 ("every role exposes all three flags,
including message").

**Use Case**: `stagecoach --message-provider claude` (with something staged) must generate the commit
using the CLAUDE manifest. Today it silently uses the global/manifest-default provider.

**User Journey**: flag/env/file → `Load()` writes `cfg.Roles["message"]` → `GenerateCommit` → `buildDeps`
resolves `ResolveRoleModel("message", cfg)` → `reg.Get(provider)` selects the message manifest →
`deps.Manifest` → `CommitStaged`/`runPipeline` Render+Execute. (Today buildDeps skips the resolve, so the
override is dropped.)

**Pain Points Addressed**: removes "I set `--message-provider` and a different agent was used" (FR-R3
violation) without disturbing the common no-override path.

## Why

- **Fixes a Major FR-R3 violation (the provider half).** The flag/env/config keys are documented and
  wired into `cfg.Roles["message"]`; only `buildDeps` read the global instead of resolving. Minimal fix.
- **Parity with the decompose path.** The multi-commit path resolves roles via `decompose.ResolveRoles`
  (its message role works); the single-commit path's `buildDeps` was the lone holdout for the provider.
  This makes the two paths consistent. Combined with the sibling task (model/reasoning), the single path
  reaches full FR-R3 parity.
- **Back-compatible.** With no message override, `ResolveRoleModel("message", cfg)` returns `cfg.Provider`
  — identical to today. Only explicit message-provider overrides (the FR-R3 use case) change behavior.
- **Single manifest-selection site.** buildDeps is the ONLY place the single-commit path picks a manifest
  (called solely from `GenerateCommit:136`, feeding both CommitStaged and runPipeline). Fixing it once
  fixes both sub-paths.
- **No new surface.** `--message-provider` is already documented (docs/cli.md, root.go); this makes it
  truthful. DOCS: none.

## What

`buildDeps` resolves the `message` role's provider (via `config.ResolveRoleModel`) before selecting the
manifest from the registry. Model/reasoning are NOT consumed here (Render call sites own those — sibling
task). The auto-detect block, Validate, IsInstalled pre-flight, and Output/StripCodeFence bridge are
unchanged. No config-layer, render, manifest, registry, CLI-routing, or doc changes.

### Success Criteria

- [ ] `buildDeps` L324: `msgProvider, _, _ := config.ResolveRoleModel("message", cfg); name := msgProvider`
      (replaces `name := cfg.Provider`), with a comment citing FR-R3 + back-compat + "model/reasoning at
      Render sites".
- [ ] The `if name == ""` auto-detect block (L325) is UNCHANGED (still fires when provider resolves to "").
- [ ] model/reasoning returns are discarded with `_` (buildDeps selects only the manifest; no unused-var).
- [ ] `TestBuildDeps_MessageProviderOverride`: `cfg.Provider="alpha"` + `cfg.Roles["message"]={Provider:
      "beta"}` → `buildDeps` returns `deps.Manifest.Name == "beta"`.
- [ ] `TestBuildDeps_NoMessageOverride_InheritsGlobal`: `cfg.Provider="alpha"`, `cfg.Roles` nil →
      `deps.Manifest.Name == "alpha"` (regression-safe back-compat).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l`, `go test -race ./...` clean/green; go.mod/go.sum
      unchanged; no new import (`config` already imported); only the listed files changed.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact one-line edit
(quoted with surrounding context), the `ResolveRoleModel` provider-return semantics (3 cases), the
back-compat proof, the stub-backed-provider test approach (with a concrete code sketch), and the scope
fences. No provider/registry internals beyond "ResolveRoleModel returns (provider, model, reasoning);
provider ⇒ reg.Get(name) selects the manifest."

### Documentation & References

```yaml
# MUST READ — the authoritative research (exact edit + test design + scope fences)
- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/P1M2T2S1/research/builddeps-message-provider.md
  why: the exact one-line fix with surrounding context (§1), the back-compat truth table (§2), the
       buildDeps-is-the-single-site proof (§3), the sibling-task non-overlap (§4), the test design with a
       concrete code sketch (§5), scope fences (§6), validation commands (§7).
  critical: §5 — the test MUST use stub-backed providers (cfg.Providers with command=<stub binary abs
       path>), NOT literal "pi"/"claude" (IsInstalled rejects absent binaries). And it is WHITE-BOX
       (package stagecoach) so it can call unexported buildDeps.

# The function being fixed
- file: pkg/stagecoach/stagecoach.go
  section: buildDeps (L316; the `name := cfg.Provider` line is L324; auto-detect block L325).
  why: the single edit site. buildDeps selects the manifest; everything after `name := …` (reg.Get,
       Validate, IsInstalled pre-flight, Output/StripCodeFence bridge) operates on `name` unchanged.
  pattern: replace ONLY the `name := cfg.Provider` initializer; keep the `if name == ""` block.
  gotcha: do NOT remove the auto-detect block — ResolveRoleModel returns "" when both message+global are
       empty, and that block handles it exactly as today.

# The resolver you call
- file: internal/config/roles.go
  section: ResolveRoleModel (L41).
  why: `func ResolveRoleModel(role string, cfg Config) (provider, model, reasoning string)`. Per-field
       precedence: [role.<role>] > [defaults] global > shipped default. No message override ⇒
       (cfg.Provider, cfg.Model, cfg.Reasoning). It does NOT consult any manifest; provider="" ⇒ registry
       auto-detect (buildDeps's existing block).
  gotcha: it reads cfg.Roles[role] safely on a nil map (returns zero/false) — so cfg.Roles=nil is fine.

# The contract — what the parallel sibling task does (NON-overlapping)
- file: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/P1M2T1S1/PRP.md
  why: the sibling P1.M2.T1.S1 fixes the MODEL+REASONING half at the Render call sites (generate.go
       CommitStaged + stagecoach.go runPipeline). It uses `_, msgModel, msgReasoning := …` (discards
       provider) and EXPLICITLY leaves manifest selection to buildDeps (this task).
  critical: do NOT duplicate the sibling's Render-site edits. This task edits ONLY buildDeps. The two
       share stagecoach.go in DIFFERENT functions (buildDeps vs runPipeline) → no merge conflict.

# The test harness + white-box test pattern
- file: pkg/stagecoach/stagecoach_test.go
  section: `package stagecoach` (WHITE-BOX, L1) + setupTestRepo/stubtest.Build usage (L78-137) + the
       cfg.Providers-in-memory pattern (L788, L841) + Result.Provider == manifest-name (L221-222).
  why: confirms (a) the test file IS white-box (can call unexported buildDeps), (b) stubtest.Build(t)
       yields a real binary whose abs path passes IsInstalled, (c) cfg.Providers is the raw
       map[string]map[string]any DecodeUserOverrides reads, (d) Result.Provider == the manifest name.
  pattern: build cfg with cfg.Providers = {"alpha":{command:bin,…}, "beta":{command:bin,…}}; call
       buildDeps(cfg, t.TempDir()); assert deps.Manifest.Name.

- file: internal/stubtest/stubtest.go   (stubtest.Build)
  why: stubtest.Build(t) compiles cmd/stubagent to a temp abs path — the `command` value that makes
       reg.IsInstalled (exec.LookPath) succeed for the test providers.

- file: PRD.md (bug report) §9.15 FR-R3 — "Every role exposes all three flags, including message."
  why: the requirement this fixes. The flag/env/file wiring already populates cfg.Roles["message"]; only
       buildDeps was reading the global.
```

### Current Codebase tree (relevant slice)

```bash
pkg/stagecoach/stagecoach.go          # buildDeps (L316; `name :=` at L324) — EDIT (one line + comment)
pkg/stagecoach/stagecoach_test.go     # package stagecoach (WHITE-BOX) — EDIT (add 2 buildDeps tests)
internal/config/roles.go            # ResolveRoleModel (3-return) — INPUT, NO edit
internal/stubtest/stubtest.go       # stubtest.Build (real binary) — INPUT, NO edit
internal/generate/generate.go       # CommitStaged (Render) — sibling task, NOT this task
internal/provider/{registry,manifest,render}.go — INPUT, NO edit
go.mod / go.sum                     # unchanged (config already imported; no new dep)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. One code edit (stagecoach.go buildDeps) + two tests added to stagecoach_test.go.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: replace ONLY the `name := cfg.Provider` initializer (L324). The `if name == ""` auto-detect
// block (L325) MUST stay — ResolveRoleModel returns "" when both message+global providers are empty, and
// that block runs the registry DefaultProvider cascade exactly as today. Do NOT restructure buildDeps.

// CRITICAL: discard model+reasoning with `_`. buildDeps selects ONLY the manifest (provider); model/
// reasoning are resolved at the Render call sites (sibling task). `msgProv, msgModel, msgReasoning := …`
// with unused msgModel/msgReasoning is a Go "declared and not used" compile error. Use
// `msgProvider, _, _ := config.ResolveRoleModel("message", cfg)`.

// CRITICAL (test): buildDeps has an IsInstalled pre-flight (exec.LookPath). It REJECTS providers whose
// command is not on $PATH. The literal built-ins "pi"/"claude" are NOT installed in CI → buildDeps
// errors. Register STUB-BACKED providers ("alpha"/"beta") via cfg.Providers with command=<stubtest.Build
// abs path> so IsInstalled passes. (exec.LookPath on an absolute path to a real executable succeeds.)

// CRITICAL (test): the test is WHITE-BOX. stagecoach_test.go is `package stagecoach` (not _test), so it can
// call the unexported buildDeps directly and read deps.Manifest.Name. Add the tests there.

// GOTCHA: buildDeps does NOT run any git command (it only constructs generate.Deps{Git: git.New(repoDir)});
// repoDir = t.TempDir() is fine (no git repo needed for the buildDeps call). git ops happen later in
// CommitStaged/runPipeline, which the direct buildDeps test does not reach.

// GOTCHA: ResolveRoleModel("message", cfg) with NO message override returns (cfg.Provider, cfg.Model,
// cfg.Reasoning) — so `name := msgProvider` == the old `name := cfg.Provider` (byte-identical). Do NOT
// special-case "no override"; the resolution IS the back-compatible fallback.

// GOTCHA: do NOT wire model/reasoning into buildDeps (that overlaps the sibling task's Render edits).
// buildDeps selects the manifest; Render (on that manifest) gets model/reasoning. Keep it split.

// GOTCHA: `config` is ALREADY imported in stagecoach.go (resolveConfig uses config.Load/config.Config).
// Do NOT add an import. (Adding an unused one fails `go vet`.)
```

## Implementation Blueprint

### Data models and structure

No new types. The change is one initializer line + its comment. `config.ResolveRoleModel` and
`generate.Deps` already exist.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT pkg/stagecoach/stagecoach.go buildDeps — resolve the message provider
  - AT L324, REPLACE:
        name := cfg.Provider
    WITH:
        // FR-R3: the single-commit path's active role is "message" — resolve its provider per-field
        // (flag > env > [role.message] > global [defaults]) so --message-provider / [role.message]
        // select the manifest. No message override ⇒ cfg.Provider (back-compatible). Model/reasoning
        // are resolved at the Render call sites (generate.CommitStaged / runPipeline); buildDeps
        // selects only the provider/manifest.
        msgProvider, _, _ := config.ResolveRoleModel("message", cfg)
        name := msgProvider
  - DO NOT touch the `if name == ""` auto-detect block (L325-334), reg.Get, Validate, the IsInstalled
    pre-flight, or the Output/StripCodeFence bridge — they now operate on the message-resolved `name`.
  - GOTCHA: use `_` for model+reasoning. Do NOT add an import (config already imported).

Task 2: ADD two white-box tests to pkg/stagecoach/stagecoach_test.go
  - Both use stubtest.Build(t) (real binary) + cfg.Providers (raw map) with two stub-backed providers.
  - TEST TestBuildDeps_MessageProviderOverride:
        bin := stubtest.Build(t)
        cfg := config.Defaults()
        cfg.Providers = map[string]map[string]any{
            "alpha": {"command": bin, "prompt_delivery": "stdin", "print_flag": "-p", "output": "raw", "strip_code_fence": true},
            "beta":  {"command": bin, "prompt_delivery": "stdin", "print_flag": "-p", "output": "raw", "strip_code_fence": true},
        }
        cfg.Provider = "alpha"
        cfg.Roles = map[string]config.RoleConfig{"message": {Provider: "beta"}} // --message-provider beta
        deps, err := buildDeps(cfg, t.TempDir())
        if err != nil { t.Fatalf("buildDeps: %v", err) }
        if deps.Manifest.Name != "beta" {
            t.Errorf("message provider override lost: deps.Manifest.Name = %q, want %q", deps.Manifest.Name, "beta")
        }
  - TEST TestBuildDeps_NoMessageOverride_InheritsGlobal:
        (same cfg.Providers alpha/beta + cfg.Provider="alpha"; cfg.Roles = nil — no override)
        deps, err := buildDeps(cfg, t.TempDir())
        if deps.Manifest.Name != "alpha" {
            t.Errorf("no-override regression: deps.Manifest.Name = %q, want %q (global)", deps.Manifest.Name, "alpha")
        }
  - WHY WHITE-BOX: buildDeps is unexported; stagecoach_test.go is `package stagecoach` (can call it).
  - WHY STUB-BACKED: buildDeps's IsInstalled pre-flight rejects non-PATH commands; the stub binary's abs
    path passes. (Mirrors the existing setupTestRepo/stubtest.Build harness at stagecoach_test.go:84-137.)
  - GOTCHA: repoDir = t.TempDir() (no git ops inside buildDeps). Validate-passing provider maps (Name from
    the key + Command=bin + valid enums).

Task 3: VERIFY (no further edits)
  - RUN the Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. No import added. The no-override
    path is byte-identical → ALL existing tests stay green (no regression).
```

### Implementation Patterns & Key Details

```go
// THE edit (the entire code change) — one initializer + comment:
msgProvider, _, _ := config.ResolveRoleModel("message", cfg) // FR-R3: honor [role.message] provider
name := msgProvider                                            // (was: name := cfg.Provider)
// ... the existing `if name == "" { auto-detect }` block is UNCHANGED and still fires when provider == "".

// THE back-compat invariant (no message override ⇒ byte-identical):
//   ResolveRoleModel("message", cfg) returns cfg.Provider when cfg.Roles has no message.Provider
//   ⇒ name == cfg.Provider ⇒ identical manifest selection to today. Only an explicit
//   --message-provider / [role.message] provider override changes the selected manifest.

// THE per-field inheritance (FR-R3/FR37a) — a message role that sets ONLY its model still inherits the
// global provider:
//   cfg.Roles["message"] = {Model: "haiku"} (Provider "") + cfg.Provider="alpha"
//   ⇒ ResolveRoleModel returns provider="alpha" ⇒ buildDeps selects "alpha" (correct: only model overridden).
```

```go
// stagecoach_test.go — the override test (the precise TDD proof for THIS task):
func TestBuildDeps_MessageProviderOverride(t *testing.T) {
	bin := stubtest.Build(t) // real stub binary; abs path ⇒ IsInstalled passes
	cfg := config.Defaults()
	cfg.Providers = map[string]map[string]any{
		"alpha": {"command": bin, "prompt_delivery": "stdin", "print_flag": "-p", "output": "raw", "strip_code_fence": true},
		"beta":  {"command": bin, "prompt_delivery": "stdin", "print_flag": "-p", "output": "raw", "strip_code_fence": true},
	}
	cfg.Provider = "alpha"
	cfg.Roles = map[string]config.RoleConfig{"message": {Provider: "beta"}} // simulates --message-provider beta
	deps, err := buildDeps(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("buildDeps: %v", err)
	}
	if deps.Manifest.Name != "beta" {
		t.Errorf("message provider override lost: Manifest.Name = %q, want %q", deps.Manifest.Name, "beta")
	}
}
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — no new dep. `config` already imported; go mod tidy MUST be a no-op.

PACKAGE EDGES: NONE added/removed. No new import.

FROZEN / NOT-EDITED:
  - internal/config/* (loaders/flags already populate cfg.Roles["message"] correctly — the bug is NOT
    there; ResolveRoleModel is consumed, not edited).
  - internal/provider/{registry,manifest,render}.go (consumed unchanged).
  - internal/generate/generate.go + pkg/stagecoach/stagecoach.go runPipeline (the sibling P1.M2.T1.S1 —
    model/reasoning at Render sites; NON-overlapping with buildDeps).
  - internal/decompose/* (multi-commit path uses ResolveRoles, not buildDeps; unaffected).
  - internal/cmd/default_action.go, README.md, docs/cli.md (the flag is already documented/wired).

DOWNSTREAM / RELATED (do NOT implement here):
  - P1.M2.T1.S1 (sibling, in flight): resolve message MODEL+REASONING at the Render call sites. After
    BOTH tasks land, the single-commit path honors all three message-role fields (provider/model/reasoning).
  - The stale 6-provider error list at buildDeps L~337 (strings.Join of ["pi","claude",...]) is a
    pre-existing cosmetic staleness — out of scope; leave it.

NO NEW DATABASE / ROUTES / CLI COMMANDS / FILES / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
gofmt -w pkg/stagecoach/stagecoach.go pkg/stagecoach/stagecoach_test.go
test -z "$(gofmt -l pkg/stagecoach/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./...        # Catches a forgotten `_` on model/reasoning returns (unused-var) or a stray import.
go build ./...      # Whole module compiles.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm `config` is already imported (no new import line added by this task):
grep -n '"github.com/dustin/stagecoach/internal/config"' pkg/stagecoach/stagecoach.go
```

### Level 2: Unit Tests (Component Validation)

```bash
# The two new white-box tests (override + no-override regression):
go test -race ./pkg/stagecoach/ -v -run 'BuildDeps|MessageProvider'
# Expected: PASS — TestBuildDeps_MessageProviderOverride (Manifest.Name=="beta"),
#   TestBuildDeps_NoMessageOverride_InheritsGlobal (Manifest.Name=="alpha").

# Full suite (the no-override path is byte-identical → no regression anywhere):
go test -race ./...
# Expected: all PASS. If a non-stagecoach package breaks, it is unrelated (this task edits only buildDeps).
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# Confirm only the listed files changed:
git diff --name-only | grep -Ev 'pkg/stagecoach/stagecoach\.go|pkg/stagecoach/stagecoach_test\.go' \
  && echo "UNEXPECTED file changed" || echo "only listed files changed (good)"
# Confirm the edit is exactly the one initializer (no auto-detect block removed):
grep -n 'ResolveRoleModel("message", cfg)\|name := msgProvider\|if name == ""' pkg/stagecoach/stagecoach.go
```

### Level 4: Creative & Domain-Specific Validation

```bash
# OPTIONAL public-API complement (proves the end-to-end routing through GenerateCommit): a DryRun test
# asserting Result.Provider == the message-override provider. Reuse setupTestRepo to register the stub,
# inject a second provider via cfg.Providers, set Options message-provider, GenerateCommit(DryRun:true),
# assert Result.Provider. (Result.Provider == manifest name — see stagecoach_test.go:221-222.) The direct
# buildDeps test in Task 2 is the precise proof; this is belt-and-suspenders. golangci-lint: `make lint`.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l pkg/stagecoach/`, `go mod tidy` no-op;
      no new import (config already imported); go.mod/go.sum byte-unchanged.
- [ ] Level 2 green: `go test -race ./...` (two new tests + no regressions).
- [ ] Level 3: binary builds; go.mod/go.sum unchanged; only stagecoach.go + stagecoach_test.go changed.

### Feature Validation

- [ ] `buildDeps` resolves `ResolveRoleModel("message", cfg)` for the provider; the `if name == ""` block
      is unchanged; model/reasoning discarded with `_`.
- [ ] Override test: `Roles["message"]={Provider:"beta"}` + global "alpha" → `Manifest.Name == "beta"`.
- [ ] No-override test: `Roles` nil + global "alpha" → `Manifest.Name == "alpha"` (back-compatible).
- [ ] buildDeps's manifest selection now feeds BOTH CommitStaged and runPipeline (single call site at
      GenerateCommit:136).

### Code Quality Validation

- [ ] One-line surgical edit + comment; mirrors how buildDeps already works (selects manifest; Render
      fills model/reasoning).
- [ ] No scope creep into the sibling task (Render model/reasoning), the config layer, or the decompose path.
- [ ] Tests follow the existing white-box + stubtest.Build harness convention.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] No docs edits (DOCS: none — `--message-provider` was already documented; this makes it truthful).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ Don't remove or restructure the `if name == ""` auto-detect block (L325). `ResolveRoleModel` returns
  "" when both message and global providers are empty; that block runs the registry cascade exactly as
  today. Replace ONLY the `name := cfg.Provider` initializer.
- ❌ Don't consume model/reasoning in buildDeps (`msgProvider, msgModel, msgReasoning := …` with unused
  msgModel/msgReasoning is a Go compile error). Use `msgProvider, _, _ := …`. Model/reasoning belong to
  the Render call sites (sibling task) — buildDeps selects only the manifest.
- ❌ Don't special-case "no message override". `ResolveRoleModel("message", cfg)` ALREADY returns
  `cfg.Provider` in that case — that IS the back-compatible fallback. An `if cfg.Roles["message"]…` branch
  would duplicate the resolution and risk divergence.
- ❌ Don't use the literal "pi"/"claude" built-ins in the test. buildDeps's `IsInstalled` pre-flight
  (exec.LookPath) rejects providers not on $PATH; those binaries are absent in CI. Register stub-backed
  providers via `cfg.Providers` with `command = <stubtest.Build abs path>`.
- ❌ Don't edit runPipeline / generate.go CommitStaged — that is the sibling P1.M2.T1.S1 (model/reasoning
  at Render). This task edits ONLY buildDeps. (The two share stagecoach.go in different functions → no
  conflict, but stay in your lane.)
- ❌ Don't add an import — `config` is already imported in stagecoach.go. An unused import fails `go vet`.
- ❌ Don't change go.mod/go.sum. This is a one-line in-place bugfix + tests; no new dependency.
- ❌ Don't wire buildDeps into the decompose path or touch ResolveRoles. buildDeps is single-commit-only;
  the multi-commit path resolves roles separately (and already works).
- ❌ Don't edit the stale 6-provider error list at buildDeps L~337 — it's pre-existing cosmetic staleness,
  out of scope for this provider-routing fix.
- ❌ Don't skip `go vet`/`go build`/`go test -race ./...` — they catch the unused-`_` mistake, a stray
  import, and any regression in the (byte-identical) no-override path.

---

## Confidence Score

**9/10** — A precisely-scoped one-line bugfix with a clear back-compat proof, a non-overlapping boundary
with the parallel sibling task (different function; sibling explicitly defers manifest selection here),
and a deterministic white-box test design (stub-backed providers to clear the IsInstalled pre-flight).
Every edit site and test is pinned to file:line with concrete code. The -1 reserves for the optional
public-API (GenerateCommit DryRun → Result.Provider) complement, which depends on confirming Result.Provider's
source (the direct buildDeps test is the primary proof regardless).
