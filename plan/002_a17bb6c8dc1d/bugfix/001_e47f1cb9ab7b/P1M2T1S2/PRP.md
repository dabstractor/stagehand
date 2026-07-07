# PRP — P1.M2.T1.S2: Honestly document pi's unsoped tooled profile

> **Scope discipline.** This subtask is **prong (b)** of the Issue 2 fix (the stager toolset is not
> actually scoped): **comment-only** honesty fixes in `builtinPi()` and `providers/pi.toml`. Prong (a)
> (tightening **claude's** allowlist) was **P1.M2.T1.S1** (already merged); prong (c) (the defensive
> HEAD-movement guard in `internal/decompose`) is **P1.M2.T1.S3** (Planned — not yet landed). S2 changes
> **comments only** — no `TooledFlags` value, no code, no tests, no behavior change. Do NOT touch
> `builtinClaude`, `providers/claude.toml`, `internal/decompose/*`, or any `.go` logic.

---

## Goal

**Feature Goal**: Replace the current **soft-language** comments on pi's tooled profile (which imply
the stager's safety comes from "stagecoach's ref-mutation monopoly") with **honest** comments that
clearly state pi's stager is **INSTRUCTIONALLY** constrained (via the §17.6 stager task prompt) and
**BEST-EFFORT** guarded (via the P1.M2.T1.S3 HEAD-movement guard), **NOT** structurally/flag-scoped —
and that a misbehaving pi stager **CAN** run `git commit`/`push`/`update-ref`/`rm -rf`. This eliminates
the misleading impression that pi meets PRD §19's "structurally constrained, cannot commit/amend/push"
claim, and explicitly contrasts pi with claude (which, post-S1, IS structurally constrained).

**Deliverable**:
1. `internal/provider/builtin.go` — the `builtinPi()` `TooledFlags` comment block (lines ~59-63) rewritten.
2. `providers/pi.toml` — the `# --- tooled mode` section comment block rewritten (Mode A reference doc).

**Success Definition**: Both comment blocks plainly say pi is instructional + best-effort (not
structural), name the S3 HEAD-movement guard as the safety net, call out the concrete dangerous
commands a misbehaving pi stager can run, and contrast with claude's structural allowlist. The
`TooledFlags` **value** is byte-for-byte unchanged; `go build ./... && go vet ./... &&
go test ./...` stay GREEN (comments don't affect compilation/tests).

---

## Why

- **PRD bugfix Issue 2 / §19 / §11.5 / §17.6 / §22.1**: the stager agent is *sold* as "structurally
  constrained — it cannot commit, amend, or push." For pi this is **not true**: pi's `tooled_flags` =
  bare flags **MINUS `--no-tools`** = pi's entire native tool system ON with **no allowlist**. A pi
  stager can run arbitrary Bash, including ref-mutating commands.
- **pi cannot be flag-scoped**: pi exposes only the all-or-nothing `--no-tools` switch (verified via
  `pi --help`); there is no git-subcommand allowlist. Disabling tools entirely (`--no-tools`) would bar
  the stager from running `git add` at all. So unlike claude (prong a), pi's profile **cannot be
  tightened** — the honest remedy is **accurate documentation**, not a flag change.
- **Current comments use soft language**: they say stager safety is "via the stager task prompt +
  stagecoach's ref-mutation monopoly … not by flag-scoping." That buries the lede — "ref-mutation
  monopoly" only holds if the stager *cannot* mutate refs itself, which for pi is **not** flag-enforced.
  A reader auditing safety would be misled into thinking §19's structural guarantee covers pi.
- **The real safety net for pi is the S3 HEAD-movement guard** (defense-in-depth): it snapshots HEAD
  before each stager call and aborts if HEAD moved when the stager returns. The comments must say so.

---

## What

Rewrite **two comment blocks** (and only those). No `TooledFlags` slice value changes; no code changes;
no test changes. Both blocks must convey four points:

1. pi has no git-scoped allowlist flag — only all-or-nothing `--no-tools`; cannot be tightened without
   disabling all tools (which would break staging).
2. pi's stager is **NOT structurally/flag-scoped** — a misbehaving pi stager **CAN** run arbitrary Bash,
   including `git commit`, `git push`, `git update-ref`, `git reset`, `rm -rf`.
3. PRD §19's "structurally constrained … cannot commit/amend/push" claim therefore does **NOT** hold
   for pi. pi's stager is instead **(a) INSTRUCTIONALLY** constrained (the §17.6 stager task prompt
   tells it to stage only) and **(b) BEST-EFFORT** guarded by the **HEAD-movement defense-in-depth
   check (P1.M2.T1.S3)** — the safety net is *that guard*, not flag-scoping.
4. **Contrast with claude**: claude's stager **IS** structurally constrained by a staging-only git
   allowlist (`Bash(git add:*,git apply:*,git status:*,git diff:*)`, post-S1).

### Success Criteria

- [ ] `builtinPi()` comment block rewritten to state instructional + best-effort, name S3, list dangerous commands, and contrast with claude.
- [ ] `providers/pi.toml` tooled-mode comment block rewritten with the same content (Mode A).
- [ ] The `TooledFlags` **value** (`--no-extensions`/`--no-skills`/`--no-prompt-templates`/`--no-context-files`/`--no-session`) is byte-for-byte **unchanged** in both files.
- [ ] No `.go` logic, no other manifest field, and no test is touched.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN.

---

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the exact two comment blocks (with current text and copy-paste-ready
replacements), the precise reasoning, the S3 forward-reference note, and the validation commands are
all below.

### Documentation & References

```yaml
# MUST READ — binding architecture (root cause + three-prong plan; S2 is prong b)
- file: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue2_stager_toolset.md
  why: Proves pi's TooledFlags = bare MINUS --no-tools (no allowlist); states pi cannot be tightened
       without --no-tools; names the three-prong fix and that S2 is "honestly document pi's unsoped
       toolset" (comment-only). Confirms the S3 HEAD-movement guard is the defense-in-depth layer.
  section: "### (b) Honestly document pi's unsoped toolset" and "### (c) Add a defensive HEAD-movement guard"

# SIBLING CONTEXT — prong (a), already merged (READ-ONLY; the claude contrast S2 must reference)
- file: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/P1M2T1S1/PRP.md
  why: Documents that claude's TooledFlags was tightened to `Bash(git add:*,git apply:*,git status:*,
       git diff:*),Read,Edit` — the STRUCTURAL allowlist S2's comments must contrast pi AGAINST.
  critical: S2 references claude's tightened allowlist verbatim; do not restate it differently.

# EDIT SITE 1 — the Go source comment
- file: internal/provider/builtin.go
  why: builtinPi() TooledFlags comment block (lines ~59-63). Currently uses soft language
       ("enforced by the stager task prompt + stagecoach's monopoly … not by flag-scoping"). Rewrite it.
  pattern: The comment sits immediately ABOVE the `TooledFlags: []string{ ... }` literal. Keep the
           literal untouched. The surrounding Manifest fields (BareFlags above, Output/StripCodeFence
           below) are unchanged.
  gotcha: Go comments are `//` per line. Keep lines a reasonable width (the file uses ~100-110 char
          lines in this block). The comment is NOT a doc comment (builtinPi is unexported) — `//` line
          comments are correct; do NOT add a `/* */` block.

# EDIT SITE 2 — the Mode A reference doc
- file: providers/pi.toml
  why: The `# --- tooled mode (v2; §11.5: the stager role) ---` comment block above the `tooled_flags`
       array. Currently soft ("stager safety is via the stager task prompt + stagecoach's ref-mutation
       monopoly, not flag-scoping"). Rewrite it. This file is HUMAN-READABLE REFERENCE DOCUMENTATION
       mirroring builtinPi() byte-for-byte (see its header comment) — it is NOT loaded at runtime.
  pattern: TOML `#` comments. The `tooled_flags = [ ... ]` ARRAY VALUE stays byte-for-byte identical;
           only the prose comment above it changes. Leave the later "# RENDERED TOOLED COMMAND" block
           and every other field/comment in the file untouched.
  gotcha: This file mirrors builtinPi() — the safety-model wording in the two files must AGREE (same
          four points). providers show does not print these comments, but the doc must match the source.

# THE FORWARD-REFERENCED GUARD (prong c, NOT yet implemented — be accurate)
- note: P1.M2.T1.S3 (the HEAD-movement guard in internal/decompose) is PLANNED, not yet landed. The
        current "HEAD moved" strings in internal/decompose are PUBLISH-TIME CAS failures (a different
        mechanism), NOT the stager-call HEAD guard. S2's comments must reference S3 as the
        defense-in-depth guard BY SUBTASK ID (P1.M2.T1.S3) so the reference is accurate now and after
        S3 lands. Do NOT claim the guard already exists in decompose code.
```

### Current Codebase tree (relevant slice)

```bash
internal/provider/builtin.go   # builtinPi() TooledFlags comment (~L59-63) — EDIT (comment only)
providers/pi.toml              # tooled-mode comment block — EDIT (Mode A, comment only)
# READ-ONLY references:
plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue2_stager_toolset.md  # root cause + 3-prong plan
plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/P1M2T1S1/PRP.md                         # claude's tightened allowlist (the contrast)
internal/provider/builtin.go   # builtinClaude() — READ-ONLY (the structural-allowlist contrast)
providers/claude.toml          # READ-ONLY (claude's tooled_flags reference doc)
```

### Desired Codebase tree (files MODIFIED; no new files)

```bash
internal/provider/builtin.go   # builtinPi() comment rewritten (TooledFlags value unchanged)
providers/pi.toml              # tooled-mode comment rewritten (tooled_flags value unchanged)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (scope): COMMENT-ONLY. Do NOT change the TooledFlags slice value in either file. The five
//   flags (--no-extensions/--no-skills/--no-prompt-templates/--no-context-files/--no-session) must stay
//   byte-for-byte identical, or builtinPi() and providers/pi.toml drift out of mirror-parity and the
//   decode-parity oracles (internal/provider/builtin_test.go) may fail.

// CRITICAL (S3 is not yet landed): the comments reference the P1.M2.T1.S3 HEAD-movement guard as a
//   FORWARD reference. Do NOT phrase it as "decompose already guards…" — phrase it as "the HEAD-movement
//   defense-in-depth check (P1.M2.T1.S3)". The existing "HEAD moved" strings in internal/decompose are
//   publish-time CAS failures (§13.5), a DIFFERENT mechanism — do not conflate them.

// GOTCHA (parity): providers/pi.toml mirrors builtinPi() (its header says "BYTE-FOR-BYTE modulo
//   comments"). The safety-model wording must AGREE across the two files — same four points, same
//   contrast with claude. A reader of either source must reach the same conclusion.

// GOTCHA (no test impact): no test asserts on these comment strings (verified: grep for
//   "no git-scoped allowlist"/"structurally"/"instructionally" in *_test.go returns nothing). So the
//   edit cannot break a test via text-matching. `go test ./...` stays green because nothing executable
//   changes.

// GOTCHA (claude contrast accuracy): claude's tightened allowlist (post-S1) is exactly
//   `Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit`. Quote it verbatim in the contrast —
//   do not paraphrase (a wrong token would mislead anyone copying it).
```

---

## Implementation Blueprint

### Task 1: Rewrite the `builtinPi()` comment in `internal/provider/builtin.go`

The current block (lines ~59-63), immediately above `TooledFlags: []string{`:

```go
		// TOOLED MODE (v2 §11.5 — the stager role). pi has no git-scoped allowlist (--help shows only the
		// all-or-nothing --no-tools), so pi's tooled profile = the bare invocation MINUS --no-tools: pi's
		// native tool system ON, everything else still off (chrome-less + ephemeral). The stager's safety
		// (git-only, never commit/update-ref/push) is enforced by the stager task prompt (§17.6) + stagecoach's
		// monopoly on ref mutations (§13.6.2/§19), not by flag-scoping.
```

Replace with (keep the `TooledFlags: []string{ ... }` literal BELOW it byte-for-byte unchanged):

```go
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
```

### Task 2: Rewrite the tooled-mode comment in `providers/pi.toml`

The current block (the `# --- tooled mode ...` prose above `tooled_flags = [`):

```toml
# --- tooled mode (v2; §11.5: the stager role) ---
# pi's tooled profile = bare MINUS --no-tools: pi's native tool system ON,
# everything else still off (chrome-less + ephemeral). pi has NO git-scoped allowlist flag;
# stager safety is via the stager task prompt + stagecoach's ref-mutation monopoly, not flag-scoping.
```

Replace with (keep the `tooled_flags = [ ... ]` array BELOW it byte-for-byte unchanged):

```toml
# --- tooled mode (v2; §11.5: the stager role) ---
# pi's tooled profile = bare MINUS --no-tools: pi's native tool system ON, everything else still off
# (chrome-less + ephemeral). pi has NO git-scoped allowlist flag (--help shows only all-or-nothing
# --no-tools), so its tools cannot be scoped to staging-only git subcommands without disabling tools
# entirely (which would bar the stager from running git at all).
#
# SAFETY MODEL — HONEST (contrast with claude): claude's stager IS structurally constrained by a
# staging-only git allowlist (Bash(git add:*,git apply:*,git status:*,git diff:*); see claude.toml).
# pi's stager is NOT structurally/flag-scoped — a misbehaving pi stager CAN run arbitrary Bash,
# including `git commit`, `git push`, `git update-ref`, `git reset`, and `rm -rf`. PRD §19's
# "structurally constrained … cannot commit/amend/push" claim therefore does NOT hold for pi. pi's
# stager is instead:
#   (1) INSTRUCTIONALLY constrained — by the §17.6 stager task prompt (it is told to stage only), and
#   (2) BEST-EFFORT guarded — by the HEAD-movement defense-in-depth check (P1.M2.T1.S3): HEAD is
#       snapshotted before each stager call and the run aborts (safety violation) if HEAD moved when the
#       stager returns. THE SAFETY NET IS THIS GUARD, NOT FLAG-SCOPING.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/provider/builtin.go :: builtinPi() comment
  - REPLACE: the 5-line TooledFlags comment block (current text above) with the honest block (above).
  - PRESERVE: the TooledFlags slice literal immediately below — byte-for-byte unchanged
    (--no-extensions/--no-skills/--no-prompt-templates/--no-context-files/--no-session).
  - PRESERVE: every other field in builtinPi() (BareFlags, Output, StripCodeFence, etc.) and all other
    functions in the file (builtinClaude, builtinGemini, …). Edit ONE comment block only.
  - STYLE: `//` line comments (builtinPi is unexported; no doc-comment `/* */`). Keep ~100-110 char lines.
  - DEPENDENCIES: none (pure comment).

Task 2: MODIFY providers/pi.toml :: tooled-mode comment (Mode A)
  - REPLACE: the 4-line `# --- tooled mode ...` prose block (current text above) with the honest block.
  - PRESERVE: the `tooled_flags = [ ... ]` array value (byte-for-byte) and the later
    `# RENDERED TOOLED COMMAND` block. Edit ONE comment block only.
  - PARITY: the wording must AGREE with Task 1 (same four points, same claude contrast) — this file
    mirrors builtinPi() byte-for-byte modulo comments (see the file's header).
  - DEPENDENCIES: none (pure comment; this file is reference doc, not loaded at runtime).
```

### Implementation Patterns & Key Details

```go
# PATTERN (comment-only honesty fix): replace "soft" hedging ("safety is via … monopoly, not flag-scoping")
#   with an explicit SAFETY MODEL — HONEST header that (1) names what is NOT true (structural scoping),
#   (2) lists the concrete dangerous commands, (3) names the two real mechanisms (§17.6 prompt + S3 guard),
#   and (4) contrasts with the sibling provider that DOES meet the structural claim (claude, post-S1).
#
# WHY reference S3 by subtask ID (P1.M2.T1.S3), not "decompose already …": S3 is PLANNED, not landed. The
#   existing "HEAD moved" strings in internal/decompose are publish-time CAS failures (§13.5), a DIFFERENT
#   mechanism. Naming the subtask makes the comment accurate now AND after S3 lands.
#
# WHY quote claude's allowlist verbatim: a wrong/paraphrased token (e.g. Bash(git add:*)) would mislead
#   anyone copying it into a config. Use the exact post-S1 value: Bash(git add:*,git apply:*,git status:*,git diff:*).
```

### Integration Points

```yaml
CODE: none (comment-only — no TooledFlags value, no logic, no signature change).
DATABASE: none.
CONFIG: none (providers/pi.toml is reference documentation, not loaded at runtime; the [provider.pi]
        override path reads .stagecoach.toml, not this file).
ROUTES: none.
SIGNALS: none.
DOWNSTREAM: none — S1 (claude) is merged; S3 (HEAD guard) is independent. This is the last documentation
  touch for pi's tooled profile in this milestone.
```

---

## Validation Loop

### Level 1: Syntax & Style (run after Tasks 1-2)

```bash
# Comments do not affect compilation, but build/vet to be safe (catches a stray edit to the slice).
go build ./...
go vet ./...
# Expected: clean.

# gofmt the edited Go file (TOML has no formatter in this repo; just keep `#` comments tidy).
gofmt -l internal/provider/builtin.go
# Expected: lists nothing. If it does: gofmt -w internal/provider/builtin.go
```

### Level 2: Tests (comment-only — must stay green)

```bash
# No new tests. The full suite must stay green (especially the decode-parity oracles that assert
# builtinPi() == decoded providers/pi.toml — those compare VALUES, not comments, so they stay green
# as long as the TooledFlags value was not touched).
go test ./...
# Expected: all PASS.
```

### Level 3: Parity & honesty verification (manual read-back)

```bash
# Confirm the TooledFlags VALUE is unchanged in both files (5 flags, same order).
grep -A6 'TooledFlags: \[\]string{' internal/provider/builtin.go | grep -- '--no-'
grep -A8 'tooled_flags = \[' providers/pi.toml | grep -- '--no-'
# Expected: both list exactly --no-extensions/--no-skills/--no-prompt-templates/--no-context-files/--no-session.

# Confirm the honesty markers are present in BOTH files and the old soft language is gone.
grep -n "SAFETY MODEL — HONEST\|NOT structurally/flag-scoped\|BEST-EFFORT guarded\|P1.M2.T1.S3" \
    internal/provider/builtin.go providers/pi.toml
# Expected: each marker appears in BOTH files.

# Confirm the old soft-language phrase is GONE (no longer claims monopoly-as-safety).
grep -n "monopoly on ref mutations\|ref-mutation monopoly, not flag-scoping" \
    internal/provider/builtin.go providers/pi.toml
# Expected: no matches (the soft phrasing was replaced).
```

### Level 4: providers show spot-check (the comment does NOT change runtime output — sanity)

```bash
# OPTIONAL: `stagecoach providers show pi` prints the manifest VALUE (TooledFlags), not these comments.
# So its output is unchanged. This is just a sanity check that nothing executable drifted.
go build -o /tmp/stagecoach ./cmd/stagecoach && /tmp/stagecoach providers show pi | grep -A6 tooled_flags
# Expected: the same 5 --no-* flags as before (unchanged).
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean.
- [ ] `go vet ./...` clean.
- [ ] `gofmt -l internal/provider/builtin.go` reports nothing.
- [ ] `go test ./...` — entire suite green (comment-only; decode-parity oracles still pass).

### Feature Validation (honest documentation)
- [ ] `builtinPi()` comment states pi is NOT structurally/flag-scoped and lists dangerous commands.
- [ ] `builtinPi()` comment names the §17.6 instructional prompt + the P1.M2.T1.S3 HEAD-movement guard as the safety net.
- [ ] `builtinPi()` comment contrasts with claude's structural allowlist (quoted verbatim).
- [ ] `providers/pi.toml` tooled-mode comment carries the same four points (parity with the Go source).
- [ ] The §19 "structurally constrained" claim is explicitly qualified as NOT holding for pi.

### Code Quality Validation
- [ ] COMMENT-ONLY: the `TooledFlags` value is byte-for-byte unchanged in both files.
- [ ] No `.go` logic, no other manifest field, no test, and no other provider touched.
- [ ] The two files AGREE (same safety-model wording) — mirror parity maintained.
- [ ] S3 is referenced by subtask ID (accurate now and after S3 lands); existing CAS "HEAD moved" strings are NOT conflated with it.

### Documentation (Mode A)
- [ ] `providers/pi.toml` tooled_flags comment updated with the instructional + best-effort model.
- [ ] The claude contrast is explicit in the reference doc.

---

## Anti-Patterns to Avoid

- ❌ **Don't change the `TooledFlags` slice value** (or the `tooled_flags` array in the TOML) — this is
  comment-only. A value change would break mirror parity with `builtinPi()` and the decode-parity oracles.
- ❌ **Don't claim the HEAD-movement guard already exists in decompose** — S3 (P1.M2.T1.S3) is PLANNED,
  not landed. Reference it BY SUBTASK ID. The existing "HEAD moved" strings are publish-time CAS failures
  (§13.5), a different mechanism — do not conflate them.
- ❌ **Don't keep the soft language** ("safety is via … monopoly, not flag-scoping") — it buries the fact
  that pi's stager is not flag-scoped and that §19's structural claim does not hold for pi. Replace it
  with the explicit SAFETY MODEL — HONEST block.
- ❌ **Don't paraphrase claude's allowlist** — quote the exact post-S1 value
  `Bash(git add:*,git apply:*,git status:*,git diff:*)` so anyone copying it gets a working token.
- ❌ **Don't edit `builtinClaude`, `providers/claude.toml`, `internal/decompose/*`, or any test** — S1
  owns claude; S3 owns the guard; this task owns only pi's two comment blocks.
- ❌ **Don't downgrade pi to `--no-tools`** to "make it safe" — that would prevent the stager from running
  `git add` at all (a behavior change, out of scope, and the contract's chosen remedy is honest docs).
- ❌ **Don't let the two files drift** — the Go source and the TOML reference must state the same safety
  model (the TOML header says it mirrors `builtinPi()` byte-for-byte modulo comments).

---

## Confidence Score

**9/10** — A comment-only honesty fix in exactly two files, with the current text and copy-paste-ready
replacements provided, the four required points enumerated, the claude contrast quoted verbatim, and the
one subtlety (S3 is planned, not landed — reference by subtask ID, don't conflate with CAS "HEAD moved")
flagged prominently. The -1 reserves for line-width/wrapping judgment in the Go comment block (the file
uses ~100-110 char lines; the implementer may reflow slightly), which is non-blocking and does not affect
the message.
