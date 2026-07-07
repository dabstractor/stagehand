# Research: StagedDiffOptions Fields — TokenLimit, DiffContext, PromptReserveTokens (P1.M1.T2.S1)

> **Purpose:** Pin the exact edit for adding three fields to `StagedDiffOptions` (`internal/git/git.go:36-44`),
> checked against the live codebase on 2026-07-04 (struct has 4 fields; the three are ABSENT — genuine add;
> `go test ./internal/git/` GREEN; git.go compiles) and the architecture `diff_capture_touchmap.md` §2.
> This is a pure struct-field addition that threads the v2.1 diff-overlay seam; the fields are UNREAD
> until M2 (DiffContext) and M4 (TokenLimit/PromptReserveTokens). No behavior change.

---

## 1. Environment & Baseline

| Item | Value / Evidence |
|---|---|
| Module | `github.com/dustin/stagecoach`, `go 1.22` |
| Edit target | `internal/git/git.go` — the `StagedDiffOptions` struct (lines 36-44) |
| Struct today | 4 fields: `MaxDiffBytes int`, `MaxMDLines int`, `Excludes []string`, `BinaryExtensions []string`. The three new fields are **ABSENT** (grep-confirmed). |
| Baseline | `go test ./internal/git/` → **ok (cached)**; `go build ./internal/git/` → OK. |
| Config layer (S1/S2 Complete) | `config.TokenLimit int` (plain; 0=unset) + `config.DiffContext *int` (pointer; nil=unset, non-nil incl. *0=explicit; default `intPtr(1)`). Verified at config.go:81-82, 174-175; file.go:52-53, 218-227, 331-341. |
| Prior PRP (S4, parallel) | Bootstrap config template + docs/CONFIGURATION.md. Does NOT touch `internal/git/git.go` → **no conflict**. |

---

## 2. The Three Fields — semantics, type choice, and source mapping

### 2.1 `TokenLimit int` (FR3d — holistic token budget)

- **Semantics:** a holistic token cap over the WHOLE payload (system prompt + style examples + diff). When
  `> 0`, it SUPERSEDES the legacy per-section caps (`MaxDiffBytes`/`MaxMDLines`) for that run; the two
  modes are mutually exclusive. When `0` (unset), the legacy caps apply unchanged.
- **Type:** plain `int` — matching the config layer (`config.TokenLimit int`). `0` IS its unset sentinel
  (FR3d: "no meaningful explicit 0"). No pointer needed.
- **Source:** `cfg.TokenLimit` (a plain int at every config layer). The 6 call sites (T2.S2) map it
  directly: `TokenLimit: cfg.TokenLimit`.
- **Consumed by:** M4 (P1.M4.T3 — the token-limit gate that switches off legacy caps when `>0`).

### 2.2 `DiffContext int` (FR3f — reduced unified context)

- **Semantics:** the `-U<n>` unified-context line count (0–3). Reduces git's `-U3` default to cut
  unchanged-context noise. `0` = changed lines only (maximal savings); `1` = default (one anchor line);
  `3` = git's default.
- **Type:** plain `int` — **NOT `*int`**. This is the subtle one. The config layer uses `*int` (to
  distinguish "user omitted the key" from "user set 0"), but `StagedDiffOptions` takes the **RESOLVED**
  value. Resolution happens at the call site (T2.S2): `*cfg.DiffContext` with a default-1 fallback when
  `cfg.DiffContext == nil`. So at this struct's level, `0` means `-U0` (valid), NEVER "unset".
- **⚠️ Guard-note (contract requirement):** `DiffContext == 0` is VALID (-U0). Callers MUST pass the
  resolved value (default 1 when the user omits it) explicitly. This must be prominent in the field's
  doc comment so (a) the call-site mapper does NOT write `if cfg.DiffContext != nil`-style skippage here,
  and (b) no future reader treats 0 as "unset" and silently skips emitting `-U0`.
- **Source:** `cfg.DiffContext` (a `*int`), dereferenced at the call site. The default-1 fallback lives
  at the call site, NOT in this struct (the struct has no "unset" state for DiffContext).
- **Consumed by:** M2 (P1.M2.T2 — the flag helper injects `-U<opts.DiffContext>` into the diff argv).

### 2.3 `PromptReserveTokens int` (FR3i — stable prompt portion cost)

- **Semantics:** the token cost of the STABLE prompt portion — system-prompt header + style examples
  (FR11) + user instruction + worst-case rejection block + margin — measured UPSTREAM (prompt/generate
  layers) and passed in so the git layer can compute `body_budget = token_limit − skeleton − promptReserve`
  for the dynamic water-fill truncation.
- **Type:** plain `int`. `0` = unset (no reserve subtracted). Only meaningful when `TokenLimit > 0`.
- **Source:** measured upstream (M4.T1.S2 measures it at the 6 call sites from `BuildUserPayload`'s
  constants + `RecentMessages` output). The git layer does NOT compute it — it receives it.
- **Consumed by:** M4 (P1.M4.T2 — the water-fill level solver subtracts it from the budget).
- **Seam rationale (system_context.md §5):** the git layer owns the diff body + numstat sizing; the
  prompt portion is measured upstream and passed in. This field IS that seam — it keeps the git layer
  free of prompt-construction concerns (no import of `internal/prompt`).

---

## 3. Placement & Style

The existing struct uses concise inline comments (`MaxDiffBytes int // byte cap …`). The three new
fields are a cohesive v2.1 feature set (the diff-payload overlay) with a "why are these here if unread"
rationale that's valuable for future readers. Placement: AFTER the `BinaryExtensions` comment block,
grouped under a brief header comment, each with an inline FR-cited comment + the DiffContext guard-note.

Adding fields to a Go struct cannot break any existing caller — the ~57 test call sites
(`StagedDiffOptions{}` zero-value or partial literals) and the 6 production call sites all continue to
compile (the new fields default to 0). gofmt re-aligns the column if needed (run `gofmt -w`).

---

## 4. Why No Test for This Task

The fields are UNREAD by the three diff functions (StagedDiff/TreeDiff/WorkingTreeDiff) until M2/M4.
An unread struct field has no observable behavior — a "struct has these fields" test would be
tautological, and there is nothing to assert about the zero-value behavior (it is the status quo). The
real coverage lands in M2 (DiffContext → argv assertions) and M4 (TokenLimit/PromptReserveTokens →
gate + water-fill). The contract explicitly says "No behavior change yet" — existing tests staying
green IS the validation. (Adding a test would be pointless churn; do not.)

---

## 5. Decisions Log

| # | Question | Decision | Rationale |
|---|---|---|---|
| D1 | Field types? | All three plain `int`. | TokenLimit matches config (plain int, 0=unset). DiffContext is the RESOLVED value (config's *int dereferenced at the call site; 0=-U0 is valid here). PromptReserveTokens is a measured count. |
| D2 | Why plain int for DiffContext when config is *int? | The git layer takes the resolved value; the *int→int resolution (nil→default 1, *0→0) is the CALL SITE's job (T2.S2). | Keeps the struct's semantics clean: 0 means -U0, never "unset". Matches the contract's guard-note. |
| D3 | Placement? | After BinaryExtensions, grouped under a v2.1 header comment. | Cohesive feature set; the header explains why unread fields exist (seam threading). |
| D4 | Add a test? | NO. | Unread fields have no behavior to test; the contract says "no behavior change yet". M2/M4 add the real coverage. Existing tests staying green is the gate. |
| D5 | Doc-comment density? | Thorough (FR citation + source + consumer + the DiffContext guard-note). | The contract requires FR citations + the guard-note; the fields have subtle semantics worth documenting. |
| D6 | Scope vs siblings? | ONLY the StagedDiffOptions struct in internal/git/git.go. | T2.S2 owns the 6 call-site mappings; M2/M4 own consumption; S4 owns bootstrap config. This task is the struct fields only. |
