# Research: P1.M1.T2.S5 — UpdateRefCAS (3-arg compare-and-swap)

Empirical validation of `git update-ref HEAD <new> <expected-old>` CAS semantics on
**git 2.54.0**, plus the design decisions the implementing agent must not have to re-derive.

## 1. Interface vs. contract-prose: the signature is `ref, newSHA, expectedOld`

The work-item CONTRACT prose writes `UpdateRefCAS(ctx, newSHA, expectedOld) error` (2 value args).
But the `Git` **interface** (already landed by S1, authoritative — exactly like S4's `parents []string`
overriding the prose `parentSHA`) declares:

```go
UpdateRefCAS(ctx context.Context, ref, newSHA, expectedOld string) error
```

**Resolution:** the interface is authoritative. The implementation builds args
`update-ref <ref> <newSHA> <expectedOld>` and forwards `ref` verbatim. The orchestrator
(P1.M3.T4) passes `ref = "HEAD"`. `ref` is NOT hardcoded to "HEAD" inside UpdateRefCAS — that would
violate the interface contract and prevent reuse on other refs (e.g. tags in v2). (Decision D1.)

## 2. Empirical `update-ref` exit codes (the single most important finding)

Run against real repos on git 2.54.0 (see transcript in §6):

| Scenario | Command | Exit | Stderr (verbatim) |
|---|---|---|---|
| CAS success (expected == current HEAD) | `update-ref HEAD C1 C0` when HEAD==C0 | **0** | _(empty)_ |
| CAS failure: stale expected (HEAD moved) | `update-ref HEAD C1 C0` when HEAD==C1 | **128** | `fatal: update_ref failed for ref 'HEAD': cannot lock ref 'HEAD': is at <actual> but expected <expected>` |
| all-zeros expected on BORN repo | `update-ref HEAD C1 000…0` when HEAD exists | **128** | `fatal: update_ref failed for ref 'HEAD': cannot lock ref 'HEAD': reference already exists` |
| all-zeros expected on UNBORN repo (root publish) | `update-ref HEAD C0 000…0` when unborn | **0** | _(empty)_ — success |
| bad newSHA (not a valid object) | `update-ref HEAD <bad> <expected>` | **128** | `fatal: update_ref failed for ref 'HEAD': cannot update ref 'refs/heads/main': trying to write ref 'refs/heads/main' with nonexistent object <bad>` |
| 2-arg FORCE form (NEVER use) | `update-ref HEAD C0` | 0 | always wins, even if HEAD moved |

### 2.1 CRITICAL DISCREPANCY with `git_plumbing_summary.md`

The Exit-Code Cheat Sheet in `git_plumbing_summary.md` lists:

> | `git update-ref HEAD <new> <old>` | CAS success | **CAS mismatch (Exit 1)** | — |

This is **WRONG** on git 2.54.0: CAS mismatch returns **exit 128, not 1**. The cheat-sheet author
likely guessed by analogy with `diff --quiet` (which does use exit 1). **This is exactly the trap
FINDING 3 warns about.** The contract is explicit and correct: *"Treat exit code ≠ 0 as the
CAS-failure signal."* The implementation MUST branch on `code != 0` and MUST NOT branch on
`code == 1` (or `code == 128`). (Decision D2.)

### 2.2 Why the stderr MUST be included in the error but NEVER matched

FINDING 3 lists two distinct stderr phrasings; §2 above adds a third ("nonexistent object"). They vary
by **scenario** (moved vs. already-exists vs. bad-sha) and will vary by **git version**. So:
- DETECTION of CAS failure = `code != 0` (stable signal). NEVER `strings.Contains(stderr, …)`.
- The trimmed stderr is still EMBEDDED in the error message for human/diagnostic value (it is
  git's own phrasing, useful in verbose mode / logs). The orchestrator detects via `errors.Is`, not
  by reading the string. (Decision D3.)

## 3. The typed error: a sentinel `ErrCASFailed` (wrapped, not a struct)

### 3.1 What the contract requires

> "On exit != 0 → return a typed error (e.g. ErrCASFailed) that the orchestrator can detect to print
> 'HEAD moved from <expected> to <actual>'."

The orchestrator needs to **detect** CAS failure distinctly from infrastructural failures (git binary
missing, context cancelled) so it can emit PRD §13.5's specific message and exit 1 (vs. a generic
error / different exit).

### 3.2 Design choice: exported sentinel var + `fmt.Errorf("%w", …)`

```go
// ErrCASFailed is returned by UpdateRefCAS when git's compare-and-swap did not match — i.e. HEAD
// moved concurrently (or expectedOld was the all-zeros hash on a repo that already has commits).
// The orchestrator detects it via errors.Is(err, ErrCASFailed) to emit PRD §13.5's "HEAD moved
// from <expected> to <actual>" message and exit 1. It is NOT returned for infrastructural failures
// (missing git binary, cancelled context); those propagate the underlying error unchanged so they
// remain distinguishable. The <actual> SHA is re-read by the orchestrator via RevParseHEAD (it is
// deliberately NOT captured here — see decision D5).
var ErrCASFailed = errors.New("git update-ref: compare-and-swap failed (ref moved since snapshot)")
```

Wrapped on the failure branch:
```go
return fmt.Errorf("%w (exit %d): %s", ErrCASFailed, code, strings.TrimSpace(stderr))
```
so `errors.Is(err, ErrCASFailed)` is `true`, and the error string still carries the exit code +
trimmed stderr for diagnostics.

### 3.3 Why a sentinel, not a struct carrying `<actual>`

Considered: a `type CASFailedError struct{ Ref, Expected, Actual string }` that re-reads HEAD
internally to populate `Actual`. **Rejected** for three reasons (decision D5):
1. **Separation of concerns:** the `internal/git` package is a plumbing layer; it does not construct
   user-facing messages or decide exit codes. Surfacing `<actual>` is an orchestrator/CLI concern
   (P1.M3.T4 / P1.M4.T3). Keeping the error dumb preserves the v2 decomposition boundary (PRD §11.3).
2. **Inherent staleness:** any `actual` captured here is immediately stale (HEAD could move again
   before the orchestrator prints). Re-reading at print time is no worse and keeps the plumbing call
   cheap/O(1) — no second git round-trip inside the "publish" primitive.
3. **Testability:** a sentinel makes the unit tests trivial (`errors.Is(err, ErrCASFailed)`); a struct
   would require the test to also stub/observe RevParseHEAD, coupling S5 to S2's internals.

This mirrors how S2/S3/S4 return plain `(value, error)` / `error` — S5 is simply the first to add ONE
exported sentinel because CAS-failure is the one outcome the orchestrator must branch on.

### 3.4 Infrastructural errors are NOT wrapped

`run()` returns `err != nil` (exitCode == -1) for LookPath miss / context cancel / start failure.
UpdateRefCAS returns that `err` **unchanged** — do NOT wrap it in `ErrCASFailed`. A missing git
binary is not a CAS failure; the orchestrator must be able to tell them apart. The branch order is
`if err != nil { return err }` FIRST, then `if code != 0 { return ErrCASFailed-wrapped }`. This is
the same ordering invariant S2/S3/S4 use (gotcha G4 of S4). (Decision D6.)

## 4. The all-zeros hash (root-commit / unborn CAS)

- Contract: *"For root commit: expected-old = all-zeros hash (40 zeros for sha-1)."*
- The CALLER (orchestrator, P1.M3.T4) constructs the all-zeros string and passes it as `expectedOld`
  when `RevParseHEAD` reported `isUnborn`. UpdateRefCAS itself has NO knowledge of `isUnborn` — it
  forwards `expectedOld` to git verbatim (mirrors S4, where `CommitTree` does not know `isUnborn`;
  the caller decides `parents`). (Decision D7.)
- Verified empirically: `update-ref HEAD <root-commit> 000…0` on an unborn repo → **exit 0** (the
  root commit is published). And `update-ref HEAD <any> 000…0` on a BORN repo → **exit 128** ("reference
  already exists"). Both are exactly the CAS contract: expected-old == all-zeros ⟺ "ref must be
  unborn." (§2 transcript.)
- sha-1 = 40 zeros (`0000000000000000000000000000000000000000`). sha-256 would be 64 zeros, but
  sha-256 repos are experimental and out of v1 scope; the tests pin 40 zeros (the default repo hash).
  S5 does NOT define an exported `ZeroSHA` constant — that is the orchestrator's concern, and a
  sha-1-specific constant would be subtly wrong on sha-256. (Decision D8.)

## 5. `run()` (NOT `runWithInput`) — update-ref takes no stdin

`git update-ref HEAD <new> <old>` reads nothing from stdin. So UpdateRefCAS delegates to S1's `run()`
helper (the base shell-out), NOT S4's `runWithInput`. `run()` is byte-identical to its landed form
(S2/S3/S4 forbid modifying it). No new helper is needed for S5. (Decision D9.) This means S5 adds
ZERO new imports to git.go — `errors`, `fmt`, `strings` are all already present (verified: the import
block is `bytes, context, errors, fmt, io, os/exec, strings`).

## 6. Empirical transcript (git 2.54.0)

```
$ # success (expected == current)
$ git update-ref HEAD C1 C0; echo EXIT=$?      # HEAD==C0
EXIT=0

$ # failure (stale expected; HEAD moved to C1 after capturing expected=C0)
$ git update-ref HEAD C1 C0 2>&1; echo EXIT=$?
fatal: update_ref failed for ref 'HEAD': cannot lock ref 'HEAD': is at <C1> but expected <C0>
EXIT=128

$ # all-zeros expected on BORN repo
$ git update-ref HEAD C1 000…0 2>&1; echo EXIT=$?
fatal: update_ref failed for ref 'HEAD': cannot lock ref 'HEAD': reference already exists
EXIT=128

$ # all-zeros expected on UNBORN repo (root publish)
$ git update-ref HEAD C0 000…0; echo EXIT=$?     # unborn repo
EXIT=0

$ # bad newSHA (nonexistent object)
$ git update-ref HEAD ffff…ff1 <expected> 2>&1; echo EXIT=$?
fatal: update_ref failed for ref 'HEAD': cannot update ref 'refs/heads/main': trying to write ref 'refs/heads/main' with nonexistent object ffff…ff1
EXIT=128
```

## 7. Test design matrix (6 cases)

| Test | Fixture | Key assertions | Proves |
|---|---|---|---|
| `TestUpdateRefCAS_Success` | initRepo + makeEmptyCommit×2 → HEAD=C1; `newCommit`=dangling child of C1 | `err==nil`; `casHEAD==newCommit` | CAS success path (FR40) |
| `TestUpdateRefCAS_StaleExpected` | initRepo + makeEmptyCommit×2 → HEAD=C1; capture expected=C1; `casMoveHEAD(C0)`; call with expected=C1 | `errors.Is(err, ErrCASFailed)`; err msg contains "(exit 128)"; `casHEAD==C0` (unchanged) | **The core race**: HEAD moved during generation → clean abort, repo untouched (§18.1, §18.2) |
| `TestUpdateRefCAS_RootCommit` | initRepo (unborn) + `casCommit(parents=nil)` → dangling root; expected=zeros | `err==nil`; `casHEAD==rootCommit` | all-zeros + unborn ⇒ publish succeeds (§13.5 root edge) |
| `TestUpdateRefCAS_AllZerosOnBornRepo` | initRepo + makeEmptyCommit → HEAD=C0; expected=zeros | `errors.Is(err, ErrCASFailed)` | all-zeros + born ⇒ fails ("reference already exists") |
| `TestUpdateRefCAS_GitBinaryMissing` | `t.Setenv("PATH","")` | err contains "git binary not found"; `!errors.Is(err, ErrCASFailed)` | infrastructural failure NOT misreported as CAS failure (gotcha D6) |
| `TestUpdateRefCAS_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)`; `!errors.Is(err, ErrCASFailed)` | ctx.Err() surfaced, not wrapped as CAS failure |

## 8. Helper-name collision avoidance with S4 (parallel landing)

S4 (landing concurrently) plans these `package git` test helpers in `committree_test.go`:
`setIdentityConfig`, `writeFile`, `stageFile`, `writeTreeOf`, `headSHA`, `commitMessage`.
S5's helpers MUST have distinct names or the package won't compile when both land. S5 uses a `cas`
prefix throughout (decision D10):

- `casCommit(t, dir, parents, msg) string` — dangling commit via raw `git commit-tree` (explicit
  identity env; `-F -` stdin; tree resolved from first parent or the empty-tree SHA). Distinct from
  S4's `commitMessage` (a reader) and from anything in S2/S3.
- `casHEAD(t, dir) string` — raw `git rev-parse HEAD` trimmed. Distinct from S4's `headSHA`.
- `casMoveHEAD(t, dir, sha)` — raw 2-arg `git update-ref HEAD <sha>` (force) to simulate the
  concurrent HEAD move. Distinct (no S4 equivalent).
- `casOut(t, dir, args...) string` — tiny raw-git stdout helper used by the above.
- `gitIdentityEnv() []string` — `append(minGitEnv(), GIT_AUTHOR_*/GIT_COMMITTER_* …)`. Reuses S2's
  `minGitEnv()`; distinct name.

Reused (NOT redeclared): `initRepo` (git_test.go), `minGitEnv` + `makeEmptyCommit` (revparse_test.go).

## 9. Decisions log

- **D1** — Interface authoritative: signature is `(ctx, ref, newSHA, expectedOld)`, NOT the prose's
  `(ctx, newSHA, expectedOld)`. `ref` forwarded verbatim; never hardcoded.
- **D2** — Branch on `code != 0`, NOT `code == 1` (cheat-sheet is wrong; real exit is 128).
- **D3** — Embed trimmed stderr in the error for diagnostics; NEVER match it for detection.
- **D4** — Sentinel `var ErrCASFailed`, wrapped via `fmt.Errorf("%w", …)`; `errors.Is` works.
- **D5** — Sentinel, not a struct; `<actual>` is re-read by the orchestrator (separation of concerns).
- **D6** — Infrastructural errors (`run` err != nil) returned UNWRAPPED; only `code != 0` wraps
  `ErrCASFailed`. Branch order: `err` first, then `code`.
- **D7** — UpdateRefCAS has no `isUnborn` knowledge; caller passes all-zeros as `expectedOld`.
- **D8** — No exported `ZeroSHA` constant (orchestrator's concern; sha-1-specific would mislead).
- **D9** — Delegates to `run()` (no stdin); `runWithInput` unused. Zero new imports.
- **D10** — `cas`-prefixed helper names to avoid collision with S4's planned helpers.
