# S2 Implementation Notes — Populate pi ReasoningLevels with verified --thinking tokens

> Scope: P1.M1.T1.S2 — populate `builtinPi().ReasoningLevels` with the VERIFIED `pi --help`
> `--thinking <level>` tokens so the FR-R6 reasoning feature is functional for pi (unfetting the shipped
> `planner=high` default). Data-only; the Render guard is already correct. Mirrors S1 (claude) exactly,
> different provider/tokens. Verified against live source 2026-07-02. **S1 (claude) is being implemented
> in parallel** — treat its PRP as a contract; coordinate the shared `docs/providers.md` row.

## 0. The input contract (what S2 consumes)

- `builtinPi()` (builtin.go:42) returns a Manifest with `ReasoningLevels: nil` + a TODO comment at
  lines 52-55. The struct field `ReasoningLevels map[string][]string` EXISTS (manifest.go:89).
- The Render guard (render.go:124-126) `if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0`
  is ALREADY CORRECT — appends tokens AFTER the model flag. Only the manifest DATA is missing.
- The shipped `planner = high` default (config/roles.go defaultRoleReasoning) is currently inert for pi.

## 1. The verified tokens (external_deps.md §pi — AUTHORITATIVE; do NOT guess)

`pi --help` exposes `--thinking <level>` with values **off|minimal|low|medium|high|xhigh**. Stagecoach's
level set is **off|low|medium|high** — map ONLY the overlap:

| stagecoach level | tokens |
|----------------|--------|
| `"high"`   | `["--thinking", "high"]` |
| `"medium"` | `["--thinking", "medium"]` |
| `"low"`    | `["--thinking", "low"]` |
| `"off"`    | NO entry (natural zero-value no-op — `off` is `--thinking off`'s own zero, and stagecoach's
              `off` means "no reasoning control" anyway). Leave `off` OUT of the map. |

`minimal`/`xhigh` have no stagecoach level — NOT mapped. The map is exactly:
`{"high": {"--thinking", "high"}, "medium": {"--thinking", "medium"}, "low": {"--thinking", "low"}}`.

## 2. The exact builtin.go edit (builtinPi)

Remove the TODO comment (lines 52-55) and add the map literal. The contract says insert **after the
BareFlags block (before the TooledFlags comment)** — honor the contract placement:

```go
		BareFlags: []string{ ... },                 // (unchanged, closes ~line 63)
		// REASONING LEVELS (v3; §12.1, FR-R6). pi exposes `--thinking off|minimal|low|medium|high|xhigh`
		// (verified `pi --help`, external_deps.md §pi). off ⇒ no entry (natural zero no-op); stagecoach's
		// level set is off|low|medium|high, so minimal/xhigh are not mapped. Tokens append after the model flag.
		ReasoningLevels: map[string][]string{
			"high":   {"--thinking", "high"},
			"medium": {"--thinking", "medium"},
			"low":    {"--thinking", "low"},
		},
		// TOOLED MODE (v2 §11.5 — the stager role). ...   // (the existing long TooledFlags comment)
```

ALSO update the function doc comment (lines 37-40): replace the "NOTE: ReasoningLevels is nil (absent)
in the shipped default. FR-D5 requires verification ..." block with a note that pi now populates
high/medium/low via `--thinking` (verified), off ⇒ no-op. (Placement is cosmetic — struct/map compare is
order-independent; the contract's "after BareFlags, before TooledFlags" is honored.)

## 3. THREE parity surfaces, TWO reflect.DeepEqual tests (the #1 failure mode)

Adding `ReasoningLevels` to `builtinPi()` breaks parity UNLESS both fixtures carry the identical
`[reasoning_levels]` table:

| surface | location | enforcing test |
|---------|----------|----------------|
| `builtinPi()` | builtin.go:42 | (the source of truth) |
| `piTOML` const | builtin_test.go:16 | `TestBuiltinManifests_DecodeParity` (:366, `{"pi", builtinPi(), piTOML}` :372) |
| `providers/pi.toml` | providers/pi.toml | `TestProviderReferenceFiles_DecodeParity` (referencefiles_test.go:39; `{"pi","providers/pi.toml"}` :19) |

Append to BOTH the `piTOML` const AND `providers/pi.toml`:
```toml
[reasoning_levels]
high = ["--thinking", "high"]
medium = ["--thinking", "medium"]
low = ["--thinking", "low"]
```

GOTCHA — TOML key order: `bare_flags`/`tooled_flags` are ARRAY values (top-level keys), NOT tables, so
all top-level keys already precede any `[table]`. Append `[reasoning_levels]` at the END (after
`strip_code_fence = true`); once a `[table]` header appears, later keys belong to it (there are none).

GOTCHA — `piTOML` currently contains a STALE `default_provider = ""` line (the DefaultProvider FIELD
was removed in plan 003; go-toml v2 ignores unknown keys, so it's harmless and the baseline is GREEN).
Do NOT touch it — leave it exactly as-is; only ADD `[reasoning_levels]` at the end. Comments are
stripped on decode, so only the DATA must match `builtinPi()`.

## 4. The tests

### (a) Extend `TestBuiltinManifests_PiFields` (builtin_test.go:242)
After the existing field assertions, add:
```go
	// ReasoningLevels: high/medium/low populated (verified pi --thinking); off absent (no-op)
	if m.ReasoningLevels == nil || len(m.ReasoningLevels["high"]) == 0 {
		t.Errorf("ReasoningLevels missing 'high' entry: %v", m.ReasoningLevels)
	}
	if _, ok := m.ReasoningLevels["off"]; ok {
		t.Errorf("ReasoningLevels should NOT have an 'off' entry (off ⇒ no-op)")
	}
```

### (b) Add `TestRender_PiReasoningThinkingTokens` (render_test.go; package provider)
Uses the REAL `builtinPi()`. pi has `ProviderFlag="--provider"` → use model `"zai/glm-5.2"` (FR-R5b
fold). Traced Args for `Render("zai/glm-5.2", "", "", "high")`:
`["--provider","zai","--model","glm-5.2","--thinking","high","--no-tools",...,"-p"]`.
```go
func TestRender_PiReasoningThinkingTokens(t *testing.T) {
	m := builtinPi() // the REAL built-in
	for _, lvl := range []string{"high", "medium", "low"} {
		s, err := m.Render("zai/glm-5.2", "", "", lvl) // FR-R5b fold: --provider zai --model glm-5.2
		if err != nil { t.Fatalf("%s: %v", lvl, err) }
		if !containsPair(s.Args, "--thinking", lvl) {
			t.Errorf("pi %s: want --thinking %s in %v", lvl, lvl, s.Args)
		}
	}
	for _, lvl := range []string{"off", ""} {
		s, err := m.Render("zai/glm-5.2", "", "", lvl)
		if err != nil { t.Fatalf("%q: %v", lvl, err) }
		if containsToken(s.Args, "--thinking") {
			t.Errorf("pi %q: want NO --thinking token in %v", lvl, s.Args)
		}
	}
}
```
Reuses `containsPair` (render_test.go:437) + `containsToken` (:447). Place near the existing reasoning
tests. Do NOT modify existing synthetic-manifest reasoning tests.

## 5. docs/providers.md — COORDINATE with S1 (claude) on the SAME row

Line 35 is the `reasoning_levels` row. **S1 (claude) edits this SAME description cell** to note claude's
`--effort`. Write the FINAL-STATE cell covering BOTH providers so the row is correct regardless of
land-order:

> `... Appended after the model flag at render. pi populates high/medium/low via \`--thinking\` (verified
> \`pi --help\`); claude via \`--effort\` (verified \`claude --help\`); all other built-ins are nil
> (graceful no-op).`

Keep the DEFAULT column `nil (none)` (schema default). If S1's claude text is already present, MERGE
(append the pi clause); if absent, write the full combined text. Leave line 59's general sentence alone.

## 6. Scope boundary — what S2 does NOT do

- NOT `builtinClaude` or any other builtin (claude = S1; the other 6 stay nil — no verified reasoning
  control, external_deps.md §"Providers with NO known reasoning control").
- NOT `render.go`/`manifest.go`/`merge.go` (guard/field/Resolve/merge already correct).
- NOT Issue 2 (message-role routing = P1.M2), Issue 3 (index-sync = P1.M3).
- NOT the stale `default_provider = ""` lines in piTOML/providers/pi.toml (harmless; out of scope).
- NOT PRD.md / tasks.json / prd_snapshot.md / plan/*.

## 7. Sources

- `external_deps.md` §pi (the verified `--thinking` flag + value set).
- `issue_findings.md` Issue 1 (root cause: every built-in ships ReasoningLevels=nil).
- `P1M1T1S1/PRP.md` (the claude sibling — exact structural template for the parity surfaces + test).
- PRD §9.15 FR-R6, §12.1 reasoning_levels; `internal/provider/{builtin,builtin_test,render_test}.go`,
  `internal/provider/manifest.go`, `providers/pi.toml`, `docs/providers.md`.
