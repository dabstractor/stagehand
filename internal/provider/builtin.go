package provider

// BuiltinManifests returns the compiled-in default provider manifests (PRD §12.3 pi, §12.4 claude,
// §12.6 opencode, §12.7 codex + cursor, §12.5.1 agy, §12.5.2 qwen-code), keyed by manifest name. These are
// the zero-config defaults a user override (config [provider.<name>]) merges onto via MergeManifest (S2) in
// the registry (P1.M2.T3).
//
// The manifests are constructed FRESH on every call (no package-level var): strPtr/boolPtr allocate new
// pointers and slice literals allocate new backing arrays each call, so no caller can corrupt a built-in
// by mutating a returned BareFlags/Env. MergeManifest (S2) already never mutates base, so the normal
// registry path is safe either way — fresh-per-call additionally guards against any direct mutation.
//
// The full set: pi + claude (the "explicit tool-disable switch" pair, S1), opencode (read-only constraint,
// S2 — `run` is already a read-only one-shot; delivery revised to stdin), codex + cursor (read-only
// constraint, S3 — codex's two revisions resolve the external_deps.md §codex discrepancy), §12.5.1 agy
// (experimental — the Gemini-CLI successor; gemini-cli itself is EOL and no longer shipped), and §12.5.2
// qwen-code (experimental). Seven providers: pi, claude, opencode, codex, cursor, agy, qwen-code.
func BuiltinManifests() map[string]Manifest {
	return map[string]Manifest{
		"pi":        builtinPi(),
		"claude":    builtinClaude(),
		"opencode":  builtinOpenCode(),
		"codex":     builtinCodex(),
		"cursor":    builtinCursor(),
		"agy":       builtinAgy(),
		"qwen-code": builtinQwenCode(),
	}
}

// builtinPi returns the pi manifest per PRD §12.3 (FULLY VERIFIED vs `pi --help`, external_deps.md §pi).
// Per FR-D2 (PRD §9.16/§12.3), the shipped pi default is DECOUPLED from any one subscription:
// default_model AND default_provider are both "" (NON-NIL empty). config init fills per-role models
// from the FR-D4 table; the user/config picks the backend. The original commit-pi setup
// (provider=zai, model=glm-5-turbo) is a documented PERSONAL OVERRIDE, not the shipped default —
// see TestBuiltinManifests_RenderedCommand_Pi_PersonalOverride.
//
// NOTE: ReasoningLevels is populated — pi `--thinking` high/medium/low (verified `pi --help`,
// external_deps.md §pi); off ⇒ no-op (no entry). minimal/xhigh have no stagecoach level.
// Per FR-D2 (PRD §9.16/§12.3), the shipped pi default is DECOUPLED from any one subscription:
// default_model is "". config init fills per-role models from the FR-D4 table; the user/config
// picks the backend (inference provider) via the model slash-prefix (v3 FR-R5b).
//
// CHROME-DISABLE (FR-C5, §9.28): verified vs `pi --help` (external_deps.md §pi, 2026-06-29).
// Per chrome surface: extensions — disabled by --no-extensions (bare_flags); skills — disabled by
// --no-skills (bare_flags); prompt-templates — disabled by --no-prompt-templates (bare_flags);
// context files (AGENTS.md/CLAUDE.md) — disabled by --no-context-files (bare_flags). MCP servers
// are NOT disabled: pi has NO --no-mcp flag (only --mcp-config <path>). --no-tools suppresses MCP
// tool USE, but configured servers may still be discovered/connected at startup. This is a
// documented, tracked LIMITATION (FR-C3), never an assumption that MCP is off.
func builtinPi() Manifest {
	return Manifest{
		Name:              "pi",
		Detect:            strPtr("pi"),
		Command:           strPtr("pi"),
		ListModelsCommand: []string{"pi", "--list-models"}, // VERIFIED 2026-07-03 via `pi --list-models` (exit 0); FLAG form, not a subcommand. FR-L2/FR-D5.
		PromptDelivery:    strPtr("stdin"),
		PrintFlag:         strPtr("-p"),
		ModelFlag:         strPtr("--model"),
		DefaultModel:      strPtr(""), // FR-D2: was glm-5-turbo; decoupled from any one subscription
		SystemPromptFlag:  strPtr("--system-prompt"),
		ProviderFlag:      strPtr("--provider"),
		SessionMode:       strPtr("append"), // VERIFIED 2026-07-05 via `pi --session-id X <isolation-flags-minus-no-session> -p "remember BANANA"` then recall returns BANANA; FR-T9.
		BareFlags: []string{
			"--no-tools",
			"--no-extensions",
			"--no-skills",
			"--no-prompt-templates",
			"--no-context-files",
			"--no-session",
		},
		// REASONING LEVELS (v3; §12.1, FR-R6). pi exposes `--thinking off|minimal|low|medium|high|xhigh`
		// (verified `pi --help`, external_deps.md §pi). off ⇒ no entry (natural zero no-op); stagecoach's
		// level set is off|low|medium|high, so minimal/xhigh are not mapped. Tokens append after the model flag.
		ReasoningLevels: map[string][]string{
			"high":   {"--thinking", "high"},
			"medium": {"--thinking", "medium"},
			"low":    {"--thinking", "low"},
		},
		// TOOLED MODE (v2 §11.5 — the stager role). pi has NO git-scoped allowlist flag (--help shows only
		// the all-or-nothing --no-tools), so pi's tooled profile = bare MINUS --no-tools: pi's native tool
		// system ON, everything else still off (chrome-less + ephemeral). There is no way to scope pi's tools
		// to staging-only git subcommands without disabling tools entirely (--no-tools would bar the stager
		// from running git at all).
		//
		// SAFETY MODEL — HONEST: unlike claude (whose stager IS structurally constrained by a staging-only
		// git allowlist — Bash(git add:*,git apply:*,git status:*,git diff:*), see builtinClaude), pi's
		// stager is NOT structurally/flag-scoped. A misbehaving pi stager CAN run arbitrary Bash, including
		// `git commit`, `git push`, `git update-ref`, `git reset`, and `rm -rf`. PRD §19's "structurally
		// constrained … cannot commit/amend/push" claim therefore does NOT hold for pi. pi's stager is
		// instead:
		//   1. INSTRUCTIONALLY constrained — by the §17.6 stager task prompt (it is instructed to stage only).
		//   2. BEST-EFFORT guarded — by the HEAD-movement defense-in-depth check (P1.M2.T1.S3): HEAD is
		//      snapshotted before each stager call and the run aborts (treated as a safety violation) if HEAD
		//      has moved when the stager returns. THE SAFETY NET IS THIS GUARD, NOT FLAG-SCOPING.
		// (stagecoach's ref-mutation monopoly, §13.6.2/§19, holds only insofar as the stager cannot itself
		// move a ref — for pi that relies on the §17.6 prompt + the S3 guard, not on TooledFlags.)
		TooledFlags: []string{
			"--no-extensions",
			"--no-skills",
			"--no-prompt-templates",
			"--no-context-files",
			"--no-session",
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// Subcommand, PromptFlag, JsonField, RetryInstruction, Env: nil (absent in §12.3).
	}
}

// builtinClaude returns the claude manifest per PRD §12.4 (VERIFIED vs `claude --help`, external_deps.md
// §claude). claude disables tools via `--tools ""` (documented "Use \"\" to disable all tools") and
// settings via `--setting-sources ""`; `--no-session-persistence` makes it ephemeral (valid only with -p).
//
// NOTE: (1) ProviderFlag is strPtr("") — §12.4 WRITES `provider_flag = "" # n/a` (non-nil empty); the
// §12.2 renderer's model-prefix fold (FR-R5b) does NOT split for provider_flag="" (claude has
// no sub-provider concept). (2) ReasoningLevels is populated — claude `--effort` (verified,
// external_deps.md §claude); off ⇒ no-op.
// (3) BareFlags has TWO "" value tokens (the args to --tools / --setting-sources) — do NOT drop them.
//
// CHROME-DISABLE (FR-C5, §9.28): verified vs `claude --help` (external_deps.md §claude). Chrome is
// COVERED via two mechanisms: --tools "" disables ALL built-in tools (MCP surfaces as tools), and
// --setting-sources "" blocks the settings files where MCP servers, skills, and extensions are
// configured. Both are in bare_flags. No per-surface gap.
func builtinClaude() Manifest {
	return Manifest{
		Name:             "claude",
		Detect:           strPtr("claude"),
		Command:          strPtr("claude"),
		PromptDelivery:   strPtr("stdin"),
		PrintFlag:        strPtr("-p"),
		ModelFlag:        strPtr("--model"),
		DefaultModel:     strPtr("sonnet"),
		SystemPromptFlag: strPtr("--system-prompt"),
		ProviderFlag:     strPtr(""), // §12.4 explicit empty (NON-NIL) — n/a for claude
		BareFlags: []string{
			"--tools", "", // disable ALL built-in tools (value arg = "")
			"--setting-sources", "", // load no settings sources (value arg = "")
			"--no-session-persistence", // ephemeral (only valid with -p)
		},
		// TOOLED MODE (v2 §11.5 — the stager role). INVERTS claude's bare mode: instead of --tools "" (disable
		// ALL tools), ENABLE tools RESTRICTED via an allowlist to Bash(git add:*,git apply:*,git status:*,git diff:*)
		// + Read + Edit — the staging-relevant toolset ONLY. This makes ref-mutating git subcommands
		// (commit/push/update-ref/reset/rebase/amend) STRUCTURALLY UNREACHABLE for the stager, delivering the
		// §19 "cannot commit/amend/push" guarantee for claude. --setting-sources "" + --no-session-persistence
		// carry over from bare.
		// # TO CONFIRM (integration, P3.M2.T3): external_deps.md §claude records --tools;
		// the item contract + codebase use --allowed-tools (the explicit-enable allow-list flag). Verify against
		// a real claude --help at the first stager run; if wrong, swap the flag token (the value is the allowlist).
		TooledFlags: []string{
			"--allowed-tools", "Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit",
			"--setting-sources", "",
			"--no-session-persistence",
		},
		// REASONING LEVELS (v3; §12.1, FR-R6). claude exposes `--effort low|medium|high` (verified vs
		// `claude --help`, external_deps.md §claude — NOT --thinking-effort). off has no entry ⇒ no-op.
		ReasoningLevels: map[string][]string{
			"high":   {"--effort", "high"},
			"medium": {"--effort", "medium"},
			"low":    {"--effort", "low"},
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// Subcommand, PromptFlag, JsonField, RetryInstruction, Env: nil (absent in §12.4).
	}
}

// builtinAgy returns the agy (Google Antigravity CLI) manifest per PRD §12.5.1 (the Gemini-CLI successor,
// superseded gemini on 2026-06-18). Flag surface RE-VERIFIED vs `agy --help` + live stdin runs on
// 2026-07-08 against agy v1.1.0. The Antigravity CLI has DIVERGED from the gemini-cli lineage it forked
// from: the bare-roles invocation the 2026-07-03 manifest assumed (`--approval-mode default -p` + stdin)
// no longer works on v1.1.0. Two corrections:
//
//   - PROMPT DELIVERY: `-p`/`--print`/`--prompt` is VALUE-TAKING (the prompt is its argument), NOT a
//     boolean print-mode flag. A bare `-p` fails with "flag needs an argument: -p". Empirically, agy
//     reads the prompt from STDIN when `-p` is ABSENT (or empty) and stdin is a pipe (non-TTY): a no-`-p`
//     run with piped stdin prints the response and exits 0. So stagecoach uses prompt_delivery="stdin" with
//     PrintFlag="" (NO bare -p). This also routes ~300 KB diffs over stdin — a bare `-p` would demand a
//     value, and argv/positional delivery would hit Linux's 128 KB MAX_ARG_STRLEN ceiling.
//   - READ-ONLY CONSTRAINT: agy v1.1.0 has NO `--approval-mode` flag (the gemini-cli lineage's flag was
//     removed). The read-only, never-ask equivalent is `--mode plan` (choices: accept-edits | plan).
//     Verified: `--mode plan` + stdin yields CLEAN commit-message output — no plan-mode formatting noise —
//     so bare roles stay read-only without polluting the message.
//
// The model flag is `--model` ONLY (`-m` is rejected: "flags provided but not defined"; agy defines short
// aliases only for -c/-i/-p). agy has no first-class system-prompt flag → sys is PREPENDED to the payload
// (§12.2).
//
// MODEL NAMES (verified 2026-07-08): agy's --model takes the DISPLAY LABEL from `agy models` VERBATIM,
// reasoning level included — e.g. "Gemini 3.5 Flash (Low)" / "GPT-OSS 120B (Medium)". Reasoning is NOT a
// separate flag (ReasoningLevels stays nil); it is baked into the label's parenthesized suffix. API-style
// ids are NOT recognized and SILENTLY fall back to agy's own default — so these labels, spaces and all,
// are the only safe tokens. NOTE: GPT-OSS 120B is subject to transient backend 503 "No capacity available"
// errors; retries succeed — that is an external capacity issue, not a stagecoach bug.
//
// §12.5.1.1 status (2026-07-08, v1.1.0): item 1 (issue #76, non-TTY stdout drop) NO LONGER REPRODUCES —
// live stdin runs from a non-TTY return stdout correctly (PONG, full commit messages). Items 2 (`--model`)
// and 3 (no system-prompt flag) CLEARED. Item 4 (tooled/stager flags) remains OPEN, so agy still cannot
// serve as a stager and stays Experimental=true until it clears.
//
// STAGER: TooledFlags is intentionally nil — agy CANNOT serve as a stager until §12.5.1.1 item 4 (the
// scoped, non-interactive, git-scoped tool combo) is verified. RenderTooled errors on nil tooled_flags.
//
// NOTE: (1) PrintFlag="" (NON-NIL empty — agy reads stdin; a bare -p is value-taking and breaks delivery).
// (2) SystemPromptFlag/ProviderFlag are strPtr("") — §12.5.1 WRITES them "" (NON-NIL empty): no sys flag
// (sys prepended, §12.2), no sub-provider. (3) DefaultModel="Gemini 3.5 Flash (Low)" (label form; verified
// 2026-07-08). (4) Experimental=boolPtr(true) (item 4 still open). (5) BareFlags=["--mode","plan"] (the
// v1.1.0 read-only equivalent of the removed --approval-mode default). (6) Subcommand/PromptFlag/JsonField/
// RetryInstruction/Env/TooledFlags/ReasoningLevels are nil (absent). agy is a Gemini-CLI-lineage
// provider (it superseded the EOL'd gemini-cli on 2026-06-18); it differs from codex/cursor in its model
// flag (--model), delivery (stdin w/o -p), bare flag (--mode plan), default model + Experimental.
//
// CHROME-DISABLE (FR-C5, §9.28): verified vs `agy --help` (agy v1.1.0, 2026-07-08). agy exposes NO
// per-surface chrome-disable switch for skills/extensions/context-files/MCP. --mode plan
// (bare_flags) is the read-only, never-ask CONSTRAINT (mutation safety, §12.7.1) — it is NOT a chrome
// substitute. Chrome MAY load; the call stays read-only and never-mutate. Documented LIMITATION
// (FR-C4), not an assumption. Re-check at the next agy --help re-verification.
func builtinAgy() Manifest {
	return Manifest{
		Name:              "agy",
		Detect:            strPtr("agy"),
		Command:           strPtr("agy"),
		ListModelsCommand: []string{"agy", "models"}, // VERIFIED 2026-07-08 via `agy models` (exit 0); FR-L2/FR-D5.
		PromptDelivery:    strPtr("stdin"),
		PrintFlag:         strPtr(""),                       // NON-NIL empty — agy reads stdin; a bare -p is value-taking and breaks delivery (verified 2026-07-08, agy v1.1.0)
		ModelFlag:         strPtr("--model"),                // `-m` is REJECTED by agy (verified 2026-07-08)
		DefaultModel:      strPtr("Gemini 3.5 Flash (Low)"), // display LABEL, verbatim incl. reasoning suffix (verified 2026-07-08)
		SystemPromptFlag:  strPtr(""),                       // §12.5.1 NON-NIL empty — no sys flag; sys prepended to payload (§12.2)
		ProviderFlag:      strPtr(""),                       // §12.5.1 NON-NIL empty — agy has no sub-provider
		BareFlags: []string{
			"--mode", "plan", // read-only, never-ask profile. agy v1.1.0 has NO --approval-mode; plan = read-only (verified 2026-07-08).
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		Experimental:   boolPtr(true), // §12.5.1.1 ships experimental (tooled/stager flags, item 4, still open)
		// TooledFlags: nil — agy cannot serve as a stager until §12.5.1.1 item 4 is verified.
		// Subcommand, PromptFlag, JsonField, RetryInstruction, Env, ReasoningLevels: nil (absent).
	}
}

// builtinQwenCode returns the qwen-code (Alibaba/Qwen) manifest per PRD §12.5.2. qwen-code
// (npm @qwen-code/qwen-code; GitHub QwenLM/qwen-code) is a FORK of Google's Gemini CLI tuned for the
// Qwen3-Coder family, reached via Alibaba Cloud Model Studio / DashScope (DASHSCOPE_API_KEY, or
// `qwen-code login` for the free coding-plan quota). It is SINGLE-BACKEND (Qwen/DashScope), so
// provider_flag is empty and a bare model is used. Its flag surface mirrors the gemini-cli lineage (the
// surface the former, now-removed gemini provider used) — a Gemini-CLI fork keeps that lineage's flags:
// stdin delivery, -m model, --approval-mode default (read-only),
// no first-class system-prompt flag → sys is PREPENDED to the payload (§12.2). NOTE: agy (§12.5.1) DIVERGED
// from this lineage in v1.1.0 (--model, --mode plan, value-taking -p) and no longer matches; do NOT treat
// agy and qwen-code as identical.
//
// Flag surface assembled from qwen-code's docs (NOT yet `--help`-verified) → ships Experimental=true
// (§12.7.2) until a real end-to-end run clears it. Marked `# TO CONFIRM` per FR-D5: the exact default
// model token (qwen3-coder-plus et al.), the model-flag token, the reasoning_levels mapping, and the
// gemini-equivalent approval mode. The FR-D5 token refresh + the per-role FR-D4 tier row are S2
// (P2.M1.T1.S2); this manifest ships a correct, documented, experimental PLACEHOLDER.
//
// STAGER: TooledFlags is intentionally nil — qwen-code CANNOT serve as a stager until the scoped,
// non-interactive, git-scoped tool combo is verified (FR-D4 fallback). RenderTooled errors on nil tooled_flags.
//
// NOTE: (1) PrintFlag="-p" (NON-NIL). (2) SystemPromptFlag/ProviderFlag are strPtr("") — NON-NIL empty:
// no sys flag (sys prepended, §12.2), single-backend (no sub-provider). (3) Experimental=boolPtr(true).
// (4) DefaultModel="qwen3-coder-plus" (# TO CONFIRM FR-D5). (5) Subcommand/PromptFlag/JsonField/
// RetryInstruction/Env/TooledFlags/ReasoningLevels are nil (absent, like agy). qwen-code is the
// gemini-lineage twin of agy, differing in Name/Detect/Command + DefaultModel + the Qwen/DashScope context.
//
// CHROME-DISABLE (FR-C5, §9.28): flag surface assembled from docs (NOT yet --help-verified; # TO
// CONFIRM per FR-D5). qwen-code exposes NO known per-surface chrome-disable switch. --approval-mode
// default (bare_flags) is the read-only CONSTRAINT, not chrome. Chrome surface is unverified —
// documented LIMITATION (FR-C4). Re-verify at the FR-D5 token refresh (S2).
func builtinQwenCode() Manifest {
	return Manifest{
		Name:             "qwen-code",
		Detect:           strPtr("qwen-code"),
		Command:          strPtr("qwen-code"),
		PromptDelivery:   strPtr("stdin"),
		PrintFlag:        strPtr("-p"),
		ModelFlag:        strPtr("-m"),
		DefaultModel:     strPtr("qwen3-coder-plus"), // # TO CONFIRM per FR-D5 (S2 owns the refresh)
		SystemPromptFlag: strPtr(""),                 // NON-NIL empty — no sys flag; sys prepended to payload (§12.2)
		ProviderFlag:     strPtr(""),                 // NON-NIL empty — single-backend (Qwen/DashScope)
		BareFlags: []string{
			"--approval-mode", "default", // read-only, never-ask profile (don't auto-run tools). # TO CONFIRM gemini-equivalent
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		Experimental:   boolPtr(true), // §12.5.2/§12.7.2 ships experimental (docs-sourced, not --help-verified)
		// TooledFlags: nil — qwen-code cannot stager until the scoped tool combo is verified (FR-D4 fallback).
		// Subcommand, PromptFlag, JsonField, RetryInstruction, Env, ReasoningLevels: nil (absent, like agy).
	}
}

// builtinOpenCode returns the opencode manifest per PRD §12.6 (VERIFIED vs `opencode run --help`,
// external_deps.md §opencode), with ONE revision (delivery). opencode `run` is non-interactive and prints
// the final message to stdout. It has no global tool-disable switch and no bare flags — `run` is already a
// read-only, non-interactive one-shot (§12.7.1 "read-only constraint").
//
// REVISION (delivery): PromptDelivery="stdin" (§12.6 said "positional"). opencode `run [message..]` reads
// the prompt from STDIN when no positional message is given (verified 2026-07-08, opencode 1.1.23). stdin
// avoids the 128 KB MAX_ARG_STRLEN ceiling that positional delivery hits on ~300 KB diffs (Appendix E #1)
// — the same reason codex ships stdin (and the former gemini did). (opencode accepts a positional message too, but a single
// argv string is capped, so stdin is required for large diffs.)
//
// NOTE: (1) Subcommand = ["run"] (§12.6 writes subcommand = ["run"] → NON-NIL 1-element). (2)
// PrintFlag/DefaultModel/SystemPromptFlag/ProviderFlag are strPtr("") — §12.6 WRITES them "" (NON-NIL
// empty): `run` is already non-interactive (no print flag), user MUST set model (no single sensible
// default — model space is huge), no sys-prompt flag (sys prepended), provider is part of the model string.
// (3) BareFlags = []string{} — §12.6 writes bare_flags = []; a present empty array decodes NON-NIL empty
// (FINDING D). (4) ReasoningLevels is nil — §12.6 OMITS the key.
//
// CHROME-DISABLE (FR-C5, §9.28): verified vs `opencode run --help` (external_deps.md §opencode,
// opencode 1.1.23, 2026-07-08). The `run` subcommand is inherently read-only by design and exposes
// NO per-surface chrome-disable switch. bare_flags is empty because `run` is already a read-only
// one-shot — that is mutation safety, NOT chrome. Chrome MAY load; the call stays read-only.
// Documented LIMITATION (FR-C4).
func builtinOpenCode() Manifest {
	return Manifest{
		Name:              "opencode",
		Detect:            strPtr("opencode"),
		Command:           strPtr("opencode"),
		ListModelsCommand: []string{"opencode", "models"}, // VERIFIED 2026-07-03 via `opencode models` (exit 0); FR-L2/FR-D5.
		Subcommand:        []string{"run"},                // §12.6 `subcommand = ["run"]` → NON-NIL 1-element slice
		PromptDelivery:    strPtr("stdin"),
		PrintFlag:         strPtr(""), // §12.6 explicit empty (NON-NIL) — `run` is already non-interactive
		ModelFlag:         strPtr("-m"),
		DefaultModel:      strPtr(""), // §12.6 explicit empty (NON-NIL) — user MUST set model (Appx E #3)
		SystemPromptFlag:  strPtr(""), // §12.6 explicit empty (NON-NIL) — no sys flag on `run`; sys prepended (§12.2)
		ProviderFlag:      strPtr(""), // §12.6 explicit empty (NON-NIL) — provider is part of the model string
		BareFlags:         []string{}, // §12.6 `bare_flags = []` → NON-NIL empty slice (FINDING D); do NOT omit
		Output:            strPtr("raw"),
		StripCodeFence:    boolPtr(true),
		// PromptFlag, JsonField, RetryInstruction, Env, ReasoningLevels: nil (absent in §12.6).
	}
}

// builtinCodex returns the codex manifest per PRD §12.7 (VERIFIED vs `codex exec --help`, external_deps.md
// §codex), with TWO revisions that resolve the §codex discrepancy flagged in external_deps.md:
//
//	(1) PromptDelivery="stdin" (§12.7 said "positional") — codex exec reads stdin via "-" (external_deps.md
//	    §codex BONUS FINDING); stdin avoids arg-length limits on ~300 KB diffs.
//	(2) BareFlags=["--sandbox","read-only","--ephemeral"] (§12.7 said
//	    ["--sandbox","read-only","--ask-for-approval","never"]) — --ask-for-approval is NOT a codex exec
//	    flag (it lives on interactive `codex`; codex exec is already non-interactive); --ephemeral keeps
//	    the one-shot session-clean.
//
// codex has no global tool-disable switch; --sandbox read-only constrains it to a read-only, never-ask
// profile (§12.7.1 "read-only constraint").
//
// NOTE: (1) Subcommand=["exec"] (§12.7 subcommand = ["exec"] → NON-NIL 1-element). (2) PrintFlag/
// DefaultModel/SystemPromptFlag/ProviderFlag are strPtr("") — §12.7 WRITES them "" (NON-NIL empty): exec
// is already non-interactive (no print flag), model comes from ~/.codex/config.toml (no default), no
// sys-prompt flag (sys PREPENDED to the payload per §12.2), no sub-provider. (3) ReasoningLevels is nil
// — §12.7 OMITS the key. (4) The sys prompt is prepended (no --system-prompt flag on codex exec).
//
// VERIFIED (integration, 2026-07-08, codex-cli 0.143.0): `codex exec` writes the assistant's final
// answer to stdout and exits 0 on success — confirmed end-to-end through a z.ai OpenAI-compatible proxy
// (openai_base_url). The full flag surface above (`exec` subcommand, `-m` model, `--sandbox read-only`,
// `--ephemeral`, stdin delivery with NO positional prompt so codex reads the prompt from stdin) is valid
// against 0.143.0. Event/progress chatter — including a benign "426 Upgrade Required" when the proxy
// rejects the responses-websocket and codex falls back to HTTP — goes to STDERR; stdout carries only the
// answer. `-o <file>` (write last message to file) and `--json` (JSONL events) remain fallback channels if
// a future stdout regression appears.
//
// CHROME-DISABLE (FR-C5, §9.28): verified vs `codex exec --help` (external_deps.md §codex,
// codex-cli 0.143.0, 2026-07-08). codex exec exposes NO per-surface chrome-disable switch for
// MCP/AGENTS.md/skills. --sandbox read-only + --ephemeral (bare_flags) are the read-only,
// session-clean CONSTRAINT (mutation safety, §12.7.1), NOT chrome. Chrome MAY load; the call stays
// read-only and never-mutate. Documented LIMITATION (FR-C4).
func builtinCodex() Manifest {
	return Manifest{
		Name:             "codex",
		Detect:           strPtr("codex"),
		Command:          strPtr("codex"),
		Subcommand:       []string{"exec"}, // §12.7 `subcommand = ["exec"]` → NON-NIL 1-element slice
		PromptDelivery:   strPtr("stdin"),  // REVISED #1 from §12.7 "positional" (codex exec reads stdin via "-")
		PrintFlag:        strPtr(""),       // §12.7 explicit empty (NON-NIL) — exec is already non-interactive
		ModelFlag:        strPtr("-m"),
		DefaultModel:     strPtr(""), // §12.7 explicit empty (NON-NIL) — model from ~/.codex/config.toml
		SystemPromptFlag: strPtr(""), // §12.7 explicit empty (NON-NIL) — no sys flag on exec; sys prepended (§12.2)
		ProviderFlag:     strPtr(""), // §12.7 explicit empty (NON-NIL) — codex has no sub-provider
		BareFlags: []string{
			"--sandbox", "read-only", // read-only, never-mutate profile
			"--ephemeral", // REVISED #2: run without persisting session files (replaces invalid --ask-for-approval)
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// PromptFlag, JsonField, RetryInstruction, Env, ReasoningLevels: nil (absent in §12.7).
	}
}

// builtinCursor returns the cursor manifest per PRD §12.7 (VERIFIED vs `agent --help`, external_deps.md
// §cursor), VERBATIM (no revisions). The standalone Cursor Agent binary is `agent` (so Detect/Command =
// "agent", NOT "cursor" — cursor is the ONLY provider where detect ≠ name). cursor's -p print mode
// defaults to FULL tool access; we override with --mode ask ("Q&A style, read-only, no edits") + --trust
// (skip the workspace-trust prompt) so it cannot mutate the repo (§12.7.1 "read-only constraint").
//
// NOTE: (1) Detect/Command = "agent" (≠ Name "cursor") — §12.7 writes detect/command = "agent". (2)
// Subcommand = []string{} — §12.7 writes subcommand = []; a present empty array decodes NON-NIL empty
// (FINDING D); write it explicitly (do NOT omit → nil). (3) DefaultModel/SystemPromptFlag/ProviderFlag
// are strPtr("") — §12.7 WRITES them "" (NON-NIL empty): cursor has per-account model availability (no
// single default), no sys-prompt flag (sys prepended), no sub-provider. (4) ReasoningLevels is nil —
// §12.7 OMITS the key. (5) The sys prompt is prepended (no --system-prompt flag on agent).
//
// TO CONFIRM (integration): that `--mode ask` wins over `-p`'s default full-tools profile — i.e. the
// combo (-p --mode ask --trust) is genuinely read-only. Expected (ask is defined as read-only Q&A);
// verify against a real run during the real-agent scaffold (P1.M5.T1.S2).
//
// CHROME-DISABLE (FR-C5, §9.28): verified vs `agent --help` (external_deps.md §cursor). cursor
// exposes NO per-surface chrome-disable switch. --mode ask + --trust (bare_flags) are the read-only
// Q&A CONSTRAINT (mutation safety, §12.7.1), NOT chrome. Chrome MAY load; the call stays read-only.
// Documented LIMITATION (FR-C4).
func builtinCursor() Manifest {
	return Manifest{
		Name:              "cursor",
		Detect:            strPtr("agent"),             // §12.7 detect = "agent" — the binary is `agent` (≠ Name "cursor")
		Command:           strPtr("agent"),             // §12.7 command = "agent"
		ListModelsCommand: []string{"agent", "models"}, // VERIFIED 2026-07-03: `agent --help` lists `models`; live run exits 1 (auth required) — valid for authed users, FR-L1 fallback covers unauthed. Binary is `agent` (≠ name). FR-L2/FR-D5.
		Subcommand:        []string{},                  // §12.7 `subcommand = []` → NON-NIL empty slice (FINDING D); do NOT omit
		PromptDelivery:    strPtr("positional"),
		PrintFlag:         strPtr("-p"), // §12.7 `-p` = non-interactive (writes answer to stdout)
		ModelFlag:         strPtr("--model"),
		DefaultModel:      strPtr(""), // §12.7 explicit empty (NON-NIL) — user must set (per-account availability)
		SystemPromptFlag:  strPtr(""), // §12.7 explicit empty (NON-NIL) — no sys flag on agent; sys prepended (§12.2)
		ProviderFlag:      strPtr(""), // §12.7 explicit empty (NON-NIL) — cursor has no sub-provider
		BareFlags: []string{
			"--mode", "ask", // "Q&A style, read-only" — overrides -p's default full-tools profile
			"--trust", // skip the workspace-trust prompt (else -p would block)
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// PromptFlag, JsonField, RetryInstruction, Env, ReasoningLevels: nil (absent in §12.7).
	}
}
