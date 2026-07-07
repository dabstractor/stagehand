# P1.M3.T1.S3 — CI workflow + .gitignore rename: Design Decisions

The non-obvious calls for the stagehand→stagecoach rename on `.github/workflows/ci.yml`,
`.github/workflows/release.yml`, and `.gitignore` (project rename Layer 4.3–4.4). Ground truth: the
ACTUAL files at `/home/dustin/projects/stagehand/` (the project root — go.mod is already
`github.com/dustin/stagecoach`), the S2 PRP (the namespace decision: KEEP `dustin/`), the work-item
CONTRACT, and PRD §20.4 (h3.96) + h2.30 ("All references to 'stagehand' must be replaced with 'stagecoach'").

Read this BEFORE implementing — it corrects FOUR inaccuracies in the work item's naive sed plan.

---

## §0 — Verified current state (re-grepped against the real files)

| File | `stagehand`/`Stagehand` refs | Details |
|------|------------------------------|---------|
| `.github/workflows/ci.yml` | **5** | L1 comment `Stagehand CI`; **L102-105 the coverage-gate module paths** `github.com/dustin/stagehand/internal/{git,provider,generate,config}` |
| `.github/workflows/release.yml` | **0** | none (reviewed — no edit needed; see §4) |
| `.gitignore` | **4** | L4 `/stagehand` (stale binary ignore); L22 comment `Stagehand repo-local config (.stagehand.toml; …)`; L23 `.stagehand.toml`; L40 comment `(Stagehand)` |
| `bin/` | n/a | already `stagecoach` + `stagecoach-test` only (S1's Makefile rebuild) — **NO** `bin/stagehand`/`bin/stagehand-test` |
| repo root | n/a | stale `./stagehand` binary EXISTS (gitignored, untracked, Jul 7 11:46); `./stagecoach` is the current one |

The plan artifacts live at `/home/dustin/projects/stagecoach/plan/012_…/`; the CODE being edited is at
`/home/dustin/projects/stagehand/` (run all commands from there).

---

## §1 — ci.yml: GLOBAL sed (both cases), NOT the work item's narrow module-path-only sed

The work item suggests `sed -i 's|github.com/dustin/stagehand|github.com/dustin/stagecoach|g' ci.yml`.
That is **INCOMPLETE**: it fixes lines 102-105 but MISSES line 1's `Stagehand CI` comment. Line 1 would
then fail the final comprehensive grep audit (P1.M5.T2.S1: "zero stagehand references in tracked files").

Use the GLOBAL two-branch sed (same shape S2 used for `.goreleaser.yaml`, extended to the capitalized form
because ci.yml L1 has `Stagehand` with a capital S):

```bash
sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' .github/workflows/ci.yml
```

This fixes ALL 5 references (L1 comment + L102-105 paths) in one shot. Verified safe: the only 5 matches
are the comment + the 4 module paths; no partial-word collisions (no `stagehandler`); `dustin/`,
`github.com/`, `internal/` are unaffected (they contain no `stagehand` substring).

---

## §2 — ci.yml lines 102-105 are FUNCTIONAL (load-bearing), not cosmetic

This is the single most important point. L102-105 are the **coverage-gate package paths** in the awk
script (the `Enforce >=85% on internal/{git,provider,generate,config}` step):

```awk
t[1]="github.com/dustin/stagehand/internal/git"
t[2]="github.com/dustin/stagehand/internal/provider"
t[3]="github.com/dustin/stagehand/internal/generate"
t[4]="github.com/dustin/stagehand/internal/config"
...
if(!(t[i] in tot)){ printf("::error::%s — no coverage data\n",t[i]); fail=1; continue }
```

The awk reads `coverage.out`, which Go generates with the **REAL** module path — `github.com/dustin/stagecoach/...`
(go.mod is already renamed). The `t[i]` keys are matched against `tot[pkg]` (built from coverage.out's
`$1` field). If `t[i]` still says `stagehand`, it matches NOTHING in `tot` ⇒ EVERY package prints
`::error::... — no coverage data` ⇒ `fail=1` ⇒ **the coverage gate FAILS on the next CI run**.

So this is not a cosmetic rename — it is a CI-breaking bug left by M1.T1.S1's Go-only sed. The global sed
in §1 fixes it (the `t[i]` paths become `github.com/dustin/stagecoach/internal/...`, matching coverage.out).

KEEP `dustin/`: the sed touches only `stagehand`→`stagecoach`; `github.com/dustin/` is preserved. This
matches go.mod (`github.com/dustin/stagecoach`) + S2's namespace decision (3 sources: go.mod + PRD §21.2/§21.3
+ the goreleaser owner note). Do NOT change `dustin`→`dabstractor`.

---

## §3 — .gitignore: DELETE the stale `/stagehand` line (do NOT rename it — it would duplicate)

The work item says "line 4 `/stagehand` → `/stagecoach`" and suggests
`sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' .gitignore`. But **`/stagecoach` is ALREADY
present on line 5** (verified: exactly one `/stagecoach` line, L5). Running that global sed would turn L4
`/stagehand` → `/stagecoach`, creating a **DUPLICATE** `/stagecoach` (L4 and L5). That is sloppy (two
identical ignore lines).

The stale `/stagehand` (L4) is the ignore for the OLD root binary name. The binary is now `stagecoach`
(Makefile builds `./bin/stagecoach`; a root build would be `./stagecoach`, covered by L5). So L4 is pure
cruft — **DELETE it**, don't rename it:

```bash
# Step 1: remove the stale /stagehand binary-ignore (line 5 /stagecoach already covers the renamed binary).
sed -i '/^\/stagehand$/d' .gitignore
# Step 2: rename the remaining refs (L22 comment, L23 .stagehand.toml, L40 comment).
sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g' .gitignore
```

After step 1, L4 is gone (L5 `/stagecoach` shifts up). Step 2 then renames:
- L22 `# Stagehand repo-local config (per-repo .stagehand.toml; …)` → `# Stagecoach repo-local config (per-repo .stagecoach.toml; …)`
- L23 `.stagehand.toml` → `.stagecoach.toml` (no duplicate — `.stagecoach.toml` did NOT already exist)
- L40 `# Go build / test artifacts (Stagehand)` → `# Go build / test artifacts (Stagecoach)`

Note: `.stagecoach.toml` (L23) is a genuine rename, NOT a dedup (unlike `/stagecoach`). M2.T2.S1 renamed
the config-file DISCOVERY in Go code, but .gitignore (a separate non-Go file) still had the old
`.stagehand.toml` — this task fixes it.

---

## §4 — release.yml: NO CHANGE (0 refs; the dustin/ SECRETS comments are correct)

Verified: `grep -ci stagehand release.yml` → 0. The work item's caution about "SECRETS comments
referencing `dustin/homebrew-tap`" is a RED HERRING: those references (L5-6:
`contents:write on dustin/homebrew-tap`, `dustin/scoop-bucket`) are **CORRECT** and must NOT be changed.
S2 established (3 independent sources) that the org is `dustin/`:
1. go.mod = `github.com/dustin/stagecoach` (the `go install` module path).
2. PRD §21.2/§21.3 = `brew install dustin/tap/stagecoach`, `scoop install dustin/stagecoach`.
3. The existing `.goreleaser.yaml` `release.github.owner: dustin` (explicit override of the git remote).

release.yml's `dustin/homebrew-tap` + `dustin/scoop-bucket` match the goreleaser config's owners
(S2 preserves them). Changing them to `dabstractor/` would make release.yml INCONSISTENT with go.mod +
PRD + goreleaser. **ACTION: review release.yml, confirm 0 stagehand refs, make NO edit.** Document the
review in the commit/PR; do not touch the file.

---

## §5 — Stale binary cleanup: the work item targets the WRONG path

The work item says "Remove stale `bin/stagehand` and `bin/stagehand-test`: `rm -f bin/stagehand
bin/stagehand-test`." Two problems:

1. **`bin/` already has only `stagecoach` + `stagecoach-test`** (S1's Makefile rebuild produced the
   renamed binaries). `bin/stagehand` and `bin/stagehand-test` DO NOT EXIST. So that `rm -f` is a NO-OP
   (`-f` makes it silently succeed).
2. **The ACTUAL stale binary is at the REPO ROOT**: `./stagehand` (9144205 bytes, Jul 7 11:46, gitignored
   by .gitignore L4). This is the old root-level `go build` artifact. There is no `./stagehand-test` at root.

Correct cleanup (from the project root `/home/dustin/projects/stagehand`):

```bash
rm -f stagehand stagehand-test      # remove stale ROOT binaries (gitignored, untracked); -f = no error if absent
```

(Optionally also `rm -f bin/stagehand bin/stagehand-test` as a defensive no-op — it does nothing, but
matches the work item's literal instruction and is harmless.)

NOTE: these are UNTRACKED artifacts (`git check-ignore stagehand` confirms `stagehand` is gitignored).
They do NOT affect the tracked-files rename audit (P1.M5.T2.S1 audits TRACKED files). Removing them is a
cleanliness step so a developer does not accidentally run the old `./stagehand` binary. This is why §3's
removal of .gitignore L4 `/stagehand` is safe: once the stale binary is gone, the ignore is pure cruft.

---

## §6 — Scope boundaries (do NOT touch)

This task owns: `.github/workflows/ci.yml` (edit) + `.gitignore` (edit) + `release.yml` (review, no edit)
+ stale untracked binary cleanup. It does NOT touch:

- `Makefile` (S1, Complete), `.goreleaser.yaml` (S2, parallel), `go.mod` (M1.T1.S1, Complete), all `.go`
  files (M1/M2, Complete).
- `README.md` + `docs/*.md` (M4.T1 — Planned).
- `providers/*.toml` + `FUTURE_SPEC.md` (M4.T2 — Planned).
- `plan/` historical artifacts (M5.T1 — Planned).
- `PRD.md` (read-only).

Disjoint files ⇒ no conflict with S1 (Makefile) or S2 (.goreleaser.yaml). Zero file overlap.

---

## §7 — Validation

The rename is a pure text substitution; validation is deterministic grep + a YAML sanity check:

```bash
# Zero stagehand refs in the edited tracked files:
grep -rni stagehand .github/workflows/ci.yml .gitignore    # → no output (0 refs)
# ci.yml coverage-gate paths now match go.mod's module path:
grep -n 'github.com/dustin/stagecoach/internal/' .github/workflows/ci.yml   # → L102-105
# release.yml still 0 refs (and dustin/ preserved):
grep -ci stagehand .github/workflows/release.yml            # → 0
grep -n 'dustin/homebrew-tap\|dustin/scoop-bucket' .github/workflows/release.yml   # → present (CORRECT)
# .gitignore: single /stagecoach (no duplicate), .stagecoach.toml present:
grep -c '^/stagecoach$' .gitignore                          # → 1
grep -n '.stagecoach.toml' .gitignore                       # → the renamed line
# YAML still valid (actionlint if installed; else a go yaml parse or just visual):
actionlint 2>/dev/null || echo "(actionlint not installed — visual review of ci.yml YAML structure)"
# scope: only ci.yml + .gitignore changed among TRACKED files:
git status --short
```
