# P1.M3.T1.S2 — New-repo conventional-commit fallback prompt: design decisions

The new-repo (≤1 commit) sibling of P1.M3.T1.S1. Read **S1's** `design-decisions.md` first — the
principles there (leaf package, verbatim canonical constants, constants carry NO trailing newline so the
builder owns every `\n`, in-package tests, go.mod unchanged, parameter-decoupled-from-config) all carry
over IDENTICALLY. This file records ONLY what is different or specific to the §17.2 fallback.

The authoritative source text is **PRD §17.2** (selected_prd_content `h3.61`), reproduced verbatim:

```
You are a commit message generator.

Output ONLY the commit message. No preamble, no markdown, no code fences.

Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.

Target ~50 characters (~7 words). Format: type(scope): description
```

The work-item contract (`item_description`) paraphrases this with "…" elisions and pins the signature
`BuildFallbackPrompt(subjectTarget int) string`. **Implement the §17.2 VERBATIM text, not the elided
paraphrase.** (Appendix A, PRD `h2.24`: "canonical strings to be committed verbatim".)

---

## §0. Scope — MODIFY two existing files (S1 created them; S2 appends a sibling)

Unlike S1 (which CREATED `internal/prompt/system.go` + `system_test.go`), S2 **edits** both: it APPENDS
one unexported string constant + one exported function to `system.go`, and APPENDS test functions to
`system_test.go`. **S1's content (the mature-repo path) must remain byte-for-byte intact** — do not
reformat, reorder, rename, or touch S1's constants/functions/comments. The diff against `main` is purely
additive at the end of each file.

- **OWNS:** `BuildFallbackPrompt(subjectTarget int) string` (PRD §9.3 FR14 / §17.2) + its body constant.
- **DOES NOT OWN:** the mature-repo prompt (S1, done), the user payload §17.3 (S3), the orchestrator
  branch `CommitCount ≤ 1` (P1.M3.T4), or `git.CommitCount` itself (P1.M1.T3.S3, done).

## §1. Signature — EXPORTED, string-only, no error (matches S1's `BuildSystemPrompt`)

```go
func BuildFallbackPrompt(subjectTarget int) string
```

- **Exported** because the caller is `internal/generate` (the orchestrator, P1.M3.T4) — cross-package.
- **Returns `string` ONLY** — there is no failure mode. (subjectTarget is a plain int; the body is a
  constant; `fmt.Sprintf` cannot fail.) No `(string, error)`.
- **Takes `subjectTarget int` as a PARAMETER** — decoupled from `config` (the orchestrator passes
  `cfg.SubjectTargetChars`); `system.go` does NOT import `internal/config`. Identical decoupling to S1's
  `BuildSystemPrompt(examples, hasMultiline, subjectTarget)`.

## §2. THE key decision — interpolate the CHAR count; keep "(~7 words)" FIXED

The §17.2 canonical string ends with `Target ~50 characters (~7 words). Format: type(scope): description`.
The work-item signature passes `subjectTarget int`, so the parameter must be USED (else it would be
parameterless). The question is what to interpolate.

**Decision:** interpolate ONLY the character count; "(~7 words)" stays verbatim.

```go
fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)
```

**Why (this is the load-bearing call — read it):**

1. **Exact analogy with S1.** S1's `subjectTargetLine` turns §17.1's `Target ~50 characters for the
   subject line.` into `fmt.Sprintf("Target ~%d characters for the subject line.", subjectTarget)` —
   the literal `50` becomes `%d`; everything else ("for the subject line.") is fixed canonical text. By
   the SAME rule, in §17.2's `Target ~50 characters (~7 words). Format: …` only the `50` becomes `%d`;
   "(~7 words)" and "Format: type(scope): description" are fixed canonical text.
2. **The default path is byte-canonical.** With `subjectTarget = 50` (the `Config.SubjectTargetChars`
   default, P1.M1.T4.S1) the output is EXACTLY PRD §17.2 — `Target ~50 characters (~7 words). Format:
   type(scope): description`. The exact-match canonical test pins this.
3. **Config-driven by design.** `SubjectTargetChars` is user-tunable; both prompt paths (mature §17.1
   AND new-repo §17.2) must honor it for consistency. The orchestrator passes the SAME
   `cfg.SubjectTargetChars` to `BuildSystemPrompt` and `BuildFallbackPrompt`.
4. **"(~7 words)" is a gloss, not a spec.** It is the PRD's own human-facing analogy (50 chars ≈ 7
   words). Scaling it (`~%d words` with `subjectTarget/7`) would (a) introduce an undocumented magic
   constant (÷7), (b) add integer-rounding ambiguity, and (c) deviate from the verbatim canonical
   string — strictly worse than a fixed gloss. The char count is the ACTUAL enforced target; the word
   count is illustrative.

**Rejected alternative:** hardcode `50` and ignore the parameter. Rejected because the work-item
signature deliberately takes `subjectTarget int`, and because it would make the new-repo path ignore
the user's configured `SubjectTargetChars` (inconsistent with the mature path).

## §3. Decomposition — one body constant + one inline `Sprintf`

§17.2 is fully static except the char count in the last line. The cleanest auditable decomposition that
honors S1's "constants carry NO trailing newline; the builder owns every `\n`" rule:

```go
// fallbackPromptBody is §17.2 MINUS the final target/format line. Verbatim from PRD §17.2, no trailing
// newline (BuildFallbackPrompt owns the inter-block "\n\n").
const fallbackPromptBody = `You are a commit message generator.

Output ONLY the commit message. No preamble, no markdown, no code fences.

Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.`

func BuildFallbackPrompt(subjectTarget int) string {
	return fallbackPromptBody + "\n\n" +
		fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)
}
```

`"\n\n"` between body and the target line yields exactly ONE blank line (body's last line
"…names." + `\n` ends it + `\n` blank + target). Matches §17.2's topology. Do NOT reuse S1's
`subjectTargetLine` — it emits the §17.1 wording ("…for the subject line."), which is WRONG for §17.2.

Naming: `fallbackPromptBody` (unexported, snake-free Go `camelCase`/`lowerCamel`) is consistent with
S1's `maturePromptHeader` / `antiReuseProhibition` naming style. No collision with any S1 identifier.

## §4. Verbatim source = PRD §17.2 (all ASCII — NO em-dash here)

Copy the text from PRD §17.2 (the `selected_prd_content` `h3.61` block), NOT the work-item paraphrase.
Unlike §17.1, §17.2 contains **no em-dash** and **no other non-ASCII bytes** — every char is ASCII
(the `~` is an ASCII tilde, U+007E). So there is no UTF-8 trap here (S1's em-dash gotcha does not
apply). Still copy character-for-character: "ESSENCE", "preamble", "code fences", "type(scope):
description", "(~7 words)".

## §5. How §17.2 DIFFERS from §17.1 (the anti-copy-paste guards — the tests assert these)

The #1 implementation risk is copy-pasting S1's mature constants into the fallback. §17.2 is a SMALLER,
SINGLE-LANE prompt (new repo ⇒ no history ⇒ no style examples ⇒ no anti-reuse ⇒ single-line subject).
Differences the tests must pin as ABSENT:

| §17.1 element (mature)            | In §17.2? | Why                                                  |
|-----------------------------------|-----------|------------------------------------------------------|
| "no quoting." clause              | ABSENT    | §17.2 output contract is shorter                     |
| "If a body is warranted …"        | ABSENT    | new repo ⇒ single-line subject only                  |
| "Match the tone and style …"      | ABSENT    | no examples to match                                 |
| `---` example markers             | ABSENT    | no examples                                          |
| `antiReuseProhibition` block      | ABSENT    | nothing to reuse (no examples shown)                 |
| multi-line rule (allow/single)    | ABSENT    | fixed single-line (conventional-commit `type(scope)`)|
| "for the subject line." wording   | ABSENT    | §17.2 uses "(~7 words). Format: type(scope): …"      |

And what §17.2 ADDS that §17.1 lacks: `Format: type(scope): description` (the conventional-commit
scaffold) and `(~7 words)`.

## §6. Placement — `internal/prompt/system.go` (per the work item); `fmt` already imported

The work item says "Add to internal/prompt/system.go". So the constant + function go IN `system.go`
(alongside S1's `BuildSystemPrompt`), NOT a new file. `system.go` already imports `"fmt"` and
`"strings"` (S1) — `BuildFallbackPrompt` uses `fmt` only; **NO new import, NO new file, NO go.mod
change.** `go mod tidy` is a no-op; `git diff --exit-code go.mod go.sum` is empty.

## §7. Leaf package, no config import, no new deps (unchanged from S1)

`internal/prompt` imports ONLY stdlib (`fmt`, `strings`). It does NOT import `internal/config`,
`internal/git`, `internal/provider`, `os/exec`, or any third-party. The `subjectTarget` PARAMETER is the
decoupling seam (same as S1). This keeps `internal/prompt` a leaf in the import graph — no cycle.

## §8. Orchestrator wiring (DOWNSTREAM — do NOT implement here)

P1.M3.T4 (`CommitStaged`) owns the branch. `git.CommitCount` (P1.M1.T3.S3) returns `(int, error)` — `0`
for an unborn repo (git exit 128 mapped to `0, nil`), else the parsed count. The orchestrator:

```go
count, err := g.CommitCount(ctx)              // P1.M1.T3.S3
// … err handling …
var sys string
if count <= 1 {                               // FR14: ≤1 commit → §17.2 fallback
	sys = prompt.BuildFallbackPrompt(cfg.SubjectTargetChars)
} else {                                      // FR10–FR13: >1 commit → §17.1 mature
	recent := /* g.RecentMessages(ctx, 20) */
	sys = prompt.BuildSystemPrompt(recent, prompt.DetectMultiline(recent), cfg.SubjectTargetChars)
}
```

S2 ships ONLY `BuildFallbackPrompt`. The `if count <= 1` is the orchestrator's job. (Note: FR14 says
"≤1 commit"; `CommitCount` returns `0` for unborn and `1` for a single root commit — both route to the
fallback. That boundary is the orchestrator's concern, not the prompt builder's.)

## §9. Test strategy — exact-match canonical + properties (incl. §17.1-absence guards) + interpolation

Mirror S1's `system_test.go` style (in-package `package prompt`, `strings` + `testing`, table-driven,
diff-friendly `%q` failure output). APPEND to the existing `system_test.go` (do not touch S1's tests):

1. **`TestBuildFallbackPrompt_CanonicalExact`** — one exact-match case, `subjectTarget = 50`, pinning
   the FULL §17.2 string byte-for-byte. Independently derived from PRD §17.2 (not from the impl).
2. **`TestBuildFallbackPrompt_Properties`** — a table of structural checks:
   - role present; §17.2 output contract present; essence present; `Format: type(scope): description`
     present; `(~7 words)` present.
   - **§17.1 elements ABSENT** (the anti-copy-paste guards from §5): "no quoting", "If a body is
     warranted", "Match the tone and style", "---", "CRITICAL: You MUST NOT copy",
     "for the subject line", "multi-line".
   - blank-line topology: exactly the body + ONE blank line + the target line; no trailing newline.
3. **`TestBuildFallbackPrompt_SubjectTargetInterpolated`** — `subjectTarget = 72` ⇒ contains
   `Target ~72 characters (~7 words). Format: type(scope): description`; `~50 characters` ABSENT;
   `(~7 words)` STILL present (the fixed gloss survives a non-default target — pins §2).

No subprocess, no temp repo, no git — pure-function tests (same as S1's prompt tests).

## §10. Frozen files (do NOT edit)

Everything EXCEPT the two additive edits to `internal/prompt/system.go` and
`internal/prompt/system_test.go`: `internal/provider/*`, `internal/config/*`, `internal/git/*`,
`cmd/stagecoach/main.go`, `pkg/*`, `Makefile`, `go.mod`, `go.sum`, and S1's existing symbols/comments
within the two prompt files. The diff is purely append-at-end-of-file for both.
