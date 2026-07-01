# Issue 5: Bootstrap Config Not Functional for pi (Minor, Latent)

## Root Cause

`config init` writes a bootstrap config via `GenerateBootstrapConfig` (`internal/config/bootstrap.go`)
that:
- Sets `[defaults] provider = "pi"`.
- Sets `[role.planner] model = "gpt-5.4"`, `[role.stager] model = "gpt-5.4-mini"` (routed to pi),
  `[role.message] model = "gpt-5.4-nano"`, `[role.arbiter] model = "gpt-5.4-mini"`.
- Does **NOT** write `default_provider` anywhere.

The per-role models come from `roleDefaults` (`internal/config/role_defaults.go`):
```go
"pi": {
    "planner": "gpt-5.4",
    "stager":  "gpt-5.4-mini",
    "message": "gpt-5.4-nano",
    "arbiter": "gpt-5.4-mini",
},
```

**Currently masked**: With the Issue 1 bug, the CLI emits `--provider pi` (the manifest name), which
errors before pi ever routes the model. Once Issue 1 is fixed (callers pass `""` → Render omits
`--provider` when `DefaultProvider` is `""`), pi receives:
```
pi --model gpt-5.4-nano --system-prompt … -p
```
with **no** `--provider` flag. Per the pi.toml comment, pi's own default backend is "google", where
`gpt-5.4-nano` does not exist → **model-not-found**.

## The Fix (Two Options)

### Option A (Recommended): Default pi's per-role models to `""` in the bootstrap

When `default_provider` is not set for pi, the bootstrap should write `model = ""` for each role so
pi picks its own backend default model. This is the safe, conservative approach — it avoids writing
models that cannot route without a verified sub-provider.

**Change**: In `bootstrap.go`'s `buildBootstrapConfig`, when the target provider is pi AND no
`default_provider` is configured, write `model = ""` for all roles (or omit the `[role.*]` blocks
entirely, letting pi's manifest defaults apply — but pi's `DefaultModel` is `""` per FR-D2, so pi
would need its own model selection).

Actually, since pi's `DefaultModel` is `""` (FR-D2), writing `model = ""` means pi gets no model
flag at all. Whether pi works depends on whether pi picks a sensible default when no model is
specified. Per the PRD's Appendix E #12, this is an open question — the shipped bootstrap should not
write models that cannot route.

**Conservative fix**: Write `model = ""` for pi roles AND add a comment in the bootstrap noting the
user must configure either a `default_provider` (with a compatible model) or set models compatible
with pi's default backend.

### Option B: Write a `default_provider` into the bootstrap `[provider.pi]`

Write `default_provider = "<verified-sub-provider>"` (the OpenAI-routing pi sub-provider) into the
bootstrap. This requires verifying which pi sub-provider routes the `gpt-5.4*` models (Appendix E
#12 open question). Since this requires external verification, it's riskier.

**Recommended**: Use Option A (empty models) until a verified routing sub-provider is confirmed
(per Appendix E #12). Add a clear comment in the bootstrap explaining the user must configure a
model+sub-provider combination.

## Implementation Detail

In `buildBootstrapConfig` (`bootstrap.go`), the per-role models come from `DefaultModelsForProvider(target)`.
For pi, these are the `gpt-5.4*` models. The fix:

```go
// When target is pi and no default_provider is set, use empty models
// (pi needs a sub-provider to route gpt-5.4* models).
models := DefaultModelsForProvider(target)
if target == "pi" {
    // pi's gpt-5.4* models require a sub-provider (default_provider) to route.
    // Without one, leave models empty so pi picks its own backend default.
    // The user must configure [provider.pi] default_provider + compatible models.
    for role := range models {
        models[role] = ""
    }
}
```

Then the bootstrap writes `[role.*] model = ""` for pi, and the header comment should explain:
```
# NOTE: pi requires a default_provider (sub-provider) to route models.
# The shipped models are empty — set [provider.pi] default_provider and
# compatible per-role models, or let pi pick its own backend default.
```

**Alternatively**: Only blank the models if `default_provider` is NOT set. But the bootstrap doesn't
currently check whether `default_provider` is set (it's a built-in manifest field, not a config
field). The cleanest approach: always blank pi models in the bootstrap (since the bootstrap is the
out-of-box experience and doesn't set `default_provider`).

## Test Strategy

1. **Unit test for `GenerateBootstrapConfig`**: Assert that when target="pi", the generated config
   has `model = ""` for all `[role.*]` blocks (not `gpt-5.4*`).

2. **Unit test asserting the bootstrap is functional**: After fixing Issue 1, verify that a config
   with `provider = "pi"` and `model = ""` produces a valid pi invocation (pi invoked with no
   `--provider` and no `--model` — pi picks its own defaults).

## Dependency

This issue is **latent** and only surfaces after Issue 1 is fixed. It should be implemented AFTER
Issue 1. If Issue 1 is not yet fixed, this fix is a no-op (pi errors on `--provider pi` before model
routing).

## Files to Touch

| File | Change | Doc Mode |
|------|--------|----------|
| `internal/config/bootstrap.go` | Blank pi models when no sub-provider; add explanatory comment | Mode A (bootstrap output docs) |
| `internal/config/bootstrap_test.go` | Assert pi bootstrap has empty models | — |
