# Research Findings — P3.M2.T3.S1 (stager.go: tooled stageConcept + freezeSnapshot)

Authoritative empirical findings driving the PRP. Every section is load-bearing.

## §1. The verbatim contract + scope boundary

CONTRACT (P3.M2.T3.S1, from the work item):
1. RESEARCH NOTE: The stager (§13.6.2, FR-M5) is the ONLY tooled role. It receives a concept's
   title+description as a task (prompt/stager.go from P3.M1.T1.S2), runs with tools ON via RenderTooled
   mode (P1.M1.T2.S1). It stages changes via git add and hunk-staging. It MUST NOT commit/amend/push —
   stagehand owns all ref mutations. After stager[i] returns, the orchestrator FREEZES tree[i] =
   write-tree BEFORE stager[i+1] starts (§13.6.3 invariant 1). Stager failure: retry once, then treat
   as empty (FR-M8). The stager mutates the INDEX only (not refs).
2. INPUT: Deps with Stager manifest (tooled_flags non-empty) from P3.M2.T1.S1, prompt/stager.go from
   P3.M1.T1.S2, Render with RenderTooled mode from P1.M1.T2.S1.
3. LOGIC: Create internal/decompose/stager.go. Implement
   `func stageConcept(ctx, deps Deps, concept prompt.PlannerCommit) error`: build stager task prompt
   (BuildStagerTask), Render with RenderTooled mode, Execute. Non-zero exit: return error (caller
   retries once, then treats as empty). On success, the index holds the concept's changes. Implement
   `func freezeSnapshot(ctx, deps Deps) (treeSHA string, err error)`: calls deps.Git.WriteTree to freeze
   the current index into an immutable tree. This MUST be called synchronously after stageConcept
   returns and BEFORE the next stageConcept starts.
4. OUTPUT: stageConcept stages one concept's changes; freezeSnapshot returns the frozen tree SHA. The
   orchestrator (P3.M4.T1.S1) interleaves these with message generation.
5. DOCS: none — internal agent call + git operation.

SCOPE BOUNDARY (frozen / owned elsewhere — do NOT edit):
- internal/decompose/roles.go — SHIPPED by P3.M2.T1.S1. Defines Deps {Git, Registry, Config, Roles
  RoleManifests, Verbose}, RoleManifests{Planner, Stager, Message, Arbiter}, ResolveRoles. CONSUMED
  VERBATIM. Deps has NO Models field.
- internal/decompose/planner.go — SHIPPED by P3.M2.T2.S1 (RUNNING IN PARALLEL). Defines callPlanner +
  ErrPlannerFailed. CONSUMED (the orchestrator drives stager per planner concept). Do NOT edit; do NOT
  rely on planner_test.go's test helpers existing (parallel hazard — see §9).
- internal/prompt/stager.go — SHIPPED by P3.M1.T1.S2. CONSUMED: BuildStagerTask(title, description).
  The stager has NO system-prompt constant (task is the user payload; system prompt = "").
- internal/provider/{render,executor}.go — CONSUMED: Manifest.Render(..., mode...RenderTooled),
  provider.Execute(ctx, spec, timeout, vb) → (stdout, stderr, err), provider.RenderTooled, CmdSpec.
- internal/git/git.go — CONSUMED: WriteTree(ctx) → (sha, err). (TreeDiff/RevParseTree/ReadTree are the
  message/arbiter roles' concern — P3.M2.T4/P3.M3 — NOT this task.)
- internal/config/roles.go — CONSUMED: ResolveRoleModel("stager", cfg) → (provider, model).
- internal/decompose/{message,arbiter,chain,decompose}.go — DO NOT EXIST YET. This task creates ONLY
  stager.go (+ stager_test.go). Other tasks own the rest.
- cmd/, pkg/stagehand/ — UNCHANGED (the orchestrator P3.M4.T1.S1 wires stageConcept/freezeSnapshot;
  NOT this task).

## §2. The SHIPPED Deps shape (NO Models field)

internal/decompose/roles.go defines:
```go
type Deps struct {
    Git      git.Git
    Registry *provider.Registry
    Config   config.Config
    Roles    RoleManifests
    Verbose  *ui.Verbose
}
type RoleManifests struct {
    Planner provider.Manifest // bare
    Stager  provider.Manifest // tooled (TooledFlags non-empty after fallback)
    Message provider.Manifest // bare
    Arbiter provider.Manifest // bare
}
```
There is NO Models/RoleModels field on Deps. (RoleModels is ResolveRoles's 2nd return value; the
orchestrator retains it locally.) stageConcept takes ONLY Deps, so it CANNOT read a pre-resolved stager
(provider, model). THE RULE (identical to callPlanner's decision, P3.M2.T2.S1 §3): derive it via
`prov, mdl := config.ResolveRoleModel("stager", deps.Config)` — the SAME function ResolveRoles uses.
ResolveRoleModel signature (internal/config/roles.go:28): `func ResolveRoleModel(role string, cfg Config) (provider, model string)`.
Do NOT add a Models field to Deps (owned by the shipped roles.go — editing = conflict). Do NOT add a
model param to stageConcept (the contract signature is fixed). The derivation is correct for every
reachable case: ResolveRoles (P3.M2.T1.S1) guarantees deps.Roles.Stager.TooledFlags is non-empty (the
FR-D4 fallback switches to a TooledFlags-capable provider, AND switches the model to that provider's
stager default), and FR-R5b guards the dangerous bare-model-no-provider-on-pi case at ResolveRoles time
(BEFORE stageConcept runs). So ResolveRoleModel("stager", deps.Config) returns the correct (provider,
model) that Render needs.

## §3. stageConcept pipeline (5 steps; the tooled analogue of the v1 generate loop, MINUS the loop)

stageConcept is the TOOLED half of the decompose pipeline (PRD §13.6.2). It is structurally SIMPLER
than callPlanner: ONE Execute call, NO retry loop, NO parse, NO output contract (the stager returns
free-form text; the truth source is the index). The retry-once-then-empty (FR-M8/M12) is the CALLER's
job (the orchestrator, P3.M4.T1.S1) — the contract is explicit: "Non-zero exit: return error (caller
retries once, then treats as empty)."

Pipeline:
1. Derive the stager (provider, model): `prov, mdl := config.ResolveRoleModel("stager", deps.Config)`.
2. Build the stager task: `task := prompt.BuildStagerTask(concept.Title, concept.Description)`.
3. Render the stager manifest in TOOLED mode: `spec, err := deps.Roles.Stager.Render(mdl, prov, "", task, provider.RenderTooled)`.
   NOTE: system prompt is "" (empty) — prompt/stager.go has NO system-prompt constant (the task IS the
   user payload; §17.6). Render's (model, provider) arg order vs ResolveRoleModel's (provider, model)
   return order: pass (mdl, prov). `*spec` derefs the Render pointer.
4. Execute: `_, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)`.
   Execute logs the command + raw output via vb internally (VerboseCommand + VerboseRawOutput), so the
   stager's "list of paths staged" free-form text is observable in verbose mode WITHOUT stageConcept
   capturing/returning stdout. stageConcept discards stdout (the contract returns only error).
5. Result: execErr == nil → return nil (the index now holds the concept's changes — the agent mutated
   it via git add / git apply --cached). execErr != nil → return error wrapped in ErrStagerFailed.

## §4. The tooled-mode rendering (RenderTooled; empty TooledFlags errors; empty system prompt)

internal/provider/render.go:
```go
func (m Manifest) Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode) (*CmdSpec, error)
```
- The `mode ...RenderMode` param is VARIADIC and defaults to RenderBare when omitted. stageConcept MUST
  pass `provider.RenderTooled` explicitly (5th positional arg) — the stager is the ONLY tooled role
  (§13.6.2 / §11.5). Omitting mode would silently render BARE (tools off) — WRONG for a stager.
- RenderTooled appends TooledFlags (git tools on). RenderTooled on a manifest with nil/empty TooledFlags
  returns an error: "provider %q: tooled mode requires non-empty tooled_flags". ResolveRoles GUARANTEES
  deps.Roles.Stager.TooledFlags is non-empty (the FR-D4 fallback), so this error never fires in prod —
  but stageConcept MUST still wrap it in ErrStagerFailed (defensive; a misconfigured test Deps would
  hit it).
- Render calls Validate + Resolve internally (safe on the unresolved deps.Roles.Stager manifest from
  reg.Get — same as the planner/generate pattern).
- System prompt: pass "" as the 3rd arg. With SystemPromptFlag=="" the Render fallback would PREPEND
  sys to payload — but sys is "" so nothing is prepended (the delimiter logic is guarded by `sys != ""`).
  With a SystemPromptFlag set (e.g. claude has one), "" sys → no flag emitted (guarded by `sys != ""`).
  Either way, "" sys is a clean no-op. The stager's task is delivered as the user payload (§17.6).

## §5. Execute error contract + the NO-retry design (caller owns retry)

internal/provider/executor.go: `func Execute(ctx, spec, timeout, vb) (stdout, stderr, err)`:
- timeout (timeout > 0 derives context.WithTimeout) → err IS context.DeadlineExceeded.
- signal/parent cancel → err IS context.Canceled.
- non-zero exit → wrapped *exec.ExitError.
- start failure → wrapped LookPath/start error.
- success → err == nil.

CRITICAL DESIGN DECISION (vs callPlanner): stageConcept has NO retry loop. The contract is explicit —
"return error (caller retries once, then treats as empty)". So stageConcept returns an error for ANY
execErr (timeout, cancel, non-zero exit) and ANY render error, wrapped in ErrStagerFailed. The
orchestrator (P3.M4.T1.S1) does:
```go
err := stageConcept(ctx, deps, concept)
if err != nil {
    // FR-M8/M12: retry once
    err = stageConcept(ctx, deps, concept)
    if err != nil { /* treat as empty (tree[i] == tree[i-1]); log; continue */ }
}
tree, _ := freezeSnapshot(ctx, deps)  // freeze BEFORE stager[i+1]
```
So stageConcept's job is purely: tooled-Render → Execute → error-or-nil. It does NOT distinguish
timeout vs non-zero-exit vs cancel (all are "stager failed" → caller retries). It does NOT parse output
(there is no JSON contract; exit code is the signal). This is the simplest of the four role modules.

NOTE on wrapping context.Canceled: wrapping execErr (which may be context.Canceled) with `fmt.Errorf("%w: %v", ErrStagerFailed, execErr)` means the %w chain reaches BOTH ErrStagerFailed AND context.Canceled — the orchestrator can still `errors.Is(err, context.Canceled)` to abort-on-signal rather than retry. So wrapping is safe and non-lossy. (If the orchestrator wants to distinguish "user hit Ctrl-C" from "stager crashed", it can — the chain preserves both.)

## §6. ErrStagerFailed sentinel semantics

```go
var ErrStagerFailed = errors.New("decompose: stager failed")
```
Wrap ALL genuine stager failures (render error, exec-non-zero, timeout, cancel) with `%w` so
`errors.Is(err, ErrStagerFailed)` is true. The orchestrator detects stager failures via errors.Is to
apply the FR-M8/M12 retry-once-then-empty. This mirrors callPlanner's ErrPlannerFailed (P3.M2.T2.S1).

Non-rescue semantics: stageConcept mutates the INDEX only (the agent runs git add / git apply --cached).
It NEVER commits, amends, or moves refs (the prompt guardrails §17.6 + the structural tooled_flags
scope + the fact that stagehand owns all ref mutations). A stager failure leaves the index in whatever
state the agent left it; the orchestrator retries or treats-as-empty. freezeSnapshot is READ-ONLY w.r.t.
refs (WriteTree writes a tree object to the object store but touches NEITHER index NOR HEAD — §13.2).
Neither stageConcept's nor freezeSnapshot's errors involve a HEAD move or a ref — they are NOT
generate.RescueError scenarios (no snapshot-then-CAS here; the CAS is P3.M2.T4 / P3.M4's job).

## §7. freezeSnapshot = thin WriteTree wrapper; §13.6.3 invariant 1 (immutability)

freezeSnapshot is the §13.6.3 invariant-1 primitive: "tree[i] is frozen before stager[i+1] starts."
Implementation:
```go
func freezeSnapshot(ctx context.Context, deps Deps) (string, error) {
    treeSHA, err := deps.Git.WriteTree(ctx)
    if err != nil {
        return "", err
    }
    return treeSHA, nil
}
```
- WriteTree (internal/git/git.go) runs `git write-tree`, which serializes the CURRENT index into a tree
  object and returns its SHA. It is READ-ONLY w.r.t. refs/index (writes a tree object to the object
  store; does NOT modify the index or HEAD — §13.2). After this call, treeSHA is an IMMUTABLE record of
  "what was staged at time T" — whatever the NEXT stageConcept does to the live index afterward CANNOT
  reach treeSHA. That is the entire safety basis for stager[i+1] ∥ message[i].
- WriteTree FAILS (exit 128) when the index has unresolved merge conflicts; it returns a clear error
  ("unresolved merge conflicts in the index — resolve them first"). freezeSnapshot propagates it as-is
  (WriteTree's error is already descriptive; no sentinel needed — a freeze failure is infrastructural,
  not role-specific, and aborts the whole run).
- The VALUE of a named freezeSnapshot (vs the orchestrator calling WriteTree directly): it documents
  the §13.6.3 invariant-1 semantics ("freeze the snapshot before next stageConcept"), centralizes the
  freeze point, and gives the orchestrator a single named seam. The contract explicitly names it.
- Verbose logging: the verbose API (ui/verbose.go) has only VerboseCommand, VerboseRawOutput,
  VerboseRetry — no generic "log message". WriteTree in generate.go is silent. So freezeSnapshot is
  silent (no verbose output). The orchestrator (P3.M4.T1.S1) may log the tree SHA via the Output (↳/color)
  layer when it calls freezeSnapshot. Do NOT edit verbose.go (owned elsewhere).
- MUST be called synchronously after stageConcept returns and BEFORE the next stageConcept starts. That
  ordering is the ORCHESTRATOR's responsibility (P3.M4.T1.S1); freezeSnapshot itself is just the freeze.

## §8. The stub tooled-mode testing challenge (manifest needs TooledFlags; stub is a git no-op)

stubtest.Manifest (internal/stubtest/stubtest.go) builds a provider.Manifest with BareFlags=nil AND
TooledFlags=nil (it never sets TooledFlags). RenderTooled on a manifest with empty TooledFlags ERRORS
("tooled mode requires non-empty tooled_flags"). So stageConcept tests CANNOT use stubtest.Manifest
directly — they need a manifest WITH TooledFlags set.

SOLUTION: stager_test.go defines a LOCAL helper that wraps stubtest.Manifest and sets TooledFlags:
```go
func tooledStubManifest(t *testing.T, bin string, o stubtest.Options) provider.Manifest {
    m := stubtest.Manifest(bin, o)
    m.TooledFlags = []string{"--tooled-stub-flag"} // non-empty so RenderTooled succeeds; the stub ignores all args
    return m
}
```
TooledFlags is `[]string` (internal/provider/manifest.go:68). Any non-empty slice works — the stub agent
(cmd/stubagent) reads stdin + env, emits output, exits; it IGNORES its argv entirely (verified: main.go
calls io.Copy(io.Discard, os.Stdin) then emits STAGEHAND_STUB_OUT/script). So TooledFlags=["--anything"]
is a valid no-op for the stub while satisfying RenderTooled's non-empty requirement.

THE STUB IS A GIT NO-OP: the stub does NOT run git (it can't — it's a fixed binary that emits canned
output). So a "successful" stageConcept (stub exit 0) does NOT actually stage anything — the index is
unchanged. Consequences for tests:
- stageConcept SUCCESS tests assert ONLY the exit-code → error mapping (stub exit 0 → nil; exit 1 →
  ErrStagerFailed). They CANNOT assert "the index holds the concept's changes" via the stub (the stub
  can't run git add). The staging behavior is validated at the ORCHESTRATOR level with a REAL agent OR
  (better) by testing freezeSnapshot's immutability directly (§7) — the freeze is what makes the
  stager's index-mutation safe, and that IS testable with real git.
- freezeSnapshot tests ARE meaningful with real git: the test stages files via git add (in the test
  fixture), calls freezeSnapshot, stages MORE files, calls freezeSnapshot again, and asserts the FIRST
  tree is immutable (its content is frozen despite subsequent index changes). This is the load-bearing
  test for §13.6.3 invariant 1. See §10 for the exact test.

## §9. Test fixture name-collision hazard (parallel planner_test.go) — use DISTINCT prefixed names

planner.go (P3.M2.T2.S1) is RUNNING IN PARALLEL. Its PRP instructs planner_test.go to COPY fixture
helpers (initRepo, writeFile, stageFile, commitRaw, runGit, gitOut) from generate_test.go verbatim,
"owning" those names in package decompose. stager_test.go is ALSO package decompose. In Go, test files
in the SAME package share scope — so a DUPLICATE declaration of `initRepo` in both planner_test.go and
stager_test.go is a COMPILE ERROR ("initRepo redeclared in this block").

stager_test.go ALSO cannot RELY on planner_test.go's helpers existing: they run in parallel, so
stager_test.go may be written/compiled before planner_test.go exists. (They could see each other's
helpers once both compile — but that creates a hidden cross-file dependency that breaks if either is
missing.)

SOLUTION (robust regardless of planner_test.go's state): stager_test.go defines its OWN fixture
helpers with a DISTINCT prefix (e.g. `stgInitRepo`, `stgWriteFile`, `stgStageFile`, `stgCommitRaw`,
`stgRunGit`, `stgGitOut`) — copied from generate_test.go's helpers but renamed. This guarantees:
(a) no collision with planner_test.go's `initRepo` etc.; (b) stager_test.go is self-contained (compiles
whether or not planner_test.go exists). Copy the BODIES verbatim from internal/generate/generate_test.go
(those helpers are unimportable from package decompose — they live in package generate; generate_test
owns its own copies for the same reason). This is the lowest-friction, merge-safe choice.

(If a future task wants to DRY these, it can extract a shared internal/decompose/fixture_test.go — but
that is out of scope here and would itself collide with both files unless coordinated. Prefixed names
are the safe parallel-safe default.)

## §10. The fixture helpers needed + the freeze-immutability test

stager_test.go needs (copied from internal/generate/generate_test.go, renamed with `stg` prefix):
- stgInitRepo(t, repo) — git init + set user.name/user.email (so commit-tree works).
- stgWriteFile(t, repo, name, body) — write a working-tree file (UNstaged by default; staging is a
  separate explicit step).
- stgStageFile(t, repo, name) — git add one file (the stager's job, simulated by the test).
- stgCommitRaw(t, repo, msg) — git commit-tree a parent (for a non-unborn repo; needed so the index has
  a HEAD to diff against and so WriteTree produces a non-empty-tree base).
- stgRunGit(t, repo, args...) / stgGitOut(t, repo, args...) — for assertions (git ls-tree, git status).

The KEY test — TestFreezeSnapshot_Immutable (§13.6.3 invariant 1):
1. stgInitRepo; stgCommitRaw(t, repo, "initial") — a parent commit.
2. stgWriteFile(t, repo, "a.txt", "a\n"); stgStageFile(t, repo, "a.txt") — stage file A.
3. tree1, err := freezeSnapshot(ctx, deps) — freeze the index (A staged).
4. stgWriteFile(t, repo, "b.txt", "b\n"); stgStageFile(t, repo, "b.txt") — stage file B (index now A+B).
5. tree2, err := freezeSnapshot(ctx, deps) — freeze again (A+B staged).
6. Assert: err == nil both; tree1 != "" && tree2 != ""; tree1 != tree2 (B was added).
7. Assert IMMUTABILITY: stgGitOut(t, repo, "ls-tree", "--name-only", tree1) contains "a.txt" and NOT
   "b.txt"; stgGitOut(t, repo, "ls-tree", "--name-only", tree2) contains BOTH "a.txt" and "b.txt".
   This proves tree1 was FROZEN — staging B AFTER the first freeze did NOT alter tree1. That is invariant 1.

The stageConcept tests (using the tooled stub):
- TestStageConcept_Success: stub exit 0 → stageConcept returns nil. (Repo state irrelevant — stub is a
  git no-op; assert only the error mapping.)
- TestStageConcept_NonZeroExit: stub Options{Exit: 1} → stageConcept returns non-nil err;
  errors.Is(err, ErrStagerFailed) == true.
- TestStageConcept_Timeout: stub Options{SleepMS: 2000}; cfg.Timeout = 100ms → err non-nil;
  errors.Is(err, ErrStagerFailed) == true; errors.Is(err, context.DeadlineExceeded) == true (the %w
  chain reaches it).
- TestStageConcept_RenderTooledMode: build Deps with a manifest whose TooledFlags is EMPTY (the raw
  stubtest.Manifest, NOT tooledStubManifest) → stageConcept returns err; errors.Is(err, ErrStagerFailed)
  == true; err.Error() mentions "tooled". This proves stageConcept uses RenderTooled (not RenderBare) —
  a bare render of an empty-TooledFlags manifest would SUCCEED (BareFlags nil → no-op), so the error
  proves the tooled path. (Inverted: tooledStubManifest + exit 0 → nil, which is the happy path.)
- TestStageConcept_BuildStagerTask wiring: hard to assert directly (the stub ignores stdin). Document
  that BuildStagerTask is unit-tested in prompt/stager_test.go; stageConcept just calls it. Optionally,
  a stub that echoes stdin could capture the task — but the stub reads stdin to /dev/null. Skip; rely on
  prompt/stager_test.go coverage + a code-level rg check that BuildStagerTask(concept.Title, concept.Description) is called.

## §11. Config fields consumed (read-only)

stageConcept reads: deps.Config (for ResolveRoleModel's per-role lookup + the diff/timeout values are
not needed by stageConcept — it passes deps.Config.Timeout to Execute). freezeSnapshot reads nothing
from Config (WriteTree takes no options). Concretely:
- config.ResolveRoleModel("stager", deps.Config) — reads cfg.Roles["stager"] then falls back to
  cfg.Provider/cfg.Model for empty fields (the 5-layer precedence, FR-R3).
- deps.Config.Timeout — passed to provider.Execute (per-attempt timeout; Execute derives the context).
All read-only. No new config keys. config.Defaults() populates Timeout=120s etc.

## §12. No new deps, no import cycle, zero edits, additive file

- stager.go is the 3rd file in package decompose (roles.go 1st, planner.go 2nd). Same `package decompose`.
- Imports: "context"; "errors"; "fmt"; "github.com/dustin/stagehand/internal/config";
  "github.com/dustin/stagehand/internal/git"; "github.com/dustin/stagehand/internal/prompt";
  "github.com/dustin/stagehand/internal/provider". (config/git/prompt/provider already imported by
  roles.go; "context"/"errors"/"fmt" are stdlib. No new third-party. No import cycle.)
- ZERO edits to any shipped file (roles.go, planner.go, prompt/*, provider/*, git/*, config/*, cmd/*,
  pkg/stagehand all CONSUMED read-only). go.mod/go.sum UNCHANGED.
- stageConcept/freezeSnapshot are consumed later (P3.M4.T1.S1); NO caller wiring here → zero merge
  friction with the parallel planner.go.

## §13. Summary of the design (one paragraph)

stageConcept is the TOOLED, no-retry, no-parse counterpart of callPlanner: derive the stager model via
ResolveRoleModel("stager", deps.Config); build the §17.6 task from the concept's title+description;
Render the stager manifest in RenderTooled mode (system prompt empty); Execute once; return nil on
exit-0 or an ErrStagerFailed-wrapped error on any failure (render/exec/timeout/cancel). It mutates the
INDEX only (never refs). freezeSnapshot is a thin, documented WriteTree wrapper that freezes the current
index into an immutable tree SHA — the §13.6.3 invariant-1 primitive the orchestrator calls
synchronously after each stageConcept and before the next. Neither function retries or commits; both are
non-rescue (no snapshot-then-CAS; refs move only at P3.M2.T4's UpdateRefCAS). The stub can't run git, so
stageConcept tests assert the exit-code → error mapping (via a local tooledStubManifest helper that sets
TooledFlags), and freezeSnapshot's immutability is tested with real git + ls-tree assertions. Test
fixtures use DISTINCT prefixed names (stg*) to avoid colliding with the parallel planner_test.go.
