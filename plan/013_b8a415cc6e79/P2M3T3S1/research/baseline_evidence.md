# P2.M3.T3.S1 — Research Notes: Baseline Evidence (Mode B Final Verification)

**Task**: Comprehensive grep audit + build/test verification (the catch-all Mode B gate for the
P2 provider-lineup correction: gemini removal + agy v1.1.0 re-verification).

**Method**: Empirical — ran every contract check against the current working tree
(all P2.M1/M2 + P2.M3.T1 committed; P2.M3.T2 applied in working tree).

---

## 1. PASS (a) — Grep audit: `git grep -in 'gemini' -- docs/ README.md providers/ internal/ pkg/`

Every remaining `gemini`/`Gemini` hit falls into exactly ONE of two LEGITIMATE categories.
**Zero hits imply gemini is a shipped built-in provider.**

### Category A — MODEL NAMES (display labels from `agy models` / FR-D4 tiers). LEAVE AS-IS.
| Location | Text | Why legit |
|---|---|---|
| `docs/providers.md:85` | `Gemini 3.5 Flash (Low)` | model display label (agy default) |
| `docs/providers.md:131` | `Gemini 3.5 Flash (High/Medium/Low)` | FR-D4 per-role model tier table row |
| `internal/cmd/config.go:614,618` | `# model = "Gemini 3.5 Flash (High/Medium)"` | COMMENTED config-init exemplar (agy column) |
| `internal/cmd/config_test.go:390` | `model = "Gemini 3.5 Flash (High)"` | test assertion on agy planner model |
| `internal/config/bootstrap_test.go:71,86,87` | `Gemini 3.5 Flash (High/Low/Medium)` | bootstrap test assertions (agy models) |
| `internal/config/role_defaults.go:68,70,71` | `"Gemini 3.5 Flash (High/Low/Medium)"` | FR-D4 agy tier values (compiled) |
| `internal/config/role_defaults.go:26` | `"Gemini 3.5 Flash ..."` | doc comment on agy tiers |
| `internal/provider/builtin.go:176,192,207` | `Gemini 3.5 Flash (Low)` | agy DefaultModel + label note |
| `internal/provider/builtin_test.go:149,650,688,690` | `Gemini 3.5 Flash (Low)` | agy manifest/render tests |
| `providers/agy.toml:21,28,76` | `Gemini 3.5 Flash (Low)` | reference manifest comments |

### Category B — LINEAGE / HISTORY comments (EOL / superseded / fork). LEAVE AS-IS.
| Location | Text | Why legit |
|---|---|---|
| `README.md:355` | `Google's gemini / Gemini CLI is no longer shipped — superseded by agy ... 2026-06-18` | explicit removal note (correct) |
| `docs/providers.md:88` | `a Gemini-CLI fork for Qwen3-Coder via DashScope` | qwen-code lineage |
| `internal/generate/realagent_test.go:48` | `gemini (removed; superseded by agy)` | test-fixture comment (providerNames slice) |
| `internal/provider/builtin.go:16,154,155,156,166,195,196,222,225,226,235,245,276` | `Gemini-CLI successor ... EOL ... superseded ... gemini-cli lineage` | lineage comments |
| `internal/config/role_defaults.go:29` | `API-style ids (gemini-3.5-flash) are silently ignored` | model-name caveat |
| `providers/agy.toml:30,35,36` | `gemini-3.5-flash` / `DIVERGENCE FROM GEMINI-CLI LINEAGE` | lineage comments |
| `providers/qwen-code.toml:24,31,69` | `gemini-equivalence` / `Gemini CLI fork` / `# TO CONFIRM gemini-equivalent` | lineage comments |

**Classification rubric for the implementing agent:** a hit is STALE (must fix) ONLY if the
surrounding sentence states or implies gemini is a CURRENT shipped built-in provider (e.g.
"gemini is built in", "the eight providers ... gemini", a `providers/gemini.toml` reference, a
built-in enumeration list containing `gemini`, or `BuiltinManifests()`/`preferredBuiltins`
containing a `"gemini"` entry). Model-name labels and lineage/history prose are NOT stale.

**Baseline finding: ZERO stale references.** No fix expected. (See `providers/gemini.toml` check
below — confirms the file is gone.)

---

## 2. PASS (b) — Count check across the six contract files

Command: `grep -rniE 'eight built|8 built|the 8 built-in|eight providers'` over the 6 files.

| File | Provider count | Says "gemini as built-in"? | Status |
|---|---|---|---|
| `docs/cli.md` | (no count clause / uses 7-provider order) | no | CLEAN |
| `docs/configuration.md` | (no count clause / uses 7-provider order) | no | CLEAN |
| `docs/how-it-works.md` | (no count clause / uses 7-provider order) | no | CLEAN |
| `docs/README.md:35` | `the 7 built-in providers` | no | CLEAN (D8 fixed by T2) |
| `docs/providers.md:3,7,74,92` | `7` / `Seven` | no | CLEAN (D4–D7 fixed by T1) |
| `README.md:355` | `Seven built-ins` | explicit "no longer shipped" note | CLEAN |

**Baseline finding: ZERO "eight"/"8" provider framing; zero gemini-as-built-in.** No fix expected.

---

## 3. PASS (c) — Smoke test (`make build` + `providers list` + `providers show agy` + targeted `go test`)

### 3a. `make build` → exits 0, produces `./bin/stagecoach`.
```
go build -ldflags "-X main.version=dev" -o bin/stagecoach ./cmd/stagecoach
```

### 3b. `./bin/stagecoach providers list` → EXACTLY 7 NAME rows, NO `gemini` row.
```
NAME       DETECTED  DEFAULT
agy        ✓
claude     ✓
codex      ✓
cursor     ✓
opencode   ✓
pi         ✓         (default)
qwen-code  ✗
```
- **Assertion target**: count exactly 7 data rows; none named `gemini`.
- **ENVIRONMENT-VARIANT**: the ✓/✗ DETECTED marks AND the `(default)` marker depend on what is on
  `$PATH` at run time. DO NOT assert on them — assert only on the NAME column count + absence of gemini.
- Provider order is sorted ascending by Name (tabwriter output). 7 names: agy, claude, codex, cursor,
  opencode, pi, qwen-code.

### 3c. `./bin/stagecoach providers show agy` → contract literals.
go-toml v2 emits **single-quoted literal strings** and OMITS nil pointer fields. Relevant lines:
```
name = 'agy'
list_models_command = ['agy', 'models']
print_flag = ''
model_flag = '--model'
default_model = 'Gemini 3.5 Flash (Low)'
bare_flags = ['--mode', 'plan']
experimental = true
```
- **`print_flag = ''`** — empty (NON-NIL empty; agy reads stdin, no bare -p). Contract's "print = """
  maps to this `print_flag = ''` literal.
- **`model_flag = '--model'`** — contract literal.
- **`bare_flags = ['--mode', 'plan']`** — contract's "--mode plan in bare essentials".
- `experimental = true` — also expected (§12.5.1.1 item 4 open).
- OMITTED (nil, correct): tooled_flags, subcommand, prompt_flag, system_prompt_flag (empty→shown?),
  provider_flag, session_mode, json_field, retry_instruction, env, reasoning_levels.
  NOTE: go-toml v2 OMITS nil `*string`/`*bool` but EMITS non-nil-empty `strPtr("")` as `key = ''`.
  agy has `SystemPromptFlag=strPtr("")`, `ProviderFlag=strPtr("")` → these DO appear as empty
  `system_prompt_flag = ''` / `provider_flag = ''`. They are NOT asserted by the contract.

### 3d. `go test ./internal/provider/... ./internal/config/... ./internal/cmd/...` → all PASS (baseline):
```
ok  github.com/dustin/stagecoach/internal/provider
ok  github.com/dustin/stagecoach/internal/config
ok  github.com/dustin/stagecoach/internal/cmd
```

---

## 4. Supporting facts (sources of truth, all READ-ONLY for this task)

- `internal/provider/builtin.go`: `BuiltinManifests()` returns 7 entries; comment "Seven providers:
  pi, claude, opencode, codex, cursor, agy, qwen-code." No `builtinGemini`.
- `internal/provider/registry.go:11`: `preferredBuiltins = ["pi","opencode","cursor","agy","qwen-code","codex","claude"]` (7).
- `internal/provider/manifest.go`: 22 `toml:` tags. `PrintFlag *string toml:"print_flag"`.
- `internal/cmd/providers.go`: `providers list` = tabwriter NAME/DETECTED/DEFAULT table over
  `reg.List()` (sorted asc by Name); `providers show <name>` = `reg.MarshalTOML(name)` (go-toml v2).
- `providers/` dir: agy, claude, codex, cursor, opencode, pi, qwen-code. **NO gemini.toml** (confirmed).
- `Makefile`: `make build` = `go build -ldflags "-X main.version=dev" -o bin/stagecoach ./cmd/stagecoach`.
- Architecture audit `plan/013_b8a415cc6e79/architecture/docs_drift_audit.md`: the prior residual
  drift (D1–D9) was ALL in `docs/providers.md` + `docs/README.md` — owned by T1/T2. cli.md,
  configuration.md, how-it-works.md, README.md were confirmed CLEAN. This task re-confirms that holds.

## 5. Working-tree / dependency state at T3 start (CONTRACT from prior siblings)
- P2.M3.T1 (docs/providers.md, D1–D7) → COMMITTED.
- P2.M3.T2 (docs/README.md, D8/D9) → COMMITTED (applied in working tree during research; will be
  committed before/at T3 execution).
- P2.M1 (models_test.go `[provider.gemini]` fixture removal, builtin.go gemini removal) → COMMITTED.
- **Expected working-tree state when T3.S1 starts: CLEAN** (all prior siblings committed).
  ⇒ The normal outcome is **zero new edits** (pure verification). If the grep audit reveals a stale
  reference, make the minimal in-place fix; `git diff --name-only` must then list ONLY that file.

## 6. Conclusion / confidence
All three passes pass clean on the baseline. This is a verification gate with a conditional-fix
protocol. One-pass success likelihood is very high: the checks are deterministic, the expected output
is captured verbatim, and the only judgment call (classify each grep hit) is pinned by the rubric +
the table above. **Confidence: 10/10** (no fix needed; if one were needed, the rubric makes it mechanical).
