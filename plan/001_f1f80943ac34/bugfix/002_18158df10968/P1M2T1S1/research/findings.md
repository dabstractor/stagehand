# P1.M2.T1.S1 — Research Findings (synthesis)

Full root-cause + blast-radius trace: `../architecture/issue_analysis.md` ISSUE 2 (verified SAFE +
mechanical). This note captures the exact, compiler-verified facts the S1 implementer needs.

## The type change (config package only)

`Config.Output` is `string` (`internal/config/config.go:35`) → change to `*string`.
- `boolPtr` lives at `config.go:7`. ADD `func strPtr(s string) *string { return &s }` next to it.
- `Defaults()` (`config.go:67-70`) seeds `Output: "raw"` AND `StripCodeFence: boolPtr(true)` → REMOVE
  BOTH initializers. Both fields then default to `nil`. (The manifest's own `Resolve()` supplies the
  §12.1 `raw`/`true` fallbacks — `manifest.go:138-148`, `DefaultOutput`/`DefaultStripCodeFence` — so
  Defaults() no longer needs to. That is the ENTIRE point of this fix: make `[generation]` a true
  opt-in override, not an always-on default.)
- `fileGeneration.Output` (`file.go:41`) STAYS a plain `string` (go-toml decodes it fine). ONLY the
  resolved `Config.Output` becomes a pointer.

## Loader edits (pointer-symmetric with StripCodeFence)

- `file.go` `materialize` (159-160): `if g.Output != "" { o := g.Output; c.Output = &o }`.
- `file.go` `overlay` (210-211): `if src.Output != nil { dst.Output = src.Output }` (pure pointer copy;
  now symmetric with the StripCodeFence line directly below it, 213-214 — UNCHANGED).
- `git.go` `loadGitConfig` (127): `} else if found { c.Output = &v }`. SAFE: each `if v, found, err := …`
  is a fresh short-variable declaration (NOT a loop var), so `&v` is a distinct address each time —
  exactly like the existing `c.StripCodeFence = &v` at git.go:152-155 (UNCHANGED).

## Confirmed: nothing else in the config layer touches these fields

- `load.go` (loadEnv/loadFlags/Load): grep for `Output|StripCodeFence` → ZERO matches. ✓
- `cmd/config.go`: writes a STATIC `exampleConfigTemplate` string (no `Config{}` literal, no
  `toml.Marshal`). ✓ So no Config-marshal test path exists in the CLI.

## ⚠️ CRITICAL sequencing gotcha — the repo is transiently non-compiling after S1

The ONLY non-test, out-of-package consumer of `config.Config.Output` is `pkg/stagehand/stagehand.go:206-208`:
```go
if cfg.Output != "" {   // compile-break: *string vs string
    o := cfg.Output
    m.Output = &o
}
```
This is **S2's** scope (`P1.M2.T1.S2`): S2 changes it to `if cfg.Output != nil { m.Output = cfg.Output }`.
Therefore, after S1 lands:
- ✅ `go build ./internal/config/...` + `go vet ./internal/config/...` + `go test ./internal/config/...` = GREEN.
- ❌ `go build ./...` (whole repo) FAILS at `pkg/stagehand/stagehand.go:206` — EXPECTED, fixed by S2.

The S1 implementer MUST NOT "fix" `pkg/stagehand/stagehand.go` (scope violation; collides with S2).
The S1 success gate is scoped to `./internal/config/...`.

## Test edits (compiler-driven — every `cfg.Output != ""` / `Output: "..."` is a break pointing here)

- `config_test.go` `TestDefaults` (46-47): assert `c.Output == nil` and `c.StripCodeFence == nil`.
- `config_test.go` `TestTOMLMarshalKeysAndNoColorExclusion` (47-72): marshals `Defaults()`; with nil
  pointers go-toml OMITS the keys (free omitempty — same behavior `provider.Registry.MarshalTOML`
  already relies on in prod). FIX: marshal a Config with EXPLICIT values
  (`c := Defaults(); c.Output = strPtr("raw"); c.StripCodeFence = boolPtr(true)`) so the key-presence
  assertions for `output`/`strip_code_fence` still validate the toml tags.
- `file_test.go`: `TestLoadTOMLValid` (82-83 deref), `TestOverlayPartial` (114 struct-literal →
  `strPtr("json")`; 119-120 deref; StripCodeFence assertion → `want nil`), `TestOverlayStripCodeFenceFalse`
  Case 2 (407 literal → `strPtr("json")`; preserve "nil src must not clobber" by pre-setting
  `dst.StripCodeFence = boolPtr(true)` then asserting it survived — see PRP).
- `git_test.go`: `TestLoadGitConfig_*` (111-112 deref; 133 `cfg.Output != nil`; 345-346 → `want nil`,
  "default preserved" now means nil).

## Why no external/online research is warranted

This is a mechanical pointer-ification WITHIN the existing Go config package, using the project's own
established `*bool`/`boolPtr` pattern. No external libraries, APIs, or new patterns are introduced.
All needed "documentation" is the binding architecture doc + the compiler error list.
