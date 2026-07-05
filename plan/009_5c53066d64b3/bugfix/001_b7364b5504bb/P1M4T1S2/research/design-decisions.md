# P1.M4.T1.S2 — Design Decisions

Update the README.md Features-table "Multi-turn fallback" row (line 68) for multi-turn path coverage.
This is a CONDITIONAL task (item_description §3): review the row, decide whether it implies commit-path-
only, and EITHER leave it unchanged (documenting why) OR make a minimal update. The contract leans toward
"no change may be needed."

---

## §0 — The row already says "stagehand" generically ⇒ the contract's no-change criterion is met

The verbatim row (README.md:68):
```
| Multi-turn fallback | Lossless multi-turn fallback: when a one-shot generation of a large diff fails,
  stagehand re-delivers the full diff across session turns so the message still lands — no truncation,
  no extra commits ([how it works](docs/how-it-works.md#multi-turn-generation-fallback) · [knobs](docs/configuration.md#built-in-defaults)). |
```

The contract's decision criterion (§3): *"If the existing wording is already generic enough to cover all
paths (it says 'stagehand' generically, not 'the commit path'), no change may be needed."* Applying it:

| Phrase | Path-specific? | Verdict |
|---|---|---|
| "stagehand re-delivers the full diff" | NO — "stagehand" (the tool), not "the commit path" | generic ✓ |
| "a one-shot generation of a large diff fails" | NO — any generation path (commit / dry-run / hook) | generic ✓ |
| "across session turns" | NO | generic ✓ |
| "so the message still lands" | mild commit flavor ("lands") but defensible ("is produced" — dry-run prints it, hook writes it, commit commits it) | generic-ish ✓ |
| "no extra commits" | commit-FLAVORED wording, but a UNIVERSALLY-TRUE anti-misconception note (multi-turn yields ONE message/commit delivered across turns, NOT N commits) — accurate on dry-run (zero commits) and hook (one via git) too | accurate, not a scoping claim ✓ |

The criterion's literal test — "it says 'stagehand' generically, not 'the commit path'" — is SATISFIED. The
row never says "on the commit path", "when committing", "snapshot", "commit-tree", or "HEAD". "no extra
commits" is a clarification (defending against the "multi-turn = multiple commits" misconception), not a
scoping statement. **⇒ No row-text change is required.**

## §1 — §5c (the authoritative docs-sync analysis) does NOT flag the README row

`docs/architecture/research_config_provider_docs.md` §5c ("Docs-sync implications") enumerates the sync
needs after Issue 3/4. It lists: how-it-works.md (the `--verbose` per-chunk note + the FR-T12 paragraph
verify — both S1's edits) and configuration.md (re-check the defaults/limitation blockquote). **It does
NOT mention README.md at all.** The original auditor — who documented the row verbatim in §5b — did not
consider it stale. This independently corroborates §0: the row was already accurate/generic.

## §2 — The decisive structural insight: the README is the high-level POINTER; how-it-works.md carries the path detail

The contract: *"Keep changes minimal — the README is a high-level overview, not a path-by-path reference."*
The row links to `docs/how-it-works.md#multi-turn-generation-fallback`. P1.M4.T1.S1 (the parallel sibling,
the contract) ADDS a one-sentence "Multi-turn runs on every generation path — the snapshot commit flow,
`--dry-run`, and hook mode" note to THAT section (S1 edit (d)) + a hook-mode multi-turn note (S1 edit (a)).

So the path-by-path coverage is carried by the LINKED how-it-works.md section (post-S1), NOT by the README
row. The row's job is to point there. Duplicating the path enumeration in the README row would (a) violate
"not a path-by-path reference" and (b) create a second site to keep in sync. **⇒ The row should stay
high-level/generic; the path detail belongs in how-it-works.md (where S1 is putting it).**

## §3 — PRIMARY RECOMMENDATION: leave the row byte-unchanged; document the decision in a brief comment

Per §0/§1/§2, the row needs no text change. The contract's corresponding instruction: *"in that case,
document that decision in a brief comment and leave the row unchanged."* So: keep the row text byte-
identical, and add a brief Markdown `<!-- -->` comment recording WHY (so a future maintainer doesn't
re-litigate or accidentally narrow the row).

PLACEMENT: the comment CANNOT go inside the Markdown table (a `<!-- -->` line between table rows terminates
the table and breaks rendering). It goes in the blank line BETWEEN the Features table and the next section
(`## Install`), which already exists (the table ends at the `| Discovery | … |` row, followed by a blank
line, then `## Install`). That gap is the safe, discoverable home.

The comment (exact text, §"Implementation Blueprint"):
```markdown
<!-- Multi-turn fallback (Features row above): intentionally generic — "stagehand" re-delivers, NOT "the
     commit path". Multi-turn runs on EVERY generation path (snapshot commit, `--dry-run`, hook mode); the
     per-path detail lives in docs/how-it-works.md#multi-turn-generation-fallback (linked from the row), so
     this high-level row deliberately does NOT enumerate paths. "no extra commits" is an anti-misconception
     note (one message/commit, not N), accurate on all three paths. Do not narrow this row. (P1.M4.T1.S2.) -->
```

## §4 — FALLBACK (only if the reviewer judges the row commit-path-only): a 4-word insertion, no path enumeration

IF a reviewer disagrees with §0 and reads "so the message still lands — no extra commits" as implying
commit-path-only, the MAXIMAL acceptable change is a 4-word insertion that signals broad coverage WITHOUT
enumerating paths (respecting "not a path-by-path reference") and WITHOUT bloat:

- BEFORE: `…stagehand re-delivers the full diff across session turns so the message still lands — no truncation, no extra commits…`
- AFTER:  `…stagehand re-delivers the full diff across session turns so the message still lands on any generation path — no truncation, no extra commits…`

(The insertion is the phrase **" on any generation path"** immediately after "lands".) This is the ceiling
of acceptable change: do NOT enumerate "commit / --dry-run / hook" in the row (that's how-it-works.md's
job), do NOT add a sentence, do NOT touch the links. If this fallback is taken, the §3 comment is still
added (it documents the coverage intent either way).

## §5 — Mode B validation: rendered table unchanged + comment present + no code/other-doc churn

This is a Mode-B docs task. Validation = (a) the rendered Features table is byte-identical to before (the
row text unchanged — only a source comment is added, invisible in rendered GitHub markdown); (b) the
comment is present in the gap before `## Install`; (c) `git status` shows ONLY README.md modified; (d) the
two row links still resolve (`docs/how-it-works.md#multi-turn-generation-fallback` — which S1's edit (d)
reinforces — and `docs/configuration.md#built-in-defaults`); (e) NO `.go`/test/other-doc file touched;
`go build ./... && go test ./...` green and unchanged (no code touched). If the §4 fallback is taken,
additionally diff-confirm the 4-word insertion is the ONLY row-text change.

## §6 — Coordination with the parallel sibling (P1.M4.T1.S1)

S1 edits docs/how-it-works.md ONLY (hook-mode note + "three paths" sentence + per-chunk-estimate note +
FR-T12 verify). S2 edits README.md ONLY. Different files ⇒ NO conflict, NO merge hazard. S1's "three paths"
sentence in how-it-works.md is what makes the README row's generic pointer land on a page that DOES state
the coverage — the two tasks compose: README = high-level pointer, how-it-works.md = the detail. Do NOT
duplicate S1's path enumeration in the README row (§2).
