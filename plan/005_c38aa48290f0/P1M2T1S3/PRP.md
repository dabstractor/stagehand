name: "P1.M2.T1.S3 — Format-mode prompt scaffolds + locale line, applied everywhere a message is produced"
description: |
  Wire cfg.Format (auto|conventional|gitmoji|plain, from S1) and cfg.Locale (free-form, from S1) into the
  system-prompt builders so a non-`auto` format REPLACES the style-examples block (the "Match the tone…"
  section AND the anti-reuse warning; history examples are not fetched/embedded) with an explicit per-mode
  scaffold (§17.8), and a non-empty locale APPENDS one line — `Write the commit message in <lang>.` — in
  EVERY mode and both repo-age variants. This is the FR-F1–F6 prompt half: it consumes S1's config
  (cfg.Format/cfg.Locale) and S2's compiled-in gitmoji table (prompt.RenderGitmojiTable). Signature changes
  land on `BuildSystemPrompt`, `BuildFallbackPrompt`, and `BuildPlannerSystemPrompt`; the four call sites
  (message role, decompose message, public API, planner) thread the two config fields through. `auto` +
  empty-locale output stays BYTE-IDENTICAL to today (FR-F1). This subtask does NOT touch config/flags
  (S1, landed), `--context` (S2.T2.S1), or `--template`/message-finalization (S2.T2.S2).

---

## Goal

**Feature Goal**: Every place stagecoach produces a commit message honors the resolved `--format` mode and
`--locale`: `auto` behaves exactly as today; `conventional`/`gitmoji`/`plain` swap the learned-style block
for an explicit format contract (and omit the history examples entirely); and a set locale appends the
one-line language instruction in all modes and both repo-age paths. The planner's *partitioning* prompt is
untouched (FR-F5) except that its single-call-shortcut message (FR-M11) obeys the same substitution + locale.

**Deliverable**: Format/locale-aware prompt builders in `internal/prompt/` (new `format.go` + edits to
`system.go` and `planner.go`), with the three builder signatures extended and the four non-test call sites
updated to pass `cfg.Format` / `cfg.Locale`. Table-driven per-mode prompt tests plus a stub-agent
integration test asserting the rendered system prompt reflects the mode. Docs: a `docs/how-it-works.md`
prompt-construction subsection on format-mode substitution and the locale line.

**Success Definition**:
- `cfg.Format == "auto"` **and** `cfg.Locale == ""` → every builder emits BYTE-IDENTICAL output to today
  (the existing `*_CanonicalExact` tests pass unchanged — this is the load-bearing regression guard, FR-F1).
- `conventional` → the "Match the tone and style…" line, the anti-reuse block, and all `---` example lines
  are ABSENT; the prompt contains `type(scope): description` and the standard type vocabulary
  (`feat fix docs style refactor perf test build ci chore revert`); the multi-line rule and subject-target
  line are retained (FR-F2).
- `gitmoji` → the examples block is replaced by the "Begin the subject with exactly ONE emoji…" instruction
  followed by `prompt.RenderGitmojiTable()` (contains e.g. `🎨`); no `---` example lines (FR-F3).
- `plain` → no examples, no format contract; only the output rules, essence instruction, multi-line rule,
  and subject-target line (FR-F4).
- Any non-empty `cfg.Locale` appends exactly one line `Write the commit message in <lang>.` in every mode
  and in both the mature (`BuildSystemPrompt`) and new-repo (`BuildFallbackPrompt`) paths, and on the
  planner builder (FR-F6). Empty locale → no such line (byte-identical to no-locale).
- Applied at ALL message-production sites: message role (`internal/generate`), decompose message
  (`internal/decompose/message.go`), the public API (`pkg/stagecoach`), and the planner FR-M11 shortcut
  (`internal/decompose/planner.go`). The arbiter N+1 message inherits automatically (it flows through the
  decompose message path). The arbiter *decision* prompt (`BuildArbiterSystemPrompt`) is unchanged.
- `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run` all green; coverage on
  `internal/generate` and `internal/config`-adjacent packages stays ≥85%.

## User Persona

**Target User**: The "plan-holder" / "API-key refusenik" (PRD §7) who runs `stagecoach --format gitmoji`,
`--format conventional`, `--format plain`, or `--locale French` (or sets `stagecoach.format` /
`[generation].format` / `STAGECOACH_FORMAT`, resolved by S1). Immediate consumer: the hook-exec runtime
(P1.M3.T2.S1), which builds the message-role prompt through these same builders.

**Use Case**: A user whose repo history is idiosyncratic (or empty) wants a clean, explicit commit format
without teaching stagecoach from bad examples — `--format conventional`/`gitmoji`/`plain` — or wants
messages authored in another language — `--locale "Spanish"`. Both compose (`--format gitmoji --locale ja`).

**User Journey**: S1 resolves `cfg.Format`/`cfg.Locale` through the precedence chain and validates Format →
the message/planner/arbiter sites call the (now format/locale-aware) builders → the model receives the
mode scaffold (not the learned-style block) plus the locale line → emits e.g. `🎨 Refactor auth flow`
in the requested language. Duplicate rejection (§9.7) still runs in every mode (unchanged; it operates on
the generated subject, not the prompt).

**Pain Points Addressed**: (1) Incumbents ship 20-locale i18n prompt files; stagecoach appends one sentence
and lets the model do it (FR-F6). (2) A repo with a weird/empty history can't produce a sane learned-style
prompt; the format modes are the "ignore my history" escape hatch (FR-F1/F4). (3) gitmoji users get the
canonical table compiled in, offline (S2's `RenderGitmojiTable`), no network fetch.

## Why

- **FR-F1 (PRD §9.19)**: `--format` mode ∈ auto|conventional|gitmoji|plain, default auto. "Any other mode
  **replaces the style-examples block** in the system prompt with that mode's explicit instructions (§17.8);
  the history examples are omitted entirely." This subtask IS that replacement.
- **FR-F5**: "Format applies everywhere a message is produced. The message role, the planner's single-call
  shortcut message (FR-M11), and the arbiter-triggered (N+1)-th commit message all honor the resolved
  format. The planner's *partitioning* prompt is unaffected." → four call sites, planner contract unchanged.
- **FR-F6**: Locale appends one line `Write the commit message in <lang>.` in every format mode and both
  repo-age variants; empty = no line; passed verbatim (no validation, no i18n tables).
- **§17.8** is the exact scaffold spec: non-auto REPLACES the "Match the tone…" block + the anti-reuse
  warning; retains the output rules, essence-not-filenames instruction, and multi-line rule; conventional /
  gitmoji / plain bodies as quoted below; locale appended to any mode + both repo-age variants; the
  planner's partitioning prompt is unchanged but its FR-M11 message undergoes the same substitution.
- **Scope fence**: S1 (landed) owns `cfg.Format`/`cfg.Locale` + validation; S2 (landed) owns the gitmoji
  table (`prompt.RenderGitmojiTable`). S3 is the prompt-scaffold + locale-line + swap-wiring consumer of
  both. `--context` (user payload) is S2.T2.S1; `--template` + the shared message-finalization seam is
  S2.T2.S2 — do NOT build those here.

## What

Extend the three system-prompt builders to be format/locale-aware, dispatch inside the `prompt` package,
and thread `cfg.Format`/`cfg.Locale` at the four call sites. Behavior by mode (all built from S1's resolved
`cfg.Format`, already validated to one of the four values):

| Mode | Style-examples block | Scaffold body inserted | Multi-line rule + subject target | Locale line (if set) |
|---|---|---|---|---|
| `auto` | kept (verbatim today) | none | kept | appended |
| `conventional` | **removed** (no examples, no anti-reuse) | `type(scope): description` + type vocab | kept | appended |
| `gitmoji` | **removed** | "Begin the subject with exactly ONE emoji…" + `RenderGitmojiTable()` | kept | appended |
| `plain` | **removed** | none (no format contract) | kept | appended |

### Success Criteria

- [ ] `BuildSystemPrompt`, `BuildFallbackPrompt`, `BuildPlannerSystemPrompt` gain `format, locale string`
      params; all four non-test callers + all test callers updated.
- [ ] `auto` + empty locale is byte-identical to today (existing canonical-exact tests unchanged & green).
- [ ] `conventional`/`gitmoji`/`plain` replace the examples block per §17.8; multi-line rule + subject
      target retained; locale line appended in every mode when set.
- [ ] Planner: partitioning contract unchanged (FR-F5); style-examples block swapped for the scaffold body
      when non-auto; locale appended (FR-M11 shortcut message obeys the mode).
- [ ] Arbiter decision prompt unchanged; N+1 message inherits via the decompose message path.
- [ ] Table-driven per-mode tests + a stub-agent test asserting the rendered system prompt.
- [ ] `docs/how-it-works.md` prompt-construction subsection documents format-mode substitution + locale.
- [ ] Full build/test/vet/lint green.

## All Needed Context

### Context Completeness Check

_This PRP names the exact builder signatures to change, the four call sites (with line numbers), the exact
constant to split (`maturePromptHeader`) and the compile-time-concat trick that keeps `auto` byte-identical,
the verbatim §17.8 scaffold text, the S1 config fields and S2 renderer to consume, the exact test file &
patterns to mirror (including the stub stdin-capture knob), and the scope fences vs S2.T2. An implementer
with no prior codebase knowledge can complete it from this document + codebase access._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/prd_snapshot.md
  why: The authoritative spec. §9.19 (FR-F1..F6) and §17.8 are the contract. §17.1/§17.2 are the auto-mode
       prompts that must be preserved byte-identically; §17.5 is the planner contract that must stay unchanged.
  section: "§9.19 Message shaping" + "§17.8 Format modes, locale, and context" + "§17.1/§17.2/§17.5"
  critical: |
    §17.8 verbatim scaffold bodies (use these exact strings):
      conventional: "Format: type(scope): description. type ∈ feat|fix|docs|style|refactor|perf|test|build|
                     ci|chore|revert; scope optional."
      gitmoji:      "Begin the subject with exactly ONE emoji from the gitmoji list below (the emoji
                     character itself, not a :shortcode:), followed by a space and the description."
                     — then a blank line, then RenderGitmojiTable().
      plain:        no scaffold body (no examples, no format contract).
    Locale (FR-F6): "Write the commit message in <lang>." — ONE line, any mode, both repo-age variants.
    FR-F5: planner PARTITIONING prompt unchanged; only its FR-M11 message obeys the substitution + locale.

- docfile: plan/005_c38aa48290f0/architecture/system_context.md
  why: §3 "internal/prompt" states plainly: "No format/locale/context seams exist. The style-examples block
       is unconditional; §17.8 requires it to become swappable (conventional/gitmoji/plain replace it
       entirely), locale is a one-line append." §4 lists the testing patterns (stub agent, table-driven).
       §5 maps §9.19 → "swappable examples block + 3 scaffolds (§17.8), gitmoji constant, locale line".
  section: "## 3 internal/prompt" + "## 4 Testing patterns" + "## 5 Feature → seam map"
  critical: "This is a NET-NEW seam: no existing conditional in the builders. Add the dispatch inside the
             prompt package so the four call sites change minimally (just thread two args)."

- file: internal/prompt/system.go
  why: THE file to edit. `maturePromptHeader` (lines 19-27) bundles the role + output rules + essence + the
       "Match the tone and style…" intro line as its LAST line. `antiReuseProhibition` (32-36),
       `multilineRuleAllow`/`multilineRuleSingle` (40-45), `subjectTargetLine(int)` (50-52),
       `fallbackPromptBody` (131-136), `BuildFallbackPrompt(int)` (160-163), `BuildSystemPrompt(examples,
       hasMultiline, subjectTarget)` (165-198). Constants carry NO trailing newline; the Build* funcs own
       ALL inter-block newline placement (design rule — keep it).
  pattern: |
    - Split maturePromptHeader into `promptPreamble` (role+output-rules+essence, NO Match line) and
      `examplesIntro` ("Match the tone and style of these recent commits from this repository:"), then
      `const maturePromptHeader = promptPreamble + "\n\n" + examplesIntro` (Go compile-time string-constant
      concatenation → auto path stays byte-identical automatically; the CanonicalExact test proves it).
    - Non-auto message prompt = promptPreamble + "\n\n" + [scaffold body + "\n\n" if non-empty] +
      <multiline rule> + "\n" + subjectTargetLine(subjectTarget), then locale suffix.
  gotcha: "The ONLY non-ASCII byte in system.go today is the em-dash (U+2014) in antiReuseProhibition; the
           gitmoji scaffold adds emoji glyphs via RenderGitmojiTable. Keep files UTF-8; run gofmt (no diff)."

- file: internal/prompt/planner.go
  why: `plannerSystemPrompt` const (26-40) is the §17.5 partitioning contract — MUST stay verbatim (FR-F5).
       `BuildPlannerSystemPrompt(examples []string)` (80-91) appends "\n\n" + the "---\n<msg>\n" examples
       loop. Change its signature to (examples, format, locale) and, when format != "auto", append the
       scaffold BODY instead of the examples loop; append the locale suffix in all modes.
  pattern: "auto → plannerSystemPrompt + \"\\n\\n\" + examples-loop (today) + locale-suffix; non-auto →
            plannerSystemPrompt + \"\\n\\n\" + formatScaffoldBody(format) + locale-suffix. Reuse the SAME
            formatScaffoldBody helper as the message builder (DRY)."
  gotcha: "Do NOT edit plannerSystemPrompt or BuildPlannerUserPayload — the partitioning prompt is unchanged."

- file: internal/prompt/gitmoji.go
  why: S2 (landed). `prompt.RenderGitmojiTable() string` renders the §17.8 'emoji + meaning' block, one
       line per entry '<emoji> - <description>', NO trailing newline (caller owns placement). The gitmoji
       scaffold body embeds this. `prompt.GitmojiTable` ([]Gitmoji, 75 entries) is also available if a
       different separator is ever wanted (not needed here).
  pattern: "gitmoji scaffold body = instruction line + \"\\n\\n\" + RenderGitmojiTable()."
  gotcha: "Do NOT re-fetch or re-embed the table; call RenderGitmojiTable(). FR-F3: no network fetch, ever."

- file: internal/config/config.go
  why: S1 (landed). `Config.Format string` (line 85, values auto|conventional|gitmoji|plain, default "auto")
       and `Config.Locale string` (line 89, free-form, default ""). Consumed as `cfg.Format` / `cfg.Locale`.
       There is NO typed enum — compare against the literal mode strings. Format is already VALIDATED by S1
       (`validateFormat`, load.go:356) — the builders can trust cfg.Format is one of the four values, but
       should still default any unexpected value to the auto path (defensive; never panic).
  pattern: "Read cfg.Format / cfg.Locale directly off config.Config (they are NOT role-scoped; do NOT route
            them through ResolveRoleModel)."
  gotcha: "Locale is deliberately NOT validated (FR-F6) — pass it verbatim into the sentence."

- file: internal/generate/generate.go
  why: CALL SITE #1 (message role). `buildSystemPrompt(ctx, g, cfg, isUnborn)` (312-328) calls
       BuildFallbackPrompt(cfg.SubjectTargetChars) at 314 & 321 and BuildSystemPrompt(msgs,
       DetectMultiline(msgs), cfg.SubjectTargetChars) at 327. cfg is config.Config (has Format/Locale).
  pattern: "Add cfg.Format, cfg.Locale to all three calls. NO new branching needed (see Key Details)."

- file: internal/decompose/message.go
  why: CALL SITE #2 (decompose message role; also feeds the arbiter N+1 message via generateMessage).
       `messageSystemPrompt(ctx, g, cfg, isUnborn)` (227-243) is a byte-for-byte re-port of site #1 (same
       three calls at 229/236/242), using deps.Config. Update identically.
  pattern: "Add cfg.Format, cfg.Locale to the three calls (cfg here = deps.Config)."

- file: pkg/stagecoach/stagecoach.go
  why: CALL SITE #3 (public API wrapper — a THIRD verbatim copy of the message-prompt helper). Lines
       395/402/408 call BuildFallbackPrompt/BuildSystemPrompt exactly like sites #1/#2. MUST be updated or
       the signature change fails to compile.
  pattern: "Add cfg.Format, cfg.Locale to the three calls. Confirm cfg here is config.Config with the fields."
  gotcha: "This caller is easy to miss — grep confirms it. A missed caller = build break."

- file: internal/decompose/planner.go
  why: CALL SITE #4 (planner FR-M11). `callPlanner` (62) builds the system prompt at line 84:
       `sysPrompt := prompt.BuildPlannerSystemPrompt(examples)`. Change to
       `BuildPlannerSystemPrompt(examples, deps.Config.Format, deps.Config.Locale)`. The single-shortcut
       message is emitted BY the planner per its JSON contract, so format/locale must reach the planner
       system prompt (the message role never runs for FR-M11).
  pattern: "One-line change at planner.go:84. Do NOT touch plannerExamples / BuildPlannerUserPayload."

- file: internal/prompt/system_test.go
  why: THE test-pattern model. TestBuildSystemPrompt_CanonicalExact (13) compares full output to a const
       `want` built by "..." + "\n" concatenation. TestBuildSystemPrompt_Properties (56) is table-driven
       with a `check func(t, p string)` per case using strings.Contains/Count/Index.
       TestBuildFallbackPrompt_* and TestDetectMultiline mirror this. Helpers `suffix`/`near` (328/336) for
       readable failures. Package `prompt` (internal) — unexported consts/helpers are reachable.
  pattern: |
    - Keep the existing CanonicalExact tests (update call args to `..., "auto", ""`) — they PROVE FR-F1
      byte-identity; do not weaken them.
    - Add per-mode subtests: conventional/gitmoji/plain × {locale "", "French"} asserting scaffold present,
      examples/anti-reuse ABSENT, multiline+target retained, locale line present/absent.
  gotcha: "Stdlib testing only (no testify). t.Run + t.Errorf('%q'…) house style."

- file: internal/generate/generate_test.go
  why: THE stub-agent integration pattern. Lines 555-592 (the exclusion test) set
       `STAGECOACH_STUB_STDINFILE` via t.Setenv, run CommitStaged with a stubtest.Manifest, then
       os.ReadFile the captured stdin and assert on its content. Because the stub manifest has NO
       system_prompt_flag and uses stdin delivery, provider.Render PREPENDS the system prompt to the
       payload with a "\n\n" delimiter (render.go:157) — so the captured stdin CONTAINS the rendered
       system prompt. Lines 420-452 show the ArgsFile knob (argv capture) for reference.
  pattern: |
    - Build a real temp repo (mature: ≥2 commits so BuildSystemPrompt path runs), stubtest.Build(t),
      stubtest.Manifest(bin, Options{Out: "🎨 refactor thing", StdinFile knob via t.Setenv}).
    - cfg := config.Config{Provider:"stub", Model:"stub", Timeout:…, Format:"gitmoji", Locale:"French",
      SubjectTargetChars:50, MaxDuplicateRetries:…}. Run CommitStaged; read captured stdin.
    - Assert the captured stdin (system-prompt half) Contains "Begin the subject with exactly ONE emoji",
      Contains a table row (e.g. "🎨 - "), Contains "Write the commit message in French.", and does NOT
      Contain "Match the tone and style".
  gotcha: "STAGECOACH_STUB_STDINFILE is an ENV knob read by cmd/stubagent/main.go:36 (t.Setenv it directly),
           not an Options field. The stub Out can be any non-empty string that passes the dedupe check."

- docfile: docs/how-it-works.md
  why: Mode-A doc target. The 'Prompt engineering' section (line 183) has 'System prompt (mature repos)',
       'System prompt (new repos)', 'User payload'. Add a short subsection on format-mode substitution +
       the locale line (concise, user-facing; no PRD-section jargon).
  section: "## Prompt engineering (line 183)"
  critical: "Mode A = user-facing docs required. Keep it brief and accurate; do not document --context or
             --template (not this subtask)."
```

### Current Codebase tree (relevant slice)

```bash
internal/prompt/
  system.go        # BuildSystemPrompt/BuildFallbackPrompt (§17.1/§17.2) — EDIT (split header, add params, dispatch)
  planner.go       # BuildPlannerSystemPrompt (§17.5) — EDIT (add params, swap examples↔scaffold, locale)
  gitmoji.go       # RenderGitmojiTable() / GitmojiTable — CONSUME (S2, landed)
  payload.go       # BuildUserPayload (§17.3) — unchanged (context injection is S2.T2.S1)
  arbiter.go       # BuildArbiterSystemPrompt() — unchanged (decision prompt; no message, no examples)
  stager.go        # unchanged
  *_test.go        # system_test.go / planner_test.go call the builders → update signatures + add cases
internal/generate/generate.go        # buildSystemPrompt (312-328) — CALL SITE #1
internal/decompose/message.go        # messageSystemPrompt (227-243) — CALL SITE #2
pkg/stagecoach/stagecoach.go           # buildSystemPrompt copy (395-408) — CALL SITE #3
internal/decompose/planner.go        # callPlanner (84) — CALL SITE #4
internal/config/config.go            # Config.Format / Config.Locale (S1, landed) — READ-ONLY
docs/how-it-works.md                 # 'Prompt engineering' section — EDIT (Mode A doc)
```

### Desired Codebase tree with files to be added

```bash
internal/prompt/format.go        # NEW — format scaffold bodies (conventional/gitmoji/plain), localeSuffix
                                 #       helper, and the shared buildFormatSystemPrompt assembler. Keeps
                                 #       system.go/planner.go edits minimal.
internal/prompt/format_test.go   # NEW — table-driven per-mode tests for the scaffold bodies + locale suffix.
# (system.go, planner.go, system_test.go, planner_test.go, the 4 call-site files, docs/how-it-works.md: EDIT)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (FR-F1): `auto` + empty-locale MUST be byte-identical to today. Achieve it by (a) splitting
// maturePromptHeader via COMPILE-TIME constant concatenation so the assembled bytes don't change, and
// (b) gating the locale suffix on `locale != ""`. The existing *_CanonicalExact tests are the proof — keep
// them (only update call args to add "auto", "").

// CRITICAL: There are FOUR callers of BuildSystemPrompt/BuildFallbackPrompt (generate.go, decompose/
// message.go, pkg/stagecoach/stagecoach.go) and ONE of BuildPlannerSystemPrompt (decompose/planner.go). ALL
// must be updated for the signature change or the build breaks. grep to confirm none are missed:
//   grep -rn 'BuildSystemPrompt\|BuildFallbackPrompt\|BuildPlannerSystemPrompt' --include='*.go' | grep -v 'func Build'

// CRITICAL: NO new call-site branching. Both BuildSystemPrompt AND BuildFallbackPrompt dispatch to the
// scaffold when format != "auto". So the repo-age branch in each caller (isUnborn/≤1 → Fallback; else
// mature) stays as-is: for a mature repo in non-auto mode BuildSystemPrompt is called (examples IGNORED,
// hasMultiline honored — FR-F2 "FR12 detection still runs"); for a new repo BuildFallbackPrompt is called
// (hasMultiline implicitly false, correct — no history). Callers just thread cfg.Format, cfg.Locale.

// GOTCHA: In non-auto modes the `examples []string` arg to BuildSystemPrompt is IGNORED (history is not
// embedded — FR-F1). Callers may still pass the fetched msgs (cheap) so hasMultiline is real; that is fine.
// Do NOT add logic to skip the RecentMessages fetch — keep the caller diff to two extra args.

// GOTCHA: cfg.Format is a plain string, already validated by S1 to one of {auto,conventional,gitmoji,plain}.
// The builder's switch should default any UNKNOWN value to the auto path (defensive; the validated invariant
// makes this unreachable, but never panic). Do NOT re-validate here (S1 owns that; re-validating duplicates).

// GOTCHA: Locale is passed VERBATIM (FR-F6): "Write the commit message in " + locale + "." No trimming
// beyond the empty check, no BCP-47 parsing, no i18n table. `locale=="fr"` and `locale=="French"` both work.

// GOTCHA (planner, FR-F5): plannerSystemPrompt (the §17.5 partitioning contract) is UNCHANGED. Only the
// trailing style-examples block is swapped for the scaffold body when non-auto; the contract line "Match the
// repository's commit style shown below…" then refers to the scaffold, which is acceptable and minimal.

// GOTCHA: The arbiter DECISION prompt (BuildArbiterSystemPrompt, zero-arg) is untouched — the arbiter emits
// {"target":…}, not a message. The N+1 leftover commit's MESSAGE is produced by the decompose message path
// (generateMessage → messageSystemPrompt), so fixing CALL SITE #2 covers the arbiter N+1 automatically.

// GOTCHA: constants carry NO trailing newline; the assembler owns inter-block "\n" placement (existing rule).
// RenderGitmojiTable() also returns no trailing newline. Match this — pin exact bytes in a canonical test.
```

## Implementation Blueprint

### Data models and structure

No new types. New unexported constants + helpers in `internal/prompt/format.go`:

```go
package prompt

import "strings"

// §17.8 conventional scaffold body (FR-F2). Replaces the style-examples block. The subject-target line is
// appended separately by the caller (config-driven), so it is NOT duplicated here.
const conventionalScaffold = "Format: type(scope): description. type ∈ feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert; scope optional."

// §17.8 gitmoji scaffold instruction (FR-F3). The compiled-in table (S2) is appended after a blank line.
const gitmojiScaffoldInstruction = "Begin the subject with exactly ONE emoji from the gitmoji list below (the emoji character itself, not a :shortcode:), followed by a space and the description."

// formatScaffoldBody returns the §17.8 mode-specific contract block that REPLACES the style-examples block.
// Empty string for "auto" (auto keeps the examples block, handled by the caller) and "plain" (no format
// contract). Any unknown mode (should be unreachable — S1 validates) → "" (defensive, auto-like).
func formatScaffoldBody(format string) string {
	switch format {
	case "conventional":
		return conventionalScaffold
	case "gitmoji":
		return gitmojiScaffoldInstruction + "\n\n" + RenderGitmojiTable()
	default: // "auto", "plain", or unknown
		return ""
	}
}

// withLocale appends the FR-F6 locale instruction as ONE line, or returns s unchanged when locale is empty.
// Normalizes to a single "\n" separator so every builder shares one rule (auto/scaffold/planner alike).
func withLocale(s, locale string) string {
	if locale == "" {
		return s
	}
	return strings.TrimRight(s, "\n") + "\nWrite the commit message in " + locale + "."
}
```

Shared message-prompt assembler for the non-auto path (also in `format.go`), reused by both
`BuildSystemPrompt` and `BuildFallbackPrompt`:

```go
// buildFormatSystemPrompt assembles the non-auto message system prompt (§17.8): the shared preamble
// (role + output rules + essence — NO "Match the tone…" line), the mode scaffold body (empty for plain),
// the retained multi-line rule (selected by hasMultiline — FR-F2 detection still runs), and the
// subject-target line. Locale is applied by the caller via withLocale.
func buildFormatSystemPrompt(format string, hasMultiline bool, subjectTarget int) string {
	var b strings.Builder
	b.WriteString(promptPreamble)
	b.WriteString("\n\n")
	if body := formatScaffoldBody(format); body != "" {
		b.WriteString(body)
		b.WriteString("\n\n")
	}
	if hasMultiline {
		b.WriteString(multilineRuleAllow)
	} else {
		b.WriteString(multilineRuleSingle)
	}
	b.WriteByte('\n')
	b.WriteString(subjectTargetLine(subjectTarget))
	return b.String()
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/prompt/system.go — split the header constant (preserve auto byte-identity)
  - REPLACE the single maturePromptHeader literal with three consts:
      promptPreamble = `You are a commit message generator.\n\nOutput ONLY the commit message...function names.`
        (i.e. today's maturePromptHeader MINUS the final "Match the tone…" line and its preceding blank line)
      examplesIntro  = "Match the tone and style of these recent commits from this repository:"
      maturePromptHeader = promptPreamble + "\n\n" + examplesIntro   // compile-time concat; auto stays identical
  - VERIFY: the existing TestBuildSystemPrompt_CanonicalExact still passes with the auto call (proves identity).
  - PLACEMENT: system.go. Keep the "constants carry no trailing newline" doc note.

Task 2: CREATE internal/prompt/format.go — scaffold bodies + withLocale + buildFormatSystemPrompt
  - IMPLEMENT: conventionalScaffold, gitmojiScaffoldInstruction consts; formatScaffoldBody(format);
    withLocale(s, locale); buildFormatSystemPrompt(format, hasMultiline, subjectTarget) per the snippets.
  - FOLLOW pattern: system.go (strings.Builder, no trailing newline, doc comments cite PRD §9.19/§17.8).
  - CONSUME: prompt.RenderGitmojiTable() (S2) for the gitmoji body.

Task 3: EDIT internal/prompt/system.go — extend BuildSystemPrompt + BuildFallbackPrompt
  - CHANGE signature: func BuildSystemPrompt(examples []string, hasMultiline bool, subjectTarget int, format, locale string) string
      - if format == "auto": existing §17.1 assembly (unchanged); else: buildFormatSystemPrompt(format, hasMultiline, subjectTarget)
      - RETURN withLocale(<assembled>, locale)
  - CHANGE signature: func BuildFallbackPrompt(subjectTarget int, format, locale string) string
      - if format == "auto": existing §17.2 assembly (unchanged); else: buildFormatSystemPrompt(format, false, subjectTarget)
      - RETURN withLocale(<assembled>, locale)
  - CRITICAL: the auto branches are BYTE-for-BYTE the current bodies; only the return is wrapped in withLocale
    (a no-op when locale==""). Do NOT alter the §17.1/§17.2 assembly.
  - UPDATE the doc-comment examples in system.go that show the old call signature (lines ~119).

Task 4: EDIT internal/prompt/planner.go — extend BuildPlannerSystemPrompt
  - CHANGE signature: func BuildPlannerSystemPrompt(examples []string, format, locale string) string
      - Write plannerSystemPrompt + "\n\n"
      - if format == "auto": the existing "---\n"+ex+"\n" examples loop (unchanged);
        else: WriteString(formatScaffoldBody(format))   // scaffold REPLACES examples (FR-F5 single-shortcut)
      - RETURN withLocale(b.String(), locale)
  - PRESERVE: plannerSystemPrompt const and BuildPlannerUserPayload verbatim (FR-F5 partitioning unchanged).

Task 5: EDIT the four call sites — thread cfg.Format, cfg.Locale
  - internal/generate/generate.go:314,321,327 → add cfg.Format, cfg.Locale to each Build* call.
  - internal/decompose/message.go:229,236,242 → add cfg.Format, cfg.Locale (cfg = deps.Config).
  - pkg/stagecoach/stagecoach.go:395,402,408 → add cfg.Format, cfg.Locale.
  - internal/decompose/planner.go:84 → BuildPlannerSystemPrompt(examples, deps.Config.Format, deps.Config.Locale).
  - NO other logic changes; NO new branching. grep-verify zero remaining old-arity calls.

Task 6: EDIT internal/prompt/system_test.go + planner_test.go — update signatures + add per-mode cases
  - Update every existing Build* call to the new arity with ("auto", "") so canonical-exact tests still prove
    FR-F1 byte-identity.
  - ADD table-driven cases (per §17.8) for conventional/gitmoji/plain × {locale "", "French"} on
    BuildSystemPrompt, BuildFallbackPrompt, and BuildPlannerSystemPrompt:
      * examples-block ABSENT (no "Match the tone and style", no antiReuseProhibition text, no "---" lines);
      * conventional: contains "type(scope): description" and the full type vocab list;
      * gitmoji: contains gitmojiScaffoldInstruction AND a RenderGitmojiTable() row (e.g. "🎨 - ");
      * plain: contains neither examples nor any format-contract text; still has the multiline rule + target;
      * multiline rule + subjectTargetLine retained in conventional/gitmoji/plain;
      * locale: "French" → ends with "\nWrite the commit message in French."; "" → no such line.
  - ADD one canonical-exact test per mode (locale set + unset) pinning the EXACT bytes (mirrors
    TestBuildSystemPrompt_CanonicalExact) so topology is locked.

Task 7: CREATE internal/prompt/format_test.go — unit-test the helpers
  - TestFormatScaffoldBody: auto/plain → ""; conventional → conventionalScaffold; gitmoji → instruction +
    "\n\n" + RenderGitmojiTable(); unknown → "".
  - TestWithLocale: ""→unchanged; "French" appends exactly one "\nWrite the commit message in French." with a
    single-newline separator (test both a string ending in "\n" and one without).

Task 8: CREATE the stub-agent integration test (internal/generate/*_test.go, follow generate_test.go:555)
  - Real temp repo with ≥2 commits (mature path). t.Setenv("STAGECOACH_STUB_STDINFILE", captureFile).
    stubtest.Build(t) + stubtest.Manifest(bin, Options{Out:"🎨 refactor auth"}). cfg with Format:"gitmoji",
    Locale:"French". Run CommitStaged; os.ReadFile the capture; assert it Contains the gitmoji instruction,
    Contains a table row, Contains "Write the commit message in French.", and does NOT Contain "Match the
    tone and style". (Because the stub uses stdin delivery with no system_prompt_flag, the rendered system
    prompt is prepended into the captured stdin — render.go:157.)

Task 9: EDIT docs/how-it-works.md — Mode-A prompt-construction doc
  - Under "## Prompt engineering", add a "### Format modes and locale" subsection: explain auto (default,
    learned style), conventional/gitmoji/plain (replace the learned-style examples with an explicit
    contract; history examples omitted), and --locale (appends "Write the commit message in <lang>." in every
    mode). Keep it brief and user-facing. Do NOT mention --context/--template (later subtasks).
```

### Implementation Patterns & Key Details

```go
// BuildSystemPrompt — auto path unchanged; non-auto dispatches; locale wraps the result.
func BuildSystemPrompt(examples []string, hasMultiline bool, subjectTarget int, format, locale string) string {
	if format != "auto" {
		return withLocale(buildFormatSystemPrompt(format, hasMultiline, subjectTarget), locale)
	}
	var b strings.Builder
	b.WriteString(maturePromptHeader) // == promptPreamble + "\n\n" + examplesIntro (byte-identical to today)
	b.WriteByte('\n')
	for _, ex := range examples {
		b.WriteString("---\n")
		b.WriteString(ex)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(antiReuseProhibition)
	b.WriteByte('\n')
	b.WriteByte('\n')
	if hasMultiline {
		b.WriteString(multilineRuleAllow)
	} else {
		b.WriteString(multilineRuleSingle)
	}
	b.WriteByte('\n')
	b.WriteString(subjectTargetLine(subjectTarget))
	return withLocale(b.String(), locale) // no-op when locale == "" → FR-F1 byte-identity preserved
}

// BuildFallbackPrompt — auto path unchanged; non-auto uses the scaffold with hasMultiline=false (new repo).
func BuildFallbackPrompt(subjectTarget int, format, locale string) string {
	if format != "auto" {
		return withLocale(buildFormatSystemPrompt(format, false, subjectTarget), locale)
	}
	s := fallbackPromptBody + "\n\n" +
		fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)
	return withLocale(s, locale)
}

// BuildPlannerSystemPrompt — contract unchanged; examples↔scaffold swap; locale wraps.
func BuildPlannerSystemPrompt(examples []string, format, locale string) string {
	var b strings.Builder
	b.WriteString(plannerSystemPrompt)
	b.WriteString("\n\n")
	if format == "auto" {
		for _, ex := range examples {
			b.WriteString("---\n")
			b.WriteString(ex)
			b.WriteByte('\n')
		}
	} else {
		b.WriteString(formatScaffoldBody(format)) // "" for plain → contract + (locale) only
	}
	return withLocale(b.String(), locale)
}

// CALL SITE shape (all three message copies — generate.go / decompose/message.go / pkg/stagecoach):
//   return prompt.BuildFallbackPrompt(cfg.SubjectTargetChars, cfg.Format, cfg.Locale), nil
//   return prompt.BuildSystemPrompt(msgs, prompt.DetectMultiline(msgs), cfg.SubjectTargetChars, cfg.Format, cfg.Locale), nil
```

### Integration Points

```yaml
CONFIG (READ-ONLY — S1 landed):
  - consume: config.Config.Format (auto|conventional|gitmoji|plain), config.Config.Locale (free-form).
  - NO new flag/env/git-config/TOML keys; NO validation (S1 owns it).

PROMPT PACKAGE (S2 landed):
  - consume: prompt.RenderGitmojiTable() for the gitmoji scaffold body.

CALL SITES (thread two args, no logic change):
  - internal/generate/generate.go, internal/decompose/message.go, pkg/stagecoach/stagecoach.go (message role),
    internal/decompose/planner.go (planner FR-M11). Arbiter N+1 inherits via decompose message path.

DOCS (Mode A):
  - docs/how-it-works.md → new "### Format modes and locale" under "## Prompt engineering".

OUT OF SCOPE (do NOT touch):
  - payload.go / BuildUserPayload (--context is S2.T2.S1).
  - post-generation message finalization / --template (S2.T2.S2).
  - arbiter.go BuildArbiterSystemPrompt (decision prompt, no message).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/prompt/format.go internal/prompt/format_test.go internal/prompt/system.go internal/prompt/planner.go
go build ./...                         # all four call sites MUST compile — proves no caller was missed
go vet ./...
golangci-lint run                      # .golangci.yml: errcheck,gosimple,govet,ineffassign,staticcheck,unused

# Expected: zero errors. If `go build` fails on a Build* arity mismatch, a caller was missed — re-grep.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/prompt/... -v       # canonical-exact (auto) unchanged; per-mode + locale + helper tests

# Must include and pass:
#  - TestBuildSystemPrompt_CanonicalExact / TestBuildFallbackPrompt_CanonicalExact (auto, "") — BYTE-IDENTICAL (FR-F1).
#  - Per-mode: conventional/gitmoji/plain replace examples (no "Match the tone", no anti-reuse, no "---").
#  - gitmoji contains RenderGitmojiTable() rows; conventional contains the type vocab; plain has neither.
#  - Multi-line rule + subjectTargetLine retained in every non-auto mode.
#  - Locale: "French" → trailing "\nWrite the commit message in French." in every mode + both repo-age paths
#    + the planner builder; "" → no locale line.
#  - TestFormatScaffoldBody / TestWithLocale (format_test.go).

go test ./internal/generate/... ./internal/decompose/... ./pkg/stagecoach/... -v
# Expected: existing suites green (signature threading only); stub-agent format test green.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary and drive each mode against a real stub provider (or eyeball with --dry-run --verbose).
go build -o /tmp/stagecoach ./cmd/stagecoach

# The stub-agent format test is the authoritative integration check (Task 8): it renders the REAL system
# prompt through provider.Render and asserts the gitmoji scaffold + locale line reach the agent's stdin.
go test ./internal/generate/... -run 'Format|Gitmoji|Locale' -v

# Optional manual smoke (needs a real agent on PATH; otherwise rely on the stub test):
#   stagecoach --format gitmoji --dry-run --verbose      # verbose prints the resolved command; message is 🎨-prefixed
#   stagecoach --format conventional --locale French --dry-run
```

### Level 4: Regression & Cross-cutting

```bash
go test -race ./...                    # full suite (make test)
make coverage-gate                     # ≥85% on internal/{git,provider,generate,config} (unaffected packages stay green)
golangci-lint run ./...

# Byte-identity guard: with cfg.Format="auto" & cfg.Locale="", the generated system prompt must equal the
# pre-change output. The retained *_CanonicalExact tests enforce this — they MUST pass without their want
# strings being edited (only the call args gain "auto", "").
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` compiles (all four Build* call sites updated — no arity mismatch).
- [ ] `go test ./...` green; existing `*_CanonicalExact` tests pass with only call-arg edits (FR-F1 proof).
- [ ] `go vet ./...` + `golangci-lint run` clean; `gofmt` no diff (emoji + em-dash runes preserved).
- [ ] `make coverage-gate` ≥85% on the four core packages.

### Feature Validation
- [ ] auto + empty locale → byte-identical to today (mature, new-repo, and planner).
- [ ] conventional/gitmoji/plain replace the examples block (no "Match the tone…", no anti-reuse, no "---").
- [ ] gitmoji embeds RenderGitmojiTable(); conventional has the type vocab; plain has no format contract.
- [ ] Multi-line rule + subject-target retained in all non-auto message modes.
- [ ] Locale line appended in every mode + both repo-age variants + the planner; absent when empty.
- [ ] Applied at all four sites; arbiter N+1 inherits; arbiter decision prompt unchanged.
- [ ] Stub-agent test proves the rendered system prompt reflects the mode + locale.

### Code Quality Validation
- [ ] Matches internal/prompt conventions: constants no trailing newline; assembler owns "\n" placement;
      doc comments cite PRD §9.19 / §17.8; pure + defensive (unknown format → auto-like, no panic).
- [ ] Format logic lives in the prompt package (format.go); call-site diffs are two-arg threads only.
- [ ] No config/flag/env changes; no payload.go/arbiter.go/template changes.

### Documentation & Deployment
- [ ] docs/how-it-works.md has a concise, accurate "Format modes and locale" subsection (Mode A).
- [ ] No new env vars / flags / config keys introduced by this subtask.

---

## Anti-Patterns to Avoid

- ❌ Don't rewrite the §17.1/§17.2/§17.5 auto assembly — wrap it; auto + empty locale MUST stay byte-identical.
- ❌ Don't duplicate the promptPreamble text — split maturePromptHeader with compile-time constant concat.
- ❌ Don't add repo-age branching at the call sites — both BuildSystemPrompt and BuildFallbackPrompt dispatch
  to the scaffold, so callers only thread cfg.Format/cfg.Locale.
- ❌ Don't miss pkg/stagecoach/stagecoach.go — it is a third verbatim copy of the message-prompt helper.
- ❌ Don't validate/normalize the locale — pass it verbatim (FR-F6); only the empty check gates the line.
- ❌ Don't edit plannerSystemPrompt or BuildArbiterSystemPrompt — the partitioning + decision prompts are
  unchanged (FR-F5).
- ❌ Don't re-fetch/re-embed the gitmoji table — call prompt.RenderGitmojiTable() (S2). No network fetch (FR-F3).
- ❌ Don't build --context or --template here — those are S2.T2.S1 / S2.T2.S2.
- ❌ Don't hardcode "50" in scaffolds — the subject-target line is config-driven (subjectTargetLine).

---

## Confidence Score

**9/10** for one-pass implementation success. The seam is well-understood: S1 (config) and S2 (gitmoji
table) are landed and their exact identifiers are pinned; the four call sites are enumerated with line
numbers (including the easy-to-miss pkg/stagecoach copy); the byte-identity trick (compile-time constant
concat + locale gated on empty) makes FR-F1 mechanically enforceable by the existing canonical tests; the
§17.8 scaffold text is quoted verbatim; and the stub-agent assertion has a concrete, existing pattern to
copy (generate_test.go:555, STAGECOACH_STUB_STDINFILE). The −1 is a genuine spec ambiguity flagged in the
gotchas: whether `plain` retains the multi-line rule (§17.8's intro says the multi-line rule is retained for
all non-auto modes; FR-F4's "output rules + essence + subject-length target only" reads narrower). This PRP
resolves it in favor of the intro (retain the multi-line rule + FR12 detection in every non-auto mode, per
FR-F2's "detection still runs") and pins it with a canonical test; if review prefers the narrower reading,
it is a one-line change in buildFormatSystemPrompt (skip the multi-line rule when format=="plain").
