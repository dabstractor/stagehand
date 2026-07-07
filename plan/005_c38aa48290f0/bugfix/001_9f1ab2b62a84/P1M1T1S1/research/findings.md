# Research Notes — P1.M1.T1.S1 (lazygit foreign-key WARNING)

## Scope
Add a **best-effort foreign-key conflict WARNING** to `lazygitEntry.Install()` (internal/cmd/integrate_lazygit.go)
BEFORE the existing `integrate.Apply(ActionUpsert)` call. Mirrors `gitAliasEntry.Install()`'s foreign-conflict
surfacing (FR-I4 parity + §9.21 no-mangle promise). No signature change; the write still flows through Apply's
no-mangle protocol.

## Key source anchors (VERIFIED line numbers in current tree)
- `internal/cmd/integrate_lazygit.go`
  - `lazygitEntry.Install()` — **line 302** (item says ~163; current file = 302). Delegates entirely to `integrate.Apply`.
  - `lazygitTarget.findKeyItem(key string) *yaml.Node` — **line 193**. Finds an UNMARKED item whose key value == key. Already used by Status().
  - `lazygitTarget.Parse(data []byte) error` — **line 67**.
  - `lazygitEntry.Status()` — **line 283** (existing foreign probe: `tgt.findKeyItem(e.key) != nil → StatusForeign`).
  - `lazygitEntry.resolvedPath()` — **line 336**.
- `internal/integrate/protocol.go`
  - `Apply()` final outcome assignment — **lines 228–235**. See CRITICAL FINDING below.
  - `ApplyOptions.Out io.Writer` (nil ⇒ os.Stderr); `Apply` resolves nil itself (line ~110).

## Pattern to mirror (git-alias foreign warning)
- `internal/cmd/integrate_gitalias.go` `gitAliasEntry.Install()` — nil-Out resolution at top of method:
  `out := opts.Out; if out == nil { out = os.Stderr }`. The foreign warning is then appended to `preview`.
  - NOTE the structural difference: git-alias owns its own preview+confirm (does NOT use Apply), so its
    WARNING rides inside the `preview` string → visible to the Confirm func's `diff` arg.
  - lazygit DELEGATES diff+confirm to `integrate.Apply`, so it CANNOT inject into the preview; it writes the
    WARNING directly to `opts.Out` BEFORE the Apply call. This is the contract's chosen mechanism (item §3c).

## Test patterns (VERIFIED)
- `internal/cmd/integrate_lazygit_test.go`
  - `newIsolatedLazygitEntry(t, key)` — sets `configPath` to a temp file (never touches real config).
  - `TestLazygitEntry_Install_ConfirmReceivesDiff` — **line 577**. Captures diff via Confirm func; pattern for interactive test.
  - `TestLazygitEntry_Status_States` — **line 671**. Contains `foreignYAML := ...` at **line 701** (unmarked `<c-a>` entry).
  - Existing `TestLazygitEntry_Install_Creates` passes `InstallOptions{Yes: true}` with **Out omitted (nil)** → see nil-Out gotcha.
- `internal/cmd/integrate_gitalias_test.go`
  - `TestGitAlias_Install_ForeignConflictInPreview` — **line 208**. Asserts preview contains "WARNING".
- testdata: `internal/cmd/testdata/lazygit/golden_input.yml` (key `'b'`, NOT `<c-a>` → no conflict; use for NoForeignNoWarning).

## ⚠️ CRITICAL FINDING — CONTRACT CORRECTION (OutcomeCreated → OutcomeUpdated)
The item description (§6, TestLazygitEntry_Install_ForeignKeyWarning) says:
> assert res.Outcome == OutcomeCreated (it appended, not NoChange).

This assertion is **FACTUALLY WRONG** and the test will FAIL if written as-is. Verified against
`internal/integrate/protocol.go` lines 228–235:

```go
// ---- success ----
if missing {                       // missing == errors.Is(rerr, os.ErrNotExist)
    res.Outcome = OutcomeCreated   // ONLY when the file did NOT exist
} else if opts.Action == ActionRemove {
    res.Outcome = OutcomeRemoved
} else {
    res.Outcome = OutcomeUpdated   // ANY upsert on an EXISTING file ← this branch
}
```

In the foreign-key test, the file EXISTS (we pre-wrote foreignYAML) and has no marker
(`exists = HasEntry() == false`). Upsert APPENDS stagecoach's entry → newBytes != orig → Apply writes
→ `missing` is false, action is Upsert → **`OutcomeUpdated`**. OutcomeCreated is reserved exclusively
for the missing-file/create path (FR-I3g). The Outcome constant is FILE-centric, not entry-centric.

→ The implementer MUST assert `res.Outcome == integrate.OutcomeUpdated` (NOT OutcomeCreated).
   The "appended, not NoChange" intent is still satisfied (Updated ≠ NoChange).

## Gotcha — `opts.Out` can be nil
`fmt.Fprintf(opts.Out, ...)` (literal contract §3c) panics if opts.Out is nil. Apply tolerates nil Out
(it resolves to os.Stderr internally), but our probe writes BEFORE Apply. Resolve first, mirroring
gitAliasEntry.Install:
```go
out := opts.Out
if out == nil {
    out = os.Stderr
}
// ... fmt.Fprintf(out, "WARNING: ...", e.key)
```
This keeps existing `TestLazygitEntry_Install_Creates` (Out=nil, fresh file → no probe hit anyway) and
all other nil-Out installs robust even if a future foreign+nil-Out case arises.

## Probe is BEST-EFFORT (never return early / never error)
- (a) `data, rerr := os.ReadFile(e.resolvedPath())` — if `rerr != nil` (missing/unreadable): SKIP probe,
  fall through to Apply (Apply handles the create path). Do NOT return the error.
- (b) `probe := &lazygitTarget{key: e.key}; probe.Parse(data)` — if Parse != nil: SKIP (Apply will refuse
  with its own parse error). Do NOT return early.
- (c) `probe.findKeyItem(e.key) != nil` → print WARNING; else no-op.
- Use a THROWAWAY target (`&lazygitTarget{key: e.key}`) SEPARATE from Apply's `tgt` — no shared state.

## No new imports needed
`fmt` and `os` are already imported in integrate_lazygit.go. The change is purely additive inside `Install()`.

## WARNING wording (EXACT — from contract §3c; all sources agree)
```
WARNING: a %s binding already exists (not managed by stagecoach); installing will create a duplicate customCommands entry — use --key to choose a different binding.
```
Contains the substrings the tests assert: "WARNING" ✓ and "duplicate" ✓.

## Duplicate-count assertion (exactly TWO `<c-a>` entries)
yaml.v3 Node-API encode preserves mapping key order AND emits `<c-a>` single-quoted (verified by existing
`TestIntegrateLazygitKeyFlag` which asserts `strings.Contains(data, "key: '<c-s>'")`). So both the foreign
(`- key: '<c-a>'`) and stagecoach (`- key: '<c-a>' # stagecoach-integration`) lines contain the substring
`key: '<c-a>'` → `strings.Count(data, "key: '<c-a>'") == 2`. The robust alternative is to re-parse and count
sequence items with `Content[0].Value=="key" && Content[1].Value=="<c-a>"`.

## DOCS (Mode A)
`docs/cli.md` lazygit target section (~lines 292–340) has "No-mangle behavior" + "Idempotency" subsections but
NO conflict note. Add a "Conflicting key behavior" note mirroring the git-alias target's "Conflicting alias
behavior" subsection (docs/cli.md ~lines 269–277). Lazygit semantics differ from git-alias: it does NOT
overwrite (customCommands is a sequence) — it APPENDS, creating a duplicate. The note must say so.

## Validation commands (VERIFIED — Makefile + .github/workflows/ci.yml)
- `go build ./...`
- `go vet ./internal/cmd/...`
- `go test -race ./internal/cmd/ -run 'TestLazygitEntry_Install' -v`
- `go test -race ./...` (full suite; CI gate)
