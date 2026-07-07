# P1.M3.T1.S2 — Design Decisions (empty-message guard in runPipeline after RunCommitHooks)

Ground truth read before writing this note:
- **Bug-Fix PRD §h2.2/h3.3 Issue 4** (a hook that empties the message file must abort, not create an empty-
  message commit; git aborts "Aborting commit due to empty commit message." exit 1). The fix: after
  RunCommitHooks, if the finalized message is empty (after trimming), abort with the BARE ErrEmptyMessage.
- **The S1 CONTRACT** (P1.M3T1S1/PRP.md — DONE, shipped): added the guard to `generate.CommitStaged` after its
  hooks block (generate.go:436-439: `if strings.TrimSpace(msg) == "" { return Result{}, ErrEmptyMessage }`).
  S1 is the SAME-pattern precedent: same sentinel, same bare propagation (exit 1, NOT rescue), same placement
  (after the hooks block, before CommitTree). git status confirms generate.go + hooks_freeze_test.go modified.
- **pkg/stagecoach/stagecoach.go runPipeline** (read L412-706): the hooks block at L651-674
  (`if deps.Hooks != nil { ft, fm, herr := RunCommitHooks(..., dryRun, ...); if herr != nil { if dryRun { ...
  warn-and-print ... } else { return herr } } else { treeSHA, msg = ft, fm } }`); the dry-run early return at
  L678 (`if dryRun { ...; return Result{Message: msg}, nil }`); CommitTree at L694. The INSERTION POINT is
  between L674 (the hooks block's closing `}`) and L677 (the dry-run comment / `if dryRun`).
- **`generate.ErrEmptyMessage`** (finalize.go:45): `var ErrEmptyMessage = errors.New("stagecoach: empty commit
  message — aborted")` — EXPORTED. pkg/stagecoach already imports `generate` (L21) + `strings` (L15).
- **`buildDeps` wires `Hooks: hooks.DefaultRunner{}`** (stagecoach.go:387) ⇒ `GenerateCommit(..., DryRun:true)`
  exercises hooks in runPipeline (the `if deps.Hooks != nil` branch is live).
- **Test idiom** (stagecoach_test.go L236 TestGenerateCommit_DryRun): `setupTestRepo(t, stubtest.Options{Out:
  ...})` (builds stub + temp repo + chdir + cleanup) → `repoDir, _ := os.Getwd()` → `writeFile`/`stageFile` →
  `headSHA` → `GenerateCommit(ctx, Options{Provider:"stub", DryRun:true})`. Helpers `setupTestRepo`/`writeFile`/
  `stageFile`/`headSHA` are in stagecoach_test.go.
- Verified at research time: `go build ./... && go test ./...` GREEN (S1's guard is in place).

---

## §0 — Scope: runPipeline ONLY (pkg/stagecoach/stagecoach.go)

**S2 owns:** the empty-message guard in `runPipeline` (the dry-run/SystemExtra path) + its tests in
`stagecoach_test.go`. Issue 4 names THREE call sites with the same gap:
- `generate.CommitStaged` → **S1 (DONE)**.
- `pkg/stagecoach.runPipeline` → **S2 (this task)**.
- `decompose.publishCommit` → **S3 (P1.M3.T1.S3, Planned)**.

**Frozen / do NOT touch:** `internal/generate/generate.go` (S1's guard — already shipped), `internal/decompose/*`
(S3's scope), `internal/hooks/*`, `internal/cmd/*`. No conflict with S1 (generate.go) — S2 is pkg/stagecoach only.

---

## §1 — The guard: `if strings.TrimSpace(msg) == "" { return Result{}, generate.ErrEmptyMessage }`

**Decision:** insert ONE guard between the hooks block's closing `}` (L674) and the `if dryRun` early return
(L678). This is the SAME pattern S1 used in CommitStaged (generate.go:436-439), down to the sentinel + the bare
propagation. It fires AFTER the hooks block unconditionally — covering both (a) the hook-exited-0 case (`else`
branch set `msg = fm = ""`) and (b) the hook-exited-non-zero under dryRun case (warn-and-print set `msg =
re.Candidate`, which may be ""). HEAD + live index are untouched (the abort returns before CommitTree → no
update-ref ran). pkg/stagecoach already imports `strings` + `generate` ⇒ NO new import.

---

## §2 — ErrEmptyMessage is NOT a *RescueError ⇒ it does NOT enter the dryRun warn-and-print (the KEY point)

**Decision:** the guard returns the BARE `generate.ErrEmptyMessage`. This is the load-bearing subtlety: runPipeline's
dryRun warn-and-print (FR-V8a, L655-668) handles ONLY `*generate.RescueError` (`errors.As(herr, &re)`). It is
INSIDE the hooks block's `if herr != nil` branch. The empty-message guard is AFTER the hooks block — it never
enters that branch. And `ErrEmptyMessage` is a bare `error`, NOT a `*RescueError`, so even if it reached the
warn-and-print logic it wouldn't match `errors.As(&re)`. ⇒ Under BOTH dryRun and commit paths, an empty message
→ `ErrEmptyMessage` → exitcode.For() → **exit 1** (NOT exit 0 warn-and-print, NOT exit 3 rescue).

This is the contract's explicit requirement: "an empty message means the commit would be aborted even under
--dry-run." The dryRun warn-and-print is for hook REJECTIONS (a non-empty candidate the user should see); an
empty message is always a hard abort. (research §1.)

---

## §3 — Placement: AFTER the hooks block, BEFORE `if dryRun` (the contract's exact spot)

**Decision:** the guard sits at L674-678 (between the hooks block's `}` and the dry-run comment). Rationale:
- It guards the FINAL `msg` unconditionally (whether hooks ran or not, whether dryRun or commit). If
  `deps.Hooks == nil`, msg is the EditMessage-validated/generated message (non-empty) ⇒ the guard is a no-op
  (zero behavioral change for the no-hooks case).
- It fires BEFORE the `if dryRun` early return ⇒ catches the dry-run case (the most important: proving dryRun
  does NOT swallow the abort).
- It fires BEFORE CommitTree ⇒ no commit created on the commit path (HEAD unchanged).
- Same placement S1 used in CommitStaged (after-block, before CommitTree) — consistency.

(The contract's "under the `else` branch where herr==nil" reading is equally valid but guards ONLY the
hooks-ran-success case; the after-block placement is strictly more defensive at zero cost — recommend after-block,
matching S1.)

---

## §4 — TDD: the dryRun test is the load-bearing one (proves the abort is NOT swallowed)

**Decision:** write the FAILING test FIRST, then the guard. TWO tests in `pkg/stagecoach/stagecoach_test.go`:

1. **TestGenerateCommit_DryRun_HookEmptiesMessage_AbortsExit1** — THE key test. `setupTestRepo` (stub Out a
   NON-empty message) + install a `commit-msg` hook that empties the file (`> "$1"; exit 0`) + `DryRun:true`.
   BEFORE the guard: runPipeline's dryRun path returns `Result{Message: ""}, nil` (exit 0 — the bug: an empty
   dry-run preview). AFTER the guard: returns `Result{}, generate.ErrEmptyMessage` (exit 1). Assert
   `errors.Is(err, generate.ErrEmptyMessage)`. This proves the empty-message abort is NOT swallowed by FR-V8a's
   warn-and-print (which only handles *RescueError; the emptying hook exits 0 ⇒ no RescueError ⇒ the `else`
   branch ⇒ msg="" ⇒ the guard fires).

2. **TestGenerateCommit_HookEmptiesMessage_NoCommit** — the commit-path test. `setupTestRepo` + the same
   emptying hook + `SystemExtra:"test"` (forces runPipeline with dryRun=false, since `!DryRun && SystemExtra==""`
   would delegate to CommitStaged). Assert `errors.Is(err, generate.ErrEmptyMessage)` + HEAD UNCHANGED (no
   commit created — the abort returned before CommitTree).

Both use the existing `setupTestRepo`/`writeFile`/`stageFile`/`headSHA` helpers. The hook is a REAL shell script
in `<repoDir>/.git/hooks/commit-msg` (chmod 0755) — `buildDeps` wires `deps.Hooks=DefaultRunner`, so
RunCommitHooks exec's it. Ensure `filepath` + `errors` are imported in stagecoach_test.go (add if missing).

---

## §5 — No new imports in stagecoach.go; go.mod UNCHANGED

**Decision:** stagecoach.go already imports `strings` (L15) + `generate` (L21). The guard adds NO import.
stagecoach_test.go may need `filepath` (for `filepath.Join(repoDir, ".git", "hooks")`) + `errors` (for
`errors.Is`) — add if not already imported. No new external dep; `go mod tidy` is a no-op.

---

## §6 — No conflict with S1 (sequential) or S3 (parallel-later)

**Decision:** S1 is DONE (generate.go shipped). S2 edits pkg/stagecoach/stagecoach.go (a DIFFERENT file) — zero
overlap with S1's generate.go. S3 (decompose.publishCommit) is a later task; S2 does NOT touch internal/decompose.
The three guards are independent (one per call site) and can land in any order.

---

## Summary table (the 6 calls at a glance)

| § | Decision | Source |
|---|----------|--------|
| 0 | runPipeline ONLY (pkg/stagecoach); CommitStaged=S1(done), publishCommit=S3 | contract |
| 1 | `if strings.TrimSpace(msg)=="" { return Result{}, generate.ErrEmptyMessage }` after the hooks block | S1 precedent |
| 2 | ErrEmptyMessage is BARE (not *RescueError) ⇒ exit 1 under both dryRun + commit (NOT warn-and-print exit 0) | contract |
| 3 | Placement: after the hooks block (L674), before `if dryRun` (L678) | contract + S1 |
| 4 | TDD: dryRun test (the key — abort NOT swallowed) + commit-path test (SystemExtra; no commit) | contract |
| 5 | No new imports (strings+generate present); test may need filepath+errors | stagecoach.go imports |
| 6 | No conflict: S2 is pkg/stagecoach only; S1=generate.go (done); S3=decompose (later) | scope |
