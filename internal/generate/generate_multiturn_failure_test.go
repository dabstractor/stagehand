// generate_multiturn_failure_test.go — the multi-turn FAILURE/INVARIANT integration tests (PRD §9.24
// FR-T7 + FR-T1 negative conditions b/d).
//
// UNIQUE vs T3.S3 (generate_test.go:868-941): T3.S3's skip tests assert ONLY Kind==ErrRescue; the
// duplicate test exercises a FINAL-turn dedupe failure. THESE tests add: (a) the mid-turn Execute-error
// abort path (entirely new — Run returns "",false,execErr on the FIRST turn's error), and (b)/(c) the
// full rescue invariant (TreeSHA==frozen, ParentSHA, atomic-HEAD, idempotent-index) + the stub-counter
// proof that Run was/wasn't entered.
//
// Mechanism for a mid-turn failure WITHOUT modifying the stub (forbidden by T4.S1 + "existing harness"):
// the stub has ONE exit knob — STAGEHAND_STUB_EXIT, applied to EVERY call. Setting it GLOBALLY ("1")
// makes the one-shot exit 1 BUT its stdout is still "" (script[0]) ⇒ CommitStaged's non-zero-exit
// branch falls through to ParseOutput("") ⇒ ok=false ⇒ exhausts (MaxDuplicateRetries=0) ⇒ the FR-T1
// gate fires (all 4 conds hold) ⇒ Run's turn 1 exits 1 ⇒ provider.Execute returns a wrapped
// *exec.ExitError (executor.go:77) ⇒ Run aborts (FR-T7) ⇒ the byte-identical rescue
// (generate.go:339, Kind:ErrRescue). The failure lands on turn 1 (Run aborts at the FIRST error —
// equivalent FR-T7 coverage to a later turn).
//
// The counter is the discriminator between "gate fired + Run aborted" and "gate skipped": the stub
// increments STAGEHAND_STUB_COUNTER once per process. counter=="1" ⇒ ONLY the one-shot ran (gate
// SKIPPED Run — conds b/d false); counter=="2" ⇒ one-shot + exactly 1 multi-turn turn (gate FIRED,
// Run aborted at turn 1). This is the assertion T3.S3's Kind-only skip tests LACK.
package generate

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/provider"
	"github.com/dustin/stagecoach/internal/stubtest"
)

// assertMultiTurnRescue runs CommitStaged expecting a *RescueError{Kind:ErrRescue} and asserts the
// FULL FR-T7 / idempotent invariant:
//   - TreeSHA == the frozen snapshot (`git write-tree` taken before the run — same index ⇒ same tree);
//   - ParentSHA == pre-run HEAD (the rescue's manual `git commit-tree -p` parent);
//   - HEAD unchanged (atomic-HEAD §18.1 — rescue must not move refs);
//   - staged index unchanged both name-set and full diff (idempotent-index §20.2); AND
//   - the stub counter == wantCalls (1 ⇒ one-shot only / gate skipped Run; 2 ⇒ one-shot + 1
//     multi-turn turn / gate fired, Run aborted at turn 1).
//
// Returns the *RescueError for scenario-specific extra assertions (e.g. scenario (a)'s Cause != nil).
//
// Template: research-tests-ui.md §2 (TestCommitStaged_ParseFailRescue +
// TestCommitStaged_IdempotentIndexOnFailure).
func assertMultiTurnRescue(t *testing.T, repo string, m provider.Manifest, cfg config.Config, wantCalls int) *RescueError {
	t.Helper()
	beforeHEAD := headSHA(t, repo)
	beforeIndex := gitOut(t, repo, "diff", "--cached", "--name-only")
	beforeIndexFull := gitOut(t, repo, "diff", "--cached")
	// `git write-tree` is safe pre-run: it writes the index as an immutable tree object, does NOT
	// mutate index/refs. Its output IS the frozen TreeSHA CommitStaged will compute (same index ⇒
	// same tree) — so we assert re.TreeSHA == frozen without inspecting CommitStaged internals.
	frozen := strings.TrimSpace(gitOut(t, repo, "write-tree"))

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	var re *RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want *RescueError (FR-T7 rescue)", err)
	}
	if re.Kind != ErrRescue {
		t.Fatalf("Kind = %v, want ErrRescue (FR-T7: byte-identical to one-shot-exhaustion ⇒ exit 3; NOT ErrTimeout/124)", re.Kind)
	}
	if re.TreeSHA != frozen {
		t.Errorf("TreeSHA = %q, want frozen snapshot %q (WriteTree taken before generation)", re.TreeSHA, frozen)
	}
	if re.ParentSHA != beforeHEAD {
		t.Errorf("ParentSHA = %q, want %q (pre-run HEAD = the rescue parent)", re.ParentSHA, beforeHEAD)
	}
	if got := headSHA(t, repo); got != beforeHEAD {
		t.Errorf("HEAD moved %q → %q (atomic-HEAD §18.1 — rescue must not move refs)", beforeHEAD, got)
	}
	if got := gitOut(t, repo, "diff", "--cached", "--name-only"); got != beforeIndex {
		t.Errorf("staged file set changed (idempotent-index §20.2):\nbefore: %q\nafter:  %q", beforeIndex, got)
	}
	if got := gitOut(t, repo, "diff", "--cached"); got != beforeIndexFull {
		t.Errorf("staged index content changed (idempotent-index §20.2 full diff)")
	}
	cf := m.Env["STAGEHAND_STUB_COUNTER"]
	if cf == "" {
		t.Fatalf("manifest Env lacks STAGEHAND_STUB_COUNTER (use stubtest.NewScript/appendScriptManifest)")
	}
	raw, rerr := os.ReadFile(cf)
	if rerr != nil {
		t.Fatalf("read stub counter: %v", rerr)
	}
	if got := strings.TrimSpace(string(raw)); got != strconv.Itoa(wantCalls) {
		t.Errorf("stub invocations = %s, want %d (1 = one-shot only / multi-turn skipped; 2 = one-shot + 1 turn / Run aborted)", got, wantCalls)
	}
	return re
}

// (a) MID-TURN FAILURE → RESCUE (FR-T7). Global STAGEHAND_STUB_EXIT=1 ⇒ the one-shot exits 1 but its
// stdout is "" (script[0]) ⇒ parse-fail ⇒ exhaust ⇒ gate fires (conds a/b/c/d all hold) ⇒ Run's turn 1
// exits 1 ⇒ Execute returns a wrapped *exec.ExitError ⇒ Run aborts ⇒ rescue. Counter == 2.
func TestCommitStaged_MultiTurnMidTurnFailureRescue(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "hello world\n") // payload ≫ chunkTokens(4) ⇒ cond (b) true; N≥2
	stageFile(t, repo, "new.txt")

	// Script[0]="" ⇒ one-shot parse-fail ⇒ exhaust ⇒ gate. Global EXIT=1 ⇒ EVERY call exits non-zero.
	// The one-shot's exit-1 is swallowed (non-zero-exit branch falls through to ParseOutput("") ⇒
	// ok=false); Run's turn-1 exit-1 ⇒ Run aborts (FR-T7) ⇒ rescue. (No per-call exit knob without a
	// stub change; turn-1 failure == later-turn failure coverage — Run aborts at the first error.)
	m := appendScriptManifest(t, bin, []string{"", "ok", "ok", "feat: unreachable"})
	m.Env["STAGEHAND_STUB_EXIT"] = "1" // mutable Env map (optsEnvMap); applies to every stub call

	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0  // exactly 1 one-shot call ⇒ counter math clean
	cfg.MultiTurnChunkTokens = 4 // tiny ⇒ cond (b) true; N≥2
	cfg.MultiTurnFallback = true // cond (c) (default true; explicit)

	re := assertMultiTurnRescue(t, repo, m, cfg, 2) // 1 one-shot + 1 multi-turn turn (aborted at turn 1)
	// FR-T7: the multi-turn turn error supersedes one-shot's lastCause and propagates as RescueError.Cause.
	if re.Cause == nil {
		t.Errorf("Cause = nil, want the wrapped *exec.ExitError from the failed multi-turn turn (executor.go:77)")
	}
}

// (b) SMALL-PAYLOAD SKIP (FR-T1b negative). SessionMode="append" but a TINY diff + default chunkTokens
// ⇒ EstimateTokens(payload) ≤ 32000 ⇒ cond (b) FALSE ⇒ gate skips Run ⇒ existing rescue. Counter == 1.
// DISTINCT from T3.S3's TestCommitStaged_MultiTurnSkipped_SmallPayload (Kind-only): adds idempotent-index
// + the counter proof that multi-turn was NEVER entered.
func TestCommitStaged_MultiTurnSmallPayloadSkip_RescueInvariant(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "tiny.txt", "hi\n") // tiny diff ⇒ EstimateTokens(payload) ≤ 32000 ⇒ cond (b) false
	stageFile(t, repo, "tiny.txt")

	m := appendScriptManifest(t, bin, []string{""}) // SessionMode="append" (cond d true); script[0]="" ⇒ exhaust
	cfg := config.Defaults()                        // MultiTurnChunkTokens=32000 (default) ⇒ cond (b) false
	cfg.MaxDuplicateRetries = 0

	assertMultiTurnRescue(t, repo, m, cfg, 1) // one-shot ONLY; Run never entered
}

// (c) NON-APPEND PROVIDER SKIP (FR-T1d negative). SessionMode UNSET (claude-shaped) + large payload ⇒
// cond (d) FALSE ⇒ gate skips Run silently ⇒ existing rescue. Counter == 1. The large file +
// chunkTokens=4 keep cond (b) TRUE so cond (d) is the ONLY failing condition (isolates the non-append skip).
func TestCommitStaged_MultiTurnNonAppendSkip_RescueInvariant(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "new.txt", "hello world\n") // payload ≫ chunkTokens(4) ⇒ cond (b) true
	stageFile(t, repo, "new.txt")

	m := stubtest.NewScript(t, bin, []string{""}) // RAW NewScript ⇒ SessionMode nil ⇒ cond (d) false
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0
	cfg.MultiTurnChunkTokens = 4 // cond (b) true ⇒ ONLY cond (d) fails

	assertMultiTurnRescue(t, repo, m, cfg, 1) // one-shot ONLY; Run never entered
}
