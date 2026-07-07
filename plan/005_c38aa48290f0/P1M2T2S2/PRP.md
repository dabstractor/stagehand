name: "P1.M2.T2.S2 — --template with mandatory $msg + shared message-finalization seam in all three commit paths"
description: |
  Add the full-precedence `--template '<tpl>'` config surface (`[generation].template`,
  `STAGECOACH_TEMPLATE`, `stagecoach.template`, `--template`; FR-F8), HARD-error when the resolved value
  lacks the literal `$msg`, and implement `ApplyTemplate(msg, tpl)` behind a NAMED shared seam
  `generate.FinalizeMessage(msg, cfg)` invoked AFTER parse/cleanup and BEFORE the duplicate check in
  every commit path — the three generate/dedupe loops (generate.go, pkg/stagecoach runPipeline,
  decompose message.go) plus the FR-M11 planner shortcut. So dedupe (§9.7) judges the templated subject,
  and every commit in a decompose run is templated uniformly. The seam is the explicit ordering slot
  where P1.M5.T1.S1 later inserts the `--edit` editor gate (editor slots AFTER template, per FR-E3).
  Empty template ⇒ byte-identical to today (the default).

---

## Goal

**Feature Goal**: `stagecoach --template '$msg (#205)'` wraps EVERY commit message a run produces (single
commit, every decompose commit, the FR-M11 shortcut, the arbiter N+1) by substituting the literal `$msg`
with the full generated message — applied after parse/cleanup and before the duplicate check, so §9.7
judges the final subject as it will land. A resolved template that lacks `$msg` is a hard configuration
error at load. The template is resolved through the standard 5-layer precedence
(`[generation].template` < `stagecoach.template` < `STAGECOACH_TEMPLATE` < `--template`), exactly like
`format`/`locale`. All of this funnels through ONE named seam — `generate.FinalizeMessage` — that
P1.M5.T1.S1's `--edit` gate will extend.

**Deliverable**:
1. A new `internal/generate/finalize.go` with `ApplyTemplate(msg, tpl string) string` (pure substitution)
   and `FinalizeMessage(msg string, cfg config.Config) string` (the named seam; today = ApplyTemplate).
2. `Config.Template` field + full-precedence plumbing across config.go / file.go / git.go / load.go /
   root.go (mirroring `format`/`locale`).
3. `validateTemplate` (hard `$msg` error) called once at the Load() tail.
4. The seam invoked at 4 explicit sites (generate.go loop, stagecoach.go runPipeline loop,
   decompose/message.go loop, decompose/decompose.go `runSingleShortcut`); arbiter/one-file/escape
   covered transitively.
5. Unit + integration tests (ApplyTemplate/FinalizeMessage pure tests; validateTemplate table;
   dedupe-sees-templated-subject; uniform application across a decompose run; `config init --template`
   collision regression).
6. Docs: `docs/cli.md` `--template` row (incl. the `$msg` contract + the hard error) and env/git table;
   `docs/configuration.md` `template` key across the file-format / defaults / env / git-config tables.

**Success Definition**:
- `template == ""` (default) → every commit path emits BYTE-IDENTICAL messages to today (all existing
  generate/decompose/stagecoach tests pass unchanged — the load-bearing regression guard).
- `--template '$msg (#205)'` → a generated `Fix the parser` lands as `Fix the parser (#205)`, and the
  dedupe check compares `Fix the parser (#205)` against recent subjects.
- A resolved template `(#205)` (no `$msg`) → `stagecoach` exits non-zero at config load with
  `template: invalid template "(#205)": must contain the literal $msg (e.g. "$msg (#205)")`.
- On a decompose run, ALL commits (including the FR-M11 shortcut message and the arbiter N+1) carry the
  template.
- `stagecoach --template x` sets `cfg.Template`; `stagecoach config init --template` still parses as the
  inert-reference-config bool (no collision).
- `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run` all green.

## User Persona

**Target User**: The "plan-holder" (PRD §7.1) working an issue tracker who wants every commit in a session
tagged with a ticket/PR reference (`$msg (#205)`) or a fixed prefix, without hand-editing each message.

**Use Case**: `stagecoach --template '$msg (#812)'` on a multi-commit decompose run → every commit gets the
`(#812)` suffix; or `stagecoach.template = "[skip ci] $msg"` in repo git config for a docs repo.

**User Journey**: run `stagecoach --template '$msg (#205)'` → `config.Load` resolves + validates
`cfg.Template` → each commit path generates a bare message, `FinalizeMessage` substitutes `$msg` → the
dedupe check sees the final subject → the templated message lands via the normal plumbing path.

**Pain Points Addressed**: incumbents (aicommits/opencommit) offer message templates; parity requires it.
The wrinkle unique to stagecoach: it must apply UNIFORMLY across a decomposed multi-commit run and be
visible to the dedupe check — both handled by putting it at the single pre-dedupe seam.

## Why

- **FR-F8 (PRD §9.19)**: *"`--template '<tpl>'` / `STAGECOACH_TEMPLATE` / `stagecoach.template` /
  `[generation].template`, default empty. `<tpl>` must contain the literal `$msg` (hard error otherwise),
  which is replaced with the full generated message after parsing/cleanup and before the duplicate check
  (§9.7 must judge the final subject as it will land). Applies to every commit message in a run (all
  decompose commits included)."* — this subtask IS that feature.
- **§17.8 (Template)**: *"Template (FR-F8) is not a prompt feature. It is a post-generation string
  substitution (parse → cleanup → template → duplicate check); the model never sees it."* → the seam is
  post-generation, NOT in the prompt package's prompt-building path.
- **FR-E3 (PRD §9.22)** + the item's OUTPUT clause: the seam is the explicit ordering contract for
  P1.M5.T1.S1 — *"the template (FR-F8) is applied before the editor opens, so the user edits the final
  text."* The named `FinalizeMessage` seam is where `--edit` slots in, AFTER template.
- **Scope fences**: this is the message-FINALIZATION seam (post-generation), disjoint from S2.T2.S1's
  `--context` (user-payload PRE-generation) and S3's `--format`/`--locale` (system-prompt PRE-generation).
  See `research/design-decisions.md`.

## What

Add `--template` (full precedence), validate `$msg`, and route every commit path's parsed message through
one named finalization seam that substitutes `$msg` before the dedupe check.

### Success Criteria

- [ ] `internal/generate/finalize.go`: `ApplyTemplate(msg, tpl)` (empty tpl ⇒ identity;
      `strings.ReplaceAll(tpl, "$msg", msg)`) + `FinalizeMessage(msg, cfg)` seam with the FR-E3 ordering
      docstring.
- [ ] `Config.Template string \`toml:"template"\`` + `Defaults()` `Template: ""`; plumbed across file/git/env/flag
      (mirroring `format`/`locale`) with `validateTemplate` at the Load() tail.
- [ ] Seam invoked after `parseFail = false` / before `ExtractSubject` in generate.go, stagecoach.go
      runPipeline, decompose/message.go; and on `plannerMsg` before the dup-check in `runSingleShortcut`.
- [ ] Dedupe compares the TEMPLATED subject; a templated subject matching a recent subject triggers a retry.
- [ ] Every commit in a decompose run is templated (shortcut, per-concept, arbiter N+1) — verified by test.
- [ ] Resolved template without `$msg` → hard error at load; `""` → byte-identical to today.
- [ ] `config init --template` (bool) unaffected by the new global `--template` (string).
- [ ] `docs/cli.md` + `docs/configuration.md` rows added.
- [ ] Full build/test/vet/lint green.

## All Needed Context

### Context Completeness Check

_This PRP names the new file + the two function signatures, the exact 4 insertion points with line numbers
and the anchor lines, the six config-plumbing spots with the `format`/`locale` precedent lines, the
validation call site, the cobra flag-collision analysis, the transitive-coverage proof for
arbiter/one-file/escape, the exact test files to mirror, and every docs row. An implementer with no prior
codebase knowledge can complete it from this document + codebase access._

### Documentation & References

```yaml
- docfile: plan/005_c38aa48290f0/prd_snapshot.md
  why: Authoritative spec. §9.19 FR-F8 (the feature + the $msg hard error + "before the duplicate check" +
       "every commit message in a run"), §17.8 "Template" (post-generation, not a prompt feature), §9.7
       (duplicate rejection — what the templated subject is judged against), §9.22 FR-E1/E3 (the --edit gate
       that slots AFTER template — the ordering contract this seam publishes).
  section: "§9.19 FR-F8" + "§17.8 (Template paragraph)" + "§9.7" + "§9.22 FR-E1/E3"
  critical: |
    FR-F8: default empty; MUST contain literal `$msg` (hard error otherwise); replaced with the FULL
    generated message AFTER parse/cleanup and BEFORE the duplicate check; applies to EVERY commit in a run.
    §17.8: "the model never sees it" — post-generation substitution, NOT a prompt edit.
    FR-E3: "the template is applied BEFORE the editor opens" — editor (P1.M5.T1.S1) slots AFTER this seam.

- docfile: plan/005_c38aa48290f0/P1M2T2S2/research/design-decisions.md
  why: The load-bearing calls — the 4 insertion points (with line numbers), the transitive-coverage proof
       for arbiter/one-file/escape (why only 4 edits achieve "every message"), the seam placement in
       `generate` (import graph), $msg semantics (ReplaceAll, full message, multi-line), the cobra
       `--template` collision analysis, and the byte-identity regression guard.
  section: "all (short)"
  critical: "§2 (transitive coverage) + §3 (seam design/ordering contract) + §5 (config-init collision)."

- file: internal/generate/generate.go
  why: CALL SITE #1 (single-commit loop) + the pattern the seam file lives beside. The accept sequence is
       L237 `parseFail = false` → L238 `signal.SetCandidate(m)` → L240 `subject := ExtractSubject(m)` →
       L241 `IsDuplicate` → L248 `msg = m`. buildSystemPrompt already threads cfg.Format/cfg.Locale (S3) —
       cfg is available. Insert `m = FinalizeMessage(m, cfg)` right after L237, before L238.
  pattern: |
    parseFail = false
    m = FinalizeMessage(m, cfg) // §9.19 FR-F8 seam — template BEFORE dedupe (§9.7 judges the final subject)
    signal.SetCandidate(m)
    subject := ExtractSubject(m)
  gotcha: "Same package — call FinalizeMessage unqualified. SetCandidate / rejected / candidate / msg MUST all
           observe the templated m ⇒ apply the seam BEFORE SetCandidate, at this single point."

- file: pkg/stagecoach/stagecoach.go
  why: CALL SITE #2 (runPipeline — the public-API copy of the loop, L480-532). Accept sequence: L518
       `parseFail = false` → L519 `signal.SetCandidate(m)` → L521 `subject := generate.ExtractSubject(m)`.
       cfg is `cfg` (config.Config). This package imports `generate`.
  pattern: "insert `m = generate.FinalizeMessage(m, cfg)` after L518 `parseFail = false`, before L519 SetCandidate."
  gotcha: "Easy to miss — a THIRD copy of the loop. Qualify as generate.FinalizeMessage."

- file: internal/decompose/message.go
  why: CALL SITE #3 (decompose message role, `generateMessage`, loop L118-174). Accept sequence: L161
       `parseFail = false` → L163 `subject := generate.ExtractSubject(m)`. cfg is `deps.Config`. This is
       ALSO the func the arbiter N+1 and one-file/FR-M11-fallback reuse ⇒ templating here covers them.
  pattern: |
    parseFail = false
    m = generate.FinalizeMessage(m, deps.Config) // §9.19 FR-F8 seam — before dedupe
    subject := generate.ExtractSubject(m)
  gotcha: "No signal.SetCandidate here (decompose loop is signal-free). Insert between L161 and L163."

- file: internal/decompose/decompose.go
  why: CALL SITE #4 (FR-M11 `runSingleShortcut`, L316-352). The planner's message is used DIRECTLY (no
       generateMessage loop on the happy path) ⇒ it must be templated explicitly BEFORE the dup-check.
       Current L322-329:
         msg := plannerMsg
         if dupCheckMessage(ctx, deps, plannerMsg, isUnborn) { msg, err = generateMessage(...) ... }
  pattern: |
    msg := generate.FinalizeMessage(plannerMsg, deps.Config) // §9.19 FR-F8 — template the planner's message
    if dupCheckMessage(ctx, deps, msg, isUnborn) {           // dup-check the TEMPLATED subject
        msg, err = generateMessage(ctx, deps, baseTree, tStart) // fallback already templates internally
        ...
    }
  gotcha: "Pass the TEMPLATED `msg` (not `plannerMsg`) to dupCheckMessage so §9.7 sees the final subject.
           The generateMessage fallback templates internally (call site #3) — do NOT double-template it."

- file: internal/decompose/chain.go
  why: PROOF of transitive coverage (NO EDITS). Arbiter null → resolveNewCommit (L109) calls
       generateMessage → templated via call site #3. Tip-amend/mid-chain REUSE existing (already-templated)
       in-run commit messages. Read to confirm; do not edit.
  gotcha: "Do NOT add ApplyTemplate here — it would double-template the N+1 (generateMessage already did)."

- file: internal/config/config.go
  why: Add the Config.Template field + Defaults. Format(L85)/Locale(L89) are the precedent; add Template
       right after Locale. Defaults() sets Format(L153)/Locale(L154) — add Template after L154.
  pattern: |
    // Template is the §9.19 FR-F8 message template. When non-empty it MUST contain the literal `$msg`
    // (validated at Load — hard error otherwise); the substituted message lands AFTER parse/cleanup and
    // BEFORE the duplicate check (§9.7). Standard 5-layer precedence (file→git→env→flag). Empty = no template.
    Template string `toml:"template"`
    // ...in Defaults():
    Template: "", // §9.19 FR-F8 default (empty = no template; validateTemplate accepts "")
  gotcha: "toml:\"template\" — Template IS a config-file key ([generation].template), unlike --context."

- file: internal/config/file.go
  why: [generation] file decode. fileGeneration has Format(L59)/Locale(L60) — add Template after L60.
       materialize copies them (~L239-243) and merge (~L370-374) — add the same non-empty-copy line in BOTH.
  pattern: |
    // struct (after L60): Template string `toml:"template"` // V2.1 — §9.19 FR-F8 message template (validated at Load)
    // materialize (mirror Format): if g.Template != "" { c.Template = g.Template }
    // merge (mirror Format):      if src.Template != "" { dst.Template = src.Template }
  gotcha: "Template is a SCALAR (non-empty REPLACE), like format/locale — NOT a union like Exclude."

- file: internal/config/git.go
  why: git-config layer. stagecoach.format(L130)/stagecoach.locale(L135) — add a stagecoach.template block
       right after L139, same shape (gitConfigGet → raw copy, no validation here).
  pattern: |
    if v, found, err := gitConfigGet(repoDir, "stagecoach.template"); err != nil {
        return nil, err
    } else if found {
        c.Template = v
    }

- file: internal/config/load.go
  why: env + flag layers + validation. loadEnv format/locale at L242-247 → add STAGECOACH_TEMPLATE.
       loadFlags format/locale at L336-345 → add --template. validateFormat call at L169 → add
       validateTemplate call right after. Add the validateTemplate func near validateFormat (L356).
  pattern: |
    // loadEnv (after L247):
    if v, ok := os.LookupEnv("STAGECOACH_TEMPLATE"); ok && v != "" { cfg.Template = v }
    // loadFlags (after L345):
    if fs.Changed("template") { if v, err := fs.GetString("template"); err == nil { cfg.Template = v } }
    // Load() tail (after the validateFormat block, L171):
    if err := validateTemplate(cfg.Template); err != nil { return nil, fmt.Errorf("template: %w", err) }
    // near validateFormat:
    func validateTemplate(tpl string) error {
        if tpl == "" || strings.Contains(tpl, "$msg") { return nil }
        return fmt.Errorf("invalid template %q: must contain the literal $msg (e.g. %q)", tpl, "$msg (#205)")
    }
  gotcha: "STAGECOACH_TEMPLATE uses the SAME presence-semantic (ok && v != \"\") as STAGECOACH_FORMAT. Validate
           ONCE on the fully-resolved value (not per-layer), exactly like validateFormat."

- file: internal/cmd/root.go
  why: register the GLOBAL --template flag. flagFormat/flagLocale vars at L74-75; their StringVar
       registrations follow the arbiter flags (near L150-155). Add flagTemplate + a StringVar.
  pattern: |
    // near L74-75: add `flagTemplate string` to the format/locale var block.
    // after the --locale StringVar:
    pf.StringVar(&flagTemplate, "template", "",
        "Wrap every commit message: the literal $msg is replaced with the generated message, e.g. "+
            "\"$msg (#205)\" (env STAGECOACH_TEMPLATE; git stagecoach.template; [generation].template; "+
            "default empty). Must contain $msg. (Distinct from `config init --template`.)")
  gotcha: |
    COBRA COLLISION (safe): `config init` has a LOCAL bool `--template` (cmd/config.go L144). The global
    persistent string --template is SKIPPED for `config init` by pflag's AddFlagSet (local name already
    present) — the bool wins there, the string wins everywhere else. Disambiguate in the help text (done
    above) and add the regression test. Do NOT rename either flag.

- file: internal/cmd/config.go
  why: READ ONLY — confirm `config init` registers `--template` as a LOCAL bool (L144:
       `configInitCmd.Flags().Bool("template", false, ...)`) so the collision reasoning holds. Do not edit.

- file: internal/prompt/format.go
  why: Reference for the S3 (parallel) message-shaping surface — proves --template is NOT a prompt feature
       (it is post-generation). Do NOT put ApplyTemplate here; the seam lives in `generate`.
  gotcha: "S3 owns format.go / system.go / planner.go SYSTEM prompts. This subtask touches NONE of them."

- file: internal/generate/dedupe.go
  why: ExtractSubject / IsDuplicate — the dedupe primitives the seam runs BEFORE. The finalize.go file
       lives beside this (same package), matching its pure-function + FROZEN-signature doc style.

- file: internal/generate/generate_test.go
  why: Test harness precedent — stub manifest (stubtest.Manifest) + git test double, retry/dedupe assertions.
       Mirror it for the "dedupe sees the templated subject" test.
  pattern: "Drive CommitStaged with a stub agent output + a recent-subjects double; assert Result.Message is templated."

- file: internal/config/load_test.go
  why: validateFormat test precedent — table-driven valid/invalid + the precedence-layer tests. Mirror for
       validateTemplate (valid: "", "$msg", "$msg (#205)"; invalid: "(#205)", "${msg}") and a template
       precedence test (file<git<env<flag).

- file: docs/cli.md
  why: Mode-A docs. Global-flags 6-col table: --format(L38)/--locale(L39) rows → add --template after L39.
       Env/git mini-table at L168-169 → add a --template row. NOTE the SEPARATE `config init --template`
       bool row at L106 — leave it; it is a different command (disambiguate, do not merge).
  critical: |
    Global-flags row:
    | `--template <tpl>` | string | "" | `STAGECOACH_TEMPLATE` | `stagecoach.template` | Wrap every commit message: `$msg` is replaced with the generated message, e.g. `"$msg (#205)"`. Must contain the literal `$msg` (else hard error, exit 1). Applies to every commit in a run. Also `[generation].template`. Distinct from `config init --template`. |
    Env/git row (L168-169 block):
    | `--template` | `STAGECOACH_TEMPLATE` | `stagecoach.template` |

- file: docs/configuration.md
  why: Mode-A docs. Mirror format/locale across: the file-format [generation] example (L104-105), the
       defaults table (L128-129), the env table (L162-163), and the git-config table (L185-186).
  critical: |
    File example (after L105): # template = ""       # wrap every message; must contain literal $msg, e.g. "$msg (#205)"
    Defaults table:   | `template` | `""` | `config.Defaults()` (§9.19 FR-F8) |
    Env table:        | `STAGECOACH_TEMPLATE` | `--template` | Message template; `$msg` = generated message; must contain `$msg` (hard error) | `STAGECOACH_TEMPLATE='$msg (#205)' stagecoach` |
    Git table:        | `stagecoach.template` | string | `git config --get stagecoach.template` | Message template; the literal `$msg` is replaced with the generated message. Must contain `$msg` (hard error, exit 1). |
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  finalize.go      # NEW — ApplyTemplate + FinalizeMessage (the seam), beside dedupe.go
  generate.go      # EDIT — CALL SITE #1 (L237-238)
  dedupe.go        # UNCHANGED (ExtractSubject/IsDuplicate — the primitives the seam precedes)
  finalize_test.go # NEW — ApplyTemplate/FinalizeMessage unit tests
internal/decompose/
  message.go       # EDIT — CALL SITE #3 (L161-163, generateMessage loop)
  decompose.go     # EDIT — CALL SITE #4 (runSingleShortcut L322-323)
  chain.go         # UNCHANGED (arbiter N+1 reuses generateMessage — transitively templated)
pkg/stagecoach/stagecoach.go # EDIT — CALL SITE #2 (runPipeline L518-519)
internal/config/
  config.go        # EDIT — Config.Template field + Defaults()
  file.go          # EDIT — fileGeneration.Template + materialize + merge
  git.go           # EDIT — stagecoach.template gitConfigGet
  load.go          # EDIT — STAGECOACH_TEMPLATE env, --template flag, validateTemplate + call
  load_test.go     # EDIT — validateTemplate table + template precedence test
internal/cmd/
  root.go          # EDIT — flagTemplate var + --template StringVar
  config.go        # UNCHANGED (READ — confirm config init --template is a local bool)
docs/cli.md            # EDIT — --template global-flags row + env/git row
docs/configuration.md  # EDIT — template across file/defaults/env/git tables
```

### Desired Codebase tree

One new file (`internal/generate/finalize.go`) + its test (`internal/generate/finalize_test.go`). All
else is edits. `ApplyTemplate`/`FinalizeMessage` live in `generate` (imported by decompose + pkg/stagecoach;
already imports config) so all four call sites reach them with no new import edges.

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (byte-identity): template=="" MUST reproduce today's bytes in EVERY path. ApplyTemplate
// short-circuits `if tpl == "" { return msg }`. All existing generate/decompose/stagecoach tests must pass
// UNCHANGED — that is the regression guard proving the seam is transparent when the feature is off.

// CRITICAL (ordering — FR-F8): apply the seam AFTER provider.ParseOutput succeeds (parseFail = false) and
// BEFORE ExtractSubject/IsDuplicate. The dedupe check (§9.7) MUST see the templated subject. Placing it
// after dedupe would let a templated subject collide with history undetected.

// CRITICAL (candidate/rescue): in generate.go and stagecoach.go the seam goes BEFORE signal.SetCandidate(m)
// so the rescue candidate note shows the message that would land. rejected/candidate/msg all use templated m.

// CRITICAL (transitive coverage): only 4 explicit edits. Arbiter N+1 (chain.go resolveNewCommit) and the
// one-file short-circuit both call generateMessage (call site #3) → templated. Escape hatch calls
// generate.CommitStaged (call site #1) → templated. Tip-amend/mid-chain reuse already-templated messages.
// Do NOT add ApplyTemplate in chain.go — that double-templates the N+1.

// CRITICAL ($msg semantics): strings.ReplaceAll(tpl, "$msg", msg) — ALL occurrences, FULL message
// (subject+body). "$msg"-only ⇒ identity. Multi-line + "$msg (#205)" ⇒ suffix after the body (spec:
// "the full generated message"); dedupe then extracts the unchanged first line. Do NOT special-case the
// subject — that violates FR-F8.

// CRITICAL (validation once): validateTemplate runs at the Load() tail on the FULLY RESOLVED value, exactly
// like validateFormat — a low-layer template overridden by a valid higher layer is NOT an error.

// COBRA COLLISION (safe, must test): root's persistent string --template and config init's local bool
// --template coexist. pflag AddFlagSet skips the inherited flag when the local name exists ⇒ config init
// keeps its bool, root keeps its string. No rename. Add a test that both parse as expected.

// SCOPE FENCE: this is POST-generation. Do NOT touch the prompt package (system.go/planner.go/format.go —
// S3) or the user-payload builders (payload.go — S2.T2.S1). §17.8: "the model never sees" the template.
```

## Implementation Blueprint

### Data models and structure

One new file, two pure functions, one new Config scalar (`Template string`).

```go
// internal/generate/finalize.go  (NEW)
package generate

import (
	"strings"

	"github.com/dustin/stagecoach/internal/config"
)

// ApplyTemplate applies the §9.19 FR-F8 message template: every literal "$msg" in tpl is replaced with the
// full generated message. Empty tpl ⇒ msg unchanged (the default; byte-identical to the pre-feature path).
// This is a POST-generation substitution (§17.8: "the model never sees it"), applied AFTER parse/cleanup
// and BEFORE the duplicate check so §9.7 judges the final subject as it will land. Substitution is literal
// and covers the FULL message (subject+body); "$msg" alone is the identity template.
func ApplyTemplate(msg, tpl string) string {
	if tpl == "" {
		return msg
	}
	return strings.ReplaceAll(tpl, "$msg", msg)
}

// FinalizeMessage is the shared message-finalization SEAM (§9.19 FR-F8): the single ordered pipeline every
// commit path funnels a parsed+cleaned message through to obtain the FINAL message as it will land. Today
// it is one stage — ApplyTemplate(msg, cfg.Template). It is invoked AFTER ParseOutput and BEFORE
// ExtractSubject/IsDuplicate in every generation loop, and on the planner's FR-M11 shortcut message before
// its dup-check, so the dedupe check (§9.7) always sees the templated subject.
//
// ORDERING CONTRACT (P1.M5.T1.S1): the --edit editor gate slots AFTER this seam (FR-E3: the template is
// applied before the editor opens). Extend the pipeline as template → (future) editor → publish; keep
// template first.
func FinalizeMessage(msg string, cfg config.Config) string {
	return ApplyTemplate(msg, cfg.Template)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/generate/finalize.go
  - IMPLEMENT: ApplyTemplate(msg, tpl string) string + FinalizeMessage(msg string, cfg config.Config) string
    (snippets above), with the FR-F8 / §17.8 / FR-E3 doc comments.
  - IMPORTS: strings + internal/config (already an import edge in the package).

Task 2: EDIT internal/config/config.go — add the Template field + default
  - ADD `Template string \`toml:"template"\`` after Locale (L89), doc: §9.19 FR-F8, 5-layer precedence,
    validated at Load, empty = no template.
  - ADD to Defaults(): `Template: "",` after L154 (Locale).

Task 3: EDIT internal/config/file.go — [generation].template decode + materialize + merge
  - ADD `Template string \`toml:"template"\`` to fileGeneration after L60.
  - materialize (mirror Format, ~L239): `if g.Template != "" { c.Template = g.Template }`.
  - merge (mirror Format, ~L370): `if src.Template != "" { dst.Template = src.Template }`.

Task 4: EDIT internal/config/git.go — stagecoach.template
  - ADD a gitConfigGet("stagecoach.template") block after L139 (same shape as stagecoach.locale).

Task 5: EDIT internal/config/load.go — env + flag + validation
  - loadEnv (after L247): `if v, ok := os.LookupEnv("STAGECOACH_TEMPLATE"); ok && v != "" { cfg.Template = v }`.
  - loadFlags (after L345): `if fs.Changed("template") { if v, err := fs.GetString("template"); err == nil { cfg.Template = v } }`.
  - ADD validateTemplate(tpl string) error near validateFormat (L356).
  - CALL it at the Load() tail after the validateFormat block (L171).

Task 6: EDIT internal/cmd/root.go — register --template (global persistent string)
  - ADD flagTemplate to the format/locale var block (L74-75).
  - ADD pf.StringVar(&flagTemplate, "template", "", "<help — $msg contract + distinct from config init --template>")
    after the --locale registration.

Task 7: EDIT the 4 seam sites — invoke FinalizeMessage after parse, before dedupe
  - internal/generate/generate.go: insert `m = FinalizeMessage(m, cfg)` after L237 (`parseFail = false`),
    before L238 (`signal.SetCandidate(m)`).
  - pkg/stagecoach/stagecoach.go: insert `m = generate.FinalizeMessage(m, cfg)` after L518, before L519.
  - internal/decompose/message.go: insert `m = generate.FinalizeMessage(m, deps.Config)` after L161, before L163.
  - internal/decompose/decompose.go (runSingleShortcut): change L322 to
    `msg := generate.FinalizeMessage(plannerMsg, deps.Config)` and pass `msg` (templated) to dupCheckMessage (L323).
  - Do NOT edit chain.go (arbiter reuses generateMessage/existing messages — transitively templated).

Task 8: CREATE internal/generate/finalize_test.go
  - ApplyTemplate: "" ⇒ identity; "$msg" ⇒ identity; "$msg (#205)" + "Fix parser" ⇒ "Fix parser (#205)";
    multiple "$msg"; multi-line message ("Sub\n\nBody") + "$msg (#205)" ⇒ suffix after Body, first line "Sub".
  - FinalizeMessage: cfg.Template="" ⇒ identity; cfg.Template set ⇒ applied.

Task 9: EDIT internal/config/load_test.go — validateTemplate + precedence
  - Table (mirror validateFormat): valid "", "$msg", "$msg (#205)", "[skip ci] $msg"; invalid "(#205)", "${msg}", "msg".
  - Template precedence: [generation].template < stagecoach.template < STAGECOACH_TEMPLATE < --template.
  - Load rejects a resolved no-$msg template (assert the "template: invalid template" error).

Task 10: ADD integration tests (dedupe-sees-templated + uniform decompose)
  - internal/generate: with a stub agent producing "Fix parser" and cfg.Template="$msg (#205)", assert
    Result.Message == "Fix parser (#205)"; and that a templated subject matching a recent subject forces a
    retry (mirror generate_test.go's dedupe test with the template set).
  - internal/decompose: with cfg.Template set, assert EVERY CommitResult.Message carries the template
    (per-concept + the FR-M11 shortcut path). Mirror the existing decompose orchestration test harness.

Task 11: ADD the config-init collision regression test (internal/cmd)
  - Assert `stagecoach config init --template` parses the LOCAL bool (true) and `stagecoach --template x`
    sets cfg.Template — the two flags do not clash. Mirror any existing cmd flag test.

Task 12: EDIT docs/cli.md + docs/configuration.md
  - cli.md: --template global-flags row (after L39) + env/git row (L168-169 block). Leave the config-init
    --template row (L106) intact.
  - configuration.md: template in the [generation] file example (after L105), defaults table (L128-129),
    env table (L162-163), git-config table (L185-186).

Task 13 (optional, consistency): if internal/cmd/config.go's exampleConfigTemplate lists format/locale in
  its [generation] block, add a commented `# template = ""  # ... must contain $msg` line alongside them
  (Mode-A reference-config parity). Skip if format/locale are not present there.
```

### Implementation Patterns & Key Details

```go
// generate.go — the seam at the single accept point (unqualified; same package):
parseFail = false
m = FinalizeMessage(m, cfg) // §9.19 FR-F8 seam — template BEFORE dedupe (§9.7 judges the final subject)
signal.SetCandidate(m)      // rescue candidate = the message that would land
subject := ExtractSubject(m)
if IsDuplicate(subject, recent) { rejected = append(rejected, subject); candidate = m; continue }
msg = m

// runSingleShortcut — template the planner's message before the dup-check:
msg := generate.FinalizeMessage(plannerMsg, deps.Config) // §9.19 FR-F8
if dupCheckMessage(ctx, deps, msg, isUnborn) {           // dup-check the TEMPLATED subject
	var err error
	msg, err = generateMessage(ctx, deps, baseTree, tStart) // fallback templates internally — no double-apply
	if err != nil { return DecomposeResult{}, err }
}

// load.go — validation mirrors validateFormat exactly:
func validateTemplate(tpl string) error {
	if tpl == "" || strings.Contains(tpl, "$msg") {
		return nil
	}
	return fmt.Errorf("invalid template %q: must contain the literal $msg (e.g. %q)", tpl, "$msg (#205)")
}
// ...Load() tail:
if err := validateFormat(cfg.Format); err != nil { return nil, fmt.Errorf("format: %w", err) }
if err := validateTemplate(cfg.Template); err != nil { return nil, fmt.Errorf("template: %w", err) }
```

### Integration Points

```yaml
CONFIG (this subtask OWNS the field + full precedence):
  - add: Config.Template `toml:"template"` (config.go) + Defaults() Template: "".
  - wire: file.go ([generation].template decode+materialize+merge), git.go (stagecoach.template),
          load.go (STAGECOACH_TEMPLATE env + --template flag), root.go (--template global flag).
  - validate: validateTemplate at the Load() tail (hard $msg error).

GENERATE PACKAGE (owns the seam):
  - new: internal/generate/finalize.go (ApplyTemplate + FinalizeMessage).
  - the seam is imported/called by generate.go, decompose (message.go + decompose.go), pkg/stagecoach.

SEAM SITES (4):
  - generate.go loop, stagecoach.go runPipeline loop, decompose/message.go loop, decompose runSingleShortcut.
  - transitive: arbiter N+1 (chain.go), one-file short-circuit, --single escape — NO edits.

ORDERING CONTRACT (published for P1.M5.T1.S1):
  - FinalizeMessage is the seam; --edit slots AFTER template (FR-E3). Documented in FinalizeMessage's godoc.

DOCS (Mode A):
  - docs/cli.md (--template row + env/git row), docs/configuration.md (template key across 4 tables).

OUT OF SCOPE:
  - Prompt package (system.go/planner.go/format.go — S3). User payload (payload.go — S2.T2.S1).
  - --edit editor gate (P1.M5.T1.S1 — consumes this seam later). Hook exec (P1.M3 — git owns that message).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
gofmt -w internal/generate/finalize.go internal/generate/finalize_test.go \
        internal/config/config.go internal/config/file.go internal/config/git.go internal/config/load.go \
        internal/cmd/root.go internal/generate/generate.go pkg/stagecoach/stagecoach.go \
        internal/decompose/message.go internal/decompose/decompose.go
go build ./...   # all 4 seam sites + the new file must compile
go vet ./...
golangci-lint run
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./internal/generate/... -v   # ApplyTemplate/FinalizeMessage + dedupe-sees-templated
go test ./internal/config/... -v     # validateTemplate table + template precedence + no-$msg load rejection
go test ./internal/decompose/... -v  # uniform template across a decompose run (shortcut + per-concept)
go test ./internal/cmd/... -v        # config init --template bool vs root --template string (collision)
go test ./pkg/stagecoach/... -v       # runPipeline threads the template
```

### Level 3: Integration Testing (System Validation)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach
# Flag exists + disambiguated:
/tmp/stagecoach --help 2>&1 | grep -A2 -- '--template'
/tmp/stagecoach config init --help 2>&1 | grep -- '--template'   # still the inert-config bool
# Hard error on a resolved no-$msg template:
STAGECOACH_TEMPLATE='(#205)' /tmp/stagecoach --dry-run 2>&1 | grep 'must contain the literal $msg'
# Optional stub-agent smoke: with STAGECOACH_TEMPLATE='$msg (#205)' and a stub agent, --dry-run prints the
# templated message (mirror internal/generate/generate_test.go's stub pattern).
```

### Level 4: Regression & Cross-cutting

```bash
go test -race ./...     # full suite (make test)
make coverage-gate      # ≥85% on core packages
golangci-lint run ./...

# Byte-identity guard: with template=="" every commit path equals the pre-change output — all pre-existing
# generate/decompose/stagecoach tests MUST pass UNCHANGED (they never set cfg.Template).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` compiles (all 4 seam sites + finalize.go).
- [ ] `go test ./...` green; pre-existing generate/decompose/stagecoach tests pass unchanged (empty-template byte-identity).
- [ ] `go vet ./...` + `golangci-lint run` clean; `gofmt` no diff.

### Feature Validation
- [ ] `--template ""`/unset → every commit path byte-identical to today.
- [ ] `--template '$msg (#205)'` → `Fix parser` lands as `Fix parser (#205)`; dedupe compares the templated subject.
- [ ] Every commit in a decompose run is templated (per-concept, FR-M11 shortcut, arbiter N+1).
- [ ] Resolved template without `$msg` → hard error `template: invalid template ...` at load (all layers).
- [ ] Full precedence works: file < git < env < flag (template precedence test passes).
- [ ] `config init --template` still parses as the inert-config bool; root `--template x` sets cfg.Template.

### Code Quality Validation
- [ ] Seam lives in `internal/generate` (finalize.go), imported by all callers — no new import edges/cycle.
- [ ] FinalizeMessage godoc publishes the FR-E3 ordering contract for P1.M5.T1.S1.
- [ ] Config plumbing mirrors format/locale exactly (scalar REPLACE, validate-once-at-tail).
- [ ] No prompt-package / user-payload edits (post-generation only, §17.8).
- [ ] No double-templating (chain.go untouched; the FR-M11 fallback path not re-templated).

### Documentation & Deployment
- [ ] docs/cli.md `--template` row (with $msg contract + hard error) + env/git row; config-init row untouched.
- [ ] docs/configuration.md `template` key across file/defaults/env/git tables.
- [ ] STAGECOACH_TEMPLATE / stagecoach.template / [generation].template all documented.

---

## Anti-Patterns to Avoid

- ❌ Don't apply the template AFTER the dedupe check — §9.7 must judge the FINAL subject (FR-F8).
- ❌ Don't put ApplyTemplate in the prompt package — §17.8: "the model never sees it" (post-generation).
- ❌ Don't add ApplyTemplate in chain.go / one-file / escape — they reuse generateMessage/CommitStaged and
  are already templated; a second application double-wraps.
- ❌ Don't special-case subject-only substitution — `$msg` = the FULL message (subject+body) per spec.
- ❌ Don't validate per-layer — validate the resolved value once at the Load() tail (like validateFormat).
- ❌ Don't rename `config init --template` or the new global flag — the pflag collision is safe; disambiguate via help.
- ❌ Don't make Template flag-only — unlike --context it has the FULL precedence surface (mirror format/locale).
- ❌ Don't edit the S3 prompt scaffolds or the S2.T2.S1 user-payload builders — disjoint parallel work.

---

## Confidence Score

**9/10** for one-pass implementation success. The seam is narrow and fully pinned: one new file with two
pure functions, four insertion points with exact line numbers and anchor lines, the config plumbing copied
verbatim from the `format`/`locale` precedent (six spots, all located), the validation mirroring
`validateFormat`, and the transitive-coverage proof that only four edits achieve "every commit message in a
run." The two genuine judgment calls — (1) seam placement in `generate` (resolved by the import graph:
decompose + pkg/stagecoach already depend on generate) and (2) the `config init --template` cobra collision
(resolved by pflag's AddFlagSet skip semantics, and backed by a regression test) — are both settled and
test-locked. The −1 is residual risk in the decompose/stagecoach integration tests needing the existing stub
harness wired correctly; the pure-function and config tests are mechanical.
