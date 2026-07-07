# P2.M1.T3.S1 Research Findings — Stager `files` block + guardrails wording

Verified by reading the live tree (not assumed). All byte content below is exact.

## §1. Current state (verified)

### 1.1 `internal/prompt/stager.go` (FULL file is ~63 lines)

- **Package**: `prompt`. **NO imports today** (just `package prompt` + comments + consts + the func).
- **`stagerInstruction` const (line 20)** — UNCHANGED by this task:
  ```go
  const stagerInstruction = "Stage, but do NOT commit, all changes in this repository that match this concept:"
  ```
- **`stagerGuardrails` const (lines 38–42)** — the SECOND SENTENCE changes wording; everything else byte-identical:
  ```go
  const stagerGuardrails = "Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply\n" +
  	"only the relevant hunks via `git apply --cached`). Stage ONLY changes belonging to this\n" +
  	"concept; leave unrelated changes unstaged. Do not commit, do not amend, do not push, do not\n" +
  	"modify file contents — only update the index. When done, reply with the list of paths you\n" +
  	"staged and stop."
  ```
  - The line comment at stager.go:34 already flags: `NOTE the EM-DASH "—" (U+2014) in "file contents — only update the index"` and `NOTE the TWO BACKTICK chars`. Both survive the wording change.
- **`BuildStagerTask` (lines 60–62)** — current 2-arg signature + topology:
  ```go
  func BuildStagerTask(title, description string) string {
  	return stagerInstruction + "\n\n" + title + "\n" + description + "\n\n" + stagerGuardrails
  }
  ```
- **Doc comment (lines 37–58)** documents the topology and the call site `prompt.BuildStagerTask(concept.Title, concept.Description)` at line 41 — the doc-comment example must be updated to the 3-arg form (it's a comment; not compiled, but stale docs are scope-creep risk for a reviewer).

### 1.2 `internal/decompose/stager.go:99` — the SOLE production call site

```go
task := prompt.BuildStagerTask(concept.Title, concept.Description)
```
This is the **only** `BuildStagerTask` call outside tests (confirmed by `grep -rn BuildStagerTask --include=*.go | grep -v /plan/`). It becomes:
```go
task := prompt.BuildStagerTask(concept.Title, concept.Description, concept.Files)
```

### 1.3 The test seam is TRANSPARENT — NO seam change needed

`internal/decompose/roles.go:74`:
```go
stager func(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error
```
Takes the **whole** `prompt.PlannerCommit` — `concept.Files` flows through it untouched. The `BuildStagerTask` signature change is invisible to this seam and to every orchestrator test that injects a fake stager. **No change to roles.go, roles_test.go, or any decompose test.**

### 1.4 `prompt.PlannerCommit.Files` ALREADY EXISTS (P2.M1.T1.S1 — COMPLETE)

`internal/prompt/planner.go:83–87`:
```go
type PlannerCommit struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Files       []string `json:"files"` // FR-M3: every path this concept touches; guidance, not a constraint
}
```
**This task consumes `concept.Files` — it does NOT add it.** `concept.Files` may be `nil` (planner omitted the key or emitted `"files":null`) or `[]string{}` — both MUST omit the files block (see §2.2).

### 1.5 `internal/prompt/stager_test.go` — current test structure (FULL file read)

Seven `BuildStagerTask(...)` call sites, ALL 2-arg today → ALL need a 3rd arg:
- Line 27 `BuildStagerTask(title, description)` — CanonicalExact
- Line 39 `BuildStagerTask(title, desc)` — Properties (`p`)
- Line 120 `BuildStagerTask(weirdTitle, "desc")` — title-verbatim sub-test
- Line 129 `BuildStagerTask("title", multiDesc)` — multi-line desc sub-test
- Line 159 `BuildStagerTask("", "desc")` — empty-title edge
- Line 174 `BuildStagerTask("title", "")` — empty-desc edge
- Line 189 `BuildStagerTask("", "")` — both-empty edge

**Two property-table NEEDLES break due to the guardrails line re-wrapping (CRITICAL — see §3):**
- Line ~64: `{"guardrails: Stage ONLY clause present", "Stage ONLY changes belonging to this\nconcept", true}` — wording + the `\n` position BOTH change.
- Line ~71: `{"guardrails: 'reply-with-paths' instruction present", "reply with the list of paths you\nstaged and stop", true}` — the `\n` DISAPPEARS in the target (line 5 becomes one line).

The `near` helper (used in the test) is defined in `internal/prompt/system_test.go:507` — shared across the package's white-box tests; unchanged.

### 1.6 Doc-comment call-site example

`internal/prompt/stager.go:41` (inside the `BuildStagerTask` doc comment):
```go
//	task := prompt.BuildStagerTask(concept.Title, concept.Description)
```
Stale after the signature change. Update to the 3-arg form for doc/code consistency (cosmetic; not compiled).

---

## §2. Target state (PRD §17.6 verbatim)

### 2.1 New `stagerGuardrails` const (concatenated string — PRD §17.6 line 1826)

The ONLY change is the **second sentence**. Old → New:
- OLD: `Stage ONLY changes belonging to this concept; leave unrelated changes unstaged.`
- NEW: `Stage ONLY the changes the description assigns to this concept (the files above are where they live); leave everything else unstaged.`

Target concatenated string (5 lines; the em-dash and both backtick commands are byte-identical to today):
```
Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply
only the relevant hunks via `git apply --cached`). Stage ONLY the changes the description
assigns to this concept (the files above are where they live); leave everything else unstaged.
Do not commit, do not amend, do not push, do not modify file contents — only update the index.
When done, reply with the list of paths you staged and stop.
```

Source form (mirroring today's ~100-char-per-source-line wrapping; double-quoted `+`-concatenated because of the backticks):
```go
const stagerGuardrails = "Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply\n" +
	"only the relevant hunks via `git apply --cached`). Stage ONLY the changes the description\n" +
	"assigns to this concept (the files above are where they live); leave everything else unstaged.\n" +
	"Do not commit, do not amend, do not push, do not modify file contents — only update the index.\n" +
	"When done, reply with the list of paths you staged and stop."
```

### 2.2 New `BuildStagerTask` signature + topology (PRD §17.6 sketch)

Signature: `func BuildStagerTask(title, description string, files []string) string`

Topology (the files block is **conditionally inserted** between description and guardrails; OMITTED ENTIRELY — no blank-line artifact — when `len(files)==0`):
```
stagerInstruction                                  // "...match this concept:" (no trailing \n)
"\n\n"                                             // ONE blank line
title
"\n"                                               // title + description CONSECUTIVE (no blank between)
description
[ IF len(files) > 0:
    "\n\n"                                         // ONE blank line
    "Files for this concept (where these changes live):\n"
    strings.Join(files, "\n")                      // one path per line
]
"\n\n"                                             // ONE blank line before guardrails
stagerGuardrails                                   // (no trailing \n)
```

Implementation (needs `"strings"` imported — stager.go has NO imports today, so add an import block):
```go
import "strings"

func BuildStagerTask(title, description string, files []string) string {
	out := stagerInstruction + "\n\n" + title + "\n" + description
	if len(files) > 0 {
		out += "\n\n" + "Files for this concept (where these changes live):\n" + strings.Join(files, "\n")
	}
	return out + "\n\n" + stagerGuardrails
}
```

**`len(files) > 0` covers BOTH `nil` and `[]string{}`** — both omit the block. No blank-line artifact: the `"\n\n"` before the block and the block itself are inside the `if`, so when omitted, `out` jumps straight from `description` to the `"\n\n"` + guardrails (identical to today's no-files rendering).

### 2.3 Call-site update (`internal/decompose/stager.go:99`)

```go
task := prompt.BuildStagerTask(concept.Title, concept.Description, concept.Files)
```

---

## §3. CRITICAL: test-needle breaks from the guardrails re-wrapping

The wording change re-wraps the guardrails block. Two existing needles in `TestBuildStagerTask_Properties` break and MUST be updated (a missed one = compile-stale-but-test-fail):

### Needle 1 — the "Stage ONLY clause" (line ~64)

- OLD needle (matches today's wrapping where `\n` is between "this" and "concept"):
  `"Stage ONLY changes belonging to this\nconcept"`
- NEW needle (target wraps `\n` between "description" and "assigns"; "this concept" is now contiguous on line 3):
  `"Stage ONLY the changes the description\nassigns to this concept (the files above are where they live)"`

### Needle 2 — the "reply-with-paths" instruction (line ~71) — ⚠️ NOT obvious

- OLD needle (today line 4 ends "...paths you" and line 5 is "staged and stop."):
  `"reply with the list of paths you\nstaged and stop"`
- NEW needle: in the TARGET, line 5 is the WHOLE sentence `When done, reply with the list of paths you staged and stop.` — there is NO `\n` between "you" and "staged" anymore. The needle MUST drop the `\n`:
  `"reply with the list of paths you staged and stop"`
- **If this needle is not updated, `TestBuildStagerTask_Properties` FAILS** even though the wording is correct, because `strings.Contains` won't find the old `\n`-spanning substring.

### Needles that SURVIVE unchanged (verify, don't touch)

- `"Use git to stage the relevant files and hunks"` — line 1 unchanged. ✅
- `"`git add <path>`"` — line 1 unchanged. ✅
- `"`git apply --cached`"` — line 2 unchanged. ✅
- `"<path>"` — line 1 unchanged. ✅
- `"Do not commit, do not amend, do not push"` — line 4, contiguous in both old and new. ✅
- `"only update the index"` — line 4/5, contiguous in both. ✅
- Em-dash check `"contents — only"` — unchanged (the em-dash byte is in the same phrase). ✅
- All anti-copy-paste ABSENT assertions (§17.1/§17.5 elements) — unaffected. ✅

---

## §4. New / updated test cases (architecture brief §3.2)

### 4.1 `TestBuildStagerTask_CanonicalExact` — rewrite `want`; TWO cases

**Case A — files present** (`files = []string{"a.go", "b.go"}`): `want` includes the files block.
```
Stage, but do NOT commit, all changes in this repository that match this concept:

Refactor auth middleware
Stage internal/auth/middleware.go and its callers in internal/api/.

Files for this concept (where these changes live):
a.go
b.go

Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply
only the relevant hunks via `git apply --cached`). Stage ONLY the changes the description
assigns to this concept (the files above are where they live); leave everything else unstaged.
Do not commit, do not amend, do not push, do not modify file contents — only update the index.
When done, reply with the list of paths you staged and stop.
```

**Case B — files absent** (`files = nil`): `want` is byte-identity to Case A **MINUS** the files block (no `Files for this concept` line, no blank-line artifact). The description is followed directly by `\n\n` + guardrails. This proves the omit-when-empty topology.

### 4.2 NEW `TestBuildStagerTask_FilesBlock_OmittedWhenEmpty`

Both `BuildStagerTask(t, d, nil)` and `BuildStagerTask(t, d, []string{})` MUST NOT contain the substring `"Files for this concept"`. (Proves `nil` AND empty-slice both omit cleanly.)

### 4.3 `TestBuildStagerTask_Properties` — update needles + add files-block assertions

- Call `BuildStagerTask(title, desc, []string{"a.go", "b.go"})` (files present, so we can assert the block).
- **Replace** Needle 1 (the "Stage ONLY clause") per §3.
- **Replace** Needle 2 (the "reply-with-paths") per §3.
- **ADD** a PRESENT assertion: `"Files for this concept (where these changes live):"` PRESENT.
- **ADD** a PRESENT assertion: the per-path rendering `"a.go\nb.go"` PRESENT (joined by `\n`).
- **UPDATE** the "blank-line topology: one blank line after description" sub-test: with files present, description is followed by `\n\n` + the files block, NOT directly by guardrails. Change the assertion from `strings.Contains(p, desc+"\n\n"+stagerGuardrails)` to `strings.Contains(p, desc+"\n\n"+"Files for this concept (where these changes live):")`. (The no-files topology is already pinned by CanonicalExact Case B.)
- KEEP the em-dash check, both backtick-command needles, the `<path>` token, the title-before-description ordering, the title-verbatim and multi-line-description sub-tests (all still valid — just add the 3rd `nil`/`files` arg to those `BuildStagerTask` calls).

### 4.4 `TestBuildStagerTask_EdgeCases` — add 3rd arg (nil) to all 3 sub-tests

`BuildStagerTask("", "desc")` → `BuildStagerTask("", "desc", nil)`, etc. The assertions (starts with instruction, contains guardrails) are unchanged.

---

## §5. Scope fence (what NOT to touch)

- **`internal/prompt/planner.go`** — P2.M1.T1.S1 (Files field — COMPLETE) and P2.M1.T2.S1 (mode-conditional prompt — COMPLETE/parallel). Do NOT touch.
- **`internal/decompose/planner.go`, `internal/decompose/decompose.go`** — P2.M1.T2.S1 owns the `BuildPlannerSystemPrompt` call-site; the FR-M3b coverage check is P2.M1.T1.S2 (COMPLETE). Do NOT touch.
- **`internal/decompose/roles.go`, `roles_test.go`, `stager_test.go` (decompose)** — the seam is transparent; NO change.
- **`docs/how-it-works.md`, `docs/cli.md`, `docs/configuration.md`** — P2.M1.T2.S2 (Mode A doc edit — parallel) owns the doc narrative. The stager files block is internal prompt construction (no user-facing surface; item §5 DOCS: none). Do NOT touch any doc.
- **`PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`** — read-only.
- **Files that change = EXACTLY 3**: `internal/prompt/stager.go`, `internal/prompt/stager_test.go`, `internal/decompose/stager.go`.

---

## §6. Validation commands (verified convention from sibling PRPs)

```bash
cd /home/dustin/projects/stagecoach
gofmt -l internal/prompt/stager.go internal/prompt/stager_test.go internal/decompose/stager.go   # expect EMPTY
go build ./...                              # expect success
go vet ./internal/prompt/ ./internal/decompose/   # expect clean
go test -race ./internal/prompt/ -run "TestBuildStagerTask" -v   # the rewritten + new tests
go test ./...                               # FULL regression
git status --short                          # expect EXACTLY 3 files
```
