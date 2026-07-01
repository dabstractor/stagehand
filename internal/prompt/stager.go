package prompt

// Stager task prompt — committed VERBATIM from PRD §17.6. The stager is a TOOLED agent (git access,
// repo-scoped) that stages exactly one concept's changes and stops (PRD §13.6.2 / FR-M5). Unlike the
// planner (§17.5) and arbiter (§17.7), the stager's prompt is delivered AS the user payload with a
// minimal/empty system prompt (the orchestrator's concern — no system-prompt constant here). There is
// NO JSON contract (the stager returns free-form text; the index is the truth source) and NO parse
// function (failure is exit-code-driven per §13.6.6). BuildStagerTask is the sole export.
//
// Constants are defined WITHOUT trailing newlines; BuildStagerTask owns ALL inter-block newline
// placement so the §17.6 blank-line topology lives in exactly one auditable place (mirrors the
// system.go/payload.go convention).
//
// §17.6 is ENTIRELY ASCII EXCEPT one em-dash "—" (U+2014) in the guardrails block.

// stagerInstruction is the verbatim §17.6 instruction line (trailing COLON, mirroring payload.go's
// userInstruction). NO trailing newline.
const stagerInstruction = "Stage, but do NOT commit, all changes in this repository that match this concept:"

// stagerGuardrails is the verbatim §17.6 five-line git-instructions + hard-guardrails block. It is the
// prompt-level restatement of §13.6.2/§17.6's structural guardrails (no commit/amend/push/ref-mutation;
// only update the index), enforced STRUCTURALLY too via tooled_flags (§12.1; stagehand owns all ref ops).
//
// NOTE the TWO BACKTICK chars (`git add <path>` and `git apply --cached`) — hence a double-quoted "..."
// literal (a backtick raw string cannot contain backticks; will not compile).
// NOTE the EM-DASH "—" (U+2014) in "file contents — only update the index" — the ONE non-ASCII byte.
// Do NOT replace with an ASCII hyphen (verbatim §17.6 fidelity).
// NOTE the literal `<path>` token inside `git add <path>` is instructive (part of the command example),
// NOT a runtime placeholder. Only <title>/<description> are placeholders.
// NO trailing newline.
const stagerGuardrails = "Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply\n" +
	"only the relevant hunks via `git apply --cached`). Stage ONLY changes belonging to this\n" +
	"concept; leave unrelated changes unstaged. Do not commit, do not amend, do not push, do not\n" +
	"modify file contents — only update the index. When done, reply with the list of paths you\n" +
	"staged and stop."

// BuildStagerTask implements PRD §17.6 / §13.6.2 / FR-M5: assemble the stager task prompt (delivered as
// the user payload; system prompt minimal/empty — not a prompt-package constant). The orchestrator
// (P3.M2.T3.S1) calls this for each concept[i] from the planner's output:
//
//	task := prompt.BuildStagerTask(concept.Title, concept.Description)
//
// The stager returns free-form text ("list of paths staged"); the truth source is the index
// (git diff --cached --name-only), hence NO JSON contract / NO parse — the caller reads the exit code.
// The guardrails are ALSO enforced structurally (tooled_flags §12.1; stagehand owns all ref ops —
// §17.6's safety proof).
//
// ASSEMBLY TOPOLOGY (PRD §17.6):
//
//	stagerInstruction       // "...match this concept:" (no trailing \n)
//	'\n' '\n'               // ONE blank line before the concept
//	title                   // the concept's short label (verbatim; single-line per planner §17.5)
//	'\n'                    // title + description on CONSECUTIVE lines (§17.6 — no blank between)
//	description             // the staging instructions (verbatim; may be multi-line)
//	'\n' '\n'               // ONE blank line before the guardrails
//	stagerGuardrails        // the 5-line git-instructions + hard-guardrails block (no trailing \n)
//
// title and description are interpolated VERBATIM from the planner's PlannerCommit{Title, Description}.
// Defensive: empty title/description do not panic (the planner always supplies both per §17.5).
func BuildStagerTask(title, description string) string {
	return stagerInstruction + "\n\n" + title + "\n" + description + "\n\n" + stagerGuardrails
}
