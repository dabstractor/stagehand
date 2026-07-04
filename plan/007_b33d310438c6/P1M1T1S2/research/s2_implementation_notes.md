# S2 Implementation Notes — materialize + overlay field-merge (DiffContext *int)

> Scope: P1.M1.T1.S2 — wire `TokenLimit` + `DiffContext` from the file decode struct through
> `materialize()` and `overlay()` into `config.Config`. **A contract correction is required**: the
> contract's prescribed overlay guard `if src.DiffContext != 0` is PROVABLY BROKEN (see §1). The only
> correct fix is `Config.DiffContext = *int` (nil = unset), which extends S1's `*int` from the FILE
> struct to the resolved Config. Verified against the live load flow 2026-07-04.

## 0. What S1 produces (the input)

S1 adds two flat fields (plain `int` on both structs) + Defaults seeds:
- `config.Config`: `TokenLimit int` + `DiffContext int` (PLAIN int — S1's boundary).
- `fileGeneration`: `TokenLimit int` + `DiffContext int` (plain in S1; S2 changes DiffContext to `*int`).
- `Defaults()`: `TokenLimit: 0` (FR3d unset), `DiffContext: 1` (FR3f -U1).
- `config_test.go TestDefaults`: asserts `TokenLimit==0`, `DiffContext==1`.
- Nothing reads them yet. Baseline `go test ./internal/config/` is GREEN.

S1's stated INTENT (verbatim): "fileGeneration.DiffContext becomes *int in S2 to disambiguate 0
(changed-lines-only) from unset." S1 scoped the `*int` to the FILE struct only and did NOT realize the
disambiguation is lost the moment `materialize` collapses `*int → plain int` (S2 must extend `*int` to
`Config.DiffContext` too — see §1).

## 1. CONTRACT CORRECTION — the overlay guard is broken (proven via the load flow)

The S2 contract prescribes, in `overlay()`: `if src.DiffContext != 0 { dst.DiffContext = src.DiffContext }`,
claiming "plain `!= 0` is fine HERE ... an explicit 0 set during materialize propagates as a real 0."
**This claim is false.** Trace the actual load flow (`internal/config/load.go`):

```
Load():
  cfg := Defaults()                          // cfg.DiffContext = 1              (load.go:82)
  g   := loadTOML(globalPath)                // g = materialize(fileConfig)      (load.go:96)
  overlay(&cfg, g)                           // merge global file onto Defaults  (load.go:100)
  r   := loadRepoLocalConfig()               // r = materialize(repoFile)
  overlay(&cfg, r)                           // merge repo onto (Defaults+global)(load.go:123)
  gc  := loadGitConfig(repoDir)              // gc: "designed for NON-ZERO overlay" (git.go:106)
  overlay(&cfg, gc)                          // merge git-config                  (load.go:138)
```

`overlay` is between EVERY layer (global file, repo file, git config) and the final config. With the
contract's `!= 0` guard, an explicit `diff_context = 0` in a file traces:

1. `g := materialize(file)` → `g.DiffContext = 0` (the `*int` path correctly produces 0).
2. `overlay(&cfg, g)` → `src.DiffContext = 0` → `0 != 0` is **FALSE** → `cfg.DiffContext` is NOT
   updated → stays `1` (Defaults).

**Result: `cfg.DiffContext == 1`, not 0.** The explicit 0 is silently clobbered by overlay's `!= 0`
guard. The contract's OWN verification ("a user setting diff_context=0 in a file yields
Config.DiffContext==0 end-to-end") FAILS — in global-only, repo-only, AND global+repo scenarios,
because overlay is always in the path. `git.go:106` confirms even the git-config layer uses the
non-zero overlay, so diff_context=0 cannot be set via ANY layer under the contract's design.

**Root cause (information-theoretic):** `overlay` merges `Config → Config` (plain ints). With a plain
int, "unset" (0 after materialize) and "explicit 0" are INDISTINGUISHABLE — both are `0`. The non-zero
overlay convention (a documented v1 limitation: "a file cannot override a field to its zero value",
`materialize` doc comment) therefore CANNOT express "override to 0". For `MaxDiffBytes` this is fine
(0 is never valid), but for `DiffContext` 0 IS a valid, meaningful value (FR3f: "0 = changed lines only,
maximal savings"). The `*int` on `fileGeneration` disambiguates file-decode, but that bit is LOST at
`materialize`'s `*int → int` collapse and overlay cannot recover it.

## 2. The correct fix — Config.DiffContext MUST be *int

The ONLY way to satisfy FR3f (diff_context=0 file-configurable) + the contract's end-to-end test is to
make `Config.DiffContext` a `*int` (nil = unset), exactly like the existing nullable-scalar precedents
`Output *string` and `StripCodeFence *bool` (config.go:88-89, 97). Then:

- `Defaults()`: `DiffContext: intPtr(1)` (non-nil ⇒ the -U1 default is a real value, not "unset").
- `fileGeneration.DiffContext`: `*int` (S1 already planned this).
- `materialize`: `if g.DiffContext != nil { c.DiffContext = g.DiffContext }` (copy the POINTER; nil ⇒ unset).
- `overlay`: `if src.DiffContext != nil { dst.DiffContext = src.DiffContext }` (nil ⇒ inherit lower layer).

Trace the fixed design for explicit 0, global-only:
1. `g := materialize(file with diff_context=0)` → `g.DiffContext = intPtr(0)` (non-nil).
2. `overlay(&cfg, g)` → `src.DiffContext != nil` TRUE → `cfg.DiffContext = intPtr(0)`.
3. **Result: `cfg.DiffContext == intPtr(0)` → `*cfg.DiffContext == 0`. ✅**

And unset: `g.DiffContext = nil` → overlay skips → `cfg.DiffContext` stays `intPtr(1)`. ✅

This is the design the contract's own verification requires. **It supersedes S1's "Config.DiffContext
plain int" scoping** — S1's intent (0-vs-unset) is only achievable with `*int` on Config, not just on
the file struct. S2 therefore re-edits the three config.go spots S1 added (field, Defaults seed,
TestDefaults assertion). See §4 for the parallel-edit coordination.

### TokenLimit is FINE with plain int + `!= 0` everywhere
FR3d: `token_limit` default `0` = unset ⇒ legacy caps; "a non-zero token_limit supersedes both legacy
caps." There is no meaningful "explicit 0" — 0 IS the unset sentinel. So `TokenLimit` plain int with
`if .TokenLimit != 0` in BOTH materialize and overlay is correct (matches MaxDiffBytes/MaxMdLines).
**Only DiffContext needs the `*int` treatment.**

## 3. The exact edits (corrected design)

### config.go
1. Add an `intPtr` helper next to `boolPtr`/`strPtr` (lines 7-9):
   `func intPtr(i int) *int { return &i }`
2. Config struct: `DiffContext int` → `DiffContext *int` (comment: `*int — nil ⇒ unset (default 1/-U1); non-nil incl. *0 ⇒ explicit (FR3f 0=changed-lines-only). *int not plain int so overlay can distinguish unset from explicit 0.`). **TokenLimit stays plain `int`.**
3. `Defaults()`: `DiffContext: 1,` → `DiffContext: intPtr(1),` (comment: FR3f -U1 default; non-nil).

### config_test.go
4. `TestDefaults`: `DiffContext` assertion becomes `if c.DiffContext == nil || *c.DiffContext != 1`.
   (TokenLimit assertion `c.TokenLimit != 0` unchanged.)

### file.go
5. `fileGeneration.DiffContext`: plain `int` → `*int`. (TokenLimit plain `int`.)
6. `materialize` (next to MaxDiffBytes/MaxMdLines guards ~line 212-216):
   ```go
   if g.TokenLimit != 0 {
       c.TokenLimit = g.TokenLimit
   }
   if g.DiffContext != nil {
       c.DiffContext = g.DiffContext // *int: nil ⇒ unset; non-nil (incl. *0) ⇒ override
   }
   ```
7. `overlay` (next to MaxDiffBytes/MaxMdLines guards ~line 314-318):
   ```go
   if src.TokenLimit != 0 {
       dst.TokenLimit = src.TokenLimit
   }
   if src.DiffContext != nil {
       dst.DiffContext = src.DiffContext
   }
   ```

### file_test.go
8. Table-driven test (materialize + overlay): unset⇒1, explicit 1⇒1, explicit 0⇒0, explicit 3⇒3, across
   global-only / repo-only / global+repo overlay. See the PRP for the full table. All rows PASS under
   the `*int` design (and FAIL under the contract's literal `!= 0` overlay — which is the proof the
   correction is necessary).

## 4. Parallel-edit coordination with S1 (IMPORTANT)

S2 re-edits the THREE config.go spots S1 is adding in parallel (Config.DiffContext field, Defaults seed,
TestDefaults assertion) — changing DiffContext from plain int to `*int`. Two ways to reconcile:
- **Preferred:** fold the `*int` into S1 (S1 makes `Config.DiffContext` `*int` + `Defaults intPtr(1)` +
  `TestDefaults *DiffContext==1` from the start). S1's PRP already planned `*int` for the file struct;
  extending it to Config is a one-field refinement that makes S1's stated 0-vs-unset intent actually work.
- **Fallback:** S2 re-edits config.go after S1 lands (plain int → `*int`). The orchestrator sequences
  S2 after S1 to avoid a merge conflict on the DiffContext field line.

Either way, S2's file.go edits (materialize/overlay/fileGeneration) are S2's own — no conflict there.

## 5. Ripple to S3 + future consumers (informational — NOT S2's edits)

- **S3 (git.go loadGitConfig):** when `stagehand.diffContext` is found, set `c.DiffContext = intPtr(v)`
  (nil when absent) — NOT plain int — so the git-config layer also distinguishes unset from explicit-0.
  git.go:106's "non-zero overlay" comment needs a carve-out note for DiffContext (now `*int`/`!= nil`).
- **Future consumers (P1.M1.T2 StagedDiffOptions, P1.M2 -U<diff_context>):** will deref `*cfg.DiffContext`
  (default 1 when nil-safe — resolve via `dc := 1; if cfg.DiffContext != nil { dc = *cfg.DiffContext }`).
  Those subtasks aren't written yet → no current break.

## 6. Scope discipline — what S2 does NOT do

- NOT the git-config resolver (S3 — but S2's `*int` design is the contract S3 must follow).
- NOT the bootstrap template / docs (S4 / P1.M1.T4).
- NOT StagedDiffOptions / the 6 call sites / the diff functions (P1.M1.T2 / P1.M2+).
- NOT PRD.md / tasks.json / prd_snapshot.md / plan/*.

## 7. Sources

- `load.go:82,100,123,138` — the Defaults→overlay flow (overlay is in every layer's path).
- `git.go:106` — confirms the git-config layer is also "designed for NON-ZERO overlay()".
- `config.go:7-9,88-89,97` — the `boolPtr`/`strPtr` helpers + `Output *string`/`StripCodeFence *bool` nullable-scalar precedent.
- `file.go:186-282` (materialize) + `283-330` (overlay) — the non-zero merge convention + the doc comment "a file cannot override a field to its zero value".
- PRD §9.1 FR3d (token_limit, 0=unset) + FR3f (diff_context, 0=changed-lines-only); §16.1 layer-1 defaults.
- `P1M1T1S1/PRP.md` — S1's plain-int boundary + the stated 0-vs-unset intent for fileGeneration.DiffContext.
