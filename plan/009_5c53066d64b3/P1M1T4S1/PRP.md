---
name: "P1.M1.T4.S1 — Integration: N+1 turns, --session-id present, --no-session dropped, final parsed+deduped, commit lands"
description: |
  Add ONE integration test that proves the multi-turn generation fallback's end-to-end HAPPY PATH with a
  verified RENDER CONTRACT (PRD §9.24 FR-T4/T6). It drives `generate.CommitStaged` through a stub agent
  whose one-shot turn fails to parse (⇒ the FR-T1 gate fires), then delivers the captured payload across
  N+1 session turns, and asserts: (a) a single correct commit lands; (b) the stub was invoked N+1 times
  in the multi-turn phase; (c) EVERY multi-turn turn's rendered argv contains `--session-id <stable-id>`
  and does NOT contain `--no-session`; (d) turn 1's argv carries the system-prompt flag and turns 2..N+1
  do NOT. PRD §9.24 FR-T4/T6; §20.1 layer 3 (stub-provider integration).
  ⚠️ **NON-NEGOTIABLE: do NOT duplicate P1.M1.T3.S3.** `internal/generate/generate_test.go` ALREADY
  contains `TestCommitStaged_MultiTurnFallbackSuccess` (+ skip/duplicate siblings) and the reusable helper
  `appendScriptManifest(t, bin, responses)` — all added by T3.S3 (COMPLETE). Those tests assert the commit
  lands + the trigger truth table. They do NOT verify the per-turn render contract (c)/(d). T4.S1's UNIQUE
  contribution is the render-contract verification. ⇒ New test in a NEW file
  `internal/generate/generate_multiturn_test.go` (`package generate`, white-box) that REUSES
  generate_test.go's helpers (same package = same binary) and the `appendScriptManifest` helper. No second
  happy-path-only test; no re-test of the trigger gate.
  ⚠️ **THE mechanism — verbose buffer captures EVERY turn's command (NO stub change).** `provider.Execute`
  calls `vb.VerboseCommand(<cmd + args space-joined>)` on EVERY call (executor.go:71); `multiturn.Run` calls
  Execute once per turn (multiturn.go:160/171/182) passing `deps.Verbose`; CommitStaged threads
  `deps.Verbose` into Run. ⇒ wiring `Deps{Verbose: ui.NewVerbose(&buf, true)}` captures all N+1 multi-turn
  commands PLUS the 1 one-shot command. The flag substrings `--session-id`/`--no-session`/`--system` are
  unique CLI tokens (the payload goes via STDIN — only its byte SIZE is logged — so diff/system-prompt
  text never appears in the command line), making `strings.Count` assertions EXACT. This avoids modifying
  cmd/stubagent entirely (the stub's single ArgsFile OVERWRITES each call ⇒ holds only the final turn, so
  it cannot alone prove "every turn" — the verbose buffer does).
  ⚠️ **THE manifest MUST simulate pi's isolation flag or the test is vacuous.** `stubtest.NewScript` leaves
  BareFlags EMPTY. With empty BareFlags, NO turn contains `--no-session`, so "multi-turn drops
  --no-session" is untestable. ⇒ the test MUST set `m.BareFlags = []string{"--no-session"}` and
  `m.SystemPromptFlag = &"--system"` on top of `appendScriptManifest`. Then one-shot `Render` includes
  `--no-session` (BareFlags) and no `--session-id`; multi-turn `RenderMultiTurn` drops `--no-session` and
  adds `--session-id <id>` (render.go:233). This split is the clean discriminator between the one-shot
  command line and the N+1 multi-turn command lines.
  ⚠️ **N is NOT predicted — it is DERIVED.** chunkTokens=50 (tiny, per contract) + a ~1–2 KB staged file ⇒
  payload ≫ 50 tokens ⇒ N≥2. The script `["", "ok", "ok", "feat: add big thing"]` + the stub's
  clamp-to-last makes the test CORRECT for any N≥2 (intermediate turns' stdout is discarded by Run; only
  turn N+1 is parsed). Assert N+1 ≥ 3 (i.e. N≥2) so the N=1 failure mode (final turn would emit "ok") fails
  loudly. Read N+1 from the verbose `--session-id` count; cross-check the shared counter file == N+2
  (1 one-shot + N+1 multi-turn).
  Deliverable: ONE new file `internal/generate/generate_multiturn_test.go` with one test
  (`TestCommitStaged_MultiTurnRenderContract`) + small local helpers as needed. INPUT = wired CommitStaged
  (P1.M1.T3.S3), pi SessionMode="append" (P1.M1.T1.S4 — but the test sets SessionMode on the STUB manifest
  directly), RenderMultiTurn (P1.M1.T1.S3). Touches ONLY the new test file. Test-only; no production code,
  no stub change, no go.mod, no docs. Non-overlapping with P1.M1.T3.S4 (multiturn_test.go + how-it-works.md)
  and P1.M1.T4.S2 (failure-path siblings).
---

## Goal

**Feature Goal**: Prove, via a single end-to-end integration test against the stub agent, that the
multi-turn fallback's happy path satisfies the **render contract** of PRD §9.24 FR-T6 across ALL N+1
turns — the part NOT covered by P1.M1.T3.S3's commit-lands/trigger tests. Specifically: every
multi-turn turn renders with `--session-id` (stable across turns) and WITHOUT `--no-session`, turn 1
additionally carries the system-prompt flag and turns 2..N+1 do not, the stub is invoked exactly N+1
times in the multi-turn phase, and the final turn's parsed+deduped message becomes a single correct
commit.

**Deliverable**: ONE new file `internal/generate/generate_multiturn_test.go` (`package generate`,
white-box) containing `TestCommitStaged_MultiTurnRenderContract` (plus any small private helpers
local to that file). It reuses `generate_test.go`'s package-level helpers (`initRepo`, `writeFile`,
`stageFile`, `headSHA`, `commitRaw`, `gitOut`, `runGit`, `shaRe`, `sliceContains`,
`appendScriptManifest`) — no duplication.

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l internal/generate/` clean;
`go test -race ./internal/generate/ -run TestCommitStaged_MultiTurnRenderContract -v` PASSES; the full
suite `go test -race ./internal/generate/...` stays green (no regression to T3.S3's tests or any
other). The test asserts all four contract points (a)–(d) and the N≥2 guard. `git diff --stat` shows
ONLY the one new file.

## User Persona

**Target User**: The Stagehand contributor/maintainer (PRD §20 QA). Transitively US9.24 / G21 (the
multi-turn fallback) — this test is the regression net that proves a future refactor of
`RenderMultiTurn`/`Run` cannot silently break the per-turn argv contract (drop `--no-session`, add
`--session-id`, system-prompt turn-1-only) while still producing a commit.

**Use Case**: CI runs this test on every change to `internal/provider/render.go` (`RenderMultiTurn`),
`internal/generate/multiturn.go` (`Run`), or `internal/generate/generate.go` (the FR-T1 gate). A
regression in the render variant (e.g. forgetting to drop `--no-session`, or emitting the system
prompt on every turn) turns this test red.

**User Journey**: (internal test) build stub → init temp repo + seed commit → stage a LARGE file →
configure a SessionMode="append" stub manifest WITH `BareFlags=["--no-session"]` + a system-prompt
flag + a script that fails one-shot then returns "ok"/"ok"/…/"feat: …" → run `CommitStaged` with a
verbose sink → assert commit + render contract.

**Pain Points Addressed**: Closes the coverage gap left by T3.S3 (which proves the commit lands and
the gate fires, but never inspects the rendered argv). Without this test, a bug that drops
`--session-id` or keeps `--no-session` on multi-turn turns would ship silently (the stub ignores
flags, so T3.S3's commit still lands).

## Why

- **The render contract is the fragile part.** `RenderMultiTurn` makes three non-obvious deltas vs
  `Render` (drop `--no-session`, add `--session-id <id>`, emit the system-prompt flag on turn 1
  only — render.go:226/233). None of them affect the stub (it ignores flags), so a unit test of
  `RenderMultiTurn` alone (covered by P1.M1.T1.S5 golden tests) does not prove the ORCHESTRATOR
  (`Run` → `Execute`) actually drives the verified flag set end-to-end across N+1 real invocations.
  Only an integration test with per-turn argv observation closes that gap (PRD §20.5: "unit tests
  with stub agents cannot reach them").
- **Non-overlapping with T3.S3/S4.** T3.S3 owns the gate wiring + commit-lands; T3.S4 owns the
  chunk-math truth table + token_limit non-interaction; T4.S2 (sibling) owns the FAILURE paths
  (mid-turn error → rescue, small-payload skip, non-append skip). T4.S1 owns the HAPPY-PATH RENDER
  CONTRACT — a distinct, narrow, high-value assertion set.
- **Cheap, fast, deterministic.** Reuses the compiled-once stub (`stubtest.Build`), a real temp git
  repo, and tiny chunkTokens to bound N. No real provider, no network (PRD §20.1 layer 3).
- **No user-facing surface change** (PRD "DOCS: none — test-only").

## What

One white-box integration test that: (1) forces one-shot exhaustion (script line 0 = `""`), (2)
triggers the FR-T1 gate (all four conditions hold), (3) observes every multi-turn turn's rendered
argv via a verbose sink + the final turn's byte-exact argv via ArgsFile, and (4) asserts the commit
lands. No production code changes. No stub changes. No new dependencies.

### Success Criteria

- [ ] New file `internal/generate/generate_multiturn_test.go`, `package generate`, white-box.
- [ ] Exactly one test `TestCommitStaged_MultiTurnRenderContract` (+ private helpers if needed); reuses
      `generate_test.go` helpers (no copy of `initRepo`/`shaRe`/`sliceContains`/`appendScriptManifest`).
- [ ] (a) `res.CommitSHA` matches `shaRe` (`^[0-9a-f]{7,64}$`); `headSHA(t,repo)==res.CommitSHA`;
      `gitOut(t,repo,"log","--format=%B","-n1",res.CommitSHA)=="feat: add big thing"`;
      `res.Subject=="feat: add big thing"`.
- [ ] (b) `strings.Count(buf.String(), "--session-id") == N+1` (where N+1 is the multi-turn turn count)
      AND the stub counter file reads `N+2` (1 one-shot + N+1 multi-turn). Derive N+1 from the count.
- [ ] (c) all `stagehand-[0-9a-f]{32}` session ids found in the buffer are IDENTICAL (stable, FR-T6);
      `strings.Count(buf.String(), "--no-session") == 1` (only the one-shot turn); the ArgsFile
      (final turn) `sliceContains(args,"--session-id")` and NOT `sliceContains(args,"--no-session")`.
- [ ] (d) `strings.Count(buf.String(), "--system") == 1` (turn 1 only); ArgsFile (turn N+1>1) NOT
      `sliceContains(args,"--system")`.
- [ ] N+1 ≥ 3 asserted (N≥2 guard).
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l internal/generate/` clean; full generate suite green.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior knowledge can implement this from: the test skeleton below, the
assertion table, the helper-reuse note, and the four cited reference sites (executor.go:71,
multiturn.go:160/171/182, render.go:226/233, generate_test.go:541/868). The mechanism (verbose buffer
captures every turn's command) is fully explained; no guesswork.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- file: internal/generate/generate_test.go
  why: (1) REUSE these package-level helpers — do NOT redeclare: initRepo, writeFile, stageFile,
       headSHA, commitRaw, gitOut, runGit (lines 22-66), shaRe (line 68), sliceContains (line 528),
       appendScriptManifest (line 854). (2) TestCommitStaged_MultiTurnFallbackSuccess (line ~868) is
       the PRIOR happy-path test — read it to AVOID duplicating its assertions (commit-lands only).
       (3) TestCommitStaged_MessageRoleOverride (line 541) is the ArgsFile argv-capture template
       (read args, split on "\x00", sliceContains) — mirror it for the final-turn cross-check.
  pattern: stubtest.Build(t) → initRepo → commitRaw → writeFile → stageFile → stubtest.Manifest/
           appendScriptManifest → CommitStaged(ctx, Deps{...}, cfg) → assert Result + git log.
  gotcha: "same package = same test binary" — helpers are visible to generate_multiturn_test.go
          WITHOUT import; redeclaring any of them ⇒ compile error (redeclared in this package).

- file: internal/generate/multiturn.go
  why: confirms Run calls provider.Execute once per turn at lines 160 (turn 1), 171 (turns 2..N),
       182 (turn N+1) — each passing deps.Verbose ⇒ the verbose sink captures all N+1 multi-turn
       commands. Also confirms intermediate turns' stdout is DISCARDED (only turn N+1 is parsed) ⇒
       the script's clamp-to-last is safe for any N≥2.
  pattern: Run(ctx, deps, cfg, deps.Manifest, sysPrompt, payload, msgModel, msgReasoning).

- file: internal/provider/render.go
  section: "RenderMultiTurn" (line 185; body line 203)
  why: the three render deltas UNDER TEST — (1) capability gate *SessionMode=="append"; (2) turn-1-only
       system prompt: turnSys = sysPrompt iff turn==1 else "" (line ~226), gating BOTH the
       system_prompt_flag emission AND the prepend-fallback; (3) session-flags block (line ~233):
       BareFlags MINUS the exact "--no-session" token, PLUS "--session-id", sessionID (fresh slice).
  critical: this is what makes --no-session count==1 and --session-id count==N+1 in the buffer.

- file: internal/provider/executor.go
  section: "Execute" (line 71 — the VerboseCommand call)
  why: PROVES the verbose buffer captures every turn. `vb.VerboseCommand(strings.Join(append([]string{
       spec.Command}, spec.Args...), " "))` runs on EVERY Execute (success AND failure paths). The
       payload is NOT in args (PromptDelivery="stdin" ⇒ spec.Stdin), so only flag tokens + the system
       prompt VALUE appear. VerbosePayload logs the byte SIZE only (never contents).
  critical: a system-prompt VALUE with internal newlines makes turn-1's command span multiple physical
       lines — so DO NOT split the buffer by "\n" to find turns; use strings.Count / regexp on the
       whole buffer instead (the flag tokens themselves contain no newlines).

- file: internal/generate/generate.go
  section: "CommitStaged" — the FR-T1 trigger gate (search "FR-T1 multi-turn fallback trigger gate")
  why: confirms conditions a–d and that deps.Verbose is threaded into Run; confirms the one-shot loop
       runs `MaxDuplicateRetries+1` attempts (so MaxDuplicateRetries=0 ⇒ exactly 1 one-shot call ⇒
       counter math = N+2 total).
  pattern: the gate calls Run only when MultiTurnFallback && EstimateTokens(payload)>MultiTurnChunkTokens
           && resolved.SessionMode=="append" (after one-shot !success).

- file: internal/stubtest/stubtest.go
  why: NewScript(t, bin, responses) writes a script file + counter file in t.TempDir(); Manifest(bin,o)
       sets Env (a MUTABLE map — you can add STAGEHAND_STUB_ARGSFILE to it after the fact). Build(t)
       compiles the stub once (cached).
  gotcha: the stub's STAGEHAND_STUB_ARGSFILE OVERWRITES each call ⇒ holds only the LAST turn after the
          run (turn N+1). It CANNOT prove "every turn" alone — that's why the verbose buffer is primary.

- prd: PRD.md §9.24 (FR-T4 turn protocol, FR-T6 session lifecycle) + §20.1 layer 3 (stub integration)
  why: FR-T4 defines the N+1 protocol (turn 1 = sys prompt + preamble + chunk 1; turns 2..N = chunks;
       turn N+1 = final instruction, parsed by the existing pipeline). FR-T6 defines the render variant
       (drop --no-session, add --session-id, system prompt turn-1-only, never --continue).
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  generate.go                 # CommitStaged + the FR-T1 gate (calls multiturn.Run)
  multiturn.go                # Run (N+1 protocol) — calls provider.Execute per turn w/ deps.Verbose
  generate_test.go            # has helpers + appendScriptManifest + T3.S3 multi-turn tests (DO NOT dup)
  multiturn_test.go           # T3.S4 unit tests (chunk math, truth table) — DO NOT dup
internal/provider/render.go   # RenderMultiTurn (the render contract UNDER TEST)
internal/provider/executor.go # Execute → vb.VerboseCommand per call (the capture mechanism)
internal/stubtest/stubtest.go # Build, NewScript, Manifest, Options (ArgsFile/Script/Counter knobs)
cmd/stubagent/main.go         # the fake agent (env-driven; ArgsFile overwrites; script clamps to last)
```

### Desired Codebase tree with files to be added

```bash
internal/generate/
  generate_multiturn_test.go  # NEW — TestCommitStaged_MultiTurnRenderContract (+ private helpers if any)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: do NOT redeclare generate_test.go's helpers (initRepo, writeFile, stageFile, headSHA,
// commitRaw, gitOut, runGit, shaRe, sliceContains, appendScriptManifest). Same package ⇒ visible.
// Redeclaring ⇒ "redeclared in this package" compile error. IMPORT NOTHING from generate_test.go.

// CRITICAL: the manifest MUST set BareFlags=["--no-session"] + SystemPromptFlag="--system". A bare
// stubtest.NewScript/appendScriptManifest manifest has EMPTY BareFlags ⇒ no turn contains
// --no-session ⇒ "multi-turn drops --no-session" is untestable (count would be 0, not 1). Setting
// BareFlags simulates the pi manifest so one-shot Render includes it and RenderMultiTurn drops it.

// CRITICAL: the verbose buffer's turn-1 command contains the system-prompt VALUE (multi-line) joined
// by spaces ⇒ it spans multiple physical lines. DO NOT split buf by "\n" to enumerate turns. Use
// strings.Count(buf, "--session-id") / regexp.FindAllString for the WHOLE buffer instead. The flag
// tokens --session-id/--no-session/--system contain no newlines and do not appear in the payload
// (payload is STDIN ⇒ only its byte size is logged via VerbosePayload).

// CRITICAL: MaxDuplicateRetries=0 ⇒ the one-shot loop runs EXACTLY 1 attempt ⇒ counter math is
// clean: total stub calls = 1 (one-shot) + (N+1) (multi-turn) = N+2. Any other value muddies the
// "N+1 multi-turn invocations" assertion.

// CRITICAL: the stub's STAGEHAND_STUB_ARGSFILE OVERWRITES per call ⇒ after the run it holds ONLY
// turn N+1. Use it for the FINAL-TURN byte-exact cross-check only; the verbose buffer is the
// EVERY-TURN source of truth. (Per-turn files for all turns would require a stub change — avoided.)

// CRITICAL: rely on the script's clamp-to-last for correctness with any N≥2. Script
// ["","ok","ok","feat: add big thing"]: turn-1/2 priming emit "ok"; turn 3..N clamp to the message
// (intermediate stdout discarded by Run); turn N+1 clamps to the message ⇒ parsed ⇒ committed.
// DO NOT try to size the script to an exact N you cannot predict (the payload is built inside
// CommitStaged). Assert N+1≥3 (N≥2) so N=1 (final turn would emit "ok" ⇒ commit "ok") fails loudly.

// MINOR: set cfg.MultiTurnChunkTokens=50 (tiny) AND stage a ~1–2 KB file so EstimateTokens(payload)
// ≫ 50 (cond b holds) AND N≥2 (payload spans ≥2 chunks). chunkTokens=50 + a large file ⇒ N≈ several.
```

## Implementation Blueprint

### Data models and structure

No production data models. The test builds:
- A **stub manifest** = `appendScriptManifest(t, bin, script)` + `m.BareFlags=["--no-session"]` +
  `m.SystemPromptFlag=&"--system"` + `m.Env["STAGEHAND_STUB_ARGSFILE"]=argsFile`.
- A **config** = `config.Defaults()` with `MaxDuplicateRetries=0`, `MultiTurnChunkTokens=50`,
  `MultiTurnFallback=true` (explicit).
- A **verbose sink** = `var buf bytes.Buffer; vb := ui.NewVerbose(&buf, true)`.
- A **large staged file** to force `EstimateTokens(payload) > 50` and `N≥2`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/generate/generate_multiturn_test.go (package generate, white-box)
  - IMPORTS: bytes, context, os, path/filepath, regexp, strings, testing +
             internal/config, internal/git, internal/provider, internal/stubtest, internal/ui.
  - DO NOT redeclare generate_test.go helpers (initRepo/writeFile/stageFile/headSHA/commitRaw/gitOut/
    runGit/shaRe/sliceContains/appendScriptManifest) — same package, reuse directly.
  - TEST: TestCommitStaged_MultiTurnRenderContract (body per the Pattern block below).
  - PRIVATE HELPERS (only if needed): e.g. a local func to extract the multi-turn session id set via
    regexp; keep them unexported and file-local. Do NOT add a second happy-path test.
  - NAMING/PLACEMENT: snake_case file name generate_multiturn_test.go; one exported Test func.

Task 2: VERIFY (no file change)
  - RUN the Validation Loop (Level 1 + Level 2). Fix until green. `git diff --stat` ⇒ only the new file.
```

### Implementation Patterns & Key Details

```go
// generate_multiturn_test.go — the render-contract integration test (PRD §9.24 FR-T4/T6).
//
// UNIQUE vs T3.S3: T3.S3's TestCommitStaged_MultiTurnFallbackSuccess asserts the commit lands + the
// gate fires; it NEVER inspects the rendered argv and sets NO BareFlags/SystemPromptFlag. THIS test
// verifies the per-turn render contract (c)/(d) by observing every turn's command via the verbose
// sink (Execute→VerboseCommand per turn) + the final turn's byte-exact argv via ArgsFile.

var sessionIDRe = regexp.MustCompile(`stagehand-[0-9a-f]{32}`)

func TestCommitStaged_MultiTurnRenderContract(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")

	// LARGE staged file ⇒ EstimateTokens(payload) ≫ 50 (cond b) AND spans ≥2 chunks (N≥2).
	// ~60 lines × ~40 chars ≈ 2.4 KB ⇒ ~600 tokens ⇒ N ≈ several at chunkTokens=50.
	var b strings.Builder
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&b, "line %02d: padding content for a large diff\n", i)
	}
	writeFile(t, repo, "big.txt", b.String())
	stageFile(t, repo, "big.txt")

	// Script: line 0 "" (one-shot parse-fail ⇒ exhaust ⇒ gate fires); "ok","ok" priming; message LAST
	// (clamp-to-last ⇒ turn N+1 emits the message for any N≥2; intermediate turns' stdout discarded).
	script := []string{"", "ok", "ok", "feat: add big thing"}
	m := appendScriptManifest(t, bin, script) // SessionMode="append" (conds d + RenderMultiTurn gate)

	// SIMULATE pi's isolation flag + system-prompt flag so the render contract is OBSERVABLE:
	//   one-shot Render  ⇒ argv has --no-session (BareFlags), no --session-id.
	//   multi-turn Render ⇒ argv has --session-id, NO --no-session (dropped), --system on turn 1 only.
	m.BareFlags = []string{"--no-session"}
	spf := "--system"
	m.SystemPromptFlag = &spf

	// Byte-exact argv for the FINAL turn (turn N+1) — the stub's ArgsFile overwrites each call.
	argsFile := filepath.Join(t.TempDir(), "args")
	m.Env["STAGEHAND_STUB_ARGSFILE"] = argsFile // m.Env is a mutable map (optsEnvMap)

	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0     // ⇒ exactly 1 one-shot call ⇒ counter math = N+2 total
	cfg.MultiTurnChunkTokens = 50   // tiny ⇒ N≥2 (payload ≫ 50 tokens)
	cfg.MultiTurnFallback = true    // cond c (default true; explicit for clarity)

	// Verbose sink captures EVERY Execute command (one-shot + N+1 multi-turn).
	var vbuf bytes.Buffer
	vb := ui.NewVerbose(&vbuf, true)

	res, err := CommitStaged(context.Background(),
		Deps{Git: git.New(repo), Manifest: m, Verbose: vb}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v (expected multi-turn success)", err)
	}

	log := vbuf.String()

	// --- (a) commit lands ---
	if !shaRe.MatchString(res.CommitSHA) {
		t.Errorf("CommitSHA = %q, want hex SHA", res.CommitSHA)
	}
	if res.Subject != "feat: add big thing" {
		t.Errorf("Subject = %q, want %q (final-turn parsed message)", res.Subject, "feat: add big thing")
	}
	if got := headSHA(t, repo); got != res.CommitSHA {
		t.Errorf("HEAD = %q, want %q (commit must land)", got, res.CommitSHA)
	}
	if msg := gitOut(t, repo, "log", "--format=%B", "-n1", res.CommitSHA); msg != "feat: add big thing" {
		t.Errorf("git log message = %q, want %q", msg, "feat: add big thing")
	}

	// --- (b) N+1 multi-turn invocations (derive N+1 from the --session-id count) ---
	multiTurnCalls := strings.Count(log, "--session-id") // every multi-turn turn emits exactly one
	if multiTurnCalls < 3 {
		t.Fatalf("multi-turn turn count = %d, want ≥3 (N≥2); is the payload large enough / chunkTokens tiny?",
			multiTurnCalls)
	}
	N := multiTurnCalls - 1 // N chunks; multiTurnCalls == N+1
	// Counter cross-check: 1 one-shot + (N+1) multi-turn == N+2 total stub invocations.
	// (appendScriptManifest/NewScript created the counter file in its own t.TempDir(); read it via the
	//  same Env the stub saw — re-derive the path from m.Env["STAGEHAND_STUB_COUNTER"].)
	if cf := m.Env["STAGEHAND_STUB_COUNTER"]; cf != "" {
		if raw, rerr := os.ReadFile(cf); rerr == nil {
			got := strings.TrimSpace(string(raw))
			if got != fmt.Sprintf("%d", N+2) {
				t.Errorf("stub counter = %s, want %d (1 one-shot + %d multi-turn)", got, N+2, multiTurnCalls)
			}
		}
	}

	// --- (c) every multi-turn turn: --session-id present, stable id, --no-session ABSENT ---
	ids := sessionIDRe.FindAllString(log, -1)
	if len(ids) != multiTurnCalls {
		t.Errorf("session id occurrences = %d, want %d (one per multi-turn turn)", len(ids), multiTurnCalls)
	}
	for _, id := range ids {
		if id != ids[0] {
			t.Errorf("session id not stable: %q != %q (FR-T6 requires ONE id per run)", id, ids[0])
		}
	}
	if noSes := strings.Count(log, "--no-session"); noSes != 1 {
		t.Errorf("--no-session count = %d, want exactly 1 (one-shot only; multi-turn must drop it)", noSes)
	}
	// Final-turn byte-exact cross-check (ArgsFile holds turn N+1):
	rawArgs, _ := os.ReadFile(argsFile)
	args := strings.Split(string(rawArgs), "\x00")
	if !sliceContains(args, "--session-id") {
		t.Errorf("final-turn argv lacks --session-id; args=%v", args)
	}
	if sliceContains(args, "--no-session") {
		t.Errorf("final-turn argv must NOT contain --no-session; args=%v", args)
	}

	// --- (d) turn-1-only system prompt flag ---
	if sysCnt := strings.Count(log, "--system"); sysCnt != 1 {
		t.Errorf("--system count = %d, want 1 (turn 1 only; turns 2..N+1 omit it)", sysCnt)
	}
	if sliceContains(args, "--system") {
		t.Errorf("final-turn argv (turn N+1>1) must NOT contain --system; args=%v", args)
	}
}
```

> The skeleton above uses `fmt.Fprintf`/`fmt.Sprintf` — add `"fmt"` to the imports. If you prefer to
> avoid `fmt` in the file-builder, use `strconv` instead; either is fine (both already idiomatic in
> this repo). Keep the assertion logic byte-for-byte as shown — each line maps to a contract point.

### Integration Points

```yaml
TEST WIRING (the ONLY integration):
  - manifest: appendScriptManifest(t, bin, script) + m.BareFlags=["--no-session"] +
              m.SystemPromptFlag=&"--system" + m.Env["STAGEHAND_STUB_ARGSFILE"]=argsFile
  - config:   config.Defaults() then MaxDuplicateRetries=0, MultiTurnChunkTokens=50, MultiTurnFallback=true
  - deps:     Deps{Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&vbuf, true)}
  - drive:    CommitStaged(context.Background(), deps, cfg)

NO PRODUCTION CODE / NO STUB CHANGE / NO go.mod / NO DOCS (test-only, PRD "DOCS: none — test-only").
NO overlap with P1.M1.T3.S4 (multiturn_test.go + how-it-works.md) or P1.M1.T4.S2 (failure-path tests).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                              # Compiles incl. the new test file. Expect exit 0.
go vet ./internal/generate/                 # (and `go vet ./...`) Expect zero diagnostics.
gofmt -w internal/generate/generate_multiturn_test.go
test -z "$(gofmt -l internal/generate/)" && echo "gofmt clean" || echo "GOFMT DIRTY"
# Expected: all clean. If go vet complains about the fmt.Fprintf/unused import, trim imports.
```

### Level 2: Unit/Integration Tests (Component Validation)

```bash
# The new test (white-box, real git + stub agent):
go test -race ./internal/generate/ -run TestCommitStaged_MultiTurnRenderContract -v
# Expected: PASS with all four contract points (a)-(d) + the N≥2 guard satisfied.

# Full generate suite must stay green (T3.S3/S4 multi-turn tests + all prior):
go test -race ./internal/generate/...
# Expected: all PASS — no regression.

# Whole module (defensive — no cross-package fallout):
go test -race ./...
# Expected: all PASS.
```

### Level 3: Integration Testing (System Validation)

```bash
# Confirm the test actually EXERCISES multi-turn (not a silent one-shot success). A passing run must
# show N+1 ≥ 3 in the assertion; if it ever passes with multiTurnCalls<3 the t.Fatalf above catches it.
# Inspect the rendered argv by hand once (sanity for the render contract):
go test ./internal/generate/ -run TestCommitStaged_MultiTurnRenderContract -v -count=1
# Expected: PASS; the test internally verified --session-id count==N+1, --no-session count==1, --system count==1.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# (Optional) Lint the new file if golangci-lint is installed:
golangci-lint run ./internal/generate/ 2>/dev/null || echo "golangci-lint not installed (Makefile lint is project-wide; run \`make lint\` in CI)."
# Mutation check (manual, confirms the test BITES): temporarily edit internal/provider/render.go
# RenderMultiTurn to (1) NOT drop "--no-session" OR (2) emit the system_prompt_flag on every turn, then
# re-run — the test MUST go red at the --no-session==1 / --system==1 assertion. Revert after.
# Expected: the test fails on the mutated render, proving it guards the contract.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `go build ./...`, `go vet ./...`, `gofmt -l internal/generate/`.
- [ ] Level 2 green: `go test -race ./internal/generate/ -run TestCommitStaged_MultiTurnRenderContract -v` AND `go test -race ./internal/generate/...`.
- [ ] `git diff --stat` shows ONLY `internal/generate/generate_multiturn_test.go` (no production/stub/go.mod/docs changes).

### Feature Validation

- [ ] (a) commit lands: shaRe + headSHA==CommitSHA + git log round-trips "feat: add big thing".
- [ ] (b) `strings.Count(buf,"--session-id")==N+1`; counter file == N+2; N+1≥3 asserted.
- [ ] (c) all session ids identical; `strings.Count(buf,"--no-session")==1`; ArgsFile final turn has --session-id, NOT --no-session.
- [ ] (d) `strings.Count(buf,"--system")==1`; ArgsFile final turn NOT --system.
- [ ] The manifest sets BareFlags=["--no-session"] + SystemPromptFlag="--system" (else the test is vacuous).

### Code Quality Validation

- [ ] Reuses generate_test.go helpers (no redeclaration); `package generate` white-box.
- [ ] Does NOT duplicate T3.S3's happy-path test or T3.S4's truth table.
- [ ] No stub/production change; no new deps; no docs.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] Test doc-comment explains the UNIQUE value (render contract) vs T3.S3 and the verbose-buffer mechanism.
- [ ] No env vars / no user-facing docs (test-only).

---

## Anti-Patterns to Avoid

- ❌ Don't duplicate `TestCommitStaged_MultiTurnFallbackSuccess` (T3.S3) or the trigger truth table
  (T3.S4). T4.S1 verifies the RENDER CONTRACT only — a distinct assertion set.
- ❌ Don't redeclare `initRepo`/`shaRe`/`sliceContains`/`appendScriptManifest` — same package, reuse them.
- ❌ Don't omit `m.BareFlags=["--no-session"]` + `m.SystemPromptFlag="--system"`. Without them the
  stub manifest has empty BareFlags ⇒ `--no-session` appears in NO turn ⇒ the drop assertion is
  vacuous (count 0, not 1) and the turn-1 system-prompt check is meaningless.
- ❌ Don't split the verbose buffer by `"\n"` to enumerate turns — turn-1's command embeds the
  multi-line system-prompt VALUE. Use `strings.Count` / `regexp.FindAllString` on the whole buffer.
- ❌ Don't rely on the single ArgsFile to prove "every turn" — it overwrites and holds only turn N+1.
  The verbose buffer is the every-turn source of truth; ArgsFile is the final-turn byte-exact cross-check.
- ❌ Don't predict/hardcode an exact N — the payload is built inside CommitStaged. Derive N+1 from the
  `--session-id` count and rely on the script's clamp-to-last; assert N+1≥3 so N=1 fails loudly.
- ❌ Don't set MaxDuplicateRetries≠0 — it muddies the counter math (one-shot would make >1 call).
  MaxDuplicateRetries=0 ⇒ exactly 1 one-shot call ⇒ counter == N+2, clean.
- ❌ Don't modify `cmd/stubagent` or any production file to capture per-turn argv — the verbose buffer
  already captures every turn with zero stub change.
- ❌ Don't add a `//go:build integration_real` tag — this is a CI stub test (PRD §20.1 layer 3), not the
  opt-in real-agent suite.
