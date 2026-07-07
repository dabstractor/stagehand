# P1.M1.T3.S1 Research Findings — Add v2.1 [generation] keys to exampleConfigTemplate

## 1. The edit site (authoritative)

`internal/cmd/config.go` — `const exampleConfigTemplate` starts at **L497** (backtick raw string literal).
The `[generation]` block spans **L563–L574**:

```
# ---------------------------------------------------------------------------
# [generation] — diff capture & output tuning (PRD §16.2)
# ---------------------------------------------------------------------------
# [generation]
# max_diff_bytes        = 300000  # byte cap on the non-markdown diff section
# max_md_lines          = 100     # per-file line cap for markdown diffs
# max_duplicate_retries = 3       # re-generation attempts when the subject duplicates a recent commit
# subject_target_chars  = 50      # target subject-line length for truncation
# output                = "raw"   # agent output mode: "raw" | "json" — applies to parsing across ALL providers
# strip_code_fence      = true    # strip ` + "`" + ` fences from agent output (all providers)
# max_commits           = 12      # safety cap on auto-decompose (PRD §9.14 FR-M4); default 12
# binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
# NOTE: [generation] output/strip_code_fence override any per-provider [provider.<name>] values.
```

**Insertion point**: AFTER the `# binary_extensions …` line (L573) and BEFORE the `# NOTE: [generation] …` line (L574). All template lines are at **column 0** (no Go indentation — raw literal content is verbatim).

## 2. Column alignment (verified by counting existing lines)

- The `=` sign lands at **0-indexed column 24**. Rule: key is **left-justified to width 21**, then ` = ` (space, `=`, space). Examples: `max_duplicate_retries` (21, no pad), `max_diff_bytes` (14 → 7 pad), `binary_extensions` (17 → 4 pad).
- The trailing `#` description marker lands after a **value field of width 8** (left-justified). Examples: `300000`(6)+2sp, `"raw"`(5)+3sp, `true`(4)+4sp, `[]`(2)+6sp.

New keys → pad widths: `exclude`(7→14), `format`(6→15), `locale`(6→15), `template`(8→13), `push`(4→17).

## 3. CRITICAL — backtick handling in the push line

`exampleConfigTemplate` is a Go **raw string literal delimited by backticks** (`` ` ``). A literal backtick CANNOT appear inside it. The `push` description contains `` `git push` `` (markdown code span), so the line MUST be split with the `` + "`" + `` concatenation idiom — **exactly** as the existing two lines do:

- L560: `# auto_stage_all = true     # run ` + "`git add -A`" + ` when nothing is staged`
- L571: `# strip_code_fence      = true    # strip ` + "`" + ` fences from agent output (all providers)`

Rendered output of the push line must be:
`# push                  = false   # run `git push` after a fully-successful run; on failure commits stand (§9.22 FR-P1)`

The other 4 new lines contain NO backticks → plain single raw-literal segments (no split). The `template` line uses `$msg` inside double quotes — `$` is NOT special in a raw string literal, so **no escaping** is needed.

## 4. Defaults verified (Config struct + docs/configuration.md)

| key      | toml tag  | default    | source                                           |
|----------|-----------|------------|--------------------------------------------------|
| exclude  | `exclude` | `[]` (nil) | internal/config/config.go `Exclude []string`     |
| format   | `format`  | `"auto"`   | config.go Format comment; docs/configuration.md L131 |
| locale   | `locale`  | `""`       | config.go Locale comment; docs L132              |
| template | `template`| `""`       | config.go Template comment; docs L133            |
| push     | `push`    | `false`    | config.go Push comment; docs L134                |

docs/configuration.md commented example (L105–108) wording agrees with the issue_analysis.md lines.

## 5. Test approach — existing tests are SELF-UPDATING (no golden snapshot)

In `internal/cmd/config_test.go`:
- `TestConfigInit_Template_WritesInert` (L409) asserts `got == exampleConfigTemplate` (writes to GLOBAL path via `setupNoRepo`). Both sides reference the same const → adding lines changes both equally → **test stays green automatically**.
- `TestConfigInit_TemplateIsInert` (L448) asserts no uncommented TOML header + sections/env/git docs present. New commented lines do **not** break it.
- `TestConfigInit_Force_OverwritesTemplate` (L564) asserts `content == exampleConfigTemplate` → also self-updating.
- `TestConfigInit_TemplateFlag_CollisionSafe` (L498) — unrelated (flag parsing).

**Conclusion**: There is NO hardcoded snapshot/golden file of the template. The existing exact-match tests compare to the const, so they need NO change. Only a NEW positive test is required.

## 6. `config init --template` honors `--config`

`runConfigInit` (config.go L432): `path := config.ResolveConfigPath(flagConfig)` → writes the template to that path. docs/configuration.md L69 confirms `--config` is honored by `config init`. Existing precedent: `TestConfigInit_ConfigFlag_WritesOverride` (L198) uses `["--config", override, "config", "init"]` with `override := filepath.Join(t.TempDir(), "foo.toml")` (parent exists). So the new test can use `--config <tmpPath>` cleanly.

## 7. Test helper idiom (from TestConfigInit_TemplateIsInert)

```go
_, origOut, origErr, origRunE := saveRootState(t)
defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()
setupNoRepo(t)
rootCmd.SetOut(io.Discard); rootCmd.SetErr(io.Discard)
rootCmd.SetArgs([]string{"config", "init", "--template", "--config", tmpPath})
err := Execute(context.Background())
```
Then `os.ReadFile(tmpPath)` + `strings.Contains`.

## 8. Validation commands (verified present in repo)

- `gofmt -w internal/cmd/config.go internal/cmd/config_test.go`
- `go build ./...`
- `go vet ./internal/cmd/...`
- `go test ./internal/cmd/... -run 'ConfigInit_Template' -v`
- `go test ./...`
- `golangci-lint run ./internal/cmd/...` (if configured)
- Manual: `go build -o /tmp/stagecoach ./cmd/stagecoach && /tmp/stagecoach config init --template --config /tmp/ref.toml && grep -nE 'exclude|format|locale|template|push' /tmp/ref.toml`
