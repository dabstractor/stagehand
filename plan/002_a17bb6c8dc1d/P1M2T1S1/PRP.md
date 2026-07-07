# PRP — P1.M2.T1.S1: Implement `builtinAgy()` manifest + add to `BuiltinManifests()` + create `providers/agy.toml`

> **Scope discipline.** This subtask adds the **7th built-in provider, `agy`** (Google Antigravity CLI,
> the Gemini-CLI successor per PRD §12.5.1), following the established `builtinGemini()` pattern. It
> ships `experimental = true` (non-TTY stdout bug, issue #76) with empty `tooled_flags` (cannot serve as
> stager until §12.5.1.1 item 4 clears). The **precise `preferredBuiltins` ordering** and **pi
> `default_model`→empty** are P1.M2.T2.S1's scope — this task only *appends* agy to keep the sync
> invariant green. Do NOT touch the Render/Validate/Resolve/Merge logic (P1.M1 is complete).
>
> **No external research needed.** agy's flag surface is FIXED by the contract (it ships experimental
> precisely because it is not `--help`-verified). Implement the spec'd manifest verbatim — do NOT
> investigate agy's real flags.

---

## Goal

**Feature Goal**: Make `agy` a compiled-in built-in provider whose manifest exactly matches PRD §12.5.1
(as refined by the work-item contract), so `stagecoach providers list` shows it, `providers show agy`
prints its manifest, and it can serve the bare roles (planner/message/arbiter) — but NOT the stager
(empty `tooled_flags`). It ships `experimental = true`.

**Deliverable**:
1. `internal/provider/builtin.go`: a `builtinAgy()` function + a `"agy": builtinAgy()` entry in
   `BuiltinManifests()` (+ updated doc comment).
2. `internal/provider/registry.go`: `"agy"` appended to `preferredBuiltins` (keeps the sync test green).
3. `providers/agy.toml`: a reference manifest mirroring `builtinAgy()` (header style = `gemini.toml`).
4. Test edits: the `agyTOML` decode-parity literal + DecodeParity case + reference-files entry +
   KeysAndCount `6→7` + (conventional) an `AgyFields` + `RenderedCommand_Agy` test.
5. `docs/providers.md`: agy row in the built-in table + experimental/non-TTY note.

**Success Definition**: `go build ./... && go vet ./... && go test ./internal/provider/...` are GREEN,
`agy` is a `BuiltinManifests()` key, `providers show agy` prints its manifest, and all three sync guards
(`TestBuiltinManifests_DecodeParity`, `TestProviderReferenceFiles_DecodeParity`,
`TestProviderReferenceFiles_AllBuiltinsCovered`, `TestPreferredBuiltins_MatchesBuiltinKeys`) pass.

---

## Why

- **PRD §12.5.1**: `agy` (Antigravity CLI) **superseded `gemini` (Gemini CLI) on 2026-06-18** and is the
  Gemini lineage's current surface. The Antigravity coding-plan quota is reachable only through `agy`.
- It matters structurally like every provider: Stagecoach must render a concrete command line for it. Per
  §12.7.2, a provider added from docs (not a verified `--help`) ships `experimental = true` until a real
  end-to-end run clears the `# TO CONFIRM` items.
- **§12.5.1.1 item 1 (the blocker):** `agy -p`/`--print` **silently drops stdout when invoked from a
  non-TTY** (issue #76) — exactly how Stagecoach spawns agents. So `agy` is unusable for any role until
  upstream fixes it or Stagecoach PTY-shims the child (out of scope here). It still ships as a built-in so
  it is discoverable and ready.

---

## What

Add `agy` as the 7th built-in, mirroring `gemini` (its closest structural twin — same read-only
`--approval-mode default` profile, same no-system-prompt-flag → prepend, same stdin delivery). Two
deliberate deviations from gemini: `default_model = "gemini-2.5-pro"` (agy runs the Gemini family) and
`experimental = true`. `tooled_flags` is intentionally nil (agy cannot stager until §12.5.1.1 item 4).

### Success Criteria

- [ ] `BuiltinManifests()` has 7 keys including `"agy"`; `builtinAgy()` returns the contract manifest.
- [ ] `preferredBuiltins` includes `"agy"` (pi still first) — sync test green.
- [ ] `providers/agy.toml` exists and decodes to a Manifest `reflect.DeepEqual` to `builtinAgy()`.
- [ ] `agyTOML` (test literal) decodes `reflect.DeepEqual` to `builtinAgy()`.
- [ ] `builtinAgy()` passes `Validate()` (Name + Command set; Output/PromptDelivery enums valid).
- [ ] `go build ./... && go vet ./... && go test ./internal/provider/...` GREEN.
- [ ] `docs/providers.md` documents agy (experimental + non-TTY issue #76).

---

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the exact manifest (field-by-field, nil/empty resolved), the exact pattern to
copy (`builtinGemini`), every test that breaks (+ the precise edit), and the DeepEqual trap are below.

### Documentation & References

```yaml
# MUST READ — the agy spec (authoritative flag surface)
- file: plan/002_a17bb6c8dc1d/prd_snapshot.md
  why: §12.5.1 is the canonical agy manifest + §12.5.1.1 lists the TO-CONFIRM items (non-TTY bug #76 is
       why experimental=true). The work-item CONTRACT refines it: tooled_flags=nil (not []), experimental=true.
  section: "12.5.1 Built-in provider: Antigravity CLI (agy)" and "12.5.1.1 TO CONFIRM (agy)"

# THE PATTERN TO COPY — agy is a structural twin of gemini
- file: internal/provider/builtin.go
  why: builtinGemini() is the exact template (read-only --approval-mode default, no sys flag → prepend,
       stdin delivery). builtinAgy() differs only in default_model + Experimental + (no subcommand).
  pattern: each builtin is a fresh-per-call func returning Manifest with strPtr/boolPtr fields; absent
           fields left NIL; explicit-empty string fields use strPtr("") (NON-NIL). BuiltinManifests() is a
           map literal. Copy the doc-comment style (NOTE bullets re: NON-NIL empty vs NIL absent).
  gotcha: BuiltinManifests() doc comment says "All six providers are now present" → update to seven + §12.5.1.

# The decode-parity oracle pattern (the #1 trap lives here)
- file: internal/provider/builtin_test.go
  why: the *TOML consts (piTOML…cursorTOML, lines 16-130) are LITERAL strings; TestBuiltinManifests_DecodeParity
       (311) does reflect.DeepEqual(builtin, decoded-TOML). Add agyTOML + a DecodeParity case.
  pattern: mirror geminiTOML exactly (same fields, same NON-NIL-empty "" for print/sys/provider flags).
  gotcha: TooledFlags=nil ⇒ OMIT tooled_flags from agyTOML (absent⇒nil). Do NOT write tooled_flags=[].
          subcommand is NOT in the contract ⇒ OMIT it (nil), like gemini (do NOT write subcommand=[]).

# The reference-file sync guard (second decode-parity + coverage)
- file: internal/provider/referencefiles_test.go
  why: providerFiles (14-26) lists each providers/<name>.toml; TestProviderReferenceFiles_DecodeParity reads
       the file and DeepEquals it to the builtin; TestProviderReferenceFiles_AllBuiltinsCovered asserts
       every builtin has a file (and vice versa). Adding agy to builtins WITHOUT a providerFiles entry FAILS.
  pattern: add {"agy","providers/agy.toml"} to providerFiles; create providers/agy.toml with field lines
           BYTE-FOR-BYTE identical to agyTOML (modulo comments).

# preferredBuiltins sync invariant (REQUIRED edit, not optional)
- file: internal/provider/registry.go
  why: preferredBuiltins (line 15) MUST stay set-equal to BuiltinManifests() keys — enforced by
       TestPreferredBuiltins_MatchesBuiltinKeys. Adding agy to builtins without adding to preferredBuiltins
       ⇒ count mismatch ⇒ test FAILS.
  gotcha: append "agy" at the END (least-preferred — appropriate for experimental). The ORDER is finalized
          by P1.M2.T2.S1 ("Reorder preferredBuiltins") — do NOT reorder/preempt it.

# The reference doc to mirror for providers/agy.toml header/style
- file: providers/gemini.toml
  why: the header block (WHAT THIS FILE IS / HOW TO USE / RENDERED COMMAND / TOOLS-DISABLE CATEGORY) is the
       style to copy for providers/agy.toml. Field lines must equal agyTOML.

# Docs to update (Mode A)
- file: docs/providers.md
  why: "## The 6 built-in providers" (line 55) → 7; add an agy row to the table (59-66) noting experimental
       + non-TTY stdout drop (issue #76). Follow the existing column format exactly.
```

### Current Codebase tree (relevant slice)

```bash
internal/provider/builtin.go           # builtinGemini() template + BuiltinManifests() map — EDIT (add builtinAgy + entry)
internal/provider/registry.go          # preferredBuiltins (L15) — EDIT (append "agy")
internal/provider/builtin_test.go      # *TOML consts + DecodeParity + KeysAndCount — EDIT
internal/provider/referencefiles_test.go # providerFiles — EDIT (add agy entry)
internal/provider/registry_test.go     # comment-only edit (L35 "6 built-ins"); assertions are dynamic
providers/gemini.toml                  # header/style TEMPLATE for the new providers/agy.toml (READ-ONLY)
providers/agy.toml                     # CREATE (mirrors builtinAgy / agyTOML)
docs/providers.md                      # built-in table — EDIT (add agy row, experimental note)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 trap): reflect.DeepEqual across THREE artifacts. go-toml/v2 distinguishes:
//   - ABSENT key            → nil slice / nil *string/*bool
//   - `key = []`            → NON-NIL empty slice ([]string{})
//   - `key = ""`            → NON-NIL *string ("")
// builtinAgy() (Go), agyTOML (test literal), and providers/agy.toml (file) MUST decode to the same Manifest.
//   • TooledFlags: leave NIL in Go; OMIT `tooled_flags` from BOTH TOMLs (do NOT write `tooled_flags = []`).
//   • Subcommand: OMIT (nil) on all three sides — NOT in the contract; matches gemini (not `subcommand = []`).
//   • print_flag/system_prompt_flag/provider_flag: strPtr("") in Go AND `... = ""` in TOML (NON-NIL empty).

// CRITICAL: preferredBuiltins (registry.go:15) is set-equality-checked against BuiltinManifests() keys by
//   TestPreferredBuiltins_MatchesBuiltinKeys. You MUST append "agy" to preferredBuiltins or that test fails
//   (count 6 vs 7). Keep pi first (the test asserts preferredBuiltins[0]=="pi"). Append at END.

// CRITICAL: providers/*.toml are NOT loaded at runtime (built-ins are compiled in), but
//   TestProviderReferenceFiles_DecodeParity READS them and DeepEquals to the builtin. So providers/agy.toml
//   is a real test oracle, not just docs — its field lines must equal agyTOML exactly.

// GOTCHA: agy has nil TooledFlags ⇒ Manifest.Render(..., RenderTooled) ERRORS for agy (render.go:120,
//   "tooled mode requires non-empty tooled_flags"). This is CORRECT (agy cannot stager). No test iterates
//   all builtins in tooled mode, so this breaks nothing. Do NOT add tooled_flags just to silence it.
```

---

## Implementation Blueprint

### Data models and structure

No new types. `builtinAgy()` returns the existing `Manifest` struct (which already has `TooledFlags` and
`Experimental` from P1.M1.T1.S1). `strPtr`/`boolPtr` helpers exist (manifest.go:182-183).

### The manifest (the exact `builtinAgy()` body)

```go
// builtinAgy returns the agy (Google Antigravity CLI) manifest per PRD §12.5.1 (the Gemini-CLI successor,
// superseded gemini on 2026-06-18). Flag surface assembled from Antigravity's docs + issue tracker (NOT
// yet `--help`-verified) → ships Experimental=true (§12.7.2) until §12.5.1.1 items clear. agy has no
// first-class system-prompt flag → sys is PREPENDED to the payload (§12.2), like gemini. `--approval-mode
// default` is a read-only, never-ask profile (§12.7.1 "read-only constraint").
//
// BLOCKER (§12.5.1.1 item 1): agy -p/--print silently drops stdout when spawned from a non-TTY (issue #76)
// — exactly how stagecoach spawns agents. agy is unusable for any role until upstream fixes it or stagecoach
// PTY-shims the child. Shipping experimental keeps it discoverable/ready.
//
// STAGER: TooledFlags is intentionally nil — agy CANNOT serve as a stager until §12.5.1.1 item 4 (the
// scoped, non-interactive, git-scoped tool combo) is verified. RenderTooled errors on nil tooled_flags.
//
// NOTE: (1) PrintFlag="-p" (NON-NIL). (2) SystemPromptFlag/ProviderFlag are strPtr("") — §12.5.1 WRITES
// them "" (NON-NIL empty): no sys flag (sys prepended, §12.2), no sub-provider. (3) default_model is
// "gemini-2.5-pro" (agy runs the Gemini family). (4) Experimental=boolPtr(true) (ships experimental).
// (5) Subcommand/PromptFlag/DefaultProvider/JsonField/RetryInstruction/Env/TooledFlags are nil (absent,
// like gemini). agy is the Gemini-lineage twin of gemini, differing in default_model + Experimental.
func builtinAgy() Manifest {
	return Manifest{
		Name:             "agy",
		Detect:           strPtr("agy"),
		Command:          strPtr("agy"),
		PromptDelivery:   strPtr("stdin"),
		PrintFlag:        strPtr("-p"),
		ModelFlag:        strPtr("-m"),
		DefaultModel:     strPtr("gemini-2.5-pro"),
		SystemPromptFlag: strPtr(""), // §12.5.1 NON-NIL empty — no sys flag; sys prepended to payload (§12.2)
		ProviderFlag:     strPtr(""), // §12.5.1 NON-NIL empty — agy has no sub-provider
		BareFlags: []string{
			"--approval-mode", "default", // read-only, never-ask profile (don't auto-run tools)
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		Experimental:   boolPtr(true), // §12.5.1.1 ships experimental (non-TTY stdout drop, issue #76)
		// TooledFlags: nil — agy cannot serve as a stager until §12.5.1.1 item 4 is verified.
		// Subcommand, PromptFlag, DefaultProvider, JsonField, RetryInstruction, Env: nil (absent, like gemini).
	}
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/provider/builtin.go
  - ADD: the builtinAgy() function above (place it directly AFTER builtinGemini(), since agy is the
         Gemini-CLI successor — §12.5.1 follows §12.5).
  - ADD: `"agy": builtinAgy(),` to the BuiltinManifests() map (place after the "gemini": line).
  - UPDATE: the BuiltinManifests() doc comment — "All six providers are now present" → seven, and add
            "§12.5.1 agy (experimental — the Gemini-CLI successor)" to the §-reference list.
  - NAMING: builtinAgy (camelCase, matches builtinPi/builtinGemini/etc.).
  - GUARDRAIL: do NOT modify any other builtin or the Render/Validate/Resolve/Merge logic.

Task 2: MODIFY internal/provider/registry.go
  - EDIT: preferredBuiltins (line 15) — APPEND "agy" at the END:
          `var preferredBuiltins = []string{"pi", "claude", "gemini", "opencode", "codex", "cursor", "agy"}`
  - UPDATE: the comment — note agy is experimental (least-preferred; order finalized by P1.M2.T2.S1).
  - WHY REQUIRED: TestPreferredBuiltins_MatchesBuiltinKeys enforces set-equality; without this it fails (6 vs 7).
  - GUARDRAIL: keep "pi" first; do NOT reorder existing entries (that is P1.M2.T2.S1's scope).

Task 3: CREATE providers/agy.toml
  - MIRROR: the header-doc style of providers/gemini.toml (WHAT THIS FILE IS / HOW TO USE / RENDERED
            COMMAND / TOOLS-DISABLE CATEGORY blocks). Adapt copy for agy (experimental, non-TTY issue #76,
            the §12.5.1 rendered command).
  - FIELD LINES: BYTE-FOR-BYTE identical to agyTOML (Task 4), modulo comments. Specifically:
        name = "agy"
        detect = "agy"
        command = "agy"
        prompt_delivery = "stdin"
        print_flag = "-p"
        model_flag = "-m"
        default_model = "gemini-2.5-pro"
        system_prompt_flag = ""
        provider_flag = ""
        bare_flags = ["--approval-mode", "default"]
        output = "raw"
        strip_code_fence = true
        experimental = true
  - OMIT: tooled_flags (nil), subcommand, prompt_flag, default_provider, json_field, retry_instruction, [env].
  - GOTCHA: this file IS a test oracle (TestProviderReferenceFiles_DecodeParity reads it) — not just docs.

Task 4: MODIFY internal/provider/builtin_test.go
  - ADD const agyTOML (after cursorTOML, ~line 130) — a `...` raw-string literal with the SAME field lines
    as providers/agy.toml (Task 3). Add a header comment: "agyTOML — PRD §12.5.1; experimental=true;
    tooled_flags omitted (nil — agy cannot stager). Decoding it must match builtinAgy()."
  - EDIT TestBuiltinManifests_KeysAndCount: comment (162) "exactly 6 keys" → 7; `len(m) != 6` → `!= 7`
    (167); message "want 6" → "want 7" (168).
  - ADD `{"agy", builtinAgy(), agyTOML},` to the TestBuiltinManifests_DecodeParity table (after cursor).
  - (CONVENTIONAL) ADD TestBuiltinManifests_AgyFields — mirror TestBuiltinManifests_GeminiFields (381):
    assert Name/Detect/Command/PromptDelivery/PrintFlag/ModelFlag/DefaultModel/SystemPromptFlag("" non-nil)/
    ProviderFlag("" non-nil)/BareFlags/Output/StripCodeFence, PLUS Experimental != nil && *Experimental==true,
    AND TooledFlags == nil.
  - (CONVENTIONAL) ADD TestBuiltinManifests_RenderedCommand_Agy — mirror RenderedCommand_Gemini (455):
    argv := renderArgs(builtinAgy(), "", "", "<sys>"); want:
    ["agy","-m","gemini-2.5-pro","--approval-mode","default","-p"].

Task 5: MODIFY internal/provider/referencefiles_test.go
  - ADD `{"agy", "providers/agy.toml"},` to the providerFiles slice (after cursor, ~line 24).
  - UPDATE: the providerFiles comment "the 6 shipped reference manifests" → 7.
  - This single edit makes BOTH TestProviderReferenceFiles_DecodeParity AND
    TestProviderReferenceFiles_AllBuiltinsCovered cover agy (no other change needed there).

Task 6: MODIFY internal/provider/registry_test.go
  - COMMENT-ONLY: line 35 "exactly the 6 built-ins" → 7. (The count assertions at 40-41 / 118-119 use
    len(BuiltinManifests()) dynamically — they auto-adjust; NO code change.)

Task 7: MODIFY docs/providers.md
  - EDIT: "## The 6 built-in providers" (line 55) → "## The 7 built-in providers".
  - ADD a table row after the gemini row (line 63), following the exact column format:
    | `agy` | stdin | `-p` | `-m` | `gemini-2.5-pro` | (prepended) | Read-only constraint (`--approval-mode default`) |
  - ADD a one-line note under the table (or in the agy row) that agy is **experimental** (PRD §12.5.1) due
    to the non-TTY stdout drop (issue #76), and cannot serve as a stager (empty tooled_flags).
```

### Implementation Patterns & Key Details

```go
// PATTERN: agy is the twin of gemini (builtinGemini). Copy gemini's structure, then:
//   - DefaultModel: "gemini-2.5-pro" (gemini already has this — same)
//   - ADD Experimental: boolPtr(true)  (gemini has none → defaults false)
//   - PrintFlag: strPtr("-p")           (gemini has strPtr(""); agy HAS a -p print flag per §12.5.1)
//   - everything else identical to gemini (same read-only profile, same no-sys-flag prepend, stdin delivery).
//
// GOTCHA (DeepEqual): the THREE artifacts (Go struct / agyTOML literal / providers/agy.toml file) must
// agree on every nil-vs-empty decision. The safe rule: if a field is NOT in the contract table, leave it
// NIL in Go and OMIT it from both TOMLs. TooledFlags and Subcommand are the two that are tempting to write
// as `= []` (matching the PRD §12.5.1 prose) but MUST be omitted to keep DeepEqual green (contract = nil).
```

### Integration Points

```yaml
CODE: builtin.go (builtinAgy + map entry), registry.go (preferredBuiltins +1). No Render/Validate/
      Resolve/Merge changes (P1.M1 complete; agy satisfies Validate as-is).
CONFIG: none (no config-schema change).
ROUTES/CLI: none this task (providers list/show auto-discover agy via the registry; experimental display
            logic, if any, is a later task).
DOCS (Mode A): docs/providers.md built-in table + experimental note.
DOWNSTREAM (NOT this task): P1.M2.T2.S1 reorders preferredBuiltins (agy likely moves before gemini) and
  sets pi default_model="". Do NOT preempt either.
```

---

## Validation Loop

### Level 1: Syntax & Style

```bash
go build ./...
go vet ./...
gofmt -l internal/provider/builtin.go internal/provider/registry.go internal/provider/builtin_test.go internal/provider/referencefiles_test.go internal/provider/registry_test.go
# Expected: clean. Run gofmt -w on any listed file.
```

### Level 2: The provider-package test suite (the real gate — all sync guards live here)

```bash
go test ./internal/provider/... -v
# Expected: PASS. Verify explicitly:
#   TestBuiltinManifests_KeysAndCount ................ len == 7
#   TestBuiltinManifests_DecodeParity ................ agy case passes (builtinAgy == decoded agyTOML)
#   TestBuiltinManifests_AgyFields ................... (new) Experimental==true, TooledFlags==nil
#   TestBuiltinManifests_RenderedCommand_Agy ......... (new) argv == [agy -m gemini-2.5-pro --approval-mode default -p]
#   TestBuiltinManifests_NameMatchesKey .............. agy included (Name=="agy"==key)
#   TestBuiltinManifests_Validate .................... agy passes Validate
#   TestPreferredBuiltins_MatchesBuiltinKeys ......... set-equal (7 == 7), pi first
#   TestProviderReferenceFiles_DecodeParity/agy ...... providers/agy.toml == builtinAgy
#   TestProviderReferenceFiles_AllBuiltinsCovered .... agy has a file AND a builtin entry
```

> **If `TestBuiltinManifests_DecodeParity/agy` or `TestProviderReferenceFiles_DecodeParity/agy` fails:**
> it is a nil-vs-empty mismatch. Read the DeepEqual diff in the failure output, then make `builtinAgy()`,
> `agyTOML`, and `providers/agy.toml` agree. The usual culprit: a stray `tooled_flags = []` or
> `subcommand = []` in a TOML (decodes non-nil) vs nil in Go — remove those keys.

### Level 3: Whole-repo build/test (no transient break expected here)

```bash
go build ./...   # Expected: clean (unlike config-refactor tasks, this one does NOT break other packages)
go test ./...    # Expected: all PASS. If a non-provider package breaks, it hardcodes the 6-provider set —
                 # fix that call site to use len(BuiltinManifests()) dynamically (do NOT weaken agy).
```

### Level 4: Behavioral spot-check (proves discoverability)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach
/tmp/stagecoach providers show agy | head   # prints the agy manifest TOML (experimental=true, bare_flags, …)
/tmp/stagecoach providers list | grep agy   # agy appears (detected iff `agy` is on $PATH; experimental marker TBD)
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on edited files.
- [ ] `go test ./internal/provider/...` PASS (all four sync guards green).
- [ ] `go test ./...` PASS (no non-provider package regresses).
- [ ] DeepEqual parity holds: `builtinAgy()` == decoded `agyTOML` == decoded `providers/agy.toml`.

### Feature Validation
- [ ] `BuiltinManifests()["agy"]` exists and equals `builtinAgy()`.
- [ ] `builtinAgy()`: Experimental != nil && *Experimental == true; TooledFlags == nil.
- [ ] `preferredBuiltins` contains "agy" (pi still first); `TestPreferredBuiltins_MatchesBuiltinKeys` green.
- [ ] `providers show agy` prints the manifest; `providers list` shows agy.
- [ ] agy CANNOT stager (`RenderTooled` errors on nil tooled_flags — verified, not worked around).

### Code Quality Validation
- [ ] `builtinAgy()` follows the `builtinGemini()` pattern exactly (fresh-per-call, strPtr/boolPtr, NOTE bullets).
- [ ] `providers/agy.toml` header style matches `providers/gemini.toml`; field lines equal `agyTOML`.
- [ ] Test additions (`AgyFields`, `RenderedCommand_Agy`, DecodeParity case, providerFiles entry) follow the
      per-provider convention already established for the other six.
- [ ] No edits to Render/Validate/Resolve/Merge/executor (P1.M1 complete and untouched).

### Documentation
- [ ] `docs/providers.md` lists agy in the built-in table with the experimental + non-TTY (issue #76) note.
- [ ] `BuiltinManifests()` doc comment updated (seven providers; §12.5.1 agy).

---

## Anti-Patterns to Avoid

- ❌ **Don't write `tooled_flags = []` or `subcommand = []` in either TOML** — they decode NON-NIL empty and
  break `reflect.DeepEqual` vs the nil Go fields. Omit those keys (contract = nil).
- ❌ **Don't omit the `preferredBuiltins` edit** — `TestPreferredBuiltins_MatchesBuiltinKeys` enforces
  set-equality; agy-in-builtins without agy-in-preferredBuiltins fails (6 vs 7).
- ❌ **Don't reorder `preferredBuiltins` beyond appending agy** — the order is P1.M2.T2.S1's scope.
- ❌ **Don't add `tooled_flags` to make agy stager-capable** — agy intentionally CANNOT stager until
  §12.5.1.1 item 4 (the `RenderTooled` error is correct behavior).
- ❌ **Don't try to verify agy's real `--help` flags** — the contract ships it experimental precisely
  because it is unverified; implement the spec'd manifest verbatim.
- ❌ **Don't forget `providers/agy.toml`** — `TestProviderReferenceFiles_AllBuiltinsCovered` fails if a
  builtin lacks a reference file (and the file is itself a decode-parity oracle).
- ❌ **Don't hardcode `6`** anywhere — use `len(BuiltinManifests())` dynamically for provider counts.

---

## Confidence Score

**9/10** — A well-scoped "add a 7th builtin following an established twin pattern" task. Every break site
is enumerated (verified by grep), the manifest is fully specified (nil/empty resolved for DeepEqual), and
the #1 trap (three-way `reflect.DeepEqual` parity + the preferredBuiltins sync invariant) is flagged
prominently with the exact resolution. The -1 reserves for the conventional `AgyFields`/`RenderedCommand`
test wording (low-risk, follows the gemini template) and the docs-table phrasing (non-blocking).
