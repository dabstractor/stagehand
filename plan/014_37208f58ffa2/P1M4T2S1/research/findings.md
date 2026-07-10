# Findings — P1.M4.T2.S1 (Docs sync for orphaned-run lock reclamation)

Mode B documentation task. The FR-K1–K7 reclamation machinery LANDED in P1.M1–P1.M3, but the
**Mode A riding-docs were never applied** — confirmed by grep: `docs/` + `README.md` contain ZERO
mentions of `watchdog`, `SIGHUP`, `orphan`, `lock status`, `no_parent_watchdog`. So this task must
add ALL of the user-facing reclamation docs (not just a coherence pass). Pure docs — no source.

## 0. Current docs state (the gap) — confirmed by grep

- `grep -rniE "SIGINT|SIGTERM|signal" docs/ README.md` → docs/how-it-works.md says "the signal path
  releases the file" generically; **NO signal is enumerated**, and **NO "SIGINT/SIGTERM only" claim
  exists**. README FAQ never mentions signals. So the stale-reference check (contract d) is a
  verification no-op: nothing claims "SIGINT/SIGTERM only"; we just ADD the SIGHUP content.
- `grep -rniE "watchdog|orphan|SIGHUP|lock status|no_parent_watchdog" docs/ README.md` → ZERO hits.
  The entire reclamation feature set is undocumented in the human-readable docs.
- The BINARY's own bootstrap config template (`internal/config/bootstrap.go`, a PRODUCTION file — out
  of scope) ALREADY documents `STAGECOACH_NO_PARENT_WATCHDOG=1` and `# no_parent_watchdog = false`
  correctly (P1.M2.T1.S1). So `config init`'s written template is already right — do NOT touch it.

## 1. The exact user-facing behavior (from the LANDED code — match these strings verbatim)

### `stagecoach lock status` output — `internal/cmd/lock.go` `runLockStatus`
No lock held → prints to **stdout** and exits **0** (a read that found nothing is success):
```
no run lock for <repoDir>
```
With a lock → prints to **stdout**, exits **0** always (even when dead/orphaned — the USER decides):
```
Lock: <path>
  pid:       <pid>
  hostname:  <hostname>
  repo:      <repo>
  timestamp: <timestamp>
  snapshot:  <sha>          # ONLY printed if contents.Snapshot != ""
  alive:     <bool>
  orphaned:  true (holder reparented — launcher has exited)   # Unix, appearsOrphaned==true
  orphaned:  false                                           # alive && not orphaned (Windows always lands here)
  orphaned:  unknown (holder is dead)                        # holder process is dead
```
NOTE the em-dash `—` in "reparented — launcher". Read-only: never acquires the flock, never breaks a
lock (FR52 preserved). Works outside a git repo (the `lock` group's no-op PersistentPreRunE skips
config.Load). Registered as `lock` command group with leaf `status` — `stagecoach lock` alone prints help.

### Busy (contention) message — `internal/cmd/default_action.go` `handleLockContention`
Exit **5 (Busy)**, to stderr, lock path on its OWN line (FR-K5):
```
stagecoach: another stagecoach run is already in progress on <repo> (pid <N> on <host>).
Your newly-staged changes will remain staged — re-run stagecoach after it finishes.

Lock: <path>
```
Plus, ONLY when the holder appears orphaned (FR-K4's test), an extra hint line:
```
The holder's launcher appears to have exited — it may be orphaned and holding this lock uselessly. You may safely `kill <N>` or `rm <path>` to clear it. See `stagecoach lock status`.
```

### Signal handling — `internal/signal/signal_unix.go` (caught) + `signal_windows.go`
Caught set extended {os.Interrupt, SIGTERM} → {os.Interrupt, SIGTERM, **SIGHUP**} (Unix). Windows
OMITS SIGHUP. All three route through the SAME rescue path (cancel ctx → rescue if snapshot armed →
`OnRescueExit`/`ReleaseCurrent` removes the lock FILE → exit). `RestoreDefault` disarms all three for
the update-ref window. SIGHUP = terminal-hangup complement to the watchdog's detach case.

### Parent-death watchdog — `internal/watchdog/watchdog.go` + `default_action.go:92`
Armed after `lock.Acquire` (default_action.go:92) **unless** `cfg.NoParentWatchdog`. Records parent pid
at Arm time; polls `os.Getppid()` ~1s (Unix) + Linux best-effort `prctl(PR_SET_PDEATHSIG)`. On a
parent-pid CHANGE (reparenting — NOT `getppid()==1`) → `signal.Trigger(SIGTERM)` → the SAME rescue +
lock-release exit path. Self-termination only; FR52 "never force-break" preserved unchanged. Windows
no-op (FR-K7). One arming covers both single-commit + decompose paths.

### Opt-out (FR-K6) — three surfaces, NO flag
| Layer | Name | Notes |
|---|---|---|
| Env | `STAGECOACH_NO_PARENT_WATCHDOG` | presence-semantic; DIRECT set so `=false` is an escape hatch (mirrors `STAGECOACH_NO_VERIFY`). All-caps prefix. |
| Git config | `stagecoach.noParentWatchdog` | camelCase; bool via `git config --bool`. |
| Config file | `[generation] no_parent_watchdog` | snake_case under the `[generation]` table; only-true-propagates (mirrors `no_verify`/`push`); `= false` is a no-op. |
Default **OFF** (watchdog runs by default). **SIGHUP + `lock status` are independent of this flag and always on.**

## 2. Exit codes — NO new code; do NOT add 129/143

`docs/cli.md` "Exit codes" table already lists `3 = Rescue`. SIGHUP/watchdog route to the **same
rescue path** (exit 3 when a snapshot is armed). Do NOT add signal-specific 129/143 rows — the existing
`3 (Rescue)` row is correct and sufficient. (Pre-snapshot codes are unreachable in the common case and
would only confuse readers; the PRD §15.4 table does not enumerate them either.)

## 3. PRD wording to match verbatim (anchors)

- **§9.27 FR-K1–K7** — PRD.md:558–570 (the authoritative feature descriptions).
- **§15.3 `lock status`** — PRD.md:1539: "*`stagecoach lock status`* — Read-only diagnostic for this
  repo's run lock (§9.27, FR-K4): prints the lock path, the holder's pid/hostname/repo/timestamp/snapshot,
  whether the holder is alive, and (Unix) whether it appears orphaned (reparented). With no lock held,
  prints 'no run lock for `<repo>`'. Changes nothing; the user decides whether to kill/rm. Never
  auto-breaks (FR52)."
- **§16.2 git-config example** — PRD.md:1709: `noParentWatchdog = false   # v2.7 (§9.27, FR-K6): opt
  out of the parent-death lock watchdog`
- **§16.3 Git-config keys** — PRD.md:1699–1709 (the `[stagecoach]` ini block).
- **§18.4 Signal handling** — PRD.md:2009–2017 (now lists SIGINT, SIGTERM, SIGHUP; SIGHUP paragraph).
- **§18.5 Concurrency/lock** — PRD.md:2019–2047 (incl. the "Orphaned-but-alive" paragraph at 2039 and
  the contention-message orphan hint at 2047: "(If the holder's launcher has exited — e.g. you closed
  lazygit — it is orphaned and holding this lock uselessly; `stagecoach lock status` (FR-K4) confirms,
  then `kill <N>` or `rm <path>` to clear it.)").
- **§21.5 README structure** — PRD.md:2147–2158 (10-section marketing surface; FAQ is item 10 — the
  natural home for the lazygit/IDE self-exit note; no dedicated "safety" section).

## 4. Docs insertion points (exact headings + line numbers, verified)

### README.md
- **FAQ** (the only "safety/concurrency" surface): heading `### Will it corrupt my repo?` and the
  `**Safe to run twice.**` paragraph (~line 339) describe the per-repo run lock. ADD: a sentence that
  an orphaned holder (launcher closed — lazygit TUI / IDE / detaching terminal) **self-exits** via the
  parent-death watchdog (FR-K1), and that `stagecoach lock status` (FR-K4) shows the holder's
  path/liveness so you can decide to `kill`/`rm`. Mirror §18.5's "closed without killing it" phrasing.
- **Full CLI + config reference** area (~the `## Full CLI and config reference` heading): the list of
  subcommands/commands shown there can gain a one-line `lock status` mention so users DISCOVER it.
  (Do NOT re-document the full surface — point at docs/cli.md.)

### docs/how-it-works.md — `### Per-repo run lock (FR52)` (lines 166–180)
This is THE architecture-overview lock section (the delta_prd Mode A target). After the
"**Auto-release + file reaping.**" paragraph (line 179), ADD a new paragraph(s):
- "**Orphaned-but-alive — the launcher-closed case (FR-K1–K7).**" — mirror §18.5:2039 wording. Cover:
  the third state (launcher closed without killing the child → reparented to init, pid alive, reaping
  never fires), the parent-death watchdog (self-termination, parent-pid CHANGE detection not
  getppid()==1, Linux prctl fast path + ~1s poll; Unix-only), SIGHUP joining {SIGINT,SIGTERM} (terminal
  hangup → rescue + lock release; Windows omits), `lock status` (read-only; user kill/rm), the
  `no_parent_watchdog` opt-out for intentional detach (nohup/setsid/systemd-run), and FR52 preserved.
- ADD a cross-ref sentence to `cli.md#lock-status` and `configuration.md` for the opt-out keys.

### docs/cli.md
- NEW subcommand entry `### lock status` — INSERT after `### models [<provider>]` (ends ~line 371) and
  BEFORE `## Exit codes` (line 373). Mirror the format of `### hook status` (line 99) / `### providers
  list` (line 143): a sentence + the output block (from §1 above) + the no-lock case + exit 0 + "never
  auto-breaks (FR52)". Add a `stagecoach lock status` example fenced block.
- **Exit codes** table (373–381): NO CHANGE (3=Rescue covers the rescue path). Add a sentence under it
  that SIGHUP/parent-death route to the rescue path (exit 3 when snapshot armed) — do NOT add 129/143.
- **Flag ↔ env ↔ git-config map** (388–): ADD a row for the opt-out:
  `— (no flag) | STAGECOACH_NO_PARENT_WATCHDOG | stagecoach.noParentWatchdog` (mirrors how no-flag rows
  like `--max-commits` are shown; also note `[generation].no_parent_watchdog` in config).

### docs/configuration.md
- **Environment variables** table (169–202): ADD a `STAGECOACH_NO_PARENT_WATCHDOG` row (mirror
  `STAGECOACH_MULTI_TURN_FALLBACK`'s "(no flag)" style): "Opt out of the parent-death lock watchdog
  (§9.27 FR-K6); presence-semantic — `=1`/true disables; `=false` is an explicit escape hatch. SIGHUP
  handling and `lock status` are unaffected (always on)." Example `STAGECOACH_NO_PARENT_WATCHDOG=1 stagecoach`.
- **Git-config keys** table (203–231): ADD `stagecoach.noParentWatchdog` row (mirror `stagecoach.push`):
  bool via `git config --bool`; "Opt out of the parent-death lock watchdog (§9.27 FR-K6). Default false
  (watchdog runs by default)."
- **Built-in defaults** table (124–124+): ADD `no_parent_watchdog | false | config.Defaults() (§9.27
  FR-K6 — parent-death watchdog runs by default)` near the `no_verify` row.
- **File format `[generation]` section** (lines 104–119, the commented keys): ADD a commented
  `# no_parent_watchdog    = false  # opt out of the parent-death lock watchdog — set true if you launch
  via nohup/setsid/systemd-run (§9.27 FR-K6)` line (mirror the bootstrap template's wording).
- **Lock file location** (269–279): ADD a cross-ref — "To inspect the current repo's lock holder (path,
  pid/host, liveness, orphan status), run `stagecoach lock status` (FR-K4); see [CLI reference](cli.md#lock-status)."

### docs/README.md (light touch)
- **Capability index** (after the v2.1 list): ADD a bullet "**Concurrency & lock reclamation** →
  how-it-works.md#per-repo-run-lock-fr52 · cli.md#lock-status · configuration.md (no_parent_watchdog)".
- The "How Stagecoach works" Documentation-index row description can gain "lock reclamation (FR-K1–K7)".

## 5. Validation tooling (verified available)
- `.markdownlint.json` → `MD013` (line length) OFF, `MD033` (inline HTML) OFF, `MD060` OFF. Long lines
  are fine (existing docs are line-wrap-free). `npx` + `node` are installed → run
  `npx markdownlint-cli2 'README.md' 'docs/**/*.md'` (or `npx markdownlint-cli README.md docs/*.md`).
- No `make docs`/`make markdownlint` target — invoke markdownlint directly.
- Smoke test (delta_prd DOC.T1.S1): `make build && ./bin/stagecoach lock status` (expect
  "no run lock for <cwd>", exit 0); `./bin/stagecoach lock --help` shows the group.

## 6. Scope fence (touch ONLY these; everything else READ-ONLY)
TOUCH: `README.md`, `docs/how-it-works.md`, `docs/cli.md`, `docs/configuration.md`, `docs/README.md`.
DO NOT TOUCH: `PRD.md`, `plan/**`, `tasks.json`, `prd_snapshot.md`, any `internal/*`/`cmd/*` source,
`providers/*.toml`, `FUTURE_SPEC.md`, `.markdownlint.json`, `Makefile`, `go.mod`, the bootstrap config
template (already correct).
