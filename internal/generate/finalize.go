package generate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
)

// ApplyTemplate applies the §9.19 FR-F8 message template: every literal "$msg" in tpl is replaced with the
// full generated message. Empty tpl ⇒ msg unchanged (the default; byte-identical to the pre-feature path).
// This is a POST-generation substitution (§17.8: "the model never sees it"), applied AFTER parse/cleanup
// and BEFORE the duplicate check so §9.7 judges the final subject as it will land. Substitution is literal
// and covers the FULL message (subject+body); "$msg" alone is the identity template.
func ApplyTemplate(msg, tpl string) string {
	if tpl == "" {
		return msg
	}
	return strings.ReplaceAll(tpl, "$msg", msg)
}

// FinalizeMessage is the shared message-finalization SEAM (§9.19 FR-F8): the single ordered pipeline every
// commit path funnels a parsed+cleaned message through to obtain the FINAL message as it will land. Today
// it is one stage — ApplyTemplate(msg, cfg.Template). It is invoked AFTER ParseOutput and BEFORE
// ExtractSubject/IsDuplicate in every generation loop, and on the planner's FR-M11 shortcut message before
// its dup-check, so the dedupe check (§9.7) always sees the templated subject.
//
// ORDERING CONTRACT (P1.M5.T1.S1): the --edit editor gate slots AFTER this seam (FR-E3: the template is
// applied before the editor opens). Extend the pipeline as template → (future) editor → publish; keep
// template first.
func FinalizeMessage(msg string, cfg config.Config) string {
	return ApplyTemplate(msg, cfg.Template)
}

// ErrEmptyMessage is the §9.22 FR-E1 abort signal: the editor returned an empty message (after stripping
// comments + whitespace). It is an INTENTIONAL abort, NOT a rescue — HEAD and the index are untouched
// (the editor runs after WriteTree but before CommitTree; the orphan tree is gc'd eventually). The CLI
// maps it to exit 1 with "empty commit message — aborted" (NOT exit 3/124 — no manual-recovery recipe).
var ErrEmptyMessage = errors.New("stagecoach: empty commit message — aborted")

// EditContext carries the snapshot + git boundary the editor gate needs to build the EDITMSG summary
// (§9.22 FR-E1: "the message plus a commented summary (tree SHA, diff-tree --name-status of the snapshot)").
// TreeSHA is always the frozen snapshot tree; NameStatus is the raw `git diff-tree --name-status -r` output
// (best-effort — "" if unavailable). Git resolves the .git dir + the editor command.
type EditContext struct {
	Git        git.Git // the git boundary (for GitDir + Editor)
	TreeSHA    string  // the frozen snapshot tree (treeB in decompose; treeSHA in single)
	NameStatus string  // raw A/M/D lines for the summary (best-effort; "" if unavailable)
}

// EditMessage is the §9.22 FR-E1 editor gate — a POST-dedupe, PRE-publish stage. cfg.Edit==false ⇒
// identity (the default; byte-identical to the pre-feature path). When true: write msg + a commented
// summary to <gitDir>/STAGECOACH_EDITMSG, open the resolved editor (`git var GIT_EDITOR` via sh -c), strip
// comment lines + trailing whitespace on close, return the edited message.
//
// An empty result (after strip) ⇒ ErrEmptyMessage (caller aborts: exit 1, NOT a rescue). A non-zero editor
// exit (e.g. vim's `:cq`) ⇒ a wrapped error (treated as an abort, NOT committed).
//
// ORDERING (FR-E3 + FR-F8): EditMessage runs AFTER generate.FinalizeMessage (which applies the template,
// pre-dedupe) and AFTER the dedupe check (so the user's hand-written message bypasses the re-check — git
// parity). The template was applied before, so the user edits the FINAL text.
func EditMessage(ctx context.Context, msg string, cfg config.Config, editCtx EditContext) (string, error) {
	if !cfg.Edit {
		return msg, nil // THE no-op guard — the byte-identity regression invariant
	}

	// 1. Resolve the .git dir + the EDITMSG path.
	gitDir, err := editCtx.Git.GitDir(ctx)
	if err != nil {
		return "", fmt.Errorf("--edit: resolve git dir: %w", err)
	}
	editMsgPath := filepath.Join(gitDir, "STAGECOACH_EDITMSG")

	// 2. Build the EDITMSG content: message + commented summary.
	var buf strings.Builder
	buf.WriteString(msg)
	buf.WriteString("\n\n# Please edit this commit message. Lines starting with '#' will be removed,\n")
	buf.WriteString("# and trailing whitespace will be stripped. An empty message aborts the commit.\n")
	fmt.Fprintf(&buf, "#\n# Tree: %s\n", editCtx.TreeSHA)
	if editCtx.NameStatus != "" {
		buf.WriteString("# Changes:\n")
		for _, line := range strings.Split(strings.TrimRight(editCtx.NameStatus, "\n"), "\n") {
			fmt.Fprintf(&buf, "# %s\n", line)
		}
	}
	if err := os.WriteFile(editMsgPath, []byte(buf.String()), 0o644); err != nil {
		return "", fmt.Errorf("--edit: write %s: %w", editMsgPath, err)
	}

	// 3. Resolve the editor (git var GIT_EDITOR → VISUAL → EDITOR → vi). Best-effort fallback; never fatal.
	editor, err := editCtx.Git.Editor(ctx)
	if err != nil || editor == "" {
		editor = firstNonEmpty(os.Getenv("VISUAL"), os.Getenv("EDITOR"), "vi")
	}

	// 4. Invoke via sh -c (the value is shell-interpreted — may contain args). Interactive stdio.
	cmd := exec.CommandContext(ctx, "sh", "-c", editor+" \"$@\"", "--", editMsgPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("--edit: editor %q exited with error: %w", editor, err) // abort (e.g. vim :cq)
	}

	// 5. Read back + strip comment lines + trailing whitespace (git parity).
	raw, err := os.ReadFile(editMsgPath)
	if err != nil {
		return "", fmt.Errorf("--edit: read %s: %w", editMsgPath, err)
	}
	edited := stripCommentsAndTrim(string(raw))
	if edited == "" {
		return "", ErrEmptyMessage // §9.22 FR-E1 abort — exit 1, NOT a rescue
	}
	return edited, nil
}

// stripCommentsAndTrim removes lines beginning with '#', trims trailing whitespace per line, drops empty
// lines, and joins the survivors with '\n'. Mirrors git's prepare-commit-msg cleanup (comment-char '#').
func stripCommentsAndTrim(s string) string {
	var kept []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if t := strings.TrimRight(line, " \t\r"); t != "" {
			kept = append(kept, t)
		}
	}
	return strings.Join(kept, "\n")
}

// firstNonEmpty returns the first non-empty string from vs, or "" if all are empty.
func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}
