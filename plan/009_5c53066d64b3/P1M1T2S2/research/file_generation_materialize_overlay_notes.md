# Research: fileGeneration + materialize + overlay for MultiTurn fields

> **Purpose:** Pin the exact `internal/config/file.go` edits for P1.M1.T2.S2 — threading the two
> multi-turn config knobs (`MultiTurnFallback`, `MultiTurnChunkTokens`) through the FILE-decode layer.
> Built on S1 (LANDED — `Config.MultiTurnFallback bool` + `Config.MultiTurnChunkTokens int` + Defaults
> true/32000 + TestDefaults pins, all in config.go). All line numbers verified live on 2026-07-05.

---

## 1. Baseline state (verified)

### 1.1 S1 LANDED — the resolved-Config fields exist (config.go)
```go
// config.go:83-85 (struct)
MaxDuplicateRetries  int  `toml:"max_duplicate_retries"`
MultiTurnFallback    bool `toml:"multi_turn_fallback"`     // default true (FR-T1c)
MultiTurnChunkTokens int  `toml:"multi_turn_chunk_tokens"` // default 32000 (FR-T3)
SubjectTargetChars   int  `toml:"subject_target_chars"`
// config.go:178-180 (Defaults)
MaxDuplicateRetries:  3,
MultiTurnFallback:    true,  // FR-T1c
MultiTurnChunkTokens: 32000, // FR-T3
```
`TestDefaults` already pins `true` / `32000` (S1). **This task consumes those landed fields.**

### 1.2 file.go has NO existing MultiTurn refs — genuine add
`grep MultiTurn internal/config/file.go` → no matches. The three edits below are net-new.

### 1.3 The two guard templates (the house-style patterns to mirror)
- **Int guard** (`!= 0`, every positive-default int): `TokenLimit`, `MaxDuplicateRetries`, `MaxCommits`,
  `SubjectTargetChars` all use `if g.X != 0 { c.X = g.X }` (materialize) / `if src.X != 0 { dst.X = src.X }`
  (overlay). For `MultiTurnChunkTokens` (default 32000), `!= 0` and `> 0` are equivalent for all valid
  user input; the contract says use `!= 0` to match house style.
- **Bool guard** (only-true-propagates): `AutoStageAll`, `Verbose`, `Push` use `if g.X { c.X = true }`
  (materialize) / `if src.X { dst.X = true }` (overlay). `MultiTurnFallback` (default true) mirrors this.
  ⚠️ **ACCEPTED limitation (per the contract + research §3b):** a user setting
  `multi_turn_fallback = false` in a file is silently ignored — the field stays at its `true` default.
  This is the SAME documented "v1 limitation" `AutoStageAll` (also default-true) carries. The contract
  explicitly accepts "follow whatever pattern auto_stage_all uses." Making it `*bool` (like `DiffContext`
  is `*int`) would fix the limitation but widens scope and diverges from the delta — NOT done here.
  S3 surfaces the limitation in `docs/configuration.md`.

### 1.4 NO `merge()` function — overlay IS the merge
`func overlay(dst, src *Config)` is at **file.go:294** (field-by-field merge across global→repo→git-config).
`func materialize(fc *fileConfig, timeout time.Duration) *Config` is at **file.go:193** (file→Config copy,
the delta's "loadTOML overlay"). The contract's "merge" = overlay; "loadTOML overlay" = materialize.

## 2. The three edits (exact current → target, verified line numbers)

### Edit 1 — `fileGeneration` struct (file.go:54-55)
Insert the two fields between `MaxDuplicateRetries` and `SubjectTargetChars` (matches §16.2's [generation]
table ordering + the resolved-Config struct order from S1).

**Current:**
```go
	MaxDuplicateRetries int      `toml:"max_duplicate_retries"`
	SubjectTargetChars  int      `toml:"subject_target_chars"`
```
**Target:**
```go
	MaxDuplicateRetries  int      `toml:"max_duplicate_retries"`   // re-gen attempts on duplicate subject
	MultiTurnFallback    bool     `toml:"multi_turn_fallback"`     // §9.24 FR-T1c multi-turn fallback (default true); only-true-propagates (mirrors AutoStageAll)
	MultiTurnChunkTokens int      `toml:"multi_turn_chunk_tokens"` // §9.24 FR-T3 per-request chunk size in tokens (default 32000); != 0 guard (mirrors TokenLimit)
	SubjectTargetChars   int      `toml:"subject_target_chars"`
```
(gofmt re-aligns the type + tag columns after the insert.)

### Edit 2 — `materialize` (file.go:229-234)
Insert the two clauses between the `MaxDuplicateRetries` clause and the `SubjectTargetChars` clause.

**Current:**
```go
	if g.MaxDuplicateRetries != 0 {
		c.MaxDuplicateRetries = g.MaxDuplicateRetries
	}
	if g.SubjectTargetChars != 0 {
		c.SubjectTargetChars = g.SubjectTargetChars
	}
```
**Target:**
```go
	if g.MaxDuplicateRetries != 0 {
		c.MaxDuplicateRetries = g.MaxDuplicateRetries
	}
	// §9.24 FR-T1c — multi_turn_fallback (bool; only-true-propagates, mirrors AutoStageAll —
	// cannot disable via file, same v1 limitation; S3 documents this in docs/configuration.md).
	if g.MultiTurnFallback {
		c.MultiTurnFallback = true
	}
	// §9.24 FR-T3 — multi_turn_chunk_tokens (int; != 0, mirrors TokenLimit/MaxDuplicateRetries).
	if g.MultiTurnChunkTokens != 0 {
		c.MultiTurnChunkTokens = g.MultiTurnChunkTokens
	}
	if g.SubjectTargetChars != 0 {
		c.SubjectTargetChars = g.SubjectTargetChars
	}
```

### Edit 3 — `overlay` (file.go:343-348)
Insert the two clauses between the `MaxDuplicateRetries` clause and the `SubjectTargetChars` clause.

**Current:**
```go
	if src.MaxDuplicateRetries != 0 {
		dst.MaxDuplicateRetries = src.MaxDuplicateRetries
	}
	if src.SubjectTargetChars != 0 {
		dst.SubjectTargetChars = src.SubjectTargetChars
	}
```
**Target:**
```go
	if src.MaxDuplicateRetries != 0 {
		dst.MaxDuplicateRetries = src.MaxDuplicateRetries
	}
	// §9.24 FR-T1c — multi_turn_fallback (bool; only-true-propagates, mirrors AutoStageAll/Push —
	// cannot disable via file, same v1 limitation).
	if src.MultiTurnFallback {
		dst.MultiTurnFallback = true
	}
	// §9.24 FR-T3 — multi_turn_chunk_tokens (int; != 0, mirrors TokenLimit/MaxCommits).
	if src.MultiTurnChunkTokens != 0 {
		dst.MultiTurnChunkTokens = src.MultiTurnChunkTokens
	}
	if src.SubjectTargetChars != 0 {
		dst.SubjectTargetChars = src.SubjectTargetChars
	}
```

## 3. Why this is behavior-free for existing tests (the regression guarantee)

The new `fileGeneration` fields default to Go zero-values (`false` / `0`) when a TOML file omits them.
- materialize: `if g.MultiTurnFallback { … }` → `false` → skip → `c.MultiTurnFallback` keeps the
  `Defaults()` value (`true`). `if g.MultiTurnChunkTokens != 0 { … }` → `0` → skip → keeps `32000`.
- overlay: same — `false`/`0` → skip → lower layer's value wins.

So every existing config-test fixture (TOML files without `multi_turn_*` keys) resolves to the SAME
`Defaults()` values as before → `TestDefaults` (S1) still pins `true`/`32000` → the whole `go test ./...`
suite stays green. The only NEW behavior is: a TOML file that SETS `multi_turn_chunk_tokens` (or
`multi_turn_fallback = true`) now propagates — and that has no existing test (S3 adds it).

## 4. Scope boundaries (do NOT do)

- Do NOT add a test in S2. The dedicated file/overlay unit tests for these fields are **P1.M1.T2.S3**
  ("Config unit tests + Mode A docs/configuration.md"). Adding them here overlaps S3. S2's validation is
  the existing suite staying green + greps confirming the wiring.
- Do NOT edit `docs/*` (the `[generation]` table + the multi_turn_fallback=false limitation note ride
  with S3 — contract point 5).
- Do NOT edit `internal/config/git.go` (the git-config resolver). The contract LOGIC (point 3) lists only
  fileGeneration/materialize/overlay. The git-config resolver reading `stagehand.multiTurnFallback` /
  `stagehand.multiTurnChunkTokens` keys is NOT in S2's LOGIC. The overlay edit DOES make the fields
  participate in the precedence chain, so a future git-config resolver would compose; but adding the
  git-config keys is a separate concern (plan 007 had it as a distinct subtask; plan 009's breakdown
  doesn't list one → multi-turn appears file-only by design).
- Do NOT add CLI flags or env vars (contract point 4: "Env/flag layers are OUT OF SCOPE (no new CLI
  flags/env per the delta)"). Do NOT touch `internal/config/load.go`.
- Do NOT make `MultiTurnFallback` a `*bool` on `fileGeneration` (research §3b recommends (a) accept the
  limitation, NOT (b) widen to *bool). Mirrors `AutoStageAll bool`, NOT `DiffContext *int`.
- Do NOT edit `internal/config/config.go` (S1 LANDED — read-only here), `internal/provider/*`,
  `internal/generate/*`, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

## 5. Decisions log

| # | Point | Decision | Why |
|---|---|---|---|
| D1 | Bool guard for MultiTurnFallback | `if g.MultiTurnFallback { c.MultiTurnFallback = true }` (only-true-propagates) | Mirrors AutoStageAll/Verbose/Push — the house bool pattern. Contract: "follow auto_stage_all." ACCEPTED limitation (false-in-file ignored); S3 documents it. |
| D2 | Int guard for MultiTurnChunkTokens | `!= 0` (NOT `> 0`) | Mirrors TokenLimit/MaxDuplicateRetries/MaxCommits. For default 32000 + positive user values, `!= 0`≡`> 0`; contract says use `!= 0` for house style. |
| D3 | Make MultiTurnFallback `*bool` on fileGeneration? | NO | Research §3b recommends (a) accept the limitation over (b) widen to *bool. The delta says "follow auto_stage_all." *bool diverges + widens scope. |
| D4 | Add a test? | NO (S3 owns it) | Plan: P1.M1.T2.S3 = "Config unit tests + Mode A docs." S2 is production plumbing only; existing suite green IS the regression proof. |
| D5 | Edit git.go (git-config keys)? | NO | Contract LOGIC lists only fileGeneration/materialize/overlay. git-config keys for multi-turn are not in plan 009's breakdown (file-only by design). overlay's new clauses still let a future git-config resolver compose. |
| D6 | Field placement | Between MaxDuplicateRetries and SubjectTargetChars (struct + materialize + overlay) | Matches S1's resolved-Config order + §16.2's [generation] table order; keeps the cluster contiguous. |
