//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestE2EScenarios exercises the PRD §20.5 must-cover regression set via t.Run subtests.
// Stub-reachable scenarios (S2/S3/S4/S6-single/S7) run by default.
// Stager-dependent scenarios (S1/S5/loop-S6) skip unless STAGEHAND_RUN_REAL=1.
func TestE2EScenarios(t *testing.T) {
	bin := buildStagehand(t)
	stub := buildStub(t)

	t.Run("S1_NothingStagedNFiles_NCommits", func(t *testing.T) {
		// REAL-only: needs a tooled stager for multi-concept decompose.
		skipIfNotReal(t, "S1 needs a tooled stager for multi-concept decompose")
		prov, model := realAgent(t)
		cfg := writeStubConfig(t, stub, "")
		baseEnv := stubEnv(nil)

		t.Run("auto", func(t *testing.T) {
			repo := newRepo(t)
			seedCommit(t, repo, "readme.md", "init")
			for _, f := range []string{"a.txt", "b.txt", "c.txt"} {
				writeFile(t, repo, f, f+"\n")
			}
			args := []string{"--provider", prov}
			if model != "" {
				args = append(args, "--model", model)
			}
			res := runStagehand(t, bin, repo, cfg, baseEnv, args...)
			if res.ExitCode != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
			}
			n := commitCount(t, repo)
			if n < 1 {
				t.Fatalf("commit count = %d, want ≥1", n)
			}
		})

		t.Run("forced", func(t *testing.T) {
			repo := newRepo(t)
			seedCommit(t, repo, "readme.md", "init")
			for _, f := range []string{"a.txt", "b.txt", "c.txt"} {
				writeFile(t, repo, f, f+"\n")
			}
			args := []string{"--provider", prov, "--commits", "3"}
			if model != "" {
				args = append(args, "--model", model)
			}
			res := runStagehand(t, bin, repo, cfg, baseEnv, args...)
			if res.ExitCode != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
			}
			n := commitCount(t, repo)
			if n < 1 {
				t.Fatalf("commit count = %d, want ≥1", n)
			}
		})
	})

	t.Run("S2_OneFile_NoPlannerCall", func(t *testing.T) {
		// One un-staged file → runOneFileShortcut → planner BYPASSED (FR-M2b).
		canaryScript := filepath.Join(t.TempDir(), "canary.sh")
		canaryMarker := t.TempDir() + "/planner_called"
		if err := os.WriteFile(canaryScript,
			[]byte("#!/bin/sh\ntouch \""+canaryMarker+"\"\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("write canary script: %v", err)
		}
		extras := `
[provider.canary]
command = ` + fmt.Sprintf("%q", canaryScript) + `
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "canary"

[role.planner]
provider = "canary"
`
		cfg := writeStubConfig(t, stub, extras)
		msgMarker := t.TempDir() + "/msg.marker"

		t.Run("stub", func(t *testing.T) {
			repo := newRepo(t)
			seedCommit(t, repo, "readme.md", "init")
			writeFile(t, repo, "solo.txt", "solo\n")

			env := stubEnv(map[string]string{
				"STAGEHAND_STUB_OUT":    "feat: solo file",
				"STAGEHAND_STUB_MARKER": msgMarker,
			})
			res := runStagehand(t, bin, repo, cfg, env, "--provider", "stub")
			if res.ExitCode != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
			}
			if n := commitCount(t, repo); n != 2 {
				t.Fatalf("commit count = %d, want 2", n)
			}
			names := diffTreeNames(t, repo, headSHA(t, repo))
			if !contains(names, "solo.txt") {
				t.Errorf("commit files = %v, want solo.txt", names)
			}
			if _, err := os.Stat(canaryMarker); err == nil {
				t.Error("planner canary marker exists — planner was called (FR-M2b violation)")
			}
		})

		t.Run("real", func(t *testing.T) {
			prov, model := realAgent(t)
			repo := newRepo(t)
			seedCommit(t, repo, "readme.md", "init")
			writeFile(t, repo, "solo.txt", "solo\n")
			args := []string{"--provider", prov}
			if model != "" {
				args = append(args, "--model", model)
			}
			res := runStagehand(t, bin, repo, cfg, stubEnv(nil), args...)
			if res.ExitCode != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
			}
			if n := commitCount(t, repo); n < 2 {
				t.Fatalf("commit count = %d, want ≥2", n)
			}
		})
	})

	t.Run("S3_ConcurrentFile_Excluded", func(t *testing.T) {
		// One-file change; after stub MARKER (post-freeze), write sentinel UN-STAGED.
		// Assert: sentinel NOT in commit, remains in working tree (FR-M1b/M1c).
		msgMarker := t.TempDir() + "/msg.marker"

		t.Run("stub", func(t *testing.T) {
			cfg := writeStubConfig(t, stub, "")
			repo := newRepo(t)
			seedCommit(t, repo, "readme.md", "init")
			writeFile(t, repo, "kept.txt", "kept\n")

			env := stubEnv(map[string]string{
				"STAGEHAND_STUB_OUT":      "feat: keep",
				"STAGEHAND_STUB_MARKER":   msgMarker,
				"STAGEHAND_STUB_SLEEP_MS": "800",
			})

			sentinel := filepath.Join(repo, "intruder.txt")
			resCh := make(chan e2eResult, 1)
			go func() {
				resCh <- runStagehand(t, bin, repo, cfg, env, "--provider", "stub")
			}()
			waitForMarker(t, msgMarker, 10*time.Second)
			if err := os.WriteFile(sentinel, []byte("concurrent\n"), 0o644); err != nil {
				t.Fatalf("write sentinel: %v", err)
			}
			res := <-resCh

			if res.ExitCode != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
			}
			if n := commitCount(t, repo); n != 2 {
				t.Fatalf("commit count = %d, want 2", n)
			}
			names := diffTreeNames(t, repo, headSHA(t, repo))
			if contains(names, "intruder.txt") {
				t.Errorf("commit files = %v; intruder.txt must NOT be present (FR-M1b/M1c)", names)
			}
			status := statusPorcelain(t, repo)
			if !strings.Contains(status, "?? intruder.txt") {
				t.Errorf("status = %q; want \"?? intruder.txt\"", status)
			}
		})

		t.Run("real", func(t *testing.T) {
			prov, model := realAgent(t)
			cfg := writeStubConfig(t, stub, "")
			repo := newRepo(t)
			seedCommit(t, repo, "readme.md", "init")
			writeFile(t, repo, "kept.txt", "kept\n")

			args := []string{"--provider", prov}
			if model != "" {
				args = append(args, "--model", model)
			}

			sentinel := filepath.Join(repo, "intruder.txt")
			resCh := make(chan e2eResult, 1)
			go func() {
				resCh <- runStagehand(t, bin, repo, cfg, stubEnv(nil), args...)
			}()
			time.Sleep(2 * time.Second)
			os.WriteFile(sentinel, []byte("concurrent\n"), 0o644)
			res := <-resCh

			if res.ExitCode != 0 {
				t.Logf("real agent exit code = %d (non-fatal); stderr:\n%s", res.ExitCode, res.Stderr)
			}
			names := diffTreeNames(t, repo, headSHA(t, repo))
			if contains(names, "intruder.txt") {
				t.Errorf("commit files = %v; intruder.txt must NOT be present", names)
			}
		})
	})

	t.Run("S4_MultiBackendBareModel_HardError", func(t *testing.T) {
		// FR-R5b: multi-backend provider with bare model → HARD ERROR (exit 1).
		extras := `
[provider.testmulti]
command = ` + fmt.Sprintf("%q", stub) + `
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
model_flag = "--model"
provider_flag = "--provider"
default_model = "x"
`
		cfg := writeStubConfig(t, stub, extras)

		t.Run("stub", func(t *testing.T) {
			repo := newRepo(t)
			seedCommit(t, repo, "readme.md", "init")
			writeFile(t, repo, "change.txt", "change\n")

			env := stubEnv(map[string]string{
				"STAGEHAND_STUB_OUT": "feat: x",
			})
			res := runStagehand(t, bin, repo, cfg, env,
				"--provider", "testmulti", "--model", "bare")
			if res.ExitCode != 1 {
				t.Fatalf("exit code = %d, want 1 (FR-R5b); stderr:\n%s", res.ExitCode, res.Stderr)
			}
			if !strings.Contains(res.Stderr, "must be inference/model") {
				t.Errorf("stderr missing 'must be inference/model'; got:\n%s", res.Stderr)
			}
			if n := commitCount(t, repo); n != 1 {
				t.Errorf("commit count = %d, want 1 (unchanged)", n)
			}
		})

		t.Run("real", func(t *testing.T) {
			repo := newRepo(t)
			seedCommit(t, repo, "readme.md", "init")
			writeFile(t, repo, "change.txt", "change\n")

			res := runStagehand(t, bin, repo, cfg, stubEnv(nil),
				"--provider", "testmulti", "--model", "bare")
			if res.ExitCode != 1 {
				t.Fatalf("exit code = %d, want 1; stderr:\n%s", res.ExitCode, res.Stderr)
			}
			if !strings.Contains(res.Stderr, "must be inference/model") {
				t.Errorf("stderr missing 'must be inference/model'; got:\n%s", res.Stderr)
			}
		})
	})

	t.Run("S5_ArbiterReconciliation", func(t *testing.T) {
		skipIfNotReal(t, "S5 needs a tooled stager for multi-concept decompose + arbiter")
		prov, model := realAgent(t)
		cfg := writeStubConfig(t, stub, "")
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		for _, f := range []string{"a.txt", "b.txt", "c.txt", "overlap.txt"} {
			writeFile(t, repo, f, f+"\n")
		}
		args := []string{"--provider", prov, "--commits", "3"}
		if model != "" {
			args = append(args, "--model", model)
		}
		res := runStagehand(t, bin, repo, cfg, stubEnv(nil), args...)
		if res.ExitCode != 0 {
			t.Fatalf("exit code = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
		n := commitCount(t, repo)
		if n < 2 {
			t.Fatalf("commit count = %d, want ≥2", n)
		}
	})

	t.Run("S6_Rescue", func(t *testing.T) {
		// Empty STAGEHAND_STUB_OUT → unparseable → rescue (exit 3).
		msgMarker := t.TempDir() + "/msg.marker"

		t.Run("single_rescue", func(t *testing.T) {
			cfg := writeStubConfig(t, stub, "")
			repo := newRepo(t)
			seedCommit(t, repo, "readme.md", "init")
			writeFile(t, repo, "solo.txt", "solo\n")

			env := stubEnv(map[string]string{
				"STAGEHAND_STUB_OUT":    "",
				"STAGEHAND_STUB_MARKER": msgMarker,
			})
			res := runStagehand(t, bin, repo, cfg, env, "--provider", "stub")
			if res.ExitCode != 3 {
				t.Fatalf("exit code = %d, want 3 (Rescue); stderr:\n%s", res.ExitCode, res.Stderr)
			}
			if n := commitCount(t, repo); n != 1 {
				t.Errorf("commit count = %d, want 1 (unchanged)", n)
			}
		})

		t.Run("loop_rescue", func(t *testing.T) {
			skipIfNotReal(t, "S6 loop-rescue needs a tooled stager")
			t.Skip("loop-rescue is covered in-process by TestDecompose_MessageRescuePartial")
		})
	})

	t.Run("S7_CASAbort_HeadMoved", func(t *testing.T) {
		// Stub writes MARKER then SLEEPs; test moves HEAD during the sleep → CAS abort.
		msgMarker := t.TempDir() + "/msg.marker"

		t.Run("stub", func(t *testing.T) {
			cfg := writeStubConfig(t, stub, "")
			repo := newRepo(t)
			seedCommit(t, repo, "readme.md", "init")
			writeFile(t, repo, "solo.txt", "solo\n") // write before staging
			stageFile(t, repo, "solo.txt")

			env := stubEnv(map[string]string{
				"STAGEHAND_STUB_OUT":      "feat: x",
				"STAGEHAND_STUB_MARKER":   msgMarker,
				"STAGEHAND_STUB_SLEEP_MS": "1500",
			})

			resCh := make(chan e2eResult, 1)
			go func() {
				resCh <- runStagehand(t, bin, repo, cfg, env, "--provider", "stub")
			}()
			waitForMarker(t, msgMarker, 10*time.Second)
			runGit(t, repo, "commit", "--allow-empty", "-m", "concurrent")
			concurrentSHA := headSHA(t, repo)
			res := <-resCh

			if res.ExitCode != 1 {
				t.Fatalf("exit code = %d, want 1 (CAS abort); stderr:\n%s", res.ExitCode, res.Stderr)
			}
			currentSHA := headSHA(t, repo)
			if currentSHA != concurrentSHA {
				t.Errorf("HEAD moved: was %s, now %s", concurrentSHA, currentSHA)
			}
		})

		t.Run("real", func(t *testing.T) {
			prov, model := realAgent(t)
			cfg := writeStubConfig(t, stub, "")
			repo := newRepo(t)
			seedCommit(t, repo, "readme.md", "init")
			writeFile(t, repo, "solo.txt", "solo\n")
			stageFile(t, repo, "solo.txt")

			args := []string{"--provider", prov}
			if model != "" {
				args = append(args, "--model", model)
			}

			resCh := make(chan e2eResult, 1)
			go func() {
				resCh <- runStagehand(t, bin, repo, cfg, stubEnv(nil), args...)
			}()
			time.Sleep(3 * time.Second)
			runGit(t, repo, "commit", "--allow-empty", "-m", "concurrent")
			concurrentSHA := headSHA(t, repo)
			res := <-resCh

			if res.ExitCode == 1 {
				currentSHA := headSHA(t, repo)
				if currentSHA != concurrentSHA {
					t.Errorf("HEAD moved: was %s, now %s", concurrentSHA, currentSHA)
				}
			}
		})
	})
}

// contains reports whether ss contains s.
func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
