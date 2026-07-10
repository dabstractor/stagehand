# Research Notes — P1.M1.T1.S1 (stager-fallback blanking guard + guidance comment)

Verification of the task-description claims against the CURRENT working tree (2026-07-10). The
architecture doc (`architecture/bootstrap_pi_model_bug.md` §Issue 1) is accurate. These notes record
exact line numbers, the guard-placement reasoning, and the test-pin that S1 must touch.

## VERIFIED CODE — bootstrap.go (buildBootstrapConfig, target string)

| Line | Code | Role |
|------|------|------|
| 164 | `models := DefaultModelsForProvider(target)` | models for the target provider |
| 165 | `piBlanked := target == "pi"` | TRUE only when target IS pi |
| 166-172 | `if piBlanked { for role := range models { models[role] = "" } }` | blank ALL target roles (pi only) |
| 174 | `stagerName, stagerModel := StagerFallback(target, models)` | returns `("pi","gpt-5.4-mini")` for non-pi non-stager targets |
| 175-179 | `if piBlanked { ... stagerModel = "" }` | re-blank (only fires when target==pi) |
| 182 | `piHasOverrides := piBlanked && len(overrides) > 0` | |
| 183 | `applyOverrides(models, &stagerModel, overrides)` | override may set stagerModel |
| 186-195 | `if piBlanked && !piHasOverrides { b.WriteString("# NOTE: pi is a multi-backend ...") }` | big NOTE block (ONLY when target==pi) |
| 200 | `var stagerAnnotation string` | |
| 201-203 | `if stagerName != target { stagerAnnotation = target+" cannot serve as the stager ... routed to "+stagerName+" ..." }` | fallback annotation |
| 204 | `writeRoleBlock(&b, "stager", stagerName, stagerModel, stagerAnnotation)` | writes the [role.stager] block |

## THE BUG (confirmed)
When target ∈ {agy, opencode, qwen-code, codex, cursor} (providers with empty tooled_flags — cannot
serve as stager): `piBlanked` is false → `StagerFallback` returns `("pi", "gpt-5.4-mini")` (a FRESH
bare model from DefaultModelsForProvider("pi")) → the `if piBlanked` guard at 175 does NOT fire →
`stagerModel = "gpt-5.4-mini"` (bare) is written → FR-R5b HARD ERROR at role resolution. Decomposition
(the DEFAULT action when nothing is staged + dirty tree) fails before the planner runs.

## THE FIX — two edits

### Edit 1: new blanking guard (after line 179, BEFORE applyOverrides at line 182)
```go
if piBlanked {
    ...
    stagerModel = ""
}
// NEW — stager fell back to pi for a non-pi target: blank the bare fallback model so it isn't
// written as an FR-R5b-violating bare model. pi remains the stager (stager-capable); only the
// MODEL is blanked. Placed BEFORE applyOverrides so an explicit override can still set a model.
if stagerName == "pi" && stagerName != target {
    stagerModel = ""
}
```
Placement is task-specified ("after the StagerFallback call and after the existing if piBlanked
guard"). Mirrors the pi-target path's blank-then-override ordering.

### Edit 2: extend the stager annotation (after line 203, before writeRoleBlock at 204)
```go
if stagerName != target {
    stagerAnnotation = target + " cannot serve as the stager (no tooled_flags); routed to " + stagerName + " (the first stager-capable provider)."
}
// NEW — when the stager fell back to pi and no override supplied a model (stagerModel still ""),
// the bare fallback was blanked; append the multi-backend guidance so the user prefixes the backend.
if stagerName == "pi" && stagerName != target && stagerModel == "" {
    stagerAnnotation += " pi is a multi-backend provider — prefix the model with your inference backend, e.g. model = \"zai/glm-5.2\". A bare model (no '/') on pi is a config error (FR-R5b)."
}
```
- The guidance wording is the SAME sentence as the target==pi NOTE at lines 187-188 (stripped of the
  "# NOTE: "/"# " prefixes). The task explicitly says "same wording as the target==pi guidance".
- `stagerModel == ""` correctly handles the override edge case: if `overrides["stager"]` set a valid
  prefixed model (applyOverrides at 183), stagerModel != "" → no guidance (the user supplied one).
  This is correct — guidance only when the model is actually blank.
- writeRoleBlock prints the annotation as a single `# <annotation>` comment line inside the
  [role.stager] block (bootstrap.go writeRoleBlock: `if annotation != "" { fmt.Fprintf(b, "# %s\n", annotation) }`).
  The annotation becomes one (long, valid-TOML) comment line — matches the task's "via writeRoleBlock's
  annotation parameter".

## DELTA 1 — S1 MUST update the directly-broken pinning test (bootstrap_test.go:87)

`TestBuildBootstrapConfig_AgyStagerFallback` (bootstrap_test.go:74) asserts the BUGGY value:
```go
// line 87
assertContains(t, content, "[role.stager]", `model = "gpt-5.4-mini"`)
```
The S1 code change makes this assertion FAIL (the value is now `""`). A code change that breaks a
test that pins the exact value being changed CANNOT leave that test red and claim a green gate.
So S1 MUST flip line 87 to `model = ""`. This is the MINIMUM necessary to keep the gate green —
NOT scope creep into S2.

What S1 does NOT do (S2's job): extend the `gpt-5.4` negative guard (bootstrap_test.go:41-43, which
today only runs for target=="pi") to the stager-fallback cases, and add the guidance-comment
assertion. S1 just flips the one pin + optionally asserts the guidance comment appears.

NOTE the plan/tasks label the test-update subtask "S2" while the task description body says
"P1.M1.T1.S3 (test updates)" — minor plan-label inconsistency. Either way: S1 = the code fix + the
single directly-broken pin; the broader test coverage is the sibling test subtask.

## DELTA 2 — which targets are affected / unaffected
- AFFECTED (stager falls back to pi, model now blanked): agy, opencode, qwen-code, codex, cursor
  (all have empty tooled_flags). The existing test only covers agy; the others aren't value-tested
  today (S2/P1.M1.T2 can add coverage).
- UNAFFECTED: pi (target==pi, piBlanked path already correct), claude (stager-capable — StagerFallback
  returns (claude, "sonnet"), no fallback). The `TestGenerateBootstrapConfig_NamedProvider` (claude)
  test is unaffected and must still pass.

## DELTA 3 — the big NOTE block (lines 186-195) is NOT fired for the stager-fallback case
`if piBlanked && ...` → only when target==pi. For target=agy, piBlanked is false → the big multi-line
NOTE is NOT emitted → only the STAGER annotation carries the guidance. This is correct and intended:
agy's planner/message/arbiter models are valid (prefixed or single-backend bare), so only the
pi-fallback stager needs the prefix guidance. Do NOT move/duplicate the NOTE block.

## SCOPE BOUNDARIES (sibling subtasks — do NOT implement here)
- **P1.M1.T1.S2**: extend the `gpt-5.4` negative guard to stager-fallback cases + add guidance-comment
  assertions. S1 only flips the one broken pin (line 87).
- **P1.M1.T2.S1**: post-bootstrap ValidateModel regression net (the test that catches this class of
  bug automatically; lives in config_test external package — imports internal/provider).
- **P1.M2.T1.*** : Issue 2 (commented-out pi provider block bare models — DIFFERENT code path,
  bootstrap.go:205-222 commented-block loop). Do NOT touch.
- **P1.M2.T2.*** : Issue 3 (config upgrade backup). Do NOT touch.
- **P1.M3.*** / **P1.M4.*** : minor fixes (Issues 4/5/6) + docs. Do NOT touch.
- DOCS: none (this fix makes the code match docs/configuration.md:40 which already says pi models
  are left EMPTY). No user-facing/docs change.
