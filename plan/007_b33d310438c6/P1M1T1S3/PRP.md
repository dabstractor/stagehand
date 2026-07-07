---
name: "P1.M1.T1.S3 — git-config resolver keys (stagecoach.tokenLimit / stagecoach.diffContext)"
description: |
  Wire `stagecoach.tokenLimit` and `stagecoach.diffContext` into the git-config resolver `loadGitConfig`
  (`internal/config/git.go`), mirroring the existing `stagecoach.maxDiffBytes` int-key block exactly. Adds
  two `gitConfigGet` + `parseInt` blocks: (a) `stagecoach.tokenLimit` → `c.TokenLimit` (plain int); (b)
  `stagecoach.diffContext` → `c.DiffContext` (`*int`). ⚠️ S2 has LANDED: `Config.DiffContext` is `*int`
  (config.go:82) and the `intPtr` helper exists (config.go:11) — verified in the working tree. Because
  DiffContext is `*int`, its block parses into a local `var n int` (parseInt's `dst` is `*int`, and
  `&c.DiffContext` would be `**int`) and then wraps `c.DiffContext = intPtr(n)`. The write is
  UNCONDITIONAL inside the `found` gate — NO `!= 0` value guard — so an explicit `git config
  stagecoach.diffContext 0` survives as a non-nil `*int` pointing to 0 (FR3f: 0 = changed-lines-only is a
  first-class value), per the contract's load-bearing NOTE. S3 touches ONLY `internal/config/git.go` +
  its test. NO docs (the user-facing key reference is S4). NO code changes outside the git-config resolver.
---

## Goal

**Feature Goal**: Make `git config stagecoach.tokenLimit <N>` and `git config stagecoach.diffContext <N>`
resolve into `config.Config` via the per-repo git-config layer (PRD §16.3, FR36; precedence layer 3 per
FR34), following the established `gitConfigGet` + `parseInt` idiom — with correct 0-vs-unset semantics
for `diffContext` (an explicit `0` is preserved, not dropped).

**Deliverable**: Two new int-key blocks in `loadGitConfig` (`internal/config/git.go`), inserted among the
existing int blocks (maxDiffBytes/maxMdLines/maxDuplicateRetries/subjectTargetChars). Plus a table-driven
test in `internal/config/git_test.go` covering unset / 0 / 1 / 3 (and tokenLimit unset / 120000) using
the existing `initRepo` + `setGitConfig` + `loadGitConfig` test idiom. No other file changes.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green
with the new test passing — critically, `git config stagecoach.diffContext 0` yields a non-nil
`*cfg.DiffContext == 0` (the explicit-0 row is load-bearing), and an unset key yields `cfg.DiffContext == nil`
(so overlay inherits the default). `git diff --stat` shows ONLY `internal/config/git.go` +
`internal/config/git_test.go`.

## User Persona

**Target User**: The Stagecoach user who configures diff-payload knobs via per-repo `git config` (FR34
layer 3) instead of a TOML file — and the contributor implementing the diff-function consumers (P1.M1.T2
StagedDiffOptions, P1.M2+ the `-U<diff_context>` diff functions) who will read `cfg.TokenLimit` /
`cfg.DiffContext`.

**Use Case**: A user runs `git config stagecoach.diffContext 0` to maximize diff savings (changed-lines-
only, FR3f) and expects it to take effect (overriding the file's `diff_context = 1`, since git config is
HIGHER precedence per FR34). Or `git config stagecoach.tokenLimit 120000` to cap the holistic payload
(FR3d).

**Pain Points Addressed**: Without S3, the two new knobs (landed as Config fields in S1, wired through the
file layer in S2) are **invisible to the git-config layer** — `git config stagecoach.diffContext 0` would be
silently ignored. S3 closes the layer-3 gap so both keys resolve through the full FR34 precedence chain.

## Why

- **Completes the FR34 precedence chain for the two new knobs.** S1 added the Config/fileGeneration fields;
  S2 wired file→Config (materialize/overlay) with the `*int` correction that makes an explicit
  `diff_context = 0` survive. S3 is the git-config layer (precedence layer 3, ABOVE the file). Without it,
  git-config users cannot set either knob, and an explicit `git config stagecoach.diffContext 0` would be
  impossible — contradicting FR3f (0 is a first-class value) and the contract's own end-to-end requirement.
- **Lowest-risk mechanical addition.** The pattern is established (the four existing int-key blocks); S3
  adds two more following it verbatim. The only subtlety is the `*int` wrap for DiffContext (S2's contract),
  which is precisely specified below.
- **Preserves an explicit 0 (the contract's load-bearing NOTE).** `git config` has no native nil-int —
  `git config stagecoach.diffContext 0` returns the string `"0"`. Because git config is HIGHER precedence
  than the file (FR34), an explicit 0 here MUST override the file's 1. S3 gates the write on the `found`
  boolean (key present) — NOT on the parsed value — so the explicit 0 is written as `intPtr(0)` and
  survives overlay. A naive `!= 0` value guard would silently drop it (the exact bug the contract warns
  against).
- **TokenLimit is the simple case.** FR3d: `0` IS TokenLimit's unset sentinel (no meaningful "explicit 0"),
  so plain `int` + the standard `found`-gated `parseInt(... &c.TokenLimit)` is correct — an exact mirror
  of MaxDiffBytes.

## What

Two new int-key blocks in `loadGitConfig` (`internal/config/git.go`):

1. `stagecoach.tokenLimit` → `c.TokenLimit` (plain int) — an **exact mirror** of the `stagecoach.maxDiffBytes`
   block (`gitConfigGet` → `found` gate → `parseInt(&c.TokenLimit)`).
2. `stagecoach.diffContext` → `c.DiffContext` (`*int`) — mirrors the maxDiffBytes block's structure, but
   parses into a local `var n int` and wraps `c.DiffContext = intPtr(n)` (because `Config.DiffContext` is
   `*int` per S2, so `&c.DiffContext` would be `**int` — incompatible with `parseInt`'s `dst *int`). The
   wrap is UNCONDITIONAL inside the `found` gate (no `!= 0` value guard).

Plus a table-driven test in `internal/config/git_test.go` covering unset / 0 / 1 / 3.

### Success Criteria

- [ ] `loadGitConfig` reads `stagecoach.tokenLimit` (camelCase key) into `c.TokenLimit` (plain int) when the
      key is present; absent ⇒ `c.TokenLimit == 0`.
- [ ] `loadGitConfig` reads `stagecoach.diffContext` (camelCase key) into `c.DiffContext` (`*int`) when the
      key is present; absent ⇒ `c.DiffContext == nil`.
- [ ] An explicit `git config stagecoach.diffContext 0` ⇒ `cfg.DiffContext != nil && *cfg.DiffContext == 0`
      (the write is gated on `found`, NOT on `!= 0`).
- [ ] The diffContext block uses the `var n int` + `parseInt(&n)` + `c.DiffContext = intPtr(n)` idiom (NOT
      `&c.DiffContext`, which is `**int`).
- [ ] Both blocks reuse the existing `gitConfigGet` + `parseInt` helpers; the only new helper reference is
      `intPtr` (S2's helper, already in config.go).
- [ ] A non-integer value (`stagecoach.diffContext=abc`, `stagecoach.tokenLimit=NaN`) returns a wrapped error
      (mirrors the existing `stagecoach.timeout` bad-value behavior).
- [ ] The new table-driven test passes (unset / 0 / 1 / 3 for diffContext; unset / 120000 for tokenLimit).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test -race ./...` green.
- [ ] ONLY `internal/config/git.go` + `internal/config/git_test.go` change (git diff --stat confirms).
- [ ] NO docs, NO bootstrap template, NO Config/file/overlay edits, NO consumer/diff-function edits.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the EXACT current `maxDiffBytes` block (verbatim, the mirror
template), the EXACT target blocks for both keys (ready to paste, including the `*int` wrap for
DiffContext), the precise insertion point (after maxMdLines, before maxDuplicateRetries), the verified
post-S2 state (`Config.DiffContext` is already `*int`, `intPtr` exists — confirmed via grep), the 0-vs-
unset trace table, and the exact test idiom (`initRepo`/`setGitConfig`/`loadGitConfig`) with the
load-bearing explicit-0 row. The S2 `*int` contract is treated as landed (it IS landed in the working tree).

### Documentation & References

```yaml
# MUST READ — the *int contract S3 implements (S2 has LANDED — verified)
- docfile: plan/007_b33d310438c6/P1M1T1S2/PRP.md
  why: "S2 established Config.DiffContext = *int (nil = unset) so overlay's `!= nil` guard preserves an explicit diff_context=0. S2's DOWNSTREAM HOOK states: 'S3 must set c.DiffContext = intPtr(v) when stagecoach.diffContext is found (nil when absent).' S3 is the fulfillment of that hook."
  critical: "Config.DiffContext is *int (config.go:82, VERIFIED in working tree); intPtr helper exists (config.go:11, VERIFIED). S3 reuses intPtr — do NOT reinvent it. TokenLimit stays plain int (S1). The DiffContext block CANNOT use `&c.DiffContext` (that is **int); parse into a local `var n int` then `c.DiffContext = intPtr(n)`."

- docfile: plan/007_b33d310438c6/architecture/diff_capture_touchmap.md
  why: "§3(c) names loadGitConfig (git.go) as the git-config resolver and the maxDiffBytes 4-line gitConfigGet+parseInt idiom as the pattern to copy verbatim. Confirms camelCase keys (stagecoach.maxDiffBytes / maxMdLines / stripCodeFence)."
  critical: "Keys are CAMELCASE (git rejects underscores). The two new keys are exactly `stagecoach.tokenLimit` and `stagecoach.diffContext`."

- docfile: plan/007_b33d310438c6/P1M1T1S1/PRP.md
  why: "S1 added the Config fields (TokenLimit plain int, DiffContext) and Defaults seeds. S1's DOWNSTREAM HOOK for S3: 'read stagecoach.tokenLimit / stagecoach.diffContext git-config keys into Config.' S3 is that read."
  critical: "S1's plain-int Config.DiffContext was UPGRADED to *int by S2. S3 implements against the post-S2 *int state (verified), not S1's original plain int."

# The file under edit
- file: internal/config/git.go
  why: "EDIT loadGitConfig: insert two int-key blocks among the existing int blocks (maxDiffBytes at 181, maxMdLines 188, maxDuplicateRetries 195, subjectTargetChars 202, return at 210). The maxDiffBytes block (181-186) is the verbatim mirror template for TokenLimit; the DiffContext block mirrors it structurally but wraps with intPtr."
  pattern: "`if v, found, err := gitConfigGet(repoDir, \"stagecoach.<key>\"); err != nil { return nil, err } else if found { if err := parseInt(repoDir, \"stagecoach.<key>\", v, &c.<Field>); err != nil { return nil, err } }` — the 4-line idiom (git.go:181-186). DiffContext adds `var n int` + `c.DiffContext = intPtr(n)` because c.DiffContext is *int."
  gotcha: "(1) camelCase keys (tokenLimit/diffContext), NOT snake_case — git rejects underscores. (2) DiffContext is *int: use the local-n + intPtr wrap, NOT &c.DiffContext. (3) The write inside `found` is UNCONDITIONAL — never `if n != 0`. (4) loadGitConfig returns a PARTIAL config (absent keys stay zero/nil); do NOT call Defaults() here."

- file: internal/config/git_test.go
  why: "EDIT: add TestLoadGitConfig_TokenLimit_DiffContext (table-driven) + a parse-error test. Reuses the existing `initRepo(t, repo)` (line 24) + `setGitConfig(t, repo, key, value)` (line 46) + `loadGitConfig(repo)` idiom + `t.Setenv(\"HOME\", t.TempDir())` global-config isolation. Mirror TestLoadGitConfig_ReadsValues (line ~75) for the happy path and TestLoadGitConfig_BadTimeout (line ~203) for the parse-error path."
  pattern: "setGitConfig(t, repo, \"stagecoach.diffContext\", \"0\"); cfg, err := loadGitConfig(repo); assert cfg.DiffContext != nil && *cfg.DiffContext == 0."
  gotcha: "The UNSET row must assert `cfg.DiffContext == nil` (NOT *1) — loadGitConfig does NOT apply Defaults; nil is correct and is what makes overlay skip to inherit the *1 default. Isolate HOME so no global stagecoach.diffContext leaks in."

# Read-only refs (do NOT edit in S3)
- file: internal/config/config.go
  why: "READ-ONLY. S1/S2 own it. Lines 11 (intPtr), 81 (TokenLimit int), 82 (DiffContext *int), 174-175 (Defaults seeds). S3 READS intPtr + the *int field type; does NOT modify config.go."
- file: internal/config/load.go
  why: "READ-ONLY. The overlay flow (Defaults → overlay file → overlay gitconfig) that consumes loadGitConfig's output. Unchanged. S3's nil-when-absent output is what makes overlay correctly skip/inherit."

# PRD authority
- prd: PRD.md §9.1 FR3d (token_limit, default 0 = unset ⇒ legacy caps) + FR3f (diff_context, integer 0–3, default 1; 0 = changed-lines-only); §9.8 FR34 (precedence: git-config layer 3, ABOVE the file) + FR36 (keys under stagecoach.* section, read via `git config --get`); §16.3 (git-config keys alternative to a file).
  why: "FR3f is WHY an explicit 0 must be preserved (0 is a meaningful value). FR34 is WHY an explicit git-config 0 overrides the file's 1 (git config is higher precedence). FR36 is the keys-under-stagecoach.* + camelCase authority."
```

### Current Codebase Tree (relevant slice)

```bash
stagecoach/
└── internal/config/
    ├── git.go          # EDIT: loadGitConfig +2 int-key blocks (tokenLimit, diffContext)
    ├── git_test.go     # EDIT: + table-driven test + parse-error test
    ├── config.go       # READ-ONLY (S1/S2): intPtr (L11), TokenLimit int (L81), DiffContext *int (L82)
    ├── file.go         # READ-ONLY (S2): materialize/overlay wired with *int DiffContext
    └── load.go         # READ-ONLY: the overlay flow that consumes loadGitConfig
```

### Desired Codebase Tree After S3

```bash
stagecoach/
└── (only existing files modified — no new files)
    internal/config/git.go          # loadGitConfig +stagecoach.tokenLimit +stagecoach.diffContext blocks
    internal/config/git_test.go     # +TestLoadGitConfig_TokenLimit_DiffContext +parse-error test
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/config/git.go` | MODIFY | Add the two int-key blocks to `loadGitConfig` (tokenLimit exact mirror; diffContext `*int` wrap). |
| `internal/config/git_test.go` | MODIFY | Add the table-driven test (unset/0/1/3) + parse-error test. |

**Explicitly NOT touched**: `config.go` (S1/S2), `file.go` (S2), `load.go` (overlay flow — unchanged),
`bootstrap.go` + `docs/CONFIGURATION.md` (S4 — the user-facing key reference), `StagedDiffOptions` + 6 call
sites (P1.M1.T2), the diff functions / `-U<diff_context>` (P1.M2+), any other package, `PRD.md`,
`tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (camelCase keys): git config rejects underscores ("invalid key: stagecoach.diff_context").
// The two new keys are EXACTLY `stagecoach.tokenLimit` and `stagecoach.diffContext` (matching the existing
// stagecoach.maxDiffBytes / maxMdLines / autoStageAll / stripCodeFence camelCase convention). NOT snake_case.

// CRITICAL (*int wrap for DiffContext): Config.DiffContext is *int (S2, verified config.go:82). parseInt's
// signature is `parseInt(repo, key, value string, dst *int) error` — it writes through a *int. &c.DiffContext
// is **int, NOT assignable. ⇒ parse into `var n int` (`parseInt(..., &n)`) then `c.DiffContext = intPtr(n)`.
// intPtr is S2's helper (config.go:11). TokenLimit is plain int → `&c.TokenLimit` works directly (exact mirror).

// CRITICAL (no != 0 value guard): the write inside the `found` branch MUST be unconditional.
// `c.DiffContext = intPtr(n)` — NOT `if n != 0 { c.DiffContext = intPtr(n) }`. The contract's load-bearing
// NOTE: an explicit `git config stagecoach.diffContext 0` must survive. The `found` boolean (key present) is
// the ONLY gate. A `!= 0` value guard would drop the explicit 0 (the bug being avoided).

// GOTCHA (partial config): loadGitConfig returns a PARTIAL *Config — absent keys stay at zero/nil. It does
// NOT call Defaults(). So an unset stagecoach.diffContext ⇒ c.DiffContext == nil (NOT *1). The -U1 default
// comes from Defaults() in load.go, applied via overlay. This nil-when-absent is what makes overlay skip and
// inherit the default — preserve it.

// GOTCHA (parseInt error format): parseInt returns `fmt.Errorf("git config %s: invalid integer %q: %w", ...)`.
// Reuse parseInt (do NOT inline strconv.Atoi) so both new keys get the identical, grep-able error shape and
// the existing TestLoadGitConfig_BadTimeout-style error test pattern applies.

// GOTCHA (test isolation): mirror the existing tests' `t.Setenv("HOME", t.TempDir())` so no global
// ~/.gitconfig stagecoach.* key leaks into the unset row. (FINDING E in the existing git_test.go.)

// GOTCHA (S2 dependency is satisfied): S2 has LANDED (verified: Config.DiffContext *int at config.go:82,
// intPtr at config.go:11). S3 does NOT depend on any unbuilt sibling — it implements against the real tree.
```

## Implementation Blueprint

### Data models and structure

No new types. Two int-key blocks reusing the existing `gitConfigGet` + `parseInt` helpers and S2's `intPtr`.
The relevant existing precedent (unchanged) is the model to mirror:

```go
// git.go loadGitConfig — the EXISTING int-key block (the verbatim mirror template), git.go:181-186
	if v, found, err := gitConfigGet(repoDir, "stagecoach.maxDiffBytes"); err != nil { // camelCase!
		return nil, err
	} else if found {
		if err := parseInt(repoDir, "stagecoach.maxDiffBytes", v, &c.MaxDiffBytes); err != nil {
			return nil, err
		}
	}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: git.go — add the stagecoach.tokenLimit block (EXACT mirror of maxDiffBytes; plain int)
  - LOCATE: internal/config/git.go, loadGitConfig, the int blocks (maxDiffBytes 181-186, maxMdLines
    188-193, maxDuplicateRetries 195-200, subjectTargetChars 202-207, return at 210).
  - INSERT after the maxMdLines block (after line 193, before maxDuplicateRetries at 195) — groups the
    four diff-capture knobs (maxDiffBytes/maxMdLines/tokenLimit/diffContext). (Alternative: append before
    `return c, nil` at 210 — also acceptable.) Insert:
        // §9.1 FR3d — token_limit via git config (camelCase key). 0 = unset ⇒ legacy caps (no meaningful explicit 0).
        if v, found, err := gitConfigGet(repoDir, "stagecoach.tokenLimit"); err != nil { // camelCase!
            return nil, err
        } else if found {
            if err := parseInt(repoDir, "stagecoach.tokenLimit", v, &c.TokenLimit); err != nil {
                return nil, err
            }
        }
  - This is an EXACT structural mirror of the maxDiffBytes block (plain int → &c.TokenLimit is *int ✓).
  - KEY: `stagecoach.tokenLimit` (camelCase). FIELD: `c.TokenLimit` (plain int).

Task 2: git.go — add the stagecoach.diffContext block (*int wrap; UNCONDITIONAL write inside `found`)
  - INSERT immediately after the tokenLimit block (Task 1). Insert:
        // §9.1 FR3f — diff_context via git config (camelCase key, integer 0–3). Config.DiffContext is *int
        // (S2): nil when the key is absent (found=false → overlay inherits the default *1); non-nil — incl.
        // *0 — when found. The write is UNCONDITIONAL inside `found`: an explicit "git config
        // stagecoach.diffContext 0" must survive as intPtr(0) (0 = changed-lines-only is a first-class value).
        if v, found, err := gitConfigGet(repoDir, "stagecoach.diffContext"); err != nil { // camelCase!
            return nil, err
        } else if found {
            var n int
            if err := parseInt(repoDir, "stagecoach.diffContext", v, &n); err != nil {
                return nil, err
            }
            c.DiffContext = intPtr(n) // *int: parse into local n (NOT &c.DiffContext which is **int), then wrap.
        }
  - WHY the local `var n int`: parseInt's dst is *int; &c.DiffContext is **int (DiffContext is *int per S2).
    Reuse parseInt (consistent error format) then wrap with intPtr (S2's helper, config.go:11).
  - KEY: `stagecoach.diffContext` (camelCase). FIELD: `c.DiffContext` (*int).
  - DO NOT: add `if n != 0` (that drops an explicit 0 — the contract's forbidden bug). DO NOT pass
    &c.DiffContext to parseInt (type mismatch). DO NOT inline strconv.Atoi (loses the parseInt error shape).

Task 3: git_test.go — table-driven test (the contract's required verification: unset / 0 / 1 / 3)
  - ADD TestLoadGitConfig_TokenLimit_DiffContext. Reuse `initRepo` (git_test.go:24) + `setGitConfig`
    (git_test.go:46) + `loadGitConfig` + `t.Setenv("HOME", t.TempDir())`. Table rows:
      diffContext rows (the load-bearing ones):
        - unset (no key)          → cfg.DiffContext == nil                        (nil ⇒ overlay inherits *1)
        - "0"                     → cfg.DiffContext != nil && *cfg.DiffContext == 0  (EXPLICIT-0 — must survive)
        - "1"                     → cfg.DiffContext != nil && *cfg.DiffContext == 1
        - "3"                     → cfg.DiffContext != nil && *cfg.DiffContext == 3
      tokenLimit rows:
        - unset (no key)          → cfg.TokenLimit == 0
        - "120000"                → cfg.TokenLimit == 120000
  - Each row: fresh `t.TempDir()` + `initRepo` + (optionally) one setGitConfig + `loadGitConfig` + assert.
  - CRITICAL assertion: the UNSET row asserts `cfg.DiffContext == nil` (NOT *1) — loadGitConfig returns a
    PARTIAL config; nil is correct (overlay applies the default). The EXPLICIT-0 row is the load-bearing
    proof the `found`-gated unconditional write works (it would FAIL if a `!= 0` value guard were added).

Task 4: git_test.go — parse-error test (mirror TestLoadGitConfig_BadTimeout, git_test.go:~203)
  - ADD TestLoadGitConfig_BadTokenLimit_DiffContext (or extend the table). Assert non-integer values return
    a wrapped error:
        setGitConfig(t, repo, "stagecoach.diffContext", "abc")  → loadGitConfig err != nil
        setGitConfig(t, repo, "stagecoach.tokenLimit", "NaN")   → loadGitConfig err != nil
  - Mirror the existing bad-timeout test's assertion shape (`if err == nil { t.Fatal(...) }`).

Task 5: VALIDATE
  - RUN: gofmt -w internal/config/git.go internal/config/git_test.go
  - RUN: go build ./... ; go vet ./... ; go test -race ./...
  - RUN the new test specifically: go test -race -run 'TestLoadGitConfig_TokenLimit_DiffContext|TestLoadGitConfig_BadTokenLimit_DiffContext' ./internal/config/ -v
  - GREP: confirm both keys present and NO `!= 0` value guard on diffContext:
        grep -n "stagecoach.tokenLimit\|stagecoach.diffContext" internal/config/git.go    # → 2 keys (4 refs: 2 gitConfigGet + 2 parseInt)
        grep -n "DiffContext = intPtr\|n != 0" internal/config/git.go                    # → intPtr present; ZERO "n != 0"
  - FIX-FORWARD: if the explicit-0 row fails, a `!= 0` value guard was added — remove it.
```

### Implementation Patterns & Key Details

```go
// === git.go — the EXACT mirror template (maxDiffBytes, EXISTING — the model) ===
	if v, found, err := gitConfigGet(repoDir, "stagecoach.maxDiffBytes"); err != nil { // camelCase!
		return nil, err
	} else if found {
		if err := parseInt(repoDir, "stagecoach.maxDiffBytes", v, &c.MaxDiffBytes); err != nil {
			return nil, err
		}
	}

// === git.go — stagecoach.tokenLimit (EXACT mirror; plain int) ===
	if v, found, err := gitConfigGet(repoDir, "stagecoach.tokenLimit"); err != nil { // camelCase!
		return nil, err
	} else if found {
		if err := parseInt(repoDir, "stagecoach.tokenLimit", v, &c.TokenLimit); err != nil {
			return nil, err
		}
	}

// === git.go — stagecoach.diffContext (*int wrap; UNCONDITIONAL write inside `found`) ===
	if v, found, err := gitConfigGet(repoDir, "stagecoach.diffContext"); err != nil { // camelCase!
		return nil, err
	} else if found {
		var n int
		if err := parseInt(repoDir, "stagecoach.diffContext", v, &n); err != nil {
			return nil, err
		}
		c.DiffContext = intPtr(n) // *int (S2): nil when absent; non-nil incl. *0 when found
	}
```

```go
// === git_test.go — the table-driven proof (the explicit-0 row is load-bearing) ===
// Reuses initRepo (L24) + setGitConfig (L46) + loadGitConfig + t.Setenv("HOME", t.TempDir()).
//
//   unset:  cfg.DiffContext == nil          (PARTIAL config — overlay later inherits *1)
//   "0":    *cfg.DiffContext == 0           ← the contract's preserved-explicit-0 row
//   "1":    *cfg.DiffContext == 1
//   "3":    *cfg.DiffContext == 3
//   tokenLimit unset: cfg.TokenLimit == 0
//   tokenLimit "120000": cfg.TokenLimit == 120000
//
// The "0" row PASSES under the `found`-gated unconditional write and FAILS if a `!= 0` value guard is
// added (the 0 would be dropped → cfg.DiffContext == nil → *cfg.DiffContext panics/fails). That is the
// proof the contract's NOTE is honored.
```

```go
// === git_test.go — parse-error test (mirror TestLoadGitConfig_BadTimeout) ===
//   setGitConfig(t, repo, "stagecoach.diffContext", "abc") → loadGitConfig err != nil
//   setGitConfig(t, repo, "stagecoach.tokenLimit", "NaN")  → loadGitConfig err != nil
// parseInt returns "git config stagecoach.<key>: invalid integer %q: %w" — the same shape as the timeout
// bad-value error. Assert err != nil (do NOT assert the exact message text beyond "invalid integer").
```

### Integration Points

```yaml
GIT-CONFIG RESOLVER (internal/config/git.go loadGitConfig):
  - key added: "stagecoach.tokenLimit"   → c.TokenLimit (plain int); absent ⇒ 0
  - key added: "stagecoach.diffContext"  → c.DiffContext (*int); absent ⇒ nil; found ⇒ intPtr(n) incl. *0

PRECEDENCE (internal/config/load.go — UNCHANGED, consumes S3's output):
  - FR34: git-config (layer 3) is ABOVE the file (layer 4) ⇒ an explicit "git config stagecoach.diffContext 0"
    overrides a file's diff_context=1. S3's intPtr(0) (non-nil) flows through overlay's `!= nil` guard
    (S2) and wins. Correct by construction — S3 does NOT touch load.go.

NO-TOUCH (explicitly — owned by sibling/later subtasks):
  - internal/config/config.go            # S1/S2: intPtr (L11), TokenLimit int (L81), DiffContext *int (L82)
  - internal/config/file.go              # S2: materialize/overlay (the file→Config path)
  - internal/config/load.go              # the overlay flow (unchanged; consumes S3's partial *Config)
  - internal/config/bootstrap.go + docs/CONFIGURATION.md   # S4: the user-facing key reference + template
  - internal/git StagedDiffOptions + 6 call sites           # P1.M1.T2
  - the 3 diff functions (-U<diff_context>)                 # P1.M2+
  - any other package; PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S3):
  - S4 (bootstrap/docs): document `stagecoach.tokenLimit` / `stagecoach.diffContext` in the user-facing key
    reference + the bootstrap template. S3 is the internal resolver; S3 adds NO docs.
  - P1.M1.T2 (StagedDiffOptions): map cfg.TokenLimit / cfg.DiffContext at the call sites.
  - P1.M2+ (diff functions): deref *cfg.DiffContext (dc := 1; if cfg.DiffContext != nil { dc = *cfg.DiffContext }).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach

gofmt -w internal/config/git.go internal/config/git_test.go
gofmt -l .                       # Expected: empty after the -w
go vet ./internal/config/...     # Expected: exit 0
go build ./...                   # Expected: exit 0
```

### Level 2: Unit Tests — the table-driven proof (the contract's required verification)

```bash
cd /home/dustin/projects/stagecoach

# The new test — the explicit-0 row is the load-bearing assertion
go test -race -run 'TestLoadGitConfig_TokenLimit_DiffContext|TestLoadGitConfig_BadTokenLimit_DiffContext' ./internal/config/ -v

# Full config suite (proves the existing ReadsValues / MissingKeysIgnored / BadTimeout tests still green)
go test -race ./internal/config/ -v

# Expected: ALL PASS. The explicit "git config stagecoach.diffContext 0" row yields *cfg.DiffContext == 0.
# The unset row yields cfg.DiffContext == nil. Under a (forbidden) `!= 0` value guard the explicit-0 row
# would FAIL (0 dropped → nil) — the test passing is the proof the `found`-gated write is correct.
```

### Level 3: Whole-Repository Regression

```bash
cd /home/dustin/projects/stagecoach

go test -race ./...              # Expected: ALL packages green
go vet ./...                     # Expected: exit 0

# Confirm both keys present + the *int wrap, and NO forbidden `!= 0` value guard
grep -n "stagecoach.tokenLimit\|stagecoach.diffContext" internal/config/git.go   # → 2 keys (4 refs)
grep -n "DiffContext = intPtr" internal/config/git.go                          # → 1 match (the wrap)
grep -n "n != 0" internal/config/git.go                                        # → ZERO matches (no value guard)

# Confirm ONLY the 2 intended files changed
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/config/git.go + internal/config/git_test.go only.
```

### Level 4: End-to-End Behavior Smoke (the contract's "verify" clause — a real git repo)

```bash
cd /home/dustin/projects/stagecoach

# Prove the two keys resolve through loadGitConfig exactly as the contract specifies, against a REAL
# temp repo + real `git config` (no Go test harness — the actual user surface):
tmp=$(mktemp -d); git -C "$tmp" init -q; git -C "$tmp" config user.name T; git -C "$tmp" config user.email t@e

# Build a throwaway probe (deleted after) that calls loadGitConfig and prints the two fields.
cat > /tmp/s3_probe.go <<'EOF'
package main
import ("fmt"; "os"; "stagecoach/internal/config")
func main() {
	cfg, err := config.Load ... // NOTE: loadGitConfig is unexported; use a test instead (see below).
	_ = cfg; _ = err; _ = os.Args
	fmt.Println("use the in-package test — loadGitConfig is unexported")
}
EOF
# loadGitConfig is unexported ⇒ the authoritative end-to-end check is the in-package test (Level 2).
# The table-driven test IS the real-git-repo verification (initRepo shells out to real `git init` +
# `git config`). The explicit-0 / unset / 1 / 3 rows are the contract's "unset, =0, =1, =3" cases.
rm -f /tmp/s3_probe.go; rm -rf "$tmp"

# (If a CLI-level smoke is desired post-S4, `stagecoach --verbose` will print the resolved diff_context;
#  that surface is S4's. S3's verification is the in-package test, which exercises real git.)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test -race ./...` — all packages green (incl. the new table-driven + parse-error tests).
- [ ] `grep "stagecoach.tokenLimit\|stagecoach.diffContext" internal/config/git.go` → 2 keys present.
- [ ] `grep "n != 0" internal/config/git.go` → ZERO (no forbidden value guard); `grep "DiffContext = intPtr"` → 1.

### Feature Validation

- [ ] `stagecoach.tokenLimit` (camelCase) resolves into `c.TokenLimit` (plain int); absent ⇒ 0.
- [ ] `stagecoach.diffContext` (camelCase) resolves into `c.DiffContext` (`*int`); absent ⇒ nil.
- [ ] An explicit `git config stagecoach.diffContext 0` ⇒ `*cfg.DiffContext == 0` (write is `found`-gated, NOT `!= 0`-gated).
- [ ] A non-integer value returns a wrapped "invalid integer" error (both keys).
- [ ] The diffContext block uses `var n int` + `parseInt(&n)` + `c.DiffContext = intPtr(n)` (NOT `&c.DiffContext`).

### Scope Discipline Validation

- [ ] ONLY `internal/config/{git,git_test}.go` modified (git diff --stat confirms).
- [ ] Did NOT edit `config.go` (S1/S2), `file.go` (S2), `load.go` (overlay flow), `bootstrap.go`/docs (S4).
- [ ] Did NOT touch `StagedDiffOptions`/call sites (P1.M1.T2) or the diff functions (P1.M2+).
- [ ] Did NOT add docs (S4 owns the user-facing key reference).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] Both blocks mirror the existing `maxDiffBytes` int-key idiom (gitConfigGet → `found` gate → parseInt).
- [ ] camelCase keys match the existing `stagecoach.maxDiffBytes`/`maxMdLines`/`autoStageAll` convention.
- [ ] The DiffContext `*int` wrap reuses S2's `intPtr` helper (no reinvented pointer helper).
- [ ] The 0-vs-unset semantics are documented in code comments (the unconditional-write rationale).
- [ ] The test isolates `HOME` (no global-config leak into the unset row), mirroring the existing tests.

---

## Anti-Patterns to Avoid

- ❌ Don't use snake_case keys (`stagecoach.diff_context`, `stagecoach.token_limit`) — git rejects underscores
  ("invalid key"). The keys are EXACTLY `stagecoach.tokenLimit` and `stagecoach.diffContext` (camelCase,
  matching maxDiffBytes/maxMdLines/autoStageAll/stripCodeFence).
- ❌ Don't pass `&c.DiffContext` to `parseInt` — `Config.DiffContext` is `*int` (S2, verified), so
  `&c.DiffContext` is `**int`, incompatible with `parseInt`'s `dst *int`. Parse into `var n int` then
  `c.DiffContext = intPtr(n)`.
- ❌ Don't add a `!= 0` value guard on the DiffContext write (`if n != 0 { c.DiffContext = intPtr(n) }`) —
  that drops an explicit `git config stagecoach.diffContext 0`, the exact bug the contract's NOTE forbids.
  The `found` boolean (key present) is the ONLY gate; the write is unconditional inside `found`.
- ❌ Don't treat `Config.DiffContext` as a plain int — S2 upgraded it to `*int` (verified config.go:82).
  S3 implements against the post-S2 state, not S1's original plain int. If `&c.DiffContext` type-checks,
  S2 has NOT landed — STOP and confirm S2 is merged first.
- ❌ Don't call `Defaults()` inside `loadGitConfig` — it returns a PARTIAL *Config (absent keys stay
  zero/nil). The -U1 default is applied by `Defaults()` in load.go via overlay. An unset
  `stagecoach.diffContext` MUST yield `c.DiffContext == nil` (so overlay skips and inherits *1), NOT *1.
- ❌ Don't inline `strconv.Atoi` for the new keys — reuse `parseInt` so the error message shape
  (`git config stagecoach.<key>: invalid integer %q`) is consistent and the existing error-test pattern applies.
- ❌ Don't edit `config.go` to "add" `intPtr` or change `DiffContext`'s type — S2 already landed both
  (verified). S3 only READS them. Editing config.go crosses the S1/S2 boundary.
- ❌ Don't edit `load.go`, `file.go`, `bootstrap.go`, `docs/CONFIGURATION.md`, `StagedDiffOptions`, or the
  diff functions — those are S2 / S4 / P1.M1.T2 / P1.M2+. S3 is `internal/config/git.go` + its test ONLY.
- ❌ Don't add user-facing docs for the two keys — S4 owns the key reference + bootstrap template. S3 is the
  internal resolver (contract point 5: "DOCS: none").
- ❌ Don't write the unset-row assertion as `*cfg.DiffContext == 1` — loadGitConfig does NOT apply Defaults;
  the correct unset assertion is `cfg.DiffContext == nil`. (The *1 default appears only after overlay in load.go.)
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a two-block mechanical addition to one function, with the EXACT mirror template
(`maxDiffBytes`, git.go:181-186) quoted verbatim and the EXACT target blocks for both keys provided
ready-to-paste — including the one structural deviation (the `*int` wrap for DiffContext: `var n int` +
`parseInt(&n)` + `c.DiffContext = intPtr(n)`) that S2's `*int` design requires. The S2 dependency is NOT a
risk: it has ALREADY LANDED in the working tree (verified via grep — `Config.DiffContext` is `*int` at
config.go:82, `intPtr` exists at config.go:11, `Config.TokenLimit` is plain int at config.go:81), so S3
implements against the real post-S2 state, not a speculative contract. The contract's load-bearing NOTE
(the `found`-gated unconditional write, no `!= 0` value guard) is documented with a 4-row trace table
showing exactly how unset/0/1/3 each resolve, and the table-driven test's explicit-0 row is the load-bearing
assertion that would FAIL under the forbidden `!= 0` guard. The test idiom (`initRepo`/`setGitConfig`/
`loadGitConfig` + `t.Setenv("HOME", ...)`) is established and quoted from the existing git_test.go. The only
residual uncertainty (not 10/10) is minor placement taste (group after maxMdLines vs append before return —
both specified as acceptable) and whether the implementer adds the recommended parse-error test (a
mirror of the existing BadTimeout test, not strictly contract-required). No code outside
`internal/config/git.go` is in scope, so the blast radius is one function + its test.
