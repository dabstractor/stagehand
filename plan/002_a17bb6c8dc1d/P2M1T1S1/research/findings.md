# P2.M1.T1.S1 Research Findings — Binary Detection + Placeholder

Empirically verified on **git 2.54.0** in a throwaway repo (see commands below). These findings are
load-bearing for the PRP — they pin the exact wire formats `detectBinaryFiles` / `fileStatuses` must parse.

## §1. numstat binary format (the PRIMARY signal, FR3a)

`git diff --cached --numstat` emits one line per changed file, **tab-delimited**:

```
1	0	notes.txt        # text file: added=1, deleted=0
-	-	logo.png         # BINARY file: added="-", deleted="-"
```

Confirmed: `-` (literal ASCII hyphen) in BOTH the added and deleted columns ⇒ binary. Detection predicate:
`fields[0] == "-" && fields[1] == "-"`.

**CRITICAL — git CONTENT-SNIFFS, it does not key off the extension.** A file named `fake.png` whose
CONTENT is text emits `1\t0\tfake.png` (treated as TEXT), NOT `-/-`. This is precisely why FR3a mandates
a SUPPLEMENTAL extension denylist: numstat catches real binaries git sniffs; the denylist catches files
git misclassifies (e.g. a `.png` that is actually text would NOT be caught by either — but a genuine
binary that git failed to sniff as binary WOULD be caught by the `.png` extension). The two signals are
complementary, hence UNION.

## §2. name-status format (the status source, FR3b)

`git diff --cached --name-status` emits one line per changed file:

```
A	logo.png          # 2 fields: <status>\t<path>
A	notes.txt
M	assets/logo.png
D	old.bin
T	config            # type-change (text↔binary/symlink/etc.)
R100	src.png	dst.png   # 3 fields: <status><score>\t<src>\t<dst>  (rename/copy)
```

`<status>` ∈ {A, M, D, T, R, C}. R/C carry a similarity score suffix (`R100`, `R75`, …). For the
`map[string]string` (path→status), the DESTINATION path is `fields[len-1]`; the status is `fields[0]`.
This cleanly separates renames into 3 fields (unlike numstat's `=>` format — see §4).

## §3. The useless hunk binary detection REPLACES

Without filtering, `git diff --cached -- logo.png` produces the useless body:
```
diff --git a/logo.png b/logo.png
new file mode 100644
index 0000000..978e1df
Binary files /dev/null and b/logo.png differ
```
FR3b replaces this with ONE placeholder line: `A\t[binary] logo.png`. The status comes from name-status
(§2); the path is the same path numstat flagged.

## §4. RENAME GOTCHA — numstat `=>` format vs name-status 3-field (COORDINATION w/ S2)

git 2.54 has **`diff.renames` ON by default** (since git 2.9), so even WITHOUT `-M`:
- numstat shows a rename as `0\t0\told.png => new.png` (path field contains ` => `, counts `0/0`)
- name-status shows it as `R100\told.png\tnew.png` (clean 3 fields)

Implications for S1's primitives:
- A PURE binary rename shows `0/0` (NOT `-/-`) in numstat ⇒ the numstat `-/-` check does NOT flag it.
  BUT `isBinaryByExtension("old.png => new.png")` ⇒ `filepath.Ext` returns `.png` (last dot) ⇒ the
  extension check DOES flag it. So renames of binaries are still caught by the union. ✓
- The numstat path KEY `old.png => new.png` ≠ the name-status KEY `new.png`. **S2 (StagedDiff
  integration) reconciles these** (it owns the stitching). S1's `detectBinaryFiles` faithfully keys the
  map by whatever numstat emitted (the `=>` form for renames); `fileStatuses` keys by the clean
  destination. This mismatch is a DOCUMENTED coordination point for S2, NOT a bug in S1.
- Accepted: binary renames are rare in a single staging operation (FR3b examples are A and M). Do NOT
  over-engineer; document and move on.

## §5. Exit-code semantics (matches existing simple-branch methods)

`git diff --numstat` and `git diff --name-status` (WITHOUT `--quiet`):
- exit 0 on success, INCLUDING when there are no changes (empty stdout). **No exit-1 inversion** (that
  is exclusive to `--quiet`, used by HasStagedChanges — FINDING 6).
- exit 128 on a corrupt repo / unresolvable tree SHA (TreeDiff path) / bad pathspec.

⇒ Use the SIMPLE branch form: `if err != nil { return err }; if code != 0 { return wrapped error }` —
identical to StagedDiff/DiffTree/StagedFileCount. NOT HasStagedChanges' switch form.

## §6. The run() helper contract (the INPUT)

`func (g *gitRunner) run(ctx, repo string, args ...string) (stdout, stderr, exitCode int, err error)`:
- resolves git via exec.LookPath; targets repo via `-C` flag (NOT cmd.Dir); `[]string` args, NO shell.
- **INVARIANT: a non-zero git exit returns (stdout, stderr, exitCode, nil)** — `err` is nil, the code is
  in `exitCode`. Only infrastructural failures (LookPath miss, ctx cancel, start/I-O) return err != nil
  with exitCode = -1.
- ⇒ every new method: handle `err != nil` first (propagate, unwrapped — infrastructural), THEN
  `code != 0` (wrap with exit code + trimmed stderr).

`g.workDir` is the repo path. Methods call `g.run(ctx, g.workDir, args...)`.

## §7. Variadic `diffArgs` supports all three FR3c application points

`detectBinaryFiles(ctx, diffArgs...)` / `fileStatuses(ctx, diffArgs...)` build:
`["diff"] + diffArgs + ["--numstat"]` (resp. `--name-status`). This serves all three consumers:
- S2 StagedDiff: `diffArgs = ["--cached"]` ⇒ `git diff --cached --numstat`
- P2.M2.T2.S2 WorkingTreeDiff: `diffArgs = []` ⇒ `git diff --numstat`
- P2.M2.T1.S2 TreeDiff: `diffArgs = [treeA, treeB]` ⇒ `git diff <treeA> <treeB> --numstat`

## §8. Test fixtures (reuse existing helpers, same package `package git`)

Defined in `internal/git/committree_test.go` (available to all `internal/git/*_test.go` since same pkg):
- `initRepo(t, dir)` — `git init` + repo-local user.name/user.email.
- `writeFile(t, dir, name, body string)` — WriteFile 0644.
- `stageFile(t, dir, name string)` — `git add <name>`.
- `writeTreeOf(t, dir) string`, `setIdentityConfig(t, dir)` — also available.

**To create a REAL binary for tests**: write bytes with a NUL or a PNG header
(`"\x89PNG\r\n\x1a\n..."`) — git content-sniffs these as binary ⇒ numstat emits `-/-`. To test the
extension-denylist path INDEPENDENTLY of numstat: write TEXT content to a `.png` file (numstat emits
`1/0`, so ONLY the extension check catches it). This separation is the key to testing each signal in
isolation.

**Pure functions** (`isBinaryByExtension`, `binaryPlaceholderLine`) need NO repo — table tests directly.
**gitRunner methods** (`detectBinaryFiles`, `fileStatuses`) need a temp repo via initRepo + stageFile.

## §9. Package/idiom facts

- New file: `internal/git/binary.go`, `package git` (same package as git.go — methods hang off `*gitRunner`).
- Tests: `internal/git/binary_test.go`, `package git` (internal test, like stagediff_test.go).
- Imports needed: `context`, `fmt`, `path/filepath`, `strings`. (NO new deps; go.mod/go.sum UNCHANGED.)
- `.golangci.yml`: errcheck/gosimple/govet/ineffassign/staticcheck/unused enabled. `binary_test.go` is
  NOT in the exclude list (only `stagediff_test.go` is) ⇒ keep errcheck-clean there.
- Lint/style gate: `gofmt -l`, `go vet`, `golangci-lint run`, `go test ./...`.

## §10. Confidence: 9/10

All wire formats empirically verified on git 2.54.0. The only residual uncertainty is the rename `=>`
reconciliation between numstat and name-status (§4), which is S2's responsibility — S1's primitives are
faithful and correct. No import cycle (single package). No new dependencies.
