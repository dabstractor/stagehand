name: "P1.M7.T1.S2 — Stale-reference sweep and known-stale string fix"
description: |

  Mode-B (changeset-level docs-correctness) subtask. There is ONE confirmed stale string:
  `internal/cmd/config.go`'s `exampleConfigTemplate` header still says "This binary supports
  config_version = 2" (and has a commented `# config_version = 2` example line) while
  `config.CurrentConfigVersion = 3`. Fix both. Then grep-VERIFY (do not assume) that no stale claims
  survive across README.md, docs/, and user-facing strings (cobra help text + templates) for five
  patterns: pre-v3 terminology (`--planner-agent` flag, `[agent.*]` config blocks presented as
  current), "config_version = 2", superseded non-goal phrasings ("no hook installer",
  "--edit is deferred/v1.1"), and any reference presenting `COMPETITOR-ANALYSIS.md` as an existing
  repo file (it does not exist). Fix what is found; for each pattern that is clean, RECORD that clean
  result in the commit message. OUTPUT: zero stale references in shipped docs and user-facing strings;
  the one known-stale template header corrected.

---

## Goal

**Feature Goal**: Every shipped doc (`README.md`, `docs/*.md`) and every user-facing string (cobra
command help text, the `exampleConfigTemplate` written to disk by `config init --template`) is free of
stale pre-v3 / superseded / non-existent-file claims, and the one CONFIRMED stale claim — the
`exampleConfigTemplate` `config_version` header still asserting "2" while the binary is v3 — is corrected.
The sweep is performed, not assumed: each of the five patterns is grepped and its result (fixed OR
verified-clean) is recorded per-pattern.

**Deliverable**: A one-constant edit to `internal/cmd/config.go` (the `exampleConfigTemplate` string:
two `2`→`3` replacements in its `config_version` block) plus a grep-verification sweep whose per-pattern
clean results are captured in the commit message. NO other file is modified (README.md and docs/README.md
are owned by the parallel P1.M7.T1.S1; PRD.md / FUTURE_SPEC.md / tasks.json are read-only).

**Success Definition**:
- `internal/cmd/config.go`'s `exampleConfigTemplate` `config_version` block reads `3` in BOTH the header
  prose ("This binary supports config_version = 3.") and the commented example line (`# config_version = 3`).
- `go build ./...` compiles; `go test ./internal/cmd/... ./internal/config/...` passes (the `--template`
  equality test stays green because the constant is edited on both sides of the comparison).
- A documented grep sweep proves ZERO stale hits for each of the five patterns within the task scope
  (docs/ + README + user-facing .go strings), with the migration-code / regex-comment / v2-test-fixture
  occurrences explicitly distinguished (those are CORRECT, not stale).

## User Persona

**Target User**: A user who runs `stagecoach config init --template` to get the inert reference config and
reads its header to learn which schema the binary speaks — and any reader of the shipped docs who must not
be misled by a claim that contradicts the shipped binary.

**Use Case**: The user opens the generated config, sees the `config_version` block, and trusts that the
number and the prose match the binary's `CurrentConfigVersion` (3). A v2 number silently teaches them the
wrong schema and undercuts the FR-B4 version-warning contract.

**Pain Points Addressed**: A stale "supports config_version = 2" in a user-facing template is a quiet lie
about the binary's contract; pre-v3 / superseded phrasings left in docs make shipped features (hook mode,
`--edit`) look deferred when they are real.

## Why

- **Item contract (LOGIC/OUTPUT)**: fix the confirmed stale `exampleConfigTemplate` header; grep-verify the
  five patterns across README/docs/user-facing strings; record a clean result per pattern; output zero stale
  references + the one correction.
- **architecture/system_context.md §6 item 2**: "config_version is 3; only remnant is the stale '= 2' prose
  in `exampleConfigTemplate` (fix in docs sweep)." This task owns that fix.
- **PRD §9.17 FR-B4**: the binary's `CurrentConfigVersion` is the contract a config_version line is judged
  against — a template that prints "2" while the constant is 3 is a self-contradiction.
- **delta_prd.md expectation**: the pre-v3 terminology sweep must come back clean (verified, not assumed).

## What

User-visible behavior: a user running `config init --template` gets a config whose `config_version` block
correctly states the binary speaks schema v3 (header prose + example line both `3`). No shipped doc or
user-facing string anywhere else carries a stale pre-v3 claim, a superseded non-goal phrased as current, or
a pointer to the non-existent `COMPETITOR-ANALYSIS.md`.

### Success Criteria

- [ ] `exampleConfigTemplate`'s `config_version` header prose says "supports config_version = 3."
      (was 2).
- [ ] `exampleConfigTemplate`'s commented example line is `# config_version = 3` (was `# config_version = 2`).
- [ ] `go build ./...` succeeds; `go test ./internal/cmd/... ./internal/config/...` is green.
- [ ] Commit message records, per pattern, the grep result (fixed OR verified-clean) for: (1) pre-v3
      terminology, (2) `config_version = 2`, (3) superseded non-goal phrasings, (4) COMPETITOR-ANALYSIS.md,
      distinguishing the CORRECT occurrences (migration code, regex comment, v2 test fixtures, read-only
      PRD/FUTURE_SPEC historical context) from any that were actually stale.

## All Needed Context

### Context Completeness Check

_This PRP names the EXACT constant to edit (`exampleConfigTemplate` in `internal/cmd/config.go`), the
EXACT two `2`→`3` replacements (header prose line ~526 + commented example line ~528), the EXACT
authoritative version source (`config.CurrentConfigVersion = 3` at `internal/config/config.go:11`), the
EXACT test that governs safety (`config_test.go:438` full-string equality on the constant), the EXACT five
grep patterns + their known-CORRECT occurrences to exclude, and the EXACT build/test/lint validation
commands. An implementer with no prior codebase knowledge can do this from the document + file access._

### Documentation & References

```yaml
- file: internal/cmd/config.go
  why: THE file containing the one confirmed stale string. `exampleConfigTemplate` (const, begins ~L497)
       is the inert commented config written by `config init --template` — it IS user-facing (lands on a
       user's disk) and IS the Mode-A config documentation surface.
  pattern: Go raw-ish string constant built with `+` concatenation (backticks for the body, `"..."` slices
           for inline code). The `config_version` block lives ~L524–L529.
  gotcha: edit BOTH the header prose AND the commented example line (two `2`→`3` in the same constant).
          Do NOT touch line ~122 (the regex-anchoring comment that uses `# config_version = 2` as its
          teaching example — that is CORRECT, not stale).

- file: internal/config/config.go
  why: authoritative version source. Line 11: `const CurrentConfigVersion = 3`. The template header MUST
       match this constant's value.
  pattern: the constant has a doc comment citing PRD §9.17 FR-B4 and explaining v3 = inference provider
           folded into model slash-prefix (FR-B7, FR-R5b).

- file: internal/cmd/config_test.go
  why: governs whether the edit is safe. Line ~438: `if got != exampleConfigTemplate` — the `--template`
       output is compared by FULL-STRING equality to the constant. Editing the constant keeps BOTH sides
       of the comparison identical → the test stays green. No test pins the literal substring "= 2".
  pattern: populated-bootstrap tests (config_test.go:180/218/287/340) assert `config_version = 3` on the
           POPULATED path (not the `--template` path) and are unrelated to this edit.
  gotcha: if the full-string-equality test fails after your edit, you changed the constant body in a way
          that diverged from what `config init --template` writes — re-read the constant, you likely
          introduced a typo or altered whitespace. The fix is 2 digits, nothing more.

- file: internal/cmd/default_action_test.go + internal/config/file_test.go
  why: these contain `config_version = 2` as v2 FIXTURE INPUTS to the migration tests (e.g.
       default_action_test.go:1204, config_test.go:1167+, file_test.go:548/669). They represent legacy
       files being upgraded → CORRECT. Do NOT "fix" them to 3 — that would delete the test coverage for
       v2→v3 migration.
  pattern: test inputs are real v2 documents; the assertions verify they migrate to 3.

- docfile: plan/005_c38aa48290f0/P1M7T1S2/research/stale_reference_sweep_findings.md
  why: THE completed grep sweep. Lists the one confirmed stale string (with exact line refs), the
       test-safety analysis, and the per-pattern clean results (which occurrences are CORRECT vs stale).
       Mirror its findings into the commit message's per-pattern record.
  section: "The ONE confirmed stale string" + "Patterns verified CLEAN".

- file: plan/005_c38aa48290f0/P1M7T1S1/PRP.md  (the parallel sibling task)
  why: S1 owns README.md + docs/README.md. Its scope fence explicitly DEFERS the code stale-string
       (`internal/cmd/config.go`) to THIS task (S2). Do not touch README.md or docs/README.md — S1 does.
  section: S1 "Known Gotchas" first block + S1 "scope fence" notes name `internal/cmd/config.go` as S2's.

- file: plan/005_c38aa48290f0/architecture/system_context.md  (§6, items 2 and 5)
  why: the source of "one confirmed stale string" (item 2) and "COMPETITOR-ANALYSIS.md does not exist"
       (item 5). Cite both as the contract for this task.

- url: PRD §9.17 FR-B4 (CurrentConfigVersion contract), §10.2 v1.1-resolved (--edit graduated),
       §10.4 v2.1 (hook mode + integrations accepted — "hook installer"/"--edit" are no longer deferred)
  why: the product contracts that decide which phrasings are stale.
  section: read in plan/005_c38aa48290f0/prd_snapshot.md (read-only PRD copy).
```

### Current Codebase tree (the slice this task touches)

```bash
internal/cmd/config.go          # EDIT — exampleConfigTemplate config_version block: 2 -> 3 (header prose + example line)
internal/config/config.go       # READ-ONLY — CurrentConfigVersion = 3 (authoritative; the value the header must match)
internal/cmd/config_test.go     # READ-ONLY — full-string equality test (stays green; documents the safety contract)
# README.md, docs/README.md     -> owned by P1.M7.T1.S1 (parallel) — DO NOT TOUCH
# docs/{cli,configuration,how-it-works,providers}.md -> Mode-A docs (M1–M6 / P1.M6.T2.S1) — DO NOT TOUCH
# PRD.md, FUTURE_SPEC.md, tasks.json, prd_snapshot.md -> read-only / orchestrator-owned
```

### Desired Codebase tree with files to be edited

```bash
internal/cmd/config.go          # EDIT only — two `2`->`3` replacements inside the exampleConfigTemplate string constant
# (no new files; no other files modified)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (one file, two digits): this task edits ONLY internal/cmd/config.go, and ONLY the two
     `config_version` mentions inside the exampleConfigTemplate constant (header prose + commented
     example line). Nothing else. If you are editing a second file, you are out of scope. -->

<!-- CRITICAL (do NOT "fix" the CORRECT `= 2` occurrences): the version-regex comment at
     internal/cmd/config.go:122 (teaches why commented lines are ignored) and the v2 FIXTURE inputs in
     *_test.go (default_action_test.go:1204, config_test.go:1167+, file_test.go:548/669) legitimately
     contain `config_version = 2`. They are NOT stale — changing them deletes migration test coverage.
     Only the exampleConfigTemplate header+example-line are stale. -->

<!-- CRITICAL (the equality test is the gate): config_test.go:438 compares the --template output to
     exampleConfigTemplate by FULL-STRING equality. Editing the constant keeps both sides equal, so the
     test stays green. If it fails, you changed more than the two digits — re-read the constant. -->

<!-- CRITICAL (scope fences vs siblings): README.md + docs/README.md are P1.M7.T1.S1's (running in
     parallel). docs/*.md Mode-A pages belong to M1–M6 / P1.M6.T2.S1. PRD.md / FUTURE_SPEC.md /
     tasks.json are read-only. This task touches exactly one constant in one .go file. -->

<!-- CRITICAL (record clean results, don't just fix one): the deliverable is evidence the SWEEP was
     done, not only the one known fix. The commit message must state, per pattern, that it was grepped
     and is clean (citing where the CORRECT occurrences live), so a reviewer can see the verification. -->
```

## Implementation Blueprint

### Data models and structure

No data models. The deliverable is a two-digit edit to a Go string constant, plus a documented grep sweep.
The `exampleConfigTemplate` constant's `config_version` block (current → desired):

```
# schema the file was written for. This binary supports config_version = 2.   →   ... = 3.
# ---------------------------------------------------------------------------
# config_version = 2                                                          →   # config_version = 3
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/cmd/config.go — fix the exampleConfigTemplate config_version block (the confirmed stale string)
  - FIND: the `const exampleConfigTemplate = ...` string (begins ~L497). Its `config_version` block sits
    ~L524–L529.
  - REPLACE (both, in ONE edit call against the original file — two non-overlapping replacements):
      (a) header prose:  "This binary supports config_version = 2."  ->  "This binary supports config_version = 3."
      (b) example line:  "# config_version = 2"                       ->  "# config_version = 3"
  - VERIFY the value matches `config.CurrentConfigVersion` (internal/config/config.go:11 == 3).
  - PRESERVE: every other character of the constant (whitespace, the surrounding comment block, the
    `# On load, if this is missing/older...` paragraph, the remediation hints). This is a 2-digit change.
  - DO NOT TOUCH: the regex-anchoring comment at L122 (uses `# config_version = 2` to teach the regex —
    CORRECT); the v2 test fixtures in *_test.go (CORRECT migration inputs).

Task 2: GREP-VERIFY the five patterns and record a clean-result-per-pattern (the sweep deliverable)
  Run the sweep from repo root (commands in Validation Loop Level 2). For EACH pattern, capture the result:
    Pattern 1 — pre-v3 terminology (`--planner-agent` flag, `[agent.*]` blocks as CURRENT):
       EXPECTED CLEAN. `--planner-agent` appears only as a role description in planner.go:31 (not a flag).
       `[agent.*]` appears only in migration code + v2 test fixtures (not a current v3 config shape).
    Pattern 2 — `config_version = 2`:
       EXPECTED CLEAN after Task 1 (within exampleConfigTemplate). The remaining `= 2` hits are the
       regex comment (L122) and v2 test fixtures — CORRECT.
    Pattern 3 — superseded non-goal phrasings ("no hook installer", "--edit is deferred/v1.1"):
       EXPECTED CLEAN in docs/README/user-facing strings. (PRD.md carries them correctly struck-through;
       hookexec.go:57 has a CORRECT runtime error: "--edit is not valid with hook exec".)
    Pattern 4 — COMPETITOR-ANALYSIS.md as an existing repo file:
       EXPECTED CLEAN in docs/README/user-facing strings. (Referenced only in read-only PRD.md +
       FUTURE_SPEC.md as the historical planning evidence base — out of scope to modify.)
  RECORD in the commit message, per pattern: grepped <command>, result = clean (or fixed), with the
  CORRECT occurrences explicitly named so a reviewer sees they were not missed.

Task 3: VALIDATE — build, test, lint, and the post-fix grep gate
  - `go build ./...`                         (compiles — the constant is a string; must stay valid Go)
  - `go test ./internal/cmd/... ./internal/config/...`  (the --template equality test + migration tests green)
  - `go vet ./internal/cmd/...` and `golangci-lint run` (if available) — no new issues from the edit
  - Post-fix grep gate: confirm exampleConfigTemplate now contains `supports config_version = 3` and
    `# config_version = 3`, and that NO `config_version = 2` survives inside the constant body.
```

### Implementation Patterns & Key Details

```go
// The exampleConfigTemplate config_version block AFTER the fix (matches config.CurrentConfigVersion = 3):
//
// # config_version — schema version (PRD §9.17 FR-B4). Top-level metadata, NOT a [defaults] key and
// # NOT a precedence layer (§16.1): it never overrides another field; it only tells stagecoach which
// # schema the file was written for. This binary supports config_version = 3.
// # ---------------------------------------------------------------------------
// # config_version = 3
//
// BOTH the header prose and the commented example line move 2 -> 3. That is the entire code change.

// SAFETY: the constant is consumed by `config init --template` (internal/cmd/config.go ~L443:
// `content = exampleConfigTemplate`). config_test.go:438 asserts the written file EQUALS the constant,
// so editing the constant keeps the equality test green. No substring test pins "= 2".
```

### Integration Points

```yaml
internal/cmd/config.go (exampleConfigTemplate constant, ~L497–end):
  - edit: header prose  "supports config_version = 2."  ->  "... = 3."   (Task 1a)
  - edit: example line  "# config_version = 2"          ->  "# config_version = 3"  (Task 1b)

NOT touched (scope fences):
  - README.md, docs/README.md                -> P1.M7.T1.S1 (parallel sibling)
  - docs/{cli,configuration,how-it-works,providers}.md -> Mode-A docs (M1–M6 / P1.M6.T2.S1)
  - PRD.md, FUTURE_SPEC.md, tasks.json, prd_snapshot.md -> read-only / orchestrator-owned
  - internal/cmd/config.go:122 (regex comment), *_test.go v2 fixtures -> CORRECT, not stale
```

## Validation Loop

### Level 1: Build & Compile (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
go build ./...
# Expected: exit 0. The constant is a Go string; the two-digit edit cannot break compilation unless
# whitespace/punctuation was accidentally altered. If it fails to compile, re-read the constant.

go vet ./internal/cmd/...
# Expected: no new diagnostics. (Optional: `golangci-lint run` if the linter is installed.)
```

### Level 2: Unit Tests + Post-Fix Grep Gate (the PRIMARY gates)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity

# (A) Tests governing the edit's safety. The --template equality test (config_test.go:438) compares the
#     written file to exampleConfigTemplate by full-string equality — editing the constant keeps it green.
#     The migration tests feed v2 fixtures and assert upgrade to 3 — unaffected by the template edit.
go test ./internal/cmd/... ./internal/config/...
# Expected: PASS. If config_test.go:438 fails, you changed the constant body beyond the two digits —
# re-read it and fix the divergence (whitespace, a stray character, an altered comment).

# (B) Post-fix grep gate — the template now reads v3 in BOTH spots, and NO "= 2" survives in its body.
echo "=== template header must show = 3 (expect 1) ==="
grep -c 'supports config_version = 3' internal/cmd/config.go
echo "=== template example line must be = 3 (expect >=1) ==="
grep -c '^# config_version = 3' internal/cmd/config.go
echo "=== exampleConfigTemplate body must contain NO surviving '= 2' (expect 0) ==="
awk '/const exampleConfigTemplate = `/,/^`$/' internal/cmd/config.go | grep -c 'config_version = 2'
# Expected: header=1, example line>=1, surviving '= 2' inside the constant body = 0.

# (C) THE SWEEP — five patterns, scoped to docs/ + README + user-facing .go strings. Record each result.
echo "=== Pattern 1: pre-v3 terminology as CURRENT (expect CLEAN in user-facing) ==="
echo "--planner-agent as a flag:"; grep -rn -- '--planner-agent' --include='*.go' --include='*.md' . | grep -v '/plan/' | grep -v '.git/' || echo "  CLEAN"
echo "[agent.*] config blocks presented as current v3 shape (excludes migration code + v2 fixtures):"
grep -rn '\[agent\.' --include='*.go' --include='*.md' . | grep -v '/plan/' | grep -v '.git/' | grep -viE 'rewriteV2|migration|upgrade|test|fixture|abandoned' || echo "  CLEAN (all hits are migration code / comments / v2 fixtures)"

echo "=== Pattern 2: 'config_version = 2' AFTER the fix (CORRECT hits only) ==="
grep -rn 'config_version = 2' --include='*.go' --include='*.md' --include='*.toml' . | grep -v '/plan/' | grep -v '.git/'
# Expected: ONLY (a) config.go:122 regex-teaching comment, (b) *_test.go v2 fixture inputs. NO exampleConfigTemplate hits.
echo "  (If the only hits are config.go:122 + *_test.go fixtures, Pattern 2 is CLEAN.)"

echo "=== Pattern 3: superseded non-goal phrasings as CURRENT (expect CLEAN in docs/README/user-facing) ==="
grep -rni 'no hook installer\|hook installer' docs/ README.md || echo "  CLEAN (docs/README)"
grep -rni 'edit.*deferred\|deferred.*edit\|--edit.*v1\.1\|edit.*v1\.1' docs/ README.md || echo "  CLEAN (docs/README)"
echo "  NOTE: PRD.md carries these correctly struck-through (read-only, out of scope); hookexec.go:57's"
echo "        '--edit is not valid with hook exec' is a CORRECT runtime error (FR-E4), not a stale claim."

echo "=== Pattern 4: COMPETITOR-ANALYSIS.md as an existing repo file (expect CLEAN in docs/README/user-facing) ==="
grep -rni 'COMPETITOR-ANALYSIS' docs/ README.md || echo "  CLEAN (docs/README)"
test -f COMPETITOR-ANALYSIS.md && echo "  REVIEW: file now exists (unexpected)" || echo "  CONFIRMED: COMPETITOR-ANALYSIS.md does not exist"
echo "  NOTE: referenced only in read-only PRD.md + FUTURE_SPEC.md as historical planning evidence (out of scope)."

echo "=== Scope fence — git status shows ONLY internal/cmd/config.go modified ==="
git status --porcelain
# Expected: exactly one modified file: internal/cmd/config.go. (README.md/docs/README.md may show as
# modified IF P1.M7.T1.S1 landed in the same working tree — that is S1's change, not yours.)
```

### Level 3: Behavior Check (the generated-template gate)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
# Confirm the user-facing artifact actually carries v3 after the edit (config init --template writes the constant).
tmp=$(mktemp -d)
STAGECOACH_CONFIG="$tmp/c.toml" go run ./cmd/stagecoach config init --template --force >/dev/null 2>&1 || true
echo "=== generated --template config_version block ==="
grep -A1 -B1 'config_version' "$tmp/c.toml" 2>/dev/null | head
rm -rf "$tmp"
# Expected: the generated file's config_version block reads 3 (header prose + example line), proving the
# user-facing artifact is fixed, not just the source constant.
# (If `go run config init --template` needs flags the harness doesn't supply, fall back to the Level-2
#  grep gate on the constant body — that is sufficient proof the artifact is correct.)
```

### Level 4: Documentation-Coherence Sweep (the clean-result-per-pattern record)

```bash
cd /home/dustin/projects/stagecoach-competitor-feature-parity
# The deliverable is EVIDENCE the sweep was done. Capture each pattern's clean result for the commit message.
cat <<'EOF'
>>> Record these per-pattern results in the commit message (copy the CLEAN lines verbatim) <<<
Pattern 1 (pre-v3 terminology): grepped --planner-agent + [agent.*]; CLEAN in user-facing strings
  (planner.go:31 describes the planner ROLE, not the abandoned flag; [agent.*] appears only in the
   v2->v3 migration code rewriteV2ToV3 and v2 test fixtures, never as a current v3 config shape).
Pattern 2 (config_version = 2): FIXED in exampleConfigTemplate (header + example line -> 3);
  the only remaining = 2 hits are config.go:122 (regex-teaching comment) and *_test.go v2 fixtures — CORRECT.
Pattern 3 (superseded non-goals): CLEAN in docs/README/user-facing ("hook installer"/"--edit deferred"
  appear only in read-only PRD.md, correctly struck-through; hookexec.go:57 is a CORRECT runtime error).
Pattern 4 (COMPETITOR-ANALYSIS.md): CLEAN in docs/README/user-facing; file does not exist; referenced
  only in read-only PRD.md + FUTURE_SPEC.md as historical planning evidence (out of scope).
EOF
# Expected: implementer pastes the four per-pattern clean results into the commit message body.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `go build ./...` compiles; `go vet ./internal/cmd/...` clean.
- [ ] Level 2 (A): `go test ./internal/cmd/... ./internal/config/...` passes (incl. the `--template`
      full-string-equality test at config_test.go:438).
- [ ] Level 2 (B): `exampleConfigTemplate` header reads `supports config_version = 3`; example line reads
      `# config_version = 3`; ZERO `config_version = 2` survives inside the constant body.
- [ ] Level 2 (C): all five sweep patterns recorded clean (or fixed) in the commit message.

### Feature Validation

- [ ] The one CONFIRMED stale string is corrected (header prose + example line → 3).
- [ ] No stale pre-v3 terminology (`--planner-agent`, `[agent.*]` as current) in user-facing strings.
- [ ] No "no hook installer" / "--edit is deferred/v1.1" presented as current in docs/README/user-facing.
- [ ] No reference presents `COMPETITOR-ANALYSIS.md` as an existing repo file in docs/README/user-facing.
- [ ] The CORRECT `config_version = 2` occurrences (regex comment L122, v2 test fixtures) are untouched.

### Code Quality Validation

- [ ] Only `internal/cmd/config.go` is modified (git status confirms — modulo S1's parallel README work).
- [ ] The edit is exactly two `2`→`3` digits inside one string constant — no whitespace/comment drift.
- [ ] Scope fences respected: README.md / docs/README.md (S1), docs/*.md Mode-A (M1–M6 / P1.M6.T2.S1),
      PRD.md / FUTURE_SPEC.md / tasks.json (read-only) all untouched.

### Documentation & Deployment

- [ ] Commit message records, per pattern, the grep command run and its clean result (with the CORRECT
      occurrences explicitly named) — so the sweep is auditable, not just the single fix.
- [ ] The generated `config init --template` artifact (or, failing that, the constant body) reads v3.

---

## Anti-Patterns to Avoid

- ❌ Don't edit any file other than `internal/cmd/config.go` (README/docs/README is S1's; docs/*.md Mode-A
  is M1–M6 / P1.M6.T2.S1's; PRD.md/FUTURE_SPEC.md/tasks.json are read-only).
- ❌ Don't "fix" the CORRECT `config_version = 2` occurrences — the regex comment at config.go:122 and the
  v2 fixtures in `*_test.go` are intentional (teaching the regex / testing v2→v3 migration). Changing them
  deletes test coverage.
- ❌ Don't change more than the two digits — the `--template` equality test (config_test.go:438) compares
  the whole constant; any whitespace/comment drift fails it. If it fails, re-read the constant.
- ❌ Don't skip the sweep and only fix the one known string — the deliverable is a per-pattern clean-result
  RECORD. Grep all five patterns and state each result in the commit message.
- ❌ Don't touch model names in the template's `[role.*]` examples (`gemini-2.5-pro` etc.) — they are
  illustrative in the inert `--template` reference; live model currency is the manifests' job (M2 / FR-D5),
  not this stale-string sweep.
- ❌ Don't modify PRD.md or FUTURE_SPEC.md to soften the `COMPETITOR-ANALYSIS.md` mentions — they are
  read-only, and the mentions are historical planning context, not claims the file exists for a visitor.
