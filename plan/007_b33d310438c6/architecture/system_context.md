# System Context — Diff Payload Optimization (FR3d–FR3i)

> **Plan:** 007_b33d310438c6
> **Changeset:** the diff-capture optimizations added to PRD §9.1 in commit `ccb15c0`
> ("Add diff payload optimization requirements to PRD").
> **Read alongside:** `diff_capture_touchmap.md` (exact code seams) and
> `git_diff_semantics.md` (authoritative git facts + verification commands).

---

## 1. What this changeset IS (the gap)

The Stagecoach codebase is **mature and nearly feature-complete** against PRD v2.1. An audit
of every PRD feature confirmed that **all** of the following are ALREADY IMPLEMENTED and must NOT
be re-planned: v1 single-commit core, v2.0 multi-commit decomposition, per-role provider/model
config, binary filtering (FR3a/b/c), the agy/qwen-code providers, cascading provider priority
+ tier defaults, config bootstrap + schema versioning, payload exclusions (FR-X), message
shaping (FR-F format/locale/context/template), git hook mode (FR-H), tool integrations (FR-I),
`--edit`/`--push` (FR-E/FR-P), and discovery `models` (FR-L).

The **sole unimplemented delta** is the block of six diff-payload requirements added to §9.1 by
the latest PRD commit:

| FR | Feature | Status |
|----|---------|--------|
| **FR3d** | `token_limit` holistic token-budget overlay (supersedes legacy caps when >0) | ❌ NOT implemented |
| **FR3e** | `-M` rename detection on every `git diff` (deterministic) | ❌ NOT implemented |
| **FR3f** | `-U<diff_context>` reduced context (default `-U1`, configurable 0–3) | ❌ NOT implemented |
| **FR3g** | Compact `--numstat` change skeleton prepended to the payload | ❌ NOT implemented |
| **FR3h** | Strip the `index <oid>..<oid> <mode>` line from each file diff | ❌ NOT implemented |
| **FR3i** | Dynamic water-fill truncation under `token_limit` | ❌ NOT implemented |

Grep evidence: `TokenLimit`/`DiffContext`/`token_limit`/`diff_context` appear **nowhere** in
non-test `*.go`. The diff functions pass raw `git diff --cached` / `git diff A B` / `git diff`
with **no** `-M`, **no** `-U`, **no** numstat skeleton, **no** index stripping, **no** token
budget.

**Scope is precisely FR3d–FR3i and their config/option plumbing. Nothing else.**

---

## 2. The three sibling diff functions (FR3c parity is mandatory)

All six optimizations must apply identically in **every diff path** (PRD FR3c). Three functions
in `internal/git/git.go` are the implementation surface:

| Function | Lines | Diff domain |
|----------|-------|-------------|
| `StagedDiff` | `642–764` | `git diff --cached` (single-commit + hook + pkg) |
| `TreeDiff` | `1094–1207` | `git diff <treeA> <treeB>` (decompose concept diffs + arbiter) |
| `WorkingTreeDiff` | `1228–1341` | `git diff` (working-tree-vs-index; **no live caller** — test-only today) |

**Critical structural fact (from `diff_capture_touchmap.md` §1):** these three functions
**DUPLICATE their logic inline** — there is NO shared argv/cap helper. Each has the same
three-part structure (markdown per-file → binary/excluded placeholders → non-markdown
aggregate). Each builds its `git diff` argv at **3 sites** (markdown name-list, markdown
per-file, non-markdown aggregate) = **9 argv sites total** that all need `-M` + `-U<n>`.

➡ **Recommended first step: extract a shared `buildDiffArgs(domain, opts)` helper** to collapse
the 9× duplication, then every flag/skeleton/strip change is single-site. This is M2.T1.

The existing variadic-`diffArgs` helpers in `internal/git/binary.go`
(`detectBinaryFiles(ctx, diffArgs...)`, `fileStatuses(ctx, diffArgs...)`) are the proven
pattern to mirror.

---

## 3. Config & option plumbing (the seams)

See `diff_capture_touchmap.md` §2–§4 for exact line numbers. Summary:

- **`StagedDiffOptions`** (`internal/git/git.go:36–44`): add `TokenLimit int`,
  `DiffContext int`, and `PromptReserveTokens int` (the last measured upstream — see §5 below).
- **`config.Config`** (`internal/config/config.go:77–93`, flat `[generation]` fields): add
  `TokenLimit int toml:"token_limit"` (default 0) and `DiffContext` (default 1) next to
  `MaxDiffBytes`/`MaxMdLines`.
- **Wiring (3 sites, all next to `MaxDiffBytes`/`MaxMdLines`):** `materialize`
  (`file.go:~205–215`), `overlay` (`file.go:~308–318`), git-config resolver
  (`internal/config/git.go:181–205` — keys `stagecoach.tokenLimit` / `stagecoach.diffContext`,
  camelCase per existing `maxDiffBytes`/`maxMdLines` convention).
- **`Defaults()`** (`config.go:155–190`): `TokenLimit: 0`, `DiffContext: 1`.
- **`bootstrap.go` template** (`~282–298`): commented `token_limit`/`diff_context` lines;
  annotate legacy caps "ignored when token_limit is set".
- **6 production call-site struct literals** (`diff_capture_touchmap.md` §2 table):
  `generate.go:163`, `hook/exec.go:104`, `stagecoach.go:423`, `decompose/planner.go:69`,
  `decompose/message.go:71`, `decompose/decompose.go:608`. Each maps `cfg → git.StagedDiffOptions`
  inline; add `TokenLimit`/`DiffContext` (+ `PromptReserveTokens` where applicable).

### ⚠ Design decision: `DiffContext` 0-vs-unset

`DiffContext` default is `1`, but `0` is a VALID value (`-U0` = changed lines only, FR3f). The
standard `if src.X != 0` overlay guard conflates "unset" with "explicit 0" → an explicit `0`
would be silently overridden by the default `1`. **Resolution (recommended): use `*int` pointer
for the file-layer field** — there is direct precedent (`Output *string`, `StripCodeFence *bool`
at `config.go:88–89`). Resolve to a concrete `int` on `Config.DiffContext` with default 1, where
"nil pointer ⇒ default 1". The implementer must handle this in `materialize`/`overlay`.

---

## 4. Authoritative git facts (empirically verified against git 2.54.0)

All facts in `git_diff_semantics.md` were re-verified by running real git commands. Lock-in
results:

- **`-M` (rename):** `git diff --cached -M` emits a pure rename as just
  `similarity index 100%` / `rename from` / `rename to` (NO index line, NO patch body —
  cheapest form). `-M` **wins over `diff.renames=false`** (confirmed). Use `-M` only, never
  `-C` (O(files²), no value).
- **`-U<n>` (context):** `-U1` and `-U0` are valid; default is `-U3`. Composes with `--cached`
  and tree-to-tree `git diff A B`. `-U1` is the recommended default (one anchor line, ~token
  savings vs `-U3`).
- **`--numstat`:** one tab-separated line per file `added \t deleted \t path`; binary =
  `-\t-\tpath`. **With `-M` the path column shows `old => new` rename notation** (and even
  without `-M`, git 2.54 shows it — renames are on by default). ➡ For the FR3g skeleton sizing
  (FR3i), **run numstat WITHOUT `-M` OR parse defensively for `=>`/`{...}`**. Cleanest: parse
  the path field, splitting on ` => ` and taking the destination (right side) to key the
  size map, so it matches the patch's `b/` destination paths.
- **`index` line:** shape `index <oid>..<oid> <mode>` (e.g. `index 600d48a..62b056e 100644`).
  **No git flag suppresses it** — post-capture line stripping is the only way. Strip every line
  matching `^index `. KEEP `diff --git`, `---`, `+++`, `@@`, content, and (recommended)
  `rename from`/`rename to`/`similarity index`.
- **Token estimate:** `tokens ≈ chars / 4` (conservative `chars / 3` for code-heavy diffs).
  Model-agnostic; standard.
- **Water-fill:** find level `L` such that `Σ min(s_i, L) = body_budget`; files ≤ L kept whole,
  files > L capped at L. O(n log n) sort-and-walk (algorithm in `git_diff_semantics.md` §6).
  Verified on `[10,20,30]@30→[10,10,10]`, `[5,100]@50→[5,45]`, `[10,10]@50→no truncation`.

---

## 5. The FR3i coupling seam (prompt-reserve)

PRD FR3i: `body_budget = token_limit − skeleton − prompt − margin`, where "prompt" = the system
prompt + style examples (FR11). The git layer owns the diff body and the numstat sizing (dual-use:
skeleton for the model + size input for water-fill). The prompt portion is built **after**
`StagedDiff` returns, so its size is not known inside the diff function.

**Chosen seam (lowest coupling):** add `PromptReserveTokens int` to `StagedDiffOptions`. Each
call site measures the **stable** prompt portion (system-prompt header constants + the
`RecentMessages` examples + the user-instruction line + a worst-case rejection-block estimate =
`maxDuplicateRetries × ~max-rejection-line-len` + a safety margin) using the shared
chars/4 estimator, BEFORE calling the diff function, and passes the reserve in. The git layer
then computes `body_budget = token_limit − skeleton_tokens − promptReserve` and water-fills.
A shared `estimateTokens(s string) int` utility (chars/4) is the single source of truth for
both the prompt-reserve measurement and the per-file numstat sizing.

> When `token_limit == 0`, `PromptReserveTokens` is ignored and the legacy per-section caps
> apply byte-identically (the regression invariant, §6).

---

## 6. Regression invariants (acceptance criteria)

1. **`token_limit == 0` (default) ⇒ byte-identical legacy behavior EXCEPT the always-on FR3f/FR3e/FR3g/FR3h transforms.** Concretely:
   - FR3e (`-M`), FR3g (skeleton prepend), FR3h (index-strip) are **always on** regardless of `token_limit`. They alter output for every run.
   - FR3f (`-U1` default context) **replaces git's `-U3`** — this CHANGES existing golden tests even at `token_limit==0`.
   - The legacy `max_diff_bytes`/`max_md_lines` caps and their `... [diff truncated at N bytes/lines]` sentinels remain byte-identical ONLY for the diff *body*; the surrounding transforms still apply.
2. **`token_limit > 0` ⇒ water-fill replaces the byte/line caps.** The `... [truncated]` sentinel (shorter form, per PRD FR3i) is emitted per truncated file; the `at N bytes` sentinels do NOT appear.
3. **Existing test fixtures WILL change** under FR3f (`-U3`→`-U1`) and FR3h (index lines gone) and FR3g (skeleton added). Update them; these are expected deltas, not regressions. The fixtures that assert on `diff --git a/<file>` boundary markers (kept) and the truncation sentinels (kept at `token_limit==0`) are the stability anchors.
4. **Payload-only, never commit-affecting.** Every transform is on what the agent *sees*; the snapshot/commit are untouched (same invariant as `[binary]`/`[excluded]` placeholders).

---

## 7. Data flow (end to end)

```
config.toml / git-config (token_limit, diff_context)
   → Defaults() + materialize() + overlay() [file.go] + resolveGitConfig() [git.go]
   → config.Config{TokenLimit(NEW), DiffContext(NEW), MaxDiffBytes, MaxMdLines, ...}
   → 6 call sites: cfg → git.StagedDiffOptions{TokenLimit, DiffContext, PromptReserveTokens(NEW), ...}
                    + measure PromptReserveTokens upstream (chars/4 over stable prompt parts)
   → StagedDiff / TreeDiff / WorkingTreeDiff(opts)   [git.go: 642 / 1094 / 1228]
        ├─ buildDiffArgs(domain, opts) [NEW shared helper] → "diff" + domain + "-M" + "-U<n>"
        ├─ Part 1: md per-file diff (-M -U<n>) → line cap (token_limit==0) or unchanged
        ├─ binary/excluded placeholders (unchanged)
        ├─ FR3g: numstat skeleton → prepend compact skeleton block (always on)
        ├─ Part 2: non-md aggregate diff (-M -U<n>)
        │     ├─ FR3h: strip "^index " lines (always on)
        │     ├─ token_limit==0: byte cap + "... [truncated at N bytes]" (unchanged)
        │     └─ token_limit>0:  size by numstat → water-fill → per-file cap + "... [truncated]"
        └─ returns payload string (skeleton + placeholders + md + shaped non-md)
   → prompt.BuildUserPayload / BuildPlannerUserPayload (diff = verbatim tail)
   → provider.Render → agent stdin
```

---

## 8. Out of scope (do NOT plan these)

- Any feature outside FR3d–FR3i (all other PRD features are implemented).
- A `--token-limit` / `--diff-context` CLI flag (PRD specifies config-file + git-config keys
  only for these two; FR35 lists the env var set and neither appears). Env var
  `STAGECOACH_TOKEN_LIMIT` / `STAGECOACH_DIFF_CONTEXT` are also NOT in FR35 — config/git-config
  only. (If the implementer finds value in flags, flag it, but the PRD does not require them.)
- Copy detection (`-C`) — explicitly rejected by FR3e.
- A per-model context-window registry — FR3d is model-agnostic by design (`token_limit` is
  user-set to their model's window; no registry).
- Changing the committed content — all transforms are payload-only.

---

## 9. Risks flagged for implementers

1. **9× flag-insertion duplication** — extract the shared helper FIRST (M2.T1) or risk
   divergence between the three functions.
2. **`DiffContext` 0-vs-unset** — use `*int` (precedent: `Output`/`StripCodeFence`); see §3.
3. **FR3i prompt-reserve timing** — StagedDiff is called once before the retry loop; measure
   the stable prompt portion upstream (§5).
4. **numstat rename notation** — parse the `=>`/`{...}` path field or run numstat without `-M`
   for the size map; key the size map on the destination path to match the patch's `b/` paths.
5. **Golden-test churn** — FR3f (`-U1`) + FR3h (index-strip) + FR3g (skeleton) change existing
   fixtures even at `token_limit==0`; update them deliberately (§6).
6. **`WorkingTreeDiff` has no live caller** — parity changes there are exercised only by its
   unit tests; keep them green but don't expect e2e coverage.
