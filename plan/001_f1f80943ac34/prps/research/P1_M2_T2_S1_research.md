# Research — P1.M2.T2.S1: internal/provider/parse.go — parseOutput(raw, m) (msg, ok)

## Goal of this note
Pin the exact byte-exact behavior of the §12.9 output-parsing pipeline BEFORE writing the
PRP, resolve the three genuine ambiguities (closer search scope, the "fallback flag for
verbose logging", and the unexported-vs-exported question), and validate the algorithm with
a throwaway Go prototype that passes every contract test case.

---

## 1. The three authoritative sources (they agree verbatim)

| Source | Pipeline statement |
|---|---|
| **PRD §12.9** | (1) `s=TrimSpace(raw)`; (2) if `strip_code_fence` && starts with ``` or ~~~ → drop opener line + everything from last closer; (3) switch output: raw→msg=s; json→Unmarshal, on fail find first `{`…last `}` balanced substring + retry, extract `obj[json_field]` string, ANY fail→fall back to raw + set parse-fallback flag; (4) `\r\n`→`\n`, collapse 3+ `\n`→2; (5) `msg=TrimSpace(msg); ok=msg!=""` |
| **decisions.md §4** | Identical 5-step pipeline. Table-driven tests: raw, fenced-raw, fenced-json, json-in-prose, fallback-to-raw, empty→(ok=false). |
| **Work item LOGIC** | Identical. "drop the fence-opener line (incl. any language tag) and everything from the LAST matching closer onward"; "find first '{' and matching last '}' (brace-balanced substring) and retry". |

All three agree. The work-item contract is THE spec.

## 2. Ambiguity #1 — WHERE to search for the fence closer (RESOLVED)

PRD: "remove the first line (the fence opener) and everything from the **last** fence
closer onward." Work item: "everything from the **LAST matching** closer onward".

Decision: **two-phase on the trimmed string `s`**, using the SAME 3-char marker for opener
and closer (``` matches ```, ~~~ matches ~~~ — `fence := s[:3]`):
1. Drop the opener line: `if nl := IndexByte(s,'\n'); nl!=-1 { s = s[nl+1:] } else { s = "" }`.
2. In the REMAINING body, find the LAST index of `fence`; cut from there: `if last := LastIndex(s, fence); last!=-1 { s = s[:last] }`.
3. Re-trim.

Searching the body (post-opener-removal) is correct: it never matches the opener itself,
and it finds the trailing closer. Validated for ``` / ~~~ / lang-tag / no-closer cases
(prototype, §6). `fence := s[:3]` is safe because step 1 only enters when `HasPrefix(s,
"```")` or `HasPrefix(s,"~~~")`, so `s` is ≥3 chars and the prefix IS the marker.

Edge — pathological nested fences (` ```\na\n```\nb\n``` `) keep the middle block; this is
the literal "last closer" contract and is acceptable (commit messages do not nest fences).

## 3. Ambiguity #2 — the "fallback flag for verbose logging" (RESOLVED — keep pure)

The signature is FIXED at `parseOutput(raw string, m Manifest) (string, bool)`. There is no
return slot for a flag, and `msg=s` (raw) is byte-identical whether json succeeded or fell
back, so the CALLER cannot distinguish the two paths.

Decision: **parseOutput stays PURE** — no `log`, no `os`, no config/verbose import (those
would break the milestone's "self-contained; no import cycle" + testability constraints).
The fallback-to-raw BEHAVIOR (json fails ⇒ msg=s, ok = msg!="") is the testable contract and
IS implemented. Verbose logging of the fallback EVENT is owned by the generate layer
(P1.M6.T1.S1), which holds the `--verbose` UI handle (internal/ui). This PRP states this
explicitly so the implementer does NOT try to wire logging into parseOutput. (If M6 later
needs the signal, it can export a richer helper or wrap parseOutput — out of scope here.)

## 4. Ambiguity #3 — exported vs unexported (RESOLVED — follow the task)

The task title/signature is `parseOutput` (lowercase p) ⇒ **unexported**, package-private to
`internal/provider`. Consequence: the test file MUST be white-box (`package provider`, like
manifest_test.go) to call it. The cross-package consumer (generate, P1.M6.T1.S1) will, when
it arrives, either export it (`ParseOutput`) or call through an exported provider entry
point (e.g. an Executor wrapper). That export decision belongs to M6, NOT this task. This
task implements the lowercase `parseOutput` exactly as specified.

## 5. JSON extraction details

- Decode target: `var obj map[string]any` (go 1.22 ⇒ `any` is fine).
- Strict string extraction: `obj[field].(string)`. A non-string value (number/bool/null) OR
  a missing field OR an empty JSONField ⇒ type assertion fails ⇒ `extractJSON` returns
  `("", false)` ⇒ parseOutput falls back to raw (msg=s). This matches "extract as a string"
  and the robust-fallback philosophy (§17.4). (Coercing numbers via fmt.Sprint is NOT done —
  a commit message is always a JSON string field.)
- Two attempts: (a) `json.Unmarshal([]byte(s), &obj)` on the whole; (b) on failure, slice
  `s[firstIndexByte('{') : lastIndexByte('}')+1]` (guard `end>start`) and retry. This
  recovers json-embedded-in-prose. ANY remaining failure ⇒ raw fallback.
- json.Unmarshal never panics on bad input (returns error), so no recover needed.

## 6. Prototype validation (throwaway, all 12 cases PASS)

Algorithm implemented + run against every contract case + edge cases. Results:
```
[OK] raw clean                   → "feat: add parser", ok=true
[OK] fenced-raw                  → "feat: add parser", ok=true
[OK] fenced-raw langtag          → "feat: x", ok=true
[OK] tilde fence                 → "feat: x", ok=true
[OK] fenced-json                 → "feat: x", ok=true
[OK] json-in-prose               → "feat: x", ok=true
[OK] malformed-json-fallback     → "{\"result\": feat: x}" (raw), ok=true
[OK] empty                       → "", ok=false
[OK] newline-normalize           → "feat: x\n\nbody" (\r\n + 3+NL collapse), ok=true
[OK] json-not-string-field       → "{\"result\": 42}" (raw fallback), ok=true
[OK] fence-strip-off-keeps-fence → fence kept when StripCodeFence=false, ok=true
[OK] whitespace-only             → "", ok=false
```
This validates: fence strip (```/~~~/langtag), closer search on body, json whole+balanced
retry, prose-embedded json, strict-string extraction fallback, \r\n+collapse normalization,
empty/whitespace → ok=false.

## 7. Newline normalization

- Order: convert `\r\n`→`\n` FIRST (`strings.ReplaceAll`), THEN collapse 3+ `\n`→`\n\n`.
  Collapsing after CRLF conversion is correct (a `\r\n\r\n\r\n` becomes `\n\n\n` then `\n\n`).
- Idiomatic collapse: package-level `var collapseNewlines = regexp.MustCompile(\`\n{3,}\`)`,
  `collapseNewlines.ReplaceAllString(msg, "\n\n")`. (regexp is stdlib; compile-once is fine.)
- Lone `\r` (old-Mac) is OUT OF SCOPE — PRD only names `\r\n`. Do not touch lone `\r`.

## 8. Output-mode default

`m.Output == ""` is the §12.1/§12.9 default and means raw. Treat `""` and `"raw"`
identically (do NOT route empty Output through the json branch). Only `m.Output == "json"`
takes the JSON path. (Mirrors how manifest.go treats empty PromptDelivery as stdin.)

## 9. Purity / scope / import discipline

- Imports needed: `encoding/json`, `regexp`, `strings`. ALL stdlib. NO `os`, `log`,
  `fmt` (no error formatting needed — parseOutput returns ok=false, not an error), NO
  config/provider-executor imports, NO go-toml/cobra.
- parseOutput reads ONLY m.Output / m.JSONField / m.StripCodeFence (the three §12.1 fields
  marked "consumed by the output parser, not by Render" in manifest.go). It ignores all
  other Manifest fields.
- Do NOT modify manifest.go, manifest_test.go, main.go, Makefile, go.mod, go.sum, internal/ui.
- Do NOT run `go mod tidy` (stdlib-only file; tidy is risky per P1.M1.T1.S1).
- parse.go is a SIBLING file in the SAME `package provider` — use a plain `package provider`
  line (the package doc is owned by manifest.go; do NOT repeat `// Package provider`).

## 10. Validation gates (verified valid on host)
go 1.26.4 present; `go test ./...` currently GREEN (internal/provider manifest tests +
internal/ui). Gates (all single-command, no &&/heredoc/for):
`go build ./internal/provider/`, `go vet ./internal/provider/`,
`test -z "$(gofmt -l internal/provider/)"`, `go test ./internal/provider/`, `go test ./...`.

## 11. DOCS impact
Mode A — godoc on `parseOutput` and the unexported `extractJSON` helper citing PRD §12.9 and
§17.4 (raw-default rationale) + the fallback-to-raw behavior. No README/docs/providers TOML
created here (the providers doc surface is fed by builtin.go in M2.T3.S1 / synced in
M8.T4.S2). Consistent with the sibling PRP P1.M2.T1.S1's Mode-A godoc style.
