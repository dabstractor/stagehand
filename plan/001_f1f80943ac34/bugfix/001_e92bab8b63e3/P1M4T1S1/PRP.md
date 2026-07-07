# PRP — P1.M4.T1.S1: Apply `cfg.Output`/`cfg.StripCodeFence` onto the manifest in `buildDeps`

**Issue**: PRD Issue 4 (Minor) — the `[generation]` `output` / `strip_code_fence` config fields (file **and** git-config) are loaded but never applied.
**PRD refs**: §16.2 (`[generation] output`, `[generation] strip_code_fence`), §16.1 (layer-1 defaults), §16.3 (git-config keys), §12.9 (parse uses the manifest's `output`/`strip_code_fence`).
**Binding decisions**: `plan/.../bugfix/001_e92bab8b63e3/architecture/decisions.md` **D4**; `.../architecture/seam_config_and_autostage.md` **Part A**.

---

## Goal

**Feature Goal**: Close the missing `cfg → manifest` bridge for the two `[generation]` output-tuning fields. In `buildDeps`, after `m.Validate()` and the Issue-3 pre-flight, copy `cfg.Output` and `cfg.StripCodeFence` onto the resolved provider `Manifest`'s pointer fields so that `provider.ParseOutput` honors them end-to-end — making the `[generation]` (and git-config `stagecoach.*`) values override the per-manifest per-provider defaults (broader setting wins).

**Deliverable**:
1. **Code** (~6 lines): one block added to `pkg/stagecoach/stagecoach.go::buildDeps`, inserted between the `reg.IsInstalled(m)` pre-flight and the final `return generate.Deps{...}`.
2. **Docs (Mode A — ride with the implementing subtask)**:
   - `docs/configuration.md` — affirm `output`/`strip_code_fence` now apply to parsing (override per-manifest defaults); reflect the `stagecoach.output` / `stagecoach.stripCodeFence` git-config keys.
   - `internal/cmd/config.go::exampleConfigTemplate` — make the `[generation] output/strip_code_fence` comment accurate; add a one-line note that these tune ALL providers.

**Success Definition**:
- `go build ./...`, `go vet ./...`, `go test -race ./...`, `make lint` all green with the change in place.
- Setting `[generation] output = "json"` (or `git config stagecoach.output json`) causes `ParseOutput` to parse JSON; `git config stagecoach.stripCodeFence false` disables fence stripping — overriding whatever the resolved manifest's per-provider value was.
- The pre-existing `buildDeps` contract (unknown-provider → exit 1; `Validate()` → exit 1; missing-command pre-flight → exit 1) is **unchanged** — the new block is strictly additive and runs only on the success path.
- (End-to-end test assertions live in sibling subtask **P1.M4.T1.S2**; this PRP ships the bridge + docs only.)

## Why

- **User impact**: Users who set `output = "json"` / `strip_code_fence = false` (or the git-config equivalents) reasonably expect the parsing pipeline to honor them. Today these knobs are a **silent no-op** — `config.Config.Output`/`.StripCodeFence` are populated by every loader and asserted on in tests, but no production code consumes them; `provider.ParseOutput` reads only `deps.Manifest.Output`/`.StripCodeFence`.
- **Smaller change / least surprising**: Applying the cfg values onto the manifest is ~6 lines in one function and **keeps** a documented, `config init`-advertised capability. The alternative (remove the fields) touches ~6 files + ~6 tests and contradicts the shipped canonical config template. (See decisions.md **D4** for the rejection rationale.)
- **Scope respect**: This is the "apply" half of Issue 4. It does **not** touch the unrelated `file.go` "cannot set false via file" quirk (out of scope per contract) and does **not** alter `ParseOutput` (it already reads these manifest pointers).

## What

### User-visible behavior (after fix)
- `[generation] output = "json"` (config file) → the agent's stdout is parsed as JSON (extract `json_field`); non-JSON output falls back to raw with the `fellback` log flag (PRD §12.9 step 3).
- `git config stagecoach.output json` → same, via the git-config loader.
- `git config stagecoach.stripCodeFence false` → ``` fences are NOT stripped from agent output, overriding a per-provider default of `true`.
- A `[provider.<name>] output = "json"` per-provider override is **still merged** by the registry, but a `[generation] output` value now **wins** over it (broader layer overrides narrower) — consistent with "generation config tunes ALL providers".

### Success Criteria
- [ ] `buildDeps` copies `cfg.Output` (guarded on `!= ""`) and `cfg.StripCodeFence` (unconditional) onto the manifest via **local variables** (no `&cfg.*` aliasing), placed **after** `m.Validate()` **and** after the Issue-3 `reg.IsInstalled(m)` pre-flight, **before** the `return generate.Deps{...}`.
- [ ] The asymmetry from the contract is preserved exactly: `Output` guarded on `cfg.Output != ""`; `StripCodeFence` applied unconditionally.
- [ ] `docs/configuration.md` affirms these two knobs apply to parsing.
- [ ] `internal/cmd/config.go` `exampleConfigTemplate` `[generation]` comment is accurate + notes "tunes all providers".
- [ ] No change to `ParseOutput`, the registry merge, the config loaders, or the `file.go` false-set quirk.

## All Needed Context

### Context Completeness Check
✅ Passes the "No Prior Knowledge" test: the exact function, line numbers, the pointer-field semantics, the insertion point, the precedence rationale, and the validation commands are all specified below.

### Documentation & References

```yaml
# MUST READ — binding architecture decisions for this exact fix
- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/decisions.md
  section: D4 — Fix Issue 4 by APPLYING [generation] output/strip_code_fence onto the manifest (not removing)
  why: This is THE binding decision. Contains the exact ~6-line patch, the copy-to-local rationale, the
       "broader setting wins" precedence rule, the explicit "do not fix the file.go false-set quirk"
       out-of-scope note, and the dependency-on-M1 (config handoff) sequencing.
  critical: Apply cfg values AFTER Validate()+pre-flight. Do NOT re-Validate(). Mirror the `o := cfg.Output`
            copy-to-local pattern for BOTH fields. StripCodeFence is applied UNCONDITIONALLY (no guard).

- file: plan/001_f1f80943ac34/bugfix/001_e92bab8b63e3/architecture/seam_config_and_autostage.md
  section: PART A — Dead [generation] output / strip_code_fence config fields (Issue 4)
  why: Proves (with exact line citations) that cfg.Output/cfg.StripCodeFence ARE populated by every loader
       but NEVER consumed; that ParseOutput reads ONLY the manifest; that buildDeps is the single seam.
  critical: §A.8 names buildDeps as the insertion point. §A.5/A.6 confirm ParseOutput + the manifest build
            path. (Note: §A.8's "re-Validate" suggestion is SUPERSEDED by binding contract item 3 + D4 — do
            NOT re-Validate; ParseOutput's switch default handles unknown output as raw.)

# The consumer — do not modify, just confirm it reads these pointer fields
- file: internal/provider/parse.go
  lines: 44-52 (ParseOutput signature + Resolve(); 56 strip; 62 switch on *r.Output)
  why: Confirms ParseOutput reads ONLY m.Resolve().Output / .StripCodeFence (pointer fields). No parser
       change is needed once buildDeps writes those pointers.
  gotcha: The `switch *r.Output` has a `default:` branch that treats an unrecognized value as raw — so an
          invalid cfg.Output (e.g. "yaml") degrades gracefully rather than panicking. This is why no
          re-Validate() is required.

# The manifest pointer-field design — read to understand WHY copy-to-local
- file: internal/provider/manifest.go
  lines: 78-80 (Output *string, StripCodeFence *bool); 137-159 (Resolve())
  why: Explains the *string/*bool override-signal design (nil=inherit, non-nil=override). Assigning a
       non-nil pointer = "this value wins". Resolve() fills nils with defaults but PRESERVES present values.
  gotcha: Manifest fields are POINTERS. You must assign &local, never &cfg.Field (aliasing the cfg copy).

# The exact function to edit
- file: pkg/stagecoach/stagecoach.go
  lines: 154-198 (buildDeps); insert between the IsInstalled pre-flight (~189-195) and the return (~197)
  why: THE single seam. cfg.Output / cfg.StripCodeFence are in scope (cfg is the value param). m is the
       resolved, Validate()-d, pre-flighted manifest.
  pattern: Mirror the existing plain-error returns (`fmt.Errorf("unknown provider %q", name)` etc.).

# The cfg fields (already resolved by the time buildDeps runs)
- file: internal/config/config.go
  lines: 31-32 (Output string; StripCodeFence bool); 64-65 (Defaults: Output "raw", StripCodeFence true)
  why: cfg.Output is ALWAYS non-empty post-Defaults (so the `!= ""` guard is effectively always-true, kept
       for defensive correctness). cfg.StripCodeFence is ALWAYS a concrete bool.

# Docs to update (Mode A)
- file: docs/configuration.md
  lines: 47-54 ([generation] example block); 65-80 (Built-in defaults table incl. output/strip_code_fence
         rows ~78-79); 111-116 (git-config keys table — currently OMITS stagecoach.output / stripCodeFence)
  why: Affirm output/strip_code_fence now apply to PARSING (override per-manifest defaults). The git-config
       keys for these two fields EXIST in git.go but are absent from the docs table — add them for accuracy.
- file: internal/cmd/config.go
  lines: 154-162 (exampleConfigTemplate [generation] block: output="raw", strip_code_fence=true comments)
  why: These comment lines become accurate post-fix. Add a one-line note that these tune ALL providers
       (the broader layer), per contract item 5 / decision D6.

# Existing test that exercises the SAME buildDeps tail (must stay green unchanged)
- file: pkg/stagecoach/stagecoach_test.go
  lines: 485-589 (TestGenerateCommit_MissingProviderCommand_Issue3)
  why: Proves the pre-flight + return path below the insertion point is intact. S1 must not perturb it.
       S2 (sibling subtask) adds the dedicated cfg→ParseOutput end-to-end test.
```

### Current Codebase tree (relevant slice)

```bash
pkg/stagecoach/
  stagecoach.go          # buildDeps (line 154) — THE edit site
  stagecoach_test.go     # TestGenerateCommit_MissingProviderCommand_Issue3 (485) — must stay green
internal/provider/
  parse.go              # ParseOutput — reads m.Output/.StripCodeFence (NO change)
  manifest.go           # Manifest pointer fields + Resolve() (NO change)
internal/config/
  config.go             # Config.Output/.StripCodeFence (NO change); Defaults()
  file.go, git.go       # loaders that populate the cfg fields (NO change; quirk out of scope)
docs/
  configuration.md      # Mode A doc edit
internal/cmd/
  config.go             # exampleConfigTemplate — Mode A doc edit
```

### Desired Codebase tree (files MODIFIED — no new files)

```bash
pkg/stagecoach/stagecoach.go    # +~6 lines in buildDeps (cfg→manifest bridge)
docs/configuration.md         # affirm output/strip_code_fence apply to parsing + git-config keys
internal/cmd/config.go        # exampleConfigTemplate [generation] comment accuracy + "all providers" note
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL — Manifest.Output / .StripCodeFence are *string / *bool POINTERS (manifest.go:78-80).
// Assign a NON-NIL pointer = "override". Copy the cfg value into a LOCAL first, then take its address:
//   o := cfg.Output; m.Output = &o          // ✅ correct
//   m.Output = &cfg.Output                   // ❌ aliases the cfg value-param's address (avoid per D4)

// CRITICAL — ordering: apply AFTER m.Validate() AND AFTER the reg.IsInstalled(m) pre-flight, BEFORE the
// return generate.Deps{...}. Do NOT move it above Validate() (the pre-flight/Validate contract must run
// on the registry-merged manifest first). Do NOT re-Validate() after — contract item 3 + D4 do not call
// for it, and ParseOutput's switch-default already degrades an unknown Output to raw.

// CRITICAL — keep the asymmetry EXACTLY:
//   if cfg.Output != "" { o := cfg.Output; m.Output = &o }   // guarded
//   scf := cfg.StripCodeFence; m.StripCodeFence = &scf        // UNCONDITIONAL (no guard)
// StripCodeFence has no guard because cfg.StripCodeFence is always a concrete bool post-Defaults and the
// "broader setting wins" rule means even the default `true` should consistently override a per-provider value.

// OUT OF SCOPE — the file.go "cannot set false via file" quirk (file.go materialize ~line 153:
//   if g.StripCodeFence { c.StripCodeFence = true }  // v1 limitation: cannot set false via file
// ). Via the FILE loader strip_code_fence can only be turned ON. The false case is exercisable via
// git-config (`stagecoach.stripCodeFence false`) or direct cfg injection. DO NOT fix this here (contract).

// NOTE — cfg.Output is always non-empty post-Defaults ("raw"), so the `!= ""` guard is effectively always
// true in production. Keep the guard anyway (defensive; matches D4 verbatim).
```

## Implementation Blueprint

### The single edit (pkg/stagecoach/stagecoach.go :: buildDeps)

In `buildDeps`, between the closing `}` of the `if !reg.IsInstalled(m) { ... }` pre-flight block and the `return generate.Deps{Git: git.New(repoDir), Manifest: m}, nil`, insert:

```go
	// Apply [generation] output/strip_code_fence onto the resolved manifest (PRD Issue 4 / §16.2 / §12.9).
	// cfg.Output / cfg.StripCodeFence are populated by every loader (file, git-config) and Defaults, but
	// were previously dropped here — ParseOutput reads ONLY the manifest's pointer fields. Copying them
	// onto the manifest makes the [generation] / git-config values override the per-provider per-manifest
	// values (broader setting wins), which ParseOutput then honors. (decisions.md D4.)
	//
	// Copy into locals (not &cfg.*) to avoid aliasing the cfg value-param's address. Output is guarded
	// (defensive; it is always non-empty post-Defaults); StripCodeFence is applied unconditionally so the
	// broader [generation] layer consistently overrides any per-manifest default. No re-Validate():
	// ParseOutput's switch-default degrades an unknown Output to raw.
	if cfg.Output != "" {
		o := cfg.Output
		m.Output = &o
	}
	scf := cfg.StripCodeFence
	m.StripCodeFence = &scf
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY pkg/stagecoach/stagecoach.go :: buildDeps (the bridge)
  - INSERT: the ~15-line block (above) between the reg.IsInstalled(m) pre-flight `}` and the
            `return generate.Deps{Git: git.New(repoDir), Manifest: m}, nil` (current ~line 197).
  - PLACEMENT: strictly AFTER `m.Validate()` (~181) AND after the `if !reg.IsInstalled(m)` block (~189-195);
               strictly BEFORE the return.
  - EXACT semantics: `if cfg.Output != "" { o := cfg.Output; m.Output = &o }` then
                     `scf := cfg.StripCodeFence; m.StripCodeFence = &scf`.
  - DO NOT: re-Validate(); touch ParseOutput/manifest/registry/config-loaders; fix the file.go quirk;
            change the unknown-provider / Validate / pre-flight error returns.
  - VERIFY after: `go build ./...` && `go vet ./...` (the edit is additive; types already line up —
                  m.Output is *string, m.StripCodeFence is *bool; cfg.Output is string, cfg.StripCodeFence bool).

Task 2: MODIFY docs/configuration.md (Mode A — affirm parsing applies)
  - In the "Built-in defaults" table (output/strip_code_fence rows ~78-79): no value change, but add a
    short prose note immediately after the table (or as a footnote) that `output` and `strip_code_fence`
    apply to PARSING of agent output and override the per-manifest (per-provider) defaults — i.e. setting
    `output = "json"` makes Stagecoach parse the agent's stdout as JSON across all providers.
  - In the "Git-config keys" table (~111-116): ADD two rows documenting the keys git.go already reads —
    `stagecoach.output` (string, `git config --get stagecoach.output`, "Agent output mode: raw | json")
    and `stagecoach.stripCodeFence` (bool, `git config --get --bool stagecoach.stripCodeFence`,
    "Strip ``` fences from agent output"). The camelCase bool key matches git.go:152.
  - KEEP tone/format consistent with the existing tables. Do NOT invent new env vars or CLI flags
    (there are none for these fields — intentional; loadEnv/loadFlags don't set them).

Task 3: MODIFY internal/cmd/config.go :: exampleConfigTemplate (Mode A — comment accuracy)
  - In the `[generation]` block (~lines 154-162), the existing lines are:
        # output                = "raw"   # agent output mode: "raw" | "json"
        # strip_code_fence      = true    # remove ` fences from agent output
    These are now ACCURATE (previously a silent no-op). Update the trailing comments to convey they
    APPLY to parsing, and ADD a one-line note (e.g. above or below the two lines) that these tune ALL
    providers (the broader layer, overriding per-provider [provider.<name>] values).
  - EXAMPLE replacement (keep the ` + "`" + ` Go-concatenation for backticks exactly as the template uses):
        # output           = "raw"   # agent output mode: "raw" | "json" — applies to parsing across ALL providers
        # strip_code_fence = true    # strip ` + "`" + ` fences from agent output (all providers)
        # NOTE: [generation] output/strip_code_fence override any per-provider [provider.<name>] values.
  - DO NOT change any other part of exampleConfigTemplate; preserve the surrounding section banners.
```

### Implementation Patterns & Key Details

```go
// PATTERN — the bridge (pkg/stagecoach/stagecoach.go::buildDeps, after Validate()+pre-flight):
//   m is *already* Validate()-d, registry-merged, and pre-flighted (IsInstalled passed).
//   cfg is the fully-resolved value-param Config (Output always non-empty; StripCodeFence always concrete).
if cfg.Output != "" {
	o := cfg.Output           // local — avoid aliasing cfg.Output's address
	m.Output = &o             // non-nil pointer => override (manifest pointer-field semantics)
}
scf := cfg.StripCodeFence    // local — avoid aliasing cfg.StripCodeFence's address
m.StripCodeFence = &scf      // ALWAYS set; broader [generation] layer overrides per-manifest default

// WHY this works end-to-end: ParseOutput(raw, m) calls m.Resolve() (fills nils, preserves present values),
// then switches on *r.Output / checks *r.StripCodeFence. Because m.Output/.StripCodeFence are now non-nil
// cfg values, Resolve() preserves them and ParseOutput honors them. No parser change.
```

### Integration Points

```yaml
CODE:
  - file: pkg/stagecoach/stagecoach.go
    function: buildDeps
    change: "+~15 line block (comment + 6 LOC logic) between IsInstalled pre-flight and return"
    risk: ADDITIVE only. The new block executes solely on the buildDeps success path. The unknown-provider,
          Validate(), and missing-command (Issue 3) error returns are upstream of it and unchanged.

NO DATABASE / NO NEW CONFIG KEYS / NO NEW ROUTES / NO NEW DEPENDENCIES.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After the code edit (Task 1) — fix before proceeding to docs.
go build ./...            # all packages compile (Makefile `build` compiles the binary; this covers all)
go vet ./...              # vet clean
make lint                 # golangci-lint (.golangci.yml present) — zero findings
# Expected: zero errors. m.Output (*string) / m.StripCodeFence (*bool) already type-match cfg.Output (string)
# / cfg.StripCodeFence (bool) via the &local pointers, so this compiles on the first pass.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The dedicated cfg→ParseOutput end-to-end test is OWNED BY S2 (P1.M4.T1.S2).
# For S1, the gate is: the FULL existing suite stays green (the change is additive; buildDeps'
# pre-flight/Validate contract is untouched). Specifically the buildDeps-tail test must still pass:
go test -race ./pkg/stagecoach/ -run TestGenerateCommit_MissingProviderCommand_Issue3 -v
# Expected: PASS unchanged (proves the pre-flight + return path below the insertion point is intact).

# Full suite (Makefile `test`):
go test -race ./...
# Expected: all packages pass. If any FAILS, it is a regression from the edit — READ the output and fix
# (most likely cause: accidentally perturbing the pre-flight/return ordering — re-check placement).

# Coverage gate (not directly affected — pkg/stagecoach isn't in the gate set — but confirm no regression):
make coverage-gate   # >=85% on internal/{git,provider,generate,config}
```

### Level 3: Integration / End-to-End Smoke (manual proof the bridge works)

> This proves the bridge BEFORE the S2 test lands. Use a stub agent that emits fenced/raw output.

```bash
# Build the binary + stub agent (repo has cmd/stubagent and a providers/ dir with stub manifests).
go build -o bin/stagecoach ./cmd/stagecoach
go build -o bin/stubagent ./cmd/stubagent   # if a build target/helper exists; else `go build ./cmd/stubagent`

# Set up a scratch repo.
tmp=$(mktemp -d) && cd "$tmp"
git init -q && git config user.email t@t && git config user.name t
echo a > a.txt && git add a.txt && git commit -qm "init"

# CASE A — [generation] output="json" makes ParseOutput parse JSON (override a raw manifest default).
#   Write a config whose [generation] output=json and a stub provider whose manifest output=raw, emit
#   valid JSON {"subject":"..."}; confirm the committed subject is the extracted JSON field, not the raw blob.
cat > .stagecoach.toml <<'EOF'
[provider.stub]
command = "<abs path to bin/stubagent>"
prompt_delivery = "stdin"
output = "raw"                 # per-provider says raw
strip_code_fence = true
json_field = "subject"
[generation]
output = "json"                # [generation] overrides -> ParseOutput parses JSON
EOF
echo b > b.txt && git add b.txt
STAGECOACH_PROVIDER=stub ../../bin/stagecoach --dry-run   # expect the JSON-extracted subject, proving the bridge

# CASE B — git-config stripCodeFence=false disables fence stripping.
#   With the stub emitting a fenced ```message``` block and the manifest strip_code_fence=true:
git config stagecoach.stripCodeFence false   # camelCase bool key (git.go:152)
echo c > c.txt && git add c.txt
../../bin/stagecoach --provider stub --dry-run   # expect the fence retained (not stripped) -> bridge honored

# Expected: CASE A parses JSON (fellback semantics if malformed); CASE B retains the fence.
# (If CASE A/B reveal the values are still ignored, the bridge is misplaced — re-check Task 1 placement.)
cd - && rm -rf "$tmp"
```

### Level 4: Doc Validation

```bash
# Render sanity: the config init template still writes valid TOML.
go build -o /tmp/stagecoach ./cmd/stagecoach
/tmp/stagecoach config init && head -40 .stagecoach.toml   # confirm the [generation] block + new note read well
# Markdown lint (repo has .markdownlint.json):
npx --yes markdownlint-cli docs/configuration.md 2>/dev/null || true   # advisory; match existing doc style
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean.
- [ ] `go vet ./...` clean.
- [ ] `go test -race ./...` — all packages pass (incl. `TestGenerateCommit_MissingProviderCommand_Issue3`).
- [ ] `make lint` — zero findings.

### Feature Validation
- [ ] Task 1 block present in `buildDeps`, placed after `m.Validate()` AND after the `reg.IsInstalled(m)` pre-flight, before the return.
- [ ] Copy-to-local form (`o := cfg.Output`; `scf := cfg.StripCodeFence`) — NO `&cfg.*` aliasing.
- [ ] Asymmetry preserved: `Output` guarded on `!= ""`; `StripCodeFence` unconditional.
- [ ] No re-Validate(); no change to ParseOutput/manifest/registry/loaders; file.go quirk untouched.
- [ ] Level 3 smoke: `[generation] output="json"` → JSON parsed; `stagecoach.stripCodeFence false` → fence retained.
- [ ] `docs/configuration.md` affirms parsing applies + documents the two git-config keys.
- [ ] `internal/cmd/config.go` template comment accurate + "all providers" note added.

### Code Quality Validation
- [ ] Follows existing `buildDeps` plain-error / comment style.
- [ ] Additive only — no behavioral change to the error/exit paths.
- [ ] No new imports, no new dependencies, no new files.

### Documentation & Boundaries
- [ ] Mode A docs (configuration.md + config.go template) shipped here (S1); P1.M5 (Mode B sweep) will reconcile against them last.
- [ ] The dedicated cfg→ParseOutput end-to-end test is explicitly deferred to **P1.M4.T1.S2** (sibling) — do not add it here.

---

## Anti-Patterns to Avoid

- ❌ Don't `m.Output = &cfg.Output` — alias the cfg value-param's address; use a local (`o := cfg.Output; m.Output = &o`).
- ❌ Don't guard StripCodeFence on anything — it is applied unconditionally (the contract mandates the asymmetry).
- ❌ Don't re-`Validate()` the manifest after applying — not in the contract/D4; ParseOutput's switch-default already degrades unknown Output to raw.
- ❌ Don't move the block above `Validate()` or the pre-flight — the registry-merged manifest must be validated and pre-flighted first.
- ❌ Don't touch `ParseOutput`, the registry merge, the config loaders, or the `file.go` false-set quirk — all out of scope.
- ❌ Don't add env vars or CLI flags for these fields (there are none by design; loadEnv/loadFlags don't set them).
- ❌ Don't add the end-to-end test here — that is P1.M4.T1.S2's deliverable; S1 ships code + docs only.

---

## Confidence Score

**9 / 10** — This is a precisely-scoped, ~6-line additive bridge at a single, already-identified seam
(`buildDeps`), with a binding architecture decision (D4) providing the exact patch and rationale, and a
complete seam report (Part A) proving the consumer (`ParseOutput`) already reads the target fields. The
only residual uncertainty is the Level 3 smoke harness setup (stub agent + manifests), which is a
verification convenience, not a delivery risk — the unit suite (`go test -race ./...`) is the hard gate,
and the dedicated end-to-end test is explicitly S2's. Docs are small Mode A touches with exact line cites.
