package generate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/stubtest"
	"github.com/dustin/stagecoach/internal/ui"
)

// verboseCommandPrefix is the marker ui.Verbose emits before each joined argv. Each provider.Execute
// call appends exactly one "DEBUG: command: <argv>\n" block, so splitting the verbose buffer on this
// marker yields one element per Execute (the first element is pre-first-command preamble). Because a
// system-prompt VALUE may contain newlines, a single command block can span multiple physical lines —
// which is exactly why we must NOT split the buffer on "\n" to enumerate turns.
const verboseCommandPrefix = "DEBUG: command: "

// TestCommitStaged_MultiTurnRenderContract verifies the per-turn RENDER CONTRACT of the multi-turn
// generation fallback's happy path (PRD §9.24 FR-T4/T6) — the part NOT covered by P1.M1.T3.S3's
// TestCommitStaged_MultiTurnFallbackSuccess (which asserts the commit lands and the FR-T1 gate fires
// but never inspects the rendered argv).
//
// UNIQUE value of this test: across ALL N+1 multi-turn turns it asserts that
//
//	(c) every turn's argv carries --session-id <stable-id> and does NOT carry --no-session; and
//	(d) the system-prompt flag is emitted on turn 1 ONLY.
//
// MECHANISM (no stub change): provider.Execute calls vb.VerboseCommand(<joined argv>) on EVERY
// call (executor.go), and multiturn.Run calls Execute once per turn passing deps.Verbose. So wiring
// Deps{Verbose: ui.NewVerbose(&buf, true)} captures the one-shot command PLUS all N+1 multi-turn
// commands. The manifest sets BareFlags=["--no-session"] + SystemPromptFlag="--system" to simulate
// pi's isolation flags so the render deltas are OBSERVABLE: one-shot Render includes --no-session
// (and --system, since Render has no turn gate); multi-turn RenderMultiTurn drops --no-session, adds
// --session-id, and emits --system on turn 1 only. The final turn's byte-exact argv is cross-checked
// via the stub's STAGECOACH_STUB_ARGSFILE (which overwrites each call ⇒ holds only turn N+1).
//
// COUNTING NOTE: a system-prompt VALUE with internal newlines makes a single command span multiple
// physical lines, so the buffer is split into per-command BLOCKS on verboseCommandPrefix (NOT on
// "\n"). The one-shot Render call ALSO emits --system (Render has no turn-1-only gate), so the
// render contract is asserted on the MULTI-TURN subset only (blocks containing --session-id).
func TestCommitStaged_MultiTurnRenderContract(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")

	// LARGE staged file ⇒ EstimateTokens(payload) ≫ 50 (cond b) AND spans ≥2 chunks (N≥2).
	// ~60 lines × ~40 chars ≈ 2.4 KB ⇒ ~600 tokens ⇒ N ≈ several at chunkTokens=50.
	var b strings.Builder
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&b, "line %02d: padding content for a large diff\n", i)
	}
	writeFile(t, repo, "big.txt", b.String())
	stageFile(t, repo, "big.txt")

	// Script: line 0 "" (one-shot parse-fail ⇒ exhaust ⇒ gate fires); "ok","ok" priming; message LAST
	// (clamp-to-last ⇒ turn N+1 emits the message for any N≥2; intermediate turns' stdout discarded).
	script := []string{"", "ok", "ok", "feat: add big thing"}
	m := appendScriptManifest(t, bin, script) // SessionMode="append" (conds d + RenderMultiTurn gate)

	// SIMULATE pi's isolation flag + system-prompt flag so the render contract is OBSERVABLE:
	//   one-shot Render     ⇒ argv has --no-session (BareFlags) + --system, no --session-id.
	//   multi-turn Render   ⇒ argv has --session-id, NO --no-session (dropped), --system on turn 1 only.
	m.BareFlags = []string{"--no-session"}
	spf := "--system"
	m.SystemPromptFlag = &spf

	// Byte-exact argv for the FINAL turn (turn N+1) — the stub's ArgsFile overwrites each call.
	argsFile := filepath.Join(t.TempDir(), "args")
	m.Env["STAGECOACH_STUB_ARGSFILE"] = argsFile // m.Env is a mutable map (optsEnvMap)

	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0   // ⇒ exactly 1 one-shot call ⇒ counter math = N+2 total
	cfg.MultiTurnChunkTokens = 50 // tiny ⇒ N≥2 (payload ≫ 50 tokens)
	cfg.MultiTurnFallback = true  // cond c (default true; explicit for clarity)

	// Verbose sink captures EVERY Execute command (one-shot + N+1 multi-turn).
	var vbuf bytes.Buffer
	vb := ui.NewVerbose(&vbuf, true)

	res, err := CommitStaged(context.Background(),
		Deps{Git: git.New(repo), Manifest: m, Verbose: vb}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v (expected multi-turn success)", err)
	}

	log := vbuf.String()

	// --- (a) commit lands ---
	if !shaRe.MatchString(res.CommitSHA) {
		t.Errorf("CommitSHA = %q, want hex SHA", res.CommitSHA)
	}
	if res.Subject != "feat: add big thing" {
		t.Errorf("Subject = %q, want %q (final-turn parsed message)", res.Subject, "feat: add big thing")
	}
	if got := headSHA(t, repo); got != res.CommitSHA {
		t.Errorf("HEAD = %q, want %q (commit must land)", got, res.CommitSHA)
	}
	if msg := gitOut(t, repo, "log", "--format=%B", "-n1", res.CommitSHA); msg != "feat: add big thing" {
		t.Errorf("git log message = %q, want %q", msg, "feat: add big thing")
	}

	// --- split the verbose buffer into per-command BLOCKS (NOT lines: sys-prompt values embed "\n") ---
	commands := splitVerboseCommands(log)
	if len(commands) == 0 {
		t.Fatalf("verbose buffer yielded no commands: %q", log)
	}
	// Classify: a multi-turn command is one whose argv carries --session-id; the one-shot does not.
	var multiTurnCmds []string
	var oneShotCmds []string
	for _, c := range commands {
		if strings.Contains(c, "--session-id") {
			multiTurnCmds = append(multiTurnCmds, c)
		} else {
			oneShotCmds = append(oneShotCmds, c)
		}
	}

	// --- (b) N+1 multi-turn invocations + exactly 1 one-shot call ---
	if len(oneShotCmds) != 1 {
		t.Errorf("one-shot command count = %d, want exactly 1 (MaxDuplicateRetries=0)", len(oneShotCmds))
	}
	multiTurnCalls := len(multiTurnCmds)
	if multiTurnCalls < 3 {
		t.Fatalf("multi-turn turn count = %d, want ≥3 (N≥2); is the payload large enough / chunkTokens tiny?",
			multiTurnCalls)
	}
	N := multiTurnCalls - 1 // N chunks; multiTurnCalls == N+1
	// Counter cross-check: 1 one-shot + (N+1) multi-turn == N+2 total stub invocations.
	if cf := m.Env["STAGECOACH_STUB_COUNTER"]; cf != "" {
		if raw, rerr := os.ReadFile(cf); rerr == nil {
			got := strings.TrimSpace(string(raw))
			if got != fmt.Sprintf("%d", N+2) {
				t.Errorf("stub counter = %s, want %d (1 one-shot + %d multi-turn)", got, N+2, multiTurnCalls)
			}
		}
	}

	// --- (c) every multi-turn turn: --session-id present, stable id, --no-session ABSENT ---
	// session-id occurrences across the WHOLE buffer must equal the multi-turn turn count (one each);
	// the one-shot carries none.
	if sidTotal := strings.Count(log, "--session-id"); sidTotal != multiTurnCalls {
		t.Errorf("--session-id count = %d, want %d (one per multi-turn turn, none one-shot)",
			sidTotal, multiTurnCalls)
	}
	ids := sessionIDRe.FindAllString(log, -1)
	if len(ids) != multiTurnCalls {
		t.Errorf("session id occurrences = %d, want %d (one per multi-turn turn)", len(ids), multiTurnCalls)
	}
	for _, id := range ids {
		if id != ids[0] {
			t.Errorf("session id not stable: %q != %q (FR-T6 requires ONE id per run)", id, ids[0])
		}
	}
	// --no-session must appear ONLY on the one-shot turn (BareFlags) and on ZERO multi-turn turns.
	if noSesTotal := strings.Count(log, "--no-session"); noSesTotal != 1 {
		t.Errorf("--no-session total count = %d, want exactly 1 (one-shot only)", noSesTotal)
	}
	for i, c := range multiTurnCmds {
		if strings.Contains(c, "--no-session") {
			t.Errorf("multi-turn turn %d argv must NOT contain --no-session; block=%q", i+1, c)
		}
		if !strings.Contains(c, "--session-id") {
			t.Errorf("multi-turn turn %d argv lacks --session-id; block=%q", i+1, c)
		}
	}
	// Final-turn byte-exact cross-check (ArgsFile holds turn N+1):
	rawArgs, _ := os.ReadFile(argsFile)
	args := strings.Split(string(rawArgs), "\x00")
	if !sliceContains(args, "--session-id") {
		t.Errorf("final-turn argv lacks --session-id; args=%v", args)
	}
	if sliceContains(args, "--no-session") {
		t.Errorf("final-turn argv must NOT contain --no-session; args=%v", args)
	}

	// --- (d) turn-1-only system prompt flag (within the multi-turn subset) ---
	// NOTE: the one-shot Render ALSO emits --system (Render has no turn gate), so the contract is
	// asserted on multi-turn commands only: exactly ONE (turn 1) carries --system, turns 2..N+1 do not.
	multiTurnSys := 0
	for _, c := range multiTurnCmds {
		if strings.Contains(c, "--system") {
			multiTurnSys++
		}
	}
	if multiTurnSys != 1 {
		t.Errorf("multi-turn turns with --system = %d, want 1 (turn 1 only; turns 2..N+1 omit it)",
			multiTurnSys)
	}
	if sliceContains(args, "--system") {
		t.Errorf("final-turn argv (turn N+1>1) must NOT contain --system; args=%v", args)
	}
}

// splitVerboseCommands splits the verbose buffer into per-command blocks. Each provider.Execute call
// emits exactly one "DEBUG: command: <joined argv>" block; the marker is unique to command lines
// (VerboseRawOutput/VerbosePayload/VerboseStderr use different prefixes). A single command block may
// span multiple physical lines because the system-prompt VALUE can embed newlines — hence splitting
// on the command marker (not "\n"). The first element (preamble before any command) is discarded.
func splitVerboseCommands(buf string) []string {
	parts := strings.Split(buf, verboseCommandPrefix)
	if len(parts) <= 1 {
		return nil
	}
	return parts[1:]
}

// sessionIDRe matches the multi-turn session id token logged in every turn's rendered argv
// (FR-T6: "stagecoach-<32 hex>"). Used to assert the id is STABLE across all N+1 turns.
var sessionIDRe = regexp.MustCompile(`stagecoach-[0-9a-f]{32}`)
