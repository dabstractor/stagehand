# Delta PRD — Payload / Diff Optimization (FR3d–FR3i)

> **Plan:** 007_b33d310438c6 · **Source PRD sections changed:** §9.1 (FR3d–FR3i), §16.1, §16.2 config example.
> **Composes over:** the fully-implemented v2.0/v3/v2.1 core + plan 006's FR52 run lock. No commit / rescue / CAS / lock logic is touched.

---

## 0. What changed (precise diff)

`diff plan/006_c23e6f286ae7/prd_snapshot.md PRD.md` adds **6 new functional requirements** under §9.1 and **2 small config updates** under §16. ~14 changed lines, all in one cohesive area: **how the staged/working-tree/tree-to-tree diff payload is shaped before it reaches the agent.**

| FR | Title | Essence | Size |
|---|---|---|---|
| **FR3d** | Token budget overlay (model-agnostic) | New `token_limit` config key (integer tokens; `0` = unset ⇒ legacy per-section caps). When set, governs the **total** payload (prompt + examples + diff), superseding `max_diff_bytes` + `max_md_lines` for that run. Mutually exclusive with the legacy caps. | Config plumbing |
| **FR3e** | Rename detection (deterministic) | Pass `-M` on **every** `git diff` invocation for compact rename representation (regardless of user `diff.renames` config / git version). `-C` deliberately NOT enabled (O(files²), no value). | Tiny flag add |
| **FR3f** | Reduced diff context | Capture diffs with `-U<diff_context>` (default `1`, configurable `0`–`3`) instead of git's `-U3` default. `0` = changed lines only. | Tiny flag add |
| **FR3g** | Compact change skeleton (completeness floor) | Prepend `git diff --cached --numstat` (one `added\tdeleted\tpath` line per changed file, rename/status annotated) **before** any diff body. Guarantees the model sees the full shape of the change even when bodies are truncated. `--numstat` over `--stat` (no ASCII bars). Dual-use: also the sizing input for FR3i. | Small prepend |
| **FR3h** | Index-line stripping | Strip the `index <oid>..<oid> <mode>` line from each file diff (~30 bytes/file, useless blob OIDs). Retain `diff --git`/`---`/`+++`/`@@`. | Small post-process |
| **FR3i** | Truncation under `token_limit` (dynamic water-fill) | When `token_limit` set and payload exceeds it: allocate the diff body across files by a **dynamic, size-aware water-fill** (no static per-file cap). `body_budget = token_limit − skeleton − prompt − margin`. If `Σ size_i ≤ body_budget`, include everything whole (common case). Else find water level `L` with `Σ min(size_i, L) = body_budget`: files smaller than `L` whole + untouched; files larger truncated to exactly `L` (first `L` tokens + `... [truncated]` sentinel). Minimizes truncation, fully utilizes budget, no file monopolizes. | **The algorithmic core** |

**Config changes (§16.1 + §16.2):**
- `defaults.go`: add `token_limit = 0` (unset ⇒ legacy caps, FR3d) and `diff_context = 1` (FR3f).
- `[generation]` config example: add `token_limit` and `diff_context` keys with explanatory comments; annotate the legacy caps as "ignored when token_limit is set".

**Nothing was removed or rewritten.** FR3a–FR3c (binary filtering, already implemented) are unchanged; the new FRs extend the same "applies in every diff path" contract (FR3c parity).

---

## 1. Scope delta

### In scope (build)
- **FR3e, FR3f, FR3h** — always-on diff optimizations applied unconditionally in every diff path (`StagedDiff`, `TreeDiff`, `WorkingTreeDiff` — FR3c parity).
- **FR3g** — numstat skeleton prepended in every diff path (always-on; also serves as FR3i's sizing input).
- **FR3d + FR3i** — the opt-in `token_limit` mode: config field + the dynamic water-fill truncation algorithm that consumes FR3g's skeleton for per-file sizing.

### Out of scope (do not build)
- Any change to commit / rescue / CAS / lock logic (§13.1–§13.5, §18.5). This delta only reshapes the *payload string* handed to the agent.
- Copy detection `-C` (explicitly rejected by FR3e — O(files²), negligible value).
- A per-model context-window registry. FR3d is model-agnostic **by design** — the user sets `token_limit` to their model's window; stagecoach keeps no registry.
- Re-partitioning the legacy per-section caps. They stay byte-for-byte; they're simply bypassed when `token_limit != 0`.

### Modification of completed work (note for implementer)
The new FRs plug into the **already-implemented** diff capture in `internal/git/git.go` (`StagedDiff` ~L642, `TreeDiff` ~L1094, `WorkingTreeDiff` ~L1228 — all three share `StagedDiffOptions` and the `defaultMaxDiffBytes`/`defaultMaxMDLines` capping logic). FR3e/FR3f are flag additions to the existing `g.run(ctx, g.workDir, "diff", "--cached", ...)` call sites; FR3h is a post-capture string filter; FR3g is a numstat capture + prepend. **FR3i is the one net-new algorithm** — it replaces the current "aggregate byte cap" with a budget-aware water-fill **only when `token_limit` is set**; the legacy path is untouched.

---

## 2. Architecture (leveraging prior research)

`plan/006_c23e6f286ae7/architecture/system_context.md` §0 already mapped the diff-capture seams (though that delta focused on the lock). The relevant, re-confirmed seams for this delta:

| Seam | File | Detail |
|---|---|---|
| **Diff capture (3 paths, FR3c parity)** | `internal/git/git.go` | `StagedDiff` (L642), `TreeDiff` (L1094, decompose concept diff), `WorkingTreeDiff` (L1228, decompose T_start snapshot). All three build a `git diff` argv and apply post-capture caps. **All three get `-M`/`-U<diff_context>` flags, index-line stripping, and the skeleton prepend.** The water-fill truncation applies wherever the legacy `defaultMaxDiffBytes` byte-cap currently fires. |
| **`StagedDiffOptions`** | `internal/git/git.go` L36 | The shared options struct threaded through all three diff paths. **Add `TokenLimit int` and `DiffContext int` here** so every path receives them from one place. |
| **Config fields** | `internal/config/config.go` L77–78 (next to `MaxDiffBytes`/`MaxMdLines`) | **Add `TokenLimit int` and `DiffContext int`** in the `[generation]` block. Wire `Defaults()` (L168) + the TOML loaders (`internal/config/file.go` L210–214, `312–316`) + the git-config resolver (`internal/config/git.go` L184–191) for `stagecoach.tokenLimit` / `stagecoach.diffContext`. |
| **Config init template** | `internal/config/bootstrap.go` L282–289 (`generationCommented`) | **Add `token_limit` + `diff_context` to the commented `[generation]` section**, annotate legacy caps as "ignored when token_limit is set (FR3d)". |
| **Where the payload is composed** | `internal/generate/generate.go` L162–216 + `internal/prompt/payload.go` | The diff string from `StagedDiff` flows into `prompt.BuildUserPayload(diff, ...)`. **FR3d's budget is holistic (prompt + examples + diff)**, so the truncation decision needs the prompt/example sizes too — the cleanest seam is to compute the budget inside the git layer using a *measured* prompt+example size passed down (or to truncate the diff against a reserve and let the prompt layer assert it fits). The implementer picks the lower-coupling option; the git layer already owns the diff body so water-fill naturally lives there, with the prompt+example+margin reserve threaded in. |

No new packages, no new third-party dependencies, no new exit codes, no new CLI flags (`token_limit` and `diff_context` are config-file + git-config keys only — neither is a flag per §16; confirm during M2 if a `--token-limit`/`--diff-context` flag is desired, but the PRD specifies them as config knobs).

---

## 3. Phase & milestones

### Phase P1 — Payload / diff optimization (FR3d–FR3i)

A single cohesive phase. The natural split is **always-on optimizations** (small, independent, apply regardless of config) vs **the opt-in token-budget mode** (the algorithmic core that depends on the skeleton).

---

#### Milestone P1.M1 — Always-on diff optimizations (FR3e, FR3f, FR3g, FR3h)

Unconditional improvements applied in every diff path (`StagedDiff` + `TreeDiff` + `WorkingTreeDiff`, FR3c parity). No config-gated behavior — they just make every payload smaller and more complete.

**Task P1.M1.T1 — Add `-M`, `-U<diff_context>`, and `diff_context` config (FR3e, FR3f).**
- Add `DiffContext int` to `StagedDiffOptions` and to `Config` (`[generation]`); default `1` (FR3f), range `0`–`3` (clamp/validate at Load). Resolve through the standard 5-layer precedence including git-config key `stagecoach.diffContext`.
- Pass `-M` (FR3e) on every `git diff` argv in all three paths (deterministic renames; no `-C`).
- Pass `-U<diff_context>` (FR3f) on every `git diff` argv; clamp `diff_context` into `0`–`3` before use.
- **Docs (Mode A, ride with this task):** `internal/config/bootstrap.go` `generationCommented` (add `diff_context`); `docs/configuration.md` File-format example + Built-in defaults table + a one-line note that reduced context is the default. Field comments in `config.go`.

**Task P1.M1.T2 — Index-line stripping + numstat skeleton (FR3h, FR3g).**
- FR3h: strip every `index <oid>..<oid> <mode>` line from captured diffs in all three paths (regex or line-filter on the `diff --git …`-bounded blocks; preserve `diff --git`/`---`/`+++`/`@@`).
- FR3g: prepend `git diff --cached --numstat` (staged path) / the tree-to-tree and working-tree equivalents (decompose paths) — one `added\tdeleted\tpath` line per changed file with rename/status annotation — **before** any diff body. Prefer `--numstat` over `--stat` (no ASCII bars). Ensure a file whose body is fully truncated (under FR3i in M2) still appears in the skeleton.
- **Docs (Mode A):** comment on the new skeleton/stripping helper(s) in `git.go`; note the skeleton in `docs/configuration.md`'s diff-capture prose if a subsection is added (defer the full "token-budget mode" subsection to M2).

**Milestone exit:** all three diff paths emit rename-compact, reduced-context, index-stripped, skeleton-prefixed payloads; `diff_context` config works end-to-end; legacy caps still apply unchanged; existing tests green.

---

#### Milestone P1.M2 — Token-budget mode (FR3d + FR3i)

The opt-in holistic budget. Activated only when `token_limit > 0`; otherwise the M1/legacy path runs byte-for-byte. **Depends on P1.M1.T2** (FR3i sizes off the FR3g numstat skeleton — "one `git` call, dual-use").

**Task P1.M2.T1 — `token_limit` config + mutual-exclusivity (FR3d).**
- Add `TokenLimit int` to `StagedDiffOptions` and to `Config` (`[generation]`); default `0` (unset ⇒ legacy caps). Resolve through the 5-layer precedence including git-config key `stagecoach.tokenLimit`.
- Document + enforce the mutual-exclusivity contract: a non-zero `token_limit` **supersedes** `max_diff_bytes` + `max_md_lines` for that run (the legacy fields are simply not consulted). No hard error — they're ignored.
- **Docs (Mode A):** `internal/config/bootstrap.go` `generationCommented` (add `token_limit` + annotate legacy caps as "ignored when token_limit is set"); `docs/configuration.md` — add `token_limit` to the File-format example + Built-in defaults table, and a **new "Token-budget mode" subsection** (sibling to "Exclusion globs") explaining: holistic cap over prompt+examples+diff, model-agnostic (no registry), mutual-exclusivity with legacy caps, and the water-fill truncation behavior from FR3i. Field comment in `config.go`.

**Task P1.M2.T2 — Dynamic water-fill truncation algorithm (FR3i).**
- Implement the budget allocator: `body_budget = token_limit − skeleton − prompt − margin`. The prompt+example size is measured (≈4 chars/token) and passed into the git layer (see §2 seam note — thread the reserve in via `StagedDiffOptions` or a sibling param; pick the lower-coupling option).
- Token-estimate each changed file's body size **from the FR3g numstat skeleton's per-file line counts** (one git call, dual-use — no extra sizing invocation).
- Common case: `Σ size_i ≤ body_budget` ⇒ include every file whole, stop (no truncation).
- Else: find water level `L` such that `Σ min(size_i, L) = body_budget` (sort sizes ascending, sweep). Files `< L` whole + untouched; files `≥ L` truncated to exactly `L` (first `L` tokens + `... [truncated]` sentinel). Preserve each file's `diff --git`/hunk headers alongside its (possibly truncated) body. Split the aggregate non-markdown diff on `diff --git` boundaries — no extra git calls.
- Guarantee the four FR3i properties: (a) only files that actually exceed `L` are truncated, each minimally; (b) unused share from small files reclaimed; (c) budget fully utilized; (d) no file monopolizes (all capped at `L`), yet large substantive files still get the bulk.
- Unit-test the allocator directly (deterministic size vectors → expected truncation points, reclamation, full-budget-utilization, monotonicity in `L`). E2E: a repo with one huge + several small files under a tight `token_limit` produces a skeleton-complete, water-filled payload.
- **Docs (Mode A):** algorithm comment on the truncation function referencing FR3i; the "Token-budget mode" subsection from P1.M2.T1 covers the user-facing behavior.

**Milestone exit:** `token_limit = 120000` (e.g.) yields a payload that fits the budget via water-fill with the skeleton always complete; `token_limit = 0` is byte-identical to the pre-delta behavior; legacy caps still honored when unset.

---

### Mode B — Sync changeset-level documentation (depends on P1.M1 + P1.M2)

Cross-cutting docs that only make sense once the whole delta is in place. The breakdown agent should turn this into a **final Task** after both milestones.

- **`README.md`** — add a bullet under **Features** for the new payload-optimization capability: model-agnostic `token_limit` holistic budget (no per-model registry), reduced-context / rename-compact / skeleton-prefixed diffs by default. Keep it to one or two lines; this is a capability blurb, not a how-to (the how-to lives in `docs/configuration.md`).
- **`docs/configuration.md`** — confirm the "Token-budget mode" subsection (M2.T1) and the `diff_context`/`token_limit` table rows (M1.T1/M2.T1) are consistent with each other and with `bootstrap.go`; no separate change here beyond what Mode A already added (this task is the consistency sweep).
- **No** changes to the PRD, FUTURE_SPEC, or COMPETITOR-ANALYSIS docs (the PRD already reflects this delta — it is the source of this work).

---

## 4. Acceptance

- `token_limit = 0` (default): payload output is **byte-identical** to the pre-delta behavior (regression guard — keep/extend the existing `StagedDiff` golden tests).
- `token_limit > 0`: total payload (prompt + examples + diff) ≤ `token_limit` (within margin); every changed file appears in the numstat skeleton; the water-fill properties (a)–(d) from FR3i hold under unit test.
- Every diff path (`StagedDiff`, `TreeDiff`, `WorkingTreeDiff`) emits `-M`, `-U<diff_context>`, index-line-stripped, skeleton-prefixed output (FR3c parity).
- `diff_context` honored `0`–`3`; out-of-range clamped/validated at Load with no surprise.
- `config init` template and `docs/configuration.md` carry `token_limit` + `diff_context`; README Features lists the capability.
- All existing unit + E2E tests green.
