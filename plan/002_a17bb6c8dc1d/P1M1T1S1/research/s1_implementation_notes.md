# S1 Implementation Notes — TooledFlags + Experimental on Manifest

> Scope: P1.M1.T1.S1 — add `TooledFlags []string` + `Experimental *bool` to the provider Manifest,
> update Resolve() (Experimental default → false; TooledFlags left nil), no new Validate rules.
> Verified against the live source on 2026-07-01.

## 1. The exact current Manifest struct + conventions (internal/provider/manifest.go)

Two field regimes, documented in the struct's own doc comment:
- **Pointer scalars** (`*string`/`*bool`): nil = absent (→ inherit/default), non-nil = explicit (even
  `*false` / `*""`). Helpers: `strPtr(s)`, `boolPtr(b)` (manifest.go bottom).
- **Slices** (`[]string`): nil = natural "absent" sentinel (absent → nil; present → non-nil even if
  empty). The doc comment names them explicitly: "Slices (Subcommand, BareFlags) and the Env map stay
  plain". BareFlags/Subcommand are the precedent.

The two new fields map cleanly onto the existing regimes (contract point 1):
- `TooledFlags []string` → SLICE regime (same as BareFlags/Subcommand). toml `tooled_flags`.
- `Experimental *bool`   → POINTER-SCALAR regime (same as StripCodeFence). toml `experimental`.

## 2. Exact placement + doc comments (from architecture/manifest_v2_delta.md §1)

Place TooledFlags immediately after BareFlags (its tooled-mode analog), under a new section comment;
place Experimental after it. The delta gives the verbatim comments:

```go
	// --- tooled mode (v2; §11.5, §12.1) ---
	// Flags for the STAGER role (tools on, git-scoped, non-interactive). nil/empty => this provider
	// does not support tooled mode and cannot serve as a stager. Used in place of BareFlags when
	// mode=="tooled" in Render.
	TooledFlags []string `toml:"tooled_flags"`

	// --- experimental (§12.7.2, §12.5.1) ---
	// true => provider ships from docs/issue-tracker research, not a verified --help. `providers list`
	// marks experimental providers distinctly.
	Experimental *bool `toml:"experimental"`
```

So they slot into the existing struct between `BareFlags` and the `// --- output ---` block.

## 3. Resolve() — exactly one new line; TooledFlags left as-is (manifest_v2_delta.md §1)

Add the Experimental default among the pointer-scalar defaults (logically next to StripCodeFence,
the other *bool, or at the end of the pointer block):
```go
	if out.Experimental == nil {
		out.Experimental = boolPtr(false)
	}
```
TooledFlags is NOT touched in Resolve — nil stays nil (same as Subcommand/BareFlags). The existing
trailing comment `// Subcommand / BareFlags / Env: left as-is (nil stays nil).` MUST be extended to
include TooledFlags: `// Subcommand / BareFlags / TooledFlags / Env: left as-is (nil stays nil).`

Also: the struct-level doc comment enumerates "Slices (Subcommand, BareFlags)" — extend to
"(Subcommand, BareFlags, TooledFlags)" for consistency. And the Resolve doc comment says "The four
PRD-defaulted fields take their §12.1 defaults" — Experimental is a §12.7.2 default (false), so either
keep "four" (referring strictly to §12.1) or reword; not load-bearing.

## 4. Validate() — NO new rules (contract + delta §1 confirm)

The two fields have no enum/required semantics at this layer:
- TooledFlags is a free-form flag slice (validated only at Render-time in P1.M1.T2: tooled mode with
  empty tooled_flags → error — NOT this subtask's concern).
- Experimental is a *bool; nil is allowed (Resolve defaults it). No value to reject.

So Validate() is UNCHANGED. (A nil Experimental passes Validate; Resolve guarantees non-nil after.)

## 5. Nothing else compiles-broken (verified)

- **builtin.go**: every `builtinXxx()` uses NAMED struct literals (`Manifest{Name:..., BareFlags:...}`)
  — none enumerates all fields. Adding two fields is a no-op for them (the new fields default to nil).
  Do NOT add agy here — that is P1.M2.T1.S1 (separate subtask).
- **merge.go**: MergeManifest handles TooledFlags/Experimental in S2 (P1.M1.T1.S2) — NOT this subtask.
- **render.go**: Render gains a mode param in P1.M1.T2.S1 — NOT this subtask. Existing Render ignores
  TooledFlags (uses BareFlags only) and still compiles.
- **parse.go / executor.go**: read Output/StripCodeFence only; unaffected.

## 6. Test extensions in manifest_test.go (natural, low-risk — prove the new contract)

Existing tests that should be EXTENDED (they already assert the slice/pointer regimes):
- `TestResolve_SlicesLeftNil` (:398) — currently asserts Subcommand + BareFlags stay nil. ADD:
  `r.TooledFlags` stays nil (proves TooledFlags follows the slice regime — not defaulted in Resolve).
- `TestResolve_AppliesDefaultsToNilOptionals` (:343) — asserts defaults applied. ADD:
  `r.Experimental != nil && *r.Experimental == false` (proves the false default is materialized).
- `TestResolve_PreservesExplicitValues` (:355) — asserts explicit values preserved. ADD:
  Experimental: boolPtr(true) in the input → `*r.Experimental == true` (NOT clobbered to false).
- OPTIONAL: `TestUnmarshal_FullManifest` (:30) — the pi TOML has no tooled_flags/experimental, so both
  decode to nil; could assert `m.TooledFlags == nil && m.Experimental == nil`. (Mild value; not required.)

These reuse the same-package `boolPtr`/`strPtr` helpers and the existing `Manifest{...}` literal style.
No new test file; co-located in manifest_test.go.

## 7. Scope discipline — what S1 does NOT do

- NOT merge.go (S2 owns MergeManifest for the two fields).
- NOT render.go / RenderMode (P1.M1.T2.S1 owns the bare/tooled mode param).
- NOT builtin.go / agy / providers/*.toml (P1.M2.T1/T2 own agy + tooled_flags on pi/claude).
- NOT user-facing docs/*.md (contract: "No user-facing docs change yet" — inline struct comments only).
- NOT PRD.md / tasks.json / prd_snapshot.md / plan/*.
