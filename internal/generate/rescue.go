// This file adds the user-facing rescue render function for the FR43–FR45
// rescue protocol (P1.M6.T1.S3): when a snapshot was taken but a commit was
// NOT created, [Rescue] prints the PRD §18.3 recovery block (the failure
// notice, the "files were safely snapshotted" line, the frozen Tree ID, and
// the EXACT copy-pasteable `git commit-tree ... | xargs git update-ref HEAD`
// manual command) to the injected [*ui.Output]. It is a PURE RENDER function
// — no git binary, no exec, no filesystem, no os.Exit, no return value — the
// loop control / signal wiring / exit-code setting are the CALLER's job
// (CommitStaged P1.M6.T1.S1 + the signal handler P1.M6.T2.S1). It uses a
// plain "package generate" line because [generate.go] (P1.M6.T1.S1) OWNS the
// // Package generate doc comment, mirroring how dedupe.go (sibling) defers
// to generate.go and internal/git/log.go defers to git.go.
package generate

import (
	"fmt"

	"github.com/dustin/stagehand/internal/ui"
)

// Rescue prints the PRD §18.3 rescue block to stderr via out.Progressf when a
// commit was NOT created after the staged index was safely snapshotted into
// the git object store (FR43 condition: TREE_SHA set and NEW_SHA not). It is
// a PURE RENDER function: no return value, no os.Exit, no git/exec/filesystem.
//
// When it fires — the non-CAS post-snapshot failure paths
// (PRD §18.2 failure-mode table: generation timeout, SIGINT/SIGTERM
// post-snapshot per FR45 + §18.4, empty/invalid-output-after-retries, and
// duplicate-subject exhaustion after all retries; decisions.md §3 RESCUE call
// sites). It does NOT fire on an update-ref CAS failure (§18.2): that path
// prints its OWN message and exits 1, so Rescue never covers it.
//
// What it prints — FR44 demands a failure notice + the frozen TREE_SHA + the
// EXACT manual recovery command. The block (PRD §18.3 verbatim; the proven
// reference_impl.md §5 handle_error() baseline ported and ENRICHED):
//
//	❌ Commit generation failed.
//	------------------------------------------------------------
//	Your staged files were safely snapshotted before generation.
//	Tree ID: <tree>
//
//	To commit the originally staged files manually:
//	  git commit-tree [-p <parent>] -m "Your message" <tree> | xargs git update-ref HEAD
//
//	(omit "-p <parent>" if this is the repository's first commit)
//	------------------------------------------------------------
//
// followed, ONLY when candidate != "", by the PRD §18.3 candidate-message
// enrichment:
//
//	A candidate message was produced but rejected: "<candidate>". You can use
//	it manually in the command above.
//
// so the user's spent quota is never wasted. candidate is "" when no rejected
// message exists (e.g. the SIGINT/SIGTERM path never has one).
//
// Two deliberate enrichments over the reference script:
//
//   - Root omits "-p". reference_impl.md §5 ALWAYS printed `-p $PARENT_SHA`.
//     But for an unborn repo, git.RevParseHEAD returns hasParent=false and
//     PARENT_SHA="" (context: internal/git/plumbing.go). Showing `-p <empty>`
//     is wrong, so when parent=="" the manual command omits `-p` entirely
//     (`git commit-tree -m "Your message" <tree> | xargs git update-ref HEAD`).
//     The static omit-note keeps a LITERAL "<parent>" placeholder (an empty
//     root parent would otherwise read `omit "-p "`, which is broken).
//
//   - Candidate-message enrichment (above) is not in the reference script.
//
// Stream routing — FR51 stream discipline (internal/ui/output.go): the rescue
// block is a FAILURE/RECOVERY notice, NOT a commit result, so EVERY line is
// written via out.Progressf (stderr ALWAYS). Routing it to out.Resultf/stdout
// would corrupt `stagehand --dry-run | tee /tmp/msg.txt` pipelines, because
// the failure notice would leak into the piped message file. The ❌ notice is
// optionally wrapped via out.Red (red SGR when color on, no-op when off);
// color is orthogonal to routing.
//
// Exit code — Rescue does NOT set it. ui.ExitRescue == 3 (internal/ui/exitcode.go,
// PRD §15.4) is the CALLER's responsibility: CommitStaged returns it on the
// parse-fail-after-retries / duplicate-exhaustion / timeout paths
// (P1.M6.T1.S1), and the signal handler sets it on a post-snapshot
// SIGINT/SIGTERM (P1.M6.T2.S1, §18.4). Rescue only renders.
func Rescue(out *ui.Output, tree, parent, candidate string) {
	// Build the manual recovery command, branching on parent. parent=="" means
	// an unborn repo (no HEAD yet), so the root commit has no -p <parent>.
	cmd := fmt.Sprintf("git commit-tree -m \"Your message\" %s | xargs git update-ref HEAD", tree)
	if parent != "" {
		cmd = fmt.Sprintf("git commit-tree -p %s -m \"Your message\" %s | xargs git update-ref HEAD", parent, tree)
	}

	// Write the §18.3 block to stderr (FR51): one Progressf per logical line;
	// each format string owns its trailing newline so multi-line composition is
	// deterministic. The ❌ notice is wrapped via out.Red (red SGR when color
	// on, no-op when off); color is orthogonal to the stderr routing.
	out.Progressf("%s\n", out.Red("❌ Commit generation failed."))
	out.Progressf("------------------------------------------------------------\n")
	out.Progressf("Your staged files were safely snapshotted before generation.\n")
	out.Progressf("Tree ID: %s\n\n", tree)
	out.Progressf("To commit the originally staged files manually:\n")
	out.Progressf("  %s\n\n", cmd)
	out.Progressf("(omit \"-p <parent>\" if this is the repository's first commit)\n")
	out.Progressf("------------------------------------------------------------\n")

	// Candidate-message enrichment (PRD §18.3): only when a rejected message
	// was produced. Use explicit "%s" double-quote interpolation, NOT Go's %q,
	// which would escape quotes and mismatch the contract's literal "<msg>".
	if candidate != "" {
		out.Progressf("A candidate message was produced but rejected: \"%s\". You can use it manually in the command above.\n", candidate)
	}
}
