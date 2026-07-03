# P1.M6.T2.S1 — `config init --interactive` wizard: design decisions

Researched 2026-07-03. This file records the WHY behind every decision in the PRP. Read it if a task
seems under-specified.

## 1. The keystone contract: byte-identity with `GenerateBootstrapConfig`

FR-L3: the wizard "writes the same file FR-B1 writes." The work item's TEST contract is explicit:
"assert the written TOML equals the equivalent GenerateBootstrapConfig output **with edits applied**."

That demands a single shared generator that takes the chosen provider AND per-role model edits, where
**no edits ⇒ byte-identical to today's `config init`**. The existing seam:

- `config.GenerateBootstrapConfig(prov string) string` (bootstrap.go) — PURE, no I/O; calls
  `buildBootstrapConfig(target, installed)`.
- `config.buildBootstrapConfig(target, installed)` — the PURE TOML generator. Models come from
  `DefaultModelsForProvider(target)`; pi is blanked; stager falls back when the target can't stage.

**Decision:** refactor `buildBootstrapConfig` to take a 3rd param `overrides map[string]string`
(role→model), applied to the `models` map AFTER the pi-blank/stager-fallback computation. Add the
public seam `config.GenerateBootstrapConfigWithOverrides(prov, overrides)`. Refactor
`GenerateBootstrapConfig(prov)` to delegate (`return GenerateBootstrapConfigWithOverrides(prov, nil)`).
With nil overrides the output is byte-identical → the golden test is trivially correct by construction
and the contract holds.

Override application points (so the stager-provider routing decision is preserved):
- planner / message / arbiter: `models[role] = v` when present in overrides.
- stager: `stagerModel = v` when present (stagerName stays the fallback result — the user edits the
  MODEL, not which provider stages).

## 2. The pi NOTE must not lie when the wizard fills models

`buildBootstrapConfig` emits a NOTE for pi: "The shipped per-role models are empty so you can supply
your own backend/model." If the wizard FILLS pi's models (overrides non-empty), that sentence is false.

**Decision:** branch the note on `piBlanked && !overridesHasAny(overrides)`:
- no overrides → the existing "empty, supply your own" note (byte-identical, golden test passes).
- overrides present → a shorter "pi is a multi-backend provider; models carry the inference/ prefix
  (e.g. `zai/glm-5.2`)" note (true, format-focused).

This keeps byte-identity for the no-edit case and correctness for the edit case.

## 3. TTY gate + piped-stdin testing (the tension)

FR-L3: "TTY-gated (non-TTY → exit 1 pointing at plain config init)." But the test contract wants
"answers piped" through a reader. A hard `ui.IsTerminal(os.Stdin)` gate would make piped-stdin tests
fail (piped stdin is not a TTY).

**Decision — split the gate from the IO:**
- The PURE wizard `runInteractiveWizard(r io.Reader, w io.Writer, reg, installed, default) (...)` reads
  answers from `r`, writes prompts to `w`. NO `os.Stdin`, NO `IsTerminal` inside → fully unit-testable
  with `strings.NewReader` + `*bytes.Buffer`.
- The TTY gate is a package-level overridable var:
  `var interactiveStdinIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }`.
  `runConfigInitInteractive` checks it first; non-TTY → `exitcode.New(exitcode.Error, …)` pointing at
  plain `config init`. The happy-path Execute test sets `interactiveStdinIsTTY = func() bool { return
  true }` and pipes answers via `rootCmd.SetIn(strings.NewReader(...))` → cobra's `cmd.InOrStdin()`.
- The non-TTY-gate Execute test runs unmodified: test-runner stdin is not a TTY → gate fires → exit 1.

This reuses the existing `ui.IsTerminal` precedent (FR51 color gating, output.go).

## 4. Multi-backend prefix prompting (FR-D2 / FR-R5b)

Work item: "Multi-backend providers (pi, opencode) must PROMPT for the `inference/` model prefix and
never guess."

render.go:110 enforces FR-R5b ONLY for `provider_flag` providers (pi): a non-empty bare model (no `/`)
on pi is a HARD ERROR. opencode has NO provider_flag, so Render passes the model verbatim — but
opencode's correct form is STILL `backend/model` (e.g. `openai/gpt-5.4`). A bare opencode model is
functionally wrong though not a Render error.

**Decision — predicate `needsInferencePrefix(name, providerFlag)`:**
`name == "pi" || name == "opencode" || providerFlag != ""`.
- Matches the work item's explicit (pi, opencode) set.
- For these providers, an EDITED model that is non-empty and lacks `/` is REJECTED with a re-prompt
  ("include the inference backend as a prefix, e.g. `zai/glm-5.2`"). Loop until valid or blank.
- (Mirrors `isMultiBackendText` in cmd/config.go, which is `name == "pi" || providerFlag != ""` — but
  ADDS opencode, which the migration classifier deliberately omits because opencode never carried a v2
  default_provider.)

`providerFlag` is read from the registry manifest: `reg.Get(name)` → merged-but-UNRESOLVED (registry.go
comment), so handle nil: `pf := ""; if m.ProviderFlag != nil { pf = *m.ProviderFlag }`.

## 5. Detection without a loaded config (config init skips config.Load)

`shouldSkipConfigLoad` (root.go:211) returns true for `init`/`path`/`upgrade` → during
`config init --interactive`, `Config()` is nil.

**Decision:** reuse the cmd-package helpers from providers.go (same package, no import):
- `newRegistry()` — builds the merged registry; when `Config()` is nil it uses NO overrides
  (built-ins only). Exactly what `config init` needs.
- `installedNames(reg)` — detected provider names.
- `resolvedDefault(Config(), reg, installed)` — when cfg is nil → `reg.DefaultProvider(installed)`
  (FR-D1 detected default). This is the highlighted default in the menu.

No reach into `config.bootstrapProviderNames` (unexported, wrong package).

## 6. Menu ordering in FR-D1 priority

The detected set should list in priority order (pi first), not `reg.List()` ascending. But
`provider.preferredBuiltins` is UNEXPORTED, and cmd/config.go's LOCAL copy is STALE (missing
`qwen-code`) — a known footgun.

**Decision:** add a tiny exported `provider.PreferredBuiltins() []string` (returns the slice; or a
copy). Use it to order the detected menu. This is NOT a manifest-schema change (no Manifest field, no
providers/*.toml, no builtin.go edit) → does not collide with P1.M6.T1.S1's contract. The default
HIGHLIGHT still comes from `resolvedDefault`/`DefaultProvider` (correct internally).

## 7. Flag composition (`--interactive` + `--force` / `--provider` / `--template`)

Work item: "composes with --force (and remains mutually sensible with --provider/--template)."
- `--interactive --force` → composes: wizard runs, overwrites an existing file (force check in the
  shared file-write).
- `--interactive --provider <name>` → pre-selects `<name>` (skip the provider-pick prompt, go straight
  to role editing). `<name>` validated via `reg.Get` (known built-in) exactly as runConfigInit does
  today; detection is NOT required for an explicit `--provider` pin (parity with `config init
  --provider claude`).
- `--interactive --template` → MUTUALLY EXCLUSIVE usage error (exit 1): `--interactive` writes a
  populated config; `--template` writes the inert reference — choose one.

## 8. Where the code lives (minimal blast radius)

- EDIT `internal/config/bootstrap.go`: `buildBootstrapConfig` +overrides param; new
  `GenerateBootstrapConfigWithOverrides`; `GenerateBootstrapConfig` delegates; conditional pi note.
- EDIT `internal/provider/registry.go`: add `PreferredBuiltins() []string` (one function).
- EDIT `internal/cmd/config.go`: register `--interactive` flag (one line in `init()`); branch at the
  TOP of `runConfigInit` (`if interactive { return runConfigInitInteractive(cmd, args) }`). Extract a
  shared `writeBootstrapFile(cmd, path, content, force) error` (force-check + MkdirAll + WriteFile)
  used by BOTH the non-interactive and interactive paths (DRY; one place for the file-write contract).
- CREATE `internal/cmd/config_init_interactive.go`: `flagConfigInteractive` var, the TTY-gate package
  var, `runConfigInitInteractive`, the PURE `runInteractiveWizard`, `needsInferencePrefix`, menu/render
  helpers. Read answers via `cmd.InOrStdin()` (cobra `SetIn` in tests).
- CREATE `internal/cmd/config_init_interactive_test.go`: pure wizard tests (golden byte-identity +
  edits + multi-backend re-prompt + invalid-choice re-prompt), the non-TTY gate Execute test, the
  Execute happy-path file-write test (forced TTY + SetIn), `--interactive --template` conflict,
  `--interactive --provider` pre-select, `--interactive --force` overwrite.
- EDIT `docs/cli.md`: `--interactive` row in the config init flag table + a paragraph + example.
- EDIT `docs/configuration.md`: an "Interactive bootstrap" note in the Bootstrap section.

## 9. Answer protocol (for scriptable tests)

Each prompt reads ONE line from the reader (`bufio.Scanner` or `bufio.Reader`). Empty line = accept
default; non-empty = the value (validated/re-prompted for multi-backend). Prompts in order:
1. Provider pick (skipped when `--provider <name>`): "<n>. <name> [(default)] — choose: ".
2. For each role in [planner, stager, message, arbiter]: "<role> model [<default>]: ".
Accept-all-defaults for a 4-role provider (no --provider) = five empty lines: `"\n\n\n\n\n"`.
EOF (reader exhausted) on a prompt = error (so truncated scripts fail loudly, not silently).
