# Findings — P3.M3.T1.S1: Arbiter agent call + JSON parse (internal/decompose/arbiter.go)

Empirical findings from reading the shipped code + the sibling PRPs (planner P3.M2.T2.S1,
stager P3.M2.T3.S1, message P3.M2.T4.S1 [in-flight parallel]). All line refs verified against
the current tree.

## §1 The verbatim CONTRACT (work item)

1. RESEARCH NOTE: The arbiter (§13.6.5, FR-M9) is BARE and runs only if StatusPorcelain
   (P2.M2.T2.S1) is non-empty after the loop. It receives: SHAs, subjects, and file-lists
   (diff-tree) of every commit made this run, plus a diff of remaining changes (WorkingTreeDiff).
   Returns JSON: {"target": "<sha>"} or {"target": null}. Ambiguous → null. May only target a
   commit from this run. The arbiter only DECIDES; stagecoach performs all git (FR-M10). Output
   is parsed via ParseArbiterOutput (P3.M1.T1.S3).
2. INPUT: prompt/arbiter.go (P3.M1.T1.S1), StatusPorcelain (P2.M2.T2.S1), the list of commits
   made this run (SHAs, subjects, file-lists).
3. LOGIC: Create internal/decompose/arbiter.go. Implement
   `func runArbiter(ctx, deps Deps, commits []CommitInfo, leftoverDiff string) (prompt.ArbiterOutput, error)`:
   build system prompt (BuildArbiterSystemPrompt), build user payload (BuildArbiterUserPayload
   with commit list + diff), Render with bare mode, Execute, ParseArbiterOutput. If the returned
   target SHA is not in the commits-made list → treat as null (ambiguous). Define
   `type CommitInfo struct { SHA, Subject string; Files []git.FileChange }`.
4. OUTPUT: runArbiter returns a target SHA (or nil for new commit). Consumed by the resolution
   logic (S2 = P3.M3.T2.S1).
5. DOCS: none — internal agent call.

## §2 SCOPE — runArbiter is the INVOCATION only (S2 owns resolution)

- runArbiter is the BARE arbiter agent call + parse + in-list validation. It ONLY DECIDES.
- It does NOT perform resolution (new commit / tip amend / mid-chain chain rebuild). That is
  P3.M3.T2.S1 (the "S2" the contract names). runArbiter returns an ArbiterOutput; S2 acts on it.
- It does NOT construct []CommitInfo — the ORCHESTRATOR (P3.M4.T1.S1) builds it from each run
  commit's SHA + Subject + DiffTree(sha,isRoot) file-list.
- It does NOT call git.Git methods at all in the happy path: leftoverDiff is a PARAMETER
  (the orchestrator pre-computes it via WorkingTreeDiff), commits is a PARAMETER (the orchestrator
  pre-computes it via DiffTree), and the StatusPorcelain TRIGGER is the orchestrator's gate
  (FR-M9: orchestrator checks `StatusPorcelain(ctx) != ""` BEFORE calling runArbiter). runArbiter
  uses deps only for Roles.Arbiter (Render) + Config (ResolveRoleModel + Timeout) + Verbose.
- Documented assumption: runArbiter is called only when the orchestrator confirmed leftovers exist.

## §3 The prompt/arbiter.go API (SHIPPED — CONSUMED VERBATIM)

File: internal/prompt/arbiter.go (read in full). Symbols:

- `func BuildArbiterSystemPrompt() string` — ZERO-arg (§17.7 has NO <style examples> placeholder,
  unlike §17.5 planner). Returns the verbatim §17.7 system prompt.
- `func BuildArbiterUserPayload(commits []ArbiterCommit, leftoverDiff string) string` — assembles
  commitsHeader + per-commit blocks (SHA\nSubject\nfiles...) + leftoverHeader + leftoverDiff (verbatim).
- `func ParseArbiterOutput(raw string) (ArbiterOutput, error)` — whole-string json.Unmarshal, then
  brace-balanced fallback via `extractJSONObject` (defined in planner.go, same package — REUSED).
  Returns a non-nil error on parse failure. Does NOT validate target-in-list ("the caller owns that").
- `type ArbiterCommit struct { SHA string; Subject string; Files []string }` — NOTE: Files is []string
  ("the file-list (diff-tree --name-only)"), NOT []git.FileChange.
- `type ArbiterOutput struct { Target *string \`json:"target"\` }` — nil ⇔ null ⇔ new commit;
  &"<sha>" ⇔ amend that commit.
- CRITICAL: "§17.7 defines NO retry instruction — this layer does not export one." There is NO
  ArbiterRetryInstruction constant. This is a strong design signal: the arbiter is SINGLE-SHOT.

## §4 The CommitInfo → ArbiterCommit conversion (the ONE type seam)

- The contract defines `type CommitInfo struct { SHA, Subject string; Files []git.FileChange }`
  (Files = []git.FileChange because the orchestrator populates it from DiffTree's return type).
- prompt.ArbiterCommit.Files is []string (paths only). So runArbiter MUST convert each
  git.FileChange → its Path string. git.FileChange (internal/git/git.go:18) = `{Status, SrcPath,
  Path string}`; Path is ALWAYS set (SrcPath only for R/C renames). So: `files[j] = c.Files[j].Path`.
- The Status (A/M/D) is NOT included in the arbiter payload (ArbiterCommit.Files doc: "diff-tree
  --name-only"). Paths only.

## §5 The Deps shape (SHIPPED roles.go) — runArbiter's input

- `type Deps struct { Git git.Git; Registry *provider.Registry; Config config.Config; Roles
  RoleManifests; Verbose *ui.Verbose }` (roles.go). The arbiter manifest is `deps.Roles.Arbiter`
  (BARE per RoleManifests doc: "Arbiter provider.Manifest // bare").
- Deps has NO Models field (same as planner/stager/message). The arbiter (provider, model) is
  derived via `config.ResolveRoleModel("arbiter", deps.Config)` (see §8). Do NOT add a Models field.

## §6 The execution pattern (mirror callPlanner / stageConcept EXACTLY)

callPlanner (planner.go) is the closest sibling. runArbiter mirrors it with these deltas:
- NO retry loop (single attempt — §17.7 has no retry instruction; the arbiter is "when in doubt null").
- NO working-tree diff capture (leftoverDiff is a PARAMETER).
- Parse via prompt.ParseArbiterOutput (NOT provider.ParseOutput).
- Parse/timeout/cancel/non-zero-exit → graceful null (NOT an error — see §7).

Pipeline (runArbiter body):
  prov, mdl := config.ResolveRoleModel("arbiter", deps.Config)          // §8
  arbiterCommits := convert(commits); validSHAs := set of commit SHAs    // §4
  sysPrompt := prompt.BuildArbiterSystemPrompt()                         // §3 (zero-arg)
  payload := prompt.BuildArbiterUserPayload(arbiterCommits, leftoverDiff)
  spec, rerr := deps.Roles.Arbiter.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)
  if rerr != nil { return ArbiterOutput{}, fmt.Errorf("%w: render: %w", ErrArbiterFailed, rerr) }
  out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)
  if execErr != nil {
      if DeadlineExceeded || Canceled { return ArbiterOutput{nil}, nil }   // graceful → null
      // non-zero exit — fall through to parse (stdout may be partial)
  }
  parsed, perr := prompt.ParseArbiterOutput(out)
  if perr != nil { return ArbiterOutput{nil}, nil }                        // graceful → null
  if parsed.Target != nil && *parsed.Target != "" && validSHAs[*parsed.Target] {
      return parsed, nil                                                    // confident in-list target
  }
  return ArbiterOutput{nil}, nil                                            // empty/not-in-list → null (ambiguous)

Render arg order: Render(model, provider, sys, payload, mode). ResolveRoleModel returns
(provider, model) — pass (mdl, prov, ...). `*spec` derefs the Render pointer. mode = RenderBare.

Execute: provider.Execute(ctx, spec, timeout, vb) → (stdout, stderr, err). err is
context.DeadlineExceeded on timeout, context.Canceled on cancel, wrapped *exec.ExitError on
non-zero exit. deps.Verbose may be nil (nil-safe; Execute + ui.Verbose handle nil).

## §7 The ERROR CONTRACT (arbiter owns the null decision — the KEY design call)

The arbiter's failure mode is BENIGN: null → resolution makes a NEW commit for the leftovers;
NO work is lost (§13.6.5 "when in doubt, prefer a NEW commit (return null)"). This is UNIQUE among
the four roles (planner = non-rescue abort; stager = retry-then-empty; message = rescue with
frozen tree). Therefore runArbiter OWNS the null decision rather than punting to S2:

- Confident in-list target (parse OK + non-empty + in validSHAs): return (ArbiterOutput{&t}, nil).
- Parse failure / timeout / cancel / empty target / target-NOT-in-list: return (ArbiterOutput{nil},
  nil) — graceful degradation to null per §13.6.5. NOT an error.
- Render error (the ONE true infra failure — manifest misconfigured; near-impossible post-
  ResolveRoles): return (ArbiterOutput{}, fmt.Errorf("%w: render: %w", ErrArbiterFailed, err)).

Rationale: surfacing timeout/parse-fail as errors would force S2 to duplicate the "→ null" logic;
degrading internally keeps S2 dead simple (it reads out.Target; nil = new commit, &sha = amend)
and matches §13.6.5 "when in doubt, null" at EVERY layer. ErrArbiterFailed exists for consistency
with the sibling sentinels (ErrPlannerFailed/ErrStagerFailed/ErrMessageFailed) + verbose logging on
the render path. S2 should treat ANY runArbiter error as null too (defensive), but in practice
runArbiter only errors on render.

Contrast with planner.go: callPlanner returns ErrPlannerFailed on timeout/cancel/parse-fail because
the planner is load-bearing (non-rescue abort). The arbiter is NOT load-bearing — null is always
valid. This asymmetry is INTENTIONAL and correct.

## §8 ResolveRoleModel + Config (CONSUMED)

- `func config.ResolveRoleModel(role string, cfg config.Config) (provider, model string)`
  (internal/config/roles.go). call: `config.ResolveRoleModel("arbiter", deps.Config)`. Returns
  (provider, model); note RETURN ORDER vs Render's ARG order (§6). Reads cfg.Roles["arbiter"] then
  falls back to cfg.Provider/cfg.Model. The ("","") sentinel means "use manifest defaults" — fine
  for Render (Render Validate+Resolves). FR-R5b guards the dangerous bare-model-no-provider-on-pi
  case at ResolveRoles time (BEFORE runArbiter runs), so the derivation is correct for every
  reachable case.
- Config fields runArbiter reads: Timeout (Execute's per-attempt timeout; default 120s). That's it.
  (No diff caps — leftoverDiff is pre-computed. No MaxCommits — the safety cap is the planner's.)

## §9 git.FileChange + the SHA validation (in-list check)

- git.FileChange = `{Status, SrcPath, Path string}` (git.go:18). Path always set.
- "May only target a commit from this run" + "not in the commits-made list → treat as null
  (ambiguous)": build a `map[string]bool` (or struct{}) of commits[i].SHA (full 40-char SHAs —
  BuildArbiterUserPayload writes ArbiterCommit.SHA which the doc says is "the commit's full SHA
  (40/64 hex)"). The arbiter is INSTRUCTED to copy a SHA "from the list" verbatim (§17.7 prompt:
  `{"target": "<sha from the list>"}`), so it echoes a full SHA. Validate via EXACT membership:
  `validSHAs[*parsed.Target]`. A truncated / non-matching target → null (safe; the arbiter was told
  to copy verbatim and didn't). Prefix-matching is a possible robustness enhancement but the contract
  reads "in the list" → exact match is correct + deterministic.

## §10 No retry — confirmed by THREE signals

1. The work-item contract (§1 point 3) lists "Render with bare mode, Execute, ParseArbiterOutput"
   as sequential steps with NO retry — contrast the planner contract (P3.M2.T2.S1) which EXPLICITLY
   says "Retry once on unparseable JSON".
2. prompt/arbiter.go: "§17.7 defines NO retry instruction — this layer does not export one." There
   is no ArbiterRetryInstruction constant (grep confirmed: only PlannerRetryInstruction exists).
3. §13.6.5 "when in doubt, null" — the arbiter's core philosophy is graceful degradation, not
   correction. A parse failure IS a "doubt" → null.
=> Single attempt. On parse failure → null (not retry, not error).

## §11 Test fixtures (arb* prefix — collision avoidance)

- planner_test.go owns the UN-PREFIXED fixture names (initRepo, writeFile, stageFile, commitRaw,
  headSHA, runGit, gitOut). stager_test.go owns stg*-prefixed. message_test.go (P3.M2.T4.S1,
  in-flight) owns msg*-prefixed. ALL are package decompose → a duplicate declaration is a COMPILE
  ERROR. arbiter_test.go MUST use DISTINCT arb*-prefixed names (arbInitRepo, arbWriteFile,
  arbStageFile, arbCommitRaw, arbRunGit, arbGitOut, arbHeadSHA) — copied verbatim from
  internal/generate/generate_test.go (those helpers are unimportable from package decompose).
- stubtest: `Build(t)` compiles cmd/stubagent ONCE (cached); `Manifest(bin, Options{Out, Exit,
  SleepMS, Stderr, Script})` (single-response BARE manifest — nil BareFlags is fine for RenderBare);
  `NewScript(t, bin, []string{...})` (call-varying). The stub emits Options.Out on stdout; its Output
  mode is IRRELEVANT (runArbiter calls prompt.ParseArbiterOutput, NOT provider.ParseOutput).
- arbiter_test.go builds Deps{Git: git.New(repo), Config: config.Defaults(), Roles: RoleManifests{
  Arbiter: stubtest.Manifest(...)}, Verbose: nil} (NO ResolveRoles — direct stub manifest).

## §12 Test cases

1. Confident target: commits=[{shaA,...},{shaB,...}], stub emits `{"target": "<shaA>"}` →
   runArbiter returns ArbiterOutput{&shaA}, nil (Target non-nil, points at shaA).
2. Null target: stub emits `{"target": null}` → ArbiterOutput{nil}, nil.
3. Target-NOT-in-list (ambiguous→null): stub emits `{"target": "<bogus>"}` (not in commits) →
   ArbiterOutput{nil}, nil (the contract's load-bearing in-list check).
4. Empty target string: stub emits `{"target": ""}` → ArbiterOutput{nil}, nil.
5. Parse failure → null: stub emits "not json at all" → ArbiterOutput{nil}, nil (graceful; NOT error).
6. Timeout → null: stub SleepMS=2000, cfg.Timeout=100ms → ArbiterOutput{nil}, nil (graceful).
7. Non-zero exit but valid stdout: stub Exit=1, Out=`{"target":"<shaA>"}` → ArbiterOutput{&shaA}, nil
   (falls through to parse; partial-but-valid stdout accepted — mirrors planner/generate).
8. Render error → ErrArbiterFailed: pass a manifest whose Render fails (e.g. a manifest that fails
   Validate) → ArbiterOutput{}, err; errors.Is(err, ErrArbiterFailed) true.
9. Payload assembled correctly (assert the conversion): capture the stub's stdin and assert the
   payload contains each commit's SHA + Subject + each FileChange.Path (the []git.FileChange→[]string
   conversion), the headers, and the leftoverDiff tail. (Extends the stub to tee stdin, OR use a
   helper that calls BuildArbiterUserPayload directly and asserts — simpler.)
10. left-diff verbatim: assert the leftoverDiff string appears verbatim in the rendered payload.

## §13 Validation gates

- `go build ./... && go vet ./...` clean; `golangci-lint run` clean (.golangci.yml: errcheck/
  gosimple/govet/ineffassign/staticcheck/unused); `gofmt -l internal/ pkg/` empty.
- `go test ./internal/decompose/... -run Arbiter -v` green; full `go test ./...` green.
- go.mod/go.sum UNCHANGED (no new deps — config/git/prompt/provider all already imported by roles.go
  / planner.go). git IS imported (for git.FileChange in CommitInfo) — already a decompose import
  via roles.go. NO import cycle (decompose → git/config/prompt/provider is one-way).

## §14 One-paragraph summary

arbiter.go is the 5th file of internal/decompose (after roles/planner/stager/message). It exports
ErrArbiterFailed + type CommitInfo{SHA,Subject,Files []git.FileChange} + runArbiter(ctx, deps,
commits, leftoverDiff) (prompt.ArbiterOutput, error). runArbiter converts []CommitInfo →
[]prompt.ArbiterCommit (Files []git.FileChange → []string via .Path), builds the §17.7 prompt
(BuildArbiterSystemPrompt zero-arg + BuildArbiterUserPayload), Renders deps.Roles.Arbiter BARE
(model from ResolveRoleModel("arbiter")), Executes once, parses via ParseArbiterOutput, and returns
a confident in-list target or degrades to null (ArbiterOutput{nil}) on ANY indecision (parse fail,
timeout, cancel, empty/not-in-list target) — the arbiter OWNS the null decision per §13.6.5. Only a
render error returns a wrapped ErrArbiterFailed. NO retry (§17.7 has none). NO resolution (S2 owns
it). NO git reads (orchestrator pre-computes commits + leftoverDiff). Consumed by P3.M3.T2.S1.
