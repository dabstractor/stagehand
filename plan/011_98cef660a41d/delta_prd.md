# Delta PRD — v2.5: Closed-loop token budget + stale lock-file reaping

| Field | Value |
|---|---|
| **Delta from** | session 010 (v2.4 — hook execution on the commit path, FR-V1–V8) |
| **Delta to** | v2.5 |
| **Change size** | Small-to-medium. Two reliability-hardening fixes; **no new CLI/config surface, no new user-facing feature.** Both compose with existing mechanisms. |
| **PRD diff** | +10 / −4 lines: new `This revision (v2.5)` note; new **FR3j**; amended **FR3d** (one clause); amended **FR52** (one clause); §18.5 expanded (Contents note amended, Mechanism paragraph rewritten, +2 new paragraphs: *Stale-file reaping*, *Exit-path release*). |

---

## 1. What changed (and what did NOT)

v2.5 is two independent fixes surfaced by a large-diff failure investigation. Neither adds a flag, a config key, a subcommand, or a manifest field. Both harden guarantees the v2.4 spec already *claimed* but the v2.4 design only *approximated*.

### 1.1 Closed-loop token-budget guarantee — **NEW FR3j** (§9.1), amends FR3d

The `token_limit` water-fill (FR3i, **already implemented** in `internal/git/tokengate.go::applyWaterFillGate`) sized each diff body by subtracting *estimates* of the non-body parts (skeleton + system prompt + payload framing + margin) from `token_limit`. That is an **open-loop** budget: the worst-case `PromptReserveTokens` (`internal/prompt/reserve.go`) can drift from the *actually-assembled* prompt (real rejected-subject count vs the max, `chars/4` vs real token density, framing measured in isolation). In practice the assembled prompt was observed to land **slightly over** `token_limit` (~152K delivered against a 150K limit) — before the model's own per-request sub-window unreliability even came into play.

**FR3j closes the loop.** After the water-fill produces the gated diff, stagecoach assembles the *actual* full prompt (system prompt + `BuildUserPayload(gatedDiff)`), measures it with the **same** `EstimateTokens` (`chars/4`) used for sizing, and if it exceeds `token_limit`, reduces the body budget by the overshoot plus a small slack and re-applies FR3i's per-file truncation, re-measuring until it fits (a bounded loop — converges in 1–2 passes).

> **Invariant (FR3j):** `EstimateTokens(assembledFullPrompt) ≤ token_limit`, always. The prompt is **never** delivered over `token_limit`. Being *under* is always fine and requires no correction.

FR3i is **reframed** (not removed) as the fast, fair *first-cut allocator* that runs under FR3j's closed-loop guarantee. FR3d's wording is amended to call the fit a "closed-loop guarantee (FR3j), not an estimate."

### 1.2 Stale lock-file reaping — amends FR52 (§9.9) and §18.5

The v2.4 lock design (`internal/lock/lock.go`, **already implemented**) leaned on `flock`'s auto-release-on-exit and explicitly claimed "no stale locks to reap" and "no PID-liveness heuristics." That claim is true for the *lock* (the flock itself never goes stale) but **false for the lock *file***: the file on disk survives any non-deferred exit — the signal-rescue path's `os.Exit(3)` (in `internal/signal/signal.go::handle`), `SIGKILL`, or a crash all skip `defer locker.Release()` (in `internal/cmd/default_action.go`). Each interrupted run orphans one `<hash>.lock` file; over time, unbounded disk litter.

FR52 / §18.5 corrects the over-claim and adds two complementary mechanisms:

1. **Stale-file reaping (self-healing, in `lock.Acquire`).** After taking its own flock, `Acquire` removes every `*.lock` in the lock directory whose recorded `pid` is **not a live process on its recorded `hostname`** (`kill(pid, 0)` → `ESRCH`). A dead `pid` holds no open fd → no flock → unlinking is safe (it cannot defeat contention the way unlinking a *live* holder's inode-bound flock would). A **live** pid is never reaped, even if it appears stuck — preserving FR52's "never force-break" guarantee. Hostname-matching scopes reaping to this host; a recycled pid on this host is a benign miss.
2. **Exit-path release (prevention, in the signal-rescue path).** The signal handler now releases the lock file *before* calling `os.Exit(3)`, via an **injected seam** (the `signal` package is stdlib-only and cannot import `lock` — mirrors the existing `RescueFormat` injected-seam pattern). This removes the *common* staleness producer (Ctrl-C during generation), so reaping is a backstop for `SIGKILL`/crash rather than the hot path.

> **No commit/CAS/rescue behavior changes.** The lock still serializes stagecoach-with-stagecoach; the §13.5 CAS is still the never-clobber-HEAD guarantee. Only disk hygiene + the signal-exit path change.

---

## 2. Scope delta

### 2.1 New requirements

- **FR3j — Closed-loop budget guarantee (never over, under is fine).** See §1.1. P0 (→ G1/G3, composing with the existing FR3i gate).

### 2.2 Amended requirements (note what changed)

- **FR3d (§9.1).** Wording only: the fit is now "a closed-loop guarantee (FR3j), not an estimate." No behavioral change to the unset/`0` path or the legacy-caps mutual exclusivity.
- **FR52 (§9.9).** Wording only: notes that the *lock* cannot go stale but orphaned lock *files* are reaped by pid-liveness. No change to contention behavior, the no-op fast path, or the "never force-break" guarantee.
- **§18.5 (the lock detail).** "Contents" note: `pid`/`hostname` are now also reused for stale-*file* reaping. "Mechanism" paragraph: corrected the "no stale locks to reap" over-claim. **Two new paragraphs added:** *Stale-file reaping (lock vs file)* and *Exit-path release (prevention)*.

### 2.3 Removed requirements

None.

---

## 3. Implementation context (reference the completed v2.4 work)

Both fixes compose with **already-implemented** v2.4 mechanisms. The implementing agent reads these, not the PRD prose:

**Token-budget gate (FR3j composes here):**
- `internal/git/tokengate.go::applyWaterFillGate(mdDiffs, nmDiff, skeleton, tokenLimit, promptReserve)` — the PURE FR3i first-cut allocator. Sizing + enforcement both use `EstimateTokens` over the same body (`sectionBody` via `atAtRe` ≡ `truncateByWaterFill`'s split) → coherence.
- `internal/git/waterfill.go::allocByWaterFill`, `internal/git/truncatediff.go::truncateByWaterFill`, `internal/git/tokens.go::EstimateTokens` — the composed helpers.
- `internal/prompt/reserve.go` — the worst-case `PromptReserveTokens` helpers (`MessageReserveTokens` / `PlannerReserveTokens` / `ArbiterReserveTokens`), which inject `git.EstimateTokens` as `TokenEstimator` to stay leaf-pure. **FR3j's seam follows this same injection pattern** (the git gate cannot import `prompt`).
- **6 gate call sites** (all carry `TokenLimit` + `PromptReserveTokens`): `generate.CommitStaged` (generate.go:208), `pkg/stagecoach.runPipeline` (stagecoach.go:437), `hook.exec.Run` (exec.go:123), `decompose.generateMessage` (message.go:91), `decompose.callPlanner` (planner.go:79), `decompose.runArbiterPhase` (decompose.go:663). FR3j says the guarantee holds for **every role** that routes through the gate (message / planner / arbiter).

**Lock (FR52 reaping composes here):**
- `internal/lock/lock.go::Acquire` (opens `O_CREATE`, `flock`, writes `pid=/hostname=/repo=/timestamp=/snapshot=`), `Release` (close fd → `os.Remove`, best-effort), `SetSnapshot` package wrapper over the `current atomic.Pointer[Locker]` singleton.
- `internal/signal/signal.go::handle` — the rescue path: forwards signal → cancels ctx → prints `RescueFormat` → `opts.Exit(3)`. `Options` already has injected seams (`RescueFormat`, `Out`, `Kill`, `Exit`); wired in `cmd/stagecoach/main.go`.
- `internal/cmd/default_action.go:59-67` — `lock.Acquire` + `defer locker.Release()`. The defer is what `os.Exit(3)` skips.

**No pid-liveness helper exists yet** — `kill(pid, 0)`/`ESRCH` appears nowhere in `internal/` (verified). It must be added.

---

## 4. Documentation impact

**Mode A (ride with the work):**
- FR3j → `docs/configuration.md` (the `token_limit` note, ~line 160: strengthen "so the payload always fits" to name the closed-loop guarantee).
- FR52 reaping → `internal/lock/lock.go` package/struct/method doc comments (3 sites currently say "no stale locks" / "auto-released on process death (no stale locks)" — correct the over-claim in-place; this is code commentary, rides with the reaping work).

**Mode B (changeset-level, depends on all implementing work):**
- `docs/how-it-works.md` — two overview-claim corrections that only read correctly once both halves land:
  1. The "Holistic token budget" paragraph (~line 148): note the closed-loop backstop (the water-fill is the first cut; the assembled prompt is re-measured and re-trimmed to never exceed `token_limit`).
  2. The lock section (~lines 170, 179): flip "no stale-lock reaping or PID-liveness checks needed" and "No stale locks, no PID-liveness checks, no reaping" → the lock-*file* reaping reality + the exit-path release. This is the headline doc correction.

`README.md` has no token-limit or lock-staleness claim to update (verified: no `token_limit`/`stale`/`reap` mentions in the feature surface). No Mode B README work.

---

# IMPLEMENTATION PLAN

## Phase P1 — Reliability hardening (v2.5): closed-loop token budget + stale lock-file reaping

One phase, two milestones (the fixes are independent and touch different subsystems), plus a Mode B docs sweep that depends on both. Sized to the change: ~5 implementing subtasks total, no new CLI/config/manifest surface.

### Milestone P1.M1 — Closed-loop token-budget guarantee (FR3j, amended FR3d)

**The seam decision (stated up front so the breakdown agent doesn't relitigate it):** the closed loop must measure the *real assembled prompt* (`sysPrompt + BuildUserPayload(gatedDiff, context, rejected)`), which the git gate cannot do — it has no prompt knowledge, and `prompt` cannot be imported into `git` (leaf-purity). The existing codebase already solves exactly this shape: `internal/prompt/reserve.go` **injects** `git.EstimateTokens` as a `TokenEstimator` callback so the prompt package stays dependency-free. FR3j uses the **same injection pattern in the other direction**: the git gate accepts an injected `MeasureAssembled func(gatedDiff string) int` callback (the caller closes over `sysPrompt`/`context`/`rejected` and returns `EstimateTokens(sysPrompt + BuildUserPayload(gatedDiff, …))`), and runs the bounded re-trim loop **inside the git diff function** where the raw ungated sections are retained (so it can re-run `applyWaterFillGate` at a reduced `tokenLimit` and preserve FR3i's water-fill fairness — a flat re-cut of an already-gated string would lose fairness).

This keeps the loop in one place (covers all 6 call sites via the shared `StagedDiffOptions` field) rather than duplicating the loop logic across `generate`/`pkg`/`hook`/`decompose`.

#### Task P1.M1.T1 — Closed-loop re-trim in the token gate

- **Subtask P1.M1.T1.S1 — The closed-loop helper + injected seam (story points: 2).**
  Add a `MeasureAssembled func(gatedDiff string) int` field to `git.StagedDiffOptions` (and document it as the FR3j closed-loop seam). Inside the three diff functions that route through `applyWaterFillGate` (`StagedDiff`/`TreeDiff`/`WorkingTreeDiff` in `internal/git/git.go`), after the first-cut gate, run the bounded loop: `assembled := opts.MeasureAssembled(gatedDiff)`; if `assembled > opts.TokenLimit`, set `effectiveLimit := opts.TokenLimit - (assembled - opts.TokenLimit) - slack` (slack ≈ a small constant; reuse the existing `tokenBudgetMargin` discipline or a tighter closed-loop slack), re-run `applyWaterFillGate(mdDiffs, nmDiff, skeleton, effectiveLimit, opts.PromptReserveTokens)` over the **same retained sections**, re-measure, repeat until `assembled ≤ opts.TokenLimit` (cap the loop at e.g. 4 passes — the estimate is already close, so 1–2 suffices; a defensive cap prevents a pathological loop). If `MeasureAssembled == nil` (e.g. legacy/library callers, or `TokenLimit == 0`), behave exactly as today (first-cut only) — **no behavior change when `token_limit` is unset or no callback is supplied.** Pure-unit-testable like `applyWaterFillGate` (extend `tokengate_test.go`): assert the invariant `EstimateTokens(assembled) ≤ token_limit` holds for skewed estimators that deliberately drift from the worst-case reserve, and assert the loop converges. *prd_selectors: h3.17*
- **Subtask P1.M1.T1.S2 — Wire `MeasureAssembled` at all 6 gate call sites (story points: 2, depends on S1).**
  At each of `generate.CommitStaged`, `pkg/stagecoach.runPipeline`, `hook.exec.Run`, `decompose.generateMessage` (message role: `BuildUserPayload`); `decompose.callPlanner` (planner role: `BuildPlannerUserPayload`); `decompose.runArbiterPhase` (arbiter role: `BuildArbiterUserPayload`) — pass a `MeasureAssembled` closure that captures that site's `sysPrompt` + `context` + current `rejected` and returns `git.EstimateTokens(sysPrompt + <role's BuildUserPayload>(gatedDiff, …))`. Only supply the callback when `cfg.TokenLimit != 0` (unset ⇒ legacy path, no callback, no change). The closure uses the **same** `EstimateTokens` used for sizing (single-estimator rule — no second formula). *prd_selectors: h3.17*

  - **Mode A docs (ride with S2):** `docs/configuration.md` `token_limit` note (~line 160) — strengthen "so the payload always fits your model's context window" to name the closed-loop guarantee ("…re-measured after assembly and re-trimmed so the assembled prompt never exceeds `token_limit` (FR3j)"). One clause; no new knob.

### Milestone P1.M2 — Stale lock-file reaping + exit-path release (amended FR52)

Two tasks, independent of M1; either may proceed first. Both are small.

#### Task P1.M2.T1 — Stale-file reaping in `lock.Acquire`

- **Subtask P1.M2.T1.S1 — pid-liveness helper + reap loop (story points: 2).**
  Add a `processAlive(pid int, hostname string) bool` helper to `internal/lock` (cross-platform: on Unix, `syscall.Kill(pid, 0)` — `ESRCH` ⇒ dead, `nil` ⇒ alive, `EPERM` ⇒ alive-but-not-ours; on Windows, `lock_windows.go` returns `false` always since flock is a no-op stub there and reaping is a no-op too). Hostname check: alive only if the recorded hostname matches `os.Hostname()` (scopes reaping to this host — never reap a file a *different* machine's live process holds on a shared filesystem). In `Acquire`, **after** successfully taking the flock on `<hash>.lock`, glob the lock directory for `*.lock`, parse each via the existing `parseContents`, and `os.Remove` every file whose `processAlive(pid, hostname)` is false. **Never** unlink a file whose pid is alive (the inode-bound-flock hazard — would let a contender `O_CREATE` a fresh inode and flock it free, defeating FR52). Reaping is best-effort: parse/remove errors are ignored (a malformed file is skipped; the next `Acquire` retries). Update the `lock.go` doc comments (package doc + `Acquire` + `Locker`) that say "no stale locks" / "auto-released on process death (no stale locks)" to the corrected lock-vs-file framing. Unit-test in `lock_test.go` with a temp lock dir: seed a `<deadpid>.lock` (a pid guaranteed not to exist, e.g. a very high pid) + a live-pid file (this process's own pid) + a foreign-hostname file, then `Acquire` and assert only the dead-pid file is removed. *prd_selectors: h3.25, h3.91*

#### Task P1.M2.T2 — Exit-path release (the signal-rescue seam)

- **Subtask P1.M2.T2.S1 — `lock.ReleaseCurrent` package wrapper (story points: 1).**
  Add `func ReleaseCurrent()` to `internal/lock`, mirroring the existing `SetSnapshot` package wrapper over the `current atomic.Pointer[Locker]` singleton: `if l := current.Load(); l != nil { l.Release() }` (nil-safe for library use where no lock is held). `Release` already closes the fd (releasing the flock) **then** `os.Remove`s the file — so `ReleaseCurrent` is exactly the cleanup the deferred `locker.Release()` would have run, made callable from the signal path. Idempotent (the `l.file == nil` guard in `Release` makes a second call a no-op).
- **Subtask P1.M2.T2.S2 — `signal.Options` injected seam + main.go wiring (story points: 1, depends on S1).**
  Add an `OnRescueExit func()` field to `signal.Options` (defaulted to a no-op in `Install` if nil, mirroring how `RescueFormat`/`Kill`/`Exit` are defaulted). In `signal.handle`, call `h.opts.OnRescueExit()` **immediately before** `h.opts.Exit(...)` in **both** exit branches — the post-snapshot rescue branch (`Exit(3)`, after the rescue message is printed) AND the pre-snapshot branch (`Exit(130/143)`). Rationale (verified): the lock is acquired at the very start of `default_action.go:59` (well before the snapshot is armed via WriteTree + `signal.SetSnapshot` deep in `generate.CommitStaged`/`runPipeline`), so a Ctrl-C in the **pre-snapshot** window (lock held, snapshot not yet armed) also orphans the lock file via `os.Exit` skipping `defer locker.Release()`. Calling `OnRescueExit` in both branches closes both windows; `lock.ReleaseCurrent` is nil-safe (no lock in library use / before `lock.Acquire`) and idempotent (`Release`'s `l.file==nil` guard makes a second call a no-op), so the double-coverage is safe. Wire it in `cmd/stagecoach/main.go`'s `signal.Install(...)` call: `OnRescueExit: lock.ReleaseCurrent`. This removes the common staleness producer (any Ctrl-C after the lock is acquired), so reaping (M2.T1) is the backstop for `SIGKILL`/crash only. Verify via `signal_test.go` that the injected `OnRescueExit` is called exactly once on a signal in **both** the armed-snapshot and pre-snapshot branches. *prd_selectors: h3.90, h3.91*

### Task P1.M3 — Sync changeset-level documentation (Mode B)

Depends on **all** implementing subtasks (P1.M1.T1.S2, P1.M2.T1.S1, P1.M2.T2.S2). The overview doc reads correctly only once both halves land.

- **Subtask P1.M3.S1 — `docs/how-it-works.md` overview corrections (story points: 1).**
  Two corrections: (a) *Holistic token budget* paragraph (~line 148): append a sentence noting the closed-loop backstop — after the water-fill, the assembled prompt is re-measured with the same estimator and re-trimmed so it never exceeds `token_limit` (FR3j); the water-fill is the first-cut allocator under that guarantee. (b) *Lock section* (~lines 170, 179): flip the now-false "no stale-lock reaping or PID-liveness checks needed" / "No stale locks, no PID-liveness checks, no reaping" claims → the corrected framing: the `flock` *lock* still auto-releases on process death (never stale), but orphaned lock *files* are now reaped on acquire by pid-liveness on this host, and the signal-rescue path releases the file before exiting (so reaping is a backstop for `SIGKILL`/crash). Keep the existing "never force-breaks a live lock" guarantee prominent. Verify with a `docs/` grep that no stale "no stale locks" / "no reaping" claim remains. *prd_selectors: h3.17, h3.91*

---

## 5. Acceptance (folded into the tasks above)

- **FR3j invariant (in P1.M1.T1.S1's tests):** for any `token_limit > 0` and any estimator drift, `EstimateTokens(sysPrompt + BuildUserPayload(gatedDiff, …)) ≤ token_limit` after the gate. Going under is fine. Unset `token_limit` ⇒ byte-identical to today.
- **FR52 reaping (in P1.M2.T1.S1's tests):** a dead-pid lock file is removed on `Acquire`; a live-pid file is never removed; a foreign-hostname file is never removed.
- **Exit-path release (in P1.M2.T2.S2's tests):** a signal in **both** the post-snapshot (rescue) and pre-snapshot (lock-held-but-not-armed) windows calls `OnRescueExit` once before the exit; the lock file is gone after the (faked) exit in both cases.
- **No regressions:** the existing `tokengate_test.go` / `waterfill_test.go` / `truncatediff_test.go` / `lock_test.go` / `signal_test.go` suites pass unchanged in behavior (the closed loop and reaping are strictly additive).

---

## 6. Out of scope

- Anything not in §1. The hook-execution work (FR-V1–V8) from session 010 is **complete and untouched**. No new providers, no new flags, no config-schema bump, no manifest changes. The multi-turn fallback (FR-T1–T12) deliberately ignores `token_limit` (FR-T12) and is unaffected — FR3j governs only the one-shot path.
