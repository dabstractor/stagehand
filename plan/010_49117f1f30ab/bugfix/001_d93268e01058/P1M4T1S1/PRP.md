---
name: "P1.M4.T1.S1 (Issue 5, test-only) — Replace strings.Contains with line-structure assertions in 3 decompose hook tests so they catch a regression of the Issue 2 trailing-newline fix"
description: |

  Test-quality fix (Issue 5, minor — no runtime bug). Three decompose hook tests assert the hook's
  `[HOOK-RAN]` append is present via `strings.Contains(msg, "[HOOK-RAN]")`, which PASSES even on the Issue 2
  corruption (`feat: add new[HOOK-RAN]` — the marker glued onto the subject line). That is why Issue 2
  shipped undetected through the decompose suite. Now that the Issue 2 fix (trailing newline before hooks,
  P1.M1.T2.S1) has LANDED, tighten these 3 assertions to a LINE-STRUCTURE check so a regression of the
  trailing-newline behavior fails the test. No code, no setup, no negative-test changes.

  CONTRACT (item_description §3): for each of the 3 tests, replace the `strings.Contains` assertion with a
  line-structure check — recommended form `strings.Contains("\n"+msg+"\n", "\n[HOOK-RAN]\n")`. This fails on
  `feat: add new[HOOK-RAN]` (no `\n` before the marker) but passes on `feat: add new\n[HOOK-RAN]`. Do NOT
  change the test setup, the hook installation, or the negative test (TestResolveArbiter_MidChain_SkipsHooks
  — asserts ABSENCE, correct as-is).

  THE 3 TESTS (exact current assertions):
    (a) internal/decompose/message_test.go:428 — TestPublishCommit_PrepareCommitMsgAnnotates:
        `if !strings.Contains(logMsg, "[HOOK-RAN]") { t.Errorf(...) }`
    (b) internal/decompose/chain_test.go:649 — TestResolveArbiter_NullNewCommit_RunsHooks:
        `if !strings.Contains(headMsg, "[HOOK-RAN]") { t.Errorf(...) }`
    (c) internal/decompose/chain_test.go:675 — TestResolveArbiter_TipAmend_RunsHooks:
        `if !strings.Contains(headMsg, "[HOOK-RAN]") { t.Errorf(...) }`

  DELIVERABLE (2 test files; nothing else): MODIFY the assertion CONDITION (and optionally tighten its error
  message) in those 3 tests. `strings.Contains(X, "[HOOK-RAN]")` → `strings.Contains("\n"+X+"\n",
  "\n[HOOK-RAN]\n")` where X is `logMsg` (a) or `headMsg` (b/c).

  SCOPE NOTE (the Issue 2 fix is LANDED, design §2): `internal/hooks/runner.go:107-110` — `if
    !strings.HasSuffix(finalMsg, "\n") { finalMsg += "\n" }` before `msgFile.WriteString(finalMsg)`. So the
    tightened assertions PASS against the current code; they FAIL only on a regression (the marker glued onto
    the subject). That is the guard this task adds.

  SCOPE NOTE (why pad BOTH sides, design §1): the helpers `msgRunGit`/`msgGitOut` (message_test.go:51-58)
    and `chnRunGit` (chain_test.go:52-58) `strings.TrimSpace` the git output, so the trailing `\n` git emits
    is STRIPPED — `logMsg`/`headMsg` end at `[HOOK-RAN]` with no trailing newline. The trailing `+"\n"` pad
    restores that boundary (the marker is the last line); the leading `"\n"+` makes the idiom correct even
    if the marker were first (defensive — it isn't, here). The checked break is BEFORE the marker.

  SCOPE BOUNDARY (do NOT touch): the negative test `TestResolveArbiter_MidChain_SkipsHooks` (chain_test.go
    ~698-710 — asserts ABSENCE of `[HOOK-RAN]`, correct as-is); the test setups; the hook installations
    (`msgInstallHook` / `chnBuildChainWithHook` — both use the identical `echo '[HOOK-RAN]' >> "$1"` hook);
    the helpers; any non-test file; go.mod/go.sum. This is a 3-assertion test-only change.

  INPUT (upstream — already built): the Issue 2 fix (P1.M1.T2.S1, runner.go:107-110). OUTPUT: 3 decompose
    hook tests assert line-structure parity; a regression of the Issue 2 trailing-newline fix fails them.

  ⚠️ Change ONLY the assertion condition (+ optionally its error message). Leave setup/hooks/negative-test.
  ⚠️ Pad BOTH sides: `strings.Contains("\n"+X+"\n", "\n[HOOK-RAN]\n")` — the helpers TrimSpace, so without
     the trailing `+"\n"` the check fails spuriously (no `\n` after the last-line marker).
  ⚠️ Leave `TestResolveArbiter_MidChain_SkipsHooks` (absence assertion) byte-identical.

  Deliverable: 2 modified test files; `go test ./internal/decompose/` green; no code/other-file changes.

---

## Goal

**Feature Goal**: Convert the 3 decompose hook tests' marker assertions from a loose substring check
(`strings.Contains`) to a line-structure check so they verify the git-parity outcome (the `[HOOK-RAN]`
append lands on its OWN line) and would fail on a regression of the Issue 2 trailing-newline fix. This
closes the test-quality gap (Issue 5) that let Issue 2 ship undetected.

**Deliverable** (2 test files modified; nothing else):
1. `internal/decompose/message_test.go` — `TestPublishCommit_PrepareCommitMsgAnnotates`: replace the
   `strings.Contains(logMsg, "[HOOK-RAN]")` assertion with `strings.Contains("\n"+logMsg+"\n",
   "\n[HOOK-RAN]\n")`.
2. `internal/decompose/chain_test.go` — `TestResolveArbiter_NullNewCommit_RunsHooks` +
   `TestResolveArbiter_TipAmend_RunsHooks`: same replacement (`headMsg` in place of `logMsg`).

**Success Definition**: the 3 tests still PASS against the current (fixed) code; each assertion now requires
the marker on its own line; `git revert`-ing the Issue 2 fix (runner.go:107-110) would make them FAIL (the
regression guard); the negative test + setups + hooks + helpers are byte-unchanged; `go test
./internal/decompose/` green; `git status` shows ONLY the 2 test files.

## User Persona

**Target User**: The maintainer guarding the hooks feature's git-parity (the test suite itself, transitively
every future contributor). Issue 2 (a corrupted subject line) shipped BECAUSE these tests passed on the
corrupt output. This task makes the tests assert the real invariant so it can't recur silently.

**Use Case**: A future change to `internal/hooks/runner.go` message-file writing regresses the trailing
newline → the next `go test ./internal/decompose/` FAILS one of these 3 tests (marker not on its own line)
→ the regression is caught at the gate, not in a user's commit history.

**User Journey**: (internal) contributor edits runner.go → `go test` → the tightened assertion reports
`"...want [HOOK-RAN] on its own line..."` → contributor fixes the newline before merging.

**Pain Points Addressed**: A passing test that doesn't test the real invariant (false confidence). The loose
`strings.Contains` made the suite green on corrupt output; the line-structure check makes it faithful.

## Why

- **It IS Issue 5.** The Bug-Fix PRD §h2.3/h3.4 names this exact gap: the decompose/arbiter hook tests
  assert only `strings.Contains`, masking the Issue 2 newline corruption. This task tightens them.
- **Guards the Issue 2 fix (P1.M1.T2.S1).** That fix landed (runner.go:107-110) but had no regression test
  that could catch its removal — these 3 tests become that guard.
- **Trivial, isolated, no-risk.** 3 assertion edits in 2 test files; no code, no setup, no behavior change.
  Cannot affect production; only makes the suite stricter.

## What

Three one-line assertion-condition replacements (plus optional error-message tightening) across two
white-box test files in `package decompose`. No new tests, no new helpers, no code, no other files.

### Success Criteria

- [ ] `message_test.go:428` — the condition is `strings.Contains("\n"+logMsg+"\n", "\n[HOOK-RAN]\n")` (was
      `strings.Contains(logMsg, "[HOOK-RAN]")`); the `t.Errorf` still names the variable; setup/hooks unchanged.
- [ ] `chain_test.go:649` (`_NullNewCommit_RunsHooks`) — the condition is
      `strings.Contains("\n"+headMsg+"\n", "\n[HOOK-RAN]\n")`; setup/hooks unchanged.
- [ ] `chain_test.go:675` (`_TipAmend_RunsHooks`) — the condition is
      `strings.Contains("\n"+headMsg+"\n", "\n[HOOK-RAN]\n")`; setup/hooks unchanged.
- [ ] `TestResolveArbiter_MidChain_SkipsHooks` (chain_test.go ~698-710) is byte-UNCHANGED (it asserts
      ABSENCE — correct).
- [ ] `go test ./internal/decompose/ -run 'TestPublishCommit_PrepareCommitMsgAnnotates|TestResolveArbiter_' -v`
      → the 3 tightened tests PASS; the MidChain negative test still PASSES; the rest of the suite green.
- [ ] `go build ./... && go test ./...` GREEN; `gofmt -l` clean; go.mod/go.sum byte-unchanged; `git status`
      shows ONLY the 2 test files; NO `.go` non-test file touched.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the 3 exact current assertions
+ their verbatim replacements (below), the §1 explanation of the double-sided pad (why the helpers' TrimSpace
requires it), and the LEAVE list (the negative test + setups + hooks). No hooks/runner/generate knowledge
required — this is 3 assertion edits.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/P1M4T1S1/research/design-decisions.md
  why: the 5 decisions. §0 (scope: 3 assertions / 2 test files; test-only), §1 (THE assertion form + why pad
       BOTH sides — the helpers TrimSpace so the trailing `+"\n"` is required; FAILS on corruption, PASSES on
       fixed), §2 (Issue 2 fix is LANDED at runner.go:107-110 → tightened assertions pass today, fail on
       regression), §3 (all 3 tests use the identical echo-append hook ⇒ uniform `<subject>\n[HOOK-RAN]`
       shape), §4 (LEAVE the negative test + setups + hooks), §5 (no conflict with parallel P1.M3.T1.S3).
  critical: §1 (the double-sided pad — omit the trailing `+"\n"` and the check fails spuriously because
       TrimSpace stripped the marker's trailing newline), §4 (do NOT touch the MidChain absence test).

# MUST READ — the file being edited (test 1 + the helper that bounds the assertion form)
- file: internal/decompose/message_test.go   (EDIT; one of two files)
  section: `TestPublishCommit_PrepareCommitMsgAnnotates` (L407-430) — the assertion at L428 (`if
           !strings.Contains(logMsg, "[HOOK-RAN]")`) + its L429 `t.Errorf`; the hook install at L419
           (`msgInstallHook(... "#!/bin/sh\necho '[HOOK-RAN]' >> \"$1\"\n")`). `msgRunGit`/`msgGitOut`
           (L51-63) return `strings.TrimSpace(...)` ⇒ `logMsg` is trimmed (the §1 pad rationale).
  why: the EXACT current text to replace + confirmation the helper trims (so the pad is needed).
  critical: replace ONLY the L428 condition; keep the L429 `t.Errorf` (optionally tighten its text). The
       hook install + setup are unchanged.

# MUST READ — the other file being edited (tests 2 & 3 + the negative test to leave alone)
- file: internal/decompose/chain_test.go   (EDIT; the other file)
  section: `TestResolveArbiter_NullNewCommit_RunsHooks` (L631-652, assertion L649) +
           `TestResolveArbiter_TipAmend_RunsHooks` (L654-677, assertion L675) — both
           `if !strings.Contains(headMsg, "[HOOK-RAN]")`. `chnRunGit` (L52-58) TrimSpace ⇒ `headMsg` trimmed.
           `chnBuildChainWithHook` (L624-627) installs the SAME `echo '[HOOK-RAN]' >> "$1"` hook for both.
           `TestResolveArbiter_MidChain_SkipsHooks` (~L698-710) — the NEGATIVE test (asserts ABSENCE): LEAVE.
  why: the EXACT current text for tests 2 & 3 + the negative-test anchor (do NOT touch it).
  critical: replace ONLY the L649 and L675 conditions; leave the MidChain test, the `chnBuildChainWithHook`
       helper, and the setups byte-identical.

# MUST READ — confirms the Issue 2 fix is LANDED (the tightened assertions pass today)
- file: internal/hooks/runner.go   (READ ONLY — do NOT edit)
  section: L107-110 — `if !strings.HasSuffix(finalMsg, "\n") { finalMsg += "\n" }` preceding
           `msgFile.WriteString(finalMsg)` (L110). This is the P1.M1.T2.S1 fix.
  why: confirms the message file fed to `prepare-commit-msg` ends with `\n`, so the hook's `echo ... >> $1`
       append lands on its own line ⇒ `git log --format=%B` (trimmed) = `<subject>\n[HOOK-RAN]` ⇒ the
       tightened assertions PASS against the current code (and fail only on a regression).
  critical: do NOT edit runner.go (it's already fixed). This task is test-only.

# The bug context (in your context as selected_prd_content h3.4 / h2.3)
- file: plan/010_…/bugfix/001_d93268e01058/prd_snapshot.md (Bug-Fix PRD)
  section: §h2.3 / §h3.4 (Issue 5) — the test-quality gap + the recommended `strings.Contains("\n"+logMsg+
           "\n", "\n[HOOK-RAN]\n")` form.
  critical: the recommended assertion form is the contract — use it (the `"\n"+X+"\n"` double pad).

# Confirms the parallel task is code-only (no test-file overlap)
- docfile: plan/010_49117f1f30ab/bugfix/001_d93268e01058/P1M3T1S3/PRP.md
  section: the header — it inserts an empty-message guard in `internal/decompose/message.go` (`publishCommit`
           CODE). It does NOT edit message_test.go or chain_test.go.
  why: confirms no parallel-edit conflict. This task edits ONLY those two test files.
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  message_test.go   # *** EDIT *** — TestPublishCommit_PrepareCommitMsgAnnotates assertion (L428). helpers msgRunGit/msgGitOut (TrimSpace) — unchanged.
  chain_test.go     # *** EDIT *** — _NullNewCommit_RunsHooks (L649) + _TipAmend_RunsHooks (L675) assertions. chnRunGit (TrimSpace), chnBuildChainWithHook, MidChain_SkipsHooks — unchanged.
  message.go        # READ ONLY (P1.M3.T1.S3 edits this in parallel — publishCommit empty-msg guard). NOT this task.
internal/hooks/
  runner.go         # READ ONLY — L107-110 Issue 2 fix (LANDED). NOT this task.
go.mod / go.sum     # UNCHANGED (test-only; strings already imported).
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. TWO in-place test edits: message_test.go (1 assertion) + chain_test.go (2 assertions).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (pad BOTH sides, design §1): the helpers msgRunGit/msgGitOut (message_test.go:51-58) and
//   chnRunGit (chain_test.go:52-58) return strings.TrimSpace(...). So logMsg/headMsg have NO trailing "\n"
//   (the marker is the last line, end-trimmed). The assertion MUST be strings.Contains("\n"+X+"\n",
//   "\n[HOOK-RAN]\n") — the trailing +"\n" restores the boundary TrimSpace removed. Omit it and the check
//   fails spuriously on the (correct) fixed output.

// CRITICAL (the checked break is BEFORE the marker): the Issue 2 fix guarantees a "\n" BEFORE [HOOK-RAN]
//   (the message file ends with "\n" before the hook appends). The assertion's "\n[HOOK-RAN]" half verifies
//   exactly that. It correctly FAILS on the corruption "feat: add new[HOOK-RAN]" (no preceding "\n").

// GOTCHA (LEAVE the negative test): TestResolveArbiter_MidChain_SkipsHooks (chain_test.go ~698-710) asserts
//   the ABSENCE of [HOOK-RAN] in rebuilt commits (the §20.2 mid-chain-fidelity invariant). That is correct
//   as-is — do NOT "tighten" it, do NOT touch it.

// GOTCHA (LEAVE setups + hooks): the 3 tests use the IDENTICAL hook — msgInstallHook (message_test.go:419)
//   and chnBuildChainWithHook (chain_test.go:627) both install "#!/bin/sh\necho '[HOOK-RAN]' >> \"$1\"\n".
//   Do NOT change the hook body, the repo setup, or the publishCommit/resolveArbiter calls.

// GOTCHA (white-box package): both test files are `package decompose` (they call unexported
//   publishCommit/resolveArbiter). No import changes needed — `strings` is already imported in both.

// GOTCHA (the Issue 2 fix is LANDED): runner.go:107-110 ensures the trailing newline. The tightened
//   assertions PASS today. They exist to catch a REGRESSION (removing/reverting L107-110).
```

## Implementation Blueprint

### Data models and structure

```go
// NO data models. A test-only change. The "structure" is the 3 assertion-condition replacements.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/decompose/message_test.go — TestPublishCommit_PrepareCommitMsgAnnotates assertion
  - LOCATE the assertion (L428-429):
        if !strings.Contains(logMsg, "[HOOK-RAN]") {
            t.Errorf("committed message = %q, want it to carry the [HOOK-RAN] append (hooks ran)", logMsg)
        }
  - REPLACE with:
        if !strings.Contains("\n"+logMsg+"\n", "\n[HOOK-RAN]\n") {
            t.Errorf("committed message = %q, want [HOOK-RAN] on its own line (the hook append must not glue onto the subject — Issue 2 parity)", logMsg)
        }
  - GOTCHA: replace ONLY the condition + (optionally) the error message. The hook install (L419), the setup,
      the publishCommit call, and msgGitOut are unchanged. The variable is `logMsg`.

Task 2: EDIT internal/decompose/chain_test.go — TestResolveArbiter_NullNewCommit_RunsHooks assertion
  - LOCATE (L649-650):
        if !strings.Contains(headMsg, "[HOOK-RAN]") {
            t.Errorf("HEAD message = %q, want it to carry [HOOK-RAN] (resolveNewCommit ran hooks)", headMsg)
        }
  - REPLACE with:
        if !strings.Contains("\n"+headMsg+"\n", "\n[HOOK-RAN]\n") {
            t.Errorf("HEAD message = %q, want [HOOK-RAN] on its own line (resolveNewCommit ran hooks — Issue 2 parity)", headMsg)
        }
  - GOTCHA: the variable is `headMsg`. chnBuildChainWithHook + chnRunGit + the resolveArbiter call unchanged.

Task 3: EDIT internal/decompose/chain_test.go — TestResolveArbiter_TipAmend_RunsHooks assertion
  - LOCATE (L675-676):
        if !strings.Contains(headMsg, "[HOOK-RAN]") {
            t.Errorf("amended tip message = %q, want it to carry [HOOK-RAN] (resolveTipAmend ran hooks — amend parity)", headMsg)
        }
  - REPLACE with:
        if !strings.Contains("\n"+headMsg+"\n", "\n[HOOK-RAN]\n") {
            t.Errorf("amended tip message = %q, want [HOOK-RAN] on its own line (resolveTipAmend ran hooks — amend parity, Issue 2)", headMsg)
        }
  - GOTCHA: the variable is `headMsg`. LEAVE the negative test (TestResolveArbiter_MidChain_SkipsHooks,
      ~L698-710) byte-identical — it asserts ABSENCE and is correct as-is.

Task 4: VERIFY (test-only validation)
  - `gofmt -w internal/decompose/message_test.go internal/decompose/chain_test.go`
  - `go test ./internal/decompose/ -run 'TestPublishCommit_PrepareCommitMsgAnnotates|TestResolveArbiter_' -v`
      → the 3 tightened tests PASS; the MidChain negative test still PASSES.
  - `go build ./... && go test ./...` → GREEN (no code changed; the suite is just stricter).
  - `git status` → ONLY message_test.go + chain_test.go; `git diff --exit-code` on internal/hooks/runner.go,
      internal/decompose/message.go, go.mod → all unchanged.
```

### Implementation Patterns & Key Details

```go
// THE assertion form (the contract's recommendation — pad BOTH sides):
//   strings.Contains("\n"+X+"\n", "\n[HOOK-RAN]\n")
// where X = logMsg (test 1) or headMsg (tests 2 & 3).

// WHY double-sided pad: msgRunGit/msgGitOut/chnRunGit all return strings.TrimSpace(...). The marker is the
// LAST line of the (trimmed) message, so without the trailing +"\n" there's no "\n" after [HOOK-RAN] and
// the check fails on the CORRECT output. The leading "\n"+ makes it correct even if the marker were first.

// THE regression guard (this is the whole point):
//   fixed output  "feat: add new\n[HOOK-RAN]"  → padded "\nfeat: add new\n[HOOK-RAN]\n" → CONTAINS "\n[HOOK-RAN]\n" ✓ PASS
//   corrupt output "feat: add new[HOOK-RAN]"   → padded "\nfeat: add new[HOOK-RAN]\n"   → no "\n[HOOK-RAN]\n"   ✗ FAIL
// (The old strings.Contains passed on BOTH — that's why Issue 2 shipped undetected.)

// ALTERNATIVE (equivalent, more readable — optional): split into lines and check for an exact line:
//   found := false
//   for _, line := range strings.Split(headMsg, "\n") {
//       if line == "[HOOK-RAN]" { found = true; break }
//   }
//   if !found { t.Errorf(...) }
// Either form is correct; the contract recommends the inline Contains-with-pad form (used above).
```

### Integration Points

```yaml
TEST.FILES (the ONLY edits): internal/decompose/message_test.go (1 assertion) + chain_test.go (2 assertions).

LEFT-UNCHANGED (do NOT edit):
  - internal/decompose/chain_test.go :: TestResolveArbiter_MidChain_SkipsHooks (~L698-710) — ABSENCE check.
  - the test setups, the hook installations (msgInstallHook / chnBuildChainWithHook), the helpers
    (msgRunGit/msgGitOut/chnRunGit/chnInstallHook/chnBuildChainWithHook).
  - internal/decompose/message.go (P1.M3.T1.S3 edits this in parallel — NOT this task).
  - internal/hooks/runner.go (the Issue 2 fix is LANDED — NOT this task).
  - every non-test file; go.mod/go.sum; PRD.md; Makefile.

GO.MODULE: change NONE. `strings` is already imported in both test files. `go mod tidy` is a no-op.

DEPENDENCY: the Issue 2 fix (P1.M1.T2.S1, runner.go:107-110) — LANDED. Without it the tightened assertions
      would FAIL (the marker would glue onto the subject); the task assumes the fix is in place (it is).
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/decompose/message_test.go internal/decompose/chain_test.go
go vet ./internal/decompose/
# Confirm the 3 tightened conditions landed:
grep -n 'Contains("\\n"+logMsg+"\\n", "\\n\[HOOK-RAN\]\\n")' internal/decompose/message_test.go
grep -n 'Contains("\\n"+headMsg+"\\n", "\\n\[HOOK-RAN\]\\n")' internal/decompose/chain_test.go   # expect 2 hits
# Confirm the negative test is UNCHANGED (still an absence check):
grep -n 'TestResolveArbiter_MidChain_SkipsHooks' internal/decompose/chain_test.go
# Expected: go vet clean; the 3 tightened Contains conditions present; the MidChain test still present.
```

### Level 2: The 3 tests pass + no regression

```bash
go test ./internal/decompose/ -run 'TestPublishCommit_PrepareCommitMsgAnnotates|TestResolveArbiter_NullNewCommit_RunsHooks|TestResolveArbiter_TipAmend_RunsHooks|TestResolveArbiter_MidChain_SkipsHooks' -v
# Expected: all 4 PASS. The 3 tightened tests pass against the (fixed) code; the MidChain negative test passes.
go test ./internal/decompose/    # the full decompose suite — no regression.
# If a tightened test FAILS with "...want [HOOK-RAN] on its own line...", either the Issue 2 fix regressed
# (re-check runner.go:107-110) or the pad is wrong (re-check the trailing +"\n").
```

### Level 3: Scope check + regression no-op (no code touched)

```bash
go build ./...   # Expect clean & unchanged (no non-test file touched).
go test ./...    # Expect GREEN & unchanged (test-only; the suite is stricter, not different).
git status --porcelain
# Expected: exactly TWO modified files — message_test.go + chain_test.go. NOTHING else.
git diff --exit-code internal/hooks/runner.go internal/decompose/message.go go.mod go.sum PRD.md \
  && echo "runner.go / message.go / go.mod / PRD UNCHANGED (expected)"
! git diff --name-only | grep -vE '_test\.go$' | grep -E '\.go$' && echo "OK: no non-test .go modified"
```

### Level 4: The regression-guard proof (the assertions actually catch Issue 2)

```bash
# Prove the tightened assertion FAILS on the Issue 2 corruption (the guard works). A 1-minute reasoning
# check (no code change needed):
#   corrupt = "feat: add new[HOOK-RAN]"
#   strings.Contains("\n"+corrupt+"\n", "\n[HOOK-RAN]\n")
#     = strings.Contains("\nfeat: add new[HOOK-RAN]\n", "\n[HOOK-RAN]\n")
#     = false  (no "\n" immediately before [HOOK-RAN])  ✗ → t.Errorf fires. GUARD WORKS.
#   fixed  = "feat: add new\n[HOOK-RAN]"
#     = strings.Contains("\nfeat: add new\n[HOOK-RAN]\n", "\n[HOOK-RAN]\n")
#     = true   ✓ → passes against the current (fixed) code.
# (Optional empirical proof: temporarily revert runner.go:107-110, run the 3 tests — they FAIL; restore.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean & unchanged; `go vet ./...` clean; `gofmt -l` clean on the 2 test files.
- [ ] `go test ./...` GREEN & unchanged (test-only — no code touched).
- [ ] `git status` shows EXACTLY TWO modified files: message_test.go + chain_test.go. No other file touched.
- [ ] go.mod/go.sum/runner.go/message.go byte-unchanged; no non-test `.go` file modified.

### Feature Validation
- [ ] The 3 assertions use `strings.Contains("\n"+X+"\n", "\n[HOOK-RAN]\n")` (X = logMsg / headMsg).
- [ ] The 3 tests PASS against the current (fixed) code.
- [ ] `TestResolveArbiter_MidChain_SkipsHooks` (absence check) is byte-UNCHANGED and still passes.
- [ ] Setups, hook installations, and helpers are byte-unchanged.
- [ ] (Reasoning) the tightened assertion FAILS on `feat: add new[HOOK-RAN]` (the Issue 2 corruption).

### Code Quality Validation
- [ ] The 3 edits are minimal (the assertion condition + optionally the error message); nothing else.
- [ ] The error messages name the variable and the invariant ("on its own line").
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (negative test / setups / code frozen).

### Documentation
- [ ] [Mode A] none needed — test-only change, no user-facing impact. (The tightened assertion's error
      message is self-documenting.)

---

## Anti-Patterns to Avoid

- ❌ **Don't forget the trailing `+"\n"`.** The helpers `strings.TrimSpace` the git output, so `logMsg`/
  `headMsg` have no trailing newline. `strings.Contains("\n"+X+"\n", "\n[HOOK-RAN]\n")` — the trailing pad
  is REQUIRED; without it the check fails spuriously on the correct output. (§1)
- ❌ **Don't check only `strings.Contains(X, "\n[HOOK-RAN]")` (one-sided).** That misses the trailing
  boundary; use the double-sided pad form the contract specifies. (§1)
- ❌ **Don't touch the negative test.** `TestResolveArbiter_MidChain_SkipsHooks` asserts ABSENCE of
  `[HOOK-RAN]` (the §20.2 mid-chain-fidelity invariant) — it is correct as-is. Leave it byte-identical. (§4)
- ❌ **Don't change the setup, the hook body, or the helpers.** The hook is `echo '[HOOK-RAN]' >> "$1"` in
  all 3 tests (msgInstallHook / chnBuildChainWithHook); the setups and `publishCommit`/`resolveArbiter`
  calls are unchanged. Only the assertion condition changes. (§0/§4)
- ❌ **Don't edit any non-test file.** The Issue 2 fix is already in `internal/hooks/runner.go:107-110`
  (LANDED). This task is test-only. `internal/decompose/message.go` is the parallel P1.M3.T1.S3's scope. (§5)
- ❌ **Don't change the variable names.** Test 1 is `logMsg`; tests 2 & 3 are `headMsg`. Substitute the
  correct one in each `"\n"+X+"\n"` pad. (gotcha)
- ❌ **Don't assume the assertions will fail today.** The Issue 2 fix is in place → they PASS now. They
  exist to catch a FUTURE regression. If they fail on green code, the pad is wrong (re-check §1). (§2)
