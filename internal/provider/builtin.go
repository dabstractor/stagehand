package provider

// BuiltinManifests returns the compiled-in default provider manifests (PRD §12.3 pi, §12.4 claude,
// §12.5 gemini, §12.6 opencode, §12.7 codex + cursor, §12.5.1 agy), keyed by manifest name. These are the zero-config
// defaults a user override (config [provider.<name>]) merges onto via MergeManifest (S2) in the registry
// (P1.M2.T3).
//
// The manifests are constructed FRESH on every call (no package-level var): strPtr/boolPtr allocate new
// pointers and slice literals allocate new backing arrays each call, so no caller can corrupt a built-in
// by mutating a returned BareFlags/Env. MergeManifest (S2) already never mutates base, so the normal
// registry path is safe either way — fresh-per-call additionally guards against any direct mutation.
//
// The full §12.7 set: pi + claude (the "explicit tool-disable switch" pair, S1), gemini + opencode
// (read-only constraint, S2), codex + cursor (read-only constraint, S3 — codex's two revisions
// resolve the external_deps.md §codex discrepancy), and §12.5.1 agy (experimental — the Gemini-CLI
// successor). All eight providers are now present (pi, claude, gemini, opencode, codex, cursor, agy, qwen-code).
func BuiltinManifests() map[string]Manifest {
	return map[string]Manifest{
		"pi":        builtinPi(),
		"claude":    builtinClaude(),
		"gemini":    builtinGemini(),
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
// external_deps.md §pi); off ⇒ no-op (no entry). minimal/xhigh have no stagehand level.
// Per FR-D2 (PRD §9.16/§12.3), the shipped pi default is DECOUPLED from any one subscription:
// default_model is "". config init fills per-role models from the FR-D4 table; the user/config
// picks the backend (inference provider) via the model slash-prefix (v3 FR-R5b).
func builtinPi() Manifest {
	return Manifest{
		Name:             "pi",
		Detect:           strPtr("pi"),
		Command:          strPtr("pi"),
		PromptDelivery:   strPtr("stdin"),
		PrintFlag:        strPtr("-p"),
		ModelFlag:        strPtr("--model"),
		DefaultModel:     strPtr(""), // FR-D2: was glm-5-turbo; decoupled from any one subscription
		SystemPromptFlag: strPtr("--system-prompt"),
		ProviderFlag:     strPtr("--provider"),
		BareFlags: []string{
			"--no-tools",
			"--no-extensions",
			"--no-skills",
			"--no-prompt-templates",
			"--no-context-files",
			"--no-session",
		},
		// REASONING LEVELS (v3; §12.1, FR-R6). pi exposes `--thinking off|minimal|low|medium|high|xhigh`
		// (verified `pi --help`, external_deps.md §pi). off ⇒ no entry (natural zero no-op); stagehand's
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
		// (stagehand's ref-mutation monopoly, §13.6.2/§19, holds only insofar as the stager cannot itself
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

// builtinGemini returns the gemini manifest per PRD §12.5 (VERIFIED vs `gemini --help`, external_deps.md
// §gemini), with prompt_delivery REVISED to "stdin" per the work-item contract (external_deps.md §gemini
// recommendation + Appendix E #1: stdin avoids arg-length limits on ~300 KB diffs; gemini appends stdin
// to the prompt). gemini has no global tool-disable switch; `--approval-mode default` constrains it to a
// read-only, never-ask profile (§12.7.1 "read-only constraint").
//
// NOTE: (1) PrintFlag/SystemPromptFlag/ProviderFlag are strPtr("") — §12.5 WRITES them "" (NON-NIL empty):
// no print flag (positional/stdin implies one-shot), no sys-prompt flag (sys PREPENDED to the payload per
// §12.2), no sub-provider. (2) ReasoningLevels is nil — §12.5 OMITS the key. (3) The sys
// prompt is prepended to the payload (no --system-prompt flag exists on gemini-cli).
func builtinGemini() Manifest {
	return Manifest{
		Name:             "gemini",
		Detect:           strPtr("gemini"),
		Command:          strPtr("gemini"),
		PromptDelivery:   strPtr("stdin"), // REVISED from §12.5 "positional" (work-item + external_deps.md + Appx E #1)
		PrintFlag:        strPtr(""),      // §12.5 explicit empty (NON-NIL) — positional/stdin implies one-shot
		ModelFlag:        strPtr("-m"),
		DefaultModel:     strPtr("gemini-3.1-pro"), // WAS gemini-2.5-pro — refreshed per FR-D5 (verified 2026-07-02)
		SystemPromptFlag: strPtr(""),               // §12.5 explicit empty (NON-NIL) — no sys flag; sys prepended to payload (§12.2)
		ProviderFlag:     strPtr(""),               // §12.5 explicit empty (NON-NIL) — gemini has no sub-provider
		BareFlags: []string{
			"--approval-mode", "default", // read-only, never-ask profile (don't auto-run tools)
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// Subcommand, PromptFlag, JsonField, RetryInstruction, Env, ReasoningLevels: nil (absent in §12.5).
	}
}

// builtinAgy returns the agy (Google Antigravity CLI) manifest per PRD §12.5.1 (the Gemini-CLI successor,
// superseded gemini on 2026-06-18). Flag surface VERIFIED vs `agy --help` + live -p runs on 2026-07-03:
// the model flag is `--model` ONLY (`-m` is rejected: "flags provided but not defined"; agy defines short
// aliases only for -c/-i/-p). Still ships Experimental=true (§12.7.2) until the remaining §12.5.1.1 items
// clear. agy has no first-class system-prompt flag → sys is PREPENDED to the payload (§12.2), like gemini.
// `--approval-mode default` is a read-only, never-ask profile (§12.7.1 "read-only constraint").
//
// MODEL NAMES (verified 2026-07-03): agy's --model takes the DISPLAY LABEL from `agy models` VERBATIM,
// reasoning level included — e.g. "Gemini 3.5 Flash (Low)" / "Gemini 3.1 Pro (High)". Reasoning is NOT a
// separate flag (ReasoningLevels stays nil); it is baked into the label's parenthesized suffix. API-style
// ids (gemini-3.5-flash) are NOT recognized and SILENTLY fall back to agy's own default (the backend logs
// "Requested entity was not found" but the run succeeds on the fallback model) — so these labels, spaces
// and all, are the only safe tokens.
//
// §12.5.1.1 item 1 (issue #76, non-TTY stdout drop): NO LONGER REPRODUCES — 2026-07-03 live `agy -p` runs
// from a non-TTY returned stdout correctly. Kept experimental until the full §12.5.1.1 checklist clears.
//
// STAGER: TooledFlags is intentionally nil — agy CANNOT serve as a stager until §12.5.1.1 item 4 (the
// scoped, non-interactive, git-scoped tool combo) is verified. RenderTooled errors on nil tooled_flags.
//
// NOTE: (1) PrintFlag="-p" (NON-NIL). (2) SystemPromptFlag/ProviderFlag are strPtr("") — §12.5.1 WRITES
// them "" (NON-NIL empty): no sys flag (sys prepended, §12.2), no sub-provider. (3) default_model is
// "Gemini 3.5 Flash (Low)" (label form; refreshed from gemini-3.1-pro per FR-D5, verified 2026-07-03).
// (4) Experimental=boolPtr(true) (ships experimental).
// (5) Subcommand/PromptFlag/JsonField/RetryInstruction/Env/TooledFlags/ReasoningLevels are nil (absent,
// like gemini). agy is the Gemini-lineage twin of gemini, differing in default_model + Experimental.
func builtinAgy() Manifest {
	return Manifest{
		Name:             "agy",
		Detect:           strPtr("agy"),
		Command:          strPtr("agy"),
		PromptDelivery:   strPtr("stdin"),
		PrintFlag:        strPtr("-p"),
		ModelFlag:        strPtr("--model"),               // `-m` is REJECTED by agy (verified 2026-07-03)
		DefaultModel:     strPtr("Gemini 3.5 Flash (Low)"), // display LABEL, verbatim incl. reasoning suffix (verified 2026-07-03)
		SystemPromptFlag: strPtr(""),               // §12.5.1 NON-NIL empty — no sys flag; sys prepended to payload (§12.2)
		ProviderFlag:     strPtr(""),               // §12.5.1 NON-NIL empty — agy has no sub-provider
		BareFlags: []string{
			"--approval-mode", "default", // read-only, never-ask profile (don't auto-run tools)
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		Experimental:   boolPtr(true), // §12.5.1.1 ships experimental (non-TTY stdout drop, issue #76)
		// TooledFlags: nil — agy cannot serve as a stager until §12.5.1.1 item 4 is verified.
		// Subcommand, PromptFlag, JsonField, RetryInstruction, Env, ReasoningLevels: nil (absent, like gemini).
	}
}

// builtinQwenCode returns the qwen-code (Alibaba/Qwen) manifest per PRD §12.5.2. qwen-code
// (npm @qwen-code/qwen-code; GitHub QwenLM/qwen-code) is a FORK of Google's Gemini CLI tuned for the
// Qwen3-Coder family, reached via Alibaba Cloud Model Studio / DashScope (DASHSCOPE_API_KEY, or
// `qwen-code login` for the free coding-plan quota). It is SINGLE-BACKEND (Qwen/DashScope), so
// provider_flag is empty and a bare model is used. Its flag surface mirrors gemini/agy (§12.5/§12.5.1)
// EXACTLY: stdin delivery, -m model, --approval-mode default (read-only), no first-class system-prompt
// flag → sys is PREPENDED to the payload (§12.2).
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
// external_deps.md §opencode), VERBATIM (no revisions). opencode `run` is non-interactive and prints the
// final message to stdout. It has no global tool-disable switch and no bare flags — `run` is already a
// read-only, non-interactive one-shot (§12.7.1 "read-only constraint").
//
// NOTE: (1) Subcommand = ["run"] (§12.6 writes subcommand = ["run"] → NON-NIL 1-element). (2)
// PrintFlag/DefaultModel/SystemPromptFlag/ProviderFlag are strPtr("") — §12.6 WRITES them "" (NON-NIL
// empty): `run` is already non-interactive (no print flag), user MUST set model (no single sensible
// default — model space is huge), no sys-prompt flag (sys prepended), provider is part of the model string.
// (3) BareFlags = []string{} — §12.6 writes bare_flags = []; a present empty array decodes NON-NIL empty
// (FINDING D). (4) ReasoningLevels is nil — §12.6 OMITS the key.
func builtinOpenCode() Manifest {
	return Manifest{
		Name:             "opencode",
		Detect:           strPtr("opencode"),
		Command:          strPtr("opencode"),
		Subcommand:       []string{"run"}, // §12.6 `subcommand = ["run"]` → NON-NIL 1-element slice
		PromptDelivery:   strPtr("positional"),
		PrintFlag:        strPtr(""), // §12.6 explicit empty (NON-NIL) — `run` is already non-interactive
		ModelFlag:        strPtr("-m"),
		DefaultModel:     strPtr(""), // §12.6 explicit empty (NON-NIL) — user MUST set model (Appx E #3)
		SystemPromptFlag: strPtr(""), // §12.6 explicit empty (NON-NIL) — no sys flag on `run`; sys prepended (§12.2)
		ProviderFlag:     strPtr(""), // §12.6 explicit empty (NON-NIL) — provider is part of the model string
		BareFlags:        []string{}, // §12.6 `bare_flags = []` → NON-NIL empty slice (FINDING D); do NOT omit
		Output:           strPtr("raw"),
		StripCodeFence:   boolPtr(true),
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
// TO CONFIRM (integration): that `codex exec` writes the assistant's final answer to stdout and exits 0
// on success. Expected; -o <file> (write last message to file) and --json (JSONL events) are fallback
// output channels if stdout proves unreliable. Verify during the real-agent scaffold (P1.M5.T1.S2).
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
func builtinCursor() Manifest {
	return Manifest{
		Name:             "cursor",
		Detect:           strPtr("agent"), // §12.7 detect = "agent" — the binary is `agent` (≠ Name "cursor")
		Command:          strPtr("agent"), // §12.7 command = "agent"
		Subcommand:       []string{},      // §12.7 `subcommand = []` → NON-NIL empty slice (FINDING D); do NOT omit
		PromptDelivery:   strPtr("positional"),
		PrintFlag:        strPtr("-p"), // §12.7 `-p` = non-interactive (writes answer to stdout)
		ModelFlag:        strPtr("--model"),
		DefaultModel:     strPtr(""), // §12.7 explicit empty (NON-NIL) — user must set (per-account availability)
		SystemPromptFlag: strPtr(""), // §12.7 explicit empty (NON-NIL) — no sys flag on agent; sys prepended (§12.2)
		ProviderFlag:     strPtr(""), // §12.7 explicit empty (NON-NIL) — cursor has no sub-provider
		BareFlags: []string{
			"--mode", "ask", // "Q&A style, read-only" — overrides -p's default full-tools profile
			"--trust", // skip the workspace-trust prompt (else -p would block)
		},
		Output:         strPtr("raw"),
		StripCodeFence: boolPtr(true),
		// PromptFlag, JsonField, RetryInstruction, Env, ReasoningLevels: nil (absent in §12.7).
	}
}
