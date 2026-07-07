# P3.M2.T4.S1 — Empirical Research Findings

**Work item**: Implement per-concept message generation (tree-to-tree diff) + serialized publication loop.
**Files read**: `internal/generate/generate.go`, `internal/generate/{dedupe,rescue}_test.go`, `internal/git/git.go`
(all), `internal/decompose/{roles,planner,stager,stager_test}.go`, `internal/decompose/planner_test.go`,
`internal/prompt/{system,payload,planner}.go`, `internal/provider/{render,executor,parse}.go`,
`internal/config/config.go`, `internal/signal/signal.go`, `internal/stubtest/stubtest.go`,
`internal/generate/generate_test.go`, the stager PRP (`plan/.../P3M2T3S1/PRP.md`).

---

## §1. The contract (verbatim, abridged)

Per the work item: implement the message half of the pipeline. For each concept *i*: (c) generate
message[i] from `TreeDiff(tree[i-1], tree[i])` using the message-role manifest (BARE mode, same
generate/dedupe loop as v1 CommitStaged but with the CONCEPT diff instead of StagedDiff); (f)
`CommitTree(tree[i], [newSHA[i-1]], msg[i]) → newSHA[i]`; (g) `UpdateRefCAS(HEAD, newSHA[i],
newSHA[i-1])` (CAS) — on CAS failure, abort with the §13.5 message. **"The message generation function
is a variant of generate.CommitStaged's loop that takes a diff string instead of calling StagedDiff."**

## §2. SCOPE — this task is the PRIMITIVES, NOT the orchestrator loop

**Critical disambiguation.** The contract prose ("implement the core loop") describes the FULL
orchestrator, but the plan splits it:
- **P3.M2.T4.S1 (THIS TASK)** = the message-generation primitive + the publication primitive.
- **P3.M4.T1.S1** = the orchestrator that interleaves stageConcept/freezeSnapshot/generateMessage/
  publishCommit into the per-concept loop + the safety cap + planner-failure handling.
- **P3.M4.T1.S2** = per-concept failure isolation (FR-M12) + the multi-commit rescue variant +
  loop-level signal handling.

So "the core loop" in the contract = the **generate→parse→dedupe RETRY loop inside `generateMessage`**
(a variant of CommitStaged's step-5 loop) — NOT the concept-iteration loop. The serialized publication
is the `publishCommit` primitive (CommitTree + UpdateRefCAS); the SERIALIZATION ACROSS concepts (strict
CAS ordering) is the orchestrator's concern (P3.M4.T1.S1). This mirrors the established pattern:
P3.M2.T3.S1 shipped `stageConcept`+`freezeSnapshot` primitives (NOT the loop); this task ships
`generateMessage`+`publishCommit` primitives. **Do NOT implement the concept-iteration loop, the
overlap goroutine scheduling, or the signal arming** (those are P3.M4).

## §3. Deliverables — ONE file `internal/decompose/message.go` (package decompose)

The stager PRP's scope-boundary list named the expected files: `{message,arbiter,chain,decompose}.go`.
**`message.go` is this task's file.** It holds (mirroring stager.go's "2 related functions" pattern):
- `var ErrMessageFailed = errors.New("decompose: message generation failed")` — sentinel for
  generation-step INFRA failures (TreeDiff/RevParseHEAD/RecentSubjects/render/empty-diff).
- `func generateMessage(ctx context.Context, deps Deps, treeA, treeB string) (string, error)` — the
  generate/dedupe/parse loop over a tree-to-tree diff (§5).
- `var ErrPublicationFailed = errors.New("decompose: publication failed")` — sentinel for publication-
  step INFRA failures (CommitTree/non-CAS-UpdateRefCAS).
- `func publishCommit(ctx context.Context, deps Deps, tree, parentSHA, msg string) (string, error)` —
  CommitTree + UpdateRefCAS (§6).
- Two private helpers re-ported from generate.go (§7): `messageSystemPrompt`, `messageRecentSubjects`.

NO caller wiring (the orchestrator P3.M4.T1.S1 consumes these). NO edits to any shipped file. ONE test
file `message_test.go` (package decompose).

## §4. generateMessage signature + internal derivation

`func generateMessage(ctx context.Context, deps Deps, treeA, treeB string) (string, error)`

It derives **everything else internally**, keeping it self-contained and the orchestrator's call site
trivial (`msg, err := generateMessage(ctx, deps, tree[i-1], tree[i])`):
- `diff = deps.Git.TreeDiff(ctx, treeA, treeB, opts)` — opts from cfg (§5.1).
- `parentSHA, isUnborn, _ = deps.Git.RevParseHEAD(ctx)` — the CURRENT HEAD (== newSHA[i-1] after concept
  i-1 published; "" + isUnborn on the unborn base for concept 0). **Why derive rather than take a param:**
  (a) the rescue recovery command needs the parent that concept[i]'s commit will use == current HEAD;
  (b) the system-prompt builder needs the current born/unborn state; (c) both are `RevParseHEAD`, called
  once. **Why safe under overlap:** the concurrent stager[i+1] mutates the INDEX (git add), NOT HEAD —
  `RevParseHEAD` is immune to concurrent staging (verified: run() targets HEAD via `-C` + rev-parse, no
  index read). So `message[i] ∥ stager[i+1]` is safe at the RevParseHEAD level.
- `sysPrompt = messageSystemPrompt(...)` + `recent = messageRecentSubjects(...)` (§7).
- `prov, mdl := config.ResolveRoleModel("message", deps.Config)` — derive the message (provider, model)
  (Deps has no Models field; identical pattern to callPlanner/stageConcept).

## §5. generateMessage body — the CommitStaged step-5 loop, diff source swapped

Faithful port of `generate.CommitStaged` step 5 (read generate.go lines ~190-260). Differences from
CommitStaged: (1) diff source = `TreeDiff(treeA, treeB)` NOT `StagedDiff`; (2) uses `deps.Roles.Message`
+ `provider.RenderBare` (the message role is bare — §13.6.2); (3) NO WriteTree (the freeze is the
orchestrator's freezeSnapshot, already done); (4) NO CommitTree/UpdateRefCAS (those are publishCommit);
(5) NO signal.SetSnapshot/RestoreDefault/ClearSnapshot (§8 — signal is loop-scoped, owned by P3.M4).

```
1. diff = TreeDiff(treeA, treeB, {MaxDiffBytes, MaxMdLines, BinaryExtensions})  // §13.6.3 invariant 2
   if err -> return "", fmt.Errorf("%w: tree diff: %w", ErrMessageFailed, err)
   if diff == "" -> return "", fmt.Errorf("%w: empty concept diff %s..%s", ErrMessageFailed, treeA, treeB)
     (defensive — the orchestrator's FR-M8 check means treeA != treeB, so TreeDiff is non-empty)
2. parentSHA, isUnborn, err = RevParseHEAD(ctx)
   if err -> return "", fmt.Errorf("%w: rev-parse head: %w", ErrMessageFailed, err)
3. sysPrompt, err = messageSystemPrompt(ctx, deps.Git, deps.Config, isUnborn)   // §7
   recent, err = messageRecentSubjects(ctx, deps.Git, isUnborn)                  // §7 (FRESH each call)
4. prov, mdl = config.ResolveRoleModel("message", deps.Config)
   resolved = deps.Roles.Message.Resolve(); retryInstr = *resolved.RetryInstruction
5. LOOP attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++:
   payload = prompt.BuildUserPayload(diff, rejected)
   if parseFail { payload = retryInstr + "\n\n" + payload }                       // FR29 corrective
   spec, rerr = deps.Roles.Message.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)
     if rerr -> return "", fmt.Errorf("%w: render: %w", ErrMessageFailed, rerr)
   out, _, execErr = provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
     if execErr != nil:
       if errors.Is(execErr, context.DeadlineExceeded):
         return "", &generate.RescueError{Kind: generate.ErrTimeout, TreeSHA: treeB,
                                          ParentSHA: parentSHA, Candidate: candidate, Cause: execErr}
       if errors.Is(execErr, context.Canceled):
         return "", &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: treeB,
                                          ParentSHA: parentSHA, Candidate: candidate, Cause: execErr}
       lastCause = execErr  // non-zero exit (*exec.ExitError) — fall through to parse (stdout may be partial)
     else { lastCause = nil }
   m, ok, _ = provider.ParseOutput(out, deps.Roles.Message)
     if !ok { parseFail = true; candidate = m; deps.Verbose.VerboseRetry(attempt+1, "parse failed ..."); continue }
   parseFail = false
   subject = generate.ExtractSubject(m)
   if generate.IsDuplicate(subject, recent):
     rejected = append(rejected, subject); candidate = m; deps.Verbose.VerboseRetry(...); continue     // FR32
   msg = m; success = true; break
6. if !success:
   return "", &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: treeB,
                                    ParentSHA: parentSHA, Candidate: candidate, Cause: lastCause}
7. return msg, nil
```

**Field reuse from generate.go (all EXPORTED):** `generate.RescueError{Kind, TreeSHA, ParentSHA,
Candidate, Cause}` (struct, exported), `generate.ErrTimeout`, `generate.ErrRescue` (the Kind sentinels),
`generate.ExtractSubject(msg)`, `generate.IsDuplicate(subject, recent)`. **Why TreeSHA = treeB:** the
frozen concept tree that needs committing (the §18.3/FR-M12 rescue recovery command is
`git commit-tree -p <parentSHA> -m "…" treeB | xargs git update-ref HEAD`). **Why ParentSHA = RevParseHEAD:**
== newSHA[i-1], the commit parent (== the rescue command's -p).

## §6. publishCommit — the serialized publication primitive

`func publishCommit(ctx context.Context, deps Deps, tree, parentSHA, msg string) (string, error)`

**parentSHA is EXPLICIT (not derived)** — the CAS must use the EXACT expected-old = newSHA[i-1] that the
orchestrator holds. Re-reading HEAD would race (HEAD could move between the orchestrator's decision and
publishCommit's re-read). The orchestrator passes newSHA[i-1] verbatim.

```
parents = parentSHA != "" ? []string{parentSHA} : nil            // root commit (concept 0 on unborn)
newSHA, err = deps.Git.CommitTree(ctx, tree, parents, msg)
  if err -> return "", fmt.Errorf("%w: commit-tree: %w", ErrPublicationFailed, err)
expectedOld = parentSHA
if parentSHA == "" { expectedOld = strings.Repeat("0", 40) }     // all-zeros for root CAS (matches CommitStaged)
err = deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld)
  if err != nil:
    if errors.Is(err, git.ErrCASFailed):
      actual, _, _ = deps.Git.RevParseHEAD(ctx)                  // re-read for the §13.5 message (CommitStaged D5)
      return "", &generate.CASError{TreeSHA: tree, Expected: expectedOld, Actual: actual, Message: msg}
    return "", err                                                // non-CAS infra — propagate (wrap? see §6 note)
return newSHA, nil
```

**Reuse `generate.CASError`** (exported struct; its `.Error()` IS the §13.5 message: "HEAD moved from
<Expected> to <Actual> while generating; aborting ... git commit-tree ... | xargs git update-ref HEAD").
So publishCommit returns `*generate.CASError` on CAS — the CLI/orchestrator handles it exactly as
CommitStaged's CASError. **Do NOT wrap CASError in ErrPublicationFailed** (must stay `errors.As`-able).
Wrap the CommitTree failure in ErrPublicationFailed. For the non-CAS UpdateRefCAS failure: propagate
verbatim (it is git infra; matches CommitStaged's `return Result{}, err` for non-CAS).

## §7. The two prompt helpers — re-port generate.go's unexported buildSystemPrompt + recentSubjects

`generate.buildSystemPrompt` and `generate.recentSubjects` are **UNEXPORTED** (generate.go). message.go is
package `decompose` — it cannot call them. Re-port them as PRIVATE helpers (verbatim logic; ~10 lines
each). This keeps decompose self-contained without an export-and-couple change to generate (generate.go
is a shipped file; editing it to export helpers is out of scope and risks a conflict).

```go
// messageSystemPrompt — verbatim port of generate.buildSystemPrompt (PRD §9.3/§17.1/§17.2).
func messageSystemPrompt(ctx context.Context, g git.Git, cfg config.Config, isUnborn bool) (string, error) {
    if isUnborn { return prompt.BuildFallbackPrompt(cfg.SubjectTargetChars), nil }
    n, err := g.CommitCount(ctx); if err != nil { return "", err }
    if n <= 1 { return prompt.BuildFallbackPrompt(cfg.SubjectTargetChars), nil }
    msgs, err := g.RecentMessages(ctx, 20); if err != nil { return "", err }
    return prompt.BuildSystemPrompt(msgs, prompt.DetectMultiline(msgs), cfg.SubjectTargetChars), nil
}
// messageRecentSubjects — verbatim port of generate.recentSubjects (PRD §9.7 FR31).
func messageRecentSubjects(ctx context.Context, g git.Git, isUnborn bool) ([]string, error) {
    if isUnborn { return nil, nil }
    return g.RecentSubjects(ctx, 50)
}
```

**Why fetch recent FRESH each call (not once):** after concept[i-1] publishes, its subject is now in
history — message[i]'s dedupe MUST include it (prevents duplicate subjects ACROSS the run's concepts).
This is correct and matches CommitStaged (which fetches fresh). System prompt likewise reflects the
growing history (concept[0] on unborn → fallback; concept[1] → CommitCount=1 → fallback; concept[2] →
mature). This is the intended §13.6 behavior.

## §8. SIGNAL-FREE — generateMessage/publishCommit do NOT touch the signal package

`generate.CommitStaged` arms signal.SetSnapshot/SetCandidate/RestoreDefault/ClearSnapshot. **generateMessage
and publishCommit MUST NOT.** Reason: `signal.RestoreDefault` is a ONE-SHOT that PERMANENTLY stops signal
delivery for the process (it stops the goroutine + closes the channel). CommitStaged calls it once (one
commit, fine). In a decompose loop, calling RestoreDefault per concept in publishCommit would disable
rescue-mode signal handling for ALL subsequent concepts. Loop-level signal handling is the orchestrator's
job (P3.M4.T1.S2 "multi-commit rescue variant"). The primitives instead return typed errors
(`*generate.RescueError`, `*generate.CASError`) carrying ALL the context the orchestrator/CLI needs to
print the rescue/§13.5 message. This is the same return-typed-error-not-print pattern CommitStaged uses
toward the CLI. **FIRM SCOPE BOUNDARY: no signal import in message.go.**

## §9. Imports for message.go (NO import cycle)

```go
import (
    "context"; "errors"; "fmt"; "strings"
    "github.com/dustin/stagecoach/internal/config"   // ResolveRoleModel
    "github.com/dustin/stagecoach/internal/generate" // RescueError, CASError, ErrTimeout, ErrRescue, ExtractSubject, IsDuplicate
    "github.com/dustin/stagecoach/internal/git"      // StagedDiffOptions, ErrCASFailed
    "github.com/dustin/stagecoach/internal/prompt"   // BuildUserPayload, BuildSystemPrompt, BuildFallbackPrompt, DetectMultiline
    "github.com/dustin/stagecoach/internal/provider" // Execute, RenderBare, ParseOutput
)
```
**No import cycle:** generate imports {config, git, prompt, provider, signal, ui}; it does NOT import
decompose. decompose → generate is a clean one-way dependency. (ui is NOT needed — deps.Verbose is the
ui.Verbose handle, obtained via Deps; no direct ui symbol referenced in message.go.) Module path:
`github.com/dustin/stagecoach` (confirmed go.mod).

## §10. Test-fixture name collision — message_test.go MUST use the `msg*` prefix

Package `decompose` test files already declare (confirmed via grep):
- **planner_test.go** — UN-PREFIXED: `initRepo, writeFile, commitRaw, runGit, plannerDeps`.
- **stager_test.go** — `stg*` prefix: `stgInitRepo, stgWriteFile, stgStageFile, stgCommitRaw, stgRunGit,
  stgGitOut, tooledStubManifest, stagerDeps, stagerDepsWithConfig`.

**message_test.go is ALSO package decompose** → a duplicate `func initRepo` is a COMPILE ERROR. Use
DISTINCT `msg*`-prefixed names (copy bodies VERBATIM from generate_test.go, rename): `msgInitRepo,
msgWriteFile, msgStageFile, msgCommitRaw, msgRunGit, msgGitOut, msgHeadSHA`. Plus a `messageDeps`
helper (mirrors stagerDeps). The message role is BARE → use `stubtest.Manifest` DIRECTLY (no
tooledStubManifest needed; stubtest.Manifest's nil BareFlags is fine for RenderBare — append(nil) no-op,
confirmed: generate_test.go uses stubtest.Manifest for the bare CommitStaged). For dedupe-retry tests use
`stubtest.NewScript` (call-varying responses), mirroring TestCommitStaged_DedupeRetryThenSuccess.

## §11. Test cases (message_test.go)

**generateMessage:**
- `TestGenerateMessage_Success` — repo+initial commit; build two trees (stage file A → write-tree treeA;
  stage file B → write-tree treeB); stub returns "feat: add b"; assert returns "feat: add b".
- `TestGenerateMessage_DedupeRetryThenSuccess` — HEAD subject = "feat: existing"; NewScript
  ["feat: existing","feat: fresh"]; assert returns "feat: fresh" (FR32).
- `TestGenerateMessage_ParseFailRescue` — stub returns "" (or garbage) for all attempts; assert
  `errors.As(err, &re)` and `re.Kind == generate.ErrRescue` and `re.TreeSHA == treeB`.
- `TestGenerateMessage_Timeout` — cfg.Timeout=100ms, stub SleepMS=2000; assert `errors.As(err,&re)`,
  `re.Kind == generate.ErrTimeout`, `errors.Is(err, context.DeadlineExceeded)`.
- `TestGenerateMessage_EmptyDiff` — defensive: call generateMessage with treeA==treeB; assert
  `errors.Is(err, ErrMessageFailed)`.
- `TestGenerateMessage_UsesBareMode` — assert the stub manifest (nil TooledFlags) renders fine (bare);
  this is structurally guaranteed (RenderBare never errors on nil BareFlags). Optionally assert the
  emitted diff in the stub output matches TreeDiff (use a stub that echoes stdin).

**publishCommit:**
- `TestPublishCommit_Success` — repo+initial commit (HEAD=X); build a tree (stage file → write-tree);
  publishCommit(tree, X, "msg"); assert returns newSHA, `headSHA == newSHA`,
  `gitOut("log","--format=%B","-n1",newSHA) == "msg"`, HEAD's tree == the passed tree.
- `TestPublishCommit_RootCommit` — unborn repo (no commits); publishCommit(tree, "", "msg"); assert
  newSHA, HEAD == newSHA, the commit has NO parent (`gitOut("log","--format=%P","-n1")` == "").
- `TestPublishCommit_CASFailure` — repo+initial (HEAD=X); make a CONCURRENT commit
  (`msgCommitRaw(repo,"concurrent")` → HEAD=Z); publishCommit(tree, parentSHA=X, "msg"); assert
  `errors.As(err, &ce)`, `ce.Expected == X`, `ce.Actual == Z`, HEAD STILL == Z (unmoved — the dangling
  commit exists but HEAD is untouched), `ce.Error()` contains "HEAD moved".

## §12. Validation gates (verified from stager PRP + Makefile pattern)

```bash
go build ./... && go test ./...           # GREEN
go vet ./... && golangci-lint run         # clean
gofmt -l internal/ pkg/                   # empty
git status --short                        # 2 new files (message.go, message_test.go) ONLY
```
go.mod/go.sum UNCHANGED (all imports already used by roles.go/stager.go; generate is a new decompose
import but generate is already a package). No new external deps.

## §13. One-paragraph design summary

`generateMessage` is generate.CommitStaged's step-5 generate→parse→dedupe loop with the diff source
swapped from `StagedDiff` (index-vs-HEAD) to `TreeDiff(treeA, treeB)` (§13.6.3 invariant 2), reusing
CommitStaged's exported `RescueError`/`ErrTimeout`/`ErrRescue`/`ExtractSubject`/`IsDuplicate` and the
v1 `prompt.BuildUserPayload`/`BuildSystemPrompt`/`BuildFallbackPrompt`/`DetectMultiline`. It derives the
rescue's parent + the prompt's born-state from `RevParseHEAD` (safe under concurrent staging). It is
BARE (the message role) and signal-free. `publishCommit` is CommitStaged's step-7+8
(`CommitTree`+`UpdateRefCAS`) factored out, taking parentSHA explicitly (the exact CAS expected-old),
returning `*generate.CASError` (whose `.Error()` IS the §13.5 message) on CAS failure. Together they are
the message + publication primitives the orchestrator (P3.M4.T1.S1) interleaves into the serialized
per-concept loop; neither touches signal (loop-scoped signal is P3.M4.T1.S2).
