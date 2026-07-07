---
name: "P1.M1.T1.S2 — Populate pi ReasoningLevels with verified --thinking tokens"
description: |
  Bugfix Issue 1 (pi half). `builtinPi()` ships `ReasoningLevels: nil`, so the FR-R6 reasoning feature
  (the shipped `planner=high` default, `--reasoning high`, every `--<role>-reasoning` flag) is inert for
  pi. Populate it with the VERIFIED `pi --help` tokens: `--thinking <level>` (low|medium|high; pi also
  accepts minimal/xhigh but stagecoach's level set is off|low|medium|high — map only the overlap). The
  Render guard (render.go:124-126) is ALREADY correct — only the manifest DATA is missing. CRITICAL: the
  data change ripples to TWO `reflect.DeepEqual` parity fixtures (the `piTOML` const + the shipped
  `providers/pi.toml`) that MUST also carry the matching `[reasoning_levels]` table or the provider test
  suite fails. pi ONLY (claude = S1). One doc line (docs/providers.md) — COORDINATE with S1 on the same
  row. +1 render test + extend PiFields.
---

## Goal

**Feature Goal**: Make the FR-R6 reasoning feature functional for the `pi` provider by populating its
`ReasoningLevels` manifest table with the verified `pi --help` `--thinking <level>` tokens, so a resolved
reasoning level of `high`/`medium`/`low` emits `--thinking <level>` at `Render` (and `off`/`""` remain a
silent no-op, FR-R6). This unfetters the shipped `planner=high` default and the documented
`--reasoning high` for any role backed by pi.

**Deliverable** (data + parity fixtures + tests + one doc line):
1. `internal/provider/builtin.go` `builtinPi()`: add `ReasoningLevels: map[string][]string{"high":{"--thinking","high"},"medium":{"--thinking","medium"},"low":{"--thinking","low"}}` after the `BareFlags` block (before the TooledFlags comment); remove the TODO(FR-D5) comment (lines 52-55); update the function doc comment.
2. `internal/provider/builtin_test.go` `piTOML` const: append the matching `[reasoning_levels]` table (parity with builtinPi).
3. `providers/pi.toml` (shipped reference file): append the matching `[reasoning_levels]` table (parity with builtinPi).
4. `internal/provider/builtin_test.go` `TestBuiltinManifests_PiFields`: add a `ReasoningLevels["high"]` non-empty + no-`off`-key assertion.
5. `internal/provider/render_test.go`: add `TestRender_PiReasoningThinkingTokens` using the REAL `builtinPi()`, asserting `--thinking <level>` for high/medium/low and no-op for off/`""`.
6. `docs/providers.md` (~line 35): note pi now populates high/medium/low via `--thinking` (COORDINATE with S1's claude `--effort` note on the same row).

**Success Definition**: `builtinPi().Render("zai/glm-5.2", "", "", "high")` produces a CmdSpec whose Args
contain the consecutive tokens `--thinking` then `high` (after the FR-R5b fold emits `--provider zai
--model glm-5.2`); `medium`/`low` likewise; `off`/`""` append no `--thinking` token and never error.
`builtinPi().ReasoningLevels["high"]` is non-empty; `["off"]` is absent. Both `reflect.DeepEqual` parity
tests (DecodeParity + ReferenceFiles_DecodeParity) stay green. `go build/vet/gofmt` clean and
`go test -race ./...` green.

## User Persona

**Target User**: The Stagecoach user who runs the planner (or any role) on `pi` and expects the documented
`--reasoning high` (or the shipped `planner=high` default) to actually engage deeper reasoning — and the
contributor wiring real per-role reasoning values (P1.M2).

**Use Case**: `stagecoach --reasoning high` (or a decompose run whose planner resolves to pi with
`planner=high`) should invoke `pi --provider <backend> --model <m> --thinking high …`.

**Pain Points Addressed**: Today the reasoning feature is completely inert for pi — `ReasoningLevels` is
nil, so Render's reasoning branch is a no-op; the advertised `--reasoning high` and the `planner=high`
default do nothing. The fix makes pi honor them.

## Why

- **Ships the FR-R6 P0 feature for pi.** The reasoning feature (§9.15 FR-R6) is non-functional
  out-of-the-box because NO provider populates `ReasoningLevels`. This subtask lands the pi half
  (claude is the S1 sibling, running in parallel).
- **Render's guard already works.** `render.go:124-126` (`if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0`)
  is correct; only the manifest data is absent. So this is a minimal data change, not a logic change.
- **Verified, not guessed.** external_deps.md §pi confirms `pi --help` exposes `--thinking <level>`
  (off|minimal|low|medium|high|xhigh). The contract's map (high/medium/low → `--thinking <level>`) is
  exactly the verified overlap with stagecoach's off|low|medium|high level set.
- **Honest no-op elsewhere.** Providers without a verified reasoning control (gemini/agy/qwen-code/
  opencode/codex/cursor) stay nil — the FR-R6 graceful no-op then applies honestly rather than
  advertising an inert feature.

## What

A data-only manifest change for `pi` (no logic), propagated to its two `reflect.DeepEqual` parity
fixtures, plus a focused render test and one doc line. `off` deliberately has no map entry (stagecoach's
`off` means "no reasoning control"; the level resolves to a no-op) → stays a no-op.

### Success Criteria

- [ ] `builtinPi()` has `ReasoningLevels` with keys `high`/`medium`/`low` → `["--thinking", <level>]`.
- [ ] `builtinPi()` has NO `off` key (off ⇒ no-op).
- [ ] `builtinPi().Render("zai/glm-5.2","","","high")` Args contain `--thinking` then `high` (consecutive; after the FR-R5b `--provider zai --model glm-5.2` fold).
- [ ] `pi.Render("zai/glm-5.2","","","medium"|"low")` likewise emits `--thinking <level>`.
- [ ] `pi.Render("zai/glm-5.2","","","off")` and `("zai/glm-5.2","","","")` append NO `--thinking` token, no error.
- [ ] `piTOML` const + `providers/pi.toml` each carry the identical `[reasoning_levels]` table.
- [ ] Both `TestBuiltinManifests_DecodeParity` and `TestProviderReferenceFiles_DecodeParity` stay green.
- [ ] `TestBuiltinManifests_PiFields` asserts `ReasoningLevels["high"]` non-empty + no `off` key.
- [ ] `docs/providers.md` reasoning_levels row notes pi `--thinking` high/medium/low (and claude `--effort`, via S1 coordination).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP states the verified tokens (with the external_deps.md source), the exact
insertion point in `builtinPi()`, the THREE parity surfaces (and why all three must change), the complete
render-test (with the traced Args), the existing test helpers to reuse (`containsPair`/`containsToken`/
`assertStr`/`assertNilStr`), the exact TOML block, and the docs/providers.md coordination with S1. The
stale-`default_provider`-line gotcha (harmless, ignored) is called out so the implementer doesn't derail.

### Documentation & References

```yaml
# MUST READ — the verified-token source (authoritative; do NOT guess)
- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/architecture/external_deps.md
  why: "§pi (lines 6-21) is the authoritative token source: `pi --help` exposes `--thinking <level>` (off|minimal|low|medium|high|xhigh). Gives the exact map: high/medium/low → [\"--thinking\", <level>]; off → no tokens. Confirms the overlap with stagecoach's off|low|medium|high set (minimal/xhigh not mapped)."
  critical: "§claude (lines 27-35) is S1 (claude --effort), NOT this subtask. S2 is pi ONLY. §'Providers with NO known reasoning control' lists the 6 that stay nil (graceful no-op)."

- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/P1M1T1S1/PRP.md
  why: "The claude SIBLING — the exact structural template (same 5 files, same 3 parity surfaces, same 2 reflect.DeepEqual tests, same test pattern). S2 mirrors it for pi/--thinking. Read it to align structure + to coordinate the shared docs/providers.md row."
  critical: "S1 and S2 BOTH edit docs/providers.md line 35 (the reasoning_levels row) — S1 for claude --effort, S2 for pi --thinking. Write the FINAL-STATE cell mentioning BOTH so the row is correct regardless of land-order. S1 and S2 edit DIFFERENT builtin constructors (builtinPi vs builtinClaude) and DIFFERENT consts/tests — no conflict there."

- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/architecture/issue_findings.md
  why: "Issue 1 root cause: every built-in ships ReasoningLevels=nil → Render's reasoning branch is a no-op → planner=high / --reasoning high are inert. Confirms the Render guard is correct and only the data is missing."

# The production file under edit
- file: internal/provider/builtin.go
  why: "EDIT. builtinPi() (line 42): remove the TODO(FR-D5) comment (lines 52-55); insert ReasoningLevels after the BareFlags block (before the TooledFlags comment, ~line 63); update the function doc comment (lines 37-40)."
  pattern: "Map literal `map[string][]string{\"high\": {\"--thinking\", \"high\"}, \"medium\": ..., \"low\": ...}`. NO `off` key (off ⇒ no-op). Same-package map literals need no helper."
  gotcha: "Update the function doc comment (lines 37-40) 'NOTE: ReasoningLevels is nil ... FR-D5 requires verification' to state pi now populates high/medium/low via --thinking (verified). Placement after BareFlags is per the contract; struct/map comparison is order-independent so it's cosmetic."

# The parity fixtures (MUST change or DeepEqual tests break)
- file: internal/provider/builtin_test.go
  why: "EDIT (fixture + test). piTOML const (line 16) MUST append the [reasoning_levels] table — TestBuiltinManifests_DecodeParity (line 366, entry {\"pi\", builtinPi(), piTOML} at line 372) does reflect.DeepEqual(builtinPi(), decoded piTOML). TestBuiltinManifests_PiFields (line 242) EXTEND with a ReasoningLevels assertion."
  pattern: "DecodeParity is reflect.DeepEqual — builtin and decoded-TOML must match EXACTLY (map order-independent; slice element-wise). Append the table at the END of piTOML (after strip_code_fence = true; bare_flags/tooled_flags are array VALUES not tables, so all top-level keys precede the first [table])."
  gotcha: "piTOML currently contains a STALE `default_provider = \"\"` line (the DefaultProvider FIELD was removed in plan 003; go-toml v2 ignores unknown keys → harmless, baseline GREEN). Do NOT touch it — only ADD [reasoning_levels] at the end. Forgetting piTOML → TestBuiltinManifests_DecodeParity FAILS on DeepEqual. #1 one-pass failure mode."

- file: providers/pi.toml
  why: "EDIT (shipped reference file). TestProviderReferenceFiles_DecodeParity (referencefiles_test.go:39; entry {\"pi\",\"providers/pi.toml\"} at :19) does reflect.DeepEqual(decoded providers/pi.toml, builtinPi()). MUST append the identical [reasoning_levels] table (with a comment block in the file's style)."
  gotcha: "Comments are stripped on decode; only the data must match builtinPi(). Append at the END (the file's trailing 'absent fields' comment block lists what is NOT set; reasoning_levels now IS set — append the table after it, optionally note the comment)."

- file: internal/provider/render_test.go
  why: "EDIT (new test). Has containsPair(args, flag, val) (line 437) + containsToken(args, token) (line 447) helpers, same package (package provider) — can call builtinPi() directly. The existing synthetic-manifest reasoning test is the placement pattern; S2 uses the REAL built-in per the contract."

# Cross-references (read-only — do NOT edit in S2)
- file: internal/provider/render.go
  why: "Render's reasoning guard (lines 124-126) is ALREADY CORRECT — no edit. Confirms tokens append AFTER the model flag. Render(model, sysPrompt, userPayload, reasoning, mode...) — pi ProviderFlag=\"--provider\" so \"zai/glm-5.2\" FOLDS to --provider zai --model glm-5.2 (FR-R5b), THEN --thinking high appends."
- file: internal/provider/manifest.go
  why: "ReasoningLevels field (line 89) + Resolve (line 180, left as-is) + Validate (no constraint). Already exist and correct — NO edit. DefaultProvider field is GONE (so the stale default_provider lines in the TOML are ignored)."
- file: docs/providers.md
  why: "EDIT (Mode A doc, ~line 35). The reasoning_levels row's description cell should note pi populates high/medium/low via --thinking (AND claude via --effort — S1 coordination). DEFAULT column stays 'nil (none)' (schema default)."

- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/P1M1T1S2/research/s2_implementation_notes.md
  why: "Distilled S2 findings: verified --thinking tokens, the three parity surfaces, the exact builtin edit, the matching TOML block, the complete render test (with traced Args), the stale-default_provider gotcha, and the pi-only scope boundary + docs coordination with S1."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── internal/provider/
│   ├── builtin.go            # EDIT (builtinPi +ReasoningLevels; remove TODO; doc comment)
│   ├── builtin_test.go       # EDIT (piTOML const +table; PiFields +assertion)
│   ├── render.go             # read-only — reasoning guard ALREADY correct
│   ├── render_test.go        # EDIT (+ TestRender_PiReasoningThinkingTokens)
│   ├── manifest.go           # read-only — ReasoningLevels field/Resolve already exist
│   └── referencefiles_test.go # read-only ref — enforces providers/*.toml parity
├── providers/pi.toml         # EDIT (+ [reasoning_levels] table; parity-tested)
└── docs/providers.md         # EDIT (reasoning_levels row description, ~line 35; coordinate w/ S1)
```

### Desired Codebase Tree After S2

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/provider/builtin.go          # builtinPi +ReasoningLevels +remove TODO +doc fix
    internal/provider/builtin_test.go     # piTOML +table; PiFields +assertion
    internal/provider/render_test.go      # +TestRender_PiReasoningThinkingTokens
    providers/pi.toml                     # +[reasoning_levels] table
    docs/providers.md                     # reasoning_levels row description (pi --thinking + claude --effort)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/provider/builtin.go` | MODIFY | `builtinPi()` +ReasoningLevels (verified `--thinking`); remove TODO; doc comment. |
| `internal/provider/builtin_test.go` | MODIFY | `piTOML` const +`[reasoning_levels]` (parity); `PiFields` +assertion. |
| `internal/provider/render_test.go` | MODIFY | +real-built-in render test (`--thinking` emit + off/`""` no-op). |
| `providers/pi.toml` | MODIFY | +`[reasoning_levels]` table (parity with builtin). |
| `docs/providers.md` | MODIFY | reasoning_levels row notes pi `--thinking` high/medium/low (+ claude `--effort` via S1). |

**Explicitly NOT touched**: `render.go`/`manifest.go`/`merge.go` (logic already correct), `builtinClaude()`
(claude `--effort` = S1 / P1.M1.T1.S1), the other 6 builtins, the stale `default_provider = ""` lines in
piTOML/providers/pi.toml (harmless, ignored), Issue 2 (message-role routing = P1.M2), Issue 3
(index-sync = P1.M3), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL — THREE parity surfaces, TWO reflect.DeepEqual tests. Adding ReasoningLevels to builtinPi()
// breaks TestBuiltinManifests_DecodeParity (builtin_test.go:366) AND TestProviderReferenceFiles_DecodeParity
// (referencefiles_test.go:39) UNLESS the piTOML const AND providers/pi.toml ALSO carry the identical
// [reasoning_levels] table. Forgetting EITHER fixture is the #1 one-pass failure mode.

// CRITICAL — use the VERIFIED flag. pi --help exposes `--thinking <level>` (off|minimal|low|medium|high|xhigh).
// Map ONLY the overlap with stagecoach's off|low|medium|high set: high/medium/low → ["--thinking", <level>].
// minimal/xhigh have no stagecoach level — NOT mapped. Source: external_deps.md §pi.

// CRITICAL — NO `off` key in the map. stagecoach's `off` means "no reasoning control" and is the natural
// zero no-op (absent key → nil slice → len 0 → Render appends nothing). Do NOT add "off": {"--thinking", "off"}.

// CRITICAL (docs coordination) — S1 (claude) edits the SAME docs/providers.md line 35 row for claude --effort.
// Write the FINAL-STATE cell mentioning BOTH pi (--thinking) and claude (--effort) so the row is correct
// regardless of which lands first. Do NOT overwrite S1's claude clause with pi-only text.

// GOTCHA — piTOML (and providers/pi.toml) contain a STALE `default_provider = ""` line. The DefaultProvider
// FIELD was removed in plan 003; go-toml v2 ignores unknown keys, so it is harmless (baseline is GREEN).
// Do NOT touch it — leave it exactly as-is; only ADD [reasoning_levels] at the end.

// GOTCHA — TOML key order: bare_flags/tooled_flags are ARRAY VALUES (top-level keys), NOT tables. All
// top-level keys already precede any [table]. Append [reasoning_levels] at the END (after strip_code_fence = true).

// GOTCHA — go-toml decodes [reasoning_levels] high = ["--thinking", "high"] into map[string][]string.
// reflect.DeepEqual against the builtin literal passes (map comparison is order-independent; slice element-wise).
// The data must match; comments are stripped on decode so they don't affect parity.

// GOTCHA — the render test uses the REAL builtinPi() (per contract). pi has ProviderFlag="--provider" so
// "zai/glm-5.2" FOLDS (FR-R5b) to --provider zai --model glm-5.2, THEN --thinking <level> appends after the
// model flag. Traced Args for Render("zai/glm-5.2","","","high"):
//   ["--provider","zai","--model","glm-5.2","--thinking","high","--no-tools","--no-extensions","--no-skills",
//    "--no-prompt-templates","--no-context-files","--no-session","-p"]. containsPair(args,"--thinking","high")→true.
```

## Implementation Blueprint

### Data models and structure

No schema/logic change — the `ReasoningLevels map[string][]string` field and the Render guard already
exist. This is a data population. The relevant existing types/helpers (unchanged):

```go
// internal/provider/manifest.go (EXISTING — unchanged)
ReasoningLevels map[string][]string `toml:"reasoning_levels"`

// internal/provider/render.go (EXISTING — unchanged; the guard that CONSUMES the data)
if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0 {
    args = append(args, r.ReasoningLevels[reasoning]...)   // after the model flag
}

// internal/provider/render_test.go (EXISTING helpers — reuse)
func containsPair(args []string, flag, val string) bool   // line 437
func containsToken(args []string, token string) bool      // line 447
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: builtin.go — add ReasoningLevels to builtinPi()
  - LOCATE builtinPi() (line 42). Find the BareFlags block close `},` (~line 63), immediately followed
    by the `// TOOLED MODE (v2 §11.5 …)` comment block.
  - INSERT between BareFlags close and the TooledFlags comment:
        // REASONING LEVELS (v3; §12.1, FR-R6). pi exposes `--thinking off|minimal|low|medium|high|xhigh`
        // (verified `pi --help`, external_deps.md §pi). off ⇒ no entry (natural zero no-op); stagecoach's
        // level set is off|low|medium|high, so minimal/xhigh are not mapped. Tokens append after the model flag.
        ReasoningLevels: map[string][]string{
            "high":   {"--thinking", "high"},
            "medium": {"--thinking", "medium"},
            "low":    {"--thinking", "low"},
        },
  - REMOVE the TODO comment (lines 52-55):
        // ReasoningLevels: nil — TODO(FR-D5): populate reasoning_levels tokens once verified
        // (e.g. claude --thinking-effort low|medium|high; verify per provider's --help/docs).
        // nil is safe: FR-R6 makes it a graceful no-op (call sites pass reasoning="" in S1).
  - UPDATE the function doc comment (lines 37-40): replace the "NOTE: ReasoningLevels is nil (absent) in
    the shipped default. FR-D5 requires verification before populating — …" block with:
    "NOTE: ReasoningLevels is populated — pi `--thinking` high/medium/low (verified `pi --help`,
    external_deps.md §pi); off ⇒ no-op (no entry). minimal/xhigh have no stagecoach level."
  - DO NOT: touch builtinClaude (S1) or any other builtin; touch render.go/manifest.go/merge.go.

Task 2: builtin_test.go — piTOML const + PiFields assertion (PARITY)
  - LOCATE the piTOML const (line 16). It ends with `output = "raw"\nstrip_code_fence = true\n`.
  - APPEND (before the closing backtick), a blank line then:
        [reasoning_levels]
        high = ["--thinking", "high"]
        medium = ["--thinking", "medium"]
        low = ["--thinking", "low"]
  - EXTEND TestBuiltinManifests_PiFields (line 242) — after the existing field assertions (e.g. after the
    TooledFlags DeepEqual or the StripCodeFence check), add:
        // ReasoningLevels: high/medium/low populated (verified pi --thinking); off absent (no-op)
        if m.ReasoningLevels == nil || len(m.ReasoningLevels["high"]) == 0 {
            t.Errorf("ReasoningLevels missing 'high' entry: %v", m.ReasoningLevels)
        }
        if _, ok := m.ReasoningLevels["off"]; ok {
            t.Errorf("ReasoningLevels should NOT have an 'off' entry (off ⇒ no-op)")
        }
  - WHY: TestBuiltinManifests_DecodeParity (line 366, entry line 372) does reflect.DeepEqual(builtinPi(),
    decoded piTOML) — the const MUST match or it fails.
  - DO NOT touch the stale `default_provider = ""` line in piTOML (harmless, ignored; out of scope).

Task 3: providers/pi.toml — matching [reasoning_levels] table (PARITY)
  - APPEND at the END of the file (after the trailing 'absent fields' comment block), a comment + table:
        # --- reasoning levels (v3; §12.1, FR-R6) ---
        # pi exposes `--thinking off|minimal|low|medium|high|xhigh` (verified `pi --help`, external_deps.md §pi).
        # off has no entry ⇒ graceful no-op (FR-R6); minimal/xhigh have no stagecoach level. Tokens append
        # after the model flag at render.
        [reasoning_levels]
        high = ["--thinking", "high"]
        medium = ["--thinking", "medium"]
        low = ["--thinking", "low"]
  - WHY: TestProviderReferenceFiles_DecodeParity (referencefiles_test.go:39) does
    reflect.DeepEqual(decoded providers/pi.toml, builtinPi()). Comments are stripped on decode; only the
    data must match.
  - DO NOT touch the stale `default_provider` line if present (harmless, ignored).

Task 4: render_test.go — real-built-in render test
  - ADD (package provider; reuses containsPair line 437 + containsToken line 447):
        func TestRender_PiReasoningThinkingTokens(t *testing.T) {
            m := builtinPi() // the REAL built-in (not synthetic)
            // high/medium/low → --thinking <level> appended after the model flag (FR-R5b fold first)
            for _, lvl := range []string{"high", "medium", "low"} {
                s, err := m.Render("zai/glm-5.2", "", "", lvl) // folds to --provider zai --model glm-5.2
                if err != nil {
                    t.Fatalf("%s: %v", lvl, err)
                }
                if !containsPair(s.Args, "--thinking", lvl) {
                    t.Errorf("pi %s: want --thinking %s in %v", lvl, lvl, s.Args)
                }
            }
            // off / "" → no --thinking token, never an error (FR-R6 no-op)
            for _, lvl := range []string{"off", ""} {
                s, err := m.Render("zai/glm-5.2", "", "", lvl)
                if err != nil {
                    t.Fatalf("%q: %v", lvl, err)
                }
                if containsToken(s.Args, "--thinking") {
                    t.Errorf("pi %q: want NO --thinking token in %v", lvl, s.Args)
                }
            }
        }
  - PLACEMENT: near the existing reasoning tests (e.g. TestRender_ReasoningTokensAppended).
  - DO NOT: modify the existing synthetic-manifest reasoning tests (they stay valid); do NOT duplicate S1's
    claude test.

Task 5: docs/providers.md — reasoning_levels row (~line 35) — COORDINATE with S1
  - LOCATE line 35: "| `reasoning_levels` | table | nil (none) | Per-level reasoning-effort token lists
    (off/low/medium/high); nil/empty ⇒ graceful no-op (FR-R6). Appended after the model flag at render. |"
  - WRITE the FINAL-STATE description cell covering BOTH pi and claude (so the row is correct whether S1
    or S2 lands first):
        "... Appended after the model flag at render. pi populates high/medium/low via `--thinking`
        (verified `pi --help`); claude via `--effort` (verified `claude --help`); all other built-ins
        are nil (graceful no-op)."
  - If S1's claude clause is already present, MERGE (append the pi clause); if absent, write the full
    combined text. KEEP the DEFAULT column "nil (none)" (schema default). Leave line 59's general sentence alone.

Task 6: VALIDATE
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: gofmt -l .            # must be empty
  - RUN: go test -race ./...   # DecodeParity + ReferenceFiles_DecodeParity + new test all green
  - FIX-FORWARD: if a DeepEqual parity test fails, the fixture (piTOML or providers/pi.toml) doesn't
    match builtinPi() — reconcile the [reasoning_levels] table byte-for-byte (data, not comments).
```

### Implementation Patterns & Key Details

```go
// === builtin.go — the ReasoningLevels literal in builtinPi() (after BareFlags, before TooledFlags) ===
		BareFlags: []string{
			"--no-tools", "--no-extensions", "--no-skills",
			"--no-prompt-templates", "--no-context-files", "--no-session",
		},
		// REASONING LEVELS (v3; §12.1, FR-R6). pi exposes `--thinking off|minimal|low|medium|high|xhigh`
		// (verified `pi --help`, external_deps.md §pi). off ⇒ no entry (natural zero no-op); stagecoach's
		// level set is off|low|medium|high, so minimal/xhigh are not mapped. Tokens append after the model flag.
		ReasoningLevels: map[string][]string{
			"high":   {"--thinking", "high"},
			"medium": {"--thinking", "medium"},
			"low":    {"--thinking", "low"},
		},
		// TOOLED MODE (v2 §11.5 — the stager role). ...   // (existing comment, unchanged)
		TooledFlags: []string{ ... },
```

```toml
# === the [reasoning_levels] table appended to piTOML (builtin_test.go) AND providers/pi.toml ===
[reasoning_levels]
high = ["--thinking", "high"]
medium = ["--thinking", "medium"]
low = ["--thinking", "low"]
```

```go
// === traced Args for pi.Render("zai/glm-5.2", "", "", "high") — ProviderFlag="--provider" so FR-R5b FOLDS ===
//   ["--provider", "zai", "--model", "glm-5.2", "--thinking", "high",
//    "--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session", "-p"]
// containsPair(args, "--thinking", "high") → true. For "off"/"" → no "--thinking" token anywhere.
```

### Integration Points

```yaml
MANIFEST DATA (internal/provider/builtin.go builtinPi):
  - added: 'ReasoningLevels: map[string][]string{"high":{"--thinking","high"},"medium":...,"low":...}'
  - no "off" key (off ⇒ no-op); minimal/xhigh not mapped (no stagecoach level)

PARITY FIXTURES (must match builtinPi EXACTLY — reflect.DeepEqual):
  - internal/provider/builtin_test.go : piTOML const + [reasoning_levels] table
  - providers/pi.toml                 : + [reasoning_levels] table (commented, file style)

TESTS:
  - builtin_test.go TestBuiltinManifests_PiFields : +ReasoningLevels["high"] assertion + no-"off" check
  - render_test.go TestRender_PiReasoningThinkingTokens : real-built-in --thinking emit + off/"" no-op

DOC (Mode A): docs/providers.md reasoning_levels row description (pi --thinking + claude --effort via S1 coordination)

NO-TOUCH (explicitly):
  - internal/provider/render.go     # reasoning guard (lines 124-126) ALREADY correct
  - internal/provider/manifest.go   # ReasoningLevels field/Resolve already exist; DefaultProvider already gone
  - internal/provider/merge.go      # ReasoningLevels merge already exists
  - internal/provider/builtin.go builtinClaude + other 6 builtins   # claude=S1; others stay nil
  - stale `default_provider = ""` lines in piTOML/providers/pi.toml  # harmless (field gone; ignored on decode)
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S2):
  - S1 (P1.M1.T1.S1): populate claude ReasoningLevels with verified --effort tokens (parallel; shares docs/providers.md row)
  - P1.M2: resolve the message role on the single-commit path (Issue 2) — where pi reasoning reaches the message role
  - P1.M6: README/cli.md doc sweep scoping --reasoning to providers that honor it
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -l .                       # Expected: empty (run gofmt -w on any listed file)
go vet ./internal/provider/...   # Expected: exit 0
go build ./...                   # Expected: exit 0
```

### Level 2: Unit Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

# The new real-built-in render test
go test -race -run 'TestRender_PiReasoningThinkingTokens' ./internal/provider/ -v

# The parity tests (the #1 risk surface — must stay green) + the field test
go test -race -run 'TestBuiltinManifests_DecodeParity|TestProviderReferenceFiles_DecodeParity|TestBuiltinManifests_PiFields' ./internal/provider/ -v

# Full provider suite
go test -race ./internal/provider/ -v

# Expected: new test PASS (--thinking high/medium/low emitted after the FR-R5b fold; off/"" no-op);
# BOTH parity tests PASS (piTOML + providers/pi.toml match builtinPi); PiFields PASS.
```

### Level 3: Whole-Repository Regression

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages pass
go vet ./...                     # Expected: exit 0

# Confirm ONLY the 5 intended files changed (S2's contribution; S1's claude files may also appear)
git diff --stat -- internal/ providers/ docs/
# Expected (S2): internal/provider/{builtin,builtin_test,render_test}.go + providers/pi.toml + docs/providers.md
```

### Level 4: Behavior Smoke (the contract's probe — optional)

```bash
cd /home/dustin/projects/stagecoach

# Inline probe via a throwaway in-package test (delete after) — proves the data + guard end-to-end:
cat > internal/provider/zz_smoke_test.go <<'EOF'
package provider
import "testing"
func TestZZ_PiThinkingSmoke(t *testing.T) {
	m := builtinPi()
	s, _ := m.Render("zai/glm-5.2", "", "", "high")
	t.Logf("pi high args: %v", s.Args)   // expect --thinking high present (after --provider zai --model glm-5.2)
}
EOF
go test -run TestZZ_PiThinkingSmoke -v ./internal/provider/ ; rm -f internal/provider/zz_smoke_test.go
# (The permanent TestRender_PiReasoningThinkingTokens in Task 4 supersedes this — no need to keep it.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages pass (new test + both parity tests green).

### Feature Validation

- [ ] `builtinPi().ReasoningLevels` has `high`/`medium`/`low` → `["--thinking", <level>]`; no `off` key.
- [ ] `pi.Render("zai/glm-5.2","","","high")` emits `--provider zai --model glm-5.2 --thinking high`.
- [ ] `pi.Render("zai/glm-5.2","","","medium"|"low")` likewise emits `--thinking <level>`.
- [ ] `pi.Render("zai/glm-5.2","","","off")` and `("zai/glm-5.2","","","")` append no `--thinking`, no error.
- [ ] `piTOML` const + `providers/pi.toml` carry the identical `[reasoning_levels]` table.
- [ ] `TestBuiltinManifests_DecodeParity` + `TestProviderReferenceFiles_DecodeParity` green.
- [ ] `TestBuiltinManifests_PiFields` asserts `ReasoningLevels["high"]` non-empty + no `off` key.
- [ ] `docs/providers.md` reasoning_levels row notes pi `--thinking` (+ claude `--effort` via S1).

### Scope Discipline Validation

- [ ] ONLY the 5 intended files modified by S2 (git diff --stat confirms; S1's claude files may also appear).
- [ ] Did NOT edit `render.go`/`manifest.go`/`merge.go` (logic already correct).
- [ ] Did NOT touch `builtinClaude` or the other 6 builtins (claude = S1; others stay nil).
- [ ] Did NOT touch the stale `default_provider = ""` lines in piTOML/providers/pi.toml (harmless, ignored).
- [ ] Did NOT address Issue 2 (message-role = P1.M2) or Issue 3 (index-sync = P1.M3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Verified `--thinking` tokens used (per external_deps.md §pi); minimal/xhigh not mapped.
- [ ] `off` deliberately absent (graceful no-op, not `--thinking off`).
- [ ] Parity fixtures match builtinPi() byte-for-byte (data).
- [ ] Test uses the REAL `builtinPi()` (per contract), reusing `containsPair`/`containsToken`.
- [ ] docs/providers.md row mentions BOTH pi and claude (S1 coordination).

---

## Anti-Patterns to Avoid

- ❌ Don't map `minimal`/`xhigh` — they have no stagecoach level (off|low|medium|high). Map ONLY
  high/medium/low → `["--thinking", <level>]`.
- ❌ Don't forget the parity fixtures. Adding `ReasoningLevels` to `builtinPi()` breaks
  `TestBuiltinManifests_DecodeParity` (piTOML const) AND `TestProviderReferenceFiles_DecodeParity`
  (providers/pi.toml) unless BOTH carry the matching `[reasoning_levels]` table. #1 trap.
- ❌ Don't add an `"off"` key. stagecoach's `off` is the natural zero no-op; `off` must be absent (→ nil
  slice → len 0 → Render appends nothing).
- ❌ Don't put the `[reasoning_levels]` table before the top-level keys in the TOML — `bare_flags`/
  `tooled_flags` are array VALUES (top-level keys), and once a `[table]` header appears later keys belong
  to it. Append `[reasoning_levels]` at the END (after `strip_code_fence = true`).
- ❌ Don't touch the stale `default_provider = ""` line in piTOML/providers/pi.toml — the field was
  removed in plan 003; go-toml v2 ignores unknown keys (baseline is GREEN). Leave it as-is.
- ❌ Don't edit `render.go`/`manifest.go`/`merge.go` — the reasoning guard, field, Resolve behavior, and
  merge all already exist and are correct. Only the manifest DATA is missing.
- ❌ Don't populate claude here (claude uses `--effort` — that's S1 / P1.M1.T1.S1, running in parallel).
- ❌ Don't leave the other 6 providers' `ReasoningLevels` anything but nil — they have no verified
  reasoning control (external_deps.md §"Providers with NO known reasoning control"); the FR-R6 graceful
  no-op applies honestly.
- ❌ Don't change the `default` column of the docs/providers.md reasoning_levels row — it's the schema
  default ("nil (none)"), not pi-specific. Only update the DESCRIPTION cell.
- ❌ Don't write a pi-only docs/providers.md row that overwrites S1's claude clause — write the FINAL-STATE
  cell mentioning BOTH providers (pi --thinking + claude --effort) so the row is correct regardless of
  land-order. (S1 and S2 edit the SAME line.)
- ❌ Don't use a synthetic manifest in the new test — the contract requires the REAL `builtinPi()` (the
  synthetic-manifest case is already covered by TestRender_ReasoningTokensAppended).

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a data-only manifest change with the verified tokens sourced from external_deps.md
(`--thinking`, exactly the high/medium/low overlap with stagecoach's level set), the Render guard already
correct, and the complete test + TOML blocks specified verbatim. It is the exact structural mirror of S1
(claude), which validates the approach. The one non-obvious trap — the THREE parity surfaces (builtin.go +
piTOML const + providers/pi.toml) enforced by TWO `reflect.DeepEqual` tests — is called out as the #1
failure mode with the exact mitigation (append the matching table to both fixtures). The stale
`default_provider = ""` line (harmless, ignored) is flagged so the implementer doesn't derail. The
render-test Args are traced concretely (pi ProviderFlag="--provider" → FR-R5b fold → `--provider zai
--model glm-5.2 --thinking high`). The residual uncertainty (not 10/10) is purely mechanical: ensuring
the two TOML fixtures match the builtin map byte-for-byte (data) and the docs/providers.md row converging
with S1's parallel claude edit — both caught/mitigated by the deterministic `go test -race` parity gates
and the explicit "write the combined final-state row" instruction. claude (S1), Issue 2 (P1.M2), and
Issue 3 (P1.M3) are cleanly fenced and untouched.
