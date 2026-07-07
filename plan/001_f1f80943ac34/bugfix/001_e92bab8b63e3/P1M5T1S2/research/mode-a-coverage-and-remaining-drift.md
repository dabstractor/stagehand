# Research Note — P1.M5.T1.S2: docs/*.md overview coherence

## What Mode-A (implementing subtasks) ALREADY covered

Exact git diffs (`git show <sha> -- docs/`):

| Subtask | Issue(s) | File(s) touched | Edit landed |
|---------|----------|-----------------|-------------|
| P1.M1.T1 (1368895) | 1 | `docs/cli.md` | appended "`--config` is honored by every command — including the default commit action…" prose (line 30) |
| P1.M2.T1 (9b8dc15) | 3 | `docs/cli.md` + `docs/how-it-works.md` | cli exit-code row → "**provider command missing on `$PATH` (checked before the snapshot)**" (line 81); how-it-works failure table → added "Agent missing on `$PATH` \| 1 (Error)" row (line 57) |
| P1.M3.T1 (f8db87e) | 2, 6 | `docs/cli.md` | `--dry-run` flag desc → "Run the full generate→parse→duplicate-check pipeline (same as a real commit, including retry)…" (line 26) |
| P1.M4.T1 (f1ca18a) | 4 | `docs/configuration.md` | added "output/strip_code_fence apply to parsing… override any per-provider defaults" paragraph (line ~81) + `stagecoach.output` / `stagecoach.stripCodeFence` git-config rows |

P1.M4.T2 (Issue 7) made NO doc edits (internal UX, no docs surface). README synced by S1 (79e7676).

## Contract coherence checklist vs. current docs state

| Coherence check (from item LOGIC) | Status | Where |
|------------------------------------|--------|-------|
| failure-modes table: agent-missing → exit 1 (Issue 3) | ✅ DONE (M2) | how-it-works.md:57 |
| dry-run overview: full pipeline + **snapshot** (Issues 2/6) | ⚠️ partial | cli.md:26 says "full generate→parse→duplicate-check (same as a real commit, incl retry)" — covers snapshot **by reference** ("same as a real commit") but the explicit step enumeration OMITS the write-tree snapshot |
| config overview: `[generation]` knobs apply (Issue 4) | ✅ DONE (M4) | configuration.md:~81 |
| config overview: `--config` honored everywhere incl. default action (Issue 1) | ❌ GAP in config ref | cli.md:30 has it; **configuration.md** discovery/preference prose does NOT (only the env-var table row `STAGECOACH_CONFIG ─↣ --config` cross-refs it, no prose) |
| no stale claim that dry-run skips snapshot/dup-check | ✅ verified | none found anywhere |

## Remaining overview drift NOT covered by Mode-A (the S2 deliverables)

- **A (PRIMARY, configuration.md):** config-reference prose never tells a reader about the `--config <path>` discovery override being honored by the default action. The env-var table names `STAGECOACH_CONFIG`/`--config` but the "Precedence" + "Config file paths" sections — the overview a user reads to understand discovery — omit it. Mirror cli.md:30 / README:121.
- **B (RECOMMENDED, providers.md):** manifest reference documents `output`/`strip_code_fence` as per-provider manifest fields (schema line ~28-30, Output parsing line ~110-122) but never notes that a `[generation] output`/`strip_code_fence` value OVERRIDES them (Issue 4). Cross-ref configuration.md.
- **C (OPTIONAL, cli.md):** refine the `--dry-run` enumeration to name the snapshot step explicitly. Current text is already accurate by reference ("same as a real commit") — this is a clarity polish, not a fix.

## No-edit findings (confirmed coherent, leave alone)

- how-it-works.md "Rescue protocol" / Snapshot invariant #3 — correctly scoped to post-snapshot failures; the Issue-3 pre-flight is pre-snapshot, no contradiction.
- how-it-works.md "Why raw output, not JSON" — architecture-level framing of the default; not a config overview; out of scope (configuration.md owns `[generation]`).
- docs/README.md index — "11 global flags / 7-layer precedence / 18-field schema" all accurate.
- providers.md command/detect — no stale claim about missing-command behavior (that lives in how-it-works/cli failure tables).

## Validation harness

- `npx markdownlint-cli2 'docs/**/*.md'` → MUST be 0 error(s). markdownlint-cli2 v0.22.1 cached via npx.
- `.markdownlint.json`: default=true, MD013/MD033/MD060 off (MD060 is non-standard → silently ignored). NOT in CI (ci.yml) — run manually.
- Coherence greps vs cli.md (source of truth) + `go build ./... && go vet ./... && go test ./...` sanity (no .go touched).
