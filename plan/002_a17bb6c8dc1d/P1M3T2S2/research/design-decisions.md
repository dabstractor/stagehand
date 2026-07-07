# Design Decisions — P1.M3.T2.S2 (ResolveRoleModel)

The authoritative implementation sketch is `architecture/config_v2_delta.md` §4 (quoted verbatim in the
PRP Blueprint). This file records the NON-OBVIOUS design calls an implementer must make — the things the
delta sketch does not spell out and that are easy to get wrong.

---

## §0 — Scope: a NEW `internal/config/roles.go` + `internal/config/roles_test.go`. Nothing else.

The item contract gives the implementer a choice: "in internal/config/config.go **(or a new
internal/config/roles.go)**". **Choose the new `roles.go`.** Three reasons:

1. **`config.go` is FROZEN.** It is owned by P1.M3.T1.S1 (SHIPPED). The PRP for the parallel sibling
   (P1.M3.T2.S1) explicitly freezes it. A sibling task must not edit another task's frozen file.
2. **`load.go` is the parallel sibling's territory** (P1.M3.T2.S1 owns load.go + load_test.go). Putting
   `ResolveRoleModel` in load.go would create a merge-conflict surface during parallel execution. A new
   `roles.go` is 100% disjoint → zero merge risk.
3. **Cohesion.** `ResolveRoleModel` is the *read-side* counterpart to load.go's *write-side* (`setRoleProvider`
   /`setRoleModel`). Giving the role-resolution read logic its own file (`roles.go`) is cleaner than burying
   it in either config.go or load.go, and it is where future role helpers (e.g. a `ValidateRoleConfig` if
   ever needed) would naturally live.

So: **CREATE `internal/config/roles.go`** (the function) + **CREATE `internal/config/roles_test.go`**
(the tests). Do NOT edit config.go, load.go, file.go, git.go, or any *_test.go.

---

## §1 — Pure function, VALUE semantics. `cfg Config` (NOT `*Config`); return `(provider, model string)`.

The delta §4 and the item contract both specify the signature `func ResolveRoleModel(role string, cfg Config)
(provider, model string)`. Honor it exactly:

- **`cfg Config` by VALUE**, not pointer. `Defaults()` returns Config by value; `Load()` returns `*Config`
  but callers dereference (`ResolveRoleModel("planner", *cfg)`). A pure read never needs a pointer — taking
  Config by value makes the "no side effects" contract obvious and matches the delta sketch.
- **Named return values `(provider, model string)`** — the names document the layer each string belongs to
  (`provider` is the role's resolved provider, `model` the role's resolved model).
- **No error return.** This is a *resolution* function, not a *validation* function. Every combination of
  inputs has a well-defined output (worst case: `("", "")` = manifest defaults). Validation (FR-R5 model/
  provider mismatch, FR-R5b model-requires-provider) is the registry's job at manifest-resolution time, NOT
  here. See §6.

---

## §2 — The 7-layer precedence COLLAPSES to a 2-level check. This is correct, not a simplification.

PRD §16.1 / FR34 defines the full 7-layer precedence (lowest→highest): built-in defaults → built-in
provider defaults → global file → repo file → git config → env → flags. The item contract (point #1) and the
delta §4 make the crucial observation: **by the time `ResolveRoleModel` runs, the loaders have ALREADY merged
all 7 layers into `cfg`.** Specifically:

- `cfg.Roles[role]` holds the **highest-precedence per-role value across all layers** (flag/env wrote into it
  via load.go's `setRoleProvider`/`setRoleModel`; the file/git layers wrote into it via file.go's
  `overlay()`/`materialize()`). Because overlay() and the setRole* helpers both do **per-field merge** (a
  higher layer setting only `Model` does not erase a lower layer's `Provider`), `cfg.Roles[role]` is the
  fully-resolved per-role table. The flag>env>file>git precedence is ALREADY baked in.
- `cfg.Provider` / `cfg.Model` hold the **fully-resolved GLOBAL [defaults]** across all layers.

Therefore `ResolveRoleModel` needs only:
1. Read `cfg.Roles[role]` — take whatever fields are non-empty (per-field).
2. For any field still empty, fall back to `cfg.Provider` / `cfg.Model`.

That is EXACTLY the delta §4 logic. Do NOT try to re-walk the 7 layers (that work is done; re-doing it here
would duplicate the loaders and risk divergence). Do NOT reach into the manifest (layer "built-in provider
defaults") — see §5.

---

## §3 — Independent PER-FIELD resolution (FR-R3 field-merge at the role→global boundary).

Provider and Model are resolved **independently**. A role can override *only* the model (and inherit the
global provider), *only* the provider (and inherit the global model), both, or neither. This is the
per-field merge semantics of FR-R3 / FR37a, applied at the role→global fallback boundary.

Concretely the logic is (delta §4 verbatim):
```go
provider, model := "", ""
if rc, ok := cfg.Roles[role]; ok {
    if rc.Provider != "" { provider = rc.Provider }
    if rc.Model    != "" { model    = rc.Model    }
}
if provider == "" { provider = cfg.Provider }
if model    == "" { model    = cfg.Model    }
return provider, model
```
The two `if rc.X != ""` are independent, and the two `if x == ""` fallbacks are independent. NEVER write
`if rc.Provider != "" { provider = rc.Provider; model = rc.Model }` (coupling the two — a model-only role
override would be dropped if its provider is empty). A test pins each of the four combinations.

---

## §4 — `("", "")` is the SENTINEL for "use manifest defaults". The manifest layer is NOT resolved here.

The lowest precedence layer in FR-R3 is "built-in manifest default" (the provider manifest's `default_model`
and, for auto-detection, the registry's `DefaultProvider` cascading pick). The item contract (point #1) is
explicit: **the manifest default is handled by Render / the registry, NOT by `ResolveRoleModel`.** Concretely:

- `model == ""` ⇒ the consumer (registry/Render) uses the resolved provider manifest's `default_model`.
- `provider == ""` ⇒ the consumer applies auto-detection (`Registry.DefaultProvider(installed)`, FR-D1).

So `ResolveRoleModel` returns `("", "")` when NOTHING in any role/global layer set a value, and that is the
correct, intended signal to the downstream consumer. Do NOT look up manifests, do NOT import
`internal/provider` (that would risk an import cycle — config is a leaf and provider consumes config), do
NOT call `Defaults()` or `DefaultProvider`. Document the `("", "")` semantics in the doc comment so consumers
(P3.M2.T1.S1 `decompose/roles.go`, the CLI) know that an empty model means "manifest default_model".

---

## §5 — An UNKNOWN role name falls back to global (map zero-value). This is correct, document + test it.

`ResolveRoleModel` takes `role string` — an arbitrary string, not an enum. If `role` is not one of the four
canonical roles (`planner`/`stager`/`message`/`arbiter`) — e.g. a typo `"palnner"` — `cfg.Roles[role]`
returns the zero `RoleConfig{}` with `ok == false`, so the function falls through to the global
`cfg.Provider`/`cfg.Model`. That is the *right* behavior (an unknown role just inherits global) and requires
no special handling. Add a test (`TestResolveRoleModel_UnknownRoleFallsBackToGlobal`) so the behavior is
pinned and intentional, not accidental. (Canonical-role validation is the caller's concern, not this pure
function's.)

---

## §6 — What this function deliberately does NOT do (scope boundaries — do not creep).

- **NO manifest lookup.** See §4. `("", "")` is the sentinel; the registry/Render resolves the manifest layer.
- **NO FR-R5 / FR-R5b validation.** "Switching a role's provider without updating its model is a config error
  stagecoach surfaces" (FR-R5) and "model requires provider for multi-provider agents" (FR-R5b) are validation
  concerns surfaced at manifest-resolution time (the registry knows whether a provider is multi-backend and
  whether its manifest has the model). `ResolveRoleModel` is a pure string resolver; it has no manifest
  knowledge. Pushing validation here would couple config→provider (import cycle) and split the validation
  logic across two packages. Keep it pure.
- **NO edits to config.go / load.go / file.go / git.go.** Those are frozen / sibling-owned. The function is a
  new file (§0).
- **NO flag registration, NO env/flag reading.** That is P1.M3.T2.S1 (load.go, DONE) and P4.M1.T1.S1
  (root.go, future). `ResolveRoleModel` only READS the already-resolved `cfg`.
- **NO changes to `Defaults()`, `Load()`, or any other existing function.** It is purely additive.

---

## §7 — `roleNames` already exists in load.go (same package); tests MAY reference it but need not.

load.go (P1.M3.T2.S1, DONE) defines `var roleNames = []string{"planner", "stager", "message", "arbiter"}`.
Because `roles_test.go` is `package config` (white-box), it can reference `roleNames` directly — useful for
one "iterate all canonical roles" test proving `ResolveRoleModel` works for each. BUT the explicit per-case
tests (global-fallback, model-only override, provider-only override, both-empty, unknown-role) should hardcode
their role strings (`"planner"`, `"message"`, etc.) so they are self-documenting and do not silently change
meaning if `roleNames` is ever reordered. Reference `roleNames` for the canonical-roles iteration test only.

---

## §8 — No new dependency; `go.mod`/`go.sum` byte-unchanged.

`roles.go` needs NO imports — it is pure map access + string assignment (the body is ~6 lines, all stdlib-free).
`roles_test.go` uses only `testing` (already standard). `go mod tidy` MUST be a no-op; `git diff --exit-code
go.mod go.sum` MUST be empty.

---

## §9 — Test strategy (white-box `package config`, mirror config_test.go / load_test.go style).

Construct `Config` values directly via `Defaults()` + manual field sets (do NOT go through `Load()` — that
pulls in env/file/git and obscures the unit under test; `ResolveRoleModel` is a pure function of `cfg`). One
`t.Errorf` per assertion, mirroring `TestDefaults` in config_test.go. Required cases:

1. **Global fallback (Roles nil):** `cfg.Roles == nil` ⇒ returns `(cfg.Provider, cfg.Model)`.
2. **Global fallback (role absent):** Roles set but role not a key ⇒ returns global (the §5 unknown-role case,
   which is the same code path; one test covers both — but ALSO add an explicit non-canonical-name test for
   clarity).
3. **Per-role full override:** `cfg.Roles["planner"] = {agy, gemini-2.5-pro}`, global `{pi, ""}` ⇒
   `(agy, gemini-2.5-pro)`.
4. **Per-role MODEL-only override:** `cfg.Roles["message"] = {"", gpt-5.4-nano}`, global `{pi, ""}` ⇒
   `(pi, gpt-5.4-nano)` — provider inherits global, model is the override.
5. **Per-role PROVIDER-only override:** `cfg.Roles["stager"] = {agy, ""}`, global `{pi, gpt-5.4}` ⇒
   `(agy, gpt-5.4)` — model inherits global, provider is the override.
6. **Both empty everywhere:** global `{"", ""}`, Roles nil ⇒ `("", "")` (manifest-default sentinel).
7. **Canonical-roles iteration:** for each `role in roleNames`, a per-role override wins over global; with no
   override, global wins. (Proves all four roles resolve.)
8. **Precedence is already-baked:** set `cfg.Roles["planner"]` to a value and assert it is returned as-is
   (documents that `ResolveRoleModel` trusts the loaders' merge — it does not re-derive precedence).

These 8 cases fully exercise the §2/§3 logic with no external dependencies.
