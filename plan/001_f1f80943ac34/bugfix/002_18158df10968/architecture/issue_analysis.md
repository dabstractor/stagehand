# Issue Analysis & Fix Plan — Bugfix-002

Verified root cause, exact fix location, blast radius, and test patterns for all four issues.
All file:line refs verified against `/home/dustin/projects/stagecoach` (2026-06-30).

---

## ISSUE 1 (Major) — Explicit `--config`/`STAGECOACH_CONFIG` to a NONEXISTENT file is silently ignored

### Root cause (verified)
`config.Load` (internal/config/load.go:48-65) resolves the global-file path:
```go
globalPath := opts.ConfigPathOverride
if globalPath == "" {
    if env := os.Getenv("STAGECOACH_CONFIG"); env != "" { globalPath = env } else { globalPath = globalConfigPath() }
}
if g, err := loadTOML(globalPath); err != nil { return nil, fmt.Errorf("global config: %w", err) }
else if g != nil { overlay(&cfg, g) }
```
`loadTOML` (internal/config/file.go:99-102) returns `(nil, nil)` on `os.IsNotExist`:
```go
data, err := os.ReadFile(path)
if err != nil {
    if os.IsNotExist(err) { return nil, nil }  // not an error: layer simply absent
    return nil, fmt.Errorf("read config %s: %w", path, err)
}
```
There is **no record** of whether `globalPath` came from an explicit source (`--config` /
`STAGECOACH_CONFIG`) or from discovery (`globalConfigPath()`). A missing explicit path is
indistinguishable from "discovery default absent" → `cfg.Provider == ""` → `buildDeps`
auto-detects the first **installed** built-in (pi/claude/gemini/…) and invokes the REAL agent.
Contrast: a *malformed* or *directory* explicit path correctly errors (loadTOML parse error /
read error), so only the *missing* case is the dangerous inconsistency.

### Fix (localized to internal/config/load.go)
Track whether the path is explicit, and make a missing explicit file a hard error:
```go
explicit := opts.ConfigPathOverride != "" || os.Getenv("STAGECOACH_CONFIG") != ""
...
g, err := loadTOML(globalPath)
if err != nil { return nil, fmt.Errorf("global config: %w", err) }
if g == nil && explicit {
    return nil, fmt.Errorf("config file not found: %s", globalPath)
}
```
`loadTOML`'s `(nil,nil)` contract is PRESERVED (discovery path still tolerates absence). The
error propagates to the CLI as `config: config file not found: <path>` → `exitcode.Error` (1),
before any provider is resolved. No change to `loadTOML`, `file.go`, or the discovery path.

### Blast radius
- **Production:** internal/config/load.go ONLY (Load). `--config` already flows via
  `ConfigPathOverride` (internal/cmd/root.go:79, PersistentPreRunE:42-55); `STAGECOACH_CONFIG`
  is already read in Load. No wiring changes.
- **Tests:** internal/config/load_test.go. Existing `TestLoad_ConfigPathOverride` (load_test.go:317)
  and `TestLoad_STAGECOACH_CONFIG_EnvPath` (load_test.go:330) use EXISTING files (still pass). ADD:
  a missing `ConfigPathOverride` path → error containing `"config file not found"`; a missing
  `STAGECOACH_CONFIG` path → same error; the discovery path with no global file → still `(nil)` /
  no error (regression guard).

### Test patterns to reuse (internal/config, package-private helpers)
- `loadEnvSetup(t) (home, repo, globalDir string)` — load_test.go:62; isolates HOME/XDG, makes a repo.
- `writeConfigFile(t, dir, relPath, body) string` — load_test.go:42.
- `newFlagSet(t) *pflag.FlagSet` — load_test.go:70.
- Error-contract idiom: `strings.Contains(err.Error(), "<tag>")`; layer tag is `"config"` (the
  `load config:` wrap from resolveConfig) or `"global config"`.

---

## ISSUE 2 (Major) — Manifest-level `output` / `strip_code_fence` silently clobbered by `[generation]` defaults

### Root cause (verified)
`buildDeps` (pkg/stagecoach/stagecoach.go:206-211) overrides the resolved manifest unconditionally:
```go
if cfg.Output != "" { o := cfg.Output; m.Output = &o }       // ALWAYS true: Defaults() sets "raw"
if cfg.StripCodeFence != nil { m.StripCodeFence = cfg.StripCodeFence } // ALWAYS true: Defaults() sets boolPtr(true)
```
`Defaults()` (internal/config/config.go:67-70) seeds `Output: "raw"` (non-empty `string`) and
`StripCodeFence: boolPtr(true)` (non-nil `*bool`). So the manifest's OWN `output`/`strip_code_fence`
are always overwritten. Consequence (verified end-to-end in the PRD repro):
- `[provider.X] output = "json"` + `json_field`, no `[generation]` → JSON NOT parsed; raw blob used.
- `[provider.X] strip_code_fence = false`, no `[generation]` → fence stripped anyway.
- `providers show <name>` prints `output = 'json'` (the registry's pre-override manifest) but
  parsing uses `raw` — a debugging trap. (Note: `providers show` is unaffected by the Config change;
  it reads the registry manifest. The fix makes parsing MATCH what `show` reports.)

### Fix — two-part, tri-state (the `*bool` model `StripCodeFence` already follows)

**Part 1 (config package — type + defaults):**
1. `internal/config/config.go`: change `Output string` → `Output *string` (line 35); add
   `func strPtr(s string) *string { return &s }` next to the existing `boolPtr` (line 11);
   in `Defaults()` REMOVE both `Output: "raw"` and `StripCodeFence: boolPtr(true)` (lines 68-69) —
   leave them nil. (The manifest's `Resolve()` supplies `raw`/`true` per §12.1 — manifest.go:138-148,
   `DefaultOutput`/`DefaultStripCodeFence`.)
2. `internal/config/file.go` `materialize` (159-160): `if g.Output != "" { o := g.Output; c.Output = &o }`
   (`fileGeneration.Output` stays a plain `string` — go-toml decodes it fine; only the resolved
   Config becomes a pointer). `overlay` (210-211): `if src.Output != nil { dst.Output = src.Output }`
   (now pointer-symmetric with StripCodeFence). StripCodeFence lines (162-163, 213-214) UNCHANGED.
3. `internal/config/git.go` `loadGitConfig` (124-127): `if found { c.Output = &v }` (v is a per-loop
   local — `&v` safe). StripCodeFence (152-155) UNCHANGED.

**Part 2 (the bridge — delivers the behavior):**
4. `pkg/stagecoach/stagecoach.go` `buildDeps` (206-211): `if cfg.Output != nil { m.Output = cfg.Output }`
   (drop the local copy). StripCodeFence guard UNCHANGED (`!= nil`). After the change BOTH branches
   are `if cfg.X != nil { m.X = cfg.X }` — manifest wins by default; `[generation]` overrides only
   when explicitly set.

### Why this is correct & safe (blast radius — verified mechanical)
- `cfg.Output`/`cfg.StripCodeFence` are consumed ONLY in `buildDeps` (copy onto manifest). Every
  other pipeline stage reads `deps.Manifest.*` (already `*string`/`*bool`). `loadEnv`/`loadFlags`
  do NOT touch Output/StripCodeFence. `cmd/config.go` writes a STATIC template (no Config marshal).
  So no runtime consumer breaks.
- **Test edits (compile-driven):** config_test.go `TestDefaults` (39-44: Output→`nil`/deref
  assertion; the `TestTOMLMarshalKeysAndNoColorExclusion` at 47-72 marshals `Defaults()` and checks
  `output =`/`strip_code_fence =` — with nil defaults these keys vanish from the marshal; UPDATE the
  test to marshal a Config with explicit values OR drop those two key assertions); file_test.go
  (82-83 deref, 114 struct-literal `strPtr("json")`, 119-120 deref, 407 literal); git_test.go
  (111-112 deref, 133 `!= nil`, 345-346 deref); stagecoach_test.go (846 `cfg.Output = nil`).

### Test patterns (behavior validation)
- `TestOverlayPartial` (file_test.go:108) is the clobber-detection contract model.
- pkg/stagecoach `setupTestRepo`/`setupScriptedRepo` (stagecoach_test.go:60-180) register a stub via
  repo-local `.stagecoach.toml` and exercise the REAL config.Load+registry path. ADD end-to-end tests:
  (a) `[provider.stub] output="json"`+`json_field`, no `[generation]` → parsed JSON (message ==
  json_field value); (b) `[provider.stub] strip_code_fence=false` + fenced stub output → fence
  preserved; (c) `[generation] output="json"` overrides a `[provider.stub] output="raw"` manifest;
  (d) regression: no `[generation]`, manifest default `raw` → raw still works.

### Documentation (Mode A — rides with Part 2)
- docs/configuration.md line 84: "These `[generation]` values override any per-provider defaults —
  the broader layer wins" → MUST become "an opt-in OVERRIDE; when unset, the per-provider manifest
  value is honored". Lines 79-80 (defaults table) and 49-56 ([generation] block) updated.
- docs/providers.md line 124 + lines 30-32: same "broader layer wins" wording → opt-in override.

---

## ISSUE 3 (Minor) — Raw multi-line `git write-tree` error instead of clean "resolve merge conflicts" message

### Root cause (verified)
`git.WriteTree` (internal/git/git.go:218-224) on `code != 0` appends the FULL trimmed stderr:
```go
if code != 0 {
    return "", fmt.Errorf("git write-tree: unresolved merge conflicts in index (exit %d): %s", code, strings.TrimSpace(stderr))
}
```
git's stderr for unmerged paths is multi-line noisy (`f.txt: unmerged (...)` ×N +
`fatal: git-write-tree: error building trees`). Behavior is CORRECT (exit 1, model never invoked,
HEAD/index untouched — WriteTree is step 3, pre-generation) — only the MESSAGE is wrong. The
"↳ Generating with <provider>…" label prints before it (cosmetic; write-tree is still step 3).

### Fix (localized to internal/git/git.go WriteTree)
Return a single clean line when the failure is unresolved conflicts. Two acceptable variants
(implementer's choice; the PRD accepts "at minimum, wrap to the friendly text"):
- **(preferred, accurate)** On `code != 0`, run `git ls-files -u`; if non-empty → return a clean
  error (e.g. `errors.New("unresolved merge conflicts in the index — resolve them first, then re-run stagecoach")`);
  else return the original detailed error (some other write-tree failure). The extra git call is on
  the failure path only (not hot).
- **(minimal)** Just drop the `%s` stderr from the message and keep a clean single line (the
  exit-128-on-populated-index is unambiguously unmerged in practice).

Either way: the error propagates through CommitStaged (generate.go step 3) AND runPipeline
(stagecoach.go step 3) unchanged → `handleGenError` default branch → `exitcode.New(Error, err)` →
main prints `stagecoach: <clean msg>` + exit 1. STILL pre-generation, STILL exit 1, HEAD untouched.

### Blast radius
- **Production:** internal/git/git.go `WriteTree` only (one method; both call sites covered).
- **Tests:** internal/git/writetree_test.go `TestWriteTree_MergeConflict` (122-133) asserts
  `strings.Contains(err.Error(), "unresolved merge conflicts")`. KEEP that substring in the clean
  message (so the existing assertion still holds) and ADD an assertion that the message does NOT
  contain the raw noise (`fatal: git-write-tree`, `error building trees`). The fixture
  `makeMergeConflict` (writetree_test.go:22-68) is reused as-is.

### Documentation (Mode A)
- docs/how-it-works.md §"Failure modes and exit codes" (line 53) — if it quotes the merge-conflict
  wording, align it; else note "none — message-only change".

---

## ISSUE 4 (Minor) — `--dry-run` can exit 3/124 + full rescue recipe (surprising for a preview)

### Root cause (verified)
`--dry-run` correctly runs the full pipeline incl. the write-tree snapshot (FR49, bugfix-001 fix).
On generation failure, `pkg/stagecoach.runPipeline` returns a `*generate.RescueError` (timeout→124 /
rescue→3), identical to the commit path. `handleGenError` (internal/cmd/default_action.go:169-188)
then prints the FULL `FormatRescue` recovery block (Tree ID + manual `git commit-tree` recipe) and
maps to exit 3/124. For an operation that was never going to commit, the recovery recipe is odd,
and `msg=$(stagecoach --dry-run)` gets a non-zero exit + no message on stdout.

### Fix (localized to the CLI layer — internal/cmd/default_action.go handleGenError)
The library API (`pkg/stagecoach`) is UNCHANGED (still returns `*RescueError`); only the CLI
rendering special-cases dry-run. In `handleGenError`, BEFORE the existing RescueError branch, add:
```go
if flagDryRun {
    var re *generate.RescueError
    if errors.As(err, &re) {  // dry-run timeout OR rescue
        msg := "could not generate a commit message; run without --dry-run to see retries and the recovery recipe"
        if errors.Is(err, generate.ErrTimeout) {
            msg = "generation timed out; run without --dry-run to see the recovery recipe"
        }
        fmt.Fprintln(stderr, msg)
        return exitcode.New(exitcode.Error, nil)  // exit 1, silent (already printed)
    }
}
```
`flagDryRun` is a package var in `internal/cmd` (root.go:40), readable directly from
`handleGenError` (same package). Result: dry-run generation failure → exit 1 + short stderr line,
no rescue recipe. CAS / nothing-to-commit / generic errors on dry-run are unaffected (they already
exit 1/2 appropriately).

### Blast radius & regression guards
- **Production:** internal/cmd/default_action.go `handleGenError` only.
- **Tests:** internal/cmd/default_action_test.go. The COMMIT-path tests stay as-is:
  `TestRunDefault_Rescue` (504) → exit 3, `TestRunDefault_Timeout` (563) → exit 124 (neither uses
  `--dry-run`). ADD dry-run variants: `--dry-run` + timeout → `exitcode.For == Error` (1) + short
  msg + NO `git commit-tree` recipe in stderr; `--dry-run` + rescue (blank stub) → exit 1 + short
  msg. Use `setupStubRepoWithTimeout` (default_action_test.go:109) and the `rootCmd.SetArgs({"--dry-run"})`
  pattern from `TestRunDefault_DryRun` (268).
- **Library test guard:** pkg/stagecoach `TestGenerateCommit_Timeout` "dryrun" subtest
  (stagecoach_test.go:296-362) asserts the LIBRARY returns `*RescueError{Kind:ErrTimeout}` →
  `exitcode.For == Timeout` (124). This STILL HOLDS — the library is unchanged; only the CLI wraps
  it to exit 1. Do NOT change this test.

### Documentation (Mode A)
- docs/cli.md §"Exit codes" (line 76) and §"Global flags" `--dry-run` (line 26): note that a dry-run
  GENERATION failure exits 1 with a short message (not 3/124 + recovery recipe).

---

## Cross-cutting: changeset-level documentation (Mode B — final task)
README.md touches: §"Quick start" `--dry-run` preview (line 66); §"Configure your agent"
`--config` note (line 121) + `[generation] output/strip_code_fence` blurb (line 119, line 170).
docs/how-it-works.md §"Failure modes and exit codes" (line 53) + §"Rescue protocol" (line 66) if
they describe dry-run exit codes. These overview/feature docs are swept by the FINAL task and
depend on every implementing subtask.
