# P3.M2.T2.S1 — Research Findings (decompose/planner.go)

Empirical findings from reading the shipped code (roles.go, prompt/planner.go, generate.go,
stagehand.go, git.go, render.go, executor.go, stubtest.go, config.go) + the P3.M2.T1.S1 PRP.

## §1. CONTRACT (verbatim from the work item)

Create `internal/decompose/planner.go`. Implement:
```go
func callPlanner(ctx context.Context, deps Deps, forcedCount int, isUnborn bool) (prompt.PlannerOutput, error)
```
- capture working-tree diff via `deps.Git.WorkingTreeDiff`;
- build system prompt (style examples = RecentMessages); build user payload (forced-count prefix if forcedCount>0);
- Render with BARE mode; Execute; ParsePlannerOutput with ONE retry on parse failure;
- single⇔message: if single==true, output carries Message (the single-shortcut, FR-M11);
- safety cap: if output.Count > deps.Config.MaxCommits AND forcedCount==0 → error
  `planner proposed N commits; exceeds max_commits (M); use --commits or --max-commits`;
- planner failure (§13.6.6): no commits made yet → surface error, exit NON-RESCUE.
OUTPUT: parsed PlannerOutput, consumed by the orchestrator (P3.M4.T1.S1). DOCS: none (internal).

## §2. Deps shape — from the NOW-SHIPPED roles.go (P3.M2.T1.S1, ALREADY IMPLEMENTED)

`internal/decompose/roles.go` EXISTS (created 07:53). Its Deps is FROZEN:
```go
type Deps struct {
    Git      git.Git
    Registry *provider.Registry
    Config   config.Config
    Roles    RoleManifests   // {Planner, Stager, Message, Arbiter provider.Manifest} — bare for planner
    Verbose  *ui.Verbose
}
```
**Deps has NO Models/RoleModels field.** RoleModels is ResolveRoles's 2nd return value; the
orchestrator retains it locally. callPlanner takes ONLY Deps → it CANNOT read a pre-resolved
planner (provider,model). See §3.

## §3. KEY DESIGN DECISION — planner model/provider derivation

Deps carries Config + Roles (manifests) but NOT the resolved per-role (provider,model) pairs.
callPlanner needs the planner model for `Render(model, provider, sys, payload)`. Options:
- (A) re-derive via `config.ResolveRoleModel("planner", deps.Config)`; (B) add a Models field to Deps
  (FORBIDDEN — Deps owned by the shipped parallel task; editing roles.go = conflict); (C) extra param
  (FORBIDDEN — contract signature is fixed, no model param).

**DECISION: (A) — `prov, mdl := config.ResolveRoleModel("planner", deps.Config)`.** It is the SAME
function ResolveRoles uses (reads cfg.Roles["planner"] → falls back to cfg.Provider/cfg.Model), so it
is CONSISTENT with roles resolution and honors per-role overrides (FR-R1/D3: planner=flagship). It is
CORRECT for every case that reaches callPlanner: FR-R5b fires at ResolveRoles time (BEFORE the
orchestrator calls callPlanner) for the dangerous bare-model-no-provider-on-pi case, so callPlanner
never runs misconfigured; config-init always writes `[defaults] provider`, so the normal case returns
a non-empty provider. (Edge: no provider anywhere + no per-role model → auto-detected at ResolveRoles;
re-derivation returns ("","") → Render falls back to manifest defaults — benign; documented.)
callPlanner is therefore fully self-contained given Deps (testable with just a Deps, like generate).

## §4. prompt/planner.go API (P3.M1.T1.S1 — SHIPPED; CONSUMED VERBATIM)

```go
// style examples = RecentMessages(ctx,20); NO DetectMultiline/SubjectTargetChars (§17.5 omits both).
BuildPlannerSystemPrompt(examples []string) string        // nil-safe (unborn → nil examples)

// forcedCount<=0 ⇒ normal instruction + "\n\n" + diff; >0 ⇒ prepends forced directive line.
BuildPlannerUserPayload(diff string, forcedCount int) string

// whole-string Unmarshal, then brace-balanced fallback (handles JSON in prose/fences).
// Returns non-nil error on parse failure (caller retries). Does NOT validate single⇔message.
ParsePlannerOutput(raw string) (PlannerOutput, error)

PlannerRetryInstruction = "Respond with ONLY the JSON object described, no other text."  // retry prepend

type PlannerOutput struct {
    Count   int             // == len(Commits); ==1 iff Single
    Single  bool            // true ⇒ single-shortcut (§13.6.4)
    Commits []PlannerCommit // 1..N; nil if "commits":null
    Message string          // present iff Single==true (zero "" otherwise)
}
type PlannerCommit struct { Title, Description string }
```
**callPlanner OWNS the single⇔message contract** (ParsePlannerOutput's doc says so explicitly).

## §5. Execution pattern — mirror generate.CommitStaged step 5 (the proven v1 loop)

```go
spec, err := deps.Roles.Planner.Render(mdl, prov, sysPrompt, payload, provider.RenderBare) // PLANNER IS BARE
out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)          // (stdout, stderr, err)
parsed, perr := prompt.ParsePlannerOutput(out)
```
- Render signature: `Render(model, provider, sysPrompt, userPayload string, mode ...RenderMode)`.
  ResolveRoleModel returns (provider, model) — pass them as (mdl, prov). Mode defaults to RenderBare;
  pass `provider.RenderBare` explicitly to document that the planner is the bare role.
- Execute returns `(stdout, stderr, err)`. execErr handling (mirror generate):
  - `errors.Is(execErr, context.DeadlineExceeded)` (timeout) → return ErrPlannerFailed-wrapped,
    NON-RESCUE (no snapshot during planning). NO retry (mirror generate: immediate).
  - `errors.Is(execErr, context.Canceled)` → propagate, NON-RESCUE.
  - non-zero exit (*exec.ExitError) → fall through to parse (stdout may be partial); record cause.
- Retry: maxAttempts=2 (1 initial + 1 retry). Trigger retry on ParsePlannerOutput error AND on the
  single⇔message validation error (§6). On retry, prepend `PlannerRetryInstruction + "\n\n"` to the
  payload, and call `deps.Verbose.VerboseRetry(attempt, reason)`. (Execute handles VerboseCommand /
  VerboseRawOutput internally.)

## §6. single⇔message + light semantic validation (the caller-owned contract)

`validatePlannerOutput(out) error`:
- `out.Count < 1` → error ("count < 1");
- `out.Single && out.Message == ""` → error ("single==true but message is empty") — THE LOAD-BEARING
  single⇔message check (the shortcut is unusable without a message);
- `!out.Single && len(out.Commits) == 0` → error ("single==false but no commits").
A validation error triggers the ONE retry (same budget as a parse failure — it is "the model did not
follow the output contract", correctable by the retry instruction). If it persists after the retry →
ErrPlannerFailed. (Lenient on Single==false + non-empty Message: harmless; orchestrator ignores it.)

## §7. Safety cap (FR-M4) — distinct, NON-retryable, exact message

AFTER a successful parse+validate (on the accepted output), BEFORE returning:
```go
if forcedCount == 0 && parsed.Count > deps.Config.MaxCommits {
    return PlannerOutput{}, fmt.Errorf(
        "planner proposed %d commits; exceeds max_commits (%d); use --commits or --max-commits",
        parsed.Count, deps.Config.MaxCommits)
}
```
- Auto-mode ONLY (forcedCount==0). Forced mode trusts the user's --commits (the orchestrator/CLI layer
  validated forcedCount against MaxCommits — out of callPlanner's scope).
- NOT wrapped in ErrPlannerFailed — it is a distinct, actionable remediation (the planner SUCCEEDED;
  its proposal just exceeds the cap). Returns immediately (no retry — a reasoning decision won't change
  on retry). The orchestrator surfaces it; it is also non-rescue.

## §8. ErrPlannerFailed sentinel (consistent with generate.ErrRescue/ErrTimeout style)

```go
var ErrPlannerFailed = errors.New("decompose: planner failed")
```
Wrap ALL genuine planner failures (parse-after-retry, single⇔message-after-retry, exec-non-zero-after-
retry, timeout, canceled, render error) with `%w`. The orchestrator (P3.M4.T1.S1) treats ANY callPlanner
error as NON-RESCUE — NO snapshot is taken during planning (§13.6.6: "no commits have been made yet;
surface the error and exit non-rescue"). The safety-cap error (§7) is separate but also non-rescue.

## §9. isUnborn + style examples + diff

- `isUnborn` is a param. Short-circuit `RecentMessages` on unborn (return nil examples) — mirrors
  generate.buildSystemPrompt / recentSubjects. `BuildPlannerSystemPrompt(nil)` is safe (planner prompt
  + a blank line, no "---" lines).
- `WorkingTreeDiff(ctx, git.StagedDiffOptions{MaxDiffBytes, MaxMdLines, BinaryExtensions})` — values
  from deps.Config (mirrors CommitStaged's StagedDiff call). **callPlanner does NOT gate on empty
  diff** — the orchestrator gates (FR-M1: decomposition activates iff nothing staged AND working tree
  has changes). Documented assumption.

## §10. Config fields (config.Defaults())

`MaxCommits=12` (FR-M4), `MaxDiffBytes=300000`, `MaxMdLines=100`, `BinaryExtensions=nil`,
`Timeout=120s`. ResolveRoleModel(role, cfg) → (provider, model). All present; all consumed read-only.

## §11. Test pattern (mirror generate_test.go)

- `stubtest.Build(t)` compiles cmd/stubagent ONCE (cached); `stubtest.Manifest(bin, Options{Out:<json>})`
  for single-response, `stubtest.NewScript(t, bin, []string{bad, good})` for call-varying (retry).
- Build Deps directly (NO ResolveRoles): `Deps{Git: git.New(repo), Config: config.Defaults(),
  Roles: RoleManifests{Planner: stubManifest}, Verbose: nil}`. The stub manifest passes Render's
  Validate+Resolve (generate_test proves this). ParsePlannerOutput ignores the stub's Output mode, so
  the stub just emits the JSON string on stdout (Options{Out: `<json>`} with default Output "raw").
- Real git repo via the generate_test fixture helpers (initRepo/writeFile/commitRaw/runGit) — copy them
  into planner_test.go (git's _test.go helpers are unimportable; generate_test owns its own copies).
  Create UNSTAGED working-tree files (write WITHOUT staging) so WorkingTreeDiff is non-empty; commit a
  few messages first for style examples (mature repo), or leave unborn to exercise the nil-examples path.
- Cases: happy multi-commit (Count=3, Commits populated); single-shortcut (Single=true, Message set);
  forced-count (forcedCount=3 → payload carries the directive — assert via a verbose capture or a stub
  that echoes stdin); parse-retry-then-success (NewScript [badJSON, goodJSON]); safety-cap error
  (Count=15, cfg.MaxCommits=12, forcedCount=0 → exact message); single-without-message → retry-then-
  success or retry-then-error; unparseable-after-retry → ErrPlannerFailed; timeout (SleepMS>Timeout →
  DeadlineExceeded → ErrPlannerFailed); unborn repo (nil examples path).

## §12. No new deps / no import cycle / zero merge friction

planner.go imports `context/errors/fmt` + `config/git/prompt/provider` — ALL already imported by
roles.go in the SAME package. go.mod/go.sum UNCHANGED. Package decompose is NEW; roles.go exists;
planner.go is the 2ND file (additive — different file, no edit to roles.go).

## §13. Scope boundary (frozen / owned elsewhere — DO NOT edit)

- roles.go (Deps/RoleManifests/RoleModels/ResolveRoles/computeInstalled/isMultiProvider/setRole) —
  CONSUMED (shipped by P3.M2.T1.S1). Do NOT touch.
- prompt/planner.go (+ system.go/payload.go for the RecentMessages precedent) — CONSUMED.
- provider/{render,executor}.go (Render/Execute/CmdSpec/RenderBare) — CONSUMED.
- git/git.go (WorkingTreeDiff/RecentMessages/StagedDiffOptions) — CONSUMED.
- config/{config,roles}.go (Config/ResolveRoleModel) — CONSUMED.
- stager.go/message.go/arbiter.go/chain.go/decompose.go — DO NOT EXIST YET (other tasks own them).
- cmd/, pkg/stagehand/ — UNCHANGED (the orchestrator P3.M4.T1.S1 wires callPlanner; NOT this task).
```
DELIVERABLES (2 new files, 0 edits to existing files):
  CREATE internal/decompose/planner.go — package decompose; callPlanner + validatePlannerOutput +
    ErrPlannerFailed + buildPlannerExamples (private RecentMessages short-circuit).
  CREATE internal/decompose/planner_test.go — stubtest + real-git integration tests (§11).
```
