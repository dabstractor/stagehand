# P2.M1.T1.S1 Research Findings — builtinQwenCode() + registry priority + providers/qwen-code.toml

Empirically derived from the live `internal/provider/*` source (plan 003). These pin the EXACT edits +
the stale-contract correction so the implementing agent does not chase a symbol that does not exist.

## §0. THE CONTRACT REFERENCE `builtinFuncs` IS STALE — registration is the `BuiltinManifests()` map literal

The work-item contract says "Register in builtinFuncs (builtin.go)". **There is NO `builtinFuncs` symbol.**
`grep -rn builtinFuncs internal/provider/` returns nothing. The actual registration mechanism is the
**map literal returned by `BuiltinManifests()`** (builtin.go:17-28):

```go
func BuiltinManifests() map[string]Manifest {
	return map[string]Manifest{
		"pi":       builtinPi(),
		"claude":   builtinClaude(),
		"gemini":   builtinGemini(),
		"opencode": builtinOpenCode(),
		"codex":    builtinCodex(),
		"cursor":   builtinCursor(),
		"agy":      builtinAgy(),
	}
}
```

⇒ The edit is: **add `"qwen-code": builtinQwenCode(),`** to this map literal (alphabetical-ish grouping
is loose; placement is not load-bearing — registry.List() sorts). AND update the `BuiltinManifests()`
doc comment which currently says **"All seven providers are now present"** (builtin.go:16) → "eight".
Do NOT search for `builtinFuncs`; it is not there.

## §1. `preferredBuiltins` is a package var (registry.go:16) — insert "qwen-code" between gemini and codex

```go
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "codex", "claude"}
```
FR-D1 target order (PRD §9.16 h3.32): `[pi, opencode, cursor, agy, gemini, qwen-code, codex, claude]`.
⇒ edit registry.go:16 → `... "agy", "gemini", "qwen-code", "codex", "claude"`. This single-line edit is the
ENTIRE priority change — `DefaultProvider`/`FirstTooledProvider` iterate `preferredBuiltins`, so the new
rank flows automatically. New ranks: pi(1) opencode(2) cursor(3) agy(4) gemini(5) **qwen-code(6)** codex(7) claude(8).

**MINOR accuracy fix in the SAME file**: `NewRegistry` (registry.go:36) allocates
`make(map[string]Manifest, len(userOverrides)+7)` — the `+7` headroom hint is now stale (8 built-ins).
Update to `+8`. (A map auto-grows so `+7` still WORKS, but keep the hint honest — it is a one-char fix in
the file you are already editing.)

## §2. The mirror template is `builtinAgy()` (the closest gemini-lineage twin with Experimental=true)

`builtinQwenCode()` differs from `builtinAgy()` in ONLY THREE things: Name/Detect/Command, DefaultModel,
and the doc comment. EVERY other field is byte-identical to agy (both are experimental gemini-CLI forks,
single-backend, stdin delivery, no sys-prompt flag, `--approval-mode default` bare, nil TooledFlags).

| field              | agy                    | qwen-code              | source                |
|--------------------|------------------------|------------------------|-----------------------|
| Name               | "agy"                  | "qwen-code"            | contract (a)          |
| Detect             | "agy"                  | "qwen-code"            | contract (a)          |
| Command            | "agy"                  | "qwen-code"            | contract (a)          |
| DefaultModel       | "gemini-2.5-pro"       | "qwen3-coder-plus"     | contract (a) # TO CONFIRM FR-D5 |
| Experimental       | boolPtr(true)          | boolPtr(true)          | contract (a) + PRD §12.5.2 |
| PromptDelivery     | "stdin"                | "stdin"                | (same)                |
| PrintFlag          | "-p"                   | "-p"                   | (same)                |
| ModelFlag          | "-m"                   | "-m"                   | (same)                |
| SystemPromptFlag   | "" (NON-NIL)           | "" (NON-NIL)           | (same)                |
| ProviderFlag       | "" (NON-NIL)           | "" (NON-NIL)           | (same) single-backend |
| BareFlags          | ["--approval-mode","default"] | ["--approval-mode","default"] | (same) |
| Output             | "raw"                  | "raw"                  | (same)                |
| StripCodeFence     | boolPtr(true)          | boolPtr(true)          | (same)                |
| TooledFlags        | nil                    | nil                    | (same) — cannot stager |
| Subcommand/PromptFlag/JsonField/RetryInstruction/Env/ReasoningLevels | nil | nil | (same) |

⇒ Copy `builtinAgy()` verbatim, change Name/Detect/Command to "qwen-code", DefaultModel to
"qwen3-coder-plus", rewrite the doc comment (Qwen3-Coder / DashScope / `# TO CONFIRM` / experimental).

## §3. The MODEL TOKEN question (FR-D5) — S1 sets a PLACEHOLDER; S2 owns the refresh

The PRD §12.5.2 + the work-item contract BOTH say `default_model = "qwen3-coder-plus"` with a
**`# TO CONFIRM per FR-D5`** note. NOTE: the codebase has ALREADY refreshed the OTHER gemini-lineage
tokens to their REAL current values (`gemini-2.5-pro`, not the PRD's aspirational `gemini-3.1-pro`) — that
is FR-D5 ("re-verify at implementation") applied. **The actual token verification/refresh for qwen-code is
S2's deliverable** (P2.M1.T1.S2: "qwen-code tier row (FR-D4) + model-token refresh (FR-D5)"). S1 therefore
ships the manifest-level `DefaultModel = "qwen3-coder-plus"` AS A DOCUMENTED PLACEHOLDER (`# TO CONFIRM`),
exactly as agy ships `gemini-2.5-pro` while still `experimental`. The per-ROLE tier table
(`qwen3-coder-plus` planner/arbiter, `qwen3-coder-flash` stager/message per FR-D4) is S2's job in
`internal/config/role_defaults.go` — **S1 MUST NOT touch role_defaults.go**.

## §4. Tests to UPDATE (contract item e) — exactly two assertions

1. **`TestPreferredBuiltins_MatchesBuiltinKeys`** (registry_test.go:15) — the `wantOrder` slice:
   ```go
   wantOrder := []string{"pi", "opencode", "cursor", "agy", "gemini", "codex", "claude"}
   ```
   → insert "qwen-code" between "gemini" and "codex". (The `len(set) != len(bk)` guard stays dynamic and
   passes since both grow to 8.) The `preferredBuiltins[0] != "pi"` guard is unaffected.

2. **`TestBuiltinManifests_KeysAndCount`** (builtin_test.go:194) — `want 7` → `want 8`, and add
   `"qwen-code"` to the `for _, k := range []string{...}` slice.

**No other existing test changes.** Verified case-by-case:
- `TestDefaultProvider` (registry_test.go): no case installs qwen-code; relative ranks of the providers IN
  each case are preserved by inserting qwen-code between gemini(5) and codex(7→8). E.g. `{codex,claude}`
  → codex still wins (codex now 7, claude 8); `{claude,gemini}` → gemini(5) still beats claude(8). PASS UNCHANGED.
- `TestFirstTooledProvider`: qwen-code has nil TooledFlags → skipped; no case installs it. PASS UNCHANGED.
- `TestNewRegistry_NoOverrides_HasAllBuiltins`: `len(r.manifests) != len(BuiltinManifests())` is dynamic. PASS.
- `TestNewRegistry_NewName_AddedVerbatim` / `TestDecodeUserOverrides`: use `len(BuiltinManifests())+1`. dynamic. PASS.

## §5. Tests to ADD (pattern consistency — every other builtin has these)

Every built-in manifest has, in builtin_test.go: a `*Fields` test, a `*TOML` const + a `DecodeParity`
table entry, and a `RenderedCommand_*` test. qwen-code is the ONLY one lacking them if omitted → a
coverage gap and a broken `DecodeParity` byte-faithfulness story. ADD (all mirror the agy tests):

1. **`qwenCodeTOML` const** (next to `agyTOML`, builtin_test.go:129) — byte-faithful to `builtinQwenCode()`:
   name/detect/command="qwen-code", prompt_delivery="stdin", print_flag="-p", model_flag="-m",
   default_model="qwen3-coder-plus", system_prompt_flag="", provider_flag="", bare_flags=["--approval-mode","default"],
   output="raw", strip_code_fence=true, experimental=true. OMIT subcommand/prompt_flag/json_field/
   retry_instruction/tooled_flags/env/reasoning_levels (nil in the built-in). (Identical shape to agyTOML.)
2. **`TestBuiltinManifests_QwenCodeFields`** — copy `TestBuiltinManifests_AgyFields`, change the 3 values
   (Detect/Command="qwen-code", DefaultModel="qwen3-coder-plus") + keep Experimental non-nil true, TooledFlags nil.
3. **DecodeParity table entry**: `{"qwen-code", builtinQwenCode(), qwenCodeTOML},` in the `TestBuiltinManifests_DecodeParity` table.
4. **`TestBuiltinManifests_RenderedCommand_QwenCode`** — copy the agy rendered-command test; want argv =
   `["qwen-code", "-m", "qwen3-coder-plus", "--approval-mode", "default", "-p"]` (stdin delivery → payload
   piped, NOT in argv; identical structure to agy).

## §6. `providers/qwen-code.toml` mirrors `providers/agy.toml` (the reference-file header pattern)

`providers/*.toml` are human-readable REFERENCE docs (NOT loaded at runtime — built-ins are compiled in).
Each has a big comment header (WHAT THIS FILE IS / HOW TO USE IT AS A CONFIG OVERRIDE / RENDERED COMMAND /
EXPERIMENTAL / STAGER / TOOLS-DISABLE CATEGORY) then the field lines mirroring the manifest.
`providers/qwen-code.toml` mirrors `providers/agy.toml`'s structure, adapted for qwen-code:
- RENDERED COMMAND: `qwen-code -m qwen3-coder-plus --approval-mode default -p < "<sys>\n\n<user payload>"`
- EXPERIMENTAL note: ships from docs (not `--help`-verified) per §12.5.2 / §12.7.2; # TO CONFIRM items
  (model flag token, exact default model, reasoning levels, gemini-equivalent approval mode).
- DashScope note: reached via `DASHSCOPE_API_KEY` or `qwen-code login` (the free coding-plan quota); single-backend.
- STAGER note: TooledFlags intentionally nil → cannot stager until verified (FR-D4 fallback).
- Field lines mirror builtinQwenCode() byte-for-byte (modulo comments), including `experimental = true`.

## §7. Parallel-sibling coordination — ZERO overlap (safe)

The parallel work item P1.M3.T1.S2 (config upgrade on-disk →v3) touches **only** `internal/cmd/config.go`
+ `internal/cmd/config_test.go`. This task (P2.M1.T1.S1) touches **only** `internal/provider/{builtin.go,
builtin_test.go, registry.go, registry_test.go}` + `providers/qwen-code.toml`. **No shared file.** No
coordination needed beyond confirming the non-overlap (done). Adding qwen-code to the registry is
TRANSPARENT to the config layer: the bootstrap iterates `BuiltinManifests()`/`reg.List()` dynamically
(commented-provider blocks auto-include qwen-code); the in-memory v3 migration's `v2MultiBackendBuiltins
={"pi"}` is unaffected (qwen-code is single-backend). S2 (qwen-code tier row + token refresh +
docs/providers.md) is SEQUENTIAL after S1, not parallel — S1 must not touch `internal/config/role_defaults.go`
or `docs/providers.md` (S2 owns them).

## §8. Rendered command (verified via the renderArgs helper semantics — §12.2 algorithm)

`renderArgs(builtinQwenCode(), "", "", "<sys>")` (model="" → default "qwen3-coder-plus"):
```
["qwen-code", "-m", "qwen3-coder-plus", "--approval-mode", "default", "-p"]
```
stdin delivery ⇒ the payload (`<sys>\n\n<user>`) is PIPED, NOT in argv; print_flag `-p` is LAST (§12.2).
Identical structure to agy's rendered command (the only diff is the binary name + default model).

## §9. Confidence: 9/10

The manifest is a near-verbatim copy of `builtinAgy()` (the closest twin) with 3 changed values; the
registry edit is one line + a one-char hint; the toml mirrors `providers/agy.toml`; the test changes are
2 mandated updates + 4 pattern-consistency additions that copy existing agy tests. The only residual
uncertainty is the `# TO CONFIRM` model token (deliberately deferred to S2 per FR-D5). No new types, no
new imports, no interface change, no dependency change.
