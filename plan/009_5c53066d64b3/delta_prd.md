# Delta PRD — Multi-turn generation fallback (v2.3 → G21, FR-T1–T12)

## Diff analysis & sizing

**Document diff (PRD.md vs `plan/008_82253c999440/prd_snapshot.md`):** exactly **one new line** — the v2.3 revision-history row. The entire `§9.24` spec body and `FR-T1`–`FR-T12` are byte-identical in both documents (they were forward-specced into the PRD before the prior snapshot was taken). `FUTURE_SPEC.md` is **already** updated (the lossless-multi-turn-graduation note is present); only its lossy map-reduce form remains rejected.

**Implementation reality (what makes this a real delta, not a 1-line bookkeeping task):** the multi-turn feature is **entirely unimplemented**. Verified by grep — no `SessionMode`/`session_mode`, `MultiTurn`/`multi_turn`, or `--session-id` symbols exist anywhere in `internal/`, `cmd/`, or `pkg/`. Specifically:
- `internal/provider/manifest.go` `Manifest` struct has **no** `session_mode` field.
- `internal/provider/builtin.go` pi manifest has **no** `SessionMode: "append"` (and its `BareFlags` still carries `--no-session` with no multi-turn render variant).
- `internal/config/config.go` + `file.go` have **no** `MultiTurnFallback` / `MultiTurnChunkTokens` fields, defaults, TOML tags, overlay, or merge.
- `internal/generate/generate.go` `CommitStaged` retry loop (line 226) exhausts straight into `&RescueError{…}` (line 288) — **no fallback branch** between one-shot-exhausted and rescue.
- `docs/configuration.md`, `docs/how-it-works.md`, `README.md` do **not** mention multi-turn.

**Prior session (008)** implemented v2.2 arbiter-freeze-parity (FR-M1d) + planner files (FR-M3/M3b/M4) — **unrelated to this feature**. `plan/008_82253c999440/architecture/` holds no reusable research for multi-turn (it is a genuinely new topic; the `§9.24` spec is self-contained and cites its empirical evidence inline — 169K-token one-shot failure vs 266K-token-across-5-turns success — so **no web research is needed**).

**Sizing verdict: medium feature.** New generation path + one manifest field + two config knobs + a render variant + tests + docs — but **message-role only**, reusing the existing parse (§9.6) / dedupe (§9.7) / rescue (§9.10) / CAS / lock unchanged (`§9.24` FR-T7/T10). One phase, one milestone, four tasks + a Mode B doc sync. **Not** a large/multi-phase effort.

---

## Scope delta

The authoritative requirement is **PRD §9.24, FR-T1–FR-T12** (unchanged from the prior snapshot — reference it by number; do not re-spec). This delta PRD scopes its **implementation**. The revision-history row already in PRD.md is the governance signal that this feature is now in scope; no further PRD.md edit is required.

**In scope (message role, single-commit path §13.1–§13.5 only):**
- New manifest field `session_mode` (`""` default | `"append"`); pi ships `"append"` (verified per FR-T9).
- New config keys `multi_turn_fallback` (bool, default `true`) and `multi_turn_chunk_tokens` (int, default `32000`).
- A lossless multi-turn fallback path: capture-once → chunk → N+1 append-session turns → final-turn stdout through the **existing** parse + dedupe pipeline.
- Trigger gate inserted between one-shot-retry-exhaustion and rescue (FR-T1 a–d).
- Per-turn timeout + total-budget progress line (FR-T5); `--verbose` surface (FR-T11).
- Best-effort failure → existing rescue unchanged (FR-T7).

**Out of scope (explicit, per FR-T10 / the revision row):** planner, stager, and arbiter roles; decompose path (§13.6); any change to commit / rescue-message / CAS / run-lock / signal-handling logic; lossy map-reduce chunking (permanently rejected). `token_limit` (FR3d) continues to govern **only** the one-shot path (FR-T12).

**Removed requirements:** none.

---

## Requirements (implementation work, grouped for the breakdown agent)

### R1 — Provider surface: `session_mode` manifest field + pi value + multi-turn render variant (FR-T6, FR-T8, FR-T9)
- Add `SessionMode` to the `Manifest` struct (`internal/provider/manifest.go`, alongside the `// --- retry ---` / output groups; TOML tag `session_mode`). Follow the struct's existing convention for a simple string-defaults-to-empty field. Ensure config-overridable merge (`MergeManifest`) treats it like a plain inherited field.
- Set `SessionMode: strPtr("append")` (or the plain-string form, matching the chosen struct type) on the **pi** manifest only (`internal/provider/builtin.go` `builtinPi()`). All other builtins (claude/opencode/codex/cursor/agy/gemini) ship empty — do **not** set speculatively (FR-T9 verification duty).
- **FR-T9 verification (pi, blocking for the pi value):** before merging the pi value, confirm the append-turn rendering empirically — `pi --session-id X <isolation-flags-minus-no-session> -p "remember BANANA"` then `pi --session-id X <same flags> -p "recall it"` returns `BANANA`. Record the exact verified flag set + date in a code comment on the pi manifest (same discipline as the existing FR-D5 `# VERIFIED …` comments).
- **Render variant (FR-T6):** extend `Manifest.Render` (`internal/provider/render.go`, signature `Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode)`) — or add a sibling render path — that, for a multi-turn turn, produces the bare flag set **minus** pi's `--no-session`, **plus** `--session-id <id>`, keeping `-p` and (on turn 1 only) the `system_prompt_flag`. Turns 2..N+1 render identically with the same `--session-id`. Do **not** use `--continue`/`-c`. The system prompt is supplied on turn 1 only; later turns rely on the session carrying it.
- *Mode A doc (rides with this requirement):* `docs/providers.md` — document the `session_mode` manifest field, that only pi declares `"append"` today, and the FR-T9 verification bar for adding another provider.
- *Mode A doc (rides with this requirement):* `docs/configuration.md` `[provider.<name>]` section — list `session_mode` if provider-level override is exposed there (mirror how other manifest fields are surfaced).

### R2 — Config surface: `multi_turn_fallback` + `multi_turn_chunk_tokens` (FR-T1c, FR-T3)
- Add two fields to the resolved `Config` generation struct (`internal/config/config.go`, next to `TokenLimit` / `MaxDuplicateRetries`): `MultiTurnFallback bool` (TOML `multi_turn_fallback`, default `true`) and `MultiTurnChunkTokens int` (TOML `multi_turn_chunk_tokens`, default `32000`).
- Set both in `config.Defaults()` (the block around `Timeout: 120*time.Second`, `TokenLimit: 0`, `MaxDuplicateRetries: 3`).
- Add the pair to the file-config `generation` struct (`internal/config/file.go`), the `loadTOML` overlay (mirror the `TokenLimit != 0` / `MaxDuplicateRetries != 0` guards — note `MultiTurnChunkTokens` should use a `> 0` guard, and `MultiTurnFallback` is a bool with no natural "unset" sentinel, so follow whatever pattern `auto_stage_all`/other bool defaults use), and the `merge` function (`if src.X { dst.X = src.X }` / `if src.Y != 0 { dst.Y = src.Y }`).
- *Mode A doc (rides with this requirement):* `docs/configuration.md` `[generation]` table (the rows beside `token_limit` / `max_duplicate_retries`) — add the two keys with their defaults and a one-line purpose each, and update the commented `[generation]` template block + the inert-template note. State FR-T12 (these do not interact with `token_limit`).

### R3 — Generate core: chunking + N+1 turn protocol + trigger gate + failure→rescue + verbose (FR-T1, T2, T3, T4, T5, T7, T11, T12)
- **Capture once (FR-T2):** the multi-turn payload is the SAME captured payload the one-shot path builds (FR3g skeleton + diff bodies, FR3c binary placeholders, FR-X4 exclude placeholders) — unmodified, no `token_limit` water-fill (FR-T12). Reuse the existing payload-construction in `CommitStaged` (`internal/generate/generate.go`); do not recompute.
- **Chunk sizing (FR-T3):** new helper (e.g. `internal/generate/multiturn.go`) — `N = ceil(payload_tokens / multi_turn_chunk_tokens)` using `git.EstimateTokens` (`internal/git/tokens.go:25`, chars/4). Split into N consecutive chunks ≤ the budget; anchor each boundary **forward** to the next newline so no diff line fractures. Prefix each chunk `"PART i/N:"` emitted outside the budget.
- **Trigger gate (FR-T1 a–d) — wire into `CommitStaged`:** immediately before the existing `return Result{}, &RescueError{Kind: ErrRescue, …}` at `generate.go:288` (one-shot loop exhausted), evaluate all four conditions: (a) loop exhausted on empty/unparseable output (already true at that point); (b) `EstimateTokens(payload) > cfg.MultiTurnChunkTokens` (small-payload failure skips multi-turn → rescue); (c) `cfg.MultiTurnFallback`; (d) resolved manifest `SessionMode == "append"`. If any fails, fall through to the existing rescue unchanged. If all hold, run the multi-turn attempt.
- **N+1 turn protocol (FR-T4):** mint a fresh session id `stagecoach-<run-uuid>`; turn 1 = sys prompt (via `system_prompt_flag`) + priming preamble (verbatim with N interpolated: *"I will send a git diff in N parts. After each part, reply with exactly: ok. Do not analyze or write any commit message until I explicitly ask at the end."*) + chunk 1; turns 2..N = `"PART i/N:"` + chunk i; turn N+1 = *"Now write the commit message for the diff above. Output ONLY the message."*. Intermediate `ok` stdout is discarded. The final turn's stdout is parsed by the **existing** pipeline (§9.6) and runs through duplicate rejection (§9.7) unchanged — reuse `parseOutput` + the dedupe path, do not fork them.
- **Per-turn timeout + budget (FR-T5):** each turn is a separate `Execute` with `ctx` timeout = `cfg.Timeout`; total budget = `cfg.Timeout × (N+1)`. Print a one-line progress notice at fallback time (*"falling back to multi-turn: N+1 turns, ~Mm total"*). `--verbose` logs each turn (FR-T11).
- **Failure handling (FR-T7):** any turn non-zero-exit (not timeout), turn timeout, or final-turn parse/dedupe failure → abort multi-turn and proceed to the **existing** rescue (`return … &RescueError{…}` at `generate.go:288`) exactly as a one-shot failure would. Multi-turn is pure upside; it can never leave the run worse than one-shot-exhausted.
- **Verbose (FR-T11):** under `--verbose`, log trigger, N+1, per-chunk token estimate, session id, and per-turn payload size + raw stdout/stderr.
- *Mode A doc (rides with this requirement):* `docs/how-it-works.md` — add a concise "Multi-turn fallback" subsection under generation: when it triggers (the four conditions), lossless chunking, N+1 turns, that the final turn reuses the normal parse/dedupe, and that failure falls back to the standard rescue. Cross-link §9.24.

### R4 — Tests (acceptance proof)
- **Unit (`internal/generate/multiturn_test.go`):** chunk-count ceil (`EstimateTokens` math); newline-anchored boundaries (no fractured diff line); `"PART i/N:"` prefix outside budget; trigger-gate truth table (each of the four FR-T1 conditions independently flipping to false → skip → rescue); `token_limit` non-interaction (FR-T12 — multi-turn uses the untruncated payload even when `token_limit` is set).
- **Integration (stub provider, extends the `internal/stubtest` stub-agent pattern already used by the decompose suite):** a stub that simulates `session_mode = "append"` (recalls prior-turn content by session id) — assert (a) N+1 invocations against one session id; (b) `--no-session` dropped and `--session-id <id>` present on every turn; (c) final-turn stdout parsed + deduped via the existing pipeline; (d) a mid-turn non-zero exit aborts to rescue with the frozen `TREE_SHA` (existing rescue path, unchanged message); (e) small-payload one-shot failure (payload ≤ chunk) skips multi-turn and goes straight to rescue; (f) a provider with `session_mode = ""` skips multi-turn silently. No real agent in CI.

---

## Documentation impact

**Mode A (doc-with-work, rides with the implementing requirement — noted as sub-bullets above):**
- `docs/providers.md` + `docs/configuration.md` provider section ← R1 (`session_mode` field, pi-only, FR-T9 bar).
- `docs/configuration.md` `[generation]` table + template ← R2 (the two new keys + FR-T12 note).
- `docs/how-it-works.md` generation section ← R3 (multi-turn trigger/chunking/protocol/rescue).

**Mode B (changeset-level, depends on R1–R4 — the breakdown agent should emit this as a final Task):**
- `README.md` — add a one-line feature blurb for the multi-turn fallback in the feature surface / "how it works" area (it is a user-visible reliability win for large diffs). Keep it to a sentence; do not expand scope.
- **Confirm (not edit):** `FUTURE_SPEC.md` already carries the lossless-multi-turn-graduation note and the revised chunking rejection (lossy only). Verify it is consistent with the shipped behavior; no edit expected.

---

## Reference to completed work (do not re-implement)

- **Parse / dedupe / rescue / CAS / lock are DONE and must be reused unchanged** (`internal/generate/{generate.go,dedupe.go,rescue.go,finalize.go}`, `internal/git`, `internal/config`). Multi-turn only **inserts a branch** before the existing rescue and **reuses** `parseOutput`, the dedupe path, and `*RescueError`.
- **`Manifest.Render` (`internal/provider/render.go:89`) and `MergeManifest`** are the established extension points — add the multi-turn render variant and the `session_mode` field through them, not via a parallel mechanism.
- **`git.EstimateTokens` (`internal/git/tokens.go:25`)** is the shared token estimator (chars/4) — use it for both chunk sizing (FR-T3) and the gate (FR-T1b); do not introduce a second estimator.
- **Config patterns** (`TokenLimit`/`MaxDuplicateRetries` across `config.go` `Defaults()` + `file.go` load/overlay/merge) are the exact template for the two new keys.
- **Stub-agent integration pattern** (`internal/stubtest`, used by the decompose suite) is the template for the multi-turn stub test — do not stand up a new harness.

## Risks / notes for the implementer
- **FR-T9 is a blocking verification** for the pi `session_mode = "append"` value: the append-turn rendering (`--no-session` dropped, `--session-id` added, sys prompt set on turn 1 only and recalled) must be confirmed empirically before the pi value ships. If verification fails, ship pi with `""` and the whole feature is inert until resolved — surface this clearly.
- **No new exit codes, no CLI flags, no rescue-message changes.** Multi-turn is internally triggered; the only user-visible surfaces are the progress line (FR-T5), `--verbose` (FR-T11), and the two config keys (R2).
- **Per-turn timeout compounds.** A 120s × (N+1) budget can be many minutes; the FR-T5 progress line is mandatory so the user is not left staring at a blank prompt.
