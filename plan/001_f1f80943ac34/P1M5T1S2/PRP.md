---
name: "P1.M5.T1.S2 — Real-agent integration test scaffold (§20.1 layer 4): //go:build integration_real, STAGECOACH_RUN_REAL=1, drives all 6 real provider manifests through CommitStaged"
description: |

  THIS IS A TEST-ONLY TASK. Deliver ONE new file: `internal/generate/realagent_test.go`
  (package generate, `//go:build integration_real`). It is the PRD §20.1 layer-4 "Integration — real
  agents (opt-in, not in CI)" suite: a table-driven `TestRealAgents` that, ONLY when built with
  `-tags integration_real` AND `STAGECOACH_RUN_REAL=1` is set, resolves each of the 6 SHIPPED builtin
  provider manifests (pi/claude/gemini/opencode/codex/cursor) via the registry, skips any whose binary
  is not on `$PATH`, then runs `generate.CommitStaged` (P1.M3.T4.S2, COMPLETE) against a temp git repo
  with a staged change using a REAL agent subprocess, and asserts a non-empty commit message was
  produced AND a commit was created (HEAD moved, message round-trips). NOT run in CI.

  CONTRACT (P1.M5.T1.S2, verbatim):
    1. RESEARCH: "PRD §20.1 layer 4 — a //go:build integration_real suite that invokes the actual
       pi/claude/gemini/etc. if installed and STAGECOACH_RUN_REAL=1. Used manually before releases;
       skipped in CI. All 6 agents are installed on this machine (see external_deps.md)."
    2. INPUT: "CommitStaged (P1.M3.T4.S2). Real agent binaries on $PATH."
    3. LOGIC: "Create test files with //go:build integration_real that, when STAGECOACH_RUN_REAL=1 is
       set, run CommitStaged with each real provider manifest against a temp repo with staged changes,
       asserting a non-empty commit message is produced and a commit is created. Skip gracefully if the
       agent isn't installed. These are manual release-gate tests, NOT CI. Mock: no mocks — uses real
       agents; guarded by build tag + env var."
    4. OUTPUT: "A real-agent test suite for manual pre-release verification of all 6 manifests
       (resolves the TO CONFIRM items in external_deps.md)."
    5. DOCS: "none — test infrastructure."

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `internal/generate/generate.go` — CommitStaged orchestrator (P1.M3.T4.S2, Complete). READ ONLY.
    - `internal/provider/*` — manifests (P1.M2.T2), registry (P1.M2.T3), render (P1.M2.T4),
      executor (P1.M2.T5), parse (P1.M2.T6). READ ONLY (the suite CONSUMES these via the registry).
    - `internal/git/*` — git wrapper (P1.M1.T2/T3). READ ONLY.
    - `internal/generate/generate_test.go` — declares the reusable helpers (initRepo/writeFile/...).
      READ ONLY; REUSE its helpers, do NOT redeclare.
    - `internal/generate/invariants_test.go` (S1, present) — the §20.2 stub-based invariant suite.
      READ ONLY; coexists (different helper + test-function names). The new file is COMPLEMENTARY:
      S1 = stub invariants (CI), S2 = real-agent smoke (manual).

  DELIVERABLE (ONE new file, ~120–180 LOC):
    CREATE internal/generate/realagent_test.go   (package generate, //go:build integration_real)
      - File head: `//go:build integration_real` + BLANK LINE + `package generate` (mirror
        internal/provider/procgroup_unix.go's `//go:build !windows` style).
      - `TestRealAgents` (table-driven): one subtest per provider in registry order
        [pi, claude, gemini, opencode, codex, cursor].
      - THREE gates, all `t.Skip` (never t.Fatal): build tag (implicit), `STAGECOACH_RUN_REAL=1` env,
        and per-subtest `reg.IsInstalled(m)`.
      - `realConfig(name)` helper: env-overridable model+provider per provider (§"Implementation
        Patterns"); `Timeout` from `config.Defaults()` (120s/attempt — ample).
      - `logResolvedCommand(t, deps, cfg)` helper: renders the spec, logs `spec.Command` +
        truncated `spec.Args` so the operator can SEE the exact real invocation (resolves the
        external_deps.md TO CONFIRM items visually).
      - REUSES `initRepo`/`writeFile`/`stageFile`/`commitRaw`/`headSHA`/`gitOut`/`shaRe` from
        generate_test.go (same package — DO NOT redeclare). Helper names DISTINCT from S1's
        (`snapshotRepo`/`treeSHAFromErr`/`assertInvariants`) so the two files coexist under the tag.

  SUCCESS: `STAGECOACH_RUN_REAL=0 go test -tags integration_real ./internal/generate/ -run TestRealAgents
  -v` skips cleanly (env gate); `STAGECOACH_RUN_REAL=1 ... -timeout 30m` runs the installed subset
  green (manual); `make test` (NO tag) excludes the file and stays green; `go vet -tags integration_real
  ./internal/generate/` clean; `gofmt -l internal/generate/` empty; `git status` shows ONLY the new
  file. NO production-code changes. NO new go.mod deps. Resolves the external_deps.md TO CONFIRM items.

---

## Goal

**Feature Goal**: Give Stagecoach a PRD §20.1 layer-4 "Integration — real agents (opt-in, not in CI)"
suite: a single, auditable, table-driven `TestRealAgents` that proves EVERY one of the 6 SHIPPED
builtin provider manifests (pi/claude/gemini/opencode/codex/cursor) can be driven end-to-end through
`generate.CommitStaged` using the ACTUAL agent binaries — producing a non-empty commit message and
landing a real commit — as a manual pre-release gate. This is the ONLY place the real-agent surface is
exercised in the repo, and it resolves the two `// TO CONFIRM (integration)` notes in
`internal/provider/builtin.go` (codex `exec` stdout; cursor `--mode ask` read-only).

**Deliverable**: ONE new Go test file — `internal/generate/realagent_test.go` (`package generate`,
`//go:build integration_real`) — containing a table-driven `TestRealAgents` (one subtest per provider)
that: resolves each builtin manifest via `provider.NewRegistry(nil)`, skips on three gates (build tag,
`STAGECOACH_RUN_REAL=1`, binary-on-`$PATH`), drives `generate.CommitStaged` with a REAL agent subprocess
against a temp git repo holding a staged change, logs the resolved command, and asserts a commit was
created (HEAD moved, message round-trips, valid SHA, non-empty `Result.Changes`).

**Success Definition**:
- The file is EXCLUDED from `make test` / `make coverage` / CI by the build tag (they pass no `-tags`).
- With `-tags integration_real` but `STAGECOACH_RUN_REAL != "1"`, `TestRealAgents` skips cleanly with a
  "set STAGECOACH_RUN_REAL=1" message (no real agents run, no API cost).
- With `-tags integration_real` and `STAGECOACH_RUN_REAL=1`, each subtest whose binary is on `$PATH`
  runs the real agent and passes; each missing binary skips gracefully (`<name> (<bin>) not on $PATH`).
- A passing `TestRealAgents/codex` RESOLVES the codex `// TO CONFIRM` (exec writes stdout, exits 0).
- A passing `TestRealAgents/cursor` RESOLVES the cursor `// TO CONFIRM` (`--mode ask --trust` is a
  read-only one-shot that yields a valid message).
- No production-code edits; `git status` shows ONLY `internal/generate/realagent_test.go`.
- `go vet -tags integration_real ./internal/generate/` clean; `gofmt -l internal/generate/` empty.

## User Persona

**Target User**: the Stagecoach maintainer / release engineer (PRD §20). This is test infrastructure,
not a user-facing feature.

**Use Case**: before tagging a release, run
`STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestRealAgents -v -timeout 30m`
on a machine where all 6 agents are installed and authenticated. A green run is a release gate that
every shipped manifest actually works against the real CLI it wraps. A failing subtest flags either a
regression in a manifest OR an environment issue (model unavailable) — the logged resolved command lets
the operator tell which.

**Pain Points Addressed**: the 6 manifests were verified against `--help` only (external_deps.md); the
two `// TO CONFIRM` items (codex stdout, cursor read-only) were never exercised end-to-end. This suite
is the executable form of that verification — it catches manifest regressions (wrong flag, changed
output channel) before they ship to users.

## Why

- **Realizes PRD §20.1 layer 4 as an executable gate.** Layer 4 ("real agents, opt-in, not in CI") is
  otherwise unimplemented; this is the ONLY suite that exercises the real subprocess path.
- **Resolves the two external_deps.md TO CONFIRM items.** `builtinCodex` and `builtinCursor` carry
  `// TO CONFIRM (integration):` notes that explicitly defer confirmation to "the real-agent scaffold
  (P1.M5.T1.S2)". A passing codex/cursor subtest closes them.
- **Catches manifest regressions cheaply.** A future edit to a builtin manifest (e.g. a flag rename in
  a new claude version) would silently break the shipped default; this suite catches it pre-release.
- **Complements S1 without overlap.** S1 (`invariants_test.go`) proves repo SAFETY via the stub
  (CI-runnable). S2 proves provider CORRECTNESS via real agents (manual). Zero overlap; both ship.
- **Avoids scope creep.** No production code, no new deps, no CI/Makefile change (the build tag +
  `make test`'s lack of `-tags` already gives full CI exclusion). CommitStaged + the registry already
  exist and are COMPLETE.

## What

A new `internal/generate/realagent_test.go` (`package generate`, `//go:build integration_real`) with:

1. **A `realDefaults` map** of best-effort model+provider per provider (env-overridable):
   `pi`→{model "", provider "zai"}; `claude`/`gemini`→{model "", provider ""} (manifest defaults);
   `opencode`→{model "anthropic/claude-sonnet-4", provider ""} (manifest default is ""); `codex`/`cursor`
   →{model "", provider ""} (agent-config-driven). Sourced from `architecture/external_deps.md`.
2. **An `envOr(key, def)` helper** reading `STAGECOACH_REAL_MODEL_<NAME>` / `STAGECOACH_REAL_PROVIDER_<NAME>`.
3. **A `realConfig(name)` helper** returning `config.Defaults()` with `Model`/`Provider` set from the
   env/map (so pi gets `--provider zai`, opencode gets `-m anthropic/claude-sonnet-4`, etc.).
4. **A `logResolvedCommand(t, name, m, cfg)` helper** that `m.Render(...)`s the spec and `t.Logf`s the
   command + args (payload truncated) — the operator's audit trail, resolves TO CONFIRM visually.
5. **`TestRealAgents`** — gates on `STAGECOACH_RUN_REAL==1`; builds a registry; iterates the 6 names;
   per name: `Get` the manifest, `Skip` if `!IsInstalled`, seed a temp repo (born + initial commit +
   staged real-ish file), `logResolvedCommand`, call `CommitStaged` with `Deps{Git: git.New(repo),
   Manifest: m}`, assert the commit (non-empty message, valid SHA, HEAD moved, round-trip, non-empty
   Changes), and `t.Logf` the produced message.

**Provider order** (subtests): `[pi, claude, gemini, opencode, codex, cursor]` — registry preference
order (registry.go `preferredBuiltins`), deterministic.

### Success Criteria

- [ ] `internal/generate/realagent_test.go` exists; FIRST line `//go:build integration_real`, blank line,
      then `package generate`; compiles under `-tags integration_real` only.
- [ ] `TestRealAgents` has a subtest for EACH of the 6 providers (named `TestRealAgents/<name>`).
- [ ] All three gates skip via `t.Skip` (never `t.Fatal`): env != "1"; `!reg.IsInstalled(m)`.
- [ ] Each run subtest asserts: `err==nil`, `res.Message != ""`, `shaRe.MatchString(res.CommitSHA)`,
      `headSHA(repo)==res.CommitSHA`, `gitOut log %B == res.Message`, `len(res.Changes)>0`.
- [ ] `logResolvedCommand` emits the resolved argv for every subtest (audit trail).
- [ ] `make test` (no tag) stays green and does NOT run any real agent (file excluded).
- [ ] No production-code edits; `git status` shows ONLY the new test file; no new go.mod deps.

## All Needed Context

### Context Completeness Check

_Pass._ A Go test author with no prior knowledge of this repo can implement this from: the exact
CommitStaged + Deps signature (§"Implementation Patterns"), the registry Get/IsInstalled/DetectCommand
seam, the per-provider model/provider default map + WHY (external_deps.md), the reusable
`generate_test.go` helpers (so they are NOT redeclared), the build-tag + double-gate convention, and
the deterministic assertions on the commit (not the message). The TO CONFIRM resolution is a side
effect of codex/cursor passing.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M5T1S2/research/findings.md
  why: THE decisive doc. §1 CommitStaged seam; §2 the 6 manifests' model/provider defaults;
       §3 Render's model/provider fallback (the headline gotcha — pi needs provider=zai, opencode needs
       a model); §4 the env-overridable default map; §5 registry Get/IsInstalled; §6 the two TO CONFIRM
       items this resolves; §7 reusable helpers (reuse, don't redeclare) + S1 coexistence; §8 build-tag
       + CI-exclusion proof; §9 deterministic assertions; §10 validation commands; §11 risks.
  critical: §3 (model/provider fallback), §4 (default map), §8 (double gate), §7 (reuse + distinct names).

- docfile: plan/001_f1f80943ac34/architecture/external_deps.md
  why: AUTHORITATIVE record of the 6 agent CLIs as verified on this machine (2026-06-29). §pi confirms
       `--provider zai --model glm-5-turbo`; §opencode rendered example `-m anthropic/claude-sonnet-4`;
       §codex TO CONFIRM (exec → stdout) + BONUS (stdin via "-"); §cursor TO CONFIRM (--mode ask
       read-only). Source of the realDefaults map values + the items this suite resolves.

- file: internal/generate/generate.go   (P1.M3.T4.S2 — READ only; the orchestrator under test)
  section: CommitStaged — the contract the suite drives. It calls deps.Manifest.Render(cfg.Model,
       cfg.Provider, sysPrompt, payload) → provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose) →
       provider.ParseOutput(out, deps.Manifest) → CommitTree → UpdateRefCAS → DiffTree. Passing a REAL
       provider.Manifest (not stubtest.Manifest) is what makes this a real-agent run. Result fields:
       CommitSHA, Subject, Message, Provider, Model, Changes.
  why: confirms CommitStaged is EXPORTED and takes Deps{Git, Manifest, Verbose} — pass a real manifest
       from the registry and git.New(repo); Verbose can be nil (the receiver is nil-safe).
  pattern: the suite mirrors generate_test.go's TestCommitStaged_Success fixture (initRepo →
       commitRaw("initial") → writeFile + stageFile → CommitStaged → assert HEAD==CommitSHA + log round-trip).
  gotcha: a BORN repo (CommitCount<=1) uses the fallback system prompt (§17.2) — fine for a smoke test.
       Dedupe is vacuous (only subject "initial" to avoid) → single attempt → no retry storms.

- file: internal/provider/builtin.go   (P1.M2.T2 — READ only; the 6 manifests under test + TO CONFIRM)
  section: BuiltinManifests() (the map) + builtinCodex/builtinCursor carry the literal
       `// TO CONFIRM (integration):` comments naming THIS task ("verify during the real-agent scaffold
       (P1.M5.T1.S2)"). builtinCursor: Detect="agent" (≠ Name "cursor") — the ONLY such provider.
  why: this suite is the closure of those TODOs. A passing codex/cursor subtest = confirmation.
  gotcha: opencode/codex/cursor have DefaultModel="" → the test MUST supply a model for opencode (and
       may rely on agent config for codex/cursor). See findings §3/§4.

- file: internal/provider/registry.go   (P1.M2.T3 — READ only; how to Get a manifest + detect install)
  section: NewRegistry(nil) (pure built-ins, no user-config noise), Get(name), IsInstalled(m) probes
       m.DetectCommand() via exec.LookPath (correctly resolves cursor → "agent").
  why: NewRegistry(nil) is the RIGHT seam — it exercises the SHIPPED manifests exactly as the CLI
       resolves them. IsInstalled is the graceful-skip mechanism (skip per-subtest, never fatal).
  pattern: `reg := provider.NewRegistry(nil); m, ok := reg.Get(name); if !reg.IsInstalled(m) { t.Skip }`.

- file: internal/provider/render.go   (P1.M2.T4 — READ only; the model/provider fallback logic)
  section: Render's `modelToUse := model; if "" { = *r.DefaultModel }` + `providerToUse` likewise; then
       `if *r.ProviderFlag != "" && providerToUse != ""` / `if *r.ModelFlag != "" && modelToUse != ""`.
  why: explains WHY pi needs cfg.Provider="zai" (manifest default_provider="" → no --provider → google
       provider → glm-5-turbo fails) and WHY opencode needs cfg.Model (manifest default_model="").
       This is the headline gotcha; the realDefaults map encodes the fix.

- file: internal/generate/generate_test.go   (P1.M3.T4.S2 — READ only; SOURCE OF REUSABLE HELPERS)
  section: the unexported helpers at the TOP (package generate, internal test): initRepo, writeFile,
       stageFile, commitRaw (--allow-empty), headSHA, gitOut, runGit, shaRe (`^[0-9a-f]{7,64}$`).
  why: your new file is ALSO `package generate` → call these DIRECTLY. DO NOT redeclare any
       (compile error: redeclared in this block). Mine TestCommitStaged_Success for the fixture pattern.
  gotcha: also reuse `shaRe` for the CommitSHA assertion (already declared — do NOT redeclare).

- file: internal/provider/procgroup_unix.go   (P1.M2.T5.S2 — READ only; the build-tag STYLE reference)
  section: line 1 `//go:build !windows`, line 2 BLANK, line 3 `package provider`.
  why: the EXACT format your file head must mirror, substituting `integration_real`. The blank line
       after the constraint is REQUIRED (Go parses the build constraint only with the blank line).

- file: internal/generate/invariants_test.go   (P1.M5.T1.S1 — READ only; the S1 suite, present)
  section: declares snapshotRepo/treeSHAFromErr/assertInvariants/repoSnapshot (its OWN helpers) and
       TestInvariants; reuses generate_test.go's helpers.
  why: COEXISTENCE — your helper names MUST differ (realConfig/envOr/logResolvedCommand) so both files
       compile together under `-tags integration_real`. Confirmed: no existing func is TestRealAgents.

- file: Makefile   (READ only; CI-exclusion proof)
  section: `test:` → `go test -race ./...` and `coverage:` → `go test -coverprofile=... ./...`.
       Neither passes `-tags` → your integration_real file is EXCLUDED from both. CI-excluded by design.

- url: (PRD internal) PRD.md §20.1 layer 4 ("Integration — real agents (opt-in, not in CI). A
       //go:build integration_real suite that invokes the actual pi/claude/etc. if installed and
       STAGECOACH_RUN_REAL=1. Used manually before releases; skipped in CI.").
  why: AUTHORITATIVE spec for WHAT this suite is. Verbatim contract.
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  generate.go                 # P1.M3.T4.S2 — CommitStaged orchestrator (READ only; under test)
  generate_test.go            # P1.M3.T4.S2 — package generate; HAS reusable helpers + Success fixture
  invariants_test.go          # P1.M5.T1.S1 — package generate; stub invariant suite (present; coexists)
  realagent_test.go           # ← NEW (this task): //go:build integration_real; TestRealAgents
internal/provider/
  builtin.go                  # P1.M2.T2 — the 6 manifests + the TO CONFIRM comments (READ only)
  registry.go                 # P1.M2.T3 — NewRegistry/Get/IsInstalled (READ only)
  render.go                   # P1.M2.T4 — Render model/provider fallback (READ only)
  procgroup_unix.go           # P1.M2.T5.S2 — the //go:build STYLE reference (READ only)
internal/git/git.go           # P1.M1.T2/T3 — git.New (READ only)
Makefile                      # test/coverage (NO -tags) → CI exclusion (UNCHANGED)
go.mod                        # module github.com/dustin/stagecoach (UNCHANGED — no new deps)
```

### Desired Codebase tree with files to be added

```bash
internal/generate/realagent_test.go   # NEW — package generate; //go:build integration_real; TestRealAgents.
# ALL other files UNCHANGED. No production-code edits, no go.mod changes, no new packages, no Makefile change.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (BUILD TAG FORMAT): the file's FIRST line MUST be `//go:build integration_real` (Go 1.17+
// syntax), followed by a BLANK line, then `package generate`. Mirror procgroup_unix.go's
// `//go:build !windows` exactly. Without the blank line, `go` may not parse the constraint. With it,
// `go test ./...` (no -tags) EXCLUDES the file → CI-safe; `go test -tags integration_real ./...` includes it.

// CRITICAL (DOUBLE GATE — never run in CI, never run accidentally): the build tag alone is not enough.
// Also gate at runtime: `if os.Getenv("STAGECOACH_RUN_REAL") != "1" { t.Skip("...set STAGECOACH_RUN_REAL=1...") }`.
// A maintainer running `go test -tags integration_real ./...` WITHOUT the env var would otherwise spawn
// 6 slow, network-bound, API-cost-incurring real agents. The env gate is defense-in-depth (PRD §20.1:
// "if installed and STAGECOACH_RUN_REAL=1"). Skip via t.Skip — NEVER t.Fatal for a gate.

// CRITICAL (MODEL/PROVIDER IS ENVIRONMENT-SPECIFIC): Render uses cfg.Model, falling back to the
// manifest DefaultModel; cfg.Provider falls back to DefaultProvider. The manifests do NOT encode a
// working model for every env:
//   - pi:    default_provider="" → no --provider → pi uses google → glm-5-turbo (a zai model) FAILS.
//            → the test MUST set cfg.Provider="zai" (commit-pi convention; external_deps §pi).
//   - opencode: default_model="" → no -m → opencode may reject. → set cfg.Model="anthropic/claude-sonnet-4".
//   - codex/cursor: default_model="" → no model flag → agent uses its OWN config (config.toml / per-account).
//            → leave cfg.Model="" (correct; agent-config-driven).
// Fix = the env-overridable realDefaults map. If a model isn't available in this env, the operator sets
// STAGECOACH_REAL_MODEL_<NAME>. A failing subtest with the logged command distinguishes "manifest wrong"
// from "model unavailable" — that's WHY logResolvedCommand exists.

// CRITICAL (cursor: detect ≠ name): cursor's binary is `agent` (Detect="agent", ≠ Name "cursor"). The
// registry's IsInstalled probes m.DetectCommand() → "agent" → correct. Do NOT exec.LookPath("cursor").

// CRITICAL (REUSE, DON'T REDECLARE): generate_test.go is `package generate` (internal test) and
// declares initRepo, writeFile, stageFile, commitRaw, headSHA, gitOut, runGit, shaRe. Your new file is
// ALSO `package generate` → call them DIRECTLY. Re-declaring any → "redeclared in this block" compile
// error. ALSO: S1 (invariants_test.go) is present under the SAME tag — its helpers are snapshotRepo/
// treeSHAFromErr/assertInvariants/repoSnapshot. Name your OWN helpers DIFFERENTLY (realConfig, envOr,
// logResolvedCommand) so both files compile together under -tags integration_real.

// GOTCHA (NO t.Parallel): real agents are subprocess-heavy, network-bound, and may share state/auth.
// Run subtests SERIALLY (no t.Parallel()) to avoid resource contention and flaky auth-rate-limiting.
// (Matches S1 + generate_test.go, which also run serially.)

// GOTCHA (REAL AGENTS ARE SLOW + COSTLY): use Go's -timeout (e.g. 30m) at the COMMAND level — distinct
// from cfg.Timeout (120s/attempt, the per-attempt generation kill). A born repo with one "initial"
// commit takes the fallback prompt path (CommitCount<=1) and dedupe is vacuous → typically ONE attempt,
// so cfg.Timeout=120s is ample. Document the manual run command with -timeout 30m.

// GOTCHA (ASSERT THE COMMIT, NOT THE WORDS): the agent's message is nondeterministic. Assert on the
// COMMIT: err==nil, res.Message != "", shaRe.MatchString(res.CommitSHA), headSHA(repo)==res.CommitSHA,
// gitOut(repo,"log","--format=%B","-n1",res.CommitSHA)==res.Message, len(res.Changes)>0. Do NOT assert
// on subject wording/type.

// GOTCHA (NO t.Fatal ON A GATE; t.Fatal ON A RUN): gates (env, install) → t.Skip. A RUN that errors
// (err != nil) → t.Fatalf with the error + resolved command — that is a real regression/release-blocker.
```

## Implementation Blueprint

### Data models and structure

No production data models. The test file declares small test-only values:

```go
//go:build integration_real

package generate

// realDefault is the best-effort model+provider for one provider's real run (env-overridable).
// `model==""` means "let Render fall back to the manifest DefaultModel, or emit no model flag
// (agent-config-driven for codex/cursor)". `provider==""` means "no --provider flag".
type realDefault struct {
	model, provider string
}

// realDefaults — sourced from architecture/external_deps.md (2026-06-29 verification).
// Override per-run via STAGECOACH_REAL_MODEL_<NAME> / STAGECOACH_REAL_PROVIDER_<NAME>.
var realDefaults = map[string]realDefault{
	"pi":       {"", "zai"},                    // glm-5-turbo from manifest default; provider=zai (commit-pi)
	"claude":   {"", ""},                       // sonnet from manifest default
	"gemini":   {"", ""},                       // gemini-2.5-pro from manifest default
	"opencode": {"anthropic/claude-sonnet-4", ""}, // manifest default is "" → MUST supply a model
	"codex":    {"", ""},                       // model from ~/.codex/config.toml
	"cursor":   {"", ""},                       // per-account default model
}

// providerNames — registry preference order (registry.go preferredBuiltins); deterministic subtest order.
var providerNames = []string{"pi", "claude", "gemini", "opencode", "codex", "cursor"}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY the seams exist and the baseline is green (READ + RUN, no edit)
  - RUN: `go test -race ./internal/generate/ -v` → green (proves CommitStaged + helpers + stub present;
      this is the baseline you must not regress).
  - READ: internal/generate/generate.go — confirm `func CommitStaged(ctx, Deps, cfg) (Result, error)`
      is EXPORTED, Deps{Git, Manifest, Verbose}, Result{CommitSHA,Subject,Message,Provider,Model,Changes}.
  - READ: internal/generate/generate_test.go TOP — confirm the reusable helpers (initRepo, writeFile,
      stageFile, commitRaw, headSHA, gitOut, runGit, shaRe) are `package generate` (internal test).
  - READ: internal/provider/registry.go — confirm NewRegistry(nil)/Get(name)/IsInstalled(m).
  - READ: internal/provider/procgroup_unix.go line 1-3 — confirm the build-tag FORMAT (//go:build + blank + package).
  - READ: internal/generate/invariants_test.go — confirm S1's helper NAMES (snapshotRepo, etc.) so you
      pick distinct names. Confirm no existing func is TestRealAgents.
  - GOTCHA: if generate_test.go is `package generate_test` (external) or shaRe is missing, STOP and
      report. (It IS `package generate` and shaRe IS present — verified.)

Task 2: CREATE internal/generate/realagent_test.go — file head + helpers (the config + logging layer)
  - FILE: CREATE internal/generate/realagent_test.go. FIRST line `//go:build integration_real`, BLANK
      line, then `package generate` + a file doc comment naming PRD §20.1 layer 4 + the TO CONFIRM items.
  - IMPORTS: context, fmt, os, strings, testing, time (stdlib) + config, git, provider (existing
      internal pkgs already used by generate_test.go). NO new go.mod deps.
  - IMPLEMENT:
      func envOr(key, def string) string                              // os.Getenv(key) or def if ""
      func realConfig(name string) config.Config                      // config.Defaults() + env/map model+provider
      func logResolvedCommand(t *testing.T, name string, m provider.Manifest, cfg config.Config)
  - realConfig body:
      cfg := config.Defaults()            // Timeout=120s/attempt, MaxDuplicateRetries=3
      d := realDefaults[name]
      cfg.Model    = envOr("STAGECOACH_REAL_MODEL_"    + strings.ToUpper(name), d.model)
      cfg.Provider = envOr("STAGECOACH_REAL_PROVIDER_" + strings.ToUpper(name), d.provider)
      return cfg
  - logResolvedCommand body:
      t.Helper()
      spec, err := m.Render(cfg.Model, cfg.Provider, "<system prompt>", "<staged diff>")
      if err != nil { t.Logf("[%s] render error: %v", name, err); return }
      args := strings.Join(spec.Args, " ")
      if len(args) > 200 { args = args[:200] + " …(truncated)" }
      t.Logf("[%s] resolved command: %s %s   (stdin=%t)", name, spec.Command, args, spec.Stdin != "")
  - WHY: logResolvedCommand is the operator's audit trail — it makes the TO CONFIRM items visible
      (you SEE `-p --mode ask --trust` for cursor; `exec --sandbox read-only --ephemeral` for codex).
  - GOTCHA: t.Helper() at the top of each helper so failures/​logs point at the CALLING subtest.
      Do NOT name any helper the same as a generate_test.go OR invariants_test.go helper.

Task 3: ASSEMBLE TestRealAgents — the three gates + the table + the run + the assertions
  - FILE: append `func TestRealAgents(t *testing.T)`.
  - GATE 1 (env): `if os.Getenv("STAGECOACH_RUN_REAL") != "1" { t.Skip("skipping real-agent suite; set STAGECOACH_RUN_REAL=1 and build with -tags integration_real") }`
  - BODY:
      reg := provider.NewRegistry(nil)   // pure built-ins — no user-config noise
      for _, name := range providerNames {
          name := name
          t.Run(name, func(t *testing.T) {
              m, ok := reg.Get(name)
              if !ok { t.Fatalf("registry has no builtin %q (keep providerNames in sync with BuiltinManifests)", name) }
              if !reg.IsInstalled(m) { t.Skipf("%s (%s) not on $PATH", name, m.DetectCommand()) }  // GATE 2 (install)
              // Fixture (mirror TestCommitStaged_Success): born repo + initial commit + staged file.
              repo := t.TempDir(); initRepo(t, repo); commitRaw(t, repo, "initial")
              writeFile(t, repo, "main.go", "package main\n\nfunc main() { println(\"hello\") }\n")
              stageFile(t, repo, "main.go")
              cfg := realConfig(name)
              logResolvedCommand(t, name, m, cfg)
              // RUN: real agent via CommitStaged. Timeout per attempt = cfg.Timeout (120s).
              res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
              if err != nil { t.Fatalf("real agent %s failed end-to-end: %v\n(resolved cmd logged above)", name, err) }
              // ASSERT the commit (NOT the message words — nondeterministic).
              if res.Message == ""           { t.Errorf("res.Message is empty") }
              if !shaRe.MatchString(res.CommitSHA) { t.Errorf("CommitSHA = %q, want hex SHA", res.CommitSHA) }
              if got := headSHA(t, repo); got != res.CommitSHA { t.Errorf("HEAD = %q, want %q (commit did not land)", got, res.CommitSHA) }
              if got := gitOut(t, repo, "log", "--format=%B", "-n1", res.CommitSHA); got != res.Message {
                  t.Errorf("git log message = %q, want %q (message did not round-trip)", got, res.Message)
              }
              if len(res.Changes) == 0 { t.Errorf("res.Changes empty, want the staged file reported by DiffTree") }
              t.Logf("[%s] OK — commit %s: %q", name, res.CommitSHA[:min(7,len(res.CommitSHA))], res.Message)
          })
      }
  - WHY: one auditable table; adding a 7th manifest = one providerNames entry + one realDefaults row.
  - GOTCHA: NO t.Parallel() (real agents are subprocess/network/auth-heavy — run serially). The
      `name := name` capture is optional under go.mod's go1.22+ loop semantics but harmless; add it.
  - NOTE on min(): Go 1.21+ has a builtin `min`; go.mod toolchain is 1.26 → use it. (If you prefer,
      drop the `[:7]` truncation and log the full SHA — either is fine.)

Task 4: FINAL VALIDATION (the gate — does NOT require real agents to pass)
  - RUN: `go test -tags integration_real ./internal/generate/` → COMPILES (proves the file + tag are
      well-formed; runs but every subtest skips on the env gate → PASS).
  - RUN: `STAGECOACH_RUN_REAL=0 go test -tags integration_real ./internal/generate/ -run TestRealAgents -v`
      → every subtest SKIPS with the env message (proves gate 1; no real agents run).
  - RUN: `gofmt -w internal/generate/realagent_test.go`; `gofmt -l internal/generate/` (empty).
  - RUN: `go vet -tags integration_real ./internal/generate/` (clean).
  - RUN: `make test` → green AND no real agent ran (file excluded — no -tags). This is the CI-exclusion proof.
  - RUN: `git status --short` → ONLY `?? internal/generate/realagent_test.go`.
  - (OPTIONAL, manual, requires agents+network) RUN:
      `STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestRealAgents -v -timeout 30m`
      → installed subset passes; missing binaries skip. A green codex+cursor RESOLVES the TO CONFIRM items.
```

### Implementation Patterns & Key Details

```go
//go:build integration_real

// Package generate test: the PRD §20.1 layer-4 "Integration — real agents (opt-in, not in CI)" suite.
// Built ONLY under -tags integration_real; runs ONLY when STAGECOACH_RUN_REAL=1. NOT in CI
// (make test / make coverage pass no -tags). Drives generate.CommitStaged against each of the 6 real
// builtin provider manifests (pi/claude/gemini/opencode/codex/cursor). Resolves the two
// `// TO CONFIRM (integration)` notes in internal/provider/builtin.go (codex exec→stdout; cursor --mode ask).
package generate

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/provider"
)

type realDefault struct{ model, provider string }

// realDefaults — best-effort per-provider model+provider (env-overridable). "" ⇒ fall back to the
// manifest DefaultModel / emit no flag. Sourced from architecture/external_deps.md.
var realDefaults = map[string]realDefault{
	"pi":       {"", "zai"},
	"claude":   {"", ""},
	"gemini":   {"", ""},
	"opencode": {"anthropic/claude-sonnet-4", ""},
	"codex":    {"", ""},
	"cursor":   {"", ""},
}

var providerNames = []string{"pi", "claude", "gemini", "opencode", "codex", "cursor"}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// realConfig builds a config tuned for one provider's real run. Model+provider come from the
// env-overridable realDefaults map; Timeout/MaxDuplicateRetries inherit config.Defaults() (120s/3).
func realConfig(name string) config.Config {
	cfg := config.Defaults()
	d := realDefaults[name]
	cfg.Model = envOr("STAGECOACH_REAL_MODEL_"+strings.ToUpper(name), d.model)
	cfg.Provider = envOr("STAGECOACH_REAL_PROVIDER_"+strings.ToUpper(name), d.provider)
	return cfg
}

// logResolvedCommand renders the manifest to its concrete argv and logs it (payload truncated) so the
// operator can audit the EXACT real invocation — this is what makes the external_deps.md TO CONFIRM
// items (codex exec flags, cursor --mode ask) visually verifiable.
func logResolvedCommand(t *testing.T, name string, m provider.Manifest, cfg config.Config) {
	t.Helper()
	spec, err := m.Render(cfg.Model, cfg.Provider, "<system prompt>", "<staged diff>")
	if err != nil {
		t.Logf("[%s] render error (manifest may be invalid): %v", name, err)
		return
	}
	args := strings.Join(spec.Args, " ")
	if len(args) > 200 {
		args = args[:200] + " …(truncated)"
	}
	t.Logf("[%s] resolved command: %s %s   (stdin=%t)", name, spec.Command, args, spec.Stdin != "")
}

// TestRealAgents drives each real builtin provider manifest through CommitStaged end-to-end. Opt-in:
// build tag (integration_real) + STAGECOACH_RUN_REAL=1 + binary on $PATH. NOT in CI.
func TestRealAgents(t *testing.T) {
	if os.Getenv("STAGECOACH_RUN_REAL") != "1" {
		t.Skip("skipping real-agent suite; set STAGECOACH_RUN_REAL=1 and build with -tags integration_real")
	}
	reg := provider.NewRegistry(nil)
	for _, name := range providerNames {
		name := name
		t.Run(name, func(t *testing.T) {
			m, ok := reg.Get(name)
			if !ok {
				t.Fatalf("registry has no builtin %q (keep providerNames in sync with BuiltinManifests)", name)
			}
			if !reg.IsInstalled(m) {
				t.Skipf("%s (%s) not on $PATH", name, m.DetectCommand())
			}
			repo := t.TempDir()
			initRepo(t, repo)
			commitRaw(t, repo, "initial")
			writeFile(t, repo, "main.go", "package main\n\nfunc main() { println(\"hello\") }\n")
			stageFile(t, repo, "main.go")

			cfg := realConfig(name)
			logResolvedCommand(t, name, m, cfg)

			res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
			if err != nil {
				t.Fatalf("real agent %s failed end-to-end: %v\n(resolved command logged above — distinguish manifest bug vs unavailable model)", name, err)
			}
			// Assert the COMMIT, not the message words (the agent's text is nondeterministic).
			if res.Message == "" {
				t.Errorf("res.Message is empty; agent produced no parseable commit message")
			}
			if !shaRe.MatchString(res.CommitSHA) {
				t.Errorf("CommitSHA = %q, want a hex SHA", res.CommitSHA)
			}
			if got := headSHA(t, repo); got != res.CommitSHA {
				t.Errorf("HEAD = %q, want %q (commit did not land on HEAD)", got, res.CommitSHA)
			}
			if got := gitOut(t, repo, "log", "--format=%B", "-n1", res.CommitSHA); got != res.Message {
				t.Errorf("git log message = %q, want %q (message did not round-trip into the commit)", got, res.Message)
			}
			if len(res.Changes) == 0 {
				t.Errorf("res.Changes is empty; DiffTree reported no landed file")
			}
			short := res.CommitSHA
			if len(short) > 7 {
				short = short[:7]
			}
			t.Logf("[%s] OK — committed %s: %q", name, short, res.Message)
		})
	}
}

// (Avoid an unused-import error if a refactor drops a helper's use of fmt/time; the file above imports
//  only what it uses: context, fmt (via t.Fatalf %v is fine without fmt — REMOVE fmt import if unused),
//  os, strings, testing, config, git, provider. Trim unused imports before gofmt.)
// NOTE: the snippet above does not reference `fmt` or `time` directly — if you keep the body as shown,
//       OMIT `fmt` and `time` from the import block (gofmt/vet will flag unused imports). Only import
//       what each helper actually references.
```

### Integration Points

```yaml
TEST SUITE (PRD §20.1 layer 4):
  - the new internal/generate/realagent_test.go is picked up ONLY under -tags integration_real. `make
    test` (= go test -race ./...) and `make coverage` pass NO -tags → the file is EXCLUDED → zero real
    agents run in CI. No Makefile change; no CI-config change (P1.M5.T3 owns the GitHub Actions matrix
    and must NOT add -tags integration_real to the test step).

PRODUCTION CODE (frozen — read-only dependency):
  - the suite drives generate.CommitStaged (P1.M3.T4.S2) and CONSUMES provider.NewRegistry/Get/
    IsInstalled (P1.M2.T3), provider.Manifest.Render (P1.M2.T4), git.New (P1.M1.T2), config.Defaults
    (P1.M1.T4). It modifies NONE of them. A failing RUN subtest = either a manifest regression (real
    bug — file a work item) or an environment issue (model unavailable — set the env override).

COVERAGE (PRD §20.3 ≥85%): this suite is BUILD-TAGGED OUT of the default build, so it contributes ZERO
  to the coverage gate (it is not compiled under go test ./...). It neither helps nor hurts the gate.

PARALLEL COORDINATION (P1.M5.T1.S1 — invariants_test.go, present):
  - S1 is package generate, NO build tag, CI-runnable, drives the STUB. S2 is package generate,
    //go:build integration_real, manual, drives REAL agents. They share the generate_test.go helpers
    and have DISTINCT helper + test-function names → they coexist cleanly under -tags integration_real.
    Zero overlap: S1 = repo-safety (stub, CI); S2 = provider-correctness (real, manual).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After creating the file — the file ONLY compiles under the tag, so vet/compile WITH -tags.
go test -tags integration_real ./internal/generate/        # COMPILES + runs (all subtests skip on env gate)
go vet -tags integration_real ./internal/generate/         # clean
gofmt -w internal/generate/realagent_test.go
gofmt -l internal/generate/                                 # must be empty

# Expected: zero errors. If go vet reports "redeclared in this block" → you redeclared a
# generate_test.go OR invariants_test.go helper; rename yours. If it reports an unused import → trim it.
```

### Level 2: The Gates (Component Validation — NO real agents run)

```bash
# Gate 1: env not set → every subtest SKIPS cleanly (no real agent spawned, no API cost).
# (Run from a shell where STAGECOACH_RUN_REAL is unset:)
unset STAGECOACH_RUN_REAL
go test -tags integration_real ./internal/generate/ -run TestRealAgents -v
# Expected: --- SKIP: TestRealAgents ("skipping real-agent suite; set STAGECOACH_RUN_REAL=1 …")
# OR, if env is explicitly 0:
STAGECOACH_RUN_REAL=0 go test -tags integration_real ./internal/generate/ -run TestRealAgents -v
# Expected: same SKIP.

# Gate 2 (install): with the env set, subtests for MISSING binaries SKIP; installed ones RUN. On a
# machine with none of the agents, ALL subtests SKIP with "<name> (<bin>) not on $PATH":
STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestRealAgents -v
# Expected (no agents): one SKIP per provider ("<name> (<bin>) not on $PATH"). No failures.
```

### Level 3: The Real Manual Gate (System Validation — requires agents + network + API)

```bash
# THE release gate. Requires all 6 agents installed + authenticated (external_deps.md: all present).
# -timeout 30m: Go's test timeout (real agents are slow); distinct from cfg.Timeout=120s per attempt.
STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestRealAgents -v -timeout 30m
# Expected: --- PASS for each installed agent; the t.Logf lines show the resolved command + produced message.
#   - A PASSING TestRealAgents/codex  → RESOLVES the codex  TO CONFIRM (exec → stdout, exit 0).
#   - A PASSING TestRealAgents/cursor → RESOLVES the cursor TO CONFIRM (--mode ask --trust read-only one-shot).
# If a subtest FAILS: read the logged "resolved command" → is the flag wrong (manifest bug) or is the
#   model unavailable (env: set STAGECOACH_REAL_MODEL_<NAME>)? Distinguish before filing a bug.

# Per-provider model override examples (if a default isn't available in this env):
STAGECOACH_RUN_REAL=1 STAGECOACH_REAL_MODEL_OPENCODE=openai/gpt-4o \
    go test -tags integration_real ./internal/generate/ -run TestRealAgents/opencode -v -timeout 10m
```

### Level 4: CI-Exclusion Proof & No-Regression (confidence, no real agents)

```bash
# CI-exclusion proof: make test passes NO -tags → the integration_real file is EXCLUDED → no real run.
make test            # == go test -race ./...  → green; no real agent spawned.
make coverage        # == go test -coverprofile=... ./... → the tagged file is NOT compiled → no coverage effect.

# No-regression: the generate package (shared helpers + S1 + existing tests) must stay green.
go test -race ./internal/generate/ -v      # green; TestRealAgents is ABSENT (file excluded without the tag).

# Scope audit: ONLY the new file changed.
git status --short                          # → ?? internal/generate/realagent_test.go  (nothing else)

# Build sanity (the tagged test file compiles into a test binary):
go vet -tags integration_real ./...         # clean (proves no package-wide break under the tag)
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l internal/generate/` empty; `go vet -tags integration_real ./internal/generate/` clean.
- [ ] Level 1: `go test -tags integration_real ./internal/generate/` COMPILES (file + tag well-formed).
- [ ] Level 2: env unset → `TestRealAgents` SKIPS (no real agent run, no API cost).
- [ ] Level 2: `STAGECOACH_RUN_REAL=1` with no agents → every subtest SKIPS ("not on $PATH"), no failures.
- [ ] Level 3 (manual): `STAGECOACH_RUN_REAL=1 … -timeout 30m` → installed subset PASS; codex+cursor
      green RESOLVE the TO CONFIRM items.
- [ ] Level 4: `make test` green AND no real agent ran (file excluded); `git status` shows ONLY the new file.
- [ ] No new `go.mod` dependencies (stdlib + existing internal imports only).

### Feature Validation

- [ ] File head: `//go:build integration_real` + blank line + `package generate`.
- [ ] `TestRealAgents` has a subtest for EACH of the 6 providers (`pi/claude/gemini/opencode/codex/cursor`).
- [ ] Three gates all `t.Skip` (never `t.Fatal`): env `!= "1"`; `!reg.IsInstalled(m)`; (build tag is implicit).
- [ ] Each run asserts: `err==nil`, `res.Message != ""`, `shaRe.MatchString(res.CommitSHA)`,
      `headSHA(repo)==res.CommitSHA`, `gitOut log %B == res.Message`, `len(res.Changes)>0`.
- [ ] `logResolvedCommand` emits the resolved argv for every subtest (audit trail / TO CONFIRM visibility).
- [ ] Model/provider env-overridable; `pi`→provider `zai`, `opencode`→model `anthropic/claude-sonnet-4`.
- [ ] cursor handled via `reg.IsInstalled` (probes `agent`, not `cursor`).

### Code Quality Validation

- [ ] `package generate` (internal test) — REUSES initRepo/writeFile/stageFile/commitRaw/headSHA/gitOut/shaRe
      from generate_test.go; NONE redeclared.
- [ ] Helper names (`realConfig`, `envOr`, `logResolvedCommand`, `realDefault`, `realDefaults`,
      `providerNames`) DISTINCT from S1's (`snapshotRepo`/`treeSHAFromErr`/`assertInvariants`/`repoSnapshot`).
- [ ] NO `t.Parallel()` (real agents are subprocess/network/auth-heavy — run serially).
- [ ] NO production-code edits; NO new packages; NO go.mod changes; NO Makefile/CI changes.

### Documentation & Deployment

- [ ] File-level doc comment names PRD §20.1 layer 4 and the two external_deps.md TO CONFIRM items resolved.
- [ ] `realDefaults` map is documented (env-overridable; source = external_deps.md).
- [ ] The manual run command (`STAGECOACH_RUN_REAL=1 go test -tags integration_real … -timeout 30m`) is
      stated in the file doc comment so a future maintainer discovers it.
- [ ] No new env vars/config keys/CLI flags in PRODUCTION (the `STAGECOACH_REAL_*` / `STAGECOACH_RUN_REAL`
      vars are TEST-ONLY, consumed solely by this tagged file).

---

## Anti-Patterns to Avoid

- ❌ **Don't edit production code.** This is a TEST-ONLY task. generate.go / provider / git / config are
  READ-ONLY and COMPLETE. If a real run FAILS, the bug is in a manifest (or the env lacks a model) — but
  fixing a manifest is a DIFFERENT work item (and would need its own PRP). This task only writes the test.
- ❌ **Don't redeclare the generate_test.go helpers.** That file is `package generate` (internal test).
  Re-declaring `initRepo`/`headSHA`/`shaRe`/etc. → "redeclared in this block". Reuse them.
- ❌ **Don't use helper names that collide with S1 (invariants_test.go).** Under `-tags integration_real`
  BOTH files compile; duplicate `snapshotRepo`/`assertInvariants`/etc. → compile error. Pick distinct names.
- ❌ **Don't gate with `t.Fatal`.** The env gate and the install gate are OPT-IN conditions → `t.Skip`.
  (Only a RUN that errors → `t.Fatalf`, because that's a genuine release-blocker.)
- ❌ **Don't run real agents in CI.** The build tag is the CI fence; do NOT add `-tags integration_real`
  to `make test`, `make coverage`, or the P1.M5.T3 GitHub Actions test step. The env var is the second fence.
- ❌ **Don't assert on the message WORDS.** The agent's text is nondeterministic. Assert on the COMMIT
  (non-empty message, valid SHA, HEAD moved, round-trip, non-empty Changes) — those are deterministic.
- ❌ **Don't use `t.Parallel()`.** Real agents spawn subprocesses, hit the network, and may share auth /
  rate limits. Run subtests serially.
- ❌ **Don't hardcode `exec.LookPath("cursor")`.** cursor's binary is `agent` (Detect≠Name). Use
  `reg.IsInstalled(m)` (which probes `m.DetectCommand()` → "agent") — it's already correct.
- ❌ **Don't omit the model for opencode / the provider for pi and expect success.** Render falls back to
  the manifest defaults, which are "" (opencode model) / "" (pi provider → google → glm-5-turbo fails).
  Use the `realDefaults` map; it encodes the verified working config (env-overridable).
- ❌ **Don't forget the blank line after `//go:build integration_real`.** Without it Go may not parse the
  build constraint and the file could be compiled unconditionally (real agents in CI!). Mirror
  procgroup_unix.go exactly.
