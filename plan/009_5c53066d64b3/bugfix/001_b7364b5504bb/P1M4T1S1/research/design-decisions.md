# P1.M4.T1.S1 — Design Decisions & Research Notes

> Research backing `PRP.md`: a Mode-B docs sync of `docs/how-it-works.md` so the multi-turn section +
> hook-mode section reflect that multi-turn now runs on all three generation paths (commit / dry-run /
> hook) after the P1.M2/P1.M3 propagation, plus the Issue-3 verbose/progress per-chunk estimate. Docs-only —
> no code, no tests.

## 0. Scope: ONLY `docs/how-it-works.md`. Touch NOTHING else.

The task is the documentation half of the multi-turn propagation changeset. Edits land in TWO sections of
ONE file: the **multi-turn section** (L262–298) and the **hook-mode section** (L300–324). Explicitly do NOT
modify `docs/configuration.md` (its multi-turn knobs at L110-111/137-138/155-157 are already accurate) or
`docs/providers.md` (its `session_mode` subsection at L29/40-49 is already accurate) — the task forbids it.
`README.md` is a SEPARATE subtask (P1.M4.T1.S2) — do not touch it. No `.go` files, no tests (Mode B).

## 1. Edit (a) — hook-mode section: add the multi-turn-in-hook-mode note (the main edit)

The hook-mode section (`## Hook mode vs the snapshot-based flow`, L300) lists the hook contract (pre-commit
hooks honored / no snapshot / never-block / no rescue) and **does NOT mention multi-turn** — so post-
propagation it's stale (multi-turn now fires in hook mode too, confirmed at `internal/hook/exec.go:215`).
Add a concise (2-sentence) note. PLACE it after the "Hook mode" bullets block (the "No rescue protocol"
bullet ends “…the commit simply proceeds without an AI message.”) and before `### When to use which`
(L320). Verbatim text (the implementer pastes this):

> **Multi-turn fallback in hook mode.** The [multi-turn fallback](#multi-turn-generation-fallback) is
> available in hook mode too: on a large diff with an append-mode provider, the hook tries it as one extra
> attempt before the never-block exit. On success the generated message is written to the commit-message
> file; on any failure — a turn error, an empty final parse, or a duplicate subject — the hook still exits
> 0 with the message file untouched (FR-H5 preserved).

This matches the task's (a) verbatim intent and the P1.M3.T1.S2 contract (write ONLY on
`cause==nil && ok2 && !duplicate`; every failure → exhaustion error → cmd `neverBlock` → exit 0 + untouched
msg-file). The markdown link target `#multi-turn-generation-fallback` is the existing anchor for the
multi-turn section header (GitHub lowercases + hyphenates the `## Multi-turn generation fallback` heading).

## 2. Edit (b) — multi-turn FR-T12 paragraph: VERIFY (already accurate post-Issue-4); optional minor tweak

The "`token_limit` does not apply (FR-T12)" paragraph (end of the multi-turn section) currently says
multi-turn "re-captures the diff with `token_limit` disabled and delivers the **untruncated** diff across
the N+1 turns … (The re-capture is skipped when `token_limit` is unset, since the one-shot payload is
already untruncated in that case.)" The Issue-4 fix (P1.M1.T2.S1) made `mtPayload` ALWAYS rebuild from
`diff` via `prompt.BuildUserPayload(diff, …)` (NOT reused from the one-shot `payload`, which could carry the
retryInstr corrective preamble) — so the docs' "delivers the untruncated diff" claim is now CORRECT (the
fix made the code match the docs). **Required action: read it, confirm it still reads accurately — NO
change strictly needed.** The task's "captured ONCE and unmodified" is a paraphrase (that exact phrase is
NOT in the file — grep confirmed; the only "captured" hit is an unrelated arbiter/T_start line at L119).
Optional (only if the implementer wants extra precision): append one short clause noting the multi-turn
payload is rebuilt fresh from the diff — but keep it user-facing (do NOT drag in the retryInstr/preamble
implementation detail, which is too deep for how-it-works.md). Default: leave (b) as-is.

## 3. Edit (c) — the per-chunk token estimate note (Issue 3 / FR-T11) — word it ACCURATELY (always-on, not verbose-only)

Issue 3 (P1.M1.T3.S1) extended the fallback progress line to include the per-chunk token budget:
`internal/generate/generate.go:340` → `"↳ falling back to multi-turn: %d turns (chunks of ~%d tokens),
~%dm total\n"`. CRITICAL ACCURACY POINT: that `fmt.Fprintf(os.Stderr, …)` is **UNCONDITIONAL** (always-on
progress, like every `↳ …` line per FR51b) — it is NOT gated behind `--verbose`. The separate
`deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")` is the verbose-gated line, and the
per-TURN payload/raw-output detail is emitted inside `generate.Run`. So the task's framing ("--verbose
shows the per-chunk token estimate") is slightly imprecise: the chunk budget is in the ALWAYS-ON progress
line; `--verbose` adds the trigger warning + per-turn detail. Word the note to match the code:

> That progress line also reports the per-chunk token budget each chunk targets; with `--verbose`, each
> turn additionally prints its payload size and raw agent output (FR-T11).

PLACE it by extending the existing sentence “Each turn is a separate provider invocation with its own
timeout; total wall-clock ≈ `timeout × (N+1)`, surfaced on the progress line at fallback time.” (append the
new sentence right after it). One sentence; accurate; covers FR-T11.

## 4. The "three paths" coverage sentence (serves the OUTPUT's "across all three paths" goal)

The task OUTPUT says "reflecting multi-turn availability across all three paths." Edit (a) covers the hook
path explicitly; the commit path is already documented; to make the three-path coverage unambiguous IN the
multi-turn section, add ONE sentence at the end of the intro paragraph (the one ending “…the model can
handle the same content delivered in smaller pieces.”):

> Multi-turn runs on every generation path — the snapshot commit flow, `--dry-run`, and hook mode (where
> it composes with the never-block contract; see [Hook mode](#hook-mode-vs-the-snapshot-based-flow) below).

Verified the three gate sites exist: `internal/generate/generate.go:304` (CommitStaged),
`pkg/stagecoach/stagecoach.go:555` (runPipeline / dry-run), `internal/hook/exec.go:215` (hook.Run). The
cross-link target `#hook-mode-vs-the-snapshot-based-flow` is the existing anchor for the hook section
header. One sentence; low-risk; directly delivers the OUTPUT.

## 5. No code/test/other-doc changes (Mode B validation)

This subtask changes ONLY `docs/how-it-works.md`. Validation is: (1) the 4 edits above land in the right
anchors; (2) markdown renders (anchors resolve, no broken links); (3) `git status` shows ONLY
docs/how-it-works.md modified; (4) NO `.go` file changed ⇒ `go build ./... && go test ./...` is a no-op
regression check (still green, unchanged). No new tests (docs-only). The `docs/configuration.md` and
`docs/providers.md` multi-turn content is already accurate (research_config_provider_docs.md §5b confirmed)
— do NOT touch them (the task forbids it).

## Sources
- `docs/how-it-works.md` L262–324 (the two sections to edit) — read in full; exact anchors confirmed via grep.
- `plan/009…/docs/architecture/research_hook_exec.md §7` — "the hook-mode section should note multi-turn is
  available in hook mode too (composes with never-block)" — the doc-debt this subtask discharges.
- `plan/009…/docs/architecture/research_config_provider_docs.md §5b` — confirms configuration.md/providers.md
  are already accurate (do not touch) + the exact how-it-works multi-turn section content.
- `plan/009…/P1M3T1S2/PRP.md` (parallel) — the hook gate contract (WriteMessageFile ONLY on
  `cause==nil && ok2 && !duplicate`; every failure → exhaustion → neverBlock → exit 0 + untouched) — grounds
  edit (a)'s "on any failure … exits 0 with the message file untouched (FR-H5)".
- `internal/generate/generate.go:340` — the progress line is UNCONDITIONAL (`fmt.Fprintf(os.Stderr,…)`); the
  `VerboseWarn` (L343) is the verbose-gated line — grounds edit (c)'s accurate wording.
- `internal/{generate/generate.go:304, hook/exec.go:215}` + `pkg/stagecoach/stagecoach.go:555` — the three gate
  sites confirm the "three paths" claim (edit (d)).
- `plan/009…/P1M1T2S1/research/mtpayload_fix_notes.md` (Issue 4) — mtPayload rebuilt from `diff` ⇒ the FR-T12
  paragraph is already accurate (edit (b) is a verify/no-op).
- `plan/009…/P1M1T3S1/research/progress_line_chunk_tokens.md` (Issue 3) — the `(chunks of ~%d tokens)`
  addition to the progress line (edit (c)).
