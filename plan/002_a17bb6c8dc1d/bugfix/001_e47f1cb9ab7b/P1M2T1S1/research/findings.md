# P1.M2.T1.S1 — Research Findings (synthesis)

Binding architecture: `../architecture/issue2_stager_toolset.md` (three-pronged fix). THIS subtask
is ONLY prong (a) — tighten **claude's** TooledFlags. Prong (b) (pi honest doc) is S2; prong (c)
(HEAD guard in decompose.go) is S3. Do NOT touch pi or decompose here.

## The change (one string, three artifacts — byte-for-byte identical)

`Bash(git:*),Read,Edit`  →  `Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit`

(contract-specified value, verbatim — single spaces after each `git`, comma-separated, no extra spaces).
`Bash(git:*)` permitted EVERY git subcommand (commit/push/update-ref/reset/rebase/amend). The allowlist
restricts to staging-relevant subcommands only → those ref-mutating commands are structurally unreachable
for the claude stager (delivers PRD §19's "cannot commit/amend/push" claim for claude).

## ⚠️ THE #1 TRAP — `reflect.DeepEqual` parity across THREE artifacts

Two decode-parity oracles + the field/render tests all assert the EXACT TooledFlags slice:
1. `TestBuiltinManifests_DecodeParity` (builtin_test.go) — `builtinClaude()` vs the `claudeTOML` literal.
2. `TestProviderReferenceFiles_DecodeParity` (referencefiles_test.go) — `builtinClaude()` vs `providers/claude.toml`.
3. `TestBuiltinClaude` (fields, wantTooled) + `TestBuiltinManifests_RenderedCommand_Claude_Tooled`.

⇒ `builtin.go` (builtinClaude), the `claudeTOML` test literal, and `providers/claude.toml` must all carry
the IDENTICAL new string. If any one lags at `Bash(git:*)`, DeepEqual fails. Use the contract string
BYTE-FOR-BYTE on all three (the spaces matter).

## The complete edit set (verified by grep — these are ALL the `Bash(git:*)` sites for claude)

- `internal/provider/builtin.go:109` — the TooledFlags value. ALSO update the multi-line comment above it
  (lines 100-104): replace "Bash(git:*) (git only)" with the staging-only allowlist explanation (git
  add/apply/status/diff only; explicitly excludes commit/push/update-ref/reset/rebase/amend).
- `providers/claude.toml:68` — the tooled_flags value. ALSO update the section comment (57-65) and the
  RENDERED TOOLED COMMAND comment (line 74).
- `internal/provider/builtin_test.go:60` — the claudeTOML literal value (decode-parity oracle #1).
- `internal/provider/builtin_test.go:314,316` — TestBuiltinClaude wantTooled comment + value.
- `internal/provider/builtin_test.go:762,773` — TestBuiltinManifests_RenderedCommand_Claude_Tooled comment + value.

## Confirmed NO edits needed (verified — do NOT touch these)

- `internal/provider/render_test.go` — `TestRender_TooledModeAppendsTooledFlags` uses `dualModeManifest()`
  (a CUSTOM fixture, `--allowed-tools git:*`, NOT claude) and only checks token PRESENCE (`--allowed-tools`
  present). It does not snapshot claude's allowlist string. (The contract's "TestRender_Tooled" mention is
  imprecise — the actual claude-snapshot render test is `TestBuiltinManifests_RenderedCommand_Claude_Tooled`
  in builtin_test.go, which IS edited above.)
- `internal/provider/merge_test.go:29` — custom merge fixture (`--allowed-tools git:*`), NOT claude.
- `internal/decompose/*` — stager tests check ONLY `len(TooledFlags) != 0` (non-empty); they never reference
  the allowlist string. The new value is still non-empty → all decompose stager tests stay green.
- `docs/providers.md:94` — generic prose ("claude: git/read/edit allowlist") is still accurate; this task's
  DOCS scope is providers/claude.toml only (docs/providers.md is swept by P1.M6).
- `PRD.md` — READ-ONLY (carries the old `Bash(git:*)` examples; not this task's concern).

## Why no external/online research is warranted

The exact replacement string is FIXED by the work-item contract. Claude Code's `--allowed-tools` Bash-prefix
syntax is already flagged as a "# TO CONFIRM at integration" item (P3.M2.T3 real-stager run); this task
ships the contract's specified string verbatim. Nothing to look up online.
