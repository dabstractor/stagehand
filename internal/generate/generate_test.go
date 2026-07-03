package generate

// White-box unit tests for the snapshot-based atomic-commit orchestrator
// CommitStaged (P1.M6.T1.S1). It is `package generate` (NOT `generate_test`)
// so it sits in the same package as CommitStaged and the package-private
// gitClient/runner interfaces + firstLine/instruction, exactly like
// dedupe_test.go / rescue_test.go / signal_test.go. The two consumer-side
// interfaces (gitClient + runner) make CommitStaged fully stubbable WITHOUT a
// real git binary or a real agent: a stubGit implements gitClient with canned
// returns + a call log, and a stubRunner implements runner with a scripted
// response list (Run returns canned stdout/error, Parse returns canned
// (msg,ok)). Every two-nested-loop behavior is exercised hermetically.
//
// Integration against a REAL temp repo + the stub agent binary is the SEPARATE
// task P1.M6.T3.S2 (it has the temp-repo harness); THIS task ships the
// stubbable unit tests. Dependencies are stdlib bytes/context/errors/strings/
// testing + internal/{config,git,provider,ui} ONLY (NO testify, NO real LLM,
// NO real git). ui.NewOutput(&stdout,&stderr,false,true) keeps verbose off and
// noColor=true so out.Red is a no-op and captured text is plain.

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/ui"
)

// Compile-time guarantees that the REAL shipped types satisfy the consumer
// interfaces structurally (Go duck-typing): the stub-only unit tests below
// never drift from the production wiring, because a method-signature change
// on *git.Git / *provider.Executor that breaks the contract fails to compile
// here. *git.Git needs the additive git.DiffTreeNameStatus seam; *provider.Executor
// needs the additive (*Executor).Parse seam (both P1.M6.T1.S1).
var (
	_ gitClient = (*git.Git)(nil)
	_ runner    = (*provider.Executor)(nil)
)

// stubResponse scripts ONE Run→Parse pair in the inner loop: Run returns
// (stdout, err) and the IMMEDIATELY following Parse returns (msg, ok). The
// stubRunner indexes responses by Run call number; once the script is
// exhausted the LAST entry is replayed (clamp), matching the stub-agent
// harness convention (stubprovider_test.go Gotcha #3) so dup-exhaustion's
// repeated-Run case needs only one entry.
type stubResponse struct {
	stdout string // what Run returns on stdout
	err    error  // what Run returns as its error (nil / *TimeoutError / context.Canceled / *AgentError)
	msg    string // what the paired Parse returns
	ok     bool   // what the paired Parse returns
}

// stubRunner implements runner. Run returns the scripted (stdout, err) for the
// Nth call and captures the delivered payload (so the corrective-retry test
// can assert the diff STAYED and the instruction slot swapped). Parse reads
// the SAME response index (Run and Parse are always called as a pair), so it
// returns the (msg, ok) paired with the most recent Run. runCalls/parseCalls
// and the payload log are the observable call log the tests assert on.
type stubRunner struct {
	responses  []stubResponse
	runCalls   int
	parseCalls int
	payloads   []string
	lastSys    string // the system prompt delivered to the most recent Run (SystemExtra assertion)
}

func (r *stubRunner) Run(_ context.Context, _ provider.Manifest, _, _, sys string, payload string) (string, error) {
	i := r.runCalls
	r.runCalls++
	r.payloads = append(r.payloads, payload)
	r.lastSys = sys
	resp := r.response(i)
	return resp.stdout, resp.err
}

func (r *stubRunner) Parse(_ string, _ provider.Manifest) (string, bool) {
	i := r.runCalls - 1 // the response paired with the most recent Run
	if i < 0 {
		i = 0
	}
	r.parseCalls++
	resp := r.response(i)
	return resp.msg, resp.ok
}

// response returns the scripted stubResponse for index i, clamping overflow to
// the LAST entry (so a single dup response covers dup-exhaustion's 4 calls).
// It panics on an empty script — the tests always seed at least one entry.
func (r *stubRunner) response(i int) stubResponse {
	if len(r.responses) == 0 {
		panic("stubRunner: empty response script")
	}
	if i >= len(r.responses) {
		i = len(r.responses) - 1
	}
	return r.responses[i]
}

// stubGit implements gitClient with canned returns + a call log. Every method
// returns its configured value/err and records the call so tests assert call
// COUNTS (WriteTree/CommitTree/UpdateRefCAS) and the EXACT args passed to
// CommitTree/UpdateRefCAS (for the root-commit and head-moved cases).
type stubGit struct {
	// canned returns
	diff          string
	diffErr       error
	parentSHA     string
	hasParent     bool
	revParseErr   error
	treeSHA       string
	writeTreeErr  error
	count         int
	countErr      error
	subjects      []string
	subjectsErr   error
	newSHA        string
	commitTreeErr error
	casErr        error
	diffTreeOut   string

	// call log
	stagedDiffCalls   int
	revParseCalls     int
	writeTreeCalls    int
	commitCountCalls  int
	recentSubjCalls   int
	commitTreeCalls   int
	casCalls          int
	commitTreeParent  string
	commitTreeMsg     string
	commitTreeTree    string
	casRef            string
	casNew            string
	casExpected       string
	diffTreeNameCalls int
}

func (g *stubGit) StagedDiff(git.DiffSettings) (string, error) {
	g.stagedDiffCalls++
	return g.diff, g.diffErr
}

func (g *stubGit) RevParseHEAD() (string, bool, error) {
	g.revParseCalls++
	return g.parentSHA, g.hasParent, g.revParseErr
}

func (g *stubGit) WriteTree() (string, error) {
	g.writeTreeCalls++
	return g.treeSHA, g.writeTreeErr
}

func (g *stubGit) CommitTree(parent, msg, tree string) (string, error) {
	g.commitTreeCalls++
	g.commitTreeParent = parent
	g.commitTreeMsg = msg
	g.commitTreeTree = tree
	return g.newSHA, g.commitTreeErr
}

func (g *stubGit) UpdateRefCAS(ref, newSHA, expected string) error {
	g.casCalls++
	g.casRef = ref
	g.casNew = newSHA
	g.casExpected = expected
	return g.casErr
}

func (g *stubGit) CommitCount() (int, error) {
	g.commitCountCalls++
	return g.count, g.countErr
}

func (g *stubGit) RecentMessages(int) (string, error) {
	return "", nil // unused by CommitStaged except via FetchExamples (count>1 path)
}

func (g *stubGit) RecentSubjects(int) ([]string, error) {
	g.recentSubjCalls++
	return g.subjects, g.subjectsErr
}

func (g *stubGit) DiffTreeNameStatus(string) (string, error) {
	g.diffTreeNameCalls++
	return g.diffTreeOut, nil
}

// rescueRendered reports whether the §18.3 rescue block was rendered to
// stderr (its failure-notice marker), the assertion the candidate / no-rescue
// cases hinge on.
func rescueRendered(stderr *bytes.Buffer) bool {
	return strings.Contains(stderr.String(), "Commit generation failed")
}

// newTestOutput wires a *ui.Output whose stdout+stderr are captured into the
// given buffers, with verbose off and noColor=true (so out.Red is a no-op and
// the captured text is plain for substring assertions).
func newTestOutput(stdout, stderr *bytes.Buffer) *ui.Output {
	return ui.NewOutput(stdout, stderr, false, true)
}

// baseDeps builds a Deps wired to the given stubs with a config whose
// MaxDuplicateRetries is overrideable. It pre-seeds a non-root repo with one
// recent subject that does NOT collide with the generated messages, so a test
// gets the happy-path shape unless it overrides stubGit.subjects.
func baseDeps(g *stubGit, r *stubRunner, maxDup int) Deps {
	g.parentSHA = "parent000000000000000000000000000000000000" // 40-char-ish parent SHA
	g.treeSHA = "tree0000000000000000000000000000000000000a"
	g.newSHA = "newsha0000000000000000000000000000000000000b"
	g.diff = "DIFFCONTENT"
	g.hasParent = true
	g.count = 5
	g.subjects = []string{"existing: unrelated subject"}
	g.diffTreeOut = "A\tfeature.go\n"
	cfg := config.Default()
	cfg.MaxDuplicateRetries = maxDup
	return Deps{
		Git:      g,
		Runner:   r,
		Manifest: provider.Manifest{Name: "stub"},
		Config:   cfg,
		// Output is wired by each test to its own captured buffers.
	}
}

// TestCommitStaged_HappyPath proves the success path: a unique subject on the
// first try yields Result, prints the FR42 success block to stdout (the
// [<short>] <subject> line + the diff-tree name-status lines), does NOT
// rescue, and calls CommitTree + UpdateRefCAS EXACTLY once.
func TestCommitStaged_HappyPath(t *testing.T) {
	g := &stubGit{}
	r := &stubRunner{responses: []stubResponse{{stdout: "feat: add thing", msg: "feat: add thing", ok: true}}}
	deps := baseDeps(g, r, 3)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Output = newTestOutput(stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if err != nil {
		t.Fatalf("CommitStaged returned error %v; want nil", err)
	}
	if res.CommitSHA != g.newSHA {
		t.Errorf("Result.CommitSHA = %q; want %q", res.CommitSHA, g.newSHA)
	}
	if res.Subject != "feat: add thing" {
		t.Errorf("Result.Subject = %q; want %q", res.Subject, "feat: add thing")
	}
	if res.Message != "feat: add thing" {
		t.Errorf("Result.Message = %q; want %q", res.Message, "feat: add thing")
	}
	if r.runCalls != 1 {
		t.Errorf("Run call count = %d; want 1 (unique first try)", r.runCalls)
	}
	if g.commitTreeCalls != 1 || g.casCalls != 1 {
		t.Errorf("CommitTree=%d CAS=%d; want 1/1", g.commitTreeCalls, g.casCalls)
	}
	// FR42 success print to stdout: [<short>] <subject> + diff-tree lines.
	if missing := containsAll(stdout.String(), "[newsha0] feat: add thing", "A\tfeature.go"); missing != "" {
		t.Errorf("stdout missing %q\n--got--\n%s", missing, stdout.String())
	}
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT contain a rescue block on success\n--got--\n%s", stderr.String())
	}
	// CAS used the parent as the expected-old (non-root, 3-arg form).
	if g.casExpected != g.parentSHA {
		t.Errorf("CAS expected = %q; want parent %q (3-arg CAS)", g.casExpected, g.parentSHA)
	}
}

// TestCommitStaged_DupRetryThenSuccess proves the OUTER duplicate-rejection
// loop: the first generated subject duplicates a recent commit (rejected,
// appended to the rejected list) and the SECOND is unique (committed). Run is
// called exactly twice and the second payload carries the rejection block.
func TestCommitStaged_DupRetryThenSuccess(t *testing.T) {
	g := &stubGit{}
	r := &stubRunner{responses: []stubResponse{
		{stdout: "feat: dup", msg: "feat: dup", ok: true},
		{stdout: "feat: unique", msg: "feat: unique", ok: true},
	}}
	deps := baseDeps(g, r, 3)
	g.subjects = []string{"feat: dup"} // the first generated subject collides
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Output = newTestOutput(stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if err != nil {
		t.Fatalf("CommitStaged returned error %v; want nil", err)
	}
	if res.Subject != "feat: unique" {
		t.Errorf("Result.Subject = %q; want the second (unique) subject", res.Subject)
	}
	if r.runCalls != 2 {
		t.Errorf("Run call count = %d; want 2 (one dup + one unique)", r.runCalls)
	}
	// The second payload must carry the §17.3 rejection block (the rejected
	// subject) AND the diff (Ambiguity #2: the diff STAYS).
	if missing := containsAll(r.payloads[1], "feat: dup", "DIFFCONTENT",
		"already exist", "entirely new message"); missing != "" {
		t.Errorf("2nd payload missing %q\n--got--\n%s", missing, r.payloads[1])
	}
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT rescue on success\n--got--\n%s", stderr.String())
	}
}

// TestCommitStaged_ParseRetryThenSuccess proves the INNER parse-correction
// loop: the first Parse returns ok=false, the corrective retry REBUILDS the
// payload with the manifest RetryInstruction (the diff STAYS), and the second
// Parse returns ok=true (committed). Run is called exactly twice, both within
// the FIRST outer iteration (the inner budget is fresh per outer iter).
func TestCommitStaged_ParseRetryThenSuccess(t *testing.T) {
	g := &stubGit{}
	r := &stubRunner{responses: []stubResponse{
		{stdout: "junk", msg: "", ok: false},          // parse miss → corrective retry
		{stdout: "ok", msg: "feat: parsed", ok: true}, // parse hit
	}}
	deps := baseDeps(g, r, 3)
	deps.Manifest = provider.Manifest{
		Name:             "stub",
		RetryInstruction: "Please output a valid commit message only.",
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Output = newTestOutput(stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if err != nil {
		t.Fatalf("CommitStaged returned error %v; want nil", err)
	}
	if res.Subject != "feat: parsed" {
		t.Errorf("Result.Subject = %q; want the corrected subject", res.Subject)
	}
	if r.runCalls != 2 {
		t.Errorf("Run call count = %d; want 2 (parse miss + corrective retry)", r.runCalls)
	}
	// The FIRST payload uses the original instruction; the SECOND uses the
	// RetryInstruction — but BOTH keep the diff (Ambiguity #2).
	if !strings.Contains(r.payloads[0], instruction) {
		t.Errorf("1st payload = %q; want the original instruction %q", r.payloads[0], instruction)
	}
	if missing := containsAll(r.payloads[1],
		deps.Manifest.RetryInstruction, // instruction slot swapped
		"DIFFCONTENT",                  // diff STAYS
	); missing != "" {
		t.Errorf("2nd payload missing %q (corrective retry must keep diff + swap instruction)\n--got--\n%s", missing, r.payloads[1])
	}
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT rescue on success\n--got--\n%s", stderr.String())
	}
}

// TestCommitStaged_ParseFailAfterInner proves the inner-budget-exhaustion
// path: both Parse attempts (the full 2-try inner budget) return ok=false, so
// no parseable message is produced → Rescue(candidate="") + ErrRescue. Run is
// called exactly twice (the inner 2-try budget), not multiplied across the
// outer loop.
func TestCommitStaged_ParseFailAfterInner(t *testing.T) {
	g := &stubGit{}
	r := &stubRunner{responses: []stubResponse{
		{stdout: "x", msg: "", ok: false},
		{stdout: "y", msg: "", ok: false},
	}}
	deps := baseDeps(g, r, 3)
	deps.Manifest = provider.Manifest{Name: "stub", RetryInstruction: "retry harder"}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Output = newTestOutput(stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if !errors.Is(err, ErrRescue) {
		t.Fatalf("CommitStaged error = %v; want ErrRescue", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("Result.CommitSHA = %q; want empty on failure", res.CommitSHA)
	}
	if r.runCalls != 2 {
		t.Errorf("Run call count = %d; want 2 (the fresh inner 2-try budget, exhausted)", r.runCalls)
	}
	if !rescueRendered(stderr) {
		t.Errorf("stderr must contain the rescue block on parse-fail\n--got--\n%s", stderr.String())
	}
	// candidate is "" on parse-fail (no valid message): no candidate line.
	if strings.Contains(stderr.String(), "candidate message") {
		t.Errorf("stderr must NOT contain the candidate line for candidate=\"\"\n--got--\n%s", stderr.String())
	}
	// CommitTree/CAS were never reached.
	if g.commitTreeCalls != 0 || g.casCalls != 0 {
		t.Errorf("CommitTree=%d CAS=%d; want 0/0 (failure before COMMIT)", g.commitTreeCalls, g.casCalls)
	}
}

// TestCommitStaged_DupExhaustion proves the OUTER budget exhaustion path: every
// generated subject is a duplicate, so the loop runs the full 1 initial + N
// retries. With MaxDuplicateRetries=3 that is EXACTLY 4 Run calls, then
// Rescue(candidate=last msg) + ErrRescue. The candidate line must appear.
func TestCommitStaged_DupExhaustion(t *testing.T) {
	g := &stubGit{}
	r := &stubRunner{responses: []stubResponse{{stdout: "feat: dup", msg: "feat: dup", ok: true}}}
	deps := baseDeps(g, r, 3)          // MaxDuplicateRetries = 3
	g.subjects = []string{"feat: dup"} // every generated subject collides
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Output = newTestOutput(stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if !errors.Is(err, ErrRescue) {
		t.Fatalf("CommitStaged error = %v; want ErrRescue", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("Result.CommitSHA = %q; want empty on dup-exhaustion", res.CommitSHA)
	}
	// ★ The exact counting contract: 1 initial + MaxDuplicateRetries retries.
	if r.runCalls != 4 {
		t.Errorf("Run call count = %d; want EXACTLY 4 (1 initial + 3 retries)", r.runCalls)
	}
	if !rescueRendered(stderr) {
		t.Errorf("stderr must contain the rescue block on dup-exhaustion\n--got--\n%s", stderr.String())
	}
	// candidate = the last msg (a valid-but-rejected message): the candidate
	// line must appear with the message text.
	if missing := containsAll(stderr.String(), "candidate message", "feat: dup"); missing != "" {
		t.Errorf("stderr missing %q (dup-exhaustion must pass candidate=msg)\n--got--\n%s", missing, stderr.String())
	}
}

// TestCommitStaged_NothingToCommit proves the empty-diff gate (FR5): when
// StagedDiff returns "" CommitStaged returns ErrNothingToCommit WITHOUT taking
// a snapshot (WriteTree is NOT called) and WITHOUT rendering Rescue.
func TestCommitStaged_NothingToCommit(t *testing.T) {
	g := &stubGit{}
	r := &stubRunner{responses: []stubResponse{{stdout: "ignored", msg: "ignored", ok: true}}}
	deps := baseDeps(g, r, 3)
	g.diff = "" // nothing staged
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Output = newTestOutput(stdout, stderr)

	_, err := CommitStaged(context.Background(), deps)
	if !errors.Is(err, ErrNothingToCommit) {
		t.Fatalf("CommitStaged error = %v; want ErrNothingToCommit", err)
	}
	if g.writeTreeCalls != 0 {
		t.Errorf("WriteTree call count = %d; want 0 (no snapshot on empty diff)", g.writeTreeCalls)
	}
	if r.runCalls != 0 {
		t.Errorf("Run call count = %d; want 0 (generation never starts on empty diff)", r.runCalls)
	}
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT rescue on empty diff (no snapshot)\n--got--\n%s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must be empty on nothing-to-commit\n--got--\n%s", stdout.String())
	}
}

// TestCommitStaged_WriteTreeErrorReturnedDirectly proves a WriteTree failure
// (FR8 conflict) is returned DIRECTLY — NOT wrapped as ErrRescue — because no
// tree was created, so there is nothing to rescue. Rescue is NOT rendered.
func TestCommitStaged_WriteTreeErrorReturnedDirectly(t *testing.T) {
	g := &stubGit{}
	r := &stubRunner{responses: []stubResponse{{stdout: "x", msg: "x", ok: true}}}
	deps := baseDeps(g, r, 3)
	writeErr := errors.New("git: unresolved merge conflicts in index")
	g.writeTreeErr = writeErr
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Output = newTestOutput(stdout, stderr)

	_, err := CommitStaged(context.Background(), deps)
	if !errors.Is(err, writeErr) {
		t.Errorf("CommitStaged error = %v; want the WriteTree error returned directly (NOT ErrRescue)", err)
	}
	if errors.Is(err, ErrRescue) {
		t.Errorf("WriteTree error must NOT be reported as ErrRescue (no tree to rescue)")
	}
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT rescue on a WriteTree failure (no snapshot)\n--got--\n%s", stderr.String())
	}
	if g.writeTreeCalls != 1 {
		t.Errorf("WriteTree call count = %d; want 1", g.writeTreeCalls)
	}
}

// TestCommitStaged_Timeout proves the timeout path: Run returns a
// *provider.TimeoutError (wraps context.DeadlineExceeded, NOT context.Canceled)
// so Rescue IS rendered (candidate="") and ErrRescue is returned.
func TestCommitStaged_Timeout(t *testing.T) {
	g := &stubGit{}
	r := &stubRunner{responses: []stubResponse{{
		stdout: "",
		err:    &provider.TimeoutError{Deadline: time.Now()},
	}}}
	deps := baseDeps(g, r, 3)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Output = newTestOutput(stdout, stderr)

	_, err := CommitStaged(context.Background(), deps)
	if !errors.Is(err, ErrRescue) {
		t.Fatalf("CommitStaged error = %v; want ErrRescue", err)
	}
	if !rescueRendered(stderr) {
		t.Errorf("stderr must contain the rescue block on timeout\n--got--\n%s", stderr.String())
	}
	// candidate is "" on timeout: no candidate line.
	if strings.Contains(stderr.String(), "candidate message") {
		t.Errorf("stderr must NOT contain the candidate line on timeout\n--got--\n%s", stderr.String())
	}
}

// TestCommitStaged_SignalCancelNoDoubleRescue proves the double-rescue guard
// (Ambiguity #5): when Run returns a BARE context.Canceled (the signal path),
// the signal handler already rendered Rescue + exited, so CommitStaged returns
// ErrRescue WITHOUT rendering Rescue a second time (stderr stays clean of the
// rescue block).
func TestCommitStaged_SignalCancelNoDoubleRescue(t *testing.T) {
	g := &stubGit{}
	r := &stubRunner{responses: []stubResponse{{
		stdout: "",
		err:    context.Canceled,
	}}}
	deps := baseDeps(g, r, 3)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Output = newTestOutput(stdout, stderr)

	_, err := CommitStaged(context.Background(), deps)
	if !errors.Is(err, ErrRescue) {
		t.Fatalf("CommitStaged error = %v; want ErrRescue (signal path)", err)
	}
	// ★ The defining assertion: CommitStaged must NOT render Rescue on the
	// signal-cancel path (the handler already did).
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT contain the rescue block on signal-cancel (double-rescue guard)\n--got--\n%s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must be empty on signal-cancel\n--got--\n%s", stdout.String())
	}
}

// TestCommitStaged_HeadMoved proves the CAS-conflict path (§13.5): when
// UpdateRefCAS fails (HEAD moved concurrently), CommitStaged prints the
// head-moved message + manual recovery to stderr and returns ErrHeadMoved —
// NEVER --force and NEVER Rescue. CommitTree WAS called (the commit object
// exists); restoreSignalHandler already ran (it is called before the CAS).
func TestCommitStaged_HeadMoved(t *testing.T) {
	g := &stubGit{}
	r := &stubRunner{responses: []stubResponse{{stdout: "feat: thing", msg: "feat: thing", ok: true}}}
	deps := baseDeps(g, r, 3)
	g.casErr = errors.New("git exited 128 (update-ref HEAD ...): ... is at X but expected Y")
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Output = newTestOutput(stdout, stderr)

	_, err := CommitStaged(context.Background(), deps)
	if !errors.Is(err, ErrHeadMoved) {
		t.Fatalf("CommitStaged error = %v; want ErrHeadMoved", err)
	}
	// CommitTree was called (the commit object was created before the CAS).
	if g.commitTreeCalls != 1 {
		t.Errorf("CommitTree call count = %d; want 1 (commit created before CAS)", g.commitTreeCalls)
	}
	// The §13.5 head-moved message + manual recovery to stderr.
	if missing := containsAll(stderr.String(),
		"HEAD moved while generating",
		"aborting to avoid a non-fast-forward",
		"Your generated message was: feat: thing",
		"git commit-tree",
		"xargs git update-ref HEAD",
		"-p "+g.parentSHA, // non-root keeps -p
	); missing != "" {
		t.Errorf("stderr missing %q (§13.5 head-moved message)\n--got--\n%s", missing, stderr.String())
	}
	// NEVER Rescue on the head-moved path.
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT contain the rescue block on head-moved\n--got--\n%s", stderr.String())
	}
	// stdout is clean (failure path; nothing printed to stdout).
	if stdout.Len() != 0 {
		t.Errorf("stdout must be empty on head-moved\n--got--\n%s", stdout.String())
	}
}

// TestCommitStaged_RootCommit proves the root-commit path: RevParseHEAD returns
// hasParent=false (parentSHA=""), so CommitTree is called with parent="" (omits
// -p) and UpdateRefCAS uses expected="" (the 1-arg no-expected form, legal ONLY
// for the root commit). The newRepo gate (count<=1) is also exercised.
func TestCommitStaged_RootCommit(t *testing.T) {
	g := &stubGit{}
	r := &stubRunner{responses: []stubResponse{{stdout: "feat: initial commit", msg: "feat: initial commit", ok: true}}}
	deps := baseDeps(g, r, 3)
	// Override to the root-commit shape AFTER baseDeps (else baseDeps' defaults win).
	g.hasParent = false // unborn repo → root commit
	g.parentSHA = ""
	g.count = 0 // newRepo gate: count<=1 → §17.2 conventional-commit prompt
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Output = newTestOutput(stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if err != nil {
		t.Fatalf("CommitStaged returned error %v; want nil (root commit succeeds)", err)
	}
	if res.CommitSHA != g.newSHA {
		t.Errorf("Result.CommitSHA = %q; want %q", res.CommitSHA, g.newSHA)
	}
	// ★ Root commit: CommitTree called with parent="".
	if g.commitTreeParent != "" {
		t.Errorf("CommitTree parent = %q; want \"\" (root commit omits -p)", g.commitTreeParent)
	}
	// ★ Root commit: UpdateRefCAS called with expected="" (1-arg form).
	if g.casExpected != "" {
		t.Errorf("CAS expected = %q; want \"\" (root commit, 1-arg no-expected form)", g.casExpected)
	}
	if g.casRef != "HEAD" || g.casNew != g.newSHA {
		t.Errorf("CAS = (%q,%q); want (HEAD, %q)", g.casRef, g.casNew, g.newSHA)
	}
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT rescue on success\n--got--\n%s", stderr.String())
	}
	// FR42 success print still fires for the root commit.
	if !strings.Contains(stdout.String(), "[newsha0] feat: initial commit") {
		t.Errorf("stdout missing the FR42 success line for root commit\n--got--\n%s", stdout.String())
	}
}

// TestCommitStaged_DryRun proves the FR49 dry-run seam (P1.M7.T1.S1): with
// deps.DryRun=true the full pipeline runs (diff→snapshot→generate→parse→
// dedupe) and yields a UNIQUE message, but CommitStaged short-circuits BEFORE
// commit-tree/update-ref. Asserts: Result.CommitSHA=="" (no commit/ref
// mutation), Subject+Message carry the generated message, the message is
// printed to stdout, and crucially CommitTree + UpdateRefCAS are NEVER called
// (the dangling tree would be unreachable, but the refs must not move) and NO
// rescue is rendered (the run succeeded). The zero-value (DryRun=false) path
// is covered by TestCommitStaged_HappyPath above, so this test isolates the
// seam behavior.
func TestCommitStaged_DryRun(t *testing.T) {
	g := &stubGit{}
	r := &stubRunner{responses: []stubResponse{{stdout: "feat: dry-run change", msg: "feat: dry-run change", ok: true}}}
	deps := baseDeps(g, r, 3)
	deps.DryRun = true // FR49: run the pipeline, skip the commit
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Output = newTestOutput(stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if err != nil {
		t.Fatalf("CommitStaged returned error %v; want nil (dry-run succeeds)", err)
	}
	// ★ CommitSHA must be empty — NO commit/ref mutation.
	if res.CommitSHA != "" {
		t.Errorf("Result.CommitSHA = %q; want \"\" (dry-run must NOT commit)", res.CommitSHA)
	}
	if res.Subject != "feat: dry-run change" || res.Message != "feat: dry-run change" {
		t.Errorf("Result = {%q,%q}; want the generated message in Subject+Message", res.Subject, res.Message)
	}
	// ★ The defining assertion: the commit plumbing is NEVER reached.
	if g.commitTreeCalls != 0 {
		t.Errorf("CommitTree call count = %d; want 0 (dry-run short-circuits before commit-tree)", g.commitTreeCalls)
	}
	if g.casCalls != 0 {
		t.Errorf("UpdateRefCAS call count = %d; want 0 (dry-run short-circuits before update-ref)", g.casCalls)
	}
	// The message is printed to stdout (byte-clean for `| tee`).
	if !strings.Contains(stdout.String(), "feat: dry-run change") {
		t.Errorf("stdout must contain the dry-run message\n--got--\n%s", stdout.String())
	}
	// Dry-run is a SUCCESS path: NO rescue.
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT rescue on dry-run (success path)\n--got--\n%s", stderr.String())
	}
}

// TestCommitStaged_SystemExtra proves the SystemExtra seam (P1.M7.T1.S1): when
// deps.SystemExtra is non-empty it is appended to the built system prompt and
// delivered to the agent's Run call. Asserts r.lastSys CONTAINS the extra
// text (the rest of the prompt is exercised by the prompt-builder tests).
// Empty SystemExtra (the M6.T1.S1 default) is a no-op, covered by every other
// test in this file.
func TestCommitStaged_SystemExtra(t *testing.T) {
	g := &stubGit{}
	r := &stubRunner{responses: []stubResponse{{stdout: "feat: thing", msg: "feat: thing", ok: true}}}
	deps := baseDeps(g, r, 3)
	deps.SystemExtra = "X-TRA-PROJECT-CONVENTION"
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Output = newTestOutput(stdout, stderr)

	res, err := CommitStaged(context.Background(), deps)
	if err != nil {
		t.Fatalf("CommitStaged returned error %v; want nil", err)
	}
	if res.CommitSHA != g.newSHA {
		t.Errorf("Result.CommitSHA = %q; want %q (SystemExtra must not block the commit)", res.CommitSHA, g.newSHA)
	}
	// ★ The extra guidance reached the agent's system prompt.
	if !strings.Contains(r.lastSys, "X-TRA-PROJECT-CONVENTION") {
		t.Errorf("delivered system prompt = %q; want it to contain the SystemExtra text", r.lastSys)
	}
	if rescueRendered(stderr) {
		t.Errorf("stderr must NOT rescue on success\n--got--\n%s", stderr.String())
	}
}

// TestCommitStaged_VerboseRetryMarkers proves the FR50 verbose sink
// (Deps.Output with verbose=true) emits the generation-attempt and
// duplicate-subject retry markers to stderr across the two-nested-loop, and
// that verbose=false leaves stderr empty of traces (byte-clean). It is the
// generate-side complement to provider.TestRun_VerboseEmitsResolvedCommandAndRawStdout:
// the executor owns the resolved-command + raw-stdout traces; generate owns the
// loop/retry markers, so this test does NOT assert on raw stdout.
func TestCommitStaged_VerboseRetryMarkers(t *testing.T) {
	t.Run("verbose emits retry markers to stderr", func(t *testing.T) {
		g := &stubGit{}
		r := &stubRunner{responses: []stubResponse{
			{stdout: "feat: dup", msg: "feat: dup", ok: true},       // collides → OUTER retry
			{stdout: "feat: unique", msg: "feat: unique", ok: true}, // unique → COMMIT
		}}
		deps := baseDeps(g, r, 3)
		g.subjects = []string{"feat: dup"} // first generated subject collides
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		deps.Output = ui.NewOutput(stdout, stderr, true, true) // verbose=true

		if _, err := CommitStaged(context.Background(), deps); err != nil {
			t.Fatalf("CommitStaged returned error %v; want nil", err)
		}
		got := stderr.String()
		// At least one generation-attempt marker (the inner-loop top).
		if !strings.Contains(got, "generation attempt") {
			t.Errorf("stderr missing %q\n--got--\n%s", "generation attempt", got)
		}
		// The duplicate-subject marker from the OUTER rejection.
		if !strings.Contains(got, "duplicate subject") {
			t.Errorf("stderr missing %q\n--got--\n%s", "duplicate subject", got)
		}
		if !strings.Contains(got, "feat: dup") {
			t.Errorf("stderr missing the rejected subject %q\n--got--\n%s", "feat: dup", got)
		}
		// FR51: verbose touches stderr ONLY — stdout must carry the FR42
		// success block and nothing from the verbose traces.
		if !strings.Contains(stdout.String(), "feat: unique") {
			t.Errorf("stdout missing the FR42 success line\n--got--\n%s", stdout.String())
		}
		if strings.Contains(stdout.String(), "resolved command") ||
			strings.Contains(stdout.String(), "generation attempt") {
			t.Errorf("stdout leaked a verbose trace (FR51 violation)\n--got--\n%s", stdout.String())
		}
	})

	t.Run("verbose=false leaves stderr empty of traces", func(t *testing.T) {
		g := &stubGit{}
		r := &stubRunner{responses: []stubResponse{
			{stdout: "feat: dup", msg: "feat: dup", ok: true},
			{stdout: "feat: unique", msg: "feat: unique", ok: true},
		}}
		deps := baseDeps(g, r, 3)
		g.subjects = []string{"feat: dup"}
		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		deps.Output = newTestOutput(stdout, stderr) // verbose=false, noColor=true

		if _, err := CommitStaged(context.Background(), deps); err != nil {
			t.Fatalf("CommitStaged returned error %v; want nil", err)
		}
		// Verbose is off: NO verbose markers on stderr (success path renders
		// nothing to stderr at all, so it stays empty).
		if strings.Contains(stderr.String(), "generation attempt") ||
			strings.Contains(stderr.String(), "duplicate subject") ||
			strings.Contains(stderr.String(), "resolved command") {
			t.Errorf("stderr must be empty of verbose traces with verbose=false\n--got--\n%s", stderr.String())
		}
	})
}
