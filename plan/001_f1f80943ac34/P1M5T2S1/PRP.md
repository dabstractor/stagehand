---
name: "P1.M5.T2.S1 — providers/*.toml reference files (6 agents, human-readable with comments)"
description: |

  THIS IS A DOCUMENTATION TASK. Deliver SEVEN new files at repo root: six human-readable TOML reference
  manifests under `providers/` (`pi.toml`, `claude.toml`, `gemini.toml`, `opencode.toml`, `codex.toml`,
  `cursor.toml`) that BYTE-FOR-BYTE mirror the compiled-in built-ins (`internal/provider/builtin.go`) but
  carry explanatory comments (§12.1 field semantics, §12.3–§12.7 delivery/rendering notes, the codex/cursor
  `# TO CONFIRM` items, §12.7.1 tools-disable asymmetry), PLUS one sync-guard test
  `internal/provider/referencefiles_test.go` that decode-parities each file against `BuiltinManifests()`.

  The 6 .toml files are REFERENCE DOCUMENTATION, NOT runtime config: they are NOT loaded by the binary
  (built-ins are compiled into the Go binary in `builtin.go`). They serve as templates a user can study
  and copy into their config to override/define a provider (§12.8) — a header comment in each file
  documents that §12.8 transform.

  CONTRACT (P1.M5.T2.S1, verbatim):
    1. RESEARCH NOTE: "PRD §14 lists providers/ with pi.toml, claude.toml, gemini.toml, opencode.toml,
       codex.toml, cursor.toml. These are human-readable reference files (not loaded at runtime — built-ins
       are compiled in). They serve as templates users can copy into their config to override/define
       providers (§12.8). Include the research discrepancy fixes (codex: no --ask-for-approval, stdin
       delivery; see external_deps.md)."
    2. INPUT: "Built-in manifests from P1.M2.T2.S1-S3."
    3. LOGIC: "Create providers/{pi,claude,gemini,opencode,codex,cursor}.toml files mirroring the compiled-in
       manifests with explanatory comments (what each field does, the delivery/rendering notes from §12.3-12.7).
       Include the TO CONFIRM comments for codex/cursor. These are reference documentation, not runtime config."
    4. OUTPUT: "6 reference TOML files in providers/ that users can study and copy."
    5. DOCS: "[Mode A] These files ARE documentation — each includes field-level comments explaining the
       manifest schema (§12.1) and agent-specific notes."

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - `internal/provider/builtin.go` — the 6 compiled-in manifests (P1.M2.T2). READ ONLY — the SOURCE OF
      TRUTH the files MIRROR. (Note: builtin.go is the mirror target; the NEW test reads it, never edits it.)
    - `internal/provider/manifest.go` / `merge.go` / `registry.go` — Manifest schema, merge, registry.
      READ ONLY (the test CONSUMES `BuiltinManifests()` + `toml.Unmarshal`).
    - `internal/config/*.go` — the config loader. READ ONLY. Do NOT wire `providers/` into it (the files
      are inert docs; the loader reads `.stagecoach.toml`, not a `providers/` dir).
    - `internal/provider/builtin_test.go` — the existing decode-parity suite (`piTOML`…`cursorTOML`
      constants + `TestBuiltinManifests_DecodeParity`). READ ONLY; the new test MIRRORS its pattern in a
      SEPARATE file (do NOT modify builtin_test.go).
    - `README.md` — does not exist yet (P1.M5.T4 owns it). Do NOT create it.

  DELIVERABLE (SEVEN new files, NO production-code edits):
    CREATE providers/pi.toml                       # flat `name="pi"`; mirrors builtinPi; §12.3 notes
    CREATE providers/claude.toml                   # mirrors builtinClaude; §12.4 notes (--tools "" etc.)
    CREATE providers/gemini.toml                   # mirrors builtinGemini; §12.5 notes (stdin revision)
    CREATE providers/opencode.toml                 # mirrors builtinOpenCode; §12.6 notes (bare_flags [])
    CREATE providers/codex.toml                    # mirrors builtinCodex; §12.7 notes (2 revisions + TO CONFIRM)
    CREATE providers/cursor.toml                   # mirrors builtinCursor; §12.7 notes (detect=agent + TO CONFIRM)
    CREATE internal/provider/referencefiles_test.go # package provider; round-trip decode-parity sync-guard

  SUCCESS: `go test ./internal/provider/ -run TestProviderReferenceFiles -v` green (each file decodes ==
  its builtin); `gofmt -l internal/provider/referencefiles_test.go` empty; `go vet ./internal/provider/`
  clean; `make test` green (new test runs, not tagged); `git status` shows ONLY the 7 new files; no
  production-code changes; no new go.mod deps.

---

## Goal

**Feature Goal**: Ship PRD §14's `providers/` directory as **human-readable reference documentation**:
six TOML files that mirror — byte-for-byte, modulo comments — the six compiled-in built-in provider
manifests (pi/claude/gemini/opencode/codex/cursor from `internal/provider/builtin.go`), each annotated
with field-level §12.1 schema explanations, §12.3–§12.7 delivery/rendering notes, the §12.7.1
tools-disable asymmetry framing, and the two `# TO CONFIRM (integration)` items (codex, cursor). These
files are NOT loaded at runtime (built-ins are compiled in); they are templates users study and copy to
override/define a provider (§12.8). A round-trip decode-parity test guarantees the docs never drift from
the compiled-in code.

**Deliverable**: Seven new files, no production-code edits — six reference TOML manifests under
repo-root `providers/` (`{pi,claude,gemini,opencode,codex,cursor}.toml`) plus one Go test
`internal/provider/referencefiles_test.go` (`package provider`) whose `TestProviderReferenceFiles_*`
reads each `providers/<name>.toml`, decodes it with `toml.Unmarshal`, and asserts
`reflect.DeepEqual(decoded, BuiltinManifests()[name])`. The six files are the product; the test is the
sync-guard that proves they mirror the code.

**Success Definition**:
- The six `providers/*.toml` exist, are valid TOML, and each decode-paring test passes against its
  compiled-in manifest (nil-vs-empty semantics faithfully reproduced — see §"Known Gotchas").
- Each file is genuinely self-documenting: a reader who knows nothing of this repo can understand every
  field from the comments alone (§12.1 glossary inline) and knows how to use it as a §12.8 override.
- The codex + cursor files carry the `# TO CONFIRM (integration)` items verbatim.
- `make test` is green (the new test is included — NO build tag); `go vet`/`gofmt` clean.
- `git status` shows ONLY the 7 new files; no production code, no go.mod, no Makefile, no loader changes.

## User Persona

**Target User**: the Stagecoach end user / tinkerer (PRD §7.3 "the multi-agent tinkerer"; §7.2 "the
API-key refusenik") who wants to (a) understand how Stagecoach wraps a given agent CLI, or (b) override a
built-in or add a brand-new provider (§12.8). Secondary: contributors reading the provider system.

**Use Case**: "I want to see EXACTLY what flags Stagecoach sends to `codex` / `claude` / `cursor`, and
copy a template to define my own `[provider.myagent]`." They open `providers/codex.toml`, read the
comments, and either learn the manifest schema or copy the body into their config wrapped in
`[provider.<name>]`.

**Pain Points Addressed**: the compiled-in manifests live in Go source (`builtin.go`) — readable but not
copy-pasteable as a config template, and not discoverable without cloning. Shipped `.toml` files give a
browsable, documented, copy-ready reference at the repo root (and in the release archive).

## Why

- **Realizes PRD §14's `providers/` layout item.** §14 lists exactly these six files; P1.M5.T2 ships them.
- **Makes the provider system legible.** The manifest schema (§12.1), the per-agent delivery/rendering
  choices (§12.3–§12.7), and the tools-disable asymmetry (§12.7.1) are otherwise spread across the PRD
  and Go source. One file per agent concentrates the explanation next to the values.
- **Carries the honesty notes.** The codex discrepancy (no `--ask-for-approval`; stdin; `--ephemeral`)
  and the two `# TO CONFIRM` items (codex stdout; cursor `ask`-wins) are documented where a user will
  actually read them — in the agent's own manifest file — not buried in a research log.
- **Enables §12.8 overrides with zero friction.** A user copies a file's body, wraps it in
  `[provider.<name>]`, edits one field, done. No recompilation (§12.8).
- **The sync-guard test makes the docs trustworthy forever.** Without it, a future builtin change (flag
  rename, new default) silently desyncs the .toml → the docs lie. The decode-parity test fails loudly.

## What

Six flat-schema TOML files (`name = "<agent>"` at top — NOT `[provider.<name>]` tables) and one test.

1. **Format decision (frozen):** flat manifest schema, identical to PRD §12.3–§12.7, to
   `providers show <name>` output (`Registry.MarshalTOML`), and to the `piTOML`…`cursorTOML` decode-parity
   constants in `builtin_test.go`. The `[provider.<name>]` table form is the *config-override* syntax
   (§12.8) and is documented in each file's HEADER COMMENT, not used as the file format. Rationale: the
   contract says "mirror the compiled-in manifests"; the compiled-in `Manifest` struct marshals flat, and
   the decode-parity test (the validation gate) requires flat format.
2. **Content skeleton:** each file's effective (de-commented) TOML MUST equal the corresponding
   `piTOML`/`claudeTOML`/`geminiTOML`/`opencodeTOML`/`codexTOML`/`cursorTOML` constant in
   `builtin_test.go` (lines 16–135) — those are byte-faithful to `builtin.go`, proven by
   `TestBuiltinManifests_DecodeParity`. Comments are ADDED around that skeleton; no TOML key/value line
   is added, removed, or reordered.
3. **Comment layers per file:** (a) header block — what the file is, §12.8 override instructions,
   schema reference; (b) field-level comments — the §12.1 glossary (what each present field does); (c)
   a trailing note on absent fields + their Resolve defaults; (d) agent-specific delivery/rendering notes
   (§12.3–§12.7); (e) §12.7.1 tools-disable category; (f) for codex+cursor, the `# TO CONFIRM` items.
4. **Sync-guard test:** `internal/provider/referencefiles_test.go` (`package provider`) — table-driven
   over the six names, reads each file (repo root via `runtime.Caller`), `toml.Unmarshal`, `reflect.DeepEqual`
   to `BuiltinManifests()[name]`. Mirrors `TestBuiltinManifests_DecodeParity` (string-based) but file-based.

### Success Criteria

- [ ] `providers/{pi,claude,gemini,opencode,codex,cursor}.toml` all exist at repo root, valid TOML.
- [ ] Each file's header comment states it is reference documentation (not runtime-loaded) + the §12.8
      override recipe (wrap in `[provider.<name>]`, delete the `name` line).
- [ ] Every PRESENT field has a field-level comment (§12.1 glossary); absent fields are explained in a
      trailing note with their Resolve defaults.
- [ ] The nil-vs-empty pattern matches `builtin.go` EXACTLY (see the fidelity table in §"Known Gotchas"):
      pi `default_provider = ""` present; claude bare_flags keeps the two `""` value tokens; gemini/codex
      `prompt_delivery = "stdin"`; opencode `bare_flags = []` + cursor `subcommand = []` written as `[]`;
      cursor `detect`/`command = "agent"`.
- [ ] codex + cursor files carry the `# TO CONFIRM (integration)` items (codex stdout-on-success; cursor
      ask-wins-over-`-p`).
- [ ] `go test ./internal/provider/ -run TestProviderReferenceFiles -v` green.
- [ ] `make test` green (new test runs — no build tag); `gofmt -l`/`go vet ./internal/provider/` clean.
- [ ] `git status --short` shows ONLY the 7 new files; no production-code edits; no go.mod change.

## All Needed Context

### Context Completeness Check

_Pass._ An author who has never seen this repo can implement this from: the canonical content skeletons
(the `piTOML`…`cursorTOML` constants, reproduced verbatim in §"Implementation Blueprint"); the §12.1
field glossary (reproduced inline); the nil-vs-empty fidelity table (§"Known Gotchas"); a fully-worked
`pi.toml` example (§"Implementation Patterns"); the per-provider delivery/rendering notes (§"Implementation
Tasks"); the exact decode-parity test code (§"Data models"); and the validation commands. The "mirror the
compiled-in manifest" requirement is made deterministic by the sync-guard test.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/001_f1f80943ac34/P1M5T2S1/research/findings.md
  why: THE decisive doc. §2 the flat-format decision; §3 the per-manifest nil-vs-empty fidelity TABLE
       (the high-risk cells); §4 the round-trip decode-parity test design; §5 confirmations (no runtime
       loading of providers/; providers show is flat); §6 the two TO CONFIRM items; §7 scope guard.
  critical: §3 (fidelity table — THE error source), §4 (test design), §2 (flat NOT [provider.x]).

- file: internal/provider/builtin.go   (P1.M2.T2 — READ only; THE source of truth the files MIRROR)
  section: BuiltinManifests() + builtinPi/builtinClaude/builtinGemini/builtinOpenCode/builtinCodex/
           builtinCursor. Each constructor's doc comment records WHY each field is nil-vs-empty and the
           §12.x section + any revision (e.g. codex "REVISED #1/#2", cursor "detect≠name").
  why: this is the exact struct the .toml must decode to. Mine it for the per-field explanations you
       paraphrase into the file comments.
  pattern: the doc comments on each constructor ARE the authoritative field rationale — reuse them.
  gotcha: builtinCodex has PromptDelivery="stdin" + BareFlags=[--sandbox,read-only,--ephemeral] (NOT the
          PRD §12.7 positional/--ask-for-approval); builtinCursor has Detect/Command="agent", Subcommand=[].
          The .toml MUST match the CONSTRUCTOR (with revisions), not the raw PRD §12.7 prose.

- file: internal/provider/builtin_test.go   (P1.M2.T2 — READ only; lines 16–135 = canonical content)
  section: the const piTOML/claudeTOML/geminiTOML/opencodeTOML/codexTOML/cursorTOML blocks (the
           byte-faithful TOML proven == each constructor via TestBuiltinManifests_DecodeParity) +
           TestBuiltinManifests_DecodeParity (the pattern the new test mirrors).
  why: THESE CONSTANTS ARE THE CONTENT SKELETON. Each providers/<name>.toml's de-commented body must
       equal its constant verbatim. Copy the constant, then add comments; do not touch a key/value line.
  pattern: TestBuiltinManifests_DecodeParity does `toml.Unmarshal([]byte(tc.toml), &decoded)` then
           `reflect.DeepEqual(tc.got, decoded)` — the new file-based test does the same, reading from disk.
  gotcha: note HOW the constants encode empty-vs-absent (e.g. pi `default_provider = ""` is a LINE;
          claude has no default_provider line at all). Reproduce EXACTLY.

- file: internal/provider/manifest.go   (P1.M2.T1 — READ only; the §12.1 schema + Resolve defaults)
  section: the Manifest struct field doc comments (the §12.1 glossary source) + the Default* constants
           (DefaultPromptDelivery="stdin", DefaultOutput="raw", DefaultStripCodeFence=true,
           DefaultRetryInstruction="Output ONLY the commit message. No preamble, no markdown, no quotes.")
           + Resolve() (fills nil optionals to these defaults).
  why: the §12.1 field glossary you write into the comments is paraphrased from these struct doc comments;
       the "absent fields take these Resolve defaults" trailing note uses the Default* constants verbatim.
  gotcha: a nil PromptDelivery resolves to "stdin"; a nil Output to "raw"; a nil StripCodeFence to true;
          a nil RetryInstruction to the DefaultRetryInstruction string; nil print_flag/model_flag/etc → "".
          The trailing "absent fields" note must state these EXACT defaults.

- docfile: plan/001_f1f80943ac34/architecture/external_deps.md   (the verified --help captures)
  why: §pi/§claude/§gemini/§opencode/§codex/§cursor give the live flag verifications + the rendered
       command + the codex DISCREPANCY (§codex: --ask-for-approval is NOT a codex exec flag; stdin via
       "-"; --ephemeral) + the cursor TO CONFIRM (ask-wins-over-`-p`). Source the "Rendered" line + the
       agent-specific caveats in each file's comment block.
  critical: §codex DISCREPANCY + BONUS (stdin via "-") — the rationale for codex's two revisions.

- docfile: plan/001_f1f80943ac34/P1M2T1S1/research/go-toml-pointer-behavior.md   (nil-vs-empty decode)
  why: FINDING C (absent key → nil) + FINDING D (present `key = ""`/`key = []` → NON-NIL empty) — the
       REASON the files must reproduce the exact present/absent pattern. Cite it in the fidelity gotcha.

- url: (PRD internal) PRD.md §12.1 (h3.37 schema), §12.3–§12.7 (per-provider), §12.7.1 (tools-disable
       asymmetry), §12.8 (user-defined override syntax), §14 (package layout — providers/ at root),
       Appendix D (h2.27 quick reference table).
  why: AUTHORITATIVE spec for schema + per-agent notes + the §12.8 override recipe in the header comment.

- file: internal/provider/registry.go   (P1.M2.T3 — READ only; confirms the flat-output seam)
  section: MarshalTOML(name) marshals the flat Manifest struct (what `providers show` prints). Confirms
           the flat reference format is consistent with the CLI's own output.
  why: reassures that flat `name = "<x>"` is the right file format (matches `providers show`).
```

### Current Codebase tree (relevant slice)

```bash
internal/provider/
  builtin.go                 # P1.M2.T2 — the 6 compiled-in manifests (THE mirror source). READ only.
  builtin_test.go            # P1.M2.T2 — piTOML…cursorTOML constants (lines 16-135) + DecodeParity. READ only.
  manifest.go                # P1.M2.T1 — Manifest struct + §12.1 glossary + Resolve defaults. READ only.
  registry.go                # P1.M2.T3 — MarshalTOML (flat output). READ only.
  referencefiles_test.go     # ← NEW (this task): round-trip decode-parity sync-guard (package provider).
providers/                   # ← NEW (this task): does NOT exist yet
  pi.toml                    # ← NEW
  claude.toml                # ← NEW
  gemini.toml                # ← NEW
  opencode.toml              # ← NEW
  codex.toml                 # ← NEW
  cursor.toml                # ← NEW
config loader (internal/config/*.go)   # reads .stagecoach.toml, NOT providers/. UNCHANGED.
```

### Desired Codebase tree with files to be added

```bash
providers/pi.toml            # NEW — flat name="pi"; mirrors builtinPi; §12.3 notes; explicit tool-disable.
providers/claude.toml        # NEW — mirrors builtinClaude; §12.4 notes (--tools "" / --setting-sources "").
providers/gemini.toml        # NEW — mirrors builtinGemini; §12.5 notes (stdin revision; --approval-mode).
providers/opencode.toml      # NEW — mirrors builtinOpenCode; §12.6 notes (subcommand=["run"]; bare_flags=[]).
providers/codex.toml         # NEW — mirrors builtinCodex; §12.7 notes (2 revisions) + TO CONFIRM.
providers/cursor.toml        # NEW — mirrors builtinCursor; §12.7 notes (detect=agent) + TO CONFIRM.
internal/provider/referencefiles_test.go   # NEW — package provider; decode-parity over the 6 files.
# ALL other files UNCHANGED. No production-code edits, no go.mod change, no Makefile change, no loader change.
```

### Known Gotchas of our codebase & Library Quirks

```toml
# CRITICAL (FLAT FORMAT, NOT [provider.<name>]): each file uses top-level `name = "<agent>"` — the SAME
# schema as PRD §12.3–§12.7 and `providers show`. Do NOT write `[provider.pi]` / `name = ...` inside a
# table — that is the §12.8 CONFIG-OVERRIDE syntax (what .stagecoach.toml uses), not the manifest format.
# The decode-parity test `toml.Unmarshal(data, &Manifest{})` decodes a FLAT doc into one Manifest; a
# `[provider.pi]` table would decode into a map, not a Manifest → test fails. The §12.8 recipe goes in a
# HEADER COMMENT, not the file body.

# CRITICAL (NIL vs PRESENT-EMPTY — THE fidelity gotcha): go-toml/v2 decodes an ABSENT key to nil, but a
# PRESENT `key = ""` (or `key = []`) to a NON-NIL empty pointer/slice (go-toml-pointer-behavior FINDING
# C/D). The built-in constructors deliberately use BOTH (e.g. pi DefaultProvider is strPtr("") = NON-NIL
# empty; claude DefaultProvider is nil = ABSENT). The .toml MUST reproduce the exact same pattern or it
# no longer mirrors the compiled-in manifest and the decode-parity test FAILS. The fidelity table below
# is authoritative — "✓ =" means WRITE the line with that value; "—" means DO NOT write the line at all.

#   field            | pi            | claude        | gemini        | opencode          | codex            | cursor
#   -----------------+---------------+---------------+---------------+-------------------+------------------+---------------
#   detect           | "pi"          | "claude"      | "gemini"      | "opencode"        | "codex"          | "agent" (!= name)
#   command          | "pi"          | "claude"      | "gemini"      | "opencode"        | "codex"          | "agent"
#   subcommand       | —             | —             | —             | ["run"]           | ["exec"]         | [] (NON-NIL empty)
#   prompt_delivery  | "stdin"       | "stdin"       | "stdin" REV   | "positional"      | "stdin" REV#1    | "positional"
#   prompt_flag      | —             | —             | —             | —                 | —                | —
#   print_flag       | "-p"          | "-p"          | ""            | ""                | ""               | "-p"
#   model_flag       | "--model"     | "--model"     | "-m"          | "-m"              | "-m"             | "--model"
#   default_model    | "glm-5-turbo" | "sonnet"      | "gemini-2.5-pro"| ""              | ""               | ""
#   system_prompt_flag| "--system-prompt" | "--system-prompt" | "" | ""              | ""               | ""
#   provider_flag    | "--provider"  | ""            | ""            | ""                | ""               | ""
#   default_provider | "" (NON-NIL)  | —             | —             | —                 | —                | —
#   bare_flags       | [6 flags]     | [--tools,"",--setting-sources,"",--no-session-persistence] | [--approval-mode,default] | [] (NON-NIL empty) | [--sandbox,read-only,--ephemeral] REV#2 | [--mode,ask,--trust]
#   output           | "raw"         | "raw"         | "raw"         | "raw"             | "raw"            | "raw"
#   json_field       | —             | —             | —             | —                 | —                | —
#   strip_code_fence | true          | true          | true          | true              | true             | true
#   retry_instruction| —             | —             | —             | —                 | —                | —
#   env              | —             | —             | —             | —                 | —                | —
#
# HIGH-RISK cells (a careless copy fails the mirror — verify each against builtin.go):
#   - pi default_provider = ""  -> NON-NIL empty (WRITE the line; do NOT omit).
#   - claude bare_flags         -> the TWO "" value tokens (--tools "" / --setting-sources "") MUST appear.
#   - gemini/codex prompt_delivery = "stdin" -> REVISED from the PRD's "positional" (match the constructor).
#   - opencode bare_flags = []  -> WRITE `bare_flags = []` (present-empty array -> NON-NIL empty). Omitting -> nil -> FAIL.
#   - cursor subcommand = []    -> WRITE `subcommand = []` (present-empty -> NON-NIL empty). Omitting -> nil -> FAIL.
#   - codex bare_flags          -> [--sandbox, read-only, --ephemeral] (REVISED; NOT the PRD's --ask-for-approval).
#   - cursor detect/command     -> "agent" (the binary), NOT "cursor" (the name). The ONLY provider where detect != name.

# CRITICAL (THE CONTENT SKELETON IS FIXED): each file's de-commented body MUST equal the corresponding
# piTOML/claudeTOML/geminiTOML/opencodeTOML/codexTOML/cursorTOML constant in builtin_test.go (lines
# 16-135) VERBATIM — those are proven == the constructors by TestBuiltinManifests_DecodeParity. Add
# comments AROUND that skeleton; do NOT add/remove/reorder any TOML key/value line. The simplest correct
# method: paste the constant's body, then insert comment lines. Comments are stripped on decode, so a
# correctly-commented file decodes to the SAME struct as the bare constant.

# GOTCHA (COMMENTS DO NOT AFFECT DECODE): inline comments after a value (e.g.
#   prompt_delivery = "stdin"   # REVISED ...
# ) and full-line comments (# ...) are both stripped by the TOML parser. The decode-parity test passes
# regardless of how much you comment, AS LONG AS the key/value lines are untouched. Comment freely.

# GOTCHA (absent fields ≠ forgotten fields): for fields that are nil in the builtin (prompt_flag,
# json_field, default_provider on claude/gemini/..., retry_instruction, env), do NOT invent a value —
# OMIT them from the file and explain in a trailing note that they are absent and take their §12.1
# Resolve defaults at runtime (prompt_delivery->"stdin", output->"raw", strip_code_fence->true,
# retry_instruction->"Output ONLY the commit message. No preamble, no markdown, no quotes.",
# print_flag/model_flag/etc -> "" i.e. no flag emitted).

# GOTCHA (THE FILES ARE INERT — do NOT wire them in): the config loader reads .stagecoach.toml, NOT a
# providers/ directory (grep-verified: no embed/ReadDir/Glob of providers/ in internal/config). Do NOT
# add //go:embed, do NOT add a loader path, do NOT touch the registry. The 6 files are pure docs. The
# ONLY code this task adds is the sync-guard test, which READS the files (never loads them as config).

# GOTCHA (NO build tag on the test): unlike P1.M5.T1.S2's integration_real suite, this test runs in CI
# (`make test`). It is a fast, deterministic, offline decode-parity check — do NOT gate it behind a tag.
```

## Implementation Blueprint

### Data models and structure

No production data models. The task adds one test file whose only "model" is the table of providers:

```go
// internal/provider/referencefiles_test.go  (package provider)
// providerFiles — the 6 shipped reference manifests (PRD §14), each decode-parity-checked against its
// compiled-in built-in. repoPath is relative to the repo root (providers/<name>.toml).
var providerFiles = []struct {
	name     string
	repoPath string
}{
	{"pi", "providers/pi.toml"},
	{"claude", "providers/claude.toml"},
	{"gemini", "providers/gemini.toml"},
	{"opencode", "providers/opencode.toml"},
	{"codex", "providers/codex.toml"},
	{"cursor", "providers/cursor.toml"},
}
```

### §12.1 field glossary (reusable comment source — paraphrase into each file's field comments)

These are the field explanations to attach to each PRESENT field (paraphrased from `manifest.go`'s
struct doc comments + PRD §12.1). Absent fields are explained once in the trailing note.

```text
name              = identity / the value you pass to --provider. (In a config override, the [provider.<name>]
                    table key supplies this — the body carries no `name` line; see header comment.)
detect            = command looked up on $PATH to decide if the provider is "installed". Absent -> falls
                    back to `command`. (cursor: the binary is `agent`, so detect="agent" != name "cursor".)
command           = the executable to run. Resolved via exec.LookPath; may be an absolute path.
subcommand        = tokens inserted between command and flags (opencode ["run"], codex ["exec"]).
                    An empty array [] (cursor) means none — but is intentionally present (NON-NIL empty).
prompt_delivery   = how the user payload (system-built prompt + diff) reaches the agent:
                      "stdin" (DEFAULT; avoids arg-length limits) | "positional" (final positional arg) |
                      "flag" (after prompt_flag).
prompt_flag       = used ONLY when prompt_delivery = "flag". (Absent for all six built-ins.)
print_flag        = token(s) that put the agent into non-interactive "print and exit" mode. Empty -> none.
model_flag        = the model-selection flag (e.g. --model, -m).
default_model     = model used if the user specifies none. Empty -> the user MUST set a model (opencode,
                    codex, cursor — whose model spaces are huge / config/account-driven).
system_prompt_flag= if the agent supports a system prompt. Empty -> the system prompt is PREPENDED to the
                    user payload (the §12.2 fallback; used by gemini/opencode/codex/cursor).
provider_flag     = sub-provider selection flag (pi has --provider zai|anthropic|google|...). Empty -> none.
default_provider  = default sub-provider (e.g. "zai" for pi). pi sets it explicitly to "" (NON-NIL empty):
                    "do NOT add --provider unless the user configures one."
bare_flags        = flags appended VERBATIM to make the call tool-less / session-less / extension-less /
                    chrome-less / ephemeral. These are agent-specific — see §12.7.1 (some agents have an
                    explicit tool-disable switch; others use a read-only constraint instead).
output            = "raw" (cleaned stdout IS the message — DEFAULT) | "json" (extract json_field).
json_field        = the field to extract when output = "json". (Absent for all six built-ins -> raw.)
strip_code_fence  = strip a single layer of ``` or ~~~ code fence from the output if present.
retry_instruction = instruction prepended on a parse-retry (empty/invalid output). (Absent for all six ->
                    the §12.1 default: "Output ONLY the commit message. No preamble, no markdown, no quotes.")
[env]             = extra env vars set ONLY for the agent subprocess (never global). (Absent for all six.)
```

### Canonical content skeletons (VERBATIM from builtin_test.go lines 16–135 — the decode-parity oracles)

Each `providers/<name>.toml`'s **de-commented body** must equal its skeleton below EXACTLY. Paste the
skeleton, then add comments. (Whitespace/capitalization of VALUES must match; comment placement is free.)

```toml
# === pi === (mirrors builtinPi; §12.3)
name = "pi"
detect = "pi"
command = "pi"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "--model"
default_model = "glm-5-turbo"
system_prompt_flag = "--system-prompt"
provider_flag = "--provider"
default_provider = ""
bare_flags = [
  "--no-tools",
  "--no-extensions",
  "--no-skills",
  "--no-prompt-templates",
  "--no-context-files",
  "--no-session",
]
output = "raw"
strip_code_fence = true

# === claude === (mirrors builtinClaude; §12.4)  -- note the TWO "" value tokens in bare_flags
name = "claude"
detect = "claude"
command = "claude"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "--model"
default_model = "sonnet"
system_prompt_flag = "--system-prompt"
provider_flag = ""
bare_flags = [
  "--tools", "",
  "--setting-sources", "",
  "--no-session-persistence",
]
output = "raw"
strip_code_fence = true

# === gemini === (mirrors builtinGemini; §12.5; prompt_delivery REVISED to "stdin")
name = "gemini"
detect = "gemini"
command = "gemini"
prompt_delivery = "stdin"
print_flag = ""
model_flag = "-m"
default_model = "gemini-2.5-pro"
system_prompt_flag = ""
provider_flag = ""
bare_flags = [
  "--approval-mode", "default",
]
output = "raw"
strip_code_fence = true

# === opencode === (mirrors builtinOpenCode; §12.6; bare_flags = [] is intentionally NON-NIL empty)
name = "opencode"
detect = "opencode"
command = "opencode"
subcommand = ["run"]
prompt_delivery = "positional"
print_flag = ""
model_flag = "-m"
default_model = ""
system_prompt_flag = ""
provider_flag = ""
bare_flags = []
output = "raw"
strip_code_fence = true

# === codex === (mirrors builtinCodex; §12.7; prompt_delivery="stdin" REV#1, bare_flags REV#2)
name = "codex"
detect = "codex"
command = "codex"
subcommand = ["exec"]
prompt_delivery = "stdin"
print_flag = ""
model_flag = "-m"
default_model = ""
system_prompt_flag = ""
provider_flag = ""
bare_flags = ["--sandbox", "read-only", "--ephemeral"]
output = "raw"
strip_code_fence = true

# === cursor === (mirrors builtinCursor; §12.7; detect/command="agent" (!= name); subcommand=[] NON-NIL empty)
name = "cursor"
detect = "agent"
command = "agent"
subcommand = []
prompt_delivery = "positional"
print_flag = "-p"
model_flag = "--model"
default_model = ""
system_prompt_flag = ""
provider_flag = ""
bare_flags = ["--mode", "ask", "--trust"]
output = "raw"
strip_code_fence = true
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: VERIFY the baseline + mine the source of truth (READ + RUN, no edit)
  - RUN: `go test ./internal/provider/ -run TestBuiltinManifests_DecodeParity -v` -> green (proves the
      piTOML..cursorTOML constants == the constructors; these constants ARE your content skeletons).
  - READ: internal/provider/builtin.go -> the 6 constructors + their doc comments (per-field rationale,
      the revisions, the TO CONFIRM notes). Paraphrase these into the file comments.
  - READ: internal/provider/builtin_test.go lines 16-135 -> copy the 6 const blocks as your skeletons.
  - READ: internal/provider/manifest.go -> the Manifest struct doc comments + Default* constants (your
      §12.1 glossary + the "absent fields -> Resolve defaults" trailing note).
  - READ: plan/.../architecture/external_deps.md -> the per-provider rendered command + caveats.
  - GOTCHA: confirm `providers/` does NOT exist yet (`ls providers/` -> no such dir). It is created by
      writing the first file.

Task 1: CREATE providers/pi.toml (the exemplar — get this one RIGHT, then mirror its structure)
  - SKELETON: the "=== pi ===" block above (verbatim).
  - HEADER COMMENT block (top of file), covering:
      * WHAT: "Reference manifest for the `pi` built-in provider (PRD §12.3). Human-readable documentation
        — NOT loaded at runtime (built-ins are compiled into the binary in internal/provider/builtin.go).
        This file mirrors builtinPi() byte-for-byte (modulo comments)."
      * HOW TO USE AS OVERRIDE (§12.8): "To override the built-in or define a variant, copy the FIELD
        lines below (NOT this header) into your config (~/.config/stagecoach/config.toml or a repo-local
        .stagecoach.toml) wrapped in a `[provider.<name>]` table, and DELETE the `name = ...` line (the
        table key supplies the name). Change only the fields you want; absent fields inherit the built-in."
      * RENDERED (external_deps.md §pi): show `pi --provider zai --model glm-5-turbo --system-prompt "<sys>"
        --no-tools --no-extensions --no-skills --no-prompt-templates --no-context-files --no-session -p
        < <user payload via stdin>` and note "byte-for-byte the commit-pi invocation."
  - FIELD-LEVEL COMMENTS: prepend/inline the §12.1 glossary entry for EACH present field (name, detect,
      command, prompt_delivery, print_flag, model_flag, default_model, system_prompt_flag, provider_flag,
      default_provider, each bare_flags entry, output, strip_code_fence).
      * default_provider = ""  -> comment "NON-NIL empty: do NOT add --provider unless the user configures
        one (e.g. set 'zai' for GLM). pi's own default provider is 'google'."
      * bare_flags -> comment "§12.7.1 EXPLICIT tool-disable switch: a pure text-in/text-out call with no
        agent loop (the 'bare' ideal). Verified vs `pi --help`."
  - TRAILING NOTE: "Absent fields (subcommand, prompt_flag, json_field, retry_instruction, [env]) take
      their §12.1 defaults at resolve time: prompt_delivery->stdin, output->raw, strip_code_fence->true,
      retry_instruction->'Output ONLY the commit message. No preamble, no markdown, no quotes.'"
  - VERIFY: `go test ./internal/provider/ -run TestProviderReferenceFiles -run pi -v` will not exist YET
      (test is Task 7); instead eyeball vs the "=== pi ===" skeleton.

Task 2: CREATE providers/claude.toml
  - SKELETON: the "=== claude ===" block (verbatim — INCLUDING the two "" value tokens in bare_flags).
  - HEADER: same structure as pi; cite §12.4. RENDERED (external_deps.md §claude): `claude -p --model sonnet
      --system-prompt "<sys>" --tools "" --setting-sources "" --no-session-persistence < <user payload>`.
  - FIELD NOTES:
      * provider_flag = ""  -> "NON-NIL empty: claude has no sub-provider concept (§12.4 'n/a')."
      * bare_flags:
          - `--tools ""`        -> "disable ALL built-in tools (claude --help: 'Use \"\" to disable all
            tools'). §12.7.1 EXPLICIT tool-disable switch."
          - `--setting-sources ""` -> "load no settings sources (clean slate)."
          - `--no-session-persistence` -> "ephemeral (only valid with -p)."
      * note: "--system-prompt REPLACES the default; --append-system-prompt is the additive alternative
        (a user who wants CC's default persona retained can switch the flag). output=json +
        json_field='result' is an alternative if raw proves unreliable."
  - TRAILING NOTE: absent fields incl. default_provider take §12.1 Resolve defaults.

Task 3: CREATE providers/gemini.toml
  - SKELETON: the "=== gemini ===" block (verbatim — prompt_delivery="stdin" is the REVISION).
  - HEADER: cite §12.5. RENDERED: `gemini -m gemini-2.5-pro --approval-mode default < "<sys>\n\n<user
      payload>"` (stdin).
  - FIELD NOTES:
      * prompt_delivery = "stdin" -> "REVISED from §12.5 'positional' (work-item + external_deps.md §gemini
        + Appendix E #1): gemini appends stdin to the prompt, and stdin avoids arg-length limits on ~300 KB
        diffs. `-p/--prompt` is DEPRECATED — do not use it."
      * print_flag = "" -> "NON-NIL empty: positional/stdin implies one-shot (no separate print flag)."
      * system_prompt_flag = "" -> "NON-NIL empty: gemini-cli has NO system-prompt flag -> the system prompt
        is PREPENDED to the payload (§12.2 fallback)."
      * bare_flags `--approval-mode default` -> "§12.7.1 READ-ONLY CONSTRAINT (gemini has NO global
        tool-disable switch): 'default' = don't auto-run tools, never ask. Choices: default|auto_edit|yolo."
  - TRAILING NOTE: same pattern (absent -> §12.1 defaults).

Task 4: CREATE providers/opencode.toml
  - SKELETON: the "=== opencode ===" block (verbatim — subcommand=["run"], bare_flags=[] NON-NIL empty).
  - HEADER: cite §12.6. RENDERED: `opencode run -m anthropic/claude-sonnet-4 "<sys>\n\n<user payload>"`.
  - FIELD NOTES:
      * subcommand = ["run"] -> "`opencode run` is non-interactive and prints the final message to stdout."
      * default_model = "" -> "NON-NIL empty: opencode's model space is huge and user-specific (format
        provider/model, e.g. anthropic/claude-sonnet-4) -> the user MUST set a model (Appendix E #3)."
      * system_prompt_flag = "" -> "no sys-prompt flag on `run` -> system prompt PREPENDED (§12.2)."
      * bare_flags = [] -> "NON-NIL empty array: `run` is already a read-only, non-interactive one-shot
        (§12.7.1 read-only constraint) — no extra bare flags needed. (Present-but-empty, NOT absent.)"
      * note: "--agent <name> exists for finer persona control against a user opencode.json (future)."
  - TRAILING NOTE.

Task 5: CREATE providers/codex.toml  (the discrepancy-resolution file)
  - SKELETON: the "=== codex ===" block (verbatim — prompt_delivery="stdin" REV#1, bare_flags REV#2).
  - HEADER: cite §12.7 + external_deps.md §codex (the DISCREPANCY). RENDERED: `codex exec -m <model>
      --sandbox read-only --ephemeral < "<sys>\n\n<user payload>"` (stdin via "-").
  - FIELD NOTES:
      * subcommand = ["exec"] -> "`codex exec` (alias `e`) is the non-interactive runner."
      * prompt_delivery = "stdin" -> "REV#1 from §12.7 'positional': `codex exec --help` says 'If not
        provided (or if - is used), instructions are read from stdin.' stdin avoids arg-length limits on
        large diffs (external_deps.md §codex BONUS)."
      * system_prompt_flag = "" -> "NO system-prompt flag on `codex exec` -> system prompt PREPENDED (§12.2)."
      * bare_flags:
          - `--sandbox read-only` -> "sandbox forbids writes/network mutations (read-only, never-mutate)."
          - `--ephemeral` -> "REV#2: run without persisting session files (replaces §12.7's
            `--ask-for-approval never`, which is NOT a `codex exec` flag — it lives on interactive `codex`;
            `codex exec` is already non-interactive and never blocks on approval)."
        -> "§12.7.1 READ-ONLY CONSTRAINT: codex has NO global tool-disable switch; this is the safe,
          non-interactive, non-mutating profile."
      * TO CONFIRM comment (REQUIRED by contract): "# TO CONFIRM (integration): that `codex exec` writes
        the assistant's final answer to stdout and exits 0 on success. Expected; fallbacks: -o <file>
        (write last message to file), --json (JSONL events). Resolved by the real-agent suite (P1.M5.T1.S2)."
  - TRAILING NOTE.

Task 6: CREATE providers/cursor.toml  (the detect!=name file)
  - SKELETON: the "=== cursor ===" block (verbatim — detect/command="agent", subcommand=[] NON-NIL empty).
  - HEADER: cite §12.7. RENDERED: `agent -p --mode ask --trust --model <model> "<sys>\n\n<user payload>"`.
  - FIELD NOTES:
      * detect = "agent" / command = "agent" -> "the standalone Cursor Agent binary is `agent` (NOT
        `cursor`) — the ONLY provider where detect != name. NOTE: some installs expose this as
        `cursor agent`; if `agent` is not on $PATH, override with command='cursor' subcommand=['agent']."
      * subcommand = [] -> "NON-NIL empty array: no subcommand tokens (present-but-empty, NOT absent)."
      * default_model = "" -> "NON-NIL empty: cursor has per-account model availability -> the user sets one."
      * system_prompt_flag = "" -> "NO system-prompt flag -> system prompt PREPENDED (§12.2)."
      * bare_flags:
          - `--mode ask` -> "Q&A style, READ-ONLY (no edits) — overrides -p's default FULL-tools profile
            (§12.7.1 read-only constraint)."
          - `--trust` -> "skip the workspace-trust prompt that would otherwise block -p."
        -> "Deliberately does NOT set --force / --yolo (those force-allow commands)."
      * TO CONFIRM comment (REQUIRED by contract): "# TO CONFIRM (integration): that `--mode ask` wins over
        -p's default full-tools profile — i.e. the combo (-p --mode ask --trust) is genuinely read-only.
        Expected ('ask' = read-only Q&A); verify against a real run. Resolved by P1.M5.T1.S2."
  - TRAILING NOTE.

Task 7: CREATE internal/provider/referencefiles_test.go (the sync-guard)
  - FILE: package provider. Imports: os, path/filepath, reflect, runtime, testing, go-toml/v2.
  - repoRoot() helper: `_, file, _, _ := runtime.Caller(0); return filepath.Join(filepath.Dir(file), "..", "..")`
      (file = .../internal/provider/referencefiles_test.go -> repo root = ../../). Bulletproof.
  - TestProviderReferenceFiles_DecodeParity: table over providerFiles (Task "Data models"); for each:
        data, err := os.ReadFile(filepath.Join(repoRoot(), tc.repoPath))  // fail fatalf on ReadFile err
        var decoded Manifest
        toml.Unmarshal(data, &decoded)                                  // fail fatalf on decode err
        want := BuiltinManifests()[tc.name]
        reflect.DeepEqual(decoded, want)                                // fail Errorf with %+v diff
  - TestProviderReferenceFiles_AllBuiltinsCovered: assert BuiltinManifests() keys == the providerFiles
        names (so a 7th builtin added later without a .toml is caught). (Optional but recommended.)
  - WHY: this is the ONLY deterministic proof the docs mirror the code; fails loudly on drift.
  - GOTCHA: BuiltinManifests() is EXPORTED -> compiles in package provider (same as builtin_test.go).
      Do NOT add a build tag (runs in CI / make test). runtime.Caller path makes it invocation-independent.

Task 8: FINAL VALIDATION
  - RUN: `go test ./internal/provider/ -run TestProviderReferenceFiles -v` -> green (all 6 decode == builtin).
  - RUN: `go test ./internal/provider/ -v` -> green (new test coexists with the existing suite, no collision).
  - RUN: `gofmt -w internal/provider/referencefiles_test.go`; `gofmt -l internal/provider/` (empty).
  - RUN: `go vet ./internal/provider/` (clean).
  - RUN: `make test` -> green (full suite; the .toml files are inert — not compiled, not loaded).
  - RUN: `git status --short` -> ONLY the 7 new files (providers/*.toml + referencefiles_test.go).
  - SANITY (optional): `go run ./cmd/stagecoach providers show pi` -> prints the flat manifest; eyeball
      that providers/pi.toml's de-commented body matches the CLI output (it must — both derive from builtinPi).
```

### Implementation Patterns & Key Details

```toml
# EXEMPLAR — providers/pi.toml target quality. (Other files mirror this structure; values/notes vary.)
# Reproduce the EXACT comment layers: header block, field comments, trailing absent-fields note.
# The de-commented body of this file MUST equal the "=== pi ===" skeleton in §"Implementation Blueprint".

# ============================================================================
# pi — reference manifest for the `pi` built-in provider (PRD §12.3)
# ============================================================================
#
# WHAT THIS FILE IS
#   Human-readable REFERENCE DOCUMENTATION for the `pi` provider. It mirrors the
#   compiled-in manifest `builtinPi()` in internal/provider/builtin.go BYTE-FOR-BYTE
#   (modulo comments). It is NOT loaded at runtime — built-ins are compiled into
#   the Go binary. (The config loader reads .stagecoach.toml, not this directory.)
#
# HOW TO USE IT AS A CONFIG OVERRIDE (PRD §12.8)
#   To override the built-in pi (or define a variant), copy the FIELD lines below
#   (NOT this header) into your config file and wrap them in a `[provider.pi]`
#   table, then DELETE the `name = "pi"` line — the table key supplies the name.
#   Change only the fields you want; fields you omit inherit the built-in's value.
#       # ~/.config/stagecoach/config.toml  (or a repo-local .stagecoach.toml)
#       [provider.pi]
#       default_model = "glm-5.2"          # e.g. override only the model
#
# RENDERED COMMAND (external_deps.md §pi; matches commit-pi byte-for-byte)
#   pi --provider zai --model glm-5-turbo --system-prompt "<sys>" \
#      --no-tools --no-extensions --no-skills --no-prompt-templates \
#      --no-context-files --no-session -p   < <user payload via stdin>
#
# TOOLS-DISABLE CATEGORY (§12.7.1): EXPLICIT switch — pi offers literal --no-* flags,
#   so the call is a pure text-in/text-out with NO agent loop (the "bare" ideal).
# ============================================================================

# --- identity / discovery ---
name = "pi"                         # identity; the value you pass to --provider.
detect = "pi"                       # command looked up on $PATH to decide "installed". Absent -> command.
command = "pi"                      # the executable to run (exec.LookPath; may be absolute).

# --- prompt delivery ---
prompt_delivery = "stdin"           # pipe the user payload (system prompt + diff) to stdin (DEFAULT;
                                    # avoids arg-length limits vs positional).

# --- non-interactive mode ---
print_flag = "-p"                   # -p / --print: process the prompt and exit (one-shot).

# --- model ---
model_flag = "--model"              # the model-selection flag.
default_model = "glm-5-turbo"       # model used if the user specifies none.

# --- system prompt ---
system_prompt_flag = "--system-prompt"  # pi supports a first-class system prompt (delivered via flag).

# --- sub-provider ---
provider_flag = "--provider"        # pi routes to backends: zai | anthropic | google | ...
default_provider = ""               # NON-NIL empty: do NOT add --provider unless the user configures
                                    # one (e.g. set "zai" for GLM). pi's own default provider is "google".

# --- bare mode (§12.7.1: explicit tool-disable switch) ---
bare_flags = [                      # appended VERBATIM to make the call bare + ephemeral.
  "--no-tools",                     # disable ALL tools (pure text-in/text-out).
  "--no-extensions",                # disable extension discovery.
  "--no-skills",                    # disable skill discovery/loading.
  "--no-prompt-templates",          # disable prompt-template discovery.
  "--no-context-files",             # disable AGENTS.md / CLAUDE.md discovery.
  "--no-session",                   # don't save the session (ephemeral).
]

# --- output ---
output = "raw"                      # cleaned stdout IS the commit message (DEFAULT).
strip_code_fence = true             # strip one layer of ``` / ~~~ if the agent wraps the output.

# --- absent fields (PRD §12.1) ---
# subcommand, prompt_flag, json_field, retry_instruction, and [env] are NOT set for pi and therefore
# omitted above. At resolve time they take their §12.1 defaults: prompt_delivery -> "stdin",
# output -> "raw", strip_code_fence -> true, retry_instruction ->
# "Output ONLY the commit message. No preamble, no markdown, no quotes.",
# and print_flag/model_flag/etc -> "" (no flag emitted).
```

### Integration Points

```yaml
DOCUMENTATION (PRD §14 providers/ layout):
  - the 6 files live at repo root in providers/ (sibling of internal/, cmd/, docs/). They are inert:
    nothing imports, embeds, globs, or loads them. They ship in the release archive as browsable docs.
  - the README (P1.M5.T4, not yet created) will later LINK to providers/ as "see the shipped reference
    manifests". No action here — just don't break the path providers/<name>.toml.

VALIDATION (the sync-guard test):
  - internal/provider/referencefiles_test.go (package provider) runs in `make test` (NO build tag).
    It decode-parities each providers/<name>.toml against BuiltinManifests()[name] via reflect.DeepEqual.
    It is the deterministic guarantee that the docs mirror the code; it fails on any drift.

PRODUCTION CODE (frozen — read-only dependency):
  - the test CONSUMES provider.BuiltinManifests() (P1.M2.T2), provider.Manifest (P1.M2.T1), and
    go-toml/v2 (already a go.mod dep). It modifies NONE of them. The 6 .toml files reference builtin.go
    conceptually but are not parsed by it.

PARALLEL COORDINATION (P1.M5.T1.S2 — real-agent suite, being implemented in parallel):
  - S2's realagent_test.go (//go:build integration_real) RESOLVES the codex/cursor # TO CONFIRM items at
    runtime. THIS task (S1) DOCUMENTS those same items as comments in providers/codex.toml + cursor.toml.
    No conflict: S2 runs real agents (manual); S1 ships static docs. Both carry the TO CONFIRM language
    consistently. Neither edits the other's files (S2 is internal/generate/, S1 is providers/ + a new
    internal/provider/ test).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Validate the .toml files parse (the decode-parity test in Level 2 is the real check; this is a quick
# syntax sanity net using the existing binary if handy):
go test ./internal/provider/ -run TestProviderReferenceFiles -v          # parses + decodes every file

# Go-side style for the test file:
gofmt -w internal/provider/referencefiles_test.go
gofmt -l internal/provider/                                              # must be empty
go vet ./internal/provider/                                              # clean

# Expected: zero errors. If TestProviderReferenceFiles_DecodeParity reports a diff for provider X,
# the .toml does NOT mirror builtinX() — read the diff, fix the .toml's nil-vs-empty pattern (see the
# fidelity table in §"Known Gotchas"), re-run. (Do NOT "fix" it by editing builtin.go — builtin is frozen.)
```

### Level 2: Decode-Parity (the sync-guard — proves the docs mirror the code)

```bash
# THE gate: each providers/<name>.toml decodes to EXACTLY BuiltinManifests()[name].
go test ./internal/provider/ -run TestProviderReferenceFiles -v
# Expected: PASS for all 6 (pi, claude, gemini, opencode, codex, cursor). Any FAIL prints the
# built-in vs decoded structs — align the .toml's present/absent fields to the built-in (not vice-versa).

# Coexistence: the new test must not collide with the existing decode-parity suite.
go test ./internal/provider/ -v
# Expected: green; TestBuiltinManifests_DecodeParity (string-based) AND TestProviderReferenceFiles_*
# (file-based) both pass — they are independent assertions of the same invariant from two sources.

# Coverage check (optional): the new test adds a few covered lines to manifest.go's decode path.
go test ./internal/provider/ -cover
```

### Level 3: Whole-Repo Integration (the .toml files are inert — nothing breaks)

```bash
# Full suite: the 6 .toml files are NOT compiled, NOT embedded, NOT loaded — they cannot break anything.
make test            # == go test -race ./... -> green.

# CLI consistency: `providers show <name>` prints the flat manifest; the .toml's de-commented body
# should match it (both derive from the same constructor). Eyeball one:
go run ./cmd/stagecoach providers show pi
# Expected: a flat TOML block whose keys/values match providers/pi.toml's field lines (modulo the
# single-vs-double quoting cosmetic — go-toml emits single quotes; the .toml uses double; both valid).

# Scope audit: ONLY the 7 new files.
git status --short
# Expected: ?? providers/{pi,claude,gemini,opencode,codex,cursor}.toml
#           ?? internal/provider/referencefiles_test.go
#           (nothing else — no production code, no go.mod, no Makefile, no loader.)
```

### Level 4: Documentation Quality (manual review — the docs are the product)

```bash
# (Manual) Open each providers/<name>.toml and confirm:
#   1. Header states it is reference docs (not runtime-loaded) + the §12.8 override recipe.
#   2. Every PRESENT field has a §12.1-glossary comment (what it does).
#   3. The trailing note explains absent fields + their Resolve defaults.
#   4. codex.toml + cursor.toml carry the # TO CONFIRM (integration) items.
#   5. The §12.7.1 tools-disable category is noted (explicit switch vs read-only constraint).
#   6. cursor's detect/command="agent" (!= name) and the [] empty arrays are called out.
# (Optional) render the file in a TOML-aware viewer to confirm comments read cleanly alongside values.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l internal/provider/` empty; `go vet ./internal/provider/` clean.
- [ ] Level 2: `go test ./internal/provider/ -run TestProviderReferenceFiles -v` green (all 6 decode == builtin).
- [ ] Level 2: `go test ./internal/provider/ -v` green (new test coexists, no collision with existing suite).
- [ ] Level 3: `make test` green (the .toml files are inert — no breakage); `git status` shows ONLY the 7 files.
- [ ] No new `go.mod` dependencies (stdlib + existing go-toml/v2 only).

### Feature Validation

- [ ] `providers/{pi,claude,gemini,opencode,codex,cursor}.toml` all exist, valid TOML, flat `name = "<x>"`.
- [ ] Each file's de-commented body equals its `piTOML`/…/`cursorTOML` skeleton (decode-parity proves it).
- [ ] Header comment: "reference docs, not runtime-loaded" + the §12.8 override recipe + the rendered cmd.
- [ ] Every present field has a §12.1 comment; absent fields explained in the trailing note w/ Resolve defaults.
- [ ] High-risk cells correct: pi `default_provider = ""`; claude keeps the two `""` bare-flag tokens;
      gemini/codex `prompt_delivery = "stdin"`; opencode `bare_flags = []` + cursor `subcommand = []`;
      codex `bare_flags = [--sandbox,read-only,--ephemeral]`; cursor `detect`/`command = "agent"`.
- [ ] codex.toml + cursor.toml carry the `# TO CONFIRM (integration)` items verbatim.
- [ ] §12.7.1 tools-disable category noted per file (explicit switch for pi/claude; read-only constraint for
      gemini/opencode/codex/cursor).

### Code Quality Validation

- [ ] `referencefiles_test.go` is `package provider` (consistent with builtin_test.go); uses `runtime.Caller`
      for invocation-independent path resolution; NO build tag (runs in CI).
- [ ] Test asserts `reflect.DeepEqual(decoded, BuiltinManifests()[name])` — not a subset/loose check.
- [ ] No production-code edits; no go.mod/Makefile/loader changes; no `//go:embed` of providers/.

### Documentation & Deployment

- [ ] The 6 files are self-documenting (a reader needs no other file to understand the manifest).
- [ ] Comments are accurate to `builtin.go`'s constructor doc comments + external_deps.md (no invented facts).
- [ ] The §12.8 override example in each header is correct (wrap in `[provider.<name>]`, delete the `name` line).
- [ ] The files ship in the release archive (they're at repo root — no release-config change needed; the
      goreleaser task P1.M5.T3.S2 must include `providers/` in the archive, which is the default for non-
      Go files — flag this as a note to P1.M5.T3 if it uses a narrow `include` list).

---

## Anti-Patterns to Avoid

- ❌ **Don't use `[provider.<name>]` table format.** That is the config-OVERRIDE syntax (§12.8), not the
  manifest format. The files mirror the compiled-in `Manifest` struct, which is flat (`name = "<x>"`).
  A table would fail the decode-parity test AND not match `providers show` output.
- ❌ **Don't conflate nil (absent) with `""`/`[]` (present-empty).** go-toml distinguishes them (FINDING
  C/D). The built-ins use BOTH deliberately. Reproduce the exact pattern from the fidelity table; do not
  "tidy" an empty value by omitting it (e.g. opencode `bare_flags = []` and cursor `subcommand = []` MUST
  be written as `[]`, not omitted — omitting makes them nil and the mirror fails).
- ❌ **Don't edit `builtin.go` to "fix" a decode-parity mismatch.** `builtin.go` is the frozen source of
  truth (P1.M2.T2). If the test reports a diff, the `.toml` is wrong — align the `.toml` to the built-in.
- ❌ **Don't wire `providers/` into anything.** No `//go:embed`, no loader path, no registry change. The
  files are inert documentation; the config loader reads `.stagecoach.toml`, not a `providers/` directory.
- ❌ **Don't use the PRD §12.7 prose values for codex.** The compiled-in codex has TWO revisions
  (`prompt_delivery = "stdin"`; `bare_flags = [--sandbox, read-only, --ephemeral]` — `--ask-for-approval`
  was dropped). Mirror `builtinCodex()` (with revisions), not the raw PRD §12.7 block.
- ❌ **Don't drop the `# TO CONFIRM` comments.** The contract explicitly requires them in codex.toml +
  cursor.toml. They are documented honesty (§12.7.2), resolved at runtime by P1.M5.T1.S2.
- ❌ **Don't add a build tag to the sync-guard test.** Unlike P1.M5.T1.S2's real-agent suite, this is a
  fast, offline, deterministic check that belongs in CI (`make test`). Gating it would defeat its purpose
  (catching docs/code drift before release).
- ❌ **Don't modify `builtin_test.go`.** It is complete (P1.M2.T2). Put the file-based decode-parity test
  in a NEW file (`referencefiles_test.go`); same package, distinct test-function names.
- ❌ **Don't invent field values.** Every value comes from `builtin.go` (or its decode-parity constant).
  If a field is absent in the built-in, OMIT it and document it in the trailing note — never fabricate.
