# S1 Implementation Notes — config struct fields + Defaults() + fileGeneration struct

> Scope: P1.M1.T1.S1. Add `TokenLimit` + `DiffContext` to `config.Config`, the `fileGeneration` decode
> struct, and seed them in `Defaults()`. Pure scaffolding — nothing reads them yet (S2 wires
> materialize/overlay; S3 git-config; S4 bootstrap/docs). Verified against live source 2026-07-04.

## 1. Exact current edit targets (3 spots)

### a. config.Config struct — internal/config/config.go:77-78 (the MaxDiffBytes/MaxMdLines pair)
```go
	MaxDiffBytes        int `toml:"max_diff_bytes"`        // byte cap on non-markdown diff section
	MaxMdLines          int `toml:"max_md_lines"`          // per-file line cap for markdown diffs
	MaxDuplicateRetries int `toml:"max_duplicate_retries"` // re-gen attempts on duplicate subject
```
Fields are FLAT on Config (not a nested struct — confirmed by touchmap §3). INSERT the two new fields
right after MaxMdLines (line 78), before MaxDuplicateRetries — keeps the diff-capture caps grouped:
```go
	MaxDiffBytes        int `toml:"max_diff_bytes"`        // byte cap on non-markdown diff section
	MaxMdLines          int `toml:"max_md_lines"`          // per-file line cap for markdown diffs
	TokenLimit          int `toml:"token_limit"`           // FR3d holistic token cap (0 = unset ⇒ legacy caps); consumed by S2/S4
	DiffContext         int `toml:"diff_context"`          // FR3f reduced diff context (0–3; default 1); consumed by S2/S4
	MaxDuplicateRetries int `toml:"max_duplicate_retries"` // re-gen attempts on duplicate subject
```

### b. fileGeneration struct — internal/config/file.go:49-62
```go
type fileGeneration struct {
	MaxDiffBytes        int      `toml:"max_diff_bytes"`
	MaxMdLines          int      `toml:"max_md_lines"`
	MaxDuplicateRetries int      `toml:"max_duplicate_retries"`
	...
}
```
Add the SAME two fields after MaxMdLines (line 51), plain `int` (see §3 below for why NOT *int in S1):
```go
	MaxDiffBytes        int      `toml:"max_diff_bytes"`
	MaxMdLines          int      `toml:"max_md_lines"`
	TokenLimit          int      `toml:"token_limit"`   // FR3d — plumbed in S2 (materialize/overlay)
	DiffContext         int      `toml:"diff_context"`  // FR3f — becomes *int in S2 (0-vs-unset); plain int here
	MaxDuplicateRetries int      `toml:"max_duplicate_retries"`
```

### c. Defaults() — internal/config/config.go:168-169
```go
		MaxDiffBytes:        300000,
		MaxMdLines:          100,
		MaxDuplicateRetries: 3,
```
INSERT after MaxMdLines (line 169):
```go
		MaxDiffBytes:        300000,
		MaxMdLines:          100,
		TokenLimit:          0, // FR3d: 0 = unset ⇒ legacy per-section caps (max_diff_bytes/max_md_lines) apply unchanged
		DiffContext:         1, // FR3f: reduced context (-U1) default; 0 = changed-lines-only, 3 = git default
		MaxDuplicateRetries: 3,
```

## 2. Defaults values (per FR3d/FR3f, confirmed by PRD §16.1)

- `TokenLimit: 0` — FR3d: "default `0` = unset". When 0, the legacy per-section caps (MaxDiffBytes /
  MaxMdLines) apply unchanged. §16.1 layer-1 lists "token_limit 0 (unset ⇒ legacy per-section caps; FR3d)".
- `DiffContext: 1` — FR3f: "default `-U1`". §16.1 layer-1 lists "diff_context 1 (FR3f)". Range 0–3
  (0 = changed-lines-only; 3 = git default). Config.DiffContext holds the resolved concrete int.

## 3. CRITICAL: Config.DiffContext is a plain int in S1; the *int disambiguation is S2

For `max_diff_bytes`/`max_md_lines`, fileGeneration uses plain `int` and materialize/overlay guard with
`!= 0` (file.go:210-220, 312-322) — valid because 0 bytes/lines is never a meaningful user value.

For `diff_context`, **0 IS meaningful** (FR3f: "0 = changed lines only"). So the `!= 0` guard cannot
distinguish "user set 0" from "user omitted the key". S2 (P1.M1.T1.S2: "materialize + overlay field-merge
(with DiffContext *int pointer)") resolves this by making **fileGeneration.DiffContext a `*int`** (nil =
omitted; non-nil = explicit, even 0).

**S1 keeps BOTH Config.DiffContext and fileGeneration.DiffContext as plain `int`** (the contract NOTE:
"this subtask keeps it a plain int on Config"). S1 does NOT pre-empt S2's *int design on the file struct,
and S1 does NOT add the materialize/overlay lines (that's S2's whole job). The fields are dead
(unconsumed) after S1 — Config.DiffContext always reads 1 (Defaults) until S2 wires the file→Config path.

## 4. No existing test breaks (verified)

- config_test.go TestDefaults (line 11): field-specific assertions (`c.MaxDiffBytes != 300000`,
  `c.MaxMdLines != 100`), NOT an exhaustive DeepEqual. Adding fields does NOT break it.
- file_test.go: field-specific (`cfg.MaxDiffBytes != 12345`, `dst.MaxMdLines != 100`). NOT exhaustive.
- No `reflect.DeepEqual(Defaults(), Config{...})` or fileGeneration field-count assertion exists.
- `go test ./internal/config/` is GREEN.
=> Adding the two fields is non-breaking. RECOMMENDED: extend TestDefaults with `TokenLimit==0` and
`DiffContext==1` assertions (matches the convention that every Defaults() field is asserted) — pins the
seed values so a future edit can't silently change them.

## 5. gofmt alignment

The struct fields are column-aligned (the type column pads to the longest name in the block,
~`MaxDuplicateRetries`/`SubjectTargetChars` = 18 chars). `TokenLimit` (10) / `DiffContext` (11) are
shorter; gofmt realigns the block automatically. RUN `gofmt -w` after the edits — do NOT hand-align.

## 6. Scope discipline (S1 vs S2/S3/S4)

S1 = the 3 struct/seed additions (Config struct, fileGeneration struct, Defaults()) + the recommended
TestDefaults assertions. NOTHING ELSE.
- NOT S1: materialize/overlay field-merge (S2 = P1.M1.T1.S2), git-config keys stagehand.tokenLimit/
  diffContext (S3 = P1.M1.T1.S3), bootstrap template + docs/CONFIGURATION.md (S4 = P1.M1.T1.S4).
- NOT S1: any consumer (StagedDiffOptions, the 6 call sites, the diff functions) — those are P1.M1.T2 /
  P1.M2+.
- DOCS: NONE in S1 (contract point 5: "This subtask adds only internal struct fields"). The bootstrap +
  docs/CONFIGURATION.md are S4.
- The new fields are DEAD (unconsumed) after S1 — `go build` compiles, `go test` green, no behavior change.
