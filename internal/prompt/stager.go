package prompt

import "strings"

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

// stagerFilesHeader is the verbatim §17.6 files-block header (trailing COLON — mirrors
// stagerInstruction). Rendered ONLY when len(files) > 0 (PRD §17.6: "an empty list simply omits the
// files block"). NO trailing newline (package convention); BuildStagerTask emits "\n" then the joined
// paths after it. Pure ASCII.
const stagerFilesHeader = "Files for this concept (where these changes live):"

// stagerGuardrails is the verbatim §17.6 five-line git-instructions + hard-guardrails block. It is the
// prompt-level restatement of §13.6.2/§17.6's structural guardrails (no commit/amend/push/ref-mutation;
// only update the index), enforced STRUCTURALLY too via tooled_flags (§12.1; stagecoach owns all ref ops).
// The second sentence references the surfaced files block ("the files above are where they live").
//
// NOTE the TWO BACKTICK chars (`git add <path>` and `git apply --cached`) — hence a double-quoted "..."
// literal (a backtick raw string cannot contain backticks; will not compile).
// NOTE the EM-DASH "—" (U+2014) in "file contents — only update the index" — the ONE non-ASCII byte.
// Do NOT replace with an ASCII hyphen (verbatim §17.6 fidelity).
// NOTE the literal `<path>` token inside `git add <path>` is instructive (part of the command example),
// NOT a runtime placeholder. Only <title>/<description>/<files> are placeholders.
// NO trailing newline.
const stagerGuardrails = "Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply\n" +
	"only the relevant hunks via `git apply --cached`). Stage ONLY the changes the description\n" +
	"assigns to this concept (the files above are where they live); leave everything else unstaged.\n" +
	"Do not commit, do not amend, do not push, do not modify file contents — only update the index.\n" +
	"When done, reply with the list of paths you staged and stop."

// BuildStagerTask implements PRD §17.6 / §13.6.2 / FR-M5: assemble the stager task prompt (delivered as
// the user payload; system prompt minimal/empty — not a prompt-package constant). The orchestrator
// (P3.M2.T3.S1) calls this for each concept[i] from the planner's output:
//
//	task := prompt.BuildStagerTask(concept.Title, concept.Description, concept.Files)
//
// The stager returns free-form text ("list of paths staged"); the truth source is the index
// (git diff --cached --name-only), hence NO JSON contract / NO parse — the caller reads the exit code.
// The guardrails are ALSO enforced structurally (tooled_flags §12.1; stagecoach owns all ref ops —
// §17.6's safety proof).
//
// files is GUIDANCE (where the concept's changes live), NOT a hard constraint — FR-M1c
// (content ⊆ T_start) remains the sole content guarantee. An empty/nil list OMITS the files block
// entirely (no blank-line artifact) — PRD §17.6 line 1818.
//
// ASSEMBLY TOPOLOGY (PRD §17.6):
//
//	stagerInstruction       // "...match this concept:" (no trailing \n)
//	'\n' '\n'               // ONE blank line before the concept
//	title                   // the concept's short label (verbatim; single-line per planner §17.5)
//	'\n'                    // title + description on CONSECUTIVE lines (§17.6 — no blank between)
//	description             // the staging instructions (verbatim; may be multi-line)
//	[if len(files) > 0:     // the files block, ONLY when non-empty (omitted cleanly otherwise)
//		'\n' '\n'           // ONE blank line before the files block
//		stagerFilesHeader   // "Files for this concept (where these changes live):" (no trailing \n)
//		'\n'                // header + first path on consecutive lines
//		strings.Join(files, '\n')]  // one path per line (single \n between paths)
//	'\n' '\n'               // ONE blank line before the guardrails
//	stagerGuardrails        // the 5-line git-instructions + hard-guardrails block (no trailing \n)
//
// title, description, and files are interpolated VERBATIM from the planner's PlannerCommit{Title,
// Description, Files}. Defensive: empty title/description/files do not panic (the planner always
// supplies title+description per §17.5; files may legitimately be nil/empty ⇒ block omitted).
func BuildStagerTask(title, description string, files []string) string {
	var b strings.Builder
	b.WriteString(stagerInstruction)
	b.WriteString("\n\n")
	b.WriteString(title)
	b.WriteByte('\n')
	b.WriteString(description)
	if len(files) > 0 {
		b.WriteString("\n\n")
		b.WriteString(stagerFilesHeader)
		b.WriteByte('\n')
		b.WriteString(strings.Join(files, "\n"))
	}
	b.WriteString("\n\n")
	b.WriteString(stagerGuardrails)
	return b.String()
}
