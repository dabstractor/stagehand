// Command stubagent is a tiny fake-agent binary for Stagehand's integration/property tests
// (PRD §20.1 layer 3). It reads the prompt from stdin and writes a canned commit message to
// stdout, with behavior (output, exit code, simulated timeout, stderr, and per-call output
// variation for the dedupe loop) controlled entirely by STAGEHAND_STUB_* environment variables —
// set via a test-only provider.Manifest's Env map (the existing Manifest.Env→CmdSpec.Env→cmd.Env
// seam). It is invoked through provider.Execute exactly like a real agent. STDLIB ONLY; no
// internal/*, no third-party.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// envInt reads key as a non-negative int; any parse error / negative → def. Never panics.
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}

func main() {
	// 1. Drain stdin FIRST (deadlock guard, §4): the executor pipes the payload via a bounded OS
	//    pipe (~64 KiB). If we slept before draining and the payload exceeded the buffer,
	//    parent+child would deadlock. Tee to file if STAGEHAND_STUB_STDINFILE is set (payload-capture
	//    for tests); else drain to Discard. Both branches drain FULLY (the deadlock guard must hold).
	//    /dev/null (Stdin=="") → io.Copy returns immediately.
	if sf := os.Getenv("STAGEHAND_STUB_STDINFILE"); sf != "" {
		var buf bytes.Buffer
		io.Copy(&buf, os.Stdin)
		os.WriteFile(sf, buf.Bytes(), 0o644)
	} else {
		io.Copy(io.Discard, os.Stdin)
	}

	// 1b. Write the readiness marker (if STAGEHAND_STUB_MARKER is set). This tells the test
	//     harness that stdin has been drained and generation is in-flight. Must happen BEFORE
	//     the sleep so the test can race HEAD movement deterministically.
	if marker := os.Getenv("STAGEHAND_STUB_MARKER"); marker != "" {
		_ = os.WriteFile(marker, []byte("1"), 0o644)
	}

	// 1c. Write the received argv (if STAGEHAND_STUB_ARGSFILE is set). This lets tests observe
	//     the exact rendered command-line end-to-end (model, reasoning tokens, etc.). Must happen
	//     AFTER stdin drain (deadlock guard) and AFTER the marker write (test synchronization).
	//     Join with NUL so flag values containing spaces survive; tests split on "\x00".
	if argsFile := os.Getenv("STAGEHAND_STUB_ARGSFILE"); argsFile != "" {
		_ = os.WriteFile(argsFile, []byte(strings.Join(os.Args, "\x00")), 0o644)
	}

	// 2. Sleep AFTER draining (timeout simulation). The parent isn't blocked on stdin anymore.
	if ms := envInt("STAGEHAND_STUB_SLEEP_MS", 0); ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}

	// 3. Stderr (captured separately by Execute; useful for verbose-mode / stderr tests).
	if s := os.Getenv("STAGEHAND_STUB_STDERR"); s != "" {
		fmt.Fprint(os.Stderr, s)
	}

	// 4. Select + write stdout. Script mode ⇒ call-varying (dedupe loop); else single-response OUT.
	out := os.Getenv("STAGEHAND_STUB_OUT")
	if scriptFile := os.Getenv("STAGEHAND_STUB_SCRIPT"); scriptFile != "" {
		out = selectScripted(scriptFile)
	}
	fmt.Fprint(os.Stdout, out) // EXACTLY `out` — no extra newline (ParseOutput trims; assertions stay byte-exact)

	// 5. Exit with the configured code (non-zero simulates a failed agent → orchestrator retry/rescue).
	os.Exit(envInt("STAGEHAND_STUB_EXIT", 0))
}

// selectScripted returns the call-indexed line of the script file, advancing a file-backed counter
// so successive invocations of the stub (each a fresh process) get successive responses. Blank lines
// are significant (empty output ⇒ ParseOutput ok=false ⇒ orchestrator retries = parse-failure-then-rescue).
func selectScripted(scriptFile string) string {
	data, err := os.ReadFile(scriptFile)
	if err != nil {
		return "" // missing/unreadable script ⇒ empty output
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return ""
	}
	index := 0
	if counterFile := os.Getenv("STAGEHAND_STUB_COUNTER"); counterFile != "" {
		index = readCounter(counterFile)
		writeCounter(counterFile, index+1) // best-effort; serial callers make races impossible (§3)
	}
	if index < 0 || index >= len(lines) {
		index = len(lines) - 1 // clamp to last → stable tail after the script is exhausted
	}
	return lines[index]
}

func readCounter(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func writeCounter(path string, n int) {
	_ = os.WriteFile(path, []byte(strconv.Itoa(n)), 0o644)
}
