# Research: Go `encoding/json` Unmarshal into `map[string]any`

## Summary
When unmarshaling JSON into `map[string]any`, Go uses a fixed set of concrete types (`string`, `float64`, `bool`, `nil`, `map[string]any`, `[]any`). Extract a field with the comma-ok type assertion. `json.Unmarshal` rejects trailing non-whitespace data but tolerates leading/trailing whitespace; a UTF-8 BOM will cause an error.

## Findings

### 1. JSON-to-Go type mapping for `interface{}`/`any` targets

> **Source:** [`encoding/json` — func Unmarshal](https://pkg.go.dev/encoding/json#Unmarshal) (section "To unmarshal JSON into an interface value")

| JSON type | Go concrete type |
|-----------|----------------|
| `string`  | `string`       |
| `number`  | `float64`      |
| `boolean` | `bool`         |
| `null`    | `nil`          |
| `object`  | `map[string]any` |
| `array`   | `[]any`        |

The docs state verbatim: *"To unmarshal JSON into an interface value, Unmarshal stores one of these in the interface value: `bool`, `float64`, `string`, `[]any`, `map[string]any`, `nil`"* ([pkg.go.dev](https://pkg.go.dev/encoding/json#Unmarshal)). Also confirmed in the Go blog post ["JSON and Go"](https://go.dev/blog/json).

### 2. Type assertion to extract a string

Use the **comma-ok idiom** on the value pulled from the map:

```go
var obj map[string]any
if err := json.Unmarshal([]byte(s), &obj); err != nil { ... }

v, ok := obj["result"].(string)
if !ok {
    // field missing, null, or non-string — handle fallback
}
```

A **type switch** is the right tool when multiple types are possible ([Go spec — Type switches](https://go.dev/ref/spec#Type_switch_statement), [Effective Go — Type switches](https://go.dev/doc/effective_go#type_switch)):

```go
switch v := obj["result"].(type) {
case string:
    // use v directly
case nil:
    // key absent or JSON null
default:
    // number, bool, object, array — fallback
}
```

### 3. Trailing garbage / extra content after JSON

`json.Unmarshal` **returns an error** if there is trailing non-whitespace content after the first valid JSON value (e.g. `{"a":1} extra` → error). The check happens in [`checkValid`](https://cs.opensource.google/go/go/+/refs/tags/go1.23.0:src/encoding/json/scanner.go;l=56) and the error is a `*json.SyntaxError` with a message like `"invalid character 'e' after top-level value"` ([scanner.go source](https://cs.opensource.google/go/go/+/refs/tags/go1.23.0:src/encoding/json/scanner.go)). Use [`json.Decoder`](https://pkg.go.dev/encoding/json#Decoder.Decode) if you need to consume multiple concatenated values.

### 4. Whitespace and BOM

**Whitespace** (space, tab, `\n`, `\r`) before or after the top-level JSON value is silently skipped — no error ([pkg.go.dev](https://pkg.go.dev/encoding/json#Unmarshal)). A **UTF-8 BOM** (`EF BB BF`) at the start is **not** whitespace and **will** cause a `json.SyntaxError` because the scanner encounters an unexpected byte before `{` or `[`. Strip it explicitly if your data source may include it.

### Recommended fallback policy

**Fall back to raw stdout text** when the field is absent or not a `string`. Do **not** stringify a non-string value (e.g. a `float64` `42` → `"42"`), because:
- The value is almost certainly a schema mismatch from the CLI, not an intentional commit message.
- Re-marshal/stringify masks errors silently.
- Treating the raw stdout as the message is the safest recovery when JSON parsing yields unexpected types.

```go
v, ok := obj["result"].(string)
if !ok {
    return rawStdout  // fallback to raw text
}
return v
```

## Sources
- **Kept:** [encoding/json — Unmarshal](https://pkg.go.dev/encoding/json#Unmarshal) — authoritative type-mapping table and whitespace behavior
- **Kept:** [JSON and Go — Go Blog](https://go.dev/blog/json) — confirms dynamic types with code example
- **Kept:** [Go spec — Type switches](https://go.dev/ref/spec#Type_switch_statement) — type switch semantics
- **Kept:** [Effective Go — Type switches](https://go.dev/doc/effective_go#type_switch) — idiomatic usage
- **Kept:** [json/scanner.go source](https://cs.opensource.google/go/go/+/refs/tags/go1.23.0:src/encoding/json/scanner.go) — trailing-content error logic
- **Dropped:** various Stack Overflow answers — secondary commentary, superseded by primary docs

## Gaps
- Exact BOM handling could be confirmed by running a tiny Go test program (`json.Unmarshal([]byte("\xEF\xBB\xBF{}"), &v)`). The behavior is well-established but not explicitly documented in the package doc.
- `json.Number` alternative (via `Decoder.UseNumber()`) is out of scope since we use `json.Unmarshal`, not `Decoder`.

```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "Produced a focused research brief covering all four research questions (type mapping, type assertion pattern, trailing garbage behavior, whitespace/BOM) with a recommended fallback policy. No code changes — research-only deliverable written to the specified output path."
    }
  ],
  "changedFiles": [],
  "testsAddedOrUpdated": [],
  "commandsRun": [],
  "validationOutput": [
    "Brief covers all 4 sub-questions with primary-source citations (pkg.go.dev/encoding/json, Go spec, Effective Go, Go source).",
    "Type-mapping table includes all 6 JSON types. Type-assertion pattern shows both comma-ok and type switch.",
    "Trailing-garbage and BOM behaviors documented with scanner.go source reference.",
    "Fallback recommendation: fall back to raw stdout, do not stringify non-string values."
  ],
  "residualRisks": [
    "BOM behavior is based on source-level knowledge, not an explicit package-doc statement; a quick runtime test would confirm."
  ],
  "noStagedFiles": true,
  "diffSummary": "No diffs — research-only deliverable written to /home/dustin/projects/stagecoach/plan/001_f1f80943ac34/P1M2T6S1/research/go-json-map-types.md",
  "reviewFindings": [
    "no blockers"
  ],
  "manualNotes": "This is a research subagent task; no implementation files were changed. The brief recommends falling back to raw stdout when the JSON field is absent or non-string, rather than stringifying the value."
}
```
