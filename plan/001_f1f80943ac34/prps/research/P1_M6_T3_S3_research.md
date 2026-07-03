# Research Notes — P1.M6.T3.S3 `internal/generate/invariants_test.go` (§18.1 safety property tests)

## 1. What this task IS
The **defensibility proof**: a dedicated test file that asserts PRD §18.1 (the invariant) holds
across **every** failure path. It is a *consumer* of the integration suite (P1.M6.T3.S2 =
`integration_test.go`), NOT a re-implementation. It reuses the in-package harness defined there and
adds (a) the exhaustive cross-path property assertions and (b) the static source-level tripwires.

## 2. The §18.1 invariant (authoritative text)
- **PRD §18.1**: "The repository's refs and index are modified only at the final `update-ref` step,
  and only if HEAD is unchanged since the snapshot. Every code path that does not reach a successful
  `update-ref` leaves the repository byte-for-byte unchanged (modulo harmless dangling objects)."
- **decisions.md §7**: "Idempotent index … Atomic HEAD … Snapshot immutability … Never call
  `git update-ref` without the expected-old (except root commit). Never `--force`. Never `git commit`."
- **PRD §20.2** (property tests): Idempotent index = `git diff --cached --name-only` before==after;
  Atomic HEAD = `git rev-parse HEAD` unchanged after CAS failure; Snapshot immutability =
  `git cat-file -p <TREE_SHA>` stable regardless of later staging.

## 3. The reusable in-package harness (from `integration_test.go` — the INPUT)
`invariants_test.go` MUST be `package generate` (white-box, same as integration_test.go /
stubprovider_test.go) so it can call these **already-defined, battle-tested** helpers directly — zero
duplication, exactly as the contract's "INPUT: the integration suite's failure-path scenarios" intends:

| helper (in `package generate` test files) | purpose |
|---|---|
| `gitRun(tb, dir, args...)` | fail-fast raw-git exec seam (bootstrap + assertions) |
| `newTempRepo(t)` | fresh unborn temp repo + deterministic identity + gpgsign=false |
| `writeStage(t, dir, path, content)` | write+`git add` a file (the staged snapshot) |
| `seedCommit(t, dir, msg)` | one deterministic parent commit |
| `e2eDeps(t, dir, script, cfg, stdout, stderr)` | Deps wired to REAL *git.Git + REAL *provider.Executor + stub Manifest |
| `headMoverRunner{inner, tb, dir}` | decorator that moves HEAD mid-Run (the CAS-fail driver) |
| `NewStubManifest(t, StubConfig{...})` / `StubResponse{Emit,Hang,Fail}` | the stub agent config |
| `headSHA`, `stagedFiles`, `assertHeadAndIndexUnchanged`, `containsAll`, `rescueRendered` | assertion helpers |
| sentinels `ErrRescue`, `ErrHeadMoved`, `ErrNothingToCommit` | from generate.go |

## 4. CRITICAL nuance — "Atomic HEAD" on the CAS-fail path (do NOT assert before==after)
The CAS-failure is *caused* by a concurrent HEAD movement (the `headMoverRunner` advances HEAD on
`HEAD^{tree}` during `Runner.Run`). So `beforeHead != afterHead` BY CONSTRUCTION. The §20.2 phrasing
"`git rev-parse HEAD` unchanged after a CAS failure" means **Stagehand's failed `update-ref` was a
no-op** (git refused the swap) — i.e. HEAD was left exactly where the concurrent mover put it, NOT
force-advanced to the stagehand commit. The correct assertion for the CAS path is therefore:
- `afterHead == <the decorator's concurrent commit>` (subject == "concurrent commit elsewhere"), AND
- the stagehand commit object exists but is **dangling** (unreachable from HEAD / any ref).

`TestIntegration_HeadMovedCASFailure` already encodes this; the invariant test mirrors it. The
simple `assertHeadAndIndexUnchanged` (before==after) is ONLY valid for the non-CAS failure paths
(parse-fail, timeout, dup-exhaustion) where nothing moves HEAD.

## 5. Failure-path matrix (the scenarios to assert across)
| path | stub script | deps | expected error | index-idempotent | atomic-HEAD assertion |
|---|---|---|---|---|---|
| parse-fail (inner exhausted) | `[{Emit:""},{Emit:""}]` | `e2eDeps` | `ErrRescue` | before==after | before==after (nothing moved) |
| timeout | `[{Hang:true}]`, cfg.Timeout=short | `e2eDeps` | `ErrRescue` | before==after | before==after |
| dup-exhaustion | all dups across `MaxDuplicateRetries+1` | `e2eDeps` | `ErrRescue` | before==after | before==after |
| CAS-fail | `[{Emit:"feat: ok"}]` | `headMoverRunner` | `ErrHeadMoved` | before==after | stagehand commit DANGLING, HEAD==decorator's commit |

(decision D from S2: under RAW output a non-empty string parses ok=true, so parse-fail needs `Emit:""`.)

## 6. Snapshot immutability — standalone proof
Pure git-content-addressable property, testable WITHOUT the full orchestrator:
1. seed repo + stage `a.go`; `deps.Git.WriteTree()` (the REAL wrapper) → `treeSHA`.
2. `before = gitRun(dir,"cat-file","-p",treeSHA)`.
3. `writeStage("b.go",...)` (simulate concurrent staging mid-run).
4. `after = gitRun(dir,"cat-file","-p",treeSHA)`.
5. assert `before == after` (the frozen tree is immutable). Bonus: a fresh `WriteTree()` now yields a
   *different* SHA (proving the index DID change but the OLD snapshot is frozen).

## 7. Static tripwires — AST-based (robust, exhaustive, stdlib-only)
Enumerated the COMPLETE production git surface (`g.run`/`exec.Command` in non-test internal/git +
cmd/stagehand): subcommands used are exactly `diff, rev-list, log, rev-parse, write-tree, commit-tree,
update-ref, diff-tree, add`. **No standalone `"commit"` token, no `"--force"`.** The 1-arg update-ref
form is guarded by `if expected != ""` in `UpdateRefCAS` (plumbing.go:143-150).

**Naive `strings.Contains(src,"commit")` is USELESS** — internal/git source is full of comments like
"Stagehand NEVER runs `git commit`". So the static checks parse the source with **`go/parser`+`go/ast`**
(stdlib only — matches the no-external-deps stance), walk every `*ast.CompositeLit` `[]string{...}`
element + every direct arg of `g.run`/`exec.Command` calls, collect the string literals, and assert:
- none == `"commit"` (no git commit; `"commit-tree"` ≠ `"commit"` so safe),
- none == `"--force"` (never force),
- `"update-ref"` appears in exactly ONE composite literal (UpdateRefCAS) whose file also contains the
  `if expected != ""` guard text (1-arg form only reachable for root).

Files to scan: non-test `*.go` under `internal/git/` (and optionally the whole module). A test reads
them via `filepath.Glob(filepath.Join("..","git","*.go"))` (Go test CWD == package dir; `go:embed`
can't reach `..`).

## 8. Validation tooling (verified)
Makefile: `test`=`go test ./...`, `vet`=`go vet ./...`, `fmt`=`gofmt -s -w .`, `lint`=`golangci-lint run`.
Baseline: `go vet ./internal/generate/` clean; `go test ./internal/generate/` → `ok … 1.281s`.
NO build tag on integration_test.go → invariant tests run in normal CI (only `integration_real` is
opt-in). The new file uses NO build tag.

## 9. DOCS impact
None. This is test-infra only (proves §18.1; changes no shipped behavior / user-facing docs). The
§18.1 invariant text in PRD.md IS the documentation; these tests are its executable proof.
