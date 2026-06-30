# Research Findings — P1.M4.T1.S1 (Apply cfg.Output/StripCodeFence onto the manifest in buildDeps)

Research-only scout. All paths repo-relative. Working tree: `/home/dustin/projects/stagehand`.

## 1. The seam that is missing

`provider.ParseOutput` (internal/provider/parse.go:44-52) reads ONLY the manifest's pointer fields:
```go
func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool) {
	r := m.Resolve()              // nil-pointer-safe deref; copy
	s := strings.TrimSpace(raw)
	if *r.StripCodeFence {        // ← reads MANIFEST, not cfg
		s = strings.TrimSpace(stripCodeFence(s))
	}
	switch *r.Output {            // ← reads MANIFEST, not cfg
	case "json": ...
	case "raw":  ...
	}
```
ParseOutput has NO access to `config.Config`. Its only inputs are the raw stdout + the `Manifest`.

`buildDeps` (pkg/stagehand/stagehand.go:154-198) builds the manifest from `cfg.Providers` (`[provider.X]`
overrides merged onto built-ins) and returns `generate.Deps{..., Manifest: m}`. It NEVER references
`cfg.Output` / `cfg.StripCodeFence`. → the `[generation]` output/strip_code_fence values are dropped on
the floor. This is the exact seam to close.

## 2. The cfg fields are fully resolved before buildDeps

- `config.Config` (internal/config/config.go:31-32): `Output string`, `StripCodeFence bool` — plain,
  always-resolved scalars.
- `Defaults()` (config.go:64-65): `Output: "raw"`, `StripCodeFence: true`.
- file loader (file.go:151-155 materialize, 202-206 overlay) populates them from `[generation]`.
- git-config loader (git.go:124-128 / 152-156) populates them from `stagehand.output` /
  `stagehand.stripCodeFence` (camelCase key for the bool).
- load.go layers them via overlay. No env/CLI layer sets them (intentional).
- CONCLUSION: `cfg.Output` is always a non-empty "raw"|"json" (or whatever string the user typed);
  `cfg.StripCodeFence` is always a concrete bool by the time buildDeps runs.

## 3. The manifest pointer-field design (why copy-to-local matters)

Manifest.Output is `*string`, Manifest.StripCodeFence is `*bool` (manifest.go:78-80). The
pointer-scalar design exists because go-toml/v2 has no omitempty: a nil = "inherit built-in", a non-nil
(even `*false` / `*""`) = "override". `Resolve()` (manifest.go:151-159) fills nils with defaults but
PRESERVES present values — the correctness keystone.

So assigning `m.Output = &o` (non-nil) = "this value wins". Assigning into a LOCAL `o := cfg.Output`
(not `&cfg.Output`) avoids aliasing the cfg value's address (cfg is a value-param copy; the local makes
the ownership unambiguous). The contract/decision D4 mandate this copy-to-local form.

## 4. Precedence semantic: "[generation] (broader) overrides [provider.X] (narrower)"

`reg.Get(name)` already merged any `[provider.<name>]` body onto the built-in BEFORE buildDeps returns.
Applying cfg.Output/StripCodeFence AFTER that merge means the `[generation]` value wins over a
per-provider `[provider.X]` value. This is the documented intent ("generation config tunes ALL
providers") and matches decisions.md D4. ParseOutput then reads the (now cfg-overridden) manifest
pointers — no parser change needed.

## 5. Exact insertion point + ordering

buildDeps current tail (verified):
```go
	m, ok := reg.Get(name)              // line ~176
	...
	if err := m.Validate(); err != nil { // line ~181  ← Validate() runs here
		return generate.Deps{}, fmt.Errorf("provider %q: %w", name, err)
	}
	// Pre-flight (Issue 3, P1.M2.T1.S1): IsInstalled LookPath
	if !reg.IsInstalled(m) {            // line ~189
		return generate.Deps{}, fmt.Errorf("provider %q: command %q not found. Is the agent installed?", ...)
	}
	// ← INSERT cfg→manifest bridge HERE (after Validate + pre-flight, before return)
	return generate.Deps{Git: git.New(repoDir), Manifest: m}, nil  // line ~197
```
Contract item 3: apply AFTER Validate() and AFTER the Issue 3 pre-flight. No re-Validate() needed —
ParseOutput's `switch *r.Output` `default` branch already treats an unrecognized output value as raw
(graceful). (The seam doc's A.8 suggested re-validating; the binding contract + D4 do NOT — follow the
contract.)

## 6. The asymmetry: Output is guarded, StripCodeFence is unconditional

Contract item 3:
```go
if cfg.Output != "" {     // guard: "" would mean "don't override" (defensive; default is always "raw")
    o := cfg.Output
    m.Output = &o
}
scf := cfg.StripCodeFence // ALWAYS apply (default true) — broader setting wins, even for the default
m.StripCodeFence = &scf
```
StripCodeFence has NO guard because cfg.StripCodeFence is always a concrete bool post-Defaults; applying
it unconditionally is what makes `[generation]` (the broader layer) consistently override the per-manifest
default. Keep the asymmetry EXACTLY as specified.

## 7. Out of scope (do NOT touch)

- The file.go "cannot set false via file" quirk (materialize line ~153: `if g.StripCodeFence { c.StripCodeFence = true }`).
  Via the FILE loader strip_code_fence can only be turned ON; turning it OFF requires git-config
  (`stagehand.stripCodeFence false`) or a direct cfg injection. Contract item 3 explicitly forbids fixing
  this here. The git-config path still exercises the false case end-to-end.
- The config-package loader tests that assert cfg.Output/cfg.StripCodeFence are populated — they stay.
- Per-provider `[provider.X] output/strip_code_fence` override — still works (merged before buildDeps).

## 8. Docs touched (Mode A — rides with this implementing subtask)

- `docs/configuration.md`: "Built-in defaults" table lists `output`/`strip_code_fence` (lines ~78-79).
  Affirm these now apply to PARSING (override the per-manifest defaults). Also the `[generation]`
  example block (lines ~47-54) and the git-config-keys table (lines ~112-116, which currently LISTS
  provider/model/timeout/auto_stage_all but OMITS stagehand.output / stagehand.stripCodeFence) — the
  git-config keys for these two fields exist (git.go reads them) and should be reflected for accuracy.
- `internal/cmd/config.go` exampleConfigTemplate `[generation]` block (lines 154-162): the
  output/strip_code_fence comment lines are now accurate. Add a one-line note that these tune ALL
  providers (the broader layer), per contract item 5.

## 9. Boundaries with sibling/other subtasks

- S2 (P1.M4.T1.S2) owns the dedicated end-to-end TEST that cfg Output/StripCodeFence reach ParseOutput.
  S1 ships the bridge + docs only. S1's own validation = build clean, vet clean, FULL existing suite
  still green (the change is additive; buildDeps' pre-flight/Validate contract is untouched). The existing
  `TestGenerateCommit_MissingProviderCommand_Issue3` (stagehand_test.go:485) must still pass unchanged —
  it exercises the same buildDeps tail below the insertion point.
- P1.M4.T2 (Issue 7 auto-stage notice) is unrelated — no shared code.
- P1.M5 (Mode B docs sweep) runs last and depends on every implementing subtask; S1's Mode A doc edits
  are the authoritative source P1.M5 will reconcile against.

## 10. Validation commands (verified from Makefile)

- `go build ./...` (Makefile `build` compiles the binary; `go build ./...` covers all packages)
- `go vet ./...`
- `go test -race ./...` (Makefile `test`)
- `make lint` (golangci-lint; .golangci.yml present)
- `make coverage-gate` enforces >=85% on internal/{git,provider,generate,config} (not affected by this
  pkg/stagehand + docs change, but run it to confirm no regression).
