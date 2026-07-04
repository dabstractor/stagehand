# README Race-Free Safety Property + Stale-Claim Sweep — P1.M1.T3.S1 Research

> Empirically verified against the live repo at 2026-07-03, after the FR52 run-lock feature landed
> (P1.M1.T1.S1 lock primitive COMPLETE; P1.M1.T2.S1 Busy exit code COMPLETE; P1.M1.T2.S2 contention
> wiring COMPLETE on disk; P1.M1.T2.S3 e2e in flight). Every grep result below was re-confirmed by
> running the actual command before writing this file.

## 1. What this task is (Mode B changeset-level doc sync)

This is the changeset-level documentation task for the plan/006 "Per-repo run lock (FR52)"
feature. It has three legs (item contract 3a/3b/3c):

- **3a (the ONE real edit):** surface the new race-free / safe-to-double-invoke safety property in
  README.md's safety section.
- **3b (stale-claim sweep):** grep README.md + docs/ for stale concurrency claims that imply two
  stagehand processes can safely race without a lock; fix any found.
- **3c (catch-all verification):** confirm docs/cli.md has the Busy=5 row and docs/how-it-works.md
  has the Per-repo run lock subsection; add either if missing.

## 2. Empirical findings: 3b and 3c are no-ops; 3a is one additive edit

### 2.1 The stale-claim sweep (3b) returns ZERO matches

```
$ grep -rniE 'no lock|safe without|safely race|without a lock|don.t need a lock|no need to lock|only.*defense|cas is the (only|sole)' README.md docs/
(no output — exit 1)
```

No doc anywhere claims the CAS is the sole concurrency defense, or that two runs are safe without a
lock, or uses "no lock"/"concurrent runs are safe without" language. README.md does not discuss
concurrency at all today; docs/how-it-works.md ALREADY has the correct two-stage defense (lock +
CAS, see 2.3); docs/configuration.md documents the lock LOCATION. **Conclusion: the sweep is a
verification no-op.** No stale wording exists to fix.

### 2.2 The README safety claim is accurate, not stale (leave it; only APPEND)

- README:4 (hero): *"...commits only what was staged when it started, atomically, and **can never
  corrupt your repo**."* — this corruption guarantee is defended by the **CAS** (§13.5), which is
  **universal** (holds even cross-host / shared FS). It is NOT overclaimed — the CAS genuinely
  prevents corruption under any concurrency (the loser aborts, leaving a dangling snapshot, never a
  clobbered HEAD). The lock is defense-in-depth for UX (avoid the redundant run / dangling
  snapshot), not a corruption guard. **LEAVE the hero.**
- README:326–328 (FAQ "Will it corrupt my repo?"): the existing 2-sentence answer is purely about
  atomicity ("A failed generation leaves the repo byte-for-byte unchanged"). It contains no
  concurrency claim, stale or otherwise. **LEAVE it; APPEND the new race-free paragraph here** (this
  is the safety section the new property composes with — contract 3a names "Never corrupt your repo"
  as the anchor).

### 2.3 The catch-all (3c) is ALREADY satisfied — sibling docs present and correct

| Doc | Required content | Status at HEAD | Owner |
|---|---|---|---|
| `docs/cli.md` | Busy=5 exit-code row + contention explanation | ✅ L374 (`\| 5 \| Busy …`), L379 (no-op-vs-Busy prose) | P1.M1.T2.S1 (Complete) |
| `docs/how-it-works.md` | "Per-repo run lock (FR52)" subsection | ✅ L144–157 (two-stage defense, per-host limit, never-in-repo, no-op fast path, auto-release, Windows stub) | P1.M1.T1.S1 (Complete) |
| `docs/configuration.md` | Lock file location (XDG resolution) | ✅ L233–239 (XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache; sha256 hash; never in repo) | (landed) |

**The catch-all does not fire.** All three sibling-owned docs are present, correct, and mutually
consistent with §18.5. This task must NOT edit them (sibling-owned — see §4 scope).

## 3. The README edit (3a) — the ONE additive change

**Insertion point:** the FAQ answer "### Will it corrupt my repo?" (README:326–328), immediately
after the existing atomicity sentence. This is the safety section the contract names ("alongside
'Never corrupt your repo'"); the new property "composes with" the atomicity pitch (system_context
§5).

**Drafted text (the implementer may polish wording; substance is fixed by contract 3a):**

> **Safe to run twice.** A per-repo run lock prevents two concurrent commit-producing runs from
> racing on HEAD, so an accidental double-invoke degrades gracefully: if nothing new is staged it
> exits `0` (*nothing to do — an in-progress run already covers your staged changes*); if genuinely
> new work is staged it exits `5` (Busy) and leaves your changes staged to re-run. (On a shared
> filesystem across hosts the lock can't help — the atomic `update-ref` CAS is the
> never-clobber-HEAD guarantee there.)

**Why this wording satisfies contract 3a (all four substance points):**
1. "safe to invoke twice" / "double-invoke degrades gracefully" → the race-free property. ✅
2. "per-repo run lock prevents two concurrent commit-producing runs from racing on HEAD" → names the
   mechanism and scope (commit-producing runs, HEAD race). ✅
3. "exit 0 'nothing to do' if nothing new is staged, or a clear 'busy, retry' exit otherwise" → the
   two contention outcomes, with the exact exit codes (0, 5) and message fragments that match
   docs/cli.md:379 and docs/how-it-works.md:155. ✅
4. "don't overclaim shared-filesystem safety — the CAS covers that, not the lock" → the parenthetical
   per-host caveat. ✅

**Consistency anchors (so the README wording matches the landed docs):**
- Exit codes: `0` (Success) and `5` (Busy) per docs/cli.md:370–374 / internal/exitcode/exitcode.go.
- No-op message fragment: "nothing to do — an in-progress run already covers your staged changes"
  (docs/cli.md:379 / how-it-works.md:155).
- "never-clobber-HEAD" CAS guarantee: how-it-works.md:149 ("the second, never-clobber-HEAD
  guarantee").
- Do NOT promise cross-host/shared-FS lock coverage (the per-host limit — how-it-works.md:151).

## 4. Scope discipline (what this task must NOT touch)

| File | This task? | Why |
|---|---|---|
| `README.md` | ✅ EDIT (one append) | Owns the race-free bullet (3a). |
| `docs/cli.md` | ❌ NO-TOUCH (verify only) | P1.M1.T2.S1 — Busy row already present (3c catch-all does not fire). |
| `docs/how-it-works.md` | ❌ NO-TOUCH (verify only) | P1.M1.T1.S1 — FR52 subsection already present. |
| `docs/configuration.md` | ❌ NO-TOUCH (verify only) | Lock-location section already present. |
| `docs/providers.md`, `docs/README.md` | ❌ NO-TOUCH | No concurrency content; sweep returned nothing. |
| any `*.go` (production/test) | ❌ NO-TOUCH | The feature is code-complete (S1/S2 landed, S3 in flight). Doc-only task. |
| `PRD.md`, `tasks.json`, `prd_snapshot.md`, `.gitignore` | ❌ NO-TOUCH | Forbidden. |

## 5. Parallel-execution note (P1.M1.T2.S3 in flight)

P1.M1.T2.S3 adds `internal/e2e/lock_scenarios_test.go` — a `//go:build e2e` TEST file under a build
tag excluded from the default suite. It touches NO docs and NO README. There is zero overlap with
this task (README.md + the docs sweep). The two are fully independent.

## 6. Decisions log

- **D1** — The hero "can never corrupt your repo" (README:4) is accurate (CAS-defended) and is NOT
  edited. The new property is APPENDED to the FAQ safety answer, not the hero (keeps the pitch
  tight; the contract's anchor is "Never corrupt your repo" = the FAQ).
- **D2** — The stale-claim sweep (3b) is a genuine no-op (zero grep matches). Do NOT invent a stale
  claim to fix; do NOT rewrite the accurate CAS/atomicity sentences.
- **D3** — The catch-all (3c) does not fire: cli.md (Busy row), how-it-works.md (FR52 subsection),
  configuration.md (lock location) are all present and correct at HEAD. Verify-and-confirm only; do
  NOT edit sibling docs.
- **D4** — The new README paragraph must NOT overclaim shared-filesystem safety: the per-host caveat
  (CAS, not the lock, covers cross-host) is mandatory per contract 3a.
- **D5** — Wording must stay consistent with the landed docs' exit codes (0/5) and message fragments
  ("nothing to do …", "never-clobber-HEAD") so the README doesn't drift from cli.md/how-it-works.md.
