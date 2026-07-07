# Research: MultiTurnFallback + MultiTurnChunkTokens Config Fields (P1.M1.T2.S1)

> **Purpose:** Pin the exact, source-verified edit for adding two generation scalars to the resolved
> `Config` struct, checked against the live codebase on 2026-07-05. **Both fields are ABSENT (genuine add);
> baseline `go test ./internal/config/` is GREEN (1.657s).** The prior parallel PRP (P1.M1.T1.S5) is
> test+docs in `internal/provider` + `docs/` — it does NOT touch `internal/config` → no conflict.

---

## 1. Environment & Baseline

| Item | Value / Evidence |
|---|---|
| Module | `github.com/dustin/stagecoach`, `go 1.22` |
| Edit target | `internal/config/config.go` (Config struct + Defaults() + TestDefaults assertion in config_test.go) |
| Fields today | ABSENT — `grep MultiTurnFallback|MultiTurnChunkTokens|multi_turn internal/config/config.go` → none. Genuine add. |
| Baseline | `go test ./internal/config/` → **ok (1.657s)**. |
| Prior PRP (S5) | Render unit tests (`internal/provider/render_test.go`) + Mode A docs (`docs/providers.md`, `docs/configuration.md`). NO `internal/config` edit → **no conflict**. |
| Sibling tasks | P1.M1.T2.S2 (fileGeneration + materialize + overlay — the file-decode layer), P1.M1.T2.S3 (config unit tests + [generation] table doc). THIS task = the resolved Config struct + Defaults() only. |

---

## 2. The Two Fields — semantics, type, default (per PRD §9.24 / §16.1)

| Field | Go type | TOML tag | Default | FR | Rationale |
|---|---|---|---|---|---|
| `MultiTurnFallback` | `bool` | `multi_turn_fallback` | `true` | §9.24 FR-T1c | Multi-turn fallback enabled by default (gated trigger FR-T1; the trigger gate reads `cfg.MultiTurnFallback` in P1.M1.T3.S3). |
| `MultiTurnChunkTokens` | `int` | `multi_turn_chunk_tokens` | `32000` | §9.24 FR-T3 | Per-request chunk size (tokens est) for multi-turn fallback (the protocol reads `cfg.MultiTurnChunkTokens` in P1.M1.T3.S2). |

**Type choice — plain `bool` / plain `int` (NOT pointers):** the resolved `Config` is the final,
post-resolution value (its docstring config.go:47-61: "flat, resolved, plain-typed"). The `*bool`/`*int`
pointer pattern (like `DiffContext *int`, `StripCodeFence *bool`) exists for the FILE-DECODE layer
(`fileGeneration` in file.go) to distinguish "key absent" from "explicit zero/false" during overlay — that
is S2's concern, NOT the resolved Config. Precedent: `AutoStageAll bool` (default true) is the exact sibling
for `MultiTurnFallback`; `MaxDuplicateRetries int` (default 3) is the exact sibling for
`MultiTurnChunkTokens`. Both new fields sit immediately after `MaxDuplicateRetries`.

---

## 3. The Exact Insertion Points (verified against live source)

### 3.1 Config struct (config.go:83-84)
```go
	MaxDuplicateRetries int  `toml:"max_duplicate_retries"` // re-gen attempts on duplicate subject
	SubjectTargetChars  int  `toml:"subject_target_chars"`  // target subject length for truncation
```
**Target** — insert the two fields BETWEEN MaxDuplicateRetries and SubjectTargetChars:
```go
	MaxDuplicateRetries  int  `toml:"max_duplicate_retries"`   // re-gen attempts on duplicate subject
	MultiTurnFallback    bool `toml:"multi_turn_fallback"`     // §9.24 FR-T1c multi-turn fallback (lossless large-diff priming); default true; consumed by P1.M1.T3.S3 trigger gate
	MultiTurnChunkTokens int  `toml:"multi_turn_chunk_tokens"` // §9.24 FR-T3 per-request chunk size (tokens est) for multi-turn; default 32000; consumed by P1.M1.T3.S2 protocol
	SubjectTargetChars   int  `toml:"subject_target_chars"`    // target subject length for truncation
```
(gofmt re-aligns the type + tag columns automatically — run `gofmt -w`.)

### 3.2 Defaults() (config.go:176-177)
```go
		MaxDuplicateRetries: 3,
		SubjectTargetChars:  50,
```
**Target** — insert the two defaults BETWEEN them:
```go
		MaxDuplicateRetries:   3,
		MultiTurnFallback:     true,  // §9.24 FR-T1c default (multi-turn fallback enabled)
		MultiTurnChunkTokens:  32000, // §9.24 FR-T3 default (per-request chunk size, tokens est)
		SubjectTargetChars:    50,
```

### 3.3 TestDefaults (config_test.go:46-51) — pin the new defaults
The existing `TestDefaults` enumerates scalar defaults sequentially (MaxDiffBytes→MaxMdLines→TokenLimit→
DiffContext→MaxDuplicateRetries→SubjectTargetChars→…). Insert the two new assertions between the
MaxDuplicateRetries check (line 46-48) and the SubjectTargetChars check (line 49-51):
```go
	if c.MaxDuplicateRetries != 3 {
		t.Errorf("MaxDuplicateRetries = %d, want 3", c.MaxDuplicateRetries)
	}
	if !c.MultiTurnFallback {
		t.Errorf("MultiTurnFallback = false, want true (§9.24 FR-T1c)")
	}
	if c.MultiTurnChunkTokens != 32000 {
		t.Errorf("MultiTurnChunkTokens = %d, want 32000 (§9.24 FR-T3)", c.MultiTurnChunkTokens)
	}
	if c.SubjectTargetChars != 50 {
		t.Errorf("SubjectTargetChars = %d, want 50", c.SubjectTargetChars)
	}
```

---

## 4. Why a TestDefaults Assertion (and not just S3's tests)

The contract says "DOCS: none — the [generation] table doc rides with P1.M1.T2.S3" and S3 owns the
dedicated config unit tests. BUT the existing `TestDefaults` (config_test.go:11) ALREADY pins every scalar
default in the resolved Config (MaxDiffBytes, MaxMdLines, TokenLimit, DiffContext, MaxDuplicateRetries,
SubjectTargetChars, …). Leaving the two new defaults UNVERIFIED in `TestDefaults` would be inconsistent with
the file's own pattern and would let a future edit silently change `true`→`false` or `32000`→`0` with no
test catching it. Adding the two assertions is a 6-line, zero-risk accompaniment that matches the existing
enumeration. S3 then owns the file/overlay/materialize tests (the decode-layer coverage) — no overlap.

---

## 5. Decisions Log

| # | Question | Decision | Rationale |
|---|---|---|---|
| D1 | Field types? | `MultiTurnFallback bool`, `MultiTurnChunkTokens int` (plain, NOT pointers). | The resolved Config is flat/plain-typed (config.go:47-61). Pointer pattern is file-decode-layer (S2). Precedent: AutoStageAll (bool, true), MaxDuplicateRetries (int, 3). |
| D2 | Placement? | Immediately after `MaxDuplicateRetries` (struct :83, Defaults :176). | The contract: "immediately after MaxDuplicateRetries". Keeps the generation-scalar cluster contiguous; the §16.2 config example lists them right after max_duplicate_retries too. |
| D3 | TOML tags? | `multi_turn_fallback`, `multi_turn_chunk_tokens` (snake_case). | Matches §16.2 verbatim + the existing snake_case leaf convention. Tags are documentation parity (the resolved Config is never directly decoded — fileConfig is). |
| D4 | TestDefaults assertions? | YES — add 2 (pin true / 32000). | The existing TestDefaults pins every scalar default; leaving the new ones unverified is inconsistent + fragile. 6 lines, zero-risk. S3 owns the decode-layer tests (no overlap). |
| D5 | Docs? | NONE. | Contract point 5: the [generation] table doc rides with P1.M1.T2.S3. |
| D6 | Scope vs siblings? | ONLY config.go (struct + Defaults) + config_test.go (TestDefaults). NOT file.go (S2 = fileGeneration/materialize/overlay), NOT docs (S3), NOT internal/provider (S5). | This task is the resolved-Config layer only. |
