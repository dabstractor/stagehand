# Research Notes — P1.M1.T4.S1 (Multi-turn integration: render-contract + commit-lands)

## 1. What already exists (verified on disk) — DO NOT duplicate

`internal/generate/generate_test.go` **already has** a multi-turn happy-path test added by the
parallel/preceding item **P1.M1.T3.S3**: `TestCommitStaged_MultiTurnFallbackSuccess` (line ~868),
plus skip/duplicate siblings (`_MultiTurnSkipped_NonAppend`, `_SmallPayload`, `_DuplicateRescue`).
It also has a reusable helper **`appendScriptManifest(t, bin, responses)`** (line ~854) that wraps
`stubtest.NewScript` and sets `SessionMode="append"`. **T4.S1 MUST NOT re-test the happy-path commit
or the trigger truth table** — those are owned. T4.S1's UNIQUE contribution is the **render-contract
verification** the contract names: (c) every turn's argv has `--session-id` + stable value + no
`--no-session`, and (d) turn-1 has the system-prompt flag, later turns don't. T3.S3's tests do NONE
of that (they never set `BareFlags`/`SystemPromptFlag` and never inspect argv).

⇒ New test goes in a **NEW file** `internal/generate/generate_multiturn_test.go` (`package generate`,
white-box) so it shares `generate_test.go`'s helpers (`initRepo`, `writeFile`, `stageFile`,
`headSHA`, `commitRaw`, `gitOut`, `runGit`, `shaRe`, `sliceContains`, `appendScriptManifest`) —
same package = same test binary = free reuse, NO duplication.

## 2. THE key mechanism — verbose buffer captures EVERY turn's command (no stub change)

`provider.Execute` (executor.go:71) calls `vb.VerboseCommand(strings.Join(append([]string{spec.Command},
spec.Args...), " "))` → prints `"DEBUG: command: <cmd> <args...>\n"` on EVERY Execute. `multiturn.Run`
calls `provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)` once per turn (multiturn.go:160/171/182).
`CommitStaged` passes `deps.Verbose` straight into `Run` (generate.go gate). ⇒ wiring
`Deps{Verbose: ui.NewVerbose(&buf, true)}` captures **all N+1 multi-turn commands + the 1 one-shot
command** in `buf`. `ui.NewVerbose(w io.Writer, on bool) *Verbose` (verbose.go:33). This is the same
seam the sibling `TestCommitStaged_ResolvesSubProviderFromManifest` uses.

## 3. THE manifest MUST simulate pi's isolation flag (or the test is vacuous)

`stubtest.Manifest`/`NewScript` set only Command/PromptDelivery/Output/StripCodeFence/Env —
**`BareFlags` is empty**. With empty BareFlags, NO turn's argv contains `--no-session`, so "multi-turn
drops --no-session" is untestable. ⇒ the test MUST set `m.BareFlags = []string{"--no-session"}` and
`m.SystemPromptFlag = &"--system"` (simulate the pi manifest). Then:
- **one-shot** `Render` includes BareFlags ⇒ one-shot command has `--no-session`, no `--session-id`.
- **multi-turn** `RenderMultiTurn` (render.go:233) drops `--no-session`, adds `--session-id <id>`.

This `--session-id`-present / `--no-session`-absent split is the **clean discriminator** between the
one-shot command line and the N+1 multi-turn command lines in the buffer.

## 4. Assertion strategy (robust to exact N, no value-interference)

The flag substrings `--session-id`, `--no-session`, `--system` are unique CLI tokens; they do NOT
appear in the commit-message system-prompt value or the model name. The payload is delivered via
STDIN (PromptDelivery="stdin") so it is NOT in the command args (only its byte SIZE is logged, via
VerbosePayload). ⇒ substring counts on the verbose buffer are safe and exact:

- (b) N+1 multi-turn invocations: `strings.Count(buf, "--session-id") == N+1`. Cross-check: the
  shared `STAGECOACH_STUB_COUNTER` file == `1 (one-shot) + (N+1) (multi-turn) == N+2`.
- (c) every turn has --session-id: count == N+1 (above). stable value: `regexp.FindAllString(buf,
  -1)` for `stagecoach-[0-9a-f]{32}` ⇒ all N+1 must be IDENTICAL (one id minted per Run, FR-T6).
  --no-session dropped from multi-turn: `strings.Count(buf, "--no-session") == 1` (only the one-shot
  turn). PLUS a byte-exact cross-check on the FINAL turn via ArgsFile (see §5).
- (d) turn-1-only sys prompt: `strings.Count(buf, "--system") == 1` (only turn 1 emits it; turns
  2..N+1 pass turnSys="" ⇒ flag omitted, render.go:226).

N itself is NOT predicted — it's DERIVED from the run (N+1 = the --session-id count). Assert N+1 ≥ 3
(N≥2) so the N=1 failure mode (final turn would emit "ok" → commit "ok") fails loudly instead of
silently. The script + clamp-to-last (§6) makes the test correct for ANY N≥2.

## 5. ArgsFile = byte-exact cross-check of the FINAL turn (turn N+1)

The stub's `STAGECOACH_STUB_ARGSFILE` writes `strings.Join(os.Args, "\x00")` and **OVERWRITES** each
call (stubagent main.go:35) ⇒ after the run it holds only the LAST invocation (turn N+1). That's the
`TestCommitStaged_MessageRoleOverride` sliceContains pattern (generate_test.go:541). For turn N+1
(>1): `sliceContains(args, "--session-id")` true; `sliceContains(args, "--no-session")` false;
`sliceContains(args, "--system")` false. Set via `m.Env["STAGECOACH_STUB_ARGSFILE"] = argsFile`
(`m.Env` is a mutable map from `optsEnvMap`; NewScript leaves it set). This complements (does not
replace) the verbose-buffer every-turn coverage — ArgsFile proves byte-exact argv for one turn, the
verbose counts prove the property across ALL turns. (Per-turn files for ALL turns would need a stub
change — DELIBERATELY avoided: the verbose buffer already covers every turn with zero stub change.)

## 6. Script + clamp (robust to any N≥2)

Script = `["", "ok", "ok", "feat: add big thing"]`. Trace (counter starts absent):
- call 0 (one-shot, MaxDuplicateRetries=0 ⇒ 1 attempt): line 0 `""` → ParseOutput ok=false ⇒ loop
  exhausts ⇒ FR-T1 gate fires (conds a–d all hold).
- turn 1 (call 1): line 1 `"ok"`. turn 2 (call 2): line 2 `"ok"`. turn 3..N: clamp → line 3
  `"feat: add big thing"` (intermediate turns' stdout is DISCARDED by Run — only turn N+1 is parsed).
- turn N+1 (final): clamp → line 3 `"feat: add big thing"` ⇒ parsed ⇒ dedupe ⇒ committed.

Works for any N≥2 (clamp guarantees the final turn emits the message). cfg.MultiTurnChunkTokens=50
(tiny, per contract) + a ~1–2 KB staged file ⇒ payload ≫ 50 tokens ⇒ N≥2 (asserted). Keep
MaxDuplicateRetries=0 so one-shot = exactly 1 call (clean counter: N+2 total).

## 7. Assertions summary (the 4 contract points)

(a) commit lands: `shaRe.MatchString(res.CommitSHA)`; `headSHA==res.CommitSHA`; `git log --format=%B
-n1` round-trips `"feat: add big thing"`. (b) `Count("--session-id")==N+1` + counter==N+2. (c) stable
session id (regex all-equal) + `Count("--no-session")==1` + ArgsFile sliceContains(--session-id) and
NOT --no-session. (d) `Count("--system")==1` + ArgsFile NOT --system (turn N+1>1).

## 8. Scope / non-overlap

- INPUTS: P1.M1.T3.S3 (wired CommitStaged + the gate), P1.M1.T1.S4 (pi SessionMode="append" — but
  the test sets SessionMode on the STUB manifest directly; no real pi needed), P1.M1.T1.S3
  (RenderMultiTurn). All COMPLETE per plan_status.
- Touches ONLY the new file `internal/generate/generate_multiturn_test.go`. No production code, no
  stub change, no go.mod change, no docs (test-only per contract). Non-overlapping with P1.M1.T3.S4
  (which edits multiturn_test.go + docs/how-it-works.md) and P1.M1.T4.S2 (the failure-path siblings).
