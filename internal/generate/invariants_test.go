// §20.2 Property/invariant test suite — the executable form of PRD §20.2.
//
// Drives generate.CommitStaged through every post-snapshot §18.2 failure path
// (parse fail, timeout, CAS fail, SIGINT/context-cancel, plus duplicate-exhaustion
// and agent-nonzero-exit) using the stub provider (internal/stubtest) against a real
// git binary in a temp repo, and asserts the THREE §20.2 invariants via raw git
// queries before/after:
//
//	(I1) Idempotent index  — git diff --cached --name-only AND git diff --cached
//	      byte-identical to the pre-run snapshot.
//	(I2) Atomic HEAD       — git rev-parse HEAD unchanged by Stagecoach
//	      (CAS: == the externally-moved concurrent commit).
//	(I3) Snapshot immutability — git cat-file -p <TREE_SHA> byte-identical
//	      AFTER staging extra content into the index.
//
// Refs: PRD §18.1 (invariant), §18.2 (failure modes), §20.2 (property tests).
package generate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/stubtest"
)

// repoSnapshot captures the pre-run repo state for the §20.2 invariant comparison.
type repoSnapshot struct {
	head       string // git rev-parse HEAD
	indexNames string // git diff --cached --name-only  (I1: names)
	indexFull  string // git diff --cached              (I1: byte-for-byte)
}

// snapshotRepo runs the three git queries that capture the repo state before a
// CommitStaged run. Used by the harness to capture the "before" state for the
// §20.2 invariant assertions.
func snapshotRepo(t *testing.T, repo string) repoSnapshot {
	t.Helper()
	return repoSnapshot{
		head:       headSHA(t, repo),
		indexNames: gitOut(t, repo, "diff", "--cached", "--name-only"),
		indexFull:  gitOut(t, repo, "diff", "--cached"),
	}
}

// treeSHAFromErr extracts the TreeSHA from a post-snapshot error: either
// *RescueError (parse-fail / timeout / SIGINT / duplicate / nonzero) or
// *CASError (CAS failure). Fails the test if neither matches or the SHA is empty
// (an empty SHA would indicate a pre-snapshot failure, which is out of scope).
func treeSHAFromErr(t *testing.T, err error) string {
	t.Helper()
	var re *RescueError
	if errors.As(err, &re) {
		if re.TreeSHA == "" {
			t.Fatalf("RescueError.TreeSHA empty (pre-snapshot failure? out of scope): %v", err)
		}
		return re.TreeSHA
	}
	var ce *CASError
	if errors.As(err, &ce) {
		if ce.TreeSHA == "" {
			t.Fatalf("CASError.TreeSHA empty: %v", err)
		}
		return ce.TreeSHA
	}
	t.Fatalf("error is neither *RescueError nor *CASError (got %T): %v", err, err)
	return "" // unreachable
}

// assertInvariants is the §20.2 "property": asserts all three invariants (I1+I2+I3)
// against the post-run repo state. Every scenario subtest funnels through this, so
// the invariants are asserted uniformly and can never be silently dropped.
//
//	I1 — idempotent index:  names + full diff byte-identical to before.
//	I2 — atomic HEAD:       HEAD == wantHead ("" ⇒ use before.head).
//	I3 — snapshot immutability: cat-file -t == "tree"; cat-file -p stable across
//	      a subsequent stageFile of NEW content.
func assertInvariants(t *testing.T, repo string, before repoSnapshot, treeSHA, wantHead string) {
	t.Helper()

	// I1: idempotent index — names + full byte-for-byte.
	if got := gitOut(t, repo, "diff", "--cached", "--name-only"); got != before.indexNames {
		t.Errorf("I1 (idempotent index) names: got %q, want %q", got, before.indexNames)
	}
	if got := gitOut(t, repo, "diff", "--cached"); got != before.indexFull {
		t.Errorf("I1 (idempotent index) full diff mutated: before=%q after=%q", before.indexFull, got)
	}

	// I2: atomic HEAD — unchanged by Stagecoach (CAS: == the externally-moved commit).
	if wantHead == "" {
		wantHead = before.head
	}
	if got := headSHA(t, repo); got != wantHead {
		t.Errorf("I2 (atomic HEAD): got %q, want %q", got, wantHead)
	}

	// I3: snapshot immutability — cat-file -t == tree; cat-file -p stable across a subsequent stage.
	if got := gitOut(t, repo, "cat-file", "-t", treeSHA); got != "tree" {
		t.Fatalf("I3 (snapshot immutability): cat-file -t %q = %q, want \"tree\" (object missing?)", treeSHA, got)
	}
	treeBefore := gitOut(t, repo, "cat-file", "-p", treeSHA)
	writeFile(t, repo, "immutable_probe.txt", "must not alter the frozen snapshot tree")
	stageFile(t, repo, "immutable_probe.txt")
	treeAfter := gitOut(t, repo, "cat-file", "-p", treeSHA)
	if treeAfter != treeBefore {
		t.Errorf("I3 (snapshot immutability): cat-file -p %q changed after staging (content-addressing violated)", treeSHA)
	}
}

// scenario is one row of the TestInvariants table: a named §18.2 failure mode
// plus the closure that drives CommitStaged into it and reports the expected
// post-run HEAD.
type scenario struct {
	name string
	run  func(t *testing.T, ctx context.Context, repo, bin string) (err error, wantHead string)
}

// moveHEADWithoutStaging creates a commit via plumbing (commit-tree +
// update-ref) using the current HEAD's tree, so the index remains unchanged.
// A porcelain commit (--allow-empty) would commit any staged changes,
// violating the idempotent-index invariant's pre-condition.
func moveHEADWithoutStaging(t *testing.T, repo, msg string) string {
	t.Helper()
	parent := headSHA(t, repo)
	// Use the parent commit's tree (not write-tree from the index) so the
	// concurrent commit doesn't include staged changes. This keeps the
	// index→HEAD diff unchanged (staged.txt still shows as new).
	parentTree := gitOut(t, repo, "rev-parse", parent+"^{tree}")
	newSHA := gitOut(t, repo, "commit-tree", parentTree, "-p", parent, "-m", msg)
	gitOut(t, repo, "update-ref", "HEAD", newSHA, parent) // CAS move HEAD
	return newSHA
}

// TestInvariants is the §20.2 table-driven property suite. For each failure mode,
// it creates a fresh temp repo with a staged file, snapshots the pre-run state,
// drives CommitStaged to the failure via the stub, then asserts all three
// invariants (I1+I2+I3).
func TestInvariants(t *testing.T) {
	bin := stubtest.Build(t)

	scenarios := []scenario{
		{
			name: "ParseFail",
			run: func(t *testing.T, _ context.Context, repo, bin string) (error, string) {
				m := stubtest.NewScript(t, bin, []string{""})
				cfg := config.Defaults()
				cfg.MaxDuplicateRetries = 0
				_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
				if err != nil {
					var re *RescueError
					if !errors.As(err, &re) || !errors.Is(err, ErrRescue) {
						t.Errorf("ParseFail: want *RescueError/ErrRescue, got %T %v", err, err)
					}
				}
				return err, ""
			},
		},
		{
			name: "DuplicateExhaustion",
			run: func(t *testing.T, _ context.Context, repo, bin string) (error, string) {
				// The harness's HEAD subject is "initial" (from commitRaw("initial")).
				// The stub returns "initial" — matching the HEAD subject → duplicate → retry
				// exhausted (retries=0) → *RescueError{ErrRescue}. No additional commits needed;
				// the index (staged.txt) is untouched.
				m := stubtest.NewScript(t, bin, []string{"initial"})
				cfg := config.Defaults()
				cfg.MaxDuplicateRetries = 0
				_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
				if err != nil {
					var re *RescueError
					if !errors.As(err, &re) || !errors.Is(err, ErrRescue) {
						t.Errorf("DuplicateExhaustion: want *RescueError/ErrRescue, got %T %v", err, err)
					}
				}
				return err, ""
			},
		},
		{
			name: "AgentNonzeroExit",
			run: func(t *testing.T, _ context.Context, repo, bin string) (error, string) {
				m := stubtest.Manifest(bin, stubtest.Options{Exit: 2, Out: ""})
				cfg := config.Defaults()
				cfg.MaxDuplicateRetries = 0
				_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
				if err != nil {
					var re *RescueError
					if !errors.As(err, &re) || !errors.Is(err, ErrRescue) {
						t.Errorf("AgentNonzeroExit: want *RescueError/ErrRescue, got %T %v", err, err)
					}
				}
				return err, ""
			},
		},
		{
			name: "Timeout",
			run: func(t *testing.T, _ context.Context, repo, bin string) (error, string) {
				if testing.Short() {
					t.Skip("skipping slow Timeout scenario in -short mode")
				}
				m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: slow", SleepMS: 2000})
				cfg := config.Defaults()
				cfg.Timeout = 150 * time.Millisecond
				_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
				if err != nil {
					if !errors.Is(err, ErrTimeout) {
						t.Errorf("Timeout: errors.Is(err, ErrTimeout) = false, got %v", err)
					}
				}
				return err, ""
			},
		},
		{
			name: "SigintContextCancel",
			run: func(t *testing.T, ctx context.Context, repo, bin string) (error, string) {
				if testing.Short() {
					t.Skip("skipping slow SigintContextCancel scenario in -short mode")
				}
				m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: hang", SleepMS: 3000})
				cctx, cancel := context.WithCancel(ctx)
				done := make(chan error, 1)
				go func() {
					_, e := CommitStaged(cctx, Deps{Git: git.New(repo), Manifest: m}, config.Defaults())
					done <- e
				}()
				// Let snapshot (step 3) + generation start — snapshot is microseconds,
				// stub sleeps 3000ms → snapshot is long done → TreeSHA is non-empty.
				time.Sleep(150 * time.Millisecond)
				cancel() // simulate SIGINT: ctx cancelled → Execute returns context.Canceled
				err := <-done
				if err != nil {
					if !errors.Is(err, ErrRescue) {
						t.Errorf("SigintContextCancel: errors.Is(err, ErrRescue) = false, got %v", err)
					}
				}
				return err, ""
			},
		},
		{
			name: "CASFailure",
			run: func(t *testing.T, _ context.Context, repo, bin string) (error, string) {
				if testing.Short() {
					t.Skip("skipping slow CAS scenario in -short mode")
				}
				parent := headSHA(t, repo)
				m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: x", SleepMS: 400})
				done := make(chan error, 1)
				go func() {
					_, e := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, config.Defaults())
					done <- e
				}()
				// Let the orchestrator snapshot + enter generation (stub sleeping 400ms).
				time.Sleep(150 * time.Millisecond)
				// Move HEAD mid-generation via plumbing (commit-tree + update-ref) so the
				// index is NOT touched. A porcelain commit (--allow-empty) would commit
				// staged.txt, clearing the index and violating I1.
				concurrent := moveHEADWithoutStaging(t, repo, "concurrent commit")
				err := <-done
				if err != nil {
					var ce *CASError
					if !errors.As(err, &ce) || !errors.Is(err, ErrCASFailed) {
						t.Fatalf("CAS: want *CASError/ErrCASFailed, got %T %v", err, err)
					}
					if ce.Expected != parent || ce.Actual != concurrent {
						t.Errorf("CAS context: expected=%q actual=%q; want %q/%q", ce.Expected, ce.Actual, parent, concurrent)
					}
				}
				// wantHead = concurrent: HEAD == the externally-moved commit
				// (orchestrator did NOT land/force — atomic HEAD special case).
				return err, concurrent
			},
		},
	}

	for _, sc := range scenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			// Shared fixture: born repo + initial commit + one staged file.
			repo := t.TempDir()
			initRepo(t, repo)
			commitRaw(t, repo, "initial")
			writeFile(t, repo, "staged.txt", "snapshotted content")
			stageFile(t, repo, "staged.txt")

			before := snapshotRepo(t, repo)
			err, wantHead := sc.run(t, context.Background(), repo, bin)
			if err == nil {
				t.Fatal("expected a failure-path error, got nil")
			}
			treeSHA := treeSHAFromErr(t, err)
			assertInvariants(t, repo, before, treeSHA, wantHead)
		})
	}
}
