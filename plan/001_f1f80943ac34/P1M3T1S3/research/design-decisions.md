# P1.M3.T1.S3 — User Payload Assembly: Design Decisions

The 10 non-obvious calls for `internal/prompt/payload.go` (`BuildUserPayload`). Read this BEFORE
implementing — it is the single most important reference. Ground truth: PRD §17.3 (the `selected_prd_content`
`h3.62` block, re-verified byte-for-byte against `plan/001_f1f80943ac34/prd_snapshot.md` lines 1117–1142),
PRD §9.3 FR15, PRD §9.7 FR30–FR32. Sibling contracts: S1/S2 `internal/prompt/system.go` (the pattern to
mirror), P1.M1.T3.S1 `git.StagedDiff` (the `diff` input), P1.M3.T2 dedupe loop (the `rejected` input),
P1.M2.T4 `Manifest.Render` (the downstream consumer of the output).

---

## §0 — NEW file `payload.go`, NOT an append to `system.go`

S1 created `system.go`; S2 (implementing in parallel) APPENDS to `system.go`. S3 creates a SEPARATE new
file `internal/prompt/payload.go` (+ `internal/prompt/payload_test.go`). Three reasons:

1. **Zero merge conflict with S2.** S2 appends `fallbackPromptBody` + `BuildFallbackPrompt` to the tail of
   `system.go`. If S3 also appends to `system.go`, the two appends collide. A new file sidesteps this
   entirely (S1's own PRP anticipated this: "S3 adds payload.go to this package").
2. **Cohesion.** The user payload (§17.3) is a distinct concern from the system prompt (§17.1/§17.2).
   `payload.go` + `system.go` is the natural split.
3. **Symmetry with the PRD.** §17.1/§17.2 are "the system prompt"; §17.3 is "the user payload" — a
   different PRD section, a different file.

Same `package prompt`. Touches ONLY the two new files. NO edit to `system.go` / `system_test.go` (S1/S2
own those), NO edit to any provider/config/git/cmd file.

---

## §1 — THE signature (implement the work-item form EXACTLY)

```go
func BuildUserPayload(diff string, rejected []string) string
```

- **EXPORTED** — the caller is `internal/generate` (P1.M3.T4 orchestrator), cross-package.
- **Returns `string` ONLY** — there is NO failure mode. Empty `diff` ⇒ the instruction (+ optional
  rejection block) with an empty tail; empty/nil `rejected` ⇒ the normal (colon) payload. Neither panics.
- **`diff string`** is the raw `git.StagedDiff` return value (the concatenated markdown + non-markdown
  diff text, possibly byte/line-capped with a sentinel). It is appended VERBATIM — no normalization.
- **`rejected []string`** is the list of duplicate subjects from the dedupe loop (P1.M3.T2, FR30: each is
  "the first line of the message" — guaranteed single-line). `len(rejected) == 0` selects the normal path.

Decoupled from `config`, `git`, `provider` — pure `(strings)`. Mirrors S1's `BuildSystemPrompt` and S2's
`BuildFallbackPrompt` (both exported, string-only, parameter-decoupled).

---

## §2 — THE load-bearing call: COLON (normal) vs PERIOD (rejection) — §17.3 is authoritative

This is the decision most likely to be implemented wrong. PRD §17.3 renders the instruction line
**differently** in the two cases (re-verified against `prd_snapshot.md`):

- **Normal** (`prd_snapshot.md` line 1123): `Generate a commit message for these changes:` ← **colon**
- **Rejection** (`prd_snapshot.md` line 1130): `Generate a commit message for these changes.` ← **period**

The work-item paraphrase shows the colon uniformly ("`'Generate a commit message for these changes:\n\n'
+ diff`") and says "insert the §17.3 rejection block between the instruction and the diff". That paraphrase
elides the period. **The PRD §17.3 is authoritative, NOT the paraphrase** — this is the exact precedent S1
and S2 established ("copy from §17.x character-for-character; the work-item paraphrase elides").

Decision: implement TWO instruction constants, each byte-identical to its §17.3 rendering:

```go
const userInstruction       = "Generate a commit message for these changes:"  // §17.3 normal  (colon)
const userInstructionReject = "Generate a commit message for these changes."  // §17.3 rejection (period)
```

Why this is consistent with FR15 ("a short, stable string"): the CORE "Generate a commit message for these
changes" IS stable; only the terminal punctuation adapts to context — a colon introduces the diff directly
(normal), a period terminates the directive before the `IMPORTANT:` block (rejection). FR15's "(e.g., …:)"
describes the normal case; §17.3 supplies the rejection variant. No contradiction.

The canonical tests pin BOTH renderings byte-for-byte, so a reviewer can see the period-vs-colon is
intentional and PRD-grounded, not a typo.

---

## §3 — Rejection block topology (exact)

The full rejection payload, top to bottom (PRD §17.3, re-derived independently):

```
Generate a commit message for these changes.          ← userInstructionReject (period) + '\n'
                                                       ← '\n'  (blank line)
IMPORTANT: The following messages were REJECTED because they already exist    ← preamble line 1 + '\n'
in git history. You MUST generate something COMPLETELY DIFFERENT:             ← preamble line 2 + '\n'
- <subject 1>                                          ← list item + '\n'   (per rejected subject)
- <subject 2>                                          ← list item + '\n'
                                                       ← '\n'  (blank line after the list)
Create an entirely new message with different wording. ← epilogue (no trailing '\n' on the constant)
                                                       ← '\n' '\n'  (blank line before the diff)
<diff payload>                                         ← diff, appended verbatim
```

Assembly (strings.Builder):
```go
b.WriteString(userInstructionReject)          // "…changes."
b.WriteString("\n\n")                         // instruction + blank line
b.WriteString(rejectionPreamble)              // "IMPORTANT: …\n…DIFFERENT:"  (no trailing \n)
b.WriteByte('\n')                             // end the preamble's second line
for _, s := range rejected {
    b.WriteString("- ")
    b.WriteString(s)
    b.WriteByte('\n')
}
b.WriteByte('\n')                             // blank line after the list
b.WriteString(rejectionEpilogue)              // "Create an entirely new message with different wording."
b.WriteString("\n\n")                         // blank line before the diff
b.WriteString(diff)                           // verbatim
```

Normal payload (no loop, fast path):
```go
return userInstruction + "\n\n" + diff         // "…changes:" + blank + diff
```

---

## §4 — Constants carry NO trailing newline; the builder owns every '\n'

Same rule as S1 (`maturePromptHeader`, `antiReuseProhibition`) and S2 (`fallbackPromptBody`): each
canonical string constant is defined WITHOUT a trailing newline, and `BuildUserPayload` owns all
inter-block newline placement. This keeps the §17.3 blank-line topology in exactly one auditable place
(the builder body) and makes the constants copy-paste-faithful to the PRD text blocks.

The four constants (all verbatim from §17.3, NO trailing newlines):
- `userInstruction` — `Generate a commit message for these changes:` (colon)
- `userInstructionReject` — `Generate a commit message for these changes.` (period)
- `rejectionPreamble` — the two-line `IMPORTANT: …\n…DIFFERENT:` block (raw string literal)
- `rejectionEpilogue` — `Create an entirely new message with different wording.`

---

## §5 — All ASCII; the `<>` are annotation placeholders, NOT literal text

Unlike §17.1's `antiReuseProhibition` (which contains an em-dash U+2014), §17.3 is **entirely ASCII**.
No em-dash, no non-ASCII bytes. Do NOT introduce any.

The `<diff payload>`, `<rejected subject 1>`, `<rejected subject 2>` tokens in the PRD code block are
**structural annotations** (placeholders for dynamic content), exactly like §17.1's excluded
`(up to 20, ≤100 lines total)` annotation (commit-pi-origin.md §4). They are NOT literal output. The diff
is the runtime `diff` argument; the subjects are the runtime `rejected` slice elements. The literal text
is everything else.

---

## §6 — The diff is appended VERBATIM (no trailing-newline normalization)

`BuildUserPayload` does NOT trim, normalize, or alter the `diff`. It is a pure assembler. The diff's shape
(including whether it ends with `\n`) is `git.StagedDiff`'s contract (P1.M1.T3.S1):
- the markdown part ensures each file diff ends with `\n`;
- the non-markdown part is written as-is, and after a byte/line cap appends a `... [diff truncated at N bytes/lines]`
  sentinel WITHOUT a trailing newline.

So the payload's trailing bytes == the diff's trailing bytes. That is correct and intended — the diff is
"the stdin content for the provider executor" (work item OUTPUT) and the executor pipes whatever it
receives. Do not second-guess StagedDiff.

Defensive: if `diff == ""` (the orchestrator gates on `HasStagedChanges` first, so this is unreachable in
practice), the payload is just the instruction (+ optional rejection block) with an empty tail. Harmless.

---

## §7 — `len(rejected) == 0` ⇒ normal path; nil and empty slice are equivalent

The branch is `if len(rejected) == 0`. A `nil` slice and a zero-length `[]string{}` both have length 0, so
both take the normal (colon) path. This is the FIRST thing the dedupe loop (P1.M3.T2) influences: on the
initial generation attempt `rejected` is empty/nil ⇒ normal payload; on a duplicate-rejection retry
`rejected` is non-empty ⇒ the §17.3 rejection block is inserted. `BuildUserPayload` owns the assembly for
BOTH; it does NOT own the retry counter or the subject-extraction (those are P1.M3.T2 / FR30–FR33).

---

## §8 — Subjects are assumed single-line (FR30); NO sanitization

FR30 ("Extract the generated subject — the first line of the message") guarantees each `rejected` element
is a single line with no embedded newline. So `"- " + s + "\n"` always yields exactly one list line. We do
NOT `TrimSpace` or re-`strings.Split` the subjects — `BuildUserPayload` is a pure assembler that trusts the
upstream (FR30) contract, exactly as S1 trusts `RecentMessages`' trimming contract. Documenting the
assumption in the doc-comment is enough; sanitizing would obscure the §17.3 fidelity. (If P1.M3.T2 ever
ships a non-first-line subject, that is a P1.M3.T2 bug, not a payload bug.)

---

## §9 — Imports: `"strings"` ONLY (not even `"fmt"`)

`payload.go` needs `strings` (for `strings.Builder` in the rejection path). It does NOT need `fmt` — there
is no `%d`/`%v` interpolation here (unlike S1/S2's subject-target line). So the import block is a single
line: `import "strings"`. `internal/prompt` stays a stdlib-only LEAF (no `internal/config`, no
`internal/git`, no `internal/provider`, no third-party). `go mod tidy` MUST be a no-op;
`git diff --exit-code go.mod go.sum` MUST be empty.

Normal-path fast return uses plain `+` concatenation (no Builder) — readable and allocation-cheap for a
one-shot call. The rejection path uses `strings.Builder` because of the subject loop.

---

## §10 — Test strategy (mirror S1/S2)

`internal/prompt/payload_test.go`, `package prompt` (white-box — same as every `_test.go` in this repo),
imports `strings` + `testing`. Pure-function tests — NO subprocess, NO temp repo, NO git.

1. **`TestBuildUserPayload_NormalCanonicalExact`** — empty `rejected`, a known `diff`; assert the FULL
   string equals `userInstruction + "\n\n" + diff` byte-for-byte (i.e. the §17.3 normal rendering).
2. **`TestBuildUserPayload_RejectionCanonicalExact`** — two rejected subjects + a known diff; assert the
   FULL string equals the §17.3 rejection rendering byte-for-byte (independently derived).
3. **`TestBuildUserPayload_Properties`** — a table of structural invariants:
   - normal: instruction ends with `:`; rejection block ABSENT (`IMPORTANT:`, `REJECTED`, `Create an
     entirely` all absent); diff present verbatim.
   - rejection: instruction ends with `.` (NOT `:`); `IMPORTANT:` present; each subject present on its own
     `- `-prefixed line; `Create an entirely new message with different wording.` present; blank-line
     topology (the subject list is immediately followed by a blank line then the epilogue).
   - the diff is ALWAYS the exact tail (`strings.HasSuffix(out, diff)`), in BOTH paths.
4. **`TestBuildUserPayload_EdgeCases`** — `nil` rejected == `[]string{}` rejected == normal path; a single
   rejected subject yields exactly one `- ` line; `diff == ""` yields instruction (+ optional block) with
   empty tail (defensive, no panic).

Reuse the `near`/`suffix` failure helpers from `system_test.go` if useful (same package — they are
already defined; do NOT redeclare them).

---

## §11 — Downstream wiring (do NOT implement; just honor the string's role)

The orchestrator (P1.M3.T4 `CommitStaged`) wires it:

```go
payload := prompt.BuildUserPayload(diff, rejected)   // S3 — this subtask
spec, _ := manifest.Render(model, provider, sys, payload)  // P1.M2.T4 — payload is the 4th arg
out, _ := executor.Execute(ctx, spec)                // P1.M2.T5 — spec.Stdin for stdin delivery
msg := parse.ParseOutput(out)                        // P1.M2.T6
```

`render.go` line 100 (`case "stdin": spec.Stdin = payload`) confirms `BuildUserPayload`'s return value is
literally the stdin content for stdin-delivery providers. When `system_prompt_flag == ""`, `Render`
PREPENDS the system prompt to the payload (`sys + "\n\n" + payload`) — that is `Render`'s concern, not
`BuildUserPayload`'s. The `BuildUserPayload(diff string, rejected []string) string` signature is FROZEN
after S3.

The `rejected` slice originates in the dedupe loop (P1.M3.T2): `[]string{}` on the first attempt,
non-empty (the matched duplicate subjects) on retries up to `max_duplicate_retries` (default 3, FR32).
