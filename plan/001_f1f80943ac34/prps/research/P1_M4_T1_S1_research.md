# Research — P1.M4.T1.S1: internal/prompt/examples.go (history examples + multi-line detection)

## Source-of-truth contracts

- **Task contract** (`tasks.json` P1.M4.T1.S1): `FetchExamples(g, n=20) ([]string, hasMultiline bool, err)`.
  - if `CommitCount<=1` → `([],false,nil)` (new-repo path).
  - else take `RecentMessages(20)`, **trim blank lines**, **cap total at 100 lines**.
  - Multi-line detect = port the awk heuristic: **split the raw log on `---` separators; if ANY group has >1 non-empty line → hasMultiline=true**.
  - MOCKING: temp-repo seeded single-line-only → false; add one multi-line → true; trimming/cap asserted.
- **PRD FR10** = `git rev-list --count HEAD`; **FR11** = last 20 full msgs `git log --format="---%n%B" -20`, trimmed, capped at 100 lines; **FR12** = detect multi-line by scanning the examples.
- **PRD §17.1** renders the examples per-message between `---` separators: "(up to 20, ≤100 lines total)" → each `[]string` element = ONE commit message (group), blank-trimmed.

## Dependency (DONE: P1.M3.T4.S1) — exact signatures

From `internal/git/log.go`:
- `func (g *Git) CommitCount() (int, error)` — `git rev-list --count HEAD`; unborn repo → `(0, nil)`.
- `func (g *Git) RecentMessages(n int) (string, error)` — `git log --format=---%n%B -<n>`; returns RAW output **verbatim** (no trim, no cap, `---` separators intact, multi-line body blank lines INTACT); unborn repo → `("", nil)`.

`*git.Git` therefore satisfies a `HistoryReader{ CommitCount() (int,error); RecentMessages(n int) (string,error) }` interface via Go structural typing — this is the decoupling seam (prompt production code imports NOTHING from git; the generate layer injects `*git.Git`).

## The awk heuristic (reference_impl.md §6) — ported + empirically verified

Reference:
```
examples = git log --format='---%n%B' -20 | sed '/^$/d' | head -100
has_multiline = awk '/^---$/{ if(lines>1) found=1; lines=0; next } { lines++ } END { print found+0 }'
```
Go port = a scanner over the RAW `---%n%B` stream:
1. Split on lines.
2. A line whose trimmed content is exactly `---` is a group separator (flush current group).
3. A blank line (trimmed == "") is dropped (the `sed '/^$/d'` step).
4. Any other (non-blank) line is appended to the current group (kept verbatim, NOT trimmed — faithful to sed which only strips empty lines).
5. `hasMultiline = true` if ANY group (after dropping blanks) has >1 non-blank line.

### ★ CRITICAL GOTCHA — the reference awk MISSES the last group ★
The reference awk only sets `found=1` INSIDE the `/^---$/` action (i.e. when it sees the NEXT separator). The `END` block prints `found+0` but does NOT re-check the final accumulated `lines`. So if the OLDEST commit (the LAST group, which has no trailing `---` after it) is the ONLY multi-line commit, the awk prints **0** (verified empirically):

```
$ printf -- '---\nfix: newest single\n---\nfeat: oldest subject\n\noldest body\n' \
  | sed '/^$/d' | awk '/^---$/{ if(lines>1) found=1; lines=0; next } { lines++ } END { print found+0 }'
0
```
The task contract says "if ANY group has >1 non-empty line" — i.e. check ALL groups incl. the last. So the Go port MUST flush+check the final group (don't reproduce the awk bug). Verified the port intent returns 1 for that input.

### cap vs hasMultiline ordering
- The reference computes hasMultiline over the **head-100'd** stream.
- The task contract says "**split the raw log**" → compute hasMultiline over ALL groups from the full `RecentMessages(20)` (before the 100-line cap). Follow the CONTRACT: cap affects only the returned `examples`, NOT `hasMultiline`.

## Decisions for the []string semantics

- Each element = ONE commit message (group), blank-line-trimmed, lines joined by "\n".
  - single-line "fix: bug" → `"fix: bug"`.
  - multi-line "feat: x\n\nbody1\nbody2" → `"feat: x\nbody1\nbody2"` (intra-message blank line removed by the trim step).
- 100-line cap = **whole-message** accumulation (newest-first): include a group while `total + len(group) <= 100`; else stop. This keeps ≤100 total non-blank lines AND whole messages (cleaner than the reference's flat `head -100` which can cut a message mid-way; justified by the `[]string`-of-messages return type and PRD §17.1's per-message rendering).
- `hasMultiline` computed over ALL groups (pre-cap), per contract.

## Test strategy (matches project white-box + real-git philosophy)

`internal/prompt` does not exist yet → examples.go OWNS `// Package prompt` doc; tests are white-box `package prompt` (matches internal/ui, internal/provider, internal/git).

1. **Fake-based logic tests** (deterministic, exhaustive) — a `fakeReader{count int; raw string; err error}` implementing `HistoryReader` feeds CANNED raw `---%n%B` strings:
   - new-repo (count≤1) → `([],false,nil)`.
   - single-line-only history → `hasMultiline=false`, examples = the messages.
   - one multi-line group anywhere (incl. LAST group) → `hasMultiline=true`.
   - blank-line trimming (intra-message blanks removed).
   - 100-line cap (many groups → stop at 100 total lines; later groups dropped).
   - error propagation (CommitCount err, RecentMessages err).
2. **Real-git integration test** (contract "temp-repo seeded") — build a temp repo via os/exec git, seed single-line commits → `hasMultiline=false`; add one multi-line commit → `hasMultiline=true`; pass a real `*git.Git` (test-only import of internal/git — NO cycle: git does not import prompt).

## Validation commands (verified the toolchain: go1.26, module builds)
- `go build ./internal/prompt/`
- `go vet ./internal/prompt/`
- `test -z "$(gofmt -l internal/prompt/)"`
- `go test ./internal/prompt/`
- `go test ./...`

## Scope boundaries
- ONLY create `internal/prompt/examples.go` + `internal/prompt/examples_test.go`. Do NOT create system.go (S2), payload.go (S3). Do NOT modify git/ui/provider/main/Makefile/go.mod/go.sum. stdlib-only imports in production (strings). Test imports: stdlib (testing, os, os/exec, path/filepath, fmt) + internal/git (test-only). No go mod tidy.
