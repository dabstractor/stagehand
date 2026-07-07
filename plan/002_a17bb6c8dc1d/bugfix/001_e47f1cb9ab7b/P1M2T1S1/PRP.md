# PRP — P1.M2.T1.S1: Tighten claude's `TooledFlags` to a staging-only git allowlist

> **Scope discipline.** This subtask is **only prong (a)** of the Issue 2 fix: tighten **claude's**
> `tooled_flags` so ref-mutating git commands (commit/push/update-ref/reset/rebase/amend) are
> **structurally unreachable** by the stager — delivering PRD §19's "cannot commit/amend/push" claim for
> claude. Prong (b) (honestly documenting pi's unsoped profile) is **P1.M2.T1.S2**; prong (c) (the
> defensive HEAD-movement guard in `internal/decompose`) is **P1.M2.T1.S3**. Do NOT touch `builtinPi`,
> `providers/pi.toml`, or anything in `internal/decompose`.
>
> **No external research needed.** The exact replacement string is FIXED by the work-item contract. The
> Claude `--allowed-tools` syntax is already a tracked "# TO CONFIRM at integration" item — ship the
> contract's value verbatim.

---

## Goal

**Feature Goal**: Replace claude's overly-broad stager allowlist `Bash(git:*)` (which permits **every**
git subcommand, including `git commit`/`push`/`update-ref`/`reset`/`rebase`/`amend`) with a staging-only
allowlist `Bash(git add:*,git apply:*,git status:*,git diff:*)` so the claude stager can only run
staging-relevant git operations and is structurally unable to mutate refs.

**Deliverable**:
1. `internal/provider/builtin.go` (`builtinClaude()`): the `TooledFlags` value + the explanatory comment.
2. `providers/claude.toml`: the `tooled_flags` value + the section comment + the rendered-command comment.
3. `internal/provider/builtin_test.go`: the `claudeTOML` literal, the `TestBuiltinClaude` `wantTooled`,
   and the `TestBuiltinManifests_RenderedCommand_Claude_Tooled` expected argv.

**Success Definition**: `go build ./... && go vet ./... && go test ./internal/provider/...` are GREEN;
`BuiltinManifests()["claude"].TooledFlags` contains the staging-only allowlist (not `Bash(git:*)`); and
both decode-parity oracles stay green (`builtinClaude()` == decoded `claudeTOML` == decoded
`providers/claude.toml`).

---

## Why

- **PRD §19 / §11.5 / §17.6 / §22.1**: the stager agent is *sold* as "structurally constrained — it
  cannot commit, amend, or push, because stagecoach owns every ref mutation." The shipped claude
  `Bash(git:*)` allowlist does NOT deliver this: it permits **every** git subcommand.
- A misbehaving claude stager could run `git commit`, `git push --force`, `git update-ref HEAD`, or
  `git reset --hard HEAD~5` — breaking the ref-mutation monopoly that underpins Stagecoach's safety model.
- The fix restricts the allowlist to `git add`/`git apply`/`git status`/`git diff` (+ `Read`/`Edit`),
  making ref mutation structurally unreachable for the claude stager. (Full root-cause + the three-pronged
  plan: `architecture/issue2_stager_toolset.md`.)

---

## What

Change the single allowlist token in claude's `TooledFlags`:

- **From:** `"Bash(git:*),Read,Edit"`
- **To:** `"Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit"`

in three byte-for-byte-identical places (the Go struct, the test TOML literal, and the reference TOML
file), and update the accompanying comments to explain the staging-only restriction. Everything else in
claude's manifest (`--setting-sources ""`, `--no-session-persistence`, the bare flags, all other fields)
is unchanged.

### Success Criteria

- [ ] `builtinClaude().TooledFlags` == `["--allowed-tools","Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit","--setting-sources","","--no-session-persistence"]`.
- [ ] `providers/claude.toml` `tooled_flags` decodes to the identical slice (decode-parity green).
- [ ] The `claudeTOML` test literal decodes to the identical slice (decode-parity green).
- [ ] `Bash(git:*)` appears NOWHERE in claude's manifest/toml/tests (the old value is fully gone).
- [ ] `go build ./... && go vet ./... && go test ./internal/provider/...` GREEN.

---

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the exact before/after string, every edit site (with line numbers), the
decode-parity trap, and the confirmed list of files that must NOT be touched are all below.

### Documentation & References

```yaml
# MUST READ — binding architecture (root cause + the three-pronged plan; this task = prong (a) only)
- file: plan/002_a17bb6c8dc1d/bugfix/001_e47f1cb9ab7b/architecture/issue2_stager_toolset.md
  why: proves Bash(git:*) permits commit/push/update-ref/reset; specifies the staging-only replacement
       string; lists the files to touch. Scope this task to claude ONLY (pi = S2, HEAD guard = S3).
  section: "The Fix — (a) Tighten claude's tooled_flags allowlist"

# The edit site (Go)
- file: internal/provider/builtin.go
  why: builtinClaude() TooledFlags (line 109) carries the old "Bash(git:*),Read,Edit"; the multi-line
       comment above it (100-104) explains the (now-wrong) "Bash(git:*) (git only)" rationale.
  pattern: the TooledFlags slice is a []string literal; only the SECOND element (the allowlist value)
           changes — keep "--allowed-tools", "--setting-sources","", "--no-session-persistence" as-is.
  gotcha: update the comment to explain the staging-only allowlist (git add/apply/status/diff; explicitly
          EXCLUDES commit/amend/push/update-ref/reset/rebase).

# The reference doc / decode-parity oracle #2 (TOML file)
- file: providers/claude.toml
  why: the tooled_flags array (line 68) + the section comment (57-65) + the RENDERED TOOLED COMMAND
       comment (line 74). This file IS a test oracle (TestProviderReferenceFiles_DecodeParity reads it).
  pattern: mirror providers/gemini.toml's comment style. Field line must equal the claudeTOML literal.
  gotcha: the allowlist string CONTAINS SPACES — keep it inside the TOML double-quotes exactly as specified.

# The decode-parity oracle #1 + field/render tests (test file)
- file: internal/provider/builtin_test.go
  why: claudeTOML literal (line 60); TestBuiltinClaude wantTooled (314 comment, 316 value);
       TestBuiltinManifests_RenderedCommand_Claude_Tooled want argv (762 comment, 773 value).
  gotcha: all three must use the byte-for-byte identical new string or reflect.DeepEqual fails.
```

### Current Codebase tree (relevant slice)

```bash
internal/provider/builtin.go          # builtinClaude() TooledFlags (L109) + comment (L100-104) — EDIT
providers/claude.toml                 # tooled_flags (L68) + comments (L57-65, L74) — EDIT
internal/provider/builtin_test.go     # claudeTOML (L60), TestBuiltinClaude (L314,316), Claude_Tooled (L762,773) — EDIT
internal/provider/render_test.go      # uses dualModeManifest() (NOT claude); generic token check — NO EDIT
internal/provider/merge_test.go       # custom merge fixture (NOT claude) — NO EDIT
internal/decompose/                   # stager tests check len(TooledFlags)!=0 only — NO EDIT
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 trap): reflect.DeepEqual parity across THREE artifacts. builtinClaude() (Go struct),
//   claudeTOML (builtin_test.go literal), and providers/claude.toml (file) must carry the IDENTICAL
//   allowlist string. Two decode-parity tests enforce this:
//     - TestBuiltinManifests_DecodeParity        (builtinClaude vs claudeTOML literal)
//     - TestProviderReferenceFiles_DecodeParity  (builtinClaude vs providers/claude.toml file)
//   If any one still says "Bash(git:*)", the test fails. Copy the contract string BYTE-FOR-BYTE
//   (the single spaces in "git add:*", "git apply:*", etc. are load-bearing).

// GOTCHA (spaces in TOML): the new allowlist contains spaces ("git add:*"). It MUST stay inside TOML
//   double-quotes (it already is). go-toml decodes a quoted string with spaces fine; DeepEqual compares
//   the full string. Do not "tidy" the spaces differently across the three artifacts.

// SCOPE: edit claude ONLY. builtinPi/providers/pi.toml is P1.M2.T1.S2 (pi has no git-scoped allowlist
//   flag — it is documented honestly there, not tightened here). internal/decompose HEAD guard is
//   P1.M2.T1.S3. Do NOT touch either.

// CONFIRMED no-op sites: render_test.go (dualModeManifest fixture, token-presence only), merge_test.go
//   (custom fixture), internal/decompose/* (len(TooledFlags)!=0 only — the new value is still non-empty).
```

---

## Implementation Blueprint

### Data models and structure

No model changes. A single string value inside an existing `[]string` (`Manifest.TooledFlags`).

### The exact before/after

```go
// BEFORE (builtin.go:109, claudeTOML:60, claude.toml:68, test expectations)
"--allowed-tools", "Bash(git:*),Read,Edit",

// AFTER (all three artifacts + test expectations — byte-for-byte identical)
"--allowed-tools", "Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit",
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/provider/builtin.go :: builtinClaude()
  - EDIT the TooledFlags value (line 109): "Bash(git:*),Read,Edit" →
    "Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit" (the contract string, verbatim).
  - EDIT the comment block above it (lines 100-104): replace the "Bash(git:*) (git only)" rationale with a
    staging-only explanation. Example wording:
      // TOOLED MODE (v2 §11.5 — the stager role). INVERTS claude's bare mode: instead of --tools "" (disable
      // ALL tools), ENABLE tools RESTRICTED via an allowlist to Bash(git add:*,git apply:*,git status:*,git diff:*)
      // + Read + Edit — the staging-relevant toolset ONLY. This makes ref-mutating git subcommands
      // (commit/push/update-ref/reset/rebase/amend) STRUCTURALLY UNREACHABLE for the stager, delivering the
      // §19 "cannot commit/amend/push" guarantee for claude. --setting-sources "" + --no-session-persistence
      // carry over from bare.
  - PRESERVE: the "--allowed-tools" token, "--setting-sources","", "--no-session-persistence", the
    "# TO CONFIRM (integration)" note, and the entire BareFlags block + all other claude fields.
  - GUARDRAIL: do NOT touch builtinPi or any other builtin.

Task 2: MODIFY providers/claude.toml :: tooled_flags section
  - EDIT the tooled_flags value (line 68): "Bash(git:*),Read,Edit" → the contract string (verbatim).
  - EDIT the section comment (lines 57-65): explain the staging-only allowlist (git add/apply/status/diff
    + Read + Edit; explicitly EXCLUDES commit/amend/push/update-ref/reset/rebase). Keep the
    "# TO CONFIRM (P3.M2.T3)" line.
  - EDIT the RENDERED TOOLED COMMAND comment (line 74): update the example to show the new allowlist string
    in the --allowed-tools argument.
  - GOTCHA: this file is a decode-parity oracle — its field line MUST equal the claudeTOML literal (Task 3).

Task 3: MODIFY internal/provider/builtin_test.go
  - EDIT the claudeTOML literal (line 60): "Bash(git:*),Read,Edit" → the contract string (verbatim).
    (This is decode-parity oracle #1 — must match builtinClaude() byte-for-byte.)
  - EDIT TestBuiltinClaude wantTooled: the comment (line 314) and the value (line 316) → the contract string.
    Suggested comment: "// TooledFlags: 5 tokens (tools ENABLED + staging-only git allowlist; --allowed-tools TO CONFIRM at integration)".
  - EDIT TestBuiltinManifests_RenderedCommand_Claude_Tooled want argv: the comment (line 762/773) and the
    "--allowed-tools" value (line 773) → the contract string. The rest of the want argv
    (--model sonnet / --system-prompt <sys> / --setting-sources "" / --no-session-persistence / -p) is unchanged.
```

### Implementation Patterns & Key Details

```go
// PATTERN: a single-element edit inside an existing []string literal. Keep the slice structure and the
// surrounding tokens (--allowed-tools, --setting-sources, --no-session-persistence) byte-identical; only
// the allowlist value string changes.
//
// GOTCHA (DeepEqual): the THREE carriers of the value — builtinClaude() (Go), claudeTOML (test literal),
// providers/claude.toml (file) — must agree EXACTLY. The cleanest workflow: paste the contract string into
// all three in one pass, then run `go test ./internal/provider/... -run 'DecodeParity|Claude' -v` to confirm.
```

### Integration Points

```yaml
CODE: builtin.go (builtinClaude TooledFlags + comment) only. No Render/Validate/Resolve/Merge changes.
CONFIG: none.
ROUTES/CLI: none (providers show claude auto-reflects the new value via the registry).
DOCS (Mode A): providers/claude.toml tooled_flags section comment + rendered-command comment.
OUT OF SCOPE: builtinPi/providers/pi.toml (S2), internal/decompose HEAD guard (S3), docs/providers.md (P1.M6 sweep).
```

---

## Validation Loop

### Level 1: Syntax & Style

```bash
go build ./...
go vet ./...
gofmt -l internal/provider/builtin.go internal/provider/builtin_test.go
# Expected: clean. Run gofmt -w on any listed file.
```

### Level 2: The provider-package test suite (the real gate)

```bash
go test ./internal/provider/... -v
# Expected: PASS. Verify explicitly:
#   TestBuiltinManifests_DecodeParity ............ claude case: builtinClaude == decoded claudeTOML
#   TestProviderReferenceFiles_DecodeParity/claude  providers/claude.toml == builtinClaude
#   TestBuiltinClaude ............................ TooledFlags == new staging-only allowlist
#   TestBuiltinManifests_RenderedCommand_Claude_Tooled  argv uses the new allowlist
#   TestPreferredBuiltins_MatchesBuiltinKeys ..... unaffected (no key change)
#   TestRender_TooledModeAppendsTooledFlags ...... unaffected (uses dualModeManifest, not claude)
```

> **If a decode-parity test fails for claude:** the three artifacts disagree on the allowlist string.
> Read the DeepEqual diff, then make `builtinClaude()`, `claudeTOML`, and `providers/claude.toml` carry the
> byte-for-byte identical contract string (watch the single spaces in "git add:*" etc.).

### Level 3: Whole-repo build/test (no transient break expected)

```bash
go build ./...   # Expected: clean (this is a value-only change; no signature/type change).
go test ./...    # Expected: all PASS. internal/decompose stager tests check len(TooledFlags)!=0 — still green.
```

### Level 4: Behavioral spot-check (proves the structural guarantee)

```bash
go build -o /tmp/stagecoach ./cmd/stagecoach
/tmp/stagecoach providers show claude | grep -A3 tooled_flags
# Expect: --allowed-tools "Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit" ...
# (and NO "Bash(git:*)" anywhere in the output).
grep -rn 'Bash(git:\*)' internal/provider/ providers/claude.toml   # Expected: ZERO matches (old value gone).
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on edited files.
- [ ] `go test ./internal/provider/...` PASS — both decode-parity tests green for claude.
- [ ] `go test ./...` PASS (internal/decompose stager tests unaffected — TooledFlags still non-empty).
- [ ] `grep -rn 'Bash(git:\*)' internal/provider providers/claude.toml` → ZERO matches.

### Feature Validation
- [ ] `builtinClaude().TooledFlags` contains `Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit`.
- [ ] `providers/claude.toml` and `claudeTOML` decode to the identical slice (DeepEqual).
- [ ] claude's TooledFlags is still non-empty (stager-capable; the empty-check still passes).

### Code Quality Validation
- [ ] Only the allowlist value + accompanying comments changed; slice structure and other tokens preserved.
- [ ] Comments explain the staging-only restriction and the ref-mutation exclusion (§19 alignment).
- [ ] No edits to builtinPi, providers/pi.toml, render_test.go, merge_test.go, or internal/decompose.

### Documentation
- [ ] `providers/claude.toml` tooled_flags section comment + rendered-command comment updated (Mode A).

---

## Anti-Patterns to Avoid

- ❌ **Don't leave `Bash(git:*)` in any of the three artifacts** — decode-parity DeepEqual will fail, and
  the structural guarantee is only delivered if ALL of claude's manifest surfaces agree.
- ❌ **Don't alter the spaces in the allowlist string** across artifacts — `git add:*` (one space) is
  load-bearing for byte-for-byte DeepEqual; "tidy" differently and parity breaks.
- ❌ **Don't touch pi (`builtinPi`/`providers/pi.toml`)** — pi's honest-documentation fix is S2.
- ❌ **Don't add the HEAD-movement guard** — that is S3 (internal/decompose).
- ❌ **Don't change claude's BareFlags, the `--setting-sources ""`/`--no-session-persistence` tokens, or any
  other claude field** — only the allowlist value (+ its comments) changes.
- ❌ **Don't edit render_test.go / merge_test.go** — they use custom fixtures and generic token checks, not
  claude's allowlist. (Verify, don't edit.)
- ❌ **Don't try to verify Claude Code's real `--allowed-tools` syntax online** — the exact string is fixed
  by the contract; the flag-vs-`--tools` question is already a tracked "# TO CONFIRM at integration" item.

---

## Confidence Score

**9/10** — A surgical, single-string change to one provider's `TooledFlags`, carried consistently across
three byte-for-byte-identical artifacts. Every edit site is enumerated (verified by grep + line numbers),
the decode-parity trap is flagged with the exact resolution, and the confirmed no-op sites (render/merge
fixtures, decompose len-checks) are listed to prevent over-editing. The -1 reserves for the exact comment
wording (non-blocking) and the residual "# TO CONFIRM at integration" on Claude's flag syntax (pre-existing,
tracked, out of this task's scope).
