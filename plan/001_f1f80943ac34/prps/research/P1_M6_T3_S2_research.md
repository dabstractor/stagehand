# Research Note — P1.M6.T3.S2

**Task:** `internal/generate/integration_test.go` — end-to-end flow (temp-repo + stub)
driving the REAL `CommitStaged` against a REAL temp git repo + the REAL stub
agent binary, across all 7 contract paths.

This note captures the verified facts + the 4 non-obvious design decisions the
PRP depends on. Every signature below was read from shipped code on 2026-07-02.

---

## 0. What already exists (verified — do NOT recreate)

- `internal/generate/generate.go` — `CommitStaged(ctx, deps Deps) (Result, error)`,
  `Deps{Git gitClient; Runner runner; Manifest provider.Manifest; Config config.Config; Output *ui.Output}`,
  `Result{CommitSHA, Subject, Message}`, sentinels `ErrNothingToCommit`/`ErrRescue`/`ErrHeadMoved`.
  `gitClient` and `runner` are PACKAGE-PRIVATE interfaces (white-box only — so this
  test MUST be `package generate`, NOT `package generate_test`).
- `internal/generate/stubprovider_test.go` — EXPORTED-to-the-test-binary helpers
  the integration test composes with ONE call each:
  - `BuildStubBinary(t testing.TB) string` — compiles `./testdata/stubagent` once (sync.Once), returns path.
  - `NewStubManifest(t testing.TB, cfg StubConfig) provider.Manifest` — wires the stub as a real provider.Manifest (Command=stub binary, PromptDelivery=DeliveryStdin, Env carries STAGEHAND_STUB_*).
  - `StubConfig{Script []StubResponse; StateFile string; StdinLog string}` — Script MUST be non-empty; StateFile REQUIRED when len(Script)>1.
  - `StubResponse{Emit string; Hang bool; Fail int}` — precedence Hang > Fail>0 > Emit; JSON tags `{emit,hang,fail}` mirror the binary.
- `internal/generate/testdata/stubagent/main.go` — the fake-agent binary (package main, stdlib-only). Reads STAGEHAND_STUB_SCRIPT (JSON array), STAGEHAND_STUB_STATE (counter file; advances N→N+1 BEFORE acting), STAGEHAND_STUB_STDIN (optional capture). Selects script[min(N,len-1)] (CLAMP, never wrap).
- `internal/git` — real `*git.Git` with `New(dir)`, `RevParseHEAD`, `WriteTree`, `CommitTree`, `UpdateRefCAS`, `DiffTreeNameStatus`, `StagedDiff(DiffSettings)`, `CommitCount`, `RecentMessages`, `RecentSubjects`. The package-level git doc + the `*ExitError` type live here.
- `internal/provider` — real `*provider.Executor` with `NewExecutor(dir string)`, `Run(ctx,m,model,provider,sys,payload) (string,error)`, `Parse(raw,m) (string,bool)`.
- `internal/config.Default()` → Timeout=120s, MaxDuplicateRetries=3, MaxDiffBytes=300000, MaxMdLines=100, SubjectTargetChars=50.

## 1. DECISION A — repo setup: drive raw git via os/exec (the cross-package harness is unreachable)

The `internal/git` temp-repo harness (`newTempRepo`/`writeFileStage`/`seedCommits`
in `gittestutil_test.go`) is `package git` (WHITE-BOX). Its symbols are compiled
ONLY into `package git`'s test binary and are **NOT importable from
`package generate`**. Separately, `*git.Git` deliberately does NOT expose
`init` / `add <file>` / `commit` (stage.go documents that staging POLICY is
CLI-only; git.Git only exposes plumbing + primitives like AddAll).

⇒ The integration test must drive the REAL git binary directly via
`os/exec.Command("git", args...).Dir = dir` for repo bootstrap (init, user
identity, gpgsign=false, write+stage files, seed history). This mirrors the git
package's harness EXACTLY (same deterministic identity
`stagehand@example.com` / `Stagehand Test`, same repo-local
`commit.gpgsign=false` hardening). It is a SMALL local helper (a `gitRun(t,dir,args...)`
fail-fast wrapper + `seedCommit`/`writeStage`/`newTempRepo`-equivalents) — NOT a
re-implementation of plumbing (plumbing goes through the REAL `*git.Git` wired
into Deps.Git). PRD §20.1 layer 3 explicitly wants the REAL git binary for the
repo; this honors it.

## 2. DECISION B — the integration test is `package generate` (white-box)

Because `gitClient` and `runner` are package-private interfaces AND the stub
helpers live in `stubprovider_test.go` (`package generate`), the file MUST be
`package generate` (same as every sibling _test.go in this package). This lets
the CAS test wrap the real `*provider.Executor` in a test-local decorator that
implements the private `runner` interface (see DECISION C).

## 3. DECISION C — CAS-FAILURE is deterministic via a `headMover` runner decorator

The contract: "move HEAD mid-test (commit elsewhere) → UpdateRefCAS fails, HEAD
unchanged, ErrHeadMoved (no force)". The ONLY code that executes in the
[RevParseHEAD captures parentSHA] → [UpdateRefCAS] window is the generation loop
(i.e. `Runner.Run`). So HEAD must move DURING `Run`. Two options were weighed:

- (a) a `time.Sleep`-based background goroutine that commits — FLAKY (race vs the
  instant stub emit).
- (b) **a deterministic test-local decorator** `headMoverRunner{inner runner; dir string; moved bool}`
  implementing `runner`: on the FIRST `Run` it advances HEAD via PLUMBING
  (`commit-tree` on `HEAD^{tree}` + `update-ref HEAD <new>`, so the INDEX/staged
  file is UNTOUCHED), then delegates `Run`/`Parse` to the real `*provider.Executor`.

(b) is strictly better (deterministic, no flake). CRITICAL sub-gotcha: the
decorator MUST move HEAD with plumbing on `HEAD^{tree}` — NOT `git commit
--allow-empty`, which would consume/commit the test's STAGED file and corrupt
the index snapshot. After the move: parentSHA (old, captured by CommitStaged
before the loop) ≠ HEAD (new) → real `git update-ref HEAD <newSHA> <parentSHA>`
exits 128 → ErrHeadMoved, HEAD stays at the decorator's commit, the stagehand
commit object is DANGLING (unreachable). This is the REAL CAS failure path,
asserted with real git.

## 4. DECISION D — PARSE-FAIL-THEN-RESCUE uses Emit="" (empty), NOT "garbage"

`provider.parseOutput` returns `ok = (TrimSpace(msg) != "")` (parse.go line 75).
With `NewStubManifest` (Manifest.Output == "" ⇒ treated as RAW), a NON-EMPTY
"garbage" Emit parses to ok=true (msg = the garbage) → it would then pass dedupe
(if non-dup) and COMMIT, NOT rescue. The ONLY way to make Parse return ok=false
in the integration test is an EMPTY stdout, i.e. `StubResponse{Emit:""}` on BOTH
inner-try entries. This matches the unit test
(`generate_test.go` `TestCommitStaged_ParseFailAfterInner` uses `msg:"",ok:false`).
The contract's word "garbage" really means "empty/unparseable output"; a literal
non-empty garbage string is a trap that would silently commit. Document this.

---

## 5. Verified assertion mechanics (real git, no library)

- commit exists + is a commit: `git cat-file -t <sha>` → `commit\n`.
- commit's TREE: parse the `^tree ([0-9a-f]{40})$` line from `git cat-file -p <sha>`;
  compare to `gitRun(t,dir,"write-tree")` (re-snapshot the UNCHANGED index post-run).
- commit's PARENT: the `^parent ([0-9a-f]{40})$` line from `git cat-file -p <sha>`
  == oldHead captured before the run (root commit has NO parent line).
- commit's MESSAGE: `git log -1 --format=%B <sha>` (TrimSpace) == `Result.Message`.
- HEAD unchanged (failure paths): `git rev-parse HEAD` before == after.
- index unchanged (failure paths): `git diff --cached --name-only` before == after.
- short-SHA success line: stdout contains `[<sha[:7]>] <subject>`.

## 6. Per-test Deps wiring shape

```go
dir := t.TempDir()
gitRun(t, dir, "init", "-q")
gitRun(t, dir, "config", "user.email", "stagehand@example.com")
gitRun(t, dir, "config", "user.name", "Stagehand Test")
gitRun(t, dir, "config", "commit.gpgsign", "false")
// ... write + stage files, seed history as needed ...
g, _ := git.New(dir)
cfg := config.Default()
cfg.MaxDuplicateRetries = 3   // (or override per test)
cfg.Timeout = 500*time.Millisecond // (TIMEOUT test only)
state := filepath.Join(t.TempDir(), "c")
manifest := NewStubManifest(t, StubConfig{Script: [...], StateFile: state})
var stdout, stderr bytes.Buffer
deps := Deps{
    Git: g, Runner: provider.NewExecutor(""), Manifest: manifest,
    Config: cfg, Output: ui.NewOutput(&stdout, &stderr, false, true),
}
res, err := CommitStaged(context.Background(), deps)
```

## 7. The 7 contract paths → exact stub script + assertions (summary)

| Path | Repo | Script | Expected err | Key assertions |
|---|---|---|---|---|
| SUCCESS | ≥1 commit, staged file | `[{Emit:"feat: add feature\n\nBody."}]` | nil | commit exists; tree==write-tree; parent==oldHead; `%B`==Result.Message; stdout `[short] subj` |
| DUP-RETRY-THEN-SUCCESS | history w/ subject "feat: dup"; staged file | `[{Emit:"feat: dup"},{Emit:"feat: unique"}]` | nil | Result.Subject=="feat: unique"; commit message == unique |
| PARSE-FAIL-THEN-RESCUE | staged file | `[{Emit:""},{Emit:""}]` (EMPTY — Decision D) | ErrRescue | rescue rendered; NO commit; HEAD+index unchanged |
| TIMEOUT | staged file | `[{Hang:true}]`; cfg.Timeout=500ms | ErrRescue | rescue rendered; NO commit; HEAD+index unchanged |
| CAS-FAILURE | ≥1 commit, staged file; runner=headMover decorator | `[{Emit:"feat: ok"}]` | ErrHeadMoved | §13.5 msg printed; NO rescue; HEAD==decorator's commit (NOT stagehand's); stagehand commit dangling |
| ROOT-COMMIT | unborn repo, staged file | `[{Emit:"feat: initial"}]` | nil | commit has NO parent line; HEAD advanced from unborn; tree==write-tree |
| NOTHING-STAGED | ≥1 commit, NOTHING staged | `[{Emit:"ignored"}]` | ErrNothingToCommit | NO WriteTree-side effect (no dangling tree created); no rescue; HEAD+index unchanged |

Note: NOTHING-STAGED is asserted at the generate layer (CommitStaged returns
ErrNothingToCommit) — the CLI layer (P1.M7.T2), NOT generate, performs
auto-stage-all. This is the v2 seam (decisions.md §1).

## 8. DOCS impact

This task has no Mode-A per-item DOCS line (it is a test-only artifact). It
defers to the Mode-B changeset-level doc sync in M8. No doc file edit is required
in this task. The integration suite OUTPUT feeds the §18.1 invariant assertions
in P1.M6.T3.S3 (the next task).
