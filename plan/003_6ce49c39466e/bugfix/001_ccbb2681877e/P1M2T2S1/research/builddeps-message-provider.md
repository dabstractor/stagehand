# Research — P1.M2.T2.S1: Use message role's provider in buildDeps for manifest selection

> Scope: the PROVIDER half of Issue 2 (PRD §9.15 FR-R3). The sibling task P1.M2.T1.S1 (in flight) fixes
> the MODEL+REASONING half (Render call sites in `generate.CommitStaged` + `runPipeline`). THIS task fixes
> the PROVIDER half: `buildDeps` selects the manifest from `cfg.Provider` (global) — it must instead
> resolve the `message` role's provider so `--message-provider X` / `[role.message] provider = "X"`
> selects manifest X on the single-commit path. The two tasks are COMPLEMENTARY and NON-OVERLAPPING
> (different functions; the sibling explicitly discards the provider with `_` and leaves manifest
> selection to buildDeps).

---

## 1. The fix (one-line, surgical)

`pkg/stagecoach/stagecoach.go` `buildDeps` (L316). Today the provider-name resolution starts at **L324**:

```go
func buildDeps(cfg config.Config, repoDir string) (generate.Deps, error) {
	overrides, err := provider.DecodeUserOverrides(cfg.Providers)
	if err != nil {
		return generate.Deps{}, fmt.Errorf("provider overrides: %w", err)
	}
	reg := provider.NewRegistry(overrides)

	name := cfg.Provider          // ← L324: GLOBAL provider only (the bug)
	if name == "" {               // ← L325: auto-detect block (unchanged, stays)
		var installed []string
		for _, m := range reg.List() {
			if reg.IsInstalled(m) {
				installed = append(installed, m.Name)
			}
		}
		name = reg.DefaultProvider(installed)
	}
	if name == "" { … error … }
	m, ok := reg.Get(name)        // ← now Gets the message-resolved provider
	… Validate, IsInstalled pre-flight, Output/StripCodeFence bridge …
	return generate.Deps{Git: git.New(repoDir), Manifest: m}, nil
}
```

**Replace L324** `name := cfg.Provider` with:

```go
	// FR-R3: the single-commit path's active role is "message" — resolve its provider per-field
	// (flag > env > [role.message] > global [defaults]) so --message-provider / [role.message] select
	// the manifest. No message override ⇒ cfg.Provider (back-compatible). Model/reasoning are resolved
	// at the Render call sites (generate.CommitStaged / runPipeline); buildDeps selects only the provider.
	msgProvider, _, _ := config.ResolveRoleModel("message", cfg)
	name := msgProvider
```

The `if name == ""` auto-detect block (L325) is UNCHANGED — it still runs when BOTH the message role
provider AND the global provider are empty (the common no-config case). Everything downstream
(`reg.Get(name)`, `Validate`, the `IsInstalled` pre-flight, the Output/StripCodeFence bridge) now
operates on the message-resolved provider. `config` is ALREADY imported (resolveConfig uses config.Load).

---

## 2. Why this is correct + back-compatible

`config.ResolveRoleModel("message", cfg)` (internal/config/roles.go:41) resolves per-field with
precedence `flag > env > [role.message] > [defaults] global > shipped default`. For the PROVIDER field
specifically, three cases:

| cfg state | ResolveRoleModel("message", cfg) provider | buildDeps `name` | Behavior |
|-----------|-------------------------------------------|------------------|----------|
| `Roles["message"].Provider="beta"`, `cfg.Provider="alpha"` | `"beta"` (per-role wins) | `"beta"` | **FIX**: message override selects manifest beta |
| `Roles` empty/nil, `cfg.Provider="alpha"` | `"alpha"` (falls through to global) | `"alpha"` | **unchanged** (byte-identical to today) |
| `Roles` empty, `cfg.Provider=""` | `""` (global empty) | `""` → auto-detect block runs | **unchanged** (registry DefaultProvider) |
| `Roles["message"].Provider=""`, `cfg.Provider="alpha"` | `"alpha"` (per-field inheritance) | `"alpha"` | **unchanged** (FR-R3/FR37a per-field: empty role field inherits global) |

So the COMMON case (no message override) is byte-identical: `ResolveRoleModel` returns `cfg.Provider`
exactly, and `name := msgProvider` == the old `name := cfg.Provider`. Only an EXPLICIT message-provider
override changes behavior — precisely the FR-R3 use case ("`--message-provider` must work").

The model/reasoning returns are DISCARDED with `_` here — they are resolved at the Render call sites by
the sibling task (P1.M2.T1.S1). buildDeps only selects WHICH manifest; Render (on that manifest) gets the
model/reasoning. Keeping the concerns split avoids a "declared and not used" compile error and avoids
overlapping with the sibling task.

---

## 3. buildDeps is the single manifest-selection site for the single-commit path

Grep confirms `buildDeps` is called from EXACTLY ONE place: `GenerateCommit` at stagecoach.go:136.
`GenerateCommit` then branches:
- **Common path** (`!DryRun && SystemExtra==""`): `generate.CommitStaged(ctx, deps, cfg)` — uses `deps.Manifest`.
- **Advanced path** (`DryRun || SystemExtra!=""`): `runPipeline(ctx, deps, cfg, …)` — uses `deps.Manifest`.

Both consume the SAME `deps` built by `buildDeps`. So fixing `buildDeps` makes `--message-provider`
select the manifest for BOTH the common and dryRun/extra paths. (The decompose multi-commit path does
NOT use buildDeps — it uses `decompose.ResolveRoles` per decompose/roles.go:8 — so it is unaffected, and
its message role already works via internal/decompose/message.go.)

---

## 4. The sibling-task boundary (P1.M2.T1.S1 — NON-overlapping)

The sibling task edits:
- `internal/generate/generate.go` `CommitStaged` — Render uses `ResolveRoleModel("message")` for MODEL/REASONING.
- `pkg/stagecoach/stagecoach.go` `runPipeline` — same (the shared `model` local var + Render).
- A stub `STAGECOACH_STUB_ARGSFILE` knob for test observability.

It explicitly:
- Uses `_, msgModel, msgReasoning := …` (discards the provider) — "provider→manifest selection stays in
  buildDeps (P1.M2.T2.S1)."
- Does NOT touch `buildDeps`.

MY task edits `buildDeps` (different function). The two share only the file `pkg/stagecoach/stagecoach.go`,
in DIFFERENT functions (`buildDeps` L316 vs `runPipeline` L401) → **no textual merge conflict**. Together
they deliver full FR-R3 on the single path: provider (this task) + model/reasoning (sibling).

---

## 5. Test design (white-box buildDeps; stub-backed providers)

`pkg/stagecoach/stagecoach_test.go` is `package stagecoach` (WHITE-BOX) — it can call the unexported
`buildDeps` directly, and already imports `config` + `internal/stubtest`. The existing harness:
`stubtest.Build(t)` compiles a real stub binary to an absolute temp path; providers are registered with
`command = "<abs path>"` so `reg.IsInstalled` (exec.LookPath on an absolute path) succeeds.

`buildDeps`'s `IsInstalled` pre-flight (L~359) REJECTS any provider whose command is not on $PATH — so
the test CANNOT use the literal built-in names "pi"/"claude" (their binaries aren't on PATH in CI).
Instead register TWO stub-backed providers via `cfg.Providers` (the raw `map[string]map[string]any` that
`DecodeUserOverrides` decodes), both pointing at the stub binary:

```go
bin := stubtest.Build(t)
cfg := config.Defaults()
cfg.Providers = map[string]map[string]any{
	"alpha": {"command": bin, "prompt_delivery": "stdin", "print_flag": "-p", "output": "raw", "strip_code_fence": true},
	"beta":  {"command": bin, "prompt_delivery": "stdin", "print_flag": "-p", "output": "raw", "strip_code_fence": true},
}
cfg.Provider = "alpha"
cfg.Roles = map[string]config.RoleConfig{"message": {Provider: "beta"}} // --message-provider beta
deps, err := buildDeps(cfg, t.TempDir())   // repoDir unused by buildDeps (no git ops inside)
// assert deps.Manifest.Name == "beta"
```

Two cases (TDD per the contract):
- **Override**: `cfg.Provider="alpha"`, `cfg.Roles["message"]={Provider:"beta"}` → `deps.Manifest.Name == "beta"`.
- **No-override regression**: `cfg.Provider="alpha"`, `cfg.Roles=nil` → `deps.Manifest.Name == "alpha"` (proves back-compat).

Notes:
- `buildDeps` does NOT run any git command (it only constructs `generate.Deps{Git: git.New(repoDir), …}`);
  `repoDir = t.TempDir()` is fine (no git repo needed for the buildDeps call itself).
- The stub-backed providers Validate cleanly (Name from key + Command=bin non-empty + valid enums).
- `ResolveRoleModel` reads `cfg.Roles[role]` safely on a nil map (returns zero/false) — so `cfg.Roles=nil`
  in the no-override case is fine.
- OPTIONAL complement: a public-API `GenerateCommit(…, DryRun:true)` test asserting `Result.Provider`
  (Result.Provider == manifest name, per existing stagecoach_test.go:221-222). The direct `buildDeps` test
  is the precise, deterministic proof for THIS task; the public-API test is belt-and-suspenders.

---

## 6. Scope fences (NOT this task)

- **NOT model/reasoning resolution** — that is the sibling P1.M2.T1.S1 (Render call sites). buildDeps
  selects only the provider/manifest; discard model+reasoning with `_`.
- **NOT the decompose path** — it uses `decompose.ResolveRoles` (not buildDeps); its message role already
  works. buildDeps is single-commit-only.
- **NOT config-layer** — the loaders/flags already populate `cfg.Roles["message"]` correctly (the bug
  report confirmed this); the ONLY defect is buildDeps reading `cfg.Provider` instead of resolving.
- **NOT render.go/manifest.go/registry.go** — buildDeps consumes them unchanged.
- **NOT docs** — DOCS: none (FR-R3 `--message-provider` was already documented; this makes it truthful).
- **NOT the stale 6-provider error list** at buildDeps L~337 (`strings.Join([]string{"pi","claude",…})`)
  — that is a pre-existing cosmetic staleness (missing agy/qwen-code), out of scope for this fix. Leave it.

---

## 7. Validation commands (verified against this codebase)

```bash
# Build + vet + fmt (go vet catches a forgotten `_` on the model/reasoning returns → unused-var)
go build ./...
go vet ./...
gofmt -l pkg/stagecoach/

# The two new white-box tests (override + no-override regression):
go test -race ./pkg/stagecoach/ -v -run 'BuildDeps|MessageProvider'

# Full suite (the no-override path is byte-identical → no regression in any existing test):
go test -race ./...

# go.mod/go.sum unchanged (no new dep; config already imported):
git diff --exit-code go.mod go.sum
```
