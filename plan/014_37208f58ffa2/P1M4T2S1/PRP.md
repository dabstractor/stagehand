name: "P1.M4.T2.S1 — Update README.md + docs/ for orphaned-run lock reclamation (FR-K1–K7, §9.27)"
description: >
  THE documentation-sync (Mode B) task for the §9.27 orphaned-run lock reclamation feature set that
  LANDED in P1.M1–P1.M3. The Mode A "riding docs" were NEVER applied (grep confirms zero mentions of
  watchdog/SIGHUP/orphan/lock status/no_parent_watchdog in README.md + docs/), so this task ADDS the
  entire user-facing doc surface — not merely a coherence pass. It edits ONLY docs: README.md,
  docs/how-it-works.md, docs/cli.md, docs/configuration.md, docs/README.md. Concretely: (1) add a
  `### lock status` subcommand entry to docs/cli.md (FR-K4) + an opt-out row in the Flag↔env↔git-config
  map; (2) extend docs/how-it-works.md "Per-repo run lock (FR52)" with an "Orphaned-but-alive"
  paragraph (watchdog FR-K1/K2, SIGHUP FR-K3, lock status FR-K4, opt-out FR-K6, Windows no-op FR-K7);
  (3) add STAGECOACH_NO_PARENT_WATCHDOG env row + stagecoach.noParentWatchdog git-config row +
  no_parent_watchdog default/file-key to docs/configuration.md; (4) add a watchdog self-exit + lock
  status sentence to the README FAQ run-lock entry (the lazygit/IDE case) and a discoverable one-liner
  in the CLI reference area; (5) add a capability-index bullet to docs/README.md. Wording must match
  the verbatim PRD §9.27/§15.3/§18.4/§18.5 text and the EXACT binary output strings. Verify (not fix)
  that no doc claims "SIGINT/SIGTERM only" (none does). NO source code, NO PRD/tasks/plan edits. The
  binary's own bootstrap config template (internal/config/bootstrap.go) already documents the env var +
  file key correctly — do NOT touch it. Validates via markdownlint (MD013/MD033/MD060 off) +
  `make build && ./bin/stagecoach lock status` smoke test + grep guards.

---

## Goal

**Feature Goal**: Make the shipped human-readable documentation (README.md + docs/) consistent with
the FR-K1–K7 orphaned-run lock reclamation feature set shipped in P1.M1–P1.M3, so a user reading the
docs can discover (a) the `stagecoach lock status` diagnostic subcommand, (b) the parent-death
watchdog's self-exit behavior (the lazygit/IDE/detaching-terminal "closed without killing it" case),
(c) SIGHUP joining the caught signals, and (d) the `no_parent_watchdog` opt-out (env / git-config /
config-file). The docs must match the PRD §9.27/§15.3/§16.2/§16.3/§18.4/§18.5 wording AND the exact
binary output strings.

**Deliverable**: Edits to FIVE docs files only — `README.md`, `docs/how-it-works.md`, `docs/cli.md`,
`docs/configuration.md`, `docs/README.md`. No new files. The deltas are: a new `### lock status`
subcommand section + an opt-out row in docs/cli.md; an "Orphaned-but-alive (FR-K1–K7)" paragraph in
docs/how-it-works.md; three additions (env row, git-config row, default/file key) in
docs/configuration.md; a watchdog+lock-status sentence in the README FAQ + a CLI-reference one-liner;
a capability-index bullet in docs/README.md.

**Success Definition**:
- `grep -rniE "watchdog|SIGHUP|orphan|lock status|no_parent_watchdog" README.md docs/` returns hits in
  all FIVE files (the feature is now documented across the human-readable surface).
- `grep -rniE "SIGINT|SIGTERM" README.md docs/` returns NO line that claims "SIGINT/SIGTERM only" (it
  never did — this is the verification, not a fix); the new SIGHUP content is present.
- `npx markdownlint-cli2 'README.md' 'docs/**/*.md'` is CLEAN (the config disables MD013/MD033/MD060;
  long lines and `<details>`/`> [!NOTE]` blocks are already used throughout — match the existing style).
- `make build && ./bin/stagecoach lock status` prints `no run lock for <cwd>` and exits 0 (smoke test
  from the delta_prd DOC.T1.S1); `./bin/stagecoach lock --help` shows the `lock` command group.
- `git status --porcelain` shows ONLY the five docs files (scope guard). NO source, NO PRD, NO
  plan/tasks edits.

## User Persona (if applicable)

**Target User**: A Stagecoach user who launches it from lazygit (`<c-a>`), an IDE, or a detaching
terminal — the primary launch path (§9.21) — and the maintainer who supports them.
**Use Case**: The user closed lazygit mid-run, then a later `stagecoach` invocation printed "another
stagecoach run is already in progress … Lock: <path>" (exit 5/Busy). They need to (a) understand WHY an
orphaned holder self-exits, (b) discover `stagecoach lock status` to inspect the holder, and (c) know
the `no_parent_watchdog` escape hatch if they launch via `nohup`/`setsid`/`systemd-run`.
**User Journey**: README FAQ ("Will it corrupt my repo?" / "Safe to run twice.") → links into
docs/how-it-works.md#per-repo-run-lock-fr52 → docs/cli.md#lock-status → docs/configuration.md (opt-out).
**Pain Points Addressed**: "the lock stays forever" report; a blocked user with no diagnostic; users of
intentional-detach workflows tripping the watchdog.

## Why

- **FR-K1–K7 traceability to docs**: the reclamation machinery shipped (P1.M1–P1.M3 = "Complete") but
  the Mode A riding-docs were skipped; this item closes the doc gap so the feature is discoverable and
  the behavior is explained (not magic).
- **The FAQ is the support surface**: the README FAQ already answers "Will it corrupt my repo?" and
  describes the run lock ("Safe to run twice") — that is exactly where the orphaned-holder self-exit
  behavior belongs, and where a confused user lands first.
- **Coherence with PRD §21.5**: the README marketing surface (item 10 = FAQ) must not contradict the
  shipped binary. The delta_prd DOC.T1.S1 explicitly tasked this final coherence pass.

## What

Doc-only edits across five files. Every addition must match (a) the verbatim PRD §9.27/§15.3/§18.4/§18.5
wording and (b) the EXACT binary output strings captured in research/findings.md §1.

### Success Criteria
- [ ] **docs/cli.md** has a new `### lock status` subcommand section (parallel to `### hook status` /
      `### providers list`) showing the exact output block, the no-lock case, exit 0, and "never
      auto-breaks (FR52)"; the Flag↔env↔git-config map has a new opt-out row
      (`— (no flag) | STAGECOACH_NO_PARENT_WATCHDOG | stagecoach.noParentWatchdog`); the Exit-codes
      table is UNCHANGED (do NOT add 129/143).
- [ ] **docs/how-it-works.md** "Per-repo run lock (FR52)" subsection has a new "**Orphaned-but-alive —
      the launcher-closed case (FR-K1–K7).**" paragraph (after "Auto-release + file reaping.") covering
      the watchdog, SIGHUP, `lock status`, the opt-out, Windows no-op, and FR52-preserved.
- [ ] **docs/configuration.md** has: a `STAGECOACH_NO_PARENT_WATCHDOG` env-var row; a
      `stagecoach.noParentWatchdog` git-config row; a `no_parent_watchdog` default (`false`) row; a
      commented `# no_parent_watchdog = false` key in the `[generation]` file-format block; a `lock
      status` cross-ref in "Lock file location".
- [ ] **README.md** FAQ run-lock entry ("Safe to run twice.") gains a sentence that an orphaned holder
      (launcher closed — lazygit/IDE/detaching terminal) self-exits via the parent-death watchdog and
      that `stagecoach lock status` inspects the holder; the "Full CLI and config reference" area gains
      a one-line `lock status` mention so users DISCOVER it.
- [ ] **docs/README.md** capability index gains a "Concurrency & lock reclamation" bullet linking the
      three anchors.
- [ ] NO doc claims "SIGINT/SIGTERM only" (verified — none does); SIGHUP is named where signal handling
      is discussed.
- [ ] markdownlint clean on all five files; `./bin/stagecoach lock status` smoke test passes; scope guard
      shows only the five docs files.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact verbatim output strings (from internal/cmd/lock.go + default_action.go), the verbatim
PRD wording to mirror (§9.27/§15.3/§18.4/§18.5 with PRD.md line numbers), the exact docs insertion points
(file + heading + line number), the exact opt-out surface names (env/git-config/file), the markdownlint
config (which rules are off), the validation commands, and the scope fence.

### Documentation & References

```yaml
# MUST READ — the codebase-specific findings for THIS item (exact output strings, opt-out surface,
#              docs insertion points with line numbers, PRD anchors, validation tooling).
- docfile: plan/014_37208f58ffa2/P1M4T2S1/research/findings.md
  why: "§0 the doc gap (grep proves zero mentions today); §1 the EXACT user-facing strings to match
        verbatim (lock status output, Busy message, signal set, watchdog, opt-out table); §2 do NOT add
        129/143; §3 PRD anchors with line numbers; §4 docs insertion points (file+heading+line); §5
        markdownlint config + smoke test; §6 scope fence."

# MUST READ — the verbatim PRD wording to mirror (the docs are DERIVED from the PRD).
- docfile: PRD.md
  section: "§9.27 (line 558) FR-K1–K7; §15.3 (line 1539) lock status; §16.2 (line 1709) noParentWatchdog;
            §16.3 (line 1699) git-config block; §18.4 (line 2009) signal handling; §18.5 (line 2019,
            esp. the Orphaned-but-alive paragraph at 2039 + contention hint at 2047); §21.5 (2147) README
            structure."
  why: "The docs must match this wording. Copy the FR-K1–K7 phrasing, the 'closed without killing it'
        framing, the 'self-termination, never contender-side force-breaking' guarantee, and the
        parent-pid-CHANGE-not-getppid()==1 detection language verbatim (lightly trimmed for prose)."

# MUST READ — the lock status output format (match these strings EXACTLY, incl. the em-dash).
- file: internal/cmd/lock.go
  why: "runLockStatus prints: no-lock → 'no run lock for <repoDir>' (exit 0); with-lock → 'Lock: <path>',
        '  pid:       <pid>', '  hostname:  <host>', '  repo:      <repo>', '  timestamp: <ts>',
        '  snapshot:  <sha>' (ONLY if set), '  alive:     <bool>', then '  orphaned:  true (holder
        reparented — launcher has exited)' / '  orphaned:  false' / '  orphaned:  unknown (holder is
        dead)'. Exit 0 always. The column alignment (the extra spaces) and the em-dash '—' matter."
  gotcha: "Do NOT document 129/143 exit codes — `lock status` always exits 0. The output goes to STDOUT."

# MUST READ — the Busy (contention) message format (lock path on its own line + the orphan hint).
- file: internal/cmd/default_action.go
  why: "handleLockContention (exit 5) prints to STDERR: 'stagecoach: another stagecoach run is already
        in progress on <repo> (pid <N> on <host>).\\nYour newly-staged changes will remain staged —
        re-run stagecoach after it finishes.\\n\\nLock: <path>' + (if orphaned) 'The holder's launcher
        appears to have exited — it may be orphaned and holding this lock uselessly. You may safely
        `kill <N>` or `rm <path>` to clear it. See `stagecoach lock status`.' Mirror this in how-it-works.md."

# MUST READ — the signal set (SIGHUP joins SIGINT/SIGTERM on Unix; Windows omits it).
- file: internal/signal/signal_unix.go
  why: "caughtSignals() returns {os.Interrupt, SIGTERM, SIGHUP}. signal_windows.go returns {os.Interrupt,
        SIGTERM} (no SIGHUP). All three share RestoreDefault for the update-ref window. State this in
        how-it-works.md exactly (FR-K3)."
- file: internal/signal/signal_windows.go
  why: "Windows omits SIGHUP — cite when documenting the Windows no-op (FR-K7)."

# MUST READ — the opt-out surface (three layers, NO flag) + the arming gate.
- file: internal/config/config.go
  why: "Field NoParentWatchdog bool toml:\"no_parent_watchdog\" (line 151); default false (line 224). The
        comment (144–151) names all three surfaces: STAGECOACH_NO_PARENT_WATCHDOG /
        stagecoach.noParentWatchdog / [generation].no_parent_watchdog. NO CLI flag."
- file: internal/config/load.go
  why: "Line 329: os.LookupEnv(\"STAGECOACH_NO_PARENT_WATCHDOG\") — presence-semantic, DIRECT set (can be
        false = escape hatch, mirrors STAGECOACH_NO_VERIFY). All-caps env prefix."
- file: internal/config/git.go
  why: "Line 191: c.NoParentWatchdog = v from git key stagecoach.noParentWatchdog (camelCase, --bool)."
- file: internal/cmd/default_action.go
  why: "Line 92: the arming gate — `if !cfg.NoParentWatchdog { watchdog.Arm(ctx, 1*time.Second) }` —
        AFTER lock.Acquire, so the watchdog runs BY DEFAULT. One arming covers single-commit + decompose."

# CONTEXT — the binary's own config template ALREADY documents the env var + file key (do NOT touch it,
#           but you may quote its wording — it is the canonical phrasing the docs should match).
- file: internal/config/bootstrap.go
  why: "Line 256: 'STAGECOACH_NO_PARENT_WATCHDOG=1   # opt out of the parent-death lock watchdog (§9.27
        FR-K6)'. Line 307: '# no_parent_watchdog    = false  # opt out of the parent-death lock watchdog
        — set true if you launch via nohup/setsid/systemd-run (§9.27 FR-K6)'. This file is PRODUCTION
        (out of scope) — its wording is the source of truth to mirror in docs/configuration.md."

# CONTEXT — the existing docs structure to mirror (format/anchors).
- file: docs/cli.md
  why: "Subcommand entries follow `### <name>` + prose + fenced output/example (see `### hook status`
        line 99, `### providers list` line 143, `### models` line 351). The Flag↔env↔git-config map
        (line 388) is a 3-col table. The Exit-codes table (373) lists 3=Rescue — leave UNCHANGED."
- file: docs/how-it-works.md
  why: "`### Per-repo run lock (FR52)` (line 166) is the lock subsection; the 'Auto-release + file
        reaping.' paragraph ends at line 179 — INSERT the new paragraph after it."
- file: docs/configuration.md
  why: "Env-vars table (169), Git-config keys table (203), Built-in defaults table (124), the commented
        `[generation]` file-format keys (104–119), 'Lock file location' (269). Mirror row formats exactly."
- file: docs/README.md
  why: "The Capability index (after the v2.1 list) and the Documentation index table — add a bullet/row."
- file: README.md
  why: "The FAQ `### Will it corrupt my repo?` + the `**Safe to run twice.**` paragraph (~line 339) and
        `## Full CLI and config reference`. Match the `> [!NOTE]` / `**bold-lead.**` prose style."

# CONTEXT — markdownlint config (rules off) + validation.
- file: .markdownlint.json
  why: "{ default:true, MD013:false, MD033:false, MD060:false } — line length, inline HTML, and
        'no punctuation at heading end' are DISABLED. The docs use long lines, `<details>`, and
        `> [!NOTE]` freely; match that. No `make docs` target — invoke `npx markdownlint-cli2` directly."
```

### Current Codebase tree (relevant slice)

```bash
README.md                         # EDIT — FAQ run-lock entry + CLI-reference one-liner
docs/README.md                    # EDIT — capability-index bullet (+ Documentation-index row tweak)
docs/how-it-works.md              # EDIT — "Per-repo run lock (FR52)": new "Orphaned-but-alive" paragraph
docs/cli.md                       # EDIT — new `### lock status` section + opt-out row in flag map
docs/configuration.md             # EDIT — env row + git-config row + default + file key + lock-status xref
# READ-ONLY references (do NOT edit):
PRD.md                            # READ-ONLY — §9.27/§15.3/§16.2/§16.3/§18.4/§18.5/§21.5 (the wording source)
internal/cmd/lock.go              # READ-ONLY — the EXACT lock status output strings (mirror verbatim)
internal/cmd/default_action.go    # READ-ONLY — the EXACT Busy message + the arming gate (line 92)
internal/signal/signal_unix.go    # READ-ONLY — caught set {SIGINT,SIGTERM,SIGHUP}
internal/signal/signal_windows.go # READ-ONLY — Windows omits SIGHUP (FR-K7)
internal/config/config.go         # READ-ONLY — NoParentWatchdog field + the 3-surface comment
internal/config/load.go           # READ-ONLY — STAGECOACH_NO_PARENT_WATCHDOG env (presence/DIRECT)
internal/config/git.go            # READ-ONLY — stagecoach.noParentWatchdog git key
internal/config/bootstrap.go      # READ-ONLY — the binary's template ALREADY documents it (mirror wording)
.markdownlint.json                # READ-ONLY — MD013/MD033/MD060 off
Makefile                          # READ-ONLY — `make build` (no docs target)
FUTURE_SPEC.md                    # READ-ONLY — leave alone (deferred-features list; no SIGINT/SIGTERM claim)
```

### Desired Codebase tree with files to be added/edited

```bash
# FIVE docs files edited (NO new files). See "Implementation Tasks" for exact insertions.
README.md              # +FAQ sentence(s) +CLI-reference one-liner
docs/README.md         # +1 capability-index bullet
docs/how-it-works.md   # +1 "Orphaned-but-alive (FR-K1–K7)" paragraph in the lock subsection
docs/cli.md            # +`### lock status` section +1 row in the Flag↔env↔git-config map
docs/configuration.md  # +1 env row +1 git-config row +1 default row +1 commented [generation] key +1 xref
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (Mode A docs were skipped): grep PROVES the docs have ZERO mentions of watchdog/SIGHUP/
     orphan/lock status/no_parent_watchdog today. This task ADDS the full surface — do not assume a
     "coherence pass" is enough. Run: grep -rniE "watchdog|SIGHUP|orphan|lock status|no_parent_watchdog" README.md docs/ -->

<!-- CRITICAL (match the binary strings VERBATIM): the lock status output has specific column alignment
     ("  pid:       <pid>" — note the gap so the values column-align) and an em-dash "—" in "reparented
     — launcher has exited". Copy the exact bytes from internal/cmd/lock.go. A docs example that drifts
     from the binary breaks the docs/README.md "binary is authoritative" promise. -->

<!-- CRITICAL (do NOT add 129/143 exit codes): docs/cli.md's Exit-codes table lists 3=Rescue. SIGHUP and
     parent-death route to the SAME rescue path (exit 3 when a snapshot is armed). Adding signal-specific
     129/143 rows would CONTRADICT the shipped behavior in the common case and confuse readers. Leave the
     table unchanged; at most add one sentence that SIGHUP/parent-death take the rescue path. -->

<!-- CRITICAL (env var is ALL-CAPS, presence-semantic): the PRD §9.27 FR-K6 text wrote "stagecoach_NO_PARENT_WATCHDOG"
     (lowercase prefix) — that is a PRD typo. The CODE (load.go:329) and the binary template (bootstrap.go:256)
     use "STAGECOACH_NO_PARENT_WATCHDOG" (all-caps prefix, matching STAGECOACH_NO_VERIFY/STAGECOACH_PUSH).
     Use the all-caps form in the docs. It is presence-semantic with a DIRECT set, so "=1"/"=true" disables
     the watchdog and "=false" is an explicit escape hatch (document both). -->

<!-- CRITICAL (config-file table is [generation]): the file key is `no_parent_watchdog` (snake_case) under
     the `[generation]` table — NOT a top-level key and NOT `[stagecoach]`. (git-config uses camelCase
     `stagecoach.noParentWatchdog`; the config FILE uses snake_case `no_parent_watchdog`. Do not mix them up.)
     It is only-true-propagates (mirrors no_verify/push): `= false` is a no-op; say so. -->

<!-- CRITICAL (NO CLI flag): FR-K6 lists ONLY env + git-config + config-file. Do NOT invent a
     `--no-parent-watchdog` flag. In the Flag↔env↔git-config map, the Flag column is "— (no flag)". -->

<!-- GOTCHA (SIGHUP is Unix-only): caught set is {SIGINT,SIGTERM,SIGHUP} on Unix; Windows OMITS SIGHUP
     (signal_windows.go). The watchdog is also a Unix-only feature (Windows no-op, FR-K7). State both
     when documenting — do not imply SIGHUP works on Windows. -->

<!-- GOTCHA (markdownlint rules): MD013 (line length) is OFF — the existing docs are single-long-line
     paragraphs (no hard wrapping). Match that: write prose as long lines, do NOT hard-wrap at 80 cols
     (wrapping would be inconsistent and a reviewer red flag). MD033 (inline HTML) and MD060 are also
     off, so `> [!NOTE]`, `<details>`, and bold-lead paragraphs are fine — mirror the existing style. -->

<!-- GOTCHA (link anchors): the docs cross-reference each other by anchor (e.g. cli.md#lock-status,
     configuration.md#environment-variables). New `### lock status` heading ⇒ anchor `#lock-status`.
     Verify every new internal link resolves (markdownlint does NOT check anchors — do it by grep). -->

<!-- GOTCHA (do NOT touch internal/config/bootstrap.go): the binary's config template ALREADY documents
     STAGECOACH_NO_PARENT_WATCHDOG and no_parent_watchdog (P1.M2.T1.S1). It is a PRODUCTION file. Your
     job is the human-readable docs/ files only. You may QUOTE the template's wording but not edit it. -->
```

## Implementation Blueprint

### Data models and structure
None — this is a documentation task. No code, no schemas. The "data" is the exact output strings and
config key names captured in research/findings.md §1 and the Documentation & References YAML above.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/how-it-works.md — the architecture-overview lock subsection (the delta_prd Mode A target)
  - LOCATE: `### Per-repo run lock (FR52)` (line 166). Find the paragraph beginning "**Auto-release +
    file reaping.**" (line 179) — INSERT a new paragraph immediately AFTER it.
  - ADD a paragraph titled (bold-lead, matching the section's style): "**Orphaned-but-alive — the
    launcher-closed case (FR-K1–K7).**" Mirror PRD §18.5:2039 wording. Cover, in 1–2 short paragraphs:
      * the THIRD lock state: a holder whose launcher CLOSED WITHOUT KILLING IT (closing the lazygit TUI,
        quitting an IDE, a detaching terminal) → the child is reparented to init/a subreaper and KEEPS
        RUNNING (pid alive → pid-liveness reaping never fires; SIGINT/SIGTERM never delivered) → the lock
        outlives the launcher. This is the "lock stays forever" report.
      * the fix is SELF-TERMINATION, never contender-side force-breaking: on startup stagecoach records
        its parent pid and arms a parent-death watchdog (FR-K1) that, on parent death, routes the process
        through the SAME rescue + lock-release exit path the signal handler uses (abandoning an in-flight
        commit is always safe — HEAD moves only at update-ref; the snapshot is a gc'able orphan whose SHA
        the rescue recipe prints).
      * detection is by parent-pid CHANGE (reparenting), NOT the brittle getppid()==1 test (FR-K2) —
        subreaper-safe. Linux uses prctl(PR_SET_PDEATHSIG) as a best-effort fast path plus a ~1s getppid
        poll; Darwin/other Unix poll only. Unix-only (FR-K7); on Windows flock is already a no-op and the
        §13.5 CAS is the guarantee.
      * SIGHUP joins the caught signals {SIGINT, SIGTERM, SIGHUP} (FR-K3): a terminal hangup routes
        through rescue + lock release instead of Go's default terminate-and-leave-the-file. (Windows
        omits SIGHUP.)
      * `stagecoach lock status` (FR-K4) prints the holder's path/pid/host/repo/timestamp/snapshot +
        liveness + (Unix) orphan status — READ-ONLY; the USER decides whether to kill/rm. Never auto-breaks.
      * the opt-out `no_parent_watchdog` (FR-K6; default OFF) for intentional detach (nohup/setsid/
        systemd-run). SIGHUP handling and `lock status` are independent of it and always on.
      * FR52's "never force-break" guarantee is preserved unchanged (the watchdog is the SAME process
        abandoning its own unwanted work).
  - ADD a cross-ref sentence: "See [CLI reference — lock status](cli.md#lock-status) and
    [Configuration — no_parent_watchdog](configuration.md#environment-variables)."
  - DO NOT duplicate the full lock-status output block here (that lives in cli.md) — reference it.

Task 2: EDIT docs/cli.md — the new `### lock status` subcommand section + the opt-out map row
  - SUBTASK 2a — NEW `### lock status` section:
      * INSERT after `### models [<provider>]` (ends ~line 371) and BEFORE `## Exit codes` (line 373).
      * Mirror the format of `### hook status` (line 99) / `### providers list` (line 143): a `### lock
        status` heading, a one-paragraph description, then a fenced ```text output block, then a fenced
        ```bash example.
      * DESCRIPTION (match PRD §15.3:1539 + internal/cmd/lock.go): "Read-only diagnostic for this repo's
        run lock (§9.27, FR-K4). Prints the lock path, the holder's pid/hostname/repo/timestamp/snapshot,
        whether the holder process is alive, and — on Unix — whether it appears orphaned (reparented).
        With no lock held, prints `no run lock for <repo>` and exits 0. It acquires no flock and never
        breaks/removes a lock (FR52 preserved); you decide whether to `kill <pid>` or `rm <path>`. Works
        outside a git repo."
      * OUTPUT BLOCK (copy EXACTLY from internal/cmd/lock.go — alignment + em-dash matter):
            ```text
            Lock: /home/you/.cache/stagecoach/locks/<hash>.lock
              pid:       12345
              hostname:  laptop
              repo:      /home/you/proj
              timestamp: 2026-07-10T00:00:00Z
              snapshot:  <tree-sha>          # only shown once the snapshot is armed
              alive:     true
              orphaned:  true (holder reparented — launcher has exited)
            ```
        Then list the three `orphaned:` outcomes ("true (holder reparented — launcher has exited)" /
        "false" / "unknown (holder is dead)") and the no-lock case `no run lock for <repo>` (exit 0).
        State: exit 0 in ALL cases (even dead/orphaned) — the read is the help, the action is yours.
  - SUBTASK 2b — Flag↔env↔git-config map (line 388 table): ADD a row:
        `— (no flag) | STAGECOACH_NO_PARENT_WATCHDOG | stagecoach.noParentWatchdog`
      Append a parenthetical "(also `[generation].no_parent_watchdog` in the config file)" mirroring how
      `--max-commits` notes its config-file alias. (NO flag exists — FR-K6 is env/git-config/file only.)
  - DO NOT change the Exit-codes table. Optionally add ONE sentence under it: "SIGHUP (Unix) and the
    parent-death watchdog route through the rescue path (exit 3 when a snapshot is armed) — see
    [how-it-works.md — Per-repo run lock](how-it-works.md#per-repo-run-lock-fr52)."

Task 3: EDIT docs/configuration.md — env row + git-config row + default + file key + lock-status xref
  - SUBTASK 3a — Environment variables table (line 169): ADD a row mirroring STAGECOACH_MULTI_TURN_FALLBACK's
    "(no flag)" style:
        `STAGECOACH_NO_PARENT_WATCHDOG | (no flag) | Opt out of the parent-death lock watchdog (§9.27
        FR-K6). Presence-semantic: `=1`/`true` disables it; `=false` is an explicit escape hatch. SIGHUP
        handling and `lock status` are unaffected (always on). | STAGECOACH_NO_PARENT_WATCHDOG=1 stagecoach`
  - SUBTASK 3b — Git-config keys table (line 203): ADD a row mirroring `stagecoach.push`:
        `stagecoach.noParentWatchdog | bool | git config --get --bool stagecoach.noParentWatchdog | Opt
        out of the parent-death lock watchdog (§9.27 FR-K6). Default false (watchdog runs by default).`
  - SUBTASK 3c — Built-in defaults table (line 124): ADD near the `no_verify` row:
        `no_parent_watchdog | false | config.Defaults() (§9.27 FR-K6 — parent-death watchdog runs by default)`
  - SUBTASK 3d — File format `[generation]` commented keys (lines 104–119): ADD a commented line mirroring
    the binary template (bootstrap.go:307) wording:
        `# no_parent_watchdog    = false  # opt out of the parent-death lock watchdog — set true if you launch via nohup/setsid/systemd-run (§9.27 FR-K6)`
  - SUBTASK 3e — "Lock file location" (line 269): ADD a cross-ref sentence at the end: "To inspect the
        current repo's lock holder (path, pid/host, liveness, orphan status), run `stagecoach lock status`
        (FR-K4); see [CLI reference — lock status](cli.md#lock-status)."

Task 4: EDIT README.md — FAQ run-lock entry + CLI-reference discoverability
  - SUBTASK 4a — FAQ: LOCATE `### Will it corrupt my repo?` and the `**Safe to run twice.**` paragraph
    (~line 339). ADD a sentence (or a short `> [!NOTE]`) that an orphaned holder self-exits: e.g. "If the
    launcher closed without killing stagecoach — you closed the lazygit TUI, quit your IDE, or detached the
    terminal mid-run — the orphaned run self-exits via a parent-death watchdog (FR-K1) and releases the
    lock, so it never strands. `stagecoach lock status` (FR-K4) shows the holder's path and liveness so
    you can decide whether to `kill`/`rm` yourself; it never force-breaks a live lock." Keep the §21.5
    FAQ tone (plain language). Do NOT bloat the FAQ — one tight sentence cluster.
  - SUBTASK 4b — CLI reference discoverability: in the `## Full CLI and config reference` area, add a
    one-line mention of `lock status` so users can find it (e.g. add `stagecoach lock status` to the
    example command list, or a `> [!NOTE]` pointing at docs/cli.md#lock-status). Do NOT re-document the
    surface — point at the docs.

Task 5: EDIT docs/README.md — capability index
  - ADD a bullet to the Capability index (after the v2.1 list):
        "- **Concurrency & lock reclamation** → [how-it-works.md#per-repo-run-lock-fr52](how-it-works.md#per-repo-run-lock-fr52)
         · [cli.md#lock-status](cli.md#lock-status) · [configuration.md](configuration.md#environment-variables) (no_parent_watchdog)"
  - OPTIONALLY extend the "How Stagecoach works" Documentation-index row description with "lock
    reclamation (FR-K1–K7)".

Task 6: VERIFY — markdownlint, smoke test, grep guards, scope guard
  - npx markdownlint-cli2 'README.md' 'docs/**/*.md'           # clean (MD013/MD033/MD060 off)
  - make build && ./bin/stagecoach lock status                 # "no run lock for <cwd>", exit 0
  - ./bin/stagecoach lock --help && ./bin/stagecoach --help    # lock group listed
  - grep guards (see Validation Loop Level 4)
  - git status --porcelain                                      # ONLY the five docs files
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN (subcommand entry — mirror docs/cli.md `### hook status` / `### providers list`):
     ### lock status
     <one-paragraph description, §15.3 wording>
     ```text
     <EXACT output block from internal/cmd/lock.go>
     ```
     ```bash
     stagecoach lock status
     ```
     <no-lock case + exit 0 + "never auto-breaks (FR52)">
-->

<!-- PATTERN (how-it-works paragraph — bold-lead, long-line prose, match §18.5:2039):
     **Orphaned-but-alive — the launcher-closed case (FR-K1–K7).** The two states above (dead holder,
     file reaped; live holder, never touched) miss a third that arises from stagecoach's primary launch
     path: a parent process — the lazygit TUI, an IDE, a detaching terminal — that closes without killing
     its child. ... The remedy is self-termination, never contender-side force-breaking (FR-K1): ...
     (copy/trim PRD §18.5:2039 verbatim — it is already the canonical prose).
-->

<!-- PATTERN (config rows — mirror the EXACT column format of an adjacent row; do not invent columns).
     Env table cols: Variable | Mirrors flag | Description | Example.
     Git-config table cols: Key | Type | Reads with | Description.
     Defaults table cols: Option | Default | Source.
-->

<!-- CRITICAL: every internal link anchor must match a real heading. `### lock status` → `#lock-status`.
     `### Per-repo run lock (FR52)` → `#per-repo-run-lock-fr52`. `## Environment variables` →
     `#environment-variables`. Verify with: grep -nE "^#+ " docs/*.md | grep -i <slug>. -->
```

### Integration Points

```yaml
DOC CROSS-LINKS (anchors must resolve):
  - docs/cli.md gains `#lock-status` (the new `### lock status` heading).
  - docs/how-it-works.md `#per-repo-run-lock-fr52` already exists — link INTO it from README FAQ + docs/README.md.
  - docs/configuration.md `#environment-variables` / `#git-config-keys` / `#lock-file-location` already exist.
  - New content links OUT to these; verify each resolves (markdownlint does NOT check anchors).
CONSISTENCY:
  - README FAQ + docs/how-it-works.md + docs/cli.md + docs/configuration.md must use the SAME names:
    `stagecoach lock status`, `STAGECOACH_NO_PARENT_WATCHDOG`, `stagecoach.noParentWatchdog`,
    `[generation].no_parent_watchdog`, "parent-death watchdog", "orphaned-but-alive".
  - Do NOT contradict the "binary is authoritative" promise (docs/README.md) — match the binary strings.
NO build/config/runtime integration — this is docs-only. `make build` is run only for the smoke test.
```

## Validation Loop

### Level 1: Markdown lint (Immediate Feedback)

```bash
# markdownlint on the edited files (config disables MD013 line-length, MD033 inline-HTML, MD060).
npx markdownlint-cli2 'README.md' 'docs/**/*.md'
# Expected: clean. If a finding appears, it is almost certainly a real style issue (e.g. a stray blank
# line, a list indent) — fix it. MD013/MD033/MD060 are OFF, so long lines / <details> / > [!NOTE] are fine.

# If markdownlint-cli2 is unavailable, fall back:
npx -y markdownlint-cli README.md docs/README.md docs/cli.md docs/configuration.md docs/how-it-works.md
# Expected: clean.
```

### Level 2: Anchor + wording consistency (grep-based)

```bash
# The new `### lock status` heading exists and its anchor slug is `lock-status`.
grep -nE "^### lock status" docs/cli.md
# Expected: 1 hit.

# The feature is now documented across all FIVE files.
for f in README.md docs/how-it-works.md docs/cli.md docs/configuration.md docs/README.md; do
  echo "== $f =="; grep -ciE "watchdog|SIGHUP|orphan|lock status|no_parent_watchdog|noParentWatchdog" "$f"
done
# Expected: every file ≥1 (README may only name `lock status` + `watchdog`/orphan; that is fine).

# The three opt-out surface names are spelled consistently and correctly.
grep -rn "STAGECOACH_NO_PARENT_WATCHDOG" docs/ README.md          # all-caps env (NOT stagecoach_NO_...)
grep -rn "stagecoach.noParentWatchdog" docs/ README.md            # camelCase git key
grep -rn "no_parent_watchdog" docs/ README.md                     # snake_case file key
# Expected: each ≥1 hit; NO "stagecoach_NO_PARENT_WATCHDOG" (lowercase-prefix) anywhere.
grep -rn "stagecoach_NO_PARENT_WATCHDOG" docs/ README.md && echo "FAIL: wrong env casing" || echo "OK: env casing correct"

# The exact binary output string is present (em-dash + alignment) in docs/cli.md.
grep -n "reparented — launcher has exited" docs/cli.md            # em-dash, NOT a hyphen
grep -n "no run lock for" docs/cli.md
# Expected: ≥1 hit each.

# The Exit-codes table was NOT given bogus 129/143 rows.
grep -nE "\| *129 *\||\| *143 *\|" docs/cli.md && echo "WARN: signal exit codes added — do NOT (3=Rescue covers it)" || echo "OK: no 129/143 rows"
```

### Level 3: Smoke test against the built binary (docs ↔ binary parity)

```bash
make build
./bin/stagecoach lock status            # Expect: "no run lock for <cwd>", exit 0
./bin/stagecoach lock status; echo "exit=$?"   # exit=0
./bin/stagecoach lock --help            # Expect: shows the `lock` group + `status` leaf
./bin/stagecoach --help | grep -i lock  # Expect: the `lock` command is listed
# Expected: the docs' described behavior matches the binary. If the output format in the docs drifts,
#           copy the exact bytes from the binary run into docs/cli.md.
```

### Level 4: Stale-reference + scope guards

```bash
# Guard 1: NO doc claims "SIGINT/SIGTERM only" (the contract's stale-reference check). Today none does;
#          confirm, and ensure new content NAMES SIGHUP wherever signal handling is discussed.
grep -rniE "SIGINT|SIGTERM" docs/ README.md
# Expected: any hit must NOT say "only"/"just SIGINT/SIGTERM"; SIGHUP must appear alongside in how-it-works.md.
grep -rniE "only.*SIGINT|SIGINT.*only|just.*SIGINT" docs/ README.md && echo "FAIL: stale SIGINT/SIGTERM-only claim" || echo "OK: no stale signal claim"
grep -rni "SIGHUP" docs/how-it-works.md docs/cli.md
# Expected: SIGHUP named in both.

# Guard 2: scope — ONLY the five docs files changed.
git status --porcelain
# Expected: README.md, docs/README.md, docs/cli.md, docs/configuration.md, docs/how-it-works.md ONLY.
git diff --name-only | grep -vE '^(README\.md|docs/(README|cli|configuration|how-it-works)\.md)$' && echo "FAIL: out-of-scope file edited" || echo "OK: scope clean"

# Guard 3: NO production/PRD/plan/tasks files touched.
git diff --name-only | grep -E '^(PRD\.md|plan/|tasks\.json|prd_snapshot\.md|internal/|cmd/|providers/|FUTURE_SPEC\.md|\.markdownlint\.json|Makefile|go\.mod)' && echo "FAIL: forbidden file edited" || echo "OK: no forbidden files"

# Guard 4: the binary's bootstrap template was NOT edited (it already documents the opt-out correctly).
git diff --name-only | grep -q 'internal/config/bootstrap.go' && echo "FAIL: edited production template (out of scope)" || echo "OK: bootstrap.go untouched"

# Guard 5: internal links resolve (the new lock-status anchor + the referenced anchors exist).
grep -nE "^### lock status" docs/cli.md                       # anchor target exists
grep -nE "^### Per-repo run lock" docs/how-it-works.md        # linked anchor exists
grep -nE "^## Environment variables" docs/configuration.md    # linked anchor exists
```

## Final Validation Checklist

### Technical Validation
- [ ] `npx markdownlint-cli2 'README.md' 'docs/**/*.md'` clean (MD013/MD033/MD060 off — long lines OK)
- [ ] `make build && ./bin/stagecoach lock status` prints `no run lock for <cwd>`, exit 0 (Level 3 smoke)
- [ ] `./bin/stagecoach lock --help` + `./bin/stagecoach --help | grep -i lock` show the `lock` group
- [ ] grep guards (Level 2 + Level 4) all pass: opt-out names spelled correctly; em-dash output present;
      no 129/143 rows; no "stagecoach_NO_PARENT_WATCHDOG" (wrong casing); no stale SIGINT/SIGTERM-only claim

### Feature Validation
- [ ] docs/cli.md: new `### lock status` section with the EXACT output block (em-dash, alignment) + the
      opt-out row in the Flag↔env↔git-config map; Exit-codes table UNCHANGED
- [ ] docs/how-it-works.md: "Orphaned-but-alive (FR-K1–K7)" paragraph (watchdog/SIGHUP/lock status/opt-out/
      Windows no-op/FR52-preserved) mirroring PRD §18.5:2039
- [ ] docs/configuration.md: STAGECOACH_NO_PARENT_WATCHDOG env row + stagecoach.noParentWatchdog git row +
      no_parent_watchdog default row + commented [generation] key + lock-status xref
- [ ] README.md: FAQ run-lock entry names the watchdog self-exit (lazygit/IDE/detaching-terminal) +
      `stagecoach lock status`; CLI-reference area has a discoverability one-liner
- [ ] docs/README.md: capability-index bullet links the three anchors
- [ ] A user reading the docs can discover the lock status subcommand, the watchdog behavior, SIGHUP, and
      the opt-out (the contract's OUTPUT criterion)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY the five docs files (Level 4 Guard 2)
- [ ] NO edit to PRD.md, plan/**, tasks.json, prd_snapshot.md, any internal/* or cmd/* source,
      providers/*.toml, FUTURE_SPEC.md, .markdownlint.json, Makefile, go.mod (Guards 3 & 4)
- [ ] internal/config/bootstrap.go UNCHANGED (it already documents the opt-out — Guard 4)

---

## Anti-Patterns to Avoid

- ❌ Don't add 129/143 exit-code rows — `3 (Rescue)` already covers the SIGHUP/watchdog rescue path.
- ❌ Don't hard-wrap prose at 80 columns — MD013 is OFF and the existing docs are long-line; wrapping is
  inconsistent and a reviewer red flag.
- ❌ Don't drift from the binary's exact output strings (alignment, em-dash) — docs/README.md promises the
  binary is authoritative.
- ❌ Don't invent a `--no-parent-watchdog` flag — FR-K6 is env + git-config + config-file only.
- ❌ Don't mix the three key spellings: env = `STAGECOACH_NO_PARENT_WATCHDOG` (all-caps), git =
  `stagecoach.noParentWatchdog` (camelCase), file = `[generation].no_parent_watchdog` (snake_case).
- ❌ Don't edit the binary's bootstrap template (internal/config/bootstrap.go) — it already documents the
  opt-out; this task is human-readable docs only.
- ❌ Don't claim SIGHUP or the watchdog work on Windows — both are Unix-only (FR-K7).
- ❌ Don't re-document the full lock-status output in how-it-works.md — reference docs/cli.md (DRY).
