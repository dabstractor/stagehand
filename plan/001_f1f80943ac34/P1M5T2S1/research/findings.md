# Research: providers/*.toml reference files (P1.M5.T2.S1)

> Scope: 6 human-readable TOML reference files at repo-root `providers/` (pi/claude/gemini/opencode/
> codex/cursor) that MIRROR the compiled-in manifests (`internal/provider/builtin.go`) with explanatory
> comments. Per the contract these are **reference documentation, NOT runtime config** — they are not
> loaded by the binary (built-ins are compiled in); they serve as templates users study/copy (§12.8).

## 1. Authoritative sources (ranked)

| Source | What it gives | Authority |
|---|---|---|
| `internal/provider/builtin.go` | the 6 compiled-in `Manifest` constructors (the thing these files MIRROR) | PRIMARY — the source of truth |
| `internal/provider/builtin_test.go` lines 16–135 | `piTOML`…`cursorTOML` constants — byte-faithful TOML proven == builtin via `TestBuiltinManifests_DecodeParity` | PRIMARY — the canonical CONTENT skeleton |
| `internal/provider/manifest.go` | the `Manifest` struct + §12.1 field semantics + Resolve defaults | PRIMARY — field glossary |
| `plan/.../architecture/external_deps.md` | live `--help` captures (2026-06-29); codex discrepancy; rendered commands | PRIMARY — agent-specific notes |
| PRD §12.1 / §12.3–§12.7 / §12.7.1 / §12.8 / Appendix D | schema, per-provider notes, tools-disable asymmetry, override syntax, quick ref | PRIMARY — explanatory copy |
| `plan/.../P1M2T1S1/research/go-toml-pointer-behavior.md` | nil-vs-empty decode behavior (FINDING C/D) — THE fidelity gotcha | PRIMARY — why absent ≠ `""` |

## 2. DECISION — file format: FLAT `name = "<x>"`, NOT `[provider.<x>]`

The reference files use the **flat manifest schema** (top-level `name = "pi"`, …), identical to:
- PRD §12.3–§12.7 (how the PRD itself presents each manifest),
- `providers show <name>` output (`Registry.MarshalTOML` marshals the flat `Manifest` struct),
- the `piTOML`…`cursorTOML` decode-parity constants.

They do **NOT** use the `[provider.<name>]` table form — that is the **config-file override** syntax
(§12.8, e.g. the existing `.stagecoach.toml` `[provider.pi]`). The contract says "mirror the compiled-in
manifests"; the compiled-in `Manifest` struct marshals flat. A **header comment** in each file explains
the §12.8 transform ("to use as an override: wrap this body in `[provider.<name>]` and delete the `name`
line") — so the "copy into your config" use case is documented without sacrificing the flat mirror.

## 3. THE fidelity gotcha — nil (ABSENT) vs `strPtr("")` (present-empty `= ""`)

go-toml/v2 (FINDING C/D): an ABSENT key decodes to `nil`; a PRESENT `key = ""` decodes to a NON-NIL
pointer/slice whose value is the zero value. The built-in constructors deliberately distinguish these
(`default_provider = ""` on pi is NON-NIL empty ≠ absent). **The reference files must reproduce the
exact same present/absent pattern** or they no longer mirror the compiled-in manifest.

Per-manifest field map (`✓` = present in the TOML; `—` = ABSENT, must NOT appear):

| field | pi | claude | gemini | opencode | codex | cursor |
|---|---|---|---|---|---|---|
| name | ✓ pi | ✓ claude | ✓ gemini | ✓ opencode | ✓ codex | ✓ cursor |
| detect | ✓ pi | ✓ claude | ✓ gemini | ✓ opencode | ✓ codex | ✓ **agent** |
| command | ✓ pi | ✓ claude | ✓ gemini | ✓ opencode | ✓ codex | ✓ **agent** |
| subcommand | — | — | — | ✓ ["run"] | ✓ ["exec"] | ✓ [] (NON-NIL empty) |
| prompt_delivery | ✓ stdin | ✓ stdin | ✓ **stdin** (revised) | ✓ positional | ✓ **stdin** (rev#1) | ✓ positional |
| prompt_flag | — | — | — | — | — | — |
| print_flag | ✓ -p | ✓ -p | ✓ "" | ✓ "" | ✓ "" | ✓ -p |
| model_flag | ✓ --model | ✓ --model | ✓ -m | ✓ -m | ✓ -m | ✓ --model |
| default_model | ✓ glm-5-turbo | ✓ sonnet | ✓ gemini-2.5-pro | ✓ "" | ✓ "" | ✓ "" |
| system_prompt_flag | ✓ --system-prompt | ✓ --system-prompt | ✓ "" | ✓ "" | ✓ "" | ✓ "" |
| provider_flag | ✓ --provider | ✓ "" | ✓ "" | ✓ "" | ✓ "" | ✓ "" |
| default_provider | ✓ "" (NON-NIL) | — | — | — | — | — |
| bare_flags | ✓ 6 flags | ✓ [--tools,"",--setting-sources,"",--no-session-persistence] | ✓ [--approval-mode,default] | ✓ [] (NON-NIL empty) | ✓ [--sandbox,read-only,**--ephemeral**] (rev#2) | ✓ [--mode,ask,--trust] |
| output | ✓ raw | ✓ raw | ✓ raw | ✓ raw | ✓ raw | ✓ raw |
| json_field | — | — | — | — | — | — |
| strip_code_fence | ✓ true | ✓ true | ✓ true | ✓ true | ✓ true | ✓ true |
| retry_instruction | — | — | — | — | — | — |
| env | — | — | — | — | — | — |

**Bold** = the high-risk cells (where a careless copy fails the mirror):
- pi `default_provider = ""` — NON-NIL empty (do NOT omit).
- claude bare_flags — TWO `""` value tokens (`--tools ""` / `--setting-sources ""`); do NOT drop them.
- gemini/codex `prompt_delivery = "stdin"` — REVISED from §12.5/§12.7 "positional".
- opencode `bare_flags = []` + cursor `subcommand = []` — present-but-empty arrays (NON-NIL empty);
  must be WRITTEN as `[]`, not omitted (omitting → nil → mirror fails).
- codex `bare_flags` — REVISED: dropped `--ask-for-approval never` (not a `codex exec` flag); added
  `--ephemeral`.
- cursor `detect`/`command = "agent"` — ≠ name "cursor" (the only provider where detect ≠ name).

## 4. DECISION — validation: a round-trip decode-parity test (the sync-guard)

`builtin_test.go` ALREADY proves manifest fidelity via `TestBuiltinManifests_DecodeParity` (decodes the
`piTOML`…`cursorTOML` string constants and `reflect.DeepEqual`s them to the builtin constructors). The
reference-file task extends the SAME pattern to FILES: a new `internal/provider/referencefiles_test.go`
(`package provider`) reads each `providers/<name>.toml`, `toml.Unmarshal`s it, and `reflect.DeepEqual`s
it to `BuiltinManifests()[name]`. Comments are stripped by the parser, so a correctly-commented file
decodes to the SAME struct as the bare parity constant.

This is the ONLY deterministic proof that the docs mirror the code. Without it the files silently drift
(a builtin flag changes, the .toml isn't updated → docs lie). It is cheap (one file, ~40 LOC, stdlib +
existing go-toml) and runs in `make test` (no build tag). **The 6 files are the deliverable; the test is
the sync-guard.**

Path resolution: tests run with CWD = the package dir (`internal/provider/`); repo root is 2 levels up.
Resolve robustly via `runtime.Caller(0)` → `filepath.Dir(file)` → `Join(.., .., "providers", name+".toml")`
(bulletproof regardless of invocation). `BuiltinManifests()` is EXPORTED → the test compiles in either
`package provider` or `package provider_test`; use `package provider` for consistency with `builtin_test.go`.

## 5. Confirmations (what's already true — no wiring needed)

- **No runtime loading of `providers/`:** the config loader (`internal/config/*.go`) reads the single
  global + repo-local config files (e.g. `.stagecoach.toml`), NOT a `providers/` directory. No `embed`,
  no `ReadDir`, no `Glob` of `providers/`. The 6 files are inert documentation. (grep-verified.)
- **`providers show` uses flat `MarshalTOML`** → the flat reference format is consistent with the CLI's
  own output. Good for user mental model.
- **No README yet** (README is P1.M5.T4) → no cross-doc to keep in sync; the README task will LINK to
  `providers/` later.
- **`providers/` is at repo root** per PRD §14 (sibling of `internal/`, `cmd/`, `docs/`).
- **go.mod module:** `github.com/dustin/stagecoach`, go 1.22. No new deps for the test (stdlib +
  existing go-toml/v2).

## 6. The two `# TO CONFIRM` items to carry in the codex + cursor files

From PRD Appendix E #4 + §12.7 inline notes (the SAME items P1.M5.T1.S2's real-agent suite resolves):
1. **codex** — that `codex exec` writes the assistant's FINAL answer to stdout and exits 0 on success.
   Fallbacks: `-o <file>` (last message to file), `--json` (JSONL events). Expected; unconfirmed.
2. **cursor** — that `--mode ask` wins over `-p`'s default FULL-tools profile (i.e. the combo is
   genuinely read-only). Expected (`ask` = read-only Q&A); unconfirmed.

These are documentation honesty (§12.7.2), NOT blockers. Carry them as `# TO CONFIRM (integration):`
comments in the codex + cursor files exactly as the contract requires.

## 7. Scope guard (do NOT do — owned elsewhere / out of scope)

- Do NOT wire `providers/` into any loader, registry, or `//go:embed` — they are inert docs.
- Do NOT change `builtin.go`, `manifest.go`, `merge.go`, `registry.go`, config, or any production code.
- Do NOT add a build tag to the new test (it runs in CI alongside the rest of `make test`).
- Do NOT use `[provider.<name>]` table format — flat `name = "<x>"` (§2 above).
- Do NOT re-open the codex discrepancy; the built-in already resolves it (`--ephemeral`, stdin).
