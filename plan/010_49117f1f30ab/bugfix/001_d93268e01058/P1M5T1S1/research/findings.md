# P1.M5.T1.S1 — Research Findings
## Sweep README.md and docs/how-it-works.md for hooks-feature accuracy (Mode B docs sync)

---

## 0. Task contract (verbatim from item_description)

Mode B changeset-level docs sync. The 4 hook git-parity bug fixes are COMPLETE (P1.M1.T1 argc=1,
P1.M1.T2 trailing newline, P1.M2.T1 stagecoach.noVerify, P1.M3.T1 empty-message guard). The per-file docs
(docs/cli.md:44, docs/configuration.md:155) + config.go comment were ALREADY fixed in P1.M2.T1.S1 (Mode A).
THIS task sweeps ONLY the overview/README-level docs:

- README.md:71 (feature-table row "Commit hooks on every `stagecoach` commit")
- README.md:367-369 (FAQ "Does it run my pre-commit hooks?")
- docs/how-it-works.md:310-324 ("Commit hooks on the plumbing path" section)
- docs/how-it-works.md:337 (snapshot-based flow feature-list bullet "Honors pre-commit hooks")

The 4 accuracy checks:
- (a) stale key `stagecoach.no_verify` (should be `stagecoach.noVerify`) — **CHECK**
- (b) claims prepare-commit-msg runs with 2 args (should be 1) — **CHECK**
- (c) implies message files have no trailing newline (git always adds one) — **CHECK**
- (d) omits the empty-message abort behavior (should mention it, like the `--edit` abort) — **CHECK**

"If any of these are found, update the doc text. If none are found (likely for README which is high-level),
document that the docs were reviewed and no changes were needed."

---

## 1. VERDICT per location (all 4 read in full)

### Check (a) — stale `stagecoach.no_verify` key name: **NOT PRESENT in either overview doc.**
`grep -cE 'no_verify|noVerify|argc|two arg|2 arg' README.md` → **0**; same for `docs/how-it-works.md` → **0**.
Both docs mention ONLY the `--no-verify` FLAG (correct, unchanged). The stale `stagecoach.no_verify` KEY lived
in docs/cli.md:44 + docs/configuration.md:155 + config.go:130 — **all already corrected to `stagecoach.noVerify`
in P1.M2.T1.S1 (Mode A, Complete)**. → **NO CHANGE for (a).**

### Check (b) — 2-args / argc claim: **NOT PRESENT.**
Neither overview doc mentions prepare-commit-msg's argv at all. The false "VERIFIED argc=2" claim was in
`internal/hooks/runner.go:52/178` (a CODE comment, fixed in P1.M1.T1.S1) — NOT in these docs. → **NO CHANGE for (b).**

### Check (c) — trailing-newline implication: **NOT PRESENT.**
Neither overview doc describes the message-file format. The bug was in `internal/hooks/runner.go:103` (CODE,
fixed in P1.M1.T2.S1). → **NO CHANGE for (c).**

### Check (d) — empty-message abort omission: **FOUND in ONE location.**
- **docs/how-it-works.md:310-324 ("Commit hooks on the plumbing path")** — the DETAILED section. It describes
  the rescue-on-hook-failure path ("A hook that exits non-zero or times out aborts the run as a **rescue**
  (exit code 3) — no commit is created …") but OMITS the Issue-4 behavior: a `prepare-commit-msg`/`commit-msg`
  hook that EMPTIES the message file aborts with **exit 1, no commit** (mirroring `git commit`'s "Aborting
  commit due to empty commit message." and the `--edit` path's empty-result abort). This is the ONE substantive
  edit. The contract: "should mention it for completeness, like the --edit abort."
  - **The `--edit` parallel (verbatim, for wording consistency)**: `docs/cli.md:42` — "An empty result aborts
    (exit 1, not a rescue)."
- **README.md:71 (feature-table row)** — high-level; does NOT enumerate ANY failure mode (no rescue, no abort).
  Adding empty-message abort here would be inconsistent with the row's abstraction level. → **NO CHANGE.**
- **README.md:367-369 (FAQ)** — high-level; already lists ONE abort example ("a hook that stages brand-new
  content aborts the run"). That example is about `pre-commit` + the freeze; the empty-message abort is a
  `prepare-commit-msg`/`commit-msg` behavior — a different concern. The FAQ answers "does it run my hooks",
  not "enumerate every abort". → **NO CHANGE required** (optionally the implementer MAY add a brief clause, but
  the contract explicitly says README is "likely high-level" → no change).
- **docs/how-it-works.md:337 (feature-list bullet)** — terse bullet ("Honors pre-commit hooks … `--no-verify`
  skips pre-commit + commit-msg"). Does not enumerate failure modes. → **NO CHANGE.**

### SUMMARY
- **3 of 4 checks (a/b/c) → NO stale references anywhere in the overview docs.** The bug fixes left no stale
  text at this level (the stale text was per-file docs + code comments, all already fixed).
- **Check (d) → ONE substantive edit**: add the empty-message-after-hooks abort to the DETAILED hooks section
  in docs/how-it-works.md (the one place that DOES enumerate hook failure modes).
- **README.md → reviewed, NO change required** (high-level; both locations clean + appropriately abstract).

---

## 2. The ONE edit — exact insertion point + recommended prose

**File**: `docs/how-it-works.md`, section `## Commit hooks on the plumbing path`.

**Current text (the insertion neighborhood, read in full)**:
```
`--no-verify` mirrors `git commit --no-verify`: it skips `pre-commit` and `commit-msg` only
(`prepare-commit-msg` and `post-commit` still run). A hook that exits non-zero or times out aborts the
run as a **rescue** (exit code 3) — no commit is created, HEAD and the index are byte-for-byte
unchanged, and the rescue recipe is printed. `post-commit` is best-effort: its exit code is logged as a
warning but cannot undo an already-landed commit (git itself disregards it).
```

**Insertion**: add ONE sentence AFTER the rescue sentence ("…the rescue recipe is printed.") and BEFORE
"`post-commit` is best-effort". This groups the two ABORT behaviors together (rescue on hook failure;
abort on empty message) — the natural place a reader looks for "what happens if a hook misbehaves".

**Recommended prose** (mirrors the rescue sentence's structure + the `--edit` parallel at cli.md:42):
> A hook that empties the message file (a rejection or force-re-edit pattern) aborts with **exit 1** and no
> commit created — mirroring `git commit`'s "Aborting commit due to empty commit message." and the `--edit`
> path's empty-result abort (exit 1, not a rescue). HEAD and the index are untouched at that point (no
> `update-ref` has run).

This is accurate to the Issue 4 fix (P1.M3.T1.S1, Complete): the empty-message guard in `generate.CommitStaged`
/ `runPipeline` / `decompose.publishCommit` returns a non-rescue error → exit 1, no commit, HEAD/index clean.

---

## 3. Dependency: the Issue 4 fix is LANDED (the docs must describe shipped behavior)

The empty-message abort IS shipped behavior (P1.M3.T1.S1 — Complete). The guard sits after `RunCommitHooks`
in all 3 commit paths:
- `internal/generate/generate.go` (`CommitStaged`, after the hooks block)
- `pkg/stagecoach/stagecoach.go` (`runPipeline`, after the hooks block)
- `internal/decompose/message.go` (`publishCommit`, after `RunCommitHooks`)

→ the docs edit describes EXISTING shipped behavior, not a future promise. (If a reviewer questions the
behavior, the implementation is already in the tree.)

---

## 4. Scope boundary (do NOT edit — owned elsewhere / already done)

- **docs/cli.md:44 + docs/configuration.md:155 + config.go:130** — the `stagecoach.noVerify` key-name fix is
  ALREADY DONE (P1.M2.T1.S1, Mode A, Complete). Do NOT re-touch them.
- **docs/how-it-works.md** other sections (snapshot flow, decompose, prompt engineering, hook-mode
  trade-off, multi-turn fallback) — NOT in scope.
- **README.md** — reviewed; NO change required (high-level, clean). Do NOT add failure-mode detail to the
  feature-table row or rewrite the FAQ.
- **docs/how-it-works.md:337** (feature-list bullet) — terse; NOT in scope for the abort detail.
- **ANY source code / tests** — Mode B docs-only. The 4 bug fixes are the INPUT, read-only.

---

## 5. Validation (docs-only — lightweight)

```bash
# 1. The empty-message abort is now documented in the detailed hooks section:
grep -nE 'empty.*message|Aborting commit|exit 1' docs/how-it-works.md
# 2. NO stale key name / argc / newline references were introduced (or were ever present):
grep -cE 'no_verify|argc|two arg|2 arg' README.md docs/how-it-works.md   # expect 0 / 0
# 3. The --edit parallel wording stays consistent (cli.md:42 unchanged — READ ONLY):
grep -n 'empty result aborts' docs/cli.md
# 4. Markdown sanity (if a linter is available; else visual review):
markdownlint docs/how-it-works.md 2>/dev/null || echo "(no markdownlint — visual review)"
# 5. Scope: ONLY docs/how-it-works.md changed (README reviewed, no change):
git status --porcelain   # expect ONLY docs/how-it-works.md
```

No `go build`/`go test` (no code change). The gate: the empty-message abort appears in the detailed hooks
section; no stale (a/b/c) references exist; markdown well-formed; only docs/how-it-works.md modified.

---

## 6. Confidence & risks

**Confidence: 9.5/10.** Tiny, precise docs edit (one sentence) + a documented review. All 4 locations read
in full; the (a/b/c) absence is grep-confirmed (count 0); the (d) finding is unambiguous (the detailed
section enumerates failure modes but omits this one); the parallel wording is pinned (cli.md:42); the
underlying behavior is shipped (P1.M3.T1.S1 Complete).

**Risks (low):**
- **Over-editing README.** The contract says README is "likely high-level" → no change. A tempted implementer
  might add failure-mode detail to the feature-table row or FAQ. The PRP scopes README to REVIEW-ONLY.
- **Touching already-fixed files.** docs/cli.md / docs/configuration.md / config.go were fixed in P1.M2.T1.S1;
  re-editing them is out of scope and risks churn. The PRP's LEAVE list is explicit.
- **Inaccurate prose.** The empty-message abort must be described as "exit 1, not a rescue, no commit,
  HEAD/index untouched" (matching Issue 4's non-rescue error), NOT as a rescue (exit 3). The PRP quotes the
  cli.md:42 parallel ("exit 1, not a rescue") and the rescue sentence it sits next to, so the wording is pinned.
