# S2 Implementation Notes — Qualify the docs/cli.md contention-behavior prose (line 379)

> Scope: P1.M1.T1.S2 — a single-paragraph, doc-only rewrite at `docs/cli.md:379` that scopes the FR52
> no-op fast path (exit 0) to the **single-commit (staged) path** and documents that the **decompose
> path** exits `5` (Busy) on an accidental double-run. Mirrors S1 (README.md:330) and aligns with S3
> (docs/how-it-works.md:155). Verified against the live file 2026-07-02.

## 0. What changes and why (Issue 1 — doc over-claim)

`docs/cli.md:379` currently states the no-op fast path **generically**: "if a contending run's staged
changes are already covered by the in-progress run's published snapshot, it exits **0**". This is
**false on the decompose path**: the holder publishes a **working-tree** snapshot (`T_start`,
`internal/decompose/decompose.go:169` `lock.SetSnapshot(tStart)`), while a lock-free contender can only
compute an **index** snapshot (`git write-tree`) — and with nothing staged (decompose's activation
condition, FR-M1) the index tree == `baseTree` == `HEAD^{tree}` ≠ `T_start`. So `contenderTree == snap`
is **always false** on decompose → the contender exits `5` (Busy), never 0. The behavior is correct and
safe (defense-in-depth); only the doc over-promises. The fix is to qualify the doc (Option 1,
issue_analysis.md's recommended lowest-risk fix) — NO code change.

## 1. The exact current text (docs/cli.md:379 — ONE logical markdown line)

```
Code `5` (Busy) is distinct from the commit-failure codes so scripts can tell "busy, retry" from "failed." Contention on the per-repo run lock (FR52) has two behaviors: if a contending run's staged changes are already covered by the in-progress run's published snapshot, it exits **0** ("nothing to do — an in-progress run already covers your staged changes"); if genuinely new work is staged, it exits **5** with the holder's pid/host and leaves the new changes staged for a re-run. Stagecoach never force-breaks the lock.
```

## 2. The exact target text (the contract's authoritative phrasing — ONE logical markdown line)

```
Code `5` (Busy) is distinct from the commit-failure codes so scripts can tell "busy, retry" from "failed." Contention on the per-repo run lock (FR52) has two behaviors. On the single-commit path (changes staged): if a contending run's staged changes are already covered by the in-progress run's published index snapshot, it exits **0** ("nothing to do — an in-progress run already covers your staged changes"); if genuinely new work is staged, it exits **5** with the holder's pid/host and leaves the new changes staged for a re-run. On the decompose path (nothing staged, working tree dirty): an accidental double-run exits **5** (Busy) rather than 0 — the holder publishes a working-tree snapshot (`T_start`) that a lock-free contender cannot reproduce from the index alone, so it conservatively refuses. Stagecoach never force-breaks the lock.
```

### What is PRESERVED verbatim (the contract's "keep" list)
- First sentence: `Code \`5\` (Busy) is distinct from the commit-failure codes so scripts can tell "busy, retry" from "failed."`
- Last sentence: `Stagecoach never force-breaks the lock.`
- The single-commit path's BOTH outcomes: exit **0** (nothing-to-do) AND exit **5** (Busy if genuinely new work is staged).
- The "nothing to do — an in-progress run already covers your staged changes" string (consistent w/ README/how-it-works).

### What is ADDED
- Path scoping: "On the single-commit path (changes staged)" vs "On the decompose path (nothing staged, working tree dirty)".
- The decompose-path clause: accidental double-run exits **5** (Busy) rather than 0, with the working-tree-snapshot (`T_start`) rationale.
- Precision: single-commit path's snapshot is the "published **index** snapshot" (vs decompose's working-tree `T_start`).

## 3. Scope discipline — the boundaries

- EDIT ONLY line 379 (the contention-behavior prose paragraph). It is ONE logical markdown line.
- Do NOT edit the **exit-code TABLE** (lines 368-375: `| Code | Meaning |` … `| 124 | Timeout |`). The
  table is generic and does NOT over-claim — it correctly says "`5` | Busy — another stagecoach run holds
  the per-repo lock; retry after it finishes." Leave it.
- Do NOT edit the "Exit codes mirror the constants in `internal/exitcode/exitcode.go`…" explanation
  paragraph (immediately above line 379) — it is correct and unrelated.
- Do NOT edit README.md (S1), docs/how-it-works.md (S3), any `.go`/test file, PRD.md, tasks.json,
  prd_snapshot.md, or plan/*.
- NO code change. `go test ./...` is a sanity check only (must stay green; doc change shouldn't affect it).

## 4. Coordination with siblings (parallel execution)

- **S1 (P1.M1.T1.S1)** scopes README.md:330 ("Safe to run twice") using the SAME per-path semantics
  (single-commit → 0-or-5; decompose → 5). S1's PRP is the structural template; my docs/cli.md prose
  carries the same semantics in cli.md's voice. S1 explicitly defers docs/cli.md to S2 — no conflict
  (different files).
- **S3 (P1.M1.T1.S3)** will qualify docs/how-it-works.md:155 ("No-op fast path" subsection). S2's
  wording is a ready reference for S3; minor drift is fine (P1.M3.T1 does the Mode-B coherence sweep).

## 5. Markdown style

- `.markdownlint.json` is configured (root). The existing line 379 is ONE long logical line (the project
  allows long lines — MD013 is evidently disabled or the existing line already violates it). Keep the
  rewrite as ONE logical markdown line (do NOT hard-wrap) to match the surrounding style and avoid
  introducing a new lint difference.
- Balance the inline markup: backticks (`` `5` ``, `` `0` ``, `` `T_start` ``), bold (`**0**`, `**5**`),
  and the quoted `"busy, retry"` / `"nothing to do — …"` / `"failed."` strings.

## 6. Sources

- `architecture/issue_analysis.md` Issue 1 (root cause: working-tree T_start vs index baseTree
  snapshot-axis mismatch; Option 1 = qualify docs, recommended).
- `architecture/system_context.md` (line 29: docs/cli.md line 379 = contention-behavior prose, scope 1).
- `P1M1T1S1/PRP.md` (the README sibling — same semantics, structural template).
- PRD §18.5 (FR52 contention behavior); the live `docs/cli.md:379`.
