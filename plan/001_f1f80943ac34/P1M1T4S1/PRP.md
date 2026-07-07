---
name: "P1.M1.T4.S1 — Config struct, defaults, and generation-tuning fields"
description: |
  Land the foundational `internal/config` package: a flat, fully-resolved `Config` struct (12 fields,
  TOML-tagged per PRD §16.2) and a `Defaults()` function returning the PRD §16.1 Layer-1 values. This is
  the FIRST subtask of the 4-subtask Configuration System (P1.M1.T4) and the contract every later layer
  builds on: S2 (TOML file loading), S3 (git-config reader), S4 (env+CLI + `Load()` orchestrator), and
  the provider registry (P1.M2.T3) all consume THIS struct as their resolved output type. PRD §16.1
  (resolution order) + §16.2 (config-file structure) are the spec; arch `go_ecosystem_patterns.md` §2.2
  (struct tagging) + §5.4 (omitempty limitation) are the patterns.
  ⚠️ **THE central design call — Config is a FLAT, RESOLVED struct; it is NOT a TOML decode target.** The
  item mandates a flat field set AND `Timeout time.Duration`. go-toml/v2 CANNOT unmarshal the §16.2 string
  `"120s"` into a `time.Duration` field, which proves this struct holds already-parsed values — it is the
  OUTPUT of the precedence resolver, not the input to `toml.Unmarshal`. The arch doc §2.2 example instead
  shows a NESTED `Config{Defaults, Provider map, Generation}` with `Timeout string` — that is the decode
  shape. **The item contract overrides the arch example: S1 ships the flat, plain-typed, resolved struct.**
  Three reinforcements: (1) `time.Duration` only makes sense for a resolved struct; (2) §5.4/FINDING 5
  pointer workaround is for the MERGE/overlay layer (S2–S4), where "field absent" must beat "field=false"
  — a resolved struct has no absent fields, so S1 uses plain types; (3) consumers (S2–S4, M2.T3) read
  `cfg.Timeout`/`cfg.Verbose` directly — pointers would force `*cfg.Verbose` everywhere for zero benefit.
  Consequence: S1 does NOT unmarshal the §16.2 file into `Config`. Section grouping ([defaults]/
  [generation]) and the `"120s"`→Duration parse are S2's job (S2 introduces its own nested decode struct
  with string durations, then merges field-by-field into this plain `Config`).
  ⚠️ **THE second design call — TOML tags are §16.2 leaf names; NoColor is `toml:"-"`.** The flat struct
  carries snake_case leaf tags (`provider`, `auto_stage_all`, `max_diff_bytes`, …) so a flat round-trip
  uses the file vocabulary. `NoColor` is CLI/UI-only (PRD §15.2: `--no-color`/`STAGECOACH_NO_COLOR`/
  `NO_COLOR`, TTY-aware) and has NO config-file key → tagged `toml:"-"` so it never leaks into a file.
  go-toml/v2 honors `toml:"-"` (encoding/json-aligned); a test proves it empirically.
  ⚠️ **THE third design call — Provider/Model default to `""`, NOT "pi".** PRD §16.1 Layer-1 does NOT pin a
  provider or model. `Provider==""` ⇒ auto-detect (PRD §15.2 "auto-detected"); `Model==""` ⇒ use the
  provider manifest's `default_model` (PRD §16.2 comment). §16.2's `provider = "pi"` is the USER example
  file, not the built-in default. Defaults() leaves both empty.
  ⚠️ **THE go-toml/v2 dependency MUST be genuinely used.** struct tags are string literals and do NOT
  count as an import. `go mod tidy`/lint would strip an unused `require`. S1 justifies the dep via
  `config_test.go` (`toml.Marshal(Defaults())` → assert expected keys present + NoColor absent). First
  build/test needs NETWORK to fetch the module (unlike T1.S1's zero-dep build).
  ⚠️ **File-location note.** Item says `internal/config/config.go`; PRD §16.1 says `defaults.go`. Arch
  §2.2+§2.3 co-locate Config + Defaults() in `config.go`. ⇒ **one file, `config.go`** (item + arch agree;
  §16.1's "defaults.go" is the conceptual Layer-1 label). No Makefile/other-file changes.
  Deliverable: `internal/config/config.go` (Config struct + `Defaults()`), `internal/config/config_test.go`
  (Defaults assertions + TOML marshal/NoColor-exclusion test), and `go.mod`/`go.sum` updated with
  `github.com/pelletier/go-toml/v2` v2.2+. INPUT = Go module from P1.M1.T1.S1. Touches ONLY
  `internal/config/` + `go.mod`/`go.sum`. Parallel item P1.M1.T3.S5 touches only `internal/git/` — no
  overlap. This is the FIRST P1.M1.T4 subtask — opens the Configuration System consumed by S2–S4 and M2.T3.
---

## Goal

**Feature Goal**: Define the single resolved-configuration type for all of Stagecoach — a flat
`Config` struct whose 12 fields carry the PRD §16.2 scalars (provider, model, timeout, auto-stage,
verbose, no-color, diff/md/retry caps, subject target, output mode, fence stripping) as plain Go
types (`time.Duration` for Timeout), plus a `Defaults()` function returning the PRD §16.1 Layer-1
values. This type is the contract produced by the 7-layer precedence resolver (FR34) and read by
every downstream consumer.

**Deliverable**:
1. **CREATE** `internal/config/config.go` (`package config`) —
   (a) `Config` struct: exactly the 12 fields below, with §16.2 snake_case `toml:` tags and
       `NoColor` tagged `toml:"-"`. Field order grouped as [defaults] → CLI-only → [generation]
       with a comment mapping each group to its §16.2 section.
   (b) `func Defaults() Config` returning the §16.1 Layer-1 values (by value, not pointer).
   (c) `import "time"` (only import; struct tags need no toml import in this file).
2. **CREATE** `internal/config/config_test.go` (`package config`, white-box) —
   (a) `TestDefaults`: asserts all 12 fields of `Defaults()` equal the §16.1/§16.2 expected values.
   (b) `TestTOMLMarshalKeysAndNoColorExclusion`: `toml.Marshal(Defaults())` succeeds; all 11
       TOML-tagged keys appear; `no_color`/`NoColor` does NOT appear (proves `toml:"-"` + uses dep).
3. **MODIFY** `go.mod` — add `require github.com/pelletier/go-toml/v2 <v2.2.x>`; **create**
   `go.sum` (via `go get`). No `toolchain` line added; `go 1.22` unchanged.

No other files touched. **No Makefile change.** No nested decode struct, no `Load()`, no file/env/
CLI reading — all deferred to S2–S4.

**Success Definition**: `go build ./...`, `go vet ./...`, and `gofmt -l internal/config/` are clean;
`go mod tidy` is a no-op (deps genuinely used); `go test -race ./internal/config/` passes (both new
test functions green) and the full suite `go test -race ./...` stays green; `Defaults()` returns the
exact §16.1 values (Timeout=120s, AutoStageAll=true, MaxDiffBytes=300000, MaxMdLines=100,
MaxDuplicateRetries=3, SubjectTargetChars=50, Output="raw", StripCodeFence=true, Provider="",
Model="", Verbose=false, NoColor=false); marshaling `Defaults()` emits the 11 §16.2 leaf keys and
omits `no_color`; `go.mod` declares `github.com/pelletier/go-toml/v2` ≥ v2.2 with a matching
`go.sum`.

## User Persona

**Target User**: Downstream Stagecoach subtasks — the config loaders (P1.M1.T4.S2 TOML, S3 git-config,
S4 env+CLI+`Load()`), the provider registry (P1.M2.T3), and ultimately the generation pipeline
(P1.M3) + CLI (P1.M4.T1) that read the resolved config. Transitively US8 (configuration & precedence,
FR34) and every user story that depends on a tunable knob (timeout, caps, output mode).

**Use Case**: Every precedence layer produces a `Config`; every consumer reads `cfg.<Field>`. This
subtask fixes the type and the Layer-1 baseline so S2–S4 have a concrete target to merge into.

**User Journey**: (internal API, no end-user surface yet) `cfg := config.Defaults()` → (S2–S4)
overlay file/git/env/flag values onto `cfg` → consumers read resolved `cfg.Timeout`, `cfg.Model`, …

**Pain Points Addressed**: Removes "what shape is resolved config / where do defaults live / is
Timeout a string or Duration" ambiguity for S2–S4 and M2.T3 by fixing one plain struct + baseline now.

## Why

- **Contract for the whole Configuration System.** S2 (TOML), S3 (git-config), S4 (env+CLI+`Load()`)
  all MERGE into and RETURN this `Config`. Landing the type + baseline first lets those layers be
  implemented and tested against a stable target (no churn later).
- **Locks the resolved representation.** A flat, plain-typed, value-returning struct makes every
  consumer a one-liner (`cfg.MaxDiffBytes`) and keeps the §5.4 pointer complexity where it belongs
  (the overlay layer), not on the hot read path.
- **Pins Layer-1 defaults from the spec.** PRD §16.1 enumerates the built-in defaults; encoding them
  as `Defaults()` makes "higher layer wins" (FR34) mechanically correct from layer 1 upward.
- **Introduces the first external dependency deliberately.** go-toml/v2 lands here (used by the test)
  so S2's file loader can import it without a separate dependency-add subtask. (Cobra lands later in
  P1.M4.T1 — NOT here.)
- **No user-facing surface change** (PRD "DOCS: none yet") — public config docs arrive with
  `config init` (P1.M4.T1.S4, Mode A).

## What

A compiled `internal/config` package exporting `Config` (12 plain-typed, TOML-tagged fields) and
`Defaults() Config` (PRD §16.1 values), with `go-toml/v2` declared in `go.mod`/`go.sum`. No parsing,
no file I/O, no precedence logic, no provider map (provider manifests are P1.M2.T1 — out of scope).

### Success Criteria

- [ ] `internal/config/config.go` exists, `package config`, `import "time"`, no other imports.
- [ ] `Config` struct has exactly these 12 fields, types, and tags (gofmt-aligned):
      `Provider string toml:"provider"` · `Model string toml:"model"` ·
      `Timeout time.Duration toml:"timeout"` · `AutoStageAll bool toml:"auto_stage_all"` ·
      `Verbose bool toml:"verbose"` · `NoColor bool toml:"-"` ·
      `MaxDiffBytes int toml:"max_diff_bytes"` · `MaxMdLines int toml:"max_md_lines"` ·
      `MaxDuplicateRetries int toml:"max_duplicate_retries"` ·
      `SubjectTargetChars int toml:"subject_target_chars"` · `Output string toml:"output"` ·
      `StripCodeFence bool toml:"strip_code_fence"`.
- [ ] `func Defaults() Config` returns: `Provider:""`, `Model:""`, `Timeout:120*time.Second`,
      `AutoStageAll:true`, `Verbose:false`, `NoColor:false`, `MaxDiffBytes:300000`,
      `MaxMdLines:100`, `MaxDuplicateRetries:3`, `SubjectTargetChars:50`, `Output:"raw"`,
      `StripCodeFence:true`.
- [ ] `Defaults()` returns by VALUE (`Config`, not `*Config`).
- [ ] `go.mod` has `require github.com/pelletier/go-toml/v2 <v2.2.x>`; `go.sum` exists and matches.
- [ ] `config_test.go` has `TestDefaults` + `TestTOMLMarshalKeysAndNoColorExclusion`, both passing.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/config/`, `go mod tidy` (diff-empty) clean.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge of this repo can implement this from: the field/tag
table below, the `Defaults()` value table, the arch §2.2/§5.4 references, and the two test specs. No
git/provider/generation knowledge required — this subtask is pure data + one dependency.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- url: https://github.com/pelletier/go-toml/v2
  why: confirms struct-tag semantics (snake_case keys), `toml:"-"` field exclusion, and that
       `omitempty` is intentionally unsupported (use pointers/custom marshaler — NOT needed in S1).
  critical: go-toml/v2 marshals time.Duration as int64 nanoseconds and has no TextUnmarshaler for
       Duration — so do NOT round-trip a non-zero Timeout through marshal→unmarshal in S1 (see gotchas).

- file: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md
  section: "2.2 Struct Definitions with go-toml/v2 Tags" (lines ~331-410) and "2.3 Layer 1: Built-in Defaults"
  why: canonical pattern for toml-tagged config structs + a Defaults() constructor. NOTE the arch
       example uses a NESTED Config{Defaults,Provider map,Generation} with `Timeout string` — that is the
       DECODE shape; S1 ships the FLAT resolved shape per the item contract (see design call #1). Use the
       arch for tag STYLE, not for the struct layout.
  pattern: snake_case `toml:"..."` tags; `func Defaults() …` returning a populated struct literal.

- file: plan/001_f1f80943ac34/architecture/go_ecosystem_patterns.md
  section: "5.4 Omitempty Workarounds" (lines ~1254-1325) + FINDING 5 in critical_findings.md
  why: explains the pointer workaround for "field absent vs zero". CRITICAL SCOPE NOTE: this is for the
       MERGE/overlay layer (S2-S4), NOT for S1's resolved struct. S1 must use PLAIN types (no *bool).
  pattern: pointers (*string,*bool) or map[string]any decode — OUT OF SCOPE for S1; referenced only so the
       implementer knows why Config is deliberately pointer-free here.

- file: PRD.md
  section: "16.1 Resolution order (FR34)" and "16.2 Full config file example"
  why: §16.1 gives the Layer-1 default values; §16.2 gives the TOML key names + section grouping
       ([defaults]/[generation]) and the Provider/Model "" semantics.
  critical: §16.1 does NOT pin provider/model (both default ""); NoColor is NOT in §16.2 (CLI-only → toml:"-").

- file: internal/git/git_test.go
  why: established test convention for this repo — white-box `package <pkg>`, stdlib `testing` only,
       direct assertions, no table-driven boilerplate required. Mirror its style for config_test.go.
  pattern: `package git` (white-box); `t.Helper()` for helpers; `t.Errorf` for field-by-field assertions.

- file: plan/001_f1f80943ac34/P1M1T1S1/PRP.md
  why: confirms the INPUT contract — module `github.com/dustin/stagecoach`, `go 1.22`, `internal/config/`
       dir already exists (empty). No deps before this subtask (go.sum absent).
```

### Current Codebase tree (relevant slice)

```bash
go.mod                          # module github.com/dustin/stagecoach ; go 1.22 ; NO require
# (go.sum absent — zero deps)
internal/
  config/                       # EXISTS, EMPTY (T1.S1 created the dir)
  git/                          # populated by T2/T3 (RevParseHEAD…AddAll, StagedFileCount)
cmd/stagecoach/main.go           # `package main; func main(){}` stub (T1.S1)
Makefile                        # build/test/coverage/lint/clean/help (T1.S2)
```

### Desired Codebase tree with files to be added

```bash
internal/
  config/
    config.go                   # NEW — Config struct (12 fields, toml tags) + Defaults() ; import "time"
    config_test.go              # NEW — TestDefaults + TestTOMLMarshalKeysAndNoColorExclusion
go.mod                          # MODIFIED — +require github.com/pelletier/go-toml/v2 <v2.2.x>
go.sum                          # NEW — module hash for go-toml/v2 (created by `go get`)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: go-toml/v2 has NO omitempty in struct tags (FINDING 5 / arch §5.4). The pointer/map
// workaround is for the MERGE layer (S2-S4) where "absent" must beat "false". S1's Config holds
// RESOLVED values (every field concrete) → use PLAIN types (bool, int, string, time.Duration).
// Do NOT add *bool/*int here — it would force *cfg.Verbose on every consumer for zero S1 gain.

// CRITICAL: go-toml/v2 cannot unmarshal the §16.2 string "120s" into a time.Duration field, and
// marshals a Duration as int64 nanoseconds. => Config is the RESOLVED type, NOT a decode target.
// The "120s"→Duration parse + [defaults]/[generation] section decode are S2's job (separate nested
// decode struct with a string duration field + time.ParseDuration). In S1, validate tags via
// toml.Marshal + key-presence ONLY — do NOT round-trip a non-zero Timeout through unmarshal.

// CRITICAL: an unused `require` in go.mod is stripped by `go mod tidy`. struct tags are string
// literals and do NOT count as an import. => go-toml/v2 MUST be imported in config_test.go
// (toml.Marshal) so the dependency is genuinely used and `go mod tidy` is a no-op.

// CRITICAL: go-toml/v2 marshals EVERY tagged field (no omitempty) — including Provider="" and
// Model="" (emitted as `provider = ""`). That is expected and fine for the marshal-presence test.
// NoColor is tagged `toml:"-"` so it is the ONLY field excluded from TOML.

// MINOR: Defaults() returns Config BY VALUE (item: "returning Config"). Arch §2.2/§2.3 shows *Config;
// the item contract wins. A value return is correct — Config is an immutable resolved snapshot.

// MINOR: PRD §16.1 names the file `defaults.go`; arch §2.2/§2.3 and the item say `config.go`.
// config.go wins (co-locates Config + Defaults, matches the arch example the item cites). One file.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/config/config.go
package config

import "time"

// Config is the fully-resolved Stagecoach configuration: the single value produced by the 7-layer
// precedence resolver (PRD §16.1, FR34) and read by every consumer — the TOML/git/env/CLI loaders
// (P1.M1.T4.S2-S4), the provider registry (P1.M2.T3), and the generation pipeline.
//
// DESIGN CALL: flat + plain-typed + RESOLVED. Every field holds a concrete value (Timeout is already
// a time.Duration). This struct is NOT unmarshaled directly from the §16.2 file: that file uses
// [defaults]/[generation] subtables and string durations ("120s"). The loaders (S2-S4) decode into
// their own intermediate structs (pointer or map-based — see arch §5.4) and merge field-by-field
// INTO this plain Config. Keeping Config plain means consumers read cfg.Timeout / cfg.Verbose with
// zero dereferencing. The toml tags use §16.2 snake_case leaf names; section grouping is S2's concern.
type Config struct {
	// [defaults] (PRD §16.2)
	Provider     string        `toml:"provider"`      // "" => auto-detect (PRD §15.2)
	Model        string        `toml:"model"`         // "" => provider manifest default_model
	Timeout      time.Duration `toml:"timeout"`       // generation timeout; Defaults: 120s
	AutoStageAll bool          `toml:"auto_stage_all"` // git add -A when nothing staged (PRD §9.4)
	Verbose      bool          `toml:"verbose"`       // print resolved cmd, raw output, retries

	// CLI / UI only — NOT in the §16.2 config file (PRD §15.2: --no-color / STAGECOACH_NO_COLOR / NO_COLOR)
	NoColor bool `toml:"-"` // TTY-aware at runtime; set by UI layer (P1.M4.T3.S1)

	// [generation] (PRD §16.2)
	MaxDiffBytes        int    `toml:"max_diff_bytes"`        // byte cap on non-markdown diff section
	MaxMdLines          int    `toml:"max_md_lines"`          // per-file line cap for markdown diffs
	MaxDuplicateRetries int    `toml:"max_duplicate_retries"` // re-gen attempts on duplicate subject
	SubjectTargetChars  int    `toml:"subject_target_chars"`  // target subject length for truncation
	Output              string `toml:"output"`                // "raw" | "json"
	StripCodeFence      bool   `toml:"strip_code_fence"`      // strip ``` fences from agent output
}

// Defaults returns the built-in Layer-1 configuration (PRD §16.1): timeout 120s, auto_stage_all
// true, max_diff_bytes 300000, max_md_lines 100, max_duplicate_retries 3, output "raw",
// strip_code_fence true, subject_target_chars 50. Provider and Model are "" (Layer 1 does not pin
// them): empty Provider => auto-detect (PRD §15.2); empty Model => use the manifest default_model
// (PRD §16.2). Verbose/NoColor are false (NoColor is ultimately TTY-aware in the UI layer).
//
// Returned BY VALUE: Config is an immutable resolved snapshot after Load(); a value return avoids
// nil-pointer hazards and lets callers copy freely.
func Defaults() Config {
	return Config{
		Provider:            "",
		Model:               "",
		Timeout:             120 * time.Second,
		AutoStageAll:        true,
		Verbose:             false,
		NoColor:             false,
		MaxDiffBytes:        300000,
		MaxMdLines:          100,
		MaxDuplicateRetries: 3,
		SubjectTargetChars:  50,
		Output:              "raw",
		StripCodeFence:      true,
	}
}
```

> The struct literal above is gofmt-canonical (run `gofmt -w` to confirm alignment of the `toml:"…"`
> tag column — gofmt aligns adjacent tagged fields; do not hand-align).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD go-toml/v2 dependency
  - RUN: `go get github.com/pelletier/go-toml/v2@latest`
  - VERIFY: `require github.com/pelletier/go-toml/v2 <v2.2.x>` appears in go.mod AND go.sum is created.
  - WHY FIRST: config_test.go (Task 3) imports it; the package must compile+resolve before `go test`.
  - NETWORK: this is the FIRST external dep — first `go get`/`go build` needs network (module cache fetch).
  - CONSTRAINT: resolved version MUST be >= v2.2 (arch doc target). If @latest pins a v3+ (unlikely),
    pin explicitly: `go get github.com/pelletier/go-toml/v2@v2.2.3` (or newest v2.2.x).

Task 2: CREATE internal/config/config.go
  - IMPLEMENT: Config struct (12 fields, tags per the Data Models block) + Defaults() (values per §16.1/§16.2).
  - IMPORTS: `import "time"` ONLY. Do NOT import go-toml here (struct tags are string literals; the
    dep is imported in the TEST file, Task 3).
  - NAMING: CamelCase struct + exported fields (Go convention); snake_case only inside toml:"..." tags.
  - PLACEMENT: single file internal/config/config.go (Config + Defaults co-located — see file-location note).
  - GOTCHA: gofmt the file — let it align the toml tag column; do not hand-format.

Task 3: CREATE internal/config/config_test.go
  - IMPLEMENT: `package config` (white-box), stdlib `testing` + `strings`, and `github.com/pelletier/go-toml/v2`.
  - TEST A TestDefaults: call `c := Defaults()`; assert each of the 12 fields via t.Errorf (see spec below).
    Cover BOTH groups: [defaults] (incl. Provider=="" , Model=="" , Timeout==120*time.Second) and
    [generation] (incl. SubjectTargetChars==50), plus NoColor==false / Verbose==false.
  - TEST B TestTOMLMarshalKeysAndNoColorExclusion: `data, err := toml.Marshal(Defaults())` => err nil;
    assert the marshaled string contains each of the 11 leaf keys as `"<key> ="` (provider, model,
    timeout, auto_stage_all, verbose, max_diff_bytes, max_md_lines, max_duplicate_retries,
    subject_target_chars, output, strip_code_fence); then set `c.NoColor = true`, re-marshal, and
    assert the output contains NEITHER `no_color` NOR `NoColor` (proves toml:"-" exclusion + uses dep).
  - DO NOT write a marshal->unmarshal round-trip of Defaults() (Duration marshals to nanos; see gotchas).
  - PATTERN: mirror internal/git/git_test.go style (white-box package, direct t.Errorf assertions).

Task 4: VERIFY (no file change)
  - RUN the full Validation Loop (Level 1 + Level 2 below). Fix until green. `go mod tidy` must be a no-op.
```

### Implementation Patterns & Key Details

```go
// config_test.go — TestDefaults (assert EVERY field; do not rely on reflect.DeepEqual for clarity,
// but a DeepEqual fallback against a hand-built literal is acceptable as a second assertion).
func TestDefaults(t *testing.T) {
	c := Defaults()
	// [defaults] (PRD §16.1 does not pin provider/model => "")
	if c.Provider != "" { t.Errorf("Provider = %q, want %q", c.Provider, "") }
	if c.Model != "" { t.Errorf("Model = %q, want %q", c.Model, "") }
	if c.Timeout != 120*time.Second { t.Errorf("Timeout = %v, want 120s", c.Timeout) }
	if !c.AutoStageAll { t.Errorf("AutoStageAll = false, want true") }
	if c.Verbose { t.Errorf("Verbose = true, want false") }
	// CLI/UI-only
	if c.NoColor { t.Errorf("NoColor = true, want false") }
	// [generation] (PRD §16.1 + subject_target_chars=50 from §16.2)
	if c.MaxDiffBytes != 300000 { t.Errorf("MaxDiffBytes = %d, want 300000", c.MaxDiffBytes) }
	if c.MaxMdLines != 100 { t.Errorf("MaxMdLines = %d, want 100", c.MaxMdLines) }
	if c.MaxDuplicateRetries != 3 { t.Errorf("MaxDuplicateRetries = %d, want 3", c.MaxDuplicateRetries) }
	if c.SubjectTargetChars != 50 { t.Errorf("SubjectTargetChars = %d, want 50", c.SubjectTargetChars) }
	if c.Output != "raw" { t.Errorf("Output = %q, want %q", c.Output, "raw") }
	if !c.StripCodeFence { t.Errorf("StripCodeFence = false, want true") }
}

// config_test.go — TestTOMLMarshalKeysAndNoColorExclusion (proves tags + justifies the dep)
func TestTOMLMarshalKeysAndNoColorExclusion(t *testing.T) {
	data, err := toml.Marshal(Defaults())
	if err != nil { t.Fatalf("toml.Marshal(Defaults()) err = %v", err) }
	s := string(data)
	for _, key := range []string{
		"provider", "model", "timeout", "auto_stage_all", "verbose",
		"max_diff_bytes", "max_md_lines", "max_duplicate_retries",
		"subject_target_chars", "output", "strip_code_fence",
	} {
		if !strings.Contains(s, key+" =") { // go-toml/v2 emits `key = value` with spaces
			t.Errorf("marshaled TOML missing key %q:\n%s", key, s)
		}
	}
	// NoColor is toml:"-" and must NEVER appear in a config file (PRD §15.2: flag/env only).
	nc := Defaults()
	nc.NoColor = true
	data2, err := toml.Marshal(nc)
	if err != nil { t.Fatalf("toml.Marshal(NoColor=true) err = %v", err) }
	if strings.Contains(string(data2), "no_color") || strings.Contains(string(data2), "NoColor") {
		t.Errorf("NoColor leaked into TOML (toml:\"-\" not honored):\n%s", data2)
	}
}
```

> If `TestTOMLMarshalKeysAndNoColorExclusion` fails on the `no_color` assertion, go-toml/v2 is NOT
> honoring `toml:"-"` (it does, per its encoding/json-aligned tag semantics). Verify against the
> go-toml/v2 README; the documented exclusion tag is `toml:"-"`. Do not "fix" it by removing the tag.

### Integration Points

```yaml
GO MODULE (go.mod / go.sum):
  - add:     "require github.com/pelletier/go-toml/v2 <v2.2.x>" (via `go get`)
  - create:  go.sum (module hash) — first dependency in the repo
  - preserve: `module github.com/dustin/stagecoach`, `go 1.22`; NO `toolchain` line added

DOWNSTREAM CONTRACTS (do NOT implement here — just honor the shape they will consume):
  - P1.M1.T4.S2 (TOML load):  will define a nested decode struct (Defaults/Generation/Provider map)
        with string `timeout`, decode §16.2 files, then merge field-by-field into THIS Config.
  - P1.M1.T4.S3 (git-config): will read `stagecoach.*` keys and overlay onto THIS Config.
  - P1.M1.T4.S4 (env+CLI+Load): will apply STAGECOACH_* env + cobra flags onto THIS Config; defines Load().
  - P1.M2.T3 (provider registry): reads cfg.Provider/cfg.Model to select + override a manifest.
  => Config field names/types/tags are now FROZEN for downstream. Do not rename after this subtask.

NO DATABASE / NO ROUTES / NO CONFIG-FILE WRITING (config init is P1.M4.T1.S4).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After Task 1 (go get) + Task 2 (config.go) + Task 3 (config_test.go):
go build ./...                       # Compiles the whole module incl. the new package. Expect exit 0.
gofmt -w internal/config/            # Auto-align the toml tag column; then verify:
test -z "$(gofmt -l internal/config/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
go vet ./internal/config/            # (and `go vet ./...` project-wide) Expect zero diagnostics.
go mod tidy && git diff --exit-code go.mod go.sum   # tidy MUST be a no-op (dep is genuinely used).
# Expected: all clean. If `go mod tidy` mutates go.mod, the go-toml import in the test is missing/unused.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The two new test functions (white-box, stdlib testing):
go test -race ./internal/config/ -v
# Expected: PASS — TestDefaults + TestTOMLMarshalKeysAndNoColorExclusion.

# Full suite must stay green (no regression in internal/git or elsewhere):
go test -race ./...
# Expected: all packages PASS.
```

### Level 3: Integration Testing (System Validation)

```bash
# No runtime integration to exercise yet (no Load(), no file I/O, no CLI). Validate the build artifact
# and the dependency resolution end-to-end:
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"   # main.go stub still links.
go list -m github.com/pelletier/go-toml/v2                          # prints the resolved v2.2.x version.
grep -q '^require github.com/pelletier/go-toml/v2' go.mod && echo "require present"
test -f go.sum && echo "go.sum present"
# Expected: binary builds; go list prints a v2.2.x; require line + go.sum both present.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Smoke-check the TOML output shape by hand (sanity for the S2 author who will consume these tags):
go test ./internal/config/ -run TestTOMLMarshalKeysAndNoColorExclusion -v
# Inspect: the marshaled TOML is FLAT (all keys at root) with snake_case names — this is the resolved
# flat shape, NOT the §16.2 [defaults]/[generation] sectioned file. Sectioning is S2's decode struct.
# Confirm: Provider/Model appear as `provider = ""` / `model = ""` (omitempty unsupported — expected).
# Confirm: NO `no_color` key anywhere (toml:"-" honored).
# (Optional) Lint if golangci-lint is installed:
golangci-lint run ./internal/config/ 2>/dev/null || echo "golangci-lint not installed (Makefile lint gate is project-wide; run `make lint` in CI)."
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/config/`, `go mod tidy` (no-op).
- [ ] Level 2 green: `go test -race ./internal/config/ -v` (both tests) AND `go test -race ./...`.
- [ ] Level 3: binary builds; `go list -m github.com/pelletier/go-toml/v2` prints v2.2.x; go.mod+go.sum present.

### Feature Validation

- [ ] `Config` has exactly the 12 fields/types/tags listed in Success Criteria.
- [ ] `Defaults()` returns the exact §16.1/§16.2 values (incl. Provider="" , Model="" , Timeout=120s,
      SubjectTargetChars=50, Output="raw", StripCodeFence=true, AutoStageAll=true).
- [ ] `Defaults()` returns by value (`Config`, not `*Config`).
- [ ] `NoColor` is `toml:"-"` and is absent from marshaled TOML (test proves it).
- [ ] The 11 TOML-tagged keys all appear in `toml.Marshal(Defaults())` output.
- [ ] go-toml/v2 ≥ v2.2 declared and genuinely imported (test), so `go mod tidy` is stable.

### Code Quality Validation

- [ ] Follows repo conventions: white-box `package config`, stdlib `testing`, `t.Errorf` assertions
      (mirrors `internal/git/git_test.go`).
- [ ] File placement matches the desired tree (single `config.go` + `config_test.go`).
- [ ] No pointers on Config (plain resolved types — §5.4 workaround deferred to S2-S4 overlay layer).
- [ ] No premature scope: no `Load()`, no file/env/CLI reading, no provider map, no nested decode struct.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Doc comments on `Config` and `Defaults()` explain the flat-resolved design call + downstream contract.
- [ ] No new env vars / no user-facing docs (PRD "DOCS: none yet" — public config docs come with config init, P1.M4.T1.S4).
- [ ] `go.mod`/`go.sum` changes are the ONLY non-`internal/config/` files touched.

---

## Anti-Patterns to Avoid

- ❌ Don't make `Config` nested (`Config{Defaults, Generation, …}`) or add a `Provider` map — the item
  mandates a flat field set, and provider manifests are P1.M2.T1 (out of scope). The arch §2.2 nested
  example is the DECODE shape for S2, not S1's resolved type.
- ❌ Don't use pointer types (`*bool`, `*int`) on Config. The §5.4 omitempty/overlay workaround belongs
  to the MERGE layer (S2-S4), where "absent vs zero" matters. A resolved struct has no absent fields.
- ❌ Don't store `Timeout` as a `string` "to make TOML easy" — the item mandates `time.Duration`. The
  string→Duration parse is S2's concern (its decode struct holds the string).
- ❌ Don't unmarshal the §16.2 file (or a marshaled Config) directly into `Config` — section grouping +
  string durations require S2's nested decode struct. Validate tags via marshal+key-presence only.
- ❌ Don't pin Provider/Model to "pi"/a model in `Defaults()` — §16.1 Layer-1 leaves both `""` (auto-detect
  / manifest default). §16.2's `provider = "pi"` is the user example file, not the built-in default.
- ❌ Don't add `go-toml/v2` to go.mod without importing it somewhere — `go mod tidy` will strip an unused
  require. The test's `toml.Marshal` is the genuine, justified usage.
- ❌ Don't import go-toml in `config.go` itself (struct tags are string literals) — import it in the test.
- ❌ Don't tag `NoColor` with a real key or leave it untagged — it is CLI/UI-only and must be `toml:"-"`.
- ❌ Don't return `*Config` from `Defaults()` — the item says `Config` (by value).
- ❌ Don't skip `go mod tidy`/`gofmt`/`go vet` — they are cheap gates that catch the dep-import gap and
  formatting drift before downstream subtasks freeze on this struct's shape.
