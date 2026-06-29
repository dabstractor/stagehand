# Research — commit-pi origin (the script stagehand's prompt is ported from)

> **Source:** `/home/dustin/projects/git-scripts/commit-pi` (9.5 KB zsh script, read in full 2026-06-29).
> PRD §2.1 names commit-pi the "originating tool"; PRD §17.1 says the system prompt was "ported and
> refined from commit-pi"; PRD Appendix A says the canonical strings are "committed verbatim to
> internal/prompt/system.go". This brief is the ground-truth diff between commit-pi's ORIGINAL prompt
> and PRD §17.1's REFINED version, so the implementer ports the RIGHT text (PRD wins on conflict).

## 1. The mature-repo prompt commit-pi ACTUALLY shipped

commit-pi builds the prompt in zsh (the `commit_count -gt 1` branch). Verbatim, with `$json_instruction`
and `$examples` and `$multiline_rule` shown expanded:

```
You are a commit message generator.
Return valid JSON only. No markdown, no XML.
Format: {"commit_message": "<message>"}
For multiline messages, use literal \n for newlines (e.g., "subject\n\nbody").
IMPORTANT: Do NOT use double quotes (") inside the message text. Use single quotes instead if needed.

Focus on the ESSENCE of the change (the intent/purpose), not specific implementation details like filenames or function names.

Match the tone and style of these recent commits:
---
<body 1>
---
<body 2>
...

CRITICAL: You MUST NOT copy or reuse ANY phrasing from the examples above. The examples show the STYLE to match - the format, tone, and conventions. But producing the same text you have seen is STRICTLY FORBIDDEN. Your output must be completely original wording that describes THIS specific change. Reusing example text is a critical failure.
<rule>
Target ~50 characters for the subject line.
```

Where `<rule>` is one of:
- (has_multiline) `"Only add a body (blank line + description) if the history shows multiline commits AND these changes truly warrant detailed explanation. Otherwise, use a single-line subject only."`
- (else)          `"Only output a single-line subject (no body)."`

## 2. The EXACT awk multi-line heuristic (the provenance of `DetectMultiline`)

```zsh
examples=$(git log --format="---%n%B" -20 | sed '/^$/d' | head -100)
has_multiline=$(echo "$examples" | awk '/^---$/{if(lines>1)found=1; lines=0; next} {lines++} END{print found+0}')
```

How it works:
- `git log --format="---%n%B" -20` → emits `---\n<body>\n---\n<body>…` (newest-first), one `---` per commit.
- `sed '/^$/d'` → strips BLANK lines (truly-empty `^$` only; whitespace-only lines are kept).
- `head -100` → cap at 100 lines (matches PRD FR11 / `RecentMessages` maxRecentMessageLines=100).
- The awk:
  - On a `^---$` separator: **if the PREVIOUS commit accumulated `lines > 1`, set `found=1`**; reset `lines=0`; continue.
  - On any other line: `lines++`.
  - `END { print found+0 }` → prints `1` if ANY commit had >1 (non-blank) line, else `0`.

**Semantics: `has_multiline == 1` ⇔ at least one commit message has more than one NON-EMPTY line** (the
`sed '/^$/d'` strips only truly-empty lines `^$`; a whitespace-only line SURVIVES and is counted).

### Faithful Go port

`RecentMessages` (P1.M1.T3.S3) returns the SAME data already split + trimmed (NUL-delimited, each element
a trimmed full message, capped at 100 lines keeping complete messages). So the `---`/`sed`/`head` plumbing
is already done; `DetectMultiline` only needs the per-message ">1 non-empty line" check:

```go
// DetectMultiline is a faithful port of commit-pi's awk heuristic:
//   awk '/^---$/{if(lines>1)found=1; lines=0; next} {lines++} END{print found+0}'
// over `git log --format="---%n%B" -20 | sed '/^$/d' | head -100`.
// RecentMessages has already split on the NUL delimiter, trimmed each message, and capped at
// 100 lines keeping complete messages — so DetectMultiline only needs the per-message
// ">1 NON-EMPTY line" test the awk's `lines` counter implements.
func DetectMultiline(examples []string) bool {
	for _, msg := range examples {
		if countNonEmptyLines(msg) > 1 {
			return true
		}
	}
	return false
}

// countNonEmptyLines mirrors commit-pi's `sed '/^$/d'` EXACTLY: a truly-empty line is dropped,
// a whitespace-only line SURVIVES and is counted (sed's /^$/ matches only the empty string).
func countNonEmptyLines(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		if line != "" { // exact sed '/^$/d' mirror — NOT strings.TrimSpace (that would also drop "   ")
			n++
		}
	}
	return n
}
```

> **Why non-empty-line-count and NOT `strings.Contains(msg, "\n")` (or `TrimSpace != ""`):** all three
> agree for every realistic git message (RecentMessages trims, so a single-line subject has no `\n` and
> a `subject\n\nbody` has one). But the awk runs over `sed '/^$/d'` output, which strips only truly-empty
> lines — a whitespace-only line SURVIVES and is counted. `countNonEmptyLines` (`line != ""`) mirrors
> that EXACTLY; `strings.TrimSpace(line) != ""` would NOT (it drops whitespace-only lines the awk keeps).
> The boolean result is identical on RecentMessages' trimmed output regardless (a whitespace-only body
> line is always accompanied by real content that already makes the count >1), but the `line != ""` form
> is the literally-faithful port — prefer it so the provenance claim is unimpeachable.

## 3. The diff commit-pi → PRD §17.1 (what "refined" changed)

PRD §17.1 is AUTHORITATIVE on conflict. Port the PRD text, not commit-pi's.

| Piece | commit-pi (original) | PRD §17.1 (refined — USE THIS) | Why |
|---|---|---|---|
| **Output contract** | JSON instruction (`Return valid JSON…`, "no double quotes inside the message") | **Raw-output contract** (§17.4 design call): `Output ONLY the commit message. No preamble, no markdown, no code fences, no quoting. If a body is warranted, use a blank line between subject and body.` | PRD §17.4: raw output removes the double-quote constraint + fragile `sed` parse. JSON stays a per-provider option (ParseOutput), NOT the prompt contract. |
| **Essence line** | `…not specific implementation details like filenames or function names.` | `…not implementation details like filenames or function names.` (dropped "specific") | PRD wording; cosmetic. Use PRD. |
| **Examples intro** | `Match the tone and style of these recent commits:` | `Match the tone and style of these recent commits from this repository:` (+ "from this repository") | PRD wording. Use PRD. |
| **Anti-reuse: dash** | `the STYLE to match - the format` (ASCII hyphen `-`) | `the STYLE to match — format` (**em-dash `—` U+2014**) | PRD refined to em-dash. **Use the em-dash VERBATIM** (gotcha). |
| **Anti-reuse: wording** | `…completely original wording that describes…` / `format, tone, and conventions` | `…entirely original wording describing…` / `format, tone, length, conventions` (+ "length") | PRD wording. Use PRD. |
| **Examples block** | `---\n<body>` per commit (from `--format="---%n%B"`) | `---\n<commit N full message>` per commit | IDENTICAL structure — `---` before EACH message. ✅ |
| **`(up to 20, ≤100 lines total)`** | **NOT present** (commit-pi interpolates `$examples` directly) | Appears in §17.1's structural block | See §4 — it is an annotation, NOT literal text. EXCLUDE. |
| **Multi-line rules** | identical wording to PRD | identical wording to PRD | IDENTICAL (only "multi-line" hyphenation differs cosmetically). ✅ |
| **Subject target** | `Target ~50 characters for the subject line.` | `Target ~50 characters for the subject line.` | IDENTICAL. Parameterized as `~%d` via `subjectTarget`. ✅ |

## 4. The `(up to 20, ≤100 lines total)` annotation — EXCLUDE from the runtime prompt

PRD §17.1's code block contains the line:
```
...
(up to 20, ≤100 lines total)

CRITICAL: ...
```

**This is NOT literal prompt text.** Evidence:
1. **commit-pi does not emit it.** commit-pi interpolates `$examples` directly (`Match the tone and style of these recent commits:\n$examples\n\nCRITICAL:…`) with no such annotation. The PRD is a faithful generalization of commit-pi, so a line commit-pi never had is structural, not literal.
2. **Appendix A says** the canonical strings are committed verbatim "with the diff/examples/rejection-list interpolated at runtime." The `(up to 20, ≤100 lines total)` describes the INTERPOLATION (how many examples, how long), not the message to the model. It is metadata about `$examples`, just like `<commit 1 full message>` is a placeholder.
3. **The constraints are already enforced upstream:** `RecentMessages(ctx, 20)` (the caller, P1.M3.T4) bounds the count to 20 and `maxRecentMessageLines=100` (P1.M1.T3.S3) bounds the lines. Telling the model about limits it cannot observe is noise.

**Decision:** the runtime prompt's examples block is ONLY the `---`-separated messages. Do not emit `(up to 20, ≤100 lines total)`. (If a reviewer disagrees, the fix is one `b.WriteString("…")` line — but the default is exclude, matching the origin script.)

## 5. The exact blank-line topology of the assembled prompt

Derived from commit-pi's zsh string concatenation (blank lines between every block) and confirmed by
PRD §17.1's code-block formatting. `BuildSystemPrompt` must reproduce this exactly:

```
You are a commit message generator.
<blank>
Output ONLY the commit message. No preamble, no markdown, no code fences,
no quoting. If a body is warranted, use a blank line between subject and body.
<blank>
Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.
<blank>
Match the tone and style of these recent commits from this repository:
---
<msg1>
---
<msg2>
<blank>
CRITICAL: You MUST NOT copy or reuse ANY phrasing from the examples above.
They show the STYLE to match — format, tone, length, conventions. Producing
the same text you have seen is STRICTLY FORBIDDEN. Your output must be
entirely original wording describing THIS specific change. Reusing example
text is a critical failure.
<blank>
<multi-line rule>
Target ~50 characters for the subject line.
```

Notes:
- **No blank line between the last example and the first `---`?** Correct — `---` follows the intro line directly, and each `---` precedes its message directly (no blank).
- **Blank line after the LAST example, before CRITICAL.** Yes (commit-pi: `…$examples\n\nCRITICAL`).
- **No blank line between the multi-line rule and the Target line.** Correct (consecutive lines).
- The em-dash (`—`) in the anti-reuse block is the ONLY non-ASCII byte in the whole prompt.

## 6. What commit-pi's prompt did NOT have (and neither does stagehand's)

- No JSON contract in stagehand (raw output — §17.4).
- No `(up to 20…)` annotation (§4 above).
- No example COUNT or LIMIT text — the examples block is pure `---`-separated messages.
- The "new repo" (≤1 commit) prompt (§17.2) is a SEPARATE subtask (P1.M3.T1.S2) — not this file.
- The user payload (§17.3, "Generate a commit message for these changes:" + diff) is a SEPARATE subtask
  (P1.M3.T1.S3) — not this file.
