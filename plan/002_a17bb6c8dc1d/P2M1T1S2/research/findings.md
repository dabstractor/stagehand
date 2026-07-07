# P2.M1.T1.S2 Research Findings — StagedDiff Binary-Filtering Integration

Empirically verified on **git 2.54.0** in a throwaway repo + codebase inspection. Load-bearing for the
PRP.

## §1. S1's `binary.go` is ALREADY FULLY IMPLEMENTED (consume the real interface)

`internal/git/binary.go` exists and matches S1's PRP contract EXACTLY. The five symbols to consume:

```go
var defaultBinaryExtensions = map[string]bool{ /* 36 FR3a exts */ }
func isBinaryByExtension(path string, extraExts []string) bool
func binaryPlaceholderLine(status, path string) string                       // "<status>\t[binary] <path>"
func (g *gitRunner) detectBinaryFiles(ctx, diffArgs ...string) (map[string]bool, error)   // numstat + built-in denylist (nil extraExts!)
func (g *gitRunner) fileStatuses(ctx, diffArgs ...string) (map[string]string, error)      // dest-path → A/M/D/T/R100
```

**CRITICAL — `detectBinaryFiles` hardcodes `nil` for extraExts** (it has NO extraExts parameter; the
union inside is `(added=="-"&&deleted=="-") || isBinaryByExtension(path, nil)`). So it applies the
BUILT-IN denylist only. The user's `binary_extensions` override CANNOT reach detection through
`detectBinaryFiles`. ⇒ S2 must SUPPLEMENT: after calling `detectBinaryFiles`, additionally test every
path in `fileStatuses` against `isBinaryByExtension(path, opts.BinaryExtensions)`. The union in S2
becomes: `binSet[path] || isBinaryByExtension(path, opts.BinaryExtensions)`.

## §2. The two `StagedDiff` call sites do NOT pass BinaryExtensions yet

```go
// internal/generate/generate.go:143  (CommitStaged)
diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
    MaxDiffBytes: cfg.MaxDiffBytes,
    MaxMDLines:   cfg.MaxMdLines,
})
// pkg/stagecoach/stagecoach.go:247  (public Commit) — identical construction
```

`config.Config.BinaryExtensions []string` (config.go:81, default nil) is fully plumbed through config +
file.go, but neither call site forwards it. **S2 must add `BinaryExtensions: cfg.BinaryExtensions`** at
both sites (1 line each) or the field is dead. Default nil ⇒ built-in denylist only ⇒ no behavior change
for default config (back-compatible).

## §3. Pathspec `:!<path>` exclude DROPS the binary body (verified)

```
$ git diff --cached -- a.go logo.png
... a.go hunk ...
diff --git a/logo.png b/logo.png
Binary files /dev/null and b/logo.png differ          # ← the useless hunk

$ git diff --cached -- a.go ':!logo.png'
... a.go hunk ONLY ...                                  # ← logo.png body GONE ✓
```

`:!<path>` works AFTER `--`, alongside globs (`:!*.lock`) and the markdown excludes (`:!*.md`).
Confirmed: appending `:!logo.png` to the existing `nmArgs` excludes (all `:!`-prefixed, no positive
pathspec) correctly removes logo.png from the aggregate. The existing `TestStagedDiff_ExcludesLockSnapMapVendor`
proves the all-exclude pathspec-list form works.

## §4. Rename key mismatch (the S1 coordination point) — reconcile by keying off `fileStatuses`

```
$ git mv logo.png brand.png   (logo.png is a real binary)
$ git diff --cached --numstat
-\t-\tlogo.png => brand.png          # numstat keys it as "logo.png => brand.png" (path CONTAINS " => ")
$ git diff --cached --name-status
R100\tlogo.png\tbrand.png            # name-status keys the DESTINATION "brand.png" → "R100"
```

- `detectBinaryFiles` keys renames by the `=> ` numstat string; `fileStatuses` keys by the clean
  destination. The two keys DIFFER for renames.
- ⇒ S2 iterates over **`fileStatuses`** (destination keys) and looks the binary set up BY destination.
  `binSet[<dest>]` matches for non-rename A/M/D (same path in both). For a rename, `binSet` holds the
  `=> ` key (miss), BUT `isBinaryByExtension(<dest>, …)` catches `.png`/etc. via the denylist. So the
  union still catches binary renames via the extension signal. The orphaned `=> ` key in `binSet` is
  harmless (never read).
- Accepted edge case: a renamed binary with NO denylisted extension AND not in BinaryExtensions, that
  git sniffs as binary — `binSet` flags it under the `=> ` key, which the destination-keyed lookup
  misses. Vanishingly rare (binary rename of an extension-less file); FR3b examples are A and M. Document,
  don't over-engineer.

## §5. CRITICAL GOTCHA — do NOT mutate the package-level `defaultExcludes`

Current Part-2 code:
```go
excludes := opts.Excludes
if len(excludes) == 0 {
    excludes = defaultExcludes          // ← aliases the PACKAGE-LEVEL var
}
nmArgs = append(nmArgs, excludes...)    // ← appends to nmArgs (copies) — SAFE today
```

If S2 naively does `excludes = append(excludes, ":!"+path)` when `opts.Excludes` is empty, it APPENDS to
`defaultExcludes` itself (the package var), corrupting it for every subsequent call / test. **S2 must
collect binary excludes in a SEPARATE slice** (`binExcludes`) and append it to `nmArgs` (which copies),
NEVER to `excludes`. Verified: this is the same reason the current code appends `:!*.md` to `nmArgs`, not
to `excludes`.

## §6. Placement — binary section sits BETWEEN Part 1 (markdown) and Part 2 (non-markdown)

Work item: "after the markdown section and before the non-markdown capture … placeholder lines appear in
the payload between markdown and non-markdown sections". So:
1. Part 1 (markdown) — UNCHANGED (still the first git call ⇒ existing GitBinaryMissing/ContextCancelled
   tests still pass: they fail on the markdown-list call first).
2. **NEW binary section**: `detectBinaryFiles("--cached")` + `fileStatuses("--cached")` → emit
   placeholders to `b` + collect `binExcludes`.
3. Part 2 (non-markdown) — append `binExcludes...` to `nmArgs`; byte cap applies to `nmDiff` ONLY
   (placeholders are tiny metadata lines, NOT counted against `max_diff_bytes` — they REPLACE the binary
   bodies, so they are already cheaper).

## §7. Deterministic output — sort the binary paths before emitting

`fileStatuses` returns a `map` ⇒ iteration order is non-deterministic. Sort the collected binary paths
(`sort.Strings`) before emitting placeholders so the payload (and tests) are reproducible. Needs
`import "sort"` (stdlib; git.go does not currently import it — add it). Existing stagediff_test.go uses
`strings.Contains`-based assertions (order-independent), so sorting is a robustness + reproducibility
enhancement, not a correctness requirement.

## §8. All 12 existing stagediff_test.go tests still pass (verified by trace)

None stage a binary file, so `detectBinaryFiles` returns an empty set, no placeholders are emitted, and
the only behavioral change vs. today is 2 extra read-only git calls (numstat + name-status) that return
empty. `TestStagedDiff_NothingStaged` ⇒ empty map ⇒ output "" unchanged. `TestStagedDiff_GitBinaryMissing`
/ `_ContextCancelled` ⇒ fail on the markdown-list call (Part 1) BEFORE the binary section, so the error
shape is unchanged. ✓ No existing test needs modification to keep passing.

## §9. Test fixtures (same package `git`, reuse — do NOT redefine)

Defined in `committree_test.go` (`writeFile`, `stageFile`, `setIdentityConfig`, `writeTreeOf`, `headSHA`,
`commitMessage`) + `git_test.go` (`initRepo`). All available to `stagediff_test.go`. To create a REAL
binary for an integration test: `writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00")` — git
content-sniffs it ⇒ numstat `-/-` ⇒ `detectBinaryFiles` flags it. To test the user `BinaryExtensions`
override INDEPENDENTLY of the built-in denylist: use an extension NOT in the 36-entry built-in list (e.g.
`.dat`) with BINARY content, then pass `BinaryExtensions: []string{"dat"}`.

## §10. lint / scope facts

- `.golangci.yml` EXCLUDES `internal/git/stagediff_test.go` from errcheck ⇒ new tests there are
  errcheck-exempt (but keep gosimple/govet/staticcheck/unused clean).
- `internal/git/binary.go` + `binary_test.go` are S1's (in progress / done) — DO NOT edit them. S2 only
  CONSUMES their exported-to-package symbols.
- No other in-progress task touches `generate.go` / `stagecoach.go` (P2.M2, P3, P4 are all "Planned") ⇒
  the 2 call-site edits are conflict-free.
- go.mod/go.sum UNCHANGED (stdlib `sort` only). The `Git` INTERFACE is UNCHANGED (StagedDiff signature
  is unchanged; BinaryExtensions is a new struct field, not a new method).
