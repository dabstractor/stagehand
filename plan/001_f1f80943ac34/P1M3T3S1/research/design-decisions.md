# P1.M3.T3.S1 — Rescue message formatting: design decisions

> The SINGLE source of truth for the judgment calls in this subtask. Read BEFORE implementing.
> Every decision is pinned by a test in `rescue_test.go`. The PRD is authoritative; where the PRD
> is ambiguous, the choice is made on the principle of **literal CONTRACT fidelity + maximal
> auditability**, and the alternative is recorded with reasoning.

The work-item CONTRACT (item_description, point 3) is:

> Create `internal/generate/rescue.go` with `FormatRescue(treeSHA, parentSHA, candidateMsg string) string`
> returning the §18.3 formatted message. If `parentSHA == ""` (root commit): recovery command omits -p.
> If `candidateMsg != ""`: append the 'A candidate message was produced but rejected' note with the message.
> The orchestrator (P1.M3.T4) calls this and the CLI layer prints it + exits 3.

---

## §0 — Scope boundary: FORMAT ONLY, nothing else

This subtask exports **ONE** pure function: `FormatRescue`. It assembles and returns a string.
It does NOT:

- **print** anything (the CLI layer P1.M4.T1 does `fmt.Fprintln` / `fmt.Fprint`),
- **exit** with code 3 (the exit-code system P1.M4.T3.S3 does that),
- **detect the rescue condition** (TREE_SHA set + NEW_SHA not set — that's the orchestrator
  P1.M3.T4's `if treeSHA != "" && newSHA == ""` gate; FR43),
- **install the SIGINT/SIGTERM handler** (P1.M4.T2 — the "(interrupted)" notice is theirs; see §5),
- **decide** which failure mode triggers rescue (timeout 124→3, parse→3, dedupe-exhaustion→3 —
  §18.2's table; that mapping is the orchestrator's),
- **fetch** TREE_SHA / PARENT_SHA from git (P1.M1.T2.S3 WriteTree + P1.M1.T2.S2 RevParseHEAD supply
  them; FormatRescue just receives them as strings).

`FormatRescue` is a **pure string assembler**: 3 string inputs → 1 string output, no I/O, no error.
Mirrors `prompt.BuildUserPayload` (P1.M3.T1.S3) and `generate.ExtractSubject`/`IsDuplicate`
(P1.M3.T2.S1) exactly — Stagecoach's generation pipeline is built from small exported pure functions
that the orchestrator composes. Implementing any of the above here would duplicate P1.M3.T4 /
P1.M4.T1 / P1.M4.T2 / P1.M4.T3 and couple a pure-logic file to os/git/signal. **Don't.**

---

## §1 — The frozen signature

```go
func FormatRescue(treeSHA, parentSHA, candidateMsg string) string
```

- **Exported** (capitalized `F` — the work-item contract; the public API P1.M3.T5 may re-export it).
- **Three `string` params** in this EXACT order: `treeSHA, parentSHA, candidateMsg`.
- **Returns ONE `string`** — NO error (formatting has no failure mode). NO `(string, error)`.
  NO `*strings.Builder` return (callers want a value, not a builder).
- The signature is **FROZEN** after this subtask. Downstream consumer (P1.M3.T4 orchestrator):

  ```go
  // rescue path (FR43–FR45): treeSHA set, newSHA not set
  msg := generate.FormatRescue(treeSHA, parentSHA, candidateMsg)
  // CLI layer (P1.M4.T1) prints msg + sets exit 3 (P1.M4.T3.S3)
  ```

  `parentSHA` is `""` for a root commit (unborn repo — `git.RevParseHEAD` returns isUnborn=true ⇒
  parentSHA="", FINDING 1). `candidateMsg` is the parsed-but-rejected message (duplicate-exhaustion
  or parse failure with a message in hand — §18.3 last paragraph), or `""` (timeout / interrupt /
  no message produced).

---

## §2 — §18.3 verbatim: the exact message structure

The rescue message is **§18.3 rendered character-for-character** (prd_snapshot.md lines 1177–1186,
re-verified byte-for-byte — see "verification" below). It is a 10-line block:

```
❌ Commit generation failed.                                   ← line 1 (❌ = U+274C)
------------------------------------------------------------   ← line 2 (EXACTLY 60 '-')
Your staged files were safely snapshotted before generation.   ← line 3
Tree ID: <TREE_SHA>                                            ← line 4 (substitute treeSHA)
                                                               ← line 5 (BLANK)
To commit the originally staged files manually:                ← line 6
  git commit-tree -p <PARENT_SHA> -m "Your message" <TREE_SHA> | xargs git update-ref HEAD   ← line 7 (2 LEADING SPACES; substitute; omit -p if root)
                                                               ← line 8 (BLANK)
(omit "-p <PARENT_SHA>" if this is the repository's first commit)   ← line 9
------------------------------------------------------------   ← line 10 (EXACTLY 60 '-')
```

**Verification performed against prd_snapshot.md (the authoritative PRD text):**

| fact | how verified | value |
|------|--------------|-------|
| separator dash count (line 1178) | `sed -n '1178p' … \| tr -cd '-' \| wc -c` | **60** |
| separator dash count (line 1186) | `sed -n '1186p' … \| tr -cd '-' \| wc -c` | **60** |
| separator bytes | `sed -n '1178p' … \| od -c` | 60× `-` then `\n` (pure ASCII, no spaces) |
| ❌ bytes (line 1177) | `sed -n '1177p' … \| od -c` | `342 235 214` = **U+274C CROSS MARK** (NON-ASCII) |
| command leading spaces (line 1183) | `sed -n '1183p' … \| cat -A` | **exactly 2 spaces** before `git` |

**CRITICAL — the ❌ is non-ASCII.** Unlike §17.3 (the user payload, pure ASCII), §18.3's first line
begins with `❌` (U+274C, 3 UTF-8 bytes). Go source files are UTF-8, so the literal `❌` may be
written directly in a double-quoted string: `"❌ Commit generation failed."`. Do NOT replace it with
`[X]`, `x`, `!`, or an ASCII stand-in — the test asserts the literal `❌` byte sequence. `go vet`
and `gofmt` accept it (they are UTF-8 clean). (This is a deliberate §18.3 fidelity call, NOT an
oversight — the PRD author chose the emoji for the failure notice.)

**The placeholders `<TREE_SHA>` and `<PARENT_SHA>` are STRUCTURAL ANNOTATIONS in §18.3** — they mark
where dynamic content goes; they are NOT literal output. `FormatRescue` substitutes the runtime
`treeSHA` / `parentSHA` arguments (exactly as `prompt.BuildUserPayload` substitutes the runtime `diff`
for `<diff payload>` — P1.M3.T1.S3 design-decisions §5). In the CONCRETE render (see Appendix B.5:
`git commit-tree -p abc1234 -m "Your message" 9f3a1c...`), the SHAs are real values, not `<…>` tokens.

---

## §3 — THE key decision: the "(omit -p …)" hint line stays ALWAYS

§18.3 line 9 is: `(omit "-p <PARENT_SHA>" if this is the repository's first commit)`. The work-item
CONTRACT specifies exactly ONE dynamic modification beyond literal §18.3: *"If parentSHA == "" (root
commit): recovery command omits -p."* It says NOTHING about removing line 9.

**Decision: line 9 is emitted in BOTH cases (rooted and rootless), byte-for-byte §18.3-verbatim.**

Reasoning (literal-CONTRACT + maximal-auditability):

1. The CONTRACT says *"returning the §18.3 formatted message."* §18.3 includes line 9. Removing it
   would be an **undocumented deviation** from §18.3 — a guess about intent.
2. The CONTRACT lists exactly ONE dynamic change (command omits -p for root). By the principle of
   minimal surprise, **everything else stays §18.3-verbatim**, including line 9.
3. Line 9 is **static, case-independent guidance** ("if this is the repository's first commit"). In
   the root case the command already omits -p, and line 9 reaffirms WHY — mildly redundant but never
   wrong. In the rooted case it warns the user about the first-commit variant.
4. This yields **ONE canonical form, two cases differing only in the command line** — maximally
   testable (two `want` strings, identical except the command line).
5. Any trimming of line 9 is the **CLI/orchestrator's prerogative** (P1.M3.T4 / P1.M4.T1), not the
   formatter's. `FormatRescue`'s job is the §18.3 message, full stop.

**Alternative considered + rejected:** *"drop line 9 when parentSHA == '' (root), keep it when
rooted."* Rejected because (a) it contradicts literal §18.3, (b) it's undocumented in the CONTRACT,
(c) it forks the output into two structurally-different forms (harder to test/audit), (d) the
redundancy in the root case is harmless. If a reviewer later wants it dropped, that is a one-line
change gated behind a test — but the DEFAULT, faithful-to-PRD behavior is to keep it.

**Pinned by tests:** both the rooted and rootless canonical-exact `want` strings INCLUDE line 9
verbatim; a structural-invariant test asserts line 9 is present in both cases.

---

## §4 — Root-commit handling: omit the `-p <PARENT_SHA>` segment

The command line (line 7) is the ONLY fully-dynamic line. Two forms:

- **Rooted** (`parentSHA != ""`):
  `  git commit-tree -p <parentSHA> -m "Your message" <treeSHA> | xargs git update-ref HEAD`
- **Root / unborn** (`parentSHA == ""`):
  `  git commit-tree -m "Your message" <treeSHA> | xargs git update-ref HEAD`
  (the ` -p <parentSHA>` segment is OMITTED entirely — CONTRACT point 3)

This **mirrors `git.CommitTree`** (P1.M1.T2.S4, git.go line 240): `parents == nil/empty ⇒ root commit
(no -p appended); each element appends a -p <parent>`. Same root-commit semantics, same gate
(`parentSHA == ""` ⇐⇒ `len(parents) == 0`). FR39 confirms: *"if PARENT_SHA is non-empty, `git
commit-tree -p …`; else `git commit-tree -m …` (root commit)."*

**Assembly detail (keep it a single conditional splice, NOT fmt.Sprintf):** build the command with
`strings.Builder`; write the fixed prefix `  git commit-tree`; `if parentSHA != ""` write
` -p ` + parentSHA; then write the fixed tail ` -m "Your message" ` + treeSHA + ` | xargs git update-ref
HEAD`. This needs ONLY `"strings"` (no `fmt` import) — keeping rescue.go a stdlib-only leaf like
dedupe.go (§8). The 2 leading spaces are part of the prefix literal.

**GOTCHA — the quotes around `Your message`:** the `-m "Your message"` uses literal ASCII
double-quotes (§18.3 line 1183, verified). These are part of the shell-command template the USER will
copy-paste; they are NOT Go-string delimiters. In a Go double-quoted literal they must be escaped
(`\"`) or the segment split across `WriteString` calls. The cleanest is to write the fixed segments
as raw literals split around the dynamic SHAs (see Implementation Blueprint).

**Pinned by tests:** the rooted `want` command line contains `-p ` + parentSHA; the rootless `want`
command line contains NO `-p` substring at all; both contain treeSHA in BOTH the `Tree ID:` line and
the command.

---

## §5 — The "(interrupted)" variant is OUT OF SCOPE (signal handler)

Appendix B.5 (prd_snapshot.md line 1389) renders the rescue with `❌ Commit generation failed
(interrupted).` — but §18.3 (the authoritative FR43–FR45 spec, line 1177) renders it as
`❌ Commit generation failed.` (no "(interrupted)").

**Decision: `FormatRescue` produces the §18.3 BASE form** — `❌ Commit generation failed.` —
**always.** The "(interrupted)" suffix is the **signal handler's** (P1.M4.T2) concern.

Reasoning:

1. `FormatRescue`'s signature has **no `interrupted` / `reason` parameter** (CONTRACT point 3: exactly
   three string params). There is no way to signal "interrupted" into it.
2. §18.3 (the spec this subtask implements) uses the base form; B.5 is an **example terminal session**
   showing the SIGINT case — a different render path owned by the signal handler.
3. B.5 ALSO differs from §18.3 in a second way (it drops the "(omit -p)" hint line) — confirming B.5
   is the signal handler's bespoke render, NOT `FormatRescue`'s output. Treat B.5 as out-of-scope.
4. The signal handler (P1.M4.T2, per FINDING 8) will: forward the signal to the child's process group,
   then either (a) print its own `❌ Commit generation failed (interrupted).` notice and call a variant,
   or (b) call `FormatRescue` and the CLI prepends/appends "(interrupted)". That wiring is P1.M4.T2,
   NOT this subtask.

**Do NOT** add an `interrupted bool` param, a `reason string` param, or a second `FormatRescueInterrupted`
function. The CONTRACT freezes the 3-param signature. If P1.M4.T2 needs the interrupted variant, it
post-processes the string or the orchestrator passes a composed `candidateMsg`/notice — that's their
call. `FormatRescue` = §18.3 base, period.

**Pinned by tests:** the canonical-exact `want` first line is `❌ Commit generation failed.` (no
"(interrupted)"); a structural test asserts the output does NOT contain "interrupted".

---

## §6 — Candidate note: position, separator, exact text

§18.3 last paragraph (prd_snapshot.md line 1189):

> If the failure was a duplicate-exhaustion or parse failure *with a candidate message in hand*,
> additionally print: *"A candidate message was produced but rejected: \"<msg>\". You can use it
> manually in the command above."* — so the user's wait wasn't wasted.

**Exact note text (substitute `<msg>` → candidateMsg):**
```
A candidate message was produced but rejected: "<candidateMsg>". You can use it manually in the command above.
```
(literal ASCII double-quotes around `<candidateMsg>` — part of the rendered output, the user copies
the quoted message into the recovery command's `-m` slot).

**Gate:** appended **only when `candidateMsg != ""`** (CONTRACT point 3). Empty/nil candidateMsg ⇒
the boxed message alone, no note. (Timeout / interrupt / no-message-produced rescues pass `""`.)

**Position + separator (the judgment call):** the PRD says "additionally print" — i.e. IN ADDITION
to the boxed message (lines 1–10), as a separate paragraph. The boxed message ends at the closing
separator (line 10, 60 dashes). The note references "the command above" (the command is inside the
box, line 7) ⇒ the note must come AFTER the box.

**Decision: append the note AFTER the closing separator, separated by ONE blank line.** The return
value's tail is then: `…------------------------------------------------------------` (closing sep, line 10) + `\n\n` +
`A candidate message … command above.` (no trailing newline).

Reasoning:
- "additionally print" = a separate paragraph, not inside the box (the box is a self-contained
  §18.3 unit closed by line 10's separator).
- A single blank line between the box and the note is the conventional, readable separator and makes
  "the command above" unambiguous (the box is visually distinct above the note).
- Putting the note INSIDE the box (before the closing separator) would change the box's line count
  and break the "10-line box" invariant — rejected.

**Trailing newline (§7) applies:** even with the candidate note, the return value has NO trailing
newline; the CLI adds it.

**Pinned by tests:** a canonical-exact test with `candidateMsg != ""` asserts the full output =
box + `\n\n` + note; a structural test asserts the candidate note is present iff `candidateMsg != ""`,
and that the note contains `"<candidateMsg>"` (with the literal quotes).

---

## §7 — Trailing newline: NONE (the CLI adds it)

`FormatRescue` returns the message **WITHOUT a trailing `\n`** — matching `prompt.BuildUserPayload`'s
convention (P1.M3.T1.S3 design-decisions: "Constants are defined WITHOUT trailing newlines;
BuildUserPayload owns ALL inter-block newline placement"). The internal blank lines (line 5, line 8,
and the candidate-note separator) ARE part of the return value; only the FINAL byte is not `\n`.

The CLI layer (P1.M4.T1) prints it with a trailing newline: `fmt.Fprintln(os.Stderr, msg)` (Println
appends `\n`). Keeping the trailing newline out of the formatter means the formatter is a pure
value-assembler and the CLI owns the I/O newline — clean separation (§0).

**Pinned by tests:** a structural invariant asserts `!strings.HasSuffix(got, "\n")` for all cases
(no candidate, with candidate, rooted, rootless).

---

## §8 — Package + imports: `package generate`, `"strings"` ONLY

- **Package:** `package generate` — the SAME package as `dedupe.go` (P1.M3.T2.S1). rescue.go + rescue_test.go
  ADD to it; no new package, no merge-collision risk (different filenames).
- **Imports (rescue.go):** EXACTLY `"strings"` (for `strings.Builder`). NO `"fmt"` (avoid Sprintf —
  build with Builder + WriteString, §4), NO `internal/*`, NO third-party.
- **Imports (rescue_test.go):** `"strings"` (for `strings.Repeat` in the independently-derived `want` +
  `strings.Contains`/`HasSuffix` in structural asserts) + `"testing"`. NO `fmt`.
- **`go mod tidy` is a NO-OP.** `git diff --exit-code go.mod go.sum` MUST be empty. `internal/generate`
  stays a stdlib-only leaf (same as after P1.M3.T2.S1).

**GOTCHA — do NOT add a second `// Package generate` doc comment.** dedupe.go (P1.M3.T2.S1) already
carries the package doc comment (`// Package generate provides …`). Go tooling (`revive`/`golint`
rule `redundant-build-comment` / duplicate-package-comment) WARNS if a SECOND file in the package has
a `// Package generate …` comment. rescue.go must use **function-level doc comments only** (on
`FormatRescue` and any constants) — NO `// Package generate` line. (dedupe.go is FROZEN — do not edit
it to generalize the package doc; that's a coordination concern for a later sweep, not this subtask.)

---

## §9 — Test strategy: mirror `payload_test.go` (independently-derived `want`)

`FormatRescue` is a pure string assembler with a multi-line output — the EXACT shape of
`prompt.BuildUserPayload` (P1.M3.T1.S3). Mirror `internal/prompt/payload_test.go`'s convention:

- **Canonical-exact tests:** build the `want` string **independently from §18.3** (NOT by calling
  `FormatRescue` or referencing its constants) using string concatenation + `strings.Repeat("-", 60)`,
  so a `got == want` match is MEANINGFUL (a typo in either would diverge). Compare with `got != want`
  and the `--- got ---\n%q\n--- want ---\n%q` error format (payload_test.go lines 19/45).
- **Four canonical cases:** (rooted, no candidate), (rootless, no candidate), (rooted, with candidate),
  (rootless, with candidate). Each is a full-output byte-for-byte assertion.
- **Structural-invariant table (`TestFormatRescue_Properties`):** a `cases := []struct{...}` table +
  `for _, tc := range cases { t.Run(tc.name, …) }` loop (mirror `parse_test.go` / `dedupe_test.go`),
  asserting the load-bearing invariants that the canonical tests don't isolate:
  - contains `❌` (U+274C);
  - exactly TWO separator lines (top + bottom), each exactly 60 dashes (`strings.Count` + a helper
    that splits on `\n` and checks each `== rescueSepLen` line);
  - command line contains `treeSHA` AND contains `parentSHA` iff `parentSHA != ""`;
  - command line contains `-p ` iff `parentSHA != ""`;
  - `Tree ID: <treeSHA>` line present;
  - line 9 `(omit "-p <PARENT_SHA>" …)` present in BOTH rooted and rootless cases;
  - candidate note present iff `candidateMsg != ""`; when present, contains `"<candidateMsg>"`
    (literal quotes) and the exact §18.3 note text;
  - output does NOT contain "interrupted" (§5);
  - `!strings.HasSuffix(got, "\n")` (§7 — no trailing newline).

**Defensive cases:** empty `treeSHA` (produces a message with empty SHAs — no panic; the orchestrator
gates on treeSHA != "" so this won't happen in practice, but the pure function must not panic);
`candidateMsg` containing quotes/newlines (appended verbatim — no sanitization, matching §17.3's
"subjects appended verbatim" philosophy; a test pins that quotes are NOT escaped).

**NO subprocess, NO temp repo, NO git.** rescue is pure-function — use the `payload_test.go`/
`parse_test.go` table style, NOT the `internal/git/*_test.go` temp-repo style.

---

## §10 — Frozen files + upstream/downstream contracts

**FROZEN (do NOT edit — touches ONLY rescue.go + rescue_test.go):**
`internal/generate/dedupe.go` + `dedupe_test.go` (P1.M3.T2.S1, parallel-implementing — assume they
exist per the parallel_execution_context contract), `internal/prompt/*`, `internal/git/*`,
`internal/provider/*`, `internal/config/*`, `cmd/stagecoach/main.go`, `pkg/*`, `Makefile`, `go.mod`,
`go.sum`.

**UPSTREAM (inputs — consume, don't implement):**
- `treeSHA`: `git.WriteTree(ctx)` (P1.M1.T2.S3) — the snapshot tree SHA; the orchestrator captures it
  pre-generation and holds it for the rescue path. Non-empty when rescue is reachable (FR43: TREE_SHA set).
- `parentSHA`: `git.RevParseHEAD(ctx)` (P1.M1.T2.S2) — `sha` if `isUnborn == false`, `""` if
  `isUnborn == true` (FINDING 1: unborn detected by exit code 128, NOT string emptiness). The
  orchestrator captures this pre-generation; `""` ⇒ root commit ⇒ command omits -p (§4).
- `candidateMsg`: the `msg` return of `provider.ParseOutput(raw, manifest)` (P1.M2.T6.S1, Step 4/5
  trim + newline-normalize) IF a message was produced before the duplicate/parse rejection; `""`
  otherwise (timeout/interrupt/no-output). The orchestrator decides which (§18.2).

**DOWNSTREAM (consumers — honor the signature, don't implement their logic):**
- `P1.M3.T4` (orchestrator `CommitStaged`): calls `generate.FormatRescue(treeSHA, parentSHA,
  candidateMsg)` on the rescue path (FR43–FR45), hands the string to the CLI layer.
- `P1.M4.T1` (CLI default action): prints the string (`fmt.Fprintln`) — likely to stderr (it's an
  error notice) — and sets exit 3.
- `P1.M4.T3.S3` (exit codes): the `3` (P1.M4.T3.S3's `ExitError`).
- `P1.M4.T2` (signal handler): produces the "(interrupted)" variant (§5) by its own means.
- `P1.M3.T5` (public API `pkg/stagecoach`): may re-export `FormatRescue`.

The `FormatRescue(treeSHA, parentSHA, candidateMsg string) string` signature is **FROZEN** after
this subtask.
