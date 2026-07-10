# P1.M3.T1.S2 Research Findings — multiturn.go + workdesc.go + hook/exec.go message-role timeout

> Research for: wiring `config.ResolveRoleTimeout("message", cfg)` at the message-role Execute sites in
> the multi-turn transport (`multiturn.go`), the work-description transport (`workdesc.go`), and the hook
> runtime (`hook/exec.go`). This is the SECOND consumer-wiring subtask of P1.M3; S1 wired
> `internal/generate/generate.go` (CommitStaged) and is the exact template to mirror.

---

## §0. STATE OF THE WORLD (verified against the working tree)

- **S1 (P1.M3.T1.S1) is ALREADY IMPLEMENTED** in the working tree. Confirmed by grep + read:
  - `internal/generate/generate.go:264` — `_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)`
  - `internal/generate/generate.go:269` — `msgTimeout := config.ResolveRoleTimeout("message", cfg)` (5-line comment block 265–268)
  - `internal/generate/generate.go:340` — `out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)` (Execute uses msgTimeout)
  - `internal/generate/generate.go:431` — `totalMin := int((msgTimeout * time.Duration(turns)).Minutes())` (budget display uses msgTimeout)
  - `internal/generate/generate_test.go:495` — `func TestCommitStaged_MessageRoleTimeout` (the S1 proof test, present)
  - `generate.go`'s Execute + budget sites NO LONGER reference `cfg.Timeout` (grep for `cfg.Timeout` in generate.go shows only the line-87 ErrTimeout comment + the line-267 ResolveRoleTimeout comment).
- **The dependency — `config.ResolveRoleTimeout` — is LANDED** (P1.M2.T1.S1, COMPLETE). See §3.
- **The 3 S2 target files are UNTOUCHED** (still read `cfg.Timeout`): confirmed by `grep -rn 'cfg.Timeout' internal/generate/ internal/hook/` (see §1).
- **CONCLUSION**: S2 mirrors the S1 pattern (resolve `msgTimeout` once near the message-role resolution, swap the `cfg.Timeout` Execute + budget sites). S1 is the concrete, in-tree reference — not just a PRP intent.

---

## §1. THE EXACT EDIT SITES (verified by grep — line numbers + unique strings)

### multiturn.go — `func Run` (signature at :145; receives `cfg config.Config`)
```
:132  // Per-turn timeout = cfg.Timeout (FR-T5; Execute shadows ctx with WithTimeout).   [DOC COMMENT — update]
:165  if _, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose); execErr != nil {   [turn 1]
:176      if _, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose); execErr != nil {  [turns 2..N]
:187  out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)                        [turn N+1 final]
```
- 3 Execute sites + 1 doc comment. All inside `Run`. Run receives `cfg` ⇒ can resolve `msgTimeout` locally.
- IMPORTS: `config` (yes) + `time` (yes) — NO new import.

### workdesc.go — `func RunWorkDescription` (signature at :63; receives `cfg config.Config`)
```
:75   out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)   [turn 1]
:106          out2, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)  [forced-conclusion turn, FR-W6]
:122      out, _, execErr = provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)        [answer turn, note `=` not `:=`]
```
- 3 Execute sites. All inside `RunWorkDescription`. Receives `cfg` ⇒ resolves `msgTimeout` locally.
- IMPORTS: `config` (yes) + `time` (**NO** — but NOT needed for the source change; `ResolveRoleTimeout` returns `time.Duration`, stored in a local. NO new import for the .go source.)
- GOTCHA: line 122 uses `=` (assignment to the existing `out` from line 75), NOT `:=`. Preserve that — only swap the `cfg.Timeout` arg.

### hook/exec.go — `func Run` (the hook runtime)
```
:162  _, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)   [Step F — the co-location ANCHOR]
:182      out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)   [one-shot Execute in the dedupe loop]
:252          totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())          [multi-turn budget DISPLAY, FR-T5]
```
- IMPORTS: `config` (yes) + `time` (yes) — NO new import. `config.ResolveRoleModel("message", cfg)` already at :162.
- **TWO cfg.Timeout sites** (182 Execute + 252 budget display). The contract text mentioned only :182; :252 is the IDENTICAL FR-T5 budget-display site S1 updated in generate.go:431. See §6 for the decision (update BOTH, resolve a local `msgTimeout`).

---

## §2. THE S1 PATTERN (the in-tree template to mirror — `internal/generate/generate.go`)

S1's `CommitStaged` resolves the message-role model AND timeout together, co-located, then uses the local
at the Execute + budget sites:

```go
_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)       // generate.go:264
// FR-R7/FR25: resolve the message role's timeout so [role.message].timeout / --message-timeout
// bound the message agent's one-shot generation (and the multi-turn total budget, FR-T5) instead
// of the flat cfg.Timeout. With no per-role override ResolveRoleTimeout returns cfg.Timeout
// (the message role has no built-in) — behavior-preserving by default.
msgTimeout := config.ResolveRoleTimeout("message", cfg)                    // generate.go:269
...
out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)  // generate.go:340 (was cfg.Timeout)
...
totalMin := int((msgTimeout * time.Duration(turns)).Minutes())             // generate.go:431 (was cfg.Timeout)
```

S2 mirrors this PER FILE:
- **multiturn.go**: resolve `msgTimeout` at the top of `Run` (Run has no ResolveRoleModel call — it receives msgModel/msgReasoning as params; so resolve msgTimeout once at the top of Run, with a comment citing FR-R7/FR-T5). Swap the 3 Execute sites + the :132 doc comment.
- **workdesc.go**: resolve `msgTimeout` at the top of `RunWorkDescription` (also receives msgModel/msgReasoning as params; resolve msgTimeout once at top). Swap the 3 Execute sites.
- **hook/exec.go**: resolve `msgTimeout` immediately after the existing :162 `ResolveRoleModel("message", cfg)` (the hook HAS the model-resolution line — co-locate the timeout twin, exactly like generate.go:264/269). Swap :182 Execute + :252 budget.

---

## §3. THE DEPENDENCY — `config.ResolveRoleTimeout` (LANDED; consume, do NOT rebuild)

`internal/config/roles.go:128`:
```go
func ResolveRoleTimeout(role string, cfg Config) time.Duration {
    if rc, ok := cfg.Roles[role]; ok && rc.Timeout != 0 {
        return rc.Timeout            // per-role override wins
    }
    if d, ok := defaultRoleTimeouts[role]; ok {
        return d                     // built-in (planner=480s ONLY)
    }
    return cfg.Timeout               // global fallback
}
```
- `defaultRoleTimeouts` (roles.go:12) = `{planner: 480s}`. **The "message" role has NO built-in.**
- ⇒ `ResolveRoleTimeout("message", cfg)` returns `cfg.Roles["message"].Timeout` if non-zero, ELSE `cfg.Timeout`.
- ⇒ **behavior-preserving by default**: with no `[role.message].timeout` override, `msgTimeout == cfg.Timeout` byte-for-byte. Every existing test that sets only `cfg.Timeout` (and no `cfg.Roles["message"]`) is UNCHANGED.
- READ-ONLY for this task. Do NOT edit roles.go / defaultRoleTimeouts / ResolveRoleTimeout.

## §4. THE BEHAVIOR-PRESERVING-BY-DEFAULT PROOF (the regression safety net)

`config.RoleConfig.Timeout` is a plain `time.Duration`, 0 ⇒ inherit (config.go:42). `config.Config.Timeout` is `time.Duration`, default 120s (config.go:71). So:
- ResolveRoleTimeout("message", cfg) with no `cfg.Roles["message"]` entry ⇒ returns `cfg.Timeout` verbatim.
- All 6 Execute sites (3 multiturn + 3 workdesc) + 1 hook Execute + 1 hook budget become `cfg.Timeout`'s value when no override ⇒ byte-identical to today.
- Existing regression canaries that must stay GREEN unchanged:
  - `multiturn_test.go`: TestRun_HappyPath, TestRun_TurnError, TestRun_FinalParseEmpty, TestRun_NonAppendManifest (all set cfg via config.Defaults(), no Roles["message"]).
  - `generate_workdesc_test.go`: TestCommitStaged_WorkDescription_HappyPath, _RoundBudgetForcesConclusion, _NoCascadeToMultiTurn, _NonAppendProviderRescues, _OffByDefault (no Roles["message"]).
  - `hook/exec_test.go`: TestRun_TimeoutNeverBlock (cfg.Timeout=50ms, no Roles), TestRun_MultiTurnSuccess_WritesMessageFile, TestRun_MultiTurnFailure_NeverBlock, etc.

## §5. THE TEST HARNESS (for the 3 new behavioral-proof tests)

### provider.Execute timeout mechanism (executor.go:44)
```go
func Execute(ctx, spec, timeout time.Duration, vb) (...) {
    if timeout > 0 { ctx, cancel = context.WithTimeout(ctx, timeout); defer cancel() }  // SHADOW
    ... cmd.Wait() ...
    if ctxErr := ctx.Err(); ctxErr != nil { return ..., ctxErr }  // timeout → DeadlineExceeded
}
```
- A stub with `SleepMS` > the timeout ⇒ killed at the timeout ⇒ returns `context.DeadlineExceeded`.
- ⇒ a test with `cfg.Timeout=30s` (large) + `cfg.Roles["message"].Timeout=150ms` (small) + stub `SleepMS=2000` ⇒ Execute killed at 150ms (NOT 30s) ⇒ proves the MESSAGE-role timeout is the bound. (Exact mechanism S1 used at generate_test.go:495.)

### Test 1 — multiturn_test.go (proves multiturn.go:165/176/187)
- Harness: `stubtest.Manifest(bin, stubtest.Options{Out, Exit, SleepMS})` returns a `provider.Manifest`; supports `RenderMultiTurn` (PROVEN by multiturn_test.go TestRun_TurnError, which uses stubtest.Manifest + Run). Set `SessionMode=&"append"` (turn-1 RenderMultiTurn gate).
- `Run(ctx, Deps{}, cfg, m, sysPrompt, payload, msgModel, msgReasoning)` — Deps{} is fine (Run only uses deps.Verbose, nil-safe).
- NEW `TestRun_MessageRoleTimeoutBoundsTurn`: cfg.Timeout=30s, cfg.Roles["message"]={Timeout:150ms}, stub SleepMS=2000, MultiTurnChunkTokens=1 ⇒ turn-1 Execute killed at 150ms ⇒ cause != nil && errors.Is(cause, context.DeadlineExceeded).
- IMPORT: multiturn_test.go does NOT import `"time"` today (imports: config + stubtest + stdlib). The new test uses `time.Millisecond`/`time.Second` ⇒ ADD `"time"` to the import block.

### Test 2 — generate_workdesc_test.go (proves workdesc.go:75/106/122, via CommitStaged)
- The workdesc path is CLEANLY ISOLATABLE: generate.go:282 `workDescActive := cfg.WorkDescription != ""` ⇒ the workdesc branch runs RunWorkDescription and SKIPS the one-shot/multi-turn default loop entirely (comment generate.go:274–281). So a small message-role timeout times out RunWorkDescription's turn-1 Execute WITHOUT touching S1's one-shot msgTimeout path.
- On a per-turn timeout, RunWorkDescription returns cause=DeadlineExceeded ⇒ CommitStaged (generate.go:308) returns `&RescueError{Kind: ErrTimeout, TreeSHA, ParentSHA, Candidate, Cause}`.
- Harness: clone TestCommitStaged_WorkDescription_HappyPath (initRepo/commitRaw/writeFile/stageFile) but use `stubtest.Manifest(bin, {Out:"feat: never reached", SleepMS:2000})` + SessionMode=append (RenderMultiTurn gate) + cfg.WorkDescription="add x" + cfg.Timeout=30s + cfg.Roles["message"]={Timeout:150ms}.
- IMPORT: generate_workdesc_test.go does NOT import `"time"` today. The new test uses `time.Millisecond`/`time.Second` ⇒ ADD `"time"` to the import block.

### Test 3 — hook/exec_test.go (proves hook/exec.go:182; clones TestRun_TimeoutNeverBlock)
- Harness: TestRun_TimeoutNeverBlock (:269) is the template — `stubtest.Manifest(stubBin, {SleepMS:5000})` + `cfg := config.Config{Timeout: 50ms, MaxDuplicateRetries:2}` + real repo + msgFile. The hook one-shot Execute (line 182) times out ⇒ `errors.Is(execErr, DeadlineExceeded)` ⇒ returns `errors.New("stagecoach: hook generation timed out")` ⇒ msg-file UNTOUCHED (never-block).
- NEW `TestRun_MessageRoleTimeoutNeverBlock`: clone it but FLIP which field carries the small timeout — `cfg.Timeout=30s` (large) + `cfg.Roles["message"]={Timeout:50ms}` (small) + stub SleepMS=5000 ⇒ the 50ms message-role timeout (NOT 30s) bounds the one-shot Execute ⇒ "hook generation timed out" + msg-file untouched.
- The hook one-shot uses `deps.Manifest.Render` (NOT RenderMultiTurn) ⇒ no SessionMode needed (it times out before multi-turn). 
- IMPORT: exec_test.go already imports `"time"` (TestRun_TimeoutNeverBlock uses time.Millisecond/Second). NO new import.
- NOTE: this test does NOT exercise the :252 budget display (the one-shot times out before multi-turn). The :252 site is grep-guarded (uses the same `msgTimeout` local as :182); over-testing the display line is low-value (S1's reasoning applies).

---

## §6. THE hook/exec.go :252 DECISION (update BOTH :182 AND :252 — resolve a local)

The contract text said: "In hook/exec.go line 182, replace `cfg.Timeout` with `config.ResolveRoleTimeout(...)`."
But hook/exec.go has TWO cfg.Timeout message-role sites:
- :182 — the one-shot Execute (the contract's site).
- :252 — `totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())` — the multi-turn budget DISPLAY.

Decision: **update BOTH, resolving a local `msgTimeout`** (co-located with the existing :162 ResolveRoleModel), because:
1. :252 is the IDENTICAL FR-T5 budget-display site S1 updated in generate.go:431 (`msgTimeout * time.Duration(turns)`). S1's PRP explicitly made the generate.go budget line use msgTimeout "so the FR-T5 progress line reflects the message-role budget."
2. Leaving :252 on `cfg.Timeout` while `generate.Run` (multiturn.go, also wired by THIS task) uses `msgTimeout` internally ⇒ the hook's PRINTED `~Mm total` would be WRONG whenever a `[role.message].timeout` override is set (inconsistent with the actual per-turn bound). FR-T5 applies to hook mode too (the contract itself states "hook mode resolves the message role").
3. Resolving a local (vs the contract's suggested inline call) is CLEANER for 2 sites and matches S1's generate.go + this task's own multiturn.go/workdesc.go approach (all resolve a local). It also matches the existing :162 ResolveRoleModel co-location.
4. The behavior is identical either way (ResolveRoleTimeout is pure/deterministic) — this is purely about covering both FR-T5 sites consistently.

This is the correct, complete interpretation; the contract's "line 182" mention is an oversight (only one of the two sites was enumerated). Documented in the PRP's Anti-Patterns so the implementer doesn't "helpfully" inline only :182 and leave :252 inconsistent.

---

## §7. DOC-COMMENT ACCURACY (multiturn.go:132)

```
:132 // Per-turn timeout = cfg.Timeout (FR-T5; Execute shadows ctx with WithTimeout). Intermediate turns'
```
This comment becomes inaccurate after the edit (the per-turn timeout is now `msgTimeout`, not `cfg.Timeout`).
Update it to: `// Per-turn timeout = msgTimeout = ResolveRoleTimeout("message", cfg) (FR-R7/FR-T5; Execute shadows ctx with WithTimeout).`
- workdesc.go's doc comments reference "FR-T7 parity" generically (role-agnostic) ⇒ no update needed.
- hook/exec.go's doc at :274 references "turn error/timeout" generically ⇒ no update needed.

---

## §8. SCOPE FENCES (what NOT to touch)

- `internal/generate/generate.go` — S1 (DONE). Do NOT edit (the Run call at :436 still passes `cfg`; S2 wires Run's internals, NOT the call site).
- `internal/config/roles.go` — ResolveRoleTimeout + defaultRoleTimeouts (LANDED). READ-ONLY.
- `internal/config/config.go` — RoleConfig.Timeout, Config.Timeout (LANDED). READ-ONLY.
- planner/stager/arbiter Execute sites — P1.M3.T2.S1 (decompose path). NOT this task.
- Docs (README/docs) — P1.M4.T2.S1. Contract: "DOCS: none — internal wiring."
- Run's signature (multiturn.go:145) + RunWorkDescription's signature (workdesc.go:63) — UNCHANGED. Both already receive `cfg`; S2 resolves msgTimeout locally. NO signature change (matches S1's explicit "leave Run's signature alone" — and S1 left the Run CALL at generate.go:436 passing cfg).

## §9. VALIDATION COMMANDS (project-specific, verified)
- Build: `go build ./...` + `GOOS=windows go build ./...` + `GOOS=linux go build ./...`
- Vet: `go vet ./internal/generate/... ./internal/hook/...`
- Fmt: `gofmt -l internal/generate/multiturn.go internal/generate/workdesc.go internal/hook/exec.go internal/generate/multiturn_test.go internal/generate/generate_workdesc_test.go internal/hook/exec_test.go` (must be empty)
- Focused tests: `go test ./internal/generate/ -run 'TestRun_MessageRoleTimeoutBoundsTurn|TestCommitStaged_WorkDescription_MessageRoleTimeout|TestRun_HappyPath|TestRun_TurnError|TestCommitStaged_WorkDescription' -v` + `go test ./internal/hook/ -run 'TestRun_MessageRoleTimeoutNeverBlock|TestRun_TimeoutNeverBlock|TestRun_MultiTurn' -v`
- Full: `make test` (race) + `make lint` + `make coverage-gate` (PRD §20.3 ≥85% on internal/{git,provider,generate,config}).
