# Stagecoach — Delta PRD (v1.0 → v2.0)

**Base revision:** `plan/001_f1f80943ac34/prd_snapshot.md` (v1.0, single-commit core — **fully implemented**, all P1.M1–M5 tasks Complete).
**Target revision:** `PRD.md` (v2.0).
**Diff size:** +530 / −44 lines, +486 net (~33% growth). This is a **large feature addition**, not a tweak — a full-structure delta PRD is warranted.

---

## 1. What changed (size check)

The headline addition is **multi-commit decomposition, promoted from the deferred-v2 roadmap into the core spec**. It composes the already-built snapshot/atomic-commit primitive N times through a four-role agent pipeline. The revision also adds six supporting clusters that decomposition depends on or ships alongside. The v1 single-commit core (§9, §13.1–§13.5) is unchanged in behavior and remains the path taken when something is already staged.

| Cluster | PRD sections | Net effect | Depends on (v1) |
|---|---|---|---|
| **G. Multi-commit decomposition** (headline) | §9.14, §11.4, §13.6, §17.5–§17.7, FR-M1–M12 | New `internal/decompose/`; new public `Decompose()`; new CLI flags; arbiter + chain rebuild | `generate.CommitStaged`, git plumbing, provider executor |
| **A. Manifest schema** (`tooled_flags`, `Render(mode)`, cross-layer field-merge) | §11.5, §12.1, §12.2, FR37a | Modify `manifest.go`, `render.go`, `merge.go`/`registry.go`; fixes a v1 misroute bug | provider package |
| **B. Binary/non-text diff filtering** | §9.1, FR3a–c | New `internal/git/binary.go`; modify `diff.go`; applied in all diff paths | git package, diff capture |
| **C. Per-role provider/model config** | §9.15, §16.4, FR-R1–R5b | Modify `config.go`, role resolution; new `[role.*]` tables | config package |
| **D. Cascading provider priority + tiered/decoupled defaults** | §9.16, FR-D1–D5 | Modify `registry.go`, `builtin.go`; pi default decoupled from z.ai | provider registry |
| **E. Config bootstrap & versioning** | §9.17, FR-B1–B6 | Rework `config init`; add `config upgrade`; `config_version` | config, cmd |
| **F. Antigravity (`agy`) provider** | §12.5.1 | New built-in manifest (experimental); `providers/agy.toml`; PTY-shim research | provider package, D |
| **H. Public API `Decompose()`** | §14.1 | Modify `pkg/stagecoach/stagecoach.go` | generate, decompose |
| **I. CLI wiring** | §15.2, §15.3, §15.5 | New flags; default-action branching; config subcommand changes | all of the above |

The clusters are ordered by dependency in the table; the suggested phasing in §10 follows it.

---

## 2. Scope delta — by cluster

For each cluster: **new vs modified**, the requirements that need implementation tasks, the exact files affected, references to prior research/completed work, and **Mode A documentation** (docs that ride with the implementing work).

### Cluster A — Manifest schema: `tooled_flags`, `Render(mode)`, cross-layer field-merge

**New requirements:**
- **`tooled_flags` field on `Manifest`** (§11.5, §12.1). A second flag-set expressing "tooled but safe" (a git/read/edit allowlist + non-interactive approval) in each provider's idiom. Empty/nil ⇒ provider is bare-roles-only and **cannot serve as the stager** (Cluster G, FR-M5). Field-merges like the other optional slices.
- **`Render(mode)`** (§12.2). `Render` gains a `mode` argument (`"bare"` | `"tooled"`; default `"bare"`); it appends `m.tooled_flags` when `mode=="tooled"`, `m.bare_flags` otherwise. **`mode=="tooled"` with empty `tooled_flags` is an error** ("provider cannot serve as a stager"). The `CmdSpec`/executor downstream are unchanged; only the flag-set selection differs.
- **Bare vs tooled terminology** (§11.5). Document the two invocation modes; bare = planner/message/arbiter, tooled = stager only.

**Modified requirement (bug fix):**
- **FR37a — `[provider.<name>]` field-merges across ALL config layers** (corrects the v1 "key-level whole-block replace"). A repo `[provider.pi]` setting only `default_model` must **not** erase a `default_provider` set in the global file. (The v1 misroute — `glm-5-turbo` routed to `openrouter` instead of `zai` — is the motivating bug.) Field-merge onto the *built-in* manifest remains the registry's separate job. This changes the merge semantics in `internal/config/` (file→file→gitconfig overlay) and/or `internal/provider/registry.go`; the v1 per-field pointer-scalar merge pattern (FINDING 5) is the model.

**Files:** modify `internal/provider/manifest.go` (add `TooledFlags`), `internal/provider/render.go` (add `mode`), `internal/provider/merge.go` / `internal/provider/registry.go` (cross-layer field-merge per FR37a), `internal/config/file.go`/`load.go` (layer overlay for providers). Existing built-in manifests get empty `tooled_flags` until Cluster F/G pins them.

**Mode A docs:** inline Go-doc comments on `Manifest.TooledFlags` and `Render`'s `mode` parameter documenting the bare/tooled contract; a comment in each `providers/*.toml` reference file noting where `tooled_flags` would go (the schema doc).

**Prior research applies:** `go_ecosystem_patterns.md` §5.4 (overlay/merge) and FINDING 5 (pointer scalars for field-presence) — directly reused.

---

### Cluster B — Binary / non-text diff filtering

**New requirements (§9.1, FR3a–c):**
- **FR3a — detect non-text files** via `git diff --cached --numstat` (emits `-\t-\t<path>` for binaries) supplemented by an **extension denylist** (full list in §9.1 FR3a), overridable via a new `binary_extensions` config field. git's numstat is authoritative where it fires; the denylist covers misclassifications.
- **FR3b — emit a one-line placeholder** `<status>\t[binary] <path>` (status from `git diff --cached --name-status`) instead of the useless `Binary files … differ` hunk. Preserves filename + change kind, which the decomposition planner needs for grouping (FR-M3).
- **FR3c — applies in EVERY diff path:** the staged diff (FR1–4), the multi-commit working-tree snapshot (§13.6.2), and the per-concept tree-to-tree concept diff (§13.6.3). Identical placeholder format in all three.

**Files:** new `internal/git/binary.go` (`DetectNonText`, `BinaryPlaceholder`); modify `internal/git/diff.go` (`StagedDiff` integration), plus the new working-tree + tree-to-tree diff paths added in Cluster G. New config field `binary_extensions` in `config.go` (merges with the built-in denylist).

**Mode A docs:** none user-facing beyond config — `binary_extensions` documented in the `config init` template (Cluster E) and §16.2 reference.

**Prior research applies:** FINDING 7 (diff capture/capping is post-capture truncation) — the binary filter is a pre/during-capture step in the same diff pipeline.

---

### Cluster C — Per-role provider/model configuration

**New requirements (§9.15, §16.4, FR-R1–R5b):**
- **FR-R1 — four roles:** planner, stager, message, arbiter (§13.6.2). Each resolves its provider+model independently.
- **FR-R2 — global default:** `[defaults].provider`/`.model` (`--provider`/`--model`, `STAGECOACH_PROVIDER`/`_MODEL`, `stagecoach.provider`/`.model`) is the fallback for any unoverridden role. On the single-commit path the only active role is `message` ⇒ **identical to v1, fully back-compatible.**
- **FR-R3 — per-role overrides:** `[role.<role>].provider`/`.model` config; `STAGECOACH_<ROLE>_PROVIDER`/`_MODEL` env; `--<role>-provider`/`--<role>-model` flags. Precedence (highest wins): flag > env > per-role config > global config > built-in manifest `default_model`.
- **FR-R4 — one model covers all** unless you opt into granularity.
- **FR-R5 — model strings are provider-specific:** a role's `model` is interpreted by *that role's* resolved provider's manifest. Switching a role's provider without updating its model is a surfaced config error.
- **FR-R5b — model requires provider for multi-provider agents** (pi, opencode, agy): `(provider, model)` is a coupled unit; stagecoach always emits `--provider <p>` alongside `--model <m>` for these (or the combined `provider/model` form), never a bare `--model`. Single-backend agents (claude, codex, gemini, cursor) emit only `--model`.

**Files:** modify `internal/config/config.go` (add `Roles map[string]RoleModel` / `map[string]map[string]any` raw, mirroring the existing `Providers` pattern), `internal/config/load.go` (role resolution + precedence), `internal/cmd/root.go` (new flags + env). The per-role `RoleModel` type also surfaces in `pkg/stagecoach.DecomposeOptions` (Cluster H) and `internal/decompose/roles.go` (Cluster G).

**Mode A docs:** Go-doc on the role-resolution function; the `[role.*]` tables and precedence documented in the `config init` template (Cluster E) and §16.4 reference.

**Prior research applies:** the existing config 7-layer precedence machinery (P1.M1.T4) is extended, not replaced.

---

### Cluster D — Cascading provider priority + tiered, decoupled defaults

**New requirements (§9.16, FR-D1–D5):**
- **FR-D1 — cascading provider priority.** The auto-default provider is the highest-priority *installed* built-in, in order: **pi, opencode, cursor, agy, gemini, codex, claude.** Implemented as `Registry.DefaultProvider(installed)` over `preferredBuiltins` (one slice in the registry — trivial to reorder). User-defined providers are never auto-selected.
- **FR-D2 — decoupled from any one subscription.** **pi no longer ships `glm-*`/`zai` as default** (that was the author's personal z.ai subscription). The shipped pi `default_model` is **empty** (was `"glm-5-turbo"`); the z.ai/GLM setup becomes a documented *override*. This is a behavior change to the pi built-in manifest.
- **FR-D3 — universal role→tier strategy:** planner=smart, stager=mid (tooled, needs reliable tool calls — deliberately not the fastest), message=fast (bare, cheapest ideal), arbiter=mid.
- **FR-D4 — per-provider default-model table** (materialized by `config init`, Cluster E). Exemplars current as of 2026-07; the full table is in §9.16 FR-D4.
- **FR-D5 — RESEARCH DIRECTIVE (blocking for defaults).** Model lineups iterate roughly monthly (Sonnet 5 shipped 2026-07-01). The implementing agent **MUST verify, per provider, the current flagship/mid/fast model names** against each provider's live docs/`--help` before pinning any default, and record verified names + date in the manifest source. Defaults must be authored to be trivially refreshable (one table/constant set per provider). The ongoing automated-refresh process is **out of scope** for this delta.

**Files:** modify `internal/provider/registry.go` (`DefaultProvider`, `preferredBuiltins`), `internal/provider/builtin.go` (pi `default_model=""`; tiered tables as constants), `internal/cmd/providers.go` (`providers list` shows the cascading resolved default).

**Modified behavior:** `providers list` resolved-default line; pi manifest `default_model`/`default_provider` defaults.

**Mode A docs:** `providers list` output format; pi personal-override note in `providers/pi.toml` reference (matches §12.3's "NOT the shipped default" framing).

**Prior research applies:** `external_deps.md` (pi default provider is `google`; confirmed flag surfaces for all six v1 agents). **FR-D5 requires fresh research** for current model tokens per provider (and for the new `agy` — Cluster F).

---

### Cluster E — Config bootstrap & versioning

**New requirements (§9.17, FR-B1–B6):**
- **FR-B1 — `config init` writes a POPULATED, working config** (not the v1 inert all-commented template). It runs cascading detection (FR-D1), writes `[defaults] provider=<detected>` and that provider's `[role.*]` per-role default models (FR-D4) **uncommented**; other *installed* providers written as commented-out `[role.*]` blocks. Parent dirs created; existing file not overwritten unless `--force`. Path always printed.
- **FR-B2 — flags:** `config init --provider <name>` targets one; `--force` overwrites; `--template` retains the v1 inert reference behavior.
- **FR-B3 — bootstrap on install + first-run fallback.** Where an install method permits a post-install step (Homebrew `post_install`, curl\|sh, Scoop), run the equivalent of `config init`. First-run fallback: if stagecoach starts with no global config and no `STAGECOACH_CONFIG`, auto-write the bootstrap config once, print a notice with the path, and continue — the tool is never "unconfigured."
- **FR-B4 — config schema version.** Every config file carries `config_version=<int>`; binary knows `CurrentConfigVersion` (compile-time constant). On load: missing/older ⇒ warning + remediation (`config upgrade` / `config init --force`); newer ⇒ "file ahead of binary" warning. **Advisory only, no auto-migration** (no existing users). `config_version` is metadata, **not** a precedence layer (§16.1).
- **FR-B5 — `config upgrade`** rewrites an existing config to `CurrentConfigVersion` in place: preserve user values for keys that still exist, comment out removed/renamed keys with a note. Simple, idempotent.
- **FR-B6 — help de-duplication.** Remove the manual "Subcommands:" block from `config`/`providers` parent `Long` text; cobra's auto-generated "Available Commands" is the single source. (Fixes a v1 redundancy.)

**Files:** modify `internal/cmd/config.go` (rework `init`; add `upgrade`; add `--provider`/`--force`/`--template`), `internal/config/config.go`+`load.go` (`config_version` field + `CurrentConfigVersion` constant + staleness warning), `internal/cmd/root.go` (first-run bootstrap in `PersistentPreRunE`), `.goreleaser.yaml`/install script (post-install hook — coordinate with P1.M5.T3 work), `internal/cmd/config.go`+`providers.go` (FR-B6 dedup).

**Modified behavior:** `config init` (template → populated); `config` command help (dedup).

**Mode A docs:** the **populated `config init` template is itself the primary user-facing config documentation** — it must include explanatory comments for `[defaults]`, `[generation]` (with `binary_extensions`, `max_commits`), `[provider.X]` (with `tooled_flags` note), `[role.*]`, and `config_version`. This rides with the `init` rework.

**Prior research applies:** FINDING 5 (pointer scalars) governs how `config upgrade` detects present-vs-absent keys; go-toml/v2 patterns.

---

### Cluster F — Antigravity CLI (`agy`) provider

**New requirement (§12.5.1):** a built-in manifest for **`agy`**, Google's Gemini-CLI successor (superseded `gemini` on 2026-06-18). Its coding-plan quota is reachable only through `agy` — the same structural reason every provider exists. **Flag surface is assembled from docs + issue tracker, NOT `--help`-verified** ⇒ the heaviest `# TO CONFIRM` load of any built-in ⇒ ships `experimental=true` until a real run clears the items.

**`# TO CONFIRM` (§12.5.1.1) — these gate `agy`'s usability and carry as the manifest's honest caveats:**
1. **CRITICAL/blocking — non-TTY stdout drop (issue [#76](https://github.com/google-antigravity/antigravity-cli/issues/76)):** `agy -p` silently drops stdout when spawned as a subprocess (exactly how stagecoach spawns agents). **Gate:** either a PTY-shim workaround (allocate a PTY for the `agy` child while still capturing bytes) **or** wait for upstream. Gates ALL `agy` roles. This is the single biggest open item.
2. Model flag (`-m` vs `--model`).
3. System-prompt flag (none in gemini-cli lineage → prepend; confirm whether `agy` gained one).
4. **Tooled (stager) flags:** the exact non-interactive, git-scoped, **non-`--dangerously-skip-permissions`** combination. Until known, `tooled_flags=[]` ⇒ `agy` cannot stager (bare roles only, once item 1 clears).
5. `--print-timeout` wiring to stagecoach's `--timeout`.

**Files:** new built-in manifest in `internal/provider/builtin.go` (`agy`, `experimental=true`); new `providers/agy.toml` reference; update Appendix D quick-ref (and the `providers list`/cascading order from Cluster D). A **PTY-shim path in the executor** (`internal/provider/executor.go` + possibly a `creack/pty`-style dep) gated on item 1 — research the exact approach; if a dep is needed it rides with this cluster.

**Mode A docs:** `providers/agy.toml` reference file with the full `# TO CONFIRM` block commented inline (honest, matches §12.7.2 progressive-verification ethos); Appendix D row.

**Prior research applies / extends:** `external_deps.md` does NOT cover `agy` (predates it). FR-D5's research directive (Cluster D) covers current Gemini model tokens for `agy`. The non-TTY stdout bug is **new research** — confirm whether the PTY-shim works under stagecoach's `SysProcAttr.Setpgid`/process-group model (cross-ref `critical_findings.md` and the signal-handling work in P1.M2.T5.S2/P1.M4.T2.S1).

---

### Cluster G — Multi-commit decomposition (the headline)

**New requirements (§9.14, §11.4, §13.6, FR-M1–M12).** This composes the v1 `CommitStaged` primitive N times. New package `internal/decompose/`.

**Activation (FR-M1, §13.6.1):** decomposition activates **iff** nothing is staged AND the working tree has changes. If anything is staged, the single-commit primitive runs **unchanged** — stagecoach never re-partitions a hand-staged index.

**Modes (FR-M2):**
- **Auto-decompose (default)** — planner decides count + partition; if it judges one commit correct, emits the message in the same call (**single-call shortcut**, FR-M11).
- **Forced count `--commits N`** (N≥2) — skip the count decision; planner only partitions into exactly N.
- **Single (escape hatch) `--single`/`--no-decompose`/`--commits 1`** — planner bypassed; v1 behavior (`git add -A` → one `CommitStaged`).

**The four roles (FR-M3/M5/M9, §13.6.2):** planner (bare, JSON), stager (**tooled** — runs git, mutates index, never commits), message (bare — this *is* the v1 §13.1–§13.5 agent, reused), arbiter (bare, JSON). Roles are per-role-configurable (Cluster C). New prompts in §17.5 (planner), §17.6 (stager task), §17.7 (arbiter).

**The pipeline (§13.6.3, FR-M6/M7):**
- Planner → `concepts[0..N-1]` (single-shortcut → done).
- For each concept *i*: `stager[i]` (mutates index, accumulates) → **freeze `tree[i]=write-tree` BEFORE `stager[i+1]` starts** → `message[i]` over the **tree-to-tree** diff `diff(tree[i-1],tree[i])` ‖ `stager[i+1]` (overlapped) → `commit-tree -p newSHA[i-1] tree[i] msg[i]` → `update-ref HEAD newSHA[i] newSHA[i-1]` (serialized CAS).
- Two-index-growth model: **accumulate, never reset**; residue lives only in the working tree.

**Three invariants making `stager[i+1] ∥ message[i]` safe (§13.6.3):** (1) `tree[i]` frozen before `stager[i+1]`; (2) concept diff is tree-to-tree, never index-vs-HEAD (immune to concurrent staging + commits landing); (3) `update-ref`s serialize. **Concept diffs deliberately do NOT reuse v1 `StagedDiff`** (which is index-vs-HEAD) — new `internal/git/tree.go` `TreeDiff`.

**Safety + edge handling (FR-M4/M8/M10/M12, §13.6.5–§13.6.6):**
- **max_commits cap** (default 12) unless `--commits`/`--max-commits` forces higher (FR-M4).
- **Empty-concept skip:** `tree[i]==tree[i-1]` ⇒ skip commit[i], never an empty commit (FR-M8).
- **Stager non-zero** ⇒ retry once, then treat as empty (FR-M8).
- **Arbiter (§13.6.5, FR-M9/M10):** if `git status --porcelain` non-empty after loop, arbiter returns `{"target": <sha>|null}`. **Stagecoach performs ALL git; arbiter only decides.** `null`→new (N+1)-th commit; `target==HEAD`→plumbing amend of tip; `target==earlier commit[i]`→**deterministic linear-chain rebuild** via `read-tree`/`write-tree`/`commit-tree` (NEVER interactive rebase; HEAD only). Ambiguous → `null`. Amend restricted to commits made *this run*.
- **Per-concept failure isolation (FR-M12, §13.6.6):** `message[i]` failure → rescue **for concept i only** (scoped to frozen `tree[i]`); prior commits 0..i-1 stand; remaining staged work left in index. `stager[i+1]` in flight is allowed to complete so its staging isn't lost. CAS failure on commit[i] → abort run with §13.5 message; prior commits stand. Planner fails → error, nothing snapshotted (exit 1, not rescue).

**New CLI flags (§15.2):** `--commits <N>` (0=auto; 1≡`--single`), `--single`/`--no-decompose`, `--max-commits <N>` (default 12); plus the per-role flags from Cluster C.

**New exit-code behavior (§18.2):** planner-unparseable → 1 (pre-snapshot); `max_commits` exceeded → 1; stager-stages-nothing/exits-nonzero-twice → 0 (skip+continue); `message[i]` mid-loop fail → 3 (scoped rescue); arbiter invalid target → 0 (default null).

**Files (new + modify):**
- **NEW `internal/decompose/`** package: `decompose.go` (`Decompose` orchestrator), `roles.go` (per-role resolution, pairs with Cluster C), `planner.go` (planner call + JSON parse/retry), `stager.go` (tooled call, `mode=tooled`; snapshot/overlap scheduling), `arbiter.go` (arbiter call + amend/new/rebuild), `chain.go` (linear-chain rebuild for mid-chain amend — FR-M10).
- **NEW prompts** `internal/prompt/planner.go`, `stager.go`, `arbiter.go` (§17.5–§17.7; planner/arbiter use **JSON contracts** — unlike free-form messages per §17.4 — with robust parse + one retry).
- **NEW git ops** `internal/git/tree.go`: `RevParseTree` (`HEAD^{tree}`/empty-tree base), `TreeDiff` (tree-to-tree concept diff), `ReadTree`, `StatusPorcelain`. Coordinate with Cluster B (binary placeholders in tree-diff).
- **NEW stub roles** for integration tests: stub planner (canned JSON), stub stager (scripted `git add` of named paths — **no real tooled agent in CI**), stub arbiter (canned target/null).
- Modify `internal/generate/rescue.go` (scoped per-concept rescue variant — §18.3 multi-commit variant), `internal/git/stage.go` (`StagedFileCount` may already exist; confirm), `internal/cmd/default_action.go` (branch: `HasStagedChanges`||`--single` → `GenerateCommit`; else → `Decompose`).

**Mode A docs:** Go-doc on `Decompose`/`DecomposeOptions`/`DecomposeResult`/`RoleModel` (Cluster H); the §17.5–§17.7 prompt constants are internal reference material (PRD is the source of truth, verbatim-as-canonical like §17.1).

**Prior research applies (heavily):** `git_plumbing_reference.md` §2/§3/§7 (`commit-tree`/`update-ref` CAS/`diff-tree`) are the publication primitives; `git_plumbing_summary.md` atomic sequence is the per-concept unit. **The mid-chain rebuild (FR-M10) needs the exact `read-tree`/`write-tree`/`commit-tree` sequence finalized during implementation** — Appendix E item 9 calls this out; it is the one piece the PRD leaves to implementation planning. The v1 property/invariant test harness (P1.M5.T1.S1) is the model for the new invariants below.

**New property/invariant tests (§20.2, ride with this cluster):** *concept isolation* (each commit's `diff-tree` = exactly its concept's files, no leakage); *loop index cleanliness* (after a clean run `status --porcelain` empty, or arbiter reconciled everything); *mid-chain amend fidelity* (rebuilt chain's non-target commits byte-identical; only target's tree grew by the leftover set). Plus the decompose stub suite: auto-decompose into N, `--commits N`, single-shortcut, empty-concept skip, mid-loop rescue, arbiter new/tip-amend/mid-chain-rebuild, binary-placeholder propagation, and `stager[i+1] ∥ message[i]` overlap interleaving checks.

---

### Cluster H — Public API: `Decompose()`

**New (§14.1):** add to `pkg/stagecoach/stagecoach.go`:
- `type DecomposeOptions struct { Options; Count int; Single bool; MaxCommits int; Planner, Stager, Arbiter RoleModel }`
- `type RoleModel struct { Provider, Model string }`
- `type DecomposeResult struct { Commits []Result; Amended int; Provider string }`
- `func Decompose(ctx, opts DecomposeOptions) (DecomposeResult, error)` — NO-OP (delegates to `GenerateCommit`) when `Single` or `Count==1`. Caller must ensure nothing staged (CLI gates on `HasStagedChanges`). Mark `// Stable as of v2.0`; keep structs additive-only (Appendix E item 6 stance, extended to v2).

`Options` (existing) docstring updated: it's the per-concept primitive inside `Decompose` for the message role.

**Mode A docs:** Go-doc comments on `Decompose`/`DecomposeOptions`/`DecomposeResult`/`RoleModel` documenting the contract, the no-op-on-single semantics, the caller-must-stage-nothing precondition, and the v2.0 stability note.

---

### Cluster I — CLI wiring

**New flags (§15.2, all ride with their clusters):** `--commits`/`--max-commits`/`--single`/`--no-decompose` (G); `--planner-*`/`--stager-*`/`--arbiter-provider`/`--arbiter-model` + env (C). `--provider`/`--model` help text updated to "global default for all roles."

**Default-action branching (§14.1, §15.1):** `main.go`/`default_action.go`: parse flags → if something staged OR `--single`/`--commits 1` → `GenerateCommit`; else → `Decompose`. The auto-stage-all "nothing staged → one commit" v1 behavior survives only as the `--single` escape hatch (and as the default when something is staged).

**`config` subcommand changes (§15.3):** `config init` reworked (E), `config upgrade` added (E). `providers list` resolved-default line updated (D).

**Mode A docs:** the new global flags + env documented in CLI help text (cobra auto-gen) and must match PRD §15.2/FR-R3. Exit-code additions documented in `--help` (§15.4).

---

## 3. Modified / removed requirements (awareness — no new tasks unless noted)

- **`FR38` (config init):** changed from "commented template" to "populated working config" — **reworked in Cluster E** (this *is* a new task; listed here because it modifies an existing FR rather than adding one).
- **pi manifest defaults:** `default_model` `glm-5-turbo`→`""`, `default_provider` comment updated (Cluster D / FR-D2).
- **`providers list` resolved-default:** now shows cascading priority order (Cluster D).
- **Removed:** the old §10.3 "v2 deferred" framing and N1's "v1 always one commit" — both superseded by decomposition being in-scope. No tasks; awareness only.
- **`v1.0 → v2.0` status bump + "this revision" metadata header line:** doc-only, no tasks.

---

## 4. Leveraged prior research & completed work

**Completed v1 work that is REUSED (not re-implemented):**
- `internal/generate/generate.go` `CommitStaged` — the per-concept primitive; the message role literally *is* this. Do not duplicate; `decompose` composes it.
- `internal/git` plumbing (`write-tree`/`commit-tree`/`update-ref` CAS/`diff-tree`/`rev-parse HEAD`/`diff --cached --quiet`) — all P1.M1.T2 work; reused for publication. New ops added in Cluster G's `tree.go`.
- `internal/provider` (manifest/registry/render/executor/parse/merge) — extended (Clusters A/D/F), not replaced.
- `internal/config` precedence — extended for roles + versioning (Clusters C/E).
- v1 stub provider + property/invariant test harness (P1.M5.T1) — the model for the decompose stub suite + new invariants.
- `internal/signal` + `internal/provider/procgroup_*` — the stager (tooled agent) and its process-group killing reuse this.

**Prior research files in `plan/001_f1f80943ac34/architecture/` that STILL APPLY** (read first, do not re-research):
- `git_plumbing_reference.md` — `commit-tree`/`update-ref`/`diff-tree`; the atomic sequence. **The mid-chain rebuild plumbing (FR-M10) is the one gap to finalize** (Appendix E item 9).
- `git_plumbing_summary.md` — exit-code cheat sheet, Go patterns.
- `critical_findings.md` — FINDINGS 1–7 (unborn repo trap, `commit-tree -F -`, CAS-failure detection, go-toml pointer-scalars for field-merge, `--cached --quiet` inverted exits, diff-capping). All reused.
- `go_ecosystem_patterns.md` §5.4 overlay/merge, §1 cobra setup, §4 signals.
- `external_deps.md` — verified flag surfaces for the six v1 agents.

**Genuinely NEW research needed (not in prior sessions):**
1. **FR-D5 (Cluster D):** current flagship/mid/fast model tokens per provider (pi, opencode, cursor, agy, gemini, codex, claude) + verification date. Record in manifest source.
2. **`agy` (Cluster F):** live `--help` capture; the five `# TO CONFIRM` items (esp. the non-TTY stdout PTY-shim feasibility under stagecoach's process-group model). 
3. **pi OpenAI routing (Appendix E item 12):** which pi sub-provider routes to an OpenAI model, so pi's default + `default_provider` wire end-to-end.

---

## 5. Documentation impact

### Mode A — docs that ride WITH the implementing work
| Cluster | Doc artifact | Where |
|---|---|---|
| A | Go-doc on `Manifest.TooledFlags` + `Render(mode)`; schema comment in `providers/*.toml` | source + reference toml |
| B | `binary_extensions` in `config init` template + §16.2 ref | Cluster E template |
| C | Go-doc on role resolution; `[role.*]` + precedence in `config init` template | Cluster E template |
| D | `providers list` output; pi personal-override note in `providers/pi.toml` | cmd + reference toml |
| E | **The populated `config init` template itself** (primary config doc: `[defaults]`, `[generation]` w/ `binary_extensions`+`max_commits`, `[provider.X]` w/ `tooled_flags` note, `[role.*]`, `config_version`) | `config init` output |
| F | `providers/agy.toml` with full `# TO CONFIRM` block; Appendix D row | reference toml |
| G | Go-doc on `Decompose`/`DecomposeOptions`/`DecomposeResult`/`RoleModel` | `pkg/stagecoach` |
| H | (same as G) | `pkg/stagecoach` |
| I | New global flags + env + exit codes in cobra `--help` | CLI help text |

### Mode B — changeset-level documentation (final task, depends on all clusters)
`README.md` is materially stale after this changeset and must be updated as a final, cross-cutting doc sweep:
- **Headline:** multi-commit decomposition as a workflow property (the §5 "three workflow properties" framing — was "two"); a "stage-while-it-thinks across a loop" payoff blurb.
- **New flags:** `--commits N`, `--single`/`--no-decompose`, `--max-commits`, per-role flags; the `--commits 3` / `--planner-model` example invocations (§15.5).
- **`agy` provider** in the providers list + cascading-default note.
- **Cascading default + decoupled-from-z.ai** posture (so users aren't surprised pi's default changed).
- **`config init` populated-config** behavior + `config upgrade` in the quick-start/config sections.
- The competitive-comparison table row (multi-commit now "Yes") and the "right model for the right job" value prop (§5 #6) in positioning.

This is a **final Mode B "Sync changeset-level documentation" requirement** depending on all clusters; the breakdown agent should turn it into a terminal task (mirrors P1.M5.T4/T5). The PRD itself (`docs/PRD.md`) stays read-only.

---

## 6. Open questions / research directives carried into implementation

(Appendix E items 7–13, restated for the breakdown agent's task scoping — each is a research sub-step inside its cluster, not a blocker on starting:)
- #7 `agy` non-TTY stdout (PTY-shim vs upstream #76) — **gates all agy roles** (Cluster F).
- #8 `agy` tooled (stager) flags — **gates agy as stager** (Cluster F/G).
- #9 mid-chain amend plumbing exact sequence — **finalize + prove via the §20.2 fidelity invariant** (Cluster G `chain.go`).
- #10 stager toolset scope per provider — pin minimal allowlist (git add/read/edit/apply) (Cluster A/F).
- #11 verify current model names per provider (FR-D5, **blocking for defaults**) (Cluster D).
- #12 pi OpenAI routing — wire pi default + `default_provider` end-to-end (Cluster D).
- #13 `config upgrade` mechanics — keep simple until a real rename occurs (Cluster E).

---

## 7. Suggested phasing for the breakdown agent

Dependency-ordered; the headline (G) is gated on A, B, C. Each cluster maps to ~1 milestone; G to 2–3.

1. **Phase 1 — Foundations:** Cluster A (manifest schema) + Cluster B (binary filtering). Pure additive/bug-fix; unblocks everything. (A also ships FR37a, the v1 misroute fix.)
2. **Phase 2 — Configuration & defaults:** Cluster C (per-role) + Cluster D (cascading/tiered/decoupled, incl. FR-D5 research) + Cluster E (bootstrap/versioning). These are interdependent (E materializes C+D; D's research feeds E).
3. **Phase 3 — `agy` provider:** Cluster F (can overlap Phase 2; research-heavy, experimental-gated).
4. **Phase 4 — Decomposition core:** Cluster G (decompose package, prompts, git tree ops, pipeline, arbiter/chain-rebuild, per-concept rescue, stub suite + new invariants). The big one — likely 2–3 milestones (roles/planner/stager/arbiter, then arbiter-rebuild + failure-isolation + concurrency invariants).
5. **Phase 5 — Public API & CLI wiring:** Cluster H (`Decompose()` public) + Cluster I (flags, default-action branching, config subcommands). Wires Phase 4 to the user.
6. **Phase 6 — Mode B docs:** the final README + cross-cutting doc sweep (§5 Mode B), depending on all above.

**Sizing guidance:** G is the bulk of the work; A–F are each small-to-medium; H/I are small. Keep subtask granularity tight — the prior session's per-subtask `context_scope` CONTRACT DEFINITION style is the template. Do not re-research git plumbing / go-toml / cobra / signals — they are done (§4).
