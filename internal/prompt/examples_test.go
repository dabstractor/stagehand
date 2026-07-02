package prompt

// White-box test for package prompt (matches the internal/ui, internal/provider,
// internal/git convention: the _test.go file is `package prompt`, NOT
// `package prompt_test`, so it can exercise the unexported splitExampleGroups
// helper and the unexported exampleLineCap constant directly). Logic tests use
// a deterministic fakeReader implementing HistoryReader; the integration test
// drives the REAL git binary via os/exec through a *git.Git (TEST-ONLY import
// of internal/git — NO cycle: git does not import prompt).

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dustin/stagehand/internal/git"
)

// fakeReader is a deterministic HistoryReader for the logic tests. CommitCount
// returns (count, cerr); RecentMessages ignores n and returns (raw, rerr) so a
// canned `---%n%B` stream feeds FetchExamples verbatim. It implements the
// HistoryReader interface structurally (just like *git.Git does in the
// integration test below).
type fakeReader struct {
	count int
	raw   string
	cerr  error
	rerr  error
}

func (f fakeReader) CommitCount() (int, error)            { return f.count, f.cerr }
func (f fakeReader) RecentMessages(_ int) (string, error) { return f.raw, f.rerr }

// Compile-time assertion that fakeReader satisfies HistoryReader (catches a
// signature drift at compile time, not at the call site).
var _ HistoryReader = fakeReader{}

// examplesEqual is a small order-sensitive slice equality helper (no testify:
// the project uses stdlib testing only). It compares whole-message elements
// exactly.
func examplesEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestFetchExamples_NewRepoReturnsEmpty covers the FR39 root case: a repo with
// 0 or 1 commit has no history to learn a style from, so FetchExamples returns
// ([], false, nil) — an EMPTY result that is NOT an error (PRD §17.2's
// conventional-commit fallback hinges on this gate).
func TestFetchExamples_NewRepoReturnsEmpty(t *testing.T) {
	for _, count := range []int{0, 1} {
		got, hasML, err := FetchExamples(fakeReader{count: count}, DefaultExampleCount)
		if err != nil {
			t.Fatalf("count=%d: err = %v; want nil", count, err)
		}
		if hasML {
			t.Errorf("count=%d: hasMultiline = true; want false", count)
		}
		if len(got) != 0 {
			t.Errorf("count=%d: examples = %v; want empty", count, got)
		}
	}
}

// TestFetchExamples_SingleLineOnlyNoMultiline: an all-single-line history
// yields hasMultiline=false and the messages newest-first, blank-trimmed. The
// raw stream mirrors git log --format=---%n%B -20 output.
func TestFetchExamples_SingleLineOnlyNoMultiline(t *testing.T) {
	raw := "---\nfix: a\n---\nfeat: b\n"
	want := []string{"fix: a", "feat: b"} // newest-first: git log emits newest-first
	got, hasML, err := FetchExamples(fakeReader{count: 2, raw: raw}, DefaultExampleCount)
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}
	if hasML {
		t.Errorf("hasMultiline = true; want false")
	}
	if !examplesEqual(got, want) {
		t.Errorf("examples = %#v; want %#v", got, want)
	}
}

// TestFetchExamples_DetectsMultiline_AnyGroup: a single multi-line group
// (subject + body) among the messages flips hasMultiline to true, and the
// blank line inside it is trimmed (the element is "feat: x\nbody line").
func TestFetchExamples_DetectsMultiline_AnyGroup(t *testing.T) {
	raw := "---\nfeat: x\n\nbody line\n---\nfix: b\n"
	want := []string{"feat: x\nbody line", "fix: b"}
	got, hasML, err := FetchExamples(fakeReader{count: 2, raw: raw}, DefaultExampleCount)
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}
	if !hasML {
		t.Errorf("hasMultiline = false; want true (a group has >1 non-blank line)")
	}
	if !examplesEqual(got, want) {
		t.Errorf("examples = %#v; want %#v", got, want)
	}
}

// TestFetchExamples_DetectsMultiline_LastGroup is ★ THE AWK-BUG FIX ★: the
// ONLY multi-line commit is the LAST/oldest group (no trailing `---`). The
// reference awk's END block never re-checks the final accumulated group, so it
// would print 0 here. The Go port MUST flush + check the last group and
// therefore report hasMultiline=true.
func TestFetchExamples_DetectsMultiline_LastGroup(t *testing.T) {
	raw := "---\nfix: newest single\n---\nfeat: oldest sub\n\noldest body\n"
	got, hasML, err := FetchExamples(fakeReader{count: 2, raw: raw}, DefaultExampleCount)
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}
	if !hasML {
		t.Errorf("hasMultiline = false; want true (the last/oldest group is multi-line — the awk-bug fix)")
	}
	// The examples still come through newest-first, blank-trimmed.
	want := []string{"fix: newest single", "feat: oldest sub\noldest body"}
	if !examplesEqual(got, want) {
		t.Errorf("examples = %#v; want %#v", got, want)
	}
}

// TestFetchExamples_TrimsBlankLines: consecutive blank lines inside a commit
// message are dropped (the sed '/^$/d' step), and the remaining lines are
// joined by "\n" verbatim (interior whitespace is NOT trimmed). This group is
// multi-line (>1 non-blank line) so hasMultiline is also true.
func TestFetchExamples_TrimsBlankLines(t *testing.T) {
	raw := "---\nfeat: x\n\n\nbody\n"
	got, hasML, err := FetchExamples(fakeReader{count: 2, raw: raw}, DefaultExampleCount)
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}
	if !hasML {
		t.Errorf("hasMultiline = false; want true (group has >1 non-blank line)")
	}
	want := []string{"feat: x\nbody"}
	if !examplesEqual(got, want) {
		t.Errorf("examples = %#v; want %#v", got, want)
	}
}

// TestSplitExampleGroups_KeepsLinesVerbatim is a focused unit test on the
// helper (white-box): it verifies a non-blank line keeps its interior
// whitespace (sed strips only empty lines), and that a multi-blank-line group
// collapses to its non-blank lines joined by the preserved separators.
func TestSplitExampleGroups_KeepsLinesVerbatim(t *testing.T) {
	raw := "---\n  feat: indented  \n---\nfix: b\n"
	groups := splitExampleGroups(raw)
	if len(groups) != 2 {
		t.Fatalf("groups = %d; want 2", len(groups))
	}
	if len(groups[0]) != 1 || groups[0][0] != "  feat: indented  " {
		t.Errorf("group[0] = %#v; want [\"  feat: indented  \"] (verbatim, not trimmed)", groups[0])
	}
	if !examplesEqual(groups[1], []string{"fix: b"}) {
		t.Errorf("group[1] = %#v; want [\"fix: b\"]", groups[1])
	}
}

// TestFetchExamples_CapsAt100Lines: with more than exampleLineCap non-blank
// lines across several groups, the returned examples span at most 100 lines,
// later whole groups are dropped, and NO message is cut mid-way (every element
// is a whole group). hasMultiline is still computed over ALL groups pre-cap.
func TestFetchExamples_CapsAt100Lines(t *testing.T) {
	// Build groups whose line counts are 60, 50, 10. Total = 120 > 100.
	// Newest-first accumulation: group0(60) -> total 60; group1(50) -> 60+50=110
	// > 100 -> STOP. So only group0 is returned (60 <= 100), groups 1 and 2 are
	// dropped whole, and group2 (multi-line) still drives hasMultiline=true.
	var group0, group1, group2 strings.Builder
	group0.WriteString("---\n")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&group0, "g0 line %d\n", i)
	}
	group1.WriteString("---\n")
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&group1, "g1 line %d\n", i)
	}
	group2.WriteString("---\n")
	group2.WriteString("g2 a\n")
	group2.WriteString("g2 b\n") // multi-line group -> hasMultiline must be true even though dropped
	raw := group0.String() + group1.String() + group2.String()

	got, hasML, err := FetchExamples(fakeReader{count: 3, raw: raw}, DefaultExampleCount)
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}

	// hasMultiline is computed over ALL groups (pre-cap): group2 has 2 lines.
	if !hasML {
		t.Errorf("hasMultiline = false; want true (group2 has >1 line, even though the cap drops it)")
	}

	// Only group0 fits: total lines == 60 <= 100; group1 (110>100) stops it.
	if len(got) != 1 {
		t.Fatalf("len(examples) = %d; want 1 (only group0 fits the whole-message cap)", len(got))
	}
	wantLines := 60
	if lineCount(got) != wantLines {
		t.Errorf("total kept lines = %d; want %d (<= %d)", lineCount(got), wantLines, exampleLineCap)
	}
	if lineCount(got) > exampleLineCap {
		t.Errorf("total kept lines = %d; exceeds cap %d", lineCount(got), exampleLineCap)
	}
	// group0 must be whole (60 lines), not cut mid-way.
	if strings.Count(got[0], "\n")+1 != 60 {
		t.Errorf("group0 element has %d lines; want 60 (whole message, not cut)", strings.Count(got[0], "\n")+1)
	}
}

// TestFetchExamples_PropagatesErrors: a CommitCount error and a RecentMessages
// error both propagate verbatim (NOT swallowed) — the unborn case is already
// (0,nil)/("",nil) in git, so any error here is a genuine failure.
func TestFetchExamples_PropagatesErrors(t *testing.T) {
	t.Run("commit count error", func(t *testing.T) {
		cerr := errors.New("boom: rev-list failed")
		ex, hasML, err := FetchExamples(fakeReader{count: 5, cerr: cerr}, DefaultExampleCount)
		if err != cerr {
			t.Errorf("err = %v; want %v (propagated verbatim)", err, cerr)
		}
		if hasML {
			t.Errorf("hasMultiline = true; want false on error")
		}
		if ex != nil {
			t.Errorf("examples = %v; want nil on error", ex)
		}
	})
	t.Run("recent messages error", func(t *testing.T) {
		rerr := errors.New("log boom")
		ex, hasML, err := FetchExamples(fakeReader{count: 5, rerr: rerr}, DefaultExampleCount)
		if err != rerr {
			t.Errorf("err = %v; want %v (propagated verbatim)", err, rerr)
		}
		if hasML {
			t.Errorf("hasMultiline = true; want false on error")
		}
		if ex != nil {
			t.Errorf("examples = %v; want nil on error", ex)
		}
	})
}

// lineCount sums the line counts of each whole-message example (each element's
// "\n"-count + 1). Used by the cap test to assert the total stays <= 100.
func lineCount(examples []string) int {
	var n int
	for _, ex := range examples {
		n += strings.Count(ex, "\n") + 1
	}
	return n
}

// ---------------------------------------------------------------------------
// Real-git integration test (satisfies the contract "temp-repo seeded" MOCKING)
// ---------------------------------------------------------------------------

// seedGitRepo shells out to the REAL git binary via os/exec to build an
// isolated repo at dir and append len(msgs) commits — one per message, in
// order, so the LAST message is newest/HEAD. It mirrors internal/git's
// gittestutil_test.go seeding (which is unexported package-git _test.go and so
// cannot be imported here): git init -q, REPO-LOCAL deterministic config
// (user.email/user.name/commit.gpgsign=false), one UNIQUE file per commit so
// the tree changes, and `git commit -q -m <whole msg as ONE -m arg>` so
// multi-line bodies reproduce verbatim.
//
// `init` is a no-op on an already-initialized dir, so this helper may be
// called MORE THAN ONCE on the same dir to APPEND commits (e.g. add a
// multi-line commit after an initial single-line batch). To keep every
// commit's tree change distinct across such append calls, the file index is
// offset by `start` — the caller passes the count of commits already present
// so filenames (and thus the staged content) never collide between calls.
func seedGitRepo(t *testing.T, dir string, start int, msgs []string) {
	t.Helper()
	gitCmd := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitCmd("init", "-q") // no-op on an already-initialized dir
	gitCmd("config", "user.email", "stagehand@example.com")
	gitCmd("config", "user.name", "Stagehand Test")
	gitCmd("config", "commit.gpgsign", "false") // repo-local, zero production impact
	for i, msg := range msgs {
		idx := start + i
		name := fmt.Sprintf("file%d.txt", idx)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(fmt.Sprintf("content %d\n", idx)), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		gitCmd("add", name)
		gitCmd("commit", "-q", "-m", msg) // whole msg as ONE -m arg (multi-line reproduced verbatim)
	}
}

// TestFetchExamples_RealGitTempRepo drives the REAL git binary (git 2.54) end
// to end: a temp repo wrapped by *git.Git (which satisfies HistoryReader
// structurally), seeded single-line-only -> hasMultiline==false; then ONE
// multi-line commit is added -> hasMultiline==true. It imports internal/git
// TEST-ONLY (no cycle) and t.Skip's cleanly if the git binary is absent.
func TestFetchExamples_RealGitTempRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git binary not found in PATH: %v", err)
	}
	dir := t.TempDir()
	// Seed a SINGLE-LINE-only history first (>=2 commits so count>1).
	firstBatch := []string{"feat: first feature", "fix: second fix"}
	seedGitRepo(t, dir, 0, firstBatch)

	g, err := git.New(dir)
	if err != nil {
		t.Fatalf("git.New(%q): %v", dir, err)
	}
	// *git.Git must satisfy prompt.HistoryReader structurally (decoupling seam).
	var _ HistoryReader = g

	ex, hasML, err := FetchExamples(g, DefaultExampleCount)
	if err != nil {
		t.Fatalf("FetchExamples (single-line): err = %v; want nil", err)
	}
	if hasML {
		t.Errorf("single-line-only history: hasMultiline = true; want false")
	}
	if len(ex) == 0 {
		t.Fatalf("single-line-only history: examples empty; want the seeded messages")
	}
	// Newest-first: HEAD ("fix: second fix") should be the first example's
	// (only) line.
	if first := strings.TrimSpace(ex[0]); first != "fix: second fix" {
		t.Errorf("newest example = %q; want %q", first, "fix: second fix")
	}

	// Now add ONE multi-line commit -> hasMultiline MUST flip to true. Pass
	// start=len(firstBatch) so its file is distinct from the first batch.
	seedGitRepo(t, dir, len(firstBatch), []string{"feat: add multi-line commit\n\nThis commit has a body\nspanning multiple lines."})
	ex2, hasML2, err := FetchExamples(g, DefaultExampleCount)
	if err != nil {
		t.Fatalf("FetchExamples (after multi-line): err = %v; want nil", err)
	}
	if !hasML2 {
		t.Errorf("after one multi-line commit: hasMultiline = false; want true")
	}
	if len(ex2) == 0 {
		t.Fatalf("after multi-line commit: examples empty; want the seeded messages")
	}
	// The newest example is the multi-line commit; verify its body survived the
	// blank trim (joined by "\n", blank paragraph line dropped).
	wantNewest := "feat: add multi-line commit\nThis commit has a body\nspanning multiple lines."
	if got := ex2[0]; got != wantNewest {
		t.Errorf("newest example = %q; want %q (blank-trimmed, lines joined by \\n)", got, wantNewest)
	}
}
