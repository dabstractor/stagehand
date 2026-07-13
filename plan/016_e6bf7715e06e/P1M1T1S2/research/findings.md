# Research Findings — P1.M1.T1.S2 (Mirror CHROME-DISABLE notes in providers/*.toml)

## 1. The deliverable (mirror S1 into 7 TOML reference files)

providers/*.toml are HUMAN-READABLE REFERENCE DOCUMENTATION that mirror the compiled-in manifests in
builtin.go byte-for-byte (modulo comments). They are NOT loaded at runtime. Add a `# CHROME-DISABLE`
comment paragraph to each file's header block, mirroring the note S1 (P1.M1.T1.S1) adds to the
corresponding `builtinXxx()` doc-comment in builtin.go.

7 files: providers/{pi,claude,agy,qwen-code,opencode,codex,cursor}.toml.

## 2. S1 HAS LANDED — builtin.go notes are the source of truth (read them)

S1 landed during this research: `grep -c CHROME-DISABLE internal/provider/builtin.go` == 7. The 7
landed `// CHROME-DISABLE (FR-C5, §9.28):` notes were read and VERIFIED to match the per-provider
content below (including the qwen-code "(S2)" cross-reference ambiguity — simplify it in the TOML
mirror). READ the actual builtin.go notes at implementation time and mirror their wording
(translating `//` → `#`). If for any reason the count is not 7, fall back to S1's PRP
(plan/016_*/P1M1T1S1/PRP.md Tasks 2-8) as the contract.

## 3. CRITICAL: the decode-parity test (comments are safe; fields must NOT change)

`internal/provider/referencefiles_test.go::TestProviderReferenceFiles_DecodeParity` reads each
providers/*.toml, `toml.Unmarshal`s it into a `Manifest`, and `reflect.DeepEqual`-checks against the
builtin. Implications:
- TOML `#` comments are INERT — `toml.Unmarshal` ignores them, so adding `# CHROME-DISABLE` comment
  lines to the header does NOT change the decoded `Manifest` → the test stays GREEN.
- DO NOT change any FIELD line (name=, bare_flags=, etc.) — those must stay byte-identical to builtin.go.
- This test is the safety net: if S2 accidentally alters a field, the parity test fails immediately.

There is also `TestProviderReferenceFiles_AllBuiltinsCovered` (every builtin has a .toml and vice-
versa) — S2 adds no new file, so it's unaffected.

## 4. Header structure + placement anchor (all 7 files share this shape)

Each providers/*.toml opens with a header comment block delimited by `# ===...` lines:
```
# ===...
# <name> — reference manifest for the `<name>` built-in provider (PRD §...)
# ===...
#
# WHAT THIS FILE IS ...
# HOW TO USE IT AS A CONFIG OVERRIDE ...
# RENDERED COMMAND ...
# TOOLS-DISABLE CATEGORY (§12.7.1): <...>     ← existing note (the pattern to mirror)
# ====...                                       ← closing delimiter of the header block
```
PLACE the `# CHROME-DISABLE (FR-C5, §9.28):` paragraph IMMEDIATELY AFTER the `TOOLS-DISABLE CATEGORY`
paragraph and BEFORE the closing `# ===` delimiter. TOOLS-DISABLE line per file (grep-verified):
- pi.toml:26, claude.toml:24, agy.toml:53, qwen-code.toml:39, opencode.toml:25, codex.toml:24,
  cursor.toml:24. (Line numbers may drift slightly; anchor on the `TOOLS-DISABLE CATEGORY` text.)

## 5. Comment style (TOML `#`, not Go `//`)

S1 writes Go `//` doc-comments; S2 writes TOML `#` comments. The marker becomes
`# CHROME-DISABLE (FR-C5, §9.28):` (grep-able, mirrors S1's `// CHROME-DISABLE (FR-C5, §9.28):`).
Indentation: `#   ` (hash + 3 spaces) for continuation lines, matching the existing TOOLS-DISABLE
paragraph's style.

## 6. Per-provider content (mirror S1's substance, CONCISE per item: 1-4 lines)

The item says "Keep it concise (1-4 lines per provider)" AND "must match the builtin.go note in
substance: what chrome surfaces are disabled (by which flag), what is not (no switch), and the
limitation status." S1's Go notes are longer (5-7 lines for pi); condense for TOML while keeping the
three required substance elements + the verification source/date (FR-D5/FR-T9 discipline).

**pi** (verified vs `pi --help`, 2026-06-29): 4 chrome surfaces disabled by named flags; MCP gap.
**claude** (verified vs `claude --help`): chrome COVERED via --tools "" + --setting-sources "".
**agy** (verified vs `agy --help`, v1.1.0, 2026-07-08): no chrome switch; --mode plan is constraint only.
**qwen-code** (docs-only, NOT --help-verified; # TO CONFIRM per FR-D5): no chrome switch; --approval-mode
  default is constraint only; surface unverified.
**opencode** (verified vs `opencode run --help`, opencode 1.1.23, 2026-07-08): run is read-only by
  design; bare_flags empty; no chrome switch.
**codex** (verified vs `codex exec --help`, codex-cli 0.143.0, 2026-07-08): no chrome switch;
  --sandbox read-only + --ephemeral are constraint only.
**cursor** (verified vs `agent --help`): no chrome switch; --mode ask + --trust are constraint only.

Verification dates confirmed in plan/016_*/architecture/external_deps.md.

## 7. Scope boundaries (do NOT do)
- Do NOT modify builtin.go (S1 owns it — S2 mirrors its output).
- Do NOT change any FIELD line in providers/*.toml (decode-parity test enforces byte-identity).
- Do NOT add a new providers/*.toml file or a new builtin (T2/AllBuiltinsCovered scope).
- Do NOT modify the builtin_test.go chrome-contract assertions (P1.M1.T2.S1).
- Do NOT touch docs/providers.md / README.md (P1.M2.T1).
- Do NOT invent flags (FR-C4) — the notes document what exists, including gaps.

## 8. Validation

Test/doc-only (TOML comments). Gates: `go build ./...` (no-op for comments), `go test
./internal/provider/...` (the decode-parity test MUST still pass — proves fields unchanged), `make
test`, `make lint`. A grep guard: 7 CHROME-DISABLE markers across the 7 files. No external libs.
