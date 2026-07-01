# P3.M1.T1.S3 — Empirical Findings: arbiter system prompt + JSON contract

The arbiter is the **third and final** decomposition prompt (planner §17.5 = S1 ✅; stager §17.6 = S2
parallel; arbiter §17.7 ← THIS). It is a **bare** agent that runs only if the working tree is non-empty
after the loop, receives the commits made this run (SHA + subject + file-list each) + a leftover diff, and
returns `{"target": "<sha>"}` or `{"target": null}`. This document is the authoritative evidence base for
PRP.md; every section number is referenced from there.

---

## §1 — The VERBATIM §17.7 system prompt (the `arbiterSystemPrompt` constant)

Captured character-for-character from `PRD.md` lines 1580–1592 (the §17.7 fenced code block, marked
"(sketch)" but treated as authoritative — same precedent as §17.1/§17.2/§17.5 per S1/S2):

```
You reconcile leftover changes into commits that were just made. You are given the commits
created this run (with their messages and changed files) and a diff of changes that were not
included in any of them.

Decide: do these leftovers logically belong WITH one of those commits, or do they warrant a
NEW commit?
- Choose an existing commit only if the leftovers are part of the SAME logical change.
- When in doubt, prefer a NEW commit (return null) — never force a fit.
- You may only target a commit from the provided list.

Respond with ONLY JSON: {"target": "<sha from the list>"} or {"target": null}.
```

**Literal-vs-placeholder distinction (the #1 text-fidelity trap):** the JSON-contract line
`Respond with ONLY JSON: {"target": "<sha from the list>"} or {"target": null}.` is LITERAL instructive
text shown to the model — the `<sha from the list>` token STAYS in the constant VERBATIM (it is NOT a
runtime placeholder). There is NO `<style examples>` runtime placeholder in §17.7 (unlike §17.5 which
appends style examples). So `arbiterSystemPrompt` is the ENTIRE §17.7 fenced block, start-to-finish.

**The constant ENDS at:** `Respond with ONLY JSON: {"target": "<sha from the list>"} or {"target": null}.`
with **NO trailing newline** (the load-bearing prompt-package convention — see §7).

### §1.1 — The EM-DASH (the ONE non-ASCII byte)

Line 7 of §17.7 — `When in doubt, prefer a NEW commit (return null) — never force a fit.` — contains an
**em-dash "—" (U+2014)** between "(return null)" and "never force a fit". Confirmed via
`grep -n "never force a fit" PRD.md` → line 1586. This is the ONE non-ASCII byte in §17.7 (same byte as
§17.1's antiReuseProhibition and §17.6's stagerGuardrails). It **MUST** be preserved verbatim — never
replaced with an ASCII hyphen "-". Document it in the const's doc comment (mirror system.go's / S2's
"NOTE the EM-DASH" annotation).

### §1.2 — NO backticks in §17.7 ⇒ a backtick RAW STRING LITERAL works

§17.7's text contains double quotes (the JSON contract line: `"target"`) and angle brackets (`<sha from
the list>`) but **NO backtick characters**. Therefore `arbiterSystemPrompt` can be a backtick raw string
literal `` `...` `` (no escaping of the double quotes needed) — EXACTLY like planner.go's
`plannerSystemPrompt`. (Contrast S2's `stagerGuardrails`, which HAS backticks and MUST use a
double-quoted literal.) The em-dash is a UTF-8 byte literal inside the raw string — fine, no escaping.

### §1.3 — `BuildArbiterSystemPrompt()` takes NO arguments (KEY difference from the planner)

Because §17.7 has NO `<style examples>` placeholder, there is nothing to append at runtime. So
`BuildArbiterSystemPrompt() string` (zero args) just returns the verbatim `arbiterSystemPrompt` constant.
This is the opposite of `BuildPlannerSystemPrompt(examples []string)`. The thin function wrapper still
exists for API symmetry with the planner/stager Build* family AND to keep the constant private (so the
no-trailing-newline invariant is enforced in one place — see §7).

---

## §2 — The user payload format (the ONE ambiguous point — a design decision)

§17.7 says only: *"User payload: the commit list + the leftover diff."* §13.6.5 elaborates: *"the SHAs,
messages, and file-lists (`diff-tree`) of every commit made this run, plus a diff of the remaining
changes (with binary placeholders)."* **Neither specifies the exact assembly format.** This is the load-
bearing design decision (analogous to S1's forced-count prepend topology). DECISION, pinned by a
canonical-exact test:

```
The commits made this run (message and changed files for each):

<sha1>
<subject1>
<file1>
<file2>

<sha2>
<subject2>
<file3>

A diff of changes that were not included in any commit:

<leftoverDiff verbatim>
```

Topology, precisely:
- **Section 1 header** = `arbiterCommitsHeader` const (`"The commits made this run (message and changed
  files for each):"`, trailing COLON, NO trailing newline) — phrasing mirrors §17.7's own wording
  ("commits created this run (with their messages and changed files)").
- ONE blank line, then **one block per commit**. Each block: `<sha>\n<subject>\n` then each file on its
  own line `<file>\n`, then ONE blank line separating it from the next block.
- **Section 2 header** = `arbiterLeftoverHeader` const (`"A diff of changes that were not included in any
  commit:"`, trailing COLON, NO trailing newline).
- ONE blank line, then `leftoverDiff` appended **VERBATIM** as the tail (mirrors payload.go / planner.go's
  "diff is the exact tail" convention — no normalization, no added trailing newline).

**NOTE: these two headers are NOT verbatim PRD text** (§17.7 does not give them) — they are a clean,
auditable assembly choice. They are named private constants so the topology lives in one place. Rationale:
clear, distinct section headers help the model (a) identify the SHA to return and (b) understand the
leftover diff's role; they are unlikely to collide with diff content.

**Defensive edge cases (no panic):**
- Empty/nil `commits` ⇒ the loop body never runs; output is `arbiterCommitsHeader + "\n\n" +
  arbiterLeftoverHeader + "\n\n" + leftoverDiff` (semantically odd but impossible in practice — the
  arbiter only runs after ≥1 commit). Acceptable.
- A commit with empty `Files` ⇒ block is `<sha>\n<subject>\n` + the blank separator (no file lines).
- Empty `leftoverDiff` ⇒ headers with nothing after (acceptable; impossible in practice — the arbiter only
  runs when the working tree is non-empty).

---

## §3 — The types: `ArbiterCommit` + `ArbiterOutput` (the *string null semantics)

### §3.1 — `ArbiterCommit` (input to BuildArbiterUserPayload)

Per the work item ("SHA+subject+file-list each") + §13.6.5 ("SHAs, messages, and file-lists"):

```go
// ArbiterCommit is one commit made this run, as shown to the arbiter (§13.6.5: SHA + message + file-list).
type ArbiterCommit struct {
    SHA     string   // the commit's full SHA (40/64 hex) — the value the arbiter may return as "target".
    Subject string   // the commit's subject line (one line; the "messages" of §13.6.5).
    Files   []string // the file-list (diff-tree name-only) for this commit; may be empty.
}
```

Field naming: `SHA` (Go initialism-caps convention — the codebase uses `sha` for vars and `SHA` in
comments/structs; `SHA` is idiomatic). `Subject` matches the work item's "subject" wording (§13.6.5 calls
them "messages"; the consumer decompose/arbiter.go decides whether to pass the subject or full message —
the field is a plain string either way). `Files []string` — one path per element (from `diff-tree
--name-only`).

### §3.2 — `ArbiterOutput` (the JSON contract — Target *string)

The §17.7 contract is `{"target": "<sha from the list>"}` OR `{"target": null}`. This is a **tri-state
need expressed as a two-state field** — the canonical Go solution is a **`*string` pointer**:

```go
// ArbiterOutput is the arbiter's JSON response (§17.7). A nil Target means {"target": null} → new commit;
// a non-nil Target points at the SHA to amend. The caller (decompose/arbiter.go) validates Target is in
// the provided commit list and non-empty; ParseArbiterOutput does NOT validate (it only parses).
type ArbiterOutput struct {
    Target *string `json:"target"` // nil ⇔ null ⇔ new commit; &"<sha>" ⇔ amend that commit.
}
```

**`encoding/json` semantics for `*string` (verified):**
| Model output JSON              | `out.Target` after Unmarshal | Meaning |
|--------------------------------|------------------------------|---------|
| `{"target": null}`             | `nil`                        | new commit (the "when in doubt" default) |
| `{"target": "abc1234"}`        | `&"abc1234"`                 | amend commit abc1234 |
| `{}` (field absent)            | `nil`                        | treated as new commit (nil, same as null — the safe default) |
| `{"target": ""}`               | `&""` (non-nil, empty)       | degenerate — caller rejects (empty ⇒ not in list ⇒ new commit) |
| `{"target": 123}`              | **Unmarshal ERROR**          | number into *string is a type error ⇒ ParseArbiterOutput returns error |

So **nil Target ⇔ "new commit"** is the safe, correct default for both `null` and missing-field. The
caller (decompose/arbiter.go, P3.M3.T1.S1) owns: (a) rejecting empty/non-list SHAs → default to null
(§13.6.5 "Ambiguous → default to null"), and (b) the target-in-list validation. ParseArbiterOutput only
parses — it does NOT have the list, so it CANNOT validate (same principle as ParsePlannerOutput not
validating single⇔message). A non-string target (number/bool/object/array) is a JSON type error and yields
a non-nil error — correct, since it violates the §17.7 contract.

---

## §4 — `ParseArbiterOutput` algorithm (REUSE planner.go's `extractJSONObject` — CRITICAL)

The arbiter's output is structured JSON (a single `target` field), so the same two real model failure
modes apply as the planner: JSON wrapped in prose ("Here is the decision: {...}"), and JSON wrapped in a
code fence despite "no code fences" (```json\n{...}\n```). So `ParseArbiterOutput` mirrors
`ParsePlannerOutput` exactly:

```
1. s := strings.TrimSpace(raw)
2. Attempt 1: json.Unmarshal([]byte(s), &out)  → if nil error, return (out, nil)
3. Attempt 2: sub, found := extractJSONObject(s); if found { json.Unmarshal([]byte(sub), &out) → success or error }
4. else return (ArbiterOutput{}, fmt.Errorf("arbiter output: not valid JSON: %w", err1))
```

### §4.1 — CRITICAL: `extractJSONObject` ALREADY EXISTS in package `prompt` — DO NOT REDECLARE

**This is the #1 implementation trap for THIS task.** planner.go (S1, COMPLETE — verified at
`internal/prompt/planner.go:161`) already defines a package-level unexported:
```go
func extractJSONObject(s string) (string, bool) { /* ~30-line brace-balanced state machine */ }
```
Because planner.go and arbiter.go are in the **same package** (`prompt`), an unexported function is shared
across the package. If arbiter.go defines its OWN `extractJSONObject`, the Go compiler errors:
**"extractJSONObject redeclared in this package"** (duplicate-symbol compile failure).

**THE RULE: arbiter.go must NOT define `extractJSONObject`. It CALLS the existing planner.go copy
directly** (same package ⇒ unexported is accessible from arbiter.go). This is the cleanest correct move:
zero code duplication, zero new private function, no redeclaration. (This differs from S1's situation: S1
created the FIRST copy because no prompt-package copy existed yet. S3 reuses that copy.)

Sanity: the planner.go extractJSONObject is byte-identical to provider/parse.go's algorithm (per S1's
findings), so its behavior is exactly what ParseArbiterOutput needs. No behavior difference. arbiter.go's
test file can also reuse planner_test.go's `TestExtractJSONObject` coverage implicitly (it tests the same
shared function) — but add an arbiter-specific parse table anyway (§8).

### §4.2 — ParseArbiterOutput returns a NON-NIL error on failure

Do NOT swallow parse failures (a zero ArbiterOutput with nil error would hide the problem from the caller).
Return `(ArbiterOutput{}, fmt.Errorf("arbiter output: not valid JSON: %w", err))` so decompose/arbiter.go
can trigger its retry / fallback. The retry/ambiguous-default is the CALLER's job (P3.M3.T1.S1,
§13.6.5 "Ambiguous → null"); ParseArbiterOutput only parses.

---

## §5 — The exports + the consumer contract

The work item specifies EXACTLY three exports + two types:
- `func BuildArbiterSystemPrompt() string` — returns `arbiterSystemPrompt` (§1.3; zero args).
- `func BuildArbiterUserPayload(commits []ArbiterCommit, leftoverDiff string) string` — §2's assembly.
- `func ParseArbiterOutput(raw string) (ArbiterOutput, error)` — §4's two-attempt parse.
- `type ArbiterCommit struct{...}` (§3.1) + `type ArbiterOutput struct{...}` (§3.2).

**Consumer:** `internal/decompose/arbiter.go` (P3.M3.T1.S1, planned). Confirmed by
`architecture/decompose_architecture.md` — the Four Agent Roles table lists the arbiter as
`JSON {target: "<sha>"|null}` reusing `prompt/arbiter.go`, and the "Arbiter Resolution" section confirms
the consumer drives: null→new commit, target==HEAD→tip amend, target==mid-chain→rebuild,
ambiguous→null. THIS task provides ONLY the prompt-construction + JSON-parse layer; the git mechanics are
P3.M3.T1.S1/T2.

---

## §6 — NO `ArbiterRetryInstruction` (do NOT invent one)

§17.5 defines an explicit retry instruction ("Respond with ONLY the JSON object described, no other
text.") and S1 exported it as `PlannerRetryInstruction`. **§17.7 defines NO retry instruction.** The work
item's export list (§5) does NOT include one. Therefore arbiter.go does NOT define an
`ArbiterRetryInstruction` (anti-pattern: do not invent what §17.7 / the work item don't specify). If the
caller (P3.M3.T1.S1) wants a generic retry, it can pass a plain string; this layer is not its home. Note
this asymmetry explicitly so the implementer doesn't copy-paste S1's retry-const pattern.

---

## §7 — The prompt-package conventions to mirror (unchanged from S1/S2)

Confirmed by reading `internal/prompt/system.go`, `payload.go`, `planner.go`:
1. **No trailing newline on ANY string constant.** The Build* functions own ALL inter-block newline
   placement. The §17.7 blank-line topology must live in exactly one auditable place (the Build function).
   This is load-bearing.
2. **Build-owns-newlines + rich doc comments** citing the PRD section, diagramming the assembly topology,
   and noting gotchas (the em-dash; the *string null semantics; the no-style-examples difference).
3. **Minimal decoupled params** — BuildArbiterUserPayload takes `[]ArbiterCommit` + a diff string (no git,
   no config; mirrors BuildUserPayload/BuildPlannerUserPayload taking a diff string).
4. **Backtick raw string for `arbiterSystemPrompt`** (§1.2 — no backticks in §17.7; the JSON double-quotes
   need no escaping in a raw string; the em-dash is a UTF-8 byte literal).
5. **The diff is the verbatim tail** — leftoverDiff is appended with no normalization (payload.go
   precedent).
6. **Imports:** `encoding/json` + `fmt` + `strings` (same as planner.go). NO import of provider (the
   prompt package is a zero-internal-dep leaf; extractJSONObject is reused from planner.go in-package).

---

## §8 — Test conventions (mirror planner_test.go / system_test.go)

Confirmed by reading `internal/prompt/planner_test.go`, `system_test.go`:
1. **Package `prompt` (internal test)** — unexported `arbiterSystemPrompt`/`arbiterCommitsHeader`/
   `arbiterLeftoverHeader` ARE visible; the shared `extractJSONObject` is reused.
2. **REUSE `near()`/`suffix()`** from `system_test.go` (bottom of file, lines 328/336). Do NOT redeclare
   (duplicate-symbol compile error — same trap as §4.1 but for test helpers).
3. **Canonical-exact tests** — independently-derived `want` (built from §17.7 + §2's design, NOT from the
   impl), asserted with a `%q` diff on failure. Three: `BuildArbiterSystemPrompt_CanonicalExact`,
   `BuildArbiterUserPayload_CanonicalExact` (multiple commits with files).
4. **Properties table** — anti-copy-paste guards (§17.1 mature elements ABSENT: "You are a commit message
   generator", "CRITICAL: You MUST NOT copy", "Target ~"; §17.5 planner elements ABSENT: "You are a
   commit-planning assistant", "Respond with ONLY JSON, no prose, no code fences", "Decompose these
   un-staged changes"; §17.6 stager elements ABSENT: "Stage, but do NOT commit", "`git add <path>`") +
   §17.7 PRESENT guards (the em-dash, the JSON-contract line, "prefer a NEW commit (return null)").
5. **ParseArbiterOutput table** — `{"target": null}`→Target nil; `{"target":"abc"}`→Target &"abc"; JSON
   in prose (fallback); JSON in code fence (fallback); leading/trailing whitespace trimmed;
   `{"target":123}`→error (non-string); extra fields ignored; malformed→error; empty→error; unbalanced
   braces→error; **round-trip** (Marshal an ArbiterOutput with nil Target and with &"sha" → parse back).
6. **`TestArbiterOutput_NullSemantics`** — the *string tri-state: nil Target from `null`, non-nil from a
   SHA, and that `len==0 && Target==nil` distinguishes "new commit". (The strongest guard on the §3.2
   design.)

---

## §9 — Scope boundaries (frozen / owned elsewhere — do NOT edit)

- `internal/prompt/planner.go` (S1, complete) — **REUSED** for `extractJSONObject` (§4.1). UNCHANGED.
- `internal/prompt/stager.go` (S2, parallel) — separate standalone new file; ZERO merge friction.
- `internal/prompt/system.go`, `payload.go`, `*_test.go` — UNCHANGED (REUSED for pattern + helpers).
- `internal/provider/parse.go` — UNCHANGED (the algorithm is already mirrored in planner.go).
- `internal/decompose/*` — does NOT exist yet; THIS task does NOT create it (P3.M2.*/P3.M3.*).
- `go.mod` / `go.sum` — UNCHANGED (stdlib only: encoding/json + fmt + strings).
- Sibling prompt files: arbiter.go is a NEW standalone file → zero merge friction with S1/S2.

**DELIVERABLES (2 new files, 0 modifications):** `internal/prompt/arbiter.go` +
`internal/prompt/arbiter_test.go`. Purely additive — no existing caller or test breaks.
