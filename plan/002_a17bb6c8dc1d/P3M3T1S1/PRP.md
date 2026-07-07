---
name: "P3.M3.T1.S1 — Implement internal/decompose/arbiter.go: arbiter agent call + JSON parse + in-list validation (PRD §13.6.5, §9.14 FR-M9, §17.7)"
description: |

  CREATE ONE NEW FILE `internal/decompose/arbiter.go` (package `decompose`, the 5th file after the
  shipped roles.go, planner.go, stager.go and the in-flight message.go) and ONE NEW TEST FILE
  `arbiter_test.go`. arbiter.go is the arbiter half of the multi-commit decomposition pipeline
  (PRD §13.6.5): `runArbiter` is the BARE arbiter-role invocation that is the decompose analogue of
  callPlanner's Render→Execute→parse pattern, specialized to the arbiter's `{"target": "<sha>" | null}`
  JSON output contract. It converts the run's commits ([]CommitInfo, carrying SHAs + subjects + diff-tree
  file-lists) into `[]prompt.ArbiterCommit` (the FileChange→path-string seam), builds the §17.7 prompt
  (BuildArbiterSystemPrompt zero-arg + BuildArbiterUserPayload), Renders the resolved arbiter manifest
  in BARE mode, Executes ONCE (no retry — §17.7 defines no retry instruction), parses
  `prompt.ParseArbiterOutput`, and validates the target is one of THIS run's commits (§13.6.5 "may only
  target a commit from this run"). It returns a confident in-list target, or degrades to null
  (ArbiterOutput{nil}) on ANY indecision (parse failure, timeout, cancel, empty/not-in-list target) — the
  arbiter OWNS the §13.6.5 "when in doubt, null" decision rather than punting to the resolution logic.
  Only a render error returns a wrapped ErrArbiterFailed. runArbiter performs ZERO git reads (the
  orchestrator pre-computes the []CommitInfo via DiffTree and the leftoverDiff via WorkingTreeDiff, and
  gates the call on StatusPorcelain != "" per FR-M9). It ONLY DECIDES; resolution (new commit / tip amend
  / mid-chain rebuild) is P3.M3.T2.S1 ("S2"). Consumed by the orchestrator (P3.M4.T1.S1); NO caller wiring.

  CONTRACT (P3.M3.T1.S1, verbatim from the work item):
    1. RESEARCH NOTE: The arbiter (§13.6.5, FR-M9) is bare and runs only if StatusPorcelain (P2.M2.T2.S1)
       is non-empty after the loop. It receives: SHAs, subjects, and file-lists (diff-tree) of every
       commit made this run, plus a diff of remaining changes (WorkingTreeDiff). Returns JSON:
       {"target": "<sha>"} or {"target": null}. Ambiguous → null. May only target a commit from this run.
       The arbiter only DECIDES; stagecoach performs all git (FR-M10). Output is parsed via
       ParseArbiterOutput (P3.M1.T1.S3).
    2. INPUT: prompt/arbiter.go from P3.M1.T1.S3, StatusPorcelain from P2.M2.T2.S1, the list of commits
       made this run (SHAs, subjects, file-lists).
    3. LOGIC: Create internal/decompose/arbiter.go. Implement
       `func runArbiter(ctx, deps Deps, commits []CommitInfo, leftoverDiff string) (prompt.ArbiterOutput,
       error)`: build system prompt (BuildArbiterSystemPrompt), build user payload (BuildArbiterUserPayload
       with commit list + diff), Render with bare mode, Execute, ParseArbiterOutput. If the returned
       target SHA is not in the commits-made list → treat as null (ambiguous). Define
       `type CommitInfo struct { SHA, Subject string; Files []git.FileChange }`.
    4. OUTPUT: runArbiter returns a target SHA (or nil for new commit). Consumed by the resolution logic (S2).
    5. DOCS: none — internal agent call.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/decompose/roles.go — SHIPPED (P3.M2.T1.S1). Defines Deps {Git, Registry, Config, Roles
      RoleManifests, Verbose}, RoleManifests{Planner,Stager,Message,Arbiter}. CONSUMED: deps.Roles.Arbiter
      (the BARE arbiter manifest) + deps.Config + deps.Verbose. Deps has NO Models field (runArbiter
      derives the arbiter (provider, model) via ResolveRoleModel — see findings §5/§8).
    - internal/decompose/planner.go — SHIPPED (P3.M2.T2.S1). CONSUMED as the SIBLING PATTERN (callPlanner's
      Render(RenderBare)→Execute→handle pattern + ErrPlannerFailed sentinel + ResolveRoleModel derivation).
      Do NOT edit. runArbiter mirrors callPlanner MINUS the retry loop (the arbiter is single-shot).
    - internal/decompose/stager.go — SHIPPED (P3.M2.T3.S1). CONSUMED as a sibling sentinel/convention
      reference (ErrStagerFailed). Do NOT edit.
    - internal/decompose/message.go — IN-FLIGHT PARALLEL (P3.M2.T4.S1, assumed shipped per the parallel
      context). CONSUMED as a sibling reference (ErrMessageFailed/ErrPublicationFailed). Do NOT edit.
    - internal/prompt/arbiter.go — SHIPPED (P3.M1.T1.S3). CONSUMED VERBATIM: BuildArbiterSystemPrompt (zero-
      arg), BuildArbiterUserPayload, ParseArbiterOutput, ArbiterCommit, ArbiterOutput.
    - internal/git/git.go — CONSUMED: the git.FileChange type (CommitInfo.Files) + the DiffTree/StatusPorcelain/
      WorkingTreeDiff contracts (the orchestrator uses these to BUILD the []CommitInfo + leftoverDiff params;
      runArbiter does NOT call them). EmptyTreeSHA NOT needed.
    - internal/provider/{render,executor}.go — CONSUMED: Manifest.Render(...,RenderBare),
      provider.Execute(ctx,spec,timeout,vb) → (stdout,stderr,err).
    - internal/config/{config,roles}.go — CONSUMED: Config.Timeout, ResolveRoleModel("arbiter",cfg).
    - internal/decompose/{chain,decompose}.go — DO NOT EXIST YET. chain.go (P3.M3.T2.S1 — the "S2"
      resolution), decompose.go (P3.M4.T1.S1 — the orchestrator). This task creates ONLY arbiter.go.
    - internal/signal/* — DO NOT IMPORT. runArbiter is SIGNAL-FREE (a pure decide-and-return call).
    - cmd/, pkg/stagecoach/ — UNCHANGED (the orchestrator P3.M4.T1.S1 wires runArbiter).

  DELIVERABLES (2 new files, 0 edits to existing files, 0 breaking changes):
    CREATE internal/decompose/arbiter.go — package `decompose`; ErrArbiterFailed sentinel; type
      CommitInfo{SHA,Subject,Files []git.FileChange}; runArbiter (the bare arbiter invocation + parse +
      in-list validation); a private convertArbiterCommits helper (the FileChange→path-string seam) +
      targetInRun validator.
    CREATE internal/decompose/arbiter_test.go — stubtest-driven + real-git integration tests. Fixture
      helpers use DISTINCT arb*-prefixed names (parallel-safe vs planner_test.go's un-prefixed +
      stager_test.go's stg* + message_test.go's msg* — findings §11).

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing tests still pass; runArbiter
  returns ArbiterOutput{&sha} on a confident in-list target; runArbiter returns ArbiterOutput{nil} (null)
  on parse-failure / timeout / cancel / empty-target / target-not-in-list (graceful, NOT an error);
  runArbiter returns a wrapped ErrArbiterFailed ONLY on a render error; runArbiter uses RenderBare (the
  arbiter manifest) and derives the model via ResolveRoleModel("arbiter"); runArbiter performs NO retry
  (single attempt); runArbiter performs ZERO git reads (commits + leftoverDiff are params); the
  CommitInfo→ArbiterCommit conversion maps git.FileChange.Path (NOT Status/SrcPath) into the []string
  payload; only 2 git changes (arbiter.go, arbiter_test.go).

---

## Goal

**Feature Goal**: Implement the arbiter agent invocation for multi-commit decomposition leftover
reconciliation (PRD §13.6.5 / FR-M9) as a self-contained module `internal/decompose/arbiter.go`.
`runArbiter(ctx, deps, commits []CommitInfo, leftoverDiff string)` is the BARE arbiter-role call — the
decompose analogue of callPlanner's Render→Execute→parse pattern, specialized to the arbiter's
`{"target": "<sha>" | null}` JSON output contract: it converts the run's commits ([]CommitInfo, each
carrying SHA + Subject + a `[]git.FileChange` diff-tree file-list) into `[]prompt.ArbiterCommit` (the
FileChange→path-string seam), builds the §17.7 prompt (BuildArbiterSystemPrompt zero-arg +
BuildArbiterUserPayload), Renders the resolved arbiter manifest in BARE mode, Executes ONCE (no retry —
§17.7 defines no retry instruction; the arbiter is "when in doubt, null"), parses
`prompt.ParseArbiterOutput`, and validates the returned target is one of THIS run's commits (§13.6.5 "may
only target a commit from this run"). It returns a confident in-list target, or degrades to null
(`prompt.ArbiterOutput{Target: nil}`) on ANY indecision (parse failure, timeout, cancel, empty target,
target-not-in-list) — the arbiter OWNS the §13.6.5 "when in doubt, prefer a NEW commit (return null)"
decision rather than punting it to the resolution logic. Only a render error returns a wrapped
`ErrArbiterFailed`. runArbiter performs ZERO git reads: the orchestrator pre-computes the []CommitInfo
(via DiffTree) and the leftoverDiff (via WorkingTreeDiff), and gates the call on `StatusPorcelain != ""`
per FR-M9. It ONLY DECIDES — resolution (new commit / tip amend / mid-chain chain rebuild) is P3.M3.T2.S1.

**Deliverable** (2 new files in the existing `decompose` package):
1. `internal/decompose/arbiter.go` — `ErrArbiterFailed` sentinel; `type CommitInfo struct { SHA, Subject
   string; Files []git.FileChange }`; `runArbiter(ctx context.Context, deps Deps, commits []CommitInfo,
   leftoverDiff string) (prompt.ArbiterOutput, error)`; private `convertArbiterCommits` + `targetInRun`.
2. `internal/decompose/arbiter_test.go` — stubtest-driven + real-git integration tests against a real temp
   git repo (fixture helpers with DISTINCT arb*-prefixed names).

**Success Definition**:
- Confident target (happy path): commits=[{shaA,"feat: a",[FileChange{Path:"a.go"}]},{shaB,...}], stub
  emits `{"target": "<shaA>"}` ⇒ runArbiter returns ArbiterOutput whose Target is non-nil and points at
  shaA; nil error.
- Null target: stub emits `{"target": null}` ⇒ ArbiterOutput{Target: nil}, nil error.
- Target-NOT-in-list (the contract's load-bearing in-list check): stub emits `{"target": "<bogus-sha>"}`
  where bogus-sha is NOT in commits ⇒ ArbiterOutput{Target: nil}, nil error (ambiguous→null; NOT an error).
- Empty target string: stub emits `{"target": ""}` ⇒ ArbiterOutput{Target: nil}, nil error.
- Parse failure → null: stub emits `"not json at all"` ⇒ ArbiterOutput{Target: nil}, nil error (graceful
  degradation; NOT ErrArbiterFailed — the arbiter owns the null decision).
- Timeout → null: stub SleepMS=2000 with cfg.Timeout=100ms ⇒ ArbiterOutput{Target: nil}, nil error
  (graceful; NOT an error).
- Non-zero exit but valid stdout: stub Exit=1, Out=`{"target": "<shaA>"}` ⇒ ArbiterOutput{Target: &shaA},
  nil error (falls through to parse; partial-but-valid stdout accepted — mirrors planner/generate).
- Render error → ErrArbiterFailed: a manifest whose Render fails ⇒ ArbiterOutput{}, non-nil err;
  `errors.Is(err, ErrArbiterFailed)` true.
- Conversion seam: the rendered payload (captured via the stub's stdin) contains each commit's SHA +
  Subject + each `git.FileChange.Path` (Status/SrcPath NOT present), the §17.7 headers, and the
  leftoverDiff tail verbatim.
- runArbiter uses RenderBare (the arbiter manifest) and derives the model via
  `config.ResolveRoleModel("arbiter", deps.Config)`.
- runArbiter performs NO retry (exactly ONE Execute call per invocation — assert via the stub's call
  counter); runArbiter performs ZERO git reads (deps.Git is unused in the happy path).
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 2 new files, nothing else.

## User Persona

**Target User**: the decompose orchestrator (`internal/decompose/decompose.go`, P3.M4.T1.S1) and the
arbiter resolution logic (`internal/decompose/chain.go`, P3.M3.T2.S1 "S2"), and by extension the end user
running `stagecoach` on an un-staged working tree (the default action routes to decompose per FR-M1).
arbiter.go is internal plumbing — NOT user-facing CLI text. The user never invokes the arbiter directly;
the orchestrator calls runArbiter ONCE, after the per-concept loop, only when `StatusPorcelain != ""`
(some changes no stager claimed). runArbiter decides whether those leftovers fold into an existing
run-commit or warrant a new one; S2 acts on that decision.

**Use Case**: after the per-concept loop publishes commits 0..N-1, the orchestrator checks
`StatusPorcelain(ctx)`. If non-empty, it (a) computes the leftoverDiff via `WorkingTreeDiff`, (b) builds
`[]CommitInfo` from each run-commit's SHA + Subject + `DiffTree(sha, isRoot)` file-list, and (c) calls
`out, err := runArbiter(ctx, deps, commits, leftoverDiff)`. runArbiter returns a decision: a non-nil
Target (one of the run's SHAs) ⇒ S2 amends/rebuilds; a nil Target (or an error, which S2 also treats as
null) ⇒ S2 stages the leftovers, snapshots, generates a message, and publishes an (N+1)-th commit. If
`StatusPorcelain == ""` (the perfect run), runArbiter is never called (§13.6.5).

**Pain Points Addressed**: (a) leftover changes that no stager claimed must not be silently dropped — the
arbiter decides whether they fold into an existing run-commit (amend/rebuild) or get their own new commit,
so NO working-tree change is ever lost; (b) the arbiter ONLY decides (FR-M10) — it has no git access and
cannot corrupt history; stagecoach performs ALL ref mutations; (c) an indecisive / unparseable / out-of-
range arbiter output degrades SAFELY to "new commit" (§13.6.5 "when in doubt, null"), so a flaky model
never blocks the leftovers from being committed; (d) the "may only target a commit from this run" guard
(§13.6.5) is enforced IN runArbiter — a hallucinated or stale SHA is treated as null, never honored.

## Why

- **Closes the arbiter half of PRD §13.6.5 / §9.14 FR-M9.** The arbiter is the fourth and final decompose
  role (planner bare, stager tooled, message bare, arbiter bare). It reconciles leftover changes that no
  stager claimed into the just-made commit set. This task is the literal invocation + parse + in-list
  validation implementation. With it, S2 (P3.M3.T2.S1) has its decision input; the orchestrator
  (P3.M4.T1.S1) has its arbiter entry point.
- **The bare, single-shot variant of the proven callPlanner pattern.** callPlanner already does "Render
  bare → Execute → parse JSON → (retry once)" for the planner role. runArbiter is the SAME algorithm with
  THREE arbiter-specific deltas: (1) NO retry (§17.7 defines no retry instruction; "when in doubt, null");
  (2) graceful degradation to null on ANY indecision (the arbiter's failure mode is benign — null ⇒ new
  commit, no work lost — unlike the planner's load-bearing non-rescue abort); (3) the in-list target
  validation (§13.6.5 "may only target a commit from this run"). No new concept — the snapshot/CAS
  machinery is untouched (the arbiter performs ZERO git reads and ZERO ref mutations).
- **Unblocks arbiter resolution (P3.M3.T2.S1) + the orchestrator (P3.M4.T1.S1).** S2 consumes runArbiter's
  ArbiterOutput to drive new-commit / tip-amend / mid-chain-rebuild. The orchestrator cannot complete the
  decompose pipeline until runArbiter exists (the leftovers branch). This is the 5th foundation file of
  the `internal/decompose/` package (roles.go, planner.go, stager.go, message.go, arbiter.go).
- **Lowest-risk, maximal-reuse, backward-compatible.** Two NEW files in an EXISTING package (decompose);
  ZERO edits to any shipped file (roles/planner/stager/prompt/git/provider/config all CONSUMED).
  go.mod/go.sum untouched (config/git/prompt/provider already imported by roles.go/planner.go; git is
  imported for FileChange — already a decompose import via roles.go's Deps.Git field). No import cycle
  (decompose → git/config/prompt/provider is one-way). runArbiter is consumed later (P3.M4.T1.S1); no
  caller wiring here → zero merge friction.

## What

One new file `internal/decompose/arbiter.go` in the existing `decompose` package exporting one sentinel +
one type + one function (+ private helpers), and one new test file. No new dependencies. No caller wiring
(that is P3.M4.T1.S1). Specifically:

- **`ErrArbiterFailed`** (exported package-level sentinel): `errors.New("decompose: arbiter failed")`.
  Wrapped (%w) around the ONE true infra failure — a render error (the arbiter manifest could not be
  rendered; near-impossible post-ResolveRoles, but wrapped for consistency with the sibling sentinels
  ErrPlannerFailed/ErrStagerFailed/ErrMessageFailed + verbose logging). The orchestrator/S2 treats ANY
  runArbiter error as null (defensive), but in practice runArbiter only errors on render. Agent failures
  (timeout/cancel/parse-fail) and semantic ambiguity (empty/not-in-list target) do NOT return errors —
  they degrade to `ArbiterOutput{nil}` (the arbiter OWNS the null decision — findings §7).
- **`type CommitInfo struct { SHA, Subject string; Files []git.FileChange }`** (exported): one commit made
  this run, as the orchestrator builds it from `DiffTree(sha, isRoot)` output. SHA is the full commit SHA
  (40/64 hex — the value the arbiter may return as "target"); Subject is the commit's subject line;
  Files is the diff-tree file-list (`[]git.FileChange`, the orchestrator passes DiffTree's return verbatim).
  runArbiter converts these to `[]prompt.ArbiterCommit` (Files []string) — see convertArbiterCommits.
- **`runArbiter(ctx context.Context, deps Deps, commits []CommitInfo, leftoverDiff string) (prompt.ArbiterOutput,
  error)`**: the bare arbiter invocation. Pipeline (findings §6): derive arbiter (provider, model) via
  `config.ResolveRoleModel("arbiter", deps.Config)` → convert []CommitInfo → []prompt.ArbiterCommit +
  build the valid-SHA set → BuildArbiterSystemPrompt (zero-arg) → BuildArbiterUserPayload(commits, diff) →
  Render BARE → Execute ONCE → on timeout/cancel return null; on non-zero exit fall through to parse →
  ParseArbiterOutput → on parse error return null → validate target in valid-SHA set → return the
  ArbiterOutput (confident target) or null (empty/not-in-list). Render error → wrapped ErrArbiterFailed.
- **`convertArbiterCommits(commits []CommitInfo) []prompt.ArbiterCommit`** (private): the FileChange→path
  seam. For each CommitInfo, map `[]git.FileChange` → `[]string` via `.Path` (Path is always set;
  Status/SrcPath are NOT part of the arbiter payload — ArbiterCommit.Files doc: "diff-tree --name-only").
  Preserve SHA + Subject verbatim.
- **`targetInRun(target string, validSHAs map[string]struct{}) bool`** (private, OR inline): the §13.6.5
  "may only target a commit from this run" check. Exact membership of `target` in the set of run-commit
  full SHAs. Empty target ⇒ false. (The arbiter is instructed to copy a SHA "from the list" verbatim per
  §17.7, so exact match is correct + deterministic; a truncated/non-matching target ⇒ false ⇒ null.)

### Success Criteria

- [ ] `internal/decompose/arbiter.go` is package `decompose`, has a file doc comment citing PRD §13.6.5 +
      §9.14 FR-M9 + §17.7, and defines `ErrArbiterFailed` + `type CommitInfo` + `runArbiter` EXACTLY as
      the contract (signature `runArbiter(ctx context.Context, deps Deps, commits []CommitInfo,
      leftoverDiff string) (prompt.ArbiterOutput, error)`).
- [ ] runArbiter uses the arbiter manifest from `deps.Roles.Arbiter` (BARE mode) and derives the arbiter
      (provider, model) via `config.ResolveRoleModel("arbiter", deps.Config)` (Deps has no Models field —
      findings §5/§8).
- [ ] runArbiter converts []CommitInfo → []prompt.ArbiterCommit via convertArbiterCommits, mapping each
      `git.FileChange.Path` into the `[]string` payload (Status/SrcPath dropped), and builds the §17.7
      prompt (BuildArbiterSystemPrompt zero-arg + BuildArbiterUserPayload(commits, leftoverDiff)).
- [ ] runArbiter Renders `deps.Roles.Arbiter.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)`,
      Executes via `provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)` ONCE (NO retry), and
      parses via `prompt.ParseArbiterOutput`.
- [ ] runArbiter returns the ArbiterOutput (non-nil Target) on a confident in-list target; returns
      `prompt.ArbiterOutput{Target: nil}` (null) on parse-failure / timeout / context-cancel / empty
      target / target-not-in-list — all with a NIL error (graceful degradation per §13.6.5).
- [ ] runArbiter returns `prompt.ArbiterOutput{}, fmt.Errorf("%w: ...", ErrArbiterFailed, cause)` ONLY on a
      render error; the error is `errors.Is`-able to ErrArbiterFailed. No other path returns an error.
- [ ] runArbiter performs NO git reads (deps.Git is unused in the function body — commits + leftoverDiff
      are params; the StatusPorcelain trigger is the orchestrator's gate, not runArbiter's).
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED; only 2 git changes (arbiter.go, arbiter_test.go).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the contract + scope boundary
(findings §1/§2); the prompt/arbiter.go API surface including the FileChange→path-string seam (findings §3/§4);
the NOW-SHIPPED Deps shape (no Models field) and the model-derivation decision (findings §5/§8); the exact
execution pattern to mirror from callPlanner with the arbiter's deltas (findings §6); the ERROR CONTRACT —
the arbiter OWNS the null decision (findings §7); the SHA in-list validation (findings §9); the no-retry
confirmation (findings §10); the imports + no-cycle (findings §13); the test-fixture collision rule (arb*
prefix — findings §11); the test cases (findings §12); the validation gates (findings §13). No prior
decompose knowledge beyond roles.go's Deps is required — runArbiter is fully self-contained.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (contract + 14 sections of load-bearing facts)
- docfile: plan/002_a17bb6c8dc1d/P3M3T1S1/research/findings.md
  why: §1 the verbatim contract + scope boundary; §2 SCOPE (this task = runArbiter the INVOCATION only,
       NOT resolution — S2/P3.M3.T2.S1 owns new-commit/tip-amend/mid-chain-rebuild; runArbiter performs
       ZERO git reads); §3 the prompt/arbiter.go API (BuildArbiterSystemPrompt zero-arg, BuildArbiterUserPayload,
       ParseArbiterOutput, ArbiterCommit/ArbiterOutput; §17.7 defines NO retry instruction); §4 the ONE type
       seam (CommitInfo.Files []git.FileChange → ArbiterCommit.Files []string via .Path); §5 the SHIPPED Deps
       shape (NO Models field); §6 the runArbiter body (mirror callPlanner MINUS retry + graceful null);
       §7 the ERROR CONTRACT (the arbiter OWNS the null decision — render error is the ONLY ErrArbiterFailed);
       §8 ResolveRoleModel + Config.Timeout; §9 git.FileChange + the exact-match SHA validation; §10 no-retry
       (3 confirming signals); §11 the test-fixture arb*-prefix rule; §12 the test cases; §13 validation gates;
       §14 the one-paragraph summary.
  critical: §2 (runArbiter is the INVOCATION only — do NOT implement resolution or call StatusPorcelain/
            WorkingTreeDiff/DiffTree; the orchestrator pre-computes commits + leftoverDiff); §7 (parse-fail/
            timeout/cancel/empty/not-in-list → ArbiterOutput{nil} with NIL error — do NOT wrap them in
            ErrArbiterFailed; the arbiter OWNS the null decision per §13.6.5; ONLY render error returns a
            wrapped ErrArbiterFailed); §10 (NO retry — single attempt; §17.7 has no retry instruction);
            §4 (map git.FileChange.Path into []string — Status/SrcPath are NOT in the arbiter payload);
            §11 (arbiter_test.go MUST use arb*-prefixed fixture names — planner=unprefixed, stager=stg*,
            message=msg* all collide in package decompose).

# MUST READ — the SHIPPED prompt/arbiter.go (P3.M1.T1.S3) — runArbiter's CONSUMED API
- file: internal/prompt/arbiter.go
  section: BuildArbiterSystemPrompt() (ZERO-arg — §17.7 has no <style examples> placeholder, unlike §17.5
           planner); BuildArbiterUserPayload(commits []ArbiterCommit, leftoverDiff string) (assembles
           commitsHeader + per-commit blocks [SHA\nSubject\nfiles...] + leftoverHeader + leftoverDiff verbatim);
           ParseArbiterOutput(raw) (ArbiterOutput, error) (whole-string json.Unmarshal then brace-balanced
           fallback via extractJSONObject defined in planner.go — REUSED; returns error on parse failure; does
           NOT validate target-in-list); type ArbiterCommit{SHA, Subject string; Files []string} (Files =
           "diff-tree --name-only" PATHS, not FileChange); type ArbiterOutput{Target *string `json:"target"`}
           (nil ⇔ null ⇔ new commit; &"<sha>" ⇔ amend).
  why: these are the EXACT symbols runArbiter calls. The FileChange→[]string conversion exists because
       ArbiterCommit.Files is []string while CommitInfo.Files is []git.FileChange. ParseArbiterOutput does NOT
       validate target-in-list ("the caller owns that") — encode it in targetInRun.
  gotcha: §17.7 defines NO retry instruction — there is NO ArbiterRetryInstruction constant (only
          PlannerRetryInstruction exists). This is a deliberate design signal: the arbiter is single-shot.

# MUST READ — the SHIPPED Deps/RoleManifests (P3.M2.T1.S1) — runArbiter's input contract
- file: internal/decompose/roles.go
  section: type Deps struct { Git git.Git; Registry *provider.Registry; Config config.Config; Roles
           RoleManifests; Verbose *ui.Verbose } — the injectable collaborators. RoleManifests{Planner, Stager,
           Message, Arbiter provider.Manifest} — the arbiter manifest is deps.Roles.Arbiter (BARE per the
           RoleManifests doc: "Arbiter provider.Manifest // bare").
  why: confirms Deps has Config + Roles + Git + Verbose but NO Models field (findings §5). runArbiter reads
       deps.Roles.Arbiter (the bare manifest) and derives the (provider, model) from deps.Config. deps.Git is
       available but UNUSED by runArbiter (the orchestrator pre-computes commits + leftoverDiff — findings §2).
       Do NOT edit this file (shipped; editing = conflict).

# MUST READ — the sibling pattern (callPlanner) — the structure runArbiter mirrors (minus retry)
- file: internal/decompose/planner.go
  section: callPlanner — the bare counterpart of runArbiter. The model derivation via ResolveRoleModel, the
           Render(RenderBare)→Execute→parse→handle pattern, the ErrPlannerFailed sentinel, the execErr handling
           (DeadlineExceeded/Canceled ⇒ immediate; non-zero exit ⇒ fall through to parse).
  why: runArbiter mirrors callPlanner's structure (derive model → build prompt → Render BARE → Execute →
       parse) BUT runArbiter has NO retry loop (single attempt — §17.7 has no retry instruction), parses via
       prompt.ParseArbiterOutput (not ParsePlannerOutput), and degrades to null on ANY indecision (callPlanner
       wraps failures in ErrPlannerFailed). The ErrArbiterFailed sentinel mirrors ErrPlannerFailed. The
       ResolveRoleModel derivation + Render arg order + `*spec` deref + Execute 3-tuple handling are IDENTICAL.
- file: internal/decompose/stager.go
  section: ErrStagerFailed sentinel + stageConcept (the tooled sibling). Confirms the package conventions
           (file doc comment citing PRD sections; sentinel + function(s); model derivation via ResolveRoleModel;
           Render mode explicit; %w wrapping).
  why: confirms the sentinel-naming convention (Err<Role>Failed) and the Render→Execute→handle shape. The
       arbiter is the BARE analogue (RenderBare, like planner/message).

# MUST READ — git.FileChange (the type in CommitInfo.Files) + the orchestrator-side git contracts
- file: internal/git/git.go
  section: type FileChange struct { Status, SrcPath, Path string } (line 18) — Path is ALWAYS set (the
           destination path); SrcPath is non-empty only for R/C renames; Status is A/M/D/R/C/T/U. The arbiter
           payload uses ONLY Path (ArbiterCommit.Files []string = "diff-tree --name-only" paths). DiffTree(ctx,
           sha, isRoot) ([]FileChange, error) — how the orchestrator BUILDS CommitInfo.Files (NOT called by
           runArbiter). StatusPorcelain(ctx) (string, error) — the FR-M9 arbiter TRIGGER the orchestrator gates
           on (output != "" ⇒ call runArbiter; NOT called by runArbiter). WorkingTreeDiff(ctx, opts) — how the
           orchestrator computes leftoverDiff (NOT called by runArbiter — it is a PARAMETER).
  why: confirms the FileChange→path-string mapping (use .Path), that runArbiter does NOT call these git
       methods (they are orchestrator-side), and the exact field names. CommitInfo.Files is []git.FileChange
       because the orchestrator passes DiffTree's return verbatim (no conversion at the orchestrator — the
       conversion lives in runArbiter's convertArbiterCommits).
  gotcha: runArbiter does NOT call StatusPorcelain/WorkingTreeDiff/DiffTree. The contract INPUT list names
          them as the DATA that flows into the arbiter decision (orchestrator-computed), not as runArbiter
          calls. runArbiter's signature takes commits + leftoverDiff as PARAMETERS — do not re-fetch them.

# MUST READ — Render (bare) + Execute (the provider seam)
- file: internal/provider/render.go
  section: `Manifest.Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode) (*CmdSpec,
           error)` — mode defaults to RenderBare (variadic); runArbiter MUST pass provider.RenderBare (the
           arbiter role is bare per §13.6.2/§13.6.5). ARG ORDER: Render takes (model, provider, sys, payload,
           mode) — pass (mdl, prov, sysPrompt, payload, RenderBare). Render calls Validate+Resolve internally
           (safe on the unresolved deps.Roles.Arbiter manifest).
- file: internal/provider/executor.go
  section: `Execute(ctx, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout, stderr string, err
           error)` — err is context.DeadlineExceeded on timeout, context.Canceled on parent cancel, wrapped
           *exec.ExitError on non-zero exit. Execute internally calls vb.VerboseCommand + vb.VerboseRawOutput.
           deps.Verbose may be nil (nil-safe).
  why: runArbiter's ONLY provider calls. Pass `*spec` (deref the Render pointer) and deps.Config.Timeout.
       Handle execErr: DeadlineExceeded/Canceled ⇒ return ArbiterOutput{nil} (null) immediately; non-zero
       exit ⇒ fall through to ParseArbiterOutput (stdout may be partial/valid) — mirrors callPlanner/generate.

# MUST READ — ResolveRoleModel + Config.Timeout
- file: internal/config/roles.go
  section: `func ResolveRoleModel(role string, cfg Config) (provider, model string)` — runArbiter calls
           ResolveRoleModel("arbiter", deps.Config). Returns (provider, model) — note the RETURN ORDER vs
           Render's ARG order (findings §6/§8). Reads cfg.Roles["arbiter"] then falls back to cfg.Provider/
           cfg.Model. FR-R5b guards the dangerous bare-model-no-provider-on-pi case at ResolveRoles time
           (BEFORE runArbiter runs), so the derivation is correct for every reachable case.
- file: internal/config/config.go
  section: Config — Timeout (120s default). runArbiter reads ONLY deps.Config.Timeout (Execute's per-attempt
           timeout). No diff caps (leftoverDiff is pre-computed), no MaxCommits (the safety cap is the
           planner's), no dedupe (the arbiter has no duplicate concept).
  why: runArbiter reads deps.Config.Timeout. That is the only Config field. config.Defaults() populates it.

# MUST READ — ParseArbiterOutput's brace-balanced helper is in planner.go (prompt pkg)
- file: internal/prompt/planner.go
  section: extractJSONObject(s) (string, bool) (line 161) — the brace-balanced JSON extractor ParseArbiterOutput
           REUSES (it is package-level in prompt, NOT redeclared in arbiter.go). Also PlannerRetryInstruction
           (line 49) — NOTE there is NO ArbiterRetryInstruction equivalent (§17.7 defines none).
  why: confirms ParseArbiterOutput works (the helper exists + is shared) and that NO retry instruction exists
       for the arbiter (the absence is the signal for single-shot).

# MUST READ — the test infrastructure (stubtest) + the test-repo fixture pattern
- file: internal/stubtest/stubtest.go
  section: `Build(t)` (compiles cmd/stubagent ONCE, cached); `Manifest(bin, Options{Out, Exit, SleepMS, Stderr,
           Script, Counter, Output, StripCodeFence})` (single-response BARE manifest — nil BareFlags is FINE for
           RenderBare); `NewScript(t, bin, responses)` (call-varying). Options.Out is the stub's single-response
           stdout; Options.Counter exposes the call count (assert NO retry = exactly 1 call).
  why: arbiter_test.go builds Deps{Git: git.New(repo), Config: config.Defaults(), Roles: RoleManifests{Arbiter:
       stubtest.Manifest(...)}, Verbose: nil} (NO ResolveRoles). The arbiter role is BARE → use stubtest.Manifest
       DIRECTLY (no tooled helper — that's stager_test.go's). The stub emits JSON on stdout; ParseArbiterOutput
       parses it (the stub's Output mode is IRRELEVANT — runArbiter does not call provider.ParseOutput). For the
       NO-retry assertion, use Options.Counter (or NewScript with one response + assert only one call consumed).
  gotcha: stubtest.Manifest passes Render's Validate+Resolve; the stub manifest leaves BareFlags nil (Render
          appends nil → no-op). The stub does NOT care about the model flag.

# MUST READ — the test-repo fixture helpers (copy into arbiter_test.go with arb* prefix — findings §11)
- file: internal/generate/generate_test.go
  section: the fixture helpers (initRepo, writeFile, stageFile, commitRaw, headSHA, runGit, gitOut) +
           TestCommitStaged_Success (the canonical stubtest+real-repo integration test pattern) + shaRe.
  why: arbiter_test.go needs a real git repo with a few commits (to build []CommitInfo with REAL SHAs + real
       DiffTree file-lists) and to test runArbiter end-to-end. Copy the fixture helpers VERBATIM but RENAME
       with an `arb` prefix (arbInitRepo, arbWriteFile, arbStageFile, arbCommitRaw, arbRunGit, arbGitOut,
       arbHeadSHA) to avoid colliding with planner_test.go's un-prefixed + stager_test.go's stg* + message_test.go's
       msg* copies (all in package decompose — a duplicate declaration is a compile error).

# MUST READ — the message PRP (P3.M2.T4.S1, in-flight parallel) — the immediate sibling
- docfile: plan/002_a17bb6c8dc1d/P3M2T4S1/PRP.md
  section: ErrMessageFailed/ErrPublicationFailed sentinels + generateMessage/publishCommit (the message role
           sibling). Confirms the package is growing (roles/planner/stager/message/arbiter) + the test-fixture
           collision rule (message used msg*; arbiter uses arb*).
  why: confirms the sentinel-naming convention (Err<Role>Failed), the file-doc-comment convention, and that
       message_test.go owns msg*-prefixed fixtures (so arbiter_test.go's arb* prefix is required to coexist).

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §13.6.5 (the arbiter: runs only if StatusPorcelain non-empty; receives SHAs+messages+file-lists
       of every run commit + a diff of remaining changes; returns {"target":"<sha>"} or {"target":null};
       ambiguous → null; "may only target a commit from this run"; stagecoach performs ALL git, arbiter only
       decides; the perfect run skips the arbiter)
- url: PRD.md §9.14 FR-M9 (arbiter agent bare; {"target":"<sha>"|null}) / FR-M10 (arbiter resolution —
       S2/P3.M3.T2.S1 owns it; null/tip/mid-chain/ambiguous→null)
- url: PRD.md §17.7 (the arbiter system prompt — committed verbatim in prompt/arbiter.go; the JSON contract
       `{"target": "<sha from the list>"}` instructs the arbiter to copy a SHA verbatim; NO retry instruction)
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  roles.go            # SHIPPED (P3.M2.T1.S1) — READ (CONSUMED): Deps, RoleManifests. Deps has NO Models field.
  planner.go          # SHIPPED (P3.M2.T2.S1) — READ (PATTERN): callPlanner, ErrPlannerFailed (the bare sibling
                      #   runArbiter mirrors MINUS the retry loop + MINUS error-on-failure).
  stager.go           # SHIPPED (P3.M2.T3.S1) — READ (PATTERN): ErrStagerFailed, stageConcept (sentinel convention).
  message.go          # IN-FLIGHT PARALLEL (P3.M2.T4.S1) — READ (PATTERN): ErrMessageFailed/ErrPublicationFailed
                      #   (sentinel convention; assumed shipped per the parallel-execution context).
  arbiter.go          # DOES NOT EXIST YET — THIS TASK CREATES IT.
  planner_test.go     # SHIPPED — owns UN-PREFIXED fixture names (initRepo, writeFile, ...). COLLISION hazard.
  stager_test.go      # SHIPPED — owns stg*-prefixed fixture names. COLLISION hazard.
  message_test.go     # IN-FLIGHT (P3.M2.T4.S1) — owns msg*-prefixed fixture names. COLLISION hazard.
  arbiter_test.go     # DOES NOT EXIST YET — THIS TASK CREATES IT (arb*-prefixed fixtures).
internal/prompt/
  arbiter.go          # SHIPPED (P3.M1.T1.S3) — READ (CONSUMED): BuildArbiterSystemPrompt (zero-arg),
                      #   BuildArbiterUserPayload, ParseArbiterOutput, ArbiterCommit, ArbiterOutput.
  planner.go          # READ (CONTEXT): extractJSONObject (shared by ParseArbiterOutput) + PlannerRetryInstruction
                      #   (NOTE: NO ArbiterRetryInstruction equivalent exists).
internal/git/
  git.go              # READ (CONSUMED TYPE): FileChange{Status,SrcPath,Path} (CommitInfo.Files). DiffTree/
                      #   StatusPorcelain/WorkingTreeDiff are the ORCHESTRATOR's calls (NOT runArbiter's).
internal/provider/
  render.go           # READ (CONSUMED): Manifest.Render(...,RenderBare), RenderBare.
  executor.go         # READ (CONSUMED): provider.Execute(ctx, spec, timeout, vb) → (stdout, stderr, err).
internal/config/
  config.go           # READ (CONSUMED): Config.Timeout, config.Defaults().
  roles.go            # READ (CONSUMED): ResolveRoleModel("arbiter", cfg) → (provider, model).
internal/stubtest/
  stubtest.go         # READ (test infra): Build, Manifest (bare), NewScript, Options.Counter.
internal/generate/
  generate_test.go    # READ (test pattern): fixture helpers (copy + arb*-rename) + TestCommitStaged_Success.
go.mod / go.sum       # UNCHANGED (module github.com/dustin/stagecoach; config/git/prompt/provider already
                      #   imported by roles.go/planner.go; git imported for FileChange — already via Deps.Git).
.golangci.yml         # READ: errcheck/gosimple/govet/ineffassign/staticcheck/unused.
```

### Desired Codebase tree with files to be added

```bash
internal/decompose/arbiter.go          # NEW — package `decompose` (5th file); the arbiter agent call:
                                      #   var ErrArbiterFailed = errors.New("decompose: arbiter failed")
                                      #   type CommitInfo struct { SHA, Subject string; Files []git.FileChange }
                                      #   func runArbiter(ctx context.Context, deps Deps, commits []CommitInfo, leftoverDiff string) (prompt.ArbiterOutput, error)
                                      #   func convertArbiterCommits(commits []CommitInfo) []prompt.ArbiterCommit   (private)
                                      #   func targetInRun(target string, validSHAs map[string]struct{}) bool        (private)
internal/decompose/arbiter_test.go     # NEW — stubtest (bare stubtest.Manifest) + real-git integration tests
                                      #   (fixture helpers with arb*-prefixed names). Cases: confident target; null
                                      #   target; target-not-in-list→null; empty-target→null; parse-failure→null;
                                      #   timeout→null; non-zero-exit-but-valid-stdout; render-error→ErrArbiterFailed;
                                      #   conversion/payload assertions; no-retry (exactly 1 Execute call).
# go.mod/go.sum UNCHANGED. roles.go + planner.go + stager.go + message.go + prompt/* + git/* + provider/*
# + config/* + cmd/* + pkg/stagecoach all UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (SCOPE — this task is the INVOCATION only — findings §2): runArbiter is the BARE arbiter agent call
//   + parse + in-list validation. It ONLY DECIDES. It does NOT implement resolution (new commit / tip amend /
//   mid-chain chain rebuild) — that is P3.M3.T2.S1 ("S2"). It does NOT construct []CommitInfo (the orchestrator
//   builds it from each run commit's SHA + Subject + DiffTree(sha,isRoot)). It does NOT call git.Git methods at
//   all: leftoverDiff is a PARAMETER (orchestrator-computed via WorkingTreeDiff), commits is a PARAMETER
//   (orchestrator-computed via DiffTree), and the StatusPorcelain TRIGGER is the orchestrator's gate (FR-M9:
//   orchestrator checks StatusPorcelain != "" BEFORE calling runArbiter). runArbiter uses deps only for
//   Roles.Arbiter (Render) + Config (ResolveRoleModel + Timeout) + Verbose. Do NOT add resolution logic, do NOT
//   call StatusPorcelain/WorkingTreeDiff/DiffTree, do NOT wire a caller (that is P3.M4.T1.S1).

// CRITICAL (the arbiter OWNS the null decision — findings §7): runArbiter's failure mode is BENIGN — null ⇒
//   the resolution makes a NEW commit for the leftovers; NO work is lost (§13.6.5 "when in doubt, prefer a NEW
//   commit (return null)"). This is UNIQUE among the four roles (planner = non-rescue abort; stager = retry-then-
//   empty; message = rescue with frozen tree). Therefore runArbiter degrades to ArbiterOutput{nil} (NOT an error)
//   on ANY indecision: parse failure, timeout, context cancel, empty target, target-not-in-list. ONLY a render
//   error returns a wrapped ErrArbiterFailed (the one true infra failure; near-impossible post-ResolveRoles).
//   Do NOT mirror callPlanner's "wrap timeout/parse-fail in ErrXxxFailed" — the arbiter is NOT load-bearing;
//   surfacing those as errors would force S2 to duplicate the "→ null" logic. S2 reads out.Target: nil ⇒ new
//   commit, &sha ⇒ amend. (S2 should treat ANY runArbiter error as null too, defensively, but in practice
//   runArbiter only errors on render.)

// CRITICAL (NO retry — findings §10): runArbiter performs exactly ONE Execute call. The contract lists
//   "Render → Execute → ParseArbiterOutput" as sequential steps with NO retry (contrast the planner contract
//   which EXPLICITLY says "Retry once on unparseable JSON"). prompt/arbiter.go states "§17.7 defines NO retry
//   instruction — this layer does not export one" — there is NO ArbiterRetryInstruction constant (only
//   PlannerRetryInstruction exists). And §13.6.5 "when in doubt, null" means a parse failure IS a "doubt" ⇒ null.
//   Do NOT add a retry loop. Do NOT prepend a retry instruction (there is none to prepend).

// CRITICAL (the FileChange→path-string seam — findings §4): CommitInfo.Files is []git.FileChange (the
//   orchestrator passes DiffTree's return verbatim); prompt.ArbiterCommit.Files is []string ("diff-tree
//   --name-only" PATHS). convertArbiterCommits maps each FileChange → its .Path (Path is ALWAYS set; SrcPath
//   is rename-source-only; Status is A/M/D and NOT part of the arbiter payload). Preserve SHA + Subject verbatim.

// CRITICAL (exact-match SHA validation — findings §9): "may only target a commit from this run" + "not in the
//   commits-made list → treat as null (ambiguous)". Build a set of the run-commits' full SHAs (BuildArbiterUserPayload
//   writes ArbiterCommit.SHA which the doc says is "the commit's full SHA (40/64 hex)"). The arbiter is
//   INSTRUCTED to copy a SHA "from the list" verbatim (§17.7: `{"target": "<sha from the list>"}`), so it echoes
//   a full SHA. Validate via EXACT membership (validSHAs[target]). An empty target OR a truncated/non-matching
//   target ⇒ false ⇒ null (safe). Do NOT do prefix matching (the contract reads "in the list"; exact is
//   deterministic + the arbiter is told to copy verbatim).

// CRITICAL (reuse prompt's types — do NOT define decompose ArbiterOutput/ArbiterCommit): runArbiter RETURNS
//   prompt.ArbiterOutput (the SHIPPED struct with Target *string). It does NOT define its own output type. The
//   ONLY new type is CommitInfo (the INPUT the orchestrator builds — distinct from prompt.ArbiterCommit because
//   CommitInfo.Files is []git.FileChange, the orchestrator's natural DiffTree return).

// CRITICAL (Render arg order vs ResolveRoleModel return order — findings §6/§8): ResolveRoleModel returns
//   (provider, model); Render takes (model, provider, sys, payload, mode). So pass (mdl, prov, sysPrompt,
//   payload, provider.RenderBare). `*spec` derefs the Render pointer. deps.Config.Timeout → Execute.
//   deps.Verbose may be nil (nil-safe — Execute + ui.Verbose handle nil).

// GOTCHA (test fixture name collision — findings §11): planner_test.go owns the UN-PREFIXED names
//   (initRepo, writeFile, stageFile, commitRaw, headSHA, runGit, gitOut); stager_test.go owns stg*-prefixed;
//   message_test.go owns msg*-prefixed. arbiter_test.go is ALSO package decompose — a duplicate declaration is
//   a COMPILE ERROR. Use DISTINCT arb*-prefixed names (arbInitRepo, arbWriteFile, arbStageFile, arbCommitRaw,
//   arbRunGit, arbGitOut, arbHeadSHA) — copied verbatim from internal/generate/generate_test.go.

// GOTCHA (the arbiter manifest is BARE — findings §6): Render deps.Roles.Arbiter with provider.RenderBare (the
//   arbiter is the bare role per §13.6.2/§13.6.5 — only the STAGER is tooled). Render's mode param is VARIADIC
//   and defaults to RenderBare, so OMITTING it would also be bare — but pass it explicitly for clarity + parity
//   with callPlanner/stageConcept. The stub's nil BareFlags is fine for RenderBare (append(nil) no-op).

// GOTCHA (package decompose is growing — arbiter.go is the 5th file): roles.go, planner.go, stager.go are
//   shipped; message.go is in-flight (parallel); arbiter.go is 5th. Same `package decompose`. Add a file doc
//   comment (cite §13.6.5 + FR-M9 + §17.7). No import cycle (decompose → git/config/prompt/provider is one-way).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/decompose/arbiter.go — package decompose (the 5th file after roles.go + planner.go + stager.go + message.go)

// ErrArbiterFailed is the sentinel for the arbiter's ONE true infra failure: a render error (the arbiter
// manifest could not be rendered). Wrapped (%w) so errors.Is works. Near-impossible post-ResolveRoles (the
// manifest was Validated + install-checked), but wrapped for consistency with the sibling sentinels
// (ErrPlannerFailed/ErrStagerFailed/ErrMessageFailed) + verbose logging.
//
// IMPORTANT — the arbiter OWNS the §13.6.5 "when in doubt, null" decision: agent failures (timeout, cancel,
// parse-fail) and semantic ambiguity (empty target, target-not-in-list) do NOT return errors — they degrade
// to prompt.ArbiterOutput{Target: nil} (null ⇒ new commit; no work lost). The resolution logic (S2) reads
// out.Target: nil ⇒ new commit, &sha ⇒ amend. (S2 should treat ANY runArbiter error as null too, defensively.)
var ErrArbiterFailed = errors.New("decompose: arbiter failed")

// CommitInfo is one commit made this run, as the orchestrator builds it for the arbiter (PRD §13.6.5:
// "SHAs, messages, and file-lists (diff-tree) of every commit made this run"). The orchestrator populates
// Files from git.DiffTree(sha, isRoot) verbatim (hence []git.FileChange, not []string). runArbiter converts
// these to []prompt.ArbiterCommit (the prompt layer's []string-path form) via convertArbiterCommits.
type CommitInfo struct {
	SHA     string             // the commit's full SHA (40/64 hex) — the value the arbiter may return as "target".
	Subject string             // the commit's subject line (§13.6.5's "messages").
	Files   []git.FileChange   // the diff-tree file-list (DiffTree's return verbatim); may be empty.
}

// (runArbiter + convertArbiterCommits + targetInRun — see Implementation Tasks)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/decompose/arbiter.go — package doc + imports + ErrArbiterFailed + CommitInfo
  - FILE DOC: cite PRD §13.6.5 (the arbiter: runs only if StatusPorcelain non-empty; receives run-commits'
    SHAs+subjects+file-lists + a leftover diff; returns {"target":"<sha>"}|{"target":null}; ambiguous→null;
    "may only target a commit from this run"; arbiter only DECIDES; stagecoach performs all git) + §9.14 FR-M9
    (arbiter agent bare) + §17.7 (the arbiter system prompt + JSON contract; NO retry instruction). Note
    arbiter.go is the arbiter agent invocation (runArbiter), the decompose analogue of callPlanner's Render→
    Execute→parse pattern specialized to the arbiter's {"target":"<sha>"|null} JSON contract. Note it is
    SINGLE-SHOT (no retry — §17.7 defines no retry instruction). Note the arbiter OWNS the §13.6.5 "when in
    doubt, null" decision (degrades to null on ANY indecision; only render error returns ErrArbiterFailed).
    Note it performs ZERO git reads (the orchestrator pre-computes commits + leftoverDiff) and ONLY DECIDES
    (resolution is P3.M3.T2.S1). Consumed by the orchestrator (P3.M4.T1.S1); no caller wiring.
  - IMPORTS: "context"; "errors"; "fmt"; "github.com/dustin/stagecoach/internal/config";
    "github.com/dustin/stagecoach/internal/git"; "github.com/dustin/stagecoach/internal/prompt";
    "github.com/dustin/stagecoach/internal/provider".
    (NO "ui" — deps.Verbose is the ui.Verbose handle via Deps; no direct ui symbol. NO "signal". All these are
    already imported by roles.go/planner.go EXCEPT confirm each is USED (golangci/unused rejects unused imports).
    git IS used (CommitInfo.Files []git.FileChange). prompt IS used (BuildArbiterSystemPrompt/UserPayload/
    ParseArbiterOutput/ArbiterCommit/ArbiterOutput). config (ResolveRoleModel). provider (Render/Execute/
    RenderBare). context + errors + fmt (stdlib).)
  - DEFINE `var ErrArbiterFailed = errors.New("decompose: arbiter failed")` + `type CommitInfo struct {...}`
    with the doc comments above (findings §4 for the FileChange rationale).

Task 2: CREATE internal/decompose/arbiter.go — convertArbiterCommits + targetInRun (private helpers)
  - DEFINE `func convertArbiterCommits(commits []CommitInfo) ([]prompt.ArbiterCommit, map[string]struct{})`:
    returns the converted []prompt.ArbiterCommit AND the valid-SHA set (built once; targetInRun consumes it).
    Body:
      out := make([]prompt.ArbiterCommit, len(commits))
      valid := make(map[string]struct{}, len(commits))
      for i, c := range commits {
          files := make([]string, len(c.Files))
          for j, f := range c.Files {
              files[j] = f.Path   // Path is ALWAYS set; Status/SrcPath are NOT part of the arbiter payload
          }
          out[i] = prompt.ArbiterCommit{SHA: c.SHA, Subject: c.Subject, Files: files}
          valid[c.SHA] = struct{}{}
      }
      return out, valid
    DOC: cite §13.6.5 + the ArbiterCommit.Files doc ("diff-tree --name-only" paths). Explain the FileChange→
    .Path seam (CommitInfo.Files is []git.FileChange from the orchestrator's DiffTree; ArbiterCommit.Files is
    []string). The valid map is the §13.6.5 "may only target a commit from this run" set (full SHAs).
  - DEFINE `func targetInRun(target string, validSHAs map[string]struct{}) bool`:
    if target == "" { return false }
    _, ok := validSHAs[target]
    return ok
    DOC: §13.6.5 "may only target a commit from this run" + "not in the commits-made list → treat as null
    (ambiguous)". Exact membership (the arbiter is instructed to copy a SHA "from the list" verbatim per §17.7;
    a truncated/non-matching/empty target ⇒ false ⇒ null). Single-resolver: a full SHA matches exactly one entry.
  - GOTCHA: define these as small private helpers (Go order is irrelevant). Keep convertArbiterCommits returning
    BOTH the slice AND the set so runArbiter builds the set ONCE (no double loop).

Task 3: CREATE internal/decompose/arbiter.go — runArbiter (the entry point)
  - SIGNATURE: `func runArbiter(ctx context.Context, deps Deps, commits []CommitInfo, leftoverDiff string)
    (prompt.ArbiterOutput, error)`.
  - BODY (mirror callPlanner's Render→Execute→parse MINUS retry + graceful null; see Implementation Patterns):
      // 1. Derive the arbiter (provider, model) — Deps has no Models field (findings §5/§8).
      prov, mdl := config.ResolveRoleModel("arbiter", deps.Config)

      // 2. Convert []CommitInfo → []prompt.ArbiterCommit + build the valid-SHA set (the in-list check).
      arbiterCommits, validSHAs := convertArbiterCommits(commits)

      // 3. Build the §17.7 system prompt (zero-arg) + user payload (commit list + leftover diff).
      sysPrompt := prompt.BuildArbiterSystemPrompt()
      payload := prompt.BuildArbiterUserPayload(arbiterCommits, leftoverDiff)

      // 4. Render the arbiter manifest in BARE mode (the arbiter is the bare role, §13.6.2/§13.6.5).
      spec, rerr := deps.Roles.Arbiter.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)
      if rerr != nil {
          return prompt.ArbiterOutput{}, fmt.Errorf("%w: render: %w", ErrArbiterFailed, rerr)
      }

      // 5. Execute ONCE (NO retry — §17.7 defines no retry instruction; the arbiter is "when in doubt, null").
      out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)
      if execErr != nil {
          if errors.Is(execErr, context.DeadlineExceeded) || errors.Is(execErr, context.Canceled) {
              // Timeout / cancel → graceful null (the arbiter OWNS the null decision, §13.6.5).
              return prompt.ArbiterOutput{Target: nil}, nil
          }
          // Non-zero exit — stdout may be partial/valid; fall through to parse (mirrors callPlanner/generate).
      }

      // 6. Parse the arbiter's JSON output.
      parsed, perr := prompt.ParseArbiterOutput(out)
      if perr != nil {
          // Parse failure → graceful null (NOT an error — "when in doubt, null", §13.6.5).
          return prompt.ArbiterOutput{Target: nil}, nil
      }

      // 7. Validate the target is one of THIS run's commits (§13.6.5 "may only target a commit from this run";
      //    "not in the commits-made list → treat as null (ambiguous)"). Empty target ⇒ null.
      if parsed.Target != nil && targetInRun(*parsed.Target, validSHAs) {
          return parsed, nil  // confident in-list target — S2 amends/rebuilds
      }
      return prompt.ArbiterOutput{Target: nil}, nil  // empty / not-in-list → null (ambiguous → new commit)
  - DOC COMMENT: cite PRD §13.6.5 + §9.14 FR-M9 + §17.7; diagram the pipeline (derive model → convert commits +
    valid set → BuildArbiterSystemPrompt + BuildArbiterUserPayload → Render bare → Execute ONCE → timeout/cancel⇒null
    / non-zero-exit⇒fall-through → ParseArbiterOutput / parse-fail⇒null → in-list validate → confident target OR
    null); note it is the decompose analogue of callPlanner's Render→Execute→parse (MINUS retry + graceful null);
    note it is SINGLE-SHOT (§17.7 has no retry instruction); note it performs ZERO git reads (commits +
    leftoverDiff are params; the StatusPorcelain trigger is the orchestrator's gate, FR-M9); note it ONLY DECIDES
    (resolution is P3.M3.T2.S1); note the error contract (render error ⇒ ErrArbiterFailed; everything else ⇒ null).
  - GOTCHA: Render's mode param is variadic — pass `provider.RenderBare` explicitly (5th arg). ResolveRoleModel
    returns (provider, model); Render takes (model, provider) — pass (mdl, prov). `*spec` derefs the Render
    pointer. deps.Config.Timeout is the per-attempt timeout (Execute derives the context). deps.Verbose may be nil.
  - GOTCHA: `parsed.Target` is *string — deref with `*parsed.Target` ONLY after the nil check. Returning `parsed`
    (the parsed ArbiterOutput) on the confident-target path is correct — Target already points at the validated SHA.
  - GOTCHA: do NOT call deps.Git anywhere (the orchestrator pre-computes commits + leftoverDiff; golangci/unused
    will NOT flag deps as unused because deps.Roles.Arbiter + deps.Config + deps.Verbose ARE used — but ensure you
    do not leave an unused `deps.Git` reference).

Task 4: CREATE internal/decompose/arbiter_test.go — fixture helpers (copied from generate_test.go, arb*-renamed)
  - IMPORTS: "context"; "errors"; "os/exec"; "strings"; "testing"; "time";
    "github.com/dustin/stagecoach/internal/config"; "github.com/dustin/stagecoach/internal/git";
    "github.com/dustin/stagecoach/internal/prompt"; "github.com/dustin/stagecoach/internal/provider";
    "github.com/dustin/stagecoach/internal/stubtest".
    Package: `decompose` (internal test — runArbiter/convertArbiterCommits/targetInRun/CommitInfo visible).
  - COPY the fixture helpers from generate_test.go VERBATIM but RENAME with the `arb` prefix: arbInitRepo,
    arbWriteFile, arbStageFile, arbCommitRaw, arbRunGit, arbGitOut, arbHeadSHA (they are unimportable from
    package decompose; planner_test/stager_test/message_test own their own copies for the same reason). Do NOT
    use un-prefixed / stg* / msg* names — collision = compile error.
  - ADD a helper `func arbDeps(t *testing.T, repo string, m provider.Manifest) Deps` that builds
    `Deps{Git: git.New(repo), Config: config.Defaults(), Roles: RoleManifests{Arbiter: m}, Verbose: nil}`.
  - ADD a helper `func arbCommits(t *testing.T, repo string) []CommitInfo` that builds a small []CommitInfo from
    a real repo: arbCommitRaw a couple of commits, then for each use git.New(repo).DiffTree(sha, isRoot) to
    populate Files. (This exercises the REAL FileChange→path conversion end-to-end.)

Task 5: CREATE internal/decompose/arbiter_test.go — the test cases (findings §12)
  - TestRunArbiter_ConfidentTarget: repo with commits → []CommitInfo with real SHAs; stub emits
    `{"target": "<shaA>"}` (shaA = the FIRST commit's full SHA). Assert: nil error; out.Target != nil;
    *out.Target == shaA.
  - TestRunArbiter_NullTarget: stub emits `{"target": null}`. Assert: out.Target == nil; nil error.
  - TestRunArbiter_TargetNotInList: stub emits `{"target": "0123456789abcdef0123456789abcdef01234567"}`
    (a 40-hex SHA NOT in the run). Assert: out.Target == nil; nil error (ambiguous→null; the load-bearing
    in-list check). Confirm it is NOT an error.
  - TestRunArbiter_EmptyTarget: stub emits `{"target": ""}`. Assert: out.Target == nil; nil error.
  - TestRunArbiter_ParseFailureNull: stub emits `"not json at all"` (Options.Out). Assert: out.Target == nil;
    nil error (graceful; NOT ErrArbiterFailed — confirm errors.Is is NOT needed since err == nil).
  - TestRunArbiter_TimeoutNull: cfg.Timeout=100ms (set deps.Config.Timeout), stub Options.SleepMS=2000. Assert:
    out.Target == nil; nil error (graceful; NOT an error — contrast planner which returns ErrPlannerFailed on
    timeout). (Optional: confirm via Options.Counter that the stub was called once — no retry.)
  - TestRunArbiter_NonZeroExitValidStdout: stub Options.Exit=1, Options.Out=`{"target": "<shaA>"}`. Assert:
    out.Target != nil; *out.Target == shaA; nil error (falls through to parse; partial-but-valid accepted).
  - TestRunArbiter_RenderError: build a manifest whose Render fails (e.g. a minimal Manifest with an invalid
    field that fails Validate, OR a helper that returns a spec Render cannot build — mirror how other tests
    force a render error, e.g. an empty Command). Assert: ArbiterOutput{} (zero value / nil Target);
    errors.Is(err, ErrArbiterFailed) true. (If forcing a render error is awkward, construct a provider.Manifest
    with Command="" so Validate fails inside Render.)
  - TestRunArbiter_NoRetry (exactly 1 Execute call): set Options.Counter to a temp file path (the stub writes
    its call count there — read stubtest.go for the exact mechanism) and have the stub emit INVALID json; assert
    runArbiter returns null AND the stub was invoked EXACTLY once (read the count == 1 from the Counter file) —
    confirms NO retry loop (contrast planner's 2 attempts).
  - TestRunArbiter_PayloadConversion: capture the rendered payload. Either (a) extend the stub to tee its stdin
    to a buffer and assert the payload contains each commit's SHA + Subject + each FileChange.Path (NOT Status/
    SrcPath) + the §17.7 headers + the leftoverDiff tail verbatim; OR (b) SIMPLER + more focused: call
    convertArbiterCommits directly on a []CommitInfo with known FileChange values and assert the resulting
    []prompt.ArbiterCommit.Files == the expected []string paths (the unit test for the seam). Do BOTH: the unit
    test on convertArbiterCommits (deterministic) + an end-to-end assertion that BuildArbiterUserPayload(
    converted, diff) contains the SHA/Subject/path/diff (via the stub's stdin tee OR by calling
    BuildArbiterUserPayload directly in a helper test).
  - TestConvertArbiterCommits + TestTargetInRun: small unit tests for the private helpers (package decompose
    internal test). convertArbiterCommits: empty input ⇒ empty slice + empty map; FileChange{Status:"A",
    Path:"a.go"} ⇒ "a.go"; rename FileChange{Status:"R100", SrcPath:"old.go", Path:"new.go"} ⇒ "new.go"
    (SrcPath dropped). targetInRun: ""⇒false; known⇒true; unknown⇒false.
  - GOTCHA: for the timeout test, set deps.Config.Timeout = 100 * time.Millisecond (config.Defaults() gives
    120s). For the no-retry assertion, the stub's Counter (stubtest.Options.Counter) increments per Execute —
    assert == 1. For real SHAs, use arbCommitRaw + arbHeadSHA / git rev-parse to get full 40-hex SHAs.
```

### Implementation Patterns & Key Details

```go
// runArbiter — the bare arbiter invocation (mirror callPlanner MINUS retry + graceful null).
// The arbiter OWNS the §13.6.5 "when in doubt, null" decision: ANY indecision (parse-fail/timeout/cancel/
// empty-target/target-not-in-list) ⇒ ArbiterOutput{nil} with a NIL error. ONLY a render error ⇒ ErrArbiterFailed.
func runArbiter(ctx context.Context, deps Deps, commits []CommitInfo, leftoverDiff string) (prompt.ArbiterOutput, error) {
    // 1. Derive the arbiter (provider, model) — Deps has no Models field.
    prov, mdl := config.ResolveRoleModel("arbiter", deps.Config)

    // 2. Convert []CommitInfo → []prompt.ArbiterCommit (FileChange→path seam) + build the valid-SHA set.
    arbiterCommits, validSHAs := convertArbiterCommits(commits)

    // 3. Build the §17.7 system prompt (zero-arg) + user payload.
    sysPrompt := prompt.BuildArbiterSystemPrompt()
    payload := prompt.BuildArbiterUserPayload(arbiterCommits, leftoverDiff)

    // 4. Render the arbiter manifest BARE (the arbiter is the bare role).
    spec, rerr := deps.Roles.Arbiter.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)
    if rerr != nil {
        return prompt.ArbiterOutput{}, fmt.Errorf("%w: render: %w", ErrArbiterFailed, rerr) // the ONE error path
    }

    // 5. Execute ONCE (NO retry). Timeout/cancel ⇒ graceful null; non-zero exit ⇒ fall through to parse.
    out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)
    if execErr != nil {
        if errors.Is(execErr, context.DeadlineExceeded) || errors.Is(execErr, context.Canceled) {
            return prompt.ArbiterOutput{Target: nil}, nil // graceful → null
        }
        // Non-zero exit — fall through (stdout may be partial/valid), mirrors callPlanner/generate.
    }

    // 6. Parse. Parse failure ⇒ graceful null (NOT an error — "when in doubt, null").
    parsed, perr := prompt.ParseArbiterOutput(out)
    if perr != nil {
        return prompt.ArbiterOutput{Target: nil}, nil
    }

    // 7. Validate target in-list (§13.6.5 "may only target a commit from this run"; empty ⇒ null).
    if parsed.Target != nil && targetInRun(*parsed.Target, validSHAs) {
        return parsed, nil // confident in-list target — S2 amends/rebuilds
    }
    return prompt.ArbiterOutput{Target: nil}, nil // empty / not-in-list → null (ambiguous → new commit)
}

// convertArbiterCommits — the FileChange→path-string seam + the valid-SHA set (built once).
func convertArbiterCommits(commits []CommitInfo) ([]prompt.ArbiterCommit, map[string]struct{}) {
    out := make([]prompt.ArbiterCommit, len(commits))
    valid := make(map[string]struct{}, len(commits))
    for i, c := range commits {
        files := make([]string, len(c.Files))
        for j, f := range c.Files {
            files[j] = f.Path // Path ALWAYS set; Status/SrcPath NOT part of the arbiter payload
        }
        out[i] = prompt.ArbiterCommit{SHA: c.SHA, Subject: c.Subject, Files: files}
        valid[c.SHA] = struct{}{}
    }
    return out, valid
}

// targetInRun — §13.6.5 "may only target a commit from this run"; exact membership (full SHAs).
func targetInRun(target string, validSHAs map[string]struct{}) bool {
    if target == "" {
        return false
    }
    _, ok := validSHAs[target]
    return ok
}
```

### Integration Points

```yaml
CONSUMED (runArbiter reads, does NOT define):
  - prompt.BuildArbiterSystemPrompt() — zero-arg §17.7 system prompt.
  - prompt.BuildArbiterUserPayload([]ArbiterCommit, leftoverDiff) — §17.7 user payload.
  - prompt.ParseArbiterOutput(raw) — JSON parse (whole-string + brace-balanced fallback).
  - prompt.ArbiterCommit / prompt.ArbiterOutput — the SHIPPED types (runArbiter RETURNS ArbiterOutput).
  - config.ResolveRoleModel("arbiter", cfg) → (provider, model).
  - deps.Roles.Arbiter.Render(model, provider, sys, payload, provider.RenderBare).
  - provider.Execute(ctx, spec, timeout, vb) → (stdout, stderr, err).
  - git.FileChange — the CommitInfo.Files element type (Path is the field used).

PRODUCED (runArbiter defines, consumed later):
  - type CommitInfo{SHA, Subject, Files []git.FileChange} — consumed by the orchestrator (P3.M4.T1.S1) to
    build the []CommitInfo from each run-commit's SHA + Subject + DiffTree(sha,isRoot).
  - runArbiter(...) (prompt.ArbiterOutput, error) — consumed by the orchestrator (P3.M4.T1.S1) which calls it
    AFTER the per-concept loop + the StatusPorcelain != "" gate. The orchestrator then dispatches to the
    resolution logic (P3.M3.T2.S1 "S2"): out.Target == nil (or err != nil, treated as null) ⇒ new commit;
    *out.Target == HEAD ⇒ tip amend; *out.Target == earlier commit ⇒ mid-chain rebuild.
  - ErrArbiterFailed — the orchestrator/S2 detect render-failure via errors.Is (and treat it as null).

NOT TOUCHED (owned elsewhere — do NOT edit):
  - internal/decompose/chain.go — DOES NOT EXIST YET (P3.M3.T2.S1 "S2" resolution). runArbiter does NOT
    implement new-commit/tip-amend/mid-chain-rebuild.
  - internal/decompose/decompose.go — DOES NOT EXIST YET (P3.M4.T1.S1 orchestrator). runArbiter does NOT wire
    a caller, does NOT check StatusPorcelain, does NOT compute leftoverDiff, does NOT build []CommitInfo.
  - git.StatusPorcelain / WorkingTreeDiff / DiffTree — orchestrator-side (runArbiter does NOT call them).
  - cmd/, pkg/stagecoach/ — UNCHANGED.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after creating arbiter.go — fix before proceeding.
cd /home/dustin/projects/stagecoach
gofmt -w internal/decompose/arbiter.go internal/decompose/arbiter_test.go   # format
go vet ./internal/decompose/...                                               # vet
golangci-lint run ./internal/decompose/...                                    # lint (.golangci.yml)

# Expected: Zero errors. READ the output and fix before proceeding (unused imports, shadowed vars, etc.).
# golangci-lint enforces errcheck/gosimple/govet/ineffassign/staticcheck/unused.
```

### Level 2: Unit + Integration Tests (Component Validation)

```bash
cd /home/dustin/projects/stagecoach

# The arbiter-specific tests.
go test ./internal/decompose/... -run 'Arbiter|ConvertArbiterCommits|TargetInRun' -v

# Confirm NO fixture-name collisions across the whole decompose package (a duplicate declaration is a
# compile error — this also catches any arb*/stg*/msg*/un-prefixed clash).
go build ./internal/decompose/...
go test ./internal/decompose/... -v

# Full suite — ensure no regressions (arbiter.go must not break planner/stager/message/roles tests).
go test ./...

# Expected: All tests pass. The arbiter tests cover: confident target, null target, target-not-in-list→null,
# empty-target→null, parse-failure→null, timeout→null, non-zero-exit-but-valid-stdout, render-error→ErrArbiterFailed,
# no-retry (exactly 1 Execute call), and the convertArbiterCommits/targetInRun unit tests.
```

### Level 3: Integration Testing (System Validation)

```bash
cd /home/dustin/projects/stagecoach

# Build the whole module (catches import-cycle / missing-symbol errors across packages).
go build ./...

# Confirm go.mod/go.sum are UNCHANGED (no new deps — config/git/prompt/provider already imported).
git diff --name-only go.mod go.sum   # Expected: (no output)

# Confirm only the 2 new files are the change set.
git status --short                   # Expected: 2 new files (internal/decompose/arbiter.go + arbiter_test.go)

# Smoke: runArbiter is internal (no CLI surface yet — the orchestrator is P3.M4). The integration validation
# IS the stubtest+real-git test suite (Level 2). There is no standalone binary to invoke for the arbiter alone.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Arbiter-specific invariants (asserted in the test suite, not a separate command):
#   - runArbiter performs ZERO git reads (deps.Git unused in the happy path) — assert via the stubtest harness.
#   - runArbiter performs exactly ONE Execute call (NO retry) — assert via stubtest Options.Counter == 1.
#   - runArbiter OWNS the null decision: parse-fail/timeout/cancel/empty/not-in-list ⇒ ArbiterOutput{nil}, nil
#     error (NOT ErrArbiterFailed) — assert err == nil in those tests.
#   - the FileChange→path conversion drops Status/SrcPath — assert via TestConvertArbiterCommits.
#   - the in-list check is exact (full SHAs) — assert a 40-hex not-in-list SHA ⇒ null.

# Lint + format final gate (project-wide).
golangci-lint run
gofmt -l internal/ pkg/   # Expected: empty output
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] All tests pass: `go test ./...`
- [ ] No vet errors: `go vet ./...`
- [ ] No lint errors: `golangci-lint run`
- [ ] No formatting issues: `gofmt -l internal/ pkg/` empty
- [ ] go.mod/go.sum UNCHANGED (no new dependencies)

### Feature Validation

- [ ] All success criteria from "What" section met.
- [ ] runArbiter returns ArbiterOutput{&sha} on a confident in-list target; ArbiterOutput{nil} on
      parse-fail/timeout/cancel/empty/not-in-list (all with nil error); wrapped ErrArbiterFailed ONLY on render.
- [ ] runArbiter uses RenderBare + ResolveRoleModel("arbiter") + a SINGLE Execute (NO retry).
- [ ] runArbiter performs ZERO git reads (deps.Git unused in the happy path).
- [ ] convertArbiterCommits maps git.FileChange.Path into []string (Status/SrcPath dropped).
- [ ] targetInRun enforces exact full-SHA membership (empty ⇒ false ⇒ null).
- [ ] Manual/integration testing successful: the stubtest+real-git suite (Level 2) is green.
- [ ] Error cases handled gracefully: ALL indecision ⇒ null (nil error); render ⇒ ErrArbiterFailed.
- [ ] No fixture-name collisions in package decompose (arb* prefix coexists with un-prefixed/stg*/msg*).

### Code Quality Validation

- [ ] Follows existing codebase patterns (mirrors callPlanner's Render→Execute→parse; sentinel Err<Role>Failed).
- [ ] File placement matches the desired codebase tree (internal/decompose/arbiter.go + arbiter_test.go).
- [ ] Anti-patterns avoided (no retry where none is specified; no error-swallowing where an error IS warranted;
      no git reads the orchestrator owns; no resolution logic that S2 owns).
- [ ] Dependencies properly managed (only stdlib + config/git/prompt/provider, all pre-existing).
- [ ] No import cycle (decompose → git/config/prompt/provider is one-way).

### Documentation & Deployment

- [ ] File doc comment cites PRD §13.6.5 + §9.14 FR-M9 + §17.7.
- [ ] runArbiter + CommitInfo + ErrArbiterFailed + the private helpers have doc comments explaining the
      error contract (the arbiter OWNS the null decision) and the FileChange→path seam.
- [ ] No new environment variables or config keys (the arbiter reuses the "arbiter" role from ResolveRoles).

---

## Anti-Patterns to Avoid

- ❌ Don't implement resolution (new commit / tip amend / mid-chain rebuild) — that is S2 (P3.M3.T2.S1).
      runArbiter ONLY DECIDES and returns an ArbiterOutput.
- ❌ Don't call StatusPorcelain / WorkingTreeDiff / DiffTree inside runArbiter — the orchestrator pre-computes
      commits + leftoverDiff and gates on StatusPorcelain (FR-M9). runArbiter takes them as PARAMETERS.
- ❌ Don't add a retry loop — §17.7 defines NO retry instruction; the arbiter is single-shot; "when in doubt, null".
- ❌ Don't wrap timeout/cancel/parse-fail in ErrArbiterFailed — the arbiter OWNS the null decision; those degrade
      to ArbiterOutput{nil} (nil error). ONLY a render error returns a wrapped ErrArbiterFailed.
- ❌ Don't define decompose ArbiterOutput/ArbiterCommit types — REUSE prompt's (runArbiter RETURNS prompt.ArbiterOutput;
      CommitInfo is the only new type, distinct because its Files is []git.FileChange).
- ❌ Don't include Status/SrcPath in the arbiter payload — ArbiterCommit.Files is []string PATHS (".Path" only).
- ❌ Don't use prefix matching for the in-list check — exact full-SHA membership (the arbiter is told to copy
      verbatim; a non-exact match ⇒ null, safely).
- ❌ Don't use un-prefixed/stg*/msg* fixture names in arbiter_test.go — use arb* (package decompose collision).
- ❌ Don't skip validation because "it should work" — run the full suite + lint + vet + gofmt gate.
- ❌ Don't ignore the parallel-execution context — message.go (P3.M2.T4.S1) is in-flight; do NOT edit it or its
      test file; coexist via the arb* fixture prefix.
