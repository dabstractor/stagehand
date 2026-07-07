---
name: "P1.M1.T1.S2 — Fix all four decompose role Render calls (planner, stager, message, arbiter)"
description: |
  Bugfix-001 Issue 1 (Critical) — the DECOMPOSE half of the provider/sub-provider deconflation.
  Each of the four decompose role files (`internal/decompose/{planner,stager,message,arbiter}.go`)
  derives `(prov, mdl) := config.ResolveRoleModel("<role>", deps.Config)` and passes `prov` as the
  Render provider argument. `prov` is the manifest/agent NAME (e.g. "pi"), NOT the sub-provider — the
  same conflation S1 fixed in `generate.go` (commit 9ff53e6). Fix: change each Render call's provider
  argument from `prov` to `""` so Render falls back to the manifest's merged `DefaultProvider` (FR37a),
  and since `prov` is now unused at each call site, change `prov, mdl :=` → `_, mdl :=` at the four
  declaration lines. Add an inline comment at each Render call (Mode A). Add one synchronous,
  race-free unit test PER role (pi-shaped stub + `ui.NewVerbose(&buf, true)` asserting the buffer
  contains `--provider openrouter` and NOT `--provider pi`), plus one end-to-end test through the full
  decompose loop (2-concept partition) with a thread-safe verbose sink. render.go/merge.go/roles.go/
  config/roles.go/decompose/roles.go are UNCHANGED (already correct). No CLI binary test (that is T2).
---

## Goal

**Feature Goal**: Eliminate the provider/sub-provider conflation at all four decompose-role Render call
sites so each role (planner/stager/message/arbiter) renders the **sub-provider** (`zai`, `openrouter`, …)
resolved from the role manifest's merged `DefaultProvider` (FR37a) instead of the manifest/agent name
(`pi`). With the fix, a pi-shaped decompose config with `default_provider = "openrouter"` emits
`--provider openrouter` from every role (or omits `--provider` when `DefaultProvider` is unset — pi's
shipped default, §12.3). This closes the decompose half of Critical Issue 1 (S1 closed the single-commit
half in `generate.go`).

**Deliverable** (4 production files + their 4 test files + the shared decompose_test.go):
1. `internal/decompose/planner.go` L61 + L91: `prov, mdl :=` → `_, mdl :=`; Render's provider arg
   `prov` → `""`; inline comment.
2. `internal/decompose/stager.go` L60 + L66: same two edits + comment.
3. `internal/decompose/message.go` L102 + L122: same two edits + comment.
4. `internal/decompose/arbiter.go` L81 + L91: same two edits + comment.
5. `internal/decompose/{planner,message,arbiter}_test.go`: +1 synchronous unit test each
   (`TestCallPlanner_ResolvesSubProvider`, `TestGenerateMessage_ResolvesSubProvider`,
   `TestRunArbiter_ResolvesSubProvider`) asserting the verbose buffer.
6. `internal/decompose/stager_test.go` (create-or-extend): +1 synchronous `TestStageConcept_ResolvesSubProvider`
   (no `stagerDeps` helper exists — build `Deps` inline mirroring `plannerDeps` with `Roles: RoleManifests{Stager: m}`).
7. `internal/decompose/decompose_test.go`: +1 end-to-end test `TestDecompose_RoleResolvesSubProvider`
   (full `Decompose()` over a 2-concept partition with a thread-safe verbose sink + `dcmStagerSeam`).

**Success Definition**: All four role files pass `""` (not `prov`) to Render and discard the now-unused
`prov`; each role, with a pi-shaped manifest (`DefaultProvider="openrouter"`, `ProviderFlag="--provider"`)
and `cfg.Provider="pi"`, renders a command containing `--provider openrouter` and NOT `--provider pi`;
with no `DefaultProvider`, `--provider` is omitted (back-compatible — all existing decompose tests pass
byte-identical); `go build/vet/gofmt` clean and `go test -race ./...` green.

## User Persona

**Target User**: Every Stagecoach user who runs multi-commit decomposition (`stagecoach` with an un-staged
dirty tree → auto-decompose, or `stagecoach --commits N`) with the default provider (pi) configured —
the bootstrap config (`[defaults] provider = "pi"`), `git config stagecoach.provider pi` (PRD §15.5's
recommended setup), `--provider pi`, or `STAGECOACH_PROVIDER=pi`.

**Use Case**: A user sets `default_provider = "openrouter"` under `[provider.pi]` so pi routes to an
OpenRouter backend, then runs `stagecoach` over a multi-change working tree to produce 2–3 logical commits.

**Pain Points Addressed**: Today EVERY decompose role emits `pi --provider pi …` (an invalid
sub-provider) and silently ignores the user's `default_provider` — exactly the bug S1 fixed for the
single-commit path, still present for the planner/stager/message/arbiter. The fix makes the four
decompose roles honor the merged `DefaultProvider`, consistent with `generate.go` post-S1.

## Why

- **Critical, default-path bug — decompose half.** Issue 1 is the single Critical issue in the v2.0 QA
  pass. S1 fixed the single-commit path; S2 fixes the four-role decompose path. Together they make pi
  (the shipped default provider) actually work for BOTH code paths.
- **Defeats an already-correct layer.** FR37a's field-merge correctly preserves `default_provider`
  across config layers into each role manifest's `DefaultProvider`. That value is correct at the merge
  layer — but Render never reads it because each role caller overrides it with the manifest name via
  `ResolveRoleModel`'s `prov`. S2 lets the already-correct merge take effect for all four roles.
- **render.go is already correct.** Render's `provider == "" → *r.DefaultProvider` fallback is right;
  the bug is purely that the callers pass the wrong (non-empty) value. So the fix is a one-token change
  at each of the four call sites, not a redesign. (Confirmed in `issue1_provider_conflation.md`:
  "No changes needed to render.go / config/roles.go / provider/merge.go.")
- **`ResolveRoleModel` is already correct for its OTHER caller.** `ResolveRoles` (decompose/roles.go)
  uses `prov` for `reg.Get(prov)` (manifest lookup) — that is CORRECT and untouched. Only the Render
  callers misuse `prov`; S2 changes only the 4 Render-call files.
- **Minimal, surgical, back-compatible.** Existing tests use `config.Defaults()` (Provider="") + nil
  Verbose, so they already hit `Render(mdl, "", ...)` and are byte-identical after the fix.

## What

Four production files get the identical two-token edit (Render provider arg `prov`→`""`, declaration
`prov, mdl :=`→`_, mdl :=`) plus an inline comment; their four test files each get one new synchronous
unit test; `decompose_test.go` gets one end-to-end loop test. No changes to `render.go`,
`provider/merge.go`, `config/roles.go`, `decompose/roles.go`, the single-commit `generate.go` (S1 done),
the CLI, any `Result`/`DecomposeResult` struct, or any user-facing docs (Mode A = inline comments only).

### Success Criteria

- [ ] Each of the four role files calls `deps.Roles.<Role>.Render(mdl, "", ...)` (not `prov`).
- [ ] Each of the four declaration lines is `_, mdl := config.ResolveRoleModel("<role>", deps.Config)`.
- [ ] Each changed Render call carries an inline comment explaining `prov` is the manifest name, not the
      sub-provider; Render resolves the sub-provider from the merged `DefaultProvider`.
- [ ] Per-role unit test (planner/message/arbiter via `*Deps` helpers; stager via inline Deps + direct
      `stageConcept`): pi-shaped manifest (`DefaultProvider="openrouter"`, `ProviderFlag="--provider"`) +
      `cfg.Provider="pi"` + `ui.NewVerbose(&buf, true)` → rendered argv contains `--provider openrouter`.
- [ ] Each per-role unit test asserts the argv does NOT contain `--provider pi` (conflation signature).
- [ ] E2E test: a full `Decompose()` over a 2-concept partition with a thread-safe verbose sink emits
      `--provider openrouter` and not `--provider pi`, and passes under `-race`.
- [ ] With no `DefaultProvider` (stubtest default), `--provider` is omitted → all existing decompose
      tests pass unchanged.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] No edits to `render.go`, `merge.go`, `config/roles.go`, `decompose/roles.go`, `generate.go`, CLI,
      or any `docs/*.md` (Mode A = inline comments only).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the exact buggy Render line and declaration line in each of the
four files (with grep-confirmed line numbers), proves `prov` is used only at the Render call (so
`_, mdl :=` is correct), quotes Render's verbatim fallback that makes `""` correct, mirrors the exact
S1 comment + `""` convention, lists the real per-role test helpers + the cross-package pointer-field
trick, provides a complete per-role test body, gives a thread-safe verbose sink for the `-race`-clean
E2E test, and supplies executable validation commands. The architecture analysis
(`issue1_provider_conflation.md`) pre-resolved root cause and the no-change-needed list.

### Documentation & References

```yaml
# MUST READ — the binding issue analysis (do not re-litigate)
- docfile: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue1_provider_conflation.md
  why: "§Call Site 2-5 names the 4 decompose role files + their (prov,mdl) declarations + Render calls; §The Fix gives the exact table (Render(mdl, prov,...) → Render(mdl, \"\",...)); §What still works proves the back-compat cases (DefaultProvider set → emits it; unset → omits --provider; non-pi → ProviderFlag empty → never emits). §Test Strategy gives the caller-level unit-test + E2E approach."
  critical: "States render.go/merge.go/config-roles.go need NO changes (already correct). States ResolveRoleModel's prov is CORRECT for reg.Get in ResolveRoles — only the 4 Render callers misuse it. S2 = the 4 decompose files ONLY; S1 (generate.go) is DONE (commit 9ff53e6); the CLI E2E binary test is T2 (P1.M1.T2.S1)."

# The predecessor PRP — S1 set the convention S2 mirrors
- docfile: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/P1M1T1S1/PRP.md
  why: "S1 fixed the SAME bug in generate.go with the comment + \"\" pattern (quoted verbatim in §Implementation Patterns). S2 copies that comment style (adapted: 'ResolveRoleModel returns the manifest name') and the same test approach (pi-shaped stub + verbose-buffer argv capture + the negative assertion)."
  critical: "S1's gotchas apply verbatim to S2: the cross-package pointer-field trick (&localVar, since provider.strPtr is unexported), the verbose-capture chain (Execute→VerboseCommand→buffer), the mandatory negative assertion (!Contains \"--provider pi\"), and back-compat (existing tests use Provider=\"\" + nil Verbose → byte-identical)."

# The four production files under edit
- file: internal/decompose/planner.go
  why: "EDIT TARGET. callPlanner: L61 `prov, mdl := config.ResolveRoleModel(\"planner\", deps.Config)`; L91 `deps.Roles.Planner.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)`. prov used ONLY at L91 (grep-confirmed)."
  pattern: "L61 → `_, mdl := config.ResolveRoleModel(\"planner\", deps.Config)`; L91 → `deps.Roles.Planner.Render(mdl, \"\", sysPrompt, payload, provider.RenderBare)` + comment. Do NOT touch the retry loop, safety cap, validatePlannerOutput, plannerExamples, or any other line."
  gotcha: "Render is inside the retry loop — keep the loop and the 4th/5th args (sysPrompt, payload, provider.RenderBare) exactly. Only the 2nd positional arg (prov→\"\") changes."

- file: internal/decompose/stager.go
  why: "EDIT TARGET. stageConcept: L60 `prov, mdl := config.ResolveRoleModel(\"stager\", deps.Config)`; L66 `deps.Roles.Stager.Render(mdl, prov, \"\", task, provider.RenderTooled)`. prov used ONLY at L66."
  pattern: "L60 → `_, mdl := ...`; L66 → `deps.Roles.Stager.Render(mdl, \"\", \"\", task, provider.RenderTooled)` + comment. NOTE the TWO consecutive \"\" args: arg2 = provider (the fix), arg3 = empty system prompt (pre-existing — the task IS the payload). Do NOT collapse or reorder them."
  gotcha: "stager uses provider.RenderTooled (not RenderBare) — preserve it. freezeSnapshot is NOT edited."

- file: internal/decompose/message.go
  why: "EDIT TARGET. generateMessage: L102 `prov, mdl := config.ResolveRoleModel(\"message\", deps.Config)`; L122 `deps.Roles.Message.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)`. prov used ONLY at L122."
  pattern: "L102 → `_, mdl := ...`; L122 → `deps.Roles.Message.Render(mdl, \"\", sysPrompt, payload, provider.RenderBare)` + comment. Render is inside the generate→dedupe loop; keep the loop intact."
  gotcha: "`resolved := deps.Roles.Message.Resolve()` at L103 and `retryInstr := *resolved.RetryInstruction` at L104 stay — Render does its own Resolve internally (reads DefaultProvider from deps.Roles.Message). messageSystemPrompt/messageRecentSubjects/publishCommit unchanged."

- file: internal/decompose/arbiter.go
  why: "EDIT TARGET. runArbiter: L81 `prov, mdl := config.ResolveRoleModel(\"arbiter\", deps.Config)`; L91 `deps.Roles.Arbiter.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)`. prov used ONLY at L91."
  pattern: "L81 → `_, mdl := ...`; L91 → `deps.Roles.Arbiter.Render(mdl, \"\", sysPrompt, payload, provider.RenderBare)` + comment."
  gotcha: "runArbiter is single-shot (no retry). convertArbiterCommits/targetInRun unchanged. Keep the 'when in doubt, null' semantics."

# Cross-references (read-only — do NOT edit in S2)
- file: internal/provider/render.go
  why: "Render's provider fallback (providerToUse := provider; if \"\" → *r.DefaultProvider; if ProviderFlag != \"\" && providerToUse != \"\" → append flag+value). Confirms \"\" emits DefaultProvider or omits --provider. NOT edited."
- file: internal/config/roles.go
  why: "ResolveRoleModel(role, cfg) returns (provider, model) = manifest name (+ model). Correct for reg.Get; the decompose Render callers (S2) stop using its provider return for Render. NOT edited."
- file: internal/decompose/roles.go
  why: "ResolveRoles uses prov from ResolveRoleModel for reg.Get(prov) (CORRECT — manifest lookup). That usage is untouched by S2. setRole stores the manifest on deps.Roles.X. NOT edited."
- file: internal/stubtest/stubtest.go
  why: "Build(t) string; Manifest(bin, Options) Manifest (Options{Out,Script,Counter,Exit,SleepMS}); NewScript(t,bin,[]string) Manifest. Returned provider.Manifest has EXPORTED *string/*bool fields → set pi-shaped via &localVar. NOT edited."

# Test files under edit
- file: internal/decompose/planner_test.go
  why: "EDIT TARGET (test). Has plannerDeps(t,repo,m) (L55, builds Deps{Git,Config:Defaults,Roles:RoleManifests{Planner:m},Verbose:nil}) + helpers initRepo/commitRaw/writeFile/runGit. +1 test: TestCallPlanner_ResolvesSubProvider."
- file: internal/decompose/message_test.go
  why: "EDIT TARGET (test). Has messageDeps(t,repo,m) (L71) + the call generateMessage(ctx,deps,treeA,treeB) (needs 2 real tree SHAs — see TestGenerateMessage_* for the treeA/baseTree setup). +1 test: TestGenerateMessage_ResolvesSubProvider."
- file: internal/decompose/arbiter_test.go
  why: "EDIT TARGET (test). Has arbDeps(t,repo,m) (L66) + runArbiter(ctx,deps,commits,leftoverDiff). +1 test: TestRunArbiter_ResolvesSubProvider."
- file: internal/decompose/stager_test.go
  why: "EDIT TARGET (test). NO stagerDeps helper — build Deps inline (mirror plannerDeps: Roles:RoleManifests{Stager:m}) and call stageConcept(ctx,deps,concept) directly. The stubtest agent can't run git, but stageConcept only needs to RENDER (captured) + Execute (stub returns 0) — actual staging is irrelevant for this assertion. +1 test: TestStageConcept_ResolvesSubProvider."
- file: internal/decompose/decompose_test.go
  why: "EDIT TARGET (test, the E2E loop test). Has dcmDeps/dcmDepsWithConfig/dcmStagerSeam/dcmOutBuffer + many TestDecompose_* loop tests (TestDecompose_AutoMultiCommit_HappyPath is the 2-concept template). +1 test: TestDecompose_RoleResolvesSubProvider (full Decompose + thread-safe verbose sink + 2-concept partition)."

- docfile: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/P1M1T1S2/research/s2_decompose_role_notes.md
  why: "Distilled S2 findings: the 4 exact call sites w/ line numbers, the prov-only-at-Render grep proof (→ _,mdl :=), the S1 convention, the Render fallback, all per-role test helpers, the stubtest pointer-field trick, back-compat proof, and the -race-consideration for the E2E test (lockedWriter)."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── internal/decompose/
│   ├── planner.go        # EDIT (L61 _,mdl; L91 Render "" + comment)
│   ├── stager.go         # EDIT (L60 _,mdl; L66 Render "" + comment)
│   ├── message.go        # EDIT (L102 _,mdl; L122 Render "" + comment)
│   ├── arbiter.go        # EDIT (L81 _,mdl; L91 Render "" + comment)
│   ├── planner_test.go   # EDIT (+1 test)
│   ├── stager_test.go    # EDIT (+1 test, inline Deps)
│   ├── message_test.go   # EDIT (+1 test)
│   ├── arbiter_test.go   # EDIT (+1 test)
│   ├── decompose_test.go # EDIT (+1 E2E loop test)
│   ├── roles.go          # read-only (ResolveRoles reg.Get(prov) — CORRECT, untouched)
│   └── decompose.go      # read-only (orchestrator)
├── internal/generate/generate.go  # read-only (S1 done, commit 9ff53e6)
├── internal/provider/render.go    # read-only (fallback ALREADY CORRECT)
└── internal/config/roles.go       # read-only (ResolveRoleModel correct for reg.Get)
```

### Desired Codebase Tree After S2

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/decompose/{planner,stager,message,arbiter}.go       # Render "" + _,mdl + comment (4 files)
    internal/decompose/{planner,stager,message,arbiter}_test.go  # +1 unit test each (4 files)
    internal/decompose/decompose_test.go                          # +1 E2E loop test
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/decompose/{planner,stager,message,arbiter}.go` | MODIFY | `_, mdl :=` declaration; Render `""`; inline comment. |
| `internal/decompose/{planner,stager,message,arbiter}_test.go` | MODIFY | +1 synchronous sub-provider-resolution unit test each. |
| `internal/decompose/decompose_test.go` | MODIFY | +1 full-`Decompose` E2E test (thread-safe verbose sink). |

**Explicitly NOT touched in S2**: `internal/generate/*` (S1 done), `internal/provider/render.go` &
`merge.go` (already correct), `internal/config/roles.go` (correct for reg.Get),
`internal/decompose/roles.go` (ResolveRoles `reg.Get(prov)` is correct — untouched),
`internal/decompose/{decompose,chain}.go`, the CLI (`internal/cmd/*`), any `Result`/`DecomposeResult`
struct, `docs/*.md` (Mode A = inline comments only), `PRD.md`, `tasks.json`, `prd_snapshot.md`.

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL — prov is the manifest NAME, not the sub-provider. ResolveRoleModel returns cfg.Provider
// (the registry key, e.g. "pi") as its first value. Render's 2nd positional param IS the sub-provider
// (zai/openrouter/...). Passing prov makes Render emit "--provider pi" (invalid) and ignore the merged
// DefaultProvider. The fix passes "" so Render falls back to *r.DefaultProvider (render.go).

// CRITICAL — prov is unused after the fix → Go COMPILE ERROR ("declared but not used"). Change the
// declaration `prov, mdl :=` → `_, mdl :=` at all four sites. Do NOT leave `prov, mdl :=` and add
// `_ = prov` (works but is noisier; the contract prefers `_, mdl :=`). mdl is still used by Render.

// CRITICAL — ResolveRoleModel's prov return is STILL used (correctly) by ResolveRoles in
// internal/decompose/roles.go for reg.Get(prov). That is a DIFFERENT call site — S2 does NOT edit
// roles.go. Only the 4 Render-call files discard prov. Do not "fix" roles.go.

// CRITICAL (stager Render has TWO "" args). stager.go L66 is `Render(mdl, prov, "", task, RenderTooled)`.
// After the fix it is `Render(mdl, "", "", task, RenderTooled)`: arg2="" (provider, the FIX), arg3=""
// (empty system prompt, PRE-EXISTING — the stager task IS the payload). Do NOT collapse to one "".

// GOTCHA (cross-package pointer fields): provider.strPtr/boolPtr are UNEXPORTED, so a test in package
// `decompose` cannot call them. stubtest.Manifest's fields are EXPORTED (*string/*bool). Set pi-shaped
// fields by taking the address of a LOCAL: `pflag, dp, mflag, dm := "--provider", "openrouter",
// "--model", "x"; m.ProviderFlag = &pflag; m.DefaultProvider = &dp; m.ModelFlag = &mflag; m.DefaultModel = &dm`.
// Do NOT take & of a loop var. (Same trick S1 used.)

// GOTCHA (verbose capture): VerboseCommand joins [Command]+Args with spaces → strings.Contains(buf,
// "--provider openrouter") matches the token pair. The NEGATIVE assertion !Contains(buf, "--provider
// pi") is the direct regression guard: before the fix the buffer WOULD contain "--provider pi"
// (ResolveRoleModel returned "pi" because cfg.Provider="pi").

// GOTCHA (back-compat): existing decompose tests use cfg := config.Defaults() (Provider="") + nil
// Verbose. ResolveRoleModel returns ("","") → Render(mdl, "", ...) before AND after the fix →
// byte-identical (--provider omitted; stubtest default has no DefaultProvider). Do NOT modify them.

// GOTCHA (-race for the E2E loop test): runLoop overlaps stager[i+1] (Render→Execute→VerboseCommand)
// with message[i] in a goroutine (Render→Execute→VerboseCommand) → BOTH write deps.Verbose's sink.
// bytes.Buffer is NOT concurrency-safe → a Verbose-on loop test FAILS go test -race. The 4 per-role
// unit tests are SYNCHRONOUS (no goroutines) → race-free. For the E2E test, pass a thread-safe sink
// (lockedWriter: sync.Mutex + bytes.Buffer) to ui.NewVerbose, OR assert only on the planner (which
// renders synchronously before runLoop). Recommend lockedWriter for a faithful full-loop assertion.

// GOTCHA (message E2E tree SHAs): generateMessage(ctx, deps, treeA, treeB) needs TWO real tree SHAs.
// Existing message_test.go builds them via WriteTree after staging different file sets. For the
// per-role message unit test, mirror that setup (write+stage file → WriteTree → treeB; base tree =
// git.EmptyTreeSHA or a parent WriteTree). The stub returns a subject; the test only asserts the
// RENDERED argv, so the message content is secondary — but diff must be non-empty or generateMessage
// returns ErrMessageFailed ("empty concept diff").

// SECURITY (PRD §19): VerboseCommand logs ARGV ONLY (Command+Args), NEVER spec.Env (which carries
// *_API_KEY). Asserting on the buffer is safe — no credential leakage. (ui/verbose.go doc comment.)
```

## Implementation Blueprint

### Data models and structure

No data-model changes. `Deps`, `RoleManifests`, `config.Config`, `config.ResolveRoleModel`,
`provider.Manifest.Render`, and `ui.Verbose` are all unchanged. Relevant existing signatures (verbatim):

```go
// internal/config/roles.go (UNCHANGED)
func ResolveRoleModel(role string, cfg Config) (provider, model string)

// internal/provider/render.go (UNCHANGED — the fallback that makes "" correct)
func (m Manifest) Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode) (*CmdSpec, error)
//   providerToUse := provider; if providerToUse == "" { providerToUse = *r.DefaultProvider }

// internal/ui/verbose.go (UNCHANGED — the capture sink)
func NewVerbose(w io.Writer, on bool) *Verbose
func (v *Verbose) VerboseCommand(cmd string) // "DEBUG: command: "+cmd+"\n" when on && w!=nil

// internal/decompose/roles.go (UNCHANGED — Deps + per-role helpers)
type Deps struct { Git git.Git; Registry *provider.Registry; Config config.Config; Roles RoleManifests; Verbose *ui.Verbose; Out io.Writer; stager func(...) }
type RoleManifests struct { Planner, Stager, Message, Arbiter provider.Manifest }
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/decompose/planner.go
  - L61: `prov, mdl := config.ResolveRoleModel("planner", deps.Config)` → `_, mdl := config.ResolveRoleModel("planner", deps.Config)`
  - L91: prepend the comment (see Implementation Patterns) then change the 2nd Render arg prov → "":
        spec, rerr := deps.Roles.Planner.Render(mdl, "", sysPrompt, payload, provider.RenderBare)
  - PRESERVE: the retry loop, 4th/5th args, the safety cap, validatePlannerOutput, plannerExamples.
  - DO NOT touch any other line. VERIFY: go build ./internal/decompose/.

Task 2: EDIT internal/decompose/stager.go
  - L60: `prov, mdl := ...` → `_, mdl := config.ResolveRoleModel("stager", deps.Config)`
  - L66: comment + change arg2 prov → "" (keep arg3 "" = empty sysPrompt, RenderTooled):
        spec, rerr := deps.Roles.Stager.Render(mdl, "", "", task, provider.RenderTooled)
  - PRESERVE: freezeSnapshot, Execute, RenderTooled, the two "" args distinction.

Task 3: EDIT internal/decompose/message.go
  - L102: `prov, mdl := ...` → `_, mdl := config.ResolveRoleModel("message", deps.Config)`
  - L122: comment + arg2 prov → "":
        spec, rerr := deps.Roles.Message.Render(mdl, "", sysPrompt, payload, provider.RenderBare)
  - PRESERVE: resolved := deps.Roles.Message.Resolve() + retryInstr (L103-104), the dedupe loop, the
    *generate.RescueError returns, messageSystemPrompt/messageRecentSubjects/publishCommit.

Task 4: EDIT internal/decompose/arbiter.go
  - L81: `prov, mdl := ...` → `_, mdl := config.ResolveRoleModel("arbiter", deps.Config)`
  - L91: comment + arg2 prov → "":
        spec, rerr := deps.Roles.Arbiter.Render(mdl, "", sysPrompt, payload, provider.RenderBare)
  - PRESERVE: convertArbiterCommits, targetInRun, the single-shot Execute, "when in doubt null" semantics.

Task 5: ADD TestCallPlanner_ResolvesSubProvider (internal/decompose/planner_test.go)
  - Add imports if missing: "bytes", "github.com/dustin/stagecoach/internal/ui". (config/git/provider/stubtest already imported.)
  - Reuse plannerDeps(t, repo, m); override Verbose + cfg.Provider on the returned deps.
  - BODY:
        bin := stubtest.Build(t)
        repo := t.TempDir(); initRepo(t, repo); commitRaw(t, repo, "initial")
        writeFile(t, repo, "a.txt", "content") // UNSTAGED (callPlanner reads WorkingTreeDiff)
        m := stubtest.Manifest(bin, stubtest.Options{Out: validMultiJSON}) // 3-concept JSON (already in file)
        pflag, dp := "--provider", "openrouter"
        m.ProviderFlag, m.DefaultProvider = &pflag, &dp           // pi-shaped: merged DefaultProvider MUST be honored
        deps := plannerDeps(t, repo, m)
        deps.Config.Provider = "pi"                               // the manifest NAME — the conflation source; must NOT be emitted
        var buf bytes.Buffer
        deps.Verbose = ui.NewVerbose(&buf, true)
        out, err := callPlanner(context.Background(), deps, 0, false)
        if err != nil { t.Fatalf("callPlanner: %v", err) }
        _ = out
        cmd := buf.String()
        if !strings.Contains(cmd, "--provider openrouter") {
            t.Errorf("planner command missing --provider openrouter\ngot: %s", cmd) }
        if strings.Contains(cmd, "--provider pi") {
            t.Errorf("planner command emits manifest name as sub-provider (conflation)\ngot: %s", cmd) }
  - COVERAGE: positive (--provider openrouter) + negative (--provider pi) + success.

Task 6: ADD TestStageConcept_ResolvesSubProvider (internal/decompose/stager_test.go)
  - Add a stagerDeps-equivalent inline (NO stagerDeps helper exists): mirror plannerDeps with Stager.
  - BODY:
        bin := stubtest.Build(t)
        repo := t.TempDir(); initRepo(t, repo)
        m := stubtest.Manifest(bin, stubtest.Options{Out: ""})    // stager ignores stdout; exit 0
        pflag, dp := "--provider", "openrouter"
        m.ProviderFlag, m.DefaultProvider = &pflag, &dp
        deps := Deps{Git: git.New(repo), Config: config.Defaults(), Roles: RoleManifests{Stager: m}}
        deps.Config.Provider = "pi"
        var buf bytes.Buffer
        deps.Verbose = ui.NewVerbose(&buf, true)
        concept := prompt.PlannerCommit{Title: "feat: x", Description: "stage a.txt"}
        if err := stageConcept(context.Background(), deps, concept); err != nil {
            t.Fatalf("stageConcept: %v", err) }                   // stub returns 0; actual staging irrelevant here
        cmd := buf.String()
        if !strings.Contains(cmd, "--provider openrouter") { t.Errorf(...) }
        if strings.Contains(cmd, "--provider pi") { t.Errorf(...) }
  - Add imports: "bytes", internal/ui, internal/prompt, internal/git, internal/config (as needed).
  - GOTCHA: stageConcept's Render is RenderTooled; the assertion is identical (--provider openrouter).

Task 7: ADD TestGenerateMessage_ResolvesSubProvider (internal/decompose/message_test.go)
  - Reuse messageDeps(t, repo, m); needs 2 tree SHAs (generateMessage diffs treeA..treeB). Mirror the
    existing TestGenerateMessage_* tree setup: baseTree = git.EmptyTreeSHA (or a parent WriteTree);
    write+stage a file → WriteTree → treeB (non-empty diff so generateMessage does not short-circuit).
  - Pi-shape m (ProviderFlag/DefaultProvider via &local); deps.Config.Provider = "pi"; deps.Verbose on.
  - Stub out a valid subject (stubtest.Options{Out: "feat: msg"}).
  - Assert buf Contains "--provider openrouter" && !Contains "--provider pi".

Task 8: ADD TestRunArbiter_ResolvesSubProvider (internal/decompose/arbiter_test.go)
  - Reuse arbDeps(t, repo, m); call runArbiter(ctx, deps, commits, leftoverDiff). commits can be a
    single CommitInfo; leftoverDiff a non-empty string. Stub out valid arbiter JSON
    (`{"target": null}`) so runArbiter returns cleanly.
  - Pi-shape m; deps.Config.Provider = "pi"; deps.Verbose on.
  - Assert buf Contains "--provider openrouter" && !Contains "--provider pi".

Task 9: ADD TestDecompose_RoleResolvesSubProvider (internal/decompose/decompose_test.go) — E2E loop
  - Add a thread-safe sink type (race-safe under -race):
        type lockedBuffer struct { mu sync.Mutex; b bytes.Buffer }
        func (l *lockedBuffer) Write(p []byte) (int, error) { l.mu.Lock(); defer l.mu.Unlock(); return l.b.Write(p) }
    Add imports: "sync", "bytes", "internal/ui".
  - Build ALL FOUR roles pi-shaped (a small helper: piShape(m) sets ProviderFlag/DefaultProvider via
    &local; apply to each stubtest.Manifest). Use dcmStagerSeam(t, repo, conceptFiles) so the loop can
    actually stage (stubtest can't run git) — mirror TestDecompose_AutoMultiCommit_HappyPath.
  - cfg := config.Defaults(); cfg.Provider = "pi" (so ResolveRoleModel returns "pi"); do NOT set Single/Commits=1.
  - deps.Verbose = ui.NewVerbose(&locked, true) (the lockedBuffer).
  - Planner stub returns a 2-concept partition (valid 2-commit JSON).
  - Run Decompose(ctx, deps); assert no error; assert locked.b (snapshot via mu) Contains
    "--provider openrouter" && !Contains "--provider pi".
  - GOTCHA: the planner renders SYNCHRONOUSLY before runLoop; the stager/message render concurrently
    during runLoop — the lockedBuffer makes concurrent writes -race-safe. The planner alone suffices for
    the positive assertion, but the lockedBuffer lets you also see stager/message tokens.

Task 10: VALIDATE (run all gates; all must pass before declaring done)
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: gofmt -l .            # must be empty
  - RUN: go test -race ./internal/decompose/ -v   # 4 new unit tests + 1 E2E green; existing unchanged
  - RUN: go test -race ./...                      # full suite green
  - FIX-FORWARD: read failures, fix, re-run. Do NOT skip.
```

### Implementation Patterns & Key Details

```go
// === The comment + "" edit at each of the 4 Render calls (mirror S1's generate.go comment) ===
// planner.go L91 / message.go L122 / arbiter.go L91 (Bare):
		// Pass "" for the sub-provider: ResolveRoleModel returns the manifest/agent NAME (the registry
		// key, e.g. "pi"), NOT the upstream backend. Render resolves the real sub-provider from the
		// manifest's merged DefaultProvider (FR37a) — emitting "--provider <DefaultProvider>", or
		// omitting --provider when DefaultProvider is unset (pi's shipped default, §12.3). Same fix as
		// generate.go (P1.M1.T1.S1). The prov return of ResolveRoleModel is still used correctly by
		// ResolveRoles (reg.Get) — only this Render call stops using it.
		spec, rerr := deps.Roles.Planner.Render(mdl, "", sysPrompt, payload, provider.RenderBare)

// stager.go L66 (Tooled — note the TWO "" args):
		// Pass "" for the sub-provider (see note above); ResolveRoleModel's prov is the manifest name,
		// not the backend. The SECOND "" is the empty system prompt (pre-existing — the task IS the
		// payload). Same fix as generate.go (P1.M1.T1.S1).
		spec, rerr := deps.Roles.Stager.Render(mdl, "", "", task, provider.RenderTooled)

// === The declaration edit at each of the 4 sites (prov unused → discard) ===
	// 1. Derive the <role> model — Deps has no Models field. (Provider is the manifest name; it is NOT
	// passed to Render — Render resolves the sub-provider from the manifest's DefaultProvider.)
	_, mdl := config.ResolveRoleModel("<role>", deps.Config)
```

```go
// === The Render fallback that makes "" correct (render.go — UNCHANGED, for reference) ===
	r := m.Resolve()
	providerToUse := provider
	if providerToUse == "" { providerToUse = *r.DefaultProvider }   // fires now that we pass ""
	if *r.ProviderFlag != "" && providerToUse != "" {
		args = append(args, *r.ProviderFlag, providerToUse)          // → "--provider openrouter"
	}
```

```go
// === The thread-safe verbose sink for the -race-clean E2E loop test (Task 9) ===
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}
func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
// deps.Verbose = ui.NewVerbose(&locked, true)  → concurrent stager[i+1] ∥ message[i] writes are safe.
```

### Integration Points

```yaml
PRODUCTION (internal/decompose/{planner,stager,message,arbiter}.go):
  - declaration: `prov, mdl :=` → `_, mdl :=` (4 sites)
  - Render call arg2: prov → "" (4 sites)
  - inline comment on each Render call (Mode A)

NO-TOUCH (explicitly — render.go/merge.go already correct; ResolveRoleModel correct for reg.Get):
  - internal/provider/render.go        # provider=="" → *r.DefaultProvider fallback ALREADY correct
  - internal/provider/merge.go         # FR37a default_provider merge ALREADY correct
  - internal/config/roles.go           # ResolveRoleModel returns manifest name for reg.Get (correct)
  - internal/decompose/roles.go        # ResolveRoles reg.Get(prov) usage CORRECT — untouched
  - internal/generate/generate.go      # S1 already fixed (commit 9ff53e6)
  - internal/decompose/{decompose,chain}.go  # orchestrator/chain unchanged
  - internal/cmd/*                     # CLI unchanged
  - Result / DecomposeResult structs   # unchanged

TEST (internal/decompose/*_test.go):
  - +4 synchronous per-role unit tests (planner/stager/message/arbiter) — race-free verbose capture
  - +1 E2E Decompose loop test (thread-safe verbose sink + dcmStagerSeam + 2-concept partition)

DOCS (Mode A — rides with the work):
  - inline comment on each of the 4 Render calls (Tasks 1-4). No docs/*.md, no separate docs subtask.

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S2):
  - T2 (P1.M1.T2.S1): E2E CLI integration test driving the real binary with a pi-shaped stubagent config
  - P1.M6 (docs sweep): README/docs consistency (S2 is internal-comment-only — no user-facing change)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l .                          # Expected: empty (gofmt -w the 4 .go + test files if listed)
go vet ./internal/decompose/...     # Expected: exit 0
go build ./...                      # Expected: exit 0

# Confirm the 4 Render calls now pass "" and the 4 declarations discard prov
grep -rn "Render(mdl, \"\"" internal/decompose/          # Expected: 4 matches (planner/stager/message/arbiter)
grep -rn "_, mdl := config.ResolveRoleModel" internal/decompose/  # Expected: 4 matches
# Confirm stager kept RenderTooled + the two "" args
grep -n "Render(mdl, \"\", \"\", task, provider.RenderTooled)" internal/decompose/stager.go  # Expected: 1 match
# Expected: Zero output/errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

# The 4 new per-role sub-provider-resolution tests (synchronous, race-free)
go test -race -run 'TestCallPlanner_ResolvesSubProvider|TestStageConcept_ResolvesSubProvider|TestGenerateMessage_ResolvesSubProvider|TestRunArbiter_ResolvesSubProvider' ./internal/decompose/ -v

# The new E2E loop test (thread-safe verbose sink → -race-clean)
go test -race -run 'TestDecompose_RoleResolvesSubProvider' ./internal/decompose/ -v

# The existing decompose suite MUST still pass byte-identical (cfg.Defaults() Provider="" + nil Verbose)
go test -race ./internal/decompose/ -v
# Expected: 5 new tests PASS (each buf Contains "--provider openrouter", NOT "--provider pi");
#           ALL existing TestCallPlanner_*/TestStageConcept_*/TestGenerateMessage_*/TestRunArbiter_*/
#           TestDecompose_* pass unchanged.
```

### Level 3: Whole-Repository Regression (No Behavior Change Elsewhere)

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...                 # Expected: ALL packages pass (including generate, post-S1)
go vet ./...                        # Expected: exit 0

# Confirm ONLY internal/decompose/ changed in source
git diff --stat -- internal/ pkg/ cmd/
# Expected: only the 4 internal/decompose/*.go production files + their 4 _test.go + decompose_test.go.

# Confirm render.go / merge.go / config-roles.go / decompose-roles.go / generate.go were NOT touched
git diff --name-only -- internal/provider/render.go internal/provider/merge.go internal/config/roles.go internal/decompose/roles.go internal/decompose/decompose.go internal/decompose/chain.go internal/generate/generate.go
# Expected: (empty — no output)
```

### Level 4: Bug-Reproduction Cross-Check (manual smoke — optional for S2)

> S2 fixes the decompose LIBRARY path. The user-visible CLI end-to-end (`stagecoach` over a multi-change
> tree with `--verbose`) exercises all four roles, but the dedicated CLI-binary E2E test is T2
> (P1.M1.T2.S1). This smoke confirms the fix holds through the real binary for the decompose path.

```bash
cd /home/dustin/projects/stagecoach
go build -o bin/stagecoach ./cmd/stagecoach && go build -o bin/stubagent ./cmd/stubagent

# Throwaway repo + pi-shaped stub config + a multi-change working tree (auto-decompose triggers)
cd /tmp && rm -rf repro && mkdir repro && cd repro
git init -q && git config user.email t@t.com && git config user.name t && git commit -q --allow-empty -m init
echo a > a.txt; echo b > b.txt   # UNSTAGED multi-change tree (FR-M1 routing → Decompose)
SH=/home/dustin/projects/stagecoach/bin/stagecoach; STUB=/home/dustin/projects/stagecoach/bin/stubagent
cat > config.toml <<EOF
config_version = 2
[defaults]
provider = "pi"
[provider.pi]
command = "$STUB"
detect  = "$STUB"
provider_flag = "--provider"
default_provider = "openrouter"
model_flag = "--model"
default_model = "gpt-5.4-nano"
system_prompt_flag = "--system"
prompt_delivery = "stdin"
print_flag = "-p"
output = "raw"
tooled_flags = ""
[provider.pi.env]
STAGECOACH_STUB_OUT = "feat: repro"
EOF
STAGECOACH_CONFIG=config.toml $SH --dry-run --verbose --no-color 2>&1 | grep "DEBUG: command" | grep -c "provider openrouter"
# Expected (after fix): ≥1 (every decompose role emits --provider openrouter).
STAGECOACH_CONFIG=config.toml $SH --dry-run --verbose --no-color 2>&1 | grep "DEBUG: command" | grep -c "provider pi"
# Expected (after fix): 0 (no role emits the manifest name as sub-provider).
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages pass (5 new decompose tests green; existing unchanged).

### Feature Validation

- [ ] All 4 role files call `deps.Roles.<Role>.Render(mdl, "", ...)` (grep confirms 4 matches).
- [ ] All 4 declarations are `_, mdl := config.ResolveRoleModel("<role>", deps.Config)` (grep confirms 4).
- [ ] stager Render keeps `RenderTooled` + the two `""` args (grep confirms).
- [ ] Each changed Render call has an inline comment (manifest name vs sub-provider; references generate.go S1).
- [ ] Per-role unit tests: pi-shaped manifest + cfg.Provider="pi" → argv Contains "--provider openrouter".
- [ ] Per-role unit tests: argv does NOT Contain "--provider pi" (conflation signature).
- [ ] E2E loop test: full Decompose over a 2-concept partition emits "--provider openrouter", not "--provider pi", under -race.
- [ ] With no DefaultProvider (stubtest default), `--provider` omitted → existing tests pass unchanged.

### Scope Discipline Validation

- [ ] ONLY `internal/decompose/*.go` production + `*_test.go` modified (git diff --stat confirms).
- [ ] Did NOT edit `render.go`, `merge.go` (already correct).
- [ ] Did NOT edit `config/roles.go` or `decompose/roles.go` (ResolveRoleModel/ResolveRoles correct for reg.Get).
- [ ] Did NOT edit `generate.go` (S1 done) or `decompose.go`/`chain.go` (orchestrator/chain).
- [ ] Did NOT add the CLI-binary E2E test (that is T2 / P1.M1.T2.S1).
- [ ] Did NOT modify any `docs/*.md` (Mode A = inline comments only).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Comments explain the WHY (manifest name vs sub-provider), mirroring S1's generate.go comment.
- [ ] Per-role tests reuse existing helpers (plannerDeps/messageDeps/arbDeps; inline Deps for stager).
- [ ] Each test has the negative assertion (`!strings.Contains(..., "--provider pi")`) — the real regression guard.
- [ ] Pointer fields set via local-address (not the unexported provider.strPtr).
- [ ] E2E test uses a thread-safe verbose sink (lockedBuffer) so it passes `-race`.
- [ ] No new imports beyond what each test needs (bytes, sync, internal/ui, internal/prompt as applicable).

---

## Anti-Patterns to Avoid

- ❌ Don't "fix" render.go / merge.go / config-roles.go / decompose-roles.go — all already correct. The bug
  is purely the 4 Render callers passing the manifest name; the fix is at the call sites only. (issue1
  analysis confirms.) `ResolveRoleModel`'s `prov` is STILL correct for `reg.Get` in `ResolveRoles`.
- ❌ Don't leave `prov, mdl :=` after dropping `prov` from Render — Go fails to compile ("declared but not
  used"). Use `_, mdl :=` (contract-preferred) at all 4 sites. Do NOT scatter `_ = prov` statements.
- ❌ Don't collapse stager's two `""` args into one — `Render(mdl, "", "", task, RenderTooled)`: arg2 is
  the provider (the fix), arg3 is the empty system prompt (pre-existing). Two distinct `""`.
- ❌ Don't change the model arg (`mdl`) — Render already falls back to `*r.DefaultModel` when `mdl==""`.
  Only the provider arg is wrong. And don't change RenderBare/RenderTooled modes.
- ❌ Don't thread a real sub-provider through config in this subtask — that's a larger redesign. Passing
  `""` to let Render resolve from the merged manifest is the minimal, correct, contract-specified fix.
- ❌ Don't fix `generate.go` here — S1 already did (commit 9ff53e6). S2 is the 4 decompose files only.
- ❌ Don't use the unexported `provider.strPtr` in tests (cross-package can't). Set pointer fields via
  `&localVar` — the fields are exported.
- ❌ Don't forget the negative assertion — `Contains(buf, "--provider openrouter")` alone passes even on a
  buggy build that ALSO emits `--provider pi`. `!Contains(buf, "--provider pi")` is the real guard.
- ❌ Don't run the E2E loop test with a plain `*bytes.Buffer` as the verbose sink — `runLoop` overlaps
  stager[i+1] ∥ message[i] (both write the buffer) → `go test -race` FAILS. Use a mutex-guarded sink
  (lockedBuffer). The 4 per-role unit tests are synchronous → race-free → can use a plain buffer.
- ❌ Don't modify existing tests to add Verbose — they pass nil Verbose (no-op) and rely on
  `config.Defaults()` (Provider=""), which already hit `Render(mdl, "", ...)`. They are byte-identical.
- ❌ Don't add docs/*.md changes — Mode A is an inline comment on each Render call, nothing more.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a four-times-repeated two-token edit (`prov, mdl :=` → `_, mdl :=` + Render arg
`prov` → `""`) at grep-confirmed lines, with `prov`-only-at-Render proven by grep (so `_, mdl :=` is the
correct, compiler-satisfying form). The correctness is proven by quoting Render's verbatim fallback
(`""` → `*r.DefaultProvider`), and S1 already shipped + validated the identical pattern for generate.go
(commit 9ff53e6), giving a known-good comment template to mirror. The per-role tests are fully specified
(real `*Deps` helpers named, the cross-package pointer-field trick, the verbose-capture chain, the
mandatory negative assertion); they are synchronous and race-free. Back-compat is guaranteed (existing
tests use `config.Defaults()` Provider="" + nil Verbose → byte-identical). The only residual uncertainty
(not 10/10) is the E2E loop test's thread-safe-sink boilerplate + the per-role message/arbiter test
fixture details (tree SHAs for generateMessage, CommitInfo/leftoverDiff for runArbiter) — all of which
have named existing test templates to mirror, and any compile/fixture error is caught immediately by
`go test -race ./internal/decompose/`. The T2 (CLI-binary E2E) boundary is cleanly fenced and cannot be
broken by S2.
