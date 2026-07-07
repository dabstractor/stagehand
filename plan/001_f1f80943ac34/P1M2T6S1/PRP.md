---
name: "P1.M2.T6.S1 — parseOutput pipeline (trim, fence strip, raw/json, JSON-in-prose, fallback): the FOURTH and final stage of the provider pipeline — PRD §12.9 / §9.6 (FR26–FR29) / §17.4"
description: |

  Land the SOLE subtask of Output Parsing Pipeline (P1.M2.T6): `internal/provider/parse.go` exporting
  `func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)` — the exact 5-step
  §12.9 pipeline that turns a provider's captured stdout (from P1.M2.T5.S1 `Execute`) into the commit
  message. It is the consumer of the frozen `Manifest` (P1.M2.T1.S1 — reads `Output`, `JsonField`,
  `StripCodeFence` via `Resolve()`) and the producer of `(msg, ok, fellback)` for the generate
  orchestrator (P1.M3.T4 — `ok==false` ⇒ parse-retry with `retry_instruction`, FR29) and dedupe loop
  (P1.M3.T2 — keys on `msg`). This closes the provider pipeline: manifests (T1–T3) describe the agent;
  Render (T4) composes the CmdSpec; Execute (T5) runs it; ParseOutput (T6) reads stdout.

  THE 5-STEP PIPELINE (PRD §12.9, AUTHORITATIVE):
    1. `s = strings.TrimSpace(raw)`.
    2. If `m.strip_code_fence` and `s` starts with ``` or ~~~: remove the first line (opener +
       language tag) and everything from the LAST fence closer onward. Re-trim.
    3. Switch on `m.output`:
         - raw → `msg = s`.
         - json → `json.Unmarshal` whole; on failure find first `{`…matching `}` (brace-balanced) and
           retry; extract `obj[m.json_field]` as a string. ANY failure ⇒ fall through to raw
           (`msg = s`) and set `fellback = true`.
    4. Normalize newlines: `\r\n`→`\n`; collapse 3+ consecutive `\n` to 2.
    5. `msg = strings.TrimSpace(msg)`; `ok = msg != ""`.

  INPUT: the captured stdout `string` (from `Execute`, P1.M2.T5.S1) + the `Manifest` (P1.M2.T1.S1).
  Treat P1.M2.T1.S1's `Manifest` struct + `Resolve()` as a CONTRACT (do NOT edit manifest.go).

  OUTPUT: `(msg string, ok bool, fellback bool)` — `ok==false` triggers a parse-retry with corrective
  instruction (FR29) downstream; `fellback==true` is the "json parse failed, used raw" logging flag
  (§12.9 step 3). DOCS: none — internal parser.

  ⚠️ **THE signature reconciliation — implement the WORK-ITEM signature, NOT PRD §12.9's.** PRD §12.9
  sketches `parseOutput(raw string, m Manifest) (msg string, ok bool)` (lowercase, 2 returns). The
  work item (binding) names `ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)`
  (EXPORTED, 3 returns). Implement the work-item form: EXPORTED (capital P) because the consumer is
  `internal/generate` (P1.M3.T4/T2) — a different package; and the third return `fellback` surfaces the
  "parse-fallback flag for logging" the PRD itself names in step 3 so the orchestrator need not re-
  derive it. See research design-decisions.md §0.

  ⚠️ **THE resolve call — ParseOutput calls `m.Resolve()` internally but NOT `m.Validate()`.** Parsing
  only dereferences `Output`/`JsonField`/`StripCodeFence`; if `m` is unresolved (nil pointers — e.g. a
  manifest straight from the registry, which stores merged-but-unresolved per P1.M2.T3) a bare `*m.Output`
  PANICS. `Resolve()` (P1.M2.T1.S1) guarantees every pointer non-nil on a COPY (caller's m untouched) —
  mirrors `Manifest.Render` (render.go). Validate's Name/Command requiredness is the orchestrator's
  concern, not the parser's. See research design-decisions.md §2.

  ⚠️ **THE json_field extraction — comma-ok type assertion; non-string ⇒ FALLBACK (never stringify).**
  `json.Unmarshal` into `map[string]any` decodes string→string, number→float64, null→nil, etc. The
  message field must be a `string`; use `v, ok := obj[field].(string)`. On `!ok` (absent/null/non-string)
  fall back to raw + `fellback=true`. Do NOT `fmt.Sprintf` a non-string (masks a schema mismatch and can
  produce a nonsense commit subject). See research go-json-map-types.md.

  ⚠️ **THE brace-balanced retry — track inString AND escaped flags (ASCII-safe, no rune decode).**
  Whole-string `json.Unmarshal` fails on trailing prose. The retry finds the first `{` and scans to the
  matching `}` at depth 0 — but a depth-only counter miscounts braces/quotes INSIDE string values (RFC
  8259 §7 allows `{`/`}` unescaped in strings). Track `inString bool` + `escaped bool`; re-run
  `json.Unmarshal` on the extracted substring as the final validator. Byte scanning is UTF-8-safe
  (`{`,`}`,`"`,`\` are ASCII <0x80, never continuation bytes — RFC 3629 §3). See research
  json-in-prose-brace-balanced.md.

  ⚠️ **THE normalization — literal PRD: `\r\n`→`\n` THEN collapse 3+ `\n`→2. Do NOT also handle lone `\r`.**
  Implemented with a manual `strings.Builder` pass (NO `regexp` import — keeps deps at
  `encoding/json`+`strings`; codebase has no regexp usage and a one-shot CLI needs none). Applies to
  `msg` AFTER the output-mode switch. Final `TrimSpace` is last. See research design-decisions.md §6.

  Deliverable: `internal/provider/parse.go` (`package provider`, imports `encoding/json`+`strings`) —
  `func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)` implementing the exact
  §12.9 5-step pipeline, with small unexported helpers (`stripCodeFence`, `extractJSONObject`,
  `normalizeNewlines`) for testability. PLUS `internal/provider/parse_test.go` (`package provider`) — a
  table-driven `TestParseOutput` covering every work-item scenario + edge cases. Touches ONLY these two
  NEW files — NO go.mod/go.sum change (stdlib only), NO edit to manifest.go/merge.go/builtin.go/
  registry.go/render.go/executor.go/procgroup_*.go or any frozen file.

---

## Goal

**Feature Goal**: Implement the provider pipeline's output-parsing stage (PRD §12.9 / FR26–FR29): a
pure function that converts a provider's captured stdout into the cleaned commit message, robustly
handling both `raw` (the default) and `json` output modes with a graceful fallback to raw on any JSON
failure. This is what makes raw output viable as the v1 design call (PRD §17.4): there is nothing to
escape, nothing to parse structurally in the common case, and the robust cleanup pipeline (trim →
fence-strip → normalize) handles the rare case of a model wrapping output in a code fence — while JSON
mode remains available for agents (Claude Code) whose `--output-format json` is more reliable.

**Deliverable**:
1. **CREATE** `internal/provider/parse.go` (`package provider`, imports `encoding/json`, `strings`) —
   `func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)` implementing the
   exact §12.9 5-step pipeline, plus three unexported helpers: `stripCodeFence(s string) string`,
   `extractJSONObject(s string) (string, bool)`, `normalizeNewlines(s string) string`.
2. **CREATE** `internal/provider/parse_test.go` (`package provider`, imports `strings`, `testing`) — a
   table-driven `TestParseOutput` over a `[]struct{ name; raw; manifest; wantMsg; wantOK; wantFallback }`
   slice covering every work-item scenario (raw clean / raw+fence / json valid / json-in-prose / json
   invalid→fallback / empty→ok=false / multi-newline normalization) PLUS edge cases (~~~ tilde fence,
   fence+lang tag, json_field missing, json_field non-string→fallback, strip_code_fence=false,
   nested braces in JSON strings, escaped quotes, JSON+trailing prose, \r\n normalization,
   fellback=false always in raw mode). Pure-function tests — no subprocess, no mocking.

No other files touched. **No go.mod/go.sum change** (stdlib `encoding/json`+`strings` only). NO edit to
`manifest.go`/`merge.go`/`builtin.go`/`registry.go`/`render.go`/`executor.go`/`procgroup_*.go` or any
frozen config/git file.

**Success Definition**: `go build ./...` succeeds; `go test -race ./internal/provider/` is green with
the new table-driven suite passing every scenario; `gofmt -l internal/provider/` is clean; `go vet
./internal/provider/` is clean; `golangci-lint run` (if available) clean; go.mod/go.sum byte-unchanged
(`git diff --exit-code go.mod go.sum` empty); every frozen file byte-unchanged; the returned triple
satisfies the truth table in research design-decisions.md §1 (notably `ok==false` ⇒ `msg==""`, and
`fellback` is true ONLY in json mode on JSON failure).

## User Persona

**Target User**: The generate orchestrator (P1.M3.T4 `CommitStaged` — calls `ParseOutput(stdout, m)`
right after `Execute` returns, then branches on `ok`) and the dedupe loop (P1.M3.T2 — keys on the
returned `msg`). Transitively the verbose-mode UI (P1.M4.T3.S2 — logs `fellback`). End-user persona is
"the plan-holder" / "the API-key refusenik" / "the multi-agent tinkerer" (PRD §7) running any of the 6
verified agent CLIs (pi, claude, gemini, opencode, codex, cursor) whose stdout — clean prose OR
fence-wrapped OR JSON — must be reduced to one commit message.

**Use Case**: After `Execute` captures the agent's stdout, the orchestrator calls
`ParseOutput(stdout, manifest)`; for a `raw` provider (pi/gemini/codex/cursor/opencode default) the
cleaned stdout IS the message; for a `json` provider (Claude Code with `--output-format json`,
`json_field="result"`) the configured field is extracted, with an automatic fallback to raw if the model
emits malformed JSON or omits the field. If the message ends up empty (`ok==false`), the orchestrator
retries once with the manifest's `retry_instruction` (FR29).

**User Journey**: (internal API, no new end-user surface) `Execute` → stdout string →
`ParseOutput(stdout, m)` → `(msg, ok, fellback)` → orchestrator: if `!ok` retry (FR29) else proceed to
dedupe (P1.M3.T2) → commit (P1.M3.T4). `fellback` ⇒ verbose log "json parse failed, used raw output".

**Pain Points Addressed**: Removes the fragile `sed` extraction + "no double quotes inside the message"
constraint that `commit-pi`'s JSON contract imposed (PRD §17.4). Raw + robust cleanup makes the common
case bulletproof; JSON mode with graceful fallback makes the Claude-Code case reliable; `ok==false` +
retry makes the "model added a preamble" failure self-correcting.

## Why

- **Closes the provider pipeline.** Manifests (T1–T3) → Render (T4) → Execute (T5) → **ParseOutput (T6)**.
  T6 is the final stage; without it the orchestrator (P1.M3.T4) has stdout but no message. Every
  downstream module (P1.M3.T1 prompts, P1.M3.T2 dedupe, P1.M3.T3 rescue, P1.M3.T4 orchestrator,
  P1.M3.T5 public API) is blocked until a clean `(msg, ok, fellback)` is producible.
- **Satisfies PRD §9.6 (FR26–FR29).** FR26 raw default; FR27 json alternative; FR28 the robust
  extraction pipeline (trim → fence-unwrap → json-attempt-then-balanced-substring → fallback); FR29 the
  empty/failed-parse retry trigger — `ok==false` is exactly the FR29 signal the orchestrator keys on.
- **Implements the v1 design call (§17.4).** Raw output is more robust for free-form prose than JSON;
  the cleanup pipeline (§12.9) handles fence-wrapping. JSON stays available for agents where it's
  more reliable. This subtask IS that pipeline.
- **No new user-facing surface** (PRD "DOCS: none — internal parser"). No new dependency (stdlib only).

## What

A compiled `internal/provider` package with one new exported pure function `ParseOutput` and three
unexported helpers, all in a single new file `parse.go`, with a single new table-driven test file
`parse_test.go`. No new types, no new exports beyond `ParseOutput`, no CLI, no config, no I/O. The
function is a deterministic string-in/string-out transformation parameterized by three manifest fields
(`Output`, `JsonField`, `StripCodeFence`), producing a triple whose meaning is fixed by the truth table
in research design-decisions.md §1.

### Success Criteria

- [ ] `internal/provider/parse.go` exists, `package provider`, imports EXACTLY `encoding/json` and
      `strings` (NO `regexp`, NO `fmt`, NO third-party). Defines exported
      `func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)` implementing the
      exact §12.9 5-step order: TrimSpace → (stripCodeFence if enabled) → output switch (raw/json w/
      brace-balanced retry + fallback) → normalizeNewlines → TrimSpace + `ok=msg!=""`.
- [ ] `fellback` is true ONLY in `json` mode when JSON parsing/extraction fails (whole Unmarshal fails
      AND balanced-substring Unmarshal fails, OR the field is absent/null/non-string); in that case
      `msg` is the cleaned raw `s`. `fellback` is ALWAYS false in `raw` mode.
- [ ] `extractJSONObject` tracks `inString` + `escaped` flags and finds the first `{`…matching `}` at
      depth 0, correctly ignoring braces/quotes inside JSON string values; re-runs `json.Unmarshal` on
      the result as the final validator.
- [ ] `stripCodeFence` detects a leading ```` ``` ```` or `~~~`, removes the first line (opener +
      language tag) and everything from the LAST matching closer onward, re-trims; no-closer ⇒ keeps
      the body (lenient).
- [ ] `normalizeNewlines` does `\r\n`→`\n` then collapses 3+ consecutive `\n` to exactly 2 (literal
      PRD; does NOT touch lone `\r`).
- [ ] `ParseOutput` calls `m.Resolve()` internally (nil-pointer-safe deref on a copy) but NOT
      `m.Validate()` (parsing needs only Output/JsonField/StripCodeFence).
- [ ] `internal/provider/parse_test.go` exists, `package provider`, a table-driven `TestParseOutput`
      covering ALL work-item scenarios + the edge cases enumerated in the Implementation Blueprint.
- [ ] `go build ./...` succeeds; `go test -race ./internal/provider/` green; `gofmt -l
      internal/provider/` clean; `go vet ./internal/provider/` clean.
- [ ] go.mod/go.sum byte-unchanged (`git diff --exit-code go.mod go.sum` empty).
- [ ] Every frozen file byte-unchanged (`manifest.go`/`merge.go`/`builtin.go`/`registry.go`/
      `render.go`/`executor.go`/`procgroup_unix.go` + all their `_test.go`, `internal/config/*`,
      `internal/git/*`).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the 5-step §12.9
pipeline (quoted verbatim in the description + the Goal), the four design calls (research
design-decisions.md §0–§6 — the most important read), the frozen `Manifest`+`Resolve()` contract
(manifest.go), the Go/RFC backing for the json-field type assertion (go-json-map-types.md) and the
brace-balanced extractor (json-in-prose-brace-balanced.md, with a copy-ready implementation sketch),
and the in-package table-driven test convention (manifest_test.go / executor_test.go). No
generate/CLI/signal-handler knowledge required — T6 is a single pure-function file + its tests.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M2T6S1/research/design-decisions.md
  why: the SINGLE most important read — reconciles the work-item vs PRD signature (§0, exported 3-return
       ParseOutput), the fellback truth table (§1), the internal Resolve()-not-Validate() call (§2), the
       json-field comma-ok + non-string⇒fallback policy (§3), the brace-balanced inString+escaped design
       (§4), the fence-strip opener⇒closer logic (§5), the literal-PRD normalization + no-regexp choice
       (§6), the stdlib-only constraint (§7), and the in-package table-driven test placement (§8).
  critical: §0 (exported ParseOutput, 3 returns — do NOT implement PRD's lowercase 2-return parseOutput),
       §2 (call Resolve() not Validate()), §3 (non-string field ⇒ fallback, never stringify), §1 (the
       fellback truth table — the orchestrator's contract) are the things most likely to be implemented wrong.

- docfile: plan/001_f1f80943ac34/P1M2T6S1/research/go-json-map-types.md
  why: authoritative Go encoding/json behavior for `map[string]any` decode — the JSON→Go type table
       (string→string, number→float64, null→nil, object→map[string]any, array→[]any), the comma-ok type
       assertion idiom, the trailing-content error semantics (why whole-string Unmarshal fails on prose),
       and the whitespace/BOM tolerance. Backs design-decisions §3.
  critical: the type table (a non-string field value is float64/bool/map/nil, NOT a usable message) and
       the trailing-content failure (the reason step 3's whole-Unmarshal fails on JSON-in-prose, forcing
       the brace-balanced retry). URLs: pkg.go.dev/encoding/json#Unmarshal, go.dev/blog/json.

- docfile: plan/001_f1f80943ac34/P1M2T6S1/research/json-in-prose-brace-balanced.md
  why: authoritative RFC 8259 §7 + RFC 3629 §3 backing for the brace-balanced extractor — proves braces
       are legal unescaped inside JSON strings (so a depth-only counter is WRONG), gives the concrete
       failing example (`{"msg": "She said \"hello\" {world}"}`), and provides a COPY-READY stdlib-only
       Go implementation sketch (strings.IndexByte fast-forward + byte scan + inString + escaped flags +
       depth counter). Backs design-decisions §4.
  critical: the inString+escaped state machine (without `escaped`, an escaped quote `\"` toggles inString
       wrongly and every subsequent brace miscounts) and the UTF-8 ASCII-safety note (byte scan is safe;
       no utf8.DecodeRune needed). URLs: rfc-editor.org/rfc/rfc8259#section-7, rfc-editor.org/rfc/rfc3629#section-3.

- file: internal/provider/manifest.go   (P1.M2.T1.S1 — read for the Manifest struct + Resolve(); do NOT edit)
  section: the `Manifest` struct (Output/JsonField/StripCodeFence pointer fields) + `Resolve()` (defaults
           nil Output→"raw", nil StripCodeFence→true, nil JsonField→"") + the Default* constants.
  why: the INPUT type. ParseOutput takes `m Manifest` and must dereference `*m.Output`/`*m.JsonField`/
       `*m.StripCodeFence` — which are `*string`/`*bool` and PANIC if nil. Resolve() (which ParseOutput
       calls internally per design-decisions §2) makes them safe. The `DefaultOutput`/`DefaultStripCodeFence`
       constants are the values Resolve applies.
  critical: the fields are POINTERS (not plain string/bool) — never `*m.Output` on an unresolved manifest.
       Call `r := m.Resolve()` first, then deref `*r.Output`. Do NOT edit manifest.go.

- file: internal/provider/render.go   (P1.M2.T4.S1 — read for the Resolve() call-site pattern; do NOT edit)
  section: the top of `Manifest.Render` — `r := m.Resolve()` then safe `*r.X` deref on the copy.
  why: the EXACT pattern to mirror for ParseOutput's internal Resolve() call (design-decisions §2).
       Render does Validate() THEN Resolve(); ParseOutput does Resolve() ONLY (no Validate — see §2).
  critical: Resolve() returns a COPY — the caller's manifest is untouched. ParseOutput does NOT call
       Validate() (parsing needs only the three output fields, not Name/Command).

- file: internal/provider/executor.go   (P1.M2.T5.S1 — read to confirm the upstream contract; do NOT edit)
  section: the `Execute` return signature `(stdout string, stderr string, err error)`.
  why: confirms ParseOutput's `raw` input is the `stdout` string Execute returns (the agent's captured
       stdout). stderr + err are the orchestrator's concern, NOT the parser's (ParseOutput takes only
       stdout). This is the hand-off point between T5 and T6.
  critical: ParseOutput takes ONLY `raw` (stdout) + `m` (manifest) — it does NOT see stderr or err. The
       orchestrator decides retry/rescue from err; ParseOutput decides message-extraction from stdout.

- file: internal/provider/manifest_test.go   (read for the in-package test convention; do NOT edit)
  section: the `assertStr`/`assertNilStr` helpers + the grouped, documented test-function style.
  why: the test STYLE to follow — `package provider` (white-box), table-driven or grouped functions
       each with a `// ---- Test... ----` header comment, `t.Helper()` in helpers. parse_test.go mirrors
       this. Also shows how to construct a `Manifest` literal with `strPtr`/`boolPtr` (same-package
       access to the unexported helpers) OR with Resolve() applied.
  critical: tests are IN-PACKAGE (`package provider`, not `package provider_test`) — same as every other
       _test.go in this dir. This lets the table build manifests with the unexported strPtr/boolPtr helpers
       or assert against Default* constants directly.

- url: https://pkg.go.dev/encoding/json#Unmarshal
  why: the AUTHORITATIVE semantics — the "To unmarshal JSON into an interface value" type table (string→
       string, number→float64, …), the trailing-non-whitespace error, and whitespace tolerance. Cited in
       go-json-map-types.md.
  critical: confirms a JSON string field decodes to Go `string` (so the `.(string)` assertion is correct)
       and that whole-string Unmarshal ERRORS on trailing prose (the trigger for the brace-balanced retry).

- url: https://www.rfc-editor.org/rfc/rfc8259#section-7
  why: the AUTHORITATIVE JSON string grammar — `unescaped = %x20-21 / %x23-5B / %x5D-10FFFF` includes
       `{` (0x7B) and `}` (0x7D), proving braces appear legally INSIDE string values (so a depth-only
       counter is incorrect; inString tracking is required).
  critical: the proof that extractJSONObject MUST track string state, not just brace depth.
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; require go-toml/v2 + pflag  (UNCHANGED — T6 adds NO dep: stdlib json+strings)
go.sum                          # unchanged
internal/
  config/                       # P1.M1.T4 — untouched
  generate/                     # P1.M3 (empty stub) — the FUTURE consumer of ParseOutput; untouched
  git/                          # P1.M1.T2/T3 — untouched
  provider/                     # T1..T5 created; this subtask adds the parse stage
    manifest.go / manifest_test.go          # P1.M2.T1.S1 — Manifest + Resolve() + Default*   (CONTRACT — do NOT edit)
    merge.go / merge_test.go                # P1.M2.T1.S2                                    (do NOT edit)
    builtin.go / builtin_test.go            # P1.M2.T2/S3                                    (do NOT edit)
    registry.go / registry_test.go          # P1.M2.T3                                       (do NOT edit)
    render.go / render_test.go              # P1.M2.T4 — CmdSpec + Render (Resolve pattern)   (do NOT edit)
    executor.go                             # P1.M2.T5.S1 — Execute returns (stdout,...)      (do NOT edit)
    procgroup_unix.go / procgroup_windows*.go  # P1.M2.T5.S1/S2                              (do NOT edit)
    executor_test.go                        # P1.M2.T5.S1                                    (do NOT edit)
    parse.go                                # NEW (this subtask) ← ParseOutput + helpers
    parse_test.go                           # NEW (this subtask) ← table-driven TestParseOutput
cmd/stagecoach/main.go           # stub — untouched
Makefile                        # build/test(-race)/coverage/lint/clean/help — untouched
```

### Desired Codebase tree with files to be added

```bash
internal/
  provider/
    parse.go         # NEW — func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)
                     #       + unexported stripCodeFence / extractJSONObject / normalizeNewlines
    parse_test.go    # NEW — table-driven TestParseOutput (all scenarios + edge cases)
# manifest/merge/builtin/registry/render/executor/procgroup_* UNCHANGED. go.mod/go.sum UNCHANGED.
# Every other file UNCHANGED. After T6: the provider pipeline (manifest→render→execute→parse) is complete.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (signature — exported 3-return, NOT PRD's lowercase 2-return): implement
//   func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)
// The PRD §12.9 narrative shows `parseOutput(...) (msg string, ok bool)` — that is the PRE-fallback-flag
// sketch. The work item (binding) adds the EXPORTED capital + the `fellback bool` return. Exported
// because the consumer is internal/generate (a different package). See research design-decisions.md §0.

// CRITICAL (Resolve, not Validate): the Manifest Output/JsonField/StripCodeFence fields are *string/*bool
// POINTERS. Dereferencing a nil one PANICS. A manifest obtained from the registry may be unresolved
// (P1.M2.T3 stores merged-but-unresolved). ParseOutput MUST call `r := m.Resolve()` first and deref `*r.X`.
// Do NOT call Validate() — parsing needs only the three output fields, not Name/Command. Resolve() copies;
// the caller's manifest is untouched. Mirrors render.go's `r := m.Resolve()`. See design-decisions §2.

// CRITICAL (fellback truth table): fellback is true ONLY in json mode on JSON failure (both Unmarshal
// attempts fail, OR the field is absent/null/non-string); then msg = cleaned raw s. fellback is ALWAYS
// false in raw mode (nothing failed). ok = (msg != "") in BOTH modes. ok==false ⇒ msg=="". See §1.

// CRITICAL (non-string json_field ⇒ fallback, NEVER stringify): json.Unmarshal into map[string]any
// decodes string→string, number→float64, null→nil, object→map, array→[]any. Use `v, ok := obj[field].(string)`.
// On !ok fall back to raw + fellback=true. Do NOT fmt.Sprintf a non-string (a float64 42→"42" masks a
// schema mismatch and can yield a nonsense commit subject). See go-json-map-types.md.

// CRITICAL (brace-balanced retry needs inString AND escaped): whole-string json.Unmarshal fails on
// trailing prose (`{"a":1} extra` → error). The retry scans from the first `{` to the matching `}` at
// depth 0 — but a depth-ONLY counter miscounts braces/quotes INSIDE string values (RFC 8259 §7 allows
// `{`/`}` unescaped in strings). MUST track inString bool + escaped bool (escaped handles `\"` so it does
// NOT toggle inString). Re-run json.Unmarshal on the extracted substring as the final validator. See
// json-in-prose-brace-balanced.md for the copy-ready sketch + the failing example.

// CRITICAL (UTF-8 safety of byte scan): scanning s byte-by-byte for '{','}','"','\\' is SAFE in UTF-8 —
// these are all ASCII (<0x80) and RFC 3629 §3 guarantees ASCII bytes never appear as continuation bytes.
// NO utf8.DecodeRune / range-rune needed. (range over string yields runes and would need index math;
// prefer `for i := 0; i < len(s); i++ { c := s[i] }`.)

// GOTCHA (fence opener determines the closer): if s starts with ``` the closer searched (strings.LastIndex)
// is ```; if it starts with ~~~ the closer is ~~~. Do NOT search for the other fence token. Remove the
// WHOLE first line (opener + language tag, e.g. ```json) and everything from the LAST closer onward, then
// re-trim. No closer found (opener only) ⇒ keep the body after the opener line (lenient; don't return "").

// GOTCHA (stripCodeFence is a PREFIX check): only a LEADING fence triggers stripping. A ``` that appears
// mid-message (not at the start) is left alone. PRD §12.9 step 2: "if s starts with ``` or ~~~".

// GOTCHA (normalization is LITERAL PRD): step 4 is `\r\n`→`\n` THEN collapse 3+ consecutive `\n` to 2.
// Do NOT also convert lone `\r` (old-Mac) — the PRD does not list it; adding it is scope creep. Implement
// with a manual strings.Builder pass; do NOT import regexp (the codebase has no regexp usage and a one-shot
// CLI needs none — keeps imports to encoding/json+strings). Applies to msg AFTER the output switch.

// GOTCHA (order is load-bearing): step 2 (fence-strip) runs BEFORE step 3 (output switch) on `s`; step 4
// (normalize) runs AFTER step 3 on `msg`; step 5 (final TrimSpace + ok) is LAST. For json mode the
// extracted field value is normalized too (it may contain \r\n or 3+ newlines). Do not normalize before
// extraction or trim before fence-strip.

// GOTCHA (step 3 raw branch does NOT set fellback): `case "raw": msg = s` — fellback stays its zero value
// (false). Only the json branch can set fellback=true. Ensure fellback is declared once (`fellback := false`)
// and set only in the json fallback path.

// GOTCHA (in-package tests): parse_test.go is `package provider` (white-box), NOT `package provider_test`.
// This matches every other _test.go in internal/provider/ and lets the table use the unexported
// strPtr/boolPtr helpers + reference Default* constants. No external process, no mustBin, no mocking —
// ParseOutput is a pure function.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/provider/parse.go
package provider

import (
	"encoding/json"
	"strings"
)

// ParseOutput is the fourth and final stage of the provider pipeline (PRD §12.9 / §9.6 FR26–FR29):
// manifests (T1–T3) describe the agent; Render (T4) composes the CmdSpec; Execute (T5) runs it and
// captures stdout; ParseOutput (T6) turns that stdout into the commit message. It is a pure function
// over the captured stdout string and the manifest's three output fields.
//
// THE 5-STEP PIPELINE (PRD §12.9, AUTHORITATIVE — implement in EXACTLY this order):
//
//  1. s = strings.TrimSpace(raw).
//  2. If m.strip_code_fence and s starts with ``` or ~~~: remove the first line (fence opener +
//     language tag) and everything from the LAST fence closer onward. Re-trim.
//  3. Switch on m.output:
//       raw  → msg = s.
//       json → json.Unmarshal whole into map[string]any; on failure find the first '{' and the matching
//              '}' (brace-balanced substring) and retry; extract obj[m.json_field] as a string. ANY
//              failure ⇒ fall through to raw (msg = s) and set fellback = true.
//  4. Normalize newlines: \r\n → \n; collapse 3+ consecutive \n to 2.
//  5. msg = strings.TrimSpace(msg); ok = msg != "".
//
// RETURN CONTRACT (the orchestrator's, P1.M3.T4 + dedupe P1.M3.T2):
//   - msg:    the cleaned commit message ("" if the output was empty after cleanup).
//   - ok:     msg != "". ok==false ⇒ the orchestrator retries once with m.retry_instruction (FR29).
//   - fellback: true ONLY in json mode when JSON parsing/extraction failed and the cleaned raw stdout
//             was used instead (the "parse-fallback flag for logging" of §12.9 step 3). ALWAYS false
//             in raw mode. A pure logging signal — it does NOT change retry behavior (that's `ok`).
//
// WHY EXPORTED + 3 RETURNS (not PRD's lowercase 2-return parseOutput): the consumer is internal/generate
// (P1.M3.T4/T2), a different package ⇒ capital P. The third return surfaces the fallback flag so the
// orchestrator/verbose-UI can log it without re-deriving it. See research design-decisions.md §0/§1.
//
// WHY Resolve() NOT Validate(): the Output/JsonField/StripCodeFence fields are *string/*bool POINTERS;
// dereferencing a nil one panics. A manifest from the registry may be unresolved (P1.M2.T3). Resolve()
// (P1.M2.T1.S1) guarantees every pointer non-nil on a COPY (caller's m untouched) — mirrors render.go.
// Validate()'s Name/Command requiredness is the orchestrator's concern, not the parser's.
func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool) {
	r := m.Resolve() // nil-pointer-safe deref; copy — caller's m untouched (mirrors render.go)

	// Step 1: trim leading/trailing whitespace.
	s := strings.TrimSpace(raw)

	// Step 2: optional single-layer code-fence unwrap (``` or ~~~). PREFIX check only.
	if *r.StripCodeFence {
		s = strings.TrimSpace(stripCodeFence(s))
	}

	// Step 3: output-mode switch.
	switch *r.Output {
	case "json":
		msg, fellback = parseJSON(s, *r.JsonField)
	case "raw":
		msg = s
		fellback = false // raw mode never falls back (nothing failed)
	default:
		// Validate() rejects invalid Output; if we get here the caller skipped Validate. Treat as raw
		// (the PRD default) rather than panic — robustness over strictness for an internal helper.
		msg = s
		fellback = false
	}

	// Step 4: normalize newlines (\r\n→\n, then collapse 3+ \n to 2). Literal PRD; no lone-\r handling.
	msg = normalizeNewlines(msg)

	// Step 5: final trim + ok. ok==false ⇒ orchestrator retries with retry_instruction (FR29).
	msg = strings.TrimSpace(msg)
	ok = msg != ""
	return msg, ok, fellback
}

// parseJSON implements §12.9 step 3's json branch: try json.Unmarshal on the whole string; on failure
// extract the first brace-balanced {...} substring and retry; then pull obj[field] as a string. On ANY
// failure (both Unmarshals fail, field absent/null/non-string) return (s, true) — the cleaned raw string
// with the fallback flag set. The caller (ParseOutput) then normalizes s as if it were raw.
//
// Returns (message, fellback). fellback==true ⇔ JSON extraction failed and message == the raw input s.
func parseJSON(s, field string) (string, bool) {
	// Attempt 1: whole-string Unmarshal.
	var obj map[string]any
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		// Attempt 2: brace-balanced substring (handles JSON embedded in prose / trailing commentary).
		sub, found := extractJSONObject(s)
		if !found {
			return s, true // no '{' at all, or unbalanced → fallback to raw
		}
		if err := json.Unmarshal([]byte(sub), &obj); err != nil {
			return s, true // balanced span still isn't valid JSON → fallback
		}
	}
	// Extract the configured field as a string. Absent / null / non-string ⇒ fallback (never stringify).
	v, strOK := obj[field].(string)
	if !strOK {
		return s, true // field missing, JSON null, or a non-string type (number/bool/object/array)
	}
	return v, false
}

// extractJSONObject finds the first '{' in s and scans to the matching '}' that returns brace depth to
// zero, correctly ignoring braces and quotes that appear INSIDE JSON string values (RFC 8259 §7 allows
// '{' and '}' unescaped in strings). Returns the balanced substring (inclusive of the braces) and true,
// or "" and false if there is no '{' or the braces never balance.
//
// State machine: `inString` suppresses brace counting inside "..."; `escaped` (one-byte lookahead)
// consumes the byte after a backslash inside a string so an escaped quote `\"` does NOT toggle inString.
// Byte scanning is UTF-8-safe: '{' '}' '"' '\\' are all ASCII (<0x80) and RFC 3629 §3 guarantees ASCII
// bytes never appear as UTF-8 continuation bytes — no utf8.DecodeRune needed.
func extractJSONObject(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false // consume this byte literally (it was preceded by '\' inside a string)
			continue
		}
		if inString {
			switch c {
			case '\\':
				escaped = true // next byte is escaped
			case '"':
				inString = false
			}
			continue // inside a string — braces/quotes-except-closer don't affect depth
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], true // balanced — inclusive slice
			}
		}
	}
	return "", false // ran off the end with depth > 0 — unbalanced
}

// stripCodeFence removes a single layer of markdown code fence if s STARTS with ``` or ~~~ (PRD §12.9
// step 2): it drops the entire first line (the opener plus any language tag, e.g. ```json) and everything
// from the LAST occurrence of the SAME fence token onward, then the caller re-trims. If there is no
// newline after the opener (opener-only line) the result is "". If no matching closer is found the body
// after the opener line is returned as-is (lenient). A fence token that appears mid-string (not at the
// very start) is left untouched — this is a prefix check only.
func stripCodeFence(s string) string {
	var fence string
	switch {
	case strings.HasPrefix(s, "```"):
		fence = "```"
	case strings.HasPrefix(s, "~~~"):
		fence = "~~~"
	default:
		return s // no leading fence
	}
	// Drop the first line (opener + language tag).
	nl := strings.IndexByte(s, '\n')
	if nl < 0 {
		return "" // opener only, nothing follows
	}
	body := s[nl+1:]
	// Drop everything from the LAST matching closer onward.
	if last := strings.LastIndex(body, fence); last >= 0 {
		body = body[:last]
	}
	return body
}

// normalizeNewlines implements §12.9 step 4: convert "\r\n" → "\n", then collapse runs of 3+ consecutive
// "\n" into exactly "\n\n". It does NOT touch a lone "\r" (old-Mac) — the PRD lists only "\r\n". Built
// with a manual pass (no regexp import — keeps the package's import set to encoding/json + strings).
func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var b strings.Builder
	b.Grow(len(s))
	run := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\n' {
			run++
			if run <= 2 {
				b.WriteByte(c) // keep at most two consecutive newlines
			}
			// 3rd+ consecutive newline: skip (collapse)
			continue
		}
		run = 0
		b.WriteByte(c)
	}
	return b.String()
}
```

```go
// internal/provider/parse_test.go
package provider

import (
	"strings"
	"testing"
)

// TestParseOutput is the table-driven suite for the §12.9 pipeline. Pure-function tests — no subprocess,
// no mocking. Covers every work-item scenario (raw clean / raw+fence / json valid / json-in-prose / json
// invalid→fallback / empty→ok=false / multi-newline) plus edge cases. Manifests are built with Resolve()
// applied (the documented call path) OR via literals with strPtr/boolPtr (same-package helpers).
func TestParseOutput(t *testing.T) {
	// Helper: a raw-mode manifest (strip_code_fence on/off configurable).
	rawManifest := func(strip bool) Manifest {
		return Manifest{Name: "t", Command: strPtr("t"), Output: strPtr("raw"), StripCodeFence: boolPtr(strip)}.
			Resolve()
	}
	// Helper: a json-mode manifest with a given json_field.
	jsonManifest := func(field string) Manifest {
		return Manifest{Name: "t", Command: strPtr("t"), Output: strPtr("json"), JsonField: strPtr(field),
			StripCodeFence: boolPtr(true)}.Resolve()
	}

	cases := []struct {
		name         string
		raw          string
		manifest     Manifest
		wantMsg      string
		wantOK       bool
		wantFallback bool
	}{
		// --- raw mode (FR26) ---
		{"raw clean", "fix: handle nil deref in parser", rawManifest(true), "fix: handle nil deref in parser", true, false},
		{"raw trims surrounding whitespace", "  \n fix: x \n ", rawManifest(true), "fix: x", true, false},
		{"raw with ``` fence", "```json\nfix: x\n```", rawManifest(true), "fix: x", true, false},
		{"raw with ~~~ fence", "~~~\nfix: x\n~~~", rawManifest(true), "fix: x", true, false},
		{"raw fence with language tag", "```text\nfix: x\n```", rawManifest(true), "fix: x", true, false},
		{"raw fence multi-line body", "```\nfix: x\n\nbody line 2\n```", rawManifest(true), "fix: x\n\nbody line 2", true, false},
		{"raw strip_code_fence=false preserves fence", "```fix: x```", rawManifest(false), "```fix: x```", true, false},

		// --- empty / whitespace-only (FR29 trigger: ok=false) ---
		{"empty raw → ok=false fellback=false", "", rawManifest(true), "", false, false},
		{"whitespace-only raw → ok=false", "   \n\t  \n", rawManifest(true), "", false, false},
		{"fence-only (empty body) → ok=false", "```\n```", rawManifest(true), "", false, false},

		// --- newline normalization (§12.9 step 4) ---
		{"collapse 3+ newlines to 2", "fix: x\n\n\n\nbody", rawManifest(true), "fix: x\n\nbody", true, false},
		{"crlf normalized", "fix: x\r\n\r\nbody", rawManifest(true), "fix: x\n\nbody", true, false},
		{"keep exactly 2 newlines", "fix: x\n\nbody", rawManifest(true), "fix: x\n\nbody", true, false},

		// --- json mode (FR27) ---
		{"json valid field extracted", `{"result":"fix: x"}`, jsonManifest("result"), "fix: x", true, false},
		{"json with surrounding whitespace", "\n  {\"result\":\"fix: x\"}  \n", jsonManifest("result"), "fix: x", true, false},
		{"json-in-prose (brace-balanced retry)", `Here is the message: {"result":"fix: x"} hope it helps!`,
			jsonManifest("result"), "fix: x", true, false},
		{"json nested object, field is string", `{"meta":{"k":1},"result":"fix: x"}`, jsonManifest("result"), "fix: x", true, false},
		{"json message with newlines is normalized", "{\"result\":\"fix: x\n\n\nbody\"}", jsonManifest("result"), "fix: x\n\nbody", true, false},

		// --- json fallback (fellback=true) ---
		{"json invalid → fallback to raw", "this is not json at all", jsonManifest("result"), "this is not json at all", true, true},
		{"json field missing → fallback", `{"other":"x"}`, jsonManifest("result"), `{"other":"x"}`, true, true},
		{"json field null → fallback", `{"result":null}`, jsonManifest("result"), `{"result":null}`, true, true},
		{"json field non-string (number) → fallback", `{"result":42}`, jsonManifest("result"), `{"result":42}`, true, true},
		{"json field non-string (object) → fallback", `{"result":{"a":1}}`, jsonManifest("result"), `{"result":{"a":1}}`, true, true},
		{"json empty → ok=false fellback=true", "", jsonManifest("result"), "", false, true},

		// --- brace-balanced correctness (strings containing braces/quotes) ---
		{"braces inside json string not miscounted",
			`{"result":"fix: handle { edge } case"}`, jsonManifest("result"),
			"fix: handle { edge } case", true, false},
		{"escaped quotes inside json string",
			`{"result":"She said \"hello\""}`, jsonManifest("result"),
			`She said "hello"`, true, false},
		{"escaped quotes + braces inside json string (in prose)",
			`Here: {"result":"a { b } \"c\""}`, jsonManifest("result"),
			`a { b } "c"`, true, false},
		{"json-in-prose with trailing semicolons/commentary",
			`Sure! {"result":"feat: add parser"} Let me know.`, jsonManifest("result"),
			"feat: add parser", true, false},

		// --- unresolved-manifest safety (Resolve() called internally) ---
		{"unresolved raw manifest does not panic (Output nil→raw default)",
			"fix: x", Manifest{Name: "t", Command: strPtr("t")}, "fix: x", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, ok, fb := ParseOutput(tc.raw, tc.manifest)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if fb != tc.wantFallback {
				t.Errorf("fellback = %v, want %v", fb, tc.wantFallback)
			}
			if msg != tc.wantMsg {
				t.Errorf("msg = %q, want %q", msg, tc.wantMsg)
			}
			// INVARIANT: ok == false ⇒ msg == "" (the FR29 contract).
			if !ok && msg != "" {
				t.Errorf("ok=false but msg=%q (must be empty)", msg)
			}
		})
	}
}

// TestExtractJSONObject_BalancedCorrectness targets the brace-balanced helper directly for the cases
// most likely to be miscounted by a naive depth-only counter (string state + escape state).
func TestExtractJSONObject_BalancedCorrectness(t *testing.T) {
	cases := []struct {
		in       string
		want     string
		wantFound bool
	}{
		{`{"a":1}`, `{"a":1}`, true},
		{`pre {"a":1} post`, `{"a":1}`, true},
		{`{"a":{"b":2}}`, `{"a":{"b":2}}`, true},                       // nested
		{`{"a":"}"}`, `{"a":"}"}`, true},                               // '}' inside string
		{`{"a":"{"}`, `{"a":"{"}`, true},                               // '{' inside string
		{`{"a":"\"{"}`, `{"a":"\"{"}`, true},                           // escaped quote then '{' in string
		{`no braces here`, ``, false},
		{`{ "unbalanced`, ``, false},                                   // depth never returns to 0
	}
	for _, c := range cases {
		got, found := extractJSONObject(c.in)
		if found != c.wantFound {
			t.Errorf("extractJSONObject(%q) found=%v want %v", c.in, found, c.wantFound)
			continue
		}
		if found && got != c.want {
			t.Errorf("extractJSONObject(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestStripCodeFence_FenceVariants targets the fence helper directly.
func TestStripCodeFence_FenceVariants(t *testing.T) {
	cases := []struct{ in, want string }{
		{"```json\nfix\n```", "fix"},
		{"```\nfix\n```", "fix"},
		{"~~~\nfix\n~~~", "fix"},
		{"no fence", "no fence"},
		{"```", ""},           // opener only
		{"```\nfix", "fix"},   // no closer — keep body
	}
	for _, c := range cases {
		if got := strings.TrimSpace(stripCodeFence(c.in)); got != c.want {
			t.Errorf("stripCodeFence(%q) (trimmed) = %q, want %q", c.in, got, c.want)
		}
	}
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/provider/parse.go — ParseOutput + helpers
  - PACKAGE: `package provider`; IMPORTS: EXACTLY "encoding/json", "strings" (NO regexp/fmt/third-party).
  - IMPLEMENT exported `func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)`
      per the Data Models block: Resolve() internally (NOT Validate); 5-step §12.9 order; declare
      `fellback := false` once, set true ONLY in the json fallback path.
  - IMPLEMENT unexported `parseJSON(s, field string) (string, bool)` — whole Unmarshal → brace-balanced
      retry → field `.(string)` assertion; any failure returns (s, true).
  - IMPLEMENT unexported `extractJSONObject(s string) (string, bool)` — IndexByte('{') fast-forward +
      byte scan with inString + escaped + depth; return balanced inclusive slice or ("", false).
  - IMPLEMENT unexported `stripCodeFence(s string) string` — prefix check (``` or ~~~); drop first line
      + from last closer onward; lenient no-closer.
  - IMPLEMENT unexported `normalizeNewlines(s string) string` — ReplaceAll \r\n→\n then manual collapse
      3+\n→2 (strings.Builder, run counter).
  - GOTCHA: signature is EXPORTED ParseOutput with 3 returns (NOT PRD's lowercase 2-return parseOutput).
  - GOTCHA: call Resolve() not Validate(); deref *r.Output/*r.JsonField/*r.StripCodeFence on the copy.

Task 2: CREATE internal/provider/parse_test.go — table-driven suite
  - PACKAGE: `package provider` (in-package white-box — matches manifest_test.go/executor_test.go);
      IMPORTS: "strings", "testing".
  - IMPLEMENT TestParseOutput as a []struct table (see Data Models) covering EVERY work-item scenario +
      edge cases. Use rawManifest()/jsonManifest() helpers that build a Manifest then call Resolve()
      (the documented call path), plus one unresolved-manifest case to prove Resolve() is called internally.
  - IMPLEMENT TestExtractJSONObject_BalancedCorrectness + TestStripCodeFence_FenceVariants targeting the
      helpers directly (nested braces, braces/escaped-quotes in strings, fence variants, no-closer).
  - ASSERT the invariant `!ok ⇒ msg == ""` in every row.
  - GOTCHA: no subprocess, no mustBin, no mocking — pure function. Use same-package strPtr/boolPtr helpers.

Task 3: VERIFY (no further file change)
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum MUST be byte-unchanged. Every frozen file
      (manifest/merge/builtin/registry/render/executor/procgroup_* + tests, config/*, git/*) MUST be
      byte-unchanged. `go test -race ./internal/provider/` MUST be green (incl. the existing executor
      suite — ParseOutput adds no regressions).
```

### Implementation Patterns & Key Details

```go
// THE internal Resolve() call — nil-pointer-safe deref on a copy (mirrors render.go). Do NOT Validate().
r := m.Resolve()
s := strings.TrimSpace(raw)
if *r.StripCodeFence {
	s = strings.TrimSpace(stripCodeFence(s))
}
switch *r.Output {
case "json":
	msg, fellback = parseJSON(s, *r.JsonField) // fellback set here only
case "raw":
	msg = s
	fellback = false
}
msg = normalizeNewlines(msg)
msg = strings.TrimSpace(msg)
ok = msg != ""

// THE json branch — two Unmarshal attempts, then a string type-assertion; any failure ⇒ (s, true).
var obj map[string]any
if err := json.Unmarshal([]byte(s), &obj); err != nil {
	sub, found := extractJSONObject(s)
	if !found { return s, true }
	if err := json.Unmarshal([]byte(sub), &obj); err != nil { return s, true }
}
v, strOK := obj[field].(string)   // comma-ok — absent/null/non-string all land here
if !strOK { return s, true }
return v, false

// THE brace-balanced extractor — inString + escaped (the two flags a naive counter omits). ASCII-safe.
depth, inString, escaped := 0, false, false
for i := start; i < len(s); i++ {
	c := s[i]
	if escaped { escaped = false; continue }
	if inString {
		switch c { case '\\': escaped = true; case '"': inString = false }
		continue
	}
	switch c {
	case '"': inString = true
	case '{': depth++
	case '}': depth--; if depth == 0 { return s[start:i+1], true }
	}
}
return "", false

// THE fence strip — opener token IS the closer token; PREFIX check; lenient no-closer.
// THE normalization — ReplaceAll \r\n→\n THEN manual collapse 3+\n→2; NO lone-\r; NO regexp.
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - change: NONE. ParseOutput uses stdlib encoding/json + strings ONLY. `go mod tidy` MUST be a no-op.
        `git diff --exit-code go.mod go.sum` MUST be empty.

PACKAGE EDGES (import graph):
  - parse.go → (stdlib: encoding/json, strings) ONLY. It does NOT import internal/config, internal/git,
        os/exec, context, fmt, or regexp. It is in package provider (same as manifest.go) so the Manifest
        type + Resolve() + strPtr/boolPtr need no import.
  - parse_test.go → (stdlib: strings, testing) ONLY. In-package (package provider) — uses strPtr/boolPtr
        + Default* constants directly.

UPSTREAM CONTRACT (the input — do NOT implement, just consume):
  - P1.M2.T1.S1 Manifest: ParseOutput takes `m Manifest` and reads *m.Output / *m.JsonField /
        *m.StripCodeFence via Resolve(). Treat manifest.go as FROZEN — do NOT add methods or fields there.
  - P1.M2.T5.S1 Execute: returns `(stdout, stderr, err)`. ParseOutput consumes ONLY the stdout string.
        stderr + err are the orchestrator's concern (retry/rescue from err; ParseOutput never sees them).

DOWNSTREAM CONTRACTS (the output — do NOT implement here, just honor the triple's meaning):
  - P1.M3.T4 (orchestrator CommitStaged): `msg, ok, fellback := provider.ParseOutput(stdout, m)`.
        `if !ok` ⇒ retry once with m.RetryInstruction (FR29); else proceed. Log fellback in verbose mode.
  - P1.M3.T2 (dedupe): keys on `msg` (subject extraction). fellback is irrelevant to dedupe.
  - P1.M4.T3.S2 (verbose UI): logs "provider %s: json parse failed, used raw output" when fellback==true.
  => The `(msg string, ok bool, fellback bool)` signature is FROZEN after T6. Do not change it.

FROZEN FILES (do NOT edit):
  - internal/provider/manifest.go (+_test.go), merge.go, builtin.go, registry.go, render.go,
        executor.go, procgroup_unix.go, procgroup_windows*.go, executor_test.go, render_test.go,
        registry_test.go, builtin_test.go, merge_test.go, manifest_test.go.
  - internal/config/*, internal/git/*, cmd/stagecoach/main.go, Makefile, go.mod, go.sum.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format the new files
gofmt -w internal/provider/parse.go internal/provider/parse_test.go

# Vet the provider package (compiles parse.go + parse_test.go)
go vet ./internal/provider/

# Confirm the import set is exactly stdlib (no regexp, no third-party)
head -20 internal/provider/parse.go   # → import ( "encoding/json" "strings" )

# go.mod/go.sum MUST be unchanged (stdlib only)
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum unchanged ✓"

# Expected: Zero errors. If `go vet` flags an unused import, remove it. If it reports a nil-deref or a
# type error, re-check the Resolve() call (design-decisions §2) and the comma-ok assertion (§3).
```

### Level 2: Unit Tests (THE KEYSTONE — table-driven suite)

```bash
# Run the new suite verbosely (every subtest listed)
go test -race -v ./internal/provider/ -run TestParseOutput
go test -race -v ./internal/provider/ -run 'TestExtractJSONObject|TestStripCodeFence'

# Full provider package (the new suite + the existing executor/manifest/render/registry suites)
go test -race ./internal/provider/

# Coverage of the new file specifically
go test -coverprofile=coverage.out ./internal/provider/ && go tool cover -func=coverage.out | grep parse.go

# Expected: All pass. If a row fails, READ the (msg, ok, fellback) triple it printed vs the want triple —
# the most common cause is a missed flag (escaped in extractJSONObject) or normalizing before extraction.
# Coverage target (PRD §20.3): ≥85% on parse.go; the table is designed to hit every branch.
```

### Level 3: Whole-Module Integration (No Regressions)

```bash
# Build everything (parse.go compiles into the provider package)
go build ./...

# Full test suite with the race detector (the existing executor suite MUST stay green)
go test -race ./...

# Lint (if golangci-lint is installed; the Makefile `lint` target)
golangci-lint run ./internal/provider/ 2>/dev/null || echo "(golangci-lint not installed; skipped)"

# Confirm frozen files are byte-unchanged
git diff --exit-code internal/provider/manifest.go internal/provider/merge.go internal/provider/builtin.go \
  internal/provider/registry.go internal/provider/render.go internal/provider/executor.go \
  internal/provider/procgroup_unix.go internal/provider/executor_test.go && echo "frozen files unchanged ✓"

# Expected: build succeeds; all tests green; go.mod/go.sum + every frozen file byte-unchanged.
```

### Level 4: Contract & Correctness Reasoning (No Subprocess Needed)

```bash
# ParseOutput is a pure function — no server, no DB, no subprocess. The "integration" is the truth-table
# contract. Verify by reasoning + the table:
#
#   1. ok==false ⇒ msg==""  (the FR29 retry trigger; asserted in every table row).
#   2. fellback true ONLY in json mode on JSON failure; always false in raw mode (asserted per row).
#   3. Order: fence-strip (step 2) runs on `s` BEFORE the output switch (step 3); normalize (step 4)
#      runs on `msg` AFTER; final trim (step 5) last. (Covered by "json message with newlines is
#      normalized" + "raw with fence" rows.)
#   4. Brace-balanced correctness: braces/escaped-quotes inside JSON strings do NOT miscount (covered by
#      the dedicated TestExtractJSONObject_BalancedCorrectness rows — the cases a naive counter gets wrong).
#   5. Resolve() safety: an unresolved manifest (Output=nil) does not panic (the "unresolved raw manifest"
#      row).
#
# (No Level-4 commands to run beyond Levels 1–3 — there is no runtime to start. The table IS the proof.)
```

## Final Validation Checklist

### Technical Validation

- [ ] All validation levels completed; `go build ./...` succeeds; `go test -race ./internal/provider/` green.
- [ ] `go vet ./internal/provider/` clean; `gofmt -l internal/provider/` empty.
- [ ] `go mod tidy` is a no-op; `git diff --exit-code go.mod go.sum` empty (stdlib json+strings only).
- [ ] `go test -race ./...` green (the existing executor/manifest/render/registry suites — no regressions).
- [ ] Every frozen file byte-unchanged (`git diff --exit-code` on manifest/merge/builtin/registry/render/
      executor/procgroup_* + their `_test.go`, config/*, git/*).

### Feature Validation

- [ ] `func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)` — EXPORTED, 3 returns
      (NOT PRD's lowercase 2-return `parseOutput`).
- [ ] The 5-step §12.9 order is exact: TrimSpace → stripCodeFence(if) → output switch → normalizeNewlines
      → TrimSpace + ok.
- [ ] json branch: whole Unmarshal → brace-balanced retry → `obj[field].(string)`; ANY failure ⇒ (s, true).
- [ ] `extractJSONObject` tracks inString + escaped (not just depth); re-validates via json.Unmarshal.
- [ ] `fellback` true ONLY in json mode on failure; always false in raw mode; `ok==false ⇒ msg==""`.
- [ ] `stripCodeFence`: prefix check, opener⇒closer token, drop first line + last-closer-onward, lenient.
- [ ] `normalizeNewlines`: `\r\n`→`\n` then collapse 3+`\n`→2; no lone-`\r`; no regexp.
- [ ] ParseOutput calls Resolve() internally (not Validate); nil Output does not panic.

### Code Quality Validation

- [ ] Follows the package's established patterns: in-package white-box tests, table-driven, grouped doc-
      comments, `t.Helper()` where helpers assert (here helpers build fixtures, so optional), stdlib-only.
- [ ] File placement matches the desired codebase tree (parse.go + parse_test.go in internal/provider/).
- [ ] Anti-patterns avoided: no PRD-lowercase signature; no Validate() call; no non-string stringification;
      no depth-only brace counter; no regexp; no lone-\r handling; no edit to any frozen file.
- [ ] Doc comments cite PRD §12.9 + the design calls (signature, Resolve, fellback truth table, brace
      state machine, UTF-8 ASCII-safety).

### Documentation & Deployment

- [ ] Doc comment on `ParseOutput` cites PRD §12.9 (the 5-step pipeline), §9.6 (FR26–FR29), the return
      contract, and the why-exported + why-Resolve-not-Validate rationales.
- [ ] No new environment variables, config, CLI surface, or user-facing docs (PRD "DOCS: none — internal
      parser").

---

## Anti-Patterns to Avoid

- ❌ Don't implement PRD §12.9's lowercase 2-return `parseOutput` — implement the work-item's EXPORTED
      3-return `ParseOutput(raw, m) (msg, ok, fellback)`. The consumer is cross-package (internal/generate).
      (research design-decisions §0)
- ❌ Don't dereference `*m.Output` without Resolve() — the fields are pointers and panic on nil. Call
      `r := m.Resolve()` first (mirrors render.go). Don't call Validate() (parsing needs only the three
      output fields). (research design-decisions §2)
- ❌ Don't stringify a non-string json_field (`fmt.Sprintf("%v", v)`) — a number/bool/object field is a
      schema mismatch; fall back to raw + fellback=true instead. (research go-json-map-types.md)
- ❌ Don't write a depth-only brace counter for extractJSONObject — braces/quotes appear inside JSON
      strings (RFC 8259 §7). Track `inString` AND `escaped` (escaped handles `\"`). (research
      json-in-prose-brace-balanced.md)
- ❌ Don't use `range s` (runes) for the brace scan and then index back into `s` — use a byte loop
      `for i := 0; i < len(s); i++ { c := s[i] }`; ASCII bytes are UTF-8-safe. (research §4)
- ❌ Don't skip re-running `json.Unmarshal` on the balanced substring — the extraction only finds a
      syntactically-plausible span; Unmarshal is the validator.
- ❌ Don't normalize newlines BEFORE the output switch, or trim before fence-strip — order is load-bearing
      (step 2 before 3; step 4 after 3; step 5 last).
- ❌ Don't handle lone `\r` or import `regexp` — follow the PRD literally (only `\r\n`); keep imports at
      `encoding/json`+`strings`. (research design-decisions §6)
- ❌ Don't set `fellback=true` in raw mode, or leave `fellback` unset (declare once as false; set only in
      the json fallback path). (research design-decisions §1)
- ❌ Don't edit manifest.go (or any frozen file) to add a Parse method — ParseOutput is a free function
      taking `m Manifest`; the Manifest type stays as P1.M2.T1.S1 froze it.
- ❌ Don't write `package provider_test` (external test) — every _test.go in internal/provider/ is in-
      package white-box; mirror that to reach strPtr/boolPtr + Default* constants.

---

## Confidence Score

**9/10** for one-pass implementation success. The pipeline is fully specified by PRD §12.9 (essentially
pseudocode) and pinned by the work item's exact contract; the four non-obvious design calls (exported
3-return signature, internal Resolve()-not-Validate(), non-string⇒fallback, inString+escaped brace
state machine) are each backed by an authoritative source (Go encoding/json docs, RFC 8259 §7, RFC 3629
§3) with copy-ready implementation sketches in the research briefs; the test table enumerates every
work-item scenario plus the edge cases a naive implementation gets wrong (braces/escaped-quotes in
strings, non-string fields, no-closer fences, unresolved manifests); validation is pure stdlib `go
test`/`go vet`/`gofmt`/`go build` (all executable on the dev box — no cross-compile or CI needed,
unlike T5.S2). The one residual risk — an overlooked normalization-order or fence-edge interaction — is
caught by the dedicated table rows (`json message with newlines is normalized`, `raw fence multi-line
body`, `crlf normalized`) and the `!ok ⇒ msg==""` invariant asserted in every row.
