# Code Context — PRD Issues 4 & 7

Research-only scout report. No code was changed. Quotes are from the working tree at
`/home/dustin/projects/stagehand` (commit-clean). All file paths are repo-relative.

---

## PART A — Dead `[generation] output` / `strip_code_fence` config fields (Issue 4)

### A.1 The Config struct fields ARE populated

`internal/config/config.go:35-38` — the two fields live under the `[generation]` block, with TOML tags:

```go
	// [generation] (PRD §16.2)
	...
	Output              string `toml:"output"`           // "raw" | "json"
	StripCodeFence      bool   `toml:"strip_code_fence"` // strip ``` fences from agent output
```

`Defaults()` (config.go:62-75) seeds them: `Output: "raw"`, `StripCodeFence: true`.

### A.2 file.go — parsed from the `[generation]` TOML section

`internal/config/file.go:36-44` — `fileGeneration` decode struct mirrors §16.2:

```go
type fileGeneration struct {
	MaxDiffBytes        int    `toml:"max_diff_bytes"`
	MaxMdLines          int    `toml:"max_md_lines"`
	MaxDuplicateRetries int    `toml:"max_duplicate_retries"`
	SubjectTargetChars  int    `toml:"subject_target_chars"`
	Output              string `toml:"output"`
	StripCodeFence      bool   `toml:"strip_code_fence"`
}
```

`materialize()` (file.go:151-155) copies them into the partial `*Config`:

```go
	if g.Output != "" {
		c.Output = g.Output
	}
	if g.StripCodeFence {
		c.StripCodeFence = true // v1 limitation: cannot set false via file
	}
```

`overlay()` (file.go:202-206) merges them in the layer cascade:

```go
	if src.Output != "" {
		dst.Output = src.Output
	}
	if src.StripCodeFence {
		dst.StripCodeFence = true
	}
```

### A.3 git.go — read from `stagehand.*` git-config keys

`internal/config/git.go:124-128` and `git.go:152-156`:

```go
	if v, found, err := gitConfigGet(repoDir, "stagehand.output"); err != nil {
		return nil, err
	} else if found {
		c.Output = v
	}
	...
	if v, found, err := gitConfigBool(repoDir, "stagehand.stripCodeFence"); err != nil { // camelCase!
		return nil, err
	} else if found {
		c.StripCodeFence = v
	}
```

### A.4 load.go — layers merge these fields

`internal/config/load.go:73-79` calls `overlay(&cfg, gc)` for the git-config layer (and likewise for
global + repo-local TOML). No env-var or CLI-flag layer sets Output/StripCodeFence (loadEnv/loadFlags
handle only provider/model/timeout/verbose/no-color).

**Conclusion so far:** `cfg.Output` / `cfg.StripCodeFence` are fully populated through the 7-layer
resolver and always hold a resolved value (`"raw"` / `true` by default).

### A.5 ParseOutput reads ONLY the Manifest — NEVER cfg

`internal/provider/parse.go:44-52`:

```go
func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool) {
	r := m.Resolve() // nil-pointer-safe deref; copy — caller's m untouched (mirrors render.go)

	// Step 1: trim leading/trailing whitespace.
	s := strings.TrimSpace(raw)

	// Step 2: optional single-layer code-fence unwrap (``` or ~~~). PREFIX check only.
	if *r.StripCodeFence {
		s = strings.TrimSpace(stripCodeFence(s))
	}

	// Step 3: output-mode switch.
	switch *r.Output {
	case "json":
		msg, fellback = parseJSON(s, *r.JsonField)
	case "raw":
		msg = s
	...
```

`ParseOutput` takes a `Manifest` (not a `Config`) and reads only `*r.Output` / `*r.StripCodeFence` —
the manifest's resolved pointer fields. It has no access to `cfg` at all.

### A.6 Manifest.Output / Manifest.StripCodeFence and how the manifest is built

`internal/provider/manifest.go:78-80`:

```go
	// --- output (§12.1) ---
	Output         *string `toml:"output"`           // raw|json; nil => Resolve→"raw".
	JsonField      *string `toml:"json_field"`       // used only when Output=="json".
	StripCodeFence *bool   `toml:"strip_code_fence"` // nil => Resolve→true.
```

These are `*string` / `*bool` pointers (the override-signal design). `Resolve()` (manifest.go:151-159)
fills nils with `DefaultOutput="raw"` / `DefaultStripCodeFence=true`.

The manifest consumed by `ParseOutput` is built in **`pkg/stagehand/stagehand.go:buildDeps`**
(stages 3–5 at buildDeps). `buildDeps` constructs the registry from `cfg.Providers` (the `[provider.X]`
map) + `BuiltinManifests()`, then `reg.Get(name)` returns the merged manifest:

```go
func buildDeps(cfg config.Config, repoDir string) (generate.Deps, error) {
	overrides, err := provider.DecodeUserOverrides(cfg.Providers)
	...
	reg := provider.NewRegistry(overrides)
	...
	m, ok := reg.Get(name)
	...
	return generate.Deps{Git: git.New(repoDir), Manifest: m}, nil
}
```

`cfg.Output` and `cfg.StripCodeFence` are **never referenced** in `buildDeps`. They are silently
dropped. The registry only merges `cfg.Providers` (`[provider.X]` bodies) onto the built-ins via
`MergeManifest` (`internal/provider/merge.go:62-69`). So the `[generation] output`/`strip_code_fence`
values have no path to the manifest.

**A repo-wide grep confirms it:** `cfg.Output` / `cfg.StripCodeFence` are read ONLY by config-package
tests (`internal/config/{config,file,git}_test.go`) that assert the loaders populate them. No
production consumer exists.

### A.7 `config init` template documents them as usable

`internal/cmd/config.go` — `exampleConfigTemplate` (the `const` near the bottom of the file) advertises
both under `[generation]`:

```
# [generation]
# max_diff_bytes        = 300000  # byte cap on the non-markdown diff section
# max_md_lines          = 100     # per-file line cap for markdown diffs
# max_duplicate_retries = 3       # re-generation attempts when the subject duplicates a recent commit
# subject_target_chars  = 50      # target subject-line length for truncation
# output                = "raw"   # agent output mode: "raw" | "json"
# strip_code_fence      = true    # remove ` fences from agent output
```

And the git-config header documents `stagehand.output` / `stagehand.stripCodeFence` indirectly via the
`stagehand.*` example block. So users are told these knobs work — but they do not.

### A.8 Integration point / decision

The exact seam where `cfg.Output` / `cfg.StripCodeFence` SHOULD be applied onto the manifest is
**`pkg/stagehand/stagehand.go` → `buildDeps`**, after `m, ok := reg.Get(name)` and before
`return generate.Deps{...}`. Concretely, after the existing `m.Validate()` call, one would:

```go
	// Apply [generation] output/strip_code_fence overrides onto the resolved manifest.
	if cfg.Output != "" {
	    o := cfg.Output
	    m.Output = &o
	}
	m.StripCodeFence = &cfg.StripCodeFence   // ALWAYS set (default true); overwrites any nil
	if err := m.Validate(); err != nil { ... }   // re-validate the new Output value
```

(Note: `cfg.StripCodeFence` is a plain bool, always set post-Defaults, so assigning a `&cfg.StripCodeFence`
pointer is fine but mutates the cfg copy — safer to copy into a local first, mirroring the `o := cfg.Output`
pattern.)

**ALTERNATIVE (recommended for a "dead code" fix per Issue 4's framing):** remove the fields entirely:
delete from `Config` struct + `Defaults()` (config.go), `fileGeneration` + `materialize` + `overlay`
(file.go), the two git-config reads (git.go), the `config init` template lines (config.go), and the
loader tests that assert them (config_test.go:46-49, file_test.go:81-82/118/140, git_test.go:108-112/
133-140/163/345-346). The provider-manifest path (`[provider.X] output`/`strip_code_fence`) already
covers per-provider override of these knobs, so removing the `[generation]` duplicates loses no real
capability. The two fixes are mutually exclusive — the parent must pick one.

---

## PART B — Auto-stage "(0 files)" cosmetic notice (Issue 7)

### B.1 The exact notice code

`internal/cmd/default_action.go:60-91` — the `!hasStaged` → `cfg.AutoStageAll` branch:

```go
	if !hasStaged {
		switch {
		case flagNoAutoStage:
			// FR19: --no-auto-stage + nothing staged → exit 2 "Nothing staged." ...
			return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing staged."))
		case cfg.AutoStageAll:
			// FR16/FR18: auto-stage all, print the transparent notice, re-check.
			if err := g.AddAll(ctx); err != nil {
				return exitcode.New(exitcode.Error, fmt.Errorf("git add -A: %w", err))
			}
			n, err := g.StagedFileCount(ctx)
			if err != nil {
				return exitcode.New(exitcode.Error, fmt.Errorf("git diff --cached --name-only: %w", err))
			}
			fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d files).", n))) // FR18 (text verbatim, em-dash; colorized)
			hasStaged, err = g.HasStagedChanges(ctx)
			if err != nil {
				return exitcode.New(exitcode.Error, fmt.Errorf("staged changes check: %w", err))
			}
			if !hasStaged {
				// FR17: clean tree even after auto-stage.
				return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
			}
		default:
			// cfg.AutoStageAll==false (config), no --no-auto-stage flag → don't auto-stage; exit 2.
			return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
		}
	}
```

- **Notice print:** `default_action.go:78`.
- **`n` source:** `g.StagedFileCount(ctx)` at `default_action.go:74`.
- **FR17 exit-2 "Nothing to commit.":** `default_action.go:85` (returned when `!hasStaged` after the
  re-check). The same exit-2 string is also returned from `handleGenError`
  (`default_action.go:179`) when `generate.ErrNothingToCommit` bubbles up from the pipeline — but on
  a clean tree the CLI-layer short-circuit at line 85 fires first, so the pipeline is never reached.

### B.2 How `N` (file count) is computed

`internal/git/git.go` `StagedFileCount` (the second function in the read window, around lines 650-690):

```go
func (g *gitRunner) StagedFileCount(ctx context.Context) (int, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "diff", "--cached", "--name-only")
	if err != nil {
		return 0, err
	}
	if code != 0 {
		return 0, fmt.Errorf("git diff --cached --name-only: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	count := 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(line) != "" {
			count++ // trailing newline → final "" element is skipped; empty output → count 0
		}
	}
	return count, nil
}
```

On a clean tree, `AddAll` (git.go `AddAll`) is a documented no-op (`git add -A` exits 0, index
unchanged), so `git diff --cached --name-only` emits empty stdout → `count == 0`. That 0 flows into the
`(%d files)` format, producing the cosmetic `Nothing staged — staging all changes (0 files).` line.

`AddAll` (git.go, around lines 620-650): `g.run(ctx, g.workDir, "add", "-A")`, treats any non-zero exit
as an error; on a clean tree it exits 0 and mutates nothing.

### B.3 Where to add the `if N == 0` short-circuit

Insert between the `StagedFileCount` error guard (line 77, the closing `}` of the `if err != nil`
block) and the notice `Fprintln` (line 78). Minimal patch:

```go
			n, err := g.StagedFileCount(ctx)
			if err != nil {
				return exitcode.New(exitcode.Error, fmt.Errorf("git diff --cached --name-only: %w", err))
			}
			if n == 0 {
				// Clean tree: AddAll staged nothing. Skip the FR18 "(0 files)" notice and go
				// straight to the FR17 exit-2 path (Issue 7 cosmetic fix).
				return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit."))
			}
			fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d files).", n)))
```

This makes the FR17 path at line 85 redundant for the n==0 case (but leave it: it's a belt-and-suspenders
re-check that also catches a race where the index changes between StagedFileCount and HasStagedChanges).
The exit code and message are identical, so exit-code semantics (§15.4: exit 2 NothingToCommit) are
preserved.

**Alternative (smaller behavioral change):** keep the re-check but gate only the *notice* on `n > 0`:

```go
			if n > 0 {
				fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d files).", n)))
			}
```

This still hits the existing FR17 `if !hasStaged` block at line 85 for the exit-2. Both are valid; the
first is cleaner (one early return), the second is more conservative (keeps the HasStagedChanges re-check
as the single source of truth for "still nothing"). Recommend the early-return form since `n == 0` is a
strict subset of `!hasStaged` here.

---

## B.4 FR18 literal template reference

PRD.md:259 (and the snapshot at `plan/001_f1f80943ac34/prd_snapshot.md:259`):

> **FR18.** Print a transparent notice when auto-staging occurs, e.g. `Nothing staged — staging all changes (3 files).`

The production literal (`default_action.go:78`) is `Nothing staged — staging all changes (%d files).`
with an em-dash (—, U+2014) and "files" always plural (no singular form). The notice is routed to
**stderr** (so stdout stays clean for piping, FR51) and colorized via `u.Yellow(...)`. Existing test
`TestRunDefault_AutoStageNotice_FR18` asserts the plaintext substring survives ANSI wrapping.

---

## Tests covering these paths

### Part A (dead fields)
- `internal/config/config_test.go:46-49` — asserts `Defaults()` sets `Output="raw"`, `StripCodeFence=true`.
- `internal/config/file_test.go:81-82, 118, 140` — asserts `materialize`/`overlay` copy Output/StripCodeFence.
- `internal/config/git_test.go:108-112, 133-140, 163, 345-346` — asserts `loadGitConfig` reads
  `stagehand.output` / `stagehand.stripCodeFence`.
- **No test** asserts that `cfg.Output`/`cfg.StripCodeFence` ever reach `ParseOutput` — because they
  don't. `internal/provider/parse_test.go` exercises `ParseOutput` exclusively via hand-built `Manifest`
  values; it never touches `config.Config`.

If the chosen fix is "apply cfg → manifest in buildDeps," add a test in
`pkg/stagehand/stagehand_test.go` that sets `[generation] output="json"` + `strip_code_fence=false` in
config and asserts the resulting `deps.Manifest` carries those values (and that `ParseOutput` then
honors them). If the chosen fix is "remove the fields," delete the config tests listed above.

### Part B (auto-stage notice)
- `internal/cmd/default_action_test.go` `TestRunDefault_AutoStageNotice_FR18` (around line 396-430) —
  writes 2 unstaged files, asserts stderr contains `Nothing staged — staging all changes (2 files).`.
- `internal/cmd/default_action_test.go` `TestRunDefault_NothingStaged_FR17` (around line 293-328) —
  clean tree, asserts exit 2 + HEAD unchanged, but **does NOT assert the absence of the "(0 files)"
  notice** — so the cosmetic bug is currently untested. A regression test for Issue 7 should add
  `if strings.Contains(stderr, "staging all changes") { t.Errorf(...) }` to this test (or a new
  dedicated test).
- `internal/git/addall_test.go` `TestAddAll_CleanTreeNoOp` — proves AddAll is a no-op on a clean tree
  and `StagedFileCount` returns 0.
- `internal/git/stagedcount_test.go` `TestStagedFileCount_NothingStaged` — proves count==0 on empty output.

---

## Architecture (how the pieces connect)

**Config flow (Part A):**
`config.Load` (load.go) runs the 7-layer resolver → `*Config`. `Config.Output`/`Config.StripCodeFence`
are populated by `Defaults()` + `materialize`/`overlay` (file.go) + `loadGitConfig` (git.go). The
resolved `cfg` is passed to `pkg/stagehand.GenerateCommit` → `buildDeps(cfg, repoDir)`. `buildDeps`
builds the provider `Manifest` from `BuiltinManifests()` + `cfg.Providers` (`[provider.X]`), via
`provider.NewRegistry` + `MergeManifest`. The manifest's `Output`/`StripCodeFence` pointer fields are
what `provider.ParseOutput` actually reads. **There is no cfg→manifest bridge for these two fields** —
the seam is missing.

**Auto-stage flow (Part B):**
`runDefault` (default_action.go) is the CLI default action. It checks `g.HasStagedChanges`; if false and
`cfg.AutoStageAll`, it calls `g.AddAll` then `g.StagedFileCount`, prints the FR18 notice to stderr, then
re-checks `HasStagedChanges`; if still false, returns exit-2 "Nothing to commit." On a clean tree
`AddAll` is a no-op and `StagedFileCount` returns 0, so the notice reads "(0 files)" right before the
exit-2. The generation pipeline (`pkg/stagehand.GenerateCommit` → `generate.CommitStaged`) is only
reached when something is staged; it independently returns `ErrNothingToCommit` if the staged diff is
empty, which `handleGenError` (default_action.go:179) also maps to exit-2 "Nothing to commit."

---

## Start Here

**Part A:** open `pkg/stagehand/stagehand.go` (`buildDeps`, ~line 155) — this is the single seam. Decide
apply-vs-remove; the parent must pick. If applying, the edit is ~5 lines after `m.Validate()`. If
removing, the edit touches config.go / file.go / git.go / config.go template + ~6 test files.

**Part B:** open `internal/cmd/default_action.go` lines 74-78. Add `if n == 0 { return ...NothingToCommit... }`
between the `StagedFileCount` error guard and the `Fprintln` notice. Add a regression assertion to
`TestRunDefault_NothingStaged_FR17` in `internal/cmd/default_action_test.go` (~line 293).

## Supervisor coordination
None needed — findings are self-contained; both fixes are local and well-bounded. No decision blocking.
