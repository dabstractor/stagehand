---
name: "P1.M1.T1.S3 — Add RenderMultiTurn sibling method (drop --no-session, add --session-id, sys prompt turn-1-only)"
description: |
  Add a SIBLING method `func (m Manifest) RenderMultiTurn(model, sysPrompt, userPayload, reasoning, sessionID string, turn int) (*CmdSpec, error)`
  to `internal/provider/render.go` (alongside `Render`) that renders ONE turn of the multi-turn generation
  fallback (PRD §9.24 FR-T6) against an existing session id. ⚠️ Do NOT widen `Render`'s signature (blast
  radius 24+ call sites) and do NOT add a `RenderMode` value (the variadic `mode ...RenderMode` carries no
  place for a session id) — research-provider.md §2 Option B (sibling method) is the chosen shape. S1 is
  LANDED (`Manifest.SessionMode *string` + Resolve default `""` + Validate enum), so the capability gate
  `*r.SessionMode == "append"` is safe to deref and compiles TODAY; S3 is code-independent of S2/S4. The
  method rebuilds the §12.2 argv EXACTLY as Render does, with three multi-turn deltas: (1) a capability
  gate after Resolve; (2) turn-1-only system prompt via a `turnSys` local (= sysPrompt if turn==1 else "");
  (3) the bare-flags block becomes BareFlags MINUS the exact token "--no-session", PLUS "--session-id",
  sessionID (fresh slice — never mutate r.BareFlags). The print_flag stays LAST; the payload+delivery switch
  is identical to Render. On turns > 1 no system_prompt_flag is emitted AND the payload is not prepended
  with sys (the session carries it). FR-T9 verified for pi (2026-07-05): --no-session dropped, --session-id
  added, -p kept, --continue/-c NOT used. S3 touches ONLY render.go + its test. NO mocking (pure data
  transformation; os.Environ() is the sole side effect, matching Render). NO docs (rides with S5).
---

## Goal

**Feature Goal**: Provide the per-turn command renderer the multi-turn fallback protocol (P1.M1.T3.S2) will
call N+1 times — one `RenderMultiTurn` call per turn — producing a `*CmdSpec` whose argv drops `--no-session`,
appends `--session-id <id>`, and emits the system prompt on turn 1 only, for a provider whose manifest
declares `session_mode = "append"` (pi). Render and every existing caller stay byte-for-byte unchanged.

**Deliverable**: A new `RenderMultiTurn` method in `internal/provider/render.go` (appended after `Render`),
plus 4 unit tests in `internal/provider/render_test.go` (golden turn-1, turn-2-no-sys, capability-gate
error, manifest-immutability). No other file changes.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green;
the golden turn-1 test pins the FR-T9-verified pi argv (`--no-session` absent, `--session-id <id>` present
before `-p`, `--system-prompt <sys>` present on turn 1); the turn-2 test proves no `--system-prompt` and no
sys-prepend; the gate test proves a non-append provider errors; the immutability test proves `m.BareFlags`
is untouched. `Render` and its 24+ call sites are unchanged (`git diff` on render.go shows only an ADDITION
after Render's closing brace). `git diff --stat` shows ONLY render.go + render_test.go.

## User Persona

**Target User**: The contributor implementing the multi-turn fallback protocol (P1.M1.T3.S2 — the N+1 turn
loop that calls `RenderMultiTurn` per turn) and the integration tests (P1.M1.T4); and the end user with a
very large diff whose one-shot generation repeatedly fails on a session-capable provider (pi).

**Use Case**: P1.M1.T3.S2 mints a session id (`stagecoach-<run-uuid>`), then for turn 1 calls
`RenderMultiTurn(model, sys, chunk1, reasoning, sessionID, 1)`; for turns 2..N `RenderMultiTurn(model, "",
chunkI, reasoning, sessionID, i)`; for turn N+1 `RenderMultiTurn(model, "", finalPrompt, reasoning,
sessionID, N+1)`. Each call yields the argv the executor (P1.M2.T5) runs against the SAME session id,
appending a recallable turn.

**Pain Points Addressed**: Without S3 there is no way to render a multi-turn invocation — `Render` always
emits `--no-session` (ephemeral) and has no session-id parameter, so the fallback protocol cannot construct
its per-turn commands. S3 is the renderer that makes the protocol possible.

## Why

- **Required by the multi-turn fallback (§9.24).** FR-T6 specifies the turn protocol against ONE session id,
  with turn-1 system prompt and a dropped `--no-session`. The renderer must produce that argv; S3 is it.
- **Sibling method protects the 24+ Render call sites.** Widening `Render`'s signature (adding a sessionID
  param) would break every caller (generate.go, pkg/stagecoach, hook/exec, decompose roles, tests); a new
  `RenderMode` value cannot carry the session-id string through the variadic `mode ...RenderMode`. The
  sibling is the only shape that keeps Render byte-identical — the same pattern used when `RenderTooled`
  was added (research §2).
- **Capability gate enforces FR-T8/T9.** Only a verified-append provider (pi) may multi-turn. The gate
  `*r.SessionMode != "append"` → error makes it impossible to render a multi-turn turn for a provider
  whose append mechanism is unverified, so the trigger (FR-T1 condition d) and the renderer agree.
- **FR-T9 verified, not speculative.** The exact flag transformation (BareFlags − `--no-session` +
  `--session-id <id>`, sys prompt turn-1-only, `-p` kept, no `--continue`) was confirmed by a live pi run
  (2026-07-05, fr-t9-verification.md). S3 encodes that verified transformation; it does not invent flags.
- **Pure data transformation — no mocking surface.** Like Render, RenderMultiTurn performs no spawning
  (the sole side effect is `os.Environ()` for Env). It is fully unit-testable with Manifest literals.

## What

One new method (`RenderMultiTurn`) appended after `Render` in `internal/provider/render.go`, plus 4 tests.
The method mirrors Render's §12.2 argv construction with three deltas (capability gate; turn-1-only sys via
a `turnSys` local; session-flags block = filtered BareFlags + `--session-id`). Render itself is NOT modified.

### Success Criteria

- [ ] `func (m Manifest) RenderMultiTurn(model, sysPrompt, userPayload, reasoning, sessionID string, turn int) (*CmdSpec, error)` exists in render.go after `Render`.
- [ ] It calls `m.Validate()` then `m.Resolve()` (like Render), then the capability gate `if *r.SessionMode != "append" { return nil, fmt.Errorf("provider %q: multi-turn render requires session_mode=\"append\"", m.Name) }`.
- [ ] The argv order matches §12.2 / Render EXACTLY: subcommand → provider_flag/model slash-split → model_flag → reasoning → system_prompt_flag (turn==1 only) → session-flags block → print_flag (LAST) → payload-by-delivery.
- [ ] The session-flags block is a FRESH slice: `r.BareFlags` with the exact token `"--no-session"` filtered out, then `"--session-id", sessionID` appended. `r.BareFlags` is NOT mutated.
- [ ] On turn==1 with sysPrompt!="" the system_prompt_flag is emitted; on turn>1 it is NOT (turnSys local).
- [ ] On turn>1 the payload is NOT prepended with sysPrompt (the prepend-fallback guard keys on turnSys, which is "" for turn>1).
- [ ] The model slash-split (FR-R5b), reasoning tokens (FR-R6), print_flag-LAST, delivery switch, and `Env = os.Environ() + manifest Env` are identical to Render.
- [ ] The 4 unit tests pass (golden turn-1, turn-2-no-sys, gate error, immutability).
- [ ] `Render` and its 24+ call sites are byte-identical (`git diff internal/provider/render.go` shows only the new method added after Render's brace).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] ONLY `internal/provider/render.go` + `internal/provider/render_test.go` change.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the EXACT current `Render` method (verbatim, render.go:89-159 — the
pipeline to mirror), the EXACT `RenderMultiTurn` target body (ready to paste, with the three deltas), the
verified FR-T9 pi argv (the golden turn-1 args), the turn-2 expected args, the capability-gate error
message verbatim, the test idiom (`reflect.DeepEqual` golden + `containsToken`/`containsPair` helpers +
`TestRender_Pi_ByteForByteCommitPi`/`TestRender_DoesNotMutateManifest` templates), and the verified S1-landed
state (`SessionMode *string` + Resolve default). The turn-1-only insight (a single `turnSys` local makes
both the flag guard and the prepend guard turn-correct) is the key non-obvious detail.

### Documentation & References

```yaml
# MUST READ — the Render pipeline to mirror + the sibling-method decision
- docfile: plan/009_5c53066d64b3/architecture/research-provider.md
  why: "§2 gives Render's exact signature (render.go:89), the §12.2 args order (render.go:96-143), the mode ternary (render.go:124-136), and the two options for a multi-turn variant. Option A (new RenderMode value) is REJECTED (variadic mode carries no session-id slot); Option B (sibling method) is chosen. §2 also confirms there is NO existing mechanism to drop a specific bare_flag token — the renderer treats BareFlags as opaque, so S3 must filter by exact token '--no-session'."
  critical: "The sibling-method shape is the authority. Do NOT widen Render's signature and do NOT add a RenderMode value. The '--no-session' filter is exact-token (pi-only-shipped value, FR-T9 verified)."

- docfile: plan/009_5c53066d64b3/architecture/fr-t9-verification.md
  why: "Records the VERIFIED pi flag set (2026-07-05 live run): BareFlags MINUS --no-session, PLUS --session-id <id>; --system-prompt on turn 1 only; -p kept; --continue/-c NOT used. Gives the exact turn-1 rendering and the recall-proven verdict."
  critical: "This is the FR-T9 verification S3's transformation encodes. The golden turn-1 args in the PRP are derived verbatim from this file. The render contract section states the position constraint (--session-id before -p is acceptable; the stub test asserts presence/absence)."

- docfile: plan/009_5c53066d64b3/P1M1T1S1/PRP.md
  why: "S1 LANDED SessionMode *string (manifest.go:66) + Resolve default strPtr('') (manifest.go:177-178) + Validate ''|'append' enum (manifest.go:121-123). S3's capability gate derefs *r.SessionMode — safe because Resolve guarantees non-nil. S1's downstream-hook note literally specifies S3's gate."
  critical: "S1 is LANDED — S3 compiles against the real tree. S3 does NOT re-edit manifest.go. The gate is *r.SessionMode (the RESOLVED value), checked AFTER Resolve()."

- docfile: plan/009_5c53066d64b3/P1M1T1S2/PRP.md
  why: "S2 (in parallel) adds the MergeManifest clause making SessionMode config-overridable. S3 is CODE-INDEPENDENT of S2: RenderMultiTurn reads *r.SessionMode off whatever manifest it's handed; its UNIT TESTS set SessionMode directly in a Manifest literal (no registry/merge path). S2/S4 make the gate ever pass in production; S3's logic is correct regardless."
  critical: "Do NOT block S3 on S2. Do NOT edit merge.go (S2) or builtin.go/pi.toml (S4) in S3. The unit-test Manifest literal sets SessionMode: strPtr('append') directly."

# The file under edit
- file: internal/provider/render.go
  why: "EDIT (1 addition). Append RenderMultiTurn immediately AFTER Render's closing brace (render.go:159). Do NOT modify Render or any other function. The method mirrors Render's pipeline (Validate → Resolve → model fallback → args build → payload/delivery switch → env) with the 3 deltas (gate, turnSys, session-flags block)."
  pattern: "Copy Render's body (render.go:91-159) and apply the deltas: (a) insert the capability gate after r := m.Resolve(); (b) add turnSys local and use it in the sys-flag guard + prepend-fallback guard; (c) replace the mode ternary with the session-flags block (filtered BareFlags + --session-id). Drop the `mode ...RenderMode` parameter (the sibling takes sessionID + turn instead)."
  gotcha: "(1) Build a FRESH sessionArgs slice for the filtered block — range over r.BareFlags, never assign into it (immutability). (2) turnSys local is the single change that makes BOTH the sys-flag guard and the prepend guard turn-correct — do not inline two separate turn checks. (3) Keep print_flag LAST (after the session-flags block). (4) The model slash-split HARD-ERROR path is identical to Render (FR-R5b) — copy it verbatim. (5) Env = os.Environ() + manifest Env, identical to Render (the sole side effect)."

- file: internal/provider/render_test.go
  why: "EDIT (4 new tests). Reuses the existing helpers containsToken/containsPair + the reflect.DeepEqual golden idiom. TestRender_Pi_ByteForByteCommitPi (render_test.go:90) is the golden template; TestRender_DoesNotMutateManifest (render_test.go:271) is the immutability template; TestRender_FR5b_RejectsBareModelOnMultiProvider (render_test.go:231) is the error-template. Build a pi-shape Manifest literal with SessionMode: strPtr('append')."
  pattern: "wantArgs := []string{...}; spec, err := m.RenderMultiTurn(...); if err != nil { t.Fatal(err) }; if !reflect.DeepEqual(spec.Args, wantArgs) { t.Errorf(...) }. For the gate test: m with SessionMode absent/'' → expect err != nil mentioning 'session_mode'. For immutability: snapshot m.BareFlags, call RenderMultiTurn, assert m.BareFlags unchanged (still contains '--no-session')."
  gotcha: "The golden turn-1 args EXCLUDE '--no-session' and INCLUDE '--session-id','<id>' BEFORE '-p'. The turn-2 args are identical MINUS '--system-prompt','<sys>'. Stdin (stdin delivery) is the payload ONLY on both turns (no sys prepend — sys goes via the flag on turn 1, and is absent on turn 2). Use a distinct sessionID literal (e.g. 'stagecoach-test') so the assertion is exact."

# Read-only refs (do NOT edit in S3)
- file: internal/provider/manifest.go
  why: "READ-ONLY (S1 landed). SessionMode *string (line 66); Resolve strPtr('') default (177-178); Validate enum (121-123). S3 derefs *r.SessionMode — do NOT change the field."
- file: internal/provider/merge.go
  why: "READ-ONLY (S2). The MergeManifest SessionMode clause (in parallel) is what makes a config override reach the registry. S3 reads the resolved value; it does not merge."
- file: internal/provider/builtin.go + providers/pi.toml
  why: "READ-ONLY (S4). The shipped pi SessionMode='append' value (FR-T9 verified). S3's unit tests set SessionMode in a literal; the shipped value is S4's job."

# PRD authority (already in the selected content)
- prd: PRD.md §9.24 FR-T6 (turn protocol: turn-1 sys prompt via system_prompt_flag, session carries it; --no-session dropped, --session-id added; --continue/-c NOT used), FR-T8 (session_mode field: "" default | "append"), FR-T9 (verification duty), FR-T10 (message-role scope); §12.2 (token order); §12.1 (session_mode field).
  why: "FR-T6 is the turn-rendering contract S3 implements. FR-T8/T9 is the capability gate. §12.2 is the argv order RenderMultiTurn mirrors. The turn-1-only system prompt and the --no-session/--session-id swap are FR-T6's exact requirements."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/provider/
    ├── render.go         # EDIT: + RenderMultiTurn method (after Render)
    ├── render_test.go    # EDIT: + 4 RenderMultiTurn tests
    ├── manifest.go       # READ-ONLY (S1): SessionMode *string + Resolve + Validate
    ├── merge.go          # READ-ONLY (S2): MergeManifest clause (parallel)
    └── builtin.go        # READ-ONLY (S4): pi SessionMode="append" (parallel/later)
```

### Desired Codebase Tree After S3

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/provider/render.go          # +RenderMultiTurn (appended after Render); Render unchanged
    internal/provider/render_test.go     # +4 tests
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/provider/render.go` | MODIFY (append only) | Add `RenderMultiTurn` after `Render`. **Render and every other function unchanged.** |
| `internal/provider/render_test.go` | MODIFY (append only) | Add 4 `TestRenderMultiTurn_*` tests. **Existing tests unchanged.** |

**Explicitly NOT touched**: `Render` (byte-identical), `manifest.go` (S1 — landed), `merge.go` (S2),
`builtin.go`/`providers/pi.toml` (S4 — the shipped pi `"append"` value), docs (S5 — render-behavior doc rides
with S5), `multiturn.go`/`generate.go` (P1.M1.T3 — the N+1 protocol that CALLS RenderMultiTurn), any other
package, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (sibling, NOT a Render signature widening): the variadic `mode ...RenderMode` carries NO place
// for a session-id string. Widening Render breaks 24+ call sites; a new RenderMode value can't carry the id.
// RenderMultiTurn is a SEPARATE method with its own (sessionID, turn) params. Render stays byte-identical.

// CRITICAL (capability gate AFTER Resolve): `r := m.Resolve()` guarantees *r.SessionMode is non-nil (S1
// Resolve default ""). Dereference *r.SessionMode only after Resolve(). The gate error is exactly:
//   fmt.Errorf("provider %q: multi-turn render requires session_mode=\"append\"", m.Name)

// CRITICAL (turn-1-only sys via ONE local): introduce `turnSys := sysPrompt; if turn != 1 { turnSys = "" }`
// and use turnSys in BOTH the sys-flag emission guard AND the prepend-fallback guard. This single local
// makes turn>1 emit no --system-prompt AND prepend nothing (the session carries it) — do not write two
// separate turn checks (they'd drift). The prepend guard is `if *r.SystemPromptFlag == "" && turnSys != ""`.

// CRITICAL (fresh slice, never mutate): build `sessionArgs := make([]string, 0, len(r.BareFlags)+2)`,
// range over r.BareFlags skipping the exact token "--no-session", then append "--session-id", sessionID.
// Do NOT assign into r.BareFlags or re-slice it. (r is a Resolve() copy, but r.BareFlags may share the
// caller's underlying array — a fresh slice is the only safe, provably-non-mutating approach.)

// GOTCHA (exact-token filter): filter by `f == "--no-session"` (the EXACT token). Only pi ships this token
// today; a provider without it is unaffected (its BareFlags pass through, then --session-id appends). Do NOT
// filter by prefix or by index — the manifest's BareFlags order is not guaranteed across providers.

// GOTCHA (print_flag LAST): --session-id <id> goes BEFORE -p (print_flag). §12.2 mandates print_flag is
// ALWAYS the last flag; the FR-T9 verified rendering confirms --session-id precedes -p.

// GOTCHA (FR-R5b HARD ERROR identical to Render): a provider_flag provider (pi) given a bare model (no "/")
// returns the same error as Render. Copy that branch verbatim — do not weaken or drop it.

// GOTCHA (no mocking): RenderMultiTurn performs NO spawning. The sole side effect is os.Environ() for Env,
// identical to Render. Unit tests need no stubs/processes — Manifest literals + reflect.DeepEqual suffice.

// GOTCHA (Env identical to Render): env := os.Environ(); for k,v := range r.Env { env = append(env, k+"="+v) }.
// Manifest Env entries appended last → exec last-wins → override. Copy verbatim.

// GOTCHA (S1 landed, S2/S4 independent): S3 compiles against the real tree (SessionMode exists). S3's unit
// tests set SessionMode: strPtr("append") in a Manifest literal — they do NOT depend on S2 (merge) or S4
// (builtin pi value). Do NOT edit merge.go/builtin.go/pi.toml in S3.
```

## Implementation Blueprint

### Data models and structure

No new types. `RenderMultiTurn` returns the existing `*CmdSpec` (render.go:13-27). It reuses the existing
`Manifest`, `RenderMode` (untouched), and `strPtr`/`boolPtr` helpers. The relevant existing precedent (the
model to mirror — unchanged) is `Render` (render.go:89-159).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: render.go — append RenderMultiTurn after Render
  - LOCATE: internal/provider/render.go, Render's closing brace (render.go:159). Append the new method
    immediately after it (before EOF / the package's next section if any).
  - PASTE the method body (ready-to-paste — see "Implementation Patterns" below). Structure:
      func (m Manifest) RenderMultiTurn(model, sysPrompt, userPayload, reasoning, sessionID string, turn int) (*CmdSpec, error) {
          // 1. Validate + Resolve (identical to Render)
          if err := m.Validate(); err != nil {
              return nil, fmt.Errorf("provider render %q: %w", m.Name, err)
          }
          r := m.Resolve()
          // 2. Capability gate (FR-T8/T9) — AFTER Resolve so *r.SessionMode is non-nil
          if *r.SessionMode != "append" {
              return nil, fmt.Errorf("provider %q: multi-turn render requires session_mode=\"append\"", m.Name)
          }
          // 3. model default fallback (identical to Render)
          modelToUse := model
          if modelToUse == "" { modelToUse = *r.DefaultModel }
          // 4. FR-T6 turn-1-only system prompt (the single local that makes both guards turn-correct)
          turnSys := sysPrompt
          if turn != 1 { turnSys = "" }
          // 5. args build (subcommand → FR-R5b fold → model_flag → reasoning → sys flag → session block → print_flag)
          args := make([]string, 0, 16)
          args = append(args, r.Subcommand...)
          // FR-R5b fold — IDENTICAL to Render (bare model on pi = HARD ERROR)
          if *r.ProviderFlag != "" && modelToUse != "" {
              if i := strings.Index(modelToUse, "/"); i >= 0 {
                  args = append(args, *r.ProviderFlag, modelToUse[:i])
                  modelToUse = modelToUse[i+1:]
              } else {
                  return nil, fmt.Errorf("provider render %q: model %q on %s must be inference/model, e.g. \"zai/glm-5.2\"", m.Name, modelToUse, m.Name)
              }
          }
          if *r.ModelFlag != "" && modelToUse != "" { args = append(args, *r.ModelFlag, modelToUse) }
          // FR-R6 reasoning — IDENTICAL to Render (silent no-op)
          if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0 { args = append(args, r.ReasoningLevels[reasoning]...) }
          // FR-T6 turn-1-only system prompt flag (turnSys, NOT sysPrompt)
          if *r.SystemPromptFlag != "" && turnSys != "" { args = append(args, *r.SystemPromptFlag, turnSys) }
          // FR-T6 session-flags block: BareFlags MINUS "--no-session" + "--session-id", sessionID (FRESH slice)
          sessionArgs := make([]string, 0, len(r.BareFlags)+2)
          for _, f := range r.BareFlags {
              if f == "--no-session" { continue }
              sessionArgs = append(sessionArgs, f)
          }
          sessionArgs = append(sessionArgs, "--session-id", sessionID)
          args = append(args, sessionArgs...)
          // print_flag LAST (identical to Render)
          if *r.PrintFlag != "" { args = append(args, *r.PrintFlag) }
          // 6. unified payload + prepend fallback keyed on turnSys (identical shape to Render)
          payload := userPayload
          if *r.SystemPromptFlag == "" && turnSys != "" { payload = turnSys + "\n\n" + userPayload }
          // 7. delivery switch (identical to Render)
          spec := &CmdSpec{Command: *r.Command, Args: args}
          switch *r.PromptDelivery {
          case "stdin":      spec.Stdin = payload
          case "positional": spec.Args = append(spec.Args, payload)
          case "flag":       spec.Args = append(spec.Args, *r.PromptFlag, payload)
          default:           return nil, fmt.Errorf("provider render %q: unsupported prompt_delivery %q", m.Name, *r.PromptDelivery)
          }
          // 8. Env (identical to Render — sole side effect: os.Environ())
          env := os.Environ()
          for k, v := range r.Env { env = append(env, k+"="+v) }
          spec.Env = env
          return spec, nil
      }
  - DOC COMMENT: above the method, state it is a sibling of Render (not a mode), the 3 deltas, the FR-T6
    turn-1-only rule, and the FR-T9 verification reference (see "Implementation Patterns" for the comment).
  - DO NOT: modify Render; widen Render's signature; add a RenderMode value; mutate r.BareFlags; reorder
    print_flag (it stays LAST); drop the FR-R5b hard-error branch.

Task 2: render_test.go — TestRenderMultiTurn_PiTurn1_Golden (the byte-for-byte FR-T9 pin)
  - BUILD a pi-shape Manifest literal: Name "pi", Command strPtr("pi"), ProviderFlag strPtr("--provider"),
    ModelFlag strPtr("--model"), SystemPromptFlag strPtr("--system-prompt"), PrintFlag strPtr("-p"),
    PromptDelivery strPtr("stdin"), SessionMode strPtr("append"), BareFlags []string{"--no-tools",
    "--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session"}.
  - CALL: spec, err := m.RenderMultiTurn("zai/glm-5.2", "<sys>", "<payload>", "", "stagecoach-test", 1).
  - ASSERT (reflect.DeepEqual, the load-bearing golden):
        wantArgs := []string{"--provider","zai","--model","glm-5.2","--system-prompt","<sys>",
            "--no-tools","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files",
            "--session-id","stagecoach-test","-p"}
        spec.Command == "pi"; reflect.DeepEqual(spec.Args, wantArgs); spec.Stdin == "<payload>".
  - ALSO assert containsToken(spec.Args, "--session-id") AND !containsToken(spec.Args, "--no-session") (belt
    + suspenders on the FR-T6 swap).
  - This is the FR-T9 verification pin: --no-session dropped, --session-id added (before -p), sys on turn 1.

Task 3: render_test.go — TestRenderMultiTurn_PiTurn2_NoSysPromptFlag_NoPrepend
  - Same pi-shape Manifest. CALL: spec, err := m.RenderMultiTurn("zai/glm-5.2", "<sys>", "<payload>", "",
    "stagecoach-test", 2).  (sysPrompt is "<sys>" BUT turn==2 ⇒ it must be suppressed.)
  - ASSERT:
        wantArgs := []string{"--provider","zai","--model","glm-5.2",
            "--no-tools","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files",
            "--session-id","stagecoach-test","-p"}   // NO "--system-prompt","<sys>"
        reflect.DeepEqual(spec.Args, wantArgs); spec.Stdin == "<payload>" (NOT "<sys>\n\n<payload>").
  - This is the load-bearing turn-1-only assertion: turn>1 ⇒ no sys flag AND no sys prepend, EVEN THOUGH
    sysPrompt was passed non-empty. (If turnSys were not used, this test fails — sys would leak onto turn 2.)

Task 4: render_test.go — TestRenderMultiTurn_NonAppendProviderErrors (capability gate)
  - BUILD a Manifest with SessionMode ABSENT (nil) or strPtr("") (e.g. a claude-shape, or the pi-shape with
    SessionMode removed). CALL: spec, err := m.RenderMultiTurn("zai/glm-5.2", "<sys>", "<p>", "", "id", 1).
  - ASSERT: err != nil; err.Error() contains "session_mode"; spec == nil. (Resolve defaults nil→"" → gate
    fires. A strPtr("") explicit value also fires.) Mirror TestRender_FR5b_RejectsBareModelOnMultiProvider's
    error-assertion idiom.

Task 5: render_test.go — TestRenderMultiTurn_DoesNotMutateManifest (immutability)
  - BUILD the pi-shape Manifest (with "--no-session" in BareFlags). SNAPSHOT before: wantBare :=
    append([]string(nil), m.BareFlags...). CALL RenderMultiTurn(...). ASSERT reflect.DeepEqual(m.BareFlags,
    wantBare) (still contains "--no-session"). Mirror TestRender_DoesNotMutateManifest (render_test.go:271).
  - This proves the filtered sessionArgs block built a FRESH slice (did not assign into r.BareFlags).

Task 6: VALIDATE
  - RUN: gofmt -w internal/provider/render.go internal/provider/render_test.go
  - RUN: go build ./... ; go vet ./... ; go test -race ./...
  - RUN the new tests specifically: go test -race -run 'TestRenderMultiTurn' ./internal/provider/ -v
  - RUN the Render regression: go test -race -run 'TestRender' ./internal/provider/ -v  # all existing Render tests stay green
  - GREP: confirm Render is unchanged and RenderMultiTurn exists:
        git diff internal/provider/render.go | grep -E '^-' | grep -v '^---'   # Expected: EMPTY (no deletions in render.go — pure addition)
        grep -n "func (m Manifest) RenderMultiTurn" internal/provider/render.go  # → 1 match
  - FIX-FORWARD: if turn-2 golden fails with a "--system-prompt" present, turnSys wasn't used in the sys-flag
    guard. If immutability fails, the filter mutated r.BareFlags (use a fresh sessionArgs slice).
```

### Implementation Patterns & Key Details

```go
// === render.go — RenderMultiTurn (ready to paste, after Render's closing brace) ===
// (Doc comment first — explains it is a sibling, the 3 deltas, FR-T6, FR-T9.)
//
// RenderMultiTurn renders ONE turn of the multi-turn generation fallback (PRD §9.24 FR-T6) against an
// existing session id. It is a SIBLING of Render (not a RenderMode): the variadic `mode ...RenderMode`
// carries no place for a session id, so widening Render (24+ call sites) or adding a mode value is
// rejected (research-provider.md §2 Option B). Render and every existing caller stay byte-for-byte
// unchanged.
//
// The argv is §12.2 with three multi-turn deltas (FR-T6, verified for pi 2026-07-05 — see
// architecture/fr-t9-verification.md):
//   1. capability gate (FR-T8/T9): errors unless *r.SessionMode == "append";
//   2. turn-1-only system prompt (FR-T6): the session carries it after turn 1 — a `turnSys` local
//      (= sysPrompt iff turn==1 else "") keys BOTH the flag-emission guard and the prepend-fallback guard;
//   3. session-flags block (FR-T6): BareFlags MINUS the exact "--no-session" token, PLUS "--session-id",
//      sessionID (a FRESH slice — r.BareFlags is never mutated).
// print_flag stays LAST; the payload+delivery switch + Env are identical to Render. On turns > 1 no
// system_prompt_flag is emitted AND the payload is not prepended with sys. `--continue`/`-c` is NEVER used
// (FR-T6: incompatible with `--session-id`). Like Render, this performs NO spawning — os.Environ() for Env
// is the sole side effect. P1.M1.T3.S2 calls this once per turn (N+1 calls).
func (m Manifest) RenderMultiTurn(model, sysPrompt, userPayload, reasoning, sessionID string, turn int) (*CmdSpec, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("provider render %q: %w", m.Name, err)
	}
	r := m.Resolve()

	// FR-T8/T9 capability gate: only an "append"-capable provider may multi-turn. Resolve guarantees
	// *r.SessionMode is non-nil (S1 default ""), so the deref is safe.
	if *r.SessionMode != "append" {
		return nil, fmt.Errorf("provider %q: multi-turn render requires session_mode=\"append\"", m.Name)
	}

	modelToUse := model
	if modelToUse == "" {
		modelToUse = *r.DefaultModel
	}

	// FR-T6 turn-1-only system prompt: one local keys both the flag guard and the prepend guard.
	turnSys := sysPrompt
	if turn != 1 {
		turnSys = ""
	}

	args := make([]string, 0, 16)
	args = append(args, r.Subcommand...)

	if *r.ProviderFlag != "" && modelToUse != "" {
		if i := strings.Index(modelToUse, "/"); i >= 0 {
			args = append(args, *r.ProviderFlag, modelToUse[:i])
			modelToUse = modelToUse[i+1:]
		} else {
			return nil, fmt.Errorf(
				"provider render %q: model %q on %s must be inference/model, e.g. \"zai/glm-5.2\"",
				m.Name, modelToUse, m.Name)
		}
	}
	if *r.ModelFlag != "" && modelToUse != "" {
		args = append(args, *r.ModelFlag, modelToUse)
	}

	if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0 {
		args = append(args, r.ReasoningLevels[reasoning]...)
	}

	if *r.SystemPromptFlag != "" && turnSys != "" {
		args = append(args, *r.SystemPromptFlag, turnSys)
	}

	// FR-T6 session-flags block: BareFlags MINUS "--no-session", PLUS "--session-id", sessionID.
	// Fresh slice — never mutate r.BareFlags (it may alias the caller's underlying array).
	sessionArgs := make([]string, 0, len(r.BareFlags)+2)
	for _, f := range r.BareFlags {
		if f == "--no-session" {
			continue
		}
		sessionArgs = append(sessionArgs, f)
	}
	sessionArgs = append(sessionArgs, "--session-id", sessionID)
	args = append(args, sessionArgs...)

	if *r.PrintFlag != "" {
		args = append(args, *r.PrintFlag) // LAST per §12.2
	}

	payload := userPayload
	if *r.SystemPromptFlag == "" && turnSys != "" {
		payload = turnSys + "\n\n" + userPayload
	}

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

	env := os.Environ()
	for k, v := range r.Env {
		env = append(env, k+"="+v)
	}
	spec.Env = env

	return spec, nil
}
```

```go
// === render_test.go — the golden turn-1 args (the FR-T9 byte-for-byte pin) ===
wantArgs := []string{
	"--provider", "zai", "--model", "glm-5.2",
	"--system-prompt", "<sys>",
	"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files",
	// NOTE: "--no-session" is ABSENT (filtered).
	"--session-id", "stagecoach-test",
	"-p", // print_flag LAST
}
// spec.Stdin == "<payload>" (sys goes via --system-prompt; NOT prepended for pi)

// === render_test.go — the golden turn-2 args (turn-1-only sys assertion) ===
wantArgsTurn2 := []string{
	"--provider", "zai", "--model", "glm-5.2",
	// NOTE: NO "--system-prompt","<sys>" (turn>1 ⇒ turnSys="" ⇒ flag suppressed).
	"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files",
	"--session-id", "stagecoach-test",
	"-p",
}
// spec.Stdin == "<payload>" (turnSys="" ⇒ no prepend — even though sysPrompt arg was "<sys>")
```

```go
// === render_test.go — the pi-shape Manifest literal the 4 tests reuse ===
m := Manifest{
	Name:             "pi",
	Command:          strPtr("pi"),
	ProviderFlag:     strPtr("--provider"),
	ModelFlag:        strPtr("--model"),
	SystemPromptFlag: strPtr("--system-prompt"),
	PrintFlag:        strPtr("-p"),
	PromptDelivery:   strPtr("stdin"),
	SessionMode:      strPtr("append"),
	BareFlags:        []string{"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session"},
}
```

### Integration Points

```yaml
RENDER (internal/provider/render.go):
  - method added: "func (m Manifest) RenderMultiTurn(model, sysPrompt, userPayload, reasoning, sessionID string, turn int) (*CmdSpec, error)"
  - capability gate: '*r.SessionMode != "append" → error'
  - deltas: turn-1-only sys (turnSys local); session-flags block (filtered BareFlags + --session-id); print_flag LAST

NO-TOUCH (explicitly — owned by sibling/later subtasks):
  - internal/provider/render.go Render method   # byte-identical (git diff shows only the addition)
  - internal/provider/manifest.go               # S1 (LANDED): SessionMode field + Resolve + Validate
  - internal/provider/merge.go                  # S2 (parallel): MergeManifest SessionMode clause
  - internal/provider/builtin.go + providers/pi.toml   # S4: pi SessionMode="append" (FR-T9)
  - docs/*                                       # S5: render-behavior doc / providers.md / configuration.md
  - multiturn.go / generate.go                   # P1.M1.T3: the N+1 turn protocol that CALLS RenderMultiTurn
  - any other package; PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S3):
  - P1.M1.T3.S2 (the turn protocol): calls RenderMultiTurn(model, sys, chunk, reasoning, sessionID, turn) per
    turn; mints the session id; supplies turn-1 sys + priming, turns 2..N chunks, turn N+1 the final prompt.
  - P1.M1.T4 (integration tests): exercises RenderMultiTurn end-to-end via the stub agent, asserting
    --session-id present, --no-session dropped, final parsed+deduped, commit lands.
  - S4 (pi builtin): ships SessionMode="append" so the gate passes for pi in production. S3's unit tests set
    it in a literal (independent of S4).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/provider/render.go internal/provider/render_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/provider/...   # Expected: exit 0
go build ./...                   # Expected: exit 0
```

### Level 2: Unit Tests — the 4 new tests + the Render regression

```bash
cd /home/dustin/projects/stagecoach

# The new RenderMultiTurn tests — the golden turn-1 + turn-2 are the load-bearing assertions
go test -race -run 'TestRenderMultiTurn' ./internal/provider/ -v

# The existing Render tests MUST stay green (proves Render is byte-identical)
go test -race -run 'TestRender' ./internal/provider/ -v

# Full provider suite
go test -race ./internal/provider/ -v

# Expected: ALL PASS. Turn-1 golden: --no-session absent, --session-id present before -p, --system-prompt on
# turn 1. Turn-2 golden: NO --system-prompt, no sys prepend (even though sysPrompt arg was non-empty).
# Under a turn-leak bug (turnSys not used), turn-2 fails with a stray --system-prompt. The gate test errors
# on a non-append provider. The immutability test keeps m.BareFlags intact.
```

### Level 3: Whole-Repository Regression

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages green
go vet ./...                     # Expected: exit 0

# Confirm Render is UNCHANGED (pure addition) and RenderMultiTurn exists
git diff internal/provider/render.go | grep -E '^-[^-]' | grep -v '^---'   # Expected: EMPTY (no removed lines)
git diff internal/provider/render.go | grep -E '^\+[^+]' | grep -v '^+++' | head   # Expected: the new method
grep -n "func (m Manifest) RenderMultiTurn" internal/provider/render.go    # → 1 match
grep -n "func (m Manifest) Render" internal/provider/render.go             # → 2 matches (Render + RenderMultiTurn)

# Confirm ONLY the 2 intended files changed
git diff --stat -- internal/ pkg/ cmd/ docs/ providers/
# Expected: internal/provider/render.go + internal/provider/render_test.go only.
```

### Level 4: FR-T9 Render-Contract Smoke (the verified flag transformation, via a throwaway in-package test)

```bash
cd /home/dustin/projects/stagecoach

# Inline proof (delete after): RenderMultiTurn reproduces the FR-T9-verified pi transformation.
cat > internal/provider/zz_mt_smoke_test.go <<'EOF'
package provider
import ("reflect"; "testing")
func TestZZ_MultiTurnFRT9Smoke(t *testing.T) {
	m := Manifest{Name: "pi", Command: strPtr("pi"), ProviderFlag: strPtr("--provider"),
		ModelFlag: strPtr("--model"), SystemPromptFlag: strPtr("--system-prompt"),
		PrintFlag: strPtr("-p"), PromptDelivery: strPtr("stdin"), SessionMode: strPtr("append"),
		BareFlags: []string{"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session"}}
	spec, err := m.RenderMultiTurn("zai/glm-5.2", "<sys>", "<payload>", "", "stagecoach-frt9", 1)
	if err != nil { t.Fatalf("err=%v", err) }
	want := []string{"--provider", "zai", "--model", "glm-5.2", "--system-prompt", "<sys>",
		"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files",
		"--session-id", "stagecoach-frt9", "-p"}
	if !reflect.DeepEqual(spec.Args, want) { t.Fatalf("turn1 args:\n got %v\nwant %v", spec.Args, want) }
	if spec.Stdin != "<payload>" { t.Fatalf("Stdin=%q want <payload>", spec.Stdin) }
	// turn 2: no sys flag, no prepend
	spec2, _ := m.RenderMultiTurn("zai/glm-5.2", "<sys>", "<payload>", "", "stagecoach-frt9", 2)
	for _, a := range spec2.Args { if a == "--system-prompt" { t.Fatal("turn2 leaked --system-prompt") } }
	if spec2.Stdin != "<payload>" { t.Fatalf("turn2 Stdin=%q want <payload> (no prepend)", spec2.Stdin) }
	t.Log("FR-T9 verified transformation reproduced (turn-1 sys, --no-session dropped, --session-id added) ✅")
}
EOF
go test -run TestZZ_MultiTurnFRT9Smoke -v ./internal/provider/ ; rm -f internal/provider/zz_mt_smoke_test.go
# Expected: PASS. This is the exact argv fr-t9-verification.md recorded from the live pi run.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green (incl. the 4 new RenderMultiTurn tests + all existing Render tests).
- [ ] `git diff internal/provider/render.go` shows ONLY additions (Render byte-identical).
- [ ] `grep "func (m Manifest) RenderMultiTurn" internal/provider/render.go` → 1 match.

### Feature Validation

- [ ] `RenderMultiTurn(model, sysPrompt, userPayload, reasoning, sessionID string, turn int) (*CmdSpec, error)` exists after `Render`.
- [ ] Capability gate `if *r.SessionMode != "append"` fires after Resolve with the exact error message.
- [ ] Turn-1 golden: `--no-session` absent; `--session-id <id>` present before `-p`; `--system-prompt <sys>` present; Stdin = payload only.
- [ ] Turn-2: no `--system-prompt`; Stdin = payload (no sys prepend) — even when sysPrompt arg is non-empty.
- [ ] The session-flags block is a fresh slice; `m.BareFlags` is unchanged after the call (immutability test).
- [ ] FR-R5b bare-model hard-error, FR-R6 reasoning no-op, print_flag-LAST, delivery switch, and Env are identical to Render.

### Scope Discipline Validation

- [ ] ONLY `internal/provider/{render,render_test}.go` modified (git diff --stat confirms).
- [ ] `Render` is byte-identical (git diff shows no removed lines in render.go).
- [ ] Did NOT edit `manifest.go` (S1), `merge.go` (S2), `builtin.go`/`providers/*.toml` (S4), docs (S5).
- [ ] Did NOT implement the turn loop / `multiturn.go` (P1.M1.T3) — S3 is the renderer only.
- [ ] Did NOT widen `Render`'s signature or add a `RenderMode` value (sibling method, per research §2).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] The method mirrors `Render`'s pipeline shape (Validate → Resolve → model fallback → args → payload/delivery → env).
- [ ] The `turnSys` local is the SINGLE turn-1-only mechanism (used in both the flag guard and the prepend guard — no duplicated turn checks).
- [ ] The session-flags block builds a fresh slice (range over r.BareFlags; never assigns into it).
- [ ] The exact-token `"--no-session"` filter (not prefix, not index).
- [ ] Doc comment states it is a sibling (not a mode), the 3 deltas, FR-T6 turn-1-only, and the FR-T9 reference.

---

## Anti-Patterns to Avoid

- ❌ Don't widen `Render`'s signature or add a `RenderMode` value. The variadic `mode ...RenderMode`
  carries no session-id slot; widening breaks 24+ call sites. RenderMultiTurn is a SIBLING method with its
  own `(sessionID, turn)` params (research §2 Option B, chosen).
- ❌ Don't modify `Render`. Render and its golden tests (`TestRender_Pi_ByteForByteCommitPi`,
  `TestRender_GoldenPerProvider`) must stay byte-identical. `git diff render.go` should show ONLY additions.
- ❌ Don't dereference `*r.SessionMode` before `r := m.Resolve()`. S1's Resolve guarantees non-nil; deref
  only after Resolve. The gate goes AFTER Resolve (and after Validate).
- ❌ Don't write two separate turn-1 checks (one for the flag, one for the prepend). Use a single `turnSys`
  local in BOTH guards — two checks will drift and the turn-2 golden test is what catches a leak.
- ❌ Don't mutate `r.BareFlags` (or re-slice it) to drop `--no-session`. Build a FRESH `sessionArgs` slice
  (range + skip). r.BareFlags may alias the caller's underlying array; the immutability test pins this.
- ❌ Don't filter `--no-session` by prefix or index. Use the EXACT token `f == "--no-session"` (only pi ships
  it; a provider without it is unaffected — its flags pass through, then `--session-id` appends).
- ❌ Don't move `print_flag` from LAST. §12.2 mandates it; the FR-T9 rendering shows `--session-id <id>`
  precedes `-p`. The session-flags block goes BEFORE print_flag.
- ❌ Don't drop or weaken the FR-R5b bare-model hard-error branch. A provider_flag provider (pi) given a bare
  model must error identically to Render. Copy that branch verbatim.
- ❌ Don't add `--continue`/`-c`. FR-T6 explicitly forbids it (incompatible with `--session-id`). The
  session-flags block is ONLY (filtered BareFlags + `--session-id <id>`).
- ❌ Don't implement the turn loop, chunking, the trigger gate, or `multiturn.go` — those are P1.M1.T3. S3 is
  the per-turn RENDERER ONLY. P1.M1.T3.S2 calls RenderMultiTurn N+1 times.
- ❌ Don't edit `manifest.go` (S1 landed), `merge.go` (S2), `builtin.go`/`pi.toml` (S4), or docs (S5). S3 is
  render.go + its test ONLY. S3 is code-independent of S2/S4 (its unit tests set SessionMode in a literal).
- ❌ Don't extract a shared helper out of Render to "DRY" the pipeline. That touches Render's internals and
  risks the 24+ call sites' byte-identical output. Duplicate the args-build in the sibling (contract:
  "rebuilds the §12.2 args EXACTLY as Render does"); both have independent golden tests. A future refactor
  can DRY it; out of scope for S3.
- ❌ Don't introduce mocking/stubbing in the unit tests. RenderMultiTurn is pure data transformation (the
  sole side effect is `os.Environ()` for Env); Manifest literals + `reflect.DeepEqual` suffice.
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a single-method addition with the EXACT current `Render` body quoted verbatim (the
pipeline to mirror), the EXACT `RenderMultiTurn` target body provided ready-to-paste (including the doc
comment), the EXACT golden turn-1 and turn-2 args derived verbatim from the FR-T9 verification record, the
exact capability-gate error message, and the exact pi-shape test Manifest literal. Four independent
de-riskings: (1) S1 is ALREADY landed (`SessionMode *string` + Resolve default — verified), so the gate
compiles and `*r.SessionMode` is safe to deref TODAY; (2) the sibling-method shape is the explicit choice of
research-provider.md §2 (Option B), with Option A (signature widening / new RenderMode) documented as
rejected — Render's byte-identical guarantee is structural, not luck; (3) the turn-1-only insight (one
`turnSys` local keys both guards) is the single non-obvious mechanism, and the turn-2 golden test is the
load-bearing assertion that catches a leak; (4) the FR-T9 transformation was verified by a LIVE pi run
(2026-07-05), so the flag set S3 encodes is confirmed, not speculative. S3 is code-independent of S2/S4
(unit tests set SessionMode in a literal), so the parallel-execution dependency is not a risk. The only
residual uncertainty (not 10/10) is whether the implementer remembers the `turnSys` local in BOTH guards
(the PRP repeats this in 4 places + the turn-2 test catches it) and the fresh-slice immutability (caught by
Task 5). No code outside `internal/provider/render.go` is in scope, so the blast radius is one method + its
tests, and Render is provably untouched (the Level-3 git-diff gate asserts zero removed lines).
