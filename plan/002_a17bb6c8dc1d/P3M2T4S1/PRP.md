---
name: "P3.M2.T4.S1 — Implement internal/decompose/message.go: per-concept message generation (tree-to-tree diff) + serialized publication (PRD §13.6.2/§13.6.3, §9.14 FR-M6/M7/M8/M12, §13.1–§13.5)"
description: |

  CREATE ONE NEW FILE `internal/decompose/message.go` (package `decompose`, the 4th file after the
  shipped roles.go, planner.go, stager.go) and ONE NEW TEST FILE `message_test.go`. message.go is the
  message + publication half of the multi-commit decomposition pipeline (PRD §13.6.3). `generateMessage`
  is the BARE message-role invocation that is "a variant of generate.CommitStaged's loop that takes a
  diff string instead of calling StagedDiff": it computes the concept diff via `TreeDiff(tree[i-1],
  tree[i])` (§13.6.3 invariant 2 — tree-to-tree, NEVER index-vs-HEAD), builds the v1 system prompt +
  fetches fresh recent subjects, and runs the SAME bounded generate→parse→dedupe retry loop as
  CommitStaged (FR26–FR33), reusing generate's exported `RescueError`/`ErrTimeout`/`ErrRescue`/
  `ExtractSubject`/`IsDuplicate`. It returns the message, or a `*generate.RescueError` on generation
  failure (timeout/parse-fail/duplicate-exhausted/non-zero-exit/cancel) carrying TreeSHA=tree[i] +
  ParentSHA + Candidate for the §18.3/FR-M12 rescue. `publishCommit` is the serialized publication
  primitive (§13.6.3 / FR-M7): CommitTree(tree[i], [newSHA[i-1]], msg) → newSHA[i], then
  UpdateRefCAS(HEAD, newSHA[i], newSHA[i-1]) (CAS) — returning newSHA on success or a `*generate.CASError`
  (whose .Error() IS the §13.5 "HEAD moved" message) on CAS failure. Both are SIGNAL-FREE primitives
  consumed by the orchestrator (P3.M4.T1.S1); NO caller wiring, NO concept-iteration loop, NO overlap
  goroutine scheduling, NO signal arming here (those are P3.M4).

  CONTRACT (P3.M2.T4.S1, verbatim from the work item):
    1. RESEARCH NOTE: Per §13.6.3, message[i] reasons over `git diff tree[i-1] tree[i]` (tree-to-tree,
       never index-vs-HEAD). The message agent is bare — it reuses v1's generate primitives (system
       prompt, dedupe, parse). The concept diff comes from TreeDiff (P2.M2.T1.S2). Publication is
       serialized: commit[i] = commit-tree -p newSHA[i-1] tree[i] msg[i], then update-ref HEAD newSHA[i]
       newSHA[i-1] (CAS). tree[-1] is RevParseTree(HEAD) or the empty tree for unborn. Empty-concept skip
       (FR-M8): if tree[i]==tree[i-1], skip commit[i] (THE ORCHESTRATOR's job — generateMessage only
       guards empty diff defensively). Overlap: message[i] can overlap stager[i+1] (the ORCHESTRATOR
       launches message[i] in a goroutine; generateMessage itself is synchronous & has NO concurrency
       code). The orchestrator manages this interleaving.
    2. INPUT: Deps from P3.M2.T1.S1, TreeDiff from P2.M2.T1.S2, generate primitives (ExtractSubject,
       IsDuplicate, RescueError, CASError) from existing code, prompt.BuildUserPayload/BuildSystemPrompt/
       BuildFallbackPrompt/DetectMultiline.
    3. LOGIC: The message generation function is a variant of generate.CommitStaged's loop that takes a
       diff STRING instead of calling StagedDiff. publishCommit = CommitTree + UpdateRefCAS.
    4. OUTPUT: generateMessage returns the message (or *RescueError); publishCommit returns newSHA (or
       *CASError). The orchestrator (P3.M4.T1.S1) composes them into the serialized publication loop.
    5. DOCS: none — internal agent call + git operations.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
    - internal/decompose/roles.go — SHIPPED (P3.M2.T1.S1). Defines Deps {Git, Registry, Config,
      Roles RoleManifests, Verbose}, RoleManifests{Planner,Stager,Message,Arbiter}. CONSUMED: deps.Roles.
      Message is the BARE message manifest. Deps has NO Models field (generateMessage derives the message
      (provider, model) via ResolveRoleModel — see findings §4).
    - internal/decompose/planner.go — SHIPPED (P3.M2.T2.S1). CONSUMED as the SIBLING PATTERN (callPlanner's
      Render→Execute→handle + ErrPlannerFailed sentinel + ResolveRoleModel derivation). Do NOT edit.
    - internal/decompose/stager.go — SHIPPED (P3.M2.T3.S1). Defines stageConcept (tooled) + freezeSnapshot.
      CONSUMED by the orchestrator (P3.M4), NOT by this task — but freezeSnapshot's tree[i] outputs feed
      generateMessage's (treeA, treeB) inputs. Do NOT edit.
    - internal/generate/generate.go — SHIPPED (v1). CONSUMED: CommitStaged's step-5 loop (the loop to port
      — read it), and EXPORTED RescueError/CASError/ErrTimeout/ErrRescue/ErrCASFailed/ExtractSubject/
      IsDuplicate. Its UNEXPORTED buildSystemPrompt/recentSubjects are re-ported as private helpers
      (§7) — do NOT edit generate.go.
    - internal/git/git.go — CONSUMED: TreeDiff(treeA,treeB,opts), RevParseHEAD(), CommitTree(tree,
      parents,msg), UpdateRefCAS(ref,newSHA,expectedOld), ErrCASFailed, StagedDiffOptions, EmptyTreeSHA.
    - internal/prompt/{system,payload}.go — CONSUMED: BuildUserPayload, BuildSystemPrompt,
      BuildFallbackPrompt, DetectMultiline.
    - internal/provider/{render,executor,parse}.go — CONSUMED: Manifest.Render(...,RenderBare),
      provider.Execute(ctx,spec,timeout,vb), provider.ParseOutput(out,manifest).
    - internal/config/{config,roles}.go — CONSUMED: Config (Timeout, MaxDuplicateRetries, MaxDiffBytes,
      MaxMdLines, BinaryExtensions, SubjectTargetChars), ResolveRoleModel("message",cfg).
    - internal/decompose/{arbiter,chain,decompose}.go — DO NOT EXIST YET. arbiter.go (P3.M3.T1),
      chain.go (P3.M3.T2), decompose.go (P3.M4.T1.S1 — the orchestrator). This task creates ONLY
      message.go (+ message_test.go).
    - internal/signal/* — DO NOT IMPORT. generateMessage/publishCommit are SIGNAL-FREE (findings §8:
      RestoreDefault is one-shot; loop-scoped signal arming is P3.M4.T1.S2's job).
    - cmd/, pkg/stagecoach/ — UNCHANGED (the orchestrator P3.M4.T1.S1 wires generateMessage/publishCommit).

  DELIVERABLES (2 new files, 0 edits to existing files, 0 breaking changes):
    CREATE internal/decompose/message.go — package `decompose`; ErrMessageFailed + ErrPublicationFailed
      sentinels; generateMessage (the bare message-role generate/dedupe/parse loop over a tree-to-tree
      diff); publishCommit (the serialized CommitTree+UpdateRefCAS publication); messageSystemPrompt +
      messageRecentSubjects (private re-ports of generate's unexported helpers).
    CREATE internal/decompose/message_test.go — stubtest-driven + real-git integration tests. Fixture
      helpers use DISTINCT msg*-prefixed names (parallel-safe vs planner_test.go's un-prefixed + the
      stager_test.go's stg* copies — findings §10).

  SUCCESS: `go build ./... && go test ./...` green; `go vet ./...` + `golangci-lint run` clean;
  `gofmt -l internal/ pkg/` empty; go.mod/go.sum UNCHANGED; all existing tests still pass; generateMessage
  returns the message on stub-success, a *generate.RescueError(ErrRescue) on parse-exhaustion, a
  *generate.RescueError(ErrTimeout) on timeout; generateMessage uses RenderBare (the message manifest);
  generateMessage reuses generate.ExtractSubject/IsDuplicate and prompt.BuildUserPayload; publishCommit
  returns newSHA + advances HEAD on success, returns *generate.CASError on CAS failure (HEAD unmoved);
  root commit (parentSHA="") uses no -p + all-zeros expectedOld; all generation infra failures wrap
  ErrMessageFailed, all publication infra failures wrap ErrPublicationFailed; CASError/RescueError are
  NOT wrapped (errors.As-able).

---

## Goal

**Feature Goal**: Implement the message generation + serialized publication primitives for multi-commit
decomposition (PRD §13.6.2 / §13.6.3 / FR-M6/M7/M8/M12) as a self-contained module
`internal/decompose/message.go`. `generateMessage(ctx, deps, treeA, treeB)` is the BARE message-role
invocation that is "a variant of generate.CommitStaged's loop that takes a diff string instead of calling
StagedDiff": it computes the per-concept concept diff via `deps.Git.TreeDiff(ctx, treeA, treeB, opts)`
(§13.6.3 invariant 2 — tree-to-tree, never index-vs-HEAD), derives the rescue parent + the prompt's
born-state from `RevParseHEAD`, builds the v1 system prompt (mature/fallback) + fetches fresh recent
subjects, and runs the SAME bounded generate→parse→dedupe retry loop as CommitStaged (reusing
`generate.ExtractSubject`/`generate.IsDuplicate`/`prompt.BuildUserPayload`), Rendering the resolved
message manifest in BARE mode. It returns the message, or a `*generate.RescueError` on generation
failure (timeout/parse-fail/duplicate-exhausted/non-zero-exit/cancel) carrying TreeSHA=treeB +
ParentSHA + Candidate for the §18.3 / FR-M12 rescue. `publishCommit(ctx, deps, tree, parentSHA, msg)` is
the serialized publication primitive (§13.6.3 / FR-M7): `CommitTree(tree, [parentSHA], msg) → newSHA`,
then `UpdateRefCAS(HEAD, newSHA, expectedOld)` where expectedOld = parentSHA (or all-zeros for a root
commit). It returns newSHA on success or a `*generate.CASError` (whose `.Error()` IS the §13.5
"HEAD moved" message) on CAS failure. Both are SIGNAL-FREE primitives (findings §8) consumed by the
orchestrator (P3.M4.T1.S1).

**Deliverable** (2 new files in the existing `decompose` package):
1. `internal/decompose/message.go` — `ErrMessageFailed` + `ErrPublicationFailed` sentinels;
   `generateMessage(ctx, deps, treeA, treeB string) (string, error)`;
   `publishCommit(ctx, deps, tree, parentSHA, msg string) (string, error)`;
   private `messageSystemPrompt` + `messageRecentSubjects`.
2. `internal/decompose/message_test.go` — stubtest-driven + real-git integration tests against a real
   temp git repo (fixture helpers with DISTINCT msg* names).

**Success Definition**:
- generateMessage success (stub "feat: add b"): returns "feat: add b".
- generateMessage dedupe retry (NewScript ["feat: existing","feat: fresh"], HEAD subject "feat:
  existing"): returns "feat: fresh" (FR32).
- generateMessage parse-exhaustion (stub "" for all attempts): returns non-nil err; `errors.As(err,
  &re)` true; `re.Kind == generate.ErrRescue`; `re.TreeSHA == treeB`.
- generateMessage timeout (cfg.Timeout=100ms, stub SleepMS=2000): returns non-nil err; `errors.As(err,
  &re)` true; `re.Kind == generate.ErrTimeout`; `errors.Is(err, context.DeadlineExceeded)` true.
- generateMessage uses RenderBare (the message manifest; nil TooledFlags is fine for bare — not asserted
  via error, but the stub's bare render succeeds).
- generateMessage empty-diff guard: treeA==treeB → returns non-nil err; `errors.Is(err, ErrMessageFailed)`.
- publishCommit success: returns newSHA; `headSHA == newSHA`; `git log --format=%B -n1 newSHA == msg`;
  HEAD's tree == the passed tree.
- publishCommit root commit (parentSHA="", unborn repo): returns newSHA; HEAD == newSHA; the commit has
  NO parent (`git log --format=%P -n1` == "").
- publishCommit CAS failure: pre-move HEAD via a concurrent commit (HEAD X→Z); publishCommit(tree,
  parentSHA=X, "msg") returns `*generate.CASError`; `ce.Expected == X`; `ce.Actual == Z`; HEAD STILL ==
  Z (the CAS refused to clobber; the dangling commit exists but HEAD is untouched); `ce.Error()`
  contains "HEAD moved".
- `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
  internal/ pkg/` empty; go.mod/go.sum UNCHANGED; `git status --short` = 2 new files, nothing else.

## User Persona

**Target User**: the decompose orchestrator (`internal/decompose/decompose.go`, P3.M4.T1.S1) and, by
extension, the end user running `stagecoach` on an un-staged working tree (the default action routes to
decompose per FR-M1). message.go is internal plumbing — NOT user-facing CLI text. The user never invokes
the message role directly; the orchestrator calls generateMessage once per (non-skipped) concept from the
planner's partition, possibly overlapped with the next stager, then publishCommit to land the commit.

**Use Case**: once the orchestrator has frozen `tree[i]` (via freezeSnapshot from stager.go) and verified
`tree[i] != tree[i-1]` (FR-M8 empty-skip, the orchestrator's job), it calls `msg, err :=
generateMessage(ctx, deps, tree[i-1], tree[i])` (possibly in a goroutine overlapped with stager[i+1]).
On success it calls `newSHA, err := publishCommit(ctx, deps, tree[i], newSHA[i-1], msg)` — the
serialized CAS-protected publication (commit[i] parents to newSHA[i-1]; HEAD moves only if it still
equals newSHA[i-1]). On generateMessage failure the orchestrator enters the per-concept rescue
(FR-M12, P3.M4.T1.S2) using the returned *generate.RescueError; on publishCommit CAS failure the
orchestrator aborts the run with the §13.5 message (prior commits stand).

**Pain Points Addressed**: (a) the message must reason over the TREE-TO-TREE concept diff (§13.6.3
invariant 2), NOT StagedDiff (index-vs-HEAD) — generateMessage is the single seam that swaps the diff
source while reusing the entire proven v1 generate/dedupe loop; (b) publication must be SERIALIZED
(commits land in strict CAS order even though generation may overlap) — publishCommit is one CAS step,
and the orchestrator's strict ordering comes from each CAS requiring HEAD == the previous newSHA;
(c) a CAS failure (user committed elsewhere) must NEVER clobber HEAD — publishCommit returns the
§13.5 message and leaves history untouched; (d) per-concept rescue (FR-M12) must carry the frozen
tree[i] + parent for manual recovery — generateMessage's *RescueError carries exactly that.

## Why

- **Closes the message + publication half of PRD §13.6.3 / §9.14 FR-M6/M7/M8/M12.** generateMessage is
  the third of the four decompose role-invocations (planner bare, stager tooled, message bare, arbiter
  bare) and publishCommit is the per-concept publication step. With these, the orchestrator
  (P3.M4.T1.S1) has its message + publication entry points; P3.M3.* (arbiter) and the single-shortcut
  path can assume they exist.
- **The bare, tree-diff variant of the proven v1 generate loop.** v1 already does "diff → system prompt
  → recent subjects → Render(bare) → Execute → ParseOutput → ExtractSubject → IsDuplicate → retry →
  accept" for ONE commit (generate.CommitStaged). generateMessage is the SAME loop with ONE delta: the
  diff source is `TreeDiff(treeA, treeB)` instead of `StagedDiff`. No new concept — it even reuses
  generate's exported RescueError/ErrTimeout/ErrRescue/ExtractSubject/IsDuplicate and the v1 prompt
  builders. publishCommit is CommitStaged's step-7+8 (CommitTree + UpdateRefCAS) factored out.
- **Unblocks the decompose pipeline (P3.M4).** The orchestrator cannot run until generateMessage +
  publishCommit exist — every concept needs a message (from the tree-to-tree diff) and a serialized
  CAS-protected commit. This is the 4th foundation file of the `internal/decompose/` package (roles.go,
  planner.go, stager.go, message.go).
- **Lowest-risk, maximal-reuse, backward-compatible.** Two NEW files in an EXISTING package (decompose);
  ZERO edits to any shipped file (roles/planner/stager/generate/git/prompt/provider/config all CONSUMED).
  go.mod/go.sum untouched (generate is a new decompose import but is already a package — no new module
  deps). No import cycle (generate does not import decompose — findings §9). generateMessage/publishCommit
  are consumed later (P3.M4.T1.S1); no caller wiring here → zero merge friction.

## What

One new file `internal/decompose/message.go` in the existing `decompose` package exporting two sentinels
+ two functions, plus two private helpers, and one new test file. No new dependencies. No caller wiring
(that is P3.M4.T1.S1). Specifically:

- **`ErrMessageFailed`** (exported package-level sentinel): `errors.New("decompose: message generation
  failed")`. Wrapped (%w) around generation-STEP INFRA failures (TreeDiff error, RevParseHEAD error,
  RecentMessages/RecentSubjects error, render error, empty-diff guard). The orchestrator detects them
  via `errors.Is(err, ErrMessageFailed)`. **Generation failures (timeout/parse/dup/non-zero/cancel) are
  NOT wrapped here** — they return `*generate.RescueError` directly so `errors.As(err, &re)` works.
  Mirrors callPlanner's ErrPlannerFailed + stageConcept's ErrStagerFailed.
- **`ErrPublicationFailed`** (exported package-level sentinel): `errors.New("decompose: publication
  failed")`. Wrapped around publication-STEP INFRA failures (CommitTree error). The CAS failure returns
  `*generate.CASError` directly (NOT wrapped) so `errors.As(err, &ce)` works. Non-CAS UpdateRefCAS
  failures propagate verbatim (git infra; matches CommitStaged's `return Result{}, err`).
- **`generateMessage(ctx context.Context, deps Deps, treeA, treeB string) (string, error)`**: the bare
  message-role generate/dedupe/parse loop over a tree-to-tree concept diff. Pipeline (findings §5):
  TreeDiff(treeA,treeB) → RevParseHEAD (parent+isUnborn) → system prompt + recent subjects →
  ResolveRoleModel("message") → the CommitStaged step-5 loop (Render BARE → Execute → ParseOutput →
  ExtractSubject → IsDuplicate → retry bounded by MaxDuplicateRetries) → return msg or *RescueError.
- **`publishCommit(ctx context.Context, deps Deps, tree, parentSHA, msg string) (string, error)`**: the
  serialized publication primitive. CommitTree(tree, parents, msg) → UpdateRefCAS(HEAD, newSHA,
  expectedOld) → return newSHA or *CASError. parents = [parentSHA] if parentSHA != "" else nil (root);
  expectedOld = parentSHA if parentSHA != "" else all-zeros (40).
- **`messageSystemPrompt` / `messageRecentSubjects`** (private): verbatim re-ports of generate.go's
  unexported `buildSystemPrompt` / `recentSubjects` (findings §7) — unexported in generate, so re-port
  privately to keep decompose self-contained (no edit to generate.go).

### Success Criteria

- [ ] `internal/decompose/message.go` is package `decompose`, has a file doc comment citing PRD
      §13.6.2/§13.6.3 + FR-M6/M7/M8/M12, and defines `ErrMessageFailed` + `ErrPublicationFailed` +
      `generateMessage` + `publishCommit` + the two private helpers EXACTLY as the contract (signatures
      `generateMessage(ctx, deps Deps, treeA, treeB string) (string, error)` and
      `publishCommit(ctx, deps Deps, tree, parentSHA, msg string) (string, error)`).
- [ ] generateMessage computes the concept diff via `deps.Git.TreeDiff(ctx, treeA, treeB,
      git.StagedDiffOptions{MaxDiffBytes, MaxMdLines, BinaryExtensions})` (tree-to-tree — §13.6.3
      invariant 2; NOT StagedDiff) and derives the rescue parent + isUnborn via `deps.Git.RevParseHEAD`.
- [ ] generateMessage Renders the message manifest from `deps.Roles.Message` in BARE mode
      (`deps.Roles.Message.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)`) and derives the
      message (provider, model) via `config.ResolveRoleModel("message", deps.Config)`.
- [ ] generateMessage runs the CommitStaged step-5 loop: `prompt.BuildUserPayload(diff, rejected)` →
      Render → `provider.Execute` → `provider.ParseOutput` → `generate.ExtractSubject` →
      `generate.IsDuplicate`, bounded by `deps.Config.MaxDuplicateRetries`, with the FR29
      retry-instruction prepend on parse-fail and the FR32 rejection-list append on duplicate.
- [ ] generateMessage returns the message on success; a `*generate.RescueError{Kind: ErrTimeout,
      TreeSHA: treeB, ParentSHA, Candidate, Cause}` on timeout; a `*generate.RescueError{Kind: ErrRescue,
      TreeSHA: treeB, ParentSHA, Candidate, Cause}` on parse-exhaustion/duplicate-exhaustion/cancel;
      an `ErrMessageFailed`-wrapped error on infra failures (TreeDiff/RevParseHEAD/recent/render) and
      the empty-diff guard. It does NOT touch the signal package.
- [ ] publishCommit calls `deps.Git.CommitTree(ctx, tree, parents, msg)` then
      `deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld)`; parents/expectedOld are root-aware
      (parentSHA=="" → nil parents + all-zeros expectedOld). On `errors.Is(err, git.ErrCASFailed)` it
      re-reads HEAD and returns `&generate.CASError{TreeSHA: tree, Expected: expectedOld, Actual, Message:
      msg}`; the CASError is NOT wrapped in ErrPublicationFailed. CommitTree failure is wrapped in
      ErrPublicationFailed.
- [ ] `go build ./... && go test ./...` GREEN; `go vet ./...` + `golangci-lint run` clean; `gofmt -l
      internal/ pkg/` empty; go.mod/go.sum UNCHANGED; only 2 git changes (message.go, message_test.go).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the contract + scope
boundary (findings §1/§2/§3); the generateMessage pipeline + the CommitStaged step-5 loop port (findings
§5); the internal parent/isUnborn derivation decision + RevParseHEAD concurrency-safety (findings §4);
publishCommit's root-aware CAS (findings §6); the two re-ported prompt helpers (findings §7); the
SIGNAL-FREE mandate + why (findings §8); the imports + no-cycle (findings §9); the test-fixture
collision rule (msg* prefix — findings §10); the test cases (findings §11); the validation gates
(findings §12). No prior decompose knowledge beyond roles.go's Deps + stager.go's freezeSnapshot output
is required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE empirical findings (contract + 13 sections of load-bearing facts)
- docfile: plan/002_a17bb6c8dc1d/P3M2T4S1/research/findings.md
  why: §1 the verbatim contract; §2 SCOPE (this task = the primitives generateMessage + publishCommit, NOT
       the orchestrator loop — "the core loop" = the generate/dedupe RETRY loop inside generateMessage);
       §3 the deliverables (one file message.go); §4 generateMessage's signature + internal derivation
       (parentSHA+isUnborn via RevParseHEAD, safe under concurrent staging); §5 the generateMessage body
       (the CommitStaged step-5 loop, diff source swapped); §6 publishCommit (root-aware CAS, reuses
       generate.CASError); §7 the two re-ported prompt helpers (generate.buildSystemPrompt/recentSubjects
       are UNEXPORTED — re-port privately); §8 SIGNAL-FREE (RestoreDefault is one-shot — loop signal is
       P3.M4.T1.S2); §9 imports + no-cycle; §10 the test-fixture msg*-prefix rule; §11 the test cases;
       §12 the validation gates; §13 the one-paragraph summary.
  critical: §2 (DO NOT implement the concept-iteration loop / overlap goroutines / signal arming — those
            are P3.M4; generateMessage + publishCommit are SIGNAL-FREE synchronous primitives); §4 (derive
            parentSHA + isUnborn INTERNALLY via RevParseHEAD — do NOT add a parentSHA param to
            generateMessage; RevParseHEAD is safe under concurrent staging because the stager mutates the
            INDEX not HEAD); §6 (publishCommit takes parentSHA EXPLICITLY — the CAS needs the EXACT
            expected-old = newSHA[i-1]; do NOT re-read HEAD for the CAS expected-old); §7 (re-port
            buildSystemPrompt + recentSubjects PRIVATELY — do NOT edit generate.go to export them);
            §8 (NO signal import in message.go); §10 (message_test.go MUST use msg*-prefixed fixture names).

# MUST READ — the v1 loop to PORT (CommitStaged step 5) + the exported types/functions to REUSE
- file: internal/generate/generate.go
  section: CommitStaged (the 10-step pipeline — read the WHOLE function) — generateMessage ports step 5
           (the generate→parse→dedupe loop) with the diff source swapped; publishCommit ports steps 7+8
           (CommitTree + UpdateRefCAS). The EXPORTED symbols to reuse: type RescueError{Kind, TreeSHA,
           ParentSHA, Candidate, Cause} + its Unwrap(); type CASError{TreeSHA, Expected, Actual, Message}
           + its Error() (which IS the §13.5 message) + Unwrap(); var ErrTimeout, ErrRescue, ErrCASFailed;
           func ExtractSubject(message) string; func IsDuplicate(subject, recent) bool. The UNEXPORTED
           buildSystemPrompt(ctx,g,cfg,isUnborn) + recentSubjects(ctx,g,isUnborn) are re-ported privately
           (findings §7) — they are NOT reachable from package decompose.
  why: the authoritative source for the loop's exact structure (the attempt loop, the DeadlineExceeded /
       Canceled / ExitError branching, the parseFail flag, the rejected slice, the candidate variable,
       the lastCause variable, the success break + the post-loop RescueError). Copy its CONTROL FLOW
       faithfully; swap: StagedDiff→TreeDiff, deps.Manifest→deps.Roles.Message, the implicit cfg.Model/
       cfg.Provider→ResolveRoleModel("message"), and DROP the signal.* + CommitTree/UpdateRefCAS/DiffTree/
       Result steps (those live in publishCommit / the orchestrator).
  pattern: the loop's execErr handling — `if errors.Is(execErr, context.DeadlineExceeded) { return
           &RescueError{Kind: ErrTimeout, TreeSHA: treeB, ParentSHA: parentSHA, Candidate: candidate,
           Cause: execErr} }` (immediate rescue on timeout, NO retry); `if errors.Is(execErr,
           context.Canceled) { return &RescueError{Kind: ErrRescue, ...} }`; else `lastCause = execErr`
           (non-zero exit — fall through to ParseOutput; stdout may be partial). MATCH THIS EXACTLY.
  gotcha: CommitStaged's loop uses `cfg.Model`/`cfg.Provider` for Render; generateMessage uses the
          ResolveRoleModel("message") pair (prov, mdl) — Render arg order is (model, provider, sys,
          payload, mode) so pass (mdl, prov, ...). CommitStaged uses `deps.Manifest`; generateMessage
          uses `deps.Roles.Message`. CommitStaged captures `out, _, execErr`; generateMessage does too.

# MUST READ — the SHIPPED Deps/RoleManifests (P3.M2.T1.S1) — generateMessage/publishCommit's input
- file: internal/decompose/roles.go
  section: type Deps struct { Git git.Git; Registry *provider.Registry; Config config.Config; Roles
           RoleManifests; Verbose *ui.Verbose } — the injectable collaborators. RoleManifests{Planner,
           Stager, Message, Arbiter provider.Manifest} — the message manifest is deps.Roles.Message (BARE).
  why: confirms Deps has Config + Roles + Git + Verbose but NO Models field (generateMessage derives the
       message (provider, model) via ResolveRoleModel — findings §4). publishCommit uses deps.Git.
       Do NOT edit this file (shipped; editing = conflict).

# MUST READ — the parallel siblings (callPlanner + stageConcept) — the patterns to mirror
- file: internal/decompose/planner.go
  section: callPlanner — the bare counterpart of generateMessage. The model derivation via
           ResolveRoleModel, the Render(RenderBare)→Execute→handle pattern, the ErrPlannerFailed
           sentinel, the retry-once-on-parse design.
  why: generateMessage mirrors callPlanner's structure (derive model → diff → system prompt + recent →
       Render BARE → loop) BUT generateMessage is the generate/dedupe/parse loop (callPlanner is the
       JSON parse + single-shortcut). The ErrMessageFailed sentinel mirrors ErrPlannerFailed. The
       ResolveRoleModel derivation is IDENTICAL.
- file: internal/decompose/stager.go
  section: stageConcept (tooled, no retry) + freezeSnapshot (WriteTree wrapper). freezeSnapshot's tree[i]
           output is generateMessage's treeB input; tree[i-1] is treeA.
  why: confirms the package conventions (file doc comment citing PRD sections; sentinel + function(s);
       model derivation via ResolveRoleModel; Render mode explicit; %w wrapping). generateMessage's
       *RescueError return + publishCommit's *CASError return are the message/publication analogues.

# MUST READ — TreeDiff (the concept-diff source) + the git plumbing for publishCommit
- file: internal/git/git.go
  section: TreeDiff(ctx, treeA, treeB, opts) — `git diff <treeA> <treeB>` with the same caps/excludes/
           FR3c binary filtering as StagedDiff (the tree-to-tree analogue). RevParseHEAD(ctx) → (sha,
           isUnborn, err) — the current HEAD (parent for rescue + isUnborn for prompt). CommitTree(ctx,
           tree, parents, msg) → (sha, err) — builds a dangling commit (parents nil/empty ⇒ root; message
           via stdin -F -). UpdateRefCAS(ctx, ref, newSHA, expectedOld) → error — the SOLE ref mutation;
           returns ErrCASFailed (wrapped) on mismatch. ErrCASFailed sentinel. EmptyTreeSHA const (the
           unborn tree[-1] base — the orchestrator resolves it, NOT generateMessage). StagedDiffOptions
           {MaxDiffBytes, MaxMdLines, Excludes, BinaryExtensions} — TreeDiff's opts.
  why: generateMessage calls TreeDiff(treeA, treeB, opts) for the concept diff + RevParseHEAD for the
       rescue parent/isUnborn. publishCommit calls CommitTree + UpdateRefCAS. The CAS-failure detection
       is `errors.Is(err, git.ErrCASFailed)`; the re-read for the §13.5 Actual is RevParseHEAD (mirrors
       CommitStaged's D5). opts fields come from cfg (MaxDiffBytes/MaxMdLines/BinaryExtensions).
  gotcha: TreeDiff is NOT unborn-aware — the caller (orchestrator) resolves trees via RevParseTree and
          converts the unborn base to EmptyTreeSHA. generateMessage just takes two tree SHAs. If
          treeA==treeB, TreeDiff returns ("", nil) — generateMessage guards the empty diff defensively
          (the orchestrator's FR-M8 check means this shouldn't fire, but guard anyway).

# MUST READ — the v1 prompt builders (CONSUMED by the re-ported helpers + the loop)
- file: internal/prompt/system.go
  section: BuildSystemPrompt(examples, hasMultiline, subjectTarget) (§17.1 mature) +
           BuildFallbackPrompt(subjectTarget) (§17.2 new-repo) + DetectMultiline(examples) (FR12).
  why: messageSystemPrompt calls these (the message role IS the §13.1–§13.5 agent unchanged — §13.6.2).
- file: internal/prompt/payload.go
  section: BuildUserPayload(diff, rejected) (§17.3 — the user payload for the loop).
  why: generateMessage calls BuildUserPayload each attempt (the rejected slice grows on duplicate retry).

# MUST READ — Render (bare) + Execute + ParseOutput (the provider seam)
- file: internal/provider/render.go
  section: `Manifest.Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode) (*CmdSpec,
           error)` — mode defaults to RenderBare (variadic). generateMessage MUST pass provider.RenderBare
           (the message role is bare). ARG ORDER: Render takes (model, provider, sys, payload, mode) — pass
           (mdl, prov, sysPrompt, payload, RenderBare). `*spec` derefs the pointer.
- file: internal/provider/executor.go
  section: `Execute(ctx, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout, stderr string, err
           error)` — err is context.DeadlineExceeded on timeout, context.Canceled on cancel, wrapped
           *exec.ExitError on non-zero exit. Execute internally calls vb.VerboseCommand +
           vb.VerboseRawOutput. deps.Verbose may be nil (nil-safe).
- file: internal/provider/parse.go
  section: `ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)` — the §12.9 5-step
           pipeline (trim → strip-fence → raw/json → normalize → trim+ok). ok==false ⇒ retry.
  why: generateMessage's provider calls. Match CommitStaged's usage: `out, _, execErr := Execute(...)`;
       `m, ok, _ := ParseOutput(out, deps.Roles.Message)`.

# MUST READ — ResolveRoleModel + Config fields
- file: internal/config/roles.go
  section: `func ResolveRoleModel(role string, cfg Config) (provider, model string)` — generateMessage
           calls ResolveRoleModel("message", deps.Config). Returns (provider, model) — note the RETURN
           ORDER vs Render's ARG order (findings §5).
- file: internal/config/config.go
  section: Config — Timeout (120s default), MaxDuplicateRetries (3), MaxDiffBytes (300000), MaxMdLines
           (100), BinaryExtensions (nil), SubjectTargetChars (50). config.Defaults() populates them.
  why: generateMessage reads deps.Config.{MaxDuplicateRetries, MaxDiffBytes, MaxMdLines,
       BinaryExtensions, SubjectTargetChars, Timeout}. publishCommit reads nothing from Config.

# MUST READ — the test infrastructure (stubtest) + the test-repo fixture pattern
- file: internal/stubtest/stubtest.go
  section: `Build(t)` (compiles cmd/stubagent ONCE, cached); `Manifest(bin, Options{Out, Exit, SleepMS,
           Stderr, Script, Counter, Output, StripCodeFence})` (single-response BARE manifest — nil
           BareFlags is FINE for RenderBare); `NewScript(t, bin, responses)` (call-varying for dedupe
           retry tests). The stub reads stdin → /dev/null, emits Options.Out on stdout, exits Options.Exit.
  why: message_test.go builds Deps{Git: git.New(repo), Config: config.Defaults(), Roles: RoleManifests{
       Message: stubtest.Manifest(...)}, Verbose: nil} (NO ResolveRoles). The message role is BARE → use
       stubtest.Manifest DIRECTLY (no tooledStubManifest helper — that's stager_test.go's for the TOOLED
       stager). For dedupe-retry tests use stubtest.NewScript (mirrors TestCommitStaged_DedupeRetryThenSuccess).
  gotcha: stubtest.Manifest sets TooledFlags=nil — but generateMessage uses RenderBare (nil BareFlags is a
          no-op append), so the bare render succeeds. (stageConcept needed a tooled helper because the
          stager is TOOLED; the message role is BARE — no helper needed.)

# MUST READ — the test-repo fixture helpers (copy into message_test.go with msg* prefix — findings §10)
- file: internal/generate/generate_test.go
  section: the fixture helpers (initRepo, writeFile, stageFile, headSHA, commitRaw, gitOut, runGit) +
           TestCommitStaged_Success + TestCommitStaged_DedupeRetryThenSuccess (the canonical
           stubtest+real-repo integration test pattern) + shaRe.
  why: message_test.go needs a real git repo to test generateMessage (build two trees via git add +
       write-tree) and publishCommit (commit + HEAD + CAS). Copy the fixture helpers VERBATIM but RENAME
       with a `msg` prefix (msgInitRepo, msgWriteFile, msgStageFile, msgCommitRaw, msgRunGit, msgGitOut,
       msgHeadSHA) to avoid colliding with BOTH planner_test.go's un-prefixed copies AND stager_test.go's
       stg* copies (all in package decompose — a duplicate declaration is a compile error).

# MUST READ — the stager PRP (P3.M2.T3.S1) — the immediate predecessor + the freezeSnapshot contract
- docfile: plan/002_a17bb6c8dc1d/P3M2T3S1/PRP.md
  section: freezeSnapshot (the §13.6.3 invariant-1 WriteTree wrapper) — its tree[i] output feeds
           generateMessage's treeB; the orchestrator passes tree[i-1] as treeA. The tooled stager +
           the per-concept snapshot model.
  why: confirms the snapshot/freeze semantics (tree[i] frozen before stager[i+1]) and that the message
       role is the BARE §13.1–§13.5 agent (unchanged). Confirms the test-fixture collision hazard (stager
       used stg*; this task uses msg*).

# MUST READ — the PRD spec (authoritative requirements)
- url: PRD.md §13.6.3 (invariant 2: message[i] reasons over `git diff tree[i-1] tree[i]`, NEVER index-vs-
       HEAD; invariant 3: update-refs serialize; the pipeline: snapshot[i]→message[i](tree-to-tree)→commit[i]→
       update-ref serialized in order)
- url: PRD.md §9.14 FR-M6 (per-concept snapshot + overlapped generation: message[i] uses the concept diff
       tree-to-tree) / FR-M7 (serialized publication: commit-tree -p newSHA[i-1] tree[i] msg[i]; update-ref
       CAS) / FR-M8 (empty-concept skip — the ORCHESTRATOR's job) / FR-M12 (message[i] generation fails →
       rescue for concept i; CAS failure → abort with §13.5 message; prior commits stand)
- url: PRD.md §13.1–§13.5 (the single-commit primitive generateMessage is a variant of — the loop + the
       commit-tree + update-ref CAS + the §13.5 "HEAD moved" message)
- url: PRD.md §13.6.2 (the message role: bare; "this IS the §13.1–§13.5 agent, unchanged")
- url: PRD.md §9.5–§9.7 (FR21–FR33: the generation + output-parsing + duplicate-rejection the loop runs)
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  roles.go            # SHIPPED (P3.M2.T1.S1) — READ (CONSUMED): Deps, RoleManifests. Deps has NO Models field.
  planner.go          # SHIPPED (P3.M2.T2.S1) — READ (PATTERN): callPlanner, ErrPlannerFailed (the bare sibling).
  stager.go           # SHIPPED (P3.M2.T3.S1) — READ (PATTERN): stageConcept, freezeSnapshot, ErrStagerFailed.
  message.go          # DOES NOT EXIST YET — THIS TASK CREATES IT.
internal/generate/
  generate.go         # READ (PORT THE LOOP + REUSE EXPORTED TYPES): CommitStaged step 5 (loop) + steps 7/8
                      #   (commit-tree + update-ref) + RescueError/CASError/ErrTimeout/ErrRescue/ExtractSubject/
                      #   IsDuplicate (all EXPORTED). buildSystemPrompt/recentSubjects (UNEXPORTED → re-port).
  dedupe.go           # READ (REUSE): ExtractSubject, IsDuplicate (exported).
  rescue.go           # READ (CONTEXT): FormatRescue (the §18.3 message the CLI prints from RescueError).
internal/git/
  git.go              # READ (CONSUMED): TreeDiff, RevParseHEAD, CommitTree, UpdateRefCAS, ErrCASFailed,
                      #   EmptyTreeSHA, StagedDiffOptions. (StatusPorcelain/WorkingTreeDiff/RevParseTree/ReadTree
                      #   are the planner/arbiter roles' concern — NOT this task.)
internal/prompt/
  system.go           # READ (CONSUMED): BuildSystemPrompt, BuildFallbackPrompt, DetectMultiline.
  payload.go          # READ (CONSUMED): BuildUserPayload.
  planner.go          # READ (CONTEXT): PlannerCommit (the concept type the orchestrator threads — NOT used
                      #   directly by generateMessage, which takes tree SHAs, not concepts).
internal/provider/
  render.go           # READ (CONSUMED): Manifest.Render(...,RenderBare), RenderBare.
  executor.go         # READ (CONSUMED): provider.Execute(ctx, spec, timeout, vb).
  parse.go            # READ (CONSUMED): provider.ParseOutput(out, manifest).
internal/config/
  config.go           # READ (CONSUMED): Config (Timeout, MaxDuplicateRetries, MaxDiffBytes, MaxMdLines,
                      #   BinaryExtensions, SubjectTargetChars), config.Defaults().
  roles.go            # READ (CONSUMED): ResolveRoleModel("message", cfg) → (provider, model).
internal/stubtest/
  stubtest.go         # READ (test infra): Build, Manifest (bare — fine for the message role), NewScript.
internal/generate/
  generate_test.go    # READ (test pattern): fixture helpers (copy + msg*-rename) + the canonical tests.
go.mod / go.sum       # UNCHANGED (module github.com/dustin/stagecoach; generate is a new decompose import
                      #   but generate is already a package — no new module deps).
.golangci.yml         # READ: errcheck/gosimple/govet/ineffassign/staticcheck/unused.
```

### Desired Codebase tree with files to be added

```bash
internal/decompose/message.go          # NEW — package `decompose` (4th file); the message + publication:
                                      #   var ErrMessageFailed = errors.New("decompose: message generation failed")
                                      #   var ErrPublicationFailed = errors.New("decompose: publication failed")
                                      #   func generateMessage(ctx context.Context, deps Deps, treeA, treeB string) (string, error)
                                      #   func publishCommit(ctx context.Context, deps Deps, tree, parentSHA, msg string) (string, error)
                                      #   func messageSystemPrompt(...) (private re-port of generate.buildSystemPrompt)
                                      #   func messageRecentSubjects(...) (private re-port of generate.recentSubjects)
internal/decompose/message_test.go     # NEW — stubtest (bare stubtest.Manifest + NewScript) + real-git
                                      #   integration tests (fixture helpers with msg*-prefixed names). Cases:
                                      #   generateMessage success/dedupe-retry/parse-rescue/timeout/empty-diff;
                                      #   publishCommit success/root-commit/CAS-failure.
# go.mod/go.sum UNCHANGED. roles.go + planner.go + stager.go + generate/* + git/* + prompt/* + provider/*
# + config/* + cmd/* + pkg/stagecoach all UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (SCOPE — this task is the PRIMITIVES, NOT the orchestrator — findings §2): generateMessage +
//   publishCommit are SIGNAL-FREE synchronous primitives consumed by the orchestrator (P3.M4.T1.S1). Do NOT
//   implement the concept-iteration loop (the "for i in 0..N-1"), the overlap goroutine scheduling
//   (stager[i+1] ∥ message[i] — the orchestrator launches generateMessage in a goroutine), the FR-M8
//   empty-skip comparison (tree[i]==tree[i-1] — the orchestrator compares BEFORE calling generateMessage),
//   or the loop-level signal arming (RestoreDefault is ONE-SHOT — can't be per-concept; that's P3.M4.T1.S2).
//   "The core loop" in the contract = the generate/dedupe RETRY loop INSIDE generateMessage (a variant of
//   CommitStaged's step-5 loop) — NOT the concept-iteration loop.

// CRITICAL (SIGNAL-FREE — findings §8): generateMessage and publishCommit MUST NOT import or call the
//   signal package. signal.RestoreDefault is a ONE-SHOT that PERMANENTLY stops signal delivery for the
//   process; calling it per concept in publishCommit would disable rescue-mode for all later concepts.
//   Instead, return typed errors (*generate.RescueError on generation failure, *generate.CASError on CAS
//   failure) carrying ALL the context (TreeSHA, ParentSHA, Candidate / TreeSHA, Expected, Actual, Message)
//   the orchestrator/CLI needs to print the rescue/§13.5 message. This is the SAME return-typed-error-
//   not-print pattern CommitStaged uses toward the CLI.

// CRITICAL (generateMessage derives parentSHA + isUnborn INTERNALLY — findings §4): generateMessage takes
//   ONLY (ctx, deps, treeA, treeB). It derives the rescue's parent + the prompt's born-state via
//   `parentSHA, isUnborn, _ := deps.Git.RevParseHEAD(ctx)`. Why safe under overlap: the concurrent
//   stager[i+1] mutates the INDEX (git add), NOT HEAD — RevParseHEAD is immune. Why correct: after concept
//   i-1 publishes, HEAD == newSHA[i-1] == the parent concept[i]'s commit will use. Do NOT add a parentSHA
//   param to generateMessage. (publishCommit takes parentSHA EXPLICITLY — see below.)

// CRITICAL (publishCommit takes parentSHA EXPLICITLY — findings §6): the CAS MUST use the EXACT expected-
//   old = newSHA[i-1] that the orchestrator holds. publishCommit does NOT re-read HEAD for the CAS
//   expected-old (that would race: HEAD could move between the orchestrator's decision and publishCommit's
//   re-read). The orchestrator passes newSHA[i-1] verbatim. publishCommit's CAS-failure re-read of HEAD is
//   ONLY for the §13.5 message's Actual field (after the CAS already failed — mirrors CommitStaged's D5).

// CRITICAL (reuse generate's EXPORTED types — do NOT define decompose RescuErr/CASError): generateMessage
//   returns *generate.RescueError (exported struct); publishCommit returns *generate.CASError (exported
//   struct whose .Error() IS the §13.5 message). The CLI/orchestrator already handle these (from
//   CommitStaged) via errors.As — DRY + uniform. Do NOT wrap them in ErrMessageFailed/ErrPublicationFailed
//   (must stay errors.As-able). ErrMessageFailed wraps only the INFRA failures (TreeDiff/RevParseHEAD/
//   recent/render/empty-diff); ErrPublicationFailed wraps only CommitTree failure.

// CRITICAL (re-port buildSystemPrompt + recentSubjects PRIVATELY — findings §7): generate.go's
//   buildSystemPrompt(ctx,g,cfg,isUnborn) and recentSubjects(ctx,g,isUnborn) are UNEXPORTED. message.go
//   (package decompose) cannot call them. Re-port them VERBATIM as private messageSystemPrompt /
//   messageRecentSubjects (~10 lines each). Do NOT edit generate.go to export them (shipped file; risk +
//   scope). This keeps decompose self-contained.

// CRITICAL (Render arg order vs ResolveRoleModel return order — findings §5): ResolveRoleModel returns
//   (provider, model); Render takes (model, provider, sys, payload, mode). So pass (mdl, prov, sysPrompt,
//   payload, provider.RenderBare). `*spec` derefs the Render pointer. deps.Config.Timeout → Execute.
//   deps.Verbose may be nil (nil-safe — generate uses deps.Verbose unconditionally).

// CRITICAL (the message role is BARE — findings §10): Render the message manifest with provider.RenderBare
//   (the message role is bare per §13.6.2 — "this IS the §13.1–§13.5 agent, unchanged"). Render's mode param
//   is VARIADIC and defaults to RenderBare, so OMITTING it would also be bare — but pass it explicitly for
//   clarity + parity with callPlanner/stageConcept. The stub's nil TooledFlags/BareFlags is fine for
//   RenderBare (append(nil) no-op) — so message_test.go uses stubtest.Manifest DIRECTLY (no tooled helper).

// CRITICAL (recent subjects fetched FRESH each call — findings §7): after concept[i-1] publishes, its
//   subject is in history — message[i]'s dedupe MUST include it (prevents duplicate subjects ACROSS the
//   run's concepts). messageRecentSubjects calls g.RecentSubjects(ctx, 50) each generateMessage call. Same
//   for the system prompt (CommitCount grows). This is the intended §13.6 behavior (matches CommitStaged).

// CRITICAL (root commit CAS — findings §6): when parentSHA=="" (concept 0 on an unborn repo), publishCommit
//   passes NO -p to CommitTree (parents=nil) and expectedOld = strings.Repeat("0", 40) (all-zeros) to
//   UpdateRefCAS — mirrors CommitStaged's isUnborn path. For concept i>0, parentSHA=newSHA[i-1] (real SHA),
//   expectedOld=newSHA[i-1].

// GOTCHA (test fixture name collision — findings §10): planner_test.go owns the UN-PREFIXED names
//   (initRepo, writeFile, commitRaw, runGit); stager_test.go owns the stg*-prefixed names. message_test.go
//   is ALSO package decompose — a duplicate declaration is a COMPILE ERROR. Use DISTINCT msg*-prefixed names
//   (msgInitRepo, msgWriteFile, msgStageFile, msgCommitRaw, msgRunGit, msgGitOut, msgHeadSHA) — copied
//   verbatim from internal/generate/generate_test.go (those helpers are unimportable from package decompose).

// GOTCHA (TreeDiff empty on treeA==treeB — findings §5): the orchestrator's FR-M8 check (tree[i]==tree[i-1]
//   → skip) means generateMessage is only called when treeA != treeB, so TreeDiff is non-empty. But guard
//   defensively: if diff == "", return fmt.Errorf("%w: empty concept diff %s..%s", ErrMessageFailed, treeA,
//   treeB). Do NOT treat it as a rescue (no snapshot-then-CAS semantics; it's a caller-contract violation).

// GOTCHA (package decompose is growing — message.go is the 4th file): roles.go, planner.go, stager.go are
//   shipped; message.go is 4th. Same `package decompose`. Add a file doc comment (cite §13.6.2/§13.6.3 +
//   FR-M6/M7/M8/M12). No import cycle (decompose → generate is one-way; generate does not import decompose).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/decompose/message.go — package decompose (the 4th file after roles.go + planner.go + stager.go)

// ErrMessageFailed is the sentinel for message-generation INFRA failures (TreeDiff error, RevParseHEAD
// error, RecentMessages/RecentSubjects error, render error, empty-diff guard). Generation failures
// (timeout/parse/duplicate-exhaustion/non-zero-exit/cancel) return *generate.RescueError DIRECTLY (not
// wrapped) so errors.As(err, &re) works. The orchestrator (P3.M4.T1.S1) dispatches: errors.As(RescueError)
// → per-concept rescue (FR-M12); errors.Is(ErrMessageFailed) → generation-step infra failure.
var ErrMessageFailed = errors.New("decompose: message generation failed")

// ErrPublicationFailed is the sentinel for publication-step INFRA failures (CommitTree error). The CAS
// failure returns *generate.CASError DIRECTLY (not wrapped) so errors.As(err, &ce) works. Non-CAS
// UpdateRefCAS failures propagate verbatim (git infra; matches CommitStaged).
var ErrPublicationFailed = errors.New("decompose: publication failed")

// (generateMessage + publishCommit + the two private helpers — see Implementation Tasks)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/decompose/message.go — package doc + imports + sentinels
  - FILE DOC: cite PRD §13.6.2 (message role: bare, "this IS the §13.1–§13.5 agent, unchanged") /
    §13.6.3 (invariant 2: message[i] reasons over tree-to-tree concept diff; invariant 3: update-refs
    serialize) + §9.14 FR-M6/M7/M8/M12. Note message.go is the message-role generation (generateMessage)
    + the serialized publication (publishCommit). Note generateMessage is "a variant of generate.CommitStaged's
    loop that takes a diff string instead of calling StagedDiff" (tree-to-tree via TreeDiff). Note
    publishCommit is the CommitTree + UpdateRefCAS pair (a CAS-protected publication step). Note both are
    SIGNAL-FREE primitives consumed by the orchestrator (P3.M4.T1.S1).
  - IMPORTS: "context"; "errors"; "fmt"; "strings";
    "github.com/dustin/stagecoach/internal/config"; "github.com/dustin/stagecoach/internal/generate";
    "github.com/dustin/stagecoach/internal/git"; "github.com/dustin/stagecoach/internal/prompt";
    "github.com/dustin/stagecoach/internal/provider".
    (NO "ui" — deps.Verbose is the ui.Verbose handle via Deps; no direct ui symbol. NO "signal" — findings §8.
    Verify at Task 7: drop any unused import (golangci/unused + go vet reject it). All these are already
    used by roles.go EXCEPT generate — confirm generate is imported (RescueError/CASError/ExtractSubject/
    IsDuplicate) and git (TreeDiff/CommitTree/UpdateRefCAS/ErrCASFailed/StagedDiffOptions).)
  - DEFINE `var ErrMessageFailed = errors.New("decompose: message generation failed")` + `var
    ErrPublicationFailed = errors.New("decompose: publication failed")` with the doc comments above.

Task 2: CREATE internal/decompose/message.go — messageSystemPrompt + messageRecentSubjects (private)
  - (Define these BEFORE generateMessage so generateMessage's body reads top-down — or after; Go order is
    irrelevant. Define them as small private helpers.)
  - `func messageSystemPrompt(ctx context.Context, g git.Git, cfg config.Config, isUnborn bool) (string,
    error)`: verbatim re-port of generate.buildSystemPrompt (findings §7). Body:
      if isUnborn { return prompt.BuildFallbackPrompt(cfg.SubjectTargetChars), nil }
      n, err := g.CommitCount(ctx); if err != nil { return "", err }
      if n <= 1 { return prompt.BuildFallbackPrompt(cfg.SubjectTargetChars), nil }
      msgs, err := g.RecentMessages(ctx, 20); if err != nil { return "", err }
      return prompt.BuildSystemPrompt(msgs, prompt.DetectMultiline(msgs), cfg.SubjectTargetChars), nil
  - `func messageRecentSubjects(ctx context.Context, g git.Git, isUnborn bool) ([]string, error)`:
      if isUnborn { return nil, nil }
      return g.RecentSubjects(ctx, 50)
  - DOC COMMENTS: cite that these are verbatim re-ports of generate.go's UNEXPORTED buildSystemPrompt /
    recentSubjects (re-ported privately to keep decompose self-contained — do NOT edit generate.go). Note
    they are called FRESH each generateMessage call (recent subjects grow as concepts publish — intentional;
    prevents cross-concept duplicate subjects; matches CommitStaged).

Task 3: CREATE internal/decompose/message.go — generateMessage (the bare message-role loop)
  - SIGNATURE: `func generateMessage(ctx context.Context, deps Deps, treeA, treeB string) (string, error)`.
  - BODY (see Implementation Patterns for the exact code + findings §5):
      // 1. Concept diff (§13.6.3 invariant 2 — tree-to-tree, never index-vs-HEAD).
      diff, err := deps.Git.TreeDiff(ctx, treeA, treeB, git.StagedDiffOptions{
          MaxDiffBytes:     deps.Config.MaxDiffBytes,
          MaxMdLines:       deps.Config.MaxMdLines,
          BinaryExtensions: deps.Config.BinaryExtensions,
      })
      if err != nil { return "", fmt.Errorf("%w: tree diff: %w", ErrMessageFailed, err) }
      if diff == "" { return "", fmt.Errorf("%w: empty concept diff %s..%s", ErrMessageFailed, treeA, treeB) }
      // 2. Current HEAD (parent for rescue + isUnborn for prompt). Safe under overlap: stager mutates the
      //    INDEX, not HEAD.
      parentSHA, isUnborn, err := deps.Git.RevParseHEAD(ctx)
      if err != nil { return "", fmt.Errorf("%w: rev-parse head: %w", ErrMessageFailed, err) }
      // 3. System prompt (v1, unchanged — the message role IS the §13.1–§13.5 agent) + recent subjects
      //    (FRESH — includes just-committed concepts for cross-concept dedupe).
      sysPrompt, err := messageSystemPrompt(ctx, deps.Git, deps.Config, isUnborn)
      if err != nil { return "", fmt.Errorf("%w: system prompt: %w", ErrMessageFailed, err) }
      recent, err := messageRecentSubjects(ctx, deps.Git, isUnborn)
      if err != nil { return "", fmt.Errorf("%w: recent subjects: %w", ErrMessageFailed, err) }
      // 4. Derive the message (provider, model) — Deps has no Models field (findings §4).
      prov, mdl := config.ResolveRoleModel("message", deps.Config)
      resolved := deps.Roles.Message.Resolve()
      retryInstr := *resolved.RetryInstruction
      // 5. GENERATION+DEDUPE LOOP — a variant of CommitStaged's step-5 loop (diff = concept diff, not StagedDiff).
      var rejected []string
      var candidate string
      var parseFail bool
      var lastCause error
      var msg string
      success := false
      for attempt := 0; attempt <= deps.Config.MaxDuplicateRetries; attempt++ {
          payload := prompt.BuildUserPayload(diff, rejected)
          if parseFail { payload = retryInstr + "\n\n" + payload } // FR29 corrective preamble
          spec, rerr := deps.Roles.Message.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)
          if rerr != nil { return "", fmt.Errorf("%w: render: %w", ErrMessageFailed, rerr) }
          out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)
          if execErr != nil {
              if errors.Is(execErr, context.DeadlineExceeded) {
                  return "", &generate.RescueError{Kind: generate.ErrTimeout, TreeSHA: treeB,
                      ParentSHA: parentSHA, Candidate: candidate, Cause: execErr} // immediate rescue, NO retry
              }
              if errors.Is(execErr, context.Canceled) {
                  return "", &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: treeB,
                      ParentSHA: parentSHA, Candidate: candidate, Cause: execErr}
              }
              lastCause = execErr // non-zero exit — fall through to ParseOutput (stdout may be partial)
          } else { lastCause = nil }
          m, ok, _ := provider.ParseOutput(out, deps.Roles.Message)
          if !ok {
              parseFail = true; candidate = m
              deps.Verbose.VerboseRetry(attempt+1, "parse failed (no valid commit message)")
              continue // FR29 retry
          }
          parseFail = false
          subject := generate.ExtractSubject(m)
          if generate.IsDuplicate(subject, recent) {
              rejected = append(rejected, subject); candidate = m
              deps.Verbose.VerboseRetry(attempt+1, fmt.Sprintf("subject %q matches an existing commit", subject))
              continue // FR32 retry
          }
          msg = m; success = true; break
      }
      if !success {
          return "", &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: treeB,
              ParentSHA: parentSHA, Candidate: candidate, Cause: lastCause}
      }
      return msg, nil
  - DOC COMMENT: cite PRD §13.6.2/§13.6.3 + FR-M6/M7/M12; note it is "a variant of generate.CommitStaged's
    loop that takes a diff string instead of calling StagedDiff" (tree-to-tree via TreeDiff — invariant 2);
    note the internal parentSHA/isUnborn derivation via RevParseHEAD (safe under overlap); note the FRESH
    recent subjects (cross-concept dedupe); note it returns *generate.RescueError on generation failure
    (NOT wrapped — errors.As-able) and ErrMessageFailed-wrapped on infra failure; note it is SIGNAL-FREE.
  - GOTCHA: Render's (model, provider) arg order vs ResolveRoleModel's (provider, model) return — pass
    (mdl, prov). `*spec` derefs. The TreeSHA in every RescueError is treeB (the concept tree). The ParentSHA
    is the RevParseHEAD-derived current HEAD (== newSHA[i-1]).

Task 4: CREATE internal/decompose/message.go — publishCommit (the serialized publication primitive)
  - SIGNATURE: `func publishCommit(ctx context.Context, deps Deps, tree, parentSHA, msg string) (string, error)`.
  - BODY (findings §6):
      var parents []string
      if parentSHA != "" { parents = []string{parentSHA} } // root commit (concept 0 on unborn) ⇒ nil parents
      newSHA, err := deps.Git.CommitTree(ctx, tree, parents, msg)
      if err != nil { return "", fmt.Errorf("%w: commit-tree: %w", ErrPublicationFailed, err) }
      expectedOld := parentSHA
      if parentSHA == "" { expectedOld = strings.Repeat("0", 40) } // all-zeros for root CAS (CommitStaged parity)
      if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
          if errors.Is(err, git.ErrCASFailed) {
              actual, _, _ := deps.Git.RevParseHEAD(ctx) // re-read for the §13.5 message's Actual (D5)
              return "", &generate.CASError{TreeSHA: tree, Expected: expectedOld, Actual: actual, Message: msg}
          }
          return "", err // non-CAS infra — propagate verbatim (matches CommitStaged)
      }
      return newSHA, nil
  - DOC COMMENT: cite PRD §13.6.3 invariant 3 (update-refs serialize) + §9.14 FR-M7 (commit-tree -p
    newSHA[i-1] tree[i] msg[i]; update-ref CAS) + FR-M12 (CAS failure → abort with §13.5 message; prior
    commits stand). Note parentSHA is EXPLICIT (the exact CAS expected-old = newSHA[i-1] the orchestrator
    holds — do NOT re-read HEAD for the expected-old; the re-read is ONLY for the §13.5 Actual after the CAS
    fails). Note the root-commit path (parentSHA=="" ⇒ no -p + all-zeros expectedOld). Note it returns
    *generate.CASError (whose .Error() IS the §13.5 message) on CAS — NOT wrapped (errors.As-able). Note it
    is SIGNAL-FREE.
  - GOTCHA: do NOT wrap the CASError in ErrPublicationFailed (must stay errors.As-able). The non-CAS
    UpdateRefCAS failure propagates VERBATIM (not wrapped) — matches CommitStaged's `return Result{}, err`.

Task 5: CREATE internal/decompose/message_test.go — package + imports + msg*-prefixed fixture helpers
  - PACKAGE: `decompose` (internal test — generateMessage/publishCommit/ErrMessageFailed/ErrPublicationFailed
    visible).
  - IMPORTS: "context"; "errors"; "os"; "os/exec"; "regexp"; "strings"; "testing"; "time";
    "github.com/dustin/stagecoach/internal/config"; "github.com/dustin/stagecoach/internal/generate";
    "github.com/dustin/stagecoach/internal/git"; "github.com/dustin/stagecoach/internal/provider";
    "github.com/dustin/stagecoach/internal/stubtest".
    (NOTE: only import what each test uses; drop unused. generate for RescueError/CASError/ErrTimeout/
    ErrRescue; git for git.New + StagedDiffOptions; provider for provider.Manifest; regexp for the shaRe.
    prompt is NOT needed (generateMessage takes tree SHAs, not concepts).)
  - COPY the fixture helpers from internal/generate/generate_test.go VERBATIM but RENAME with the `msg`
    prefix (findings §10): msgInitRepo, msgWriteFile, msgStageFile, msgCommitRaw, msgRunGit, msgGitOut,
    msgHeadSHA. Also copy `var shaRe = regexp.MustCompile(...)`. (They are unimportable from package
    decompose — generate_test owns them in package generate; renaming avoids colliding with BOTH
    planner_test.go's un-prefixed copies AND stager_test.go's stg* copies in package decompose.)
  - DEFINE `func messageDeps(t *testing.T, repo string, m provider.Manifest) Deps`:
      return Deps{Git: git.New(repo), Config: config.Defaults(), Roles: RoleManifests{Message: m}, Verbose: nil}
    (NO ResolveRoles — the test injects the manifest directly, mirroring stagerDeps. The message role is
    BARE → use stubtest.Manifest DIRECTLY, no tooled helper.)

Task 6: CREATE internal/decompose/message_test.go — the generateMessage test cases (findings §11)
  - TestGenerateMessage_Success: repo := t.TempDir(); msgInitRepo(t, repo); msgCommitRaw(t, repo,
    "initial"); build two trees: msgWriteFile(t, repo, "a.txt", "a\n"); msgStageFile(t, repo, "a.txt");
    treeA := msgGitOut(t, repo, "write-tree"); msgWriteFile(t, repo, "b.txt", "b\n"); msgStageFile(t, repo,
    "b.txt"); treeB := msgGitOut(t, repo, "write-tree"); bin := stubtest.Build(t); m := stubtest.Manifest(bin,
    stubtest.Options{Out: "feat: add b"}); deps := messageDeps(t, repo, m); msg, err := generateMessage(ctx,
    deps, treeA, treeB). Assert: err == nil; msg == "feat: add b".
  - TestGenerateMessage_DedupeRetryThenSuccess: msgCommitRaw(t, repo, "feat: existing") (HEAD subject =
    "feat: existing"); build treeA/treeB as above; m := stubtest.NewScript(t, bin, []string{"feat:
    existing", "feat: fresh"}); msg, err := generateMessage(ctx, deps, treeA, treeB). Assert: err == nil;
    msg == "feat: fresh". (FR32: the first subject duplicates the HEAD subject → rejected → retry → fresh.)
  - TestGenerateMessage_ParseFailRescue: m := stubtest.Manifest(bin, stubtest.Options{Out: ""}) (empty
    output → ParseOutput ok=false for all MaxDuplicateRetries+1 attempts); _, err := generateMessage(ctx,
    deps, treeA, treeB). Assert: err != nil; `var re *generate.RescueError; errors.As(err, &re)` true;
    re.Kind == generate.ErrRescue; re.TreeSHA == treeB.
  - TestGenerateMessage_Timeout: cfg := config.Defaults(); cfg.Timeout = 100*time.Millisecond; m :=
    stubtest.Manifest(bin, stubtest.Options{SleepMS: 2000}); deps := messageDeps(...) BUT with cfg (build
    Deps manually: Deps{Git: git.New(repo), Config: cfg, Roles: RoleManifests{Message: m}, Verbose: nil});
    _, err := generateMessage(ctx, deps, treeA, treeB). Assert: err != nil; errors.As(err, &re) true;
    re.Kind == generate.ErrTimeout; errors.Is(err, context.DeadlineExceeded) true (the %w chain reaches it
    via RescueError.Unwrap → ErrTimeout... NOTE: verify whether RescueError chains the Cause — generate.go's
    RescueError.Unwrap() returns e.Kind, NOT e.Cause; so errors.Is(err, context.DeadlineExceeded) is true
    ONLY if ErrTimeout IS context.DeadlineExceeded. CHECK generate.go: ErrTimeout = errors.New(...) — a
    DISTINCT sentinel, NOT context.DeadlineExceeded. So assert errors.Is(err, generate.ErrTimeout) instead
    — do NOT assert errors.Is(err, context.DeadlineExceeded) unless re.Cause is chained. Read generate.go's
    RescueError.Unwrap before asserting.).
  - TestGenerateMessage_EmptyDiff: treeA == treeB (build ONE tree, pass it twice); _, err :=
    generateMessage(ctx, deps, treeA, treeA). Assert: err != nil; errors.Is(err, ErrMessageFailed) true;
    strings.Contains(err.Error(), "empty concept diff") true.
  - (Optionally) TestGenerateMessage_BareRender — assert the stub manifest (nil TooledFlags/BareFlags)
    renders + executes without error (structurally guaranteed; a smoke test). Low priority.

Task 7: CREATE internal/decompose/message_test.go — the publishCommit test cases (findings §11)
  - TestPublishCommit_Success: repo := t.TempDir(); msgInitRepo(t, repo); msgCommitRaw(t, repo, "initial");
    parentSHA := msgHeadSHA(t, repo); build a tree: msgWriteFile(t, repo, "new.txt", "hello\n");
    msgStageFile(t, repo, "new.txt"); tree := msgGitOut(t, repo, "write-tree"); deps := messageDeps(t,
    repo, stubtest.Manifest(bin, stubtest.Options{})) (manifest irrelevant — publishCommit does NOT call
    the agent); newSHA, err := publishCommit(ctx, deps, tree, parentSHA, "feat: add new"). Assert: err ==
    nil; shaRe.MatchString(newSHA); msgHeadSHA(t, repo) == newSHA; msgGitOut(t, repo, "log", "--format=%B",
    "-n1", newSHA) == "feat: add new"; HEAD's tree == tree (msgGitOut(t, repo, "rev-parse", "HEAD^{tree}")
    == tree).
  - TestPublishCommit_RootCommit: repo := t.TempDir(); msgInitRepo(t, repo) (NO initial commit — unborn);
    build a tree: msgWriteFile(t, repo, "root.txt", "x\n"); msgStageFile(t, repo, "root.txt"); tree :=
    msgGitOut(t, repo, "write-tree"); newSHA, err := publishCommit(ctx, deps, tree, "", "feat: root").
    Assert: err == nil; msgHeadSHA(t, repo) == newSHA; msgGitOut(t, repo, "log", "--format=%P", "-n1") ==
    "" (NO parent — root commit).
  - TestPublishCommit_CASFailure: repo := t.TempDir(); msgInitRepo(t, repo); msgCommitRaw(t, repo,
    "initial"); parentSHA := msgHeadSHA(t, repo) (== X); msgCommitRaw(t, repo, "concurrent") (HEAD now ==
    Z — simulates a concurrent commit moving HEAD); actualZ := msgHeadSHA(t, repo); build tree (as above);
    _, err := publishCommit(ctx, deps, tree, parentSHA, "feat: msg"). Assert: err != nil; `var ce
    *generate.CASError; errors.As(err, &ce)` true; ce.Expected == parentSHA (== X); ce.Actual == actualZ (==
    Z); msgHeadSHA(t, repo) == actualZ (HEAD UNMOVED — the CAS refused to clobber; the dangling commit
    exists but HEAD is untouched); strings.Contains(ce.Error(), "HEAD moved") true.
  - GOTCHA: publishCommit does NOT call the agent — the Deps.Manifest is irrelevant for publishCommit tests
    (any manifest; the default stubtest.Manifest(bin, stubtest.Options{}) is fine). Only generateMessage
    tests need the stub to behave.

Task 8: VERIFY — build + vet + lint + format + status
  - RUN: `go build ./... && go test ./... && go vet ./... && golangci-lint run && gofmt -l internal/ pkg/`.
  - ASSERT: GREEN; gofmt output empty; `git status --short` shows ONLY message.go + message_test.go.
  - FIX any unused import (drop it), any golangci errcheck/gosimple/govet/ineffassign/staticcheck/unused
    issue, any format drift. Re-run until clean.
```

### Implementation Patterns & Key Details

```go
// PATTERN: generateMessage — the CommitStaged step-5 loop, diff source swapped (findings §5).
//   Faithfully port generate.CommitStaged's loop; the ONLY deltas: diff = TreeDiff (not StagedDiff),
//   deps.Roles.Message + RenderBare (not deps.Manifest), ResolveRoleModel("message") (not cfg.Model/
//   cfg.Provider), DROP signal.* + CommitTree/UpdateRefCAS/DiffTree/Result (publishCommit + orchestrator).
func generateMessage(ctx context.Context, deps Deps, treeA, treeB string) (string, error) {
    diff, err := deps.Git.TreeDiff(ctx, treeA, treeB, git.StagedDiffOptions{
        MaxDiffBytes: deps.Config.MaxDiffBytes, MaxMdLines: deps.Config.MaxMdLines,
        BinaryExtensions: deps.Config.BinaryExtensions,
    })
    if err != nil { return "", fmt.Errorf("%w: tree diff: %w", ErrMessageFailed, err) }
    if diff == "" { return "", fmt.Errorf("%w: empty concept diff %s..%s", ErrMessageFailed, treeA, treeB) }
    parentSHA, isUnborn, err := deps.Git.RevParseHEAD(ctx)
    if err != nil { return "", fmt.Errorf("%w: rev-parse head: %w", ErrMessageFailed, err) }
    sysPrompt, err := messageSystemPrompt(ctx, deps.Git, deps.Config, isUnborn)
    if err != nil { return "", fmt.Errorf("%w: system prompt: %w", ErrMessageFailed, err) }
    recent, err := messageRecentSubjects(ctx, deps.Git, isUnborn)
    if err != nil { return "", fmt.Errorf("%w: recent subjects: %w", ErrMessageFailed, err) }
    prov, mdl := config.ResolveRoleModel("message", deps.Config)
    resolved := deps.Roles.Message.Resolve()
    retryInstr := *resolved.RetryInstruction
    var rejected []string
    var candidate string
    var parseFail, success bool
    var lastCause error
    var msg string
    for attempt := 0; attempt <= deps.Config.MaxDuplicateRetries; attempt++ {
        payload := prompt.BuildUserPayload(diff, rejected)
        if parseFail { payload = retryInstr + "\n\n" + payload }
        spec, rerr := deps.Roles.Message.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)
        if rerr != nil { return "", fmt.Errorf("%w: render: %w", ErrMessageFailed, rerr) }
        out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)
        if execErr != nil {
            if errors.Is(execErr, context.DeadlineExceeded) {
                return "", &generate.RescueError{Kind: generate.ErrTimeout, TreeSHA: treeB,
                    ParentSHA: parentSHA, Candidate: candidate, Cause: execErr}
            }
            if errors.Is(execErr, context.Canceled) {
                return "", &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: treeB,
                    ParentSHA: parentSHA, Candidate: candidate, Cause: execErr}
            }
            lastCause = execErr
        } else { lastCause = nil }
        m, ok, _ := provider.ParseOutput(out, deps.Roles.Message)
        if !ok {
            parseFail, candidate = true, m
            deps.Verbose.VerboseRetry(attempt+1, "parse failed (no valid commit message)")
            continue
        }
        parseFail = false
        subject := generate.ExtractSubject(m)
        if generate.IsDuplicate(subject, recent) {
            rejected, candidate = append(rejected, subject), m
            deps.Verbose.VerboseRetry(attempt+1, fmt.Sprintf("subject %q matches an existing commit", subject))
            continue
        }
        msg, success = m, true
        break
    }
    if !success {
        return "", &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: treeB,
            ParentSHA: parentSHA, Candidate: candidate, Cause: lastCause}
    }
    return msg, nil
}

// PATTERN: publishCommit — the CommitStaged step-7+8 factored out (findings §6).
func publishCommit(ctx context.Context, deps Deps, tree, parentSHA, msg string) (string, error) {
    var parents []string
    if parentSHA != "" { parents = []string{parentSHA} }
    newSHA, err := deps.Git.CommitTree(ctx, tree, parents, msg)
    if err != nil { return "", fmt.Errorf("%w: commit-tree: %w", ErrPublicationFailed, err) }
    expectedOld := parentSHA
    if parentSHA == "" { expectedOld = strings.Repeat("0", 40) }
    if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
        if errors.Is(err, git.ErrCASFailed) {
            actual, _, _ := deps.Git.RevParseHEAD(ctx)
            return "", &generate.CASError{TreeSHA: tree, Expected: expectedOld, Actual: actual, Message: msg}
        }
        return "", err
    }
    return newSHA, nil
}
```

### Integration Points

```yaml
CONSUMED-BY (NOT this task — the orchestrator P3.M4.T1.S1 wires these):
  - generateMessage: the orchestrator calls `msg, err := generateMessage(ctx, deps, tree[i-1], tree[i])`
    for each NON-skipped concept (after freezeSnapshot froze tree[i] and the FR-M8 check confirmed
    tree[i] != tree[i-1]). On *generate.RescueError → per-concept rescue (FR-M12); on ErrMessageFailed →
    generation-step infra failure. (tree[i-1] for i==0 on an unborn repo = git.EmptyTreeSHA — the
    orchestrator resolves it via RevParseTree, NOT generateMessage.)
  - publishCommit: the orchestrator calls `newSHA, err := publishCommit(ctx, deps, tree[i], newSHA[i-1],
    msg)` (newSHA[i-1] for i==0 on unborn = "" → root commit). On *generate.CASError → abort the run with
    the §13.5 message (prior commits stand); on success → thread newSHA into the next concept's parentSHA.

NO CONFIG CHANGES: generateMessage + publishCommit read existing Config fields only (Timeout,
  MaxDuplicateRetries, MaxDiffBytes, MaxMdLines, BinaryExtensions, SubjectTargetChars). No new config keys.

NO ROUTE/CLI CHANGES: internal package; no cmd/ or pkg/stagecoach edits (the orchestrator P3.M4.T1.S1 +
  CLI P4 wire these).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after creating message.go — fix before proceeding
go build ./internal/decompose/                     # compile the new file in isolation
go vet ./internal/decompose/                       # vet the package
gofmt -w internal/decompose/message.go             # format
golangci-lint run ./internal/decompose/...         # lint (errcheck/gosimple/govet/ineffassign/staticcheck/unused)

# Project-wide validation
go build ./... && go vet ./... && golangci-lint run && gofmt -l internal/ pkg/

# Expected: Zero errors. If errors exist, READ the output and fix before proceeding.
# Common: unused import (drop it — generate/git/etc. must ALL be used), the Render arg-order mismatch
# (ResolveRoleModel returns (provider,model); Render takes (model,provider,sys,payload,mode)).
```

### Level 2: Unit / Integration Tests (Component Validation)

```bash
# Test the new package as it is created
go test ./internal/decompose/ -v -run 'TestGenerateMessage|TestPublishCommit'

# Full decompose package (ensure no regression in roles/planner/stager tests)
go test ./internal/decompose/ -v

# Full test suite for affected areas (generate is imported — ensure no regression)
go test ./internal/generate/ ./internal/decompose/ -v

# Expected: All tests pass. If failing, debug root cause and fix the implementation (NOT the shipped tests).
# Key assertions: generateMessage success/dedupe/parse-rescue/timeout/empty-diff; publishCommit success/
# root/CAS-failure. See findings §11 for the exact cases + the RescueError.Unwrap caveat in the timeout test.
```

### Level 3: Integration Testing (System Validation)

```bash
# No server/CLI wiring here (the orchestrator P3.M4.T1.S1 + CLI P4 do that). The integration validation
# is the real-git test suite in message_test.go (Level 2), which exercises:
#   - generateMessage against a real temp git repo with two frozen trees (git write-tree) + a stub agent;
#   - publishCommit against a real temp git repo: real CommitTree + UpdateRefCAS, HEAD advancement verified
#     via git rev-parse / git log, the CAS failure simulated by a pre-publish concurrent commit.

# Manual smoke (optional, in a scratch repo): build + a one-off Go program is NOT needed — the test suite
# IS the integration validation. If desired, run the property/invariant tests (P1.M5) once they exist.

# Expected: All integration tests pass; HEAD advances correctly; CAS failures leave HEAD unmoved.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# (No MCP / Docker / Playwright / DB / security-scan validation for this internal package — it is pure
#  Go + git plumbing exercised by the real-git test suite in Level 2.)

# Concurrency reasoning (manual review, not automated): confirm generateMessage is safe to call overlapped
# with the NEXT stager (stager[i+1] ∥ message[i]). The proof: generateMessage reads HEAD (RevParseHEAD) +
# history (CommitCount/RecentMessages/RecentSubjects) + computes TreeDiff(treeA,treeB) over FROZEN trees —
# NONE of these read or depend on the live INDEX, which is the only thing the concurrent stager mutates.
# The frozen trees are immutable (write-tree). So message[i] is immune to concurrent staging. (The overlap
# itself is the orchestrator's concern — P3.M4.T1.S1 — but generateMessage must be SAFE under it. Verify by
# reading generateMessage: it touches NO index state.)

# CAS ordering reasoning (manual review): confirm publishCommit is the serialized publication step. Each
# call does CommitTree (dangling object — does NOT move HEAD) THEN UpdateRefCAS (the SOLE ref mutation, a
# CAS requiring HEAD == expectedOld). Two publishCommit calls CANNOT both succeed if they race on the same
# HEAD: the second's CAS fails because the first moved HEAD. The strict ordering is inherent in the CAS
# chain (each concept's expectedOld == the previous concept's newSHA). (The serialized invocation is the
# orchestrator's concern — P3.M4.T1.S1 — but publishCommit must ENFORCE one CAS at a time. It does.)

# Expected: manual reasoning confirms the overlap-safety + CAS-serialization invariants.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully
- [ ] All tests pass: `go test ./... -v`
- [ ] No linting errors: `golangci-lint run`
- [ ] No vet errors: `go vet ./...`
- [ ] No formatting issues: `gofmt -l internal/ pkg/` (empty)
- [ ] go.mod/go.sum UNCHANGED

### Feature Validation

- [ ] generateMessage computes the concept diff via TreeDiff(treeA, treeB) (tree-to-tree — NEVER StagedDiff)
- [ ] generateMessage uses deps.Roles.Message in RenderBare mode + derives (provider, model) via ResolveRoleModel
- [ ] generateMessage runs the CommitStaged step-5 loop (BuildUserPayload → Render → Execute → ParseOutput →
      ExtractSubject → IsDuplicate, bounded by MaxDuplicateRetries, FR29 retry-instruction, FR32 rejection)
- [ ] generateMessage returns *generate.RescueError on generation failure (ErrTimeout/ErrRescue) with
      TreeSHA=treeB + ParentSHA + Candidate; ErrMessageFailed-wrapped on infra failure
- [ ] generateMessage does NOT import/call the signal package (SIGNAL-FREE)
- [ ] publishCommit does CommitTree + UpdateRefCAS; root-aware (parentSHA=="" ⇒ nil parents + all-zeros)
- [ ] publishCommit returns *generate.CASError on CAS failure (HEAD unmoved; .Error() is the §13.5 message);
      ErrPublicationFailed-wrapped on CommitTree failure; non-CAS UpdateRefCAS propagates verbatim
- [ ] publishCommit takes parentSHA EXPLICITLY (the exact CAS expected-old; does NOT re-read HEAD for it)
- [ ] All success criteria from "What" section met (the test cases in findings §11 pass)
- [ ] Error cases handled gracefully: parse-rescue, timeout, empty-diff, CAS-failure

### Code Quality Validation

- [ ] Follows existing codebase patterns (callPlanner/stageConcept structure; generate.CommitStaged loop)
- [ ] File placement matches the desired codebase tree (message.go = the 4th decompose file)
- [ ] Anti-patterns avoided (no signal in a loop-iteration primitive; no orchestrator loop here; no
      re-reading HEAD for the CAS expected-old; no wrapping CASError/RescueError in sentinels)
- [ ] Dependencies properly managed (generate is a new one-way decompose import; no cycle)
- [ ] No new config keys; no CLI/route changes (internal package)

### Documentation & Deployment

- [ ] Code is self-documenting with clear doc comments (file doc + per-function docs citing PRD sections)
- [ ] ErrMessageFailed/ErrPublicationFailed sentinels documented (what they wrap vs what's returned raw)
- [ ] The two re-ported helpers are documented as verbatim re-ports (why re-ported, not exported from generate)

---

## Anti-Patterns to Avoid

- ❌ Don't implement the concept-iteration loop, the overlap goroutine scheduling, the FR-M8 empty-skip
  comparison, or the loop-level signal arming — those are the orchestrator (P3.M4.T1.S1 / P3.M4.T1.S2).
  This task is the PRIMITIVES generateMessage + publishCommit.
- ❌ Don't import or call the signal package in message.go (RestoreDefault is one-shot; loop signal is
  P3.M4.T1.S2). Return typed errors instead.
- ❌ Don't re-read HEAD to derive publishCommit's CAS expected-old — take parentSHA EXPLICITLY (the exact
  newSHA[i-1] the orchestrator holds). The CAS must race-detect a moved HEAD, not paper over it.
- ❌ Don't add a parentSHA param to generateMessage — it derives parentSHA + isUnborn internally via
  RevParseHEAD (safe under overlap; correct after concept i-1 published).
- ❌ Don't wrap *generate.RescueError or *generate.CASError in ErrMessageFailed/ErrPublicationFailed — they
  must stay errors.As-able.
- ❌ Don't edit generate.go to export buildSystemPrompt/recentSubjects — re-port them privately.
- ❌ Don't reuse StagedDiff (index-vs-HEAD) for the message diff — use TreeDiff(treeA, treeB) (§13.6.3
  invariant 2). The single-commit path's StagedDiff is explicitly NOT reused here.
- ❌ Don't skip the real-git integration tests — publishCommit's CAS + root-commit behavior can ONLY be
  verified against a real git repo (the stub is a git no-op).
- ❌ Don't use un-prefixed or stg*-prefixed fixture names in message_test.go — use msg* (collision with
  planner_test.go's un-prefixed + stager_test.go's stg* copies = compile error).
- ❌ Don't assert `errors.Is(err, context.DeadlineExceeded)` for generateMessage timeout unless
  RescueError.Unwrap chains the Cause — read generate.go's RescueError.Unwrap first (it returns e.Kind =
  generate.ErrTimeout, a DISTINCT sentinel; assert errors.Is(err, generate.ErrTimeout)).
