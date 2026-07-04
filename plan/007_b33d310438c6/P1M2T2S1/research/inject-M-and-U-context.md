# Research — P1.M2.T2.S1: Inject -M + -U<diff_context> via buildDiffArgs; update golden fixtures

> Scope: FR3e (rename detection, `-M` always-on) + FR3f (reduced context, `-U<diff_context>` default 1).
> Both injected at the SINGLE shared argv site (`buildDiffArgs`, created by the parallel P1.M2.T1.S1) so
> they land in all 3 patch-argv paths (md-list / per-file / nmArgs) across StagedDiff/TreeDiff/
> WorkingTreeDiff. Then update golden fixtures + add rename and -U0 positive tests.
>
> **PREREQUISITE (parallel, assume LANDED): P1.M2.T1.S1.** It added `buildDiffArgs(domain ...string)
> []string` (returns `append([]string{"diff"}, domain...)`) immediately before StagedDiff, routed the 9
> argv sites (3 per function) through it, byte-identical. THIS task EXTENDS the helper's signature to take
> `opts` (for DiffContext) and injects `-M` + `-U<ctx>` after the domain. The 9 call sites change from
> `buildDiffArgs("--cached")` → `buildDiffArgs(opts, "--cached")` (opts is in scope in all 3 functions).

---

## 1. The helper change (signature + body + clamp)

`buildDiffArgs` today (post-P1.M2.T1.S1): `func buildDiffArgs(domain ...string) []string`. THIS task:

```go
// buildDiffArgs returns the leading argv for a `git diff`: ["diff", domain…, "-M", "-U<ctx>"].
// -M is ALWAYS ON (FR3e: deterministic rename detection — wins over diff.renames=false, never -C).
// -U<ctx> is the effective unified-context width (FR3f): opts.DiffContext clamped to [0,3]; the default
// production value is 1 (callers resolve via config.DiffContextValue, nil⇒1). 0 is VALID (-U0 = changed
// lines only); an out-of-range value defensively clamps to 1. domain is the diff domain
// ("--cached" / treeA treeB / none). The caller appends trailing tokens (--name-only / -- / paths).
// FR3e/FR3f are ALWAYS ON (system_context §6 invariant 1) — not gated on token_limit.
func buildDiffArgs(opts StagedDiffOptions, domain ...string) []string {
	ctx := opts.DiffContext
	if ctx < 0 || ctx > 3 {
		ctx = 1 // defensive: production resolves via DiffContextValue (nil⇒1); guards a malformed opts
	}
	args := append([]string{"diff"}, domain...)
	args = append(args, "-M")                     // FR3e
	args = append(args, fmt.Sprintf("-U%d", ctx)) // FR3f
	return args
}
```

**Token order** (the item's contract): `["diff", domain…, "-M", "-U<ctx>"]` THEN the caller appends its
trailing tokens (`--name-only -- *.md *.markdown` / `-- <file>` / `-- excludes :!*.md :!*.markdown
binExcludes`). So e.g. StagedDiff md-list becomes `["diff","--cached","-M","-U1","--name-only","--","*.md","*.markdown"]`.

**`fmt` is already imported** in git.go (88KB file; `fmt.Fprintf`/`Sprintf` used throughout — verify, but
near-certain). No new import.

---

## 2. The -U0 / default-1 resolution (DEFINITIVE — the key design decision)

Three independent sources AGREE, removing all ambiguity:

- **`StagedDiffOptions.DiffContext` doc** (git.go ~L60-66): *"0 is VALID (-U0 = changed lines only) —
  this is a PLAIN int (not *int) because the git layer takes the RESOLVED value: callers MUST pass the
  resolved context (default 1 when the user omits it) explicitly, NEVER a '0 means unset' sentinel."*
- **`system_context.md` line 91**: *"DiffContext default is 1, but 0 is a VALID value (-U0 = changed-lines-only, FR3f)."*
- **`config.DiffContextValue()`** (config.go ~L201-205): resolves the config-layer `*int DiffContext` →
  nil returns 1 (the default); non-nil returns `*c.DiffContext` (incl. explicit 0). `config.Defaults()`
  sets `DiffContext: intPtr(1)` (config.go L175).

**So:** the helper maps `0→-U0`, `1→-U1`, `2→-U2`, `3→-U3`, out-of-range→1. In PRODUCTION, opts.DiffContext
is ALWAYS the resolved value (1 default, or explicit [0,3]) — the 6 call sites all use `cfg.DiffContextValue()`.
Only TESTS passing a bare `StagedDiffOptions{}` get DiffContext=0 → -U0.

This is why the item's two test clauses coexist: the golden fixtures (updated to pass `DiffContext: 1`)
show -U1 ("context shrinks -U3→-U1"); the NEW -U0 test passes `DiffContext: 0` and asserts changed-lines-only.

---

## 3. The golden-fixture REALITY (verified — most pass unchanged)

I read all 3 test files. **CRITICAL: every assertion is substring/structural (`strings.Contains`,
`strings.Count`, `out != ""`, `count != 1`), NOT exact-output.** None assert on context-line shape. So:

- **-M won't break anything**: -M only changes output when a delete+add-similar pair (>50% similar) is
  staged. NONE of the fixtures stage such a pair (they stage simple additions `a.go`/`b.go`/`a.md`, binary
  files, exclusions, byte-cap blobs). No rename is detected ⇒ no `rename from`/`rename to` ⇒ output
  byte-identical for -M.
- **-U won't break substring tests**: -U changes context-line COUNT, but the substring assertions check
  file presence (`Contains "a.md"`), boundary markers (`Count "diff --git a/only.md"`), binary/excluded
  placeholders (`Contains "A\t[binary] logo.png"`), and truncation sentinels (`Contains "... [diff truncated..."`).
  File headers (`diff --git`, `---`, `+++`, `@@`) are retained at every -U level (incl. -U0). So the
  substring tests PASS at -U0, -U1, -U3 alike.
- **Line/byte-count BOUND tests stay green**: `TestStagedDiff_MarkdownLineCap` asserts `lineCount ≤ 11`
  (an UPPER bound) — fewer context lines only makes it smaller ⇒ still ≤ 11. `TestStagedDiff_NonMarkdownByteCap`
  asserts `len(out) < 200` — fewer context bytes ⇒ still < 200.

**THEREFORE: for THIS task (M2.T2, -M + -U only), the existing 3 golden suites very likely pass UNCHANGED.**
The item's "UPDATE the existing golden fixtures … context lines shrink from -U3 to -U1" + system_context §6
line 158 ("Existing test fixtures WILL change under FR3f") anticipate the CUMULATIVE M2+M3+M4 changes
(M3's numstat skeleton prepend, M4's truncation, FR3h's index-strip — those DO change substring-visible
output). For M2.T2 alone, the fixture churn is expected to be ~zero.

**Update strategy (run-driven, not blind):**
1. Apply the helper change.
2. RUN `go test ./internal/git/ -run 'TestStagedDiff|TestTreeDiff|TestWorkingTreeDiff' -v`.
3. ANY test that breaks ⇒ it implicitly depended on -U3 context lines; update its expectation (keep the
   boundary-marker `diff --git` + truncation-sentinel assertions — those are the stability anchors per
   system_context §6 line 158). For a body-shape assertion, drop the expected context lines to match the
   test's DiffContext (0 if it passes `{}`; 1 if you set it).
4. Tests that pass ⇒ leave them. (Per the struct doc, a test exercising the DEFAULT path SHOULD pass
   `DiffContext: 1` to mirror production — optional correctness improvement, not required for green.)
5. Do NOT add `DiffContext: 1` to tests whose purpose is unrelated (binary/excludes/truncation) — they're
   fine at `{}` (→-U0); churning them is noise.

---

## 4. -M scope: 3 patch paths ONLY; binary.go UNTOUCHED

The item's contract #3: "the helper must apply to all three [md-list, per-file, nmArgs] since they all
route through it." So -M/-U land on the 3 PATCH argv paths per function (9 sites total). They do NOT land on:

- **`binary.go`'s `detectBinaryFiles`/`fileStatuses`** — these build their OWN argv
  (`["diff", diffArgs…, "--numstat"/"--name-status"]`) and are called with the domain. P1.M2.T1.S1 left
  them alone (pattern to mirror, not refactor); THIS task leaves them alone too. WHY no -M on numstat:
  git_diff_semantics §3 warns "-M makes numstat's path column use `=>`/`{...}` rename notation — harder
  to parse." Keep numstat/name-status -M-free so their parsing stays simple. (FR3e "every diff path" is
  satisfied by the 3 patch paths; numstat is a sizing/status helper, not the model-facing patch.)

**Order in binary.go calls is UNCHANGED** — the 3 functions still call `g.detectBinaryFiles(ctx, "--cached")`
etc. with the bare domain; those internal argv builders are not routed through buildDiffArgs.

---

## 5. The 2+ NEW positive tests

### (a) Rename detection (-M) → `rename from`/`rename to` (NOT delete+add)
Stage a pure rename (git mv, or stage a delete + an add of identical content), call StagedDiff with
DiffContext:1, assert the output contains `rename from`/`rename to` (and `similarity index 100%`), and
does NOT contain a delete+add pair for the same content. This proves FR3e.

```go
func TestStagedDiff_RenameDetectedCompact(t *testing.T) {
	repo := t.TempDir(); initRepo(t, repo)
	writeFile(t, repo, "old.go", "package main\nfunc a(){}\n")
	stageFile(t, repo, "old.go")
	execGit(t, repo, "commit", "-qm", "base")           // establish baseline
	// pure rename: identical content, new path
	writeFile(t, repo, "new.go", "package main\nfunc a(){}\n")
	stageFile(t, repo, "new.go")
	os.Remove(filepath.Join(repo, "old.go"))
	execGit(t, repo, "add", "-A")                        // stage the rename (delete old + add new)
	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{DiffContext: 1})
	if err != nil { t.Fatalf("StagedDiff: %v", err) }
	if !strings.Contains(out, "rename from") || !strings.Contains(out, "rename to") {
		t.Fatalf("FR3e: expected compact rename (rename from/to), got:\n%s", out)
	}
	if strings.Contains(out, "rename from") && strings.Contains(out, "rename to") {
		// good; optionally assert similarity index 100%
	}
}
```
NOTE: -M's 50% similarity threshold ⇒ identical content = 100% = a pure rename (no patch body, just
`similarity index 100%` / `rename from` / `rename to`). Mirror the helpers in stagediff_test.go
(`initRepo`/`writeFile`/`stageFile`/`execGit`).

### (b) -U0 (DiffContext=0) → changed lines only
Stage a multi-line edit, call StagedDiff with DiffContext:0, assert the output has NO unchanged context
lines (only `+`/`-` changed lines + the `@@` hunk header). Contrast with DiffContext:1 (one anchor line).

```go
func TestStagedDiff_DiffContextZero_ChangedLinesOnly(t *testing.T) {
	repo := t.TempDir(); initRepo(t, repo)
	writeFile(t, repo, "a.go", "package main\nfunc a(){}\nfunc b(){}\nfunc c(){}\n")
	stageFile(t, repo, "a.go"); execGit(t, repo, "commit", "-qm", "base")
	writeFile(t, repo, "a.go", "package main\nfunc a(){}\nfunc B(){}\nfunc c(){}\n") // edit middle line only
	stageFile(t, repo, "a.go")
	g := New(repo)
	out, err := g.StagedDiff(context.Background(), StagedDiffOptions{DiffContext: 0})
	if err != nil { t.Fatalf("StagedDiff: %v", err) }
	// -U0: the unchanged "func a(){}" / "func c(){}" context lines must be ABSENT (no leading-space context).
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "  ") {
			// a single-leading-space line inside a hunk is a -U>0 context line; -U0 has none.
			// (guard against "diff --git" header lines which start with "d")
		}
	}
	// Simpler robust assertion: with -U1 (DiffContext:1) the unchanged "func a" line IS present; with -U0 it is NOT.
	out1, _ := g.StagedDiff(context.Background(), StagedDiffOptions{DiffContext: 1})
	if !strings.Contains(out1, "func a()") {
		t.Fatalf("DiffContext:1 should retain one anchor context line, got:\n%s", out1)
	}
	if strings.Contains(out, "func a()") {
		t.Fatalf("DiffContext:0 (-U0) should drop unchanged context (changed-lines-only), but got:\n%s", out)
	}
}
```
(The simplest faithful assertion: the unchanged `func a(){}` line appears at -U1 but NOT at -U0.)

### (c) Optional: -U1 default-shape test
If the implementer wants to pin the production default explicitly, add a test passing `DiffContext: 1`
asserting exactly one anchor context line each side of a change (mirrors git_diff_semantics §2's -U1 shape).

---

## 6. system_context §6 — the regression invariant (the acceptance criteria)

`system_context.md` §6 invariant 1 (lines 151-159) is AUTHORITATIVE for this task:
- **FR3e (-M), FR3g (skeleton), FR3h (index-strip) are ALWAYS ON** regardless of token_limit.
- **FR3f (-U1 default) REPLACES git's -U3 — this CHANGES existing golden tests even at token_limit==0.**
- "Existing test fixtures WILL change under FR3f (-U3→-U1) ... Update them; these are expected deltas,
  not regressions. The fixtures that assert on `diff --git a/<file>` boundary markers (kept) and the
  truncation sentinels (kept at token_limit==0) are the stability anchors."
- "Payload-only, never commit-affecting." (Every transform is on what the agent SEES; snapshot/commit untouched.)

So: -M/-U are UNCONDITIONAL (not `if token_limit > 0`). They apply at token_limit==0 (the default). This
matches the item's "FR3e/FR3f are ALWAYS ON (not gated on token_limit)." The helper does NOT check
opts.TokenLimit — it always emits -M/-U.

---

## 7. Scope fences (NOT this task)

- **NOT numstat skeleton (FR3g, M3)** — separate git call + prepend; not argv. (And deliberately NO -M on
  numstat — §3's `=>` notation.)
- **NOT index-line strip (FR3h, M2.T3)** — post-capture `^index ` line filter; separate task.
- **NOT token-limit gate / water-fill (FR3d/FR3i, M4)** — opts.TokenLimit/PromptReserveTokens are UNREAD
  here (the helper emits -M/-U unconditionally).
- **NOT binary.go** — detectBinaryFiles/fileStatuses keep their own argv (no -M/-U on numstat/name-status).
- **NOT the 6 call sites** — P1.M1.T2.S2 already maps cfg→opts (DiffContextValue). The opts the functions
  receive already carries the resolved DiffContext.
- **NOT docs** — DOCS: none (the user-facing diff_context knob was documented in P1.M1.T1.S4).
- **NOT -C** — explicitly rejected by FR3e (O(files²), no value).

---

## 8. Validation commands

```bash
gofmt -w internal/git/git.go internal/git/stagediff_test.go internal/git/treediff_test.go internal/git/workingtreediff_test.go
go vet ./internal/git/        # catches an unused opts param / a broken append / missing fmt import.
go build ./...
go test ./internal/git/ -run 'TestStagedDiff|TestTreeDiff|TestWorkingTreeDiff' -v   # the golden suites
go test -race ./...           # no regression (the 3 diff functions feed generate/decompose/hook)
git diff --exit-code go.mod go.sum
# Confirm -M/-U land in the 3 patch argv paths and NOT in binary.go's numstat/name-status:
grep -n '"-M"\|"-U' internal/git/git.go          # only inside buildDiffArgs (+ its doc)
grep -n '"-M"\|"--numstat"\|"--name-status"' internal/git/binary.go   # numstat/name-status UNCHANGED (no -M)
```
