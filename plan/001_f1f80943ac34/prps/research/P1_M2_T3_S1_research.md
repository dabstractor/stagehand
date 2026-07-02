# Research — P1.M2.T3.S1: internal/provider/builtin.go — 6 verified manifests

## Goal
Fix the exact manifest values + the TOML round-trip test approach BEFORE writing the PRP.
All findings below are verified empirically on the host (go 1.26.4, go-toml/v2 v2.4.2).

## 1. The Manifest type ALREADY EXISTS (dependency P1.M2.T1.S1 DONE)
- `internal/provider/manifest.go` defines `Manifest` (all §12.1 fields, `toml:"..."` tags) +
  `Rendered` + `Render`. This task CONSUMES it; it does NOT redefine it.
- `internal/provider/manifest_test.go` ALREADY contains an unexported `sixBuiltinManifests()
  map[string]Manifest` that returns the six manifests (the Render golden-table oracle). Its own
  doc comment (line 11) says: "the executor/builtin.go task (M2.T3.S1) will own them".
  => The authoritative in-codebase values already exist. `Builtins()` MUST return manifests
     BYTE-IDENTICAL to `sixBuiltinManifests()`, then `sixBuiltinManifests()` should be refactored
     to `return Builtins()` (single source of truth, DRY).

## 2. The six manifests — exact field values (from sixBuiltinManifests() oracle == external_deps §B)
pi        §B.1: stdin, -p, --model, glm-5-turbo, --system-prompt, --provider, 6 no-* bare_flags,
                 RetryInstruction = §12.1 default; Output=raw, StripCodeFence=true.
claude    §B.2: stdin, -p, --model, sonnet, --system-prompt, BareFlags = 7 SLICE elements encoding
                 5 logical flags (see §3), Output=raw, StripCodeFence=true.
gemini    §B.3: positional, PrintFlag="", -m, gemini-2.5-pro, SystemPromptFlag="", BareFlags=
                 ["--approval-mode","default"], Output=raw, StripCodeFence=true.
opencode  §B.4: Subcommand=["run"], positional, PrintFlag="", -m, DefaultModel="", SystemPromptFlag="",
                 BareFlags=nil, Output=raw, StripCodeFence=true.
codex     §B.5 (CORRECTED §C.2): Subcommand=["exec"], PromptDelivery=stdin (NOT positional),
                 -m, DefaultModel="", SystemPromptFlag="", BareFlags=["--sandbox","read-only",
                 "--ask-for-approval","never","--ephemeral"], Output=raw, StripCodeFence=true.
cursor    §B.6: command="agent", detect="agent", positional, -p, --model, DefaultModel="",
                 SystemPromptFlag="", BareFlags=["--mode","ask","--trust"], Output=raw,
                 StripCodeFence=true.

## 3. RESOLVED: the "claude BareFlags has 5 entries" ambiguity
The work-item spot-check says "claude BareFlags has 5 entries". Verified:
- The SLICE has 7 elements: ["--setting-sources","","--tools","","--disable-slash-commands",
  "--no-chrome","--no-session-persistence"] (because --setting-sources and --tools each take an
  empty-string ARGUMENT that disables them).
- "5" = 5 distinct logical FLAGS (PRD §12.4's 3 + §C.1's 2 added = 5). Verified by counting
  leading-dash tokens.
=> PRP tells the agent: assert `len(claude.BareFlags) == 7` (the literal slice) OR count
   distinct `-`-prefixed tokens == 5. Do NOT assert `len == 5` (it FAILS). The §C.1 correction
   is "add --disable-slash-commands + --no-chrome" = 2 flags onto PRD's 3.

## 4. CRITICAL GOTCHA: go-toml/v2 round-trips nil slice -> []string{}
Empirically verified: marshaling a Manifest with `Subcommand: nil` or `BareFlags: nil` produces
`subcommand = []` / `bare_flags = []`, and unmarshaling that yields a NON-nil `[]string{}`.
Therefore `reflect.DeepEqual(origManifest, roundTripped)` FAILS for opencode (nil BareFlags),
pi (nil Subcommand), etc.
=> The TOML round-trip test MUST normalize nil <-> empty before comparing. Two verified
   approaches (both PASS for claude-7-elem + opencode-nil + all-zero):
   A) normalize slices to nil when len==0, then reflect.DeepEqual; OR
   B) Marshal(orig) == Marshal(Unmarshal(Marshal(orig))) (idempotency; immune to nil/empty).
   Recommend A as the primary "struct survives round-trip" check (it catches a missing toml
   tag as a field-level loss), with spot-checks for the §C-corrected fields.

## 5. go-toml/v2 becomes a DIRECT dependency — `go mod tidy` is now SAFE
- builtin_test.go imports `github.com/pelletier/go-toml/v2` for the round-trip test.
  (manifest.go itself imports NO go-toml; the §12.1 tags were inert reflect strings — that
  contract is preserved.)
- Verified: `go build ./...` and `go vet ./internal/provider/` are CLEAN even if go.mod still
  marks go-toml `// indirect` (Go 1.26 tolerates importing an indirect dep in a _test.go).
- Verified: `go mod tidy` flips go-toml/v2 to the direct `require` block, keeps cobra direct,
  keeps mousetrap+pflag indirect, go.sum unchanged. Safe + makes go.mod honest.
  => PRP gate: after creating builtin_test.go, run `go mod tidy` (single command). This UNDOES
     the prior task's "no tidy" caveat — but that caveat existed ONLY because go-toml had no
     importer yet; now it does.

## 6. Package-doc ownership (DOCS = Mode A)
- `manifest.go` line 1 OWNS the `// Package provider` doc comment (dependency task). Go allows
  exactly ONE package comment; builtin.go must use a plain `package provider` line and NOT
  repeat it (duplicate-package-comment lint).
- The work item's DOCS line ("package doc listing the six built-ins + pointer to §12.8") is
  satisfied by a comprehensive godoc comment ON the `Builtins()` function itself: list the six
  names, note the §C corrections, point to §12.8 for user-defined providers, note this is the
  base for registry overrides (M2.T3.S2) + config-override base (M5.T3.S1) + reference manifests
  (M8.T1.S1). This godoc feeds PROVIDERS.md (M8.T4.S2). Create NO README/docs/ files here.

## 7. Validation gates (verified valid on host)
- `go build ./internal/provider/`, `go vet ./internal/provider/`,
  `test -z "$(gofmt -l internal/provider/)"`, `go test ./internal/provider/`,
  `go test ./...`, `go mod tidy` (single commands). Full suite currently GREEN.

## 8. Scope boundaries
- CREATE: internal/provider/builtin.go (+ builtin_test.go).
- MODIFY: internal/provider/manifest_test.go ONLY to change sixBuiltinManifests() body to
  `return Builtins()` (DRY; preserves the golden-table contract).
- MODIFY: go.mod via `go mod tidy` (go-toml -> direct).
- DO NOT: redefine Manifest/Render, touch manifest.go/parse.go/parse_test.go/main.go/Makefile,
  create README/docs/providers TOML, run agents at test time (executor = M2.T4.S1).
