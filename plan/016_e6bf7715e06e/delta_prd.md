# Delta PRD — v2.9: Chrome-disable for every provider

| Field              | Value                                                                                                                                                                                                                       |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **From**           | v2.8 (`plan/015_b461e4720495/prd_snapshot.md`)                                                                                                                                                                              |
| **To**             | v2.9 (`plan/016_e6bf7715e06e/prd_snapshot.md`)                                                                                                                                                                              |
| **Diff size**      | +21 / −8 lines in the PRD. Of those, 13 changed lines are the cosmetic `dustin` → `dabstractor` author rename (**already implemented in code** — see §3). The substantive delta is **~9 added lines**: 1 revision entry, 1 goal (G25), 1 new section §9.28 (FR-C1–C5), 1 bullet added to §12.7.1. |
| **Priority**       | P2 (→ G25)                                                                                                                                                                                                                  |
| **Sizing verdict** | **SMALL.** A verification + documentation discipline layered on already-shipped provider manifests. The PRD itself states: *"No commit/CAS/rescue/lock changes; mutation safety unchanged (the read-only constraint stays)."* One phase, two milestones, three tasks. |

---

## 1. Motivation

§12.7.1's tools-disable asymmetry guarantees **mutation** safety (pi/claude turn tools off with explicit switches; codex/cursor/opencode/agy/qwen-code pin a read-only, never-ask profile). That is sufficient for the §18.1 repo-integrity invariant, but it says nothing about **agent chrome** — skills, extensions/prompt-templates, AGENTS.md/CLAUDE.md-style context files, and MCP servers — which a provider may still discover, load, spawn, and inject around the call even when it cannot mutate the repo.

On a one-shot commit-message call this chrome is pure overhead: MCP servers spawn as subprocesses (startup latency, failure modes); skill/context injection consumes prompt tokens and quota the user did not intend to spend; a misbehaving MCP server or skill can skew the run nondeterministically. v2.9 makes every built-in provider render **chrome-less** for every surface the agent CLI exposes a switch for, and treats any surface it cannot switch off as a **documented, tracked limitation** rather than a hidden assumption.

This is a **completeness + honesty** pass, not new behavior: pi and claude already disable most chrome; the read-only-constrained providers mostly have no chrome switches to set. The work is (a) verify each provider's `--help` for chrome switches, (b) set any that exist and aren't set, (c) record a per-provider **CHROME-DISABLE** note, and (d) extend the docs to present chrome as a separate safety axis.

---

## 2. What changed (precise diff)

### New requirements — §9.28 Chrome-disable for every provider (FR-C1–C5, → G25)

| FR     | Requirement (summary)                                                                                                                                                                                                                                                                                                                                  | Code impact                                                                                                                                                                                                                                                                                       |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| FR-C1  | Defines **chrome** = {skills, extensions/prompt-templates, context files, MCP servers}. A call is "chrome-less" when none are loaded/connected. Excludes the model, stagecoach's system prompt, and the diff payload.                                                                                                                                  | None (definition).                                                                                                                                                                                                                                                                                |
| FR-C2  | For each built-in provider, `bare_flags`/`env` MUST include the agent's literal disable flag for **every chrome surface the agent CLI exposes a switch for** — verified end-to-end per provider, never assumed. pi sets the bar; claude covers it via `--tools ""`/`--setting-sources ""`. Any exposed-but-unset switch is a defect to fix.             | **Per-provider audit + possible flag additions.** Most already set; verification may surface a small number of flags to add for the read-only-constrained providers.                                                                                                                              |
| FR-C3  | **MCP** specifically: where a provider offers a real MCP-disable flag, set it. pi today has NO `--no-mcp` (only `--mcp-config <path>`) — `--no-tools` suppresses MCP tool *use* but servers may still connect at startup. That gap is **documented**, not assumed.                                                                                     | Document the pi MCP-server-discovery gap in the CHROME-DISABLE note. claude's `--tools ""`/`--setting-sources ""` already cover MCP.                                                                                                                                                              |
| FR-C4  | Read-only-constrained providers (codex/cursor/opencode/agy/qwen-code): disable the chrome they DO expose a switch for (per FR-C2); the read-only constraint **stays** as the mutation guarantee (it is NOT a chrome substitute); where no chrome switch exists, record the limitation in the manifest comment **and `docs/providers.md`** — never concealed. | Verification + documentation. No invented flags (FR-C4c is explicit about this).                                                                                                                                                                                                                 |
| FR-C5  | **Verification + tracking duty.** Each built-in provider's manifest header gains a **CHROME-DISABLE** note (alongside the existing `TOOLS-DISABLE CATEGORY` note) recording, per surface, what is disabled (by which flag), what is not (no switch), and the verification date/source — same discipline as FR-D5/FR-T9. The §12.7.1 asymmetry table (in `docs/providers.md`) is extended with a **"Chrome-disable"** column. | **Manifest comment additions** in `internal/provider/builtin.go` (7 providers) + **`docs/providers.md`** table column.                                                                                                                                                                           |

### Modified requirement — §12.7.1 (Tools-disable asymmetry)

Extended with a 4th consequence bullet: *"Chrome is a separate axis — see §9.28 (v2.9). Mutation safety says nothing about agent chrome… §9.28 (FR-C1–C5) requires every provider to disable each chrome surface the agent exposes a switch for and to document any surface it cannot."* This is a **conceptual/doc extension** of the asymmetry section — no change to the tools-disable mechanism itself.

### Removed requirements

None.

---

## 3. Already implemented — NO TASKS (author rename)

The diff also renames the author `dustin` → `dabstractor` in PRD metadata (Author field, §7.1 persona) and §21 install paths (Homebrew tap, `go install`, GitHub URL, Scoop bucket). **This is already fully implemented in the codebase** — verification:

- `go.mod`: `module github.com/dabstractor/stagecoach` ✓
- All Go imports use `github.com/dabstractor/stagecoach/...` ✓
- `.goreleaser.yaml`: owner/homepage/tap/scoop all `dabstractor` (with an explicit owner-override comment) ✓
- `git remote origin` → `dabstractor/stagecoach` ✓
- `docs/README.md` install commands already `dabstractor` ✓

The only remaining `dustin` references are **build artifacts** (`bin/`, `dist/config.yaml`, `coverage.out`) and local agent-session logs (`.pi-subagents/artifacts/`), all of which regenerate and are not source. **The breakdown agent must NOT create tasks for the rename** — it is a PRD-metadata catch-up only.

---

## 4. Current codebase state (leveraged — do NOT re-derive)

The provider manifests in `internal/provider/builtin.go` already carry substantial chrome-disable coverage. The verification pass confirms and documents this; it is not greenfield.

| Provider      | Current `bare_flags` (chrome-relevant)                                                                                                                | Chrome status today                                                                                                                                                                                          |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **pi**        | `--no-tools --no-extensions --no-skills --no-prompt-templates --no-context-files --no-session`                                                        | **Extensions, skills, prompt-templates, context-files ALREADY disabled.** MCP gap: pi has no `--no-mcp` (only `--mcp-config`); `--no-tools` suppresses MCP tool *use* but servers may still connect. → **Document (FR-C3).** |
| **claude**    | `--tools "" --setting-sources "" --no-session-persistence`                                                                                            | **Chrome covered**: `--tools ""` disables all tools (MCP surfaces as tools); `--setting-sources ""` blocks the settings files where MCP/skills/extensions are configured. → **Confirm + document (FR-C2/C3).** |
| **agy**       | `--mode plan`                                                                                                                                         | Read-only constraint; no chrome switches set. agy v1.1.0 diverged from gemini-cli. → **Verify `--help` for any chrome switches; else document limitation (FR-C4).**                                          |
| **qwen-code** | `--approval-mode default` (`# TO CONFIRM`)                                                                                                            | Gemini-cli fork; experimental. → **Verify `--help`; document (FR-C4).**                                                                                                                                      |
| **opencode**  | `[]` (empty — `run` is inherently read-only/non-interactive)                                                                                          | No chrome switches set. → **Verify `--help`/`run --help`; document (FR-C4).**                                                                                                                                |
| **codex**     | `--sandbox read-only --ephemeral`                                                                                                                     | Read-only constraint; no chrome switches set. → **Verify `exec --help`; document (FR-C4).**                                                                                                                  |
| **cursor**    | `--mode ask --trust`                                                                                                                                  | Read-only constraint; no chrome switches set. → **Verify `agent --help`; document (FR-C4).**                                                                                                                 |

**Verification input (already researched — do not re-research from scratch):** the per-provider `--help` findings live in `plan/001_f1f80943ac34/architecture/external_deps.md` (the richest catalog) and subsequent sessions (`plan/010_*, plan/012_*, plan/014_*`). Those catalogs predate the chrome focus (they record tool-disable flags, not chrome switches specifically), so the verification genuinely re-reads them **for chrome surfaces** — but the `--help` output is already captured; no new agent invocations are required to author the notes.

**Where the CHROME-DISABLE note goes:** the existing per-provider doc-comments in `internal/provider/builtin.go` (e.g., the block above `builtinPi()`, `builtinClaude()`, …). Each already carries a `TOOLS-DISABLE CATEGORY`-style discussion; FR-C5 adds a parallel **CHROME-DISABLE** paragraph.

**Where the asymmetry table lives:** `docs/providers.md` → "## The 7 built-in providers" (the 7-row table) and "## Tools-disable asymmetry" (the two-bullet section). FR-C5 adds a "Chrome-disable" column to the table; the §12.7.1 extension adds the "chrome is a separate axis" bullet to the asymmetry section.

---

## 5. Phase 1 — Chrome-disable for every provider (FR-C1–C5, → G25)

### Functional requirement (authoritative text: PRD §9.28)

Every built-in provider renders **chrome-less** (skills, extensions/prompt-templates, context files, MCP servers) for every surface the agent CLI exposes a disable switch for; any surface a provider cannot switch off is a documented, tracked limitation with a re-check date — never a hidden assumption. Mutation safety (the read-only constraint / tool-disable switches) is unchanged and is NOT treated as a chrome substitute.

### Milestone M1 — Manifest verification, flag-setting, CHROME-DISABLE notes, and tests

#### Task M1.T1 — Per-provider chrome audit + flag-setting + CHROME-DISABLE manifest notes (FR-C1, FR-C2, FR-C3, FR-C4, FR-C5)

For each of the 7 built-in providers in `internal/provider/builtin.go`:

1. **Verify** (against the captured `--help` findings in `plan/001_*/architecture/external_deps.md` and successors, plus any live `--help` already referenced in the manifest header) which chrome surfaces {skills, extensions/prompt-templates, context files, MCP} the agent CLI exposes a disable switch for.
2. **Set** any exposed-but-currently-unset chrome-disable flag in `bare_flags` (FR-C2). Expected to be a small number — pi and claude are already complete; the read-only-constrained providers mostly expose no per-surface switches. **Do NOT invent flags** (FR-C4c): where no switch exists, the limitation is recorded, not fabricated.
3. **Document the pi MCP gap** (FR-C3): pi has no `--no-mcp`; `--no-tools` suppresses MCP tool *use* but configured servers may still connect at startup. This goes in pi's CHROME-DISABLE note as a tracked limitation, not an assumption.
4. **Add a CHROME-DISABLE note** to each provider's doc-comment block (FR-C5), recording per surface: what is disabled (by which flag), what is not (no switch available), and the verification date/source — the same discipline as the existing FR-D5/FR-T9 verification notes already in those headers.

- **Mode A docs (ride with the work):** the CHROME-DISABLE note IS the per-provider documentation artifact; it lives in `internal/provider/builtin.go` alongside the code it describes.

#### Task M1.T2 — Tests asserting the chrome-disable contract (FR-C2, FR-C5)

In `internal/provider/builtin_test.go`, add focused assertions that pin the chrome-disable contract where it is testable from the manifest struct (no real agent needed):

- **pi**: `BareFlags` contains `--no-extensions`, `--no-skills`, `--no-prompt-templates`, `--no-context-files` (the four chrome surfaces pi exposes switches for — already present; this guards against regression).
- **claude**: `BareFlags` contains `--setting-sources` (whose `""` value blocks the settings files where MCP/skills/extensions are configured) and `--tools` (whose `""` value disables all tools incl. MCP).
- A consistency check: for each provider, the chrome surfaces its CHROME-DISABLE note claims are disabled by a flag must actually appear in `bare_flags`/`env` (parse the note's claimed flags and assert presence — a lightweight guard that the note and the flags don't drift).
- Read-only-constrained providers: assert the existing read-only constraint flag is present (codex `--sandbox`/`read-only`; cursor `--mode`/`ask`; agy `--mode`/`plan`; qwen-code `--approval-mode`; opencode empty-by-design) — i.e., the mutation-safety guarantee that FR-C4(b) says "stays" is in fact present.

These are cheap table-driven tests; they do not invoke any agent.

### Milestone M2 — Sync changeset-level documentation (Mode B)

#### Task M2.T1 — Extend `docs/providers.md` + cross-cutting overview for chrome-disable (FR-C4c, FR-C5, §12.7.1 extension)

This is the Mode B changeset-level documentation task; it depends on M1 (it records the verified manifest state).

1. **`docs/providers.md` — "## The 7 built-in providers" table**: add a **"Chrome-disable"** column (FR-C5) recording, per provider, the chrome outcome in one short phrase (e.g., pi: "extensions/skills/templates/context off; MCP use suppressed (servers may connect — tracked)"; claude: "via `--tools ""`/`--setting-sources ""`"; codex/cursor/agy/qwen-code/opencode: "no per-surface switch; read-only constraint only — documented limitation").
2. **`docs/providers.md` — "## Tools-disable asymmetry" section**: add the **"Chrome is a separate axis"** bullet (the §12.7.1 v2.9 extension) explaining that mutation safety ≠ chrome safety, and pointing at the per-provider Chrome-disable column + the CHROME-DISABLE manifest notes. Reframe the existing two-bullet asymmetry as the *mutation* axis and add chrome as a parallel *isolation/determinism/cost* axis.
3. **`docs/README.md` — Documentation index / capability index**: add a one-line entry under the providers row (or the capability index) noting chrome-disable as a v2.9 safety refinement, linking to the providers.md asymmetry section.
4. **`docs/how-it-works.md`** (line ~197, the "No provider mutates the repository" safety paragraph): extend with one sentence noting that every provider also renders chrome-less where the agent CLI exposes a switch (skills/extensions/context/MCP), with un-switchable surfaces documented as limitations — so the safety story covers both mutation and chrome.
5. **Top-level `README.md`**: if it carries a safety/bare-mode bullet, add a concise mention that commit-message calls are chrome-less where the agent allows it. Keep it brief — this is a refinement of existing "bare mode" messaging, not a new feature pitch.

---

## 6. Documentation impact (summary)

| Mode | Artifact                                                                                       | Owner task |
| ---- | ---------------------------------------------------------------------------------------------- | ---------- |
| A    | CHROME-DISABLE notes in `internal/provider/builtin.go` per-provider headers                    | M1.T1      |
| A    | `internal/provider/builtin_test.go` chrome-contract assertions                                 | M1.T2      |
| B    | `docs/providers.md` (Chrome-disable column + asymmetry section), `docs/README.md`, `docs/how-it-works.md`, top-level `README.md` | M2.T1      |

No new CLI flags, config keys, or user-facing surface are introduced by v2.9 — `docs/cli.md` and `docs/configuration.md` are **not** affected.

---

## 7. What is NOT changing

- **No commit / CAS / rescue / lock / index behavior changes** (PRD v2.9 revision statement).
- **No mutation-safety change** — the read-only constraint / tool-disable switches stay exactly as-is; chrome-disable is an additional axis, not a replacement.
- **No manifest schema change** — no new manifest field. Chrome-disable is expressed via the existing `bare_flags`/`env` fields plus a doc-comment note. (Contrast with FR-R6 reasoning, which added a `reasoning_levels` table; v2.9 adds nothing structural.)
- **No new CLI flags, env vars, config keys, or git-config keys.**
- **The author rename (`dustin` → `dabstractor`) is already implemented** (§3) — no tasks.
- **No removal of the existing `experimental` flags** on agy/qwen-code — chrome-disable is orthogonal to the §12.5.1.1 / §12.5.2 verification status.

---

## 8. Risk

| Risk                                                                                                | Likelihood | Impact | Mitigation                                                                                                                                                                                                                       |
| --------------------------------------------------------------------------------------------------- | ---------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A read-only-constrained provider silently loads chrome (MCP server, AGENTS.md) we cannot switch off | Medium     | Low    | This is the *status quo*; v2.9 makes it **documented** (FR-C4c) rather than hidden. The call stays read-only/never-mutate, so the §18.1 invariant holds regardless.                                                              |
| Verification invents a flag that doesn't exist (fabricated chrome-disable)                           | Low        | Low    | FR-C4c is explicit: document, don't invent. M1.T2's note/flag consistency test catches drift.                                                                                                                                   |
| CHROME-DISABLE note drifts from the actual `bare_flags`                                             | Low        | Low    | M1.T2 asserts the flags the note claims are present in `bare_flags`/`env`.                                                                                                                                                      |
| Over-scoping: treating this as a large refactor                                                     | —          | —      | This is a P2 verification+docs pass. The PRD states no core-behavior change. The plan above is deliberately 1 phase / 2 milestones / 3 tasks.                                                                                    |

---

## 9. References

- **PRD (authoritative):** §9.28 (FR-C1–C5), §12.7.1 (4th consequence bullet), G25. `plan/016_e6bf7715e06e/prd_snapshot.md`.
- **Prior research (verification input):** `plan/001_f1f80943ac34/architecture/external_deps.md` (richest per-provider `--help` catalog), plus `plan/010_*, plan/012_*, plan/014_*/architecture/external_deps.md` for later re-verifications.
- **Code surface:** `internal/provider/builtin.go` (7 `builtin*` funcs + doc-comments), `internal/provider/builtin_test.go`.
- **Doc surface:** `docs/providers.md` (§"The 7 built-in providers", §"Tools-disable asymmetry"), `docs/README.md`, `docs/how-it-works.md`, top-level `README.md`.
