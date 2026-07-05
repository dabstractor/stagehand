---
name: "P2.M1.T1.S1 — Add Files []string to PlannerCommit + parse round-trip tests (PRD §9.14 FR-M3 / §17.5)"
description: |

  Implement FR-M3's data-model half: add `Files []string \`json:"files"\`` to the `PlannerCommit` struct
  in `internal/prompt/planner.go` so `ParsePlannerOutput` (generic `json.Unmarshal`) populates it FOR
  FREE — each concept carries the paths it touches, as guidance for the stager. Then update the two
  parse tests in `internal/prompt/planner_test.go` (the table + the round-trip) to exercise Files.

  This is a LEAF data-model change: ONE field + TWO test functions. No parse-code change (json.Unmarshal
  populates the field automatically). No validation change (`validatePlannerOutput` — in a DIFFERENT
  package — must NOT enforce non-empty Files; files is guidance, FR-M1c is the sole content guarantee).

  CONTRACT (P2.M1.T1.S1, verbatim):
    1. RESEARCH: PlannerCommit (prompt/planner.go:57-61) has Title+Description, NO Files. ParsePlannerOutput
       (~171) uses json.Unmarshal generically — adding the field populates it FOR FREE (no parse-code change).
       validatePlannerOutput (~152) must NOT enforce non-empty Files (files is guidance).
    2. INPUT: None (leaf data-model change). PlannerCommit is defined in internal/prompt/planner.go.
    3. LOGIC: Add `Files []string \`json:"files"\`` to PlannerCommit with doc comment 'every path this
       concept touches; guidance, not a constraint (FR-M1c is the content guarantee)'. Do NOT touch
       validatePlannerOutput. TDD: update planner_test.go TestParsePlannerOutput table cases to add Files
       to the round-trip cases; add a "files":null case (→ nil) and a concept-missing-files-key case
       (→ nil); update TestParsePlannerOutput_RoundTrip to include Files in the original.
    4. OUTPUT: PlannerCommit.Files populated by ParsePlannerOutput for consumption by P2.M1.T1.S2 (coverage
       check), P2.M1.T2.S1 (JSON contract references files), and P2.M1.T3.S1 (stager files block).
    5. DOCS: none — internal data-model field, no user-facing surface.

  ⚠️ §1 — THE COMPILE-BREAK GOTCHA (load-bearing). planner_test.go:440 does `out.Commits[i] != original.
  Commits[i]` — a DIRECT struct comparison. TODAY PlannerCommit has only string fields (comparable).
  Adding `Files []string` makes the struct NON-COMPARABLE (Go forbids == / != on structs containing
  slices) ⇒ that line WILL NOT COMPILE. You MUST rewrite it to `!reflect.DeepEqual(...)` AND add
  `"reflect"` to planner_test.go's imports. Verified: this is the ONLY direct PlannerCommit struct
  comparison in the repo (other test hits compare .Subject/.SHA strings on CommitResult).

  ⚠️ §2 — ParsePlannerOutput needs NO change. It is a generic json.Unmarshal; the new field is populated
  automatically. Do NOT edit ParsePlannerOutput, extractJSONObject, BuildPlannerSystemPrompt,
  BuildPlannerUserPayload, or any const. This task is the struct field + the two parse tests. Period.

  SCOPE BOUNDARY (frozen / owned by sibling subtasks — do NOT edit):
    - `plannerSystemPrompt` const + mode-conditional rules + soft target → P2.M1.T2.S1 (T2).
    - `BuildStagerTask` files param + guardrails wording → P2.M1.T3.S1 (T3).
    - FR-M3b coverage check (logs unclaimed paths) → P2.M1.T1.S2 (S2).
    - `validatePlannerOutput` (internal/decompose/planner.go:152) → different package; not touched.
    - `docs/how-it-works.md` → T2 ride-along.
    - internal/decompose/*_test.go → PARALLEL P1.M1.T3.S2.

  DELIVERABLES (0 NEW files, 2 EDITED files):
    EDIT internal/prompt/planner.go       — add `Files []string` to PlannerCommit (+ doc comment).
    EDIT internal/prompt/planner_test.go  — TestParsePlannerOutput table + comparison loop; +
                                            TestParsePlannerOutput_RoundTrip (Files + reflect.DeepEqual);
                                            + "reflect" import.

  SUCCESS: ParsePlannerOutput populates PlannerCommit.Files for populated input, nil for "files":null and
  for a missing files key; the round-trip test survives (reflect.DeepEqual); `go build/vet/test ./...` green;
  go.mod/go.sum unchanged; EXACTLY 2 files changed.

---

## Goal

**Feature Goal**: Implement the FR-M3 data-model half — add a `Files []string` field to `PlannerCommit`
so each partitioned concept carries the list of paths it touches (guidance for the stager / coverage
check), populated automatically by the existing generic JSON parse. This is the foundation that S2
(coverage check), T2 (JSON contract references `files`), and T3 (stager files block) consume.

**Deliverable** (0 NEW + 2 EDITED):
1. **EDIT `internal/prompt/planner.go`** — add `Files []string \`json:"files"\`` to the `PlannerCommit`
   struct with the contract-mandated doc comment.
2. **EDIT `internal/prompt/planner_test.go`** — update `TestParsePlannerOutput` (add Files to the
   round-trip table cases; add a `"files":null` case and a missing-files-key case; extend the comparison
   loop to assert `c.Files`); update `TestParsePlannerOutput_RoundTrip` (add `Files` to the original;
   rewrite the `!=` struct comparison to `!reflect.DeepEqual`; add the `"reflect"` import).

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `ParsePlannerOutput`
populates `PlannerCommit.Files` for `"files":["a.go","b.go"]`, leaves it nil for `"files":null` and for a
missing files key; the round-trip test compiles (reflect.DeepEqual) and passes; `validatePlannerOutput` is
untouched; go.mod/go.sum byte-unchanged; EXACTLY the 2 listed files change.

## User Persona

**Target User**: the implementing agents for S2/T2/T3 (the downstream consumers of `PlannerCommit.Files`).
This is an internal data-model field with no user-facing surface.

**Use Case**: S2's coverage check unions `concept.Files` across concepts; T3's `BuildStagerTask` renders
the files block from `concept.Files`; T2's JSON contract documents the `files` key. All three read the
field this task adds.

**Pain Points Addressed**: today the planner JSON carries no per-concept file list, so the stager has no
guidance on where to look and the coverage check has nothing to union. Adding the field (populated for
free by the generic parse) unblocks all three downstream tasks without any parse-code change.

## Why

- **Closes the FR-M3 data-model half.** FR-M3 (PRD §9.14) specifies the planner output contract as
  `commits:[{title, description, files}]`. The struct must carry `Files` for the contract to be typed.
- **Zero parse-code change.** `ParsePlannerOutput` is a generic `json.Unmarshal`; the field is populated
  automatically. This is the lowest-risk way to land FR-M3's data model.
- **Foundation for S2/T2/T3.** The coverage check (S2), the JSON-contract doc (T2), and the stager files
  block (T3) all read `concept.Files`. Landing the field first (this task) lets those siblings proceed.
- **Files is guidance, not a constraint.** FR-M1c (freeze-subset verification) is the SOLE content
  guarantee; `Files` only tells the stager where to look. So `validatePlannerOutput` must NOT enforce
  non-empty Files — and (different package) is not touched by this task anyway.

## What

A one-field struct addition + two test-function updates. No parse-code change, no validation change, no
new types, no interface change, no production-code import change.

### Success Criteria

- [ ] `PlannerCommit` in `internal/prompt/planner.go` has `Files []string \`json:"files"\`` with the doc
      comment "FR-M3: every path this concept touches; guidance, not a constraint (FR-M1c is the content guarantee)."
- [ ] `ParsePlannerOutput` is UNCHANGED (no edit) — it populates Files via the existing generic json.Unmarshal.
- [ ] `TestParsePlannerOutput` table has Files in the round-trip cases; a `"files":null` case (→ nil); a
      concept-missing-files-key case (→ nil); and the comparison loop asserts `c.Files` via reflect.DeepEqual.
- [ ] `TestParsePlannerOutput_RoundTrip` includes `Files: []string{...}` on a commit in `original` and
      compiles (the `!=` rewritten to `!reflect.DeepEqual`); `"reflect"` is imported.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; go.mod/go.sum byte-unchanged; EXACTLY 2 files.
- [ ] `validatePlannerOutput` (internal/decompose/planner.go) is UNCHANGED; `plannerSystemPrompt` const,
      `BuildPlannerSystemPrompt`, `BuildStagerTask`, all docs UNCHANGED.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact field + doc
comment (§3 of findings — copy-ready), the compile-break gotcha + its fix (§1/§2 — the single sharp
edge), the exact test changes (§4 — which const to edit, which cases to add, the comparison-loop +
round-trip rewrites), the json.Marshal/Unmarshal []string behavior (§5), and the scope fence (§0 — what
NOT to touch). No decompose/git/provider knowledge required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE scout brief (§1.1 current state, §2.1 target field, §3.1 test changes)
- docfile: plan/008_82253c999440/docs/architecture/planner_prompt.md
  why: §1.1 (PlannerCommit at planner.go:57-61 has NO Files; ParsePlannerOutput ~171 is generic
       json.Unmarshal ⇒ field populated FOR FREE); §2.1 (the EXACT target field + doc comment); §3.1
       items 6-7 (the EXACT test changes for TestParsePlannerOutput + _RoundTrip).
  critical: §2.1 is the verbatim field; §3.1 items 6-7 are the verbatim test-change list. §4 item 2
       ("Files is guidance — do NOT add enforcement to validatePlannerOutput") is the validation fence.

# MUST READ — the findings (the compile-break gotcha + the json behavior)
- docfile: plan/008_82253c999440/P2M1T1S1/research/findings.md
  why: §1 (parse is FREE — do NOT edit ParsePlannerOutput); §2 (THE COMPILE-BREAK: planner_test.go:440
       `!=` must become `!reflect.DeepEqual` + add the reflect import — WITHOUT this the build fails);
       §3 (the exact field); §4 (the exact test changes — cleanMulti edit + 4 cases + null case +
       comparison loop + round-trip); §5 (json.Marshal []string: no omitempty ⇒ nil→"files":null→nil);
       §6 (zero overlap with parallel P1.M1.T3.S2).
  critical: §2 is the thing most likely to derail one-pass success — a struct with a slice is NOT
       comparable in Go; the existing `out.Commits[i] != original.Commits[i]` WILL NOT COMPILE after the
       field is added. Fix it to reflect.DeepEqual or the build breaks.

# MUST READ — the file being EDITED (the struct definition)
- file: internal/prompt/planner.go   (EDIT — add the field)
  section: `type PlannerCommit struct` (L57-61 — currently Title + Description only). Add `Files []string
           \`json:"files"\`` with the contract doc comment. Do NOT touch ParsePlannerOutput (~171), the
           plannerSystemPrompt const (L29-49), BuildPlannerSystemPrompt (~105), BuildPlannerUserPayload
           (~138), extractJSONObject, or any other symbol.
  why: this is the ONE production-code edit. The struct is consumed by ParsePlannerOutput (generic
       Unmarshal — populates Files for free), PlannerOutput.Commits, and downstream decompose code.
  pattern: match the existing field style (`\`json:"..."\`` tag + inline `// ...` doc comment).
  gotcha: do NOT add `omitempty` to the tag — the round-trip test relies on nil marshaling to
          `"files":null` (and parsing back to nil) for exactness; omitempty would omit the key entirely
          (still parses to nil, so functionally fine, but the contract tag is plain `json:"files"`).

# MUST READ — the test file being EDITED (the two functions + the compile-break line)
- file: internal/prompt/planner_test.go   (EDIT — TestParsePlannerOutput + _RoundTrip + reflect import)
  section: imports (L3-7 — add "reflect"); `TestParsePlannerOutput` (table ~L226-321 — the cleanMulti
           const + the 4 cases reusing it + the comparison loop at L363-366); `TestParsePlannerOutput_
           RoundTrip` (~L364-392 — the `out.Commits[i] != original.Commits[i]` at L440).
  why: the two test functions to update. cleanMulti is shared by 4 cases (clean/prose/code-fenced/
       whitespace) — editing it to add `"files":["a.go","b.go"]` on concept A exercises Files across all
       4 + (concept B with no files key) the missing-key-→-nil case in one row.
  pattern: the comparison loop already does per-field Title/Description checks — ADD a Files check in
           the same style (reflect.DeepEqual for slice correctness).
  gotcha: L440 `out.Commits[i] != original.Commits[i]` is a DIRECT struct comparison that WILL NOT
          COMPILE once PlannerCommit has a slice field. Rewrite to !reflect.DeepEqual + add the import.

# The PRD basis (in your context as selected_prd_content)
- file: PRD.md
  section: §9.14 FR-M3 (h3.30 — the planner output contract `commits:[{title,description,files}]`); §17.5
           (h3.81 — the JSON contract line with `"files": ["<path>", ...]`); §13.6.2 (h4.4 — the roles
           table: planner output `JSON {count, single, commits:[{title,description,files}], message?}`).
  critical: FR-M3b ("files' real job is telling each concept's stager where to look" + "a diagnostic
           only … does not hard-constrain the stager") confirms Files is GUIDANCE — never validated.

# The parallel sibling (coordination — ZERO overlap)
- docfile: plan/008_82253c999440/P1M1T3S2/PRP.md   (PARALLEL — arbiter freeze-parity tests)
  why: it touches internal/decompose/chain_test.go + decompose_test.go ONLY. This task touches
       internal/prompt/planner.go + planner_test.go ONLY. No shared file — no coordination needed.
```

### Current Codebase tree (relevant slice)

```bash
internal/prompt/
  planner.go        # EDIT: add Files []string to PlannerCommit (L57-61). ParsePlannerOutput UNCHANGED.
  planner_test.go   # EDIT: TestParsePlannerOutput (table + loop) + _RoundTrip (reflect.DeepEqual) + reflect import.
  {stager,system,payload,reserve,...}.go  # UNCHANGED (stager.go is T3; reserve.go is T2-adjacent).
go.mod / go.sum     # UNCHANGED (no new import — reflect is stdlib; production code adds nothing).
```

### Desired Codebase tree with files to be added/changed

```bash
internal/prompt/planner.go        # EDIT. PlannerCommit += Files []string `json:"files"` (+ doc comment).
                                  #   ParsePlannerOutput / consts / builders UNCHANGED.
internal/prompt/planner_test.go   # EDIT. + "reflect" import;
                                  #   TestParsePlannerOutput: cleanMulti += files on concept A; 4 cases
                                  #     updated; + a "files":null case; comparison loop += reflect.DeepEqual(Files);
                                  #   TestParsePlannerOutput_RoundTrip: original += Files on a commit;
                                  #     L440 `!=` → `!reflect.DeepEqual`.
# go.mod/go.sum UNCHANGED. validatePlannerOutput (decompose/planner.go) UNCHANGED. No other file touched.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (§1/§2 — THE COMPILE-BREAK): planner_test.go:440 `out.Commits[i] != original.Commits[i]` is a
// DIRECT struct comparison. PlannerCommit today has only string fields (comparable); adding Files []string
// makes it NON-COMPARABLE (Go forbids == / != on structs containing slices) ⇒ COMPILE ERROR. You MUST
// rewrite to !reflect.DeepEqual(out.Commits[i], original.Commits[i]) AND add "reflect" to the imports.
// Verified: this is the ONLY direct PlannerCommit struct-==/!= in the repo.

// CRITICAL (ParsePlannerOutput needs NO change): it is a generic json.Unmarshal into PlannerOutput.
// Adding the field makes Unmarshal populate it AUTOMATICALLY. Do NOT edit ParsePlannerOutput,
// extractJSONObject, or any builder/const. Editing them = scope creep into T2/S2.

// CRITICAL (validatePlannerOutput is in a DIFFERENT package): it lives in internal/decompose/planner.go:152,
// NOT internal/prompt/planner.go. This task does not touch it. And per FR-M3b, Files is guidance — it must
// NOT be validated (FR-M1c is the sole content guarantee). Do NOT add any Files check anywhere.

// GOTCHA (json tag is plain `json:"files"`, NO omitempty): nil marshals to "files":null → parses to nil;
// []string{} → "files":[] → []string{}; ["x","y"] → ["x","y"]. The round-trip is exact. reflect.DeepEqual
// distinguishes nil from []string{} — the correct semantics. Do NOT add omitempty (it would omit the key
// on nil, which still parses to nil, but the contract tag is plain).

// GOTCHA (cleanMulti is shared by 4 cases): cleanMulti feeds the clean / prose-wrapped / code-fenced /
// whitespace cases. Editing it to add "files":["a.go","b.go"] on concept A means ALL 4 expected outputs
// get Files on concept A. Concept B (no files key) stays nil — covering the missing-key-→-nil case in the
// SAME row. Then add ONE dedicated "files":null case. This is the lowest-churn way to satisfy the contract.

// GOTCHA (comparison loop uses reflect.DeepEqual for the slice): the existing loop checks c.Title and
// c.Description (strings, fine). ADD a Files check: if !reflect.DeepEqual(c.Files, want.Files) {...}.
// reflect.DeepEqual(nil, nil)==true, (nil, []string{})==false, (["a"],["a"])==true — exactly right.

// GOTCHA (do NOT touch the system prompt / stager / docs): plannerSystemPrompt const, BuildPlannerSystemPrompt,
// BuildStagerTask, docs/how-it-works.md are T2/T3's scope. This task is the struct field + the two parse
// tests ONLY. Touching them = a conflict with sibling subtasks.

// GOTCHA (no production-code import change): only the TEST file gains "reflect" (stdlib). planner.go adds
// no import. go.mod/go.sum byte-unchanged.
```

## Implementation Blueprint

### Data models and structure

No new TYPES. `PlannerCommit` gains one field. The v3 `PlannerOutput` / `ParsePlannerOutput` /
`extractJSONObject` are unchanged.

```go
// === internal/prompt/planner.go — EDIT PlannerCommit (L57-61) ===

// PlannerCommit is one partitioned concept from the planner (§17.5 JSON contract).
type PlannerCommit struct {
	Title       string   `json:"title"`       // "<short concept>" — a short label for the concept.
	Description string   `json:"description"` // "<precisely which files/hunks belong here, by path>" — staging instructions.
	Files       []string `json:"files"`       // FR-M3: every path this concept touches; guidance, not a constraint (FR-M1c is the content guarantee).
}
```

```go
// === internal/prompt/planner_test.go — EDIT imports (add "reflect") ===
import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)
```

```go
// === internal/prompt/planner_test.go — EDIT TestParsePlannerOutput ===

// 1) cleanMulti: add "files":["a.go","b.go"] on concept A; concept B keeps NO files key (→ nil).
cleanMulti := `{"count":2,"single":false,"commits":[{"title":"A","description":"d1","files":["a.go","b.go"]},{"title":"B","description":"d2"}]}`

// 2) The 4 cases reusing cleanMulti (clean / prose / code-fenced / whitespace): concept A now expects
//    Files:[]string{"a.go","b.go"}; concept B stays Files:nil (zero value — no change). E.g.:
PlannerOutput{Count: 2, Single: false, Commits: []PlannerCommit{
	{Title: "A", Description: "d1", Files: []string{"a.go", "b.go"}},
	{Title: "B", Description: "d2"}, // Files nil (missing key)
}}, false},

// 3) ADD a dedicated "files":null case:
nullFiles := `{"count":1,"single":false,"commits":[{"title":"N","description":"d","files":null}]}`
//   → PlannerOutput{Count:1, Single:false, Commits:[]PlannerCommit{{Title:"N",Description:"d"}}}, false
//   (Files nil — the zero value; no need to spell it)

// 4) The comparison loop (currently L363-366): ADD a Files check after Title/Description:
for i, c := range out.Commits {
	if c.Title != tc.wantOut.Commits[i].Title { ... }
	if c.Description != tc.wantOut.Commits[i].Description { ... }
	if !reflect.DeepEqual(c.Files, tc.wantOut.Commits[i].Files) {
		t.Errorf("Commits[%d].Files = %v, want %v", i, c.Files, tc.wantOut.Commits[i].Files)
	}
}
```

```go
// === internal/prompt/planner_test.go — EDIT TestParsePlannerOutput_RoundTrip ===

// 1) original: add Files to a commit (e.g. Commits[0]).
original := PlannerOutput{
	Count:  3,
	Single: false,
	Commits: []PlannerCommit{
		{Title: "A", Description: "dA", Files: []string{"x", "y"}}, // ← Files survives the round-trip
		{Title: "B", Description: "dB"},
		{Title: "C", Description: "dC"},
	},
	Message: "",
}

// 2) L440: the direct struct comparison COMPILE-BREAKS once PlannerCommit has a slice. Rewrite:
for i := range out.Commits {
	if !reflect.DeepEqual(out.Commits[i], original.Commits[i]) {
		t.Errorf("Commits[%d] = %+v, want %+v", i, out.Commits[i], original.Commits[i])
	}
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: internal/prompt/planner.go — ADD Files []string to PlannerCommit
  - ADD the field per the Blueprint above (exact tag + doc comment).
  - GOTCHA: do NOT edit ParsePlannerOutput, extractJSONObject, any const, any builder. The field is
            populated by the existing generic json.Unmarshal for free.
  - GOTCHA: do NOT add omitempty to the tag.
  - NAMING/PLACEMENT: field named Files, tag `json:"files"`, placed after Description in the struct.

Task 2: internal/prompt/planner_test.go — add "reflect" import
  - ADD "reflect" to the import block (alphabetical: encoding/json, reflect, strings, testing).

Task 3: internal/prompt/planner_test.go — EDIT TestParsePlannerOutput (table + comparison loop)
  - EDIT cleanMulti: concept A gets "files":["a.go","b.go"]; concept B keeps no files key.
  - UPDATE the 4 cases reusing cleanMulti (clean multi-commit JSON / prose / code-fenced / whitespace):
    concept A expects Files:[]string{"a.go","b.go"}.
  - ADD a "files":null case → Commits[0] with Files nil (zero value).
  - (The missing-files-key-→-nil case is covered by concept B in the cleanMulti row AND by the existing
    singleMsg/extraFields cases which have no files key — no separate case needed, but you MAY add one
    for explicitness if desired.)
  - EDIT the comparison loop: ADD `if !reflect.DeepEqual(c.Files, tc.wantOut.Commits[i].Files) {...}`
    after the Title/Description checks.
  - GOTCHA: the existing singleMsg / extraFields / nullCommits cases need NO change (their concepts have
            no files key ⇒ Files nil ⇒ the zero value already matches; the new reflect.DeepEqual loop
            check passes for nil==nil).

Task 4: internal/prompt/planner_test.go — EDIT TestParsePlannerOutput_RoundTrip
  - ADD Files:[]string{"x","y"} to original.Commits[0] (at least one commit; others can stay nil).
  - REWRITE L440 `out.Commits[i] != original.Commits[i]` → `!reflect.DeepEqual(out.Commits[i], original.Commits[i])`.
  - GOTCHA: without this rewrite the file WILL NOT COMPILE (PlannerCommit now contains a slice). This is
            the load-bearing fix — see findings §2.

Task 5: VERIFY (run all gates; fix before declaring done)
  - gofmt -w internal/prompt/planner.go internal/prompt/planner_test.go
  - go build ./... && go vet ./internal/prompt/
  - go test -race ./internal/prompt/ -run "TestParsePlannerOutput" -v   # the updated table + round-trip
  - go test ./...   # full regression — no other test should change (PlannerCommit is only compared in the round-trip)
  - git diff --exit-code go.mod go.sum → empty.
  - git status → EXACTLY 2 files (planner.go, planner_test.go). validatePlannerOutput/consts/stager/docs UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// PATTERN (generic parse populates the field for free): ParsePlannerOutput does json.Unmarshal into
//   PlannerOutput.Commits []PlannerCommit. Adding the field ⇒ Unmarshal fills it. NO parse edit.

// CRITICAL (struct-with-slice is non-comparable): the existing `out.Commits[i] != original.Commits[i]`
//   is a direct struct ==. Once PlannerCommit has []string, Go rejects it at compile time. Fix:
//   !reflect.DeepEqual(out.Commits[i], original.Commits[i]). Add "reflect" import.

// PATTERN (lowest-churn table edit): cleanMulti is shared by 4 cases. Put files on concept A (covers the
//   "files present" signal across all 4) and leave concept B without the key (covers "missing-key → nil"
//   in the same row). Add ONE dedicated "files":null case. The comparison loop gets ONE reflect.DeepEqual.

// CRITICAL (Files is guidance — never validated): do NOT touch validatePlannerOutput (different package
//   anyway) and do NOT add any non-empty-Files check. FR-M1c (freeze-subset verification in runLoop) is
//   the sole content guarantee. A concept with files:[]/null/missing must parse and flow through cleanly.
```

### Integration Points

```yaml
PROMPT PACKAGE (internal/prompt/planner.go — EDIT):
  - add: "PlannerCommit.Files []string `json:\"files\"` (+ doc comment). Populated by ParsePlannerOutput's
          existing generic json.Unmarshal (NO parse edit)."

TESTS (internal/prompt/planner_test.go — EDIT):
  - add: "reflect import; TestParsePlannerOutput table (cleanMulti files + null case) + comparison-loop
          Files check; TestParsePlannerOutput_RoundTrip Files + reflect.DeepEqual rewrite (compile-fix)."

GO MODULE (go.mod/go.sum): change NONE. reflect is stdlib; production code adds no import.

UPSTREAM (consume, do NOT edit): the v3 PlannerOutput / ParsePlannerOutput / extractJSONObject are
      COMPLETE. This task only adds a field to a struct they already decode generically.

DOWNSTREAM (consumers — not this task):
  - S2 (P2.M1.T1.S2): FR-M3b coverage check unions concept.Files vs DiffTreeNames(baseTree, tStart).
  - T2 (P2.M1.T2.S1): the plannerSystemPrompt JSON-contract line references "files" (the field makes the
        contract typed).
  - T3 (P2.M1.T3.S1): BuildStagerTask renders concept.Files as the stager files block.

FROZEN/LEAVE (do NOT edit):
  - ParsePlannerOutput, extractJSONObject, plannerSystemPrompt, BuildPlannerSystemPrompt,
    BuildPlannerUserPayload, PlannerRetryInstruction (internal/prompt/planner.go).
  - BuildStagerTask, stagerGuardrails (internal/prompt/stager.go) → T3.
  - validatePlannerOutput, callPlanner (internal/decompose/planner.go) → different package / S2.
  - internal/decompose/*_test.go → PARALLEL P1.M1.T3.S2.
  - docs/how-it-works.md → T2. go.mod/go.sum, Makefile, PRD.md.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
cd /home/dustin/projects/stagehand
gofmt -w internal/prompt/planner.go internal/prompt/planner_test.go
go vet ./internal/prompt/
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; gofmt -l empty; go.mod/go.sum unchanged.
# CRITICAL: if `go build ./...` fails with "struct containing []string cannot be compared", you missed
# the L440 reflect.DeepEqual rewrite (Task 4). Fix it before proceeding.
```

### Level 2: Unit Tests (the parse round-trip + table)

```bash
# The two updated tests, verbose:
go test -race ./internal/prompt/ -run "TestParsePlannerOutput" -v
# Expected: all green. Specifically:
#   - the clean/prose/code-fenced/whitespace cases pass with concept A Files=["a.go","b.go"]
#   - the "files":null case passes (Files nil)
#   - the comparison loop's reflect.DeepEqual(Files) passes for all cases
#   - TestParsePlannerOutput_RoundTrip passes (Files survives marshal→parse; reflect.DeepEqual holds)

# The full prompt suite (regression — the other planner tests are UNCHANGED by this task):
go test -race ./internal/prompt/ -v
```

### Level 3: Integration / Compile Proof

```bash
# The compile-fix is itself the integration proof: the whole module must build with the new slice field.
go build ./...
# Expected: builds cleanly. If it fails on planner_test.go:440, apply the Task 4 reflect.DeepEqual fix.

# Verify no OTHER PlannerCommit struct comparison broke (there are none, but confirm):
go vet ./...   # vet would surface any remaining == on the struct if missed
```

### Level 4: Regression & Audit

```bash
cd /home/dustin/projects/stagehand
go test ./...                 # FULL regression — only the 2 parse tests change behavior
git status --short            # Expected: EXACTLY 2 files:
                              #   M internal/prompt/planner.go
                              #   M internal/prompt/planner_test.go
# Expected: full test green; only 2 files; go.mod/go.sum unchanged; validatePlannerOutput / consts /
#   stager / docs / internal/decompose/* byte-unchanged.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `gofmt -l internal/` empty; `go vet ./internal/prompt/` clean; `go build ./...` succeeds.
- [ ] Level 2: `go test -race ./internal/prompt/ -run TestParsePlannerOutput -v` green (table + round-trip).
- [ ] Level 3: `go build ./...` + `go vet ./...` clean (the compile-fix holds; no other struct-== broke).
- [ ] Level 4: `go test ./...` green; `git status` shows EXACTLY 2 files; go.mod/go.sum unchanged.

### Feature Validation

- [ ] `PlannerCommit.Files []string \`json:"files"\`` exists with the contract doc comment.
- [ ] `ParsePlannerOutput` is UNCHANGED and populates Files for `"files":["a.go","b.go"]`; nil for
      `"files":null` and for a missing files key.
- [ ] `TestParsePlannerOutput` exercises Files (present, null, missing-key) + the comparison loop checks it.
- [ ] `TestParsePlannerOutput_RoundTrip` includes Files in `original` and compiles (reflect.DeepEqual).
- [ ] `validatePlannerOutput` is UNCHANGED (Files is guidance, never validated).

### Code Quality Validation

- [ ] The field follows the existing struct style (tag + inline doc comment); placed after Description.
- [ ] The compile-break (`!=` → `reflect.DeepEqual`) is the minimal fix; `reflect` imported.
- [ ] No parse-code / const / builder / validation / docs / stager change (scope fence honored).
- [ ] File placement matches the desired tree (only planner.go + planner_test.go); anti-patterns avoided.

### Documentation & Deployment

- [ ] The field's doc comment names FR-M3 + the guidance-not-constraint rationale (FR-M1c).
- [ ] N/A — no user-facing docs (item contract: "DOCS: none — internal data-model field").
- [ ] Implementation summary records: the free-parse fact, the compile-break fix, the scope fence.

---

## Anti-Patterns to Avoid

- ❌ **Don't edit `ParsePlannerOutput`.** It is a generic `json.Unmarshal`; the field is populated for free.
  Editing it (or `extractJSONObject`) is scope creep with zero benefit.
- ❌ **Don't leave `out.Commits[i] != original.Commits[i]` as-is.** A struct with a `[]string` field is
  NON-COMPARABLE in Go — the build WILL FAIL. Rewrite to `!reflect.DeepEqual(...)` and add the `reflect`
  import. This is the single most likely one-pass failure; fix it in Task 4.
- ❌ **Don't add `omitempty` to the `json:"files"` tag.** The contract tag is plain; nil marshals to
  `"files":null` and parses back to nil (exact round-trip). omitempty would omit the key (still parses to
  nil, so functionally fine) but diverges from the contract tag.
- ❌ **Don't touch `validatePlannerOutput`.** It lives in a different package (`internal/decompose/planner.go`)
  and Files must NOT be validated — FR-M3b is explicit that `files` is guidance only (FR-M1c is the sole
  content guarantee). A concept with `files:[]` / `null` / missing must parse and flow cleanly.
- ❌ **Don't touch the system prompt, stager, or docs.** `plannerSystemPrompt`, `BuildPlannerSystemPrompt`,
  `BuildStagerTask`, `stagerGuardrails`, and `docs/how-it-works.md` are T2/T3's scope. This task is the
  struct field + the two parse tests ONLY.
- ❌ **Don't add a separate missing-files-key test case if concept B already covers it.** Putting files on
  concept A and leaving concept B without the key covers BOTH "files present" and "missing-key → nil" in
  one cleanMulti row. Add ONE dedicated `"files":null` case. (You MAY add an explicit missing-key case for
  clarity, but it is not required.)
- ❌ **Don't forget the `"reflect` import.** The comparison-loop + round-trip rewrites both use
  `reflect.DeepEqual`; without the import the test file won't compile.
- ❌ **Don't edit `internal/decompose/*_test.go`.** That is the PARALLEL sibling P1.M1.T3.S2's domain
  (arbiter freeze-parity tests). Zero overlap — keep it that way.
