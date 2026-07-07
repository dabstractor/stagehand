# P1.M3.T2.S1 — Design Decisions

**Subtask**: Subject extraction + exact-match check + rejection-list retry logic
**File**: `internal/generate/dedupe.go` (`package generate`) + `internal/generate/dedupe_test.go`
**PRD**: §9.7 FR30–FR33 (snapshot lines 280–283), Appendix B.4 (lines 1374–1379)
**Scope**: TWO pure functions only — `IsDuplicate(subject string, recent []string) bool` and
`ExtractSubject(message string) string`. The retry LOOP, the `max_duplicate_retries` counter, the rescue
path, and the rejection-list assembly are the ORCHESTRATOR (P1.M3.T4) — explicitly NOT this subtask.

---

## §0 — The boundary: TWO primitives, NOT a loop

The work-item is explicit and MUST be respected:

> "This subtask provides the check + extract functions; the orchestrator wires the loop."
> "The retry LOOP itself lives in the orchestrator (P1.M3.T4) which calls IsDuplicate and, on match,
> appends the subject to a rejected list and calls BuildUserPayload with the rejection list."

So `dedupe.go` exports exactly two functions. There is NO retry function, NO counter, NO rescue call,
NO `BuildUserPayload` call, NO git fetch, NO config read here. Those are P1.M3.T4. Implementing them here
would (a) duplicate the orchestrator, (b) couple this pure-logic file to `git`/`config`/`prompt`, and
(c) violate the work-item's explicit scope. The deliverable is the two primitives the loop is built from.

The orchestrator's intended shape (for context only — do NOT implement here):

```go
recent, _ := g.RecentSubjects(ctx, 50)               // FR31 (P1.M1.T3.S4 — already built)
rejected := []string{}
for attempt := 0; ; attempt++ {
    payload := prompt.BuildUserPayload(diff, rejected) // P1.M3.T1.S3 (parallel — implementing now)
    raw, _ := exec.Execute(ctx, manifest.Render(...))   // P1.M2.T5/T4
    msg, ok, _ := provider.ParseOutput(raw, manifest)   // P1.M2.T6.S1
    if !ok { ... FR29 retry ... }
    subject := generate.ExtractSubject(msg)             // <-- THIS SUBTASK (FR30)
    if generate.IsDuplicate(subject, recent) {          // <-- THIS SUBTASK (FR32)
        rejected = append(rejected, subject)
        if attempt >= cfg.MaxDuplicateRetries { /* FR33 rescue (P1.M3.T3) */ break }
        continue                                        // retry with rejection list
    }
    break // accept
}
```

## §1 — The signatures (frozen, per work-item)

```go
func IsDuplicate(subject string, recent []string) bool
func ExtractSubject(message string) string
```

- Both EXPORTED (capitalized). The caller (orchestrator P1.M3.T4) is in the SAME package (`internal/generate`),
  so export is not strictly required for it — but the work-item writes them capitalized, and the public
  library API (P1.M3.T5 `pkg/stagecoach`) may re-export them. Match the contract verbatim.
- Neither returns an error — there is NO failure mode (pure transformations over in-memory strings).
  This mirrors `prompt.BuildUserPayload` (string-only, no error) and `prompt.DetectMultiline` (bool-only).
- `recent []string` (not a `map`/set type) — the caller passes the raw slice from `git.RecentSubjects`.
  IsDuplicate builds the set INTERNALLY (§3). This keeps the caller simple and the signature stable.

## §2 — ExtractSubject: "first line (split on \n, trim)" — faithful & efficient

PRD FR30: "Extract the generated subject (first line of the message)." The work-item paraphrase:
"first line (split on \n, trim)". Implement EXACTLY that: take the first line, trim it.

```go
func ExtractSubject(message string) string {
    // First line = everything up to the first '\n'. IndexByte is O(pos) and allocates nothing —
    // semantically identical to strings.Split(message, "\n")[0] but avoids building the full line slice.
    first := message
    if nl := strings.IndexByte(message, '\n'); nl >= 0 {
        first = message[:nl]
    }
    return strings.TrimSpace(first)
}
```

WHY IndexByte and not `strings.Split(message, "\n")[0]`:
- Identical RESULT (Split[0] == prefix-up-to-first-\n). Verified.
- Split allocates a `[]string` of EVERY line (the whole message is tokenized). A commit message is small,
  but the repo's style (see `git.go` `parseDiffTree`, `StagedDiff`, `normalizeNewlines` in `parse.go`)
  consistently prefers IndexByte/byte-scanning over full Split for the "first occurrence" case. Match it.
- `strings.SplitN(message, "\n", 2)[0]` would also be efficient and reads closer to the prose; IndexByte
  is chosen for zero allocation. Either is acceptable — the test suite is agnostic to the choice.

WHY TrimSpace on the first line (and NOT on the whole message first):
- The whole message is ALREADY trimmed by `provider.ParseOutput` (P1.M2.T6.S1, Step 5:
  `msg = strings.TrimSpace(msg)`) AND newline-normalized (Step 4: `\r\n`→`\n`). So leading newlines never
  reach ExtractSubject in the real pipeline. Trimming the whole message first is therefore redundant.
- The per-line TrimSpace IS needed for a subject line with TRAILING whitespace (e.g. `"fix: foo  \nbody"`):
  ParseOutput's whole-string TrimSpace does NOT touch mid-string trailing spaces, so the first line may
  carry trailing spaces. TrimSpace cleans them. This is the faithful "trim" of the work-item.
- Be literal to the work-item: "split on \n, trim" = `TrimSpace(firstLine)`. Do NOT trim the whole message
  first (that would deviate from the literal spec; and a test that passes a leading-newline message should
  expect "" from Split[0] — matching commit-pi's `head -1` behavior, NOT a skip-blank behavior).

EDGE CASES (all covered by tests):
- `""` → `""` (IndexByte returns -1; TrimSpace("") = "").
- `"fix: foo"` (single line) → `"fix: foo"`.
- `"fix: foo\n\nbody"` → `"fix: foo"` (body excluded — this is the POINT of FR30).
- `"fix: foo  \nbody"` → `"fix: foo"` (trailing spaces trimmed).
- `"  fix: foo\nbody"` → `"fix: foo"` (leading spaces on the line trimmed — note: ParseOutput's
  whole-string TrimSpace already stripped these, but the per-line TrimSpace is belt-and-suspenders).
- `"fix: foo\r\nbody"` (defensive — ParseOutput converts `\r\n`→`\n` so this won't arrive, but if a
  caller bypasses ParseOutput): IndexByte finds `\n` (the `\r` is at nl-1), `message[:nl]` =
  `"fix: foo\r"`, TrimSpace strips the trailing `\r` → `"fix: foo"`. Robust.

## §3 — IsDuplicate: exact match via a map (O(1) lookup), case-sensitive

PRD FR32: "If the subject exactly matches one of the 50, retry …". commit-pi uses `grep -Fxq` — exact
whole-LINE match, case-SENSITIVE (no `-i`), NOT a substring/prefix match. The Go port builds a
`map[string]struct{}` (set) from `recent` and does a single map lookup (O(1)).

```go
func IsDuplicate(subject string, recent []string) bool {
    set := make(map[string]struct{}, len(recent))
    for _, s := range recent {
        set[s] = struct{}{}
    }
    _, dup := set[subject]
    return dup
}
```

WHY a map and not `slices.Contains` / a linear scan:
- The work-item explicitly says "build a set from recent, check exact match" and "Go uses a map/set for
  O(1) lookup". A map IS the set. `slices.Contains` would be O(n) per lookup — fine for n=50, but the
  work-item mandates the set, so honor it (and it future-proofs if `recent` grows).
- The set is built per-call. The orchestrator calls IsDuplicate once per attempt (≤4 calls total for
  default 3 retries). Building a 50-entry map 4 times is ~microseconds — negligible. Do NOT cache the set
  across calls here (the signature takes `recent []string` each call; caching would require state the
  function doesn't own — that's the orchestrator's concern if it ever wants it).

WHY case-SENSITIVE (no lowercasing):
- `grep -Fxq` is case-sensitive by default. "exactly matches" (FR32) = byte-for-byte equality. Lowercasing
  both sides would treat `"Fix: foo"` and `"fix: foo"` as duplicates — that is NOT what FR32 specifies.
  A near-duplicate differing only in case is a DIFFERENT commit subject; rejecting it would be wrong.
- A test pins this: `IsDuplicate("Fix: Foo", []string{"fix: foo"}) == false`.

WHY exact (not substring/prefix):
- `grep -Fxq`'s `-x` flag means "match the whole line". So `"fix: foo"` must EQUAL a recent subject
  exactly — `"fix: foobar"` (prefix) is NOT a match, `"x fix: foo y"` (substring) is NOT a match.
- A test pins this: `IsDuplicate("fix: foobar", []string{"fix: foo"}) == false`.

WHY NO trimming inside IsDuplicate:
- Both inputs are pre-trimmed by their producers:
  - `subject` comes from `ExtractSubject`, which TrimSpaces the first line (§2).
  - `recent` comes from `git.RecentSubjects` (P1.M1.T3.S4), which TrimSpaces each line and skips empties
    (see git.go: `s := strings.TrimSpace(line); if s == "" { continue }`).
- So a plain `set[subject]` lookup is correct — adding defensive trimming would mask a producer bug.
  Document the contract; trust the upstream guarantees (same philosophy as `prompt.BuildUserPayload`
  trusting that `rejected` elements are single-line per FR30).

EDGE CASES (all covered by tests):
- Match present → `true`.
- No match → `false`.
- `recent == nil` or `len==0` → `false` (empty set; `make(map, 0)` is fine, lookup misses).
- `subject == ""` → `false` (RecentSubjects never stores "" — it skips empties — so "" is never a key;
  and an empty subject is the ParseOutput `ok==false` path, not a duplicate).
- Match is in the MIDDLE of the slice → `true` (set membership is order-independent).
- Duplicate entries in `recent` → still `true` (set dedups; lookup unaffected).

## §4 — Package, imports, file placement

- `package generate` (the orchestrator P1.M3.T4 lands in this same package later). `internal/generate/`
  is currently EMPTY — so `dedupe.go` + `dedupe_test.go` are the FIRST files in the package. There is
  NO merge-collision risk (unlike S1/S2/S3 in `internal/prompt`, which had to coordinate file edits).
- Imports: `dedupe.go` imports `"strings"` ONLY (IndexByte + TrimSpace). IsDuplicate needs NO import
  (builtin `map`/`range`). `dedupe_test.go` imports `"strings"` + `"testing"`. NO `"fmt"`, NO `internal/*`,
  NO third-party. `go mod tidy` MUST be a no-op; `git diff --exit-code go.mod go.sum` MUST be empty.
- `internal/generate` stays a stdlib-only LEAF for this subtask. (Later, the orchestrator T4 will import
  `git`/`provider`/`prompt`/`config` — but that is T4's import graph, not dedupe.go's.)
- Naming: file `dedupe.go` (the work-item names it). Functions `IsDuplicate` / `ExtractSubject` (exported,
  per work-item). Test file `dedupe_test.go`, `package generate` (white-box, same as parse_test.go).

## §5 — Test strategy: table-driven pure-function tests (mirror parse_test.go)

dedupe.go has NO git dependency, NO I/O, NO subprocess. So tests are PURE-FUNCTION table tests in the
style of `internal/provider/parse_test.go` and `internal/prompt/system_test.go` — NOT the temp-repo style
of `internal/git/*_test.go` (those need a real git repo; dedupe does not). No `initRepo`/`makeEmptyCommit`,
no `t.TempDir()`, no mocking.

`dedupe_test.go` (`package generate`, white-box):

- `TestExtractSubject` — table: multi-line (body excluded), single-line, empty, trailing spaces trimmed,
  `\r\n` defensive, leading-whitespace line, whitespace-only message.
- `TestIsDuplicate` — table: match present, no match, nil recent, empty subject, case-sensitive,
  exact-not-substring, match-in-middle, duplicate-in-recent.
- Pin the FR30 contract ("subject = first line") and the FR32 contract ("exactly matches" = case-sensitive,
  whole-subject equality) with explicit assertions + inline PRD citations (mirror parse_test.go's
  comment-per-case style).

No `near`/`suffix` helpers needed (return types are `bool`/`string`; `strings.Contains` suffices). Do NOT
redeclare helpers from other packages (they don't exist in `package generate` anyway).

## §6 — What this subtask does NOT touch (frozen files)

Do NOT edit: `internal/prompt/*` (S1/S2/S3 own the prompt layer; S3's payload.go is being implemented in
parallel RIGHT NOW), `internal/git/*` (RecentSubjects already built — read-only contract), `internal/provider/*`
(ParseOutput already built — read-only contract), `internal/config/*` (MaxDuplicateRetries default 3 —
read-only contract, owned by the orchestrator's retry loop, not by dedupe), `cmd/*`, `pkg/*`, `Makefile`,
`go.mod`, `go.sum`. Only `internal/generate/dedupe.go` + `dedupe_test.go` are created.

## §7 — Upstream/downstream contracts (consume, do not implement)

UPSTREAM (inputs — already built, read-only):
- `git.RecentSubjects(ctx, n int) ([]string, error)` (P1.M1.T3.S4) → the `recent` arg. Returns up to n
  single-line, TrimSpace'd, newest-first subjects; `nil` on unborn/non-repo (exit 128). Default n=50 (FR31).
- `provider.ParseOutput(raw, m Manifest) (msg string, ok bool, fellback bool)` (P1.M2.T6.S1) → the `message`
  arg to ExtractSubject. Step 4 normalizes `\r\n`→`\n` + collapses 3+`\n`→2; Step 5 TrimSpaces. So msg is
  trimmed & newline-normalized on arrival.

DOWNSTREAM (consumers — not yet built, just honor the signature):
- Orchestrator P1.M3.T4 (`CommitStaged`) — calls `ExtractSubject(msg)` then `IsDuplicate(subject, recent)`
  in its retry loop (§0 sketch). On match it `append`s the subject to `rejected` and calls
  `prompt.BuildUserPayload(diff, rejected)` (P1.M3.T1.S3, parallel) for the next attempt.
- Public API P1.M3.T5 (`pkg/stagecoach`) — may re-export the two functions.

=> The `IsDuplicate(subject string, recent []string) bool` and `ExtractSubject(message string) string`
signatures are FROZEN after this subtask.
