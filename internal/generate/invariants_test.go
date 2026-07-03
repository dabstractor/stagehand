// This file is the PRD §20.2 layer-4 property/invariant suite for the
// snapshot-based atomic-commit orchestrator generate.CommitStaged
// (P1.M6.T3.S3). It is white-box `package generate` (NO build tag — it runs in
// normal CI exactly like integration_test.go) and is a CONSUMER of the
// P1.M6.T3.S2 in-package harness: it reuses, WITHOUT re-implementation,
// newTempRepo, writeStage, seedCommit, e2eDeps, headMoverRunner,
// NewStubManifest / StubConfig / StubResponse, and the assertion helpers
// headSHA, stagedFiles, gitRun, commitParentLine, containsAll from the sibling
// _test.go files. It adds the three EXHAUSTIVE §18.1 property assertions
// (idempotent index across every failure path, atomic HEAD on the CAS-fail
// path, snapshot immutability under concurrent staging) plus the source-level
// static tripwires (go/ast over the production git source: never git commit,
// never --force, 1-arg update-ref only reachable via the root-commit guard).
//
// PRD §18.1 (the invariant): "The repository's refs and index are modified
// only at the final update-ref step, and only if HEAD is unchanged since the
// snapshot. Every code path that does not reach a successful update-ref leaves
// the repository byte-for-byte unchanged (modulo harmless dangling objects)."
// decisions.md §7 enumerates the three properties (idempotent index / atomic
// HEAD / snapshot immutability) and the three prohibitions (never git commit,
// never --force, never update-ref without the expected-old except root). These
// tests are that invariant's executable, auditable proof — a regression to it
// fails loudly and specifically.
//
// Atomic-HEAD nuance (research §4): the CAS-failure is CAUSED by a concurrent
// HEAD movement (headMoverRunner advances HEAD during Run), so beforeHead !=
// afterHead BY CONSTRUCTION. The §20.2 phrasing "HEAD unchanged after a CAS
// failure" therefore means Stagehand's failed update-ref was a NO-OP (git
// refused the swap) — HEAD is left where the concurrent mover put it, NOT
// force-advanced to the stagehand commit. The stagehand commit object exists
// but is DANGLING (unreachable from HEAD). The atomic-HEAD test asserts exactly
// that, NOT beforeHead==afterHead.
//
// Dependencies: stdlib (bytes/context/errors/go/ast/go/parser/go/token/os/
// path-filepath/strconv/strings/testing/time) + internal/{config,git,provider,
// ui} + the in-package S2/S1 helpers ONLY. go/ast + go/parser are STDLIB (no
// go.mod change). NO testify, NO go-git, NO build tag.
package generate

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/ui"
)

// ---------------------------------------------------------------------------
// §20.2 "Idempotent index" — the staged snapshot is byte-identical before==after
// a failing CommitStaged on EVERY failure path (parse-fail/rescue, timeout,
// dup-exhaustion, CAS-fail).
// ---------------------------------------------------------------------------

// TestInvariants_IdempotentIndexAcrossFailurePaths proves §20.2 "Idempotent
// index" across EVERY failure path: the staged index snapshot
// (git diff --cached --name-only) is byte-for-byte UNCHANGED before==after a
// failing CommitStaged — on parse-fail/rescue, timeout, dup-exhaustion, AND
// CAS-fail. The CAS row is INCLUDED because headMoverRunner advances HEAD via
// HEAD^{tree} (NOT the index), so the staged snapshot is untouched even though
// HEAD moves; this is the invariant that distinguishes Stagehand from porcelain
// that would consume/commit the index. (The atomic-HEAD property for the CAS
// path — the stagehand commit is dangling — is asserted by the dedicated
// TestInvariant_AtomicHEAD_CASFailureIsNoOp.)
func TestInvariants_IdempotentIndexAcrossFailurePaths(t *testing.T) {
	// One row per failure path. buildDeps wires the FULL real pipeline (real
	// *git.Git + real *provider.Executor + stub Manifest) for the non-CAS paths
	// via the shared e2eDeps, and the headMoverRunner decorator for CAS (the
	// only path that moves HEAD mid-run). extraSeed lets dup-exhaustion make its
	// emitted subject a recent-commit duplicate.
	rows := []struct {
		name      string
		wantErr   error
		extraSeed string // additional subject to seed before the run (dup-exhaust)
		buildDeps func(t *testing.T, dir string, stdout, stderr *bytes.Buffer) Deps
	}{
		{
			name:    "parse-fail",
			wantErr: ErrRescue,
			// Decision D (S2 research §4): under RAW output a NON-EMPTY string
			// parses ok=true and would COMMIT; Emit:"" is the only way to force
			// a parse miss. Both inner-try entries empty ⇒ inner budget
			// exhausted ⇒ Rescue("") + ErrRescue.
			buildDeps: func(t *testing.T, dir string, stdout, stderr *bytes.Buffer) Deps {
				return e2eDeps(t, dir, []StubResponse{{Emit: ""}, {Emit: ""}}, config.Default(), stdout, stderr)
			},
		},
		{
			name:    "timeout",
			wantErr: ErrRescue,
			// A hanging stub + a short Timeout fires the executor's ctx deadline
			// ⇒ *TimeoutError ⇒ Rescue("") + ErrRescue.
			buildDeps: func(t *testing.T, dir string, stdout, stderr *bytes.Buffer) Deps {
				cfg := config.Default()
				cfg.Timeout = 500 * time.Millisecond
				return e2eDeps(t, dir, []StubResponse{{Hang: true}}, cfg, stdout, stderr)
			},
		},
		{
			name:      "dup-exhaustion",
			wantErr:   ErrRescue,
			extraSeed: "feat: dup", // makes every emitted subject a duplicate
			// 1 initial + MaxDuplicateRetries retries (default 4 total). Every
			// emitted subject collides ⇒ outer budget exhausted ⇒
			// Rescue(candidate=msg) + ErrRescue.
			buildDeps: func(t *testing.T, dir string, stdout, stderr *bytes.Buffer) Deps {
				cfg := config.Default()
				script := make([]StubResponse, cfg.MaxDuplicateRetries+1)
				for i := range script {
					script[i] = StubResponse{Emit: "feat: dup"}
				}
				return e2eDeps(t, dir, script, cfg, stdout, stderr)
			},
		},
		{
			name:    "cas-fail",
			wantErr: ErrHeadMoved,
			// headMoverRunner moves HEAD DURING Run (plumbing on HEAD^{tree},
			// never git commit --allow-empty — that would corrupt the staged
			// index), so the final UpdateRefCAS(expected=parentSHA) fails ⇒
			// ErrHeadMoved. HEAD moves but the INDEX does not — the invariant
			// under test here.
			buildDeps: func(t *testing.T, dir string, stdout, stderr *bytes.Buffer) Deps {
				g, err := git.New(dir)
				if err != nil {
					t.Fatalf("git.New(%q): %v", dir, err)
				}
				manifest := NewStubManifest(t, StubConfig{
					Script:    []StubResponse{{Emit: "feat: ok message"}},
					StateFile: filepath.Join(t.TempDir(), "c"),
				})
				return Deps{
					Git:      g,
					Runner:   &headMoverRunner{inner: provider.NewExecutor(""), tb: t, dir: dir},
					Manifest: manifest,
					Config:   config.Default(),
					Output:   ui.NewOutput(stdout, stderr, false, true),
				}
			},
		},
	}

	for _, tc := range rows {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := newTempRepo(t)
			seedCommit(t, dir, "feat: baseline")
			if tc.extraSeed != "" {
				seedCommit(t, dir, tc.extraSeed)
			}
			writeStage(t, dir, "feature.go", "package x\n")

			// Capture the staged index snapshot BEFORE CommitStaged runs.
			beforeStaged := stagedFiles(t, dir)

			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			deps := tc.buildDeps(t, dir, stdout, stderr)

			res, err := CommitStaged(context.Background(), deps)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("%s: CommitStaged error = %v; want %v", tc.name, err, tc.wantErr)
			}
			if res.CommitSHA != "" {
				t.Errorf("%s: Result.CommitSHA = %q; want empty (no commit on a failure path)", tc.name, res.CommitSHA)
			}

			// ★ §18.1 / §20.2 "Idempotent index": the staged snapshot is
			// byte-identical before==after on EVERY failure path.
			if got := stagedFiles(t, dir); got != beforeStaged {
				t.Errorf("%s: IDEMPOTENT-INDEX violated: before=%q after=%q", tc.name, beforeStaged, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// §20.2 "Atomic HEAD" — Stagehand's failed CAS is a no-op (HEAD is left where
// the concurrent mover put it, NOT force-advanced; the stagehand commit dangles).
// ---------------------------------------------------------------------------

// TestInvariant_AtomicHEAD_CASFailureIsNoOp proves §20.2 "Atomic HEAD" on the
// CAS-fail path: Stagehand's failed update-ref is a NO-OP. Because the CAS
// failure is CAUSED by a concurrent HEAD movement (headMoverRunner advances
// HEAD during Run), beforeHead != afterHead BY CONSTRUCTION — the §20.2 phrasing
// "HEAD unchanged" means Stagehand did NOT force-advance HEAD to the stagehand
// commit. So this asserts: (1) afterHead's subject is the decorator's
// "concurrent commit elsewhere" (NOT the stagehand message); (2) NO commit
// reachable from HEAD has the stagehand subject — the stagehand commit object
// is DANGLING (it was built by commit-tree but update-ref refused the swap);
// (3) afterHead's parent == beforeHead (the decorator built a child of the
// original); (4) stdout is empty and stderr carries the §13.5 head-moved
// message. It MIRRORS TestIntegration_HeadMovedCASFailure but factors out the
// INVARIANT assertions (the dangling proof + the no-force proof) for a loud,
// specific regression signal.
func TestInvariant_AtomicHEAD_CASFailureIsNoOp(t *testing.T) {
	dir := newTempRepo(t)
	seedCommit(t, dir, "feat: baseline")
	writeStage(t, dir, "feature.go", "package x\n")
	beforeHead := headSHA(t, dir)

	g, err := git.New(dir)
	if err != nil {
		t.Fatalf("git.New(%q): %v", dir, err)
	}
	const stagehandSubject = "feat: ok message"
	manifest := NewStubManifest(t, StubConfig{
		Script:    []StubResponse{{Emit: stagehandSubject}},
		StateFile: filepath.Join(t.TempDir(), "c"),
	})
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps := Deps{
		Git:      g,
		Runner:   &headMoverRunner{inner: provider.NewExecutor(""), tb: t, dir: dir},
		Manifest: manifest,
		Config:   config.Default(),
		Output:   ui.NewOutput(stdout, stderr, false, true),
	}

	res, err := CommitStaged(context.Background(), deps)
	if !errors.Is(err, ErrHeadMoved) {
		t.Fatalf("CommitStaged error = %v; want ErrHeadMoved", err)
	}
	if res.CommitSHA != "" {
		t.Errorf("Result.CommitSHA = %q; want empty (no commit reached HEAD)", res.CommitSHA)
	}

	afterHead := headSHA(t, dir)

	// (1) HEAD is the decorator's concurrent commit, NOT the stagehand commit.
	if subj := strings.TrimSpace(gitRun(t, dir, "log", "-1", "--format=%s", afterHead)); subj != "concurrent commit elsewhere" {
		t.Errorf("ATOMIC-HEAD: HEAD subject = %q; want %q (Stagehand must NOT force-advance HEAD to its own commit)", subj, "concurrent commit elsewhere")
	}

	// (2) The stagehand commit is DANGLING: it is NOT reachable from HEAD. Prove
	// the negative — no commit reachable from HEAD carries the stagehand
	// subject. (The stagehand commit object was created by commit-tree but
	// update-ref refused the CAS swap, so it is left unreachable from every ref;
	// it survives only as a harmless dangling object, exactly as §18.1 allows.)
	allSubjects := gitRun(t, dir, "log", "--format=%s", "HEAD")
	if strings.Contains(allSubjects, stagehandSubject) {
		t.Errorf("ATOMIC-HEAD: the stagehand commit (%q) is reachable from HEAD — the CAS must NOT have advanced it\n--reachable subjects--\n%s", stagehandSubject, allSubjects)
	}

	// (3) The decorator's commit is a CHILD of the original HEAD (HEAD moved
	// forward by exactly one, off beforeHead).
	if parent := commitParentLine(t, dir, afterHead); parent != beforeHead {
		t.Errorf("ATOMIC-HEAD: afterHead parent = %q; want beforeHead %q (the decorator's commit must be a child of the original)", parent, beforeHead)
	}

	// (4) stdout is clean (failure path prints nothing to stdout); stderr
	// carries the §13.5 head-moved message + manual recovery.
	if stdout.Len() != 0 {
		t.Errorf("ATOMIC-HEAD: stdout must be empty on head-moved\n--got--\n%s", stdout.String())
	}
	if missing := containsAll(stderr.String(),
		"HEAD moved while generating",
		"aborting to avoid a non-fast-forward",
		"Your generated message was: "+stagehandSubject,
	); missing != "" {
		t.Errorf("ATOMIC-HEAD: stderr missing %q (§13.5 head-moved message)\n--got--\n%s", missing, stderr.String())
	}
}

// ---------------------------------------------------------------------------
// §20.2 "Snapshot immutability" — a frozen write-tree tree object is
// byte-stable even after additional files are staged mid-run.
// ---------------------------------------------------------------------------

// TestInvariant_SnapshotImmutableAfterConcurrentStaging proves §20.2 "Snapshot
// immutability": a frozen write-tree tree object is byte-stable
// (git cat-file -p <TREE_SHA> identical) even after additional files are staged
// mid-run, while a FRESH write-tree yields a DIFFERENT SHA — proving concurrent
// staging DID mutate the index but left the OLD snapshot frozen. This is the
// content-addressable guarantee that makes write-tree a safe snapshot point:
// once a tree object is written its bytes are immutable, so a mid-generation
// `git add` (or any index mutation) cannot retroactively alter the snapshot
// the commit will be built from. It exercises the REAL shipped *git.Git.WriteTree
// seam (no orchestrator) against a REAL temp repo.
func TestInvariant_SnapshotImmutableAfterConcurrentStaging(t *testing.T) {
	dir := newTempRepo(t)
	seedCommit(t, dir, "feat: baseline")
	writeStage(t, dir, "a.go", "package a\n")

	g, err := git.New(dir)
	if err != nil {
		t.Fatalf("git.New(%q): %v", dir, err)
	}

	// Freeze the index into a tree (the snapshot point).
	treeSHA, err := g.WriteTree()
	if err != nil {
		t.Fatalf("WriteTree (snapshot): %v", err)
	}
	before := gitRun(t, dir, "cat-file", "-p", treeSHA)

	// Simulate concurrent staging mid-run: stage ANOTHER file. The index now
	// differs from the snapshot, but the snapshot object must be unchanged.
	writeStage(t, dir, "b.go", "package b\n")

	after := gitRun(t, dir, "cat-file", "-p", treeSHA)
	if after != before {
		t.Errorf("SNAPSHOT-IMMUTABILITY: frozen write-tree tree %s changed after concurrent staging:\nbefore=%q\nafter =%q", treeSHA, before, after)
	}

	// Complementary proof: a FRESH write-tree now yields a DIFFERENT SHA — the
	// index DID change (b.go is now staged), but the OLD snapshot stayed frozen.
	newTree, err := g.WriteTree()
	if err != nil {
		t.Fatalf("WriteTree (after staging): %v", err)
	}
	if newTree == treeSHA {
		t.Errorf("SNAPSHOT-IMMUTABILITY: fresh write-tree == frozen tree (%s); want a DIFFERENT SHA (proving the index changed but the old snapshot is frozen)", treeSHA)
	}
}

// ---------------------------------------------------------------------------
// Static tripwires (go/ast over the PRODUCTION git source — PRD §20.2 /
// decisions.md §7). A naive strings.Contains scan is USELESS here: the git
// source is full of comments like "Stagehand NEVER runs git commit". go/ast
// visits only AST nodes (comments are out of ast.Inspect's stream), so these
// checks are exact: they collect the string-literal arguments that could flow
// into a git command (the []string{...} arg builders + the direct args of
// g.run / exec.Command) and assert none equals "commit", none equals "--force",
// and the single "update-ref" site is guarded by 'if expected != ""'.
// ---------------------------------------------------------------------------

// gitSource is a non-test production .go file under internal/git paired with
// its source text — the input to the AST scan.
type gitSource struct {
	Name string // the path returned by filepath.Glob (e.g. "../git/plumbing.go")
	Src  string // the file's full source text (os.ReadFile)
}

// readNonTestGitSources globs the PRODUCTION internal/git sources (skipping
// *_test.go) and reads each into a gitSource. A Go test's working directory is
// its package directory (internal/generate/), so "../git/*.go" resolves to
// internal/git/*.go (go:embed cannot reach ".."). It fatals if no sources are
// found (the scan universe must be non-empty or the tripwires are vacuous).
func readNonTestGitSources(t *testing.T) []gitSource {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "git", "*.go"))
	if err != nil {
		t.Fatalf("readNonTestGitSources: glob: %v", err)
	}
	var out []gitSource
	for _, p := range paths {
		if strings.HasSuffix(filepath.Base(p), "_test.go") {
			continue // production sources only (the tripwires guard SHIPPED code)
		}
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("readNonTestGitSources: read %s: %v", p, err)
		}
		out = append(out, gitSource{Name: p, Src: string(src)})
	}
	if len(out) == 0 {
		t.Fatal("readNonTestGitSources: no non-test internal/git sources found (scan universe is empty)")
	}
	return out
}

// collectGitStringArgs parses src as Go and returns every string-literal that
// could flow into a git command: (a) the string elements of any []string{...}
// composite literal (the arg-builder form in plumbing.go —
// []string{"update-ref", ref, newSHA} / []string{"commit-tree", "-p", ...}),
// and (b) the direct string-literal args of a call to a "run" method or
// "Command" function (the g.run("diff", "--cached", ...) /
// exec.Command("git", ...) forms). Comments are NOT visited (go/ast keeps them
// out of ast.Inspect's node stream), so prose like "NEVER git commit" cannot
// false-fail the scan. Identifiers (ref/newSHA/parent/...) are not BasicLits
// and are therefore ignored. Returns literals in source order.
func collectGitStringArgs(t *testing.T, name, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("collectGitStringArgs: parse %s: %v", name, err)
	}
	var args []string
	addLit := func(lit *ast.BasicLit) {
		if lit == nil || lit.Kind != token.STRING {
			return
		}
		if s, err := strconv.Unquote(lit.Value); err == nil {
			args = append(args, s)
		}
	}
	// isStringSlice reports whether e denotes []string or [N]string (the
	// arg-builder composite type). plumbing.go uses []string exclusively.
	isStringSlice := func(e ast.Expr) bool {
		at, ok := e.(*ast.ArrayType)
		if !ok {
			return false
		}
		id, ok := at.Elt.(*ast.Ident)
		return ok && id.Name == "string"
	}
	// callName extracts the called function/method name (bare ident OR
	// selector x.Foo → "Foo") so g.run / exec.Command / run all resolve.
	callName := func(fun ast.Expr) (string, bool) {
		switch fn := fun.(type) {
		case *ast.Ident:
			return fn.Name, true
		case *ast.SelectorExpr:
			return fn.Sel.Name, true
		}
		return "", false
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CompositeLit:
			if isStringSlice(x.Type) { // []string{"update-ref", ...} arg builders
				for _, el := range x.Elts {
					if lit, ok := el.(*ast.BasicLit); ok {
						addLit(lit)
					}
				}
			}
		case *ast.CallExpr:
			// g.run("diff", ...) / run(...) / exec.Command("git", ...): collect
			// the DIRECT string-literal args (identifiers like g.git/args are
			// not BasicLits and skipped).
			if nm, ok := callName(x.Fun); ok && (nm == "run" || nm == "Command") {
				for _, a := range x.Args {
					if lit, ok := a.(*ast.BasicLit); ok {
						addLit(lit)
					}
				}
			}
		}
		return true
	})
	return args
}

// TestStatic_NeverCallsGitCommit is the source-level tripwire for decisions.md
// §7's "Never git commit" prohibition. It unions the string-literal git args
// across every non-test internal/git source and asserts NONE equals "commit"
// (the porcelain that would mutate HEAD directly). "commit-tree" is EXPECTED
// and safe ("commit-tree" ≠ "commit"). The dynamic CAS test proves the runtime
// behavior; this is the static guard so a future contributor cannot reintroduce
// `git commit` without this test failing loudly.
func TestStatic_NeverCallsGitCommit(t *testing.T) {
	for _, f := range readNonTestGitSources(t) {
		for _, a := range collectGitStringArgs(t, f.Name, f.Src) {
			if a == "commit" {
				t.Errorf("NeverGitCommit: %s builds a git command with arg %q (decisions.md §7: NEVER `git commit`; use commit-tree + update-ref)", filepath.Base(f.Name), a)
			}
		}
	}
}

// TestStatic_NeverForceUpdatesRef is the source-level tripwire for the
// "Never --force" prohibition (PRD §18.2; decisions.md §7). A forced update-ref
// would overwrite a concurrent HEAD movement — the exact bug the CAS plumbing
// was designed to structurally prevent.
func TestStatic_NeverForceUpdatesRef(t *testing.T) {
	for _, f := range readNonTestGitSources(t) {
		for _, a := range collectGitStringArgs(t, f.Name, f.Src) {
			if a == "--force" {
				t.Errorf("NeverForce: %s builds a git command with arg %q (PRD §18.2 / decisions.md §7: NEVER --force)", filepath.Base(f.Name), a)
			}
		}
	}
}

// TestStatic_UpdateRefOnlyCASGuarded is the source-level tripwire for the
// "update-ref only with the expected-old (except root)" prohibition. The
// dynamic TestInvariant_AtomicHEAD_CASFailureIsNoOp proves the runtime CAS
// behavior; this asserts at the SOURCE level that "update-ref" appears in
// exactly ONE non-test internal/git source (plumbing.go's UpdateRefCAS) and
// that file ALSO contains the guard 'if expected != ""' — i.e. the 1-arg
// no-expected form is reachable ONLY for the root commit (expected==""),
// never as a silent overwrite.
func TestStatic_UpdateRefOnlyCASGuarded(t *testing.T) {
	files := readNonTestGitSources(t)

	var refFiles []string
	var plumbing *gitSource
	for i := range files {
		for _, a := range collectGitStringArgs(t, files[i].Name, files[i].Src) {
			if a == "update-ref" {
				refFiles = append(refFiles, filepath.Base(files[i].Name))
				break
			}
		}
		if filepath.Base(files[i].Name) == "plumbing.go" {
			plumbing = &files[i]
		}
	}

	if len(refFiles) != 1 {
		t.Fatalf("UpdateRefCASGuarded: expected update-ref in exactly ONE non-test internal/git source; got %d: %v", len(refFiles), refFiles)
	}
	if refFiles[0] != "plumbing.go" {
		t.Errorf("UpdateRefCASGuarded: update-ref should live only in plumbing.go; got %q", refFiles[0])
	}
	if plumbing == nil {
		t.Fatal("UpdateRefCASGuarded: plumbing.go not found among non-test internal/git sources")
	}
	// The CAS guard: the 1-arg (no expected) form is reachable ONLY when
	// expected=="" (the root commit). Every non-root commit appends expected.
	if !strings.Contains(plumbing.Src, `if expected != ""`) {
		t.Errorf("UpdateRefCASGuarded: plumbing.go missing the CAS guard 'if expected != \"\"' (1-arg update-ref must be reachable only for the root commit)")
	}
}
