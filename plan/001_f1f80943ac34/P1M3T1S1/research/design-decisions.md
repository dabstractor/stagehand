# Design Decisions — P1.M3.T1.S1 (mature-repo system prompt)

> The authoritative read alongside `commit-pi-origin.md`. Each § is a non-obvious call an implementer
> could get wrong; each cites its evidence. Numbered for cross-reference from the PRP's gotchas/tests.

## §0 — File placement & scope (what THIS subtask owns)

- **CREATE** `internal/prompt/system.go` (`package prompt`) + `internal/prompt/system_test.go`
  (`package prompt`). The `internal/prompt/` directory EXISTS but is empty (confirmed).
- **OWNS:** the mature-repo system prompt assembly (PRD §17.1) + multi-line detection (§9.3 FR12). Two
  exported functions: `BuildSystemPrompt` (FR13) and `DetectMultiline` (FR12), plus unexported
  canonical string constants.
- **DOES NOT OWN:** the new-repo (≤1 commit) conventional-commit prompt (§17.2 → P1.M3.T1.S2) or the
  user payload (§17.3 → P1.M3.T1.S3). S2 and S3 will add to the SAME `internal/prompt` package later.
  Keep this file self-contained (define the mature-repo constants here; do not pre-empt S2/S3 layouts).
- The arch `system_context.md` package layout sketches `internal/prompt/` → `system.go, examples.go,
  payload.go`. `system.go` (this subtask) holds the mature-repo prompt + `DetectMultiline`; `payload.go`
  is S3. `examples.go` is not needed — examples are assembled inside `BuildSystemPrompt`.

## §1 — `BuildSystemPrompt` signature: EXPORTED, returns `string`, NO error

Implement EXACTLY the work-item signature:
```go
func BuildSystemPrompt(examples []string, hasMultiline bool, subjectTarget int) string
```
- **Exported** (capital B): the caller is `internal/generate` (P1.M3.T4 `CommitStaged`), a different
  package. Same reasoning as `provider.ParseOutput` (P1.M2.T6.S1 design-decisions §0).
- **Returns `string` only (no `error`):** there is NO failure mode. `hasMultiline` is a bool that
  selects one of two constant rules; `subjectTarget` is an int formatted into one line; `examples` is a
  slice whose zero value simply emits an empty examples block (defensive — see §9). The orchestrator
  gates on `CommitCount > 1` before calling, so examples are non-empty in practice. (Mirrors how
  `Manifest.Render` returns `(*CmdSpec, error)` because Validate can fail; BuildSystemPrompt validates
  nothing, so no error.)

## §2 — `DetectMultiline` is a SEPARATE exported helper (FR12 ≠ FR13)

Add a second exported function:
```go
func DetectMultiline(examples []string) bool
```
- **Why separate:** §9.3 splits this into FR12 ("Detect whether the history contains multi-line
  commits … by scanning the examples") and FR13 ("Construct the system prompt with … multi-line rule
  conditioned on FR12"). `BuildSystemPrompt` TAKES `hasMultiline` as a parameter (the work-item
  signature is binding), so the detection is a distinct concern the caller performs.
- **The orchestrator (P1.M3.T4) wires them:**
  ```go
  recent, _ := git.RecentMessages(ctx, 20)              // P1.M1.T3.S3 → []string, newest-first, trimmed
  hasMulti := prompt.DetectMultiline(recent)            // FR12
  sys := prompt.BuildSystemPrompt(recent, hasMulti, cfg.SubjectTargetChars)  // FR13
  ```
  Separating them also makes `BuildSystemPrompt` trivially unit-testable (pass any bool).
- **Faithful port of commit-pi's awk** (see commit-pi-origin.md §2): `true` ⇔ ANY example has >1
  NON-BLANK line. Implement via `countNonEmptyLines(msg) > 1` (NOT `strings.Contains(msg, "\n")` — the
  awk strips blanks then counts; the exact port removes all doubt about whitespace-only edge lines).

## §3 — Canonical constants: verbatim from PRD §17.1, NOT commit-pi

PRD §17.1 is AUTHORITATIVE on conflict (it is the "refined" version). commit-pi shipped the JSON
contract; PRD §17.4 replaced it with the raw-output contract. So:
- **DO** use PRD §17.1's raw-output contract header.
- **DO NOT** use commit-pi's `$json_instruction` ("Return valid JSON only…", "no double quotes").
- Define each piece as an UNEXPORTED Go string const (`maturePromptHeader`, `antiReuseProhibition`,
  `multilineRuleAllow`, `multilineRuleSingle`). `BuildSystemPrompt` is the ONLY exported surface;
  constants stay unexported (nothing outside the package needs them). Appendix A says "committed verbatim
  to internal/prompt/system.go as Go string constants" — constants, yes; exported, no need.

## §4 — Examples block: `---\n` BEFORE each message; EXCLUDE the `(up to 20…)` annotation

- Format (confirmed by commit-pi `git log --format="---%n%B"` AND PRD §17.1): one `---` line precedes
  EACH example. So `for _, ex := range examples { b.WriteString("---\n"); b.WriteString(ex);
  b.WriteByte('\n') }`.
- `RecentMessages` returns TRIMMED messages (no trailing newline), so appending `\n` after each is
  required so the next `---` starts on its own line.
- **EXCLUDE** the `(up to 20, ≤100 lines total)` line — it is a structural annotation, not literal text
  (commit-pi never emitted it; the caps are enforced upstream by `RecentMessages` n=20 / 100-line cap).
  See commit-pi-origin.md §4.

## §5 — The em-dash (U+2014) — use it VERBATIM, do not substitute a hyphen

PRD §17.1 anti-reuse: `They show the STYLE to match — format, tone, length, conventions.` uses an
**em-dash** `—` (U+2014, UTF-8 `0xE2 0x80 0x94`). commit-pi used an ASCII hyphen `-`. **Use the PRD's
em-dash.** Go source is UTF-8, so the constant holds the 3 em-dash bytes verbatim. Do NOT type `-`.
This is the ONLY non-ASCII byte in the entire prompt — a test asserts its presence (guards against an
accidental hyphen that looks identical in many editors).

## §6 — `subjectTarget` wiring: `fmt.Sprintf("Target ~%d characters for the subject line.", n)`

- The literal `~` (PRD: "Target ~50 characters") is preserved in the format string.
- The caller passes `cfg.SubjectTargetChars` (P1.M1.T4.S1 `Config.SubjectTargetChars int`, default 50,
  TOML `subject_target_chars`). `BuildSystemPrompt` is decoupled from `config` (pure function — §8), so
  it only formats the int. The orchestrator does the wiring.
- Do NOT hardcode `50` inside BuildSystemPrompt — the parameter exists precisely so config/tests can
  drive it. (A test passes e.g. 72 and asserts `Target ~72 characters…`.)

## §7 — Imports: `fmt` + `strings` ONLY; no config/git/provider; go.mod UNCHANGED

- `system.go` imports EXACTLY `fmt` (the `Sprintf`) and `strings` (the `strings.Builder` + `Split` +
  `TrimSpace` in `countNonEmptyLines`). NO third-party, NO `internal/config`, NO `internal/git`,
  NO `internal/provider`.
- It consumes `[]string` (from `RecentMessages`) and `int` (from `config`) — both plain types, no
  package coupling. This keeps `internal/prompt` a leaf in the import graph (config/git/provider never
  import prompt; prompt imports nothing internal).
- **`go mod tidy` is a no-op.** `git diff --exit-code go.mod go.sum` MUST be empty (mirrors
  P1.M2.T6.S1's stdlib-only constraint).

## §8 — No dependency cycle risk (leaf package)

`internal/prompt` must NOT import `internal/config` or `internal/git`:
- `config` carries `Provider map[string]map[string]any` and is consumed by `provider`; if `prompt`
  imported `config` it would still be acyclic, BUT it needlessly couples a pure string-builder to the
  config machinery. Keep `BuildSystemPrompt` taking plain `( []string, bool, int )` — the orchestrator
  reads `cfg.SubjectTargetChars` and passes the int. This is the same decoupling `provider.Render`
  uses (it takes plain strings, not a `*Config`).
- Concretely: `prompt` ← (fmt, strings). `generate` ← (prompt, git, provider, config). One-way.

## §9 — Empty/nil examples: defensive, NEVER panic

- `DetectMultiline(nil)` → `false` (loop over zero examples). `DetectMultiline([]string{})` → `false`.
- `BuildSystemPrompt(nil, false, 50)` → emits the header, an EMPTY examples block (no `---` lines), the
  anti-reuse block, the single-line rule, and the target. No panic, no index-out-of-range.
- The orchestrator gates on `CommitCount > 1`, so empty examples never happen in production (a repo with
  >1 commit always yields ≥1 message from `RecentMessages`). The defensive handling is purely for
  unit-test ergonomics and robustness — a `nil` slice must not crash `strings.Builder`.

## §10 — Multi-line rule selection: a plain if/else, two verbatim constants

```go
rule := multilineRuleSingle                       // "Only output a single-line subject (no body)."
if hasMultiline {
    rule = multilineRuleAllow                     // "Only add a body (blank line + description) if the history shows multi-line commits AND these changes truly warrant detailed explanation. Otherwise, use a single-line subject only."
}
```
- Both strings are verbatim from PRD §17.1 (note PRD hyphenates "multi-line"; preserve it).
- The selection is the ONLY branch in `BuildSystemPrompt` — everything else is linear assembly.

## §11 — Assembly topology (the exact newline placement)

`BuildSystemPrompt` builds with a `strings.Builder` in this order (derived in commit-pi-origin.md §5;
matches PRD §17.1's blank-line topology EXACTLY, minus the excluded annotation):

```
maturePromptHeader        // "...from this repository:"  (NO trailing newline)
'\n'
for each ex: "---\n" + ex + '\n'
'\n'                      // blank line between last example and CRITICAL
antiReuseProhibition      // "...is a critical failure."  (NO trailing newline)
'\n' '\n'                 // blank line between anti-reuse and the rule
rule                      // the selected multi-line rule  (NO trailing newline)
'\n'
"Target ~%d characters for the subject line."   (subjectTarget)
```

- Constants are defined WITHOUT trailing newlines (cleaner); `BuildSystemPrompt` owns ALL inter-block
  newline placement, so the topology is in exactly one place and is trivially auditable.
- One subtlety: between the anti-reuse block and the rule there is ONE blank line → two `\n` bytes
  (the block's terminating `\n` would be redundant if the constant ended in `\n`; since it does not,
  emit `'\n','\n'`). Between the rule and the Target line there is NO blank line → one `\n`.

## §12 — Test strategy (table-driven, in-package, no I/O)

- `internal/prompt/system_test.go`, `package prompt` (white-box — matches every `_test.go` in this repo:
  `git/recentmessages_test.go`, `provider/manifest_test.go` are all in-package).
- **`TestBuildSystemPrompt`** — a `[]struct{ name; examples; hasMultiline; subjectTarget; want string }`
  table (or assert on SUBSTRINGS / structural properties where the full string is unwieldy). Rows:
  - single-line examples, hasMultiline=false → single-line rule present, allow-rule ABSENT.
  - multi-line examples, hasMultiline=true → allow-rule present, single-line rule ABSENT.
  - subjectTarget=72 → `Target ~72 characters…`.
  - the em-dash is present in the anti-reuse block (guards §5).
  - the raw-output contract (`Output ONLY the commit message`) is present; the JSON contract
    (`Return valid JSON`) is ABSENT (guards §3 — we ported the PRD, not commit-pi).
  - `---` count == len(examples) (one before each).
  - examples appear in order; the `(up to 20, ≤100 lines total)` annotation is ABSENT (guards §4).
  - empty examples → no panic, no `---`, header + anti-reuse + rule + target all present.
- **`TestDetectMultiline`** — `[]struct{ examples; want }`:
  - nil / empty → false.
  - all single-line → false.
  - one multi-line (`subject\n\nbody`) → true.
  - whitespace-only body lines (`subject\n   \nbody`) → true (non-blank count > 1 — the awk-faithful
    behavior; documents the exact-port choice over `strings.Contains`).
  - a message that is `subject\n\n` (subject + trailing blanks) → after RecentMessages trim it is
    `subject` → false; but DetectMultiline takes the trimmed slice, so feed `[]string{"subject"}` → false.
- **`TestCountNonBlankLines`** (optional, targets the helper): `""`→0, `"one"`→1, `"a\n\nb"`→2,
  `"a\n   \nb"`→2, `"\n\n"`→0.
- No subprocess, no temp repo, no `git` — `BuildSystemPrompt`/`DetectMultiline` are pure functions.
  (The git-side integration — feeding real `RecentMessages` output — is the orchestrator's job, P1.M3.T4.)
