# Research — P1.M4.T1.S3: internal/prompt/payload.go — AssemblePayload

## Objective
`AssemblePayload(diff, instruction string, rejected []string) string` in the EXISTING
`internal/prompt` package (examples.go from S1 owns `// Package prompt`; system.go from S2 is a
sibling; THIS file is a third sibling with a FILE-level comment + plain `package prompt`). Pure
string builder — no git, no config, no error return. Assembles the user payload (the `payload`
argument the executor feeds to the agent's stdin / positional / flag in the generate inner loop,
P1.M6.T1.S1).

## ★ D5 — the byte-order decision (EXPLICIT FLAGGED) ★

- **reference_impl.md §3** (proven daily-driver `commit-pi`): `printf '%s\n\n%s' "$diff" "$user_prompt"`
  ⇒ stdin = **`<diff>\n\n<instruction>`** (diff FIRST, instruction LAST). System prompt travels via
  the `--system-prompt` flag separately.
- **PRD §17.3 prose** shows the base payload as `Generate a commit message…` THEN `<diff>` — the
  REVERSE (instruction-then-diff). **This prose is illustrative; it is OVER-RULED.**
- **reference_impl.md §4 discrepancy row D5** is the binding call: *"Follow reference ordering
  (`<diff>\n\n<instruction>`). Treat PRD §17.3's prose as illustrative. Flag this as an explicit
  decision so it is reviewed, not silently flipped."*
- **decisions.md §2/§3** codify it: `AssemblePayload(diff, instruction, rejected []string) string`
  with "ordering = diff-then-instruction per reference_impl.md §3".
- **Rationale (recency):** placing the imperative ("Generate a commit message…") CLOSEST to where
  the model begins generation leverages recency; the diff is context, the instruction is the trigger.

### The work-item parenthetical contradiction (resolved)
The task description says: "insert the §17.3 rejection block AFTER the instruction and BEFORE the
diff … yielding `<diff>\n\n<instruction>\n\n<rejection block>`. (Preserve diff-first; append
rejection block last so the model sees it immediately before generating.)"

The phrase "BEFORE the diff" is a copy-paste artifact of PRD §17.3's (over-ruled) prose. The
RESOLVED, REPEATED, UNAMBIGUOUS layout is:

- **Base (len(rejected)==0):** `<diff>\n\n<instruction>`
- **Retry (len(rejected)>0):** `<diff>\n\n<instruction>\n\n<rejection block>` — diff stays FIRST,
  the rejection block is APPENDED LAST (closest to generation start, maximum recency).

→ Implement the RESOLVED layout (diff-first, rejection-block-last). Ignore the "BEFORE the diff"
fragment.

## The §17.3 rejection block (verbatim bytes — confirmed with `cat -A` on PRD.md §17.3)

```
IMPORTANT: The following messages were REJECTED because they already exist
in git history. You MUST generate something COMPLETELY DIFFERENT:
- <rejected subject 1>
- <rejected subject 2>

Create an entirely new message with different wording.
```

Byte-exact structure:
- `rejectionHeader` = TWO lines:
  `"IMPORTANT: The following messages were REJECTED because they already exist\nin git history. You MUST generate something COMPLETELY DIFFERENT:"`
- ONE `\n` (NOT a blank line) then the bullets — `"- " + subject` per rejected subject, one per
  line, joined by `\n` (order = slice order).
- a BLANK line (`\n\n`) after the LAST bullet.
- `rejectionFooter` = `"Create an entirely new message with different wording."`

So the assembled rejection block =
`rejectionHeader + "\n" + strings.Join(bullets, "\n") + "\n\n" + rejectionFooter`
where `bullets[i] = "- " + rejected[i]`.

→ Commit these as named Go string constants VERBATIM from PRD §17.3 (same convention S2 used for
the §17.1/§17.2/Appendix A system-prompt strings in system.go).

## Function contract
`func AssemblePayload(diff, instruction string, rejected []string) string`
- `diff`: the staged-diff string from `git.StagedDiff` (P1.M3.T3.S1), PASSED IN — no internal/git
  import (prompt production code imports NO internal/git; the seam is scalar parameters).
- `instruction`: the imperative string, PASSED IN (caller M6 supplies "Generate a commit message
  for these changes:" per §17.3; payload.go does NOT hardcode it — it is a parameter, like `target`
  in BuildSystemPrompt).
- `rejected`: the list of already-rejected duplicate subjects from the outer dup loop
  (decisions.md §3: `rejected = append(rejected, subject)`). EMPTY ⇒ base layout (no block).
- Returns the assembled `payload` string. Pure, deterministic, no error.

## Output consumer
The returned `payload` is fed to the executor as `payload`/stdin in the generate inner loop:
`stdout, err = provider.Run(ctx, sys, payload)` (decisions.md §3; P1.M2.T4.S1 Executor.Run takes
`payload string`; P1.M6.T1.S1 CommitStaged calls `payload = prompt.AssemblePayload(diff,
instruction, rejected)`). Manifest.Render maps the payload to stdin/positional/flag delivery per §12.2.

## Conventions (from S1/S2 — MUST mirror exactly)
- **File header:** FILE-level comment `// This file adds the user-payload assembler …` + plain
  `package prompt`. Do NOT emit `// Package prompt` (examples.go OWNS it — same rule S2 followed for
  system.go, mirroring internal/git/log.go deferring to git.go).
- **Imports:** stdlib ONLY — `strings` (and `fmt` only if needed; fmt is NOT needed here — no
  `%d` interpolation). NO internal/git, NO internal/config, NO go-git, NO testify, NO new dep.
  Do NOT touch go.mod/go.sum; no `go mod tidy`.
- **Constants:** committed VERBATIM from PRD §17.3 (rejectionHeader, rejectionFooter, rejection
  bullet prefix `"-"`/`"- "`). No exported default constant is REQUIRED by the contract (unlike
  S1's DefaultExampleCount / S2's DefaultSubjectTargetChars — the instruction is a parameter, and
  §17.3 has no numeric default for the payload). Keep constants unexported.
- **Tabs, insert_final_newline** (per .editorconfig): gofmt enforces.
- **godoc** cites reference_impl.md §3 + D5, PRD §17.3, and the (diff-first / rejection-block-last)
  EXPLICIT decision.

## Test strategy (white-box `package prompt`, stdlib testing/strings ONLY)
AssemblePayload is a PURE function (no git, no IO) → NO real-git integration test is needed
(match system_test.go, which is pure). White-box `package prompt` so tests can reference the
unexported constants. Use `strings.Contains` (robust to exact join whitespace) for presence
checks; use exact-whole-string equality (`got == want`) for the "base layout" and "deterministic
bytes" cases.

Contract MOCKING scenarios (from the task):
1. **Base ordering diff-first:** `AssemblePayload("DIFF", "DO", nil)` ⇒ EXACTLY
   `"DIFF\n\nDO"` (the diff precedes the instruction; assert `strings.HasPrefix(got, "DIFF")`,
   the instruction comes AFTER, and `got == "DIFF\n\nDO"`).
2. **Rejection block present with each subject (non-empty rejected):**
   `AssemblePayload("D", "I", []string{"s1","s2"})` ⇒ contains the rejectionHeader, the
   rejectionFooter, `"- s1"`, `"- s2"`, and the block sits AFTER the instruction (`strings.Index`
   of instruction < index of rejectionHeader). Layout == `"D\n\nI\n\n" + block`.
3. **Empty rejected ⇒ no block:** `AssemblePayload("D","I",[]string{})` AND `AssemblePayload("D","I",nil)`
   ⇒ EXACTLY `"D\n\nI"`; assert it does NOT contain the rejectionHeader / rejectionFooter.
4. **Deterministic bytes:** same inputs ⇒ identical output (assert exact equality on a fixed
   input across the base and retry cases); one rejected subject ⇒ one bullet, the order of
   subjects preserved in bullet order.

## Validation gates (verified to PASS on the current codebase before this task)
- `go build ./internal/prompt/` — compiles (the new file joins the existing package).
- `go vet ./internal/prompt/` — clean (compiles _test.go too).
- `test -z "$(gofmt -l internal/prompt/)"` — no unformatted files (empty output = pass).
- `go test ./internal/prompt/` — PASS (S1 examples + S2 system tests STILL green + new payload tests).
- `go test ./...` — whole-module green; git/ui/provider unaffected.

## Scope boundaries (DO NOT cross)
- ONLY create `internal/prompt/payload.go` + `internal/prompt/payload_test.go`.
- examples.go, examples_test.go, system.go, system_test.go UNCHANGED.
- internal/git, internal/ui, internal/provider, cmd/stagehand/main.go, Makefile, go.mod, go.sum
  UNCHANGED. NO README/docs/providers TOML. NO go-git. NO `go mod tidy`.
- The `// Package prompt` doc appears EXACTLY ONCE (in examples.go); payload.go uses a FILE-level
  comment + plain `package prompt`.
