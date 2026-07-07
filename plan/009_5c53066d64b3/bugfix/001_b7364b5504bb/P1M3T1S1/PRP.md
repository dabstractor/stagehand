---
name: "P1.M3.T1.S1 — Bind resolved manifest + hoist payload in hook.Run loop (the refactor prerequisite for the FR-T1 multi-turn gate on the hook path)"
description: |

  A pure 3-line refactor of `internal/hook/exec.go` that prepares the hook generation loop for the FR-T1
  multi-turn trigger gate (landed by the SIBLING subtask P1.M3.T1.S2). Today the loop (exec.go:157-205)
  (a) resolves the manifest INLINE and discards it — `retryInstr := *deps.Manifest.Resolve().RetryInstruction`
  (L151) — so the manifest's `SessionMode` (which the gate needs) is unreachable; and (b) declares `payload`
  LOOP-scoped with `:=` (L158), so it does not survive the loop for a post-loop gate. This subtask binds the
  resolved manifest to a variable and hoists `payload` to function scope. NO behavioral change; NO new logic;
  the gate itself is S2.

  THE 3 EDITS (exact — verified against the live tree + resolution_strategy.md ISSUE 2 Edit 1/2/3):
    Edit 1 (L151): `retryInstr := *deps.Manifest.Resolve().RetryInstruction`
                   → `resolved := deps.Manifest.Resolve()`  (new line)
                     `retryInstr := *resolved.RetryInstruction`
    Edit 2 (after L155 `var parseFail bool`): add  `var payload string // hoisted: survives the loop for the FR-T1 gate`
    Edit 3 (L158): `payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)`
                   → `payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)`  (`:=` → `=`)

  ⚠️ **#1 — ZERO shadowing risk (verified).** `grep "payload\|resolved" internal/hook/exec.go` returns ONLY
      L158/L160/L163 (all `payload`, all inside the loop). `payload` appears nowhere else in `Run` ⇒ hoisting
      it + switching L158 to `=` turns the loop's per-iteration declaration into an assignment to the hoisted
      var (L160 assign + L163 read behave identically). `resolved` is unused elsewhere ⇒ no clash. `go vet`
      is the deterministic confirmation. See research verification.md §2.

  ⚠️ **#2 — `resolved` is bound because the gate needs `SessionMode`.** `Manifest.Resolve()` (manifest.go:150)
      returns a `Manifest` with `SessionMode *string` (non-nil after Resolve) AND `RetryInstruction *string`.
      The inline `*deps.Manifest.Resolve().RetryInstruction` discarded the manifest, so `SessionMode` was
      unreachable. Binding `resolved` keeps `retryInstr` byte-identical AND exposes `resolved.SessionMode` for
      S2's FR-T1 gate. See verification.md §3.

  ⚠️ **#3 — Pure refactor; existing hook tests stay green.** The edits change ONLY variable binding/scoping —
      no control-flow, Render/Execute/ParseOutput call, or return changes. `retryInstr` is byte-identical;
      `payload`'s value at every iteration is unchanged (same `BuildUserPayload` call, same assignment
      semantics, just function-scoped now). `go test ./internal/hook/...` MUST stay green (the contract's
      no-behavioral-change proof). See verification.md §5.

  ⚠️ **#4 — No conflict with the parallel work item (P1.M2.T1.S2).** P1.M2.T1.S2 (dry-run runPipeline gate)
      is **pkg/stagecoach ONLY** — its PRP explicitly excludes `internal/hook/exec.go` as P1.M3's scope (lines
      64/242/297-298/476/606). This task edits `internal/hook/exec.go` ONLY. Zero file overlap ⇒ no merge
      conflict. (P1.M2.T1.S1, Complete, did the analogous `payload` hoist in runPipeline — the precedent.)
      See verification.md §4.

  Deliverable: MODIFIED `internal/hook/exec.go` (the 3 edits above + an inline comment on the hoisted
  `payload` noting it's for the FR-T1 gate). NO other file. NO go.mod change. NO new logic. OUTPUT: the
  resolved manifest bound (`resolved`) and `payload` hoisted to function scope in `hook.Run`, available for
  the FR-T1 gate. Consumed by P1.M3.T1.S2. DOCS: none — pure refactor, no user-facing/config/API surface
  change.

---

## Goal

**Feature Goal**: Prepare `internal/hook/exec.go`'s `Run` generation loop for the FR-T1 multi-turn trigger
gate (P1.M3.T1.S2) by (a) binding the resolved manifest to a variable (exposing `resolved.SessionMode` the
gate needs) and (b) hoisting `payload` from loop scope to function scope (so it survives the loop for a
post-loop gate). Pure refactor — zero behavioral change.

**Deliverable**: MODIFIED `internal/hook/exec.go` — the 3 edits (bind `resolved` at L151; add `var payload
string` after L155; switch L158 `:=`→`=`) + a one-line comment on the hoisted `payload`. No other file.

**Success Definition**: `go vet ./internal/hook/` clean (no shadowing); `go build ./...` clean; `go test
./internal/hook/...` green (no behavioral change — existing tests pass unmodified); `gofmt -l` clean; only
`internal/hook/exec.go` changed; go.mod/go.sum byte-unchanged; the bound `resolved` and hoisted `payload`
are visible at function scope after the loop (ready for S2's gate).

## User Persona

**Target User**: The sibling subtask P1.M3.T1.S2 (the FR-T1 multi-turn gate on the hook path), which needs
`resolved.SessionMode` (to decide multi-turn eligibility) and the post-loop `payload` (the captured
one-shot payload the multi-turn session reuses, FR-T2). Transitively: a user running `stagecoach hook exec`
(most often indirectly via the installed `prepare-commit-msg` hook) whose large diff should fall back to
multi-turn — currently it can't because the gate isn't wired (Issue 2).

**Use Case**: (internal prerequisite) S2 will read `*resolved.SessionMode` and the hoisted `payload` after
the one-shot loop to decide + execute multi-turn fallback. This task makes those two values reachable.

**User Journey**: (refactor; no runtime change) hook.Run → loop resolves manifest (now bound) + builds
payload (now hoisted) → after the loop, S2's gate reads both → multi-turn fallback (S2) if the trigger
fires. Today the loop works identically; only the variable lifetimes change.

**Pain Points Addressed**: Without binding `resolved`, the gate cannot read `SessionMode` (the manifest is
inline-discarded). Without hoisting `payload`, it doesn't survive the loop for the gate. This refactor
unblocks S2 with zero behavioral risk.

## Why

- **Unblocks the hook-path multi-turn propagation (Issue 2).** The FR-T1 gate landed in `CommitStaged`
  (generate.go) and is being ported to `runPipeline` (P1.M2.T1.S2). The hook path (`hook.Run`) is the third
  duplicated loop and needs the same gate (P1.M3.T1.S2). This refactor is the prerequisite: it exposes the
  two values the gate reads, with no behavioral change, so S2 is a pure gate insertion.
- **Minimal, safe, contract-specified.** Three line-accurate edits (resolution_strategy.md ISSUE 2 Edit
  1/2/3), zero shadowing (verified), no logic change. The cheapest possible unblock.
- **No surface change.** Internal refactor; no config/API/flag/doc impact.

## What

Three edits to `internal/hook/exec.go` (bind `resolved`; hoist `payload`; `:=`→`=`). No new logic, no
control-flow change, no new types, no new deps. The gate itself is S2.

### Success Criteria

- [ ] L151: `resolved := deps.Manifest.Resolve()` + `retryInstr := *resolved.RetryInstruction` (replaces the
      inline `*deps.Manifest.Resolve().RetryInstruction`).
- [ ] After L155 (`var parseFail bool`): `var payload string` with a comment noting it's hoisted for the
      FR-T1 gate.
- [ ] L158: `payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)` (`:=` → `=`).
- [ ] `go vet ./internal/hook/` clean (no shadowing); `go build ./...` clean; `gofmt -l` clean.
- [ ] `go test ./internal/hook/...` green (existing tests unchanged — no behavioral change).
- [ ] Only `internal/hook/exec.go` changed; go.mod/go.sum byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can do this from: the 3 exact edits (with line anchors),
the shadowing-safety evidence (research §2), the `Resolve()` API confirmation (§3), and the no-conflict
confirmation (§4). No multi-turn/gate/decompose knowledge required — this is a variable-binding refactor.

### Documentation & References

```yaml
# MUST READ — the live-tree verification (line-accuracy + shadowing + no-conflict)
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M3T1S1/research/verification.md
  why: §1 (the 3 edits match L151/L154-155/L158 exactly), §2 (ZERO shadowing — the load-bearing safety
       check), §3 (Resolve() returns Manifest with SessionMode+RetryInstruction), §4 (no conflict with
       P1.M2.T1.S2 — pkg/stagecoach only), §5 (pure refactor; tests stay green).
  critical: §2 (shadowing) and §3 (why `resolved` must be bound) are the things that confirm the refactor
       is safe and purposeful.

# The authoritative edit spec (cited by the contract)
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/docs/architecture/resolution_strategy.md
  section: "ISSUE 2 ... Edit 1 — Bind `resolved` (line 151) / Edit 2 — Hoist `payload` (lines 154-155) /
           Edit 3 — Loop body: `:=` → `=` (line 158)".
  why: the exact 3 edits this task implements (matches the contract verbatim).
  critical: Edit 1 binds `resolved` (NOT a rename of retryInstr — a NEW variable holding the Resolve() result).

# The loop map (cited by the contract)
- docfile: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/docs/architecture/research_hook_exec.md
  section: "## 2. The `hook.Run` loop (exec.go:157-205)" — the full loop with line numbers; §4 (variables in
           scope); §5 (CommitStaged vs hook diff — why the gate ports cleanly).
  why: confirms the loop structure + that `payload`/`resolved` are loop-local today (hoisting is safe).

# The file being edited — READ the loop before editing
- file: internal/hook/exec.go
  section: Run → Step F (L149-151: ResolveRoleModel + the inline Resolve) → Step G var block (L154-155) →
           the loop (L157-205; L158 payload, L160 retryInstr preamble, L163 Render).
  why: the EXACT anchors for the 3 edits. Note `payload` is used at L158 (decl), L160 (assign), L163 (read)
       — all inside the loop; hoisting + `:=`→`=` preserves all three.
  critical: do NOT touch the loop body logic (Render/Execute/ParseOutput/FinalizeMessage/dedupe/WriteMessageFile),
       the never-block returns (timeout/cancel), or the exhaustion return. ONLY the 3 edits.

# The Resolve() API (why `resolved` is needed)
- file: internal/provider/manifest.go
  section: `func (m Manifest) Resolve() Manifest` (L150); `SessionMode *string` (L66; Resolve→strPtr("") L177);
           `RetryInstruction *string` (L88; Resolve→default L193).
  why: confirms `resolved := deps.Manifest.Resolve()` yields a Manifest with non-nil SessionMode (the gate's
       need) + non-nil RetryInstruction (so `*resolved.RetryInstruction` is safe).
  critical: the inline form discarded the manifest; binding it is the ONLY way to reach SessionMode.

# The parallel PRP — the no-conflict confirmation (READ to be sure)
- file: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M2T1S2/PRP.md
  why: confirms P1.M2.T1.S2 is pkg/stagecoach ONLY and explicitly excludes internal/hook/exec.go (P1.M3's
       scope). Zero file overlap ⇒ this refactor and the parallel gate-insertion don't collide.
  critical: P1.M2.T1.S1 (Complete) already did the analogous payload hoist in runPipeline — same pattern.

# The consumer (NOT this task)
- file: plan/009_5c53066d64b3/bugfix/001_b7364b5504bb/P1M3T1S2/PRP.md (when it exists)
  why: P1.M3.T1.S2 inserts the FR-T1 gate, reading `*resolved.SessionMode` + the hoisted `payload` after the
       loop. This task exposes those two values; S2 consumes them.
```

### Current Codebase tree (relevant slice)

```bash
internal/hook/
  exec.go           # Run loop (L149-205) — EDIT (3 edits: bind resolved L151; hoist payload after L155; :=→= L158)
  exec_test.go (or *_test.go)  # existing hook tests — UNCHANGED (must stay green)
internal/provider/manifest.go  # Resolve() Manifest + SessionMode/RetryInstruction — UNCHANGED (the API consumed)
go.mod / go.sum                 # UNCHANGED (no new dep)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. ONE in-place edit to internal/hook/exec.go (3 line-level changes + a comment).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — ZERO shadowing, verified): payload appears ONLY at L158/L160/L163 (all inside the loop);
// resolved is unused elsewhere. Hoisting payload + switching L158 :=→= is safe — the loop's payload becomes
// an assignment to the hoisted var. `go vet` confirms. (research §2)

// CRITICAL (#2 — bind `resolved` to expose SessionMode): the inline `*deps.Manifest.Resolve().RetryInstruction`
// DISCARDS the manifest. Resolve() returns a Manifest with non-nil SessionMode (the FR-T1 gate's input) +
// non-nil RetryInstruction. Binding `resolved` keeps retryInstr byte-identical AND makes resolved.SessionMode
// reachable for S2. (research §3)

// GOTCHA (do NOT touch the loop body): only the 3 edits. The Render/Execute/ParseOutput/FinalizeMessage/
// dedupe/WriteMessageFile calls, the never-block returns (timeout/cancel), and the exhaustion return are
// UNCHANGED. Any behavioral drift breaks the "no behavioral change" guarantee + existing tests.

// GOTCHA (Edit 3 is `:=` → `=`, not a rename): L158 keeps `prompt.BuildUserPayload(diff, cfg.Context,
// rejected)` verbatim — only the assignment operator changes (payload is now declared at function scope by
// Edit 2). Do NOT alter the BuildUserPayload arguments.

// GOTCHA (comment on the hoisted payload): add `// hoisted: survives the loop for the FR-T1 gate` so a future
// reader understands WHY payload is function-scoped (otherwise it looks like it could be loop-local again).

// GOTCHA (no new imports/deps): exec.go already imports prompt/config/provider/generate/fmt/errors. The 3
// edits add nothing. `go mod tidy` is a no-op.
```

## Implementation Blueprint

### Data models and structure

No new types. The 3 edits, as a precise diff:

```go
// internal/hook/exec.go — Run(), Step F + Step G var block + loop top.

// ── Edit 1 (L151): bind the resolved manifest ──────────────────────────────────────────────
// BEFORE (L149-151):
//	_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
//	retryInstr := *deps.Manifest.Resolve().RetryInstruction
// AFTER:
	_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
	resolved := deps.Manifest.Resolve()            // bound: the FR-T1 gate (P1.M3.T1.S2) reads resolved.SessionMode
	retryInstr := *resolved.RetryInstruction        // byte-identical to the previous inline form

// ── Edit 2 (after L155): hoist payload to function scope ──────────────────────────────────
// BEFORE (L154-155):
//	var rejected []string
//	var parseFail bool
// AFTER:
	var rejected []string
	var parseFail bool
	var payload string // hoisted: survives the loop for the FR-T1 multi-turn gate (P1.M3.T1.S2)

// ── Edit 3 (L158): loop body `:=` → `=` ───────────────────────────────────────────────────
// BEFORE (L157-158):
//	for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
//		payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)
// AFTER:
	for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {
		payload = prompt.BuildUserPayload(diff, cfg.Context, rejected) // `=` (declared above), not `:=`
// (L160 `payload = retryInstr + "\n\n" + payload` and L163 `... payload, msgReasoning` are UNCHANGED —
//  they already assign/read payload, identical semantics now that it's function-scoped.)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: APPLY the 3 edits to internal/hook/exec.go
  - EDIT 1 (L151): replace the inline `retryInstr := *deps.Manifest.Resolve().RetryInstruction` with the
      2-line `resolved := deps.Manifest.Resolve()` + `retryInstr := *resolved.RetryInstruction`.
  - EDIT 2 (after L155 `var parseFail bool`): add `var payload string // hoisted: survives the loop for the
      FR-T1 multi-turn gate (P1.M3.T1.S2)`.
  - EDIT 3 (L158): change `payload :=` to `payload =` (keep the BuildUserPayload call verbatim).
  - DO NOT touch the loop body logic, the never-block returns, or the exhaustion return.
  - DO NOT add imports/deps.

Task 2: VERIFY (no behavioral change + no shadowing)
  - RUN: gofmt -w internal/hook/exec.go && go vet ./internal/hook/ && go build ./...
  - RUN: go test ./internal/hook/...   # existing tests MUST stay green (no behavioral change)
  - RUN: go test ./...                  # whole-repo green (belt-and-suspenders; no code outside exec.go changed)
  - CONFIRM: only internal/hook/exec.go changed (`git diff --name-only`); go.mod/go.sum byte-unchanged.

Task 3: HAND-OFF (no further edit)
  - Confirm `resolved` and the hoisted `payload` are visible at function scope AFTER the loop (the FR-T1
      gate's read site). This is the unblock for P1.M3.T1.S2; no gate is inserted here.
```

### Implementation Patterns & Key Details

```go
// THE bind (Edit 1) — resolved is the manifest the gate reads SessionMode from:
resolved := deps.Manifest.Resolve()
retryInstr := *resolved.RetryInstruction   // byte-identical to the old inline form

// THE hoist (Edit 2 + Edit 3) — payload survives the loop for the post-loop gate:
var payload string // hoisted: survives the loop for the FR-T1 gate (P1.M3.T1.S2)
for attempt := ... {
	payload = prompt.BuildUserPayload(diff, cfg.Context, rejected)  // `=`, not `:=`
	// ... L160 assign / L163 read UNCHANGED ...
}
// (after the loop) S2 reads `*resolved.SessionMode` + `payload` — both now in scope.
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. The 3 edits add no imports. `go mod tidy` is a no-op.

PACKAGE EDGES: NONE. exec.go stays in package hook; it uses the already-imported prompt/config/provider/
      generate. No new dependency.

UPSTREAM (the inputs — consume, do NOT edit):
  - deps.Manifest.Resolve() (manifest.go:150) — returns Manifest with SessionMode + RetryInstruction.
  - prompt.BuildUserPayload — the loop's payload builder (call UNCHANGED; only the assignment operator).

DOWNSTREAM (the consumer — NOT this task):
  - P1.M3.T1.S2 (FR-T1 gate on the hook path): reads `*resolved.SessionMode` + the hoisted `payload` after
      the loop to decide + execute multi-turn fallback. This task exposes those two values.

FROZEN/LEAVE (do NOT edit):
  - The loop body (Render/Execute/ParseOutput/FinalizeMessage/dedupe/WriteMessageFile), the never-block
    returns (timeout/cancel), the exhaustion return.
  - internal/provider/*, internal/generate/*, internal/config/*, pkg/stagecoach/*, internal/cmd/*.
  - All other hook files (exec_test.go etc.). PRD.md, go.mod, Makefile.

NO NEW DATABASE / ROUTES / CLI / FILES / CONFIG / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style + shadowing

```bash
gofmt -w internal/hook/exec.go
go vet ./internal/hook/     # MUST be clean — confirms no shadowing from the hoist/bind
go build ./...              # MUST be clean
# Expected: go vet clean (the load-bearing check — see research §2); go build clean.
```

### Level 2: No behavioral change (existing hook tests)

```bash
go test ./internal/hook/...  # MUST be green — the refactor changes no behavior
# Expected: every existing hook test (parse-success, duplicate-retry, parse-fail-retry, never-block on
# timeout/cancel/exit-1, exhaustion) passes UNCHANGED. If any fails, the refactor drifted behavior — revert.
```

### Level 3: Whole-repo + frozen-file check

```bash
go test ./...                # Expect all PASS (no code outside exec.go changed).
git diff --name-only         # Expect ONLY internal/hook/exec.go.
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
git diff --exit-code internal/provider internal/generate internal/config pkg internal/cmd && echo "frozen packages UNCHANGED (expected)"
# Confirm the 3 edits landed (and only those):
grep -n "resolved := deps.Manifest.Resolve\|var payload string\|payload = prompt.BuildUserPayload" internal/hook/exec.go
# Expected: 3 hits (the bind, the hoist, the `=` assignment).
```

### Level 4: Scope-availability reasoning (the unblock proof)

```bash
# The refactor's purpose is to make `resolved` + `payload` reachable after the loop. Verify by reasoning:
#   1. `resolved` is declared at function scope (Step F, ~L151) ⇒ in scope after the loop (the gate's site).
#   2. `payload` is declared at function scope (the new `var payload string`, after L155) ⇒ in scope after
#      the loop; its last-assigned value (the final one-shot payload) is what the gate reads.
#   3. `retryInstr` is byte-identical (same `*resolved.RetryInstruction`). No behavior change.
# (No Level-4 commands beyond Levels 1–3 — `go vet` + green tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go vet ./internal/hook/` clean (no shadowing); `go build ./...` clean; `gofmt -l` clean.
- [ ] `go test ./internal/hook/...` green (no behavioral change); `go test ./...` green (no regression).
- [ ] go.mod/go.sum byte-unchanged; ONLY `internal/hook/exec.go` changed.

### Feature Validation
- [ ] L151 binds `resolved` (manifest no longer inline-discarded); `retryInstr` byte-identical.
- [ ] `var payload string` hoisted after L155; L158 uses `=` (not `:=`).
- [ ] `resolved.SessionMode` + the hoisted `payload` are reachable after the loop (the unblock for S2).

### Code Quality Validation
- [ ] The 3 edits match resolution_strategy.md ISSUE 2 Edit 1/2/3 (and the contract) verbatim.
- [ ] No loop-body logic / never-block return / exhaustion return touched.
- [ ] Anti-patterns avoided (see below); no out-of-scope churn.

### Documentation
- [ ] Inline comment on the hoisted `payload` notes it's for the FR-T1 gate (so a future reader doesn't
      re-tighten the scope). No docs/*.md edits (internal refactor; P1.M4 owns the changeset doc sync).

---

## Anti-Patterns to Avoid

- ❌ **Don't touch the loop body.** Only the 3 edits (bind `resolved`; hoist `payload`; `:=`→`=`). Any change
      to Render/Execute/ParseOutput/FinalizeMessage/dedupe/WriteMessageFile/the never-block returns/the
      exhaustion return breaks the "no behavioral change" guarantee + existing tests. (research §5)
- ❌ **Don't rename `retryInstr` or alter the BuildUserPayload call.** Edit 1 keeps `retryInstr` byte-identical
      (same `*…RetryInstruction`); Edit 3 keeps the BuildUserPayload arguments verbatim — only the assignment
      operator changes. (research §1)
- ❌ **Don't inline the gate (that's S2).** This task is the REFACTOR prerequisite (bind + hoist). Inserting
      the FR-T1 gate, reading `*resolved.SessionMode` + `payload`, executing multi-turn, preserving FR-H5
      never-block — all of that is P1.M3.T1.S2. Stop at the 3 edits.
- ❌ **Don't worry about shadowing (it's verified safe) — but DO run `go vet`.** The grep proves `payload`/
      `resolved` are loop-local today; `go vet` is the deterministic confirmation. If `go vet` flags
      anything, the edits were misapplied — recheck. (research §2)
- ❌ **Don't edit any other file.** pkg/stagecoach/stagecoach.go is P1.M2.T1.S2's scope; internal/generate is
      the frozen reference gate; internal/provider/manifest.go is the consumed API. This task is
      internal/hook/exec.go ONLY. (research §4)
- ❌ **Don't add imports/deps.** The 3 edits use already-imported symbols. `go mod tidy` is a no-op.

---

## Confidence Score

**10/10** — a contract-specified, line-accurate 3-line refactor (bind `resolved`; hoist `payload`; `:=`→`=`)
verified against the live tree: the anchors (L151/L154-155/L158) match exactly, `Manifest.Resolve()` returns
the type with the `SessionMode` the gate needs, the shadowing-safety grep returns only the 3 in-loop `payload`
sites (hoisting is provably safe), and the parallel P1.M2.T1.S2 explicitly excludes `internal/hook/exec.go`
(zero file overlap). The edits match `resolution_strategy.md` ISSUE 2 Edit 1/2/3 verbatim, and the
no-behavioral-change guarantee is checked by the existing `go test ./internal/hook/...` suite (which must
pass unmodified). The refactor's sole purpose — making `resolved.SessionMode` + the hoisted `payload`
reachable after the loop for S2's gate — is confirmed by scope reasoning. No residual risk.
