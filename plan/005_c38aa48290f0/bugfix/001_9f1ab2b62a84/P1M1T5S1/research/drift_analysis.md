# Documentation Drift Analysis — P1.M1.T5.S1

**Scope**: Verify README.md and `docs/*.md` reflect the four bug fixes (P1.M1.T1–T4).
**Method**: For each fix, identify the exact doc claim(s) that COULD drift, then probe the docs
to confirm the claim still holds (or is now MORE accurate) after the fix.

**Headline result**: **ZERO documentation drift.** Every fix brought the *code* into compliance
with docs that were already accurate. No doc file requires changes. The expected outcome per the
work item ("NO changes needed") is confirmed by direct inspection.

---

## Fix-by-fix analysis

### Issue 1 — lazygit foreign-key duplicate binding (T1) → NO DRIFT

**What T1 changed (commit `0dd57a3`)**: `lazygitEntry.Install()` now probes for an *unmarked*
`customCommands` entry sharing the target key and prints a `WARNING` to stderr before proceeding.

**The doc claim that could drift**: any prose asserting lazygit install is *silent*, *overwrites*,
or *never warns* on a foreign key.

**Probe**:
- `docs/cli.md` L328 ("**Conflicting key behavior**") ALREADY documents the post-T1 behavior
  verbatim: *"If an **unmarked** entry already binds your target key (e.g. `<c-a>`), `install`
  prints a `WARNING` to stderr noting that a duplicate `customCommands` entry will be created,
  then proceeds through the normal no-mangle preview/confirm flow (outcome: *Updated*)."*
- `docs/cli.md` L290 ("No-mangle protocol") still holds — T1 added a warning, it did NOT weaken the
  preview/backup/re-parse protocol.
- `README.md` does NOT mention the lazygit no-mangle guarantee or foreign-key behavior at all (grep
  for `mangle` across README = 0 hits). README's lazygit section (L170–L189) is purely the install
  command + the manual YAML block.

**Verdict**: docs/cli.md already matches the T1-fixed code; README makes no claim that drifted.

### Issue 2 — hook exec progress noise on no-op (T2) → NO DRIFT

**What T2 changed (commit `0385f21`)**: the "Generating with <provider>…" progress line is now
emitted only AFTER the source-gate / empty-diff no-op checks pass (i.e. only when generation
actually runs).

**The doc claim that could drift**: any prose asserting hook exec prints "Generating…" for no-op
sources, OR any prose contradicting the "exits 0 having done nothing" no-op guarantee.

**Probe**:
- `docs/cli.md` L113 ("**Source-gated no-op (FR-H4)**") says hook exec "exits 0 having done nothing
  when a message source is present (`message`/`template`/`merge`/`squash`/`commit`) or nothing is
  staged." It makes NO claim about a "Generating…" line. After T2, "having done nothing" is
  *literally* true (no noise) — the doc is now MORE accurate.
- No doc anywhere claims the pre-T2 noisy behavior. README L265 "↳ generating with pi…" is in the
  main `stagecoach` snapshot-workflow ASCII diagram, NOT `hook exec`.

**Verdict**: no doc describes the pre-T2 noise; the no-op guarantee prose is unchanged and now holds
exactly.

### Issue 3 — config template missing v2.1 `[generation]` keys (T3) → NO DRIFT

**What T3 changed (commit `73b84e0`)**: added the 5 commented lines (`exclude`, `format`,
`locale`, `template`, `push`) to `exampleConfigTemplate` (internal/cmd/config.go).

**The doc claim that could drift**: `docs/configuration.md` L112 says the inert template
"documents every available option." If docs/configuration.md itself listed the keys but the code
template did not, that's a *code-vs-doc* inconsistency — but the DOC is the source of truth here.

**Probe**:
- `docs/configuration.md` built-in defaults table ALREADY lists all 5 keys: `format` (L131),
  `locale` (L132), `template` (L133), `push` (L134); `exclude` is in the populated-config example
  (L104) + the dedicated "Exclusion globs" section (L222+).
- `docs/configuration.md` L112 ("documents every available option") is now *satisfied* by the
  T3-fixed template — the doc was already correct; T3 fixed the code to match the doc.
- `docs/cli.md` L187 (`--template` flag: "Write the inert all-commented reference config") — still
  accurate; no key list there to drift.

**Verdict**: docs/configuration.md was already complete and correct; T3 reconciled the code template
to it. No doc edit needed.

### Issue 4 — IsTerminal /dev/null misfire (T4) → NO DRIFT

**What T4 changed (PRP P1M1T4S1, in progress)**: `ui.IsTerminal` now uses a true isatty ioctl
probe (TCGETS/TIOCGETA on Unix, GetConsoleMode on Windows) so `/dev/null` returns `false`.

**The doc claim that could drift**: any prose about the `--interactive` TTY gate (FR-L3) or the
`DefaultConfirm` non-interactive auto-decline (FR-I3c) — specifically any claim that a *particular*
non-TTY stream does/doesn't trigger it.

**Probe**:
- `docs/cli.md` L188 (`--interactive` flag): "Non-TTY → exit 1 (use plain `config init`)."
- `docs/cli.md` L190: "Non-TTY stdin exits 1 pointing at plain `config init`."
- `docs/configuration.md` L52: "Non-TTY stdin exits 1 pointing at plain `config init`."
- These describe the FR-L3 gate generically. After T4, the `/dev/null` case correctly trips this
  gate — the docs were always the *intended* behavior; T4 makes `/dev/null` comply. No doc ever
  claimed `/dev/null` was a terminal or would bypass the gate.
- No doc mentions isatty internals, char devices, or `/dev/null` by name (grep = 0 hits in docs/).

**Verdict**: the FR-L3 / FR-I3c prose is unchanged and now holds for the `/dev/null` edge case.
No drift.

---

## Cross-cutting sweep (docs/README.md, docs/how-it-works.md, docs/providers.md)

- `docs/README.md` (documentation index): lists v2.1 additions per page (L33–L36) — accurate,
  unchanged by any of the four internal-behavior fixes.
- `docs/how-it-works.md`: architecture/pipeline prose; none of the four fixes alter described
  architecture. The hook-vs-snapshot trade-off (FR-H7) section is unaffected.
- `docs/providers.md`: manifest schema + 7 built-ins; none of the four fixes touch provider
  manifests. The `agy` non-TTY stdout note (L77) is unrelated to Issue 4 (that's a provider bug,
  not IsTerminal).

**Verdict**: no drift in any overview doc.

---

## Conclusion

All four fixes are **internal-behavior bug fixes**, not new features. The docs were written to the
intended/PRD behavior; the fixes reconcile the CODE to that intent. Therefore:

- **No doc file requires changes.**
- The deliverable is a **verification report** confirming docs accuracy (per work item 3(d): "If NO
  documentation drift is found … document this finding … make zero file changes. Do NOT invent
  documentation changes.").

The only mandatory, auditable output is the per-fix drift-check evidence (the probes above),
captured in a verification report under the work-item directory.
