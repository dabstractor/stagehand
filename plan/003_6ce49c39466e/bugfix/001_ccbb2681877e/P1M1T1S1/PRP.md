---
name: "P1.M1.T1.S1 — Populate claude ReasoningLevels with verified --effort tokens"
description: |
  Bugfix Issue 1 (claude half). `builtinClaude()` ships `ReasoningLevels: nil`, so the FR-R6 reasoning
  feature (the shipped `planner=high` default, `--reasoning high`, every `--<role>-reasoning` flag) is
  inert for claude. Populate it with the VERIFIED `claude --help` tokens: `--effort <level>`
  (low|medium|high) — NOT the `--thinking-effort` the PRD Suggested Fix guessed (external_deps.md §claude).
  The Render guard (render.go:126) is already correct — only the manifest DATA is missing. CRITICAL: the
  data change ripples to TWO `reflect.DeepEqual` parity fixtures (the `claudeTOML` const + the shipped
  `providers/claude.toml`) that MUST also carry the matching `[reasoning_levels]` table or the provider
  test suite fails. claude ONLY (pi = S2). One doc line (docs/providers.md). +1 render test + extend
  ClaudeFields.
---

## Goal

**Feature Goal**: Make the FR-R6 reasoning feature functional for the `claude` provider by populating
its `ReasoningLevels` manifest table with the verified `claude --help` `--effort <level>` tokens, so a
resolved reasoning level of `high`/`medium`/`low` emits `--effort <level>` at `Render` (and `off`/`""`
remain a silent no-op, FR-R6). This unfetters the shipped `planner=high` default and the documented
`--reasoning high` for any role backed by claude.

**Deliverable** (data + parity fixtures + tests + one doc line):
1. `internal/provider/builtin.go` `builtinClaude()`: add `ReasoningLevels: map[string][]string{"high":{"--effort","high"},"medium":{"--effort","medium"},"low":{"--effort","low"}}` between `TooledFlags` and `Output`; update its doc comment + the trailing nil-list comment.
2. `internal/provider/builtin_test.go` `claudeTOML` const: append the matching `[reasoning_levels]` table (parity with builtinClaude).
3. `providers/claude.toml` (shipped reference file): append the matching `[reasoning_levels]` table (parity with builtinClaude).
4. `internal/provider/builtin_test.go` `TestBuiltinManifests_ClaudeFields`: add a `ReasoningLevels["high"]` non-empty assertion.
5. `internal/provider/render_test.go`: add `TestRender_ClaudeReasoningEffortTokens` using the REAL `builtinClaude()`, asserting `--effort <level>` for high/medium/low and no-op for off/`""`.
6. `docs/providers.md` (~line 35): note claude populates high/medium/low via `--effort`.

**Success Definition**: `claude.Render("sonnet", "", "", "high")` produces a CmdSpec whose Args contain
the consecutive tokens `--effort` then `high`; `medium`/`low` likewise; `off`/`""` append no `--effort`
token and never error. `builtinClaude().ReasoningLevels["high"]` is non-empty. Both `reflect.DeepEqual`
parity tests (DecodeParity + ReferenceFiles_DecodeParity) stay green. `go build/vet/gofmt` clean and
`go test -race ./...` green.

## User Persona

**Target User**: The Stagecoach user who runs the planner (or any role) on `claude` and expects the
documented `--reasoning high` (or the shipped `planner=high` default) to actually engage deeper
reasoning — and the contributor wiring real per-role reasoning values (P1.M2).

**Use Case**: `stagecoach --provider claude --reasoning high` (or a decompose run whose planner resolves
to claude with `planner=high`) should invoke `claude … --effort high …`.

**Pain Points Addressed**: Today the reasoning feature is completely inert for claude — `ReasoningLevels`
is nil, so Render's reasoning branch is a no-op; the advertised `--reasoning high` and the `planner=high`
default do nothing. The fix makes claude honor them.

## Why

- **Ships the FR-R6 P0 feature for claude.** The reasoning feature (§9.15 FR-R6) is non-functional
  out-of-the-box because NO provider populates `ReasoningLevels`. Task P1.M1.T1.S1 (the original keystone)
  was marked Complete without populating it. This subtask lands the claude half.
- **Render's guard already works.** `render.go:126` (`if reasoning != "" && len(r.ReasoningLevels[reasoning]) > 0`)
  is correct; only the manifest data is absent. So this is a minimal data change, not a logic change.
- **Verified, not guessed.** external_deps.md §claude confirms `claude --help` exposes `--effort
  low|medium|high` — NOT the `--thinking-effort` the PRD Suggested Fix guessed. S1 uses the verified flag.
- **Honest no-op elsewhere.** Providers without a verified reasoning control (gemini/agy/qwen-code/
  opencode/codex/cursor) stay nil — the FR-R6 graceful no-op then applies honestly rather than
  advertising an inert feature.

## What

A data-only manifest change for `claude` (no logic), propagated to its two `reflect.DeepEqual` parity
fixtures, plus a focused render test and one doc line. `off` deliberately has no map entry (claude's
`--effort` has no "off" value) → stays a no-op.

### Success Criteria

- [ ] `builtinClaude()` has `ReasoningLevels` with keys `high`/`medium`/`low` → `["--effort", <level>]`.
- [ ] `builtinClaude()` has NO `off` key (off ⇒ no-op).
- [ ] `claude.Render("sonnet","","","high")` Args contain `--effort` then `high` (consecutive).
- [ ] `claude.Render("sonnet","","","medium"|"low")` likewise emits `--effort <level>`.
- [ ] `claude.Render("sonnet","","","off")` and `("sonnet","","","")` append NO `--effort` token, no error.
- [ ] `claudeTOML` const + `providers/claude.toml` each carry the identical `[reasoning_levels]` table.
- [ ] Both `TestBuiltinManifests_DecodeParity` and `TestProviderReferenceFiles_DecodeParity` stay green.
- [ ] `TestBuiltinManifests_ClaudeFields` asserts `ReasoningLevels["high"]` non-empty.
- [ ] `docs/providers.md` reasoning_levels row notes claude populates high/medium/low via `--effort`.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP states the verified tokens (with the external_deps.md source), the exact
insertion point in `builtinClaude()`, the THREE parity surfaces (and why all three must change), the
complete render-test (with the traced Args), the existing test helpers to reuse (`containsPair`/
`containsToken`/`assertStr`), and the exact TOML block. The contract + external_deps.md pre-resolved the
`--effort` (not `--thinking-effort`) correction.

### Documentation & References

```yaml
# MUST READ — the verified-token source (do not use the PRD's --thinking-effort guess)
- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/architecture/external_deps.md
  why: "§claude (lines 27-35) is the authoritative token source: `claude --help` exposes `--effort <level>` (low|medium|high). Gives the exact map: high/medium/low → [\"--effort\", <level>]. Confirms the PRD Suggested Fix's `--thinking-effort` is WRONG."
  critical: "§pi (lines 13-21) shows pi uses `--thinking` — that is S2 (P1.M1.T1.S2), NOT this subtask. S1 is claude ONLY. §38-42 lists the providers that stay nil (graceful no-op)."

- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/architecture/issue_findings.md
  why: "Issue 1 root cause: every built-in ships ReasoningLevels=nil → Render's reasoning branch is a no-op → planner=high / --reasoning high are inert. Confirms the Render guard is correct and only the data is missing."

# The production file under edit
- file: internal/provider/builtin.go
  why: "EDIT. builtinClaude() (~line 89-136): insert ReasoningLevels between the TooledFlags block and Output (~line 134); update the doc comment (line 101) + the trailing nil-list comment (line 135)."
  pattern: "Map literal `map[string][]string{\"high\": {\"--effort\", \"high\"}, ...}`. NO `off` key (off ⇒ no-op). strPtr/boolPtr helpers are same-package; map literals need no helper."
  gotcha: "Update BOTH the doc comment '(2) ReasoningLevels is nil ...' (line 101) AND the trailing 'Subcommand, PromptFlag, ..., ReasoningLevels: nil' comment (line 135) — remove ReasoningLevels from the nil list."

# The parity fixtures (MUST change or DeepEqual tests break)
- file: internal/provider/builtin_test.go
  why: "EDIT (fixture + test). claudeTOML const (line 45) MUST append the [reasoning_levels] table — TestBuiltinManifests_DecodeParity (line 366) does reflect.DeepEqual(builtinClaude(), decoded claudeTOML). TestBuiltinManifests_ClaudeFields (line 295) EXTEND with a ReasoningLevels assertion."
  pattern: "DecodeParity is reflect.DeepEqual — builtin and decoded-TOML must match EXACTLY (map order-independent; slice element-wise). Append the table at the END of claudeTOML (after strip_code_fence = true; top-level keys must precede any [table])."
  gotcha: "Forgetting claudeTOML → TestBuiltinManifests_DecodeParity FAILS on DeepEqual. This is the #1 one-pass failure mode."

- file: providers/claude.toml
  why: "EDIT (shipped reference file). TestProviderReferenceFiles_DecodeParity (referencefiles_test.go:39) does reflect.DeepEqual(decoded providers/claude.toml, builtinClaude()). MUST append the identical [reasoning_levels] table (with a comment block in the file's style)."
  gotcha: "The contract's DOCS note mentions only docs/providers.md line 35, but providers/claude.toml is PARITY-TESTED (not just docs) — it MUST get the table too or the test fails. Comments are stripped on decode; only the data must match builtinClaude()."

- file: internal/provider/render_test.go
  why: "EDIT (new test). Has containsPair(args, flag, val) (line 437) + containsToken(args, token) (line 447) helpers, same package (package provider) — can call builtinClaude() directly. TestRender_ReasoningTokensAppended (line 387) is the pattern but uses a SYNTHETIC manifest; S1 uses the REAL built-in per the contract."

# Cross-references (read-only — do NOT edit in S1)
- file: internal/provider/render.go
  why: "Render's reasoning guard (lines 124-127) is ALREADY CORRECT — no edit. Confirms tokens append AFTER the model flag. Render(model, sysPrompt, userPayload, reasoning, mode...) — claude ProviderFlag=\"\" so 'sonnet' passes verbatim (no FR-R5b split)."
- file: internal/provider/manifest.go
  why: "ReasoningLevels field (line 89) + Resolve (line 180, left as-is) + Validate (no constraint). Already exist and correct — NO edit."
- file: docs/providers.md
  why: "EDIT (Mode A doc, ~line 35). The reasoning_levels row's description cell should note claude populates high/medium/low via --effort. DEFAULT column stays 'nil (none)' (schema default)."

- docfile: plan/003_6ce49c39466e/bugfix/001_ccbb2681877e/P1M1T1S1/research/s1_implementation_notes.md
  why: "Distilled S1 findings: verified tokens, the three parity surfaces, the exact builtin edit, the matching TOML block, the complete render test (with traced Args), and the claude-only scope boundary."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
├── internal/provider/
│   ├── builtin.go            # EDIT (builtinClaude +ReasoningLevels; comments)
│   ├── builtin_test.go       # EDIT (claudeTOML const +table; ClaudeFields +assertion)
│   ├── render.go             # read-only — reasoning guard ALREADY correct
│   ├── render_test.go        # EDIT (+ TestRender_ClaudeReasoningEffortTokens)
│   ├── manifest.go           # read-only — field/Resolve/Validate already exist
│   └── referencefiles_test.go # read-only ref — enforces providers/*.toml parity
├── providers/claude.toml     # EDIT (+ [reasoning_levels] table; parity-tested)
└── docs/providers.md         # EDIT (reasoning_levels row description, ~line 35)
```

### Desired Codebase Tree After S1

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/provider/builtin.go          # builtinClaude +ReasoningLevels +comment fixes
    internal/provider/builtin_test.go     # claudeTOML +table; ClaudeFields +assertion
    internal/provider/render_test.go      # +TestRender_ClaudeReasoningEffortTokens
    providers/claude.toml                 # +[reasoning_levels] table
    docs/providers.md                     # reasoning_levels row description
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/provider/builtin.go` | MODIFY | `builtinClaude()` +ReasoningLevels (verified `--effort`); doc + nil-list comments. |
| `internal/provider/builtin_test.go` | MODIFY | `claudeTOML` const +`[reasoning_levels]` (parity); `ClaudeFields` +assertion. |
| `internal/provider/render_test.go` | MODIFY | +real-built-in render test (`--effort` emit + off/`""` no-op). |
| `providers/claude.toml` | MODIFY | +`[reasoning_levels]` table (parity with builtin). |
| `docs/providers.md` | MODIFY | reasoning_levels row notes claude `--effort` high/medium/low. |

**Explicitly NOT touched**: `render.go`/`manifest.go`/`merge.go` (logic already correct), `builtinPi()`
(pi `--thinking` = S2 / P1.M1.T1.S2), the other 6 builtins, Issue 2 (message-role routing = P1.M2),
Issue 3 (index-sync = P1.M3), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL — THREE parity surfaces, TWO reflect.DeepEqual tests. Adding ReasoningLevels to builtinClaude()
// breaks TestBuiltinManifests_DecodeParity (builtin_test.go:366) AND TestProviderReferenceFiles_DecodeParity
// (referencefiles_test.go:39) UNLESS the claudeTOML const AND providers/claude.toml ALSO carry the identical
// [reasoning_levels] table. Forgetting EITHER fixture is the #1 one-pass failure mode.

// CRITICAL — use the VERIFIED flag. claude --help exposes `--effort <level>` (low|medium|high), NOT
// --thinking-effort (the PRD Suggested Fix was wrong). Source: external_deps.md §claude.

// CRITICAL — NO `off` key in the map. claude's --effort has no "off" value; off must remain a graceful
// no-op (absent key → nil slice → len 0 → Render appends nothing). Do NOT add "off": {"--effort", "off"}.

// GOTCHA — TOML key order: all top-level keys (name…strip_code_fence) MUST precede the first [table] header.
// Append [reasoning_levels] at the END of claudeTOML and providers/claude.toml (after strip_code_fence = true).

// GOTCHA — go-toml decodes [reasoning_levels] high = ["--effort", "high"] into map[string][]string. reflect.DeepEqual
// against the builtin literal passes (map comparison is order-independent; slice is element-wise). The data must
// match; comments are stripped on decode so they don't affect parity.

// GOTCHA — the render test uses the REAL builtinClaude() (per contract), NOT a synthetic manifest. claude has
// ProviderFlag="" so Render does NOT split "sonnet" (no FR-R5b fold). Traced Args for Render("sonnet","","","high"):
// ["--model","sonnet","--effort","high","--tools","","--setting-sources","","--no-session-persistence","-p"].
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
Task 1: builtin.go — add ReasoningLevels to builtinClaude()
  - LOCATE builtinClaude() (~line 89). Find the TooledFlags block close `},` immediately followed by
    `Output: strPtr("raw"),`.
  - INSERT between them:
        // REASONING LEVELS (v3; §12.1, FR-R6). claude exposes `--effort low|medium|high` (verified vs
        // `claude --help`, external_deps.md §claude — NOT --thinking-effort). off has no entry ⇒ no-op.
        ReasoningLevels: map[string][]string{
            "high":   {"--effort", "high"},
            "medium": {"--effort", "medium"},
            "low":    {"--effort", "low"},
        },
  - UPDATE the function doc comment (line ~101): replace "(2) ReasoningLevels is nil — §12.4 OMITS the
    key entirely." with "(2) ReasoningLevels is populated — claude `--effort` (verified, external_deps.md
    §claude); off ⇒ no-op.".
  - UPDATE the trailing nil-list comment (line ~135): remove `ReasoningLevels` from
    "Subcommand, PromptFlag, JsonField, RetryInstruction, Env, ReasoningLevels: nil (absent in §12.4).".
  - DO NOT: touch builtinPi (S2) or any other builtin; touch render.go/manifest.go/merge.go.

Task 2: builtin_test.go — claudeTOML const + ClaudeFields assertion (PARITY)
  - LOCATE the claudeTOML const (line 45). It ends with `output = "raw"\nstrip_code_fence = true\n`.
  - APPEND (before the closing backtick), a blank line then:
        [reasoning_levels]
        high = ["--effort", "high"]
        medium = ["--effort", "medium"]
        low = ["--effort", "low"]
  - EXTEND TestBuiltinManifests_ClaudeFields (line 295) — after the existing field assertions, add:
        if m.ReasoningLevels == nil || len(m.ReasoningLevels["high"]) == 0 {
            t.Errorf("ReasoningLevels missing 'high' entry: %v", m.ReasoningLevels)
        }
        if _, ok := m.ReasoningLevels["off"]; ok {
            t.Errorf("ReasoningLevels should NOT have an 'off' entry (off ⇒ no-op)")
        }
  - WHY: TestBuiltinManifests_DecodeParity (line 366) does reflect.DeepEqual(builtinClaude(), decoded
    claudeTOML) — the const MUST match or it fails.

Task 3: providers/claude.toml — matching [reasoning_levels] table (PARITY)
  - APPEND at the END of the file (after `strip_code_fence = true`), a comment block + the table:
        # --- reasoning levels (v3; §12.1, FR-R6) ---
        # claude exposes `--effort low|medium|high` (verified vs `claude --help`, external_deps.md §claude).
        # off has no entry ⇒ graceful no-op (FR-R6). Tokens append after the model flag at render.
        [reasoning_levels]
        high = ["--effort", "high"]
        medium = ["--effort", "medium"]
        low = ["--effort", "low"]
  - WHY: TestProviderReferenceFiles_DecodeParity (referencefiles_test.go:39) does
    reflect.DeepEqual(decoded providers/claude.toml, builtinClaude()). Comments are stripped on decode;
    only the data must match.

Task 4: render_test.go — real-built-in render test
  - ADD (package provider; reuses containsPair line 437 + containsToken line 447):
        func TestRender_ClaudeReasoningEffortTokens(t *testing.T) {
            m := builtinClaude() // the REAL built-in (not synthetic)
            // high/medium/low → --effort <level> appended after the model flag
            for _, lvl := range []string{"high", "medium", "low"} {
                s, err := m.Render("sonnet", "", "", lvl)
                if err != nil { t.Fatalf("%s: %v", lvl, err) }
                if !containsPair(s.Args, "--effort", lvl) {
                    t.Errorf("claude %s: want --effort %s in %v", lvl, lvl, s.Args)
                }
            }
            // off / "" → no --effort token, never an error (FR-R6 no-op)
            for _, lvl := range []string{"off", ""} {
                s, err := m.Render("sonnet", "", "", lvl)
                if err != nil { t.Fatalf("%q: %v", lvl, err) }
                if containsToken(s.Args, "--effort") {
                    t.Errorf("claude %q: want NO --effort token in %v", lvl, s.Args)
                }
            }
        }
  - PLACEMENT: near TestRender_ReasoningTokensAppended (line 387).
  - DO NOT: modify the existing synthetic-manifest reasoning tests (they stay valid).

Task 5: docs/providers.md — reasoning_levels row (~line 35)
  - LOCATE the row: "| `reasoning_levels` | table | nil (none) | Per-level reasoning-effort token lists
    (off/low/medium/high); nil/empty ⇒ graceful no-op (FR-R6). Appended after the model flag at render. |"
  - UPDATE the DESCRIPTION cell to note claude's population, e.g. append:
    "claude populates high/medium/low via `--effort` (verified `claude --help`); all other built-ins are
    nil (graceful no-op)."
  - KEEP the DEFAULT column "nil (none)" (it's the schema default, not claude-specific).

Task 6: VALIDATE
  - RUN: go build ./...
  - RUN: go vet ./...
  - RUN: gofmt -l .            # must be empty
  - RUN: go test -race ./...   # DecodeParity + ReferenceFiles_DecodeParity + new test all green
  - FIX-FORWARD: if a DeepEqual parity test fails, the fixture (claudeTOML or providers/claude.toml)
    doesn't match builtinClaude() — reconcile the [reasoning_levels] table byte-for-byte (data, not comments).
```

### Implementation Patterns & Key Details

```go
// === builtin.go — the ReasoningLevels literal in builtinClaude() (after TooledFlags, before Output) ===
		TooledFlags: []string{
			"--allowed-tools", "Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit",
			"--setting-sources", "",
			"--no-session-persistence",
		},
		// REASONING LEVELS (v3; §12.1, FR-R6). claude exposes `--effort low|medium|high` (verified vs
		// `claude --help`, external_deps.md §claude — NOT --thinking-effort). off has no entry ⇒ no-op.
		ReasoningLevels: map[string][]string{
			"high":   {"--effort", "high"},
			"medium": {"--effort", "medium"},
			"low":    {"--effort", "low"},
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
```

```toml
# === the [reasoning_levels] table appended to claudeTOML (builtin_test.go) AND providers/claude.toml ===
[reasoning_levels]
high = ["--effort", "high"]
medium = ["--effort", "medium"]
low = ["--effort", "low"]
```

```go
// === traced Args for claude.Render("sonnet", "", "", "high") — ProviderFlag="" so NO FR-R5b split ===
//   ["--model", "sonnet", "--effort", "high", "--tools", "", "--setting-sources", "", "--no-session-persistence", "-p"]
// containsPair(args, "--effort", "high") → true. For "off"/"" → no "--effort" token anywhere.
```

### Integration Points

```yaml
MANIFEST DATA (internal/provider/builtin.go builtinClaude):
  - added: 'ReasoningLevels: map[string][]string{"high":{"--effort","high"},"medium":...,"low":...}'
  - no "off" key (off ⇒ no-op)

PARITY FIXTURES (must match builtinClaude EXACTLY — reflect.DeepEqual):
  - internal/provider/builtin_test.go : claudeTOML const + [reasoning_levels] table
  - providers/claude.toml             : + [reasoning_levels] table (commented, file style)

TESTS:
  - builtin_test.go TestBuiltinManifests_ClaudeFields : +ReasoningLevels["high"] assertion
  - render_test.go TestRender_ClaudeReasoningEffortTokens : real-built-in --effort emit + off/"" no-op

DOC (Mode A): docs/providers.md reasoning_levels row description (claude populates high/medium/low via --effort)

NO-TOUCH (explicitly):
  - internal/provider/render.go     # reasoning guard (lines 124-127) ALREADY correct
  - internal/provider/manifest.go   # field/Resolve/Validate already exist
  - internal/provider/merge.go      # ReasoningLevels merge already exists
  - internal/provider/builtin.go builtinPi + other 6 builtins   # pi=S2; others stay nil
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S1):
  - S2 (P1.M1.T1.S2): populate pi ReasoningLevels with verified --thinking tokens (external_deps.md §pi)
  - P1.M2: resolve the message role on the single-commit path (Issue 2) — where claude reasoning reaches the message role
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
go test -race -run 'TestRender_ClaudeReasoningEffortTokens' ./internal/provider/ -v

# The parity tests (the #1 risk surface — must stay green)
go test -race -run 'TestBuiltinManifests_DecodeParity|TestProviderReferenceFiles_DecodeParity|TestBuiltinManifests_ClaudeFields' ./internal/provider/ -v

# Full provider suite
go test -race ./internal/provider/ -v

# Expected: new test PASS (--effort high/medium/low emitted; off/"" no-op); BOTH parity tests PASS
# (claudeTOML + providers/claude.toml match builtinClaude); ClaudeFields PASS.
```

### Level 3: Whole-Repository Regression

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages pass
go vet ./...                     # Expected: exit 0

# Confirm ONLY the 5 intended files changed
git diff --stat -- internal/ providers/ docs/
# Expected: internal/provider/{builtin,builtin_test,render_test}.go + providers/claude.toml + docs/providers.md
```

### Level 4: Behavior Smoke (the contract's probe — optional)

```bash
cd /home/dustin/projects/stagecoach

# Inline probe via a throwaway in-package test (delete after) — proves the data + guard end-to-end:
cat > internal/provider/zz_smoke_test.go <<'EOF'
package provider
import "testing"
func TestZZ_ClaudeEffortSmoke(t *testing.T) {
	m := builtinClaude()
	s, _ := m.Render("sonnet", "", "", "high")
	t.Logf("claude high args: %v", s.Args)   // expect --effort high present
}
EOF
go test -run TestZZ_ClaudeEffortSmoke -v ./internal/provider/ ; rm -f internal/provider/zz_smoke_test.go
# (The permanent TestRender_ClaudeReasoningEffortTokens in Task 4 supersedes this — no need to keep it.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages pass (new test + both parity tests green).

### Feature Validation

- [ ] `builtinClaude().ReasoningLevels` has `high`/`medium`/`low` → `["--effort", <level>]`; no `off` key.
- [ ] `claude.Render("sonnet","","","high")` emits `--effort high` (and medium/low likewise).
- [ ] `claude.Render("sonnet","","","off")` and `("sonnet","","","")` append no `--effort`, no error.
- [ ] `claudeTOML` const + `providers/claude.toml` carry the identical `[reasoning_levels]` table.
- [ ] `TestBuiltinManifests_DecodeParity` + `TestProviderReferenceFiles_DecodeParity` green.
- [ ] `TestBuiltinManifests_ClaudeFields` asserts `ReasoningLevels["high"]` non-empty.
- [ ] `docs/providers.md` reasoning_levels row notes claude `--effort` high/medium/low.

### Scope Discipline Validation

- [ ] ONLY the 5 intended files modified (git diff --stat confirms).
- [ ] Did NOT edit `render.go`/`manifest.go`/`merge.go` (logic already correct).
- [ ] Did NOT touch `builtinPi` or the other 6 builtins (pi = S2; others stay nil).
- [ ] Did NOT address Issue 2 (message-role = P1.M2) or Issue 3 (index-sync = P1.M3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Verified `--effort` tokens used (not the PRD's `--thinking-effort` guess).
- [ ] `off` deliberately absent (graceful no-op, not `--effort off`).
- [ ] Parity fixtures match builtinClaude() byte-for-byte (data).
- [ ] Test uses the REAL `builtinClaude()` (per contract), reusing `containsPair`/`containsToken`.

---

## Anti-Patterns to Avoid

- ❌ Don't use `--thinking-effort` — the PRD Suggested Fix guessed wrong. `claude --help` exposes
  `--effort <level>` (external_deps.md §claude is authoritative).
- ❌ Don't forget the parity fixtures. Adding `ReasoningLevels` to `builtinClaude()` breaks
  `TestBuiltinManifests_DecodeParity` (claudeTOML const) AND `TestProviderReferenceFiles_DecodeParity`
  (providers/claude.toml) unless BOTH carry the matching `[reasoning_levels]` table. This is the #1 trap.
- ❌ Don't add an `"off"` key. claude's `--effort` has no "off" value; `off` must be a no-op (absent key).
- ❌ Don't put the `[reasoning_levels]` table before the top-level keys in the TOML — once a `[table]`
  header appears, subsequent keys belong to it. Append at the END (after `strip_code_fence = true`).
- ❌ Don't edit `render.go`/`manifest.go`/`merge.go` — the reasoning guard, field, Resolve behavior, and
  merge all already exist and are correct. Only the manifest DATA is missing.
- ❌ Don't populate pi here (pi uses `--thinking` — external_deps.md §pi; that's S2 / P1.M1.T1.S2).
- ❌ Don't leave the other 6 providers' `ReasoningLevels` anything but nil — they have no verified
  reasoning control (external_deps.md §38-42); the FR-R6 graceful no-op applies honestly.
- ❌ Don't change the `default` column of the docs/providers.md reasoning_levels row — it's the schema
  default ("nil (none)"), not claude-specific. Only update the DESCRIPTION cell.
- ❌ Don't use a synthetic manifest in the new test — the contract requires the REAL `builtinClaude()`
  (the synthetic-manifest case is already covered by TestRender_ReasoningTokensAppended).

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: This is a data-only manifest change with the verified tokens sourced from external_deps.md
(`--effort`, not the PRD's `--thinking-effort` guess), the Render guard already correct, and the complete
test + TOML blocks specified verbatim. The one non-obvious trap — the THREE parity surfaces (builtin.go +
claudeTOML const + providers/claude.toml) enforced by TWO `reflect.DeepEqual` tests — is called out as the
#1 failure mode with the exact mitigation (append the matching table to both fixtures). The render-test
Args are traced concretely (claude ProviderFlag="" → no FR-R5b split → `--model sonnet --effort high`).
The residual uncertainty (not 10/10) is purely mechanical: ensuring the two TOML fixtures match the
builtin map byte-for-byte (data) — which the deterministic `go test -race` parity gates catch immediately.
pi (S2), Issue 2 (P1.M2), and Issue 3 (P1.M3) are cleanly fenced and untouched.
