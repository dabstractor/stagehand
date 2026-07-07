---
name: "P1.M2.T4.S1 — Render manifest to CmdSpec (args + stdin source) — PRD §12.2 command rendering"
description: |

  Land the SOLE subtask of Command Rendering (P1.M2.T4): a method `func (m Manifest) Render(model,
  provider, sysPrompt, userPayload string) (*CmdSpec, error)` that implements the PRD §12.2 "Command
  rendering algorithm" EXACTLY and returns a `CmdSpec{Command, Args, Stdin, Env}` — the full subprocess
  invocation spec the executor (P1.M2.T5) consumes. Build Args in the §12.2 token order
  (subcommand → provider_flag → model_flag → system_prompt_flag → bare_flags → print_flag → payload),
  resolve the prompt_delivery switch (stdin|positional|flag), assemble the stdin payload WITH the
  system-prompt-prepend fallback (sys prepended to the payload when `system_prompt_flag == ""`), and
  build Env = os.Environ() + manifest.Env. PLUS table-driven GOLDEN tests per provider asserting the
  rendered Args/Stdin match PRD §12.3–§12.7 — with pi byte-for-byte identical to the commit-pi
  invocation (THE headline requirement).

  Builds DIRECTLY on the already-landed `Manifest` + `Validate()` + `Resolve()` + `DetectCommand()` +
  `strPtr`/`boolPtr` + the `Default*` constants (S1, `manifest.go`) and the 6 built-in manifests
  `BuiltinManifests()` (S2 + S3, `builtin.go`). It does NOT edit any of those files (S1's contract is
  FROZEN; the parallel registry PRP P1.M2.T3.S1 also depends on manifest.go being untouched — placing
  Render in a NEW `render.go` avoids any merge collision AND satisfies "a method of the Manifest type",
  since Go permits a type's methods to live in any file of the same package).

  ⚠️ **THE central design call — §12.2 ALGORITHM IS AUTHORITATIVE; the §12.3–§12.7 narrative "Rendered:"
  blocks are ILLUSTRATIVE.** For pi (§12.3) the narrative coincides with the algorithm. For claude (§12.4)
  and cursor (§12.7) the narrative shows `-p` FIRST (`claude -p --model ...`, `agent -p --mode ask ...`),
  but the §12.2 algorithm appends `print_flag` LAST (after bare_flags). The renderer follows §12.2:
  **print_flag is ALWAYS last.** This is CONFIRMED by the existing headline test
  `TestBuiltinManifests_RenderedCommand_Pi_MatchesCommitPi` (builtin_test.go:339 — pins `-p` last with the
  comment "print_flag LAST (matches §12.3 + commit-pi)") AND by `TestBuiltinManifests_RenderedCommand_Cursor`
  (builtin_test.go:583 — explicitly notes the cursor narrative differs from the algo and that "§12.2 is
  authoritative"). Render's Args are byte-compatible with the existing test-only `renderArgs` helper.

  ⚠️ **THE second design call — the stdin / system-prompt fallback is ONE unified `payload` string.**
  Compute `payload = userPayload`; if `*resolved.SystemPromptFlag == "" && sys != ""` then
  `payload = sys + "\n\n" + userPayload` (the prepend fallback for agents with no sys flag:
  gemini/opencode/codex/cursor). Then: stdin delivery → `spec.Stdin = payload` (nothing appended);
  positional → `args += [payload]`; flag → `args += [promptFlag, payload]`. This single rule produces the
  correct result for ALL SIX built-ins (verified in research §2). The delimiter is exactly `"\n\n"`
  (every §12.5–§12.7 narrative writes `"<sys>\n\n<user payload>"`); empty sys → no prepend.

  ⚠️ **THE third design call — Render calls `Validate()` THEN `Resolve()` internally (defensive).**
  The registry's documented consumer lifecycle is `Get → Validate → Resolve → consume`; Render OWNS the
  tail so it is robust to a caller that skipped Validate/Resolve. `m.Validate()` returns its error
  (covers the work item's "Validate prompt_delivery mode" + missing Command/Name); `m.Resolve()` makes
  every pointer non-nil (safe deref, no nil-panics) on a COPY (the caller's m is never mutated). The
  delivery switch's default case also returns a wrapped error (belt-and-suspenders — Validate already
  rejects an invalid prompt_delivery).

  ⚠️ **THE fourth design call — CmdSpec is `{Command, Args, Stdin, Env}`; Stdin="" means "no stdin pipe".**
  CmdSpec does NOT carry the delivery mode, so Stdin disambiguates. Render sets Stdin = payload for stdin
  delivery, "" for positional/flag. The executor (P1.M2.T5) contract: `if spec.Stdin != "" { pipe it }
  else { os.DevNull }` — matching PRD §12.2's `cmd.Stdin = (delivery=="stdin") ? reader : /dev/null`.

  ⚠️ **THE fifth design call — Env = append(os.Environ(), "KEY=VAL"…); manifest env WINS on collision.**
  PRD §12.2: `cmd.Env = os.Environ() + m.env`. Append manifest entries AFTER os.Environ() so exec's
  last-wins semantics make manifest env override the parent. TESTABILITY: os.Environ() is
  machine-dependent → tests assert manifest-env membership (`"KEY=VAL"` ∈ spec.Env) and
  `len(spec.Env) >= len(os.Environ())`, NOT full Env equality.

  ⚠️ **THE sixth design call — model/provider DEFAULTS applied inside Render when the param is "".**
  Mirrors the existing `renderArgs` scaffolding (builtin_test.go:137): `modelToUse = model ||
*r.DefaultModel`
  (so the pi golden test passes with `model=""` → glm-5-turbo). Symmetric `providerToUse = provider ||
  *r.DefaultProvider` — harmless for every built-in (their resolved DefaultProvider is "") but correctly
  honors a §12.8 user manifest's `default_provider`. Callers may pass an explicit value to override.

  Deliverable: `internal/provider/render.go` (`package provider`) — the `CmdSpec` struct + the `Render`
  method + (optionally) a `payload` helper; and `internal/provider/render_test.go` (`package provider`,
  white-box) — ~10 test groups incl. the 6-provider golden table. INPUT = S1's `Manifest`/`Validate`/
  `Resolve`/`strPtr`/`boolPtr` and S2/S3's `BuiltinManifests`. Touches ONLY the two new files — NO
  go.mod/go.sum change (stdlib only), NO edit to any frozen file. OUTPUT = the CmdSpec the executor
  (P1.M2.T5) runs and that the generate flow (P1.M3.T4) hands to the executor.

---

## Goal

**Feature Goal**: Implement the provider renderer — the function that turns a resolved provider
`Manifest` plus a (model, provider, sysPrompt, userPayload) tuple into a concrete `CmdSpec`
(command + args + stdin content + environment) per PRD §12.2. It is the second half of the provider
system's "produce a concrete command line" mission (§12): manifests (T1/T2/T3) describe the agents; the
renderer (T4) composes the invocation; the executor (T5) runs it; the parser (T6) reads the result.

**Deliverable**:
1. **CREATE** `internal/provider/render.go` (`package provider`) —
   (a) `type CmdSpec struct { Command string; Args []string; Stdin string; Env []string }`.
   (b) `func (m Manifest) Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error)` —
       Validate → Resolve → build Args per §12.2 token order → resolve the prompt_delivery switch →
       assemble Stdin/payload WITH the sys-prepend fallback → build Env = os.Environ() + m.Env.
   (c) Imports: `fmt`, `os` ONLY (stdlib; no go-toml, no os/exec — Render does not spawn anything).
2. **CREATE** `internal/provider/render_test.go` (`package provider`, white-box) — the ~10 test groups
   in Implementation Tasks, all passing. Uses S1's unexported `strPtr`/`boolPtr` (same package);
   `reflect.DeepEqual` for Args/slice comparison; set-membership for Env (os.Environ() is
   machine-dependent). The 6-provider golden table is the keystone.

No other files touched. **No go.mod/go.sum change** (stdlib only — Render uses `fmt` + `os`). NO edit to
`manifest.go`/`manifest_test.go` (S1), `merge.go`/`merge_test.go` (S2), `builtin.go`/`builtin_test.go`
(S2/S3), or any file outside `internal/provider/render*.go`.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/` clean;
`go mod tidy` is a no-op; `go test -race ./internal/provider/ -v` passes (S1/S2/S3 + parallel-registry
tests STILL green + all new render tests green) and the full suite `go test -race ./...` stays green;
the pi golden test renders byte-for-byte the commit-pi invocation (`pi --provider zai --model
glm-5-turbo --system-prompt "<sys>" --no-tools … --no-session -p` with user via stdin); all 6 built-in
providers render their exact §12.3–§12.7 Args (algorithm order, print_flag last); the sys-prepend
fallback produces `"<sys>\n\n<user>"` for the no-sys-flag agents; Env carries os.Environ() + manifest
entries (manifest wins on collision). go.mod/go.sum and every frozen file byte-unchanged.

## User Persona

**Target User**: The executor (P1.M2.T5 — runs the CmdSpec via `exec.Command(spec.Command, spec.Args...)`
with `cmd.Stdin`/`cmd.Env` from the spec) and the generate flow (P1.M3.T4 — calls `reg.Get(name) →
m.Render(...)` to produce the spec, then hands it to the executor). Transitively every user story routed
through "call an agent" (§8) and FR9/FR21 (generation). The verbose CLI mode (P1.M4.T3.S2) also prints
the resolved command (`spec.Command + spec.Args`) for debugging.

**Use Case**: The generate flow resolves the active provider manifest from the registry (P1.M3.T4:
`m, _ := reg.Get(cfg.Provider); m.Validate()`), assembles the system prompt (P1.M3.T1) and user payload
(P1.M3.T1.S3), then calls `spec, err := m.Render(model, provider, sysPrompt, userPayload)`. The executor
runs `spec`; the parser (T6) reads stdout. The renderer is the bridge "logical intent → concrete argv".

**User Journey**: (internal API, no end-user surface yet) `Registry.Get` → `Manifest.Render(model,
provider, sys, user)` → `*CmdSpec` → executor (`exec.Command(spec.Command, spec.Args...)`, stdin from
`spec.Stdin`, env from `spec.Env`) → stdout → parser.

**Pain Points Addressed**: Removes "how does a manifest become a concrete command line / where does the
system prompt go (flag vs stdin) / in what order are tokens emitted / how are env vars composed"
ambiguity by landing one tested renderer now — the single site that owns PRD §12.2.

## Why

- **The renderer is the core of agent-agnosticism (§12).** "given a logical intent … produce a concrete
  command line for a specific agent" — the renderer IS that production. Without it the manifest schema
  (T1/T2/T3) is inert data.
- **Pins the commit-pi byte-for-byte guarantee.** The pi provider MUST reproduce the exact invocation
  `commit-pi` uses today (§12.3 + the work-item headline). The golden test is the regression gate.
- **Unlocks the executor (P1.M2.T5) and the generate flow (P1.M3.T4).** Both consume `*CmdSpec`.
  Landing the CmdSpec shape + Render now lets T5 be "dumb exec over a fully-specified spec" and M3.T4
  be "call Render, hand to executor".
- **Proves the §12.2 algorithm end-to-end against all 6 verified providers.** The golden table is the
  integration proof that the manifest fields (S1) + the 6 built-ins (S2/S3) + the render algorithm
  produce the correct, PRD-matching invocations — including the non-obvious print_flag-last ordering
  and the sys-prepend fallback.
- **No user-facing surface change** (PRD "DOCS: none — internal algorithm"). The verbose-mode docs come
  with P1.M4.T3.S2.
- **No new dependency.** Stdlib only (`fmt`, `os`). go.mod/go.sum unchanged.

## What

A compiled `internal/provider` package exporting `CmdSpec` + `Manifest.Render`, layered on S1's
`Manifest`/`Validate`/`Resolve` and S2/S3's `BuiltinManifests`. No execution, no parsing, no CLI, no
config edits.

### Success Criteria

- [ ] `internal/provider/render.go` exists, `package provider`, imports EXACTLY `fmt`, `os`. It does NOT
      import `os/exec`, `go-toml`, or `internal/config`.
- [ ] `type CmdSpec struct { Command string; Args []string; Stdin string; Env []string }` (all exported;
      the executor and verbose-CLI read these directly).
- [ ] `func (m Manifest) Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error)`:
      calls `m.Validate()` (return its error); calls `m.Resolve()` (safe deref); builds Args in §12.2
      order; resolves the prompt_delivery switch; assembles Stdin/payload with the sys-prepend fallback;
      builds Env = `append(os.Environ(), manifest-env as "KEY=VAL"...)`.
- [ ] Args token order is EXACTLY: `[subcommand...]` → (`provider_flag`,`provider` if both non-empty) →
      (`model_flag`,`model` if both non-empty) → (`system_prompt_flag`,`sys` if both non-empty) →
      `bare_flags...` → (`print_flag` if non-empty) → (payload per delivery switch). **print_flag is
      LAST (after bare_flags) for every provider.**
- [ ] `model`/`provider` params default to the resolved manifest's `DefaultModel`/`DefaultProvider` when
      the param is `""` (mirrors the `renderArgs` scaffolding).
- [ ] Stdin/payload: `payload = userPayload`; if `*resolved.SystemPromptFlag == "" && sys != ""` then
      `payload = sys + "\n\n" + userPayload`. stdin → `spec.Stdin = payload`; positional →
      `args += [payload]`; flag → `args += [promptFlag, payload]`. (Stdin = "" for positional/flag.)
- [ ] Env: `append(os.Environ()...)` then `"KEY=VAL"` for each `m.Env` entry; manifest entries appended
      AFTER os.Environ() (last-wins → manifest overrides).
- [ ] `render_test.go` has the ~10 test groups below, all passing. The 6-provider golden table asserts
      EXACT Args + Stdin per provider (pi byte-for-byte commit-pi).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/`, `go mod tidy` (diff-empty) clean;
      `go test -race ./internal/provider/` AND `go test -race ./...` green.
- [ ] go.mod/go.sum byte-unchanged; every file outside the two new `render*.go` files byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the exact §12.2
algorithm + token order (research §1), the unified payload/sys-prepend rule (research §2), the golden
CmdSpec table per provider (research §3), the 6 design calls (research §4), the `Manifest`/`Validate`/
`Resolve`/`strPtr`/`boolPtr` contracts (already landed — read `manifest.go`/`builtin.go`), and the ~10
test specs. No git/generate/CLI knowledge required — the renderer is a pure function over an already-landed
type (modulo the documented `os.Environ()` side-effect for Env, handled in tests via set-membership).

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M2T4S1/research/render-algorithm-and-golden-specs.md
  why: the SINGLE most important read — the §12.2 algorithm token order (§1), the unified payload +
       sys-prepend fallback rule (§2), the EXACT golden CmdSpec table for all 6 providers (§3 — assert
       these values verbatim in the table-driven tests), the 6 design calls (§4 — placement/signature/
       Validate-Resolve/Stdin-semantics/Env/model-defaults), the byte-compatibility note vs renderArgs
       (§5), and the import set (§6).
  critical: §3 (the golden table) and §4 Call A (print_flag LAST; §12.2 is authoritative over the
       §12.4/§12.7 narratives) are the things most likely to be implemented wrong. Read before writing
       Render or the golden tests.

- file: internal/provider/manifest.go   (S1 — COMPLETE; read, do NOT edit)
  why: the EXACT Manifest type Render operates on + `func (m Manifest) Validate() error` (returns error
       on missing Name/Command or invalid prompt_delivery/output enum — Render calls this first) and
       `func (m Manifest) Resolve() Manifest` (fills every nil optional pointer to its default so Render
       can deref safely; returns a COPY — caller's m untouched). Also the `Default*` constants and the
       unexported helpers `strPtr`/`boolPtr` (same package — use in tests).
  critical: after Resolve EVERY pointer is non-nil: Command may still be nil only if it was nil (Validate
       catches that); PromptDelivery/PrintFlag/ModelFlag/DefaultModel/SystemPromptFlag/ProviderFlag/
       DefaultProvider/PromptFlag are all safe `*r.X`. Do NOT edit this file.

- file: internal/provider/builtin.go   (S2 + S3; read, do NOT edit)
  why: `func BuiltinManifests() map[string]Manifest` returns the 6 built-ins the golden table tests
       iterate. The per-provider field values (esp. PromptDelivery, SystemPromptFlag="", PrintFlag,
       BareFlags, DefaultModel, Subcommand, Command) DETERMINE the golden Args — read them to recompute
       the table if you doubt a value. NOTE: cursor Command="agent" (≠ Name "cursor"); gemini+codex
       PromptDelivery="stdin" (REVISED from the §12.5/§12.7 "positional" narratives); codex BareFlags use
       "--ephemeral" (REVISED — not "--ask-for-approval"); opencode BareFlags=[]string{} (NON-NIL empty);
       claude BareFlags contain TWO "" value tokens (--tools "" / --setting-sources "") — do NOT drop them.
  critical: the REVISED values (gemini/codex stdin; codex --ephemeral) mean the golden Args follow the
       BUILTIN manifests, NOT the §12.5/§12.7 narrative "Rendered" blocks. Do NOT edit this file.

- file: internal/provider/builtin_test.go   (S2/S3 — read for the golden pattern; do NOT edit)
  section: `renderArgs` (line ~137) + `TestBuiltinManifests_RenderedCommand_*` (lines ~339–600).
  why: `renderArgs` is the TEST-ONLY faithful port of §12.2 (flags-only argv). Render's Args are
       byte-compatible with renderArgs's output (renderArgs returns Command as element[0]; CmdSpec splits
       Command out — same tokens, same order). The 6 `RenderedCommand_*` tests give the EXACT expected
       argv per provider — reuse those values as the golden Args. The cursor test's comment explicitly
       documents "§12.2 is authoritative" (the narrative differs).
  critical: renderArgs is NOT reused by Render (Render implements §12.2 standalone) and is NOT deleted
       (frozen test file). They coexist. Do NOT edit this file.

- file: plan/001_f1f80943ac34/P1M2T3S1/PRP.md   (parallel registry PRP — read for the contract)
  section: the registry's documented consumer lifecycle "Get → Validate → Resolve → consume".
  why: Render is the CONSUMER of the registry's merged manifest. The registry stores MERGED manifests
       (no Validate/Resolve); Render owns Validate+Resolve (design call C). Render must work on a manifest
       from `reg.Get(name)` (merged, unresolved). This PRP also confirms manifest.go is treated as FROZEN
       by the parallel work — justifying Render's placement in a new render.go (design call A).
  critical: Render must NOT assume the manifest is already Validated/Resolved (it does both itself).

- file: PRD.md
  section: "12.2 Command rendering algorithm" (h3.38) — the EXACT pseudocode Render implements.
  why: the algorithm. Note the `cmd.Stdin = (delivery=="stdin") ? reader : /dev/null` and `cmd.Env =
       os.Environ() + m.env` lines, and the "Note on system prompt + stdin" (sys via flag when a flag
       exists; sys PREPENDED to stdin payload when no flag exists).
  critical: the §12.2 ALGORITHM is authoritative; the §12.3–§12.7 narrative "Rendered:" blocks are
       illustrative (claude §12.4 and cursor §12.7 show -p FIRST, which the algorithm does NOT). Render
       follows the algorithm (print_flag last). Confirmed by builtin_test.go's cursor + pi tests.

- file: PRD.md
  section: "12.1 The manifest schema" (h3.37) — the field semantics Render dereferences.
  why: prompt_delivery values (stdin|positional|flag); print_flag ("nil/\"\" => no print flag");
       system_prompt_flag ("nil/\"\" => prepend sys to payload"); the §12.2 note. Render's behavior maps
       1:1 to these.

- file: plan/001_f1f80943ac34/architecture/critical_findings.md
  section: FINDING 5 (go-toml has no omitempty → pointer design) — the reason S1's optional scalars are
       pointers, which Resolve normalizes for Render.
  why: explains WHY Render must Resolve before deref (raw nil pointers would panic). Already solved by S1's
       pointer design + Resolve; Render just calls Resolve.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; require go-toml/v2 + pflag  (UNCHANGED — Render adds NO dep)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 — untouched; do NOT import (cycle, and unneeded)
  git/                          # P1.M1.T2/T3 — untouched
  provider/                     # S1 created; S2 added merge+builtin(pi/claude/gemini/opencode); S3 added codex/cursor; T3(parallel) adds registry
    manifest.go                 # S1 — Manifest + Validate + Resolve + DetectCommand + Default* + strPtr/boolPtr  (CONTRACT — do NOT edit)
    manifest_test.go            # S1 — tests  (do NOT edit)
    merge.go                    # S2 — MergeManifest  (do NOT edit)
    merge_test.go               # S2 — tests  (do NOT edit)
    builtin.go                  # S2+S3 — BuiltinManifests() (6 keys) + constructors  (do NOT edit)
    builtin_test.go             # S2+S3 — tests incl. renderArgs + the 6 RenderedCommand_* golden argv  (do NOT edit)
    registry.go                 # P1.M2.T3.S1 (parallel) — Registry  (may not exist yet when you start; Render does NOT depend on it)
    registry_test.go            # P1.M2.T3.S1 (parallel) — tests
    render.go                   # NEW (this subtask) ← CmdSpec + (m Manifest) Render  (stdlib: fmt, os)
    render_test.go              # NEW (this subtask) ← ~10 test groups incl. the 6-provider golden table
cmd/stagecoach/main.go           # `package main; func main(){}` stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  provider/
    render.go                   # NEW — CmdSpec struct + func (m Manifest) Render(model, provider, sys, user) (*CmdSpec, error)
    render_test.go              # NEW — ~10 test groups, package provider (white-box)
# manifest.go/manifest_test.go (S1) + merge.go/merge_test.go (S2) + builtin.go/builtin_test.go (S2/S3)
# + registry.go/registry_test.go (T3, parallel) UNCHANGED. go.mod/go.sum UNCHANGED. Every other file UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (design call A — NEW file render.go; do NOT edit manifest.go): the parallel registry PRP
// (P1.M2.T3.S1) treats manifest.go as FROZEN, and S1's Manifest/Validate/Resolve/DetectCommand are a
// CONTRACT. Adding Render to manifest.go risks a merge collision with the parallel registry work. Go
// permits a type's methods to live in ANY file of the same package, so `(m Manifest) Render(...)` in
// render.go is STILL "a method of the Manifest type". CmdSpec is declared in render.go too. render.go is
// the ONLY new non-test file.

// CRITICAL (design call — print_flag is LAST, NOT first): the §12.2 algorithm appends print_flag AFTER
// bare_flags. The §12.4 (claude) and §12.7 (cursor) narrative "Rendered:" blocks show -p FIRST — those
// are ILLUSTRATIVE and WRONG vs the algorithm. Render follows the algorithm (confirmed by the pi
// byte-for-byte test + the cursor test's own comment "§12.2 is authoritative"). Do NOT reorder to match
// the claude/cursor narratives.

// CRITICAL (design call C — Validate then Resolve INSIDE Render): Render calls m.Validate() first (return
// its error — covers the work item's "Validate prompt_delivery mode" + missing Command/Name), then
// m.Resolve() (every pointer non-nil → safe `*r.X` deref, no nil-panics). Resolve returns a COPY; the
// caller's m is never mutated. Do NOT deref raw m pointers (a nil SystemPromptFlag would panic).

// CRITICAL (design call — unified payload + sys-prepend): compute ONE payload string and reuse it.
//   payload := userPayload
//   if *r.SystemPromptFlag == "" && sysPrompt != "" { payload = sysPrompt + "\n\n" + userPayload }
// stdin → spec.Stdin = payload; positional → args += [payload]; flag → args += [r.PromptFlag, payload].
// The delimiter is EXACTLY "\n\n" (every §12.5–§12.7 narrative writes "<sys>\n\n<user payload>"). Empty
// sys → no prepend (payload = userPayload). Do NOT condition the prepend on delivery mode — the single
// `*r.SystemPromptFlag == ""` check is correct for ALL delivery modes (verified research §2).

// CRITICAL (design call — model/provider defaults applied when param is ""): modelToUse := model; if
// modelToUse == "" { modelToUse = *r.DefaultModel }. providerToUse := provider; if providerToUse == "" {
// providerToUse = *r.DefaultProvider }. This makes the pi golden test pass with model="" (→ glm-5-turbo)
// and honors a §12.8 default_provider. An explicit non-empty param always wins. Do NOT skip the default
// fallback (opencode/codex/cursor golden tests pass explicit models; pi/gemini rely on the default).

// CRITICAL (design call E — Stdin="" means "no stdin pipe"): CmdSpec does not carry delivery mode. Render
// sets Stdin = payload for stdin delivery, "" for positional/flag. The executor (T5) pipes via
// strings.NewReader(spec.Stdin) when non-empty, else os.DevNull. Do NOT set Stdin for positional/flag.

// CRITICAL (design call F — Env: os.Environ() FIRST, manifest entries AFTER): env := os.Environ(); for
// k,v := range r.Env { env = append(env, k+"="+v) }. Manifest entries appended LAST → exec last-wins →
// manifest overrides the parent env. Tests MUST NOT assert full Env (os.Environ() is machine-dependent) —
// assert set-membership ("KEY=VAL" ∈ spec.Env) + len(spec.Env) >= len(os.Environ()).

// GOTCHA: cursor is the ONLY built-in where Command != Name (Command="agent"). spec.Command = *r.Command
// (NOT m.Name). The pi golden test asserts spec.Command == "pi"; the cursor golden asserts "agent".

// GOTCHA: claude's BareFlags = ["--tools","","--setting-sources","","--no-session-persistence"] — TWO ""
// value tokens (the args to --tools / --setting-sources). append(r.BareFlags...) copies them verbatim.
// Do NOT filter empty strings from BareFlags (they are meaningful value args).

// GOTCHA: opencode's BareFlags = []string{} (NON-NIL empty) and Subcommand = ["run"]. append(nil...) and
// append([]string{}...) are both no-ops for Args, but Subcommand MUST be appended (["run"] → "run" in
// Args). opencode is positional delivery → the payload IS the trailing arg.

// GOTCHA: Render's import set is `fmt` + `os` ONLY. Do NOT import `strings` unless you use it (the
// payload uses `+` concatenation, not strings.Join — so `strings` is likely unused; `go vet` flags
// unused imports). Do NOT import os/exec (spawning is the executor's job). Do NOT import go-toml.

// GOTCHA: Render does NOT set cmd.Stdin to a reader or spawn anything — it returns a CmdSpec (pure data).
// The os.Environ() call is the ONLY side effect (acceptable; documented; tests handle via set-membership).

// GOTCHA: Validate() REQUIRES a non-empty Name + non-nil/non-empty Command. Any hand-built Manifest{}
// literal in the TESTS (Test 5/6/7/8) MUST set Name (and Command) or Render returns an error and the
// spec is nil (nil-spec.Args would panic). The built-in manifests all have Name set, so the golden
// table (Test 1) is fine. Always set Name on hand-built test manifests.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/provider/render.go
package provider

import (
	"fmt"
	"os"
)

// CmdSpec is the fully-specified subprocess invocation produced by Manifest.Render (PRD §12.2). It is
// the contract between the renderer (this package, T4) and the executor (P1.M2.T5): the executor runs
// `exec.Command(spec.Command, spec.Args...)` with cmd.Stdin/cd.Env derived from Stdin/Env. It is pure
// data — Render performs NO spawning (the os.Environ() call for Env is the sole side effect).
//
// Stdin semantics: Stdin carries the payload to pipe for stdin-delivery providers; it is "" for
// positional/flag-delivery providers, which the executor interprets as "use os.DevNull" (matching PRD
// §12.2 `cmd.Stdin = (delivery=="stdin") ? reader : /dev/null`). CmdSpec intentionally does NOT carry
// the delivery mode — Stdin="" disambiguates.
//
// Env semantics: os.Environ() (the parent process env) followed by the manifest's Env entries as
// "KEY=VAL". exec.Cmd.Env uses last-wins, so manifest entries (appended last) override the parent —
// matching PRD §12.2 `cmd.Env = os.Environ() + m.env`.
type CmdSpec struct {
	Command string   // the executable (resolved manifest.Command), e.g. "pi", "agent"
	Args    []string // the flag portion AFTER command, in §12.2 token order (NOT including Command)
	Stdin   string   // payload to pipe (stdin delivery); "" → executor uses os.DevNull
	Env     []string // os.Environ() + manifest Env entries as "KEY=VAL" (manifest wins on collision)
}

// Render turns a provider Manifest + a (model, provider, sysPrompt, userPayload) tuple into a CmdSpec
// per PRD §12.2 "Command rendering algorithm". It is the bridge "logical intent → concrete argv".
//
// Lifecycle: Render calls m.Validate() (returns its error — covers "Validate prompt_delivery mode" +
// missing Command/Name) then m.Resolve() (every pointer non-nil → safe deref on a COPY; caller's m
// untouched). This makes Render robust to a caller that obtained the manifest from Registry.Get and
// skipped Validate/Resolve (the registry stores merged-but-unresolved manifests per P1.M2.T3).
//
// Token order (§12.2 — AUTHORITATIVE; the §12.3–§12.7 narrative "Rendered:" blocks are illustrative):
//
//	args = [subcommand...]
//	+ (provider_flag, provider)        if provider_flag != "" && provider != ""
//	+ (model_flag,    model)           if model_flag    != "" && model    != ""
//	+ (system_prompt_flag, sys)        if system_prompt_flag != "" && sys != ""
//	+ bare_flags...
//	+ print_flag                       if print_flag != ""        // ALWAYS LAST (after bare_flags)
//	+ payload                          per prompt_delivery switch (positional/flag only)
//
// model/provider default to the resolved manifest's DefaultModel/DefaultProvider when the param is ""
// (mirrors the renderArgs test scaffolding; lets the pi golden test pass with model="" → glm-5-turbo,
// and honors a §12.8 user manifest's default_provider). An explicit non-empty param always wins.
//
// System-prompt + payload (§12.2 "Note on system prompt + stdin"): when system_prompt_flag != "" the
// sys prompt is emitted via the flag and the payload is just the user payload; when system_prompt_flag
// == "" the sys prompt is PREPENDED to the payload as a fallback (delimiter "\n\n", matching every
// §12.5–§12.7 narrative). The unified payload is then routed by the delivery switch: stdin → spec.Stdin;
// positional → trailing arg; flag → prompt_flag + payload.
func (m Manifest) Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("provider render %q: %w", m.Name, err)
	}
	r := m.Resolve() // safe `*r.X` deref for every pointer; copy — caller's m untouched

	// model/provider default fallback (param "" → manifest default). Explicit non-empty wins.
	modelToUse := model
	if modelToUse == "" {
		modelToUse = *r.DefaultModel
	}
	providerToUse := provider
	if providerToUse == "" {
		providerToUse = *r.DefaultProvider
	}

	// §12.2 token order. append(nil, x...) is safe for absent slices (Subcommand/BareFlags nil → no-op).
	args := make([]string, 0, 16)
	args = append(args, r.Subcommand...)
	if *r.ProviderFlag != "" && providerToUse != "" {
		args = append(args, *r.ProviderFlag, providerToUse)
	}
	if *r.ModelFlag != "" && modelToUse != "" {
		args = append(args, *r.ModelFlag, modelToUse)
	}
	if *r.SystemPromptFlag != "" && sysPrompt != "" {
		args = append(args, *r.SystemPromptFlag, sysPrompt)
	}
	args = append(args, r.BareFlags...)
	if *r.PrintFlag != "" {
		args = append(args, *r.PrintFlag) // LAST per §12.2
	}

	// Unified payload + the system-prompt-prepend fallback (§12.2 note). The single sys-flag check is
	// correct for ALL delivery modes (research §2). Delimiter is exactly "\n\n"; empty sys → no prepend.
	payload := userPayload
	if *r.SystemPromptFlag == "" && sysPrompt != "" {
		payload = sysPrompt + "\n\n" + userPayload
	}

	// prompt_delivery switch. stdin → payload to Stdin (nothing appended); positional → trailing arg;
	// flag → prompt_flag + payload. Default → error (Validate already rejects invalid values).
	spec := &CmdSpec{Command: *r.Command, Args: args}
	switch *r.PromptDelivery {
	case "stdin":
		spec.Stdin = payload
	case "positional":
		spec.Args = append(spec.Args, payload)
	case "flag":
		spec.Args = append(spec.Args, *r.PromptFlag, payload)
	default:
		return nil, fmt.Errorf("provider render %q: unsupported prompt_delivery %q", m.Name, *r.PromptDelivery)
	}

	// Env = os.Environ() + manifest Env entries (manifest appended last → exec last-wins → override).
	env := os.Environ()
	for k, v := range r.Env {
		env = append(env, k+"="+v)
	}
	spec.Env = env

	return spec, nil
}
```

> **gofmt note:** run `gofmt -w internal/provider/render.go internal/provider/render_test.go`. Do not
> hand-align. One doc comment per exported identifier (citing PRD §12.2 + the design calls) is required —
> it seeds the verbose-CLI docs (P1.M4.T3.S2) later.
>
> **Imports:** EXACTLY `fmt`, `os` in render.go. If you reach for `strings` (e.g. strings.Join), prefer
> `+` concatenation (the payload is two-piece) and omit `strings` — `go vet` flags unused imports.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/provider/render.go — CmdSpec + Render (Validate/Resolve/Args/switch)
  - DECLARE type CmdSpec struct { Command string; Args []string; Stdin string; Env []string }.
  - IMPLEMENT func (m Manifest) Render(model, provider, sysPrompt, userPayload string) (*CmdSpec, error)
      per the Data Models block: Validate → Resolve → model/provider default fallback → §12.2 token order
      → unified payload + sys-prepend → delivery switch → Env. Return *CmdSpec, nil.
  - IMPORTS: "fmt", "os". (Add "errors" only if you prefer errors.New over fmt.Errorf — fmt.Errorf is
      used above for %q/%w wrapping, so "fmt" suffices.)
  - GOTCHA: print_flag LAST. Do NOT deref raw m pointers — deref the RESOLVED r. Stdin="" for non-stdin.
      Env appends manifest entries AFTER os.Environ(). Do NOT import os/exec/strings/go-toml.

Task 2: CREATE internal/provider/render_test.go — the ~10 test groups (see Test Specs below)
  - PACKAGE: `package provider` (white-box — may use S1's strPtr/boolPtr, though most tests use the
      built-in manifests directly). Imports: testing, reflect, os (for os.Environ() length check),
      fmt/strings as needed.
  - MIRROR repo test style: stdlib testing, direct t.Errorf("X = %v, want %v", got, want),
      reflect.DeepEqual for Args/slices. (See builtin_test.go's RenderedCommand_* tests for the idiom.)
  - THE KEYSTONE: TestRender_GoldenPerProvider — a table-driven test iterating BuiltinManifests(),
      one row per provider with EXACT expected {Command, Args, Stdin} from research §3.

Task 3: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. S1's manifest*.go,
      S2's merge*.go + builtin*.go, S3's builtin tests MUST be byte-unchanged. The parallel registry
      tests (if present) MUST stay green. `go test -race ./...` green.
```

### Test Specs (render_test.go — ~10 groups)

```go
// Inputs shared across the golden table unless a row overrides: sys="<sys>", user="<user>".
// model/provider per row. Env is asserted via set-membership (os.Environ() is machine-dependent).

// 1. THE KEYSTONE — golden Args+Stdin for ALL 6 built-in providers (research §3, byte-compatible with
//    builtin_test.go's renderArgs outputs; pi is byte-for-byte commit-pi).
func TestRender_GoldenPerProvider(t *testing.T) {
	pi := builtinPi(); claude := builtinClaude(); gemini := builtinGemini()
	opencode := builtinOpenCode(); codex := builtinCodex(); cursor := builtinCursor()
	cases := []struct {
		name     string
		m        Manifest
		model    string
		provider string
		wantCmd  string
		wantArgs []string
		wantStdin string
	}{
		{"pi", pi, "", "zai", "pi",
			[]string{"--provider","zai","--model","glm-5-turbo","--system-prompt","<sys>",
				"--no-tools","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session","-p"},
			"<user>"}, // stdin; sys via flag → only user via stdin
		{"claude", claude, "sonnet", "", "claude",
			[]string{"--model","sonnet","--system-prompt","<sys>",
				"--tools","","--setting-sources","","--no-session-persistence","-p"}, // -p LAST
			"<user>"},
		{"gemini", gemini, "", "", "gemini",
			[]string{"-m","gemini-2.5-pro","--approval-mode","default"},
			"<sys>\n\n<user>"}, // stdin; no sys flag → sys PREPENDED
		{"opencode", opencode, "anthropic/claude-sonnet-4", "", "opencode",
			[]string{"run","-m","anthropic/claude-sonnet-4","<sys>\n\n<user>"}, // positional → payload trailing
			""},
		{"codex", codex, "gpt-5", "", "codex",
			[]string{"exec","-m","gpt-5","--sandbox","read-only","--ephemeral"},
			"<sys>\n\n<user>"}, // stdin (REVISED builtin); no sys flag → PREPENDED
		{"cursor", cursor, "gpt-5", "", "agent", // Command="agent" (≠ Name "cursor")
			[]string{"--model","gpt-5","--mode","ask","--trust","-p","<sys>\n\n<user>"}, // -p LAST; positional
			""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := tc.m.Render(tc.model, tc.provider, "<sys>", "<user>")
			if err != nil { t.Fatalf("%s: Render error: %v", tc.name, err) }
			if spec.Command != tc.wantCmd { t.Errorf("%s: Command = %q, want %q", tc.name, spec.Command, tc.wantCmd) }
			if !reflect.DeepEqual(spec.Args, tc.wantArgs) {
				t.Errorf("%s: Args =\n got %v\nwant %v", tc.name, spec.Args, tc.wantArgs)
			}
			if spec.Stdin != tc.wantStdin { t.Errorf("%s: Stdin = %q, want %q", tc.name, spec.Stdin, tc.wantStdin) }
		})
	}
}

// 2. THE headline: pi is byte-for-byte the commit-pi invocation (explicit, not just a table row).
func TestRender_Pi_ByteForByteCommitPi(t *testing.T) {
	spec, err := builtinPi().Render("", "zai", "<sys>", "<user>")
	if err != nil { t.Fatalf("Render: %v", err) }
	wantArgs := []string{"--provider","zai","--model","glm-5-turbo","--system-prompt","<sys>",
		"--no-tools","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session","-p"}
	if spec.Command != "pi" || !reflect.DeepEqual(spec.Args, wantArgs) || spec.Stdin != "<user>" {
		t.Errorf("pi not byte-for-byte commit-pi:\n Command=%q\n Args=%v\n Stdin=%q",
			spec.Command, spec.Args, spec.Stdin)
	}
}

// 3. System-prompt-prepend fallback: no sys flag → sys prepended to payload (delimiter "\n\n"); with flag → not prepended.
func TestRender_SystemPromptPrependFallback(t *testing.T) {
	// gemini has NO sys flag (SystemPromptFlag resolves to "") → sys prepended to stdin payload.
	got, _ := builtinGemini().Render("", "", "SYS", "USER")
	if got.Stdin != "SYS\n\nUSER" { t.Errorf("gemini prepend: Stdin = %q, want SYS\\n\\nUSER", got.Stdin) }
	// pi HAS a sys flag → sys via flag, payload = user only.
	got2, _ := builtinPi().Render("", "zai", "SYS", "USER")
	if got2.Stdin != "USER" { t.Errorf("pi (sys flag): Stdin = %q, want USER", got2.Stdin) }
	// Empty sys + no flag → no prepend (no leading newlines).
	got3, _ := builtinGemini().Render("", "", "", "USER")
	if got3.Stdin != "USER" { t.Errorf("empty sys: Stdin = %q, want USER", got3.Stdin) }
}

// 4. model default fallback: model="" → DefaultModel (pi→glm-5-turbo, gemini→gemini-2.5-pro); explicit wins.
func TestRender_ModelDefaultFallback(t *testing.T) {
	byDefault, _ := builtinPi().Render("", "zai", "", "")   // model="" → glm-5-turbo
	if !containsPair(byDefault.Args, "--model","glm-5-turbo") { t.Errorf("model default not applied: %v", byDefault.Args) }
	explicit, _ := builtinPi().Render("custom-model", "zai", "", "") // explicit wins
	if !containsPair(explicit.Args, "--model","custom-model") { t.Errorf("explicit model lost: %v", explicit.Args) }
}

// 5. provider default fallback: provider="" → DefaultProvider (all built-ins "" → no --provider); a §12.8 default honored.
func TestRender_ProviderDefaultFallback(t *testing.T) {
	got, _ := builtinPi().Render("", "", "", "USER") // provider="" → DefaultProvider="" → no --provider
	for i, a := range got.Args { if a == "--provider" { t.Errorf("unexpected --provider at %d: %v", i, got.Args) } }
	// A §12.8 user manifest with default_provider="zai" + provider_flag → honored when caller passes "".
	// NOTE: Name is REQUIRED (Validate rejects an empty Name) — always set it on hand-built manifests.
	user := Manifest{Name: "myagent", Command: strPtr("agent"), ProviderFlag: strPtr("--provider"), DefaultProvider: strPtr("zai")}
	got2, err := user.Render("", "", "", "USER")
	if err != nil { t.Fatalf("user manifest Render: %v (did you forget Name?)", err) }
	if !containsPair(got2.Args, "--provider","zai") { t.Errorf("default_provider not honored: %v", got2.Args) }
}

// 6. Env: os.Environ() + manifest entries as "KEY=VAL"; manifest wins on collision; membership-only assertions.
func TestRender_Env(t *testing.T) {
	osEnvLen := len(os.Environ())
	// NOTE: Name is REQUIRED (Validate rejects an empty Name) — always set it on hand-built manifests.
	m := Manifest{Name: "pi", Command: strPtr("pi"), Env: map[string]string{"PI_OFFLINE": "1", "DEBUG": "x"}}
	spec, err := m.Render("", "", "", "USER")
	if err != nil { t.Fatalf("Render: %v", err) }
	if len(spec.Env) < osEnvLen+2 { t.Errorf("Env len = %d, want >= %d", len(spec.Env), osEnvLen+2) }
	set := map[string]bool{}
	for _, e := range spec.Env { set[e] = true }
	if !set["PI_OFFLINE=1"] { t.Errorf("manifest env PI_OFFLINE=1 missing: %v", spec.Env) }
	if !set["DEBUG=x"] { t.Errorf("manifest env DEBUG=x missing: %v", spec.Env) }
}

// 7. flag delivery: payload appended as (prompt_flag, payload).
func TestRender_FlagDelivery(t *testing.T) {
	// NOTE: Name is REQUIRED (Validate rejects an empty Name) — always set it on hand-built manifests.
	m := Manifest{Name: "flagagent", Command: strPtr("agent"), PromptDelivery: strPtr("flag"), PromptFlag: strPtr("--prompt")}
	spec, err := m.Render("", "", "", "PAYLOAD")
	if err != nil { t.Fatalf("flag manifest Render: %v (did you forget Name?)", err) }
	if !containsPair(spec.Args, "--prompt","PAYLOAD") { t.Errorf("flag delivery: %v", spec.Args) }
	if spec.Stdin != "" { t.Errorf("flag delivery Stdin = %q, want empty", spec.Stdin) }
}

// 8. Validate error propagation: missing Command → error; invalid prompt_delivery → error.
func TestRender_ValidateErrors(t *testing.T) {
	if _, err := (Manifest{}).Render("", "", "", "U"); err == nil { t.Error("want error for empty manifest (no command)") }
	if _, err := (Manifest{Name: "x", Command: strPtr("pi"), PromptDelivery: strPtr("bogus")}).Render("", "", "", "U"); err == nil {
		t.Error("want error for invalid prompt_delivery")
	}
}

// 9. Resolve is non-mutating: the caller's manifest is untouched after Render.
func TestRender_DoesNotMutateManifest(t *testing.T) {
	m := builtinPi()
	origSys := m.SystemPromptFlag
	_, _ = m.Render("", "zai", "<sys>", "<user>")
	if m.SystemPromptFlag != origSys { t.Errorf("Render mutated SystemPromptFlag: %v vs %v", m.SystemPromptFlag, origSys) }
}

// 10. (optional) Render is byte-compatible with the test-only renderArgs for the flags portion.
func TestRender_CompatWithRenderArgs(t *testing.T) {
	// renderArgs returns Command as element[0]; CmdSpec splits Command out. Same tokens, same order.
	flags := renderArgs(builtinCodex(), "", "gpt-5", "<sys>")
	spec, _ := builtinCodex().Render("gpt-5", "", "<sys>", "<user>")
	if spec.Command != flags[0] { t.Errorf("Command mismatch: %q vs %q", spec.Command, flags[0]) }
	if !reflect.DeepEqual(spec.Args, flags[1:]) { t.Errorf("Args != renderArgs flags:\n got %v\nwant %v", spec.Args, flags[1:]) }
}

// --- helpers ---
func containsPair(args []string, flag, val string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == val { return true }
	}
	return false
}
```

> **Note on Test 10:** `renderArgs` is an unexported test helper in `builtin_test.go` (same package), so
> `render_test.go` can call it (same package). If you prefer independence, drop Test 10 — the golden table
> (Test 1) already pins the exact Args. Test 10 is a nice "compatibility regression" guard but optional.

### Implementation Patterns & Key Details

```go
// The §12.2 token-order builder — the heart of Render. append(nil, x...) is safe (absent slices no-op).
args := make([]string, 0, 16)
args = append(args, r.Subcommand...)
if *r.ProviderFlag != "" && providerToUse != "" { args = append(args, *r.ProviderFlag, providerToUse) }
if *r.ModelFlag != "" && modelToUse != ""        { args = append(args, *r.ModelFlag, modelToUse) }
if *r.SystemPromptFlag != "" && sysPrompt != ""  { args = append(args, *r.SystemPromptFlag, sysPrompt) }
args = append(args, r.BareFlags...)
if *r.PrintFlag != "" { args = append(args, *r.PrintFlag) } // LAST per §12.2

// The unified payload + sys-prepend fallback (ONE rule, all delivery modes). Delimiter EXACTLY "\n\n".
payload := userPayload
if *r.SystemPromptFlag == "" && sysPrompt != "" {
	payload = sysPrompt + "\n\n" + userPayload
}
switch *r.PromptDelivery {
case "stdin":      spec.Stdin = payload
case "positional": spec.Args = append(spec.Args, payload)
case "flag":       spec.Args = append(spec.Args, *r.PromptFlag, payload)
default:           return nil, fmt.Errorf("...: unsupported prompt_delivery %q", m.Name, *r.PromptDelivery)
}

// Env — os.Environ() first, manifest entries last (exec last-wins → manifest overrides parent).
env := os.Environ()
for k, v := range r.Env { env = append(env, k+"="+v) }
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. Render uses ONLY stdlib (fmt, os). `go mod tidy` MUST be a no-op.
        `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - internal/provider → (stdlib: fmt, os) ONLY. It does NOT import os/exec (spawning is the executor T5),
        go-toml (manifests are already decoded), or internal/config (cycle; unneeded).

FROZEN FILES (do NOT edit):
  - internal/provider/manifest.go + manifest_test.go (S1): Manifest + Validate + Resolve + DetectCommand
        + strPtr/boolPtr are a CONTRACT (Render CALLS Validate+Resolve; it does not redefine them).
  - internal/provider/merge.go + merge_test.go (S2), builtin.go + builtin_test.go (S2/S3): the 6 built-ins
        Render tests iterate; renderArgs (test helper) is reused by Test 10 only (read-only call).
  - internal/provider/registry.go + registry_test.go (P1.M2.T3.S1, parallel): may or may not be present
        when Render starts — Render does NOT depend on the registry (it takes a Manifest directly). Keep
        both green.
  - internal/config/*, internal/git/*, cmd/stagecoach/main.go, Makefile.

DOWNSTREAM CONTRACTS (do NOT implement here — just honor the shapes they will consume):
  - P1.M2.T5 (executor): exec.Command(spec.Command, spec.Args...); cmd.Stdin = (spec.Stdin != "") ?
        strings.NewReader(spec.Stdin) : <os.DevNull>; cmd.Env = spec.Env. The CmdSpec shape is now FROZEN.
  - P1.M3.T4 (generate flow): m, _ := reg.Get(cfg.Provider); spec, err := m.Render(model, provider, sys,
        user); hand spec to the executor. (Render's Validate covers the registry's unresolved output.)
  - P1.M4.T3.S2 (verbose mode): print spec.Command + " " + strings.Join(spec.Args, " ") for debugging.
  => CmdSpec {Command, Args, Stdin, Env} + Manifest.Render(...) are now FROZEN for downstream. Do not
     change signatures after this subtask.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after creating render.go — fix before proceeding
gofmt -w internal/provider/render.go internal/provider/render_test.go   # format (idempotent)
go vet ./internal/provider/                                             # catches unused imports (e.g. stray "strings")
go build ./...                                                          # compiles the whole module

# go.mod/go.sum MUST be unchanged (stdlib only)
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged ✓"

# Expected: Zero errors. If `go vet` flags an unused import, remove it (do NOT import strings/os/exec
# unless actually used — Render uses `fmt` + `os` only).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The renderer + its tests
go test -race ./internal/provider/ -run 'Render' -v

# The full provider suite (S1/S2/S3 + parallel registry + new render tests — all must stay green)
go test -race ./internal/provider/ -v

# Expected: all green. The keystone TestRender_GoldenPerProvider + TestRender_Pi_ByteForByteCommitPi
# MUST pass. If a golden Args mismatch, re-read research §3 + the builtin manifest values (the REVISED
# gemini/codex stdin delivery + codex --ephemeral are the most likely surprises).
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-module test suite (provider + config + git + cmd) — nothing else may regress
go test -race ./...

# Coverage for the renderer (informational; the Makefile coverage gate ≥85% is a P1.M5 concern)
go test -coverprofile=coverage.out ./internal/provider/ && go tool cover -func=coverage.out | grep -E 'render.go|Render'

# Expected: `go test -race ./...` green; render.go functions covered (Render's happy path + each delivery
# branch + the Validate-error path all exercised by the ~10 test groups).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# (No subprocess spawning in this subtask — Render returns pure data. The executor T5 owns spawning.)
# Manual reasoning check: for pi, the rendered Args + Stdin reconstruct the commit-pi invocation:
#   pi --provider zai --model glm-5-turbo --system-prompt "<sys>" \
#      --no-tools --no-extensions --no-skills --no-prompt-templates \
#      --no-context-files --no-session -p  <  <user payload via stdin>
# (The golden tests assert this byte-for-byte; this is a human eyeball confirmation of the same.)
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed; `go test -race ./...` green.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/provider/` clean.
- [ ] `go mod tidy` is a no-op; `git diff --exit-code go.mod go.sum` empty (stdlib only).
- [ ] Every frozen file byte-unchanged (`git diff --exit-code internal/provider/manifest.go internal/provider/merge.go internal/provider/builtin.go` and their `_test.go`).

### Feature Validation

- [ ] All success criteria from "What" section met.
- [ ] `TestRender_GoldenPerProvider` passes — all 6 providers render their exact §12.3–§12.7 Args.
- [ ] `TestRender_Pi_ByteForByteCommitPi` passes — pi is byte-for-byte the commit-pi invocation.
- [ ] print_flag is LAST for every provider (claude + cursor included — NOT the narrative's -p-first order).
- [ ] The sys-prepend fallback produces `"<sys>\n\n<user>"` for no-sys-flag agents (gemini/opencode/codex/cursor).
- [ ] Env carries os.Environ() + manifest entries; manifest wins on collision (membership-asserted).

### Code Quality Validation

- [ ] Follows existing codebase patterns (stdlib testing, reflect.DeepEqual, t.Errorf idiom).
- [ ] File placement matches the desired tree (new render.go + render_test.go ONLY).
- [ ] Anti-patterns avoided (no raw-pointer deref before Resolve; no os/exec import; no full-Env equality assertions).
- [ ] Imports are exactly `fmt`, `os` (no unused imports).

### Documentation & Deployment

- [ ] Code is self-documenting (doc comments on CmdSpec + Render citing PRD §12.2 + the design calls).
- [ ] No new env vars or config (Render is pure over its inputs modulo os.Environ()).

---

## Anti-Patterns to Avoid

- ❌ Don't edit `manifest.go` (FROZEN by S1 + the parallel registry PRP) — put Render + CmdSpec in a NEW
  `render.go` (same package → still a method of Manifest).
- ❌ Don't reorder tokens to match the §12.4/§12.7 narrative "Rendered:" blocks — §12.2 is authoritative;
  print_flag is LAST (the pi byte-for-byte test + the cursor test comment confirm it).
- ❌ Don't deref raw `m` pointers before `Resolve()` (a nil SystemPromptFlag/Command would panic).
- ❌ Don't skip `Validate()` inside Render (the work item requires "Validate prompt_delivery mode"; the
  registry hands Render merged-but-unresolved manifests).
- ❌ Don't set `spec.Stdin` for positional/flag delivery (Stdin="" is the "no stdin pipe" signal for the
  executor).
- ❌ Don't assert full `spec.Env` equality in tests (os.Environ() is machine-dependent — use membership).
- ❌ Don't filter empty strings out of `BareFlags` (claude's `--tools ""` / `--setting-sources ""` rely on
  the empty value tokens).
- ❌ Don't import `os/exec` (spawning is the executor T5's job) or `go-toml` (manifests are already decoded).
- ❌ Don't catch all exceptions / swallow the Validate error — surface it wrapped with the provider name.
