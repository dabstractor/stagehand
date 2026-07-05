package provider

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dustin/stagehand/internal/signal"
	"github.com/dustin/stagehand/internal/ui"
)

// Execute runs a provider CmdSpec as a subprocess and returns its captured stdout, captured stderr,
// and a result error. It is the third stage of the provider pipeline (PRD §9.5/FR24–FR25): manifests
// (T1–T3) describe the agent; Render (T4) composes the CmdSpec; Execute (T5.S1) runs it; the parser
// (T6) reads stdout. The returned stdout is the agent's raw output for the parser; err signals
// timeout/failure for the orchestrator's retry/rescue (§18.2).
//
// I/O: spec.Stdin (non-empty) is piped to the child's stdin; "" leaves stdin nil so os/exec gives the
// child /dev/null (the CmdSpec contract from render.go). stdout and stderr are captured to SEPARATE
// buffers and returned even on error (partial output is useful for the rescue path §18.3 + verbose
// mode). cmd.Env = spec.Env when non-empty (Render builds os.Environ()+manifest env); nil Env ⇒ the
// child inherits the parent env. cmd.Dir is NOT set — the agent runs in the user's CWD.
//
// Timeout: when timeout > 0 a context.WithTimeout is derived (shadowing ctx — load-bearing: the later
// ctx.Err() reads the timeout ctx). The Config.Timeout default is 120s (PRD FR25). timeout ≤ 0 ⇒ no
// timeout (the parent ctx still applies).
//
// Kill semantics (PRD §18.2/§18.4, FINDING 8): the child runs as its own process-group leader
// (Setpgid ⇒ PGID==PID). On ctx cancellation (timeout OR parent/signal cancel) cmd.Cancel sends
// SIGTERM to the WHOLE group (-pid), killing grandchildren too; cmd.WaitDelay (3s) is the grace
// before Go escalates to SIGKILL. The platform specifics live in procgroup_unix.go (this file is
// platform-agnostic — setupProcessGroup is the single seam).
//
// Error contract (check ctx.Err() FIRST):
//
//   - timeout        ⇒ err IS context.DeadlineExceeded  (orchestrator: exit 124 + rescue)
//   - signal/parent  ⇒ err IS context.Canceled          (orchestrator: exit 3 + rescue)
//   - non-zero exit  ⇒ wrapped *exec.ExitError           (orchestrator: retry, then rescue)
//   - start failure  ⇒ wrapped LookPath/start error      (orchestrator: "command not found", exit 1)
//   - success        ⇒ err == nil
func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout string, stderr string, err error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout) // SHADOW — see doc; do not rename
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	if spec.Stdin != "" {
		cmd.Stdin = strings.NewReader(spec.Stdin) // "" ⇒ nil ⇒ /dev/null (CmdSpec contract)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out // separate capture
	cmd.Stderr = &errb
	if len(spec.Env) > 0 {
		cmd.Env = spec.Env // Render populates; nil ⇒ inherit parent env
	}
	setupProcessGroup(cmd) // platform seam (procgroup_*.go): Setpgid + Cancel + WaitDelay

	vb.VerboseCommand(strings.Join(append([]string{spec.Command}, spec.Args...), " "))
	vb.VerbosePayload(len(spec.Stdin)) // size only (never contents) — exposes whether the token-limit gate ran
	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("provider %q: start: %w", spec.Command, err)
	}
	signal.RegisterChild(cmd.Process.Pid) // arm signal forwarding (Setpgid ⇒ PGID==PID)
	defer signal.ClearChild()             // clear before return so a later signal can't kill a recycled PID

	if werr := cmd.Wait(); werr != nil {
		vb.VerboseRawOutput(out.String())
		vb.VerboseStderr(errb.String()) // surface the provider's real failure reason (upstream errors live on stderr)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return out.String(), errb.String(), ctxErr // timeout → DeadlineExceeded; cancel → Canceled
		}
		return out.String(), errb.String(), fmt.Errorf("provider %q: %w", spec.Command, werr) // exit failure
	}
	vb.VerboseRawOutput(out.String())
	vb.VerboseStderr(errb.String()) // surface provider warnings/diagnostics even on success
	return out.String(), errb.String(), nil
}
