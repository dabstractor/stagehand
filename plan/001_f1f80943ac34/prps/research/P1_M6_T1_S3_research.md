# Research — P1.M6.T1.S3: internal/generate/rescue.go — rescue protocol (FR43–FR45)

## 1. What this task builds

A single exported render function in the existing `internal/generate` package:

```go
func Rescue(out *ui.Output, tree, parent, candidate string)
```

It prints the PRD §18.3 rescue block to the user when a commit was NOT created after
a snapshot was taken (TREE_SHA set, NEW_SHA not). It is a PURE RENDER function — no git,
no exec, no os.Exit, no return value. The exit code (`ui.ExitRescue == 3`) is set by the
CALLER (CommitStaged failure paths P1.M6.T1.S1 + the signal handler P1.M6.T2.S1).

## 2. Sources of truth (read in this order)

| Source | Section | What it pins |
|---|---|---|
| `PRD.md` | §18.3 | The verbatim rescue message block (failure notice, snapshot line, `Tree ID:`, manual command, omit note). |
| `PRD.md` | §18.2 table | Which failures trigger rescue (timeout, SIGINT/SIGTERM post-snapshot, parse-fail-after-retries, dup-exhaustion) vs which DON'T (CAS failure → its own message + exit 1). |
| `PRD.md` | §18.4 | Signal handler calls the rescue path if the snapshot was taken. |
| `PRD.md` | §9 FR43/FR44/FR45 | FR43 rescue condition; FR44 print notice + TREE_SHA + exact manual command; FR45 SIGINT/SIGTERM → rescue. |
| `architecture/reference_impl.md` | §5 | The proven `handle_error()` message to port (the baseline). PRD §18.3 governs wording; reference is the behavioral baseline. |
| `architecture/decisions.md` | §3 pseudocode | `RESCUE` call sites (err/timeout, !ok parse, exhausted dups) — shows Rescue is reached from the generate orchestrator; §3 does NOT put loop control in rescue.go. |
| `architecture/decisions.md` | §9 porting map | `handle_error() rescue → internal/generate/rescue.go: verbatim message + PRD enrichment (candidate msg)`. |

## 3. The exact message to print (reconciled)

The block (PRD §18.3 governs; reference_impl.md §5 is the proven baseline; the work-item
CONTRACT gives the command template + candidate wording — contract wins on those):

```
❌ Commit generation failed.
------------------------------------------------------------
Your staged files were safely snapshotted before generation.
Tree ID: <tree>

To commit the originally staged files manually:
  <COMMAND>

(omit "-p <parent>" if this is the repository's first commit)
------------------------------------------------------------
```

Then, ONLY if `candidate != ""`:
```
A candidate message was produced but rejected: "<candidate>". You can use it manually in the command above.
```

### <COMMAND> is DYNAMIC on `parent` (★ the root-commit enrichment ★)
- `parent != ""` (non-root):
  `git commit-tree -p <parent> -m "Your message" <tree> | xargs git update-ref HEAD`
- `parent == ""` (root commit / unborn repo — git.RevParseHEAD returns hasParent=false, PARENT_SHA=""):
  `git commit-tree -m "Your message" <tree> | xargs git update-ref HEAD`   (NO `-p`)

This is the work-item enrichment over the reference script (which always showed `-p
$PARENT_SHA`). The omit-note is a STATIC hint (literal `<parent>` placeholder, NOT
interpolated — interpolating an empty root parent would read `omit "-p "` which is broken).

## 4. Stream routing — STDERR, not stdout (FR51)

`ui.Output` (internal/ui/output.go, P1.M1.T2.S2) enforces the FR51 stream discipline:
- `Progressf` → **stderr** ALWAYS (human-facing progress/diagnostics).
- `Resultf` → **stdout** (ONLY the commit result `[sha] subject` + diff-tree).
- `Verbosef` → stderr, gated by verbose.

The rescue block is a FAILURE/RECOVERY notice — NOT a commit result. Therefore it MUST be
written via `out.Progressf(...)` (stderr). Writing it to `Resultf`/stdout would corrupt
`stagehand --dry-run | tee` pipelines (FR51). This is a hard correctness property: a rescue
test MUST assert the block appears on the captured stderr AND that stdout stays clean.

## 5. Color

`ui.Output.Red(s)` wraps `s` in the red SGR sequence when `o.color` is true; returns `s`
unchanged when color is off (NO_COLOR set, non-TTY/piped, or `--no-color`). The `❌`
failure-notice line MAY be wrapped via `out.Red(...)` to match the reference's red emphasis.
Tests MUST construct the Output with `noColor=true` (`ui.NewOutput(..., false, true)`) so the
captured text is plain and substring assertions hold regardless of the Red wrapper. Color is
ORTHOGONAL to routing — the routing methods do not auto-colorize; callers wrap tokens.

## 6. ui.Output API used (verified in internal/ui/output.go)

```go
func NewOutput(stdout, stderr io.Writer, verbose, noColor bool) *Output
func (o *Output) Progressf(format string, args ...any) error   // ← rescue writes here (stderr)
func (o *Output) Red(s string) string                           // optional failure-notice styling
const ExitRescue int = 3                                        // set by CALLER, NOT by Rescue
```
`Output` fields are UNEXPORTED → an external/white-box test constructs it via `ui.NewOutput`
(cannot use a struct literal from `package generate`).

## 7. Test construction & assertions (the contract MOCKING matrix)

`internal/generate/rescue_test.go` — white-box `package generate` (house convention, matches
dedupe_test.go even for exported fns). Imports: stdlib `bytes`, `io`, `strings`, `testing` +
`github.com/dustin/stagehand/internal/ui`. No testify, no real git, no exec.

Construction (noColor=true so color is off → plain captured text):
```go
var stderr bytes.Buffer
o := ui.NewOutput(io.Discard, &stderr, false, true) // stdout=discard, verbose=false, noColor=true
Rescue(o, tree, parent, candidate)
got := stderr.String()
```

Contract MOCKING bullets → assertions:
1. **tree SHA present**: `strings.Contains(got, tree)`.
2. **manual command present**: `strings.Contains(got, "git commit-tree") && strings.Contains(got, "xargs git update-ref HEAD")`.
3. **omit-(-p) note present**: `strings.Contains(got, "first commit")` (static note text).
4. **candidate line present iff candidate != ""**:
   - candidate="" → `!strings.Contains(got, "candidate message")`.
   - candidate="feat: x" → `strings.Contains(got, "A candidate message was produced but rejected:") && strings.Contains(got, candidate)`.
5. **root (parent=="") omits -p in the printed COMMAND**: `!strings.Contains(got, "commit-tree -p")` (checks the command adjacency; the static note's `-p` is NOT adjacent to `commit-tree`, so this isolates the command). Non-root → `strings.Contains(got, "commit-tree -p")`.
6. **FR51 routing**: assert stdout stays clean — pass a separate stdout buffer, assert it does NOT contain "Commit generation failed".

### Why `commit-tree -p` adjacency (not a global `-p` check) for the root test
The static omit-note literally contains `-p` (`(omit "-p <parent>" if...)`). A global
`!strings.Contains(got, "-p")` for root would FALSE-FAIL because of the note. The note never
contains `commit-tree`, so `commit-tree -p` is unambiguous to the command line.

## 8. Exact command-substring assertions (most precise)
- non-root: `strings.Contains(got, fmt.Sprintf("commit-tree -p %s -m \"Your message\" %s", parent, tree))`.
- root:     `strings.Contains(got, fmt.Sprintf("commit-tree -m \"Your message\" %s", tree))`.

## 9. Scope / boundaries
- ONLY create `internal/generate/rescue.go` + `internal/generate/rescue_test.go`.
- `internal/generate` package ALREADY EXISTS (dedupe.go, P1.M6.T1.S2). rescue.go is ADDED to it.
- Use a PLAIN `package generate` line + file-level comment DEFERRING the `// Package generate`
  doc to generate.go (P1.M6.T1.S1, not yet created) — EXACTLY as dedupe.go does (precedent:
  internal/git/log.go defers to git.go).
- Imports: stdlib `fmt` + `github.com/dustin/stagehand/internal/ui` ONLY. No go-git, no testify,
  no new dep. Do NOT modify go.mod/go.sum. No `go mod tidy`.
- Do NOT create generate.go (S1 CommitStaged), the signal handler (M6.T2.S1), or the integration
  harness (M6.T3). Do NOT call os.Exit from Rescue. Do NOT modify ui, git, prompt, provider, config.

## 10. Confidence
9/10. Pure render function, fully specified message, no git/I/O beyond the injected writer,
clear test matrix. Minor residual: exact phrasing of the static omit-note (I chose the PRD §18.3
wording with a literal `<parent>` placeholder) — but the MOCKING bullets only require the note be
PRESENT (substring "first commit"/"omit"), so wording flexibility doesn't threaten test success.
