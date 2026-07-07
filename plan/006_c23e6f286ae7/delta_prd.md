# Delta PRD — Per-repo run lock (FR52)

| Field | Value |
|---|---|
| **Source PRD** | `PRD.md` (v2.1 specification) |
| **Prior session** | `plan/005_c38aa48290f0` — completed the six v2.1 capabilities (§9.18–§9.23); entire v2.0/v3 core implemented |
| **Delta scope** | **Exactly one new feature: FR52 — Per-repo run lock.** Plus its cross-reference sentences (§13.4, §18 intro), the new spec section §18.5, and a rejected-alternative note in Appendix F. |
| **Size class** | Small-to-medium. **One** new requirement, **one** new subsystem (`internal/lock`), **one** new exit code. Defense-in-depth over the *already-implemented* §13.5 CAS — does **not** re-implement the snapshot/commit machinery. |

---

## 1. What actually changed (diff analysis)

A literal `diff` of `plan/005_c38aa48290f0/prd_snapshot.md` vs `PRD.md` yields five edits, all in service of a single feature:

1. **§9.9 (Commit creation) — new FR52.** "Per-repo run lock." A commit-producing run acquires an exclusive, non-blocking lock scoped to the repo before snapshotting/generating, so two stagecoach processes cannot race on HEAD (which would otherwise trip the §13.5 CAS and, on the duplicate-run path, surface the "already committed" message).
2. **§18 (Concurrency) intro — one sentence added.** "At most one stagecoach process may produce commits in a given repo at a time … a per-repo run lock (FR52 / §18.5) serializes concurrent invocations."
3. **§13.4 (Stage-while-generating) — one sentence added.** The single-process safety note, plus "the per-repo run lock makes that race impossible to stumble into."
4. **§18.5 — new section (~24 lines).** The authoritative spec: scope, location (per-system runtime dir, never inside the repo), contents (pid/hostname/repo/timestamp/`snapshot=`), mechanism (advisory `flock`, `LOCK_EX|LOCK_NB`, auto-released on exit → no stale locks), contention behavior (no-op fast path + non-zero `Busy` exit naming the holder), and limits (per-host only; the CAS covers a shared filesystem).
5. **Appendix F — one rejected-alternative entry.** "Run lock, not a run queue": explains why an auto-committing queue was rejected and records the depth-1 subtractive queue as a future possibility (out of scope).

**Nothing else changed.** No goals/non-goals moved, no other FRs altered, no provider/config/distribution changes. This is a self-contained concurrency hardening layer.

---

## 2. Scope delta

### 2.1 New requirement (the only work)

**FR52 — Per-repo run lock (P0).** Implement §18.5 in full. Before a commit-producing run snapshots or generates, acquire an exclusive, non-blocking lock scoped to the repo. Two behaviors on contention:

- **No-op fast path (exit 0).** If the holder has published a `snapshot=` and the contending run's own staged snapshot (`git write-tree`, index-read-only, safe to take without the lock) is byte-identical to it, nothing new has been staged since the holder began → the contending run is a redundant accidental-double-run → exit 0 with "nothing to do — an in-progress run already covers your staged changes."
- **Busy refusal (new non-zero exit `Busy`).** If a genuine second batch is staged, exit non-zero with a message naming the holder (pid, hostname, repo) and the lock path. Distinct from the commit-failure codes so scripts can tell "busy, retry" from "failed." **Never block; never force-break the lock.**

**Mechanism & location (prescribed by §18.5):**
- `flock(2)` with `LOCK_EX | LOCK_NB` on `<hash>.lock`. Released automatically on process exit (incl. `SIGKILL`/crash) → **no stale locks, no PID-liveness heuristics.** This is a deliberate rejection of the `O_CREAT|O_EXCL`+PID-check pattern.
- `<hash>` = `sha256` hex of the repo's **canonical absolute path**. Two repos → independent locks; two terminals in the same repo → contention (correct).
- Location: `$XDG_RUNTIME_DIR/stagecoach/locks/<hash>.lock` when set; else `$XDG_CACHE_HOME/stagecoach/locks/`, falling back to `~/.cache/stagecoach/locks/`. **Never inside the repo** (would pollute `git status`, be committable, be ambiguous across worktrees, lost on clone).
- Lock-file contents (one `key=value` per line): `pid`, `hostname`, `repo`, start `timestamp`, and `snapshot=<frozen-tree-sha>` once the holder freezes its snapshot. `pid`/`hostname`/`repo` are diagnostic (used by the contention message); `snapshot=` drives the no-op fast path. None of it is used for stale-lock reaping.

### 2.2 Modified requirements (note only — no re-implementation)

- **§13.4 / §13.5 (snapshot + CAS).** Unchanged in behavior. The lock is the **first** line of defense (prevents the common local double-run); the existing §13.5 CAS (already implemented as `git.UpdateRefCAS` / `ErrCASFailed`) is the **second** (the never-clobber-HEAD guarantee, which holds even on a shared filesystem the lock cannot cover). **Both stay — defense in depth.** No changes to the CAS path.

### 2.3 Removed requirements

None.

### 2.4 Documentation impact

**Mode A — doc-with-work (rides with the implementing tasks, as sub-bullets):**
- `docs/cli.md` — add the **`Busy` exit code** to the §15.4 exit-codes reference, and document the two contention behaviors (no-op exit 0, busy refusal). *(rides with the contention/wiring task)*
- `docs/how-it-works.md` — a concurrency subsection: the two-stage defense (run lock + CAS), the **per-host limit** (shared/network filesystems are the CAS's job, not the lock's), the "never inside the repo" location rationale, and the no-op fast path. *(rides with the lock primitive task — it owns the §18.5 spec)*
- `docs/configuration.md` — lock-file location resolution via `XDG_RUNTIME_DIR` / `XDG_CACHE_HOME` / `~/.cache`. *(rides with the lock primitive task)*

**Mode B — changeset-level docs (final task):**
- `README.md` — surface the new **race-free / safe-to-double-invoke** safety property in the safety section (next to the existing snapshot/atomic-commit pitch). This is a cross-cutting safety claim that only reads correctly once the whole change lands, so it is a standalone final requirement depending on the above.

### 2.5 Out of scope (explicitly)

- **Depth-1 subtractive run queue** (auto-commit batch 2 via `diff(T1,T2)` with a disjoint-files precondition). Appendix F records it as a future possibility. **Do not build it.** Only the no-op-on-empty-delta fast path is adopted from the queue idea.
- Shared-filesystem / cross-host mutual exclusion. The lock is per-host by design; the CAS covers that gap.
- Serializing stagecoach against *other* tools (editors, other coding agents). That is the snapshot/freeze boundary's job (FR-M1b), not the lock's.

---

## 3. Reference to completed work (do not re-implement)

The entire v2.0/v3 core is implemented and green (`plan/005_c38aa48290f0/architecture/system_context.md` §0). This delta **composes over** it:

- **CAS is done.** `git.UpdateRefCAS` / `ErrCASFailed` (`internal/git`) implement §13.5. The lock is *additional*; do not touch the CAS.
- **Default action entry points exist.** `runDefault` (single-commit, `internal/cmd/default_action.go:34`), `runDecompose` (`:308`), and `shouldDecompose` (`:291`). The lock acquires at the top of both commit-producing paths, before the first `WriteTree` (single) / before `FreezeWorkingTree` / `T_start` capture (decompose).
- **Exit-code registry exists.** `internal/exitcode` (`Success=0, Error=1, NothingToCommit=2, Rescue=3, Timeout=124`) maps errors via `For()`. Add `Busy` here; wire it through `For()`/`ExitError`.
- **Snapshot freeze exists.** Single path: `git.WriteTree` → tree SHA. Decompose path: `git.FreezeWorkingTree` → `T_start`. The `snapshot=` field is written from whichever freeze the holder took.
- **Index-read-only tree read exists.** The no-op fast path's contending `write-tree` is the same call the single path uses; it reads the index without mutating refs and is safe to take without the lock.
- **XDG dir-resolution pattern exists** in `internal/config` (global config at `$XDG_CONFIG_HOME`). Reference that pattern, but resolve against `XDG_RUNTIME_DIR` / `XDG_CACHE_HOME` per §18.5 (different XDG base dirs).

**Net-new:** the `golang.org/x/sys` dependency (for `unix.Flock`) is **not** currently in `go.mod` — it must be added. This is the only new third-party dependency the feature introduces.

---

## 4. Plan (proportional: 1 phase, 1 milestone, 3 tasks)

### Phase P1 — Per-repo run lock (FR52)

Single milestone. No re-architecture; purely an additive concurrency layer over the implemented core.

#### Milestone P1.M1 — The run lock and its contention behavior

**Task P1.M1.T1 — Lock primitive (`internal/lock` + `golang.org/x/sys`)**
The location resolver, `flock`-based acquire/release, and lock-file contents read/write. Self-contained and unit-testable against temp files.
- Add `golang.org/x/sys` to `go.mod`; use `unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)` (auto-released when the fd/process closes — no stale-lock reaping).
- Resolve `<hash>.lock` per §18.5: `XDG_RUNTIME_DIR/stagecoach/locks/` → else `XDG_CACHE_HOME/stagecoach/locks/` → else `~/.cache/stagecoach/locks/`; `<hash>` = `sha256` hex of the repo's canonical absolute path (resolve symlinks to a stable path; two terminals in one repo must hash identically).
- Lock-file contents writer/reader: `pid`, `hostname`, `repo`, start `timestamp`, and `snapshot=<frozen-tree-sha>` (the snapshot field is written *later*, after the holder freezes — expose a `SetSnapshot`/update path). Diagnostic fields only; never used for reaping.
- *Docs (Mode A):* `docs/configuration.md` (lock-file location resolution); `docs/how-it-works.md` concurrency subsection (§18.5 spec — two-stage defense, per-host limit, never-in-repo rationale).
- *Tests:* real `flock` contention in temp dirs; location resolution across the three env states; contents round-trip; auto-release across a forked/exited holder.

**Task P1.M1.T2 — Contention behavior, `Busy` exit code, and wiring (both commit paths)**
Depends on P1.M1.T1. The no-op fast path, the busy refusal, the new exit code, and acquisition at the top of `runDefault` and `runDecompose` (read-only subcommands bypass).
- Add `Busy` to `internal/exitcode`, distinct from `0/1/2/3/124` (recommend `5`; confirm in-task it conflicts with no existing code and update the §15.4 comment block). Wire through `ExitError` so `For()` returns it.
- **No-op fast path:** on contention, if the holder published `snapshot=` and the contending run's own `git write-tree` (index-read-only, taken without the lock) is byte-identical to it → exit 0, "nothing to do — an in-progress run already covers your staged changes."
- **Busy refusal:** otherwise read holder `pid`/`hostname`/`repo`/lock-path and exit `Busy` with the §18.5 message. Never block; never force-break.
- **Acquire** at the top of `runDefault` (before the first `WriteTree`) and `runDecompose` (before `FreezeWorkingTree`/`T_start`). **Update `snapshot=`** after the single-path `WriteTree` and after the decompose `T_start` freeze. Bypass entirely for read-only subcommands (`config`, `providers`, `models`, `integrate list`/`status`, `hook status`, `--version`, `--help`).
- *Docs (Mode A):* `docs/cli.md` — `Busy` exit code in the §15.4 reference; the two contention behaviors.
- *Tests:* e2e (`internal/e2e`) — a held lock + a contending run exits `Busy` with the holder named; the accidental-double-run (nothing new staged) exits 0; the lock is absent after the holder exits (no stale lock); `--dry-run` and read-only subcommands never acquire.

**Task P1.M1.T3 — Sync changeset-level documentation (Mode B)**
Depends on P1.M1.T1 and P1.M1.T2. The cross-cutting safety claim.
- `README.md` — add the race-free / safe-to-double-invoke safety property to the safety section, alongside the existing snapshot/atomic-commit pitch. Keep it consistent with the per-host limit (don't overclaim shared-filesystem safety). Verify no stale concurrency claims survive.

---

## 5. Acceptance

- Two concurrent `stagecoach` invocations on one repo: the loser exits `Busy` naming the holder; the winner commits normally.
- An accidental double-run with nothing newly staged exits 0 ("nothing to do").
- No lock file remains after any exit path (incl. `SIGKILL`) — `flock` auto-release verified.
- Read-only subcommands and `--dry-run` never touch the lock.
- The §13.5 CAS is unchanged and still independently covers shared-filesystem races.
- `docs/cli.md`, `docs/how-it-works.md`, `docs/configuration.md`, and `README.md` reflect the feature with no stale claims.
