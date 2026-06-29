# Built-in Manifests Research: pi & claude (P1.M2.T2.S1)

> Scope: the FIRST of three built-in-manifest subtasks (P1.M2.T2). This one lands **pi** and **claude**
> — the two providers in the "explicit tool-disable switch" category (PRD §12.7.1). S2/S3 land the other
> four (gemini/opencode, codex/cursor). Input = S1's `Manifest` type (`internal/provider/manifest.go`,
> COMPLETE) + its unexported `strPtr`/`boolPtr` helpers. Output = `BuiltinManifests() map[string]Manifest`
> consumed by the registry (P1.M2.T3).

---

## 1. Exact field values (PRD §12.3 / §12.4, cross-checked vs `architecture/external_deps.md`)

Both manifests are **FULLY VERIFIED** against live `--help` (external_deps.md, 2026-06-29). The values
below are the literal transcription target for `builtin.go`.

| field (Go)        | toml key            | pi value                                              | claude value                                        |
| ----------------- | ------------------- | ----------------------------------------------------- | --------------------------------------------------- |
| `Name`            | `name`              | `"pi"`                                                | `"claude"`                                          |
| `Detect`          | `detect`            | `"pi"`                                                | `"claude"`                                          |
| `Command`         | `command`           | `"pi"`                                                | `"claude"`                                          |
| `Subcommand`      | `subcommand`        | **nil** (absent in §12.3)                             | **nil** (absent in §12.4)                           |
| `PromptDelivery`  | `prompt_delivery`   | `"stdin"`                                             | `"stdin"`                                           |
| `PromptFlag`      | `prompt_flag`       | **nil** (absent; only used when delivery=="flag")     | **nil**                                             |
| `PrintFlag`       | `print_flag`        | `"-p"`                                                | `"-p"`                                              |
| `ModelFlag`       | `model_flag`        | `"--model"`                                           | `"--model"`                                         |
| `DefaultModel`    | `default_model`     | `"glm-5-turbo"`                                       | `"sonnet"`                                          |
| `SystemPromptFlag`| `system_prompt_flag`| `"--system-prompt"`                                   | `"--system-prompt"`                                 |
| `ProviderFlag`    | `provider_flag`     | `"--provider"`                                        | `""` ← **EXPLICIT empty** (§12.4: `provider_flag = "" # n/a`) |
| `DefaultProvider` | `default_provider`  | `""` ← **EXPLICIT empty** (§12.3: `default_provider = ""`) | **nil** (absent in §12.4)                    |
| `BareFlags`       | `bare_flags`        | 6 tokens (see below)                                  | 5 tokens (see below)                                |
| `Output`          | `output`            | `"raw"`                                               | `"raw"`                                             |
| `JsonField`       | `json_field`        | **nil** (absent; only used when output=="json")       | **nil**                                             |
| `StripCodeFence`  | `strip_code_fence`  | `true`                                                | `true`                                              |
| `RetryInstruction`| `retry_instruction` | **nil** (absent → Resolve fills §12.1 default)        | **nil**                                             |
| `Env`             | `env`               | **nil** (no `[env]` table in §12.3)                   | **nil**                                             |

### pi `BareFlags` (exact order — a slice's element order IS its data)
```
"--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session"
```
external_deps.md §pi confirms every one: `--no-tools/-nt`, `--no-extensions/-ne`, `--no-skills/-ns`,
`--no-prompt-templates/-np`, `--no-context-files/-nc`, `--no-session`. (`-p/--print` reads stdin.)

### claude `BareFlags` (exact order)
```
"--tools", "", "--setting-sources", "", "--no-session-persistence"
```
external_deps.md §claude confirms: `--tools ""` documented as *"Use \"\" to disable all tools"*;
`--setting-sources <sources>` (empty → load none); `--no-session-persistence` (only valid with `-p`).
**Note the two empty-string tokens** (`""` after `--tools` and after `--setting-sources`) — these are
the VALUE arguments to those flags (disable-all / load-none). They MUST be present as empty strings in
the slice, not omitted — a bare `["--tools", "--setting-sources", ...]` would be a different (broken)
command. The slice literally encodes `--tools ""` and `--setting-sources ""`.

---

## 2. The explicit-empty vs absent distinction (the pointer-design payoff, applied to built-ins)

This is the single most important subtlety. The PRD TOML for these two providers mixes **keys written
with an explicit empty value** and **keys omitted entirely**. go-toml/v2 decodes these DIFFERENTLY
(S1's verified FINDING C/D): a present `x = ""` → non-nil `*""`; an absent key → `nil`. The literal
construction in `builtin.go` MUST reproduce this exact nil/non-nil pattern, because:

1. **Faithfulness.** A built-in manifest should be byte-equivalent to decoding the PRD TOML — otherwise
   a user reading `providers show pi` (P1.M4.T1.S3) would see a different manifest than §12.3 shows,
   and a `MergeManifest(builtin, override)` would behave subtly differently than merging onto a decoded
   built-in.
2. **The decode-parity test depends on it.** `TestBuiltinManifests_DecodeParity` embeds the §12.3/§12.4
   TOML, decodes it, and asserts `reflect.DeepEqual(builtin, decoded)`. DeepEqual on `Manifest`
   compares pointer TARGETS and treats nil ≠ non-nil. So a literal `DefaultProvider: strPtr("")` (pi)
   must match the decoded `default_provider = ""` (non-nil `*""`), AND a literal *absent* `DefaultProvider`
   (claude) must match the decoded absent key (nil). Getting this wrong fails the parity test — which is
   the point.

### The explicit-empty map (MUST be non-nil `*""`, NOT nil)
- **pi**: `DefaultProvider` → `strPtr("")` (§12.3 writes `default_provider = ""`).
- **claude**: `ProviderFlag` → `strPtr("")` (§12.4 writes `provider_flag = "" # n/a`).

### The absent map (MUST be nil — do NOT "helpfully" set them)
- **both**: `Subcommand`, `PromptFlag`, `JsonField`, `RetryInstruction`, `Env`.
- **claude only**: `DefaultProvider` (§12.4 omits the key entirely).
- **pi only**: (none extra — pi sets `default_provider` explicitly).

> Rule of thumb: if a key appears in the §12.3/§12.4 TOML block (even as `""`), set it with `strPtr`;
> if it does NOT appear, leave the field nil. `Resolve()` turns nil optionals into `*""`/defaults at
> consume time anyway, so functionally a nil and a non-nil `*""` behave the same AFTER Resolve — but the
> PARITY test and `providers show` faithfulness require matching the TOML's nil/non-nil pattern exactly.

---

## 3. §12.2 render verification — the byte-for-byte commit-pi check

PRD §12.2 is the AUTHORITATIVE rendering algorithm. Reproducing it verbatim:

```
args = [m.subcommand...]
if m.provider_flag and provider != "":   args += [m.provider_flag, provider]
if m.model_flag and model != "":          args += [m.model_flag, model]
if m.system_prompt_flag and sys != "":    args += [m.system_prompt_flag, sys]
args += m.bare_flags
if m.print_flag != "":                    args += [m.print_flag]
# prompt_delivery "stdin" → payload via stdin, nothing appended
cmd = exec.Command(m.command, args...)
```

### pi, rendered with provider="zai", model=""(→default glm-5-turbo), sys="<sys>"
Walking §12.2 with the pi manifest:
- subcommand nil → nothing
- provider_flag="--provider" set AND provider="zai"≠"" → `--provider zai`
- model_flag="--model" set AND model(default)="glm-5-turbo"≠"" → `--model glm-5-turbo`
- system_prompt_flag="--system-prompt" set AND sys="<sys>"≠"" → `--system-prompt <sys>`
- bare_flags → all 6 verbatim
- print_flag="-p"≠"" → `-p`

Result argv:
```
["pi", "--provider", "zai", "--model", "glm-5-turbo", "--system-prompt", "<sys>",
 "--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates",
 "--no-context-files", "--no-session", "-p"]
```
This is **byte-for-byte identical** to PRD §12.3's "Rendered" block AND to the commit-pi invocation
(external_deps.md §pi confirms: *"Rendered command (matching commit-pi byte-for-byte)"*). This is the
explicit work-item requirement ("Verify the rendered command matches commit-pi byte-for-byte") and the
single most important assertion in the test suite.

### claude, rendered with provider=""(n/a), model="sonnet", sys="<sys>"
- subcommand nil → nothing
- provider_flag="" → §12.2 condition `if m.provider_flag and ...` is FALSE → **no provider flag emitted**
  (this is the whole point of `provider_flag = ""` — claude has no sub-provider concept)
- model_flag="--model", model="sonnet" → `--model sonnet`
- system_prompt_flag="--system-prompt", sys="<sys>" → `--system-prompt <sys>`
- bare_flags → `--tools "" --setting-sources "" --no-session-persistence`
- print_flag="-p" → `-p`

Result argv (per §12.2, print_flag LAST):
```
["claude", "--model", "sonnet", "--system-prompt", "<sys>",
 "--tools", "", "--setting-sources", "", "--no-session-persistence", "-p"]
```

---

## 4. The §12.4 illustrative-order discrepancy (documented, NOT a bug)

PRD §12.4's "Rendered" block shows:
```
claude -p --model sonnet --system-prompt "<sys>" --tools "" --setting-sources "" --no-session-persistence
```
i.e. `-p` (print_flag) appears SECOND, right after `claude`. But §12.2's algorithm appends `print_flag`
LAST (after `bare_flags`). **These disagree for claude.** For pi they agree (both have `-p` last).

Resolution: **§12.2 is authoritative** (it is titled "Command rendering algorithm"); the §12.4 rendered
block is a hand-written illustration that happens to place `-p` early. For CLI argument parsers, flag
order is irrelevant (no parser cares whether `-p` comes before or after `--model`), so both invocations
are functionally identical and valid. The claude render test MUST assert against §12.2's output
(`-p` last), NOT §12.4's illustration — and the implementer must NOT "fix" the manifest to force `-p`
early (there is no manifest field that controls inter-field ordering; that is purely the renderer's
P1.M2.T4 concern).

> This is why the **pi** render test is the mandatory byte-for-byte check (§12.2 and §12.3 agree, and
> it is the explicit work-item requirement), while **claude** is verified primarily by decode-parity +
> field assertions (its render order is illustrative-only in the PRD). If a claude render test is
> included, assert §12.2's output and add a comment citing this discrepancy.

---

## 5. Construction approach: literal `strPtr`/`boolPtr` + decode-parity test (NOT runtime TOML decode)

Two approaches were considered:

- **(A) Literal construction** via the same-package `strPtr`/`boolPtr` helpers (S1, already present in
  `manifest.go`). `builtin.go` then needs **ZERO imports** — consistent with S1/S2's "non-test code is
  stdlib-only" discipline.
- **(B) Embed the §12.3/§12.4 TOML and `toml.Unmarshal` at runtime.** Self-documenting (source IS the
  PRD TOML) but adds a go-toml/v2 import to non-test code + a decode-error path (panic-on-malformed).

**Decision: A.** Rationale:
1. Consistency — S1 (`manifest.go`) and S2 (`merge.go`) both keep non-test code import-free; `builtin.go`
   joining them keeps the package's production surface stdlib-only and go.mod provably unchanged.
2. `strPtr`/`boolPtr` is the established in-package construction idiom (used by `Resolve()` and S2's
   `sampleBase()` test helper).
3. The correctness guarantee B provides ("source matches PRD TOML") is recovered by the
   `TestBuiltinManifests_DecodeParity` test: it embeds the verbatim §12.3/§12.4 TOML in the TEST file,
   decodes it, and asserts `reflect.DeepEqual(builtin, decoded)`. So the PRD TOML literally lives in the
   source (in the test), and any literal-transcription error in `builtin.go` is caught. The toml import
   stays test-only, exactly as S1 mandated.

The decode-parity test is therefore the keystone: it is what makes "the built-in matches PRD §12.3/§12.4
exactly" an executable, machine-checked claim rather than a hopeful comment.

---

## 6. Fresh-per-call (no shared mutable state)

`BuiltinManifests()` constructs the manifests **fresh on every call** (via unexported `builtinPi()` /
`builtinClaude()` constructors that use `strPtr`/`boolPtr` + slice literals). Rationale:
- `strPtr` allocates a new pointer each call; a slice literal allocates a new backing array each call.
  So fresh-per-call has **zero shared mutable state** — no caller can corrupt a built-in by mutating a
  returned `BareFlags` element or `Env` entry.
- `MergeManifest` already guarantees it never mutates `base` (S2's aliasing guard), so through the
  normal registry path a package-level `var` would also be safe — but a *direct* caller
  (`m := builtins["pi"]; m.BareFlags[0] = "x"`) would mutate a shared `var`. Fresh-per-call eliminates
  even that risk at negligible cost (registry calls it once at init).
- NO package-level `var` holding the built-ins. The function is the sole construction site.

---

## 7. What this subtask is NOT (scope fences)

- NOT the registry (P1.M2.T3) — `BuiltinManifests()` returns the map; the registry imports it + merges
  user overrides via `MergeManifest` (S2).
- NOT the renderer (P1.M2.T4) — the §12.2 argv-builder exists ONLY inside the pi render test (throwaway
  test scaffolding to prove the manifest data; the real renderer is P1.M2.T4).
- NOT gemini/opencode/codex/cursor — those are S2/S3 of this task. `BuiltinManifests()` returns ONLY
  pi + claude here. (S2/S3 will ADD their constructors to the same map-returning function.)
- NOT the reference `providers/*.toml` files — those ship as human-readable files in P1.M5.T2. The
  built-ins here are COMPILED IN (Go literals), the zero-config default.
- NOT a go.mod change — `builtin.go` has zero imports; `builtin_test.go` uses only `testing` + `reflect`
  (+ `go-toml/v2` for the decode-parity test, already in go.mod, test-only).
