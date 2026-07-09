# P2.M2.T1.S1 — Per-Item Evidence (collected at HEAD `bb3cb3b`)

This file is the raw evidence backing every PASS verdict in the PRP. All commands run from
`/home/dustin/projects/stagecoach`. State at HEAD: `bb3cb3b` ("Add gemini removal verification
PRP and evidence"), a descendant of `2f77bd0` ("Re-verify and fix agy manifest against v1.1.0").

## 0. Baseline
- `go build ./...` → EXIT 0 ("BUILD OK")
- `go test ./internal/provider/... -run 'Agy|RenderedCommand' -v` → all PASS, including
  `TestBuiltinManifests_AgyFields` and `TestBuiltinManifests_RenderedCommand_Agy`.
- `func builtinAgy()` lives at `internal/provider/builtin.go:198` (doc comment `:154-197`;
  return literal `:199-217`). The item_description's "builtin.go:199-217" = the function body.

## 1. builtinAgy() field-by-field (the 8 contract fields)

| # | field | expected | actual (line) | verdict |
|---|-------|----------|---------------|---------|
| a | PrintFlag | `strPtr("")` non-nil empty | `strPtr("")` @ :205 | PASS |
| b | ModelFlag | `strPtr("--model")` (NOT `-m`) | `strPtr("--model")` @ :206 | PASS |
| c | PromptDelivery | `strPtr("stdin")` | `strPtr("stdin")` @ :204 | PASS |
| d | DefaultModel | `strPtr("Gemini 3.5 Flash (Low)")` | `strPtr("Gemini 3.5 Flash (Low)")` @ :207 | PASS |
| e | BareFlags | `[]string{"--mode","plan"}` (NOT approval-mode) | `[]string{"--mode","plan"}` @ :210-212 | PASS |
| f | ListModelsCommand | `[]string{"agy","models"}` | `[]string{"agy","models"}` @ :203 | PASS |
| g | Experimental | `boolPtr(true)` | `boolPtr(true)` @ :215 | PASS |
| h | TooledFlags | `nil` (omitted) | omitted; `// TooledFlags: nil …` comment @ :216 | PASS |

Source (awk `NR>=198 && NR<=217` builtin.go) confirms every field verbatim.

## 2. role_defaults.go agy column (display labels)

Block at `internal/config/role_defaults.go:65-72`:
- `planner` = `"Gemini 3.5 Flash (High)"` @ :68 → PASS (flagship/smart, high thinking)
- `message` = `"Gemini 3.5 Flash (Low)"` @ :70 → PASS (fast/cheapest, low thinking)
- `arbiter` = `"Gemini 3.5 Flash (Medium)"` @ :71 → PASS (mid tier)
- `stager` = `""` @ :69 → PASS (nil TooledFlags → bootstrap applies FR-D4 fallback)

All four use the display-label form (verbatim `agy models` label incl. "(Low/Medium/High)"
reasoning suffix), NOT API-style ids. The FR-D5 provenance comment (`:28-32`, refreshed
2026-07-03 per the table) documents the agy label refresh.

## 3. Rendered command (no -p; stdin delivery)

`TestBuiltinManifests_RenderedCommand_Agy` (builtin_test.go:686-701) asserts argv =
`["agy","--model","Gemini 3.5 Flash (Low)","--mode","plan"]` with NO `-p` and the
`<sys>\n\n<user payload>` piped to stdin (model="" → default label). The corrected render from
§12.5.1 is:
`agy --model "Gemini 3.5 Flash (Low)" --mode plan < <sys+user payload via stdin>` → PASS.

## 4. providers/agy.toml reference-doc parity (mirrors builtinAgy byte-for-byte)

`providers/agy.toml` field lines (the body, not the header):
- `name = "agy"`, `detect = "agy"`, `command = "agy"`
- `list_models_command = ["agy", "models"]`
- `prompt_delivery = "stdin"`
- `print_flag = ""` (NON-NIL empty; documented value-taking `-p` breakage)
- `model_flag = "--model"`
- `default_model = "Gemini 3.5 Flash (Low)"`
- `system_prompt_flag = ""`, `provider_flag = ""`
- `bare_flags = ["--mode", "plan"]`
- `output = "raw"`, `strip_code_fence = true`, `experimental = true`
- `tooled_flags` intentionally omitted (nil) → cannot stager.
→ All match builtinAgy(). PASS. The test fixture `agyTOML` (builtin_test.go:142-160) is identical.

## 5. In-code documentation (Mode A — the 2026-07-08 verification record)

- builtin.go doc comment carries "2026-07-08" at `:156, :175, :182, :193`; the return-literal
  field comments carry it at `:203, :205, :206, :207, :211`. → present and dated. PASS.
- providers/agy.toml header carries "2026-07-08" at `:20, :27, :35, :45, :63` (rendered-command,
  model-names, divergence, experimental, list_models_command). → present and dated. PASS.

## 6. Architecture audit cross-reference
`plan/013_b8a415cc6e79/architecture/code_gemini_agy_audit.md` "Check 1" pre-confirmed all eight
manifest fields MATCH at builtin.go:199-217, with the same field→line table. This subtask is the
independent re-verification of that audit row.
