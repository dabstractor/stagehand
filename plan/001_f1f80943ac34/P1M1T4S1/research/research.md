# Research Notes — P1.M1.T4.S1 (Config struct, defaults, generation-tuning fields)

## 1. What exists today (verified on disk)

- `go.mod`: `module github.com/dustin/stagecoach`, `go 1.22`, **no require block**, **no go.sum** (zero deps — T1.S1).
- `internal/config/` exists but is **empty** (T1.S1 created the dir only).
- No file imports `package config` anywhere (grep confirmed). Greenfield for this package.
- `Makefile` targets: `build` (`go build -ldflags …`), `test` (`go test -race ./...`), `coverage`, `lint` (`golangci-lint run`). No `go mod tidy` gate in Makefile (but CI/lint may enforce tidiness — keep deps genuinely used).
- Parallel item **P1.M1.T3.S5** touches **only `internal/git/`** (AddAll + StagedFileCount). **No overlap** with `internal/config/`. Real input dependency is **P1.M1.T1.S1** (the Go module).

## 2. The struct shape — the central design call (FLAT resolved struct)

The item lists a **flat** field set (Provider, Model, Timeout, AutoStageAll, Verbose, NoColor,
MaxDiffBytes, MaxMdLines, MaxDuplicateRetries, SubjectTargetChars, Output, StripCodeFence). The
architecture doc §2.2 instead shows a **nested** Config{Defaults, Provider map, Generation}. These
conflict. **Resolution: the item contract wins → flat struct.** Three independent reinforcements:

1. **`Timeout time.Duration` (item) vs `Timeout string` (arch §2.2).** go-toml/v2 cannot unmarshal
   the §16.2 string `"120s"` into a `time.Duration` field. The fact that the item mandates
   `time.Duration` is proof the struct holds **already-resolved** values, i.e. it is the OUTPUT of
   the precedence resolver, **not** a direct TOML decode target. (Arch §2.2 keeps it a `string`
   precisely because its struct IS the decode target.)
2. **§5.4 / FINDING 5 (omitempty) applies to the MERGE layer, not here.** Pointers (`*bool`) are
   needed where "field absent" must be distinguished from "field = false" — that is the overlay in
   S2–S4. A resolved struct has no absent fields, so S1 uses plain types.
3. **Consumer ergonomics.** S2–S4, M2.T3 read `cfg.Timeout`/`cfg.Verbose` directly; pointers would
   force `*cfg.Verbose` everywhere for zero S1 benefit.

⇒ **S1 Config = plain, flat, resolved. TOML tags = §16.2 snake_case leaf names. Section grouping
([defaults]/[generation]) is S2's job via a nested decode struct.** Pointers are explicitly OUT of
scope for S1.

## 3. TOML tag mapping (derived from PRD §16.2)

| Field | §16.2 section | leaf key | tag |
|---|---|---|---|
| Provider | [defaults] | provider | `toml:"provider"` |
| Model | [defaults] | model | `toml:"model"` |
| Timeout | [defaults] | timeout | `toml:"timeout"` |
| AutoStageAll | [defaults] | auto_stage_all | `toml:"auto_stage_all"` |
| Verbose | [defaults] | verbose | `toml:"verbose"` |
| NoColor | **CLI/UI only** (not in §16.2) | — | `toml:"-"` (exclude) |
| MaxDiffBytes | [generation] | max_diff_bytes | `toml:"max_diff_bytes"` |
| MaxMdLines | [generation] | max_md_lines | `toml:"max_md_lines"` |
| MaxDuplicateRetries | [generation] | max_duplicate_retries | `toml:"max_duplicate_retries"` |
| SubjectTargetChars | [generation] | subject_target_chars | `toml:"subject_target_chars"` |
| Output | [generation] | output | `toml:"output"` |
| StripCodeFence | [generation] | strip_code_fence | `toml:"strip_code_fence"` |

`NoColor` is TTY-aware / flag+env only (PRD §15.2 `--no-color` / `STAGECOACH_NO_COLOR` / `NO_COLOR`);
it has **no** config-file representation → `toml:"-"`. go-toml/v2 honors `toml:"-"` (encoding/json-
aligned semantics); the test `TestNoColorExcludedFromTOML` proves it empirically.

## 4. Defaults() values (PRD §16.1 + §16.2)

§16.1 Layer-1 explicitly pins: timeout 120s, auto_stage_all true, max_diff_bytes 300000,
max_md_lines 100, max_duplicate_retries 3, output raw, strip_code_fence true. §16.2 adds
`subject_target_chars = 50`. §16.1 does **not** pin Provider/Model → both `""`:
- `Provider == ""` ⇒ auto-detect (PRD §15.2 "auto-detected").
- `Model == ""` ⇒ use provider manifest's `default_model` (PRD §16.2 comment).

Verbose=false (PRD §15.2/§16.2). NoColor=false (TTY-aware at runtime; UI layer P1.M4.T3.S1).

`Defaults()` returns **Config by value** (item says "returning Config"; arch §2.3 shows `*Config` —
item wins). Value return = immutable resolved snapshot, no nil hazards.

## 5. File location — minor discrepancy resolved

Item: `internal/config/config.go`. PRD §16.1: `internal/config/defaults.go`. Arch §2.2 + §2.3:
`internal/config/config.go` (co-locates Config + Defaults). ⇒ **config.go** (item + arch agree).
§16.1's "defaults.go" is the conceptual Layer-1 label; landing Defaults() in config.go alongside
Config matches the arch doc example the item itself cites. One file, lower surprise.

## 6. go-toml/v2 dependency

- Target: `github.com/pelletier/go-toml/v2` **v2.2+** (arch doc header line 3).
- Add via `go get github.com/pelletier/go-toml/v2@latest` (resolves to newest v2.2.x) → writes
  `require` into go.mod + creates `go.sum`.
- **Must be genuinely used** or `go mod tidy`/lint strips it. S1 justifies it via `config_test.go`
  (`toml.Marshal(Defaults())` → asserts keys present, NoColor excluded). struct tags alone are
  string literals and do NOT count as an import.
- First build/test needs **network** to fetch the module (unlike T1.S1's zero-dep build).

## 7. Validation approach (no testing framework to introduce — Go stdlib `testing`)

Pattern from `internal/git/*_test.go`: white-box `package config`, table-free direct assertions,
`t.TempDir()` not needed here (pure data). `go test -race ./internal/config/` is the gate (matches
Makefile `test`). No new framework — Go stdlib `testing` is the established convention (T2/T3).

## 8. GOTCHA: time.Duration ↔ TOML "120s" string

- go-toml/v2 has no `TextUnmarshaler` for `time.Duration`; it marshals Duration as its int64
  **nanosecond** value, and unmarshaling a TOML integer into a `time.Duration` field is not
  guaranteed (may error or yield nanos). The §16.2 file uses the human string `"120s"`.
- ⇒ **Do NOT attempt a full marshal→unmarshal round-trip of a Config with a non-zero Timeout in
  S1.** Validate tags via marshal+key-presence only (no unmarshal). The string→Duration parse is
  S2's concern (S2's decode struct uses a string duration field + `time.ParseDuration`).
