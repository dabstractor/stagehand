# Merge Semantics & the Map-Aliasing Gotcha (P1.M2.T1.S2)

> Short, decisive note backing the `MergeManifest` design. Read alongside
> `../P1M2T1S1/research/go-toml-pointer-behavior.md` (why pointers exist) and
> `../../architecture/go_ecosystem_patterns.md` §2.4 (the overlay pattern).

## 1. The three merge regimes (from the work-item contract)

`MergeManifest(base, override Manifest) Manifest` overlays the override onto a copy of `base`,
per PRD §16.1 ("a user override that sets only `default_model` leaves all other fields from the
built-in manifest intact"). The contract pins THREE different rules by field kind:

| Field kind            | Fields                                          | Rule                                                          |
|-----------------------|-------------------------------------------------|---------------------------------------------------------------|
| Scalar pointer (`*string`/`*bool`) | Detect, Command, PromptDelivery, PromptFlag, PrintFlag, ModelFlag, DefaultModel, SystemPromptFlag, ProviderFlag, DefaultProvider, Output, JsonField, StripCodeFence, RetryInstruction | `override.Field != nil` → take override. **Explicit `""`/`false` WINS** (non-nil is the override signal — this is the entire point of S1's pointer design). |
| Slice (`[]string`)    | Subcommand, BareFlags                           | `len(override.Slice) > 0` → **replace wholesale** (no element merge). Empty/nil → keep base. |
| Map (`map[string]string`) | Env                                          | **merge key-by-key** into the result: each override key overwrites the same base key; base keys absent from override survive. nil/empty override → keep base. |

Why slices use "non-empty" (len>0) instead of "non-nil": a user-written `bare_flags = []` is almost
never a deliberate "clear the built-in flags" intent; treating empty as "not overridden" is the safe,
predictable reading and matches the contract's literal wording ("if override is non-empty, replace
entirely"). (Decode still produces non-nil-but-empty for `bare_flags = []` per S1 FINDING D — merge
simply chooses to treat that as a no-op, which is the contract.)

## 2. The CRITICAL map-aliasing bug (and the fix)

`out := base` copies the **struct header** — slices, maps, and pointers are copied **by header**,
so `out.Env`, `out.Subcommand`, `out.BareFlags` still point at the **same underlying data** as
`base`.

- **Slices are safe.** We only ever **reassign the header** (`out.BareFlags = override.BareFlags`)
  on a non-empty override, or leave it untouched. We never `append` to or index-assign the shared
  backing array, so the caller's `base.BareFlags` is never mutated. (Read-only sharing of slice
  backing arrays is idiomatic and cheap.)
- **The Env map is NOT safe** with the naive approach. `out.Env[k] = v` would mutate the caller's
  `base.Env` map (same object). MergeManifest MUST be side-effect-free on its inputs.

**Fix — allocate a fresh map and copy both sides into it:**

```go
if len(override.Env) > 0 {
    merged := make(map[string]string, len(base.Env)+len(override.Env))
    for k, v := range base.Env {
        merged[k] = v
    }
    for k, v := range override.Env {
        merged[k] = v // override key wins
    }
    out.Env = merged // breaks the alias to base.Env
}
```

`TestMergeManifest_DoesNotMutateInputs` asserts `base.Env` is byte-identical before/after the call
(this is the test that would catch the aliasing bug if the fresh-map step were forgotten).

## 3. Name is deliberately NOT field-merged

`result.Name = base.Name` (a side-effect of `out := base`; no Name-override logic is added).
Rationale: `name` is the `[provider.<name>]` **table key**, never written into the table body, so a
decoded override always has `Name == ""`. The registry (P1.M2.T3) owns setting the final `Name` from
the table key — it must do so for brand-new providers (§12.8, where `base` is the zero `Manifest`)
anyway. Keeping Name out of the field-merge makes MergeManifest a pure, predictable overlay. The
`TestMergeManifest_NamePreservedFromBase` test pins this.

## 4. MergeManifest does NOT call Validate

A partial override legitimately lacks `Command` (it inherits the built-in's). So MergeManifest
returns the merged struct **without** validating — `Validate` is the **registry's** (P1.M2.T3)
post-merge step (lifecycle: decode → merge → Validate → Resolve → consume). One test
(`TestMergeManifest_MergedResultValidates`) still asserts a fully-merged result passes `Validate`,
proving S2 composes correctly with S1.

## 5. No new imports, no new deps

`merge.go` is pure logic (field assignment, `len`, `for range`, `make`) → **zero imports**, stays
`package provider`, stdlib-only — consistent with S1's design call #4 (provider imports nothing
outside stdlib; no config, no toml). `go.mod`/`go.sum` byte-unchanged.
