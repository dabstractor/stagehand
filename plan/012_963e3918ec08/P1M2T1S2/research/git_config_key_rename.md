# S2 Verified Surface Map — Rename stagehand.* git-config keys → stagecoach.* (P1.M2.T1.S2)

> Verified against the LIVE codebase at `/home/dustin/projects/stagehand` (module path ALREADY
> `github.com/dustin/stagecoach` — M1 complete; on-disk dir name unchanged). The plan-staging dir
> `/home/dustin/projects/stagecoach` has ONLY plan/ — NOT the codebase. Research the code at
> `/home/dustin/projects/stagehand`. Research only.

## 1. The codebase is at /home/dustin/projects/stagehand (NOT .../stagecoach)

`pwd`-equivalent: `/home/dustin/projects/stagehand`. `head -1 go.mod` → `module github.com/dustin/stagecoach`
(M1 renamed the module path; the on-disk directory keeps its original name). The cwd of the plan
(`/home/dustin/projects/stagecoach`) contains only `plan/`. ALL research/edits target
`/home/dustin/projects/stagehand`.

## 2. THREE "stagehand." categories in .go files — ONLY ONE is S2's scope

`grep -rn 'stagehand\.' --include='*.go' /home/dustin/projects/stagehand` surfaces ~220 hits across ~25 files,
but they split into THREE distinct categories. **S2 renames ONLY category A.** Misclassifying B or C is the
primary failure mode (it overlaps sibling tasks or renames non-git-config tokens).

### A. Git-config-key refs — RENAME (S2's scope, ~100 sites)
The `stagehand.` git-config section: `stagehand.provider`, `.model`, `.timeout`, `.output`, `.format`,
`.locale`, `.template`, `.autoStageAll`, `.verbose`, `.stripCodeFence`, `.push`, `.noVerify`, `.maxDiffBytes`,
`.maxMdLines`, `.tokenLimit`, `.diffContext`, `.maxDuplicateRetries`, `.subjectTargetChars`, `.role.planner`,
`.role.stager`, `.role.arbiter`, `.role.message`, `.commits`, `.max_commits`, `.context`, `.edit`,
`.auto_stage_all` (snake_case in the bootstrap template), `.does.not.exist` (test), and the `(stagehand.*)`
section shorthand. Appears as:
  - **Quoted key literals** (70): `gitConfigGet(repoDir, "stagehand.provider")`, `setGitConfig(t, repo, "stagehand.timeout", "90")`, test assertions `strings.Contains(err.Error(), "stagehand.timeout")`, table rows `{"stagehand.diffContext", "abc"}`. → `internal/config/git.go`, `git_test.go`, `load_test.go`.
  - **Help text** (15): root.go `"... (env STAGECOACH_PROVIDER, git stagehand.provider; ...)"`, `"...git stagehand.role.planner)"`. Preceded by `git ` (space).
  - **Template / Long descriptions**: bootstrap.go:243-269 + cmd/config.go:494-541 `git config stagehand.provider pi`, `(stagehand.*)`, `git config --get stagehand.<key>`.
  - **Error-source string**: load.go:212 `src = "git config stagehand.provider"`; git.go:152 `fmt.Errorf("git config stagehand.timeout: %w")`.
  - **Comments**: load.go:132/188/198/422/430, config.go:122/130, git.go:213, git_test.go:132/136/147/197/216, load_test.go:645/1186/1446/1453/1464, config_test.go:490.

### B. `.stagehand.toml` filename refs — LEAVE (P1.M2.T2.S1's scope)
The repo-local config FILENAME. `file.go:127 func repoLocalConfigPath() string { return ".stagehand.toml }`,
plus `".stagehand.toml"` test fixtures + comments in file_test.go/load.go/load_test.go/bootstrap.go/cmd/config.go/
cmd/*_test.go/signal_integration_test.go/stagecoach_test.go. The `stagehand.` here is preceded by `.` (`.stagehand.`).
**S2 MUST NOT rename these** — P1.M2.T2.S1 ("Rename config file discovery paths") owns the filename.

### C. `pkg/stagehand.<thing>` package-path comment refs — LEAVE (M1 residue; NOT git config keys)
Stale COMMENTS referencing the pre-rename package path: `pkg/stagehand.runPipeline`, `pkg/stagehand.buildDeps`
in render.go:34, reserve.go:66, generate.go:28/61, providers.go:106/124/137, decompose/roles.go. The `stagehand.`
here is preceded by `/` (`/stagehand.`). M1 renamed the actual directory + package declarations but missed
these comment refs. They are NOT git config keys. **S2 MUST NOT rename these** — out of scope (the final audit
P1.M5.T2.S1 catches residue; S2 is git-config-keys only).

## 3. The scope-safe mechanism — perl negative-lookbehind (ONE pass, provably correct)

The distinguishing rule: rename `stagehand.` UNLESS preceded by `.` (`.stagehand.toml`, category B) or `/`
(`pkg/stagehand.`, category C). A perl negative lookbehind does exactly this in one pass:

```
grep -rl 'stagehand\.' --include='*.go' /home/dustin/projects/stagehand | grep -v '/.git/' \
  | xargs perl -pi -e 's/(?<![.\/])stagehand\./stagecoach./g'
```

- `(?<![.\/])stagehand\.` matches `stagehand.` NOT preceded by `.` or `/`.
- `"stagehand.` (preceded by `"`) → renamed ✓. ` stagehand.` (help, preceded by space) → renamed ✓.
  `(stagehand.` → renamed ✓. `git config stagehand.` → renamed ✓.
- `.stagehand.` (filename, preceded by `.`) → NOT matched → `.stagehand.toml` survives ✓.
- `/stagehand.` (pkg path, preceded by `/`) → NOT matched → `pkg/stagehand.` survives ✓.

perl is on ubuntu-latest (CI) and every dev platform. It is the cleanest scope-safe approach — far less
error-prone than phased seds + manual comment edits. (Fallback if perl is somehow unavailable: phased seds —
`s/"stagehand\./"stagecoach./g` for quoted keys, `s/git stagehand\./git stagecoach./g` + `s/git config
stagehand\./git config stagecoach./g` + `s/(stagehand\.\*)/(stagecoach.*)/g` for anchored contexts — then
manual edit of the ~15 free-form comment refs listed in §2.A. Verify the same way.)

## 4. The verification gates (the scope-boundary proof)

After the rename, run ALL of these from `/home/dustin/projects/stagehand`:

```bash
# (1) ZERO residual git-config-key stagehand. refs (categories B and C excluded):
grep -rn 'stagehand\.' --include='*.go' . | grep -v '/.git/' | grep -vE '\.stagehand\.|/stagehand\.'
#   EXPECT: empty. (Excludes .stagehand. filename refs AND /stagehand. pkg-path refs.)

# (2) .stagehand.toml SURVIVES (P1.M2.T2.S1's scope — unchanged):
grep -rn '\.stagehand\.toml' --include='*.go' . | grep -v '/.git/'   # EXPECT: non-empty, UNCHANGED count

# (3) pkg/stagehand. comment refs SURVIVE (M1 residue, not S2's job):
grep -rn 'pkg/stagehand\.' --include='*.go' . | grep -v '/.git/'     # EXPECT: non-empty, UNCHANGED

# (4) the stub/setter↔reader coupling is consistent (tests set stagecoach.* AND git.go reads stagecoach.*):
go test ./internal/config/... -count=1    # EXPECT: PASS (loadGitConfig reads stagecoach.*; tests set stagecoach.*)
```

## 5. Adjacent/parallel task boundaries

- **P1.M2.T1.S1 (env vars STAGEHAND_→STAGECOACH_)** is "Implementing" IN PARALLEL. It targets UPPERCASE
  `STAGEHAND_` (case-sensitive); S2 targets lowercase `stagehand.`. They touch SOME of the same files
  (load.go, load_test.go, root.go, bootstrap.go, config.go) but DIFFERENT substrings — no conflict in content,
  though a parallel git-edit may produce a merge race (orchestrator's concern, not S2's). S2 does NOT depend
  on S1 landing first (lowercase is independent of uppercase).
- **P1.M2.T2.S1 (.stagehand.toml → .stagecoach.toml)** owns category B. S2 LEAVES it.
- **P1.M4.T1.S2 (docs)** owns docs/configuration.md git-config refs (Mode A docs ride with M4, NOT S2).
  S2 is `--include='*.go'` ONLY — no .md edits.
- **P1.M5.T2.S1 (final audit)** catches residual `pkg/stagehand.` comment refs (category C) later. S2 LEAVES them.

## 6. The setter↔reader consistency (why the rename must be atomic)

`internal/config/git.go` loadGitConfig READS `"stagehand.provider"` via gitConfigGet; `git_test.go`/`load_test.go`
SET `"stagehand.provider"` via setGitConfig. The perl pass renames BOTH atomically → setter writes `stagecoach.*`,
reader reads `stagecoach.*` → they match → `go test ./internal/config/...` passes. If only one side renamed,
tests would fail (set `stagecoach.provider`, read `stagehand.provider` → not found → assertion failure).
